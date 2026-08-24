#!/usr/bin/env bash
set -euo pipefail

# Builds AgentDeck.app. The default is the unsigned Debug build the Swift test
# gate runs against; AGENTDECK_APP_CONFIGURATION=Release produces the universal
# candidate that scripts/package-macos-app.sh signs and packages.
#
# The Release helper is the exact artifact `make build-all` already produced, so
# the App, its embedded helper, and the standalone CLI archives carry one
# version and one commit rather than three independently stamped builds.

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="$repo_root/apps/macos/AgentDeck.xcodeproj"
build_root=${AGENTDECK_MACOS_BUILD_DIR:-"$repo_root/apps/macos/build"}
# The Embed AgentDeck Helper build phase reads $(PROJECT_DIR)/build/agentdeck, so
# the helper path is fixed by the Xcode project and does not follow
# AGENTDECK_MACOS_BUILD_DIR. Pointing it elsewhere would leave the phase copying
# a stale helper while this script reported building a fresh one.
helper="$repo_root/apps/macos/build/agentdeck"
derived_data="$build_root/DerivedData"
configuration=${AGENTDECK_APP_CONFIGURATION:-Debug}
dist_dir=${AGENTDECK_DIST_DIR:-"$repo_root/dist"}
build_number=${AGENTDECK_APP_BUILD_NUMBER:-}

case "$configuration" in
Debug | Release) ;;
*)
  echo "AGENTDECK_APP_CONFIGURATION must be Debug or Release: $configuration" >&2
  exit 2
  ;;
esac

if [[ -n $build_number && ! $build_number =~ ^[1-9][0-9]*$ ]]; then
  echo "AGENTDECK_APP_BUILD_NUMBER must be a positive integer: $build_number" >&2
  exit 2
fi

if ! xcodebuild -version >/dev/null 2>&1; then
  echo "A full Xcode installation must be selected to build AgentDeck.app." >&2
  exit 1
fi

mkdir -p "$build_root" "$(dirname "$helper")"

xcodebuild_arguments=(
  -project "$project"
  -scheme AgentDeck
  -configuration "$configuration"
  -derivedDataPath "$derived_data"
  CODE_SIGNING_ALLOWED=NO
  CODE_SIGNING_REQUIRED=NO
  CODE_SIGN_IDENTITY=
)

if [[ $configuration == Release ]]; then
  arm64_helper="$dist_dir/agentdeck_darwin_arm64"
  amd64_helper="$dist_dir/agentdeck_darwin_amd64"
  for source_binary in "$arm64_helper" "$amd64_helper"; do
    if [[ ! -f $source_binary ]]; then
      echo "missing release helper build: $source_binary (run \`make build-all\` first)" >&2
      exit 1
    fi
  done
  lipo -create -output "$helper" "$arm64_helper" "$amd64_helper"
  chmod 0755 "$helper"
  helper_architectures=$(lipo -archs "$helper")
  for architecture in arm64 x86_64; do
    if ! grep -Fw "$architecture" <<<"$helper_architectures" >/dev/null; then
      echo "universal helper is missing $architecture: $helper_architectures" >&2
      exit 1
    fi
  done
  xcodebuild_arguments+=(ARCHS="arm64 x86_64" ONLY_ACTIVE_ARCH=NO)
  if [[ -n ${AGENTDECK_APP_VERSION:-} ]]; then
    xcodebuild_arguments+=("AGENTDECK_MARKETING_VERSION=$AGENTDECK_APP_VERSION")
  fi
  if [[ -n $build_number ]]; then
    xcodebuild_arguments+=("AGENTDECK_PROJECT_VERSION=$build_number")
  fi
else
  # Go refuses to replace an existing universal Mach-O with a single-arch
  # debug executable. Build at a fresh same-directory path, then atomically
  # replace the generated helper so Release -> Debug verification is repeatable.
  debug_staging=$(mktemp -d "$(dirname "$helper")/.agentdeck-debug.XXXXXX")
  trap 'rm -rf "$debug_staging"' EXIT
  env GOCACHE="${GOCACHE:-/private/tmp/agent-deck-go-build}" \
    GOMODCACHE="${GOMODCACHE:-/private/tmp/agent-deck-go-mod}" \
    go build -mod=vendor -trimpath -o "$debug_staging/agentdeck" "$repo_root/cmd/agentdeck"
  mv -f "$debug_staging/agentdeck" "$helper"
  rmdir "$debug_staging"
  trap - EXIT
fi

xcodebuild "${xcodebuild_arguments[@]}" build

app="$derived_data/Build/Products/$configuration/AgentDeck.app"
embedded_helper="$app/Contents/Helpers/agentdeck"
if [[ ! -x "$embedded_helper" ]]; then
  echo "$configuration app build did not embed agentdeck helper: $embedded_helper" >&2
  exit 1
fi

app_binary="$app/Contents/MacOS/AgentDeck"
if ! otool -L "$app_binary" | grep -Fq '@rpath/AgentDeckShared.framework/Versions/A/AgentDeckShared'; then
  echo "Built app does not load its embedded AgentDeckShared framework through @rpath." >&2
  otool -L "$app_binary" >&2
  exit 1
fi

if [[ $configuration == Release ]]; then
  widget="$app/Contents/PlugIns/AgentDeckWidget.appex"
  framework_info="$app/Contents/Frameworks/AgentDeckShared.framework/Resources/Info.plist"
  if [[ ! -d $widget ]]; then
    echo "Release app build did not embed the widget extension: $widget" >&2
    exit 1
  fi
  if [[ ! -f $framework_info ]]; then
    echo "Release app build did not embed AgentDeckShared metadata: $framework_info" >&2
    exit 1
  fi
  if [[ -n $build_number ]]; then
    for plist in \
      "$app/Contents/Info.plist" \
      "$widget/Contents/Info.plist" \
      "$framework_info"; do
      actual_build=$(plutil -extract CFBundleVersion raw -o - "$plist")
      if [[ $actual_build != "$build_number" ]]; then
        echo "bundle build $actual_build in $plist does not match candidate build $build_number" >&2
        exit 1
      fi
    done
  fi
  # A universal App that ships a single-architecture binary is an Intel or
  # Apple silicon release that only reports itself as universal.
  for binary in "$app_binary" "$embedded_helper" "$widget/Contents/MacOS/AgentDeckWidget"; do
    binary_architectures=$(lipo -archs "$binary")
    for architecture in arm64 x86_64; do
      if ! grep -Fw "$architecture" <<<"$binary_architectures" >/dev/null; then
        echo "release binary $binary is missing $architecture: $binary_architectures" >&2
        exit 1
      fi
    done
  done
fi

printf '%s\n' "Unsigned $configuration AgentDeck.app built at $app"
