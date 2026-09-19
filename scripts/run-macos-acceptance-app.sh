#!/usr/bin/env bash
set -euo pipefail

# Launches an isolated copy of the Debug AgentDeck.app for native manual
# acceptance, such as quota notification delivery (subscription-quota MA-F3).
#
# The notification service only admits a team-signed application launched from
# outside /tmp: an ad-hoc or unsigned bundle, or one run from /private/tmp, is
# rejected with "Notifications are not allowed for this application" before any
# permission prompt. So the copy is:
#   - given its own bundle identifier, so permission and Notification Centre
#     records never mix with the installed AgentDeck;
#   - stripped of the Widget extension and test bundles, so nothing registers
#     in the real Widget gallery;
#   - team-signed (AGENTDECK_ACCEPTANCE_SIGN_IDENTITY, or the first Developer ID
#     or Apple Development identity in the keychain) — never ad-hoc;
#   - placed in the ignored apps/macos/build/acceptance directory;
#   - run against a temporary AGENTDECK_TEST_HOME, so it never reads or writes
#     real AgentDeck, Codex, or Claude state.
#
# Usage: scripts/run-macos-acceptance-app.sh [--seed-quota-alert] [suffix]
#   --seed-quota-alert  turn quota reading on and record a Claude five-hour
#                       window at 80 % with thresholds 75, leaving alerts off so
#                       turning them on in Settings is the permission request.
#   suffix              bundle identifier suffix (default: quotaalerts).
#
# Cleanup after acceptance: quit the app, then run this script with --cleanup
# [suffix]. The notification permission entry for the acceptance identifier
# stays in System Settings › Notifications until removed there.

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
lsregister=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
seed=0
cleanup=0
suffix=quotaalerts
for argument in "$@"; do
  case "$argument" in
  --seed-quota-alert) seed=1 ;;
  --cleanup) cleanup=1 ;;
  -*) echo "unknown option: $argument" >&2; exit 2 ;;
  *) suffix=$argument ;;
  esac
done
if [[ ! $suffix =~ ^[a-z0-9]+$ ]]; then
  echo "suffix must be lowercase letters and digits: $suffix" >&2
  exit 2
fi

bundle_id="com.kitdine.agentdeck.acceptance.$suffix"
acceptance_dir="$repo_root/apps/macos/build/acceptance"
app="$acceptance_dir/AgentDeckAcceptance-$suffix.app"

if (( cleanup )); then
  if [[ -d $app ]]; then
    "$lsregister" -u "$app" >/dev/null 2>&1 || true
    rm -rf "$app"
  fi
  echo "removed $app; remove $bundle_id from System Settings › Notifications if it is listed"
  exit 0
fi

if ! git -C "$repo_root" check-ignore -q "$app"; then
  echo "refusing: $app is not ignored by Git" >&2
  exit 1
fi

identity=${AGENTDECK_ACCEPTANCE_SIGN_IDENTITY:-}
if [[ -z $identity ]]; then
  identity=$(security find-identity -v -p codesigning |
    awk -F'"' '/"(Developer ID Application|Apple Development): / { print $2; exit }')
fi
if [[ -z $identity ]]; then
  echo "no team signing identity; notifications cannot be accepted with an ad-hoc build" >&2
  exit 1
fi

AGENTDECK_APP_CONFIGURATION=Debug bash "$repo_root/scripts/build-macos-app.sh"
built="$repo_root/apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app"

mkdir -p "$acceptance_dir"
if [[ -d $app ]]; then
  "$lsregister" -u "$app" >/dev/null 2>&1 || true
  rm -rf "$app"
fi
ditto "$built" "$app"
rm -rf "$app/Contents/PlugIns"
plist="$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier $bundle_id" "$plist"
/usr/libexec/PlistBuddy -c "Delete :CFBundleDisplayName" "$plist" >/dev/null 2>&1 || true
/usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string AgentDeck Acceptance" "$plist"

codesign --force --timestamp=none --sign "$identity" "$app/Contents/Helpers/agentdeck"
if [[ -d "$app/Contents/Frameworks" ]]; then
  find "$app/Contents/Frameworks" -maxdepth 1 \( -name '*.framework' -o -name '*.dylib' \) -print0 |
    xargs -0 -n1 codesign --force --timestamp=none --sign "$identity"
fi
codesign --force --timestamp=none --sign "$identity" "$app"
codesign --verify --strict "$app"
team=$(codesign -dv "$app" 2>&1 | awk -F= '/^TeamIdentifier=/ { print $2 }')
if [[ -z $team || $team == "not set" ]]; then
  echo "refusing: $app has no team identifier" >&2
  exit 1
fi
"$lsregister" -f "$app"

test_root=$(mktemp -d /private/tmp/agentdeck-menubar-acceptance.XXXXXX)
case "$test_root" in
/private/tmp/agentdeck-menubar-acceptance.*) ;;
*) echo "refusing unsafe acceptance root: $test_root" >&2; exit 1 ;;
esac
test_home="$test_root/home"
mkdir -p "$test_home"
chmod 700 "$test_root" "$test_home"

if (( seed )); then
  helper="$app/Contents/Helpers/agentdeck"
  isolated() { env -i HOME="$test_home" PATH=/usr/bin:/bin LANG=en_US.UTF-8 "$@"; }
  isolated "$helper" --format json desktop quota-settings --reading on --alerts off --thresholds 75 >/dev/null
  now=$(date +%s)
  printf '{"rate_limits":{"five_hour":{"used_percentage":80,"resets_at":%d}}}' $((now + 10800)) |
    isolated "$helper" quota capture >/dev/null
fi

open -n --env AGENTDECK_TEST_HOME="$test_home" "$app"
cat <<EOF
bundle identifier: $bundle_id
signed by:         $identity (team $team)
application:       $app
isolated home:     $test_home
cleanup:           scripts/run-macos-acceptance-app.sh --cleanup $suffix; rm -rf $test_root
EOF
