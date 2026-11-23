---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — Architecture Review

## Round 1 — 2026-11-17

对象：HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，architecture.md blob `9f24920b6696f0d9c0c2fa60089f1c804c4d2ad2`，SHA-256 `43a3f2a398724c3b207d62066a3f1ae25d382f0d01437f3158ad7fe4a0009d45`；实现对照 main `a224b0b824d2cad451d3bd83153832d65506f0fe`。草稿来自只读 Claude Opus 5.5 设计 worker，由主控按用户 2026-11-17 决定 D1（Widget 仅 large 显示脱敏邮箱）、D2（已知 Claude 上下文变化后重新确认前忽略状态栏捕获）、D3（失效只适用于 Claude）写入。

独立方法：冷上下文 Codex，executedModel gpt-6.2-sol、reasoning effort=high（运行时提供），与 Claude 起草者不同 harness；只读静态调用路径与事务时序推演，未运行测试或真实客户端。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 5/10。基本方向成立：独立身份投影、Claude 额度恒为归属未确认、Codex 保留原账号隔离、新增字段保持 WireVersion 1。Reviewer 确认 `staleAgainstCurrent` 只比较 `ObservedAt`、旧 Swift 遇未知原因值会拒绝整个快照、helper 刷新预算 40 秒、备份经 SQLite online backup 后只净化部分表、手动 usage 刷新绕过间隔与退避。

### 发现

