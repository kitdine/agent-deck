#!/usr/bin/env bash
set -euo pipefail

# Cross-surface ownership acceptance (installed-state-ownership task 8).
#
# One agentdeck_acceptance build is carried through the real installers
# (scripts/manage-install.sh, scripts/install-local-app.sh, install claim and
# install register) in a temporary account, PREFIX, APPDIR and Homebrew-shaped
# Cellar. Nothing here resolves the machine's real account: the guard below
# refuses to continue unless the binary is an acceptance build.
#
# Every "does not access the injected real state" observation runs under the
# access barrier, after a positive control (the guard itself refuses inside the
# barrier) and a negative control (touching the directory is killed). An
# unavailable barrier, for example a nested sandbox, fails the run; it is never
# downgraded to a no-write assertion.
#
# Outside this script's evidence: real Homebrew file actions (the cask steps
# below only replay the registration protocol on a local stand-in), the native
# App's ownership session (scripts/test-macos-app.sh), and two real Apps
# running side by side.

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-ownership.XXXXXX")
background=()
cleanup() {
  local pid
  for pid in "${background[@]:-}"; do
    [ -z "$pid" ] || kill "$pid" 2>/dev/null || :
  done
  wait 2>/dev/null || :
  rm -rf "$temporary"
}
trap cleanup EXIT
temporary=$(cd "$temporary" && pwd -P)

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}
export GOMODCACHE=${GOMODCACHE:-/private/tmp/agent-deck-go-mod}
go_bin=${GO:-go}

account="$temporary/account"
home="$temporary/home"
real="$account/.agentdeck"
receipt="$account/Library/Application Support/AgentDeck/installation.json"
lock="$account/Library/Application Support/AgentDeck/ownership.lock"
export AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account"
export HOME="$home"
mkdir -p "$account" "$home"

step() { printf '\n== %s\n' "$*"; }
pass() { printf 'ok   %s\n' "$*"; }
fail() {
  printf 'FAIL %s\n' "$*" >&2
  [ ! -s "$temporary/last.log" ] || sed 's/^/     /' "$temporary/last.log" >&2
  exit 1
}

