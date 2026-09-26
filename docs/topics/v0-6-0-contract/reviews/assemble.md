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

## Repair handoff — PR4-1077 — 2026-09-16 — snapshot-performance

This handoff covers three signed commits made on top of PR #4 head `1077fbc`:
`b898577`, `44d0395` and `15584e8`. It records Repair dispositions only; it keeps
all earlier review history and issues no re-review verdict. Delivered state:
HEAD `15584e86d3067cd4c435e935057c33039b49316a`, tree
`cb238e76f49c61a29b3d77731a878d5187083507`, pushed to
`origin/feature/snapshot-performance`.

Scope note for the re-reviewer: commits `51b948a`, `51bb758`, `34ee4e3`,
`7705692`, `00729f1`, `d5298b2`, `3d71e36` and `1077fbc` landed after the
PR4-C847-R3 handoff. This record has no handoff section for them, and this
handoff asserts no disposition for their content. The complete unreviewed
integration delta since the Round 2 PASS is `7e455a8f..15584e8`.

- `PR4-1077-CI1` (CI `verify`, run `35098804057`) closed by `b898577`:
  `TestUnifiedScanRuntimeMatchesLegacyAcrossSourceMutations` failed under
  `make test-race` with `partial append shared scan: usage scan failed`. The
  underlying error was SQLite extended code 517 (`SQLITE_BUSY_SNAPSHOT`). Usage
  source publication uses deferred transactions that read before writing, and
  the round's session goroutine committed `watch.fingerprint.session` to the same
  core database in between. The session checkpoint now waits for the usage
  domain's terminal state after the session consumer is released, restoring one
  core writer per round. Out-of-round concurrent writers can still trigger 517;
  that pre-existing defect is tracked separately as Lane A
  `ad-bug-usage-scan-busy-snapshot` and is not repaired here.
- PR #4 review `5222665284` (2026-09-16, recorded here as `PR4-R5222-F1` to
  `PR4-R5222-F13`), verified against current code:
  - `F9` double range hashing, closed by `44d0395`: the coordinator and
    `ValidateCapturedRange` skip the second read when identity, size, mtime and
    ctime are unchanged; growth and same-size rewrites are still hashed and
    rejected.
  - `F10` lock contention, partly valid, closed by `44d0395`: admission already
    blocks on `wake` rather than spinning; the per-record coordinator lock in the
    reader is removed.
  - `F11` duplicated dev:inode extraction, closed by `15584e8`: session, usage
    and the derived cache use `ingest.FileGeneration` / `ingest.FileIdentity`.
  - `F1`, `F3`, `F4`, `F6`, `F7`, `F8`, `F12` not reproduced in current code:
    legacy refresh also returned errors rather than stale data; shared streams
    still pass through the batch orphan check; restore already reserves the
    sessions WAL, SHM and journal files; `OpenSessions` mints an epoch on rebuild;
    source updates still validate and commit in a transaction; helper capture
    drains output on timeout; `failureStage` is read by the contract report and
    tests.
  - `F2` by design: metadata is read outside the transaction and revalidated
    inside it by identity, size, change time and prefix hash. `F5` is no
    regression: the PR adds the XCTest refresh guard where none existed.
    No change for either.
  - `F13` commit attribution was corrected before this handoff and is not a
    code finding.

Repair verification:

- RED/GREEN: `TestSessionCheckpointWaitsForUsageCoreWrites` failed 5/5 with the
  checkpoint wait removed and passes with it. Under `GOMAXPROCS=1` and `-race`,
  the CI contract test failed 4 of 45 runs before `b898577` and 0 of 45 after.
- New ingest regressions `TestCoordinatorReadsUnchangedSourceBodyOnce` and
  `TestStreamRejectsSameSizeRewriteInDomainPublicationWindow`: PASS.
- `make check-whitespace`, `make check-go-test-runner`,
  `scripts/run-go-test.sh ./...`, `scripts/run-go-test.sh -race ./...` and
  `make vet`: PASS on the final code.
- PR #4 CI at `15584e8` (two `verify`, two `desktop`): SUCCESS.
- The three commits carry verified SSH signatures and
  `Co-Authored-By: Claude <noreply@anthropic.com>`.

Repair complete. An independent integration re-review of `7e455a8f..15584e8` and
a target-bound integration gate remain required before merge. This handoff
section itself is uncommitted.

## Round 3 — 2026-09-16 — snapshot-performance

## 📋 snapshot-performance 集成复评

📊 总体评分：9.5/10

✅ 复评结论：PASS

Checklist: 54/54 complete. Incomplete: None.

Reviewer: Codex。Method: `ln-12-delivery-reviewer` 的 Blue-only 复评；按项目
禁止未获请求的委派，因此未启动 subagent。逐项复核本记录 Round 2 之后的全部
Repair finding，并审查完整增量 `7e455a8f..15584e8`、受影响运行路径、最新 PR
拓扑与精确 HEAD CI。Scope: 第二批 `snapshot-performance` 的集成候选；其余四个
未完成方向、aggregate `assemble`、PR merge、topic retirement 与 release 不在本轮。

Reviewed state: target main
`f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f` / tree
`f95e1a3a264337a52ab8de0182c60615c520c849`；source and PR head
`15584e86d3067cd4c435e935057c33039b49316a` / tree
`cb238e76f49c61a29b3d77731a878d5187083507`。main 仍是 source 的 merge base
和祖先，operation class 仍为 direct-to-main fast-forward candidate。Reviewed
ContentState:
`v0-6-0-contract:integration:snapshot-performance:pr4:15584e86d3067cd4c435e935057c33039b49316a`。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无未关闭问题，无新增 finding。

### 🟢 优点

- `PR4-E6-F1`、`PR4-E6-F2`、`PR4-E6-F3`、`PR4-E6-F4` 均 CLOSED：迁移
  哨兵、单次 inventory 复用、terminal receipt 失败后的 round 推进及新建 state
  root 再规范化都保留在当前实现和聚焦回归中。
