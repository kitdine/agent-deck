---
status: active
created: 2026-10-02
---

# 缺陷：文档改动后旧 PASS 门禁被用于当前状态

## 现象

Beads `ad-bug-hook-gate-ignores-content-state`，用户明确授权 Lane A 与三个关联 Hook issue 共用一个 PR。
Base `b864ce41b8d6ce22c475fbc42db6c5cad6b19f6b`，branch `fix/hook-consistency-batch`。
隔离回归 `test_document_gate_requires_current_latest_round_blob` 在未修改 Hook 的基线失败。

## 根因

Check 1 interpreted the last Markdown PASS/gate without verifying the document blob. A changed subject plus dirty review produced an incorrect awaiting_commit instruction.

## 修复边界

For document tasks require one full current document blob in the latest non-fenced review round; unknown/ambiguous identities or changed prototype specimens do not authorize a dispatch diagnosis. Task implementation ownership matching is unchanged.
代码限 `scripts/hooks/beads-consistency.py` 和对应 Python tests。
不访问真实产品数据、不写 Beads/CEv1 backend、不改变产品或阶段授权契约。
本 issue 保留独立 WorkUnit `fix:hook-gate-ignores-content-state`、回归和关闭证据。
同 PR 另含已完成图标 topic 的整体归档及集成指针，不提前归档本修复。

## 验证

- Failure-first: `python3 -B -m unittest discover -s scripts/hooks -p beads_consistency_test.py -k HookBatchRegressionTest`，基线三个 issue 的测试分别失败；`output/hook-batch-evidence/red.log`。
- 修复后同一测试模块 75 tests PASS，包含既有 Codex/Claude 事件隔离测试与新增真实临时 Git 仓库路径测试；`output/hook-batch-evidence/green.log`。
- L2 Hook 子系统；没有 Go 产品变化，因此无需与影响无关的 Go build/race 或发布 L4。
- CodeGraph sync 因当前 PATH 缺 node 失败；使用定向源码检查。独立 review、L0、CEv1 与远端交付待完成。

## Review — Round 1

## 📋 Hook 批次冷独立评审

📊 综合评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

- HB1-F1（P2），`retired_lifecycle_terms`：围栏内和引号内生命周期仍误报。独立探针复现；修复为跟踪完整围栏并仅解析明确声明起始句。
- HB1-F2（P2），同函数：后续无关的 “Do not skip review” 会抑制前面的真实声明。独立探针复现；修复为按声明子句识别，不以整行否定词筛除。
- HB1-F3（P3），test module：`unittest.main()` 位于新增类之前，直接脚本只执行68测试。移至文件末尾。

### 🟡 改进建议

无。

### 🟢 优点

三个原始缺陷均有失败先行回归；归档五文件历史、merge父节点和结果tree一致。

### 📝 总结

Reviewer: 冷上下文 `hook_batch_review`；主代理直接复现并确认上述三项。
Method/Scope: 全批代码、测试、三个修复载体、图标归档及指针；只读独立审查。
Reviewed HEAD: `b864ce41b8d6ce22c475fbc42db6c5cad6b19f6b`。
原Hook SHA256 `3584610c8b6d23213fa30fae6e32ef1ca48f298b7bdd2fc884238f6276dc8e8c`；
原tests SHA256 `ca637535883401bfc2a17ae73d26db4d57f79895c2b3f41673b0c6da15d83ad7`。
Evidence: discovery73/direct68；review-red.log确认5个子场景失败。
Completion gate: NOT_VERIFIED，候选仍在修复复评边界。

HB1-F1/HB1-F2/HB1-F3 -> repaired in candidate；新增引用/围栏/否定句回归，
直接脚本75/75通过，等待独立复评。没有将作者修复当成PASS。

## Review — Round 2

## 📋 Hook 引用上下文复评

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

HB1-F1（P2）仍部分开放：句号切分丢失外层引用上下文，
`"Old contract. Lifecycle: open -> drafting -> closed. End quote."`
及行内代码例子仍误报。主代理复现3个失败子场景，见 review2-red.log。

### 🟡 改进建议

无。

### 🟢 优点

HB1-F2 CLOSED：声明后的无关否定词不再抑制声明。
HB1-F3 CLOSED：直接运行和discovery均执行75项。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；主代理直接核验。
Method/Scope: 冻结修复候选，只读复评及对抗探针。
Hook SHA256 `bb3f75c77aee4d9ef354ed2ff299920880f00d4b0154f3604eda3fc81e7c6c47`；
Tests SHA256 `6bdf18a44ee998be8515b971ccfdac737f9817ca0c3933126c6016ed6bb45ab0`。
Evidence: 75/75与git diff --check通过，但上述反例失败。
Completion gate: NOT_VERIFIED。

HB1-F1 -> repaired in candidate：先屏蔽引号/行内代码中的完整引用，再切分子句；
单独反引号状态token保持可识别。新增多句及单/双引号反例；等待独立复评。

## Review — Round 3

## 📋 Hook 批次最终独立复评

📊 综合评分：9.5/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。HB1-F1 CLOSED：完整引用/围栏在分句前排除，真实声明仍识别。
HB1-F2 CLOSED：无关否定句不遮蔽声明。HB1-F3 CLOSED：两种入口均执行全部测试。

### 🟡 改进建议

无。

### 🟢 优点

逐issue失败先行回归保留，75 tests通过；新增真实Git目录与字面路径验证。
未知文档内容态保守不建议状态迁移；无真实数据、权限或产品契约变化。

### 📝 总结

Reviewer: 冷上下文独立 `hook_batch_review`，主代理复核源码摘要与实际测试结果。
Method/Scope: 本批三项Hook修复与测试、fix载体、图标归档及指针；只读独立复评。
Reviewed HEAD `b864ce41b8d6ce22c475fbc42db6c5cad6b19f6b`；
Hook SHA256 `5d3362471971d82f3c8be61e5a0fef9aa7121621cb8cedc93ccb7fc5ec225c10`；
Tests SHA256 `f2fa86ad45563da1989cd4f554b5b9c4a503a1b07f537b2196bc82792997c2d2`。
Evidence: direct/discovery各75 PASS；6个独立附加探针通过；L0 whitespace/topic/diff检查通过。
图标归档的PR30、issue29、Beads关闭与实际merge态CEv1五门禁由主代理直接复查。
其真实菜单栏视觉仍由用户延后且未验证。没有声明v0.6.5整体完成。
Completion gate: VERIFIED，2026-10-02 Neo4j MCP精确候选查询3/3通过；无missing/invalidated/unresolved。
ContentState: `hook-consistency-batch:state:71ea6647459f9678ae41a49d94c397bb52782d9780d149ba947be9203d00e77e`。
该候选覆盖全批变更，随后仅将本行的待查询状态更新为实际门禁结果；产品/测试未变。

### Task checkpoint

提交建议：精确候选Task门禁通过后执行用户已授权签名提交。
推送建议：核实签名与范围后普通push及一个draftPR；当前head远端review/CI通过才合并。
