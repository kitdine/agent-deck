---
status: historical
retired: 2026-10-07
topic: v0-6-5-contract
subject: v0-6-5-contract
---

# Final v0.6.5 contract review

## Round 1 — 2026-10-06

## 📋 Task3 独立冷上下文契约评审

📊 总体评分：6/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

**V065-COMPAT-R1-F1 — P1 — OPEN：候选包含 sessions 数据库迁移，超出已批准的 v0.6.5 例外。**

位置：`docs/topics/v0-6-5-contract/tasks.md:455`；`docs/specs/cli-design.md:457`；`internal/store/store.go:56,166`。

- **风险：** Task3 要求最终候选、实际契约与版本兼容性约定一致。当前候选不能满足这一条件，不能因其修复了原始缺陷、已合并、历史门禁通过或数据保留测试通过而完成 Task3。
- **独立核验：** 直接比较发布基线 `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52` 与 PR24 merge `6e8c1c4d2942782152f5e6b5de89b0942fc57ce4` 的 `internal/store/store.go`，确认新增 `session_sources.parser_context`，同时修改 CREATE TABLE 和执行 ALTER TABLE 的迁移路径。当前 main `71e8047e0dd8d638be8587ff1a016c38a9cea635` 保留两者。
- **权威冲突：** `cli-design.md:457–481` 明确要求 `sessions.sqlite3` schema migration 使用 MINOR；唯一 v0.6.5 例外仅覆盖只读 Doctor/health 输出，明确排除数据库及持久化格式变化。
- **原始证据：** `session-schema-source.diff` SHA256 `430bfabf26b35a446a56cea74bd0c767aac26fc79a4e723b4be8043c4f12446c`；`session-schema-observation.log` SHA256 `9ebbdba3a1a031813b6f262a4b8621be53e2a30dc9b00ebbc745af828db31aa0`。已核对现存测试日志的 `TestSessionParserContextUpgradePreservesIndexAndCore PASS`，未重跑。
- **分类：** 这是版本政策／契约不兼容，不是本评审证明的运行时损坏、数据丢失或降级失败。该差异早于当前文档修改，但直接属于 Task3 的最终兼容性审查范围。
- **有界解决：** 保持 Task3/Topic 未完成，将确切冲突交由用户决定兼容性、版本、成员或实现的处理方向，再在独立授权范围内落实并复评。不得由 reviewer 新选版本、新增豁免或修改产品实现。

**V065-CONTRACT-R1-F2 — P2 — OPEN：状态摘要复制了 finding 标识和详细结论。**

位置：`docs/status.md:19`、`docs/status.md:288`。

- **风险：** 状态文档形成与完整评审记录并行的 finding 载体；后续处置容易漂移，而且违反现行状态摘要规则。
- **证据：** 第 19 行包含 `V065-COMPAT-R1-F1` 及 PR24 schema／Doctor 例外冲突描述；第 288 行再次包含 finding ID。`docs/documentation-workflow.md` 的 Status 规则明确要求状态摘要不包含 finding IDs、描述或处置，完整报告保留于评审记录。
- **有界解决：** 两处仅保留 Task2 已通过、Task3/Topic 未完成、候选未提交及完整评审链接等状态信息。finding 身份、证据和处理边界保留在对应完整评审记录。此修正不解决 F1，也不能产生 Task3 PASS。

### 🟡 建议改进 — 推荐

无额外建议性 finding。

### 🟢 优点

- 候选明确揭示 schema 冲突，没有利用 core schema30、历史 PR24 门禁或保留数据测试掩盖版本问题。
- 手册新增 Doctor 描述与 `cli-design.md:2473–2557` 一致：五态及优先级、共享预算、只读指导、路径隔离和无自动 OS 恢复均保持明确。README 更新到设计文档 version31 有依据。
- 十二份未恢复原始证据逐项保留身份、恢复结果和适用性限制，没有把新回归日志伪装成旧收据。
- 性能 FAIL/#43、真实安装及 WidgetTimeline／因果／恢复限制、footer 行政结论与产品修复的区别仍保留。
- 两个 Widget、三个 Hook carrier 当前仍在原路径且 `status: active`，未提前退休。

