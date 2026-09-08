# Runtime Toolchain

Read this file when a runtime capability this repository expects is missing,
misbehaving, or being configured for the first time. `AGENTS.md` carries the
short form; this file carries the detail and the degradation rules.

The distinction that matters throughout: **this repository declares which
capabilities it depends on and how to behave without them. It does not store
their addresses.** Endpoints, ports, and install paths differ per machine and
per operator, so recording them here would produce a file that is wrong for
every clone but one — the same failure mode `scripts/hooks/beads-consistency.py`
avoids by identifying the repository through a file it owns rather than a path.

## MCP servers / MCP 服务

Configuration lives at the user level, not in the repository. There is no
`.mcp.json`, deliberately: an MCP endpoint is an operator's local deployment,
and a committed address would be a credential-adjacent fact that goes stale.

| Server | Used for | Transport | Without it |
| --- | --- | --- | --- |
| `neo4j` | `completion-evidence/v1` gates, WorkUnit and criterion records | HTTP | Evidence work is `BLOCKED`, never silently skipped. Report the exact failure. A configured-but-unreachable provider is not an absent one. |
| `neo4j-mem` | Durable project memory in the `agent-deck:` namespace | HTTP | Continue with repository sources. Report the limitation only when it materially affects the task; this never triggers completion-evidence fallback. |
| `codegraph` | Symbol/callgraph lookup over the indexed tree; `.codegraph/` sits at the repository root and its daemon is local | stdio | Fall back to `rg`/`fd` immediately. Indexing is the operator's decision, so a missing index is not an error to fix. |

Two operational facts that have already cost time in this repository:

- **A client establishes MCP connections once, at session start.** If a server
  was down then, restoring the server does not restore the session's access —
  the tools stay absent until the client reconnects (`/mcp` in Claude Code, or a
  new session). Probing the endpoint directly can therefore show it healthy
  while the tools remain unavailable. Report both facts rather than concluding
  the capability does not exist.
