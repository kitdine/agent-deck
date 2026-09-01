---
status: historical
topic: desktop-app
subject: presentation-period-scoping
retired: 2026-09-01
---

# Review log — desktop-app / presentation-period-scoping

> **Merge note (2026-08-20).** The user explicitly merged this delivered producer
> slice into task 3 `menubar-experience`. The rounds below remain unchanged as
> exact-state history for commit `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`;
> `presentation-period-scoping` is no longer a live task or an independent
> completion boundary. The combined task's current review continues in
> [`menubar-experience.md`](menubar-experience.md).

## Round 1 — 2026-08-19

- Reviewed state: commit `54e12ed9ceac8e26e07a8809670e1657923f9eab`;
  tree `0f9d1ec3cff7e9dc8be0f42b829d5a74de64ca11`.
- Reviewer: Codex
- Method: Initial formal Review under `development-workflow`, supplemented by
  `ln-12-delivery-reviewer` and one frozen independent challenge round with
  three fresh-context lenses: contract/API compatibility, data and period
  semantics, and tests/oracles. The previous Spark verdict was not treated as
  a project review because it left no workflow-compliant record or status
  transition. CodeGraph traced the two producer/decoder paths; every retained
  panel candidate was checked directly against the exact commit and current
  authoritative contracts.
- Scope: `54e12ed^..54e12ed` for task 3's Go usage/session producers, JSON
  contract and fixtures, standalone Swift verifier, production Swift DTOs and
  decoder tests. Current uncommitted menu-bar, project, scheme, localization,
  helper-runner, and application work was excluded.
- Findings:
  - [P1] PPS-F1 — an unavailable session index leaves
    `SessionsPeriods.Items` nil, so Go emits `data.sessions.periods.items:null`.
    The new Swift decoder accepts a missing `periods` family but rejects that
    present object because `items` must be an array. An isolated real producer
    envelope reproduced `DecodingError.valueNotFound`, so the partial snapshot
    cannot reach the intended unavailable UI state. -> Initialize the
    unavailable producer family with a non-null empty collection and add an
    actual producer-JSON-to-Swift regression path; `encoding/json` documents
    that nil slices encode as `null`:
    <https://pkg.go.dev/encoding/json#Marshal>.
  - [P1] PPS-F2 — the commit crosses task 3's explicit two-owner boundary with
    task 4 provider candidates/options, `provider.use` Swift decoding,
    sequential helper behavior, provider fixtures/contract output, and an App
    localization test. The localization test reads
    `AgentDeckApp/Localizable.xcstrings`, but that file is absent from the
    reviewed commit and only exists as later untracked work. -> Remove or
    relocate every provider-switch, helper-sequencing, and localization hunk to
    its owning task, leaving task 3 self-contained. Review does not authorize
    history rewriting.
  - [P1] PPS-F3 — the canonical fixtures are not valid examples of the current
    producer contract. The complete fixture has one scope, one period, one
    daily row, one rhythm cell, and one subtotal although the contract requires
    3 scopes, 3 periods per available scope, 90 daily rows, 168 rhythm cells,
    and 6 subtotals. The partial fixture declares `sessions.available:false`
    while `sessions.periods.available:true`, masking PPS-F1. Usage fixtures and
    tests also place every event today, so copying today's quality/pricing into
    `7d` and `30d` would stay green; the standalone Swift gate ignores
    `usage.presentation` and never consumes the legacy fixture. -> Make complete
    and partial fixtures producer-realistic and contract-valid, add
    distinguishable today/7d/30d values for both clients, assert the exact nine
    session `(period, client)` keys, and execute legacy/additive checks in both
    Swift decoder paths.
  - [P2] PPS-F4 — `distinct_projects` inserts `projectLabel(Project)` into its
    set unconditionally. `Metadata.Project` is the cleaned full project
    identity, but `projectLabel` discards parent paths and maps missing projects
    to `""`; different projects with the same basename collapse and an
    unattributed session counts as one project. -> Count distinct non-empty
    normalized project identities and keep basename projection only for
    display rows.
  - [P2] PPS-F5 — session period membership checks only `last >= start` and
    discards the local-day end. A future-dated session is therefore counted in
    today, 7d, and 30d even though its last event is not inside those bounded
    periods. -> Use a half-open local-calendar interval `start <= last < end`
    and cover end-boundary, offset, and DST cases with the standard `time`
    calendar operations: <https://pkg.go.dev/time>.
  - [P2] PPS-F6 — the contract requires an empty concrete-client scope to keep
    its record with unavailable families, but the producer marks periods,
    daily, quality, pricing, and rhythm available with synthetic zeros. ->
    Emit unavailable families with non-null empty collections for an
    unpopulated client and add producer/Swift assertions for that state.
