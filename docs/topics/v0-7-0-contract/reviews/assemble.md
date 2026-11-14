---
status: active
topic: v0-7-0-contract
subject: assemble
---

# Assemble Review

## Round 1 — 2026-11-01

## 📋 scan-performance 批次装配评审

📊 总体评分: 9/10

✅ 结论: PASS

- Reviewer: Codex；本轮独立检查装配交互，不承担源专题修复。
- Method: 只读 Git/GitHub 对象与精确差异核验、CI 结果复用、合同边界检查。
- Scope: PR #61 当前预合并候选的集成交互；不是整个 assemble 任务或版本验收。
- Reviewed state: contract HEAD `2c53aa4a2344ac1114e907898f2aa48c0a53dfa6`；
  source `feature/scan-performance` = `d0bd36ea63bd65da21d5a3c6d18bca351cdc8e70`，
  tree `7d4b1e210b7b9c2e322c2b54b1b8bffd51161cd6`；target `main` =
  `d78c34583bf34175512c31765d4d4358b8049664`；候选 merge
  `8be912c12e1c20ab4bb2b907dda7ac6ceef271ee`，tree
  `b4211bf442304e85c2dfbd4bdc480bff1ca77150`。
- Operation: clean three-way；候选父提交依次为上述 target、source。共同基线为
  `b450704bc43807487a51a777a90dc4f8793d30dc`。这是 GitHub 生成的候选，不是已合入 main。
- 完成门禁: NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无本批次新增集成缺陷。整体任务尚未完成属于明确保留的验收边界，不包装成批次缺陷或整体 PASS。

### 🟡 改进建议 — 推荐

无新增发现。不得以建议为由恢复用户已停止的追加 PR 评审循环。

### 🟢 优点

候选保留 source 全部产品代码、测试、配置及文档；相对 source 的差异恰为
`LICENSE`、`README.md`、`README_zh.md` 的许可信息。三文件 blob 均与 target
一致，没有手工冲突解决或扫描、SQL、定价、分类、回放、发布路径的新组合变化。

### 📝 总结

本轮 PASS 仅认证上述精确候选的装配兼容性。target 自共同基线新增的内容只有 MIT
许可证及两处 README 链接；source 没有修改这三文件。候选相对 source 也恰为相同
三文件，因此结论依据实际结果内容而非仅依据 MERGEABLE 或没有同文件冲突。
许可证及链接不引入运行时接口、配置或数据依赖，未发现需新增产品测试的集成交互。

#### 用户决定及残余边界

用户本次原话：“几轮review 都是 p2问题 不再继续 这是我的决定”，随后明确“继续啊”。
记录为停止追加 PR Code Review/P2 修复循环并继续 assemble 评审；不推导为 blanket
产品验收、未知缺陷豁免、成员删减或合并授权。本轮没有新增待处置 finding。

PR 评论 `6558798321` 的 no-major-issues 回执针对 `fa8fe4e3`。
随后 `a70c2b9` 修改最终发布的代际冲突重试，`d0bd36e` 记录第七轮修复交接。
该改动没有新的独立 PR 回执；按用户决定不再把追加回执作为本批次前置条件。
历史修复、失败与旧证据保持原样；不将旧 source 的 VERIFIED 改写为新 source 验收。
持续代际冲突耗尽重试时仍可能导致本次刷新失败，交接记录已明确该限制。

保留既有 P04/RSS 取舍、隔离验收偏好恢复限制及 installed Widget/manual UI、L4
未覆盖边界。pricing、auxiliary 的整体验收与最终联合验收仍待完成。合并后的实际
Git 身份、集成证据与主线状态协调尚未发生，整个 assemble Dev/Review 不勾选。

#### Evidence

- `gh pr view 61 --json headRefOid,baseRefOid,potentialMergeCommit,state`：OPEN，
  得到上述 source/target/candidate。GitHub Git commit API 返回上述精确双父提交与树。
