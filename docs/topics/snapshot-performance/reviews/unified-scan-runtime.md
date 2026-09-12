---
status: active
topic: snapshot-performance
subject: unified-scan-runtime
---

## Round 1 — 2026-09-11

## 📋 Unified scan runtime 实现评审

📊 总体评分：4/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**USR-R1-F1 — 高：晚到请求错误复用已完成域，未建立新的观察轮次。**

位置：internal/scanruntime/scanruntime.go:396–408、round.wait。
startOrJoin 仅检查整个 round 是否结束；只要 session 尚未完成，所有相同 home
请求都会返回旧 round，既不比较观察 cutoff，也不排队 follow-up。
usageDone 已关闭时，后来的 usage 请求立即成功返回旧结果，即使请求前已有新输入。
这违反 architecture §6.1 和 V06，以及 Task 1 的 fresh request 完成条件。

证据：临时 Go overlay 的 TestReviewLateUsageRequestRequiresFreshObservation
以确定性、可达的 usage completed/session processing 状态调用真实 startOrJoin
和 wait；断言失败：
`same_round=true usage=completed session=processing err=<nil>`。
这是内部状态机复现，不冒充真实文件或进程端到端测试。
现有 TestAcceptedRoundContinuesAfterWaiterDetachesAndUsesFiniteFollowUp
只在整个 round.done 关闭后测试新 round，未保护此窗口。

💡 修复：记录有限源观察边界；未被当前观察覆盖的请求绑定到不会被后续请求
无限推迟的 follow-up。保留当前 session 尾部执行，但不得以旧 usage 成功满足
新请求。增加 domain 已完成、另一域未完成及持续新请求的确定性回归。
Disposition：OPEN。

**USR-R1-F2 — 高：accepted 请求没有持久化、去重或重启恢复语义。**

位置：internal/scanruntime/scanruntime.go:185、369–393、765–799。
client 每次调用生成新 Request；server 解码该字段后从不使用它，直接 startOrJoin
并发送 accepted。整个服务仅保存内存 round 和 owner.json（stateID/nonce/protocol/PID）；
没有 accepted requirement/terminal journal、重复 ID 查询或恢复读取。
owner.json 是进程身份，不是请求 receipt。

风险：ack 后 worker 崩溃会丢失已接受义务；相同请求重试无法获得原接受结果，
只能重新扫描。数据库已提交并不说明某一 accepted 请求要求的域/观察已经完成。
architecture §6 明确要求先持久化再确认、幂等重连及检查点恢复，这属于 Task 1，
不能推迟到 Task 2/3。

💡 修复：实现有界私有请求 journal，先持久化 accepted requirement 再 ack；
按稳定请求 ID 去重/查询，记录终结及过期处置，重启时与域检查点协调恢复。
覆盖 ack 前后崩溃、commit 后 receipt 前崩溃以及同 ID 重试。
Disposition：OPEN。

**USR-R1-F3 — 高：生产路径仍先启动旧调度器，未实现 plan-before-admission。**

位置：internal/scanruntime/scanruntime.go:504–506；
internal/ingest/ingest.go:261–312。
executeProduction 仍直接 NewCoordinator/Start，再启动两个域的 Scan。
Start 立即创建读取 workers 和 schedule；schedule 在两个域尚未决定 skip/append/full
时即可按整个 source.Size × 64 申请预算并交给 read。它只有事后 allSkipped 检查，
没有等待双域规划，也没有按已验证所需范围做 body admission。

风险：双域本可 skip 的源依然可能先被打开和解码；单域 append 不能据域范围
规划共享读取。该实现是旧 coordinator 外包了一层 worker，未交付任务文本明确
要求的 replacement engine。这里不以最终 1 秒/10 秒未达标作为 finding。

💡 修复：在 body admission 前生成双方的 per-source 计划与可信读取边界，
仅调度所需 union range；将 reduction 和确定性 publication 分离并按实际持有内存
建立预算。以执行次序屏障和读字节/解码计数保护 both-skip 与 append/full 场景；
保留原语义 goldens 和两个历史取消/预算回归。
Disposition：OPEN。

### 🟡 改进建议 — 推荐

