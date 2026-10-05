#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 1 ]]; then
  echo "usage: check-macos-widget-intents.sh <app-path>" >&2
  exit 2
fi

# WidgetKit's serialized configuration identifies the containing app. Both
# executable bundles must advertise the same configuration intent identifiers.
for bundle in "$1" "$1/Contents/PlugIns/AgentDeckWidget.appex"; do
  metadata="$bundle/Contents/Resources/Metadata.appintents/extract.actionsdata"
  for intent in ClientPeriodWidgetIntent ClientWidgetIntent QuotaWidgetIntent; do
    identifier=$(/usr/bin/plutil -extract "actions.$intent.identifier" raw -o - "$metadata" 2>/dev/null) || {
      echo "missing Widget configuration intent $intent in $bundle" >&2
      exit 1
    }
    [[ $identifier == "$intent" ]] || exit 1
  done
done
