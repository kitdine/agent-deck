# Codex Provider Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and safely install a provider switcher that routes Codex through official or AIGocode Headroom while preserving one `custom` session group and the existing ChatGPT login.

**Architecture:** Keep a canonical script and tests in this workspace, then install the verified script to `~/.local/bin/codex-provider-mode`. Provider metadata and the mutable AIGocode token live in a mode-`0600` JSON file outside `.codex`; switching atomically regenerates only the top-level `model_provider` and `[model_providers.custom]`, with redacted backups.

**Tech Stack:** Bash 3.2-compatible shell, Python 3 standard library, JSON, TOML text transformation, macOS `osascript`/`open`, Codex CLI 0.144.1 diagnostics.

## Global Constraints

- Both modes must use `model_provider = "custom"` so their sessions remain in one list.
- `~/.codex/auth.json`, session JSONL files, and Codex SQLite databases must never be modified.
- ChatGPT login remains active through `requires_openai_auth = true` in both modes.
- AIGocode requests use `experimental_bearer_token` read from the protected JSON source.
- The token must never appear in command output or backups.
- The JSON source must be a current-user-owned regular file with exact mode `0600`.
- Every shell command run during implementation is prefixed with `rtk`.
- This workspace is not a Git repository; replace commit steps with explicit diff and checksum checkpoints.

---

## File Map

- Create `bin/codex-provider-mode`: canonical switcher implementation.
- Create `tests/test-codex-provider-mode.sh`: isolated-HOME regression suite with no real credentials.
- Create `config/providers.example.json`: installable provider source template containing a rejected sentinel token.
- Modify `docs/superpowers/specs/2026-07-13-codex-provider-mode-design.md` only if execution reveals a verified contract change.
- Install verified `bin/codex-provider-mode` to `/Users/jobshen/.local/bin/codex-provider-mode`.
- Create `/Users/jobshen/.config/codex-provider-mode/providers.json` only when absent; never overwrite an operator-edited credential file.

### Task 1: Implement the isolated switch contract with tests

**Files:**
- Create: `tests/test-codex-provider-mode.sh`
- Create: `config/providers.example.json`
- Create: `bin/codex-provider-mode`

**Interfaces:**
- Consumes: `HOME`, `$HOME/.codex/config.toml`, and `$HOME/.config/codex-provider-mode/providers.json`.
- Produces: `codex-provider-mode official|aigocode|status [--restart]` and timestamped redacted backups under `$HOME/.backup-codex-config`.

- [ ] **Step 1: Create the failing isolated regression test**

Create `tests/test-codex-provider-mode.sh` with this complete test harness:

```bash
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
[model_providers.custom]
name = "Legacy"
base_url = "http://127.0.0.1:1/v1"
requires_openai_auth = true
wire_api = "responses"

[features]
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
assert_not_contains "$home/.codex/config.toml" 'experimental_bearer_token'
assert_contains "$home/.codex/config.toml" 'model_reasoning_effort = "high"'
assert_contains "$home/.codex/config.toml" '[features]'
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
```

- [ ] **Step 2: Make the test executable and verify it fails because the implementation is absent**

Run:

```bash
rtk proxy chmod +x tests/test-codex-provider-mode.sh
rtk test tests/test-codex-provider-mode.sh
```

Expected: FAIL because `bin/codex-provider-mode` does not exist.

- [ ] **Step 3: Create the provider source template**

Create `config/providers.example.json`:

```json
{
  "official": {
    "name": "OpenAI Official via Headroom",
    "base_url": "http://10.0.0.103:15021/v1"
  },
  "aigocode": {
    "name": "AIGocode via Headroom",
    "base_url": "http://10.0.0.103:15022/v1",
    "bearer_token": "CHANGE_ME"
  }
}
```

- [ ] **Step 4: Implement the minimal safe switcher**

Create `bin/codex-provider-mode` with the following implementation:

```bash
#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"
RESTART="${2:-}"
CONFIG="$HOME/.codex/config.toml"
PROVIDERS_FILE="$HOME/.config/codex-provider-mode/providers.json"
BACKUP_DIR="$HOME/.backup-codex-config"

usage() {
  cat <<'EOF'
Usage:
  codex-provider-mode official [--restart]
  codex-provider-mode aigocode [--restart]
  codex-provider-mode status
EOF
}

case "$MODE" in
  official|aigocode|status) ;;
  *) usage; exit 2 ;;
esac

if [[ "$MODE" == "status" ]]; then
  [[ $# -eq 1 ]] || { usage; exit 2; }
else
  [[ $# -le 2 ]] || { usage; exit 2; }
  [[ -z "$RESTART" || "$RESTART" == "--restart" ]] || { usage; exit 2; }
fi

[[ -f "$CONFIG" ]] || { printf 'ERROR: %s not found\n' "$CONFIG" >&2; exit 1; }

python3 - "$MODE" "$CONFIG" "$PROVIDERS_FILE" "$BACKUP_DIR" <<'PY'
import datetime
import json
import os
import re
import stat
import sys
import tempfile
from pathlib import Path

mode, config_arg, providers_arg, backup_arg = sys.argv[1:]
config = Path(config_arg)
providers_path = Path(providers_arg)
backup_dir = Path(backup_arg)

def die(message):
    raise SystemExit(f"ERROR: {message}")

try:
    info = providers_path.lstat()
except FileNotFoundError:
    die(f"provider source not found: {providers_path}")
if not stat.S_ISREG(info.st_mode):
    die(f"provider source is not a regular file: {providers_path}")
if info.st_uid != os.getuid():
    die(f"provider source is not owned by uid {os.getuid()}: {providers_path}")
if stat.S_IMODE(info.st_mode) != 0o600:
    die(f"provider source must have mode 0600: {providers_path}")

try:
    providers = json.loads(providers_path.read_text())
except (OSError, UnicodeError, json.JSONDecodeError) as exc:
    die(f"cannot read provider source: {exc}")
if not isinstance(providers, dict):
    die("provider source root must be an object")

def provider(name):
    value = providers.get(name)
    if not isinstance(value, dict):
        die(f"provider entry must be an object: {name}")
    display_name = value.get("name")
    base_url = value.get("base_url")
    if not isinstance(display_name, str) or not display_name.strip():
        die(f"provider name is missing: {name}")
    if not isinstance(base_url, str) or not re.fullmatch(r"https?://[^\s]+/v1/?", base_url):
        die(f"provider base_url must be an http(s) URL ending in /v1: {name}")
    return display_name.strip(), base_url.rstrip("/")

def toml_string(value):
    return json.dumps(value, ensure_ascii=False)

def redact_token(text):
    lines = text.splitlines()
    output = []
    in_custom = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            in_custom = stripped == "[model_providers.custom]"
        if in_custom and re.match(r"^\s*experimental_bearer_token\s*=", line):
            output.append('experimental_bearer_token = "<redacted>"')
        else:
            output.append(line)
    return "\n".join(output).rstrip() + "\n"

def replace_top_level_provider(text):
    lines = text.splitlines()
    output = []
    seen = False
    in_table = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            in_table = True
        if not in_table and re.match(r"^\s*model_provider\s*=", line):
            if not seen:
                output.append('model_provider = "custom"')
                seen = True
        else:
            output.append(line)
    if not seen:
        index = next((i for i, line in enumerate(output) if line.strip().startswith("[")), len(output))
        output.insert(index, 'model_provider = "custom"')
        if index + 1 < len(output) and output[index + 1].strip():
            output.insert(index + 1, "")
    return "\n".join(output).rstrip() + "\n"

def replace_custom_table(text, block):
    lines = text.splitlines()
    output = []
    found = False
    skipping = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            if stripped == "[model_providers.custom]":
                found = True
                skipping = True
                output.extend(block.splitlines())
                continue
            skipping = False
        if not skipping:
            output.append(line)
    if not found:
        index = next((i for i, line in enumerate(output) if line.strip().startswith("[")), len(output))
        output[index:index] = block.splitlines() + [""]
    return "\n".join(output).rstrip() + "\n"

text = config.read_text()

if mode == "status":
    custom_match = re.search(r"(?ms)^\[model_providers\.custom\]\s*$(.*?)(?=^\[|\Z)", text)
    block = custom_match.group(1) if custom_match else ""
    base_match = re.search(r'^\s*base_url\s*=\s*"([^"]+)"', block, re.MULTILINE)
    active_url = base_match.group(1) if base_match else "unknown"
    active_mode = "unknown"
    for candidate in ("official", "aigocode"):
        _, candidate_url = provider(candidate)
        if active_url.rstrip("/") == candidate_url:
            active_mode = candidate
    token = providers.get(active_mode, {}).get("bearer_token") if active_mode != "unknown" else None
    print(f"Mode: {active_mode}")
    print("Model provider: custom")
    print(f"Endpoint: {active_url}")
    print("ChatGPT auth required: yes")
    print(f"Provider source: {providers_path}")
    print(f"Token configured: {'yes' if isinstance(token, str) and token.strip() and token != 'CHANGE_ME' else 'no'}")
    raise SystemExit(0)

display_name, base_url = provider(mode)
block_lines = [
    "[model_providers.custom]",
    f"name = {toml_string(display_name)}",
    f"base_url = {toml_string(base_url)}",
    "requires_openai_auth = true",
    "supports_websockets = true",
    'wire_api = "responses"',
]
if mode == "aigocode":
    token = providers[mode].get("bearer_token")
    if not isinstance(token, str) or not token.strip() or token == "CHANGE_ME":
        die("aigocode bearer_token is missing or unchanged")
    block_lines.insert(4, f"experimental_bearer_token = {toml_string(token)}")

new_text = replace_custom_table(replace_top_level_provider(text), "\n".join(block_lines))
if new_text == text:
    print(f"Codex provider already active: {mode}")
    raise SystemExit(0)

backup_dir.mkdir(parents=True, exist_ok=True)
stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S-%f")
backup = backup_dir / f"config.{stamp}.toml"
fd = os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as handle:
    handle.write(redact_token(text))

original_mode = stat.S_IMODE(config.stat().st_mode)
fd, temp_name = tempfile.mkstemp(prefix=".config.toml.", dir=config.parent)
try:
    with os.fdopen(fd, "w") as handle:
        handle.write(new_text)
        handle.flush()
        os.fsync(handle.fileno())
    os.chmod(temp_name, original_mode)
    os.replace(temp_name, config)
finally:
    if os.path.exists(temp_name):
        os.unlink(temp_name)

print(f"Codex provider switched to: {mode}")
print("Model provider remains: custom")
print(f"Endpoint: {base_url}")
print(f"Redacted backup: {backup}")
PY

if [[ "$MODE" != "status" && "$RESTART" == "--restart" ]]; then
  if ! osascript -e 'quit app "Codex"'; then
    printf 'WARN: Codex App quit request failed; configuration was already switched.\n' >&2
  fi
  if ! open -a Codex; then
    printf 'ERROR: Codex App restart failed; configuration was already switched.\n' >&2
    exit 1
  fi
fi
```