- `PR4-C847-F1`、`PR4-C847-F2`、`PR4-C847-F3` 均 CLOSED：session
  checkpoint 重绑定、公开结果/receipt 的 raw error 脱敏及 admission 取消后的全部
  stream 终结均存在于当前路径；稳定 `error_code` 保留。
- `PR4-C847-R2-F1`、`PR4-C847-R2-F2`、`PR4-C847-R3-F1`、
  `PR4-C847-R3-F2` 均 CLOSED：Darwin 长路径 fallback 的私有目录约束、receipt
  rename 后目录同步、有限 inventory checkpoint 及 post-rename 内存/磁盘一致性均有
  对应失败态测试。
- `PR4-1077-CI1` CLOSED：session checkpoint 等待 usage core writer，当前 HEAD
  的 race contract 与两套远端 `verify` 均通过；独立 Lane A carrier
  `ad-bug-usage-scan-busy-snapshot` 继续承接 out-of-round writer 问题。
- `PR4-R5222-F9`、`PR4-R5222-F10`、`PR4-R5222-F11` CLOSED；
  `PR4-R5222-F1`、`F3`、`F4`、`F6`、`F7`、`F8`、`F12` 在当前代码中仍不可
  复现；`F2` 是事务内再验证的既定设计，`F5` 未形成回归，`F13` 已在交付提交前
  纠正归因。逐项复核未发现 disposition 回退。

### 📝 总结

完整变更为 50 个文件、2559 insertions / 358 deletions。受影响交互包括 shared
ingest 的有限范围与 source identity、scanruntime admission/round/receipt 生命周期、
usage/session/core writer 时序、watch checkpoint、CLI/embedded-helper progress 与
公开错误边界、macOS 菜单栏失败呈现及 schema 26 fixture。当前实现把 source rewrite
防护放在读取后和领域发布前，把所有未 admission stream 在取消时终结，并让 receipt
在 rename 后错误时继续反映已安装文档；这些是前轮 finding 的 owning boundaries，
未以调用方特例遮盖。

Evidence:

- Git topology：`git merge-base main HEAD` 返回 `f2b7d23...`；PR #4 的 base/head
  分别为 `f2b7d23...` / `15584e8...`，GitHub 报告 `MERGEABLE`、`CLEAN`。
- 当前 HEAD 的两套 `verify` 与两套 `desktop` checks 均为 SUCCESS。复用同一 HEAD
  已记录的 `scripts/run-go-test.sh ./...`、`scripts/run-go-test.sh -race ./...`、
  `make vet`、`make build-all`、whitespace 与 runner checks；代码、测试、依赖、
  配置和 toolchain 未在这些结果后改变，因此不因复评阶段重复执行。
- 用户提供的最终 branch review 结果为无 actionable correctness defects，且受影响
  Go packages 与 command package 的 scoped regression tests 通过；本轮又直接核对
  finding 对应实现、测试与 consumer path，没有把该结论单独当作门禁。
- `git diff --check 7e455a8f..HEAD` 通过。已接受的 cold-import、unchanged-refresh
  CPU、20-sample、V01-V19 与 native/manual 例外继续按原技术结果保留，未改写为 PASS。

Residual uncertainty: merge 尚未执行，merge 后 main result identity、remote branch
protection 与 merge receipt 仍属于后续交付；四个剩余版本方向和 aggregate assemble
也保持开放。这些不是当前 fast-forward candidate 的复评 finding。

完成门禁：VERIFIED（2/2；上述 ContentState）。`integration-readiness` 与
`source-continuity` 均绑定当前 PR head，missing、invalidated 与 unresolved 为空。

Task checkpoint：ad-v060c-assemble-dev / batch snapshot-performance；content_state=v0-6-0-contract:integration:snapshot-performance:pr4:15584e86d3067cd4c435e935057c33039b49316a；gate=VERIFIED；aggregate open。
提交建议：提交本轮复评记录及同批 contract/status 同步；产品修复已在 `15584e8`，门禁 VERIFIED 后不再修改产品代码。
推送建议：取得推送授权、检查新文档提交的消息/归因/签名及远端仍指向 `15584e8` 后，推送 `feature/snapshot-performance`；PR #4 merge 仍需独立授权并保持 CI/branch protection 成功。

## Round 4 — 2026-09-17 — subscription-quota

## 📋 subscription-quota 集成评审

📊 总体评分：6.5/10

✅ 评审结论：FAIL

Checklist: 54/54 complete. Incomplete: None.

Reviewer: Codex。Method: `ln-12-delivery-reviewer` 的 Blue-only 初始集成评审；
按项目禁止未获请求的委派，因此未启动 subagent。Scope: 第三批
`subscription-quota` 的 main-to-feature integration-base refresh；审查
`aac6bb1` 的冲突解法、受影响消费者及 exact-state 开发验证。其余三个未完成
方向、aggregate `assemble`、push、feature-to-main assembly、topic retirement
与 release 不在本轮。

Reviewed state: target main
`4dd10f4bf0bfcea2b4cceede5ac9f24465f6a656` / tree
`e8f50cf605e520f694a81762d87bfa8592cde1a5`；source pre-refresh
`9e1e869`；three-way merge candidate
`aac6bb187b5f42923d8d6994189301833dafcdc0` / tree
`adf6ed43062daa6bc14c9bc3446ce636d925ed4f`。Parents are `9e1e869` and
`4dd10f4`；common base `4737076`。Reviewed ContentState:
`v0-6-0-contract:integration:subscription-quota:candidate:aac6bb187b5f42923d8d6994189301833dafcdc0`。

### 🔴 严重问题 — 必须修复

