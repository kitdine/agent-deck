---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — Tasks

来源为 `ad-bug-claude-quota-output-plan` 与 `ad-bug-codex-quota-plan-display`。两项共用 quota/store/wire 和原生消费者，版本成员及集成由 v0.7.5 契约维护。保留每项的验收和交付映射，文档、单项任务或单个 PR 的进度不关闭原 Bug。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar.md | [x] | [x] |
| ux/widget.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

分解输入为当前 [需求](requirements.md)、[架构](architecture.md)、[最终菜单栏](ux/menubar.md)和[最终 Widget](ux/widget.md)。以各自正式记录中的有效候选为准。2026-11-19 用户批准 A5.7 改用独立守卫文件。架构、受影响的任务分解及任务 3 的守卫实现重新开放。此前投递顺序、旧 ACK 和世代规则的适用证据仍由正式记录承接。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `quota-storage-compatibility` | [x] | [x] |
| 2. `identity-adapters` | [ ] | [ ] |
| 3. `quota-delivery-guard` | [x] | [x] |
| 4. `quota-refresh-publication` | [x] | [x] |
| 5. `menubar-subscription-identity` | [x] | [x] |
| 6. `widget-subscription-identity` | [x] | [x] |
| 7. `subscription-identity-acceptance` | [x] | [x] |

| Anchor | 依赖 | 验证 |
| --- | --- | --- |
| `quota-storage-compatibility` | 分解评审与门禁、基线和迁移分配检查点；跨层实现先过接口冻结 | L3 |
| `identity-adapters` | 分解评审与门禁、基线检查点；跨层实现先过接口冻结 | L3 |
| `quota-delivery-guard` | `quota-storage-compatibility` | L3 |
| `quota-refresh-publication` | 存储、adapter、投递守卫 | L3 |
| `menubar-subscription-identity` | `quota-refresh-publication` | L2，原生布局与可访问性 |
| `widget-subscription-identity` | `quota-refresh-publication` | L3 |
| `subscription-identity-acceptance` | 任务 1–6 的适用独立评审和门禁证据可用，GUI PATH 修复在同一验收构建内；原生决定仅限任务 7 收口 | L3 |

## 阶段与工作边界

- 工作区 `agent-deck.subscription-account-identity`，分支 `feature/subscription-account-identity`，主题基线 `4ac56db32e13c4e925b648c620ba76fc588a7781`。记录此次对照的目标 main `a224b0b824d2cad451d3bd83153832d65506f0fe`。该目标包含 PR #74 的 GUI Claude 发现路径与 PR #75 的 `clientDisplayLabel`，core schema 为 35。主控在开发开始前依架构要求纳入实际目标 main，确认这些修复仍在，并绑定新的候选；此检查点本身不授予 Git 操作权限。
- 主控在存储任务开工前核对目标 main 与全部在途批准迁移，尤其是 v0.7.5 的 Codex JSONL 会话身份 core 迁移候选，及实际纳入目标的其他 store 改动。记录各候选 SHA/指纹、core/session/解析器所有权、实际编号和应用顺序。分类性能已沿无迁移路径进入此次对照 main；后续若出现需迁移的方案，先按版本契约重新分诊。主题使用符号 `N` 表示主控分配的 **core** 迁移，独立 session/parser 版本按各自所有者的契约。集成前在最终目标上复核编号、触发器和兼容读者；若顺序变化，按实际受影响的跨版本升级路径补验。
- 主控先排定共享文件写者。存储任务先交接 schema 调整、`quota.Store` 接口及共享 Swift schema 消费者；守卫任务随后独占告警、`cmd/agentdeck/quota.go` 的告警编排和 Swift helper/通知代码；刷新发布任务接手 quota 命令、协调器、wire 及共用测试。交接记录已完成的符号、契约、候选指纹和待验证项；确认旧写者及其子操作已停再移交。菜单栏和 Widget 仅在文件及测试无重叠时并行，Widget 如需修改 `publishWidgetSnapshot`，从刷新发布任务交接 `EmbeddedHelperRunner.swift` 后写入。主控对齐 v0.7.5 serving-health/self-scan、selection 与 quota 对 store/serving 的重叠，以及 token mix、模型标签、GUI PATH 对 Swift 文件及共享测试的重叠；保留已通过的新路径。原型保持设计输入身份。
- D1 限 large Widget 显示脱敏邮箱，任何 Widget 均无组织名。D2 在已知 Claude 变化之后、重新确认发布之前丢弃状态栏捕获。D3 登录上下文失效只适用于 Claude。C0–C12 按架构 A0 逐条承接；CLI 文本兼容，WireVersion 1 增量，告警无账号。读取、同意与提醒权限沿既有入口，禁止额外认证操作。真实 CLI 采样逐次单独授权，常规测试只使用合成或获准脱敏夹具。
- 主控拥有文档、正式评审、Beads、CEv1、集成与 Git。评审和门禁通过后按已授权的 issue 边界分别 commit、push origin 和创建 PR，push/PR Actions 均通过后请求 Codex review。两个 issue 的提交归属与 PR 顺序见下节。交付上限不含 merge、安装、release 或 deploy。GUI PATH 为单独 Lane A，主题复用主线发现规则。

## 来源问题、提交归属与交付顺序

以下交付范围按两个来源 issue 的 Beads 验收和 v0.7.5 契约制定。Claude 指 `ad-bug-claude-quota-output-plan`，Codex 指 `ad-bug-codex-quota-plan-display`。表中数字为上表的 anchor 序号。每个共享实现和共享测试由对应 anchor 的唯一写者负责，归 Claude PR 一次。Codex PR 只提交 Codex 专属增量及其独立验收，依赖已验证的 Claude PR 基线。共享路径的承载方只决定改动归属，Codex Issue 仍需证明实际使用这些共用能力。

