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
  release authority. Read `.agent-instructions/beads.md` only when current work
  requires Beads coordination, and resolve task IDs from live Beads state.

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
| Completion evidence, WorkUnits, Topic gates, or Neo4j project memory | [Evidence](.agent-instructions/evidence.md) |
| Review-record creation or updates | [Review Records](.agent-instructions/review-records.md) |
| Branching, merging, version assembly, or integration review | [Branching](.agent-instructions/branching.md) |

The stable documentation index is [`docs/README.md`](docs/README.md), current
execution state is [`docs/status.md`](docs/status.md), and the primary product
contract is [`docs/specs/cli-design.md`](docs/specs/cli-design.md).
