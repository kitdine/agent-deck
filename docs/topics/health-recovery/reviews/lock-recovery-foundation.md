---
status: active
topic: health-recovery
subject: lock-recovery-foundation
---

# Health Recovery Lock Recovery Foundation Review

## Round 1 — 2026-09-23

## 📋 health-recovery / lock-recovery-foundation 评审

📊 总体评分：4/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**LRF-R1-F1** [cmd/agentdeck/main.go:5031] `scan_lock` 为 `lock_legacy` 时，doctor 文本仍输出 `manual prerequisite: Remove state.lock only after that confirmation.`
- 行为风险：操作者被引导删除**另一把**锁（`state.lock`），而它可能属于正在运行的进程；这正是本 topic 要消除的"不安全删锁指引"。`lock_owner_unknown` 分支已按 `check.Resource` 区分文件名，`lock_legacy` 分支漏了。
- 证据：`renderDoctorText` 的 `case "lock_legacy":` 两行均为常量文本，不读取 `check.Resource`；`ux/cli-health-recovery.md` 的 legacy 规格以"所识别的锁文件"为对象，architecture `prereq_legacy_lock_removal` 语义为"remove only the identified legacy lock file"。
- 💡 修复：按 `check.Resource` 输出 `state.lock` / `scan.lock`（`next:` 行一并核对），并加 `scan_lock` legacy 文本用例。

**LRF-R1-F2** [internal/doctor/doctor.go:218] 新增 `scan_lock` 行改变了 desktop wire 快照，全仓测试失败；实现交接中"`scripts/run-go-test.sh ./...` PASS"与实测不符。
- 行为风险：Task 1 的 L3 验收（全仓套件）未满足；desktop canonical fixtures 与 snapshot-performance golden 不再是生产者输出，下游 Task 3 与 Swift fixture 消费方会基于过期样本工作。
- 证据：`scripts/run-go-test.sh ./...` → `internal/desktop` `TestCanonicalFixturesAreReproducibleProducerOutput/{snapshot-complete,snapshot-partial,snapshot-empty-client,snapshot-schema-ahead}.json` FAIL（`want "name": "scan_lock"`）；`cmd/agentdeck` `TestSnapshotPerformanceContractSyntheticCorpus` FAIL（`snapshot reference differs at byte 76237`，golden 中 `state_lock` 之后为 `schema`）。同一测试在 `main`（与本分支代码基线相同）PASS。
- 💡 修复：确认差异仅为新增 `scan_lock` 行后，用 `AGENTDECK_UPDATE_FIXTURES=1` 与 `AGENTDECK_UPDATE_SNAPSHOT_PERFORMANCE_FIXTURE=1` 重新生成 `desktop/fixtures/v1/*` 与 `cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json`；在 `tasks.md` Task 1 的 Files and ownership 中登记这两处 fixture；若 macOS 测试消费这些 fixture，按 Project Rules 补跑对应 Swift 测试。

**LRF-R1-F3** [cmd/agentdeck/main_test.go:3607] `TestQuotaRefreshLockYieldsUnknownResource` 失败：`--home` 不是全局参数，命令以 exit 2 退出，从未到达配额锁路径。
- 行为风险：任务要求的"quota refresh 产生 `resource: unknown`"断言实际未被验证；交接声称的验证不成立。
- 证据：`main_test.go:3609: exit = 2, want 1; stderr = unknown flag: --home`。
- 💡 修复：按 `desktop quota refresh` 实际参数构造调用（或使用现有 quota 测试的环境注入方式），使测试真正在配额锁被占用时断言 text/JSON 的 `resource: unknown`。

**LRF-R1-F4** [cmd/agentdeck/main_test.go:3645] `TestSchemaAheadPrecedenceOverStateBusy` 失败：状态锁被占且 schema 为 9999 时，`session list --format=json` 返回 `state_busy` 而非 `schema_ahead`。
- 行为风险：任务要求的"`schema_ahead` 优先于 `ErrLockContention`"在 CLI 层没有有效证据；可能是夹具与 `schemaAheadAtRest` 快照探针不匹配（store 层 `TestOpenReportsFutureSchemaAheadWhileStateLockIsHeld` 使用手建非 WAL 数据库并 PASS），也可能是 CLI 路径的真实优先级缺陷，目前无法区分。
- 证据：`main_test.go:3679: code = "state_busy", want 'schema_ahead'`（耗时 5.03s，即完整 `lockWait` 超时后探针未判定为 ahead）。
- 💡 修复：先诊断探针为何不确定；若为夹具问题，改用与 store 层一致的确定性 future-schema 夹具；若为产品缺陷，在本任务范围内修复并保留该 CLI 用例。