| Anchor | Claude PR 验收与提交归属（含共享） | Codex PR 验收与提交归属 |
| --- | --- | --- |
| 1 `quota-storage-compatibility` | 独占迁移、通用身份状态、Claude 世代/闸门接口、三表换代、备份/Restore、共享 schema 兼容。 | 复用同一迁移。验收升级和恢复中原 Codex 隔离键及旧库兼容；若需 Codex 关联的特有存取实现，提交独立增量并复用通用表/API。 |
| 2 `identity-adapters` | 独占公用脱敏和套餐投影、Claude auth status、HMAC 与 prose 分类；旧 Codex 探测仍工作。 | 同一个 app-server 的 `account/read`、单 reader 和共同键检查，Codex 原字段筛查及 `prolite` 映射；有关联、无证据、冲突、旧缓存夹具。 |
| 3 `quota-delivery-guard` | 独占 Go/Swift 投递协议、旧提醒和 ACK 隔离。 | 复用通用通知信封和现有 Codex 交付条件；以基线与叠加候选证明 Codex 不走 Claude 世代失效，无重复改动。 |
| 4 `quota-refresh-publication` | 独占周期、捕获、一致读取、通用 wire 和协调器；证明 Claude 身份与 usage 可分别成功、失败和恢复。 | 仅接入 Codex 双请求和有条件的 quota 关联提交/投影；证明换账号/工作区、退出、身份失败和旧 quota 不串线，旧计划值仍可诊断。 |
| 5 `menubar-subscription-identity` | 独占共享行、布局、时刻、AX 和 Claude 独立套餐/身份文案。 | 复用布局；仅提交 Codex 特有套餐与关联提示接线/断言，证明脱敏邮箱、可用工作区、来源/时间及切换。 |
| 6 `widget-subscription-identity` | 独占共享 App Group 白名单、三态、到期 entry、各尺寸过滤和 Claude 未确认呈现。 | 仅提交 Codex 的 large 额度套餐/关联枚举接线与断言，证明 Pro 100 与菜单栏一致且 small/medium 无账号、任何尺寸无工作区。 |
| 7 `subscription-identity-acceptance` | 独占共享夹具基础和联合验收入口，分别记录原 parse_failed 界定、同 App 恢复、未确认归属、Claude 状态转换、通知实测。 | 记录同账号官方套餐证据与原始 planType 对照、源码/App/helper 身份、切换矩阵和跨表面呈现，叠加候选重跑受影响共享用例。 |

Claude origin 须覆盖有效两行、缺行、格式/时区异常、额外模型行和旧失败恢复，及旧 CLI、缺字段、API-key、退出、账号/组织变化。保留旧安装 helper 的已知构建和坏 stdout/调用环境的未知范围。实际套餐、脱敏邮箱与组织、失败的独立原因和时间通过获准表面验证。Codex origin 以已获准的同账号证据对照 `prolite` 与 ChatGPT Pro 100，核对 89% 已用对应 11% 剩余及重置观察，映射仅作用于文案。双侧隐私均按任务 2、4、6 及联合证据证明。

主控在每次交接登记源 issue、anchor、共享/专属文件或符号和本次候选。先让 Claude 承载候选独立通过，冻结并记录实际 head；在该 head 上构建 Codex 叠加候选并验证增量与交互，Claude head 自身必须通过构建、旧 Codex 回归及自身验收。由主控按 issue 分别提交、推送并创建 PR，先 Claude 再 Codex；Claude PR 面向获准目标，Codex PR 可对准 Claude 分支以只显示自身增量。每个实际 PR head 各自等适用 push/PR Actions 通过后请求 Codex review。依赖 head 改动或未来重新定基时，主控核对影响并按新 head 复核和补验。任一 PR 的创建、评审或通过均不授予 merge；改目标、合并和后续发布仍以实际授权为准。

## 实现分解

### 任务 1、2、4 的接口冻结

主控在有界的实现交接记录中冻结 Go 接口，记录目标与候选指纹、文件/符号、字段类型及缺失语义、隐私等级、事务边界、失败分类，以及任务 1、2、4 共用的最小编译和表驱动夹具。任务 1 拥有并负责定稿拟新增的 `internal/quota/identity_contract.go` 中的跨层值类型和 store 接口，任务 2 负责核对 adapter 输入、输出形状和禁止字段，任务 4 负责核对调度和投影读取约定；主控确认后才准接写。冻结至少涵盖身份状态/失败、只含允许数据的显示值、关联比较结论与 Claude 私有比较材料、`failure_detail`、周期与世代期望、事务结果与旧库缺省。原始 Codex 共同键和账号 ID 不纳入身份接口，原有额度隔离输入沿 C2/C8。

冻结前，任务 1 可盘点 schema 和旧读者、构造升级/旧写者/备份夹具；任务 2 可冻结 vendor schema/样本、验证独立进程和纯解析夹具；任务 4 可盘点发布入口并准备与新增类型无关的时序测试。跨层 DTO、持久接口、接线和依赖这些形状的测试等冻结后进行。若形状需改，原接口写者提交拟变更，主控重核存储、adapter、调度、wire 和受影响证据，再交接；共享文件仍只由一个写者持有。