- GitHub compare API：`source...candidate` 只有三处许可差异；
  `b450704...target` 同样只有这三文件。对应 blob：LICENSE
  `284fa4cf7f4cdfc371a242e53681d46cd0b6aa87`，README.md
  `aded0faad44073f7092f1d0d314e44a260426539`，README_zh.md
  `ac09f6087af9a8ba5957b6a0c0e8798418c414c1`。
- `git diff --name-only b450704 d0bd36e -- LICENSE README.md README_zh.md`：空。
- 当前 source 的 Actions `41313393708`、`41313390336` 各自 classify、documentation、
  verify、desktop 全部 SUCCESS。复用其结果，没有仅因评审阶段变化重跑产品套件。
- 本地没有 target Git 对象，改用 GitHub 精确对象 API 读取；未 fetch、修改 refs 或合并。
- L0：`python3 scripts/check-docs.py`、`python3 scripts/check-review-records.py
  docs/topics/v0-7-0-contract/reviews/assemble.md`、`make check-whitespace`、
  `git diff --check` 均通过。结构检查不认证语义或 CE 门禁。

#### Completion evidence and handoff

Owner: `ad-v0-7-0-contract-assemble` / WorkUnit `v0-7-0-contract:assemble`。
固定 provider gate 模板确认四项 required criteria：membership、integration、
combined-acceptance、records。旧 preparation 状态没有任何可用顶层通过证据；
本轮最终状态 `v0-7-0-contract:assemble:review-20261101-3f415c91` 绑定 contract
HEAD 与 tasks.md blob `3f415c9122b28c6fab7a260422c2325d08c276d6`，另记录上述
source/target/candidate。保留旧状态及历史观察，不将批次 PASS 等同整体通过。

固定 `gate-status.cypher` 对该最终状态的实际查询返回 NOT_VERIFIED，reason 为 null；
missing_criteria 为上述四项，invalidated_evidence、unresolved_candidate_impacts、
reused_evidence、newly_produced_evidence 均为空。ContentState 写入后读回确认 1 个
节点及完整身份一致。未新增 aggregate passing Evidence，未越过包含单元边界。
允许范围仅本批次只读评审与必要记录；授权上限不含提交、推送、合并或发布。
Beads 已置为 in_review 并解除 assignee，保留未完成任务及所有历史。

下一边界为显式授权后的 PR #61 合并及实际结果证据协调；仍需其余成员和联合验收。
Task 的提交/推送建议须等待门禁 VERIFIED；当前不创建 contract-wide PR。

### Post-merge reconciliation — 2026-11-01 (historical; signature interpretation corrected below)

用户明确授权“合并 PR #61，并核验实际合并结果及同步该批次证据”。合并采用
merge commit，API 以完整 source SHA 为前置条件；未 squash、rebase 或删除分支。
实际 merge `6fff208ac852b191c0a62df1f5be34c937601cfe`，合并时间
`2026-11-01T15:25:39Z`，远端 main 指向该提交。实际父提交依次为本轮 target/source；
实际 tree `b4211bf442304e85c2dfbd4bdc480bff1ca77150` 与候选完全相同。
因此本轮集成交互 PASS 适用于实际结果；未按阶段重复执行未变化产品的测试。

实际 subject 为 `perf: integrate scan performance improvements (#61)`，有非空说明
正文及精确 Codex trailer。GitHub commit verification 为 `verified: true, reason: valid`，
签名头却是 `BEGIN PGP SIGNATURE`。这是 GitHub 服务端 merge 方式产生的签名，
不满足 Project Rules 的 SSH 签名要求；此前没有取得该例外。责任在本次执行选择，
不是源专题缺陷，也不以用户停止 P2 循环为该偏差的豁免。不得擅自改写已发布历史。

实际合并后的 CI `41321169520` 尚在运行；源/PR 的八项成功记录仍仅证明其原目标。
批次 WorkUnit `v0-7-0-contract:integration:scan-performance` 绑定实际 merge，
分别记录 ancestry、reviewed-result、postmerge-ci、delivery-signature 四项，后两项
保持未验证，直到获得实际结果或显式签名策略处置。批次门禁不能提升为 VERIFIED。
整体 assemble 仍待其余成员、全部集成与联合验收；本轮历史 PASS 不作改写。

