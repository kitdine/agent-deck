---
status: active
plan: desktop-app
task: desktop-wire-contract
created: 2026-08-13
updated: 2026-08-13
---

# Review log — desktop-app / desktop-wire-contract

## 📋 Round 1 — 2026-08-13

📊 总体评分：7/10

✅ 结论：FAIL

- 已评审状态：`3b709a8` 加未提交的 17 文件 Task 候选；评审范围 SHA-256
  `f11ef5b48f123e118cf6a5ea765cd413da138f9783df2cba07bef0250bfc9e25`.
  与任务无关的 `AGENTS.md` 工作区指导变更不在候选范围内。
- 评审者：Codex，独立只读 Review。
- 范围：`desktop-wire-contract` Go 命令与聚合、只读 store 访问、JSON/error
  合同、Go/Swift fixtures、GUI JSON registry、living specifications、计划状态与
  文档索引。

### 🔴 严重问题（必须修复）

[docs/specs/cli-design.md:1713; internal/desktop/desktop.go:147-159; internal/session/doctor.go:19-32] 稳定 desktop 合同承诺每次 snapshot “creates no state”，但针对健康既有数据库的一次普通 snapshot 会创建 SQLite WAL sidecar。

- 行为风险：文档化的 filesystem 合同不真实。依赖“不创建”承诺设计的 desktop host 或 sandbox，会在成功刷新后观察到新产生且持久存在的 `agentdeck.sqlite3-wal`、`agentdeck.sqlite3-shm`、`sessions.sqlite3-wal` 和 `sessions.sqlite3-shm`。
- 证据：在隔离 state root 中执行并关闭 `state migrate` 与 `session rebuild` 后，snapshot 前文件清单没有任何 `-wal` 或 `-shm`。当前候选的一次 `desktop snapshot` 退出 `0`，随后文件清单出现全部四个 sidecar。现有 `TestCheckHealthCompatibleIndexReportsOKWithoutMutatingTheDatabaseFile` 也通过，并明确要求 quick health 后存在 session sidecar。

💡 有界修复：选择并编码一个真实边界：要么让 desktop read/health 路径在受支持的并发写入语义下避免创建文件；要么把合同收窄为“不创建数据库或改变已提交内容”，并明确允许 owner-only SQLite sidecar。增加一个断言所选 filesystem 行为的 `desktop snapshot` 回归测试。

[docs/README.md:57; docs/specs/cli-design.md:4] 本 Task 已把 living specification 升至 version 25，但权威文档索引仍标示 CLI Design version 24。

- 行为风险：从项目索引进入的读者会获得过期的合同身份，Review/status 来源因此无法就哪一版 desktop wire contract 具有权威达成一致。
- 证据：候选把 `docs/specs/cli-design.md` frontmatter 改为 `version: 25`，而 `docs/README.md` 仍写着 `Active, version 24`。

💡 有界修复：在 Repair 候选中把索引同步为 version 25，并在 Re-review 前保持 Task Review cell 未勾选。

### 🟡 建议改进（推荐）

[internal/session/session.go:1023-1037; internal/desktop/desktop.go:262-271] `session.List` 把全部可见 session 结果载入内存之后，recent sessions 才被截断。

- 证据：SQL query 没有 `LIMIT`；`sessionsSnapshot` 随后才把返回的 Go slice 截到 `recent-limit`。

💡 有界改进：增加只读、有界的 metadata query，返回 total 加最多 `recent-limit` 行；或者明确记录为什么五分钟 desktop refresh 边界可以接受全索引获取。

[internal/desktop/desktop_test.go:40-67; internal/desktop/desktop.go:297-309] Privacy 回归覆盖直接验证了 provider 和 session redaction，但没有验证 health recovery metadata。

- 证据：`healthSnapshot` 会转发 doctor recovery command，而 `TestSnapshotsRedactPrivateDomainFields` 只 marshal provider 和 session DTO。

💡 有界改进：增加 synthetic doctor report 断言：health output 在保留获准安全 recovery command 的同时，不能携带 secret value、credential reference、endpoint、path 或 raw configuration。

### 🟢 优点

- 命令暴露专用 JSON-only、独立 versioned DTO，并有稳定 typed input error 与 partial-section warning。
- Provider/session redaction 是显式 DTO 转换，不依赖 domain-object JSON tag；空 collection 已归一化，Go 与 Swift 共享 canonical fixtures。
- Swift verifier 会验证 envelope identity 并拒绝不支持的 wire version；GUI JSON contract 已包含新 leaf command。
- 已检查的聚合路径不会扫描 source、迁移、改变权限或发起网络请求。
- `./internal/desktop`、`./internal/store` 和 `./cmd/agentdeck` 的独立检查通过；Swift fixture verifier 通过两份 canonical fixture；`git diff --check` 通过。

