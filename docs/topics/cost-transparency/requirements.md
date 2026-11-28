---
status: active
created: 2026-11-18
---

# 按请求发生时的 API 价核算用量

## 目标与来源

本主题承接 `ad-cost-transparency`，修订 v0.8.0 已有的 Codex 和 Claude API 等价计价。用户在 T3 编排线程确认计价场景、历史重算和补充的两条 Codex 套餐政策后授权继续。`ad-subscription-quota-conversion` 使用本主题的逐调用结果估算额度窗口价值。

用户应能复算一笔调用、会话或期间的金额，知道用了哪些 token、费率版本、执行条件和来源，并能找到导致金额缺失或者发生变化的原因。当前读路径在历史条目缺价时以现价补齐而未标识。Codex 用量已可记录缓存写总数，现行公式只从原始输入中扣除缓存读取。[现行价格解析](../../../internal/usage/usage.go) · [现行目录契约](../../specs/cli-design.md#price-catalog)

## 计价口径

| 结果 | 定义 |
| --- | --- |
| 公开 API 等价 | 用请求发生时适用的第一方 API 费率及有证据的模型、执行条件计算。可独立于实际付费渠道取得。明确的 Standard 或指定日期参考另带假设。 |
| 渠道估算 | 已证实的 API Key 按适用费率，第三方沿用当时可归属的路由和倍率快照，Enterprise 美元按量使用与当时协议相符的专用价卡。ChatGPT 或 Claude 个人套餐内含、超额类型不明、仅证实官方路由时 `pricing.provider_estimate` 为空。内含与确证免费的额外费用零只在套餐状态表达。旧 wire 的 `provider_cost` 表示原有倍率估算，继续供依赖该口径的现有读者使用。 |
| 套餐状态 | 已证实的内含、活动免费或额外计量。仅能证实登录方式时，保留内含及超额状态未知。已结算金额须有同范围收据。 |
| 计价覆盖 | 完整金额、部分已知或未知，附组件和原因。零价需有明确的当前计算口径及适用证据。 |

每一笔独立完成的模型请求分别计价。同轮多次调用、模型和速度字样相同的不同请求各自匹配。JSONL 和遥测重叠遵从既有数值所有权，重算只消费已准入事实。原事件键、事件时间、会话、token 和路由观察保留。

CLI、菜单栏用公开 API 等价作为用量金额的主口径，渠道估算在详情单列。依序展示完整金额、带 `已知` 标签的已知小计、同一口径覆盖整段的参考金额，最后为不可用。参考必须标明标准档、指定日期、历史旧目录或 TTL 假设。区间显示于详情。历史金额在升级后可能变化或变为空，这是新口径的预期。旧版 Claude 缺 TTL 的 5m 默认金额保留为标记参考；来源确证只能是 5m 时仍可按确定组件计。

## 决定金额的条件

- **身份和时间。** 保留观测原名，优先使用可证的实际模型。显式别名、预览和 A/B 映射须覆盖发生时间、渠道和适用组。时间精度不足以确定费率边界时，只有可证的共同部分或区间可公开。
- **输入形式。** 按供应商定义计普通输入、缓存读、各 TTL 写和输出。OpenAI 有独立写入价的部分替代普通输入价。Claude 缓存读写是独立计数；已经计入输出的推理 token 只计一次。旧版日志缺失字段和观察到的零分开。
- **上下文。** 按单请求和该价卡的输入定义判断。Astra 在超过 272,000 时整次请求换档，Haiku 5.5 在超过 100,000 时整次请求换档。历史 Claude 超过 200,000 的 beta 策略以及恢复标准价各用自己的适用期。费率规则需要明示阈值相等、整次切档或分段的语义。
- **执行档和费率范围。** 按可证的执行档、端点、模型及地域选价。Standard、Fast/Priority、Ultrafast、Batch、Flex 及后台折扣各自匹配。供应商响应已降级时用实际档；只有发起时偏好时标为未确认。以发布价卡组件为准，按有依据的组合应用倍率。保留促销起止及厂商原时区。
- **请求结果和独立计量。** 只有文档认可的失败、拒绝、免费期或工具计量才能改变渠道费用或附加费。可以取得的 token 金额仍独立计算。仅有本地工具名、对话文本或请求错误时，无法证明额外计费单位。

## Codex 套餐与评审

| 变更 | 适用范围 | 生效证据与结果 |
| --- | --- | --- |
| Auto-review 免费 | ChatGPT 登录的 **Approve for me** 安全自动审查，身份和活动类别均可核实时适用 | 2026-10-06 已公开确认，额外收费 $0，额度占用为零。底层模型可以缺价，API 等价独立求值。[Tibo 公告](https://x.com/thsottiaux/status/2107368734981517634) · [官方帮助](https://help.openai.com/en/articles/11369540-using-codex-with-your-chatgpt-plan) |
| 取消长上下文加价 | Codex 或 Work 的 GPT‑6.1 Sol，且用量确实来自订阅内含额度 | 2026-10-11 已公开确认，同期默认上下文由 272K 提至 500K。只移除额外上下文倍率，实际消耗仍由窗口观测。API Key 超过 272K 仍用 API 长档。[Tibo 公告](https://x.com/thsottiaux/status/2109430894604026039) · [模型价卡](https://developers.openai.com/api/docs/models/gpt-6.1-sol) · [套餐说明](https://learn.chatgpt.com/docs/pricing#included-usage) |
| 企业 USD | 适用 Enterprise 美元价卡且使用 ChatGPT 登录 Codex | 现行价卡对该通道的所有 Codex 模型免上下文加价，并无独立缓存写费。个人套餐中其余模型的免加价资格需要各自证据。[Enterprise 价卡](https://help.openai.com/en/articles/20001415-chatgpt-rate-card-enterprise-token-based-pricing) |

两则公告的存档时间为 2026-10-06 07:13 UTC 和 2026-10-11 23:48 UTC。公告时间证明当时已宣布政策，无法单独识别某个账号首次适用的时刻。前序调用按有证历史待遇处理。API 等价使用该模型的 API 条款。计划内速度倍率与 API 速度费率分开，例如 GPT‑6.2 Astra 和 Sol 的套餐内 Ultrafast 规则为 6×，合资格公开 API 促销期为 3×。[公告存档](https://airesets.dev/receipts/) · [速度政策](https://learn.chatgpt.com/docs/pricing#speed-multipliers)

`auto-code-review`、`codex-auto-review`、`slim` 单独出现时只证明观测名称。2026-04-30 的报告描述当时的 GPT‑5.4 Thinking；已确认包含 [Codex 的 Luna 选择提交](https://github.com/openai/codex/commit/c4f42d161ae44a8d696ee9fb595709661979d187) 的客户端在相应 API Key 路径使用 `gpt-5.6-luna`。该提交日期不代替用户客户端版本，也不证明 ChatGPT 后端映射。Approve for me 审批审查、GitHub PR 评审、本地 `/review` 和 Codex Security 分别保留活动依据。免费须有可唯一关联的类型化审批审查字段及登录证据。只有名字，或现有来源未收集 guardian 用量时，保留活动未知或覆盖缺口。现行遥测开关仅为 Memories 自动启用。[审查报告](https://alignment.openai.com/auto-review/) · [代码评审适用范围](https://learn.chatgpt.com/docs/pricing) · [采集边界](../../../internal/telemetry/codexconfig.go)

## 历史重算与产品边界

- 以已存逐请求事实、路由快照和同一规则版本重算。已有的当前目录和 bundled 通用日期只能支持参考值。带明确历史覆盖的正式价卡进入历史结果；用新知识修正旧价格时保留来源和生效精度。
- 缺字段时只从现有允许的来源提取可唯一关联到原事件的安全事实。不能复原时保留原 token、已知小计和原因。只有会话合计时，不能还原按请求非线性的上下文档。
- 升级后摘要、会话、统计、菜单栏的计价结果须引用一致且完整发布的代次。价格更新不应因无关源文件触发全量重扫。重算中断可重试。
- Widget 在本版作为旧金额读者接受兼容验证。沿用 `provider_cost` 的既有语义和 App Group v1 契约。带已知金额的现有订阅用量在新结果没有渠道估算时，旧读者仍得到原口径输出。用代码候选的快照回放验证；复现回归时只做恢复兼容所需的修复。Widget 界面和原型探索属于独立范围。
- 复用现有价格导入、CLI 和本地派生展示。升级后本地派生重算属于本主题。套餐积分换算、账单服务接入、上游客户端修改和新提供商支持属于其他任务。开发验收使用隔离状态，实际安装或操作用户当前状态需要单独授权。

## 本版覆盖与历次范围

| 历史规划项 | v0.8.0 处置 |
| --- | --- |
| models.dev 作为目录 | 2026-11-18 用户重开价源选择，并在核对官方规则为主、固定目录补充的设计与两条套餐政策补充后确认继续。采用该设计。models.dev v1 可校验 context tiers，单写入价和无历史价边界的条目用作参考。 |
| LiteLLM 错价及 GPT‑5.6 Sol | 保留固定目录复现旧结果并用官方价逐请求验收。Sol 必须包含首发与 2026-08-21 变价。Luna、Terra 包含 2026-07-30 变价。缺公告时区的边界仅使用确证范围。 |
| `auto-code-review`、实验别名 | 采用已证实的活动、时间、渠道与实际模型，缺映射时保留原名及未知。 |
| Context Efficiency | `ad-context-efficiency` 在 backlog。它负责效率诊断，本版只核算已记录的计费用量。 |
| 积分和额度 | 用户撤销 credits 金额换算。额度仅由 `ad-subscription-quota-conversion` 用独立观测的百分点估算。 |

首发历史规则必须覆盖有可复核价源的 GPT‑5.6 Sol/Terra/Luna 变价前后、GPT‑6.1 Sol、GPT‑6.2 Astra/Sol，以及 Anthropic Haiku 5.5、Opus/Sonnet 4.6 的 1M GA 边界和 Sonnet 5.5 缓存读变价。规则只有当前报价时注明覆盖起点；缺历史证据打开缺口并保留相应参考。发布验收需逐项核对纳入的价卡与历史数据覆盖，不能用补零满足覆盖。[OpenAI 价格变更](https://developers.openai.com/api/docs/changelog) · [Claude 价格变更](https://platform.claude.com/docs/en/release-notes/overview)

## 可复算验收

下列公开价卡均在 2026-11-18 核验。单价单位为 USD / 百万 token，阈值相等使用低档。

| 模型和价卡 | 普通输入 | 缓存读 | 缓存写 | 输出 |
| --- | ---: | ---: | --- | ---: |
| Astra Standard 输入 ≤272,000 | 8 | 0.40 | 30m 为 10，12h 为 20 | 40 |
| Astra Standard 输入 >272,000，整次调用 | 16 | 0.80 | 30m 为 20，12h 为 40 | 60 |
| Haiku 5.5 输入 ≤100,000 含读写 | 0.10 | 0.01 | 5m 为 0.125，1h 为 0.20 | 0.50 |
| Haiku 5.5 输入 >100,000 含读写，整次调用 | 0.50 | 0.05 | 5m 为 0.625，1h 为 1 | 2.50 |
| GPT‑6.1 Sol Standard 输入 >272,000 | 4 | 0.20 | 30m 为 5，12h 为 10 | 15 |
| Astra Enterprise USD 签入 Codex Standard | 8 | 0.40 | 不另计 | 40 |

[Astra API](https://developers.openai.com/api/docs/models/gpt-6.2-astra) · [OpenAI 缓存](https://developers.openai.com/api/docs/guides/prompt-caching) · [Claude 价卡](https://platform.claude.com/docs/en/about-claude/pricing) · [Sol API](https://developers.openai.com/api/docs/models/gpt-6.1-sol) · [Enterprise USD](https://help.openai.com/en/articles/20001415-chatgpt-rate-card-enterprise-token-based-pricing)

| 样本 | 预期 |
| --- | --- |
| Astra Standard，272,000 输入含 100,000 缓存读、20,000 次 30m 写，另有 10,000 输出 | `$1.856`；只加一个普通输入 token 后 `$3.512016`。同一超阈值样本适用现行 Enterprise USD Codex Standard 条款时为 `$1.816008`。 |
| Haiku 5.5，49,000 普通输入、40,000 缓存读、10,000 次 5m 写、1,000 次 1h 写和 5,000 输出 | `$0.00925`；只加一个普通输入 token 后 `$0.0462505`。 |
| GPT‑6.1 Sol Standard，300,000 未缓存输入、10,000 输出 | 公开 API 等价 `$1.35`。确认订阅内含且处于免加价政策时，额度侧仅免长上下文额外倍率。 |
| 2026-10-06 公告前后、安全审查和同名未确认活动 | 历史未证、套餐内安全审查零额外费用、仅名称未定，以及 API Key 正常计量均可分辨。 |
| 历史目录、旧日志、损坏导入、刷新中断及重复升级 | 不引入重复调用，不用今日费率充作旧价，前后解释可追溯，稳定的既有读者可解码，失效缓存和恢复符合架构。 |
