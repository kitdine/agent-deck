---
status: active
topic: v0-8-0-contract
subject: assemble
---

# Assemble Review

## Round 1 — 2026-11-28

## 📋 第一批装配评审：计价与额度换算

📊 总体评分: 9/10

✅ 结论: PASS

- **Reviewer**：Antigravity Gemini 4 Neon，`thought_level` high，冷上下文只读，请求号 `v080-assemble-integration-review-e11-a1`。开发者为 OpenCode GPT-6.2 Sol（variant xhigh，请求号 `v080-assemble-integration-dev-e11-a1`），二者为不同的 harness。
- **Method**：只读核对 Git 对象与暂存差异，逐项检查冲突解决与新增交互；评审方另行运行定向测试与 Swift 校验，不直接采信开发者日志。
- **Scope**：第一批装配候选与 `main` 之间的集成交互。不是整个 `assemble` 任务，也不是版本验收；第二批 `claude-local-quota` 另行装配。
- **Reviewed state**：
  - 工作区 `agent-deck.subscription-quota-conversion`，分支 `feature/subscription-quota-conversion`，合并尚未提交，结果在暂存区；
  - source（HEAD）`5fd65f53e5f5f138de5da2c66754de4acd3f0805`，经合并提交 `9053d1bf8569818f0a00eabc89084742cdfaed74` 包含计价分支的 `05612b69dee5bf647d07020d67846c6953ba5e43`；
  - target（MERGE_HEAD）`main` `2408fc96f0fc575f3cb849f6842abbd2badf2ce8`，即 tag `v0.7.5`；
  - 共同基线 `a224b0b824d2cad451d3bd83153832d65506f0fe`；
  - 候选为暂存区中相对 HEAD 的 208 个路径，按路径排序的 `blob TAB path` 清单 SHA-256 为 `572adcce5c7ecf74f131d81bc6cba9ab420da5fb8e0351b177b3ba2919a3813d`，评审方独立复算一致。
- **Operation**：三方合并，带冲突解决。22 个文件冲突，由开发者解决；主控另在三份专题文档中记录迁移号的最终值。
- 完成门禁: NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- **迁移**：v0.7.5 已发布的额度身份迁移保持 36，计价迁移顺延为 37，额度采样迁移顺延为 38，核心上限为 38。36 号迁移体与 `main` 逐字节一致。各发布 tag 的 schema 上限为 v0.6.0 30、v0.6.5 30、v0.7.0 35、v0.7.5 36，没有任何已发布构建写过 37，因此未发布分支按旧编号建的库不在升级目标内是安全的。从 36、34、12 升级，以及回滚重试，都有测试覆盖并通过。
- **服务端 schema 兼容**：Go 的 `serving_schema.go` 与 Swift 的 `DesktopWire.swift` 在整个矩阵上一致：上限 38；未来版本拒绝；31–35 到 36 接受；31–36 到 37 或 38 拒绝并重建；37 到 38 保留为过期并实时重投影订阅；38 到 37 拒绝。`serving_cache.go` 同时保留了计价的有效期检查与 v0.7.5 的订阅实时重投影。
- **桌面 wire**：两侧字段全部保留，没有丢失或改名；`replacingSubscription` 保留 `pricing`。v1 夹具、`gui-json-contract.json`（`price.list` 固定在 `2026-11-27T12:00:00Z`）和 `synthetic-snapshot.json` 都与各自的 Go 生产者一致，并通过两种 Swift 校验。
- **额度**：显式采样先取得已发布的刷新锁，再开刷新周期、核对投递守卫、固定 Claude 代次，然后才探测。写缓存时保留 `confirmed_generation`，从不调用 `PublishClaudeCaptureGate`，因此采样不能重开已阻断的捕获门禁。身份变化、认证失败和 Codex 换号时，样本都会作废。
- **备份与恢复**：可移植备份清除 `quota_session_samples` 与 `quota_identities` 并压缩；恢复先迁移到 38，再清理额度状态并压缩。
- **文档**：三份专题文档对 36、37、38 的记录一致。

### 📝 总结

**评审方执行的验证**，日志均在 `/Users/jobshen/.local/state/agentdeck-controller/v080/`，退出码全部为 0：

- 候选复算与 `git diff --cached --check`；
- `go build ./...` 与 `go vet ./...`；
- `scripts/run-go-test.sh -p 1 -count=1`，覆盖 `./internal/store`、`./internal/desktop`、`./internal/quota`、`./internal/backup`、`./internal/sessionorigin`；
- `./cmd/agentdeck` 的 schema 12 升级、端到端、快照性能、额度与桌面定向测试；
- `AgentDeckFoundationVerifier` 与 `desktop/fixtures/v1/verify.swift`。

**开发者的验证**：构建、vet、`make build-all` 和 Swift 检查都通过。全量 Go 首轮在三个包失败，修正后串行重跑通过；五个包的竞态测试以 `-p 1` 串行通过。主控核对过，首轮全量之后改动的文件都只落在这三个已重跑的包里。

**排除范围**：
- `main` 上未改动的运行时、Widget、应用生命周期、CI 与归档代码，由 v0.7.5 契约的装配记录和订阅账号与套餐辨识专题的评审覆盖；
- 两个成员专题内部未被合并触及的计价引擎与估值算法，由各自专题的评审覆盖；
- 完整 XCTest 未在本轮重跑。

**门禁**：完成门禁为 NOT_VERIFIED，因为 `assemble` 横跨两批，第二批尚未开始，任务边界的证据在两批都合入后统一记录。本轮 PASS 只认证上述候选的装配兼容性。
