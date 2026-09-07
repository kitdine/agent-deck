---
status: active
topic: schema-version-signal
subject: ux/menubar-schema-signal.md
created: 2026-09-06
updated: 2026-09-07
---

# Review — ux/menubar-schema-signal.md

## Round 1 — 2026-09-06

- **Reviewed state**: HEAD `fb8c6fbfffb99d217d76dfd1ee0b54e4e21d628a`,
  文档 blob `5be703d667ac1a5ccba08061937ebbd506b0eae5`（未跟踪，`ux/` 目录整体
  untracked）。对照物：`requirements.md` blob
  `b0854d0057df1dfd5bd07f6cec57e5c523f7333f`、`tasks.md` blob
  `5f7223eb6dea68a65f6668bdfd948634cba06103`。
- **Reviewer**: Claude Code（主 agent，非独立冷上下文；见 Method 的局限声明）
- **Method**: 设计/契约类目标的维度评审——前提有效性、与当前实现的一致性、
  场景覆盖、决策完备性、内部矛盾、隐性耦合。逐条核对文档引用的 17 处源码坐标；
  运行项目自带的文档集审计 `scripts/check-topic-docs.sh`；对照四份历史 GUI
  `ux/*.md` 与 `prototype/README.md` 检验本文档的标本证据类别。未执行 Xcode
  构建或应用运行（评审阶段对生产代码只读，且本目标无实现）。
- **Scope**: `docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
  全文 531 行。不含 `architecture.md`（未写）、CLI 呈现、以及本条件之外的
  任何菜单栏状态。
- **局限**（2026-09-07 更正，见文末 Correction 小节）: 本轮评审在一个未参与
  起草的会话中进行，上下文对该文档是冷的，角色也是分离的——评审会话从未写过
  该文档或 `prototype/`。原记录称"评审者与起草者同为本会话主 agent"，那是
  错的。真实局限是另一条且较弱：起草与评审同为 Claude Code 运行时，而
  `AGENTS.md` 只要求冷上下文加分离角色，未要求换运行时。

📊 综合评分: 7/10

✅ 评审结论: FAIL

### 🔴 严重问题——必须修复

**[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:293-406]
R1-F1 — 本文档以手绘 ASCII 框图充当 GUI 标本，与项目既定的"原型即设计真相"
脱钩，且全文 0 处提及 `prototype/`。**

- **行为风险**: 三重。
  其一，**设计真相分叉**。`prototype/README.md:3` 声明该原型是"产品全部界面的
  唯一设计真相：菜单栏面板、小组件、设置窗口，以及 CLI 的逐字符输出"，并记录了
  它在 2026-08-20 从 topic 目录移到仓库根的理由——"在第二个 topic 下再放一份
  拷贝，等于制造第二个设计真相"。本文档改的正是菜单栏面板，却在原型之外另立了
  一套标本。项目四份历史 GUI 面文档无一例外把原型作为标本来源
  （`archive/topics/desktop-app/ux/menubar.md:20-23`、`ux/widget.md:14-15`、
  `ux/settings.md:113-115`、`archive/topics/work-signals/ux/session-work-signals.md:15-19`），
  其中 work-signals 那份写明"Where this document and the prototype disagree,
  the prototype is right"。本文档没有这一条款，也就没有冲突时的裁定规则。
  其二，**该进原型的状态没进去**。原型有 `?surface=states` 状态一览页，现存
  `normal|empty|aged|partial|unavailable` 五档舞台态（`prototype/src/States.jsx`，
  `README.md` 记为"空数据 / 过期 / 部分不可用 / 完全不可用四种降级"），这一页
  存在的目的正是并列比较降级态。本设计的 S1 是一个新的组合降级态——抑制
  `partial` 与四条 consequence warning、新增一条 error 通知、面板体/footer/
  popover 三处新文案——它不在那一页里。同理，Copy 表的 7 个新 key 也不在原型的
  i18n 字典中，而 `work-signals` 的 ux 文档明确把文案表定位为"approved
  prototype dictionary"的增量。
  其三，**把原型能当场证伪的问题推给了真机**。文档自述"The footer is the only
  element with a truncation risk … It is specified to fit at 280 pt"
  （:397-406），随后把它列入 manual checklist。等宽 ASCII 框图判定不了
  `不可用 · 应用版本过旧` 在 280 pt 真实字体下是否被 `.truncationMode(.middle)`
  吃掉；原型能——它已有 `?measure=1` 溢出量具（"列出所有被裁切的容器"）与
  `?contract=1` 断言，且 README 记录过"peak 的日期第一版就是这样 DOM 里有、
  屏幕上没有"的同类缺陷。当前写法把一个可在标本阶段判定的结论降级成了
  实现后的人工观察。
- **证据**:
  - `rg -n -i 'prototype|原型' docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
    → 无匹配。
  - `git ls-files prototype/` → 23 个受跟踪文件；`prototype/src/States.jsx`
    存在；`prototype/README.md` 第 3 行即"唯一设计真相"声明。
  - `rg -n -i 'prototype' docs/archive/topics/*/ux/*.md` → 四份历史 GUI 文档
    各自在开头声明原型来源，`session-work-signals.md:19` 写明原型优先。
  - `docs/documentation-workflow.md:126-131` 记录了同一缺口的既往代价：
    "An independent UI audit of the menu-bar design scored it zero on
    aesthetics because no prototype existed, which under that tool's own rules
    forced a redesign verdict its total contradicted. The score was invalid,
    and the document was also genuinely missing the evidence class the question
    needed."
  - 文档 Scope 段"adds no new geometry and no new visual vocabulary"属实
    （复用 `noticeRow`、可展开 health row、既有 tab mark、既有 badge），但
    复用几何不等于该状态已存在于设计真相中——新的是状态组合与文案，不是几何。
- 💡 **有界补救**（一轮内可完成，四项）:
  1. 在文档开头加入与四份历史 GUI 文档同形的标本来源声明：指向
     `/prototype/`、给出打开本条件的 URL、并写入"与原型冲突时以原型为准"条款。
  2. 在 `prototype/src/States.jsx` 增加本条件的舞台态（建议 `state=schema`），
     使 `?surface=states` 能并列渲染 S1，并按 D3/S2/S4 覆盖"仅此一项问题"与
     "叠加其他问题"两种排布；Copy 表的 7 个 key 进入原型 i18n 字典的中英两套。
  3. 用 `?measure=1` 在 280 pt 下判定 footer 字符串是否被裁，把结论写回
     :397-406，替换掉当前的"specified to fit"；若确实截断，就地采用文档已经
     写好的 fallback，而不是留给真机。
  4. 保留现有 ASCII 标本作为文中内联对照（S0/S1 的层级与顺序它表达得清楚），
     但标注它索引的是原型的哪一个 URL——`menubar.md:336` 用的正是这个办法
     （"They are an index to the prototype, not …"）。

### 🟡 建议改进——推荐

**[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:145-153]
R1-F2 — D7 要求菜单栏图标在本条件下加 badge，但没有指定这个谓词落在哪一层，
而当前 badge 的判据与本条件的来源分属两套生命周期。**

- **行为风险**: `menuBarBadged` 定义为 `presentation.isBadged`
  （`MenuBarViewModel.swift:1315`），而 `isBadged` 是
  `AgentDeckShared/EmbeddedHelperRunner.swift:861-863` 的 public 属性，取值为
  `qualifiers.contains(.offline) || qualifiers.contains(.failing)`——由 helper
  的刷新状态派生。本条件则来自快照内容（`health.checks[].code`）。要让 D7 成立，
  实现者必须二选一：改 Shared 层的 `isBadged`（影响该属性的其他消费者，且把
  快照内容语义压进一个刷新状态类型），或只在 App 层扩展 `menuBarBadged`
  （Shared 与 App 从此对"是否 badged"给出不同答案）。文档对 accessibility
  label 指定到了排序位置（"ranked after `badgedOffline` and `badgedFailing`"），
  对 badge 本身却只在 Data requirements 第 5 行写"same condition predicate as
  row 1"，没有回答归属。这是实现者必须自行发明的契约。
- **证据**: `rg -n 'menuBarBadged|isBadged' apps/macos/ | rg -v Tests` →
  四处，调用链为 `MenuBarItemController.swift:53` → `menuBarBadged` →
  `presentation.isBadged`（Shared）。本条件下 helper 正常、快照正常，
  `qualifiers` 不含 `.offline`/`.failing`，故今天 `isBadged` 为 false——
  D7 关于"今天图标与 *Icon only* 偏好字节等同"的论断因此成立，但也正说明
  badge 需要一个新的判据来源。
- 💡 **有界改进**: 在 D7 补一句归属决策——badge 谓词加在 App 层的
  `menuBarBadged` 上、`presentation.isBadged` 不变，并说明理由（`isBadged`
  描述的是刷新状态，不是快照内容）；或把这一项列入"Open questions for
  `architecture.md`"的第 5 条交给契约阶段裁定。两种写法都可接受，未写不可。

### 🟢 优点

- **实现一致性极高**。文档引用的 17 处源码坐标逐条核对全部命中，包括
  `NoticeStripView`（`MenuBarSurfaceView.swift:414`）、`HealthDetailView`
  （:467）、`isExpanded = true`（:509）、复制按钮块（:555-570）、`FooterView`
  （:573）与 `lineLimit(1)`/`.truncationMode(.middle)`（:586-589）、
  `panelTabs`（`MenuBarViewModel.swift:452`）、`footer`（:1148）、
  `healthDetail`（:1262）、`menuBarText`（:1296）、`menuBarBadged` 与
  `menuBarAccessibilityLabel`（:1315-1321）、`DesktopHealthCheckV1`
  （`DesktopWire.swift:941-953`，确为单个 `count`）、`WidgetDesktopSnapshotV1`
  （`WidgetSnapshot.swift:3-19`，确实不解码 `health`/`provider`）、
  `exclamationmark.triangle.fill`（`MenuBarItemController.swift:76`）、
  `t(_:_:)`（`DesktopCopy.swift:315-317`，确经
  `String(format:locale:arguments:)`）、`doctor.Check`（`doctor.go:23-29`）、
  `desktop.HealthCheck`（`desktop.go:214-220`）。设计类文档最常见的失效模式
  ——断言不存在的字段或行为——在这里一处都没有。
- **D2 的 consequence 清单经得起反向检验**。`internal/desktop/desktop.go`
  共 14 处 `result.warn(...)`，其中 `work_signals_unavailable`（:573-605）与
  `provider_candidates_unavailable`（:305）都在 `store.OpenReadOnly` 成功分支
  内，本条件下不可达；`state_close_failed`（:261）同理；`sessions_*` 与
  `health_unavailable` 走独立路径。文档列出的四条抑制项与"不抑制"的三条
  独立事实，与代码可达性一致，measured starting state 的三条 warning 也对得上。
- **D5 的论证方式值得沿用**。它不是"觉得复制按钮不好看"，而是从
  `recovery_command` 是 shell 字符串、永不翻译这一事实推出"英文句子落进中文
  UI"，再推出该字段对本检查必须缺席——一个判断同时约束了呈现与线上契约，
  并写进了 Data requirements 的拒绝行。
- **文案被一条已存在的测试反向约束**。`DesktopCopyTests.testNoStringOffersAnUpdateCheck`
  （`DesktopCopyTests.swift:33-48`）的 11 个禁用短语逐字核对无误，7 条新文案
  全部避开；文档还说清了"命名恢复方式不等于执行更新"这条与 v0.5.0 撤回更新
  检查的边界关系。
- **D8 的三条理由按权重排序，且第一条是硬事实**，不是偏好；并写明了复议条件
  （"Revisit if the widget projection ever carries health for another reason"）。
- **S5 的"无粘滞"是一条真正的状态契约**——没有 dismissal 记录、没有"最近已恢复"
  文案，下一份不含该检查的快照即完全复原。降级态设计最容易漏的就是这一条。

### 📝 小结

