#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: render-homebrew-formula.sh <template> <version-tag> <checksums-file> <output>" >&2
  exit 2
fi

template=$1
tag=$2
checksums=$3
output=$4
if [[ ! $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "Homebrew formula requires a stable semantic version tag: $tag" >&2
  exit 2
fi
if [[ ! -f $template || -L $template || ! -f $checksums || -L $checksums ]]; then
  echo "formula template and checksums must be regular files" >&2
  exit 2
fi

checksum_for() {
  local filename=$1
  awk -v filename="$filename" '
    $2 == filename {
      if (found) exit 2
      found=1
      value=$1
    }
    END {
      if (!found) exit 1
      print value
    }
  ' "$checksums"
}

arm64_archive="agentdeck_${tag}_darwin_arm64.tar.gz"
amd64_archive="agentdeck_${tag}_darwin_amd64.tar.gz"
arm64_sha=$(checksum_for "$arm64_archive") || {
  echo "missing or duplicate checksum for $arm64_archive" >&2
  exit 1
}
amd64_sha=$(checksum_for "$amd64_archive") || {
  echo "missing or duplicate checksum for $amd64_archive" >&2
  exit 1
}
if [[ ! $arm64_sha =~ ^[0-9a-f]{64}$ || ! $amd64_sha =~ ^[0-9a-f]{64}$ ]]; then
  echo "release checksums must be lowercase SHA-256 values" >&2
  exit 1
fi

version=${tag#v}
temporary=$(mktemp "${output}.tmp.XXXXXX")
trap 'rm -f "$temporary"' EXIT
sed \
  -e "s/@TAG@/$tag/g" \
  -e "s/@VERSION@/$version/g" \
  -e "s/@ARM64_SHA256@/$arm64_sha/g" \
  -e "s/@AMD64_SHA256@/$amd64_sha/g" \
  "$template" >"$temporary"
if grep -Eq '@(TAG|VERSION|ARM64_SHA256|AMD64_SHA256)@' "$temporary"; then
  echo "formula template contains unresolved placeholders" >&2
  exit 1
fi
ruby -c "$temporary" >/dev/null
chmod 0644 "$temporary"
mv "$temporary" "$output"
