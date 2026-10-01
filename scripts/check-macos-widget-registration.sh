#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 1 ]]; then
  echo "usage: check-macos-widget-registration.sh <installed-app-path>" >&2
  exit 2
fi

extension="$1/Contents/PlugIns/AgentDeckWidget.appex"
[[ -d $extension ]] || { echo "installed Widget extension is missing: $extension" >&2; exit 1; }
identifier=$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$extension/Contents/Info.plist")
registration=$(/usr/bin/pluginkit -m -A -D -vv -i "$identifier")
if ! awk -v expected="$extension" '
  /^[[:space:]]*Path = / {
    path = $0
    sub(/^[[:space:]]*Path = /, "", path)
    paths++
    if (path == expected) matches++
  }
  END { exit !(paths == 1 && matches == 1) }
' <<<"$registration"; then
  echo "Widget must have exactly one registration at $extension" >&2
  printf '%s\n' "$registration" >&2
  exit 1
fi