- SAI-ARCH-R1-F1 高：世代只保护 Claude usage 成功写入。旧失败仍经 `recordFailure → PutEnvelopeFailure` 写入，可在失效后覆盖新世代的失败、尝试时间与退避；状态栏命令先读 stdin、执行原命令、最后以 `time.Now()` 写入，跨过失效与重新确认时会被 pending 检查接受；已发出的旧提醒 ACK 可在清账后补回去重记录。修复：世代校验覆盖成功、失败、窗口替换与提醒 ACK；捕获入口记录到达边界，链式命令结束后在同一写事务比较边界、世代与 pending，保留原命令先执行的契约，以跨进程屏障测试。
- SAI-ARCH-R1-F2 高：D2 的"同一周期成功"可能永远等不到——身份与 usage 到期分钟错开时两侧都成功却无共同周期；A5 允许失效后新 usage 显示，却保留 `invalidated_reason` 直到联合成功，Widget 会持续隐藏新窗口。修复：规定 pending 期间的成对调度与两侧退避约束、导致失效的当轮能否参与重新确认，并分别定义"当前额度可展示"与"允许状态栏恢复"的条件。
- SAI-ARCH-R1-F3 高：数据库事务与 serving revision 不能单独阻止旧数据再发布。`BuildSubscription` 分别读取窗口与 envelope 可跨过失效提交；serving 缓存在 revision 变化时仍返回旧 payload 只标 `stale`，Swift 启动恢复仍可安装；A11 裁掉身份时间后，Widget 无法按 `2 × interval` 让邮箱过期。修复：一致的 quota 读取事务、缓存恢复时的 quota 校验与清理规则、Widget 判断有效期所需的最小字段。
- SAI-ARCH-R1-F4 高：Codex `confirmed` 未绑定当前显示的额度。只存 `association` 与观察时间，下一轮仅 usage 到期、窗口已换成另一账号时仍留 `confirmed`。修复：绑定同轮成功提交的身份与额度版本，任一侧被替换、门禁重新验证或证据过期即降级，覆盖身份单独成功、额度单独成功、共同键不匹配与一侧写入失败。
- SAI-ARCH-R1-F5 高：Claude 转换规则遗漏迁移后的首个权威退出状态（v35 有额度无身份行，首次 `auth status` 即 `signed_out`/`non_subscription`）；一行未区分当前尝试、上次成功与比较基准，`identified A → unknown → identified B` 无明确覆盖顺序。修复：含迁移初始态的转换表，区分三者，定义退出后旧展示值清理与重复退出的幂等性。
- SAI-ARCH-R1-F6 高：快照阶段的 `secure_delete` 不能证明备份无身份残留——旧值可能已在线上库空闲页，Restore 的 `DELETE` 只证明逻辑行消失。修复：归档前重建或压实已净化数据库并 checkpoint，验证归档实际字节，明确 Restore 对带身份档案的净化边界；测试以独立的邮箱、组织名、关联值哨兵先写入再替换，检查逻辑行与 core 条目字节，不以全档扫描 `@` 代替。
- SAI-ARCH-R1-F7 高：白名单字符串仍可夹带禁止外流的数据（如 `workspaceName` 或 `planTier` 为邮箱）；现有 Codex RPC 错误包装含 `env.Error` 原文，Claude parser 错误可能含整行。修复：在身份适配边界定义安全显示值、字段类型与跨字段泄漏检查，把共享传输与 parser 的外部错误限定为安全类别，日志、wire、数据库、错误文本使用不同隐私哨兵，并给出 Unicode 可见字符与双向控制的可测边界。
- SAI-ARCH-R1-F8 中：本机秘密缺少崩溃与并发创建契约——对最终路径直接 `O_EXCL` 时读者可能看到空文件、崩溃可能留下短文件；所引用的 vault 实际先写私有临时文件、同步、再硬链接发布。修复：完整文件原子发布、打开后校验、旧 key 丢失时的世代处理，覆盖并发首次创建与中断重试。
- SAI-ARCH-R1-F9 高：迁移与兼容查询缺少落点。直接把 `quota_identities` 加进现有 trigger 表清单，全新库会在 v31 对未建成的表建 trigger，已在 v35 的库不会重跑 v31；只读 guard 放行 v27–29，但 `Envelope` 固定查询 v30 才有的 `failure_observed_at`。修复：新迁移建表后安装 trigger，明确旧版本 SELECT 投影与默认值，分别测全新库、v35 升级与带已开启 quota 的旧只读库。
- SAI-ARCH-R1-F10 高：21 秒只是子预算之和。Codex 当前 5 秒包含握手、额度响应与 `Wait`，阻塞式 `Scanner.Scan()` 无单请求超时，`account/read` 卡住会吃掉 usage 预算，取消子进程则无法在同一进程发 usage；A12 只给已发 RPC 的一侧记失败，握手前失败可能让到期的一侧没有退避；另有 20 秒跨进程锁等待，叠加后超过 helper 40 秒。修复：单 reader 按请求 ID 分派、忽略超时后的迟到响应、共享进程致命错误的归类，锁等待与清理纳入总截止，并用假进程覆盖静默、迟到、畸形、非零退出、取消与锁竞争。
- SAI-ARCH-R1-F11 中：A0 缺整条 C3（同意、原命令链、字段缺失、大小限制、关闭读取时注销），C9 应补记手动失败不推进后台退避与当前跨进程 quota 锁，C6 的 `plan_reason=not_reported` 仅适用原生缺字段语义，C2/C8 应写明原始 `accountId` 只在进程内处理而既有摘要仍用于隔离；旧开放问题需逐项标处置；CLI 示例应为 `agentdeck --format json quota`。
- SAI-ARCH-R1-F12 中：A4 未处理"可信官方展示名优先"，未定义 `raw_only` 的 `short_name` 与消费者回退（`short_name` 为空时 Widget 可能漏掉原套餐）；A10 缺身份来源、最近失败与上次成功同时表达的契约；A11 的 App Group 保留说明混入 `identity.plan_display`。修复：逐字段定义 absent/null、来源、展示回退与封闭值集合，列出 App Group 写入与 Widget 读取的允许列表及中英文消费者。
- SAI-ARCH-R1-F13 中：验证策略不能区分诊断恢复与历史根因。修复：把 schema、退出码、命令参数冻结为带版本来源的夹具并定义未知版本保守降级；验收用同一隔离 helper 构建绑定源码与版本，验证"有效两行 → 缺行、坏形状、坏时区 → 成功恢复"，断言成功清除失败细分，额外模型行从未进入窗口；区分取得批准的脱敏证据与自动化运行方式。

逐项：需求与范围未完整覆盖；A0 需修订；当前实现事实核对与 Reviewer 结论一致；隐私、并发一致性、测试与隔离未通过；schema、wire 与预算部分成立。

进入第 1 轮有界修复：由只读设计 worker 起草修订稿，主控核对后写回 architecture.md，再交冷独立复评；UX 文档与原型改动同步检查受影响字段。

### Round 1 修复交接

