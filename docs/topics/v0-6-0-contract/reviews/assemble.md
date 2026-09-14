---
status: active
topic: v0-6-0-contract
subject: assemble
---

# Version assembly reviews

## Round 1 — 2026-09-08 — schema-version-signal

### 📋 首批集成评审

📊 总体评分：9/10

✅ 评审结论：PASS

Reviewer: Codex. Method: 单评审者的集成交互检查，复用已独立评审的 feature
及契约文档；本轮准备者与集成检查者相同，不声称新增独立产品评审。
Scope: 首批 schema-version-signal；其余五个功能方向不在本批。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

无。

### 🟢 优点

- 六项源任务已交付；固定主题 gate 对源树返回 VERIFIED 3/3。
- main 自共同基线仅增加两份已通过复评的契约文档，未改变任何产品消费者。
- 不通过契约分支中转 feature；本记录和批次状态随 feature PR 进入 main。

### 📝 总结

Target: main `47370767d4f195dd6c23c663861a01cecc81f3ac`，tree
`850bd790b5de7885237d561681b59c9be80cc19e`。
Source: feature/schema-version-signal `58df42d2ee84923ce43224162260cd07cb0016fa`，
tree `263f3d6bb8f80b160f9a46ec214cb05918f97b4f`。
Common base: `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`。

Operation: clean three-way main-to-feature synchronization, followed by a
direct-to-main PR preserving ancestry. Synchronization required no conflict
resolution. The source's product code/tests/configuration and stable specs are
unchanged by integration; target-only paths are the two contract documents.
Additional integration edits are this record, contract batch handoff and the
integrated docs/status.md projection. Final signed source commit and GitHub
merge commit/tree are bound through CEv1 and the PR/Beads receipt, avoiding a
self-referential commit hash in this file.

Interactions checked: version membership versus feature assignment; source
governance correction versus contract worktree/PR topology; aggregate task versus
partial batch; topic-owned progress versus integrated project status. No runtime
consumer, SQL, wire fixture, dependency or build configuration changes originate
from the target branch. Existing product evidence therefore remains applicable.

Evidence:

- Six closed source task records; source topic gate VERIFIED 3/3, with no
  missing, invalidated or unresolved evidence.
- git merge-base and both three-dot changed-path sets; clean synchronization
  index added only contract tasks.md and reviews/tasks.md.
- Final candidate L0 topic-doc, whitespace and diff checks are required before
  commit; push/PR CI and branch protection must pass before merge.
- Inherited implementation evidence is retained at its original ContentStates;
  the batch uses explicit target-bound roll-ups, not relabeled historical facts.

完成门禁：VERIFIED（2/2，候选树 `1322e76c1bed6ad6c9434d4bd26472eb43e6d6c9`）

The fixed integration gate returned VERIFIED with no missing, invalidated or
unresolved evidence. L0 checks passed; signed synchronization commit `0d7a0e6`
contains that exact tree. This receipt-only update changes no integration
behavior. Final commit/CI/merge receipts are retained in CEv1 and Beads.

Retained limits: Task 5 manual larger-text/narrow-layout and VoiceOver/interaction
checks were explicitly waived by the user. No actual layout judgment, speech,
disclosure operation or notice click navigation is claimed verified here.
This batch does not complete aggregate assemble, archive topics or release v0.6.0.

Task checkpoint：ad-v060c-assemble-dev / batch schema-version-signal；content_state=1322e76c1bed6ad6c9434d4bd26472eb43e6d6c9；gate=VERIFIED；aggregate open。
提交建议：本批门禁 VERIFIED 后提交同步结果与三份集成记录/状态文件；用户已授权完整合入。
推送建议：检查签名、提交消息和目标后推送 feature/schema-version-signal，PR 指向 main，保留历史并等待 CI。

## Round 2 — 2026-09-14 — snapshot-performance

## 📋 snapshot-performance 集成评审

📊 总体评分：9.5/10

✅ 评审结论：PASS

Reviewer: Codex。Method: 单评审者的独立集成检查；复用已完成的源 topic 与
Lane A fix 评审，不重新评审其未变化的产品实现。Scope: 第二批
snapshot-performance；其余四个未完成方向及 aggregate assemble 不在本批。

