---
status: active
topic: auxiliary-session-classification
subject: ux/settings.md
---

# Settings 框架评审

## 2026-10-27 证据范围与协调勘误

用户授权修正 Beads 状态后，确认先前将整个文档集缺稿归入本前置文档
document-integrity 的阻塞理由过宽。依据 docs/documentation-workflow.md 的文档集规则，
设计期间允许声明未编写文档；完整集合须在 tasks.md 文档集评审前补齐。
本勘误不改变语义 PASS、不撤销原发现、不豁免文档集/CI 检查，也不新建语义评审轮次。
下面各轮原 BLOCKED 及提交建议作为当时记录保留；当前协调以本勘误的交付对象及 CEv1 新观察为准。

- 交付对象：`6b288ac27bb1c773212dd1baaee91d6317a4687d`；正文 blob `578976e5f1e00bbcc2cf753597cc798ed9934cd2`，与已评审正文一致。
- 新 ContentState：`auxiliary-session-classification:delivered-ux-settings.md-276cd57ad18a687a`；指纹 `276cd57ad18a687a1923c52421369d7d731499438ccdd4afab2de3685ad6f906`。
- 交付目标查询结果：VERIFIED（2026-10-27）；missing criteria 与 unresolved impacts 均为空。
  此结果仅绑定上述交付对象的 framework，不宣称当前全主题或最终 UX 已完成。
- 复用来源：`auxiliary-session-classification:ux-settings-r2-c81d50aaee58257e:independent-review`。新观察绑定交付对象并通过 rolls_up 显式引用；
  提交只固化已评审内容与记录，不引入新的正文决策。标本按各自交付 commit 核验，未用当前其他表面改动冒充旧 manifest。
- 本文检查范围：格式、真实本地链接、评审记录、空白和已绑定标本完整性；不包含尚未设计的另一文档。
  完整文档集缺稿仍由 tasks.md 承接。检查结果及门禁以新目标 CEv1 记录为准。
- 协调边界：本次仅确认已交付 framework；整份 UX 文档仍待 architecture 后 final-surface reconciliation，不关闭任务。后续共享原型变化不能因本次绑定历史交付对象而自动获得最终表面认可。

历史段落中的“尚未提交/不得提交”和旧 BLOCKED 不再用作当前待评审队列依据；
所有已有历史事实和旧目标证据保持不变。

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

## Round 3 — 2026-10-27

## 📋 Settings 最终表面评审

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-SET-R3-F1 — P2 — 回执缺失后本组写入永久锁死，没有架构已提供的同 ID 恢复路径（OPEN）**

位置：ux/settings.md:103（saveUnknown 行“禁止再次写入”）、:115（receiptMissing“仍保持未知/禁止重写”）、:144（“未知结果与冲突必须能通过键盘到达 Read latest values”）；
prototype/src/auxiliarySettingsModel.js `read` 分支（saveUnknown 且无 serverState 时只写入 outcome=unknown 回执、status 不变）与
`save` 守卫（status 为 saveUnknown 时原样返回）；AuxiliarySettings.jsx `locked` 含 saveUnknown，saveUnknown 下只渲染 Read request receipt。

- 行为风险：一旦写入结果未知且本机读不到回执，默认范围与全部覆盖的保存、撤销、查找控件一直禁用；文档又规定该状态存于
  App 级、关闭重开 Settings 后保留，用户在本次 App 生命周期内无法再更改任何分类设置，也没有可执行的下一步。
