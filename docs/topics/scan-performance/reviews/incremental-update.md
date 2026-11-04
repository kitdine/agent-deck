---
status: active
topic: scan-performance
subject: incremental-update
---

# incremental-update 评审记录

## Round 1 — 2026-10-28

### 📋 增量更新实现评审

📊 综合评分：8/10

✅ 评审结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；未参与本任务实现，独立阅读当前实现与交接。
- Method：单评审者源码与调用路径检查、精确状态证据复用、隔离时钟边界实验；无委派。
- Scope：`internal/scanruntime/scanruntime.go`、`serving.go`、`serving_test.go`，以及 serving 读取、发布、等待与覆盖率契约。
- Reviewed state：HEAD `2098313634584dbe73a5f09b739176f85f931f29`；候选指纹 `e632cd5356dc8d6ac7b69d73f34eb829bf63985e2a2f4cbc4e1aacc3e01e29ab`。
- WorkUnit：`scan-performance:incremental-update`。
- ContentState：`scan-perf-iu-impl-20261028-state`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无已确认的产品正确性缺陷。

### 🟡 改进问题 — 必须关闭后通过

#### IU-R1-F1 — P2：新增回归测试在本地午夜附近确定性失败

位置：`internal/scanruntime/serving_test.go:216-217`、`:292-293`。

- 状态：OPEN；本任务内修复。
- 行为风险：测试把 `time.Now()` 当作固定日内时间，种子记录却写在两分钟前；
  本地午夜后的前两分钟，该记录属于前一天，固定的今天 hero 期望值仍包含其 28 tokens。
  正确实现因此会使本测试及包含它的 Go 套件失败，结果依赖执行时刻。
- 证据：只通过 Go overlay 把本测试的 `now := time.Now()` 替换为
  `time.Date(2026, time.October, 28, 0, 1, 30, 0, time.Local)`，保持产品代码、夹具操作与断言不变。
  `scripts/run-go-test.sh -overlay=<隔离 overlay.json> ./internal/scanruntime -run '^TestServingSettledUpdatePlansOncePublishesFullAndMatchesFreshOracle$'`
  返回 1：append 得到 41、期望 69；late-previous-day 得到 0、期望 28；remove 得到 0、期望 28；rewrite 通过。
  四个场景均先通过前面的 fresh-oracle payload 比较，排除了增量路径与 oracle 分歧这一解释。
- 对照：唯一变化是把注入时间的小时从 0 改为 12，同一命令的四个场景全部通过。
  两次均使用临时 HOME/state root 与合成记录，没有访问真实日志或产品数据库。
- 根因分类：测试夹具缺陷，高置信度；不是已证实的产品跨天计算缺陷。
- 💡 修复：给该测试选择明确、固定的本地日内时刻，并确保种子和追加记录属于断言期望的日期；
  如覆盖午夜，则单独定义相应日期归属和期望值。保留完整 oracle 比较与 hero 断言，不用重试、跳过或放宽断言规避。

### 🟢 优点

- 已有可读 full publication 时复用单次 planning/发布路径，避免无效的优先批次投影；
  缓存缺失、跨日或身份失效仍回到原有分批路径。
- `ReadServingSnapshot` 检查身份、epoch、schema、权限、校验和及本地日窗口；
  revision 不同会标记 stale，而不是抹掉旧 publication。
- 新测试比较 append、late、rewrite、remove 后的 usage/session payload 与 fresh oracle，
  并检查新 publication ID 和完整 coverage；这些检查在午夜实验中也通过。
- 正式 P10 对比记录显示更新延迟下降，并如实保留事务尾部分位数变差及基线失败样本。

### 📝 总结

本轮因 IU-R1-F1 未关闭而 FAIL；不要求修改已经验证等价的产品计算。
产品验收门禁与测试质量评审是不同结论：既有四项产品准则的精确状态门禁仍为 VERIFIED，
午夜实验没有推翻其产品等价性，不能把错误断言当作产品准则失败，也不能用该门禁覆盖本轮测试缺陷。

#### 内容身份与证据

按 ContentState 原配方复算：取 HEAD 差异及未跟踪路径，排除 `docs/` 与 `apps/macos/.build/`，
按字节排序，以 `path:<git hash-object>` 拼接（LF 分隔、无末尾 LF）后 SHA-256。
结果与登记指纹一致，三个 blob 为：

- `internal/scanruntime/scanruntime.go`：`8a8514cd621031fa1a952e529c7d66feef9e71ac`
- `internal/scanruntime/serving.go`：`4d7600663fc533cb4444280f648e42930038e9f0`
- `internal/scanruntime/serving_test.go`：`de9334e62116f841c82a91eb0a965689737179d3`

