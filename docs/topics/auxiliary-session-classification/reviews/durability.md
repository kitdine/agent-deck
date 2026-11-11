---
status: active
topic: auxiliary-session-classification
subject: durability
---

# durability 评审记录

## Round 1 — 2026-11-10

## 📋 Task 8 durability 独立评审

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewer: Codex；独立于开发者 claude-code，本会话未参与实现。
- Method: 工作区绑定与开发移交核验、CodeGraph 定位、定向源代码审查、隔离 Go overlay 反例；未委派。
- Scope: Task 8 的导出清洗、恢复校验与绑定重激活；在获得决定性反例后停止扩大验证。
- Reviewed state: HEAD `13fdf29dfd8e589d21fc0774fa3cab868d6fdc0d`，候选指纹
  `4a36b6762b5af2d302c04d6902f96fd3cd0a4f026e2f304877fdb94181cc37b1`。
  算法为以下有序 `path + 空格 + blob + 换行` 字节串的 SHA-256；不含状态与评审记录。

| Path | Git blob |
| --- | --- |
| cmd/agentdeck/session_origin_test.go | 97455df7e7a081ed256f6f5c710c72c17c771f54 |
| internal/backup/backup.go | 36fec5735df108c8885a856326015f506050b0a5 |
| internal/backup/origin_durability_test.go | e770a18c015185857b710ff6691dc39f66a62e5e |
| internal/sessionorigin/reconcile.go | c2769b73bfab552bbaeabd4eabb32488ed50da6e |
| internal/store/session_origin.go | cf0d4a2fd17b3245e17401d997c0df7d8536dbf3 |

### 🔴 严重问题 — 必须修复

**ASC-DUR-R1-F1 — P2：恢复校验只检查行值，接受缺少主键的 origin 表。状态：OPEN。**

位置：`internal/backup/backup.go:263`（`validateOriginState`，尤其末尾重复行检查）。

- 行为风险：一个 schema version 正确、行值合法且无重复行，但缺少 `(client, session_id)`
  唯一约束的 `origin_overrides` 能通过校验。Restore 返回成功并创建目标；后续显式 set
  无法执行依赖该冲突目标的 UPSERT，用户获得不可正常写入的恢复状态。
- 合同依据：`tasks.md` §8 与 `architecture.md` §8 要求恢复前验证新增表/schema；
  恢复后的显式 set 必须能重新绑定当前会话。仅检查现存重复数据不能证明结构满足该合同。
- 证据：在原有 `newOriginSource` 和 `tamperCore` fixture 上，保留每行和列而移除主键：

  ```sql
  CREATE TABLE origin_overrides_loose AS SELECT * FROM origin_overrides;
  DROP TABLE origin_overrides;
  ALTER TABLE origin_overrides_loose RENAME TO origin_overrides;
  ```

  `tamperCore` 重新计算 entry size/SHA-256 并重新加密，保证不是摘要错误掩盖结构校验。
  调用 Restore 到不存在的隔离 target，实际返回 nil；打开恢复后的 core，调用
  `ApplyOriginMutation`，参数为新的 RequestID/PayloadHash、OverrideSet、codex/s-new、
  value=auxiliary、由 `originBinding("codex", "s-new", birthOK)` 生成的绑定及 fixtureNow。
  实际报错：`SQL logic error: ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE constraint (1)`。
  临时 overlay 用例 `TestReviewRestoreRejectsMissingOriginPrimaryKey` 以期望 ErrInvalidArchive
  断言失败，已同时确认恢复成功与其后的真实写入失败。
- 现有测试 `TestRestoreRejectsInvalidOriginState/two_overrides_for_one_session` 同样移除了主键，
  但又加入重复行，只证明重复数据检查有效，不能保护无重复数据时的结构校验。

💡 有界修复：在写目标之前验证 portable origin 表所需的列及关键约束，至少确认覆盖身份唯一性
与 singleton 约束能支持实际写路径；结构不合法时返回 `ErrInvalidArchive`，目标保持未创建/未修改。
增加“合法行、缺失唯一键”回归，并保留旧 schema 合法迁移和正常恢复后显式 set 的正向保护。
不要求扩大为所有历史表的通用 schema 审计。

### 🟡 建议改进 — 推荐

无额外发现。全量 Go 回归未在开发移交中提供，本轮因决定性阻断反例未补跑；修复后的最终候选须完成适用 L3 验证。

### 🟢 优点

清洗发生在私有快照，摘要在清洗后计算；恢复将覆盖置为 unmatched，重激活在协调层和存储条件中均检查绑定。
新增 fixture 覆盖同 ID 不同出生、limited、显式重绑及失败回滚。以上为源代码审查结论，不替代尚缺的完整验收证据。

### 📝 总结

当前候选未满足 Task 8 的恢复 schema 校验合同，Round 1 FAIL；产品代码、测试与配置未修改。

Evidence: 使用 Go 1.27.3，`scripts/run-go-test.sh -overlay <临时 overlay.json> ./internal/backup
-run '^TestReviewRestoreRejectsMissingOriginPrimaryKey$'`，退出 1，用例 5.93 秒、包 7.075 秒。
overlay 仅在仓库测试副本末尾添加上述反例，所有数据库均为临时 fixture；没有使用用户真实数据库。
复现方法与关键输出已完整记于本记录，临时 overlay 和日志不作为持久引用，记录后删除。
未执行全量/race/vet；未将开发者报告转换为本轮已通过证据。