- 证据：在 prototype 目录以 node 直接驱动模型——`receiptMissing` 舞台保存默认 auxiliary 后为 saveUnknown/`fixture-request-1`；
  连续 3 次 read 仍为 saveUnknown，回执 `{"outcome":"unknown","revision":null}`；`retry` 仍为 saveUnknown；新的默认保存和覆盖 clear
  均返回同一对象（未处理）。架构 §5 规定事务内先检查 request_id：同 ID/同负载若已提交则返回原回执、不再执行 CAS，未提交则按 CAS 执行一次，
  因此同 ID 重提本身是幂等的安全恢复；ux/cli.md:116–118 对同一情况给出“read the request receipt before retrying”。
  Settings 文档既禁止重写又未说明出口，且 :144 要求的 Read latest values 在 saveUnknown 下不存在，文档内部矛盾。
  `check-auxiliary-settings-final.mjs` 的 “missing receipt stays unknown” 只断言保持未知，未覆盖退出路径。
- 💡 修复建议：为回执缺失定义明确出口，并与架构/CLI 一致——例如读回仍为 unknown 后提供“以同一请求 ID 重试同一操作”
  （同负载、同 expected_revision，由 core 的 request_id 去重保证不重复写入），并/或允许 Read latest values 刷新已确认值；
  写清何时解锁其他写入、旧 pending 如何结束，消除 :103/:115 与 :144 的矛盾。更新模型、窗口、检查脚本（含退出后的状态与幂等断言）及 manifest。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- clear 回执固定 `override/automatic/effective=null`、`not_queried`，只有独立查询（inspect 与当前 revision 一致）才显示分类，
  与架构 §5 clear 回执合同一致；tombstone 保留版本、从未存在的键 `expected_revision=0` 返回 no_change/0，CAS 先于同值判定。
- replayed 舞台保留原回执 revision 3，同时列表显示较新的 Auxiliary/revision 4，未被旧回执覆盖。
- 默认 DTO 区分 confirmed 与 value，loading/loadFailed 不显示 Work；来源三态只读、不显示数据根，来源不可用不阻止覆盖。
- 模型合同检查 `node prototype/tools/check-auxiliary-settings-final.mjs` 独立复跑 11 项 failures 0。

### 📝 总结

- Reviewer: Claude Code，独立评审会话，未参与 Settings 设计或 final-surface 候选。
- Method: 单评审者；只读核对 ux/settings.md、architecture.md §5/§7、ux/cli.md 未知结果段、原型 diff 与完整模型/窗口源码，
  以 node 直接驱动内存模型复现；独立复跑模型合同检查。发现决定性问题后未重跑 536 项浏览器断言或 axe。
- Scope: ux/settings.md final-surface 候选及共享标本；不含原生、Go CAS 或真实持久化验收。
- Reviewed state: HEAD `27498c0e34304d1cffceeb9cf66021ac980f7587`；document blob `b6719bd5fece429d80f91f3f1850e99ef943e72a`。
- Specimen identity: manifest SHA-256 `e7192145cb47f32c8a08dcca660ae6eff4a0818e494d6a4434a778f04bb36a46`；`files` 13 项 `shasum -c` 全部 OK。
- Fingerprint: `83a13f5cb736c60772d6340e2cc794403f9449d609c67590da09c229b8d805aa`，SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: `python3 scripts/check-docs.py` 退出 0（结构检查，不认证语义或门禁）。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；ContentState: `auxiliary-session-classification:ux-settings-r3-83a13f5cb736c607`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-SET-R3-F1 为 fail；Review 保持未勾选，Beads 返回修复。
- Disposition: ASC-SET-R1-F1 仍 CLOSED（loading/loadFailed 显示 Unavailable、不补 Work，本轮未回退）；ASC-SET-R3-F1 OPEN。
- Residual uncertainty: 真实 Go 事务、跨重启回执读回、native AX/Dynamic Type 仍属实现验收。

修复：auxiliary-session-classification / ux/settings.md / ASC-SET-R3-F1

## Repair candidate — 2026-10-27（Round 3 后）