通过标准 `gate-status.cypher` 查询该 WorkUnit/ContentState：4 项 required criteria 齐全，
`missing_criteria`、`invalidated_evidence`、`unresolved_candidate_impacts` 均为空。
复用 `scan-perf-iu-impl-20261028-` 前缀的 `partition-equivalence`、`nonadditive-statistics`、
`generation-switch`、`update-latency` 四条适用证据，不重复全量测试与性能批次。
开发交接为 Beads comment `565fd599-6af1-5857-bc8c-2eea3fe5dfba`；
其中 Go 全量、race、vet、双架构 Release 与 native 测试属于开发阶段记录，不冒充本轮独立执行结果。
native 首轮有一个焦点依赖失败，154 项重跑通过；本轮没有重新验证该环境现象。

P10 正式 ABBA，每臂 4 × 25 次写入：S0 median 5129.1 ms、p95 5485.4 ms；
候选 median 2310.0 ms、p95 2535.8 ms。median 比值 0.450，95% 区间 [0.446, 0.456]；
p95 比值 0.462。S0 有 1 次 helper 失败，候选 0 次；tool_invalid 与 interrupted_reruns 均为 0。
p99 比值 0.460，区间上界为 +Inf，不能描述为有有限置信上界的改善。

事务数 26047 → 9600，最大事务 244.5 → 105.4 ms；p95 7.2 → 25.8 ms，
p99 33.2 → 58.1 ms，session_init 总时长 25.8 → 33.5 s。
取舍：重复投影与事务总数下降、端到端更新更快，但单事务尾部分布并未全面改善；
按当前无数值性能门的契约记录这些代价，不将它们单独判为失败。

独立新增验证仅为上述午夜/中午对照；overlay 是诊断输入，不是工作树修改。
原产品及测试文件保持不变。发现确定性阻断后停止扩大验证范围。
`python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/incremental-update.md`、
`make check-whitespace` 与 `git diff --check` 均通过。

### 下一步指令

修复：scan-performance / incremental-update / IU-R1-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-28（Round 1 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围仅 IU-R1-F1。Round 1 的评分与 FAIL 结论保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- IU-R1-F1 -> repaired in candidate：
  - **复现：** 用 Go overlay 把该测试的时钟改为本地 `00:01`（不改工作树）重跑，
    得到 append 41/期望 69、late-previous-day 0/期望 28、remove 0/期望 28、rewrite 通过，与 Round 1 证据一致；
    根因确认为测试夹具把 `time.Now()` 当作固定日内时刻。
  - **修复：** `TestServingSettledUpdatePlansOncePublishesFullAndMatchesFreshOracle` 的时钟固定为本地
    `2026-10-20 12:00`。种子记录（前两分钟）、append（前一分钟）、rewrite 与当天 hero 期望同属该日，
    late-previous-day 仍落在前一天；fresh-oracle 全量比较、批次数、新 publication 与 hero 断言均保持不变，
    无重试、跳过或放宽断言。产品代码未改。
  - **回归保护：** 固定日期比实际运行日早 8 天，四个场景全部通过，说明 seed、更新 round、oracle 与读取
    都只经注入时钟取日期，结果不依赖墙上时间；同一 overlay 把固定时刻改为 `00:01` 时 hero 断言仍按上述数值失败，
    断言保持敏感。

受影响证据处理：

- **内容状态：** 只有 `internal/scanruntime/serving_test.go` 变为 `d64a756dec4e3861ac91d68b3b0b6df65767cedd`，
  两个产品 blob 不变；HEAD 仍为 `2098313634584dbe73a5f09b739176f85f931f29`。按既有配方复算 scoped fingerprint
  `b0db6048fb823822740089cc45d9cdb7740c1eda6722551015aacda287cbf67e`；CEv1 ContentState `scan-perf-iu-r1f-20261028-state`。
- **CEv1：** 四项准则在新状态下各记一条 `scan-perf-iu-r1f-20261028-*` 证据并 `rolls_up` 到对应
  `scan-perf-iu-impl-20261028-*`。三项正确性准则以本次重跑的回归与包测试直接记录；`update-latency` 因产品文件与
  Perf 构建输入不变而沿用正式 P10 结果，未重跑性能批次。原实现证据保持原状态不改写。

验证：

