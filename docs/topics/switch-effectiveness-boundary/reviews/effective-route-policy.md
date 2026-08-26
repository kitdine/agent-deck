---
status: active
topic: switch-effectiveness-boundary
subject: effective-route-policy
---

# Review log — switch-effectiveness-boundary / effective-route-policy

## Round 1 — 2026-08-26

- Reviewed state: HEAD `8703fedf90571b7f3807a44034af2dd9fe2c8032` plus
  scoped Task 3 implementation diff fingerprint
  `74e3c0a3b5227126cb48e3eb950233d74371d7396ee31c2fb192bcfc8d5c57e3`.
  Final synchronized `tasks.md` blob is
  `ae3221604ec43bd6cb77643056d605676b10472e`; `docs/status.md` blob is
  `12423f1ab81f6283c60e67260588fb39ffe8bc77`.
- Reviewer: Codex, independently reviewing the implementation authored by
  `claude-code`; this workflow turn kept production code, tests, and
  configuration read-only.
- Method: Formal code-and-tests Review under `development-workflow`. CodeGraph
  traced the one-snapshot reconcile, prior-state classifier, transaction,
  route-effect policy, resolver/cost/restart paths, and affected tests; focused
  source/contract comparison found a decisive accepted-delivery failure-path
  contradiction, so broad verification stopped. Developer-reported focused
  build/vet/tests remain implementation evidence but were not independently
  rerun in this round.
- Scope: Contract 3's one settings snapshot, ordered prior-state classifier,
  `advance`/`retain`/`unknown` effects, observation fields, resolver isolation,
  cost/restart consequences, and Task 3 test protection.
- Findings:
  - **[P1] E1-F1 — an unreadable or unparsable settings snapshot drops the
    accepted Hook instead of recording an indeterminate retained observation.**
    `reconcileClaudeConfigChange` (`cmd/agentdeck/main.go:3001-3041`) retries
    `ReadClaudeSettingsSnapshot`, then returns `lastConfigErr` when every attempt
    fails. `runUsageHookEvent` ignores that error, so neither observation nor
    route is written. The approved contract requires a read/parse failure to
    classify `indeterminate`, write no matched route, and preserve the accepted
    delivery in `usage_session_observations` with the unreadable conflict state
    (`tasks.md:160-163`; `architecture.md:214-215`, `:475-476`, `:718-730`).
    Current tests only prove the snapshot reader returns an error; none runs the
    Hook/reconcile path and asserts `retain` + `prior_state=indeterminate` +
    `conflict_scan=unreadable` with no route.
- Evidence: `main.go:3008-3035` proves all-attempt read/parse failure exits before
  `RecordHookDelivery`; `config_test.go:805-818` expects reader errors but does
  not assert the operation-level disposition. The route classifier can already
  store `retain` and `indeterminate`, but it receives no delivery on this branch.
- Completion gate: FAILED — CEv1 gate
  `switch-effectiveness-boundary:effective-route-policy` bound E1-F1 to exact
  content state
  `054d64a767bbc21b960737a5d59be137ce4ed0e77b4723fc0961a68282d8d874`;
  the single-snapshot, route-effect-policy, and L2/scope criteria are disproved.
- Verdict: REOPEN

## 📋 Effective route policy 独立评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`cmd/agentdeck/main.go:3034`] E1-F1：settings snapshot 连续读/解析失败时直接返回
error，Hook caller 又忽略 error，导致已接受的 ConfigChange observation 整条丢失。
- 行为风险：诊断流无法证明发生过该 Hook，`conflict_scan=unreadable` 永远不会产生；实现
把合同规定的 `indeterminate/retain` 错写成“双流零记录”。
- 证据：三次失败后在调用 `RecordHookDelivery` 之前返回；现有测试只验证 reader 报错。
💡 有界修复：在最终 snapshot read/parse failure 时仍构造 matched/indeterminate 的
`HookDelivery`（或等价的显式分类输入），让 shared transaction 写一条
`route_effect=retain`、`prior_state=indeterminate`、`conflict_scan=unreadable` 的
observation，同时不写 route；增加 command/reconcile 回归覆盖 missing 与 malformed 两类
settings，并断言 Hook 仍 fail-open。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 正常路径确实只读取并解析一次 settings document，match/conflict 共用同一 snapshot。
- Prior-state 顺序、同 transaction route read、keyed/no-key/indeterminate 分支清楚。
- Rotation、deletion 与其他 indeterminate matched case 使用 `retain`，mismatch 保持
`unknown`。
- Cost、session-after-switch 和 restart 回归覆盖了关键 attribution 后果。