- Scope: ASC-SET-R3-F1 only；作者 Codex，待独立复评，保留 Round 3 FAIL / FAILED。
- Disposition: ASC-SET-R3-F1 -> repaired in candidate。回执查询仍 unknown 后提供同 ID 重试，
  保留原目标、负载、expected_revision；确定结果结束 pending 并解锁，冲突读最新值后重新选择。
  不以查询无回执推断未提交，不用新 ID 盲写，也不将原回执版本当当前覆盖。
- Changes: ux/settings.md、共享 Settings 模型/窗口、模型与浏览器检查、manifest 和修复截图。
- Evidence: 14 项模型合同检查通过；构建通过；en/dark/320、zh/light/320 各 70 项浏览器断言，
  共 140 项、0 失败。覆盖关闭重开后恢复、后续新保存、clear 幂等、较新覆盖保留及 CAS 冲突。
  r3fix-receipt-retry-zh-320.png 已目视检查。未重跑未改变的完整矩阵与 axe。
- Candidate identity: HEAD `27498c0e34304d1cffceeb9cf66021ac980f7587`；document blob `090ce4e2151722f3c89c08714e2501cc8eee41ce`；
  manifest SHA-256 `8be1d4a4e253f737f7029c29092127312b02aa28adbfd892bad96d855c293f32`。
- Limits: 合成内存标本；不宣称真实 CAS/持久化/native 验收、独立 PASS 或 CEv1 VERIFIED。

## Round 4 — 2026-10-27

## 📋 Settings 最终表面修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-SET-R3-F1 — CLOSED**：ux/settings.md:103、:115–121、:150 改为回执查询仍 unknown 后提供“以同一请求 ID 重试同一操作”，
保留 App 级 pending 的 request_id/目标/负载/expected_revision，确定结果结束 pending 并解锁，CAS conflict 经 Read latest values 结束旧 pending；
原 :103/:115 与 :144 的矛盾已消除。模型在 `retry` 分支仅于 saveUnknown 且回执 outcome=unknown 时以原 pending 调用 apply，
窗口仅在同一条件下渲染该按钮。

独立以 node 驱动内存模型复现：receiptMissing 保存默认 auxiliary → 读回前 retry 不生效（仍 saveUnknown）→ 读回 outcome unknown →
retry 得 saved、默认 auxiliary、回执 request_id 与原 `fixture-request-1` 相同、pending 清空 → 之后新保存 all 使用 `fixture-request-2` 成功；
在读回后把目标覆盖版本外部改为 9 再 retry 得 conflict 且保留 pending，Read latest values 后为 reloaded、pending 清空；
成功后以原 ID/原负载再提交得 replayed、revision 1，不重复写入。目视 r3fix-receipt-retry-zh-320.png：回执 unknown、
待确认操作、请求 ID/expected revision 0、“查询请求回执”与“以同一请求 ID 重试同一操作”同时可见。

### 📝 总结

- Reviewer: Claude Code，原 Round 3 独立评审会话，未参与修复。
- Method: 单评审者；核对修复 diff、文档恢复语义与架构 §5 request_id/CAS 顺序，node 驱动模型复现退出、冲突与重放路径，
  独立复跑 `node prototype/tools/check-auxiliary-settings-final.mjs`（14 项，failures 0）；修复者 140 项浏览器断言与构建保留在 Repair 段，本轮未重跑；
  其他颜色/布局/辅助功能结构未改变，不作新 axe/native 声明。
- Scope: ASC-SET-R3-F1 及其对 ux/settings.md final-surface 与共享标本的影响；ASC-SET-R1-F1 未回退，仍 CLOSED。
- Reviewed state: HEAD `27498c0e34304d1cffceeb9cf66021ac980f7587`；document blob `090ce4e2151722f3c89c08714e2501cc8eee41ce`。
- Specimen identity: manifest SHA-256 `8be1d4a4e253f737f7029c29092127312b02aa28adbfd892bad96d855c293f32`；`files` 14 项 `shasum -c` 全部 OK。
- Fingerprint: `c2420303bc2e0aae3d4b44dd0b6863c46aae6215ae5ced11adc2029bc1b929b4`，SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: `python3 scripts/check-docs.py` 退出 0；`git diff --check` 退出 0；评审记录经 `scripts/check-review-records.py` 校验。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；ContentState: `auxiliary-session-classification:ux-settings-r4-c2420303bc2e0aae`。
- Completion gate: VERIFIED
- Disposition: ASC-SET-R1-F1 CLOSED；ASC-SET-R3-F1 CLOSED。
- Residual uncertainty: 真实 Go 事务与跨重启回执读回、native AX/Dynamic Type 仍属实现验收。