- `scripts/run-go-test.sh ./internal/scanruntime/` 与 `-race ./internal/scanruntime/` 通过；`go vet`、`gofmt -l` 无输出。
- 未重跑 Go 全量、desktop race、双架构构建、native 测试与性能批次：本次只改测试夹具时钟，产品与构建输入不变。
- **CEv1 gate：** 以共享 provider 的 `gate-status.cypher` 查询 `scan-performance:incremental-update` /
  `scan-perf-iu-r1f-20261028-state`：status VERIFIED，`missing_criteria`、`invalidated_evidence`、
  `unresolved_candidate_impacts` 均为空。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / incremental-update

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 2 — 2026-10-28

### 📋 增量更新复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：OpenCode / GPT-6 Astra；未参与修复。
- Method：单评审者，逐项修复核对、blob 反向重建比对、精确状态 CEv1 查询与谱系证据复用；未委派。
- Scope：IU-R1-F1 及固定测试时钟对既有断言、产品与验证证据的影响。
- Reviewed state：HEAD `2098313634584dbe73a5f09b739176f85f931f29`；候选指纹 `b0db6048fb823822740089cc45d9cdb7740c1eda6722551015aacda287cbf67e`。
- WorkUnit：`scan-performance:incremental-update`。
- ContentState：`scan-perf-iu-r1f-20261028-state`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭后通过

无未关闭问题；无新增发现。

### 🟢 优点

- IU-R1-F1 CLOSED：`internal/scanruntime/serving_test.go:216-218` 将时钟固定为本地
  `2026-10-20 12:00`；seed 的 11:58、append/rewrite 的 11:59 都在期望的当天，late 记录仍在前一天。
  日期归属不再取决于实际运行是否在午夜附近。
- 仅替换这段固定时钟及两行注释回原 `now := time.Now()`，在内存中计算 Git blob，
  精确得到 Round 1 的 `de9334e62116f841c82a91eb0a965689737179d3`。
  因而验证了其余 fixture、fresh-oracle 比较、publication、coverage 与 hero 断言均未变，
  没有重试、跳过或放宽断言。
- 两个产品 blob 与 Round 1 完全一致，修复边界保持为测试夹具时钟。

### 📝 总结

唯一历史发现 IU-R1-F1 已关闭；无仍开放、回归或新增阻断项。本轮 PASS。
修复交接为 Beads comment `b6eee9a7-027e-5ec8-b8fe-fae756fc40a0`。

#### Evidence

- 按原配方重算 scoped fingerprint，得到上述 `b0db6048...`；测试 blob 为
  `d64a756dec4e3861ac91d68b3b0b6df65767cedd`，两个产品 blob 与 Round 1 列值一致。
- 原候选仅记录了 hash，没有写入 Git 对象库，直接 `git cat-file` 无法取回；
  改用上述内存反向重建并与原 hash 比对，确认精确修复范围，不把未存储的 hash 当作可读 Git 对象。
- 标准 `gate-status.cypher` 对本轮 WorkUnit/ContentState 返回 VERIFIED，4 项 required criteria 齐全；
  missing、invalidated、unresolved 列表均为空。新状态的四条 `scan-perf-iu-r1f-20261028-*` 证据适用，
  对应 `rolls_up` 谱系有效；旧状态证据不直接冒充新状态观察。
- 复用修复阶段的 scanruntime 包测试、race、vet/gofmt 及固定日期四场景通过证据。
  仅测试时钟变化，不重复 Go 全量、desktop race、native、构建或正式 P10 批次。
  本轮独立执行的是内容身份与修复范围核验、证据门禁查询、评审文档检查，不宣称独立重跑产品测试。
- Round 1 中记录的性能取舍与 native 焦点环境限制继续有效。
- 文档收尾检查使用 `python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/incremental-update.md`、
  `make check-whitespace` 与 `git diff --check`。

任务矩阵仍有 `event-updates-and-cli`、`otel-capture-and-reconciliation`、`cross-surface-acceptance` 未完成，
本轮不跨越 Topic 完成边界。当前 CEv1 WorkUnit 未登记 contains 父关系；不据此宣称整个 Topic 完成，
也不在本次单项复评中补建层级。

Task checkpoint：`ad-scan-performance-incremental-update`；content_state `b0db6048fb823822740089cc45d9cdb7740c1eda6722551015aacda287cbf67e`；gate VERIFIED。

提交建议：将本任务的三个 scanruntime 文件、此评审记录及 tasks.md 对应任务更新作为一个逻辑提交；须单独授权。

推送建议：目标 `origin/feature/scan-performance`；须先获提交与推送授权，核验实际提交范围、签名、远端分支及待推送历史。

### 下一步指令

开发：scan-performance / event-updates-and-cli

WORKFLOW_WORKSPACE: agent-deck.scan-performance
