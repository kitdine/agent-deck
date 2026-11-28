---
status: active
created: 2026-11-18
---

# 窗口观测配对和整窗估值

## 事实与入口

使用 `cost-transparency` 已审定的调用键、会话键、事件时刻、币种、价格覆盖与额度资格。在窗口观察时间内选取已去重请求。可证的额度豁免活动在全会话金额中仍可有 API 等价，其金额从窗口分子排除。任一有资格的请求缺完整价或币种不一致时，显示已知金额及金额缺口。

使用 [既有观察](../../../internal/quota/state.go)、[探测器](../../../internal/quota/scheduler.go) 与 [额度门禁](../../../internal/quota/gate.go)。默认会话读取是只读结果。样本命令遵循 [CLI 入口](ux/cli.md)。现有状态线或后台刷新可以继续更新普通额度视图；保存的前后端点只从该两次显式探测的成功结果取得，失败时不回读普通缓存充作当前端点。

### 账号和窗口实例

- **Codex。** 每端取 `account/rateLimits/read` 成功返回的同算法账号摘要，两个摘要必须存在且相等。测量证实探测账号可比，目标会话使用同一账号仍是估值假设。以供应商 `limitId` 和槽组成现有 `window_key`，保留窗长和来源。primary 与两个池的同窗长无等价关系。
- **Claude。** 两端显式使用既有 `/usage` 解析通道，且属于同一窗口。当前没有可比较的账号 ID，默认在结果报告 `account_unverified`，账号同一作为条件。发生可观察的官方路由切换、认证失败或探测来源变化时关闭估值，当前未观测到的重登录保留为假设。状态线的 `Current session` 对应共享的五小时窗口，其读数与 prose 端点不可混配。
- **边界。** 对同 client、来源、窗口键和窗长，只在 `resets_at` 均可用且相等时配对。Codex 使用归一化 UTC 秒；Claude prose 使用原精度解析后的同一 UTC 分钟，容差为零，时间解析歧义或来源变化则缺可比较端点。两次观测都在其声明的重置锚之前且在已知窗口起止之内。任意一端确认的活动重置或两端之间已有的 `observed_reset_at`、换号、倒序和时钟歧义都会阻止估值。重置锚相等可作为配对前提，期间隐形重置仍可能发生，估值必须带假设。
- **基线。** 旧库记录前一值及时间，缺前值的来源、重置等必要元数据时只显示最近观察和缺口。v0.7.5 账号代次属于可选的集成增强，当前两端采样路径可独立运行。

### 采样存储和事务

在 (35 → 36 → 37) 的当前合成升级路径中，36 为计价主题迁移，37 为本主题。移到已含其他迁移的目标时，在计价结果所在主线的当前最高 schema 后按 [迁移规则](../../../.agent-instructions/branching.md) 顺延，对组合升级后的编号、原生识别和恢复重新验证。

新表 `quota_session_samples` 的键为 `(client, slot)`，`slot` 只能为 `active` 或 `latest`。列为 `client`、`slot`、`generation`（随机、唯一且非空的本次标识）、`session_id`（选中本机用量会话键）、`started_at`、可空 `finished_at`、非空 `start_json`、可空 `finish_json`。时间用 UTC RFC3339Nano。`active` 只有首端点，`latest` 包含两端。每端使用带 `version=1` 的白名单结构，字段为 `probe_started_at`、`probe_completed_at`、`source`、可空 `account_digest`、`windows[]`。窗口字段为 `window_key`、`pool`、`slot`、`window_minutes`、`used_percent`、`resets_at`、可空 `observed_reset_at`、可空 `precision_pp`；精度字段仅用于已知四舍五入的供应商读数。金额与估值按当前可读事实推导。

串行化访问复用现有跨进程 quota refresh 锁。探测期间不保持 SQLite 写事务；完成探测后以单事务核对门禁、选中会话和采样代次再发布。`start` 初始化 `active`，已有同一会话则幂等读回，一次其他会话只能在取消或结束后开始。`finish` 要求同一 `active` 的 `generation` 和会话，在一个事务内用完成内容替换 `latest` 并删去 `active`。失败保存仍有效的首端点，便于重试；已知换号时将其失效，返回原因。`cancel` 仅删匹配当前代次的 `active`，没有同会话活动记录时可重放。不同会话的操作不改变已存样本。普通探测与选中快照各按原状态规则更新。

锁覆盖客户端探测和样本发布，普通后台探测也经其既有锁；样本只保存本次探测返回的端点，已观察重置取同一次配额存储事务确定的值。开始、结束、取消或失效可在同一生成值上做条件写入，旧响应无法提交给后来的测量。`session show` 只选择本会话匹配的 `active` 与 `latest`。已知账号或来源变更清除此客户端的样本；清除用量状态或删除相应会话时清除其采样引用。

### 便携性与隐私

当前 [备份](../../../internal/backup/backup.go) 复制核心 SQLite 后对专用表去除本地数据。将 `quota_session_samples` 加入打包前私有副本的 `secure_delete`、事务清理及 WAL checkpoint；正式归档中的表为空。恢复兼容旧库及外部含该表的归档，迁移后也清理该表。私有副本上清理失败则按原路径失败。验收检查归档表、自由页及 WAL 边界的敏感哨兵。会话键、摘要和端点只在本机使用，普通导出、日志、通知、App Group 或 CLI 都不追加账号摘要或原始载荷。

