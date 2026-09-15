---
status: active
topic: snapshot-performance
subject: worker-scan-development-design.md
---

# Worker Scan Development Design Review

## Round 1 — 2026-09-10

## 📋 补充开发设计评审

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。未发现需要修改本设计的已证实缺陷。

### 🟡 改进建议 — 建议处理

无。本轮没有延期处理的低严重度问题。

### 🟢 做得好的方面

- 单例范围绑定 canonical state root，并区分进程锁权威、诊断 PID、
  worker nonce、版本协商和私有 IPC，避免以连接失败为由启动第二个扫描者。
- scope 仅决定前台等待；初始请求仍承担两个域的执行义务。新输入进入有限的
  后续 observation，旧域完成不能冒充新请求的新鲜结果。
- 数据行、游标、上下文和域检查点决定完成；接受回执先持久化再确认。
  崩溃恢复从已提交检查点核实结果，核心与会话域保持独立失败语义。
- 先计划 skip/range，再共享读取；归约和数据库发布分离，同时保留各域源顺序、
  ownership、partial-line 和 FTS 语义。预算覆盖队列以外的实际持有者。
- 冷导入、SQL batch 和并发参数是需要测量的候选；拒绝过的 batch-credit
  实验没有成为实现基线。前台时延与全局成本分开，worker CPU/RSS 不被遗漏。
- W0-W7 明确依赖、边界和验证；V01-V19 覆盖进程、数据、协议和 UI 风险。
  原有性能目标、只读 snapshot 与原生验收边界仍然保留。

### 📝 总结

Reviewer：Codex 主代理。

Method：单代理文档评审；按前提真实性、当前实现一致性、场景覆盖、决策完整性、
内部矛盾与隐含依赖审查。无子代理；本轮不构成 worker 实现或原生运行时验收。

Scope：补充设计全部 15 节；关联 requirements/architecture 契约、Documents
矩阵及文档所述当前 CLI、desktop、ingestion 和 Swift helper 行为。现有 Task 2
实现的完整代码评审和历史其他文档复评不属于本轮。

Reviewed state：工作区 `agent-deck.snapshot-performance`，
分支 `feature/snapshot-performance`；HEAD
`446a58f1f6716f257680879e5dbf3b61365c8cb2`；文档 blob
`d67954bafe491cbe396508b8b68341f81f0afa31`。

ContentState：
`urn:ce:agent-deck:state:document:49fd6c956f96cf48b133fba0e1594366507111e58685b5e8a74cfb997a938f3c`。
摘要公式为 SHA-256(`head=<HEAD>;document=<blob>`)。

Evidence：

- CodeGraph 首次结构定位提示索引属于 main；仅用作定位，当前行为改由 topic
  工作区源码核实，没有创建索引。
- `cmd/agentdeck/desktop.go:139` 的刷新路径先持有现有状态锁、打开两个库、
  启动 shared coordinator，再运行两域服务；会话 checkpoint 写入失败会影响
  域结果。`main.go:2021`、`:3295` 附近确认 legacy 扫描与隐式扫描入口。
- `internal/desktop/desktop.go:238` 使用只读 store；当前 `SignalsBatch`
  与 presentation 复用存在。usage/session 源码保留不同游标、partial-line、
  priority 与 registry 语义，不能仅取最小字节偏移冒充兼容 resume。
- `EmbeddedHelperRunner.swift:204` 的 `runLines` 收集行但等待退出才返回；
  `:348` 的 snapshot 先刷新再取快照，`:466` 的 refresh 使用 `run`。
  新进度订阅确实需要执行期间的数据接口；现有 120 秒刷新和 30 秒 snapshot
  默认 deadline 未被本设计无声延长。
- MenuBarSurfaceView 的 loadingSurface 与 MenuBarViewModel 的 refreshing
  状态已存在；设计要求后续原型、双语、可访问性和原生验证，没有将它们写成已通过。
- requirements 的完整周期性能目标与 architecture 的独立 checkpoint、epoch、
  dirty/revision、8 MiB 私有缓存和只读回退契约逐项对照，无未说明的冲突。
- 本轮本地链接/锚点检查通过：14 个链接；W0-W7、V01-V19 声明完整。
  包到场景的对应关系另经人工核对，结构检查不替代语义评审。
- 必需 L0：`bash scripts/check-topic-docs.sh`、`make check-whitespace`、
  `git diff --check` 在本轮记录状态执行并通过；被评审文档 blob 未变化。
