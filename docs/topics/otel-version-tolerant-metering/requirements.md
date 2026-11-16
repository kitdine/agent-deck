---
status: active
created: 2026-11-15
updated: 2026-11-15
---

# OTel 版本兼容计量与自动分类需求

## 来源与范围

来源为 Beads `ad-bug-otel-version-pinned-metering`。范围仅限 AgentDeck 仓库，
不修改、不依赖 Codex 源码。core schema 升至 35，扩展原因码约束并保存来源 thread_id，不新增表。

用户决定（2026-11-15）：
- 自动补计已有的合格历史记录。
- 分类必须开箱自动可用：可读会话得到 work 或 auxiliary，不以 unknown 为常态，
  不以逐会话设置或 adapter 配置为前提。内置规则集可持续补充。
- 不接受修改上游。

早先草稿的 producer 身份合同、none_fresh、额外身份表和历史身份全覆盖机制、
"仅凭正向证据"与 exec A/B 选项均已撤回。

## 已核实的问题

1. `internal/telemetry/reconcile.go` 的 `supportTable` 比较精确版本，新版本全部
   落入 `capture_only/producer_unsupported`。
2. `SettleTelemetry` 不重新评估历史拒绝行。
3. Codex 用量解析优先取 `payload.session_id`，子线程的 JSONL 事实记在父 ID 下，
   OTel 的 conversation.id 是子线程 ID。结算与撤销只比对 accounting ID，
   子线程可被 JSONL 与 OTel 各计一次。0.165.0 下已存在。
4. `internal/sessionorigin/codexotel.go` 的规则钉死两个 0.165.0 条目。
5. `Reduce` 从不输出 work，无规则命中的会话全部是 unknown。
6. 来源状态把全部 OTel 所有的会话称为 background，规则版本写死 v1，
   并把未配置的可选 adapter 显示得像故障。

以下行为正确，保持不变：无计数 completed 为 `unmetered/no_completed_usage`；
计数不完整为 `capture_only/token_components_missing`；无效为 invalid；
同线程同记录时间计数不一致为 conflict。

## 目标

### 计量

1. 准入按家族与事件形状判定，版本只作来源信息。
2. 保留隐私白名单、无效与不完整计数拒绝、计数冲突、重放幂等、观察键不变、
   按会话的 JSONL/OTel 排他所有权。
3. 有 JSONL 证据的线程归 JSONL，包括通过 payload.id 或规范 rollout 文件名识别的子线程；
   存量可恢复重复在升级后首次成功完成的结算前撤销；JSONL 晚到时在写入事实的同一事务撤销。
4. 历史合格记录自动补计，使用原始计数、原始键、原事件时间，每轮限量，可重复执行。

### 分类

5. 内置规则集按固定优先级自动执行。每条规则有带版本的 ID、匹配条件与结果。
   新增规则是加一个表项和它的用例，不引入规则语言、插件或用户编写规则的要求。
6. 可读会话默认 work。unknown 只表示尚未分类。
7. 无 JSONL 的 OTel 会话：TUI 家族判 auxiliary，exec 家族判 work。
   两者都是推断，文档与诊断输出如实说明。
8. 用户 override 与已配置 adapter 的证据优先级不变。override 是可选纠正手段。
9. 扫描、历史补计、结算改变会话后自动重新分类，无需用户操作。
10. 来源状态区分"内置规则已就绪"与"可选 adapter 未启用"，计数与规则一致。
11. 每个结果带原因码与规则 ID 供诊断，普通列表视图不增加噪声。

## 明确的例外与残余风险

- 结构检查发现不了"形状不变而计数语义改变"的上游变更。
- 改名副本使用自身 payload.id 保护。不可读且没有规范文件名或已保存身份的来源，
  保留未评估状态，自动重试或由正常来源清理处理；不将读取失败视为无 JSONL 证据。
- `.jsonl.zst` 不在扫描范围，其会话只由 OTel 计量。
- 主会话 ID 下未写入 JSONL 的请求不单独补计。
- 开启 Memories 时，exec 家族的后台整理会话显示为 work。
- TUI 若出现无 rollout 的主会话，会被判为 auxiliary。
- 未被任何规则识别的后台会话显示为 work，直到补充规则。

## 验收矩阵

| 场景 | 计量 | 自动分类 / 原因 / 规则 |
| --- | --- | --- |
| 任意合法版本串的两个家族 | 同一规则准入 | 按规则 |
| 空或非法版本、未知家族/transport/形状 | producer_unsupported | 不涉及 |
| 无计数 / 不完整 / 无效 / 冲突 | 现行行为不变 | 不涉及 |
| 同一记录重放 | 键不变，幂等 | 不变 |
| 普通 Codex 或 Claude 会话，无规则命中 | 不变 | work / default_work / default-work-v1 |
| claude-mem 映射命中 | 不变 | auxiliary / maintenance_origin / 现有规则 |
| claude-mem 未配置 | 不变 | 其余会话 work，来源状态不报故障 |
| claude-mem 已启用但读不到 | 不变 | work / origin_source_unavailable，来源状态告警 |
| 子线程，事实在父 ID 下，规范文件名 | JSONL 所有 | 按 JSONL 会话归约 |
| 存量已 otel_owned 的子线程 | 首次成功完成结算前撤销 | 标记移除后重新分类 |
| 子线程 JSONL 晚到（含前缀块） | 同事务撤销 | 同上 |
| 线程已有 jsonl_owned 观察后来新观察 | 不计 | 不变 |
| reverted 线程文件名 | 取第一个 UUID | 不变 |
| 改名子线程与尚无 checkpoint 的前缀 | payload.id 原子撤销，前缀存在时不晋升 | 正常自动归约 |
| ephemeral exec 主会话 | OTel 计一次 | work / default_work / codex-otel-exec-work-v1 |
| TUI 标题请求 | OTel 计一次 | auxiliary / maintenance_origin / codex-otel-tui-background-v1 |
| 0.165.0 存量会话 | 数字不变 | TUI 为 auxiliary，exec 为 work，override 保留 |
| auxiliary 与 work 证据并存 | 不变 | work / conflicting_origin |
| 历史合格行 | 补计到原事件日 | 结算后自动分类 |
| 尚未分类的会话 | 不变 | unknown / origin_metadata_missing，下次扫描后消失 |
| 用户 override | 不变 | override 优先 |

## 生效范围与验收

本稿取代 epoch 5 跨仓库设计。旧评审与门禁仅证明旧候选，当前设计须重新评审。
版本无关准入与分类按本稿统一执行，自动归类允许明确的保守默认与推断，不要求用户配置。
分类结果不参与数字准入。来源配置/读取失败仍提供诊断，不能伪造 successful/complete。
原生及 CLI 沿用既有布局和交互，仅更新默认类别、原因与来源文案；具体文案见架构。

## 升级收敛的完成边界

64 个来源是单批预算，不是首次升级结算的整体预算。首次成功结算前自动连续处理
本次捕获的待补填来源集合，每批原子保存已确认身份和精确撤销。取消、崩溃或仍有
不可恢复读取缺口时该次结算未完成，正常启动/扫描/周期自动重试；不宣称重复已消除。
期间保持既有最后成功数据，并通过现有 incomplete/partial 或错误状态报告未完成。
新晋升及未确认 owned 的贡献恢复都等待完成检查，不能仅删除金额而保留可恢复重复的路径。
