#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d /private/tmp/agentdeck-completion-install.XXXXXX)
trap 'rm -rf "$temporary"' EXIT

dist="$temporary/dist"
binary="$dist/agentdeck"
manager="$root/scripts/manage-install.sh"

make -C "$root" DIST_DIR="$dist" build >/dev/null

install_for() {
  local shell=$1 home=$2 prefix=$3 rc=$4
  HOME="$home" PREFIX="$prefix" COMPLETION_SHELL="$shell" COMPLETION_RC="$rc" TMPDIR=/private/tmp \
    bash "$manager" install "$binary" >/dev/null
}

uninstall_from() {
  local home=$1 prefix=$2
  HOME="$home" PREFIX="$prefix" TMPDIR=/private/tmp bash "$manager" uninstall >/dev/null
}

smoke_shell() {
  local shell=$1 home=$2 prefix=$3 rc=$4 output
  case "$shell" in
    fish)
      output=$(HOME="$home" XDG_CONFIG_HOME="$home/.config" PATH="$prefix/bin:$PATH" fish -c 'complete -C "agentdeck "')
      ;;
    zsh)
      output=$(HOME="$home" ZDOTDIR="$home" PATH="$prefix/bin:$PATH" zsh -ic 'whence _agentdeck')
      ;;
    bash)
      output=$(HOME="$home" PATH="$prefix/bin:$PATH" bash --noprofile --rcfile "$rc" -ic 'complete -p agentdeck' 2>/dev/null)
      ;;
  esac
  printf '%s\n' "$output" | grep -E 'provider|_agentdeck|complete' >/dev/null
}

for shell in fish zsh bash; do
  case_root="$temporary/$shell"
  home="$case_root/home"
  prefix="$case_root/prefix"
  case "$shell" in
    fish) prefix="$case_root/prefix'quoted"; rc="$home/.config/fish/config.fish" ;;
    zsh) rc="$home/.zshrc" ;;
    bash) rc="$home/.bashrc" ;;
  esac
  mkdir -p "$(dirname "$rc")"
  printf '# user configuration\n' >"$rc"
  chmod 0640 "$rc"
  install_for "$shell" "$home" "$prefix" "$rc"
  completion="$prefix/share/agentdeck/completions/agentdeck.$shell"
  manifest="$prefix/share/agentdeck/install-manifest"
  expected_rc="$case_root/expected.rc"
  test -x "$prefix/bin/agentdeck"
  test -f "$completion"
  grep -F 'manifest_version=2' "$manifest" >/dev/null
  grep -F "completion_shell=$shell" "$manifest" >/dev/null
  grep -F 'rc_separator_added=0' "$manifest" >/dev/null
  grep -F '# >>> agentdeck completion >>>' "$rc" >/dev/null
  test "$(stat -f '%Lp' "$rc")" = 640
  smoke_shell "$shell" "$home" "$prefix" "$rc"
  printf '# user change after install\n' >>"$rc"
  printf '# user configuration\n# user change after install\n' >"$expected_rc"
  uninstall_from "$home" "$prefix"
  test ! -e "$prefix/bin/agentdeck"
  test ! -e "$completion"
  test -d "$prefix/bin"
  test -d "$prefix/share/agentdeck/completions"
  grep -F '# user configuration' "$rc" >/dev/null
  grep -F '# user change after install' "$rc" >/dev/null
  cmp "$expected_rc" "$rc"
  if grep -F '# >>> agentdeck completion >>>' "$rc" >/dev/null; then
    echo "$shell managed block survived uninstall" >&2
    exit 1
  fi
done

tamper_root="$temporary/tamper"
tamper_home="$tamper_root/home"
tamper_prefix="$tamper_root/prefix"
tamper_rc="$tamper_home/.config/fish/config.fish"
mkdir -p "$(dirname "$tamper_rc")"
printf '# keep\n' >"$tamper_rc"
install_for fish "$tamper_home" "$tamper_prefix" "$tamper_rc"
cp "$tamper_rc" "$tamper_rc.original"
sed 's|^source .*|source /tmp/not-agentdeck|' "$tamper_rc" >"$tamper_rc.tmp"
mv "$tamper_rc.tmp" "$tamper_rc"
if uninstall_from "$tamper_home" "$tamper_prefix" >/dev/null 2>&1; then
  echo 'uninstall trusted a modified managed block' >&2
  exit 1
