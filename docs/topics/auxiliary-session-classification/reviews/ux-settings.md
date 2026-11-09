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

## Round 7 — 2026-11-07

## 📋 Settings 重新设计候选评审

📊 总体评分：6.5/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

**ASC-SET-R7-F1 — P2 — OPEN：core 不可读时仍承诺手动分类可用。**

位置：[Settings 自动识别](../ux/settings.md#自动识别只读)第 246–248 行；
`prototype/src/AuxiliarySettings.jsx:61–65, 148–159, 286`。

- 行为风险：文档明确把“含 core 不可读”纳入“手动分类不受影响、仍可设置或撤销”，
  将外部识别来源失效与保存覆盖记录的 core 失效混为一谈。用户看到仍可操作的承诺，实际无法查找或管理覆盖；
  实现者也可能把尚未读回的覆盖或分类当成已确认结果。
- 证据：独立浏览器打开 `?settings=1&auxSettings=loadFailed&auxMenu=normal&tab=sessions&lang=en&theme=light`，
  再打开 Manage。两条来源均显示 `Local usage database unreadable`；来源说明仍为
  `While a source is unavailable, detection reports Unknown, which counts as Work. Manual classification still works.`；
  ID 输入与 Find 的 `disabled` 均为 true，列表显示 `The list cannot be read right now; it is not treated as empty.`。
  [架构 §5](../architecture.md#5-写入幂等与前台读)要求 clear 读取 core 的键和版本，不把 core 读取失败解释为空覆盖；
  [架构 §7](../architecture.md#7-cli-与桌面协议)规定 core 不可读时依赖字段为 null。
- 💡 有界修复：区分外部适配器不可用和 core 不可读。只有前者能在 core/身份可读时继续管理覆盖；
  后者明确说明当前分类/覆盖不可读及恢复入口，不承诺当前可写，也不推断 Unknown 或已生效覆盖。
  同步中英文案和对应标本，保持已保存数据不被删除的承诺。

**ASC-SET-R7-F2 — P2 — OPEN：手动分类 sheet 的键盘焦点可以越出模态范围。**

位置：[Settings 键盘、焦点与辅助功能](../ux/settings.md#键盘焦点与辅助功能)第 322–327 行；
`prototype/src/AuxiliarySettings.jsx:131–139, 196–199, 242`。

- 行为风险：文档规定 sheet 为模态对话框，打开后由其接收焦点，但原型只把正文/工具栏设为 inert，
  没有封闭 Tab 顺序。键盘用户能离开仍打开的 sheet，使当前操作位置和模态语义不一致。
- 证据：独立浏览器在 normal 标本打开 Manage，将焦点放到 `[data-as-sheet-done]`，按一次 Tab。
  读取结果为 `dialogOpen=true, inSheet=false, document.activeElement.tagName=BODY`。
  源码 sheet 的 keydown 仅处理 Escape，没有 Tab/Shift+Tab 循环或等效原生模态机制。
  本结论是合成浏览器标本的确定反例，不是对原生 VoiceOver 的推断。
- 💡 有界修复：在文档明确 sheet 内的首尾焦点循环，并在标本落实 Tab/Shift+Tab 封闭；
  锁定状态仍须能到达查询、重试和完成。保留 Escape 只关闭 sheet、关闭后回到 Manage 的规则。

### 🟡 建议改进 — 推荐

**ASC-SET-R7-F3 — P3 — OPEN（本轮必须修复）：标题改名使历史证据链接失效。**

位置：`ux/settings.md:128`，以及 `reviews/origin-store.md:28`、`reviews/settings-control.md:66` 的引用。

- 行为风险：从既有评审证据无法跳转到默认范围合同，L0 文档门禁失败。
- 证据：HEAD 中存在 `## 默认范围`；当前改为 `## 默认会话范围`，没有保留旧锚点。
  `python3 scripts/check-docs.py` exit 1，分别报告上述两处 `missing Markdown anchor: ../ux/settings.md#默认范围`。
- 💡 有界修复：在新标题前保留 `默认范围` 兼容锚点，修复导航而不重写历史评审结论。

### 🟢 优点

- 默认范围只呈现最近确认值，在途目标另行表达；失败、未知、冲突与派生发布延迟仍分开。
- 手动分类移入 sheet，主设置页保留摘要；core-only clear 后的独立分类查询、同请求身份重试和跨窗口 pending 语义均有明确约束。
- manifest 的 11 个源码/检查脚本及 5 张新截图均与实际 SHA-256 一致；旧设计与新候选边界清楚，分页方案明确仍待用户决定。

### 📝 总结

- Reviewer：Codex；本会话未参与此重新设计或产品实现。
- Method：单评审者冷上下文审查；文档/架构对照、工作树专属 CodeGraph 源码追踪、标本哈希核对、
  保留截图目视检查及独立命名浏览器中的最小交互反例。确认阻断后停止扩大验证。
- Scope：`ux/settings.md` 重新设计及 manifest 绑定的共享原型。窗口分页仅审查其候选边界，未批准采用；
  不评审或修改并行原生 `settings-control` 实现，不更改其历史 PASS 或用户限定的验收范围。
- Reviewed state：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；
  document blob `9cd1c1a328edcbcf29062fc8582e9c216c8fe04c`；
  prototype manifest SHA-256 `3d04935e02646769030092571c80964a6562ff89bfe7756ef705c0b4ef3e6924`。
- Fingerprint：`03b581b3fd6dac3b493f10ea37f2ab6124f3d431a6845321563ad8ccd3a1e8b5`；
  SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Dependency：requirements blob `50da36642b7db63fe8286893141a42b464a5559c`；
  architecture blob `416d63c922b9ea445a53a9774388efa13776af1c`，与文档声明一致。
  稳定 CLI Design 的已交付 Settings 条目作为基线读取，本主题新增偏好按自己的 requirements/architecture 审查。
- Evidence：`node prototype/tools/check-auxiliary-settings-final.mjs` 为 14 项、0 失败，仅证明合成模型；
  文档 L0 的两处失效链接及两项浏览器反例见上。`bash scripts/check-topic-docs.sh auxiliary-session-classification`、
  `make check-whitespace`、`git diff --check` 的检查序列未产生诊断，结束码 0。
  作者登记的 412/444 项浏览器检查不是本轮重跑结果，不能排除本轮发现。
- Disposition：ASC-SET-R7-F1、ASC-SET-R7-F2、ASC-SET-R7-F3 OPEN。
  旧 ASC-SET-R1-F1、ASC-SET-R3-F1、ASC-SET-R5-F1 的历史关闭事实保留；不把旧候选 PASS 迁移到重设计。
- WorkUnit：`auxiliary-session-classification:ux/settings.md`；
  ContentState：`auxiliary-session-classification:ux-settings-r7-03b581b3fd6dac3b`。
- Completion gate：FAILED
- Gate scope：当前文档与合成标本的 document-integrity、independent-review；已登记 1 个 ContentState、
  2 条失败 Evidence 和 4 条关系，关系预检全部 ok。正式精确状态查询返回 FAILED，两项失败证据均为
  target_matches/applicable=true、malformed/invalidated/unresolved=false。未到达 Task 或 Topic 完成/交付边界。
- Handoff：同一文档任务 `ad-auxiliary-session-classification-doc-ux-settings` 返回修复，Documents 的 Review 保持未勾选。
  本轮仅写评审/状态及证据协调记录，未修复文档或原型，未提交、推送。

## Repair candidate — 2026-11-07（Round 7 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-SET-R7-F1、ASC-SET-R7-F2、ASC-SET-R7-F3，未委派。
- 复现：三项先按 Round 7 证据确认。`python3 scripts/check-docs.py` 修复前报告两处 `missing Markdown anchor: ../ux/settings.md#默认范围`；
  原型 loadFailed 的来源说明为 `…Manual classification still works.`；源码 sheet 的 keydown 只处理 Escape。
- ASC-SET-R7-F1 → repaired in candidate。[自动识别](../ux/settings.md#自动识别只读)按失效位置分两种说明：
  外部来源不可用（core 可读）保留原句，手动分类照常；core 不可读时说明当前分类和手动分类都不可读、暂时不能修改、
  已保存的不会被删除，并附「重新读取」；sheet 状态条说明查找与修改暂不可用，列表保持「不会当作没有覆盖」。
  不推断 Unknown，也不把读取失败前的覆盖当作仍然生效。原型 `coreUnavailable` 改为跟随 loadFailed 状态，
  `SOURCE_DOWN` 不再包含 loadFailed；重新读取成功后来源、覆盖、默认范围按读回结果恢复。
- ASC-SET-R7-F2 → repaired in candidate。[键盘、焦点与辅助功能](../ux/settings.md#键盘焦点与辅助功能)规定 sheet 内
  Tab / Shift+Tab 首尾循环、焦点停在 sheet 本身时的进入方向，以及锁定时循环只在可用的状态操作与「完成」之间进行；
  Escape 规则不变。原型在 document 上监听 Tab，焦点因控件被禁用掉到 body 时也回到 sheet；窗口外的舞台控件不受影响。
- ASC-SET-R7-F3 → repaired in candidate。「默认会话范围」标题前加 `<a id="默认范围"></a>` 兼容锚点；未改动历史评审记录。
- 回归保护：`check-auxiliary-settings.js` 新增 10 项（焦点循环 6 项，含锁定 saveUnknown 下在「查询结果」与「完成」间循环；
  core 不可读说明、sheet 锁定说明、恢复后来源与查找可用 4 项），三种 source* 场景另断言说明属于外部来源一类。
- 验证（本候选内容）：`npm run build` 通过；浏览器检查长列表每组 113 项 × 中英深浅 4 组 = 452 项，分页每组 121 项 × 4 组 = 484 项，0 失败；
  `node prototype/tools/check-auxiliary-settings-final.mjs` 14 项 0 失败；菜单栏 176 项 0 失败。
  真实按键（agent-browser `press`）复核 Round 7 反例：长列表 en/light 与分页 zh/dark 中，从「完成」按 Tab 得到 `dialogOpen=true, inSheet=true`，
  焦点到客户端选项；Shift+Tab 回到「完成」。
  scoped axe 4.12.1（loadFailed）：主表面 en/light、zh/dark 均 0 violations、0 incomplete；sheet 0 violations，
  incomplete 仅为遮罩下无法确定背景的 color-contrast；新增的 bad 状态条文字手算对比度浅色 5.93、深色 4.69。
  `python3 scripts/check-docs.py` 通过（修复前的两处失效锚点消失），`bash scripts/check-topic-docs.sh auxiliary-session-classification` exit 0，
  `bash scripts/check-whitespace.sh` exit 0，`git diff --check` 无输出。未运行产品测试（无产品代码变更）。
- Candidate identity：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；ux/settings.md blob `2b873cd65cb8e6c68cdd900f5ca41533de04e011`（未提交）；
  原型 AuxiliarySettings.jsx SHA-256 `ecc8b956aebae204b01fe28fe8278def4cd164586ed02e40ce203d539d6df4ce`、
  check-auxiliary-settings.js SHA-256 `7193230b872b92e80cf226a4e7f05a27923041e6ffac67615f229b6261279d85`；
  manifest 阶段 `redesign-r1-repair-r7`，SHA-256 `294305252a547d38a044c78a78061a7ca95a6ae88f868a07b350e806161c56d7`
  （上一份 `3d04935e02646769030092571c80964a6562ff89bfe7756ef705c0b4ef3e6924` 记入 previous_manifest_sha256）。
- 原 FAIL/OPEN 与 Round 7 的 FAILED 门禁保留；本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 8 — 2026-11-08

## 📋 Settings 重新设计修复复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。ASC-SET-R7-F1、ASC-SET-R7-F2 均在当前候选关闭，未发现修复引入的新阻断。

### 🟡 建议改进 — 推荐

无。ASC-SET-R7-F3 已关闭，不保留本轮待修发现。

### 🟢 优点

- **ASC-SET-R7-F1 — CLOSED**：`ux/settings.md:249–262` 区分外部来源不可用与 core 不可读，
  `AuxiliarySettings.jsx:63–69, 311–313` 按当前读取状态选择说明。
  独立浏览器打开长列表 en/light 的 loadFailed 标本并打开 Manage：来源说明的类型为 `core`，
  明确当前分类和手动分类不可读、暂时不可修改、已保存内容不删除；sheet 状态条同样说明不可查找/修改。
  ID 输入禁用，默认范围没有选中项。点击列表的 Read again 后，状态变为 `ready`，core 说明消失，
  输入恢复可用，列表显示两项，来源按读回结果恢复。sourceUnavailable 对照仍为 `adapter` 说明，
  输入与移除均可用，未把外部适配器失败误变成全组不可用。架构 §4 的 core 存储归属与本次说明一致。
- **ASC-SET-R7-F2 — CLOSED**：`ux/settings.md:338–342` 明确首尾循环与锁定状态下的可达操作；
  `AuxiliarySettings.jsx:142–162` 在当前可聚焦集合中处理 Tab/Shift+Tab，并回收落到 body 的焦点。
  独立命名浏览器以真实 `press Tab` / `press Shift+Tab` 验证长列表 en/light 和分页候选 zh/dark：
  从 Done 向前到 Codex，再向后回到 Done，始终 `inSheet=true`。
  分页候选的正文和工具栏在 sheet 打开时均 inert；Escape 只关闭 sheet，窗口保留、焦点回到 Manage，
  正文和工具栏解除 inert。另在 zh/dark 中为 Claude old-01 发起 Auxiliary 写入，进入 `saveUnknown`：
  ID 输入禁用，请求身份 `fixture-request-1` 保留；真实按键在 Check result 与 Done 间双向循环，操作仍可达。
- **ASC-SET-R7-F3 — CLOSED**：`ux/settings.md:129` 保留 `<a id="默认范围"></a>`。
  原 `origin-store` 和 `settings-control` 历史引用无需改写；`python3 scripts/check-docs.py` exit 0，
  上轮两处 missing Markdown anchor 已消失。

### 📝 总结

- Reviewer：Codex；沿用 Round 7 评审角色，未参与 Claude Code 的修复，未委派。
- Method：逐项处置既有发现；核对修复交接、当前文档、所选工作树 CodeGraph 源码、manifest 身份，
  以及独立浏览器中的真实按键与读回恢复。源码定位不足的顶层浏览器检查脚本使用一次有界源码读取。
- Scope：ASC-SET-R7-F1/F2/F3 与受影响的文案、默认确认状态、来源降级、sheet 焦点和锚点引用。
  长列表仍为确定的默认方案；分页的候选身份不变，本轮 PASS 不批准采用分页。
- Reviewed state：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；
  document blob `2b873cd65cb8e6c68cdd900f5ca41533de04e011`；
  prototype manifest SHA-256 `294305252a547d38a044c78a78061a7ca95a6ae88f868a07b350e806161c56d7`。
- Fingerprint：`8424e6bfac038d3b24e513eac849df516421a3f7720af7d9c9ff30f54a03f007`；
  SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence：11 个源码/检查脚本及 5 张截图的 SHA-256 全部与 manifest 一致；
  修复记录的 AuxiliarySettings.jsx、浏览器检查脚本、文档和 manifest 身份均与本轮一致。
  本轮直接取得的浏览器结果见逐项处置；文档结构检查通过。
  本轮状态同步后的 `check-review-records.py --base 27a12fc3c811d176cbd17fdf8df1293b20eca093`、
  `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 均 exit 0。
  复用 Repair candidate 的同内容构建、452/484 项浏览器检查、14 项合成模型合同和 176 项菜单栏兼容结果，
  不将它们记作本轮重跑，也不据此宣称真实存储或原生验收通过。
  前轮已读取的需求、架构、模型合同及未变化标本边界继续适用；本轮额外核对了架构 §4 的 core 存储归属。
- Disposition：ASC-SET-R7-F1、ASC-SET-R7-F2、ASC-SET-R7-F3 CLOSED。
  ASC-SET-R1-F1、ASC-SET-R3-F1、ASC-SET-R5-F1 的历史关闭保持；未观察到修复使未确认默认、回执恢复或来源语义回退。
  Round 7 的 FAIL 和旧状态 FAILED 证据保持原样，不覆盖或重标记。
- WorkUnit：`auxiliary-session-classification:ux/settings.md`；
  ContentState：`auxiliary-session-classification:ux-settings-r8-8424e6bfac038d3b`。
- Completion gate：VERIFIED
- Gate scope：当前文档与 manifest 绑定标本的 document-integrity、independent-review。
  登记 1 个新 ContentState、2 条 pass Evidence 和 4 条关系，关系预检全部 ok；
  正式精确状态查询 VERIFIED 2/2，missing_criteria 为空，当前两条证据均 target_matches/applicable=true，
  malformed/invalidated/unresolved=false。查询保留的历史 Round 3 invalidated 条目不匹配当前目标，不影响本轮结果。
  CEv1 中该文档没有 contains 父节点；Beads 所属专题仍有未完成实现任务，因此不跨越 Topic 完成边界。
- Residual scope：原生 Settings 视图需要另行按新设计重做；本次未改变 `settings-control` 的 awaiting_commit、
  历史实现 PASS 或用户已有验收排除决定。浏览器结果不证明原生 VoiceOver、Dynamic Type、真实 CAS、磁盘持久化或 Widget 刷新。
- Handoff：Documents 的 Settings Review 勾选；交付建议仅覆盖文档任务，不包含并行原生实现。
  下一产品工作为按新版长列表设计重做 `settings-control` 视图，仍需用户下达相应阶段指令。

Task checkpoint：`ad-auxiliary-session-classification-doc-ux-settings`；content_state `8424e6bfac038d3b24e513eac849df516421a3f7720af7d9c9ff30f54a03f007`；gate VERIFIED。
提交建议：单独授权提交本次 Settings 文档、共享原型及检查脚本、manifest/截图、本评审记录、prototype README 对应说明和 tasks.md 的文档状态块；逐 hunk 排除并行 Swift 实现及其任务记录。
推送建议：目标 origin/feature/auxiliary-session-classification；授权提交完成后，核对整个分支待推送提交范围和签名，再单独授权推送。当前未配置该分支的 upstream，未执行推送或远端检查。

## Round 9 — 2026-11-08

## 📋 Settings 默认分页设计评审

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

**ASC-SET-R9-F1 — P2 — OPEN**

默认分页卸载实时用量页时丢失 OTel 配置托管状态，破坏既有开启与关闭合同。

位置为 [窗口分页](../ux/settings.md#窗口分页)第 113 行和[影响](../ux/settings.md#影响)第 449–452 行，
共享标本的 `prototype/src/Settings.jsx:246,306` 与 `prototype/src/TelemetrySettings.jsx:101–104,111–136`。

- 行为风险。文档把工具栏分页确立为唯一产品布局，并承诺各组的行和合同继续成立。
  标本的页容器以当前页为 key，实时用量组又按当前页条件挂载。切走该页会销毁其本地 `managed`、`result` 和 `consent`。
  `otelEnabled` 保存在外层，返回后仍显示采集开启，但刚完成的托管写入及其状态已经丢失。
  用户随后关闭采集时无法走到移除 AgentDeck 托管配置的分支。该标本会把有外部副作用的状态生命周期错误传递给实现。
- 直接证据。在独立浏览器会话打开
  `?settings=1&auxSettings=normal&auxMenu=normal&tab=sessions&lang=en&theme=light`，
  进入 Live usage，点击采集开关，再点击 Write and turn on。
  此时 `enabled=true,result=written`。依次切到 General、Live usage 后，
  `enabled=true,result=null`。随后关闭采集得到 `enabled=false,result=null`，
  再次开启又出现 `consent=write`。
- 对照证据。重新载入同一合成入口，执行写入后直接关闭采集，结果为
  `enabled=false,result=removed`。跨页操作是两条路径唯一的行为差异。
  源码中 `managed` 从未随写入更新的 `stage.exporter` 重新初始化，因此返回页面后 `ownsBlock=false`。
- 归属。窗口分页在本轮成为默认产品路径，该问题属于本轮文档绑定标本的跨页生命周期。
  旧长列表及可选分页的历史 PASS 保留其原有内容身份。
- 💡 有界修复。为分页明确写入事实、配置所有权、未解决结果和确认流程的保留规则。
  将相关状态放到稳定的窗口或 App 所有者，或保留页实例，并补充“写入成功 → 切页往返 → 关闭采集”回归。
  同时用原有失败、恢复冲突和待确认场景核对跨页行为，确保一次切页不会清除仍需行动的结果。
  更新对应文档、检查脚本和 manifest 身份后提交独立复评。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 四页的名称、顺序、内容归属和默认入口已经确定，原生整窗验收包含逐页尺寸、控件着色与 sheet 行为。
- 当前 manifest 的 14 个源码或检查文件与 11 张截图均通过 SHA-256 核对。

### 📝 总结

- Reviewer：Codex，独立于本轮设计作者，未委派。
- Method：单评审者冷上下文审查，文档与前轮记录对照、当前工作树 CodeGraph 定位、
  未索引窗口容器的有界源码检查，以及独立浏览器中的最小跨页反例和无跨页对照。
  取得阻断反例后停止扩大产品验证。
- Scope：默认分页设计、整窗视觉与验收规则，以及 manifest 绑定共享标本中的受影响生命周期。
  本轮证据来自文档和合成浏览器。原生整窗、VoiceOver、真实 Codex 配置写入及持久化仍由相应实现任务验收。
- Reviewed state：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`，
  document blob `fd50942383f77109ade18e6eaf66b8f4c000673c`，
  prototype manifest SHA-256 `a959646c4b0aa5d5dd3c31032b966e17df72aead2b4c3508c57a1852d31f068d`。
- Fingerprint：`3352895629fef99d64183edee436177e7fdcaabb493b18dca1994ec5f51deefe`。
  沿用 SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence：25 个 manifest 条目哈希一致。浏览器复现与对照的输入、顺序和输出见 ASC-SET-R9-F1。
  作者的 484 项分页检查与 14 项模型检查保留为作者记录，本轮新增证据是其未覆盖的跨页状态反例。
  `python3 scripts/check-docs.py` exit 0，完成相关链接、专题文档集和评审结构检查。
  `make check-whitespace` 与 `git diff --check` 均未报告诊断，检查序列 exit 0。
- Disposition：ASC-SET-R9-F1 OPEN。Round 8 对 ASC-SET-R7-F1/F2/F3 的关闭保留。
  本轮 `AuxiliarySettings.jsx` 与 `auxiliarySettingsModel.js` 的哈希仍与上一候选一致，
  该保留只覆盖既有修复事实。
- WorkUnit：`auxiliary-session-classification:ux/settings.md`。
- ContentState：`auxiliary-session-classification:ux-settings-r9-3352895629fef99d`。
- Completion gate：FAILED
- Gate scope：document-integrity、independent-review。入口查询确认当前状态缺少这两项证据。
  已登记新 ContentState、1 条 pass 和 1 条 fail Evidence，新增 4 条关系且预检全部 ok。
  MCP 写入响应返回更新计数，另以稳定 ID 和完整 attributes_json 读回确认两条证据及 WorkUnit 更新共 3/3。
  精确状态查询返回 FAILED，两条当前证据均 target_matches/applicable=true，
  malformed/invalidated/unresolved=false。document-integrity 通过，independent-review 被本轮反例否定。
  查询中的 Round 3 历史 invalidated 条目不匹配当前目标。文档 WorkUnit 返回 in_progress，
  保留此前已交付提交及其证据，当前边界保持开放。
- Handoff：文档任务 `ad-auxiliary-session-classification-doc-ux-settings` 返回修复，
  Documents 的 Settings Review 保持未勾选。修复范围为 ASC-SET-R9-F1，
  完成后对新文档和标本进行独立复评。

## Repair candidate — 2026-11-08（Round 9 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-SET-R9-F1，未委派。
- 复现：按 Round 9 证据确认根因。`Settings.jsx` 的正文容器以当前页为 key，各组按当前页条件挂载，
  切页会卸载 `TelemetrySettings`，丢掉其本地 `managed`、`result`、`consent`；返回后 `ownsBlock=false`，关闭采集不走移除分支。
- ASC-SET-R9-F1 → repaired in candidate。原型四页常驻：去掉正文的随页 key，各组始终渲染，非当前页设 `hidden`
  （`settingsTabs.css` 显式隐藏，因为 `.settings-group` 自带 `display:flex`），切页后新页滚回顶部。
  [窗口分页](../ux/settings.md#窗口分页)写明切页只换显示、不重建页，写入事实、配置所有权、未解决结果和进行中的确认在往返后保留，
  关窗规则仍归各组自己的文档；原生映射要求页控制器常驻、不用随页变化的 `.id`，这些状态归 App 级 controller；
  [原生整窗验收](../ux/settings.md#原生整窗验收)增加切页往返回归。
- 回归保护：新增 `prototype/tools/check-settings-pane-lifecycle.js`，覆盖 Codex 标本四种情形：
  默认写入（同意区往返、写入后往返、关闭得到 `removed`、再开启重新要求同意）、写入被拒、恢复冲突、已有其他配置
  （冲突区往返、仅开启接收端后往返、关闭不移除用户配置）；每次往返另断言只显示当前页、切回的是同一组实例。
  `check-auxiliary-settings.js` 改为只统计未隐藏的组，方向键切页一项改断言实时用量显示、会话隐藏。
- 修复前复现：临时把实时用量组改回按当前页条件挂载，en/light 四种情形全部失败。默认写入情形依次失败于
  同意区保留、写入结果保留和「关闭得到 removed」，与 Round 9 的 `result=null` 一致。之后恢复修复版 `Settings.jsx`
  （SHA-256 `d30210b35f4a3ded3d8f30b94cd654e61d0ffb87e24396926e413e121b29c2a5`），全量复跑通过。
- 验证（本候选内容）：切页生命周期每组 43 项 × 中英深浅 4 组 = 172 项，0 失败；
  Round 9 原路径（Sessions 入口 en/light，写入并开启 → 通用 → 实时用量 → 关闭）依次为 `written`、`written`、`removed`，
  再开启出现写入同意区。`check-auxiliary-settings.js` 分页 4 组各 121 项、长列表 zh/dark 与 en/light 各 113 项，0 失败；
  菜单栏 176 项、模型合同 14 项 0 失败；`npm run build` 通过。各页窗口高度与修复前相同，视觉无变化，截图未重拍。
  `python3 scripts/check-docs.py` 通过，`bash scripts/check-topic-docs.sh` exit 0，`bash scripts/check-whitespace.sh` exit 0，
  `git diff --check` 无输出。未运行产品测试（无产品代码变更）。
- 原 FAIL/OPEN 与 Round 9 的 FAILED 门禁保留；本候选不自称 Review PASS 或 CEv1 VERIFIED。
  CEv1：本会话 Neo4j MCP 不可达，未查询、未写入，由复评在提供方可用时按下方身份绑定。未提交、未推送。
- Candidate identity：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`；ux/settings.md blob `ee49b63034a47a20b0e86775acf49863f926bf6c`（未提交）；
  manifest 阶段 `redesign-r1-tabs-default-repair-r9`，SHA-256 `0a3f7e5c58caaa0bfce8afb77c1b54ea5b4737b154c18f35bc5845bd987f934d`
  （上一份 `a959646c4b0aa5d5dd3c31032b966e17df72aead2b4c3508c57a1852d31f068d` 记入 previous_manifest_sha256），
  新增 TelemetrySettings.jsx 与 check-settings-pane-lifecycle.js 两个条目；
  Fingerprint `ff235d3e74eb41f828a46eca550d719b35b38f0b258baa1b8ee696592abc0320`，
  沿用 SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- 2026-11-09 重发修复指令后复核：无新评审轮次，候选未改动。重新计算 HEAD、文档 blob、manifest SHA-256 与 Fingerprint，
  均与上方一致；manifest 列出的 27 个文件与截图哈希无漂移；切页生命周期检查复跑 172 项 0 失败。未改原型、文档或产品代码。

## Round 10 — 2026-11-09

## 📋 Settings 分页生命周期修复复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。ASC-SET-R9-F1 已关闭，受影响路径未发现新阻断。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-SET-R9-F1 — CLOSED**。[窗口分页](../ux/settings.md#窗口分页)第 130–146 行明确跨页保留写入事实、
  配置所有权、未解决结果和确认流程，并要求原生页控制器常驻、业务状态由 App controller 持有。
  `Settings.jsx` 去掉正文随页变化的 key，四页常驻，非当前页通过 `hidden` 隐藏，
  `settingsTabs.css` 显式覆盖组容器的 flex 显示。`TelemetrySettings` 的 managed/result/consent 因此跨页保留。
- 独立浏览器按 Round 9 原入口执行“写入并开启 → 通用 → 实时用量 → 关闭采集”，
  结果依次为 `enabled=true,result=written`、`enabled=true,result=written`、`enabled=false,result=removed`。
  再次开启出现 `consent=write`。往返前后遥测组节点身份相同，隐藏时计算样式为 `display:none`，
  通用页仅显示 general 与 menubar 两组。
- 以当前 `check-settings-pane-lifecycle.js` 在 en/light 独立执行写入被拒、恢复冲突和已有其他配置三种场景，
  分别为 9、13、10 项检查，全部通过。检查包含未完成确认、拒绝结果、恢复冲突、配置预览和托管移除语义。
- zh/dark 的会话未知写入在切页往返后保持 `saveUnknown` 与同一 `fixture-request-1`，会话组节点身份相同。
  打开锁定 sheet 后，真实 Tab 与 Shift+Tab 均停留在可用的“完成”按钮，正文和工具栏均 inert。
  Escape 关闭 sheet 后焦点回到“管理…”，当前页仍为 sessions，正文及工具栏解除 inert，未知写入状态继续保留。

### 📝 总结

- Reviewer：Codex，沿用 Round 9 独立评审角色，未参与修复，未委派。
- Method：逐项处置既有发现，核对修复交接、工作树内容与 manifest，使用当前工作树 CodeGraph 定位，
  对未索引的检查脚本和受影响的 sheet 生命周期作有界源码检查，再独立执行原反例、三条补充路径与真实键盘操作。
- Scope：ASC-SET-R9-F1 及跨页显示、OTel 状态、会话未知写入和 sheet 焦点。
  证据来自文档和合成浏览器，原生页控制器、VoiceOver、真实配置写入与持久化属于 settings-control 的验收范围。
- Reviewed state：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`，
  document blob `ee49b63034a47a20b0e86775acf49863f926bf6c`，
  prototype manifest SHA-256 `0a3f7e5c58caaa0bfce8afb77c1b54ea5b4737b154c18f35bc5845bd987f934d`。
- Fingerprint：`ff235d3e74eb41f828a46eca550d719b35b38f0b258baa1b8ee696592abc0320`，
  沿用 SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence：manifest 的 16 个源码或检查文件和 11 张截图共 27 项哈希一致。
  独立交互结果见上。复用 Repair candidate 中相同候选的构建、172 项切页生命周期检查、
  484 项分页及 226 项长列表检查、176 项菜单栏检查和 14 项模型合同检查。
  本轮重新执行其中三种生命周期场景的 32 项检查，用于独立核验该修复的失败与确认路径。
  本轮状态同步后，`python3 scripts/check-docs.py`、`make check-whitespace` 和 `git diff --check`
  在 fail-fast 检查序列中全部通过。最终候选指纹及 27 项 manifest 条目与入口核验一致。
- Disposition：ASC-SET-R9-F1 CLOSED。ASC-SET-R1-F1、ASC-SET-R3-F1、ASC-SET-R5-F1
  以及 ASC-SET-R7-F1/F2/F3 的历史关闭保留。本轮对 AuxiliarySettings 的改动为根节点传入 hidden，
  状态模型和来源规则继续沿用既有内容，受影响的请求保留与 sheet 键盘行为已直接核验。
- WorkUnit：`auxiliary-session-classification:ux/settings.md`。
- ContentState：`auxiliary-session-classification:ux-settings-r10-ff235d3e74eb41f8`。
- Completion gate：VERIFIED
- Gate scope：document-integrity、independent-review，精确状态查询 VERIFIED 2/2。
  两条新 pass Evidence 与 WorkUnit 更新经稳定 ID 和完整 attributes_json 读回确认 3/3，
  4 条关系预检全部 ok，写入响应确认新增 4 条关系。
  当前两条证据均 target_matches/applicable=true、malformed/invalidated/unresolved=false，missing_criteria 为空。
  返回的 Round 3 历史 invalidated 条目不匹配当前目标。Round 9 的旧目标与失败证据继续保留。
  文档 WorkUnit 没有 contains 父节点或子节点，专题矩阵中任务 6–9 继续由各自实现与验收边界管理。
- Handoff：Documents 的 Settings Review 勾选，文档任务交到 awaiting_commit 并释放复评认领。
  下一实现任务为按已批准分页设计完成 settings-control。

Task checkpoint：`ad-auxiliary-session-classification-doc-ux-settings`，content_state `ff235d3e74eb41f828a46eca550d719b35b38f0b258baa1b8ee696592abc0320`，gate VERIFIED。
提交建议：单独授权提交已复评的 Settings 文档任务，纳入设计文档、共享原型及对应检查脚本、manifest 与新截图、评审记录、prototype README 和 tasks.md 的文档状态 hunks。
推送建议：目标 origin/feature/auxiliary-session-classification。先完成授权提交并核验整个分支的待推送提交范围与 SSH 签名，再单独授权推送。当前分支未配置 upstream。
