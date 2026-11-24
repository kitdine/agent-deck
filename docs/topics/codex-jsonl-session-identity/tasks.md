---
status: active
created: 2026-11-17
---

# Codex JSONL 子会话身份与用量归属 — Tasks

来源为 `ad-bug-codex-subagent-jsonl-session-identity`（从 `ad-bug-codex-subagent-usage-attribution` 拆分），v0.7.5 Batch 3 / P2，Lane B。验收要求区分实际执行子线程和直接父线程，修正缺失或错归的模型与会话索引/用量身份，隔离继承父历史，处理并去重新旧用量记录，并使 P/C/All 按已证请求及定价对账。回填可重复执行，保全既有安全数据。纯重归的守恒边界、去重/补采的合法变化和未恢复小计见 [requirements.md](requirements.md) 与 [architecture.md](architecture.md)。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

本主题无独立交互 surface，UX 文档不适用。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `native-source-proof` | [x] | [x] |
| 2. `fenced-state-layout` | [x] | [x] |
| 3. `usage-ledger-replay` | [x] | [x] |
| 4. `otel-ownership-receipts` | [x] | [x] |
| 5. `session-effective-index` | [x] | [x] |
| 6. `session-origin-bindings` | [x] | [x] |
| 7. `scoped-usage-aggregation` | [ ] | [ ] |
| 8. `same-generation-publication` | [ ] | [ ] |
| 9. `portable-state-recovery` | [ ] | [ ] |
| 10. `source-issue-acceptance` | [ ] | [ ] |

## 基线、授权与写入归属

