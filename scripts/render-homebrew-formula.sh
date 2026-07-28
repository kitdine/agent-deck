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
if [[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  formula_class=Agentdeck
elif [[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rc\.(0|[1-9][0-9]*)$ ]]; then
  formula_class=AgentdeckRc
else
  echo "Homebrew formula requires a stable or rc.N semantic version tag: $tag" >&2
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
awk \
  -v tag="$tag" \
  -v version="$version" \
  -v formula_class="$formula_class" \
  -v arm64_sha="$arm64_sha" \
  -v amd64_sha="$amd64_sha" \
  '{
    gsub(/@TAG@/, tag)
    gsub(/@VERSION@/, version)
    gsub(/@FORMULA_CLASS@/, formula_class)
    gsub(/@ARM64_SHA256@/, arm64_sha)
    gsub(/@AMD64_SHA256@/, amd64_sha)
    print
  }' \
  "$template" >"$temporary"
if grep -Eq '@(TAG|VERSION|FORMULA_CLASS|ARM64_SHA256|AMD64_SHA256)@' "$temporary"; then
  echo "formula template contains unresolved placeholders" >&2
  exit 1
fi
ruby -c "$temporary" >/dev/null
chmod 0644 "$temporary"
mv "$temporary" "$output"
