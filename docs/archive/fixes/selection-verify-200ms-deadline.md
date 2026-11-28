---
status: historical
retired: 2026-11-27
created: 2026-11-17
---

# 缺陷：高负载下健康 generation 被 200ms 内部期限误判不可用

载体 `ad-bug-selection-verify-200ms-deadline`，v0.7.5，Lane A。工作区 `agent-deck.fix.selection-verify-200ms-deadline`。前两轮发现经有界修复，第 3 轮冷复评 PASS。

## 现象

旧诊断在高 CPU 压力下观察 session scope/classify 返回 generation_unavailable；定向轻载可通过。已有隔离诊断将底层错误定位为 context deadline exceeded；历史压力数字不作为本轮已执行证据。

## 根因

来源指出 OpenServingGenerationReader 的固定 200ms 子期限及 Select 将其当不可用。当前源码与控制延迟回归由 Developer 先核对。

## 修复边界

只修内部过短预算导致的误判，保留调用者取消、有界等待、向量一致性/移动验证、实际损坏和不可用错误；不增加公开设置或削弱 state ownership。若需要新公开输出/优先级契约先报告，不能借修复扩大范围。

## 验证

HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。验证覆盖大于 200ms 的受控等待、取消与不可用对照、受影响包的常规及 race 测试、入口盘点和所有权拒绝。最终状态与验收结果见第 3 轮。

## 实现

`sessionorigin.Reader.Select` 的默认验证（`Verify == nil`）改用新增的 `store.VerifyServingGeneration`：预算为普通只读读取的 busy timeout（`readOnlyBusyTimeout`，5s），且不超过调用方期限；锁等待仍用 identity profile 的短 busy timeout，缺失、不安全、被替换、损坏或更新版本的状态与 `ReadServingGeneration` 同样失败。`ReadServingGeneration` / `OpenServingGenerationReader` 保持 200ms 身份预算（`servingIdentityBudget`），App 启动恢复与 serving 发布不受影响。新增 `internal/sessionorigin/selection_verify_latency_test.go` 与 `internal/store/serving_generation_test.go` 中的延迟、取消用例。原 Developer 报告位于已失效的协调线程，不作证据；以下为主控直接观察。

| 路径 | Blob |
| --- | --- |
| internal/sessionorigin/selection.go | 6771d4789911f6d271db6db92f3cb7382f379ca7 |
| internal/store/read_pool.go | 3ce64ac9f67c958f11f5a316621b7bcbbb41002d |
| internal/store/serving_generation.go | d28d443cac3fd193ee56f98e1c48a7d40b9298cd |
| internal/store/serving_generation_test.go | 6def286cc1597b8942d46f91624d0cffa0454fdf |
| internal/sessionorigin/selection_verify_latency_test.go | 3f25855736bbdfc4f0cfd23b0d6a1fc79f4acc00 |

主控在 Go 1.27.3 darwin/amd64 上运行：`TestSelectDefaultVerifyOutlastsIdentityBudgetOnHealthyState` 与 `TestSelectDefaultVerifyHonorsCallerCancellation` 通过；`scripts/run-go-test.sh -count=1 -race ./internal/store ./internal/sessionorigin` 通过；`go vet` 两包退出 0，`gofmt -l` 无输出，`git diff --check` 退出 0。

## Review — Round 1

方法：冷独立 Codex，executedModel gpt-6.2-astra、reasoning effort=xhigh，只读；在 `git archive HEAD` 的临时副本中覆盖候选文件运行探针。五个 blob 与上表一致。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 7/10。单次 `Select` 修复有效：只恢复旧调用后，健康慢读测试在 302ms 以 `context deadline exceeded` 失败，候选通过；调用方期限与取消、所有权、私有路径与权限、设备号与 inode 复核、schema 检查均保留；启动恢复与发布的身份读取仍为 200ms；未扩张公开设置、CLI 参数、输出字段或 schema。`-race` 定向 19 项通过。

- SEL-R1-F1 P1：`SessionScopeBatch` 串行读取九个视图，每个视图重新获得 5s 验证预算，CLI 到整批结束才输出。首项正常、其余八项身份查询持续超过 5s 时，旧接线 1.67s 返回一个可用视图，候选 40.34s 才返回；九项都健康但每项验证约 3.5s 时候选用 31.77s；Swift helper 默认 30s 终止进程，整批结果无法送达。应为整批验证设置共享的剩余预算，并为组包与输出留出时间，耗尽时沿用逐视图不可用语义且保留已验证结果，补"首项正常、后续持续慢"的批次回归。
- SEL-R1-F2 P2：store 测试先启动 300ms 释放计时器再记录起点，取消测试在 SQL 回调进入前启动 50ms 取消计时器；模拟 80ms 调度停顿时正确代码分别测得 219.98ms 与 `delays=0` 而失败。应以同步信号确认已进入等待或 SQL 回调后再释放或取消，超时断言只作防挂起保护。
- SEL-R1-F3 P2：期限测试把 `readOnlyBusyTimeout` 直接传给内部构造函数；把 `VerifyServingGeneration` 实际预算改为 10s 时全部相关测试仍通过。应通过 `VerifyServingGeneration` 本身观察有效期限，使 10s 变体被检出。