| 架构逻辑 | 唯一 Go 所有者及路径 | 交接 |
| --- | --- | --- |
| A4 套餐选择、冲突、友好名与安全原值规则 | 任务 2，拟新增 `internal/quota/plan_display.go`。同一作者实现通用规则及按 issue 拆交的 Codex 映射。 | 任务 4 只调用并编码结果；任务 5/6 按 wire 三态呈现。 |
| A5.2 身份状态、新鲜度、重新验证地板的判定和有效期 | 任务 4，拟新增 `internal/desktop/subscription_identity.go`，由 `BuildSubscription` 一致读后调用。 | 任务 1 只维护触发器并提供序号、时间和原始状态值；原生层用有效期决定呈现。 |
| A5.3 Claude 转换表 | 任务 1，拟新增 `internal/quota/identity_transition.go`，在 `internal/quota/store.go` 的身份事务内用同一纯判定。 | 任务 3 接收事务内失效结论，任务 4 只编排和消费已提交结果。 |
| A5.6 新 usage 可见、同周期证据核验及提交后闸门发布条件 | 任务 1，拟新增 `internal/quota/identity_reconfirmation.go`，供 `CommitClaudeUsage` 与后续闸门事务调用。 | 任务 4 负责传入周期证据及在提交结束后获取启动标识和单调时刻；读取结果只看存储快照。 |
| A6.5 HMAC 和 key id 生成 | 任务 2，拟新增 `internal/quota/identity_secret.go`，连同 A6.6 私有密钥生命周期。 | 冻结接口仅将比较材料交给受守卫的 store 事务，无可导出或 wire 路径。 |
| A9 prose 失败分类与脱敏错误 | 任务 2，`internal/quota/claude.go` 的 `ParseClaudeProse` 与类型化错误。 | 任务 1 负责受守卫的存取和成功清除，任务 4 转交类型并投影，任务 7 收口历史限制。 |

### 1. `quota-storage-compatibility`

- **所有权。** `internal/store/migrations.go`、`internal/store/store.go` 的 `CurrentSchemaVersion` 及实际 quota/serving trigger 安装点，`internal/quota/store.go` 的 `Store`、`Record`、`Windows`、`Envelope` 与新守卫事务 API，`internal/backup/backup.go` 的备份暂存、`Restore`、`validateCoreSnapshot`，`internal/desktop/serving_schema.go` 的 `compatibleServingCoreSchema`。同步这些文件的对应测试、CLI schema 诊断契约、`apps/macos/AgentDeckShared/DesktopWire.swift` 中的 core schema 兼容判断、`apps/macos/AgentDeckVerification/main.swift`、原生 schema 测试和 `desktop/fixtures/v1/` 四份当前 schema 金标。旧 producer 夹具继续用于兼容断言。
- **交付。** 先打通 schema 版本化旧读路径，再在同一候选切换写入 SQL 与迁移。迁移 `N` 事务内建立 cycle 和 projection revision，复制三张旧 quota 表到 `_v2`、删除旧表，建立单行/每客户端身份状态、serving/projection revision 及重新验证触发器。保留迁移 31 的原表清单，旧 alert 表形状检查仅用于前 `N`，旧表名永久停用。兼容 v27 的 backoff 缺列、v27–29 的失败时刻未知，以及 30 到 `N−1` 的缺省；把未知顺序传给消费方。旧连接与旧二进制分别由表换代和 SchemaAhead 拒绝。提供事务内序号、基线、世代、闸门、窗口/告警账本清理与一致读接口。失效事务的入口接收守卫能力，投递守卫内的文件协调由任务 3 提供。迁移、现行读写者、备份/Restore 和 schema 检查一起落地。
- **备份与恢复。** 按 A8 在私有快照上 secure-delete、checkpoint，再以 VACUUM INTO 的压实副本归档和计算摘要，核验完整性、空身份表、零 freelist 和无 WAL。Restore 保留空目标和既有凭据条目范围；先读档案实际 schema/表形，再迁移与清除源身份、额度/提醒和周期状态。含身份 schema 或表的外部档案即使表空也须无条件压实；前 `N` 旧档依契约清逻辑额度数据。目标不继承秘密或投递镜像；保留失败回滚。一般 export 继续排除身份。
- **证据。** `scripts/run-go-test.sh ./internal/store ./internal/quota ./internal/desktop ./internal/backup ./cmd/agentdeck`、影响 schema 的 Swift decode/verifier 与金标、相关 race、vet、隐私检查。提供全新库、35 含 quota 行升级、当前目标和组合迁移顺序、v27/28/29/30/35 只读矩阵、触发器实际增量和已预热 WAL 旧写者的失败对照。备份逐项扫描归档 core 字节，证明新旧哨兵、自由页和页内残留均按 A8 清除；记录旧档、外部档、非空目标、失败回滚与一般导出断言。旧失败顺序对视图与提醒的断言由任务 3、4 消费同一夹具补齐。

### 2. `identity-adapters`

- **所有权。** `internal/quota/codex.go` 的 `ProbeCodex`、JSON-RPC reader 和 rateLimits 解码，`internal/quota/claude.go` 的 `ProbeClaudeProse`、`ParseClaudeProse`，新增身份 adapter/脱敏、套餐投影及 Claude 关联秘密实现，及 `internal/quota/testdata/` 与对应测试。adapter 通过下述冻结接口交接分类结果、脱敏显示字段、套餐来源、进程内关联结论和类型化失败，任务 4 接线。沿 main 的 `internal/quota/claude_exec.go` 中 `claudeCommandPath` 查找一次，auth 与 usage 共用路径、环境和工作目录；发现规则归原 GUI PATH 修复。
- **交付。** 按 A13.1 冻结隔离 Codex schema、已获准 Claude 2.1.325 脱敏样本及版本/时间/命令/退出码或 schema 摘要。Codex 同握手、单 reader 按 ID 分派，只发送到期请求，丢弃迟到身份结果，核对 refresh 参数并关闭令牌刷新；实现子截止、进程退出与分侧失败矩阵。共同键仅在临时进程内比对，未证实即未确认。Claude 按真实字段、登录类型和组织存在性分类，未证实形状保守降级。保留严格两行 prose 语法与已有窗口，细分诊断并清理各 adapter 的错误正文。按 A4/A6 在写入前完成邮箱脱敏、显示名清洗、禁止值筛查、套餐展示与安全旧值兼容；原字段、原始 stdout/stderr、认证路径和外部错误正文不得穿越边界。Claude 关联值用本机 HMAC；受管根目录下原子创建私有密钥，校验打开的同一描述符，区分缺失重建与不安全拒绝。只从刷新门禁通过时加载；本任务交付可测试接口，不开启运行时探测。
- **证据。** `scripts/run-go-test.sh ./internal/quota`、相关 `-race`、`make check-privacy` 与扩展无凭据/无认证网络断言。提供 A13.2 的 grapheme、危险值与名称矩阵、有关联和无关联假 schema、计划来源冲突、异常进程/取消/WaitDelay、迟到响应不削减 usage 预算、四类 prose 结果、密钥并发创建/崩溃/权限与描述符替换，以及错误与输出哨兵。真实样本未证实的共同键和官方展示名字段留在保守分支。