无额外风格建议。已得到阻断性反例，按项目要求停止扩大验证。

### 🟢 优点

- 域结果和前台等待 scope 分离；worker nonce 绑定 accepted/terminal 帧。
- 已有真实 Unix socket/helper 启动和域差异测试；这些通过项不等于上述缺失契约已实现。
- 本轮不要求修复 unowned Task 2 的 presentation/cache 候选，也不以原生 UI
  或最终性能目标作为 Task 1 的附加门槛。

### 📝 总结

Reviewer：Codex 主代理。Method：单代理、已批准契约对照、定向源码审查和
临时 overlay 确定性反例；不声称冷上下文独立性。没有修改生产、测试或配置。
Scope：Task 1 的 runtime 和摄入启动/观察/receipt 路径；其它调用入口和全部
并发/兼容场景未在阻断反例之后继续全面验证，不能从未列出 finding 推断通过。

HEAD：802df10163290b3ad995b6f8ae0defa7c409f3b5。
Reviewed content fingerprint：aeafa5416348e18e1f74ad931df713a5d6f77093cbbe0b75f75204c01097832b。
配方：SHA-256，首行为 head=<HEAD>，随后按下表顺序写入
`<git hash-object --no-filters>  <path>`，每行含末尾 LF。
状态文档和不属于 Task 1 的候选不在该实现指纹内。

| Path | Blob |
| --- | --- |
| cmd/agentdeck/contract_test.go | 710e7f54d978f8a4fd7ccaaf3e71f7c69c00031c |
| cmd/agentdeck/desktop.go | dd3b3218a5d7eb2f624bbe770f4c832847507fa3 |
| cmd/agentdeck/desktop_test.go | 5a8cd158eefa18a9d16381be8246e637f356a184 |
| cmd/agentdeck/e2e_test.go | f2f86164539a26b28e9589e6ff9e22b91adb6972 |
| cmd/agentdeck/main.go | d453cb7be64c3800e8927673c909cba7624c6a4c |
| cmd/agentdeck/release_test.go | 5109a49f4a2ee4504c048339e9e0be54aae9100c |
| cmd/agentdeck/scan_runtime_test.go | 4c58fcc54cf971e346a891f9ab2cd6abe2e5de4c |
| cmd/agentdeck/snapshot_performance_contract_test.go | a630301633e15dfa2238314f778ba18d41e487f8 |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | 8d7588699831ab0eab1e57dc6b0e41afdd020c31 |
| internal/ingest/ingest.go | 490fa310fc31ad3d18adabed3b680e53404c05e5 |
| internal/ingest/ingest_test.go | 3219c4e5d885f6d41a8e11118a4d8b5180ffabf2 |
| internal/scanruntime/scanruntime.go | 1cce0e9402fa114632d95306175465be3f07b95a |
| internal/scanruntime/scanruntime_test.go | 99c217d01f49df6537c2d11325ab371f68adfb92 |
| internal/session/session.go | cfb0c8e38587290d115a1026b4d2c1e1e79d0490 |
| internal/session/ingestion_sql_test.go | 60f9536246a387313aff281d3e1289ddab6a7448 |
| internal/usage/usage.go | 25d5f231732f42571f5a68813a7d96a4c65ca0c2 |
| internal/usage/ingestion_statements.go | b233579b228fba2ed4ddfaaa4430e9844069e005 |
| internal/usage/ingestion_statements_test.go | 8f26bc491860b150a0ff549e6b2aa0fcec792bf8 |
| internal/usage/source_classification_test.go | ffeeb4e272098567085a3b9a4d0abaaa0dc1d6d3 |

开发交接宣告的 b919c08ac09700b6165f7cd1b7f9604d966104d78b4f9cce508c224eda1babbe
未在本轮按其列出的路径顺序复现；不把该旧状态的 VERIFIED 直接移用于当前内容，
也不改写旧观察。上述有序明细固定本轮实际被审候选。