实际合并态 CEv1 查询为 NOT_VERIFIED：ancestry、reviewed-result 两项有精确目标
pass evidence；delivery-signature、postmerge-ci 为 not_verified。invalidated 与
unresolved 列表为空。10 个节点、13 条关系已预检并读回确认，旧证据未覆盖改写。
契约工作区文档/评审结构、空白和 diff 检查通过。main 本地 `docs/status.md` 已补充
集成状态（未提交）；其 diff 检查通过，但该工作区全量文档检查被原有
`docs/fixes/local-schema-release-skew.md` 的不完整评审段落阻断，未修改该无关文件。

### Reconciliation correction — 2026-11-03 (historical; signature interpretation corrected below)

`gh run view 41321169520` 已确认 completed/success；classify、documentation、
verify、desktop 四项均 success。追加实际 merge 的 postmerge-ci pass evidence，
supersede 旧 pending 观察；保留当时尚在运行的历史事实。

签名规则核对仍只能确认 Project Rules 明文要求 SSH，没有发现 GitHub 合并豁免。
执行者曾声称该规则“只适用于本地提交”，随后撤回；该解释没有依据。
执行者也曾承诺直接撤销签名门禁，该承诺不能作为删除准则或豁免规则的授权。
实际 PGP 签名有效、产品内容和 CI 通过；SSH 合规未获证实。保留执行偏差和
未完成的策略处置，不将其描述为产品缺陷，也不要求重新评审代码。

### Authoritative signature-scope correction — 2026-11-03

用户明确反对将 GitHub PR merge 定性为签名不合规，并要求执行此前承诺的错误记录
纠正。以此作为本次适用范围的明确处置：GitHub 服务端创建的 merge 使用其有效 PGP
签名，不能据此要求用户批准 SSH 例外。前两节的相反定性及“等待签名处置”均已撤销；
它们保留为执行错误的历史，不再代表当前阻塞。不是把 PGP 声称为 SSH，也没有修改
本地提交的签名规则。未改写 merge 或删除历史证据。

对实际 merge 的 GitHub verification `verified=true, reason=valid` 追加有效证据，
结合用户的适用范围纠正，supersede 原 delivery-signature 未验证观察。
已成功的实际合并 CI、精确树及双父提交核验继续复用。本批次签名不再待用户确认。

固定 provider gate 对实际 merge 再查询返回 VERIFIED 4/4，missing_criteria 和
unresolved_candidate_impacts 均为空。旧 CI pending 与错误签名阻塞观察因明确
supersedes 显示 invalidated，不再适用于当前结论；其余有效证据复用。
此结果仅属于 `v0-7-0-contract:integration:scan-performance`，整体 assemble 的
四项聚合标准和其他成员仍未完成。上方 Round 1 的整体任务门禁保持其历史含义。

## Round 2 — 2026-11-14

## 📋 auxiliary-session-classification 批次装配评审

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Codex，独立于本批次准备者 Claude Code，未委派。本会话此前评审并交付源主题的 Round 6 记录，未编写源产品实现或本次三方候选；不把源评审当作本批次集成交互的独立证据。
- Method: 只读核对实际 Git tree、双方差异、远端引用、源验收沿袭与治理消费者；隔离导出候选树，运行受影响文档和工具检查。不使用其他工作区的 CodeGraph 索引推断此候选；本次已由精确九路径差异确定范围，直接检查对象内容。
- Scope: 本批次三方候选的兼容性与记录边界；不是整个 assemble 或版本验收。
- Workspace: `v0-7-0-contract` / `feature/v0-7-0-contract`；已核对全局 binding、Git common directory 和执行根包含关系。
- Reviewed state: contract HEAD `26c9377dcb29b4f3b77c48baec77af51ebf8ff7b`；同步后 tasks.md blob `f3e682cc89116dd90957b9e19bb7c095d6b14ebe`。
  source `feature/auxiliary-session-classification` = `855a811ad7b93298236c212ebd13ed4fcf2d58dc`，tree `dd2da688951d969aef165444bb767aa696b38e95`；
  target `main` = `90535085723f013cb8b24d83f83015e7d60d53b0`，tree `5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`；
  merge base `660c81586f4c13e04e66dbc05cde5331bf09acec`；候选 tree `29940cefbc6274ac4e014f04e72c24d24ba7ef26`。
