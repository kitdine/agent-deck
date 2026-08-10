#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: verify-release-artifacts.sh <dist-dir> <version> <commit>" >&2
  exit 2
fi

dist_dir=$1
version=$2
commit=$3

if [[ -z $version || $version == *[^A-Za-z0-9._-]* ]]; then
  echo "invalid release version: $version" >&2
  exit 2
fi
if [[ ! $commit =~ ^[0-9a-f]{40}$ ]]; then
  echo "invalid release commit: $commit" >&2
  exit 2
fi
if [[ ! -d $dist_dir ]]; then
  echo "release directory does not exist: $dist_dir" >&2
  exit 1
fi

for tool in file go grep mktemp shasum strings tar; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required release verification tool not found: $tool" >&2
    exit 1
  fi
done

dist_dir=$(cd "$dist_dir" && pwd)
checksum_file="$dist_dir/agentdeck_${version}_checksums.txt"
arm64_archive="agentdeck_${version}_darwin_arm64.tar.gz"
amd64_archive="agentdeck_${version}_darwin_amd64.tar.gz"

test -f "$checksum_file"
test "$(wc -l <"$checksum_file" | tr -d ' ')" -eq 2
(
  cd "$dist_dir"
  shasum -a 256 -c "$(basename "$checksum_file")"
)

temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-release-artifacts.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

for arch in arm64 amd64; do
  archive_name="agentdeck_${version}_darwin_${arch}.tar.gz"
  archive="$dist_dir/$archive_name"
  test -f "$archive"

  listing=$(tar -tzf "$archive")
  if [[ $listing != agentdeck ]]; then
    echo "release archive $archive_name must contain exactly agentdeck" >&2
    exit 1
  fi

  extracted="$temporary/$arch"
  mkdir -p "$extracted"
  tar -xzf "$archive" -C "$extracted"
  binary="$extracted/agentdeck"
  test -x "$binary"

  binary_type=$(file "$binary")
  if [[ $arch == arm64 ]]; then
    grep -F "arm64" <<<"$binary_type" >/dev/null
  else
    grep -E "x86_64|x86-64" <<<"$binary_type" >/dev/null
  fi

  build_metadata=$(go version -m "$binary")
  grep -F "GOOS=darwin" <<<"$build_metadata" >/dev/null
  grep -F "GOARCH=$arch" <<<"$build_metadata" >/dev/null
  string_table="$extracted/strings.txt"
  strings -a "$binary" >"$string_table"
  grep -Fx "$version" "$string_table" >/dev/null
  grep -Fx "$commit" "$string_table" >/dev/null
done

if [[ $(uname -m) == arm64 ]]; then
  native_binary="$temporary/arm64/agentdeck"
  identity=$("$native_binary" --format json version)
  grep -F "\"version\":\"$version\"" <<<"$identity" >/dev/null
  grep -F "\"commit\":\"$commit\"" <<<"$identity" >/dev/null
  grep -E '"build_time":"[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}"' \
    <<<"$identity" >/dev/null

  install_home="$temporary/home"
  install_prefix="$temporary/prefix"
  mkdir -p "$install_home"
  HOME="$install_home" PREFIX="$install_prefix" \
    BINDIR="$install_prefix/bin" DATADIR="$install_prefix/share/agentdeck" \
    COMPLETION_SHELL=none bash scripts/manage-install.sh install "$native_binary" >/dev/null
  installed_identity=$("$install_prefix/bin/agentdeck" --format json version)
  grep -F "\"version\":\"$version\"" <<<"$installed_identity" >/dev/null
  grep -F "\"commit\":\"$commit\"" <<<"$installed_identity" >/dev/null
  HOME="$install_home" PREFIX="$install_prefix" \
    BINDIR="$install_prefix/bin" DATADIR="$install_prefix/share/agentdeck" \
    bash scripts/manage-install.sh uninstall >/dev/null
  test ! -e "$install_prefix/bin/agentdeck"
fi

printf 'verified release artifacts %s for %s (%s, %s)\n' \
  "$version" "$commit" "$arm64_archive" "$amd64_archive"
