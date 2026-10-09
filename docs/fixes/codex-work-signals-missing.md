---
status: active
created: 2026-10-09
---

# 缺陷：Codex 工作信号缺失，活动、工作流和工具摘要显示不可用

## 已交付历史与当前修复

本修复已于 2026-10-09 授权提交并推送，交付提交为
`3fba41a6f93655c2cc75fd18ed9ea4cba54d4bbb`，Git tree 为
`29eec830f28aeb807f5740cf38e6a7bdf1cca1bb`。对应不可变 ContentState
`fix:codex-work-signals-missing:commit:3fba41a6f93655c2cc75fd18ed9ea4cba54d4bbb`
的 CEv1 门禁已最终化为 VERIFIED 5/5；归档前回读仍为 VERIFIED，无缺失、
失效证据或未决影响。

本记录曾在提交 `869f18e45ca609d95226f1c1c43586ca4d316780` 中按
Fix records 生命周期归档，初次归档日期为 2026-10-09。随后 GitHub 对
`c264260087f4a3de2ff45b9aec28b9e983c9ab90` 的评审发现新的
`CWS-R4-F1`，该目标的门禁现为 FAILED；原 Bug 与 WorkUnit 已返回修复。
因此本记录重新启用于 `docs/fixes/`，保留全部交付与评审历史，待本次候选
完成独立复评和证据最终化后再退休。下文旧的“等待授权提交”和下一步指令
只描述当时状态；当前工作与后续集成由原 Bug
`ad-bug-codex-work-signals-missing` 与
[PR #58](https://github.com/kitdine/agent-deck/pull/58) 承接。

## 现象

Bug：`ad-bug-codex-work-signals-missing`，P1，Lane A。用户于 2026-10-09
授权 `开发：fix / codex-work-signals-missing`；开发工作区为
`agent-deck.fix.codex-work-signals-missing`，分支为
`fix/codex-work-signals-missing`，基线为
`32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`。

2026-10-08 的只读诊断发现，Codex 已有用量和工具行，却没有对应的
classified 工作信号。Activity 的 `cost_basis` 为 `none`，Workflow 的首次
编辑和触及文件数为 null，Tooling 缺少该范围的条目。任务记录中的当日
474 条 Codex 用量和 346 条工具行是那个观察时刻的计数，不是持续计数。

本次持久化回归使用隔离数据库和合成日志：同一回合包含当前格式的用户
消息、一次 `apply_patch` 和一条 token 用量。原实现导入 1 条事件和 1 次
工具调用，但读取 `codex/s/1` 工作信号得到 `sql: no rows in result set`。
这证明缺失的是分类输入，不是没有操作或测量结果为零。

## 根因

`internal/usage/usage.go` 的 Codex 解析分支原先只对
`event_msg/user_message` 调用 `state.note`。当前的
`response_item/message(role=user)` 因而不能创建 pending 信号，后续分类和
要求 classified 行的 Activity、Workflow、Tooling 聚合都缺少输入。

另外，增量扫描原先用已经提交的事件、工具和 classified 信号恢复回合
索引。Codex 在 `turn_context` 中已经观察到的回合可能仍然只有 pending
消息；跨扫描恢复时，这个索引会丢失。只补充新 envelope 仍不能修复这一
边界。最小补充实现下，四个顺序/扫描组合中有三个仍因信号缺失或保持
pending 而失败。

## 修复边界

- `internal/usage/codex_signals.go` 支持当前直接/嵌套用户消息 envelope 和旧
  `event_msg/user_message`。只临时读取 `input_text`；忽略明确的 AGENTS、
  environment、aborted-turn 和 subagent-notification 注入上下文，不让它们
  替换真实用户输入的分类意图。
- 以既有 `turn_context` 身份和索引契约归属消息。尚未观察到下一回合的
  消息创建下一索引的 pending 行；同一回合的新旧 envelope 使用同一主键，
  不增加逻辑回合。实际输出发生后，后续消息等待下一 context。
- 在既有 `usage_source_files.turn_id` 私有游标中保存带前缀的 Codex 状态，
  包含实际 context ID、索引、是否已有输出及是否等待下一 context。
  恢复后再把真实 context ID 交给事件和工具解析器。该游标不保存消息正文，
  事件的 `event_id` 仍是原始回合 ID；不增加数据库列或修改 wire。
- usage parser version 提升至 8。版本 7 的 Codex 来源按既有来源级原子
  重解析路径重读一次；版本 7 的 Claude 来源在本次 Codex-only 更新中保持
  兼容。更旧的来源仍按其已有版本更新要求处理。该兼容例外只适用于
  version 8，不自动延续到未来 parser 更新。
- 复用已有有限 inventory、context 取消和原子发布机制。取消的补采不推进
  来源版本；后续正常扫描可以重试。第二次无变化扫描不再重解析。
- 延续重复来源 last-path-wins、消失来源的候选恢复和 run binding 保留机制。
  不改事件身份、token 算法、价格、provider、credential 或 exclusions。
- 历史补采只由授权后的正常扫描执行。本阶段没有对用户真实数据库进行
  补采，也没有改变安装版。真实历史规模的耗时和预算仍需与
  `scan-performance` 的验收联合确认；合成夹具不能证明其性能目标。

生产变化限于 `internal/usage/usage.go` 和新的 Codex 信号 helper。回归位于
`internal/usage/codex_signals_test.go` 和
`cmd/agentdeck/codex_signals_integration_test.go`。既有 Claude process-start
升级测试改为明确模拟其原本声称的 version 6，避免把此次兼容的 version 7
误当作缺少 process-start 的旧格式。其余产品、Swift 界面和共享配置不变。

## 验证

### 失败先行

1. 基线上的 `TestCodexResponseMessagePersistsWorkSignal`：FAIL；夹具事件和
   工具均为 1，信号查询为 `sql: no rows in result set`。
2. 只增加 envelope 支持后的 `TestCodexMessageContextAndAppendBoundaries`：
   context 后单次扫描 PASS；另外三个组合 FAIL，分别为 pending 未晋升或
   缺少工作信号。添加游标恢复后四个组合全部 PASS。
3. `TestCodexInjectedContextDoesNotReplaceUserIntent` 在过滤实现前 FAIL，
   注入的 fault 文本错误地把真实中性请求改成 debugging/repair。
   `TestCodexSignalUpgradeReplaysOnlyCodex` 在版本更新前 FAIL，旧 Codex
   来源没有被标记为重读。补充实现后两个测试 PASS。

### 开发轮次证据（候选 cd791b5c）

| 检查 | 方法与结果 |
| --- | --- |
| 持久化与扫描边界 | 六个新的 usage 顶层回归及四个 context/append 子案例 PASS；覆盖 pending 晋升、下一回合、双 envelope、重复来源和消失来源恢复 |
| 定向补采与完整性 | version 7 Codex 来源重读并恢复分类；Claude 来源仍为 version 7；取消不推进版本；重复无变化扫描 replaced=0；原始文件字节及 event key、turn index、input/output token 快照不变 |
| CLI/桌面消费者 | `TestCodexCurrentMessageSignalsReachCLIAndDesktop` PASS；同一解析夹具在 CLI 显示 Debugging/Repair、首次编辑 2s、1 次 Edit，桌面 today/codex 的 Activity、Workflow、Tooling 与 CLI 数据相同 |
| 零与不可用 | 新回归断言 classified 的零 retries 为 0、文件数为 1、首次编辑为 2s；复用既有空范围、pending、无工具和不可用契约测试 |
| 完整 Go 回归 | `scripts/run-go-test.sh ./...` PASS；包含既有来源原子重建、run binding、exclusions、隐私、空范围和 desktop/CLI 契约回归 |
| 相关 race、vet、双架构构建 | `scripts/run-go-test.sh -race ./internal/usage` PASS；`GOCACHE=/private/tmp/agent-deck-go-build make vet build-all` PASS，包含 darwin/arm64 与 darwin/amd64 |
| 文档与 whitespace | `make check-whitespace` 与 `git diff --check` PASS；所有引用指向现有仓库文件或稳定任务/WorkUnit 身份 |

所有 Go 测试使用 `scripts/run-go-test.sh`。针对性命令为
`scripts/run-go-test.sh ./internal/usage -run '^TestCodex(ResponseMessagePersistsWorkSignal|MessageContextAndAppendBoundaries|InjectedContextDoesNotReplaceUserIntent|SignalUpgradeReplaysOnlyCodex|SecondTurnAndDuplicateEnvelopeAcrossAppend|MessageSignalsKeepDuplicateSourceOwnership)$'`
和 `scripts/run-go-test.sh ./cmd/agentdeck -run '^TestCodexCurrentMessageSignalsReachCLIAndDesktop$'`。

CEv1 WorkUnit 为 `fix:codex-work-signals-missing`，无 containing topic gate。
五项 required criteria 分别为 message-turn-contract、targeted-history-replay、
signal-projections、regression-verification 和 independent-review。最终候选
ContentState 绑定基线和全部在范围内的 Git blob 指纹；生产测试不会因后续
阶段切换而重复执行。独立评审证据由实际评审阶段提供。

## Review

Round 1 的 FAIL 保留；Round 2 独立复评 PASS，`CWS-R1-F1` 已关闭。
本记录保持 active，等待授权交付；修复及复评事实不等同于已经提交或发布。

## Review — Round 1

## 📋 Codex 工作信号修复评审 — 2026-10-09

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CWS-R1-F1 · P2 · OPEN：限额通知被当作 assistant 输出，导致用户信号归属下一回合。**

位置：`internal/usage/codex_signals.go:56–60`，以及
`internal/usage/usage.go:1564–1578`。

- 行为风险：`codexHasOutput` 对所有 `event_msg/token_count` 返回 true，
  包括 `info: null`、仅含 `rate_limits` 的通知。此类记录没有 assistant
  用量，但会设置 `codexOutput`；随后用户消息的信号索引加一，而实际
  event/tool 仍使用当前 `turn_context` 的索引。Activity、Workflow、Tooling
  因而无法使用本回合信号，原缺陷在该顺序下仍然存在。
- 证据：针对冻结候选运行隔离数据库的 Go overlay 探针
  `TestReviewCodexRateLimitBeforeUser/single`，输出
  `event_turn=1 signal_turn=2 state=pending; want 1/1/classified`。
  输入依次是 session_meta(id=s)、turn_context(turn_id=t1, model=gpt-5)、
  `event_msg` 的 `{"type":"token_count","info":null,"rate_limits":{"primary":{"used_percent":1}}}`、
  当前格式用户消息 `fix the crash`、apply_patch、正常 token_count。
  前四条 timestamp 均为 `2026-10-08T12:00:00Z`，工具在 +2s，用量在 +3s；
  最后用量的 last/total 均为 input_tokens=1000、output_tokens=100。
  将该限额通知插入既有 `TestCodexResponseMessagePersistsWorkSignal` 的
  context 与 user 之间即可复现；查询 usage_events.turn_index 与
  usage_work_signals.turn_index/state 可得到上述差异。
- 修复边界：区分限额/空用量通知与真实 assistant 输出，确保前者不触发
  下一回合归属。补充该顺序及相关 append 边界的持久化回归，同时保留
  实际输出之后下一用户消息的既有归属行为；不要扩大到新的回合契约。

### 🟡 建议改进 — 推荐

无。上述缺陷为本轮唯一已确认且必须关闭的 finding。

### 🟢 优点

当前 envelope、来源所有权、定向历史重读和 CLI/desktop 对照已有持久化
回归；实现不持久化用户消息正文，并明确区分隔离夹具与真实历史验收。

### 📝 总结

- Reviewed state：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`，
  scoped candidate SHA256 `cd791b5c47b80e35d4075ee4e2d1e2e36e36471d491b54d11ec1d2a4b6565233`。
  六个 scoped blobs 与 go.mod、go.sum、vendor/modules.txt 均逐项匹配
  开发 ContentState；Go `go1.27.1 darwin/amd64`、Darwin 25.6.0 x86_64
  与其记录一致。产品和测试在本轮保持只读。
- Reviewer：Codex，独立评审会话 `01a11fd5-edb0-7b72-9fcd-866d2b7d059e`；
  未参与该候选实现，未委派。实现交接 correlation 为
  `IMPLEMENT:wXXQDFk1xxIVgtNA`。
- Method / Scope：检查生产 diff、Codex helper、扫描恢复与信号归属路径、
  新增 usage 回归及 Decision 11 契约；以只影响临时构建输入的 Go overlay
  和隔离数据库证实 CWS-R1-F1。探针通过 `scripts/run-go-test.sh` 执行，
  选择 `-run '^TestReviewCodexRateLimitBeforeUser$'`。未修改仓库测试。
  临时探针最初的切片语法错误已修正，编译错误不算产品证据；追加探针的
  NULL 扫描断言失败未形成独立 finding。本轮在明确阻断后停止广泛验证。
- Evidence：入口 CEv1 为 NOT_VERIFIED，仅缺 independent-review，四项开发
  证据有效且无 invalidated evidence/unresolved impacts；复用其全量 Go、
  race、vet、双架构构建结果，不因阶段变化重跑。新增失败证据明确否证
  message-turn-contract 和 independent-review。评审记录结构由
  `python3 scripts/check-review-records.py docs/fixes/codex-work-signals-missing.md`
  检查；该检查不替代语义评审。全部复现必要事实已写入本记录，不依赖
  临时文件留存。
- 完成门禁：FAILED
- 结论与限制：CWS-R1-F1 未关闭，不能交付。真实历史数据性能及安装版 UI
  未验收；本轮不是全面无缺陷证明。后续修复须处理该 finding，并重点
  验证限额通知与 context/user 分隔扫描时的持久化归属。

下一步指令：`修复：fix / codex-work-signals-missing / CWS-R1-F1`

## 修复处置 — CWS-R1-F1 · 2026-10-09

`CWS-R1-F1 -> repaired in candidate`，待独立复评确认。Round 1 的原始
OPEN 描述、FAIL 结论及对应失败证据保留，不由修复阶段改写为 PASS。

本轮仅处理限额/空用量通知导致的输出误判，以及该 finding 要求的
context、通知、用户消息分隔扫描边界：

- `codexHasOutput` 不再仅凭 `token_count` 类型断言 assistant 已输出。
  `parse` 复用既有 token 校验和累计差分结果，仅在接受非零用量之后设置
  `codexOutput`；限额通知、空 info、空 usage 和零用量都不设置该标记。
  token 计数、去重、event 身份和显式 assistant/tool 输出规则保持原算法。
- 分隔扫描的失败先行回归还确认：扫描在 context 或通知后结束时，尚无
  用户信号，原游标仅保存原始 context ID，恢复后丢失已观察的索引。
  `storedCodexTurn` 现在对已观察到的 context 也保留既有私有游标结构。
  用户、工具和用量继续使用同一实际 context 的索引，不新增回合契约。
- 持久化回归 `TestCodexUsageNotificationsBeforeUserKeepCurrentTurn` 覆盖
  五种通知：info=null、仅 rate_limits、空 info、空 usage、全零 usage。
  每种分别覆盖单次扫描、context 后切分、通知后切分和用户消息后切分，
  共 20 个案例；检查真实 event/tool/signal 索引均为 1、恰有一条 classified
  debugging/repair 信号，以及无 assistant 时正确保持 pending。
- 正向控制 `TestCodexRealTokenUsageStillOpensNextUserTurn` 只用真实非零
  token 用量作为输出证据，不插入工具调用；后续用户仍归属回合 2 的
  pending/build 行，证明没有通过禁用全部 token 输出判定来消除症状。

### 本轮验证

失败先行命令：
`scripts/run-go-test.sh ./internal/usage -run '^TestCodex(UsageNotificationsBeforeUserKeepCurrentTurn|RealTokenUsageStillOpensNextUserTurn)$'`。
修复前 20 个通知案例全部 FAIL；单次扫描得到
`event_turn=1 call_turn=1 signal_turn=2 state=pending`，用户后切分得到
`signal_turn=2 state=pending`，context/通知后切分还观察到信号索引 0 或
缺少事件/工具行。真实非零用量的正向控制 PASS。修复后同一命令的 20 个
案例和正向控制全部 PASS。

| 最终相关内容的检查 | 结果 |
| --- | --- |
| Finding 持久化回归 | PASS；20 个通知/边界案例及真实用量正向控制 |
| 完整 Go 回归 | 用户授权辅助测试修正后，原始 `scripts/run-go-test.sh ./...` PASS；修正前默认及串行两次失败均保留在下述诊断记录中 |
| 相关 race、vet、双架构构建 | `scripts/run-go-test.sh -race ./internal/usage` PASS；`GOCACHE=/private/tmp/agent-deck-go-build make vet build-all` PASS，含 darwin/arm64 与 darwin/amd64；后续 doctor 测试修正仅使该包的 vet 失效，补跑 `go vet -mod=vendor ./internal/doctor` PASS，复用未受影响的 race 与生产构建 |
| 评审记录结构与 whitespace | `python3 scripts/check-review-records.py docs/fixes/codex-work-signals-missing.md`、`make check-whitespace`、`git diff --check` PASS |

本轮仍只使用原 Bug、Fix 工作区和 `fix:codex-work-signals-missing` WorkUnit。
修复候选的内容身份包含 HEAD 与七个 scoped blobs；本轮生产代码与回归变化
使此前生产验证不能直接覆盖新状态，按相关范围取得新证据。
独立复评和真实历史规模的性能验收继续保留为其各自边界。

### 已授权解除的验证阻塞：doctor 测试的共享 deadline

两次完整检查均在未修改的
`TestRegistrationCanonicalAbsenceDoesNotUseUnsafeNSErrorRef` 的第二次原生调用
失败：`unreadable canonical host confused with absence: timeout`，测试耗时
约 1.01s。仅执行该测试的隔离检查 PASS。因此不能声称减少包并发已经解除
阻塞，也不能把一次隔离 PASS 当成问题已修复。

源码 `internal/doctor/launchservices_darwin_test.go:134` 的两个独立夹具共用
同一个 `context.WithTimeout(..., time.Second)`：第一次检查真实不存在的
canonical host，第二次检查不可读目录。第二次调用继承第一次已经消耗的
deadline。用 Go overlay 仅为第二次调用建立独立的 1 秒 context，保持两次
调用的原时限、原脚本和原断言，整个 `internal/doctor` 包 PASS。这支持测试
harness 的共享预算耦合诊断，不能据此宣称系统负载或 LaunchServices 服务是
根因；生产探针按契约报告 timeout 不构成本轮 CWS 修复的直接缺陷。

该诊断对照仅改变临时构建输入，未修改仓库 doctor 源码和测试。
随后明确说明原预算是两次调用共享 1 秒，拟改为两次各自 1 秒、总计最多
2 秒；用户于 2026-10-09 回复“授权该测试修正”，将范围明确扩展到
`internal/doctor/launchservices_darwin_test.go` 这一个测试文件。

按此授权，永久变化仅是在第二次 `registrationCommand` 之前建立
`secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)`，
并把该调用的 context 参数改为 `secondCtx`。测试断言、fixture 脚本及生产
诊断代码/时限都不变。该测试的针对性检查 PASS；该包 vet PASS；随后在无
overlay、无串行参数的原始完整命令 `scripts/run-go-test.sh ./...` 下 PASS。
两次原始失败仍是历史观察，未被“重试成功”或更大时限抹去。

本轮 CWS 修复及用户授权的辅助测试修正已经验证，等待独立复评；没有在
修复阶段出具新的评审结论。

## Review — Round 2

## 📋 Codex 工作信号修复复评 — 2026-10-09

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。此前唯一 finding `CWS-R1-F1` 已关闭，本轮未发现新的阻断问题。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**CWS-R1-F1 · CLOSED**：`codexHasOutput` 不再把 token_count 类型本身
视为输出。`internal/usage/usage.go:1593` 在既有用量校验、累计差分及
零值返回之后才设置 `codexOutput`；因此 rate-only、null/empty info、
empty/zero usage 不推进用户信号归属，真实非零用量仍推进后续用户消息。
`storedCodexTurn` 同时保存尚未收到用户消息的 context 索引，覆盖此前
context/通知分隔扫描的恢复缺口。

`TestCodexUsageNotificationsBeforeUserKeepCurrentTurn` 的 5×4 案例覆盖
上述通知与单次扫描、context 后、通知后、用户后切分：断言 event/tool/
signal 索引均为 1、只有一条 classified debugging/repair 信号；用户后
切分还验证输出前保持 pending。`TestCodexRealTokenUsageStillOpensNextUserTurn`
不依赖工具调用，独立检查真实 token 输出之后的新消息仍为回合 2 的
pending/build，避免通过禁用所有 token 输出判定掩盖问题。

辅助 doctor 测试的第二次调用改用独立 1 秒 context，保留 ENOENT 与
不可读目录两组 fixture、原断言和生产探针时限，与修复记录中的用户授权
边界一致。

### 📝 总结

- Reviewed state：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`，
  修复候选 SHA256 `c7b4f8de7474b4c5aa458374c6517268882b9a1041ca079bf14e1b9481ee8a52`。
  七个 scoped blobs 及 go.mod、go.sum、vendor/modules.txt 与该 ContentState
  逐项一致；工具链仍为 `go1.27.1 darwin/amd64`。
