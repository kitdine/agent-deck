---
status: active
created: 2026-11-15
updated: 2026-11-15
---

# OTel 版本兼容计量与自动分类架构

## 范围与基线

AgentDeck HEAD 为 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692。
本稿响应 epoch 9 的 AgentDeck-only、自动可用要求，取代 epoch 5 producer 合同设计。
计量和用途独立处理，不更改 Codex、用户生产配置或现用 binary。
core schema 35 只扩展原因码约束与来源 thread_id，不新增表或通用规则引擎。

## 1. 版本兼容准入

唯一定义位于 internal/store，供 telemetry Go 判定和历史补计 SQL 共享生成。
家族为 codex_exec/codex_exec、codex-app-server/codex-tui，分别对应 producer/originator。
形状为 sse、codex.sse_event、response.completed。版本非空，长度使用既有解码字段上限，
不能含分隔符 /；不引入 semver 比较、版本列表或另一个更小长度上限。
SupportEntry.Name 仍带实际版本。Record 字段和 Key 字节保持不变。

Observation 顺序、无计数、不完整、无效、缺 thread、计数冲突行为保持。
producer_unsupported 继续涵盖家族/transport/形状不支持，能力判定使用现有路径。
Go 与 SQL 用同一组反例验证等价，身份和分类规则不能放宽计数校验。

## 2. JSONL 线程证据

现行 accounting session_id 命中、保存的来源 thread_id、规范路径解析及同线程既有
jsonl_owned 观察均为正向证据。任何命中先于 OTel 所有权晋升。

### 规范路径

单一 CodexRolloutThreadID 解析 basename。要求 rollout- 前缀、.jsonl 后缀，时间部分
为 YYYY-MM-DDThh-mm-ss 的 19 字符形状，随后为连字符和 UUID，或 UUID_UUID。
取第一个 UUID，校验 36 字符、连字符位置和十六进制，统一小写比较。
公开 rust-v0.169.0 的 rollout_file_name.rs 与 recorder.rs 是格式来源，文件名使用 ThreadId。
不能取最后 36 位；归档目录不影响 basename。不符合格式不提供路径证据。

### 元数据与来源列

usage_source_files.thread_id 为可空文本。NULL 表示尚未成功评估；空串表示成功评估后
确实不含线程身份或非 Codex 来源；UUID 表示文件首个 session_meta 的 payload.id。
增量追加沿用已保存值，rewrite/reset 从新来源重新取得，继承的后续 session_meta 不覆盖
文件自身首个身份。规范文件名与首个元数据 ID 均作为保守正向证据，矛盾不据此扩大所有权。

正常解析将首个身份随 prepared source 传递，在 checkpoint 的既有事务写入该列。
publishFacts 显式接收本批来源线程身份，在 cold 分支之前调用同事务撤销函数，
覆盖普通、cold bulk 和每个 512 行前缀块。函数仅处理 pending/otel_owned，不改变
invalid/conflict/capture_only/unmetered 或 JSONL 计数。撤销复用 store 已有贡献删除与
syncTelemetrySession，原 accounting-ID trigger 保留。

### 历史补填与前缀窗口

正常扫描对尚未评估来源分批补填，每批最多 64 来源，每来源头部读取最多 1 MiB。
首次 metadata 只读到完整记录后采用，遵守现有 captured identity/range 校验；源变化则重试。
可读的改名副本因此受保护。Claude 来源可直接标空；读取失败、源身份变化或记录超限
保持 NULL，不能标为空串。缺失文件若具有规范文件名可保存该正向 ID。
补填与已有 OTel 撤销同事务；无 parser version 全量重读。成功批次持久保存，不因后续批次失败回滚。

每轮有 pending/otel_owned 时构建一次线程证据集合，包含 usage_source_files 的列/路径，
以及有保留 Codex 事实却无 checkpoint 的 source_path（复用 orphan 查询形态）。
有未成功评估的 Codex 来源且不能从规范路径证明身份时，不晋升 pending。
存在未完成 checkpoint 的已提交 Codex 前缀时也不晋升，直到正常扫描完成或 orphan 清理。
这防止改名来源在前缀已提交、最终身份行未落库期间接收新 OTel 观察后双计。
已有 owned 的复查、JSONL 计量和精确撤销不因该门槛停止。

### 首次结算完成检查

调用 SettleTelemetry 前，来源补填协调入口捕获当前待评估来源的有限扫描上界，按稳定
keyset 连续处理 64 行一批，直至走完此集合。新来源正常写入已知 thread_id；并发来源变化
使本次完成检查失败或进入下一轮。不得用每批完成代替整体完成。
每批事务保存身份和精确撤销；重启重新查询 NULL，已成功来源不会重复读写。
本次扫描游标越过读取失败来源，继续处理后续可恢复来源；失败来源保留 NULL，结束时仍
返回未完成。下次正常触发重新尝试，避免在同次调用中无限重试一个失败来源。
取消立即停止后续批次并返回未完成，已提交批次有效。