- 本稿对应工作区 `agent-deck.codex-jsonl-session-identity`、`feature/codex-jsonl-session-identity`、HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。启动矩阵 blob `b2a9bc6718d55c905ef6bbcea1bb30a1b7166852`，指纹 `7361eceaa030ac6e1a4605bc0fcc079182060322c0142e686463f57dee42bfa8`。需求 blob `f5ebb617dd1acf54f3728712adc5f2f518d343e2`、架构 blob `a4f258224d7380cfbf7b018662793f8e3245b6af` 均已正式 Round 3 PASS 并记录 VERIFIED 4/4；架构状态为 `codex-jsonl-session-identity:architecture.md:state:8e9fabb8634a30cf1af1a39f905a98a621574c76c85d09a9d73b2f4c3a449ccc)。见 [需求评审](reviews/requirements.md#round-3--2026-11-18) 与 [架构评审](reviews/architecture.md#round-3--2026-11-18)。
- 版本合同工作树 `docs/topics/v0-7-5-contract/tasks.md` blob `14b97f89fc8a1eab0caa483536a8cb63e9a4f276`，对照 main `a224b0b824d2cad451d3bd83153832d65506f0fe`、已观察 core schema 35。**基线同步**由主控在实现前按授权同步实际目标 main，重新检查源位置、现有迁移、信号预聚合、watch 取消/释放及同版 selection/serving 写者。**编号分配**是单独检查点，主控按同步后的目标与全部在途批准迁移分配 core 追加号、Codex usage/session parser 目标及私有 session 布局版本，明确迁移执行顺序。本文使用架构中的符号，既不预留号码，也不将这两步登记为产品实现 Task。若同步导致批准契约或证据前提变化，主控按影响回到对应记录处理。
- 保持现有明文发现范围、已有 CLI/桌面/Widget 读面与其他客户端合同。仅剩 `.jsonl.zst` 的历史按已通过需求报告未恢复。价格/实际服务模型映射仍由 `ad-cost-transparency` 负责。本主题不读取真实 rollout、不改上游、不增加公开字段或交互。所有验收使用隔离合成数据。
- 下文的 owner 是变更责任面，实际认领和移交由主控核对。每个共享文件、接口及 fixture 仅一名当前写者。消费方用冻结契约和新属主文件；需要修改他人文件时，将确切调用点和所需断言交给原 owner，或主控在前一写者退出后正式串行交接。文件范围为提案允许边界，认领时复核目标内容和任何重叠。不要因选择不同测试入口重复改同一文件。
- 主控拥有正式矩阵、评审记录、Beads、CEv1 和 Git。独立评审与精确候选证据通过后，依版本合同按原 issue 交付。自动授权涵盖既有本 issue 的 commit、push origin、PR 与适用 CI 通过后的 Codex review；无需用实现 Task 重复取得这项授权。无 merge、安装、release、deploy 权限。

## 依赖、并行与集成

| 顺序 | 可执行范围 | 进入下一检查点的证据 |
| --- | --- | --- |
| 接口冻结前 | `native-source-proof` 可先做不依赖数据库的 lexer/reducer 与黄金合成输入；`fenced-state-layout` 可先做不接候选的旧路径拒绝、布局恢复模型及基线 harness。其他 owner 可独立编写所属的纯区间算术、FTS ordinal 对照、故障屏障及隔离测试夹具，限独立文件和可独立断言的行为；数据库、调用方和生产接线等待对应契约冻结。 | 主控登记文件/接口写者。共享输入样例、断言及不相交文件可先独立审计；不把接口草案算作已完成实现。 |
| C1 来源和状态契约 | 合并 1、2 的接口交付。解析输出为来源/执行/段及 capture 范围证据、调用/区间输入、白名单材料分流和可重放 checkpoint；状态输出为 fenced locator、epoch/schema gates、持久结构及快照/事务入口。冻结 source kind→parser 目标、键/证据、候选-开放区分及失败/partial 语义。 | 合成 fixture 在 parser 两消费方使用同证据；新 schema 可迁移、回滚，旧名无 staged 布局。可执行旧二进制跨命令矩阵仍由 10 验证。 |
| C2 域内结果 | 3、4、5 依赖 1+2，可在冻结边界后并行。6 依赖 1+2+5 的有效索引读合同，7 依赖 2+3 的请求/区间查询与公开切片语义。4 消费 3 的发布资格接口，4 在该接口冻结后可与 3 并行，完整旧账判定在集成时接入，避免同时改两个 publisher。 | 三方在同套小 fixture 交接：3 产出可重放账组及实际差分，4 产出原侧收据/marker 的同事务操作，5 产出 session seal/有效读图，6、7 输出同票读者所需分类及聚合。一份 core 开放入口尚未接通前，不发布中间组合。 |
| C3 同代端到端 | 8 依赖 3–7 通过域内集成，统一 core 最终 CAS、CLI/scanruntime/watch、候选 slot 生命周期和读保护。9 的备份/诊断纯校验可此前独立；实际接线依赖 8 及 2、4、5 的 fenced snapshot 和 seal。 | A6c 挂起、失败、重启、旧票与新票端到端完成，9 只导出当前安全选中代。所有写者禁止绕过 gate。 |
| C4 交付候选 | 10 依赖 1–9，使用同一综合候选跑真实旧构建、端到端和最终 L3。 | 主控核对逐项证据和候选范围，独立评审及必要修复/复评；同一 issue 的验收完整后进入既有交付顺序。 |

跨域冻结内容是实现内部契约，沿已通过架构，包括事实/证明与允许正文的分别归属、proposal 输入 stamp、effective view 界面和最终 CAS。即便实现采用同一包，也按下文分出物理文件责任。生产接线无需新增公共特性开关；中间 Task 用封闭的测试/候选入口验证，C3 统一接入。主控将 selection、serving、自扫描及分类相关的在途变更串行对齐；无需本题修改共享原生 wire/UI，只以现有 helper 和投影数据合同验收。如实际发现必须超出已批准边界的直接读取者，记录路径和具体兼容影响，交主控处理。

## 各锚点交付

下列 `TestCJSI…` 是本提案指定由各 owner 添加的测试入口名。命令执行时须从 runner 日志确认匹配测试实际进入，避免空选择被当作通过。证据等级见 [Project Rules](../../../.agent-instructions/project-rules.md)。

### native-source-proof

**依赖与所有权。** 无产品 Task 前置。独占 `internal/ingest/` 中本题的 parser/reducer、共享原生合成记录和 checkpoint 类型，以及 `internal/activity/activity.go`、`extraction.go` 的本地 tool/turn 与按需活动归属接线。涉及 `Coordinator`、`FileGeneration`、`parseCodex`、`ReadDetails`、`ReadDetailsPage`。不写 usage/session adapter。只把证实的执行身份、源位置、计量存在性与归属证据公开给内部消费者；允许正文短暂进入索引 reducer，不进持久化 proof。

**交付与验证（L2，隐私/交接纳入最终 L3）。** 证明 C 首头与直接父区分、规范名冲突零新增、继承和右锚拒绝歧义、工具完成配对、原生 token 形状、缺项/整数及完整 checkpoint；同一解析输出可同时服务 session 和 usage。拥有 `internal/ingest/cjsi_source_proof_test.go`、`internal/activity/cjsi_source_proof_test.go`。

```sh
scripts/run-go-test.sh ./internal/ingest ./internal/activity -run '^TestCJSINativeSourceProof$'
```

### fenced-state-layout

**依赖与所有权。** 主控完成独立版本分配后接持久迁移。独占 `internal/store/store.go`、`migrations.go`、`schema_probe.go` 及本题新增 layout/migration 文件，负责 `OpenSessions`、`OpenSessionsReadOnly`、`migrateSessionSchema`、schema-ahead、私有 session locator/epoch 和受管切换。为 owner 3/4/5/8 提供经批准的证据、区间、收据、候选及协调头 schema；其他 owner 不追加另一套全局 DDL。旧触发器的升级 DROP 及冷建最终结果由本锚验证，旧 telemetry 安装 SQL 只由 4 修改，两个变更在 C2 同候选核验。原 schema35 旧初始化和备份用于验收，开发期间保持真旧行为基准可还原。

**交付与验证（L3）。** 同一迁移在 schema35、冷建和中断恢复下正确；证实旧名拒读写，新 locator 只在可验证的布局上生效，从未建索引不隐式创建实际库；迁移窗口要排空既有旧连接。拥有 `internal/store/cjsi_layout_test.go`。真正旧 CLI 全入口证据由 10 提供，不能用伪旧 reader 替代。

```sh
scripts/run-go-test.sh ./internal/store -run '^TestCJSIFencedStateLayout$'
```

### usage-ledger-replay

**依赖与所有权。** 1+2。独占 `internal/usage/usage.go`、`source_pipeline.go`、`source_identity.go`、`codex_signals.go`、`cold_publish.go` 和专用账组文件。修改 `prepareSource`、`publishPrepared`、`publishFacts`、`commitUnchangedCoverageBlock`、`restoreEventRunBindings`、`restoreEventRunBindingsForSourceRange`。已有 `Summary`、`SummaryRange`、`Sessions`、`Stats`、`Diagnose` 在同一 `usage.go`，本 owner 按 7 的冻结输入/结果接入这些调用点，并按 8 的冻结门控接入最终 writer，7、8 不并行改该文件。proof 成员资格交 4，最终权威撤回不在 `publishFacts` 自行决策。

**交付与验证（L3）。** 原生新请求与旧通知独立/镜像/冲突分类；精确 token、存在性、区间不分摊，移除仅头撤回。可定位可重放与不可定位/不可重放互斥，保住真 P、历史 route/run 证据和老安全侧；按预算续扫、缺源保全、合法副本切换、改写/并发/重复稳定。交接实际差分、拟撤旧键和窗口相关证明给 4/7/8。拥有 `internal/usage/cjsi_ledger_replay_test.go`。

```sh
scripts/run-go-test.sh ./internal/usage -run '^TestCJSIUsageLedgerReplay$'
```

### otel-ownership-receipts

**依赖与所有权。** 1+2 及 3 的发布资格接口。独占 `internal/store/telemetry*.go`、`internal/telemetry/` 中本题的结算/投影调用方和 `cmd/agentdeck/telemetry.go`。用 `SettleTelemetry`、`SaveSourceThreadIdentities`、`RetractTelemetryForJSONLThreads` 原入口梳理唯一权威，接 `Reconciler.Drain/Import/Settle`、`SettleAndProject` 及 receiver；session 物理 projection 由 5，最终事务由 8，schema 迁移由 2。

**交付与验证（L3）。** 去除冷建绕过路径与旧身份/INSERT 撤回；保存原计量、state/reason、键、marker、时间、路由和接管期收据，经整线程判定同事务接管/恢复。含 pending、未命中线索、歧义和迟到观察；无旧撤回证明不伪造恢复。拥有 `internal/store/cjsi_otel_ownership_test.go`、`internal/telemetry/cjsi_otel_reconciliation_test.go`。

```sh
scripts/run-go-test.sh ./internal/store ./internal/telemetry -run '^TestCJSIOTel(Ownership|Reconciliation)$'
```

### session-effective-index

**依赖与所有权。** 1+2。独占 `internal/session/session.go`、`source_pipeline.go`、`unchanged.go`、`affected_view.go`、`scope_snapshot.go`、`telemetry_projection.go` 及本题索引文件；另独占 `cmd/agentdeck/session_viewer_data.go` 和需要的现有 viewer 顺序消费文件。覆盖 `prepareSourceUpdate`、`parsePreparedStream`、`ReadIndexSnapshot`、`affectedViewChanges`、`List/Search`、`ShowMetadata/DocumentsPage/ShowWithActivity`、`ProjectTelemetrySessions`。由 8 在 `main.go` 调用这些接口；`session/doctor.go` 归 9。

**交付与验证（L3）。** 把有效来源、稳定 doc_id、详情 ordinal、FTS 遮蔽/排序/总数、同票活动和用户排除接入底表/候选/compact。旧安全组的 rowid 序在核实前只服务旧版；中间插入和冷建顺序相同且不会重排搜索。私有候选证明/序号完整性提供给 8；给 6 有效 birth/来源读合同，给 9 导出同票 ordinal 与高水位。拥有 `internal/session/cjsi_effective_index_test.go`、`cmd/agentdeck/cjsi_viewer_order_test.go`。

```sh
scripts/run-go-test.sh ./internal/session ./cmd/agentdeck -run '^TestCJSI(EffectiveIndex|ViewerOrder)$'
```

### session-origin-bindings

**依赖与所有权。** 1+2+5。独占 `internal/sessionorigin/` 中本题的 reduce/selection/binding/reconcile/publication 实现、`internal/store/session_origin.go`、`session_origin_derive.go`、`cmd/agentdeck/session_origin.go`。接 `ResolveBinding`、`Reconcile`、`ReadIndexSnapshot` 消费和 classifying 三条入口；8 负责把结果绑定到同一次票据开放。

**交付与验证（L3）。** 出生优先级和连续证据正确，原用户 revision/时间/收据保持，未证不同 birth 全读面 unmatched、并发 clear 不复活；持久与列表/详情/Work、Auxiliary、All 同票。排除只去原范围，旧缓存不泄漏，不改变 usage All。拥有 `internal/sessionorigin/cjsi_binding_test.go`、`cmd/agentdeck/cjsi_classification_test.go`。

```sh
scripts/run-go-test.sh ./internal/sessionorigin ./cmd/agentdeck -run '^TestCJSI(Binding|Classification)$'
```

### scoped-usage-aggregation

**依赖与所有权。** 2+3。独占 `internal/usage/session_usage.go`、`signals.go`、`signals_batch.go`、`signals_cost.go`、`signals_report.go`、`projected_events.go`、`presentation.go` 及新增区间查询/reducer 文件；`internal/store/usage_partitions.go`；`internal/desktop/derived_cache.go` 及 `desktop.go` 的汇总、投影数据接线。必要的已有 usage stats 呈现文件也归本 owner。核心 `usage.go` glue 由 3，`serving_cache.go`/scope lifecycle 由 8。接 `SessionUsageSummary/SessionInvocations`、`SummaryRange` / `Stats` 的 helper、`SignalsBatch`、`presentationInputs`、`PublishUsagePartition`。保留 main 信号预聚合与兼容旧客户端快路径。

**交付与验证（L3）。** 区间只加 token/合格 distinct 会话，保留调用数、时间半开窗口、未知维度、小计/完整金额和 All 口径；负样本覆盖坏缓存、空与未证零、删除最后区间、OLD/NEW 脏键、按实际来源版本与 revision 失效。对照公开 CLI 和现有桌面数据合同，不需启动 App。拥有 usage、store、desktop 三包对应的 `cjsi_aggregation_test.go`。

```sh
scripts/run-go-test.sh ./internal/usage ./internal/store ./internal/desktop -run '^TestCJSI(ScopedAggregation|PartitionInvalidation|DesktopTotals)$'
```

### same-generation-publication

**依赖与所有权。** 3、4、5、6、7。独占 `internal/scanruntime/` 的现有 round/serving 编排和恢复、`internal/store/serving_generation.go`、`derived_generation.go`、`source_coverage.go` 以及私有协调实现；`internal/desktop/serving_cache.go`、`session_scope.go`；`internal/watch/watch.go`、`internal/ingest/metrics.go` 的发布诊断接线与必要 `internal/perftrace/` 固定观测。独占 `cmd/agentdeck/main.go` 及通用 `source_coverage.go` 的 CLI 接线，承接 show、token/活动、watch/fingerprint、purge 及 backup/doctor 包调用；包行为归各 owner，8 只在此写共享入口。对接 `BuildToday`、`prepareServingSnapshot`、`writeServingSnapshot`、`OpenServingGenerationReader`、`ReadSourceCoverage`、`AcquireMaintenance`、`markEnd`。8 统筹最终 CAS，4 保有 OTel 换侧函数，2 保有迁移/路径，5 保有索引视图/compact，7 保有聚合。

**交付与验证（L3）。** 同一差分依合同 seal，计算/序列化/文件和目录 fsync 均在阻塞写者的锁外；最终 CAS 与 OTel/coverage/binding 原子开放，晚到、崩溃与旧缓存按原语义，并发非依赖写入继续。usage-only 不打开 index，异常持有走既有 `ErrTelemetrySourceIdentityPending`、`scan_pending` / `source_identity_pending`；会话 source/watch 路径以同票读法判定可用。恢复与 CAS 依据 5 已认证计划中的 `(旧路径, 旧身份)` 撤回清单与旧基底核对删除：同票原路径被另一身份复用时，私库在该 `(ticket, attempt, source_path)` 只持久化一条替换 upsert 并遮蔽旧身份，不得要求每个逻辑删除都有独立 `op=delete` 行；仅凭同名路径不能授权保留文档 ID（来源：Task 5 CJSI-SI-R13-F5）。基线 watch 的取消/释放不回退。拥有 scanruntime、desktop、watch、store 和 CLI 中本锚的 `cjsi_publication_test.go`，独立故障屏障断言不以墙钟阈值判断锁。

```sh
scripts/run-go-test.sh ./internal/scanruntime ./internal/desktop ./internal/watch ./internal/store ./cmd/agentdeck -run '^TestCJSI(PublicationLifecycle|SlotDurability|WatchResume|ReaderConsistency)$'
```

### portable-state-recovery

**依赖与所有权。** 2+4+5+8。独占 `internal/backup/`、`internal/doctor/` 中本题的数据库健康路径及 `internal/session/doctor.go`。接既有 `Create/Restore`、`validateSessionSnapshot`、`BackupSQLiteFile`、`CheckHealth`。store 物理打开、迁移与新路径由 2 提供，8 提供同票视图/锁与共享 CLI 接线；3 的 Diagnose 不由本 owner 改写。

**交付与验证（L3）。** 在两个库中间状态或 slot 未 seal 时只导出上一安全代或现有错误；保留已确认的收据/覆盖/ordinal，清除候选、被撤正文、旧尾和缓存。Restore 改绑 epochs 与 view，不以旧票复活；校验空目标、旧 archive 迁移与实际私有围栏。doctor 区分合法 pending、损坏激活 seal、缺库和实际缺源，不打印原文。拥有 `internal/backup/cjsi_portable_state_test.go`、`internal/doctor/cjsi_health_test.go`、`internal/session/cjsi_doctor_test.go`。

```sh
scripts/run-go-test.sh ./internal/backup ./internal/doctor ./internal/session -run '^TestCJSI(PortableState|Health|IndexHealth)$'
```

### source-issue-acceptance

**依赖与所有权。** 1–9。只拥有 `cmd/agentdeck/cjsi_source_issue_acceptance_test.go` 及专用合成 fixture/harness 文件；使用已有 CLI/test helper。真实缺陷修复交回上游对应代码 owner，新的最终差分重新核对受影响证据。此锚独占旧/新二进制跨入口证据、最终 A1–A7 汇总及综合候选 L3；主控负责正式证据和提交动作。

**交付与验证（L3）。** 按下节逐行验收。各包命令是范围内快速门，最终一次完整 Go 回归、相关 race、vet 和二进制构建；精确复用当前候选已有结果。若状态/CAS 受平台差异影响，在主控目标支持平台运行对应 fixture；交叉编译只说明构建，不能背书运行时恢复。实际 rollout、native 安装与 App 交互不在验收内。

## 需求、回归与发布验收映射

表中数字指 Tasks 矩阵锚点。10 的综合 harness 运行正常入口，表中其他 owner 提供可以定位失败的包级断言。

| 验收 | 主要锚点 | 不可省略的观察 |
| --- | --- | --- |
| A1 来源与关系 | 1、3、4、5、10 | P/C/S/G 合成原生头与两种父字段，确认直接 parent 与 root/fork 不混淆。活跃/归档、规范命名、reverted、移动和冲突下，真实 index/search/show 与 usage/model/marker 一致。旧冲突文档不可公开，独立 P 保持。 |
| A2 继承与右锚 | 1、3、5、8、10 | 父历史后无第二个 C 头仍认可有证 C 请求；无锚最终回复延后、有锚才显示。重用 turn/call、未证阶段、父源缺失、半行、迟到矛盾、重启回撤。已证调用保持一次，不给未知模型猜值。 |
| A3 对账 | 3、7、10 | 原生双格式及副本 P=120/C=35/All=155、两笔，金额 140/90/230。核对路由、时段、模型及缓存/reasoning 计量。 |
| A4 新旧镜像 | 1、3、7、10 | 旧 20/R=10 按可能、包含、独立分别为 10/1、20/1、30/2；同 ID 矛盾撤到 0 确认，同数不同 ID 算两次。反序、分批、追加与重启；缺主项对显零、坏数、缺价、缺模型和累计重置。 |
| A4b 独立区间维度 | 3、7、8、10 | 会话 summary 20/调用 10，Codex/All 20，跨小时不分摊，未知模型/provider 不造行，份额 50%，完整金额 null/已知 10/20；真实 CLI 及桌面上层/期间/紧凑反馈。只删除区间也使原范围缓存失效。 |
| A5 整线程 OTel | 2、3、4、5、8、10 | 旧真触发器基线、新 schema 无触发器、未开票及未证头无撤回；7/11/13→7/17/13，再按证据为 C=5、11 或无可确认值，核对原键、时间、价和 marker。两种到达顺序、重启、S 独立结算/受阻、真实旧歧义与无恢复收据。 |
| A6a 旧新交接 | 2、3、8、10 | 真 P=40 独立；定位+可重放和不可重放的安全小计分别 50/60，补证 60/70；无双公开侧，测试预算/取消/改写/并发 CAS。 |
| A6b 金额和绑定 | 3、6、7、10 | 等价历史证据下升级与冷建对拍；范围内旧本地镜像给原 exact，继承、冲突和范围失效不可传。按纯重归、去重、漏采、修复路由/价分别对账，验证原时间和 multiplier。 |
| A6c 多代、覆盖与备份 | 2、4、5、6、7、8、9、10 | 分别停在准备、session seal、slot 已持久但未 seal、seal 后未开、core rollback、core 开放尚无公用镜像。重启后在静止向量核对 §7.6 每个既有读面及警告；安全旧版、无安全旧版、只有 All 可独立读三路。验证 cache invalidation、独立写入、单域不打开索引、持有→组合续作；只导出一代且无候选，Restore 新 epochs 和同票详情，doctor 区分合法 pending 与损坏。 |
| A6c / Round 2 F1 | 5、8、9、10 | 旧 ABC IDs 1/2/3；插入 X ID 4 源位置 2。冷建、增量、compact、备份/Restore 均为 AXBC、total=4，limit=2 页为 AX/BC/空，检查 has_more；append D 为 AXBCD。覆盖 CLI show/--all/--activity、viewer 和 `ShowWithActivity`，变更插在 count/page 间不混票。FTS 搜索仍用原稳定 ID tie-break，保护先于选定副本。 |
| A6c / Round 2 F2 | 8、10 | 文件 fsync 与目录 fsync 各自可控停住，未放行时第二连接能 `BEGIN IMMEDIATE` 并提交无依赖写，读者返回合法旧票/已有待处理信息。两处失败均无 durable seal/换侧；覆盖 rename 后目录同步失败、晚结果、变更 revision 的 CAS 拒绝和三处崩溃。完整链同一差分一次开放，旧读者不拼新代。 |
| A6d 真实缺源 | 2、3、4、5、8、9、10 | 已登记明文消失、不可读和仅压缩应保全范围内安全旧数据与未恢复提示；存在合法替代才正常切换和清理。从未登记纯压缩不产生条目、假数字或 OTel 接管。观察实际打开调用证明未读压缩，且 purge/exclude 不通过旧缓存或备份复活。 |
| A7a 用户绑定与排除 | 5、6、8、9、10 | 按 08:00 父、09:00/09:01 子头、09:04 执行、09:05 OTel 断言 birth；同/有证不同/无证不同，真实 classify list/show/Work/Auxiliary/All；并发 clear、不改原 revision。ID/path/project/client 及副本选择只影响原范围。 |
| A7b 白名单与负样本 | 1、3、5、6、7、8、9、10 | 多类禁止合成哨兵检查两个库既有/新增表、FTS shadow、候选、半行补齐前后、活动、分类、游标、派生/Widget 合同、备份和日志。允许文本只在有效未排除文档与原合法出口出现，错误和 trace 不打印哨兵。 |

### 综合测试命令与隔离

测试构造 schema35 已存 core/会话双库、可见旧安全查询、合成 OTel 原收据及固定价格/时区。测试函数创建独立临时 HOME、XDG 配置/数据/缓存、`--state-dir` 及 socket 路径，严格覆盖所有被测 CLI 子进程（包括旧二进制），以合成活跃/归档路径填充。使用测试自己的临时日志、合成凭据和输入；不继承实际 Codex/Claude 配置或用户 state。故障路径通过握手屏障（函数注入、跨进程协调和关闭/终止仅本 fixture 子进程）证实操作真到边界，再重开构造好的状态。禁止利用休眠或耗时阈值替代提交顺序；文件/目录同步失败各查 seal 和读者结果。只清本次创建的根。

```sh
# 在已同步且待验的 feature worktree 执行；只读基线 Git 树，不切换 checkout。
set -euo pipefail
cjsi_tmp="$(mktemp -d /private/tmp/agentdeck-cjsi-l3.XXXXXX)"
cjsi_old=4ac56db32e13c4e925b648c620ba76fc588a7781
mkdir -p "$cjsi_tmp/old-src"
git archive --format=tar "$cjsi_old" | tar -xf - -C "$cjsi_tmp/old-src"
export GOCACHE=/private/tmp/agent-deck-go-build
(
  cd "$cjsi_tmp/old-src"
  GOWORK=off go build -mod=vendor -trimpath -o "$cjsi_tmp/agentdeck-old" ./cmd/agentdeck
)
GOWORK=off go build -mod=vendor -trimpath -o "$cjsi_tmp/agentdeck-candidate" ./cmd/agentdeck
shasum -a 256 "$cjsi_tmp/agentdeck-old" "$cjsi_tmp/agentdeck-candidate"
CJSI_OLD_BINARY="$cjsi_tmp/agentdeck-old" \
CJSI_CANDIDATE_BINARY="$cjsi_tmp/agentdeck-candidate" \
AGENTDECK_GO_TEST_LOG="$cjsi_tmp/acceptance.log" \
scripts/run-go-test.sh ./cmd/agentdeck -run '^TestCJSISourceIssueAcceptance$'
AGENTDECK_GO_TEST_LOG="$cjsi_tmp/full.log" scripts/run-go-test.sh ./...
AGENTDECK_GO_TEST_LOG="$cjsi_tmp/race.log" scripts/run-go-test.sh -race \
  ./internal/ingest ./internal/activity ./internal/store ./internal/usage ./internal/session \
  ./internal/sessionorigin ./internal/telemetry ./internal/desktop ./internal/scanruntime \
  ./internal/watch ./internal/backup ./internal/doctor ./cmd/agentdeck \
  -run '^TestCJSI'
