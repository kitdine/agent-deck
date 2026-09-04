---
status: active
created: 2026-09-03
---

# 缺陷：脱敏备份保留 Claude 的非托管 `ANTHROPIC_API_KEY` 明文

## 现象

`agentdeck provider use` 在替换 Claude 配置前，会把当前 `settings.json` 写成一份
「脱敏备份」，落在 `<stateRoot>/client-backups/claude/<operationID>.redacted.json`。当用户
自己在 `settings.json` 里配置了 `env.ANTHROPIC_API_KEY` 时，该密钥被逐字复制进
备份文件。

用一条失败优先的回归实测，修复前的备份内容为：

```json
{
  "apiKeyHelper": "/bin/echo helper",
  "env": {
    "ANTHROPIC_API_KEY": "unmanaged-secret",
    "ANTHROPIC_BASE_URL": "https://provider.example",
    "OTHER": "keep"
  },
  "model": "opus"
}
```

备份文件权限是 `0600`，与源文件同级，因此这不是把凭据暴露给新的读者；它扩大的是
同一密钥的留存面——一份用户从未要求复制的副本，按 operation 逐次累积，且落在一个
按名字声明「已脱敏」的文件里。

这个缺口不是新发现。`internal/provider/service_test.go` 的用例注释此前明文写着
「the redactor only strips `ANTHROPIC_AUTH_TOKEN`, so an unmanaged key would still
be present……That gap is real but belongs to its own triage, not to this fix」，
本记录就是那次 triage。

## 根因

`internal/provider/config.go:799-806`，`WriteRedactedBackup` 的 Claude 分支只
`delete(env, "ANTHROPIC_AUTH_TOKEN")`。

判定它是缺陷而非既定行为的依据，是仓库自己写下的两条契约在此处不一致：

- `internal/provider/service.go:1078`，`Recover` 的文档注释：the redacted backup
  **intentionally excludes credential values**。写的是 credential **values**，
  不是「AgentDeck 自己写入的那一个 key」。
- `internal/provider/config.go:176-183`，`ClaudeConflictAPIKey` 的注释：
  `ANTHROPIC_API_KEY` 与 `apiKeyHelper` 是 Claude 认可、而 AgentDeck
  **never writes, clears, or reorders** 的两个凭据来源。

这两条并不冲突，但此前被当成一条读。所有权契约约束的是 AgentDeck 对**用户的
`settings.json`** 的操作——不写、不清、不重排；备份是 AgentDeck 自己产出的**另一个
文件**，在其中省略一个凭据，不构成对用户配置的清除。「不拥有它」是永不在 settings.json
里删除它的理由，不是把它复制到第二个文件的理由。

触发条件真实且不罕见：`ClaudeCredentialConflicts` 这一整套 advisory 存在的前提，
就是用户确实会在 `settings.json` 里放 `ANTHROPIC_API_KEY`。

## 修复边界

**改**：`WriteRedactedBackup` 的 Claude 分支追加 `delete(env, "ANTHROPIC_API_KEY")`。

**不改**：

- **`apiKeyHelper` 保留**。按 `ClaudeCredentialConflicts` 自己的表述，两个 key 中
  「one of the two keys holds a credential」——持有值的是 `ANTHROPIC_API_KEY`，
  `apiKeyHelper` 命名的是一条命令。契约说排除的是 credential **values**，所以删掉
  一条命令会丢掉可还原的配置，却不减少任何凭据值。**已知边界**：helper 命令行内可以
  内嵌密钥（本仓库测试里就有 `/bin/echo synthetic-helper-secret` 这样的写法）；那属于
  「命令字符串是否应被视作凭据」的新判定，不在本修复内，需要时另行 triage。
- **不改 Codex 分支**。Codex 侧 AgentDeck 写入的凭据字段只有
  `model_providers.custom.experimental_bearer_token`，备份删的正是它；`config.toml`
  在本仓库的建模里不承载第二个凭据来源，因此不存在对称缺口。
- **不改备份的用途与路径**。备份只被 `CreateOperation` 记进 `operations.redacted_backup_path`
  并由 `Recover` 用于诊断，仓库内没有任何还原消费者，因此少一个字段不改变它的既有用途——
  它本来就已经因为删除 `ANTHROPIC_AUTH_TOKEN` 而不是可完整还原的副本。
