---
status: active
topic: scan-performance
subject: parallel-source-pipeline
---

# parallel-source-pipeline 评审记录

## Round 1 — 2026-10-28

### 📋 并行 source 流水线实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra，独立评审角色，未参与该候选实现。
- Method：当前差异、调用链与测试检查；复用精确状态 CEv1 证据；独立隔离 Go 故障注入。
- Scope：`internal/ingest` 有序流水线、usage/session reduce 与写者、inventory staging、相关测试及性能证据。
- Reviewed state：HEAD `ebafdad6e03913db6fe5ca4991ee0c054905791b`；作用域指纹 `a91bfb14d0dcddd8f43dd64d76f0b4092a31b2f501b6d25e002bffe9bcf6daa0`。
- ContentState：`scan-perf-psp-c1-20261028-state`。
- WorkUnit：`scan-performance:parallel-source-pipeline`；Beads：`ad-scan-performance-parallel-source-pipeline`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### R1-F1 — P1：失败后等待 reduce 退出，与共享 reader 构成循环等待 — OPEN

位置：`internal/ingest/pipeline.go:157-177`；`internal/usage/source_pipeline.go:210-230`。

- 行为风险：一个 source 的打开失败本应返回有限错误，现在可使整个扫描无法自行结束。后续 reconcile 与调用方等待被拖住，直到外部取消。
- 证据：`Ordered` 遇错只设置 `stopped`、释放结果预算，随后 `wait.Wait()`；它既不取消正在进行的 reduce，也不解除失败 consumer 的共享 stream。reader 在 `internal/ingest/ingest.go:860-865` 向失败 source 的无缓冲 channel 发布时阻塞，后续 reduce 等待尚未读取的 source。调用方 `internal/scanruntime/scanruntime.go:1108` 的 deferred `ConsumerDone` 要等扫描返回才执行，因此不能解除此循环。
- 独立复现：四个合成 Claude source，usage reduce workers=4，共享 reader workers=1；首个 source 的 `Service.Open` 等其余三个打开后返回合成错误。未使用的 session consumer 预先 `ConsumerDone`。扫描在 2 秒观察窗口内没有返回；外部取消 context 后才返回原始错误。等待依赖由上述源码确认，不把 2 秒当作产品性能门。
- 现有 `TestOrderedFailureReleasesReservationsForRunningReductions` 只模拟预算等待，不包含共享 reader/channel 的失败 consumer，因此不能排除此问题。
- 💡 修复范围：在等待 worker 退出之前终止失败域的消费等待，安全解除共享 reader 对该域的发送；保留另一域继续工作及已提交前缀语义。增加真实 Coordinator 与失败 source 的隔离回归，证明无需外部取消即可结束。

#### R1-F2 — P2：丢弃归约结果时未释放其文件资源 — OPEN

位置：`internal/ingest/pipeline.go:95-116,164-170`；`internal/usage/source_pipeline.go:211-220`。

- 行为风险：后续已成功归约的 usage source 在前序 source 失败时被丢弃，却没有调用 `preparedSource.close()`；文件描述符不能在任务结束时确定释放。重复失败可能积累描述符，不能依赖 `os.File` 的非确定性 GC/finalizer 回收。
- 证据：`prepareSource` 在 `internal/usage/usage.go:1273-1277` 打开并保存文件；正常关闭在写者 `publish` 的 defer 中。`Ordered` 的已排队结果清理仅累加 Weight，停止后的 worker 仅清零 Weight，Reserve 失败还会直接覆盖整个结果；这些路径均没有结果销毁回调。
- 独立复现：同样四个 source，不启用 Coordinator；用计数的 `SourceFile.Close` 包装器观察资源。首个 Open 注入失败，另外三个成功打开；Scan 返回错误时 `opened=3 closed=0`。这与预算计数归零是两种不同的资源义务。
- 💡 修复范围：明确 reduce 结果所有权，提供恰好一次的销毁路径，覆盖已排队丢弃、停止后完成、Reserve 失败以及写入前失败；增加 Close 计数回归，同时避免正常 publish 与丢弃重复关闭。

