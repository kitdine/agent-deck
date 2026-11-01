---
status: active
topic: scan-performance
subject: otel-capture-and-reconciliation
---

# otel-capture-and-reconciliation 评审记录

## 实现交接 — 2026-10-29

- 实现方：Claude Code 主会话（`开发：scan-performance / otel-capture-and-reconciliation`）；工作区 `agent-deck.scan-performance` /
  `feature/scan-performance`；未提交。
- 候选：HEAD `27621aaced6e3bcf6fa80afba188efa242607bff`；候选指纹 `b1d64fc32c08726ddd8a8119ed354bc69c9844db928b38dab5358b14519af93a`（55 个路径）；CEv1 ContentState
  `scan-perf-otel-impl-20261029-state`，WorkUnit `scan-performance:otel-capture-and-reconciliation`（矩阵提交 `27621aa`，9 条 required 准则）
  与 9 条实现证据 `scan-perf-otel-impl-20261029-<criterion>`。
- 指纹配方：HEAD 差异与未跟踪路径并集，排除 `docs/`、`apps/macos/.build/`；按字节排序，将 `path:<git hash-object>`（删除为
  `path:deleted`）以 LF 连接、无末尾 LF，取 SHA-256。

### 实现范围对照（tasks.md 任务 7）

- 事件识别：事件名只取 `event.name` 属性（OTLP `eventName` 为 tracing target，不再使用）；白名单加入 `originator`；
  transport 由事件名判定；删除 producer instance + response id 的 canonical key。
- 存储（迁移 33，未发布，原地修订）：`telemetry_observations`（capture_only/pending/jsonl_owned/otel_owned/conflict/invalid/unmetered）、
  `telemetry_contributions`、`telemetry_sessions`、journal cursor、coverage；触发器 `telemetry_jsonl_retracts_otel` 在 JSONL 写入事务内撤销。
- 会话所有权：`SettleTelemetry` 只接受非 `otel:` 事件或 `usage_source_files` rollout 行为 JSONL 证据；settle 需 10 分钟且其后有
  complete inventory；缺失贡献在 rebuild 后补齐；conflict 不计入。
- 读取面：`internal/session` 会话索引投影 `otel:codex/<id>`（priority -1、空项目、无 documents）；文件 inventory 全集对账与
  usage orphan 清理豁免 `otel:`；会话排除约束投影；`session show`/查看器对 `otel:` 来源不做文件访问；扫描轮次与
  `session rebuild`/`watch` 之后结算并刷新投影。
- Codex 配置与联动：`internal/telemetry/codexconfig.go` 只读事实、征得同意后的受管段追加/原子替换/校验、只移除未改动段；
  listener 在 Memories off 时不启动、运行中变 off 即停（原因 `codex_memories_off`）；status 警告与 doctor `telemetry` 检查。
- CLI：`telemetry status` 字段按原型 `engineSamples`，新增 `codex_memories`/`codex_exporter`；`enable --write-codex-config`；
  `disable` 移除受管段；`codex_otel_conflict`/`codex_config_write_failed` exit 1、`codex_config_restore_incomplete` 警告 exit 0。
- App：Settings“实时用量”按原型文案与结构（同意区原位、写入/被拒/自配/移除/恢复不完整/用户自配保留）；随行 listener
  经 stdout `publication_needed` 触发刷新；Health 名称“部分用量尚未收录”。

### 支持表实验（隔离 CODEX_HOME 与 HOME、本地模拟模型服务；事实写在 Bug `ad-bug-codex-subagent-usage-attribution` 2026-10-29 评论）

| 组合 | 结论 |
| --- | --- |
| `codex_exec` / 0.165.0 / `codex_exec` / SSE | 入表：主请求只在 rollout；Memories 整理在无 rollout 的会话；计数与服务端一致 |
| 重试与失败（两次 500 后成功） | 失败尝试不产生用量事件；成功一次与 rollout 一致 |
| `codex-app-server` / 0.165.0 / `codex-tui` / SSE | 入表：形状同上 |
| TUI 会话标题生成 | 每个新会话一次独立请求、无 rollout，计为后台用量（不重复）；**Memories 关闭时同样发生**，见下 |
| 主会话压缩 | 本设置未触发独立压缩请求，未验证 |
| WebSocket、Desktop originator、guardian、Memories stage1 | 未验证，保持 `capture_only` |

### 验证

- Go：`scripts/run-go-test.sh -count=1 ./...` 27 个包通过；`go vet ./...` 干净；race：telemetry/session/store/doctor/agentdeck-perf
  与 agentdeck telemetry 测试、usage orphan 测试通过；`internal/scanruntime` 整包 race 通过（多包并发 `-count=2` 时
  `TestServingFirstBatchPublishesBeforeHistoryAndRetainsFullInventory` 曾一次超时，单独与整包重跑均通过，属负载敏感超时）。