Evidence：`GOCACHE=/private/tmp/agent-deck-go-build
AGENTDECK_GO_TEST_LOG=/private/tmp/agentdeck-usr-review.u2MG42/result.log
scripts/run-go-test.sh -overlay /private/tmp/agentdeck-usr-review.u2MG42/overlay.json
./internal/scanruntime -run '^TestReviewLateUsageRequestRequiresFreshObservation$'
-timeout 30s`，exit 1；测试逻辑断言失败，不是编译/环境错误。
临时 overlay 与日志保留在 /private/tmp/agentdeck-usr-review.u2MG42 供修复者复核，仓库中未添加测试文件。
F2/F3 是直接源码控制流与明确完成契约不符，未伪称已做 crash 或吞吐实测。
开发交接的 full Go/race/vet/build 是原有报告，本轮不重跑、不扩大其证明范围。

完成门禁：FAILED。固定 gate-status.cypher 对上述当前 ContentState 的查询返回
FAILED；freshness-cancellation-and-waiter-regressions、worker-lifecycle-and-isolation、
shared-ingestion-and-semantic-equivalence 三项有当前 applicable fail evidence。
本轮追加 4 个节点、6 条关系；预检 6/6 ok，实际创建数量匹配。
其他三项没有为此新状态补记 passing evidence；未以旧交接门禁覆盖当前失败。
复现日志 SHA-256：49f6a07a7054dc73eb6cb3692569c946cda6085b432a3e1603310d45a4412692。
复现源码 SHA-256：7bee086c605a50f4c4ce95441d0b553d7eb7310b58951d87b1b7a66119c2b4ba。
Task：ad-sp-unified-scan-runtime-dev。
WorkUnit：urn:ce:agent-deck:work-unit:snapshot-performance-unified-scan-runtime。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task Review 保持未勾选；返回同一任务修复，不创建新任务，不启动 Task 2。
未提交或推送。

下一步指令：修复：snapshot-performance / reviews/unified-scan-runtime.md / USR-R1-F1、USR-R1-F2、USR-R1-F3
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 2 — 2026-09-12

## 📋 Unified scan runtime 修复复评

📊 总体评分：6/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**USR-R1-F2 — 高：accepted request 的过期身份仍会被遗忘，未形成显式 re-evaluate 处置。**

位置：internal/scanruntime/receipts.go:29、189–217。
修复候选已在 ack 前持久化 accepted requirement，并记录 scoped/global terminal
结果；但 `receiptExpired` 只被声明和作为腾挪条件读取，没有任何路径把 receipt
转换为该状态。`prune` 在保留期后直接删除 terminal entry，随后相同稳定 request ID
会被当成全新请求接受，而不是返回 architecture §6 要求的 expired/re-evaluate
结果。journal 的持久化、去重和重启恢复只关闭了原 finding 的一部分。

- Disposition：STILL OPEN。
- 行为风险：超过保留期的重试无法区分“从未接受”与“曾接受但身份已过期”；调用方
  可能把未知旧请求重新解释成新义务，破坏稳定 request ID 的幂等边界。
- 证据：`receiptExpired` 仅见于状态声明和 `makeRoom` 条件；
  `prune` 对过期 terminal receipt 执行 `delete(j.entries, id)`，而 `accept` 对缺失 ID
  直接创建新 receipt。

💡 修复：为已裁剪 request ID 保留有界 tombstone/expired identity，或采用等价的
有界机制，使重复提交得到显式 expired/re-evaluate 响应；补充跨保留期和容量淘汰的
稳定 ID 回归，同时保持 journal 私有、有界和原子持久化。

**USR-R1-F3 — 高：plan barrier 已建立，但 reduction 仍在写事务内等待共享流。**

位置：internal/session/session.go:458–464、500–559、838 起；
internal/scanruntime/scanruntime.go:732–766。
修复候选已在 `Coordinator.Start` 前完成 usage/session 双域 plan 与 seal，并按 union
range admission；这关闭了“先读后计划”的部分缺陷。但 session 路径仍在
`scanSourceWithCoordinator` 开头执行 `BeginTx`，随后把该 transaction 作为 executor
传给 `prepareSourceUpdate`，后者才从 coordinator stream 执行 `parsePreparedStream`。
因此数据库写事务在共享输入的读取、解码和 domain reduction 全程保持打开，未满足
architecture §7.2、§8.1 和原 finding 修复要求的 reduction/publication 分离。