### 🟡 建议改进 — 推荐

无。上述两项均为本任务必须修复的缺陷。

### 🟢 优点

- 有序写者保留跨 source 决策顺序，checkpoint 再检查和全量 oracle 对照覆盖了共享 session 与归档场景。
- 性能证据报告了收益和代价：P09 相对 S0 中位耗时比 0.851、相对 B0 为 0.722；P07 最大事务由 416.3 ms 降至 169.9 ms，同时 p95 由 1.305 ms 升至 28.12 ms；RSS 中位比 1.048。
- P08 的 append 读取由 4,184,133 降至 65,064 bytes；P01 重建层中位比 0.548 明确包含 serving-open-path 的联合效果。

### 📝 总结

正常路径已有优化收益，但失败后的终止与资源销毁尚不完整，R1-F1、R1-F2 阻止通过。

#### Evidence

1. 按 ContentState 的既有 recipe 重算指纹：`git diff --name-only HEAD` 与 untracked 路径去重，排除 `docs/`、`apps/macos/.build/`，按字节排序；各行 `path:<git hash-object>`（删除为 `path:deleted`），无末尾换行拼接后 SHA-256，结果与上述指纹一致。
2. 入场使用共享 provider 的原版 `gate-status.cypher`，八项 required criteria 与 `tasks.md` 一致，精确状态初始结果 VERIFIED。复用 `scan-perf-psp-c1-20261028-*` 的全量 Go、race/vet、逻辑等价与性能批次证据，不重跑 unchanged 套件。本轮未重新证明双架构构建等其他 L3 项；出现决定性失败后停止扩大验证。
3. 独立复现不修改仓库代码或测试。临时模块复制当前 `go.mod`/`go.sum`，将 `internal`、`vendor` 指向被审工作区，用 `go run -mod=vendor .` 调用实际 `usage.New(...).Scan(ctx)`。每轮只用临时 HOME 与 state、四个合成 `.claude/projects/p/00.jsonl` 至 `03.jsonl`，每个文件一行：

   ```json
   {"type":"user","sessionId":"s","timestamp":"2026-10-28T00:00:00Z","message":{"role":"user","content":"hello"}}
   ```

   `ReduceWorkers=4`，MachineIdentity 返回固定夹具值；Open 包装器对后三个文件计数并包装 Close，第三个打开时关闭 barrier；`00.jsonl` 等 barrier 后再等 100 ms，返回 `synthetic source open failure`。先直接读，再以 `ingest.Discover`、`NewCoordinator(..., Options{Workers:1})` 共享读，预先 `ConsumerDone(ConsumerSession)`。两轮观察结果：

   ```text
   shared=false returned=synthetic source open failure opened=3 closed=0
   shared=true stalled despite source failure; opened=3 closed=0
   after external cancel: synthetic source open failure
   ```

4. 追加失败证据 `scan-perf-psp-r1-20261028-failure-cleanup`，关联同一 ContentState 与 `bounded-accounting`。节点新增 1、关系新增 2；关系 preflight 全部 ok。重新运行原版 gate，结果 FAILED：新证据 applicable=true、malformed=false。既有通过观察保留历史，不覆盖旧事实。
5. `python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/parallel-source-pipeline.md`、`make check-whitespace`、`git diff --check` 均通过。

#### 下一步指令

