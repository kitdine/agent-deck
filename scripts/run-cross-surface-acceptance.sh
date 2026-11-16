#!/usr/bin/env bash
# Task 8 E7 orchestration. Default invocation validates inputs only.
# Raw measurements remain private; no installation, publication or real-log freeze.
set -euo pipefail
umask 077

config=${1:?usage: run-cross-surface-acceptance.sh CONFIG.json [--run]}
config="$(cd "$(dirname "$config")" && pwd)/$(basename "$config")"
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"
mode=${2:---prepare}
case "$mode" in --prepare|--run) ;; *) echo "expected --prepare or --run" >&2; exit 2 ;; esac
value() { jq -er ".$1" "$config"; }
perf=$(value perf)
app=$(value candidate_app)
cli=$(value candidate_cli)
base_app=$(value baseline_app)
base_cli=$(value baseline_cli)
b0=$(value b0_cli)
cand=$(value candidate_batch)
base=$(value baseline_batch)
out=$(value output)
anchor=$(value anchor_date)
hashes=$(value hashes)
config_digest=$(shasum -a 256 "$config")
grep -Fx -- "$config_digest" "$hashes" >/dev/null || { echo "configuration is not in the sealed inputs" >&2; exit 1; }
for executable in "$perf" "$cli" "$base_cli" "$b0" "$app/Contents/MacOS/AgentDeck" "$base_app/Contents/MacOS/AgentDeck"; do
  test -x "$executable" || { echo "missing executable: $executable" >&2; exit 1; }
done
shasum -a 256 -c "$hashes"
for batch in "$cand" "$base"; do
  jq -e --arg day "$anchor" '.anchor == $day and .tz == "America/New_York"' "$batch/batch.json" >/dev/null
done
test "$(jq -r .working_copy.working_copy_identity "$cand/batch.json")" = "$(jq -r .working_copy.working_copy_identity "$base/batch.json")"
test "$(TZ=America/New_York date +%F)" = "$anchor" || { echo "anchor day changed; prepare fresh batches" >&2; exit 1; }
if [[ "$mode" != --run ]]; then
  echo "Inputs verified. No pilot or formal measurement started."
  exit 0
fi
# The tool waits for detached workers through ps. A sandbox-denied process
# list must not be mistaken for an empty list and a completed trace.
/bin/ps -Ao pid=,comm= >/dev/null

mkdir -p "$out"
exec > >(tee -a "$out/run.log") 2>&1
domain=com.kitdine.agentdeck.acceptance
quota_changed=false
quota_present=false
quota_previous=
reporter_pid=
restore_quota() {
  local restored_bool
  if $quota_changed; then
    if $quota_present; then
      case "$quota_previous" in
        1|true|YES) restored_bool=true ;;
        0|false|NO) restored_bool=false ;;
        *) echo "Unsupported saved quota preference; leaving current value unchanged" >&2; return 1 ;;
      esac
      /usr/bin/defaults write "$domain" quota.probeEnabled -bool "$restored_bool" || return 1
    else
      /usr/bin/defaults delete "$domain" quota.probeEnabled || return 1
    fi
    quota_changed=false
  fi
}
cleanup() {
  result=$?
  trap - EXIT
  [[ -z "$reporter_pid" ]] || kill "$reporter_pid" 2>/dev/null || true
  restore_quota || result=1
  printf '%s exit=%s\n' "$(date -u +%FT%TZ)" "$result"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
progress() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*" | tee "$out/progress.txt"; }
(
  while sleep 1800; do
    printf '\n30-minute progress: '
    cat "$out/progress.txt"
    [[ ! -f "$out/current.log" ]] || tail -3 "$out/current.log"
  done
) &
reporter_pid=$!

run_condition() {
  local condition=$1 key=$2 pilot_mode=$3
  shift 3
  local destination="$out/$key-$pilot_mode"
  local args=()
  [[ "$pilot_mode" != formal ]] || args+=(--formal)
  # A completed condition may be resumed; incomplete conditions are never
  # silently overwritten or selectively rerun.
  if [[ -f "$destination.complete.sha256" ]]; then
    cmp -s "$hashes" "$destination.inputs.sha256" || {
      echo "completed condition has different or unrecorded inputs; explicit reuse assessment required: $destination" >&2
      return 1
    }
    shasum -a 256 -c "$destination.complete.sha256" >/dev/null
    progress "reuse completed $key $pilot_mode"
    return
  fi
  [[ ! -e "$destination" ]] || { echo "incomplete condition retained: $destination" >&2; return 1; }
  shasum -a 256 -c "$hashes" >/dev/null
  cp "$hashes" "$destination.inputs.sha256"
  progress "$key $pilot_mode starting"
  "$perf" run --stop-on-failure "${args[@]}" --condition "$condition" --app "$app" --agentdeck "$cli" \
    --batch "$cand" --out "$destination" "$@" > "$out/current.log" 2>&1
  cp "$out/current.log" "$destination/run.log"
  local stats_path arm
  if [[ "$condition" == P09 ]]; then
    "$perf" stats "$destination" "$condition" > "$destination/stats.json"
    jq -e '.tool_invalid == 0' "$destination/stats.json" >/dev/null
  else
    for arm in baseline candidate; do
      stats_path="$destination/$arm.stats.json"
      "$perf" stats "$destination/$arm" "$condition" > "$stats_path"
      jq -e '.tool_invalid == 0' "$stats_path" >/dev/null
    done
  fi
  if [[ "$pilot_mode" == formal ]]; then
    "$perf" compare --batch-out "$destination" --condition "$condition" > "$destination/compare.json"
  fi
  # Seal all metadata, traces and statistics, not only a completion flag.
  find "$destination" -type f -exec shasum -a 256 {} + > "$destination.complete.sha256"
  progress "$key $pilot_mode completed"
}

while IFS= read -r condition; do
  if [[ "$condition" == P06 ]]; then
    # Both pilots and formal arms require 3h26m before overhead (8 windows
    # with their 120s settle). Do not knowingly start a cross-midnight batch.
    clock_time=$(TZ=America/New_York date '+%H %M %S')
    read -r hour minute second <<< "$clock_time"
    remaining=$((86400 - 10#$hour * 3600 - 10#$minute * 60 - 10#$second))
    if (( remaining <= 12360 + 2700 )); then
      progress "P06 deferred: insufficient time before local midnight; prepare next-day batches"
      exit 78
    fi
    if quota_previous=$(/usr/bin/defaults read "$domain" quota.probeEnabled 2>/dev/null); then quota_present=true; fi
    jq -n --argjson present "$quota_present" --arg value "$quota_previous" \
      '{present:$present,value:$value}' > "$out/P06-original-quota-preference.json"
    /usr/bin/defaults write "$domain" quota.probeEnabled -bool false
    quota_changed=true
  fi
  for schedule in pilot formal; do
    case "$condition" in
      P09-B0) run_condition P09 "$condition" "$schedule" --b0 "$b0" ;;
      P09-S0) run_condition P09 "$condition" "$schedule" --b0 "$base_cli" --p09-baseline-traced ;;
      *) run_condition "$condition" "$condition" "$schedule" --baseline-app "$base_app" \
           --baseline-agentdeck "$base_cli" --baseline-batch "$base" ;;
    esac
  done
  [[ "$condition" != P06 ]] || restore_quota
done < <(jq -r '.conditions // ["P01-first","P01-repeat","P01-rebuild","P02","P03","P04","P11-first","P11-full","P07","P08","P10","P09-B0","P09-S0","P06"] | .[]' "$config")
progress "selected E7 conditions complete; evidence recording and independent review remain"
