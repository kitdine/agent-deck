#!/usr/bin/env bash
set -euo pipefail

# The cask's installation protocol, formula-to-cask migration, mutual exclusion
# and the uninstall boundary, all in a temporary HOME, a temporary account
# directory, a temporary application directory, and a temporary prefix.
#
# Boundary, stated rather than implied:
#
# - The publishing preflight block, the rollback block and the uninstall script
#   are the rendered cask's own code, run by scripts/cask-local-driver.rb. The
#   helper they call is an agentdeck_acceptance build placed in a temporary
#   staged directory by this script; its receipt, ownership lock and cleanup
#   materials live in the temporary account. PlugInKit is a recording stub.
# - No program staged by Homebrew is ever executed here. On a developer
#   machine that triggers Gatekeeper. The complete Homebrew dispatch runs only
#   in CI, in scripts/test-cask-real-brew.sh.
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
caskroom="$temporary/Caskroom"
formula_records="$temporary/formula-records"
pluginkit_log="$temporary/pluginkit.log"
mkdir -p "$home" "$account" "$appdir" "$prefix/bin" "$formula_records"
: >"$pluginkit_log"

# The account directory is deliberately not HOME: nothing may depend on HOME.
export AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account"
export AGENTDECK_ACCEPTANCE_PLUGINKIT="$temporary/pluginkit"
cat >"$AGENTDECK_ACCEPTANCE_PLUGINKIT" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$pluginkit_log"
if [ "\$1" = -m ]; then
  printf '    Path = %s\n (1 plug-in)\n' "$appdir/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex"
fi
EOF
chmod 0755 "$AGENTDECK_ACCEPTANCE_PLUGINKIT"
unset HOMEBREW_AGENTDECK_REPAIR HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION

support="$account/Library/Application Support/AgentDeck"
receipt="$support/installation.json"
attempts="$support/installers/homebrew_cask/attempts"
app="$appdir/AgentDeck.app"
final_helper="$app/Contents/Helpers/agentdeck"

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

receipt_field() {
  sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p" "$receipt"
}

# tree_digest <path>: names, types, modes, sizes and content below a path.
tree_digest() {
  if [[ ! -e $1 && ! -L $1 ]]; then
    printf 'absent\n'
    return
  fi
  {
    find "$1" -exec stat -f '%N %HT %Lp %z' {} + | LC_ALL=C sort
    find "$1" -type f -exec shasum -a 256 {} + | LC_ALL=C sort
  } | shasum -a 256 | awk '{ print $1 }'
}

unchanged() {
  [[ $(tree_digest "$app") == "$app_digest" && $(tree_digest "$receipt") == "$receipt_digest" ]] ||
    fail "the App or the receipt changed: $1"
}

attempt_count() {
  if [[ -d $attempts ]]; then
    find "$attempts" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' '
  else
    printf '0\n'
  fi
}

latest_attempt() {
  sed -n 's/.*"attempt_id":"\([0-9a-f]*\)".*/\1/p' "$1/.agentdeck-attempt"
}

no_staged_copy() {
  if [[ -n $(find "$appdir" -mindepth 1 -maxdepth 1 ! -name AgentDeck.app -print -quit) ]]; then
    fail "a staged or replaced copy was left in the application directory: $1"
  fi
}

# --- a local installer driven by the cask's own declarations -------------

cask_token() {
  awk -F'"' '/^cask "/ { print $2; exit }' "$1"
}

cask_version() {
  awk -F'"' '/^  version "/ { print $2; exit }' "$1"
}

staged_path_of() {
  printf '%s/%s/%s\n' "$caskroom" "$(cask_token "$1")" "$(cask_version "$1")"
}

