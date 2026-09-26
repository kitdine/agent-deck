---
status: active
topic: health-recovery
subject: architecture.md
---

# Health Recovery Architecture Review

## Round 1 — 2026-09-22

## 📋 health-recovery / architecture.md 评审

📊 总体评分：4/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`architecture.md:36`](../architecture.md) `ARCH-R1-F1` 所有权前提与代码不符：文档把锁归给 `internal/state` 与 `internal/scan`，把扩展归给 `internal/extensions`，这三个包都不存在。
- 行为风险：实现者要么新建与现有锁协议并行的包，要么自行重新判断归属；任务分解无法把改动落到具体文件。
- 证据：`state.lock`、`scan.lock` 以及 quota/snapshot 锁都由 `internal/store/store.go:551-577` 的 `acquireNamedLock*` 统一获取；扩展发现与诊断在 `internal/extension/extension.go`。第 40 行把“死进程锁自动回收”写成新行为，但 `store.go:627` 的 `reclaimLockFromDeadProcess` 已经用 flock、`os.SameFile` 和内容复核实现了它。
💡 有界修复：按真实包重写归属，写明哪些现有函数和类型要改、哪些保持不变，并把现有回收协议当作前提引用，而不是重新描述。

[`architecture.md:17`](../architecture.md) `ARCH-R1-F2` `state_busy` 命令错误路径没有设计：新增字段只挂在 doctor 的 `Check` 上，没有覆盖失败命令的错误 envelope。
- 行为风险：CLI 契约要求的 `error.details.resource/reason/action_kind/recovery_command`，以及 `resource: unknown`，都没有生产者。state 锁和 scan 锁的竞争在命令层仍然无法区分，这正是 `ad-bug-state-busy-recovery-guidance` 的原始缺陷。
- 证据：`store.go:163` 只有一个 `ErrStateBusy` sentinel，`store.go:618` 对任何命名锁都返回同一句 "timed out waiting for state lock"；`cmd/agentdeck/main.go:456-464` 只把它映射成错误码。CLI 文档“Assumptions and unresolved implementation questions”明确要求架构决定“the typed Go owner for lock resource/reason/action detail”。
💡 有界修复：定义携带锁类别的类型化错误及其构造点，写清 CLI 在 text/JSON 两种模式下的 details 投影，以及调用方不知道锁类别时何时产出 `unknown`；保留 `state_busy` 错误码、退出码 1 和 `schema_ahead` 优先级。

[`architecture.md:38`](../architecture.md) `ARCH-R1-F3` doctor 只读锁检查缺少可实现的算法，而且自相矛盾。
- 行为风险：第 39 行说“PID 被复用时标为 unknown”，但第 38 行选定的 `kill -0` 无法检测 PID 复用，这条 fail-closed 保证没有任何机制支撑。文档也没说明只读 doctor 在不获取、不修改锁的前提下，如何区分 live、legacy、可回收三种状态。`lock_reclaimable` 是否作为 doctor 的观测结果（CLI 留给架构的开放问题）也没有决定。
- 证据：`internal/store/process_darwin.go:11-21` 只用 `syscall.Kill(pid, 0)`；`internal/doctor/doctor.go:211-229` 当前只检查 `state.lock`，并以文件年龄产出 `stale_lock`，另有 `lock_unreadable`、`state_busy` 两个 code。文档没有说明这些现有 code 如何退役或兼容，也没有新增 `scan_lock` 检查的位置。
💡 有界修复：写出只读分类算法（token 解析、liveness 判定、PID 复用或平台不支持时如何落到 `lock_owner_unknown`）；决定 `lock_reclaimable`；列出 `stale_lock`/`state_busy`/`lock_unreadable` 这三个 doctor code 到新 reason 的迁移与兼容方式；确定 `scan_lock` 行的产出点。

[`architecture.md:43`](../architecture.md) `ARCH-R1-F4` 扩展诊断分类不完整。
- 行为风险：CLI 契约冻结了 9 个扩展 reason，文档只覆盖 3 个（discovery_failed、stale_inventory、fingerprint_update_failed）。stale identity 和 native unavailable 仍然没有分开，而这正是 `ad-bug-extension-stale-mcp-recovery` 的根因；多种条件同时出现时，单一 `extensions` 行该给哪个 reason 和 action 也没有决定。
- 证据：`internal/extension/extension.go:59,178` 把缺席的存储 ID 一律放进 `MissingPaths`；`internal/doctor/doctor.go:168-170` 把五类问题相加成一个 `extension_diagnostics` 警告，并固定给出 `agentdeck extension doctor`。CLI 文档要求架构给出“the canonical extension diagnostic type replacing the current conflated `missing_paths` result”，以及 extension doctor JSON 中的 discovery status 和独立的 stale-inventory 集合。
💡 有界修复：为 9 个 reason 定义判定条件与规范诊断类型，写明 `missing_paths` 的兼容保留方式、多条件聚合规则，以及 extension doctor 的 JSON 字段。

