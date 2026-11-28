---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — Architecture

本文对 [`ux/menubar.md`](ux/menubar.md) 和 [`ux/widget.md`](ux/widget.md) 请求的每个字段给出供给或拒绝。实现以当前 main（已含 PR #74 Claude 可执行文件发现与 PR #75 `clientDisplayLabel`）为基线；本主题工作区基线 `4ac56db` 不含这两项，实现前必须先合入 main。

用户 2026-11-17 决定：D1 Widget 仅 large 显示脱敏邮箱，small/medium 不显示账号，任何 Widget 不显示组织名；D2 已知 Claude 上下文变化后，重新确认前忽略状态栏捕获；D3 登录上下文失效只适用于 Claude。本稿把 D2 落成可实现、在身份可读时必然完成的规则（A5.5、A5.6），不改变三项决定。

本稿是架构评审 Round 2 的修订稿（第二次有界修复），在 Round 1 修订稿（blob `57cd0606`）上关闭 Round 2 仍开放的 R1 发现与新发现 R2-F1–F3；各节以 `(R1-Fn)`、`(R2-Fn)` 标注。顺序判断一律不用墙钟：身份结果按刷新周期序号、状态栏捕获按同一次启动内的单调时钟比较，墙钟只用于展示。

## 术语

- **刷新周期**：一次 `agentdeck --format json desktop quota-refresh` 进程在持有 `quota-refresh.lock`（`store.AcquireQuotaRefreshLock`）期间完成的全部探测与写入。身份探测、失效与状态栏闸门发布只在刷新周期内发生，所有这些写入的 API 都要求调用者传入已持有的锁。
- **周期序号**（`cycle_seq`）：刷新周期取得锁后，在第一个写事务中把单行表 `quota_cycle.seq` 加 1 得到的整数。锁使周期串行，所以序号顺序就是尝试顺序；同一墙钟刻度或墙钟回拨都不影响它。
- **周期时刻**（`cycleAt`）：同一周期开始时取的墙钟，只用于展示（`observed_at`、`attempted_at`），不参与任何先后判断。
- **世代**（`context_generation`）：Claude envelope 上的整数，每次失效加 1。Codex 恒为 0（D3）。
- **基线**：Claude 身份行上最后一个可比较或明确退出的快照，用于判定"已知变化"（A5.3）。
- **捕获闸门**（`capture_gate`）：Claude envelope 上的状态，取 `open_initial`、`blocked`、`open` 之一；`open` 时附带发布它的启动标识与单调时刻（A5.5）。失效使其变为 `blocked`（D2）。
- **单调时刻**：darwin 上 `clock_gettime(CLOCK_MONOTONIC)`（含睡眠时间，同一次启动内跨进程单调不减），单位纳秒；**启动标识**：darwin 上 `sysctl kern.bootsessionuuid`，linux 上 `/proc/sys/kernel/random/boot_id`（单调时钟改用 `CLOCK_BOOTTIME`）。两者都经已有依赖 `golang.org/x/sys/unix` 读取。其他平台视为不支持（A5.5 的失败方向）。
- **守卫写入**：在同一个写事务（`BEGIN IMMEDIATE`）中先读世代、闸门或周期序号，再决定写或丢弃。

## A0 对 subscription-quota 契约的逐条声明（R1-F11）

| 旧契约 | 处置 | 说明 |
| --- | --- | --- |
| C0 无凭据、无认证 HTTP、断言测试 | 保留并扩展 | 新增读取只调用用户已登录客户端的公开子命令（`codex app-server` 的 `account/read`、`claude auth status`）；不读认证文件或 Keychain，不构造认证 HTTP；无凭据断言测试覆盖新增文件（A13） |
| C1 门禁（开关 → 客户端选中 → official；记录与观察 provider 不一致时抑制） | 保留并扩展 | 身份请求使用同一周期同一次 `quota.Allowed` 结果，含观察 provider 交叉校验；被拒时不启动身份进程、不发 RPC、不写尝试时间（A1.1） |
| C2 传输与握手 | 保留并扩展 | 同一 app-server 进程增加 `account/read`；读取改为单 reader 按 id 分派（A1.3）；JSON-RPC 错误正文不再进入任何错误文本（A6.4），这同时收紧现有 rateLimits 路径 |
| C2 字段映射 `planType → plan`「Opaque，只显示不分支」 | 部分替代 | 安全原值的 `plan`、wire 字段与 CLI 文本不变，仍无任何业务分支；新增 `plan_display` 投影（A4），只做文案映射。未通过 A6.3 禁止值筛查的原值不进入 `plan`、数据库或任何输出，旧字段报 `plan=null`、`plan_reason=not_reported`（R1-F7） |
| C2/C8 `accountId` 仅作隔离键 | 保留 | 原始 `accountId` 只在 adapter 进程内存在；持久化与隔离仍用既有 `accountDigest`（域 `agent-deck:quota:account:v1`）与 `discardOnAccountChange`；身份侧不新增账号键、不替换隔离键；关联判定只在进程内比较（A2.1） |
| C3 状态栏适配器 | 保留并扩展 | 保留：用户同意才注册；先执行原命令再捕获，原命令 stdout 字节透传，三条链式属性；`rate_limits` 缺失为 `not_reported`；`MaxStatusLinePayloadBytes`（64 KiB）解析上限；关闭读取或取消 Claude 时注销路由并清同意；存储来源串仍为 `claude_status_line`（wire 映射为 `claude_statusline`）。新增：进程入口、读 stdin 之前取一次单调时刻（不做 I/O，不能阻塞或改变链式调用）；链式调用结束后，写入在单个写事务内按 A5.5 的捕获闸门判定（R1-F1） |
| C4 prose 严格两行、全有或全无 | 保留；失败细分 | 接受语法不变；新增 `failure_detail`（A9）；解析错误不再携带行文本（A6.4） |
| C5 来源优先与陈旧判定 | 保留 | 平局偏好状态栏、`allowed_age` 不变；身份新鲜度另立规则（A5.2）；Claude 窗口另受世代与到达边界约束（A5.4、A5.5） |
| C6 原因闭集；Claude 无 plan | 保留 | 闭集不扩展（旧 Swift 拒绝未知值）。Claude 的 `plan=null`、`plan_reason=not_reported` 只保留原语义：额度通道不报告套餐；身份侧套餐走新字段，不借用、不改写这两个字段（A4） |
| C7 不存时间序列 | 保留；新增身份表；quota 表换代 | 每客户端一行；基线与关联判定是工作值，不是历史；被替换的身份不保留。`quota_windows`、`quota_envelopes`、`quota_alert_notices` 由迁移换代为 `_v2` 表并删除旧表，使迁移前已打开的旧连接和旧二进制无法继续写入（A7） |
| C8 Claude 无账号、`attribution_confirmed:false`；换号后旧数字继续显示 | 部分替代 | `attribution_confirmed` 恒为 false；"已知变化后继续显示"替代为失效（A5.3、A5.4）；未知变化仍按真实年龄显示；Codex 不失效（D3） |
| C9 调度 | usage 侧保留；身份侧另立 | usage：手动绕过间隔与退避；手动失败只写原因与 `failure_observed_at`，不推进后台退避链（GS-R2-F1、GS-R3-F1）；进程内 single-flight 保留；跨进程由 `quota-refresh.lock` 串行化整个周期，替代旧 C9 中"跨进程并发不处理"的接受（GS-R1-F3）。身份：独立退避，手动不能绕过；已知变化后、重新确认前提供不越过退避的联合机会（A1.2，R2-F1）；两客户端在周期内并行（A1.5） |
| C10 告警 | 保留并扩展 | 告警 ID `qa2.` 携带世代，告警无账号内容。App 持 `quota-delivery.lock` 的独占 flock 复核世代并同步提交。helper 持同一守卫提交 Claude 失效（A5.7，R2-F2）。`quota-alerts ack` 沿用既有入口，失效清空 Claude 去重账本。 |
| C11 wire 增量、WireVersion 1 | 保留并新增字段 | 见 A10 |
| C12 CLI | 保留 | `agentdeck quota` 文本输出与 golden 不变；`agentdeck --format json quota` 随 A10 增量 |
| Open question 1 `reset_allowance.total` | 保留 | — |
| Open question 2 `plan` 不可分支 | 部分替代 | 允许展示映射；仍禁止业务分支 |
| Open question 3 状态栏载荷存在性不假设 | 保留 | — |
| Open question 4 prose 严格匹配 | 保留 | 细分只改诊断类别，不放宽匹配 |
| Open question 5 Claude 无账号标识 | 保留并细化 | auth status 给出当前登录身份，但两条额度通道仍无账号 ID；`attribution_confirmed:false` 不变 |
| Open question 6 primary/secondary | 保留 | — |
| Open question 7 `allowed_age` 常量 | 保留 | 身份新鲜度另用 `2 × interval`，不调整该常量 |
| fix `quota-portable-restore-account-bound-state` | 扩展 | 恢复时一并清空身份与上下文状态；空目标契约与凭据归档范围不变（A8，R2-F3） |

## A1 门禁、触发与预算

### A1.1 门禁

身份读取与 usage 使用同一周期内同一次门禁结果（`quota.Allowed`，含 `probeEnabled && clientEnabled`、记录 provider 与观察 provider 交叉校验）。被拒时不启动身份子进程、不发 RPC、不写尝试时间。手动刷新、视图渲染、缓存恢复都走同一入口；只读路径（`agentdeck quota`、快照、缓存恢复）不探测，只按 A10 投影。

### A1.2 到期、退避与联合机会（R1-F2、R2-F1）

- **usage**：保持 C9。
- **身份**：后台触发在不处于身份退避期、且距 `attempted_at` 满一个 interval（或从未尝试）时到期；手动触发绕过间隔，不绕过身份退避。
- **身份退避**：每次真实发起的身份查询，结果为 `probe_failed`、`parse_failed` 或 `incomparable` 时推进退避链（手动与后台都推进）；起点为 interval，几何增长，上限复用 `quotaMaxBackoff`（1 小时）；结果为 `identified`、`signed_out` 或 `non_subscription` 时清零。链由 `backoff_base_at` 与 `backoff_until` 两列推导，规则与 usage 的 `failure_at`/`backoff_until` 相同。
- **联合机会（仅 Claude，`capture_gate = blocked` 且尚未重新确认期间，R2-F1）**：只为"两侧都被允许"的周期提供一次真实的联合运行，不强迫任何一侧越过自己的退避：
  1. 本周期任一侧按自己的规则到期（身份按本节，usage 按 C9；手动触发按各自的手动规则）才进入联合判断，否则本周期 Claude 不运行。
  2. 身份不在身份退避期内即运行（忽略身份间隔）；处于身份退避期则本周期不查身份。
  3. 只有本周期身份结果为可比较的 `identified`（唯一能重新确认的结果），且 usage **不在** usage 退避期内（手动触发按 C9 不受 usage 退避约束），usage 才在身份提交之后被拉入本周期（忽略 usage 间隔）。
  4. 身份结果为 `signed_out`、`non_subscription`、`unknown` 或本周期未查身份时，usage 只在它按 C9 本来就到期时运行；不因待确认被强制。
  5. 被拉入的 usage 失败按触发类型照常处理（后台推进 usage 链，手动不推进）。
- **不会饿死（含两侧都成功但到期时刻错开的情形）**：设从时刻 `t0` 起，身份每次都返回可比较的 `S`，`/usage` 每次都成功；记 App 触发 quota 刷新的间隔为 `τ`（刷新周期只在 App 触发时运行）。关键是第 2 步与第 3 步分别忽略对方的**间隔**，只尊重对方的**退避**：任一侧到期，另一侧只要不在退避期就同周期运行。由此逐情形：
  - **两侧都不在退避期，到期分钟错开**（例如身份在每个 interval 的第 0 分钟到期，usage 在第 2.5 分钟到期）：在 `t0` 之后第一个有任一侧到期的周期，身份因第 2 步运行并返回 `S`，usage 因第 3 步被拉入，两侧同周期成功，重新确认完成。错开的相位不会导致两侧在不同周期各自单独成功。失效会清空 usage 的 `observed_at` 与退避，usage 在失效后立即到期，所以导致失效的那一轮（身份结果为 `S`）本身就是联合周期。
  - **usage 在退避中、身份正常**：身份到期的周期不拉入 usage。usage 退避结束时它按 C9 到期（退避结束时刻不早于上次失败加一个 interval）；该周期身份不在退避期，第 2 步运行身份，第 3 步运行 usage，完成重新确认。最迟在 `t0 + usage 剩余退避（≤ 1 小时）+ τ`。
  - **身份在退避中、usage 正常**：usage 到期的周期只运行 usage（可展示，不能重新确认）。身份退避结束即到期，第 2 步运行身份并返回 `S`；此时 usage 刚成功、不在退避期，第 3 步把它拉入（忽略其间隔），完成重新确认。最迟在 `t0 + 身份剩余退避（≤ 1 小时）+ τ`。
  - **两侧都在退避中**：两侧都从 `t0` 起成功，因此各自退避结束后不再进入退避；在较晚结束的那一侧到期的周期，另一侧已不在退避期，按上两条完成重新确认。最迟在 `t0 + max(两侧剩余退避) + τ ≤ t0 + 1 小时 + τ`。
  - 手动刷新在身份退避期外每次都提供一次联合机会（手动 usage 按 C9 不受 usage 退避约束）。
  - 只有身份或 usage 一侧从不成功时才不会重新确认，这正是 D2 的含义。A13.2 为以上每一种情形各设一个确定性用例。起草时用一个合成模拟按第 1–4 步逐分钟推进验证了上述上界：interval 5 分钟，`τ` 1 分钟；两侧相位各取 10 个错开值，两侧剩余退避各取 0、10、30、60 分钟，usage 观测分清空与未清空两种，共 3,200 个组合全部在上界内出现联合周期，最坏为 60 分钟。
  - **前提**：以上上界假设刷新周期确实被触发（App 每 `τ` 触发一次），且 Claude 身份提交能在 `G` 内取得投递守卫（A5.7）。守卫只被 App 的无 `await` 临界区短暂持有；若某个周期因守卫竞争放弃了身份提交，该周期不计入，上界顺延一个 `τ`。以上是连续获得守卫的周期上界。若持续争用，联合机会推迟到实际获得守卫的周期。
