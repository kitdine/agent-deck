---
status: active
created: 2026-11-17
---

# v0.7.5 版本契约 — Review

## Round 1 — 2026-11-17

完成门禁：NOT_VERIFIED

2026-11-17规范字段补记：主控对原候选b67d651d901630dd7db31294edfbb41c95ed29b906b3649677056730642b7335回查此值，不改变原FAIL或推测历史查询。

对象：tasks.md；HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，blob `fd2866d32c0a91ce0f8b8623981b155abc5e7fb4`，文档指纹 `b67d651d901630dd7db31294edfbb41c95ed29b906b3649677056730642b7335`。
独立方法：冷上下文 Claude/Sonnet 5.5（requested effort=high，实际档位不可独立观测）；文档、Beads 与相关源事实检查。此前 Fable 配额失败与 Argon EOF 均未产生评审轮次。主控核对下列规则与源事实后记录正式结论。

## 📋 版本契约评审报告

📊 总体评分：7.5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**V075-CONTRACT-R1-F1 — 中：条件分支的重新分诊与迁移分配边界不完整。**
位置：成员矩阵及依赖/写入边界。新 wire 字段、新持久语义或产品优先级不能继续以 A 修复名义决定；迁移需要明确负责者与已验证基线，且共享 Widget/wire 写入也需串行协调。
证据：Bug lane 以是否需要新契约决策划分；当前 main 的 CurrentSchemaVersion=35。文档只禁止并行抢编号，没有指定分配者。主控不采纳“多改一个原生文件就必然属于 B”的推断：文件数量和路径不是 lane 判据，恢复既有语义仍可为 A。
💡 修复：写明真正的新契约触发条件、同范围已授权设计与越界待决的区别，指定主控在实际集成状态上分配迁移编号；补全 App/Widget/shared wire 的共同写入边界。

**V075-CONTRACT-R1-F2 — 中：Claude 同一 Bug 的既有行为恢复与新账号能力没有明确载体和关闭条件。**
位置：claude-quota-output-plan 成员行与所属主题要求。历史坏 stdout 未复现，不能虚构 parser 失败先行证据；同一 issue 的 A/B 工作不能分别以局部通过关闭整体。
证据：Lane A fix carrier/WorkUnit 与 Lane B origin/topic 生命周期不同；当前 issue 同时包含历史 parse_failed 和 auth status 账号/套餐范围。
💡 修复：明确由同一个功能主题承载该 Bug 的整体修复和诊断处置，原 Bug 保留 origin；没有复现证据不做猜测性解析修改，关闭需全部选定验收和授权交付。

### 🟡 建议改进 — 推荐修复

**V075-CONTRACT-R1-F3 — 低：部分允许文件只给简称。**
矩阵中的 MenuBarViewModel、WidgetViews、DesktopWire 等应解析为实际仓库路径，避免给不同 Worker 不同含义的写入边界。
💡 修复：补共同路径映射，保持表格可读。

**V075-CONTRACT-R1-F4 — 低：版本收尾未列出归档和导航责任。**
证据：Documentation Workflow 要求授权交付与证据最终化后归档 fix；完整主题整体归档并更新归档索引，fix 不另加索引项。
💡 修复：在最后任务列出条件性归档、元数据及失效链接对账，不提前执行归档或改写 main。

**V075-CONTRACT-R1-F5 — 低：尚未明确替代 subscription-quota 的旧限制。**
证据：subscription-quota/architecture.md 的 planType 表为 opaque、不可分支；requirements.md 的 Claude 账号/套餐缺失是当时两通道的限制。新主题必须明确修订范围，而非让新旧约束并列竞争。
💡 修复：引用已建立的 subscription-account-identity 草案与工作区，要求其逐条声明替代关系；不改写已交付历史。

### 🟢 优点

13 项范围、优先级、模型行方案 B、Cask deferred、serving 候选来源、隐私和交付上限均与当前权威一致。版本文档没有冒充功能设计或代码完成。

### 📝 总结

五项均须修正后复评；F1 仅按真正的新契约决策重新分诊，不能把文件边界扩大本身当作产品变更。L0 结构与空白检查在被评 blob 通过，reviewer 独立 diff 检查通过；未验证产品运行时，也未宣称 CEv1 VERIFIED。

