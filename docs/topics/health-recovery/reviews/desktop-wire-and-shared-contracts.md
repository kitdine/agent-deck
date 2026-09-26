---
status: active
topic: health-recovery
subject: desktop-wire-and-shared-contracts
---

# Health Recovery Desktop Wire and Shared Contracts Review

## Round 1 — 2026-09-24

## 📋 health-recovery / desktop-wire-and-shared-contracts 评审

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**DWSC-R1-F1** [internal/desktop/fixtures_test.go:121; desktop/fixtures/v1/snapshot-complete.json:7852] Task 3 没有交付包含新增健康字段的 schema v1 canonical JSON fixture，也没有让 fixture producer 生成带分类的锁或扩展行。
- 行为风险：现有 Go fixture 比对及 Swift fixture 解码即使全部通过，也只覆盖缺少新字段的旧形状；`resource`、`reason`、`action_kind`、`diagnostic_command` 与 `manual_prerequisite` 在真实 Go producer 输出和 Swift 消费方之间的契约漂移无法被 canonical fixture 检出。Task 3 要求的新字段样本与旧样本兼容性尚未形成一条可重现的生产者到消费者证据链。
- 证据：`rg -n '"(resource|reason|action_kind|diagnostic_command|manual_prerequisite)"' desktop/fixtures/v1/*.json` 无匹配；本任务 diff 没有 JSON fixture 变更，只修改 `desktop/fixtures/v1/verify.swift`。`TestCanonicalFixturesAreReproducibleProducerOutput` 确实逐字比较四份 fixture 与 producer，但 `buildCompleteFixture` 等现有场景没有建立带分类的锁或扩展健康行；新增 Swift 解码测试使用手写 JSON。`tasks.md:360-361,383-384` 明确要求更新 v1 fixture 中的新增字段并验证旧 fixture 的兼容解码。
- 💡 修复：在 canonical fixture producer 中构造确定性的锁或扩展健康原因，生成并提交至少一份包含新增字段的 v1 JSON fixture；保留旧形状 fixture 作为兼容样本。让 Go 生产者逐字校验、Swift fixture 解码和隐私断言覆盖这两种形状，而不手写替代生产者输出。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- `HealthCheck` 已添加任务要求的五个可选字段，并从 doctor 检查映射；Go 单元测试涵盖锁、扩展和健康行的序列化形状。
- Swift 模型列出已批准的资源、原因和七项手动前提键；未知 `resource`、`reason` 或 `action_kind` 会退为 warning、清空动作与恢复命令。手写输入测试覆盖四种锁原因和九种扩展原因。

### 📝 总结

- Reviewed state：`feature/health-recovery` HEAD `3395d8b5e03ef57ea94428668bdc9320207b6e6a` + Task 3 六个已修改文件；`sha256(git diff --binary 3395d8b5... -- <本任务六个文件>) = d5a10f2c837cfc56dd96c2bd8bd4f3acc1f0ea353d97d05f2251ac60e4b89294`。并行的 `.agent-instructions/` 与 Hook 修改不在本任务范围。
- Reviewer：codex。Method：核对 Task 3 与 architecture/UX 契约、Go/Swift diff、CodeGraph 的 canonical fixture producer/consumer 路径和现有 JSON 内容。Scope：Go wire、Swift 解码、fixture 与验证入口；产品代码、测试、配置保持只读。
- Evidence：Beads `ad-hr-desktop-wire-and-shared-contracts-dev` 为 `in_review`，Antigravity 的实现交接指向同一候选指纹；所有 v1 JSON fixture 均无新增健康字段，且六文件 diff 不含 JSON fixture。交接报告 L2 Go、Swift XCTest 和全仓 Go 通过，但这些检查未证明新增字段进入 canonical fixture；发现决定性缺口后本轮未扩大运行套件。
- 结论依据：Task 3 的明确交付物和跨语言 fixture 验证缺失。`tasks.md` Task 3 的 Dev 与 Review 两格仍未勾选，符合当前未完成状态。
- Completion gate：NOT_VERIFIED；CEv1 WorkUnit `health-recovery:desktop-wire-and-shared-contracts` 已建立五项必需准则，对 ContentState `urn:ce:agent-deck:content-state:health-recovery-desktop-wire-and-shared-contracts:review:d5a10f2c837cfc56dd96c2bd8bd4f3acc1f0ea353d97d05f2251ac60e4b89294` 的门禁查询为 0/5。本轮 FAIL 不写通过证据。