- Disposition：STILL OPEN。
- 行为风险：慢 reader、背压或另一域停顿会延长写事务持有时间，继续耦合 ingestion
  调度与 deterministic publication；当前实现仍不足以证明 replacement engine 的
  事务/内存/进度边界。
- 证据：生产路径确实以 `RequirePlans: true` 建立计划屏障；但 session 的直接控制流
  明确为 `BeginTx` → `prepareSourceUpdate` → `parsePreparedStream`，与“不得在 reducer
  等待 source/channel input 时持有 write transaction”的契约相反。现有 plan barrier
  和 union-range 测试不覆盖该事务时序。

💡 修复：在进入写事务前完成有界 session reduction，生成只含批准派生数据和发布
校验信息的结果；随后按确定性顺序开启短事务、重新验证 ownership/cursor/source
假设并原子发布。增加执行次序屏障，证明共享流未完成时写事务尚未开启，并保留
plan-before-admission、union range、语义 golden 与取消/预算回归。

### 🟡 改进建议 — 推荐

无。两个原高严重度 finding 仍未完整关闭，本轮不扩大验证范围。

### 🟢 优点

- USR-R1-F1 CLOSED：`selectRoundLocked` 在请求域已完成而另一域仍运行时创建唯一
  pending observation，后续请求复用该有限 follow-up；它等待 predecessor terminal
  后执行，不会被持续新请求无限后移。
- 定向回归 `TestLateScopeQueuesOneFollowUpBeforeOtherDomainCompletes` 在当前候选通过，
  覆盖 domain 已完成、另一域未完成和连续晚到请求的原反例窗口。
- F2 已补齐 ack 前持久化、同 ID 去重、terminal 记录与非终结 receipt 的重启重规划；
  F3 已补齐双域 plan/seal 以及 both-skip/union-range admission，均是有效但尚不完整的修复。

### 📝 总结

逐项处置：USR-R1-F1 -> CLOSED；USR-R1-F2 -> STILL OPEN；
USR-R1-F3 -> STILL OPEN。没有新 finding，也没有回归已关闭项。由于每个既有 finding
都必须关闭后才能 PASS，本轮结论为 FAIL，返回同一 Task 修复，不启动 Task 2。

Reviewer：Codex 主代理。Method：单代理、契约对照、定向源码控制流复核和一条
最小确定性回归；未声称冷上下文独立性。Scope：用户指定的三个 Round 1 finding；
发现 F2/F3 的决定性源码证据后停止广泛验证，没有重跑 repair handoff 的 full Go、
race、vet 或 cross-build，也没有修改生产、测试或配置。

HEAD：802df10163290b3ad995b6f8ae0defa7c409f3b5。
Reviewed content fingerprint：6f87021cfab2cff24f3cf45d3daa09c89d119510c855340307d7ce0bd2a47926。
配方沿用 Round 1 的 Task 1 有序 blob manifest，并加入 repair 已声明的
`internal/scanruntime/receipts.go`；首行为 `head=<HEAD>`，每个 blob/path 行及末行均含 LF。
复算值与 repair handoff 的 ContentState
`urn:ce:agent-deck:state:workspace:fcd2bfd07e9fccfe839cb00a5d06f2a3c5cdbcb166db6f304f378708d0646224`
不一致，因此不把该精确状态的 VERIFIED 门禁移用于当前候选。

Evidence：`AGENTDECK_GO_TEST_LOG=/private/tmp/agentdeck-usr-rereview-f1.log
scripts/run-go-test.sh ./internal/scanruntime -run
'^TestLateScopeQueuesOneFollowUpBeforeOtherDomainCompletes$' -timeout 30s`，exit 0；
日志 SHA-256：9913cfd57c13ef6073eb8fb5c7b187c388f287a24b497d5094ae7098fb089b41。
F2/F3 的证据为上述当前源码控制流；未把未执行的 crash、吞吐或完整 L3 检查写成通过。

