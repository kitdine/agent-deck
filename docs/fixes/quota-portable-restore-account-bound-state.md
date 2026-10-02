---
status: active
created: 2026-10-02
---

# 缺陷：便携式恢复保留源账号配额缓存

## 现象

Beads: `ad-bug-quota-portable-restore-account-bound-state`。Lane A；本轮用户已授权单 issue 端到端修复和交付。
基线 `release/v0.6.x` / `e6e37d97ec7fb25e1da7b4efcf440d0cc33abf39`；工作区
`agent-deck.fix.quota-portable-restore-account-bound-state`，分支 `fix/quota-portable-restore-account-bound-state`。

隔离的账号 A 备份恢复到目标机器后，目标首次探测之前和失败之后仍可读取 A 的 91% 用量、套餐、reset allowance 和余额；Claude 的机器本地观察也残留。三张配额表各残留两行。

## 根因

`internal/backup/backup.go` 的 Restore 在验证并恢复核心库、迁移 schema、重新加密凭据后，仅清除机器本地 StatusLineConsent。
`quota_windows`、`quota_envelopes` 和 `quota_alert_notices` 随数据库备份复制；失败探测按既有契约保留 last-known-good，无法建立目标账号身份。
`quota.Store` 的账号切换隔离只在成功观察新的 Codex 账号时清理，不能保护成功探测之前的读取。

## 修复边界

恢复成功前清空目标核心库中的上述三张表，覆盖 Codex 和无账号标识的 Claude；不改变 quota 读取、调度、schema、输出或错误码契约。
目标在首次成功读取前无源账号观察，失败探测也无可保留的源数据。清理失败使用已有 Restore 回滚，移除本次创建的目标文件并还原既有空目录权限。
保留 provider、重新加密的凭据、usage、配额开关/阈值/间隔；既有 status-line consent 清除保持生效。源数据库和加密归档不修改。
所有测试使用临时目录、合成账号、合成凭据和注入 probe；不读取真实账号或日志。

## 验证

- Failure-first：未修改产品的 `TestRestoreDiscardsAccountBoundQuotaState` 失败，日志 `/tmp/agentdeck-portable-red.log`；恢复后及失败探测后均暴露源数据，三张表残留。
- 初始 backup/quota 包回归通过：`/tmp/agentdeck-portable-green.log`。
- 新增失败回滚 fixture 覆盖新目录和既有空目录；最初目录权限断言受 umask 影响，fixture 显式 chmod 后复测。此项不涉及产品修复。
- 完整 Go：`scripts/run-go-test.sh ./...`，最终日志 `/tmp/agentdeck-portable-full-final.log`，PASS。首轮仅修正前 umask fixture 失败，最终重新运行全部包。
- Race：恢复隔离/回滚/consent 与 quota Store/Scheduler 的最终选择通过，`/tmp/agentdeck-portable-race-final.log`。首轮完整 backup/quota race 仅旧 fixture 失败；其中未改变的 quota 全包通过，未发现 race。
- `make vet build-all` 通过，日志 `/tmp/agentdeck-portable-build.log`；`make check-whitespace` 和 `git diff --check` 通过。
- 主代理复核并重跑独立消费者 overlay，`/tmp/agentdeck-portable-consumer-verified.log`，PASS。
- CodeGraph 当前 CLI 缺 node，按规则使用精确定向源码读取；不据此主张索引证据。

## Review — Round 1

### 独立评审报告

总体评分：9/10

评审结论：PASS

### 严重问题——必须修复

无。

### 建议改进——推荐

无。没有未关闭的目标内发现。

### 优点

恢复边界清理完整 source quota envelope 和 windows/notice ledger，覆盖 Claude 无账号标识情况；失败返回复用原有文件回滚，不扩大数据库或配置修改范围。
测试涵盖探测前、失败后、账号 B 成功后的隔离，以及既有空目录与新目录的回滚。

### 总结

Reviewer：冷上下文独立 `portable_cold_review`，主代理复核内容身份、源码调用链及独立 fixture 输出。
Method：只读 diff、恢复/失败路径及 CLI/Desktop 消费者检查；临时 Go overlay 对共享 `BuildSubscription` 作运行断言。
Scope：`internal/backup/backup.go` 与 `backup_test.go`，以及受影响 quota 读取和展示调用链。
Reviewed state：HEAD `e6e37d97ec7fb25e1da7b4efcf440d0cc33abf39`；产品 blob `af6b0167c8457a6f2b89badf154b7d19ee6335a0`，测试 blob `9d22afaae80f880155d5640043d59b9a491466d9`。

Evidence：独立运行 `scripts/run-go-test.sh -overlay=/tmp/agentdeck-cold-consumer-overlay.json ./internal/backup -run 'TestRestore(DiscardsAccountBoundQuotaState|RollsBackQuotaCleanupFailure)$'`，日志 `/tmp/agentdeck-cold-consumer.log`，PASS。
该 overlay 设置 official 路由，保证展示未被不适用 gate 掩盖；探测前/失败后两个客户端均无窗口、套餐、reset allowance、tightest window 或 observed reset。
CLI 调用 `desktop.Service.BuildSubscription`；Desktop 每次单独装配 subscription，不从 usage derived cache 恢复它。菜单栏消费相同 subscription clients。

限制：合成 fixture 和注入 probe，不代表真实账号探测或原生 UI 运行验收。未改变原生消费者；源码链与共享展示模型运行证据覆盖本次风险。
Completion gate：VERIFIED 4/4，`fix:quota-portable-restore-account-bound-state` at `fix:quota-portable-restore-account-bound-state:candidate:e6e37d9:af6b0167-9d22afaa`；无 missing、invalidated 或 unresolved impacts。

### Task checkpoint

Task：`fix:quota-portable-restore-account-bound-state`，精确受评身份如上。
提交建议：完整验证和精确状态门禁已通过，按既有用户授权提交代码、测试与本记录。
推送建议：核验签名及提交内容后，普通推送至独立 fix 分支并创建目标为 `release/v0.6.x` 的 draft PR。
