---
status: active
topic: switch-effectiveness-boundary
subject: hook-delivery-ledger
---

# Review log — switch-effectiveness-boundary / hook-delivery-ledger

## Round 1 — 2026-08-26

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus
  implementation diff fingerprint
  `aca01e6ad437dcb71215b74e9d14c4a0659515488d365be2dd474a3b2ee9c6ca`.
  Final synchronized `tasks.md` blob is
  `9b2984e8835f9b659fed04a9c5b401ba946a8d0d`; `docs/status.md` blob is
  `439812ff42860e83d704fcd34fe4b55786cd5fea`.
- Reviewer: Codex, independently reviewing the implementation authored by
  `claude-code`; this workflow turn kept production code, tests, and
  configuration read-only.
- Method: Formal code-and-tests Review under `development-workflow`. CodeGraph
  traced the Hook adapter, shared transaction, store opening, migration, and
  affected tests; the scoped diff and approved Task 1 contract were then checked
  directly. Broad verification stopped after the decisive scope/concurrency
  blocker. The developer-reported build, vet, and focused Go suites are retained
  as implementation evidence but were not independently rerun in this round.
- Scope: `hook-delivery-ledger` admission, migration 19, normalized observation,
  route non-regression, whole-operation idempotence/atomicity, concurrency,
  privacy, and Task 1 test protection.
- Findings:
  - **[P1] H1-F1 — Task 1 silently changes transaction policy for the entire
    core store.** `internal/store/store.go:228-234` adds
    `_txlock=immediate` to the shared `store.Open` DSN, so every core
    `BeginTx` acquires the SQLite write lock at transaction start. Task 1's
    approved file boundary (`tasks.md:108-112`) does not include `store.go`, and
    the approved architecture explicitly described the Hook ordering as a
    placement rule rather than a new concurrency primitive. The diff therefore
    changes wait/serialization behavior for unrelated provider, usage, session,
    extension, and migration transactions to make one Task's concurrent test
    pass, without an approved blast-radius contract or regression coverage.
  - **[P1] H1-F2 — the required atomic-failure case has no regression.** Task 1
    requires a route-write failure to leave zero observations and zero routes
    (`tasks.md:91-93`), but the new tests end with the same-delivery retry case at
    `internal/usage/routes_test.go:346-375`; none forces the optional route write
    to fail after the observation insert. A future partial-commit regression in
    the highest-risk transaction path would therefore pass this suite.
  - **[P2] H1-F3 — migration verification does not prove the unique delivery
    guard or exact schema.** `internal/store/store_test.go:721-775` reads only
    column names/types and counts two indexes by name. A non-unique
    `usage_session_observations_delivery` index, wrong `NOT NULL`/default/primary
    key properties, or changed index columns would still pass, despite Task 1
    requiring the unique `delivery_id` index and exact Stream 1 DDL. The v18
    preservation case also samples only two `usage_events` fields rather than
    comparing the complete pre-migration row.
- Evidence: CodeGraph showed `store.Open` has 221 callers and the new DSN is the
  ownership point for every core database connection; focused search found 27
  current Go `BeginTx` call sites. The scoped source diff establishes the
  unapproved `store.go` change and the missing negative/uniqueness assertions.
- Completion gate: FAILED — CEv1 gate
  `switch-effectiveness-boundary:hook-delivery-ledger` bound H1-F1 through
  H1-F3 to exact content state
  `650e05b54a6a45a5df9e8851873efb2e52c1658c8f772cd349abe64f7d76bdf9`;
  the scope/L2, atomic-operation, and migration-19 criteria are disproved.
- Verdict: REOPEN