- 仅文档/记录发生变化；没有运行 Go 全仓测试或原生验收，也没有把历史性能
  样本当成本设计的新实现验收。

Findings：无；没有未关闭或转交的本轮 finding。

WorkUnit：`snapshot-performance:worker-scan-development-design.md`。
Beads：`ad-sp-doc-worker-scan-design`。依据 Documents 矩阵新增缺失的文档
dispatch，未创建 W0-W7 的实现任务，未改动六项实现任务状态。

完成门禁：VERIFIED。CEv1 固定 `gate-status.cypher` 对上述 WorkUnit 与
ContentState 回查，4/4 必需判据通过；missing、invalidated、unresolved 均为空。
节点及关系写入后已读回核实，关系批次预检全部为 ok。

本设计可以作为后续开发的契约。其第 1 节“drafted, not reviewed”记录撰写时
状态；当前评审结论以本记录和 Documents 矩阵为准。W0 开发入口仍须绑定其
实际任务/证据范围；本轮不把补充包合并进已完成的 performance-contract。

Task checkpoint：`ad-sp-doc-worker-scan-design`，以上 ContentState；门禁 VERIFIED。

提交建议：取得单独授权后，仅提交补充文档、其评审记录及必要的 topic
architecture/tasks 登记 hunks；不夹带未评审的 Task 2 实现。

推送建议：取得单独授权，核实 remote、分支、完整提交消息、
Codex trailer 和 SSH 签名后推送 `feature/snapshot-performance`；目标 remote
尚未核实。推送不代表合并或发布。

下一步指令：开发：snapshot-performance / worker-scan-development-design.md / W0

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Replanning disposition — 2026-09-10

The user requested a complete topic replan and permitted topic-code replacement.
See [whole-topic review and replacement plan](tasks.md#round-2--2026-09-10).
The rounds above remain historical facts for their own content. They do not
approve the revised document set, authorize delivery, or prescribe the current
next command. The current Tasks matrix is the only execution plan.
## Round 2 — 2026-09-11

## 📋 worker-scan-development-design.md 现行设计复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无未关闭 finding。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

status: historical 和开头 Superseded 明确规定仅保留历史证据，全部操作性要求转到现行 requirements/architecture/tasks。保留的 W0-W7/V01-V19 是历史覆盖材料，不是当前任务、前置或开发指令。

### 📝 总结

Reviewer：Codex 主代理。Method：development-workflow REREVIEW，单代理，
逐项 finding 处置、当前源码/合同交叉核验、共享原型浏览器观察；未委派，
不声称冷上下文独立性。Scope：worker-scan-development-design.md。

详细跨文档证据、原型验证复用和历史 finding 处置见
[整套复评](tasks.md#round-4--2026-09-11)。本记录与该轮有明确共享审查关系。
本 subject 无其他未关闭 finding；历史 PASS 仍仅适用于其原内容。

Reviewed state：HEAD 446a58f1f6716f257680879e5dbf3b61365c8cb2；
Git blob 95077f4f4238d307db95eae02fcfd16f364b8879；content_state 8d7a623b49a203e6621ed4fb7d8c120eca41ea7d3131139b02f0166231478abf。
配方：SHA-256(head=<HEAD>;document=<blob>)。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task：ad-sp-doc-worker-scan-design；WorkUnit：snapshot-performance:worker-scan-development-design.md。

完成门禁：VERIFIED。固定 gate-status.cypher 对本轮精确 ContentState 查询通过；
required criteria 为 4 项，missing/invalidated/unresolved 均为空。
本轮追加 37 个节点、58 条关系；关系预检 58/58 ok，实际创建数量匹配。
没有复写历史观察或把旧已交付实现的证据改成新引擎验收。

Task checkpoint：ad-sp-doc-worker-scan-design；content_state 8d7a623b49a203e6621ed4fb7d8c120eca41ea7d3131139b02f0166231478abf；门禁 VERIFIED。
提交建议：单独授权后提交 worker-scan-development-design.md、对应评审和 topic 矩阵；与其他本轮文档协调暂存边界，不混入未评审 Go 候选。
推送建议：origin/feature/snapshot-performance（候选目标）；须单独授权，核验暂存范围、提交正文/署名/SSH 签名和实际远端配置。本轮未推送。
原生 worker/IPC/SQLite/Swift/VoiceOver/Dynamic Type 和完整性能指标不属于文档
PASS 的证明范围；任务 1–3 及 topic 完成边界仍开放。未提交、推送或启动实现。

下一步指令：开发：snapshot-performance / unified-scan-runtime
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