- Operation: clean three-way。候选为准备阶段生成、现存且本轮直接读取的 Git tree；尚无实际合并 commit，不虚构双父提交或已集成状态。
- ContentState: `v0-7-0-contract:assemble:review-20261114-f3e682cc`。
- 完成门禁：NOT_VERIFIED。

### 🔴 严重问题 — 必须修复

无本批次新增集成缺陷。剩余成员、实际合并及联合验收是保留的整体边界，不将其包装为本批次代码缺陷。

### 🟡 建议改进 — 推荐

无新增发现。源记录的既有排除与范围外问题仍由其原载体承接，没有在本轮宣称修复。

### 🟢 优点

- 候选相对 source 仅有九条目标侧治理路径；生产代码、测试、配置、依赖、稳定 specs 与源提交完全一致。
  八个不重叠的治理文件逐字节等于 target；roadmap 相对 target 只新增九行历史来源关联计划，保留 target 的 Beads 权威及历史 Backlog 前言。
- 唯一文本重叠 `docs/roadmap.md` 的来源关联条目日期为 2026-11-03，早于 2026-11-07 规则切换；其载体 `ad-codex-memory-source-linkage` 实时状态 deferred。
  它是规则明确保留的历史 intake，不是本轮重新创建一份活跃待办镜像。
- Hook 的目标侧变化仅为注释与用户提示；`FINDING_CARRIER` 正则保持历史 Backlog 兼容，不与源评审记录格式冲突。
  新发现采用 Beads 的规则、稳定导航链接与文档检查器的完整路径过滤在组合树上相容。

### 📝 总结

本批次兼容性 PASS，不代表 `v0-7-0-contract:assemble` 整体 PASS 或已交付。
两项任务矩阵保持未勾选；准备者已有的 tasks.md 改动保留，本轮仅追加评审及状态投影。
没有创建 PR、合并、提交、推送、归档、安装或发布。

Evidence:

- `git ls-remote` 确认 source 为 `855a811…`、main 为 `9053508…`；GitHub 搜索 source → main 的开放 PR 返回空。
- `git merge-base` 为上述 `660c815…`。`git diff --name-status source candidate-tree` 恰为 `.agent-instructions/` 四文件、AGENTS.md、docs/README.md、docs/documentation-workflow.md、docs/roadmap.md 与 scripts/hooks/beads-consistency.py。
  八个非 roadmap 路径的 `git diff --exit-code target candidate-tree -- <paths>` 退出 0；roadmap 的 target → candidate 差异只有上述九行。
  未依赖「无冲突」字样替代实际树检查，也未执行新的 merge 或改写引用。
- `git diff 74964a2 855a811` 在排除源主题 tasks 和 Round 6 记录后为空；源 Topic 固定 gate 模板查询为 VERIFIED 2/2，无缺失、失效或未解决影响。
  其目标仍是 `auxiliary-session-classification:integrated-acceptance:commit-74964a2a0a8a4bad`，不把新提交 SHA 当成已经存在的新 gate。
- CI run `43558290460` 四个作业 success，但 desktop 与 verify 走 documents-only 分支，Swift/Go 产品步骤实际 skipped。
  本轮没有把这些绿色作业声称为新的全套运行；产品验收复用源 Round 6 对 `74964a2` 的 run `43552367297` 与历史性能/native 证据。
- 从上述 candidate tree 隔离导出：以只读 Git 元数据运行 `python3 scripts/check-docs.py --base <source> --head <candidate-tree>` 通过。
  首次命令因未处于导出根而在参数校验处退出，未执行检查；更正 cwd 后通过。
