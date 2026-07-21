#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: release-notes-from-tag.sh <remote> <version-tag> <expected-commit> <output>" >&2
  exit 2
fi

remote=$1
tag=$2
expected_commit=$3
output=$4
if [[ ! $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "invalid release tag: $tag" >&2
  exit 2
fi
if [[ ! $expected_commit =~ ^[0-9a-fA-F]{40}$ ]]; then
  echo "invalid expected release commit: $expected_commit" >&2
  exit 2
fi

private_ref="refs/agentdeck-release-tags/$tag"
temporary=$(mktemp "${output}.tmp.XXXXXX")
cleanup() {
  git update-ref -d "$private_ref" >/dev/null 2>&1 || :
  rm -f "$temporary"
}
trap cleanup EXIT

# actions/checkout checks out the peeled commit for a tag event and may leave
# refs/tags/<tag> as a lightweight ref. Fetch the remote tag object into a
# private ref so release-note extraction cannot silently fall back to the
# commit message.
git fetch --force --no-tags "$remote" "refs/tags/$tag:$private_ref" >/dev/null
if [[ $(git cat-file -t "$private_ref") != tag ]]; then
  echo "release tag is not annotated: $tag" >&2
  exit 1
fi
actual_commit=$(git rev-parse "$private_ref^{}")
if [[ $actual_commit != $expected_commit ]]; then
  echo "release tag commit mismatch: expected $expected_commit, got $actual_commit" >&2
  exit 1
fi

git cat-file tag "$private_ref" | sed '1,/^$/d' >"$temporary"
if [[ ! -s $temporary ]] || ! grep -Eq '^## [^#[:space:]]' "$temporary"; then
  echo "annotated tag does not contain structured release notes: $tag" >&2
  exit 1
fi
mv "$temporary" "$output"
