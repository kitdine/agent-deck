---
status: active
topic: subscription-quota
subject: ux/menubar-quota.md
---

# Menu-bar Quota Review

## Round 1 — 2026-09-09

## 📋 菜单栏额度 UX 评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-MB-R1-F1 — 中：鼠标无法移入重置次数明细，弹层随即消失

- 位置：`ux/menubar-quota.md:128-147`；对应标本 `prototype/src/Popover.jsx:537-538,607-610`。
- 行为风险：用户从“官方重置次数”触发行移向旁边的明细以阅读额度状态和到期时间时，明细关闭。悬停弹层不是可稳定进入的阅读区域；低视力用户将指针移入内容附近阅读同样受影响。键盘打开功能不能替代鼠标路径的可用性。
- 证据：在本 worktree 启动的 Vite 原型，URL `http://127.0.0.1:4187/?tab=quota&quota=codexPlus&lang=en&width=280&measure=1`。依次执行 `hover .quota-allowance.expandable`、检查 `.quota-flyout`、`hover .quota-flyout`、`get count .quota-flyout`。首次快照为带三条 credit 的 Official resets dialog；移入后 count 为 **0**。量具同时仍显示 NO OVERFLOW，说明布局检查不能证明此交互成立。
- 源码核对：触发行 `onMouseLeave={hideCredits}` 立即调用 `openCredits(null, null)`；独立渲染的 `CreditsFlyout` 不在触发区域内，也没有接续悬停的机制。文档将“moving away”笼统作为关闭条件，未区分移入明细与离开整个交互区域，因此这是设计及其权威标本的同一缺口。
- 对照截图：[悬停触发行，明细存在](evidence/menubar-r1/hover-trigger.png)；[鼠标移入明细后，明细消失](evidence/menubar-r1/hover-detail.png)。两图使用相同内容与 URL；截图中的舞台控制条属于原型，非产品界面。
- 💡 有界修复：将触发行、通往弹层的间隙和弹层作为连续可进入的阅读区域；移入明细应保持打开，离开整个区域或按 Escape 再关闭，并保留键盘可进入/退出路径。同步该段文档和原型，增加能捕获“触发 → 移入明细 → 继续阅读 → 离开”的交互回归验证。不得仅延长一个仍会在阅读期间关闭的定时器。
- 状态：OPEN。

### 🟡 建议改进 — 推荐

无额外发现。

### 🟢 优点

新增 reading-off 状态有独立文案和设置指引；Claude 账号归属说明被定义为独立一行，避免挤掉来源与观察时间。三个 reset 概念继续保持独立层级；实际窄界面标本中能看到归属说明。

### 📝 总结

- Reviewer：Codex；Method：单主会话文档评审、agent-browser 独立 Chromium 会话、实际鼠标交互、截图与局部源码对照；非冷上下文独立评审，未委派。
- Scope：菜单栏额度 UX 及其权威原型。未评审 Widget/Settings、未修复产品、测试或原型。发现决定性反例后停止扩大验证。
- Reviewed state：HEAD `61dba47e5efbd646e4d6bf6e3edf9dc9da8d3b38`；document blob `3d76be63b53bb95b8b3b0250cbca8c7f79f3f435`。
- Prototype：[完整输入清单](evidence/menubar-r1/prototype-manifest.sha256)，对 `git ls-files prototype` 与非忽略未跟踪文件的并集排序，逐文件 SHA-256；清单 SHA-256 `21a8cb96611367b25537e3d8cf3c086f1604ef7a1099da36775615028c1fd918`。
- Content fingerprint：`5fbce23ba187e3325182e6b29ca9c043fe1d7a1b30101228dc034f60f67cc02c`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Screenshot SHA-256：hover-trigger `424210822122ce8db1a665a63540698028542de27fbf390f7607984ec4788064`；hover-detail `f53b06bed23c37c2bbe6dcb86fd00b3643a2483029220d754e8cfae7a85ac9b3`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-ux-menubar-quota-design`。依据该任务 2026-09-09T13:02:16Z 的设计交接接手；其“需求尚未复评”备注已由 requirements Round 2 与提交 `61dba47` 所取代。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:ux/menubar-quota.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:menubar:61dba47:3d76be6:21a8cb9`。
- Evidence：上述真实浏览器反例。未将作者报告的 32/32 布局检查、10 项 quota probe 通过或其他页面失败当成本轮全量验证；没有声称原生 VoiceOver、Dynamic Type、全部状态及外观矩阵已验收。文档的设计问题不能由量具 NO OVERFLOW 覆盖。
- L0：`make check-whitespace`、`git diff --check` 和相关本地链接目标检查通过；没有 UX 语义专用检查器，项目原型量具仅证明其检查范围。
- 完成门禁：FAILED；固定 gate-status.cypher 最终查询确认 ux、review 的 fail 与 L0 的 pass 均适用于本目标，无 malformed、失效或未决影响。五个基础节点、三个 requires 关系、三个证据节点和六个证据关系写入数量匹配；两批关系 preflight 全部 ok，最终 gate 已读回确认。
- 本轮有一项中等发现未关闭，Review 保持未勾选；不跨越主题完成边界，不建议提交或推送。