被评审内容为 `docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
（HEAD `fb8c6fb` + blob `5be703d6`），531 行，stage 3 的表面框架与数据需求清单。

结论为 FAIL，唯一的严重问题是标本证据类别：文档在推理、坐标核对、状态覆盖
与文案约束四个方面都做得扎实，但它为一个 GUI 表面另立了一套 ASCII 标本，
而项目已有一份自称"唯一设计真相"的原型，四份历史 GUI 文档全部以它为准。
这不是格式偏好——`docs/documentation-workflow.md:126-131` 记录过缺少这一
证据类别导致独立 UI 审计给出无效评分的先例，而本文档自己把一个原型可当场
判定的截断风险（280 pt footer）推给了实现后的人工观察。第二条为建议级：
D7 的 badge 谓词跨越了 Shared 与 App 两层而未指定归属。

残留不确定性有两项。其一（2026-09-07 更正）：原文称评审者与起草者是同一个
会话主 agent、不满足独立冷上下文要求，这是错的；两者是不同会话，评审上下文
对该文档是冷的。其二，未执行 Xcode 构建与应用运行，所有关于呈现的判断都停留
在标本层——这正是 R1-F1 要恢复的那一层。

- **Findings**: R1-F1（🔴，开放）、R1-F2（🟡，开放）。两条均指向本次评审目标
  自身的缺陷，按项目规则以 FAIL 退回修复，不设外部 carrier。
- **Evidence**:
  - `bash scripts/check-topic-docs.sh` → 仅报
    `schema-version-signal: architecture.md is declared in the Documents matrix
    but not written`，即 `tasks.md` 声明的预期缺口；本文档在矩阵中、在磁盘上、
    且被 `requirements.md` 按路径命名，未被审计报为 gap。
  - `git hash-object` 三份文档 blob，见 Reviewed state。
  - 17 处源码坐标核对（命令与结果见 🟢 第一条）。
  - `rg -n 'result\.warn\(' internal/desktop/desktop.go` → 14 处，可达性分析
    见 🟢 第二条。
  - `git ls-files prototype/` → 23 个受跟踪文件。
- **完成门禁**: NOT_VERIFIED。CEv1 查询
  （`MATCH (n:CEv1Node) WHERE n.work_unit_id CONTAINS 'schema-version-signal'`）
  返回的唯一 WorkUnit 是 `schema-version-signal:requirements.md`
  （`unit_kind: document`，`status: complete`，绑定 commit 态
  `urn:ce:agent-deck:state:commit:0fa9eef8…`）。本文档的 document 边界
  `schema-version-signal:ux/menubar-schema-signal.md` 尚未建立，因为该边界在
  评审 `Verdict: PASS` 时才跨越。本轮为 FAIL，故不记录 pass 证据，也不创建
  WorkUnit。

## Round 2 — 2026-09-06（修复轮）

- **Reviewed state**: Round 1 两条 finding 的修复。HEAD 仍为
  `fb8c6fbfffb99d217d76dfd1ee0b54e4e21d628a`。评审目标文档由 blob
  `5be703d667ac1a5ccba08061937ebbd506b0eae5` 变为
  `bab2f89c3a65e64ed639e26addf4cb50bda3b10b`（仍未跟踪）。R1-F1 的主体在原型，
  被改动的标本文件按 `sha256`：
  - `prototype/README.md` ->
    `deb9d91069cf94a0d15dba3b857b263756556e0e062b87d75dcaa3604183bc48`
  - `prototype/src/App.jsx` ->
    `4bbc6ce42e53e5aa8cf7281f9e4ebf216f92a8af5fef19d96d44dd6d2c141fd5`
  - `prototype/src/Popover.jsx` ->
    `4be0ad4a18b852ba7ce8aecd50eb80377e172b635cf2bb547a0e525054bff215`
  - `prototype/src/Stage.jsx` ->
    `20b61dd141b3431b84054f5664f7fbb22271ec546e4c9367600c506abb6e1764`
  - `prototype/src/States.jsx` ->
    `a7e7e0e087eb0057f001f79b3913a6cc520dadde4710abedba8313e5c78ad91a`
  - `prototype/src/data.js` ->
    `26bf462574cb050ea8229417091ecdf92e8ac5b6466ba2158af1ba39c00a3943`
  - `prototype/src/i18n.js` ->
    `4e2783dab846704285a401a6ee7cb1fd8b688ce2d6258729e3c63f3bc3740963`
  - `prototype/src/measure.js` ->
    `29d61f676490934dbefe712324b10ce0fdf67466cd78864f641f9c01f393d449`
  - `prototype/src/styles.css` ->
    `576a28b85a8f6aca8f3359b13f3872d6c37f612f7cea1534464cb3cc37aa03c0`
- **Reviewer**: claude-code（Round 1 FAIL 的修复轮。本轮不下评审结论、不关任何
  门禁、不授权提交；`Review` 单元格仍需一次独立复评才能勾选）
- **Scope**: R1-F1 与 R1-F2。改动限于该 UX 文档与 `prototype/`。未改动任何产品
  代码、测试、配置或线上契约；Go 与 Swift 源码一行未动。

### Round 1 findings，逐条处置

- **R1-F1**（🔴 ASCII 框图充当 GUI 标本，与"原型即设计真相"脱钩）->
  **已修复**，按 finding 给出的四项有界补救逐项落地。
  1. **标本来源声明**。文档新增 `## Where the specimens come from` 一节，指向
     `/prototype/`、给出本条件六个 URL、并写入"与原型冲突时以原型为准"条款——
     与 `session-work-signals.md:19` 同形。Scope 一节也补上了原型这一条目。
  2. **该进原型的状态进去了**。原型新增两个舞台态：`state=schema`（仅此一项
     问题）与 `state=schemaStacked`（叠加助手连不上与另一项检查没过），
     `?surface=states` 现在并列渲染八种降级态。Copy 表的 7 个 key 以同名同值
     进入 `src/i18n.js` 的中英两套字典，两处必须同改的约束写进了 Copy 表下方。
     实测起点的三项检查（`state_permissions`/`state_lock`/`database` 带
     `unknown_schema` 与两个版本号）以 `HEALTH_SCHEMA` 进入 `data.js`。
     把这一态先接到原型既有的整屏 `.unavailable` 处理上是错的，已改正：那是
     "连快照都没有"的处理，而 D6 说的是标记仍只是指针、解释在面板体里。四个
     面板体现在各自渲染归因后的不可用行（`.domain-missing`，与原型处理单个数据
     域缺失时同一套），节律块在本条件下不渲染。若不改，写进文档的"冲突以原型
     为准"会反过来悄悄推翻 D6。
  3. **280 pt 的截断问题由原型当场判定，不再推给真机**。这需要原型先具备两样
     它没有的东西，都已补上：`width=420|280` 舞台开关（280 对应实现的
     `AGENTDECK_TEST_WIDTH`），以及量具对 ellipsis 截断的检测——README 原本
     就记录了"溢出量具看不见一行文字被 ellipsis 截断"这一盲区，`measure.js`
     现在会逐个量出面板里所有 `text-overflow: ellipsis` 元素并报
     `TRUNCATED @<宽度>pt … short by <n>px`。结论写回文档 :397-406 处，替换掉
     原来的"specified to fit"：两种语言都不截断，`zh-Hans` 余 80.33 px、
     `en` 余 37.14 px。
  4. **ASCII 标本保留并标注索引**。每段框图上方注明它索引原型的哪一个 URL，
     并写明"是原型的索引而非替代，冲突以原型为准"——用的正是 `menubar.md:336`
     的办法。原来分列的 S4 与 S2 合并为一段 `schemaStacked` 标本（三行排布一次
     看全），各自的单独排布作为对照保留在其下。
- **R1-F2**（🟡 D7 未指定 badge 谓词的归属层）-> **已修复**，采用 finding 提供
  的第一种写法而非推给 `architecture.md`。D7 新增一段归属决策：谓词加在 App 层
  的 `menuBarBadged`，`presentation.isBadged` 不变，理由是后者是
  `qualifiers.contains(.offline) || .failing`——描述刷新状态，而本条件下 helper
  完全正常，条件来自快照内容 `health.checks[].code`。把内容语义压进刷新状态属性
  会改变该属性对 Shared 层所有其他消费者的答案。同时在 Open questions 末尾写明
  这一项已不是待决问题，除非 `architecture.md` 把该条件放到 Shared 层已经看得见
  的位置。

### 修复过程中发现、但不在本次范围内的三项

三项都不是本次改动引入的，也都不指向本评审目标，按项目规则不就地修。它们此前
无人看得见，是因为原型此前既没有 280 pt 宽度也没有 ellipsis 检测——本次为
R1-F1 补上这两样，才让它们显形。每一条都有 Beads carrier：

- **原型的 stat chip 在窄边界报出真机不会有的截断，且 peak 取值与实现不同形**
  -> **open**，carrier `ad-bug-prototype-statchip-fidelity`。
  `?state=normal&width=280&measure=1` 报
  `TRUNCATED @280pt strong | "15–17 时" | short by 11px`（`en` 为
  `"15–17h" | short by 3px`），见于 `normal/empty/aged/partial/pending` 五态。
  核对实现后这**不是**产品缺陷：`StatChipRow` 三行都带
  `minimumScaleFactor(0.7/0.72)`，是缩放不截断
  （`MenuBarPanelViews.swift:129-160`）；且 `DesktopFormat.hourWindow` 渲染的是
  `15:00-16:00` 这样的 1 小时窗口（`DesktopCopy.swift:417-425`），而原型的
  `formatHourRangeShort` 渲染 2 小时窗口的 `15–17 时`
  （`prototype/src/i18n.js:520-522`）。这是标本对实现的两处不忠实，方向是假阳性。
- **未归因的 footer 路由文案在 280 pt `en` 下截断 2 px** -> **open**，carrier
  `ad-bug-footer-routes-narrow-truncation`。
  `"Codex aigocode · Claude official" | short by 2px`。与上一条不同，footer 的
  实现**没有**缩放兜底——`routesText` 只有 `lineLimit(1)` 与
  `.truncationMode(.middle)`（`MenuBarSurfaceView.swift:586-589`），超长时真的会
  从中间截断。但原型是 CSS grid、实现是 `HStack` 加 `Spacer`，且 2 px 余量远小于
  两套字体度量之差，所以这是一条待真机核实的信号而非已成立的结论。
  它同时反过来支持本设计：本条件替换进去的归因短语比今天 footer 承载的字符串更短。
- **原型 `?probe=1` 有一条恒假断言，使 ALL PASS 信号长期为红** -> **open**，
  carrier `ad-bug-prototype-probe-pending-assertion`。
  `FAIL 详情里标注了待采集`（49 项中的一项）。已用 HEAD 的原型副本在 4176 端口
  独立复跑确认为既存失败：基线同样 49 项、同一条失败。根因是该断言检查
  `.pending-banner`（`probe.js:111`），而它只在 `pending` 态渲染，探针却跑在默认的
  `normal` 态上。这是自检自身的缺陷，且是最坏的一种——一条恒假断言与一条真实回归
  长得一样，下一个真实回归会被它盖住。

### Verification

按 L0（文档）＋标本层定向检查选取，未触及 Go/Swift 代码故不涉及 L1 以上：

- `npx vite build` -> 成功，4581 modules，无告警。
- 量具全扫两轮。其一按状态：`state × {normal,empty,aged,partial,pending,
  unavailable,schema,schemaStacked} × lang{zh,en} × width{420,280}` 共 32 组；
  其二在 D6 改正后按面板：两个 schema 态 × `tab{usage,breakdown,attribution,
  sessions}` × lang{zh,en} × width{420,280} 共 32 组。两轮里 schema 与
  schemaStacked 全部 `NO OVERFLOW`；其余报告项为上文两条范围外既存截断。
- 量具的变异验证：把 footer 字符串换成一条明显放不下的句子，量具报
  `TRUNCATED @280pt strong … short by 226px`，随后复原。`NO OVERFLOW` 因此是一条
  能失败的断言，不是恒真式。
- footer 字符串的直接测量（`Range.getBoundingClientRect` 取文字固有宽度，而非
  被约束后的 `scrollWidth`）：`zh-Hans` 列宽 189 px / 文字 108.67 px；
  `en` 列宽 173 px / 文字 135.86 px。
- DOM 断言：S1 提示条恰为一条且为因由行；四个 tab 全部标记；面板体、footer、
  服务商弹层三处均为归因文案，面板体是卡片内的一行而非整屏接管，弹层不再列候选；健康详情 `database` 行展开为
  两行散文、无等宽、`.card` 内 0 个按钮；菜单栏项 `badged` 且 aria-label 为
  `AgentDeck — 版本比数据库旧`，标题为空。`schemaStacked` 提示条顺序为
  `无法连接本地助手` → 因由 → `2 项检查未通过`。
