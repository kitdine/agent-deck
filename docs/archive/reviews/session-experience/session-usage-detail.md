---
status: active
plan: session-experience
task: session-usage-detail
---

# Review log — session-experience / session-usage-detail

## 📋 Round 1 评审 — session-experience / session-usage-detail

### 📊 总体评分：6/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`internal/usage/session_usage.go:76`] invocation page 在存储层仍然无界

- 行为风险：`SessionInvocations` 先调用 `s.events(ctx, client, sessionID)` 将该
  logical session 的全部 usage events 载入内存，然后才按 `page`/`limit` 切片。
  长 session 的延迟和内存占用仍随全部历史事件增长，违反本计划要求的 bounded
  invocation count/page 以及 SQL `COUNT`、`LIMIT`/`OFFSET` 合同。
- 证据：当前实现的分页计算发生在完整 `events` slice 返回之后；受影响包测试通过，
  但测试只使用两个事件，没有证明 off-page rows 未被 materialize。

💡 有界修复：通过复用现有 authoritative event filters 增加 deterministic count/page
查询，在存储层按 UTC `event_at` 和 stable event key 执行 `COUNT`、`LIMIT`/`OFFSET`；
只对请求页事件定价，并以 offset 计算一基 `Sequence`。不要改变 usage rows、parser 或
pricing rules；补充能够证明页外事件未被读取/定价的回归测试。

#### [`cmd/agentdeck/main.go:1971`] 默认 JSON `--tokens` 输出了未请求的 pagination 字段

- 行为风险：无显式 `--page`/`--limit`/`--all` 时，代码返回 `sessionShowPage`，但其
  `Pagination` 字段仍使用 `json:"pagination"`。因此完整 JSON 会包含
  `"pagination": null`，与“默认 JSON 保持完整且只有显式 paging 才添加
  `pagination.invocations`”的计划合同不一致。
- 证据：`sessionShowPage.Pagination` 没有 `omitempty`；本次候选反而把
  `omitempty` 加到了始终携带 pagination map 的 `sessionListPage`。现有 complete JSON
  测试只反序列化 invocations 数量，没有检查原始 envelope 是否缺少 pagination。

💡 有界修复：将 `omitempty` 应用于 `sessionShowPage.Pagination`，撤销无必要的
`sessionListPage` tag 变更；增加 raw JSON 回归断言，证明默认 `--tokens` 不含
`pagination`，显式分页则包含 `pagination.invocations`。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- `SessionInvocation` DTO 明确排除了 event key、source path、source offset、run ID 和
  credential metadata；现有测试也检查了私密 event/source 字符串不会进入 JSON。
- 新 read path 复用 stored deltas 和既有 event-time price resolver，没有修改 usage
  schema、ingestion parser 或 pricing rules。
- CLI 已覆盖 summary、单页/完整 JSON、文本 pagination footer、`--tokens` next command
  以及 Claude cache token components。

### 📝 摘要

- Reviewed content identity：HEAD
  `058f3277142a21d4c7fa760d45091a9c28e01f0f`；scoped candidate fingerprint
  `cf004778501c66900f0215b283a178f2a95fea7e56320254f8386e789cf60378`。
- Verdict rationale：核心 DTO、pricing reuse 和 privacy boundary 方向正确，但 storage
  pagination 仍无界且默认 JSON pagination 合同不符，不能标记 Review PASS。
- Evidence：
  `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck ./internal/usage`
  PASS。发现决定性阻断项后未运行完整 L2/L3。
- Residual uncertainty：修复最终状态仍需重新审查 authoritative filtered count/page
  query，并运行计划要求的 L3 证据。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 保持 `2/6 done`。

## 📋 Round 2 复评 — session-experience / session-usage-detail

### 📊 总体评分：5/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`internal/usage/session_usage.go:153`] bounded invocation page 行为已修复但回归保护仍未关闭

- Disposition：still open（生产实现部分已关闭）。
- 行为风险：新实现已改为独立 `COUNT(*)` 与按 `event_at,event_key` 排序的
  `LIMIT`/`OFFSET` 查询，只 materialize 请求页；但新增测试仍只验证两个事件的返回值，
  没有证明页外事件不会被读取或定价。该 storage-boundary 可在未来回退为全量读取而
  现有测试继续通过。
- 证据：`sessionInvocationCount` 与 `sessionInvocationEventsPage` 已替代原先完整
  `s.events(...)` 读取；CodeGraph 对两个新 helper 均未找到直接保护其 query boundary
  的测试，`session_usage_test.go` 只断言 page 1/page 2 的结果值。

💡 有界修复：保留当前 `COUNT`、`LIMIT`/`OFFSET` 实现，补充能够证明 off-page rows
未被 materialize/price-resolved 的测试，不修改 usage rows、parser 或 pricing rules。

#### [`cmd/agentdeck/main.go:2168`] 默认 JSON pagination 合同仍未关闭

- Disposition：still open（表现从 `pagination:null` 变为主动返回 pagination map）。
- 行为风险：无显式 `--page`/`--limit`/`--all` 时，候选现在主动返回
  `Pagination: {"invocations": ...}`。计划明确规定默认 JSON 保持完整现有 session
  JSON，只有显式 paging 才添加 `pagination.invocations`；当前修复把非合同字段写进
  默认 envelope，并让测试固化了错误行为。
- 证据：无显式 paging 分支在 `sessionShowPage` 中直接设置 `Pagination`；complete JSON
  测试也开始要求该 map，而不是断言原始 JSON 不含 `pagination`。

