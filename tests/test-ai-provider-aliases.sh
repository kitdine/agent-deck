#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KEY_SCRIPT="$ROOT/bin/ai-provider-key"
MODE_SCRIPT="$ROOT/bin/ai-provider-mode"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_contains() { grep -Fq "$2" "$1" || fail "$1 missing: $2"; }
assert_not_contains() { ! grep -Fq "$2" "$1" || fail "$1 leaked: $2"; }

home="$TMP/home"
mkdir -p "$home/.codex" "$home/.config/ai-provider-mode"
cat >"$home/.codex/config.toml" <<'TOML'
model_provider = "legacy"
TOML
cat >"$home/.config/ai-provider-mode/providers.json" <<'JSON'
{
  "providers": {
    "aigocode": {
      "host": "https://example.test",
      "codex": { "auth": "bearer", "keys": [] },
      "claude": { "auth": "bearer", "keys": [] }
    }
  },
  "keys": {}
}
JSON
chmod 600 "$home/.config/ai-provider-mode/providers.json"

printf 'test-token\n' | HOME="$home" "$KEY_SCRIPT" add aigocode codex work --default
assert_contains "$home/.config/ai-provider-mode/providers.json" '"aigocode-codex-work"'

HOME="$home" "$MODE_SCRIPT" codex aigocode --key work >"$home/mode.out"
assert_contains "$home/mode.out" 'key: aigocode-codex-work'
assert_contains "$home/.codex/config.toml" 'experimental_bearer_token = "test-token"'
HOME="$home" PYTHONPATH="$ROOT/bin" python3 -c 'import sqlite3,sys; row=sqlite3.connect(sys.argv[1]).execute("SELECT client, provider, multiplier FROM provider_selections").fetchone(); assert row == ("codex", "aigocode", "1")' "$home/.config/ai-provider-mode/usage.sqlite3"
assert_not_contains "$home/.config/ai-provider-mode/usage.sqlite3" 'test-token'

printf 'claude-token\n' | HOME="$home" "$KEY_SCRIPT" add aigocode claude home --default
assert_contains "$home/.config/ai-provider-mode/providers.json" '"aigocode-claude-home"'

HOME="$home" "$MODE_SCRIPT" claude aigocode --key home >"$home/claude.out"
assert_contains "$home/claude.out" 'key: aigocode-claude-home'
assert_contains "$home/.claude/settings.json" '"ANTHROPIC_AUTH_TOKEN": "claude-token"'

printf 'PASS: ai-provider short aliases\n'