### 3. `quota-delivery-guard`

- **转换前置交接。** 任务 1 在冻结接口上交付 A5.3 转换表和 `ApplyClaudeIdentity` 事务入口，明确 no-op/重放、仅尝试、成功不失效、需要失效的结果及期望/下一世代。交接测试覆盖同事务的基线与世代、组织变化、秘密替换、退出及重复退出。任务 3 持投递守卫进入事务，通过失效回调取得下一世代，在需要失效时先同步镜像再提交。回调错误回滚，提交不明保留镜像，下轮用已提交世代对账。事务结果确定后释放投递守卫。任务 3 把守卫入口、失败退场及跨进程证据交给任务 4，任务 4 在身份提交成功后读取世代并继续 usage。

- **所有权。** `internal/quota/alerts.go` 的 `DueAlerts`、`ValidateAlertIDs` 和 ACK 写入，Go 守卫初始化、绑定校验及世代镜像实现，`cmd/agentdeck/quota.go` 的提醒构造、refresh 返回信封和 ACK 路由。Swift 为 `apps/macos/AgentDeckApp/QuotaAlertNotifier.swift`，及 `AgentDeckShared/EmbeddedHelperRunner.swift` 的提醒传输、`DesktopRefreshCoordinator.performQuotaOperation` 和对应测试。主控与任务 4 串行交接刷新失败状态及预算测试，Swift 候选世代和目录身份字段沿用既有合同。
- **交付。** helper 在现有 ownership session 和 refresh 锁内按 A5.7 初始化或验证 `quota-delivery.lock`。持投递守卫后才能进入 Claude 失效事务。镜像先同步再提交，不明确的结果以下轮已提交世代对账。首次初始化和锁等待共用 `G`。守卫不可用时按既有 warning 报告部分失败，Codex 路径继续。提醒用 `qa2.` 携带同一读事务的世代，旧 `qa1.` 按世代 0 处理，旧世代 ACK 丢弃。App 顺序保持 refresh → deliver → ack → quota。核对物理路径、`state_root_id`、守卫绑定、现有权限和批内去重，在守卫内同步调用 `UNUserNotificationCenter.add(_:withCompletionHandler:)`，解锁后等待有界回调并仅 ACK 按时成功项。Claude 缺证或校验不通过时跳过，Codex 沿用原提交条件。
- **证据。** `scripts/run-go-test.sh ./internal/quota ./internal/store ./cmd/agentdeck`、相关 race/vet 和 `make test-macos-app` 中通知、helper、coordinator 用例。A13.2 的 FIFO/SIGSTOP 核对绑定守卫临界区，覆盖 SIGKILL 释放、实际 exec 的 CLOEXEC、镜像替换、文件领先及下轮对账、旧 ACK、无效绑定以及 SQLite 和守卫截止。在绑定文件上分别验证 helper 的 `G` 和 App 的 `G_app` 争用。Swift 注入 poster 核对持锁调用与锁外回调等待，覆盖无权限、无回调、迟到、报错、重复及缺证候选。交接同一 helper 信封及 notifier 的混合用例。Claude 守卫不可用，Codex 回调按时成功，断言既有 warning、恰一个 Codex `add` 和 ACK，以及零 Claude `add` 和 ACK。另记录只读先开的 v36 保留连接仍阻挡根排空的组合验收。任务 4 以真实隔离 helper 重放调度交错，任务 7 负责实际原生通知提交和横幅观察。

### 4. `quota-refresh-publication`

