---
status: active
plan: session-experience
task: session-document-time
---

# Review log — session-experience / session-document-time

## Round 1 — 2026-08-06

- Reviewed state: `ae762b3` plus working-tree diff SHA-256
  `5a419c826096abd12646645c86062522f1a9a539fe0a6cfcd1fd9361a202db97`.
- Reviewer: Codex (review-only round; product code, tests, and configuration
  remained read-only).
- Scope: document `event_at` ingestion and normalization, rebuildable FTS schema
  upgrade, search ordering and client filtering, text timezone rendering,
  search paging/next-command behavior, JSON compatibility, manual update, and
  task verification coverage.

### Findings

**P1 — explicit search pagination publishes a different JSON namespace from the
approved task contract.** The plan requires top-level `pagination.search`
(`docs/plans/session-experience.md:102`), while the implementation constructs
`pagination.documents` (`cmd/agentdeck/main.go:2043`) and the manual documents
that different shape (`docs/specs/cli-manual.md:635`). This is a stable JSON
contract, not renderer wording; shipping either name while the plan promises the
other leaves the `v0.5.0` desktop consumer and CLI automation with no single
authority. Bounded remediation: implement `pagination.search` as approved, or
return to design and explicitly approve one replacement contract before changing
code and manual together; add an exact JSON fixture.

**P1 — equal-time matches do not preserve selected-source document order.** The
approved ordering ends with source document order after time, client, and session
ID. `internal/session/session.go:944` inserts `d.kind` before `d.rowid`, so two
documents in the same session with the same timestamp sort alphabetically by kind
(`assistant_final` before `user_prompt`) instead of their source order. The new
test covers different timestamps only and cannot detect this. Bounded remediation:
remove `d.kind` from the tie-break chain and add a same-session, same-instant
fixture whose kinds would fail under alphabetical ordering.

**P2 — missing or malformed document time renders an empty cell instead of the
specified em dash.** The task contract says empty `event_at` renders `—`.
`renderSessionDocuments` passes the empty value to `renderDisplayTime`
(`cmd/agentdeck/main.go:3804-3808`); the shared renderer deliberately returns an
unparseable string unchanged, so empty remains empty. The parser test includes a
malformed time but no command-layer renderer assertion. Bounded remediation: map
empty event time to `—` only at the session document text boundary and cover it
without changing the global unparseable-time rule.

**P2 — the new CLI paging and rendering contracts have no command-layer
regression coverage.** No `cmd/agentdeck` test change pins text default page 20,
`--page`/`--limit`/`--all` validation and mutual exclusion, client-filter-before-
pagination behavior, exact JSON envelope shape, quoted next command, local-zone
header/value rendering, or empty-time display. The affected package suite passes,
which demonstrates that the existing suite does not protect the newly added
surface. Bounded remediation: add focused command/renderer tests for those
contracts, including JSON without explicit paging remaining the legacy complete
array.

