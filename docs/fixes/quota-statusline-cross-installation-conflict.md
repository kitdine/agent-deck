---
status: active
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

## Review — Round 2

## 📋 GitHub Codex 当前 head 评审报告

📊 总体评分：7/10（主代理基于已复现 findings 裁定）

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CS-R2-F1 — P2，OPEN。** `internal/usagehook/config.go:1006–1010`：Formula→Cask 保留同一 state root，但可执行文件变化；guard 把同 owner migration 判为外来安装，旧可执行文件卸载后指引无法执行。既有分发契约见 `docs/specs/cli-design.md:440–442`。需基于解析后的 state root 和有效 prior 识别同 owner，更新 route 但保留 prior。

**CS-R2-F2 — P2，OPEN。** 同 guard 复用的识别器在检查完整引用前拒绝 path 中的 ` --`，因此生成的合法 quoted `/tmp/state -- blue` route 被 B 当第三方 prior 覆盖。需 quote-aware 识别生成的参数形状，拒绝真正的额外参数或 shell fragments。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

普通 A/B 拒绝及保存 prior 的方向正确；这两项要求补齐既有安装迁移与合法路径边界。

### 📝 总结

Reviewer：GitHub `chatgpt-codex-connector`，review `PRR_kwDOTe7lus8AAAABQS2YoQ`；Method：远程独立代码评审，主代理直接验证契约及源码并在未改产品代码时新增两项回归，均得到有效 RED，日志 `/tmp/agentdeck-cross-install-r2-red.log`。Reviewed state：`f7d6bc4fa81404c30a6a10b2c685c54bef3ec607`。Scope：同 Bug 的托管路由占用识别、同 owner 迁移和 prior 保留。

远程 threads：`PRRT_kwDOTe7lus6oO6Xv`（comment 4163011947），`PRRT_kwDOTe7lus6oO6X2`（comment 4163011957）。Findings 均由主代理确认，原 Round 1 不删除。记录重新激活；原 retired 日期保留为第一次归档历史。

完成门禁：NOT_VERIFIED；当前修复候选不能继承旧 head 的完成结论。下一步：同 issue 修复 CS-R2-F1/F2 并独立复评。

Round 2 修复补充 CS-R2-F3（P2，主代理在修复期直接发现）：迁移成功后 settings precommit 失败，既有 enable rollback 调用 RestoreStatusLine 会撤下仍有 consent 的旧 owner route。有效 RED `/tmp/agentdeck-cross-install-r2-compensation-red.log` 显示 route 变为第三方 `printf prior`，不是迁移前仍启用的旧 route。修复为仅本次成功迁移持有私有旧 entry receipt，`RollbackStatusLineSetup` 恢复旧 route 且保留 prior；普通 setup 回滚复用既有 restore，postcommit 不补偿。后写 route 或整文件删除拒绝补偿，写入前再次检查完整配置快照。

CS-R2-F1/F2 修复：quote-aware 解析仅接受生成的单引号参数及 apostrophe escape/历史 plain words，识别器不执行 shell；same-owner 要求旧/新解析 state root 与实际 Manager state root 同一绝对规范路径，并有有效非托管 prior，保留 prior 文件不重采集。无 prior、坏 prior、历史托管 prior 或用户修改 entry 拒绝迁移，不能静默夺取其他 owner。定向及 parser/invalid-prior/rollback 生命周期测试已通过，日志 `/tmp/agentdeck-cross-install-r2-targeted.log`、`/tmp/agentdeck-cross-install-r2-parser.log`、`/tmp/agentdeck-cross-install-r2-boundaries.log`。三项 findings 待独立复评确认关闭。

## Review — Round 3

## 📋 独立冷上下文复评报告

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无新增阻塞 finding。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- CS-R2-F1 → CLOSED：旧、新命令解析后的绝对 state root 与 Manager root 一致，并有有效非托管 prior；迁移保留 prior 原字节。缺失、损坏、托管 prior 或修改 entry 均拒绝。
- CS-R2-F2 → CLOSED：正确处理生成的单引号、apostrophe escape、空格与 option-like 文本；拒绝额外参数、未闭合引用和 shell fragments。独立 overlay 额外验证引用路径内换行、Unicode 和 shell 特殊字符。
- CS-R2-F3 → CLOSED：成功迁移保存旧 route receipt，precommit 失败恢复旧托管 route，保留原 prior；普通 setup 维持旧回滚，postcommit 不补偿。
- setup 和迁移 rollback 最终替换前校验完整快照；独立注入后写配置，确认失败并保留后写内容和 prior。

### 📝 总结

Reviewer：`/root/cold_rereview`，冷上下文独立只读角色。Method：源码、base diff、调用链、安装迁移契约与全部原 findings 检查，项目 wrapper 定向测试及 `/tmp` 独立 overlay；未修改仓库/Beads/CEv1/Git。主代理直接核对测试源、五文件 blobs 并复跑 overlay 通过后正式接受 PASS。Scope：五个生产/测试文件与同 issue 修复。无专属 completeness checker，使用行为测试和 diff 检查。

Reviewed state：base `570cc9e2c818f6a4da38365194c3cd5c7bcf4e93`；HEAD `f7d6bc4fa81404c30a6a10b2c685c54bef3ec607` 加 scoped Git blobs：`internal/usagehook/config.go=1662f7547edb6d60dd7ec55169a39b35814f9518`、`config_test.go=3ec04eb205c9fb66fe7a7b22743a590e04a80f43`、`cross_installation_test.go=a5ab22ab28f23f0aa63731f43204d48bc8ecdb81`、`cmd/agentdeck/quota.go=32efe1593591b3936521151e968967645d69fb17`、`quota_test.go=9b6f21e7b2923d205faacaf57fbe2aa4e18376ab`。

Evidence：`scripts/run-go-test.sh ./internal/usagehook ./cmd/agentdeck -run 'StatusLine|ManagedAgentDeck'` 通过，完整日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.tYx7jE`；`scripts/run-go-test.sh -overlay=/tmp/agentdeck-cold-rereview-overlay.json ./internal/usagehook -run '^TestColdRereview'` 通过，完整日志 `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.ZqzPOg`；独立源 `/tmp/agentdeck-cold-rereview_test.go`，主代理复跑 `/tmp/agentdeck-cross-install-r3-main-overlay.log` 通过。`git diff --check` 通过。

完成门禁：NOT_VERIFIED（本轮完成时新候选 full/race 和 CEv1 尚待收口）；不继承旧 head 的失败后历史 gate。限制：快照检查到 rename 不是跨进程事务，不声称解决所有竞争；未真实用户状态验收。所有原 findings 关闭且无新增项，建议待 exact candidate gate 完成后按原授权增量提交、推送新 head 并重新请求远程 review/CI。

Round 3 后续收口：新候选 full `./...`、affected race `./internal/usagehook ./cmd/agentdeck`、vet `./...`、build-all 全部通过，完整日志 `/tmp/agentdeck-cross-install-r2-full.log`、`/tmp/agentdeck-cross-install-r2-race.log`、`/tmp/agentdeck-cross-install-r2-vet-build.log`。CEv1 在 Round 3 上述精确五 blob candidate 查询 VERIFIED 4/4，无 missing、invalidated 或 unresolved；旧 f7d6bc4 head 因 Round 2 findings 已另记 FAILED，未覆盖历史证据。Task checkpoint：awaiting_commit；范围为四个增量生产/测试文件及完整 Fix record，贡献者 Codex，review-only 角色不计作者。远程新 head review/CI、合并和 slot 释放仍待完成。
