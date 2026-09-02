# AGENTS.md

This file defines the operating rules for AI agents working in this project.
本文件定义 AI Agent 在本项目中的工作规则。

## Project Overview / 项目概览

- Project name / 项目名称: `AgentDeck`
- Purpose / 项目目标: One local CLI for Codex and Claude provider switching,
  usage cost, session search, extension inventory, and portable backup.
- Primary stack / 主要技术栈: Go and SQLite. Superseded Python 3 and Bash
  behavior remains available only through historical documents and Git history.
- Supported environments / 支持环境: macOS first, with portable core contracts
  for later Windows and Linux support.
- Stable documentation entry / 稳定文档入口: `docs/README.md`
- Current status / 当前状态: `docs/status.md`
- Primary product contract / 主要产品契约: `docs/specs/cli-design.md`; read the
  sections routed by the current topic/task rather than loading it front to back.

Authoritative project facts live in code, tests, configuration, repository
history, and the documents explicitly identified below. Chat history is not a
source of truth.

项目事实以代码、测试、配置、仓库历史及下文明确列出的权威文档为准。聊天记录不是
事实来源。

## Workspace and Repository Boundaries / 工作区与仓库边界

The workspace may contain one or more independent repositories:

| Path           | Repository or unit | Responsibility                                               | Release unit |
| -------------- | ------------------ | ------------------------------------------------------------ | ------------ |
| `.`            | `AgentDeck`        | Local AI provider, usage, session, extension, and backup CLI | Yes          |
| Not applicable | Not applicable     | No sibling repository is in scope                            | No           |

- Treat each listed repository as an independent ownership and release unit.
- Run Git commands from the repository they target, or use `git -C <path>`.
- Do not merge repositories, move ownership boundaries, or introduce a
  monorepo structure unless explicitly requested.
- Workspace-level files may sit outside any repository. Keep them current, but
  do not imply that they are committed when they are not.
- Never modify repositories, sibling directories, services, or infrastructure
  outside the user's stated scope.

每个仓库都应视为独立的所有权和发布单元。未经明确要求，不得跨仓库扩散改动、调整
边界或改造成 monorepo。

## Scope and Authorization / 范围与授权

- Do exactly what the user requested and keep all changes within that scope.
- Read-only inspection and proportionate verification are allowed when needed
  to understand or validate the requested work.
- Do not make unrelated fixes, refactors, formatting changes, dependency
  upgrades, migrations, or documentation rewrites.
- Preserve user work in a dirty worktree. Never revert, overwrite, or discard
  changes you did not create.
- Ask before any action that materially expands scope or changes external state.
- Do not infer authorization to commit, push, tag, release, publish, deploy,
  open a pull request, create a branch, or create a worktree.
- A request to implement, fix, validate, or finish work does not by itself
  authorize those Git, release, or deployment actions.
- If an action is irreversible, destructive, externally visible, or affects
  production-like data, obtain explicit approval immediately before it.

严格执行用户给定范围。实现或修复授权不自动包含提交、推送、发版、部署、建分支、
建 worktree 或创建 PR 的授权。

## Delegation and Model Tier / 委派与模型层级

These rules apply when the user has asked for delegation or a cheaper model.
Neither is something to reach for on its own — this section bounds them, it does
not encourage them.

**A subagent may write implementation, and nothing else.** Production code and
tests inside an already-approved task boundary are its writable surface. Review
records, `tasks.md`, `docs/status.md`, completion evidence, and Beads are not,
and neither is any Git delivery action.

The reason is the single-writer model this file already establishes. A gate that
several writers can reach stops answering "who asserted this, against which
content state" — and that question is the only thing a gate is for. The same
constraint that keeps repository documents, CEv1, and Beads from mirroring each
other keeps a subagent out of all three.

**A subagent's report is input, not evidence.** Before anything it produced
reaches an authoritative record, the main agent verifies it against the
repository. A finding, a test result, or a claim of completeness that is only
asserted in a subagent's summary has not been checked; recording it as though it
had is how a review record starts asserting things the target does not have. A
subagent may gather review material read-only, but the verdict and the record
stay with the main agent — **independence comes from a cold context and a
separate role, not from a process boundary.**

**A lower model tier is for work a machine can grade.** Search, bulk mechanical
rewrites, boilerplate tests, formatting cleanup — anything whose result the
compiler, the test suite, or a project script decides objectively. Design,
review, and re-review keep the session's default tier: their output is a
judgment that a later reader cannot re-derive cheaply, and it is what makes a
gate believable. Say which tier was used in the dispatch record when it was not
the default.

The failure this avoids is specific and recent: a wrong line in code fails a
test, while a wrong line in a design gets implemented, copied into the
downstream documents, and repaired only after several rounds notice it.

委派与降级仅在用户要求时适用。subagent 只能写已批准 task 边界内的生产代码与测试；
评审记录、`tasks.md`、`docs/status.md`、验收证据、Beads 与任何 Git 交付动作都不在
其可写范围，因为单写者模型是门禁可追溯的前提。subagent 的报告是输入而非证据，进入
权威记录前必须由主 agent 对着仓库核实；独立性来自冷上下文与不同职责，不来自进程
边界。低层级模型只用于结果可被编译器、测试或脚本客观判定的机械任务；设计、评审、
复评保持会话默认层级，并在调度记录中说明非默认的层级选择。

## Project Workflow Authorities / 项目工作流权威

The project-defined workflow skills are the primary workflow authorities within
their declared scope.

