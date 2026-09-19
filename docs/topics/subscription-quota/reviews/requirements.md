---
status: active
topic: subscription-quota
subject: requirements.md
---

# Requirements Review

## Round 1 — 2026-09-09

## 📋 需求边界评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-REQ-R1-F1 — 中：主动查询的启用边界没有进入需求与验收

- 位置：`requirements.md:192-197,273-285,302-305`。
- 行为风险：实现可以在用户仅刷新快照、尚未启用额度读取时启动已认证客户端查询，并仍满足现有十二条验收。用户关闭读取后，后台调度、状态栏采集和已有额度展示应如何处理，也没有需求约束。
- 证据：上游 Beads `ad-subscription-quota` 明确要求“App 执行用户启用的网络查询与提醒”。本文件仅明确提醒默认关闭和状态栏注册需同意；探测条件只写 `official`，手动刷新要求随快照查询。`ux/settings-quota.md:13-16,34-49` 已另行规定 quota reading opt-in、三个开关默认关闭及 `quotaProbe` 父开关，证明下游依赖了未写入需求的产品决定。`architecture.md` C9 又将手动查询写为 unconditionally。
- 💡 有界修复：在 Goals 与 Acceptance boundary 明确额度读取默认值、启用条件及关闭行为；区分手动刷新绕过轮询间隔与绕过用户启用开关，规定已有缓存、状态栏采集及提醒在关闭时的处置。核对下游相关条款，仅同步受此边界影响的内容。
- 状态：OPEN。

#### SQ-REQ-R1-F2 — 中：Claude 账号隔离缺口未在需求边界中处置

- 位置：`requirements.md:136-148,188-190,299-301,341-368`。
- 行为风险：用户从 Claude 账号 A 切换到 B 后，旧账号额度仍可能作为当前客户端额度展示，并参与提醒；新鲜度只能说明时间，不能证明账号归属。
- 证据：上游 `docs/roadmap.md` 的 Subscription design 段及 Beads `ad-subscription-quota` 将账号隔离列为整体范围。本文件的隔离验收只覆盖 Codex `accountId`，能力矩阵和 Open questions 未列 Claude 账号身份缺失及处置。现有下游 `architecture.md:266-270` 明确写 Claude 两条通路均无账号标识，因此按 client 存储，并在账号切换后继续显示旧数值，直到下次成功探测。这是对上游边界的实质降级，尚未由本需求给出接受条件。
- 💡 有界修复：将 Claude 账号身份不可得列入能力矩阵与具名缺口，明确跨账号归属无法确认时展示、缓存和提醒的边界及可失败验收。若只能保留“切换后旧值继续显示”的降级，须记录该产品取舍的明确用户决定；不得由架构静默缩小需求。
- 状态：OPEN。

### 🟡 建议改进 — 推荐

无额外发现。

### 🟢 优点

明确拒绝读取客户端凭证和直接调用账号 API；官方 reset allowance、自然窗口重置和本地观察重置有独立语义；缺失字段不得伪装为零，散文解析失败也有明确结果。

### 📝 总结

- Reviewer：Codex。
- Method：单主会话文档边界评审；对照上游 roadmap、版本契约、实时 Beads 任务及下游相关条款，核对已引用的局部实现。未委派；非独立冷上下文评审。
- Scope：仅 `subscription-quota / requirements.md`；下游文档仅用于验证需求缺口，未对其签发评审结论。未评审或修改原型、产品代码、测试和配置。
- Reviewed state：HEAD `47370767d4f195dd6c23c663861a01cecc81f3ac`；document blob `e84d8a4b5e529d979f5c2c10622b06047c28b45e`。
- Content fingerprint：`c1fca21c2fbc61b3668e89bec863d7e8906cc984b2c3769fdcc2333996098df5`，SHA-256 of `head=<HEAD>;document=<blob>`。本轮不依赖渲染标本作结论，因此不纳入 prototype 身份。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`。
- Task：`ad-sq-doc-req-design`；交接依据为 `ad-sq-doc-tasks-design` 的 claude-code 六文档 Draft handoff（2026-09-09T06:29:15Z）。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:requirements.md`。
- Target ContentState：`urn:agent-deck:content-state:subscription-quota:requirements.md:4737076:e84d8a4`。
- Evidence：实时任务及上游/下游条款交叉核对已给出两项决定性反例。`provider.Service.Current` 源码证实读取选择而不解密凭证；session 扫描路径、usagehook 顶层编辑函数和公开价格 URL 与文档列举相符。
- 本轮未重新执行真实 Codex/Claude 查询，文档内 2026-09-08 的厂商字段实测不是本轮复验结果。发现阻断问题后停止扩大验证；不声称全部来源可行性或原型运行验证通过。
- L0：`make check-whitespace`、`git diff --check` 均退出 0；需求和新增状态指针的本地 Markdown 链接目标存在。项目没有需求语义专用检查器；`check-topic-docs.sh` 的强制入口是 tasks.md 文档集评审，本轮不据此宣称整个文档集完整。
- 完成门禁：FAILED；固定 `gate-status.cypher` 最终查询确认 boundary、review 两项为适用 fail，L0 为适用 pass；均绑定本轮 Target ContentState，无 malformed、失效或未决影响。五个基础节点、三个 requires 关系、三个证据节点和六个证据关系的写入数量与提交量一致，关系 preflight 全部 ok；最终 gate 读回确认所有三项及其观测关系。
- 两项需求决策缺口未关闭，Review 保持未勾选；本轮不产生提交或推送建议，也不关闭包含主题。

