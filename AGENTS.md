# AGENTS.md

This file defines the operating rules for AI agents working in this project.
本文件定义 AI Agent 在本项目中的工作规则。

Replace every project-specific placeholder before adopting this template. Remove optional
sections that do not apply. Project-specific extensions belong in the final
section and must not weaken the core safety and authorization rules.

采用本模板前必须替换所有项目特有占位符。不适用的可选章节应直接删除。项目特有
约定统一放在末尾扩展区，且不得削弱核心安全与授权规则。

## RTK Command Policy / RTK 命令规范

Always prefix shell commands with `rtk`. RTK reduces command output while
preserving command behavior. If no specialized filter exists, it passes the
command through unchanged.

所有 shell 命令必须以 `rtk` 开头。RTK 只压缩命令输出，不改变命令行为；没有专用
过滤器时会原样执行命令。

```bash
rtk git status
rtk git diff
rtk read <file>
rtk grep <pattern> <path>
rtk test <test-command>
rtk lint <lint-command>
rtk summary <command>
```

In command chains, prefix every segment:

```bash
rtk git add <files> && rtk git commit -m "<message>"
```

Use `rtk proxy <command>` when unfiltered output is required for debugging.
Do not bypass RTK merely for convenience.

## Project Overview / 项目概览

- Project name / 项目名称: `AgentDeck`
- Purpose / 项目目标: One local CLI for Codex and Claude provider switching,
  usage cost, session search, extension inventory, and portable backup.
- Primary stack / 主要技术栈: Go and SQLite. Superseded Python 3 and Bash
  behavior remains available only through historical documents and Git history.
- Supported environments / 支持环境: macOS first, with portable core contracts
  for later Windows and Linux support.
- Primary entry document / 项目入口文档:
  `docs/specs/2026-07-13-agentdeck-cli-design.md`

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

## Repository Relationships / 仓库依赖关系

Document cross-repository dependencies here:

```text
Not applicable: this repository has no cross-repository dependencies.
```

When a provider change must be consumed by another repository:

1. Validate the provider independently.
2. Obtain explicit authorization before committing, tagging, releasing, or
   publishing it.
3. Update the consumer to an immutable released version when the project
   requires release-based dependencies.
4. Refresh lockfiles, checksums, generated dependency metadata, or vendored
   sources using the project's official commands.
5. Validate the consumer without relying on undeclared local overrides.
6. Keep provider behavior changes and consumer dependency updates in separate
   commits unless the user explicitly requests otherwise.

Do not commit temporary local dependency overrides unless the project explicitly
defines them as permanent configuration.

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

- `dev-workflow-triggers` is the primary authority for design, development,
  review, fix, re-review, and full-delivery workflow triggers.
- `handoff-sync` is the primary authority for synchronizing handoff documents,
  repository state, requirement status, and other project status records.
- When a request matches either skill, invoke it before optional generic
  workflows such as brainstorming, plan-writing, TDD orchestration, or
  branch-finishing.
- Generic skills may supplement implementation, debugging, review, or
  verification, but must not replace these project workflows or create a
  competing plan or status source.
- If a required project workflow skill is unavailable at runtime, report that
  limitation and follow the fallback process documented by the project. Do not
  silently substitute an unrelated generic workflow.
- Explicit user instructions and higher-priority system or developer
  instructions always take precedence.

项目定义的工作流技能在其适用范围内优先于通用可选流程，但不能覆盖用户当前指令，
也不能覆盖系统或开发者级指令。

## Workflow Skills and Authority / 工作流技能与权威边界

Superpowers is opt-in and provides task-level discipline; it is not the global
workflow authority for this project.

- Never invoke `superpowers:using-superpowers`.
- Do not apply the "1% applicability" rule.
- Automatically use `verification-before-completion` before claiming that work
  is complete, fixed, passing, or ready.
- Use `systematic-debugging` for non-trivial, reproducible technical failures
  before proposing or implementing fixes.
- Use `receiving-code-review` when evaluating review feedback, especially when
  the feedback is ambiguous or technically questionable.