完成门禁：FAILED。当前 ContentState 的 F2/F3 applicable fail evidence 绑定到
worker-lifecycle-and-isolation 与 shared-ingestion-and-semantic-equivalence；F1 的当前
定向通过不覆盖其余 Task criteria。Task：ad-sp-unified-scan-runtime-dev。
WorkUnit：urn:ce:agent-deck:work-unit:snapshot-performance-unified-scan-runtime。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task Review 保持未勾选；返回同一任务修复，不创建新任务，不启动 Task 2。
未提交或推送。

下一步指令：修复：snapshot-performance / reviews/unified-scan-runtime.md / USR-R1-F2、USR-R1-F3
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 3 — 2026-09-12

## 📋 Unified scan runtime 第二次修复复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- USR-R1-F1 保持 CLOSED：有限、合并的 follow-up observation 状态机及其定向
  回归未被本轮 F2/F3 修复改变；当前精确状态的 freshness/cancellation/waiter
  criterion 继续由 applicable pass evidence 覆盖。
- USR-R1-F2 CLOSED：terminal receipt 在 retention 到期或 active-capacity 腾挪时
  转换为持久 `expired` identity，清除结果但保留稳定 request ID；重复提交进入
  `scan request receipt expired; re-evaluate required`，而不是重新接受未知旧请求。
  active receipt 与 expired tombstone 各自有界，保存失败会恢复内存状态。
- USR-R1-F3 CLOSED：session 先在事务外完成共享流读取、解析和有界 reduction，
  再开启 publication transaction；事务内重新验证文件 identity/size/mtime/prefix
  以及数据库 source/cursor/partial/priority/parser state，冲突返回
  `ingest.ErrSourceChanged`，随后才原子应用并提交结果。
- 新的确定性测试覆盖 retention tombstone、capacity tombstone 以及 reduction 尚未
  释放时 publication transaction 不得启动；既有 full Go、race、vet 和 darwin
  双架构证据已绑定当前 ContentState，可直接复用。

### 📝 总结

逐项处置：USR-R1-F1 -> CLOSED（保持）；USR-R1-F2 -> CLOSED；
USR-R1-F3 -> CLOSED。没有 still-open、regressed 或新增 finding。Round 1 的三个
finding 已全部关闭，因此本轮结论为 PASS。

Reviewer：Codex 主代理。Method：单代理、Round 2 修复要求对照、当前源码控制流
复核和精确状态 CEv1 证据复用；未声称冷上下文独立性。Scope：用户指定的
USR-R1-F2、USR-R1-F3，并确认 USR-R1-F1 的既有关闭状态未回归。没有修改生产、
测试或配置，没有重复执行已由当前门禁覆盖的 full Go、race、vet 或 cross-build。

HEAD：802df10163290b3ad995b6f8ae0defa7c409f3b5。
Reviewed content fingerprint：0f4f3cdf16d8cfd523e94d56653cfeb3f3e5f12aef39aa7405e938d6de4229e9。
当前 Task 1 有序 manifest 复算值与 repair handoff 的 ContentState 完全一致；topic
状态和未归属 Task 2 的候选不在该实现指纹内。

Evidence：固定 `gate-status.cypher` 对
`urn:ce:agent-deck:state:workspace:0f4f3cdf16d8cfd523e94d56653cfeb3f3e5f12aef39aa7405e938d6de4229e9`
返回 VERIFIED，6 项 required criteria 均有 target-matching applicable pass evidence，
missing/invalidated/unresolved 均为空。F2 定向日志 SHA-256：
eb9ef0269038892a15628b25b161427c9771e271721cb946d0db8bd3e6498f92；
F3 定向日志 SHA-256：4aa2e0b048619f8116018b3febc8e41a3701f92ce2010264fe329daf0bc2e866；
full Go 日志 SHA-256：ea3fb1f15154053418050cc601a664a0c95cb9481adb32222157e7163f406486。

完成门禁：VERIFIED。Task：ad-sp-unified-scan-runtime-dev。
WorkUnit：urn:ce:agent-deck:work-unit:snapshot-performance-unified-scan-runtime。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task Review 已通过；Task 2 尚未启动。未提交或推送。