### 下一步指令

修复：subscription-quota / reviews/ux-menubar-quota.md / SQ-MB-R1-F1

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 2 — 2026-09-09

## 📋 菜单栏额度 UX 复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-MB-R1-F1 — 中：部分修复，实际跨越路径仍有未覆盖间隙

- 处置：STILL OPEN；保留原发现 ID，不另造修复任务。
- 位置：`ux/menubar-quota.md:136-156`；`prototype/src/styles.css:2804-2831`；`prototype/src/Popover.jsx:1308-1315`。
- 行为风险：直接移入明细已经能保持打开，但缓慢移动或在触发行与弹层之间短暂停留仍使明细消失，用户仍可能无法到达阅读区域。文档明确承诺路径不依赖移动速度，当前标本未兑现。
- 证据：同一 Chromium 原型 URL `http://127.0.0.1:4187/?tab=quota&quota=codexPlus&lang=en&width=280&measure=1`。悬停触发行后，DOM 实测 trigger rect 为 x=526、right=754、y=269、bottom=308；flyout rect 为 x=789、right=1021、y=260、bottom=436。两者水平间距 **35 px**，不是文档假定的 10 px。伪元素只向左延伸 10 px，到约 x=779，触发行与面板外缘之间仍有约 **25 px** 空白。
- 决定性反例：悬停 `.quota-allowance.expandable` 后，真实鼠标移到 `(765,280)`，等待 500 ms，`.quota-flyout` count 为 **0**。该位置处于触发行和明细之间的直接水平通路。对照：直接 `hover .quota-flyout` 后等待同样 500 ms，count 为 **1**。
- 截图：[直接进入明细，保持打开](evidence/menubar-r2/inside-detail.png)；[停在实际间隙，明细消失](evidence/menubar-r2/in-gap.png)。
- 回归保护核对：`prototype/src/probe.js:230-235` 紧接着派发 trigger leave 与 flyout hover，未移动实际指针经过几何间隙，因此这些断言能通过却无法发现本反例。键盘断言还混用了 hover，本轮不据此声称纯键盘路径通过；发现决定性反例后未扩大验证。
- 💡 有界修复：按真实 trigger 与 flyout 几何位置覆盖整个连续通路，包括卡片/面板内边距以及面板外间隙，而不是只覆盖 CSS 中的 10 px。对右开、左开、覆盖式布局验证慢速经过及在通路中停留；在通路中停留应保持打开，离开完整区域才关闭。补入实际坐标移动的回归，不能仅把超时加长或连续派发两个合成事件。

### 🟡 建议改进 — 推荐

无额外发现。

### 🟢 优点

新增的进入弹层取消关闭定时器机制有效：直接进入后持续阅读 500 ms，弹层保持打开。文档也已明确“阅读期间不应超时关闭”，原发现得到部分修复。

### 📝 总结

- Reviewer：Codex；Method：单主会话逐项复评、真实指针几何复现、局部源码与回归断言核对；非冷上下文独立评审，未委派。
- Scope：仅 SQ-MB-R1-F1 修复及受影响悬停路径；未修改 UX、原型、测试或配置，未声称全部布局、键盘、原生辅助技术验收通过。
- Reviewed state：HEAD `61dba47e5efbd646e4d6bf6e3edf9dc9da8d3b38`；document blob `fcd24420f2f2d9d8f1b3f61b752aeebf3d72728b`。
- Prototype：[输入清单](evidence/menubar-r2/prototype-manifest.sha256)，沿用 Round 1 的清单生成方法；清单 SHA-256 `1c15b2789b092df8a69a6d8b76ebcfa1cda1b11e706ace00c120696c372ff14c`。
- Content fingerprint：`27454046662e07d210d38d7095ea5a2fc4534878f4dfd80a309949a4aa3aae1c`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-ux-menubar-quota-design`。修复交接评论 `01a0865f-02cc-776e-b4b5-78caada83928` 为接手依据，其完成声明经本轮反例核对不能作为关闭依据。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:ux/menubar-quota.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:menubar:61dba47:fcd2442:1c15b27`。
- Evidence：上述原始复现及对照截图；旧态证据不直接重标为新态，初查显示旧态观察 target_matches=false。
- L0：`make check-whitespace`、`git diff --check` 通过；归档截图及清单存在，原型输入清单逐项校验通过。
- 完成门禁：FAILED；固定 gate-status.cypher 确认本轮 ux、review 反证适用，L0 pass 适用，无失效或未决影响。一个新 ContentState、三个 Evidence 与六个关系数量匹配，关系 preflight 全部 ok；最终 gate 读回确认。
- SQ-MB-R1-F1 仍未关闭，Review 保持未勾选；包含主题未完成，不产生提交或推送建议。

