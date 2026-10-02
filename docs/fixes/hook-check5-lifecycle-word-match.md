---
status: active
created: 2026-10-02
---

# 缺陷：Hook 将普通动词与引用误判为退役生命周期

## 现象

Beads `ad-bug-hook-check5-lifecycle-word-match`，用户明确授权 Lane A 与三个关联 Hook issue 共用一个 PR。
Base `b864ce41b8d6ce22c475fbc42db6c5cad6b19f6b`，branch `fix/hook-consistency-batch`。
隔离回归 `test_ordinary_verbs_and_quoted_retired_names_are_not_lifecycle_claims` 在未修改 Hook 的基线失败。

## 根因

Check 5 matched drafting/repairing anywhere in descriptions, including ordinary English verbs and quoted defect descriptions.

## 修复边界

Recognize affirmative lifecycle declarations with state-arrow sequences. Historical, quoted, negative or ambiguous prose remains nonblocking. Keep existing current-lifecycle and closed-task behavior.
代码限 `scripts/hooks/beads-consistency.py` 和对应 Python tests。
不访问真实产品数据、不写 Beads/CEv1 backend、不改变产品或阶段授权契约。
本 issue 保留独立 WorkUnit `fix:hook-check5-lifecycle-word-match`、回归和关闭证据。
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

## Review — Round 4

## 📋 GitHub 当前 head 评审

📊 综合评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

- GH31-F1（P2，4164362845）：`current_document_review`未识别既有路径限定blob，合法当前文档诊断漏报。
- GH31-F2（P2，4164362852）：任意prototype脏文件抑制无关当前文档诊断，违反subject边界。
- GH31-F3（P2，4164362856）：NUL Git输出含非法UTF8字节时strict decode异常。
- GH31-F4（P2，4164362863）：Markdown强调Lifecycle标签漏报。

### 🟡 改进建议

无。

### 🟢 优点

GitHub评审覆盖了本机原测试未覆盖的既有记录格式和文件系统字节边界。

### 📝 总结

Reviewer: GitHub Codex review5390091428；主代理源码核验并复现。
Reviewed state: commit `3aa1dadf83dfb72e9925e84fb96f9909ff17ae4c`。
Evidence: `output/hook-batch-evidence/github-red.log`，新回归在原head失败。
GH31-F3本机APFS拒绝创建非法名字（正规提升后Illegal byte sequence）；
以隔离子进程输出同样原始字节，实际触发subprocess UnicodeDecodeError。
这是字节流模拟，不声称APFS真实非法文件测试通过。
Completion gate: FAILED；旧head不得合并。

GH31-F1/GH31-F2/GH31-F3/GH31-F4 -> repaired in candidate：精确主体路径blob支持；
仅与本轮摘要绑定的specimen路径参与失效；filesystem codec+surrogateescape；
强调标签正规化。79项测试通过，等待独立复评及新head远端评审。

## Review — Round 5

## 📋 路径身份与specimen边界独立复评

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

GH31-F1仍部分开放：错误前缀路径被substring匹配，既有topic-relative `ux/view.md`未识别。
GH31-F2仍部分开放：bare prototype manifest与包含空格的specimen路径未正确失效。
主代理复现4个失败子场景，见 github-rereview-red.log。

### 🟡 改进建议

无。

### 🟢 优点

GH31-F3 CLOSED：真实子进程字节decode回归通过。
GH31-F4 CLOSED：强调标签可识别，普通引用仍不触发。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；Method: 候选只读复评和隔离探针，主代理复核。
Reviewed HEAD `3aa1dadf83dfb72e9925e84fb96f9909ff17ae4c`；Hook SHA256
`9c1b6a8567adf2f040b5e13d8ac565d6189dd78e53ea1938fb5a339252464ea9`；
Tests `e751d1af86b8603e6d7d6c5615c03defc0aaa4fb24c334d2f48b075ff18eb5ea`。
Evidence:79项PASS，但4个额外反例失败。Completion gate: NOT_VERIFIED。
GH31-F1/GH31-F2 -> repaired in candidate：有边界的精确repository/topic-relative身份；
完整带空格路径和whole-manifest绑定；80项PASS，等待复评。

## Review — Round 6

## 📋 文件名字节下游复评

📊 综合评分：8.5/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

GH31-F3仍部分开放：`report_fingerprint`使用strict `.encode()`，已解码的
surrogate文件名会再次抛UnicodeEncodeError。主代理与独立评审分别复现。

### 🟡 改进建议

无。

### 🟢 优点

