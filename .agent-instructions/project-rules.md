# AgentDeck Project Rules

This file contains detailed project rules routed from `AGENTS.md`. Read only
the sections selected by the current task's routing row; the core scope,
authorization, and workflow-authority rules remain in `AGENTS.md`.

## Standard Work Stages / 标准工作阶段

Use the shared `development-workflow` Skill for phase matching, execution,
completion receipts, checkpoints, and next instructions. `AGENTS.md` owns scope
and authorization; [Documentation Workflow](../docs/documentation-workflow.md)
owns project artifact dependencies. Do not maintain a second phase sequence here.

Review and re-review keep product code, tests, configuration, and runtime behavior
read-only unless repairs are explicitly authorized. Required review/status
artifacts remain in scope under the active stage. An implemented repair is not
an independent review result, and an incremental commit does not complete a
broader unfinished task, topic, or audit.

Apply the verification rules below to the exact content state. A phase change
alone does not invalidate prior evidence.

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

Select checks by affected behavior, subsystem, and risk. The command catalog is
not an unconditional checklist. For Go work in the managed sandbox, use
`GOCACHE=/private/tmp/agent-deck-go-build`; when downloads are needed, use
`GOMODCACHE=/private/tmp/agent-deck-go-mod` rather than the user's module cache.

| Purpose | Command |
| --- | --- |
| Whitespace | `make check-whitespace` |
| Scoped Go behavior | `scripts/run-go-test.sh <packages and -run selection>` |
| Go core regression suite | `scripts/run-go-test.sh ./...` |
| Relevant Go race checks | `scripts/run-go-test.sh -race <affected packages>` |
| Go vet | `make vet` |
| Go runner contract | `make check-go-test-runner` |
| Both macOS CLI architectures | `make build-all` |
| arm64 artifact size | `make check-arm64-size` |
| Hook behavior | The affected Python/shell Hook tests; use isolated state and fixtures |
| Release readiness | `make release-verify` |

`make build-all` uses the configured output directory and build metadata rather
than leaving a bare `go build` binary in the repository root. `make verify`
includes whitespace, runner, full Go tests, race tests, and vet; select it only
when that complete set is required. `make release-verify` adds the release build
and distribution checks and remains the L4 aggregate.

`make check-whitespace` checks tracked and untracked non-binary content, excluding
vendor content, for trailing whitespace, CRLF, and missing final newlines. It
checks content, not only changed lines. Record unrelated violations separately
without repairing or staging them as part of the current task.

The Go test runner invokes tests once with `-mod=vendor -count=1 -v`, preserves
the real exit status and full combined log, and prints a failure summary and tail.
It defaults GOCACHE to the path above. Use `AGENTDECK_GO_TEST_LOG` for a known,
task-specific log path when a long run may need inspection while still running.
Do not rerun merely to recover output that is already in that log.

| Level | Typical change | Required evidence |
| --- | --- | --- |
| L0 | Documentation, comments, ignore rules | Relevant format/link/discovery checks, `make check-whitespace`, and `git diff --check` |
| L1 | Localized package, renderer, or helper behavior | Affected targeted tests |
| L2 | Shared parser, schema, persisted state, or wire/text contract | Targeted tests plus the affected subsystem's regression suite; Go core contract changes require `scripts/run-go-test.sh ./...` |
| L3 | Concurrency, credentials/privacy, migration execution, build or installer behavior | L2 plus the relevant race, vet, cross-build, size, install, or privacy checks |
| L4 | Release artifacts or explicit full release validation | `make release-verify` as the aggregate gate |

- Treat paths as routing hints, not proof of impact. A shared Hook/Skill contract
  needs its own cross-runtime checks; an unchanged Go product does not gain a
  full-suite obligation solely because the modified helper is also a parser.
- Run fast targeted checks during implementation. Run the selected broader set
  once after the final relevant change; do not run aggregate gates and all their
  components again without a new failure or unresolved question.
- Reuse evidence when its command/result and exact content identity are available
  and the relevant tests, dependencies, configuration, toolchain, generated files,
  and environment remain valid. Refresh only the uncertain or changed premise;
  do not poll unchanged files or repeat a suite because a phase changed.
- Existing review or delivery checks may establish current identity. Do not add
  another equivalent status/diff/hash check merely to restate the same evidence.
- Review may add independent focused evidence while reusing an unchanged broader
  result. Commit/push checks concern the staged or committed tree, message,
  signature, and Hook effects; they do not rerun unchanged product verification.
- Verify the behavior at the layer claimed. Builds do not prove runtime behavior;
  browser behavior needs rendered checks, database claims need database evidence,
  and runtime integration needs a real client or an explicitly bounded equivalent.
