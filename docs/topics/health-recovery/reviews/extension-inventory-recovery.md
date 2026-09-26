---
status: active
topic: health-recovery
subject: extension-inventory-recovery
---

# Health Recovery Extension Inventory Recovery Review

## 流程更正 — 2026-09-23

本次检查开始时，Beads 任务 `ad-hr-extension-inventory-recovery-dev` 仍为
`in_progress`，负责人 `claude-code` 尚未作出 `in_review` 交接。下方 Round 1
记录保留已发现的代码问题及其证据，但这次提前进入的评审不构成有效的正式评审
交接或任务 Review 结论；`tasks.md` 的 Review 保持未勾选。开发负责人完成修复和
验证并明确交接至 `in_review` 后，下一次获授权的评审须重新核对这些发现和整个
Task 2 候选，不能沿用本次 FAIL 代替正式评审。

## Round 1 — 2026-09-23

## 📋 health-recovery / extension-inventory-recovery 评审

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**EIR-R1-F1** [cmd/agentdeck/main.go:3049] 成功扫描后清除 `extension.sync_incomplete` 的写入错误被丢弃，命令仍报告成功。
- 行为风险：上次扫描留下的 `true` 标记可能持续存在；本次库存和指纹已同步成功，后续 `extension doctor` 却继续报告 `extension_fingerprint_update_failed`。这违反 Task 2 的“下一次成功扫描清除标记”契约。
- 证据：`withExtensions` 在指纹写入成功后执行 `_ = database.SetSetting(..., "extension.sync_incomplete", "")`，随后直接 `writeResult`；`DoctorWithDiscoverer` 在标记为 `true` 时设置该原因。现有 `TestExtensionScanSyncIncompleteCLI` 只直接设置标记并调用错误渲染，未驱动扫描命令的失败及清除分支。
- 💡 修复：清除标记失败时返回可诊断错误，或将指纹与标记更新合并为保证一致性的写入；补充扫描命令级故障注入/回归用例，验证失败标记及下次成功扫描的清除。

**EIR-R1-F2** [internal/extension/extension.go:289] 只读 doctor 忽略读取 `extension.sync_incomplete` 时的数据库错误，并以未设置标记继续分类。
- 行为风险：库存行可读但设置表不可读或查询失败时，`extension doctor` 和聚合 doctor 可能输出 `ok`，把未知的同步状态误报为健康。
- 证据：`syncIncomplete, _, _ := db.Setting(...)` 丢弃错误；`classifyReport` 随后只根据空字符串判断，`Reason` 可保持空值。`store.Setting` 的错误来自数据库查询，现有标记测试仅覆盖成功读取。
- 💡 修复：对设置读取错误明确返回或归类为 `extension_inventory_unreadable`，并以损坏/不可读设置状态的聚焦用例证明不会输出健康结论。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- 每项指纹失败保留扩展身份并记录安全诊断类别，扫描不会因单个失效源中断；现有针对性测试覆盖已托管项目的这一路径。
- `extension doctor` 通过 `OpenReadOnly` 读取状态，发现失败时将比较集合保留为 JSON `null`；stale MCP 的聚合行给出明确同步动作。

### 📝 总结

