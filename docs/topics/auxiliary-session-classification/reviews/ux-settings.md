---
status: active
topic: auxiliary-session-classification
subject: ux/settings.md
---

# Settings 框架评审

## Round 1 — 2026-10-27

## 📋 Settings 交互框架评审

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-SET-R1-F1 — P2 — 未确认默认值在菜单栏消费者中被当作 Work 使用（OPEN）**

位置：prototype/src/auxiliarySettingsModel.js:9；prototype/src/App.jsx:172；prototype/src/Popover.jsx:1408。
关联 ux/settings.md:38、110 的默认值可信状态和跨表面合同。

- 行为风险：用户已有默认可能是 Auxiliary/All，但读取失败时新打开菜单栏却用 Work 生成正常集合和计数；Settings 的 Unavailable 只遮住局部显示，没有阻止错误默认传播。界面产生了需求明确禁止的假确认状态。
- 最小复现：共享入口 `?settings=1&auxSettings=loadFailed&auxMenu=normal&tab=sessions&lang=en`。确认 Settings 的 status=loadFailed、Saved=Unavailable、选择值为空；不点击 Read latest values，关闭 Settings，再关闭并重新打开菜单栏。
- 浏览器直接结果：新菜单栏 state=normal、scope=work、count=3，提示 `Includes 1 sessions with unknown origin.`，没有默认不可用提示。操作通过隔离 agent-browser 的 DOM click 与双 requestAnimationFrame 等待完成，是合成标本交互证据，不是产品 Swift 或持久化测量。
- 根因证据：initialAuxSettings 对 loading/loadFailed 仍填 defaultScope='work'；AuxiliarySettings 仅按 status 隐藏已保存值。App 向 Popover 无条件传递该 defaultScope，Popover 在初始化时直接消费，未收到是否已确认的信息。本例没有任何已确认旧偏好或已确认旧快照的来源，不能将合成初值解释成 last-known-good。
- 💡 修复建议：把默认值及其确认/读取状态一起传递给消费者。首次确定无配置才允许 Work；loading/loadFailed 时保留有来源的旧可靠 selection/快照或显示不可用，不创建伪正常 Work。补充读取失败和加载中关闭 Settings、重开 Popover 的组合标本及检查；读回成功后才初始化新默认。核对新设置功能关闭时兼容行为，更新 manifest/必要截图，不在此修复中发明真实存储或 Widget 实现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 默认偏好和单会话覆盖分别保存，精确身份查询与输入变化清空目标的规则明确。
- saving、失败、结果未知、冲突和发布待完成有独立反馈，窗口不拥有已接受写入的生命周期。
- 覆盖撤销、未匹配记录和隐私边界清晰；不把浏览器标本当成原生持久化证明。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与 Settings 设计。
- Method: 单评审者只读文档、原型模型、App/Popover/Settings 接口与检查脚本；隔离 agent-browser 最小反例。明确阻塞后停止广泛验证，未重跑设计者的 336 项、149 项或 axe 检查，未运行产品测试。
- Scope: ux/settings.md framework 及所绑定共享标本；不重判此前菜单栏框架历史目标，不评审 final-surface。
- Reviewed state: HEAD `298f71ea8817139941e50a5af59440a65457c77f`；document blob `8bb89895fbd8490ad3d631adf79c8249ea104744`。
- Specimen identity: manifest SHA-256 `71fbf71b9f8a5799b616169b473a10edd5a32a3767de32c5774c47cc02e6a04d`；12 项源文件、脚本和截图均 MATCH。
- Fingerprint: `c4e4785af457c3d88e10d12333d3620c431e3792db061a5d39c15da1974bd7c1`；SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 上述 browser before={loadFailed,Unavailable,empty choice}；未读回时 reopened={normal,work,3}。现有 check-auxiliary-settings.js 验证 Settings 局部不可用后立即读回，默认联动仅在成功路径检查，缺少未确认默认流向新 Popover 的组合。
- Evidence: `python3 scripts/check-docs.py` 退出 1，仅报告 ux/widget.md、architecture.md 已声明未编写；该文档集生命周期缺口与本 finding 分开，不填占位或删除矩阵。
- Environment: 临时 Vite 服务读取本工作区，复用已有依赖，不安装或改配置。无关 widget-contrast-probe 的 axe-core 解析警告未阻止 Settings 页面；无本轮 axe 通过声明。临时浏览器、服务和缓存不是持久引用。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；ContentState: `auxiliary-session-classification:ux-settings-r1-c4e4785af457c3d8`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-SET-R1-F1 为 fail；document-integrity 因 Widget/architecture 缺稿为 blocked。Review 保持未勾选，返回同一文档任务修复。
- Evidence: 评审记录解析、`make check-whitespace`、`git diff --check` 均成功；状态摘要同步不修改被评审的正文或标本。
- Residual uncertainty: 真实并发、持久化、备份恢复、Widget 和原生无障碍仍须后续设计与实现证据。

修复：auxiliary-session-classification / reviews/ux-settings.md / ASC-SET-R1-F1

## Repair candidate — 2026-10-27（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-SET-R1-F1，未委派，未修改产品 Go/Swift。
- ASC-SET-R1-F1 → repaired in candidate，待独立复评确认。初始 loading/loadFailed 现在保存
  defaultScope=null、defaultConfirmed=false；App 传递值与确认状态，Popover 在未确认时
  显示默认不可用提示，不生成分类范围、列表或计数。无来源的初值不再冒充 last-known-good。
- 标本读回/完成加载明确确认无配置后，才置 confirmed=true 并初始化 Work；已有确认选择
  不被正常默认值保存强制替换。设置功能关闭时仍按原独立菜单栏默认规则运行。
  文档补充未确认值跨消费者合同，不发明真实持久化或 Widget 实现。
