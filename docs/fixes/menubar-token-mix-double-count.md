---
status: active
created: 2026-11-17
---

# 缺陷：菜单栏 Token 构成重复计入 Codex 缓存读取

载体 `ad-bug-menubar-token-mix-double-count`，v0.7.5，Lane A；工作区 `agent-deck.fix.menubar-token-mix-double-count`，分支 `fix/menubar-token-mix-double-count`。初版独立评审FAIL；第1轮Opus修复已冻结，主控核对最终相关回归，等待冷独立复评与新候选门禁。下文初版身份与观察保留作历史。

## 现象

旧样本总量 440850572，而重叠字段作为切片相加得到 617757580；差额 176907008 正是 Codex cached input。原始事件和定价调查未证明采集/计费缺陷，本次不重开它们。

## 根因

已有诊断指出 Codex cached input 是 input 子集，菜单栏却将 raw input/cache read 当独立切片；当前基线需由本轮回归确认，不能只沿用旧安装构建结论。

## 修复边界

恢复菜单栏与 Widget 构成的互斥类别、总量和占比；主控核对 Widget 的 `TokenComponent.items` 也直接相加 raw input/cache read，属于同一缺陷。Codex/Claude 语义不同，混合数据必须来自同选择、周期和 generation。保留 raw 计量、总量、价格、后端模型分组与持久状态。优先用既有 client scopes 修正投影；允许原生消费者共用窄辅助函数，不新增 Codable/wire 字段。共享原生写者仅为本修复的 Claude/Sonnet Developer，覆盖 MenuBarViewModel、WidgetViews、必要的 DesktopWire 辅助函数及直接测试；模型行任务在该写者冻结后再进入。

2026-11-17 用户已授权：评审和门禁通过后按 issue 分别提交、推送 origin、创建 PR，且 PR 必须进行 Codex review。该授权不含合并、发布或安装；子任务仍只写代码与测试，交付和正式记录由主控负责。

## 验证

基线 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。App 与 Widget 均有直接的互斥值及占比回归，覆盖 Codex-only、Claude-only、mixed、zero、cache-heavy、缺少可用 Codex scope 和选择/周期切换。work/session 仍须按真实快照选择语义核对，不能由普通 client scope 测试冒称单独验收。

`DesktopWire.swift` 新增不可编码的纯计算 `DesktopTokenMix`；App 和 Widget 以自己所选 scope/period 及同一快照的 scopes 调用。Codex overlap 取 `min(cachedRead,input)`，Claude-only 为零；缺少可用 Codex scope 时从 raw 四项之和减代表总量推导，并截断至 input。只扣 input 内的重叠，其他 raw 数值不改。

主控直接核对全部修改路径与 blob：

| 路径 | Blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarViewModel.swift | 6b8e7bb86dabadc0a9c391e202e7f134f58e955d |
| apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift | 788677d60fa3e3238c022f799b0d8f9932e5c657 |
| apps/macos/AgentDeckShared/DesktopWire.swift | 9e0634919d64ec05dba78bac532a14ca83ff8e54 |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | f0ee39379ef4751d8733c503bbff313f7c2ec014 |
| apps/macos/AgentDeckWidget/WidgetViews.swift | 25ffe0825e343948562ed68c348edf196d7a1b9a |
| apps/macos/AgentDeckWidgetTests/WidgetPresentationTests.swift | e06d1f472ffb58afdb8d26bd101f8ad56aeee620 |
| apps/macos/AgentDeckWidgetTests/WidgetTestFixtures.swift | 3f82c808b83f2bd40ab4a53664cc84ac79389bdc |

完整身份按 HEAD、以上有序七文件、`apps/macos/AgentDeck.xcodeproj/project.pbxproj=db7f3559bbb7d174dbb81b0aac3c822361c8cd92`、`apps/macos/Package.swift=25c1460a3c75d349ba897cf7003ae343afe0f3be` 的 `head=<HEAD>;path=<blob>` recipe 计算，SHA256 为 `8cb1521765105489b326508c7c2e5ef90a11ea1ded05f72695db1fe961fbf773`。CEv1 保存完整 recipe。

主控从 T3 原始命令及输出核验：