- `?surface=widgets&contract=1` 中英各一轮 -> 13/13 `ALL PASS`。
- `?probe=1` 中英各一轮 -> 49 项、1 条既存失败，与 HEAD 基线逐项一致。
- `bash scripts/check-topic-docs.sh` -> 仅报 `architecture.md is declared in the
  Documents matrix but not written`，即 `tasks.md` 声明的预期缺口，与 Round 1
  相同。
- `make check-whitespace` -> 退出 0；`git diff --check` -> 无输出。
- 相对链接逐条解析：`../../../../prototype/`、
  `../../../../prototype/README.md`、`../requirements.md`，以及 README 反向链接
  `../docs/topics/schema-version-signal/ux/menubar-schema-signal.md`，全部命中。

### 残留

- 本轮为修复轮，不产生评审结论。**完成门禁**：未查询、未记录——文档边界
  `schema-version-signal:ux/menubar-schema-signal.md` 在评审 `PASS` 时才跨越，
  Round 1 的 `NOT_VERIFIED` 结论不因修复而改变。
- Round 1 的独立性局限依旧成立：本轮修复者与 Round 1 评审者、起草者是同一个
  会话主 agent。拟判 PASS 的复评应由独立冷上下文的评审者执行。

## Round 2 — 2026-09-06

- **Reviewed state**: HEAD `fb8c6fbfffb99d217d76dfd1ee0b54e4e21d628a`，
  文档 blob `bab2f89c3a65e64ed639e26addf4cb50bda3b10b`（Round 1 时为
  `5be703d6`，531 行 → 654 行），并新增标本载体
  `prototype/` 的工作区改动指纹
  `sha256:9d5faab6b2206d5e09be75d304e6f2390d0394dced5ba2fda6cf059f58487217`
  （`git diff prototype/`，9 个文件 +293/−47）。本轮的评审对象是文档与它所
  索引的原型标本两者，因为 R1-F1 的修复把标本移出了文档。
- **Reviewer**: Claude Code（主 agent；独立性局限见下）
- **Method**: 按 Round 1 的两条发现逐条复核，并对修复引入的原型改动做回归
  检查。关键结论全部由主 agent 亲自实跑取得，未采信文档自述的数字：
  在工作区起 dev server（4175），用 `agent-browser` 打开量具 URL 读取输出，
  并另行用 `Range.getBoundingClientRect()` 独立测量 footer 元素；再从
  `git archive HEAD` 导出一份干净的 HEAD 版原型起第二个 server（4176）
  建立回归基线。两个 server 已在取证后关闭。
- **Scope**: `docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
  全文 654 行，以及 `prototype/` 的 9 个被改文件。
- **独立性**（2026-09-07 更正，见文末 Correction 小节）: 修复由另一个
  Claude Code 会话完成（Beads 评论 2026-09-07 05:49 与 06:03），复评由本会话
  完成（05:21 的 Round 1、06:11 的 Round 2）。本会话从未写过该文档或
  `prototype/` 的任何一行，故对被复评内容是冷上下文，角色分离，满足
  `AGENTS.md` 的独立性要求。原记录称"评审者与修复者同为本会话主 agent"，
  那是错的。仍需说明的是：本轮的说服力主要来自可复现的实测命令与双 server
  差分，独立性是它的必要条件而非充分条件。

📊 综合评分: 9/10

✅ 复评结论: PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点（含已闭合的既往发现）

**R1-F1 闭合 —— 且闭合方式超出了补救清单的要求。** Round 1 提的四项逐条兑现：

1. **标本来源声明**（文档 :25-52 新增 `Where the specimens come from` 一节）。
   指向 `/prototype/`、给出六条 URL、并写入原文条款
   "**Where this document and the prototype disagree, the prototype is right.**"
   与 `archive/topics/work-signals/ux/session-work-signals.md:19` 同形。
   Scope 段也补了一行"the specimen for all of the above in `prototype/`,
   which this document indexes rather than duplicates"。
2. **状态进入原型**。`prototype/src/States.jsx` 增加 `schema` 与
   `schemaStacked` 两个舞台态，中英因由文案齐全；`i18n.js` 的 `states`
   小节从"六种数据状态"改为"八种"。实测 `?surface=states`：`.state-frame`
   计数为 **8**，页面文本含"应用比数据库旧"与"叠加其他问题"，副标题为
   "同一份界面在八种数据状态下的样子"。Copy 表的 7 个 key 也进了
   `i18n.js` 的中英两套，且原型注释写明了它与实现侧 `DesktopCopy.allKeys`
   同名、以及 `testNoStringOffersAnUpdateCheck` 对恢复措辞的约束——把两条
   跨仓约束写在了会被改动的那一侧。
3. **截断问题回到标本阶段判定，实测复现。** `measure.js` 新增第二段，
   遍历面板内 `text-overflow: ellipsis` 的元素比较 `scrollWidth/clientWidth`。
   主 agent 实跑结果与文档表格**逐位吻合**：

   | 语言 | 量具输出 | client | text | headroom |
   | --- | --- | --- | --- | --- |
   | `zh-Hans` @280pt | `NO OVERFLOW` | 189 px | 108.67 px | **80.33 px** |
   | `en` @280pt | `NO OVERFLOW` | 173 px | 135.86 px | **37.14 px** |

   420 pt 两语同样 `NO OVERFLOW`。
4. **ASCII 标本降级为索引**。文档 :340-345 明确"they are an index to the
   prototype, not a substitute for it"，并逐个标注所索引的 URL，与
   `archive/topics/desktop-app/ux/menubar.md:336` 的既有办法一致。

超出清单的两项，都指向"断言要能失败"这一点：

- **变异验证被写进了文档并可复现。** 主 agent 独立注入一条超长 footer 串，
  量具从 `over=0` 变为 `over=291px`；文档自述用它自己的串得到
  `TRUNCATED @280pt strong … short by 226px`。字符串不同故数值不同，方向
  一致——`NO OVERFLOW` 是可证伪的断言而非恒真。
- **与今日 footer 字符串的对照，实测确认。** 文档称现行
  `Codex aigocode · Claude official` 在 280 pt `en` 下截断 2 px，实测
  `scroll=175 / client=173`，**恰为 2 px**。归因后的字符串比今天的更短，
  这一行不因归因而变紧。

**R1-F2 闭合。** D7 新增一段（文档 :195-208）明确"The predicate belongs to
the App layer"：`menuBarBadged`（`MenuBarViewModel.swift:1315`）承载本条件，
`presentation.isBadged`（`AgentDeckShared/EmbeddedHelperRunner.swift:861-863`）
不变，理由是 `isBadged` 陈述的是 helper 的刷新状态而本条件陈述的是快照内容，
把内容语义推进刷新状态属性会改变该属性给 Shared 层其他消费者的答案。它还
写明了接受的代价——Shared 与 App 从此对"是否 badged"给出不同答案——并说明
这是正确的切分。"Open questions for `architecture.md`"结尾也补了一句，声明
badge 谓词的层级**不是**待决问题。两处坐标本轮重新核对无误。

**呈现规则的实测吻合。** `?state=schema&width=420&lang=zh` 下通知条只有因由
一行，`partial`、计数通知与四条 consequence warning 全部不在——与 D2/D3 一致；
footer 为 `不可用 · 应用版本过旧`。`?state=schemaStacked` 下顺序为
`无法连接本地助手` → 因由 → `2 项检查未通过`，与 S2/S4 的排布规则一致。

**修复没有回归共享标本。** 原型是四个 topic 共用的资产，本次改了 9 个文件，
故复跑了另外两项自检：`?surface=widgets&contract=1` 报 `ALL PASS`（13 项
断言），`?surface=widgets&measure=1` 报 `NO OVERFLOW`。

### 📝 小结

**逐条处置**：R1-F1（🔴）**闭合**，四项补救全部兑现，并额外补上了变异验证与
现状对照；R1-F2（🟡）**闭合**，D7 明确了谓词归属、理由与代价。本轮无新增
阻塞发现。

被复评内容为 `ux/menubar-schema-signal.md`（HEAD `fb8c6fb` + blob
`bab2f89c`）及其标本载体 `prototype/`（diff 指纹 `sha256:9d5faab6…`）。
判 PASS 的理由不是文档声称它测过了，而是主 agent 在两个 server 上实跑复现了
它的每一个关键数字：80.33 / 37.14 的余量、`NO OVERFLOW`、变异后的 291 px、
以及今日字符串的 2 px 截断。Round 1 的核心指控是"把可测量的降级成了断言"，
本轮的验收标准因此只能是"我自己测一遍"。

**残留不确定性**两项。其一（2026-09-07 更正）：原文称评审者与修复者仍是同一
会话主 agent，这是错的——修复与复评分属两个 Claude Code 会话，复评会话对被评
内容是冷的。真实的残留项是较弱的一条：两个会话同属一个运行时与一套项目指令，
故共享同样的盲区，可复现的实测命令是对这一点的对冲。其二，标本settles
的边界未变——真实 SF 字体度量、Dynamic Type 与 VoiceOver 仍在人工清单上，
文档已把 footer 一项从"开放问题"改写为"确认项"，措辞恰当。

**一项范围外发现，已带 carrier**：

- **R2-F1（🟡，范围外，先存缺陷）** — `prototype/src/probe.js:111` 的断言
  `check("详情里标注了待采集", !!$(".pending-banner"))` 已过期，导致原型交互
  自检稳定输出 `1 FAILED`，而 `prototype/README.md` 文档化的正常输出是
  `ALL PASS`。**双 server 差分证明它先于本次修复存在**：工作区（4175）与
  `git archive HEAD fb8c6fb` 的干净导出（4176）报同一条失败。根因是提交
  `151c6d3 feat(prototype): separate pending work-signal capture from
  unavailable` 让 `normal` 态渲染真实工作信号数据、不再显示待采集横幅，而
  probe 仍在默认 `normal` 态下断言横幅存在——实测确认 `state=pending` 下
  横幅出现、`state=normal` 下不出现，即界面正确、断言过期。按
  `.agent-instructions/review-records.md` 的分类，这指向被评审变更**之外**
  的先存条件，故不阻塞本轮 PASS。
  **Carrier**: Beads `ad-bug-prototype-probe-pending-banner`（P2，`open`）。
  Lane 由用户裁定；该 issue 描述中提议 Lane A 并给出理由，未自行指派。

- **Evidence**（全部由主 agent 实跑，命令可复现）:
  - `cd prototype && npm run dev -- --port 4175 --strictPort`；
    `agent-browser open 'http://127.0.0.1:4175/?state=schema&width=280&measure=1&lang=zh'`
    → 量具 `<pre>` 文本 `NO OVERFLOW`；`lang=en` 同。
  - footer 独立测量（不经量具）:
    `document.querySelectorAll(".popover *")` 中 `textOverflow==="ellipsis"`
    的唯一元素，`Range.getBoundingClientRect()` →
    zh `{client:189, textWidth:108.67, headroom:80.33}`、
    en `{client:173, textWidth:135.86, headroom:37.14}`。
  - 变异验证 → 注入超长串后 `{over:291, scroll:464, client:173}`；
    注入 `Codex aigocode · Claude official` → `{over:2, scroll:175, client:173}`。
  - `?state=schema&width=420` 两语 → `NO OVERFLOW`。
  - `?state=schema&width=420&lang=zh` 通知条 → 仅
    `此 AgentDeck 比其数据库旧，无法读取本地数据` 一行。
  - `?state=schemaStacked&width=420&lang=zh` 通知条 → 三行，顺序为
    offline、因由、计数。
  - `?surface=states` → `.state-frame` 计数 8，含两个新态。
  - `?surface=widgets&contract=1` → `ALL PASS`；
    `?surface=widgets&measure=1` → `NO OVERFLOW`。
  - 回归基线：`git archive HEAD prototype` 导出至 scratchpad，
    `npm run dev -- --port 4176`，`?probe=1` → `1 FAILED`，
    与工作区同一条断言，证明 R2-F1 先存。
  - `state=pending` → `{banner:true, detail:true}`；
    `state=normal` → `{banner:false, detail:true}`。
- **完成门禁**: 见下方 Round 2 门禁同步小节。

### Round 2 门禁同步

**完成门禁: VERIFIED**（`document` 边界，绑定候选内容态）。

- WorkUnit `schema-version-signal:ux/menubar-schema-signal.md`
  （`urn:ce:agent-deck:work-unit:schema-version-signal-ux-menubar-schema-signal`，
  `unit_kind: document`，`status: complete`）。
- 内容态
  `urn:ce:agent-deck:state:candidate:d164bcd9d2d7fc5d25bf01606383086a23b1da57f85564a93a7b3d58bd88b4ef`，
  配方
  `printf '%s' 'head=<HEAD>;document=<git hash-object of ux/menubar-schema-signal.md>;prototype=<sha256 of git diff prototype/>' | shasum -a 256`。
  它比既往的文档边界多带一个 `prototype` 分量，因为 R1-F1 之后标本已不在
  文档内——只对文档取指纹会让"标本仍在原型里且仍是这一版"这件事不可证。
- 两条 required criterion，均由本轮证据绑定为 `outcome: pass` /
  `status: VERIFIED`，记录后已复查门禁：
  - `independent-review-pass` — 评审通过且所有已记录发现闭合。
  - `specimen-measured-in-prototype` — 标本存在于仓库原型而非文档自带的第二
    份设计真相，且窄边界截断问题由标本阶段的测量回答而非推迟到真机。这一条
    是本文档特有的，因为 Round 1 正是在这里失败。
- 本内容态为**候选态**，非不可变提交态。若后续获得提交授权，需按
  `.agent-instructions/evidence.md` 对不可变 Git tree 重记一次证据并
  `supersedes` 本候选态证据——`requirements.md` 的
  `f28036a6` → `0fa9eef8` 就是这条路径的既有先例。


## Correction — 2026-09-07

Round 1 与 Round 2 的局限段原本都声称评审者与起草者/修复者是"同一会话主
agent"，因此不满足独立性要求。**这是事实错误，本节更正它，并保留错误本身
作为审计线索。**

**事实**。三方分属不同会话，Beads `ad-svs-doc-ux-menubar-schema-signal-design`
的评论时间线是证据：

| 时间 | 会话 | 动作 |
| --- | --- | --- |
| 2026-09-06 08:39 | A | 起草文档，记录 D8 与 status 行 |
| 2026-09-07 05:21 | B（本会话） | Round 1 评审，判 FAIL |
| 2026-09-07 05:49、06:03 | C | 修复 R1-F1 与 R1-F2 |
| 2026-09-07 06:11、06:16 | B（本会话） | Round 2 复评判 PASS，交付收尾 |

本会话在整个过程中从未写过 `ux/menubar-schema-signal.md` 或 `prototype/`
的任何一行——它只写评审记录、`tasks.md`、`docs/status.md`、Beads 与 CEv1。
故它对被评审内容是冷上下文，角色分离，符合 `AGENTS.md` 的
"Review independence requires a cold context and a separate role"。

**误判是怎么产生的，因为它会重犯**。两个原因叠加：

1. **Beads 的 actor 抹掉了会话边界。** wrapper 要求每次调用带
   `BEADS_ACTOR`，Claude Code 一律写 `claude-code`。于是评论时间线上四条
   记录出自三个会话，署名却完全相同。看署名推会话身份，必然推错。
2. **SessionStart 注入的历史观测读起来像自己的行为。** 会话启动时注入的
   claude-mem 观测里有"设计 schema-version-signal 菜单栏 UX surface"这类
   条目，它记录的是过去会话做过的事，但它出现在本会话上下文的开头，位置与
   本会话自己的历史无异。

同一误判也出现在会话 C 的 Beads 评论里："the repairer, the Round 1 reviewer
and the drafter are the same session agent"。三者实为三个会话。两个会话
独立犯了同一个错，说明这是上述两条结构性成因导致的，不是一次偶然口误。

**这次更正把独立性判断改成了更强还是更弱？更强。** 原记录声称的独立性比
实际更差，即错误方向是自我贬低而非自我抬高，因此 Round 2 的 PASS 结论不受
影响——它在更差的假设下已经成立。更正后仍成立，且不再需要"以实测代替独立性"
这一让步。

**仍然真实的局限**，措辞比原文弱但确实存在：三个会话同属 Claude Code 运行时
并读同一套项目指令，因此共享同样的盲区。跨运行时的独立性（例如由 Codex 复评）
本仓库并未要求，此处仅作记录。

## Round 3 — 2026-09-07

- **Reviewed state**: HEAD `7ef6e50852126c312d84c8e46e33973a2633bf32`，文档 blob
  `3113c6f151b63a0c9af32dab3ee41200642fa695`（Round 2 时为 `bab2f89c`，654 行 →
  847 行），并按 Round 2 建立的配方带上标本载体指纹
  `sha256:d2d65842de9e6db5277b3828427972662c408ea9e6806ae172b1c9eb525d6dae`
  （`git diff prototype/`，3 个文件 +26/−11）。合成候选内容态
  `9f8305a71df8db4b49d708cbec38769427a4ded9745e80950cc1fa393b963fa0`。
  本轮评审对象仍是文档与它索引的原型标本两者。
- **Reviewer**: Claude Code（本会话，对本文档与 `prototype/` 冷上下文——本会话
  未写过其中任何一行）
- **Method**: 目标类别为 design/contract。按该类别的维度逐条核对：前提有效性、
  与现行实现的一致性、场景覆盖、决策完备性、内部矛盾。所有结论均由本会话亲自
  实跑或读源取得，未采信文档自述：`git show 8d283cd:<file>` 逐条核对 32 处源码
  坐标；起 dev server（4177）用 `agent-browser` 跑量具、探针、健康详情与
  `Range.getBoundingClientRect()` 独立测量 footer；跑项目自带的
  `scripts/check-topic-docs.sh` 与 `scripts/check-whitespace.sh`。server 与浏览器
  已在取证后关闭。
- **Scope**: `docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
  全文 847 行，以及它索引的 `prototype/` 三个被改文件。未构建 Xcode、未运行应用，
  故所有关于真机排版的判断仍停留在标本层——这与文档 **Verification** 的手工清单
  边界一致。
