# AGENTS.md

This file defines the operating rules for AI agents working in this project.
本文件定义 AI Agent 在本项目中的工作规则。

## Project Overview / 项目概览

AgentDeck is a local CLI for Codex and Claude provider switching, usage cost,
session search, extension inventory, and portable backup. The primary stack is
Go and SQLite; macOS is the primary supported platform.

Use current code, tests, configuration, Git history, and the routed
authoritative documents to establish project facts. Treat chat history and
memory as discovery aids; verify their claims against current sources.

## Workspace and Repository Boundaries / 工作区与仓库边界

This repository is AgentDeck's sole ownership and release unit. No sibling
repository, service, or infrastructure is in scope unless the user explicitly
includes it.

Run Git commands from the repository they target, or use `git -C <path>`.
Do not merge repositories, move ownership boundaries, or introduce a monorepo
without explicit authorization.

For authorized workspace files outside Git, distinguish local updates from
committed repository changes.

## Scope and Authorization / 范围与授权

- Complete the user's authorized scope. Make routine implementation decisions
  within it; ask only when a material decision or scope expansion needs the user.
- Read-only inspection and proportionate verification are allowed. Do not make
  unrelated fixes, refactors, formatting changes, dependency upgrades,
  migrations, or documentation rewrites.
- Preserve existing work in a dirty worktree. Never revert, overwrite, or
  discard changes you did not create.
- Commit, push, tag, release, publish, deploy, PR creation, branch creation,
  and worktree creation require explicit authorization for the action.
  Implementation or repair authorization alone does not grant it.
- Obtain explicit approval immediately before destructive or irreversible
  actions, or actions affecting production-like data.
- For mandatory workflow state transitions already authorized by a real user
  stage command, follow Stage Command Authority below without asking again.

## Delegation and Model Tier / 委派与模型层级

- Delegate work or use a lower model tier only when the user requests it.
- A subagent may modify only production code and tests within an approved task.
  It may gather review material read-only. The main agent owns review verdicts,
  review records, tasks.md, docs/status.md, completion evidence, Beads, and all
  Git delivery actions.
- Treat subagent reports as input, not evidence. Before recording their claims
  in an authoritative source, the main agent verifies them against repository
  content and direct evidence.
- Review independence requires a cold context and a separate role; a separate
  process alone does not establish independence.
- Use lower tiers only for objectively checkable work, such as mechanical
  rewrites, boilerplate tests, or formatting. Design, review, and re-review
  retain the session's default tier. Record any non-default tier in the
  dispatch record.

## Project Workflow Authorities / 项目工作流权威

Within their declared triggers, `development-workflow` owns phase execution
and `handoff-sync` owns handoff and status synchronization.

Each system owns a distinct kind of state:

| Authority | Owns |
| --- | --- |
| Repository plans, contracts, review records, and status documents | Requirements, phase state, and review verdicts |
| CEv1 | Evidence status of a named WorkUnit for one exact target_content_state |
| Beads | Dispatch, dependencies, claims, leases, and cross-agent handoff |

Before the first Beads operation in a session, read
`.agent-instructions/beads.md`. This applies to stage-internal coordination
even when the user does not name Beads. Resolve task IDs from live state;
take command forms, store location, and status vocabulary from that authority,
not hooks, task descriptions, or previous transcripts.

Beads tasks and CEv1 WorkUnits need not map one-to-one. Cross-system IDs
correlate records; do not mirror state between systems. Beads `closed` means
no coordination remains, not that a review or evidence gate passed.
A Beads transition alone does not query or invalidate CEv1; a CEv1 result
alone does not create, claim, close, or reopen a Beads task.

### Stage Command Authority / 阶段指令授权

A real user `设计`, `开发`, `评审`, `修复`, or `复评` command authorizes all
mandatory in-scope transitions required by that stage and this project:
review and status artifacts, repository-scoped idempotent completion-evidence
writes and gate queries, Beads claims/status/labels/comments, and the
containing-unit boundary when the final Task closes.

Perform these transitions without additional user authorization. Authority
persists through required post-phase synchronization and is not consumed by
tool calls or writes. Generated next instructions neither grant nor revoke it.

Stage authority excludes commit, push, release, or deploy, destructive actions,
and out-of-scope work. Follow Scope and Authorization for those actions.

If a permission system denies an exact action, enter the non-phase
`AUTHORIZATION_WAIT` state. Preserve the pending action and active stage;
skip unrelated hooks, checks, status work, CEv1 discovery, and Beads queries.

Report the denied action and the permission system's stated reason. Offer
approval of that exact action or stopping to do another task. On approval,
resume from the pending action without repeating completed work. Do not ask
the user to restate stage authority already granted.

