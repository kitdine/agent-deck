---
status: active
topic: schema-version-signal
subject: ux/menubar-schema-signal.md
created: 2026-09-06
updated: 2026-09-06
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
- **局限**: 评审者与起草者同为本会话主 agent，不满足项目对独立冷上下文的
  要求。本轮结论为 FAIL，独立性缺失不影响 FAIL 的成立；若后续轮次拟判 PASS，
  应由独立评审者执行。

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

残留不确定性有两项。其一，本轮评审者与起草者是同一个会话主 agent，不满足
独立冷上下文要求；FAIL 不受影响，但拟判 PASS 的轮次应换独立评审者。其二，
未执行 Xcode 构建与应用运行，所有关于呈现的判断都停留在标本层——这正是
R1-F1 要恢复的那一层。

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
- **局限**: 与 Round 1 相同——评审者与修复者同为本会话主 agent，不满足独立
  冷上下文。本轮判 PASS，故这一局限是实质性的，在此明确记录而非淡化：
  本轮的说服力来自可复现的实测命令与双 server 差分，而不是来自评审者的独立性。

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

**残留不确定性**两项。其一，评审者与修复者仍是同一会话主 agent；本轮以可复现
实测命令代替独立性，但这不是等价物，记录在此供后续审计判断。其二，标本settles
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
