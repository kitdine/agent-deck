---
status: historical
created: 2026-10-01
retired: 2026-10-01
---

# 缺陷：Codex 会话丢失项目元数据并退化为日志日期项目

Beads: `ad-bug-session-project-attribution-observer-noise`。Lane A，用户明确授权
从工作区准备至 PR merge 的端到端执行及冷上下文独立评审。
工作区：`agent-deck.fix.session-project-attribution-observer-noise`；
分支：`fix/session-project-attribution-observer-noise`；PR 目标：`release/v0.6.x`。
创建基线：`6f22e76da9f720fed8d00f5a031e796bcb4f6cd6`，经远端核验。
串行 slot `.worktrees/fix`：旧 Widget Bug 已关闭、干净且最终交接释放；
规则来源为已合并 `59aa33a` 的 Serial Lane A Fix workspace entry。

## 现象

标准 Codex `session_meta.payload.id/cwd` 未进入会话元数据，项目退化为
JSONL 日期目录，影响项目聚合和最近会话身份。隔离回归也证明后续标准
message 不携带 session ID 时，原提取器在应用流内 fallback 之前调用
`ApprovedDocument`，导致可见正文遗漏。

## 根因

`extractCodex` 只识别 legacy ID；`meta` 把 payload.project 放在顶层 cwd
之前；增量读取没有已建立的会话上下文。parser v5 的 unchanged 快路径让旧
错误索引一直跳过重解析。这些均在指定基线的隔离日志/数据库复现，
不依赖桌面标签渲染或 Git worktree 合并策略。

## 修复边界

- 仅 session_meta 接受 canonical payload.id，优先于 legacy payload.session_id、
  顶层 session_id/sessionId；普通 payload.id 仍不能成为会话 ID。
- 文档提取前应用已建立会话 ID；cwd 优先于 project，原始目录仅作最后回退。
  后续真实 cwd 可替代较弱 project 回退。
- 普通/共享/fixture 读取一致。源 cursor 中持久化 parser_context，保留当前 ID、
  元数据及 cwd 优先级，包括被用户排除的会话；不从可见索引推测尾部身份。
  多会话源或无有效上下文的源安全重解析；共享计划与实际读取使用相同判据。
  已排除的源也保留完整 replay range，防止真实 cwd 使其恢复可见时丢失旧正文。
  重命名时该资格依据 cursor 的旧路径/目录判断，不能用新路径抹去旧排除事实。
- sessions.sqlite3 仅新增 parser_context 列，现有索引数据和排除不删除，核心
  数据库 schema 不变；旧 v5 数据按 parser-version 契约在下次扫描重解析。
- parser v6 触发现有按源重解析事务，旧元数据/FTS/cursor 原子替换；失败保留旧
  投影，重复扫描跳过。不重建真实数据库，不删除原始日志或用户排除。
- 不修改 usage、默认辅助会话过滤、用量分类、worktree 合并或 claude-mem。

## 验证

- 失败先行：`/private/tmp/agentdeck-spa-red.log`，三个行为测试在未修改产品代码
  上分别失败于 canonical ID、cwd 优先级和 v5 缓存跳过重解析；测试 fixture
  编译问题在记录 RED 前已修正。
- `scripts/run-go-test.sh ./internal/session` 通过；最新日志
  `/private/tmp/agentdeck-spa-final-package.log`。覆盖共享/普通 ID-less 正文和追加、
  错误旧 ID/项目恢复、发布失败事务回滚、排除和原始日志不变、重复扫描及晚到 cwd。
- Round 1 反例主进程 RED/GREEN：`agentdeck-spa-r1-f1.log` 与
  `agentdeck-spa-r1-f1-green.log`。修复后 session/store 包通过
  `/private/tmp/agentdeck-spa-repair2.log`，包括升级保留和核心 DB 字节不变。
- synthetic snapshot golden 经生成器更新后仅增加既有真实 model `gpt-5.6`；
  targeted contract 通过 `/private/tmp/agentdeck-spa-snapshot-update.log`。