## 配对与计算

给每窗使用首探测完成至末探测开始的区间，只接受两端完全夹住的目标请求。任意边界落入采样期间，只给同期观察。`finish` 的末探测须开始于选定时段最后一笔已入库调用之后。已知仍有未完成调用或延迟入账时，`finish` 返回 `unsettled`，保留 `active` 可重试。其他成功采集的结果可完成样本，零变化只给观察。当前输入缺逐请求入账回执，正向变化的条件估值仍带 `reported_usage_settled` 假设。

扫描同一时间内已准入的其他本机会话调用，过滤到同客户端和已知或可能同池的套餐请求；明确 API Key、第三方或豁免的观察排除。其他可能共享该窗口的本地调用用 `other_local_usage` 阻止估值。远端、云端和后台未采集用量只能写入独占假设。窗口候选有多个同窗长池且无法确定映射时，各显示读数和 `pool_unresolved`。对应同池各窗可以独立计算。

| 条件 | 实现结果 |
| --- | --- |
| 可比较的两端 | `d = after_used - before_used`。使用剩余显示时为 `before_remaining - after_remaining`。显示同期变化和端点时间/来源。 |
| 显式测量、账号满足对应客户端路径、匹配单池且未发现冲突 | 用 `C / (d/100)` 得条件估值，`C` 为时段内同池匹配请求的完整 API 等价。缺逐请求账号字段时保留会话使用该账号的假设；Claude 另标未知账号。请求可归于套餐的条件必须明确。存在确定的混合渠道无法拆分、池无法匹配或时间与账入证据冲突时缺估值。 |
| 重置或中间代次可能变化 | 若不能确认端点同实例则停止该窗估值。短窗可能跨重置而周窗仍有效。跨窗分段必须各自满足同范围条件；首次交付将缺匹配端点的片段标缺失。 |
| 精度已知 | 在 0–100 约束内按两端量化区间求 `d_lo`、`d_hi`。若 `d_lo > 0`，得到 `[C/(d_hi/100), C/(d_lo/100)]`。端点饱和且上界未知时不给固定额度。 |
| 观察精度未知 | 报告的 `d > 0` 且满足其他条件时，可以给基于报告比例的条件点估值，含 `reported_precision_unknown`，无法给舍入区间。`d <= 0`、有证的分母区间包含零、金额零或价不完整只显示可用数据。 |

报告固定会话、请求代次、价格版本和端点代次。在中途开始测量时顶部仍为全会话金额，分子仅含测量区间可用请求。已证不属于该池的费率事件排除；未知账户或额度资格只在显式测量、兼容路由及无已知冲突时列为假设，并明确标记覆盖。旧观测与新价可在相同观测时间范围重算。两个窗口分别取得分子，顶部完整会话金额只出现一次。

## 输出

会话 `data.usage.quota_valuation` 用 `version: 1`、`status`（`missing` / `active` / `complete`）、可空 `started_at`、可空 `finished_at`、`coverage`（`full` / `portion` / `unknown`）和 `windows[]`。可保存完整的测量而无可比窗口。每窗包含 `window_key`、`label`、`pool`、`window_minutes`，可空 `before` / `after`（`used_percent`、`observed_at`、`source`），可空 JSON 数字 `observed_delta_pp`，可空 `estimate`（`currency`、十进制字符串 `amount` 或成对 `lower` / `upper`、`basis_amount`），`assumptions[]` 和可空 `reason`。有界时只提供范围。可选值省略或置空都表示未报告。

窗口假设使用 `session_uses_sampled_account`、`account_unverified`、`all_change_from_selected_usage`、`reported_usage_settled`、`reported_precision_unknown`、`subscription_scope_assumed`。没有可用估值时，按测量、窗口身份、账号、可归属事件、价格、分母顺序在 `measurement_missing`、`measurement_in_progress`、`window_missing`、`window_unverified`、`pool_unresolved`、`account_changed`、`reset_detected`、`other_local_usage`、`unsettled`、`mixed_billing`、`pricing_incomplete`、`currency_mismatch`、`no_reliable_delta` 或 `saturated` 中给出一个主要原因。动作失败沿用统一错误 envelope 及退出码，新增拒绝原因 `measurement_conflict`、`measurement_missing`、`unsettled`、`probe_disabled`、`not_official` 和 `probe_failed`。成功返回 `action` 和 `status`。开始返回 `started_at`，结束返回 `started_at` 和 `finished_at`，取消省略两个时间字段。完成时返回同一 `quota_valuation`。

## 验证

CLI 正常入口的 text 和 JSON 需验证金额、窗口和理由。隔离测试以 `$6`、`40→44` 验证算式和条件估值，对比有本地竞争的读数相同样本。覆盖 Codex 探测摘要一致但会话账号未证、Claude 同源无账号和已知换号、只更新一窗、短窗跨重置而周窗有效、多池同窗长、延迟、超额、免费、低分辨率、封顶、旧库、立刻重试和跨进程代次竞争。存储迁移按 L3，加 race、备份/恢复及隐私检查。[项目矩阵](../../../.agent-instructions/project-rules.md#testing-and-verification--测试与验证)
