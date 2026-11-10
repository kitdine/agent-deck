#!/usr/bin/env bash
set -euo pipefail

# make install registers the installed program as the account's AgentDeck
# owner. A production binary would write the real account's receipt, so every
# install below runs an agentdeck_acceptance build whose account directory is
# this test's temporary AGENTDECK_ACCEPTANCE_ACCOUNT_HOME. The guard below
# refuses to continue if the binary is not such a build.

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-install.XXXXXX")
trap 'rm -rf "$temporary"' EXIT
temporary=$(cd "$temporary" && pwd -P)

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}
export GOMODCACHE=${GOMODCACHE:-/private/tmp/agent-deck-go-mod}
go_bin=${GO:-go}
buildinfo=github.com/kitdine/agent-deck/internal/buildinfo

prefix="$temporary/prefix"
home="$temporary/home"
account="$temporary/account"
binary="$prefix/bin/agentdeck"
manifest="$prefix/share/agentdeck/install-manifest"
default_binary="$home/.local/bin/agentdeck"
default_manifest="$home/.local/share/agentdeck/install-manifest"
receipt="$account/Library/Application Support/AgentDeck/installation.json"
state_sentinel="$home/.agentdeck/preserved"
auto_dist="$temporary/auto-dist"
export AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account"

mkdir -p "$(dirname "$state_sentinel")" "$account"
printf 'preserve user state\n' >"$state_sentinel"

# The ordinary build recipe still stamps its identity; it is never installed.
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
expected_go_version=$("$go_bin" env GOVERSION)
printf '%s\n' "$auto_identity" | grep -F "\"version\":\"$expected_version\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -F "\"commit\":\"$expected_commit\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -F "\"branch\":\"$expected_branch\"" >/dev/null
printf '%s\n' "$auto_identity" | grep -E '"build_time":"[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}"' >/dev/null

# build_acceptance <dir> <version> <commit> <branch> <build time>
build_acceptance() {
  mkdir -p "$1"
  (cd "$root" && "$go_bin" build -mod=vendor -trimpath -tags agentdeck_acceptance \
    -ldflags "-X '$buildinfo.Version=$2' -X '$buildinfo.Commit=$3' -X '$buildinfo.Branch=$4' -X '$buildinfo.BuildTime=$5'" \
    -o "$1/agentdeck" ./cmd/agentdeck)
  "$go_bin" tool nm "$1/agentdeck" | grep -F 'installation.acceptanceAccountHome' >/dev/null || {
    echo "refusing to install a binary that would use the real account: $1/agentdeck" >&2
    exit 1
  }
}

build_acceptance "$temporary/dist-123" v1.2.3 0123456789abcdef main '2026-07-15 00:00:00'
build_acceptance "$temporary/dist-124" v1.2.4 fedcba9876543210 release '2026-07-15 01:00:00'
build_acceptance "$temporary/dist-125" v1.2.5 aaaaaaaaaaaaaaaa main '2026-07-15 02:00:00'

manage() {
  HOME="$home" PREFIX="$prefix" COMPLETION_SHELL=none bash "$root/scripts/manage-install.sh" "$@"
}

manage_force() {
  FORCE=1 manage "$@"
}

manage_default() {
  env -u PREFIX HOME="$home" COMPLETION_SHELL=none bash "$root/scripts/manage-install.sh" "$@"
}

receipt_field() {
  sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p" "$receipt"
}

expect_failure() {
  local message=$1
  shift
  if "$@" >"$temporary/failure.log" 2>&1; then
    echo "$message" >&2
    exit 1
  fi
}

# --- default prefix: install registers, uninstall removes conditionally ---
manage_default install "$temporary/dist-123/agentdeck" >/dev/null
test -x "$default_binary"
test -f "$default_manifest"
identity=$("$default_binary" --format json version)
printf '%s\n' "$identity" | grep -F '"version":"v1.2.3"' >/dev/null
printf '%s\n' "$identity" | grep -F '"commit":"0123456789abcdef"' >/dev/null
printf '%s\n' "$identity" | grep -F '"branch":"main"' >/dev/null
printf '%s\n' "$identity" | grep -F '"build_time":"2026-07-15 00:00:00"' >/dev/null
printf '%s\n' "$identity" | grep -F '"go_version":"go' >/dev/null
physical_default=$(cd "$(dirname "$default_binary")" && pwd -P)/agentdeck
[ "$(receipt_field executable)" = "$physical_default" ]
[ "$(receipt_field channel)" = make ]
grep -Fx "installation_id=$(receipt_field installation_id)" "$default_manifest" >/dev/null
HOME="$home" "$default_binary" --format json doctor | grep -F '"reason":"make"' >/dev/null
manage_default uninstall >/dev/null
test ! -e "$default_binary"
test ! -e "$default_manifest"
test ! -e "$receipt"
test -z "$(ls -A "$home/.local/share/agentdeck/cleanup")"

# --- explicit prefix: overwrite refusal, FORCE upgrade, tamper refusal ---
manage install "$temporary/dist-123/agentdeck" >/dev/null
test -x "$binary"
"$binary" version | grep -F 'v1.2.3' >/dev/null
week_stats=$(HOME="$home" "$binary" --state-dir "$temporary/week-state" --format json usage stats --period week)
printf '%s\n' "$week_stats" | grep -F '"command":"usage.stats"' >/dev/null
printf '%s\n' "$week_stats" | grep -F '"range":' >/dev/null
first_id=$(receipt_field installation_id)
first_cleanup=$(sed -n 's/^cleanup_manifest=//p' "$manifest")
test -f "$first_cleanup"

expect_failure "install unexpectedly overwrote an existing binary" manage install "$temporary/dist-123/agentdeck"
[ "$(receipt_field installation_id)" = "$first_id" ]