[`apps/macos/AgentDeckApp/AgentDeckApp.swift:72`] `A4-F1`（P1）— 冲突解法删除了
subscription-quota 已交付的 fail-closed hosted-XCTest guard，却只以禁止自动
refresh 替代，未隔离新加入的设置窗口路径。

- 行为风险：当 hosted XCTest 没有收到 `AGENTDECK_TEST_HOME` 时，delegate 仍在
  第 73、82–84、125–136 行构造使用真实 HOME 的 `EmbeddedHelperRunner`，并把
  真实 `~/.claude/settings.json` URL 注入 `QuotaSettingsController`。测试或 harness
  一旦打开 Settings，`QuotaSettingsController.load()` 会先直接读取该文件，再通过
  同一 runner 调用 quota-settings transport；因此 `automaticRefreshEnabled=false`
  只能阻止启动时/周期 refresh，不能兑现“hosted test 不构造或触达真实 state”的
  已交付安全边界。
- 证据：源 topic 当前权威
  `docs/topics/subscription-quota/tasks.md:1073-1080` 明确声明 Task 7 保留该 guard；
  source parent `9e1e869` 在 `XCTestConfigurationFilePath` 存在且
  `AGENTDECK_TEST_HOME` 缺失时于 delegate 初始化阶段 fail closed。merge candidate
  删除该判断；`QuotaSettingsController.swift:61-69,126-136` 证明 Settings 的
  `onAppear` 路径读取注入 URL 并调用 transport。merge message 的安全论证只覆盖
  embedded helper 的自动 refresh，与此设置路径不等价。
- 有界修复：在 `AgentDeckApp.swift` 与 XCTest 启动环境的 owning boundary 恢复
  fail-closed 隔离，使 hosted tests 在构造 quota settings/helper transport 前必然
  获得并校验受控临时 HOME；同时保留 main 的 automatic-refresh 禁用与
  `debugTestHome` 前缀校验。增加一个能在移除隔离时失败的回归，覆盖“XCTest +
  缺失/不安全测试 HOME”以及 Settings load 不得读取或调用真实 HOME。不要仅恢复
  会让当前 hosted suite SIGILL 的旧判断；应修正 test-host 环境传递或等价的隔离
  注入，使 suite 可运行且安全边界可证明。

### 🟡 建议改进 — 推荐

无独立改进项；本轮在决定性 P1 finding 后停止扩大验证。

### 🟢 优点

- migration 冲突把 main 的 24–26 与 quota 的 27–30 保持连续，schema 常量、CLI
  rollback fixture、Swift wire fixture 与 desktop fixtures 同步为 30。
- desktop cache 与 quota 读取保持职责分离：usage/work-signals 可命中 derived
  cache，subscription 仍无条件从 core 读取；quota-first refresh 与 scan progress
  callback 均保留。
- 菜单栏与 prototype 冲突采用加法合并，quota panel 抑制 usage client controls
  的规则和 scan progress 状态都没有互相覆盖。

### 📝 总结

本轮检查 18 个 remerge-diff 文件，并把 13 个实际冲突解法映射到 schema、CLI、
desktop snapshot、macOS refresh/UI、fixture 与 prototype 消费者。实现者在同一
`aac6bb1` content state 记录的 `go build ./...`、`go vet ./...`、
`scripts/run-go-test.sh ./... -count=1` 与完整 `scripts/test-macos-app.sh`
均通过；`git diff --check 4dd10f4..aac6bb1` 也通过。根据 exact-state 复用规则，
本轮未重复这些未变化的 broad suites。CodeGraph 明确报告索引绑定 main worktree，
因此仅用于识别其不适用性；所有候选特有冲突路径均以 source 和测试证据核查。

通过的 hosted suite 不能关闭 `A4-F1`：当前 suite 正是通过删除 guard 并禁止自动
refresh 才恢复运行，且没有覆盖 Settings load 的真实 HOME 路径。该 finding 是
integration conflict resolution 引入的安全/状态隔离回退，违反源 topic 当前交付
契约，故本批评审 FAIL。aggregate `assemble`、push、assembly 与 release 继续开放。

完成门禁：FAILED。`integration-readiness` 被 `A4-F1` 的精确候选证据否定；
`source-continuity` 不足以覆盖该失败。

### 下一步指令

修复：v0-6-0-contract / assemble / A4-F1

## Round 5 — 2026-09-17 — subscription-quota

## 📋 subscription-quota 集成复评

📊 总体评分：9.5/10

✅ 复评结论：PASS

Checklist: 54/54 complete. Incomplete: None.

Reviewer: Codex。Method: `ln-12-delivery-reviewer` 的 Blue-only selective
follow-up；按项目禁止未获请求的委派，因此未启动 subagent。Scope: 仅复核
Round 4 `A4-F1`、其修复增量 `aac6bb1..f837666` 及修复触及的 hosted-XCTest
隔离路径；未重审未变化的 subscription 产品实现、其他三个未完成方向、
aggregate `assemble`、push、feature-to-main assembly、topic retirement 或 release。

Reviewed state: repaired source HEAD
`f8376660aeac324d1b7c223e85795707dae3cc29` / tree
`85a4bc5c9950841c2bce1ff5f6000eb64ffd26d4`；parent and failed state
`aac6bb187b5f42923d8d6994189301833dafcdc0`；target main remains
`4dd10f4bf0bfcea2b4cceede5ac9f24465f6a656` / tree
`e8f50cf605e520f694a81762d87bfa8592cde1a5`。Reviewed ContentState:
`v0-6-0-contract:integration:subscription-quota:candidate:f8376660aeac324d1b7c223e85795707dae3cc29`。

### 🔴 严重问题 — 必须修复

无。`A4-F1` CLOSED。

### 🟡 建议改进 — 推荐

无未关闭问题，无新增 finding。

### 🟢 优点

