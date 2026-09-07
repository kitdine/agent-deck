---
status: active
topic: schema-version-signal
subject: tasks.md
---

# Tasks Review

## Round 1 — 2026-09-07

## 📋 任务分解评审报告

📊 综合评分: 9/10

✅ 评审结论: PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 六个顺序任务覆盖 requirements 的九项验收，文件、依赖、结果及 L0–L3 验证边界明确；尚未实施的任务均未勾选。
- Task 1 把 sentinel、CLI/doctor 映射、两向版本数、partial envelope 和 Go 镜像放在同一边界，避免把中间失败留给后续任务。现有 canonical partial fixture 由空状态根生成，仅有 database_unreadable；新增可选版本字段不要求提前改写它。
- Task 2 为记录读写/清除设置独立 owner，覆盖锁释放失败、非 Hook 成功清除、只读不清除及升级抑制；未把近似计数写成精确账本。
- Task 3 要求真实 Go producer fixture、legacy 不变及共享/独立 Swift decoder 验证；Task 4 保持 App badge 与 Shared 刷新状态分离，保留独立会话警告。
- Task 5 明确原生验收未完成就保持边界开放；Task 6 覆盖全部 Contract edits，并纠正清除与停止呈现不是两次删除的措辞。范围外 architecture carrier 保留，未据旧抑制说明重引入 UX 缺陷。

### 📝 小结

- **Reviewer**: Codex，本会话未起草任务分解；前序工作仅涉及 UX 评审、交付与状态同步。未委派。
- **Method**: 单主评审，按需求覆盖、前提真实性、任务间中间态、契约一致性和验证边界检查；CodeGraph 定位结果未命中 fixture 源时，改用定向源读取。不是运行时验收。
- **Scope**: tasks.md 的文档集合、六个任务及验收映射；对照 requirements 验收 1–9、architecture C1–C6/Contract edits、已交付 UX D1–D9。
- **Reviewed state**: HEAD `66924c46a86484a074d7162854d1a11645919595`；入口文档 blob `bf17d0c2202f28806c9e7f4c090587c8d6a72f1e`；仅同步审批状态后的最终 blob `5b40a8dd6eb255ef497cb2e4eb47d6f788221541`。任务内容未因本轮改变。
- **Evidence**: `bash scripts/check-topic-docs.sh` exit 0；`make check-whitespace` 与 `git diff --check` exit 0。源码核对包括 store.open 的 acquire/deferred Release/最终返回（store.go:207–249）、runUsageHookEvent 开库失败返回 nil（main.go:2940–2955）、doctor 两个短路及 schema_outdated（doctor.go:49–83）、healthSnapshot 逐字段镜像（desktop.go:758–771）、producer fixture 的三个既有 builder 与字节比较（fixtures_test.go:28–59、143–146）、test-macos-app.sh 的 Xcode/CLT 分支及四个既有 fixture、独立 verify.swift 的逐路径循环。新增第五 fixture 的分解与现有入口相容。
- **Findings**: 无。没有用文档评审代替产品验收，也没有创建实现 dispatch。
- **Residual uncertainty**: 尚无本主题实现，锁竞争、Hook 文件生命周期、真实客户端和原生无障碍效果必须由各实施任务及 Task 5 证明；本轮不声称这些已经运行通过。
- **完成门禁**: 见下方同步结果。文档有独立 WorkUnit；主题还有六个实现任务，不跨越 Topic 完成边界。

### 门禁同步

完成门禁: VERIFIED。WorkUnit `urn:ce:agent-deck:work-unit:schema-version-signal-tasks`，目标 `urn:ce:agent-deck:state:candidate:4e9b5330567f6351d926fe23c21daaa56e8e82840d5e31dddeffbfb0c4b51d62`。
两条 required criterion 为 `independent-review-pass`、`document-set-consistent`；固定模板查询返回 2/2 有效证据，missing/invalidated/unresolved 均为空。WorkUnit 与两条 criterion、内容态与两条 evidence 分别确认创建 3 个节点；requires 2 条及证据关系 4 条均预检 ok，并确认相应创建数。

### Task checkpoint

Task checkpoint：`ad-svs-doc-tasks-design`，最终文档 blob `5b40a8dd6eb255ef497cb2e4eb47d6f788221541`，候选门禁 VERIFIED，待提交。

提交建议：仅提交本任务的 tasks.md、本评审记录，以及 docs/status.md 的主题状态同步。提交时核对作者归属，并将证据绑定到不可变提交内容态。

推送建议：目标未解析；在授权提交与不可变态证据完成后另行取得推送授权。本轮未提交或推送。主题尚有六个实现任务，不生成 Topic completion checkpoint。

### 下一步指令

开发：schema-version-signal / core-schema-contract