- **本轮为何存在**: `tasks.md` 记载 stage 7 与 stage 8 之间不排评审，D9 要到
  `tasks.md` 自己的评审才第一次见到独立评审者。用户对本文档直接下了 `评审`
  指令，故本轮就是那次独立评审，评审对象是 stage 7 之后的当前内容态整体，不限于
  D9。

📊 综合评分: 6/10

✅ 评审结论: FAIL

### 🔴 严重问题——必须修复

[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:150-152]
**UX-R3-F1（🔴，开放）— D2 抑制了 `sessions_unavailable`，而它不是本条件的后果。**

- **行为风险**: 一个同时满足"本条件成立"与"会话索引确实损坏"的用户，通知条里
  没有任何一行提到会话索引；主通知把不可用归因给 schema，并给出"升级 AgentDeck"
  这一唯一处置。升级之后会话依然缺失，而这件事从未在任何界面上被说出来。这正是
  本 topic 要消除的静默失败形状，被抑制规则重新引入了一次。
- **证据**:
  - `internal/desktop/desktop.go:252-263` — `OpenReadOnly` 失败时无条件发出
    `provider_unavailable` 与 `usage_unavailable`；这两个确实是后果。
  - `internal/desktop/desktop.go:480-497` — `loadSessions` 不在该分支内，无条件
    运行；它经 `store.OpenSessionsReadOnly`（`internal/store/store.go:73-91`）打开
    独立的 `sessions.sqlite3`，只检查 `session_sources`、`session_metadata` 两张表
    是否存在，从不读核心库的 `schema_metadata.version`。故 `sessions_unavailable`
    与 `schema_ahead` 相互独立。
  - 本文档 **Measured starting state**（:105-107）自带反证：*"On the reporter's
    machine `sessions.sqlite3` was a separate, readable store, so
    `sessions_unavailable` was absent"*。
  - `requirements.md:246` 把该库划在边界外：*"It is a separate store, was
    unaffected"*。
  - **内部矛盾**：D2 自己以"independent facts"为由保留 `sessions_close_failed`，
    而那一行出自同一个 `loadSessions`（`:489`）、同一个独立库。同一函数的两个码
    被分到相反的两侧，判据不成立。
- **本轮为何现在才提**: stage 7 对 `provider_candidates_unavailable` 用了
  "定位生产者再判可达性"这一检验并因此删掉了一个成员。该检验用在第三个码上给出
  的是相反结论，而它没有被用上。
💡 **有界修复**: 对 `sessions_unavailable` 施加与 stage 7 相同的定位生产者检验，
二选一：(a) 把它移出 D2 的抑制名单，名单剩两个码，同步改 **Data requirements**
中"通知条抑制（D2）"一行的措辞与 S0/S1 通知条标本受影响的部分；或 (b) 保留抑制，
但按 D2 为 provider 一例已有的写法，明确写出理由与它承认的代价，并说明为何与
`sessions_close_failed` 分处两侧不矛盾。契约文本里同源的那句已另有载体，见下方
"归属其他目标的一项"。

[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:334-350, :681-689, :711-719]
**UX-R3-F2（🔴，开放）— "复用既有的可展开形状"与"改动是加法、只花一个调用点"
与 `HealthCheckRowView` 的实际结构不符，且 D9 的行在该结构下根本展不开。**

- **行为风险**: stage 8 从 **What existing behavior this changes** 这一节给改动
  定尺寸。按现状写法定出的尺寸下，最省事的实现是把因由/处置散文塞进
  `HealthCheckRow.recovery` 以够到既有的可展开分支——那会同时带回等宽字体与
  *复制修复命令* 按钮，即 D5 明确拒绝的"在中文界面里给出一句英文的复制按钮"；
  另一条路则是 `hook_deliveries` 行按 `else` 分支只渲染一行 `hook_deliveries 警告`
  而没有任何因由行，正是 D9 声明要消除的那个形状。
- **证据**:
  - `MenuBarSurfaceView.swift:512` — `if let recovery = row.recovery,
    !recovery.isEmpty` 是披露区（chevron 加缩进区）的唯一入口；`:535-537` 的
    `else` 分支只渲染 `statusRow`，没有 chevron、没有缩进区。
  - `:528-532` — 展开后的内容恰恰只有 `recoveryRow(recovery)`；`:555-570` 的
    `recoveryRow` 是 `.monospaced()` 的 `Text` 加 *Copy recovery command* 按钮。
  - 本 surface 的两行都没有 `recovery_command`：`database` 由 C6 拒绝提供，
    `hook_deliveries` 由 C5 明确 "No `recovery_command`"。因此按"既有形状"两行都
    展不开，S4 标本（:622、:636）画出的 `⌄` 与缩进因由行无法由该形状产生。
  - D9 "The line re-uses the disclosure area D4 already opens for `database`" 与
    结构不符：每个 check 各自是一个 `HealthCheckRowView` 实例（`:497` 的
    `ForEach`），披露区在各自的 `VStack` 内，不存在跨行复用。
  - 无障碍一条（:685-687）称因由行与处置行 "inside the row's combined element"。
    `.accessibilityElement(children: .combine)` 只加在 `statusRow` 上（`:552`），
    `recoveryRow` 是 `VStack` 里的兄弟节点，在该元素之外。文档陈述的朗读顺序是
    一项待做的改动，不是现状。
  - 于是 :715-719 的 *"The `HealthCheckRow` extension D4 and D9 both need is
    additive … Adding the counts costs one call site"* 少算了视图侧的改动。
    该段对测试的判断本身是对的：`MenuBarViewModelTests.swift:488-495` 的
    `rows.count == 3` 与各行 `status`/`recovery` 断言确实不读 count，
    `WireFixture.failingHealth`（`AppTestFixtures.swift:469-477`）确实是
    `schema/ok`、`usage/usage_stale`、`prices/prices_missing`，三条已复核为真。
💡 **有界修复**: 在 **Health detail row** 与 **What existing behavior this changes**
两节写出 `HealthCheckRowView` 需要的两处改动——披露条件从"有 recovery"改为
"有因由或有处置"，以及新增一条非等宽、无按钮的散文展开分支——并把无障碍那一条
改写成"合并元素需要扩展到覆盖这两行"。同时把"只花一个调用点"换成实际的改动清单。
不改动任何呈现决策，只改动改动面的陈述。

### 🟡 建议改进——推荐

[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:296-300]
**UX-R3-F3（🟡，开放）— 行模型的扩展只写了 `count`，而标本按 `code` 选行、按
`supported_count` 取数。**

