---
status: active
topic: subscription-quota
subject: ux/widget-quota.md
---

# Widget Quota Review

## Round 1 — 2026-09-09

## 📋 小组件额度 UX 评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-WQ-R1-F1 — 中：Claude 归属说明没有落实，small 方案削弱已通过需求

- 位置：`ux/widget-quota.md:71-85`；`prototype/src/Widgets.jsx:520-545,571-614` 及 medium/large 分支。
- 行为风险：三个尺寸显示 Claude 数值及年龄，却没有说明这些数字无法确认属于当前账号；用户可能据旧账号额度判断可用容量。新鲜度不能证明账号归属。
- 需求依据：`requirements.md:364-371` Acceptance 11 要求在来源及新鲜度旁说明归属无法确认；Open question 6（436-441）要求尺寸放不下时决定替代显示，而未授权隐藏说明。
- 实测：本 worktree 原型 `http://127.0.0.1:4187/?surface=widgets&quota=codexPlus&widgetClient=claude&lang=en&measure=1`。Small、Medium 显示 Claude 22% 等数字，Large 显示 Claude 两窗口。三个 article 的可见文本均无 attribution statement；辅助名称只有 `Quota · Small/Medium/Large`，title 均为 null。源码没有消费 attribution_confirmed 来显示说明。
- 截图：[Large 的 Claude 数值缺少归属说明](evidence/widget-r1/claude-attribution-missing.png)。DOM 检查覆盖三个尺寸；截图支持 Large 的可见反例。量具虽显示 NO OVERFLOW，却不能证明必需内容存在。
- 文档承认新增状态尚未由标本落实；small 的 accessible name/title only 方案即使实现，也不满足可见说明要求。单端配置选择不是接受账号风险的授权。
- 💡 有界修复：在三个尺寸落实归属说明；small 使用可见说明布局，或按需求改为不展示不明归属数字的替代状态，不能只塞进 title/辅助名称。同步文档及权威原型，补中英文标本、内容存在性与固定尺寸验证；辅助名称可额外保留。若要降低需求边界，应先有明确产品决定。
- 状态：OPEN。

### 🟡 建议改进 — 推荐

无额外发现。

### 🟢 优点

尺寸深度、固定客户端选择、不偷换客户端及 reset allowance 不进入小组件的边界明确。

### 📝 总结

- Reviewer：Codex；Method：单主会话需求/UX 对照、独立 Chromium 会话、渲染截图、DOM 与局部源码核对；非冷上下文独立评审，未委派。
- Scope：Widget 额度 UX 与标本；决定性反例后停止扩大验证，未修改产品、原型、测试或配置。
- Reviewed state：HEAD `15779977af20c4c70dfbe32df1a32ea9f84ddfdf`；document blob `ab1a164e56dbd2cf741bb8886b8d97755037cdda`。
- Prototype：[输入清单](evidence/widget-r1/prototype-manifest.sha256)；沿用菜单栏记录的方法，清单 SHA-256 `05de9a0de4a5bcc91c95b449e8a893473661da70f1c9b229989a09e7b2be40ee`。
- Content fingerprint：`dcaa65102989ba14dd51db585bb5903b7cf777a5ed4cb59eae5630642512104f`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-ux-widget-quota-design`，由实时 Beads 与 Documents 矩阵确认，沿用六文档设计交接。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:ux/widget-quota.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:widget:1577997:ab1a164:05de9a0`。
- Evidence：上述三尺寸 DOM 与渲染反例。历史 28/28 与 contract ALL PASS 未当作本轮全量验证；未验原生 WidgetKit、VoiceOver 或全状态/外观矩阵。
- L0：make check-whitespace、git diff --check 和原型清单逐项检查通过；归档截图与清单链接目标存在。
- 完成门禁：FAILED；固定 gate-status.cypher 最终确认 ux/review 反证适用，L0 pass 适用，无失效或未决影响。五个基础节点、三个 requires 关系、三个证据节点和六个证据关系数量匹配，两批关系 preflight 全部 ok，最终 gate 读回确认。
- 一项发现未关闭，Review 保持未勾选，不建议提交或推送，不跨越主题完成边界。