下一步指令：修复：v0-7-5-contract / tasks.md / V075-CONTRACT-R1-F1, V075-CONTRACT-R1-F2, V075-CONTRACT-R1-F3, V075-CONTRACT-R1-F4, V075-CONTRACT-R1-F5

### Round 1 有界修复交接

五项已写入新候选，待独立复评：F1 增加真实新契约触发规则、既有授权与超范围决策的边界、主控迁移编号分配及 schema35 基线，并覆盖 App/Widget/DesktopWire 共享写者；F2 将 Claude 原问题整体归同一功能主题，不分拆关闭，无法重现坏 stdout 时不猜测 parser 修复；F3 补实际路径映射；F4 增加授权生命周期边界的 fix/主题归档及导航责任；F5 明确复用已存在账号主题草案并声明对 subscription-quota 旧约束的有限修订。
新 tasks.md blob `d7a171e7fc7c51d5ec9134cc2d8739305327551b`，HEAD 仍为 `4ac56db32e13c4e925b648c620ba76fc588a7781`，文档指纹 `1a61ec964127b4e7b64e964f380195401e4ef2dcf1a418be3122d52ddb7d9e18`。结构、whitespace 和 diff 检查通过，没有先行勾选 Review。

## Round 2 — 2026-11-17

完成门禁：NOT_VERIFIED

2026-11-17规范字段补记：主控对原候选1a61ec964127b4e7b64e964f380195401e4ef2dcf1a418be3122d52ddb7d9e18回查此值，保留原FAIL和finding。

独立冷上下文 Claude/Sonnet 5.5，requested effort=high；对象为上方修复后身份。Reviewer 建议 PASS 但同时记录两个实际范围缺口。主控按“所有真实范围内 finding 关闭后才可 PASS”记录 FAIL，不沿用该建议的严重度豁免。

### 📋 版本契约复评报告

📊 总体评分：8.5/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

无高严重度发现。

#### 🟡 建议改进 — 本轮必须关闭

**V075-CONTRACT-R2-F1 — 低：跨成员共享原生路径的登记边界仍不完整。**
账号主题、token mix、模型行与可能的 serving 读取修复均会触及 shared wire、Widget 或 App 菜单栏文件，而当前规则只分别约束组内和单工作区。主控核对矩阵列出的文件范围，确认需要覆盖跨工作区的同文件协调。
💡 有界修复：明确所有上述成员共享路径统一登记唯一写者或串行顺序，不能只约束某两项。

**V075-CONTRACT-R2-F2 — 低：分类性能路线的授权措辞不一致。**
矩阵允许索引/迁移“明确边界及编号”后继续，而后文又要求先分诊；“已确认的无迁移边界”没有明确对应本次用户的路线委托。Beads 的旧 lane-pending 是历史来源，不可当作另一次逐项确认。
💡 有界修复：明确本轮 A 只适用于无 schema 变化的预聚合恢复，依据是本次修复方式委托；若必须索引/迁移则停止并重新分诊，超出授权才等待用户，不把编号分配视为充分授权。

**V075-CONTRACT-R2-F3 — 低，措辞观察。**
批次 B1–B4 与 lane B 可能混读；现有列和定义已区分两者，没有据此发现错误派发。为关闭记录中的歧义观察，统一改为 Batch 1–4，不改变成员或优先级。

#### 🟢 优点与既有处置

R1-F1 至 R1-F5 均 CLOSED：新契约与文件数量区分、schema35 与主控分配、同一 Claude Bug 整体交付、路径映射、授权归档、有限替代旧主题约束均已核对。Reviewer 独立 L0 结构、空白及 diff 检查通过，未评审未来功能设计或宣称 CEv1 VERIFIED。

#### 📝 总结

本轮仅剩两处真实的协调/路线措辞缺口与一处可一并澄清的批次用语。范围内问题即使严重度低也不能遗留到 PASS 后；进行第 2 轮有界文档修复，再针对这些闭合点复评。

下一步指令：修复：v0-7-5-contract / tasks.md / V075-CONTRACT-R2-F1, V075-CONTRACT-R2-F2, V075-CONTRACT-R2-F3

## Round 3 — 2026-11-17

完成门禁：NOT_VERIFIED

2026-11-17规范字段补记：主控对原候选c4dbef859479669b0caad0e8c3bc7a51d71383ae556ed26fb72a09ab5e5d7625回查此值，保留原FAIL和finding。

冷上下文 Claude/Sonnet 5.5，requested effort=high，只读有界复评。对象 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、blob `e55eb016c80f0210a75a024e71668f31e2a22e0c`、指纹 `c4dbef859479669b0caad0e8c3bc7a51d71383ae556ed26fb72a09ab5e5d7625`，Reviewer 已独立复算并通过 L0。

### 📋 有界复评报告

📊 总体评分：8.5/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

无高严重度发现。

#### 🟡 建议改进 — 本轮必须关闭

**V075-CONTRACT-R3-F1 — 低：新增共享文件没有进入路径映射。**
依赖条款新增 MenuBarPanelViews 作为共同登记路径，但简称映射未列它，模型行成员只写 App 模型行。主控核对文档实际缺项及目标文件存在，确认映射与登记范围未完全对齐。
💡 有界修复：补该文件的完整路径映射，并在模型行范围直接点名；共享测试使用实际存在的完整路径，其余测试由所属批准任务明确，不增成员/路线/权限。

#### 🟢 既有闭合

R2-F1/F2/F3 全部 CLOSED；R1 五项保持 CLOSED。跨成员写入登记、无迁移性能边界、主控迁移分配、Batch 用语、13 项成员和 none 交付上限均已核对。

#### 📝 总结

仅余一个新路径说明缺口，未批准 PASS。进行第 3 轮有界修复后，只需对该修订和必要回归做独立复评；修复计数不得因换模型或新请求而重置。

下一步指令：修复：v0-7-5-contract / tasks.md / V075-CONTRACT-R3-F1

### Round 3 修复交接

主控已补 MenuBarPanelViews 的完整映射和模型行范围，并核对 MenuBarViewModelTests、DesktopWireTests、EmbeddedHelperRunnerTests 实际路径；没有改变成员、优先级或交付权限。第 3 轮修复已实现，待独立确认；不先行勾选 Review。


## Round 4 — 2026-11-17

方法：冷上下文Claude/Sonnet5.5，requested effort=high；主控核对独立报告、最终候选、完整路径和授权同步，记录正式裁决。被评HEAD为4ac56db32e13c4e925b648c620ba76fc588a7781、tasks.md blob为235afee2c9838b7e613cc6aa113b45aebf0d032c，指纹b04ba6490c2c4184bcbfc1c27691e55fe54e114c52f28b9f2d66fb1994679671。运行时model=claude-sonnet-5-5，实际effort unknown；终态无子任务或待执行操作。

### 📋 最终有界复评报告

📊 总体评分：9/10

评审结论：PASS

完成门禁：VERIFIED

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无遗留范围内事项。

#### 🟢 逐项关闭及保持

- V075-CONTRACT-R3-F1 CLOSED：模型行范围直接点名MenuBarPanelViews.swift；简称映射指向apps/macos/AgentDeckApp/MenuBarPanelViews.swift，三项共享测试指向真实完整路径，登记范围与映射一致，其余路径须所属任务批准登记。
- V075-CONTRACT-R2-F1 CLOSED：所有成员跨工作区共享原生文件与测试由主控登记唯一写者或串行顺序。
- V075-CONTRACT-R2-F2 CLOSED：性能A仅限无schema预聚合；需要索引/迁移先重新分诊，迁移编号主控在实际main分配，不把编号当授权。
- V075-CONTRACT-R2-F3 CLOSED：批次统一Batch1–4，方案字母不与lane混用。
- V075-CONTRACT-R1-F1至F5保持CLOSED：新契约触发规则、schema35基线、Claude Bug整体承载、路径映射、授权归档与旧主题有限替代均保留。
- 新真实用户授权已同步：review/gate后按issue提交、push origin、建PR；push/PR Actions全绿后Codex review并处理真实发现。merge/install/release/deploy仍未授权，assemble和版本任务保持未完成。

#### 📝 总结

