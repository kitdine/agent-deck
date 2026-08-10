---
status: historical
plan: v0-4-0-contract
task: v0-4-0-contract
retired: 2026-08-10
---

# Review log — v0-4-0-contract

The original round headings retain their historical `v0-4-0-release` scope
label. The corrected workflow ends at this contract task; those labels do not
create RC or stable-release successor tasks.

## Round 1 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD `126d739`.
- Development content state:
  `urn:ce:agent-deck:state:candidate:2dc483cfe64174573503a5d54fb48c7355d4b3a5fa9097dc0b54c0bfa43ebdfc`.
- Scoped diff SHA-256:
  `2dc483cfe64174573503a5d54fb48c7355d4b3a5fa9097dc0b54c0bfa43ebdfc`.
- Scoped file SHA-256:
  - `docs/README.md`: `8f84a371f71e63de7d6a79ad3cbfc8a06c5f088c067d6a8b4ad6efa5787bd16e`
  - `docs/plans/desktop-app.md`: `7e6efca09ad018bd5ffa85d227b62d8fdce02932551e831c8f47c312bc29ab7a`
  - `docs/plans/v0-4-0-release.md`: `7f98b6c6a3a30499e79ec16a87d8ac80e3562235ac014d19959cffcbca371511`
  - `docs/specs/cli-design.md`: `75172e3b5e573ae8b87600eeaa8daa61c63e06e1e3ebd068cf9ab48bdf5dbfde`
  - `docs/specs/cli-manual.md`: `b1089678adb26a9097d8741c20697ac08adb78dbeabaa58f2a348336a9831d24`
- Reviewer: Codex
- Scope: the single v0.4.0 specification raise, release-contract reconciliation,
  both feature plans' review state, documentation-index consistency, desktop DTO
  dependency state, and reusable L2 development evidence.

## 📋 评审报告 — v0-4-0-release / v0-4-0-contract

📊 总体评分：8/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`docs/archive/reviews/session-experience/interactive-session-viewer.md:2`]
已归档的 session-experience review 记录仍有五个文件声明
`status: active`，与归档生命周期合同冲突。

- 行为风险：`v0-4-0-contract` 必须确认两个功能计划的状态矩阵与 review
  records 一致；当前 session-experience 已归档且 6/6 Review PASS，但
  `interactive-session-viewer.md`、`session-document-time.md`、
  `session-scan-progress.md`、`session-show-layout.md` 和
  `session-usage-detail.md` 仍标记 active。恢复工作或自动检查 review 生命周期时会把
  已退休任务误判为仍活动，因而不能确认发布合同要求的状态一致性。
- 证据：上述五个文件第 2 行均为 `status: active`；同目录的
  `session-experience-contract.md` 已为 `status: historical`，归档计划矩阵六行均为
  Dev/Review `[x]`，十二个功能任务 review 记录的末次 verdict 均为 PASS。
  `docs/reviews/README.md:42` 明确要求 review 目录归档时将每个文件的 frontmatter
  设为 `status: historical`。

💡 有界修复：只把上述五个归档 review 文件的 frontmatter 从
`status: active` 改为 `status: historical`，确认历史 round 和最终 PASS 内容未变；
不要修改产品代码、living contract、specification version、功能计划矩阵或 RC 状态。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `cli-design.md` 只从 version 23 提升到 24，并只新增一条覆盖完整 v0.4.0 的
  changelog；没有重复提升 specification version。
- session-experience 与 usage-report-presentation 的归档矩阵均为 6/6
  Dev/Review 完成，每个任务 review 记录的末次 verdict 均为 PASS。
- living design/manual 已包含 session DTO、parser/index rebuild、event-time pricing、
  responsive usage text 与 interactive viewer 合同；本次候选没有改动产品或 JSON。
- desktop plan 只把 `desktop-wire-contract` 的前置依赖改为已满足，Task 1 仍为
  `[ ]/[ ]`，coherent snapshot、wire version 与 Go-owned redaction 没有被纳入 v0.4.0。
- README、release plan 与 specification version 24 的候选状态一致；CEv1 已记录绑定
  当前开发候选的 L2 pass evidence。

### 📝 总结

评审对象为 HEAD `126d739` 加五文件候选 diff
`2dc483cfe64174573503a5d54fb48c7355d4b3a5fa9097dc0b54c0bfa43ebdfc`；CEv1
开发证据绑定同一 scoped content state。
发布合同主体、版本提升、DTO 依赖和两条功能线的最终 PASS 均正确，但五个归档 review
文件仍声明 active，直接违反本 task 的 review-record 一致性要求，因此本轮为 FAIL，
`v0-4-0-contract` Review 保持未勾选，RC task 不启动。发现该决定性文档阻断后复用已绑定
候选状态的 L2 开发证据，没有重复全仓测试；修复后只需复评归档元数据、候选差异与
review 状态一致性。

- Verdict: REOPEN

### 下一步指令

修复：`v0-4-0-release` / `v0-4-0-contract` 的 session-experience 归档 review frontmatter

## Round 2 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD
  `126d73977eda16314cab7c8be0eff01232a26888`.
- Scoped tracked diff SHA-256:
  `1dc77f42bfa70235cd128954b9fb4a727241e67811aac690088cd24fbb027a29`.
- Reviewer: Codex
- Scope: Round 1 archive-lifecycle finding closure, preservation of all final
  feature-task PASS verdicts, release-contract status synchronization, and
  reusable exact-state L2 evidence.

## 📋 复评报告 — v0-4-0-release / v0-4-0-contract

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。Round 1 的唯一 finding 已关闭，没有回归或新增阻断项。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 五个已归档 session-experience review 文件的 frontmatter 均从
  `status: active` 精确改为 `status: historical`。
- 修复未改变任何历史 review round；六个 session task 和六个 usage task 的最终
  verdict 仍全部为 PASS。
- v0.4.0 living contract、specification version 24、desktop DTO 依赖边界及产品代码均
  未改变，原 L2 开发证据仍可复用。
- release plan 与 README 只同步 Task 1 Review PASS 和 1/3 状态；Task 2 仍为
  `[ ]/[ ]`，没有启动 RC 或扩大交付权限。

### 📝 总结

Finding disposition：Round 1 的归档 review lifecycle 阻断已关闭；无 still-open、
regressed 或 newly blocking finding。评审对象为 HEAD `126d739` 加本轮最终 scoped
tracked diff；修复严格限于五处 frontmatter，历史 PASS 内容不变，tracked diff 的
格式检查通过。发布合同主体复用同一产品/合同 L2 证据，复评结论为 PASS；剩余风险仅为
后续 RC task 尚未执行其独立 L4 与真实隔离状态验收，不能由本 task 的 PASS 替代。

- Verdict: PASS

### Task checkpoint

Task checkpoint：`v0-4-0-release / v0-4-0-contract`，HEAD `126d739` 加 scoped
tracked diff `1dc77f42bfa70235cd128954b9fb4a727241e67811aac690088cd24fbb027a29`，
completion gate `VERIFIED`。

提交建议：按 Task 1 原子边界提交五个 release-contract 文档、五个
session-experience 归档 review frontmatter 修复和本 review record；当前 checkpoint
不授权 commit。

推送建议：提交完成且另获 push 授权后推送 `main` 至 `origin/main`；当前 checkpoint
不授权 push。

### 下一步指令

开发：`v0-4-0-release` / `v0-4-0-release-candidate`