- **不做历史备份清理**。已存在的备份文件是既有产物，删除用户状态目录下的文件是新的
  用户可见决定，不属于本修复。

**Lane A 判定依据**：契约已存在——`Recover` 已写明备份排除 credential values，本修复
只让实现回到该契约。不新增状态、不新增输出形状、不改变任何用户可见行为（备份是内部
诊断产物，不是 CLI 输出）。

## 验证

L2：本修复改变一个持久化文件的内容。

**失败优先**：新增 `TestWriteRedactedBackupDropsTheUnmanagedClaudeAPIKey`
（`internal/provider/config_test.go`）。修复前该用例失败，失败输出即上文「现象」中
那份含 `unmanaged-secret` 的备份内容；修复后通过。用例同时锁住三件事：密钥字面量不出现、
`ANTHROPIC_API_KEY` 键不存在、而 `ANTHROPIC_BASE_URL` / `OTHER` / `model` / `apiKeyHelper`
全部保留——即「删了什么」和「留了什么」都被断言，避免一个清空文档的实现也能通过。

**加强既有用例**：`TestUseRecordsWhetherTheClaudeClientAlreadyHeldACredential`
（`internal/provider/service_test.go`）的泄漏断言原本只查 `synthetic-secret`，现改为
同时检查 `unmanaged-key-literal`，让「client carries an unmanaged api key」这一子用例
真正验证端到端路径（`service.Use` → `WriteRedactedBackup`）不泄漏非托管凭据。原注释里
「gap is real but belongs to its own triage」一句已改为指向本记录。

**命令与结果**：

```text
./scripts/run-go-test.sh ./internal/provider/ -run 'TestWriteRedactedBackupDropsTheUnmanagedClaudeAPIKey' -count=1
    → FAIL（修复前，失败优先确认）
./scripts/run-go-test.sh ./internal/provider/ -run 'TestWriteRedactedBackup|TestUseRecordsWhetherTheClaudeClientAlreadyHeldACredential' -count=1
    → passed
./scripts/run-go-test.sh ./... -count=1        → passed
go vet ./...                                    → 无输出
make check-whitespace                           → 无输出
git diff --check                                → clean
gofmt -l internal/provider/{config.go,config_test.go,service_test.go} → 无输出
```

**一处如实记录的观察**：`gofmt -l internal/ cmd/` 报告 `cmd/agentdeck/usage_stats_viewer_test.go`
未格式化。该文件不在本次改动内（`git diff --name-only` 不含它），是 `HEAD` 既有偏差，
按范围规则不在本修复中一并处理。

## Review — Round 1

- Reviewed state: HEAD `be7da8b20a56234b46edfbfbb470d34618d31812`；
  `internal/provider/config.go`、`internal/provider/config_test.go`、
  `internal/provider/service_test.go` 的 scoped diff SHA-256
  `ec238e59a392725448af75bac210db21a27c93984b725c0d66db2004418a37e9`；
  implementation content state
  `urn:ce:agent-deck:state:workspace:a166aa41c804caab483898d45bbb9212a63def8b2c2302f9d1966934f20b19fd`；
  本记录评审前 blob `3e9a18ac5d9916186ec565d34f62edeff7a32b66`。
- Reviewer: Codex（单 agent、默认模型层级的独立 code/security review）。
- Method: 先用 CodeGraph 核对 `Service.UseCredential` →
  `WriteRedactedBackup` → operation record 的生产路径，再审阅三文件 scoped diff、
  `ClaudeCredentialConflicts`/`ClaudeConfigIsKeyed` 与 CLI Design 的备份及配置所有权
  合同；CodeGraph 两次未返回点名测试体后，使用一次有界源文件读取核对完整断言。
  运行受影响聚焦测试。两处当前变更内的文档/注释错误成立后，按评审规则停止全仓
  L3 验证。