- 变异确认：异步 ACK、冲突回写、inventory 豁免、orphan 豁免、配置冲突检查，各自移除后对应测试失败。
- Native：`scripts/test-macos-app.sh` 全量通过；Release 双架构构建通过且无 `PerfDriver` 符号。
- 真实 producer 端到端（隔离 HOME、本地模拟服务）：CLI 同意写入受管段 → Debug App 2 s 起 listener → 两次 `codex exec` →
  settle 后 Memories 整理（gpt-5.6-terra 1014/12）恰计入一次、会话 `otel:codex/<id>` 带 `codex-otel-background`；主请求只按 JSONL；
  CLI 今日 3078 tokens / 3 事件 / 3 会话，App 发布 `gpt-5.6-terra`、3078、3 会话一致；`disable` 移除受管段。
- 首轮端到端发现 listener 与首启迁移争用状态锁后退出且不重启：`telemetry serve` 改为重试打开 store（最长 2 分钟），
  监督者对 status 1 退出有限重启；`TestTelemetryListenerWaitsOutAHeldStateLock` 覆盖。

### 待评审关注与已知限制

- **设计前提偏差（需用户决定）**：TUI 会话标题生成在 Memories 关闭时也产生只在 OTel 的用量；按已评审联动规则，
  Memories 关闭时采集不可用、listener 不运行，这部分会漏计。实现按已评审设计交付，未自行改变联动。
- 跨主题依赖：已在 `ad-auxiliary-session-classification-origin-adapter-reducer` 登记 `codex-otel-background` 分类接口请求；
  在其采用前后台会话出现在工作视图（数值正确，会话数偏多）。
- 原型预览说明“原型配置示例”半句未带入原生文案（原型自身标注）。

## Round 1 — 2026-10-29

## 📋 Task 7 实现独立评审

📊 综合评分：6/10

✅ 评审结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；独立评审角色，未参与本候选实现，无子代理。
- Method：契约与源代码检查、精确候选身份核验、隔离 CLI/HTTP/SQLite 故障注入。
- Scope：Task 7 的接收与恢复、会话所有权/贡献、配置写入及 CLI 调用链。发现决定性反例后停止扩大验证；不宣称其余路径全部无缺陷。
- Reviewed state：HEAD `27621aaced6e3bcf6fa80afba188efa242607bff`；按实现交接配方独立重算 55 路径指纹
  `b1d64fc32c08726ddd8a8119ed354bc69c9844db928b38dab5358b14519af93a`，与 `scan-perf-otel-impl-20261029-state` 一致。
- WorkUnit：`scan-performance:otel-capture-and-reconciliation`；Beads：`ad-scan-performance-otel-capture-and-reconciliation`。

### 🔴 严重问题 — 必须修复

#### OTEL-R1-F1 — P1：截断 journal 尾行吞掉下一条已 ACK 的有效事件

- 位置：`internal/telemetry/journal.go:119-147`、`:228-242`；接收 ACK 在 `internal/telemetry/receiver.go:73-93`。
- 行为风险：崩溃或短写留下不含 LF 的半行后，Append 直接以 O_APPEND 拼接新 JSON；fsync 成功即返回 200。
  ReadFrom 将半行与新事件一起解析为 `journal_line_unreadable`，随后 cursor 跨过整行，已确认接收的事件无法恢复。
  若尾段已触及轮转条件，半行还会阻止 reader 到达后继段。现有 torn-line 测试只断言半行暂不消费，未覆盖恢复后的追加。
- 证据：同一候选构建 `go build -mod=vendor -o "$OUT/agentdeck" ./cmd/agentdeck`；临时 HOME 内写
  `[features] memories = true`，独立 state-dir 执行 `telemetry enable --port <空闲 loopback 端口>`。
  在 journal 第一段写入无末尾 LF 的 `{"producer":"torn`，再启动 `telemetry serve`。发送一个合法 OTLP JSON
  `codex_exec/0.165.0/codex_exec/sse` 的 response.completed（合成会话、input=100/output=5，使用此隔离 root 的 token）。
  实际 HTTP **200**；SQLite 只有 `(invalid, journal_line_unreadable, thread_id='')`；合成会话 observation 数 **0**；
  cursor 为 `(segment=1, offset=499)`，等于 journal 文件 499 字节。listener 已正常终止。全程没有真实 producer、日志或用户配置访问。
- 契约：architecture.md“Journal 与保留”要求 durable ACK、崩溃恢复及已确认事件可重放。
- 💡 修复建议：在 journal 独占锁内处理残缺尾部，保证新批次从有效记录边界开始；区分未完成尾写与已持久完整行，
  保留完整记录与 cursor 一致性。补充“截断尾行→重启→追加→ACK→replay”及段边界恢复回归。
