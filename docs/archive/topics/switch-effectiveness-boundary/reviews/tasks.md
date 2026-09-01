---
status: historical
topic: switch-effectiveness-boundary
subject: tasks.md
retired: 2026-09-01
---

# Review log — switch-effectiveness-boundary / tasks.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `8a13af7155f5a5303b76bfd70ac21cb918ecedb7`;
  `tasks.md` blob `322472a9f7b8de6e95383171640c4f4b967411a3`.
- Reviewer: Codex.
- Method: Formal task-decomposition review against the approved requirements
  and architecture. Reused the current document-set checker result and
  inspected direct affected-test ownership for the config-change reconcile.
- Scope: task anchors, dependencies, file/test ownership, verification levels,
  real-session acceptance procedure, and Documents/Tasks status matrices.
- Findings:
  - [P1] T-F1 — `real-session-acceptance` step 5 still requires an unknown
    `ConfigChange` route, contradicting the approved no-route contract -> open;
    require absence of the new route and continued resolution to the prior
    custom provider/multiplier before restart.
  - [P1] T-F2 — The Tasks matrix still names task 2
    `undeterminable-route-quality` while the task breakdown renamed it
    `unadopted-switch-no-route`, and the Documents prose still says architecture
    is next after its PASS -> open; synchronize the sole dispatch/status
    authority with the reviewed decomposition and current document order.
  - [P1] T-F3 — The document-set rationale and task 4 retain the superseded
    stored/resulting-quality model even though the approved contract writes no
    route and assigns no new quality -> open; describe suppression of the
    misleading route and leave only redesign of the retained evidence sources'
    quality to the v0.6.0 topic.
  - [P1] T-F4 — Task 2 omits
    `cmd/agentdeck/claude_reload_test.go`, the existing direct test owner for
    `reconcileClaudeConfigChange` and its current matched/unknown behavior ->
    open; include and update those tests in the task's Files and coverage.
  - [P1] T-F5 — The central real-session procedure says to confirm the serving
    credential "from the client's own behavior" without naming a reproducible
    signal or the commands/queries that prove route and attribution -> open;
    define the credential/provider discriminator and the exact redacted route,
    event-attribution, and cost checks to record.
- Evidence: `tasks.md:77-79`, `:99`, `:121`, `:128`, and `:148` contradict the
  approved no-route architecture or the document's own task anchors/status.
  `cmd/agentdeck/claude_reload_test.go:15-215` directly exercises
  `reconcileClaudeConfigChange`, including the unknown-route expectation task 2
  must change, but is absent from `tasks.md:56-57`. The previously run
  `scripts/check-topic-docs.sh` passed for this unchanged candidate; it audits
  document-set presence and cannot falsify these semantic defects. Broad
  verification stopped after the decisive blockers.
- Verdict: REOPEN

### 📋 Tasks Decomposition Review

📊 总体评分：4/10

✅ 评审结论：FAIL

#### 🔴 严重问题——必须修复

[`tasks.md:77`](../tasks.md) T-F1：真实会话验收仍要求 unknown route，与已通过的
no-route 合同直接矛盾。
- 行为风险：正确实现会被手工验收判失败，或实现者为让验收通过而
  重新写入被 requirements/architecture 明确否定的 unknown route。
- 证据：Contract 3 要求 credential-deleting switch 不写任何
  `ConfigChange` route，并让 prior route 继续决定 multiplier。
💡 有界修复：将步骤 5 改为确认没有新 `ConfigChange` row，事件仍归属
前序 custom provider/multiplier；重启后再由 `SessionStart` 切到 `official`。

[`tasks.md:148`](../tasks.md) T-F2：Tasks 矩阵仍使用旧 anchor
`undeterminable-route-quality`，与正文的 `unadopted-switch-no-route` 不同；
文档状态又仍说 architecture 是下一份。
- 行为风险：Beads dispatch、开发命令和评审记录会指向不同 Task，而且唯一
  状态权威对当前 document gate 给出错误顺序。
- 证据：`tasks.md:36` 和 `:148` 的 anchor 不同；`:138-141` 已记录
  architecture PASS，`:128` 却仍说它是 next。
💡 有界修复：将矩阵 anchor 与正文统一，并把文档顺序更新为
`tasks.md` 待评审。

[`tasks.md:99`](../tasks.md) T-F3：task 4 与 Documents 理由仍使用旧的
stored/resulting-quality 模型。
- 行为风险：开发可能把一个不存在的新 route quality 纳入 v0.6.0 修改，
  扩大范围或恢复 unknown write。
