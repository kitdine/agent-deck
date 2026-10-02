---
status: active
created: 2026-10-02
---

# 缺陷：reading-off 撤下路由后设置保存失败未恢复

## 现象

Beads: `ad-bug-quota-reading-off-route-not-restored-on-save-failure`。Lane A，恢复既有外部路由与已保存 consent 一致性契约。工作区 `agent-deck.fix.quota-reading-off-route-not-restored-on-save-failure`，串行 slot `.worktrees/fix`；分支 `fix/quota-reading-off-route-not-restored-on-save-failure` 从实时远端 `release/v0.6.x` 的 `6e8c1c4d2942782152f5e6b5de89b0942fc57ce4` 创建。上一 Bug 已交付并明确释放 clean slot。

隔离 HOME/state，已有第三方 prior 和无关配置，reading on 并注册本安装 statusLine；注入普通 settings-save 错误不写 DB，再执行 `desktop quota-settings --reading off`。DB 保留 reading/consent，但外部 route 变为 `printf prior`。

## 根因

`runDesktopQuotaSettings` 先调用 `RestoreStatusLine` 修改外部文件，普通 `saveQuotaSettings` precommit 错误直接返回。邻近 statusline-disable 的补偿不能覆盖本入口。

## 修复边界

仅本次 `OutcomeRemoved`（精确本安装 route 成功撤下）记录 Manager 私有 receipt。保存失败时用同一 Manager 补偿；当前 route 必须仍等于本次恢复的 prior/absence，避免覆盖后写第三方或其他安装。恢复被撤下的原始 entry，不重新采集 prior；保留当前无关配置和 prior 文件。无 receipt、absent、restore_incomplete 不安装。补偿错误与原保存错误 join。`ErrSettingsSecureFilesFailed` 保持 postcommit 新设置、可解码输出与 advisory，不补偿。覆盖同入口 Claude-disable；不改 schema/wire，不扩展跨安装 ownership，不修改其他入口。

## 验证

- 原始 RED（只新增测试，产品代码未改）：`scripts/run-go-test.sh ./cmd/agentdeck -run '^TestDesktopQuotaSettingsReadingOffReinstallsRouteWhenSettingsSaveFails$'`，失败为 route=`printf prior`，DB 设置和 observation 不变；完整日志 `/tmp/agentdeck-quota-route-red.log`。
- GREEN 同命令：`/tmp/agentdeck-quota-route-green.log`。初次 GREEN 到达无关配置断言，测试误要求格式化空格；修正为原 fixture 的保留字节后通过，此 fixture 修正不改变原 RED 缺陷断言。
- 边界 tests：`scripts/run-go-test.sh ./cmd/agentdeck ./internal/usagehook -run 'TestDesktopQuotaSettings|TestReinstallRestoredStatusLine'`，通过；`/tmp/agentdeck-quota-route-boundaries.log`。两入口 precommit/postcommit、撤路由失败、补偿失败、后写第三方/其他安装、absent/modified、prior/无关配置/observations 保留；Manager 幂等与无 receipt 不安装。复用既有只读 settings-load 与幂等覆盖。
- 受影响 package、全量 Go、race/vet/macOS CLI/whitespace 验证进行中；最终结果在完成后补录。
- 所有行为验证使用隔离 fixture，无真实用户数据库重建或日志/usage 删除。

## Review — Round 1

## 📋 独立冷上下文评审报告

📊 综合评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**QR-R1-F1 — P2，OPEN：补偿读取快照后的配置写入会被覆盖。**

位置：`internal/usagehook/config.go:1185`。补偿读取 route 后直接调用 `writeAtomic`；临时文件创建/写入/同步期间第三方或其他安装写入的新 route 和无关字段被旧快照覆盖。独立 reviewer 在 `createTemp` seam 写入 `printf later` 与 `agentdeck --state-dir /other quota capture` 两种 route，均被覆盖，`unrelated.later` 同时丢失，仍返回 `OutcomeConfigured`。现有后写测试只覆盖补偿读取前。

💡 有界修复：临时文件完成后、最终 rename 前重新核验完整快照，变更时拒绝覆盖并报告补偿失败；确定性回归。无需跨安装 ownership 重做。

**QR-R1-F2 — P2，OPEN：补偿重建已删除配置文件且权限为 `0000`。**

位置：`internal/usagehook/config.go:1167,1187`。成功撤下恢复为 key absence 后，若整个文件被删除，`readSnapshot` 返回 exists=false/mode=0，route absence 仍满足比较，补偿创建 mode=0000 文件并报告成功，普通进程不能读取。独立隔离 fixture `Setup → Restore → os.Remove(settings.json) → Reinstall` 确定复现。

💡 有界修复：区分已有文件缺少 key 和整个文件消失；拒绝后者并回归测试。