- **周期内顺序（Claude）**：身份探测 → 身份转换事务提交（可能失效，A5.3）→ **此后**读取世代 → usage 探测 → usage 守卫提交 → 若满足 A5.6 则发布捕获闸门。usage 必须在身份提交之后读世代；身份提交失败（含 SQLite 忙或 `G` 内取不到投递守卫，A5.7）时本周期不再启动 usage（A1.5 退场条件），以免把已观察到的新上下文的 usage 写进旧世代。

### A1.3 Codex 单进程（R1-F10）

- 请求 id：`initialize`=1，`account/rateLimits/read`=2，`account/read`=3。任一侧到期才启动进程，只发送到期的请求，顺序为握手 → `account/read` → `rateLimits`。
- **单 reader 分派**：一个 goroutine 逐行读 stdout 并解码 JSON-RPC 信封，按 id 投递给仍在等待的请求（每个请求一个容量为 1 的通道）；没有等待者的 id（超时后迟到的响应）直接丢弃，不解析 result。无法解码的行、读错误或 EOF 作为进程级致命事件广播给所有未完成的请求。
- **截止**：进程截止 = 启动 + 5 s +（两侧都到期时再加 2 s）。`account/read` 的子截止为发送后 2 s（不超过进程剩余时间）；子截止到期时身份侧记 `probe_failed`，进程保留，继续发送 rateLimits。因此 rateLimits 拿到的剩余时间不少于当前实现（当前 5 s 同样包含启动与握手）。
- **收尾**：最后一个需要的响应到达后关闭 stdin，`Wait` 受进程截止约束；`cmd.WaitDelay = 500 ms`，防止孙进程持有管道导致 `Wait` 不返回。进程在响应之后非零退出或被信号终止，已应答的各侧一律记失败（沿用 Codex PR #5 第六轮规则：崩溃进程的输出不得持久化）。
- **致命事件归类**：启动失败、握手失败、应答前退出、截止到期、传输畸形时，本周期**到期且尚未成功的每一侧**各自记失败并按 A1.2 处理退避：畸形 → `parse_failed`，其余 → `probe_failed`。已经成功应答的一侧不受后续致命事件影响，除非进程随后非零退出（上一条）。
- **调用者取消**：传入的 context 取消时杀进程；失败写入使用同一 context，通常不会落库；下个周期照常重试。不新增特殊处理。
- 若生成的 schema 为 `account/read` 声明了令牌刷新参数，必须显式为 false；这是凭据操作，不得触发。

### A1.4 Claude

每个 Claude 周期调用一次 `claudeCommandPath`（`internal/quota/claude_exec.go`），结果供 `auth status`（4 s）与 `/usage`（原 10 s）共用，两次调用使用相同环境与工作目录；两次子进程都设 `WaitDelay = 500 ms`；顺序固定为先身份后 usage（A1.2）。

### A1.5 周期总截止（R1-F10）

- 既有总预算不变：Swift `EmbeddedHelperRunner.quotaRefreshTimeout = 40 s`。
- Go 侧 `desktop quota-refresh` 在命令开始时设 `D = 开始 + 37 s` 的截止。打开 store、取锁、探测、告警评估及输出按此截止分配预算。调用者取消直接传入写操作。Swift 的 3 s 余量用于启动 helper 与完成进程交接。
- **两客户端并行**：Codex 与 Claude 在两个 goroutine 中并行执行，各自内部顺序不变；进程内 per-client single-flight 不变；告警评估在两者都结束后进行。
- **SQLite 写入与守卫预算（R1-F10）**。每次刷新写操作在入口设单调截止 `min(当前时刻 + 1 s, 父 context 的截止)`，使用同一写 context 获取专用 `*sql.Conn`、申请写锁并执行事务。写操作预算 `W = 1 s`。连接保存原 `busy_timeout`，本次以 `PRAGMA busy_timeout = 0` 禁用 SQLite 内部等待。Go 只在 `BEGIN IMMEDIATE` 返回 `SQLITE_BUSY` 且尚未开启事务时，在截止内等待下一次尝试。每次允许取锁前都核对截止和取消。截止后返回本次写操作已获得的原生 `SQLITE_BUSY`，尚无此错误时返回 `context.DeadlineExceeded`。调用者显式取消沿用 context 取消。获得写锁后再次核对截止和取消，迟到取锁直接进入回滚。事务体至多执行一次，提交出错沿用既有结果规则。
- **SQLite 写入清理**。需要回滚时先执行 `ROLLBACK`，再恢复原 `busy_timeout`。`BEGIN IMMEDIATE` 的结果因取消或超时而不明确时，按可能已开启事务处理。回滚和恢复共用一个与写截止、调用者取消分离的清理 context，预算 `C = 1 s`。写步骤准入前合计预留 `W + C = 2 s`，包括提交后的设置恢复。清理结束前专用连接仍由本次操作持有。回滚或恢复任一失败时，通过 `Conn.Raw` 的回调返回 `driver.ErrBadConn` 丢弃物理连接。清理只负责释放事务和恢复连接设置，实际耗时单独记录。
- 投递守卫（A5.7）每次获取预算 `G = 1 s`。首个允许的 Claude 周期将守卫初始化及续作计入首次 `G`，同一截止也约束取锁。一个周期包含一次序号写入（探测前），一次守卫对账（取守卫及一次 24 字节镜像写），一次 Codex 合并提交或失败记录，Claude 身份提交（取守卫并写入一次）、usage 提交或失败记录、至多一次闸门发布，以及一次告警修剪写入。
- **阶段上限**：
  - 周期序号加守卫对账 `W + C + G + 0.1 = 3.1 s`。
  - 探测阶段 `P = max(Codex 7 + 0.5 + (W + C), Claude 4 + 0.5 + G + (W + C) + 10 + 0.5 + 2 × (W + C)) = 22 s`。
  - 告警评估与输出 `R = 0.5 + W + C = 2.5 s`。
- **锁等待**为 `min(20 s, D − now − (W + C + G + 0.1 s) − P − R)`。结果不大于 0 时返回既有 `ErrStateBusy`，零进程、零刷新写入。
- **退场条件**：每一步开始前核对 `剩余时间 ≥ 该步上限 + 其后必需写入`，不满足则跳过该步及之后的探测，不记尝试时间；周期序号写入失败 → 整个周期不探测；Claude 身份提交失败 → 本周期不启动 usage；任何守卫提交失败 → 不写失败记录（下个周期照常重试）。
- **最坏阶段预算**。打开 store 和等待 refresh 锁共分配 `37 − 3.1 − 22 − 2.5 = 9.4 s`。持锁周期分配 27.6 s，包含各写步骤的清理。总计 37 s，Swift 总预算为 40 s。第二个 helper 的锁等待到截止仍未取得时返回 busy，零探测、零刷新写入，App 继续读取订阅段。投递守卫竞争时按截止放弃 Claude 身份提交。验收分别检查各阶段的截止准入和受控时钟的 37 s 组合，再单独记录真实时钟下的等待结束、SQLite 返回、清理及 helper 完成时间。

## A2 Codex 身份（account/read）

- 与 rateLimits 共用同一个 app-server 进程和握手（A1.3）。
- **白名单与类型**：账号类型（字符串枚举）、`email`（字符串）、`workspaceName`（字符串或 null）、`planType`（字符串或 null）；若生成的 schema 声明了能与 rateLimits `accountId` 比对的账号或工作区标识字段，也读入（字符串），只用于 A2.1 的进程内比较。解码结构只含这些字段，其余全部丢弃；字段类型不符记 `parse_failed`。确切键名在实现任务中用 `codex app-server generate-json-schema`（隔离 `CODEX_HOME`）冻结为夹具（A13.1）；本文不猜未声明字段。
- **分类**：account 为空且需要认证 → `signed_out`；API key 类型 → `non_subscription`；ChatGPT 类型且邮箱通过 A6.1 校验 → `identified`；未知类型或邮箱无效 → `incomparable`；JSON-RPC 错误 → `probe_failed`（不保留错误正文）；畸形 → `parse_failed`。
- **显示值**：邮箱、工作区名、套餐原值都在 adapter 边界按 A6 转为安全显示值，未通过的字段为 null；原始值不出 adapter。
- **不做**：Codex 身份变化（包括 `signed_out`、`non_subscription`）不触发额度失效（D3），账号隔离仍由 `accountDigest` 负责（C8）。不为 Codex 存储任何关联值或账号键。
- Codex 可执行文件仍按裸名 `codex` 解析，GUI 启动时可能找不到；这是 PR #74 只覆盖 Claude 留下的缺口，按 tasks.md 作为单独 Lane A 处理，本主题不另写发现逻辑。

### A2.1 关联判定与绑定（R1-F4）

- **证据**：同一个 app-server 进程内，`account/read` 与 `rateLimits` 都成功应答、都给出 schema 声明的共同稳定键，且值逐字节相等。邮箱、套餐、工作区名相等或同一进程本身都不构成证据。schema 没有该键时恒为 `unconfirmed`（当前事实）。
- **原子提交**：两侧都在同一进程成功时，身份行、usage（窗口记录、修剪、envelope）与关联判定在**同一个写事务** `CommitCodexCycle(cycleSeq, cycleAt, identity, usage, evidence)` 中提交；证据成立时写 `association = 'confirmed'`、`association_cycle_seq = cycleSeq`，否则写空。任一侧失败时不写 `confirmed`。
- **自动降级**：只有身份单独成功的提交、只有 usage 单独成功的提交（包括 `PutEnvelope` 路径）、`discardOnAccountChange` 触发的丢弃，都在各自的事务里把 `association` 清空。失败写入不清空，因为它不替换任何一侧，由下一条投影条件约束。
- **投影**：`identity.quota_association = "confirmed"` 当且仅当 `association = 'confirmed'`，且身份 `success_cycle_seq`、envelope `cycle_seq` 都等于 `association_cycle_seq`（按周期序号而非墙钟比较，同刻或回拨的两个周期不会被当成同一周期），且身份 `freshness = current`（含门禁重新验证地板，A5.2），且本次投影门禁通过；身份为 `identified` 但不满足这些条件时为 `"unconfirmed"`；其余为 null。
- **覆盖**：身份单独成功、额度单独成功、共同键不匹配、一侧写入失败、账号变化丢弃、证据超过 `2 × interval`、设置变更重新验证，都在 L2 中以带共同键的假 schema 夹具验证降级。

## A3 Claude 身份（auth status）与可执行文件复用

- **可执行文件**：见 A1.4，不另写发现逻辑。
- **调用**：使用需求引用的 Claude Code 2.1.325 只读样本对应的 JSON 输出形式；命令参数、键名与退出码语义在实现任务中以脱敏夹具冻结（A13.1）。
- **输出边界**：stdout 最多读 64 KiB，超出记 `parse_failed`；stderr 读取后丢弃，不解析、不保存。
- **退出码**：截止超时或被信号终止一律 `probe_failed`。退出码非零时，只有 stdout 恰好是一个合法 JSON 对象、`loggedIn` 为布尔 `false`，且该约定已由样本夹具证实，才接受为 `signed_out`；非零退出加 `loggedIn: true` 或其他内容记 `probe_failed`。
- **白名单与类型**：`loggedIn`（布尔，必需）、`authMethod`、`apiProvider`、`email`、组织 ID、组织名、`subscriptionType`、`planTier`（均为字符串或 null）。`configDirectory`、`storedClaudeAiLogin`、路径和未知字段在解码时丢弃。必需字段缺失或类型不符记 `parse_failed`。
- **组织状态**：组织 ID 非空 → 有组织；样本夹具证实的"个人账号"表示（例如键存在且为 null）→ 明确无组织；键缺失或形式未被夹具覆盖 → 组织未知。
- **分类**：`loggedIn=false` → `signed_out`；API key 或第三方 `apiProvider` → `non_subscription`；订阅 OAuth、邮箱通过 A6.1、组织状态已知、本机秘密可用（A6.6）→ `identified`（可比较）；`authMethod` 未知、组织未知或秘密不可用 → `incomparable`；子进程失败 → `probe_failed`；畸形输出 → `parse_failed`。
- 旧版 CLI 不支持该子命令时只能从 stderr 文本识别，过于脆弱，归入 `probe_failed`，不单设 `not_supported`。