- Evidence: task 3 ownership and acceptance in `tasks.md:40-79`; presentation
  and period contracts in `architecture.md:790-875`; exact commit diff and tree;
  CodeGraph producer/consumer traces; `git cat-file -e
  HEAD:apps/macos/AgentDeckApp/Localizable.xcstrings` -> exit 128; `jq` over the
  complete fixture -> scopes/periods/daily/rhythm/subtotals `1/1/1/1/1`;
  `scripts/run-go-test.sh ./internal/usage ./internal/desktop
  ./cmd/agentdeck` -> PASS; clean `54e12ed` archive through
  `scripts/test-macos-app.sh` -> PASS for its canonical fallback; isolated empty
  state through `agentdeck desktop snapshot`, then the committed Swift verifier
  -> FAIL at `data.sessions.periods.items` because the producer emitted `null`.
  Full L2 was intentionally stopped after the decisive P1 reproducer under the
  repository review rule. CEv1 schema discovery succeeded, but no WorkUnit or
  evidence record matching this task/commit exists; a failing review does not
  cross the completion gate.
- Delivery-reviewer verdict: FAIL
- Verdict: REOPEN

Checklist: 53/54 complete

Incomplete: VERIFY-7 — the broad L2 suite was not run after a decisive P1
reproducer; outcome impact: none for this FAIL verdict; exact next action: run
`scripts/run-go-test.sh ./...` after all findings are repaired and the final
relevant content state is frozen.

## 📋 评审报告

📊 综合评分：3/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`internal/desktop/desktop.go:189`] PPS-F1：真实 partial producer 输出
`sessions.periods.items:null`，Swift 新 decoder 无法解码。
- 行为风险：session index 不可用时，整个 desktop snapshot 被拒绝，而不是显示
  sessions unavailable。
- 证据：隔离状态真实输出经提交内 Swift verifier 稳定复现
  `DecodingError.valueNotFound`。
💡 有界修复：producer 初始化非 null 空数组，并增加真实 producer JSON 到 Swift 的
回归路径。

[`docs/topics/desktop-app/tasks.md:68`] PPS-F2：Task 3 提交混入 Task 4 的
provider switch、helper sequencing 和 localization hunks。
- 行为风险：任务无法独立评审/回退；干净提交中的 XCTest 还引用提交树外的
  `Localizable.xcstrings`。
- 证据：Task 3 明确只拥有 quality/pricing/sessions DTO hunks；`git cat-file`
  证明该 catalog 不在 `54e12ed`。
💡 有界修复：把所有 Task 4 hunks 和其测试/asset 归还 Task 4，只保留 Task 3 两个
wire owner 的自包含变更。

[`desktop/fixtures/v1/snapshot-complete.json:78`] PPS-F3：canonical fixtures
违反固定集合边界，并掩盖真实 partial 状态；现有 period oracle 不能区分各周期值。
- 行为风险：不可能由 producer 生成的 fixture 仍能让 decoder gate 通过，今天值复制到
  7d/30d 也不会被发现。
- 证据：complete fixture 的 scopes/periods/daily/rhythm/subtotals 实测为
  `1/1/1/1/1`；partial fixture 把 unavailable sessions periods 标成 available。
💡 有界修复：使用 producer-realistic fixture，覆盖完整固定边界、差异化周期/客户端值、
九个唯一 session pair 和 legacy 双 Swift 路径。

### 🟡 建议改进——建议修复

[`internal/desktop/desktop.go:498`] PPS-F4：distinct project 在显示 basename 上去重，
并把空标签算作一个 project。
- 行为风险：不同目录的同名 project 被合并，无 project 的 session 反而报告一个。
- 证据：`Metadata.Project` 保存 cleaned full identity；`projectLabel` 再做 `path.Base`。
💡 有界改进：按非空 normalized identity 计数，basename 只用于展示。

