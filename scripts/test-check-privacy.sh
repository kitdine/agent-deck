#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-privacy-boundary.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

mkdir -p "$temporary/repo/docs"
git -C "$temporary/repo" init -q

# These identifiers contain the letters "sk-" but are not credential tokens.
printf 'ca%s%s\n' 'sk-' 'preflight-deprecated.md' >"$temporary/repo/docs/cask.md"
printf 'ta%s%s\n' 'sk-' 'abcdef0123456789abcdef0123456789' >"$temporary/repo/docs/task.md"
if ! (cd "$temporary/repo" && bash "$root/scripts/check-privacy.sh") >"$temporary/output" 2>&1; then
  echo "privacy scan rejected ordinary cask/task identifiers" >&2
  exit 1
fi

printf '%s%s\n' 'sk-' 'abcdefghijklmnopqrstuvwxyz' >"$temporary/repo/docs/token.md"
if (cd "$temporary/repo" && bash "$root/scripts/check-privacy.sh") >"$temporary/output" 2>&1; then
  echo "privacy scan accepted a token-shaped secret" >&2
  exit 1
fi
if ! grep -Fq 'docs/token.md' "$temporary/output"; then
  echo "privacy scan did not identify the token-bearing file" >&2
  exit 1
fi

echo "privacy token boundary PASS"