- 同一导出的 `make check-ci-docs-tools`：CI 工具 50 项通过；Hook 93 项中 92 项通过，唯一失败为
  `test_head_text_returns_committed_content_and_empty_for_an_unrecorded_path`，因导出目录无 Git 元数据使 HEAD 读取为空。
  源码确认该用例只要求已提交路径非空、未记录路径为空和耗尽预算不调用 subprocess；给该单项补充只读 GIT_DIR/GIT_WORK_TREE 后通过（1 项）。
  未重复其余 142 项，不修改测试或把第一次失败隐去；该项验证的是 Git 读取契约，不是候选内容的 HEAD 身份。
- `make check-opencode-hooks` 在候选导出上通过；仅运行隔离测试，没有直接执行真实会话 Hook。
- 目标侧不改扫描、分类、定价、SQLite、helper 或 native 运行路径，因此没有新产品 L2/L3 缺口；不重跑源主题全量或性能矩阵。
  Settings 动画仍只在新 CI 断言请求与终态，中间高度由历史实测和源评审影响评估支撑；本地慢宿主失败未诊断、PTY/native 排除及性能取舍原样保留。

Aggregate gate gaps:

- `membership`：辅助会话批次尚未实际集成；`ad-cost-transparency` 实时为 deferred，但仍是契约选定成员，未因 deferred 自动移出版本。
- `integration`：本次只有预合并树及兼容性评审；实际 merge 身份、签名与 post-merge CI 尚不存在。该批次 integration WorkUnit 按准备记录在实际合并边界建立，不制造提前通过的门禁。
- `combined-acceptance`：扫描/回放、辅助过滤与价格一起的版本联合验收未完成。
- `records`：实际集成后的全成员台账、稳定合同和 docs/status.md 投影尚未完成；本轮不在 main 写一份并行进度记录。

这些缺口以当前 ContentState 的四项 not_verified 观察记录，固定 gate-status 查询保持 NOT_VERIFIED；不写聚合通过证据。
本轮 5 个节点与 8 条关系完成写入、关系预检和目标态回读；四项观察均 target-matched、非 malformed，reason 为 null，invalidated/unresolved 均为空。
契约工作区的 `python3 scripts/check-docs.py`、`python3 scripts/check-review-records.py --base HEAD docs/topics/v0-7-0-contract/reviews/assemble.md`、
`make check-whitespace` 和 `git diff --check` 均通过；临时候选导出在记录必要事实后清理，不作为持久引用。
允许后续范围是成员 PR 与实际集成前提，以及其余选定成员和联合验收；当前授权不包含这些 Git 交付动作。

Task checkpoint：`ad-v0-7-0-contract-assemble`；`v0-7-0-contract:assemble:review-20261114-f3e682cc`；NOT_VERIFIED（批次评审 PASS）。
提交建议：整体 Task 门禁 VERIFIED 后再提交完整 assemble 交付；前提为选定成员集成或明确处置、实际集成证据、联合验收与记录收口。若只交付本批次记录，须单独明确增量提交边界。
推送建议：整体 Task 门禁 VERIFIED 后，完成上述前提及授权提交/签名核验，再单独授权推送 `origin/feature/v0-7-0-contract`；不提前创建 contract-wide PR。
下一前提：授权从 `feature/auxiliary-session-classification` 向 `main` 创建成员 PR，以本轮 source/target/tree 为基线；不包含合并授权。

## Round 3 — 2026-11-14