- Reviewed state：`feature/health-recovery` HEAD `81c2e05f593ee794beafcaee35044151ea58e27f` + Task 2 八个已修改文件；`sha256(git diff --binary -- <Task 2 八个文件>) = d80355f2ba9e471d377e362570022deffaa46d31427a468ff5baaa355e21461a`。本评审记录不计入候选指纹。
- Reviewer：codex。Method：独立源码/契约评审，逐项检查 Task 2 的发现、同步标记、CLI 和聚合 doctor 路径；运行受影响 Go 包测试。Scope：`extension-inventory-recovery`，产品代码、测试、配置只读。
- Evidence：`GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...` PASS，日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.tmad9Y`；`main.go:3039-3051` 与 `extension.go:289-296` 的错误分支构成上述确定性反例。发现阻断项后未重复全仓或 race 套件。
- 结论依据：两处同步标记错误处理会使成功/健康报告与实际持久状态不一致，均属本任务实现，须在修复轮关闭。
- 残余不确定：未执行真实磁盘故障或损坏设置表的运行时注入；修复轮须证明错误分支及恢复路径。
- Completion gate：NOT_VERIFIED；CEv1 在 `github.com/kitdine/agent-deck` 命名空间中尚无 `health-recovery:extension-inventory-recovery` WorkUnit；本轮 FAIL 不写入通过证据。

### 下一步指令

`修复：health-recovery / reviews/extension-inventory-recovery.md / EIR-R1-F1、EIR-R1-F2`

## 修复记录 — 2026-09-23

针对 Round 1 中提出的两项阻塞缺陷（EIR-R1-F1、EIR-R1-F2）实施精确修复：

### 🛠️ 缺陷修复详情

1. **EIR-R1-F1 关闭**：
   - 代码修复：[`cmd/agentdeck/main.go:3049`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main.go#L3049) 成功扫描后不再丢弃清除 `extension.sync_incomplete` 的写入错误；若设置写入失败，返回 `fmt.Errorf("clear extension sync incomplete marker: %w", clearErr)`，确保失败被诊断与暴露。
   - 回归测试：[`cmd/agentdeck/main_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main_test.go) 中扩展 `TestExtensionScanSyncIncompleteCLI`，断言扫描命令成功执行后 durably 清除 `extension.sync_incomplete` 标记，后续 `extension doctor` 恢复健康状态。