### 🟡 建议改进（本轮同样必须关闭）

**LRF-R1-F5** [cmd/agentdeck/main.go:402; internal/doctor/doctor.go:33] 已分类但无安全动作的 `lock_owner_unknown` 在 JSON 中**省略** `action_kind`，而契约要求 JSON `null`。
- 行为风险：architecture 明确 `lock_owner_unknown -> null`，并把"plain `ErrStateBusy` 省略 `reason`/`action_kind`"作为未分类调用方的区分形状；`omitempty` 把"已分类、无安全动作"与"未分类"两种形状合并，自动化无法按冻结词表区分。
- 证据：`commandLockErrorDetails.ActionKind *string \`json:"action_kind,omitempty"\``；`doctor.Check.ActionKind string \`json:"action_kind,omitempty"\``；`ux/cli-health-recovery.md` 词表 "`action_kind` … or JSON `null` when no safe action applies"；architecture Doctor Read-Only 表 `lock_owner_unknown` 行 `null`。
- 💡 修复：`error.details` 在存在 `reason` 时总是输出 `action_kind`（可为 `null`）；doctor lock 行对非健康结果输出 `action_kind: null`（健康/缺失锁仍省略），并加 JSON 形状断言。

**LRF-R1-F6** [internal/store/store.go:609; cmd/agentdeck/main_test.go] 任务验证清单中的两项断言缺失，另有一处关键渲染无覆盖。
- 行为风险：`wrapLockContention` 在分类器报错/锁已消失时回退 `lock_owner_unknown` 的分支无测试；doctor 文本的锁专属行（`next:`、`manual prerequisite:`）无任何测试，LRF-R1-F1 因此未被发现；doctor 测试未覆盖 `state_lock` 与 `scan_lock` 同时被占时的独立报告。
- 证据：`tasks.md` Task 1 Verification 列出 "falls back to `lock_owner_unknown` on stat race" 与 "`state_lock` and `scan_lock` reported independently … under contention"；`rg "manual prerequisite|lock_legacy" cmd/agentdeck/*_test.go` 无 doctor 文本用例；`TestCheckReportsLockClassificationAndLifecycle` 每个子用例只写一把锁。
- 💡 修复：为超时回退补一个可注入的竞态用例；为 `renderDoctorText` 补 live/legacy/owner_unknown × state/scan 的文本用例；doctor 测试补两把锁同时存在且分类不同的用例。

**LRF-R1-F7** [cmd/agentdeck/main_test.go:3475] 新增代码未通过 `gofmt`（多余空行）。
- 证据：`gofmt -l cmd internal` 列出 `cmd/agentdeck/main_test.go`（`usage_stats_viewer_test.go` 同时列出但不在本任务 diff 中，属既有状态，不计入本任务）。
- 💡 修复：`gofmt -w cmd/agentdeck/main_test.go`。

### 🟢 优点

- `ClassifyLock` 严格按 architecture 四步实现：先内容形状（`lockOwnerPID`）判定 legacy、再做存活探测，完全不读 mtime；矩阵测试覆盖六种非 v1 令牌并刻意设置旧 mtime。
- `ErrLockContention` 通过 `Unwrap() -> ErrStateBusy` 保持 `errors.Is`、`state_busy` 代码与 exit 1；仅 `AcquireLock`/`AcquireScanLock` 包装，配额锁保持 plain `ErrStateBusy` 且有 store 层测试锁定。
- doctor 锁检查只读（分类器仅 `Stat`/`Open`/`ReadAll`），`lock_reclaimable` 以 `ok` 报告且不计入问题数，退役代码一对一替换。
- `state`/`scan`/`unknown` 三种命令错误文本与 CLI UX 规格逐字一致；`-race` 通过。

### 📝 总结