GH31-F1 CLOSED：精确repository/topic-relative blob身份正确。
GH31-F2 CLOSED：bare manifest、空格路径和无关路径边界正确。
GH31-F4 CLOSED：强调声明与引用边界保持。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；主代理直接核验。Method: 80 tests和6个边界探针。
Hook SHA256 `dd38c8b7e75aeb34b8377b32f26448cbbb42d8349a0b577432835253bc25cbc9`；
Tests `602a62bdd8cd310853823a08f5e91c5d34badcdd002d9b832c0c284bf48dfdad`。
Evidence: filename-fingerprint-red.log复现实际encode失败。
Completion gate: NOT_VERIFIED。
GH31-F3 -> repaired in candidate：filesystem path以os.fsencode进入指纹；新增回归，81项PASS。

## Review — Round 7

## 📋 GitHub发现修复最终独立复评

📊 综合评分：9.5/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。GH31-F1 CLOSED：有边界的repository/topic-relative blob识别。
GH31-F2 CLOSED：仅绑定specimen路径与manifest失效。
GH31-F3 CLOSED：解码与下游filesystem路径指纹均安全。
GH31-F4 CLOSED：强调标签正确识别。HB1-F1/F2/F3保持关闭。

### 🟡 改进建议

无。

### 🟢 优点

四项远端发现及复评补充均有失败先行回归；81项PASS。
不改写已推送历史，保留旧head失败证据；无产品数据和新权限操作。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；主代理核验摘要及测试输出。
Method/Scope: GH31-F1..F4修复和完整Hook回归；隔离surrogate-path探针。
Reviewed HEAD `3aa1dadf83dfb72e9925e84fb96f9909ff17ae4c`；Hook SHA256
`4863b12f88af79dfecb42071d8c756477e8d6364acd5c3a64aceb22451c94706`；
Tests `08d5b3a8b6333b4bd95fd04cfcd444a56019d1a038556694346abb17081eea60`。
Evidence: 81 tests PASS，surrogate-path probe PASS，L0 diff/whitespace PASS。
Completion gate: VERIFIED，Neo4j MCP修复候选Task3/3；无missing/unresolved。
ContentState `hook-consistency-batch:state:a379d740284d9a4a3fa4a9327524efdab615c59f64a8a5aeb31c0826c623b933`。
本候选后的改动仅为将此门禁行更新为实际查询结果，代码和测试未变。

### Task checkpoint

提交建议：修复候选门禁通过后追加已授权签名提交。
推送建议：普通push同一PR，重新获得最终head远端评审及CI后才合并。

## Review — Round 8

## 📋 当前提交远端复评

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

- GH31-F5（P2）：bare prototype manifest 未解析 topic 内 specimen 路径；真实 widget/menubar 文档的 PNG 或 manifest 改变仍返回 current。
- GH31-F6（P2）：单标记 `*Lifecycle:*` 与 `_生命周期_` 漏报。

### 🟡 改进建议

无。

### 🟢 优点

此前 GH31-F1..F4 的四条讨论已带修复证据解决；本轮保留新增失败而不沿用旧 PASS。

### 📝 总结

Reviewer: GitHub Codex review 5390278946，exact HEAD `45026665dc8810811eca00ba40ca4e8e208d22e2`。
Method/Scope: 当前提交远端独立审查；主代理直接新增隔离回归复现6个失败子场景，见 `output/hook-batch-evidence/github-round8-red.log`。
主代理修复：从已评审文档的本地链接解析唯一 manifest、校验声明的完整 SHA256，并关联 manifest 自身和 files 路径；不明绑定保守跳过诊断。平衡单/双 emphasis 标签均识别。
82 tests、L0通过；真实 widget/menubar 两份文档 current=true，逐个已绑定 specimen/manifest dirty 时 false。
Completion gate: FAILED for affected source Tasks at4502666；修复候选待独立复评与新状态门禁。

## Review — Round 9

## 📋 Manifest 与 italic 修复复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。GH31-F5/F6 CLOSED；此前 HB1-F1..F3、GH31-F1..F4 继续关闭。

### 🟡 改进建议

无。

### 🟢 优点

绑定真实文档链接和 manifest 内容，且未扩大到无关 specimen；隔离测试与真实文档只读探针一致。

### 📝 总结