[`internal/desktop/desktop.go:461`] PPS-F5：session period 只有下界，没有本地日终上界。
- 行为风险：future-dated session 污染三个 period 的 count、duration、median 和 project
  count。
- 证据：`localDay` 的 end 被丢弃，筛选只检查 `Before(start)`。
💡 有界改进：使用 `[start, nextLocalMidnight)` 并覆盖边界、offset、DST。

[`internal/usage/presentation.go:357`] PPS-F6：无数据 client scope 被标成 available
零值，而权威契约要求保留 scope 但 family unavailable。
- 行为风险：UI 把“没有可用数据”误呈现为经过测量的零。
- 证据：三个 scope 始终构建为所有 family `available:true`，空 quality 还合成零记录。
💡 有界改进：空 concrete-client scope 输出 unavailable family 和非 null 空集合。

### 🟢 优点

正常有数据路径确实使用 period-first accumulator；sessions statistics 来自完整 metadata
集合而不是 recent list；recent list 仍受 limit 约束；顺序稳定；缺失 additive family 的
production Swift fallback 和 `wire_version == 1` 方向正确。定向 Go 测试和干净提交的
canonical Swift fallback 均通过。

### 📝 总结

评审对象是 commit `54e12ed9ceac8e26e07a8809670e1657923f9eab`、tree
`0f9d1ec3cff7e9dc8be0f42b829d5a74de64ca11`。六个 finding 均由该提交引入或由其
任务验收直接要求；当前工作区的后续 menu-bar 改动没有被计入。PPS-F1 是真实
producer-to-Swift 的确定性阻断，PPS-F2 破坏 task 原子边界，PPS-F3 使 canonical
evidence 不能证明契约；其余三项是会产生错误用户数据/状态的有界缺陷。因此 Round 1
必须 REOPEN。残余不确定性仅为未执行 full L2 和本机无 Xcode XCTest；两者都不能推翻
已有阻断证据。

## 下一步指令

`修复：desktop-app / reviews/presentation-period-scoping.md / PPS-F1 PPS-F2 PPS-F3 PPS-F4 PPS-F5 PPS-F6`

## Round 2 — 2026-08-19 (Repair)

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25`, with the
  repaired content uncommitted. Blob hashes of the repaired files:
  `internal/usage/presentation.go` `aad1f0084081`,
  `internal/usage/presentation_test.go` `984346d509f6`,
  `internal/desktop/desktop.go` `5be9ff0e9b5c`,
  `internal/desktop/desktop_test.go` `fa49a9ca3ba8`,
  `internal/desktop/fixtures_test.go` `2cbd1cb4d52b`,
  `desktop/fixtures/v1/snapshot-complete.json` `0ec32e389919`,
  `desktop/fixtures/v1/snapshot-partial.json` `7bdf67a0b9db`,
  `desktop/fixtures/v1/snapshot-empty-client.json` `f9e9d3d18f13`,
  `desktop/fixtures/v1/snapshot-legacy.json` `61525468f278`,
  `desktop/fixtures/v1/verify.swift` `4ca21780340d`,
  `desktop/fixtures/v1/README.md` `45f15914cd5e`,
  `apps/macos/AgentDeckShared/DesktopWire.swift` `a4ef434f83c6`,
  `apps/macos/AgentDeckTests/DesktopWireTests.swift` `5130c6c8b01e`,
  `apps/macos/AgentDeckTests/FixtureSupport.swift` `7ee27fb1dca5`,
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json` `053a47392780`.
- Repairer: Claude Code
- Method: `修复` under `development-workflow`, scoped to the six authorized
  findings. Every finding was reproduced against the reviewed content before it
  was changed, and every behavioural repair carries a test that was run against
  the reinstated defect to prove it fails without the fix.
- Scope: task 3's Go producers, canonical fixtures, standalone Swift decoder,
  production Swift DTO tests, and the GUI command-contract golden. The
  uncommitted `menubar-experience` work was deliberately left in the tree rather
  than stashed, because the app target is the only consumer of these DTOs and a
  `swiftc -typecheck` over it is what proves the wire change did not break the
  consumer. A SHA-256 fingerprint taken before and after the repair shows all 21
  menu-bar artifacts byte-identical, so nothing in this round touched them.

