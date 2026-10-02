---
status: historical
created: 2026-10-02
retired: 2026-10-02
---

# 缺陷：另一安装的 statusLine 路由被覆盖为不可执行的 prior

## 现象

Beads：`ad-bug-quota-statusline-cross-installation-conflict`，用户授权 Lane A。实时 GitHub `release/v0.6.x` base 为 `570cc9e2c818f6a4da38365194c3cd5c7bcf4e93`，PR25 已合并且前任明确释放 clean slot。工作区 `agent-deck.fix.quota-statusline-cross-installation-conflict`，分支 `fix/quota-statusline-cross-installation-conflict`，串行 `.worktrees/fix`。

隔离 HOME、A/B state-dir，第三方 statusLine 为 `printf prior`。A setup 后 B setup 原本返回 configured、覆盖 A 路由并创建含 A 托管命令的 prior。运行时拒绝链到任何 AgentDeck 托管命令，第三方状态行因此消失。

## 根因

`SetupStatusLine` 仅对完全相同的 desired entry 幂等返回；其他 entry 无条件记录 prior 并替换。`PriorStatusLineCommand` 为避免递归正确拒绝托管命令，setup 却没有相同的占用检查。

## 修复边界

在写 prior 和外部配置前拒绝已有非完全相同的托管 entry，返回 failed/modified 与明确指引：先从拥有该路由的安装禁用，再启用本安装。保留完全匹配的幂等路径、第三方命令安装与恢复、原有配置及双方 prior。复用现有托管命令识别器；无 schema、wire 或真实用户数据操作。不迁移历史版本已写入的跨安装 prior，不引入跨进程锁协议。

旧测试中期待跨 state-dir 覆盖的断言被新 failure-first 回归取代。恢复保护测试通过显式历史/外部配置 fixture 保留，而不再依赖新 setup 允许覆盖。

## 验证

- 原始 RED：产品代码未改时执行 `scripts/run-go-test.sh ./internal/usagehook -run '^TestSetupStatusLineRefusesAnotherInstallationWithoutMutation$'`，三项业务断言失败：B configured、A 路由改变、B 创建 prior。完整日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.ovjIaY`。
- GREEN：usagehook 全包通过，`/tmp/agentdeck-cross-install-green.log`。
- 主代理核验独立 reviewer 测试源及文件 blob 后，直接复跑生命周期 overlay，通过，`/tmp/agentdeck-cross-install-main-lifecycle.log`。
- 全量 Go、affected race、vet、双 macOS 架构 build、whitespace 的最终结果在下述交付检查记录。
- 全部产品行为测试使用隔离 fixtures；未读取或改写真实用户配置、日志、usage 或数据库。CodeGraph 因 node 不可用降级为限定范围源码检查。

## Review — Round 1

## 📋 独立冷上下文评审报告

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无可操作缺陷；纯格式偏好不列为 finding。

### 🟢 优点

guard 位于 prior 和配置写入前，拒绝时保留已有 owner 和 prior；本安装完全匹配仍幂等。新回归检查配置字节不变、B 无新 prior、A 第三方 prior 保留。独立 overlay 进一步覆盖 B 已有 prior 不变、A restore 后 B 可安装并最终恢复原第三方命令。CLI 仅对 configured/unchanged 授予 consent，failed 最终返回错误。

### 📝 总结

Reviewer：`/root/cold_review`，独立冷上下文只读角色，未参与实现。Method：diff、调用链、契约及定向测试审查，独立 `/tmp` overlay 生命周期测试。主代理直接核对源码、blob 和测试源并复跑 overlay 后接受 PASS。Scope：三个 usagehook 生产/测试文件。没有该代码类别专用 completeness checker；执行定向行为测试和 diff 检查。

Reviewed state：HEAD `570cc9e2c818f6a4da38365194c3cd5c7bcf4e93`；Git blob `config.go=d1344e0084f163cd2381304baa318302042d0591`、`config_test.go=3ec04eb205c9fb66fe7a7b22743a590e04a80f43`、`cross_installation_test.go=2eb2a5424febf9ff7eb59255c1fc640b8f727d8e`。

Evidence：`git diff --check` 通过；`scripts/run-go-test.sh ./internal/usagehook -run 'StatusLine|ManagedAgentDeck'` 通过，日志 `/tmp/agentdeck-cold-review-statusline.log`；`scripts/run-go-test.sh -overlay=/tmp/agentdeck-cold-review-overlay.json ./internal/usagehook -run TestColdReviewCrossInstallationLifecycle` 通过，日志 `/tmp/agentdeck-cold-review-lifecycle.log`。overlay 测试源 `/tmp/agentdeck-cold-review-lifecycle_test.go`；主代理复跑日志见验证。

Findings：无。历史版本已有 prior 链及并发 setup 的读写竞争未在本次声明解决；未把未经复现的范围外机制列为 finding。

完成门禁：NOT_VERIFIED（本轮完成时最终验证与 CEv1 尚待收口）。独立 PASS 不代替 evidence gate 或交付。

交付前检查：完整 Go 首轮仅 `internal/scanruntime` 两项隔离 Unix socket 测试遭 sandbox bind 拒绝；受控权限重跑同一 suite 全通过，日志 `/tmp/agentdeck-cross-install-full-authorized.log`。`make vet build-all` 通过，含 darwin arm64/amd64 CLI，日志 `/tmp/agentdeck-cross-install-vet-build.log`。`make check-whitespace` 和 `git diff --check` 通过。

最终 affected race（`./internal/usagehook ./cmd/agentdeck`）通过，日志 `/tmp/agentdeck-cross-install-race.log`。产品三个 blob 与独立评审完全一致。Task CEv1 `fix:quota-statusline-cross-installation-conflict` 在上述 scoped candidate 上查询 VERIFIED，四项 required criteria（failure-first regression、preservation、verification、independent review）齐备，无 missing/invalidated/unresolved。这是本轮后续 evidence 收口，不改变原评审时门禁历史。

归档边界：实现提交 `c05edf99e2f5f7a260cf6d18a8e49df46971b58e` 已核验 SSH 签名、四文件范围及 exact commit CEv1 VERIFIED 4/4。按 Fix records 生命周期归档完整 Round 1；远程当前 head review、CI 与实际 merge 仍为 Bug 关闭前提，其最终证据由 CEv1 和 Beads 交接持有。