- Candidate identity：HEAD `298f71ea8817139941e50a5af59440a65457c77f`；document blob
  `578976e5f1e00bbcc2cf753597cc798ed9934cd2`；manifest SHA-256
  `078e780ea02880fa375a801a496f08dd634ad76da92d601c7df9585eafa4479f`。
- 验证：原型构建成功；中英 × 320/520 深色四组，每组 50 项共 200 项浏览器断言无失败。
  新增 loading/loadFailed 下关闭 Settings、重开 Popover、tab 往返的不可用/无分类计数断言，
  以及确认读回后初始化和再次重开的正常选择。原保存、撤销、失败/未知/冲突、窗口生命周期检查保留。
- 设置功能关闭时菜单栏 149 项检查通过。布局/颜色未改，复用原相关检查；未声称新一轮 axe/native 通过。
- 原最小反例读回 unavailable=true、scopePanel=false、classifiedCounts=false；
  新截图 `r1fix-unconfirmed-popover.png` 已目视核对并纳入 manifest。顶部全部消耗保持原数据域，
  不将其误作工作范围的分类计数。原设计检查保留为 previous_design_checks。
- `python3 scripts/check-docs.py` 退出 1，仅报告 Widget 和 architecture 两份声明未编写文档。
  不填占位、不削减矩阵；原 Round 1 FAIL/OPEN 与原目标 FAILED 保留。新候选等待独立复评和
  证据门，Review 仍未勾选，不宣称 CEv1 VERIFIED、真实存储或原生验收。

## Round 2 — 2026-10-27

## 📋 Settings 框架修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-SET-R1-F1 — CLOSED**：默认值与 defaultConfirmed 同传；loading/loadFailed 初值为 null/false，
Popover 不再将未知值回退为 Work。文档明确无可靠旧值的标本应显示不可用，读回确认后才初始化。
已打开窗口的正常临时范围不随新默认保存立即切换。

### 📝 总结

- Reviewer: Codex，原独立评审会话；未参与修复。
- Method: 单评审者检查模型/App/Popover 变更及文档，使用隔离 agent-browser DOM click 与双 requestAnimationFrame 重放，查看哈希匹配的修复截图；未委派、不修复产品或标本。
- Scope: ASC-SET-R1-F1 的未确认默认消费者及恢复/正常保存路径；framework，不是最终 wire、真实存储或 Widget 实现验收。
- Reviewed state: HEAD `298f71ea8817139941e50a5af59440a65457c77f`；document blob `578976e5f1e00bbcc2cf753597cc798ed9934cd2`。
- Specimen identity: manifest SHA-256 `078e780ea02880fa375a801a496f08dd634ad76da92d601c7df9585eafa4479f`；13 项源文件、脚本和截图全部 MATCH。
- Fingerprint: `c81d50aaee58257ec361b1ef8479235c526af20c10f944395b68acaaefb523b8`；SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 独立浏览器分别从 loadFailed/loading 开始，Settings 均为 Saved=Unavailable；关闭设置、重开菜单栏、Usage/Sessions 往返，均为 unavailable=true、scope=null、count=null。没有点击读回前不产生分类计数。明确 Read latest values/Complete pending 后才变为 work/3，重开保持 work/3。合成读回明确无配置，非通用持久值假设。
- Evidence: 随后保存默认 Auxiliary，当前 Popover 保持 work/3，关闭重开才为 auxiliary/1；与临时选择/持久默认合同一致。目视核对 r1fix-unconfirmed-popover.png：局部不可用说明清楚，顶部总消耗仍是独立数据域。
- Evidence reuse: 未改 Settings 布局、颜色与控件语义，不重跑无关构建/axe。修复者的 200 项 Settings 和 149 项关闭功能的菜单栏检查保留为原作者证据，本轮未冒充独立复跑。未启用 auxSettings 时 auxDefaultPreference=null 的原分支保持兼容。
- Environment: 沿用本会话临时 Vite 源码服务与已有依赖；无关 axe-core 解析警告不影响该页面，未声明新 axe 结果。浏览器合成状态验证不证明真实 IO、native AX 或持久化。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；ContentState: `auxiliary-session-classification:ux-settings-r2-c81d50aaee58257e`。
- Completion gate: BLOCKED
- Gate scope: independent-review 为 pass；document-integrity 仍缺 ux/widget.md、architecture.md。保留原目标 FAIL/FAILED 历史，不覆盖旧证据。
- Hierarchy: 仍有其他设计与分解未完成，不跨越 Topic 完成边界；architecture 后仍需 final-surface reconciliation。
- Evidence: 记录与矩阵同步后，`python3 scripts/check-docs.py` 退出 1，仅报告两份下游文档缺稿；评审记录解析、`make check-whitespace`、`git diff --check` 均退出 0。
- Disposition: ASC-SET-R1-F1 CLOSED；Review 勾选为 framework PASS，Beads 保持 in_review。

Task checkpoint：ad-auxiliary-session-classification-doc-ux-settings；content_state `c81d50aaee58257ec361b1ef8479235c526af20c10f944395b68acaaefb523b8`；gate BLOCKED。
提交建议：门禁 VERIFIED 后，经授权提交 Settings 文档、共享原型相关改动、检查脚本、manifest/截图、评审记录与矩阵；前置为完成 Widget 和 architecture 文档并通过完整性检查。
推送建议：门禁 VERIFIED 后，经授权推送 feature/auxiliary-session-classification；前置同上，remote/upstream 待交付解析，提交对象和 SSH 签名需核验。

完成已声明的 ux/widget.md 和 architecture.md，通过文档完整性检查并重算门禁；保留本轮 framework PASS。