- Disposition：OPEN；修复范围为本任务接收/恢复路径；对应 `durable-ingress`。

#### OTEL-R1-F2 — P1：配置写入失败留下未受管理的变更，却报告 Nothing was changed

- 位置：`internal/telemetry/codexconfig.go:163-175`；`cmd/agentdeck/telemetry.go:253-264`、`:280-284`。
- 行为风险：先替换 Codex 配置，再写 `codex-config.json` 的摘要记录。后一步失败没有回滚，调用方以
  `codex_config_write_failed` 退出并宣称未改变文件，receiver 偏好仍 off；已有 exporter 指向未运行的接收端，
  同时没有有效摘要记录供后续安全移除。重试会把有标记的 exporter 认作 managed 并跳过补记。
- 证据：隔离 state-dir 先 enable 再 disable；在其 `telemetry/codex-config.json` 位置创建目录，确定性注入摘要文件
  原子 rename 失败；对不含 `[otel]` 的合成 Codex 配置执行 `telemetry enable --write-codex-config`。
  实际 exit **1**、stderr 含 `codex_config_write_failed` 与 `Nothing was changed`；前后字节比较证明 Codex 文件
  **已改变**，`receiver.json.enabled=false`。未读取或输出 token 内容。
- 契约：Task 7 与 CLI UX 要求失败不改配置、状态与结果文案真实；architecture.md 要求保存受管段摘要以支持安全移除。
- 💡 修复建议：把 producer 配置、管理记录与 receiver 偏好作为可恢复的操作处理；记录写入/最终偏好写入失败时，
  在不覆盖并发用户修改的前提下回滚或明确保留可恢复状态，保证错误文案与实际结果一致。覆盖摘要写入失败、重试、
  disable 恢复及 receiver 配置保存失败的边界。
- Disposition：OPEN；修复范围为本任务受管写入及调用方失败恢复；对应 `codex-config-consent`。

### 🟡 建议改进 — 推荐

无额外低严重度发现。

### 🟢 优点

- 候选身份及九项准则均可追溯；接收端先白名单化、再持久化，token 不进入 status。
- JSONL/OTel 所有权区分自身贡献，晚到 JSONL 以事务内触发器撤销贡献；合成会话来源有明确投影与分类接口。
- 实现方主动记录 TUI 标题生成与 Memories 联动的设计前提偏差。

### 📝 总结

两个独立反例分别否定持久接收与配置失败原子性，不能以现有全量通过记录覆盖。生产代码、测试和配置保持只读。

Evidence：

- 复用同一精确候选的既有 Go 全量/race/vet、native XCTest、Release 双架构和隔离真实 producer 实验记录，
  不因评审阶段切换重跑；本轮仅新构建 CLI 并执行上述两项隔离反例，不宣称独立复跑这些广域检查。
- CEv1 初次固定模板查询：九条 required 准则齐全，门禁为 VERIFIED。反例作为新的失败观察追加，取代对应两条
  实现通过观察的复用资格；保留历史事实。新观察 ID：`scan-perf-otel-review-r1-20261029-durable-ingress`、
  `scan-perf-otel-review-r1-20261029-codex-config-consent`。
- TUI 标题生成在 Memories off 时漏采的前提偏差来自既有设计，不在本轮擅自更改联动。载体为
  `ad-bug-codex-subagent-usage-attribution` 与本记录“实现交接 / 待评审关注”；实现中的 off 停止规则与当前已评审
  architecture.md:535-538 一致。若调整产品规则，须先完成相应设计决定；本轮两项实现缺陷不依赖该决定。
- 本轮未重跑 producer 支持组合实验，未给未验证组合追加支持承诺；Task 8 与整个 Topic 仍未完成。
- 收口检查：`python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/otel-capture-and-reconciliation.md`、
  `make check-whitespace`、`git diff --check` 均通过。CEv1 关系批次预检 6/6 为 ok，回读新增节点 2/2、关系 6/6；
  最终固定模板 gate 查询为 FAILED，失败准则为 `durable-ingress` 与 `codex-config-consent`，其余七项沿用原证据，
  无 unresolved candidate impacts。Beads 交接评论 `9c00deb8-6897-5555-b25c-35c03c50fed8`；同一任务回到
  `in_progress`、`round-1`，评审分配已释放，Review 保持未勾选。

完成门禁：FAILED

下一步指令：修复：scan-performance / otel-capture-and-reconciliation / OTEL-R1-F1、OTEL-R1-F2

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-29（Round 1 后）