- `A4-F1` closed：`resolveDebugHome(environment:)` 现在是 delegate 构造所有
  real-HOME-touching objects 前的单一决策。存在 `XCTestConfigurationFilePath`
  时，缺失测试 HOME 返回 `.missingForHostedTest`，不安全 HOME 返回
  `.unsafeHome`，两者都在构造 runner、snapshot store、defaults 和
  `~/.claude/settings.json` URL 前 fail closed；只有已校验的临时 HOME 返回
  `.isolated`，`.real` 仅在非 XCTest 进程可达。
- `scripts/test-macos-app.sh` 同时保留 xcodebuild 自身的
  `AGENTDECK_TEST_HOME`，并增加 `TEST_RUNNER_AGENTDECK_TEST_HOME`；Xcode 为
  launched test host 去除前缀后，delegate 实际收到隔离 HOME，关闭了导致旧 guard
  SIGILL 的环境传递缺口，而不是再次删除安全边界。
- 新回归直接覆盖 hosted+missing、hosted+unsafe、hosted+isolated 和 ordinary
  process 四种决策；断言对象正是 `init()` 使用的 resolver，不以
  `automaticRefreshEnabled` 的间接结果替代真实安全契约。

### 📝 总结

finding disposition：`A4-F1` 从 OPEN 转为 CLOSED；无 regression、无 superseded
finding、无新增 finding。修复保持 main 已评审的 XCTest 自动 refresh 禁用，并把
subscription-quota 新增的 Settings direct-read/helper transport 纳入同一 fail-closed
HOME 决策。精确提交上的 `scripts/test-macos-app.sh` 通过 54+93+25 项 Swift tests
（一个既有 expected skip），且无本 worktree helper 泄漏；同一状态的
`bash -n scripts/test-macos-app.sh`、`make check-whitespace` 与
`git diff --check` 通过。若 `TEST_RUNNER_` 变量未到达 host，delegate 会在 suite
启动时进入 `.missingForHostedTest` 并失败，因此该完整套件结果也验证了传递链。

Go 产品代码未被修复提交改变；复用 `aac6bb1` 上已记录的 `go build ./...`、
`go vet ./...` 与 `scripts/run-go-test.sh ./... -count=1`，不因复评阶段重复执行。
提交 `f837666` 具有 Good ED25519 signature、完整 body 及 Codex/Claude trailers。

Residual uncertainty: 分支尚未 push，远端 CI、feature-to-main assembly 的最终
result identity 与 branch protection 尚未产生；这些属于后续交付边界，不是当前
repaired candidate 的 finding。aggregate `assemble` 仍为 partial batch task。

完成门禁：VERIFIED（2/2；上述 ContentState）。`integration-readiness` 与
`source-continuity` 均绑定 repaired candidate，missing、invalidated 与 unresolved
为空。

Task checkpoint：ad-v060c-assemble-dev / batch subscription-quota；content_state=v0-6-0-contract:integration:subscription-quota:candidate:f8376660aeac324d1b7c223e85795707dae3cc29；gate=VERIFIED；aggregate open。
提交建议：提交 Round 5 复评记录及同批 contract handoff 同步；产品修复已在签名提交 `f837666`，不得夹带其他任务或产品改动。
推送建议：取得独立推送授权并确认新的复评文档提交消息、贡献者 trailer、SSH 签名及远端基线后，推送 `feature/subscription-quota`；feature-to-main assembly/PR/merge 仍需各自授权和成功 CI/branch protection。

## PR #5 delivery receipt — 2026-09-19

### 📋 subscription-quota 集成交付核验

📊 总体评分：9.5/10

✅ 交付核验：PASS

Reviewer: Codex automated PR review plus maintainer disposition. Method: reuse
Round 5's independent integration review for the pre-merge conflict-resolution
and hosted-XCTest boundary; verify the actual GitHub merge, final protected CI,
and final review carrier disposition. Scope: third batch's feature-to-main
assembly only; the remaining three selected v0.6.0 areas, aggregate `assemble`,
version closure, retirement and release are excluded.

Reviewed result: target parent
`4dd10f4bf0bfcea2b4cceede5ac9f24465f6a656` / source head
`f2b4ebb4c0c57e93b1c967f6764672b46905345c`; GitHub PR #5 merged at
`2026-09-19T12:34:17Z` as two-parent merge
`7f84749f8bfa96496602560df0b4d5da09e6fd9d`, result tree
`41cc1390c6a4949c4fbe5cde087925be12f356b7`. Operation class: reviewed
three-way conflict resolution plus repaired source, then feature-to-main merge.
The merge preserves the reviewed main parent and source parent; it is not a
post-hoc reimplementation of the Round 5 candidate.

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 已由用户决定延期

The final Codex review (round 13) found no P0/P1 issue. The user explicitly
instructed: repair review findings only at P0/P1; otherwise merge and record P2
findings for later work. The following in-scope P2 findings are therefore
**CLOSED by explicit user decision** and carried as deferred Lane C candidates:

- `R13-F1` -> `ad-bug-quota-reading-off-route-not-restored-on-save-failure` /
  `docs/roadmap.md` Backlog: reading-off route compensation.
- `R13-F2` -> `ad-bug-quota-statusline-cross-installation-conflict` /
  `docs/roadmap.md` Backlog: cross-state-dir managed-route conflict.
- `R13-F3` -> `ad-bug-quota-parse-failure-source-misattribution` /
  `docs/roadmap.md` Backlog: parse-failure source attribution.
- `R13-F4` -> `ad-bug-widget-large-quota-header-client-label` /
  `docs/roadmap.md` Backlog: large-widget client label.
- `R13-F5` -> `ad-bug-quota-portable-restore-account-bound-state` /
  `docs/roadmap.md` Backlog: portable restore account-bound cache.

### 📝 交付与验证

- GitHub reports PR #5 `MERGED`, `CLEAN`, and `MERGEABLE` before delivery;
  all four final protected checks (`verify` ×2, `desktop` ×2) completed SUCCESS
  on source head `f2b4ebb`.