- **所有权。** `cmd/agentdeck/quota.go` 的 `runDesktopQuotaRefresh`、load/render quota 入口，`cmd/agentdeck/quota_capture.go` 的 `runQuotaCapture`，`internal/quota/scheduler.go`、`statusline.go` 及 clock/boot 接线，`internal/desktop/desktop.go` 的 `RefreshQuota`、`subscription.go` 的 `BuildSubscription` 和 `serving_cache.go` 的 `ReadServingSnapshot`、`ReadOrBootstrapServingSnapshot`。Swift 拥有 `DesktopWire.swift` 的订阅增量解码及 `EmbeddedHelperRunner.swift` 中 `installServingPublication`、`releasePublications`、`publishStandaloneQuota`、完整刷新安装路径与对应测试。以任务 1–3 已交接的迁移、adapter 和投递 API 为基础。状态读写需要追加跨 owner 能力时先由主控登记具体符号后串行移交。
- **刷新与捕获。** 一次 `quota.Allowed` 同时管两侧，默认关闭和 official/观察 provider 校验保持。开启 37 s helper 周期，Swift 总预算 40 s；按 A1.5 分配锁等待、专用连接和写预算，资源/截止不足时按退场规则保持零额外尝试。先取 refresh 锁及周期号再探测，客户端并行、客户端内顺序固定。身份与 usage 的失败、时间与退避独立，手动 usage 保留 C9；Claude 阻塞时按 A1.2 形成不越退避的联合机会。Claude 先在任务 3 守卫下完成身份转换，再取世代、运行并守卫写入 usage；身份提交失败时该周期不启动 Claude usage。Codex 在同一提交中写两侧结果、保持原 accountDigest 隔离和同时提交关联条件、世代恒 0。A5 的确认事务与提交后单调时刻发布闸门分开；崩溃、重启、同刻、回拨均按契约。捕获在读 stdin 前记入口单调时刻，原命令先执行、stdout 字节透传、同意及注销行为保持，末尾单事务比较 boot/时刻/世代闸门。
- **一致读与发布。** 一个只读事务生成订阅段和 `{core_epoch, revision, boot, mono}`；旧版本失败顺序未知时保守抑制。在 serving 直读和 bootstrap/cache 恢复路径重新投影，失败时订阅不可用，其余数据沿旧规则。新 JSON 字段总有键，旧原因闭集与 CLI 文本金标保持；Swift 宽容解码新枚举，保留 absent/null/value，只有 absent 回退旧套餐。协调器把 held/standalone/完整刷新收敛到同一单调安装规则，过期订阅段不覆盖已安装段，其他段仍可安装。安装通过后沿现有 Widget 发布路径交接，任务 6 负责安全投影及重载语义。
- **证据。** `scripts/run-go-test.sh ./internal/quota ./internal/desktop ./internal/store ./cmd/agentdeck`、相关 race/vet、`make check-desktop-refresh-integration`、Swift wire/coordinator 套件。按 A13.2 提供零进程门禁、双侧独立退避/错相与清空情况、并发/取消/繁忙时序、37 s 总预算、旧 in-flight 成功与失败失效、启动标识/慢 stdin/FIFO/两次变更/两事务间中断、可展示新 usage 与恢复捕获的不同条件。覆盖快照读取中插入失效、旧 serving 快照、held 后释放、不同 core epoch、等 revision 比 mono、旧 producer、CLI golden 与存储/JSON/错误/perf 哨兵。用同一隔离 helper 按 A13.3 分别执行刷新和 JSON 读，证明失败细分、恢复及 DB 状态。

### 5. `menubar-subscription-identity`

- **所有权。** `apps/macos/AgentDeckApp/MenuBarPanelViews.swift` 的 `QuotaPanelView`、`headerCaption` 和额度原因呈现，必要的 `MenuBarViewModel.swift`、App copy 和 `Localizable.xcstrings`，对应 `AgentDeckAppTests`。复用 `DesktopPresentationModelV1.clientDisplayLabel`。共享文案或语音格式确需供 Widget 使用时先由主控指定单一写者和接口。
- **交付与证据。** 按最终 UX 的 M1–M11/Q1–Q7 实现中英文显示、帮助与 AX 名称。独立当前登录、上次读取、额度和尝试/观测时间；当前资格渲染时再核对 `current_until`，确认提示需要仍然当前。覆盖无邮箱退出/API key、套餐冲突、失效与新 usage 后未确认归属、老格式回退、门禁取消显示。Claude 主文案保持“账号未确认”，旧失败与身份不串线。运行 affected App suite，加原生菜单栏窄宽 280 pt 与正常宽、长套餐/组织与邮箱截断、Pro 100 单行、时间和来源不遮挡、深浅色和 VoiceOver/AX 顺序验证；原生实测纳入任务 7，按本任务的场景回链证据。实现评审使用本任务可独立运行的 App suite 与渲染断言。

### 6. `widget-subscription-identity`

- **所有权。** `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift` 的 `AppGroupDesktopSnapshotV1`、`WidgetSemanticProjection.swift`、`WidgetSnapshotPublisher.swift`、已交接的 `publishWidgetSnapshot`，以及 `AgentDeckWidget/WidgetDomain.swift`、`WidgetViews.swift`、`WidgetCopy.swift`、`WidgetTimeline.swift` 的 `QuotaTimelineProvider`、本地化及 shared/widget 测试。
- **交付。** App Group 使用 A11 允许列表和三态编码。保留既有原子快照与安全读边界；实际采用新订阅段后重写，语义变化触发受影响 Widget。只有 large 消费身份，按 entry.date 与有效 `current_until` 显示脱敏邮箱；只接受 Codex 精确 confirmed 抑制未确认提示，Claude 始终未确认。若到期早于刷新，安排到期 entry，不延长资格。所有尺寸按原因逐客户端丢弃失效窗口，保留另一端；large 有可用块时页脚仍是其观测时间，两端无块且已知失效时显示原因。无组织、历史身份或完整登录诊断被投影。
- **证据。** `make test-macos-app` 中 App Group 及 Widget suite，`make check-widget-sandbox` 和批准字段哨兵，验证旧/缺键/null/无效值往返、原子 I/O、订阅变更重写及重载、到期前/等于/晚于刷新边界、各端失效过滤、三尺寸中英文长值截断与 AX。实际 App 写入、真实 WidgetKit 读取和到期 entry 的可见结果纳入任务 7，并回链本任务。单测和原型保留各自覆盖范围。

### 7. `subscription-identity-acceptance`