- Reviewer：Codex，评审会话 `01a11fd5-edb0-7b72-9fcd-866d2b7d059e`。
  本会话承担 Round 1 和本轮复评，未参与实现或修复，未委派；修复交接
  correlation 为 `REPAIR:a65RtX6HiVqa6UZu`。复评使用独立于修复者的上下文。
- Method / Scope：逐项处置 CWS-R1-F1；检查 Codex helper、parse 的有效
  用量路径、持久化回归、未改变的 CLI/desktop 对照及新增 doctor 测试 diff。
  产品、测试与配置保持只读，仅更新本评审载体和对应工作流记录。
- Evidence：入口 gate 为 NOT_VERIFIED，仅缺 independent-review；四项
  repair-r1 证据均可复用，没有 invalidated evidence 或 unresolved impacts。
  复用同一候选的 20 个通知/边界案例、真实输出正向控制、完整 Go、usage
  race、vet、双架构构建和 doctor 针对性检查，不因复评重复运行。
  新增证据是本轮源码及断言审查、finding 关闭和评审记录校验。
  `python3 scripts/check-review-records.py docs/fixes/codex-work-signals-missing.md`、
  `make check-whitespace`、`git diff --check` 验证本轮文档变化。
- Evidence reuse：最终状态仅变更本载体的状态摘要并追加本轮报告；全部
  生产、测试、依赖文件保持修复候选内容。CEv1 为最终七文件指纹建立
  target-bound roll-up，保留四项原始修复证据及历史失败，不重写旧事实。
