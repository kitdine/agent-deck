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

## Repair handoff — PR4-E6 — 2026-09-15 — snapshot-performance

GitHub Codex 在 PR #4 的当前 HEAD
`e6d59e565ca61b5adf493173fdac502920b6415a` 上完成新一轮 review，并记录四条
source-branch finding。本节仅记录 Repair 的逐条处置与复评候选，不改写 Round 2
针对 `7e455a8f` 的历史 PASS，也不自行给出新的评审结论。修复后的五文件候选以
HEAD 加 code/test diff SHA-256
`1bc27d3de2dd5c23b688618136fb6983e77f61e00c2c78ae0d4c229d4f66cd06`
标识。

- `PR4-E6-F1`（P1，GitHub discussion `r4012545716`）closed in candidate：
  `SessionIndexEpoch` 在 generation 表缺失时返回稳定的迁移哨兵；session watch
  将其映射为过期 checkpoint，使后续受锁 scan 以可写方式完成旧索引迁移，
  不再因只读 epoch 查询直接退出。
- `PR4-E6-F2`（P2，GitHub discussion `r4012545730`）closed in candidate：
  session watch 把本轮已采集的 raw source fingerprint 直接传给 epoch 绑定逻辑，
  不再在同一次未变化 poll 中重复遍历全部 session roots。
- `PR4-E6-F3`（P1，GitHub discussion `r4012545742`）closed in candidate：
  terminal receipt 持久化失败仍作为调用方可见错误保留，但 server 总会删除已完成
  round、提升 pending round 并恢复 idle/later-request 队列状态。
- `PR4-E6-F4`（P2，GitHub discussion `r4012545748`）closed in candidate：
  `EnsureStateRoot` 创建新目录后再次解析 symlink，并仅用最终 canonical path 计算
  state ID，保证首次 client 与随后 worker 派生同一 socket。

Repair verification：

- 聚焦回归：`TestTerminalReceiptFailureStillAdvancesRoundQueue`、
  `TestPrepareStateRootRecanonicalizesNewDirectoryBelowSymlink`、
  `TestSessionWatchFingerprintReadsRootsOnce`、
  `TestSessionWatchFingerprintForcesLegacyIndexMigration` 全部通过。
- `scripts/run-go-test.sh ./...`：PASS。
- `scripts/run-go-test.sh -race ./internal/scanruntime ./cmd/agentdeck`：PASS。
- `make vet`：PASS。
- `make build-all`：PASS（darwin/arm64、darwin/amd64）。
- 修复前 PR HEAD 的两组 `verify` 与两组 `desktop` CI 均为 SUCCESS；这些远端结果
  不覆盖本地未提交候选，新的 CI 仍属于后续 push 后验证。

Repair complete；四条 finding 均已进入同一复评候选。aggregate `assemble` 仍保持
开放，提交、推送、PR 更新与 merge 均未在本阶段执行。

## Repair handoff — PR4-C847 — 2026-09-15 — snapshot-performance

GitHub Codex 对 PR #4 commit
`c8477711867d10e5dfdd0024784503143a6257ea` 的后续 review 新增三条 finding。
本节记录其 Repair 处置，不给出复评结论。修复后的八文件 code/test diff
SHA-256 为
`5c65b26fa6ba714f26c940978fba8e9a3d57149e7c35fbcb33c8d0c4e4663aa5`。

- `PR4-C847-F1`（P2，GitHub discussion `r4016831849`）closed in candidate：
  watch source 可在成功 scan 后重绑定 fingerprint；session watch 使用 scan 前捕获
  的 raw inventory 与 scan 后非零 session epoch，持久化值和进程内值保持一致，
  首次创建或迁移索引后不再多执行一轮 bootstrap scan。
- `PR4-C847-F2`（P1，GitHub discussion `r4016831863`）closed in candidate：
  usage/session 的内部 raw error 不再序列化到 JSON、NDJSON 或 receipt journal，
  scope error 只返回稳定的 domain-level 公共消息；`error_code` 继续保留。
- `PR4-C847-F3`（P1，GitHub discussion `r4016831878`）closed in candidate：
  admission 在 planning、budget wait 或 job send 阶段被取消时，会幂等终结当前及
  所有尚未 admission 的 stream，保证消费者、round 和 scan lock 都能收尾。

