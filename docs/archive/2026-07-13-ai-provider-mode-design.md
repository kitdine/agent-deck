# AI Provider Mode Switch Design

**Status:** reference, superseded by
`docs/specs/2026-07-13-agentdeck-cli-design.md`

This document describes the former legacy Python and Bash behavior. The
repository implementation was removed after the AgentDeck replacement passed
independent review; this document remains a historical contract and is not the
active product architecture.

## Goal

Use one small local configuration to switch Codex and Claude between Headroom
providers. Codex must keep one visible session group and remain visibly signed
in to ChatGPT. Claude must preserve all settings other than its endpoint and
auth token.

## Managed Files

All mutable switcher state is kept together:

```text
~/.config/ai-provider-mode/
  providers.json
  backups/
    codex/
    claude/
```

`providers.json` and every backup are mode `0600`. The scripts never modify
Codex `auth.json`, Codex session data, or Claude settings unrelated to the two
Anthropic environment variables.

## Provider JSON

The JSON has a complete provider list and a separate key registry. Provider
names and key aliases are local operator choices; key aliases must match
`[a-z0-9_-]+` and are never interpreted as credentials.

```json
{
  "providers": {
    "official": {
      "host": "http://10.0.0.103:15021",
      "codex": { "auth": "openai_login" },
      "claude": { "enabled": false }
    },
    "aigocode": {
      "host": "http://10.0.0.103:15022",
      "codex": {
        "auth": "bearer",
        "keys": ["aigo-codex-main"],
        "default_key": "aigo-codex-main"
      },
      "claude": {
        "auth": "bearer",
        "keys": ["aigo-claude-main", "aigo-claude-backup"],
        "default_key": "aigo-claude-main"
      }
    },
    "sssaicode": {
      "host": "http://10.0.0.103:15023",
      "codex": { "auth": "bearer", "keys": [] },
      "claude": { "auth": "bearer", "keys": [] }
    },
    "cubence": {
      "host": "http://10.0.0.103:15023",
      "codex": { "auth": "bearer", "keys": [] },
      "claude": { "auth": "bearer", "keys": [] }
    }
  },
  "keys": {
    "aigo-codex-main": { "value": "replace-me" },
    "aigo-claude-main": { "value": "replace-me" },
    "aigo-claude-backup": { "value": "replace-me" }
  }
}
```

`sssaicode` and `cubence` are listed now even though their Headroom endpoints
are not deployed. Switching never performs a network health check.

`official.codex` uses the existing ChatGPT login and has no JSON key.
`official.claude` is intentionally disabled because no official Claude account
is currently available. It becomes usable later by adding `auth: "bearer"`, a
key list, and (optionally) a default key.

## Commands

```text
ai-provider-mode codex <provider> [--key <alias>] [--restart]
ai-provider-mode claude <provider> [--key <alias>] [--restart]
ai-provider-mode status

ai-provider-key add <provider> <codex|claude> <alias> [--default]
ai-provider-key bind <provider> <codex|claude> <alias> [--default]
ai-provider-key update <full-key-name>
ai-provider-key unbind <provider> <codex|claude> <alias>
ai-provider-key remove <full-key-name>
ai-provider-key list
```

`add` stores every key as `<provider>-<client>-<alias>`; for example,
`ai-provider-key add aigocode codex main` creates `aigocode-codex-main`.
`bind` and `unbind` accept the same short alias and resolve it to that full
name. `update` and `remove` require the full name shown by `list`, because
they have no provider/client context. `add` and `update` read the token from a
no-echo prompt; token values never appear in arguments, output, status, or
backups. `bind` permits an existing key only when the specified provider and
client exist. `remove` refuses while the alias is still bound.

When `--key` is supplied, callers use the short alias and the switcher resolves
it as `<provider>-<client>-<alias>` before checking that provider/client's
`keys` array and the global `keys` object. Existing full key names are also
accepted. Without `--key`, a switch uses `default_key`; if absent and exactly
one key is bound, it uses that key. With multiple bound keys and no default,
it refuses and lists aliases only.

## Client Effects

Codex always writes `model_provider = "custom"` and replaces only
`[model_providers.custom]`. Its URL is `<host>/v1`, and it keeps
`requires_openai_auth = true`; this keeps the ChatGPT login and makes new
official and AIGocode sessions appear in the same `custom` group. A bearer key
is written only when the selected Codex service has `auth: "bearer"`.

Claude changes only `env.ANTHROPIC_BASE_URL` to `<host>` and
`env.ANTHROPIC_AUTH_TOKEN` to the selected bearer value. It preserves
`effortLevel`, hooks, statusLine, and every other `settings.json` field.

Before a changed configuration is written, the respective active configuration
is copied to its central backup directory with its auth value redacted. A
switch that has no effective change does not create a backup.