- The first execution of one `verify` job failed only at
  `TestDesktopQuotaRefreshReturnsDueAlertsForTheAppToDeliver`; that test and its
  production path were unchanged by the docs-only final head. The retried job
  completed SUCCESS. This is retained as an unreproduced transient CI event, not
  misrepresented as a diagnosed or fixed defect.
- Round 5 candidate evidence remains attached to its original
  `f837666` ContentState. The separately recorded merge-result roll-up is
  VERIFIED 2/2 at `v0-6-0-contract:integration:subscription-quota:merge:7f84749f8bfa96496602560df0b4d5da09e6fd9d`;
  prior candidate observations are not relabeled.

Residual uncertainty: P2 carriers are deferred, not technical resolutions. The
aggregate `assemble` task remains open until the other selected areas are
integrated or receive an explicit membership decision.

完成门禁：VERIFIED（2/2；target `v0-6-0-contract:integration:subscription-quota:merge:7f84749f8bfa96496602560df0b4d5da09e6fd9d`）。`integration-readiness` 与 `source-continuity` 都具有同一 merge result 的 PASS evidence；旧候选 evidence 保持其原始 ContentState 不变。

## Round 6 — 2026-09-21 — desktop-refresh

## 📋 desktop-refresh 集成评审

Checklist: 54/54 complete. Incomplete: None.

📊 总体评分：9.5/10

✅ 评审结论：PASS

Reviewer: Codex。Method: `ln-12-delivery-reviewer` Blue-only initial
integration review；项目规则禁止未获请求的委派，因此 independent review panel 为
None，subagent rounds consumed 为 0。Scope: 第四批 `desktop-refresh` 的
direct-to-main fast-forward candidate、与已集成 subscription-quota/App lifecycle
边界的交互、App Group publication/Widget reader、Go wire hint、菜单栏与 Widget
failure/recovery presentation，以及同批 contract/status 同步。其余两个未完成方向、
aggregate `assemble`、push、PR、merge、retirement、version closure 与 release 不在本轮。

Reviewed state: target main
`7f84749f8bfa96496602560df0b4d5da09e6fd9d` / tree
`41cc1390c6a4949c4fbe5cde087925be12f356b7`；source
`cc4151bdcae0689dd025da763b2a3b84eab10d54` / tree
`246ced1b44ae26df83159a548bf37ed9d4bea75d`。Main is the exact merge base and
ancestor, so the operation class is direct fast-forward with no conflict resolution.
Synchronized two-document candidate fingerprint:
`78cae14434c5fae66e5a30366a19b349ea427ec099b876596f9db136c2cb5720`.

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Source Task 5 的 signed commit、五项 commit-bound criteria 与 topic 五项汇总
  criteria 均保持 target-bound PASS；当前 main 是 source 的精确祖先，没有 target-side
  divergence、手写冲突解法或需要重审的中间 result tree。
- `DesktopRefreshCoordinator` 保留 subscription quota 的单一 quota lane 与 full lane
  arbitration；scheduler、publisher、bounded App Group reader、semantic per-kind reload、
  typed Widget load/timeline 和 presentation recovery 沿已评审 owning boundaries 接入，
  没有恢复 legacy `reloadAllTimelines` 或 Widget production `Data(contentsOf:)` 路径。
- stable English/Chinese contracts 与 integration checker 继续区分 automated coverage
  和 installed/native evidence。真实 60–90 秒 cycles、WidgetKit callbacks/intents、
  sleep/wake、VoiceOver、Increase Contrast 与 gallery 仍为 BLOCKED/no-waiver，未被
  integration PASS 改写为技术通过。

### 📝 总结

本轮未发现 change-caused actionable finding。受影响交互按 CodeGraph 定位后直接核对
`AgentDeckApplicationDelegate`、`DesktopRefreshCoordinator`、
`WidgetSnapshotPublisher`、App Group store/reader、Widget timeline 和 Go wire producer；
source continuity 则由精确签名 commit、Task/topic gates 与当前 main 祖先关系支持。

Evidence:

- `git merge-base main HEAD` 返回 `7f84749...`，且
  `git merge-base --is-ancestor main HEAD` 成功；`main...HEAD` 为 85 路径的完整
  desktop-refresh delivery，无 target-only change 或 conflict resolution。
- `git verify-commit HEAD` 报 Good ED25519 signature；commit subject/body 与精确
  `Co-Authored-By: Codex <noreply@openai.com>` trailer 完整。
- `make check-desktop-refresh-integration` 与 `make check-widget-sandbox` PASS。
  Task 5 exact commit 已记录的 App 121（1 skip）、Widget 45/45、Go desktop normal/race、
  topic-docs、whitespace 与 diff checks 在产品、测试、依赖和 toolchain 未变化时复用，
  不因 integration phase 重跑 broad suites。
- CEv1 source Task gate VERIFIED 5/5，desktop-refresh topic gate VERIFIED 5/5；本轮
  integration WorkUnit 的 `integration-readiness` 与 `source-continuity` 绑定同步后的
  exact candidate，missing、invalidated 与 unresolved 均为空。

Residual uncertainty: 真实 native acceptance 行仍 BLOCKED/no-waiver；分支未 push，
remote CI/branch protection、PR 与 merge-result identity 尚未产生。两项均被准确保留为
后续边界，不构成本地 fast-forward candidate 的评审 finding。aggregate `assemble`
仍对 cost transparency 和 health recovery 两个方向保持开放。

完成门禁：VERIFIED（2/2；ContentState
`v0-6-0-contract:integration:desktop-refresh:review-r6:78cae14434c5fae66e5a30366a19b349ea427ec099b876596f9db136c2cb5720`）。