- **所有权。** 主控负责集成与验收。运行入口为 `cmd/agentdeck/acceptance_test.go` 或现有 CLI quota 测试、A13.3 隔离 helper 序列、`scripts/run-macos-acceptance-app.sh` 和现有原生验收入口。若缺少隔离启动能力，先证明现有入口的限制，明确候选专用夹具和授权；需要复用的测试/验收代码限定在对应 suite，经主控派给一个写者。产品缺陷回到所属 anchor。
- **候选与环境。** 对同一构建记录目标/候选 commit、工作树状态、相关 blob 指纹、helper/App/Widget 二进制摘要、macOS/工具链、签名与 bundle/app group 标识，以及 PID/窗口、隔离 HOME、stateRoot、假 CLI 路径与夹具摘要。工具先绑定实际 App 与窗口再操作。CLI、原生 App/菜单栏、通知、Widget 分别绑定同一候选的端点；执行在临时账户/隔离标识与受控配置下，避开真实账号、已安装 App 及其 App Group。通知用 team-signed 非 /tmp 的隔离 Debug App；现有脚本移除 Widget，因此 Widget 另用带匹配隔离 entitlement 的 App/extension 构建和验收账户。必要注册/测试通知权限仅限明确批准的隔离环境。若签名、注册、权限或安全隔离条件缺失，保留原生 gate 未验证及确切障碍。
- **实测。** 在同一环境验证独立 GUI PATH 修复，复用原生受信查找路径且无手工软链或 shell rc。以假客户端驱动 A13.3 五步诊断序列，分开读取 `desktop quota-refresh --manual` 门禁和 `--format json quota` 的窗口、独立时间与错误，核对 DB。记录历史失败的已知时间/构建/调用上下文与未知部分，结论为“历史根因未复现，未作猜测性 parser 修改”。两端验证套餐映射与原值、门禁零探测、备份/Restore 去身份、已知与未知切换、旧状态栏/ACK、重新确认后恢复，与中英文菜单栏及三尺寸 Widget 的一致性。观察原生 AX、App Group 更新、真实 WidgetKit 读取和到期前后渲染；分别记录 entry 时间与宿主实际刷新时刻。单独记录注入的通知竞争、真实 App 提交调用、系统完成回调与横幅观察。
- **旧提醒验收。** 按 2026-11-18 用户确认的 A5.7 提交边界，失效事务提交后，旧世代 `add` 调用计数必须为零。原生验收分别测失效前已提交并受理和失效提交后持旧候选两种场景，核对失效提交与 AgentDeck 发起调用的顺序。系统受理回调和横幅时间单独记录为平台观察。
- **证据。** 复用各任务仍有效的候选证据，最终相关状态执行 `scripts/run-go-test.sh ./...`、相关 race/vet、Go/App 构建、`make test-macos-app`、`make check-privacy`、`make check-widget-sandbox`、`make check-desktop-refresh-integration` 与隔离 CLI、实际 App/Widget/通知验收。按源码、测试、依赖、配置、工具链和环境变化补验组合迁移与跨主题交互。分别附 Codex 和 Claude 的验收对照，共享证据可复用。保留 D2 的进程入口前不可观察区间、重启后直到再次发布的延迟、重新确认后仍在运行的旧 Claude 会话无法辨识等 A14 限制。

## 验收追踪与证据交接

以下 P1–P6 是本文件的证据索引，分别对应任务 1–6。N1–N4 是任务 7 的候选实测步骤。索引指向下列具体命令和断言组，实际执行时登记每条结果与正式 evidence ID。

### 任务 7 复用及补验清单

| 证据索引与产出任务 | 命令及必须交接的结果 | 任务 7 的复用及补验 |
| --- | --- | --- |
| P1 `quota-storage-compatibility` | `scripts/run-go-test.sh ./internal/store ./internal/quota ./internal/desktop ./internal/backup ./cmd/agentdeck`、对应 Swift schema 测试和 `AgentDeckFoundationVerifier`；记录全新/升级/只读、WAL 旧写者、revision/重新验证触发器、归档字节与 Restore 回滚结果。 | 复用版本化只读、旧连接失败、归档旧/新哨兵结果；按实际主线和叠加候选的迁移排序补跑升级、schema 消费者及隔离 Restore。 |
| P2 `identity-adapters` | `scripts/run-go-test.sh ./internal/quota`、该包 race、`make check-privacy`；记录冻结夹具摘要、双请求/截止/退出矩阵、Claude 严格解析/恢复输入、套餐分支与隐私/密钥断言结果。 | 复用纯规则和假进程矩阵；N1 验证同构建的发现、进程、DB 和对外投影，N2/N3 验证实际用户可见结果。 |
| P3 `quota-delivery-guard` | `scripts/run-go-test.sh ./internal/quota ./internal/store ./cmd/agentdeck`、对应 race/vet、`make test-macos-app` 中 notifier/helper/coordinator 用例。记录旧 ACK、FIFO/SIGSTOP、镜像领先以及绑定文件的 `G`／`G_app` 争用。混合用例验证现有 warning、单条 Codex 提交和 ACK、零 Claude 提交和 ACK。单独记录 v36 保留连接排空结果。 | 复用已控交错及成功项 ACK 结果。N4 补真实 App 的提交顺序，另记平台受理与横幅观察。最终 helper 上的跨进程调用路径由 P4 和 N1 绑定。 |
| P4 `quota-refresh-publication` | `scripts/run-go-test.sh ./internal/quota ./internal/desktop ./internal/store ./cmd/agentdeck`、对应 race/vet、`make check-desktop-refresh-integration`、`make test-macos-app` 中 wire/coordinator 用例及 A13.3 隔离 helper；记录门禁计数、守卫交错/预算、捕获、读事务、旧缓存、三态、两份 CLI 输出及 held publication 结果。 | 复用确定性顺序断言、CLI 文本金标及独立字段证据。N1 用验收 App 内实际嵌入的 helper 重放 A13.3，补同环境 GUI 路径，N2/N3 核对发布结果。 |
| P5 `menubar-subscription-identity` | `make test-macos-app` 中 App suite，记录 M1–M11/Q1–Q7、两种语言、窄宽和 AX 的自动化场景结果。 | 复用 Swift 渲染与文本断言；N2 补实际窗口和 VoiceOver。 |
| P6 `widget-subscription-identity` | `make test-macos-app` 中 App Group 和 Widget suite、`make check-widget-sandbox`；记录三态、允许字段、原子写、重载、三尺寸和到期 entry 断言结果。 | 复用编码/渲染/时间规则；N3 补实际共享容器、WidgetKit 读和到期显示。 |

