---
status: active
created: 2026-07-29
---

# Runtime Provider Attribution

Target release: `v0.2.2`.

This plan replaces client-wide `agentdeck run` ownership with client-observed
session boundaries. It does not expand project-attribution shell functions:
those functions continue to inject only Headroom project metadata.

**Release placement, decided 2026-07-29.** Staying in `v0.2.2` was reconsidered
against pulling this into `v0.2.1` alongside
[the shell integration plan](shell-integration.md), and confirmed. That plan had
to move into `v0.2.1` because `shell-init` has never shipped stable, so changing
its role is only an internal rearrangement until it does. Nothing here has an
equivalent expiring window: the `usage hook` command group and the session-route
table are pure additions, and dropping
`one_active_usage_run_per_client` plus allowing concurrent managed runs relaxes
existing behavior rather than tightening it, so introducing them in any later
release breaks nothing. Meanwhile this work carries a schema migration, depends on
two external client Hook contracts, and requires a Codex trust step AgentDeck
cannot automate — risk that should not be bundled into a release whose scope
already grew twice.

Two dependencies on `v0.2.1` follow from that ordering and are recorded where they
apply: the `shell-init` output baseline in task 1, and the shared
setup/status/remove lifecycle conventions below.

## Goal

- Attribute a resumed Codex or Claude logical session to the provider actually
  active when the resumed runtime starts.
- Split a running Claude session when Claude Code observes a provider settings
  reload.
- Allow concurrent Codex or Claude processes instead of enforcing one active
  `usage_runs` row per client.
- Preserve existing user hooks while providing an explicit, reversible
  AgentDeck hook lifecycle.

## Non-Goals

- No billing, provider, session, or process-lifecycle work in `shell-init` or
  the generated `codex()` and `claude()` project-attribution functions.
- No model-driven MCP call as a lifecycle boundary. MCP tools are not guaranteed
  to run at session start, resume, configuration reload, or process exit.
- No attempt to infer an unobserved historical runtime boundary.
- No modification of prompt, response, tool-call, or transcript contents.
- No automatic hook installation during Homebrew/package installation.

## Evidence Baseline

Gathered on 2026-07-29 at `2db056b`, before implementation:

- Schema v4 creates
  `one_active_usage_run_per_client ON usage_runs(client) WHERE ended_at IS NULL`.
  A second managed Codex launch therefore fails before the child starts with
  `UNIQUE constraint failed: usage_runs.client`.
- `StartRun` already detects external client overlap and downgrades the new run,
  but the unique index prevents the analogous managed overlap from existing.
- Exact run binding captures every source byte range changed during the whole
  client process lifetime. With two same-client processes it cannot prove which
  process wrote an overlapping range.
- Fallback attribution uses the logical session's first timestamp, so an old
  session resumed after a provider switch keeps its old provider unless
  `agentdeck run` produced an exact binding.
- Codex's current official Hook contract provides `SessionStart` command hooks.
  Input includes `session_id`, `transcript_path`, and
  `source=startup|resume|clear|compact`. Current Codex source rebuilds current
  thread configuration before resuming a non-running thread.
- Claude Code's current official Hook contract provides:
  - `SessionStart` with `session_id`, `transcript_path`, and
    `source=startup|resume|clear|compact|fork`;
  - `ConfigChange` when a settings file changes during a running session;
  - `SessionEnd`, including `reason=resume` for interactive session switches.
- Claude Code watches settings and reloads most keys in a running session.
  AgentDeck writes provider endpoint and credential values under the settings
  `env` object. `model` and `outputStyle` are documented restart exceptions;
  provider environment keys are not.
- Codex currently executes only command lifecycle hooks. Claude supports command
  and MCP-tool SessionStart hooks, but MCP cannot be the common reliable path.

## Decision

### 1. Hooks own runtime boundaries

Add an explicit usage integration lifecycle:

```text
agentdeck usage hook setup [--client codex|claude|all]
agentdeck usage hook status [--client codex|claude|all]
agentdeck usage hook remove [--client codex|claude|all]
```