Task checkpoint：ad-v060c-assemble-dev / batch desktop-refresh；content_state=v0-6-0-contract:integration:desktop-refresh:review-r6:78cae14434c5fae66e5a30366a19b349ea427ec099b876596f9db136c2cb5720；gate=VERIFIED；aggregate open。
提交建议：提交本轮 Round 6 评审记录与同批 `docs/status.md`、contract `tasks.md` 同步；source 产品交付已固定在签名提交 `cc4151b`，不得夹带其他任务或产品改动。
推送建议：取得独立推送授权并确认 checkpoint commit 的完整 message、贡献者 trailers、SSH 签名与远端目标后，推送 `feature/desktop-refresh`；PR creation 与 feature-to-main merge 仍需各自授权及成功 CI/branch protection。

## Round 7 — 2026-09-25 — health-recovery

## 📋 health-recovery 第五批集成评审

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- 目标 `origin/main` 是签名源提交的祖先；批次直接沿用已集成的四个主题，没有目标侧产品改动或手工冲突解决。
- 源主题的五项 Task 已交付；修复提交 `6c15672` 的独立 Round 2 评审 PASS，主题新候选门禁 VERIFIED 5/5，旧 `3e7ab8b` 证据没有被改写。
- CLI 的锁分类、schema-ahead 优先级和普通配额锁的 unknown 资源路径保持区分；扩展诊断与显式同步、桌面 wire、Swift 动作校验及菜单栏通知同既有 schema/Hook、刷新和配额消费者兼容。

### 📝 总结

- Reviewed state：目标 `origin/main` `a396c2f2158f579e70da3aaf9bd0bff084fc1f1a` / tree `0372bc4a929a62b7b26e73f6c8307f7704169a53`；源 `feature/health-recovery` 签名 HEAD `6c15672c20ed57a631a7da7e9c7930dc66333fe2` / tree `9af754df84bfac8c376566cf72aa718d985bd4f8`。初始批次候选 ContentState `v0-6-0-contract:integration:health-recovery:candidate:db108ce4c8ebb8eafaec4f67d2689f28dd32ba5bef319a89d0427c117fc93582` 绑定当时的 `docs/status.md` 与 contract `tasks.md` blob；本评审记录和随后的状态同步需要新的最终绑定。
- Reviewer：冷上下文独立集成审阅者，主代理核对结论。Method：Git 祖先与源/目标变更核查、源主题评审和 CEv1 lineage 对账、受影响运行时消费路径与文档契约交互检查；生产代码、测试和配置只读。Scope：第五批健康恢复集成；成本透明及本地未进入远端的 `main@e84ce1f` 不在本批。
- Evidence：`git merge-base --is-ancestor origin/main feature/health-recovery` PASS；远端引用回读仍为目标 `a396c2f` 与源 `6c15672`。源主题最终候选 `urn:ce:agent-deck:content-state:health-recovery:topic:pr-repair-review-final:56386979b685f8654ecbad26a7be65d434d8f457fd3dbfb673631bd46e838e47` 门禁 VERIFIED 5/5；同代码状态的全套 Go 和隔离 macOS XCTest 证据复用。`bash scripts/check-topic-docs.sh`、`make check-whitespace`、`git diff --check` PASS；项目没有专用的 health-recovery 集成检查器。
- Finding disposition：本轮无任务内发现。真实 VoiceOver 顺序仍为 BLOCKED/no-waiver，真实安装客户端观察仍为 SIMULATED；此前桌面刷新和配额的接受限制保留原有归属，不被本批改写。若远端 main 在交付前变化，需重新分类目标与评估交互。
- Completion gate：VERIFIED（2/2）。CEv1 对本轮初始精确候选的 `integration-readiness` 与 `source-continuity` 均找到目标绑定的 pass evidence；后者显式汇总源主题最终五项证据。门禁结论写入记录后须对最终 review/status blob 再绑定；WorkUnit 的最终目标 ID 是权威。Review PASS 和候选门禁不代替提交、PR 或合并授权。

Task checkpoint：`ad-v060c-assemble-dev` / 第五批 `health-recovery`；内容状态为本轮源 `6c15672`、目标 `a396c2f` 及最终 `docs/status.md`、contract `tasks.md`、本评审记录的精确候选；门禁 VERIFIED 2/2，aggregate 仍开放。
提交建议：先按独立范围提交源主题 Round 2 评审记录与 `health-recovery/tasks.md`，再提交本批 `docs/status.md`、contract `tasks.md` 与本记录；两次均排除并行的规则和 Hook 改动，检查贡献者、签名及新提交状态的证据绑定。
推送建议：上述签名提交及各自精确门禁 VERIFIED 后，取得独立推送授权再更新 `origin/feature/health-recovery`；PR 创建与合并仍须分别授权，并在远端 main 变化时重评目标。

### 下一步指令

`提交：v0-6-0-contract / assemble（第五批 health-recovery）`

### Round 7 source-record delivery receipt

The independent source Round 2 record and topic handoff were delivered by
SSH-signed docs-only commit `543d074d9b97d62050a5ad8b36e75c373c33c9ee` /
tree `43d9fb2b7231f52b90001853db4e448bee364d91`. It preserves the reviewed
`6c15672` product and tests; the health-recovery topic gate is VERIFIED 5/5 at
the immutable source-record ContentState. The integration WorkUnit must bind
this updated source and the final batch-document blobs before delivery.

### Round 7 target-baseline impact assessment — PR #8

PR #8 merged the signed `e84ce1f` actor-rule commit into remote main as
`6aafb5a5ae9500896b79aa5ddeee2e420bfa9d13` / tree
`47b7e3f6262845e42803c1130a404cd7c26e3202`. Its parents are the prior
reviewed target `a396c2f` and `e84ce1f`; GitHub reports a valid merge
signature. The source batch checkpoint remains signed `032d867` / tree
`9bc339cd58906bf8b2321cf7e44f5c85be64e353`.