- 最终全套 `scripts/run-go-test.sh ./...` 通过，日志
  `/private/tmp/agentdeck-spa-final-all.log`，SHA-256
  `2b46bfdd9dc07bfa53ea6ad81e5f792af946ec7bbf08faf1d96aaebefe6e7a86`。
- 最终 `-race ./internal/session ./internal/store ./internal/scanruntime`、
  `make vet check-whitespace build-all` 和 `git diff --check` 通过；两种 macOS
  架构 compile 通过。race 日志 `/private/tmp/agentdeck-spa-final-race.log`。
- 冷上下文独立 Round 5 PASS；四个独立反例最终通过，日志 SHA-256
  `15e0af61c40acadda6884824e491c25381e3a50f10ec770f04614f921e5157b9`。
  未声称真实用户数据库或部署二进制已修复/验收。

## 交接状态

Round 5 PASS，最终必需检查通过。实现已签名提交 `446509a2f770663898cec3d734f9bad2985a83ac`，
tree `0abc31092f36fa8fb7027394c8f168787c8058e3`；实际 subject/body/trailer、七个
文件 blob 和 SSH 签名均已核验；工作区干净，未触碰其他 worktree。
CEv1 Task `fix:session-project-attribution-observer-noise`，target
`fix:session-project-attribution-observer-noise:candidate:round5` VERIFIED 4/4，
missing/invalidated/unresolved 均为空。GitHub 当前 head review、CI 及 merge 待执行。

提交后的 immutable target `fix:session-project-attribution-observer-noise:commit:446509a2f770663898cec3d734f9bad2985a83ac`
也 VERIFIED 4/4；四项 target-bound roll-up 经 scope-aware preserves assessment
复用 Round 5 证据。归档仅完成本地 authorized commit/evidence 边界，不声称 PR
已经 merge；Bug 保留 awaiting_commit。后续 PR/head review/CI/actual merge、
Task/Integration gate 和 slot 交接由该 Beads Bug 及 CEv1 记录最终状态。
归档无需 archive index entry，沿用现有 Fix records 约定。

## Review — Round 1

## 📋 独立代码评审

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

`SPA-R1-F1` / P1 / `internal/session/session.go:1005`：初版从可见索引恢复
增量上下文时，把 canonical cwd 的优先级降为 project 回退。初始 cwd
`/work/original` 后追加 cwd `/work/subdir`，追加扫描选 subdir，而完整扫描选
original；项目归属依赖扫描时机，可能跨过用户排除边界。
证据：冷上下文 reviewer 的隔离 overlay 反例由主 agent 独立重跑，
`/private/tmp/agentdeck-spa-r1-f1.log` 以两种项目不等失败。
修复：cursor 原子持久化真实解析上下文和优先级，增加共享/普通追加与完整扫描
一致性及被排除尾部身份保护。`SPA-R1-F1 -> repaired in candidate`，待独立复评。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

canonical ID、cwd 优先级和 v5 重解析有原缺陷失败先行回归；事务失败保护
和原始日志不变有隔离数据库证据。

### 📝 总结

Reviewer: `/root/cold_review`；Method: `fork_turns=none` 冷上下文只读源码与
隔离 Go overlay 反例；Scope: session parser、持久化路径及新增回归。
Reviewed state: HEAD `6f22e76`，session.go blob `786789b79abefac28471f83580c5e83b3df4ba07`，
test blob `e4d614d280f30556790e4cc984a690e2d225797a`。
主 agent 已验证反例。阻断后停止 broad review。初版全套唯一失败为 synthetic
snapshot golden（canonical 元数据恢复后不再漏掉 model）；将核验差异并更新
该既有 fixture。此前 race/vet 通过不覆盖修复后的新内容。
Completion gate: FAILED（Round 1 精确状态已写入并查询 CEv1）。不得依据本轮交付完成。

## Review — Round 2

## 📋 独立复评

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