- Scope: `internal/provider/config.go`、`internal/provider/config_test.go`、
  `internal/provider/service_test.go` 与本记录；`cmd/agentdeck/*`、
  `docs/fixes/hook-transcript-admission-edges.md`、
  `docs/topics/schema-version-signal/` 及其他工作树内容排除。生产代码、测试和配置
  全程只读。

### 📋 评审报告：fix / claude-backup-api-key-redaction

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`docs/fixes/claude-backup-api-key-redaction.md:10`] R1-F1 `[P2]`：现象把托管备份
路径写成 `<stateRoot>/client-backups/claude/<operationID>.json`，但当前实现与 CLI
Design 的真实文件名均为 `<operation-id>.redacted.json`。

- 行为风险：按记录复现或排查泄漏时会检查一个产品从不创建的文件名，既可能漏过
  实际凭据副本，也使该 Lane A 记录与其证明的生产路径不一致。
- 证据：`internal/provider/service.go:1060-1073` 的 `managedBackupPath` 为 Claude
  返回 `operationID + ".redacted.json"`；`docs/specs/cli-design.md:730-733` 规定
  `<operation-id>.redacted.toml|json`。

💡 有界修复：只把现象中的路径改为
`<stateRoot>/client-backups/claude/<operationID>.redacted.json`；不改生产路径或备份
用途。

[`internal/provider/config.go:811`] R1-F2 `[P2]`：新增生产注释与测试注释把
`apiKeyHelper` 绝对描述为“not a value”/“holds no credential value”，与本记录已经
承认的 helper 命令可内嵌密钥边界及仓库现有测试数据冲突。

- 行为风险：这段注释紧邻 credential redaction policy，后续维护者会据此把 helper
  当成结构上不可能含敏感字面量，从而漏掉本记录明确留给独立 triage 的风险。当前
  Task 保留 helper 的范围决定没有问题；错误在于把“本 Task 不把它归类为直接 API-key
  value”写成“它不含 credential”。
- 证据：本记录 `:66-72` 明确写明 helper 命令行可以内嵌密钥；
  `internal/provider/switch_advisories_test.go:23` 使用
  `/bin/echo synthetic-helper-secret`；CLI Design `:795-798` 把 `apiKeyHelper`
  定义为 Claude 认可的 unowned credential source。相反，新注释位于
  `internal/provider/config.go:811-813` 与
  `internal/provider/config_test.go:951-954,984`。

💡 有界修复：保持 `apiKeyHelper` 原样保留，只把上述新增注释改为准确的范围表述：
它在本 Task 中按“命令设置、非直接 API-key value”处理，命令内嵌密钥风险已知且另行
triage；不得声称它结构上不含 credential，也不得借此扩大为 helper 行为改动。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 生产改动只在 Claude 备份副本删除 `ANTHROPIC_API_KEY`，没有清除用户原始
  `settings.json`、改动 Codex 分支或清理历史状态。
- 聚焦测试同时断言密钥字面量与 key 缺失，并锁住 base URL、其他 env、model 与
  helper 的保留；`service.Use` 路径也覆盖 managed/unmanaged 两种合成密钥。
- `WriteRedactedBackup` 仍在 operation 创建与客户端配置改写之前执行，失败不会留下
  已登记但未脱敏的 operation 记录。

### 📝 总结

评审对象由上述 HEAD、三文件 scoped diff、implementation content state 与评审前记录
blob 唯一标识。聚焦测试
`env GOCACHE=/private/tmp/agent-deck-go-build ./scripts/run-go-test.sh
./internal/provider -run 'TestWriteRedactedBackup|TestUseRecordsWhetherTheClaudeClientAlreadyHeldACredential'
-count=1` 通过，未发现运行时 redaction 回归；但 fix 记录给错实际文件名，新增安全策略
注释又否认自己已记录的 helper 内嵌密钥边界。两项都在当前 Task 的文字改动内且修复
有界，因此本轮 FAIL/REOPEN；决定性 finding 后未重复全仓 L3，修复后需在最终内容状态
重新验证。

- Findings:
  - R1-F1 `[P2]` -> open；修正托管备份的 `.redacted.json` 文件名。
  - R1-F2 `[P2]` -> open；保留 helper 行为，仅纠正三处绝对化安全注释。