### 🟡 建议改进 — 推荐

无。测试缺口属于上述必须修复问题。

### 🟢 优点

私有 receipt 只在精确本安装 route 成功撤下时产生；absent/incomplete/no receipt 不盲装。原始 entry 和 prior 保留。原保存错误保持 `errors.Is`，补偿错误 join。两入口 pre/postcommit 有覆盖；postcommit 新设置、解码结果与 advisory 保持。原回归也检查设置、observations 和无关配置。

### 📝 总结

Reviewer：`/root/cold_review`，独立冷上下文只读角色，未参与候选实现。Method：源码/diff/契约/测试审查、现有定向测试与 `/tmp` overlay 确定性复现；未改仓库、Beads 或 Git。Scope：四个生产/test 文件和相关契约/fix carrier。项目无此代码 class 专属 completeness checker，使用定向测试及 `git diff --check`。

Reviewed state：HEAD `6e8c1c4d2942782152f5e6b5de89b0942fc57ce4`，四文件 SHA-256 manifest fingerprint `286e611470d79965a6182388e2a72c9d55c3d3723dec7a52749047a0d4c62781`；reviewer 复现前后均核验一致。

Evidence：既有定向测试通过 `/tmp/agentdeck-quota-route-cold-review.log`；独立失败日志 `/tmp/agentdeck-quota-route-cold-review-race.log`、`/tmp/agentdeck-quota-route-cold-review-deleted.log`；测试源 `/tmp/agentdeck-quota-route-cold-review-race_test.go` 和 overlay `/tmp/agentdeck-quota-route-cold-review-overlay.json`。Main agent 读取测试源后直接运行 `scripts/run-go-test.sh -overlay /tmp/agentdeck-quota-route-cold-review-overlay.json ./internal/usagehook -run '^TestColdReviewReinstall'`，同样复现两项 FAIL，日志 `/tmp/agentdeck-quota-route-r1-main-red.log`，正式接受该 FAIL。

完成门禁：NOT_VERIFIED。两 finding 未关闭，本轮无 passing review evidence。

限制：隔离 fixture；未真实用户状态验收。确认阻塞问题后 reviewer 不重复全量/race/build。最终校验与 rename 的不协调外部写入仍有极短窗口，本轮不要求完整锁协议。

下一步：在同 Bug 内修复 QR-R1-F1/F2 后独立复评。

Round 1 修复：在最终 rename 前校验完整原快照（包括文件存在、mode 和字节），期间后写 route 或无关配置均拒绝覆盖并报告补偿失败；整个 settings 文件消失直接拒绝，不重建。生产代码不改变其他入口写入语义。主 agent 将两项独立 overlay regression 纳入正式 tests 并要求 OutcomeFailed。`/tmp/agentdeck-quota-route-r1-green.log` 通过。QR-R1-F1/F2 待独立复评确认关闭。

## Review — Round 2

## 📋 独立冷上下文复评报告

📊 综合评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。无新增范围内阻塞问题。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- QR-R1-F1 → CLOSED：`config.go` 的 `writeAtomicChecked` 在临时文件完成后、rename 前重新核验文件存在性、mode 和完整快照；确定性期间后写第三方/其他安装 route 与无关字段均被保留，返回 OutcomeFailed。正式 `TestReinstallRestoredStatusLinePreservesWritesDuringCompensation` 独立重跑通过。
- QR-R1-F2 → CLOSED：明确拒绝整个配置文件消失；不创建 mode=0000 文件；正式 `TestReinstallRestoredStatusLineRejectsDeletedSettings` 独立重跑通过。
- 仅本次 OutcomeRemoved 使用同一 Manager 私有 receipt 补偿；无 receipt/absent/incomplete 不盲装。原始错误 errors.Is 保持，补偿错误 join；prior/配置/observations 保留。两入口 pre/postcommit 验证，postcommit 保留已提交设置/输出/advisory。

### 📝 总结

Reviewer：`/root/cold_rereview`，新的独立冷上下文只读角色，未参与实现/修复。Method：当前源码、Round1 全历史、C3/C9 契约和 store postcommit 语义及测试审查；独立隔离 fixture 定向运行，无仓库/Beads/graph/Git 写入或下游委派。Scope：四个冻结生产/test 文件及本次补偿/receipt 支持，不扩展跨安装 ownership。当前代码 class 无专属 completeness checker。

Reviewed state：HEAD `6e8c1c4d2942782152f5e6b5de89b0942fc57ce4`，四文件 SHA-256 manifest fingerprint `fa6808627e6cbfcfef3539c59e40fd30e24ffb332b41e8acac46f5384f5daa0c`；reviewer 在运行前后独立核验一致。Main agent 读取实际复评日志、核验四文件指纹并结合自己的 R1-GREEN 直接证据接受该 PASS；QR-R1-F1/F2 正式关闭。