`SPA-R2-F1` / P1 / `internal/session/session.go:762`：晚到 cwd 升级为用户已排除
的项目时，追加发布只跳过新结果，没有移除旧 fallback 投影；同一日志/排除，
追加扫描仍有一条搜索结果，完整扫描为零。风险是已有排除规则未按真实归属生效。
证据：冷上下文 reviewer overlay，主 agent 独立重跑失败于 appended=1/full=0，
`/private/tmp/agentdeck-spa-r2-f1-red.log`。
修复：在同一个 source cursor 事务内，仅删除该 source/client/session 的旧投影，
保留原始日志、cursor context、usage 和排除；删除失败必须回滚全部状态。
`SPA-R2-F1 -> repaired in candidate`，待独立复评。共享与普通回归均通过
`/private/tmp/agentdeck-spa-r2-green.log`，包括发布失败回滚与完整扫描一致性。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

`SPA-R1-F1 closed`：保留 cursor 解析上下文及优先级，原 RED/GREEN 和新增
append/full parity 在共享/普通路径通过。升级测试保留旧索引、排除和 core DB。

### 📝 总结

Reviewer: `/root/cold_review`，延续独立只读角色；Method: 原反例核验、源码复评、
隔离追加/完整扫描 exclusion overlay；Scope: 新 parser context、缓存列升级、
持久化/排除与测试，未执行真实用户数据库操作。
Reviewed state: HEAD `6f22e76`；session blob `bcc0e8c3cb0d307f5e8fc0b4aa733577ec2fe963`，
store blob `c2ed3e11fd78b284312151805932361aa2d4d00d`，test blobs
`4f06267fddbd563afcb16e570800814a59d945c2` / `6d0761638cac84e25bbcb0939f7cc54a6461303d`。
本轮阻断后停止 broad review；此前候选全套、race、vet、whitespace 和两架构 build
通过，不作为 R2 修复后精确内容的全部验收。
Completion gate: NOT_VERIFIED。

## Review — Round 3

## 📋 独立复评

📊 总体评分：8/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

无 P1。

### 🟡 建议改进 — 必须修复

`SPA-R3-F1` / P2 / `internal/session/session.go:594-637`：从已排除 fallback
项目升级为允许 cwd 时，append-only 没有重新读取原本未入库的正文。相同日志
和排除，incremental=0/full=1；会话恢复身份后正文不完整。
证据：独立 overlay 反例由主 agent 重跑，
`/private/tmp/agentdeck-spa-r3-f1-red.log` 以预期差异失败。
修复：append eligibility 检查此前真实归属是否已排除；若是，则共享计划及实际
解析都从零重读，在按源事务中重建允许的投影。
`SPA-R3-F1 -> repaired in candidate`，待独立复评。
普通/共享的两个方向转换及失败回滚均通过；日志
`/private/tmp/agentdeck-spa-r3-green.log` 包含 session/store 全包。

### 🟢 优点

`SPA-R1-F1 closed`，`SPA-R2-F1 closed`：优先级和允许→排除转换已通过原反例
及对应共享/普通 regression。排除转换删除仅限 exact source/client/session。

### 📝 总结

Reviewer: `/root/cold_review`；Method: 只读源码、独立隔离反方向 exclusion parity。
Scope: 同 issue 修复、追加范围、发布事务、缓存升级和既有 fixture。
Reviewed state: HEAD `6f22e76`；session blob `3541c0b104ed4d1ce953af5b3eca011414089600`，
test blob `237fad6098bf5d9bcf98d426b595945db3b53e9d`，其余源同 Round 2。
阻断后停止 broad review。
Completion gate: NOT_VERIFIED。

## Review — Round 4

## 📋 独立复评

📊 总体评分：8/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

无 P1。

### 🟡 建议改进 — 必须修复

