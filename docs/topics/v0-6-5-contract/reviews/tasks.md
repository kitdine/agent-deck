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
