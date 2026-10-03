---
status: active
topic: v0-6-5-contract
subject: tasks.md
---

# tasks.md Review

## Round 1 — 2026-09-30

## 📋 v0.6.5 版本契约分解评审

📊 总体评分：9/10

✅ 评审结论：PASS

Reviewer：Codex，独立评审角色。
Method：当前评审回合未参与设计推导；以冻结文档、现行规则、Git 身份和 live Beads 交接为输入，逐项进行文档契约核对和场景走查。历史摘要仅作定位，不作为证据；未委派或执行产品代码验证。
Scope：`docs/topics/v0-6-5-contract/tasks.md` 的文档集、19 Bug 成员、三个聚合任务、修复工作区前置条件、发布基线和交付授权边界。

### 🔴 严重问题 — 必须修复

无。未发现需要修复的范围遗漏、内部矛盾或现行规则冲突。

### 🟡 建议改进 — 推荐

无。串行槽位名称和命令留给明确的 Task 1 决策，不是当前分解缺项。

### 🟢 优点

- 19 项与 roadmap 及 live Beads 完全对应：P1 六项、P2 十一项、P3 两项；均为 open、未指派。会话元数据修复没有夹带辅助会话过滤、用量删除或 worktree 分组政策。
- Task 1 先交付可执行的串行 Fix 规则；Task 2 覆盖逐项交付、行政处置和最终集成交互；Task 3 覆盖稳定契约与版本收尾。每项均有文件边界、依赖和验证级别。
- 修复基线是 peeled v0.6.0；契约分支的较新 main 基线不混入补丁线。实际 release refs 查询、治理规则回移和每次集成分类都保留明确前置条件。
- 冷上下文评审、每 issue 一分支一 PR、槽位冻结、dirty/owner 冲突、晚到发现返回原分支、显式延期和不能复现一次等场景均有处理规则。
- 文档 PASS、CEv1、Beads、Git 交付和发布彼此独立；原 v0.6.0 安装失败及 native/performance 例外没有被该计划改写为 PASS。

### 📝 总结

Reviewed state：HEAD `f4d9f44e3517f323872b2179f10c4dd45c506acb`；输入文档 blob `611cc5f8fb0f64ba2da62e08656a61e177b4ac96`，输入 fingerprint `3740b2b3660446d2f4dfb24813dc77227462df08b1fab38905037936e492d9f7`。
最终同步文档 blob `f9c510d8943b5749e7d854cda50ffb53e2247fc1`；同步仅更新 Documents Review、分解批准表述和当前交接。上述状态变更逐项复核，不改变成员、任务范围或验证要求，PASS 适用于最终文档。

Evidence：

- `git rev-parse HEAD --abbrev-ref HEAD --path-format=absolute --git-common-dir` 与本地 workspace binding 一致，工作树位于当前可写根之下。
- `git rev-parse 'v0.6.0^{}'` 得到 `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`；local/cached origin 的 `release/v0.6.x` refs 为空。未 fetch；不声称远程实时分支缺失。
- actor-qualified Beads `show` 与 comments 核实文档任务及 in_review 交接；`list --label v0.6.5 --json` 核实完整成员、priority 和当前 dispatch 状态。
- 与 `docs/roadmap.md` 的 v0.6.5 清单/修复流程/工作区提案，以及 Branching、Beads、Documentation Workflow、Evidence 和 Project Rules 的 L0–L4 矩阵核对一致。
- `bash scripts/check-topic-docs.sh` exit 0；输入态的 make check-whitespace、git diff --check 和五个本地链接/锚点结果有 exact HEAD/blob 设计交接，适用内容未改变；最终态检查在本轮收口。
- 文档集仅 tasks.md，requirements/architecture/UX 明确 n/a，版本契约不代替各 Bug 的 Lane 裁定或功能设计。

