#!/usr/bin/env bash
set -euo pipefail

# The cask's declarations, its registration step, formula-to-cask migration,
# mutual exclusion and the uninstall boundary, all in a temporary HOME, a
# temporary account directory, a temporary application directory, and a
# temporary prefix.
#
# Boundary, stated rather than implied:
#
# - Homebrew owns the App's files. This script stands in for it with plain
#   copies and removals, so nothing here proves what Homebrew does: how it
#   publishes, replaces, rolls back or restores an App is proven only in CI, in
#   scripts/test-cask-real-brew.sh.
# - The registration step and the uninstall script are the rendered cask's own
#   text, read out by scripts/cask-local-driver.rb. The helper the step calls is
#   an agentdeck_acceptance build in the temporary application directory; its
#   receipt and ownership lock live in the temporary account. The step runs
#   here without Homebrew's sandbox, so its writable_paths declaration is
#   checked as text only.
# - The uninstall script names /usr/bin/pluginkit. It is never run as written:
#   every run substitutes a recording stub, so this machine's real extension
#   registrations are neither read nor changed.
# - No program staged by Homebrew is ever executed here. On a developer
#   machine that triggers Gatekeeper.
# - The one real `brew install --cask` below is the formula conflict refusal.
#   It fails in a preflight step that only runs /bin/sh, before any helper.
#   --local-only omits that refusal check and never invokes brew.

local_only=0
if [[ ${1:-} == --local-only ]]; then
  local_only=1
  shift
fi
if [[ $# != 0 ]]; then
  echo 'usage: test-cask-migration.sh [--local-only]' >&2
  exit 2
fi

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/private/tmp}/agentdeck-cask-migration.XXXXXX")
trap 'rm -rf "${temporary:?}"' EXIT
temporary=$(cd "$temporary" && pwd -P)

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}
export GOMODCACHE=${GOMODCACHE:-/private/tmp/agent-deck-go-mod}
go_bin=${GO:-go}

home="$temporary/home"
account="$temporary/account"
appdir="$temporary/Applications"
prefix="$temporary/brew"
formula_records="$temporary/formula-records"
step_temporary="$temporary/step-tmp"
pluginkit="$temporary/pluginkit"
pluginkit_log="$temporary/pluginkit.log"
pluginkit_mode="$temporary/pluginkit.mode"
pluginkit_remove_status="$temporary/pluginkit.remove-status"
mkdir -p "$home" "$account" "$appdir" "$prefix/bin" "$formula_records" "$step_temporary"
: >"$pluginkit_log"

# The account directory is deliberately not HOME: nothing may depend on HOME.
export AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account"

support="$account/Library/Application Support/AgentDeck"
receipt="$support/installation.json"
app="$appdir/AgentDeck.app"
final_helper="$app/Contents/Helpers/agentdeck"
extension="$app/Contents/PlugIns/AgentDeckWidget.appex"

# The stub answers a query from the mode file and records every call. Its
# output shapes are the ones observed on a GitHub-hosted macOS 26.7 runner and
# the ones the script must not depend on.
cat >"$pluginkit" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$pluginkit_log"
if [ "\$1" = -r ]; then
  exit "\$(cat "$pluginkit_remove_status")"
fi
case "\$(cat "$pluginkit_mode")" in
  registered) printf '     com.kitdine.agentdeck.widget(1.0)\n            Path = %s\n\n (1 plug-in)\n' "$extension" ;;
  other-path) printf '     com.kitdine.agentdeck.widget(1.0)\n            Path = %s\n\n (1 plug-in)\n' "$temporary/elsewhere/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex" ;;
  prefix-path) printf '            Path = %s.old\n' "$extension" ;;
  no-matches) printf '  (no matches)\n' ;;
  zero) printf '(0 plug-ins)\n' ;;
  empty) ;;
  query-fail) echo 'pluginkit: lookup failed' >&2; exit 9 ;;
esac
EOF
chmod 0755 "$pluginkit"
printf 'no-matches\n' >"$pluginkit_mode"
printf '0\n' >"$pluginkit_remove_status"

helper_build="$temporary/helper/agentdeck"
mkdir -p "$(dirname "$helper_build")"
(cd "$root" && "$go_bin" build -mod=vendor -trimpath -tags agentdeck_acceptance -o "$helper_build" ./cmd/agentdeck)
"$go_bin" tool nm "$helper_build" | grep -F 'installation.acceptanceAccountHome' >/dev/null || {
  echo "refusing to install a helper that would use the real account" >&2
  exit 1
}
# From here on HOME is the temporary one. It is set after the build because the
# Go toolchain keeps its own files below HOME.
export HOME="$home"

# --- fixtures -----------------------------------------------------------