- Evidence: CodeGraph 生产调用链；CLI Design `:730-798`；上述 source/test locations；
  受影响聚焦测试 PASS；CEv1 entry gate 对非空 6 条 criteria 返回 `NOT_VERIFIED`。
- Completion gate: `FAILED`
- Verdict: REOPEN

### 下一步指令

修复：fix / claude-backup-api-key-redaction / R1-F1, R1-F2

## Repair — Round 1 — 2026-09-04

两项 finding 均成立，按评审给出的有界修复处置，生产行为一行未改。

### `R1-F1` —— 成立，已修正

核实证据后确认记录写错了文件名：`internal/provider/service.go:1064-1073` 的
`managedBackupPath` 对 Claude 返回 `operationID + ".redacted.json"`（Codex 为
`".redacted.toml"`），`docs/specs/cli-design.md:732` 规定的形状是
`<state-dir>/client-backups/<client>/<operation-id>.redacted.toml|json`。本记录
「现象」节此前写的 `<operationID>.json` 是一个产品从不创建的文件名。

处置：仅把该路径改为 `<stateRoot>/client-backups/claude/<operationID>.redacted.json`。
生产路径与备份用途未动。

### `R1-F2` —— 成立，已修正三处措辞

指出的问题准确：范围决定（保留 `apiKeyHelper`）没有错，错在把它写成了安全属性断言。
本记录 `:66-72` 已承认 helper 命令行可内嵌密钥，而新增注释却称它 "not a value" /
"holds no credential value"，两处自相矛盾；`internal/provider/switch_advisories_test.go:23`
的 `/bin/echo synthetic-helper-secret` 正是反例。这段注释紧邻 redaction 策略，后续维护者
据此会把 helper 当成结构上不可能含敏感字面量。

处置：`apiKeyHelper` 保留行为不变，只改三处措辞，改为范围表述——本 redactor 丢弃的是
直接 API-key value，helper 是命令设置而非其中之一，因此删它会损失可还原配置；同时明写
这不等于断言 helper 不可能携带密钥，命令字符串是否算凭据是另一项独立 triage：

- `internal/provider/config.go:809-816`（生产注释，并指向本记录）
- `internal/provider/config_test.go:947-953`（用例头注释）
- `internal/provider/config_test.go:989`（断言失败消息）

### 未纳入的

评审「🟡 建议改进」为空，无其他处置项。`cmd/agentdeck/*` 与
`docs/fixes/hook-transcript-admission-edges.md` 属另一 Task，未动。

## Repair — Round 1 verification — 2026-09-04

```text
./scripts/run-go-test.sh ./internal/provider/ -run 'TestWriteRedactedBackup|TestUseRecordsWhetherTheClaudeClientAlreadyHeldACredential' -count=1
    → passed
./scripts/run-go-test.sh ./... -count=1        → passed
go vet ./...                                    → 无输出
make check-whitespace                           → 无输出
git diff --check                                → clean
gofmt -l internal/provider/{config.go,config_test.go,service_test.go} → 无输出
```

本轮改动全部是注释、断言消息与文档文字，`WriteRedactedBackup` 的可执行逻辑与
`TestWriteRedactedBackupDropsTheUnmanagedClaudeAPIKey` 的断言集合均未改变，因此
Round 1 的失败优先证据继续成立。评审因决定性 finding 未跑的全仓 L3，已在本轮补跑。

## Re-review — Round 2

- Reviewed state: HEAD `be7da8b20a56234b46edfbfbb470d34618d31812`；
  `internal/provider/config.go`、`internal/provider/config_test.go`、
  `internal/provider/service_test.go` 的 scoped diff SHA-256
  `b701783da656e805c490119c708ce39b1e06150f9e027deeb3aa9be1681d06ff`；
  implementation content state
  `urn:ce:agent-deck:state:workspace:84b2d9d2f9fde1a1ccde987c7e9ad50d047d0db774f630075e2976625f0f3ba1`；
  本记录复评前 blob `eebf1657564e0a97d44f98329272f6d9a51ab559`。
- Reviewer: Codex（单 agent、默认模型层级的独立 finding-by-finding
  code/security Re-review）。
