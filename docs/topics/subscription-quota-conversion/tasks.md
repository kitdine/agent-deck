---
status: active
created: 2026-11-18
---

# 会话订阅额度换算任务

来源为 `ad-subscription-quota-conversion`。工作区 `agent-deck.subscription-quota-conversion` 使用
`feature/subscription-quota-conversion`，创建基线 `a224b0b824d2cad451d3bd83153832d65506f0fe`。
依赖已审定的 `cost-transparency` 逐调用 API 等价和资格契约。需求、CLI 和架构已复评。以下为实现拆分草案。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `quota-window-valuation` | [ ] | [ ] |
| 2. `quota-sample-storage` | [ ] | [ ] |
| 3. `quota-session-reporting` | [ ] | [ ] |

## 实现边界

### 1. `quota-window-valuation`

- 负责 `internal/quota/` 内新增的窗口配对、观测时段和估值逻辑及其测试。用架构已经定义的 DTO 字段接收已去重请求金额、两个观测和证据等级。由同样的输入得到同样的 `observed_delta_pp`、估值或缺口。基于主线的已存在 quota 类型复用可用字段。用合成金额输入先验收规则，运行时逐请求价由后续计价主题交接。
- 验收 L1。验证 `$6` 和 `4 pp`、`2 pp`、舍入边界与零分母、跨窗重置、低精度、多池和端点饱和但上界未知。覆盖零金额、`pricing_incomplete`、`currency_mismatch`、资格未知作为假设以及已排除的独立支路。多缺口只返回架构排序的一个主要原因。Codex 账号证据缺失和 Claude 同源无账号分别覆盖。保留已有 quota 解析回归。可独立于计价持久化开发。

### 2. `quota-sample-storage`

- 依赖 `quota-window-valuation` 和计价 `pricing-facts-storage` 的合成基线 `P`，额度迁移由主控核对实际组合后分配 `P+1`。启动前记录目标所含 v0.7.5 quota 储存版本，对齐 `internal/quota/store.go`、`quota_delivery_guard.go`、`internal/store/{store.go,migrations.go}`、`internal/backup/backup.go` 的实际表、Store/Windows/Envelope 接口、跨进程锁和归档清理。同期候选使用了 schema 36、`_v2` 表和 VACUUM INTO 备份方式；账号身份本身仍可选。当前题目复用该状态上现有机制，不另创建一套。
- 本任务负责上述共享存储边界、`internal/quota/scheduler.go`、`cmd/agentdeck/quota.go` 以及清除用量或删除会话时清理采样引用的既有入口。按代次原子写入，使用既有跨进程刷新锁、已授权探测及门禁。打包时去除私有样本，恢复时清理，验证失败可重试。
- 先完成独立的采样存储、备份和命令测试。用量数据清除或会话删除若必须改动计价已拥有的 `internal/usage/usage.go` 或会话入口，在其计价实现验收并交接后修订；能由本次迁移安装的数据层清理机制完成时，沿用该机制。
- `P+1` 的正式迁移和桌面共享文件在计价 `pricing-reporting` 候选验收并交接后同步。共享范围为 `internal/desktop/serving_schema.go`、`apps/macos/AgentDeckShared/DesktopWire.swift` 的 core schema 边界、`apps/macos/AgentDeckVerification/main.swift`、`apps/macos/AgentDeckTests/DesktopWireTests.swift` 及 `desktop/fixtures/v1/` 金标。在该候选上重新生成金标并跑相关 Go/Swift 验收。已有 `P` 的 serving 负载只按这个方向兼容，会话估值走只读样本入口，原生验收保持旧 App Group 金额口径。
- 验收 L3。以隔离探测夹具验证门禁、CLI 错误和退出码、start / finish / cancel 幂等、失败不回退旧端点和采样竞争。覆盖 `finish` 给出 `unsettled` 后保留进行中采样、已知换号后样本失效，以及清除用量或删除会话时清理引用。对远端用量保留条件，已观察到本机竞争采用对应原因。验证升级、备份和恢复无私有样本，Go/Swift 核对方向性兼容和 schema-ahead。

### 3. `quota-session-reporting`

- 依赖 `quota-sample-storage` 和计价 `pricing-reporting` 的明确交接，使用实际逐笔结果及已验收的会话级 API 汇总。之后负责 `internal/usage/session_usage.go`、`cmd/agentdeck/main.go`、`session_viewer_data.go` 的会话详情与 Tokens 计价配对，以及 `internal/quota/` 已存端点只读入口。跨主题共享文件由计价报告完成后交接。只读快照固定会话、事件集合、价格版本和观测代次。窗口观测与同范围内符合条件的 API 金额配对。相同输入可重放，价更新时可用原观测重算分子。缺完整信息时给出原因。
- 验收 L2。正常入口断言 JSON 路径、完整、观察/条件估值、测量中、被排除的活动、已知竞争、旧库及代码缺失的输出。覆盖两种客户端、多池及包含相同会话金额的两个窗口。读取老快照仍安全。测试中显式断言报告路径没有探测和持久写入。

## 交接顺序

以实际主线为准。采样前核对所含的 v0.7.5 quota 存储和备份路径；账号身份仍属可选增强。价格的规则、迁移与共享会话入口依赖由 Beads 记录，合成基线未核实前可先做窗口规则及隔离夹具。会话级估值首发属于 CLI。正式实施以当前矩阵评审、CEv1 及任务登记为准。
