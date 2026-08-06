---
status: active
plan: session-experience
task: session-show-layout
---

# Review log — session-experience / session-show-layout

## 📋 Round 1 评审 — session-experience / session-show-layout

### 📊 总体评分：6/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`internal/activity/activity.go:309`] activity page 的内存边界随用户页码增长

- 行为风险：`ReadDetailsPage` 在非 `--all` 路径设置 `keep = page * limit`，随后以
  `make([]Detail, 0, keep)` 预分配并保留此前所有页面。合法但很大的 `--page` 可在扫描
  前触发巨额分配甚至进程 OOM；普通深分页也保留 `page*limit` 条而不是合同要求的
  requested page，内存不是独立于 session 历史和页码的 bounded reader。
- 证据：实现只防止整数乘法溢出，没有限制 `keep` 的分配规模；现有测试仅覆盖
  page 2 / limit 1，无法暴露深页分配。该问题在进入文件扫描前即可发生。

💡 有界修复：移除由 `page*limit` 直接驱动的预分配，并让 activity source reader 的
保留内存受请求页大小的固定上界约束，同时维持 complete safe summary 和既定
deterministic order。若单次流式扫描无法同时满足排序与固定内存，使用不持久化 raw
activity 的有界方案或明确解决计划约束，不要退化为加载完整 details；补充超大空页和
深页内存边界回归测试。

#### [`cmd/agentdeck/session_show_text.go:87`] compact renderer 没有遵守声明的输出宽度

- 行为风险：renderer 只读取环境变量 `COLUMNS`，未从真实 TTY writer 获取列宽；进入
  compact mode 后又固定给 document text 48 rune、tool 18 rune、model 20 rune，未扣除
  timestamp、kind/status、duration 和分隔符。以现有 `COLUMNS=60` fixture 为例，合法
  document 行必然超过 60 列，activity 行也可溢出，窄终端仍会 wrap dense rows，违反
  responsive layout 合同。
- 证据：document 行由 time + kind + 48-rune text 直接拼接，activity 行同理使用固定
  18/20 caps；`TestRenderSessionShowTextUsesCompactDocumentRowsAtNarrowWidth` 只检查
  table header 消失和出现省略号，没有断言每行 visible width。当前也没有 wide-width
  fixture，且 rune count 不能代表 CJK/emoji 的终端 cell width。

💡 有界修复：把实际 writer TTY width（保留 `COLUMNS` 可测试 override）传给 renderer，
按终端 visible-cell width 计算各字段预算；窄屏把次要值移入 detail lines，确保每行不
超过目标宽度。补充 48/60/80 及 wide fixture 的逐行宽度断言，并至少覆盖 CJK/emoji；
不要改变 JSON shape 或引入 interactive viewer/TUI 范围。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- `ShowMetadata` 与 `DocumentsPage` 将 approved documents 改为 storage-level
  `COUNT`、`LIMIT`/`OFFSET`，保持 selected authoritative source 和 rowid order。
- Activity reader继续只生成 allowlisted safe metadata，完整 summary 与 requested detail
  page 分离，未持久化 raw activity、arguments、outputs 或 source payload。
- Section renderer保留 SESSION metadata，Documents/Activity/Token sections 及各自紧邻的
  pagination footer，并显式显示 documents/activity empty 和 source-unavailable/stale 状态。
- JSON 现有字段保持，新增 stale/activity warning 使用 additive optional fields。

### 📝 摘要

- Reviewed content identity：HEAD
  `dede1c185ea4eef44390c0441e73f73beed60937`；scoped candidate fingerprint
  `99ef16e378cfb6ab3846bcfbdd303d04a91667c9226471d5e1e31a6402d5ceb8`。
- Verdict rationale：bounded document query、safe activity summary 和 section wiring 方向
  正确，但 activity 深分页仍可无界分配，compact renderer 也不满足实际 width 合同，
  不能标记 Review PASS。
- Evidence：源码、调用路径和现有 tests 检查。发现决定性阻断项后未运行受影响包或
  完整 L2。
- Residual uncertainty：修复后需验证深页 memory boundary、TTY/COLUMNS width routing、
  narrow/wide/CJK visible width、empty/stale/partial states，并运行计划要求的 L2。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 保持 `3/6 done`。

## 📋 Round 2 复评 — session-experience / session-show-layout

### 📊 总体评分：7/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`internal/activity/activity.go:308`] 深页边界校验仍未形成通过的回归保护

- 处置：仍未关闭（部分修复）。`maxPageCandidates = 1000` 已将 `page * limit`
  驱动的候选集限制为固定上界，原先的巨额预分配/OOM 路径已被约束；但边界校验位于
  `os.Open` 之后，与新增测试声明的“打开来源前拒绝”合同不一致。
- 行为风险：非法深页请求的结果仍受来源文件是否存在或可读影响，输入边界不能在任何
  文件访问前稳定拒绝；更直接地，任务新增的回归测试当前失败，候选内容不能通过评审。
