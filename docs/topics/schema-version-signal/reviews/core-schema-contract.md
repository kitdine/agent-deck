---
status: active
topic: schema-version-signal
subject: core-schema-contract
---

# Core Schema Contract — Review

## Round 1 — 2026-09-08

## 📋 Task 1 评审报告

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CSC-R1-F1 — 中：锁失败后的 WAL 探测创建 sidecar。**

位置：`internal/store/store.go:225`；覆盖缺口位于
`internal/store/store_test.go:1548` 的 held-lock fixture。

- 行为风险：`schemaAheadAtRest` 的 `mode=ro` 连接仍会为已关闭且没有
  sidecar 的 WAL 数据库创建 `agentdeck.sqlite3-wal` 和
  `agentdeck.sqlite3-shm`。未取得 state lock 的拒绝路径因此写入了状态目录，
  违反 Task 1 明确要求的“没有数据库修改或 sidecar 创建”。
- 证据：使用当前 vendored SQLite 驱动、相同 DSN 和查询可直接复现；进一步
  编译当前 CLI，在合成 schema 99 数据库旁放置由存活进程持有的合法 state.lock，
  执行 `--state-dir <fixture> --format json provider list`，退出 1 且返回
  `schema_ahead`（99/23），同时创建上述两个文件。DELETE journal 对照组没有创建。
  产品正常 read-write open 会设置 WAL（`internal/store/store.go:270`）；新增
  测试只创建默认 DELETE journal 数据库，因此没有验证这个实际存储模式。
- 💡 修复范围：修正 Task 1 的 lock-failure probe，确保 WAL 场景也不创建或
  修改 sidecar；补充 WAL 无 sidecar、已有已提交 WAL 内容的回归，保持读取已提交
  版本、错误回退及取消边界。不要通过忽略可能含最新版本的 WAL 或删除其他进程的
  sidecar 满足断言。修复后执行 Task 1 已规定的定向、全 Go、store race 和 vet 检查。
- 处置：OPEN；仅交接修复，本轮不修改产品代码或测试。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

typed error 保留版本对与 errors.Is/errors.As；doctor 使用自己的输出路径，
将 partial 放在 envelope，并保持 upgrade prose 不进入 recovery_command。

### 📝 总结

- Reviewer：Codex。
- Method：独立于实现方的当前源码/diff 审阅、契约核对、合成 SQLite 对照实验，
  以及当前 CLI 的 held-lock 重现；未委派。worktree 无 `.codegraph/`，使用限定源码检查。
- Scope：Task 1 的九个 Go 生产/测试文件；任务状态文档仅用于范围和交接。
- Workspace：`agent-deck.schema-version-signal`，`feature/schema-version-signal`。
- Reviewed state：HEAD `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`；九个 Go
  文件的 `git diff --binary -- <sorted scoped paths>` SHA-256 为
  `d21aa25333a0bcb9158af4737f90b776bf59308c16b75daa96a864cb6ddc2751`。
  本轮使用明确收窄到产品与测试的状态，避免评审/状态文档更新改变受测身份；没有将其
  冒充实现方包含 tasks.md 和 status 行的 `cff5df8e...` 候选指纹。
- 环境：`go version go1.27.1 darwin/amd64`，`-mod=vendor`，
  `GOCACHE=/private/tmp/agent-deck-go-build`。
- 依赖 SHA-256：go.mod `b702f0c0cd98a623e9591401d7c69e121912287c2d11691c33865dbfa801dc4c`；
  go.sum `343429c5d5a5ff09170bab9aea46b0910312c29e8dabae84f7156290d98840b7`；
  vendor/modules.txt `d8f450522dbc13b563b4384c5a68b50efd6e47d3d0ffd898c0179f21809b9cd2`。
- Evidence：`go build -mod=vendor -o <temporary-binary> ./cmd/agentdeck` 成功；
  `go run -mod=vendor <temporary-probe.go> <temporary-binary>` 重现 CSC-R1-F1。
  probe SHA-256 为 `8e3951ec78150487505fa8a224518c50d4529f6ac0e2221f24822a1ebec57fdd`。
  这是合成状态下的真实 CLI 验证，不是用户数据库或真实客户端 Hook 验收。
  确认阻塞后未扩大运行全套测试。
- CEv1：实际 WorkUnit 为
  `urn:ce:agent-deck:work-unit:schema-version-signal-core-schema-contract`。
  对实现方 `urn:ce:agent-deck:state:workspace:cff5df8eca71482d018330b44235ee48a0d9f4cde1334b9801ceecdd40df4390`
  执行固定 `gate-status.cypher` 得到 NOT_VERIFIED：五条 pass 证据均 malformed，
  其 work_unit_id 写成业务 scope 而非该 WorkUnit ID。历史测试通过记录保留，
  不能复述为当前正式门禁 VERIFIED。修复验收需追加正确绑定的新证据，不能覆盖旧事实。