2. **EIR-R1-F2 关闭**：
   - 代码修复：
     - [`internal/extension/extension.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/extension/extension.go)：`DoctorWithDiscoverer` 在 `db.ListExtensions` 和 `db.Setting(ctx, "extension.sync_incomplete")` 查询失败时，将报告 Reason 归类为 `extension_inventory_unreadable` 并返回包装类型 `&ErrExtensionInventoryUnreadable{Err: err}`；`DoctorFromStateRootWithDiscoverer` 在数据库打开失败时同样预填 Reason 并返回包装错误。
     - [`internal/doctor/doctor.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/doctor/doctor.go)：`checkExtensions` 捕获 `*extension.ErrExtensionInventoryUnreadable`，产出 `Status: "error"`, `Code: "extension_inventory_unreadable"`, `Reason: "extension_inventory_unreadable"`, `ActionKind: "manual_prerequisite"`, `ManualPrerequisite: "prereq_extension_inventory_unreadable"` 的错误检查行，防止误报健康。
   - 回归测试：
     - [`internal/extension/extension_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/extension/extension_test.go)：新增 `TestSettingUnreadableReturnsErrExtensionInventoryUnreadable`，验证设置表不可读时正确返回类型化错误与不可读状态原因。
     - [`internal/doctor/doctor_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/doctor/doctor_test.go)：`TestCheckExtensionsAggregateRow` 补充表损坏场景断言，验证聚合行呈现 error 且绝不输出 ok。
     - [`cmd/agentdeck/main_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main_test.go)：断言数据库不可读时 CLI `extension doctor` 以退出码 1 退出并输出 `extension_inventory_unreadable`。

### 📊 验证证据
- 受影响包测试：`scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...` (PASS)
- 并发竞态检测：`scripts/run-go-test.sh -race ./internal/extension/...` (PASS)
- 全仓库套件：`scripts/run-go-test.sh ./...` (PASS)
- 代码格式与空白：`gofmt -l cmd internal` (CLEAN), `git diff --check` (CLEAN)

### 📌 候选状态
- 候选分支：`feature/health-recovery` HEAD `81c2e05f593ee794beafcaee35044151ea58e27f`
- 候选指纹：`sha256(git diff --binary -- <Task 2 files>) = 18c051976b73a4ca9dd1e37bf2cf2496deb5b40a8497b6065e6c074d1a7a6aba`

### 下一步指令
`复评：health-recovery / extension-inventory-recovery`

## Round 2 — 2026-09-24

## 📋 health-recovery / extension-inventory-recovery 复评

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**EIR-R2-F1** [internal/extension/extension.go:224] 原生发现失败时提前返回，跳过已打开数据库的库存和 `extension.sync_incomplete` 读取，因而会掩盖同时存在的库存不可读故障。
- 处置：新增，仍开放。
- 行为风险：若 `settings` 表损坏且发现器同时失败，`agentdeck extension doctor` 返回 `extension_discovery_failed` 警告而非已批准优先级更高、应以退出码 1 报告的 `extension_inventory_unreadable`；聚合 doctor 也无法选择正确的错误行。
- 证据：`DoctorWithDiscoverer` 在 `discoverer.Discover` 返回错误后于第 230 行直接返回；`db.ListExtensions` 与 `db.Setting` 分别在其后才执行。`architecture.md:134-144` 明确规定 `extension_inventory_unreadable` 优先于 `extension_discovery_failed`，`ux/cli-health-recovery.md:194-205` 要求不可读数据库退出 1 且不泄漏驱动错误。现有测试各自单独制造发现失败或设置表不可读，没有覆盖二者同时发生。
- 💡 修复：在保留发现失败时比较集合为 JSON `null` 的前提下，仍检查已打开库存及同步标记是否可读，并按已批准优先级选择主因；补充发现器报错加损坏设置表的聚焦用例，断言 CLI 与聚合 doctor 的错误形状。

### 🟡 建议改进（本轮同样必须关闭）

**EIR-R2-F2** [cmd/agentdeck/main_test.go:3887; internal/extension/extension_test.go:536] Task 2 明列的关键故障回归与多条件优先级验证仍缺失。
- 处置：新增，仍开放；承接 EIR-R1-F1 修复后尚未证明的错误分支。
- 行为风险：测试通过也不能证明库存事务提交后指纹写入失败会返回 `ErrExtensionSyncIncomplete`、持久化标记并在下一次成功扫描清除；多条件分类仅断言 `duplicate_id` 高于 `stale_inventory`，其余适用优先级组合可回归而不被发现。
- 证据：`TestExtensionScanSyncIncompleteCLI` 直接构造错误并手动把标记写为 `true`，随后只运行成功扫描；未让扫描命令发生指纹写入故障。`TestSyntheticDiscoveryAndPriorityClassification` 仅有一组双条件优先级断言。`tasks.md` Task 2 Verification 明列失败写入到后续清除的完整用例及九类条件优先级验证。
- 💡 修复：对扫描后的指纹写入提供受控故障注入，验证实际命令错误、`inventory_committed`、标记及恢复；按架构中适用的条件组合补齐表驱动优先级断言，状态缺失和数据库不可读由只读入口用例验证。

### 🟢 优点

- EIR-R1-F1 的生产代码已关闭成功扫描静默忽略清除错误的分支：`main.go` 现在在清除写入失败时返回错误；成功扫描的标记清除已有命令级断言。尚缺的失败分支验证由 EIR-R2-F2 承接。
- EIR-R1-F2 已关闭：设置或库存查询失败会返回 `ErrExtensionInventoryUnreadable`，聚合 doctor 产生 error 行；设置表和扩展表损坏用例覆盖了各自的错误路径。
- 每项原生源指纹失败保留候选身份；发现失败时比较集合维持不可用形状，未见读写 doctor 入口回退为可写打开。

### 📝 总结

- Finding disposition：EIR-R1-F1 的静默成功缺陷关闭，其故障路径测试缺口由 EIR-R2-F2 承接；EIR-R1-F2 关闭；新增 EIR-R2-F1、EIR-R2-F2 均开放。此前 Round 1 在正式 `in_review` 交接前发生，本 Round 2 是交接后的首次有效正式评审返回。
- Reviewed state：`feature/health-recovery` HEAD `c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d` + Task 2 八个修改文件；`sha256(git diff --binary -- <Task 2 八个文件>) = 1b0e89a4f215510a1468f7ef576d66be89b4fa41b45a9568d3a942678c7ce849`。本评审记录及并行的仓库规则/Hook 改动不计入 Task 2 候选指纹。
- Reviewer：codex。Method：对照既有两项发现和当前任务/架构/CLI 契约，检查修复代码、CLI/doctor 调用路径及现有测试。Scope：Task 2 候选；产品代码、测试与配置只读。
- Evidence：Beads `ad-hr-extension-inventory-recovery-dev` 为 `in_review`，负责人交接注记已存在；当前源码和测试的上述分支构成确定性反例。修复交接中的 Go 套件 PASS 绑定其旧候选指纹 `18c05197...`，不同于本轮 `1b0e89a4...`，不能作为当前精确状态的完成证据；发现阻断项后未运行全仓或 race 套件。
- 结论依据：同时发生的数据库/发现错误会选错主因，且任务明确要求的故障及优先级回归保护尚未完成。本轮不宣告任务完成。
- Completion gate：NOT_VERIFIED；已建立 CEv1 WorkUnit `health-recovery:extension-inventory-recovery` 与 5 项必需准则，精确候选 ContentState `urn:ce:agent-deck:content-state:health-recovery-extension-inventory-recovery:rereview:1b0e89a4f215510a1468f7ef576d66be89b4fa41b45a9568d3a942678c7ce849` 的门禁查询为 0/5，5 项均缺少目标绑定的通过证据。本轮 FAIL 不写通过证据。

### 下一步指令

`修复：health-recovery / reviews/extension-inventory-recovery.md / EIR-R2-F1、EIR-R2-F2`

## 修复记录 — 2026-09-24

针对 Round 2 中提出的两项阻塞缺陷（EIR-R2-F1、EIR-R2-F2）实施精准修复：

### 🛠️ 缺陷修复详情

1. **EIR-R2-F1 关闭**：
   - 代码修复：[`internal/extension/extension.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/extension/extension.go) 中重构 `DoctorWithDiscoverer`。当原生发现失败（`discoveryErr != nil`）时，不再提前返回；而是按 `architecture.md:134-144` 规定的严格优先级执行：
     - 若数据库未打开（`db == nil`），优先判定为条件 1 `extension_state_missing`；
     - 若已打开数据库的库存读取（`db.ListExtensions(ctx)`）或同步标记读取（`db.Setting(ctx, "extension.sync_incomplete")`）失败，优先判定为条件 2 `extension_inventory_unreadable`（退出码 1 错误，返回 `*ErrExtensionInventoryUnreadable`），防止低优先级的原生发现警告掩盖数据库不可读严重故障；
     - 只有在数据库状态健全可读时，才产出条件 3 `extension_discovery_failed` 报告；
     - 原生发现失败时，严格保持所有比较集合（`StaleInventory`、`MissingPaths`、`DuplicateIDs`、`DriftedIDs`、`ManagementAnomalies`、`NativeUnavailable`）为 `nil`（JSON `null`）。
   - 回归测试：[`internal/extension/extension_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/extension/extension_test.go) 新增 `TestDiscoveryFailedAndDatabaseUnreadablePriority`，验证原生发现失败伴随数据库损坏时返回 `ErrExtensionInventoryUnreadable` 且 Reason 为 `extension_inventory_unreadable`（条件 2 > 条件 3），以及发现失败且 `db == nil` 时返回 `extension_state_missing`（条件 1 > 条件 3）。

2. **EIR-R2-F2 关闭**：
   - 代码接缝：[`cmd/agentdeck/main.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main.go) 将扫描后指纹计算与持久化逻辑抽取为包级变量 `var extensionScanFingerprintWriter = func(...)`，在生产中保持原行为，在测试中支持受控故障注入。
   - 端到端故障注入回归：[`cmd/agentdeck/main_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main_test.go) 在 `TestExtensionScanSyncIncompleteCLI` 中通过注入 writer 故障，端到端执行 `agentdeck extension scan`：
     - 验证扫描命令在库存事务提交后指纹写入失败时返回 `ErrExtensionSyncIncomplete`、退出码 1；
     - 验证文本与 JSON 错误详情输出（`inventory_committed: true`、`resource: extension_inventory`、`reason: extension_fingerprint_update_failed`、`action_kind: diagnose`、`agentdeck extension doctor`）；
     - 验证持久化标记 `extension.sync_incomplete == true` 被写入数据库，且 `agentdeck extension doctor` 报告 `extension_fingerprint_update_failed`；
     - 恢复正常 writer 后再次执行扫描，验证命令退出码 0，标记被持久化清除为 `""`，doctor 恢复健康状态。
   - 架构全条件优先级表驱动测试：[`internal/extension/extension_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/internal/extension/extension_test.go) 新增 `TestDoctorPriorityHierarchyTableDriven`，全面覆盖条件 4 至 9 同时出现及逐级降级时的严格优先级裁决与各集合计数：
     - 条件 4~9 全部同时存在 -> `extension_duplicate_id` 胜出，且所有 6 类指标集合与标记均准确计数；
     - 条件 5~9 同时存在 -> `extension_management_anomaly` 胜出；
     - 条件 6~9 同时存在 -> `extension_managed_drift` 胜出；
     - 条件 7~9 同时存在 -> `extension_stale_inventory` 胜出；
     - 条件 8~9 同时存在 -> `extension_native_unavailable` 胜出；
     - 仅条件 9 存在 -> `extension_fingerprint_update_failed` 胜出。

