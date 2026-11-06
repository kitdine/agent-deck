---
status: active
topic: installed-state-ownership
subject: requirements.md
---

# Requirements Review

## Round 1 — 2026-11-05

### 📋 需求评审报告

📊 总体评分：7/10

✅ 评审结论：FAIL

- Reviewer: Codex；独立于本批文档作者的当前评审会话，未委派。
- Method: 设计契约评审、原始缺陷交叉核验、所需源码调用路径核验及临时目录内的设计规则反例；未执行真实安装、真实数据操作或产品实现。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。
- Scope: 本文的来源、G1–G8、A1–A8、非目标和文档边界；同时核对 CLI、架构及任务初稿。
- Reviewed state: blob `386f661f633da7c2d5bb765640228c0ec1c581f3`；fingerprint `d2a899d7e280b17dd466b4610890d4335611c4d4007b5a79a3d9b28ba2df43a1`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:requirements.md`。
- Target ContentState: `installed-state-ownership:req:review-20261105-a0fXv0-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 查询返回本 WorkUnit / ContentState 的 FAILED；review 失败证据 applicable=true，L0 通过证据有效；invalidated_evidence 与 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 全部 exit 0；主题集合检查也为 exit 0。静态检查不代表语义评审通过。

#### 🔴 严重问题 — 必须修复

**ISO-REQ-R1-F1 — P2 — 来源记录把未确认的具体触发者写成已实测事实。**