- Method: 逐项复核 R1-F1、R1-F2 的修复记录与实际内容；比较三文件 scoped diff，
  确认可执行 redaction 逻辑和测试断言集合未变；核对 live Beads task/Gate/comments
  与 CEv1 WorkUnit 的六条 active criteria；复用 Round 1 聚焦行为测试，并在最终修复
  状态运行全仓 vendored Go suite、vet、仓库 whitespace/diff 与 scoped gofmt 检查。
- Scope: R1-F1、R1-F2 及其修复触及的 `internal/provider/config.go`、
  `internal/provider/config_test.go`、本记录；同时回归核对
  `internal/provider/service_test.go`。`cmd/agentdeck/*`、
  `docs/fixes/hook-transcript-admission-edges.md`、
  `docs/topics/schema-version-signal/` 及其他工作树内容排除。生产代码、测试和配置
  全程只读。

### 📋 复评报告：fix / claude-backup-api-key-redaction

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 closed：现象中的真实路径已改为
  `<stateRoot>/client-backups/claude/<operationID>.redacted.json`，与
  `managedBackupPath` 和 CLI Design 一致；生产路径与用途未动。
- R1-F2 closed：生产注释、用例头注释与断言失败消息都改为本 redactor 的范围
  表述，明确 helper 是命令设置、不是本 Task 删除的 direct API-key value，同时明确
  命令行仍可能内嵌密钥并由独立 triage 决定；`apiKeyHelper` 行为未变。
- `WriteRedactedBackup` 的可执行改动仍只有删除 `ANTHROPIC_API_KEY`，聚焦回归仍同时
  保护 managed/unmanaged 两种合成密钥、保留字段与 `prior_keyed` 路径。
- 全仓 vendored Go suite 在最终状态通过；vet、whitespace、diff 与 scoped gofmt
  均 clean，验证后 fingerprint 未漂移。

### 📝 总结

| Finding | 处置 |
| --- | --- |
| R1-F1 | Closed；备份文件名修正为 `.redacted.json` |
| R1-F2 | Closed；三处注释改为准确的范围表述，helper 行为保持不变 |
| 新 finding | 无 |

复评对象由上述 HEAD、三文件 scoped diff、implementation content state 与复评前记录
blob 唯一标识。两条既有 finding 在新内容中均已关闭，未引入行为变化或范围扩张。
历史备份不清理、helper 命令字符串是否应另作 credential 处理仍是本 Task 明示的范围外
边界，不是未关闭 finding。故本轮 PASS。

- Evidence:
  - Round 1 聚焦测试继续有效：本轮只改注释、失败消息与记录文字，执行逻辑及断言集合
    未变。
  - `env GOCACHE=/private/tmp/agent-deck-go-build ./scripts/run-go-test.sh ./...
    -count=1`：PASS。
  - `env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`：无输出。
  - `make check-whitespace`、`git diff --check`、
    `gofmt -l internal/provider/config.go internal/provider/config_test.go
    internal/provider/service_test.go`：无输出。
  - 验证后 scoped diff SHA-256 仍为 `b701783d…`。
- Completion gate: `VERIFIED`
- Verdict: PASS

Task checkpoint：`fix:claude-backup-api-key-redaction` / implementation state
`84b2d9d2f9fde1a1ccde987c7e9ad50d047d0db774f630075e2976625f0f3ba1` /
completion gate `VERIFIED`。

提交建议：仅提交 `internal/provider/config.go`、
`internal/provider/config_test.go`、`internal/provider/service_test.go` 与
`docs/fixes/claude-backup-api-key-redaction.md`；排除 `cmd/agentdeck/*`、
`docs/fixes/hook-transcript-admission-edges.md`、
`docs/topics/schema-version-signal/` 及其他无关工作。提交前按交付规则核对 staged
files/hunks、完整英文 Conventional Commit subject/body、Codex trailer 与 SSH 签名。

推送建议：目标为当前 `main` → `origin/main`，但 `main` 已 ahead 5；仅在另行获得推送
授权、当前 Task 的签名提交存在、完整 outgoing range 均确认应推送且远端没有需先整合的
提交后执行。

### 下一步指令

提交：fix / claude-backup-api-key-redaction
