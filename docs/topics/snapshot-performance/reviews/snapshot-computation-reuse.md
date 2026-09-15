---
status: active
topic: snapshot-performance
subject: snapshot-computation-reuse
---

## Round 1 — 2026-09-12

**Checklist: 41/54 complete**<br>
**Incomplete: TRACE-1 — 阻断 finding 后未继续枚举全部验收项；TRACE-2 — 未继续审读全部 31 个候选文件；TRACE-3 — 未继续走查全部首次使用、失败、恢复与重复场景；TRACE-6 — 未继续追踪全部关键入口；TRACE-7 — 未继续核对全部运行时注册；TRACE-8 — 未继续覆盖全部边界与状态转换；TRACE-9 — 未继续审读全部异步路径；DESIGN-9 — 未完成全量 subtractive pass；VERIFY-1 — 未完成全任务测试决策账本；VERIFY-5 — 未逐项审查全部受影响测试；VERIFY-10 — 未完成所有改动文档的分类；VERIFY-12 — 未完成全部文档的冗余检查；VERIFY-14 — 未核完全部 API/配置引用。影响：本轮已有可证伪的任务级阻断缺陷，按项目规则停止扩大验证；后续复评必须覆盖这些未完成项。下一动作：修复 SCR-R1-F1 后对最终候选执行完整复评。**

## 📋 Snapshot computation reuse 实现评审

📊 总体评分：4/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SCR-R1-F1 — 高：交付只实现了部分 generation 协议，遗漏 session epoch，并违反 core epoch/revision 契约。**

位置：`docs/topics/snapshot-performance/architecture.md:755-785`、
`internal/store/migrations.go:221-253`、`internal/store/derived_generation.go:12-49`、
`internal/store/store.go:49-130`。

- 行为风险：批准架构要求 core 使用随机 database epoch、dirty 期间合并 revision
  失效，并给 session index 自有 epoch 作为 ingestion-checkpoint identity。候选在
  migration v24 中把 core epoch 固定为 `1`，每个 row trigger 都无条件执行
  `revision=revision+1,dirty=1`，而 `sessions.sqlite3` 的创建、只读打开和 rebuild
  schema 没有任何 epoch/generation 元数据。session index 的重建身份因此无法按
  已批准契约表达；core 端也没有按 dirty 边界建立稳定 generation，并在大批量
  导入时为每一行追加 generation 写放大。
- 证据：架构 755–777 行明确规定三项行为，779–785 行把 creation、restore、
  rebuild 纳入 mint/invalidate 边界；候选 SQL 与 session schema 直接反证这些
  要求。现有 `TestDerivedSnapshotGenerationTracksRelevantWrites` 只证明一次 core
  写入会递增，`TestDerivedSnapshotGenerationInstallsEveryAllowlistedWriterTrigger`
  只证明 core 表上存在三个 trigger，没有覆盖随机 epoch、dirty 合并、session
  epoch 或 restore/rebuild。原 CEv1 generation evidence 的 check 也只列出 v24
  trigger、provider/price/source 写入和 full Go 通过，不能证明遗漏的契约。