## 📋 Hook delivery ledger 独立评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`internal/store/store.go:228`] H1-F1：Task 1 通过全局 `_txlock=immediate`
改变所有 core-store 事务的加锁时机，超出批准文件与并发合同。
- 行为风险：无关 provider、usage、session、extension 与 migration 路径会更早争用写锁，
其等待和 `SQLITE_BUSY` 行为被一起改变。
- 证据：Task 1 文件表不含 `store.go`；共享 `store.Open` 有大量调用者，当前仓库有 27 个
Go `BeginTx` 调用点。
💡 有界修复：移除全局 DSN 改动，把 Hook 并发保证限制在已批准的
`RecordHookDelivery` 边界；若确实必须采用全局策略，先正式重开 architecture/tasks 并
补齐全局 blast-radius 验收，而不是在本 Task 中隐式引入。

[`internal/usage/routes_test.go:346`] H1-F2：没有 route write 失败后的双流 rollback
回归。
- 行为风险：observation 已插入、route 写失败时的部分提交回归不会被测试发现。
- 证据：现有新增用例覆盖成功、并发和 duplicate retry，但没有在 observation insert
之后强制 route insert 失败。
💡 有界修复：增加确定性的 route-write failure 注入，断言函数返回错误且 observation、
route 两张表均保持调用前状态。

### 🟡 建议改进——推荐

[`internal/store/store_test.go:721`] H1-F3：migration test 只按名称计数索引，没有验证
unique、列顺序及完整 DDL 属性。
- 证据：查询只读取 `pragma_table_info` 的 `name,type` 和 `sqlite_master` 的 index 名称
计数；非 unique delivery index 仍会通过。
💡 有界改进：断言 `pragma_index_list/index_info` 的 unique 与列，读取
`notnull/dflt_value/pk`，并完整比较 v18 route/event 行。

### 🟢 优点

- Admission 之后统一进入 `RecordHookDelivery`，无效 transcript 与 unmanaged
  ConfigChange 的双流零写入已有边界测试。
- Observation 与 optional route 位于同一代码事务，delivery ID retry 和连续相同 route
  no-op 的正向行为清楚。
- Migration 19 为纯新增表/index，resolver 路径没有读取 observation stream。

### 📝 总结

当前实现 fingerprint
`aca01e6ad437dcb71215b74e9d14c4a0659515488d365be2dd474a3b2ee9c6ca`
完成了共享 ledger 的主体结构，但 H1-F1 的全局事务语义扩张越过 Task 边界，H1-F2 与
H1-F3 又使两个明确验收条件缺少可失败的保护，因此不能 PASS。剩余不确定性集中在未
独立重跑的宽测试；项目规则要求在决定性 blocker 修复后的最终内容状态再运行它们。

## Round 2 — 2026-08-26 (repair)

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus repaired
  implementation diff fingerprint
  `f112c781b05a85df9bd0a933486f4b56b098de80b5874aab3a0ca2c42c593b43`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a
  verdict).
- Scope: H1-F1, H1-F2, H1-F3 only, as named in the repair command.
- Repair:
  - **H1-F1.** Reverted `internal/store/store.go`'s DSN back to
    `busy_timeout(5000)` + `foreign_keys(1)` with no `_txlock` change, so no
    other `BeginTx` caller's transaction semantics moved. `RecordHookDelivery`
    (`internal/usage/routes.go`) now pins a single `*sql.Conn` via
    `s.Store.DB.Conn(ctx)` and issues `BEGIN IMMEDIATE` / `COMMIT` / `ROLLBACK`
    as plain statements on that connection only, so the write-lock-at-start
    guarantee this Task's atomic transaction needs is scoped to the Hook
    boundary, not the shared store DSN. `recordSessionRouteTx` is renamed
    `recordSessionRouteConn` and takes `*sql.Conn` instead of `*sql.Tx`.
  - **H1-F2.** Added
    `TestRecordHookDeliveryRouteWriteFailureRollsBackBothStreams`
    (`internal/usage/routes_test.go`), which installs a `BEFORE INSERT`
    trigger on `usage_session_routes` that unconditionally `RAISE(ABORT, ...)`,
    forcing the route write to fail after the observation insert already
    affected one row. Asserts the call returns an error and both
    `usage_session_observations` and `usage_session_routes` are empty
    afterward, then drops the trigger and confirms a following delivery
    commits normally (proving the failed transaction released its lock rather
    than leaving the connection stuck).
  - **H1-F3.** `TestV19MigrationAddsHookDeliveryObservationLedger`
    (`internal/store/store_test.go`) now asserts full column specs
    (`notnull`/`dflt_value`/`pk`, not just name/type) via
    `pragma_table_info`, and asserts each index's uniqueness and exact column
    order via `pragma_index_list`/`pragma_index_info` — a non-unique
    `usage_session_observations_delivery` index or a reordered lookup index
    now fails the test. The v18-preservation case reads the complete
    `usage_events` row generically (`SELECT *` into a column-name-keyed map)
    before and after migration and compares with `reflect.DeepEqual`, instead
    of sampling `model`/`source_path`.