### 📝 总结

当前 Task 3 fingerprint
`74e3c0a3b5227126cb48e3eb950233d74371d7396ee31c2fb192bcfc8d5c57e3`
完成了 classifier 主体，但 E1-F1 在 snapshot failure 路径丢失合同要求的 observation，
因此不能 PASS。宽验证应在该分支修复后的最终内容状态运行。

## Round 2 — 2026-08-26 (repair)

- Reviewed state: HEAD `8703fedf90571b7f3807a44034af2dd9fe2c8032` plus repaired
  implementation diff fingerprint
  `9b54f3dc3cd6f991d528edad78f7f125832c53ef4383b07c650421907a231add`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a
  verdict).
- Scope: E1-F1 only, as named in the repair command.
- Repair:
  - Added `HookDelivery.SettingsUnreadable bool`
    (`internal/usage/routes.go`): set when a Claude ConfigChange's managed
    settings snapshot could not be read or parsed on any reconcile attempt.
    `classifyConfigChange` now checks this first and returns
    `route_effect=retain`, `prior_state=indeterminate`,
    `conflict_scan=unreadable`, `conflict_sources=''`, with `config_matched`
    left `NULL` (a match was never determined, so it is not asserted false
    either) and no route write — the delivery is still accepted and observed,
    never silently dropped.
  - `reconcileClaudeConfigChange` (`cmd/agentdeck/main.go`): when every
    attempt fails to read or parse the snapshot
    (`lastConfigErr != nil && !inspectedConfig`), it now calls
    `RecordHookDelivery` with `SettingsUnreadable: true` instead of returning
    `lastConfigErr` directly. The read error itself is no longer propagated,
    keeping the Hook fail-open exactly as a confirmed mismatch already was.
- Regression tests:
  - `TestRecordHookDeliveryConfigChangeSettingsUnreadableRecordsIndeterminateRetain`
    (`internal/usage/routes_test.go`) pins the classifier unit directly:
    `SettingsUnreadable: true` yields exactly one observation with
    `retain`/`NULL`/`indeterminate`/`unreadable`/`''` and zero routes.
  - `TestClaudeConfigChangeMissingSettingsFileRecordsIndeterminateRetain` and
    `TestClaudeConfigChangeMalformedSettingsFileRecordsIndeterminateRetain`
    (`cmd/agentdeck/claude_reload_test.go`, sharing
    `assertUnreadableSettingsRecordsIndeterminateRetain`) cover both failure
    shapes through the real `reconcileClaudeConfigChange` path — an absent
    settings file and a persistently invalid one — asserting the call returns
    `nil` (fail-open) and the same one-observation/zero-route/indeterminate
    disposition.
- Verification: `go build ./...`, `go vet ./...` clean;
  `go test ./internal/usage/... ./internal/provider/... ./internal/store/... ./cmd/agentdeck/... -count=1`
  all pass, including the three new tests above; `gofmt -l .`,
  `make check-whitespace`, `bash scripts/check-topic-docs.sh`, and
  `git diff --check` clean.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the Task WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 5 — 2026-08-26

- Reviewed state: HEAD `8703fedf90571b7f3807a44034af2dd9fe2c8032` plus
  user-specified and independently reproduced full Task 3 scope fingerprint
  `471e1eeadab61f0f21ae26fc9a20c17ec0c3b8d18d48bcab0b31e2d70c25ba21`.
  Final synchronized `tasks.md` blob is
  `949afde3a7f815ac68498d891bb4598b0c2c7242`; `docs/status.md` blob is
  `a77a7247a006981992de80c972cf7fd9b61a6094`.