- **证据**: 原型是本文档声明的设计真相，它按 code 判定：`Popover.jsx:841`
  以 `check.code === "schema_ahead" || check.code === "hook_deliveries_dropped"`
  决定是否 `expanded`，`:848`/`:856` 按同样的码选文案，`:850` 读
  `check.supported_count`。而 `healthDetail`（`MenuBarViewModel.swift:1262-1274`）
  今天三者都不读——这一点本文档在 **Measured starting state** 事实 1 已自述。
  文档只写"两行都需要行模型上的 `count`"，未写 `code` 与 `supported_count` 怎么
  到达行；且 `hook_deliveries_dropped` 这个码在全文 847 行中一次未出现，于是 D9
  的行按什么谓词被选中（check 名还是稳定码）是未定的，而 D5 已立下"文案按稳定码
  取"的规则。
- **行为风险**: 实现者需要自行发明一个契约，且最可能按 check 名匹配，与 D5 的规则
  以及 C1"码才是契约、名不是"的立论相反。
💡 **有界改进**: 在 D9 的"两行都需要 `count`"一条里补齐行模型实际要多带的东西
（稳定码、`supported_count`），并写明两行各按哪个码匹配，与标本一致。

[docs/topics/schema-version-signal/ux/menubar-schema-signal.md:752-756]
**UX-R3-F4（🟡，开放）— **Verification** 称量具在"all six combinations"上报
`NO OVERFLOW`，而同句枚举出的是八种。**

- **证据**: 句中枚举为两个状态 × 两个宽度 × 两种语言 = 8。本轮实跑八种全部为
  `overflow:0`（命令与结果见 **Evidence**），故结论方向无误，只有覆盖面的计数写少了。
- **行为风险**: 这是一条证据陈述。计数与它所描述的枚举对不上，会让后来的读者
  无法判断哪两种没跑，也无法复现同一次取证。
💡 **有界改进**: 改成八种，或写明实际跑了哪六种。

### 🟢 优点

- **32 处源码坐标全部核对为真。** 用 `git show 8d283cd:<file> | sed -n '<range>p'`
  逐条比对，无一处漂移，包括最细的两处：`statusRow` 的 `Text(row.name)` 确实落在
  `MenuBarSurfaceView.swift:547`，`isExpanded = true` 确实落在 `:509`。基准 commit
  `8d283cd`（`docs: close out v0.5.0 stable release`）存在且被逐条引用。
- **标本与文档逐字一致，本轮亲自复核。** S4 健康详情两种语言均与 :617-645 的标本
  逐字相同（`state_permissions` / `state_lock` / `hook_deliveries` 带计数行 /
  `database` 带两个数字与处置行）；S2+S4 通知条顺序为
  `无法连接本地助手` → 因由 → `2 项检查未通过`，与 :585-597 相同；`state=schema`
  下计数通知缺席、健康详情三行，与 D3 在 `problems == 1` 的要求一致；
  `?surface=states` 计到 8 个 `.state-frame`。
- **窄边界的判断是测量出来的，且本轮独立重测吻合。** 用
  `Range.getBoundingClientRect()` 另行测得 `zh` 列宽 189.00 px / 文本 108.67 px，
  `en` 列宽 172.55 px / 文本 135.86 px，与文档 :516-521 的表一致（`en` 列宽文档取
  173，故余量写作 37.14，实测 36.69，差在取整内，方向不变）。文档"既有路由文案
  在 280 pt `en` 下超出约 2 px"一说亦复现：`Codex aigocode · Claude official`
  文本 174.86 px 对列宽 172.55 px，超出 2.31 px。
- **探针结论如实。** `?probe=1` 实跑 49 条，恰好 `1 FAILED`，失败项是
  `详情里标注了待采集`，与文档 :769-772 的陈述一致，且已由
  `ad-bug-prototype-probe-pending-banner` 与
  `ad-bug-prototype-probe-pending-assertion` 承载；该轮三条健康详情断言
  （二级页面 / 不挤占 footer / 能返回）均 PASS，也与文档一致。
- **八条文案与原型字典逐字相同**（`prototype/src/i18n.js:147-154`、`:374-381`），
  两种语言都在；且经检索无一条触碰
  `DesktopCopyTests.testNoStringOffersAnUpdateCheck` 的十一个禁用词——该断言的
  词表已按 `DesktopCopyTests.swift:35-37` 核对为真。
- **stage 7 的三项吸收都落到了实处**，其中 `provider_candidates_unavailable`
  的删除给出了定位到 `internal/desktop/desktop.go:305` 与 `loadProvider` 的
  `else` 分支的可达性论证，本轮复核该论证成立。UX-R3-F1 指出的正是这套方法没有
  用在第三个码上。
- **D7 对 App 层与 Shared 层的切分论证成立。** `isBadged`
  （`EmbeddedHelperRunner.swift:861-863`）确为
  `qualifiers.contains(.offline) || qualifiers.contains(.failing)`，是刷新态语义；
  本条件是快照内容语义，两者确实是不同的问题。
- **D8 的三条理由逐条为真。** `WidgetDesktopSnapshotV1`
  （`WidgetSnapshot.swift:3-19`）确实只解码五个键、无 `health`；
  `UnavailableWidget`（`WidgetViews.swift:1272-1285`）确实已渲染
  `Data unavailable`。

### 归属其他目标的一项（不阻塞本目标）

`architecture.md:485-491` 把 `sessions_unavailable` 与另两个码并列，称
"reproduces the producer's behavior exactly"，而那句自己已让步为 "when the
separate session store is **also** unreadable"。该前提正是 D2 名单的依据，但它属于
另一份已 PASS 并已提交的文档。载体：Beads `ad-bug-arch-sessions-unavailable-not-a-consequence`
（`relates-to` 挂在 `ad-svs-doc-ux-menubar-schema-signal-design` 上）。本目标的
UX-R3-F1 独立成立，不依赖该条先关闭。

### 📝 小结

被评审内容为 HEAD `7ef6e50` 上的未提交文档 blob `3113c6f1` 加原型工作区指纹
`d2d65842`，即 stage 7 契约吸收之后的当前内容态。结论为 **FAIL**，四条发现全部
开放：两条严重（D2 抑制了一个独立事实；行结构与改动面陈述与实现不符），两条建议
（行模型扩展只写了 `count`；一处证据计数与枚举不符）。按项目规则，四条均指向本
目标自身，无一条可留到 PASS 之后。

**残留不确定性**，如实说明：本轮未构建 Xcode、未运行应用，因此关于真机 SF 字体
度量、Dynamic Type 与 VoiceOver 的一切仍未验证——这与文档 **Verification** 手工
清单所划的边界相同，不因本轮而缩小。UX-R3-F2 关于无障碍朗读顺序的判断基于源码
结构而非真机 VoiceOver 实测。另需说明的是，本会话与起草会话同属 Claude Code 运行时
并读同一套项目指令，故共享同样的盲区；本仓库未要求跨运行时独立性。

**整体评价**：文档的呈现决策本身是可信的——D1 到 D9 每条都带落地的理由，标本与
文档逐字一致且经本轮独立重测，八条文案与既有测试约束吻合，32 处坐标无一漂移。
失分集中在两处"文档对既有代码的断言"：一处把一个独立事实当成了后果，一处把一次
需要改视图的改动写成了加法。两者都不需要重做设计，改的是陈述与一个名单成员。

- **Findings**: UX-R3-F1（🔴，开放）、UX-R3-F2（🔴，开放）、UX-R3-F3（🟡，开放）、
  UX-R3-F4（🟡，开放）。四条均指向本目标自身，按项目规则以 FAIL 退回修复，不设
  外部 carrier。归属 `architecture.md` 的一项已给出 Beads 载体，见上。
- **Evidence**:
  - `git rev-parse HEAD` → `7ef6e50852126c312d84c8e46e33973a2633bf32`；
    `git hash-object` 文档 → `3113c6f1…`；`git diff prototype/ | shasum -a 256`
    → `d2d65842…`；合成候选态 `9f8305a7…`（配方同 Round 2）。
  - `bash scripts/check-topic-docs.sh` → exit 0，无输出。
    `bash scripts/check-whitespace.sh` → exit 0，无输出。本仓库未提供针对
    design/contract 文档内容的检查器，故内容层判断由人工核对承担，此处如实记录。
  - `git show 8d283cd:<file> | sed -n '<range>p'` × 32 处坐标，全部命中。
  - `npm run dev -- --port 4177`（取证后已关闭）加 `agent-browser`：
    - 八种量具组合 `?state=schema|schemaStacked & lang=zh|en & width=280|420
      & measure=1` → `document.title` 均为 `overflow:0`。
    - `?state=schemaStacked&lang=zh&width=420` 通知条 → 三行，顺序为
      offline、因由、计数；`lang=en` 同序。
    - 两种语言的健康详情 → 四行，`hook_deliveries` 带计数因由行、`database` 带
      两个数字与处置行，与 S4 标本逐字相同。
    - `?state=schema&lang=zh&width=420` → 通知条一行，健康详情三行。
    - `?surface=states` → `.state-frame` 计数 8。
    - `?probe=1` → 49 条断言，`1 FAILED`，失败项 `详情里标注了待采集`。
    - `Range.getBoundingClientRect()` 独立测量 footer，四组数值见 🟢 第三条。
  - `rg -n 'schemaSignal' prototype/src/i18n.js` → 八键 × 两语言，与 **Copy** 表
    逐字一致；对 `testNoStringOffersAnUpdateCheck` 的十一个禁用词检索无命中。
- **完成门禁**: **NOT_VERIFIED**。CEv1 查询
  （`MATCH (n) WHERE n.work_unit_id CONTAINS 'schema-version-signal'`）显示
  WorkUnit `schema-version-signal:ux/menubar-schema-signal.md`
  （`urn:ce:agent-deck:work-unit:schema-version-signal-ux-menubar-schema-signal`）
  的两条 required criterion 只有绑定 `candidate:d164bcd9…`（Round 2）与
  `commit:6479df9c…` 的 `outcome: pass` 证据。本轮的目标内容态
  `candidate:9f8305a7…` 上没有任何证据节点，故该内容态的门禁为 NOT_VERIFIED。
  本轮为 FAIL，故不记录 pass 证据、不新建节点、不改写既有证据。

### 下一步指令

修复：schema-version-signal / ux/menubar-schema-signal.md — UX-R3-F1、UX-R3-F2、
UX-R3-F3、UX-R3-F4 四条全部为必做项。

### Round 3 修复落实 — 2026-09-07

本节记录 Codex 按用户明确授权修复 UX-R3-F1 至 UX-R3-F4，不新增评审轮次，
不改变 Round 3 的 FAIL 结论。四条均已在候选中修复，最终关闭由独立复评裁定。

- **UX-R3-F1 -> repaired in candidate**：读取 `loadSessions`、
  `OpenSessionsReadOnly` 与核心库失败分支，确认会话库独立。D2 只抑制
  `provider_unavailable` 和 `usage_unavailable`，保留 `sessions_unavailable`；
  同步通知条规则、Data requirements、S0/S1 标本和未来测试要求。原型增加
  `sessions=unavailable` 开关，两种 schema 状态都可覆盖会话库独立不可读；
  中英文使用现行 `warningSessionsUnavailable` 文案。架构文档的另一条发现仍由
  `ad-bug-arch-sessions-unavailable-not-a-consequence` 承载，本轮未修改它。
- **UX-R3-F2 -> repaired in candidate**：核对 `HealthCheckRowView` 的唯一
  `recovery` 披露入口、等宽复制分支与 `statusRow` 的局部无障碍合并。文档明确
  需将展开条件扩为有因由、有恢复散文或有恢复命令，新增非等宽、无复制按钮的
  散文分支，各 check 独立展开；保留原命令分支。无障碍合并范围必须扩展并保留
  可操作的展开控件。改动面清单包含模型投影、文案派生、视图与无障碍，删除
  “只花一个调用点”的定尺寸误导。
- **UX-R3-F3 -> repaired in candidate**：D9 明确投影 `code`、`count`、
  `supported_count`；D4 匹配 `schema_ahead`，D9 匹配
  `hook_deliveries_dropped`，不按 check 名或本地化状态匹配，缺失数字不补零。
- **UX-R3-F4 -> repaired in candidate**：原枚举为两状态 × 两宽度 × 两语言，
  更正为八种；本轮重跑这八种，再加入会话库不可读变体，合计十六种。

**候选内容态**：HEAD `36e4b0e87f0ac0eda603e022afd52db26517a043`，文档 blob
`3628a50c658801426cbdb0ee6015424e885322a7`；`git diff --no-ext-diff prototype/`
的 SHA-256 为 `8ea5d0655bd5b832cfba65c2795b0c5ac9f7013e8f3e11ce52eae611a5ddacc0`。
沿用既有配方 `head=<HEAD>;document=<blob>;prototype=<diff SHA-256>`，候选 digest
为 `1c301867d18263647e80a556af7ef14da72807e27cdd7053c7862535afd6e21e`。

