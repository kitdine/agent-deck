# Codex Provider Mode Switch Design

## Goal

Update `codex-provider-mode` to switch Codex between two existing Headroom endpoints:

- `official`: `http://10.0.0.103:15021/v1`
- `aigocode`: `http://10.0.0.103:15022/v1`

The switch must preserve one combined Codex session list, keep the ChatGPT account visibly signed in in every mode, avoid changing `auth.json`, and keep frequently changing provider settings in one editable JSON source rather than hard-coding them in the script.

## Provider Source File

Provider endpoints and credentials are stored in:

```text
~/.config/codex-provider-mode/providers.json
```

The initial schema is:

```json
{
  "official": {
    "name": "OpenAI Official via Headroom",
    "base_url": "http://10.0.0.103:15021/v1"
  },
  "aigocode": {
    "name": "AIGocode via Headroom",
    "base_url": "http://10.0.0.103:15022/v1",
    "bearer_token": "replace-with-the-current-token"
  }
}
```

This JSON file is the only source that an operator edits when an endpoint or token changes. It must be a regular file owned by the current user with mode `0600`. The script rejects missing, malformed, overly permissive, or incomplete source files before changing Codex configuration.

## Configuration Contract

Both modes use the same provider ID:

```toml
model_provider = "custom"
```

The script replaces only `[model_providers.custom]`. Reusing `custom` is required because Codex stores `model_provider` on every thread and may filter the session list by that value.

Official mode uses:

```toml
[model_providers.custom]
name = "OpenAI Official via Headroom"
base_url = "http://10.0.0.103:15021/v1"
requires_openai_auth = true
supports_websockets = true
wire_api = "responses"
```

AIGocode mode uses:

```toml
[model_providers.custom]
name = "AIGocode via Headroom"
base_url = "http://10.0.0.103:15022/v1"
requires_openai_auth = true
experimental_bearer_token = "<value read from providers.json>"
supports_websockets = true
wire_api = "responses"
```

`requires_openai_auth = true` keeps Codex attached to the existing ChatGPT login in `auth.json`. In AIGocode mode, `experimental_bearer_token` overrides the bearer token used for requests while the stored authentication mode remains `chatgpt`. The active token is therefore present in both the protected JSON source and the active `config.toml`; it is never hard-coded in the script or printed.

## Script Interface

Supported commands:

```text
codex-provider-mode official [--restart]
codex-provider-mode aigocode [--restart]
codex-provider-mode status
```

The old `ccswitch` mode is removed.

For a switch, the script:

1. Validates the mode, optional flag, and existence of `~/.codex/config.toml`.
2. Loads and validates the selected provider entry from `providers.json`, including a non-empty AIGocode bearer token.
3. Creates a timestamped, mode-`0600` backup under `~/.backup-codex-config`. Any `experimental_bearer_token` value is redacted in the backup so stale tokens do not accumulate outside the JSON source and active configuration.
4. Ensures the top-level `model_provider` is exactly `custom` without changing `model`, reasoning effort, service tier, or unrelated tables.
5. Replaces only `[model_providers.custom]` with the selected mode block.
6. Writes the file atomically so an interrupted write cannot truncate the configuration.
7. Optionally restarts Codex App when `--restart` is supplied.

`status` reports the selected mode, provider ID, endpoint, ChatGPT-auth requirement, JSON source status, and whether the selected provider has a token. It never prints the token value.

## State Boundaries

The script must not:

- modify `~/.codex/auth.json`;
- modify Codex session JSONL files or SQLite databases;
- switch or create another `CODEX_HOME`;
- expose the AIGocode token in output, backups, or command arguments;
- alter unrelated top-level configuration or TOML tables.

Existing historical threads recorded under the old `ccswitch` provider ID are not migrated. New official and AIGocode threads are both recorded as `custom` and share the same visible session group.

## Error Handling

- Reject unknown modes and unknown extra arguments with usage text and a non-zero exit code.
- Reject a missing, non-regular, non-owned, non-`0600`, malformed, or incomplete provider JSON file before touching `config.toml`.
- Reject AIGocode mode when `bearer_token` is absent, empty, or still equals the documented placeholder.
- Preserve the original file mode during atomic replacement.
- If restart fails, retain the completed configuration switch and report the restart failure separately.
- Never include credential values in errors or status output.

## Verification

Verification uses an isolated temporary `HOME` first, then the real environment only after the generated configuration passes validation.

The isolated checks cover:

- official block generation;
- AIGocode block generation with a dummy JSON token;
- failure for missing, malformed, permissive, incomplete, or placeholder-bearing JSON and no configuration change;
- preservation of unrelated top-level keys and tables;
- idempotent repeated switching;
- status redaction;
- backup redaction;
- invalid argument rejection.

Real-environment checks cover:

- `codex doctor --json --summary` reports `stored auth mode: chatgpt` and `model provider: custom` in both modes;
- the effective endpoint matches the selected Headroom port;
- `auth.json` checksum is unchanged across both switches;
- session database inventory is unchanged by switching;
- after an App restart, the ChatGPT account remains signed in and existing `custom` sessions remain visible.