修复：scan-performance / parallel-source-pipeline / R1-F1、R1-F2

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-28（Round 1 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围为 R1-F1、R1-F2。Round 1 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- R1-F1 -> repaired in candidate：
  - **复现：** 按评审夹具新增 `TestFailedSourceEndsScanAndClosesDiscardedSources`（usage，四个 Claude source，
    `ReduceWorkers=4`，`00.jsonl` 在其余三个打开后再等 100 ms 返回合成错误；`shared=true` 用 `ingest.Discover`
    与 `NewCoordinator(..., Options{Workers: 1})`，预先 `ConsumerDone(ConsumerSession)`）。修复前的等待结构下
    `shared=true` 在 10 秒内不返回。
  - **生产：** `OrderedOptions.Abort` 在首个失败后、已有前缀 flush 并释放已交付结果之后、`wait.Wait()` 之前调用一次。
    usage 与 session 的 `scanSources` 在有 Coordinator 时把它设为该域的 `ConsumerDone`：reader 的 `publish` 对该域
    改走 `doneCh` 分支，不再阻塞在失败 source 的无缓冲 channel 上；仍在 reduce 的后续 source 读到截断的流后结束，
    其结果按丢弃处理，不写入。另一域的 consumer 不受影响；`ConsumerDone` 幂等，调用方的 deferred 调用不变。
    只在写者到达失败位置后才调用，所以失败前的 source 都已完整写入，已提交前缀语义不变。
  - **回归：** 上述 usage 测试（真实 Coordinator 与失败 source，不做外部取消，10 秒上限）；
    `TestOrderedAbortEndsReductionsWaitingOnFailedInput`（reduce 等待只有 Abort 才会关闭的输入，要求 Abort 恰好一次且
    `Ordered` 自行返回）。临时去掉 Abort 调用时两者都以超时失败；恢复后通过。
- R1-F2 -> repaired in candidate：
  - **复现：** 同一测试的 `shared=false` 子例：修复前 `opened=3 closed=0`。
  - **生产：** `Reduced.Discard` 声明结果所有权：未传给 write 的成功结果恰好调用一次，覆盖失败时已交付在槽中的结果、
    停止后才完成的结果（不再入槽，由 worker 自己销毁并释放预算）、Reserve 失败的结果；传给 write 的结果归 write，
    失败也一样。usage 的 reduce 把 `preparedSource.close` 作为 Discard；`usageWriter.write` 自身 `defer p.close()`，
    覆盖 begin/savepoint 失败等进入 publish 之前的路径（原 publish 中的 defer 移到 write；`close` 置空 file，重算路径
    的交接不变）。session 的 reduce 结果不持有文件，不设 Discard。
  - **回归：** usage 测试两子例都断言 `opened=3 closed=3`（恰好一次，同时排除重复关闭）；
    `TestOrderedDiscardsUnwrittenResultsOnce`（失败时已交付与仍在 reduce 的结果）与
    `TestOrderedDiscardsResultWhoseReservationFails`（预算等待中取消）断言每个成功结果写入或丢弃恰好一次，且预算归零。
    临时让 discard 失效时三例均失败；恢复后通过。

受影响证据处理（影响评估沿用性能证据，交由复评裁定）：

- **内容状态：** 修复改变 `internal/ingest/pipeline.go`、`internal/ingest/pipeline_test.go`、
  `internal/session/source_pipeline.go`、`internal/usage/source_pipeline.go`、`internal/usage/source_pipeline_test.go`
  五个 blob，其余七个条目不变。按既有配方复算 scoped fingerprint
  `343486eaadc4a4a9ba25779ab4143fa032c36939a3224422ae287c28fb2ce120`（HEAD 不变，
  `ebafdad6e03913db6fe5ca4991ee0c054905791b`）；CEv1 ContentState `scan-perf-psp-r1f-20261028-state`。
- **为何沿用性能证据：** 改动只作用于失败路径。成功路径上每个结果多带一个闭包，worker 在已持有的锁内多读一个 bool，
  写入结果的关闭从 publish 的 defer 移到 write 的 defer（每个 source 仍关闭一次）。Abort 与 Discard 只在失败后执行，
  c1 各正式批次 failures、invalid、interrupted 均为 0，二者未被触发。reduce/publication 逻辑、事务分组、预算、
  worker 数、构建输入、语料、S0/B0 基线与环境不变。这是模型评估，交由复评裁定。
- **CEv1：** `change` `scan-perf-psp-r1f-20261028-change` 对八条 `scan-perf-psp-c1-20261028-*` 证据各记
  `may_invalidate`；五项性能证据与 `reduce-equivalence` 的真实语料对照各记一条 `impact_assessment`（outcome
  `preserves`）；五项性能准则在新状态下记 roll-up 证据。`bounded-accounting`、`checkpoint-correctness`、
  `reduce-equivalence` 在新状态下以本次 Go 测试直接记录。原证据与评审的失败证据
  `scan-perf-psp-r1-20261028-failure-cleanup` 保持原状态不改写。

