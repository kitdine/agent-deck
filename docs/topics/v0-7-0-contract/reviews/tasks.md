---
status: active
topic: v0-7-0-contract
subject: tasks.md
---

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