# make_app <directory> <version>: an App whose helper is the acceptance build.
make_app() {
  local source_root=$1 version=$2
  local bundle="$source_root/AgentDeck.app"
  mkdir -p "$bundle/Contents/Helpers" "$bundle/Contents/Resources/completions" "$bundle/Contents/MacOS" \
    "$bundle/Contents/PlugIns/AgentDeckWidget.appex"
  cp "$helper_build" "$bundle/Contents/Helpers/agentdeck"
  printf '#!/bin/sh\nprintf "AgentDeck %%s\\n" "%s"\n' "$version" >"$bundle/Contents/MacOS/AgentDeck"
  chmod 0755 "$bundle/Contents/MacOS/AgentDeck"
  printf '%s\n' "$version" >"$bundle/Contents/Resources/version"
  printf '# bash completion %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.bash"
  printf '#compdef agentdeck %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.zsh"
  printf '# fish completion %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.fish"
  printf '%s\n' "$bundle"
}

# make_stub_app: for the real brew refusal only; its helper is never executed.
make_stub_app() {
  local source_root=$1 version=$2
  local bundle="$source_root/AgentDeck.app"
  mkdir -p "$bundle/Contents/Helpers" "$bundle/Contents/Resources/completions" "$bundle/Contents/MacOS"
  printf '#!/bin/sh\nexit 1\n' >"$bundle/Contents/MacOS/AgentDeck"
  printf '#!/bin/sh\nexit 1\n' >"$bundle/Contents/Helpers/agentdeck"
  chmod 0755 "$bundle/Contents/MacOS/AgentDeck" "$bundle/Contents/Helpers/agentdeck"
  local shell
  for shell in bash zsh fish; do
    printf '# %s completion %s\n' "$shell" "$version" >"$bundle/Contents/Resources/completions/agentdeck.$shell"
  done
  printf '%s\n' "$bundle"
}

render_cask() {
  local tag=$1 output=$2
  local checksums="$temporary/checksums-$tag.txt"
  printf '%s  AgentDeck_%s_universal.dmg\n' \
    cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc "$tag" >"$checksums"
  bash "$root/scripts/render-homebrew-cask.sh" \
    "$root/packaging/homebrew/agentdeck-app.rb.tmpl" "$tag" "$checksums" "$output"
}

fail() {
  echo "$*" >&2
  exit 1
}

expect_failure() {
  local message=$1
  shift
  if "$@" >"$temporary/failure.log" 2>&1; then
    cat "$temporary/failure.log" >&2
    fail "$message"
  fi
}

expect_output() {
  grep -F -- "$1" "$temporary/failure.log" >/dev/null || {
    cat "$temporary/failure.log" >&2
    fail "expected output: $1"
  }
}

reject_output() {
  if grep -F -- "$1" "$temporary/failure.log" >/dev/null; then
    cat "$temporary/failure.log" >&2
    fail "unexpected output: $1"
  fi
}

receipt_field() {
  sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p" "$receipt"
}

digest() {
  if [[ ! -e $1 ]]; then
    printf 'absent\n'
    return
  fi
  shasum -a 256 "$1" | awk '{ print $1 }'
}

# installed <helper>: what that program itself reports about its own identity.
installed() {
  "$1" --format json desktop state-location
}

# --- the cask's own declarations ------------------------------------------

cask_token() {
  awk -F'"' '/^cask "/ { print $2; exit }' "$1"
}

driver() {
  ruby "$root/scripts/cask-local-driver.rb" "$1" "$appdir" "$prefix" "$2"
}

cask_conflicting_formulae() {
  # Homebrew's conflicts_with accepts casks only, so the formula exclusion is
  # declared by the cask's preflight step. The driver evaluates that step's own
  # declaration; real_brew_install_refusal exercises it through brew.
  driver "$1" formulae
}

cask_binaries() {
  driver "$1" binaries
}

# read_argv <cask> <action>: the declared argv into the global `argv` array.
read_argv() {
  argv=()
  local item
  while IFS= read -r -d '' item; do
    argv+=("$item")
  done < <(driver "$1" "$2"; printf '\0')
}