验证：

- `go vet ./internal/...`、`gofmt -l` 无输出；`scripts/run-go-test.sh ./...` 全量通过。
- 新增 ingest 用例与既有失败释放用例 `-count=20` 通过。
- `-race` 覆盖 `internal/ingest`、`internal/usage`、`internal/session`、`internal/scanruntime`、`internal/store`，通过。
- 未重跑性能批次、真实语料模板对照与 Perf App 构建（理由见上）。
- **CEv1 gate：** 对 `scan-performance:parallel-source-pipeline` / `scan-perf-psp-r1f-20261028-state` 逐项查询 requires
  集合（八项）：每项均有绑定新状态的 pass 证据，新状态下无失败证据。使用等价的直接查询，未使用共享 provider 的
  `gate-status.cypher`。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / parallel-source-pipeline

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 2 — 2026-10-28

### 📋 并行 source 流水线缺陷复评

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewer：Codex 主会话，独立复评角色；未参与实现或修复，未委派。
- Method：按既有缺陷逐项核对源码与精确状态证据，CodeGraph 定位调用链；用 Go overlay 注入独立、确定性故障夹具，不修改仓库生产代码或测试。
- Scope：R1-F1、R1-F2 修复及直接受影响的 late-result 清理路径。
- Reviewed state：HEAD `ebafdad6e03913db6fe5ca4991ee0c054905791b`；作用域指纹 `343486eaadc4a4a9ba25779ab4143fa032c36939a3224422ae287c28fb2ce120`，按 Round 1 recipe 重算匹配。
- ContentState：`scan-perf-psp-r1f-20261028-state`。
- WorkUnit：`scan-performance:parallel-source-pipeline`；Beads：`ad-scan-performance-parallel-source-pipeline`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### R2-F1 — P1：无预算模式下晚完成结果的清理触发空指针崩溃 — NEW / OPEN

位置：`internal/ingest/pipeline.go:130-135`，具体崩溃行 134。

- 行为风险：`OrderedOptions.Budget` 是可选值；usage/session 仅在有 Coordinator 时设置它，但归约结果始终可以有正 Weight。前序 source 失败、另一 worker 在 `stopped=true` 后完成时，late 分支只检查 `result.Weight > 0` 就调用 `options.Budget.Release`。无 Coordinator 的并行扫描可能因此由原本可返回的 source 错误升级为进程级 panic。
- 证据：独立夹具令两个 worker 同时启动，source 0 等 source 1 已开始后返回错误；source 1 等 Abort 被调用才返回 `Weight: 1`，Budget 保持 nil。Go 1.27.3 下进程退出 1，栈为 `Ordered[...].func2()` → `internal/ingest/pipeline.go:134`，报 `panic: runtime error: invalid memory address or nil pointer dereference`。无需 sleep 或竞争窗口即可强制进入目标分支。
- 既有测试缺口：`TestOrderedDiscardsUnwrittenResultsOnce` 的无预算结果 Weight 为 0；usage 的 direct 子例先等其他文件打开，再额外等待 100 ms，使后续结果有机会在失败前全部入槽。这些通过结果未覆盖无预算且正权重的 late-result 分支。
- 💡 修复范围：只有实际启用预算时才释放相应 reservation；无预算仍须恰好一次销毁结果。补充 nil Budget、正 Weight、失败后才完成的确定性回归，断言原始错误正常返回且 Discard 恰好一次；保留有预算释放归零的回归。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R1-F1 -> CLOSED：`Ordered` 在失败结果清理后、`wait.Wait()` 前调用 Abort；usage/session 分别接到自身 ConsumerDone。ConsumerDone 关闭该域 doneCh，只有最后一个域退出才取消 Coordinator，消除了原记录中的失败域 reader 循环等待。复用当前状态 `scan-perf-psp-r1f-20261028-bounded-accounting` 的真实 Coordinator 回归结果；源码与该测试的失败注入和外部取消边界一致。
- R1-F2 -> CLOSED：成功归约结果通过 Discard 明确销毁责任，入槽丢弃、late-result 和 Reserve 失败均调用它；传给 write 后由 `defer p.close()` 接管，覆盖 begin/savepoint 失败，close 将 file 置 nil。复用同一当前状态证据中的 opened=3/closed=3 与所有权回归。R2-F1 是新清理分支的预算调用缺陷，不把原缺少销毁回调的发现继续记为未修复。

