---
status: active
topic: snapshot-performance
subject: scan-experience-acceptance
created: 2026-09-13
updated: 2026-09-13
---

# scan-experience-acceptance 评审记录

## Round 1 — 2026-09-13

## 📋 Task 3 实现评审

📊 综合评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SEA-R1-F1 — P2 — 旧 session 索引的组合迁移遗漏 changed_at（OPEN）**

位置：`internal/store/store.go:137`、`internal/store/store.go:142`；受影响查询为
`internal/session/unchanged.go:31`。

- 行为风险：已有 `session_sources.source_path`、但 FTS 尚无 `event_at` 的旧索引，
  在本次新增 `changed_at` 升级中先进入重建 FTS 分支并提前返回，跳过新增列。
  `OpenSessions` 返回后仍缺 `changed_at`，全局扫描的 session planning 失败，
  usage 与 session 均返回 `scan_failed`，CLI 退出 1，App 无法完成此次刷新。
  不能依靠下一次重新打开数据库才补齐本次迁移。
- 证据：隔离旧结构数据库（空源集，无用户数据）执行当前候选的真实 detached-worker
  `--format ndjson scan`，得到 `SQL logic error: no such column: changed_at (1)`；
  之后查询列状态为 `event_at=1`、`changed_at=0`。复现脚本及输出见下文。
  原有 `TestOpenSessionsRebuildsDocumentsWithoutEventAt` 仅创建 FTS 表，不同时创建
  旧 `session_sources`，因此没有保护两个迁移同时需要执行的情况。
- 归因：提前返回本身已存在，但本 Task 新增后续 `changed_at` 迁移及无条件读列，
  使原本可用的组合升级路径首次扫描失败。这不是用户已接受的性能或人工证据缺口。

💡 有界修复：一次 `OpenSessions` 完成所有适用的 session 结构升级，保留 generation
失效语义；增加同时缺少 `event_at` / `changed_at` 且保留旧 source 表的回归，
断言首次打开后所需列齐全、首次全局扫描成功，无需二次打开才能恢复。

### 🟡 建议改进 — 推荐

无。已接受的冷导入、CPU、最终 20 样本及 native/V01-V19 证据缺口不重新列为缺陷。

### 🟢 优点

- 当前候选和隔离 CGO 试验边界清楚，未将试验驱动纳入交付候选。
- ctime 跳过条件与冷源批量发布具备明确回退、事务和差分保护。
- 已有性能失败与人工证据不足被如实保留，未以开发完成决定替代技术通过。

### 📝 总结

- Reviewer：Codex，当前会话正式评审角色；未参与本候选实现，未委派。
- Method：从当前源码、diff、项目契约和原始证据独立核查，并以真实 CLI/worker
  在临时旧结构数据库上复现迁移缺陷。实现交接仅用作定位资料。
- Scope：Task 3 的 48 文件纯 Go/Swift/契约候选，重点核查增量跳过、冷发布、
  migration、CLI/App 事件流及用户批准的例外边界。发现决定性阻断后停止扩大验证；
  本轮不声称已穷尽其余代码问题。
- Reviewed state：HEAD `e1311ce1274a1f820f7834e9eb7e22ae9ea8ffb8`；
  candidate `ed6ff6c0a4a77e372e8545c3350541dd5b89265fd0cb2423f4d8888cd4b78755`。
  已按既有配方重新计算一致：`head=<HEAD>` 换行后接排序的 Git blob / path 行，
  SHA-256；排除本 topic 的 tasks.md 和 reviews。状态同步不改变该候选。
- Evidence：`bash /private/tmp/agentdeck-sea-review.JcRIXB/reproduce.sh`，真实命令退出 1。
  脚本 SHA-256 `e0e7f4fb5ef6f358f11afbbe0fa2e95d39aea8de24934e8cdce096a563f9a660`；
  `stdout.ndjson` SHA-256 `5cd98d9927a5169c737b70d7506605561a94dae02256e0af6b207946a9f3f9ae`；
  `stderr.txt` SHA-256 `0a22d9e27baab6750ca2fff4a5dde3d0694062653e2b17034d94609214359a46`。
  二进制 `/private/tmp/agentdeck-cold-selected-bin/agentdeck` SHA-256
  `822cdaa3c0b16aaaf169c30ce21247903f979de3639ae92f92f9a5da23cbc975` 与候选测量记录一致。
  复现材料保留用于修复交接，未访问 live state。
- Reused evidence：已核验既有 full/race 日志 SHA-256 分别为
  `f2c136dcfa9585b6fd2b3880edcbe10e47ea06c90e1dee5473ceb4a3c4e14c29` 和
  `08aaf4e504d6b032e7ab9221608e3531f48fd68a53bb9bc5b8572145ba4b4bf1`。
  它们仍证明原有测试通过，但不覆盖本次组合迁移复现；未重复运行全量验证。