- 证据：architecture 明确 no new route/no new quality；`tasks.md:121` 仍声称主题
  改变“quality of a stored route”。
💡 有界修复：把主题行为改为“suppress one misleading route”，task 4 只将已存
evidence source 的未来 quality redesign 留给 v0.6.0。

[`tasks.md:56`](../tasks.md) T-F4：task 2 未列出直接覆盖 reconcile 的
`cmd/agentdeck/claude_reload_test.go`。
- 行为风险：任务可以新增邻近测试却遗留旧的 matched/unknown 回归期望，
  或直到广泛套件才发现直接测试失败。
- 证据：`cmd/agentdeck/claude_reload_test.go:15-215` 调用
  `reconcileClaudeConfigChange`，并断言当前 unknown-route 行为。
💡 有界修复：将该文件加入 task 2 Files，明确更新 direct reconcile
回归覆盖。

[`tasks.md:72`](../tasks.md) T-F5：中央真实会话验收没有定义可重复的 credential/provider
判定信号与检查命令。
- 行为风险：“from the client's own behavior”无法让独立审核者判定同一轮运行
  到底是真正验证了凭据方向，还是仅看到请求成功。
- 证据：步骤 3 未命名任何 provider-side/client-side discriminator，步骤 5 也未命名
  检查 route/event attribution/cost 的命令或查询。
💡 有界修复：定义不泄漏 secret 的真实 provider 判别信号，并列出记录 session
ID、route row、event attribution 和 cost multiplier 的精确命令/查询与时间顺序。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- 四个 Task 的主体分层正确：advisory/file-boundary、route/cost、real-session
  acceptance、v0.6.0 premise correction 各自具有独立交付边界。
- task 2 明确要求验证 prior route 的读取结果，而不是只断言没有新 row。
- 真实凭据不进入仓库、审核记录不记录 credential value 的安全边界正确。

#### 📝 总结

评审绑定 HEAD `8a13af7155f5a5303b76bfd70ac21cb918ecedb7` 与 `tasks.md`
blob `322472a9f7b8de6e95383171640c4f4b967411a3`。四任务主体边界可用，但状态
authority、真实会话验收和直接测试所有权仍保留旧的 unknown-route 模型或未决定证据。
所有五项在 PASS 前都必须关闭。

#### 📌 下一步

```text
修复：switch-effectiveness-boundary / reviews/tasks.md / T-F1 T-F2 T-F3 T-F4 T-F5
```

## Round 2 — 2026-08-17

- Reviewed state: HEAD `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8`;
  `tasks.md` blob `719ff3988ebe14699fc39a8f891f5b4ee253866b`.
- Reviewer: Codex, independently evaluating the repaired task matrix.
- Method: Finding-by-finding task-decomposition Re-review. Checked the five
  repaired scopes against the approved requirements/architecture, current
  direct reconcile tests, SQLite schema, and actual `usage scan`, `session scan`,
  and JSON `session show --tokens --all` command contracts.
- Scope: T-F1–T-F5 and new blocking regressions caused by their repair.
- Finding dispositions:
  - **T-F1 — CLOSED.** The real-session procedure now verifies that the
    credential-deleting switch adds no `ConfigChange` route, the route count
    remains unchanged, and subsequent events still resolve and price through
    the prior custom provider until restart.
  - **T-F2 — CLOSED.** The task-2 breakdown and Tasks matrix both use
    `unadopted-switch-no-route`; the Documents prose now records architecture
    PASS and names `tasks.md` as the final document awaiting review.
  - **T-F3 — CLOSED.** Task 4 now says a credential-deleting switch creates no
    resulting route and leaves any future redesign to the retained prior-route
    or session-start evidence sources. The document-set rationale says this
    topic suppresses one misleading stored route rather than changing its
    quality.
  - **T-F4 — CLOSED.** Task 2 explicitly includes and describes the required
    changes to `cmd/agentdeck/claude_reload_test.go`, including removal of the
    existing unknown/multiplier-1 expectation.
  - **T-F5 — CLOSED.** The acceptance setup requires a provider-side non-secret
    credential label/key ID plus request ID, rejects insufficient signals, fixes
    UTC bounds, and gives exact scan, route-count, route-resolution, and session
    cost commands. Current CLI/schema inspection confirms those commands and
    fields exist; invocation JSON exposes catalog and provider cost so the
    distinct multiplier can be checked without exposing a credential.