fi
test -e "$tamper_prefix/bin/agentdeck"
mv "$tamper_rc.original" "$tamper_rc"
completion="$tamper_prefix/share/agentdeck/completions/agentdeck.fish"
cp "$completion" "$completion.original"
printf '# tampered\n' >>"$completion"
if uninstall_from "$tamper_home" "$tamper_prefix" >/dev/null 2>&1; then
  echo 'uninstall trusted a modified completion' >&2
  exit 1
fi
mv "$completion.original" "$completion"
uninstall_from "$tamper_home" "$tamper_prefix"

symlink_root="$temporary/symlink"
symlink_home="$symlink_root/home"
symlink_prefix="$symlink_root/prefix"
symlink_target="$symlink_home/dotfiles/fish.config"
symlink_rc="$symlink_home/.config/fish/config.fish"
mkdir -p "$(dirname "$symlink_target")" "$(dirname "$symlink_rc")"
printf '# linked configuration\n' >"$symlink_target"
ln -s "$symlink_target" "$symlink_rc"
install_for fish "$symlink_home" "$symlink_prefix" "$symlink_rc"
test -L "$symlink_rc"
grep -F '# >>> agentdeck completion >>>' "$symlink_target" >/dev/null
uninstall_from "$symlink_home" "$symlink_prefix"
test -L "$symlink_rc"
grep -F '# linked configuration' "$symlink_target" >/dev/null

duplicate_root="$temporary/duplicate"
duplicate_home="$duplicate_root/home"
duplicate_prefix="$duplicate_root/prefix"
duplicate_rc="$duplicate_home/.zshrc"
mkdir -p "$duplicate_home"
printf '# >>> agentdeck completion >>>\nforeign\n# <<< agentdeck completion <<<\n' >"$duplicate_rc"
if install_for zsh "$duplicate_home" "$duplicate_prefix" "$duplicate_rc" >/dev/null 2>&1; then
  echo 'install trusted an unmanaged marker block' >&2
  exit 1
fi
test ! -e "$duplicate_prefix/bin/agentdeck"
test ! -e "$duplicate_prefix"

dangling_root="$temporary/dangling"
dangling_home="$dangling_root/home"
dangling_prefix="$dangling_root/prefix"
dangling_target="$dangling_home/dotfiles/missing.fish"
dangling_rc="$dangling_home/.config/fish/config.fish"
mkdir -p "$(dirname "$dangling_target")" "$(dirname "$dangling_rc")"
ln -s "$dangling_target" "$dangling_rc"
if install_for fish "$dangling_home" "$dangling_prefix" "$dangling_rc" >/dev/null 2>&1; then
  echo 'install trusted a dangling shell rc symlink' >&2
  exit 1
fi
test -L "$dangling_rc"
test ! -e "$dangling_target"
test ! -e "$dangling_prefix"