[`architecture.md:29`](../architecture.md) `ARCH-R1-F5` 同步未完成（incomplete sync）的数据来源不成立。
- 行为风险：文档把 `inventory_committed` 放在健康检查字段上，但没有任何持久化记录能让后来的 doctor 或菜单栏快照知道上一次 scan 的指纹更新失败了。菜单栏 syncIncomplete 场景因此没有生产者；而 CLI 契约中 scan 命令失败时要用的 `extension_sync_incomplete` 错误本身也没有定义。
- 证据：CLI 文档“Synchronization result and recovery confirmation”把 `extension_sync_incomplete`、`inventory_committed: true` 定义为 `extension scan` 的退出 1 错误；`cmd/agentdeck/main.go:2902-2905` 的指纹步骤在库存提交之后执行，失败时直接返回错误。
💡 有界修复：二选一并写清楚——要么定义持久化标记及其写入、清除和读取方，要么把该状态限定为 scan 命令的错误，并回到菜单栏 UX 说明该场景的来源；两种方案都要定义 scan 的错误 envelope。

[`architecture.md:41`](../architecture.md) `ARCH-R1-F6` extension doctor 的只读入口没有设计。
- 行为风险：CLI 契约要求 extension doctor 不获取独占 state 锁、不 create/migrate/chmod、不写 WAL 或 setting。按当前实现，它会在状态锁竞争时失败，或者对缺失的 state 产生写入。
- 证据：`cmd/agentdeck/main.go:2883-2885` 的 `withExtensions` 对 doctor 和 scan 使用同一个 `opts.openStore`；文档的 “Doctor Read-Only” 只讨论锁文件，没有覆盖存储入口。
💡 有界修复：指定 extension doctor（以及聚合 doctor 调用 `extension.Doctor` 的路径）使用的只读存储入口，并写出 state 缺失、future schema、锁竞争、数据库不可读时的结果表。

### 🟡 建议改进 — 推荐

[`architecture.md:19`](../architecture.md) `ARCH-R1-F7` Desktop wire 与消费方契约不完整。
- 证据：
  - 字段没有映射到生产者：`internal/desktop/desktop.go:224,890` 逐字段复制 doctor 检查，`apps/macos/AgentDeckShared/DesktopWire.swift:1007` 是对应的 Swift 类型，但文档没有点名这两处及 `desktop/fixtures/v1`。
  - 缺少菜单栏文档要求的规则：新客户端遇到未知 token 时 fail closed（通用警告、action 置 null、不暴露命令）。
  - `stale_count` 与已有的 `count` 字段（`doctor.go:28`，原型也用 `count`）重复。
  - `manual_prerequisite` 的 key 词表没有定义。
💡 有界修复：给出逐字段的“生产者 → wire → Swift”映射表和 fixture 更新，写入 fail-closed 规则，复用 `count` 或说明新增字段的理由，并列出 manual prerequisite 的 key 集合。

[`architecture.md:53`](../architecture.md) `ARCH-R1-F8` 排序一节把命令错误优先级和 doctor 行顺序混在了一起，并与 CLI 契约冲突。
- 证据：CLI 文档要求 doctor “retain their existing header, check ordering”。现有顺序是 `state_permissions`、`hook_deliveries`、`state_lock`（`doctor.go:68`）、`database`、`schema` …… `extensions`；第 55-59 行却提出“Lock → Schema → Extension”的新顺序，而且没有给 `scan_lock` 定位置。`schema_ahead` 的优先级属于命令错误，不属于行顺序。
💡 有界修复：拆成两节，分别写命令错误优先级和 doctor 行顺序；保留现有顺序，写明 `scan_lock` 插在哪里。

[`architecture.md:76`](../architecture.md) `ARCH-R1-F9` 测试缝不安全，或者不存在。
- 证据：第 78 行提议用“test-only 环境变量”覆盖锁 liveness，这会给发布的二进制加上可以绕过 fail-closed 判定的开关。现有函数注入缝 `acquireNamedLockWithChecks(processAlive, tryReclaimLock)`（`store.go:581-588`）没有被复用。`ExtensionDiscoverer` 接口不存在，发现逻辑是包级函数 `discover`（`extension.go:151`），但文档没有把引入接口列为改动。“strict validation mode”也没有定义。
💡 有界修复：改用包内函数或接口注入，不用环境变量；点名要引入的接口；把 wire 校验定义为具体的测试断言。