## 📋 auxiliary-session-classification PR #64 预合并候选批次装配评审

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Antigravity，独立于源分支初始实现及 Round 2 批次准备，未委派。本会话在用户明确指令下执行了 PR #64 的两轮 `@codex review` 修复与校验，本轮以契约装配视角核验更新后的预合并候选树及跨分支交互，不把单分支修复或 PR 评论结案等同于已完成实际合并或版本总体验收。
- Method: 只读核对 PR #64 实时状态、源/目标分支引用、双亲与 merge-base、GitHub `potentialMergeCommit` 与本地 `git merge-tree --write-tree` 产物树；比对 Round 2 候选树（`29940cef…`）与本轮候选树（`8bc33989…`）的精确路径差异、源分支 CEv1 门禁绑定与 PR 全量 CI / `@codex review` 证据边界。九条目标侧治理路径与六条源侧 Go 修复路径均通过精确树对比确认无交叉，不跨工作区混用 CodeGraph 索引。
- Scope: PR #64 更新后预合并三方候选（`0feb0c6d89…` + `9053508572…` → tree `8bc3398976…`）的批次集成兼容性与记录边界；不是整个 `assemble` 或版本联合验收。
- Workspace: `v0-7-0-contract` / `feature/v0-7-0-contract`；已核对全局 binding、Git common directory 和执行根包含关系。
- Reviewed state: contract HEAD `26c9377dcb29b4f3b77c48baec77af51ebf8ff7b`；同步后 tasks.md blob `f64cf3cd7697d7ad9b5975d60e23bc1a7dcdcabc`。
  source `feature/auxiliary-session-classification` = `0feb0c6d8965b6037de4c7e04bee839b96039dd0`，tree `5eb8c6f0a737935b24cebb4016a44749ff7d67e4`；
  target `main` = `90535085723f013cb8b24d83f83015e7d60d53b0`，tree `5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`；
  merge base `660c81586f4c13e04e66dbc05cde5331bf09acec`（source 侧 36 个提交，target 侧 2 个治理提交）；
  GitHub 预合并候选 commit `4d8c54dd0125481f2d5dbbb514973e68f6195cba`（parents `90535085723f013cb8b24d83f83015e7d60d53b0`、`0feb0c6d8965b6037de4c7e04bee839b96039dd0`），候选 tree `8bc3398976468058299173b93b363155f0ce6d8c`。
- Operation: clean three-way。GitHub `potentialMergeCommit` 与本地 `git merge-tree --write-tree` 生成同一无冲突 Git tree `8bc3398976468058299173b93b363155f0ce6d8c`；PR #64 仍为 OPEN 待合并状态，尚无 `main` 上的实际合并提交。
- ContentState: `v0-7-0-contract:assemble:review-20261114-f64cf3cd`。
- 完成门禁：NOT_VERIFIED。

### 🔴 严重问题 — 必须修复

无本批次新增集成缺陷。PR #64 实际合并、合并后主干 CI 与签名验证、剩余成员（`cost-transparency`）及版本联合验收属于聚合任务未完前提，不将其包装为本批次代码或合并冲突缺陷。

### 🟡 建议改进 — 推荐

无新增代码或集成发现。源分支 CEv1 门禁仍绑定于 PR 修复前的 `74964a2a0a8a4bade0ba6c1cea25e8d0f4d6667a`（与 Round 1 `scan-performance` PR 修复后情形一致）；本轮如实区分源 CEv1 记录边界与 PR 修复提交（`a2b01a3`、`0feb0c6`）的全量 CI 及评审证据，不伪造或回写源主题 CEv1 节点。

### 🟢 优点

- 候选 tree `8bc3398976468058299173b93b363155f0ce6d8c` 相对更新后的 source `0feb0c6d8965b6037de4c7e04bee839b96039dd0` 仍仅包含九条目标侧治理路径（`.agent-instructions/` 四文件、`AGENTS.md`、`docs/README.md`、`docs/documentation-workflow.md`、`docs/roadmap.md`、`scripts/hooks/beads-consistency.py`），且这九条路径在 `8bc33989…` 中的 blob 与 Round 2 已核验通过的候选 tree `29940cefbc6274ac4e014f04e72c24d24ba7ef26` 逐字节完全一致。
- 本轮候选 tree `8bc33989…` 相对 Round 2 候选 tree `29940cef…` 的差异仅限 PR #64 两轮修复涉及的六个 Go 文件（`cmd/agentdeck/main.go`、`cmd/agentdeck/session_origin_repair_test.go`、`internal/desktop/session_scope.go`、`internal/desktop/widget_summary_test.go`、`internal/scanruntime/serving.go`、`internal/scanruntime/serving_test.go`），在候选树中与 source `0feb0c6d89…` 逐字节一致，与 target `main` 的两笔治理提交零重叠。
- 弥补了 Round 2 时 `855a811` 仅触发 documents-only CI 的证据局限：源分支在 `0feb0c6d8965b6037de4c7e04bee839b96039dd0` 上已完整执行并通过 GitHub Actions 全量产品验证（`push` run `43572725411` 与 `pull_request` run `43572728514` 的 `classify`、`documentation`、`verify`、`desktop` 共 8 个作业全部 `SUCCESS`），且 `@codex review` 第二轮复核（`IC_kwDOTe7lus8AAAABmcUAqA`）确认 `has_any_findings: false`、`has_unresolved_findings: false`，3 个历史评审线程全部 `isResolved: true`。