# run <expected exit> <description> <command...>; output is kept in last.log.
run() {
  local want=$1 what=$2 code=0
  shift 2
  "$@" >"$temporary/last.log" 2>&1 || code=$?
  [ "$code" = "$want" ] || fail "$what: exit $code, want $want"
}
said() { grep -F -- "$1" "$temporary/last.log" >/dev/null || fail "$2: output lacks '$1'"; }
unsaid() { ! grep -F -- "$1" "$temporary/last.log" >/dev/null || fail "$2: output contains '$1'"; }
receipt_field() { sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p" "$receipt"; }

# as_account runs a program the way an installed owner is run: HOME is the
# account, so the default state root is the injected real directory.
as_account() { HOME="$account" "$@"; }

# --- the acceptance binary, and proof that it is one ---
build="$temporary/build/agentdeck"
mkdir -p "$(dirname "$build")"
(cd "$root" && "$go_bin" build -mod=vendor -trimpath -tags agentdeck_acceptance -o "$build" ./cmd/agentdeck)
"$go_bin" tool nm "$build" | grep -F 'installation.acceptanceAccountHome' >/dev/null || {
  echo "refusing to run: $build would use the real account" >&2
  exit 1
}

place() { # place <path>: an independent copy of the acceptance binary
  mkdir -p "$(dirname "$1")"
  cp "$build" "$1"
}
make_app() { # make_app <bundle> <version>
  mkdir -p "$1/Contents/MacOS"
  place "$1/Contents/Helpers/agentdeck"
  printf '#!/bin/sh\necho AgentDeck %s\n' "$2" >"$1/Contents/MacOS/AgentDeck"
  chmod 0755 "$1/Contents/MacOS/AgentDeck"
  printf 'version %s\n' "$2" >"$1/Contents/version.txt"
}
keg() { # keg <prefix> <formula> <version>: prints the keg's executable
  place "$1/Cellar/$2/$3/bin/agentdeck"
  printf '%s\n' "$1/Cellar/$2/$3/bin/agentdeck"
}
point_opt() { # point_opt <prefix> <formula> <version>
  mkdir -p "$1/opt"
  rm -f "$1/opt/$2"
  ln -s "$1/Cellar/$2/$3" "$1/opt/$2"
}

# --- injected real state: data that only the owner may read ---
seed="$temporary/seed-state"
printf 'synthetic-secret\n' | "$build" --state-dir "$seed" provider add existing-owner-data \
  --endpoint https://example.invalid --clients codex >/dev/null
mv "$seed" "$real"
printf 'protected contents\n' >"$real/sentinel"
mkdir -p "$account/.codex" "$account/.claude"
printf "model = 'gpt-5.4'\n" >"$account/.codex/config.toml"
printf '{}\n' >"$account/.claude/settings.json"
printf '# account zshrc\n' >"$account/.zshrc"

profile="(version 1)
(allow default)
(deny file-read* file-write* (subpath \"$real\") (with send-signal SIGKILL))
(allow file-read-metadata (literal \"$real\"))"
barrier() { /usr/bin/sandbox-exec -p "$profile" -- "$@"; }

# controls <binary>: both controls must hold before any barrier observation.
controls() {
  local code=0
  barrier "$1" --state-dir "$real" version >"$temporary/last.log" 2>&1 || code=$?
  [ "$code" = 1 ] && grep -F state_ownership_refused "$temporary/last.log" >/dev/null ||
    fail "access barrier positive control unavailable or incorrect (exit $code)"
  code=0
  barrier /usr/bin/stat -f %z "$real/sentinel" >"$temporary/last.log" 2>&1 || code=$?
  [ "$code" = 137 ] || fail "access barrier did not kill a forbidden access (exit $code)"
}

owner_data() { # owner_data <binary> <description>: the owner reads the injected data
  run 0 "$2 migrates the injected state" as_account "$1" --format json state migrate
  run 0 "$2 reads the injected state" as_account "$1" provider list
  said existing-owner-data "$2 reads the injected state"
}
refused_behind_barrier() { # refused_behind_barrier <binary> <description> <reason>
  controls "$1"
  run 1 "$2 with the real root" barrier "$1" --state-dir "$real" provider list
  said state_ownership_refused "$2 with the real root"
  said "$3" "$2 with the real root"
  run 0 "$2 by default" barrier env HOME="$account" "$1" provider list
  said 'No providers.' "$2 by default"
}
installation_reason() { # installation_reason <binary>: doctor's installation reason
  as_account "$1" --format json doctor 2>/dev/null |
    python3 -c 'import json,sys; print(next(c["reason"] for c in json.load(sys.stdin)["data"]["checks"] if c["name"]=="installation"))'
}
lock_held() { # true while some process holds the ownership lock
  python3 -c 'import fcntl,sys
f=open(sys.argv[1],"a")
try: fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB)
except OSError: sys.exit(0)
sys.exit(1)' "$lock"
}
wait_for() { # wait_for <description> <command...>
  local what=$1 i
  shift
  for i in $(seq 1 200); do
    if "$@"; then return 0; fi
    sleep 0.05
  done
  fail "timed out waiting for $what"
}

lock_free() { ! lock_held; }
stop() { # stop <pid>: end a background session and wait for its lock to go
  kill "$1"
  wait "$1" 2>/dev/null || :
  wait_for 'the ownership lock to be released' lock_free
}

# All installer calls, including scenarios with explicit FORCE/TAKEOVER, go
# through this boundary. PREFIX alone cannot override inherited BINDIR/DATADIR.
install_env() {
  env HOME="$home" AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account" TMPDIR="$temporary" \
    PREFIX="$temporary/prefix" BINDIR="$temporary/prefix/bin" DATADIR="$temporary/prefix/share/agentdeck" \
    APPDIR="$temporary/Applications" BIN_LINK="$temporary/bin/agentdeck" RECORD_DIR="$temporary/records" \
    LSREGISTER=none COMPLETION_SHELL=none COMPLETION_RC="$home/.zshrc" FORCE=0 TAKEOVER=0 "$@"
}
manage() { install_env bash "$root/scripts/manage-install.sh" "$@"; }
local_app() { install_env bash "$root/scripts/install-local-app.sh" "$@"; }
make_binary="$temporary/prefix/bin/agentdeck"
app="$temporary/Applications/AgentDeck.app"
helper="$app/Contents/Helpers/agentdeck"
make_app "$temporary/build-1/AgentDeck.app" 1
make_app "$temporary/build-2/AgentDeck.app" 2

step '1 not installed: no access to the real state, client configuration refused'
dev="$temporary/worktree/dist/agentdeck"
place "$dev"
controls "$dev"
run 0 'default state root' barrier "$dev" provider list
said 'No providers.' 'default state root'
run 1 'explicit real root' barrier "$dev" --format json --state-dir "$real" provider list
said state_ownership_refused 'explicit real root'
said '"resource":"state_root"' 'explicit real root'
ln -s "$real" "$temporary/alias"
run 1 'symbolic link to the real root' barrier "$dev" --state-dir "$temporary/alias" provider list
said state_ownership_refused 'symbolic link to the real root'
run 0 'doctor on the real root' barrier "$dev" --format json --state-dir "$real" doctor
said '"partial":true' 'doctor on the real root'
said checks_skipped 'doctor on the real root'
run 0 'default state root in the account home' barrier env HOME="$account" "$dev" provider list
said 'No providers.' 'default state root in the account home'
config_before=$(cat "$account/.codex/config.toml" "$account/.claude/settings.json" "$account/.zshrc" | shasum -a 256)
run 1 'hook setup into the real client configuration' barrier env HOME="$account" "$dev" --format json usage hook setup --client all
said state_ownership_refused 'hook setup into the real client configuration'
said '"resource":"client_config"' 'hook setup into the real client configuration'
run 1 'shell setup into the real shell configuration' barrier env HOME="$account" "$dev" shell setup zsh
said state_ownership_refused 'shell setup into the real shell configuration'
[ "$(cat "$account/.codex/config.toml" "$account/.claude/settings.json" "$account/.zshrc" | shasum -a 256)" = "$config_before" ] ||
  fail 'a refused client configuration write changed a file'
test ! -e "$receipt" || fail 'an uninstalled process wrote a receipt'
pass 'uninstalled process: isolated by default, refused on the real root, client configuration unchanged'

step '2 local App installation becomes the owner'
run 0 'local App install' local_app install "$temporary/build-1/AgentDeck.app"
[ "$(receipt_field executable)" = "$helper" ] && [ "$(receipt_field channel)" = local_app ] || fail 'local App receipt'
[ "$(installation_reason "$helper")" = local_app ] || fail 'doctor does not report the local App owner'
owner_data "$helper" 'local App owner'
pass 'local_app owner reads and migrates the injected state'

step '3 another program is refused without takeover'
before=$(cat "$receipt")
run 1 'make install over another owner' manage install "$build"
said owned_by_other 'make install over another owner'
[ "$(cat "$receipt")" = "$before" ] || fail 'a refused installation changed the receipt'
test ! -e "$make_binary" || fail 'a refused installation published its program'
pass 'owned_by_other, receipt unchanged, nothing published'

step '4 takeover is refused while the owner holds a session'
HOME="$account" "$helper" --format ndjson watch --interval 200ms >"$temporary/watch.log" 2>&1 &
watcher=$!
background+=("$watcher")
wait_for 'the owner session to hold the ownership lock' lock_held
run 1 'takeover of a running owner' install_env TAKEOVER=1 bash "$root/scripts/manage-install.sh" install "$build"
said owner_running 'takeover of a running owner'
[ "$(cat "$receipt")" = "$before" ] || fail 'a refused takeover changed the receipt'
test ! -e "$make_binary" || fail 'a refused takeover published its program'
kill -0 "$watcher" 2>/dev/null || fail 'the owner session did not survive the refused takeover'
pass 'owner_running, receipt unchanged, owner session still running'

step '5 takeover succeeds after the owner stops; the old owner is refused'
stop "$watcher"
run 0 'takeover after the owner stopped' install_env TAKEOVER=1 bash "$root/scripts/manage-install.sh" install "$build"
[ "$(receipt_field executable)" = "$make_binary" ] && [ "$(receipt_field channel)" = make ] || fail 'make receipt after takeover'
before=$(cat "$receipt")
run 1 'make reinstall without explicit force' manage install "$build"
said 'rerun with FORCE=1' 'make reinstall without explicit force'
[ "$(cat "$receipt")" = "$before" ] || fail 'a refused reinstall changed the receipt'
owner_data "$make_binary" 'make owner'
refused_behind_barrier "$helper" 'superseded local App helper' receipt_other_executable
pass 'make owner after takeover; superseded helper cannot open the real state'

step '6 formula: claim, old keg, takeover, upgrade renewal, retained keg, second prefix, rc, stale receipt'
brew="$temporary/homebrew"
keg_09=$(keg "$brew" agentdeck 0.9)
keg_10=$(keg "$brew" agentdeck 1.0)
point_opt "$brew" agentdeck 1.0
before=$(cat "$receipt")
run 1 'formula claim over another owner' as_account "$keg_10" install claim
said owned_by_other 'formula claim over another owner'
run 1 'claim from a keg that is not current' as_account "$keg_09" install claim --takeover
said not_current_formula_keg 'claim from a keg that is not current'
[ "$(cat "$receipt")" = "$before" ] || fail 'a refused claim changed the receipt'
run 0 'formula claim with takeover' as_account "$keg_10" --format json install claim --takeover
said '"channel":"homebrew_formula"' 'formula claim with takeover'
[ "$(receipt_field executable)" = "$keg_10" ] || fail 'formula receipt after claim'
owner_data "$keg_10" 'formula owner'
refused_behind_barrier "$make_binary" 'superseded make program' receipt_other_executable

keg_20=$(keg "$brew" agentdeck 2.0)
point_opt "$brew" agentdeck 2.0
rm -rf "$brew/Cellar/agentdeck/1.0" "$brew/Cellar/agentdeck/0.9"
run 0 'upgraded keg renews with an ordinary command' as_account "$keg_20" provider list
said existing-owner-data 'upgraded keg renews with an ordinary command'
[ "$(receipt_field executable)" = "$keg_20" ] && [ "$(receipt_field channel)" = homebrew_formula ] || fail 'renewed formula receipt'

keg_30=$(keg "$brew" agentdeck 3.0)
point_opt "$brew" agentdeck 3.0
before=$(cat "$receipt")
controls "$keg_30"
run 0 'new keg beside a retained old keg' barrier env HOME="$account" "$keg_30" provider list
said 'No providers.' 'new keg beside a retained old keg'
[ "$(cat "$receipt")" = "$before" ] || fail 'a retained old keg was renewed away'
[ "$(installation_reason "$keg_30")" = receipt_other_executable ] || fail 'retained keg: doctor reason'
owner_data "$keg_20" 'retained formula owner'

second=$(keg "$temporary/homebrew-second" agentdeck 3.0)
point_opt "$temporary/homebrew-second" agentdeck 3.0
run 1 'claim from a second Homebrew prefix' as_account "$second" install claim
said owned_by_other 'claim from a second Homebrew prefix'
rc=$(keg "$brew" agentdeck-rc 3.1-rc1)
point_opt "$brew" agentdeck-rc 3.1-rc1
run 1 'claim from the rc formula' as_account "$rc" install claim
said owned_by_other 'claim from the rc formula'
[ "$(cat "$receipt")" = "$before" ] || fail 'a refused claim changed the receipt'
run 0 'rc formula claim with takeover' as_account "$rc" install claim --takeover
[ "$(receipt_field executable)" = "$rc" ] || fail 'rc receipt after takeover'
owner_data "$rc" 'rc formula owner'
refused_behind_barrier "$keg_20" 'superseded stable keg' receipt_other_executable

rm -rf "$brew/Cellar/agentdeck-rc"
[ "$(installation_reason "$keg_30")" = receipt_missing ] || fail 'stale receipt: doctor reason'
refused_behind_barrier "$keg_30" 'program beside a stale receipt' receipt_missing
run 0 'claim over a stale receipt needs no takeover' as_account "$keg_30" install claim
[ "$(receipt_field executable)" = "$keg_30" ] || fail 'receipt after claiming over a stale one'
owner_data "$keg_30" 'formula owner after a stale receipt'
pass 'formula claim, renewal and every single-owner refusal'

step '6a an uninstalled App stand-in keeps running through install, takeover and uninstall'
standin="$temporary/DerivedData/Debug/AgentDeck.app/Contents/Helpers/agentdeck"
place "$standin"
controls "$standin"
HOME="$account" /usr/bin/sandbox-exec -p "$profile" -- "$standin" --format ndjson watch --interval 200ms >"$temporary/standin.log" 2>&1 &
standin_pid=$!
background+=("$standin_pid")
sleep 1
standin_alive() { kill -0 "$standin_pid" 2>/dev/null || fail "the App stand-in stopped during: $1"; }
standin_alive 'startup'
! lock_held || fail 'an uninstalled App stand-in holds the ownership lock'

run 0 'make takeover beside the stand-in' install_env FORCE=1 TAKEOVER=1 bash "$root/scripts/manage-install.sh" install "$build"
unsaid owner_running 'make takeover beside the stand-in'
[ "$(receipt_field channel)" = make ] || fail 'make receipt beside the stand-in'
standin_alive 'make takeover'
run 0 'local App takeover beside the stand-in' install_env FORCE=1 TAKEOVER=1 \
  bash "$root/scripts/install-local-app.sh" install "$temporary/build-2/AgentDeck.app"
unsaid owner_running 'local App takeover beside the stand-in'
[ "$(receipt_field channel)" = local_app ] || fail 'local App receipt beside the stand-in'
grep -F 'version 2' "$app/Contents/version.txt" >/dev/null || fail 'local App takeover did not publish the new App'
standin_alive 'local App takeover'
run 0 'superseded make uninstall beside the stand-in' manage uninstall
said 'skipped: a later installation now owns' 'superseded make uninstall beside the stand-in'
test -f "$make_binary" || fail 'a superseded uninstall removed the program'
run 0 'local App uninstall beside the stand-in' local_app uninstall
unsaid owner_running 'local App uninstall beside the stand-in'
test ! -e "$app" || fail 'local App uninstall left the App'
test ! -e "$receipt" || fail 'local App uninstall left its receipt'
standin_alive 'local App uninstall'
run 0 'make install beside the stand-in' install_env FORCE=1 bash "$root/scripts/manage-install.sh" install "$build"
owner_data "$make_binary" 'make owner beside the stand-in'
run 0 'make uninstall beside the stand-in' manage uninstall
unsaid owner_running 'make uninstall beside the stand-in'
test ! -e "$make_binary" || fail 'make uninstall left the program'
test ! -e "$receipt" || fail 'make uninstall left its receipt'
standin_alive 'make uninstall'
pass 'make and local App: install, takeover and unregister --remove all succeed; receipts and targets removed'

step '7 cask registration protocol on a local stand-in (Homebrew file actions are not run here)'
cask_app="$temporary/CaskApplications/AgentDeck.app"
make_app "$cask_app" cask
cask_helper="$cask_app/Contents/Helpers/agentdeck"
run 0 'cask postflight registration' as_account "$cask_helper" --format json install register --channel homebrew_cask --executable "$cask_helper"
said '"channel":"homebrew_cask"' 'cask postflight registration'
[ "$(installation_reason "$cask_helper")" = homebrew_cask ] || fail 'doctor does not report the cask owner'
owner_data "$cask_helper" 'cask owner'
standin_alive 'cask registration'
HOME="$account" "$cask_helper" --format ndjson watch --interval 200ms >"$temporary/cask-watch.log" 2>&1 &
cask_watcher=$!
background+=("$cask_watcher")
wait_for 'the cask owner session to hold the ownership lock' lock_held
before=$(cat "$receipt")
run 0 'cask re-registration while its own session runs' as_account "$cask_helper" --format json install register --channel homebrew_cask --executable "$cask_helper"
said '"unchanged":true' 'cask re-registration while its own session runs'
[ "$(cat "$receipt")" = "$before" ] || fail 'a busy cask re-registration rewrote the receipt'
stop "$cask_watcher"
# A cask uninstall removes the App and never runs unregister: the receipt stays
# behind, names nothing, and the next installation needs no takeover.
rm -rf "$cask_app"
test -f "$receipt" || fail 'the stand-in cask removal deleted the receipt'
[ "$(installation_reason "$dev")" = receipt_missing ] || fail 'removed cask: doctor reason'
run 0 'make install over a stale cask receipt' install_env FORCE=1 bash "$root/scripts/manage-install.sh" install "$build"
[ "$(receipt_field channel)" = make ] || fail 'make receipt over a stale cask receipt'
owner_data "$make_binary" 'make owner after the cask was removed'
standin_alive 'cask removal and reinstallation'
stop "$standin_pid"
pass 'homebrew_cask owner, busy re-registration unchanged, removed cask leaves a stale receipt that needs no takeover'

step '8 a damaged receipt is explained by doctor'
printf 'not a receipt\n' >"$receipt"
[ "$(installation_reason "$make_binary")" = receipt_invalid ] || fail 'damaged receipt: doctor reason'
refused_behind_barrier "$make_binary" 'program beside a damaged receipt' receipt_invalid
run 0 'doctor on the real root beside a damaged receipt' barrier "$make_binary" --format json --state-dir "$real" doctor
said '"partial":true' 'doctor on the real root beside a damaged receipt'
said checks_skipped 'doctor on the real root beside a damaged receipt'
said receipt_invalid 'doctor on the real root beside a damaged receipt'
run 0 'doctor text beside a damaged receipt' as_account "$make_binary" doctor
said installation 'doctor text beside a damaged receipt'
pass 'receipt_invalid reported; doctor on the real root is a partial report'

grep -F 'protected contents' "$real/sentinel" >/dev/null || fail 'the injected state lost its sentinel'
printf '\nownership acceptance: cross-surface scenarios passed in a temporary account behind the access barrier.\n'