Repairer：Claude Code 主会话（本任务实现者）；范围仅 OTEL-R1-F1、OTEL-R1-F2。Round 1 的评分与 FAIL 结论保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- OTEL-R1-F1 -> repaired in candidate：
  - **复现：** `TestJournalAppendAfterTornTailKeepsAcknowledgedRecords` 在修复前（临时禁用尾行终结）两个子测试都失败：
    残缺尾行后追加的已 ACK 记录 replay 为空——同段时被拼成一条 `journal_line_unreadable`，轮转时 reader 停在残缺段、
    到不了后继段，与 Round 1 证据一致。
  - **修复：** `internal/telemetry/journal.go` 的 `Append` 在 journal 独占锁内、cap 与轮转判断之前检查末段最后一字节；
    不是 LF 即追加一个 LF 并 fsync（`terminateTornTail`）。锁保证此时没有进行中的已确认写入，残缺片段从未 ACK，
    终结后它单独读为一条 `journal_line_unreadable`，新批次总从记录边界开始，已持久完整行与 cursor 不变。
  - **回归保护：** 同一测试覆盖“截断尾行 → 读取 → 追加 → replay”同段与段边界（轮转）两种情形，断言 replay 依次为
    一条不可读记录与原样的已确认记录，轮转情形断言确实产生第二段。
- OTEL-R1-F2 -> repaired in candidate：
  - **复现：** 按 Round 1 场景（摘要记录位置为目录）写成 `TestTelemetryEnableFailureAfterCodexWriteLeavesNoUnmanagedBlock`
    的第一段；修复前顺序为先替换 Codex 配置、后写摘要记录，记录失败即留下无记录的受管段。
  - **修复：** `WriteManagedBlock` 先写摘要记录，再原子替换 Codex 配置；替换或写后校验失败时恢复原字节，并恢复先前记录
    （无先前记录则删除）。只有受管段在回滚前已被他人改动、无法撤回时，才保留受管段与其记录并附加
    `ErrCodexConfigBlockRemains`。CLI `enable` 在 receiver 偏好保存失败时用 `RemoveManagedBlock` 撤回刚写入的段；
    撤回不了时以 `codex_config_write_failed` 退出，文案改为说明文件已改动、receiver 未启用并提示 `telemetry disable`，
    不再宣称 Nothing was changed。`ux/cli.md` 标本注释补记这一残留情形；错误码不变，App 不解析该文案。
  - **回归保护：** `TestManagedRecordFailureChangesNothingAndRetrySucceeds`（记录失败不改 Codex 字节；重试写入且 disable 可移除）、
    `TestManagedWriteFailureRestoresThePreviousRecord`（配置不可写时先前记录原样恢复）、既有不可写目录用例补断言不留记录、
    CLI `TestTelemetryEnableFailureAfterCodexWriteLeavesNoUnmanagedBlock`（Round 1 场景 exit 1、字节不变、receiver off；
    注入偏好保存失败后字节恢复；重试与 disable 正常）、`TestTelemetryEnableReportsABlockThatCouldNotBeTakenBack`
    （回滚前段被改动：不出现 Nothing was changed，记录保留）。偏好保存失败经新测试缝 `saveTelemetryConfig` 注入，
    沿用 `saveQuotaSettings` 的既有做法。

受影响证据处理：

- **内容状态：** HEAD 仍为 `27621aaced6e3bcf6fa80afba188efa242607bff`；按实现交接配方复算 55 路径指纹
  `bb30efc541990408e2bb58ad41690d0af46b771e171bf4410196c6426bef908e`。相对实现状态只有 6 个路径变化：
  `internal/telemetry/journal.go` `c3c2cd38`、`internal/telemetry/telemetry_test.go` `3a209d0d`、
  `internal/telemetry/codexconfig.go` `71b25ad0`、`internal/telemetry/codexconfig_test.go` `b2c406e8`、
  `cmd/agentdeck/telemetry.go` `156af7a4`、`cmd/agentdeck/telemetry_test.go` `fb6dcd94`（`git hash-object` 前 8 位）。
  CEv1 ContentState `scan-perf-otel-r1f-20261029-state`。
- **CEv1：** 九项准则在新状态下各记一条 `scan-perf-otel-r1f-20261029-<criterion>`。`durable-ingress` 与
  `codex-config-consent` 以本次回归与全量测试直接记录，不 roll up 到已被 Round 1 取代的实现证据；其余七项
  `rolls_up` 到对应 `scan-perf-otel-impl-20261029-*`：修复不触及其代码路径（尾行终结对完好 journal 为 no-op），
  并在新状态重跑 Go 全量。原实现证据与 Round 1 失败观察保持原状不改写。关系预检 25/25 ok，写入 25/25。

验证：

