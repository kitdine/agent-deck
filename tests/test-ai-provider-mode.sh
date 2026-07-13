#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/bin/ai-provider-mode"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_contains() { grep -Fq "$2" "$1" || fail "$1 missing: $2"; }
assert_not_contains() { ! grep -Fq "$2" "$1" || fail "$1 leaked: $2"; }
file_mode() { python3 -c 'import os,stat,sys; print(oct(stat.S_IMODE(os.stat(sys.argv[1]).st_mode)))' "$1"; }
assert_valid_toml() { python3 -c 'import sys,tomllib; tomllib.load(open(sys.argv[1], "rb"))' "$1" || fail "$1 is not valid TOML"; }

new_home() {
  local name="$1"
  local home="$TMP/$name"
  mkdir -p "$home/.codex" "$home/.config/ai-provider-mode"
  cat >"$home/.codex/config.toml" <<'TOML'
model_provider = "legacy"
model = "gpt-test"
model_reasoning_effort = "high"

[model_providers]
[model_providers.custom] # legacy managed block
name = "Legacy"
base_url = "http://127.0.0.1:1/v1"
requires_openai_auth = true

[features] # keep inline comment
memories = true
TOML
  cat >"$home/.codex/auth.json" <<'JSON'
{"auth_mode":"chatgpt","tokens":{"access_token":"unchanged"}}
JSON
  cat >"$home/.config/ai-provider-mode/providers.json" <<'JSON'
{
  "providers": {
    "official": {
      "host": "http://10.0.0.103:15021",
      "codex": {"auth": "openai_login"},
      "claude": {"enabled": false}
    },
    "aigocode": {
      "host": "http://10.0.0.103:15022",
      "codex": {"auth": "bearer", "keys": ["aigocode-codex-work"], "default_key": "aigocode-codex-work"},
      "claude": {"enabled": false}
    }
  },
  "keys": {"aigocode-codex-work": {"value": "test-aigocode-secret"}}
}
JSON
  chmod 600 "$home/.config/ai-provider-mode/providers.json"
  printf '%s\n' "$home"
}

home="$(new_home happy)"
auth_before="$(shasum -a 256 "$home/.codex/auth.json" | awk '{print $1}')"
config_mode_before="$(file_mode "$home/.codex/config.toml")"

HOME="$home" "$SCRIPT" codex official >"$home/official.out"
assert_contains "$home/.codex/config.toml" 'model_provider = "custom"'
assert_contains "$home/.codex/config.toml" 'base_url = "http://10.0.0.103:15021/v1"'
assert_not_contains "$home/.codex/config.toml" '[model_providers.custom] # legacy managed block'
[[ "$(grep -c '^\[model_providers\.custom\]' "$home/.codex/config.toml")" -eq 1 ]] || fail 'custom provider table was duplicated'
assert_not_contains "$home/.codex/config.toml" 'experimental_bearer_token'
assert_contains "$home/.codex/config.toml" 'model_reasoning_effort = "high"'
assert_contains "$home/.codex/config.toml" '[features] # keep inline comment'

HOME="$home" "$SCRIPT" codex aigocode >"$home/aigocode.out"
assert_contains "$home/.codex/config.toml" 'base_url = "http://10.0.0.103:15022/v1"'
assert_contains "$home/.codex/config.toml" 'experimental_bearer_token = "test-aigocode-secret"'
assert_not_contains "$home/aigocode.out" 'test-aigocode-secret'

HOME="$home" "$SCRIPT" status >"$home/status.out"
assert_contains "$home/status.out" 'codex: aigocode'
assert_not_contains "$home/status.out" 'test-aigocode-secret'

config_before="$(shasum -a 256 "$home/.codex/config.toml" | awk '{print $1}')"
backup_count_before="$(find "$home/.config/ai-provider-mode/backups/codex" -type f | wc -l | tr -d ' ')"
HOME="$home" "$SCRIPT" codex aigocode >"$home/idempotent.out"
config_after="$(shasum -a 256 "$home/.codex/config.toml" | awk '{print $1}')"
backup_count_after="$(find "$home/.config/ai-provider-mode/backups/codex" -type f | wc -l | tr -d ' ')"
[[ "$config_before" == "$config_after" ]] || fail 'idempotent switch changed config'
[[ "$backup_count_before" == "$backup_count_after" ]] || fail 'idempotent switch created backup'

