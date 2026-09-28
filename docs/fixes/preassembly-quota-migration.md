---
status: active
created: 2026-09-28
---

# 缺陷：预组装 quota schema 阻断 RC1 core 迁移

## 现象

正式安装的 `v0.6.0-rc.1` App 在本机首次刷新后显示“首次刷新失败 · 还没有可显示的
数据”。会话扫描进度可以到达 completed，显示 4,016 个来源中提交 1 个、跳过
4,015 个，但刷新没有进入 desktop snapshot。

统一日志显示 App 约每 90 秒重试；失败始终位于 `global_scan_stream`，退出码 1、
`error_code=runtime_error`，无超时或输出截断。只读数据库检查显示
`schema_metadata=28`，`quota_windows`、`quota_envelopes` 和
`quota_alert_notices` 已存在，但 migration 30 的 `failure_observed_at` 尚不存在；
`PRAGMA quick_check` 为 `ok`。

## 根因

本机数据库曾被未发布的 subscription-quota 开发分支打开。该分支把
`quota_alert_notices` 定义为 migration 26；组装到 v0.6.0 后，24–26 已被其他主题
占用，同一张表被重新编号为 migration 29。本机因此保留“最终 migrations 1–26 +
旧编号 alert 表”的混合状态。

RC1 打开 core 时，migration 27 和 28 各自提交成功；migration 29 再执行
`CREATE TABLE quota_alert_notices`，因同名表已存在而回滚。core 域失败，但独立的
sessions 数据库仍能完成扫描，所以 UI 同时显示 completed 进度和整体刷新失败。
正式 v0.5.0 的 schema 是 23；该缺陷针对历史预发布开发状态，不改变稳定版升级
来源的结论。

## 修复边界

- migration 29 改为事务内的“创建或严格接管”：对象不存在时创建；存在时逐列核对
  名称、类型、NOT NULL、默认值和复合主键顺序。
- 只有与旧开发 migration 26 完全相同的表才可接管并保留数据；视图、额外/缺失列、
  类型、默认值或主键不匹配均返回专用 fail-closed 错误。
- 不使用宽松的 `CREATE TABLE IF NOT EXISTS`，不手工修改用户数据库，不改变最终
  schema 30、wire 或产品行为。

## 验证

- RED：
  `scripts/run-go-test.sh ./internal/store -run 'TestMigrations(AdoptExact|RejectMismatched)PreassemblyQuotaAlertTable$'`
  exit 1；精确旧表报 migration 29 已存在，不匹配表没有安全分类。
- GREEN：同一命令 exit 0；精确旧表的数据保留并迁移到 schema 30，不匹配表被明确
  拒绝。
- `scripts/run-go-test.sh ./internal/store` exit 0，覆盖 fresh database 和既有迁移回归。
- `scripts/run-go-test.sh ./...` exit 0；`bash scripts/check-topic-docs.sh`、
  `make check-whitespace` 与 `git diff --check` 均 exit 0。
- 独立评审与最终内容状态证据待实现内容冻结后完成。本轮未对真实
  `~/.agentdeck` 执行迁移或写入。

## Review — Round 1 — 2026-09-28

- Reviewed state：HEAD `5aa47c98336f1e18e3b77a3e687e8cf6c814ab36`；实现 diff
  SHA-256 `068bb4755cad61371e10e9f9fb15e46514bba4a03d0f0bb18028ce7281bf51ae`；
  `internal/store/migrations.go` blob `ac1687a2925b134ea3f65418d9b18ad51b866e9e`，
  `internal/store/store_test.go` blob `9cd1c3f76c1afc47f1b5593dc891757883cd2784`，
  本记录评审前 blob `91bf9cf42b2f7be1fdd0b98ac04b89254cf89c8f`；候选 ContentState
  `fix:preassembly-quota-migration:candidate:696de16f9045a35fc85b4348d463d9484944655b2e260f638575829abe49c649`。