- Evidence: `scripts/check-topic-docs.sh`, `make check-whitespace`, and
  `git diff --check` pass against the repaired candidate. Current source confirms
  `usage_events.event_at/event_key`, all queried route columns,
  `session show --client --tokens --all`, and `.data.usage`/
  `.data.invocations` response fields. Completion Evidence Profile v1 gate
  `switch-effectiveness-boundary:tasks.md` is `VERIFIED` for the reviewed
  HEAD-plus-blob state, with evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:tasks.md:rereview-round-2:719ff3988ebe14699fc39a8f891f5b4ee253866b`.
- Verdict: PASS

### 📋 Tasks Independent Re-review

📊 总体评分：9/10

✅ 复评结论：PASS

#### 🔴 严重问题——必须修复

无。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- T-F1/T-F3 已关闭：no-route 合同、真实会话验收和 v0.6.0 前提修正
  使用同一个 retained-evidence 模型。
- T-F2 已关闭：Task anchor、Documents 状态与开发顺序一致。
- T-F4 已关闭：直接 reconcile 回归测试已纳入 task 2 文件与覆盖边界。
- T-F5 已关闭：真实验收不再以“请求成功”代替凭据判定，并将
  provider audit、route row、event attribution 和 cost 证据绑定到同一 UTC 时序。

#### 📝 总结

T-F1–T-F5 均已在 blob `719ff3988ebe14699fc39a8f891f5b4ee253866b`
中关闭，没有新的 tasks.md finding。需求、架构和任务矩阵三份文档
现在均已 Review PASS，但四个实现 Task 仍未开始，因此本轮只关闭
`tasks.md` 文档 Task，不声称 topic 完成。

#### Task checkpoint

- Task：`switch-effectiveness-boundary / tasks.md` @ HEAD
  `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8` + blob
  `719ff3988ebe14699fc39a8f891f5b4ee253866b`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅提交 `tasks.md` 与本评审记录；状态文档只在能安全隔离
  该 Task hunk 时纳入，排除 `docs/README.md` 中的其他 dirty hunk 和所有无关更改。
- 推送建议：获得明确 commit/push 授权，并检查目标分支、远端和提交范围后
  再推送；本 checkpoint 不执行也不授权交付。

## Round 3 — 2026-08-25

- Reviewed state: user-authorized decomposition revision. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `tasks.md` blob
  `646de4edc9b541dc7fff5c5b8dc540321d382efc`.
- Author: Codex — this is a design reopen record, not an independent review.
- Change: the former three-task Claude-led plan is replaced by four tasks:
  shared `hook-delivery-ledger`, Claude `switch-effectiveness-contract`, shared
  `effective-route-policy`, and cross-client `real-session-acceptance`. Normal
  lifecycle transport — never manual payload injection — is required for both
  clients.
- Completion gate: NOT_VERIFIED — historical Round 2 evidence is bound to a
  superseded decomposition.
- Verdict: REOPEN — awaiting independent review of the current four-task matrix.

## Round 4 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  final synchronized `tasks.md` blob
  `3cf880b8df1ab0cbec6500c164400c368a312974`. Upstream consistency was checked
  against requirements blob `3605f402d413811290e5f56dee4361c035321823`
  and architecture blob `ced65fe13f6f9d0b07f7b3a9f572943d374ed8d3`.
- Reviewer: Codex, independently reviewing the four-task decomposition; this
  workflow turn changed only the required Review/status projection after the
  verdict, not the reviewed task scopes.
- Method: Formal decomposition Review under `development-workflow`; CodeGraph
  identified current call paths and affected tests, then the task files,
  dependencies, verification levels, security-boundary test, and transaction
  ownership were checked against source and the two upstream documents. The
  project topic checker and L0 checks were run on the final artifact state.
- Scope: Documents matrix, all four task anchors, file ownership, dependencies,
  verification levels, real-session lifecycle acceptance, and development
  readiness.
- Findings:
  - **[P1] T4-F1 — Task 1 starts persistence too early and omits the regression
    that proves the admission boundary.** `tasks.md:18-20` calls shared
    `RecordHookDelivery` immediately after `ParseEvent` accepts, but current code
    still rejects invalid transcripts and unmanaged ConfigChange paths after that
    point. The file list omits `cmd/agentdeck/hook_boundary_test.go`, whose current
    cases pin those rejections. Repair Task 1 to normalize/persist only after the
    full approved admission sequence and include that test file plus cases that
    assert rejected input writes neither stream.
  - **[P1] T4-F2 — Tasks 1 and 3 do not form independently coherent commit
    boundaries.** Task 1 creates shared `RecordHookDelivery`, makes observation
    and optional route writes atomic, and covers every accepted event
    (`tasks.md:13-44`); Task 3 later puts every route decision inside that same
    operation (`:72-118`). The matrix does not say what route policy Task 1 ships
    or tests before Task 3, so Task 1 either contains part of Task 3, temporarily
    regresses ConfigChange routing, or cannot meet its own atomic acceptance.
    Repair by merging the coupled slices or narrowing Task 1 to a complete
    independently reviewable storage boundary with an explicit non-regressing
    intermediate policy and moving shared-operation completion to Task 3.
  - **[P1] T4-F3 — verification does not cover the whole retry operation or the
    concrete migration contract.** Task 1 requires a delivery-ID retry and an
    atomic optional route, but its assertions only promise one observation; they
    do not prove that an already committed retry cannot append a second route.
    The architecture also requires fresh and previous-version migration cases,
    while the task does not name the exact schema/encoding assertions needed to
    falsify the incomplete column contract. Repair by asserting both streams on
    retry, adding the concrete v18-to-new-version and fresh-schema checks, and
    naming their test ownership.
  - **[P2] T4-F4 — real-session acceptance assumes every non-compact start
    appends/advances a route without deciding the existing no-op behavior.**
    `tasks.md:146-152` conflicts with the consecutive-identical suppression in
    `internal/usage/routes.go:62-78`. Align the procedure with the architecture's
    repaired definition of `advance` and assert the chosen row-count behavior.
- Evidence: source and CodeGraph evidence listed in the matching requirements and
  architecture rounds; current `cmd/agentdeck/hook_boundary_test.go:14-101` pins
  post-parse rejection; task overlap was checked directly at `tasks.md:13-118`;
  final-state checks: `bash scripts/check-topic-docs.sh` -> exit 0,
  `make check-whitespace` -> exit 0, and `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — T4-F1 through T4-F4 and the open upstream
  Document findings leave this decomposition WorkUnit unsatisfied; no CEv1
  completion evidence was recorded for this blob.