Reviewer: 独立 `hook_batch_review`，只读复评；主代理直接核对文件哈希、82项测试、真实两份文档所有specimen路径及L0。
Method/Scope: 新修复代码与回归；独立36项真实manifest探针及6项标签/引用探针通过。
Hook SHA256 `3f0597f27cbbc4188c14f4c0d00df73850ba18c26422f1f6893f9d851af0f94a`；tests SHA256 `22cc29a93c0512a7ae5d5c056313e3417afca1bfdd32ee195549119f690208c1`。
Reviewed HEAD `45026665dc8810811eca00ba40ca4e8e208d22e2` plus exact scoped candidate。
Completion gate: VERIFIED 3/3 at candidate `20411e2ba7d74753d0efef12f60a482afe77fa83ca61f79cec866c0215e087c4` through Neo4j MCP; prior4502666 failures preserved. Signed source requires explicit target-bound reuse assessment.

## Review — Round 10

## 📋 既有格式兼容性远端复评

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

- GH31-F7（P2）：health-recovery 的 source/specimens manifest 不能按 files 解析。
- GH31-F8（P2）：Markdown `+` 列表标记漏报；同时补足有序列表与三标记强调的同类路径。
- GH31-F9（P2）：snapshot-performance 现有 Git blob 与 Specimen manifest 标签不兼容。

### 🟡 改进建议

无。

### 🟢 优点

此前六条远端发现已闭合；保留新发现并检查仓库全部四份现有 manifest 格式。

### 📝 总结

Reviewer: GitHub Codex review5390403094，exact HEAD `acb02cd40d6fd278b9f56389a123f065b3b779de`。
Main直接复现7失败子场景（github-round10-red.log），修复后83tests通过。
支持files和source/specimens布局，来源仓库路径与manifest相对checks/specimen路径分别绑定。
真实snapshot-performance两份PASS/VERIFIED评审在未改动时current=true。
真实health-recovery最新轮声明hash不匹配且本身FAIL，仍不得沿用其旧门禁。
Completion gate: FAILED at affected old source；修复候选待独立复评与新状态门禁。

## Review — Round 11

## 📋 全仓库 manifest 格式冷复评

📊 综合评分：8.5/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

HB1-F4（P3）：legacy source空key或specimen空file在相对路径拼接后变为非空目录。
主代理两个隔离回归均RED（round11-red.log）；改为拼接前拒绝空/纯空白路径。

### 🟡 改进建议

无。

### 🟢 优点

GH31-F7/F8/F9 CLOSED；独立85项探针覆盖现有全部四份manifest、全部源/标本路径及无关文件隔离。

### 📝 总结

Reviewer: 冷独立 `hook_batch_review`，只读格式兼容性复评。
Reviewed Hook SHA256 `b2683eb11f21c529528f8e4c410ffa5df12c953c0a708bbe59aab85e101bc148`。
Main直接重跑83tests与L0；修复后83tests通过，等待独立复评。
Completion gate: NOT_VERIFIED，候选仍有未复评修复。

## Review — Round 12

## 📋 最终格式兼容性复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。HB1-F4 CLOSED；HB1-F1..F3、GH31-F1..F9继续关闭。

### 🟡 改进建议

无。

### 🟢 优点

兼容现有两种manifest布局及实际review标签，同时未知/畸形绑定保守跳过诊断。

### 📝 总结

Reviewer: 独立 `hook_batch_review`，主代理直接核对测试、哈希与L0。
Method/Scope: 最终代码/测试，83tests、snapshot-performance真实两份review探针PASS；复用仍适用85项manifest探针，空路径修复只拒绝不合法输入。
Hook SHA256 `b4422982101b34571fb57e274f970a3f4a62c10df2e92e47ddfa2c27d4509318`；tests SHA256 `46e563e8c3af1949d7e1b76aebfd411eee8f838d989a48ed49be0401c50dfb4c`。
Reviewed HEAD `acb02cd40d6fd278b9f56389a123f065b3b779de` plus scoped five-file candidate。
Completion gate: VERIFIED 3/3 at candidate `97cafcf8a817ca81512634b3a711e5c5231d591dce34d81e0ceef44ea7a895a8` through Neo4j MCP; historical source failures retained. Signed source needs target-bound reuse assessment.

## Review — Round 13

## 📋 内容身份完整性远端复评

📊 综合评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

- GH31-F10（P2）：仅看git status无法发现已提交的specimen/source变化。必须逐项验证manifest的SHA256。
- GH31-F11（P2）：整句加粗/斜体生命周期声明漏报。

### 🟡 改进建议

无。

### 🟢 优点

本轮发现改变了前轮证据的解释；保留历史，不将旧probe结果继续作为通过依据。

### 📝 总结

