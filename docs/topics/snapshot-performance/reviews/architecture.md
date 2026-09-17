---
status: active
topic: snapshot-performance
subject: architecture.md
---

## Round 1 — 2026-09-09

## 📋 架构文档评审

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无开放 finding；实现阶段的参数选择和性能证明不作为本轮未决设计问题。

### 🟢 优点

- 将已处理源检查点、数据库代次和派生缓存有效性分开，不能以缓存存在代替源导入完成。
- dirty 合并失效配合构建前推进 revision，构建后复核；并发写入、失败发布均可安全回退。
- DTO 明确保留内部 Summary；价格缓存仅提供验证成功的 availability，其他健康输入仍实时读取。
- 共享解析保留域内顺序、机器身份和提交结果，设置队列预算与私有派生记录溢写，取消和恢复边界明确。

### 📝 总结

Reviewer: Codex；session 01a085b3-3e60-7ae2-9524-3a1316aec423。
Method: 单一主评审角色的设计契约及状态转换审查、CodeGraph 定位与针对性源码核对、L0 文档检查。
本会话未起草架构文档，未委派子代理；本轮不是独立冷上下文评审或实现验收。
Scope: architecture.md；需求 PASS 和 tasks.md 作为约束、验证责任参考。
tasks.md 分解尚未获得本轮批准。

Reviewed state: HEAD f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f；
architecture.md blob 723f3743f6c0ec1d04a4e6068f88e48790b37acf；
SHA-256(head=<HEAD>;document=<blob>) =
a76cee15baa1bbddb38528792a88ccff3913ab6c3367d23fcd61ac673cec0763。
Workspace: agent-deck.snapshot-performance / feature/snapshot-performance。
Task: ad-sp-doc-arch-design；WorkUnit: snapshot-performance:architecture.md。
Prerequisite: requirements.md Round 1 PASS，文档门禁 VERIFIED；需求 blob
01a3a52943757a73873d27f78cea683741ccdd0e。需求任务未提交不等于需求未通过评审。

Evidence:

- internal/usage/presentation.go:239–264：读取 90 个本地日历日，按 today/7d/30d
  聚合，小时展示使用当前本地小时；:21 的 Summary 为 json:"-"。
  internal/desktop/desktop.go:555–574 按三个期间、三个客户端调用 Signals。
  因而小时/日期失效和独立 Summary DTO 与当前行为一致，不能直接缓存公共 JSON 后当作内部报告。
- internal/usage/usage.go:1788–1825：PriceStatus 验证历史生效时间并构建有效价格；
  internal/doctor/doctor.go:458–464 消费 available。架构要求缓存验证成功的结果，
  保留错误语义并在计划价格生效时失效；普通 doctor/price status 仍保留原路径。
- internal/store/store.go:25 的 schema 版本为 23；
  EmbeddedHelperRunner.swift:315–316 的 snapshot/refresh 超时为 30/120 秒，
  :466–500 先运行 refresh-indexes。架构正确区分超时与性能目标，不借修改超时达标。
- 复用前轮已核对的 cmd/agentdeck/desktop.go:130–173 和
  internal/desktop/desktop.go:235–274：refresh 保留分域错误，snapshot 独立打开
  core/session 并读取 live health。架构中的兼容性检查、缺失 session、缓存 miss
  无写入回退不改变这些契约。相关产品文件无变化，未重复运行 Go/Swift 套件。
- 情景推演：解析后源再次改变不得提交新字节的检查点；dirty 时禁止复用；
  构建前推进 revision 防止 dirty 期间多次变更共用身份；构建/读取期间变化拒绝旧缓存；
  失败发布不撤销已提交索引；单次构建和无界重试禁令保证持续写入时可退化。
- 缺 marker、旧 schema、未来 schema、恢复/替换索引、同大小/mtime 改写、
  时区/DST、时钟回退、未来价格、oversize、损坏/符号链接、取消及临时文件恢复
  都有明确失效或回退规则。64 MiB 队列预算不冒充单条大记录或冷导入 RSS 上限。
- L0 使用 check-topic-docs.sh、make check-whitespace、git diff --check，
  并检查架构/评审/状态文件的相对链接及未跟踪文件空白。
  结构检查不证明并发实现正确，也不验证研究表中的性能数字。