- Verdict: REOPEN

## 📋 Tasks decomposition 独立评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`tasks.md:18`] T4-F1：Task 1 把 `ParseEvent` 成功直接当作 persistence admission，
并漏掉现有 Hook boundary 回归文件。
- 行为风险：无效 transcript 或非 managed ConfigChange 可能开始写 observation。
- 证据：`cmd/agentdeck/main.go:2904-2913` 的校验发生在 parse 之后；
  `cmd/agentdeck/hook_boundary_test.go:14-101` 固定了拒绝行为。
💡 有界修复：完整 admission 后再 normalize/persist，并把该测试文件及双流零写入断言纳入
Task 1。

[`tasks.md:13`] T4-F2：Task 1 与 Task 3 同时拥有 `RecordHookDelivery`、route policy、
transaction 和相同核心文件，缺少可独立 Review/commit 的中间行为。
- 行为风险：Task 1 要么偷带 Task 3，要么临时改变 ConfigChange route，要么无法满足
自己的 atomic acceptance。
- 证据：Task 1 的 `:13-44` 与 Task 3 的 `:72-118` 共享同一操作和文件边界。
💡 有界修复：合并耦合切片，或把 Task 1 缩成完整且不改变 route 行为的存储 Task，并把
shared operation 的完成点放到 Task 3。

[`tasks.md:27`] T4-F3：retry 只断言 observation，没有断言 optional route 不重复；
migration 也没有精确的 fresh/v18 upgrade schema 断言所有权。
- 行为风险：同一 delivery 可留下一个 observation 与两个 route，或迁移生成实现者自行
猜测的 schema。
- 证据：unique `delivery_id` 只约束 observation；architecture 要求两类迁移场景。
💡 有界修复：同时断言双流 retry 结果，并列出 fresh 与 v18 升级的完整 schema/数据保留
测试及归属文件。

### 🟡 建议改进——推荐

[`tasks.md:146`] T4-F4：实机会话步骤把每个 non-compact start 写成 route advance，
但现有连续相同 route 会 no-op。
- 证据：`internal/usage/routes.go:62-78`。
💡 有界改进：待 architecture 决定 `advance` 后同步 row-count 验收。