- 完成门禁：FAILED。当前 ContentState 的 CEv1 技术门禁仍有原始性能 fail、
  native/V01-V19 not_verified；本轮额外记录组合迁移失败，不覆盖或删除旧证据。
  WorkUnit：`urn:ce:agent-deck:work-unit:snapshot-performance-scan-experience-acceptance`。
- 用户决定：沿用 tasks.md 的 2026-09-13 current-state completion decision；
  不启动新的优化、benchmark 或 CGO 采纳。此次失败只要求修复上述新增兼容性问题。
- 状态：Review 保持未勾选；同一 Beads task 返回 `in_progress`，不提交或推送。

下一步指令：`修复：snapshot-performance / reviews/scan-experience-acceptance.md / SEA-R1-F1`

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## SEA-R1-F1 修复 — 2026-09-13

- 修复者：Codex；仅处理 SEA-R1-F1，未启动新的优化、性能 campaign、CGO 采纳、
  commit 或 push。
- 处置：`SEA-R1-F1 -> repaired in candidate`，等待独立复评；Round 1 FAIL 与
  OPEN finding 保留为历史。修复前的真实 detached-worker 复现保持有效：缺
  `event_at` 的 FTS 使 `migrateSessionSchema` 提前返回，因而跳过
  `session_sources.changed_at` 的同次升级。
- 修复：`migrateSessionSchema` 现在累积并完成所有适用结构升级。FTS rebuild、
  source-table rebuild 与 `changed_at` ALTER 各自只在需要时执行，并以一个
  `rebuilt` 结果通知后续 session generation 处理；不再由先发生的升级提前返回。
- 回归：新增真实 `scan` 命令回归，构造同时缺 `session_documents.event_at` 与
  `session_sources.changed_at` 的旧索引（保留旧 source table）。第一次全局扫描
  必须退出 0，随后只读检查两个列均存在。这覆盖原始复现的首次恢复边界，而不是
  仅测试第二次打开。
- 验证：`scripts/run-go-test.sh ./cmd/agentdeck -run
  '^TestScanCommandCompletesCombinedLegacySessionIndexMigration$'`、
  `scripts/run-go-test.sh ./internal/store -run
  '^TestOpenSessionsRebuildsDocumentsWithoutEventAt$'`、
  `scripts/run-go-test.sh ./...`、相关 `-race`（store/session/scanruntime/cmd）、
  `make vet` 与 `make build-all` 全部通过。full Go log SHA-256：
  `a2967b787a18fff769fe870d28803f8af7f2398e9a80ae6731ba6a247a3440b9`；
  race log SHA-256：
  `83f174cdd18419d48817e54db1ae30648a3cb27836d5f05296bc2412710b51ac`。
- 内容：HEAD `e1311ce1274a1f820f7834e9eb7e22ae9ea8ffb8`；Task 3-owned
  49-file repair candidate fingerprint：
  `9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0`。
  配方沿用 Round 1：`head=<HEAD>` 后接排序的 blob-hash/path 行，排除
  `tasks.md` 与 review records。此次 finding 的生产/测试变更仅为
  `internal/store/store.go` 与 `cmd/agentdeck/scan_runtime_test.go`；其余候选
  文件保留为当前 Task 3 已评审边界。
- 完成门禁：仍为 FAILED。Task 3 的既有 performance 与 native/V01-V19 技术
  evidence 状态不因本次兼容性 repair 被改写；用户批准的 current-state
  disposition 与它们的原始失败/未验证事实均保留。复评必须对上述新候选独立
  处置 SEA-R1-F1，不得把该兼容性通过等同为整项 Task 3 technical gate 通过。
- 状态：Task Dev 保持勾选、Review 保持未勾选；同一 Beads task 回到 `in_review`
  等待复评，不创建新任务。

下一步指令：`复评：snapshot-performance / reviews/scan-experience-acceptance.md`

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 2 — 2026-09-13

## 📋 Task 3 修复复评

📊 综合评分：8/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。SEA-R1-F1 已关闭；本轮未发现新的阻断问题。

### 🟡 建议改进 — 推荐

无。用户已经接受的优化与人工验收缺口不重新列为修复事项。

### 🟢 优点

- `SEA-R1-F1 -> CLOSED`：`migrateSessionSchema` 累积 `rebuilt`，重建无
  `event_at` 的 FTS 后继续补齐 `changed_at`；其后由 `OpenSessions` 统一创建
  表并刷新 session generation。原来的组合升级不再依赖第二次打开。