Findings：无。
完成门禁：VERIFIED。
最终 content_state fingerprint：`48cb658cdaba271467a1032de7f229bf2d8b12050c9f0388f88f2bb6444fc0e2`。
ContentState ID：`v0-6-5-contract:tasks.md:state:48cb658cdaba271467a1032de7f229bf2d8b12050c9f0388f88f2bb6444fc0e2`。
最终 `make check-whitespace`、`bash scripts/check-topic-docs.sh` 和 `git diff --check` 均 exit 0。
canonical gate-status.cypher 返回三个 required criteria 全部 pass、target_matches/applicable true；missing、invalidated、unresolved 均为空。
MCP write 响应给出更新统计而不转交 RETURN 行；4 个 WorkUnit/criterion 节点、3 个 requires 关系经 read-back 核实，1 个状态和3 个 evidence 节点及6 条 evidence 关系经 exact-state 门禁读回核实，数量与提交批次吻合。
该文档 WorkUnit 无已登记 parent/children；项目矩阵仍有三个未完成实施 Task，不推断 Topic/Release VERIFIED。
WorkUnit：`v0-6-5-contract:tasks.md`；dispatch：`ad-v0-6-5-contract-doc-tasks-design`。
三个 required criteria：coverage、authority、readiness；本轮在判定前按现行 profile 登记并检查 requires 关系。

局限：本轮只批准分解，不复现或修复 19 个 Bug，不验证可复用槽位运行时，不证明 native、安装、性能或发布验收。三个实施 Task 尚未完成，因此不跨越 Topic/Release 完成边界。

提交建议：文档门禁 VERIFIED 后，只提交本工作树的 tasks.md 和本评审记录；需独立提交授权。
推送建议：文档门禁 VERIFIED 且授权提交、签名及归因核验后，推送 feature/v0-6-5-contract 到 origin 同名分支；计划 PR 目标 main，推送/PR 仍需各自授权。
下一项：`开发：v0-6-5-contract / fix-workspace-policy`，先解析其 dispatch 和阶段前提；本评审不执行下一阶段。

## Round 2 — 2026-10-03

## 📋 v0.6.5 版本范围修订独立评审（V065-LS-R2）

📊 总体评分：9/10

✅ 评审结论：PASS（仅限本次范围修订）

Reviewer：独立冷上下文 Codex CLI 审查角色。
Method：只读文档审查、现行权威核对、Git 身份与基线核验、场景走查及必要的既有源码核对。未参与修订写作，未委派。
Scope：仅 `docs/topics/v0-6-5-contract/tasks.md` 的 scope amendment；其他文件仅作为依据。

### 🔴 严重问题 — 必须修复

无。未发现需要记录为 `V065-LS-R2-Fn` 的可执行修复项。

### 🟡 建议改进 — 推荐

无。新功能的具体字段、分类规则与输出值属于后续独立契约设计，本轮不替其作出决定。

### 🟢 优点

- 第 10–14、89、114–125 行准确记录用户选择：仅将原 P2 Bug 对应的 `launchservices-recovery` 纳入 v0.6.5，调整 repair-only 与 new-doctor-API 排除项；其他成员和排除范围保留。
- 第 38–60、127–147 行区分版本载体与 Lane B 功能载体，选用已核验补丁线基线，并禁止将契约分支及无关 main 内容整体带入 release。
- 第 130–141 行将身份、真实冲突与合法副本、残留及未知注册、枚举失败、quick/full、输出与 health、人工指导及验收列为设计必答项，同时保留只读边界。
- 第 12–14、168–178 行保留 requirements、surface、architecture、decomposition 的独立审批前置条件。版本文档 PASS 不批准尚未存在的新功能契约，也不批准产品实现。
- 第 100–108、175–200、328–350 行保留原 Bug 的交付处置、独立审查、CEv1、Git 和发布边界。旧 intake、评审与 handoff 已明确作为历史来源，没有被当作当前 Bug 状态。

### 📝 总结

**精确审查身份：**

- Workspace：`/Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/v0-6-5-contract`
- HEAD：`1b4d145544f89ce4fabb597d1f3bed88d48ab3cc`
- 工作树文档 blob：`6e48a92a92e75e678cc512d40c491251159e0c99`

三者与委派冻结身份一致；初始状态仅目标 tasks.md 有未提交改动。

**Evidence：**