Reviewer: GitHub Codex5390551074，HEAD `8431acd4d75e2df4588a4989e3ca443e076e3b6c`。
重要纠正：前轮snapshot-performance unchanged/current=true只验证了解析兼容性，不能证明内容当前；其manifest14/28项实际已变。该正向结论已被本轮否定，正确结果应false。
Main隔离RED4（github-round13-red.log），修复为files与legacy两schema逐项读文件校验完整SHA256；整句/标签/状态平衡强调在quote masking后归一化。
83tests与L0通过，待冷独立复评。Completion gate: FAILED for affected prior source。

## Review — Round 14

## 📋 完整manifest身份与声明复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。GH31-F10/F11 CLOSED；此前HB1/GH31发现继续关闭。

### 🟡 改进建议

无。

### 🟢 优点

校验当前文件字节而非仅依赖dirty列表，旧已提交内容变化不能复用原样稿门禁。

### 📝 总结

Reviewer: 冷独立 `hook_batch_review`；main直接核对测试、文件hash、实际过期snapshot两个文档均false。
Method/Scope: 最终Hook/tests；83tests、12项两schema内容/缺失/错误hash/越界probe、84项声明/引用probe PASS；L0通过。
Hook SHA256 `443e807bab11c3bfadb55bfa5283ee58d4c80cd630369db7662218b4027e847a`；tests SHA256 `1d3df1ee45f226fe14d12081baa40ae35110abf163c7f70e99b313046746ee5b`。
Reviewed HEAD `8431acd4d75e2df4588a4989e3ca443e076e3b6c` plus scoped candidate。
Completion gate: VERIFIED 3/3 at candidate `6e9f94987f01773f0dfef9577c1c3c0db18c3d65e1543febe1eab152a3199b99` through Neo4j MCP. Superseded positive snapshot probes are not reused.

## Review — Round 15

## 📋 单文件身份入口自查

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

HB1-F5（P2）：直接 `prototype/path SHA256 <hash>` 身份也必须比较当前字节，旧实现只检查dirty。
Main新增2个RED后实现完整SHA256与root containment校验；83tests/L0通过，等待独立复评。

### 🟡 改进建议

无。

### 🟢 优点

主动检查同类入口，未仅修manifest一条路径。

### 📝 总结

Reviewer: 主代理有界自查（不是独立PASS）；HEAD `11c04e5c1c0c5ca9d9170063b0a625cdb270a5a4`。
Evidence: direct-identity-red.log两失败，随后83tests GREEN。
Completion gate: FAILED for this source gate-identity task；远端已启动的旧head评审不能替代新修复复评。

## Review — Round 16

## 📋 单文件身份解析冷复评

📊 综合评分：8.5/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

HB1-F6（P2）：直接身份省略digest时整条声明被忽略。HB1-F7（P3）：匹配hash后的英文句号被当成hash内容。
Main独立复现RED2（round16-red.log）；捕获缺省值并拒绝，接受句尾标点。83tests GREEN。

### 🟡 改进建议

无。

### 🟢 优点

HB1-F5的完整hash比较有效，含空格路径匹配正确。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；Main直接复现。Scope: 单文件身份解析。
Reviewed Hook SHA256 `1fb9c63470e64481da1d8d2972dff5eb081aac7236fe98f2062d6fdb104f9d40`。
Completion gate: NOT_VERIFIED for repair candidate，等待独立复评。

## Review — Round 17

## 📋 单文件内容身份最终复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。HB1-F5/F6/F7 CLOSED；此前所有HB1/GH31发现继续关闭。

### 🟡 改进建议

无。

### 🟢 优点

manifest与单文件两条入口都校验当前内容，缺失或不明身份保守跳过。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；Main直接核验83tests与L0、hash。
Method/Scope: 最终单文件身份补丁；独立8探针覆盖空格path、缺digest、句尾标点、错hash、dirty与已提交变化，PASS。
Hook SHA256 `32a220088ece8c45e09f78de033597cdc53a0801c55e00725abd6de17f2c9800`；tests SHA256 `1f63ecf78e0cb9e949e87f26a3789157a77046d6a0197c819d5bb841a6b2a4ae`。
Reviewed HEAD `11c04e5c1c0c5ca9d9170063b0a625cdb270a5a4` plus scoped candidate。
Completion gate: pending exact candidate query; prior failed state remains historical.

## Review — Round 18

## 📋 远端单文件与撇号复评

📊 综合评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

GH31-F12（P2）是已主动记录/修复的HB1-F5单文件hash问题别名。
GH31-F13（P2）：Don't与It's之间的真实声明被当成引用遮蔽。
Main RED1后区分词内apostrophe与真实单引号，并保留真正引用中含缩写的对照；84tests通过。

### 🟡 改进建议

无。

### 🟢 优点

单文件问题已在远端结果到达前完成独立R17复评；无绕过当前head门禁。