- 证据：`internal/activity/activity.go:308-319` 先打开文件，再检查
  `page > maxPageCandidates/limit`；
  `TestReadDetailsPageRejectsDeepPageBeforeOpeningSource` 使用不存在的来源验证拒绝顺序。
  针对性命令
  `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/activity -run '^TestReadDetailsPageRejectsDeepPageBeforeOpeningSource$' -count=1 -v`
  退出码为 `1`。

💡 有界修复：在任何 `os.Open` 之前完成非 `--all` 深页窗口校验，保持固定候选上界；
让现有针对性测试通过后，再运行该包测试和计划要求的最终 L2 验证。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- 上一轮 compact width 阻断已关闭：`sessionShowTextWidth` 读取真实 TTY 宽度并保留
  `COLUMNS` 测试 override；compact document/activity 行均按 visible-cell budget 截断。
- `TestRenderSessionShowText...` 针对性测试覆盖 48/60/80、wide layout、CJK/emoji；
  `go test -mod=vendor ./cmd/agentdeck -run '^TestRenderSessionShowText' -count=1`
  通过。
- Activity 候选保留量现在受 1000 行固定上界限制，上一轮指出的直接超大预分配风险
  已实质收敛。

### 📝 摘要

- 逐项处置：activity 内存边界发现部分修复但回归测试仍失败，保持 open；compact
  renderer width finding 已关闭；未发现新的独立阻断项。
- Reviewed content identity：HEAD
  `dede1c185ea4eef44390c0441e73f73beed60937`；scoped candidate fingerprint
  `7319d347259756915be52c4683f7d1825f64bd0f025ada106b8c8a059cb34831`。
- Verdict rationale：一个必需的 activity 边界回归测试仍以退出码 1 失败，因此保持
  `FAIL`，不得勾选 Review。
- Residual uncertainty：根据“决定性阻断后停止广泛验证”的项目规则，本轮未运行完整
  activity 包测试或 L2；修复到最终内容状态后需要补齐。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 保持 `3/6 done`。

### 🛠 修复指令

```text
根据评审修改：session-experience / session-show-layout
```

仅调整 `ReadDetailsPage` 的深页窗口校验顺序，使其在任何来源文件访问前稳定拒绝；
保持 1000 行固定候选上界及现有排序、summary、privacy 合同。先运行失败的单测，随后
运行 activity 包测试和计划要求的最终 L2；不要改动已通过的 renderer width 行为。

## 📋 Round 3 复评 — session-experience / session-show-layout

### 📊 总体评分：10/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 2 唯一未关闭项已关闭：`ReadDetailsPage` 在任何 `os.Open` 前校验非
  `--all` 深页窗口，非法页不会再受来源存在性或可读性影响。
- `maxPageCandidates = 1000` 继续提供固定候选上界，并保持既定排序、完整 safe
  summary、privacy allowlist 与 `--all` 行为。
- Round 1 的 renderer width finding 保持关闭：真实 TTY/COLUMNS 路由、
  48/60/80、wide layout、CJK/emoji visible-cell 证据均未被本次 activity 局部修复
  失效。

### 📝 摘要

- 逐项处置：bounded activity page finding 已关闭；responsive visible width
  finding 保持关闭；没有回归或新增阻断项。
- Reviewed content identity：HEAD
  `dede1c185ea4eef44390c0441e73f73beed60937`；scoped candidate fingerprint
  `1e8247a1f58176774053d5b084f46d74bdfdb0568191bfe4158b864cd4949a1c`。
- Evidence：本轮独立运行
  `go test -mod=vendor ./internal/activity -run '^TestReadDetailsPageRejectsDeepPageBeforeOpeningSource$' -count=1`
  通过；修复阶段同一最终内容状态的 activity 包测试、
  `go test -mod=vendor ./...` 与 `git diff --check` 继续有效；Round 2 的
  `TestRenderSessionShowText...` targeted evidence 继续有效。
- completion-evidence/v1：Neo4j Cypher provider 对 Task
  `urn:ce:agent-deck:task:session-experience:session-show-layout` 和 target state
  `urn:ce:agent-deck:state:candidate:1e8247a1f58176774053d5b084f46d74bdfdb0568191bfe4158b864cd4949a1c`
  返回 `VERIFIED`，2/2 required criteria 通过。
- Verdict rationale：此前所有 blocking findings 均在新内容状态关闭，targeted、L2
  与 CEv1 gate 一致，结论为 `PASS`。
- Residual uncertainty：无已知 task-scope 阻断；候选仍未提交，提交后需按项目规则
  将 verified candidate 关联到不可变 Git tree，无需因提交重跑未变化的产品验证。
- 状态同步：Dev `[x]`、Review `[x]`；`docs/README.md` 更新为 `4/6 done`。