v1_root="$temporary/v1"
v1_home="$v1_root/home"
v1_prefix="$v1_root/prefix"
mkdir -p "$v1_prefix/bin" "$v1_prefix/share/agentdeck"
cp "$binary" "$v1_prefix/bin/agentdeck"
chmod 0755 "$v1_prefix/bin/agentdeck"
v1_binary=$(cd "$v1_prefix/bin" && pwd -P)/agentdeck
v1_hash=$(shasum -a 256 "$v1_binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$v1_binary" "$v1_hash" >"$v1_prefix/share/agentdeck/install-manifest"
uninstall_from "$v1_home" "$v1_prefix"
test ! -e "$v1_binary"

v1_upgrade_root="$temporary/v1-upgrade"
v1_upgrade_home="$v1_upgrade_root/home"
v1_upgrade_prefix="$v1_upgrade_root/prefix"
v1_upgrade_rc="$v1_upgrade_home/.config/fish/config.fish"
mkdir -p "$v1_upgrade_prefix/bin" "$v1_upgrade_prefix/share/agentdeck" "$(dirname "$v1_upgrade_rc")"
cp "$binary" "$v1_upgrade_prefix/bin/agentdeck"
chmod 0755 "$v1_upgrade_prefix/bin/agentdeck"
v1_upgrade_binary=$(cd "$v1_upgrade_prefix/bin" && pwd -P)/agentdeck
v1_upgrade_hash=$(shasum -a 256 "$v1_upgrade_binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$v1_upgrade_binary" "$v1_upgrade_hash" >"$v1_upgrade_prefix/share/agentdeck/install-manifest"
HOME="$v1_upgrade_home" PREFIX="$v1_upgrade_prefix" FORCE=1 COMPLETION_SHELL=fish COMPLETION_RC="$v1_upgrade_rc" TMPDIR=/private/tmp \
  bash "$manager" install "$binary" >/dev/null
grep -F 'manifest_version=2' "$v1_upgrade_prefix/share/agentdeck/install-manifest" >/dev/null
uninstall_from "$v1_upgrade_home" "$v1_upgrade_prefix"

fake_root="$temporary/fake-process"
mkdir -p "$fake_root/bin"
printf '%s\n' '#!/bin/sh' \
  'case "$*" in' \
  '  *comm=*) printf "%s\n" "$FAKE_PS_COMM" ;;' \
  '  *ppid=*) printf "1\n" ;;' \
  'esac' >"$fake_root/bin/ps"
chmod 0755 "$fake_root/bin/ps"
detected=$(PATH="$fake_root/bin:$PATH" FAKE_PS_COMM=fish COMPLETION_SHELL=auto TMPDIR=/private/tmp bash "$manager" detect-shell)
test "$detected" = fish
detected=$(PATH="$fake_root/bin:$PATH" FAKE_PS_COMM=-zsh COMPLETION_SHELL=auto TMPDIR=/private/tmp bash "$manager" detect-shell)
test "$detected" = zsh

login_home="$fake_root/login-home"
login_prefix="$fake_root/login-prefix"
PATH="$fake_root/bin:$PATH" FAKE_PS_COMM=-bash HOME="$login_home" PREFIX="$login_prefix" COMPLETION_SHELL=auto TMPDIR=/private/tmp \
  bash "$manager" install "$binary" >/dev/null
test -f "$login_home/.bash_profile"
test ! -e "$login_home/.bashrc"
uninstall_from "$login_home" "$login_prefix"

unowned_root="$temporary/unowned"
unowned_home="$unowned_root/home"
unowned_prefix="$unowned_root/prefix"
unowned_target="$unowned_home/dotfiles/fish.config"
unowned_rc="$unowned_home/.config/fish/config.fish"
mkdir -p "$unowned_root/bin" "$(dirname "$unowned_target")" "$(dirname "$unowned_rc")"
printf '# unowned target fixture\n' >"$unowned_target"
ln -s "$unowned_target" "$unowned_rc"
printf '%s\n' '#!/bin/sh' 'case "$*" in *%u*) echo 99999 ;; *) /usr/bin/stat "$@" ;; esac' >"$unowned_root/bin/stat"
chmod 0755 "$unowned_root/bin/stat"
if PATH="$unowned_root/bin:$PATH" install_for fish "$unowned_home" "$unowned_prefix" "$unowned_rc" >/dev/null 2>&1; then
  echo 'install trusted an unowned shell rc target' >&2
  exit 1
fi
test ! -e "$unowned_prefix/bin/agentdeck"

rollback_root="$temporary/rollback"
rollback_home="$rollback_root/home"
rollback_prefix="$rollback_root/prefix"
rollback_rc="$rollback_home/.config/fish/config.fish"
mkdir -p "$rollback_prefix/bin" "$rollback_prefix/share/agentdeck/completions" "$(dirname "$rollback_rc")"
printf '# rollback sentinel\n' >"$rollback_rc"
chmod 0500 "$rollback_prefix/share/agentdeck"
if install_for fish "$rollback_home" "$rollback_prefix" "$rollback_rc" >/dev/null 2>&1; then
  chmod 0700 "$rollback_prefix/share/agentdeck"
  echo 'install unexpectedly completed with an unwritable manifest directory' >&2
  exit 1
fi
chmod 0700 "$rollback_prefix/share/agentdeck"
test ! -e "$rollback_prefix/bin/agentdeck"
test ! -e "$rollback_prefix/share/agentdeck/completions/agentdeck.fish"
test -d "$rollback_prefix/bin"
test -d "$rollback_prefix/share/agentdeck/completions"
grep -F '# rollback sentinel' "$rollback_rc" >/dev/null
if grep -F '# >>> agentdeck completion >>>' "$rollback_rc" >/dev/null; then
  echo 'failed install left a managed block' >&2
  exit 1
fi

interrupt_root="$temporary/interrupt"
interrupt_home="$interrupt_root/home"
interrupt_prefix="$interrupt_root/prefix"
interrupt_rc="$interrupt_home/.config/fish/config.fish"
interrupt_binary="$interrupt_root/agentdeck"
mkdir -p "$(dirname "$interrupt_rc")"
printf '# interrupt sentinel\n' >"$interrupt_rc"
printf '%s\n' '#!/usr/bin/env bash' 'kill -TERM "$PPID"' 'sleep 1' >"$interrupt_binary"
chmod 0755 "$interrupt_binary"
if HOME="$interrupt_home" PREFIX="$interrupt_prefix" COMPLETION_SHELL=fish COMPLETION_RC="$interrupt_rc" TMPDIR=/private/tmp \
  bash "$manager" install "$interrupt_binary" >/dev/null 2>&1; then
  echo 'install continued after a termination signal' >&2
  exit 1
fi
test ! -e "$interrupt_prefix/bin/agentdeck"
test ! -e "$interrupt_prefix/share/agentdeck/completions/agentdeck.fish"
test ! -e "$interrupt_prefix/share/agentdeck/install-manifest"
grep -F '# interrupt sentinel' "$interrupt_rc" >/dev/null
if grep -F '# >>> agentdeck completion >>>' "$interrupt_rc" >/dev/null; then
  echo 'interrupted install left a managed block' >&2
  exit 1
fi
test ! -e "$interrupt_prefix"

exact_root="$temporary/exact-rc"
exact_home="$exact_root/home"
exact_prefix="$exact_root/prefix"
exact_rc="$exact_home/.zshrc"
exact_expected="$exact_root/expected"
mkdir -p "$exact_home"
printf '# rc without trailing newline' >"$exact_rc"
cp -p "$exact_rc" "$exact_expected"
install_for zsh "$exact_home" "$exact_prefix" "$exact_rc"
HOME="$exact_home" PREFIX="$exact_prefix" FORCE=1 COMPLETION_SHELL=zsh COMPLETION_RC="$exact_rc" TMPDIR=/private/tmp \
  bash "$manager" install "$binary" >/dev/null
uninstall_from "$exact_home" "$exact_prefix"
cmp "$exact_expected" "$exact_rc"
test "$(stat -f '%Lp' "$exact_rc")" = "$(stat -f '%Lp' "$exact_expected")"

suffix_root="$temporary/suffix-rc"
suffix_home="$suffix_root/home"
suffix_prefix="$suffix_root/prefix"
suffix_rc="$suffix_home/.bashrc"
suffix_expected="$suffix_root/expected"
mkdir -p "$suffix_home"
printf '# original\n' >"$suffix_rc"
install_for bash "$suffix_home" "$suffix_prefix" "$suffix_rc"
printf '# suffix without trailing newline' >>"$suffix_rc"
printf '# original\n# suffix without trailing newline' >"$suffix_expected"
uninstall_from "$suffix_home" "$suffix_prefix"
cmp "$suffix_expected" "$suffix_rc"
