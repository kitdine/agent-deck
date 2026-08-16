#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="$repo_root/apps/macos/AgentDeck.xcodeproj"
build_root=${AGENTDECK_MACOS_BUILD_DIR:-"$repo_root/apps/macos/build"}
helper="$build_root/agentdeck"
derived_data="$build_root/DerivedData"

if ! xcodebuild -version >/dev/null 2>&1; then
  echo "A full Xcode installation must be selected to build AgentDeck.app." >&2
  exit 1
fi

mkdir -p "$build_root"
env GOCACHE="${GOCACHE:-/private/tmp/agent-deck-go-build}" \
  GOMODCACHE="${GOMODCACHE:-/private/tmp/agent-deck-go-mod}" \
  go build -mod=vendor -trimpath -o "$helper" "$repo_root/cmd/agentdeck"

xcodebuild \
  -project "$project" \
  -scheme AgentDeck \
  -configuration Debug \
  -derivedDataPath "$derived_data" \
  CODE_SIGNING_ALLOWED=NO \
  CODE_SIGNING_REQUIRED=NO \
  CODE_SIGN_IDENTITY= \
  build

app="$derived_data/Build/Products/Debug/AgentDeck.app"
embedded_helper="$app/Contents/Helpers/agentdeck"
if [[ ! -x "$embedded_helper" ]]; then
  echo "Unsigned app build did not embed agentdeck helper: $embedded_helper" >&2
  exit 1
fi

printf '%s\n' "Unsigned AgentDeck.app built at $app"