### Evidence

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/session ./internal/store ./cmd/agentdeck` — PASS.
- `git diff --check` — PASS.
- Focused source inspection confirmed the plan/implementation/manual namespace
  mismatch, SQL tie-break order, empty-time renderer behavior, and absence of
  changed command-layer tests.
- Broader `go test ./...` was not run after decisive blocking findings, per the
  project review policy.

- Verdict: REOPEN

## Round 2 — 2026-08-06

- Reviewed state: `ae762b3` plus working-tree product blobs
  `b2d079f8`, `bcd9ede6`, `daec273c`, `076f8bfc`, `194dee97`,
  `89382166`, and `88e81e5d` for the seven task files listed in Scope.
- Reviewer: Codex (re-review only; product code, tests, and configuration
  remained read-only).
- Scope: `cmd/agentdeck/main.go`,
  `cmd/agentdeck/session_command_test.go`, `docs/specs/cli-manual.md`,
  `internal/session/session.go`, `internal/session/session_test.go`,
  `internal/store/store.go`, and `internal/store/store_test.go`.

### Prior finding dispositions

- **CLOSED — P1 search pagination namespace.** Explicit search paging now
  publishes `pagination.search`; the manual agrees, and the command test rejects
  a residual `pagination.documents` namespace while preserving the legacy
  complete JSON array when paging is not explicit.
- **CLOSED — P1 equal-time source order.** Search ordering now ends with
  `d.rowid` after time, client, and session ID, without the former `d.kind`
  tie-break. `TestSearchPreservesSourceOrderForEqualEventAt` covers two
  same-session documents with the same instant.
- **CLOSED — P2 empty or malformed event time rendering.**
  `renderSessionDocumentTime` maps values that do not parse as RFC 3339 to `—`,
  and the renderer regression test covers both empty and malformed values.
- **STILL OPEN — P2 command-layer paging and timezone coverage remains
  incomplete.** `TestSessionSearchCommandPaginationContracts` covers legacy
  JSON compatibility, explicit JSON paging, client filtering before pagination,
  and invalid flag combinations; the next-command test covers shell quoting;
  the renderer test covers the time-column header and invalid-time placeholder.
  No command-layer test exercises text search's default 20-row page/footer, and
  no test fixes the local timezone and asserts that a valid UTC `event_at` is
  rendered as the corresponding local-zone value with the matching zone header.
  These were explicit Round 1 regression requirements and remain exposed to
  renderer or command wiring regressions. Bounded remediation: add one text
  search command test for default pagination/footer and one deterministic
  timezone renderer test for a valid instant and zone header/value; do not
  broaden production behavior.

### New or regressed findings

- None beyond the still-open coverage finding above.

### Evidence

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
  ./cmd/agentdeck ./internal/session` — PASS.
- Focused source and test inspection confirmed the first three findings closed
  and the remaining two command-layer assertions absent.
- L2 `go test -mod=vendor ./...` was not run after the decisive still-open P2
  finding, per project review policy.
- Plan status remains `[ ]` Dev and `[ ]` Review; no `docs/README.md` status
  synchronization was needed.
- Verdict: REOPEN

## 📋 Round 3 复评 — session-experience / session-document-time

### 📊 总体评分：7/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`cmd/agentdeck/e2e_test.go:476`] GUI JSON contract golden 未同步新增的 `event_at`

- 处置：new。
- 行为风险：`TestIsolatedEndToEndFlow` 失败，当前权威 GUI/automation
  contract fixture 仍描述旧的 `session.search` 和 `session.show.documents`
  schema，无法为本任务的 additive JSON 字段提供 L2 回归证据。
- 证据：限定复现输出的 actual contracts 在 `session.search` 和
  `session.show.documents` 中包含 `event_at`，但
  `cmd/agentdeck/testdata/gui-json-contract.json` 与之不一致。

💡 有界修复：通过现有 `UPDATE_AGENTDECK_GOLDEN=1` 路径重新生成 contract
fixture，只接受并检查本任务导致的 session `event_at` schema 差异；不得顺带接受
其他 contract 漂移。

#### [`cmd/agentdeck/reporting_clock_test.go:224`] 旧测试仍禁止无时间记录的表头显示时区

- 处置：new。
- 行为风险：该断言与已批准的文本合同冲突：`EVENT AT (<zone>)` 是列语义，缺失
  `event_at` 只应让单元格显示 `—`。冲突使 `cmd/agentdeck` 测试失败，也让本任务的
  本地时区展示存在两个互斥的测试权威。
- 证据：`TestSessionSearchTextHasNoInstantToLocalize` 在第 242 行报告
  `search text named zone without an instant`；实际输出为 `EVENT AT (UTC+8)` 且
  记录单元格为 `—`，与计划合同和新增确定性测试一致。