# purge_staged <cask>: what Homebrew does to a staged directory after a failed
# or finished operation. The path is always below this test's Caskroom.
purge_staged() {
  local staged
  staged=$(staged_path_of "$1")
  [[ $staged == "$caskroom"/*/* ]] || fail "refusing to purge $staged"
  rm -rf "${staged:?}"
}

driver() {
  local cask_file=$1 action=$2
  ruby "$root/scripts/cask-local-driver.rb" "$cask_file" "$(staged_path_of "$cask_file")" "$appdir" "$prefix" "$action"
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

# cask_install <cask> <source bundle> [action]: stage the bundle as Homebrew
# would, then run the cask's formula check, publishing block and binary links.
cask_install() {
  local cask_file=$1 source_bundle=$2 action=${3:-install}
  local token staged record
  token=$(cask_token "$cask_file")
  staged=$(staged_path_of "$cask_file")
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

  # Homebrew stages the download. A staged directory that already exists is one
  # it restored from backup, so its locator is kept.
  if [[ ! -d $staged ]]; then
    mkdir -p "$staged"
    ditto "$source_bundle" "$staged/AgentDeck.app"
  fi
  if ! driver "$cask_file" "$action"; then
    return 5
  fi
  [[ $action == install ]] || return 0
  : >"$record"
  while IFS=$'\t' read -r source target; do
    mkdir -p "$(dirname "$target")"
    ln -sfn "$source" "$target"
    printf '%s\n' "$target" >>"$record"
  done < <(cask_binaries "$cask_file")
}

# cask_uninstall <cask>: the uninstall script first, as Homebrew orders it, then
# the link removal and the purge of the staged directory. A failing script
# aborts the uninstall with everything else in place.
cask_uninstall() {
  local cask_file=$1
  local token record
  token=$(cask_token "$cask_file")
  record="$temporary/installed-$token"
  driver "$cask_file" uninstall || return 5
  if [[ -e $record ]]; then
    local target
    while IFS= read -r target; do
      [[ $target == "$prefix"/* ]] || fail "refusing to unlink $target"
      unlink "$target"
    done <"$record"
    unlink "$record"
  fi
  purge_staged "$cask_file"
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

  # The publishing blocks are classic flight blocks, which Homebrew refuses to
  # load in developer mode; the regression runs as an ordinary user does.
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
  # The refusal happens before the publishing block, so no helper ran and the
  # account has no receipt and no attempt from it.
  if [[ -e $receipt || $(attempt_count) != "$1" ]]; then
    echo "real brew refusal reached the publishing block" >&2
    exit 1
  fi
}

# --- 0. the rendered declaration ------------------------------------------

stable_cask="$temporary/agentdeck-app.rb"
render_cask v1.2.3 "$stable_cask"
upgrade_cask="$temporary/agentdeck-app-1.2.4.rb"
render_cask v1.2.4 "$upgrade_cask"

declaration=$(grep -v '^[[:space:]]*#' "$stable_cask")
for forbidden in 'writable_paths' 'writable_base' 'Dir.home' 'ENV["HOME"]' 'ENV.fetch("HOME"' \
  'expand_path' '--takeover' 'Application Support' 'delete:' 'rmdir:' 'pkgutil:' 'launchctl:'; do
  if grep -F -- "$forbidden" <<<"$declaration" >/dev/null; then
    fail "the rendered cask must not contain: $forbidden"
  fi
done
if grep -E '^[[:space:]]*app "' <<<"$declaration" >/dev/null; then
  fail "the rendered cask must not publish the App with an app stanza"
fi
# trash appears only in zap, and zap never names the App or AgentDeck state.
if [[ $(grep -c 'trash' <<<"$declaration") != 1 ]] || ! grep -F 'zap trash: [' <<<"$declaration" >/dev/null; then
  fail "trash may only appear in the zap stanza"
fi
if awk '/zap trash: \[/, /\]/' "$stable_cask" | grep -E 'AgentDeck\.app|/\.agentdeck"|Application Support' >/dev/null; then
  fail "zap must not remove the App, AgentDeck state or installation records"
fi
# The uninstall script leaves every removal to the helper.
uninstall_shell=$(driver "$stable_cask" uninstall-argv | tr '\0' '\n' | sed -n '3,/^agentdeck-cask-uninstall$/p')
if grep -E '(^|[[:space:];&|(])(rm|rmdir|trash|unlink|mv|pluginkit|sudo)[[:space:]]' <<<"$uninstall_shell" >/dev/null; then
  fail "the uninstall script must leave every removal to the helper"
fi
grep -F 'install attempt cleanup' <<<"$uninstall_shell" >/dev/null
# --authorized-repair is produced in one place, by the user's variable.
if [[ $(grep -c -- '<< "--authorized-repair"' <<<"$declaration") != 1 ]] ||
  ! grep -F 'repair_arguments << "--authorized-repair" if ENV["HOMEBREW_AGENTDECK_REPAIR"] == "1"' <<<"$declaration" >/dev/null; then
  fail "--authorized-repair must come only from HOMEBREW_AGENTDECK_REPAIR"
fi
# Both cleanup calls made by the cask's Ruby name one attempt and carry no
# repair arguments. The only use of the repair arguments is register's and the
# uninstall script's.
if [[ $(grep -c '"install", "attempt", "cleanup", "--attempt"' <<<"$declaration") != 2 ]]; then
  fail "the publishing block and the rollback block must each clean up one named attempt"
fi
if grep -A1 '"install", "attempt", "cleanup", "--attempt"' <<<"$declaration" | grep -F 'repair_arguments' >/dev/null; then
  fail "a rollback cleanup must never carry repair arguments"
fi
if [[ $(grep -c '\*repair_arguments' <<<"$declaration") != 2 ]]; then
  fail "repair arguments may only reach register and the uninstall script"
fi

# --- 1. install, upgrade, uninstall --------------------------------------

first=$(make_app "$temporary/source-1.2.3" 1.2.3)
second=$(make_app "$temporary/source-1.2.4" 1.2.4)
cask_install "$stable_cask" "$first"

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
first_attempt=$(latest_attempt "$(staged_path_of "$stable_cask")")
[[ $first_attempt =~ ^[0-9a-f]{32}$ ]]
first_materials="$attempts/$first_attempt"
[[ $(stat -f '%Lp' "$first_materials") == 700 ]]
[[ $(stat -f '%Lp' "$first_materials/manifest.json") == 600 ]]
test -x "$first_materials/helper"
test -f "$first_materials/attempt.json"
test -f "$first_materials/published"
grep -F "\"installation_id\":\"$(receipt_field installation_id)\"" "$first_materials/manifest.json" >/dev/null
grep -F "\"remove_root\":\"$app\"" "$first_materials/manifest.json" >/dev/null
if [[ -n $(find "$home" -mindepth 1 -print -quit) ]]; then
  find "$home" -mindepth 1 >&2; fail "the install wrote below HOME"
fi
no_staged_copy "first install"
"$final_helper" --format json doctor | grep -F '"reason":"homebrew_cask"' >/dev/null

# State the user owns, created between install and upgrade.
mkdir -p "$home/.agentdeck" "$home/.codex" "$home/.claude"
printf 'state\n' >"$home/.agentdeck/agentdeck.sqlite3"
printf 'key\n' >"$home/.agentdeck/credential.key"
printf 'codex config\n' >"$home/.codex/config.toml"
printf 'claude settings\n' >"$home/.claude/settings.json"
printf 'shell rc\n' >"$home/.zshrc"

# An upgrade is the old version's uninstall followed by the new version's
# install. A second install on top of a valid owner is refused instead, and
# the refusal names the migration.
expect_failure "a second cask install replaced a valid installation" cask_install "$upgrade_cask" "$second"
expect_output 'reason: owned_by_other'
expect_output 'brew uninstall agentdeck'
grep -Fx 1.2.3 "$app/Contents/Resources/version" >/dev/null
no_staged_copy "refused install"
purge_staged "$upgrade_cask"
first_materials_digest=$(tree_digest "$first_materials")

: >"$pluginkit_log"
cask_uninstall "$stable_cask"
test ! -e "$app"
test ! -e "$receipt"
# The widget is unregistered by the helper, for exactly this App's extension.
[[ $(cat "$pluginkit_log") == "-m -A -D -vv"$'\n'"-r $app/Contents/PlugIns/AgentDeckWidget.appex" ]]
test -f "$first_materials/cleaned"
cask_install "$upgrade_cask" "$second"
grep -Fx 1.2.4 "$app/Contents/Resources/version" >/dev/null
grep -F '# fish completion 1.2.4' "$prefix/share/fish/vendor_completions.d/agentdeck.fish" >/dev/null
test -f "$home/.agentdeck/agentdeck.sqlite3"
second_attempt=$(latest_attempt "$(staged_path_of "$upgrade_cask")")
[[ $second_attempt != "$first_attempt" ]]
# The earlier attempt's immutable materials are kept, apart from its marker.
unlink "$first_materials/cleaned"
[[ $(tree_digest "$first_materials") == "$first_materials_digest" ]]

cask_uninstall "$upgrade_cask"
test ! -e "$app"
test ! -e "$receipt"
test -d "$appdir"
test ! -e "$prefix/bin/agentdeck"
test ! -e "$prefix/etc/bash_completion.d/agentdeck"
test ! -e "$prefix/share/zsh/site-functions/_agentdeck"
test ! -e "$prefix/share/fish/vendor_completions.d/agentdeck.fish"
# The uninstall boundary: app-owned artifacts only.
test -f "$home/.agentdeck/agentdeck.sqlite3"
test -f "$home/.agentdeck/credential.key"
test -f "$home/.codex/config.toml"
test -f "$home/.claude/settings.json"
test -f "$home/.zshrc"
# Uninstalled materials stay for delayed cleanup and recovery.
test -d "$first_materials"
test -d "$attempts/$second_attempt"

# The real Homebrew path owns the preflight control-flow contract. In
# particular, a refusal must unwind the Caskroom receipt as well as leave the
# app and command absent; checking only stderr and the process status misses the
# partial installation caused by an exit that bypasses Homebrew's rollback.
if [[ $local_only == 0 ]]; then
  real_brew_install_refusal "$(attempt_count)"
else
  printf '%s\n' 'Local-only: real Homebrew conflict refusal not executed.'
fi

# --- 2. mutual exclusion with the CLI-only formula -----------------------

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

# Left planted for section 3, which removes it as the first step of the
# documented migration.
: >"$formula_records/agentdeck"
if cask_install "$stable_cask" "$first" 2>/dev/null; then
  fail "cask installed alongside the conflicting formula"
fi
test ! -e "$app"
test ! -e "$prefix/bin/agentdeck"

# --- 3. the documented migration path --------------------------------------

migration=$(awk '/brew uninstall agentdeck/, /brew install --cask agentdeck-app/' "$stable_cask")
grep -F 'brew uninstall agentdeck' <<<"$migration" >/dev/null
grep -F 'brew install --cask agentdeck-app' <<<"$migration" >/dev/null
unlink "$formula_records/agentdeck"

# A claimed formula whose keg is still present owns state: the cask refuses and
# names the migration, and the keg's receipt is untouched.
keg_root="$temporary/Cellar"
keg="$keg_root/agentdeck/1.0.0/bin/agentdeck"
mkdir -p "$(dirname "$keg")"
cp "$helper_build" "$keg"
"$keg" install register --channel homebrew_formula --executable "$keg" >/dev/null
[[ $(receipt_field channel) == homebrew_formula ]]
receipt_digest=$(tree_digest "$receipt")
expect_failure "the cask replaced a claimed formula that is still installed" cask_install "$stable_cask" "$first"
expect_output 'reason: owned_by_other'
expect_output 'brew uninstall agentdeck'
[[ $(tree_digest "$receipt") == "$receipt_digest" ]]
test ! -e "$app"
no_staged_copy "formula refusal"
purge_staged "$stable_cask"
# Uninstalling the formula deletes the keg. Its receipt is then stale, which is
# unowned: the cask installs with no takeover and no repair variable, and
# replaces the receipt.
rm -rf "${keg_root:?}"
cask_install "$stable_cask" "$first"
[[ $(receipt_field channel) == homebrew_cask ]]
grep -Fx 1.2.3 "$app/Contents/Resources/version" >/dev/null
test -f "$home/.agentdeck/agentdeck.sqlite3"
cask_uninstall "$stable_cask"

# --- 4. a foreign owner of the command is never taken over ----------------

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

# --- 5. conditional cleanup ------------------------------------------------

# A failure in a later artifact rolls back exactly this attempt.
before=$(attempt_count)
cask_install "$stable_cask" "$first" install-rollback
test ! -e "$app"
test ! -e "$receipt"
[[ $(attempt_count) == $((before + 1)) ]]
rolled_back=$(latest_attempt "$(staged_path_of "$stable_cask")")
test -f "$attempts/$rolled_back/cleaned"
purge_staged "$stable_cask"

# An older attempt's delayed cleanup never removes a newer installation of the
# same token, path and version: only the installation ID differs.
cask_install "$stable_cask" "$first"
current_id=$(receipt_field installation_id)
"$attempts/$rolled_back/helper" install attempt cleanup --attempt "$rolled_back" --token agentdeck-app \
  --version 1.2.3 --executable "$final_helper" >"$temporary/delayed.log"
grep -F 'skipped superseded' "$temporary/delayed.log" >/dev/null
[[ $(receipt_field installation_id) == "$current_id" ]]
test -d "$app"

# A destroyed App helper does not stop the uninstall: the staged or retained
# helper runs the same protocol.
printf 'corrupt\n' >"$final_helper"
chmod 0644 "$final_helper"
cask_uninstall "$stable_cask"
test ! -e "$app"
test ! -e "$receipt"

# --- 6. missing cleanup materials are never a reason to delete -------------

for lost in manifest helper locator; do
  cask_install "$stable_cask" "$first"
  staged=$(staged_path_of "$stable_cask")
  attempt=$(latest_attempt "$staged")
  [[ $attempt =~ ^[0-9a-f]{32}$ ]]
  original_id=$(receipt_field installation_id)
  case $lost in
    manifest) unlink "$attempts/$attempt/manifest.json" ;;
    helper) unlink "$attempts/$attempt/helper" ;;
    locator) unlink "$staged/.agentdeck-attempt" ;;
  esac
  app_digest=$(tree_digest "$app")
  receipt_digest=$(tree_digest "$receipt")
  remaining_digest=$(tree_digest "$attempts/$attempt")
  locator_backup="$temporary/locator-backup"
  restore_locator() {
    # Homebrew restores the staged directory of the version it failed to replace.
    if [[ $lost == locator ]]; then
      [[ ! -e $staged/.agentdeck-attempt ]] || unlink "$staged/.agentdeck-attempt"
    else
      cp "$locator_backup" "$staged/.agentdeck-attempt"
    fi
  }
  [[ $lost == locator ]] || cp "$staged/.agentdeck-attempt" "$locator_backup"

  # Without authorization the uninstall fails, keeps everything, and prints
  # the identity-bound repair command for this cask and this helper.
  expect_failure "uninstall succeeded without the $lost" cask_uninstall "$stable_cask"
  expect_output 'detail: cleanup_materials_unavailable'
  expect_output "install repair-identity --channel homebrew_cask --executable '$final_helper'"
  expect_output 'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION="$agentdeck_repair_installation" brew reinstall --cask agentdeck-app'
  unchanged "unauthorized uninstall ($lost)"
  test -L "$prefix/bin/agentdeck"
  expect_failure "an unauthorized install replaced a valid installation" driver "$stable_cask" install
  expect_output 'reason: owned_by_other'
  unchanged "unauthorized install ($lost)"
  restore_locator

  # Authorization alone tolerates the uninstall but cannot replace the owner.
  HOMEBREW_AGENTDECK_REPAIR=1 driver "$stable_cask" uninstall >"$temporary/failure.log" 2>&1
  expect_output 'skipped unverified'
  expect_output 'still installed'
  unchanged "authorized unbound uninstall ($lost)"
  export HOMEBREW_AGENTDECK_REPAIR=1
  expect_failure "authorization alone replaced a valid installation" driver "$stable_cask" install
  unset HOMEBREW_AGENTDECK_REPAIR
  expect_output 'reason: owned_by_other'
  unchanged "authorized unbound install ($lost)"
  restore_locator

  # A value that is not a captured ID is refused, never read as "unbound";
  # neither is an ID without the authorization, or another installation's ID.
  for bad in '' 'not-an-id' "${original_id:0:31}" 0123456789abcdef0123456789abcdef; do
    export HOMEBREW_AGENTDECK_REPAIR=1 HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION="$bad"
    expect_failure "expected installation '$bad' was accepted by uninstall" driver "$stable_cask" uninstall
    expect_failure "expected installation '$bad' was accepted by install" driver "$stable_cask" install
    unset HOMEBREW_AGENTDECK_REPAIR HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION
    unchanged "expected installation '$bad' ($lost)"
    restore_locator
  done
  export HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION="$original_id"
  expect_failure "an expected installation without authorization was accepted" driver "$stable_cask" uninstall
  expect_failure "an expected installation without authorization was accepted" driver "$stable_cask" install
  unset HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION
  unchanged "expected installation without authorization ($lost)"
  restore_locator
  no_staged_copy "refused repairs ($lost)"

  # The printed command's first half: the App helper captures its own ID and
  # writes nothing.
  captured=$("$final_helper" install repair-identity --channel homebrew_cask --executable "$final_helper")
  [[ $captured == "$original_id" ]]
  unchanged "identity capture ($lost)"
  [[ $(tree_digest "$attempts/$attempt") == "$remaining_digest" ]]

  # Its second half, as brew reinstall orders it: uninstall, then install.
  export HOMEBREW_AGENTDECK_REPAIR=1 HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION="$captured"
  driver "$stable_cask" uninstall >"$temporary/failure.log" 2>&1
  expect_output 'skipped unverified'
  unchanged "identity-bound uninstall ($lost)"
  driver "$stable_cask" install
  unset HOMEBREW_AGENTDECK_REPAIR HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION
  repaired=$(latest_attempt "$staged")
  [[ $repaired != "$attempt" && $(receipt_field installation_id) != "$original_id" ]]
  [[ $(tree_digest "$attempts/$attempt") == "$remaining_digest" ]]
  test -f "$attempts/$repaired/manifest.json"
  no_staged_copy "identity-bound repair ($lost)"
  # The repaired installation uninstalls normally, with no authorization.
  cask_uninstall "$stable_cask"
  test ! -e "$app"
  test ! -e "$receipt"
done

# --- 7. a damaged receipt needs explicit authorization ---------------------

for damage in missing invalid; do
  for materials in kept removed; do
    cask_install "$stable_cask" "$first"
    staged=$(staged_path_of "$stable_cask")
    attempt=$(latest_attempt "$staged")
    [[ $attempt =~ ^[0-9a-f]{32}$ ]]
    original_id=$(receipt_field installation_id)
    if [[ $damage == missing ]]; then unlink "$receipt"; else printf '{' >"$receipt"; fi
    if [[ $materials == removed ]]; then
      attempt_directory="$attempts/$attempt"
      rm -rf "${attempt_directory:?}"
    fi
    app_digest=$(tree_digest "$app")
    receipt_digest=$(tree_digest "$receipt")
    remaining_digest=$(tree_digest "$attempts/$attempt")
    cp "$staged/.agentdeck-attempt" "$temporary/locator-backup"

    expect_failure "uninstall removed an App it could not verify ($damage, $materials)" cask_uninstall "$stable_cask"
    expect_output "reason: receipt_$damage"
    expect_output 'HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask agentdeck-app'
    unchanged "unauthorized uninstall ($damage, $materials)"
    expect_failure "an unauthorized install replaced an unverified App" driver "$stable_cask" install
    expect_output "reason: receipt_$damage"
    expect_output 'HOMEBREW_AGENTDECK_REPAIR=1 brew install --cask agentdeck-app'
    unchanged "unauthorized install ($damage, $materials)"
    no_staged_copy "refused repair ($damage, $materials)"
    cp "$temporary/locator-backup" "$staged/.agentdeck-attempt"

    export HOMEBREW_AGENTDECK_REPAIR=1
    driver "$stable_cask" uninstall >"$temporary/failure.log" 2>&1
    expect_output 'skipped unverified'
    unchanged "authorized uninstall ($damage, $materials)"
    driver "$stable_cask" install
    unset HOMEBREW_AGENTDECK_REPAIR
    [[ $(receipt_field installation_id) != "$original_id" && $(latest_attempt "$staged") != "$attempt" ]]
    [[ $(tree_digest "$attempts/$attempt") == "$remaining_digest" ]]
    no_staged_copy "authorized repair ($damage, $materials)"
    cask_uninstall "$stable_cask"
    test ! -e "$app"
  done
done

# --- 8. authorization never reaches another valid owner --------------------

make_target="$temporary/make-prefix/bin/agentdeck"
mkdir -p "$(dirname "$make_target")" "$temporary/make-records"
cp "$helper_build" "$make_target.staged"
"$make_target.staged" install register --channel make --executable "$make_target" \
  --source "$make_target.staged" --cleanup-manifest "$temporary/make-records/manifest.json" >/dev/null
receipt_digest=$(tree_digest "$receipt")
make_digest=$(tree_digest "$make_target")
refuse_other_owner() {
  expect_failure "the cask replaced a make installation ($1)" cask_install "$stable_cask" "$first"
  expect_output 'reason: owned_by_other'
  expect_output 'make uninstall'
  [[ $(tree_digest "$receipt") == "$receipt_digest" && $(tree_digest "$make_target") == "$make_digest" ]] ||
    fail "the make installation changed ($1)"
  test ! -e "$app"
  no_staged_copy "other owner ($1)"
  purge_staged "$stable_cask"
}
refuse_other_owner "no authorization"
export HOMEBREW_AGENTDECK_REPAIR=1
refuse_other_owner "authorized"
export HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION=0123456789abcdef0123456789abcdef
refuse_other_owner "authorized with another ID"
unset HOMEBREW_AGENTDECK_REPAIR HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION

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