`SPA-R4-F1` / P2 / `internal/session/session.go:1027`：日志旧路径被精确排除，
重命名为允许的新路径后追加，append eligibility 用新路径检查旧投影资格，
只发布新回复，遗漏此前排除的 prompt；相同 raw bytes/exclusions 为 incremental=1/full=2。
主 agent 独立重跑失败，`/private/tmp/agentdeck-spa-r4-f1-red.log`。
修复：计划和解析使用 cursor 的原始 source_path 与其目录 fallback 判定旧投影
是否排除；若是，完整 replay。`SPA-R4-F1 -> repaired in candidate`，待复评。
新增共享/普通、exact path/directory-project 四种 rename+append parity 回归。

### 🟢 优点

`SPA-R1-F1 closed`，`SPA-R2-F1 closed`，`SPA-R3-F1 closed`。
两方向 cwd/exclusion 转换及事务失败回滚通过；不会修改用户的排除政策。

### 📝 总结

Reviewer: `/root/cold_review`；Method: 独立只读增量/重命名边界检查和 overlay 反例；
Scope: canAppendSession、shared plan、移动源及 cursor 事务。
Reviewed state: HEAD `6f22e76`；session blob `6ec380022e36734f1c0d87d501d4c15e63ea49de`，
test blob `d3da46607f2e686128f3a17e1189c5ffd4422517`，其余源同 Round 2。
阻断后停止 broad review。
Completion gate: NOT_VERIFIED。

## Review — Round 5

## 📋 独立复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `SPA-R1-F1 closed`：cursor 保留 cwd 优先级，追加/完整扫描一致。
- `SPA-R2-F1 closed`：允许→排除时原子移除精确 source/client/session 投影。
- `SPA-R3-F1 closed`：排除→允许时完整 replay 原正文；shared plan 与 reducer 一致。
- `SPA-R4-F1 closed`：rename+append 按原始 state.path/目录判定旧投影资格。
- 共享/普通路径有双向转换、rename、旧 v5 恢复、失败回滚与数据保留保护。

### 📝 总结

Reviewer: `/root/cold_review`；Method: 初始 `fork_turns=none` 冷上下文独立角色，
持续只读复评、四个隔离 overlay 反例最终合并通过。主 agent 逐一独立确认原失败，
源码修复与仓库回归结果；reviewer 未修改产品、测试、记录、Beads、CEv1 或 Git。
Reviewed state: HEAD `6f22e76`，session blob `cad440f48dee2b03b87cbe61094a9155aa3e859d`，
test blob `70307201ee175ad399a1617712dd59bd38f65277`，store/test blobs
`c2ed3e11fd78b284312151805932361aa2d4d00d` / `6d0761638cac84e25bbcb0939f7cc54a6461303d`。
Scope: canonical 提取、append provenance、两方向排除转换、重命名 source 归属、
事务回滚、cache 列升级、相关测试、契约和仅 model 的 golden 差异。
Evidence: `/private/tmp/agentdeck-cold-review/final-repros.log` 的四个独立反例 PASS，
及 `/private/tmp/agentdeck-spa-r4-green.log` 的 session/store 包 PASS。
无新增发现。限制：隔离日志/SQLite；未重建真实用户数据库，未验收部署二进制，
未改变或重新评审 usage parser。最终 broad checks 已通过，详见验证。
Completion gate: VERIFIED；精确 candidate:round5，4/4 required criteria，
missing/invalidated/unresolved 均为空；历史失败状态保留。

### Task checkpoint

任务 `fix:session-project-attribution-observer-noise` 无 containing topic gate。
提交建议：使用用户已有授权，提交此 issue 的七个命名文件及完整审计历史。
推送建议：实际 SSH 签名/提交内容核验、immutable CEv1 绑定后按已有授权交付。

Commit checkpoint contributors

- staged_tasks: `ad-bug-session-project-attribution-observer-noise`
- included: `codex` — implementation/tests/repair/carrier — Beads comments
  `a59f8d98`、`d6ce2900`、`f53909e1`、`29247d8d`、`f2ed9b8f` 的贡献记录。
- excluded: `/root/cold_review` — review-only，无生产/测试写入；完整报告由主 agent
  验证后记录。同为 Codex actor，trailer 去重。
- trailers: `Co-Authored-By: Codex <noreply@openai.com>`
- unresolved: none