## Round 5 — 2026-11-03

## 📋 Settings 集成修订评审（批量评审：所有文档）

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

以下是本轮开放发现，PASS 前必须关闭。

**ASC-SET-R5-F1**（P2）[ux/settings.md:90-94] 内建来源行的状态集合不完整，且数据来源未定义。
- 行为风险: 只定义“core 可读即可用”和“尚未开启 OTel 采集→无后台会话记录”。未定义 core 不可读时该行显示什么（是否沿用 sourceUnavailable），
  也未区分“已开启但尚无记录”；状态清单（第 18 行 sourceUnconfigured/sourceUnavailable/sourceUnsupported）没有加入内建行的状态。
  “是否开启 OTel”需要读取 telemetry 设置，而架构 §3.4 声明该来源不读配置，判定数据无合同（见 ASC-ARCH-R3-F5）。
- 证据: 原型 `prototype/src/AuxiliarySettings.jsx` 第 80 行（blob `3987616a953310e03956477cfe102cfabd94d82b`）在所有场景中都把内建行渲染为“可用”，没有对应状态标本。
💡 修复: 列出内建行的完整状态（可用/无记录·未开启/无记录·已开启/core 不可读）及各自安全文案与数据字段，按 ASC-ARCH-R3-F5 的 DTO 对齐，并在原型中给出至少一个非默认状态标本。

### 🟢 优点

- 内建行不提供控件、把采集开关留在既有 OTel 区域（`apps/macos/AgentDeckApp/SettingsWindowView.swift:46-62` 已存在 OTel 开关），避免重复控制点。
- 明确标本为合成文案、不代表真实采集状态或原生验收。

### 📝 总结

- Reviewer: Claude Code（claude-code），独立评审会话。
- Method: 单评审者；读 diff、来源状态章节与原型改动；未重新渲染原型（修订者会话记录了原型检查通过，本轮不把它当作本评审证据）。
- Scope: ux/settings.md 全文与 `prototype/src/AuxiliarySettings.jsx` 改动；依赖 architecture.md（Round 3 FAIL）。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/settings.md blob `23ab1149b5a74770ef064d071bf897bac608386d`；原型源 blob `3987616a953310e03956477cfe102cfabd94d82b`（均未提交）。
- Fingerprint: `6fd65e74619902de7794cd99abad786485e27f8543016be197a0d7b2cbcb457a`；SHA-256(`head=<HEAD>;document=<blob>`)，不含原型 manifest。
- Evidence: `python3 scripts/check-docs.py` 通过；`scripts/check-whitespace.sh` 无输出。
- Disposition: ASC-SET-R5-F1 开放。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；本内容状态未记录 pass 证据。
- Completion gate: NOT_VERIFIED