### 🟢 优点

- 四个 anchor 都列出文件、依赖与 L2/L3 级别，真实生命周期明确禁止手工喂 Hook。
- provider-side 非秘密 discriminator、UTC 区间和只读 SQLite/CLI 证据流程保持完整。

### 📝 总结

最终 tasks blob `3cf880b8df1ab0cbec6500c164400c368a312974` 覆盖了
client-neutral 目标，但 admission、安全回归、Task 1/3 原子边界、retry 双流断言和
route-advance 语义尚未闭合；四个实现 Task 不能开始。

## Round 5 — 2026-08-26

- Reviewed state: repair of Round 4's four findings. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `tasks.md` blob
  `a6d6a6f46954f8b7fc72b00f4c76401f203789d6`. Upstream repairs land in
  `requirements.md` blob `64cbe359ff36fd249b96593c85fb70cf542854f6` and
  `architecture.md` blob `b620adf14e53711334cc6dd038424a2947b04109`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a verdict).
- Scope: T4-F1 through T4-F4, as named in the repair command.
- Repair of T4-F1 — Task 1 no longer treats `ParseEvent` success as admission.
  It now calls `RecordHookDelivery` only after the full ordered sequence
  (bounded read, `ParseEvent`, store/home availability, `managedClaudeConfigChange`
  for a Claude `ConfigChange`, `validHookTranscript` for a `SessionStart`), and a
  rejected delivery stays fail-open writing neither stream.
  `cmd/agentdeck/hook_boundary_test.go` is added to the file list, and a named
  *Admission regressions* assertion group extends its existing cases —
  out-of-root transcript, session-mismatched base name, non-regular/symlink
  transcript, `ConfigChange` on `project_settings`/`local_settings`/
  `policy_settings`/`skills`, and `ConfigChange` on an unmanaged path — each
  requiring zero observation rows and zero route rows plus a fail-open exit.
- Repair of T4-F2 — the Task 1/Task 3 overlap is resolved by narrowing Task 1 to
  a storage boundary with an explicit non-regressing intermediate policy rather
  than by merging the slices. Task 1 emits only `advance`, `unknown`, and `none`
  from facts the current handler already has, introduces no prior-state
  classifier, and never emits `retain`; a *Route non-regression* assertion
  requires the route rows written after Task 1 to be identical to the rows the
  pre-task build writes for the same input. Task 3 now states that it owns the
  only route-behavior change in the topic, adds the classifier and `retain`, and
  changes `RecordHookDelivery`'s decision inputs rather than its storage,
  transaction, or admission boundary — so both tasks are independently
  reviewable and independently committable. Task 3 also absorbs the A16-F2
  snapshot and same-transaction rules with a matching assertion and gains
  `internal/provider/config.go` and `internal/provider/config_test.go` in its
  file list.
- Repair of T4-F3 — Task 1's *Whole-operation retry* assertion now requires the
  same-`delivery_id` retry to leave one observation **and** the same route row
  count as the first commit, closing the one-observation/two-routes gap. A
  *Cardinality* assertion states the `0 <= observations(delivery_id) <= 1`
  invariant with the failed-transaction zero-write case. A *Migration*
  assertion names both concrete cases — a fresh database reaching schema version
  19 with the table, the lookup index, the unique `delivery_id` index, and
  exactly the declared columns and types; and a populated version-18 database
  upgrading to that shape with its `usage_session_routes` and `usage_events`
  rows byte-identical — and assigns them to `internal/store/store_test.go`,
  which already owns this repository's migration assertions.
- Repair of T4-F4 (P2) — real-session steps 1 and 2 now follow the architecture's
  repaired definition of `advance`. Step 1 requires recording the route row count
  before and after: a start whose selection differs appends one row, while a
  repeated non-compact start on an unchanged selection appends **no** row and
  still records its own `route_effect=advance` observation — the preserved
  consecutive-identical no-op at `internal/usage/routes.go:56-78`, stated as
  expected rather than as a defect. Step 2 asserts the restart increases the
  route count by exactly one because the selection changed.
- Status prose: the Documents section now records that all ten findings across
  the three records were repaired on 2026-08-26 and that every `Review` cell
  stays unticked pending independent Re-review. No status cell was ticked.