### 下一步指令

修复：subscription-quota / reviews/ux-menubar-quota.md / SQ-MB-R1-F1

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 3 — 2026-09-09

## 📋 菜单栏额度 UX 复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

SQ-MB-R1-F1 — CLOSED：通路按触发行和明细的实时矩形计算，覆盖卡片内边距与面板外间隙，不再假设 10 px。真实鼠标验证右开 280 pt 和左开 420 pt 的实际 35 px 间隙；停在通路及进入明细阅读均保持打开，离开后关闭。纯键盘聚焦打开及 Escape 关闭亦通过。

### 📝 总结

- Reviewer：Codex；Method：单主会话逐项复评、Chromium 实际指针与键盘、局部源码及回归断言核对；非冷上下文独立评审，未委派。
- Scope：SQ-MB-R1-F1 修复及直接影响范围；本轮未修改原型、代码或测试。
- Reviewed state：HEAD `61dba47e5efbd646e4d6bf6e3edf9dc9da8d3b38`；document blob `f879b7e030db819de58c066940649fa2f5020ccd`。
- Prototype：[输入清单](evidence/menubar-r3/prototype-manifest.sha256)；清单 SHA-256 `05de9a0de4a5bcc91c95b449e8a893473661da70f1c9b229989a09e7b2be40ee`；沿用先前清单方法。
- Content fingerprint：`dc3c29d51ce7c7f73e0c8c2511737654fbedf3d0d55bf81d9739320801a1e179`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-ux-menubar-quota-design`。接手依据：修复交接 `01a08689-a712-754f-9f47-1e41a883055d`。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:ux/menubar-quota.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:menubar:61dba47:f879b7e:05de9a0`。
- Evidence：本 worktree Vite，`?tab=quota&quota=codexPlus&lang=en&width=280&measure=1`。右开 rect：trigger right=754，flyout left=789；定位完成并重新移入后，在 `(765,280)` 停留 500 ms，count=1；进入明细等 1000 ms，count=1；移出等 300 ms，count=0。focus trigger 后 count=1；Escape 后 count=0。
- 对称路径：`width=420&anchor=right`，trigger left=874，flyout right=839；在 `(855,280)` 等 1000 ms，count=1；进入明细等 1000 ms，count=1；移出等 300 ms，count=0。首次导航/自动定位阶段曾出现关闭及空目标，不作为通过样本；完成滚动定位并先移出再移入产生明确 mouseenter 后执行上述路径。文档明确滚动会关闭弹层。
- 源码核对：withinCreditsRegion 使用 trigger、flyout 及两者间真实通路；mousemove 在区域内取消 timer，离开才调度关闭。probe 已加入从运行时 rect 计算中点、等待后检查，并去除键盘断言里的 hover；其 MouseEvent 是合成回归，不冒充真实指针证据。作者报告的回退验证保留为交接信息，本轮不重做源码回退。
- 验证限制：未重新执行全状态/外观矩阵、覆盖式布局、原生 VoiceOver 或 Dynamic Type；本轮针对发现的右/左间隙与关闭控制。其他页面的五项历史 probe 失败未用于本目标通过或失败的依据。
- L0：make check-whitespace、git diff --check、原型输入清单逐项校验通过；历史轮次按 1、2、3 顺序保留。
- 完成门禁：VERIFIED；固定 gate-status.cypher 最终确认三项 required criterion 均由本轮适用 pass 满足，无缺失、失效或未决影响。新增 ContentState、三个 Evidence 和六个关系数量匹配，关系 preflight 全部 ok；gate 读回确认目标绑定。
- 本记录仅有 SQ-MB-R1-F1，现已关闭；不跨越包含主题完成边界。Widget、Settings、架构和任务分解仍按各自文档推进。

Task checkpoint：ad-sq-doc-ux-menubar-quota-design；content_state `dc3c29d51ce7c7f73e0c8c2511737654fbedf3d0d55bf81d9739320801a1e179`；gate VERIFIED。
提交建议：获授权后提交菜单栏 UX、相关原型及回归变更、评审证据和对应状态；共享文件按 hunk 确认任务边界。
推送建议：获授权并检查提交范围、贡献者和 SSH 签名后推送 feature/subscription-quota；远端/upstream 交付时确认。

### 下一步指令

评审：subscription-quota / ux/widget-quota.md

WORKFLOW_WORKSPACE: agent-deck.subscription-quota
