#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: release-archive.sh <dist-dir> <version>" >&2
  exit 2
fi

dist_dir=$1
version=$2
if [[ -z $version || $version == *[^A-Za-z0-9._-]* ]]; then
  echo "invalid release version: $version" >&2
  exit 2
fi

for tool in cp gzip mktemp mv shasum tar touch; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required release tool not found: $tool" >&2
    exit 1
  fi
done

mkdir -p "$dist_dir"
staging=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-release.XXXXXX")
trap 'rm -rf "$staging"' EXIT

for arch in arm64 amd64; do
  source_binary="$dist_dir/agentdeck_darwin_$arch"
  if [[ ! -f $source_binary ]]; then
    echo "missing build-all output: $source_binary" >&2
    exit 1
  fi

  archive="agentdeck_${version}_darwin_${arch}.tar.gz"
  # build-all already links with -s -w, so the source binary is the stripped
  # artifact the size gate measures; packaging copies it verbatim without a
  # second strip pass that would diverge the shipped bytes from the gate.
  mkdir -p "$staging/$arch"
  cp -X "$source_binary" "$staging/$arch/agentdeck"
  chmod 0755 "$staging/$arch/agentdeck"
  touch -t 198001010000 "$staging/$arch/agentdeck"
  COPYFILE_DISABLE=1 tar -c --format ustar --uid 0 --gid 0 --uname root --gname wheel \
    -C "$staging/$arch" -f - agentdeck | gzip -n >"$staging/$archive"
done

arm64_archive="agentdeck_${version}_darwin_arm64.tar.gz"
amd64_archive="agentdeck_${version}_darwin_amd64.tar.gz"
checksum_file="agentdeck_${version}_checksums.txt"
(
  cd "$staging"
  shasum -a 256 "$arm64_archive" "$amd64_archive" >"$checksum_file"
)

mv "$staging/$arm64_archive" "$dist_dir/$arm64_archive"
mv "$staging/$amd64_archive" "$dist_dir/$amd64_archive"
mv "$staging/$checksum_file" "$dist_dir/$checksum_file"
