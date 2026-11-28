---
status: active
created: 2026-11-18
---

# 逐调用价格解析与历史重算

## 输入与存储

使用 [已有请求事件](../../../internal/usage/usage.go) 及其数值所有权。存储和重算按稳定 `event_key`，输出仍可按 turn、会话和期间分组。JSONL 累计快照转换、遥测准入和去重仍在计价前执行。会话总 token 无法反推出请求档位。

| 输入 | 来源 | 可确定范围 |
| --- | --- | --- |
| 时间、观测模型、普通/缓存/输出计数 | `usage_events`、现有源位置和读路径 | 以事件为粒度；Codex `cache_write_tokens` 是总量，旧列零不表达字段存在。 |
| 实际模型、计数存在性及 TTL | 已允许的 JSONL 或遥测中可唯一关联到原事件的结构字段。对支持的 OpenAI 响应使用 `usage.input_tokens_details.cache_write_tokens_details` 中 `ttl_30m_tokens`、`ttl_12h_tokens` | 原始模型、计数和键不变。Codex 原生只有写入总量时缺拆分，若该时期端点仅支持 30m 则证明该桶。支持多个 TTL 后，只有请求偏好或只有总量返回 `cache_write_ttl_unverified`。|
| 服务档、端点、认证、地区、配额类别 | 可关联的响应字段、已核验生产者契约，或提供同等证明力的已存观察 | 请求值单独保留。Fast 降级响应 `default` 按 Standard；仅有 `priority` 请求字段时可以给带假设的参考。官方路由名、当前账号和客户端套餐文案都不能证明历史支付渠道。[OpenAI Fast 规则](https://developers.openai.com/api/docs/guides/fast-mode) |
| 服务商归属和倍率 | 复用精确 run、有效的位置化 route、历史 timeline | 保持原归属质量与原因，倍率只应用到已覆盖的金额。 |
| 活动类别和免费资格 | 与 event key 一对一的已允许结构证据。供应商结构字段或已验证本地来源须明确说是 Approve-for-me Guardian 审批审查、GitHub PR 评审、`/review` 或 Codex Security | 仅有 `codex-auto-review` 名称或宽泛 subagent 标记为 `activity_unverified`。guardian 未采集时不生成用量或金额，展示 `activity_coverage_unavailable`。现有 `NeedsCollection` 只因 Memories 开启，本题的免费识别不启用新采集器。 |
| 拒绝或附加计量 | 已允许、可关联的类型、执行业务状态、计费数量 | 缺字段保留未知。当前 Anthropic 对部分输出前拒绝的计费政策按日期和上游类别适用，输出中途拒绝仍按条款计费。仅解析所需枚举和计数。[Claude 变更](https://platform.claude.com/docs/en/release-notes/overview) |

可补充的事实须携带来源、生产者版本、观测时间和关联理由。一个持久的可选逐事件表存放定价所需的白名单事实及字段存在性，随原事件清除。价格导入和该表不存原始会话、请求文本、账户凭据、原始工具参数或整段响应。已有备份、恢复、只读和 schema-ahead 路径按新格式验证；旧库读成缺证据。升级编号与同版本的其他数据迁移在合入前重新对账。

## 价卡和规则版本

沿用本地目录和显式导入入口。现有 `price_catalogs`、`model_prices` 和 v1 导入可继续读取。版本化规则表与目录版本一起整批验证和事务提交。原有平价目录作为可标明口径的参考；有可信适用期和完整选择器的旧条目可升级为历史条目。已有 `.json` 导入物仍支持旧格式。禁止静默重用内容不一致的版本 ID。

历史规则由仓库的 `internal/usage/price-rules-v2.json` 人工维护，逐条保存固定来源、有效日精度、选择器和数值。`tools/genprices` 在既有固定 LiteLLM 基础和 curated gap-fill 之外加载该文件，输出内置 `model-prices.json` 的 `pricing_rules` 版本化部分；同一输入按规则 ID 确定顺序。语义摘要包含历史规则内容，规则变更形成新 bundled 版本。`make check-prices-reproducible` 比对生成结果及摘要。`price update` 保留 v1 导入的参考语义，显式官方本地价卡导入接受带源和有效域的 v2 规则；整批验证失败时旧目录继续可用。`price list` 显示该查询的明确适用条目、区间与规则 ID，`price status` 和历史输出标明版本与来源。首发包包含需求表的已核实规则。[现有生成流程](../../../tools/genprices) · [Bundled 目录](../../../internal/usage/bundled_catalog_gen.go)

每条规则有稳定 ID、目录修订或内容摘要、原始 URL、核验时间、币种和单位、有效区间及精度，并分别列明模型/别名、渠道、端点、服务档、地区、阈值及计数轴、活动类别、TTL 和各费率。公开 API、ChatGPT 套餐、Enterprise USD 使用不同的选择域。同一规则可声明适用一个组件或完整的一组；只补共同适用期和选择域里有明确依据的组件。互相冲突的同域规则需要明确证据或替代关系。

相同选择域出现 bundled 与导入 v2 规则时按已核验适用期和来源证明力比较。导入先后不能独立确立费率。具有明确替代关系或适用期更精确的官方记录优先；同级、同域和同时间冲突时为缺价并报告可见原因。

来源优先级为厂商带日期的规则与可核验历史价卡、列明适用期的官方覆盖、满足同样证据条件的固定版本目录。没有可信生效日期的资料只能从观察时刻作为参考。当前 LiteLLM 和 models.dev 可发现候选及辅助核对。models.dev v1 的 `tiers` 可以描述阈值，`context_over_200k` 字段名不足以证明实际阈值，单个 `cache_write` 不能表达两种 TTL。其 `release_date` / `last_updated` 与 AgentDeck bundled 统一生效标记均不能承担历史价卡日期。[models.dev 数据](https://models.dev/api.json) · [bundled 生效标记](../../../internal/usage/bundled_catalog_gen.go)

全部精确区间使用 `[from,to)`。厂家只声明刊登日时保留原日期和精度，未知切换窗内若存在不同费率则限于有证范围。当前公告的促销结束后由有效的新规则接续；无新费率时各受影响组件显示缺项。Tibo 公告作为政策已被公开确认的节点，早期时间与单账号放量的差异需独立证据。维护者冻结用户已同意的两个订阅政策样本和官方来源。必测的历史价格变更是 GPT‑5.6 Luna 和 Terra 的 2026-07-30 降价、GPT‑5.6 Sol 的 2026-08-21 变价（促销）、Claude Opus/Sonnet 4.6 的 2026-03-13 长上下文溢价取消及 Sonnet 5.5 的 2026-10-07 缓存读降价。[OpenAI 更新](https://developers.openai.com/api/docs/changelog) · [Claude 1M 公告](https://claude.com/resources/articles/1m-context-ga) · [Claude 更新](https://platform.claude.com/docs/en/release-notes/overview)

## 统一解释器

1. 在一次读取固定事实代次、规则知识截止、规则摘要和解释器版本。
2. 根据实际模型证据解析精确 ID，随后才使用可证的日期与渠道别名。当前模型选择、字符串前缀和首次观测日期不足以建立映射。
3. 枚举源证据支持的价卡选择。Codex 档位输入使用同一事件的有效 `last_token_usage.input_tokens`，或可唯一绑定的单请求响应输入。已有累计差值保持计量事实。若 delta 与最近调用在共同报告的计价组件不相等，记 `invocation_boundary_unverified`，缺乏可证边界的数量只保留已知适用范围或参考。现行首笔或 reset 明确使用有效 `last` 的分支仍可确立单次调用。`>` 与 `>=` 显式，整请求切档应用于规则的各组件。真实分段规则才按段拆分。5.4/5.5 价卡 `full session` 的账单单位未证时，候选档不同的金额标 `context_policy_unverified`，可以展示显式的单请求参考。例如 GPT‑5.4 已知单笔 272,001 输入，同时只能用该歧义规则时，精确金额为空，较早调用保留各自的证据与同段歧义，参考假设不会写回它们的口径。
4. 普通输入、缓存读、写入 TTL 桶和输出互斥。OpenAI 的普通输入为输入总量减读和适用写入量。Claude 使用普通输入加互斥的缓存读写来判长档。Claude 旧日志仅有缓存写总量时允许带标签的 5m 参考；部分 TTL 拆分按实际能证的桶计算，余数待定。出现负数、分项超出总计、档位与端点矛盾时返回结构化原因。
5. 使用各组件发布费率计算 `(count × rate) / 1,000,000`，附加计量按其单位单算。只有官方规则明确允许时才叠加处理模式、地区和折扣。保留精确十进制分量与汇总，输出保留已有字符串金额格式。
6. 各自计算公开 API 等价、渠道估算、套餐状态及配额适用性。公开价命中零、套餐内现金额零、安全审查额度零和无报价使用四种证据。依据适用域识别免费，不从目录里无效的订阅零费率行推断。

缺组件用部分已知或未知。能用完备且相关的候选集合界定上下界时可提供范围。若只有服务档缺失而其他条件齐全，可附 `standard_reference`，保留实际档未知。时间不匹配的整个现价卡可生成明确注明参考日期的值；该值及未完全可证的金额不进入精确历史合计和配额估值认可分子。

## 消费、更新和重算

摘要、会话、统计、调用详情和 `desktop snapshot` 共用逐调用输入、归因及 Go 计价解释。旧读者的 `catalog_base_cost`、`provider_cost`、`known_*` 和定价覆盖字段保留现有类型与语义。旧 `provider_cost` 的计算仍为目录估价乘可归属倍率，仍依赖既有归因和覆盖条件。新结果中的 `pricing.api_equivalent` 使用有证据的公开 API 价。`pricing.provider_estimate` 仅代表可证的货币渠道，个人订阅内含或渠道未知时为空。新结果缺值不会覆盖旧估算字段。两个口径共享同一已存请求集合、已有归因解析和读快照，费用结果在同一请求遍历时派生。

CLI 和菜单栏的主金额按需求表的 API 口径优先级显示。金额排序、峰值及占比只用相同币种、完全可比且至少一项非零的完整 API 等价。其余情况保留 token 顺序和诊断，金额峰值及占比为空。现有读者继续按原金额视图解释其字段。

### JSON 与原生 wire

在现有金额记录内加可选 `pricing` 对象。共用 `version=1`、`rule_set`（摘要）、`api_equivalent`、可空 `provider_estimate`、可空 `reference`、`reasons[]`。金额对象 `currency` 为 ISO 4217，`status` 为 `complete` / `partial` / `unavailable`，可空 `amount` 及 `known_amount` 为十进制字符串。`reference` 包含 `basis`、币种、可空 `as_of`、`amount` 或同时使用的 `lower` / `upper`。结构化参数使用可空 `tier`、`context_unit`、`ttl_candidates[]`，与 `basis` 相符。主视图使用完整、已知、同口径全量参考、不可用顺序。区间及无法覆盖整段的参考放详情。

| `reference.basis` | 语义与参数 |
| --- | --- |
| `standard` | 其他条件明确，执行档未确认。参考 `tier=standard`。 |
| `as_of_date` | 整套价格对齐 `as_of` 日期，针对缺历史价的观察。 |
| `legacy_catalog` | 已有旧目录的可重放参考，通过 `rule_set` 关联。 |
| `assumed_5m` | Claude 缺写 TTL 时的 5m 参考。 |
| `cache_write_ttl_range` | 有明确档但 TTL 拆分缺失，使用给定 `ttl_candidates[]`、`tier` 和价卡求上下限。 |
| `per_request_context` | 按单请求换档的参考，`context_unit=request`。用于上游提到 `full session` 而作用范围未确认的情况。 |
| `requested_tier` | 只观察到请求档，按该 `tier` 计算参考并保留执行档未知。 |

详情增加可空 `observed_model`、`priced_model`、`requested_tier`、`executed_tier`（`standard` / `fast` / `ultrafast` / `batch` / `flex` / `background_2h` / `background_8h` / `unknown`），`context_tier`（`short` / `long` / `flat` / `unknown`），以及 `components[]`。组件项包含 `kind`、非负 `quantity`、`unit`、可空 `rate` / `amount`、`rule_id` 和可用的 `source_url`、`effective_from` / `effective_until`。完成计价时可同时说明渠道独立缺项。稳定缺项原因包括 `model_unresolved`、`historical_rule_missing`、`effective_boundary_unverified`、`service_tier_unverified`、`context_policy_unverified`、`invocation_boundary_unverified`、`cache_write_ttl_unverified`、`component_unmetered`、`conflicting_counts`、`provider_route_unverified`、`activity_unverified` 和 `activity_coverage_unavailable`。未知模型、价或条件保持未报告的金额和值域。

`activity_coverage_unavailable` 是期间级采集提示。查询范围有已准入 Codex 请求且相应生产者的 Guardian 采集覆盖未能证实时发出一次。单独的 Claude、无 Codex 记录和已证实完整覆盖的选中范围不发出。提示只声明该期间 Codex 审批审查用量可能缺失，不能由当前配置推断历史是否启用，且不生成调用或修改该范围已观察请求的价格覆盖。CLI 的 `Coverage` 与菜单栏诊断位各显示一次简短说明。

`pricing.subscription` 可选，包含 `channel`（`api_key` / `third_party` / `personal_subscription` / `enterprise_usd` / `unknown`）、`status`（`included` / `free_activity` / `metered` / `unknown`）、`activity`（`guardian_approval` / `github_review` / `local_review` / `codex_security` / `ordinary` / `unknown`）、`quota_eligibility`（`eligible` / `excluded` / `unknown`）和对应 `quota_reason`（`included_observed` / `guardian_free` / `paid_channel` / `policy_unverified` / `activity_unverified`）。`subscription_long_context` 为 `exempt` / `applicable` / `not_applicable` / `unknown`，可空 `policy_id` 指向证据。所有金额只反映自身口径，不因一个标签替代其他计量。

桌面 wire 维持 v1，应用端增加可选 `pricing` 投影，使用现有客户端、期间和分组键进行对应。该投影在旧 `usage` 之外，App Group publisher 继续消费既有 v1 用量结构。桌面读者收到旧 payload 时采用原展示。`desktop/fixtures/v1` 保留旧价金标，并为应用端新字段增加完整、部分、参考、不可用与零价样本。[Desktop Wire](../../../apps/macos/AgentDeckShared/DesktopWire.swift) · [App Group](../../../apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift)

Widget 的验收仅回放既有快照契约。重点覆盖 `pricing.provider_estimate=null` 的订阅用量，核对 App Group v1 的 `provider_cost`、`known_provider_cost` 和 `pricing_complete` 仍同一旧口径流入现有格式化路径。比较原、升级候选对同一旧金额样本的输出，避免错误的空值传递、零值或替换口径。只有观察到代码候选回归才修复对应的生产者兼容路径，原有 Swift Widget 展示与排序属于既有契约。[金额读取](../../../apps/macos/AgentDeckWidget/WidgetViews.swift#L914)

使用现有 derived snapshot generation、数据库一致读及发布边界，使一次报告的规则、事实、筛选和日期桶保持一致。价卡更新或补证前完整校验，事务提交后失效相关派生代次。发布前核对版本向量；失败保留上一可读取代并标其截止与状态，首次失败为空状态。会话和历史用量的计价升级直接消费已存事实；只有现有来源可提供所需且唯一的补证时才定向读取。重试用原键去重。首次生成、并发导入、备份恢复、取消和重新启动需要差异夹具。

向 `subscription-quota-conversion` 提供 `client`、调用键、会话键、请求时刻、完整 API 等价或其缺项、币种、规则版本、渠道、活动和 `quota_eligibility` / `quota_reason`、`executed_tier`、`subscription_long_context`、适用 `policy_id`，及可空 `standard` 参考或可证组件依据。Standard 参考仍带独立口径和证据。额度主题只读取这些输出与自己采集的账号/窗口观察，不重算价格。已证实的豁免仅改变额度侧适用性和解释，不改变公开 API 金额。

## 影响与验证

L2 覆盖逐调用价格、CLI/桌面兼容、半价/切档/不支持组合、最新和历史价格源、请求证据与响应档降级。L3 覆盖迁移、备份恢复、发布原子性和隐私。用 [requirements.md](requirements.md) 的边界数字作为固定值，扩充早于缓存写入字段、混合 TTL、共享会话重算、边界日、订阅豁免与 API Key 交叉用例。有效原始计数在重复升级前后逐项相同。只在隔离状态跑真实生产者或原生呈现验证。
