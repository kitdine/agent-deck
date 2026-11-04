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
