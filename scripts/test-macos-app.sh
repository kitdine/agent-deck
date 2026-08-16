#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scratch_root=${AGENTDECK_MACOS_SWIFT_SCRATCH:-/private/tmp/agentdeck-macos-swift-build}
module_cache=${AGENTDECK_MACOS_SWIFT_MODULE_CACHE:-/private/tmp/agentdeck-swift-module-cache}

if xcodebuild -version >/dev/null 2>&1; then
  bash "$repo_root/scripts/build-macos-app.sh"
  xcodebuild \
    -project "$repo_root/apps/macos/AgentDeck.xcodeproj" \
    -scheme AgentDeck \
    -configuration Debug \
    -derivedDataPath "$repo_root/apps/macos/build/DerivedData" \
    CODE_SIGNING_ALLOWED=NO \
    CODE_SIGNING_REQUIRED=NO \
    CODE_SIGN_IDENTITY= \
    test
  exit 0
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
    "$repo_root/desktop/fixtures/v1/snapshot-partial.json"