**本轮验证**：L0 文档检查加受影响原型行为检查。`npm run build -- --outDir
/private/tmp/svs-r3-build` 在 `prototype/` 下通过（Vite 6.4.2，4581 modules）。
`npm run dev -- --port 4179 --strictPort` 加隔离 session 的 `agent-browser`：
`state=schema|schemaStacked × lang=zh|en × width=280|420 ×
sessions=readable|unavailable`、每组 `measure=1`，十六组标题均为 `overflow:0`。
逐组读取 `.notices .notice`：可读时 S1 一行、叠加态三行；不可读时分别二行与四行，
会话警告均最后出现，前面仍为离线（若有）、schema 原因、计数（若有）。
完整截图确认窄英文会话警告与页脚可见；点击原因通知仍打开健康详情，Hook 计数
137、数据库 999/23、恢复散文均存在，复制按钮为 0。

`bash scripts/check-topic-docs.sh`、`make check-whitespace`、`git diff --check`
通过。Go/Swift 产品代码未修改，无 Xcode 或真机 VoiceOver 验证；F2 的展开与
无障碍改动是明确的后续实现要求，原型测量不能替代它。已有 probe 失败未重跑，
也未声称本轮消除了它。临时构建、截图、浏览器与服务器在取证后清理。

**证据边界**：本候选的 document gate 保持 NOT_VERIFIED，独立复评尚未发生。
仅记录本轮 `specimen-measured-in-prototype` 测量证据，不写
`independent-review-pass`，不覆盖旧候选或旧提交的事实。固定 gate 模板不自带
目标态筛选，本轮给 evidence MATCH 加精确 `target_content_state` 绑定后使用其
原有失效与依赖判定；同时核对 `observed_at` 关系，不把旧态 PASS 当成本候选证据。
`tasks.md` 保持 Draft 已勾、Review 未勾；Beads 移交 `in_review`，不提交、不推送。

## Round 4 — 2026-09-07（复评）

- **Reviewed state**: HEAD `36e4b0e87f0ac0eda603e022afd52db26517a043`，文档 blob
  `3628a50c658801426cbdb0ee6015424e885322a7`（Round 3 时为 `3113c6f1`，
  847 行 → 924 行），标本载体指纹
  `sha256:8ea5d0655bd5b832cfba65c2795b0c5ac9f7013e8f3e11ce52eae611a5ddacc0`
  （`git diff prototype/`，3 个文件 +32/−11）。合成候选态
  `1c301867d18263647e80a556af7ef14da72807e27cdd7053c7862535afd6e21e`，本会话按
  既有配方独立复算，与修复轮记录的值一致。
- **Reviewer**: Claude Code（本会话；对文档与 `prototype/` 冷上下文——本会话从未
  写过其中任何一行，只写评审记录、`tasks.md`、`docs/status.md`、Beads 与 CEv1）
- **Method**: 逐条复核 UX-R3-F1 至 UX-R3-F4 在新内容态下的处置，并对修复引入的
  改动做回归检查。所有结论由本会话亲自实跑或读源取得，未采信修复轮自述的数字：
  `git show 8d283cd:<file>` 复核新增与改写的源码坐标；`git diff --stat 8d283cd
  HEAD` 确认被引用的 Swift/Go 源在基准与 HEAD 之间无差异，故坐标在两处都成立；
  起 dev server（4178）用隔离 session 的 `agent-browser` 跑十六组量具、八组通知条、
  健康详情、状态板与探针，并用 `Range.getBoundingClientRect()` 复测 footer。
  server 与浏览器已在取证后关闭。
- **Scope**: `docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
  全文 924 行，以及它索引的 `prototype/` 三个被改文件，外加为核对开关可发现性
  而读的 `prototype/README.md`。未构建 Xcode、未运行应用。
- **HEAD 变动说明**: HEAD 由 `7ef6e50` 前进到 `36e4b0e`
  （`fix(workflow): align review records and hook state parsing`），该提交只动
  `.agent-instructions/`、`AGENTS.md`、`docs/documentation-workflow.md` 与两个
  hook 脚本，不含本 topic 的任何文件；本 topic 的修复仍在工作区未提交。已复核
  `.agent-instructions/review-records.md` 的新版本，其对本轮适用的记录形状、
  finding 处置矩阵与门禁独立性规则均未改变。

📊 综合评分: 8/10

✅ 复评结论: FAIL

### 🔴 严重问题——必须修复

无。UX-R3-F1 与 UX-R3-F2 两条严重发现均已闭合，见 🟢。

### 🟡 建议改进——推荐

[prototype/src/i18n.js:143]
**UX-R4-F1（🟡，新增）— 标本字典的注释仍写"七条文案"，其下是八个键。**

- **处置**: 新增。由本候选把 `schemaSignalHookDropped` 加进该组而注释未同步造成，
  是本次变更自身的缺陷，不是既存条件。
- **证据**: `prototype/src/i18n.js:143` 写 `// schema-version-signal 的七条文案。`，
  其下 `:148-155` 为八个键（`schemaSignalNotice`、`schemaSignalCause`、
  `schemaSignalRecovery`、`schemaSignalSectionUnavailable`、`schemaSignalFooter`、
  `schemaSignalSwitchUnavailable`、`schemaSignalHookDropped`、
  `badgedSchemaSignal`）。该注释同句声明的不变量"与 Copy 表一一对应"仍然成立，
  被评审文档 :418 也已改为"eight keys"，只有这个计数是旧的。
- **行为风险**: 该注释存在的目的正是防止字典与 Copy 表漂移。一个说"七"而下面是
  八项的清点，会让后来的编辑以为多出一项，或在加第九项时继续按错误基数计数——
  即它声明要防止的那种漂移。与 UX-R3-F4 是同一类缺陷（计数与它所描述的枚举不符），
  按同一标准处理。
💡 **有界改进**: 把"七条"改为"八条"。

[prototype/README.md:41-44]
**UX-R4-F2（🟡，新增）— 修复新增的 `sessions=unavailable` URL 开关未进入原型
README 的参数清单。**

- **处置**: 新增。开关本身工作正常（本轮实测），缺的是它在标本载体自身索引里的
  登记。
- **证据**: `prototype/README.md:41-44` 是原型舞台的开关清单，逐项列出
  `lang`、`theme`、`state`、`width`、`tab`、`signal`、`settings`，其中
  `width=420|280` 正是本 topic 在框架轮加入并同步登记的先例。本次新增的
  `sessions=unavailable`（`prototype/src/Popover.jsx:801-804`）不在清单内；
  `git diff prototype/` 也确认 README 未被本次变更触及。
- **行为风险**: `prototype/README.md` 被本文档 :34-37 声明为"每个产品界面的唯一
  设计真相"，其他 topic 的作者从这份清单找开关。清单漏项使该开关只能从本 topic
  的文档里发现。此处还有一层混淆：清单里已有 `tab=usage|breakdown|attribution|sessions`，
  `sessions` 作为 `tab` 的取值出现，扫读者很容易看不出另有一个同名的 `sessions=` 参数。
💡 **有界改进**: 在 `README.md:41-44` 的参数清单里加上
  `sessions=readable|unavailable`（或等价写法），并在 `:46-49` 说明 schema 两态的
  那段里补一句它覆盖的是独立会话库不可读这一并存情形。

### 🟢 优点（含已闭合的既往发现）

**UX-R3-F1 -> 已闭合。** D2 现在只抑制 `provider_unavailable` 与
`usage_unavailable`（:152-159），并把 `sessions_unavailable` 与
`sessions_close_failed` 一起列入"独立事实、照常渲染"。新增的定位生产者论证
（:161-169）经本轮逐条复核成立：`internal/desktop/desktop.go:252-263` 确为
`OpenReadOnly` 失败分支且无条件发出那两个码；`loadSessions`（`:480-497`）确在该
分支之外；`OpenSessionsReadOnly`（`internal/store/store.go:73-91`）确实只检查
`session_sources`、`session_metadata` 两张表而不读核心库版本。文档同时补上了
一直缺席的那个变体：新的 **S1 with an independently unreadable session store**
一节（:499-517）给出两种语言的标本，并说明它"既不新增 doctor 检查也不改变
`health.problems`"——本轮核对 `Result.warn`（`internal/desktop/desktop.go:284-292`）
只写 `Partial` 与 `Warnings`，确实不触碰 health，该说法成立。文案复用既有
`DesktopCopy.warningSessionsUnavailable`（`DesktopCopy.swift:152`，已在
`allKeys` `:255` 内），中文 `无法读取会话数据` 经 `Localizable.xcstrings` 核对
一致，故未新增第九个键。抑制名单的计数在全文六处（:153、:344、:716、:783、:803、
:898）均已同步为两个码。

**UX-R3-F2 -> 已闭合。** **Health detail row**（:337-372）不再声称复用既有形状，
而是写明"今天唯一的披露谓词是非空 `row.recovery`（`:512`），展开内容只有
`recoveryRow(recovery)`（`:528-532`），D4 与 D9 都不带恢复命令"，并给出要改成
什么。三处坐标本轮全部复核为真。最关键的一句是新增的
"Do not put prose in `row.recovery`"——它正好堵住 Round 3 点名的那个失败模式
（把散文塞进 `recovery` 以够到既有分支，从而带回 D5 拒绝的复制按钮）。D9 的
"no disclosure area is shared across check rows"（:284-285）纠正了跨行复用披露区
的说法。无障碍一条（:733-738）改成如实陈述现状（"今天只有 `statusRow` 被合并
（`MenuBarSurfaceView.swift:552`），披露内容在它之外"，本轮核对 `:552` 确为
`.accessibilityElement(children: .combine)`）加一条明确的实现要求，并注明需真机核实。
**What existing behavior this changes** 的对应条目（:763-777）把"只花一个调用点"
换成了五项改动清单，并以"One initializer is not the size of the whole change"收尾。

**UX-R3-F3 -> 已闭合。** :307-317 现在写明 `HealthCheckRow` 三者皆无，
`healthDetail` 必须把 `check.code`、`check.count`、`check.supportedCount` 带进行模型
（并注明该字段的 wire 键是 `supported_count`），D4 匹配 `code == schema_ahead`、
D9 匹配 `code == hook_deliveries_dropped`，"Neither predicate matches `name` or
localized status"，缺失的数字不补零。这与标本的判定方式一致：
`prototype/src/Popover.jsx:845` 按这两个码决定 `expanded`，`:851` 读
`check.supported_count`。Data requirements 的 D9 行（:723）也从原来错误的
"C2's row-model extension carries it" 改成 "C2 supplies wire fields, not an
existing App row-model extension"。

**UX-R3-F4 -> 已闭合，且证据本身经本轮独立重跑。** :818-819 改为
"all eight combinations"。本轮实跑八组，全部 `overflow:0`；修复轮另行加测的
`sessions=unavailable` 变体本轮也全部重跑，十六组标题均为 `overflow:0`。

**修复未引入回归，逐项核对：**

- 通知条八组实测与文档逐字一致：可读会话时 S1 一行、叠加态三行；不可读时分别
  二行与四行，会话警告一律排在最后，中英文案分别为 `无法读取会话数据` 与
  `Session data could not be read`。
- 健康详情未受影响：窄边界英文叠加态下仍为四行，`hook_deliveries` 计数 137、
  `database` 999/23 与恢复散文俱在，健康详情内复制按钮计数为 0。
- `?surface=states` 仍为 8 帧；`?probe=1` 仍为 49 条断言、恰好 `1 FAILED`，
  失败项仍是既存的 `详情里标注了待采集`，未新增失败项。
- footer 复测 `zh` 列宽 189.00 px / 文本 108.67 px，与 Round 3 及文档表格一致，
  新增的会话警告行未挤压页脚。
- 被引用的 Swift/Go 源在 `8d283cd..HEAD` 之间无差异，故文档钉住的坐标在基准
  commit 与当前 HEAD 上同样成立。
- 文档结尾的 **Approval boundary**（:874-878）把已经过时的"stage 8 的评审者是
  D9 的第一位独立评审者"换成了如实记述：Round 3 判 FAIL、四条已在候选中修复、
  Review 单元格保持未勾、复评通过且证据边界满足后才回到 stage 8。