- [ ] **Step 5: Make the script executable and run the isolated suite**

Run:

```bash
rtk proxy chmod +x bin/codex-provider-mode
rtk test tests/test-codex-provider-mode.sh
```

Expected: `PASS: codex-provider-mode isolated suite` and exit code 0.

- [ ] **Step 6: Run syntax and secret scans**

Run:

```bash
rtk proxy bash -n bin/codex-provider-mode
rtk proxy bash -n tests/test-codex-provider-mode.sh
rtk proxy rg -n 'test-aigocode-secret|CHANGE_ME' bin docs/superpowers/specs
```

Expected: both syntax checks exit 0; the secret/sentinel scan finds no credential fixture in the installed script or design document.

- [ ] **Step 7: Record the non-Git review checkpoint**

Run:

```bash
rtk diff bin/codex-provider-mode
rtk diff tests/test-codex-provider-mode.sh
rtk read config/providers.example.json
```

Expected: review output contains only the provider switch implementation, its isolated tests, and the non-secret template.

### Task 2: Install without overwriting credentials

**Files:**
- Install: `/Users/jobshen/.local/bin/codex-provider-mode`
- Create if absent: `/Users/jobshen/.config/codex-provider-mode/providers.json`

**Interfaces:**
- Consumes: the verified artifacts from Task 1.
- Produces: the real `codex-provider-mode` command and one operator-editable credential source.