- **Never route around an unavailable MCP by calling its backend directly.**
  Report `BLOCKED`. Preserve the profile's upsert templates and relationship
  preflight; see [Evidence record shape](evidence.md#record-shape--记录形状)
  for the record contract and historical rationale.

`.agent-instructions/evidence.md` owns what a CEv1 record must contain and how
the gate is queried. This file only owns whether the capability is reachable.

## Hooks / 钩子

The repository registers `scripts/hooks/beads-consistency.py` on
UserPromptSubmit (scope capture) and Stop (scoped diagnostics):

| Runtime | Repository registration | Blocking transport |
| --- | --- | --- |
| Claude Code | `.claude/settings.json` | Blocker JSON |
| Codex | `.codex/hooks.json` | stderr and exit code 2 |

These entries are part of the effective Hook chain, which can also include
user-level and plugin registrations. Check the active runtime's full applicable
chain before attributing a generic Hook error to this repository script.

The observer reuses the installed shared workflow command matcher and extracts
the project's explicit `<topic> / <subject>` or `fix / <slug>` scope. It never
starts a phase. Scope state is keyed by repository, runtime, and session; a new
unmatched request clears it, while an exact continuation retains it. A stale
turn, absent scope, or unavailable parser produces no blocking diagnostic.
Ordinary requests without an explicit supported scope remain unclassified;
neither dirty files nor an assignee name supplies session ownership.

Normal Stop checks only the selected subject, or the whole topic when the user
explicitly selects that topic. Repository-wide clean-tree and stale-vocabulary
checks remain available through the explicit, non-blocking audit command:

```bash
python3 scripts/hooks/beads-consistency.py --runtime codex --audit
```

Use `--runtime claude` for that client. This audit does not create or claim work.
Repeated scoped reports are suppressed while their notes and relevant document
content remain unchanged; resolved and subsequently recurring mismatches are
reported again. Silence means no attributable new diagnostic, not workflow PASS
or a completed evidence gate. Local scope state uses
`$XDG_STATE_HOME/agentdeck/beads-hook` (default `~/.local/state/agentdeck/beads-hook`);
isolated tests can override `AGENTDECK_BEADS_HOOK_STATE_DIR` and the read-only
parser binding `AGENTDECK_WORKFLOW_HOOK`.

The consistency Hook reports possible disagreement; it does not mutate Beads.
Its current implementation and focused tests establish what it actually checks.
Read a report as diagnostic input, then bind it to the exact record, content
state, task, and active user scope before acting. A dirty path alone does not
establish that the current actor authored it or owns the task.

Reconcile a confirmed, in-scope mismatch under the already granted authority in
[Beads](beads.md). A diagnostic about another task does not authorize claiming it,
changing its verdict, or rewriting its evidence. Explicit read-only or scope
limits remain effective. Report an out-of-scope or contradicted diagnostic rather
than manufacturing a matching state, and do not suppress a valid in-scope problem.

Use Beads for command forms, store location, and status vocabulary; do not
reconstruct them from Hook output or historical task descriptions. The script
uses `.agent-instructions/beads.md` as its repository marker. Its emitted message
and exit behavior, including recursion handling, require runtime-specific tests;
a source comment or silent invocation alone is insufficient proof.

## Shared workflow and handoff Hooks

The workflow and handoff Skills remain usable without their optional Hooks.
Registration, available runtime tools, and verified behavior are distinct facts.
Inspect the active runtime's registration before attributing an error to a Hook.

The workflow Hook distinguishes progress from completion: a claim or write can
satisfy its anti-idling guard without proving the phase complete. Its Skill owns
the completion receipt and continuation contracts. Do not infer completion from
a silent Stop or a successful tool call.

The handoff Hook requires project synchronization rules. Without a configured
rule, it does not enforce automatic status synchronization; explicit Skill use
still discovers the project's existing authorities and pointers. Do not create
a handoff file merely to satisfy the Hook.

## Required command wrappers / 必用命令包装

Three commands must not be invoked directly. Each has a wrapper that supplies
something the bare command cannot infer, and in two cases the bare form silently
produces a wrong record rather than failing.

| Instead of | Use | Why |
| --- | --- | --- |
| `go test …` | `scripts/run-go-test.sh …` | Keeps a large suite from flooding the transcript while preserving the full log and the real exit status. `make check-go-test-runner` keeps the wrapper honest via `scripts/test-run-go-test.sh`. Its exact flags, log handling, and environment variables are documented in `project-rules.md` — read them there, not here. |
| `bd …` | `env BEADS_ACTOR=<codex\|claude-code> ~/.local/state/agentdeck-beads/bin/agentdeck-bd …` | The wrapper requires an actor and sets `BEADS_DIR`. Bare `bd` leaves `BEADS_ACTOR` unset and falls back to `git user.name`, recording the human operator as the author of an agent's comments and status transitions. Comments cannot be retracted. |
| ad-hoc verification | The L0–L4 matrix in `.agent-instructions/project-rules.md` | Only the commands the current risk level selects are required. `make verify` is the aggregate gate; `make release-verify` is L4 and is not a default development, review, commit, or push check. |

`make` targets are the build and verification entry points. `Makefile` is the
list; `project-rules.md` decides which of them the current work actually owes and
documents how each wrapper behaves. This table exists to say *that* a wrapper is
mandatory and what the bare command costs; it deliberately does not restate the
wrappers' behavior, because a second copy of a specification is the thing this
repository keeps having to repair.

## Workflow command syntax / 工作流命令语法

The stage commands `设计` / `开发` / `评审` / `修复` / `复评` are defined by the
`development-workflow` Skill, not by this repository. Its
`references/protocol-commands.md` is the syntax authority.

Before emitting a workflow next instruction, apply the command self-check
defined in [AGENTS.md](../AGENTS.md#runtime-contract--运行时契约).
Use the Skill's current Matching rules for accepted command forms; do not
maintain a separate parser specification in this file.

Scope form follows this repository's own usage: `work-signals / architecture.md`,
`work-signals / reviews/documents.md / R4-F1`. Keep review rules and round
counts out of the scope field — those belong to the topic's `tasks.md`.

## Local-only runtime files / 仅本地的运行时文件

`.gitignore` excludes `.claude/*` and `.codex/*` and then re-includes exactly
the two files that are contract:

| Path | Committed | What it is |
| --- | --- | --- |
| `.claude/settings.json` | Yes | Claude Code hook registration |
| `.codex/hooks.json` | Yes | Codex hook registration |
| `.claude/settings.local.json` | No | Per-operator permissions and overrides |
| `.claude/RESUME.md` | No | A session checkpoint Claude Code writes on its own; it names a `refs/claude/checkpoint-*` snapshot that expires. It is a local artifact, not project state — do not read it as a handoff and do not act on a stale one. |
| `.codegraph/` | No | Local index and daemon state |
| `output/` | No | Locally generated diagram exports, reproducible from the documents that describe them |

The entries are written `dir/*` rather than `dir/` because git cannot re-include
a file whose parent directory is itself excluded.

`CLAUDE.md` is a symlink to `AGENTS.md`. There is one file; editing either edits
both, and they must never be allowed to diverge into two documents.