### 下一步指令

`修复：health-recovery / reviews/desktop-wire-and-shared-contracts.md / DWSC-R1-F1`

## 修复记录 — 2026-09-24

针对 Round 1 的阻塞缺陷 `DWSC-R1-F1` 实施有界修复。

### 🛠️ 缺陷修复详情

- **DWSC-R1-F1 关闭**：
  - [`internal/desktop/fixtures_test.go`](../../../../internal/desktop/fixtures_test.go) 在 complete fixture 的真实 producer setup 中创建确定性的 legacy `state.lock`，由 `doctor.Service` 分类为 `lock_legacy`，再经 `desktop.Service` 生成 wire 数据；fixture 没有手写健康行。
  - [`desktop/fixtures/v1/snapshot-complete.json`](../../../../desktop/fixtures/v1/snapshot-complete.json) 现在包含 producer 生成的 `resource: state`、`reason: lock_legacy`、`action_kind: manual_prerequisite` 和 `manual_prerequisite: prereq_legacy_lock_removal`。`snapshot-legacy.json` 保持缺少这些 additive 字段，继续作为向后兼容样本。
  - [`apps/macos/AgentDeckTests/DesktopWireTests.swift`](../../../../apps/macos/AgentDeckTests/DesktopWireTests.swift) 直接从 canonical complete fixture 断言分类字段和 fail-closed 安全形状；隐私断言也改为检查这条 producer 生成的健康行。旧形状解码断言明确验证五个 additive 字段均为 `nil`。

### 📊 验证证据

- fixture 生成与逐字生产者校验：`AGENTDECK_UPDATE_FIXTURES=1 GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/desktop -run TestCanonicalFixturesAreReproducibleProducerOutput`（PASS）；随后无更新模式的受影响包测试再次逐字校验 fixture（PASS）。
- 受影响 Go 包：`GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/desktop/...`（PASS）。
- macOS 原生套件：`bash scripts/test-macos-app.sh`（PASS；Shared 81、App 121、Widget 45，0 失败，App 既有条件测试跳过 1 项）。直接执行入口因脚本未设置 executable bit 退出 126；显式使用其 Bash shebang 后完成同一脚本。
- 全仓 Go：`GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`（PASS；完整日志 `/private/tmp/agentdeck-dwsc-r1-f1-go-all.log`）。
- 范围空白：`git diff --check -- internal/desktop/fixtures_test.go apps/macos/AgentDeckTests/DesktopWireTests.swift desktop/fixtures/v1`（CLEAN）。

### 📌 候选状态

- 候选分支：`feature/health-recovery` HEAD `3395d8b5e03ef57ea94428668bdc9320207b6e6a`。
- Task 3 候选指纹：`sha256(git diff --binary 3395d8b5... -- <Task 3 八个文件>) = d134a5796bb98ba63749654be1e03ca9d6e65b8d65de3f2053ddcad439982703`。
- `DWSC-R1-F1` 已修复，等待独立复评；本修复记录不签发 PASS。

### 下一步指令

`复评：health-recovery / reviews/desktop-wire-and-shared-contracts.md`

## Round 2 — 2026-09-24