- Verification: `go build ./...`, `go vet ./...` clean;
  `go test ./internal/usage/... ./internal/store/... ./cmd/agentdeck/... -count=1`
  all pass, including the new failure-injection and strengthened migration
  tests; `git diff --check` clean.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the Task WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 5 — 2026-08-26

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus
  user-specified and independently reproduced implementation diff fingerprint
  `214b2d3e10fd13cf8f594277f540c605499d640305d23c40dbe5f9a92aa50574`.
  Final synchronized `tasks.md` blob is
  `67899bf02bd82fd0c7f1bcf15995f6f4b3bad3cc`; `docs/status.md` blob is
  `3c5f43fd3860c639d448636530289f9b26d9e06e`.
- Reviewer: Codex, independently re-reviewing the Round 4 repair; this workflow
  turn kept production code, tests, configuration, and fixtures read-only.
- Method: Formal REREVIEW under `development-workflow`; exact fingerprint
  reproduction, focused fixture/task diff inspection, an independent canonical
  fixture producer test, and reuse of the full L2/race/vet evidence recorded for
  the identical content state.
- Scope: H3-F1, all previously closed findings for regression, and Task 1's
  final L2/concurrency gate.
- Findings:
  - **H1-F1 — CLOSED, unchanged.** `BEGIN IMMEDIATE` remains scoped to the pinned
    Hook connection and the shared store DSN is unchanged.
  - **H1-F2 — CLOSED, unchanged.** The route-failure test still proves both
    streams roll back and the connection/lock is released.
  - **H1-F3 — CLOSED, unchanged.** Exact DDL/index and complete v18 row
    assertions remain present.
  - **H3-F1 — CLOSED.** The two canonical fixtures were regenerated to schema
    count `19`; their entire diff is exactly two `18` → `19` replacements, and
    the producer-reproducibility test now passes.
  - No new finding.
- Evidence: fingerprint reproduction -> exact match; fixture diff -> only
  `desktop/fixtures/v1/snapshot-complete.json:6620` and
  `snapshot-empty-client.json:4412` change from `18` to `19`;
  `scripts/run-go-test.sh ./internal/desktop -run
  TestCanonicalFixturesAreReproducibleProducerOutput` -> PASS (log
  `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.rZz6Dx`).
  The same-state Round 4 evidence is reusable: `scripts/run-go-test.sh ./...`
  -> PASS (log `agentdeck-go-test.O0vWFQ`), `go vet -mod=vendor ./...` -> PASS,
  and `go test -mod=vendor -race ./internal/usage/... ./internal/store/...
  -count=1` -> PASS.
- Completion gate: VERIFIED — CEv1 gate
  `switch-effectiveness-boundary:hook-delivery-ledger` verified all five required
  criteria for exact content state
  `b4976b1c3493af0e295914231b62daa14cf17fb1dfad08b2d18391ed542bf834`.
- Verdict: PASS

## 📋 Hook delivery ledger 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- H1-F1–H1-F3 保持关闭，局部 transaction、rollback failure 和 migration exactness
  保护均未回退。