[`architecture.md:84`](../architecture.md) `ARCH-R1-F10` 验收边界只是自我引用，没有追溯到需求。
- 证据：需求 `requirements.md:219-239` 列出 12 个验收场景及确定性 fixture 要求，文档没有把它们映射到责任组件和验证方式；第 84 行只是复述“映射准确即通过”。
💡 有界修复：增加“需求场景 → 负责组件 → 测试缝/验证”追溯表，供 `tasks.md` 分解直接使用。

[`architecture.md:65`](../architecture.md) `ARCH-R1-F11` L0 空白检查失败：该行末尾有尾随空格。
- 证据：`make check-whitespace` 退出 1，报告 `trailing whitespace: docs/topics/health-recovery/architecture.md:65`。
💡 有界修复：删除尾随空格，并在修复后重跑 `make check-whitespace`。

### 🟢 优点

- action 分类（diagnose、retry、synchronize_inventory、manual_prerequisite）与需求一致，诊断命令与恢复命令用独立字段区分，杜绝把只读命令标成修复。
- 明确拒绝暴露 lock token、原始 SQLite 错误、路径和 manifest 内容，legacy 锁只给人工前提、不给 `rm`，符合 fail-closed 安全边界。
- 桌面刷新语义正确：复制不改变健康状态，只有后续快照证明条件已清除才移除提示；1.6 秒反馈与菜单栏文档第 141 行一致。

### 📝 总结

- Reviewed state：HEAD `29694de77343bfa623ad04fa21c0e581564cda32`；`docs/topics/health-recovery/architecture.md` blob `ef63f295dfd6fa612a55a6626f3eaf47c596ecc3`（已暂存、未提交）；内容状态 `urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:374678effe183b60d9a457e01c9f9b85dee1a04f9f8e75c5e557972f55e40d56`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REVIEW 角色；Method：设计/契约评审，按前提有效性、与现有实现的一致性、场景覆盖、决策完整性、内部矛盾五个维度，逐条对照 `requirements.md`、两份 UX 文档的 “Data requirements / unresolved implementation questions” 以及 `internal/store`、`internal/doctor`、`internal/extension`、`internal/desktop`、`cmd/agentdeck`、`DesktopWire.swift` 源码。
- Scope：仅 `architecture.md`；两份 UX 文档与需求作为已评审的上游契约使用，不重新评审。项目没有针对架构文档的专用检查器；未运行代码测试，因为本文档不改变任何实现。
- Evidence：`make check-whitespace` FAIL（`ARCH-R1-F11`）；其余结论来自上面各条引用的源码位置。
- Completion gate：FAILED — review criterion 被 `ARCH-R1-F1` 至 `ARCH-R1-F10` 直接否证，L0 criterion 被 `ARCH-R1-F11` 否证。
- 流程观察（不属于文档缺陷）：评审开始时 `ad-hr-doc-arch-design` 仍为 `open`，设计授权 gate `ad-hr-doc-arch-design-gate` 未关闭，没有设计交接评论，CEv1 中也没有该文档的 WorkUnit。本轮已把任务转为 `in_progress` 供修复，并在 CEv1 中登记 WorkUnit 与本轮结果；gate 保持原状，由其授权方处理。
- Residual uncertainty：没有核对 Swift 端 `HealthCheck` 解码是否对未知 key 宽容（Codable 默认忽略，但未读到对应测试）；也没有核对原型/UX 使用的 `failed` 状态词与 doctor 现有 `error` 状态词之间的映射。前者由 `ARCH-R1-F7` 的修复负责覆盖。

## Round 2 — 2026-09-22

## 📋 health-recovery / architecture.md 复评

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`architecture.md:46`](../architecture.md) `ARCH-R1-F3` 仍未关闭，且修复引入了新的矛盾：只读锁分类删掉了 `lock_legacy`，`lock_live` 的判定也不成立。
- 处置：still open。
- 行为风险：
  - 第 48 行把 legacy token 并入 `lock_owner_unknown`，但第 100 行仍按 `lock_legacy` 提供 manual prerequisite。两份 UX 文档冻结的词表里 `lock_legacy` 是独立 reason，带人工前提和复制按钮。按当前分类，legacy 锁永远拿不到那条恢复路径。
  - 第 47 行说 `kill -0` 无法排除 PID 复用、因此落到 `lock_owner_unknown`；第 48 行又说存在「confirmed alive」的 `lock_live`。按第 47 行，任何存活 PID 都无法确认，`lock_live` 永远不可达；按第 48 行又会把复用的 PID 报成 live。两者不能同时成立。
  - 第 48 行的「recently acquired」是按年龄判定，违反需求「Age alone never …」。
  - 旧 code（`stale_lock`、`state_busy`、`lock_unreadable`）只说「migrated」，没有逐一映射；`lock_reclaimable` 在 doctor 中是 ok 还是 warning 也没写。
