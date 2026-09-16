---
status: active
topic: snapshot-performance
subject: shared-ingestion
---

# Shared Ingestion Review

## Round 1 — 2026-09-10

## 📋 共享摄取实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SI-R1-F1 — 高：未变化文件缺少 Skip 通知，形成预算与消费者相互等待。**

位置：`internal/usage/usage.go:997` 的新 Skip 通知只覆盖
`scanFileMode` 内部的 stableMetadata 分支；实际 `scanInventory` 在
`:710` 起的 unchanged 分支直接 continue，根本不会进入该方法。
关联位置：`internal/usage/usage.go:1085`；
`internal/ingest/ingest.go:299`、`:316` 和 `publish`。

- 行为风险：已索引、未变化的前置文件仍被 coordinator 调度。其首个 batch
  等待 usage 接收，而 usage 已跳过它并在读取后面的变化文件。前置文件达到
  默认 64 MiB admission budget（文件大小按 64 倍计费，约 1 MiB 即可）时，
  后续文件无法调度，预算也无法释放。session 的 Skip 不能替代 usage 的 Skip，
  ConsumerDone 又要等 usage 结束，因而形成等待环。正常增量刷新可能一直等待，
  有 deadline 的调用则失败。这不是仅存在于尚未实现 worker 模式的问题。
- 证据：使用仓库既有合成 corpus 和 `scanSnapshotPerformanceDomains`，
  给排序在前的 Claude 文件追加 1 MiB JSON 前导空白并先经 legacy 完成索引，
  然后只追加后置 Codex 文件。默认 coordinator 参数下，legacy 增量耗时
  `51.445676ms` 且成功；shared 增量耗时 `3.002904497s`，返回
  `context deadline exceeded`。该差异与上述源码等待环一致。
- 测试保护缺口：现有 mutation differential 使用小文件，无法迫使前置文件
  独占 admission budget；已取消 context 用例也不能证明这一在途等待场景。

💡 修复范围：在 usage 的真实 inventory skip 路径释放该 consumer 对源的
参与义务，审计同类早退/跳过分支；保证被两域跳过的源释放调度资源。加入
“前置未变化源占满预算、后置源变化”的回归保护，验证两域完成、取消收尾和
逻辑数据等价。不要通过增大预算、延长 deadline 或恢复被拒绝实验来掩盖问题。

Disposition：OPEN，交回同一任务 `ad-sp-shared-ingestion` 修复。

### 🟡 改进建议 — 建议处理

无。本轮停止扩大检查范围，不把尚未核实的疑点记录为 finding。

### 🟢 做得好的方面

- 共享批次保留 offset/sequence，域事务和结果边界仍分开。
- 保留 legacy 对照路径及 mutation differential，可构造隔离语料验证差异。
- 拒绝过的 batch-credit 候选没有恢复；性能目标未被测量结果无声降低。

### 📝 总结

Reviewer：Codex 主代理。

Method：单代理正式代码评审，源码路径核对与隔离 Go overlay 反例。
复用本会话已确认的 CodeGraph 跨 worktree 限制，直接核实 topic 当前源码；
没有创建索引或使用子代理。

Scope：Task 2 retained shared-ingestion 候选及其优化；实查 coordinator、
usage/session 消费与 skip 路径、desktop 调用和现有差分测试。在高严重度
反例成立后停止广泛验证，尚未完成其他 SQL/聚合优化的全部评审。
W0-W7 worker 后续实现不在本轮，补充设计的文档 PASS 不替代本实现评审。

Reviewed state：workspace `agent-deck.snapshot-performance`，
branch `feature/snapshot-performance`；HEAD
`446a58f1f6716f257680879e5dbf3b61365c8cb2`。入口候选 manifest SHA-256：
`a7a4f57b22bf66367710c9068efa6224a63104b0dc1d01735b9feac18237ed87`。
与历史 `6e4148c8…` 的 24 路径有序 manifest 比较，23 个代码/测试 blob
完全相同，仅 tasks.md 因补充文档评审登记变化。关键生产 blob：
ingest `490fa310fc31ad3d18adabed3b680e53404c05e5`；
usage `9290ed31015b0c01bee8d01732b8897d025bf498`。

Evidence：