13项成员、优先级、模型行B、无schema性能A、唯一原生写者及Cask deferred未变。复用被评内容上的check-topic-docs、whitespace/diff通过结果，未跑Go/Swift或评审未来实现。主控在PASS后仅勾选tasks.md的Documents Review。最终文档 blob 为 `14b97f89fc8a1eab0caa483536a8cb63e9a4f276`，CEv1 以 `v0-7-5-contract:tasks.md:state:b1aebbc204f9aba11ee0d9f8b5f16754015cc169e6ca9ba70de1d98aab522b4f` 确认三项必需证据有效：独立评审、格式检查和范围契约。原评审仍绑定被评 blob。历史三轮规范字段补记保留各自的精确状态与 FAIL，repair_count 为 3。此门禁只覆盖版本契约文档。

## 2026-11-18 — CI 规划增量

用户新增 `ad-bug-ci-runner-and-verify-duration`，当前版本规划共 14 项。CI issue 纳入作业平台审计和 `Verify` 执行耗时优化。登记证据来自主线运行 `44306059099` 及其分类作业和测试日志，实施范围由该 issue 承载。

Round 4 的评审和门禁绑定原 13 项文档状态。当前 Review 待核对新增规划范围，已批准成员的实现与证据沿各自工作单元继续推进。

## Round 5 — 2026-11-20，CI 增量与当前发布授权

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 阻塞发现

**V075-CONTRACT-R5-F1。** 候选仅引用已批准的本机配置边界，原生验收条款只列隔离 App、进程和窗口的绑定。执行者在受限环境验收时，无法仅凭条款判断切换或新建工作树、修改本机配置或设置 VoiceOver 是否可用。主控已在原生验收条款明确使用现有绑定工作树及现有本机配置，以及上述依赖项保持未验证。修订待独立复评。

#### 🟢 通过范围

版本 14 项成员与 Beads 精确核对，10 项关闭、3 项开放、1 项延期。CI 修复已随 PR #78 合入，push 和 PR 各 9 检查通过。当前用户授权覆盖通过门禁后的合并和稳定版发布。成员验收、版本组装、稳定契约对账与发布沿各自证据边界。保留 Cask 的延期。

#### 📝 摘要

T3 Codex `gpt-6.2-astra`，`reasoningEffort=high`。请求 `v075-e21-contract-ci-authority-review-ba80a33c`，epoch 21，HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，文档 blob `ba80a33c2c9efc1c51d3ad3040bb5792b61a33c0`，内容指纹 `79a662446c8020488995abc04c784df63776ff75801ddb0986773c0a53a70d9e`。评审复用正式 Round 4 记录与可读的原 13 项最终文档 blob，复用此候选的主题文档、whitespace 和 diff 检查。GitHub 查询确认最新公开稳定版为 v0.7.0。

## Round 6 — 2026-11-20，原生验收授权边界复评

### 📋 独立复评

复评结论：PASS

完成门禁：VERIFIED

#### 🟢 已关闭发现

**V075-CONTRACT-R5-F1 CLOSED。** 修订条款明确使用现有绑定工作树与现有本机配置。验收依赖工作树切换或创建、修改本机配置或更改 VoiceOver 设置时，该项保持未验证。隔离夹具、应用身份、进程、窗口及实际原生观察均仍适用。保留已评审的 14 项成员、PR #78 CI 增量、Cask 延期、版本组装与发布边界。PASS 覆盖本版本契约文档。

#### 📝 摘要

T3 Codex `gpt-6.2-astra`，`reasoningEffort=high`。请求 `v075-e21-contract-native-boundary-rereview-33ec51cf`，epoch 21，HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，被评 blob `33ec51cfc81bd0017a1a35bc0b7b0882daca87d9`，内容指纹 `0f33ccf10aeacb5ff4f2db8e61789637fc82be7c4a17a3a167a83f5f54c573d7`。主控核对版本契约及原生条款，只同步文档 Review 单元格，产品集成与对账仍待其自身门禁。

评审状态同步后的文档 blob 为 `38d42d9357f391e8d3269aada3729f5e82ce8e08`。CEv1 以 `v0-7-5-contract:tasks.md:state:a2d35ba4a790a77675fa7a48d63d2f0c31fb3df45cbabb79af0c56e03f740e7e` 确认独立评审、格式检查和范围契约 3/3，缺失、失效及待决影响均为空。四个节点的属性与六条关系的身份和端点已精确读回。