- Verification: `bash scripts/check-topic-docs.sh` -> exit 0. No product code
  changed in this round.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the decomposition WorkUnit stays open until an
  independent Re-review passes this blob, and the upstream requirements and
  architecture repairs must pass their own.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 6 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  final synchronized `tasks.md` blob
  `a81f3b8f8e9bf9bd594e29a361dcfe526b51f973`. Upstream consistency was checked
  against requirements blob `64cbe359ff36fd249b96593c85fb70cf542854f6`
  and architecture blob `b620adf14e53711334cc6dd038424a2947b04109`.
- Reviewer: Codex, independently re-reviewing the Round 5 repair; this workflow
  turn changed only the required Review/status projection after the verdict,
  not the reviewed task scopes.
- Method: Formal decomposition REREVIEW under `development-workflow`;
  finding-by-finding comparison against Round 4, task-boundary and dependency
  analysis, and consistency checking against the two upstream documents.
- Scope: T4-F1 through T4-F4, all four task boundaries, and any newly blocking
  dependency or decomposition regression.
- Findings:
  - **T4-F1 — CLOSED.** Task 1 now admits only after every required check and
    owns the existing Hook-boundary regressions with zero-write assertions.
  - **T4-F2 — CLOSED.** Task 1 is a non-regressing storage boundary; Task 3 owns
    the sole route-policy change and the prior-state classifier.
  - **T4-F3 — CLOSED.** Both-stream retry, exact migration-19 shape, fresh-store,
    and populated-v18 upgrade assertions have explicit ownership.
  - **T4-F4 — CLOSED.** Real-session acceptance treats a consecutive-identical
    advance as a route-row no-op with a new observation.
  - **[P1] T6-F1 — the decomposition depends on a non-executable shared-operation
    ordering.** Task 1 must persist `route_effect` in every observation and Task
    3 must classify prior-route evidence on the same transaction, but the
    architecture they both adopt inserts the `NOT NULL route_effect` observation
    before that classification. Until the contract fixes the order, neither task
    has an independently implementable transaction boundary.
- Evidence: `tasks.md:30-64` makes Task 1 own the observation row and guarded
  atomic write; `tasks.md:127-141` makes Task 3 own the in-transaction classifier;
  architecture Round 18 records the contradictory insert/classify order as
  A18-F1.
- Completion gate: NOT_VERIFIED — T6-F1 and the open upstream architecture
  criterion leave the decomposition WorkUnit unsatisfied; no CEv1 completion
  evidence was recorded for this blob.
- Verdict: REOPEN

## 📋 Tasks decomposition 独立复评

📊 总体评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`tasks.md:30`] T6-F1：Task 1 的 observation 写入与 Task 3 的事务内 classifier
依赖 architecture 中不可执行的“先插入、后分类”顺序。
- 处置：新增阻断 finding。
- 行为风险：两个 Task 无法各自满足可实现、可独立评审和可独立提交的边界。
- 证据：Task 1 要求 observation 携带 `route_effect` 并以整项 guard 写入；Task 3 才
完成 prior-state 分类，而 A18-F1 证明上游顺序在字段产生前就要求插入该行。
💡 有界修复：在 architecture 明确可执行事务顺序后，同步 Task 1/3 的 classifier、
duplicate guard、observation insert 与 optional route write 所有权和先后关系。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- T4-F1–T4-F4 均已关闭；admission 回归、Task 1/3 非重叠边界、双流 retry、迁移覆盖和
实机会话 row-count 语义都已补齐。
- 四个 anchor 仍具备明确文件、依赖、验证级别和正常 Hook lifecycle acceptance。

### 📝 总结

最终 tasks blob `a81f3b8f8e9bf9bd594e29a361dcfe526b51f973` 关闭了四个
既有 finding，但 T6-F1 继承了 architecture 的事务顺序 blocker；在共享操作顺序明确
前，当前四个实现 Task 不能开始。

## Round 7 — 2026-08-26

- Reviewed state: repair of Round 6's blocking finding. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `tasks.md` blob
  `e09f3b990c17e1b0073a12ea0da3451bbec67593`. The upstream architecture repair
  that unblocks it is blob `98bbbeaa662247b87cc774cd42e70da4d28ec6cd`
  (`reviews/architecture.md` Round 19); `requirements.md` is unchanged at blob
  `64cbe359ff36fd249b96593c85fb70cf542854f6`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a verdict).
- Scope: T6-F1 only, as named in the repair command. T4-F1 through T4-F4 stay
  CLOSED per Round 6 and were not reopened by this repair.