store 的结算事务在 readmission/晋升/ensureContributions 前重新检查未评估 Codex 来源与
未完成 checkpoint 前缀。未满足则返回明确的身份准备未完成错误，不执行新晋升或 owned
贡献恢复，不把该次算作成功结算。精确 JSONL 撤销继续由每批补填和 publishFacts 提交，
不会仅删金额后允许 ensureContributions 把重复恢复。

receiver 启动与周期、CLI import、scan/rebuild 的 SettleAndProject 全部接入相同准备入口。
计量层通过调用方注入准备回调或在既有上层协调，不反向导入 usage；直接调用 store 的路径
同样受事务内检查保护。初次迁移的 65 个来源至少需要两批，全部可恢复时随后结算成功。
CLI 未完成返回明确结果/错误而非成功回执；receiver 沿用现有监听诊断错误槽记录固定原因，
后台下一周期继续。scan/serving 使用既有 incomplete/partial 状态，保留最后成功数据，
不将本轮未完成结果发布为完成。恢复成功后清理该临时诊断并发布修正后的历史金额。
不新增配置要求、表或通用队列。

来源移除时沿用现有事实/来源清理，不主动清除用户事实。已有 jsonl_owned 观察保持粘性，
同线程新观察仍不再产生 OTel 贡献。无法恢复的不规范遗失来源保留真实的未评估状态，
由正常重试/清理处理，不以缺席伪造排他资格。此边界影响数字，不使已分类会话回到 unknown。

## 3. 自动历史补计

SettleTelemetry 在 retractIncompleteTelemetry 之后，同一事务内每轮最多重评估 2000 条
capture_only/producer_unsupported、metered=1、计数完整且家族形状匹配的行。
thread 非空则置 pending/ownership_not_settled 并写实际 support_entry；空 thread 仅改为
conversation_missing。条件由准入唯一定义生成，不合格行不占候选额度，处理后离开集合。
只改状态/原因/support_entry，保留原 key、计数和时间。

后续复用现有 10 分钟、inventory、JSONL 排他、贡献生成和分区失效逻辑。现有结算入口
receiver、import、SettleAndProject 都带到此步骤。readmission LIMIT 不宣称整个已有 settle
循环有界。对存量 otel_owned 同样复查新线程证据，首次成功完成结算前撤销全部已恢复身份对应的重复。
历史贡献落在原事件日，Changed 沿用既有投影/发布路径。

## 4. 自动规则集

internal/sessionorigin 保存小型有序规则表，每条有稳定版本 ID、优先级、匹配、结果与依据。
ruleVersions 从规则表生成，reducer 升为 origin-reducer-v2，使存量自动重算。
不引入 DSL、插件或用户写规则。兜底是结果，不能作为 work 证据与每个 auxiliary 冲突。

| 顺序 | 规则 | 条件 | 结果及性质 |
| --- | --- | --- | --- |
| 1 | claude-mem-sdk-map-v1 | 可选 adapter 启用且精确映射命中 | auxiliary，已有证据 |
| 2 | codex-otel-tui-background-v1 | OTel 所有、合法 TUI 家族条目、任意版本 | auxiliary，明确推断 |
| 3 | codex-otel-exec-work-v1 | OTel 所有、合法 exec 家族条目、任意版本 | work，保守兜底 |
| 4 | default-work-v1 | 可读会话无其他结论 | work，默认 |

有效 override 最后覆盖自动结果，保持当前绑定与优先级。旧 codex-otel-background-v1
从活动 ruleVersions 退役，重算旧自动证据；0.165 exec 归 work，TUI 保持 auxiliary，
不保留精确版本特例，override 不变。

归约先拒绝旧来源代次证据，输出 work/origin_input_mismatch；明确冲突或 auxiliary 与
真实 work 证据并存为 work/conflicting_origin；有效 auxiliary 为 auxiliary/maintenance_origin；
已启用 adapter 不可读为 work/origin_source_unavailable；无法解析规则为 work/origin_rule_unsupported；
其余依次落 exec-work 或 default-work，原因 default_work。TaskChild 仍无法确定用途时为 work。
尚无派生结果读取为 unknown/origin_metadata_missing，只表示尚未分类，下一正常扫描自动处理。

新增 default_work；已有 conflicting_origin、origin_source_unavailable、origin_rule_unsupported、
origin_input_mismatch、task_child_unconfirmed 的新派生类别为 work。覆盖缺口仍按原因码
判断，partial 不因默认 work 消失，不能谎称读取成功。列表不新增诊断噪声。
TUI 无 rollout 主会话可能被推断辅助；exec Memories 会显示工作。规则依据诚实呈现这些局限。

## 5. 自动刷新与来源状态