The new target and source share base `a396c2f`. Target-only committed changes
are `.agent-instructions/beads.md` and `project-rules.md`; the source commits
touch neither path, so this is a clean three-way candidate without textual
conflict. The target-side changes add the Antigravity Beads actor and trailer
guidance. They do not change the health CLI, Go/Swift wire, tests, dependencies
or runtime configuration. Round 7 product-interaction findings and native
limitations remain applicable; its earlier fast-forward classification and
target-bound evidence are not reused as facts about the new target. A new
target-specific ContentState and explicit preservation assessment now bind
this candidate; the integration WorkUnit target is authoritative. Refresh
that assessment if remote main changes again before PR delivery.

### PR #9 CI1 repair handoff — 2026-09-25

At signed PR head `e121f6c`, both `verify` jobs passed and both `desktop` jobs
failed only `MenuBarViewModelTests.testHealthRecoveryActionsAreClassifiedAndCopyDoesNotChangeHealth`
at line 729. The assertion searched for `"rm "` anywhere in localized safety
prose; the approved English sentence begins `First confirm ...`, whose
`confirm ` substring satisfies that search. The observed failure is a test
assertion defect, not an executable deletion action in the product. The
architecture requires a manual-prerequisite explanation and no recovery
command for a legacy lock.

The bounded repair asserts the exact localized safety prerequisite for
`prereq_legacy_lock_removal` and that the health row has no executable
`recovery` command. The source model and localization are unchanged. Under
`AGENTDECK_TEST_LOCALE=en`, the isolated `bash scripts/test-macos-app.sh` suite
passed, including the previously failing test; `git diff --check` passed.
An initial direct focused Xcode invocation could not start because the local
sandbox denied CoreSimulator initialization; it supplied no test verdict and
was not treated as a product failure. The two failing CI jobs showed the same
assertion, with no second failing test. This is a repair candidate, not a
new review verdict or a claim that PR-head CI has passed; commit, push and
fresh target-bound CEv1/CI checks remain pending.

### PR #9 Codex review Round 1 repair candidate — 2026-09-25

GitHub Codex review `5317975244` on signed head `a5a35fb` returned six P2
comments. Each was checked against the current source and approved health
contracts before repair:

- **PR9-R1-F1** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770094)) confirmed: the detached scan worker can fail on `scan.lock` before binding a socket, while the foreground discards its stderr and reports a generic startup timeout. `scanruntime.Client` now uses store-owned read-only classification after that timeout; a real helper-process regression asserts typed `scan`/`lock_legacy` contention.
- **PR9-R1-F2** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770103)) confirmed: fingerprint persistence and clearing `extension.sync_incomplete` were separate writes. They now share one `SetSettings` transaction. A trigger-induced marker-clear failure proves the fingerprint rolls back and the old marker survives; a later success updates both.
- **PR9-R1-F3** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770112)) confirmed: adoption stored an empty fingerprint from an unavailable inventory row. Store adoption now rejects that row before management metadata is written; the regression checks the row stays unmanaged.
- **PR9-R1-F4** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770119)) confirmed against the reviewed menu-bar UX table: the native mapper localized only three of six required labels, and other doctor checks also exposed wire names. Current check names now have bilingual labels; unknown future names use localized generic copy. The prototype dictionary, Swift catalog and bilingual tests agree.
- **PR9-R1-F5** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770126)) confirmed: the disclosure's replacement accessibility text omitted a classified row's affected count. It now speaks a localized affected-count phrase for classified reasons. Schema `count` is a version number and deliberately keeps its separate semantics; tests cover both cases.
- **PR9-R1-F6** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4104770135)) confirmed: unreadable-inventory CLI text printed the internal prerequisite key. Text now gives bounded English operator prose; JSON retains the stable key. The corrupted-database CLI test checks both sides.

Classification: all six findings belong to the PR change; none is waived or
carried as technically fixed without code. The local candidate has no production
execution of deletion commands. Verification: focused scan/store/CLI tests PASS;
full Go suite PASS after the final CLI text assertion (`agentdeck-go-test.oIBxfV`); affected-package Go race PASS
(`agentdeck-go-test.uh6Sz7`); `make vet` PASS; isolated macOS XCTest TEST
SUCCEEDED after an initial test-only count-meaning and localization-inventory
repair; prototype `npm run build`, topic docs, whitespace, JSON catalog and diff
checks PASS. Native VoiceOver speech remains BLOCKED/no-waiver and real-client
observation SIMULATED. This was a repair candidate at the verification checkpoint;
it had no new independent re-review verdict, signed commit, remote CI result or
exact-state CEv1 gate at that point.

The six-finding repair was subsequently committed as signed `51ec026a36412c7faeb8a6b373e324c06bab43f9`.
On 2026-09-25 the operator explicitly waived manual VoiceOver acceptance for
this topic. Spoken reading order and live announcements remain NOT TESTED, not
technical PASS; hosted XCTest evidence retains its narrower scope. Real-client
observation remains SIMULATED. PR-head CI, renewed code review and exact-state
CEv1 evidence must be evaluated against the pushed candidate.

### PR #9 Codex review Round 2 repair candidate — 2026-09-25

The first renewed Codex review after green PR-head CI completed on signed
`c11276a572e082b5a803b26fe2ca53cd0bbaef9f` as review `5324844205` and
raised four P2 comments. Each was reproduced against the current source:

- **PR9-R2-F1** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110396191)) confirmed: the old diagnostic sanitizer handled only simple ANSI CSI escapes. OSC, private CSI, C1/DEL and invalid UTF-8 could reach terminal text. The shared sanitizer now strips terminal sequences, replaces invalid UTF-8 and collapses controls into one-line spacing; focused cases cover each class.
- **PR9-R2-F2** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110396192)) confirmed: successful extension watcher scans persisted the fingerprint without clearing the prior incomplete marker. The watcher now uses the same atomic extension fingerprint-and-marker write as manual scans; a trigger-induced rollback test covers failure and retry.
- **PR9-R2-F3** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110396195)) confirmed: `ClassifyLock` could open a FIFO or symlink and used an unbounded read. It now rejects non-regular paths as owner-unknown and bounds regular token reads; tests cover a FIFO, symlink, directory and oversized regular file.
- **PR9-R2-F4** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110396196)) confirmed: three definitions sharing one canonical ID appended that ID twice. The report now lists each duplicate identity once, so `CountForReason` counts affected IDs; a triple-definition regression checks the collection and count.

