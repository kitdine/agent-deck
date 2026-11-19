---
status: active
created: 2026-11-18
---

# Codex JSONL 子会话身份与用量归属架构

## 1. 基线与范围

本文为 Round 2 有界修订，仅处理 CJSI-ARCH-R2-F1、F2。工作区 agent-deck.codex-jsonl-session-identity，分支 feature/codex-jsonl-session-identity，HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。输入为 [requirements.md](requirements.md) 工作区 blob `f5ebb617dd1acf54f3728712adc5f2f518d343e2`；[正式 Round 3](reviews/requirements.md#round-3--2026-11-18) 记录同一 blob、PASS、CEv1 VERIFIED 4/4，指纹 `bed3c6aebc30e83b2d2f0492f3a549cef11d32bbce08517b623286a53c8495ee`。该记录证明需求门禁，本文仍待评审。主题 [tasks.md](tasks.md) 当前只有需求通过，尚无批准的实现 anchor。

版本权威为 v0.7.5 合同工作树的 `docs/topics/v0-7-5-contract/tasks.md`，blob `14b97f89fc8a1eab0caa483536a8cb63e9a4f276`，成员表 Batch 3 / P2、依赖第 4–5、7–8 条。按 Lane B 推进 L3；主控在实现前对齐目标 main、交叉写者和迁移编号。对比 main `a224b0b824d2cad451d3bd83153832d65506f0fe`，集成时保留 `internal/usage/signals.go` 的预聚合以及 `internal/watch/watch.go` 的取消和释放行为，详见 §7.5。按 [文档流程](../../documentation-workflow.md#status-ownership-across-worktrees)，主题矩阵拥有未合并状态，主题集成和来源 Bug 判定另有门禁。

扫描限现有发现器内可读的活跃及归档明文。一条事实只有在来源和实际本地执行均成立时才能从该来源新增。既有 CLI、desktop serving 与 Widget 形状、价格/倍率、scope、排除和其他客户端语义沿用。内部能力表见下文。

## 2. 决定性代码与接口

下列位置以工作树 HEAD 为准（main 的差异见 §1）。评审时以符号和固定提交共同定位。

| 当前入口 | 已观察行为与改造界面 |
| --- | --- |
| `internal/usage/usage.go:1683,1771,1796,1831,1935` | looksLikeUsage 只识别旧通知；parse 的首头 thread 与后来可变的计量 session 分离，后者优先取 session_id。旧事件键含 session/turn 与计量指纹。新的 Codex 类型和分项存在性必须在共享归属层识别；不要让任何旧调用点重建另一个归属状态。 |
| `internal/session/session.go:653,924,960,1176,1390`、`internal/activity/activity.go:117` | session 有独立检查点、来源清理和 Codex 文档提取，activity 也自己跟随 meta/turn。把三者改接同一归属契约；索引写入仍负责搜索白名单，按需 activity 必须验证同一代。 |
| `internal/usage/usage.go:1129,1480,1527,2019,2045`、`internal/usage/source_pipeline.go:167` | 普通与前缀发布都可撤 OTel；重建保存/恢复原事件绑定及来源范围。统一改为经过已接受计量组的差分，旧代表行与 exact 凭据拆开。 |
| `internal/store/telemetry_ownership.go:48,93,164,320`、`internal/store/telemetry.go:321,364,531`、`internal/telemetry/projection.go:34` | 规范名解析已支持 reverted 形状；保存头身份可用头和文件名分别撤回；结算还依赖历史状态，OTel 索引另行投影。所有入口改走同一整线程判定。 |
| `internal/store/store.go:39,132`、`internal/store/migrations.go:330`、`internal/store/source_coverage.go:175,224,294` | 双库分别打开，session 索引可独立 purge，使用 WAL。core 35 有来源身份线索；现有盘点检查点跟踪实际已提交字节，须与逻辑组覆盖联合，单个完整字节游标不能宣告交接完成。 |
| `internal/store/serving_generation.go:22,117,164`、`internal/store/derived_generation.go:41,59`、`internal/scanruntime/serving.go:337`、`internal/desktop/serving_cache.go:162,191,325,414` | revision 向量与派生生成号用于现有缓存校验。跨库需要额外的共用发布选择；仍复用安全原子文件写、旧 publication 合法性校验及已有 unavailable/partial 路径。 |
| `internal/sessionorigin/binding.go:18`、`internal/sessionorigin/selection.go:352`、`cmd/agentdeck/session_origin.go:673,725,802` | birth 与覆盖的查询/写入/列表入口是独立的调用点。有效分类复用现行 reducer，持久绑定和其读出必须参与同一发布。 |

原生信封及字段位置以需求 §计数引用的上游 `1b84166252b6ecfd36be9e12c29ac01454a30d7e` 为准。该生产者测试观察的是普通根线程形状。真实版本的稳定 response_id、父字段、结束符覆盖率，guardian 服务模型和实库影响量均未知，设计使用显式证据分支。

## 3. 共享来源及右锚归属

在 ingest 的 Codex JSONL 路径增加共享的纯归属 reducer。输入是按原字节顺序的完整记录流、已验证来源代际、现有解析器使用的字段及固定的有效前缀；输出为可重放的带来源范围事实、候选/失效键、已知线程范围和覆盖缺口。usage-only 用同一规则归约 core 需要的事实，不打开 session 库，不构建文档。已共享解码的组合扫描只解码一遍；按需活动从相同规则、针对固定发布及范围重读。Claude 和 synthetic/OTel 伪源留在原入口。

1. 打开文件并捕获 identity、size、时间/变更戳、已读前缀证明和捕获终点；提交前复验身份、已使用字节及依赖的前缀，前端变动或代际不稳则丢弃拟发布的差分。字节起点的首个完整、合格 `session_meta` 固定来源身份。坏头不跳至中段；缺 ID 时只有规范名加一致旧单身份或明确本地执行证据可以交叉确认。规范名取现有规范校验与 reverted 首 UUID。头 C/名 P 则隔离该代，保持 P/C 独立来源既有贡献，隔离源未经证实的旧正文退出搜索与详情，旧数额按 §5 交接。
2. 直接父仅取自有头两个获准位置且一致、有效；forked_from、根会话与根 turn 各自只作线索。计算已证父边的有向环，环边整体退出，独立事实照常。父边是内部验证值，产品行为不依赖持久家族图；保留获准 ID 元数据足以重算。禁止从父边得到用途、路由、模型、覆盖或排除。
3. 为每代构造按字节顺序的 segment。meta、turn 切换、外来线程、继承完整性缺口及经证的执行结束形成硬边界。索引键为已证的 (执行线程, turn)，并记录段内 context 候选和该文件已观察到的同名继承候选。显式 thread 决定该条自身执行身份；仅从来源线程与执行相同的源发布。只有唯一匹配的同段 context 才提供本地模型；同名候选未排除时，显式请求可独立贡献主计数和已证调用，但缺 model/price。普通单身份旧源保持原 turn 兼容，出现混合证据后重评受影响段。
4. 对混合源无 thread 的记录，以唯一已连的本地 turn 引用，或 context 到匹配的显式请求的连续区间，取得局部右锚。请求后的无 ID 消息要等唯一的本地 task_complete 等获准右边界。工具结束可凭同一执行线程与同一 call 的唯一已证开始、连续性及无越界闭合，禁止跨越 turn/meta 完成或合成时长。EOF、时间、数值、相邻行均不提供锚。终段尚待确认时保留可回放起点/候选引用；有反证重算该段和同名候选依赖，撤掉仅依赖该连接的 model、文档、活动、信号，显式请求自身证据分别保全。turn index 按本地确定 turn 原序归约；晚到重排时替换受影响信号/索引，禁止沿用父的序号。
5. 本地 birth 优先取自有头 payload.timestamp、然后外层、否则最早明确本地请求或已证 turn/context（UTC 瞬时）。跨副本优先所有头时刻里的最早值，其后才用执行时刻；mtime、父记录、继承及已经撤掉的 OTel 标记不参与。OTel-only 仍取原合格结算的最早事件。会话摘要模型选首个非空本地模型，分调用模型只来自各自 context。

每个 checkpoint 记录最后完整行的结束位置和最早未解决的回放起点、必要的有限元数据及验证摘要。读取 offset 与已发表 offset 分开；单行跨批次时，从明文前一完整行边界重读并按原有限内存/大行上限处理，不写受影响 Codex 的 partial_line 原文；旧尾缓存仅可作为迁移识别线索，不进入新 checkpoint。解析坏段、待右锚与部分捕获均有独立覆盖，空读或读到 EOF 不能替代成功归属。若区间在预算内无法重放完，持久保存安全的候选引用和已读位置；下轮延续，已读但未可发布部分不给公开贡献。无界 candidate 以有索引的磁盘元数据工作集分批处理，不在 worker 内存无限堆积。

## 4. 请求账与只计 token 区间

以有类型和长度边界的 (codex, canonical-thread, stable-response-id) 为新调用键，大小写及 ID 规范化只按各原有 ID 类型；保存代际与范围作为证据，不以 offset、路径、时间或向量代造 response。先辨外层 token_usage_record 与 event_msg/token_count，仅取原生字段。完整主项必须显式存在；用精确整数读取并检查有界溢出和各分项关系。缓存 input 和 reasoning output 各为母项子集，总量为 input+output；总字段和两级累计不再加。可选细目用存在性位表达，定价必需但未知时金额不完整。旧比较基线只在已证本地线程和可比连续量纲内延续，缺 total 清基线，首次/重置有合格 last 才产生增量，否则仅重建基线；重复零、空和限流不新增。纯旧独立调用保留既有键/口径；混合源应审计每个已证边界。

| 已有证据 | 账本差分 |
| --- | --- |
| 一对一镜像或稳定 ID 重放 | 以同线程唯一执行证据或同一次响应区间的分项与上下界对应证明，选请求级事实替换旧代表。记录所有已证本地证据别名，异地副本和乱序重启同值。 |
| 独立 | 不同稳定响应，或可证独立旧调用段及不相交的累计区间，各保留一次。 |
| 只证明包含与余额 | 对区间按分项扣掉已认领调用的非负部分，余量一次；只有稳定执行线程、可信区间及已有合格 JSONL 所有者才能公开。重叠区间先分解成可唯一定位且不交的部分；不能分解的部分标待核。 |
| 可能重叠或缺 ID | 可重放组只发表独立确定请求；待定区间不相加。缺稳定 ID 只有唯一旧调用关联可继承调用身份，否则须有可核对的独立计数区间才有资格进入余量判定。不可重放旧组先按 §5。 |
| 同稳定响应有效计数冲突 | 该组撤回原响应、调用及金额，保留冲突证据，直到可验证新证据排除冲突。独立可信计数区间按余量规则重新判定。 |

调用继续进入 existing usage_events、事件分页、价格和调用覆盖；增设独立区间账，主键由已证明的线程、累计 epoch/区间和不交分项身份构成，带语义区间端点、可信时间界限、可选模型/独立 provider 证据、各分项存在性和覆盖原因。起止时刻由来源事实决定；跨不同粒度时按已有时区、半开窗，只在包含完整区间的查询桶计入，不能拆分的交叉窗口只报已知部分。禁用扫描时间和按比例分配。范围/归属不明的模型/provider 切片只共享覆盖不完整，未知路由余量不进 provider unknown；确有独立全区间证据才进相应模型/路由行。保留过滤交集的证据，不能用已知 M 推定 Q。客户端、会话与 All 已证余量入 token 与 distinct session，模型/provider 的全局份额分母含未分配的已证量；无新增 invocation、priced/unpriced、理由计数、活动、turn 或信号。

在现有 usage summary/stats、show token summary 和 desktop period/compact projection 的共同聚合入口加入区间贡献与覆盖，不将其伪装为事件喂入计价 SQL。受影响切片完整 base/provider 金额置 null，known 小计仅含仍有资格的已知调用，已有 pricing_complete=false、紧凑行 cost_incomplete=true；尚未更新但历史定价仍有效的旧提交值使用已有来源覆盖提示。单条已识别调用可仍有完整值；影响集合可证明不交的切片保持原完整性。保留真实支出与目录价各自资格、时间桶以及信号的调用语义。

## 5. 定向旧账、exact 与清理

通过 core 旧 source/checkpoint、事件 source_offset、历史 run/已验证范围、OTel 存量和独立索引旧来源建立有界候选工作表。只纳入受影响或不能排除受影响的 Codex；可证明无关的真 P/S、Claude 与无关来源继续原行为。以可能重叠关系的连通分量作为原子账组，边根据已验证代际/字节范围、执行区间/候选线程、验证过的副本及原观察建立。先剥离已证不相交成员；对无法界定的已公开旧孤儿，明确保存其未界定状态并把可能交叉的发布挂起，禁止把任意未读明文加入全局重叠组。索引中的旧错归正文不因计量暂留而留在公开搜索及详情。

- **定位**要求每笔拟撤旧贡献唯一连接原代际、区间/原事件和计量边界，验证其分项与原时间。**可重放**另要求现有明文或已验证等价明文可从可信起点重导全组旧事实。复用旧计量兼容解码验证旧行，新的共享 reducer 判公开归属；旧路径、旧 session_id、等量或文件能打开均不足。
- 旧侧不存在或整组两项均成立，按 §4 重算并一次替换。旧 20 + 新 R=10 仅疑似重叠时已确认 R=10，待定部分不参加小计，原真 P=40 不动。旧 20 缺定位或重放时，只有耐久证据可证明全组一对一或不相交才能换/并；否则 P=60、C 新增 0，保留最后安全旧侧和 partial。原两侧疑似重叠且没有安全旧侧时，两侧凭据保留，只返回不相交已确认小计。补证为包含或独立后分别得到 All=60 或 70，既有实际失效金额退出已知小计。
- 旧键或代表行转换先保全 run 凭据，用原已完成有效 run + 验证范围/代际 + 区间内本地执行 + 唯一同请求映射继续 exact。旧本地镜像在范围内也可作证，所选新代表可在外；移动凭同代字节连续性或存留范围凭据，继承镜像永不提供 exact。一旦有改写失效或冲突 run，重走既有事件时间 effective route、会话起点 provider 时间线、unattributed；延续原有路由和 multiplier 资格，不借父或补 1。历史对账分别记录纯重归、去重、补采、路由和缺价变化；守恒只用于同请求集、时间、价格与有效路由。
- 显式排除、purge-index 和获准删除先于保全及旧快照。path 仅排该来源，ID/project/client 按现有原范围、确认项目及合法副本选择，排 P 不传 C；core usage All 既不随 scope 也不随索引排除删除。版本化索引视图在读时仍执行最新排除；若无法在旧选择重做过滤，整体拒绝受影响的旧答案。
- 已登记受影响旧源消失、不可读或仅余压缩时，档案保全优先于两个 orphan/missing 清理。保留允许的上次已提交账、范围、检查点、索引、信号/活动及安全旧文档，按实际缺源触发覆盖、详情 stale/活动不可验证及已有 doctor 不可读来源计数；真实重建失败/代际不稳才用原对应码。仅 session 登记时不造 core 路径。存在合法可读替代须在成功换侧后清旧；无关来源或交接完成后恢复原清理。纯压缩且从未登记无新发现、证据、逐源缺条、接管或猜计数；不以“文件未见”推断压缩。默认及 --no-scan 覆盖在重启后仍能从持久记录判断。

## 6. OTel 显式事务权威与恢复收据（F1、F9）

**唯一权威。** Core 追加迁移在同一 schema 事务执行 `DROP TRIGGER IF EXISTS telemetry_jsonl_retracts_otel`，新初始化路径也不安装它。旧触发见 `internal/store/telemetry.go:67–85`，一次裸 INSERT 会按 NEW.session_id 删除贡献映射、OTel usage 和 marker，不区分来源、候选、旧账或有无收据。迁移验收查 sqlite_master 及升级/冷建结构；移除 `internal/usage/usage.go:1480` 的 threads 分支（普通发布、`source_pipeline.go:167` prefix、cold bulk 皆包含），并移除 `internal/store/telemetry_ownership.go:320` SaveSourceThreadIdentities 的头/规范名撤回。补填只登记候选、来源冲突、范围或已核验身份。旧 `SettleTelemetry`（telemetry.go:321,364–392）读取历史 usage/cursor/jsonl_owned 的 ownership 捷径退出；头、文件名和 cursor 仍可做待查线索。

按 (client,thread) 的 core 事务重评服务接受已经通过 §3–5 的发布计划及当前观察/覆盖，负责核验、生成收据、置状态、更新贡献、usage、边界会话和发布票。任何内部 Codex 发布调用方不能从 event/存储表存在性推导接管。收据、已结算旧侧的撤出和首笔非零 JSONL 的写入同事务，先记录收据再应用差分。观察接收/导入只写原准入、重放、冲突或失效材料；已有贡献因观察冲突失效时也由显式事务重评。老状态值继续使用 capture_only、pending、jsonl_owned、otel_owned、conflict、invalid、unmetered；待查及收据的子状态仅私有。

| 证据 | 处理 |
| --- | --- |
| 未评估 | 持久登记具体线程线索或真实旧孤儿重叠，保留最后安全单侧及 marker，无先例时待定。不能由全库未就绪冻结无关 S。 |
| 源头/文件名冲突 | 该来源不建事实、不触发接管、撤回、恢复或晋升；有独立来源按自身证据结算。 |
| 已确认身份但无可交接非零调用 | 可发表安全非计量事实；已有 OTel 保持，新观察待定。头、工具、基线、仅计数、冲突行或旧账重叠均无接管权。 |
| 有可公开的非零本地调用 | 同一次开放事务整线程换为 JSONL，撤销贡献及 marker，随后仍有任一合格调用时维持；余量不单独拥有线程，OTel 不逐请求补洞。 |

首次 OTel 结算维持原等待、原准入和 received_at 后完成的有限活跃/归档明文盘点，要求对**该线程**排除可能重叠；根据同代头/名、显式归属、已验证供应链或旧组限定候选。无法限定线程的真实已公开旧孤儿可阻止可能重叠的晋升，普通未读文件、家族关系、共享 root turn 不得如此。已结算侧仅待核不撤除。重评集合覆盖有关 OTel-only、历史无 pending 的 jsonl_owned、主动撤出的 JSONL 组。对旧错误抑制，仅可用仍有的原准入、结算和实际撤回凭据恢复，空的 jsonl_owned 记录无法倒推已结算。对无证旧侧维持 §5 的安全数值和 partial。

| 收据内容 | 必需字段、判定与复原 |
| --- | --- |
| 接管头 | canonical client/thread、每线程单调 takeover 期、开启/关闭 publication、支持组及其代际/覆盖证明引用、schema/收据版本、旧单侧安全性。一个线程最多一个未关接管期；重试沿用原期，后次真接管另开一期。 |
| 观察原值 | observation_key，原 state、reason、metered、support_entry、source/producer/version/class/transport、thread/model、request/response/attempt（只用既有安全字段）、record_at/observed_at/received_at、计量可空分项及存在性。记原合法准入、结算证据和值摘要或稳定历史引用；当新观察同 key 被判冲突/无效，旧收据不能覆盖它。 |
| 原已结算计量 | 原 contribution 的 observation_key→event_key；原 usage_events 的 client/session_id/event_id/event_at/model、规范计数、路径类型及必要定位、source_offset、run_id/准确绑定和实际使用的路由、资格、倍率引用。保留原事件时间和可核对的价格依据；无法从可靠引用恢复的原计量安全输入当场取值。恢复写回事实和原路由/定价输入，沿既有价格规则求钱，不把当前时间或目录快照代替原事件。 |
| 会话与恢复边界 | 原 usage_sessions 有关的 first/last/逻辑起点和 route/binding 引用；原 telemetry_sessions 的 client/session_id/source_kind/model/first_at/last_at/support_entry/settled_at；记录原公开键集合、动作 applied/restored/invalid 的内部状态。恢复后按实际原贡献重算受影响 usage session 范围并核对原 marker 投影；用户在接管后产生的独立变更不由历史快照覆盖。 |
| 幂等性 | (client,thread,period,observation_key) 唯一，原 event_key 在贡献表继续唯一。原 pending 没有原计量/marker 收据，不享有恢复特例。读到相同期开启或已关闭的 no-op 不再计数；事务失败不留下孤立撤出或可见收据。 |

最后一笔失据时，原子移出请求、金额与依附的余量；若还剩 R2=5，继续 JSONL 5。否则核实原 settled 事实、这次接管撤回、当前观察资格及当前盘点和旧账无重叠，才以同 key 在原时间复原 OTel 数值与 marker。已完全重放撤出的冲突段不单独挡恢复。无证或仍重叠则按旧侧规则，没安全侧时 C 为 0 确认且无 marker、覆盖不全。原从未结算的观察走正常等待。P/C/S 的 7/11/13 → 7/17/13 → 7/5/13、7/11/13 或 7/0 确认/13 在既有读面同代可见。

入口验收涉及 `internal/scanruntime/scanruntime.go:1197` usage-only settle、`serving.go:337` 组合 `SettleAndProject`、`internal/telemetry/reconcile.go:194`（receiver 周期及 CLI import/settle）、`internal/telemetry/projection.go:34`、`cmd/agentdeck/main.go:3445` watch 以及 `cmd/agentdeck/telemetry.go:465` 投影回调。候选状态和独立事务结算都不得直接改写共同影响的公共 marker；依赖既有索引或 serving 时统一交接。

## 7. 持久 schema、路径围栏与同代发布

### 7.1 分配和兼容围栏（F5、F7）

以 core schema 35、Codex usage parser 8、session parser 6 为兼容入口，主控分别分配 CORE_NEXT、CODEX_USAGE_NEXT、CODEX_SESSION_NEXT 及私有 session 布局版本。发布前对齐 main 和 v0.7.5 在途 schema、selection/serving 写者。Core 追加 DDL 移除触发器、建立证据/区间/收据/发布协调表；无源 I/O、无全库逻辑重算，旧源准入待核。新私有 session 扩展在各自事务生成独立候选表/FTS/epoch；现有 archive 和 serving wire 的公开 schema 保持原值，私有缓存格式单独按下文变更。

**固定旧路径围栏。** 私有索引使用新程序的唯一 locator，例如 `<root>/session-index-vNext.sqlite3`，具体内部名随 session 布局分配。旧固定 `<root>/sessions.sqlite3` 是 0600、至少一个 SQLite 页且起始字节不等于 SQLite 文件头的普通文件，内容只有固定无隐私标识，不用 symlink、空文件、仅 user_version、同名 VIEW 或假表。SQLite 旧 writer 的 `openWritableSQL` 是 sql.Open（或仅追踪包装）；初始化 pragma 或第一条 `PRAGMA journal_mode=WAL` 读取非数据库头时失败，确定落在 `migrateSessionSchema` 之前。旧 readonly 必跑 sqlite_master，也失败。旧工具允许 chmod 这个围栏和在旧名下创建空 sidecars，不能由此到达新路径。必须以仓库基线旧二进制验证具体驱动错误和 fence 内容保持（§10）。

| 已证实的旧可达路径 | 围栏/边界 |
| --- | --- |
| `store.OpenSessionsReadOnly`（store.go:105），CLI 常规读取/All fallback、classify query/clear/list 辅助、desktop scope / cache / loadSessions | 固定旧名 mode=ro，验证 sqlite_master 的两张表时失败；既有失败可能由上层表示 unavailable，而非给新路径中的有效视图。旧 classify 写 core 还受 schema-ahead 限制。 |
| `store.OpenSessions`（store.go:39,132,614），session exclude/rebuild/watch，scanruntime bring-up，telemetry import/receiver 的投影 | 旧固定名先 chmod/create sidecar，再执行 WAL pragma，失败时不会进入可 drop FTS/table 的 migrateSessionSchema，也不会访问新 locator。旧 rebuild/watch 可能在打开 core 前打开独立库，本围栏不依赖 core 为这组先拒。 |
| `store.OpenServingGenerationReader/Read`（serving_generation.go:117,164），旧缓存/CLI session presence | reader 检查路径并先读 core schema，拒绝 schema-ahead；即使某个 caller 先触碰旧 session，也只得非 SQLite 围栏。CLI stat 的存在性不等于能打开，不能成为成功空集。新 locator 以显式 store domain 取代 source_coverage.go:305 的 basename 判别，更新 desktop 的 identity/inode 校验与 sessionDatabaseName/stateDatabaseAbsent 入口。 |
| `session.CheckHealth`（internal/session/doctor.go:36）和 raw backup（internal/backup/backup.go:145，store.go:549） | 旧 doctor 固定 mode=ro 首个 SQL 失败，不扫描源；旧 backup 先读 core snapshot，经 OpenReadOnly schema-ahead 就拒，raw session online backup 若单独走旧名也失败。 |
| `session purge-index`（cmd/agentdeck/main.go:3205），`state migrate`（3190 附近） | purge 在调用 os.Remove 任何旧文件前必须打开 core，旧 core 读写 schema-ahead 拒绝；即使某个无 core 的工具误删旧 fence，新私有库仍不在旧名下。新命令发现已迁移 core 但 fence 缺失时修围栏或 fail closed，禁止回退旧库。新 purge 保留 fence 并记录 index 状态及 epoch，删除实际索引及所有候选，core 消耗保留。 |
| `backup.Restore`（backup.go:449,525,820,954）与 state-root 管理 | 旧 Restore 先校验 archive core schema 不超自身上限，再校验除 state.lock 外目标为空，以 O_EXCL 创建，不会覆盖围栏、实际 core 或私有索引；旧 archive 可恢复到另一个空根，不能借 Restore 降级同一根。显式整根删除属于原外部操作，跨根/手工 raw SQL 不在二进制兼容承诺内。 |

**安装顺序与中断。** 在现有 maintenance/state 锁下运行管理升级，先让本根旧 writer/read pool/扫描者停止并排空旧 SQLite 连接，再升级 core 到 schema-ahead（含持久 fence-installing 意图），封存旧索引的已提交线上快照。检查旧 DB/WAL 的一致性；用 SQLite online backup 或完成 checkpoint 后的干净快照，禁止生拷贝活动 WAL。只有旧版本无活跃写者且能获得旧索引维护窗口时执行；旧进程早先打开的文件描述符无法由路径改名撤回，不能把项目锁说成可吊销旧只读 FD。持久迁移日志只记阶段和验证用元数据，暂存迁移快照使用私有路径/权限。先原子替换旧主文件为已 fsync 围栏，检查开新连接失败，去掉旧代遗留的 WAL/SHM/journal 并 fsync 目录，然后才安装新私有索引、初始 seal 并登记 layout-ready。任何新候选写入的必要前提是 core schema-ahead + 验证到有效 fence + layout-ready；失败可重启恢复，仅从验证过的迁移快照续作，不向旧路径放回 staging，不自动 downgrade。若已验证的旧路径原本从未建索引，仍可装 fence 但不得创建私有 session 库。旧版在迁移中误触旧路径时至多看旧基线或收到错误；core 新 schema 首次生效之后其 purge 与 Restore 已失败。支持范围为受管停写/迁移和围栏后的新旧进程打开，无能力复活或撤回 in-flight 旧连接已有结果。新路径完全独立，旧连接无途径读取待开布局。若无法排空受管旧写者或确认一致快照，暂停 layout-ready；不以完成失败的迁移来批准开始回填。

### 7.2 成员盘点与唯一有效读取（R1-F3、R1-F4、R2-F1）

每个业务读取获取递交票据的 per-request View，内含 core tx/已开放 ticket、当前覆盖视图、session 代际及适用排除/保密遮蔽。索引包内部用一个有效 source/meta/FTS 选择层，接收该 ticket；package 外只可调用此层提供的读接口。仅证有索引且索引自身安全时才允许无 origin 的 All fallback；用盲底表判断空和 fallback 禁用。

| 实际访问者与源位置 | 接法 |
| --- | --- |
| `internal/session/session.go:1012,1646–1736` local selection、List、Search、ShowMetadata、DocumentsPage、ShowWithActivity；`affected_view.go:11,44,99` affectedSessionKeys、visibleDocumentsFiltered、affectedViewChanges；`scope_snapshot.go:56,123` ReadIndexSnapshot/searchDocuments | 使用同一票的有效 source/meta/FTS；change 比较明确传 base view 与 proposed view；旧渲染或 viewer 页面不重新从底表选择副本。 |
| `internal/store/session_origin_derive.go:24,44` bulk/单项 origin inputs；`internal/sessionorigin` selection、binding、reconcile；`cmd/agentdeck/session_origin.go:678,739,802,1271,1549,1571,1646` classify、readback/list、空探测、fallback 和 scope | 以显式有效读接口替代 raw SQL；source generations、birth、override 摘要和 activity/signals 共票。空探测要区分 unavailable，禁止返回成功空。CLI viewer tokens、activity 及 `main.go:1920,3015` 含索引/账本组合也用同票。 |
| `session.go:298,500,606,653,924–998,1574–1638,1767–1829` scan/rebuild、checkpoint、source move/delete、ReplaceDocuments、exclusions；`unchanged.go:20,82`；`source_pipeline.go:125,179,250` | 普通域合法写走既有事务并更新发布基线 revision；Codex 联合变更走 candidate writer。只在 writer/迁移内访问底表检查已提交 base 或自己的候选；unchanged、fingerprint、rebuild/cleanup 必核实基底 ticket 和候选依赖。显式排除先更新可见保护，及时清相应候选与底表；purge 串行截断票据并删除私有内容。 |
| `internal/session/telemetry_projection.go:42,76`，`internal/telemetry/projection.go:18,34,53` | index presence 改读有效 OTel 虚拟来源；新增、撤除及恢复 marker 的索引投影由发布计划执行，不允许现行全量 upsert/delete 绕过 gate。 |
| `internal/desktop/desktop.go:629`、`session_scope.go:129,174,344,437`、`serving_cache.go:162,191,229,325`；`scanruntime/serving.go`；Widget | 复用有效索引与用量 View，版本验证按 §§7.3–8；Widget 仅走现有 helper/AppGroup 安全聚合路径。基线 Swift 搜索中 `EmbeddedHelperRunner.swift:833` 仅认 root 的 core 或 serving 文件存在性；保留原入口并复核目标 main，不能把文件名可见当成票据授权。 |
| 诊断、备份、Restore、受管迁移、性能 fixture | 原始 SQLite 访问限 store/sqlite infra、写者/CAS、完整性检查和私有快照。诊断计数的业务部分通过有效视图，备份只输出允许内容。性能工具 `cmd/agentdeck-perf/template.go,batch.go` 另处理合成模板路径，不作为生产 fallback。 |

用接口收缩及自动化 SQL 源扫描防旁路，列出允许引用物理表的实现文件/SQL 角色；测试注入只有底表或未开放候选有的哨兵，走真实命令和 helper 的消费者矩阵，业务哨兵不可见。避免用 DAO 命名本身当隔离。诊断拥有的 PRAGMA/SQL 不返回清单、路径或 SQLite 原始错误。

**FTS ID 与遮蔽。** 在独立私有索引中用 monotonic 元数据分配 SQLite 正整数逻辑 doc_id，从迁移底表最大 rowid 以上起步，同一个分配器供常规写入和候选写入；崩溃可留下间隙，但不能回收/重用。迁移不改旧 surviving rowid；原行未变/合法移动保原 ID，重解析能按已验证同代逻辑文档键唯一映射才保原 ID（例如 source identity + 记录起点 + kind/序）；新/不可对应行取新 ID，禁止用文本等值枚举。FTS 底表和候选显式用 rowid=doc_id；候选按 `source` 旧/新定位和 `(source,client,session)` 逐组遮蔽底表，路径重命名包括旧 source tombstone，局部文档删除具名 doc_id。替换全组则必须遮蔽全组，即使一项 MATCH 也没有，排除先于 priority/path 选择。

当前 searchSQL 的最终键为 event_at 是否为空、event_at 降序、client、session_id、rowid（session.go:1646）。搜索沿用该键。对 base 和 candidate 各跑原 MATCH，按同一 effective metadata 选择副本，丢弃被遮蔽键再合并，按原键排序后做统一 total/limit/offset，无单表预截断、无并表 BM25。相同 doc_id 只能由有效来源出现一次。compact 在一个事务按同一 ID 显式插入/删除、核对可见集并记录 compacted-through ticket，分配高水位不回退。FTS shadow 不装逐行触发器。

**详情顺序。** 现有 `DocumentsPage` 分开执行 COUNT 和按 rowid 分页，`ShowWithActivity` 另按 rowid 全读（`internal/session/session.go:1664–1749`）。保留 doc_id 的独立身份和上面的 MATCH 规则，新增仅索引内部的详情顺序映射。对有效选择中的每个 `(source,client,session)`，每个可展示的 doc_id 恰有一个从 1 递增的 `detail_ordinal`，组内唯一。排序只取该组的 ordinal。先应用排除、遮蔽与原 priority/path 的副本选择，禁止在多个副本间拼一份详情。ordinal 不进入公共字段、搜索 tie-break 或稳定 ID 判等。

- 受影响 Codex 明文从**选中来源的已验证代际**按记录字节起点排序，同一记录按解析器固定的文档发射序，裁掉未获证/不允许的文档后给保留序列编号。无论右锚何时补齐、扫描批次或写入先后，顺序取原始记录位置；不以事件时间、到达时刻或 doc_id 推导。冷建与重解析使用同一顺序生成函数。纯追加验证原有效序列为前缀，才在后面接号；中段插入、删除、归属变动或重排时，从最早受影响处重算后缀，前缀与通过现有规则保留的 doc_id 可不变。若无安全检查点或不能证明前缀，就从验证起点重算全组顺序。合法移动可保原 ID 与相同序列，代际改变按新顺序核验，ID 保留仍遵照上文。
- 迁移和旧 archive 先把**可安全展示**的既有组按旧 rowid 顺序物化成 ordinal，包含所有 surviving rows，并标记尚未经过新明文顺序验证。这样旧安全票可按原顺序读取；若当前证据已否定某个旧文档的可见性，保护规则仍先遮蔽。第一次要为这组开放重解析/追加结果时，必须由明文验证该组的完整有效顺序，包括仍然保留的旧行。不能证明旧序列可作新前缀时，不能只按旧高水位给新文档接号。来源缺失或旧证据不足，沿 §5、§7.6 保留安全旧组或既有不可用行为，不拼入猜测的顺序。无关客户端和 synthetic 文档延续各自原展示序，按同一 ordinal 接口暴露；OTel 空文档序列保持为空。
- 候选用独立的顺序映射，按受影响组替换其有效 ordinal。候选可引用仍有效的底表 doc_id，无需复制这些正文；验证有效组的成员、序号、唯一性和 FTS 行引用后纳入同一 session seal。旧票取旧映射，新票同时取新映射和自己的文档遮蔽；禁止一个组回退到第二套排序。compact 原子合并 FTS 和顺序映射，以 `(doc_id, detail_ordinal)` 检查有效序列相等，并保持该票的逻辑 revision。备份/Restore 见 §9.1。
- 把 `DocumentsPage` 和 `ShowWithActivity` 接到同一有效详情读取器。CLI `show`、`--all`、`--activity`（`cmd/agentdeck/main.go:2947–3007`）和 viewer 文档页（`session_viewer_data.go:17–42`）在同一选票、同一 SQLite 读快照选 metadata、计算 total 和读页/全量。页码、limit 和原公开 pagination 字段不变；全量读取用同一 ORDER BY，越界仍按原空页语义报告相同 total。viewer 不跨交互持有长 SQLite 事务，每次载页检查已选票与有效详情序列身份；旧票和保护仍有效才继续该页，否则整组重新选择，并用原刷新/读取错误路径回到新序列的首页，不能把两代页接成一份。
- `ShowWithActivity` 的全量文档和 CLI/viewer `--activity` 的文档都用上述 ordinal。活动列表仍沿既有活动顺序、筛选和独立 pagination，按同一读取票验证源范围后才附上；旧范围不可验证时沿用 §7.6 指定的安全旧文档与 activity warning/现有失败语义，不能用当前文件产生的新活动补旧详情。

### 7.3 发布、可见 revision 与锁边界（R1-F2、R1-F3、R2-F2）

**四个耐久边界。** Core 准备清单 → session 候选 seal → 私有派生候选 seal → 唯一 core 开放事务。数据库使用各自 WAL；只有开放事务是对外的 fact barrier。session 候选保存在独立 delta tables/FTS，带 epoch、ticket 和 base revision，含 source/metadata、文档、tombstone、checkpoint、coverage 候选；绝不先写 `session_sources/session_metadata` 触发现有 serving/revision coverage。Core 准备还只存安全待核工作与 proposed facts 的受限清单。私有准备、分配 doc IDs、仅保留的收据材料和无效候选清理不前进 public ticket/epoch、事实 revision 或分区。收据状态对公开结果有影响时仅在真正换侧事务标记公开影响。

1. 按预算在 state/scan 串行边界登记 ticket、影响组、当前 core/index epoch、真实 base facts revision、盘点/源代际范围、路由/价输入 revision、用户 revision 和 exclusion/purge。开新票前可按安全前一一致代继续读；若确证旧正文或旧侧数值无资格，立即持久保护受影响绑定/集合，不等候选封存。保护在所有读路径应用，并使会暴露失效内容的缓存失效；其它无关域继续服务。
2. 同票封存独立 session 候选，预构建安全索引、核对已有范围，准备不会发出旧 FTS 或真实 count 变动通知。隐私和验收所需 FTS 只在 session 库的候选 FTS，未核段仅存引用。已核过的候选失去基础时代，拒绝复用，保全旧侧或安全小计。
3. **在写事务外构建并封存同一 proposed view。** 完成登记与 session seal 后释放所有前述写事务及会阻塞根内写者的锁，再做耗时读取、叠加、计算和制品生成。 以只读 core SQLite 快照固定当前开放票、权威基线与影响窗口，用已完备的不可变 core 差分覆盖同键值和显式删除，再与同一基线、已 seal 的 proposed-session View 配对。差分含最终将应用的行操作、原值前置条件和开票后预期公开 revisions、coverage。用有界的查询读接口注入基线快照与差分，对允许的 affected usage/interval、结算标记、归属、信号与摘要读取同一有效关系；按原优先级处理路由/价格、用户绑定、保护与 scope。查询先按键叠加，再过滤、聚合或计价。需要全窗的 reducer 从同一快照读依赖窗口内的安全未变事实，不逐个打开 live store，不用全库副本，不把任何未开票分区、派生或凭据写入公共缓存。现有 `prepareServingSnapshot` 会分开打开 live store（`serving_cache.go:191–264`），候选分支必须接收该 View 与一次捕获的时钟/时区/有效窗口；新旧构建复用 reducer。释放两库读快照后再序列化和进行文件 I/O，以免同步文件等待钉住旧 WAL；构建过程中快照超预算则释放并保持 pending。

   用既有私有原子写的权限/路径校验机制（`serving_cache.go:414–448`），将序列化、写入、文件 fsync、close、rename 至只属该 ticket 和 attempt 的私有 slot、目录 fsync 全部放在 **core write tx 和会阻塞其他根内写者的 state/maintenance/serving 锁之外**。写入前后检查合法文件身份、大小、校验和及可用期。只有文件与目录同步成功，才以短独立 core 元数据事务登记 durable seal（没有公开 fact/revision 变化）。seal 绑定 ticket、attempt、基线向量、差分摘要、session seal、预计开放 revisions/coverage、slot 代际与校验和、生成时刻/有效窗口和格式；session seal 已包括有效详情映射。候选 slot 与 seal 一旦生成不可覆写。跨进程回收先在协调头把已失效、且没有开放票引用的 attempt 置为可回收，再在事务外删除文件；与开票竞争时由同一协调头 CAS 判胜负。有效接受期间或已被开放票引用的 slot 不可回收。候选窗口若包含时区日切/定价有效边界，必须在 CAS 时仍然有效。不需要共同产物的单域可跳过对应 seal，共同依赖域缺 seal 时一律不开放。

   同步 fsync 可超过逻辑预算，ctx 只限**等待和结果接受**，不会中断内核调用。沿既有有限 scan worker 与 pending 读法安排单根至多一个在途 slot 写入；调用方到期限可结束等待并按 §7.6 读安全旧版，实际 I/O worker 仍占该根的候选名额，不能为同一阻塞不断起替代写入。协调头用 attempt 身份与接受期限拒绝超时、取消或被保护变更淘汰的结果；返回太晚的写入至多留下未授权私有文件。其他读者及无依赖 core 写者不等该 fsync。文件或目录同步出错时不开 seal；未完成的候选仍以既有 pending/coverage 表达，按预算复用已验证输入重试。

   **最终 core CAS 仅应用事实并开票。** 开票者在短写事务前完成必要的源范围复验、slot 文件身份/完整性核验，并持有候选只读句柄和生命周期 pin，防止受管回收或替换。最终事务只读写 core SQLite。锁定协调头后比较当前 active ticket，core/index epoch、选中的 session seal 与基础 revision（session 依赖由共同写者围栏保障），候选 attempt/状态/接受期限，两 seal 及差分摘要，原行前置条件，盘点与已验证的捕获范围、路由/价格、用户 revision、排除/purge、保护/coverage revision、窗口与有效期。写源可在 SQLite 之外追加/改写，开票只授权已验证捕获范围，盘点或 CAS 证据变化需再验证，后续新增字节须走下一代。所有会改变已读取依赖的受管写者必须更新对应 revision 或使候选失效。只在全部吻合时按相同差分先保存收据，再执行受影响的 usage/OTel 等行与标记变更、计数及 coverage/revision，验证结果 stamp 后 COMMIT 将 ticket 指向两 seal。事务内不扫描/聚合、读取源或 slot、序列化、rename 或主动同步文件；SQLite 自身 WAL commit 的持久化 I/O 仍可能等待，不能承诺事务固定墙钟时长。CAS 拒绝或 SQLite 回滚保留公开旧票；commit 结果不明时重开读取协调头与收据确认，避免盲目重复换侧。
4. core 公开选择与真正 fact mutations 一起切换；读者凭 ticket 选择已 seal 的索引差分和 slot，不能采信固定文件名。活动按需复验同代源范围；与旧票不一致就现有 warning。激活后 compact 只合并物理形态，保留 doc ID 与同一逻辑 revision，不由底表 triggers 再制造新公开 mutation。可用单个受管写事务在当前 session revision 上完成 trigger 计数并恢复至同一逻辑镜像值，或将公开 revision 唯一改由 coordinator 读取（首选后者对 Codex compact），正常非候选写与 purge/exclude 的原 revision 保留用于 CAS；禁止全局关触发。读者不得以 compact 后物理 base 回答更老 ticket；基底已经跨越就重试/返回既有错误。收尾只在对应票合法可读时回收旧候选/slot。

**可见变更分开记账。** 配置、provider、普通 session 写入、排除与 purge 保持实际 revision；候选事实不触发这些变化。独立持久 coverage/privacy overlay 记录受影响组和可判定窗口的 pending/partial 状态、失效旧侧以及保护 revision，不推进事实 ticket 或数据 epoch。和实际不交的查询不带该缺口。仅准备而 public coverage 未变时 serving、日分区及公开 revision 不失效；公开 coverage 确实改变时独立 coverage revision 失效受影响的完整性缓存，`--no-scan` 立即能读到原值附旧警告，旧缓存不能覆写 coverage。仍合法的固定旧 serving payload 仅标 stale，若安全域可 bootstrap 则使用新覆盖状态按原 contract 返回；不能通过重写旧 payload 冒充新的整批 publication。禁用任何暴露旧正文/已撤事实的缓存，已过权限/排除/日期自然失效。纯布局 compact 事务内复核并还原 base-trigger 造成的 session revision/coverage 临时改动，保留开放票据指向的**实际**公开 coverage，再写 compacted-through；计数与逻辑可见集不一致则回滚，不把 compact 用于新的排除/数据变更。

Core 开放更新正确的旧/新 usage_events、usage_sessions 和 binding，由已有 triggers 保留事实依赖；区间新增/变更/退出、公开票据和覆盖变化加显式 hook，写在同一事务。Marker/分类/信号的实际变化沿已有 owner 使相应域失效；私有收据 no-op 不标脏。以同一事务中的 OLD/NEW 数据计算脏键，删除最后一项也保留 dirty 空桶，使缓存不能复活。分区和缓存细节见 §8。相同逻辑结果/覆盖的两轮不得仅因新 attempt 编号推进公开票据。

**读与崩溃。** 读者先在 core 快照固定 ticket 和 core 值，再在 session 快照读对应 seal/epoch 及适用的 coverage；迫使重试时整体丢弃结果。跨 helper/不可同事务的拼装核对前后 ticket、状态 inode/epoch、源与保护 revision，全部相同仍须有对应 seal。一个旧请求可在当前权限与保护允许条件下，以旧票及已打开的安全句柄使用一致旧代，不把新版索引混入；如果旧票已回收、错过打开或保护已变化，整体重读或返回原有不可用结果，不能自动改读同名最新 slot。准备后死机，拿安全旧答案；session seal 后、派生未完成亦然。私有 slot 只有文件、没有有效 durable seal 时仅是不可公开孤儿；有 seal 而 core 尚未 commit，则旧票仍有效。重启按 attempt、扩展校验和基线复核，有效 seal 可在原接受条件内直接重试 CAS，否则废弃并重新构建；不能从候选文件名恢复已开票。core commit 后、当前镜像物化前，新读者据已授权 ticket 读已 seal 槽位；镜像故障不重放已提交 OTel 事务。任何活动 seal 丢失/损坏都 fail closed 于受影响域，既有读错或 partial fallback，不从稳定向量推定同代。显式 purge、排除、并发用户 revision 在共享维护序列内更新保护并使未开放计划失效。独立无依赖写者原有事务可继续，任何影响 planned 切片的写入由锁/依赖 CAS 判冲突。线程 birth 与分类按 §3 和通过的需求 同票处理：相同完整 birth 或确证连续才 matched，只有确证转写 digest，保留用户 revision、时间及收据；不同且不能证实维持旧 digest 及 unmatched/automatic；原 limited 不自动升级，set/clear 胜过过时候选。

### 7.4 Parser 与缓存身份（F2、F7）

用每域一个目标版本选择函数，入参为可信 inventory/linkage 中的 client、来源类型，而非旧错归 session ID 或数据库路径推断；明确 plaintext Codex、Claude、OTel synthetic 和 ReplaceDocuments/synthetic。未知类型跳过可复用快路而按原范围待核。Codex 新 checkpoint 专属版，Claude 保持现行版本兼容（Usage 允许已有 7、正常写 8；session 为 6），OTel/synthetic session 按自己的旧版本 6；纯压缩没有 parser。新版 Codex 只读完整行的进度/元数据，迁移时如旧 partial_line 存在按 cursor 减尾长度验证最后完整边界并在新私有索引/准备检查点中清尾，不复制到新 raw cursor 或 backup。旧覆盖没有可靠证据时不延续新快路；所有已安全发布旧账保留至交接。

| 检查界面 | 直接替换点与验证 |
| --- | --- |
| Usage inventory、读范围、恢复 | `internal/usage/usage.go:878–914,969–977,1190,1213,1286` 分类 parser outdated、append ranges、load / checkpoint CAS、prepareSource；`codex_signals.go:14` Claude 兼容不挂在全局常量等值上，旧 parser context 的迁移只可作线索。 |
| Usage publish 与 unchanged | `usage.go:1598,1631–1639` upsert / coverage-block；`source_pipeline.go:167,276–281` prefix、cheap unchanged；候选的组织选择持久目标和实际已发表的 checkpoint 版本分离。inventory/watch fingerprint 包含适用来源条目的目标与已开放版本变化，避免其他客户端的指纹因为 Codex 能力升级改变。 |
| Session inventory、planned range、coverage 与 prepare | `session.go:308–327,500–523,606–622,653–709,804–806`；`unchanged.go:20–55,82`；`source_pipeline.go:250` CAS。用选定目标判断 unchanged/append，检验完整行位置/待锚起点；原版本快路能依旧通过时不强制重读。 |
| Session writes、move、替换与派生 | `session.go:924–957,1583,1631` 移动/复制保该 source 的实际版本，重解析写新值，metadata 同来源版本；ReplaceDocuments 保独立版本。 `telemetry_projection.go:65–72` OTel 虚拟源不使用 Codex plaintext target。`scope_snapshot.go:88`、`store/session_origin_derive.go:24,44` 和 checkpoint/fingerprint 输出都消费已选来源的实际版本/发布代。 |
| Cache | `usage/projected_events.go:95` 按受影响的 client/实际已开放证据版本与 per-partition revision、解析/聚合算法版本等选 cache。`desktop/derived_cache.go:117,217` 与 `serving_cache.go:275,325` 私有格式记有效域 stamps（实际公开来源类别/版本、依赖 revisions、覆盖与票据），不以可执行文件里的 Codex 目标常量让 Claude/OTel 重读。旧 payload 若缺必要 stamp 不作为新候选的凭据；合法旧整批版本可按既有 stale/bootstrap 条件。 |

### 7.5 Usage-only 准入、续作和 main 对齐（F8、F10）

`cmd/agentdeck/main.go:3380–3458` usage watcher 目前只延迟打开 core；session watch 单独打开 session，然后处理 settlement 和 origin。普通 combined 路径由 `scanruntime/serving.go:147,319,337` 和 `scanruntime.go:1184–1260` 的实际共享盘点扇出。Usage-only 不能为了检查候选打开/创建独立库。它可 stat 新 locator、核对 core 最近合法 session seal/epoch、存在态、来源/线程的跨域依赖摘要及最近授权 serving 依赖；旧 fence stat 不是存在证据。path/file 类型不符、已存在不可读、旧孤儿或摘要缺失时只持有可能受影响的组，不把它当从未存在。

- 实际影响可能包括已有索引 source/meta/FTS、OTel projected marker、birth/override、选择成员、同一索引会话的 `show --tokens` 或 activity/signals，及已承诺共同发表的 serving 域。即使本轮没有新消息，只要一个选择中的现有会话数字、分页或共同派生值改变就持有。来源范围、依赖摘要与已验证查询范围可证明没有交集时才不持有；只看本轮是否有 session job/是否更换 owner/是否存在 SQLite 文件都不足。
- 例如无索引、无跨域派生前提的正常本地追加，或已验证与现有索引与公开组合范围无关且不改变其成员/派生的 usage-only 追加，按普通 core 事务发布；仍读同一索引会话的纯 token 追加若影响同时交付的结果，无可靠同代 seal 则持有。已显式 purge 使用已登记 tombstone/保护，不能等同随机打不开；不创建索引只为证明没有。轨迹确认所谓独立追加无任何索引打开。
- 持有时 durable 保存预算 checkpoint、受影响键和 pending，覆盖沿现有 update_pending/partial。usage-only watch 返 `store.ErrTelemetrySourceIdentityPending`，`internal/watch/watch.go:95` 转 `scan_pending` / `source_identity_pending`，不确认 fingerprint；普通 usage 返回已有 scan_incomplete 或受影响读错。组合 scan、受支持 Session/serving refresh 或维护恢复复核并消费同票，只有完成共同 seal 后才清本组提示；明确其他缺源组保持 partial。只要候选仍在，watch 指纹比对不得漏掉它。
- 对照 `a224b0b824d2cad451d3bd83153832d65506f0fe` 的 `internal/watch/watch.go` 已有 `releaseWithError`、`isCancellationOnly` 及 scan/fingerprint/persist cancellation，保留 ctx 检查、释放恰好一次及 release error 和主错误合并。正常 pending 不能在取消后被伪成成功或写 ack。 `internal/scanruntime/scanruntime.go:1571` 保留 source_identity_pending；receiver/reconcile 仍用已有状态输出，不扩大 public enum。主控同时保留 main `internal/usage/signals.go` 预聚合，对有界新输入做等价比较。

### 7.6 交接中的既有读面

| 入口 | 未开放或无安全旧版时 |
| --- | --- |
| list/search、浏览 | 同选择的安全旧票提供行、FTS、排序与 total。只有分类不可用且索引可完整回答 All 时回退独立 All 和原 session_selection availability；Work/Auxiliary 或索引自身未获证走已有错误、浏览错误页。 |
| show / --tokens / --activity | 提供合法旧基础详情；token summary 和分页 invocations 必同票，无同代账则 --tokens 失败，基础都不安全则普通读取错误。按需 activity 只可复验同代范围，否则已有 activity_warning，stale 只指真实来源缺失。 |
| 默认 usage summary/stats/signals | 正常同步尝试扫描，既有 partial=true/scan_incomplete 或读取错误；stats/signals 文本 PARTIAL，summary 原警告输出。已证分离小计可读，范围不可用错误与完全没有安全账本错误按原语义；stats 消耗仍 All。 |
| usage summary/stats --no-scan | 不读来源。持久 partial/update_pending/unknown 对应 source_coverage_partial/source_update_pending/source_coverage_unknown，JSON partial、文本 stderr。合法已知空可零且 partial，无法读取一致账本不返回完整零。 |
| usage sessions/diagnose、classify query/list、遥测/来源状态 | 一致旧形状或既有读取错误；diagnose 为存储计数，列表 binding_status，与详情 applied/effective 同票。状态命令不扩逐源缺口。 |
| desktop serving/menu bar/Widget | 保持日期/权限/排除/身份合法的旧发行为其真实时间，按既有 stale；无安全旧发行的 partial bootstrap 只暴露已验证域，未证 available=false。Widget 走已有聚合快照，保护/校验失败不能保用已失效的内容。 |

## 8. 汇总、分区和精确观测（F2、F6、F13）

**公开数据的查询合同。** 事件维度继续用有效 `usage_events`/pricing；加一个共享受公开票据和保护约束的区间 reducer。输入保留显式主项、各可选分项的存在掩码、时间上下界、线程以及独立模型/provider 证据。禁止隐含缺项为零。以无溢出的整数检查和比较，reasoning <= output、cache read <= input，均是子集，raw total 或存储的 reasoning 不重复加；缓存/定价所需成分不明时完整金额不可用。无存储错误的可信零、有效区间的显式零、没有可读账本的未知或 payload 损坏分别处理。只证余量加入实际客户端/会话的 distinct session 与 token；不会新增调用、计价调用、理由、精确度、工具或信号。未知维度只影响可能相交切片的覆盖；未知路由不落入 provider unknown。可定位模型/provider 行的 token 份额分母为对应未过滤已证总量，不能把未分配 tokens 在已分配行重新归一。scope 的 Work/Auxiliary 成员只用于原工作分析。

| 入口（固定 HEAD） | 接法 |
| --- | --- |
| `usage.go:2845,2853` Summary / SummaryRange；CLI `main.go:4118` | 同票按原 client/时间过滤加入完整落入窗口区间，去重会话，known 金额只来自合法请求。部分相交只出已知份额并 partial，日界沿原时区。 |
| `usage.go:4191` Sessions；`session_usage.go:38,72,159` SessionUsageSummary/Invocations/Count；CLI show 与 viewer | Sessions 和指定会话 summary 以同一请求及区间聚合并合并 distinct；按既有排序和会话选取。invocation 页和 total 仍仅从请求，token summary 可以大于全页调用和。不可取安全同票时走原错误。 |
| `usage.go:3866` Stats | 在现有可见桶、客户端、指定模型/provider 的求和中分别评估区间包含、可分配、可证明不相交；成本、调用来源质量、调用百分比继续请求专用。未分配 token 仍入未过滤份额分母；任何相交待定切片的完整费用 unavailable。 |
| `signals_report.go:95` Signals、`signals_batch.go:12,82`；工作活动、分类与 stats 工作块 | 区间只供已经表达消耗的 summary；调用/工具/turn/activity/rework/分类和逐信号 cost 仍由原调用事实与同票成员生成，严禁虚增信号或把余量推给某个信号。保留原来源 coverage 和工作范围错误。 |
| `usage.go:2213` Diagnose；CLI `main.go:4245` | files/events/sessions/tool_calls/exact_runs 继续原存储维度，区间不加事件数或伪造修复数；校验可见状态并只读安全的已提交逻辑事实，原形状。 |
| `presentation.go:245–284,458–523` Presentation/ProjectedPresentation/TodayPresentation、`projected_events.go:39`；`desktop/derived_cache.go:84`、serving BuildToday 和 Widget period/compact | 定价事件可以复用现有分区，区间按下面的共同 reducer 独立选取且只加一次；顶层、period、compact 与 CLI 的 known/null 约束同源。全部呈现 pricing_complete=false 和 cost_incomplete=true 仅于实际受影响切片，单条请求仍可完整。 |

**区间和分区的实际机制。** `store/usage_partitions.go:24–96,135,213` 目前事件按 (client,model,UTC day) 触发，路由/目录等用 scoped/global invalidation。最小做法保留 priced-event 分区为事件清单；只计数区间只写 canonical 区间账，以 client、session、上下界索引，使用同一 core read tx 进 reducer，不能从各日重复累加跨日区间。开放时对区间 OLD/NEW 覆盖到的 UTC 日期标脏已证具体 model、无模型的内部 aggregate-only key、相关 client/all 汇总及覆盖片；将边界 UTC 日与最终用户时区半开日/小时转换分离。未知模型/路由不为猜测的每个 model/provider 创建行；该客户端/日的 interval/coverage stamp 参与过滤汇总，含 pending proof 的切片以可证明不相交过滤。使用独立 typed key space 或独立内部 stamp 表表达 aggregate-only，禁止与 `model=''` 的已证事件碰撞、将其交给事件扫描循环或输出公共模型行。开放同事务更新 scoped dirty、真实公开 coverage 和相关 presentation revision，包括区间只有删除没有事件的情况。对超长跨度用日期区间 invalidation 与读取 intersection 懒清，不按小时复制账行。

事件分区格式继续事件列表校验，Codex 的私有 header/版本键改用上述 publisher/parser 和 scoped stamp（区分旧键 v1 与新键 v2）；Claude/unaffected 保留相容事件 payload，不以全部客户端重读解决。范围聚合/派生 payload 新私有版本同时覆盖 interval/coverage stamp，记出价集、状态 epoch、查询时区窗及有效会话代，wire 字段不增。区间存储始终是事实的一部分，不因缓存缺失、冷建、只读或坏 payload 被吞掉；坏 cache 以有效账本重建，损坏或不一致事实按已有读取错误/partial，禁止当作已证零。每次比较受影响分区及元数据（`CheckUsagePartitions`，`CheckUsageProjectionMetadata`）和公开 ticket 前后验证，读合并时以区间键唯一去重。事件 v1/v2 仍校验 key、分区归属和 checksum；区间不伪造 priced payload 记录。

**观测和对账。** `internal/ingest/metrics.go:13` 的 read_bytes/read_calls/decoded_records 仍分别计该 ingest 仪器实际字节/调用/完整解码，复验非 ingest read 如需观察用独立固定 key，不改旧 P08 分母。保留 `internal/usage/source_pipeline.go:42,59` wait/commit，`internal/session/source_pipeline.go:74` ingest write 和 `scanruntime.go:1037,1381` scan.end 在派发结果之前。准备、session seal、derived seal、open、recovery 可新增内部固定无标签 span。下列固定 count 在 coordinator 取得 durable 效果后累计到本轮 scan.end，失败、取消或重放 no-op 算 0；未提交扫描的 ingest 工作仍照旧计量。

| 固定 count | 计量时点 |
| --- | --- |
| codex_groups_committed / codex_groups_held | 本轮实际新开组数 / 扫描结束仍持有的受影响组去重数，后者不是提交事物。 |
| codex_requests_upserted / codex_requests_retracted / codex_token_intervals_committed | core 开放事务中实际逻辑调用新增或值改变 / 实际退出请求键 / 实际新增或改变的区间键；区间仅删不计这一项，可从耐久差分对账。 |
| codex_otel_threads_taken_over / codex_otel_threads_restored | 每次真实改变公开所有权的线程只计一次，以已提交收据与差分确定，头/候选/待核计 0。 |
| codex_publications_resumed | 每轮成功推进此前中断 ticket 至新的耐久边界一次；若同一轮多次准备快重试仍只计一。 |

不记录 source 路径或 ID、offset、原文、凭证或 SQL 值，也不把私有 substage 交给 doctor 或新公共命令。crash 可丢尾部 trace，逻辑对账来自持久事实、票据与收据；不重写 trace 宣称成功。实际旁路风险来自 §7.1 的旧重建、§7.2 的 raw SQL 和 §8 的 partition 快路，验收注入故障并用固定计数捕捉。

## 9. 可移植备份、Restore、doctor 与隐私（F11）

### 9.1 备份与还原

现有 `internal/backup/backup.go:96–177,201–245` 分别 online backup core 和可选 session，core 快照内清非可移植 origin 状态；`449,525–548,656–668,820,875,954` Validate/Restore 校验 archive schema、要求空目标、逐文件 O_EXCL 后 mint index epoch。两次独立 backup 无法证明同票。新实现保持现有 archive entry 名、加密、hash、可选 session、权限与空目标合同；可有**私有**可移植 layout 版本元数据，沿用 manifest 的数据库兼容版本位置，不加公开 DTO/命令。新版读取 schema35/旧 session archive 的兼容装入走原规则与已批准迁移，新 archive 用 core schema-ahead 阻止旧 Restore。

1. 备份协调用现有 maintenance/state 生命周期锁并固定一个可安全交付的开放票，锁顺序和扫描/Restore 一致；外层 backup 已持 core 时使用 held-lock 入口，禁止递归获取。通过票/epoch、保护 revision、优先级及排除确认两库可导出同一代。未开放处理可继续记录但不改变所选事实；导出期间权威输入/权限变化则丢临时出品重试或走既有 backup error，不能以两个稳定的跨代向量拼接。若没有可安全导出的完整选定状态（特别是不能证实可恢复的既有账或带不安全正文的索引），按原错误结束；不能成功输出一个貌似完整的空备份。core-only 仍可导出受保护过滤后独立安全事实，并保留恢复覆盖。
2. **Core** 快照仅含允许的已开放事实、合法原存量、当前用户状态及恢复所需的非内容已确认证据（含已撤去合法 OTel 的权威收据、实际绑定和所有权、已批准的不完整/缺源保护）。discard 未开票、计划、暂拟结果、未开收据及临时缓存；确认旧侧已失效的公开行退出可移植读面，其必要的非内容历史证据仍可按组保留。将纯待发表状态转为 restore 需重新盘点的原有 pending/unknown，将已知缺源/歧义仍保留为 partial，不能自动提升完整性。既有非可移植 origin state 清理仍执行。**Session** 从固定票的有效来源、metadata、exclusions、允许 FTS 文档及合法 checkpoint 物化至新的 SQLite 副本，保留 doc_id、与其同票的详情 ordinal/顺序验证状态和不回退的分配器，清已撤原文和旧尾，不复制任何私有候选表或打开中的底表。显式 --include-sessions 且已存在却无法导出同票的库走原错误；真实从未存在/已 purge 等经证不存在可省 session entry，禁止把打不开伪装成省略条件。
3. 不包含两个库的 live WAL/SHM/journal、原 legacy 尾缓存、候选 FTS shadow、派生缓存、Widget 快照、迁移镜像、原 JSONL、路径围栏内容。Core 的现有 SQLite snapshot 在私有副本净化后通过重写/重建干净输出去掉自由页残留；session 直接新建导出路径，避免依赖 FTS secure_delete 清影子页。先验证逻辑内容、manifest hash 和完整性后才在原目的地原子交付；检查点、计量引用和 archive 里的私有源定位仍按现有加密与最小值保留。
4. **Restore** 新旧 archive 均先验证净化结构、事实选代和原 archive schema。用原空目标锁、私有文件和失败回滚，目标导入期持久 fail-closed，创建旧路径围栏，私有 session 文件只在 archive 已包含会话时安装。新 mint core epoch、index epoch（如有）、root identity 与新 base publication；复验归档中详情映射的有效组、ID/ordinal 一对一和完整性并原序重绑，不按 rowid 重排；对缺映射的旧 archive 按 §7.2 兼容物化旧序列；原 ticket、inode、cursor 捕获/物理范围和派生 seal 不具此根授权，保留不可变历史证据并在首次盘点重新绑定，未验精确范围不能对新源产生新的 exact。旧有效计量分项、时间、路由绑定和可复核收据在兼容底座保全；保持来源无法重验的 pending/partial，按批准的保证显示旧安全值。Restore --without-session 缺少的域按原合同表达不可用或确认未建状态，不伪造 session seal，不能读源补库作为 Restore 副作用。完成后校验 store open 和旧路径实际拒绝，按原正常读面 bootstrap 新派生，绝不拷旧 publication blob；中途失败清仅本操作创建者文件或保留受管可恢复 journal，不能使半导入被当成健康。带不相容 ahead schema 的新 archive 旧程序在写任何库前拒绝。

迁移 layout 的组合维护与 backup/Restore 共用存在态和防重入协调，usage-only 的 core schema DDL 可以登记意图，不调用迁移旧 session 数据或创建私有 session 库。没有索引时原子放置非 SQLite 围栏不构成创建索引；已有旧布局未封存时 usage-only 仅能运行无联合依赖已证明的 core 工作，不能写新 session 候选。原组合入口在具备维护窗口时完成布局，之后才准候选写。旧 fence 的拒绝测试不依赖任何新 archive 私有字段。

### 9.2 诊断与隐私

`internal/doctor/doctor.go:171–198` 和 `internal/session/doctor.go:36` 迁至新 locator 的 read-only health view，明确 fence-only 与已建索引、真实不存在/已 purge 与 layout 尚未 ready、不兼容或数据库损坏；不因 fence 存在宣布已索引。已有 schema/FTS/integrity 的检查也核对该 schema 应有的 core evidence、receipt、interval、publication 结构、私有 session schema/candidate 结构、活跃票的 seal/epoch/coverage 摘要、旧触发器缺席和 fence 不可作数据库。预期未提交的 seal 缺失只走既有 pending/coverage 通道，完整旧代本身仍结构健康；活跃 seal 丢失、错 epoch、实际表损坏/旧触发器仍在则经现有 schema_incompatible / session_index_unhealthy 及已有读取错误，不能把损坏当作合法空。source-level doctor --full 只按原作用域对有效且真实有定位的登记明文计不可读数，排除 OTel/synthetic 和未开放候选；只有真实重建失败或来源不稳使用对应旧码。可按原允许 probe 打开已登记路径但诊断不扫描正文、解压或修库；计数不等于修复条数，不新增原因码、不吐原 SQL、路径或 ID。备份/诊断绝不从私有候选读取用户文档。

两库、私有候选、游标/半行、parser_context、分类、活动、信号、用量、serving/Widget 和 trace 采用已批准的元数据/文本边界。只有 session 的有效 FTS 及同规则的私有候选允许经已证本地归属、最新排除的用户输入/最终回答。候选被推翻、purge 或排除时源/文档及相关备份读法先遮蔽，再按原权限及净化规则清理。系统/developer、暗推理、非最终 phase、工具参数/结果与完整目标路径、凭证、环境、附件永不写新增状态。允许正文也不进入用量、游标、派生或诊断。旧路径围栏不携带任何原 schema/数据；路径只在必要定位及私有证据引用中。

## 10. L3 验收矩阵（F12、F13）

使用隔离双库、原生合成 JSONL、安全合成 OTel、固定时区/目录价，按项目 test wrapper。故障注入在核心事务和实际读面发生，不用伪 DTO 代替。实现交付时重确认目标 main、围栏环境与 L3 证据。以下是待执行测试合同。

| 条件 | 实际调用与具体断言 |
| --- | --- |
| A1 | 从正常活跃/归档发现路径送 P/C/S/G，断言直接父 G→C、顶层/嵌套与 fork/root/self/cycle/guardian other/legacy/reverted/rename。头 C/名 P 用冷与独立已发表两对照，列表、FTS、详情、活动、模型、调用/金额及 P/C OTel marker 零新增、无撤销；旧冲突正文须退出。 |
| A2 | C 自有头后父历史，无第二 C 头，原生 C 请求右锚；请求后在 task_complete 前后读取最终回复，commentary/partial/hidden/old-no-phase 对照。相同 turn/call 使模型及身份不明文档/工具无公开，显式请求一次。对合格与越界工具、失效回撤、无父源、半行完成及重启核对活动、信号和搜索。 |
| A3 | 原生双格式与当前/归档复制下 P=120、C=35、All=155，两次调用，合成金额 P=140、C=90、All=230；固定原时区桶、模型与缓存/reasoning 主分项逐一核对，独立 routing 与错误归属单独对账。 |
| A4/A4b | 可重放旧累计区间 20 对 R=10：可能重叠为 10/1，确证包含为 20/1，独立为 30/2；同 ID 冲突撤到 0 确认，独立同数 ID 两次。全读、反序、跨批追加及重启，覆盖累计首次/清除/重置、缺 response ID、有无区间、缺主项对显零、坏数及缺价/模型。summary、stats、sessions、show token summary=20 与 invocation=10/count=1；Codex/All=20，同桶=20、跨小时不分摊、独立 M/Q、unknown 为零或不入行、份额 50%，known 10/20 与完整金额 null、桌面两级 pricing_complete=false/紧凑 cost_incomplete。投入坏私有 payload 证实不会得假零，删最后区间时检查 OLD/NEW client/model/day/aggregate scope 与缓存版本失效，Claude/OTel compatible 分区不全量失效；存量前后与冷聚合对拍。 |
| A5 | 先在 schema35 证明旧裸 INSERT 触发器的可达效果，再在 upgraded/cold 检查 trigger 缺席并在回滚 fixture 裸 INSERT：原贡献、usage、marker 不撤；正常 publisher 拒绝未经证明孤行公开。仅头/工具、仅累计、身份补填、prefix 未开票、候选回滚均无撤回及无已应用收据。P/C/S=7/11/13→7/17/13，再分别真 5、合法原 11 和无合格 0，验原 state/reason（包括当时 settled）、key/原日/价/路由、实际行、原 marker 和自动分类；无撤回/结算证明的历史 jsonl_owned 不能当收据。两种接收顺序、盘点 captured_at > received_at、等待及有 S 线索/无 S 线索、旧重叠、观察失效、重启、两轮结算，确认 pending/fingerprint/release 与无双计。 |
| A6a/A6b | schema35 同一真 P=40；旧 20/new10 的阶段差别为 50 vs 60，补证得到 60 或 70，唯一替换不双算。普通 budget/取消/半行/源改写/并发 CAS。验证范围内旧本地镜像+范围外新行沿用原 exact provider/multiplier/reason/时间与 original route；继承副本、冲突和无效范围不转移。纯重归金额守恒、去重、漏采、路由/价覆盖原因分别核对；冷建等价历史证据对拍。 |
| A6c 发布/读面 | 故障点依次停在 core prepare、session seal、slot 持久化但 seal 尚未登记、已 seal 而 core 未开放、最终 core rollback 及 core 开放后镜像前，重启时固定向量。验证候选写源/doc/分配器不推进公开 facts revision、不触发既有底表 pending；只有独立 coverage 转移影响指定区间完整性。验 §7.6 每个真实 CLI/desktop consumer 的安全旧票、仅验证 All 和没有安全旧版，候选/底表双哨兵、空探测与 raw SQL 护栏。FTS 旧/新两表 doc_id 碰撞负样本、同时间排序、排除最高优先副本、替换零匹配和多页 count；在无逻辑变化的开票或 compact 前后验证 MATCH 的顺序、总数与 ID 不变；存量读不会回去看新文件内容。普通 usage-only append 真实提交且无索引 open，跨域组持有/重启、combined resume 后对应 warning 消退，其他缺源 warning 留存。 |
| A6c R2-F1 详情顺序 | 旧库 A/B/C 的 doc_id=1/2/3，重解析将 X 插到来源位置 2，X 的新 doc_id=4。旧票完读 ABC、total=3。以相同明文冷建作参考，候选有效视图、新开票、compact 与备份/Restore 后的全读都必须为 AXBC、total=4。limit=2 时逐页为 AX、BC，shown=2/2、has_more=true/false，第三页为空且 total=4。对 DocumentsPage、ShowWithActivity、实际 CLI show/--all/--activity、viewer 文档页核对。纯追加 D 后对拍 AXBCD；同一记录多文档、相同时间、旧资料不可复验及更高优先级副本被排除各给对照。在 count 与 page 间故障注入新票或排除，仍返回同代完整页或原读取错误；viewer 换代不续旧页。活动验证同一票的单独顺序与警告；另外断言保留 ID 与增量 FTS 同时间 tie-break 不因详情修复、compact 或 Restore 改变。 |
| A6c R2-F2 耐久边界 | 用可控屏障分别挂住 slot 文件 fsync 与目录 fsync；不释放屏障时，让另一连接成功 BEGIN IMMEDIATE/提交无依赖写入，验证调用方按既有路径得到安全旧票/待处理，未打开任何新票。注入文件或目录 fsync 失败，确保无 seal、无 OTel/usage 换侧；目录同步失败覆盖 rename 已可见的文件。放行超时 attempt 仍拒接收迟到结果。正常封存后在最终 CAS 前变动一个依赖 revision，检查拒开并保全旧结果；顺序再试无变更成功，比较真实生效值、票据与 payload。分别在持久化后未登记 seal、已 seal 未开票和 core 已提交未物化公共镜像处重启，检查旧/新读者与票据相符、候选不被提前读取、旧安全代未混新数据，合法重试不重复 OTel 换侧。断言阶段内写事务/维护锁持有状态，用屏障先后关系判断，不靠 wall-clock fsync 阈值。 |
| A6c 兼容/备份 | 基线提交 Go 二进制与升级 fixture 对照，旧 list/search/show、排除、rebuild、watch、telemetry projection、doctor/serving、backup/purge/state migrate/Restore 实际可达（必要时用同版本包的小入口）。捕获旧 reader 的 Sqlite NOTADB、旧 writer 在 WAL pragma 或连接前置 pragma 失败且 migrate 未达；hash 确认 fence/私有索引不替换、部署途中断点无 staging 暴露。旧 Restore 拒新 archive/非空目标；新 Restore 验 old archive、新 archive、含/不含索引、中断后回滚。两库提交之间与派生 seal 前备份，解密测试副本无候选或不安全内容，打开同票或已有备份错误；Restore 重绑 epochs、覆盖和保护，旧 serving 文件与旧票不能接受，覆盖/合格收据在重启后按规则有效。doctor 对合法 pending、损坏 live seal、ahead/open failure 与实际不可读旧路径比对原状态指标。 |
| A6d | 缺、不可读、仅余压缩三种已登记旧源保全，A5 有效独立 OTel 结算不受未登记压缩影响；可读合法替代交接及时清旧；未登记压缩不产生条目。显式 purge 清会话与 pending/search/旧快照，并留围栏及 core 消耗；排除不会通过子副本或还原旧快照复活。只用通过隔离 fixture 的路径。 |
| A7a/A7b | 验 08:00 父、09:00/09:01 子头、09:04 执行、09:05 OTel birth 优先级、相同时区时刻、改名、复制和重启；同 birth/有证不同/无证不同分别查询 persisted digest/state、applied、effective、list、Work/Auxiliary/All、并发 clear 与用户 revision；ID/path/project/client 及合格副本。隐私双哨兵覆盖同代/pending/compact/backup/恢复/诊断/trace，半行补齐前后逐表（含旧列、FTS shadow、cursor、parser_context 和档案净化）确认禁止正文零出现，允许正文仅在已证未排除会话搜索/详情，合成错误日志无原文。 |

来源和交接的假设沿用需求。另需以当前目标二进制运行 §7.1 围栏测试，验证管理端能达到受管 quiescence、SQLite driver 实际拒绝和原状态锁顺序。现有旧连接的 FD 不可撤回属于部署边界，任何时点可证明新布局不在旧固定路径。如果要求无停机且撤回已开始的旧连接读取，现有旧 writer/read pool 无合作协议可提供该性质；本需求只要求正确迁移、围栏与新发布，首选受管迁移窗口。当前设计不需新增产品决策；若实际消费者无法纳入既有选择层，再以该具体入口报主控。原 Bug、发布分工和迁移编号按主控权威继续。

### R1 实施验证的具体接线

- `cmd/agentdeck/main.go:1862–1925` 的 sessionCheckpointFingerprintFromRaw、persistedSessionWatchFingerprint 与 sessionWatchFingerprintFromRaw 也切到 locator+有效 epoch、已开放和待处理的对应指纹。对于有共同依赖但仍待处理的组，不返回与已确认指纹相同的值；未受影响集合复用原指纹。普通 scan 的 current target 不得从 pending 计划读出为已开放版本。
- `cmd/agentdeck/main.go:3558–3578` backup CLI 当前先 openStore，修改编排时将必要的 maintenance 获取放在 state lock 之前，再使用含锁入口准备读快照；禁止持有逆序并等待另一锁。对旧态与新态捕获都检查实际 DB 是否支持归属，不只检查 schema_metadata。受管迁移/Restore 写旧 fence 前删除或保全属于旧文件的 sidecar 内容，等 SQLite 退出后仅在同一次恢复路径下重新创建，测试旧进程在围栏后自动创建的**空** sidecar 不会使其变回数据库。
- 源码基线的旧 writable core 顺序为 `store.go:400–447` 打开 state lock、WAL、`migrations.go:551` 迁移/版本判定；readonly 是 `store.go:340–361` 直接读版本。用仓库旧二进制 fixture 断言旧 purge 在 file remover 被调用前返回 ahead，旧会话重建在调用能删表的路径前停止。生产 layout-ready 能力由 fence 验证、core 版本和实际私有索引 seal 三者共同建立；如果旧 session 仍处于迁移等待或失败中，任何 writer 不能用旧 file fallback 完成新发布。