### 📝 总结

两项原发现已关闭；新增 R2-F1 使失败处理仍不安全，因此本轮 FAIL。只更新复评、任务状态与证据；未修复产品，未提交或推送。

#### Evidence

1. 使用共享 provider 原版 `gate-status.cypher`，参数 namespace=`github.com/kitdine/agent-deck`、work_unit_id=`scan-performance:parallel-source-pipeline`、target_content_state=`scan-perf-psp-r1f-20261028-state`。八项 required criteria 与任务准则一致。入场结果 VERIFIED；两个旧 c1 观察仍列为 unresolved，但不在新状态所用证据的 lineage 中，不将其误报为当前八项证据无效。
2. 独立夹具通过 overlay 映射到包内一个新增的虚拟 `_test.go` 文件，仓库文件保持只读。夹具核心如下（包名 ingest，导入 context、errors、testing）：

   ```go
   func TestR2LateWeightedResultWithoutBudget(t *testing.T) {
       started, resume := make(chan struct{}), make(chan struct{})
       failure := errors.New("synthetic first-source failure")
       err := Ordered(context.Background(), 2,
           OrderedOptions{Workers: 2, Abort: func() { close(resume) }},
           func(i int) Reduced[int] {
               if i == 0 {
                   <-started
                   return Reduced[int]{Err: failure}
               }
               close(started)
               <-resume
               return Reduced[int]{Value: i, Weight: 1, Discard: func() {}}
           },
           func(int, int, bool) (bool, error) {
               t.Error("unexpected write")
               return true, nil
           },
           func() error { return nil })
       if !errors.Is(err, failure) {
           t.Fatalf("got %v", err)
       }
   }
   ```

   命令：以 `GO_TEST_BIN=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go` 调用
   `scripts/run-go-test.sh -overlay=<新建夹具映射 JSON> ./internal/ingest -run '^TestR2LateWeightedResultWithoutBudget$' -timeout 30s`。
   首次默认 PATH 的 Go 1.27.1 与 GOROOT 1.27.3 不匹配，只有编译失败，不作为产品证据；指定一致的 Go 1.27.3 后得到上述 panic。
   输出虽先显示测试函数 PASS，worker 的未恢复 panic 随后使整个测试进程退出 1，不能取局部 PASS 作为通过。
3. 追加 `scan-perf-psp-r2-20261028-nil-budget-failure` 到当前状态及 bounded-accounting；保留原通过观察。新增节点 1、关系 2，关系 preflight 均 ok；标准 gate 回读确认该失败证据 applicable=true、malformed=false、unresolved=false，最终 FAILED。
4. 复用原候选 Go/race/vet 等精确状态证据；发现决定性崩溃后停止扩大验证。性能数据仍是 c1 实测经修复者影响评估沿用，本轮未重新测量，也未将其描述为修复候选的新实测。性能复用论证及完整 L3 交付就绪不由本轮失败结论背书。
5. `check-review-records.py`、`check-docs.py`、`make check-whitespace` 与 `git diff --check` 均通过。夹具与测试日志为临时诊断，必要代码、命令、内容身份与失败栈已在本记录中保留，清理后不依赖临时文件。

#### 下一步指令

