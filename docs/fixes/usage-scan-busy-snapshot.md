---
status: active
created: 2026-10-02
---

# 缺陷：usage 源发布遇到外部写入时 SQLITE_BUSY_SNAPSHOT

## 现象

Beads：`ad-bug-usage-scan-busy-snapshot`，用户已确认 Lane A 并授权单 issue 端到端交付。
基线：`release/v0.6.x` / `ab5ec486e67edb5fd48745b54a302697b1db7ae9`。
工作区：`agent-deck.fix.usage-scan-busy-snapshot`；分支：`fix/usage-scan-busy-snapshot`。

首次导入、追加和重写三种源发布过程中，另一连接在 affectedSessions 读取之后提交核心库写入，扫描均返回 `database is locked (517)`。
隔离 fixture 使用临时目录、合成 Codex 日志和独立 SQLite 连接，不访问真实用户 usage 或日志数据库。

## 根因

`internal/usage/usage.go` 的 scanFileMode 使用 deferred BeginTx，先读取 affectedSessions 建立快照，再修改 usage 表。
WAL 模式下外部连接可在此期间提交；过时读快照不能升级为写事务，直接返回 SQLITE_BUSY_SNAPSHOT，busy_timeout 无法恢复它。

## 修复边界

仅在源发布事务的首次数据库读取前执行零行 UPDATE，预约 SQLite writer lock；保持既有 sql.Tx 生命周期、回滚、busy_timeout 和共享连接池配置。
WHERE 0 不修改源记录，也不触发行级更新 trigger。持锁区仅覆盖既有原子发布；日志解析和捕获校验仍在读取数据库前完成。
不修改全库事务模式、schema、错误码、输出或 provider 行为。其他独立事务入口不在本 Bug 的源发布边界内。

## 验证

- Failure-first：`TestSourcePublicationSerializesExternalWriter` 在未修复产品代码上，cold/append/rewrite 均确定性失败 517；外部 INSERT 成功。日志 `/tmp/agentdeck-usage-busy-red.log`。
- 修复后同一测试通过：发布窗口外部写入返回 SQLITE_BUSY(5)，发布后的外部写入成功；tokens 总量和 cursor 均符合预期。日志 `/tmp/agentdeck-usage-busy-green.log`；最终加强的错误码断言由全套与 race 覆盖。
- 全套 `scripts/run-go-test.sh ./...`：除两项 Unix socket 测试受沙箱 bind 限制外全部通过，日志 `/tmp/agentdeck-usage-busy-full.log`。受限用例在正规提权后单独重跑。
- `scripts/run-go-test.sh -race ./internal/usage ./internal/scanruntime`：usage 全包及其余 scanruntime 用例通过；同两项 socket fixture 正规提权复测。日志 `/tmp/agentdeck-usage-busy-race.log`。
- `make vet build-all` 成功，两种 macOS CLI 架构均构建成功。
- CodeGraph CLI 缺少 node，使用定向源码检查。
- 两项受限 socket fixture 已分别在普通和 race 正规提权重跑通过：`/tmp/agentdeck-usage-busy-runtime.log`、`/tmp/agentdeck-usage-busy-runtime-race.log`。其余已通过检查未重复运行。
- 主代理检查独立 overlay 源码并分别重跑等待与取消/失败回滚测试，均 PASS：`/tmp/agentdeck-usage-busy-wait-verified.log`、`/tmp/agentdeck-usage-busy-cancel-verified.log`。
- `make check-whitespace`、`git diff --check` PASS。

## Review — Round 1

### 📋 独立冷上下文评审报告

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无；无目标内未关闭发现。

### 🟡 建议改进——推荐

无。

### 🟢 优点

锁预约发生在 affectedSessions 的快照读取前，不改变全库设置或任何源数据。解析和捕获检查位于锁外，既有事务回滚覆盖失败和取消。
回归确定性复现旧代码的 517，覆盖 cold/append/rewrite，并验证写入排他、提交释放、tokens 和 cursor。
独立附加测试验证已有 writer 的等待，以及取消后的回滚释放。

### 📝 总结

Reviewer：`usage_cold_review`，冷上下文独立角色，默认模型层级；主代理复核源码、内容身份、日志及补充测试。
Method：只读代码/diff 检查、旧产品代码 Go overlay 对照、隔离 SQLite 时序测试。Scope：源发布事务与 `publication_lock_test.go`；未修改共享连接池语义。
Reviewed state：HEAD `ab5ec486e67edb5fd48745b54a302697b1db7ae9`；产品 blob `6d8e2d03002cf64c3d2e895ed43b824901f4f914`，测试 blob `02656ef81e8db3e342ed8d8d2308226ae80703b3`。
Evidence：独立 candidate PASS，baseline overlay 三模式均 517；取消/旧重建回滚 PASS；已有 writer 等待 PASS。日志位于系统临时目录 `agentdeck-go-test.5xOVV8`、`agentdeck-go-test.RX8Vxu`、`agentdeck-go-test.KgNvxC`、`agentdeck-go-test.BGVGh6`；主代理重跑日志如上。
限制：隔离 fixture，不声明真实用户数据库或原生 UI 验收。无新接口，无该目标专用额外静态 checker。
Completion gate：VERIFIED 4/4，`fix:usage-scan-busy-snapshot:candidate:ab5ec486:6d8e2d03-02656ef8`；missing、invalidated、unresolved 均为空。

### Task checkpoint

Task：`fix:usage-scan-busy-snapshot`；精确产品/测试身份与 VERIFIED gate 如上。
提交建议：按既有授权提交这两个代码/测试文件及本 fix carrier。
推送建议：核验签名及内容后普通推送独立 fix 分支，创建目标 release/v0.6.x 的 draft PR；exact-head GitHub review/CI 与 exact-merge integration gate 仍为关闭前提。
