#!/usr/bin/env bash
set -euo pipefail

# Formula-to-Cask migration, mutual exclusion, and the uninstall boundary, all
# in a temporary HOME, a temporary application directory, and a temporary
# Homebrew prefix.
#
# Boundary, stated rather than implied: the formula preflight is exercised by a
# real `brew install --cask` command whose prefix, Cellar, Caskroom, cache, HOME,
# and application directory all live below this test's temporary directory. The
# remaining artifact declarations — app, binary links and targets,
# conflicts_with, and the zap omission — are driven through the local installer
# below so no published artifact or real user installation is touched.

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/private/tmp}/agentdeck-cask-migration.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

home="$temporary/home"
appdir="$temporary/Applications"
prefix="$temporary/brew"
formula_records="$temporary/formula-records"
mkdir -p "$home" "$appdir" "$prefix/bin" "$formula_records"

# --- fixtures -----------------------------------------------------------

make_app() {
  local source_root=$1 version=$2
  local bundle="$source_root/AgentDeck.app"
  rm -rf "$bundle"
  mkdir -p "$bundle/Contents/Helpers" "$bundle/Contents/Resources/completions" "$bundle/Contents/MacOS"
  printf '#!/bin/sh\nprintf "AgentDeck %s\\n" "%s"\n' '%s' "$version" >"$bundle/Contents/MacOS/AgentDeck"
  printf '#!/bin/sh\nprintf "Release Version: v%s\\n" "%s"\n' '%s' "$version" >"$bundle/Contents/Helpers/agentdeck"
  chmod 0755 "$bundle/Contents/MacOS/AgentDeck" "$bundle/Contents/Helpers/agentdeck"
  printf '# bash completion %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.bash"
  printf '#compdef agentdeck %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.zsh"
  printf '# fish completion %s\n' "$version" >"$bundle/Contents/Resources/completions/agentdeck.fish"
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

# --- a local installer driven by the cask's own declarations -------------

cask_conflicting_formulae() {
  # Homebrew's conflicts_with accepts casks only, so the formula exclusion is
  # declared by the cask's preflight refusal rather than by a stanza. This parser
  # feeds the declaration-driven checks below; real_brew_install_refusal exercises
  # the refusal through `brew install --cask`, while test-macos-distribution.sh
  # separately checks that Homebrew can load the rendered cask from a throwaway tap.
  awk '/\]\.each do \|conflicting_formula\|/ {
    line = $0
    while (match(line, /"[^"]+"/)) {
      print substr(line, RSTART + 1, RLENGTH - 2)
      line = substr(line, RSTART + RLENGTH)
    }
  }' "$1"
}