修复：scan-performance / parallel-source-pipeline / R2-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-28（Round 2 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围为 R2-F1。Round 2 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- R2-F1 -> repaired in candidate：
  - **复现：** 新增 `TestOrderedDiscardsLateWeightedResultWithoutBudget`，即评审夹具加 Discard 计数：两个 worker，
    source 0 等 source 1 开始后失败，source 1 等 Abort 后返回 `Weight: 1`，Budget 为 nil。去掉修复时进程在
    `internal/ingest/pipeline.go:134` 报 nil pointer dereference 并退出，与评审栈一致。
  - **生产：** late 分支改为 `options.Budget != nil && result.Weight > 0` 时才释放；无论是否有预算，结果都先经
    `discard` 恰好销毁一次。已交付结果的批量释放与 Reserve 路径原本就判断了 Budget，不变。
  - **回归：** 上述测试断言原始错误正常返回、Discard 恰好一次、结果未写入；有预算的释放归零仍由
    `TestOrderedFailureReleasesReservationsForRunningReductions` 与 `TestOrderedDiscardsResultWhoseReservationFails` 覆盖。
  - **附带测试修正（验证阻塞，非新发现的产品缺陷）：** Round 1 修复时新增的
    `TestOrderedDiscardsResultWhoseReservationFails` 在 `-count=20` 中出现一次 `discarded 2 results, want 3`。
    原因是预算为空时任何 Reserve 都会放行：index 1 偶尔先于 index 0 预留，随后作为正常结果写入。"写入或丢弃恰好一次"
    这条不变式在该次失败中仍成立。改为 index 1–3 等 index 0 进入写者后才 reduce，写者取消后等到三个 Reserve 失败
    再返回，不再依赖 sleep；`TestOrdered*` 连续 `-count=200` 通过。

受影响证据处理（影响评估沿用性能证据，交由复评裁定）：

- **内容状态：** 修复只改变 `internal/ingest/pipeline.go` 与 `internal/ingest/pipeline_test.go` 两个 blob，其余十个条目
  不变。按既有配方复算 scoped fingerprint `b8c9947e12a9edad39d453a1f804cd70acf9610567a85263753e1558510c0de2`
  （HEAD 不变，`ebafdad6e03913db6fe5ca4991ee0c054905791b`）；CEv1 ContentState `scan-perf-psp-r2f-20261028-state`。
- **为何沿用性能证据：** 唯一的产品改动是失败后 late 分支上的一个 nil 判断。c1 各正式批次 failures、invalid、
  interrupted 均为 0，且实测扫描都有 Coordinator，Budget 非 nil，行为与之前相同。测试文件不进入产物。
- **CEv1：** `change` `scan-perf-psp-r2f-20261028-change` 对八条 `scan-perf-psp-r1f-20261028-*` 证据各记
  `may_invalidate`；五项性能准则各记一条 `impact_assessment`（outcome `preserves`），并在新状态下记 roll-up 证据。
  `bounded-accounting`、`checkpoint-correctness`、`reduce-equivalence` 以本次 Go 测试直接记录；`reduce-equivalence`
  的真实语料对照仍沿用 c1 实测（经 Round 1 修复时的影响评估）。评审失败证据 `scan-perf-psp-r2-20261028-nil-budget-failure`
  保持原状态不改写。

验证：

- `go vet ./internal/...`、`gofmt -l` 无输出；`scripts/run-go-test.sh ./...` 全量通过。
- `TestOrdered*` 连续 `-count=200` 通过。
- `-race` 覆盖 `internal/ingest`、`internal/usage`、`internal/session`、`internal/scanruntime`、`internal/store`，通过。
- 未重跑性能批次、真实语料模板对照与 Perf App 构建（理由见上）。
- **CEv1 gate：** 对 `scan-performance:parallel-source-pipeline` / `scan-perf-psp-r2f-20261028-state` 逐项查询 requires
  集合（八项）：每项均有绑定新状态的 pass 证据，新状态下无失败证据。使用等价的直接查询，未使用共享 provider 的
  `gate-status.cypher`。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / parallel-source-pipeline

WORKFLOW_WORKSPACE: agent-deck.scan-performance
## Round 3 — 2026-10-28

