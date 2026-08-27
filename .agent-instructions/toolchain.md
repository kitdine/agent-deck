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
  Writing CEv1 records over raw HTTP skips the profile's upsert templates and
  relationship preflight, which is exactly how six orphan evidence nodes were
  once created that the gate could not see. `BLOCKED` is the correct answer.

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

**This file deliberately does not copy that syntax.** A copied specification
goes stale silently, and this repository has been bitten by exactly that more
than once — most recently when a design document named `schema v19`, a number
another topic had already landed. What is recorded here is an obligation, which
belongs to the project and does not expire:

> Before emitting any `下一步指令` / `Next instruction`, treat it as input the
> user is about to paste verbatim, and self-check it against that Skill's
> Matching rules: is the command the first non-whitespace text, is it followed
> immediately by `:` or `：`, and is the scope written as `<topic> / <anchor>`?

The failure this prevents: a short stage command followed by a **space** does
not match, so the instruction reads as ordinary prose and no route, stage
authority, Beads transition, or evidence boundary is activated. The Skill's own
Matching section lists `评审 cache_test.go` as a non-matching example. An agent
that has read the parsing rule but applies it only to incoming text will keep
producing unusable instructions; the rule is a specification for both
directions.

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

## 中文摘要

本仓库只声明依赖哪些运行时能力以及缺失时如何降级，不记录它们的地址——端点与安装
路径因机器而异，写进来只会对一台机器正确。

- MCP 三个：`neo4j`（CEv1 证据，不可达时报 `BLOCKED`，绝不静默降级，也绝不绕过
  MCP 直接写后端）、`neo4j-mem`（项目记忆，缺失不影响证据门禁）、`codegraph`
  （符号检索，缺失即回退 `rg`/`fd`）。MCP 连接在会话启动时一次性建立，服务端事后
  恢复不会让本会话重新获得工具，需要客户端重连。
- Hook `beads-consistency.py` 在两个运行时的 `Stop` 上运行，只报告不写 Beads，
  分歧会让 turn 保持打开。它的输出是数据不是契约：Beads 命令形式、库位置与状态
  词汇一律以 `.agent-instructions/beads.md` 为准。
- 三条必用包装：`run-go-test.sh`、带 `BEADS_ACTOR` 的 `agentdeck-bd`、以及按
  `project-rules.md` 的 L0–L4 选择验证命令。裸 `bd` 会把 agent 的写入记成操作者
  本人，且评论无法撤销。
- 阶段命令语法属于 Skill，本文件只记录**自检义务**而不复制规则：输出下一步指令
  前，按 Skill 的 Matching 规则自检命令后是否紧跟 `:` 或 `：`。空格分隔不匹配。