- 完成门禁：VERIFIED
- 限制：真实历史规模性能与安装版 UI 未在本轮验证，仍归既有联合验收及
  交付边界；此 PASS 仅关闭本次修复及记录的 finding，不表示已安装或发布。

Task checkpoint：`ad-bug-codex-work-signals-missing`；WorkUnit
`fix:codex-work-signals-missing`，无 containing topic gate；最终七文件
ContentState 由 CEv1 的本轮记录绑定，任务等待授权提交。

提交建议：仅提交该候选七个文件，包括用户已授权的 doctor 测试修正与
本修复记录；提交前核对暂存边界、贡献者、消息正文、Codex trailer 和 SSH 签名。

推送建议：授权提交并完成 commit-object 检查后，推送到
`origin/fix/codex-work-signals-missing`；后续 PR 目标为 main，PR 创建及
合并仍需各自授权。

下一步指令：`提交：fix / codex-work-signals-missing`

## Review — Round 3

## 📋 Codex 工作信号修复归档复评 — 2026-10-09

📊 总体评分：9/10（本记录的汇总评价；GitHub Codex 未提供数值评分）

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。本轮处置的 `CWS-GH-R1-F1` 已关闭。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**CWS-GH-R1-F1 · CLOSED**：GitHub 首轮评审
`5469766539` 针对提交 `3fba41a6f93655c2cc75fd18ed9ea4cba54d4bbb`
提出 P1：交付提交与证据已最终化，但修复记录仍在 `docs/fixes/` 且
`status: active`，旧的等待提交 checkpoint 仍会被当前文档发现机制读取。
原 finding 是 [PR #58 的归档意见](https://github.com/kitdine/agent-deck/pull/58#discussion_r4229892182)，
与 `docs/documentation-workflow.md` 的 Fix records 归档要求一致。

提交 `869f18e45ca609d95226f1c1c43586ca4d316780` 将记录移至
`docs/archive/fixes/`，设置 `status: historical` 和 `retired: 2026-10-09`，
保存原交付 commit/tree/CEv1 来源，并明确旧 checkpoint 为历史状态。
原开发、失败、修复与 Round 2 独立 PASS 内容保留；Fix records 明确不要求
归档索引条目。GitHub Codex 对该新提交完成复评，
[返回未发现主要问题](https://github.com/kitdine/agent-deck/pull/58#issuecomment-6081195363)。

**CWS-R1-F1 · CLOSED**：原 token 通知与回合归属修复保持 Round 2 已验证
内容；本轮没有重新打开该 finding，也未改变生产代码、测试或依赖。

### 📝 总结

- Reviewed state：GitHub 复评提交
  `869f18e45ca609d95226f1c1c43586ca4d316780`，Git tree
  `97069df2fee4d23a4019cc9121e3ed5141664535`；六个生产/测试 scoped blobs
  及 go.mod、go.sum、vendor/modules.txt 与原交付状态一致。
- Reviewer：GitHub Codex（`chatgpt-codex-connector[bot]`），独立的 GitHub
  云端评审角色，未参与实现或归档修正。本地 Codex 仅核实仓库事实、整理
  外部报告及同步记录，没有以本地自评替代该外部结果。
- Method / Scope：保存首轮 P1 的观察、风险、证据及归档补救；逐项核对
  归档路径、frontmatter、交付来源、历史保留和当前文档入口；GitHub 复评
  绑定上述提交。新一轮人工评分仅汇总本记录，不冒充 GitHub 输出。
- Evidence：`python3 scripts/check-docs.py`、基于原提交的归档记录结构检查、
  `make check-whitespace` 和 `git diff --check` PASS；生产、测试、依赖未变，
  复用既有完整 Go、usage race、vet 与双架构构建证据。本轮追加报告只保存
  已发生的评审事实；包含报告的最终内容另由 CEv1 绑定，不重标旧观察。
- 完成门禁：VERIFIED
- 限制：GitHub 原生 code review 主要报告 P0/P1；其无主要问题反馈不等同于
  GitHub APPROVED，也不扩展为真实历史规模性能或安装版原生 UI 验收。
  后续交付及集成事实由原 Bug 和 PR #58 的记录承接。

Task checkpoint：`ad-bug-codex-work-signals-missing`，WorkUnit
`fix:codex-work-signals-missing`，无 containing topic gate。
提交建议：仅交付本归档记录的复评事实与证据状态；用户已授权自动修复交付。
推送建议：同一 `origin/fix/codex-work-signals-missing`；最终 CI、GitHub
复评及必要证据门禁通过后按用户授权集成至 main。

## Review — Round 4

## 📋 Codex 纯字符串消息兼容评审 — 2026-10-09

📊 总体评分：8/10（本记录的汇总评价；GitHub Codex 未提供数值评分）

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CWS-R4-F1 · P2 · OPEN（被评状态）**：
`internal/usage/codex_signals.go:100` 只将用户消息 content 断言为 `[]any`。
有效的纯字符串得到 nil slice，继而返回无信号。会话索引的
`extractCodex` / `textContent` 明确接受 user/input_text 的字符串形式，
所以同一会话可被检索，却仍缺少 Activity、Workflow、Tooling 归属。
- Carrier：`ad-bug-codex-work-signals-missing`；本条保持同一 Bug、工作区与 WorkUnit。
- Evidence：GitHub 评审 `5470498471` 的
  [P2 意见](https://github.com/kitdine/agent-deck/pull/58#discussion_r4230474952)，
  被评提交 `c264260087f4a3de2ff45b9aec28b9e983c9ab90`。
  本地失败先行的直接/嵌套持久化案例均已有 1 条用量和 1 次工具调用，
  信号查询均为 `sql: no rows in result set`。
💡 修复边界：接收直接/嵌套 message 的字符串 content，沿用现有注入上下文
过滤与回合归属；不改变 token、来源所有权、parser-version 或存储字段。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

`CWS-R1-F1` 与 `CWS-GH-R1-F1` 的既有关闭事实保留。后者记录的是原交付
状态的归档，新的生产兼容问题使当前修复重新启用；不是抹去原归档事实。

### 📝 总结

- Reviewed state：提交 `c264260087f4a3de2ff45b9aec28b9e983c9ab90`，tree
  `ee660e2fd950879f8eb7f6b38606a922b4b8640d`。
- Reviewer：GitHub Codex（`chatgpt-codex-connector[bot]`），评审
  `5470498471`；本地 Codex 只核实源代码、复现与记录该外部 finding。
- Method / Scope：对照既有 session 输入契约与 usage helper；CodeGraph 用于
  已索引的会话契约，目标 helper/测试查询未命中后按项目降级规则读取指定
  两文件；新持久化用例用于运行证明，不把图关系当成证明。
- Evidence：失败先行命令为
  `scripts/run-go-test.sh ./internal/usage -run '^TestCodexStringMessagePersistsWorkSignal$'`；
  direct/nested 两案例均 FAIL，原因是缺少信号而非语法、fixture 或环境问题。
- 完成门禁：FAILED
- 限制：本轮实际 GitHub 反馈包含 P2；不依据前轮对主要严重级别的概述忽略
  低级别意见。每个 in-scope finding 仍须闭环后才能合并。

## 修复 — CWS-R4-F1

当前候选在 `codexSignalMessage` 的 block-array 分支前增加字符串分支，
仍进入同一个 `codexInjectedContext` 检查。生产变化仅四行；Session 的
既有字符串契约、其他 envelope 和 Version 8 的定向恢复边界不变。

新增直接/嵌套字符串消息的持久化回归，核对一条事件、一条工具记录、
classified debugging/repair 信号及 source owner，验证 Activity、Workflow、
Tooling 与 individual/desktop batch 一致；模拟 Version 7 后重解析恢复相同
投影，随后无变化扫描不再 replaced。既有注入上下文回归补充直接/嵌套
字符串注入，保持真实中性意图；原数组案例和其他回归保留。

针对性字符串、注入上下文和历史恢复检查已转为 PASS，完整
`scripts/run-go-test.sh ./...`、`scripts/run-go-test.sh -race ./internal/usage`
及 `GOCACHE=/private/tmp/agent-deck-go-build make vet build-all` 全部 PASS，
含 darwin/arm64 与 darwin/amd64 构建。该候选仍需独立 GitHub 复评，
不能由本修复自行关闭 finding。

用户已明确限制自动流程：接下来的 GitHub 复评若仍未通过，保留现场并停止，
不再自动开启下一轮修复；通过后继续原授权的合并及相关 contract 同步。