正常 scan/rebuild 的 sessionorigin.Reconcile 保留。telemetry 不直接依赖分类包；
由 cmd/agentdeck 服务和 CLI import 接线注入分类回调，在索引投影成功后执行。
失败沿用 projectionPending 重试。扫描在会话域成功时先完成分类再发布，usage 已成功提交时，
会话索引或分类失败仍允许用量派生缓存及 serving 发布。读取端继续用输入 token 和覆盖诊断
表达分类缺口，会话域错误保留。数值身份 pending 时仍归约已可读会话，再报告 pending。
规则升级、历史补计、迟到撤销和 override 清除均自动重算，不增加必做用户命令。
前台 watch 的 session 域在身份 pending 时先完成可读会话分类，再发出 schema_version=1 的
scan_pending 事件，reason=source_identity_pending、skipped=false，changes 为已提交的域变化数。
该域检查点不确认，下一轮询自动重试，其他域可继续；恢复后发出 scan_completed 并确认检查点。
普通不可恢复扫描错误保持既有错误传播。文本模式明确显示 pending 与自动重试。
会话检查点同时绑定索引 epoch、来源指纹和活动分类规则版本。旧检查点或规则升级自动触发
一次正常分类刷新，scanruntime 与 watch 共用编码，避免未变来源掩盖规则更新。

清除覆盖时索引不存在仍可完成 core-only clear；其他索引打开或分类刷新错误返回
classification_refresh_pending，保留已提交回执，后续正常刷新重试。

source status 的 Codex 行保留 source=codex-otel-background，background_sessions 只统计规则2
命中的不同会话；新增 otel_owned_sessions 为全部 OTel 所有会话数。store 按 support_entry
分组只读统计，分类筛选复用同一规则。rule_version 为产生 background_sessions 的规则ID。
顶层 builtin_rules 按确定优先级列出内置规则ID；不通过版本字典序猜“最新规则”。

新增字段保持既有字段语义；Swift transport 接受并验证新字段。读旧 helper 时新增字段缺省，
不伪造计数；既有 schema34只读读取仍可用。非启用的可选 adapter 不影响内置规则就绪，
已启用 adapter 的实际读取失败保留 partial。core/config/receiver 其他既有读失败仍按原合同处理。
telemetry status 的历史 JSON background_sessions 键保留总 OTel 会话数兼容语义，人读改为
OTel-owned sessions counted；source status 的 background_sessions 则是分类推断数。

## 6. 表面与文案合同

沿用既有 session 列表、详情、Settings 来源行和 CLI 结构。中文/英文文案如下。

| 状态 | 中文 | English |
| --- | --- | --- |
| 默认工作 | 默认归为工作 | Work by default |
| 尚未归类 | 等待自动分类 | Waiting for automatic classification |
| 内置规则就绪 | 自动分类已启用 | Automatic classification enabled |
| 可选 adapter 未配置 | 可选增强未启用 | Optional enhancement not enabled |
| adapter 已关闭 | 可选增强已关闭 | Optional enhancement disabled |
| 启用后读取失败 | 增强来源暂不可读；会话保留在工作中 | Enhancement unavailable; sessions remain in Work |

default_work 及其规则ID出现在详情/诊断，普通列表只显示工作/辅助。人工 override 保留为可选
纠正。现有 source_not_configured JSON 原因保留，人读与原生中性呈现，不要求用户用 CLI 设置。
原生检查 SessionScopeController 原因映射、SessionOriginSettingsController 自动/来源行、
SessionOriginTransport，以及 DesktopCopy 中英文资源、desktop JSON/金标和覆盖提示。
现有布局与渲染 specimen 复用 auxiliary-session-classification 的 UX/原型，新增文案以本表为准。

## 7. 最小迁移和兼容

migration35 为同一事务中的一个 apply：usage_source_files 加 nullable thread_id；
派生 session_origins 按增加 default_work 的 CHECK 重建，不复制旧派生行。
不动用户 override、mutation receipts、原始 observations 和其他事实。
重装被 DROP 删除的 serving revision triggers，并推进派生失效，首次正常扫描重算。
OriginSchemaVersion 仍为最低版本34，CurrentSchemaVersion 升35。

同步实际 Go/Swift schema消费者、serving schema分支、SchemaAhead金标与restore校验。
schema34只读不得迁移或访问新列，旧结果原样展示；旧 binary 遇35按 SchemaAhead拒绝。
新 CHECK/列和触发器恢复、备份还原、来源重写与身份补填都须测试。
迁移35由 ownership-guard-readmission 一次落地，后续规则任务依赖它。

## 8. 被修订合同与验证

取代 scan-performance 精确版本表，扩展JSONL线程证据；保留数值/冲突/journal/会话级排他。
修订 auxiliary-session-classification §3.3 默认类别和原因表、§3.4内置规则、§7来源状态。
同步受影响 CLI设计与文案合同，主会话ID下未写入JSONL的请求仍不单独合账。

四个表面的契约变更执行L2/L3验证。Go全套和相关race/vet、原生受影响测试/构建按实际范围，
复用现有原producer隔离证据，最终用已安装0.169验证新AgentDeck的exec/ephemeral/TUI分类与计量。
所有测试使用隔离数据，明确合成来源与真实producer证据，无上游改造。