Reviewer 另测得 SQLite 独占锁 21.83ms 返回 `SQLITE_BUSY`，身份连接池占满时 5.0004s 返回 `context deadline exceeded`；单次验证错误不触发 `MaxSelectionRetries`，连续读到移动向量时三次验证各可接近 5s。

进入第 1 轮有界修复，repair_count=1。

## 第1轮有界修复 — SEL-R1-F1, SEL-R1-F2, SEL-R1-F3

repair_count=1。Developer 为 Claude Opus 5.5（请求 effort=high，实际 unknown），与 Round 1 的 Codex 评审者不同 harness。

- SEL-R1-F1：`internal/desktop/session_scope.go` 新增批次共享验证预算 `sessionScopeVerifyBudget`，九个视图及其重试共用一份剩余等待时间；每次验证的期限取调用方期限与剩余预算的较小者，用后扣除实际耗时；用尽时返回包装 `context.DeadlineExceeded` 的错误，经 `Select` 现有 `ErrGenerationUnavailable` 路径使该视图不可用（`generation_unavailable`），已验证的视图结果保留。`SessionScopeDefaultBatch` 因默认值变化重读时沿用同一预算。单视图（`SessionScopeView`、`loadSessionScopeView`）传 nil，保持本候选的单次 5s 语义。`store` 导出 `VerifyServingGenerationBudget = readOnlyBusyTimeout`，避免复制常量。预算取 5s 的推导：App 批次调用的 helper 期限为 30s，须覆盖启动、九个视图读取、验证、组包与输出；修复前验证最坏为 9 × (1 + `MaxSelectionRetries`=2) × 200ms = 5.4s，共享 5s 使其余工作至少保有原来的时间，单个慢验证仍有与单视图相同的耐心。red：`TestSessionScopeBatchSharesOneVerificationBudget` 在旧候选上 `batch with slow verifications took 40.095632992s, want at most 10s`；green 5.18s（`-race` 7.69s），首个视图可用，其余 8 个为 `generation_unavailable`。慢验证通过只作用于身份连接的 TEMP VIEW 递归 CTE 模拟，不触碰数据库文件，仍在 context 期限处结束。
- SEL-R1-F2：store 测试以 `db.Stats().WaitCount` 确认验证已进入连接等待后才释放或取消；sessionorigin 取消测试在被延迟的身份查询 SQL 回调内取消。超时只作防挂起保护。在临时副本注入 80ms 调度停顿：按评审重建的旧测试 3/3 失败（`220.45ms` 等、`delays=0`），新测试全部通过。
- SEL-R1-F3：删除直接调用内部构造函数的期限测试，新增 `TestServingGenerationVerifyEndsAtItsOwnBudget`，不给调用方期限、身份连接持续占用，经 `VerifyServingGeneration` 本身断言 `DeadlineExceeded` 且耗时在 [5s, 7s]；临时把预算改为 10s 时该测试失败（`verification budget ended after 10.000909569s, want 5s`），其余测试仍通过，证明旧测试检不出此变体。无新增生产钩子。

未改 helper 期限、wire 字段、输出结构或 schema；新导出常量只在 `internal/` 内使用。

Developer 报告的验证：`scripts/run-go-test.sh -count=1 ./internal/store ./internal/sessionorigin ./internal/desktop ./cmd/agentdeck` 退出 0；同三包 `-race` 退出 0；`go vet` 四包退出 0；改动文件 `gofmt -l` 无输出（`cmd/agentdeck/usage_stats_viewer_test.go` 为 HEAD 既有问题，未改）；`git diff --check` 退出 0。

主控核对修复后候选：

| 路径 | Blob |
| --- | --- |
| internal/desktop/session_scope.go | f119230f7c3c4e56dc608bce5301467a01b5d041 |
| internal/desktop/session_scope_batch_budget_test.go | 626ff1d4809d7f6ffb384ee1a1e5331d0090c248 |
| internal/sessionorigin/selection.go | 6771d4789911f6d271db6db92f3cb7382f379ca7 |
| internal/sessionorigin/selection_verify_latency_test.go | 7129275f2ee8a176e68c5499403340d24d79dce0 |
| internal/store/read_pool.go | 3ce64ac9f67c958f11f5a316621b7bcbbb41002d |
| internal/store/serving_generation.go | 6f2b0c48c17673bf15e7007906540495bb3955e3 |
| internal/store/serving_generation_test.go | d5052eb5efc552ff17eb201da09de052bbfa20ad |

待冷独立复评。

## Review — Round 2