- Reviewer: Codex, independently re-reviewing the Round 4 repair; this workflow
  turn kept production code, tests, and configuration read-only.
- Method: Formal REREVIEW under `development-workflow`; exact fingerprint
  reproduction, focused source/contract inspection, independent unit and real
  reconcile regressions, and reuse of Round 4's full L2 log for the identical
  content state.
- Scope: E3-F1, all previously closed Task 3 findings for regression, and the
  final L2 gate.
- Findings:
  - **E1-F1 — CLOSED, unchanged.** Unreadable settings still produce one
    fail-open `retain`/`indeterminate`/`unreadable` observation and no route.
  - **E3-F1 — CLOSED.** Reconcile retains the last successfully read completed
    `ProviderSnapshot`; unreadable delivery carries it, and the classifier writes
    `observed_provider`, `observed_multiplier`, and `observed_via_wrapper` when
    present while keeping those columns NULL when no selection was observed.
  - No new finding.
- Evidence: fingerprint reproduction -> exact match;
  `scripts/run-go-test.sh ./internal/usage -run
  TestRecordHookDeliveryConfigChangeSettingsUnreadable` -> PASS (log
  `agentdeck-go-test.bpM3JA`); `scripts/run-go-test.sh ./cmd/agentdeck -run
  'TestClaudeConfigChange(Missing|Malformed)SettingsFileRecordsIndeterminateRetain'`
  -> PASS (log `agentdeck-go-test.xiqmvv`). Round 4's same-state
  `scripts/run-go-test.sh ./...` -> PASS (log `agentdeck-go-test.NmlaBz`), with
  build/vet and documentation checks also passing.
- Completion gate: VERIFIED — CEv1 gate
  `switch-effectiveness-boundary:effective-route-policy` verified all five
  required criteria for exact content state
  `6e54f00f30ab86301f2bb45dc36681d8682f25a035f6fa25a3082de852052666`.
- Verdict: PASS

## 📋 Effective route policy 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- E1-F1 与 E3-F1 均已关闭，unreadable observation 同时保留 disposition 与已观测
  selection。
- 有 selection 与无 selection 的两个字段适用性分支都有明确测试。
- 正常 single-snapshot、prior-state classifier、retain/unknown/advance、cost 与 restart
  证据未回退。
- 全仓 L2 与独立 targeted tests 均绑定同一 fingerprint。

### 📝 总结

当前完整 Task 3 fingerprint
`471e1eeadab61f0f21ae26fc9a20c17ec0c3b8d18d48bcab0b31e2d70c25ba21`
关闭了全部历史 finding，未发现新 blocker；classifier、observation shape、route
disposition、cost/restart 和 L2 evidence 已形成一致最终候选状态，可以 PASS。剩余交付
仅是单独授权的 commit/push checkpoint。

## Round 3 — 2026-08-26

- Reviewed state: HEAD `8703fedf90571b7f3807a44034af2dd9fe2c8032` plus
  independently reproduced full Task 3 scope fingerprint
  `e69a466b9bf28f8579377b01968683c6b6be57c8d12a1ce0425a9ee13ca54255`.
  Final synchronized `tasks.md` blob is
  `3f118d8d0ccdec2509a22f9245302af7b6cf9334`; `docs/status.md` blob is
  `d4a448796037ad7e7a2386dbd678d3105a688786`.
- Reviewer: Codex, independently re-reviewing the Round 2 repair; this workflow
  turn kept production code, tests, and configuration read-only.
- Method: Formal REREVIEW under `development-workflow`; E1-F1 was checked
  against the current reconcile/classifier path and its new failure regressions
  with CodeGraph and focused source inspection. A new observation-shape blocker
  was decisive, so broad verification stopped. Round 2's `9b54f3dc...` is a
  repair task-slice fingerprint; this round binds the complete current Task 3
  state to the reproducible `e69a466b...` fingerprint above.
- Scope: E1-F1, the repaired unreadable-snapshot observation, historical Task 3
  findings for regression, and any newly blocking contract mismatch.