- **N1，隔离 CLI 与实际 App 的同环境探测。** 绑定候选 App 及其内嵌 helper 身份，正常 GUI 启动，以隔离的受信 CLI 路径核对一次 auth status 和一次 usage 的调用上下文。用同一二进制依次运行 A13.3 的 `--format json desktop quota-refresh --manual` 与 `--format json quota`，对齐门禁、五步诊断、窗口、独立时间和 DB 行；补测门禁关闭、切换和隐私。两份 issue 候选分别保留日志摘要；Codex 候选另对照已批准的同账号官方套餐证据、原值、旧缓存和无共同键形状。
- **N2，真实菜单栏。** 绑定隔离进程与窗口，覆盖 280 pt/正常宽、中英文、长值、当前/上次读取、失效/恢复、来源/时刻及 VoiceOver。核对关闭门禁后隐藏及历史状态的归属措辞。
- **N3，真实 App Group 与 WidgetKit。** 核对隔离 App 写出的允许字段和真实 extension 的读取；切换场景在 small/medium/large 和两种语言验证。记录到期 entry 时间及平台实际呈现，核对两端有/无块的页脚和无组织。
- **N4，真实通知。** 先复用 P3 的确定性交错。在明确授权的隔离签名 App 核对失效前的提交，并确认失效事务提交后持有旧候选时发起的旧世代调用为零。分别记录事务提交、调用发起、受理回调和横幅观察的时序。

P1–P6 每项由对应实现者交接自己的确切 HEAD、限定文件 blob/候选内容指纹、依赖和迁移顺序、配置/工具链/环境、命令与参数、退出状态、断言及跳过项。主控为每个 anchor 在同一内容状态绑定独立评审正式结论及其记录，并登记或查询相应 CEv1 WorkUnit 的 criterion、evidence ID、目标状态和 gate 结果。任务 7 由主控以同一方式登记 P 项引用、N 项及最终整合命令的结果，再绑定集成的独立评审和任务/主题所需门禁。Claude head 和 Codex 叠加 head 各自登记。当前分解只规定将来需记录的结果。

复用时由主控对照每条原命令/结果与实际候选、CI 和原生环境，确认其判定条件和依赖仍适用，并记录适用性判断及正式证据 ID。迁移编号或顺序、ABI/wire、scope 输入、工具链、fixture、App 签名/路径、系统通知或 Widget 运行环境变更时，针对受影响的 P 或 N 项补验；未变化证据复用。独立评审/CEv1 均按各自内容范围处理，旧候选 PASS 或跳过用例不能直接满足新候选门禁。

### 需求的 13 项验收

| 需求矩阵原顺序 | Anchor 证据与联合验收 |
| --- | --- |
| 1 关闭、非 official、未选择，含手动及渲染 | P4 断言零进程、RPC 和尝试写入，P5/P6 验证隐藏；N1–N3 复核真实入口。 |
| 2 自动/手动、单侧连续失败 | P2 假进程中断和分侧截止，P4 自动/手动间隔、独立退避、取消与用量预算；N1 绑定同环境。 |
| 3 `prolite`、`pro/pro`、未知/缺失/冲突 | P2 套餐矩阵与不参与价格/权限断言，P4 原值兼容，P5/P6 三态呈现；N1–N3 对照官方样本、来源和品牌名称。 |
| 4 Codex 缺共同键或不一致 | P2 假 schema/P4 同周期与旧隔离键断言，P5/P6 关联降级；N1–N3 核对独立呈现。 |
| 5 Claude 两侧各自成功或失败 | P4 验证时间、来源、错误分账和归属恒 false，P5 独立区域；N1/N2 对照。 |
| 6 已知账号/组织变化、退出/API-key | P1 转换表、P3 旧提醒/ACK、P4 旧在途与重新确认，P5/P6 失效。N1–N4 核对失效提交后旧世代提醒提交调用为零。 |
| 7 未知、字段暂缺、旧 CLI | P2 保守分类、P4 上次读取/真实时龄、P5/P6 降级；N1–N3 不把旧额度关联到新身份。 |
| 8 旧快照、数据库、消费者 | P1 升级和只读矩阵，P3 旧 ACK，P4 旧 producer/wire，P6 老 App Group；N1/N3 对照最终候选的兼容路径。 |
| 9 解析矩阵及旧失败恢复 | P2 有效两行、缺行、形状及时间错误、额外模型行；P4 五步序列与 DB 时间断言；N1 用实际构建复跑。 |
| 10 脱敏、名称、关联及未知字段 | P2 危险值和无凭据、P1 文件字节、P3 通知、P4 wire/日志/perf、P6 App Group 哨兵；N1–N4 只观察各允许表面。 |
| 11 portable backup、一般 export、Restore | P1 字节净化、恢复矩阵和一般 export；N1 在隔离状态重复备份/恢复并确认 JSON 不呈现源身份或额度。 |
| 12 中英文菜单栏和 Widget | P5/P6 各状态、两种语言、布局与 AX，N2/N3 真实 UI 补验。 |
| 13 历史坏 stdout 不可得 | P4 当前隔离恢复，N1 验证实际源码、嵌入 helper、调用环境并记录历史已知/未知及未复现处置。 |

### D1–D3 与 A0 契约