# run_registration <cask> [helper]: the cask's registration step as declared,
# against the installed helper or a stand-in. Its temporary file must be gone
# when it returns, whatever the outcome.
run_registration() {
  read_argv "$1" postflight-argv
  [[ ${argv[0]} == /bin/sh && ${argv[1]} == -c && ${argv[3]} == agentdeck-cask-register ]] ||
    fail "the registration step is not the declared /bin/sh wrapper"
  [[ ${argv[4]} == "$final_helper" && ${argv[5]} == "$(cask_token "$1")" && ${#argv[@]} == 6 ]] ||
    fail "the registration step must receive the installed helper and the cask token"
  if [[ -n ${2:-} ]]; then
    argv[4]=$2
  fi
  local status=0
  TMPDIR="$step_temporary" "${argv[@]}" || status=$?
  if [[ -n $(find "$step_temporary" -mindepth 1 -print -quit) ]]; then
    fail "the registration step left a temporary file behind"
  fi
  return "$status"
}

# run_uninstall_script <cask>: the cask's uninstall script as declared, with
# the recording stub in place of the system tool.
run_uninstall_script() {
  read_argv "$1" uninstall-argv
  [[ ${argv[0]} == /bin/sh && ${argv[1]} == -c && ${argv[3]} == agentdeck-widget-uninstall ]] ||
    fail "the uninstall script is not the declared /bin/sh script"
  [[ ${argv[4]} == "$extension" && ${argv[5]} == com.kitdine.agentdeck.widget && ${#argv[@]} == 6 ]] ||
    fail "the uninstall script must receive this App's extension and the release widget identifier"
  local script=${argv[2]}
  [[ $(grep -c '/usr/bin/pluginkit' <<<"$script") == 2 ]] ||
    fail "the uninstall script must query once and unregister once"
  argv[2]=${script//\/usr\/bin\/pluginkit/$pluginkit}
  "${argv[@]}"
}

# --- a stand-in for Homebrew's file handling ------------------------------

# cask_install <cask> <source bundle>: the formula check, the collision check,
# a copy in place of the app artifact, the registration step, then the links.
# A failed registration removes the copy, as a failed install step makes
# Homebrew remove the artifacts it had installed.
cask_install() {
  local cask_file=$1 source_bundle=$2
  local token record
  token=$(cask_token "$cask_file")
  record="$temporary/installed-$token"

  local formula
  for formula in $(cask_conflicting_formulae "$cask_file"); do
    if [[ -e "$formula_records/$formula" ]]; then
      echo "cask $token conflicts with the installed formula $formula" >&2
      return 3
    fi
  done

  local source target
  while IFS=$'\t' read -r source target; do
    if [[ -e $target || -L $target ]]; then
      if [[ ! -e $record ]] || ! grep -Fxq "$target" "$record"; then
        echo "cask $token refuses to take over $target, which it does not own" >&2
        return 4
      fi
    fi
  done < <(cask_binaries "$cask_file")

  if [[ -e $app ]]; then
    echo "an App already exists at $app" >&2
    return 6
  fi
  ditto "$source_bundle" "$app"
  if ! run_registration "$cask_file"; then
    rm -rf "${app:?}"
    return 5
  fi
  : >"$record"
  while IFS=$'\t' read -r source target; do
    mkdir -p "$(dirname "$target")"
    ln -sfn "$source" "$target"
    printf '%s\n' "$target" >>"$record"
  done < <(cask_binaries "$cask_file")
}

# cask_uninstall <cask>: the uninstall script first, as Homebrew orders it, then
# the App and the links. A failing script aborts with everything in place.
cask_uninstall() {
  local cask_file=$1
  local token record
  token=$(cask_token "$cask_file")
  record="$temporary/installed-$token"
  run_uninstall_script "$cask_file" || return 5
  rm -rf "${app:?}"
  if [[ -e $record ]]; then
    local target
    while IFS= read -r target; do
      [[ $target == "$prefix"/* ]] || fail "refusing to unlink $target"
      unlink "$target"
    done <"$record"
    unlink "$record"
  fi
}

real_brew_install_refusal() {
  local source_root="$temporary/real-brew-source"
  local source_bundle
  source_bundle=$(make_stub_app "$source_root" 1.2.3)

  local archive="$temporary/AgentDeck_v1.2.3_universal.zip"
  ditto -c -k --sequesterRsrc --keepParent "$source_bundle" "$archive"

  local cask_file="$temporary/agentdeck-app-real-brew.rb"
  local checksums="$temporary/real-brew-checksums.txt"
  local archive_sha
  archive_sha=$(shasum -a 256 "$archive" | awk '{ print $1 }')
  printf '%s  AgentDeck_v1.2.3_universal.dmg\n' "$archive_sha" >"$checksums"
  bash "$root/scripts/render-homebrew-cask.sh" \
    "$root/packaging/homebrew/agentdeck-app.rb.tmpl" v1.2.3 "$checksums" "$cask_file"

  ruby -e '
    path, url = ARGV
    lines = File.readlines(path)
    start = lines.index { |line| line.start_with?("  url ") }
    abort "rendered cask has no url stanza" unless start
    count = lines[start].end_with?("\\\\\n") ? 2 : 1
    lines[start, count] = ["  url #{url.dump}\n"]
    File.write(path, lines.join)
  ' "$cask_file" "file://$archive"

  command -v brew >/dev/null || {
    echo "real Homebrew is required for the cask preflight regression" >&2
    exit 1
  }
  local brew_repository
  brew_repository=$(brew --repository)

  local isolated_prefix="$temporary/isolated-brew"
  local isolated_cellar="$isolated_prefix/Cellar"
  local isolated_brew="$isolated_prefix/bin/brew"
  local isolated_appdir="$temporary/real-brew-applications"
  local tap_cask="$isolated_prefix/Library/Taps/agentdeck/homebrew-test/Casks/agentdeck-app.rb"
  mkdir -p \
    "$isolated_prefix/bin" \
    "$isolated_prefix/Library/Taps/agentdeck/homebrew-test/Casks" \
    "$isolated_cellar/agentdeck/1.2.3" \
    "$isolated_appdir"
  cp "$brew_repository/bin/brew" "$isolated_brew"
  ln -s "$brew_repository/Library/Homebrew" "$isolated_prefix/Library/Homebrew"
  cp "$cask_file" "$tap_cask"

  local isolated_prefix_real
  isolated_prefix_real=$(cd "$isolated_prefix" && pwd -P)
  if [[ $($isolated_brew --prefix) != "$isolated_prefix_real" ]]; then
    echo "isolated brew escaped its temporary prefix" >&2
    exit 1
  fi

  local brew_output="$temporary/real-brew-install.log"
  if env -u HOMEBREW_DEVELOPER \
    HOME="$home" \
    HOMEBREW_CACHE="$temporary/brew-cache" \
    HOMEBREW_LOGS="$temporary/brew-logs" \
    HOMEBREW_TEMP="$temporary/brew-temp" \
    HOMEBREW_NO_ANALYTICS=1 \
    HOMEBREW_NO_AUTO_UPDATE=1 \
    HOMEBREW_NO_ENV_HINTS=1 \
    HOMEBREW_NO_INSTALL_CLEANUP=1 \
    HOMEBREW_NO_INSTALL_FROM_API=1 \
    "$isolated_brew" install --cask \
      --appdir="$isolated_appdir" agentdeck/test/agentdeck-app >"$brew_output" 2>&1; then
    echo "real brew installed the cask alongside the conflicting agentdeck formula" >&2
    exit 1
  fi

  if ! grep -F 'The CLI-only agentdeck formula is installed' "$brew_output" >/dev/null; then
    echo "real brew refusal omitted the formula migration message" >&2
    tail -40 "$brew_output" >&2
    exit 1
  fi
  if [[ ! -d $isolated_cellar/agentdeck/1.2.3 ]]; then
    echo "real brew refusal removed the conflicting formula" >&2
    exit 1
  fi

  local unexpected
  for unexpected in \
    "$isolated_prefix/Caskroom/agentdeck-app" \
    "$isolated_appdir/AgentDeck.app" \
    "$isolated_prefix/bin/agentdeck"; do
    if [[ -e $unexpected || -L $unexpected ]]; then
      echo "real brew refusal left an installed cask artifact: $unexpected" >&2
      exit 1
    fi
  done
  # The refusal happens before the App is published, so the registration step
  # never ran and the account has no receipt from it.
  if [[ -e $receipt ]]; then
    echo "real brew refusal reached the registration step" >&2
    exit 1
  fi
}

# --- 0. the rendered declaration ------------------------------------------

stable_cask="$temporary/agentdeck-app.rb"
render_cask v1.2.3 "$stable_cask"
upgrade_cask="$temporary/agentdeck-app-1.2.4.rb"
render_cask v1.2.4 "$upgrade_cask"

declaration=$(grep -v '^[[:space:]]*#' "$stable_cask")
# The withdrawn cask protocol, and anything that would read or redirect HOME.
for forbidden in 'attempt' 'repair' 'REPAIR' '--require-unowned' '--expected-installation' '--cleanup-manifest' \
  '--source' '--takeover' 'unregister' 'staged_path' 'Dir.home' 'ENV[' 'ENV.fetch' 'expand_path' \
  'delete:' 'rmdir:' 'pkgutil:' 'launchctl:' 'pluginkit -a' '--appdir'; do
  if grep -F -- "$forbidden" <<<"$declaration" >/dev/null; then
    fail "the rendered cask must not contain: $forbidden"
  fi
done
if grep -E '^[[:space:]]*(preflight|postflight|uninstall_preflight|uninstall_postflight) do' <<<"$declaration" >/dev/null; then
  fail "the rendered cask must not use a classic flight block"
fi
[[ $(driver "$stable_cask" apps) == AgentDeck.app ]] || fail "Homebrew must publish exactly AgentDeck.app"
[[ $(driver "$stable_cask" uninstall-quit) == com.kitdine.agentdeck ]] || fail "uninstall must quit the App first"
# The step may write the installation records and nothing else.
[[ $(driver "$stable_cask" postflight-options) == 'print_stdout=true
writable_base=:home
writable_paths=["Library/Application Support/AgentDeck"]' ]] || fail "the registration step's sandbox declarations changed"
# trash appears only in zap, and zap never names the App, AgentDeck state or
# the installation records: a stale receipt is how ownership is released.
if [[ $(grep -c 'trash' <<<"$declaration") != 1 ]] || ! grep -F 'zap trash: [' <<<"$declaration" >/dev/null; then
  fail "trash may only appear in the zap stanza"
fi
if awk '/zap trash: \[/, /\]/' "$stable_cask" | grep -E 'AgentDeck\.app|/\.agentdeck"|Application Support' >/dev/null; then
  fail "zap must not remove the App, AgentDeck state or installation records"
fi
# The uninstall script removes no file: Homebrew removes the App.
read_argv "$stable_cask" uninstall-argv
if grep -E '(^|[[:space:];&|(])(rm|rmdir|trash|unlink|mv|sudo)[[:space:]]' <<<"${argv[2]}" >/dev/null; then
  fail "the uninstall script must not remove files"
fi
# The registration step removes only its own temporary file.
read_argv "$stable_cask" postflight-argv
[[ $(grep -c '/bin/rm' <<<"${argv[2]}") == 1 ]] && grep -F "trap '/bin/rm -f \"\$errors\"' EXIT" <<<"${argv[2]}" >/dev/null ||
  fail "the registration step may remove only its own temporary file"

# --- 1. install, upgrade, uninstall --------------------------------------

first=$(make_app "$temporary/source-1.2.3" 1.2.3)
second=$(make_app "$temporary/source-1.2.4" 1.2.4)
cask_install "$stable_cask" "$first" >"$temporary/install.log"
# The helper's own stdout reaches the user unchanged.
grep -Fx "registered $final_helper (homebrew_cask)" "$temporary/install.log" >/dev/null

test -d "$app"
test -L "$prefix/bin/agentdeck"
[[ $(readlink "$prefix/bin/agentdeck") == "$final_helper" ]]
grep -Fx 1.2.3 "$app/Contents/Resources/version" >/dev/null
test -L "$prefix/etc/bash_completion.d/agentdeck"
test -L "$prefix/share/zsh/site-functions/_agentdeck"
test -L "$prefix/share/fish/vendor_completions.d/agentdeck.fish"
grep -F '# bash completion 1.2.3' "$prefix/etc/bash_completion.d/agentdeck" >/dev/null
grep -F '#compdef agentdeck 1.2.3' "$prefix/share/zsh/site-functions/_agentdeck" >/dev/null
grep -F '# fish completion 1.2.3' "$prefix/share/fish/vendor_completions.d/agentdeck.fish" >/dev/null

# The receipt names the App's helper, and every record is in the account
# directory. HOME is a different directory and stays untouched.
[[ $(receipt_field channel) == homebrew_cask ]]
[[ $(receipt_field executable) == "$final_helper" ]]
[[ $(stat -f '%Lp' "$receipt") == 600 ]]
first_id=$(receipt_field installation_id)
[[ $first_id =~ ^[0-9a-f]{32}$ ]]
if [[ -n $(find "$home" -mindepth 1 -print -quit) ]]; then
  find "$home" -mindepth 1 >&2; fail "the install wrote below HOME"
fi
# The cask keeps no installer materials of its own: the lock and the receipt.
[[ $(find "$support" -mindepth 1 | LC_ALL=C sort) == "$support/installation.json
$support/ownership.lock" ]] || { find "$support" >&2; fail "the cask left installer materials in the account"; }
installed "$final_helper" | grep -F '"installed":true,"channel":"homebrew_cask"' >/dev/null
"$final_helper" --format json doctor | grep -F '"reason":"homebrew_cask"' >/dev/null

# State the user owns, created between install and upgrade.
mkdir -p "$home/.agentdeck" "$home/.codex" "$home/.claude"
printf 'key\n' >"$home/.agentdeck/credential.key"
printf 'codex config\n' >"$home/.codex/config.toml"
printf 'claude settings\n' >"$home/.claude/settings.json"
printf 'shell rc\n' >"$home/.zshrc"

# A second install on top of the installed App is Homebrew's own refusal; it
# never reaches the registration step, so the receipt is untouched.
receipt_digest=$(digest "$receipt")
expect_failure "a second cask install replaced the installed App" cask_install "$upgrade_cask" "$second"
expect_output 'an App already exists'
[[ $(digest "$receipt") == "$receipt_digest" ]]
grep -Fx 1.2.3 "$app/Contents/Resources/version" >/dev/null

# An upgrade is the old version's uninstall followed by the new version's
# install. The extension was never registered here, which is not an error. The
# App is gone between the two, and the receipt stays and names nothing.
: >"$pluginkit_log"
cask_uninstall "$stable_cask"
test ! -e "$app"
[[ $(cat "$pluginkit_log") == "-m -A -D -vv -i com.kitdine.agentdeck.widget" ]] || {
  cat "$pluginkit_log" >&2; fail "an unregistered extension must be queried once and never unregistered"
}
[[ $(digest "$receipt") == "$receipt_digest" ]] || fail "uninstalling the cask changed the receipt"
cask_install "$upgrade_cask" "$second" >/dev/null
grep -Fx 1.2.4 "$app/Contents/Resources/version" >/dev/null
grep -F '# fish completion 1.2.4' "$prefix/share/fish/vendor_completions.d/agentdeck.fish" >/dev/null
# The step ran again on the same path: a new ID, the same owner.
second_id=$(receipt_field installation_id)
[[ $second_id =~ ^[0-9a-f]{32}$ && $second_id != "$first_id" ]]
[[ $(receipt_field executable) == "$final_helper" ]]
installed "$final_helper" | grep -F '"installed":true' >/dev/null
test -f "$home/.agentdeck/credential.key"

# Re-registration needs the ownership lock even on the same path. A running
# watch holds this installation's session, so the step is refused, the receipt
# is untouched, and the same step succeeds once the session ends. What Homebrew
# then does to an upgrade whose step failed is proven only in CI. The session
# names the account's state directory explicitly, because HOME is deliberately
# a different directory here and a session on HOME's state takes no lock.
receipt_digest=$(digest "$receipt")
"$final_helper" --state-dir "$account/.agentdeck" watch --domains usage >"$temporary/watch.log" 2>&1 &
watcher=$!
for _ in $(seq 1 100); do
  [[ -s $temporary/watch.log ]] && break
  kill -0 "$watcher" 2>/dev/null || break
  sleep 0.1
done
[[ -s $temporary/watch.log ]] || { kill "$watcher" 2>/dev/null || true; fail "the watch session never started"; }
held_status=0
run_registration "$upgrade_cask" >"$temporary/failure.log" 2>&1 || held_status=$?
kill -0 "$watcher" 2>/dev/null || { cat "$temporary/watch.log" "$temporary/failure.log" >&2; fail "the watch session ended before the registration step finished"; }
kill "$watcher"
wait "$watcher" 2>/dev/null || true
[[ $held_status == 1 ]] || { cat "$temporary/failure.log" >&2; fail "registration during a session: exit $held_status, want 1"; }
expect_output '  reason: owner_running'
expect_output 'stop any agentdeck watch'
[[ $(digest "$receipt") == "$receipt_digest" ]] || fail "a refused re-registration changed the receipt"
run_registration "$upgrade_cask" >/dev/null
[[ $(receipt_field installation_id) != "$second_id" && $(receipt_field executable) == "$final_helper" ]]

# Uninstall with the extension registered: it is unregistered by exact path
# before the App goes.
printf 'registered\n' >"$pluginkit_mode"
: >"$pluginkit_log"
receipt_digest=$(digest "$receipt")
cask_uninstall "$upgrade_cask"
[[ $(cat "$pluginkit_log") == "-m -A -D -vv -i com.kitdine.agentdeck.widget"$'\n'"-r $extension" ]] || {
  cat "$pluginkit_log" >&2; fail "a registered extension must be unregistered by its exact path"
}
printf 'no-matches\n' >"$pluginkit_mode"
test ! -e "$app"
test -d "$appdir"
test ! -e "$prefix/bin/agentdeck"
test ! -e "$prefix/etc/bash_completion.d/agentdeck"
test ! -e "$prefix/share/zsh/site-functions/_agentdeck"
test ! -e "$prefix/share/fish/vendor_completions.d/agentdeck.fish"
# The uninstall boundary: app-owned artifacts only.
test -f "$home/.agentdeck/credential.key"
test -f "$home/.codex/config.toml"
test -f "$home/.claude/settings.json"
test -f "$home/.zshrc"
# The receipt is left in place and names a program that is gone. Any program
# reads that as "no owner", and another installation takes ownership without a
# takeover.
[[ $(digest "$receipt") == "$receipt_digest" ]] || fail "uninstalling the cask changed the receipt"
retained="$temporary/retained/agentdeck"
mkdir -p "$(dirname "$retained")"
cp "$helper_build" "$retained"
installed "$retained" | grep -F '"installed":false,"reason":"receipt_missing"' >/dev/null
successor="$temporary/successor/bin/agentdeck"
mkdir -p "$(dirname "$successor")"
cp "$helper_build" "$successor"
"$successor" install register --channel make --executable "$successor" >/dev/null
[[ $(receipt_field channel) == make && $(receipt_field executable) == "$successor" ]]
installed "$successor" | grep -F '"installed":true,"channel":"make"' >/dev/null

# --- 2. another valid owner refuses the cask ------------------------------

# The successor above is a live owner at another path. The cask is refused, the
# user is told how to migrate in words a cask user can follow, and the copy
# Homebrew published is removed again.
receipt_digest=$(digest "$receipt")
expect_failure "the cask replaced another valid installation" cask_install "$stable_cask" "$first"
expect_output 'install_claim_refused: Another AgentDeck installation owns AgentDeck state.'
expect_output '  reason: owned_by_other'
expect_output '  next: Keep the current installation, or uninstall it with its original installation method, then retry this Homebrew operation for agentdeck-app.'
reject_output 'takeover'
[[ $(wc -l <"$temporary/failure.log" | tr -d ' ') == 3 ]] || { cat "$temporary/failure.log" >&2; fail "the refusal must be exactly the three migration lines"; }
[[ $(digest "$receipt") == "$receipt_digest" ]] || fail "a refused cask changed the receipt"
test ! -e "$app"
test ! -e "$prefix/bin/agentdeck"
# Same path, another installation method: still another owner.
ditto "$first" "$app"
"$final_helper" install register --channel local_app --executable "$final_helper" --takeover >/dev/null
receipt_digest=$(digest "$receipt")
if run_registration "$stable_cask" >"$temporary/failure.log" 2>&1; then
  fail "the cask re-registered an App that another installation method owns"
fi
expect_output '  reason: owned_by_other'
[[ $(digest "$receipt") == "$receipt_digest" ]] || fail "a refused cask changed the receipt"
rm -rf "${app:?}" "${temporary:?}/successor"

# --- 3. the registration wrapper, against stand-in helpers ----------------

# The wrapper changes one thing: the stderr of an owned_by_other refusal. These
# stand-ins take the helper's place, so they prove the wrapper and nothing about
# the helper or about Homebrew's step sandbox.
standin="$temporary/stand in/agentdeck"
mkdir -p "$(dirname "$standin")"
wrapper_case() {
  local name=$1 want_status=$2 body=$3
  printf '#!/bin/sh\n%s\n' "$body" >"$standin"
  chmod 0755 "$standin"
  local status=0
  run_registration "$stable_cask" "$standin" >"$temporary/wrapper.out" 2>"$temporary/wrapper.err" || status=$?
  [[ $status == "$want_status" ]] || fail "wrapper $name: exit $status, want $want_status"
}
wrapper_case success 0 'printf "argv=%s|%s|%s|%s|%s|%s|%s|%s\n" "$@"; echo note >&2'
[[ $(cat "$temporary/wrapper.out") == "argv=--format|text|install|register|--channel|homebrew_cask|--executable|$standin" ]] ||
  fail "wrapper success: the helper must receive the declared arguments and a path with spaces intact"
[[ $(cat "$temporary/wrapper.err") == note ]] || fail "wrapper success: stderr must pass through"
wrapper_case owned_by_other 7 'echo before; printf "install_claim_refused: x\n  reason: owned_by_other\n  next: rerun with --takeover\n" >&2; exit 7'
[[ $(cat "$temporary/wrapper.out") == before ]] || fail "wrapper owned_by_other: stdout must pass through"
[[ $(cat "$temporary/wrapper.err") == "install_claim_refused: Another AgentDeck installation owns AgentDeck state.
  reason: owned_by_other
  next: Keep the current installation, or uninstall it with its original installation method, then retry this Homebrew operation for agentdeck-app." ]] ||
  fail "wrapper owned_by_other: the migration text changed"
wrapper_case owner_running 8 'printf "install_claim_refused: y\n  reason: owner_running\n  next: quit\n" >&2; exit 8'
[[ $(cat "$temporary/wrapper.err") == "install_claim_refused: y
  reason: owner_running
  next: quit" ]] || fail "wrapper owner_running: stderr must pass through unchanged"
wrapper_case other_error 19 'echo "chmod: operation not permitted" >&2; exit 19'
[[ $(cat "$temporary/wrapper.err") == "chmod: operation not permitted" ]] || fail "wrapper other_error: stderr must pass through unchanged"
# Only the whole line counts: a prefix, a suffix or other indentation does not.
wrapper_case near_miss 23 'printf "reason: owned_by_other\n  reason: owned_by_other_path\n   reason: owned_by_other\n" >&2; exit 23'
[[ $(cat "$temporary/wrapper.err") == "reason: owned_by_other
  reason: owned_by_other_path
   reason: owned_by_other" ]] || fail "wrapper near_miss: a partial match must not be rewritten"
# The line on a successful run is the helper's business, not a refusal.
wrapper_case success_with_reason_line 0 'printf "  reason: owned_by_other\n" >&2'
[[ $(cat "$temporary/wrapper.err") == "  reason: owned_by_other" ]] || fail "wrapper success_with_reason_line: a successful run must not be rewritten"

# --- 4. the uninstall script, against the recording stub ------------------

uninstall_case() {
  local name=$1 mode=$2 want_status=$3 want_log=$4
  printf '%s\n' "$mode" >"$pluginkit_mode"
  : >"$pluginkit_log"
  local status=0
  run_uninstall_script "$stable_cask" >/dev/null 2>"$temporary/uninstall.err" || status=$?
  [[ $status == "$want_status" ]] || fail "uninstall $name: exit $status, want $want_status"
  [[ $(cat "$pluginkit_log") == "$want_log" ]] || { cat "$pluginkit_log" >&2; fail "uninstall $name: unexpected pluginkit calls"; }
}
query="-m -A -D -vv -i com.kitdine.agentdeck.widget"
uninstall_case no_matches no-matches 0 "$query"
uninstall_case empty empty 0 "$query"
uninstall_case zero zero 0 "$query"
uninstall_case other_path other-path 0 "$query"
uninstall_case prefix_path prefix-path 0 "$query"
# A failed query is never read as "not registered".
uninstall_case query_fail query-fail 9 "$query"
grep -F 'lookup failed' "$temporary/uninstall.err" >/dev/null
# The unregister status is the script's status.
printf '7\n' >"$pluginkit_remove_status"
uninstall_case registered_remove_fails registered 7 "$query"$'\n'"-r $extension"
printf '0\n' >"$pluginkit_remove_status"
uninstall_case registered registered 0 "$query"$'\n'"-r $extension"
printf 'no-matches\n' >"$pluginkit_mode"

# The real Homebrew path owns the preflight control-flow contract. In
# particular, a refusal must unwind the Caskroom receipt as well as leave the
# app and command absent; checking only stderr and the process status misses the
# partial installation caused by an exit that bypasses Homebrew's rollback.
unlink "$receipt"
if [[ $local_only == 0 ]]; then
  real_brew_install_refusal
else
  printf '%s\n' 'Local-only: real Homebrew conflict refusal not executed.'
fi

# --- 5. mutual exclusion with the CLI-only formula -----------------------

# The exclusion is read out of the cask, so the read itself must be asserted.
# Planting one formula and watching the install refuse would stay green under a
# parser that silently lost the other, which is the unfalsifiable shape this
# suite is not allowed to keep: the lost channel would then install beside the
# cask and both would claim the `agentdeck` command.
declared_formulae=$(cask_conflicting_formulae "$stable_cask" | sort | tr '\n' ' ')
if [[ $declared_formulae != "agentdeck agentdeck-rc " ]]; then
  fail "cask must exclude both CLI formulae, read: [$declared_formulae]"
fi

# Each declared formula is then planted on its own, because the refusal has to
# hold per channel rather than only for whichever one happens to be listed first.
for conflicting in agentdeck agentdeck-rc; do
  : >"$formula_records/$conflicting"
  if cask_install "$stable_cask" "$first" 2>/dev/null; then
    fail "cask installed alongside the conflicting $conflicting formula"
  fi
  test ! -e "$app"
  test ! -e "$prefix/bin/agentdeck"
  unlink "$formula_records/$conflicting"
done

# --- 6. the documented migration path --------------------------------------

migration=$(awk '/brew uninstall agentdeck/, /brew install --cask agentdeck-app/' "$stable_cask")
grep -F 'brew uninstall agentdeck' <<<"$migration" >/dev/null
grep -F 'brew install --cask agentdeck-app' <<<"$migration" >/dev/null

# A claimed formula whose keg is still present owns state. With the formula
# record gone the conflict step no longer catches it, so the registration step
# is what refuses, and the keg's receipt is untouched.
keg_root="$temporary/Cellar"
keg="$keg_root/agentdeck/1.0.0/bin/agentdeck"
mkdir -p "$(dirname "$keg")"
cp "$helper_build" "$keg"
"$keg" install register --channel homebrew_formula --executable "$keg" >/dev/null
[[ $(receipt_field channel) == homebrew_formula ]]
receipt_digest=$(digest "$receipt")
expect_failure "the cask replaced a claimed formula that is still installed" cask_install "$stable_cask" "$first"
expect_output '  reason: owned_by_other'
reject_output 'takeover'
[[ $(digest "$receipt") == "$receipt_digest" ]]
test ! -e "$app"
# Uninstalling the formula deletes the keg. Its receipt is then stale, which is
# unowned: the cask installs with no takeover, and replaces the receipt.
rm -rf "${keg_root:?}"
cask_install "$stable_cask" "$first" >/dev/null
[[ $(receipt_field channel) == homebrew_cask ]]
grep -Fx 1.2.3 "$app/Contents/Resources/version" >/dev/null
test -f "$home/.agentdeck/credential.key"
cask_uninstall "$stable_cask"

# --- 7. a foreign owner of the command is never taken over ----------------

# The legacy script installer owns ~/.local/bin/agentdeck; a cask target that is
# already someone else's file must refuse rather than relink it.
mkdir -p "$prefix/bin"
printf '#!/bin/sh\necho legacy\n' >"$prefix/bin/agentdeck"
chmod 0755 "$prefix/bin/agentdeck"
if cask_install "$stable_cask" "$first" 2>/dev/null; then
  fail "cask took over a command it does not own"
fi
grep -F legacy "$prefix/bin/agentdeck" >/dev/null
test ! -e "$app"
unlink "$prefix/bin/agentdeck"

# The CLI-only installer keeps its own coverage in scripts/test-install.sh and
# scripts/test-completion-install.sh; it is not re-tested here, because what
# this file owns is the boundary between the two installations rather than
# either installation on its own.

# Homebrew persists its own tap trust decision in the temporary HOME. Allow
# only those two files; AgentDeck installation materials still belong solely
# to the separate account directory.
if [[ -d $home/.homebrew ]] && [[ -n $(find "$home/.homebrew" -mindepth 1 ! -path "$home/.homebrew/trust.json" ! -path "$home/.homebrew/trust.json.lock" -print -quit) ]]; then
  fail "unexpected Homebrew configuration below HOME"
fi
if [[ -n $(find "$home" -mindepth 1 -maxdepth 1 ! -name .agentdeck ! -name .codex ! -name .claude ! -name .zshrc ! -name .homebrew -print -quit) ]]; then
  find "$home" -mindepth 1 -maxdepth 4 ! -path "$home/.agentdeck/*" -print >&2
  fail "something was written below HOME"
fi

printf 'cask installation protocol, migration and mutual exclusion: PASS\n'