CEv1 WorkUnit: `auxiliary-session-classification:durability`。
Target ContentState: `auxiliary-session-classification:durability:review-r1-4a36b6762b5af2d3`。
完成门禁：FAILED。
失败证据对应 `restore-validation-reset` 与 `independent-review`；其余六项尚无目标态通过证据。
该结论不关闭 Task 或 Topic，不授权提交、推送或修复。

下一步指令：`修复：auxiliary-session-classification / durability / ASC-DUR-R1-F1`

## Round 2 — 2026-11-10

## 📋 Task 8 durability 独立复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Codex，独立于 Repair 1 开发者 claude-code；未修改产品、测试或配置。
- Method: 复用本会话的合同与 Round 1 记录；核对修复移交和候选指纹，CodeGraph 定位、源代码与测试审查，补足此前缺失的全量 Go 回归。未委派。
- Scope: ASC-DUR-R1-F1 的处置、修复影响及 Task 8 完成证据；不关闭 Task 9 或整个主题。
- Reviewed state: HEAD `13fdf29dfd8e589d21fc0774fa3cab868d6fdc0d`；同 Round 1 五路径、同算法指纹
  `176cbe65755ed9c4d0d961246837ba11cbc470a870414280afc8a806605c14d8`。
  `internal/backup/backup.go` blob 为 `461b7668626c28e50b6bcea7ff113fb1eb5164b8`，
  `internal/backup/origin_durability_test.go` blob 为 `c17298e73d661074081305a6e777165e961e3e19`；
  其余三路径 blob 与 Round 1 相同。

### 🔴 严重问题 — 必须修复

无。ASC-DUR-R1-F1 已关闭；没有发现修复引入的新阻断问题。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-DUR-R1-F1 — CLOSED。** `backup.go:263` 在目标创建之前调用 `requireUniqueKey`，
对 preferences 的 singleton 和 overrides 的 client/session_id 检查主键或完整唯一索引；
排除 partial 唯一索引。原始“合法且不重复的行、主键缺失”归档现在返回 ErrInvalidArchive，目标不创建。

`TestRestoreRejectsInvalidOriginState` 覆盖覆盖表缺主键、默认表缺 singleton 键、partial 唯一索引；
原重复身份用例仍保留。`TestRestoreAcceptsAnEquivalentFullUniqueKeyAndStillTakesExplicitSets`
验证完整等价唯一索引可恢复，且实际 ApplyOriginMutation 返回 saved，不以恢复返回 nil 代替可写性证明。
旧 schema 的校验仍在私有副本迁移后执行，不将迁移前缺少新增 origin 表直接判为非法。

### 📝 总结

唯一既有发现已关闭，当前候选复评 PASS。原 Round 1 的 FAIL 保留为旧候选历史事实；
不将其失败证据重标为新内容，也不把它误用于当前候选。

Evidence:

- Go 1.27.3，`scripts/run-go-test.sh -timeout 40m ./...`，本轮直接执行，退出 0。
  `cmd/agentdeck` 226.838 秒，`internal/backup` 165.410 秒。
  缺键与非法状态组 39.40 秒、等价唯一索引正向用例 4.13 秒；导出清洗 3.90 秒、
  出生绑定重激活 2.13 秒、真实 purge-index 保留核心选择 0.09 秒，全部通过。
- 全量覆盖包括有/无 sessions 导出恢复、用户默认与覆盖保留、派生/回执清空和新 epoch、
  同 ID 异出生/limited/未索引不重激活、显式重绑、错误摘要与 schema-ahead 拒绝、
  重置失败回滚、空目标及凭据合同。没有进行真实安装、真实外部插件库或真实用户数据操作。
- `Restore` 的调用路径不调用外部 adapter，也不重建来源定位配置；恢复 fixture 断言来源配置不存在。
  `TestClaudeMemObserverSessionsEndToEnd` 保护禁用来源后覆盖保留；禁用分支未引入删除用量的路径。
- 修复交接在 tasks.md 记录的 `-race -timeout 40m ./internal/backup/` 成功结果绑定同一候选，
  与初次 10 分钟超时明确区分，本轮复用而未重跑。初开发的 sessionorigin race 与 vet
  按未变的三路径和依赖继续复用；这些为既有持久记录的验证结果，本轮新增直接证据为上述全量回归。
- `git diff --check`、`make check-whitespace`、评审记录结构和文档结构检查通过。
  临时测试日志不作为持久引用；必要命令、结果与内容身份已记录于此。

CEv1 WorkUnit: `auxiliary-session-classification:durability`。
Target ContentState: `auxiliary-session-classification:durability:review-r2-176cbe65755ed9c4`。
完成门禁：VERIFIED。
八项 required criteria 均有当前目标态证据；无失效证据或未决候选影响。

Task checkpoint：`ad-auxiliary-session-classification-durability`；候选
`176cbe65755ed9c4d0d961246837ba11cbc470a870414280afc8a806605c14d8`；gate VERIFIED。
提交建议：单独提交 Task 8 的五个产品/测试文件及 tasks.md、当前评审记录；须由用户授权，先核验暂存范围与贡献者。
推送建议：获明确授权并完成提交对象、trailer、SSH 签名与远端分支检查后，推送至 origin/feature/auxiliary-session-classification；当前未推送。

下一步指令：`开发：auxiliary-session-classification / integrated-acceptance`