## Repair candidate — 2026-11-04（Round 5 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-SET-R5-F1，未委派。
- ASC-SET-R5-F1 → repaired in candidate。来源状态区改为读取 `session origin-source status`；内建行列出五种状态（有记录、无记录·未开启、无记录·已开启、无记录·开关未知、core 不可读）及数据字段与中英安全文案；claude-mem 六种原因对应文案；状态清单加入 codexReceiverOff/codexNoRecords/codexReceiverUnknown，loadFailed 两行均 core_unavailable。
- 原型：`AuxiliarySettings.jsx` 以 `data-state` 渲染五态，`auxiliarySettingsModel.js` 增三个 stage 场景；新增截图 `ux/prototype/settings/r5-codex-receiver-off-zh-320.png`；manifest 刷新为 integration-repair-r5（旧 manifest SHA-256 `8be1d4a4e253f737f7029c29092127312b02aa28adbfd892bad96d855c293f32`）。合成文案，不代表真实采集或原生验收。
- Candidate identity：HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/settings.md blob `bb96bb3d65d8ce625bf7250ee4e741f2286a9e03`；原型 AuxiliarySettings.jsx blob `66d1825797e80d11838d862f400f112b9526d704`、auxiliarySettingsModel.js blob `468c44fffe204344152772019079008c0c64d6f9`、manifest blob `cb5ae69ee4ee80b5614a9c23cdb005e96c1ddd75`。
- 验证：`npm run build` 通过；auxiliary CLI/Settings/Widget final 契约脚本 exit0（14/14/39 项）；浏览器 Settings 脚本 70 项、菜单栏 176 项 0 失败；Settings 五种 Codex 行状态逐一渲染核对。`python3 scripts/check-docs.py`、`scripts/check-topic-docs.sh`、`scripts/check-whitespace.sh` 见修复交接。未运行产品测试（无产品代码变更）。
- 原 FAIL/OPEN 与 NOT_VERIFIED 保留；本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 6 — 2026-11-04

## 📋 Settings 集成修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-SET-R5-F1 — CLOSED**：
  - 内建 Codex 行定义了五种状态：有记录、未开启、已开启无记录、开关未知、core 不可读。
  - 数据来源限定为 `session origin-source status` 的 JSON。
  - 三种“无记录”都不算错误；有记录时，即使接收器已关闭也显示可用。
  - 原型 `AuxiliarySettings.jsx` 用 `data-as-source-codex data-state` 渲染这五态，新增三个 stage 场景；loadFailed 下两行均为 core_unavailable。
- 新截图 `r5-codex-receiver-off-zh-320.png` 的 SHA-256（`08a4407b…c581a`）与 manifest 一致。目视确认：320 宽下显示 receiverOff 文案，无溢出。
- claude-mem 行补全了 source_disabled、source_not_yet_read、source_config_unreadable 三种文案，与架构 §7 的枚举对应。

### 📝 总结

- Reviewer: Claude Code（claude-code），Round 5 评审会话；未参与修复。
- Method: 单评审者。读文档与原型 diff；重跑 `node prototype/tools/check-auxiliary-settings-final.mjs`，14 项 0 失败（仅检查合成模型）；目视检查新截图。未重跑浏览器脚本。
- Scope: ASC-SET-R5-F1，以及来源状态段落、原型和 manifest 的变更。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/settings.md blob `bb96bb3d65d8ce625bf7250ee4e741f2286a9e03`（未提交）；原型 AuxiliarySettings.jsx blob `66d1825797e80d11838d862f400f112b9526d704`。
- Fingerprint: `4b3a36b280aba0028a9c71dbc10bfef723504ff7c64b989249ad75c9bca91ba3`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`bash scripts/check-topic-docs.sh` exit 0；`bash scripts/check-whitespace.sh` exit 0。
- Dependency: 如果 ASC-ARCH-R4-F1 把已停用 claude-mem 的 available 定为 false，与本文的“已停用”文案相容。原型中 sourceAvailable 只在三个 source* 场景和 loadFailed 下为 false，没有 disabled 场景，因此不冲突。
- Disposition: ASC-SET-R5-F1 CLOSED。
- WorkUnit: `auxiliary-session-classification:ux/settings.md`；ContentState: `auxiliary-session-classification:set-r6-4b3a36b280aba002`。
- Completion gate: VERIFIED
- Gate scope: 本文档及其合成原型标本的独立复评；不认证原生 Settings 实现。