- Reviewed state：`feature/health-recovery` HEAD `7f5392463ba18246cd9758dbd94264b79692a9ed` + 未提交候选（`cmd/agentdeck/{main.go,main_test.go,quota.go}`、`internal/doctor/{doctor.go,doctor_test.go}`、`internal/output/output.go`、`internal/store/{store.go,store_test.go}`、`docs/topics/health-recovery/tasks.md`），scoped fingerprint `sha256(git diff HEAD --binary -- <上述文件>) = b313c2a999c8dabe03593170f508fa4e984db196f817b077c01b02227253b058`；无未跟踪文件。
- Reviewer：claude-code（独立冷上下文评审角色；实现者为 codex）。Method：逐项对照 `tasks.md` Task 1、`architecture.md`、`ux/cli-health-recovery.md` 审读 diff，并执行 L3 验证。Scope：Task 1 全部文件与行为；extension、desktop wire 代码与 Swift 不在范围（仅因 LRF-R1-F2 涉及其 fixture）。
- Evidence：`gofmt -l cmd internal` → `cmd/agentdeck/main_test.go`（本任务）；`go vet` 受影响包 → 通过；`git diff --check` → 通过；`scripts/run-go-test.sh ./internal/store/... ./internal/doctor/... ./cmd/agentdeck/...` → FAIL（`cmd/agentdeck` 3 项）；`scripts/run-go-test.sh -race ./internal/store/... ./internal/doctor/...` → PASS；`scripts/run-go-test.sh ./...` → FAIL（`cmd/agentdeck`、`internal/desktop`）；对照 `main` 单跑 `TestSnapshotPerformanceContractSyntheticCorpus` → PASS。
- 结论依据：一处不安全删锁指引（F1）与三类测试失败（F2–F4）直接违反任务验收；F5–F7 为契约形状、覆盖与格式缺陷。全部 7 项均属本任务变更，须在同一修复轮关闭。
- 残余不确定：F4 的根因（夹具 vs 产品）尚未诊断。
- Completion gate：NOT_VERIFIED — CEv1 中尚无 `health-recovery:lock-recovery-foundation` 任务 WorkUnit（`missing_work_unit`）；本轮 FAIL 不写入通过证据，复评 PASS 前需建立该 WorkUnit 及其必需准则。

## Round 2 — 2026-09-23

## 📋 health-recovery / lock-recovery-foundation 评审

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**LRF-R2-F1** [internal/doctor/doctor_test.go:83] `owner_unknown` 的预期 `Check` 仍把 `ActionKind` 留空，但实现已设置为 `"null"` 以输出契约要求的 JSON `null`。
- 行为风险：全仓 Go 套件失败，Task 1 的 L3 验证未满足；此断言也没有锁定已分类且无安全动作的内部表示。
- 证据：`scripts/run-go-test.sh ./...` 的 `TestCheckReportsLockClassificationAndLifecycle/owner_unknown` 失败：实际 `ActionKind:"null"`，预期 `ActionKind:""`；同一文件 `TestCheckActionKindJSONSerialization` 已断言序列化为 JSON `null`。
- 💡 修复：将该表驱动用例的 `wantCheck.ActionKind` 对齐已批准的 `lock_owner_unknown -> JSON null` 契约，重跑该用例及受影响和全仓套件。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- Round 1 的安全指引、CLI 参数和 schema 优先级用例已在当前候选中修正；三个曾失败的聚焦 CLI/快照用例通过。
- `scan.lock` 的 legacy 文本现在明确指向 `scan.lock`，且有对应文本断言；`gofmt -l` 与 `git diff --check` 无输出。

### 📝 总结