old_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
upgrade_output=$(FORCE=1 manage install "$temporary/dist-124/agentdeck")
new_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
old_upgrade_block=$(printf 'upgrade from:\nRelease Version: v1.2.3\nGit Commit Hash: 0123456789abcdef\nGit Branch: main\nGo Version: %s\nUTC Build Time: 2026-07-15 00:00:00\nSHA-256: %s' "$expected_go_version" "$old_sha")
new_upgrade_block=$(printf 'upgrade to:\nRelease Version: v1.2.4\nGit Commit Hash: fedcba9876543210\nGit Branch: release\nGo Version: %s\nUTC Build Time: 2026-07-15 01:00:00\nSHA-256: %s' "$expected_go_version" "$new_sha")
printf '%s\n' "$upgrade_output" | grep -F "$old_upgrade_block" >/dev/null
printf '%s\n' "$upgrade_output" | grep -F "$new_upgrade_block" >/dev/null
"$binary" version | grep -F 'v1.2.4' >/dev/null
second_id=$(receipt_field installation_id)
[ "$second_id" != "$first_id" ]
test ! -e "$first_cleanup"
test -z "$(find "$prefix/bin" -name '.agentdeck.staged.*')"

original_manifest=$(cat "$manifest")
sed 's|^binary_path=.*$|binary_path=/tmp/not-agentdeck|' "$manifest" >"$manifest.tmp"
mv "$manifest.tmp" "$manifest"
expect_failure "uninstall unexpectedly trusted a mismatched manifest path" manage uninstall
test -f "$binary"
printf '%s\n' "$original_manifest" >"$manifest"

cp "$binary" "$binary.original"
printf 'tampered\n' >>"$binary"
expect_failure "uninstall unexpectedly removed a tampered binary" manage uninstall
test -f "$binary"
test -f "$manifest"
mv "$binary.original" "$binary"

# --- another owner: refused without TAKEOVER, nothing replaced ---
other="$temporary/other/agentdeck"
mkdir -p "$(dirname "$other")"
cp "$temporary/dist-125/agentdeck" "$other"
other_manifest="$temporary/other/cleanup.json"
cp "$temporary/dist-125/agentdeck" "$temporary/other/.agentdeck.next"
"$temporary/other/.agentdeck.next" install register --channel local_app --executable "$other" \
  --source "$temporary/other/.agentdeck.next" --cleanup-manifest "$other_manifest" --takeover >/dev/null
other_receipt=$(cat "$receipt")
before_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
expect_failure "install replaced another owner without TAKEOVER" manage_force install "$temporary/dist-125/agentdeck"
grep -F 'owned_by_other' "$temporary/failure.log" >/dev/null
[ "$(cat "$receipt")" = "$other_receipt" ]
[ "$(shasum -a 256 "$binary" | awk '{print $1}')" = "$before_sha" ]
test -z "$(find "$prefix/bin" -name '.agentdeck.staged.*')"

# The superseded make installation's uninstall leaves the new owner alone.
skip_output=$(manage uninstall)
printf '%s\n' "$skip_output" | grep -F 'skipped: a later installation now owns' >/dev/null
test -f "$binary"
[ "$(cat "$receipt")" = "$other_receipt" ]

FORCE=1 TAKEOVER=1 manage install "$temporary/dist-125/agentdeck" >/dev/null
[ "$(receipt_field channel)" = make ]
"$binary" version | grep -F 'v1.2.5' >/dev/null
manage uninstall >/dev/null
test ! -e "$binary"
test ! -e "$manifest"
test ! -e "$receipt"
test -f "$other"
test -f "$state_sentinel"

# --- legacy manifests: FORCE reinstall registers; legacy uninstall refuses ---
mkdir -p "$(dirname "$binary")" "$(dirname "$manifest")"
printf '#!/bin/sh\n[ "$1" = version ] || exit 2\nprintf "agentdeck v1.2.2 commit=legacycommit build_time=unknown go_version=%s\\n"\n' "$expected_go_version" >"$binary"
chmod 0755 "$binary"
legacy_binary=$(cd "$(dirname "$binary")" && pwd -P)/agentdeck
legacy_sha=$(shasum -a 256 "$legacy_binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$legacy_binary" "$legacy_sha" >"$manifest"
expect_failure "uninstall removed a legacy installation without a receipt" manage uninstall
grep -F 'predates installation receipts' "$temporary/failure.log" >/dev/null
test -f "$binary"
legacy_upgrade_output=$(FORCE=1 manage install "$temporary/dist-125/agentdeck")
legacy_upgrade_block=$(printf 'upgrade from:\nRelease Version: v1.2.2\nGit Commit Hash: legacycommit\nGit Branch: unknown\nGo Version: %s\nUTC Build Time: unknown\nSHA-256: %s' "$expected_go_version" "$legacy_sha")
printf '%s\n' "$legacy_upgrade_output" | grep -F "$legacy_upgrade_block" >/dev/null
[ "$(receipt_field executable)" = "$legacy_binary" ]
manage uninstall >/dev/null
test ! -e "$receipt"

mkdir -p "$(dirname "$binary")" "$(dirname "$manifest")"
printf '#!/bin/sh\nexit 0\n' >"$binary"
chmod 0755 "$binary"
fake_sha=$(shasum -a 256 "$binary" | awk '{print $1}')
printf 'path=%s\nsha256=%s\n' "$binary" "$fake_sha" >"$manifest"
expect_failure "install unexpectedly trusted a non-AgentDeck binary through a version 1 manifest" manage_force install "$temporary/dist-125/agentdeck"
grep -F 'exit 0' "$binary" >/dev/null
test ! -e "$receipt"

printf 'make install/uninstall: receipt registration, conditional cleanup and takeover verified in a temporary account.\n'
