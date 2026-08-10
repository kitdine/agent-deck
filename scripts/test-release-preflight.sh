#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-release-preflight.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

checker="$root/scripts/check-release-preflight-workflows.rb"
preflight="$root/.github/workflows/release-preflight.yml"
release="$root/.github/workflows/release.yml"

ruby "$checker" "$preflight" "$release"

automatic="$temporary/automatic.yml"
sed 's/^  workflow_dispatch:/  push:/' "$preflight" >"$automatic"
if ruby "$checker" "$automatic" "$release" >/dev/null 2>&1; then
  echo "preflight checker accepted an automatic trigger" >&2
  exit 1
fi

without_l4="$temporary/without-l4.yml"
sed 's/make release-verify/make verify/' "$preflight" >"$without_l4"
if ruby "$checker" "$without_l4" "$release" >/dev/null 2>&1; then
  echo "preflight checker accepted a workflow without L4" >&2
  exit 1
fi

without_artifacts="$temporary/without-artifacts.yml"
sed 's/actions\/upload-artifact@v4/actions\/upload-artifact@v3/' "$preflight" >"$without_artifacts"
if ruby "$checker" "$without_artifacts" "$release" >/dev/null 2>&1; then
  echo "preflight checker accepted an obsolete artifact upload" >&2
  exit 1
fi

without_gate="$temporary/release-without-gate.yml"
sed 's/release-preflight.yml/missing-preflight.yml/' "$release" >"$without_gate"
if ruby "$checker" "$preflight" "$without_gate" >/dev/null 2>&1; then
  echo "preflight checker accepted a release without the exact workflow gate" >&2
  exit 1
fi

with_duplicate_l4="$temporary/release-with-duplicate-l4.yml"
sed 's/make release-artifact-verify/make release-verify/' "$release" >"$with_duplicate_l4"
if ruby "$checker" "$preflight" "$with_duplicate_l4" >/dev/null 2>&1; then
  echo "preflight checker accepted duplicate L4 in the release stage" >&2
  exit 1
fi

target_sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
run_id=12345
evidence_id=urn:ce:agent-deck:evidence:v0-4-0-release-candidate:l4:test
artifact_dir="$temporary/artifacts"
version="preflight-$target_sha"
mkdir -p "$artifact_dir"
printf 'arm64 artifact\n' >"$artifact_dir/agentdeck_${version}_darwin_arm64.tar.gz"
printf 'amd64 artifact\n' >"$artifact_dir/agentdeck_${version}_darwin_amd64.tar.gz"
(
  cd "$artifact_dir"
  shasum -a 256 \
    "agentdeck_${version}_darwin_arm64.tar.gz" \
    "agentdeck_${version}_darwin_amd64.tar.gz" \
    >"agentdeck_${version}_checksums.txt"
)

ruby "$root/scripts/release-preflight-manifest.rb" create \
  "$artifact_dir/release-preflight.json" \
  kitdine/agent-deck "$target_sha" "$run_id" "$evidence_id" \
  "$artifact_dir/agentdeck_${version}_checksums.txt"
ruby "$root/scripts/release-preflight-manifest.rb" verify \
  "$artifact_dir" kitdine/agent-deck "$target_sha" "$run_id" >/dev/null

printf 'tampered\n' >>"$artifact_dir/agentdeck_${version}_darwin_arm64.tar.gz"
if ruby "$root/scripts/release-preflight-manifest.rb" verify \
  "$artifact_dir" kitdine/agent-deck "$target_sha" "$run_id" >/dev/null 2>&1; then
  echo "preflight manifest accepted a tampered artifact" >&2
  exit 1
fi