- 位置：[requirements.md](../requirements.md#来源) 第 14、19 行。
- 行为风险：后续调查和回归范围会基于未经确认的事故归因，误把可导致升级的路径当成已锁定的实际触发进程。
- 证据：来源 Beads `ad-bug-dev-schema-migrates-release-state` 的现行 description 明确写道：“Which development process ran the upgrade, and when, was never established.” 它确认的是正式版支持 schema 30、真实库为 31、scan-performance 候选包含迁移 31，以及缺少隔离这一根因。本文却以“都是用真实环境实测到的”引出表格，将触发者写成该工作区开发构建已实际打开真实库。
- 💡 修复建议：保留已确认的 schema、失败输出和恢复决定；明确具体进程及执行时间未知，将 scan-performance 路径标成具备触发条件或推断，不新增未经验证的事故事实。
- Disposition: OPEN。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

安装身份与版本号分离；真实状态和客户端配置写入分别定义；保留 schema_ahead 与禁止降级边界；不将文档批准视为来源缺陷关闭。

#### 📝 总结

本轮评审覆盖全部需求边界。当前代码的默认状态根和无安装身份的迁移链已由 CodeGraph 定位并核实：`cmd/agentdeck/main.go:1699`、`internal/platform/state.go:48`、`internal/store/store.go:376-422`。源码事实支持缺陷本身，但不能证明具体事故进程。上述来源缺陷未关闭，因此本文件 FAIL。

依赖范围为同一 HEAD 上的本主题 CLI、架构和任务初稿；任务状态同步仅增加评审指针，不改需求。相关架构反例归属 [架构评审](architecture.md)，不把它们重复计为本文件的独立发现。

Evidence: `agentdeck-bd show ad-bug-dev-schema-migrates-release-state --json` 与 comments（均通过 actor-qualified wrapper）提供上述事故不确定性及用户决定；源码调用链与当前 HEAD 一致。L0 的最终结果随本轮记录收口登记到 CEv1，静态通过不能关闭本发现。

修复范围：ISO-REQ-R1-F1；保留原始事件的不确定性。

## Repair 1 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-REQ-R1-F1；同时把 2026-11-05 修复期间的两项用户决定同步进需求（见下）。只修改 `requirements.md`。
- Beads：`ad-installed-state-ownership-doc-req` 由 claude-code 领取修复。

### ISO-REQ-R1-F1 处置：已修复，待复评

- [来源](../requirements.md#来源) 的表格改为分列"实测"和"推断"：
  - 已安装版本支持的 schema、真实库的 schema 和故障输出都保留为实测事实；
  - 10-18 那一行的触发者标为**未取证**，并引用来源缺陷描述的原文（"Which development process ran the upgrade, and when, was never established."）；scan-performance 的迁移 31 只写为"具备触发条件，属于推断"。
- 新增一段说明：本主题的必要性只依赖源码已核实的缺陷，即未安装进程能按默认路径打开并迁移真实库，不依赖第二次事故的具体触发者。
- 没有新增任何未经验证的事故事实。8-30 那一行保留"已取证"，依据是 schema-version-signal 需求中已经记录的测量结果。

### 随修复同步的用户决定

在修复架构发现 F1 和 F2 时，用户于 2026-11-05 选择了"formula 安装后显式认领"和"随时复核 + 停机交接"。需求作如下对应修改：

- G7 改为"只有一个回执、一个拥有者"，加入 formula 认领；
- 新增 G9，要求所有权交接安全；
- 术语表中"安装回执""已安装进程"的定义随之调整；
- A2 改为"认领过的 formula"；
- A6 列出旧 keg、多个前缀、stable 与 rc 同时安装、回执过期、在不同安装方式之间迁移这几种情形；
- 新增 A9，要求接管后旧拥有者不能再访问；
- 影响范围中新增 `install claim`；
- 前提中 formula 的回执方式改为认领。

### 验证

- 结构检查：`make check-whitespace`、本主题四份文档的 `git diff --check`、`bash scripts/check-topic-docs.sh installed-state-ownership`，均为 exit 0。
- `check-docs.py` 不会检查未跟踪文件中的链接；这一点在设计阶段已用一个故意写坏的链接证实。所以四份文档的 32 个本地链接和锚点逐一核对过，全部有效；评审记录中指向文档锚点的链接也都有效。
- 以上只是结构检查，不代表语义评审通过。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`requirements.md` blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；按 Round 1 的配方（SHA-256 of `head=<HEAD>;document=<blob>`）计算的指纹为 `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。用同一配方重算 Round 1 的 blob，得到 `d2a899d7…`，与记录一致，说明配方无误。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 2 — 2026-11-05

### 📋 需求复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；复核 claude-code 的 Repair 1 候选，未委派，未修改设计正文、产品代码、测试或配置。
- Method: 按原发现逐项处置；复用相同 HEAD 的源码事实，并检查修复新增的契约。设计反例与生产运行验收严格区分。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；fingerprint `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:requirements.md`；Beads: `ad-installed-state-ownership-doc-req`。
- Target ContentState: `installed-state-ownership:req:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：VERIFIED
- Gate evidence: Neo4j 固定 gate-status 对本轮精确目标返回 VERIFIED；本轮 review/L0 证据均适用，旧状态证据 target_matches=false；无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` exit 0，覆盖未跟踪 Markdown 的链接、记录解析及内部 whitespace/diff/topic 检查；显式主题集合检查也为 exit 0。
- Scope: ISO-REQ-R1-F1，以及修复同步的唯一回执、formula 显式认领、安全交接 G9/A9 和验收边界。只批准需求边界，不代表其架构方案已经实现这些要求。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-REQ-R1-F1 — CLOSED。** [来源](../requirements.md#来源) 第 14–23 行已把 10-18 的具体进程和时间标为未取证，保留来源缺陷的原句；候选含迁移 31 仅作为具备触发条件的推断。缺陷必要性转而建立在可核验的默认状态根与迁移链上，原发现关闭。

G7/A6 明确唯一用户回执与 formula 认领；G9/A9 把“旧拥有者停止访问后才可接管”作为需求，而非以一次启动时的身份判定代替生命周期。用户选择的出处由 Repair 1 和 Beads 修复交接记录关联，没有擅自改成版本号或构建标记授权。

#### 📝 总结

原发现 1 项关闭，无新增本文件发现。需求要求安全交接，架构是否满足由 [架构 Round 2](architecture.md#round-2--2026-11-05) 判定；后者 FAIL 不会把清晰的需求目标本身判为缺陷。其他三份文档仍待修复，不能启动实现或关闭来源缺陷。

Evidence: 本轮直接核对 Repair 1 对应 blob 与本文；复用同一 HEAD 下的默认状态根、迁移、schema_ahead 事实及来源缺陷记录。主题集合检查 exit 0；其余最终 L0 和精确态门禁在本轮收口字段登记。任务矩阵只更改本文件 Review 和简短状态指针，不影响本文件内容及证据。

Task checkpoint：ad-installed-state-ownership-doc-req；content_state `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`；gate VERIFIED。
提交建议：经用户明确授权后，提交本需求文件、其评审记录及必要的主题状态片段；其余失败设计不混入本需求的通过边界。也可在后续修复完成且内容仍有效时按授权成批交付。
推送建议：目标为 origin/feature/installed-state-ownership；需用户显式推送授权，并完成对应提交对象、签名及内容范围检查。当前门禁已 VERIFIED，尚未执行提交或推送。

## Round 3 — 2026-11-05

### 📋 需求复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；独立复核 claude-code Repair 2，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；fingerprint `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。
- WorkUnit: `installed-state-ownership:requirements.md`；Beads: `ad-installed-state-ownership-doc-req`。
- Target ContentState: `installed-state-ownership:req:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：VERIFIED
- Method / Scope: 核对需求内容未变，并评估 Repair 2 对已批准需求边界的影响；不重复已有效的需求评审。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-REQ-R1-F1 保持 CLOSED。需求 blob 与 Round 2 完全相同。安全交接、未安装进程隔离、Debug App 独立等目标没有被修复变更；当前架构和原型是否满足这些目标，继续由对应文档评审判断。

#### 📝 总结

本次“所有文档”复评保留需求 PASS，复用 Round 2 的同一 ContentState。Neo4j 精确态查询仍为 VERIFIED；复用 evidence `installed-state-ownership:req:r2-20261105-G-VuYw:review` 和 `...:l0`，missing_criteria、invalidated_evidence、unresolved_candidate_impacts 均为空。没有伪造新的需求检查或覆盖旧证据。

Repair 2 的物理路径解析、共享/排他锁与原型标本没有改变需求本身；架构仍需处理其所有权交接反例。此通过边界不批准其余设计、不启动实现，不关闭来源缺陷。Beads 保持 awaiting_commit。

Task checkpoint：ad-installed-state-ownership-doc-req；content_state `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`；gate VERIFIED。
提交建议：经用户明确授权后交付本需求及评审记录、必要状态片段；可等待剩余设计修复后按授权成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需要显式推送授权，并完成对应提交对象、签名及范围检查。本轮未提交或推送。

## Round 4 — 2026-11-05

### 📋 需求复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；复核 claude-code 的 Repair 3，未委派，未修复设计、产品代码或标本。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；fingerprint `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。
- WorkUnit: `installed-state-ownership:requirements.md`；Beads: `ad-installed-state-ownership-doc-req`。
- Target ContentState: `installed-state-ownership:req:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：VERIFIED
- Method / Scope: 比对未变化的需求 blob，并评估 Repair 3 的升级续登、替换和 App 资格是否改变已批准的需求边界。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-REQ-R1-F1 保持 CLOSED，需求 blob 仍与 Round 2 相同。用户选择的受锁保护自动续登，是实现“安装拥有状态、升级后继续使用”的架构手段；其触发和互斥缺陷由架构记录判断，不把清晰的需求目标改成失败。

#### 📝 总结

复用同一个 Round 2 ContentState 和 review/L0 证据。当前精确态查询仍为 VERIFIED，missing_criteria、invalidated_evidence、unresolved_candidate_impacts 为空。本轮不新增或重写需求证据，不把未完成的架构设计当作需求批准的一部分。Beads 保持 awaiting_commit，来源缺陷仍开放。

Task checkpoint：ad-installed-state-ownership-doc-req；content_state `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`；gate VERIFIED。
提交建议：经明确授权交付本需求、其评审记录及必要状态片段；也可待其余设计修复后成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式推送授权及对应提交对象、签名、范围检查。未执行提交或推送。

## Round 5 — 2026-11-05

### 📋 需求复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；复核 claude-code Repair 4，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；fingerprint `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。
- WorkUnit: `installed-state-ownership:requirements.md`；Beads: `ad-installed-state-ownership-doc-req`。
- Target ContentState: `installed-state-ownership:req:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：VERIFIED
- Method / Scope: 需求 blob 未变化，核对 Repair 4 对既定独占状态及升级使用边界的影响。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-REQ-R1-F1 保持 CLOSED。需求仍要求当前安装独占真实状态和安全交接；新预留机制是否满足它，是架构的验收对象，没有改变需求本身。

#### 📝 总结

Evidence: 实测 blob 与 Round 2 相同，复用其同一 ContentState。当前 gate-status 返回 VERIFIED，review/L0 证据有效，missing、invalidated、unresolved 均为空。没有重跑未变化需求的语义检查或新增证据。此 PASS 不批准尚未通过的预留事务设计，也不启动实现；来源缺陷保持开放。

Task checkpoint：ad-installed-state-ownership-doc-req；content_state `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`；gate VERIFIED。
提交建议：经明确授权交付需求、该评审记录和必要状态片段，或待其他设计修复后成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式授权及对应提交对象、签名和范围检查。本轮未提交或推送。

## Round 6 — 2026-11-05

### 📋 需求复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；独立复核 claude-code Repair 5，未委派，未修复设计、产品代码或测试。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`；fingerprint `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`。
- WorkUnit: `installed-state-ownership:requirements.md`；Beads: `ad-installed-state-ownership-doc-req`。
- Target ContentState: `installed-state-ownership:req:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：VERIFIED
- Method / Scope: 核对未变的需求内容和本轮安装协议的影响；不重复已有效的需求评审。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-REQ-R1-F1 保持 CLOSED。需求仍清楚要求安装拥有状态、安全交接及 Debug 独立；更换内部安装协议没有修改这些目标。

#### 📝 总结

Evidence: 实测 blob 与 Round 2 相同。当前精确态查询复用 Round 2 的 review/L0 证据并返回 VERIFIED；missing、invalidated、unresolved 均为空。此通过不批准尚未闭合的 cask 卸载链，也不启动实现或关闭来源缺陷。

Task checkpoint：ad-installed-state-ownership-doc-req；content_state `23677235a0c5ae2f354d8a73140f84315e976eaad4569bce55e0dd955a755656`；gate VERIFIED。
提交建议：经明确授权交付需求、评审记录及必要状态片段，或待其他设计完成后成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式授权及对应提交对象、签名和范围检查。本轮未提交或推送。