Claude Opus 5.5（请求 effort 为 high，实际级别未知）在原线程续跑并核对草稿。主控确认旧稿 blob 和修订稿 SHA-256 后写回 `architecture.md`。新候选 blob 为 `57cd0606281635186ec14ae1fab1fdf46c254376`，HEAD 仍为 `4ac56db32e13c4e925b648c620ba76fc588a7781`。主题文档审计和变更路径空白检查通过。

修订覆盖世代保护、成对调度与展示条件、一致读取、Codex 关联绑定、迁移初始态、备份压实与恢复、隐私边界、本机秘密、旧库投影、刷新总预算、旧契约处置、wire 和隔离验收。D1–D3 保持原决定。套餐回退、邮箱有效期、失效解除时机和 App Group 字段允许列表仍需在 UX 修复中与架构对账。待冷复评确认 F1–F13。

## Round 2 — 2026-11-17

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、blob `57cd0606281635186ec14ae1fab1fdf46c254376`。实现对照 `a224b0b824d2cad451d3bd83153832d65506f0fe`。冷独立 OpenCode `openai/gpt-6.2-astra`，请求 variant 为 xhigh，实际档位未知。独立复评结合固定 Git 基线、CodeGraph、共享契约和合成内存 SQLite 谓词探针。主控核对候选、当前来源常量、同时间优先级、捕获入口及 Swift 持有发布物的释放路径。

评审结论：FAIL

完成门禁：NOT_VERIFIED

本轮精确目标的 CEv1 查询返回 `missing_work_unit`。D1–D3 可保持，无新增用户决策。

### 既有发现

- **SAI-ARCH-R1-F1 OPEN（高）**：旧捕获在 `io.ReadAll` 返回后获取墙钟，慢输入可使已在途载荷被视为新到达。时间相同或墙钟回拨时，`arrivedAt < context_since` 放行旧推送，同刻还会按 C5 让状态栏覆盖 prose。连续两次失效后没有入口世代可核对。内存探针中谓词误用 `claude_statusline`，实际 `claude_status_line` 插入 1 行。修正拼写仍无法阻止迁移前已开连接在解禁后写入。必须定义可跨进程验证的开始与确认顺序，禁止旧协议写入，保留原命令先执行和 stdout 行为。
- **SAI-ARCH-R1-F2 CLOSED 原反例**：成对调度排除永久错峰，首个新 usage 可单独清除显示失效。一致读者在此前可观察到已提交失效，修正无中间态的承诺。后台间隔问题由 SAI-ARCH-R2-F1 承载。
- **SAI-ARCH-R1-F3 OPEN（高）**：Go 读事务和恢复重投影排除原混读。Swift 的 `releasePublications` 仍能在安装新 quota 后回装更早的 held publication。Widget 单个 timeline entry 可能在邮箱过期后继续展示数分钟。定义单调安装以及截止时不含邮箱的 entry。
- **SAI-ARCH-R1-F4 CLOSED**：共同键关联绑定当前真实提交，单侧更新降级。
- **SAI-ARCH-R1-F5 OPEN（高）**：同刻失败无法覆盖成功，回拨后权威退出被当作旧写丢弃。以可验证的尝试次序决定状态，墙钟用于展示。
- **SAI-ARCH-R1-F6 OPEN（中）**：新备份压实已覆盖原反例。Restore 面对身份表已空但空闲页仍有哨兵的外部档案，无条件字节保证不成立。统一净化范围或限定字节保证。
- **SAI-ARCH-R1-F7 OPEN（高）**：拒绝新 display 仍让相同不安全值从旧 `plan` 持久化及输出，例如与被禁止的账号标识相同的套餐值。五字符组织名等于组织 ID 会绕过最短六字符检查。为所有输出路径定义相同的禁止值处理，保留安全原值。
- **SAI-ARCH-R1-F8 OPEN（中）**：原子创建与并发读已覆盖。路径 `Lstat` 与打开可能解析到不同对象。必须在不跟随链接打开后的同一描述符校验权限、属主、类型和内容。
- **SAI-ARCH-R1-F9 OPEN（中）**：旧库可查询，但把未知 `failure_observed_at` 补为空会误以为任一状态栏成功覆盖后来的 prose 失败。旧库未知顺序应保守处理。
- **SAI-ARCH-R1-F10 OPEN（中）**：预算已包含进程、跨进程入口锁和清理概算，尚缺身份提交与启动 usage 之间的 SQLite 写锁等待。明确剩余预算及退场条件。
- **SAI-ARCH-R1-F11 CLOSED**：A0 补齐旧契约及命令顺序。
- **SAI-ARCH-R1-F12 OPEN（中）**：`decodeIfPresent` 合并缺键与显式 null，破坏仅旧 producer 缺键回退的规则。新对象 missing 时可没有旧 `plan_reason`，须用新对象自己的原因。
- **SAI-ARCH-R1-F13 OPEN（中）**：`desktop quota-refresh --manual` 需 JSON 格式，响应仅含门禁和提醒。验收每步另读 `agentdeck --format json quota` 或隔离库以断言窗口、失败及时间。