- Completion gate：FAILED；本轮反例绑定产品范围状态，针对
  `lock-precedence-read-only-probe` 追加失败证据。其余门禁缺口不因本轮 FAIL 自动补齐。
  固定 gate 查询已确认 FAILED；状态 ID 为
  `urn:ce:agent-deck:state:review:core-schema-contract:d21aa25333a0bcb9158af4737f90b776bf59308c16b75daa96a864cb6ddc2751`。
  新建两节点和两关系已回读/门禁确认，关系预检 2/2 ok。
- 文档检查：`make check-whitespace`、`bash scripts/check-topic-docs.sh`、
  `git diff --check` 通过。直接执行 topic checker 因文件执行权限失败，改由 Bash
  运行后通过；未修改脚本权限。验证后的产品 diff 指纹仍为上述 d21aa253...。
- 交接：`ad-svs-core-schema-contract-dev` 已退回 `in_progress`、`round-1`；
  topic 状态及 canonical main 的跨 topic 摘要注明未合并分支上的修复待办。
  临时探测源、二进制和合成数据库在记录重现步骤与结果后清理。
- 结论：CSC-R1-F1 阻塞 Task 1；其余五个 Task 未进入本轮。未提交或推送。

### 下一步指令

修复：schema-version-signal / core-schema-contract / CSC-R1-F1

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal

## CSC-R1-F1 修复 — 2026-09-08

- 修复者：Codex；仅处理 CSC-R1-F1，未执行复评或交付。
- 处置：repaired in candidate，等待独立复评；Round 1 FAIL 保留。
- 修改：`store.go` 的锁失败探测改用 `schema_probe.go` 的私有临时副本。
  源数据库、WAL 和 journal 仅以文件只读方式打开；SQLite 仅在副本上恢复 WAL
  和查询元数据，不打开源 SHM。副本目录 0700、文件 0600，返回前清理。
  两轮读取比较文件身份、大小、修改时间与 SHA-256；变化、非空 rollback journal、
  读取失败或取消均回退原锁错误。没有忽略已提交 WAL，也没有删除源 sidecar。
  复制开销随数据库及 WAL 大小增长；分块读取检查调用方 context。
- 回归：新增 `schema_probe_test.go`，覆盖关闭 WAL 无 sidecar，以及活跃 WAL
  中已提交 99、主库 23、未提交 100 的场景。断言仍返回 99，并比较源 DB、WAL、
  SHM、journal 的存在性和全部字节。旧实现的两个子测试均因源文件变化失败；
  修复后的测试通过。既有取消、DELETE journal、支持版本、缺失、损坏元数据和
  malformed 数据库回退测试继续通过。
- 内容：HEAD `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`；产品指纹
  `5c94fd20d833fc19cca49c491513b4a6ee49f1845920afbeb7bd7e68ccaf8581`。
  配方为 Round 1 的九文件 `git diff --binary`，按顺序追加新文件
  `internal/store/schema_probe.go`、`internal/store/schema_probe_test.go` 的
  `git diff --no-index --binary -- /dev/null <path>` 输出，再取 SHA-256。
  文档不在产品指纹内；新增文件因此也被纳入，未复用旧九文件指纹。
- 验证：定向 store/doctor/CLI 测试、全 Go（含 desktop 投影测试）、store race、
  `make vet` 均通过。使用项目 runner、vendored 依赖和 Go 1.27.1 darwin/amd64；
  go.mod、go.sum、vendor/modules.txt 与 Round 1 的摘要相同。
  原失败日志 `agentdeck-go-test.u8pYIf`；修复定向日志 `agentdeck-go-test.2d7APJ`、
  `agentdeck-go-test.Q8D0g4`；全量 `agentdeck-go-test.o5iYtZ`；race
  `agentdeck-go-test.WfEJl6`，均位于本机 TMPDIR，保留供交接。
  这些是合成状态下的 Go 回归，不是用户数据库或真实客户端 Hook 验收。
- CEv1：新目标为
  `urn:ce:agent-deck:state:repair:core-schema-contract:mDcPeB0DcdYYSnvY`。
  首次固定 gate 查询为 NOT_VERIFIED（新目标尚未记录）；旧五条 malformed 证据
  与 Round 1 fail 原样保留。已用正确 WorkUnit ID 追加五条新观察；固定 gate
  查询确认 VERIFIED（5/5），missing/invalidated/unresolved 均为空。新节点 6/6、
  关系预检 10/10 ok，写入关系回读 10/10。