方法：冷独立 Antigravity Gemini 4 Argon，运行时模型 `gemini-4-argon`，请求 thought level 为 high，实际级别未暴露。HEAD 为 `4ac56db32e13c4e925b648c620ba76fc588a7781`。上节 7 个 blob 与被评稿、当前修改前内容一致，排序后的 `路径 blob` 行 SHA-256 为 `c9b00458a31e9a905356aa7983f67fd1db9a28fa21a949532e65c7c22b09c628`。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 9/10。SEL-R1-F1 CLOSED：九个视图、读取重试及默认 scope 变化后的重读共用验证预算。两组隔离探针分别覆盖多个慢视图和默认值变化，整批约 5.2s，已验证结果保留，剩余视图沿用 `generation_unavailable`。SEL-R1-F2 CLOSED：在两个同步点前分别注入 80ms 调度停顿，相关延迟和取消测试通过。SEL-R1-F3 CLOSED：将公开验证入口预算改成 10s 后，`TestServingGenerationVerifyEndsAtItsOwnBudget` 在 10.00s 明确失败。连接钩子仅匹配当前测试的临时数据库和身份连接配置。

- SEL-R2-F1 P2，待关闭。新增 `store.VerifyServingGeneration(ctx, stateRoot)` 经过状态根所有权检查，但缺少 `cmd/agentdeck/ownership_packages_acceptance_test.go` 的入口登记和直接拒绝探针。独立 Reviewer 在 HEAD 得到退出 0，在候选得出 `uncovered exported state-root consumers: ["store.VerifyServingGeneration"]`。主控在同一候选通过 `scripts/run-go-test.sh -count=1 -tags=agentdeck_acceptance -run '^TestOwnershipPackageEntrypointInventory$' ./cmd/agentdeck` 复现退出 1。补登记并直接调用新入口，以实际所有权拒绝验证该路径。

Reviewer 的四包常规 Go 测试和 `go vet`、store/sessionorigin/desktop 的 race 测试、格式和 diff 检查退出 0。接受测试失败使该候选不能交付。当前验证证据未完成。

## 第2轮有界修复 — SEL-R2-F1

repair_count=2。主控仅改 `cmd/agentdeck/ownership_packages_acceptance_test.go`，登记新入口，并由拒绝探针直接调用 `store.VerifyServingGeneration`。原有读入口保留自己的用例，盘点注释与当前调用路径一致。上节 7 个产品和测试文件未改。新增候选 blob 为 `2ec3ae85d384efe326c7a5024345be7f479959f1`，排序后的 8 个 `路径 blob` 行 SHA-256 为 `196dba3535cc6cd8612dc4aa6e127cdf952d982c9beaacf964a6ef79460c640d`。

主控在同一工作树运行 `scripts/run-go-test.sh -count=1 -tags=agentdeck_acceptance -run '^(TestOwnershipPackageEntrypointInventory|TestOwnershipPackageEntrypointsBehindBarrier)$/^store[.]VerifyServingGeneration$' -v ./cmd/agentdeck`。入口盘点及新入口拒绝两项通过，退出 0。改动文件 `gofmt -l` 无输出，`git diff --check` 退出 0。待冷复评。

## Review — Round 3

方法为冷独立 Antigravity Gemini 4 Argon。实际模型 `gemini-4-argon`，请求 thought level 为 high，运行时未单独暴露档位。HEAD 为 `4ac56db32e13c4e925b648c620ba76fc588a7781`，八文件指纹 `196dba3535cc6cd8612dc4aa6e127cdf952d982c9beaacf964a6ef79460c640d`。Reviewer 与主控各自核对候选和定向验收。

评审结论：PASS

完成门禁：VERIFIED

总体评分 9.7/10。SEL-R2-F1 CLOSED：入口盘点包含 `store.VerifyServingGeneration`，拒绝探针直接调用新入口，保留旧入口独立用例。实际运行验证了拒绝类型和输出，两项定向 tagged 验收均通过。SEL-R1-F1、SEL-R1-F2、SEL-R1-F3 保持 CLOSED，前一轮验证的七个产品及单元测试文件均字节一致。无新增发现。

评审实际执行 CodeGraph、范围哈希、`gofmt -l`、`git diff --check` 和上节两个 tagged 验收，均退出 0。评审签发的结论覆盖八文件候选。

主控在 Go 1.27.1 darwin/amd64、macOS 26.7.3 上确认候选依赖。四包 `go build -mod=vendor`、`scripts/run-go-test.sh -count=1 ./internal/store ./internal/sessionorigin ./internal/desktop ./cmd/agentdeck`、前三包 `-race`、cmd 的 `agentdeck_acceptance` tagged vet 和两项定向验收均退出 0。CEv1 目标 `fix:selection-verify-200ms-deadline:state:196dba3535cc6cd8612dc4aa6e127cdf952d982c9beaacf964a6ef79460c640d` 四项必需证据有效：有界验证、失败先行、独立评审和相关回归。