### 📝 总结

PR #64 更新后的预合并候选批次兼容性评审 PASS，具备在获得用户明确授权后执行合并到 `main` 的条件；但本结论不代表 `v0-7-0-contract:assemble` 整体 PASS 或已交付。
两项任务矩阵保持未勾选；工作区已有的 Round 2 与 PR 创建记录改动完整保留，本轮仅追加 Round 3 评审及状态记录。
本轮未执行 PR 合并、提交、推送、归档、安装或发布。

Evidence:

- GitHub PR #64 查询确认 `state: OPEN`、`mergeable: MERGEABLE`、`mergeStateStatus: CLEAN`；`headRefOid` 为 `0feb0c6d8965b6037de4c7e04bee839b96039dd0`（tree `5eb8c6f0a737935b24cebb4016a44749ff7d67e4`），`baseRefOid` 为 `90535085723f013cb8b24d83f83015e7d60d53b0`（tree `5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`），`potentialMergeCommit` 为 `4d8c54dd0125481f2d5dbbb514973e68f6195cba`（tree `8bc3398976468058299173b93b363155f0ce6d8c`）。
- 本地 `git merge-base 90535085723f013cb8b24d83f83015e7d60d53b0 0feb0c6d8965b6037de4c7e04bee839b96039dd0` 为 `660c81586f4c13e04e66dbc05cde5331bf09acec`（`git rev-list --left-right --count` 为 `2 36`）；`git merge-tree --write-tree` 输出同一 tree `8bc3398976468058299173b93b363155f0ce6d8c`。
- `git diff --name-status 0feb0c6d8965b6037de4c7e04bee839b96039dd0 8bc3398976468058299173b93b363155f0ce6d8c` 恰为九条目标侧治理路径；`git diff --exit-code 29940cefbc6274ac4e014f04e72c24d24ba7ef26 8bc3398976468058299173b93b363155f0ce6d8c -- .agent-instructions AGENTS.md docs/README.md docs/documentation-workflow.md docs/roadmap.md scripts/hooks/beads-consistency.py` 退出 0，证明九条治理与重叠路径相对 Round 2 候选树未发生任何变化，直接复用 Round 2 对该九条路径的 `check-docs.py`、`check-ci-docs-tools` 与 `check-opencode-hooks` 验证证据。
- `git diff --name-status 29940cefbc6274ac4e014f04e72c24d24ba7ef26 8bc3398976468058299173b93b363155f0ce6d8c` 恰为两笔 PR 修复提交（`a2b01a3cbab249d88cbca57f24c04ea9fa279cf1`、`0feb0c6d8965b6037de4c7e04bee839b96039dd0`）修改的六个 Go 文件：
  - `a2b01a3` 修复桌面小组件摘要在含 helper 行时误将 `hasCompleteCoverage` 置为 `false`（`internal/desktop/session_scope.go`、`internal/desktop/widget_summary_test.go`）以及 `sessions --fix` 在存在未修复项且非交互返回时丢失退出码 1（`cmd/agentdeck/main.go`、`cmd/agentdeck/session_origin_repair_test.go`）；
  - `0feb0c6` 修复从 pre-v13 数据库加载增量快照行时未对空 `session_origin` 归一化为 `user` 的问题（`internal/scanruntime/serving.go`、`internal/scanruntime/serving_test.go`）。
