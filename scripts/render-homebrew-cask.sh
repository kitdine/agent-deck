#!/usr/bin/env bash
set -euo pipefail

# Renders the agentdeck-app Cask from a release tag and the desktop checksum
# file. It reads no network and publishes nothing; opening the tap pull request
# stays scripts/update-homebrew-tap-pr.sh's job and a separate authorization.

if [[ $# -ne 4 ]]; then
  echo "usage: render-homebrew-cask.sh <template> <version-tag> <checksums-file> <output>" >&2
  exit 2
fi

template=$1
tag=$2
checksums=$3
output=$4

if [[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  cask_token=agentdeck-app
  conflicting_casks='"agentdeck-app-rc"'
elif [[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rc\.(0|[1-9][0-9]*)$ ]]; then
  cask_token=agentdeck-app-rc
  conflicting_casks='"agentdeck-app"'
else
  echo "Homebrew cask requires a stable or rc.N semantic version tag: $tag" >&2
  exit 2
fi
if [[ ! -f $template || -L $template || ! -f $checksums || -L $checksums ]]; then
  echo "cask template and checksums must be regular files" >&2
  exit 2
fi

dmg="AgentDeck_${tag}_universal.dmg"
dmg_sha=$(awk -v filename="$dmg" '
  $2 == filename {
    if (found) exit 2
    found=1
    value=$1
  }
  END {
    if (!found) exit 1
    print value
  }
' "$checksums") || {
  echo "missing or duplicate checksum for $dmg" >&2
  exit 1
}
if [[ ! $dmg_sha =~ ^[0-9a-f]{64}$ ]]; then
  echo "release checksums must be lowercase SHA-256 values" >&2
  exit 1
fi

version=${tag#v}
temporary=$(mktemp "${output}.tmp.XXXXXX")
trap 'rm -f "$temporary"' EXIT
awk \
  -v tag="$tag" \
  -v version="$version" \
  -v cask_token="$cask_token" \
  -v conflicting_casks="$conflicting_casks" \
  -v dmg_sha="$dmg_sha" \
  '{
    gsub(/@TAG@/, tag)
    gsub(/@VERSION@/, version)
    gsub(/@CASK_TOKEN@/, cask_token)
    gsub(/@CONFLICTING_CASKS@/, conflicting_casks)
    gsub(/@DMG_SHA256@/, dmg_sha)
    print
  }' \
  "$template" >"$temporary"
if grep -Eq '@(TAG|VERSION|CASK_TOKEN|CONFLICTING_CASKS|DMG_SHA256)@' "$temporary"; then
  echo "cask template contains unresolved placeholders" >&2
  exit 1
fi
ruby -c "$temporary" >/dev/null
chmod 0644 "$temporary"
mv "$temporary" "$output"