### 📋 并行 source 流水线缺陷复评

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer：Codex 主会话，独立复评角色；未参与实现或修复，未委派。
- Method：逐项处置已有发现；精确 blob 对照、源码与回归断言检查、标准 CEv1 gate 及 lineage 核验；复用有效测试证据，补充缺少的 macOS 双架构构建证据。
- Scope：R2-F1 修复及随附 reservation-failure 测试时序修正，确认 R1-F1、R1-F2 未回归。
- Reviewed state：HEAD `ebafdad6e03913db6fe5ca4991ee0c054905791b`；作用域指纹 `b8c9947e12a9edad39d453a1f804cd70acf9610567a85263753e1558510c0de2`，按既有 recipe 重算匹配。
- ContentState：`scan-perf-psp-r2f-20261028-state`。
- WorkUnit：`scan-performance:parallel-source-pipeline`；Beads：`ad-scan-performance-parallel-source-pipeline`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R2-F1 -> CLOSED：`internal/ingest/pipeline.go:133` 在 late 分支释放前检查 `options.Budget != nil`，Discard 保持无条件调用。将这一处 guard 从当前文件反向归一化后，blob 精确恢复为上轮 `533cd509ce7e543deae70a3192bdafa064178230`，证明生产改动仅限该判断。无预算结果不再调用 nil 接口，有预算释放行为不变。
- `TestOrderedDiscardsLateWeightedResultWithoutBudget` 沿用 Round 2 的确定性因果链：source 0 等 source 1 启动后失败，source 1 等 Abort 才返回正 Weight，断言原错误、Discard=1、未写入。当前状态证据记录去掉 guard 可复现 panic，恢复后通过；无需重复同内容测试。
- R1-F1 -> CLOSED（保持）：Abort 在等待 worker 前解除本域 ConsumerDone 的阻塞；本轮唯一 guard 不改变这一控制流，usage/session 源码 blob 与上轮一致。
- R1-F2 -> CLOSED（保持）：成功结果的 Discard/write 所有权与 `p.close()` 不变；新增 guard 仅限制预算释放，不绕过资源关闭。
- reservation-failure 测试改用 headWriting channel，确保首项已持有预算后其他归约才进入 Reserve；取消后等待三项丢弃，避免依赖原先固定 sleep 的调度假设。仍断言每项写入或销毁恰好一次及预算归零，没有降低原不变式。

### 📝 总结

R1-F1、R1-F2、R2-F1 全部关闭，无新增未解决发现。本轮 PASS 绑定上述候选；保留 Round 1、Round 2 的历史失败及失败证据。测试、门禁通过与授权提交/推送仍是不同边界。

#### Evidence