### 📝 总结

**评审身份。** 审查工作区为 `.worktrees/v0-6-5-closeout`，HEAD 为 `71e8047e0dd8d638be8587ff1a016c38a9cea635`。审查前后核对七个文档和 freeze 所列 supporting artifact 的 SHA256，均无不匹配；重算 manifest digest 得到：

`48dfb9d30a88025641884dad82430786d7e3017553da41a9d0d7b63d20f91086`

这是只读冷契约评审；未读取其他 agent 的评审报告，未修改文件、执行产品测试、运行 Hooks 或操作 Git、Beads、CE。发现决定性兼容性阻塞后未扩大产品验证。已有 `task3-review-docs.log` 表明文档结构检查通过，whitespace 日志无诊断；这些检查不证明语义一致。

**Task2 六项结果的适用性。** F1 不否定十九项成员处置、实际 source/merge 连续性、已存在的行为回归和历史交付事实，也不自行撤销 Task2 在 `57b2d2cc…` 上的六项 accounting 结果。该结果的适用范围必须保持为已声明的 assembly/accounting，不得解释成版本兼容性、Task3 或 Release 已通过。当前新增契约发现应有明确 impact assessment；不能直接把旧结果重贴到本轮状态。若后续用户决定改变成员或实现，需重评被改变的 Task2 criterion。

**当前 tasks.md 文档判断。**

- Authority：符合；未擅自改写唯一 Doctor 例外，并明确外部决策边界。
- Coverage：符合；兼容性问题、风险、三项任务与独立 release/native 边界均有载明，未发现需要新增 feature requirements／UX／architecture 文档的遗漏。
- Readiness：**不具备最终契约完成／批准就绪条件**。候选可以作为真实的未完成计划，但 F1 尚无获授权解决方案，不能把既有文档门禁 3/3 当作当前最终状态的批准。当前 Documents Review 保持未勾选是合适的；最终文档语义结论应保持 FAIL／未完成，直到所依赖决策解决。

**缺失原件的影响。** 十二项未恢复原件限制历史原始过程重建，尤其不能据此重新声明本地安装／timeline PASS。当前 ledger 使用独立的不可变交付身份、保留评审及新观察支持较窄的当前 accounting，未以新豁免替代缺失证据。本次未发现这些缺失必须推翻已保存历史观察的依据；它们也不提供覆盖 F1 的依据。

**门禁与退休。** 所读初始 Task3 exact-target envelope 为五项均缺证据的 `NOT_VERIFIED`；本报告不声称已写入 FAIL 或产生新 canonical gate。主 agent 应把本次发现记录并绑定到最终同步状态，再实际查询受影响门禁。Task3 未实际 VERIFIED 前保持五个 carrier 不退休；Topic 还需要当前文档和 Tasks1/2/3 的有效目标绑定及真实父级门禁，不能从 Task2 或行政关闭推导完成。即使 Topic 日后通过，也不授予发布、安装或 Git delivery 权限。

### 下一步指令

修复范围仅为 **V065-COMPAT-R1-F1、V065-CONTRACT-R1-F2**：先保留并提交外部兼容性决策问题，纠正状态摘要载体；F1 的实质解决等待明确用户方向。本报告不生成不存在的 workflow token、阶段收据或交付授权。

Reviewer: independent cold role `/root/task3_cold_contract`, fork:none, requested gpt-6-astra/low; main owns this retained transcription and independently verified source/SQL/test/policy claims.
Method: current contract/version/safety review; retained transcript SHA-256 `d57b2e9b1514314a084153c93ee98088bee049675b2e1e0c6236c7d59d3072ce`, normalized Markdown whitespace only.
Scope: exact seven-document/support candidate48dfb9d30a88025641884dad82430786d7e3017553da41a9d0d7b63d20f91086, HEAD71e8047e0dd8d638be8587ff1a016c38a9cea635.
Evidence: `/tmp/agentdeck-v065-closeout-20261006/task3-cold-review-r1.md`; operator-local provenance, not a portable download link.
Completion gate: NOT_VERIFIED

