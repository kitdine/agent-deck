---
status: active
topic: health-recovery
subject: tasks.md
---

# Health Recovery Tasks Review

## Round 1 — 2026-09-22

## 📋 health-recovery / tasks.md 分解评审

📊 总体评分：5/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`tasks.md:363`](../tasks.md) `TASKS-R1-F1` Task 4 的动作呈现与已评审的菜单栏 UX 契约相冲突。
- 行为风险：按这份分解实现，会出现两类违约：App 提供「run」诊断命令，违反「The menu-bar app never executes these commands」；`retry` 显示动作按钮，而 UX 规定 `retry` 或 null「No action button」。另外，按钮文案用了泛化的「Copy Command」，而 UX 冻结的是按 action 区分的「Copy sync command / 复制同步命令」「Copy diagnostic command / 复制诊断命令」「Copy safety steps / 复制安全步骤」。「focus restoration after clicking Copy」也与 UX「Focus does not move」不一致。
- 证据：`tasks.md:364-369,381`；`ux/menubar-health-recovery.md:131-143` 的 Action contract 表与复制反馈说明。
💡 有界修复：把 Task 4 的 action 呈现改写为直接引用菜单栏 Action contract：只复制、不执行；`retry`/null 无按钮；三种按 action 区分的文案；复制后焦点不移动，通过 polite live region 播报 1.6 秒。

[`tasks.md:92`](../tasks.md) `TASKS-R1-F2` Tasks 1 和 2 被声明为可并行、互不依赖，但它们依赖的共享契约管线没有唯一负责人，同一批文件会被两边同时修改。
- 行为风险：
  - 两个任务都需要给 `internal/doctor` 的 `Check` 结构体（`internal/doctor/doctor.go:24-31`）增加 `resource`/`reason`/`action_kind`/`diagnostic_command`/`manual_prerequisite` 字段，但分解只给 Task 1 分配了 `checkLock`、给 Task 2 分配了 `extensions` 行，结构体本身没有负责人。
  - CLI `error.details` 的投影机制在两处都需要：Task 1 的 `ErrLockContention`，以及 Task 2 的 `ErrExtensionSyncIncomplete`（需要 `inventory_committed`）。分解没有说由谁建立、谁复用。
  - aggregate `agentdeck doctor` 的文本和 JSON 渲染（`cmd/agentdeck/main.go:4940-4946` 目前只输出 `recovery:`）要新增 `next:`、`manual prerequisite:`、`diagnose:`、`effect:` 行，但 Task 1 在 `main.go` 中只负责命令错误，Task 2 只负责 `extension doctor`/`extension scan`，这部分渲染无人负责。
  - 并行执行时，`internal/doctor/doctor.go`、`cmd/agentdeck/main.go`、`cmd/agentdeck/main_test.go` 会被两个任务同时编辑，违反本文件「without overlapping responsibility」的评审边界。
- 证据：`tasks.md:92-98,112-121,197-202`；`internal/doctor/doctor.go:24-31`；`cmd/agentdeck/main.go:4940-4946`。
💡 有界修复：把共享管线（`doctor.Check` 新字段、`error.details` 投影、aggregate doctor 的文本/JSON 渲染）分配给唯一负责人——可以是一个前置的共享契约任务，也可以明确归 Task 1 并让 Task 2 依赖它——并相应更新依赖图。

### 🟡 建议改进 — 推荐

[`tasks.md:200`](../tasks.md) `TASKS-R1-F3` 多个文件归属路径不存在或放错了位置，另有用「(or …)」留开口的写法。
- 证据：
  - `cmd/agentdeck/extension.go` 不存在，extension 子命令和 `withExtensions` 都在 `cmd/agentdeck/main.go:2883`。
  - 字符串目录的实际位置是 `apps/macos/AgentDeckApp/Localizable.xcstrings`，不是 `Resources/Localizable.xcstrings`；健康相关文案常量在 `apps/macos/AgentDeckApp/DesktopCopy.swift`（如第 169 行的 `healthCopyRecovery`），Task 4 没有把它列入。
  - App 测试目录是 `apps/macos/AgentDeckAppTests/`，不是 `AgentDeckTests/`；已有的 `MenuBarViewModelTests.swift:659` 覆盖健康详情，以及 `DesktopCopyTests.swift`，都应列为 Task 4 的负责文件。
  - `MenuBarViewModel.swift (or health presentation view models)`、`acceptance_test.go (or acceptance test harness)` 这样的写法让归属悬而未决。
  - Task 5 把 `scripts/run-go-test.sh`、`scripts/test-macos-app.sh` 列为负责文件，但它只是运行它们，不修改它们。