- Repair of T6-F1 — the decomposition now adopts the executable order A18-F1's
  repair fixed upstream, and states which task owns which part of it:
  - **Task 1 owns the ordered transaction skeleton.** Its atomic-write bullet is
    replaced by the six numbered steps — per-attempt settings snapshot before
    `BEGIN`; duplicate-delivery guard whose hit is a whole-operation no-op;
    classify on the same transaction; insert the observation carrying the
    computed `route_effect` conditionally on `delivery_id`; the zero-row insert
    taking that same no-op outcome; and the route write only on a one-row
    insert. The bullet states the reason classification precedes the insert
    (`route_effect` is `NOT NULL`), so the task no longer inherits an
    unimplementable order.
  - **Task 1's today's-behavior mapping is scoped to one step.** The
    non-regression bullet now says that mapping is the *body* of the classify
    step, and that the step's position, the duplicate guard around it, and the
    observation/route write order belong to Task 1 and do not move.
  - **Task 3 replaces that body, not the order.** Its classifier bullet now says
    it substitutes the prior-state classifier into Task 1's step 3, produces
    `route_effect`, `prior_state`, `conflict_scan`, and `conflict_sources`
    **before** the observation insert that carries them, and moves no step: the
    guard stays ahead of classification, the insert stays ahead of the optional
    route write, and both zero-row no-op outcomes are unchanged. That is what
    keeps Task 3 a decision change rather than a second transaction design.
  - Both tasks therefore have an implementable, independently reviewable, and
    independently committable transaction boundary: Task 1 can ship and be
    reviewed with its fixed mapping in place, and Task 3 changes exactly one
    step's body.
- No task anchor, dependency, verification level, or file list changed beyond
  the ownership wording above, and no status cell was ticked.
- Verification: `bash scripts/check-topic-docs.sh` -> exit 0,
  `make check-whitespace` -> exit 0, `git diff --check` -> exit 0. No product
  code changed in this round.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the decomposition WorkUnit stays open until an
  independent Re-review passes this blob, and the upstream architecture repair
  must pass its own.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 8 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  final synchronized `tasks.md` blob
  `31e267b9a885b7b07fd2743a06404f70008cda33`. Upstream consistency was checked
  against requirements blob `64cbe359ff36fd249b96593c85fb70cf542854f6`
  and independently re-reviewed architecture blob
  `98bbbeaa662247b87cc774cd42e70da4d28ec6cd`.
- Reviewer: Codex, independently re-reviewing the Round 7 repair; this workflow
  turn changed only the required Review/status projection after the verdict,
  not the reviewed task scopes.
- Method: Formal decomposition REREVIEW under `development-workflow`;
  finding-by-finding comparison against T6-F1, task-boundary analysis, and
  consistency checking against the repaired architecture transaction.
- Scope: T6-F1, Task 1/Task 3 transaction ownership, and any newly blocking
  dependency or decomposition regression.
- Findings:
  - **T6-F1 — CLOSED.** Task 1 owns the fixed transaction skeleton and its
    non-regressing classifier body; Task 3 replaces only that body's policy and
    cannot move the guard, classification, insert, or optional-write steps.
    Both tasks are independently implementable, reviewable, and committable.
  - No new decomposition finding.
- Evidence: `tasks.md:57-75` fixes Task 1's ordered skeleton and both duplicate
  no-op exits; `:160-168` limits Task 3 to the classifier body before the
  observation insert. These boundaries match architecture Round 20, while task
  anchors, dependencies, files, and verification levels remain unchanged.
- Completion gate: VERIFIED — CEv1 gate
  `switch-effectiveness-boundary:tasks.md` verified the exact
  `HEAD + tasks.md` state with subject digest
  `7f7706d5ac82139cf50aafa7a52754b048778cd5205dd09773eccc3c886ea3c4`.
- Verdict: PASS

## 📋 Tasks decomposition 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- T6-F1 已关闭：Task 1 固定事务骨架，Task 3 只替换 classifier body，提交边界不再重叠。
- 四个实现 anchor 的依赖、文件、验证级别和真实 lifecycle acceptance 保持完整。

### 📝 总结

最终 tasks blob `31e267b9a885b7b07fd2743a06404f70008cda33` 关闭了
T6-F1，未发现新的 decomposition blocker；该 topic 现在可进入
`hook-delivery-ledger` 开发。