- Findings:
  - **E1-F1 — CLOSED.** An all-attempt snapshot read/parse failure now enters
    `RecordHookDelivery`, writes one `retain`/`indeterminate`/`unreadable`
    observation, writes no route, and returns nil to keep the Hook fail-open.
    Missing and malformed settings are covered through the real reconcile path.
  - **[P2] E3-F1 — the unreadable observation drops a completed selection that
    was successfully read.** `reconcileClaudeConfigChange` reads
    `CurrentProviderSnapshot` before each failed settings read, but its final
    `SettingsUnreadable` delivery (`cmd/agentdeck/main.go:3040-3043`) carries
    neither `HasSelection` nor `Selection`. The unreadable classifier branch
    (`internal/usage/routes.go:130-141`) also never populates
    `observed_provider`, `observed_multiplier`, or `observed_via_wrapper`.
    Stream 1's contract requires these fields whenever a completed selection
    exists (`architecture.md:472-479`), because the observation records both
    what was observed and what was concluded. The new tests assert only effect,
    matched/prior/conflict fields, so they bless NULL selection columns.
- Evidence: current `main.go:3002-3010` proves provider snapshot success precedes
  settings failure; `:3040-3043` discards it. `routes.go:130-141` returns before
  copying selection fields. `claude_reload_test.go:305-313` and
  `routes_test.go:477-479` omit all three observed-selection assertions.
- Completion gate: FAILED — CEv1 gate
  `switch-effectiveness-boundary:effective-route-policy` verified E1-F1's route
  disposition closure and bound E3-F1 to exact content state
  `3d1675558b4398529089f543c21fa36213be9848c4c188d721d8b039abdb7383`;
  the single-snapshot observation and L2/scope criteria are disproved.
- Verdict: REOPEN

## 📋 Effective route policy 独立复评

📊 总体评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

[`cmd/agentdeck/main.go:3040`] E3-F1：unreadable settings observation 丢失已经成功
读取的 completed selection。
- 处置：新增 mandatory finding；E1-F1 已关闭。
- 行为风险：`observed_provider`、`observed_multiplier`、
  `observed_via_wrapper` 错误为 NULL，诊断流无法回答本次 reconcile 观察的是哪个选择，
与 Stream 1 的字段适用性合同不符。
- 证据：`SettingsUnreadable` delivery 不带 selection；classifier 直接返回；新测试不检查
三个 selection columns。
💡 有界修复：保留最后一次成功读取的 `ProviderSnapshot`，在 unreadable delivery 中
设置 `HasSelection/Selection`；unreadable classifier 写入对应 observed selection 字段，
同时继续保持 `retain/NULL/indeterminate/unreadable` 和零 route。扩展 unit 与真实
reconcile 测试断言 provider/multiplier/via-wrapper。

### 🟢 优点

- E1-F1 已关闭：missing/malformed settings 都写一条 observation 并保持 fail-open。
- `config_matched` 正确保留 NULL，未把未确定状态伪装成 mismatch。
- `retain/indeterminate/unreadable` 与零 route 的核心 disposition 已有 unit 和真实路径
覆盖。
- 正常 single-snapshot、prior-state、cost 和 restart 路径未被本次修复改动。

### 📝 总结

当前完整 Task 3 fingerprint
`e69a466b9bf28f8579377b01968683c6b6be57c8d12a1ce0425a9ee13ca54255`
关闭了 E1-F1，但 E3-F1 让 unreadable observation 缺少本已存在的 completed selection
事实，因此仍不能 PASS。宽验证应在 selection 字段修复后的最终状态运行。

## Round 4 — 2026-08-26 (repair)

- Reviewed state: HEAD `8703fedf90571b7f3807a44034af2dd9fe2c8032` plus repaired
  full Task 3 scope fingerprint
  `471e1eeadab61f0f21ae26fc9a20c17ec0c3b8d18d48bcab0b31e2d70c25ba21`,
  reproducible as `git diff HEAD --` over the seven Task 3 working-tree files
  (`cmd/agentdeck/main.go`, `cmd/agentdeck/claude_reload_test.go`,
  `internal/provider/config.go`, `internal/provider/config_test.go`,
  `internal/usage/routes.go`, `internal/usage/routes_test.go`,
  `internal/usage/usage_test.go`) piped to `shasum -a 256`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a
  verdict).