- [ ] **Step 1: Capture current script and Codex-state checksums**

Run:

```bash
rtk proxy shasum -a 256 /Users/jobshen/.local/bin/codex-provider-mode /Users/jobshen/.codex/auth.json
```

Expected: two checksum lines saved in the execution log.

- [ ] **Step 2: Install the verified script**

Run with approval because the destination is outside the workspace:

```bash
rtk proxy install -m 0755 bin/codex-provider-mode /Users/jobshen/.local/bin/codex-provider-mode
```

Expected: exit code 0 and `/Users/jobshen/.local/bin/codex-provider-mode` has mode `0755`.

- [ ] **Step 3: Create the provider JSON only if it does not already exist**

Run with approval:

```bash
rtk proxy mkdir -p /Users/jobshen/.config/codex-provider-mode
rtk proxy test -e /Users/jobshen/.config/codex-provider-mode/providers.json || rtk proxy install -m 0600 config/providers.example.json /Users/jobshen/.config/codex-provider-mode/providers.json
rtk proxy chmod 0600 /Users/jobshen/.config/codex-provider-mode/providers.json
```

Expected: the directory exists; an existing JSON file is preserved; a new file contains `CHANGE_ME` and mode `0600`.

- [ ] **Step 4: Validate safe failure before a real token is supplied**

Run:

```bash
rtk proxy shasum -a 256 /Users/jobshen/.codex/config.toml
rtk proxy codex-provider-mode aigocode
rtk proxy shasum -a 256 /Users/jobshen/.codex/config.toml
```

Expected when the JSON still contains `CHANGE_ME`: the switch exits non-zero with a redacted error, and the two config checksums are identical.

- [ ] **Step 5: Operator inserts the current AIGocode token**

The operator edits `/Users/jobshen/.config/codex-provider-mode/providers.json`, replaces only the `CHANGE_ME` JSON string with the current token, saves valid JSON, and keeps mode `0600`. Do not paste the token into chat or any shell command.

- [ ] **Step 6: Validate the credential source without displaying it**

Run:

```bash
rtk proxy python3 -c 'import json,os,stat; p=os.path.expanduser("~/.config/codex-provider-mode/providers.json"); s=os.stat(p); d=json.load(open(p)); t=d["aigocode"]["bearer_token"]; assert stat.S_IMODE(s.st_mode)==0o600 and isinstance(t,str) and t and t!="CHANGE_ME"; print("provider JSON valid; token redacted")'
```

Expected: `provider JSON valid; token redacted`.

### Task 3: Verify real switching and invariants

**Files:**
- Modify during verification: `/Users/jobshen/.codex/config.toml`
- Must remain unchanged: `/Users/jobshen/.codex/auth.json`, `/Users/jobshen/.codex/state_5.sqlite`

**Interfaces:**
- Consumes: installed script and populated provider JSON.
- Produces: evidence that both routes preserve ChatGPT auth and the `custom` session group.

- [ ] **Step 1: Capture invariant hashes and thread inventory**

Run:

```bash
rtk proxy shasum -a 256 /Users/jobshen/.codex/auth.json
rtk proxy sqlite3 -readonly -header -column /Users/jobshen/.codex/state_5.sqlite 'SELECT model_provider, archived, COUNT(*) AS threads FROM threads GROUP BY model_provider, archived ORDER BY model_provider, archived;'
rtk proxy touch /private/tmp/codex-provider-mode-backup-marker
```

Expected: the auth hash and current thread counts are recorded without mutation; the marker predates every backup produced by this verification run.

- [ ] **Step 2: Switch to AIGocode without restarting the App**

Run:

```bash
rtk proxy codex-provider-mode aigocode
rtk proxy codex-provider-mode status
rtk proxy codex doctor --json --summary | rtk proxy jq '{auth: .checks["auth.credentials"].details, config: .checks["config.load"].details, websocket: .checks["network.websocket_reachability"].details}'
```

Expected: status reports `Mode: aigocode`, endpoint `15022`, token configured `yes`, stored auth mode `chatgpt`, and model provider `custom`; no token is printed.

- [ ] **Step 3: Verify AIGocode Headroom reachability**

Run with network approval if sandbox networking blocks the LAN endpoint:

```bash
rtk proxy curl -fsS --max-time 5 http://10.0.0.103:15022/readyz
```

Expected: HTTP success from Headroom. A failure blocks live request verification but does not invalidate isolated configuration tests.

- [ ] **Step 4: Switch back to official and validate the token is removed**

Run:

```bash
rtk proxy codex-provider-mode official
rtk proxy codex-provider-mode status
rtk proxy rg -n '^experimental_bearer_token\s*=' /Users/jobshen/.codex/config.toml
rtk proxy codex doctor --json --summary | rtk proxy jq '{auth: .checks["auth.credentials"].details, config: .checks["config.load"].details, websocket: .checks["network.websocket_reachability"].details}'
```

Expected: status reports `Mode: official`, endpoint `15021`, the `rg` command returns no matches, stored auth mode remains `chatgpt`, and model provider remains `custom`.

- [ ] **Step 5: Verify state invariants after both switches**

Run:

```bash
rtk proxy shasum -a 256 /Users/jobshen/.codex/auth.json
rtk proxy sqlite3 -readonly -header -column /Users/jobshen/.codex/state_5.sqlite 'SELECT model_provider, archived, COUNT(*) AS threads FROM threads GROUP BY model_provider, archived ORDER BY model_provider, archived;'
rtk proxy python3 -c 'import os,pathlib,re; marker=os.stat("/private/tmp/codex-provider-mode-backup-marker").st_mtime_ns; files=[p for p in pathlib.Path(os.path.expanduser("~/.backup-codex-config")).glob("*.toml") if p.stat().st_mtime_ns > marker]; bad=[str(p) for p in files if re.search(r"experimental_bearer_token\s*=\s*\"(?!<redacted>)", p.read_text())]; assert not bad, "unredacted new backups: " + ", ".join(bad); print(f"new backups redacted: {len(files)}")'
```

Expected: the auth hash and thread inventory equal Step 1; only backups created during this run are scanned, and none contains an unredacted bearer token. The state database file checksum is deliberately not compared because the running Codex session can legitimately update it concurrently.

- [ ] **Step 6: Restart into the selected mode and verify the visible App state**

Select the desired final mode, then run with GUI approval:

```bash
rtk proxy codex-provider-mode official --restart
```

Expected: Codex App reopens, continues to show the ChatGPT account as signed in, and the existing `custom` sessions remain visible in one history list.

- [ ] **Step 7: Final verification checkpoint**

Run:

```bash
rtk test tests/test-codex-provider-mode.sh
rtk proxy bash -n bin/codex-provider-mode
rtk proxy cmp -s bin/codex-provider-mode /Users/jobshen/.local/bin/codex-provider-mode
rtk proxy codex-provider-mode status
```

Expected: isolated suite passes, syntax passes, installed script matches the workspace source, and status reports the intentionally selected final mode without revealing credentials.