- 证据：`architecture.md:46-50,100`；CLI 文档 JSON 词表中 lock reason 为 `lock_live`、`lock_legacy`、`lock_owner_unknown`、`lock_reclaimable`；`internal/doctor/doctor.go:211-229`。
💡 有界修复：恢复 `lock_legacy`，写成「token 无 owner 字段」的确定判定；明确 `lock_live` 的判定依据，以及 PID 复用风险如何处理（例如接受 kill -0 存活即 live，并说明 live 只会给出 retry、绝不给出删除，所以误判不危险）；删除年龄条件；给出旧 code → 新 reason 的映射表和 `lock_reclaimable` 的状态值。

[`architecture.md:56`](../architecture.md) `ARCH-R1-F4` 仍未关闭：9 个 reason 已经列出，但区分它们的判定条件和 JSON 形状仍缺。
- 处置：still open（范围缩小）。
- 行为风险：`extension_stale_inventory`（「数据库里有、原生不再有」）和 `extension_native_unavailable`（「原生路径确实不可用」）两条定义在同一个输入上无法区分，而这正是 stale MCP 那个 bug 的根因。第 68 行说 `missing_paths` 由 stale inventory 映射而来，但它历史上同时包含这两类。聚合优先级只给了「e.g.」三级；「所有详细条件保留在 JSON」和 extension doctor 的 discovery status、stale 集合都没有字段定义。
- 证据：`architecture.md:61,65,68`；`internal/extension/extension.go:178`；CLI 文档「JSON and automation contract」要求 extension doctor 提供 discovery status 和独立的 stale-inventory 集合。
💡 有界修复：写出 stale 与 native-unavailable 的判定依据（发现成功与否、adapter 对路径可用性的报告等）；给出全部 9 个 reason 的完整优先级；定义 extension doctor 和聚合 doctor 的 JSON 字段；说明 `missing_paths` 的兼容取值。

[`architecture.md:66`](../architecture.md) `ARCH-R1-F5` 仍未关闭：`ErrExtensionSyncIncomplete` 已定义为 scan 命令错误，但第 66 行仍把 `extension_fingerprint_update_failed` 列为只读 doctor 分类出的第 9 个条件。
- 处置：still open（范围缩小）。
- 行为风险：doctor 没有任何持久化来源得知上一次 scan 的指纹写入失败了，所以这一行要么永远不出现，要么逼实现者自行发明标记。菜单栏的 syncIncomplete 健康场景仍然没有生产者。
- 证据：`architecture.md:38,56,66`；菜单栏原型 `prototype/src/healthRecovery.js` 的 `syncIncomplete` 是一个健康检查状态。
💡 有界修复：二选一——要么定义持久化标记（写入方、清除条件、doctor 读取方），要么把该 reason 从 doctor 分类中移除，并注明菜单栏 syncIncomplete 状态需回到 UX 文档修订。

[`architecture.md:37`](../architecture.md) `ARCH-R1-F2` 仍未关闭：`ErrLockContention` 已定义，但没说它如何得到 `Resource` 和 `Reason`。
- 处置：still open（范围缩小）。
- 行为风险：`store.go:618` 位于 `acquireNamedLockWithChecks`，它被 `state.lock`、`scan.lock`、`quota-refresh.lock`、`derived-snapshot-cache.lock` 共用。文档没有给出锁名 → `resource` 的映射；后两个锁在冻结词表里也没有对应值。超时时的 `Reason`（live/legacy/owner_unknown）要不要当场分类、复用哪个分类器，同样没写。
- 证据：`internal/store/store.go:551-577,618`；CLI 文档 `resource` 取值只有 `state`、`scan`、`extension_inventory`、`unknown`。
💡 有界修复：给出锁名 → resource 的映射（包括另外两个锁的处理，例如映射到 `unknown` 或不包装）；说明超时路径复用 doctor 的只读分类器来得到 `Reason`，以及 `ActionKind` 怎么随之确定。

### 🟡 建议改进 — 推荐