## A4 套餐展示投影（R1-F12）

`plan_display = {state, name, short_name, basis, source, raw, reason}`。

| 字段 | 取值 | 含义 |
| --- | --- | --- |
| `state` | `known` / `raw_only` / `conflict` / `missing` | 封闭集合 |
| `name`、`short_name` | 字符串或 null | `known`：映射名或官方展示名；`raw_only`：两者都等于安全原值；`conflict`、`missing`：都为 null |
| `basis` | `vendor_display_name` / `mapping` / `raw` / null | 名称依据；`conflict`、`missing` 为 null |
| `source` | `codex_rate_limits` / `codex_account_read` / `claude_auth_status` | 来源通道 |
| `raw` | `[{field, value}]` | 通过安全检查的原值；`missing` 时为空数组 |
| `reason` | `not_reported` / `unsafe_value` / null | 只在 `missing` 时非空 |

**每个来源的判定顺序**：

1. 冻结夹具中的 schema 明确声明了官方展示名字段，且其值通过 A6.2 与 A6.3 → `known`，`basis = vendor_display_name`；schema 同时声明短名则用之，否则 `short_name = name`。当前所有证据中都没有这样的字段，此规则只在有夹具证据后生效，不猜字段名。
2. 原值命中映射表 → `known`，`basis = mapping`。映射表是唯一的代码常量：Codex `prolite` → `ChatGPT Pro 100` / `Pro 100`；Claude `pro` → `Claude Pro` / `Pro`。
3. 原值是安全 token（`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`，因此不含 `@` 或空白）→ `raw_only`，`name = short_name = 原值`，`basis = raw`。
4. 原值存在但不是安全 token → `missing`，`reason = unsafe_value`，`raw` 为空。
5. 原值缺失 → `missing`，`reason = not_reported`。

**Claude 两个来源**（`planTier`、`subscriptionType`）：任一存在的值不安全 → `missing`/`unsafe_value`；都缺失 → `missing`/`not_reported`；只有一个 → 按它判定；两者逐字相等 → 按它判定；两者不同 → `conflict`，`raw` 列出两者。

**存在性**：

- 客户端级 `plan_display`：键缺失表示旧 producer；Claude 恒为 null（额度通道没有套餐）；门禁关闭时为 null；Codex 门禁通过时为对象。
- `identity.plan_display`：身份最后一次成功为 `identified` 时为对象，否则为 null。

**消费者回退**（菜单栏胶囊、Widget large，中英文一致，R1-F12）。消费者必须区分三态，不得用 `decodeIfPresent` 把缺键与显式 null 合并（A11）：

| `plan_display` 键 | 消费者行为 |
| --- | --- |
| 缺失（旧 producer、旧 App Group 文件） | 回退到旧 `plan` 原值与 `plan_reason`，即今天的行为 |
| 显式 null（Claude，或门禁关闭） | 省略套餐胶囊。Claude 的适用额度在菜单栏可见时，套餐缺失行使用 [菜单栏 UX](ux/menubar.md) 的 `quota.reasons.plan_not_reported`。旧 `plan` 仅供缺键分支使用 |
| 对象，`short_name` 非空 | 显示 `short_name`；`help` 给 `name` 与 `raw` |
| 对象，`state = conflict` | 不显示胶囊；菜单栏行 b 显示 `quota.planDisplay.conflict` |
| 对象，`state = missing` | 省略套餐胶囊。菜单栏以对象自身的 `reason` 选择 UX 文案，`not_reported` 对应 `quota.reasons.plan_not_reported`，`unsafe_value` 对应 `quota.reasons.plan_unsafe`。其他消费者沿用既有套餐不可用呈现。旧 `plan_reason` 仅供缺键分支使用 |

品牌名在中英文界面都保持英文。

**旧 `plan` 路径与新投影使用同一筛查（R1-F7）**：Codex rateLimits 的 `planType` 在 adapter 边界先经 A6.3 禁止值筛查与 A4 安全 token 规则。通过时，`plan` 原值、数据库 `plan` 列、wire `plan` 与 CLI 文本按 C2/C12 原样保留（需求要求的原始套餐文本诊断）。未通过时，原值不写入数据库、wire、CLI、日志或错误，旧字段报 `plan = null`、`plan_reason = not_reported`（闭集内最接近的既有值），`plan_display` 报 `missing`/`unsafe_value`。安全原值因此在所有路径上保持不变，不安全值在所有路径上都不出现。映射表不参与任何金额、权限或配额分支，有测试断言这一点。

## A5 身份状态机、新鲜度与上下文世代

### A5.1 身份行的四组字段（R1-F5）

| 组 | 字段 | 写入者 |
| --- | --- | --- |
| 最近尝试 | `attempt_cycle_seq`、`attempted_at`（展示）、`attempt_failure`（`'' / probe_failed / parse_failed / incomparable`）、`failure_cycle_seq`、`backoff_base_at`、`backoff_until` | 每次真实发起的查询 |
| 上次成功 | `success_cycle_seq`、`success_state`（`'' / identified / signed_out / non_subscription`）、`success_observed_at`（展示）、`account_kind`、`email_masked`、`org_kind`、`org_name`、`plan_sources_json` | 只有结果为三种成功状态的查询；`incomparable` 不算成功 |
| 比较基准（仅 Claude） | `baseline_kind`（`'' / subscription / signed_out / non_subscription`）、`baseline_correlation`、`baseline_key_id`、`baseline_observed_at` | 只由 A5.3 转换表改变 |
| 关联（仅 Codex） | `association`、`association_cycle_seq` | A2.1 |

**覆盖顺序按尝试次序，不按墙钟（R1-F5）**：身份写入 API 要求调用者持有 `quota-refresh.lock` 并传入本周期的 `cycle_seq`；事务内若 `cycle_seq ≤ attempt_cycle_seq` 则整体丢弃（防御锁外或重放调用者）。锁覆盖从探测开始到提交结束，所以提交顺序就是尝试顺序。同一墙钟刻度内"成功后失败"的两次尝试，以及墙钟回拨后的权威退出，都按序号正确覆盖；墙钟只写入 `attempted_at`、`success_observed_at` 供展示。失败只改"最近尝试"组，从不改"上次成功"组或基线。迁移后与 Restore 后没有身份行，等价于全部为空：序号为 0、基线 `none`、显示 `unknown(never_probed)`。

### A5.2 显示状态与新鲜度

- 门禁关闭 → `identity = null`。
- 否则：从未尝试且无成功 → `unknown(never_probed)`；`failure_cycle_seq > success_cycle_seq` → `unknown(attempt_failure)`；否则 `state = success_state`。
- `freshness = current` 需同时满足：`state` 不是 `unknown`；`success_cycle_seq > quota_cycle.revalidate_seq`（见下）；`0 ≤ now − success_observed_at ≤ 2 × interval`（墙钟回拨使差值为负时按 `last_read` 处理，属于保守方向）。否则为 `last_read`。
- **重新验证地板按序号**：迁移 N 安装触发器，在 `provider_selections` 任一行插入或更新、`settings` 中 `quota.probe` 或 `quota.probe.clients` 的值插入或实际变化（`OLD.value IS NOT NEW.value`）时，执行 `UPDATE quota_cycle SET revalidate_seq = seq`。门禁重新打开或选择变化之后，必须有一个更大序号的周期重新读到身份才算 current。它替代 Round 1 的 `selectedAt` 与 `quota.identity.revalidate_after` 墙钟比较，因此 R1 中的该设置键不再存在。
- `current_until = success_observed_at + 2 × interval`，仅 `freshness = current` 时给出，否则为 null。菜单栏与 Widget 都在本地再核对 `渲染时刻 < current_until`，Widget 按 A11 在该时刻生成一个不含邮箱的 timeline entry（R1-F3）。
- `email_masked`、`organization_*`、`identity.plan_display` 只在 `success_state = identified` 时非空；`state = unknown` 而这些字段非空时，消费者按"上次读取"渲染。最后一次成功为退出状态时这些字段为空，不显示旧邮箱。

### A5.3 Claude 转换表（R1-F5）

可比较的 `identified` 观察记为 `S(c, k)`：`c` 为关联值，`k` 为 key id（A6.5）。

| 当前基线 | 本次观察 | 失效（原因） | 新基线 | 上次成功组 |
| --- | --- | --- | --- | --- |
| `none`（迁移后、Restore 后或从未成功） | `S(c, k)` | 否：无法证明变化，旧窗口按真实年龄与未确认归属显示 | `subscription(c, k)` | 写入 |
| `none` | `signed_out` / `non_subscription` | 是（同名原因）：明确不是订阅上下文，迁移前的旧窗口不可能属于当前上下文 | `exited(该状态)` | 写状态，清空显示字段 |
| `subscription(c, k)` | `S(c, k)` | 否 | 不变（更新 `baseline_observed_at`） | 写入 |
| `subscription(c, k)` | `S(c', k)`，`c' ≠ c` | 是（`context_changed`） | `subscription(c', k)` | 写入 |
| `subscription(c, k)` | `S(c', k')`，`k' ≠ k` | 是（`identity_key_reset`） | `subscription(c', k')` | 写入 |
| `subscription(…)` | `signed_out` / `non_subscription` | 是（同名原因） | `exited(该状态)` | 写状态，清空显示字段 |
| `exited(x)` | 同一个退出状态 `x` | 否（重复退出幂等） | 不变 | 只更新时间 |
| `exited(x)` | 另一种退出状态 `y` | 是（`y`） | `exited(y)` | 写状态 |
| `exited(x)` | `S(c, k)` | 是（`context_changed`）：退出期间写入的窗口不属于新订阅上下文 | `subscription(c, k)` | 写入 |
| 任意 | `incomparable` / `probe_failed` / `parse_failed` | 否：不等于已知换号 | 不变：中间的 unknown 不重置基线 | 不变，只写尝试组 |

每次转换在单个事务 `ApplyClaudeIdentity(lock, cycleSeq, cycleAt, observation)` 中完成：读身份行与 envelope → 按 A5.1 丢弃 `cycleSeq ≤ attempt_cycle_seq` 的写入 → 查表 → 写身份行 → 需要时在同一事务执行 A5.4 的失效。

### A5.4 失效事务与世代守卫（R1-F1）

**失效**（与 A5.3 同一事务）：

- `context_generation += 1`；
- 删除 `quota_windows_v2` 与 `quota_alert_notices_v2` 中 Claude 的全部行；
- 清空 Claude envelope 的 `source`、`observed_at`、`cycle_seq`、`failure`、`failure_at`、`backoff_until`、`failure_observed_at`、`failure_detail`（旧消费者因此看到 `never_probed`）；
- 写 `invalidated_at = cycleAt`（展示）、`invalidated_reason`、`capture_gate = 'blocked'`、`confirmed_generation = -1`，清空 `capture_open_boot` 与 `capture_open_mono`；
- envelope 行不存在时以 upsert 创建。

**守卫**：Claude 的每一条额度写入路径都是带期望值的单个写事务；失效只在刷新周期内发生，守卫同时防御锁外写入者与未来调用者。

| 写入路径 | 期望值 | 不一致时 |
| --- | --- | --- |
| prose 成功 `CommitClaudeUsage(lock, gen, cycleSeq, windows, reconfirmEvidence)`：窗口写入、修剪、envelope 合并为一个事务 | 身份转换提交后读取的世代（A1.2）；`cycleSeq` 大于 envelope 已存 `cycle_seq` | 整体丢弃，不写失败 |
| prose 失败 `PutEnvelopeFailure(lock, gen, cycleSeq, …)` 及 `failure_detail` | 同上 | 丢弃，不推进退避 |
| 状态栏捕获 `RecordStatusLineCapture` | 捕获闸门（A5.5） | 丢弃 |
| 告警 ACK | 告警 ID 中的世代 `g`（A5.7） | 不写去重账本 |
| 失效事务本身 | 投递守卫（A5.7）：先取守卫、先写文件 `epoch`，再提交 | 取不到守卫则放弃本周期身份提交 |
| 告警评估 `DueAlerts` | 同一读事务读取窗口、envelope 与世代，并把世代写入 ID | — |