### 📝 总结

Reviewer: GitHub Codex5390661673，HEAD `11c04e5c1c0c5ca9d9170063b0a625cdb270a5a4`。
Evidence: github-round18-red.log；source gates FAILED，修复候选待独立复评。

## Review — Round 19

## 📋 弯引号缩写边界复评

📊 综合评分：8.5/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

HB1-F8（P2）：`‘It’s history. Lifecycle: open -> drafting -> closed.’` 内部弯撇号被当成闭引号。
Main RED1后同样采用词内apostrophe边界；ASCII/curly缩写声明及真正引用对照通过，84tests GREEN。

### 🟡 改进建议

父任务明确要求推送前完整现有语法正反对照检查；不增加新语法或大重构。

### 🟢 优点

GH31-F13 ASCII缩写和直接文件身份修复保持有效。

### 📝 总结

Reviewer: 独立 `hook_batch_review`；Main直接复现round19-red.log。
Completion gate: NOT_VERIFIED for repair candidate，完整契约矩阵复评待完成。

### 本轮冻结的解析契约与对照范围

- 文档身份：最新非围栏轮次内唯一完整40/64hex Git blob；支持document/文档/Git及精确repo/topic-relative文档path标签。不匹配、缺失、歧义不得沿用门禁。
- 聚合样稿：prototype/Specimen manifest完整SHA256，经已评审文档唯一local manifest链接定位；支持现有files(path,sha256)与source映射/specimens(file,sha256)两布局。manifest自身与每一实际文件字节hash均匹配才有效；unknown、missing、bad hash、越界均跳过诊断。
- 单文件样稿：明确prototype路径（含空格）及完整SHA256/digest身份；比较当前字节，缺失/不匹配不得因git status为空而通过；普通句尾标点不改变hash。
- 生命周期：明确Lifecycle/生命周期/状态流转标签，标准无序/有序list、平衡粗斜体（label或整句），有效状态箭头序列；自然语言普通动词、历史/引用/围栏/blockquote不报警，ASCII/curly词内缩写不吞掉真实声明。
- Git路径：NUL分隔、全量untracked文件、rename/copy destination、空格/特殊字符/文件系统字节surrogateescape到fingerprint往返。无真实DB或产品数据。
- 全部四份仓库manifest用于schema/source布局检查，不能把其历史hash自动视为当前；实际过期快照必须拒绝。每类配应报警/不应报警案例，未知格式不扩大支持承诺。

## Review — Round 20

## 📋 冻结解析契约完整正反矩阵复评

📊 综合评分：9.5/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。HB1-F1..F8、GH31-F1..F13全部关闭；GH31-F12为HB1-F5别名。

### 🟡 改进建议

无。保持现有支持契约，不引入新语法或大重构。

### 🟢 优点

一次完整正反矩阵覆盖已知入口，主代理能独立重跑同一诊断而不是仅引用评审报告。

### 📝 总结

Reviewer: 冷独立 `hook_batch_review`；Main读取临时脚本确认隔离边界后直接重跑，结果一致。
Method/Scope: 冻结R19契约全部入口，1130checks/0failures：manifest251、schema22、direct77、document8、lifecycle756、apostrophe11、gitbytes3、actual-stale2。仓库direct/discovery84tests PASS，L0通过。
四份真实manifest保留拓扑但在临时目录复制当前文件并重算hash建立有效正例；逐项dirty、committed-stale、missing为反例。实际过期snapshot评审为false，未将历史hash视为当前。
原始Git bytes通过真实隔离子进程输出验证，APFS仍未声称支持创建非法UTF8文件。
Hook SHA256 `bf757f6fb31424ba760bd9d43f4bbaea5352160e646f14a1353869812538aa8f`；tests SHA256 `77c399eb4377301a63e02ad615c213a95f8d9a02336a6894803999098f511da1`。
Evidence: `output/hook-batch-evidence/final-paired-matrix.py` SHA256 `34353915bcb2db3335e347f8f22daede39d6742eb3f69e99c4a89e7781104311`；log SHA256 `b519b6577855bbaf5e088e08d4e9363875cc365cfbdc8ea06eccd6470ebb061a`。均忽略的本地证据，不写生产数据。
Reviewed HEAD `11c04e5c1c0c5ca9d9170063b0a625cdb270a5a4` plus scoped candidate。
Completion gate: VERIFIED 3/3 at candidate `048bac3b670a19a194e42ee401fd5bcd6f28e812e5ecce46dfefaf95be6939da` through Neo4j MCP; no historical failed observations reused as pass.
