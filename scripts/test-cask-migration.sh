#!/usr/bin/env bash
set -euo pipefail

# Formula-to-Cask migration, mutual exclusion, and the uninstall boundary, all
# in a temporary HOME, a temporary application directory, and a temporary
# Homebrew prefix.
#
# Boundary, stated rather than implied: this drives the artifact set the cask
# DECLARES — its app, its binary links and their targets, its conflicts_with
# entries, and what its zap list omits — through a local installer that applies
# those declarations. It is not Homebrew, and it asserts nothing about
# Homebrew's implementation. Whether Homebrew accepts the rendered cask at all is
# scripts/test-macos-distribution.sh's section 4, which loads it through a
# throwaway tap inside the local Homebrew prefix; this file needs no Homebrew and
# writes only inside its own temporary directories. Installing the real cask with
# the real `brew` belongs to the separately authorized release path, which runs
# against a published artifact this task is not permitted to produce.

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
  # declared by the cask's preflight refusal rather than by a stanza. This reads
  # that declaration; whether Homebrew accepts the cask carrying it is
  # scripts/test-macos-distribution.sh's tap-backed load check, per the boundary
  # stated at the top of this file.
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
