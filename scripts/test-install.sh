#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-install.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

prefix="$temporary/prefix"
dist="$temporary/dist"
default_dist="$temporary/default-dist"
home="$temporary/home"
binary="$prefix/bin/agentdeck"
manifest="$prefix/share/agentdeck/install-manifest"
default_binary="$home/.local/bin/agentdeck"
default_manifest="$home/.local/share/agentdeck/install-manifest"
state_sentinel="$home/.agentdeck/preserved"

mkdir -p "$(dirname "$state_sentinel")"
printf 'preserve user state\n' >"$state_sentinel"

make_at_root() {
  HOME="$home" make -C "$root" PREFIX="$prefix" DIST_DIR="$dist" COMPLETION_SHELL=none "$@"
}

make_default_at_root() {
  env -u PREFIX HOME="$home" make -C "$root" DIST_DIR="$default_dist" COMPLETION_SHELL=none "$@"
}

make_default_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BUILD_TIME=2026-07-15T00:00:00Z
test -x "$default_binary"
test -f "$default_manifest"
identity=$("$default_binary" --format json version)
printf '%s\n' "$identity" | grep -F '"version":"v1.2.3"' >/dev/null
printf '%s\n' "$identity" | grep -F '"commit":"0123456789abcdef"' >/dev/null
printf '%s\n' "$identity" | grep -F '"build_time":"2026-07-15T00:00:00Z"' >/dev/null
printf '%s\n' "$identity" | grep -F '"go_version":"go' >/dev/null
make_default_at_root uninstall
test ! -e "$default_binary"
test ! -e "$default_manifest"

make_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BUILD_TIME=2026-07-15T00:00:00Z
test -x "$binary"
test -f "$manifest"
"$binary" version | grep -F 'v1.2.3' >/dev/null

if make_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BUILD_TIME=2026-07-15T00:00:00Z >/dev/null 2>&1; then
  echo "install unexpectedly overwrote an existing binary" >&2
  exit 1
fi

make_at_root install FORCE=1 VERSION=v1.2.4 COMMIT=fedcba9876543210 BUILD_TIME=2026-07-15T01:00:00Z
"$binary" version | grep -F 'v1.2.4' >/dev/null

original_manifest=$(cat "$manifest")
sed 's|^binary_path=.*$|binary_path=/tmp/not-agentdeck|' "$manifest" >"$manifest.tmp"
mv "$manifest.tmp" "$manifest"
if make_at_root uninstall >/dev/null 2>&1; then
  echo "uninstall unexpectedly trusted a mismatched manifest path" >&2
  exit 1
fi
test -f "$binary"
printf '%s\n' "$original_manifest" >"$manifest"

cp "$binary" "$binary.original"
printf 'tampered\n' >>"$binary"
if make_at_root uninstall >/dev/null 2>&1; then
  echo "uninstall unexpectedly removed a tampered binary" >&2
  exit 1
fi
test -f "$binary"
test -f "$manifest"
mv "$binary.original" "$binary"

make_at_root install FORCE=1 VERSION=v1.2.4 COMMIT=fedcba9876543210 BUILD_TIME=2026-07-15T01:00:00Z
make_at_root uninstall
test ! -e "$binary"
test ! -e "$manifest"
test -f "$state_sentinel"
