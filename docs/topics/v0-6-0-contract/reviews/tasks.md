---
status: active
topic: v0-6-0-contract
subject: tasks.md
---

# tasks.md 评审记录

## Round 1 — 2026-09-08

### 📋 v0.6.0 契约任务分解评审

📊 总体评分：8/10

✅ 评审结论：FAIL

Reviewer: Codex

Method: 单评审者、基于当前仓库规则的文档契约检查；接受已有 Draft
handoff，未参与本轮之前的草案编辑；未使用子代理。发现确定缺口后停止扩展
核验，不把本轮视为完整集成或运行时验收。

Scope: `docs/topics/v0-6-0-contract/tasks.md`，包括 Documents 声明、版本选择、
合入与收口边界；产品代码、测试、配置和其他 topic 均保持只读。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 本轮必须关闭

**V060C-R1-F1 — Low — Documents 矩阵缺少不适用文档类型的显式声明。**

- 位置：`docs/topics/v0-6-0-contract/tasks.md:18-22`。
- 行为风险：矩阵是文档集唯一声明，但只列出 `tasks.md`，无法从该权威矩阵
  区分 requirements、UX、architecture 是不适用还是遗漏；正文解释不能替代
  项目要求的显式矩阵声明。
- 证据：`docs/documentation-workflow.md:91-97` 区分 feature 与 version-contract
  的文档职责；其中要求在 Documents 矩阵显式标注不适用类型。该文件
  `:118-127` 再次要求矩阵声明和显式不适用标记。目标正文 `:13-16` 已给出
  不需要这些文档的理由，但矩阵 `:20-22` 未体现。
- 💡 修复：仅在 Documents 矩阵补齐 requirements、UX、architecture 的 `n/a`
  行，沿用正文理由；不要创建空文件或额外文档任务，不改变版本范围和任务分解。
- Disposition: OPEN，归属本文件的现有文档任务 `ad-v060c-doc-tasks-design`。

### 🟢 优点

- 六项选定领域与当前 roadmap 的版本选择对应，并保留明确排除项。
- 明确契约 PR 和 feature PR 均直接面向 main，集成按完整 topic 分批进行。
- 区分用户豁免与已执行验收，区分部分批次、aggregate assemble 和 release。

### 📝 总结

Reviewed state:

- Workspace: `agent-deck.v0-6-0-contract`，branch `feature/v0-6-0-contract`。
- HEAD: `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`。
- 初始文档 blob: `8b6795f803df0c20e986c461e261c57e2073c036`。
- 交接同步后的文档 blob: `d44a2bf8aa3ea485e07dbc28c2c6c464056889ba`。
- 最终 content fingerprint: `3ee82e7e75b024abce716d8f1284f09c20a0a98c130c04009a0366718077526c`。
- Recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- 同步只更新 Current handoff；发现所在矩阵和正文未改变，缺口仍适用于最终状态。

Evidence:

- 已核对工作区绑定、common Git directory、branch、HEAD 和 Draft handoff。
- 交接同步及评审记录新增后，`bash scripts/check-topic-docs.sh` exit 0；该结构检查没有检出
  上述显式不适用声明缺口，不能替代语义评审。
- `make check-whitespace` exit 0；`git diff --check` exit 0（后者不覆盖未跟踪文件）。
- 版本选择对照 `docs/roadmap.md:13-56`；合入方向对照
  `.agent-instructions/branching.md` 和现有 Draft handoff 的用户澄清。
- 初始 CEv1 gate 为 NOT_VERIFIED，三项 criterion 尚无证据。
- 记录最终文档状态的反证后，固定 provider gate 返回 FAILED；反证
  `target_matches=true`、`applicable=true`、`malformed=false`。WorkUnit 的三条
  requires 关系已读回；反证的 satisfies/observed_at 两条关系预检及门禁回读有效。
- 本轮不重新验收 schema feature 的历史通过声明，不查询其整个主题门禁，
  不运行 Go/build/integration suite。修复后再完成剩余评审和比例适当的检查。

完成门禁：FAILED

WorkUnit: `v0-6-0-contract:tasks.md`。目标 ContentState 为
`v0-6-0-contract:tasks.md:state:3ee82e7e75b024abce716d8f1284f09c20a0a98c130c04009a0366718077526c`。
本轮 document-set 反证阻止完成；contract 和 L0 标准未声明整体通过。
文档 Review 保持未勾选，原任务返回修复；两项实现任务均未启动。