- 已读取本地 AGENTS.md、文档工作流、审查记录、相关 Branching／Beads／Evidence 规则及 development-workflow 的审查呈现契约。
- 本地 `feature/launchservices-recovery` 与 `origin/release/v0.6.x` 均指向 `24623ec8bf0e172bf8e630ebe203163e76634403`；本地 release ref 为旧的 `b864ce41b8d6ce22c475fbc42db6c5cad6b19f6b`。所选基线具有已发布 v0.6.0 的祖先关系。
- 冻结 HEAD 的 `internal/extension/extension.go:280`、`:383`、`:417` 确认 `extension_duplicate_id` 属于扩展清单 ID 重复语义，支持本修订禁止将其直接复用于 OS 资源的边界。
- `bash scripts/check-topic-docs.sh`：exit 0。
- 目标 `git diff --check`：exit 0。
- `make check-whitespace`：exit 2；只读沙箱禁止创建临时文件清单，未取得该检查的通过证据。

**剩余风险与边界：**本轮未重新查询远端实时 HEAD、Beads 或 CEv1，未执行产品测试及 macOS 注册验收。后续集成仍须重新解析目标；新主题契约及其实现、审查、CI、merge-commit 和归档边界均需各自完成。

**完成门禁：NOT_VERIFIED。**此处表示委派明确的“待主写者处理”，不是 CEv1 查询结果。本次 PASS 仅认可版本范围修订，不关闭 Bug、Task、Topic 或交付边界。

自动审批拒绝了独立审查工作状态的持久化写入，理由是本次只授权只读检查。写入未发生，也未重试；本报告直接返回，由主写者处理权威记录。

### 主 writer 收口

独立 reviewer session `01a10171-159e-7f82-b3ef-0e85ff00900b`，实际 metadata 为 custom / gpt-6.1-sol / xhigh / on-request / read-only，与主 session 的 provider 相同；CLI 正常 exit 0。服务端计费与响应 tier 未验证。主 writer 复核冻结文档及 release 基线祖先关系；接受本轮无 finding 的限定 PASS。

最终同步仅勾选 Documents Review 并替换当前交接为已批准、待证据/交付，未改本轮评审的 membership、doctor 边界、排除项、authority 或新 topic 前置契约。独立评审结论适用于同步后的文档；旧 Round 1 不被复用于新增 scope。主 writer 补齐必要 L0 和 canonical exact-state CEv1 后才推进 document delivery。

最终同步文档 blob：`1943d78dfc16850b41a1a8ae05f9b7bb40dac40e`。
ContentState：`v0-6-5-contract:tasks.md:state:7452e56ea15dab3f86164763aec5d6fc8ed6b49b6b774ea671def5a3d25d803b`。
完成门禁：VERIFIED，3/3 required criteria；missing、invalidated、unresolved 均为空。
主 writer 的最终 make check-whitespace、git diff --check、check-topic-docs.sh 均 exit 0，八个本地链接目标存在。
9 个追加节点及 16 条关系经标准 MCP templates 写入、关系 preflight 和精确 payload read-back 核实，数量匹配。
coverage/authority 通过明确保留评估及 target-bound roll-up 复用本轮冷评审；readiness 使用最终 L0。历史 Round 1 未用于本次新增 scope。
WorkUnit 当前没有已登记的 parent/children；不推断版本 topic、aggregate assembly 或 Release 完成。
详细收据：`/tmp/agentdeck-doctor-v065/version-final-validation.json`、`version-final-gate.json`，冷评审报告与 CLI receipt 位于同目录。

Task checkpoint：`ad-v0-6-5-contract-doc-tasks-design` / `v0-6-5-contract:tasks.md`，门禁 VERIFIED。
提交建议：仅此 worktree 的 tasks.md 和本评审记录；本轮真实用户已授予必要 signed commit 交付权限。
推送建议：actual commit message/body/tree/Codex trailer/SSH signature 核实后，ordinary push 同名 feature 分支，Draft PR 目标 main；当前真实用户已授权 exact-head Codex review、required CI 和 merge-commit，发布仍排除。
下一项：新 `launchservices-recovery` requirements 及其独立契约 progression；本 PASS 不批准任何新 doctor 产品实现，也不关闭 origin Bug。