### 新发现

- **SAI-ARCH-R2-F1（高，待修）**：身份检查即便拿到 `signed_out`，草案仍强制在使用量退避尚有效时运行后台 usage。两侧均允许时提供一次真实联合调度机会，并说明不会因一直单独工作而饿死。
- **SAI-ARCH-R2-F2（高，待修）**：旧世代提醒排队后跨过失效仍会被 App 投递，拒绝 ACK 只能保护去重账本。定义投递侧顺序及丢弃行为。
- **SAI-ARCH-R2-F3（中，待修）**：Restore 拒绝任意非空目标，含现有身份秘密的目录同样不可达。保留空目标契约和既有供应商凭据归档范围。

### UX 对账

两表面消费 `quota_context.reason`。首个新 prose 后恢复本轮窗口，其归属仍未确认。缺键旧套餐回退应保留键存在性。Widget large 邮箱须处于已识别、当前且尚未过期，实际到期 timeline 由架构负责。分别复评架构和原型修复。

### Round 2 修订稿核对

Opus 暂存稿 SHA-256 为 `b50b647a695af3284d5a57878558d809fb41b91d73fc224e4f7301c50e05f51c`。稿内完成 A1.2 联合机会推导。A5.7 的时限租约仍有一个反例。App 在租约到期前通过最后一次时钟检查，随后暂停。helper 退出后，另一 helper 在租约到期时提交世代失效。App 恢复并继续向系统提交旧通知。余量大小不影响该交错，因此未把暂存稿应用到工作树。

同一修复轮次继续定义 App 投递与失效事务共享的进程间互斥。App 在持锁时复核世代并提交。争用达到截止时返回既有忙碌结果。验收须覆盖检查后挂起、进程退出、锁对象身份、旧 ACK 和同一提醒的重复投递。macOS 已受理的通知可能延后显示，需求中旧提醒的显示边界仍待核对，原文保留。Widget large 的关联提示另需对齐 A11 的字段允许列表。

### Round 2 修复交接

Opus 的共享锁稿 SHA-256 为 `26627115e191a1c97909643427931404185fb4b9848cf56243eb9e3820fad260`。主控将锁定对象改为由既有所有权会话保护的 stateRoot 目录，世代文件只存镜像。本机隔离探针确认两个目录描述符互斥，世代文件删除重建后仍互斥，释放后竞争方可以取得锁。架构要求失效提交不明时等待下次按实际数据库世代对账，避免倒退镜像。Codex 告警不受 Claude 投递锁竞争影响。A11 候选已纳入 Codex 的既有关联枚举，限定在 large 的当前登录行使用。

候选 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，architecture.md blob `0e1fe0a0978952aa2ec28b64e9ec0d0a0e112ef3`。文档结构与空白检查通过。独立复评需核对受管目录身份、跨进程抢占、旧 ACK、调用和周期预算、Restore 及与 UX 的字段对账。现有需求的旧提醒显示项仍适用。候选只提供 App 提交顺序的设计证据，系统已受理横幅的实际显示边界仍待验收判定。

## Round 3 — 2026-11-18

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、architecture.md blob `0e1fe0a0978952aa2ec28b64e9ec0d0a0e112ef3`，指纹 `f88fd460ad6f4e758faf67e846cbe7656dd7c7f7b9cae6e7cd06e1d2b023bceb`。冷独立 Antigravity 审查报告为 Gemini 4 Argon，档位请求 high，运行回执未单独验证实际档位。主控核对架构、需求和 Widget 文档的实际候选，以及本机隔离目录锁探针。