- 历史状态的固定 CEv1 gate 查询返回 VERIFIED 5/5；它记录历史验证，
  不是本轮 PASS。新反例揭示先前小语料未覆盖的增量调度缺陷。
- 隔离 probe 通过 Go `-overlay` 注入虚拟测试文件，未修改仓库代码或测试。
  命令：`GOCACHE=/private/tmp/agent-deck-go-build
  AGENTDECK_GO_TEST_LOG=/private/tmp/agentdeck-shared-review-r1/probe.log
  scripts/run-go-test.sh -overlay /private/tmp/agentdeck-shared-review-r1/overlay.json
  ./cmd/agentdeck -run '^TestReviewR1UnchangedHeadBlocksChangedTail$' -timeout 30s`。
- 结果：exit 1；legacy 子用例 PASS，shared 子用例 FAIL。测试源码、overlay、
  日志和 manifest 保留在 `/private/tmp/agentdeck-shared-review-r1/` 供修复复现；
  不含用户生产数据，未加入 Git。
- 未重跑全仓、race、vet、build 或代表性性能 benchmark；有决定性阻塞反例后，
  更广验证应在修复达到最终相关内容状态后运行。

完成门禁：FAILED。最终 topic 状态同步后的 ContentState 为
`urn:ce:agent-deck:state:workspace:0f5e659e7912aaa7761cef76017b7a174fc5a74f5175877337671b84f0ab5885`。
沿用相同 24 路径 manifest 公式；与反例执行状态相比仅 tasks.md 交接文本变化，
23 个代码/测试 blob 未变化，因此反例适用于此状态。固定 CEv1 gate 回查确认
`l3-verification-clean` 有适用失败证据；其余历史检查未批量改写或伪造新状态 PASS。
新增 2 个节点及 2 条关系，关系预检均为 ok；回查确认失败证据适用。

本轮评审/状态记录 L0 检查通过：topic-docs、whitespace、diff-check。
复现日志 SHA-256：
`5a6df40fbf72535d1f20c048cf1f4403bcab99a73690e6fe89d9c1a68aefbe51`；
probe SHA-256：
`86f6b26ebdd9da644c73b29c5107305c99223acd8012b5aa8429f70e19492bd2`。

Task 状态：Dev 保留已完成历史，Review 未勾选；同一 Beads 任务返回修复。
文档提交所依赖的实现仍未满足交付条件；本轮没有 commit 或 push。

下一步指令：修复：snapshot-performance / reviews/shared-ingestion.md / SI-R1-F1

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 2 — 2026-09-10

## 📋 共享摄取修复复评

📊 总体评分：6/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**SI-R2-F1 — 高；新增：取消调度后未关闭后续源的通道，消费者可能永久等待。**

位置：`internal/ingest/ingest.go:303`、`:309` 的取消分支仅 finish 当前源
然后 return；剩余源的 Batches 和 finished 都没有终结。
关联消费者：`internal/usage/usage.go:1088`、`internal/session/session.go:774`
使用 channel range，只有 channel 关闭后才执行带 context 的 Wait。

- 行为风险：前置源占住预算，某域已跳过前置源并等待后面的源时，如果请求被
  取消或 deadline 到期，schedule 在中间源退出，后续源永远不会被派发。
  等待后续 Batches 的消费者无法感知 context 取消，desktop 等待两域结果
  也可能无法返回。即使 coordinator.Close 已结束，公开流仍不是终态。
- 证据：三个稳定源、一个 reader、Budget=1；Open 屏障将第一个源保持在途，
  第二个源处于 admission 等待。取消 context，确认第二个源 finished 后释放
  第一个源并等待 Close 返回。第三个源的 finished、usage/session 两个通道
  全部仍打开。该测试使用 channel 屏障确定顺序，不依靠睡眠触发竞态。
  `TestReviewR2CancellationClosesUnscheduledStreams` 确定性失败，exit 1。
- 测试保护缺口：新 admission 回归的 cancellation 段先 cancel 再调用扫描，
  数据库入口即可返回，因此不能证明已有流和调度队列的取消收尾。

💡 修复范围：让所有已登记源在调度取消时进入且仅进入一次终态，释放对应资源；
协调在途 reader 与未调度源的关闭责任，避免重复 close。消费者等待批次也应
具有明确的取消退出契约。添加上述在途/未调度混合场景及两域退出的回归保护，
保持正常完成、skip 和域独立失败语义。不要只延长 deadline。