[`architecture.md:81`](../architecture.md) `ARCH-R1-F8` 仍未关闭：doctor 行顺序与现有代码不符，而且用了占位符。
- 处置：still open。
- 证据：`internal/doctor/doctor.go:57-76` 的实际顺序是 `state`、`state_permissions`、`state_lock`（`checkLock`）、`hook_deliveries`、`database`……，文档第 82-84 行把 `hook_deliveries` 写在 `state_lock` 之前，也漏了 `state`。第 88-89 行的「...」「X.」不是可实现的顺序。
- 更正：这个错误顺序源自 Round 1 本条证据本身——Round 1 把现有顺序写成了「`state_permissions`、`hook_deliveries`、`state_lock`」，是评审记录的错误，修复只是照抄了它。以本轮按源码核对的顺序为准。
💡 有界修复：按 `doctor.go` 的实际调用顺序完整列出，并标明 `scan_lock` 的插入点。

[`architecture.md:28`](../architecture.md) `ARCH-R1-F7` 仍未关闭：wire 映射路径和 fail-closed 规则已补上，`count` 也已复用，但 `manual_prerequisite` 的 key 仍然只有「e.g.」两个示例。
- 处置：still open（范围缩小）。
- 证据：`architecture.md:28`；菜单栏文档要求「localized safe prerequisite identifier」，Swift 端需要一份封闭的 key 集合才能做本地化和 fail-closed。
💡 有界修复：列出完整的 key 集合，并说明每个 key 对应哪个 reason。

[`architecture.md:11`](../architecture.md) `ARCH-R2-F1`（新）：全文 57 行把行内代码写成了字面量 `\``（反斜杠加反引号）。
- 处置：new。
- 证据：``rg -c '\\`' docs/topics/health-recovery/architecture.md`` 返回 57。在 Markdown 中 `\`` 渲染为普通反引号字符，所以所有包名、字段名、命令都失去代码格式，并显示出多余的反引号。
💡 有界修复：把 `\`` 全部替换为 `` ` ``，然后检查渲染结果。

[`architecture.md:127`](../architecture.md) `ARCH-R2-F2`（新）：文件末尾多一个空行，L0 仍然失败。
- 处置：new。
- 证据：`git diff --check` 退出 2，报告 `docs/topics/health-recovery/architecture.md:127: new blank line at EOF.`；`make check-whitespace` 不检查这一项，所以通过。
💡 有界修复：删除末尾空行，修复后同时重跑 `make check-whitespace` 和 `git diff --check`。

### 🟢 优点

- `ARCH-R1-F1` closed：归属改为真实存在的 `internal/store` 与 `internal/extension`，并把现有 `reclaimLockFromDeadProcess` 当作前提引用。
- `ARCH-R1-F6` closed：extension doctor 和聚合 doctor 改用 `store.OpenReadOnly`（`store.go:263`，`mode=ro`，不建库、不迁移、不开 WAL、不获取 state 锁），满足 CLI 的只读要求。
- `ARCH-R1-F9` closed：环境变量开关已删除，改用现有的函数注入缝和新引入的 `ExtensionDiscoverer` 接口。
- `ARCH-R1-F10` closed：新增 12 行「需求场景 → 负责组件 → 验证」追溯表；其中与锁分类相关的行会随 `ARCH-R1-F3` 的修复一起更新。
- `ARCH-R1-F11` closed：`make check-whitespace` 通过。
- 命令错误优先级与 doctor 行顺序已拆成两节，方向正确。

### 📝 总结

- 逐项处置：
  - closed：F1、F6、F9、F10、F11。
  - still open：F2、F3、F4、F5、F7、F8。
  - 新增：`ARCH-R2-F1`、`ARCH-R2-F2`。
  - 没有 regressed 的发现；`ARCH-R1-F3` 中删除 `lock_legacy` 属于该发现修复时引入的新矛盾，已并入其 still open 描述。
- Reviewed state：HEAD `29694de77343bfa623ad04fa21c0e581564cda32`；`docs/topics/health-recovery/architecture.md` blob `854d4f075b368454fe0cb501e566ca6a5f263070`（暂存 + 未暂存修改）；内容状态 `urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:531a7b35af577118cd1bf3d28edfc4d4a109559e3a85f88ef01d40e99d86e01a`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REREVIEW 角色；Method：逐条对照 Round 1 发现，重新核对 `internal/store/store.go`、`internal/doctor/doctor.go`、`internal/extension/extension.go` 与两份 UX 文档的冻结词表。
- Evidence：`make check-whitespace` PASS；`git diff --check` FAIL（`ARCH-R2-F2`）；``rg -c '\\`'`` 返回 57。
- Completion gate：FAILED — L0 criterion 被 `ARCH-R2-F2` 否证，review criterion 被上述 still open 与新增发现否证。
- 流程观察：本轮修复没有 Beads 交接评论，任务仍为 `in_progress`；设计授权 gate `ad-hr-doc-arch-design-gate` 仍未关闭。
- Residual uncertainty：没有核对 CLI 在锁竞争时的完整调用链是否全部经过 `acquireNamedLockWithChecks`，例如 watch 路径（`internal/watch/watch.go:78`）也直接判断 `ErrStateBusy`，它是否需要改用 `ErrLockContention` 留给 `ARCH-R1-F2` 的修复说明。