1. 十二个候选路径逐项 hash 与 r2f ContentState 一致；相对 r1f 仅 pipeline.go、pipeline_test.go 两项不同。生产文件归一化对照证明改动是单一 nil guard。其余十个 blob 未变，可复用前轮对应源码核验。
2. 复用 `scan-perf-psp-r2f-20261028-bounded-accounting`、`checkpoint-correctness`、`reduce-equivalence` 中当前状态的全量 Go、相关五包 race、vet、`TestOrdered* -count=200` 及故障回归证据；不因复评轮次变化重跑这些检查。
3. 入场标准 `gate-status.cypher` 返回 NOT_VERIFIED，唯一 missing criterion 为 reduce-equivalence：r2f 证据依赖 r1f 观察，而 r2f change 对该观察的 may_invalidate 没有处置。直接检查八条 pass 不能替代 lineage 门禁。
4. 本轮追加 `scan-perf-psp-r3-20261028-impact-reduce-equivalence`，对 `scan-perf-psp-r2f-20261028-change` → `scan-perf-psp-r1f-20261028-reduce-equivalence` 记录 preserves：唯一产品变化只影响失败后无预算结果的释放，不改变成功归约或 oracle 行语义；有 Coordinator 时行为相同。新增节点 1、preserves 关系 1，preflight 为 ok，写入计数与预期一致。未覆盖旧观察，也未将旧实测重标为新实测。
5. 再次使用原版 gate，参数 namespace=`github.com/kitdine/agent-deck`、work_unit_id=`scan-performance:parallel-source-pipeline`、target_content_state=`scan-perf-psp-r2f-20261028-state`：八项 required 均有 applicable、非 malformed、无 unresolved 的当前证据，missing_criteria 为空，结果 VERIFIED。返回列表中四条旧 bounded-accounting/checkpoint-correctness 观察仍 unresolved，但不是当前通过观察的依赖，不影响当前门禁。R1/R2 失败观察保留在各自旧状态。
6. 性能数据复用 c1 实测及已记录的两次影响评估。R2 修复 guard 不进入成功路径；R1 的 Abort/Discard 不在无失败批次触发，其闭包/锁内分支及 close defer 移位仍有实现开销，故这里接受的是有明确 lineage 的复用判断，不声称重新测得 r2f 的相同比值。此前“build inputs 不变”的文字不应理解为生产源码或二进制相同；新 ContentState 已明确记录变化。本轮未重跑性能批次或真实语料模板对照。
7. 补足当前候选的 macOS 双架构 CLI 构建：Go 1.27.3，`make build-all GO=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go DIST_DIR=<本轮临时构建目录>`，退出 0。file 确认 arm64 与 x86_64 Mach-O；arm64 为 14,671,746 bytes，低于 Makefile 的 26,214,400 bytes 上限；amd64 为 15,771,440 bytes。SHA-256 分别为 `49c66b37b05ab5bd7e6d2137b518412896ff7b31d46db63c1e49de6eb2daf8b8`、`33b9c9f9bd72cede824e4f9fa649ab74911c4cc8895fcd365ebeba147a974b21`。这是构建验证，非 App 安装、原生运行或发布验收；临时二进制清理后由本节保留必要事实。
8. 仅同步本 topic 的任务矩阵和本记录，不修改全局 docs/status.md。该内容身份排除 docs，状态同步不改变上述产品指纹。后续任务仍未完成，未跨越 Topic 完成边界；CEv1 当前也未返回本 WorkUnit 的 contains 父关系，不据此制造 Topic VERIFIED。

#### Task checkpoint

Task checkpoint：`ad-scan-performance-parallel-source-pipeline`；content_state=`scan-perf-psp-r2f-20261028-state`；fingerprint=`b8c9947e12a9edad39d453a1f804cd70acf9610567a85263753e1558510c0de2`；gate=VERIFIED。

提交建议：获授权后提交本 Task 的十二个产品/测试路径、tasks.md 对应状态及本评审记录；逐项核对暂存范围与贡献者，按项目要求验证提交正文、trailer、SSH 签名及提交态证据绑定。

推送建议：获独立授权、完成提交对象核验后推送 `origin/feature/scan-performance`；origin 为 `https://github.com/kitdine/agent-deck.git`，实时远端分歧和保护规则留到授权推送门检查。本轮未提交或推送。

下一任务为 incremental-update；其 Beads 仍依赖 parallel-source-pipeline 与 serving-open-path 的交付关闭，当前不声明已就绪，不代办另一任务的交付或状态变更。

#### 下一步指令

开发：scan-performance / incremental-update

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## PR #61 修复交接 — 2026-10-31

来源：Codex 对 `3e07c892f34d775e60919774f49ae7c85afb755f` 的
[过期 inventory 回收发现](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431001380)。
已确认成功扫描只移动 coverage head，旧 checkpoint 与 inventory 持续累积。

`FinishInventory` 完成后按每事务最多 128 个 checkpoint 回收过期终态 inventory，
保留当前 head、telemetry 结算所需的最近完整 inventory，以及其他仍在 staging 的 pending
inventory。核心与会话数据库共用相同规则；不改变所有权判断或扫描覆盖范围。
`TestCompletedInventoriesReclaimCheckpointsAndRetainSettlementEvidence` 在两个数据库分别
验证五轮、每轮 257 个来源的回收，以及 pending/paused head 下结算证据和在途 staging
的保留，定向回归通过。

新候选等待 PR Code Review；此前各轮 PASS 与精确门禁只对应其原内容身份。
