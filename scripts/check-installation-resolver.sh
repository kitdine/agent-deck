#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-resolver.XXXXXX")
trap 'rm -rf "$temporary"' EXIT
cd "$root"

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}
export GOMODCACHE=${GOMODCACHE:-/private/tmp/agent-deck-go-mod}
go_bin=${GO:-go}

# Keep symbols for this check. Release build-all strips symbols, so checking a
# stripped executable would make absence meaningless. Use the ordinary build
# recipe and a tagged positive control from the same source instead.
make DIST_DIR="$temporary/production" build >/dev/null
"$go_bin" tool nm "$temporary/production/agentdeck" >"$temporary/production.nm"
if grep -F 'installation.acceptanceAccountHome' "$temporary/production.nm" >/dev/null; then
  echo 'production executable contains the acceptance account resolver' >&2
  exit 1
fi
"$go_bin" build -mod=vendor -tags agentdeck_acceptance -o "$temporary/acceptance" ./cmd/agentdeck
"$go_bin" tool nm "$temporary/acceptance" >"$temporary/acceptance.nm"
grep -F 'installation.acceptanceAccountHome' "$temporary/acceptance.nm" >/dev/null
printf 'Installation resolver isolation: production absent, acceptance present.\n'