- 新增 `TestScanCommandCompletesCombinedLegacySessionIndexMigration` 同时构造
  两个缺列条件，先执行首次 scan，再只读检查两个列，避免检查本身修复数据库。
  该测试调用实际命令处理链，但使用测试进程内 runtime；不称为独立子进程验收。
- 当前全量 Go 与相关 race 日志均包含上述回归 PASS，且哈希与修复交接一致。

### 📝 总结

- Reviewer：Codex，独立于修复实施的复评角色；未委派。
- Method / Scope：复核 Round 1 唯一 finding、迁移分支组合、generation 通知路径、
  新增回归的首次打开边界及修复后的原始 full/race 日志。复用此前未变化范围的
  证据；不重新执行已通过的相同检查，不宣称穷尽全部 Task 3 潜在问题。
- Reviewed state：HEAD `e1311ce1274a1f820f7834e9eb7e22ae9ea8ffb8`；
  candidate `9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0`。
  已按本记录既有配方重新计算一致。修复增量为 `internal/store/store.go` 和
  `cmd/agentdeck/scan_runtime_test.go`；topic status/reviews 不计入候选指纹。
- Evidence：`/private/tmp/agentdeck-sea-r1-f1-full-go.log` SHA-256
  `a2967b787a18fff769fe870d28803f8af7f2398e9a80ae6731ba6a247a3440b9`，新增回归
  在第 546 行 PASS；`/private/tmp/agentdeck-sea-r1-f1-race.log` SHA-256
  `83f174cdd18419d48817e54db1ae30648a3cb27836d5f05296bc2412710b51ac`，新增回归
  在第 882 行 PASS。原有 FTS 升级回归也通过。旧候选失败证据保留，未改写结果。
- 完成门禁：FAILED。复评时发现修复交接所称新 ContentState 尚未落图，已补录
  当前状态及作用域限定的证据关联。组合迁移修复通过不改变历史性能未达标事实，
  也不补齐最终 20 样本、V01-V19 或 real-helper/manual native 证据。
- 例外边界：tasks.md 的 current-state completion decision 明确批准开发完成，
  同时保留交付前验收边界。复评 PASS 关闭代码 finding；当前技术门禁仍不等于
  用户批准的开发完成决定。不得无授权追加性能测量或将人工检查改记为已执行。
- 状态：Task Review 勾选，Topic review 为 3/3；Beads 保持 `in_review` 等待
  验收边界解决，不进入 `awaiting_commit`，不关闭 Topic。

Task checkpoint：ad-sp-scan-experience-acceptance-dev；content_state=9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0；gate=FAILED。
提交建议：门禁 VERIFIED 后，单独提交 Task 3 当前代码、测试、必要契约、评审和状态；前提是解决已接受缺口的交付验收处置，提交仍需授权。
推送建议：门禁 VERIFIED 后，向 feature/snapshot-performance 的已核实远端目标推送；前提同上，并须先完成获授权的签名提交、远端核查及单独推送授权。

下一步指令：确认 snapshot-performance / scan-experience-acceptance 已列明的性能、最终 20 样本及 V01-V19/native 证据缺口可作为本次交付验收例外，并据此同步验收门禁，保留原始失败和未验证记录。

WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Delivery acceptance exceptions — 2026-09-13

用户明确批准 tasks.md 同名小节所引述的交付验收例外。此项同步解决 Round 2
之后的验收前提，不创建新复评轮次，不改变 Round 2 PASS 或原始技术证据。

- 当前候选：`9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0`，
  本次重新计算一致；HEAD `e1311ce1274a1f820f7834e9eb7e22ae9ea8ffb8`。
- 例外覆盖：冷导入 <=10 s、unchanged CPU <=0.5 s 的已列短缺，最终 20 样本
  campaign，以及 V01-V19 / real-helper/manual native 的已列证据不足。
- CEv1 新增三项明确标识为用户例外处置的 pass observation，supersedes 当前候选
  对应的旧验收评价。它们证明例外获批，不证明性能达标或人工检查已运行。
  原始 fail/not_verified 节点和原始日志保留；不把失败证据作为 passing roll-up。
- 完成门禁：VERIFIED（依据用户批准的验收例外）。其余三项 passing evidence
  复用，无产品修改或新测试轮次。此前 FAILED gate 仍是当时的真实历史结果。
- Beads：`awaiting_commit`；本次未提交、推送或关闭 topic。

Task checkpoint：ad-sp-scan-experience-acceptance-dev；content_state=9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0；gate=VERIFIED（已批准例外）。
提交建议：单独提交 Task 3 当前代码、测试、必要契约、评审及状态文件；须获独立提交授权。
推送建议：获独立推送授权并核实远端后，推送 feature/snapshot-performance 的已验证签名提交。

下一步建议：`提交：snapshot-performance / scan-experience-acceptance`