Disposition：OPEN；同一任务 `ad-sp-shared-ingestion` 返回修复。

### 🟡 改进建议 — 建议处理

无。阻塞反例成立后停止扩大验证。

### 🟢 做得好的方面

**SI-R1-F1 → CLOSED。** 当前 usage.go 的真实 inventory unchanged 分支已调用
Coordinator.Skip(entry.Path, ConsumerUsage)，堵塞原因已消除。新增
`TestSharedIngestionReleasesBudgetForChangedSourceAfterUnchangedSource` 使用
1 MiB 前置源与后置增量，检查两域完成和 legacy/shared 逻辑行一致；保留原预算
与 deadline，没有恢复被拒绝的 batch-credit 实验。

### 📝 总结

Reviewer：Codex 主代理；Method：单代理源码复评及隔离 Go overlay 反例。
Scope：SI-R1-F1 修复及相关流生命周期；继续读取 SQL prepared statements 和
SignalsBatch，但新阻塞成立后停止，其他剩余 SQL/聚合路径未完成全面评审。
本轮无生产代码、仓库测试或配置修改，无子代理，无 Git 交付。

Reviewed state：`agent-deck.snapshot-performance` / `feature/snapshot-performance`；
HEAD `446a58f1f6716f257680879e5dbf3b61365c8cb2`，入口 24 路径 manifest
`278e9ebf7322382a5e7866ad3df2513fded05d311708522267be00560547cf90`，
与修复进程交接内容精确相同。usage blob
`25d5f231732f42571f5a68813a7d96a4c65ca0c2`，ingest blob 仍为
`490fa310fc31ad3d18adabed3b680e53404c05e5`。

Evidence：固定 CEv1 gate 在修复入口状态返回 VERIFIED 5/5；复用其历史测试
证据并核对修复及回归源码，没有重复运行已通过的完整套件。新增反例是此前
未覆盖的在途取消场景，不把已有套件通过当成此场景通过。

复现命令：

```sh
GOCACHE=/private/tmp/agent-deck-go-build \
AGENTDECK_GO_TEST_LOG=/private/tmp/agentdeck-shared-review-r1/cancel-probe.log \
scripts/run-go-test.sh \
  -overlay /private/tmp/agentdeck-shared-review-r1/cancel-overlay.json \
  ./internal/ingest -run '^TestReviewR2CancellationClosesUnscheduledStreams$' -timeout 15s
```

源码 `cancel_probe_test.go`、overlay 与日志保留在上述私有诊断目录供修复；
仅使用合成临时文件，未接触用户数据。失败时 coordinator 已退出，三个通道
仍未关闭，不将慢编译或测试总耗时误报为死锁依据。

完成门禁：FAILED。最终同步态为
`urn:ce:agent-deck:state:workspace:ead891d7b7963afb609f0c741576c2a21223775016a72d8175884ab94d090c73`；
相同 24 路径 manifest 公式，仅 tasks.md 交接文本改变，生产/测试 blob 不变。
固定 CEv1 gate 回查确认新增失败证据适用；2 节点、2 关系写入与关系预检完成。
本轮记录的 topic-docs、whitespace、diff-check 均通过。
日志 SHA-256：`5694dbd7f3c363b048bcad40b4adf19d95009bd5f3e2ba1f79e1b2a924792cd4`；
probe SHA-256：`92007689b6cded01f0c448fc3fbd4fe934074f7f45d277e78af41e330782d9cc`。

下一步指令：修复：snapshot-performance / reviews/shared-ingestion.md / SI-R2-F1

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Replanning disposition — 2026-09-10

The user requested a complete topic replan and permitted topic-code replacement.
See [whole-topic review and replacement plan](tasks.md#round-2--2026-09-10).
The rounds above remain historical facts for their own content. They do not
approve the revised document set, authorize delivery, or prescribe the current
next command. The current Tasks matrix is the only execution plan.

The old candidate delivery/repair route is withdrawn. SI-R1-F1 remains closed
for its actual repaired candidate; SI-R2-F1 remains unremedied in the old code.
Both regression scenarios are mandatory acceptance for unified-scan-runtime;
this reassignment does not mark the old implementation PASS or complete.