### What changed in the commit topology

Round 1 reviewed commit `54e12ed`. **That commit no longer exists on `main`.**
The user's decision was that its real defect was neither its content nor its
boundary but its timing: the project commits a task after `Review PASS`, and
`54e12ed` landed before review, which is what created a failing commit to
operate on at all. `main` is therefore back at `f5935b6` and every artifact —
task 3's implementation, this round's repairs, and the separate
`menubar-experience` work — is uncommitted.

An intermediate commit `b88a38e` split the provider-switch surface out of task 3
before that decision was taken; it was reset as well. Both commits remain
reachable through the reflog and through the `pre-split-54e12ed` tag, and no
content was discarded in either direction.

- Findings and disposition:
  - **PPS-F1 — CLOSED.** Reproduced exactly: an empty state root produced
    `"sessions":{"periods":{"available":false,"items":null}}`, and both Swift
    decoders reject a present family whose `items` is null. `emptySessionsSnapshot`
    now supplies non-null empty collections.
    `TestUnavailableSessionIndexEncodesEmptyArraysRatherThanNull` asserts the
    encoded shape from a real `Service.Build`, and fails with the exact
    pre-repair payload when the defect is reinstated. Both Swift gates decode
    the real payload, and injecting `items: null` back into the repaired fixture
    still raises `DecodingError.valueNotFound` at
    `data.sessions.periods.items` — so the producer, not a decoder relaxation,
    is what fixed it.
  - **PPS-F2 — CLOSED in the tree; the commit-boundary half is now moot.** The
    App string-catalog test has been removed from `AgentDeckSharedTests`: it read
    `AgentDeckApp/Localizable.xcstrings`, which that target cannot see, and
    pinned a string count that is already stale. `menubar-experience` covers the
    catalog against the built bundle instead, which is the stronger check. The
    provider-switch surface stays in the working tree because
    `menubar-experience` depends on it; with no task-3 commit in existence there
    is no longer an impure commit to correct, and the boundary is a requirement
    on the eventual commit checkpoint rather than an open defect.
  - **PPS-F2 sub-item, partially corrected.** The finding named "sequential
    helper behavior" among the mixed hunks. `EmbeddedHelperRunner.swift` was
    **not** in `54e12ed` — `git show --stat` lists twelve files and it is not one
    of them — so that half was inferred from the working tree rather than from
    the commit. The other half does hold: `FixtureSupport.swift`'s
    multi-behaviour recorder was in the commit and exists only for the sequential
    index-refresh tests. An earlier statement in this repair that the whole
    sub-item was unreproducible was too narrow and is corrected here.
  - **PPS-F3 — CLOSED.** The canonical fixtures are no longer hand-written. They
    are generated by the producer and pinned by
    `TestCanonicalFixturesAreReproducibleProducerOutput`, with an explicit
    `AGENTDECK_UPDATE_FIXTURES=1` update mode. The complete fixture now carries
    three scopes, three periods, 90 daily buckets, 168 rhythm cells, three
    pricing records and six subtotals per the contract, with values that differ
    per client and per period so copying today's figures into `7d` and `30d`
    cannot pass. The partial fixture is the real degraded state and no longer
    declares an available session-period family beside an unavailable session
    index. A third fixture, `snapshot-empty-client.json`, carries the state
    PPS-F6 describes. The exact nine `(period, client)` session keys are asserted
    in Go, in the production Swift decoder tests, and in the standalone gate;
    that standalone gate previously had no `usage.presentation` DTOs at all and
    now decodes the family, tolerates its absence, and consumes the legacy
    fixture.
  - **PPS-F4 — CLOSED.** `projectIdentity` normalizes without discarding parents
    and `projectLabel` keeps the basename projection for display only. Distinct
    projects count non-empty identities, so two checkouts named `agent-deck`
    under different parents are two projects and an unattributed session is none.
  - **PPS-F5 — CLOSED.** Period membership is the half-open local-calendar
    interval `start <= last < end`, with `end` the next local midnight. Covered
    by an end-boundary case, a future-dated case, and a DST case over
    `America/Los_Angeles` that pins the 7-day window to seven calendar days
    across the transition.
  - **PPS-F6 — CLOSED.** A concrete client with no data returns
    `unavailablePresentationScope`: the record survives, every family reports
    unavailable, and every collection is a non-null empty array. `all` stays
    available because the contract calls it an explicit scope rather than a
    missing client, so a genuine zero is still reported as measured.