## Round 3 — 2026-09-22

## 📋 health-recovery / architecture.md 复评

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`architecture.md:99`](../architecture.md) `ARCH-R3-F1`（新）：扩展 reason 到 `action_kind`/命令的映射缺失，stale inventory 的同步恢复路径从文档中消失了。
- 处置：new。这条映射在 Round 1 草稿第 48 行存在（`extension_stale_inventory` → `action_kind: synchronize_inventory`、`recovery_command: agentdeck extension scan`），在 Round 2 修订中丢失。Round 2 复评没有发现这一点，属于评审遗漏。
- 行为风险：
  - 文档只给出了 manual prerequisite 类 reason 的 key 表，以及 `extension_fingerprint_update_failed` → `diagnose`。`extension_stale_inventory` 应产出什么 action 和命令、`extension_state_missing` 的 action 是什么、何时带 `diagnostic_command`，都没有写。
  - 文档也没有说明要退役 `internal/doctor/doctor.go:170` 固定输出的 `Recovery: "agentdeck extension doctor"`。
  - 结果是 `ad-bug-extension-stale-mcp-recovery` 的核心修复——对 stale inventory 给出 `agentdeck extension scan`——在架构里没有生产者。
- 证据：`rg -n "synchronize_inventory|extension scan"` 在当前草稿中只命中字段说明（第 25 行）、scan 错误（第 54-55 行）和刷新语义（第 186 行）。CLI 文档「Aggregate doctor presentation」和「Read-only entry and unavailable state」分别要求 stale 给出 `recovery: agentdeck extension scan`，`extension_state_missing` 只在发现成功后才给 scan。
💡 有界修复：为全部 9 个扩展 reason 加一张「reason → status → action_kind → recovery_command / diagnostic_command / manual_prerequisite」表（含 `extension_state_missing` 的条件式 scan），并写明聚合 doctor 以主 reason 的行取代现有固定的 `extension_diagnostics` + `agentdeck extension doctor`。

[`architecture.md:105`](../architecture.md) `ARCH-R1-F4` 仍未关闭（范围进一步缩小）：stale 与 native-unavailable 的判定方法已写出，但与发现流程的衔接有三处缺口。
- 处置：still open。
- 行为风险与证据：
  1. 第 105 行说 stale 判定是「today's `MissingPaths` logic, unconditional on `discoveryErr == nil`」。但 `internal/extension/extension.go:173-175` 在发现失败时会跳过整个比较，需求也要求「live discovery failed, so persisted-versus-live conclusions are unavailable」。照字面实现，发现失败时每条已存 ID 都会被报成 stale。
  2. `discover()` 在 `extension.go:269-272` 对任何一个候选算 `fingerprint(sourcePath)` 失败就中止整次发现。原生路径不可用最常见的表现正是 sourcePath 读不到，所以按现有代码它会变成 `extension_discovery_failed`，文档设想的「照常产出行并填写 per-item `Diagnostics`」够不到。文档需要说明不可用项的指纹如何处理。
  3. `discover()` 成功时仍会返回非致命 `diagnostics`（`extension.go:264-267`，canonical ID 无效的候选被跳过），现有聚合 doctor 把它们计入问题数（`doctor.go:168`），但 9 个 reason 里没有它的归属。
💡 有界修复：明确 stale、native-unavailable、drift 只在发现成功时计算；规定不可用项的指纹策略（例如记为不可用并跳过指纹，而不是中止发现）；给无效 canonical ID 的非致命诊断指定一个已冻结的 reason，或者说明它不计入健康问题。

### 🟡 建议改进 — 推荐

[`architecture.md:160`](../architecture.md) `ARCH-R1-F8` 仍未关闭（范围缩小）：文档称这是「complete, implementable order」，但漏掉了 `checkUsage` 产出的 4 行。
- 处置：still open。
- 证据：`internal/doctor/doctor.go:143` 调用 `checkUsage`，它依次产出 `usage`、`usage_sources`、`prices`、`price_provenance`、`unpriced_models`（`doctor.go:443-483`）；文档第 177 行只写了「`database` (schema_incompatible) or `usage`」。`state_lock`/`scan_lock`/`hook_deliveries` 的位置已经正确。
💡 有界修复：补全这 4 行，或者改写为「除插入 `scan_lock` 外保持现有顺序」，不再自称完整列表。

[`architecture.md:50`](../architecture.md) `ARCH-R3-F2`（新）：quota 刷新锁「从不经过交互命令错误路径」的前提与代码不符。
- 处置：new。
- 证据：`cmd/agentdeck/quota.go:378` 的 `runDesktopQuotaRefresh` 由 `desktop quota refresh`（含用户触发的 `--manual`）调用，锁竞争时直接返回普通 `ErrStateBusy`。第 53 行「none remain in the interactive CLI surface after this change」因此不成立。按文档的回退规则，它会产出 `resource: unknown`，行为本身有定义，但前提写错了。
💡 有界修复：把这条路径写成已知的 `resource: unknown` 调用方（或者说明它为何不需要锁类别），删除「无剩余调用方」的断言。

### 🟢 优点

- `ARCH-R1-F2` closed：只有 `AcquireLock`（`state`）和 `AcquireScanLock`（`scan`）构造 `ErrLockContention`；超时那一刻复用同一个分类器得到 `Reason`，再由表推出 `ActionKind`；分类器打不开文件时回退为 `lock_owner_unknown`；`Unwrap` 保持 `errors.Is(err, ErrStateBusy)` 成立，所以 `internal/watch/watch.go:78` 不受影响。遗留的 quota 路径前提错误单列为 `ARCH-R3-F2`。
- `ARCH-R1-F3` closed：`ClassifyLock` 是确定的四步算法；legacy 按 token 形状（`lockOwnerPID` 的 `ok:false`）判定，不看年龄；PID 复用被论证为有界风险（`lock_live` 只给 retry，删除只来自与存活判断无关的 legacy 路径）；`lock_reclaimable` 以 `ok` 呈现；旧 code 有逐条迁移表。以上与 `store.go:702-720`、`process_darwin.go:11-21` 一致。
- `ARCH-R1-F5` closed：定义了持久化标记 `extension.sync_incomplete`，写明写入方（scan 在指纹失败时尽力写入）、清除条件（下一次完整成功的 scan）和读取方（只读 extension doctor）；`store.SetSetting`/`Setting` 在 `store.go:387-396` 存在。
- `ARCH-R1-F7` closed：manual prerequisite key 集合封闭，一个 reason 对应一个 key，并明确 `lock_owner_unknown` 没有 key。
- `ARCH-R2-F1` closed：反引号转义已全部清除（`rg -c` 无匹配）。
- `ARCH-R2-F2` closed：末尾空行已删除，`git diff --check` 通过。

### 📝 总结

- 逐项处置：
  - closed：`ARCH-R1-F2`、`F3`、`F5`、`F7`，`ARCH-R2-F1`、`ARCH-R2-F2`。
  - still open：`ARCH-R1-F4`、`ARCH-R1-F8`。
  - 新增：`ARCH-R3-F1`、`ARCH-R3-F2`。
  - 更正：`ARCH-R3-F1` 的缺口在 Round 2 已经存在，Round 2 复评遗漏了它。
- Reviewed state：HEAD `29694de77343bfa623ad04fa21c0e581564cda32`；`docs/topics/health-recovery/architecture.md` blob `5a8c56b6bebfc4235d02aceb1d71ba26c2faf17a`；内容状态 `urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:7c512d22680107f29fd4ca0a4efb6159aa4efb4693ab22c680df05480a8e1e62`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REREVIEW 角色；Method：逐条对照前两轮发现，重新核对 `internal/store/store.go`、`internal/extension/extension.go`、`internal/doctor/doctor.go`、`cmd/agentdeck/quota.go` 与两份 UX 文档。
- Evidence：`make check-whitespace` PASS；`git diff --check` PASS；``rg -c '\\`'`` 无匹配。
- Completion gate：FAILED — L0 criterion 通过，review criterion 被 `ARCH-R1-F4`、`ARCH-R1-F8`、`ARCH-R3-F1`、`ARCH-R3-F2` 否证。
- Residual uncertainty：
  - 分类器第 3 步会把「刚以 `O_EXCL` 创建、token 尚未写入」的空锁文件判为 `lock_legacy`（`store.go:595-597` 之间有极短的窗口）。由于 legacy 的人工前提要求先确认没有 AgentDeck 进程在运行，这不构成不安全删除，本轮不列为发现。
  - 扩展 reason 对应的 doctor 状态词（现有 `warning`/`error` 与 UX 的 Warning/Failed）将随 `ARCH-R3-F1` 的映射表一起确定。