Evidence：`scripts/run-go-test.sh ./cmd/agentdeck ./internal/usagehook -run 'TestDesktopQuotaSettings|TestReinstallRestoredStatusLine'` 退出 0，两 package 所选测试通过（含 F1/F2），`/tmp/agentdeck-quota-route-r2-cold-rereview.log`。Main 指纹比对通过。最终全量/race/vet/build/L0 结果在本 carrier 验证节补录。

完成门禁：VERIFIED。Neo4j MCP 使用正式 gate-status 模板对 `fix:quota-reading-off-route-not-restored-on-save-failure` 与 exact candidate `fix:quota-reading-off-route-not-restored-on-save-failure:candidate:fa6808627e6cbfcfef3539c59e40fd30e24ffb332b41e8acac46f5384f5daa0c` 查询：4/4 required criteria 有有效 pass，missing/invalidated/unresolved 全空。代码/test scope 的 recipe 为 HEAD + 四路径 SHA-256 manifest；review/history carrier 与 L0 独立检查，不以其 gate 状态文本引入循环 identity。

限制：最终快照检查与 rename 之间仍有不协调外部写入的极短窗口，与 Round1 接受的最小 guard 边界一致，不引入完整锁协议。仅隔离 fixture，不执行真实用户状态验收；reviewer 不重复全量/race/build。

Task checkpoint：`fix:quota-reading-off-route-not-restored-on-save-failure`，上述 HEAD 与 scoped manifest；独立 review PASS，required gate VERIFIED。
提交建议：仅四个生产/test 文件和本 fix carrier，gate 已 VERIFIED，按已授权范围签名提交。
推送建议：仅本 issue 分支至 origin，gate/signature/实际 staged scope 核验后按已授权 PR→current-head review→CI→merge commit 流程交付。

最终验证（Round2 候选指纹不变）：

- 受影响四 package 回归、全量 Go、受影响 cmd quota/statusline race 与 usagehook/quota/store 全 package race 全部通过。首次全量的 sandbox Unix socket 限制在正规授权环境解除；无产品修复绕过环境失败。初轮全 package race 通过但旧候选，不用于替代最终相关 race。
- `make vet`、`make build-all`（darwin arm64/amd64）、`make check-whitespace`、`git diff --check` 全通过。新 guard 仅用于补偿；其他 writeAtomic 调用保持 nil guard 路径。
- `/tmp/agentdeck-quota-route-final-evidence.json` SHA-256 `ac5a9f6031c554c3f322b1d72558d25267341f8423d5d50438dd9af2dd78a6c0`，保留以下完整日志与 digest：

| 日志 | 结果 | SHA-256 |
| --- | --- | --- |
| `/tmp/agentdeck-quota-route-red.log` | expected fail | `b77d447f9f5acd914dd7fbe96afad9875407a69a0a48d450ed30212aea4a95fd` |
| `/tmp/agentdeck-quota-route-r1-main-red.log` | expected fail | `6985eb6d7420ae59ddbc66fa5e90d796c763c5174a86ff8e8be92aefad587a18` |
| `/tmp/agentdeck-quota-route-r1-green.log` | pass | `4881f4223f065914743148b80e520cc174012b4e3f46d074c90c23480dc86449` |
| `/tmp/agentdeck-quota-route-r2-cold-rereview.log` | pass | `3b655cd554dfea643582c873819836cc18c27d337cbf3c5019618d9275722c19` |
| `/tmp/agentdeck-quota-route-final-packages.log` | pass | `c024063bb20308c95061770ab3376fde41e02f1ebd60bda9129499b98d0c7c06` |
| `/tmp/agentdeck-quota-route-final-full.log` | pass | `b8114007ce1732f73b5b6db1adba283bbc8f7c05036ac78ffc4082dd1e027774` |
| `/tmp/agentdeck-quota-route-final-race-cmd.log` | pass | `e1a49442b8427f756377996b746e413dd1ff7dfc7177eefa2a049590b78524cc` |
| `/tmp/agentdeck-quota-route-final-race-internal.log` | pass | `9262629a0909b9654d536877e617f42035fac18e398e961ddabbe2b67162219e` |
| `/tmp/agentdeck-quota-route-final-vet.log` | pass | `205919bcb16cbc1b00590db5da121019b7d46ed23bf1de2fa52a2cdb071417f8` |
| `/tmp/agentdeck-quota-route-final-build.log` | pass | `e23d869f1476686a39d05474392101daa7e8b560a834e4c6d573786a6d27103c` |
| `/tmp/agentdeck-quota-route-final-whitespace.log` | pass | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