`setup` merges only AgentDeck-owned command-hook entries into:

```text
~/.codex/hooks.json
~/.claude/settings.json
```

It preserves every unrelated key, matcher, and hook. It is idempotent. `remove`
removes only byte-equivalent AgentDeck-owned handler entries and leaves an
otherwise empty valid document rather than guessing whether AgentDeck owns the
file. `status` reports configured, missing, drifted, and client-side trust or
activation limitations that AgentDeck can actually observe.

Package installation never runs `setup`. Codex may require the user to approve
the newly added non-managed hook through `/hooks`; AgentDeck reports that step
but never edits Codex trust state or uses `--dangerously-bypass-hook-trust`.

This is the project's second `setup`/`status`/`remove` lifecycle. `v0.2.1` ships
the first one, `agentdeck shell`, and its conventions are the ones to follow here
rather than to reinvent: the same subcommand shape, the same state vocabulary for
configured, absent, drifted, and conflicting states, the same idempotence rule,
the same "remove touches only AgentDeck-owned entries", the same refusal to
overwrite an edited or unverifiable managed region, and the same rule that no
package installation path performs setup. The implementations cannot share code —
one edits shell text blocks, the other merges JSON documents — so the consistency
has to be deliberate. Any divergence from those conventions must be a documented
decision with its reason, not an artifact of being written second.

Installed handlers invoke a hidden command that reads the documented JSON event
from stdin:

```text
agentdeck usage hook event codex
agentdeck usage hook event claude
```

The handler is silent, bounded, fail-open for the client, and rejects unknown
clients, events, sources, oversized input, or missing session IDs without
writing an attribution row.

### 2. Persist session-route boundaries

Add a schema migration that:

- drops `one_active_usage_run_per_client`;
- creates `usage_session_routes` containing client, logical session ID,
  observed time, provider and multiplier snapshots, route metadata, hook event,
  source, and attribution quality;
- indexes `(client, session_id, observed_at)`.

The table stores no prompt, response, tool argument, credential, or transcript
contents. `transcript_path` is used only to validate that the event belongs to
the declared client and session; it is not duplicated into the new table.

For each usage event, the latest route boundary for the same client and logical
session at or before the event wins. If none exists, existing session-start
fallback remains.

### 3. Codex boundary rules

- Record `SessionStart` for `startup`, `resume`, and `clear`.
- Ignore `compact`: compaction does not start a new provider runtime.
- Snapshot the completed provider selection visible when the Hook runs.
- Events already recorded before a resume boundary keep their earlier
  attribution; later events use the new boundary.
- If the Hook is absent, disabled, awaiting trust, or fails, keep existing
  estimated session-start behavior.

`agentdeck run` remains a compatible low-level launcher. It no longer rejects a
second active client run. Managed overlap downgrades every affected open run to
`estimated`; it never chooses a winning run by `INSERT OR IGNORE` and never
blocks the child merely because another process exists.

### 4. Claude boundary rules

- Record `SessionStart` for `startup`, `resume`, `clear`, and `fork`.
- Ignore `compact`.
- Record `ConfigChange` only for the user settings path AgentDeck manages.
- A provider selection matched to the observed settings produces an estimated
  time boundary. It remains estimated because an already in-flight request may
  finish after the reload boundary while still belonging to the earlier route.
- If the changed settings do not match a completed AgentDeck selection after a
  short bounded reconciliation window, record `unknown` rather than extending
  the previous provider across an observed but unidentified route change.

This implements the intended split: requests before Claude observes the change
retain the previous provider; subsequent events use the new provider, with the
single crossing-request limitation disclosed by quality.

## Failure and Safety Rules

- Hook failure never prevents Codex or Claude from starting, resuming, applying
  settings, or exiting.
- Hook stdout is empty. Diagnostics go only to AgentDeck's status/doctor
  surfaces, never into model context.
- Setup performs an atomic private replace and rolls back the first client file
  if the second client update fails.
- Existing hook arrays and unknown JSON fields remain byte-semantically intact.
- No credential value, endpoint containing embedded credentials, or transcript
  contents enter hook status or error output.