Final local candidate checks: full Go suite PASS (`agentdeck-go-test.CBCstM`),
four-finding focused race PASS (`agentdeck-go-test.KWIoi8`), `make vet` PASS,
topic-doc and whitespace checks PASS. An earlier overbroad affected-package
race run (`agentdeck-go-test.oZZxUU`) passed `internal/extension` and
`internal/store` but hit Go's 10-minute package timeout in `cmd/agentdeck`;
the test active at timeout had run for two seconds and no data race was
reported. This timeout is retained as a verification limitation, not called
a product failure or an all-package race PASS. This remains a repair candidate,
not an independent re-review verdict. The operator-waived VoiceOver session
remains untested and real-client observation remains SIMULATED. Signed delivery,
PR-head CI and the second renewed Codex review remain pending.

### PR #9 Codex review Round 3 repair candidate — 2026-09-25

The second and final renewed Codex review completed on signed, CI-green
`53244aa8cd1d9271c13f786aaf7555de338678b5` as review `5324976870`. It
raised three P2 comments, each confirmed in source and repaired locally:

- **PR9-R3-F1** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110495824)) confirmed: a watcher fingerprint transaction failure after an inventory commit left a previously empty incomplete marker empty. The watcher now best-effort sets it on failure while preserving the secure-files-after-commit exception; a trigger regression checks the old fingerprint survives and the marker becomes true.
- **PR9-R3-F2** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110495827)) confirmed: discovery failure returned before projecting an existing incomplete marker into the independent JSON flag. The flag is now set immediately after reading the setting; the primary reason remains `extension_discovery_failed` in the regression.
- **PR9-R3-F3** ([discussion](https://github.com/kitdine/agent-deck/pull/9#discussion_r4110495829)) confirmed: marker-only extension-doctor text hid the partial-commit cause and repeated the already-running diagnostic command as `next:`. Text now reports the cause/effect and gives a post-diagnosis step; the CLI regression rejects the circular instruction.

Final local checks: full Go suite PASS (`agentdeck-go-test.Jc7SDI`), focused
race PASS (`agentdeck-go-test.wYAvyO`), `make vet`, topic-doc and whitespace
checks PASS. The operator's two-round renewed-review ceiling is reached. This
is a repair candidate, not a third re-review PASS. VoiceOver remains
user-waived and untested; real-client observation remains SIMULATED. Signed
delivery and fresh PR-head CI remain pending.

### PR #9 post-review CI repair candidate — 2026-09-26

On signed `73f03f8a6a6d7b20c8c4807e7952adb0f1109041`, one of two `verify`
jobs failed during `make verify`'s race suite while the other `verify` job and
both `desktop` jobs passed. The sole failure was
`TestDesktopQuotaRefreshReturnsDueAlertsForTheAppToDeliver`: an unacknowledged
alert's ID differed between two refreshes. A focused `-race -count=10` run
reproduced the failure locally. Its fake provider assigned a new reset instant
as each probe's `observedAt + 3h`, changing the occurrence ID when the two
refreshes crossed a second boundary. The production alert identity correctly
uses the provider's reset instant; the test fixture now keeps one fixed reset
instant across refreshes. The same focused race command passed ten runs after
that correction (`agentdeck-go-test.x54XOi`). This is a test-fixture defect,
not a health-recovery product failure. No third Codex review is requested under
the operator's two-round ceiling. Signed delivery and fresh exact-head CI are
pending.

Local replay of the original `make verify` command passed its self-test and
ordinary full Go suite (`agentdeck-go-test.zT4vcx`). Its full repository race
stage hit the local runner's 10-minute per-package ceiling in `cmd/agentdeck`
and `internal/backup`, with the tests active at timeout running for about one
and three seconds respectively; no data race was reported. This does not
establish a full local race PASS. The focused 10-run race result above is the
repair reproducer, and the new PR-head CI remains the exact original-command
verification gate.

### PR #9 final review and merge receipt — 2026-09-26

The final manual Codex review completed on signed source
`d040bd180466a2b9b3683769ed091718e854d38b` without new findings
([review summary](https://github.com/kitdine/agent-deck/pull/9#issuecomment-5844703363));
there were no P0/P1 blockers or P2/P3 items to defer into Beads. Both PR-head
`verify` and both `desktop` jobs passed. The actual main-side merge commit
`3f29e26c70d7848febc237f4cb130cf7748e7bf5` has parents `6aafb5a` and
`d040bd1`, and tree `4ce5b2b1d73798f130a092e8456b253eae2aee8d` exactly
matches the verified GitHub merge preview. Relative to the source tree, only
main's Beads and project-rules governance files differ; no product overlap or
manual conflict resolution was introduced. GitHub marks the merge signature
valid, and merged-main `verify` and `desktop` CI both passed.

CEv1 source topic gate is VERIFIED 5/5 at the exact `d040bd1` state; the
integration WorkUnit is VERIFIED 2/2 at merge-result ContentState
`v0-6-0-contract:integration:health-recovery:merge-result:3f29e26c70d7848febc237f4cb130cf7748e7bf5`.
Beads records PR delivery on `ad-v060c-assemble-dev`; the two origin bugs and
`ad-health-recovery` planning item are closed. The aggregate `assemble` task
remains open for cost transparency. Manual VoiceOver is user-waived and NOT
TESTED, not a technical PASS; installed real-client observation remains
SIMULATED. Topic retirement and v0.6.0 release are separate boundaries.