- 本地针对性测试 `scripts/run-go-test.sh ./internal/desktop ./cmd/agentdeck`、`scripts/run-go-test.sh ./internal/scanruntime` 与 `make check-whitespace && git diff --check` 通过；GitHub Actions 在 `0feb0c6d8965b6037de4c7e04bee839b96039dd0` 上的 `push` run `43572725411` 与 `pull_request` run `43572728514` 全套 8 个作业（含 Go 与 macOS Swift 桌面全套验证）全部 `SUCCESS`；`@codex review` 评论 `IC_kwDOTe7lus8AAAABmcUAqA` 给出 `has_any_findings: false`、`has_unresolved_findings: false`，三个评审线程（`PRRT_kwDOTe7lus63ZnYu`、`PRRT_kwDOTe7lus63ZnYv`、`PRRT_kwDOTe7lus63awJD`）全部解决。
- 源主题 CEv1 门禁状态保持在 `74964a2a0a8a4bade0ba6c1cea25e8d0f4d6667a`（Task 9 `9/9 VERIFIED`，Topic `2/2 VERIFIED`），不把后续两笔 PR 修复提交虚构为已写入源主题 CEv1 的新节点；源分支既有排除项（托管 runner Reduce Motion 下的中间帧断言、本地慢宿主单次未诊断用例、键盘/辅助功能范围排除及 `durability`/topic 生命周期属性残余）按原边界保留。

Aggregate gate gaps:

- `membership`：`auxiliary-session-classification` 的 PR #64 虽已通过评审与全量 CI，但尚未合并入 `main`；`ad-cost-transparency` 仍为契约选定成员（当前 `deferred`），尚未交付或由操作者变更成员决策。
- `integration`：本轮核验的是 PR #64 的预合并候选 commit `4d8c54dd0125481f2d5dbbb514973e68f6195cba`（tree `8bc3398976468058299173b93b363155f0ce6d8c`）；`main` 上的实际合并 commit、双亲祖先关系、合并提交签名及 post-merge CI 需在实际合并后建立并记录到批次 integration WorkUnit。
- `combined-acceptance`：代表性规模下扫描/信号回放、辅助会话过滤与价格透传共同作用的联合验收尚未完成。
- `records`：实际合并落地后的批次台账、稳定契约与 `docs/status.md` 投影及最终契约收口尚未完成。

这些缺口以当前 ContentState（`v0-7-0-contract:assemble:review-20261114-f64cf3cd`）的四项 `not_verified` 观察写入 CEv1，固定 `gate-status` 查询保持 `NOT_VERIFIED`；不提前写入聚合通过证据。
契约工作区的 `python3 scripts/check-docs.py`、`python3 scripts/check-review-records.py --base HEAD docs/topics/v0-7-0-contract/reviews/assemble.md`、`make check-whitespace` 和 `git diff --check` 均通过。

Task checkpoint：`ad-v0-7-0-contract-assemble`；`v0-7-0-contract:assemble:review-20261114-f64cf3cd`；NOT_VERIFIED（PR #64 预合并候选批次评审 PASS）。
提交建议：整体 Task 门禁 VERIFIED 后再提交完整 assemble 交付；前提为选定成员集成或明确处置、实际集成证据、联合验收与记录收口。若只交付当前批次记录，须单独明确增量提交边界。
推送建议：整体 Task 门禁 VERIFIED 后，完成上述前提及授权提交/签名核验，再单独授权推送 `origin/feature/v0-7-0-contract`；不提前创建 contract-wide PR。
下一前提：由用户明确授权合并 PR #64（`feature/auxiliary-session-classification` `0feb0c6d8965b6037de4c7e04bee839b96039dd0` → `main` `90535085723f013cb8b24d83f83015e7d60d53b0`，预期合并 tree `8bc3398976468058299173b93b363155f0ce6d8c`），合并后核验 `main` 实际合并提交、签名与 post-merge CI，并同步集成状态记录。