评审结论：PASS

完成门禁：VERIFIED

本架构文档 4/4。契约、顺序设计、格式和独立复评证据均绑定本轮指纹。

总体评分 9.5/10。以下旧发现按文档契约关闭。

- **SAI-ARCH-R1-F1、F2、F5、F9 CLOSED**。捕获入口时间及两步发布覆盖慢输入、同刻和回拨。旧写路径不能借保留列继续写入。可见额度与恢复捕获有不同完成条件。独立周期序号和只读版本投影覆盖失败次序及旧库未知时刻。
- **SAI-ARCH-R1-F3、F4、F12 CLOSED**。读事务及 Swift 安装使用同一订阅投影顺序，迟到发布物保留新订阅段。Widget 到期用独立 timeline entry。Codex 关联绑定实际共同提交，缺键、显式 null 与对象分别投影。
- **SAI-ARCH-R1-F6、F7、F8 CLOSED**。备份压实和恢复清理覆盖既有数据，字段过滤约束旧套餐及新展示路径，关联秘密在实际打开描述符上验证。
- **SAI-ARCH-R1-F10、F11、F13 CLOSED**。A0 对账旧契约，A1.5 计入 SQLite 和目录锁等待，设计最坏预算 37 s。隔离验收分别调用刷新和 JSON quota 读取。
- **SAI-ARCH-R2-F1 CLOSED**。两侧按退避参与联合周期。已获守卫且两侧可用的周期可以解除错相。
- **SAI-ARCH-R2-F2 CLOSED（AgentDeck 提交范围）**。App 核验与调用、helper 失效事务使用同一个受管状态目录锁。持锁进程在间隙暂停时，竞争者有界返回忙碌。世代文件删建不会替换目录锁。旧 ACK 不恢复旧去重账本，提交结果不明后按数据库的实际世代对账。
- **SAI-ARCH-R2-F3 CLOSED**。Restore 以空目标为前提，保全既有凭据归档范围，世代镜像在目标重新建立。

A11 与 Widget 的 Codex 既有关联枚举、large 当前登录和组织信息边界一致。目录锁和文档检查均在隔离范围内通过。A5.7 规定 `add` 调用顺序，无法推出 macOS 的受理或横幅显示时刻。原生交错、真实横幅和 App Group 读写留到实现验收。

## Round 4 — 2026-11-18

### 📋 订阅账号架构复评

📊 综合评分：10/10

✅ 复评结论：PASS

### 🔴 必须修复

无。

### 🟡 其他改进

无。

### 🟢 已关闭

A5.7 以 Claude 失效事务提交与 AgentDeck 发起 `UNUserNotificationCenter.add(_:withCompletionHandler:)` 调用的顺序验收。A14 第 7 条把已有请求的受理回调和横幅时序列为平台观察。A13 的两方目录锁竞争继续验证有效提交先于失效，以及失效后持旧候选时零提交。

### 📝 小结

主控和独立复评核对共享目录锁、世代镜像、锁内发起调用及锁外回调与 ACK 的合同。用户决定只明确验收判据。签名隔离 App 的原生证据在任务 7 取得。 正式目标 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、architecture.md blob `88e4cab85c229116c53e410d0be872150425dad3`、指纹 `467aefd0287aaac0b09472e5232bd9649ff0937f40db8ed5f91ce1170a87b93b`。独立冷复评由 T3 确认为 Claude Sonnet 5.5，报告请求选项 `effort=low`。实际推理档位未独立观察。检查范围为已明确的通知验收决定及受影响文档。

完成门禁：VERIFIED

CEv1 精确状态通过 4/4。主控读回全部证据关系，确认无缺项、失效或待决影响。同态文档集合、链接、评审记录结构、空白和主题限定 diff 检查均退出 0。

任务检查点 `ad-subscription-account-identity-doc-arch-design`，目标状态 `subscription-account-identity:architecture.md:state:467aefd0287aaac0b09472e5232bd9649ff0937f40db8ed5f91ce1170a87b93b`。提交建议为本文件和相应评审记录单独提交。推送建议为随获准的 Claude issue 候选交付。