### 下一步指令

修复：subscription-quota / reviews/ux-widget-quota.md / SQ-WQ-R1-F1

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 2 — 2026-09-09

## 📋 小组件额度 UX 复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

SQ-WQ-R1-F1 — CLOSED：文档撤回 small 的隐藏说明方案，三尺寸均显示 `账号未确认 / account unconfirmed`。Small 与 Medium 位于数值/窗口下方、观察时间上方；Large 位于 Claude 块头部，Codex 不带该说明。中英文实际渲染与 DOM 尺寸核对均确认文本存在且未截断，符合需求要求的可见限定。

### 📝 总结

- Reviewer：Codex；Method：单主会话逐项复评、独立 Chromium 渲染、DOM 几何与尺寸合同、相关断言源码核对；非冷上下文独立评审，未委派。
- Scope：SQ-WQ-R1-F1 及直接相关三尺寸布局/配置回归；未修改产品、原型、测试或配置。
- Reviewed state：HEAD `15779977af20c4c70dfbe32df1a32ea9f84ddfdf`；document blob `02b08fde52094f3898a9830bb286c0a8616fd74b`。
- Prototype：[输入清单](evidence/widget-r2/prototype-manifest.sha256)；清单 SHA-256 `c52cd29b92efe21a783b7f0cd932afb5617ad8dc26646e2813d21164623cdef6`，沿用 Round 1 方法。
- Content fingerprint：`1e25cd9b70dee8707cf43f03f51a602ea7e6660e7fca25ae82974833b060e248`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-ux-widget-quota-design`。修复交接 `01a086c8-9900-7bc3-9af8-d7b7bfc1b28e` 为接手依据，其自述经本轮独立工具结果核对。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:ux/widget-quota.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:widget:1577997:02b08fd:c52cd29`。
- Evidence：本 worktree Vite；`surface=widgets&quota=codexPlus&widgetClient=claude&lang=en|zh&measure=1`，三个 `.w-attribution` 分别显示对应语言短句，非零尺寸且 scrollWidth 不超过 clientWidth。[英文截图](evidence/widget-r2/english.png)、[中文截图](evidence/widget-r2/chinese.png)支持 small/medium 的实际显示；Large 文案位置与裁切由 DOM 和合同核对。
- 合同验证：`quota=codexPlus&widgetClient=claude&lang=en&contract=1` 与 `quota=codexPlus&widgetClient=codex&lang=zh&contract=1` 均 ALL PASS，覆盖正向出现、Codex 负向不出现、卡片尺寸及窗口数量。源码断言检查实际文本与非零边界，而不是 title 或辅助名称；不重做作者的临时源码回退。
- L0：make check-whitespace、git diff --check、原型清单逐项校验通过；归档截图及清单链接存在。
- 完成门禁：VERIFIED；固定 gate-status.cypher 最终确认三项 required criterion 均满足，无缺失、失效或未决影响。新增一个 ContentState、三个 Evidence 和六个关系数量匹配，关系 preflight 全部 ok；最终 gate 读回确认目标关系。旧态观察未被重标。
- 限制：未重跑全状态/外观矩阵、原生 WidgetKit、VoiceOver 或系统刷新验收；本轮证据仅覆盖上述范围。全部本记录发现已关闭，包含主题仍有文档与实现工作。

Task checkpoint：ad-sq-doc-ux-widget-quota-design；content_state `1e25cd9b70dee8707cf43f03f51a602ea7e6660e7fca25ae82974833b060e248`；gate VERIFIED。
提交建议：获授权后提交 Widget UX、相关原型/合同断言、评审证据及对应任务状态；共享文件按任务范围拆分。
推送建议：获授权并检查提交内容、贡献者及 SSH 签名后推送 feature/subscription-quota；远端/upstream 待交付时确认。

### 下一步指令

评审：subscription-quota / ux/settings-quota.md

WORKFLOW_WORKSPACE: agent-deck.subscription-quota