Reviewed state: HEAD `7e455a8f1d6cf99152ea75c925f9194542ad9303`，源树
`74f6231890fa9895a83c73f100d392c224230ccc`；两份未提交集成文档候选的
workspace diff SHA-256 为
`6bb600fe36f838afbd7d2912315e60864795594485b2a247f08dd6ea5bb03f7a`，
对应 ContentState
`v0-6-0-contract:integration:snapshot-performance:candidate:6bb600fe36f838afbd7d2912315e60864795594485b2a247f08dd6ea5bb03f7a`。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 当前 main `f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f` 是源 HEAD 的
  merge base 和祖先；本地操作类准确归为 fast-forward，不需要制造同步 merge。
- 候选把 topic delivery commit `cbeaa4b2` 与其后的独立
  `xctest-state-isolation` 修复 `7e455a8f` 分开描述，并保留两者各自的评审与
  证据边界。
- contract tasks 与 integrated status 对性能未达标、最终 20 样本、V01-V19
  完整性及 real-helper/manual native 缺口使用一致的 accepted-exception 表述，
  没有把用户处置改写成技术 PASS。
- 批次状态明确保持 aggregate assemble、其余四个方向、PR、retirement 与
  release 未完成，避免部分批次越权关闭版本单元。

### 📝 总结

Target: main `f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f`，tree
`f95e1a3a264337a52ab8de0182c60615c520c849`。Source:
feature/snapshot-performance `7e455a8f1d6cf99152ea75c925f9194542ad9303`，
tree `74f6231890fa9895a83c73f100d392c224230ccc`。Common base 与 target
commit 相同。

Operation: direct-to-main fast-forward candidate；当前评审未执行 commit、push、
PR 或 merge。main 在分支创建后没有额外产品、配置或依赖改动，因此不存在
target-only consumer、冲突解决或组合状态。实际 result 产品树将与 source tree
一致；本批额外内容仅为 version-contract batch/status projection 和本评审记录。

Interactions checked: `internal/scanruntime` 的统一 worker/receipt 生命周期，
usage/session/desktop 调用路径，CLI 与 embedded helper stream/progress 协议，
derived cache generation/invalidation，store migration，以及尾部 XCTest state-dir
隔离。源分支的对应实现、消费者、回归与评审内容在 target 上未发生改变；因此
其 exact-state 证据可继续适用。CodeGraph 当前索引绑定 main worktree，工具明确
警告其结果可能缺少本分支符号；本轮未把该图输出作为关系证据，而以精确 Git
拓扑、源 diff、源评审记录与现有测试覆盖核验这些边界。

Evidence:

- `git merge-base main HEAD` 返回 `f2b7d23...`，且
  `git merge-base --is-ancestor main HEAD` 成功；`main...HEAD` 共 116 个源路径，
  没有 target-only 增量或冲突结果。
- `git diff --binary -- docs/topics/v0-6-0-contract/tasks.md docs/status.md |
  shasum -a 256` 返回上述 `6bb600fe...` 候选指纹；`git diff --check` 与
  `bash scripts/check-topic-docs.sh` 均通过。
- snapshot-performance 当前矩阵为 6/6 文档与 3/3 Tasks 通过；Task 3 的
  delivery acceptance exceptions 明确保留原始 fail/not_verified 技术结果。
- `7e455a8f` 具有 Good ED25519 signature、完整提交正文和要求的 Codex trailer；
  `docs/fixes/xctest-state-isolation.md` Round 1 为 PASS，修复门禁 VERIFIED。
- 源 topic 与 fix 的既有产品测试证据绑定其未变化提交；本轮只改变文档并复用，
  未因工作流阶段变化重复运行 Go、race 或 XCTest 套件。

Residual uncertainty: 远端 ref、PR 保护与 CI 尚未检查，因为本轮没有 push/PR/merge
授权；它们属于后续交付前提，不影响本地快进候选的 REVIEW PASS。已接受的性能
与 native/manual 缺口仍是显式残余限制，不在本轮被关闭或弱化。

完成门禁：VERIFIED（2/2；上述 ContentState）。本轮确认
`integration-readiness` 与 `source-continuity`，无 missing、invalidated 或
unresolved evidence。aggregate assemble 仍保持开放。