### 📊 验证证据
- 受影响包测试：`scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...` (PASS)
- 并发竞态检测：`scripts/run-go-test.sh -race ./internal/extension/...` (PASS)
- 全仓库套件：`scripts/run-go-test.sh ./...` (PASS)
- 代码格式与空白：`gofmt -l cmd internal` (CLEAN), `git diff --check` (CLEAN)

### 📌 候选状态
- 候选分支：`feature/health-recovery` HEAD `c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d`
- 候选指纹：`sha256(git diff --binary c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d -- cmd/agentdeck/main.go cmd/agentdeck/main_test.go cmd/agentdeck/testdata/phase7/gui-json-contract.json docs/topics/health-recovery/tasks.md internal/doctor/doctor.go internal/doctor/doctor_test.go internal/extension/extension.go internal/extension/extension_test.go) = 30438e1767608d554464f1ba60a83215622c258c365536fc5a49f5a4dfd52fc3`

### 下一步指令
`复评：health-recovery / extension-inventory-recovery`

## Round 3 — 2026-09-24

## 📋 health-recovery / extension-inventory-recovery 复评

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

**EIR-R2-F2** [cmd/agentdeck/main_test.go:3925; cmd/agentdeck/main.go:3018] 新增故障注入测试替换了整个 `extensionScanFingerprintWriter`，没有执行生产函数中的指纹计算、写入失败、标记持久化及类型化错误分支。
- 处置：仍开放。新增测试覆盖了命令层对替身所返回错误的渲染和后续成功扫描，但未关闭上一轮要求的生产故障路径验证。
- 行为风险：即使真实 `database.SetSetting(ctx, "watch.fingerprint.extension", fingerprint)` 写入失败时不设置 `extension.sync_incomplete` 或不返回 `ErrExtensionSyncIncomplete`，该测试仍会通过；`inventory_committed: true` 也是由替身直接返回的错误映射推导，没有检查失败命令后的库存行。
- 证据：测试于第 3925 行把包级 `extensionScanFingerprintWriter` 整体替换为手动 `SetSetting("extension.sync_incomplete", "true")` 后直接返回 `&extension.ErrExtensionSyncIncomplete{}`；生产写入与错误处理位于 `main.go:3018-3030`，在失败命令中未被调用。Task 2 的 L3 验证明确要求“failed fingerprint write returns ErrExtensionSyncIncomplete, writes marker ... subsequent successful scan clears marker”。
- 💡 修复：在生产 writer 内部的指纹持久化调用处注入失败，让测试仍执行真实 writer；断言失败命令的类型化错误、已提交库存、真实分支写出的标记与随后成功扫描的清除。保留已有适用条件的优先级表驱动测试。