- `scripts/run-go-test.sh -count=1 ./...` 通过；`scripts/run-go-test.sh -race -count=1 ./internal/telemetry` 通过；
  `go vet ./...` 干净；改动文件 `gofmt -l` 无输出；`git diff --check` 通过。
- 未重跑 native XCTest、Release 双架构构建与隔离真实 producer 实验：本次未改 Swift 与 App 资源，
  也未改接收白名单、所有权或联动；未重跑 Round 1 的隔离 CLI/HTTP 反例脚本，由上述等价单元与命令测试覆盖。
- **CEv1 gate：** 以共享 provider 的 `gate-status.cypher` 查询 `scan-performance:otel-capture-and-reconciliation` /
  `scan-perf-otel-r1f-20261029-state`：status VERIFIED，`missing_criteria`、`unresolved_candidate_impacts` 为空；
  `invalidated_evidence` 只列出已被 Round 1 取代、未被新证据引用的两条实现证据。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / otel-capture-and-reconciliation

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 2 — 2026-10-29

## 📋 Task 7 修复复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：OpenCode / GPT-6 Astra；沿用 Round 1 独立评审角色，未参与修复、未委派。
- Method：逐项检查修复源码、故障回归断言与精确状态证据谱系；复用有效测试证据。
- Scope：OTEL-R1-F1、OTEL-R1-F2 及修复影响的 journal/配置写入/CLI 错误路径；不是重新审计整个 Topic。
- Reviewed state：HEAD `27621aaced6e3bcf6fa80afba188efa242607bff`；55 路径指纹独立重算为
  `bb30efc541990408e2bb58ad41690d0af46b771e171bf4410196c6426bef908e`；ContentState `scan-perf-otel-r1f-20261029-state`。
- WorkUnit：`scan-performance:otel-capture-and-reconciliation`；Beads：`ad-scan-performance-otel-capture-and-reconciliation`。

### 🔴 严重问题 — 必须修复

无。两项历史发现均已关闭，无新增阻断发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **OTEL-R1-F1 CLOSED**：`internal/telemetry/journal.go:110-120,266-288` 在独占锁内、cap/轮转前终结残缺尾行并 fsync，
  新批次不会再与旧片段拼接；已有完整行及 cursor 保持原位置。
  `TestJournalAppendAfterTornTailKeepsAcknowledgedRecords` 同段与轮转两分支断言残缺行单独为 invalid、后续记录可重放，
  轮转分支还断言产生第二段。对应精确状态证据 `scan-perf-otel-r1f-20261029-durable-ingress`。
- **OTEL-R1-F2 CLOSED**：`internal/telemetry/codexconfig.go:168-218` 先写管理摘要，失败不进入 Codex 替换；
  替换/校验失败走原字节恢复和记录恢复；`cmd/agentdeck/telemetry.go:268-301` 在偏好保存失败时撤回新写受管段，
  无法撤回则保留记录并明确报告残留，不再谎称未修改。
  `TestManagedRecordFailureChangesNothingAndRetrySucceeds`、`TestManagedWriteFailureRestoresThePreviousRecord`、
  `TestTelemetryEnableFailureAfterCodexWriteLeavesNoUnmanagedBlock`、`TestTelemetryEnableReportsABlockThatCouldNotBeTakenBack`
  分别保护原反例、旧记录恢复、偏好保存失败/重试/关闭及并发编辑后的真实提示。
  对应精确状态证据 `scan-perf-otel-r1f-20261029-codex-config-consent`。

### 📝 总结

两项修复与原反例的因果路径吻合，新增断言保护失败边界，复评 PASS。CLI UX 的两行注释说明残留失败结果，
与代码一致，没有新增成功路径或自动配置授权。Round 1 的 TUI/Memories 前提偏差继续由既有 Bug 载体负责，
本轮不改变已评审联动规则或支持表；跨主题辅助会话分类依赖仍保留。

Evidence：

- 复用 `scan-perf-otel-r1f-20261029-*` 的九条精确目标证据：修复两项为新回归证据，其余七项显式 roll up 到未受影响的实现证据。
  已检查节点、observed_at/satisfies/rolls_up 关系及修复范围；被 Round 1 supersede 的两条旧通过证据未参与新状态复用。
- 复用修复阶段的 Go 全量、telemetry race、vet 通过结果；未因复评阶段切换重跑。Native/Release/真实 producer 证据按
  修复记录的影响评估复用，本轮未独立执行这些检查，也未重复 Round 1 的隔离反例脚本。
- 固定 `gate-status.cypher` 对本 WorkUnit/ContentState 查询：九项 required 均有有效 pass，`missing_criteria=[]`、
  `unresolved_candidate_impacts=[]`；历史 invalidated 列表仅包含两条已被替代的旧实现证据。当前失败证据集合为空。