### 📝 总结

已评审的 `desktop-wire-contract` 候选结构完整，contract coverage 较强；但当前 CLI 与 living specification 的 filesystem 承诺相矛盾，且文档索引报告了错误的 specification version，因此不能通过。本轮 Review 未修改 product code、tests、configuration 或两个 blocker；Review 保持打开，等待 Repair 与独立 Re-review。

修复：对齐 desktop snapshot 的 SQLite sidecar/“creates no state”合同并增加所选边界的回归覆盖，同时将 `docs/README.md` 的 CLI Design 索引版本同步为 25。

## 🔧 Round 1 Repair — 2026-08-13

- 已修复严重问题 1：`docs/specs/cli-design.md` 现在将 filesystem 边界准确限定为不创建缺失 state root/database、不迁移、不改权限、不改已提交 SQLite 内容且不联网；对已有 WAL-mode core/session 数据库，为保持正确并发读取，允许 materialize owner-only `-wal`/`-shm` sidecar。
- 回归覆盖：`TestBuildReadsCompleteIsolatedSnapshot` 在 snapshot 前确认四个 sidecar 均不存在，记录两个主数据库 SHA-256；调用 `Service.Build` 后确认两个主数据库 digest 未变，并断言四个新 sidecar 均存在且权限为 `0600`。
- 已修复严重问题 2：`docs/README.md` 的 CLI Design 索引已同步为 version 25。
- 验证：`scripts/run-go-test.sh ./internal/desktop`、`scripts/run-go-test.sh ./...`、`swift desktop/fixtures/v1/verify.swift desktop/fixtures/v1/snapshot-complete.json desktop/fixtures/v1/snapshot-partial.json`、`git diff --check` 均通过。

Repair 已完成；原 Review 保持打开，Task Review cell 保持未勾选，等待独立 Re-review。

## 🔁 Round 2 — 2026-08-13 (Re-review)

📊 总体评分：9/10

✅ 结论：PASS

- 已复评状态：`49f6d35` 加未提交的 17 文件 Task 候选；按路径排序的
  `shasum -a 256` 清单聚合 SHA-256：
  `9b016b9ab70b12e30ebb8794349273b6ed8dfd9dff0d68fd2e8f390160287d0a`。本轮已提交的
  Agent 指南不在 Task 候选范围内。
- 复评者：Codex，独立只读 Re-review。
- 范围：仅复核 Round 1 两个 blocker 的关闭状态、所选 filesystem 合同、
  回归覆盖和文档索引；未把建议改进提升为 PASS 门。

### ✅ Blocker 关闭

1. **filesystem 合同与真实 SQLite 行为一致。**
   `docs/specs/cli-design.md` 不再承诺 snapshot 完全不创建文件；它禁止创建
   缺失 state root/database、迁移、权限改变、已提交 SQLite 内容改变和网络访问，
   同时明确允许读取已有 WAL-mode core/session 数据库时生成 owner-only
   `-wal`/`-shm` sidecar，以观察并发提交。
2. **回归测试覆盖了所选边界。**
   `TestBuildReadsCompleteIsolatedSnapshot` 在 snapshot 前确认四个 sidecar 不存在，
   保存两个主数据库摘要，调用真实 `Service.Build`，随后断言主数据库摘要不变、
   四个 sidecar 均存在且权限为 `0600`。独立复评命令
   `scripts/run-go-test.sh ./internal/desktop -run '^TestBuildReadsCompleteIsolatedSnapshot$'`
   通过；完整日志为
   `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.Vjs3jE`。
3. **权威索引已经同步。**
   `docs/specs/cli-design.md` 与 `docs/README.md` 均标识 CLI Design version 25。

### 🟡 保留的非阻塞建议

- recent sessions 仍在 Go 层截断，而不是用有界 metadata query。
- health recovery metadata 的专门 privacy 回归仍可补强。

这些建议没有造成 Round 1 blocker，也未在有界 Repair 中修改；后续若处理，应作为
独立授权范围，不能回写本轮 PASS。

### 🧪 证据复用与结论

- Repair 最终相关内容在本次 Re-review 前后未变化；复用同一连续流程中记录的
  `scripts/run-go-test.sh ./internal/desktop`、`scripts/run-go-test.sh ./...`、共享
  Swift fixture verifier 和 `git diff --check` PASS。
- Re-review 另加上述 filesystem 最小回归测试，避免仅凭 Repair 自报关闭 finding。
- 两个 Round 1 blocker 均已关闭，未发现 Repair 引入的新 blocker。

`desktop-wire-contract` Re-review PASS。计划 Task 1 `Review` 可以同步为已完成；
Beads 仅在评审记录、计划状态和完成证据收口后作为最后的协调投影关闭。本结论不授权
提交、推送、Task 2 开发、preflight、发布或安装。