- 修复轮记录的候选摘要 `1c301867…` 本轮按同一配方独立复算一致；CEv1 里
  `content_state` 节点（`head=36e4b0e`）与 `specimen-measured-in-prototype` 的
  evidence 节点（`outcome: pass`）形状正确。

### 📝 小结

**逐条处置**：UX-R3-F1 已闭合；UX-R3-F2 已闭合；UX-R3-F3 已闭合；UX-R3-F4 已闭合。
新增 UX-R4-F1（🟡，开放）与 UX-R4-F2（🟡，开放），两条都在标本载体上，都由本次
变更自身造成。

**被复评内容**为 HEAD `36e4b0e` 上的未提交文档 blob `3628a50c` 加原型工作区指纹
`8ea5d0655`，合成候选态 `1c301867…`。**结论为 FAIL**——不是因为修复不足，四条
既往发现的修复都扎实且经独立实测复现，而是因为项目规则不允许把任何开放发现留到
PASS 之后，包括最低严重度的。两条新发现合起来是两行改动，且同属"标本载体的索引
没跟上标本本身"这一类，一轮即可闭合。

**残留不确定性**：未构建 Xcode、未运行应用，真机 SF 度量、Dynamic Type 与
VoiceOver 仍未验证——这与文档 **Verification** 手工清单划的边界相同，未因本轮
缩小。UX-R3-F2 的无障碍部分现在是一条明确的实现要求并注明需真机核实，本轮只能
核到它对现状的陈述为真，核不到改完之后的朗读顺序。本会话与起草、修复会话同属
Claude Code 运行时并读同一套项目指令，共享同样的盲区；本仓库未要求跨运行时独立性。

**整体评价**：这是一次高质量的修复轮。三条实质发现的修复都不是打补丁式的改写——
D2 换上了定位生产者的论证并补齐了一直缺席的会话变体标本，Health 行把"复用既有
形状"换成了带坐标的改动说明并主动堵住了评审点名的那个失败模式，行模型则把两个
匹配谓词、三个字段与"不补零"都写死了。修复轮自述的每一个数字本轮都复现。
剩下的两条是清单类遗漏，与设计判断无关。

- **Findings**: UX-R3-F1 -> closed、UX-R3-F2 -> closed、UX-R3-F3 -> closed、
  UX-R3-F4 -> closed；UX-R4-F1（🟡，开放）、UX-R4-F2（🟡，开放）。两条新发现均
  指向本目标自身，按项目规则以 FAIL 退回修复，不设外部 carrier。归属
  `architecture.md` 的既有一项仍由 Beads
  `ad-bug-arch-sessions-unavailable-not-a-consequence` 承载，本轮未变动。
- **Evidence**:
  - `git rev-parse HEAD` → `36e4b0e…`；`git hash-object` 文档 → `3628a50c…`；
    `git diff --no-ext-diff prototype/ | shasum -a 256` → `8ea5d065…`；
    合成候选态 `1c301867…`（与修复轮记录一致）。
  - `git diff --stat 8d283cd HEAD -- apps/macos internal/desktop internal/store
    internal/doctor` → 空，被引用源无差异。
  - `git show 8d283cd:<file> | sed -n '<range>p'` 复核 `:512`、`:528-532`、
    `:552`、`internal/desktop/desktop.go:252-263`、`:284-292`、`:480-497`、
    `internal/store/store.go:73-91`、`DesktopCopy.swift:152`、`:255`，全部命中。
  - `Localizable.xcstrings` 解析 → `Session data could not be read` 的 `zh-Hans`
    为 `无法读取会话数据`。
  - `npm run dev -- --port 4178`（取证后已关闭）加 `agent-browser`：
    - 十六组量具（两状态 × 两宽度 × 两语言 × 会话可读/不可读，每组 `measure=1`）
      → `document.title` 均为 `overflow:0`。
    - 八组通知条（两状态 × 两语言 × 会话可读/不可读，420 pt）→ 分别为 1/2 行与
      3/4 行，会话警告恒在末位，文案两种语言均正确。
    - `?state=schemaStacked&lang=en&width=280&sessions=unavailable` 进入健康详情
      → 四行，Hook 计数 137、数据库 999/23、恢复散文俱在，复制按钮 0 个。
    - `?surface=states` → `.state-frame` 计数 8。
    - `?probe=1` → 49 条断言，`1 FAILED`，失败项 `详情里标注了待采集`。
    - `Range.getBoundingClientRect()` 复测 footer `zh`/280 → 列宽 189.00 px、
      文本 108.67 px。
  - `bash scripts/check-topic-docs.sh` → exit 0，无输出；
    `bash scripts/check-whitespace.sh` → exit 0；`git diff --check` → exit 0。
  - `rg -n` 复核抑制名单计数在文档六处的一致性，以及
    `prototype/README.md:41-44` 参数清单不含 `sessions`。
- **完成门禁**: **NOT_VERIFIED**。CEv1 查询
  （`MATCH (n) WHERE n.work_unit_id = 'schema-version-signal:ux/menubar-schema-signal.md'`）
  显示本内容态 `candidate:1c301867…` 上只有
  `specimen-measured-in-prototype` 一条 `outcome: pass` 证据（修复轮所记，
  形状正确：`content_state` 节点带 `head=36e4b0e`，evidence 节点 `kind: evidence`），
  另一条 required criterion `independent-review-pass` 在该内容态上没有任何证据
  节点。两条必须都满足，故门禁为 NOT_VERIFIED。本轮为 FAIL，故不记录
  `independent-review-pass` 证据、不新建节点、不改写既有证据。

### 下一步指令

修复：schema-version-signal / ux/menubar-schema-signal.md — UX-R4-F1 与
UX-R4-F2 两条为必做项；UX-R3-F1 至 UX-R3-F4 已闭合，不再重复修复。

### Round 4 修复落实 — 2026-09-07

Codex 按用户授权仅修复两条新增发现，不新增评审轮次，不改写本轮 FAIL。
UX-R3-F1 至 UX-R3-F4 保持 Round 4 已闭合的处置，不重复修复。

- **UX-R4-F1 -> repaired in candidate**：`prototype/src/i18n.js` 注释由
  “七条文案”改为“八条文案”，与其下八个 schema 专属键及 UX Copy 表一致。
  字典值与运行逻辑均未改变。
- **UX-R4-F2 -> repaired in candidate**：`prototype/README.md` 参数清单增加
  `sessions=readable|unavailable`，说明仅作用于 `schema` / `schemaStacked`，
  不可读时保留独立会话库警告，可读或省略参数时不加该警告，并区分
  `tab=sessions` 的面板选择用途。

候选 HEAD `36e4b0e87f0ac0eda603e022afd52db26517a043`；UX 文档 blob 仍为
`3628a50c658801426cbdb0ee6015424e885322a7`。原型 diff SHA-256 为
`0c92880c0948846b9d420e203cfc8897725c44b993890cd4bc11af0e5e3f410c`，沿用
`head=<HEAD>;document=<blob>;prototype=<diff SHA-256>` 配方得到候选 digest
`6e32e97e94dc3d9e80fce77c71e96e6421afa00964a8568ad24072680006af1b`。

验证范围为 L0：清点两语言各八键并与 Copy 表对照；核对 README 开关说明与
现有 `Notices` 条件一致；检查 README 相对链接、topic 文档集、空白与 diff；上述检查全部通过。
本轮只有注释和说明文字变动，不改变渲染、文案值、依赖或配置，不重跑浏览器、
构建、Go 或 Swift 检查，也不声称进行了新的标本测量。

完成门禁：NOT_VERIFIED。对本候选态的 CEv1 定向查询未返回绑定该态的证据；
旧候选的测量记录保持原样，独立复评待执行。本轮 L0 检查不冒充
`specimen-measured-in-prototype` 或 `independent-review-pass`，不新增这两类
证据。文档完成边界未跨越；任务移交 `in_review`，Review 单元格保持未勾。

## Round 5 — 2026-09-07（复评）

- **Reviewed state**: HEAD `36e4b0e87f0ac0eda603e022afd52db26517a043`，文档 blob
  `3628a50c658801426cbdb0ee6015424e885322a7`（与 Round 4 相同，924 行——两条
  UX-R4 发现都在标本载体上，文档本身无需改动），标本载体指纹
  `sha256:0c92880c0948846b9d420e203cfc8897725c44b993890cd4bc11af0e5e3f410c`
  （`git diff --no-ext-diff prototype/`，4 个文件 +38/−13，较 Round 4 多出
  `prototype/README.md`）。合成候选态
  `6e32e97e94dc3d9e80fce77c71e96e6421afa00964a8568ad24072680006af1b`，本会话按
  既有配方独立复算，与修复轮记录的值一致。
- **Reviewer**: Claude Code（本会话；对文档与 `prototype/` 冷上下文——本会话从未
  写过其中任何一行）
- **Method**: 逐条复核 UX-R4-F1 与 UX-R4-F2 在新内容态下的处置；因文档 blob 未变，
  Round 3 与 Round 4 已闭合的四条不重新论证，只核验标本改动是否让它们回退。
  标本层的判断由本会话亲自实跑取得：起 dev server（4180）用隔离 session 的
  `agent-browser` 跑十六组量具、十二组通知条（含显式 `sessions=readable`）、
  四种非 schema 态的开关无效性、健康详情、状态板、探针与 footer；并用脚本程序化
  对照 Copy 表与两语言字典的键集。server 与浏览器已在取证后关闭。
- **Scope**: `prototype/src/i18n.js`、`prototype/README.md` 两处修复，以及为回归
  检查而重跑的全部标本行为；`docs/topics/schema-version-signal/ux/menubar-schema-signal.md`
  按未变 blob 复核。未构建 Xcode、未运行应用。

📊 综合评分: 9/10

✅ 复评结论: PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点（含已闭合的既往发现）

**UX-R4-F1 -> 已闭合。** `prototype/src/i18n.js:143` 现读“schema-version-signal
的八条文案”，其下 `:148-155` 恰为八个键。该注释声明的不变量本轮已程序化验证而非
目测：从文档 Copy 表抽出 8 个键名，从字典抽出两语言各 8 个键名，三个集合完全相同
且两语言一致。字典值与运行逻辑未变。

**UX-R4-F2 -> 已闭合，且修复超出了发现要求的范围。** `prototype/README.md:44`
的参数清单加入 `sessions=readable|unavailable`；`:52-54` 另加一段说明它只作用于
`schema` / `schemaStacked`、`readable` 是省略参数时的默认、并明确区分于选择面板的
`tab=sessions`——最后一条正是本发现点出的那层混淆。三项声明本轮逐条实测为真：
`sessions=readable` 与省略参数的通知条逐字相同（两状态 × 两语言共八组比对），
`sessions=unavailable` 才追加会话警告；在 `normal`、`partial`、`unavailable`、
`pending` 四个非 schema 态下该开关完全无效，通知条不含会话警告。README 的相对
链接经脚本核验无断链。

**修复未引入回归，逐项与 Round 4 的实测值比对：**

- 十六组量具（两状态 × 两宽度 × 两语言 × 会话可读/不可读，每组 `measure=1`）
  → 全部 `overflow:0`，与 Round 4 相同。
- 通知条：可读会话时 S1 一行、叠加态三行；不可读时二行与四行，会话警告恒在末位，
  两种语言文案不变。
- 窄英文叠加态健康详情仍为四行，`hook_deliveries` 计数 137、`database` 999/23
  与恢复散文俱在，健康详情内复制按钮计数 0。
- `?surface=states` 仍为 8 帧；`?probe=1` 仍为 49 条断言、恰好 `1 FAILED`，失败项
  仍是既存的 `详情里标注了待采集`，未新增失败项。
- footer `zh`/280 复测列宽 189.00 px、文本 108.67 px，与 Round 3、Round 4 一致。
- `Popover.jsx` 与 `data.js` 未被本次修复触及，`:801-804` 的开关条件与 `:845`
  的双码展开判定原样成立。

**Round 3 与 Round 4 的六条发现在本内容态下均保持闭合。** 文档 blob 未变，故
UX-R3-F1 至 UX-R3-F4 的闭合依据不受影响；本轮另行确认标本侧的改动没有削弱其中
依赖标本的两条——UX-R3-F1 的会话变体标本仍按文档 :499-517 渲染，UX-R3-F3 所依据的
按码匹配仍在 `Popover.jsx:845`，`check.supported_count` 的读取在 `:854`。

### 📝 小结

