#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: create-release-tag.sh <version-tag> <release-notes-file>" >&2
  exit 2
fi

tag=$1
notes=$2
if [[ ! $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "invalid release tag: $tag" >&2
  exit 2
fi
if [[ ! -f $notes || -L $notes || ! -s $notes ]]; then
  echo "release notes must be a non-empty regular file: $notes" >&2
  exit 2
fi
if ! grep -Eq '^## [^#[:space:]]' "$notes"; then
  echo "release notes must contain at least one level-two markdown heading" >&2
  exit 2
fi
if git rev-parse --quiet --verify "refs/tags/$tag" >/dev/null; then
  echo "release tag already exists: $tag" >&2
  exit 1
fi
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "refusing to tag a repository with tracked changes" >&2
  exit 1
fi

temporary=$(mktemp "${TMPDIR:-/tmp}/agentdeck-tag-notes.XXXXXX")
created=0
cleanup() {
  rm -f "$temporary"
  if [[ $created -eq 1 ]]; then
    git tag --delete "$tag" >/dev/null 2>&1 || :
  fi
}
trap cleanup EXIT

git tag --annotate --no-sign --cleanup=verbatim --file "$notes" "$tag"
created=1
git cat-file tag "refs/tags/$tag" | sed '1,/^$/d' >"$temporary"
if ! cmp "$notes" "$temporary"; then
  echo "created tag annotation does not exactly match release notes" >&2
  exit 1
fi

created=0
echo "created annotated tag $tag with verbatim release notes"