The canonical field records the actual initial source gate. Current failed-criterion evidence and post-transcription states require separate binding/query. No product repair or policy exception is granted.

## Round 2 — 2026-10-06

## 📋 Task3 有界复评：状态摘要修复

📊 总体评分：6/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

**V065-COMPAT-R1-F1 — P1 — OPEN，沿用 Round 1。**

位置：`docs/topics/v0-6-5-contract/tasks.md:455`。

sessions schema migration 与现行 v0.6.5 兼容性约定的冲突仍未解决。本轮没有版本、豁免、成员或产品变更，不能关闭此 finding。风险、原始证据与有界解决要求沿用 Round 1；未重复产品验证。

**V065-CONTRACT-R1-F2 — CLOSED：已核验修复。**

位置：`docs/status.md:19,288`。

- 与 `snapshots/48df/docs/status.md` 的实际差异仅为两处状态摘要调整：移除 finding ID 和具体冲突描述，保留未完成状态并链接完整契约评审。
- 原快照 SHA256 与 Round 1 freeze 相符；当前状态文档无 F1/F2 标识残留。
- 完整 finding 与历史 FAIL 保留在 `reviews/v0-6-5-contract.md`，符合 `docs/documentation-workflow.md:398` 的状态摘要规则。
- 已读取 `task3-r1-repair-l0.log`：文档结构检查通过；该结果不代表语义 PASS 或 CE 门禁通过。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

修复范围准确，没有借状态清理弱化兼容性阻塞或改写历史评审结果。

### 📝 总结

本轮仅复评 F2。相较 Round 1，manifest 中仅 `docs/status.md` 改变并新增完整 Task3 评审记录，其余原有 subject 身份未变。复评前后全部 subject/support 哈希匹配，重算 digest 为：

`e9b1b6627aa93f43f5ce49e238fb2832fce55f47928462f7f74632c2bc42dafa`

HEAD 采用该 freeze 中的 `71e8047e0dd8d638be8587ff1a016c38a9cea635`；遵守本轮禁用 Git 操作边界，未另行查询 Git。当前 `docs/status.md` blob 为 `c03b2abd616a96c72eab06f7b73d52f8db4f3910`。

**文档就绪性：** tasks.md 未改变，authority/coverage 判断沿用 Round 1；最终契约 readiness 仍未满足，Documents Review、Task3 和 Topic 不应据本次局部修复标为完成。

**门禁限制：** 记录中 `NOT_VERIFIED` 明确属于 Round 1 初始目标。本轮未查询或写入 CE，不能把该结果重贴到 `e9b1b662…`，也不声明当前 canonical gate 已 FAILED。主 agent 需在最终同步后绑定实际状态并查询。五个 carrier 的退休前置条件仍未满足。

### 下一步指令

仅剩 **V065-COMPAT-R1-F1**：等待用户对确切兼容性冲突给出处理方向，在对应授权范围内落实后复评。F2 已关闭，无需重复修复或验证；本报告不授予 Git delivery、产品修改或新豁免权限。

Reviewer: same independent cold-origin `/root/task3_cold_contract`; main verified the two-line status delta and policy source.
Method: F2-only read-only re-review; retained transcript SHA-256 `ceb4c47b068ee3efc37b50d83f850d4d3842afad83a2c0c187fec180af0ddd0c`; unchanged F1 remains open.
Scope: candidate e9b1b6627aa93f43f5ce49e238fb2832fce55f47928462f7f74632c2bc42dafa, same HEAD71e8047.
Evidence: `/tmp/agentdeck-v065-closeout-20261006/task3-cold-review-r2.md` and actual status/L0 repair.
Completion gate: FAILED

Current failed-criterion evidence remains to be recorded/query-bound by main. This local F2 closure does not approve Task3 or authorize an exception.

Main gate finalization: actual synchronized candidatefc4c2f41ee5e4e7341a54879d603e3e5f07c77d37864ca738f9677918cddf35f returned Task3 FAILED5-required and Topic FAILED2-required; current document readiness also returned FAILED. Task2 accounting separately returned VERIFIED6/6 after explicit new-finding scope assessment. Missing lifecycle/coherence scope remains open, F1 unresolved, F2 CLOSED. Original initial NOT_VERIFIED envelopes and all historical FAILs remain unchanged. Further label synchronization has its own exact target; no new waiver, retirement or delivery is inferred.