下一步指令：修复：v0-6-0-contract / reviews/tasks.md / V060C-R1-F1

WORKFLOW_WORKSPACE: agent-deck.v0-6-0-contract

## Round 2 — 2026-09-08

### 📋 v0.6.0 契约文档复评

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer: Codex；修复作者为 claude-code。

Method: 单评审者逐项复核 Round 1 finding，复用本会话已核对的文档权威，
补充草案引用的源提交/树及 schema topic 记录核对；未使用子代理，未修复产品。

Scope: `docs/topics/v0-6-0-contract/tasks.md` 的文档集与版本计划契约。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

无未关闭问题，无新增 finding。

### 🟢 优点

- V060C-R1-F1 CLOSED：当前矩阵第 23–25 行分别列出 requirements.md、
  architecture.md、`ux/` 的 `n/a`；正文保留不适用理由，没有新增空文档。
- 六项选择与 roadmap 一致；整 topic 的批次门槛、feature 直接到 main、
  aggregate assemble 和最终契约收口边界清晰。
- 用户豁免作为未执行验收保留，没有被转换为实测通过。

### 📝 总结

Reviewed state:

- Workspace: `agent-deck.v0-6-0-contract`；branch: `feature/v0-6-0-contract`。
- HEAD: `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`。
- 修复交接 blob: `70dd9edbc5372056215b1d71db33ef3d270cca62`。
- Review 勾选及 Current handoff 同步后的最终 blob:
  `ea3664b61461e70b3ed2bab0c46ec727ab3f29b5`。
- Content fingerprint:
  `b118f97e0d1ec83d61a770f87144761b52c52d3b2771dbcfb4019bfc05718427`。
- Recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

Evidence:

- claude-code 修复交接 comment `01a0845d-30d4-7543-b6ac-f74d1a600837` 与
  当前修复 blob 一致；三行矩阵直接检查确认关闭唯一 finding。
- 本轮状态同步只改变 Review 勾选和交接摘要，不改变修复结果或契约。
- 复用 Round 1 的 roadmap 和 Branching 核对；再次核对完整目标文档，
  部分批次不关闭 aggregate task，延期需明确成员决定，合入须独立授权。
- `git show -s --format='%H %T' 58df42d` 确认源 commit/tree 与草案一致；
  schema topic 的 tasks 与 contract-reconciliation 记录保留六项交付、
  主题候选 3/3 门禁和 Task 5 豁免。它们仍是发现快照，实际 batch 必须重新
  解析当时源/目标与适用证据，不把文档复评作为集成验收。
- 最终状态检查：`bash scripts/check-topic-docs.sh`、`make check-whitespace`、
  `git diff --check`，以及本文链接指向的 roadmap 锚点和评审记录存在性。
  三项命令均 exit 0，链接可解析，结果已记录到该目标状态的 L0 criterion。
- CEv1 初始查询 NOT_VERIFIED，历史 Round 1 反证 target_matches=false；
  不覆盖历史反证，追加本轮最终状态的三项新观察。

完成门禁：VERIFIED（3/3）

固定 gate-status.cypher 回读确认三项证据均 target_matches=true、
applicable=true、malformed=false；missing、invalidated、unresolved 均为空。
三条新观察与六条关系的创建计数与输入一致，关系预检全部 ok。

WorkUnit: `v0-6-0-contract:tasks.md`。本轮未越过 topic 完成边界；
`assemble` 与 `v0-6-0-contract` 两个实现任务仍未启动。
当前计划文档通过，不代表版本已组装或具备 release readiness。

Task checkpoint：ad-v060c-doc-tasks-design；content_state=b118f97e0d1ec83d61a770f87144761b52c52d3b2771dbcfb4019bfc05718427；gate=VERIFIED。
提交建议：取得提交授权后，仅提交本 topic 的 tasks.md 与 reviews/tasks.md；遵守正文、贡献者归因和签名要求。
推送建议：取得推送授权并完成提交对象检查后，推送到 origin/feature/v0-6-0-contract；契约 PR 目标 main，PR 与合并另需授权。

计划文档检查点：文档评审及证据完成，等待授权交付；不在本次复评启动 assemble。

下一步指令：提交：v0-6-0-contract / tasks.md（含 reviews/tasks.md）

WORKFLOW_WORKSPACE: agent-deck.v0-6-0-contract
