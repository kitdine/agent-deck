#!/usr/bin/env bash
# Build the real old baseline and the candidate binaries for
# TestCJSISourceIssueAcceptance in an isolated directory, then print the
# environment the external-binary submatrix needs.
#
#   cmd/agentdeck/testdata/cjsi_acceptance/build-binaries.sh <out-dir> [candidate-commit]
#   cmd/agentdeck/testdata/cjsi_acceptance/build-binaries.sh --self-test <scratch-dir>
#
# The old binary always comes from `git archive` of the baseline commit, never
# from a checkout switch. Without candidate-commit the candidate is built from
# the current working tree. Nothing is installed and no AgentDeck command runs.
# The output directory must not resolve into /tmp (or /private/tmp): the path
# is canonicalized through its nearest existing ancestor before anything is
# created, and the created directory is resolved and checked again.
set -euo pipefail

old_commit=4ac56db32e13c4e925b648c620ba76fc588a7781

# cjsi_canonical_path prints the physical absolute path of $1, which may not
# exist yet. The nearest existing ancestor is resolved with `pwd -P` (so `..`,
# `//`, symlinks and the /tmp -> /private/tmp link are all resolved); the
# remaining, not yet existing components may not be `.` or `..`.
cjsi_canonical_path() {
  local path=$1 rest="" base resolved
  case "$path" in
    "") return 1 ;;
    /*) ;;
    *) path="$PWD/$path" ;;
  esac
  # POSIX lets a leading "//" stay distinct, and pwd -P keeps it; collapse
  # every run of slashes so "//tmp" cannot escape the /tmp check.
  path=$(printf '%s' "$path" | tr -s /)
  while [ ! -d "$path" ]; do
    if [ -e "$path" ] || [ -L "$path" ]; then
      return 1 # an existing non-directory (or dangling link) cannot be the output
    fi
    base=$(basename -- "$path")
    case "$base" in
      . | .. | /) return 1 ;;
    esac
    rest="/$base$rest"
    path=$(dirname -- "$path")
  done
  resolved=$(cd -P -- "$path" && pwd -P) || return 1
  resolved=$(printf '%s' "$resolved" | tr -s /)
  resolved=${resolved%/}
  printf '%s%s\n' "$resolved" "$rest"
}

# cjsi_is_tmp reports whether $1, or any existing ancestor of it, is the
# same directory as /tmp or /private/tmp by identity (device and inode via
# `-ef`), so case-folded spellings (/PRIVATE/TMP) and firmlinked volume paths
# (/System/Volumes/Data/private/tmp) are caught as well as the plain names.
cjsi_is_tmp() {
  local dir=$1
  case "$dir" in
    /tmp | /tmp/* | /private/tmp | /private/tmp/*) return 0 ;;
  esac
  while [ ! -d "$dir" ]; do
    dir=$(dirname -- "$dir")
  done
  while :; do
    if [ "$dir" -ef /tmp ] || [ "$dir" -ef /private/tmp ]; then
      return 0
    fi
    [ "$dir" = / ] && break
    dir=$(dirname -- "$dir")
  done
  return 1
}

# cjsi_checked_out prints the canonical output path or fails if it is
# unresolvable or inside /tmp. It creates nothing.
cjsi_checked_out() {
  local canonical
  if ! canonical=$(cjsi_canonical_path "$1"); then
    printf 'cannot resolve output directory: %s\n' "$1" >&2
    return 2
  fi
  if cjsi_is_tmp "$canonical"; then
    printf 'refusing a /tmp output directory: %s (resolves to %s)\n' "$1" "$canonical" >&2
    return 2
  fi
  printf '%s\n' "$canonical"
}

cjsi_self_test() {
  local scratch=${1:?usage: build-binaries.sh --self-test <scratch-dir>} failures=0 case_path
  scratch=$(cjsi_checked_out "$scratch") || return 2
  mkdir -p "$scratch"
  ln -sfn /tmp "$scratch/link-to-tmp"
  ln -sfn /private/tmp "$scratch/link-to-private-tmp"
  expect_refused() {
    local label=$1; shift
    if (cd "$1" && cjsi_checked_out "$2" >/dev/null 2>&1); then
      printf 'FAIL refused case accepted: %s\n' "$label"
      failures=$((failures + 1))
    else
      printf 'ok   refused: %s\n' "$label"
    fi
  }
  expect_refused "/tmp" / /tmp
  expect_refused "/tmp/x" / /tmp/x
  expect_refused "/private/tmp" / /private/tmp
  expect_refused "/private/tmp/x/y" / /private/tmp/x/y
  expect_refused "//tmp/x" / //tmp/x
  expect_refused "///private//tmp/x" / ///private//tmp/x
  expect_refused "relative x from cwd //tmp" //tmp x
  expect_refused "/var/../tmp/x" / /var/../tmp/x
  expect_refused "/tmp/./x" / /tmp/./x
  expect_refused "relative x from cwd /tmp" /tmp x
  expect_refused "relative ../tmp/x from cwd /var" /var ../tmp/x
  expect_refused "symlink to /tmp" / "$scratch/link-to-tmp"
  expect_refused "symlink to /tmp plus child" / "$scratch/link-to-tmp/x/y"
  expect_refused "symlink to /private/tmp plus child" / "$scratch/link-to-private-tmp/x"
  expect_refused "non-existing .. component" / "$scratch/missing/../x"
  expect_refused "/PRIVATE/TMP (case alias)" / /PRIVATE/TMP
  expect_refused "/PRIVATE/TMP/x (case alias)" / /PRIVATE/TMP/x
  expect_refused "/Private/Tmp/x/y (case alias)" / /Private/Tmp/x/y
  expect_refused "/TMP/x (case alias of the link)" / /TMP/x
  if [ -d /System/Volumes/Data/private/tmp ]; then
    expect_refused "/System/Volumes/Data/private/tmp (volume alias)" / /System/Volumes/Data/private/tmp
    expect_refused "/System/Volumes/Data/private/tmp/x (volume alias)" / /System/Volumes/Data/private/tmp/x
  fi
  expect_refused "relative x from cwd /PRIVATE/TMP" /PRIVATE/TMP x
  case_path="$scratch/accepted/child"
  if [ "$(cjsi_checked_out "$case_path")" = "$scratch/accepted/child" ]; then
    printf 'ok   accepted: %s\n' "$case_path"
  else
    printf 'FAIL accepted case refused: %s\n' "$case_path"
    failures=$((failures + 1))
  fi
  if [ -e "$scratch/accepted" ]; then
    printf 'FAIL the check created %s\n' "$scratch/accepted"
    failures=$((failures + 1))
  fi
  rm -f "$scratch/link-to-tmp" "$scratch/link-to-private-tmp"
  printf 'self-test failures=%d\n' "$failures"
  [ "$failures" -eq 0 ]
}

if [ "${1:-}" = "--self-test" ]; then
  shift
  cjsi_self_test "$@"
  exit $?
fi

requested=${1:?usage: build-binaries.sh <out-dir> [candidate-commit]}
candidate_commit=${2:-}

out=$(cjsi_checked_out "$requested")
mkdir -p -- "$out"
# Resolve again after creation: a concurrent swap or link must not divert it.
created=$(cd -P -- "$out" && pwd -P | tr -s /)
if [ "$created" != "$out" ] || ! [ "$created" -ef "$out" ] || cjsi_is_tmp "$created"; then
  printf 'output directory changed while it was created: %s -> %s\n' "$out" "$created" >&2
  exit 2
fi

repo=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
work=$(mktemp -d "$out/build.XXXXXX")
trap 'rm -rf "$work"' EXIT

export GOWORK=off GOFLAGS=-mod=vendor GOPROXY=off
export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}

mkdir -p "$work/old-src"
git -C "$repo" archive --format=tar "$old_commit" | tar -xf - -C "$work/old-src"
(cd "$work/old-src" && go build -mod=vendor -trimpath -o "$out/agentdeck-old" ./cmd/agentdeck)

if [ -n "$candidate_commit" ]; then
  mkdir -p "$work/candidate-src"
  git -C "$repo" archive --format=tar "$candidate_commit" | tar -xf - -C "$work/candidate-src"
  (cd "$work/candidate-src" && go build -mod=vendor -trimpath -o "$out/agentdeck-candidate" ./cmd/agentdeck)
else
  (cd "$repo" && go build -mod=vendor -trimpath -o "$out/agentdeck-candidate" ./cmd/agentdeck)
fi

shasum -a 256 "$out/agentdeck-old" "$out/agentdeck-candidate"
printf 'export CJSI_OLD_BINARY=%q\n' "$out/agentdeck-old"
printf 'export CJSI_CANDIDATE_BINARY=%q\n' "$out/agentdeck-candidate"