## Round 3 — 2026-10-07

## 📋 B 决策与兼容性冷上下文复评

📊 总体评分：8/10

Verdict: FAIL

### 🔴 严重问题 — 必须修复

**V065-B-R3-F1 — P2，OPEN：当前 document 被错误描述为 awaiting_commit。**

位置：冻结 tasks.md:545。生命周期段称 “Task2/document coordination remains awaiting_commit”，而该次 document 已回到修订/评审，没有提交 checkpoint。该句无历史时间绑定。仅修正此句，区分 Task2 accounting checkpoint 与仍开放的 document；不得扩大为产品/合同修复。

### 🟡 建议改进 — 推荐

无建议性 finding。当前 B 的 local-link/discovery 检查尚无完整日志，是 verification evidence gap；不声明存在 broken link。补齐仓库已有有界 L0 后才可完成 required gate。

### 🟢 优点

**V065-COMPAT-R1-F1 CLOSED：**真实用户 B 决策仅 except PR24 列及增量迁移；主合同、手册、任务验收一致，实际合成升级/旧读/往返成立。历史 FAIL 不变。

**V065-CONTRACT-R1-F2 CLOSED，未回归：**状态摘要无旧 finding IDs，详细记录仍在本文件。

十个具名测试均真正执行 PASS；日志/快照 SHA256 匹配。两组完整 document_values 在往返中相同（append 前2条、后3条），两排除保留；integrity/FK、旧只读 index bytes、core/synthetic account/config/auth/settings bytes及0600断言真实存在。独立源码/vendor2517/2529 blob核对零差异。早期 no-tests/FTS setup failures 保留并排除。

### 📝 总结

tasks.md authority PASS、coverage PASS、readiness FAIL。Task2 accounting scope 可复用，未重新核账十九项或搜索十二缺失原件。既有性能FAIL/#43/P3行政/native/installation限制保留。旧 criterion 的 compatibility-exception clause 仅按真实用户 B 方向解释；不覆盖非例外约束，不改历史 observation。

Scope: Task3/document exact ready candidate; synthetic Go store/session paths only. No real-account restore, installed binary, all session formats, native Widget, OS recovery or Release acceptance. Five carriers remain active and unchanged.

Reviewer: independent cold `/root/b_contract_cold_review`, fork:none; requested gpt-6.1-sol/xhigh. Upstream actual model/account/Fast remain unverified. Main verified claims and owns this transcription.
Method: bounded current contract/finding review, source/vendor Git blob and log/snapshot/hash checks; no product test rerun, Hook, memory/work_state or delivery action.
Reviewed state: HEAD71e8047e0dd8d638be8587ff1a016c38a9cea635; candidate62acfd2f9c21ee4243cfb4e36ff1f3df793c4fe87ef3d50be3a654ce44fa0258; tasks.md blob f4555ca7da4ee921bc1139dc86872a0774b369d9; document digest c6bc32b4067269085159b6fc10a5fc6813120659cf9685762d527c859cc77dde.
Evidence: operator-local `/tmp/agentdeck-v065-b-20261007/cold-review-r3.md`, SHA-256 `6e4fdf8dfa4b8732c30bf20ba409c1d639cb8aa1dc5672e3f9c159ed4ae8f1db`; main independently matched source identities, ten named-stage logs/snapshots and current statement. Reviewer verified all9 subjects/17 supports before and after.

Completion gate: FAILED

Source initial gates were Task3 NOT_VERIFIED5 and document NOT_VERIFIED3. Main subsequently recorded this R3 lifecycle/review/readiness failure at the exact ready targets and queried both FAILED (`r3-gates.json`). This is neither old F1 nor a result for the later repair.

### 下一步指令

仅修复 V065-B-R3-F1，补齐当前 scoped local-link/discovery L0，再进行有界复评；主执行者拥有已授权文档修订，五个载体继续等待。无 Git/发布授权。

## Round 4 — 2026-10-07