| 项 | Anchor 证据与联合验收 |
| --- | --- |
| D1 Widget 隐私 | P6 精简编码及三个尺寸断言；N3 仅 large 当前邮箱、全部无组织。 |
| D2 Claude 捕获屏障 | P1 转换及确认事务、P4 FIFO 和时刻矩阵；N1 旧捕获丢弃、同周期重新确认后新捕获通过，N2/N3 确认新 usage 可先显示。 |
| D3 只让 Claude 登录变化失效 | P1/P4 世代及 Codex 混合缓存用例，P3 Codex 告警回归；N1–N3 Codex 状态变化时沿既有隔离与归属语义。 |
| C0 无凭据、无认证 HTTP | P2 无凭据/no-network 测试，P4 边界哨兵；N1 只执行公开 CLI 子命令，N3/N4 只验证获准字段。 |
| C1 同一读取门禁 | P4 同一 `quota.Allowed`、关闭/客户端选择/official/记录与观察 provider 不一致、零查询和零尝试；N1–N3。 |
| C2 传输、握手、错误正文 | P2 同进程初始化、按 ID 单 reader、到期请求/迟到丢弃/截止/退出、禁令牌刷新及错误正文哨兵；N1 实际 helper。 |
| C2 既有额度字段与失败 | P2 跨 limitId 的 primary/secondary、百分比、重置时刻、标签、billing、reset allowance 期望和拒绝字段；区分传输失败与合法响应缺 rateLimits（not_reported）。P4 wire/窗口键回归及 N1 旧路径复核。 |
| C2 `planType → plan` 与 `plan_display` | P2 安全值与禁值用例，P4 DB、旧 wire/CLI 原值及不安全值 `plan=null / plan_reason=not_reported`，P5/P6 展示；N1–N3 验证只改展示文案。 |
| C2/C8 `accountId` 与关联证明 | P2 进程内共同键，P1/P4 固定 digest 域与 `discardOnAccountChange`、同周期关联和世代 0；N1–N3 缺键、冲突及换上下文。 |
| C3 状态栏适配器与捕获 | P4 同意前不注册、同一 stdin 交给原命令并先执行、stdout 字节透传、原命令失败时不新增空白、捕获失败不阻止原命令、64 KiB、缺 `rate_limits`、关闭/取消时注销清同意、`claude_status_line` 到 `claude_statusline`；FIFO 验证 stdin 前单调采样且写入时事务闸门生效。N1 走实际 capture 路径。 |
| C4 严格 prose 解析 | P2 全有或全无、三类 `failure_detail` 及无原始错误文本，P4 事务中存/清细分；N1 五步。 |
| C5 来源优先与时龄 | P4 真实年龄、原 `allowed_age` 与平局偏好状态栏，P1/P4 世代和闸门，P5/P6 独立身份有效期；N1–N3。 |
| C6 原因闭集、Claude 额度无套餐 | P4 旧 Swift 消费者与 CLI golden，保持 `plan=null / plan_reason=not_reported`，新增身份套餐独立；P5/P6 和 N2/N3。 |
| C7 无历史库存、quota 表换代 | P1 验证每客户端一行、仅保留现值/工作基线、旧表消失、预热写失败、旧二进制 SchemaAhead；N1 检查实际升级路径。 |
| C8 Claude 归属始终未确认 | P4 已知变化失效、未知保留真实时龄，P5/P6 独立文案；N1–N3 含 D3 对照。 |
| C9 两侧调度 | P4 usage 手动绕过间隔/退避但失败只记原因/实际失败时刻、后台链不推进，身份退避手动也遵守；验证进程内 single-flight、跨进程串行、两客户端并行、blocked 后联合机会与退避上限；N1 同环境取消与恢复。 |
| C10 无账号告警与世代 | P3 `qa2.`、旧 ACK、失效清账本、双进程同一投递守卫、初始化与中断恢复、只读先开组合和隐私。P4 核对 refresh 链路，N4 验证失效后旧世代提交为零并记录平台观察。 |
| C11 WireVersion 1 增量 | P4 旧解码器、显式 null/缺键/对象和未知新枚举，P6 App Group 三态；N1–N3 检查实际流。 |
| C12 CLI 文本兼容与 JSON 增量 | P4 文本金标和 JSON 字段断言；N1 用同一 helper 对照 `quota` 与 `--format json quota`。 |

A0 的其他保留项继续沿已有回归。P4 检查 reset allowance 总量、primary/secondary、`allowed_age` 和状态栏载荷可缺，P2/P4 验证套餐不参与业务分支和严格 prose，P5/P6 保留 Claude 未确认，P1 的 A8 净化同时回归旧 `quota-portable-restore-account-bound-state`。

## 审查与完成边界

- 分解文档进入独立评审，执行 `bash scripts/check-topic-docs.sh subscription-account-identity`、`make check-whitespace`、`git diff --check` 与链接核对（L0）。主控对实际候选登记并查询文档门禁；只有通过适用评审与门禁的 anchor 进入开发调度。
- 任务实施阶段用本任务列出的证据覆盖对应行为，遵循 [Project Rules](../../../.agent-instructions/project-rules.md) 的 L0–L4 和 [Evidence](../../../.agent-instructions/evidence.md) 的精确内容状态规则。Go 测试均走项目 wrapper；共享 Go 核心契约变化执行完整 Go 回归，确切未变状态可复用通过证据。每个 anchor 在实际 WorkUnit 上记录命令、结果、测试夹具、候选身份、独立评审及缺口；验收事实直接写入持久记录。最终主题门禁覆盖联合验收与所有剩余要求，未执行项保持开放。
- 稳定契约和手册在 v0.7.5 对账中补充账号/套餐持久化的批准范围、独立身份字段和兼容限制。仅在实际评审、所需门禁及授权交付条件满足后按各来源缺陷处理状态；将版本集成与发布留给相应授权边界。
