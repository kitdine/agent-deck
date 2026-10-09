---
status: active
topic: v0-7-0-contract
subject: tasks.md
---

## 当前修复移交 — 2026-10-08

### V070-PR-F1 — P2 — 已修复，待独立复评

- 来源：用户于 2026-10-08 明确要求移除提前 contract 文档 PR/合并，
  contract 整体 PR/合并须等待 assemble 和版本契约收尾任务通过评审及证据门禁。
- 确认：提交 `a170d5547a650fb7fd05231cfbd01fd15ca40906` 中
  `tasks.md:213`–`216` 明确允许文档通过后提前 PR 到 main；
  `assemble` 的 reviewed/delivered membership 与收尾的 authorized delivery
  还可能把提前合并误作前置条件。该安排不符合用户明确的交付边界。
- 风险：只完成文档评审即合并整个 contract 分支，绕过版本集成与契约收尾边界。
- 修复：文档可授权提交/推送到主题分支；整体 PR/合并等待两个实施任务的
  独立评审和 VERIFIED 门禁。同步去除 assemble/收尾对早期 contract 合并的
  隐含依赖；批次契约记录留在 contract 工作区，成员 PR 仍按 assemble 集成 main。
- 状态：修复候选已完成，Review 清空，等待独立复评；本阶段不签发新的 PASS。
  下方 Round 1、原始证据及授权提交/推送均为历史事实，不代表新候选已获批准。
- 范围：仅 `tasks.md` 与本评审记录；无产品代码、测试、配置或全局规范修改。
- 验证：`python3 scripts/check-docs.py`、`make check-whitespace`、
  `git diff --check` 均通过；仅两份主题文档改变。
  候选 HEAD 为 `a170d5547a650fb7fd05231cfbd01fd15ca40906`，document blob 为
  `7f7fb77d408b400972af61f08fa16721bf689284`，按既有 recipe 计算 fingerprint
  `f425236f4dd5af001b0751619c2297fb875e22834820ceb6cd3ddf1fc039658c`。
- CEv1 修复候选：`urn:cev1:github.com/kitdine/agent-deck:content-state:f425236f4dd5af001b0751619c2297fb875e22834820ceb6cd3ddf1fc039658c`。
  固定模板查询结果 NOT_VERIFIED；原证据 target_matches=false，未复用旧 PASS。
  缺少 scope、document_set、l0 的新目标证据；本节保存了可供复评核验的本地 L0
  结果，独立语义复评仍待执行。invalidated/unresolved 列表为空，不代表语义批准。
  允许范围仅本 finding 的两份文档与必要协调/证据；不授权 Git 交付。

## Round 1 — 2026-10-08

## 📋 版本契约任务计划评审

📊 综合评分：9/10

✅ 评审结论：PASS

- Reviewer: Codex，独立评审角色；本会话未参与候选设计，无子代理。
- Method: 冷上下文、单评审者，核对当前权威文档、实时任务记录、Git
  身份和必要源码；静态文档检查。历史诊断仅作为已明确标注的历史输入。
- Scope: `docs/topics/v0-7-0-contract/tasks.md` 的版本成员、文档集、依赖、
  验收、增量集成及版本闭环；不对尚未设计的功能实现作通过声明。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；
  输入 document blob `9e9f3f919589ed5a3da2362c77c4092d6b08fd3b`；
  输入 fingerprint `2a604852927229863aa22e2a827f10d5d3816f025a2662d02b991fdf90a102d6`。
  评审同步仅勾选 Review、澄清历史 Design handoff 并添加本记录入口；最终
  document blob 和 ContentState 在下方证据中记录，未改变成员或任务语义。

### 🔴 严重问题 — 必须修复

无。没有未关闭的本轮 finding。

### 🟡 改进建议 — 建议处理

无。性能预算、credits 来源、模型别名和辅助分类仍须专题设计，但文档已
明确其责任方、依赖及进入实现的条件，不将这些未执行工作误记为计划缺陷。

### 🟢 优点

- 四个成员逐项承接当前 Roadmap 和实时 Beads 载体，价格四项最低需求均保留；
  未固化旧费率、模型映射、工作包数量或未证明的长扫描根因。
- 默认辅助隔离与完整用量保留并存，unknown、覆盖规则和跨表面契约由专题裁定；
  canonical session 修复未被重新混入功能范围。
- 当前消息兼容、历史回填、扫描预算和组合验收有明确依赖；回填保留事件与
  token 身份，要求幂等与未受影响来源跳过。
- 增量批次直接集成 main，未完成成员不被静默剔除；文档交付、产品交付、
  集成评审、版本闭环和发布验收分别保留必要边界。

