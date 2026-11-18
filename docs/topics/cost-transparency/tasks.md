---
status: active
created: 2026-11-18
---

# 计价重整任务

来源为 Beads `ad-cost-transparency`。工作区 `agent-deck.cost-transparency` 使用
`feature/cost-transparency`，创建基线为 `a224b0b824d2cad451d3bd83153832d65506f0fe`。
版本成员由 `v0-8-0-contract` 承载。需求、展示与架构已按轮次审定。以下为实现拆分草案。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| ux/menubar.md | [x] | [x] |
| ux/widget.md | n/a | n/a |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `tariff-catalog` | [ ] | [ ] |
| 2. `pricing-facts-storage` | [ ] | [ ] |
| 3. `request-pricing` | [ ] | [ ] |
| 4. `pricing-reporting` | [ ] | [ ] |

## 实现边界

### 1. `tariff-catalog`

- 负责 `tools/genprices/`、`internal/usage/bundled_catalog_gen.go`、`model-prices.json`、`price-rules-v2.json` 和规则解码、校验、规范编码的对应文件及测试。延续旧 LiteLLM 固定输入和 bundled 补缺；打包带生效时间、限定域、来源及摘要的历史规则。由此稳定规则集格式，交给导入和解释器使用。
- 验收 L2。用固定价卡验证受支持期、时区边界、短长档、缓存时长、冲突、拒绝无源覆盖、旧格式可读和重复生成摘要。预置以 [需求](requirements.md) 为准，含 Enterprise USD 的 ChatGPT 登录 Codex 范围，2026-10-06 07:13 UTC 的审批审查免费公告、2026-10-11 23:48 UTC 的 GPT‑6.1 Sol 套餐长上下文免加价，以及套餐内 Ultrafast 6× 与合资格公开 API 促销 3×。分别登记选择域、公告生效证据与 `policy_id`，保留无法核实的账号切换和历史边界。

### 2. `pricing-facts-storage`

- 依赖 `tariff-catalog`。负责 `internal/store/migrations.go`、`internal/store/store.go`、`internal/usage/price_update.go` 和 `internal/usage` 内逐调用事实的解析、写入和读回。实现期间由本任务独占 `usage.go` 写入。规则表和逐调用可空证据使用同一段迁移 `P`。观察到的主线为 35，未交付的 v0.7.5 账号识别候选使用 36，JSONL 会话身份还有待合成分配的 core 迁移。主控记录实际合成基线后分配并复核 `P`。导入失败继续使用原版本；保留原事件键和旧库。只接入可证明的实际执行档、TTL 和已授权来源中可唯一关联的活动证据。
- 同一任务负责 `internal/desktop/serving_schema.go`、`apps/macos/AgentDeckShared/DesktopWire.swift` 的 core schema 读取边界、`apps/macos/AgentDeckVerification/main.swift`、`apps/macos/AgentDeckTests/DesktopWireTests.swift` 及匹配 fixture。新费率与事实使旧 serving 发布不能代表迁移后当前计价，`P` 签入的新读者拒收旧 schema 保留发布并通过既有刷新路径重建。已有 App Group v1 内容按现行策略留供旧金额读取，Widget Swift UI 和发布格式属于现有边界。
- 验收 L3。升级、旧 JSONL、同轮多调用、delta / last 不一致、重复扫描、崩溃重试、失败导入和备份恢复均有隔离断言。核对 Go/Swift 对新 schema、旧发布失效与高版本拒绝的矩阵一致，原生验证器和对应金标同候选可用。覆盖 Guardian 未采集的事实。

### 3. `request-pricing`

- 依赖 `pricing-facts-storage`。负责 `internal/usage/usage.go` 的价格选择、解释器、已准入事件的接线和相应测试。复用费率表、已有归因及原始事实的单次遍历。冻结逐请求 API 等价、可证渠道、范围参考和旧 v1 估算，以及 [架构约定的额度交接字段](architecture.md)。GPT‑6.1 Sol 的套餐长上下文豁免、API 长档及 Auto-review 免费分别按其适用条件核算。验证相同事实与规则下的重算结果一致。
- 验收 L2，触及并发或读取迁移时加 L3 的相关检查。使用 [需求验收表](requirements.md#可复算验收) 的金额和公告前后样本，覆盖审批审查免费／身份未知／API Key、套餐免加价与 API 长档、企业美元及各域速度规则。另验历史跨日界、未知实际档、缺 TTL、旧目录、空渠道和零价。固定逐笔输出再交报告消费与额度主题。旧 App Group 口径保留原有数据来源。

### 4. `pricing-reporting`

- 依赖 `request-pricing`。负责 `internal/usage/session_usage.go` 的会话和逐笔价格投影、`cmd/agentdeck/main.go` 和 `session_viewer_data.go` 的会话金额及 Tokens 页，及既有 usage、price 报告。负责 `internal/usage/presentation.go`、`internal/desktop/` 的应用端可选 `pricing` 投影、`desktop/fixtures/v1/`、菜单栏展示和现有 Swift 应用端解码。`activity_coverage_unavailable` 在选中过的期间的 `pricing.reasons[]` 去重一次。核对旧 JSON、诊断价格条件和历史刷新。任务 2 的 core schema 与 native 验证器在其候选验收后进入本任务基线。此任务先完成并验收会话金额与共享入口，再向额度主题 `quota-session-reporting` 交接准确接口和候选。
- Widget 接收原有 App Group v1 金额语义，展示和探索原型都在本任务交付范围外。兼容回放使用旧字段和升级样本，检查期间总额、客户端和质量行、趋势、日均和峰值等现有读取。如果新 UI 的公开 API 等价与旧读者倍率估算不同，验收核对各自原定来源。只有复现具体兼容回归才改其生产者责任路径。
- 验收 L2，原生 wire、发布失败及不同时点快照一致性按相关 L3 检查。覆盖正常 CLI、日期与模型筛选、旧数据重算、菜单栏诊断和已有 wire，原生 fixture 绑定隔离构建及快照。后续版本契约据此选择集成候选。

## 交接顺序

按表中依赖逐步推进。同一共享文件在可验收候选上只由一个实现任务持有。额度主题可先完成无迁移、只用夹具的窗口规则，写价表或额度表的持久层在实际合成候选上采用顺序编号；v0.7.5 的任何真实基线变更按版本契约复核。实施开始以本矩阵独立评审、CEv1 和 Beads 任务状态为准。Git 交付有单独授权边界。