- Evidence: `scripts/run-go-test.sh ./...` PASS — this also closes Round 1's
  incomplete `VERIFY-7`, which asked for the broad L2 suite once the final
  content state was frozen; `go vet -mod=vendor ./...` PASS;
  `bash scripts/test-macos-app.sh` PASS; the standalone
  `swift desktop/fixtures/v1/verify.swift` over all four fixtures PASS;
  `swiftc -typecheck` of the whole `AgentDeckApp` target against the changed
  wire PASS; `make check-whitespace` and `git diff --check` clean. Each
  behavioural repair was additionally run against its reinstated defect and
  observed to fail: F1 on the literal `"items":null` payload, F4 on
  `distinct projects = 1, want 0`, F5 on `today/all sessions = 2`, and F6 on a
  claude scope carrying `Available:true` with synthetic zeros. Fixture
  reproducibility was verified 11 consecutive times including under
  `-shuffle=on`.
- Residual risk: the fixture generator was briefly non-reproducible because
  `doctor` reports on the state root's mode and the test let that directory be
  created implicitly, so the health section drifted with ambient filesystem
  state. The generator now creates the state root explicitly at `0700`, which is
  what a real state root is, and the drift is gone. Separately, PPS-F6 makes an
  empty concrete client report unavailable families, and
  `desktopSnapshotIsPartial` in `AgentDeckShared` treats any unavailable family
  as `partial` — so a user with only one client configured would read
  "Some data unavailable" permanently. That helper belongs to
  `menubar-experience`, not to this task, and is recorded here as a consequence
  for that task's review rather than changed under this one. XCTest could not be
  executed: this machine has Command Line Tools only, so the production Swift
  decoder assertions are syntax-checked, not run.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 5 — 2026-08-19 (Re-review)

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25`; the 15
  task-behaviour blob hashes verified in Rounds 3 and 4 remain unchanged, and
  the repaired task-boundary document was blob `e7f9fcf4bb18` before this
  round's status synchronization.
- Reviewer: Codex
- Method: Independent `复评` under `development-workflow`, limited to the sole
  open finding PPS-F7. `verification-before-completion` is used after this
  round and the task matrix are finalized, so CEv1 evidence binds the final
  synchronized candidate rather than the pre-review status blob.
- Scope: task 3's authoritative Files/Creates boundary, shared-file hunk
  ownership, exact uncommitted task content, and the unchanged evidence reused
  for PPS-F1 through PPS-F6. Current task 4 and task 6 hunks remain excluded.
- Finding dispositions:
  - **PPS-F1 through PPS-F6 — CLOSED, unchanged.** All 15 behaviour blobs match
    the state independently verified in Round 3; phase movement and the
    `tasks.md` ownership edit do not invalidate their tests or producer/decoder
    evidence.
  - **PPS-F7 — CLOSED.** Task 3 now lists
    `desktop/fixtures/v1/README.md` and the fixture-argument hunk of
    `scripts/test-macos-app.sh` under Files, plus
    `internal/desktop/fixtures_test.go` and
    `desktop/fixtures/v1/snapshot-empty-client.json` under Creates. It also
    declares the fixture-argument and presentation/legacy/empty-client assertion
    hunks in `apps/macos/AgentDeckVerification/main.swift`, which are necessary
    for the production Swift decoder path and were omitted by Round 3's original
    four-path enumeration. The embedded-helper invocation-count hunk in that
    file remains outside task 3 and is explicitly left to task 4.
- Evidence: all 15 behaviour blobs recomputed byte-identical; current task 3
  boundary and every uncommitted task-3 path compared directly; Round 4
  `make check-whitespace`, `git diff --check`, and
  `bash scripts/check-topic-docs.sh` PASS. Round 3's targeted Go tests,
  standalone four-fixture Swift gate, production
  `AgentDeckFoundationVerifier`, and Round 2's full Go/vet/typecheck evidence
  remain reusable because their code, tests, fixtures, scripts, toolchain, and
  relevant environment are unchanged.
- Completion evidence: before synchronization the configured store had no
  WorkUnit or criteria for `desktop-app:presentation-period-scoping`; its
  empty-criteria `VERIFIED` response was vacuous. After this record and the task
  matrix are finalized, the coordinator provisions the Task WorkUnit, required
  criteria, final scoped content state, and passing evidence through the fixed
  CEv1 templates, then re-queries the gate. CEv1 remains the authority for that
  result and this review verdict does not override it.
- Verdict: PASS

Checklist: 54/54 complete

Incomplete: None

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——建议修复

无。

### 🟢 优点

PPS-F1 至 PPS-F7 全部关闭。Task 3 现在同时具备两个 wire owner 的实现、真实
producer-generated fixtures、两个 Swift decoder gate、完整回归测试和按 hunk 明确的
Files/Creates 边界；Task 4/6 的共享文件修改没有被并入本任务。

### 📝 总结

复评对象是 HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25` 上的未提交
Task 3 candidate。行为内容与 Round 3 完全一致，PPS-F7 只修正权威 ownership，且新增的
`main.swift` hunk归属是关闭“两个 Swift decoder paths”所必需。所有历史 finding 已关闭，
无新的运行时、wire、fixture、测试或边界缺陷。正式复评结论为 PASS；Task 完成状态仍以
本轮状态同步后的 CEv1 gate 结果为准。

