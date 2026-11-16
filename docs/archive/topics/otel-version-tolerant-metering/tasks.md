---
status: historical
created: 2026-11-15
updated: 2026-11-15
retired: 2026-11-16
---

## Retirement reconciliation — 2026-11-16

本主题已通过 PR 集成到 main：`c270d473cbc22b5b7aee4ad00fbbbc9f02aa893b`。
集成 WorkUnit `v0-7-0-contract:integration:otel-version-tolerant-metering` 的实际 merge 状态
门禁 VERIFIED 4/4；版本组合验收见 [合同记录](../../../topics/v0-7-0-contract/reviews/assemble.md)。
本次随 v0.7.0 合同收口整体归档；以下日期条目保留当时的身份、结论和限制，
其中未提交、待交付或 active 等交接文字不再表示当前调度状态。
归档不代表 v0.7.0 已发布或已安装。


# OTel 版本兼容计量与自动分类任务

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
| UX | n/a | n/a |

epoch9替代稿已通过独立评审；完整结论见 reviews。此前跨仓库设计的 verdict/CEv1 仅作历史。
沿用现有列表、详情和Settings布局及交互；文案/值变化在架构表面合同中定义，评审覆盖原型依赖。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `version-tolerant-admission` | [x] | [x] |
| 2. `ownership-guard-readmission` | [x] | [x] |
| 3. `builtin-rule-set` | [x] | [x] |

范围见[需求](requirements.md)，字段/规则/生命周期见[架构](architecture.md)。
三个任务按顺序实现和独立评审，正式派发在设计通过后创建。

## 1. version-tolerant-admission

internal/store准入唯一定义、telemetry/reconcile.go及测试。
验证0.165、0.169、预发布及其他非空合法版本一致准入；空/分隔符/超既有上限、未知
家族/transport/形状拒绝；缺thread、无/不完整/无效计数、冲突与基线一致，Record key字节不变。

## 2. ownership-guard-readmission

依赖任务1。store/telemetry、usage解析/checkpoint/publishFacts和schema35相关代码及测试。
迁移在本任务同时扩展default_work CHECK与thread_id列，重建派生表并重装失效triggers，
同步实际Go/Swift schema消费者和恢复校验，保持用户override。

- 路径普通/reverted/归档/大小写/非规范/非rollout，取第一个UUID。
- payload.id保护可读改名子线程，事实accounting ID为父，旧事实/后到观察不重复。
- 首个session_meta归属与继承metadata、append/rewrite/reset、补填事务与源改变重试。
- 存量owned首次成功完成结算前撤销；晚到JSONL普通/cold/prefix同事务撤销，贡献和会话行清理。
- 前缀提交后新观察到达、无checkpoint窗口、失败重启、正常orphan清理。
- NULL未评估不晋升；读取失败不能置空，规范文件名可提供缺失文件的正向证据。
- 粘性jsonl_owned保护同thread后续观察，invalid/conflict等不被覆盖。
- 65 个可读改名来源且各有存量重复，64 行分批自动走完后首次成功结算全部撤销。
- 批间崩溃/取消重启、读取失败与可恢复来源混排，失败项不阻塞后续批次推进；整体未完成
  不宣称成功，不晋升或恢复未确认贡献；后续成功恢复、原事件日金额与重复恢复防护。
- receiver/import/SettleAndProject 的准备入口和 store 事务内检查，现有 incomplete/partial
  诊断与后台自动重试，恢复后清理诊断。
- 每批补填/每轮补计限量，拒绝行不饥饿后续；Go与SQL准入等价。
- 历史原事件日/费用、重复结算幂等、迟到撤销刷新投影和发布。
- schema34→35、旧库只读、新库SchemaAhead、trigger失效、backup/restore、相关金标。
- 线程证据集合构建性能按现有基线测量，不宣称整个既有settle有界。

## 3. builtin-rule-set

依赖任务2。sessionorigin规则/归约/原因码、cmd来源/telemetry输出、store分组计数、
服务与import回调接线、CLI/desktop金标及原生文案/transport/状态映射。

- 规则ID唯一、确定顺序，每条命中与不命中；fallback不参与证据冲突。
- 普通Codex/Claude默认work，TUI无rollout辅助、exec无rollout工作，所有版本一致。
- 旧0.165exec重算work，TUI保持辅助，override及其他adapter证据不受损。
- InputMismatch在辅助证据前处理；冲突/读取失败保留诊断与partial，默认类别work。
- 未配置/禁用adapter中性呈现；内置自动规则无需配置，只有未运行分类为暂态unknown。
- 自动规则revision使旧会话重算；结算/补计/撤销后自动分类；两条注入回调路径及失败重试。
- watch session-only在身份pending时先分类、发出pending事件并继续，未确认检查点自动重试，
  不阻塞其他域；恢复后完成并确认，文本/NDJSON语义与隐私一致。旧规则检查点自动刷新。
- 来源background与otel_owned两个计数、builtin_rules、有无新字段helper兼容，实际错误不吞掉。
- 中文英文默认/来源文案、详情原因规则、现有列表/Settings布局、窄窗口及辅助技术标签不回退。
- 独立HOME/CODEX_HOME/state、本地mock，用已安装0.169验证普通exec、ephemeral、TUI/标题。
  子线程/改名/前缀崩溃可用真实格式合成来源，并区分真实与合成证据。

## 验证与交付

按L2/L3。Go tests使用scripts/run-go-test.sh，受影响包及Go core全套、相关race/vet和构建。
原生对应测试/构建及改变文案的隔离显示检查；不接触真实状态。复用未变证据。
仅AgentDeck仓库，提交/推送/发布分别授权。三个任务实现及独立评审已通过；完成证据门禁见 reviews，尚未提交、推送或安装。