- Reviewer：Codex（单 agent、默认模型层级的独立代码与测试评审）。
- Method：先用工作区绑定的 CodeGraph 检查 migration 29、`migrate` 的事务与
  `schema_metadata` 更新路径，再检查精确 task diff；复用同一 ContentState 上已经通过的
  focused、store-wide、full-Go 与 L0 evidence。针对 schema 等价判定另用内存 SQLite
  构造表级 `CHECK` 约束的最小反例；反例成立后按评审规则停止宽泛验证。
- Scope：migration 29 对旧 `quota_alert_notices` 的创建/接管、数据保留、非规范对象拒绝、
  schema 版本推进，以及两项新增迁移回归。真实 `~/.agentdeck`、手工状态迁移、桌面 UI
  与无关 schema 行为不在本轮范围内。

### 📋 评审报告：fix / preassembly-quota-migration

📊 总体评分：5/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`internal/store/migrations.go:356`](../../internal/store/migrations.go#L356) `R1-F1` `[P1]`：
`ensureQuotaAlertNotices` 只比较 `PRAGMA table_info` 的列元数据，却据此把现有对象判定为
“精确旧表”。SQLite 的表级 `CHECK`、额外 `UNIQUE` 等约束不出现在该 PRAGMA 中；带有
这些非规范约束的表会通过接管、把 `schema_metadata` 推进到 30，随后仍可能拒绝合法
notice 写入。

- 行为风险：数据库会被永久标记为当前 schema，但 `quota_alert_notices` 的写入契约并不
  是 migration 29 的规范契约；后续打开不会再次校验，合法 quota notice 可能持续写入失败。
- 证据：内存 SQLite 中，规范表与额外包含 `CHECK (client = 'claude')` 的表产生完全相同的
  六行 `pragma_table_info` 输出，正好满足当前逐列比较；向后者写入合法 `client='codex'`
  notice 则以 `CHECK constraint failed` 失败。`sqlite_master.sql` 明确显示该额外约束。

💡 有界修复：让接管判定覆盖会改变表语义的 schema 元素，并增加至少一个“列元数据相同、
但存在额外表约束”的失败优先回归；该反例必须返回
`errIncompatibleQuotaAlertNoticesSchema`，而规范旧表仍需保留数据并迁移到 schema 30。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- migration 29 的 apply 与 schema 版本更新处于同一事务；不匹配返回时会回滚本轮版本推进。
- 精确旧列布局的回归覆盖数据保留并继续执行 migration 30，原始 RC1 碰撞路径得到直接保护。
- 修改保持在迁移 seam、store 回归和一个 Lane A 记录内，没有触碰真实用户状态或扩大产品契约。

### 📝 总结

评审对象由上述 HEAD、diff、三个 blob 与 ContentState 唯一标识。原始预组装表可以被接管，
既有验证也与该状态绑定；但“严格精确接管、其余 fail-closed”仍存在一个可复现的 schema
等价性漏洞，因此本轮不能通过。未对真实 `~/.agentdeck` 执行迁移或写入。

- Findings：`R1-F1` -> open；修复范围仅为非规范表级约束识别与对应回归。
- Evidence：复用同状态 legacy-adoption、implementation-verification evidence；新增内存
  SQLite 决定性反例推翻 mismatch-rejection 与 independent-review 完成主张。
- Completion gate：`FAILED`。
- Verdict：FAIL。

## Repair — Round 1 / R1-F1 — 2026-09-28

`R1-F1` repaired：migration 29 现在同时要求现有对象通过两层核对：

- `sqlite_master.sql` 与规范 `CREATE TABLE quota_alert_notices` 仅折叠空白后必须完全一致，
  因此额外 `CHECK`、`UNIQUE` 或其他表级语义都会拒绝接管；
- 原有 `PRAGMA table_info` 逐列核对继续验证名称、类型、NOT NULL、默认值和复合主键顺序。

只有未发布开发 migration 26 使用的同一份规范 DDL 可以被接管。对象不存在时仍创建
规范表；对象类型、DDL 或列元数据不匹配均返回
`errIncompatibleQuotaAlertNoticesSchema`，本轮 schema 版本更新随事务回滚。

回归扩展为三个非规范状态：缺列、列元数据相同但增加表级
`CHECK (client = 'claude')`、列元数据相同但增加 `UNIQUE (notified_at)`。修复前后证据：

- RED：`TestMigrationsRejectMismatchedPreassemblyQuotaAlertTable` 的额外 CHECK 和
  额外 UNIQUE 子用例错误返回成功；
- GREEN：focused 精确接管/不匹配测试全部通过，规范旧表继续保留数据并到达 schema 30；
- `scripts/run-go-test.sh ./internal/store` 与 `scripts/run-go-test.sh ./...` 均通过。

Round 1 的 FAIL 保持历史不变；修复候选等待独立复评。未对真实 `~/.agentdeck`
执行迁移或写入。

## Re-review — Round 2 — 2026-09-28

- Reviewed state：HEAD `5aa47c98336f1e18e3b77a3e687e8cf6c814ab36`；实现 diff
  SHA-256 `8bb56098541618110e6427b702e52d19bfe19736b2f6617adce6bbb8da8e757c`；
  `internal/store/migrations.go` blob `e9042140da319f50246eed115e9b7f96df85228b`，
  `internal/store/store_test.go` blob `1c4c0fc32108ea1333e2ffc210d12c0951ac30eb`，
  本记录复评前 blob `539a0e6cadf555eab49b09d72e8f4ab0e6c37dd4`；ContentState
  `fix:preassembly-quota-migration:repair-r1:7ab9491d06259a6f56356e73dd68a929f18f032a6522fd071741c179acbaa2c4`。
- Reviewer：Codex（单 agent、默认模型层级的独立流程复评）。
- Method：只复核 Round 1 的 `R1-F1`。使用工作区绑定的 CodeGraph 定位最终生产路径，
  检查精确修复 diff 与回归；再从 Git 历史提交 `dbd119fe7c6fe4ae8e5485ce2b94fa243913100d`
  核对未发布 migration 26 的原始 DDL。复算当前 diff 与三个 blob，确认与 Repair handoff
  和 CEv1 ContentState 完全一致；复用该状态上 focused、store-wide、full-Go 与 L0 evidence，
  未因阶段变化重复执行相同测试。
- Scope：`R1-F1` 的表级 schema 语义等价判定、CHECK/UNIQUE 反例、规范旧表接管。
  其他迁移、真实 `~/.agentdeck`、桌面 UI 和无关产品行为不在本轮范围内。

### 📋 复评报告：fix / preassembly-quota-migration

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `R1-F1` **closed**：现有对象必须同时满足规范化 `sqlite_master.sql` 完全相等和原有
  `PRAGMA table_info` 逐列相等；额外 CHECK、UNIQUE 或其他 DDL 语义不再被接管。
- 历史 migration 26 的原始 `CREATE TABLE quota_alert_notices` 与当前规范常量一致，
  空白折叠不会把另一个历史契约误当作兼容，也不会拒绝该真实来源。
- CHECK 与 UNIQUE 两个回归都经 `Open` 进入生产 migration 29 路径，并断言专用
  `errIncompatibleQuotaAlertNoticesSchema`；规范旧表仍保留数据并到达 schema 30。
- Repair handoff 的 diff、源码 blob、测试 blob 和评审前记录 blob 均已独立复算一致；
  `git diff --check` 通过，真实用户状态未被读取或迁移。

### 📝 总结

`R1-F1` 已在新的精确 ContentState 上关闭，未发现回归或新阻塞 finding。复评对象由上述
HEAD、diff、三个 blob 与 ContentState 唯一标识；同状态验证证据保持有效。剩余不确定性
仅限未在真实 `~/.agentdeck` 上执行迁移，这是本 Lane A 明确排除的破坏性操作，不影响
通过由 fixture 精确重建的历史 schema 判定修复正确性。

- Finding dispositions：`R1-F1` closed；still-open、regressed、new 均为零。
- Evidence：复用同状态 legacy-adoption、mismatch-rejection、implementation-verification；
  本轮独立源码、历史 DDL、回归路径与内容身份复核通过。
- Completion gate：`VERIFIED`；四项 required criteria 均有当前 ContentState 的
  applicable pass evidence，无 missing、invalidated 或 unresolved 项。
- Verdict：PASS。