- Scope: E3-F1 only, as named in the repair command. E1-F1 stays CLOSED from
  Round 3 and its disposition was not re-opened or re-touched.
- Repair:
  - **E3-F1 — production.** `reconcileClaudeConfigChange`
    (`cmd/agentdeck/main.go`) now keeps the last successfully read
    `store.ProviderSnapshot` in `observedSelection`/`hasObservedSelection`,
    set on each attempt where `CurrentProviderSnapshot` returns no error. The
    all-attempts-failed `SettingsUnreadable` delivery carries
    `HasSelection: hasObservedSelection, Selection: observedSelection`, so a
    selection this reconcile actually observed is no longer discarded together
    with the unreadable settings document. Nothing else about the branch moved:
    it still returns nil (fail-open) and still does not propagate
    `lastConfigErr`.
  - **E3-F1 — classifier.** `classifyConfigChange`
    (`internal/usage/routes.go`) no longer returns early from the
    `SettingsUnreadable` branch with a bare disposition. It builds the same
    `retain` / `config_matched` NULL / `prior_state=indeterminate` /
    `conflict_scan=unreadable` / `conflict_sources=''` / no-route result and,
    when `HasSelection` is set, additionally populates `observed_provider`
    (via `runtimeProviderName`), `observed_multiplier`, and
    `observed_via_wrapper`. The completed selection is read from the store,
    not from the unreadable document, so Stream 1's "when a selection exists"
    applicability (`architecture.md:472-475`) is honored. The `HookDelivery`
    doc comment, which previously said `HasSelection` is ignored when
    `SettingsUnreadable` is true, was corrected to state why it still applies.
  - Route disposition is unchanged: the unreadable branch still sets no
    `writeRoute`, so zero routes are written.
- Regression tests:
  - `TestRecordHookDeliveryConfigChangeSettingsUnreadableRecordsIndeterminateRetain`
    (`internal/usage/routes_test.go`) now supplies a completed selection
    (`custom` / `2` / via-wrapper true) and asserts, through the new
    `readObservationSelection` helper, that `observed_provider`,
    `observed_multiplier`, and `observed_via_wrapper` are `custom`, `2`, and
    `1` — the three columns Round 3 found the old test was blessing as NULL —
    while keeping every previous retain/NULL/indeterminate/unreadable/no-route
    assertion.
  - `TestRecordHookDeliveryConfigChangeSettingsUnreadableWithoutSelectionKeepsSelectionNull`
    (new, same file) pins the complementary case: with no observed selection
    the same three columns stay NULL and the route count stays zero, so
    "not applicable" is still distinguishable from an observed value rather
    than the fix defaulting them to something.
  - `assertUnreadableSettingsRecordsIndeterminateRetain`
    (`cmd/agentdeck/claude_reload_test.go`), shared by the missing-settings and
    malformed-settings tests, now also asserts `custom` / `2` / `0` for the
    three observed-selection columns through the real
    `reconcileClaudeConfigChange` path, proving the snapshot survives the
    settings failure end to end and not only at the classifier unit.
- Verification: `go build -mod=vendor ./...` and `go vet -mod=vendor ./...`
  clean; `git diff --check`, `make check-whitespace` (exit 0), and
  `bash scripts/check-topic-docs.sh` (exit 0) clean.
  `go test -mod=vendor ./internal/usage/... ./internal/provider/... ./internal/store/... ./cmd/agentdeck/... -count=1`
  passes, with the four named tests confirmed executing by name. The broad
  verification Round 3 deferred is now run on this final content state:
  `scripts/run-go-test.sh ./...` passes for the whole repository (exit 0; log
  `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.NmlaBz`),
  which is Task 3's L2 gate. `gofmt -l` reports only the pre-existing,
  out-of-scope `cmd/agentdeck/usage_stats_viewer_test.go`, untouched by this
  repair.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the Task WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.