| 检查 | 观察 |
| --- | --- |
| 将共享计算 input 临时还原为 raw input 的变异对照；WidgetPresentationTests 与 MenuBarViewModelTests | EXIT=65；Widget 四个 TokenMix 测试和 App 的 Codex/mixed/cache-heavy/周期/缺 scope 场景失败，Claude-only 和 zero 通过；变异已移除。此为等价旧算法对照，不冒称真实 CI 事故。 |
| 最终 `AgentDeckWidgetTests`、`AgentDeckSharedTests/DesktopWireTests`、`AgentDeckAppTests/MenuBarViewModelTests`、`AgentDeckAppTests/MenuBarChromeTests` | xcodebuild EXIT=0，四组 suite 均 passed；App 62/0、Chrome 39/0，Widget 与 Shared 全部选定测试零失败。 |
| whitespace 与最终 diff 检查 | exit0。 |

Xcode 命令为 `xcodebuild -project apps/macos/AgentDeck.xcodeproj -scheme AgentDeck -configuration Debug -derivedDataPath apps/macos/build/DerivedData CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO CODE_SIGN_IDENTITY= -only-testing:<Suite> test`。HOME、CFFIXED_USER_HOME、AGENTDECK_TEST_HOME 及 TEST_RUNNER_AGENTDECK_TEST_HOME 使用独立的 `/private/tmp/agentdeck-macos-xctest.XXXXXX/home` 形状，保留 unsafeHome 预检；临时输出不作为持久引用。

开发中先出现 extension 存储属性编译错误，再出现 Shared suite 名称错误（EXIT=70）；均已修正，不能把这些运行计作产品回归或最终成功。App-only 旧候选的观察只作历史，本节以最终共享实现的红绿为准。

主控核对 `prototype/src/data.js` 的 input 由总量减 cacheRead/cacheWrite/output 得到，与此次互斥口径一致；现有 Popover 与 Widgets 消费同一 tokenMix，布局、文案、颜色没有改变，无需修改原型。未运行完整 `scripts/test-macos-app.sh`，没有真实 Widget 渲染、已安装 App 或用户状态验收。未变 Go 后端不为阶段变化重跑；未声明完整 App harness 已通过。Developer 终态无待完成命令、helper 或 xcodebuild；来源未确认的空临时 HOME 未删除。

当前独立评审与五项必需 criterion 的最终 gate 待完成；未提交或推送。

## Review — Round 1

方法：冷上下文 Codex/gpt-6.1-sol，requested reasoningEffort=high；独立只读源码、producer 契约和最小仓库外探针评审。主控直接核对 producer、累加字段、投影公式与候选 blob 后裁决。

### 📋 独立评审报告

📊 总体评分：7/10

评审结论：FAIL

完成门禁：FAILED

#### 🔴 严重问题 — 必须修复

**MTM-R1-F1 — 中：非零 Codex cache write 在正常与降级路径不一致。**
位置：`DesktopWire.swift` 的 `DesktopTokenMix.init`，以及相关 App/Widget/Shared 夹具。
`internal/usage/usage.go` 的 `eventTokenTotal` 对 Codex 只返回 input+output，但统计另投影 cache_write；当前正常路径只扣 cached-read，降级路径却把 read+write 一并从 input 扣掉。输入 input=10、output=1、read=4、write=2、tokens=11 时，Codex selected 或 all+可用 Codex scope 得 `[6,1,4,2]` 合计13；同一 all totals 在 scope 缺失/不可用时得 `[4,1,4,2]` 合计11。主控通过源码与此算式确认不一致，原测试的 Codex totals 加入 write 不符合实际 producer。
💡 有界修复：按真实代表总量统一互斥构成，修正夹具；新增非零 cache write 的 Codex/all、scope 有/无/不可用一致性回归。raw、代表总量、价格、wire 和持久状态不改。当前 finding OPEN，进入第1轮修复。

#### 🟡 建议改进 — 推荐

无其他确认发现。

#### 🟢 优点

App/Widget 使用各自 scope、period 与快照，work/session 有独立投影；共享类型不可编码，未改布局、颜色、文案、后端或持久状态。

#### 📝 总结