💡 有界修复：默认 complete JSON 返回 usage/invocations 但省略 `pagination`；仅显式
paging 分支添加 `pagination.invocations`。将 `omitempty` 放在
`sessionShowPage.Pagination`，撤销无必要的 `sessionListPage` tag 变更，并用 raw JSON
分别断言默认缺失、显式分页存在。

#### [`cmd/agentdeck/main.go:2006`] `show --tokens` 提前释放 state lock，跨库读取失去一致性

- Disposition：new。
- 行为风险：命令先打开 sessions DB，随后为调用 `opts.openStore` 提前释放外层 state
  lock。`store.Open` 只在打开和 migration 期间短暂取锁，返回后不保留锁；因此 watcher
  或 scan 可在 session metadata、usage count 和 usage page 读取之间改写状态，产生同一
  response 内 total/page/session identity 不一致。
- 证据：`withSessions` 在 `showTokens` 分支调用 `lock.Release()`，再调用
  `opts.openStore()`；`store.Open` 的内部 lock 在 `open` 返回前释放，`Store` 本身不保存
  state lock。项目已有 `store.OpenWithLockHeld` 专门支持同一复合操作在已有锁内打开
  core database。

💡 有界修复：保持 `withSessions` 已取得的 state lock，在该锁内通过
`store.OpenWithLockHeld` 打开 core store，并覆盖 success/error 路径只释放一次以及
并发 writer 不能穿过复合读取的回归测试。不要扩大到 watcher 或 store lock 重构。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 1 的 storage behavior 已从全量 `s.events` 切换到 deterministic
  `COUNT`、`LIMIT`/`OFFSET`，并正确用 offset 计算一基 `Sequence`。
- 新 page query 复用了原 authoritative event query 的 client/session filters、ordering
  和 pricing input columns，未改变 usage schema 或 pricing rules。
- DTO privacy boundary 继续排除 event key、source metadata、run ID 和 credential
  metadata。

### 📝 摘要

- Finding disposition：Round 1 bounded-page finding 部分关闭但缺 regression gate；
  Round 1 JSON finding 仍 open；新增一个 cross-database state-lock finding。
- Reviewed content identity：HEAD
  `058f3277142a21d4c7fa760d45091a9c28e01f0f`；scoped candidate fingerprint
  `3ebb48bf3176f81e318c8033c30ad7eddfabf66869cfe21ba66500b3eb4e38f4`。
- Verdict rationale：storage query 方向正确，但 JSON 合同、bounded-query test protection
  与复合读取 lock boundary 仍有阻断项，不能标记 Review PASS。
- Evidence：源码/调用路径复评。Round 1 targeted test evidence 已被实现和测试改动失效；
  发现决定性阻断项后未运行完整 L2/L3。
- Residual uncertainty：修复后需验证 state-lock success/error lifecycle、off-page query
  boundary、默认/显式 JSON raw envelope，并运行计划要求的 L3。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 保持 `2/6 done`。

### 🛠 修复指令

仅完成三项：为当前 bounded SQL page 增加 off-page 未读取/未定价测试；让默认 JSON
省略 pagination、显式 paging 才返回 `pagination.invocations`；在既有 state lock 内用
`store.OpenWithLockHeld` 打开 core store并覆盖 lock lifecycle。不要修改 schema、parser、
pricing rules、watcher 或其他 session task。

## 📋 Round 3 复评 — session-experience / session-usage-detail

### 📊 总体评分：9/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

#### [`cmd/agentdeck/main.go:1958`] `sessionListPage` 的 `omitempty` 不属于本 task 必需变更

- Disposition：new，non-blocking。
- 证据：`sessionListPage` 在实际构造路径始终携带非空 pagination map，因此该 tag
  变更当前没有可观察行为，也不是 `session show --tokens` 默认 JSON 修复所需。

💡 有界改进：提交前可撤销该单一 tag 改动以保持最小 diff；不影响本轮 PASS。

### 🟢 优点

- Round 2 bounded-query test finding 已关闭：off-page row 使用无法解码的 sentinel，
  page 1 成功证明页外 row 未被 materialize 或进入 pricing path。
- Round 2 JSON finding 已关闭：默认 complete JSON 返回完整 invocations 且 raw envelope
  不含 `pagination`；显式 paging 仍返回 `pagination.invocations`。
- Round 2 state-lock finding 已关闭：`show --tokens` 在外层 state lock 内通过
  `store.OpenWithLockHeld` 打开 core store；连续 success/error/后续命令路径证明 lock
  最终释放，既有 lock primitive 保证 writer exclusion。
- DTO privacy boundary、stored-delta normalization、event-time pricing 和 deterministic
  sequence/pagination 均保持既定合同。

### 📝 摘要

- Finding disposition：Round 2 三项 blocking finding 全部 closed；新增一项 non-blocking
  最小 diff 建议，无 regressed 或 new blocking finding。
- Reviewed content identity：HEAD
  `058f3277142a21d4c7fa760d45091a9c28e01f0f`；scoped candidate fingerprint
  `db83a97039007b0bc8c598b961acc494894986e683d38215037f880646346a96`。
- Verification：聚焦 off-page 与 CLI JSON/lock regression tests PASS；
  `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck ./internal/usage`
  PASS；`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` PASS；
  `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./cmd/agentdeck ./internal/usage`
  PASS；`git diff --check` PASS。
- Verdict rationale：bounded storage page、默认/显式 JSON、复合 state lock、privacy 和
  pricing attribution 均有同一候选状态的实现及 L3 证据，无 unresolved blocking finding。
- Residual uncertainty：未运行 L4 `release-verify`，因为本 task 不涉及 release artifact；
  `sessionListPage` tag 是可选的最小 diff 清理。
- 状态同步：Dev `[x]`、Review `[x]`；`docs/README.md` 更新为 `3/6 done`。