Repair verification：

- 四个新聚焦回归以及真实 session watch bootstrap checkpoint 断言：PASS。
- `scripts/run-go-test.sh ./...`：PASS。
- `scripts/run-go-test.sh -race ./internal/watch ./internal/ingest
  ./internal/scanruntime ./cmd/agentdeck`：PASS。
- `make vet`：PASS。
- `make build-all`：PASS（darwin/arm64、darwin/amd64）。
- PR `c847771` 的两组 `verify` 与两组 `desktop` CI：SUCCESS；这些结果不覆盖
  当前未提交候选，新的 GitHub Codex review 仍需先提交并推送候选。

Repair complete；三条 finding 均已进入同一远端复评候选。aggregate `assemble`
保持开放，本轮未执行 commit、push 或 merge。

## Repair handoff — PR4-C847-R2 — 2026-09-15 — snapshot-performance

The full local review of the uncommitted `PR4-C847` candidate recorded two
additional findings. This repair remains part of the same assemble task and does
not alter the earlier review verdicts. The current candidate's scanruntime repair
delta covers `internal/scanruntime/{scanruntime.go,receipts.go,scanruntime_test.go}`.

- `PR4-C847-R2-F1` (P2, macOS socket path) closed in candidate: a caller-supplied
  long `TMPDIR` no longer makes the per-user socket endpoint exceed the Darwin
  `sockaddr_un` limit. The compact fallback is checked for symlink/type and
  current-user ownership before use.
- `PR4-C847-R2-F2` (P2, receipt durability) closed in candidate: after the
  temporary receipt file is synced and renamed, the containing directory is
  opened and synced before `save` reports success, preserving the
  persist-before-acknowledge contract across restart.

Repair verification:

- Long-`TMPDIR` endpoint and parent-directory-sync regressions: PASS.
- `scripts/run-go-test.sh ./internal/scanruntime`: PASS.
- `scripts/run-go-test.sh -race ./internal/scanruntime`: PASS.
- `make vet`, `git diff --check`: PASS.

Repair complete; independent remote re-review remains required before delivery.

## Repair handoff — PR4-C847-R3 — 2026-09-15 — snapshot-performance

The next full review of the uncommitted `PR4-C847-R2` candidate recorded two
additional findings. This Repair remains in the same aggregate `assemble` task,
preserves all earlier review history and does not issue a re-review verdict. The
current eleven-file code/test diff SHA-256 is
`cdafa3053b8d1beef2bca4e5a33db9611589fa39fd332edbff05401c5dbe4bd0`.

- `PR4-C847-R3-F1` (P1, finite session checkpoint) closed in candidate: the
  worker derives the session watch checkpoint from the exact `ingest.Discover`
  inventory used by its round and persists it before publishing session
  `completed`. The session CLI and desktop refresh no longer overwrite that
  checkpoint from a pre-request observation; the locked watch path scans its
  captured inventory and then adopts the persisted value. A top-level `scan`
  now persists the same checkpoint without a legacy caller.
- `PR4-C847-R3-F2` (P2, post-rename receipt state) closed in candidate:
  receipt-journal save reports whether rename installed the replacement.
  `accept`, `terminal`, `terminalReceipt` and `prune` roll memory back only for
  pre-rename failures. A directory-sync or later permission error remains
  caller-visible while live memory continues to match the document already
  visible on disk and after restart.

Repair verification:

- Focused finite-inventory, checkpoint-before-completion, top-level scan and
  post-rename `accept`/`terminal`/`prune` regressions: PASS.
- `scripts/run-go-test.sh ./internal/ingest ./internal/scanruntime
  ./internal/watch ./cmd/agentdeck -count=1`: PASS.
- `scripts/run-go-test.sh -race ./internal/ingest ./internal/scanruntime
  ./internal/watch ./cmd/agentdeck -count=1`: PASS.
- `scripts/run-go-test.sh ./... -count=1`, `make vet`, `make build-all` and
  `git diff --check`: PASS.

Repair complete; both findings are ready for independent re-review. No commit,
push, PR mutation or merge was performed.