## Round 5 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 评审限用户批准的 A1.5／A13.2 单调截止修订。T3 配置为 `effort=high`。核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、architecture blob `78adaf545f790263a644cc6adaedd5c090eef2bf` 和 tasks blob `eff9d88cddce41e820dda7381949337a99d8109c`。取锁授权、原生忙碌错误、A5.7 守卫次序、零探测退场及 37／40 秒预算的核对通过。

- **SAI-ARCH-A1.5-F1 OPEN**。写 context 已耗尽时，迟到取锁后如果沿用该 context，`ROLLBACK` 会被拒绝。此时单独恢复 `busy_timeout` 成功也不能结束事务。原条款需指定与写截止及取消分离的有界清理 context，并在回滚或恢复任一失败时将物理连接从池中丢弃。A13.2 需分别验收这两类失败。

评审在隔离 WAL 库连续 20 次复现过期 context 回滚失败和另一个连接 `SQLITE_BUSY`。主控用同一工作树的现有驱动执行 `TestControllerExpiredRollbackAndDiscard`，核对了两个子用例。过期回滚保持写锁，独立有界清理和返回 `driver.ErrBadConn` 丢弃这两条路径均使另一连接可以立即获得写锁，后续池租约的 `busy_timeout` 都是原设定 731 ms。提交顺序、闸门语义和其他架构范围沿用既有评审。F1 修订待独立复评。

## Round 6 — 2026-11-19

复评结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 复评限写入清理修订，T3 配置为 `effort=high`。核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、architecture blob `4e2644261cf4968f26fc18137400d5ae82a2419d`、tasks blob `eff9d88cddce41e820dda7381949337a99d8109c`。该轮为契约复评，沿用 Round 5 的 WAL 探针。

- **SAI-ARCH-A1.5-F1 CLOSED**。A1.5 明确先回滚再恢复设置、使用与写入分离的有界清理 context，并在任一清理失败时丢弃物理连接。A13.2 要求分别注入两类失败，检查写锁释放及干净的池租约。主控的产品入口 overlay 定向测试 `TestQuotaWriteDiscardsConnectionOnCleanupFailure` 两个失败注入子用例通过。
- **SAI-ARCH-A1.5-F2 OPEN**。该候选给清理增加了独立的 1 s 截止，但准入和 37 s 算式未计入这段时间。最后一次修剪或闸门写入可在写截止用尽后继续清理，导致受控的 Go 周期预算延伸到约 38 s。必须为每个写步骤预留清理，并据此更新阶段、refresh 锁等待及末步回滚的验收。

主控按 37 s helper 与 40 s Swift 预算选择在同一次 Go 周期内为各写步骤预留 `W + C`。复核同一 Claude 分支最多三次写入及清理，另有一次序号写入和一次告警修剪。准备 3.1 s、探测 22 s、告警与输出 2.5 s，打开 store 和锁等待余量 9.4 s。修订待独立复评，实际实现按最终组合验收。

## Round 7 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

**SAI-ARCH-A1.5-F2 CLOSED**。独立 Claude Opus 5.5 复评限 A1.5／A13.2 的清理配额，T3 配置 `effort=high`。核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、architecture blob `4db37b3f81fe00ac264e7cbae63cb45565aac681`、tasks blob `eff9d88cddce41e820dda7381949337a99d8109c` 和 Round 6 评审记录 blob `3a9577ac27f9f668aeac44b39c76c06922864551`。

逐项按一次写入及清理共 2 s 复算。Codex 探测 9.5 s，Claude 探测 22 s，准备和输出分别为 3.1 s 和 2.5 s，剩余打开 store 和锁等待 9.4 s。A13.2 覆盖最后一次修剪回滚、完整清理耗时及合计 37 s 的受控路径。Round 6 的 F1 关闭继续适用。主控分别注入回滚和设置恢复失败，两个子用例通过。把丢弃连接的条件改为“两种清理同时失败”后，前者检出写锁滞留，后者检出池归还错误的 `busy_timeout`。事务实现与产品验收由 Task 4 的最终组合收口。

CEv1 文档门禁通过 4/4，目标 `subscription-account-identity:architecture.md:state:717b22bb049f4598bd166b1c8a415df4970678f0adf0d383450cc95c17059ad2`。主控仅还原 A1.5 和对应的 A13.2 段落，即重算出 Round 4 架构 blob `88e4cab85c229116c53e410d0be872150425dad3`。据此保留未受影响的契约范围、通知时序及独立评审证据，新增修订段的审查和文档检查记录。九个新增节点和十七条关系均读回核对一致，门禁无缺失、失效或待解决影响。