💡 有界修复：改成真实存在的路径（或明确标注为新文件），删掉「or」式的开口，并把只运行的脚本移到 Verification 部分。

[`tasks.md:295`](../tasks.md) `TASKS-R1-F4` Task 3 的 wire 字段说明与现有代码不一致。
- 证据：Go 端 wire 类型叫 `HealthCheck`，不叫 `Check`（`internal/desktop/desktop.go:218`）；它已经有 `Recovery string` 字段，JSON tag 就是 `recovery_command`（`desktop.go:224`）。再加一个同 tag 的 `RecoveryCommand *string` 会出现重复字段。`Count *int` 写着「reuses existing field」，实际却把现有的 `Count int` 改了类型。Swift 端也已有 `recoveryCommand: String?`（`DesktopWire.swift:1007`），不属于新增字段。
💡 有界修复：以现有 `HealthCheck` 为基础，只列出真正新增的字段（`resource`、`reason`、`action_kind`、`diagnostic_command`、`manual_prerequisite`），保留 `Recovery`/`Count` 原有的类型和 tag。

[`tasks.md:168`](../tasks.md) `TASKS-R1-F5` Tasks 1–3 的验证命令不符合项目的 L0–L4 矩阵。
- 证据：Tasks 1 和 2 都改变了 Go 核心的 CLI/doctor 契约，Task 3 改变了 wire 契约。按 `.agent-instructions/project-rules.md` 的矩阵，这些属于 L2 及以上，要求「Go core contract changes require `scripts/run-go-test.sh ./...`」，而且 Go 测试必须走 wrapper。Task 1（第 170 行）和 Task 2（第 258 行）只写了「tests with `-race`」，没有点名 wrapper，也没有全量 `./...`；Task 3（第 324 行）只跑 `./internal/desktop/...`。全量套件只出现在 Task 5，不能替代各任务自己的门禁。
💡 有界修复：给每个 Go 任务写出具体命令——`scripts/run-go-test.sh <受影响包>`、对受影响包跑 `scripts/run-go-test.sh -race`，以及契约变更要求的 `scripts/run-go-test.sh ./...`。

[`tasks.md:61`](../tasks.md) `TASKS-R1-F6` 文档声称「最终 UX 对账」已经完成，但没有任何记录或证据。
- 证据：第 61-64 行写「Final UX reconciliation is verified」，第 474-479 行写「Final UX reconciliation confirms … without data gaps」。但设计进程（第 35-41 行）把它列为 architecture 之后的独立步骤，`reviews/` 下没有对应记录，`reviews/architecture.md` Round 4 的 residual 也明确说这一步「仍需确认」。
💡 有界修复：要么完成这次对账并留下记录（例如逐字段的「UX 字段 → architecture 生产者」表及结论），要么把这两处改成「待完成」，不要在状态权威里写未经验证的结论。

### 🟢 优点

- 五个任务与两个缺陷来源对应清楚：Task 1 覆盖 state-busy，Task 2 覆盖 stale MCP，两者各自有独立的验收轨道。
- Task 1、Task 2 把 architecture 的关键不变式写成了可检查的结果：分类器四步、不看年龄、`lock_reclaimable` 以 `ok` 呈现、只在发现成功时比较、JSON `null` 与 `[]` 的区别、同步标记的写入与清除。
- Task 5 把 architecture 的 16 行验收表逐行列为回归目标，并包含 `docs/specs/cli-design.md` 对账和原生残余风险记录。
- 评审边界清楚：通过后只允许创建这五个实现 Beads 任务及其开发授权 gate，不授权实现或交付。

### 📝 总结

- Reviewed state：HEAD `071fceb5d008cfe611108be0faa826d48ca5ad88`；`docs/topics/health-recovery/tasks.md` blob `28562a83d69c8acc659e6c35c4101c37b3304a67`（未提交修改）；内容状态 `urn:ce:agent-deck:content-state:health-recovery:tasks:071fceb:97f44efc577cbf6e9eb60af020c79a3b0c111c3e6dd1f4e15fd90a46c4439e56`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REVIEW 角色；Method：分解评审，逐任务对照 `requirements.md`、两份 UX 文档、`architecture.md`（`071fceb`）和项目 L0–L4 验证矩阵，并逐一核对所列文件路径在代码库中是否存在。
- Evidence：`bash scripts/check-topic-docs.sh` PASS（退出 0、无输出）；`make check-whitespace` PASS；`git diff --check` PASS；路径核对发现 `cmd/agentdeck/extension.go`、`apps/macos/AgentDeckApp/Resources/Localizable.xcstrings`、`apps/macos/AgentDeckTests/MenuBarHealthTests.swift` 不存在（`cmd/agentdeck/acceptance_test.go` 被写成可选的新文件）。
- Completion gate：FAILED — L0 criterion 通过，review criterion 被 `TASKS-R1-F1` 至 `TASKS-R1-F6` 否证。
- 流程观察：`architecture.md` 的设计授权 gate `ad-hr-doc-arch-design-gate` 仍未关闭，导致 `ad-hr-doc-arch-design` 在交付后无法关闭；`tasks.md` 的设计 gate `ad-hr-doc-tasks-design-gate` 也未关闭。两者都保持原状。
- Residual uncertainty：本轮没有逐条核对五个任务是否完整覆盖 requirements 的全部验收场景和 architecture 的全部不变式；已发现的归属和契约缺口足以判定 FAIL。修复后的复评会补上这一轮覆盖核对。

## Round 2 — 2026-09-22

## 📋 health-recovery / tasks.md 分解复评

📊 总体评分：8/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[`tasks.md:64`](../tasks.md) `TASKS-R1-F6` 仍未关闭（范围缩小）：对账表已经加上，但漏了菜单栏数据需求中的两项，却仍然断言「All requested fields … fully provisioned」。
- 处置：still open。
- 行为风险：菜单栏 UX 的「Data requirements for architecture」要求架构提供 **Effect**（bounded mutation summary，「Inventory + fingerprint are one truthful visible boundary」）和 **Partial commit**（`inventory_committed`，「Must say inventory changed and did not roll back」），并规定「must provision every element or return here with a reasoned refusal」。对账表没有这两行：表中的 `inventory_committed` 只出现在 CLI `error.details` 上，desktop wire 并不携带它。Task 4 的 required result 和字符串目录清单也都没有 effect 文案、没有「未回滚」文案。照此实现，菜单栏的 extensionStale 场景不会显示同步影响范围，syncIncomplete 场景也不会说明库存已提交、没有回滚，这与 UX 的「Partial commit is presented as rollback」验证项相冲突。
- 证据：`tasks.md:70-87` 的对账表；`tasks.md:415-442` 的 Task 4 required result；`ux/menubar-health-recovery.md:193-194,250`；`prototype/src/i18n.js` 中 `extension_stale_inventory.effect` 与 `extension_fingerprint_update_failed.effect` 两段按 reason 给出的文案。
💡 有界修复：在对账表中为 Effect 和 Partial commit 各加一行并写明处置，例如「由客户端按 reason 本地化派生：stale → effect 文案，fingerprint_update_failed 即表示已提交 → 未回滚文案；wire 不另加字段」，或者给出新增字段的方案。然后在 Task 4 的 required result、字符串目录清单和测试中加入这两段文案。

### 🟢 优点

- `TASKS-R1-F1` closed：Task 4 的 action 呈现（`tasks.md:419-435`）逐项对齐菜单栏 Action contract：只复制不执行，三种按 action 区分的双语文案，`retry`/null 无按钮，未知 key 不给按钮，复制后焦点不移动，通过 polite live region 播报 1.6 秒。
- `TASKS-R1-F2` closed：依赖图改为 1 → 2 → 3 → 4 → 5 串行。Task 1 独占共享管线：`doctor.Check` 新字段、`error.details` 投影（含 `inventory_committed`）、aggregate doctor 的 `next:`/`diagnose:`/`recovery:`/`effect:`/`manual prerequisite:` 渲染。Task 2 在 Task 1 之后接入，不再并行编辑同一批文件。
- `TASKS-R1-F3` closed：路径已对上真实位置（`AgentDeckApp/Localizable.xcstrings`、`DesktopCopy.swift`、`AgentDeckAppTests/MenuBarViewModelTests.swift`、`DesktopCopyTests.swift`），extension 命令归入 `main.go`，`acceptance_test.go` 标明是新文件，只运行的脚本移到了 Verification。
- `TASKS-R1-F4` closed：Task 3 以现有的 Go `HealthCheck` 和 Swift `DesktopHealthCheckV1`（`DesktopWire.swift:1001-1017`）为基础，只新增 5 个字段，保留 `Recovery`/`Count` 原有的类型和 tag，以及 Swift 端的 `recoveryCommand`/`count`。
- `TASKS-R1-F5` closed：Tasks 1–3 都写出了 wrapper 命令、受影响包、`-race` 和契约变更要求的 `scripts/run-go-test.sh ./...`；Task 4 走 `scripts/test-macos-app.sh`。
- 覆盖核对（Round 1 留待本轮）：Task 5 的 16 行覆盖 architecture 验收表全部行，也覆盖 requirements 的 12 个场景；原生 VoiceOver/键盘/复制行为以 PERFORMED/SIMULATED/BLOCKED 记录，满足 requirements 的原生验收要求。除上面的 Effect/Partial commit 外，没有发现 architecture 不变式缺少负责任务。

### 📝 总结

- 逐项处置：
  - closed：`TASKS-R1-F1`、`F2`、`F3`、`F4`、`F5`。
  - still open：`TASKS-R1-F6`（范围缩小到 Effect 与 Partial commit 两项）。
  - 没有 regressed 或新增的发现。
- Reviewed state：HEAD `071fceb5d008cfe611108be0faa826d48ca5ad88`；`docs/topics/health-recovery/tasks.md` blob `af8f744642002190e69c01012d455ea3e013efcc`；内容状态 `urn:ce:agent-deck:content-state:health-recovery:tasks:071fceb:268b325bfbf7cc44fa1c05eb9550fa878c015fd1d2b0faf8332ea0bf84125c43`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REREVIEW 角色；Method：逐条对照 Round 1 发现，核对 `DesktopWire.swift`、`internal/desktop/desktop.go`、菜单栏 UX 的 Data requirements 与 Action contract，并补做 Round 1 留下的覆盖核对。
- Evidence：`bash scripts/check-topic-docs.sh` PASS；`make check-whitespace` PASS；`git diff --check` PASS。
- Completion gate：FAILED — L0 criterion 通过，review criterion 被 `TASKS-R1-F6` 否证。
- 流程观察：本轮修复有 Beads 交接评论（codex）；`ad-hr-doc-arch-design-gate` 与 `ad-hr-doc-tasks-design-gate` 仍未关闭。

## Round 3 — 2026-09-22

## 📋 health-recovery / tasks.md 分解复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `TASKS-R1-F6` closed：
  - 对账表新增「Menu-bar Effect」和「Menu-bar Partial commit」两行（`tasks.md:85-86`），写明处置：客户端按 `reason` 本地化派生这两段文案，wire 只传稳定的 reason，不增加重复文本或布尔字段。理由成立：按 architecture，`extension_fingerprint_update_failed` 只在「库存已提交、指纹写入失败」时产生，reason 本身就表示已提交。
  - Task 4 在 required result、本地化清单（`Localizable.xcstrings` 与 `DesktopCopy.swift`）以及 `MenuBarViewModelTests.swift`、`DesktopCopyTests.swift` 的断言中都加入了 effect 和「未回滚」文案，中英文与 `prototype/src/i18n.js` 中已评审的文案逐字一致。
- 前两轮关闭的 `TASKS-R1-F1`–`F5` 在当前内容中没有回归；5 个任务、依赖链 1→2→3→4→5 和各任务的验证命令保持不变。

### 📝 总结

- 逐项处置：
  - `TASKS-R1-F1`–`F6` 全部 closed。
  - 没有 still open、regressed 或新增的发现。
- Reviewed state：HEAD `071fceb5d008cfe611108be0faa826d48ca5ad88`。评审时 `tasks.md` blob 为 `6820d362dec03fc57bf640ddd1bb39b06f106b5e`；评审后按项目规则做了状态同步，只改两处——文档矩阵 `tasks.md` 行的 Review 单元格勾选，以及交接段最后一句——同步后 blob 为 `0b2230b39f14eabde151590cc35d544d592ffd1f`，证据绑定到这个最终状态。内容状态 `urn:ce:agent-deck:content-state:health-recovery:tasks:071fceb:3b80e7efd39128bc04e40c53be192110d91ed28d50052ea39b810334b71cc37c`。
- Reviewer：Claude Code（claude-opus-5-5），正式 REREVIEW 角色；Method：对照 Round 2 的遗留发现，核对菜单栏 UX 的 Data requirements（`ux/menubar-health-recovery.md:193-194,250`）与原型文案，并通读 Task 4 修订段确认没有回归。
- Evidence：`bash scripts/check-topic-docs.sh` PASS；`make check-whitespace` PASS；`git diff --check` PASS（状态同步前后都运行过）。
- Completion gate：VERIFIED — 见本轮写入的 CEv1 l0/review 证据与门禁复查。
- 后续边界：按本文件「Review boundary」，PASS 允许创建这 5 个实现 Beads 任务及其开发授权 gate；它不授权实现、提交或推送。
- 流程观察：`ad-hr-doc-arch-design-gate` 与 `ad-hr-doc-tasks-design-gate` 仍未关闭，已交付的 `ad-hr-doc-arch-design` 因此无法关闭。
