#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scratch_root=${AGENTDECK_MACOS_SWIFT_SCRATCH:-/private/tmp/agentdeck-macos-swift-build}
module_cache=${AGENTDECK_MACOS_SWIFT_MODULE_CACHE:-/private/tmp/agentdeck-swift-module-cache}

if xcodebuild -version >/dev/null 2>&1; then
	test_root=$(mktemp -d /private/tmp/agentdeck-macos-xctest.XXXXXX)
	case "$test_root" in
		/private/tmp/agentdeck-macos-xctest.*) ;;
		*) echo "refusing unsafe XCTest root: $test_root" >&2; exit 1 ;;
	esac
	test_home="$test_root/home"
	mkdir -p "$test_home"
	test_helper="$repo_root/apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app/Contents/Helpers/agentdeck"

	isolation_pids() {
		ps -axo pid=,command= | awk -v helper="$test_helper" '$2 == helper { print $1 }'
	}
	baseline_pids=" $(isolation_pids | tr '\n' ' ') "
	new_isolation_pids() {
		local pid
		for pid in $(isolation_pids); do
			case "$baseline_pids" in
				*" $pid "*) ;;
				*) echo "$pid" ;;
			esac
		done
	}
	cleanup() {
		local pid
		for pid in $(new_isolation_pids); do
			kill "$pid" 2>/dev/null || true
		done
		for _ in {1..20}; do
			[[ -z "$(new_isolation_pids)" ]] && break
			sleep 0.1
		done
		for pid in $(new_isolation_pids); do
			kill -KILL "$pid" 2>/dev/null || true
		done
		rm -rf "$test_root"
	}
	trap cleanup EXIT
	trap 'exit 130' INT
	trap 'exit 143' TERM

  bash "$repo_root/scripts/build-macos-app.sh"
	set +e
	env \
		HOME="$test_home" \
		CFFIXED_USER_HOME="$test_home" \
		AGENTDECK_TEST_HOME="$test_home" \
		TEST_RUNNER_AGENTDECK_TEST_HOME="$test_home" \
		xcodebuild \
    -project "$repo_root/apps/macos/AgentDeck.xcodeproj" \
    -scheme AgentDeck \
    -configuration Debug \
    -derivedDataPath "$repo_root/apps/macos/build/DerivedData" \
    CODE_SIGNING_ALLOWED=NO \
    CODE_SIGNING_REQUIRED=NO \
    CODE_SIGN_IDENTITY= \
    test
	test_status=$?
	set -e

	for _ in {1..50}; do
		[[ -z "$(new_isolation_pids)" ]] && break
		sleep 0.1
	done
	leaked=$(new_isolation_pids)
	if [[ -n "$leaked" ]]; then
		echo "XCTest left AgentDeck helper processes from this worktree: $leaked" >&2
		test_status=1
	fi
	exit "$test_status"
fi

# Command Line Tools do not ship XCTest. The verifier executes the same
# synthetic-fixture contract checks without reading user or client state.
env \
  CLANG_MODULE_CACHE_PATH="$module_cache" \
  SWIFTPM_MODULECACHE_OVERRIDE="$module_cache" \
  SWIFTPM_CONFIG_DIR=/private/tmp/agentdeck-swiftpm-config \
  SWIFTPM_SECURITY_DIR=/private/tmp/agentdeck-swiftpm-security \
  SWIFTPM_CACHE_DIR=/private/tmp/agentdeck-swiftpm-cache \
  swift run \
    --disable-sandbox \
    --package-path "$repo_root/apps/macos" \
    --scratch-path "$scratch_root" \
    AgentDeckFoundationVerifier \
    "$repo_root/desktop/fixtures/v1/snapshot-complete.json" \
    "$repo_root/desktop/fixtures/v1/snapshot-partial.json" \
    "$repo_root/desktop/fixtures/v1/snapshot-empty-client.json" \
    "$repo_root/desktop/fixtures/v1/snapshot-legacy.json" \
    "$repo_root/desktop/fixtures/v1/snapshot-schema-ahead.json"