Task checkpoint：ad-sp-unified-scan-runtime-dev；content_state=0f4f3cdf16d8cfd523e94d56653cfeb3f3e5f12aef39aa7405e938d6de4229e9；gate=VERIFIED。
提交建议：以 unified-scan-runtime Task 1 的实现、测试、Round 1–3 评审记录和 topic 状态为一个候选提交边界；提交需另行授权。
推送建议：Task 1 授权提交并验证提交对象、署名、SSH 签名和任务关闭后，再推送 feature/snapshot-performance；推送需另行授权。

下一步指令：提交：snapshot-performance / unified-scan-runtime
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 4 — 2026-09-12

## 📋 Unified scan runtime 完整交付范围复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- USR-R1-F1、USR-R1-F2、USR-R1-F3 均保持 CLOSED；本轮扩展身份范围没有改变
  已复评的 runtime、receipt 或 reduction/publication 实现。
- 完整 Task 1 manifest 现在包含 `internal/ingest` 的 metrics 与三套平台 stat
  编译单元，以及直接保护 F3 的 `internal/session/session_test.go`。
- metrics 只提供 opt-in 原子计数，不改变正常扫描输出；平台 stat 文件以互斥
  build tag 实现 change-time 读取，覆盖 darwin、linux 和其它平台的保守退化。
- F3 测试通过确定性 barrier 证明 shared reduction 未释放前 publication
  transaction 不会启动，并在释放后验证完整扫描结果。

### 📝 总结

逐项处置：USR-R1-F1 -> CLOSED（保持）；USR-R1-F2 -> CLOSED（保持）；
USR-R1-F3 -> CLOSED（保持）。新增检查的五个交付文件没有引入新的 finding。
Round 3 的 PASS 保持有效；本轮修正的是其 ContentState 未覆盖完整可提交边界的问题。

Reviewer：Codex 主代理。Method：单代理、完整编译单元清单核对、遗漏文件源码/
差异复核，以及 completion-evidence/v1 显式 preservation lineage；未声称冷上下文
独立性。Scope：完整 unified-scan-runtime Task 1 交付 manifest。没有修改生产、
测试或配置，也没有重复运行内容未变且证据仍有效的 full Go、race、vet 或 cross-build。

HEAD：802df10163290b3ad995b6f8ae0defa7c409f3b5。
Reviewed content fingerprint：acf87d7b808cfce398022df6fc871ddab0da3024de79a0c85eb2e57f32d88642。
配方：SHA-256；首行为 `head=<HEAD>`，随后为完整 Task 1 的 25 个词典序
`<git hash-object --no-filters>  <path>` 条目，每行及末行均含 LF。相对 Round 3
新增纳入 `internal/ingest/{metrics.go,stat_darwin.go,stat_linux.go,stat_other.go}`
和 `internal/session/session_test.go`；topic 状态和未归属 Task 2 的候选仍被排除。

Evidence：从 Round 3 的 `0f4f3cdf16d8cfd523e94d56653cfeb3f3e5f12aef39aa7405e938d6de4229e9`
到当前状态记录一个仅修正身份范围、无工作树内容变化的 Change；对六项旧证据
分别记录 preservation assessment，并创建目标状态 roll-up。30/30 关系预检为
`ok`，30 条关系创建成功。固定 `gate-status.cypher` 对当前 ContentState 返回
VERIFIED；六项 required criteria 均有 target-matching applicable pass evidence，
missing/invalidated/unresolved 均为空。

完成门禁：VERIFIED。Task：ad-sp-unified-scan-runtime-dev。
WorkUnit：urn:ce:agent-deck:work-unit:snapshot-performance-unified-scan-runtime。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task Review 保持通过并等待授权提交；Task 2 尚未启动。未提交或推送。

Task checkpoint：ad-sp-unified-scan-runtime-dev；content_state=acf87d7b808cfce398022df6fc871ddab0da3024de79a0c85eb2e57f32d88642；gate=VERIFIED。
提交建议：以完整 25 文件 Task 1 manifest、Round 1–4 评审记录和 topic 状态为一个候选提交边界；排除 Task 2 presentation/cache 候选；提交需另行授权。
推送建议：Task 1 授权提交并验证提交对象、署名、SSH 签名和任务关闭后，再推送 feature/snapshot-performance；推送需另行授权。

下一步指令：提交：snapshot-performance / unified-scan-runtime
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