**逐条处置**：UX-R3-F1、UX-R3-F2、UX-R3-F3、UX-R3-F4 保持 Round 4 的 closed；
UX-R4-F1 closed；UX-R4-F2 closed。本轮无新增发现。处置矩阵中不存在开放项，故
结论为 PASS。

**被复评内容**为 HEAD `36e4b0e` 上的未提交文档 blob `3628a50c` 加原型工作区指纹
`0c92880c`，合成候选态 `6e32e97e…`。修复轮把验证范围如实声明为 L0 并明确表示
未重跑浏览器检查、不冒充标本测量证据；本轮补上了那一层——十六组量具与全部标本
行为由本会话重跑，故本轮同时构成 `specimen-measured-in-prototype` 的测量证据。

**审查过但判定不构成发现的一项，记录在此以便后来者可以低成本地不同意。**
:723 的小结句“Two requests provisioned, one refused with its fallback taken, two
confirmations, and one field this surface did not know to ask for”与其上表格的
七个实质行（第五行写明“Same as row 1”，不是独立项）逐项相加对不齐一位，差在
“recovery 行——请求 `recovery_command` 保持缺席”那一行：它请求的是“不要增加
任何东西”，归入“provisioned”还是“confirmation”本身可争。这与 UX-R3-F4 和
UX-R4-F1 不同类：那两条是对紧邻枚举的直接清点（八种组合、八个键），非真即假；
这一句是对答案形状的概括，表格本身完整且正确，也没有任何下游依赖这个数字。
按“不是发现”而非“是发现但略过”记录。

**残留不确定性**：未构建 Xcode、未运行应用，真机 SF 度量、Dynamic Type 与
VoiceOver 仍未验证——这与文档 **Verification** 手工清单划的边界相同，PASS 不缩小
它。UX-R3-F2 的无障碍部分是一条注明需真机核实的实现要求，本轮只核到它对现状的
陈述为真。本会话与起草、修复会话同属 Claude Code 运行时并读同一套项目指令，共享
同样的盲区；本仓库未要求跨运行时独立性。

**一处对本记录自身的更正**：Round 4 的 🟢 段把 `check.supported_count` 的读取
写作 `prototype/src/Popover.jsx:851`，实际在 `:854`（`:851` 是其上的注释行）。
`Popover.jsx` 自 Round 4 起未被改动，故这是原记录的坐标笔误，不是漂移；按码匹配
在 `:845` 的引用正确，该条闭合结论不受影响。

**整体评价**：两条修复都精准，其中 README 一条主动覆盖了发现里点出的
`tab=sessions` 混淆而不只是补一个词。修复轮把自己的验证范围如实降级为 L0 并
声明不冒充标本证据，这一点比修好本身更值得记下——它让本轮清楚地知道要补哪一层。
文档在五轮之后是自洽的：呈现决策有据、坐标无漂移、标本与文档逐字一致、改动面
陈述与实现相符。

- **Findings**: UX-R3-F1 -> closed、UX-R3-F2 -> closed、UX-R3-F3 -> closed、
  UX-R3-F4 -> closed、UX-R4-F1 -> closed、UX-R4-F2 -> closed。无开放发现，无新增
  发现。归属 `architecture.md` 的既有一项仍由 Beads
  `ad-bug-arch-sessions-unavailable-not-a-consequence` 承载——它指向另一份已 PASS
  并已提交的文档，按项目规则不阻塞本目标。
- **Evidence**:
  - `git rev-parse HEAD` → `36e4b0e…`；`git hash-object` 文档 → `3628a50c…`
    （与 Round 4 相同）；`git diff --no-ext-diff prototype/ | shasum -a 256`
    → `0c92880c…`；合成候选态 `6e32e97e…`（与修复轮记录一致）。
  - `npm run dev -- --port 4180`（取证后已关闭）加 `agent-browser`：
    - 十六组量具 → `document.title` 均为 `overflow:0`。
    - 十二组通知条（两状态 × 两语言 × 省略/`readable`/`unavailable`）→
      省略与 `readable` 逐字相同，`unavailable` 追加会话警告且恒在末位。
    - `state=normal|partial|unavailable|pending` 各加 `sessions=unavailable`
      → 通知条均不含会话警告，开关按 README 所述仅作用于两种 schema 态。
    - `?state=schemaStacked&lang=en&width=280&sessions=unavailable` 进入健康详情
      → 四行，137 与 999/23 俱在，复制按钮 0 个。
    - `?surface=states` → `.state-frame` 计数 8。
    - `?probe=1` → 49 条断言，`1 FAILED`，失败项 `详情里标注了待采集`。
    - `Range.getBoundingClientRect()` 复测 footer `zh`/280 → 189.00 / 108.67 px。
  - 脚本对照 Copy 表与 `prototype/src/i18n.js` 两语言键集 → 各 8 个、集合相同、
    两语言一致。
  - 脚本核验 `prototype/README.md` 的相对链接 → 无断链。
  - `bash scripts/check-topic-docs.sh` → exit 0，无输出；
    `bash scripts/check-whitespace.sh` → exit 0；`git diff --check` → exit 0。
- **完成门禁**: 见下方 Round 5 门禁同步小节。

### Round 5 门禁同步

**完成门禁: VERIFIED**（`document` 边界，绑定候选内容态）。

- WorkUnit `schema-version-signal:ux/menubar-schema-signal.md`
  （`urn:ce:agent-deck:work-unit:schema-version-signal-ux-menubar-schema-signal`）。
- 内容态
  `urn:ce:agent-deck:state:candidate:6e32e97e94dc3d9e80fce77c71e96e6421afa00964a8568ad24072680006af1b`，
  配方 `sha256(head=<HEAD>;document=<blob>;prototype=<sha256 git diff --no-ext-diff prototype/>)`，
  与既有候选态同形。
- 两条 required criterion 均由本轮证据绑定为 `outcome: pass`，各自 `supersedes`
  上一候选态 `1c301867…` 的对应证据：
  - `independent-review-pass` — 六条发现全部闭合，本轮无新增。
  - `specimen-measured-in-prototype` — 十六组量具与全部标本行为由本会话在本内容态
    上重跑。修复轮已如实声明它只做了 L0、不冒充标本测量，故这一条由本轮补足。
- 记录后已复查门禁，两条 criterion 在本内容态上均返回 `pass`。
- 本内容态为**候选态**，非不可变提交态。若后续获得提交授权，需按
  `.agent-instructions/evidence.md` 对不可变 Git tree 重记一次证据并
  `supersedes` 本候选态证据——`requirements.md` 的 `f28036a6` → `0fa9eef8`
  与本文档自己的 `d164bcd9` → `6479df9c` 都是这条路径的既有先例。

### Task checkpoint

Task checkpoint：`ad-svs-doc-ux-menubar-schema-signal-design`
（文档：schema-version-signal / ux/menubar-schema-signal.md），内容态
`head=36e4b0e` / `document=3628a50c` / `prototype=0c92880c`，候选摘要
`6e32e97e…`；完成门禁 **VERIFIED**。

提交建议：本 Task 的边界是这份 UX 文档及其标本载体。候选提交范围为
`docs/topics/schema-version-signal/ux/menubar-schema-signal.md`、
`docs/topics/schema-version-signal/reviews/ux-menubar-schema-signal.md`、
`docs/topics/schema-version-signal/tasks.md`、`docs/status.md`、
`prototype/src/i18n.js`、`prototype/src/data.js`、`prototype/src/Popover.jsx`、
`prototype/README.md`。工作区另有 `.agent-instructions/`、
`docs/documentation-workflow.md`、`docs/roadmap.md` 等与本 Task 无关的在途改动，
不应混入同一次提交。提交后需按上条对不可变 tree 重记证据并 `supersedes` 本候选态。

推送建议：目标未解析。本仓库的推送需要单独授权，且本 topic 的版本归属尚未由
任何 `vX-Y-Z-contract` 决定；在提交完成并重记不可变证据之前不构成可执行动作。

两条建议均为建议，不执行也不授权交付。

### 下一步指令

设计：schema-version-signal / tasks.md

## Round 6 — 2026-09-07（复评）

## 📋 UX 最终稿复评报告

📊 综合评分: 9/10

✅ 复评结论: PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 closed：原型权威、状态入口与测量证据仍在；R1-F2 closed：App 层 badge 决策保持不变。
- UX-R3-F1 closed：D2 仅抑制两条核心库后果，独立会话警告保留。
- UX-R3-F2 closed：披露条件、散文分支、禁止把散文放入 recovery 及无障碍实现要求保持明确。
- UX-R3-F3 closed：code/count/supported_count、双稳定码匹配和不补零规则仍完整。
- UX-R3-F4 closed：八种组合的计数与枚举相符；不可读会话变体的十六组记录保留。
- UX-R4-F1 closed：i18n.js:143 注释为八条，下方八键齐备。
- UX-R4-F2 closed：README.md:41-54 列出 sessions 参数、适用状态、默认值及与 tab 的区别。
- R2-F1 -> open，范围外 carrier `ad-bug-prototype-probe-pending-banner`；同源探针问题另有 `ad-bug-prototype-probe-pending-assertion`。既有 stat-chip、footer 和 architecture 范围外问题仍分别由 `ad-bug-prototype-statchip-fidelity`、`ad-bug-footer-routes-narrow-truncation`、`ad-bug-arch-sessions-unavailable-not-a-consequence` 承载，不改变本目标处置。

### 📝 小结

- **Reviewer**: Codex，本会话未参与被评文档或标本的起草与修复；未委派。
- **Method / Scope**: 逐条核对记录与当前修复载体，复用同内容态的 Round 5 独立测量；只更新评审及状态载体，未修改产品、原型、测试或配置。
- **Reviewed state**: HEAD `36e4b0e87f0ac0eda603e022afd52db26517a043`；文档 blob `3628a50c658801426cbdb0ee6015424e885322a7`；`git diff --no-ext-diff -- prototype/` SHA-256 `0c92880c0948846b9d420e203cfc8897725c44b993890cd4bc11af0e5e3f410c`；候选摘要 `6e32e97e94dc3d9e80fce77c71e96e6421afa00964a8568ad24072680006af1b`。本轮三个身份分量独立核对一致；同步文件不在被评文档/标本指纹内。
- **Evidence reuse**: Round 5 的十六组量具、通知条、健康详情、状态板及 footer 测量具有明确命令、结果与同一内容态；没有相关内容变化或环境失效证据，依项目规则复用，不声称本会话重新运行浏览器。原生 SF、Dynamic Type、VoiceOver 仍是实现阶段手工验收边界。
- **Gate correction**: 本轮实际固定模板查询返回 NOT_VERIFIED / missing_target_state，两条 criterion 均无本候选态证据。Round 5 的 VERIFIED 同步叙述没有当前图记录支持，历史文字保留，本轮以实际写入和复查纠正；旧 evidence 的 work_unit_id 使用逻辑名而非节点 ID，当前严格门禁也将其标为 malformed，不直接复用那些图节点。
- **Findings**: 本目标八条历史发现全部闭合，无新增发现；范围外 carrier 保留。PASS 是文档复评结论，不代表原生实现完成或交付。
- **完成门禁**: VERIFIED。固定 `gate-status.cypher` 对上述候选态返回两条有效证据、`missing_criteria=[]`、`unresolved_candidate_impacts=[]`。新增 3 个节点独立回读为 3，4 条关系预检均为 ok，写入确认创建 4 条关系。有效 evidence ID 分别以 `independent-review-pass:6e32e97e:round6` 与 `specimen-measured-in-prototype:6e32e97e:round6` 结尾。历史 malformed/失效记录未改写，也不参与当前通过判定。
- **L0**: `make check-whitespace`、`git diff --check` 通过；文档集合未变，复用 Round 5 的文档集检查。状态同步限于本主题 Review 单元格、当前摘要及本记录。

### Task checkpoint

Task checkpoint：`ad-svs-doc-ux-menubar-schema-signal-design`；候选态 `6e32e97e…`，文档边界 VERIFIED，待提交。`ad-svs-doc-tasks-design` 仍为 open，主题未完成。

提交建议：UX 文档、本评审记录、tasks.md 与 docs/status.md 的本任务同步，以及 prototype/README.md、prototype/src/i18n.js、prototype/src/data.js、prototype/src/Popover.jsx；按一个 Task 提交，隔离其他在途工作。提交后绑定不可变内容态证据。

推送建议：目标未解析；需先完成授权提交及不可变态证据，并取得单独推送授权。本轮未提交、未推送。

### 下一步指令

设计：schema-version-signal / tasks.md