- Topic 分解由 `tasks.md` Tasks 矩阵与任务 8 定义；Task 8 仍未完成。本次 CEv1 contains 查询没有返回父关系，
  不据此虚构 Topic 完成或修改层级，亦不跨越 Topic 完成门禁。
- 收口：本记录的 `scripts/check-review-records.py`、`make check-whitespace`、`git diff --check` 均通过；
  Beads 评论 `01118e16-fc89-587a-b8d7-e0d50350a9e2` 记录交接，任务已转 `awaiting_commit`，矩阵 Review 已勾选。

完成门禁：VERIFIED

Task checkpoint：ad-scan-performance-otel-capture-and-reconciliation；content_state=bb30efc541990408e2bb58ad41690d0af46b771e171bf4410196c6426bef908e；gate=VERIFIED。
提交建议：经用户授权后，提交 Task 7 的已评审实现、测试、CLI UX 补充及本任务记录/矩阵；暂存前核对候选身份与范围，按项目要求签名提交。
推送建议：本地跟踪目标为 origin/feature/scan-performance；完成授权提交、提交态验收与远端/分支检查，并取得单独推送授权后推送。

下一步指令：开发：scan-performance / cross-surface-acceptance

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## PR #61 修复交接 — 2026-10-31

来源：Codex 对 `3e07c892f34d775e60919774f49ae7c85afb755f` 的三项发现：
[pending 无后续 inventory](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431001372)、
[Memories 重开后监听未恢复](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431001375)、
[归一化冲突撤回未传播](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431001378)。

- 接收端查询 pending 是否缺少接收之后的完整 inventory，按需发起单个有限扫描，
  并在完成后继续原有 grace period 与 JSONL 所有权结算。已被 inventory 覆盖的 pending
  不重复扫描；扫描自行结算时也通知 App 读取新发布。
- 配置文件事件监听独立于接收进程与自动来源更新开关。Codex 配置原子替换后刷新真实
  状态；既有采集偏好开启且 Memories 恢复时启动接收进程，不自动开启偏好或写 Codex 配置。
- 归一化结果携带已计入贡献的撤回，后续 Settle 同步会话投影并通知发布；投影失败保留
  待同步状态，成功后清除，避免冲突会话留在会话索引或漏发刷新通知。

验证：`TestServeReconcilesNewBackgroundUsageAndPublishesConflictRetractions` 以临时状态、
合成 OTLP HTTP 请求及受控时钟，确认一次有限 inventory 后计入 1010 tokens 并投影；
冲突请求后 tokens、会话标记及索引投影撤回且通知发布。
`TestReconcilerRetriesNormalizationProjectionAfterFailure` 确认投影失败重试与成功后幂等。
两项定向 Go 回归通过。原生
`testConfigurationChangesResumeReceptionWithoutReopeningSettings` 使用临时配置文件的
真实原子替换事件，验证无需重开 Settings 即恢复监听，且没有启用/配置写入调用；
TelemetrySettingsController 的 10 项 XCTest 与日期表面用例定向运行通过。`make vet` 通过。

完整 `make test-macos-app` 通过（121 Shared、165 App、51 Widget，1 项既有跳过，
0 failure）。Go 全量首次运行中其余包通过，三项失败按原断言复核：两个 LaunchServices
1 秒超时用例在隔离运行中通过；`TestBatchTemplateStartIsANoOpAndBindsTheSourceTree`
在允许只读 `ps` 的环境通过。后者在受限沙箱中 `ps` 明确被拒，原 worker 等待因此提前结束，
导致读取尚未刷盘的空 trace；没有通过忽略 trace、放宽断言或增加超时消除失败。

本机完整相关包 race 运行中，store 包通过；scanruntime 的多个 serving 读取在既有
200 ms `ServingGenerationReader` 期限内超时，保留该运行的失败事实，完整 race 以远端
Actions 继续核对。新增接收回归的时钟驱动已修正：先观察 inventory 完成通知并确认仍为
pending，再推进 grace period，避免提前到达的发布通知被误当作冲突完成通知。
修正后的接收/投影重试两项回归以 `-race -count=10` 连续通过；store 完整包 race 通过。

本轮为用户授权的 PR 修复与交付；新候选等待独立 Codex Code Review，历史 PASS 和
精确内容门禁保持其原内容绑定。全部 Actions 成功后才发送新的 `@codex review` 请求。

## PR #61 第二轮修复交接 — 2026-10-31

Codex 对 `d767bd202d134623ca33bf55eefda1e04ad52b92` 新增
[合成会话投影使覆盖回退](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431526556) 与
[缺失 token component 被当成零](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431526563)。