HOME="$home" "$SCRIPT" codex official >"$home/official-again.out"
for backup in "$home"/.config/ai-provider-mode/backups/codex/*.toml; do
  assert_not_contains "$backup" 'test-aigocode-secret'
  [[ "$(file_mode "$backup")" == "0o600" ]] || fail "$backup mode is not 0600"
  assert_valid_toml "$backup"
done
assert_valid_toml "$home/.codex/config.toml"
[[ "$auth_before" == "$(shasum -a 256 "$home/.codex/auth.json" | awk '{print $1}')" ]] || fail 'auth.json changed'
[[ "$config_mode_before" == "$(file_mode "$home/.codex/config.toml")" ]] || fail 'config mode changed'

quoted_home="$(new_home quoted-custom)"
cat >"$quoted_home/.codex/config.toml" <<'TOML'
model_provider = "legacy"

[model_providers."custom"] # quoted semantic custom table
name = "Quoted Legacy"
base_url = "http://127.0.0.1:2/v1"
"experimental_bearer_token" = """
[features]
leaked = "multiline-span-secret"
[model_providers.custom.shadow]
"""

[other]
keep = true
TOML
HOME="$quoted_home" "$SCRIPT" codex official >"$quoted_home/official.out"
assert_contains "$quoted_home/.codex/config.toml" '[model_providers.custom]'
assert_not_contains "$quoted_home/.codex/config.toml" 'multiline-span-secret'
assert_contains "$quoted_home/.codex/config.toml" '[other]'
assert_valid_toml "$quoted_home/.codex/config.toml"
for backup in "$quoted_home"/.config/ai-provider-mode/backups/codex/*.toml; do
  assert_not_contains "$backup" 'multiline-span-secret'
  [[ "$(file_mode "$backup")" == "0o600" ]] || fail "$backup mode is not 0600"
  assert_valid_toml "$backup"
done

literal_home="$(new_home literal-multiline)"
cat >"$literal_home/.codex/config.toml" <<'TOML'
model_provider = "custom"

[model_providers.custom]
name = "Literal Legacy"
base_url = "http://127.0.0.1:5/v1"
'experimental_bearer_token' = '''
[features]
leaked = "literal-span-secret"
'''

[other]
keep = true
TOML
HOME="$literal_home" "$SCRIPT" codex official >"$literal_home/official.out"
assert_not_contains "$literal_home/.codex/config.toml" 'literal-span-secret'
assert_contains "$literal_home/.codex/config.toml" '[other]'
assert_valid_toml "$literal_home/.codex/config.toml"

duplicate_home="$(new_home duplicate-custom)"
cat >"$duplicate_home/.codex/config.toml" <<'TOML'
model_provider = "legacy"
[model_providers.custom]
name = "First"
[model_providers."custom"]
name = "Second"
TOML
before="$(shasum -a 256 "$duplicate_home/.codex/config.toml" | awk '{print $1}')"
if HOME="$duplicate_home" "$SCRIPT" codex official >"$duplicate_home/out" 2>&1; then fail 'duplicate semantic custom tables succeeded'; fi
[[ "$before" == "$(shasum -a 256 "$duplicate_home/.codex/config.toml" | awk '{print $1}')" ]] || fail 'config changed after duplicate custom failure'
[[ ! -e "$duplicate_home/.config/ai-provider-mode/backups/codex" ]] || fail 'backup created after duplicate custom failure'

non_table_home="$(new_home non-table-custom)"
cat >"$non_table_home/.codex/config.toml" <<'TOML'
model_provider = "legacy"
model_providers = { custom = [] }
TOML
before="$(shasum -a 256 "$non_table_home/.codex/config.toml" | awk '{print $1}')"
if HOME="$non_table_home" "$SCRIPT" codex official >"$non_table_home/out" 2>&1; then fail 'non-table semantic custom succeeded'; fi
[[ "$before" == "$(shasum -a 256 "$non_table_home/.codex/config.toml" | awk '{print $1}')" ]] || fail 'config changed after non-table custom failure'

bad_perm_home="$(new_home bad-perm)"
chmod 644 "$bad_perm_home/.config/ai-provider-mode/providers.json"
before="$(shasum -a 256 "$bad_perm_home/.codex/config.toml" | awk '{print $1}')"
if HOME="$bad_perm_home" "$SCRIPT" codex aigocode >"$bad_perm_home/out" 2>&1; then fail 'permissive provider file succeeded'; fi
[[ "$before" == "$(shasum -a 256 "$bad_perm_home/.codex/config.toml" | awk '{print $1}')" ]] || fail 'config changed after permission failure'

sentinel_home="$(new_home sentinel)"
sed -i.bak 's/test-aigocode-secret/replace-me/' "$sentinel_home/.config/ai-provider-mode/providers.json"
if HOME="$sentinel_home" "$SCRIPT" codex aigocode >"$sentinel_home/out" 2>&1; then fail 'sentinel token succeeded'; fi

malformed_home="$(new_home malformed)"
printf '{' >"$malformed_home/.config/ai-provider-mode/providers.json"
chmod 600 "$malformed_home/.config/ai-provider-mode/providers.json"
if HOME="$malformed_home" "$SCRIPT" codex official >"$malformed_home/out" 2>&1; then fail 'malformed JSON succeeded'; fi

missing_home="$(new_home missing)"
rm "$missing_home/.config/ai-provider-mode/providers.json"
if HOME="$missing_home" "$SCRIPT" codex official >"$missing_home/out" 2>&1; then fail 'missing provider config succeeded'; fi

nonregular_home="$(new_home nonregular)"
rm "$nonregular_home/.config/ai-provider-mode/providers.json"
mkdir "$nonregular_home/.config/ai-provider-mode/providers.json"
if HOME="$nonregular_home" "$SCRIPT" codex official >"$nonregular_home/out" 2>&1; then fail 'non-regular provider config succeeded'; fi

incomplete_home="$(new_home incomplete)"
printf '{"providers":{"official":{}},"keys":{}}' >"$incomplete_home/.config/ai-provider-mode/providers.json"
chmod 600 "$incomplete_home/.config/ai-provider-mode/providers.json"
if HOME="$incomplete_home" "$SCRIPT" codex official >"$incomplete_home/out" 2>&1; then fail 'incomplete provider config succeeded'; fi

bad_mode_home="$(new_home bad-mode)"
if HOME="$bad_mode_home" "$SCRIPT" unknown >"$bad_mode_home/out" 2>&1; then fail 'unknown mode succeeded'; fi
if HOME="$bad_mode_home" "$SCRIPT" codex official --unknown >"$bad_mode_home/extra.out" 2>&1; then fail 'unknown extra argument succeeded'; fi

printf 'PASS: ai-provider-mode isolated suite\n'
