---
status: historical
retired: 2026-10-02
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

## 交付记录

修复提交 `0171a75a328d2b23e9b1c8f1ac36a530f2233fe4` 的 SSH 签名、完整消息、三文件范围与精确产品/测试 blob 已核验；commit-bound Task CEv1 VERIFIED 4/4。按 Lane A 生命周期归档，保留全部评审和验证记录。远端当前 head review/CI、实际 merge 与 per-Bug integration gate 由 Beads/CEv1 继续跟踪，归档不表示 Bug 已关闭或版本已发布。

## Review — Round 2

### 📋 GitHub 当前 head 评审报告

📊 总体评分：7/10（主代理基于已复现发现的评估）

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

- GH28-F1 / P2：`usage.go` 写锁预约之前的 validateCaptured 无法覆盖随后锁等待期间的重写/截断；可提交旧事件和游标。来源 PR28 review comment `4163477138`。修复：取得 writer 后再校验捕获范围。Disposition：已修复 candidate，待独立复评。
- GH28-F2 / P2：锁竞争期间 context 取消被 SQLite busy_timeout 延迟约五秒。来源 PR28 review comment `4163477150`。修复：固定连接，仅在发布锁获取期间关闭阻塞 busy handler，以原 timeout 为总预算进行可取消重试，归池前恢复原 timeout。Disposition：已修复 candidate，待独立复评。

### 🟡 建议改进——推荐

无额外建议。

### 🟢 优点

原有 cold/append/rewrite 的 517 保护仍有效；本轮加强等待窗口安全与取消响应。

### 📝 总结

Reviewer：GitHub Codex，评审 head `4bafbd35a7326028419e7c992f29740fb622ea9e`；主代理验证发现。
Method：远端代码评审和本地隔离 fixture 复现。Scope：源发布写锁等待新增窗口。
Evidence：`/tmp/agentdeck-usage-busy-review-red-final.log`，rewrite/truncate 返回成功且发布旧事件；cancel 约 5.1 秒。首版 fixture 缺 turn/model，修正后正常等待对照通过，三个缺陷断言仍失败。
Completion gate：NOT_VERIFIED；旧状态证据不授权新 candidate 或旧 head 合并。本轮等待修复后独立复评及新 head CI。

## Review — Round 3

### 📋 独立冷上下文复评报告

📊 总体评分：9.5/10

✅ 复评结论：PASS

### 🔴 严重问题——必须修复

无新发现。GH28-F1 CLOSED：writer 获取后重新校验，等待期间的 rewrite/truncate 中止发布。GH28-F2 CLOSED：仅预约锁使用 SQLite timeout=0 的有界可取消重试，等待预算保持原 timeout；取消无需等待五秒。

### 🟡 建议改进——推荐

无。

### 🟢 优点

固定连接隔离临时 busy_timeout；清理在归池前恢复原值，恢复失败则弃用连接。重试仅执行零行写锁预约，不重复任何发布数据操作。
新增回归覆盖两个真实负向窗口、正常等待、零超时、短超时和成功/失败后的 timeout 恢复；publication 仍为一个原子事务。

### 📝 总结

Reviewer：`usage_wait_cold_review`，新的冷上下文独立角色、默认模型层级。主代理直接复核源码、blob 与日志。
Method：只读调用链、Go database/sql 与 vendored SQLite 清理同步检查；独立 focused race 和旧产品负对照 overlay。
Scope：`usage.go`、`publication_lock.go`、`publication_lock_test.go`。
Reviewed state：HEAD `4bafbd35a7326028419e7c992f29740fb622ea9e`；三个 blob 分别为 `7fe76f1cb8f29bd8a24c187549d0085ef3f70def`、`2c4ab1834f11a028361a9d06ae1934f4786eba7b`、`35aef6975b68d013005f7fe47a119afde3de89be`。
Evidence：独立 targeted race PASS，`/tmp/usage-publication-cold-review.log`；负对照在 rewrite/truncate 错误发布一事件、cancel 5.1096 秒，`/tmp/usage-publication-cold-review-negative.log`。主代理 targeted race PASS，`/tmp/agentdeck-usage-busy-final-targeted.log`。`git diff --check` PASS。
限制：未对底层 PRAGMA 恢复失败做故障注入；弃用连接路径经源码检查。所有运行验证使用隔离 fixture。
Completion gate：VERIFIED 4/4，`fix:usage-scan-busy-snapshot:candidate:4bafbd35:7fe76f1c-2c4ab183-35aef697`；无 missing/invalidated/unresolved。旧 head 门禁为 FAILED，新状态使用本轮重新验证和复评事实。

本轮最终验证：完整 `scripts/run-go-test.sh ./...` PASS（`/tmp/agentdeck-usage-busy-r3-full.log`）；`scripts/run-go-test.sh -race ./internal/usage ./internal/scanruntime` PASS（`/tmp/agentdeck-usage-busy-r3-race.log`）；`make vet build-all` PASS（`/tmp/agentdeck-usage-busy-r3-build.log`）；whitespace/diff checks PASS。本轮两项 socket fixture 与新产品状态一并在已批准环境中通过，无残留失败。

### Task checkpoint

Task：`fix:usage-scan-busy-snapshot`，Round3 exact candidate VERIFIED 4/4。
提交建议：三个 usage 产品/测试文件和本记录；四文件范围。
推送建议：签名核验后普通推送同一 PR28，重新请求 current-head GitHub review 并等待全部 CI；不使用旧 head 的远端结果。