- `development-workflow` is the primary authority for design, development,
  review, fix, re-review, and full-delivery workflow triggers.
- `handoff-sync` is the primary authority for synchronizing handoff documents,
  repository state, requirement status, and other project status records.
- The external Beads store is the primary authority for Agent task dispatch,
  dependency readiness, atomic claims, leases, and cross-Agent handoff. It is
  not a product-requirement, phase-status, review-verdict, evidence, Git, or
  release authority. Read `.agent-instructions/beads.md` before the first Beads
  read or write in a session, and resolve task IDs from live Beads state. A
  phase command that ends in a status, label, comment, or claim transition is
  Beads coordination even when the request never says "Beads", so the trigger is
  the operation you are about to perform, not how the work was described.
  Never reconstruct the command form, the store location, or the status
  vocabulary from a hook script, a task description, or another agent's earlier
  transcript. Those are data written at some past moment; this file is the
  contract, and only it is kept current.

The authorities use a single-writer model:

- Repository plans, contracts, review records, and status documents own
  requirements, phase state, and review verdicts.
- CEv1 owns the evidence status of a named WorkUnit for one exact
  `target_content_state`.
- Beads owns dispatch, dependencies, claims, leases, and handoff. Its `closed`
  state means no further coordination is pending; it is not a phase verdict or
  evidence result.

A Beads task and a CEv1 WorkUnit are different entity types and need not map
one-to-one. Cross-system identifiers correlate records; do not mirror status
or evidence between them. A Beads transition does not query or invalidate
CEv1, and a CEv1 result does not create, claim, close, or reopen a Beads task.

### Stage Command Authority / 阶段指令授权

A real user `设计`, `开发`, `评审`, `修复`, or `复评` command authorizes every
mandatory in-scope transition defined by that stage and this project. That
includes review and status artifacts, repository-scoped idempotent
completion-evidence writes and gate queries, Beads claims/status/labels/comments,
and the containing-unit boundary created when the final Task closes. Perform
these transitions without additional user authorization; stage authority is not
consumed by one tool call and remains active through required post-phase
synchronization.

A generated next instruction cannot grant new authority and cannot erase the
authority of the active real-user stage command. Stage authority still excludes
commit, push, release, or deploy, plus destructive actions and out-of-scope
work. Those remain exact-action checkpoints under their existing project rules.

If a permission system blocks one exact action, enter the shared non-phase
`AUTHORIZATION_WAIT` state. Skip all unrelated hooks, checks, document or status
work, CEv1 discovery, and Beads queries while waiting. Offer only two choices:
approve the exact action and resume from it, or stop and perform another task.
An approval mismatch over an already authorized stage transition is escalated as
that exact denied action; never ask the user to restate the stage's business
authorization.

- When a request matches either skill, invoke it before optional generic
  workflows such as brainstorming, plan-writing, TDD orchestration, or
  branch-finishing.
- Generic skills may supplement implementation, debugging, review, or
  verification, but must not replace these project workflows or create a
  competing plan or status source.
- A third-party or externally authored skill is a generic skill under the rule
  above, regardless of how specific its name sounds. Before invoking one, confirm
  its subject matches the actual target: a skill whose scoring anchors assume an
  implemented surface cannot judge an unimplemented design, and reporting its
  number anyway asserts a measurement that was never taken.
- An external skill's findings become project record only after independent
  verification against the repository, and only through the artifact the project
  workflow already defines. Its own scratch directories, scorecards, and handoff
  files are temporary diagnostics: extract the verified findings, then remove
  them rather than committing a parallel record.
- Prefer a lower-cost tool that fits over a broader one that overruns. Reserve
  multi-agent panels and full audits for high-value or high-risk targets, and say
  which mode was used in the review record.
- If a required project workflow skill is unavailable at runtime, report that
  limitation and follow the fallback process documented by the project. Do not
  silently substitute an unrelated generic workflow.
- Explicit user instructions and higher-priority system or developer
  instructions always take precedence.

项目定义的工作流技能在其适用范围内优先于通用可选流程，但不能覆盖用户当前指令，
也不能覆盖系统或开发者级指令。

## Runtime Contract / 运行时契约

The routed table below answers "I am doing X, which rules apply". This section
answers a question routing cannot: **what this repository expects to exist
around the agent.** Those facts are needed before the work reveals that it needs
them, so they are stated here rather than deferred. Detail, degradation rules,
and configuration guidance live in
[Toolchain](.agent-instructions/toolchain.md).

**Stage command syntax is a self-check obligation, not a copied rule.** The
commands this file grants authority to — `设计`, `开发`, `评审`, `修复`,
`复评` — are defined by the `development-workflow` Skill, and its
`references/protocol-commands.md` is their syntax authority. Copying that syntax
here would create a specification copy that goes stale silently, which this
repository has already paid for more than once. What is owed here instead:
before emitting any `下一步指令` / `Next instruction`, treat it as text the user
is about to paste verbatim and self-check it against that Skill's Matching
rules. A short stage command followed by a space does not match and activates no
route, no stage authority, no Beads transition, and no evidence boundary. Scope
is written `<topic> / <anchor>`. A parsing rule that has been read is a
specification for output as well as for input.

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

运行时契约在此内联而非路由，因为用到时才查已经太晚。阶段命令语法属于 Skill，这里
只记录自检义务：输出下一步指令前按 Skill 的 Matching 规则自检，短命令后用空格分隔
不匹配，任何路由与授权都不会触发。仓库声明依赖哪些能力及缺失时如何降级，不记录
随机器而异的地址。

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