### Supporting Skills / 辅助技能

- When a request matches `development-workflow` or `handoff-sync`, apply that
  workflow before optional supporting skills. Supporting skills must not
  replace it or create competing plans or status authorities.
- Select skills by the actual target and evidence they require, not their name
  or specificity. Do not score an unimplemented design with criteria that
  require an implemented surface.
- Independently verify external findings against the repository before adding
  them to the workflow's existing records. Extract verified findings from
  temporary skill-generated reports, then remove those temporary artifacts.
- Use the smallest suitable tool or review method. Reserve full audits and
  multi-agent panels for high-value or high-risk work, subject to Delegation
  and Model Tier above. Record the review method used.
- If a required workflow skill is unavailable, report the limitation and use
  the project's documented fallback. Do not silently substitute another skill.
- System and developer instructions, and the user's explicit instructions,
  take precedence over skill guidance.

## Runtime Contract / 运行时契约

Required capabilities and command wrappers are summarized below. Read
[Toolchain](.agent-instructions/toolchain.md) for configuration and
capability-specific degradation rules.

**Stage command syntax.** The `development-workflow` Skill's
`references/protocol-commands.md` owns command matching. Before emitting a
`下一步指令` / `Next instruction`, validate it against those rules as text
the user will paste verbatim. Use this project's `<topic> / <anchor>` scope
form. Do not infer phase authority from a command that does not match.

**Runtime capabilities.** This repository declares what it depends on and how to
behave without it; it does not store endpoints, which differ per machine.

| Capability | Used for | Without it |
| --- | --- | --- |
| `neo4j` MCP | `completion-evidence/v1` gates and records | `BLOCKED`, reported exactly; never silently skipped and never worked around by calling the backend directly |
| `neo4j-mem` MCP | Durable project memory | Continue from repository sources; does not trigger evidence fallback |
| `codegraph` MCP | Symbol and callgraph lookup | Fall back to `rg`/`fd`; a missing index is a choice, not a defect |
| `scripts/hooks/beads-consistency.py` | `Stop` hook in both runtimes; reports Beads state the tree has moved past | Reconciliation is the agent's action — the hook only ever reports and never writes to Beads |

MCP connections are established once at session start, so a server restored
mid-session stays unavailable until the client reconnects. A reachable endpoint
and an available tool are two different facts.

**Required wrappers.** `scripts/run-go-test.sh` instead of bare `go test`;
`env BEADS_ACTOR=<actor> …/agentdeck-bd` instead of bare `bd`, which otherwise
records the human operator as the author of an agent's writes. Verification
commands are selected by the L0–L4 matrix in
[Project Rules](.agent-instructions/project-rules.md), not chosen ad hoc.

`CLAUDE.md` is a symlink to this file. Editing either edits both.

## Routed Project Instructions / 按需项目规则

Read `AGENTS.md` for every repository task, then load only the routed
authorities required by the current work. A task spanning multiple rows reads
each applicable file; do not read every conditional guide by default.

| Current work | Additional authority |
| --- | --- |
| Implementation, testing, failure diagnosis, dependencies, security, configuration, or runtime behavior | [Project Rules](.agent-instructions/project-rules.md) — relevant section only |
| Commit, push, release notes, release preparation, or preflight | [Project Rules](.agent-instructions/project-rules.md) — delivery and release sections |
| Documentation, topic lifecycle, status matrices, or handoff | [Documentation Workflow](docs/documentation-workflow.md) and the documentation/handoff sections of [Project Rules](.agent-instructions/project-rules.md) |
| Beads dispatch, claims, dependencies, comments, or lifecycle | [Beads](.agent-instructions/beads.md) |
| Defect triage, bug-lane selection, or a Lane A fix record | [Beads](.agent-instructions/beads.md) — Bug lane, and [Documentation Workflow](docs/documentation-workflow.md) — Fix records |
| Completion evidence, WorkUnits, Topic gates, or Neo4j project memory | [Evidence](.agent-instructions/evidence.md) |
| Review-record creation or updates | [Review Records](.agent-instructions/review-records.md) |
| Branching, merging, version assembly, or integration review | [Branching](.agent-instructions/branching.md) |
| MCP availability, hook behavior, command wrappers, or local-only runtime files | [Toolchain](.agent-instructions/toolchain.md) |

The stable documentation index is [`docs/README.md`](docs/README.md), current
execution state is [`docs/status.md`](docs/status.md), and the primary product
contract is [`docs/specs/cli-design.md`](docs/specs/cli-design.md).