make vet
make check-whitespace
git diff --check
```

`TestCJSISourceIssueAcceptance` 的外部二进制子矩阵由 10 声明，综合命令必须看到实际子测试通过，尤其是下列名称及 A1–A7 对应 fixture。普通 `./...` 可跳过需要额外二进制的子矩阵；该全套通过不能替代带环境参数的运行。若目标分配使待兼容的旧生产二进制基线不同，主控另按其确切已发布/准入提交重复同一矩阵，并保留 schema35 基线结果，不把新版本模拟回退当作旧二进制。

| 综合子测试 | 实际边界与读取 |
| --- | --- |
| `OldBinaryFence` | 同一个真实旧构建在旧 schema35 临时根成功 list/search/show/rebuild，再对已升级 fixture 调 `session list/search/show/exclude/rebuild/purge-index`、`watch`、本地遥测投影入口、`doctor`、desktop serving/helper、`backup create/restore`、`state migrate`。逐入口断言旧定向错误/拒绝或已有 unavailable，不能读写实际私有布局。检查真旧迁移/重建没有越过 WAL pragma，purge 拒绝在 remover 前，Restore 校验 ahead 与非空目标。核对围栏文件、公开账、候选和私有索引哈希与无写副作用；比较受管迁移中断与恢复。 |
| `DetailOrder` / `SlotDurability` | 运行上表 Round 2 F1、F2；核对真实页结果、故障屏障、独立事务、候选/读票与竞态。验证桌面既有 payload/Widget 投影契约，勿使用运行中的 App。 |
| `ReplayAndHistoricalAbsence` / `PortableRecoveryAndPrivacy` | A6a–A6d 与 A7，含真正不存在和访问失败、从未登记的压缩、同代备份和明文不接触探针。升级/冷建数据相同的部分精确对拍，历史资料不足分支报原不完整状态。 |
| `IdentityAndAccounting` / `OTelOwnership` | 覆盖 A1–A5、实际 invocation/marker/金额/模型/来源状态，并用同一 fixture 检查在组成回填途中旧读面。 |

## 正式认领、证据和交付顺序

1. 主控集成并核对本文 blob，按 `bash scripts/check-topic-docs.sh codex-jsonl-session-identity`、`make check-whitespace` 和主题限定 `git diff --check` 检查正式候选，独立评审通过后才登记批准 Task、实际依赖和写者。基线同步与编号分配按上文分别落证；没有前置问题时直接推进，无额外确认流程。
2. 每个 Task 认领前主控核对 live issue 范围、前置状态、工作区路径/分支、唯一写者与接口冻结。每个完成检查点绑定实际 HEAD 和受影响源码、测试、批准契约、依赖/工具链与候选指纹，并登记命令/exit、关键观察、边界值以及旧/新二进制摘要。用完成证据记录和独立评审结论区分包级通过和同一 issue 已通过。可安全重用同一内容/环境证据；变化仅重验有影响的范围。不要用矩阵勾选或协调状态代替精确候选证据。
3. 完成 C4、综合 L3、对应评审与修复闭环后，原 issue 的已批准实现按仓库审查/本地提交规则进入 `awaiting_commit`，主控核对单一 issue 发布范围；用户文档 Task 的独立提交约定照旧。按原授权 commit、push 到 origin、创建 PR；push 和 PR 适用 Actions 均通过后再请求 Codex review，处理所有真实范围内发现并核对最终 PR head。文档阶段不判定该 Bug 已修复；不拆为无独立交付意义的多 PR。
