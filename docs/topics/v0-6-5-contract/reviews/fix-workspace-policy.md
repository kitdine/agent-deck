---
status: active
topic: v0-6-5-contract
subject: fix-workspace-policy
---

# fix-workspace-policy Review

## Round 1 — 2026-09-30

## 📋 串行 Lane A Fix 工作区规则评审

📊 总体评分：9/10

✅ 评审结论：PASS

Reviewer：Codex，独立评审角色；未参与本 Task 的实施推导。
Method：冻结五文件候选，按批准的 Task 1、现行工作流 workspace 契约及源代码进行规则核对与场景走查。历史交接只用于定位，使用 live Git、Beads 和 CEv1 验证当前状态；无委派。
Scope：`.agent-instructions/branching.md`、`.agent-instructions/toolchain.md`、`.agent-instructions/beads.md`、`docs/roadmap.md`、本 topic 的 `tasks.md`；附带本评审记录。不修改规则实现、产品、测试或配置。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。真实槽位创建/切换验收属于另行授权的执行，不由本轮规则评审虚构。

### 🟢 优点

- 明确 `进入工作：fix / <slug>`、默认 `.worktrees/fix`、issue 分支与 `agent-deck.fix.<slug>` 绑定；槽位身份不冒充租约或第二个注册表。
- 新分支来自当前受支持补丁线，release line 缺失先取得创建授权；已存在分支保留历史及 creation base，不用当前 release HEAD 重置。
- 首次进入、同 issue dirty 重入、不同 issue clean 切换、活跃 session 冲突、分支已在其他 worktree、非 worktree 路径冲突、显式暂停、晚到发现返回和部分失败均有具体处理。
- checkout 成功并验证真实路径/分支/common Git/可写根后才替换绑定；失败不发布过时 workspace-ready；未完成 issue 的释放必须保留状态和返回路径。
- CodeGraph 属于实际槽位，分支切换后同步，不借用旧 issue 或其他工作树索引；失败降级和非破坏性退出与现行规则一致。
- 规则归属集中于 Branching；Beads/Toolchain 仅补对应职责，roadmap 改为权威指针。未扩大到共享 runtime、Hook 或产品代码。

### 📝 总结

Reviewed state：HEAD `ac2faf7fe00d3eb5cfbc75330b196c54390052e2`；输入五文件 fingerprint `e425b88abc04e7ca0e08a88045df83ddb067050eff2d00152f423f165fe95175`。
最终同步 fingerprint：`b12279134480db3d508dd69bf0b6720453324d017c1604406d989526054a885b`。
Recipe：`sha256(head=<HEAD>;sorted-path=<git-blob>...)`，路径依次为 beads.md、branching.md、toolchain.md、docs/roadmap.md、topic tasks.md，使用实际完整路径。仅 tasks.md 的 Review/交接表述同步；四个规则/指针文件及 Task 实施范围均保持不变。

Evidence：

- live 分支、HEAD 与 workspace binding 一致；Beads `ad-fix-workspace-policy` 为 in_review，有五文件冻结交接；重新计算输入 fingerprint 与交接一致。
- canonical CEv1 gate 查询确认 required coverage、authority、l0。输入 l0 证据有效且 target_matches/applicable；coverage/authority 原先缺失，由本轮补充。
- 对批准的 Task 1 范围及共享 Skill 的 `references/workspace.md` 核对，命令匹配、根包含、common Git、分支、base_commit、marker 和授权边界一致。
- 只读检查共享源码 `ai-tools/hooks/development-workflow/workspace_context.py`：WORKSPACE_ID/WORKSPACE_MARKER 允许点和连字符，binding_path 按 repository+workspace_id 隔离，validate_binding 验证 live 分支/common Git/base_commit/root。各 issue 可保留独立绑定和创建出处。本证据是源码兼容性，不是当前客户端运行验收。
- 输入 L0 检查及五个新增链接/锚点复用有效 CEv1 证据；最终同步后检查 Review 记录和新增 topic 链接，不重跑不变产品检查。

Findings：无。
完成门禁：VERIFIED。
最终 `make check-whitespace`、`git diff --check`、`bash scripts/check-topic-docs.sh` 均 exit 0。
ContentState：`v0-6-5-contract:fix-workspace-policy:state:b12279134480db3d508dd69bf0b6720453324d017c1604406d989526054a885b`。
节点写入统计为4、关系 preflight 七条全部 ok、写入统计为7；canonical gate 读回三个 required criteria 的 target_matches/applicable 均 true，missing/invalidated/unresolved 均为空，登记数量与预期一致。
WorkUnit：`v0-6-5-contract:fix-workspace-policy`；dispatch：`ad-fix-workspace-policy`。
图中该 Task 无已登记 parent；项目矩阵仍有 assemble 和 v0-6-5-contract 两项未完成，不跨越 Topic 完成边界。

局限：没有创建 Fix 槽位、分支或 release line，没有运行真实 entry/reentry，未证明安装版共享 runtime 已执行新规则。首次使用必须满足治理规则交付、release line 和用户 entry/stage 权限；不会由 PASS 自动取得。

提交建议：门禁 VERIFIED 后提交五文件 Task 边界及本评审记录，需独立授权。
推送建议：门禁 VERIFIED、授权提交且签名/归因核验后，推送 origin/feature/v0-6-5-contract；规则 PR 目标 main，推送/PR 另行授权。
下一项：`开发：v0-6-5-contract / assemble`；先满足 Task 1 交付、release line 和各 issue 的独立前置条件，不在本轮执行。