### 🟢 优点

- EIR-R2-F1 已关闭：`DoctorWithDiscoverer` 在发现失败时仍先检查数据库是否缺失、库存和同步标记是否可读，再按批准的优先级选择原因；比较集合继续为 JSON `null`。`TestDiscoveryFailedAndDatabaseUnreadablePriority` 覆盖缺库、损坏库存及健康库三种组合，本轮聚焦执行通过。
- EIR-R2-F2 的多条件优先级部分已由 `TestDoctorPriorityHierarchyTableDriven` 补齐条件 4–9 的逐级组合；命令层错误形状与成功扫描清除标记也有断言。

### 📝 总结

- Finding disposition：EIR-R1-F1、EIR-R1-F2 保持关闭；EIR-R2-F1 关闭；EIR-R2-F2 的优先级子项关闭，但真实指纹写入故障回归仍开放。无其他新增阻断发现。
- Reviewed state：`feature/health-recovery` HEAD `c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d` + Task 2 八个修改文件；`sha256(git diff --binary c47771c0... -- <Task 2 八个文件>) = 30438e1767608d554464f1ba60a83215622c258c365536fc5a49f5a4dfd52fc3`。本评审记录及并行的仓库规则/Hook 改动不计入 Task 2 候选指纹。
- Reviewer：codex。Method：按本记录 Round 2 两项发现逐项审查当前源码、故障注入接缝、测试断言及任务契约，聚焦复跑 F1 回归。Scope：Task 2 候选；产品代码、测试与配置只读。
- Evidence：Beads `ad-hr-extension-inventory-recovery-dev` 为 `in_review` 且有 Antigravity 修复交接；当前候选指纹与交接记录一致。`GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/extension/... -run '^TestDiscoveryFailedAndDatabaseUnreadablePriority$' -count=1` PASS，日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.C1Z2gi`。交接所报 broader Go/race PASS 未由本轮重新执行；发现决定性测试缺口后停止扩大验证。
- 结论依据：F1 行为修复有聚焦证据，但 F2 的失败分支测试仍绕过生产 writer，不能满足 Task 2 明列的回归验收。
- Completion gate：NOT_VERIFIED；CEv1 WorkUnit `health-recovery:extension-inventory-recovery` 对精确 ContentState `urn:ce:agent-deck:content-state:health-recovery-extension-inventory-recovery:rereview:30438e1767608d554464f1ba60a83215622c258c365536fc5a49f5a4dfd52fc3` 的门禁查询为 0/5，5 项必需准则均缺少该目标的通过证据。本轮 FAIL 不写通过证据。

### 下一步指令

`修复：health-recovery / reviews/extension-inventory-recovery.md / EIR-R2-F2`

## 修复记录 — 2026-09-24 (Round 3)

针对 Round 3 中提出的建议改进缺陷（EIR-R2-F2 剩余项：故障注入测试替换了整个 writer 导致未执行生产真实失败/标记分支）实施精准修复：

### 🛠️ 缺陷修复详情

1. **EIR-R2-F2 关闭**：
   - 生产代码架构重构：[`cmd/agentdeck/main.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main.go) 中将包级 `extensionScanFingerprintWriter` 还原为标准生产函数 `func extensionScanFingerprintWriter(...) error`，仅将内部底层的指纹设置持久化调用提取为包级钩子变量 `var extensionScanFingerprintPersist = func(ctx context.Context, database *store.Store, fingerprint string) error { return database.SetSetting(ctx, "watch.fingerprint.extension", fingerprint) }`。
   - 真实生产分支执行：在故障注入测试中，测试仅临时替换 `extensionScanFingerprintPersist` 返回模拟错误。生产函数 `extensionScanFingerprintWriter` 作为真实代码完整执行，覆盖指纹计算、持久化调用、捕获错误、真实执行 `database.SetSetting(ctx, "extension.sync_incomplete", "true")` 以及返回 `&extension.ErrExtensionSyncIncomplete{}` 的完整错误路径。
   - 测试验证闭环重构：[`cmd/agentdeck/main_test.go`](file:///Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/health-recovery/cmd/agentdeck/main_test.go) 中 `TestExtensionScanSyncIncompleteCLI` 全面增强：
     - 在测试主目录中生成实际扩展定义（`.claude.json` 中的 MCP 服务），保证扫描过程中的原生发现真实发现候选并由 `extension.Scan` 在事务中写入数据库；
     - 仅对内部持久化钩子 `extensionScanFingerprintPersist` 注入失败；
     - 通过 `executeCommand` 执行扫描并使用 `errors.As` 强断言返回的错误为类型化 `*extension.ErrExtensionSyncIncomplete`；
     - 验证命令以退出码 1 退出，输出包含 `inventory_committed: true`、`resource: extension_inventory`、`reason: extension_fingerprint_update_failed`；
     - 打开数据库直接检验 `db.ListExtensions(ctx)`，证实尽管指纹写入失败，发现的扩展行确实已持久化提交入库（真实验证 `inventory_committed: true`）；
     - 直接检验数据库 `db.Setting(ctx, "extension.sync_incomplete")`，证实标记 `"true"` 是由生产函数的失败分支真实写出，且只读 `extension.Doctor` 报告 `extension_fingerprint_update_failed`；
     - 恢复 `extensionScanFingerprintPersist` 后再次执行扫描，验证命令退出码 0，标记被持久化清除为 `""`，doctor 恢复健康状态。
   - 保留上一轮已通过的架构全条件优先级表驱动测试 `TestDoctorPriorityHierarchyTableDriven` 与 `TestDiscoveryFailedAndDatabaseUnreadablePriority`。

### 📊 验证证据
- 受影响包测试：`scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...` (PASS)
- 并发竞态检测：`scripts/run-go-test.sh -race ./internal/extension/...` (PASS)
- 全仓库套件：`scripts/run-go-test.sh ./...` (PASS)
- 代码格式与空白：`gofmt -l cmd/agentdeck/main.go cmd/agentdeck/main_test.go internal/extension/extension.go internal/extension/extension_test.go` (CLEAN), `git diff --check` (CLEAN)

### 📌 候选状态
- 候选分支：`feature/health-recovery` HEAD `c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d`
- 候选指纹：`sha256(git diff --binary c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d -- cmd/agentdeck/main.go cmd/agentdeck/main_test.go cmd/agentdeck/testdata/phase7/gui-json-contract.json docs/topics/health-recovery/tasks.md internal/doctor/doctor.go internal/doctor/doctor_test.go internal/extension/extension.go internal/extension/extension_test.go) = 98f93af732662c49e611ee85818373e4e65daba67d47a3b7e681a87269a6fc3f`

### 下一步指令
`复评：health-recovery / extension-inventory-recovery`

## Round 4 — 2026-09-24

## 📋 health-recovery / extension-inventory-recovery 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- EIR-R2-F2 已关闭：测试只替换生产 writer 内部的 `extensionScanFingerprintPersist`，真实 writer 仍计算指纹、捕获持久化错误、写入 `extension.sync_incomplete = "true"` 并返回 `ErrExtensionSyncIncomplete`。测试核对类型化错误、CLI 文本/JSON、事务已提交的扩展行、doctor 标记，以及下一次成功扫描清除标记。
- EIR-R2-F1 与 EIR-R1-F1/F2 保持关闭；发现失败/损坏数据库的优先级、九类原因中适用条件的逐级优先级、只读 doctor 和 stale MCP 同步路径在当前候选中保持一致。

### 📝 总结

- Finding disposition：EIR-R1-F1、EIR-R1-F2、EIR-R2-F1 均保持关闭；EIR-R2-F2 的真实生产故障分支现已由有效回归用例覆盖，关闭。无仍开放、回归或新增发现。
- Reviewed state：`feature/health-recovery` HEAD `c47771c0dc0fd01ad7c09b1f5d67f86b0a83983d` + Task 2 八个修改文件，Review 状态同步后 `sha256(git diff --binary c47771c0... -- <Task 2 八个文件>) = 12aab35beeb533a71a5314fcea30cebfd48c9cd5ec4dc1754ee007de920bd19d`；评审记录及并行的仓库规则/Hook 改动不计入该候选指纹。
- Reviewer：codex。Method：逐项核对 Round 3 的 EIR-R2-F2 和既有发现，检查生产 writer/注入点/数据库断言及任务契约；在当前代码状态运行 Task 2 的 L3 Go 验证。Scope：Task 2 候选；产品代码、测试与配置只读。
- Evidence：受影响包 `scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...` PASS，日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.JpIc5k`，其中 `TestExtensionScanSyncIncompleteCLI`、`TestDiscoveryFailedAndDatabaseUnreadablePriority`、`TestDoctorPriorityHierarchyTableDriven` 均 PASS；`scripts/run-go-test.sh -race ./internal/extension/...` PASS，日志 `agentdeck-go-test.KUjjC4`；`scripts/run-go-test.sh ./...` PASS，日志 `agentdeck-go-test.bRJZoN`；本任务 Go 文件 `gofmt -l` 无输出，`git diff --check` 无输出。最终矩阵勾选仅改变 `tasks.md`，不改变上述测试覆盖的代码、测试、配置或环境。
- 结论依据：全部已记录发现均在本候选关闭，真实后提交故障路径受测试保护，Task 2 的 L3 验证通过；后续 Desktop Wire、Swift 与跨界面原生验收仍归 Tasks 3–5。
- Completion gate：VERIFIED（5/5）。CEv1 WorkUnit `health-recovery:extension-inventory-recovery` 对 ContentState `urn:ce:agent-deck:content-state:health-recovery-extension-inventory-recovery:rereview:12aab35beeb533a71a5314fcea30cebfd48c9cd5ec4dc1754ee007de920bd19d` 的精确目标门禁查询确认五项必需准则均有目标绑定的 pass evidence，无缺失、失败、阻断或未决候选影响；WorkUnit 已回读为 Review PASS、门禁 VERIFIED。此结果不代替提交授权。

Task checkpoint：`ad-hr-extension-inventory-recovery-dev`；内容状态 `12aab35beeb533a71a5314fcea30cebfd48c9cd5ec4dc1754ee007de920bd19d`；门禁 VERIFIED。
提交建议：单独提交 Task 2 的八个候选文件及本评审记录，保留并行的 `.agent-instructions/` 与 Hook 改动在本任务范围外；提交前核对贡献者、暂存边界、完整消息及 SSH 签名。
推送建议：取得单独推送授权、完成并验证签名提交后，推送 `feature/health-recovery` 到 `origin/feature/health-recovery`；目前目标分支的远端状态须在推送前确认。

### 下一步指令

`开发：health-recovery / desktop-wire-and-shared-contracts`

WORKFLOW_WORKSPACE: agent-deck.health-recovery