## 📋 退休前最终契约有界复评

📊 总体评分：9/10

Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**V065-B-R3-F1 CLOSED：**生命周期段区分已评审的 Task2 accounting checkpoint 与仍开放的 document/Task3，不再把 document 误记为 awaiting_commit，也不依赖瞬时 Beads 状态。

**V065-COMPAT-R1-F1 保持 CLOSED，无回归；V065-CONTRACT-R1-F2 保持 CLOSED，无回归。**B 例外与兼容性输入不变，原17项支持身份不变，未重跑十阶段验证、源码/vendor、十九成员或证据恢复。

Scoped L0 gap CLOSED：无参数 check-docs.py 实际覆盖 tracked/untracked Markdown/local links/anchors/相关 incoming links/topic 文档集/review-record结构，并执行 whitespace/diff。结构 PASS 不替代本次语义复评或 CE。旧字节前缀均保持，历史FAIL未改写。

### 📝 总结

tasks.md authority/coverage/readiness 均 PASS。Task2 accounting 保持可复用，实际初始 gate VERIFIED6/6。五个 carrier 仍 active，哈希与先前独立核验相同；本轮通过的是退休前契约及条件生命周期，不认证退休完成或 Topic/Release/交付完成。

Scope: exact repaired Task3/document candidate; synthetic Go compatibility only. Real-account restore/installed binary/native Widget/OS/release boundaries and performance FAIL/#43/P3/native残余保留。

Reviewer: same independent cold-origin `/root/b_contract_cold_review`, requested gpt-6.1-sol/xhigh; no upstream actual-model/account/Fast attestation. Main owns the authoritative transcription.
Method: selective lifecycle/record repair re-review, current checker-path/log/manifest/history-prefix/carrier-hash inspection; unchanged behavioral evidence reused.
Reviewed state: HEAD71e8047e0dd8d638be8587ff1a016c38a9cea635; candidate68bfb0eaa5e38e792591d8d759221bedbbc82e24655f28e6385936d8bc1cd8f4; tasks.md blob5ebd8e70fb1c8c9f82addb1727721a238c2c3b2b; document digestc4b7f6d2740b72b07ad9d07653821b4870d8b4bac600cb1365373d90ef95fb55.
Evidence: operator-local `/tmp/agentdeck-v065-b-20261007/cold-review-r4.md`, SHA256 3641aae917ba4086ccac4d0f2de0b9d65d1e72b8ae0662dbd28f1ca4e80789d4. Main verified9 subjects/20 supports with zero drift; scoped check-docs exit0/log SHA256 bb5d4deddd07d22c32c5c91415517fb8747c65558e0bcbd9146c70b613c74359.

Completion gate: VERIFIED

This canonical field initially records actual repaired Task3 NOT_VERIFIED5; document NOT_VERIFIED3. Main must bind the later synchronized target and query it, preserving this initial envelope. No retrospective relabelling of source observations.

### Task checkpoint

Task3 review PASS; required gate remains open pending main target binding. 提交建议：等待 required gate及单独提交授权。推送建议：等待这些条件和单独推送授权。退休须先满足 required gates，再有界核验 metadata/link/body-preservation；出现语义不确定性才新增独立评审。

Main exact-state finalization: pre-retirement candidate d32c4b9786efdd7d29a111b2b461deabc9defbb4304af33ca76ec8554e074c2f returned Task2 VERIFIED6/6, document VERIFIED3/3 and Task3 VERIFIED5/5, with no missing or unresolved criteria. Document target623ab43a2064b29c4cfad464b4cafdb695c317e4f112257a5371ce01f9565cdd. Initial repaired-target NOT_VERIFIED envelopes remain unchanged. The canonical field records this actual later query, not a claim about a future target.

Only after these gates passed, five delivered fix carriers were moved to docs/archive/fixes with historical/retired frontmatter. Every body/review-history byte was preserved; five current task links were updated. Main retirement/body/link assessment and final scoped L0/target roll-up remain explicit independent obligations; no new product/native acceptance or delivery is inferred. Raw prerequisite and retirement identities are retained in operator-local pre-retirement-gates.json and retirement-results.json.