## Round 3 — 2026-08-19 (Re-review)

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25` plus the
  uncommitted task content identified by the 15 blob hashes in Round 2. Every
  listed blob was recomputed before review and matched Round 2 exactly.
- Reviewer: Codex
- Method: Independent `复评` under `development-workflow`, using a selective
  `ln-12-delivery-reviewer` follow-up and `verification-before-completion` for
  the newly crossed Task gate. CodeGraph traced the repaired owning paths;
  retained evidence was checked against the unchanged blobs. No second
  subagent panel was needed because the frozen Round 1 panel already identified
  the risks and every disposition had a deterministic local check.
- Scope: PPS-F1 through PPS-F6 against the repaired Go producers, generated
  fixtures, standalone and production Swift decoder paths, tests, and the
  task-level commit boundary. Current menu-bar/App work remained excluded.
- Finding dispositions:
  - **PPS-F1 — CLOSED.** `emptySessionsSnapshot` initializes both collections as
    non-null empty slices. The real unavailable producer path is asserted by
    `TestUnavailableSessionIndexEncodesEmptyArraysRatherThanNull`; the partial
    fixture decodes through both Swift gates, while an injected `items:null`
    remains invalid.
  - **PPS-F2 — CLOSED for the reviewed historical defect.** The impure
    `54e12ed` commit is no longer on `main`, the localization test that depended
    on a file outside that commit is gone, and the current task 3 scope excludes
    provider-switch, helper-sequencing, localization, and menu-bar hunks. A
    later checkpoint must still stage shared files by task-owned hunk and leave
    `FixtureSupport.swift`'s current multi-behaviour change with task 4; this is
    a delivery constraint, not an open defect in task 3's period-scoping code.
  - **PPS-F3 — CLOSED.** Complete, partial, and empty-client fixtures are now
    byte-pinned producer output. The complete fixture has the required three
    scopes, differentiated today/7d/30d quality and pricing values for both
    clients, six subtotals, and the exact nine ordered session keys. The partial
    fixture carries an unavailable period family with empty arrays. Both Swift
    gates consume complete, partial, empty-client, and legacy fixtures.
  - **PPS-F4 — CLOSED.** Period statistics count non-empty normalized full
    project identities; recent display rows alone use the basename. Tests cover
    duplicate basenames and no-project sessions.
  - **PPS-F5 — CLOSED.** Session membership is `start <= last < next local
    midnight`; future/end-boundary and DST calendar-day tests pass.
  - **PPS-F6 — CLOSED.** An empty concrete client keeps its scope but every
    family is unavailable with non-null empty collections. Go and both Swift
    fixture paths cover the state.
  - **[P1] PPS-F7 — NEW.** The repair added task-required paths that the
    authoritative task 3 Files/Creates boundary does not own:
    `internal/desktop/fixtures_test.go`,
    `desktop/fixtures/v1/snapshot-empty-client.json`,
    `desktop/fixtures/v1/README.md`, and `scripts/test-macos-app.sh`. The first
    two are new files and the latter two are required changes that register and
    document the four-fixture gate. The review record cannot expand task
    ownership, and an undeclared file set cannot form the task's atomic commit
    checkpoint. -> Add these exact paths to task 3's Files/Creates boundary, or
    relocate the repair into already-owned paths and revert the undeclared
    changes. Preserve all task 4 hunks as excluded dirty work.
- Evidence: all Round 2 blob hashes matched; focused source review of
  `emptySessionsSnapshot`, session interval/project aggregation,
  `unavailablePresentationScope`, fixture generation, and both Swift decoders;
  `scripts/run-go-test.sh ./internal/usage ./internal/desktop -run <PPS repair
  tests>` PASS; standalone Swift verifier over all four fixtures PASS;
  `bash scripts/test-macos-app.sh` PASS through
  `AgentDeckFoundationVerifier`; fixture inspection showed differentiated
  quality/pricing event counts and all nine unique session keys. Round 2's
  unchanged `scripts/run-go-test.sh ./...`, `go vet`, typecheck, whitespace,
  and diff evidence remains reusable.
- Completion evidence: the configured CEv1 store contains no WorkUnit or
  criteria for `desktop-app:presentation-period-scoping`. The fixed gate query
  returned `criteria: []` and `VERIFIED`; that is a vacuous empty-criteria result,
  not completion evidence. The Task gate is therefore `NOT_VERIFIED`, and no
  evidence nodes were written while PPS-F7 keeps the boundary open.
- Verdict: REOPEN

Checklist: 54/54 complete

Incomplete: None

## 📋 复评报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`docs/topics/desktop-app/tasks.md:61`] PPS-F7：Round 2 的必要修复路径没有进入
Task 3 的权威 Files/Creates 边界。
- 处置：NEW
- 行为风险：复评通过后无法形成与已审内容一致的原子 Task checkpoint；遗漏新 fixture/
  generator 会丢失修复，直接提交又会超出批准范围。
- 证据：task 3 当前文件表未列
  `internal/desktop/fixtures_test.go`、`snapshot-empty-client.json`、fixture
  `README.md` 或 `scripts/test-macos-app.sh`，而修复和四-fixture gate 必须使用它们。
💡 有界修复：把这四个路径明确归入 task 3，或把修复迁回已有 owner 并撤回未声明路径；
Task 4 的 provider/helper/menu-bar hunks继续排除。

### 🟡 建议改进——建议修复

无。

### 🟢 优点

PPS-F1 至 PPS-F6 均已按 owning boundary 关闭。真实 unavailable producer、固定集合边界、
差异化周期/客户端值、完整 project identity、本地日历上界、DST 和空 client 状态都有
针对性证据；定向 Go、standalone Swift 和生产 Swift fallback 全部通过。

### 📝 总结

本轮复评的 15 个修复 blob 与 Round 2 完全一致。六个原 finding 全部关闭，没有发现新的
运行时或 wire-contract 缺陷；唯一开放项是修复路径未同步到 Task 3 权威 ownership，导致
提交边界和 CEv1 WorkUnit 仍不能成立。由于 workflow 不允许把未归属文件或空条件
`VERIFIED` 带过 PASS，本轮必须 REOPEN。修正 PPS-F7 后只需复核 ownership、最终
content-state 和 completion gate，不应重跑未受影响的行为检查。

## 下一步指令

`修复：desktop-app / reviews/presentation-period-scoping.md / PPS-F7`

## Round 4 — 2026-08-19 (Repair)

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25`. The 15 task
  blobs Round 3 verified are **unchanged**; this round's only content change is
  `docs/topics/desktop-app/tasks.md`, blob `e7f9fcf4bb18`.
