#!/usr/bin/env bash
set -euo pipefail

# Run the real acceptance entry with hostile inherited installer settings.
# "External" here is a sibling of its fixture, never a real installation.
root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-ownership-env.XXXXXX")
trap 'rm -rf "$temporary"' EXIT
temporary=$(cd "$temporary" && pwd -P)
external="$temporary/external targets"
mkdir -p "$temporary/fixture"
for directory in prefix bin data Applications links records home account; do
  mkdir -p "$external/$directory"
  printf 'untouched %s\n' "$directory" >"$external/$directory/sentinel"
done
printf '# untouched shell configuration\n' >"$external/shellrc"
cat >"$external/lsregister" <<'SH'
#!/bin/sh
printf 'unexpected registration\n' >>"$(dirname "$0")/shellrc"
exit 98
SH
chmod 0755 "$external/lsregister"

snapshot() {
  python3 - "$external" <<'PY'
import hashlib
import json
import os
import stat
import sys
from pathlib import Path

root = Path(sys.argv[1])
entries = {}
for path in [root, *sorted(root.rglob("*"))]:
    info = path.lstat()
    entry = [info.st_mode, info.st_ino, info.st_uid, info.st_gid,
             info.st_mtime_ns, info.st_ctime_ns]
    if stat.S_ISREG(info.st_mode):
        entry.append(hashlib.sha256(path.read_bytes()).hexdigest())
    elif stat.S_ISLNK(info.st_mode):
        entry.append(os.readlink(path))
    entries[str(path.relative_to(root))] = entry
print(json.dumps(entries, sort_keys=True, indent=2))
PY
}

snapshot >"$temporary/before.json"
status=0
env HOME="$external/home" AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$external/account" \
  PREFIX="$external/prefix" BINDIR="$external/bin" DATADIR="$external/data" \
  APPDIR="$external/Applications" BIN_LINK="$external/links/agentdeck" RECORD_DIR="$external/records" \
  FORCE=1 TAKEOVER=1 COMPLETION_SHELL=bash COMPLETION_RC="$external/shellrc" \
  LSREGISTER="$external/lsregister" TMPDIR="$temporary/fixture" \
  bash "$root/scripts/test-ownership-acceptance.sh" || status=$?
snapshot >"$temporary/after.json"
if ! cmp -s "$temporary/before.json" "$temporary/after.json"; then
  echo 'FAIL inherited installation targets or sentinels changed outside the acceptance fixture' >&2
  diff -u "$temporary/before.json" "$temporary/after.json" >&2 || :
  exit 1
fi
[ "$status" = 0 ] || exit "$status"
if [ -n "$(find "$temporary/fixture" -mindepth 1 -print -quit)" ]; then
  echo 'FAIL acceptance left artifacts outside its cleaned temporary directory' >&2
  exit 1
fi
echo 'ok   polluted installer environment: external tree unchanged, fixture artifacts cleaned'