- Claude 不再依赖 `staleAgainstCurrent`（它只比较 `ObservedAt`，失效后 `ObservedAt` 被清空，挡不住倒灌）；`CommitClaudeUsage` 在事务内检查世代与周期序号。Codex 路径保留 `staleAgainstCurrent` 与 `discardOnAccountChange`，世代恒为 0，另加周期序号单调检查。
- **告警 ID**：新前缀 `qa2.`，载荷在原字段上增加 `g`（世代）；`ValidateAlertIDs` 同时接受 `qa1.`，视为 `g = 0`。对 Claude，`qa1.` 只在当前世代为 0 时放行。
- **旧写入者无法继续**：迁移 N 把三张 quota 表换代为 `_v2` 并删除旧表（A7）。迁移前已打开的旧连接，其已准备的 `INSERT … INTO quota_windows` 在重新准备时得到 `no such table`；旧二进制重新打开数据库时遇到更高的 schema 版本被拒绝（`SchemaAhead`）。两种情况都按现有的失败开放处理丢弃本次写入，不会写进任何新表。旧表名今后不再复用，没有可被旧语义继续写入的保留列或保留表（合成探针见 A13.2）。

### A5.5 状态栏捕获闸门（D2，R1-F1）

D2 的判定用同一次启动内的单调时钟建立跨进程可验证的先后，失败一律落在"丢弃"一侧。

**Claude envelope 上的闸门列**：

- `capture_gate`：`open_initial`（迁移后、Restore 后或从未失效）、`blocked`（失效后）、`open`（重新确认已提交且已发布）；
- `confirmed_generation`：已提交重新确认的世代，未确认时为 -1；
- `capture_open_boot`、`capture_open_mono`：发布 `open` 时的启动标识与单调时刻（纳秒）。

**捕获进程**（`runQuotaCapture`）：

1. **入口，读 stdin 之前**：执行一次 `clock_gettime(CLOCK_MONOTONIC)` 得到 `entry_mono`，并取一次墙钟 `entry_wall`（只用于窗口的 `observed_at` 展示）。这是 commpage 上的时钟读取，不做 I/O，不会阻塞或失败到影响链式调用；读取失败只把 `entry_mono` 标为无效。
2. `io.ReadAll(stdin)`，解析原命令，执行链式调用，stdout 字节透传——与 C3 完全相同，链式调用之前不打开数据库。
3. 链式调用结束后读取启动标识 `entry_boot`（进程存活期间启动标识不变，读取放在链式调用之后不影响比较）。
4. 照旧以 200 ms 打开 store、读取开关；随后在专用连接上设 `busy_timeout = 200` 并开 `BEGIN IMMEDIATE` 写事务，读 Claude envelope，按下表判定。

| 闸门 | 放行条件（全部满足） | 否则 |
| --- | --- | --- |
| `open_initial` | 无（从未发生已知变化） | — |
| `blocked` | 永不放行 | 丢弃 |
| `open` | `entry_mono` 有效；`entry_boot` 非空且等于 `capture_open_boot`；`entry_mono > capture_open_mono`（严格大于，相等即丢弃）；`confirmed_generation = context_generation` | 丢弃 |

放行时在同一事务内向 `quota_windows_v2` 写入全部窗口：`source = 'claude_status_line'`（存储来源串；wire 映射为 `claude_statusline`），`observed_at = entry_wall`，`context_generation` 为当前世代；C5 的 `Accept` 照常适用。

**闸门发布**（只在刷新周期内、持锁执行）：

1. **重新确认提交**（A5.6 的 tx1）把 `confirmed_generation` 设为当前世代，闸门**保持 `blocked`**。
2. tx1 的 `COMMIT` 返回之后，才读取 `T = clock_gettime(CLOCK_MONOTONIC)` 与当前启动标识 `B`。
3. 发布事务 tx2：`BEGIN IMMEDIATE`，核对 `capture_gate = 'blocked'` 且 `confirmed_generation = context_generation`（期间没有新的失效），然后写 `capture_gate = 'open'`、`capture_open_boot = B`、`capture_open_mono = T`。

由于 `T` 取自 tx1 提交之后，任何满足 `entry_mono > T` 的捕获都在重新确认提交之后才进入；使用提交前取的时刻会放行"提交前进入、提交后写入"的捕获，本设计不这样做。

**逐项闭合**：

| 情形 | 结果 |
| --- | --- |
| 同一刻度（`entry_mono = T`） | 严格大于不成立 → 丢弃 |
| 墙钟回拨或前跳 | 闸门不读墙钟，不受影响；墙钟只进入展示用的 `observed_at` |
| 慢 stdin、长链式调用 | `entry_mono` 在读 stdin 之前已取得，等待时间不会让旧推送显得更新 |
| 延迟写入跨过两次变化：入口在 T1 之后，其后失效（`blocked`）再确认（`open`，T2 > 入口） | 写入时 `entry_mono < T2` → 丢弃；写入若落在两次之间的 `blocked` → 丢弃 |
| 入口在失效之前、写入在确认之后 | `entry_mono < T` → 丢弃 |
| 入口在失效之前、写入也在失效之前 | 放行；随后的失效会删除它 |
| 进程重启（App、helper 或 Claude Code） | 单调时钟是系统级的，不受进程重启影响 |
| 重启机器 | 新启动的捕获 `entry_boot ≠ capture_open_boot` → 丢弃；下一刷新周期持锁发现 `capture_gate = 'open'` 且标识不同，就以当前 `T'`、`B'` 重新发布（确认早在上一次启动就已提交，此时取的任何 `T'` 都晚于它）。重启后到下一周期之前的捕获被丢弃，最多延迟一个刷新间隔 |
| 平台无法取得启动标识或单调时钟（非 darwin/linux，或系统调用失败） | 无法发布 `open`，闸门保持 `blocked`，捕获在已知变化后一直被丢弃（关闭方向）；`open_initial` 不受影响 |
| 崩溃发生在 tx1 与 tx2 之间 | 闸门仍为 `blocked` 而 `confirmed_generation = context_generation`；下一刷新周期一开始就持锁执行 tx2（以届时的 `T` 发布），不需要再次重新确认 |
| 迁移前已打开的旧连接、旧二进制 | 见 A5.4 末条：旧表已删除，旧二进制被 schema 版本拒绝 |

### A5.6 当前额度可展示与状态栏恢复（R1-F2）

两个条件分开定义：

- **当前额度可展示**：Claude 存在窗口即可展示（按 C5、C8 规则，归属仍未确认）。失效删除全部旧窗口，此后每条写入都受世代或闸门守卫，所以现存窗口必然写于最近一次失效之后。`quota_context` 在 `invalidated_at` 非空时出 wire。**当前世代内第一次成功的 `CommitClaudeUsage` 清空 `invalidated_at` 与 `invalidated_reason`**，不论同周期身份是否成功。
- **允许状态栏恢复（重新确认）**：在周期 `c` 内，身份步骤观察到 `S(x, k)`，转换后的基线为 `subscription(x, k)`，且同一周期的 `CommitClaudeUsage` 以未变化的世代成功提交。`reconfirmEvidence = {cycleSeq, x}` 从身份步骤传给 usage 提交；提交事务（tx1）重新读身份行，核对 `baseline_kind = subscription`、`baseline_correlation = x`、`success_cycle_seq = cycleSeq` 与世代，满足时写 `confirmed_generation = context_generation` 并清空 `invalidated_*`。tx1 提交后按 A5.5 发布闸门（tx2）。
- **导致失效的那一轮可以参与重新确认**：失效与 usage 都在该轮内，且 usage 晚于失效提交才读世代（A1.2）。
- **中间态可见**（修正 Round 1 的说法）：失效提交与 usage 提交是两个事务，中间可能隔着 `/usage` 的最多 10 秒。期间任何一致读者（菜单栏、`agentdeck --format json quota`、Widget 的下一次写入）都可能读到已提交的失效，即"已失效"行与空窗口。同一周期内两侧都成功时，这个中间态最多持续一次 usage 探测的时长。
- 下列情况不会重新确认：只有 usage 的周期、只有身份成功的周期、任何状态栏推送、身份为 `signed_out` 或 `non_subscription` 的周期（这些周期里成功的 usage 可以展示，但闸门保持 `blocked`）。
- **必然完成**：见 A1.2 的联合机会与不饿死论证。身份或 usage 一侧若永远不能成功，闸门按 D2 一直保持 `blocked`，usage 仍可通过自身调度和手动刷新展示。
- usage 失败不清除任何失效字段，也不影响闸门。

### A5.7 告警投递与失效的跨进程顺序（R2-F2）

**不变式**。同一个受管 stateRoot 中，App 持锁复核 Claude 提醒世代并调用通知提交 API。使该提醒失效的 helper 事务持有同一把锁。失效先提交时，App 会丢弃旧世代提醒。App 在复核后被挂起时仍持锁，竞争者在截止前返回忙碌。

**锁和世代文件**。

- 投递锁使用物理根内的 `quota-delivery.lock`。根描述符以 `O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC` 打开，用于验证物理身份及相对路径操作。Go 与 Swift 用 `openat`、`O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC` 打开守卫，限时尝试 `flock(LOCK_EX | LOCK_NB)`。持锁后复核当前根路径与原描述符身份、守卫为有效用户的 0600 普通文件且只有一个链接、守卫路径与描述符指向同一 inode、根扩展属性与该 inode 的身份相同。关闭描述符或进程退出时内核释放锁。CJSI 继续使用物理目录自身的 flock 来排空 SQLite 连接。
- 物理根的 `user.agentdeck.quota-delivery-lock` 扩展属性记录守卫的 `dev` 和 `ino`，各 8 字节大端。helper 与 App 继续持有既有所有权会话。App 额外匹配候选的 `state_root_id`，其值仍来自物理目录。备份与空目标 Restore 只携带各自已允许的数据，目标按自己的物理身份初始化守卫。该协议要求支持 flock、硬链接、创建时一次性写入扩展属性和目录同步的本地文件系统。受管操作保留已建立的绑定。外部操作若要同时清理绑定、两个守卫名及镜像，须先独占所有权并让旧持有者退出。并发清除这些全部标志会与首次使用无法区分，超出正常运行的信任边界。
- 首次允许 Claude 刷新时，helper 在 ownership session 和 `quota-refresh.lock` 内初始化守卫。仅当根上尚无绑定、正式守卫及世代镜像时，创建或复核固定的 `.quota-delivery.lock.pending`，要求为有效用户的 0600 普通文件、一个链接。先同步该 inode 和根目录，再用 `XATTR_CREATE` 一次绑定暂存 inode 并同步根目录。之后用 `linkat` 将这个已绑定 inode 发布到正式名，复核两名指向同一 inode，移除暂存名并同步根目录。整个过程和随后获取守卫共用首次 `G` 的单调截止。每个阶段开始前核对截止，超时后在后续周期从已验证步骤续作。
- 发布中断时，helper 只继续发布绑定所指的同一暂存 inode。只要正式名缺失，每次续作须先重新同步根目录，成功后才能 `linkat`。这包括上一进程在写入绑定后、同步前退出的情形。同步失败时保留待发布状态和原绑定。两名并存时，只有路径均匹配绑定、总链接数为 2 才能清理暂存名。已绑定文件缺失且没有合法暂存 inode、正式名被替换、守卫或镜像已有而绑定缺失、校验失败或文件系统不支持上述语义时，守卫不可用。普通周期保留现有绑定并返回 `ErrQuotaDeliveryUnavailable`。只有取得独占所有权且确认旧守卫持有者退出后，才能显式恢复。App 以只读方式核验守卫及镜像。
- `quota-delivery.epoch` 为 16 字节魔数 `AGENTDECK-QDG-1\n` 和 8 字节大端世代。持投递守卫后用 `openat` 与 `O_NOFOLLOW` 打开并检查普通文件、有效用户、0600 和完整记录。helper 在守卫内创建、修复和写入镜像。App 在守卫或镜像不可用时跳过 Claude 提交。

**锁次序**。

| 进程 | 次序 | 投递守卫内操作 |
| --- | --- | --- |
| helper（刷新周期） | 已获所有权会话 → `quota-refresh.lock` → 投递守卫（`G ≤ 1 s`）→ SQLite 写锁（`W ≤ 1 s`） | 对账，或按下面的顺序写世代并提交 Claude 身份转换 |
| App | 既有所有权会话 → 投递守卫（`G_app ≤ 1 s`） | 验证目录、绑定和世代，调用同步入口，随后释放 |
| 状态栏捕获、`quota-alerts ack`、只读投影 | 沿用各自的所有权与数据库访问次序 | 无需获取投递守卫 |

App 持守卫期间只做本地校验并发起提交，解锁后等待回调。helper 在开启受保护的 SQLite 写事务前获取守卫。双方在各自的截止时间停止授权取锁。

**helper 路径**。