- Label simulated events, isolated fixtures, and real-session acceptance distinctly.
  Report unavailable checks and residual uncertainty; do not convert an unrun
  check or an unrelated health result into proof of completion.
- Remove only temporary materials created by the current task when no longer
  needed. Retain referenced evidence through handoff and preserve pre-existing
  user files, caches, and unrelated diagnostics.

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

For Go test failures and timeouts:

- Use `scripts/run-go-test.sh` on the first potentially slow or diagnostic run
  so the complete combined output and real exit status survive output filtering.
- For a long run, set `AGENTDECK_GO_TEST_LOG` to a known task-specific path
  before starting it; inspect that file while waiting instead of starting a
  second test process.
- Inspect the saved log before running any test again. Do not rerun merely to
  add `-v`, search for `FAIL`, view the tail, or recover output hidden by a tool.
- Before a rerun, state the unresolved question and the material change in
  package scope, `-run` selection, timeout, environment, instrumentation, or
  content that will answer it. If neither changed, reuse the saved evidence.
- Treat a timeout as a request to inspect the goroutine dump, blocked source
  line, channel or PTY wait, expected output marker, and environment. Do not
  increase sleeps or timeouts unless evidence shows forward progress is merely
  slower than the current bound.
- Stop after two failed attempts in the same diagnostic approach family and
  reassess the hypothesis before issuing another Go test command.

## Commit and Push Rules / 提交与推送规则

Use only the exact Git action authorized by the user or an explicitly confirmed
workflow delivery ceiling. Commit, push, tag, release, and history rewriting
remain separate authorization boundaries under `AGENTS.md`.

Every Codex-assisted commit requires an English Conventional Commit subject,
a non-empty body explaining what changed and why, and this exact trailer:

```text
Co-Authored-By: Codex <noreply@openai.com>
```

Use additional established identities only for material contributors. For staged
work backed by Beads tasks, perform the contributor check in [Beads](beads.md),
including its included/excluded/trailer/unresolved output before the commit.
Do not infer authorship from the current assignee, model name, or a review-only
claim. Stop for unresolved material attribution rather than inventing an identity.

Inspect the full proposed message before committing. Afterward inspect the actual
commit object, confirm its subject, body, trailer, changed paths and content, and
verify its SSH signature. Never amend or rewrite a commit to fix attribution
without explicit authorization.

### Task-Level Commit Boundary / Task 级提交边界

- Default to one reviewed, completed task per logical commit, including its code,
  tests, necessary documentation, review record, and status changes. Do not split
  one task mechanically by development/review/repair phases.
- Topic status changes in that boundary are its `tasks.md` and review records.
  Follow Documentation Workflow's worktree status ownership; do not add global
  `docs/status.md` progress changes or a companion `main` commit to a Task checkpoint.
- Split independently deliverable scopes or separately authorized boundaries.
  Do not combine different task anchors merely because they share a topic, file,
  or working tree. Classify shared files by hunk and preserve unrelated changes.
- Present the Skill's Task checkpoint with the proposed scope, exclusions,
  evidence, and delivery recommendations. Ask for commit authorization only when
  it has not already been granted for that exact action. Do not turn an advisory
  checkpoint into another approval loop or imply that it authorizes delivery.
- A commit request responding to a checkpoint selects that named scope. Otherwise
  bind it to the current user-authorized work; do not interpret it as permission
  to commit every dirty file. If multiple candidates remain, prepare their
  concrete scope split and ask which one is intended before committing.
- Later fixes or status records may form a new authorized commit when the original
  implementation was already committed. Do not rewrite history to recreate a
  single task commit. Record the broader task/audit as still in progress when an
  incremental commit leaves required work outstanding.
- Before committing, verify staged paths, hunks, content identity, the full message,
  and applicable review/verification evidence. A complete staged diff and scoped
  content comparisons can establish these facts; use additional stat/name views
  only when they answer a separate scope question.
- After committing, compare the actual commit against the intended scope and
  inspect status and Hook effects once. Re-verify product behavior only if a Hook
  or another writer changed relevant content.
- Before pushing, verify branch, remote, commit range, signed commit objects, and
  required checks. Commit permission does not grant push permission.
