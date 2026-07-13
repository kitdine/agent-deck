#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/bin/codex-provider-mode"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_contains() { grep -Fq "$2" "$1" || fail "$1 missing: $2"; }
assert_not_contains() { ! grep -Fq "$2" "$1" || fail "$1 leaked: $2"; }

new_home() {
  local name="$1"
  local home="$TMP/$name"
  mkdir -p "$home/.codex" "$home/.config/codex-provider-mode"
  cat >"$home/.codex/config.toml" <<'TOML'
model_provider = "legacy"
model = "gpt-5.6-sol"
model_reasoning_effort = "high"
service_tier = "default"

[model_providers]
[model_providers.custom] # legacy managed block
name = "Legacy"
base_url = "http://127.0.0.1:1/v1"
requires_openai_auth = true
wire_api = "responses"

[features] # keep inline comment
memories = true
TOML
  cat >"$home/.codex/auth.json" <<'JSON'
{"auth_mode":"chatgpt","tokens":{"access_token":"unchanged"}}
JSON
  cat >"$home/.config/codex-provider-mode/providers.json" <<'JSON'
{
  "official": {
    "name": "OpenAI Official via Headroom",
    "base_url": "http://10.0.0.103:15021/v1"
  },
  "aigocode": {
    "name": "AIGocode via Headroom",
    "base_url": "http://10.0.0.103:15022/v1",
    "bearer_token": "test-aigocode-secret"
  }
}
JSON
  chmod 600 "$home/.config/codex-provider-mode/providers.json"
  printf '%s\n' "$home"
}

home="$(new_home happy)"
auth_before="$(shasum -a 256 "$home/.codex/auth.json" | awk '{print $1}')"
config_mode_before="$(python3 -c 'import os,stat,sys; print(oct(stat.S_IMODE(os.stat(sys.argv[1]).st_mode)))' "$home/.codex/config.toml")"

HOME="$home" "$SCRIPT" official >"$home/official.out"
assert_contains "$home/.codex/config.toml" 'model_provider = "custom"'
assert_contains "$home/.codex/config.toml" 'base_url = "http://10.0.0.103:15021/v1"'
assert_not_contains "$home/.codex/config.toml" '[model_providers.custom] # legacy managed block'
[[ "$(grep -c '^\[model_providers\.custom\]' "$home/.codex/config.toml")" -eq 1 ]] || fail 'custom provider table was duplicated'
assert_not_contains "$home/.codex/config.toml" 'experimental_bearer_token'
assert_contains "$home/.codex/config.toml" 'model_reasoning_effort = "high"'
assert_contains "$home/.codex/config.toml" '[features] # keep inline comment'
assert_contains "$home/.codex/config.toml" 'memories = true'

HOME="$home" "$SCRIPT" aigocode >"$home/aigocode.out"
assert_contains "$home/.codex/config.toml" 'base_url = "http://10.0.0.103:15022/v1"'
assert_contains "$home/.codex/config.toml" 'requires_openai_auth = true'
assert_contains "$home/.codex/config.toml" 'experimental_bearer_token = "test-aigocode-secret"'
assert_not_contains "$home/aigocode.out" 'test-aigocode-secret'

HOME="$home" "$SCRIPT" status >"$home/status.out"
assert_contains "$home/status.out" 'Mode: aigocode'
assert_contains "$home/status.out" 'Token configured: yes'
assert_not_contains "$home/status.out" 'test-aigocode-secret'

config_before="$(shasum -a 256 "$home/.codex/config.toml" | awk '{print $1}')"
backup_count_before="$(find "$home/.backup-codex-config" -type f | wc -l | tr -d ' ')"
HOME="$home" "$SCRIPT" aigocode >"$home/idempotent.out"
config_after="$(shasum -a 256 "$home/.codex/config.toml" | awk '{print $1}')"
backup_count_after="$(find "$home/.backup-codex-config" -type f | wc -l | tr -d ' ')"
[[ "$config_before" == "$config_after" ]] || fail 'idempotent switch changed config'
[[ "$backup_count_before" == "$backup_count_after" ]] || fail 'idempotent switch created backup'

HOME="$home" "$SCRIPT" official >"$home/official-again.out"
for backup in "$home"/.backup-codex-config/*.toml; do
  assert_not_contains "$backup" 'test-aigocode-secret'
done

auth_after="$(shasum -a 256 "$home/.codex/auth.json" | awk '{print $1}')"
[[ "$auth_before" == "$auth_after" ]] || fail 'auth.json changed'
config_mode_after="$(python3 -c 'import os,stat,sys; print(oct(stat.S_IMODE(os.stat(sys.argv[1]).st_mode)))' "$home/.codex/config.toml")"
[[ "$config_mode_before" == "$config_mode_after" ]] || fail 'config mode changed'

bad_mode_home="$(new_home bad-mode)"
if HOME="$bad_mode_home" "$SCRIPT" unknown >"$bad_mode_home/out" 2>&1; then
  fail 'unknown mode succeeded'
fi

extra_arg_home="$(new_home extra-arg)"
if HOME="$extra_arg_home" "$SCRIPT" official --unknown >"$extra_arg_home/out" 2>&1; then
  fail 'unknown extra argument succeeded'
fi

bad_perm_home="$(new_home bad-perm)"
chmod 644 "$bad_perm_home/.config/codex-provider-mode/providers.json"
before="$(shasum -a 256 "$bad_perm_home/.codex/config.toml" | awk '{print $1}')"
if HOME="$bad_perm_home" "$SCRIPT" aigocode >"$bad_perm_home/out" 2>&1; then
  fail 'permissive provider file succeeded'
fi
after="$(shasum -a 256 "$bad_perm_home/.codex/config.toml" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail 'config changed after permission failure'

sentinel_home="$(new_home sentinel)"
python3 - "$sentinel_home/.config/codex-provider-mode/providers.json" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path))
data["aigocode"]["bearer_token"] = "CHANGE_ME"
open(path, "w").write(json.dumps(data))
PY
chmod 600 "$sentinel_home/.config/codex-provider-mode/providers.json"
if HOME="$sentinel_home" "$SCRIPT" aigocode >"$sentinel_home/out" 2>&1; then
  fail 'sentinel token succeeded'
fi

malformed_home="$(new_home malformed)"
printf '{' >"$malformed_home/.config/codex-provider-mode/providers.json"
chmod 600 "$malformed_home/.config/codex-provider-mode/providers.json"
if HOME="$malformed_home" "$SCRIPT" official >"$malformed_home/out" 2>&1; then
  fail 'malformed JSON succeeded'
fi

missing_home="$(new_home missing)"
rm "$missing_home/.config/codex-provider-mode/providers.json"
if HOME="$missing_home" "$SCRIPT" official >"$missing_home/out" 2>&1; then
  fail 'missing provider source succeeded'
fi

nonregular_home="$(new_home nonregular)"
rm "$nonregular_home/.config/codex-provider-mode/providers.json"
mkdir "$nonregular_home/.config/codex-provider-mode/providers.json"
if HOME="$nonregular_home" "$SCRIPT" official >"$nonregular_home/out" 2>&1; then
  fail 'non-regular provider source succeeded'
fi

incomplete_home="$(new_home incomplete)"
printf '{"official": {"name": "Official"}}' >"$incomplete_home/.config/codex-provider-mode/providers.json"
chmod 600 "$incomplete_home/.config/codex-provider-mode/providers.json"
if HOME="$incomplete_home" "$SCRIPT" official >"$incomplete_home/out" 2>&1; then
  fail 'incomplete provider source succeeded'
fi

printf 'PASS: codex-provider-mode isolated suite\n'