- Concurrent duplicate Hook delivery is idempotent.

## Tasks

### 1. `hook-config-lifecycle`

Implement setup/status/remove and exact owned-entry merge/removal for Codex and
Claude. Cover absent files, existing unrelated hooks, duplicate setup, partial
failure rollback, malformed JSON, symlinks, permissions, and Codex trust
guidance.

Acceptance:

- setup followed by setup is a no-op;
- remove preserves unrelated hooks;
- shell functions and `shell-init` output are byte-identical before and after,
  measured against the `v0.2.1` output as delivered by the shell integration plan
  — which changes that output by design, adding an activation marker and a
  presence guard. Comparing against a pre-`v0.2.1` baseline would either fail
  spuriously or mask a real regression;
- `~/.claude/settings.json` survives both writers: a provider switch preserves
  AgentDeck-owned hook entries, and hook setup and remove preserve the provider
  `env` object, verified in both orders;
- no package installation path writes hook configuration.

Verification: L3 targeted command/config tests plus the full vendored suite and
relevant privacy/file-mode checks.

### 2. `hook-boundary-storage`

Add the schema migration, strict event parser, idempotent boundary writer, and
session-route lookup. Drop the active-run unique index and make managed overlap
non-blocking and explicitly estimated.

Acceptance:

- two active same-client runs can coexist without a constraint error;
- duplicate Hook delivery creates one semantic boundary;
- unknown or malformed input writes nothing;
- a resumed session keeps pre-boundary events on the old route and assigns
  post-boundary events to the new route.

Verification: L2 targeted store/usage/CLI tests plus the full vendored suite.

### 3. `claude-reload-boundary`

Implement Claude `ConfigChange` reconciliation and estimated time splitting,
including unmatched settings and an in-flight crossing request.

Acceptance:

- matched user-settings reload changes later event attribution;
- project/local/policy/skills changes do not create provider boundaries;
- unmatched managed settings become `unknown`, not the prior provider;
- no result claims exact attribution solely from a reload timestamp.

Verification: L2 targeted provider/usage tests plus the full vendored suite.

### 4. `runtime-attribution-contract`

Update the living specification and manual after tasks 1-3 pass review. Define
Hook setup/trust/removal, boundary quality, fallback behavior, concurrent runs,
and the unchanged project-attribution shell-function scope. Raise the
specification version from whatever version is current at delivery.

Acceptance:

- no living document calls `agentdeck run` the primary resume path;
- no document says shell functions perform usage tracking;
- Hook absence and trust state have explicit visible fallbacks;
- the `usage hook` lifecycle is described in the same terms as the `shell`
  lifecycle the specification already defines, with any deliberate divergence
  named and justified;
- `docs/README.md`, plan status, and review records agree.

Verification: L0 documentation discovery/link checks and `git diff --check`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `hook-config-lifecycle` | [ ] | [ ] |
| 2. `hook-boundary-storage` | [ ] | [ ] |
| 3. `claude-reload-boundary` | [ ] | [ ] |
| 4. `runtime-attribution-contract` | [ ] | [ ] |

Order: task 1 defines the installed wire contract. Task 2 consumes that wire
contract and may proceed once its exact JSON fixtures are fixed. Task 3 depends
on task 2. Task 4 runs last.

## Backlog / Future Feature Ideas

- If Codex or Claude later exposes an authenticated local lifecycle socket,
  replace command-process startup with direct local notification while keeping
  the same event schema and database boundary contract.
- Reconsider MCP only if a client adds a guaranteed lifecycle-delivery MCP
  mechanism. A normal model-callable MCP tool is not sufficient.
- Add per-request start boundaries if Claude exposes them, allowing the
  crossing request at a settings reload to become exact rather than estimated.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

> 进入开发：`runtime-provider-attribution` / `<task-anchor>`

Read `AGENTS.md`, this plan's Decision and named task, the current usage
attribution contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after required verification passes.
An independent reviewer records a PASS round under
`docs/reviews/runtime-provider-attribution/<task-anchor>.md` before ticking
`Review`.