被评候选 `8cb1521765105489b326508c7c2e5ef90a11ea1ded05f72695db1fe961fbf773` 的七文件、Package/project 与 HEAD 均匹配。已有红绿及相关 suite 结果保留为该候选观察，不足以排除错误 fixture 掩盖的 cache-write 缺陷；停止扩大验证。主控在对应 ContentState 登记此范围内失败后查询为 FAILED，不将原绿测或建议裁决冒称通过。真实 Widget 渲染与完整 App harness 未验证。

## 修复 — MTM-R1-F1

repair_count=1。用户要求多轮提升模型后，原Developer线程从Sonnet/medium切至Opus5.5/high；运行时确认model为claude-opus-5-5，模型内部无法观测实际effort，不将请求值冒称执行证据。第一次修正正常路径也扣Codex cache write，并更正producer夹具；主控对其报告的mixed异常可用性差异提出同一finding闭合异议，因此继续同一轮修复，没有重置或新增评审轮次。

最终 `DesktopTokenMix(scope:period:)` 只读所选周期自身totals。对all/codex，overlap为 `max(0, raw四项之和 - tokens)`，截断至自己的input；Claude不扣。两个消费者同步窄签名，不再读取其他scope；没有wire/Codable字段、raw/代表总量/价格/持久化或后端分组变化。此做法与prototype的本周期余量口径一致。

主控实际核对七文件blob及原始命令输出：

| 路径 | Blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarViewModel.swift | 728bcd26cf9d57c3f948e282ee66b3c9d47afd67 |
| apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift | a65bd908f03e1a9cee2f7fd91ebe906cc81b1488 |
| apps/macos/AgentDeckShared/DesktopWire.swift | f4327c0fdfa9b458e7ad17ba9b00e62a76512c3e |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | 4de43731ce526d6e89ef87a42dd36179af68bb17 |
| apps/macos/AgentDeckWidget/WidgetViews.swift | 14246cade1e6db04ce6e88442fd536bb7ba21356 |
| apps/macos/AgentDeckWidgetTests/WidgetPresentationTests.swift | 70ca7ac1ba7d27928bc4492b6ad25b46f8e35101 |
| apps/macos/AgentDeckWidgetTests/WidgetTestFixtures.swift | aefee6aed823bdf00379c95b944af1a4d088b888 |

HEAD、Package与project身份不变，沿用既有recipe的最终指纹见本轮CEv1 ContentState。两个真正失败先行的增量分别覆盖：正常cache-write向量在旧正常路径得到总30/13而非28/11；mixed异常向量只有附加Codex scope可用时错误得到总30，而缺失/不可用已为28。主控回读后者原始xcodebuild EXIT=65及最终EXIT=0/TEST SUCCEEDED；最终Shared29、App ViewModel64、Chrome39、Widget65例均0失败，whitespace退出0，无pending/native进程。不重复Go或完整安装验收。

正常合成Codex `(11:10,1,4,2)`、Claude `(17:3,2,5,7)`、all `(28:13,3,9,9)` 对三种scope可用性统一为all `[7,3,9,9]`；mixed异常向量all `(28:13,3,13,11)`统一为 `[1,3,13,11]`。原Bug的raw all与440850572对账保持不变；client拆分仅是按producer构造的合成夹具，不是实际账号数据。

极端不一致字段无法装进代表总量时，input截0，保留其他发布分量，不编造raw；例如单Codex `[0,1,8,4]`之和13高于tokens11。此限制明确交复评，不以正常scope测试冒称覆盖。零值、cache-heavy、周期/客户端切换保护均保留。真实Widget渲染、完整App harness未验证；原型未改。该finding已实现修正但正式关闭仍待独立复评，无提交/推送。


## Review — Round 2

方法：冷上下文Antigravity/Claude Opus5.5，requested thought_level=high；主控复核实际源码与编译警告后记录正式结论。被评候选a1d747b0ac25e7b024eb13f43fd16285eb3e80a255049c9c6807b7a7a21d7626，HEAD及七文件、project/Package逐项一致，Reviewer独立重算recipe一致；无后台操作或写入。

### 📋 独立复评报告

📊 总体评分：8.4/10

评审结论：FAIL

完成门禁：FAILED

#### 🔴 严重问题 — 必须修复