- 文档验证：`make check-whitespace`、`bash scripts/check-topic-docs.sh`、
  topic 与 canonical main 的 `git diff --check` 均通过。两处状态摘要均注明
  未合并分支上的修复已实施、等待复评；Task 1 Review 仍未勾选。

### 修复交接指令

复评：schema-version-signal / core-schema-contract

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal

## Round 2 — 2026-09-08

## 📋 Task 1 复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**CSC-R1-F1：CLOSED。** `internal/store/store.go:224` 先建立私有副本，
SQLite 仅打开副本；`schema_probe.go` 通过只读文件句柄复制源 DB/WAL/journal，
不打开源 SHM。临时目录 0700、文件 0600，成功和失败路径均安排清理。
因此上一轮直接 SQLite 打开源 WAL 而创建 sidecar 的路径已移除。

`schema_probe_test.go:13` 同时覆盖关闭 WAL 和活跃 WAL：主库 23、已提交 99、
未提交 100 时返回 99，逐字节比较源 DB/WAL/SHM/journal 及存在性。
修复前两个子场景失败，当前全 Go 与 store race 日志均通过；已有取消及
supported/missing/malformed/damaged 回退检查继续通过。

### 📝 总结

- Reviewer：Codex；Method：与修复执行分开的复评角色，直接检查当前源码、
  新增测试、原始回归日志和 CEv1 血缘；未委派。不声称完全冷上下文评审。
- Scope：CSC-R1-F1、其修复引入的副本生命周期、版本读取、失败回退，以及
  Task 1 当前产品/测试状态。没有扩展到 Tasks 2–6。
- Finding disposition：本记录唯一问题 CSC-R1-F1 已关闭；没有新阻塞问题。
- Reviewed state：HEAD `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`，产品指纹
  `5c94fd20d833fc19cca49c491513b4a6ee49f1845920afbeb7bd7e68ccaf8581`。
  现场按修复节的九文件 diff 加两个新增文件 diff 配方重算一致；依赖三项摘要及
  Go 1.27.1 darwin/amd64 与修复证据一致。评审及状态文档不属于该产品指纹。
- Evidence：现场核验全 Go 日志 `agentdeck-go-test.o5iYtZ` SHA-256
  `c9c30c84155efd99e156bfcb61fb2d11085bfacf9620fc637146a9366655ca78`，
  store race 日志 `agentdeck-go-test.WfEJl6` SHA-256
  `f946fa502c102542049b49e97ae16eac67a1e2ef57ebf20ee6a038551204e0cd`，
  均与 CEv1 原始证据相符；核对修复前 `agentdeck-go-test.u8pYIf` 的失败断言。
  复用修复阶段定向/全 Go/store race/vet 证据，未因阶段切换重跑产品测试。
- Completion gate：VERIFIED（5/5）。使用当前固定 `gate-status.cypher` 查询
  WorkUnit `urn:ce:agent-deck:work-unit:schema-version-signal-core-schema-contract`，
  target `urn:ce:agent-deck:state:repair:core-schema-contract:mDcPeB0DcdYYSnvY`。
  五条修复证据 target_matches/applicable 均 true、malformed 均 false；
  missing/invalidated/unresolved 均为空。历史失败和 malformed 证据仍保留，
  不适用于当前目标。没有为复评重复创建观察或重标记历史证据。
- 限制：副本成本随 DB/WAL 大小增长，调用方取消在读取块之间检查；不声称固定
  时间或空间上限。WAL 回归是合成状态验证，不是本轮真实客户端 Hook 或原生 UI
  验收；后续任务继续承担其对应边界。没有发现需要本 Task 增补的阻塞问题。
- 文档验证：本轮状态同步后运行 `make check-whitespace`、
  `bash scripts/check-topic-docs.sh` 和 `git diff --check`，均通过；canonical main
  的状态投影 diff 检查通过。Beads 已转为 awaiting_commit、round-2。

Task checkpoint：ad-svs-core-schema-contract-dev；content_state=5c94fd20d833fc19cca49c491513b4a6ee49f1845920afbeb7bd7e68ccaf8581；gate=VERIFIED。

提交建议：按一个 Task 提交 topic 分支的 11 个产品/测试文件及本评审记录、tasks.md、docs/status.md 对应状态；canonical main 的 docs/status.md 投影单独处理，勿与 feature 分支混合暂存。提交仍需明确授权。

推送建议：授权提交并核验提交对象、签名、归属 trailer 和准确内容证据后，另行授权推送 feature/schema-version-signal 至选定远端同名分支；远端目标尚未现场核验，不直接推送 main。

### 下一步指令

开发：schema-version-signal / hook-refusal-lifecycle

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal
