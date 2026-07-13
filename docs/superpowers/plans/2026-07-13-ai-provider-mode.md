# AI Provider Mode Implementation Plan

**Goal:** Replace the old per-tool mode files with one provider JSON, two
small switchers, and one interactive key manager.

**Constraints:** Keep the implementation deliberately small. Keep regression
tests isolated with synthetic homes. Do not add endpoint health checks,
Keychain integration, data migration, or any changes to Codex
authentication/session files.

## Files

- Create: `bin/ai-provider-mode`
- Create: `bin/ai-provider-key`
- Create: `bin/ai_provider_common.py`
- Modify: `config/providers.example.json`
- Create: `tests/test-ai-provider-mode.sh`
- Install later: `~/.local/bin/{codex-provider-mode,claude-mode,ai-provider-key}`
- Create later: `~/.config/ai-provider-mode/providers.json`

## Task 1: Centralize provider configuration

- [ ] Replace the old two-provider JSON example with the `providers` and
  `keys` schema from the design.
- [ ] Include `official`, `aigocode`, `sssaicode`, and `cubence` with ports
  `15021`, `15022`, `15023`, and `15023`.
- [ ] Use one protected config path:
  `~/.config/ai-provider-mode/providers.json`.
- [ ] Move redacted active-config backups under
  `~/.config/ai-provider-mode/backups/codex` and `.../claude`.

## Task 2: Implement the unified switcher

- [ ] Accept `codex|claude <provider> [--key <alias>] [--restart]` and
  unified `status`.
- [ ] Resolve `--key` from the selected `providers.<provider>.codex.keys`; if
  omitted, choose `default_key` or the only bound key.
- [ ] Support `openai_login` without a key and bearer mode with the selected
  key's value.
- [ ] Keep `model_provider = "custom"`, `requires_openai_auth = true`, and
  only replace the custom provider block.
- [ ] Redact bearer values in backups and status output.

## Task 3: Handle Claude in the unified switcher

- [ ] Read the same JSON and accept the same provider/key selection rules.
- [ ] Reject disabled or missing Claude service definitions without changing
  `~/.claude/settings.json`.
- [ ] Change only `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN`; preserve
  all other JSON fields.
- [ ] Create a redacted central backup only when the values actually change.

## Task 4: Add the small interactive key manager

- [ ] Implement `add`, `bind`, `update`, `unbind`, `remove`, and `list` as
  specified in the design.
- [ ] Prompt without echo for add/update token values; never print them.
- [ ] Validate aliases and prevent deleting a key that remains bound.
- [ ] Keep the JSON mode `0600` after every edit.

## Task 5: Install and manually exercise

- [ ] Install `ai-provider-mode`, `ai-provider-key`, and their shared helper under `~/.local/bin`, and create the new JSON
  only if it does not already exist.
- [ ] Add the current keys with `ai-provider-key add` instead of placing token
  values in a command or chat.
- [ ] Manually invoke `status`, one Codex switch, and one Claude switch; check
  that status output and backups contain aliases/metadata but no token values.