💡 修复：在现有 store/session schema owner 中完整实现一个 generation 协议：
为新 core database 生成不可预测 epoch，并在 owned restore/rebuild 路径 mint 或
invalidate；让首次 clean→dirty mutation 才递增 revision，后续 dirty writes 合并，
同时保留并发 build 的 compare-and-clean 安全性；为 session index 增加独立 epoch，
把它接入 source/checkpoint identity 及 creation/rebuild/restore。增加确定性的迁移、
dirty coalescing、并发 publication、session rebuild 与 restore/cache-refusal 回归。
SQLite 官方文档确认 trigger 是逐行触发且可用 `WHEN` 限制执行：
[CREATE TRIGGER](https://www.sqlite.org/lang_createtrigger.html)；Go 标准库提供安全
随机源：[crypto/rand](https://pkg.go.dev/crypto/rand)。允许等价实现，但必须满足
同一批准契约。Disposition：OPEN。

### 🟡 改进建议 — 推荐

无额外建议。得到任务级阻断反证后，按项目规则停止扩大验证。

### 🟢 优点

- 派生缓存使用独立 DTO，显式保留 `PresentationReport.Summary`，没有缓存完整
  desktop snapshot、session text、provider health 或凭据。
- 缓存读取限制为 8 MiB，拒绝 symlink/非私有文件，publisher 使用独立锁、同目录
  临时文件、sync 和原子 rename；snapshot miss 保持只读回退。
- 代表性性能报告保留了 over-target cold/unchanged 样本，没有把未达目标的数据
  标成成功；这部分证据不掩盖 generation 契约缺陷。

### 📝 总结

Reviewer：Codex 主代理。Method：`development-workflow` REVIEW +
`ln-12-delivery-reviewer`，Blue-only 单代理定向评审；未使用 subagent，不声称冷
上下文独立性。Scope：Task 2 的 generation/cache owner、worker publisher、
request-local aggregation、性能 harness、fixtures 和 CLI schema contracts。
发现 SCR-R1-F1 后停止扩大验证；未列出的候选路径不能据此推断已通过。

HEAD：`7ea8dc3eaf9880babf8eb15c94308b0bec8703cb`。
Reviewed content fingerprint：
`9247907c3f59477b42a313dd0369012c8d5b9157b1b66bbdd6646a37cae0fa30`。
配方：SHA-256，首行为 `head=<HEAD>`，随后为全部 Task 2-owned 候选文件按路径
排序的 `<git hash-object --no-filters>  <path>`，每行含 LF；`tasks.md` 与本评审
记录不在 Task 2 实现指纹内。本轮重算得到相同 fingerprint。

| Path | Blob |
| --- | --- |
| cmd/agentdeck/desktop.go | 157d440cca5d0bb67f389519edd3dd0e08b2b104 |
| cmd/agentdeck/main_test.go | 861b6dec141bde0b7a7d594412e35173ac209844 |
| cmd/agentdeck/scanruntime_test_helper_test.go | e6ee8fd58246cce55705ac28aa167eb5761461ca |
| cmd/agentdeck/schema_signal_acceptance_test.go | 3b255ec8f3729f07a3afb7ff7cc3bdf6b615fe1a |
| cmd/agentdeck/session_progress_test.go | 2304baf16c3088eea9f04ea67d0b03393f02c1ce |
| cmd/agentdeck/snapshot_performance_contract_test.go | 697a89ac0636aded08397f1cfa6b452becaa9899 |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | c9708c0708c7276c5b34c37be42dced7f884b63a |
| cmd/agentdeck/testdata/snapshot-performance/README.md | d02afa8f8425cae24d216a32dfdc68229a0dfa8a |
| cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json | 54f2c835475bf1059a34438822f086becc9edb6c |
| desktop/fixtures/v1/snapshot-complete.json | 3378996bd68a509757e8ce20afed40df7f622893 |
| desktop/fixtures/v1/snapshot-empty-client.json | f0715fe19d910ba9598c3391c40ac8526529d82b |
| desktop/fixtures/v1/snapshot-schema-ahead.json | 61dddb5b3390d88ae217331a2ca4e829862943e0 |
| docs/specs/cli-design.md | 22bad4b0b0ea34319d28aa25ccaa3509018f5ee8 |
| docs/specs/cli-manual.md | 2f2f26b9618c89b30057bacba0373c03925893b0 |
| internal/desktop/derived_cache.go | e6f9815bf47c960de2b152df07b00a9fb7af364b |
| internal/desktop/derived_cache_test.go | b08603c4b27e30d579ad4a13cf3eed0b96d5c12f |
| internal/desktop/desktop.go | 2a0a09fd2fb5bfebfe892fad9126abe37fe2aee3 |
| internal/desktop/stage_profile_test.go | 2bea58b85e0a90b9962d1d38e70f6f8439856edc |
| internal/scanruntime/scanruntime.go | 380e3f449a0ae2cd5bab26fa01dca36e72bc1acb |
| internal/scanruntime/scanruntime_test.go | 7da5f871311658c221e469a4a37d0826f2e5bb66 |
| internal/store/derived_generation.go | 63cc08bc9a253bb7e20891f7914f2d15a5d1a31a |
| internal/store/derived_generation_test.go | af045dbdf79f0c49681a3c8ba821db9d0b86c4a0 |
| internal/store/migrations.go | 62b867cb7b6615f619ce3b566a2d34c7b73d6932 |
| internal/store/store.go | f61b60c01164ea3ee1f519fee9680941778b02af |
| internal/usage/presentation.go | 7e6801dde6d2e128a3aa54478923938473c8fc57 |
| internal/usage/presentation_cost_reuse_test.go | a3cac171a0d058c5effe7192c75a70852e83c6b0 |
| internal/usage/signals.go | a3fe2eb6339cbb55488313242605091cdf802aee |
| internal/usage/signals_batch.go | 030ea06cc7990a2ed8e804629f051e17f07bfd24 |
| internal/usage/signals_batch_test.go | 6e65828e5153a6be386283cf1d61fbe17025fa88 |
| internal/usage/signals_report.go | 83cca4e2360765fc8057e19db9fd5fc50ac246e2 |
| internal/usage/usage.go | 5993c3f55bd631d19fc26f440866a9f53c8e140c |

Evidence：复用同一 ContentState 的 L3/full Go/race/vet/build、派生缓存、只读回退、
等价性及 worker-accounted performance 证据；本轮没有因阶段变化重跑这些检查。
源码对照证明 SCR-R1-F1，SQLite/Go 官方文档仅支持修复机制，不用于发明本地要求。
CEv1 新增 fail evidence
`urn:ce:agent-deck:evidence:snapshot-performance-snapshot-computation-reuse:generation-protocol-review-fail:9247907c3f59`
和 `satisfies`、`observed_at`、`supersedes` 三条关系；预检 3/3 `ok`，精确读回
节点及端点 1/3。固定 `gate-status.cypher` 重查为 `FAILED`；五项未受影响证据继续
复用，generation-and-writer-invalidation 使用本轮 applicable fail evidence。

测试动作：KEEP 现有 cache DTO/publication/read-only/equivalence/performance tests；
UPDATE core generation test，使其覆盖 clean→dirty 仅递增一次及 dirty 写合并；
ADD 随机 core epoch、session epoch、creation/rebuild/restore 和并发 publication
回归。净测试组合待修复后复评，不在 REVIEW 中改测试。

文档动作：KEEP `docs/specs/cli-design.md`、`docs/specs/cli-manual.md` 的 schema 24
同步；UPDATE 仅在修复产生新的稳定 CLI/schema 行为时需要。无 UI 基线变更；Task 3
继续拥有原生体验和最终 20-sample 目标验收。

完成门禁：FAILED。
Task：`ad-sp-snapshot-computation-reuse-dev`。
WorkUnit：`urn:ce:agent-deck:work-unit:snapshot-performance-snapshot-computation-reuse`。
Workspace：`agent-deck.snapshot-performance` / `feature/snapshot-performance`。
Task Review 保持未勾选；返回同一任务修复，不创建新任务。未提交或推送。

下一步指令：修复：snapshot-performance / reviews/snapshot-computation-reuse.md / SCR-R1-F1
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## SCR-R1-F1 修复 — 2026-09-13

- 修复者：Codex；仅处理 SCR-R1-F1，未执行复评、提交或推送。
- 处置：SCR-R1-F1 -> repaired in candidate，等待独立复评；Round 1 FAIL 与
  OPEN disposition 保留。Round 1 未完成的 TRACE/DESIGN/VERIFY 项仍由复评覆盖。
- Core generation：migration v24 通过 `crypto/rand` 生成正整数 database epoch，
  不再固定为 `1`。所有 allowlisted row trigger 增加 `WHEN dirty=0`，只有首次
  clean→dirty mutation 递增 revision，后续 dirty writes 合并。cache builder 在
  读取前原子递增 revision 并清除 dirty；计算后和写文件后均比较
  epoch/revision/dirty，任何并发 mutation 都拒绝或移除候选 cache。
- Session generation：session schema owner 新增独立 `session_index_generation`
  epoch。首次创建生成 epoch，正常 reopen 保持；成功 rebuild 在同一事务内 mint，
  失败 rebuild 回滚并保留旧 epoch。session watch checkpoint 使用
  `v1:<session-epoch>:<source-fingerprint>`，并在计算 fingerprint 前后确认 epoch
  未变化。purge 后的新 index 自然获得新 identity。
- Restore：portable restore 在全部 core 写入结束后 mint 新 core epoch 并保持
  dirty；包含 session index 时也 mint 新 session epoch。restore-owned SQLite
  sidecar 均纳入既有私有文件与失败回滚边界。旧 cache 因 epoch/database identity
  不匹配而拒绝。
- 回归：focused suite 覆盖确定性 core epoch migration、clean→dirty coalescing、
  下一次 build revision、全部 allowlisted trigger、并发 publication、minted-epoch
  cache refusal、session create/reopen/rebuild success/rebuild rollback、restore 双
  epoch 与 checkpoint identity；日志 SHA-256
  `530fea93e41e7b38bbf63e76601443db5f01322a052cf0e13029b240af668023`。
- L3：`scripts/run-go-test.sh ./...` 通过，日志 SHA-256
  `96e3b5c7e3506534caa3266d55e41cea9a20cddab70775195dd35fd680c302bf`；
  store/session/desktop/backup/cmd race 通过，日志 SHA-256
  `b1c9d2cd1982dfdeb084f85d1974e17752d64f75e72cc12e499cfe58110333fa`；
  `make vet` 与 darwin arm64/amd64 `make build-all` 通过。
- 代表测量：相同 2,200 文件 / 2,343,100,564 bytes 隔离 corpus 的 cold、full
  recomputation、unchanged 三个 sample 均 complete，snapshot 与 logical-row digest
  分别保持 `db82eb84d1da0260204473d23fc4cfad4905ff2f54fb8fa2eab8dd0ff873f10e`
  和 `b1e1be6961abe5e8924774b8b5f40ad94584e68e934f5856b4217b7322560507`。
  wall 为 59.054 s / 6.181 s / 5.791 s；仅 full recomputation 达 10 s 目标。
  cold 与 unchanged 仍未达标并保留给 Task 3 最终处置。完整私有报告 SHA-256
  `5f6af73fcd43b270afa1fef87b72c50c533544e008879bb2acaa5b0412c61b89`。
- 内容：HEAD `7ea8dc3eaf9880babf8eb15c94308b0bec8703cb`；修复候选指纹
  `a9627671b780b12bc624d1677b0dd67f1459f5e8fabdc3f5a756a090ee9f4be0`。
  配方沿用 Round 1，但最终 Task 2-owned manifest 扩展为 36 文件；新增边界为
  `cmd/agentdeck/main.go`、`internal/backup/{backup.go,backup_test.go}` 与
  `internal/session/{session.go,session_test.go}`。`tasks.md` 与本评审记录仍排除。
- CEv1：新增修复 ContentState 与六项 target-bound pass evidence，共 7 nodes；
  12 条 `satisfies` / `observed_at` 关系预检全部 `ok` 并成功写入。固定
  `gate-status.cypher` 对修复状态返回 VERIFIED；6/6 required criteria 有
  applicable、target-matching、non-malformed pass evidence，missing/failed/blocked/
  unresolved 均为空。旧 generation fail evidence 只匹配旧指纹，未被改写。

完成门禁：VERIFIED。
Task：`ad-sp-snapshot-computation-reuse-dev`，保持 `in_progress` 等待复评。
Workspace：`agent-deck.snapshot-performance` / `feature/snapshot-performance`。

## Round 2 — 2026-09-13

**Checklist: 54/54 complete**<br>
**Incomplete: None**

## 📋 Snapshot computation reuse 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。SCR-R1-F1 已关闭；没有回归或新阻断 finding。

### 🟡 改进建议 — 推荐

无。cold import 与 unchanged refresh 的原始目标仍未达到，但批准的 Task 2
边界要求如实报告而不伪称通过，最终 20-sample 性能验收和目标处置属于 Task 3，
不是本轮可延后的 Task 2 finding。

### 🟢 优点

- `SCR-R1-F1 -> CLOSED`：core migration 使用 `crypto/rand` 生成正 epoch；
  allowlisted row trigger 仅在 clean→dirty 时递增 revision，dirty 写入合并。
- cache builder 先建立新的 clean build revision，计算后和原子写入后均复核
  epoch/revision/dirty；并发 mutation 会拒绝或移除候选 cache。
- session index 拥有独立 epoch，正常 reopen 保持 identity，成功 rebuild 与 portable
  restore mint 新 identity，失败 rebuild 在事务回滚后保留原 epoch。
- 所有 `watch.fingerprint.session` 生产写入均通过 epoch-bound
  `sessionCheckpointFingerprint`；旧的 raw fingerprint 会自然触发一次刷新。
- 缓存 DTO、8 MiB/private/atomic publication、只读 snapshot miss、session 独立可用性、
  request-local aggregation 和性能报告边界保持完整，没有恢复旧旁路。

### 📝 总结

Finding disposition：`SCR-R1-F1 -> CLOSED`。批准架构要求的随机 core epoch、
dirty coalescing、generation-safe publication、session epoch、checkpoint identity、
rebuild 和 restore minting 都有当前源码与确定性回归证据。未发现新 finding。

Reviewer：Codex 主代理。Method：`development-workflow` REREVIEW +
`ln-12-delivery-reviewer` selective follow-up，Blue-only 单代理；subagent rounds 0，
不声称冷上下文独立性。Scope：Round 1 SCR-R1-F1、其 12-file repair delta，以及
Round 1 未完成的 TRACE/DESIGN/VERIFY 项；产品代码、测试和配置保持只读。

HEAD：`7ea8dc3eaf9880babf8eb15c94308b0bec8703cb`。
Reviewed content fingerprint：
`a9627671b780b12bc624d1677b0dd67f1459f5e8fabdc3f5a756a090ee9f4be0`。
配方沿用 Round 1：首行为 `head=<HEAD>`，随后为最终 36-file Task 2-owned
manifest 按路径排序的 `<git hash-object --no-filters>  <path>`，每行含 LF；
`tasks.md` 与本评审记录排除。复评重算得到同一 fingerprint。

| Path | Blob |
| --- | --- |
| cmd/agentdeck/desktop.go | 09c257e656f8be0dccaf027a3293adb0132e1329 |
| cmd/agentdeck/main.go | 0ecde7dbc797e73e4b1eca82a00b650d8e861ce5 |
| cmd/agentdeck/main_test.go | b7bab4eac70a68c59c879613e5f36d8a898db44a |
| cmd/agentdeck/scanruntime_test_helper_test.go | e6ee8fd58246cce55705ac28aa167eb5761461ca |
| cmd/agentdeck/schema_signal_acceptance_test.go | 3b255ec8f3729f07a3afb7ff7cc3bdf6b615fe1a |
| cmd/agentdeck/session_progress_test.go | 2304baf16c3088eea9f04ea67d0b03393f02c1ce |
| cmd/agentdeck/snapshot_performance_contract_test.go | e7456015bd79f513cfbf11d2a121cf198b5e192d |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | c9708c0708c7276c5b34c37be42dced7f884b63a |
| cmd/agentdeck/testdata/snapshot-performance/README.md | d02afa8f8425cae24d216a32dfdc68229a0dfa8a |
| cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json | 54f2c835475bf1059a34438822f086becc9edb6c |
| desktop/fixtures/v1/snapshot-complete.json | 3378996bd68a509757e8ce20afed40df7f622893 |
| desktop/fixtures/v1/snapshot-empty-client.json | f0715fe19d910ba9598c3391c40ac8526529d82b |
| desktop/fixtures/v1/snapshot-schema-ahead.json | 61dddb5b3390d88ae217331a2ca4e829862943e0 |
| docs/specs/cli-design.md | 22bad4b0b0ea34319d28aa25ccaa3509018f5ee8 |
| docs/specs/cli-manual.md | 2f2f26b9618c89b30057bacba0373c03925893b0 |
| internal/backup/backup.go | d81783d198fe980bc28b49aa41ca27b9f76c24df |
| internal/backup/backup_test.go | 565a11455d85d30164e26848c53923e91179883c |
| internal/desktop/derived_cache.go | b1d1a0c479561a63dd1e5172860d825899191d72 |
| internal/desktop/derived_cache_test.go | 5e98ecca084e1455daa76091ba781330cd02e14d |
| internal/desktop/desktop.go | 2a0a09fd2fb5bfebfe892fad9126abe37fe2aee3 |
| internal/desktop/stage_profile_test.go | 2bea58b85e0a90b9962d1d38e70f6f8439856edc |
| internal/scanruntime/scanruntime.go | 380e3f449a0ae2cd5bab26fa01dca36e72bc1acb |
| internal/scanruntime/scanruntime_test.go | 7da5f871311658c221e469a4a37d0826f2e5bb66 |
| internal/session/session.go | 871ee62407aa9d560337aed48cf8acfe56dee425 |
| internal/session/session_test.go | e37b14faac0f5acdfcdc1ed66b96d8e59349d10a |
| internal/store/derived_generation.go | 6c0e0f38a1ae56eca5e03159c13e53f0036060f0 |
| internal/store/derived_generation_test.go | e7fe109cd454167ca3bd90851207a69d7e097047 |
| internal/store/migrations.go | 09edf34dfdae2e242aff1c079695e2089d9ea755 |
| internal/store/store.go | 4728178ef4c17ccfea0a424ff40ef41c2f36187f |
| internal/usage/presentation.go | 7e6801dde6d2e128a3aa54478923938473c8fc57 |
| internal/usage/presentation_cost_reuse_test.go | a3cac171a0d058c5effe7192c75a70852e83c6b0 |
| internal/usage/signals.go | a3fe2eb6339cbb55488313242605091cdf802aee |
| internal/usage/signals_batch.go | 030ea06cc7990a2ed8e804629f051e17f07bfd24 |
| internal/usage/signals_batch_test.go | 6e65828e5153a6be386283cf1d61fbe17025fa88 |
| internal/usage/signals_report.go | 83cca4e2360765fc8057e19db9fd5fc50ac246e2 |
| internal/usage/usage.go | 5993c3f55bd631d19fc26f440866a9f53c8e140c |

Task/plan/acceptance：core generation、session identity、cache DTO/header/publication/
read validation、request-local computation reuse、worker integration、corruption/missing/
future state fallback和完整 worker-accounted report 均为 COMPLETE/PASS。没有授权的
UI 基线变化；Task 3 继续拥有最终原生体验与 20-sample 性能 acceptance。

Evidence：修复 focused log
`530fea93e41e7b38bbf63e76601443db5f01322a052cf0e13029b240af668023`、full Go log
`96e3b5c7e3506534caa3266d55e41cea9a20cddab70775195dd35fd680c302bf`、affected race log
`b1c9d2cd1982dfdeb084f85d1974e17752d64f75e72cc12e499cfe58110333fa`、performance
report `5f6af73fcd43b270afa1fef87b72c50c533544e008879bb2acaa5b0412c61b89`
均已在复评中按原路径重算一致。复用 exact-state full Go/race/vet/darwin builds/L0；
内容、依赖、toolchain 与相关环境未变化，不因阶段变化重跑。

测试动作：KEEP generation/session/checkpoint/restore/cache/concurrency/equivalence/
performance regressions；没有 ADD/UPDATE/MERGE/DELETE。文档动作：KEEP schema 24
CLI design/manual 与 Task 2 status；没有新的稳定契约需要补写。

完成门禁：VERIFIED。固定 `gate-status.cypher` 对上述 ContentState 返回 6/6
required criteria 全部有 applicable、target-matching、non-malformed pass evidence；
missing/failed/blocked/unresolved 均为空。

Task checkpoint：`ad-sp-snapshot-computation-reuse-dev`；ContentState
`a9627671b780b12bc624d1677b0dd67f1459f5e8fabdc3f5a756a090ee9f4be0`；门禁 VERIFIED。
提交建议：经单独授权后，将 36-file Task 2 manifest、本文件与 `tasks.md` 的 Task 2
状态作为一个逻辑提交；排除 Task 3 与全局 `docs/status.md`。
推送建议：经单独授权并验证提交对象、SSH 签名、完整 outgoing range 和实时远端状态后，
推送 `feature/snapshot-performance` 到 `origin/feature/snapshot-performance`；本地尚无该
remote-tracking ref，推送将创建远端分支。

下一步指令：开发：snapshot-performance / scan-experience-acceptance
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