## 📋 health-recovery / desktop-wire-and-shared-contracts 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- DWSC-R1-F1 已关闭：complete fixture 的真实 producer setup 注入确定性的 legacy state lock，Go doctor 分类后经 Desktop Wire 输出 `resource`、`reason`、`action_kind` 与安全前提键；JSON fixture 逐字与 producer 输出一致。
- Swift XCTest 直接解码 canonical complete fixture 并断言 `lock_legacy` 的分类和安全形状；旧形状检查缺少新增字段时继续解码为 `nil`。Go 映射与 Swift 的已知令牌、未知令牌回退及隐私测试保持有效。

### 📝 总结

- Finding disposition：DWSC-R1-F1 已关闭；无仍开放、回归或新增的本任务发现。
- Reviewed state：`feature/health-recovery` HEAD `3395d8b5e03ef57ea94428668bdc9320207b6e6a` + Task 3 八个代码、测试与 fixture 文件及 `tasks.md` Review 状态；`sha256(git diff --binary 3395d8b5... -- <上述九个文件>) = ed829e1cd45581fff2259bd6b2c598d3d83e34e52589bd3349c72c7599416301`。本评审记录及并行的仓库规则/Hook 修改不计入指纹。
- Reviewer：codex。Method：逐项核对 Round 1 发现、fixture producer 与真实 JSON、Go wire 映射、Swift canonical/legacy 解码断言，并运行聚焦 Go fixture 校验与 macOS XCTest。Scope：Task 3 Go/Swift wire、v1 fixture 及其验证入口；产品代码、测试、配置只读。
- Evidence：`GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/desktop/... -run '^TestCanonicalFixturesAreReproducibleProducerOutput$' -count=1` PASS（日志 `agentdeck-go-test.pfhnzl`）；受影响包 `scripts/run-go-test.sh ./internal/desktop/...` PASS（日志 `agentdeck-go-test.eni5qT`）；修复交接的同代码状态全仓 Go 日志 `/private/tmp/agentdeck-dwsc-r1-f1-go-all.log` PASS。`bash scripts/test-macos-app.sh` 在允许脚本的 `/bin/ps` 隔离检查后 PASS（Shared 81、App 121〔1 条条件跳过〕、Widget 45，均 0 失败；日志 `/private/tmp/agentdeck-dwsc-r1-rereview-swift.log`）。首次沙箱调用因 `/bin/ps` 被拒绝而在测试前退出 126，不计作 XCTest 失败。最终 Go 格式、`git diff --check` 和 `make check-whitespace` 均无问题。
- 结论依据：已批准的新字段现在通过 producer 生成的 canonical v1 样本跨越 Go→JSON→Swift 边界；旧形状仍可解码。Task 4 的菜单栏呈现及原生交互验收不属本 Task 3。
- Completion gate：VERIFIED（5/5）。CEv1 WorkUnit `health-recovery:desktop-wire-and-shared-contracts` 对 ContentState `urn:ce:agent-deck:content-state:health-recovery-desktop-wire-and-shared-contracts:rereview:ed829e1cd45581fff2259bd6b2c598d3d83e34e52589bd3349c72c7599416301` 的精确目标门禁查询确认五项必需准则均有目标绑定的 pass evidence，无缺失、失败或未决候选影响；WorkUnit 已回读为 Review PASS、门禁 VERIFIED。此结果不代替提交授权。

Task checkpoint：`ad-hr-desktop-wire-and-shared-contracts-dev`；内容状态 `ed829e1cd45581fff2259bd6b2c598d3d83e34e52589bd3349c72c7599416301`；门禁 VERIFIED。
提交建议：单独提交 Task 3 的九个 Go、Swift、fixture 与 `tasks.md` 文件及本评审记录；排除并行的 `.agent-instructions/` 与 Hook 改动，提交前核对贡献者、完整暂存范围、消息及 SSH 签名。
推送建议：取得单独推送授权、完成并验证签名提交后，确认远端目标，再推送 `feature/health-recovery` 到 `origin/feature/health-recovery`。

### 下一步指令

`开发：health-recovery / menubar-health-presentation`

WORKFLOW_WORKSPACE: agent-deck.health-recovery