- Use TDD, brainstorming, worktrees, branch-finishing, plan-writing, or
  subagent-based workflows only when explicitly requested, required by
  higher-priority instructions, or clearly justified by task scale and risk.
- Do not create worktrees, branches, commits, pull requests, or materially
  expand subagent usage without explicit user approval.
- Do not introduce a second planning or project-state system when the project
  already defines an authoritative one.
- When GSD is active, GSD owns project state and plans. Superpowers may provide
  implementation, debugging, review, and verification discipline, but must not
  create or maintain a competing plan source.
- User instructions and applicable project instructions take precedence over
  optional skill workflows. System and developer instructions always retain
  higher priority.

Superpowers 默认不接管项目流程，仅在明确要求、上级指令要求，或任务规模与风险确有
需要时使用。它可以补充实现与验证纪律，但不能建立竞争性的计划或状态来源。

## Standard Work Stages / 标准工作阶段

Unless the project workflow or user request defines otherwise, keep these stages
separate:

1. **Design / 设计**: clarify requirements, constraints, alternatives, and
   acceptance criteria. Do not implement without approval when a design gate is
   required.
2. **Development / 开发**: implement only the approved scope and add
   proportionate tests.
3. **Review / 评审**: inspect correctness, regressions, security, deployment
   behavior, data risk, and missing coverage. Review is read-only unless fixes
   are explicitly authorized.
4. **Fix / 修改**: address only approved findings and rerun relevant checks.
5. **Re-review / 复评**: independently confirm previous findings are closed and
   no new regressions were introduced.
6. **Delivery / 交付**: commit, push, release, deploy, or open a PR only to the
   extent explicitly authorized.

Do not collapse "fix implemented" and "review passed" into the same state.
Stage boundaries do not invalidate verification evidence by themselves. Bind
evidence to the exact content state and risk; do not rerun the same full suite
merely because work moved from development to review, re-review, or delivery.

## Change Discipline / 修改纪律

- Follow existing architecture, naming, formatting, and local helper patterns.
- Prefer the smallest coherent change that satisfies the requirement.
- Add abstractions only when they remove meaningful duplication or complexity.
- Use structured parsers and APIs for structured data.
- Keep generated files generated; update them through their source or official
  generation command.
- Add comments only when they explain non-obvious intent, constraints, or risk.
- Use ASCII by default unless the file or user-facing content requires Unicode.
- Do not leave temporary diagnostics, credentials, debug output, or local-only
  configuration in the final diff.

## Testing and Verification / 测试与验证

AgentDeck has the verification command catalog below. It is not an
unconditional checklist for every stage or every change:

In the managed sandbox, set `GOCACHE=/private/tmp/agent-deck-go-build` for
every Go test, vet, and build command. If a first cross-build also needs to
download modules, set `GOMODCACHE=/private/tmp/agent-deck-go-mod` for that
command rather than writing to the user Go module cache.

```bash
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOOS=darwin GOARCH=arm64 go build -mod=vendor -trimpath ./cmd/agentdeck
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOOS=darwin GOARCH=amd64 go build -mod=vendor -trimpath ./cmd/agentdeck
rtk test make check-arm64-size
rtk test make release-verify
```

- Scale verification to the risk and blast radius of the change.
- Select the smallest complete evidence set from this risk matrix:

| Level | Typical change | Required evidence |
| ----- | -------------- | ----------------- |
| L0 | Documentation, comments, ignore rules | Relevant format/link/discovery checks and `git diff --check` |
| L1 | Localized package or renderer behavior | Affected targeted tests |
| L2 | Shared CLI, parser, SQLite schema, persisted or JSON/text contract | Targeted tests plus `go test -mod=vendor ./...` |
| L3 | Concurrency, credentials/privacy, migration execution, build or installer behavior | L2 plus only the relevant race, vet, cross-build, size, install, or privacy checks |
| L4 | Release artifact readiness or explicit full release validation | `make release-verify` as the aggregate gate |

- Path-based routing is a hint; assess actual behavior and failure modes.
- During development, run fast targeted checks. Run the selected broader level
  once after the final relevant edit, not once per workflow stage.