1. Claude 满足门禁后，在刷新周期中持投递守卫对账。读数据库当前已提交世代，将缺失、无效或不匹配的镜像写成该值。重新对账以数据库提交结果为依据。
2. Claude 身份转换前再次获取投递守卫。守卫忙或不可用时，放弃本周期的 Claude 身份提交、尝试记录和 usage。Codex 使用自身的并行路径。
3. 如果转换会失效，在投递守卫内先将完整镜像写成 `g + 1` 并成功同步，再提交 SQLite 事务。镜像写入或同步失败时回滚事务。事务提交报错或结果不明时保留镜像原状。后续周期按第 1 步核对已提交世代，避免把可能已提交的新世代回退。进程在同步之后、提交之前中断时，镜像暂时领先，世代 `g` 的提醒会被拒绝。
4. `desktop quota-refresh` 沿用一次性信封和退出行为。Claude 提醒附带生成时的世代，结果附带 `state_root_id`。守卫不可用时以现有 `quota_alerts_failed` warning 表示告警部分失败，同时保留有效的 Codex 提醒。App 对缺证的 Claude 候选跳过提交。

**App 路径**。

1. 在 `DesktopRefreshCoordinator.performQuotaOperation` 中沿用 refresh、deliver、ack 和 quota 读取次序。按 ID 去重，并读取现有通知权限。未授权时结束投递。Codex 沿用原有的提交条件。
2. App 为 Claude 限时获取投递守卫。核对目录及绑定、只读镜像和每条提醒的世代。只为通过核验的提醒调用 `UNUserNotificationCenter.add(_:withCompletionHandler:)`。从复核到同步调用返回保持锁。完成处理器只记录结果。
3. 解锁后等候回调至 `C ≤ 5 s`。用独立截止唤醒等待方，迟到回调不再改变本批 ACK。现有 `quota-alerts ack` 只收到按时成功的 ID，并在数据库中检查世代。重复输入在批内只提交一次。回调未获知而跨批重试时使用相同 ID，系统可能再次显示提醒。

**竞争与中断**。App 被挂起时，helper 在截止后放弃取得同一守卫。App 解锁或退出后，下一个周期可提交失效。镜像可在受管同步中替换，属于另一 inode。守卫身份变化或根被替换时，App 丢弃本批。系统同步提交调用停滞时继续持锁，竞争的刷新按自己的截止返回忙碌。实现验收使用注入暂停和未回调的假通知服务。

**通知提交验收边界**。按 2026-11-18 用户确认，以 Claude 失效事务提交和 AgentDeck 发起 `UNUserNotificationCenter.add(_:withCompletionHandler:)` 的顺序判定。失效提交之后，旧世代提醒的调用计数必须为零。先前提交的请求可能随后返回受理结果或显示横幅。回调和显示时间单独记录为 macOS 观察。

## A6 隐私（R1-F7、R1-F8）

### A6.1 邮箱

只在 adapter 进程内校验与脱敏，原值不离开 adapter 函数。校验：先做 NFC（`golang.org/x/text/unicode/norm`，已是依赖）；不得含空白、Cc、Cf、Zl、Zp 字符；恰好一个 `@`；本地部分 1–64 个扩展字素簇（`github.com/clipperhouse/uax29/v2`，已是依赖）；域名 1–253 个字符，含 `.`，不以 `.` 开头或结尾。脱敏：本地部分 ≥4 个字素时为首字素 + `***` + 末字素；2–3 个时为首字素 + `***`；1 个时为 `***`。域名转小写后保留。无效时不显示，结果记 `incomparable`。

### A6.2 显示名称（组织名、工作区名、官方套餐展示名）

依次处理：NFC；删除 Cc、Cf（含 U+061C、U+200B–U+200F、U+202A–U+202E、U+2060–U+2069、U+FEFF）、Co、Cs 与非字符码点；把连续 Unicode White_Space 合并为一个 U+0020 并去掉首尾空白；为空则为 null；截断到 64 个扩展字素簇（不拆簇、不加省略号，界面负责视觉截断）；最后执行 A6.3。只作用于展示副本，原始套餐诊断按 A4 处理。

### A6.3 禁止值筛查（adapter 边界，持久化之前，R1-F7）

**禁止值集合** F：同一进程从供应商收到的原始邮箱及其本地部分、组织 ID、账号 ID、共同键、rateLimits `accountId`。集合只存在于 adapter 内存中。

**筛查函数** `screen(V)`：对 NFC 与 Unicode case fold 后的 V，以下任一成立即判为不安全：

- V 与 F 中任一值完全相等，**不设长度下限**（例如五个字符的组织名等于组织 ID 也会被拒绝）；
- V 包含 F 中长度 ≥4 个字素的任一值，或长度 ≥3 个字素的邮箱本地部分；
- V 匹配邮箱形状 `[^\s@]+@[^\s@]+\.[^\s@]+`。

**适用范围：每一个会离开 adapter 的供应商字符串，新旧路径同一规则**

| 字段 | 不安全时的处理 |
| --- | --- |
| 组织名、工作区名、官方展示名 | 字段为 null；`organization_kind` 保留，界面只省略名称 |
| Codex `planType`（旧 `plan` 与 `plan_display` 共用） | 按 A4：`plan = null`、`plan_reason = not_reported`，`plan_display` 为 `missing`/`unsafe_value` |
| Claude `planTier`、`subscriptionType` | 按 A4 为 `missing`/`unsafe_value` |
| Codex `limitId`（成为 `window_key`、wire `key`，并进入告警 ID 与通知标识） | 整个 rateLimits 结果按 `parse_failed` 处理，不写任何窗口。窗口键是隔离与去重的连接键，不能静默改名 |
| Codex `limitName`（`label`）、重置额度的 `title`/`status` | 该字段按 `not_reported` 处理（`label` 为空，credit 条目丢弃该字段） |

安全原值在所有路径上保持原样；不安全值不进入数据库、wire、CLI 文本、App Group、告警 ID、日志或错误文本。Claude 的两条额度通道只产生固定窗口键与数值，不含供应商字符串，不需要筛查。

### A6.4 错误与外部文本

- 身份 adapter 只返回带类别的类型化错误，`Error()` 是常量文本，不包含响应、stdout、stderr 或路径。
- 共享 Codex 传输：`CodexFailureServerError` 不再格式化 `env.Error` 正文，最多保留整数 `code`。这收紧了现有 rateLimits 路径；该文本当前不持久化也不显示，没有消费者变化。
- Claude prose：`ParseClaudeProse` 返回 `ProseParseError{Detail}`，文本只含细分类别，去掉 `%q` 行内容与时区名。
- JSON 解码错误一律包装为常量类别，不使用 `%v` 拼接底层错误。
- 日志与 perf trace 只记录类别与计数。

### A6.5 关联值（仅 Claude）

`HMAC-SHA256(secret, "agentdeck-quota-identity/v1\0claude\0" + kind + "\0" + lower(email) + "\0" + ("org:"+id | "org:none"))`；组织未知时不计算（观察为 `incomparable`）。`key_id = HMAC-SHA256(secret, "key-id/v1")` 的前 16 位 hex。不使用无盐哈希，不把脱敏邮箱当键。关联值与 key id 只存于 `quota_identities` 的基线列，不出 wire、日志或导出。

### A6.6 本机秘密（R1-F8）

- **路径与格式**：`<stateRoot>/quota-identity.key`；内容 = 魔数 `AGENTDECK-QUOTA-IDENTITY-KEY\n` + 版本字节 `1` + 32 字节随机数。
- **创建**（参照 `credentialvault.createSeedExclusive`）：`GuardStateRoot`、`EnsureStateRoot` → 生成随机数 → 在 stateRoot 内 `CreateTemp(".quota-identity.key-*")` → `Chmod 0600` → 写入完整内容 → `Sync` → `Close` → `os.Link(临时文件, 最终路径)` → 同步目录 → 删除临时文件。最终路径只会以完整、已落盘的文件出现，读者看不到空文件或短文件。
- **并发首次创建**：`Link` 返回 `EEXIST` 时放弃自己的随机数，同步目录后读取胜出者的文件。
- **读取校验在同一描述符上完成**：先打开 stateRoot 目录描述符（与 `serving_cache` 相同的 `openServingDirectory` 做法），再 `openat(dirfd, "quota-identity.key", O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)`；在**这个描述符**上 `fstat`，要求普通文件、权限恰为 0600、`st_uid` 为当前有效用户、`st_size` 恰为魔数加 33 字节；随后从同一描述符读取全部内容，核对魔数与版本，并确认读到的字节数等于 `st_size`。路径级的 `Lstat` 不作为判定依据，打开和校验之间换成其他对象（符号链接、FIFO、他人的文件）无法通过（R1-F8）。`O_NOFOLLOW` 遇到符号链接时打开失败，按"校验失败"处理。
- **中断重试**：崩溃只会留下 `.quota-identity.key-*` 临时文件，读取路径从不读它；创建路径顺带删除超过 10 分钟的同前缀临时文件。
- **异常处理**：文件不存在 → 创建新秘密，新 key id 按 A5.3 触发 `identity_key_reset`（仅当基线为 `subscription`）。文件存在但校验失败（权限、属主、长度、魔数）→ 拒绝使用、不重建；本周期 Claude 身份记 `incomparable`，不触发失效，基线不变。
- 秘密只在刷新周期内加载；遵循非安装构建的状态所有权规则，测试一律使用临时 stateRoot。不进入备份，不跨机器复用（A8）。

### A6.7 禁止出现的位置

| 数据 | 数据库 | wire / `--format json quota` | App Group | 日志、perf trace、通知、告警、一般导出、错误文本 | 便携备份 |
| --- | --- | --- | --- | --- | --- |
| 原始邮箱、组织 ID、账号 ID、共同键 | 否 | 否 | 否 | 否 | 否 |
| 脱敏邮箱 | 是 | 是 | 是（D1） | 否 | 否 |
| 组织名、工作区名 | 是 | 是 | 否 | 否 | 否 |
| 关联值、key id、秘密 | 关联值与 key id 是；秘密只在文件中 | 否 | 否 | 否 | 否 |
| 原始 stdout、stderr、JSON-RPC 错误正文 | 否 | 否 | 否 | 否 | 否 |

## A7 存储与 schema（R1-F9，R1-F1）

需要一次迁移，编号由主控在实际 main 上分配（当前 main 为 `CurrentSchemaVersion = 35`，下文记作 `N`）。迁移 `N` 使用 `apply` 函数，在同一事务内依次执行：

1. 创建单行表 `quota_cycle(singleton INTEGER PRIMARY KEY CHECK(singleton=1), seq INTEGER NOT NULL DEFAULT 0, revalidate_seq INTEGER NOT NULL DEFAULT 0)` 并插入初值行。
2. 创建单行表 `quota_projection_revision(singleton …, revision INTEGER NOT NULL DEFAULT 0)` 并插入初值行（A10.3 的单调安装键）。
3. **换代 quota 表**：
   - 创建 `quota_windows_v2`：原 `quota_windows` 全部列，加 `context_generation INTEGER NOT NULL DEFAULT 0`、`cycle_seq INTEGER NOT NULL DEFAULT 0`，主键不变；
   - 创建 `quota_envelopes_v2`：原 `quota_envelopes` 全部列（含 v28 的 `backoff_until`、v30 的 `failure_observed_at`），加 `failure_detail`、`cycle_seq`、`context_generation`、`invalidated_at`、`invalidated_reason`、`capture_gate TEXT NOT NULL DEFAULT 'open_initial' CHECK(capture_gate IN ('open_initial','blocked','open'))`、`confirmed_generation INTEGER NOT NULL DEFAULT -1`、`capture_open_boot TEXT NOT NULL DEFAULT ''`、`capture_open_mono INTEGER NOT NULL DEFAULT 0`；
   - 创建 `quota_alert_notices_v2`：原列加 `context_generation INTEGER NOT NULL DEFAULT 0`，主键为原主键加 `context_generation`；
   - 用 `INSERT … SELECT` 把三张旧表的行复制过去（世代 0、序号 0、闸门 `open_initial`）；
   - `DROP TABLE` 三张旧表。旧表上的 serving revision 触发器随表一起删除。
4. 创建 `quota_identities`，`client` 为主键，列为 A5.1 的四组字段，全部 `NOT NULL` 并带空串或 0 默认值。
5. 调用 `installServingRevisionTriggers(ctx, tx, []string{"quota_envelopes_v2", "quota_identities"})`，与现状对 `quota_envelopes` 的处理保持一致；`quota_windows_v2` 与现状的 `quota_windows` 一样不加。**不修改**迁移 31 的 `initializeServingStorage` 表清单：全新库在 v31 时这些表还不存在，已在 v35 的库也不会重跑 v31。
6. 为 `quota_windows_v2`、`quota_envelopes_v2`、`quota_identities`、`settings`、`provider_selections`、`usage_session_observations` 安装 `AFTER INSERT/UPDATE/DELETE` 触发器，把 `quota_projection_revision.revision` 加 1。这些就是订阅投影的全部输入（quota 数据、读取开关与客户端、记录 provider、观察 provider）。
7. 安装 A5.2 的重新验证触发器（`provider_selections`，以及 `settings` 中 `quota.probe`、`quota.probe.clients` 的实际变化）。
8. 投递守卫使用独立锁文件和物理根的 inode 绑定（A5.7）。helper 在首个符合 Claude 门禁的周期初始化守卫和世代镜像 `quota-delivery.epoch`。