- Repairer: Claude Code
- Method: `修复` under `development-workflow`, scoped to PPS-F7 alone. The
  finding offers two remedies; the first was taken, and the reasoning for that
  choice is recorded below rather than left implicit.
- Scope: task 3's Files/Creates boundary in `tasks.md`. No product code, test,
  fixture, script, or generated artifact was touched.
- Findings and disposition:
  - **PPS-F7 — CLOSED.** The four paths Round 3 named are now declared in task
    3's boundary: `desktop/fixtures/v1/README.md` and `scripts/test-macos-app.sh`
    under Files, and `internal/desktop/fixtures_test.go` and
    `desktop/fixtures/v1/snapshot-empty-client.json` under Creates, whose count
    sentence moves from three new files to five. `scripts/test-macos-app.sh` is
    scoped to **the fixture-argument list only**, because task 6 already claims
    that script unqualified; scoping task 3's share by hunk resolves the double
    claim from this task's side without editing another task's list, which is the
    convention the document already uses for `project.pbxproj` and
    `EmbeddedHelperRunner.swift`.
  - **Why the first remedy and not the second.** Relocating the repair into
    already-owned paths would have meant dropping `snapshot-empty-client.json`
    and reverting both the README and the script hunk. That would have removed
    the only fixture in which either Swift decoder sees the empty concrete-client
    state, left a generated fixture set documenting a stale two-path invocation,
    and unregistered the four-fixture gate from the command that runs it —
    degrading three dispositions Round 3 had just confirmed CLOSED. The defect
    PPS-F7 identifies is that `tasks.md` under-declared the task's file set, so
    the correction belongs in `tasks.md`.
  - **One path added beyond the four enumerated, stated explicitly so re-review
    can reject it.** `apps/macos/AgentDeckVerification/main.swift` carries this
    repair's task-3 hunks — four-fixture argument handling and the
    presentation-bounds, empty-client and legacy assertions — and was equally
    undeclared. Closing PPS-F7 without it would satisfy the finding's list while
    leaving its stated consequence true, because the task-3 hunks in that file
    would still be unable to enter the task's atomic commit. It is declared
    hunk-scoped for the same reason `test-macos-app.sh` is. That file matters to
    the finding's own subject: `verify.swift` alone does not satisfy "both Swift
    decoder paths", since it carries a private DTO layer rather than
    `AgentDeckShared`'s, and `main.swift` is where the production decoder reads
    all four fixtures.
  - **Left open deliberately, for task 4's review rather than this one.** The
    other hunk in `main.swift` — the embedded-helper invocation count, changed
    from one to three when task 4 introduced the index-refresh cadence — is
    declared by no task. Recording it here rather than editing task 4's boundary
    keeps this repair inside its authorized scope.