cask_binaries() {
  # Emits "<relative-source>\t<target-path>" for every binary artifact. The
  # continuation line carrying `target:` is joined first, because consuming it
  # while parsing would swallow the following artifact.
  awk '{
    if ($0 ~ /^[[:space:]]*target:/) {
      if (previous != "") { print previous " " $0; previous = "" }
      next
    }
    if (previous != "") print previous
    previous = $0
  }
  END { if (previous != "") print previous }' "$1" |
    awk -v prefix="$prefix" -v appdir="$appdir" '
      /^  binary / {
        source = $0
        sub(/^  binary "/, "", source)
        sub(/".*/, "", source)
        target = ""
        if ($0 ~ /target:/) {
          target = $0
          sub(/.*target: "/, "", target)
          sub(/".*/, "", target)
        }
        gsub(/#\{appdir\}/, appdir, source)
        if (target == "") {
          base = source
          sub(/.*\//, "", base)
          target = prefix "/bin/" base
        }
        gsub(/#\{HOMEBREW_PREFIX\}/, prefix, target)
        printf "%s\t%s\n", source, target
      }
    '
}

cask_install() {
  local cask_file=$1 source_bundle=$2
  local token
  token=$(awk -F'"' '/^cask "/ { print $2; exit }' "$cask_file")
  local record="$temporary/installed-$token"

  local formula
  for formula in $(cask_conflicting_formulae "$cask_file"); do
    if [[ -e "$formula_records/$formula" ]]; then
      echo "cask $token conflicts with the installed formula $formula" >&2
      return 3
    fi
  done

  local line source target
  while IFS=$'\t' read -r source target; do
    if [[ -e $target || -L $target ]]; then
      if [[ ! -e $record ]] || ! grep -Fxq "$target" "$record"; then
        echo "cask $token refuses to take over $target, which it does not own" >&2
        return 4
      fi
    fi
  done < <(cask_binaries "$cask_file")

  rm -rf "$appdir/AgentDeck.app"
  ditto "$source_bundle" "$appdir/AgentDeck.app"
  : >"$record"
  while IFS=$'\t' read -r source target; do
    mkdir -p "$(dirname "$target")"
    rm -f "$target"
    ln -s "$source" "$target"
    printf '%s\n' "$target" >>"$record"
  done < <(cask_binaries "$cask_file")
}

cask_uninstall() {
  local cask_file=$1
  local token
  token=$(awk -F'"' '/^cask "/ { print $2; exit }' "$cask_file")
  local record="$temporary/installed-$token"
  if [[ -e $record ]]; then
    local target
    while IFS= read -r target; do
      rm -f "$target"
    done <"$record"
    rm -f "$record"
  fi
  rm -rf "$appdir/AgentDeck.app"
}

real_brew_install_refusal() {
  local source_root="$temporary/real-brew-source"
  local source_bundle
  source_bundle=$(make_app "$source_root" 1.2.3)

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
  if env \
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
}

# --- 1. install, upgrade, uninstall --------------------------------------

stable_cask="$temporary/agentdeck-app.rb"
render_cask v1.2.3 "$stable_cask"
first=$(make_app "$temporary/source-1.2.3" 1.2.3)
cask_install "$stable_cask" "$first"

test -d "$appdir/AgentDeck.app"
test -L "$prefix/bin/agentdeck"
"$prefix/bin/agentdeck" | grep -F 'Release Version: v1.2.3' >/dev/null
test -L "$prefix/etc/bash_completion.d/agentdeck"
test -L "$prefix/share/zsh/site-functions/_agentdeck"
test -L "$prefix/share/fish/vendor_completions.d/agentdeck.fish"
grep -F '# bash completion 1.2.3' "$prefix/etc/bash_completion.d/agentdeck" >/dev/null
grep -F '#compdef agentdeck 1.2.3' "$prefix/share/zsh/site-functions/_agentdeck" >/dev/null
grep -F '# fish completion 1.2.3' "$prefix/share/fish/vendor_completions.d/agentdeck.fish" >/dev/null

# State the user owns, created between install and upgrade.
mkdir -p "$home/.agentdeck" "$home/.codex" "$home/.claude"
printf 'state\n' >"$home/.agentdeck/agentdeck.sqlite3"
printf 'key\n' >"$home/.agentdeck/credential.key"
printf 'codex config\n' >"$home/.codex/config.toml"
printf 'claude settings\n' >"$home/.claude/settings.json"
printf 'shell rc\n' >"$home/.zshrc"

upgrade_cask="$temporary/agentdeck-app-1.2.4.rb"
render_cask v1.2.4 "$upgrade_cask"
second=$(make_app "$temporary/source-1.2.4" 1.2.4)
cask_install "$upgrade_cask" "$second"
"$prefix/bin/agentdeck" | grep -F 'Release Version: v1.2.4' >/dev/null
grep -F '# fish completion 1.2.4' "$prefix/share/fish/vendor_completions.d/agentdeck.fish" >/dev/null
test -f "$home/.agentdeck/agentdeck.sqlite3"

cask_uninstall "$upgrade_cask"
test ! -e "$appdir/AgentDeck.app"
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

# The real Homebrew path owns the preflight control-flow contract. In
# particular, a refusal must unwind the Caskroom receipt as well as leave the
# app and command absent; checking only stderr and the process status misses the
# partial installation caused by an exit that bypasses Homebrew's rollback.
real_brew_install_refusal

# --- 2. mutual exclusion with the CLI-only formula -----------------------

# The exclusion is read out of the cask, so the read itself must be asserted.
# Planting one formula and watching the install refuse would stay green under a
# parser that silently lost the other, which is the unfalsifiable shape this
# suite is not allowed to keep: the lost channel would then install beside the
# cask and both would claim the `agentdeck` command.
declared_formulae=$(cask_conflicting_formulae "$stable_cask" | sort | tr '\n' ' ')
if [[ $declared_formulae != "agentdeck agentdeck-rc " ]]; then
  echo "cask must exclude both CLI formulae, read: [$declared_formulae]" >&2
  exit 1
fi

# Each declared formula is then planted on its own, because the refusal has to
# hold per channel rather than only for whichever one happens to be listed first.
for conflicting in agentdeck agentdeck-rc; do
  : >"$formula_records/$conflicting"
  if cask_install "$stable_cask" "$first" 2>/dev/null; then
    echo "cask installed alongside the conflicting $conflicting formula" >&2
    exit 1
  fi
  test ! -e "$appdir/AgentDeck.app"
  test ! -e "$prefix/bin/agentdeck"
  rm -f "$formula_records/$conflicting"
done

# Left planted for section 3, which removes it as the first step of the
# documented migration.
: >"$formula_records/agentdeck"
if cask_install "$stable_cask" "$first" 2>/dev/null; then
  echo "cask installed alongside the conflicting formula" >&2
  exit 1
fi
test ! -e "$appdir/AgentDeck.app"
test ! -e "$prefix/bin/agentdeck"

# --- 3. the documented migration path --------------------------------------

migration=$(awk '/brew uninstall agentdeck/, /brew install --cask agentdeck-app/' "$stable_cask")
grep -F 'brew uninstall agentdeck' <<<"$migration" >/dev/null
grep -F 'brew install --cask agentdeck-app' <<<"$migration" >/dev/null
rm -f "$formula_records/agentdeck"
cask_install "$stable_cask" "$first"
"$prefix/bin/agentdeck" | grep -F 'Release Version: v1.2.3' >/dev/null
test -f "$home/.agentdeck/agentdeck.sqlite3"
cask_uninstall "$stable_cask"

# --- 4. a foreign owner of the command is never taken over ----------------

# The legacy script installer owns ~/.local/bin/agentdeck; a cask target that is
# already someone else's file must refuse rather than relink it.
mkdir -p "$prefix/bin"
printf '#!/bin/sh\necho legacy\n' >"$prefix/bin/agentdeck"
chmod 0755 "$prefix/bin/agentdeck"
if cask_install "$stable_cask" "$first" 2>/dev/null; then
  echo "cask took over a command it does not own" >&2
  exit 1
fi
grep -F legacy "$prefix/bin/agentdeck" >/dev/null
test ! -e "$appdir/AgentDeck.app"
rm -f "$prefix/bin/agentdeck"

# The CLI-only installer keeps its own coverage in scripts/test-install.sh and
# scripts/test-completion-install.sh; it is not re-tested here, because what
# this file owns is the boundary between the two installations rather than
# either installation on its own.

printf 'cask migration and mutual exclusion: PASS\n'