- Review and re-review may add independent targeted evidence while reusing a
  broader result bound to the same unchanged content state.
- A prior result may be reused only when its command/result is available in the
  current continuous workflow and a fresh status/diff/tree check proves relevant
  content, dependencies, toolchain, configuration, generated files, and relevant
  environment are unchanged. If any premise is unknown, rerun the relevant check.
- Do not run every component and then `release-verify`, which already contains
  those components, unless diagnosing a failing aggregate gate.
- Commit and push of an already verified unchanged tree require staged/commit
  tree and hook-effect checks, not another product test run.
- Verify behavior at the source of truth. Browser-visible behavior requires
  browser verification; database behavior requires database checks; deployment
  behavior requires a real runtime or documented equivalent.
- A successful build does not prove runtime correctness.
- Do not claim success based on unverifiable historical output, source inspection
  alone, or tests run before the final relevant edit without an exact-state check.
- Record commands that could not be run and state the remaining risk.
- Remove generated caches and temporary artifacts before delivery:

```bash
rtk proxy rm -rf bin/__pycache__
```

## Failure Diagnosis / 故障诊断

For non-trivial reproducible failures:

1. Reproduce the failure and capture the exact evidence.
2. Determine the failing layer and compare expected with actual behavior.
3. Inspect recent relevant changes and environmental differences.
4. Form one testable root-cause hypothesis at a time.
5. Validate the hypothesis before implementing a fix.
6. Add or update regression coverage.
7. Rerun targeted and required full verification after the final change.

Do not patch symptoms blindly or claim a root cause without evidence.

## Commit and Push Rules / 提交与推送规则

Commits and pushes require explicit authorization unless the user has invoked a
documented workflow that explicitly grants that authority.

- Use English Conventional Commit messages:

```text
feat: ...
fix: ...
docs: ...
test: ...
refactor: ...
chore: ...
```

- Each commit must contain one logical change.
- Do not mix behavior changes, dependency upgrades, generated output, and
  unrelated documentation unless explicitly requested.
- Stage only intended files. Before committing, inspect:

```bash
rtk git diff --cached --stat
rtk git diff --cached --name-only
rtk git diff --cached
```

- When the staged tree is the exact tree already verified, committing does not
  trigger product verification again. After commit, verify the commit tree,
  repository status, and whether hooks rewrote files; rerun affected checks only
  if content changed.

- Before pushing, verify the target repository, branch, remote, commit range,
  and required checks.
- Never force-push, rewrite shared history, or delete branches without explicit
  approval.
- Never use destructive commands such as `git reset --hard` or
  `git checkout -- <path>` to discard work unless explicitly authorized.

## Dependencies and Vendoring / 依赖与 Vendor

Dependency changes must be intentional and isolated.

- Use the project's package manager and lockfile.
- Do not upgrade unrelated dependencies opportunistically.
- If dependencies are vendored, regenerate vendor content using the official
  command and commit the manifest, lock/checksum files, and affected vendored
  files together.
- Validate release builds without undeclared workspace-local dependency
  overrides.
- Confirm dependency metadata, vendored source, and the published version agree.
- Document the required commands:

```bash
rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go mod tidy
rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go mod vendor
```

Delete this section if the project has no vendoring or dependency lock policy.

## Security and Sensitive Data / 安全与敏感数据

- Never commit credentials, tokens, private keys, cookies, production data,
  generated secret files, or environment files containing real secrets.
- Use fake values in tests and examples.
- Do not print or log secrets, even temporarily.
- Keep authentication and authorization checks explicit and covered by tests.
- Treat external input, file paths, SQL, templates, and shell arguments as
  untrusted.
- Follow least privilege for services, databases, CI, and deployment accounts.
- Stop and request direction if the task would expose sensitive data or weaken a
  security boundary.
- Report suspected credential exposure immediately; do not silently rotate,
  revoke, or delete external resources without authorization.

## Configuration Policy / 配置策略

Environment-specific values must remain configurable rather than being changed
in source for local convenience.

Examples that normally require configuration:

- service URLs, hostnames, proxies, and environment-specific ports
- credentials, tokens, encryption keys, and database DSNs
- tenant, namespace, account, region, or deployment identifiers
- production feature switches and integration endpoints
- machine-specific paths and container or cluster names

Reasonable code defaults may include documented development ports, non-secret
timeouts, and user-interface defaults. Examples in documentation must be clearly
labeled as examples.

不得为切换本地或部署环境而提交硬编码改动。优先使用环境变量、配置文件、密钥系统或
项目已有的运行时注入方式。

## Runtime and Deployment Verification / 运行与部署验证

Define project-specific runtime constraints here:

- Runtime topology / 运行拓扑: One on-demand `agentdeck` binary uses
  `~/.agentdeck/agentdeck.sqlite3`, a machine-bound private
  `~/.agentdeck/credential.key`, `~/.codex/config.toml`, and
  `~/.claude/settings.json`. The optional watcher is foreground-only.
- Allowed connectivity / 允许的连接方式: Normal commands and tests require no
  network. Only an explicit `agentdeck usage price update` downloads the
  configured public price catalog; provider hosts are consumed by Codex or
  Claude, not probed by AgentDeck.
- Prohibited exposure / 禁止的暴露方式: The tools must not expose network ports or alter host network configuration.
- Test-data policy / 测试数据策略: Tests use temporary homes, synthetic machine
  identities, synthetic session logs, fake credentials, and isolated encrypted
  credential stores. Real credentials, real key files, and real session sources
  are not used by automated tests.
- Deployment command / 部署命令: Not applicable; this repository produces a
  local binary, not a deployed service.
- Rollback procedure / 回滚方式: Use the operation journal and redacted client
  backup for interrupted configuration changes. Portable restore targets an
  empty AgentDeck state root and never automatically rewrites client config.

- Do not change network exposure, persistent volumes, shared databases, or
  production-like data merely to simplify validation.
- Prefer isolated environments for unreleased builds and migration tests.
- Confirm the running artifact actually contains the change; source code on disk
  is not proof that a container, service, or deployment was refreshed.
- Validate logs, health, data behavior, and user-visible behavior as applicable.
- Deployment and rollback require explicit authorization.

## Documentation / 文档规范

Authoritative documents:

| Purpose                                        | Path                                                       |
| ---------------------------------------------- | ---------------------------------------------------------- |
| Documentation index / 文档索引                 | `docs/`                                                    |
| Current execution status / 当前执行状态        | `docs/plans/2026-07-13-agentdeck-cli.md`                   |
| Requirements catalog / 需求目录                | `docs/specs/2026-07-13-agentdeck-cli-design.md`            |
| Architecture or API contract / 架构或 API 契约 | `docs/specs/2026-07-13-agentdeck-cli-design.md`            |
| Development guide / 开发指南                   | `AGENTS.md`                                                |
| Deployment guide / 部署指南                    | Not applicable; this repository has no deployment process. |
| Archived / superseded documents / 归档文档      | `docs/archive/` (see `docs/archive/README.md`)              |