### 下一步指令

修复：subscription-quota / reviews/requirements.md / SQ-REQ-R1-F1、SQ-REQ-R1-F2

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 2 — 2026-09-09

## 📋 需求边界复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SQ-REQ-R1-F1 — CLOSED：Goals 明确读取默认关闭、所有触发均受开关限制、保留缓存但不展示；Acceptance 1、2、3、12、13、14 覆盖首次使用、关闭后重启读取、provider 门禁、手动刷新只绕过间隔、提醒及状态栏同意。`architecture.md` C1/C9 与 `ux/settings-quota.md` 的父开关说明已同步。读取关闭时“reads no quota at all”覆盖被动采集；具体已注册包装器的停用实现仍由架构与实现验收落实。
- SQ-REQ-R1-F2 — CLOSED：能力矩阵新增 Account identity；Named gap 明确 Claude 无账号标识、旧账号数值可能保留，以及新鲜度不能证明归属。需求记录 2026-09-09 用户接受降级并要求就地说明；Beads 修复交接评论 `01a08609-9b55-74ab-a250-8c55106423e6` 同样记录该决定。Acceptance 11 明确展示须声明归属无法确认，界面、提醒和导出不得暗示归属；Open questions 将具体摆放与放不下时的替代显示交给对应 UX 文档。此处依据持久化决策记录关闭，不声称本会话重新取得用户决定。

### 📝 总结

- Reviewer：Codex；Method：单主会话逐项复评及受影响条款交叉核对，非冷上下文独立评审，未委派。
- Scope：`requirements.md` 的两项发现及直接相关条款；未签发 architecture、UX、tasks 或原型的评审结论。
- Reviewed state：HEAD `47370767d4f195dd6c23c663861a01cecc81f3ac`；document blob `22274f1ca492aa03ba5a4ee1b015620555dc2205`。
- Content fingerprint：`4756d69124c68b0ee51c34f5bb8284a81ac6f065f03a5d8edd11c518ad8ecfaf`，沿用 SHA-256 of `head=<HEAD>;document=<blob>`；不依赖渲染标本作本轮结论。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-req-design`。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:requirements.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:requirements.md:4737076:22274f1`。
- Evidence：对照 Round 1 发现、当前需求全文、architecture C1/C3/C9 与设置父开关说明、Beads 修复交接。沿用上轮未变的局部实现核对；未重新探测厂商接口，也未运行产品或原型验收。
- L0：`make check-whitespace`、`git diff --check` 通过；需求及状态指针的本地 Markdown 目标存在。没有需求语义专用检查器；本轮不对 tasks.md 的完整文档集签发结论。
- 完成门禁：VERIFIED；固定 `gate-status.cypher` 最终查询的三项 required criterion 均由本轮适用 pass 证据满足，无缺失、失效、未决影响或 malformed。旧 Round 1 观察正确保持 target_matches=false。新增一个 ContentState、三个 Evidence 和六个关系；写入数量匹配，关系 preflight 全部 ok，最终 gate 读回确认关系与目标一致。
- 残余范围：两个新增界面状态的标本与摆放由各自设计任务完成；这不改变本轮已明确的产品边界，也不构成 UX 通过。包含主题仍有文档及全部实现任务，不跨越 Topic 完成门禁。

Task checkpoint：ad-sq-doc-req-design；content_state `4756d69124c68b0ee51c34f5bb8284a81ac6f065f03a5d8edd11c518ad8ecfaf`；gate VERIFIED。
提交建议：获授权后提交 requirements.md、本评审记录和 tasks.md 对应状态；下游修复与原型按各自评审边界处理。
推送建议：获明确授权并完成提交内容、署名和 SSH 签名检查后推送 feature/subscription-quota；当前分支未配置 upstream，远端目标待交付时确认。

### 下一步指令

设计：subscription-quota / ux/menubar-quota.md

WORKFLOW_WORKSPACE: agent-deck.subscription-quota
