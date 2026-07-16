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
auto_dist="$temporary/auto-dist"

mkdir -p "$(dirname "$state_sentinel")"
printf 'preserve user state\n' >"$state_sentinel"

make -C "$root" DIST_DIR="$auto_dist" build >/dev/null
auto_identity=$("$auto_dist/agentdeck" --format json version)
expected_tag=$(git -C "$root" describe --tags --abbrev=0 2>/dev/null || printf 'v0.0.0')
if git -C "$root" describe --exact-match --tags HEAD >/dev/null 2>&1 && git -C "$root" diff --quiet && git -C "$root" diff --cached --quiet; then
  expected_version=$expected_tag
else
  expected_version=$expected_tag-dev
fi
expected_commit=$(git -C "$root" rev-parse HEAD)
expected_branch=$(git -C "$root" rev-parse --abbrev-ref HEAD)
expected_go_version=$(go env GOVERSION)
printf '%s\n' "$auto_identity" | grep -F "\"version\":\"$expected_version\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -F "\"commit\":\"$expected_commit\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -F "\"branch\":\"$expected_branch\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -E '"build_time":"[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}"' >/dev/null

make_at_root() {
  HOME="$home" make -C "$root" PREFIX="$prefix" DIST_DIR="$dist" COMPLETION_SHELL=none "$@"
}

make_default_at_root() {
  env -u PREFIX HOME="$home" make -C "$root" DIST_DIR="$default_dist" COMPLETION_SHELL=none "$@"
}

make_default_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BRANCH=main BUILD_TIME='2026-07-15 00:00:00'
test -x "$default_binary"
test -f "$default_manifest"
identity=$("$default_binary" --format json version)
printf '%s\n' "$identity" | grep -F '"version":"v1.2.3"' >/dev/null
printf '%s\n' "$identity" | grep -F '"commit":"0123456789abcdef"' >/dev/null
printf '%s\n' "$identity" | grep -F '"branch":"main"' >/dev/null
printf '%s\n' "$identity" | grep -F '"build_time":"2026-07-15 00:00:00"' >/dev/null
printf '%s\n' "$identity" | grep -F '"go_version":"go' >/dev/null
make_default_at_root uninstall
test ! -e "$default_binary"
test ! -e "$default_manifest"

make_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BRANCH=main BUILD_TIME='2026-07-15 00:00:00'
test -x "$binary"
test -f "$manifest"
"$binary" version | grep -F 'v1.2.3' >/dev/null

if make_at_root install VERSION=v1.2.3 COMMIT=0123456789abcdef BRANCH=main BUILD_TIME='2026-07-15 00:00:00' >/dev/null 2>&1; then
  echo "install unexpectedly overwrote an existing binary" >&2
  exit 1
fi

old_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
upgrade_output=$(make_at_root install FORCE=1 VERSION=v1.2.4 COMMIT=fedcba9876543210 BRANCH=release BUILD_TIME='2026-07-15 01:00:00')
new_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
old_upgrade_block=$(printf 'upgrade from:\nRelease Version: v1.2.3\nGit Commit Hash: 0123456789abcdef\nGit Branch: main\nGo Version: %s\nUTC Build Time: 2026-07-15 00:00:00\nSHA-256: %s' "$expected_go_version" "$old_sha")
new_upgrade_block=$(printf 'upgrade to:\nRelease Version: v1.2.4\nGit Commit Hash: fedcba9876543210\nGit Branch: release\nGo Version: %s\nUTC Build Time: 2026-07-15 01:00:00\nSHA-256: %s' "$expected_go_version" "$new_sha")
printf '%s\n' "$upgrade_output" | grep -F "$old_upgrade_block" >/dev/null
printf '%s\n' "$upgrade_output" | grep -F "$new_upgrade_block" >/dev/null
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

make_at_root install FORCE=1 VERSION=v1.2.4 COMMIT=fedcba9876543210 BRANCH=release BUILD_TIME='2026-07-15 01:00:00'
make_at_root uninstall
test ! -e "$binary"
test ! -e "$manifest"
test -f "$state_sentinel"

mkdir -p "$(dirname "$binary")" "$(dirname "$manifest")"
printf '#!/bin/sh\n[ "$1" = version ] || exit 2\nprintf "agentdeck v1.2.2 commit=legacycommit build_time=unknown go_version=%s\\n"\n' "$expected_go_version" >"$binary"
chmod 0755 "$binary"
legacy_binary=$(cd "$(dirname "$binary")" && pwd -P)/agentdeck
legacy_sha=$(shasum -a 256 "$legacy_binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$legacy_binary" "$legacy_sha" >"$manifest"
legacy_upgrade_output=$(make_at_root install FORCE=1 VERSION=v1.2.5 COMMIT=aaaaaaaaaaaaaaaa BRANCH=main BUILD_TIME='2026-07-15 02:00:00')
legacy_upgrade_block=$(printf 'upgrade from:\nRelease Version: v1.2.2\nGit Commit Hash: legacycommit\nGit Branch: unknown\nGo Version: %s\nUTC Build Time: unknown\nSHA-256: %s' "$expected_go_version" "$legacy_sha")
printf '%s\n' "$legacy_upgrade_output" | grep -F "$legacy_upgrade_block" >/dev/null
make_at_root uninstall

mkdir -p "$(dirname "$binary")" "$(dirname "$manifest")"
printf '#!/bin/sh\nexit 0\n' >"$binary"
chmod 0755 "$binary"
fake_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$binary" "$fake_sha" >"$manifest"
if make_at_root install FORCE=1 VERSION=v1.2.5 COMMIT=aaaaaaaaaaaaaaaa BRANCH=main BUILD_TIME='2026-07-15 02:00:00' >/dev/null 2>&1; then
  echo "install unexpectedly trusted a non-AgentDeck binary through a version 1 manifest" >&2
  exit 1
fi
grep -F 'exit 0' "$binary" >/dev/null