### 📝 总结

任务计划 PASS，文档集只需要 `tasks.md`，与版本契约主题的职责一致。
`assemble` 和 `v0-7-0-contract` 两个实施任务仍未开始；文档通过不代表
整个版本完成或已具备集成条件。本轮未修改产品代码、测试、配置或真实用户数据，
未执行提交、推送、PR、合并或发布。

#### Evidence

1. 当前工作区绑定为 `v0-7-0-contract`，分支 `feature/v0-7-0-contract`，
   common Git directory 与主仓库一致；HEAD 与设计移交一致。`git hash-object`
   证明输入文档与设计方移交 blob 相同。`git rev-parse 'v0.6.5^{commit}'`
   返回 `819bd4365aef59242a6a38034c379931548d4671`，与兼容基线一致。
2. 核对 [Roadmap](../../../roadmap.md#v070--scan-performance-pricing-redesign-and-work-sessions-proposed)、
   [文档工作流](../../../documentation-workflow.md#topic-structure)、
   [分支与集成规则](../../../../.agent-instructions/branching.md#merging-is-a-contract-topic-action)
   及 [版本语义](../../../specs/cli-design.md#version-number-semantics)。
   v0.7.0 的新默认值、价格语义和可能的迁移符合 MINOR 版本边界；未继承 v0.6.5
   的狭窄例外为通用豁免。当前 Price Catalog 仍使用 LiteLLM，models.dev 是待设计变更。
3. 实时读取 `ad-v070-iteration` 及四个选定 carrier；扫描/价格/辅助分类仍为
   deferred，工作信号 Bug 为 open，Lane A 仍待确认。已交付 canonical 元数据
   修复的相关载体为 closed；本轮不变更这些记录。
4. CodeGraph 的一次定位未返回目标解析分支，转为一次精确搜索和聚焦读取。
   `internal/usage/usage.go:1520` 的 `parse` 在 `event_msg/user_message`
   分支调用 `state.note`，随后拒绝非 `event_msg`，支持计划中缺口的源码前提。
   未将此源码检查当作运行时回归证明；历史一事件/一编辑诊断仍为原 Bug 的
   历史观察，本轮不依赖临时诊断产物可访问。
5. `python3 scripts/check-docs.py` 在输入候选通过，包含受影响文档集、链接及
   L0 检查；这是项目提供的可否证文档集检查，不是语义 PASS。设计移交同一
   blob 的 `make check-whitespace` 和 `git diff --check` 为 PASS，可复用。
   最终同步后的检查结果和内容身份见下方 Completion evidence。

#### Completion evidence

Completion gate: VERIFIED

文档 WorkUnit：`urn:cev1:github.com/kitdine/agent-deck:work-unit:v0-7-0-contract:tasks.md`。
三项 required criterion 为 scope、document_set、l0；依据项目评审契约、
文档集要求和 L0 矩阵建立。初次固定模板查询为 NOT_VERIFIED，三项均缺证据，
无 invalidated evidence 或 unresolved candidate impact。

最终 document blob：`2a2fbecfa038710e3fc27732d51211a0e060553b`。
最终 fingerprint：`8cee496e3cfaaeec41ce41849b283bf9c2da07e991f12c2aad68a0877d1a9d27`，
按 `sha256(head=<HEAD>;document=<Git-blob-ID>)` 计算。
ContentState：`urn:cev1:github.com/kitdine/agent-deck:content-state:8cee496e3cfaaeec41ce41849b283bf9c2da07e991f12c2aad68a0877d1a9d27`。
最终同步后的 `python3 scripts/check-docs.py`、`make check-whitespace`、
`git diff --check` 均通过；固定 `gate-status.cypher` 返回 VERIFIED，
三个 criterion 各有一条 exact-target pass observation，missing、invalidated、
unresolved 列表均为空。节点写入的 MCP 返回 mutation counters，读回确认
预期节点数；关系预检全部 ok，门禁回读确认三项 requires 和六项证据关系有效。
仅回写本记录的门禁事实不改变被评审的 `tasks.md` 内容身份。

Task checkpoint：`ad-v0-7-0-contract-doc-tasks-design`；上述最终 fingerprint；VERIFIED。

提交建议：显式授权后，将本主题的 `tasks.md` 与 `reviews/tasks.md` 作为一项
文档任务提交，保留设计和评审贡献归因；不包含产品变更或其他主题。

推送建议：显式授权后推送 `feature/v0-7-0-contract`，交付目标为 main；
提交对象、签名、远端及必要 CI/PR 条件在交付阶段核验。

下一实施任务为 `assemble`，当前尚缺契约文档授权交付及至少一个完整成员的
集成就绪结果。两个实施任务均未完成，不声明版本或 containing-unit gate 通过。

## Round 2 — 2026-10-09

## 📋 交付边界修复复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex，独立复评角色；本会话未参与候选设计或修复，无子代理。
- Method: 冷上下文单评审者，逐项核对修复移交、实际差异、项目权威和实时
  Beads 交接；仅同步评审记录和文档状态，不修复产品或计划语义。
- Scope: V070-PR-F1，文档交付、成员集成、整体 PR/合并及两个实施任务的依赖。
- Reviewed state: HEAD `a170d5547a650fb7fd05231cfbd01fd15ca40906`；输入
  document blob `7f7fb77d408b400972af61f08fa16721bf689284`，fingerprint
  `f425236f4dd5af001b0751619c2297fb875e22834820ceb6cd3ddf1fc039658c`，
  与修复移交一致。最终状态仅同步 Documents Review 和本轮状态入口。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 优点

- V070-PR-F1 — CLOSED：`tasks.md` 的 Entry conditions 明确文档只可授权
  提交/推送主题分支，整体 PR/合并等待 `assemble` 和 `v0-7-0-contract`
  两个实施任务独立评审与 VERIFIED 门禁，并保留单独的交付授权。
- `assemble` 前置条件要求主题分支上的已评审契约提交，收尾要求成员集成
  main 和主题分支上的聚合记录提交；两处均排除提前 contract 合并依赖。
- 成员 PR 直接进入 main，批次与集成评审记录留在 contract 工作区；不以
  成员 PR 偷渡提前 contract 交付。历史 Round 1 和已交付提交保持原样。

### 📝 总结

唯一历史 finding V070-PR-F1 已关闭；无仍开放、回归或新增阻断问题。
版本成员、专题边界及产品验收要求未变。结论只覆盖本次文档修复，两个实施
任务尚未开始，不证明功能、集成或版本完成。未执行 Git 交付。

#### Evidence

- 实际文档 blob 与修复移交一致；差异仅涉及交付边界、相关前置条件和状态。
- 当前 Beads 任务为 `ad-v0-7-0-contract-doc-tasks-design`，修复评论
  `bfbdb11b-1a1e-5bc9-8d36-d92741de325f` 已释放所有权；复评领取后为
  `in_review/codex`。文档任务与两个实施任务不同，不跨越整个主题门禁。
- 初次固定模板查询为 NOT_VERIFIED：scope、document_set、l0 缺少新目标
  证据；旧证据 target_matches=false，invalidated/unresolved 均为空。
- 最终状态的 L0 结果与 CEv1 身份在下方完成证据记录；Review 状态同步不改变
  被评审的交付政策。原修复检查不冒充最终文档的验证结果。

#### Completion evidence

Completion gate: VERIFIED

最终 document blob：`6453f7177f9d6fa1be09978f509bf75e8133e6ed`。
最终 fingerprint：`5607b238d4a89ac5ef49fe61d36ec71b892615e8d3b6f8cd76dbd658a36f52a4`，
采用既有 `sha256(head=<HEAD>;document=<Git-blob-ID>)` recipe。
ContentState：`urn:cev1:github.com/kitdine/agent-deck:content-state:5607b238d4a89ac5ef49fe61d36ec71b892615e8d3b6f8cd76dbd658a36f52a4`。
最终文档状态的 `python3 scripts/check-docs.py`、`make check-whitespace` 和
`git diff --check` 全部通过。四个新节点写入并逐项读回，六条关系预检均为 ok，
写入计数与提交数量一致；固定 `gate-status.cypher` 回读 VERIFIED，三个 required
criterion 均有 exact-target pass，missing/invalidated/unresolved 列表为空。
历史交付 WorkUnit 元数据保留原提交事实，本轮不声称新候选已交付；图中无直接
父子边，文档任务也不等于尚未开始的两个实施任务，不查询或声明整个版本完成。

Task checkpoint：`ad-v0-7-0-contract-doc-tasks-design`；上述最终 fingerprint；VERIFIED。

提交建议：显式授权后，仅提交本主题 `tasks.md` 和 `reviews/tasks.md` 的修复及
复评记录，保留应有贡献归因。

推送建议：显式授权并完成提交对象、签名和远端检查后，推送
`feature/v0-7-0-contract`；此边界不创建整体 PR 或合并 main。

下一实施任务为 `assemble`；须先授权交付本次契约文档，并具备至少一个完整
成员的集成就绪结果。整体 PR/合并仍等待两个实施任务通过独立评审及适用门禁。