**为什么换代而不是加列（R1-F1）**：只给旧表加列时，迁移前已打开的旧连接与旧代码仍按旧列清单写入成功，新列取默认值，相当于借保留列继续以旧语义写入。合成探针（A13.2）在同一个 WAL 库上证实：旧连接预热过的 `INSERT … ON CONFLICT … DO UPDATE` 在旧表被删除后报 `no such table: quota_windows`；对照组只加列时，同一语句写入成功且新列为默认值。旧表名今后不再复用。

现有数据升级后：Claude envelope 世代为 0、闸门为 `open_initial`，没有身份行，即基线 `none`；`quota_cycle.seq = 0`。

**只读投影随库版本变化**：`quota.Store` 携带只读打开时读到的 schema 版本，`N` 之前读旧表名，所有 SELECT 按版本投影缺失列：

| 库版本 | 处理 |
| --- | --- |
| < 27 | 维持现状：读取关闭默认值 |
| 27 | `backoff_until` 以 `''` 代替；`failure_observed_at` 视为**未知**（见下） |
| 28–29 | `failure_observed_at` 视为未知（修复当前 `Envelope` 在 v27–29 只读库上查询不存在列而失败的问题） |
| 30–(N−1) | 新列按默认值；`identity`、`quota_context`、`failure_detail` 为 null；客户端级 `plan_display` 仍由 `plan` 计算并经 A6.3 的 token 规则 |
| ≥ N | 完整 |

**未知顺序按保守方向（R1-F9）**：`failure_observed_at` 未知时，不能判断状态栏窗口与 prose 失败谁更新。`BuildSubscription` 与 `DueAlerts` 都把这种失败视为"晚于任何窗口"：不允许状态栏窗口覆盖它，`parse_failed` 时不展示数字、不产生提醒，`probe_failed` 时按真实年龄显示窗口并带失败原因。失败时刻展示 `failure_at`（非空时），否则为 null。不把未知补成空串再参与比较。

写路径总是经 `store.Open` 迁移到当前版本；旧二进制打开 ≥N 的库会被 `SchemaAhead` 拒绝。

**连带修改**：
- `CurrentSchemaVersion`；`quota.MinSchemaVersion` 的含义改为按版本选择表名；
- `compatibleServingCoreSchema` 增加 `N` 分支（31…N−1 兼容，因为订阅段在读取时重新投影，A10.3）；
- `backup.Restore` 与 A8 的删除清单改用 `_v2` 表名；迁移 29 的 `ensureQuotaAlertNotices` 形状检查只在 N 之前生效；
- Swift 兼容常量与原生上限；四份金标与 schema 测试。具体清单在实现任务中按 schema 升级惯例逐项核对。

**测试**：全新库；带 quota 行的 v35 升级库（行数与值完整复制，旧表不存在）；以 v27、v28、v29、v30、v35 只读打开且 quota 读取开启、两客户端都有行，其中 v27–29 构造"状态栏窗口 + 未知时刻的 `parse_failed`"并断言不展示数字、无提醒；两种路径下 `quota_envelopes_v2` 与 `quota_identities` 的 serving 触发器、投影 revision 触发器、重新验证触发器都存在；迁移 31 的表清单不变；A13.2 的旧连接探针。

## A8 备份、导出与恢复（R1-F6，R2-F3）

**创建备份**：

1. 在线备份得到私有快照（现状）。
2. 在快照上以 `PRAGMA secure_delete=ON` 开 `BEGIN IMMEDIATE`：清空现有非便携 origin 表与 `quota_identities`；把 `quota_envelopes_v2` 的世代、失效、闸门列与 `failure_detail` 重置为默认值，`quota_cycle` 归零；提交并 `wal_checkpoint(TRUNCATE)`。
3. 用 `VACUUM INTO '<staging>/core.compact.sqlite3'` 生成压实副本，**以它作为归档条目**并在其上计算摘要。原快照随 staging 目录删除。
4. 对压实副本校验：`integrity_check` 为 ok；`PRAGMA freelist_count = 0`；`quota_identities` 行数为 0；无 WAL 旁路文件。任一不满足则备份失败。

这样处理的原因：在线备份逐页复制源库，包括空闲页和页内未用区域；快照上的 `secure_delete` 只清零本次删除释放的空间，清不掉源库早已留下的旧值。`VACUUM INTO` 只从逻辑内容重建新文件。秘密文件不在备份条目中（条目仍只有 core、sessions、credentials、manifest，不新增条目）。

**Restore**：

- **空目标契约不变（R2-F3）**：`reserveEmptyTarget` 照旧拒绝任何非空目标目录，不为 `quota-identity.key` 或其他文件开例外。目标在 Restore 时必然没有身份秘密；目标机器在第一次刷新周期需要时创建自己的秘密（A6.6）。凭据归档范围不变：`credentials` 条目照旧由目标 vault 重新封存，`credential.key` 与 `quota-identity.key` 都不进入档案。Round 1 稿中"目标根已有的秘密保留"一句描述的是不可达状态，已删除。
- **逻辑清理**：恢复出的 core 先经 `store.Open` 迁移到 N（N 之前的档案在此完成换代），然后删除 `quota_windows_v2`、`quota_envelopes_v2`、`quota_alert_notices_v2`、`quota_identities` 的全部行，并把 `quota_cycle` 归零。便携档案排除世代镜像。Restore 在空目标中恢复，首个符合 Claude 门禁的刷新周期依据数据库世代 0 建立镜像。
- **字节清理按档案实际 schema 判定，不看内容**：`validateCoreSnapshot` 已核对档案 core 的实际 `schema_metadata.version` 等于 manifest 声明。
  - 实际版本 ≥ N，或 `sqlite_master` 中存在 `quota_identities` 表：上述删除在 `PRAGMA secure_delete=ON` 下执行，随后在 Restore 返回前对目标 core **无条件**执行 `VACUUM`。这覆盖"身份表已空、但空闲页或页内未用区域仍有哨兵"的外部档案，以及档案自身删除留下的旧页。
  - 实际版本 < N 且没有该表：这样的 schema 从未有过身份表或身份列，档案中不可能有 AgentDeck 身份数据，只做逻辑删除。迁移换代留下的空闲页只含源机器的旧 quota 窗口与 envelope，与现状 Restore 对这些表的处理（逻辑删除）相同。
- **净化边界**：任何档案恢复后，目标在逻辑上都没有身份数据；目标文件字节中也不会残留 AgentDeck 身份数据。前者由删除保证；后者对 ≥N 档案由 VACUUM 保证，对 <N 档案由"该 schema 不含身份"保证。
- VACUUM 的耗时与 core 大小成正比；Restore 是用户主动发起的低频操作，在现有进度与回滚范围内执行，失败按现有逻辑回滚已创建的目标文件。

一般导出不带身份。`agentdeck --format json quota` 是单独批准的即时投影，不是便携导出。

**测试**：
- 备份：以互不相同的哨兵作为脱敏邮箱、组织名、关联值，先写入身份行，再用另一组值更新，使旧值落入空闲空间，然后创建备份。断言归档 core 条目字节中不出现任何旧哨兵或新哨兵；`freelist_count = 0`；逻辑行为 0。
- Restore 外部档案：构造一个 ≥N 档案，身份表有行后以非 secure_delete 方式删除，使哨兵只留在空闲页；再构造一个哨兵只留在页内未用区域的档案。断言 Restore 后目标文件字节无哨兵、逻辑行为 0。
- Restore 旧档案：v35 档案恢复成功、换代完成、quota 行逻辑上被清空、未执行 VACUUM。
- 空目标：目标目录只含一个 `quota-identity.key` 时 Restore 返回 `ErrTargetNotEmpty` 且不改动目录；空目标恢复后不存在秘密文件，直到第一次刷新周期。

不以全档扫描 `@` 作为证据。

## A9 usage 诊断细分与历史收口

`ParseClaudeProse` 的接受语法不变。失败细分：

- `missing_line`：缺少任一约定行前缀；
- `shape_invalid`：正则不匹配、百分比非法或标签无法识别；
- `timezone_unresolved`：`LoadLocation` 失败或重置时刻无法解析。

细分在同一个守卫事务中与 `failure = parse_failed` 一起写入 `failure_detail`；其他失败写 `''`。成功同时清除 `failure` 与 `failure_detail`。Codex 的畸形输出仍为 `parse_failed`，`failure_detail` 为空。额外的模型行（如 `Current week (<model>)`）本来就因前缀不匹配被忽略，补回归测试证明它从未进入窗口。

历史坏 stdout 不可得：只做界定记录（已知时间、构建与调用上下文，以及未知部分），不修改 parser 去适配想象中的输出（A13.3）。

## A10 wire 与 CLI

### A10.1 增量字段（WireVersion 不变）

```json
"plan_display": {"state":"known","name":"ChatGPT Pro 100","short_name":"Pro 100","basis":"mapping","source":"codex_rate_limits","raw":[{"field":"planType","value":"prolite"}],"reason":null},
"failure_detail": null,
"quota_context": {"invalidated_at":"…","reason":"context_changed"},
"identity": {
  "state":"identified","unknown_reason":null,"freshness":"current","current_until":"…",
  "observed_at":"…","attempted_at":"…","failure":null,"failure_at":null,
  "source":"claude_auth_status",
  "email_masked":"a***e@example.test",
  "organization_kind":"organization","organization_name":"…",
  "plan_display":{"state":"known","name":"Claude Pro","short_name":"Pro","basis":"mapping","source":"claude_auth_status","raw":[{"field":"planTier","value":"pro"},{"field":"subscriptionType","value":"pro"}],"reason":null},
  "quota_association":null
}
```

| 字段 | 取值 | null / 缺失的含义 |
| --- | --- | --- |
| `projection`（订阅段级，与 `clients` 同层） | `{core_epoch, revision, boot, mono}` | 缺失：旧 producer，消费者视为最旧（A11）。不展示，只用于单调安装 |
| `plan_display`（客户端级） | A4 | 缺失：旧 producer；null：Claude，或门禁关闭 |
| `failure_detail` | `missing_line` / `shape_invalid` / `timezone_unresolved` | null：`failure` 不是 Claude prose 的 `parse_failed`，或旧库 |
| `quota_context` | `{invalidated_at, reason}`；`reason` ∈ `context_changed` / `signed_out` / `non_subscription` / `identity_key_reset` | null：未失效，或失效后已有新 usage 提交（A5.6），或门禁关闭 |
| `identity` | 下列字段 | null：门禁关闭或库版本低于 N |
| `identity.state` | `identified` / `signed_out` / `non_subscription` / `unknown` | 不为 null |
| `identity.unknown_reason` | `never_probed` / `probe_failed` / `parse_failed` / `incomparable` | 只在 `state = unknown` 时非空 |
| `identity.freshness` | `current` / `last_read` | `state = unknown` 时为 `last_read` |
| `identity.current_until` | 时刻 | 仅 `current` 时非空 |
| `identity.observed_at` | 上次成功时刻 | 从未成功时为 null |
| `identity.attempted_at` | 最近一次真实尝试时刻 | 从未尝试时为 null |
| `identity.failure`、`failure_at` | 最近一次尝试失败的类别与时刻 | 最近一次尝试成功时为 null |
| `identity.source` | `codex_account_read` / `claude_auth_status` | 不为 null |
| `identity.email_masked` | 脱敏邮箱 | `success_state ≠ identified` 时为 null |
| `identity.organization_kind` | `organization` / `workspace` / `personal` / `unknown` | `success_state ≠ identified` 时为 null |
| `identity.organization_name` | 安全显示名 | 无名称、被 A6.3 拒绝，或不是 identified 时为 null |
| `identity.plan_display` | A4 | `success_state ≠ identified` 时为 null |
| `identity.quota_association` | `confirmed` / `unconfirmed` | Claude 恒为 null；Codex 非 identified 时为 null |

身份来源、上次成功、最近一次失败三者因此能同时表达：例如 `state=unknown`、`unknown_reason=probe_failed`、`observed_at` 为 3 小时前、`email_masked` 非空，即菜单栏的"旧邮箱弱化 + 上次读取 · 3小时前 · 身份读取失败"。

### A10.2 兼容与不出 wire

- 既有字段的取值集合不扩展：旧 Swift 的 `DesktopQuotaReasonV1` 遇到未知值会抛错，整个 subscription 段解码失败会连带整个快照失败。新枚举一律放在新字段里。
- 不出 wire：完整邮箱、组织 ID、`accountId`、关联值、key id、原始 stdout、`context_generation`、`confirmed_generation`、基线列、周期序号与 `association_cycle_seq`、捕获闸门列。`projection` 中的 `boot` 是启动标识，不是账号或身份数据，只用于同一次启动内的比较。
- **absent 与 null 是两种契约**：Go 侧对新增字段一律输出键（值可为 null），从不省略；只有旧 producer 才会缺键。Swift 必须按 A11 区分。