## Round 4 — 2026-09-22

## 📋 health-recovery / architecture.md 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `ARCH-R3-F1` closed：新增「Reason → action mapping」表，9 个扩展 reason 各占一行，列出 status、`action_kind`、`recovery_command`、`manual_prerequisite` 和聚合行的 `diagnostic_command`。
  - `extension_stale_inventory` 恢复为 `synchronize_inventory` + `agentdeck extension scan`；
  - `extension_state_missing` 只在发现成功时给出 scan；
  - 明确退役 `internal/doctor/doctor.go:168-170` 固定的 `extension_diagnostics` + `agentdeck extension doctor`；
  - 验收表第 252 行把 stale 场景的断言写到了 action 和命令。
- `ARCH-R1-F4` closed：
  - 第 105 行明确 stale、native-unavailable、drift 只在发现成功时计算，与 `extension.go:173-175` 一致；发现失败时这些集合输出为 JSON `null` 而不是 `[]`。
  - 第 110 行改为单个候选指纹失败不再中止发现，并把 `fingerprint` 的两类错误（`extension.go:551-561` 的 `source unavailable`/`source unreadable`）映射为安全类别，写入已有的 per-item `Diagnostics`。这使 native-unavailable 真正可达，同时排除它被重复报成 drift，并说明 scan 会保留该行。
  - 第 118 行把无效 canonical ID 的非致命诊断定为信息项，并退役 `doctor.go:168` 中对它的计数。
- `ARCH-R1-F8` closed：行顺序补全了 `checkUsage` 的 `usage`、`usage_sources`、`prices`、`price_provenance`、`unpriced_models`，full 模式标注与 `doctor.go:447-470` 一致，`schemaErr`/`checkUsage` 出错时的两种回退形态与 `doctor.go:141-145` 一致。
- `ARCH-R3-F2` closed：第 50、53 行承认 `desktop quota refresh`（含 `--manual`，`cmd/agentdeck/quota.go:378`）会返回普通 `ErrStateBusy`，并把它定义为按设计产出 `resource: unknown` 的已知调用方，理由是冻结词表没有 quota/cache 值。
- 第 126 行正确指出聚合 doctor 在状态或数据库打不开时提前返回（`doctor.go:55-83`，该路径已使用 `store.OpenReadOnly`），所以 `extension_state_missing`/`extension_inventory_unreadable` 只由 `extension doctor` 产出。

### 📝 总结

- 逐项处置：
  - `ARCH-R1-F1`–`F11`、`ARCH-R2-F1`、`ARCH-R2-F2`、`ARCH-R3-F1`、`ARCH-R3-F2` 全部 closed。
  - 没有 still open、regressed 或新增的发现。
- Reviewed state：HEAD `29694de77343bfa623ad04fa21c0e581564cda32`；`docs/topics/health-recovery/architecture.md` blob `d5744a29e8e11a2bc9944553a2c560340df82334`；内容状态 `urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:87b6094fbb40f55784ebd5a2f9deb2872613e2e9a22096a35afed7a43b9ceef7`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REREVIEW 角色；Method：逐条对照 Round 3 的 4 条遗留发现，重新核对 `internal/extension/extension.go`、`internal/doctor/doctor.go`、`cmd/agentdeck/quota.go` 与 CLI 文档的只读入口和聚合呈现契约，并通读修订后的测试缝与验收表。
- Evidence：`make check-whitespace` PASS；`git diff --check` PASS；``rg -c '\\`'`` 无匹配。
- Completion gate：VERIFIED — 见本轮写入的 CEv1 l0/review 证据与门禁复查。
- Residual uncertainty：
  - 分类器会把「刚创建、尚未写入 token」的空锁文件判为 `lock_legacy`（`store.go:595-597` 的极短窗口）；legacy 的人工前提要求先确认没有 AgentDeck 进程在运行，因此不构成不安全删除。
  - 技能目录的 `fingerprint` 在遍历出错时返回原始 `walkErr`（`extension.go:569`），实现需把它也归入安全类别；第 110 行「never the raw path or OS error」已覆盖这一要求。
  - 这是 framework 级架构评审；最终 UX 对账（`tasks.md` 设计进程中的 final UX reconciliation）仍需确认两份 UX 文档与本架构逐字段一致。
- 流程观察：设计授权 gate `ad-hr-doc-arch-design-gate` 仍未关闭，各轮修复也都没有 Beads 交接评论。gate 由其授权方处理，本轮不改动。
