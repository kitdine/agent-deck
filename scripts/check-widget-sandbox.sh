#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
app_entitlements="$repo_root/apps/macos/AgentDeck.entitlements"
widget_entitlements="$repo_root/apps/macos/AgentDeckWidget.entitlements"
app_info="$repo_root/apps/macos/AgentDeckApp/Info.plist"
widget_info="$repo_root/apps/macos/AgentDeckWidget/Info.plist"
configuration="$repo_root/apps/macos/Config/AgentDeck.xcconfig"
shared_store="$repo_root/apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift"
widget_reader="$repo_root/apps/macos/AgentDeckWidget/WidgetSnapshot.swift"
sources="$repo_root/apps/macos/AgentDeckWidget"
project="$repo_root/apps/macos/AgentDeck.xcodeproj/project.pbxproj"

app_group=$(sed -n 's/^AGENTDECK_APP_GROUP = //p' "$configuration")
development_team=$(sed -n 's/^AGENTDECK_DEVELOPMENT_TEAM = //p' "$configuration")
expected_app_group="$development_team.group.com.kitdine.agentdeck"
if [[ -z $development_team || $app_group != "$expected_app_group" ]]; then
  echo "App Group must use the configured Developer Team prefix" >&2
  exit 1
fi

for entitlements in "$app_entitlements" "$widget_entitlements"; do
  plutil -lint "$entitlements" >/dev/null
  grep -Fq '<string>$(AGENTDECK_APP_GROUP)</string>' "$entitlements"
done

plist=$(plutil -p "$widget_entitlements")
key_count=$(printf '%s\n' "$plist" | grep -Ec '^  ".*" =>')
test "$key_count" -eq 2
printf '%s\n' "$plist" | grep -Fq '"com.apple.security.app-sandbox" => true'
printf '%s\n' "$plist" | grep -Fq '"com.apple.security.application-groups" => ['
printf '%s\n' "$plist" | grep -Fq '"$(AGENTDECK_APP_GROUP)"'

for info in "$app_info" "$widget_info"; do
  plutil -lint "$info" >/dev/null
  plist_app_group=$(plutil -extract AgentDeckAppGroupIdentifier raw -o - "$info")
  test "$plist_app_group" = '$(AGENTDECK_APP_GROUP)'
done

for source in "$shared_store" "$widget_reader"; do
  grep -Fq 'forInfoDictionaryKey: "AgentDeckAppGroupIdentifier"' "$source"
  if grep -Fq "$app_group" "$source"; then
    echo "App Group identity must be injected instead of compiled into Swift" >&2
    exit 1
  fi
done

if grep -REn --include='*.swift' \
  '((/|~)\.agentdeck/|(/|~)\.codex/|(/|~)\.claude/|sqlite3|credential\.key|session_id|working_directory|raw session)' \
  "$sources"; then
  echo "widget production source references a prohibited private-data path or field" >&2
  exit 1
fi

widget_target=$(awk '
  /A40000000000000000000005 \/\* AgentDeckWidget \*\/ = \{/ { inside=1 }
  inside { print }
  inside && /productType = "com.apple.product-type.app-extension";/ { exit }
' "$project")
test -n "$widget_target"
if printf '%s\n' "$widget_target" | grep -Fq 'AgentDeckShared.framework'; then
  echo "widget target must not link the host Shared framework" >&2
  exit 1
fi

widget_sources=$(awk '
  /A6000000000000000000000B \/\* Sources \*\/ = \{/ { inside=1 }
  inside { print }
  inside && /runOnlyForDeploymentPostprocessing = 0;/ { exit }
' "$project")
test -n "$widget_sources"
if printf '%s\n' "$widget_sources" | grep -Fq 'AppGroupSnapshotStore.swift'; then
  echo "widget target must compile the read-only snapshot reader, not the writable host store" >&2
  exit 1
fi

grep -Fq 'A6000000000000000000000C /* Embed App Extensions */' "$project"
grep -Fq 'A50000000000000000000005 /* PBXTargetDependency */' "$project"

echo "widget sandbox boundary: PASS"