**MTM-R2-F1 — 低：breakdownPanel留下新增未使用的totals绑定。**
位置：MenuBarViewModel.swift的let totals = period.totals。投影已改用exclusive，绑定不再被读；主控核对原始编译输出，警告原文为 initialization of immutable value 'totals' was never used; consider replacing with assignment to '_' or removing it。这是本次引入的真实编译告警，按当前所有范围内finding关闭规则阻塞PASS，不以低级豁免。
💡 有界修复：删除单个无用绑定，编译并复验相关ViewModel suite，不改计算、夹具或测试语义。当前OPEN。

#### 🟡 建议改进 — 推荐

无其他确认finding。

#### 🟢 旧发现关闭及边界

MTM-R1-F1 CLOSED：正常非零cache write和mixed异常只用所选period totals，不再读取其他scope；两消费者client/period/generation来自同一选择，fixture按真实producer构造。原Bug440850572对账、Claude独立cache、raw/wire/价格/持久化均保留。对四项不可能装进tokens时输入截0、保留其他发布分量的有限降级已明确评估，不产生新的finding；同一输入不随scope可用性变化。

#### 📝 总结

复用主控核验的Shared29/App64/Chrome39/Widget65均0失败及红绿观察，未重复广泛测试、Go或完整App harness，未渲染真实Widget。决定性低级告警后停止扩大。主控将此精确候选的独立评审失败登记后，gate查询为FAILED；不改变旧观察或把已关闭R1重新算开放。

## 修复 — MTM-R2-F1

repair_count=2。按小评审问题主控直接修正的要求，仅删除MenuBarViewModel的一个未使用绑定。主控实际运行隔离HOME的MenuBarViewModelTests：64例0失败，xcodebuild EXIT0/TEST SUCCEEDED；totals未使用告警消失。编译另含未修改区域的NSApplication可选值到Any告警，不据此声称零告警或扩大本修复。其他生产、测试、配置、依赖均不变，前轮Shared/Widget/Chrome的有效观察按此局部无行为影响保留，不再重跑广泛套件。源字段修正与R1行为证据不变；等待一次冷独立有界复核，不自判PASS或交付。


## Review — Round 3

方法：新冷上下文AGY，requested Opus5.5/high，有界独立复核R2剩余告警及作用域复用；T3未暴露actual executedModel/effort，均记unknown，不把请求配置当执行证据。主控核对终态无pending、只读声明、独立逐项blob和recipe重算、直接编译结果后裁决。

### 📋 最终有界复评报告

📊 总体评分：9.5/10

评审结论：PASS

完成门禁：VERIFIED

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无遗留范围内finding。

#### 🟢 关闭链与复用

MTM-R2-F1 CLOSED：仅删除未使用totals绑定，当前mixTotal和四分量均来自exclusive；读取period.totals本身无副作用，未改变运行行为。主控最终ViewModel64例0失败与该告警消失的编译证据有效，原有未改动NSApplication可选值告警不作为新发现。

MTM-R1-F1保持CLOSED：producer、共享计算、Widget、全部fixture、依赖与toolchain不变；R2完整语义核对和Shared29/Chrome39/Widget65均0失败的观察可明确复用。App64本轮重跑；零值、cache-heavy、非零write、原Bug440850572及mixed异常可用性不变式不变。极端原始分量不能装进tokens时输入截0的已披露限制不因本次无行为变化重新打开。

#### 📝 总结与Task checkpoint

最终HEAD4ac56db32e13c4e925b648c620ba76fc588a7781；唯一变化App blob为656ab01d209bca0e29a58eb583b8d89852ac03d8，其余六文件与project/Package同上轮。独立重算最终指纹d6fd15475f4b1b7eb304563291b1e89ff96dc60f7c7101f9637b1b076d476fc0一致。主控在同一ContentState登记作用域复用和独立复评，查询VERIFIED5/5，缺失/失效/未决影响均为空；旧8cb与a1候选的FAILED保留，不重标旧观察。

repair_count=2，未重置计数。提交边界为七个生产/测试文件及本fix记录，按用户已有授权准备独立签名提交、推送origin和issue PR；不含其他任务或原型修改。完整App harness、真实Widget渲染和Go未在此轮运行，不冒称完成。代码来源Claude Sonnet/Opus，Codex维护正式记录并删除一个无用绑定；AGY只做独立评审。PR须在push和PR Actions都成功后请求GitHub Codex review，原Bug在这些交付协调完成前不关闭，无merge/release/install。