- 合成 OTel 贡献与会话投影在同一事务内保留写入前已 complete 的 JSONL inventory，
  不推进 head 或 captured_at，不把原先 pending/partial 的覆盖改为 complete。
  同事务内只有合成事实发生变化；外部 JSONL 写入仍按 SQLite 隔离与原触发器失效。
  同值 source/metadata upsert 不再写入，避免反复推进 serving revision。
- completed 观测要求 input/output 均有值才可计量；缺失值保持 null，保留为
  `capture_only / token_components_missing`。Store 入口也检查该条件，settle 撤回旧版
  已计入的缺失组件贡献，重新分类并同步会话标记，重放不恢复虚构的零。

直接验证：修复前，投影回归观察到 complete → update_pending，缺失计数仍为 pending，
历史错误贡献未撤回；修复后对应回归通过。另覆盖普通/serving 两类完整轮次及重复扫描的
最终发布，两库覆盖均 complete、没有 source_update_pending；真正的 JSONL 写入仍
使覆盖 pending。输入/输出缺失、两者缺失、历史撤回及显式 0 值的边界回归通过。

本轮继续沿用用户已授权的提交/推送与 CI 循环；SQL 池提交 `c768199` 保持独立。
最终实现的本地验证：Go 1.27.3 经 `scripts/run-go-test.sh -p 1 ./...` 全套通过，
session/store/telemetry/scanruntime 四包 race 通过，`make vet` 通过。
新候选待独立 Code Review，历史失败与旧内容门禁不改写。

### 本轮 CI race 整包预算诊断