## Round 8 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Codex Astra 复核用户批准的 A5.7 独立守卫修订，T3 配置为 `reasoningEffort=ultra`。核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、architecture blob `e924c73eb742b77d11f6371633055328a4e2d5cb` 和 tasks blob `e329eadd42f59b4eb1a64984e88779692ee8b6df`。同一文件锁域、绑定身份、世代提交次序与 37／40 s 的预算保持。以下发现阻塞该文档候选。

- **SAI-ARCH-R8-F1 OPEN**。helper 在 `XATTR_CREATE` 成功而根目录尚未同步时退出，续作者仅凭读到绑定可能提前发布正式名。续作在 `linkat` 前须重新确认根目录同步成功，并以崩溃和同步失败验收正式名保持缺失。
- **SAI-ARCH-R8-F2 OPEN**。Swift 争用夹具仍持目录锁，须改用已绑定守卫的独立描述符验证 `G_app`。CLOEXEC 对照须让子进程完成 `exec` 并保持存活，持锁方只关闭本地描述符。
- **SAI-ARCH-R8-F3 OPEN**。绑定缺失、两名与镜像被同时清除时与首次初始化不可区分。列明正常运行的所有权边界及所需的本地文件系统语义。

主控已将三项修订写入相应条款及验收行，待独立复评。与任务分解对应的验收缺口单列于任务评审记录。

## Round 9 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

独立 Codex Astra 复评核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、architecture blob `8aa550e4bd233f4f7f2eca67f9b389a05729c1e2`、tasks blob `0875a84d5b154ce257ff814eea92088274953dae` 和上轮记录。T3 配置 `reasoningEffort=ultra`。复评限 A5.7 契约和受影响验收。

- **SAI-ARCH-R8-F1 CLOSED**。恢复发布前成功同步根目录，覆盖写绑定后同步前中断及恢复同步失败时正式名保持缺失。
- **SAI-ARCH-R8-F2 CLOSED**。Swift 争用、CJSI 保留连接以及实际 exec 的 CLOEXEC 使用三个独立对照。
- **SAI-ARCH-R8-F3 CLOSED**。信任边界明确全体元数据被外部并发移除无法与首次使用区分，受管恢复须独占并先退出旧持有者。列明本地文件系统能力。

首次守卫初始化与取锁共用 `G`，此前 37／40 s 预算及世代提交顺序验证结论适用。实现与实际验收归任务 3、任务 4 和 CJSI 组合。

CEv1 文档门禁 4/4 通过，目标 `subscription-account-identity:architecture.md:state:324bcc989e53137ecfa0d196906ea80b42de62183f53aa9483be757620c31542`。主控对比两稿，核对 A5.7 及相关预算、存储与验收修订。未受影响的契约范围与评审证据按明确影响记录沿用。守卫的次序与恢复条款使用本轮新证据。门禁无缺项、失效或待决影响。

## Round 10 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

独立 Codex Astra 按 `reasoningEffort=ultra` 核对 HEAD `424fe8a01ebad097a112323e5d3945918296443b`、live 父 blob `8aa550e4bd233f4f7f2eca67f9b389a05729c1e2`、草案 blob `43202f39a7b8feb09751629dc35e858c1da7fc75` 和补丁 SHA-256 `789bac6a085031349741b6932ffb59ae497d15ffe2a1e7d6936a43b7d3ea7d6f`。本轮对齐 A4 消费者表和已批准的菜单栏 UX §2／§5。主控按评审的精确补丁采纳后复核实际 blob。

菜单栏从套餐对象自身选择未报告和原值不可安全显示的本地文案。显式 null 时，适用的 Claude 额度行使用未报告文案并省略胶囊。缺键时使用旧套餐与原因字段的规则适用。Widget、wire 枚举、CLI 诊断的约束继续由既有契约和评审证据覆盖。

采纳后的架构状态 `subscription-account-identity:architecture.md:state:53710457e1ab4caff89595d675a1beda05d9dfabf68279fabaa86c6d9b49f8a7` 通过 CEv1 4/4。无缺项、失效或待决影响。任务 5 的运行时评审单独记录。