- Reviewed state：`feature/health-recovery` HEAD `7f5392463ba18246cd9758dbd94264b79692a9ed` + Task 1 未提交候选；`sha256(git diff HEAD --binary -- <本任务 14 个已修改文件>) = 95bc45e5ff4a683cddadde9a50180685da91363dcd9e5d114c76d1a3c1dfaa31`。本记录自身为未跟踪评审工件，不计入产品候选指纹。
- Reviewer：codex。Method：基于 Round 1 七项发现核对当前实现与测试，并运行聚焦回归及 L3 全仓 Go 套件。Scope：`lock-recovery-foundation` Task 1；产品代码、测试、配置均只读。
- Round 1 disposition：F1 的目标锁文件名、F2 的生产者快照、F3 的 quota CLI 参数、F4 的 schema-ahead 夹具、F5 的 JSON null 形状、F6 的竞态/文本/双锁覆盖、F7 的格式问题均已在当前候选中看到对应修正；F2/F6 的全仓验证仍受本轮 F1 阻断，不宣告这些项最终通过。
- Evidence：`scripts/run-go-test.sh ./cmd/agentdeck/... -run 'TestQuotaRefreshLockYieldsUnknownResource|TestSchemaAheadPrecedenceOverStateBusy|TestSnapshotPerformanceContractSyntheticCorpus'` PASS；`gofmt -l`（本任务八个 Go 文件）无输出；`git diff --check` PASS；`scripts/run-go-test.sh ./...` FAIL，仅 `internal/doctor` 的上述子用例失败，日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.6dhDxQ`。出现决定性失败后未继续 race/vet/Swift 验证。
- Completion gate：NOT_VERIFIED；上一轮已确认 CEv1 缺少 `health-recovery:lock-recovery-foundation` WorkUnit，本轮 FAIL 未跨越 Task 完成边界。修复轮须先关闭 LRF-R2-F1，后续 PASS 前还需建立 WorkUnit 及必需准则并查询精确内容状态的门禁。

## Round 3 — 2026-09-23

## 📋 health-recovery / lock-recovery-foundation 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- LRF-R2-F1 已关闭：`owner_unknown` 表驱动用例明确期望 `ActionKind: "null"`，与 `TestCheckActionKindJSONSerialization` 和已批准的 JSON `null` 契约一致；原失败用例与全仓 Go 套件通过。
- Round 1 的 LRF-R1-F1..F7 已在上一轮候选中逐项修正，本轮复核其现有测试、fixture 和文本/JSON 契约；全仓 Go 与锁相关 race 套件通过，未见回归。

### 📝 总结

- Finding disposition：LRF-R1-F1..F7 全部关闭；LRF-R2-F1 关闭；无仍开放、回归或新增阻断发现。
- Reviewed state：`feature/health-recovery` HEAD `7f5392463ba18246cd9758dbd94264b79692a9ed` + Task 1 未提交候选及本轮 `tasks.md` 状态；`sha256(git diff HEAD --binary -- <本任务 14 个已修改文件>) = 5a3b9a035d8ae0a1a864c7efdcba6a051dca4981d6c7b9c10acc9bfdc67825cb`。评审记录是未跟踪工件，不计入产品候选指纹。
- Reviewer：codex。Method：对照 Round 1/2 记录逐项复核候选、运行原失败用例及被变更影响的 L3/L0 检查。Scope：Task 1 的锁分类、doctor/CLI 契约、fixture 和任务状态；不覆盖后续 Tasks 2–5 的实现与原生界面验收。
- Evidence：`scripts/run-go-test.sh ./internal/doctor/... -run '^TestCheckReportsLockClassificationAndLifecycle$'` PASS；`scripts/run-go-test.sh ./...` PASS（日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.J2PJ0X`）；`scripts/run-go-test.sh -race ./internal/store/... ./internal/doctor/...` PASS（日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.Ad4KfO`）；`GOCACHE=/private/tmp/agent-deck-go-build NO_RTK=1 go vet -mod=vendor ./internal/store/... ./internal/doctor/... ./cmd/agentdeck/...` PASS；`bash scripts/check-topic-docs.sh`、`gofmt -l`、`git diff --check` PASS。
- Residual：Swift/native acceptance 属后续 Task 3–5；本 Task 的评审 PASS 不代替 CEv1 门禁或提交授权。
- Completion gate：VERIFIED（4/4 必需准则）。CEv1 WorkUnit `urn:ce:agent-deck:work-unit:health-recovery-lock-recovery-foundation` 对 ContentState `urn:ce:agent-deck:content-state:health-recovery-lock-recovery-foundation:rereview:5a3b9a035d8ae0a1a864c7efdcba6a051dca4981d6c7b9c10acc9bfdc67825cb` 的门禁查询无缺失准则、失效证据或未决影响；此结果只适用于该候选指纹。