完成门禁：VERIFIED
固定 gate-status.cypher 的最终精确状态查询确认 contract-correct、
document-checks、handoff-complete 三项均有有效 pass evidence；missing_criteria、
invalidated_evidence、unresolved_candidate_impacts 均为空。
节点批次创建 5/3 个节点，关系批次创建 3/6 条关系，与提交数量一致；
关系 preflight 全部 ok，最终门禁回读确认准则、证据及其目标绑定。

本轮确认设计契约可进入分解评审，不宣称实现达到 10 秒/1 秒/CPU/RSS 目标。
研究表是有明确限制的探索观察；本轮未重新运行预研、运行时并发或真实客户端验收。
这些验证仍由实现任务和最终验收承担，需求中保留的未达标处置义务不变。

Task checkpoint：ad-sp-doc-arch-design；上述 content state；门禁 VERIFIED。
提交建议：经单独授权提交架构文档、对应评审记录和矩阵交接；tasks.md 仍是未评审分解草稿，暂存时须明确文档骨架与交接边界。
推送建议：候选 origin/feature/snapshot-performance；需单独授权并核验提交、签名和远端，本轮未执行。

下一步指令：评审：snapshot-performance / tasks.md
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Replanning disposition — 2026-09-10

The user requested a complete topic replan and permitted topic-code replacement.
See [whole-topic review and replacement plan](tasks.md#round-2--2026-09-10).
The rounds above remain historical facts for their own content. They do not
approve the revised document set, authorize delivery, or prescribe the current
next command. The current Tasks matrix is the only execution plan.
## Round 2 — 2026-09-11

## 📋 architecture.md 现行设计复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无未关闭 finding。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

TOPIC-R3-F1 CLOSED：§9.1–§9.4 明确恢复四值白名单、专用 DTO/Summary、header/时区/价格截止点、0700/0600、8 MiB、原子发布及读后复核；Task 2 的四个链接有效。worker 不持状态写锁等待，数据库独立提交、有限轮次、receipt 恢复和资源归属有明确约束。

### 📝 总结

Reviewer：Codex 主代理。Method：development-workflow REREVIEW，单代理，
逐项 finding 处置、当前源码/合同交叉核验、共享原型浏览器观察；未委派，
不声称冷上下文独立性。Scope：architecture.md。

详细跨文档证据、原型验证复用和历史 finding 处置见
[整套复评](tasks.md#round-4--2026-09-11)。本记录与该轮有明确共享审查关系。
本 subject 无其他未关闭 finding；历史 PASS 仍仅适用于其原内容。

Reviewed state：HEAD 446a58f1f6716f257680879e5dbf3b61365c8cb2；
Git blob b67ca9e99499b6d8dcb445235a0a94f10888e9b2；content_state be89e950a2a6c1d72b711c9c0b183c160b6effcd973ae723f1d897e2ea9c357f。
配方：SHA-256(head=<HEAD>;document=<blob>)。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task：ad-sp-doc-arch-design；WorkUnit：snapshot-performance:architecture.md。

完成门禁：VERIFIED。固定 gate-status.cypher 对本轮精确 ContentState 查询通过；
required criteria 为 3 项，missing/invalidated/unresolved 均为空。
本轮追加 37 个节点、58 条关系；关系预检 58/58 ok，实际创建数量匹配。
没有复写历史观察或把旧已交付实现的证据改成新引擎验收。

Task checkpoint：ad-sp-doc-arch-design；content_state be89e950a2a6c1d72b711c9c0b183c160b6effcd973ae723f1d897e2ea9c357f；门禁 VERIFIED。
提交建议：单独授权后提交 architecture.md、对应评审和 topic 矩阵；与其他本轮文档协调暂存边界，不混入未评审 Go 候选。
推送建议：origin/feature/snapshot-performance（候选目标）；须单独授权，核验暂存范围、提交正文/署名/SSH 签名和实际远端配置。本轮未推送。
原生 worker/IPC/SQLite/Swift/VoiceOver/Dynamic Type 和完整性能指标不属于文档
PASS 的证明范围；任务 1–3 及 topic 完成边界仍开放。未提交、推送或启动实现。

下一步指令：开发：snapshot-performance / unified-scan-runtime
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
