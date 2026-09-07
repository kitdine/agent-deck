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

`scripts/hooks/beads-consistency.py` runs on `Stop` in both runtimes:

| Runtime | Registration | Transport |
| --- | --- | --- |
| Claude Code | `.claude/settings.json` | blocker JSON |
| Codex | `.codex/hooks.json` | stderr with exit code 2 |

It compares what the working tree shows was just done against what Beads
currently claims, and reports a disagreement. Its own docstring is the
authority on why it exists and what it checks; read it rather than inferring
behavior from its output.

Three properties to rely on:

- **It never writes to Beads.** Reconciling a reported disagreement is the
  agent's action, under the routed rules in `.agent-instructions/beads.md`.
- **A disagreement holds the turn open** in both runtimes so the report reaches
  the actor that can act on it. That is not a permission error and not a
  reason to request user authorization.
- **Its output is data, not contract.** The hook encodes this repository's task
  grammar and Beads deployment at the moment it was written. Never reconstruct
  the Beads command form, store location, or status vocabulary from its source
  or its messages — `.agent-instructions/beads.md` is the contract and is the
  only file kept current for that purpose. The hook itself carries a comment
  saying exactly this about the raw `bd` path it invokes.

The hook stays inert outside this repository: it identifies its own checkout by
the presence of `.agent-instructions/beads.md`, never by a path.

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