### A10.3 一致读取与缓存恢复（R1-F3）

- `BuildSubscription` 在**一个只读事务**（`BeginTx(ReadOnly)`，同一连接）内读取全部客户端的窗口、envelope、身份行、设置、provider 选择与 `quota_projection_revision.revision`；`DueAlerts` 同样。WAL 快照隔离保证不会读到"失效前的窗口加失效后的 envelope"这类组合。
- **投影键**：同一事务内第一次读取之后，取一次单调时刻 `mono` 与启动标识 `boot`，并以同一时刻换算出投影用的 `now`（所有依赖时间的字段，如 `stale`、`freshness`、`current_until`，都用这个 `now`）。`projection = {core_epoch: derived_snapshot_generation.epoch, revision, boot, mono}`。`revision` 排序数据库输入，`mono` 只在 `revision` 相等时排序依赖时间的字段。
- **订阅段不从服务缓存发布**：`ReadServingSnapshot` 与 `ReadOrBootstrapServingSnapshot` 返回发布物之前，用当前数据库以只读方式重新执行 `BuildSubscription`，替换其中的订阅段，从不返回缓存里的订阅段；重新投影失败时，订阅段替换为 `available: false`。发布物仍写入订阅段以保持格式兼容，但读者不采用。Swift 启动恢复（`installServingPublication`）因此只会安装恢复时刻的数据。serving revision 变化只标 `stale` 的现有行为不变，它不再关系到订阅数据的正确性。
- App 内存中的订阅段只按 A11 的单调规则替换；每次实际替换后 App Group 快照随之重写。

### A10.4 CLI

`agentdeck quota` 文本输出与 golden 不变；`agentdeck --format json quota` 带上述新字段（身份为批准的即时投影）。

## A11 Swift 消费（R1-F3、R1-F12）

- 新增 `DesktopQuotaIdentityV1`、`DesktopPlanDisplayV1`、`DesktopQuotaContextV1`、`DesktopQuotaProjectionV1`；新枚举宽容解码，未知值映射为 `.unrecognized`，按 unknown 或 missing 渲染。
- **三态解码（R1-F12）**：新增的可空字段一律解码为 `enum WireField<T> { case absent, null, value(T) }`：先 `container.contains(key)`，键缺失为 `.absent`；键存在时 `decodeNil(forKey:)` 为真则 `.null`，否则解码出值。不得用 `decodeIfPresent`，它把缺键与 null 合并。A4 的回退表只对 `.absent` 回退到旧 `plan`。App Group 编码同样保留三态：`.absent` 不写键，`.null` 写 `null`。
- 菜单栏与 Widget 的客户端名称统一用 `DesktopPresentationModelV1.clientDisplayLabel`，替换现有的 `client.client.capitalized`。
- **App Group 写入允许列表**：写入时投影为精简结构，未列出的字段不写。

| 对象 | 写入 App Group | Widget 读取 |
| --- | --- | --- |
| 客户端级 `plan_display` | `state`、`short_name` | large：`short_name`（A4 回退规则） |
| `quota_context` | `reason` | 三个尺寸：非空即该端无可用窗口 |
| `identity` | `state`、`freshness`、`current_until`、`email_masked`；仅 Codex 另写 `quota_association` | 仅 large |
| 不写入 | `organization_kind`、`organization_name`、`identity.plan_display`、Claude 的 `quota_association`（恒为 null，不写键）、`identity.observed_at`、`attempted_at`、`failure*`、`source` | — |

- **Codex 关联提示的最小决定（Widget UX §7）**：App Group 只为 Codex 写入既有两态枚举 `quota_association`，取值 `confirmed` 或 `unconfirmed`，按 A10 原样复制，不新增取值。它不含账号、邮箱或组织信息，不改变 D1：只有 large 读取，且只在当前登录行可见时使用；small 与 medium 不读；组织资料仍不写入。Widget large 的 Codex 登录行仅当该值恰为 `confirmed` 时省略 `与额度关联未确认`；`unconfirmed`、显式 null、缺键（旧快照）或无法识别的值一律按未确认渲染，与 UX "快照缺少证明时按未确认渲染"一致。当前样本没有共同键，实际总是未确认。
- **Widget 显示邮箱的条件**：`state = identified`、`freshness = current`、`current_until` 可解析且 **entry 的 `date`** 早于它，三者同时满足；否则省略该行。视图只用 `entry.date` 判断，不调用 `Date()`。small 与 medium 不读取 `identity`。旧 App Group 文件缺这些字段时省略该行；`plan_display` 缺键时回退 `plan`。
- **到期 entry（R1-F3）**：`QuotaTimelineProvider.timeline` 现在只返回一个 entry，并在 3–5 分钟后刷新（`WidgetTimelinePolicy`）。改为：取 `refresh = refreshDate(...)`；若 large 配置下当前登录行可见，且 `now < current_until < refresh`，返回两个 entry——`[entry(date: now)，entry(date: current_until)]`，第二个用同一快照生成，因 `entry.date ≥ current_until` 不含邮箱，策略仍为 `.after(refresh)`。若 `current_until ≤ now`，唯一的 entry 不含邮箱。WidgetKit 因此在到期那一刻切换到不含邮箱的 entry，不会多显示数分钟。
- **订阅段单调安装（R1-F3）**：`DesktopRefreshCoordinator` 保存当前已安装订阅段的投影键。所有安装入口统一经过 `installSubscription(candidate, into:)`：`installServingPublication`（包括 `releasePublications` 释放的 held publication）、`publishStandaloneQuota`、完整刷新中的 quota 结果。
  - 比较规则：`core_epoch` 不同时，只接受与当前 serving header 同 epoch 的候选（epoch 变化已由现有身份失效逻辑清空旧数据）。同 epoch、同 `boot` 时，按 `(revision, mono)` 字典序，候选更大才替换。候选缺少 `projection`（旧 producer）时视为最小。
  - 候选较旧时，保留已安装的订阅段，把它拼入候选发布物的其余部分（usage、sessions 等照常安装）。
  - `publishStandaloneQuota` 不再因 `latestSnapshot != baseline` 丢弃结果，而是按同一规则合并进当前 `latestSnapshot`。
  - 因此"持有期间 quota 操作安装了新订阅段，释放时回装更早的 held publication"不会再让订阅段倒退。
- 中英文消费者使用 ux 文档中的既有 key；本稿不新增文案 key。

## A12 错误分类总表（R1-F10）

| 侧 | 失败分类（闭集） | 是否推进退避 | 是否影响另一侧 |
| --- | --- | --- | --- |
| usage | 旧原因闭集，加 `failure_detail` | 旧 C9：只有后台失败推进；联合机会中被拉入的后台失败也推进 | 否 |
| 身份 | `probe_failed`、`parse_failed`、`incomparable` | 是，手动与后台都推进 | 否 |
| Codex 进程级：启动、握手、应答前退出、截止、传输畸形 | 本周期到期且尚未成功的每一侧各记一次（畸形 → `parse_failed`，其余 → `probe_failed`） | 按各侧规则 | 只影响尚未成功的一侧 |
| Codex 应答后非零退出或被信号终止 | 已应答的各侧记 `probe_failed` | 按各侧规则 | — |
| `account/read` 子截止 | 身份 `probe_failed`；迟到响应丢弃且不解析 | 身份推进 | 否：rateLimits 继续使用自己的预算 |
| SQLite 写锁在 `W` 内拿不到 | 不是探测失败；该写入放弃，不写失败记录 | 无 | Claude 身份提交失败 → 本周期不启动 usage（A1.5） |
| 预算不足（A1.5 退场条件） | 不发起，不记尝试 | 无 | — |
| 调用者取消 | 杀进程；失败写入随取消的 context 通常不落库 | 无 | — |
| 世代、周期序号或闸门不一致 | 不是失败；写入丢弃 | 无 | — |
| 投递守卫在 `G` 内取不到 | 不是探测失败；本周期放弃 Claude 身份提交，不写尝试、不启动 usage | 无 | 失效推迟到守卫释放之后的周期 |
| App 侧取不到守卫、完成回调报错或超时 | 不是失败；不提交或不 ACK | 无 | 未 ACK 的提醒以后再次给出 |

## A13 验证策略（R1-F13 及各发现的测试落点）

### A13.1 夹具冻结与未知版本降级

- **Codex**：在隔离 `CODEX_HOME` 下运行 `codex app-server generate-json-schema` 生成 schema，脱敏后提交为夹具；夹具头记录 codex-cli 版本、生成时间与 schema 的 sha256。`account/read` 的键名、共同键是否存在、令牌刷新参数都以它为准。
- **Claude**：使用需求引用的 2.1.325 只读样本，脱敏后提交为夹具；夹具头记录 CLI 版本、命令参数、观察到的退出码（含未登录时的退出码）与采样时间。
- **未知版本按形状保守降级**：不探测版本号（不增加子进程）。未知字段忽略；必需字段缺失或类型不符 → `parse_failed`；`authMethod` 或账号类型为未知枚举 → `incomparable`；未被夹具证实的非零退出 → `probe_failed`。没有任何路径在未知形状下得出 `identified` 或触发失效。

### A13.2 自动化测试（隔离、确定性）

一律使用临时 HOME、`CODEX_HOME`、`CLAUDE_CONFIG_DIR`、stateRoot 与注入的假可执行文件；不运行真实 codex 或 claude，不接触认证存储；macOS 原生测试只走隔离脚本。时钟与单调时钟、启动标识都通过注入的 `Clock`/`BootSource` 提供；跨进程测试用假原命令阻塞在一个由测试控制的 FIFO 上来制造确定的交错，不依赖 sleep。

**L1（纯函数）**

- 脱敏边界：1、2、3、4 个字素，组合字符、emoji、NFC 与 NFD 输入，无效值。
- 显示名清洗：每类控制与双向字符、空白合并、64 字素截断不拆簇。
- A6.3 筛查：邮箱形状；本地部分；**五个字符的组织名等于组织 ID**；长度 3 的 ID 完全相等；长度 4 的 ID 作为子串。
- A4：全部判定分支，以及消费者三态回退表。
- 缺少秘密时无法由已知邮箱复现 HMAC。
- A5.3 转换表逐行：迁移初始态、重复退出幂等、key reset、中间 unknown 不重置基线。
- A5.2：按序号的状态推导；重新验证地板；回拨（负年龄）为 `last_read`；`current_until`。

**L2（数据库与进程）**

- **尝试次序**：同一墙钟刻度内先成功后失败 → `unknown(probe_failed)`；先失败后成功 → 成功；墙钟回拨 1 小时后的权威 `signed_out` 照常生效并触发失效；重放 `cycle_seq ≤ attempt_cycle_seq` 的写入被丢弃。
- **捕获闸门**，每项一个用例：
  - 入口在 tx1 之前、写入在 tx2 之后 → 丢弃；
  - 入口等于 `T` → 丢弃；
  - 入口晚于 `T` → 放行；
  - 入口在 `T1` 之后、写入跨过第二次失效与确认（`T2`）→ 丢弃；
  - 写入落在 `blocked` → 丢弃；
  - 注入不同的启动标识 → 丢弃，下一周期重新发布后放行；
  - 启动标识不可用 → 永远 `blocked`；
  - tx1 后模拟崩溃，下一周期执行 tx2；
  - 注入墙钟回拨与前跳，结果不变；
  - 慢 stdin（FIFO 在失效之后才写入）→ 按入口时刻判定；
  - 断言链式原命令先执行、stdout 逐字节一致（含捕获被丢弃时）；
  - 断言写入行的来源串为 `claude_status_line`。
- **旧写入者**：在同一个 WAL 库上，迁移前打开一个连接并预热旧的 `INSERT … ON CONFLICT` 语句；在另一个连接上执行迁移 N；断言旧连接写入报 `no such table` 且 `_v2` 表不变。对照：只加列的方案下同一语句写入成功（本稿起草时已用 Python 标准库 `sqlite3` 合成探针复现这两个结果）。另断言旧版本二进制打开 ≥N 的库得到 `SchemaAhead`。
- **联合机会（R2-F1）**：
  - usage 在退避中、身份结果为 `identified` → usage 不被拉入；
  - 身份结果为 `signed_out` 且 usage 在退避中 → usage 不运行；
  - 身份成功、usage 退避结束后第一个周期 → 两侧同周期运行并完成重新确认；
  - 两侧到期分钟错开 → 在 `interval + 1 小时` 内出现联合周期；
  - 导致失效的那一轮可完成重新确认；
  - 新 usage 提交后 `quota_context` 为 null；
  - 中间态：在身份提交与 usage 提交之间读取，可见 `quota_context` 非空且无窗口。
