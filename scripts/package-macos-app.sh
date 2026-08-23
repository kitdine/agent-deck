#!/usr/bin/env bash
set -euo pipefail

# Signs an already built AgentDeck.app inside-out, assembles the direct-download
# and Cask artifacts from it, and invokes notarization and stapling.
#
# It performs no release action of its own: it uploads nothing, publishes
# nothing, and reaches no Apple service unless a notarization credential is
# supplied deliberately. An isolated test drives the whole path with an ad-hoc
# identity and a stubbed notary tool, which is why every external command is an
# overridable variable rather than a hardcoded invocation.

usage() {
  echo "usage: package-macos-app.sh <app-path> <version-tag> <dist-dir>" >&2
  exit 2
}

if [[ $# -ne 3 ]]; then
  usage
fi

app=$1
tag=$2
dist_dir=$3
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [[ ! -d $app ]]; then
  echo "app bundle does not exist: $app" >&2
  exit 1
fi
if [[ ! $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?$ ]]; then
  echo "desktop packaging requires a stable or rc.N semantic version tag: $tag" >&2
  exit 2
fi

version=${tag#v}
identity=${AGENTDECK_SIGN_IDENTITY:--}
entitlements_root=${AGENTDECK_ENTITLEMENTS_DIR:-"$repo_root/apps/macos"}
app_entitlements="$entitlements_root/AgentDeck.entitlements"
widget_entitlements="$entitlements_root/AgentDeckWidget.entitlements"
codesign_tool=${AGENTDECK_CODESIGN:-/usr/bin/codesign}
notary_tool=${AGENTDECK_NOTARY_TOOL:-}
stapler_tool=${AGENTDECK_STAPLER:-}
assess_tool=${AGENTDECK_SPCTL:-/usr/sbin/spctl}
skip_notarization=${AGENTDECK_SKIP_NOTARIZATION:-0}
notary_profile=${AGENTDECK_NOTARY_PROFILE:-}
notary_keychain=${AGENTDECK_NOTARY_KEYCHAIN:-}
app_group=${AGENTDECK_APP_GROUP:-group.com.kitdine.agentdeck}

for tool in ditto hdiutil lipo plutil shasum; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required packaging tool not found: $tool" >&2
    exit 1
  fi
done

app=$(cd "$app" && pwd)
mkdir -p "$dist_dir"
dist_dir=$(cd "$dist_dir" && pwd)

helper="$app/Contents/Helpers/agentdeck"
widget="$app/Contents/PlugIns/AgentDeckWidget.appex"
frameworks="$app/Contents/Frameworks"
info_plist="$app/Contents/Info.plist"

for required in "$helper" "$widget" "$info_plist"; do
  if [[ ! -e $required ]]; then
    echo "app bundle is missing $required" >&2
    exit 1
  fi
done

# 1. Preconditions. Notarization inputs are resolved before anything is signed
# or assembled, so a missing credential refuses the run instead of leaving an
# unnotarized DMG on disk that looks like a release artifact.
if [[ $skip_notarization != 1 ]]; then
  if [[ -z $notary_tool ]]; then
    notary_tool="xcrun notarytool"
  fi
  if [[ -z $stapler_tool ]]; then
    stapler_tool="xcrun stapler"
  fi
  if [[ -z $notary_profile ]]; then
    echo "notarization requires AGENTDECK_NOTARY_PROFILE, or an explicit AGENTDECK_SKIP_NOTARIZATION=1" >&2
    exit 1
  fi
  # A profile stored into a non-default keychain is only readable when the same
  # keychain is named on the submit side too. Resolving it here means a caller
  # that stored the credential somewhere that no longer exists is refused before
  # anything is signed, rather than at the upload.
  if [[ -n $notary_keychain && ! -f $notary_keychain ]]; then
    echo "notarization keychain does not exist: $notary_keychain" >&2
    exit 1
  fi
  if [[ $identity == - ]]; then
    echo "notarization requires a Developer ID identity, not the ad-hoc identity" >&2
    exit 1
  fi
fi

# 2. Version identity. The App, its embedded helper, and the tag must agree
# before any signature is applied, because a signed mismatch is a released
# mismatch.
bundle_version=$(plutil -extract CFBundleShortVersionString raw -o - "$info_plist")
if [[ $bundle_version != "$version" ]]; then
  echo "app bundle version $bundle_version does not match release tag $tag" >&2
  exit 1
fi
helper_identity=$("$helper" --format json version)
if ! grep -F "\"version\":\"$tag\"" <<<"$helper_identity" >/dev/null; then
  echo "embedded helper does not report release version $tag: $helper_identity" >&2
  exit 1
fi

# 3. Shell completions. The Cask exposes these from inside the bundle, so they
# are packaged rather than generated on the installing machine.
completions="$app/Contents/Resources/completions"
rm -rf "$completions"
mkdir -p "$completions"
"$helper" completion bash >"$completions/agentdeck.bash"
"$helper" completion zsh >"$completions/agentdeck.zsh"
"$helper" completion fish >"$completions/agentdeck.fish"
chmod 0644 "$completions"/agentdeck.*
for shell in bash zsh fish; do
  if [[ ! -s "$completions/agentdeck.$shell" ]]; then
    echo "packaged $shell completion is empty" >&2
    exit 1
  fi
done

# 4. Signing, strictly inside-out: nested code must already be sealed when the
# enclosing signature is computed, or the enclosing seal records nothing.
sign() {
  local target=$1
  local entitlements=${2:-}
  local arguments=(--force --sign "$identity" --options runtime --generate-entitlement-der)
  if [[ $identity == - ]]; then
    # An ad-hoc signature cannot carry a secure timestamp; a real identity must.
    arguments+=(--timestamp=none)
  else
    arguments+=(--timestamp)
  fi
  if [[ -n $entitlements ]]; then
    arguments+=(--entitlements "$entitlements")
  fi
  "$codesign_tool" "${arguments[@]}" "$target"
}

resolved_widget_entitlements="$dist_dir/AgentDeckWidget.resolved.entitlements"
resolved_app_entitlements="$dist_dir/AgentDeck.resolved.entitlements"
for pair in "$widget_entitlements:$resolved_widget_entitlements" "$app_entitlements:$resolved_app_entitlements"; do
  source_file=${pair%%:*}
  target_file=${pair##*:}
  sed "s|\$(AGENTDECK_APP_GROUP)|$app_group|g" "$source_file" >"$target_file"
  plutil -lint "$target_file" >/dev/null
done

sign "$helper"
if [[ -d $frameworks ]]; then
  while IFS= read -r framework; do
    sign "$framework"
  done < <(find "$frameworks" -maxdepth 1 -name '*.framework' -print)
fi
sign "$widget" "$resolved_widget_entitlements"
sign "$app" "$resolved_app_entitlements"

"$codesign_tool" --verify --deep --strict --verbose=2 "$app"
"$codesign_tool" --display --entitlements - "$widget" |
  grep -F "$app_group" >/dev/null

# 5. Direct-download and Cask artifacts. Both carry the same bundle, so a user
# who downloads the DMG and a user who installs the Cask run the same code.
dmg="$dist_dir/AgentDeck_${tag}_universal.dmg"
zip_archive="$dist_dir/AgentDeck_${tag}_universal.zip"
checksums="$dist_dir/AgentDeck_${tag}_checksums.txt"
staging=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-macos-package.XXXXXX")
trap 'rm -rf "$staging"' EXIT

ditto "$app" "$staging/AgentDeck.app"
rm -f "$dmg" "$zip_archive"
hdiutil create -quiet -srcfolder "$staging" -volname "AgentDeck $version" \
  -fs HFS+ -format UDZO -ov "$dmg"

# 6. Notarization and stapling. Missing credentials fail closed rather than
# silently producing an unnotarized artifact that looks released.
#
# Both published artifacts are stapled, not just the DMG: the ZIP carries the
# bundle directly, so a ZIP made from an unstapled bundle needs the network to
# clear Gatekeeper and fails first launch offline. One submission covers both,
# because the ticket is issued against the code signature the two share.
notary_keychain_argument=()
if [[ -n $notary_keychain ]]; then
  notary_keychain_argument=(--keychain "$notary_keychain")
fi
if [[ $skip_notarization == 1 ]]; then
  printf '%s\n' "notarization skipped by AGENTDECK_SKIP_NOTARIZATION=1"
else
  # shellcheck disable=SC2086
  $notary_tool submit "$dmg" --keychain-profile "$notary_profile" \
    ${notary_keychain_argument[@]+"${notary_keychain_argument[@]}"} --wait
  # shellcheck disable=SC2086
  $stapler_tool staple "$dmg"
  # shellcheck disable=SC2086
  $stapler_tool validate "$dmg"
  # shellcheck disable=SC2086
  $stapler_tool staple "$app"
  # shellcheck disable=SC2086
  $stapler_tool validate "$app"
fi

# 7. The ZIP is assembled last, from the bundle stapling has already amended, so
# the direct-download archive and the DMG carry the same ticket.
ditto -c -k --keepParent "$app" "$zip_archive"

# 8. Gatekeeper assessment. Its verdict is reported for every build; only a
# release run requires it to pass, because an ad-hoc or unnotarized bundle is
# correctly rejected and that rejection is itself the observable behavior.
assessment_status=0
assessment=$("$assess_tool" --assess --type execute --verbose=4 "$app" 2>&1) || assessment_status=$?
printf '%s\n' "$assessment"
if [[ ${AGENTDECK_REQUIRE_GATEKEEPER:-0} == 1 && $assessment_status -ne 0 ]]; then
  echo "Gatekeeper assessment failed for $app" >&2
  exit 1
fi

(
  cd "$dist_dir"
  shasum -a 256 "$(basename "$dmg")" "$(basename "$zip_archive")" >"$checksums"
)
rm -f "$resolved_widget_entitlements" "$resolved_app_entitlements"

printf 'packaged %s (%s, %s)\n' "$tag" "$(basename "$dmg")" "$(basename "$zip_archive")"