- H3-F1 通过官方 producer regeneration 关闭，fixture diff 极小且可复现。
- 全仓 L2、相关 race、vet 与独立 fixture targeted test 都绑定同一 fingerprint。
- Admission、route non-regression、privacy allowlist 与 resolver isolation 均有结构和测试
  保护。

### 📝 总结

当前实现 fingerprint
`214b2d3e10fd13cf8f594277f540c605499d640305d23c40dbe5f9a92aa50574`
关闭了所有历史 finding，没有新 blocker。Task 1 的实现、migration、fixtures、L2 与相关
concurrency evidence 已形成一致的最终候选状态，可以 PASS；剩余交付仅是单独授权的
commit/push checkpoint。

## Round 3 — 2026-08-26

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus
  repaired implementation diff fingerprint
  `f112c781b05a85df9bd0a933486f4b56b098de80b5874aab3a0ca2c42c593b43`.
  Final synchronized `tasks.md` blob is
  `fc4faea9936ed1ed08beead18236662909c667af`; `docs/status.md` blob is
  `a16ffcbd913e60a692f9d2afcdcf267d1a67a0d5`.
- Reviewer: Codex, independently re-reviewing the Round 2 repair; this workflow
  turn kept production code, tests, and configuration read-only.
- Method: Formal REREVIEW under `development-workflow`; finding-by-finding
  inspection with CodeGraph and focused source reads, followed by exact-state
  L2 verification. Broad verification stopped when the full Go suite produced a
  decisive change-caused fixture failure; the planned race check was not run.
- Scope: H1-F1 through H1-F3, any newly blocking repair regression, and Task 1's
  required L2 gate.
- Findings:
  - **H1-F1 — CLOSED.** The shared store DSN is restored. Only
    `RecordHookDelivery` pins one `*sql.Conn` and issues `BEGIN IMMEDIATE`, so
    unrelated `BeginTx` callers retain their prior transaction semantics.
  - **H1-F2 — CLOSED.** A trigger-forced route insert failure now proves the
    observation insert rolls back, the route table stays unchanged, and a later
    delivery can acquire the released connection/lock and commit.
  - **H1-F3 — CLOSED.** Migration tests now verify column type/null/default/PK,
    index uniqueness and column order, and complete pre/post `usage_events` row
    equality.
  - **[P1] H3-F1 — schema version 19 leaves canonical desktop fixtures stale and
    fails the required full Go suite.** `scripts/run-go-test.sh ./...` fails
    `internal/desktop.TestCanonicalFixturesAreReproducibleProducerOutput` because
    `desktop/fixtures/v1/snapshot-complete.json` and
    `snapshot-empty-client.json` still contain schema count `18`, while the
    current producer emits `19`. The implementation therefore cannot satisfy
    Task 1's L2 gate in its repaired state.
- Evidence: current source at `internal/store/store.go:228` has no `_txlock`;
  `internal/usage/routes.go:146-223` scopes manual `BEGIN IMMEDIATE` and rollback
  to one pinned connection; the new rollback test is at
  `internal/usage/routes_test.go:377-430`; strengthened migration assertions are
  at `internal/store/store_test.go:722-996`. `go vet -mod=vendor ./...` passed.
  `scripts/run-go-test.sh ./...` failed only on the two named canonical fixtures
  (`have count 18, want 19`); full log:
  `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.XR7u7x`.
- Completion gate: FAILED — CEv1 gate
  `switch-effectiveness-boundary:hook-delivery-ledger` verified H1-F2/H1-F3
  closure and bound H3-F1 to exact content state
  `4678b85618b8b62665f7ed59ebb3c6766295d4caf7d1867227a8e688b56e6758`;
  the L2/scope criterion is disproved.
- Verdict: REOPEN

## 📋 Hook delivery ledger 独立复评