- Evidence: verification level L0, because nothing outside a status and
  ownership document changed. `make check-whitespace` clean;
  `git diff --check` clean; `bash scripts/check-topic-docs.sh` clean. The 15
  content blobs listed in Round 2 were recomputed after this edit and are
  byte-identical, so Round 3's behavioural evidence — `run-go-test.sh ./...`,
  `go vet ./...`, `test-macos-app.sh`, the standalone verifier over four
  fixtures, and the `AgentDeckApp` typecheck — remains bound to the same content
  state and is not rerun. A sweep of every uncommitted task-3 path against the
  updated boundary reports no remaining undeclared path.
- Task 4 hunks preserved as excluded dirty work, which is the finding's second
  clause: a SHA-256 fingerprint of all 21 `menubar-experience` artifacts is
  byte-identical to the pre-repair baseline, and the five task-4 hunks in shared
  files — `EmbeddedHelperRunner.swift`, `AppGroupSnapshotStore.swift`,
  `EmbeddedHelperRunnerTests.swift`, `DesktopRefreshCoordinatorTests.swift` and
  `scripts/build-macos-app.sh` — remain uncommitted and untouched by rounds 2
  and 4 alike.
- Residual risk: the CEv1 Task gate stays `NOT_VERIFIED`. Round 3 established
  that the store holds no WorkUnit or criteria for
  `desktop-app:presentation-period-scoping`, and an empty-criteria `VERIFIED` is
  vacuous rather than completion evidence; this repair changed a boundary
  document and does not create that record. XCTest still cannot run on this
  machine, so the production Swift decoder assertions remain syntax-checked
  rather than executed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.