Naming and lifecycle rules are documented once, in `docs/README.md`'s "Naming
Convention" and "Document Lifecycle" sections — read there for the filename
patterns (`docs/specs/YYYY-MM-DD-<topic>-design.md`,
`docs/plans/YYYY-MM-DD-<topic>.md` with dated Follow-Up subsections, and the
plan's `Backlog / Future Feature Ideas` section for unscoped ideas). Do not
duplicate that index here; it changes as documents are added or archived.

- Treat code, tests, configuration, and repository history as current truth.
- Update the closest living document when behavior, contracts, requirements, or
  operational procedures change.
- Prefer updating a living document over creating a dated review or record.
- Create a one-off document only for a genuinely temporary investigation,
  incident, or phased plan.
- Archive superseded documents into `docs/archive/` instead of deleting them
  (`git mv`, not `rm`). Record why they were archived and where their
  conclusions now live in `docs/archive/README.md`.
- Keep the documentation index (`docs/README.md`) synchronized with active
  documents; do not re-list archived files there.
- Mark substantial documents with a status such as `active`, `reference`, or
  `historical`.
- Do not leave completed one-off plans marked active.
- Sweep documentation at the close of major delivery milestones.

## Handoff and Project State / 交接与项目状态

The handoff file is a pointer to authoritative state, not a duplicate narrative
store.

- Handoff file / 交接文件: Not applicable; no dedicated handoff file exists.
- Authoritative status source / 权威状态来源:
  `docs/plans/2026-07-13-agentdeck-cli.md`
- Requirements source / 需求来源:
  `docs/specs/2026-07-13-agentdeck-cli-design.md`
- Repository history / 仓库历史: `.` (`.git`)

At the start of resumed work:

1. Read this instruction file and the handoff pointer.
2. Inspect status and recent history for every repository in scope.
3. Read the relevant authoritative status, requirement, and contract documents.
4. Verify drift-prone facts in the current environment.
5. Do not rely only on prior chat summaries.

Use `handoff-sync` when the user requests status synchronization or when the
project's documented hook or workflow requires it. Do not hand-edit generated or
skill-owned handoff sections outside that workflow.

## Unresolved Issues / 未解决事项

- Fix in-scope issues immediately when authorized and safe.
- If an issue cannot be resolved within scope, record it in the project's
  authoritative tracker or add a precise `TODO` at the relevant code location
  when that is the documented convention.
- A useful `TODO` states the unresolved behavior, why it remains, and what
  condition or decision would allow removal.
- Do not use vague TODOs as a substitute for completing authorized work.
- Report residual risks and unverified assumptions in the final handoff.

## Release Notes / 发布说明

When releases are explicitly authorized, write release notes in the project's
required language and group entries where useful:

```text
## Features
## Improvements
## Bug Fixes
## Tests
```

Include relevant commit identifiers and note migrations, compatibility changes,
known limitations, and rollback requirements. Publishing tags or releases always
requires explicit authorization.

## Project-Specific Extensions / 项目扩展

Add only rules that are truly specific to this project, such as domain safety,
repository-specific commands, required trigger phrases, network restrictions,
data handling, or release sequencing.

仅在此处补充项目特有规则，例如领域安全边界、仓库专用命令、固定触发词、网络策略、
数据处理要求或发布顺序。不要重复上文已经定义的通用规则。

### Verification Routing / 验证路由

Use the L0-L4 matrix in **Testing and Verification**. The commands listed there
are the project catalog; only the commands selected by the current risk level are
required. `release-verify` is L4 and is not a default development, review,
re-review, commit, or push check.

### Domain Constraints / 领域约束

- Store AgentDeck provider definitions, credential metadata, and only
  authenticated credential ciphertext in `~/.agentdeck/agentdeck.sqlite3`.
  Never persist plaintext credential values.
- Derive the credential encryption key from a private random seed in
  `~/.agentdeck/credential.key` plus the stable machine identity. Never include
  the key file in portable backups or silently regenerate it when ciphertext
  already exists.
- Keep `~/.agentdeck/` directories mode `0700` and databases, key files,
  sidecars, backups, locks, and temporary files mode `0600`.
- Preserve Codex and Claude session and authentication files; source logs are
  read-only and provider switching modifies only documented configuration
  fields.
- Keep usage metadata in the core database and approved visible session text in
  the separately purgeable `sessions.sqlite3` database.
- Do not delete, overwrite, or reinstall existing legacy scripts under the
  real user's `~/.local/bin/`.

### Prohibited Actions / 禁止事项

- Do not commit provider credentials, generated local configuration, or backups.
- Do not alter unrelated Codex or Claude settings while switching providers.

### Workflow Triggers / 工作流触发词

| Trigger        | Required workflow                       | Commit/push authority            |
| -------------- | --------------------------------------- | -------------------------------- |
| Not applicable | No project-specific trigger is defined. | Explicit authorization required. |
| Not applicable | No project-specific trigger is defined. | Explicit authorization required. |

Never infer commit or push authorization from a workflow trigger unless this
table explicitly grants it.