📊 总体评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`desktop/fixtures/v1/snapshot-complete.json:6620`] H3-F1：migration 19 已把
producer 的 schema count 提升为 `19`，但两个 canonical desktop fixtures 仍为 `18`。
- 处置：新增阻断 finding；H1-F1、H1-F2、H1-F3 均已关闭。
- 行为风险：canonical producer output 与仓库 fixture 不一致，全仓 L2 测试失败；任何
依赖这些 fixtures 的 desktop 合同检查都不再代表当前 schema。
- 证据：`TestCanonicalFixturesAreReproducibleProducerOutput` 在
`snapshot-complete.json:6620` 与 `snapshot-empty-client.json:4412` 报告
`have count 18, want 19`。
💡 有界修复：使用测试报告的官方 fixture regeneration 路径只更新这两个 producer
输出，检查差异仅为 schema-19 派生变化，然后重新运行 targeted desktop fixture test 与
`scripts/run-go-test.sh ./...`。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- H1-F1 已通过局部 `*sql.Conn`/`BEGIN IMMEDIATE` 关闭，没有再改变共享 DSN。
- H1-F2 的失败注入同时证明 rollback 和连接释放。
- H1-F3 已覆盖 unique/index columns、完整 column attributes 和完整 v18 event row。
- 修复相关 packages 的开发者测试与本轮 full-suite 中非 fixture 路径均未报告失败，
`go vet` 通过。

### 📝 总结

当前实现 fingerprint
`f112c781b05a85df9bd0a933486f4b56b098de80b5874aab3a0ca2c42c593b43`
关闭了上一轮全部 finding，但 H3-F1 是由 schema-19 直接造成的新 L2 blocker；两个
canonical fixtures 与 producer 同步并让全仓测试通过前，本 Task 仍不能 PASS。Race
证据因决定性 full-suite failure 未运行，应在最终修复状态补上。

## Round 4 — 2026-08-26 (repair)

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus repaired
  implementation diff fingerprint
  `214b2d3e10fd13cf8f594277f540c605499d640305d23c40dbe5f9a92aa50574`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a
  verdict).
- Scope: H3-F1 only, as named in the repair command. H1-F1, H1-F2, and H1-F3
  stay CLOSED from Round 3 and were not re-opened or re-touched.
- Repair:
  - **H3-F1.** Regenerated the two canonical desktop producer outputs through
    the official path the failing test names —
    `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop` — rather than
    hand-editing them. The resulting diff is exactly the schema-19 derived
    change and nothing else: `desktop/fixtures/v1/snapshot-complete.json`
    line 6620 and `desktop/fixtures/v1/snapshot-empty-client.json` line 4412
    each move the `schema` health count from `18` to `19` (`git diff --stat`
    reports 2 files changed, 2 insertions, 2 deletions). No other fixture
    field, no other fixture file, and no production code moved.
  - **Scope record.** Task 1's `Files` line in `tasks.md` now names those two
    producer outputs and states that they are regenerated rather than
    hand-edited, because the schema count they record is derived from the
    migration list this task extends. This keeps the file boundary an accurate
    contract instead of leaving a required derived artifact outside it — the
    same scope-honesty rule H1-F1 was raised under.
- Verification: `go build -mod=vendor ./...`, `go vet -mod=vendor ./...`, and
  `git diff --check` clean. `go test -mod=vendor ./internal/desktop -run
  TestCanonicalFixturesAreReproducibleProducerOutput -count=1` passes.
  `scripts/run-go-test.sh ./...` now passes for the whole repository (exit 0;
  log `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.O0vWFQ`),
  which is Task 1's required L2 gate and the exact check Round 3 recorded as
  failing. The race evidence Round 3 could not reach is now supplied:
  `go test -mod=vendor -race ./internal/usage/... ./internal/store/... -count=1`
  passes (`internal/usage` 62.2s, `internal/store` 18.1s), covering the
  concurrent delivery and route-failure-rollback cases on the pinned-connection
  `BEGIN IMMEDIATE` path.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the Task WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.