- **失效守卫**：在途旧 usage 的成功与失败都因世代不一致被丢弃；旧 ACK（`qa2.` 旧世代、`qa1.`）不补回账本。
- **投递守卫与失效（R2-F2，跨进程、可抢占）**。用真实 helper 初始化守卫、绑定和 `g = 0` 镜像。Go 测试辅助二进制按 A5.7 取得投递守卫、验证根和绑定、读取镜像，随后用 `raise(SIGSTOP)` 暂停在复核与提交之间。通过测试 FIFO 记录假提交及单调时刻。提醒世代为 0，逐步执行：
  1. 启动持锁方，等它停在 `SIGSTOP`（以 `waitpid(WUNTRACED)` 确认），此时它持有守卫且已复核 `epoch = 0`。
  2. 运行真实的 helper 刷新路径，假 `claude auth status` 返回新的可比较身份 `S(c')`，需要失效。断言：helper 在 `G` 内取不到守卫，放弃身份提交，按 A1.5 预算返回；数据库 `context_generation` 仍为 0，文件 `epoch` 仍为 0，假 `/usage` 未被调用，周期总耗时不超预算。
  3. `SIGCONT` 持锁方；它写出"提交调用"记录（单调时刻 `t_add`）后解锁退出。
  4. 再次运行 helper 刷新：取得守卫，先写 `epoch = 1`，再提交失效，记录提交后的单调时刻 `t_commit`。断言 `t_add < t_commit`。
  5. 再次启动持锁方处理同一条世代 0 的提醒：复核 `epoch = 1 ≠ 0`，零提交记录。
  6. 变体：
     - 第 1 步的持锁方改为 `SIGKILL`：第 2 步的 helper 能取得守卫并完成失效；
     - 第 4 步在文件写入之后、SQLite 提交之前令 helper 崩溃：文件领先，持锁方拒绝世代 0，下一周期对账写回数据库世代；
     - 持锁方挂起时删除并重建世代镜像，helper 在两个连续周期均竞争失败。持锁方释放后 helper 才能执行失效。
     - 持锁方启动执行 `exec` 的子进程。确认子进程完成 `exec` 并保持存活后，持锁方只关闭自身描述符。另一个进程能立即获取守卫，以验证 `O_CLOEXEC`。
- **独立守卫初始化及排空组合**。分别在暂存文件、`XATTR_CREATE` 成功但根目录尚未同步、绑定同步、发布正式名、清除暂存名和镜像同步后注入中断。验证续作只使用已绑定 inode。单独注入续作的根目录同步失败，断言正式名仍缺失且绑定保留。比较 Go 与 Swift 对根别名、根替换、错误 mode／属主／链接数、符号链接、无效绑定以及守卫丢失或被替换的判定。持旧 inode 时创建同名替代品，断言零失效提交、零 Claude 通知调用、绑定保留和既有 warning。验证 App 前后根目录及扩展属性相同。组合中对既有 v36 根先只读打开，关闭 Store 与池后保留 SQLite 连接，再验证实际 refresh 能取投递守卫，排空仍等待保留连接。释放所有连接后，根目录的排空租约与投递守卫持有者可同时存在。
- **SQLite 预算（R1-F10）**。用另一连接覆盖无争用、截止前释放、持续持锁、较短父截止、取消与迟到唤醒。核对截止后停止授权取锁、迟到取得的锁先回滚、事务体至多一次、连接原有 `busy_timeout` 恢复，以及 Claude 身份提交受阻时本周期零 usage。分别注入回滚和恢复失败，断言丢弃受影响的物理连接，另一连接可以立即获得写锁，后续池租约使用正常 `busy_timeout`。用受控时钟给每次写入注入 `W + C`，包括最后一次告警修剪回滚和完整的 `C` 清理等待。验证各阶段合计 37 s、refresh 锁等待到 9.4 s 截止、预算不足时返回 busy 且零进程、零刷新写入。分开记录真实时钟的放弃观测、SQLite 返回、清理和进程退出耗时。
- **Codex 假进程**：身份静默、身份迟到（子截止后到达且被丢弃）、身份畸形、握手前退出、应答后非零退出、调用者取消；rateLimits 拿到的剩余时间 ≥ 当前实现；致命事件下两侧都推进各自退避；`WaitDelay` 生效。
- 门禁关闭时进程计数为零；手动刷新不绕过身份退避。
- A2.1 关联降级矩阵（带共同键的假 schema），含两个周期墙钟相同、序号不同的情形。
- Codex `signed_out` 不失效。
- **隐私经旧路径（R1-F7）**：`planType` 等于 rateLimits `accountId` → 数据库 `plan` 列、wire `plan`、CLI 文本、`plan_display.raw` 都不含它，`plan_reason = not_reported`；`limitId` 含邮箱 → 不写任何窗口且告警 ID 中不出现；`limitName` 不安全 → `label` 为空；安全的 `prolite` 在全部路径上不变。
- **一致读取**：`BuildSubscription` 的两次读取之间注入失效提交，断言结果不混合；服务缓存在失效后读取，订阅段来自当前数据库。
- **兼容**：旧快照、旧数据库（A7 版本表，含 v27–29 未知顺序用例）、未知枚举；CLI golden 不变。
- **absent 与 null**：Go 侧每个新增字段在 JSON 中都以键出现，Codex 门禁关闭时 `plan_display` 为 null 而非缺键；Swift 侧缺键、显式 null、对象三种输入分别得到 `.absent`、`.null`、`.value`，回退只发生在 `.absent`；App Group 往返保留三态。

**L3**

- 迁移：全新库、v35 升级、只读旧库。
- A8 备份哨兵；Restore 外部档案、旧档案与空目标策略。
- 秘密文件：并发首次创建（两个进程同时创建只产生一个文件、双方得到同一 key id）；中断后重试（遗留临时文件不被读取）；权限、属主异常拒绝且不重建；打开后把路径换成符号链接或 FIFO 时，同一描述符的校验仍判定原对象或失败（`O_NOFOLLOW`）。
- `-race`；无凭据断言覆盖新文件。

**隐私哨兵**：为原始邮箱本地部分、域名、组织名、组织 ID、账号 ID、关联值、原始 stdout 尾部各设一个独立哨兵。按接收方分别断言：日志与 perf trace（捕获输出）、wire JSON、数据库（逻辑行与文件字节；只允许脱敏邮箱与组织名的批准形态）、错误文本（`Error()` 串）、App Group 编码结果（只允许脱敏邮箱）。

**原生侧（Swift，确定性）**

- **held publication 顺序**：`holdPublications()` → `installServingPublication(P_old)`（被持有，`revision = 3`）→ quota 操作安装订阅段（`revision = 5`）→ `releasePublications()` → 断言订阅段仍为 5，而 P_old 的 usage/sessions 已安装。反向用例：候选 `revision = 6` 时替换。另含 `revision` 相同、`mono` 较大时替换，以及候选缺 `projection` 时不替换。
- **告警投递路径（R2-F2，Swift 确定性）**。注入通知 poster，记录调用与完成，并提供权限检查和 helper 结果。用独立打开的描述符持有已绑定的守卫 inode，构造与 App 的 flock 竞争。
  - 现有顺序保持为 `quota-refresh → deliver → ack → quota`，`deliver` 内部为"权限检查 → 取守卫 → 复核 → 同步调用 → 解锁 → 锁外等待完成 → ACK 成功项"。断言调用发生在持锁期间、完成等待发生在解锁之后，临界区内没有 `await`。
  - 权限未授予：零取锁、零调用，`requestAuthorization` 调用次数为 0。
  - 独立描述符在已绑定 inode 上持锁至 `G_app` 耗尽，零 Claude 调用、零 Claude ACK。
  - `epoch` 不等于提醒世代、守卫内容无效、`state_root_id` 不符、守卫路径 `(dev, ino)` 与描述符不符：对应的 Claude 提醒零调用。
  - Claude 守卫不可用时，用保留一条有效 Codex 提醒的同一 helper 信封驱动 Swift coordinator 和 notifier。断言既有 `quota_alerts_failed` warning。注入按时成功的 Codex 回调，断言恰有一个 Codex `add` 和 ACK，Claude `add` 和 ACK 均为零。
  - 同一批内重复 ID：只调用一次。
  - 完成回调永不到达或报错：守卫已释放，该 ID 不 ACK，`deliver` 在 `C` 后返回。
  - 断言不调用 `removeDeliveredNotifications`。
- **Widget 到期 entry**：固定 `now`、快照 `current_until = now + 90 s`、刷新为 `now + 4 min` → timeline 恰有两个 entry，日期分别为 `now` 与 `now + 90 s`，第一个渲染含脱敏邮箱，第二个不含 `@`；`current_until ≤ now` → 一个 entry、不含 `@`；`current_until` 晚于刷新时刻 → 一个 entry、含邮箱。small 与 medium 在任何输入下都不含 `@`。
- App Group 编码不含组织名，三态往返。

### A13.3 诊断恢复验收与历史收口（R1-F13）

- **构建绑定**：从被验收的提交构建一个隔离 helper，记录提交 SHA、工作区是否干净与二进制 sha256；全部步骤使用这一份二进制 `$BIN`。
- **隔离环境**：`T=$(mktemp -d)`；`HOME=$T/home`；`PATH=$T/bin:/usr/bin:/bin`，其中 `$T/bin/claude` 是假脚本：`auth status` 输出脱敏夹具，`-p /usage` 按 `$T/step` 文件输出预置内容；所有命令都带 `--state-dir $T/state`。
- **准备**（真实命令，均在隔离 HOME 下）：

```sh
$BIN --state-dir "$T/state" --format json provider use official --client claude
$BIN --state-dir "$T/state" --format json desktop quota-settings --reading on --clients claude
```

- **每一步**：先写入 `$T/step`，再执行下面两条命令。`desktop quota-refresh` 的响应只有 `gate_reasons` 与 `alerts`，只用来断言门禁为 null（通过）；窗口、失败与时间一律从第二条命令读取。

```sh
$BIN --state-dir "$T/state" --format json desktop quota-refresh --manual \
  | jq -e '.data.gate_reasons.claude == null'
$BIN --state-dir "$T/state" --format json quota \
  | jq '.data.clients[] | select(.client == "claude")
        | {failure, failure_detail, observed_at, windows: [.windows[] | {key, used_percent, observed_at}]}'
```

- **序列与断言**：
  - 步骤 1，有效两行：`failure == null`，窗口键恰为 `five_hour`、`seven_day`，记录 `observed_at = O1`。
  - 步骤 2，缺行：`failure == "parse_failed"`、`failure_detail == "missing_line"`、`windows == []`，`observed_at` 为本次失败尝试的时刻（≠ O1）。
  - 步骤 3，坏形状：`shape_invalid`。
  - 步骤 4，坏时区：`timezone_unresolved`。
  - 步骤 5，有效两行另含一行 `Current week (Fable): 99% used · …`：`failure == null`、`failure_detail == null`、`observed_at > O1`，`seven_day` 的 `used_percent` 等于 all-models 行的值而不是 99，窗口键仍恰为两个。
- **补充断言**：直接以只读方式打开隔离库，核对 `quota_envelopes_v2.failure_detail` 每一步与 JSON 一致，且步骤 5 后为空串。
- **两类证据分开**：自动化（CI 与本地隔离，只用夹具与假进程）可以随时运行；真实 CLI 采样只在用户逐次批准后进行，产物脱敏后作为夹具提交，并记录版本与时间，不作为日常测试运行。
- **历史**：记录"历史根因未复现，未作猜测性 parser 修改"，以及旧失败的已知时间、构建、调用上下文与未知部分；不声称修好了未知根因。

## A14 开放问题与残余风险

1. Codex `account/read` 是否存在共同稳定键：由 schema 夹具决定，没有证据时恒为 `unconfirmed`。
2. Claude `auth status` 的确切参数、键名、个人账号的组织表示与退出码语义：以样本夹具冻结（A13.1）；夹具未证实的形状一律保守降级。
3. 官方套餐展示名字段：当前证据中不存在；A4 规则 1 只在夹具证实后启用。
4. **残余风险（D2 范围内，不需要改决定）**：重新确认之后，一个在旧账号下启动、仍在运行的 Claude Code 交互会话可能继续推送旧账号的状态栏额度，AgentDeck 无法区分。这属于需求"不承诺识别所有未知切换"的范围；下一次身份周期若检测到变化会再次失效。
5. **可验证边界**：D2 的先后以捕获进程入口为准。Claude Code 在启动捕获进程之前组装载荷，这段时间（进程启动延迟）不可观察；入口晚于发布时刻 `T` 的捕获被接受。
6. **重启后的延迟**：重启机器后，到下一个刷新周期之前的状态栏捕获被丢弃，最多延迟一个刷新间隔（关闭方向）。
7. **系统通知显示时序**。按用户确认的 A5.7 提交边界验收。macOS 可能延后显示失效前已受理的请求，实际显示时序在原生验收中单独记录。
8. 状态栏捕获只写 `quota_windows_v2`，不触发 serving revision（与现状相同）；订阅段已改为读取时重新投影（A10.3），这里不新增 serving 触发器，以免状态栏推送频繁把整份发布物标为陈旧。投影 revision 触发器（A7 第 6 步）只服务订阅段的单调安装。
