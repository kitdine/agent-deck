# Task 3 有界补充交接 — 2026-10-21

本轮实施完成，待独立评审；Task 3 性能验收未通过。工作区
`agent-deck.scan-performance`，分支 `feature/scan-performance`，同一 Beads
`ad-scan-performance-incremental-projections`。没有进入 Tasks 4–7 或 Git 交付。

用户明确撤销 P01 点击分位/强制百次、P02 点击子区间、P07/P09 点击依赖，
并选择撤销 P10 的 2 秒及落盘到绘制计时；不再补点击、Quartz、窗口锚点或绘制时钟。
P03 不恢复。P07 RSS≤256MiB、完整写事务 p95≤50ms/max≤250ms 保留。
P10 只保留真实文件自动更新、publication、partial/非可加/All/full oracle 正确性。

一次同 cohort 无点击正确性试样通过：完整基线后真实追加/fsync 19-token 记录，
App 自动推进完整 publication 与匹配 App Group；增量与同输入空库 full oracle
完整输出相等，双方 partial=false，按实际 America/New_York 时区比较。
未显式 refresh、赋值模型或注入通知；不计入性能分布。历史失败/无效 trace/Round6 PASS
全部保留。输入到期为 `2026-10-22T12:48:07.489352+00:00`；不延长、不重采真实 HOME，
到期或不足运行预算即停止依赖原件。

24 个源文件的新增差异及三个文档的新说明是本轮审查范围。
以 [范围和基线摘要](incremental-update-bounded-review-scope-20261021.json)、
[已核对基线的源码差异](incremental-update-bounded-review-delta-20261021.patch)、
[完整结果](incremental-update-bounded-result-20261021.json)、
[内容身份](incremental-update-bounded-content-states-20261021.json)、
[影响评估](incremental-update-bounded-impact-20261021.json) 和
[门禁](incremental-update-bounded-gate-20261021.json) 为依据。

产品 `scan-ip-bounded-product-e14fca302e5e8572`；仪表
`scan-ip-bounded-instrumentation-df7a0429f058d27f`；编排
`scan-ip-bounded-orchestration-067e1bcc6aee7761`。
CEv1 四项 required 保留，三个技术准则按明确影响评估/目标绑定 roll-up 复用；
精确门 **FAILED**。core AcceptInventory384.921506ms、session285.331956ms
完整事务反例仍在；当时 RSS 有两个缺口，不能作通过证明。最后 DISTINCT 孤儿路径微调
在该诊断之后，仅有差分/执行计划回归，不把旧 34.479610ms 说成最终 SQL 实测。
最终 RSS 切面修订只有针对性回归与合成校准，没有代表性通过结论。

独立评审重点：

- 原事务内覆盖索引定位、无孤儿短路、关联数据删除、共享会话与回滚；prepared statements
  不改变 coverage/offset 校验或事务原子性。
- Begin/Exec 至 Commit/Rollback 返回完整边界、调用链与三段归因；不缩小计时，保留 WAL、
  durability 和全部观察开销。
- 固定 50ms、PID/birth 去重、短命/退出/重挂父进程/PID 复用、迟登记、完整起止窗口，
  256 条异步日志队列；缺口/溢出/丢记录必须无效，不能用 errno=0 空结果证明退出。
- exec-only 包实际生成器覆盖，原生 readiness 对中间/完整/已绘制旧值的区分，
  以及新验收范围与历史记录的准确性。

已有 full Go/race/vet、native 纯 readiness、双架构 CLI/build 和新差分测试按结果中的
精确范围复用；不重复未变测试、R6 或已通过技术评审，不自造新正式分布。
本轮没有独立评审 verdict。评审通过后，后续仅保留未撤销的 P07 计划；
不恢复 P01/P10 计时，不用反复采样替换已知失败，也不跨出 Task 3。

```text
评审：scan-performance / incremental-projections / update-latency
WORKFLOW_WORKSPACE: agent-deck.scan-performance

仅评审本交接定义的新增差异及三个文档的新要求/说明；冷上下文独立角色。
生产代码、测试、配置只读；先核对内容清单，再复用适用证据。
保留 P07 FAIL、历史无效 trace、Round 6 PASS 和用户撤销决定。
不运行点击或 P10 绘制计时，不委派，不进入 Tasks 4–7，不提交、推送、安装、发布或部署。
```