- Never use destructive reset/checkout operations to discard user work. Force
  push, rebase, branch deletion, and other history rewrites need their own
  explicit authorization.

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
env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go mod tidy
env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go mod vendor
```

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

## Runtime and Deployment Verification / 运行与部署验证

- Runtime topology / 运行拓扑: One on-demand `agentdeck` binary uses
  `~/.agentdeck/agentdeck.sqlite3`, a machine-bound private
  `~/.agentdeck/credential.key`, `~/.codex/config.toml`, and
  `~/.claude/settings.json`. The optional watcher is foreground-only.
- Allowed connectivity / 允许的连接方式: Normal AgentDeck product commands and
  automated product tests require no network. Only an explicit `agentdeck usage price update` downloads the
  configured public price catalog, and the desktop app may check for a newer
  stable release when that check is explicitly enabled; it defaults off, sends
  no local state, and only opens the official release page. Provider hosts are
  consumed by Codex or Claude, not probed by AgentDeck.
- Prohibited exposure / 禁止的暴露方式: Normal product operations must not add
  listening ports or alter host network configuration. Explicitly authorized
  local development/test servers must stay scoped and loopback-bound; they do
  not authorize new product listeners or public exposure.
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

### Document language / 文档语言

- Write reusable agent instruction files in English, with one authoritative statement
  per rule. Do not add translated summaries that duplicate those instructions.
- Preserve existing bilingual headings unless their links and anchors are
  updated together.
- Preserve protocol commands, markers, field names, paths, and quoted evidence
  in their original form.
- Keep requirements, designs, reviews, and status documents in their established
  primary language. Do not translate them as part of unrelated changes.
- Maintain user-facing documentation and product copy in the languages required
  by the product.
- Communicate proposals, approval requests, and results in the user's language.
- Review reports use the active Skill's presentation contract. Preserve stable
  machine values and IDs; localizing prose must not silently change the record
  fields consumed by Hooks. Status summaries follow their own project format.

### Documentation authorities and maintenance / 文档权威与维护

Use [Documentation](../docs/README.md) to discover current authorities.
[Documentation Workflow](../docs/documentation-workflow.md) owns names, document
sets, readiness, lifecycle, and retirement; [Review Records](review-records.md)
owns record metadata and locations. Keep the table below as the state-routing
contract, not a copy of document templates or phase instructions.

- Update the closest living authority when approved behavior or requirements
  change. Preserve historical records; do not rewrite them to current conventions.
- Prefer existing authorities over new narrative logs. Create a distinct record
  only when the approved investigation, incident, or plan needs one.
- Follow the subject's retirement rules, including the distinct topic and Lane A
  archive/index conventions. Do not require an archive-index entry for a fix when
  its lifecycle explicitly omits it.
- Update navigation only when topology changes. Update status only when its
  subject changes; do not touch timestamps merely to appear synchronized.
  Select the owning document through
  [Status ownership across worktrees](../docs/documentation-workflow.md#status-ownership-across-worktrees),
  rather than copying a topic transition into the global project status.
- Keep unfinished work active. A successful commit or local check does not close
  a larger project objective whose requirements remain unfinished.
- Review documentation at major delivery milestones, limited to the affected
  authorities and their pointers.

## Handoff and Project State / 交接与项目状态

A handoff pointer directs readers to current authority; it is not a second
narrative store. This project currently uses its document indexes and topic
records rather than a dedicated handoff file. Discover current targets instead
of assuming that a particular filename must exist.

When resuming work:

1. Follow the documentation index and lifecycle to the relevant status, topic,
   requirements, contracts, and any handoff pointers. Reuse complete, still-valid
   instructions and evidence already loaded in this session.
2. Check relevant repository drift when it can affect the work. Inspect history
   for a specific provenance or change question rather than rereading it on every
   turn. A new phase alone is not evidence of a content change.
3. If coordination is required, resolve the matching task, blockers, claim, and
   handoff comments through [Beads](beads.md). Do not use dispatch state as a
   substitute for product or review evidence.
4. Refresh drift-prone environment facts when the task depends on them. Treat
   chat and memory as discovery aids and verify material claims at their source.

Use `handoff-sync` for an explicit synchronization request or its documented
mandatory Hook trigger. It discovers existing authorities and pointers; an
absent automatic Hook configuration does not disable explicit Skill use.
Do not invent a handoff file, parallel status hierarchy, or generated-section
edit outside that workflow.

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

### Verification Routing / 验证路由

Use the L0-L4 matrix in **Testing and Verification**. The commands listed there
are the project catalog; only the commands selected by the current risk level are
required. `release-verify` is L4 and is not a default development, review,
re-review, commit, or push check.

### Release Decision and Preflight / 发布决策与技术预检

- When the version contract task reaches Review PASS, resolve its required
  evidence gates and applicable delivery checkpoint before declaring version
  development complete. Do not create release-candidate or release tasks under
  that plan, or infer an RC/stable-release decision from review or evidence status.
- At that terminal checkpoint, offer to commit the contract task. After an
  authorized commit, a push to any remote branch may make the commit eligible
  for technical preflight; pushing does not start preflight automatically.
- After an authorized push, ask whether to dispatch the manual
  `release-preflight` workflow for the exact pushed commit SHA. Dispatch remains
  a separately authorized external action.
- The preflight runs L4, requires an existing isolated-real-state evidence ID,
  builds and verifies candidate artifacts, and publishes only a commit-bound
  evidence artifact. It does not tag, release, publish, install, or choose a
  release channel.
- A successful same-SHA preflight lets the user choose RC, stable release, or no
  publication. Tag and publication workflows must reject a tag whose peeled
  commit lacks successful preflight evidence for that exact SHA.
- RC and stable publication reuse the same-SHA L4 and isolated-real-state
  evidence. They run only version-specific artifact identity, checksum,
  installation, and distribution checks made necessary by embedded version and
  build metadata; they must not repeat the technical preflight merely because
  the workflow stage changed.

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

### Completion Evidence and Project Memory / 验收证据与项目记忆

Read [Evidence](evidence.md) when crossing a new Document, Task, Topic, or Release
completion boundary, handling integration evidence, creating/invalidating
relevant evidence, or materially depending on durable project knowledge.
Use its WorkUnit identity, exact content binding, provider discovery, and failure
rules; do not invent a separate Plan boundary or query gates merely because a
workflow phase changed.

CEv1 evidence, review verdicts, and Beads coordination remain distinct. Durable
project memory is non-authoritative and separate from evidence; repository facts
remain authoritative. Record only within the active scope and authorization.

### Branch Model and Integration / 分支模型与集成

Read [Branching](branching.md) for a branch, merge, or version-assembly operation.
Completing a feature does not itself authorize a merge; version-contract work
owns assembly. Active version membership is recorded in that contract topic and
projected in status, while later planning belongs in the roadmap.

Classify the merge before selecting review/evidence work. Fast-forwarding to an
already verified identical tree can reuse its evidence; a new combined tree or
hand-written resolution follows the integration obligations defined in Branching.
Do not describe every merge as a previously unverified content state.

### Review Artifact Finalization / 评审产物收口

A real review/re-review stage authorizes its mandatory in-scope artifact and
coordination work under `AGENTS.md`. Explicit user limits, such as a read-only
advisory audit with no formal phase transition, still control the task.

- Use the current Skill's full report format, finding policy, checkpoints,
  post-phase-blocker behavior, and token-bound receipt.
- Map each review record to its subject and local history. Append the round to
  the location defined by Review Records; Lane A fixes use their existing file.
- For `tasks.md` review, run the document-set check required by Documentation
  Workflow. Interpret its scope and limitations there; an unrelated topic's gap
  does not authorize claiming or repairing that topic.
- Keep Review unchecked while a finding against the target remains open,
  regardless of severity. Record an explicit user decision when it closes a
  finding. PASS and a required non-VERIFIED evidence gate remain distinct.
- Perform the applicable Beads transition and durable comment under its own
  contract, only for the subject being handled. A status change does not by
  itself prove authorship, independent review, or evidence completion.
- Update the owning topic's matrix readiness and handoff only when they change;
  a topic review does not update global project status in either checkout.
  A repair may affect readiness or handoff state but cannot self-issue a review
  PASS. Do not force timestamp-only updates or copy review findings into status.
- Confirm that the record, content identity, gate result, appropriate matrix
  cell, and next instruction describe the same subject and stage. They need not
  have identical status words because they own different kinds of state.

Review does not authorize product repair or delivery. Perform only the record,
evidence, and coordination mutations already authorized by the active request.

### Where a Review Round Is Written Down / 评审轮次写在哪里

Each round has one full review record. Keep state summaries in their own
formats, following the shared Skill's status-summary contract.

| File | Carries | Never carries |
| --- | --- | --- |
| Topic review record or Lane A fix record | Full round: subject, content identity, method, findings, dispositions, evidence, verdict | Another subject's history without an explicit relationship |
| `docs/topics/<topic>/tasks.md` | Document Draft/Review and task Dev/Review cells, plus concise current state and report pointers | Finding IDs, finding descriptions or dispositions, scores, repair instructions, or round narratives |
| `docs/status.md` | Integrated project/release state, version-assembly projection, and pointers | Unmerged topic task progress, review details, scores, repair instructions, or matrix copies |
| `docs/roadmap.md` | Later version direction, unscheduled candidates, and withdrawals | Active-version membership authority, execution details, or review-round content |

Write the full report once. Update related documents only with information they
own and only when their subject changes. A PASS updates the applicable Review
cell, not a second account of the round. A repair updates status only if its
actual readiness or handoff changed; it does not manufacture a new verdict.

Historical rationale for these governance rules is preserved in commit
`36e4b0e87f0ac0eda603e022afd52db26517a043`, file
`.agent-instructions/project-rules.md`. The snapshot is provenance, not current
runtime or completion evidence; do not duplicate its incident narratives here.