💡 有界修复：调整该测试的名称和断言，使其验证无时间记录仍显示 `—` 且表头保留
固定时区；不要改变生产渲染行为。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 1 的 pagination namespace、equal-time source order、空/异常时间占位符
  三项 finding 继续保持 CLOSED。
- Round 2 唯一未关闭项现已 CLOSED：命令层测试断言默认文本页只显示 20/25 条、
  `Showing 1-20 of 25` footer 和 next command；固定 `UTC+8` 的 renderer 测试断言
  `EVENT AT (UTC+8)` 与 `2026-08-07 00:00:00`。
- 新增覆盖使用既有 `displayLocation` seam，没有扩大生产行为或依赖范围。

### 📝 摘要

- Finding disposition：Round 2 的 remaining P2 coverage finding 已关闭；新增两个
  L2 verification blocker，未发现新的产品行为回归。
- Reviewed content identity：HEAD `ae762b3`，产品 blob 为 `b2d079f8`,
  `a91dd44d`, `daec273c`, `076f8bfc`, `194dee97`, `89382166`, `88e81e5d`。
- Evidence：`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
  ./cmd/agentdeck ./internal/session` — FAIL；限定复现同样确认上述两个失败。
- Residual uncertainty：原 L2 链使用 `&&`，受影响包失败后
  `go test -mod=vendor ./...` 未执行；修复两个测试权威后必须重新运行完整 L2。
- Verdict rationale：所有已记录行为 finding 均已关闭，但必需的 L2 验证为红，
  因此不能标记 Review PASS。计划状态继续保持 Dev `[ ]`、Review `[ ]`，
  `docs/README.md` 继续保持 `0/6 done`。

### 🛠 修复指令

仅同步 `cmd/agentdeck/testdata/gui-json-contract.json` 中本任务新增的 session
`event_at` contract，并调整 `TestSessionSearchTextHasNoInstantToLocalize` 以接受带时区
表头和 `—` 单元格。随后运行 `./cmd/agentdeck ./internal/session`、完整 L2
`./...` 和 `git diff --check`；不要修改生产行为。

## 📋 Round 4 复评 — session-experience / session-document-time

### 📊 总体评分：10/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- **CLOSED — Round 3 GUI JSON contract golden。** 实际权威 fixture
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json` 只在
  `session.search` 和 `session.show.documents` schema 中各增加一个
  `event_at: string`，没有接受无关 contract 漂移。
- **CLOSED — Round 3 无时间记录的时区表头测试。** 测试已重命名为
  `TestSessionSearchTextShowsZoneHeaderAndDashWithoutInstant`，并同时断言
  `EVENT AT (UTC+8)`、`—` 和 approved visible text，不再与批准合同冲突。
- Round 1 的 pagination namespace、equal-time source order、空/异常时间占位符
  findings 和 Round 2 的命令层分页/本地时区覆盖 finding 均继续保持 CLOSED。
- 受影响包与全仓 L2 测试均通过，没有发现新 blocking、regressed finding。

### 📝 摘要

- Finding disposition：Round 3 的两个 new blocker 均 CLOSED；全部历史 finding
  已逐项关闭，无 unresolved finding。
- Reviewed content identity：HEAD `ae762b3`，task 相关 blob 为 `b2d079f8`,
  `a91dd44d`, `12b5fffb`, `f8258112`, `daec273c`, `076f8bfc`, `194dee97`,
  `89382166`, `88e81e5d`。
- Evidence：`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
  ./cmd/agentdeck ./internal/session` — PASS；`GOCACHE=/private/tmp/agent-deck-go-build
  go test -mod=vendor ./...` — PASS；`git diff --check` — PASS。
- Residual uncertainty：无超出本 task L2 范围的未验证风险；release-level RC 和
  `make release-verify` 仍由后续 `v0.4.0` release plan 负责。
- Verdict rationale：合同、实现、回归覆盖和 L2 验证一致，因此 Review PASS。
  状态同步为 Dev `[x]`、Review `[x]`，`docs/README.md` 为 `1/6 done`。