候选 `041b687aa25d04c75a8c1ec230681e81c9f3fa6c` 的 push CI 全部成功，
但 [PR verify](https://github.com/kitdine/agent-deck/actions/runs/41200011287/job/124533933171)
在完整 race 阶段触发 `cmd/agentdeck` 默认 10 分钟整包期限；普通全套测试已通过。
期限触发时 `TestTelemetryListenerWaitsOutAHeldStateLock` 仅运行 7 秒。PR 合并树与
push 树只差 LICENSE/README，排除测试代码差异。

保持默认期限、隔离运行 `scripts/run-go-test.sh -race ./cmd/agentdeck`，在 601.665 秒
同样触发整包期限，此时运行的是另一项测试
`TestSessionViewerTokensUseNormalizedSummaryAndEveryComponent`，仅 1 秒。
239 项顶层测试已通过，累计 599.24 秒，没有 data race 报告；仅限制包级并行度不能
解决该累计预算问题。第一次候选只将整包预算设为 20 分钟，全套并行 race 仍在
1202.521 秒触发期限，此时单项测试仅运行 2 秒；同时暴露了 PTY 测试失败清理的
data race（修复见 Task 6 记录），不能视为通过。

资源争用亦有同机对照：`TestIsolatedEndToEndFlow` 隔离运行 42.25 秒，全套包并行时
115.76 秒。后续候选将 `make test-race` 的包级并行度设为 1，并保留 20 分钟整包
预算；每个包内的并发测试、全部 race 测试、产品 200 ms 读取期限及各项断言均保留。
串行定向重跑仍复现部分失败，所以预算和资源争用不是全部原因；不以仅调整
执行参数声称修复。追加定位如下：

- receiver 在 Ready 后、初始 drain/settle 后才读取配置时间戳基线，会漏掉期间
  的 Memories 关闭。Ready 暂停握手固定了该顺序，修复前确定触发原 10 秒停止断言失败。
  基线现在读取配置和 Ready 之前采集，第一轮轮询仍能观察到这次改动。
  测试取消并等待 receiver 退出，失败也不遗留后台监听者。这是生产配置竞态修复。
- worker 协议测试的空两库初始化现在原 5 秒请求期限前完成，保留真实的 Unix/
  detached helper 启动、轮次、进度与重放断言；失败也回收 helper。此为协议测试夹具修复，
  不以其代替冷库迁移或性能验收。
- settled update 测试原来共用 seed/update/oracle 的 15 秒 context，oracle 入口已无预算。
  三个独立操作现各用原 15 秒预算；一次 plan/全量发布、今日汇总数值及完整 usage/session/
  coverage 的 fresh oracle 等价断言均保留。此为测试 context 生命周期修复。

Unix 轮次、detached helper、settled update 四种变更、Ready/Memories 关闭回归在修复后
各连续 3 轮 race 通过。提交前完整 CLI race 继续执行；完整 receiver 测试与当前候选 CI
仍待完成。全部 Actions 成功后才请求新评审，未新发评审 PASS 或 Topic 验收。

## PR #61 第三轮修复交接 — 2026-11-01

来源：[规范化暂态失败后重试 journal](https://github.com/kitdine/agent-deck/pull/61#discussion_r4432027292)，
评审目标 `c37829d14fde0b81e8b6ac3894ed11137d95e0e4`。用户授权沿用修复、提交、推送、
全部当前提交 Actions 成功后请求新 review 的流程；SQL 池提交 `c768199` 保持独立。

原 receiver 在 HTTP durable ACK 后丢弃 Drain 错误，周期分支只 settle 已入库的记录，
没有下一次 HTTP 时不会恢复规范化。只读核验在相同 WAL 配置下复现读后写的
`SQLITE_BUSY_SNAPSHOT(517)`，回滚后新事务可成功；接收器故障注入亦验证了原先只有
再发 HTTP 才能恢复。数据仍在 journal，问题是存活 listener 内的处理与计量停滞。

现由 actor 保留 `drainPending`，复用现有 2 秒 ticker 重试失败批次，成功即清除；
空闲无失败时不新增 journal 读取。配置关闭/记忆关闭检查仍先执行。重试从已提交 cursor
继续，沿用 observation key 幂等与 cursor/observation 同事务，不改变 ACK 持久化边界。

`TestServeRetriesFailedNormalizationWithoutAnotherDelivery` 通过临时 SQLite trigger 注入一次
规范化失败，只有一次真实 loopback HTTP 请求；解除故障后自动恢复、推进 cursor、完成一次
inventory，再推进业务时钟，发布并恰计入 1010 tokens、一份贡献及会话标记。该回归连续
3 轮 race 通过；遥测整包 race 与 `make vet` 亦通过（Go 1.27.3）。使用隔离 state/HOME 与合成记录，不宣称真实 producer 验收。

新候选待独立 Code Review；历史失败与旧内容门禁保留，不新发 PASS 或 Topic 验收。

## PR #61 第四轮修复交接 — 2026-11-01

来源：[import 撤回未投影](https://github.com/kitdine/agent-deck/pull/61#discussion_r4434849124)、
[运行中端口变更未生效](https://github.com/kitdine/agent-deck/pull/61#discussion_r4434849126)、
[空的不完整覆盖在全量发布中显示为 0](https://github.com/kitdine/agent-deck/pull/61#discussion_r4434849129)，
评审目标 `fa76d9f8de5a3027e049dc590a046c03c19a953c`。用户授权判断、修复、CI 全绿后请求新 review 并循环至无意见。
三条均核实成立；前几轮 10 条线程逐条核对当前代码，均已修复，无需再改。

`telemetry import` 直接调用 `Import`，绕过 `Drain` 的 `projectionPending`，冲突撤回后
`Settle` 无变化，`session list` 仍显示已撤回会话。新增 `Reconciler.Import` 复用同一通道，CLI 改用它。

listener 轮询配置时只处理关闭。`Serve` 现在先绑定新端口、再优雅关闭旧 server，同一 token、
journal 与 coverage 区间不变；前台 owner 没有 supervisor，所以不靠退出重启。绑定失败时继续
在原端口接收，写入 `bind_failed`，下次轮询重试，成功后清除。

`Build` 的 `usageSnapshot` 无条件可用，只有 `BuildToday` 判定空的不完整覆盖。判定收敛为
`withholdUnprovenZero`，并在 `prepareServingSnapshot` 对全量与派生缓存路径一并生效；complete
下的实测 0、partial 下已提交的用量都保持可用。

回归 `TestTelemetryImportRetractionReachesTheSessionIndex`、
`TestServeMovesToAChangedPortAndKeepsReceivingUntilItCan`（真实 loopback，含端口被占用再释放）、
`TestFullPublicationKeepsEmptyIncompleteCoverageUnavailable`，用 overlay 换回 `fa76d9f` 实现时均失败，
修复后通过。使用隔离 state/HOME 与合成记录，不宣称真实 producer 验收。

新候选待独立 Code Review；历史失败与旧内容门禁保留，不新发 PASS 或 Topic 验收。

## PR #61 第五轮修复交接 — 2026-11-01

来源：[listener 停止时观察接收配置变化](https://github.com/kitdine/agent-deck/pull/61#discussion_r4434993438)，
评审目标 `bfe24bab428b2ef7b3ace212eda4cd25aeb55199`，核实成立。App 以采集关闭启动后，CLI 在 exporter
已配置时执行 `telemetry enable` 只改 `telemetry/receiver.json`，App 只监听 `.codex/config.toml`，
listener 又未运行，接收要到重开 Settings 或重启 App 才开始。

`TelemetrySettingsController` 改为监听一组配置文件，App 同时传入这两个路径，复用同一 `load()` 与
`startListenerIfNeeded()`；不新增定时器。`testEnablingTheReceiverElsewhereStartsReceptionWithoutReopeningSettings`
用真实原子替换与 FSEvents 回归。

新候选待独立 Code Review；历史失败与旧内容门禁保留，不新发 PASS 或 Topic 验收。
