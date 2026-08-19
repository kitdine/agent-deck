---
status: active
topic: desktop-app
subject: tasks.md
---

# Review log — desktop-app / tasks.md

## Round 1 — 2026-08-17

### 📋 独立设计评审 — desktop-app / tasks.md

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: HEAD `e583cdf1959cdabaa897ad32fea8020546e6118c`；
  `docs/topics/desktop-app/tasks.md` blob
  `88880b1b00ade2419029b2ca598d83406f18ca8c`（工作区对本文档无改动，即已提交状态）。
  比对基准为同一 HEAD 的 `requirements.md`、`architecture.md`、`ux/menubar.md`、
  `ux/widget.md`——本 topic 四份已通过评审的文档。
- Reviewer: claude-code
- Target class: process / plan。它不描述行为，而是把已通过的契约切成可派发的工作
  单元并承载状态，因此适用 premise validity、consistency、decision completeness、
  internal contradiction 等维度。
- Method: 单 agent 有界评审。核心动作只有一个——**把每个 task 的交付项逐条回到它
  声称实现的那份契约核对**。这个方向是本 topic 反复出问题的地方：`ux/menubar.md`
  与 `architecture.md` 在 Round 8 之后被大幅重写，而 tasks.md 的 task 描述写于
  重写之前，正是"散文改了、依附于它的产物没跟上"这一形态的上一级——只不过这次
  依附的产物是派发指令本身。
- Scope: 六个 task 的交付项与契约的一致性、依赖关系、Documents 与 Tasks 两个
  状态矩阵、文档集审计
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0（本 topic 自带的文档集
  审计，是本类目标的必需证据：它比对本文件的 Documents 矩阵、磁盘上的文件与
  `requirements.md` 点名的 surface）；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审的文档。

#### 🔴 严重问题 — 必须修复

**T-F1 — `docs/topics/desktop-app/tasks.md:42,45`: task 3 的交付项仍描述 Round 8
已废弃的界面结构与状态模型。**

- 行为风险：`:42` 要求"Implement provider, usage, cost, recent-session, warning,
  and health summaries"——五段式，其中 provider 已被移入 footer 只读文本、
  recent-session 已被移出阅读界面（`ux/menubar.md:784` 明确 recent-session 行留在
  session detail 而不是 popover body）、warning 与 health 已不再是 section 而是
  banner（`:198`）。`:45` 要求"Define loading, stale, offline, partial, empty, and
  error states"——正是 R3-F6 认定"六个名字互不排斥"而废弃的那套模型，现行契约是
  三个 surface 加正交 qualifier 的真值表（`ux/menubar.md:120-152`）。派发时读这两行
  的实现者会去搭一个已通过评审的契约明确不要的界面；而 tasks.md 是**唯一的状态
  权威**（`:9`），其 task 描述就是派发内容本身。
- 证据：`tasks.md:42`、`:45`；`ux/menubar.md:175-199`（四段推导与 footer/banner
  归属）、`:120-152`（surface/qualifier 真值表）、`:784`（recent-session 归属）。
  本 topic 已在 R9-F1 上因同一形态失败过一次，只是那次落在 specimen 上。
- 💡 有界修复：把 `:42` 改为四段式加 footer 与 banner 的实际交付面，把 `:45` 改为
  surface/qualifier 模型，两者均直接引用 `ux/menubar.md` 的对应小节而不是复述枚举
  ——复述是这次失效的直接原因。

**T-F2 — `docs/topics/desktop-app/tasks.md:37-40,48-49`: task 3 的契约指针与
"additive、不重开 task 1"的声明只覆盖 `provider.candidates`，遗漏了本 topic 最大的
一项未实现契约 `usage.presentation`。**

- 行为风险：`architecture.md` 的 menu-bar wire extension 现在包含两个 additive
  对象：`provider.candidates` 与 `usage.presentation`。后者承载菜单栏四段的全部
  分析数据（`Client` × `Period` 的 totals、model rows、日桶、质量档、coverage、
  rhythm、client subtotals），是 Round 19 明确"尚无实现、由菜单栏任务连同 fixture
  与解码测试一并交付"的部分。tasks.md 的 `:37-40` 只把契约指针写成
  "the additive `provider.candidates` section, the switch command surface, its
  result envelope, and switch operation ownership"，`:48-49` 也只声明
  `provider.candidates` 是 additive、不重开 task 1 的评审。于是两件事悬空：谁交付
  `usage.presentation` 的 Go 实现，以及它是否重开已 PASS 并已提交的 task 1。后者
  不是理论问题——task 1 的 `Review` 已勾选（`:162`），若该对象被判定属于它，矩阵
  当前状态就是错的。
- 证据：`tasks.md:37-40`、`:48-49`、`:162`；`architecture.md` 的
  `### Additive usage.presentation` 与 `### Additive provider.candidates` 两节；
  `reviews/menubar-experience.md` Round 19 残余不确定性一段。
- 💡 有界修复：在 `:37-40` 的契约指针中加入 `usage.presentation`，并把 `:48-49`
  的 additive 声明改为同时覆盖两个对象、明确其 Go 实现与 fixture 属 task 3、
  不重开 task 1 的评审。

#### 🟡 建议改进 — 推荐

**T-F3 — `docs/topics/desktop-app/tasks.md:476-477`: 任务 3 与 4 "may proceed
independently" 与契约已不符。**

- 行为风险：`architecture.md` 现在写明 `usage.presentation` 既是菜单栏四段的来源，
  **也是 App Group 投影中用量值的来源**，而投影由 host 写、widget 读。task 4 是
  读侧（`:55-56`"backed only by the redacted App Group snapshot"），写侧的用量内容
  依赖 task 3 交付的 wire 对象。因此两者不再是可并行的兄弟任务：task 4 的数据在
  task 3 落地前不存在。按现文调度会让 task 4 在没有可读内容的情况下开工。
- 证据：`tasks.md:476-477`；`architecture.md` 的 `usage.presentation` 小节首段
  （"the source for the menu bar's four usage sections and the presentation-safe
  usage values in the App Group projection"）；`ux/widget.md:11`、`:258`。
- 💡 有界改进：把依赖句改为 task 4 依赖 task 2 与 task 3；或明确 task 4 可在
  placeholder/unavailable 路径上先行，其数据路径部分等待 task 3。

**T-F4 — `docs/topics/desktop-app/tasks.md:40-41`: task 3 的契约段落声称两份文档
"were reopened by `ux/widget.md`'s W-F1"，该状态已于 Round 19 关闭。**

- 行为风险：不改变任何交付内容，但本文件是唯一状态权威，一句已过期的状态描述与
  同文件 `:94-95` 已勾选的两个 `Review` 单元格直接矛盾，读者需要自行判断哪个为准。
- 证据：`tasks.md:40-41` 与 `:94-95`；`reviews/menubar-experience.md` Round 19。
- 💡 有界改进：改为指向两份文档已通过，或直接删除该状态句，只保留契约指针——
  状态由下方矩阵承载，task 描述里重复状态正是它容易过期的原因。

#### 🟢 优点

- **两个矩阵与磁盘、与各自评审记录一致。** Documents 矩阵四项已勾选、`tasks.md`
  自身未勾选；Tasks 矩阵 task 1、2 已勾选且各有 PASS 轮次记录。
  `bash scripts/check-topic-docs.sh` exit 0，说明矩阵、文件与 `requirements.md`
  点名的 surface 三者互相falsifiable而不是靠人读。
- **task 6 的边界写得很克制。** `:78-85` 明确它只把本 topic 的交付回填进 living
  specs，**不**抬版本号、不跑 preflight、不选发布通道、不写发布说明，并把版本级
  收口指给 `v0-5-0-contract`。这正是 AGENTS.md 的发布决策边界，写在了会被读到的
  地方。
- **verification level 逐任务给出且与内容相称**：task 1 L2（JSON/exit-code 契约）、
  task 2 与 3、4 L3（构建/应用边界、渲染与交互验收、扩展沙箱与隐私）、task 5 L4
  （聚合发布门禁）、task 6 L2。没有出现"一律 release-verify"这类过度路由。
- **状态叙述保留了完整的失败史而不是只留结论。** 十九轮的 FAIL 原因、被撤回的
  PASS、以及两条收敛判据都在文中，后续文档可以直接复用；`:465-467` 也如实记下
  "评审对象是文档而非 task 3"这一区分。
- **"Starting a task"一节把派发前必读集写全了**（AGENTS.md、requirements、
  architecture、任务本身、发布与版本契约、验证路由），并规定 `Dev` 只在验证通过后
  勾选、`Review` 只在独立评审记录 PASS 后勾选。

#### 📝 总结

评审对象是 HEAD `e583cdf1` 的 `tasks.md`。两个状态矩阵、验证路由、task 6 的发布
边界与派发前置说明都是可用的；使本轮 FAIL 的是 task 3 的交付描述：它写于 Round 8
重设计之前，仍要求实现五段式界面与六状态模型（T-F1），并且没有跟上 menu-bar wire
extension 的第二个对象 `usage.presentation`（T-F2）。这两项都直接决定派发出去的是
什么工作，而不是措辞。

T-F3 与 T-F4 是同一成因的较小两处：依赖关系与状态句都在契约变化后没有回读。

本 topic 到此出现过三次同一形态——R9-F1 是 specimen 没跟上散文，M-F4 是标题没跟上
散文，本轮是**派发指令没跟上契约**。前两次的教训写在两份 surface 记录里；这一次说明
它同样适用于状态与计划文档：契约通过之后，应当回读所有复述过该契约内容的地方，
而 task 描述是最容易被漏掉的一处，因为它读起来不像"文档内容"，像"待办清单"。

残余不确定性：T-F2 的修复需要确认 `usage.presentation` 的 Go 实现是否真的属 task 3
而非新增一个 wire 任务——Round 19 的记录倾向前者，但那是评审记录的判断，最终归属
由本文件决定，本轮不代为选择。

证据：`git rev-parse HEAD` -> `e583cdf1959cdabaa897ad32fea8020546e6118c`；
`git hash-object docs/topics/desktop-app/tasks.md` ->
`88880b1b00ade2419029b2ca598d83406f18ca8c`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / T-F1 T-F2 T-F3 T-F4
```

## Round 2 — 2026-08-17（修复轮）

- Repair state: HEAD `e583cdf1959cdabaa897ad32fea8020546e6118c`，工作区未提交；
  `docs/topics/desktop-app/tasks.md` repair blob
  `48394aaa3221af452f5e45ad3858667270946abd`。
- Repair owner: codex
- Scope: 只修复 Round 1 的 T-F1、T-F2、T-F3、T-F4；不改动四份已通过的
  requirement/architecture/UX 合同、产品代码、测试或配置。

### Finding-to-change mapping

- **T-F1 repaired in the candidate.** Task 3 不再复述已废弃的五段式界面和六状态
  枚举。派发项直接指向 `ux/menubar.md#sections` 与 `#presentation-state`，要求按现行
  reading surface、surface-level qualifiers、footer，以及 coordinator surface 加正交
  qualifier 的模型实现；recent-session rows 明确保留在 session detail，且不得另建
  第二套状态机。
- **T-F2 repaired in the candidate.** Task 3 的 architecture 指针现在同时点名
  `usage.presentation` 与 `provider.candidates`。同一 task 明确拥有两个 additive
  对象的 Go 实现、complete/partial/retained-legacy v1 fixtures、Go 与 Swift decoder
  tests；两者均不抬 `wire_version`，也不重开 task 1 已交付合同的 Review PASS。
- **T-F3 repaired in the candidate.** 依赖段改为 task 3 依赖 task 2，task 4 同时依赖
  tasks 2 与 3；理由写明 widget 的 App Group usage projection 消费 task 3 交付的
  `usage.presentation` producer，不再把 tasks 3、4 声称为可独立并行的兄弟任务。
- **T-F4 repaired in the candidate.** Task 3 描述中删除了 “Both were reopened by
  `ux/widget.md`'s W-F1” 的过期状态复述；文档状态只由下方 Documents matrix 与评审
  记录承载。

### Verification and status

- `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。
- 本轮是 Repair，不是独立复评；`tasks.md` 的 `Review` 单元格继续未勾选，未声称
  Review PASS 或 completion-evidence `VERIFIED`。

Repair status: T-F1、T-F2、T-F3、T-F4 已在候选文档中完成；Review verdict 仍为
FAIL，等待独立复评。

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md
```

## Round 3 — 2026-08-17

### 📋 独立复评 — desktop-app / tasks.md

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewed state: HEAD `e583cdf1959cdabaa897ad32fea8020546e6118c`；
  `docs/topics/desktop-app/tasks.md` 未提交工作区 blob
  `e3b48148cb3cf9026d8747b74d329fa475b1386b`。
  **记录更正**：Round 2 声明的 repair blob `48394aaa…` 与当前文件不符，且该对象不在
  仓库对象库中（工作区 blob 只被计算、不被写入）。差异原因可确定：Round 2 在计算
  哈希之后又改了同一文件的 `Next action` 行（现为 `复评：desktop-app /
  reviews/tasks.md`）。因此本轮判定的是上列 `e3b48148…`，它包含 Round 2 的全部修复
  加那一行。这不改变任何修复内容，但声明的状态必须与被判定的状态一致，故在此更正
  而不是沿用。
- Reviewer: claude-code（Round 2 的修复由 codex 完成）
- Method: 单 agent 有界复评。T-F1～T-F4 逐条回到被改动的文本核对，并验证新引入的
  两个 section 锚点确实存在。随后把 Round 1 的核心动作**对全部六个 task 重跑一遍**
  ——逐条把交付项与它所声称的契约对齐——因为 Round 1 只在 task 3 上执行了该动作，
  而 T-F1 的成因（task 描述复述契约、契约改写后没人回读）对每个 task 同样成立。
- Scope: T-F1～T-F4 的处置；六个 task 的交付项与契约一致性；依赖段；两个状态矩阵
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，`tasks.md` 的 Document gate 不关闭。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

**T-F5 — `docs/topics/desktop-app/tasks.md:56-61`: task 4 与 T-F1 同型——复述了
`ux/widget.md` 已废弃的词汇，且它是唯一没有契约指针的在制 task。**

- 处置：**新发现。Round 1 只对 task 3 执行了契约比对，未把同一动作施于 task 4，
  因此漏掉了它。** 本轮补做全六个 task 的比对时发现。
- 行为风险：`:60-61` 要求"Define **stale age**, privacy redaction, placeholder,
  snapshot, timeline, and **unavailable-host** states"。现行 `ux/widget.md` 的模型是
  三个 surface（`placeholder`、`dataSurface`、`unavailableSurface`，`:159-161`）加
  四个 qualifier（`aging`、`old`、`partial`、`empty`，`:165-166`），而 `snapshot` 与
  `timeline` 根本不是 state，是 timeline 的两个 entry point。尤其是 `stale` 这个词：
  它正是 W-F2 从 widget 侧移除的名字，移除理由写在 `ux/widget.md:144-155`——同名
  不同推导会诱使实现者在两个 surface 上共用一个 Swift 类型，而它至少在其中一个上
  是错的。派发指令把这个名字又带了回来。task 4 同时是唯一没有 `Contracts:` 指针的
  在制 task：task 2 与 task 3 都指向各自契约小节，task 4 不指向刚在 Round 13 通过的
  `ux/widget.md`——缺指针正是这段过期枚举能长期无人回读的原因。
- 证据：`tasks.md:56-61` 与 `ux/widget.md:157-171`、`:144-155`；对照
  `tasks.md:37-41`（task 3 修复后的契约指针写法）。
- 💡 有界改进：给 task 4 加一条与 task 3 同构的 `Contracts:` 指针，指向
  `ux/widget.md` 的 surface/qualifier 小节与 Timeline 小节；把 `:60-61` 改为"按该
  契约实现 surface、qualifier 与 timeline entry points"，不再枚举——与 T-F1 采用的
  同一手法，理由也相同：复述是这类失效的直接原因。

#### 🟢 优点

- **T-F1 已关闭，且用的是"指向契约"而不是"更新复述"。** `:37-41` 现在把 task 3 的
  契约指针拆成三处具体锚点（`ux/menubar.md#sections`、
  `ux/menubar.md#presentation-state`、`architecture.md#menu-bar-wire-contract-extension`），
  交付项改为"按 Sections 契约实现 reading surface、surface-level qualifiers 与
  footer"、"按 Presentation state 契约由 coordinator surface 与正交 qualifier 推导
  呈现，不得引入第二套状态机"，并明确 recent-session 行留在 session detail。两个
  锚点经核实存在（`ux/menubar.md:96` 的 `### Presentation state`、`:164` 的
  `### Sections`）。这正是 Round 1 建议的方向：让 task 描述失效的可能性从"随契约
  改写而过期"降到"锚点被改名"。
- **T-F2 已关闭，且把三件悬空的事一次写清。** `:50-53` 现在明确 task 3 负责两个
  additive wire 对象的 Go 实现、complete/partial/retained-legacy-v1 三类 fixture、
  以及 Go 与 Swift 两侧的 decoder 测试，并声明两者都不抬 `wire_version`、不重开
  task 1 已交付契约及其 Review PASS。Round 1 指出的"谁交付 `usage.presentation`"
  与"是否重开 task 1"两个问题都有了答案，且答案与 `reviews/menubar-experience.md`
  Round 19 的判断一致。retained-legacy-v1 fixture 被点名尤其对：它是 R3-F5 当年
  确立的、唯一能证明反向兼容的那份。
- **T-F3 已关闭，并写出了理由而不只是改了依赖箭头。** `:505-511` 现在是"task 3
  依赖 task 2；task 4 依赖 task 2 与 3，因为 widget 的 App Group usage projection
  消费 task 3 交付的 `usage.presentation` producer；它不是可独立起步的兄弟任务"。
  依赖关系与契约事实对齐，且后续读者能看出为什么。
- **T-F4 已关闭。** task 3 描述中那句过期的"Both were reopened by W-F1"已删除，
  状态只由 Documents 矩阵与评审记录承载——这也是 Round 1 建议的形式（task 描述里
  不重复状态）。
- **修复未越界。** 四份已通过的契约文档、产品代码、测试、配置均未改动；两个状态
  矩阵与 `bash scripts/check-topic-docs.sh` 保持一致。

#### 📝 总结

T-F1～T-F4 全部关闭，且四项都用了"指向契约、删除复述"的同一手法，而不是把复述内容
更新一遍——这使同类失效在下一次契约改写时不再自动发生。

本轮 FAIL 只因 T-F5：task 4 仍复述着 `ux/widget.md` 已废弃的词汇（`stale age`、
`unavailable-host`，以及把 timeline entry point 当作 state），并且是唯一没有契约
指针的在制 task。它与 T-F1 完全同型，Round 1 漏掉它是因为只对 task 3 做了契约比对
——这一点如实记在上面，因为该遗漏本身就是本 topic 反复出现的那个教训的又一个实例：
**做过一次的检查要施于同一类的全部对象，而不是只施于暴露问题的那一个。**

修复边界清晰：一条 `Contracts:` 指针加一条交付项改写，与 task 3 已完成的改法同构。

残余不确定性：本轮按"是否复述了已被契约改名或废弃的词汇"判断，tasks 1、5、6 未
发现同类问题（分发与回填术语稳定，task 1 已交付）；但 tasks 5、6 同样没有契约指针，
本轮不判定为缺陷——它们的契约是 `architecture.md` 的 Packaging 与 Downstream
contracts 小节，描述中没有复述其内容，因此不存在过期风险。若后续为一致性给所有
task 补指针，那是编辑选择而非修复。

证据：`git rev-parse HEAD` -> `e583cdf1959cdabaa897ad32fea8020546e6118c`；
`git hash-object docs/topics/desktop-app/tasks.md` ->
`e3b48148cb3cf9026d8747b74d329fa475b1386b`；
`git cat-file -t 48394aaa…` -> 对象不存在（Round 2 声明值已更正如上）；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / T-F5
```

## Round 4 — 2026-08-17（修复轮）

- Repair state: HEAD `e583cdf1959cdabaa897ad32fea8020546e6118c`，工作区未提交；
  `docs/topics/desktop-app/tasks.md` final repair blob
  `a397e069900e5e200b35cb2e649972432e3b2d9b`。该 hash 在 task 描述、状态段和
  `Next action` 全部写完后计算，避免重复 Round 2 的候选指纹漂移。
- Repair owner: codex
- Scope: 只修复 Round 3 的 T-F5；不改动 T-F1～T-F4 已关闭的 task 3 描述、四份
  已通过合同、产品代码、测试或配置。

### Finding-to-change mapping

- **T-F5 repaired in the candidate.** Task 4 新增与 task 3 同构的 `Contracts:` 指针：
  `ux/widget.md#surface-and-qualifiers` 负责 surface/qualifier 模型，
  `ux/widget.md#timeline` 负责 WidgetKit entry-point 行为。原先把 `stale age`、
  `placeholder`、`snapshot`、`timeline`、`unavailable-host` 混成 state 的枚举已删除，
  改为按两份锚点实现 surface、qualifier 与 timeline entry points，并明确不得引入
  parallel state vocabulary。
- 该改法保留 task 4 的 redacted App Group snapshot、App Intent、sandbox/privacy
  证明与 L3 验证边界，只移除与已通过 UX 合同冲突的派发词汇。

### Verification and status

- 两个新增 Markdown anchor 均在当前 `ux/widget.md` 中存在。
- `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。
- 本轮是 Repair，不是独立复评；`tasks.md` 的 `Review` 单元格继续未勾选，未声称
  Review PASS 或 completion-evidence `VERIFIED`。

Repair status: T-F5 已在候选文档中完成；Review verdict 仍为 FAIL，等待独立复评。

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md
```

## Round 5 — 2026-08-17

### 📋 独立复评 — desktop-app / tasks.md

📊 总体评分：9/10

✅ 结论：PASS

- Reviewed state: HEAD `e583cdf1959cdabaa897ad32fea8020546e6118c`；
  `docs/topics/desktop-app/tasks.md` 未提交工作区 blob
  `a397e069900e5e200b35cb2e649972432e3b2d9b`——与 Round 4 声明**逐字节一致**，
  Round 3 更正过的候选指纹漂移未再发生。
- Reviewer: claude-code（Round 4 的修复由 codex 完成）
- Method: 单 agent 有界复评。T-F5 回到被改动的 task 4 核对，并验证两个新锚点在
  `ux/widget.md` 中确实存在。随后重跑 Round 3 建立的判据——**对六个 task 全体**做
  契约比对，并在 Task breakdown 全段扫描已被四份契约改名或废弃的词汇，而不只看
  被点名的那一个 task。
- Scope: T-F5 的处置；T-F1～T-F4 的回归；六个 task 的契约一致性；两个状态矩阵；
  依赖段；文档集审计
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0（本类目标的必需证据）；
  `make check-whitespace` -> exit 0；`git diff --check` -> exit 0。未改动任何产品
  代码、测试、配置或被评审的文档。
- Completion evidence: `tasks.md` 的 Document gate 随本轮关闭，CEv1 按本轮的确切
  内容状态（HEAD 加 blob 指纹）记录并复查为 `VERIFIED`；绑定的是未提交候选状态，
  授权提交后需按 Git tree 重记。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **T-F5 已关闭，且与 task 3 的改法同构。** `:57-59` 新增 `Contracts:` 指针，分别
  指向 `ux/widget.md#surface-and-qualifiers` 与 `#timeline`；两个锚点经核实存在
  （`ux/widget.md:142` 的 `## Surface and qualifiers`、`:225` 的 `## Timeline`）。
  `:63-64` 把原先混淆 state 与 entry point 的枚举替换为"按这两份契约实现 surface、
  qualifier 与 timeline entry points，且不得引入 parallel state vocabulary"——
  最后半句直接对上了 W-F2 当初移除 `stale` 的理由。task 4 的 redacted snapshot、
  App Intent、sandbox/privacy 证明与 L3 边界都保留，只删掉了与已通过 UX 契约冲突的
  派发词汇。
- **Round 4 修掉了 Round 2 留下的流程缺陷。** 它在 task 描述、状态段与
  `Next action` 全部写完之后才计算候选指纹，因此本轮判定的 blob 与它声明的完全
  一致。Round 3 的那条更正没有变成一条需要反复解释的历史，而是变成了下一轮的做法。
- **T-F1～T-F4 无回归。** task 3 的三处锚点（`ux/menubar.md#sections`、
  `#presentation-state`、`architecture.md#menu-bar-wire-contract-extension`）与两个
  additive wire 对象的归属、fixture 与 decoder 测试要求、不抬 `wire_version` 的
  声明均保持 Round 3 判定时的状态；依赖段仍写明 task 4 依赖 tasks 2 与 3 及其理由；
  task 3 描述中没有回流任何状态复述。
- **全 task 词汇扫描无残留。** Task breakdown 全段再无被契约改名或废弃的词
  （`stale`、`offline`、`loading`、六状态枚举、五段式结构、把 recent-session 当作
  阅读界面内容）。仅有的两处形似命中都是正确用法：`:43` 明确 recent-session 行留在
  session detail，`:77` 的 "completion loading" 指 shell 补全加载。task 2 的
  `architecture.md#foundation-runtime` 与 task 3 的两个 architecture 锚点同样存在。
- **两个状态矩阵与现实一致，且被工具而非人眼保证。** Documents 矩阵四项已勾选、
  本文件待勾选；Tasks 矩阵 tasks 1、2 已勾选并各有 PASS 记录；
  `bash scripts/check-topic-docs.sh` exit 0 说明矩阵、磁盘文件与 `requirements.md`
  点名的 surface 三者互相可证伪。
- **task 6 的发布边界与"Starting a task"的派发前置说明保持完整**：不抬版本、不跑
  preflight、不选通道、不写发布说明；`Dev` 只在验证通过后勾选，`Review` 只在独立
  评审记录 PASS 后勾选。

#### 📝 总结

T-F5 关闭，T-F1～T-F4 无回归，disposition 矩阵内无任何未关闭项——minor 也没有——
因此结论为 PASS。评审对象是 HEAD `e583cdf1` 与 blob `a397e069`。

五轮里真正的内容问题只有一类：**task 描述复述了它所依赖的契约，契约改写后没人回读
那份复述**。五项 finding（T-F1、T-F2、T-F3、T-F4、T-F5）全是它的不同实例。关闭方式
也统一：把复述换成指向契约小节的锚点，只保留任务边界与验收级别这类契约里没有的
信息。这使下一次契约改写时，tasks.md 需要跟进的地方从"每条描述"缩小到"锚点是否
被改名"。

本文件是本 topic 的第五份、也是最后一份文档。四份 surface/contract 文档与本文件
现在全部通过，Documents 矩阵五行齐备，实现阶段可以开始；按依赖段，下一个可派发的
是 task 3 `menubar-experience`。

残余不确定性：锚点有效性由本轮人工核对，仓库没有针对 Markdown 内部锚点的检查器——
`bash scripts/check-topic-docs.sh` 校验的是文档集与矩阵，不解析锚点。若后续大量
改写小节标题，这一层没有工具兜底；这是已知且已记录的边界，不是本轮的未关闭项。

证据：`git rev-parse HEAD` -> `e583cdf1959cdabaa897ad32fea8020546e6118c`；
`git hash-object docs/topics/desktop-app/tasks.md` ->
`a397e069900e5e200b35cb2e649972432e3b2d9b`（与 Round 4 声明一致）；
`ux/widget.md:142`、`:225` 与 `architecture.md:252`、`:737` 确认四个被引用锚点存在；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 Task checkpoint

```text
Task checkpoint：desktop-app / tasks.md（blob a397e069，Review Round 5 PASS，
                CEv1 Document gate VERIFIED）
提交建议：tasks.md 与 reviews/tasks.md，加 docs/README.md 中本 topic 的状态行 ——
          按 hunk 排除并行工作的 v0-5-0-contract/tasks.md 与 docs/README.md 中
          不属本 task 的行
推送建议：未解析 —— 当前分支 main，项目未授权在此提交或推送；两者均需显式授权
```

#### 📌 下一步

```text
开发：desktop-app / menubar-experience
```

## Round 6 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/tasks.md` blob
  `fb55815c5d46847b29f9d94cf68355933e87bda6`.
- Reviewer: Codex
- Method: Single-agent formal decomposition Review supplemented by
  `ln-11-plan-reviewer`. Its coverage ledger closed 60/60 items with no
  `PENDING` or `UNPROVEN` row. One frozen independent challenge round used
  three fresh-context reviewers: execution simulation, fresh implementation,
  and adversarial failure analysis. All three returned `REVISE`; retained
  candidates were independently checked against current repository contracts
  and CodeGraph source. Candidates based only on the explicitly historical
  `Next action` block or upstream quick-action wording were rejected.
- Scope: current seven-task breakdown, matrices, file/dependency ownership,
  verification and delivery ceilings, downstream version contract, and tasks
  3–7 feasibility against the five already-PASS design documents.
- Findings:
  - [P1] T6-F1 — `tasks.md:35-48` assigns quality, pricing, and period-scoped
    session statistics to `usage.presentation`. Architecture and current code
    put quality/pricing under `data.usage.presentation`, but sessions under
    `data.sessions.periods.items[]` with a separate producer/decoder path. ->
    Split task 3 by those exact owners and name both Go/Swift DTO, fixture, and
    decoder boundaries.
  - [P1] T6-F2 — tasks 3–7 name behavior and a verification level but no
    owned/created files, violating `docs/README.md:533-538`. The work overlaps
    Go producers, fixtures, Swift wire/cache types, Xcode project, localization,
    Widget target, packaging/workflows, specs, and manual. -> Add
    `Files/creates` to every unfinished task, assigning shared-file hunks and
    new paths to one owner.
  - [P1] T6-F3 — only task 4 declares a current dependency. Task 5 consumes task
    3's producer through the App Group projection but does not depend on task 3;
    task 6 packages the App and Widget but does not depend on tasks 4/5. The
    later dependency prose is explicitly historical. -> Put the current graph
    in task definitions: fixed 1/2 -> 3; 4 -> 2/3; 5 -> 2/3; 6 -> 4/5; 7 -> all
    prior tasks Review PASS.
  - [P1] T6-F4 — task 6 literally signs, notarizes, publishes, and opens a Cask
    path while the same file, project policy, and version contract deny normal
    development certificate, secret, tap, publication, installation, and
    external-distribution authority. -> Make task 6 implement and isolated-test
    the automation; reserve real signing/notarization/upload/tap/publication for
    separately authorized exact-SHA workflows.
  - [P1] T6-F5 — task 7 asks development to close all review records and
    reconcile wire/menu-bar/widget/packaging/distribution, omits the separately
    reviewed settings/preferences surface, and gives no rule for reusing task
    6's L4 identity evidence under its L2 gate. -> Treat prior PASS records as a
    precondition, leave task 7's own closure to independent Review, add
    settings/preferences, and require L0 docs plus unchanged task 6 exact-state
    evidence reuse.
  - [P1] T6-F6 — `v0-5-0-contract/tasks.md:31,42` still says the desktop topic
    has six tasks while the current matrix and README have seven after adding
    `presentation-period-scoping`. -> Update both version-scope statements to
    seven without changing whole-topic inclusion.
  - [P2] T6-F7 — delivered task 1 still says to document update-check
    connectivity when it becomes real (`tasks.md:20-21`), while every current
    contract withdraws desktop update checks and network requests. -> Replace
    the stale bullet with preservation of the no-network boundary without
    reopening task 1 implementation or prior Review PASS.
- Evidence: current HEAD/blob above; `docs/README.md:533-538` readiness rule;
  current requirements/UX/architecture and version-contract comparisons;
  CodeGraph ownership paths for usage/session producers, fixtures, Swift
  decoders/App Group projection and tests; Makefile/scripts show L2/L3/L4
  mechanisms exist. `bash scripts/check-topic-docs.sh` passed in the frozen
  challenge and again after the formal artifacts were written; `make
  check-whitespace` and `git diff --check` -> exit 0. No external source was
  needed because findings concern repository-owned scope, authority, and data
  ownership.
- Plan-reviewer verdict: REVISE
- Verdict: REOPEN

Checklist: 60/60 complete

Incomplete: None

## 📋 评审报告

📊 综合评分：4/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`tasks.md:35`] T6-F1：task 3 把 period-scoped sessions 错归到
`usage.presentation`。
- 行为风险：实现会复制 session schema，或与 `data.sessions.periods.items[]` 形成两个
  owner。
- 证据：architecture 与当前 Go 分别把 usage、sessions 放在独立路径。
💡 有界修复：按两个 wire owner 拆写 task 3，并点名各自 Go/Swift DTO、fixture、decoder。

[`tasks.md:35`] T6-F2：tasks 3–7 没有文件/新文件 ownership。
- 行为风险：多个 task 会重叠修改 producer、fixtures、Swift types、Xcode project、
  localization 和 release scripts，无法形成安全 task commit 或判断遗漏。
- 证据：README readiness 明确要求 anchor、files、verification level；当前缺 files。
💡 有界修复：为每个 task 增加 `Files/creates`，共享文件按 hunk/owner 分配。

[`tasks.md:84`] T6-F3：当前 dependency graph 不完整。
- 行为风险：task 5 可在 producer task 3 前启动，task 6 可在 App/Widget 未完成时启动。
- 证据：只有 task 4 写了 dependency；Widget projection 消费 task 3，distribution 打包
  tasks 4/5。
💡 有界修复：在 task 段写出 fixed 1/2 -> 3 -> 4/5 -> 6 -> 7。

[`tasks.md:106`] T6-F4：task 6 混合实现与未授权真实发布。
- 行为风险：普通 `开发` 无法 literal 完成外部动作；执行会越过 authority ceiling。
- 证据：task 写 publish/sign/notarize；同文件与 policy 明确不授权。
💡 有界修复：只实现并隔离验证 automation；真实动作留给单独授权 exact-SHA workflow。

[`tasks.md:119`] T6-F5：task 7 reconciliation/Review ownership 不可执行。
- 行为风险：settings 可被漏出 living specs，development 被要求关闭自己的 Review，
  identity 又可能在 L2 下无证据地重断言。
- 证据：列表不点名 settings；Review closure 与独立 Reviewer 冲突；identity 属于 task 6
  L4 exact state。
💡 有界修复：prior PASS 作为 precondition，补 settings/preferences，明确 L0 与 L4
  evidence reuse。

[`v0-5-0-contract/tasks.md:31`] T6-F6：version contract 仍把 desktop 记为六任务。
- 行为风险：version closure 可漏算新增 producer task或错误判断 readiness。
- 证据：desktop matrix 与 README 是七任务，version contract 两处仍写 six。
💡 有界修复：两处改为 seven，保留 whole-topic inclusion。

### 🟡 建议改进——推荐

[`tasks.md:20`] T6-F7：task 1 保留已撤销的 update-check scope。
- 证据：requirements、architecture、task 4 和 change table 均撤销更新检查与 desktop
  network request。
💡 有界改进：改为维护 no-network boundary，不重开 task 1 implementation/PASS。

### 🟢 优点

- 七个 anchor 唯一，Document matrix 与磁盘/requirements surface 集合一致。
- Task 3 独立承接 Go producer，tasks 4/5 使用 L3、task 6 使用 L4 的风险分层方向合理。
- Update-check 移除、work-signal 延后、Widget privacy 和 version-wide spec raise 分离
  的总体 scope 决策正确。
- 三视角 challenge 未发现缺失的第八个产品 task；问题集中在 ownership、依赖、
  authority 和跨 unit 计数，无需重做整个分解。

### 📝 摘要

评审对象为 HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56` 与 `tasks.md`
blob `fb55815c5d46847b29f9d94cf68355933e87bda6`。七-task 方向成立，但 T6-F1～
T6-F6 使 owner、文件边界、依赖、发布 authority、最终 reconciliation 和 version
scope 仍不可执行；T6-F7 是不得递延的小型 stale scope。`ln-11` closure 为 60/60，
三名独立 reviewer 均返回 `REVISE`，故正式结论为 FAIL/REOPEN。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / T6-F1 T6-F2 T6-F3 T6-F4 T6-F5 T6-F6 T6-F7
```

## Round 7 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 6's seven open findings.
  `docs/topics/desktop-app/tasks.md` is now blob
  `008df2eec006f588f103497a2a7a5b191d964de3`, from
  `fb55815c5d46847b29f9d94cf68355933e87bda6`.
  `docs/topics/v0-5-0-contract/tasks.md` is now blob
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`. HEAD is still
  `58fe5d300c5af572adef81a69a856a6aef9cea56`. No code, test, configuration, or
  fixture changed; this is a decomposition-document repair.
- Reviewer: claude-code (repair round for Round 6's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: T6-F1 through T6-F7 as named in the repair command. Tasks 1 and 2 are
  already Review PASS; only T6-F7 touches task 1, and it touches its scope
  statement rather than its implementation or its verdict.

- Round 6 findings, dispositions:
  - **T6-F1** period-scoped sessions mis-assigned to `usage.presentation` ->
    **Fixed.** Task 3 now states two wire owners in a table, one row per
    sub-change, each naming its wire location, its Go producer, and its Swift
    decoder: `quality`/`pricing` at `data.usage.presentation.scopes[]` produced by
    `internal/usage/presentation.go` and decoded by `DesktopUsageQualityV1` /
    `DesktopUsagePricingV1`; `sessions.periods.items[]` at `data.sessions` produced
    by `internal/desktop/desktop.go`'s `SessionsSnapshot` and decoded by
    `DesktopSessionsSnapshotV1`. The task also restates that the bounded recent
    list stays a recent list and the panel's statistics come from the period
    records, which is the specific confusion that would have produced a duplicated
    session schema under usage.
  - **T6-F2** no file ownership on tasks 3–7 -> **Fixed.** Every unfinished task
    now carries `Files` and `Creates`, in the same style as the topics whose
    `tasks.md` has already passed. Shared files are qualified by hunk rather than
    listed whole:
    - `DesktopWire.swift` — task 3 owns the quality, pricing, and sessions DTO
      hunks; task 2 owns the rest, as reviewed.
    - `EmbeddedHelperRunner.swift` — task 4 owns the refresh-coordinator and
      presentation-state hunks; the process and capture contract stays task 2's.
    - `AppGroupSnapshotStore.swift` — task 5 owns the projection hunks that add the
      period-scoped fields; the atomic-write and fail-closed behavior stays task 2's.
    - `project.pbxproj` — task 4 owns app-target hunks, task 5 widget-target hunks,
      task 6 signing and packaging build settings. The document says explicitly that
      whichever lands second rebases and that a whole-file regeneration by either is
      a defect, because that file is the one most likely to be rewritten wholesale
      by a tool and silently swallow another task's hunks.
    - Task 3 additionally records that it owns *no* Swift presentation code and
      stops at the decoded types, and task 4 that it owns neither `DesktopWire.swift`
      nor `AppGroupSnapshotStore.swift`. A negative boundary is what stops two tasks
      from both believing they own a file.
  - **T6-F3** incomplete dependency graph -> **Fixed**, in the task definitions
    rather than as a separate diagram that could drift from them: 3 depends on 1
    and 2 (both PASS, so it is startable now); 4 on 2 and 3; 5 on 2 and 3; 6 on 4
    and 5; 7 on 1 through 6 at Review PASS. Task 5's dependency on 3 carries its
    reason, since it is the one that looks absent: the widget consumes task 3's
    producer *through the App Group projection* rather than by reading the wire, so
    the dependency is real even though the two tasks share no Go file. Tasks 4 and 5
    are also stated to be independent of each other, which the previous text left
    ambiguous.
  - **T6-F4** task 6 mixed implementation with unauthorized real release ->
    **Fixed** by splitting the verb from the act. The task now opens by stating it
    builds and tests the automation and performs no real release action, and says
    why: the Developer ID certificate, the notarization credential, the tap write,
    and the publication decision are each authority this topic does not hold, and
    the version contract reserves the release decision anyway. "Sign, notarize,
    publish" made the task literally uncompletable, which is worse than making it
    smaller — an implementer either stops and asks, or reaches past the ceiling.
    What replaces it: implement the build, the signing *invocation*, the
    notarization and stapling *invocation*, asset assembly, Cask rendering, and
    migration behavior; test all of it in isolation with ad-hoc identities, a
    stubbed notarization response, a local tap fixture, and a temporary `HOME` and
    prefix; verify the install matrix against locally built artifacts, with
    Gatekeeper assessed on a locally signed bundle. A test needing the real
    Developer ID, the real Apple service, or the real tap is out of scope by
    construction. The real actions are named as belonging to the separately
    authorized exact-SHA workflows, and the task's own Review PASS is stated not to
    be a release decision.
  - **T6-F5** task 7 unexecutable -> **Fixed** on all four points the finding
    raised. The settings window and its four preferences are added to the
    reconciliation list, with the reason recorded — a separately reviewed surface
    absent from that list is how a whole surface reaches release without entering
    the living specification. Prior tasks' `PASS` becomes a precondition the task
    *verifies and reports on*, never something it ticks: a record is closed by the
    independent Review that passes it, and task 7 cannot close its own review
    either. Identity reuse is now a rule rather than a gap: identity is an artifact
    property task 6 established at L4 against a content state, so task 7 cites it
    when status, diff, and tree hash prove the tree unchanged, and otherwise hands
    the recheck back to task 6 at task 6's level. The verification level drops from
    L2 to L0 for the same reason — the task changes documentation and reuses
    evidence, and the L2 claim implied a contract test it neither changes nor can
    produce.
  - **T6-F6** version contract still said six tasks -> **Fixed.**
    `v0-5-0-contract/tasks.md:31` and `:42` now read seven. Whole-topic inclusion is
    untouched: the sentence still says all of its tasks ship together and a topic is
    merged whole or not at all. A sweep for other stale counts found none — the two
    remaining matches for "six" are `docs/README.md`'s six *documents*, which is
    correct, and Round 6's own finding text, which is history.
  - **T6-F7** task 1 kept withdrawn update-check scope -> **Fixed.** The bullet
    asking task 1 to document update-check connectivity "when implementation makes
    it real" is replaced by the boundary that actually holds: the snapshot reaches
    no network, neither does any other desktop surface, and what a later task must
    not do is add the first outbound request. The replacement says in one line that
    the check never became real and that task 1's implementation and Review PASS are
    untouched, so the correction cannot be read as reopening a delivered task.

- Verification performed by this repair round, not yet independently confirmed:
  - Every existing path named in the new `Files` lists was checked to exist on
    disk — 40 paths, all present. A `Files` list that names a path that is not
    there would be a new defect of exactly the kind T6-F2 is about, so this was
    checked mechanically rather than by reading.
  - The two wire owners in T6-F1's table were read back against the source:
    `internal/desktop/desktop.go:53,108,111` shows `Usage.Presentation` typed as
    `usage.PresentationReport` while `Sessions` is a separate `SessionsSnapshot`,
    and `DesktopWire.swift:91,210,246,275-276,372,390` shows the matching split on
    the Swift side. The architecture table at
    `architecture.md#shape-of-the-two-provisioned-changes` names the same four
    fields, so the task now agrees with both the contract and the code.
  - The dependency graph was printed back from the document and compared to the
    shape the finding specified; it matches on all five rows.
  - Verification level L0: documentation only, no code, test, configuration, or
    fixture touched, so no product test was run and none is claimed.
    `make check-whitespace`, `bash scripts/check-topic-docs.sh`, and
    `git diff --check` all exit 0.
  - Not claimed: that the decomposition is now complete. Round 6's three-perspective
    challenge found no missing eighth product task, and this repair adds none; but
    whether the ownership, dependencies, authority, and reconciliation now hold
    together is the Re-review's question, not something a repair can assert about
    itself.
- Evidence: `git hash-object docs/topics/desktop-app/tasks.md` ->
  `008df2eec006f588f103497a2a7a5b191d964de3`;
  `git hash-object docs/topics/v0-5-0-contract/tasks.md` ->
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`; the path-existence sweep and source
  cross-checks above; `make check-whitespace`,
  `bash scripts/check-topic-docs.sh`, and `git diff --check` -> exit 0. Two
  documents changed, in two topics; no other file.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md / Round 7
```

## Round 8 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/tasks.md` blob
  `008df2eec006f588f103497a2a7a5b191d964de3`;
  `docs/topics/v0-5-0-contract/tasks.md` blob
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`. Both blobs match what Round 7
  claimed, so this Re-review judges exactly the repaired content state.
- Reviewer: claude-code（独立复评，未修改任何被评审文档或产品代码）
- Method: 单 agent 有界复评。两个动作：把 Round 6 的七项发现逐条回到修复后的文本
  与其声称的依据（architecture 契约、Go/Swift 源码、磁盘路径）核对；再对修复**新
  引入或未覆盖**的边界做一次针对性检查，重点是 T6-F2 所属的那一类——文件 owner
  是否真的无缺口。
- Scope: T6-F1～T6-F7 的关闭状态；tasks 3–7 的 `Files`/`Creates`/依赖是否自洽；
  与 `ux/widget.md`、`ux/settings.md`、`architecture.md` 的一致性；
  `v0-5-0-contract/tasks.md` 的计数。
- Evidence: 43 条 `Files`/`Creates` 路径的存在性逐条机械核对；
  `git cat-file -e HEAD:<path>` 对其中 5 条做提交基线核对；
  `internal/desktop/desktop.go:53,108,111,420`、
  `internal/usage/presentation.go:17-134`、
  `apps/macos/AgentDeckShared/DesktopWire.swift:91,275-276,372,390,446`、
  `architecture.md:818-827,861-871`；`ux/widget.md:112,236,398`；
  `make check-whitespace`、`bash scripts/check-topic-docs.sh`、
  `git diff --check` 均 exit 0。未运行产品测试，本轮为 L0 文档复评。

### Round 6 发现的关闭判定

- **T6-F1 已关闭。** task 3 现以表格给出两个 wire owner，逐条与源码一致：
  `data.usage.presentation.scopes[].quality.items[]` / `…pricing.items[]` 由
  `internal/usage/presentation.go` 产出、由 `DesktopUsageQualityV1` /
  `DesktopUsagePricingV1` 解码；`data.sessions.periods.items[]` 由
  `internal/desktop/desktop.go` 的 `SessionsSnapshot` 产出、由
  `DesktopSessionsSnapshotV1` 解码。`architecture.md:865-871` 的四行字段表与该表
  逐字段吻合，`sessions.items[]` 保持 recent list 的表述亦一致。
- **T6-F2 部分关闭。** tasks 3–7 都有了 `Files`/`Creates`，`DesktopWire.swift`、
  `EmbeddedHelperRunner.swift`、`AppGroupSnapshotStore.swift`、`project.pbxproj`
  四份共享文件都按 hunk 分派并写了否定边界。但仍有一处无 owner 的产物，见 R8-F1。
- **T6-F3 已关闭。** 依赖写在 task 定义内：3←1,2；4←2,3；5←2,3；6←4,5；7←1–6。
  与发现给定的五行图形完全一致，且 task 5 依赖 task 3 的理由（经 App Group
  projection 消费而非直读 wire）被写出，tasks 4/5 互不依赖也已声明。
- **T6-F4 已关闭。** task 6 以"构建并测试自动化、不执行任何真实发布动作"开头，
  实现项写成 signing/notarization 的 *invocation*，隔离测试用 ad-hoc 身份、桩化
  公证响应、本地 tap fixture 与临时 `HOME`，并明示真实动作属单独授权的 exact-SHA
  workflow、本 task 的 Review PASS 不是发布决定。authority ceiling 不再被跨越。
- **T6-F5 已关闭。** 四点全部落地：settings 及其四项 preferences 进入
  reconciliation 列表；prior PASS 变为 precondition 且明确 task 7 既不能关闭他人
  也不能关闭自己的 Review；identity 复用规则绑定 task 6 的 L4 exact state 并要求
  status/diff/tree hash 证明未变；验证级别由 L2 降为 L0 并说明理由。
- **T6-F6 已关闭。** `v0-5-0-contract/tasks.md:31,42` 均为 seven，whole-topic
  inclusion 措辞未动。全仓 `six` 复核未见其他陈旧计数。
- **T6-F7 已关闭。** task 1 的 update-check 条目替换为 no-network boundary，并声明
  该修正不重开 task 1 的实现与 Review PASS。与 task 4 的"不交付任何更新检查"一致。

### 🔴 严重问题——必须修复

[`docs/topics/desktop-app/tasks.md:152-168`] R8-F1：task 5 的 widget 本地化产物没有
owner，而这正是 T6-F2 要消除的那一类缺口。
- 行为风险：`ux/widget.md:398` 把"`en` 与 `zh-Hans` 在每个尺寸都不截断"列为验收
  条件，`:112` 与 `:236` 给出 App Intent 参数值与全部 copy 的双语对照。仓库中唯一的
  字符串目录是 `apps/macos/AgentDeckApp/Localizable.xcstrings`，它被 task 4 独占列在
  `Files` 中，而 task 4 的交付条只说"为**两个 surface**（菜单栏与设置）提供中英文
  字符串"，不含 widget。task 5 的 `Files` 与 `Creates` 都没有任何本地化目录，任务正文
  也无一条本地化要求。结果只有两种：task 5 去改 task 4 独占的文件（T6-F2 明令禁止的
  重叠），或 widget 以未本地化状态交付并在 Review 时撞上 `ux/widget.md` 的验收条件 8。
- 证据：`tasks.md:110`（task 4 的"both surfaces"）、`:115`（xcstrings 归 task 4）、
  `:152-168`（task 5 的 Files/Creates，无本地化产物，正文无本地化条目）；
  `ux/widget.md:112,236,398`；全仓仅一个字符串目录（`find apps/macos -name '*.xcstrings'`）。
- 💡 有界修复：在 task 5 的 `Creates` 中加入 widget target 自己的字符串目录（与其
  target 源码同属新建物），并加一条交付项要求它按 `ux/widget.md` 的 copy 表提供
  `en`/`zh-Hans`；或改为把 widget 字符串并入 task 4 的目录并在两个 task 中写明该
  文件的 hunk 归属。二者取一即可，但必须择一写死——现状是两个 task 都没有它。

[`docs/topics/desktop-app/tasks.md:76-79`] R8-F2：task 3 的 `Creates: no new file`
在提交基线上不成立，且该断言被用作拆分理由。
- 行为风险：`Files` 列出的 `internal/usage/presentation.go`、
  `internal/usage/presentation_test.go`、`desktop/fixtures/v1/snapshot-legacy.json`
  在 HEAD `58fe5d3` 中并不存在，它们只是工作区中未提交的在制品。tasks.md 是派发文
  档，其基线是被评审的提交状态；把待新建的 producer 写成"已存在的 producer"，并据此
  断言"no new file. Every change lands in an existing producer, fixture, or decoder,
  which is itself the argument for the split above"，会让实现者以为只需改动既有文件，
  也让 task commit 的边界判断少算三个新文件。Round 7 的"40 paths, all present"核对
  是在脏工作区上做的，因此没有暴露这一点。
- 证据：`git cat-file -e HEAD:internal/usage/presentation.go` 等三条均失败；
  `git status --short` 显示三者为 `??`；`tasks.md:76-79` 的 `Creates` 断言。
  同类问题也出现在 task 4：`apps/macos/AgentDeckApp/Localizable.xcstrings` 与
  `Assets.xcassets/` 同样不在 HEAD 中，却列在 `Files` 而非 `Creates`。
- 💡 有界修复：把这五条按提交基线移入各自 task 的 `Creates`，或在 `Files` 处注明
  它们当前是未提交在制品、由本 task 首次提交；并把 task 3 的拆分理由改写为不依赖
  "无新文件"这一事实。

### 🟡 建议改进——推荐

[`docs/topics/desktop-app/tasks.md:481-487`] R8-F3：Tasks 矩阵前言仍写"tasks 3
through 6 are re-derived"，且把再拆分说成尚未发生。
- 证据：该段紧接七行矩阵之上，而 2026-08-18 的再拆分把
  `presentation-period-scoping` 插入为 task 3，被重新推导的集合实为 tasks 3–7；
  `menubar-experience` 已是 task 4。同段又称"decomposition happens properly once
  the Documents matrix is green"，而再拆分已在下一节 `What changed in the
  decomposition (2026-08-18)` 中完成。Round 7 的陈旧计数扫描只搜了 `six` 一词，
  区间写法因此漏网。
- 💡 有界改进：把区间改为 tasks 3–7，并把该段时态改为陈述已完成的再拆分，指向下
  一节；不改动矩阵本身。

### 🟢 优点

- 两个 wire owner 的表格是本轮最有价值的修复：它把 owner、wire 路径、Go producer、
  Swift decoder 四列并置，逐条可与源码对照，我按该表核对时无一处不符。
- 否定边界（task 3 "Owns no Swift presentation code"、task 4 "Does not own
  DesktopWire.swift or AppGroupSnapshotStore.swift"）是防止双 owner 的正确手法，
  也是 R8-F1 之所以能被一眼看出的原因——缺的那一个恰好没有任何一方声明。
- `project.pbxproj` 的三方 hunk 分派加"整文件重新生成即缺陷"的约定，针对的是该文件
  最真实的失效形态。
- task 6 把动词与行为分离（invocation 与真实发布），比单纯缩小 scope 更准确。
- task 7 把 identity 证据复用写成规则而非留白，且据此下调验证级别，方向正确。

### 📝 摘要

Round 7 的七项修复中六项（T6-F1、T6-F3～T6-F7）经独立核对确认关闭，T6-F2 只关闭了
大部分：四份共享文件的 hunk 分派成立，但 widget 的本地化产物仍无 owner（R8-F1），
且五条列在 `Files` 中的路径在提交基线上并不存在、task 3 更据此断言"no new file"
（R8-F2）。两者都属 T6-F2 本身的问题域，因此不能作为新一轮的遗留递延。R8-F3 是同型
的小型陈旧区间。文档集审计与 L0 检查全部通过，七-task 方向与依赖图不再有异议。

- Verdict: FAIL — `Documents` 矩阵中 `tasks.md` 的 `Review` 单元格保持未勾选。
  本轮对被评审文档的唯一改动是 `tasks.md` 的 `Current tasks document review`
  当前评审状态段落（本轮裁定后的状态同步，判定依据取自评审前的 blob
  `008df2eec006f588f103497a2a7a5b191d964de3`）；未改动任何 task 定义、矩阵单元格、
  产品代码、测试或配置，未关闭任何 gate，不授权提交或推送。Beads
  `ad-desktop-doc-tasks-design` 回到 `in_progress`，`round-3`，处置已入 comment。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / R8-F1 R8-F2 R8-F3
```

## Round 9 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 8's three open findings.
  `docs/topics/desktop-app/tasks.md` is now blob
  `8462223a8b4b770194e0a65a9ea183e1760e168f`, from
  `008df2eec006f588f103497a2a7a5b191d964de3` — the blob Round 8 judged. HEAD is
  still `58fe5d300c5af572adef81a69a856a6aef9cea56`. No other file changed; no
  code, test, configuration, or fixture.
- Reviewer: claude-code (repair round for Round 8's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: R8-F1, R8-F2, R8-F3. Round 8 confirmed T6-F1 and T6-F3 through T6-F7
  closed and T6-F2 partially closed; this round finishes T6-F2's remaining gap and
  the two other findings, and revisits nothing Round 8 confirmed.

- Round 8 findings, dispositions:
  - **R8-F1** the widget's localized strings had no owner -> **Fixed** by giving
    the widget its own catalog, the first of the two options the finding offered.
    Task 5 now `Creates`
    `apps/macos/AgentDeckWidget/Localizable.xcstrings` alongside its target
    sources, and carries a delivery bullet requiring `en` and `zh-Hans` for the
    `Copy` table and the `Client` App Intent parameter values, naming
    `ux/widget.md`'s acceptance condition 8 as the condition it satisfies. Task 4's
    `apps/macos/AgentDeckApp/Localizable.xcstrings` is stated to be its exclusively,
    and each task says the other's catalog is not its to edit.
    - Why two catalogs rather than one shared file: a shared catalog is precisely
      the two-tasks-one-file overlap the `Files` lists exist to prevent, and an
      extension target does not read the containing app's catalog by virtue of
      membership. The document also says that strings which turn out identical
      between the two surfaces stay duplicated rather than hoisted, so nobody
      "tidies" the two catalogs back into the overlap later.
  - **R8-F2** `Files` listed paths absent from the commit baseline, and the split
    argument rested on that error -> **Fixed**, and the underlying practice is now
    stated rather than left implicit. Task 3 opens its file lists with the rule:
    the lists are stated against the commit baseline, a path existing only as
    uncommitted work in progress is a file the task *creates*, and checking against
    a dirty tree hides exactly that distinction. Then the five paths move:
    - task 3 `Creates` `internal/usage/presentation.go`,
      `internal/usage/presentation_test.go`, and
      `desktop/fixtures/v1/snapshot-legacy.json`, with its commit boundary stated
      as covering three new files rather than none;
    - task 4 `Creates` `apps/macos/AgentDeckApp/Localizable.xcstrings` and
      `apps/macos/AgentDeckApp/Assets.xcassets/`, both moved out of `Files`.
    - The split argument is rewritten to rest on the two sub-changes having
      different owners — different producer, different DTO family, different
      decoder — and says explicitly that this is the right reason: the sub-changes
      would still need separating if they shared a file, and would still be
      separate if both were new. The `Creates: no new file` sentence is gone.
  - **R8-F3** stale range and tense above the Tasks matrix -> **Fixed.** The
    paragraph now says the re-derivation happened in the 2026-08-18 pass, points at
    the `What changed in the decomposition (2026-08-18)` section that records it,
    gives the range as tasks 3 through 7, and names the two consequences that make
    the range checkable — `presentation-period-scoping` became task 3 and
    `menubar-experience` moved to task 4. The matrix itself is untouched.

- One contradiction of my own found while re-checking, and fixed here rather than
  left for the next round: task 7 listed
  `reviews/desktop-app-contract.md` in **both** `Files` and `Creates`. It is a file
  that does not exist yet, so it belongs only in `Creates`; `Files` now holds just
  the two specs and the `docs/README.md` stage row.

- Verification performed by this repair round, not yet independently confirmed:
  - The baseline check Round 7 got wrong is now mechanical and runs against the
    commit, not the working tree: every path in a `Files` list must resolve under
    `git cat-file -e HEAD:<path>`, and every path in a `Creates` list must not. That
    check now reports consistent for all five tasks. Running it is what surfaced the
    task 7 duplication above — the same class of error, caught by the same check,
    which is the argument for making it mechanical instead of read-by-eye.
    Directory-shaped entries such as "sources under `apps/macos/AgentDeckApp/`" are
    locations rather than claimed-new paths and are excluded from the assertion.
  - The five paths R8-F2 named were each confirmed absent from `HEAD` and present
    as `??` in the working tree, so their move to `Creates` is grounded rather than
    assumed.
  - Catalog ownership is now stated in exactly two places, one per task, naming two
    different files; grepping `xcstrings` returns those two plus task 4's
    exclusivity sentence, and no third claimant.
  - Verification level L0: documentation only. `make check-whitespace`,
    `bash scripts/check-topic-docs.sh`, and `git diff --check` all exit 0. No
    product test was run and none is claimed.
  - Not claimed: that no further ownership gap exists. R8-F1 was found by asking
    which artifact `ux/widget.md` requires that no task names, and the mechanical
    check above cannot ask that question — it verifies the paths that are listed,
    not the ones that should be. That remains the Re-review's question.
- Evidence: `git hash-object docs/topics/desktop-app/tasks.md` ->
  `8462223a8b4b770194e0a65a9ea183e1760e168f`; baseline `Files`/`Creates` check
  consistent across tasks 3–7; `git cat-file -e HEAD:<path>` failing for the five
  paths now under `Creates`; `find apps/macos -name '*.xcstrings'` confirming the
  repository still holds exactly one catalog, which is why the second is a `Creates`
  entry; `make check-whitespace`, `bash scripts/check-topic-docs.sh`, and
  `git diff --check` -> exit 0. One document changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md / Round 9
```

## Round 10 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/tasks.md` blob
  `8462223a8b4b770194e0a65a9ea183e1760e168f`（Round 9 声称的修复后 blob，一致）;
  `docs/topics/v0-5-0-contract/tasks.md` blob
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`（自 Round 8 未变）。
- Reviewer: claude-code（独立复评）
- Method: 单 agent 有界复评，两步。第一步把 R8-F1～R8-F3 逐条回到修复后的文本与其
  依据核对，并**自行重跑** Round 9 提出的基线机械核对而不是采信其结论。第二步做
  Round 9 自己声明无法回答的那个问题——"契约要求的产物中，有没有哪一个是任何 task
  都没有点名的"——沿 `architecture.md` 的 Build and configuration 一节、SwiftPM
  manifest 与 Xcode target 表反查，而不是沿已列出的路径正查。
- Scope: R8-F1～R8-F3 的关闭状态；tasks 3–7 的 `Files`/`Creates` 基线一致性；
  Round 9 自行修正的 task 7 重复项；未被任何 task 认领的交付产物。
- Evidence: 自行执行的基线核对——38 条 `Files` 路径经
  `git cat-file -e HEAD:<path>` 全部命中，7 条 `Creates` 路径全部落空，无一违反
  Round 9 立下的规则；`apps/macos/Package.swift:24-28`；
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj:191-207`；
  `architecture.md:698-704`；`scripts/check-privacy.sh:4-41`；
  `grep -n 'xcstrings' tasks.md` 仅三处、无第三方声索；
  `make check-whitespace`、`bash scripts/check-topic-docs.sh`、`git diff --check`
  均 exit 0。L0 文档复评，未运行产品测试，未修改任何 task 定义或产品文件。

### Round 8 发现的关闭判定

- **R8-F1 已关闭。** task 5 `Creates`
  `apps/macos/AgentDeckWidget/Localizable.xcstrings`，并新增一条交付项，点名
  `ux/widget.md` 的 `Copy` 表与 `Client` App Intent 参数值，且把验收条件 8 认作本
  task 的义务。两侧互斥声明齐备：task 4 声明 app 目录归其独有、task 5 声明不编辑
  它，task 5 声明 widget 目录归其所有、task 4 同样声明不编辑。全文件 `xcstrings`
  仅三处出现，无第三方声索。"相同字符串保持重复而不上提"一句堵住了日后把两个目录
  合并回重叠状态的路径，这是修复中考虑得比发现本身更远的一处。
- **R8-F2 已关闭，且关闭方式比发现要求的更强。** 五条路径已按提交基线归位：
  task 3 `Creates` 三条 Go/fixture 新文件并把 commit boundary 改写为"三个新文件而
  非零个"，task 4 `Creates` 两条 app 资源。task 3 更把做法写成规则——"文件清单以提交
  基线陈述，只存在于未提交工作区的路径是本 task 创建的文件"——这正是 Round 7 出错的
  地方被制度化。我按该规则独立重跑 38+7 条核对，无一违反。拆分理由已改写为依据两个
  不同 owner 而非"无新文件"，并明确说明后者既错也是错的理由。
- **R8-F3 已关闭。** 矩阵前言改为陈述 2026-08-18 已完成的再拆分、指向记录该次改动的
  小节、区间改为 tasks 3–7，并写出两个可核对的推论（`presentation-period-scoping`
  成为 task 3、`menubar-experience` 移至 task 4）。矩阵七行未动。
- **Round 9 自查发现的 task 7 重复项已关闭。** `reviews/desktop-app-contract.md`
  现仅在 `Creates` 中，`Files` 只余两份 spec 与 `docs/README.md` 的阶段行。该问题由
  Round 9 新立的机械核对捕获，这是把眼看改成机检的直接收益。

### 🔴 严重问题——必须修复

[`docs/topics/desktop-app/tasks.md:137-139,184-186`] R10-F1：tasks 4 与 5 都把新测试
放进 `apps/macos/AgentDeckTests/`，但该目录在两套构建系统里都只是
`AgentDeckShared` 的测试面，而扩容它所需的 `Package.swift` 与 test-target hunk
不属于任何 task。
- 行为风险：`apps/macos/Package.swift:24-28` 的 `AgentDeckSharedTests` 以
  `path: "AgentDeckTests"`、`dependencies: ["AgentDeckShared"]` 定义；
  `project.pbxproj:191-207` 中唯一的 `com.apple.product-type.bundle.unit-test`
  目标同样只依赖 `AgentDeckShared`。task 4 `Creates` 的设置窗口与菜单栏项源码在
  `apps/macos/AgentDeckApp/`，task 5 `Creates` 的 timeline provider、App Intent
  配置与四类 widget 视图在新建的 `AgentDeckWidget/`，两者都不在该测试目标的可见
  范围内。于是实现者只有三条路：改 `apps/macos/Package.swift`——全文件在 tasks.md
  中零次出现，无人认领；改 `project.pbxproj` 的 test-target hunk——task 4 认领
  app-target hunk、task 5 认领 widget-target hunk、task 6 认领签名与打包 build
  settings，test-target hunk 恰好是三者都不覆盖的那一类；或者干脆不写这些测试，
  而 `architecture.md:702` 明写工程须定义"isolated unit-test targets"（复数）。
  这与 T6-F2、R8-F1 同型：契约要求的产物无 owner，差别只在这次缺的是测试目标的
  归属而不是文件本身。
- 证据：`apps/macos/Package.swift:24-28`；
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj:191-207`（唯一 unit-test 目标
  `AgentDeckSharedTests`，单一 target dependency）；`architecture.md:698-704`；
  `tasks.md:137-139`（task 4 的 `Creates`）、`:184-186`（task 5 的 `Creates`）；
  `grep -n 'Package.swift\|AgentDeckSharedTests\|test-target' tasks.md` 无匹配。
  已交付的 tasks 1/2 无 `Files` 清单，且 task 2 的 Review PASS 绑定其当时的内容
  状态，不能被读成对新目标的持续授权。
- 💡 有界修复：择一写死并落到 `Files`/`Creates` 上。要么把 `apps/macos/Package.swift`
  与 `project.pbxproj` 的 test-target hunk 明确分派（例如 task 4 建 app 测试目标、
  task 5 建 widget 测试目标，各自认领自己那一份 hunk 与 manifest 段落，并沿用已有
  的"后落地者 rebase、整文件重生成即缺陷"约定）；要么明确规定这两个 task 的可测逻辑
  一律落在 `AgentDeckShared`、`AgentDeckTests/` 因此无需扩容，同时把两个 `Creates`
  里的"their tests"改写为对 Shared 侧行为的测试。前者更贴合 `architecture.md:702`
  的复数表述。

### 🟡 建议改进——推荐

[`docs/topics/desktop-app/tasks.md:180,190`] R10-F2：task 5 的沙箱证明没有可执行它的
产物。
- 证据：task 5 要求"Prove the Widget cannot read AgentDeck databases, credentials,
  client config, or raw session sources"，验证级别写作"L3 including extension
  sandbox and privacy checks"，而它在 `Files` 中为此点名的唯一产物是
  `scripts/check-privacy.sh`。该脚本（`:4-41`）枚举仓库文件并按
  `AKIA…`/`BEGIN … PRIVATE KEY`/`sk-…`/`ghp_…` 四个模式扫描凭据字面量，与扩展进程
  能否读取数据库、凭据或原始 session 源无关。执行沙箱证明的那个产物——无论是新脚本、
  测试用例，还是 `AgentDeckVerification` 那个 bundle-boundary 可执行文件的扩展——在
  `Files` 与 `Creates` 中都不存在。
- 💡 有界改进：在 task 5 的 `Creates` 中点名承担该证明的产物，并把
  `check-privacy.sh` 的角色写清（仓库凭据扫描，非沙箱证明）；若沙箱证明确定由手工
  验收承担，则明写它是手工步骤而非脚本，以免 L3 的"privacy checks"被读成
  `check-privacy.sh` 已经覆盖。

### 🟢 优点

- Round 9 把 Round 7 出错的核对方式改成了机械规则并写进文档正文，而不是只修五条
  路径。我独立重跑 38+7 条，规则自洽且当前无违反——这是本记录里第一次出现"下一轮
  可以低成本复核"的核对方式。
- 该机械核对当轮就抓到了 Round 9 自己引入之外的一处旧错（task 7 的 `Files`/`Creates`
  重复），并且修复轮主动记录了它而非留给下一轮，这是正确的处理。
- 两个字符串目录的取舍写出了理由与反向约束（相同字符串保持重复），使该边界在后续
  "整理"压力下仍可辩护。
- Round 9 明确声明自己无法回答"是否还存在其他 owner 缺口"，把该问题交给复评。
  R10-F1 正是沿这条提示反查得到的，说明该声明不是免责套话。

### 📝 摘要

R8-F1、R8-F2、R8-F3 三项经独立核对确认关闭，Round 9 自查修正的 task 7 重复项亦已
关闭；`Files`/`Creates` 的提交基线一致性由我重跑的 38+7 条核对独立确认。剩余问题
不在已记录的发现里，而在 Round 9 明确留给复评的那一类：R10-F1 指出 tasks 4 与 5 的
新测试所需的测试目标扩容（`Package.swift` 与 `project.pbxproj` 的 test-target
hunk）无人认领，与 T6-F2、R8-F1 同型；R10-F2 指出 task 5 的扩展沙箱证明没有可执行
它的产物。七-task 方向、依赖图、wire owner、发布 authority 与版本计数均无新异议。

- Verdict: FAIL — `Documents` 矩阵中 `tasks.md` 的 `Review` 单元格保持未勾选。
  本轮对被评审文档的唯一改动是 `tasks.md` 的当前评审状态段落（裁定后的状态同步，
  判定依据取自评审前的 blob `8462223a8b4b770194e0a65a9ea183e1760e168f`）；未改动任何
  task 定义、矩阵单元格、产品代码、测试或配置，未关闭任何 gate，不授权提交或推送。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / R10-F1 R10-F2
```

## Round 11 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 10's two open findings.
  `docs/topics/desktop-app/tasks.md` is now blob
  `34912e6f518150ed6d2d0cec6038a41762d347ac`, from
  `8462223a8b4b770194e0a65a9ea183e1760e168f` — the blob Round 10 judged. HEAD is
  still `58fe5d300c5af572adef81a69a856a6aef9cea56`. No other file changed; no code,
  test, configuration, or fixture.
- Reviewer: claude-code (repair round for Round 10's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: R10-F1 and R10-F2. Round 10 confirmed R8-F1 through R8-F3 and the task 7
  duplication closed, and re-ran the baseline check itself; none of that is
  revisited here.

- Round 10 findings, dispositions:
  - **R10-F1** the test-target expansion tasks 4 and 5 need had no owner ->
    **Fixed** by the first of the two options, which is the one
    `architecture.md`'s plural "isolated unit-test targets" supports:
    - Task 4 `Creates` the `AgentDeckAppTests` Xcode target with sources under
      `apps/macos/AgentDeckAppTests/`; task 5 `Creates` `AgentDeckWidgetTests`
      under `apps/macos/AgentDeckWidgetTests/`. Each owns its own test-target hunks
      in `project.pbxproj`, stated in its `Files` entry rather than left to be
      inferred.
    - The `project.pbxproj` convention is corrected from three ways to **four**:
      task 4 owns the app target and `AgentDeckAppTests`, task 5 the widget target
      and `AgentDeckWidgetTests`, task 6 the signing and packaging build settings.
      The test-target hunks were exactly the class none of the three previously
      covered, which is what made the gap invisible.
    - Both tasks now say explicitly that their tests do **not** go in
      `apps/macos/AgentDeckTests/`, with the reason: that directory is bound by
      `path: "AgentDeckTests"` to `AgentDeckSharedTests`, which links
      `AgentDeckShared` only. The existing files each task lists there stay, because
      what they test — the refresh coordinator, the helper runner, the App Group
      store — is Shared code. That distinction is the load-bearing part: the old
      wording was not merely vague, it named a directory that cannot compile the
      sources the tasks create.
    - `apps/macos/Package.swift` is now named and assigned to **no task**, as a
      decision rather than silence: the package exists only to exercise
      `AgentDeckShared` without Xcode, as its own header states, and cannot host an
      application or app-extension test bundle. A later task needing a Shared-only
      test target may extend it and must claim it. Naming a file in order to say
      nobody changes it is what closes an ownership gap that would otherwise read as
      an oversight — the finding's point was the zero occurrences, not the zero
      changes.
  - **R10-F2** the sandbox proof had no artifact that could perform it ->
    **Fixed**, and split honestly rather than pointed at a new script wholesale.
    Task 5 `Creates` `scripts/check-widget-sandbox.sh` for the **static** half —
    the widget entitlements grant the App Group and nothing else, no widget source
    references a database, credential, client-configuration, or session path, and
    the widget target links no module that could reach one. The **runtime** half is
    written as a manual acceptance step on macOS 26 with the extension actually
    running, because a sandbox denial is observable only from inside the running
    process; claiming a script proves it would be the same overstatement the finding
    caught. `scripts/check-privacy.sh` keeps its place in `Files` with its real job
    spelled out — it greps repository files for credential literals and says nothing
    about what a process can open — and the L3 level's "privacy checks" is now
    defined as the two halves above rather than left to be read as that script.

- Verification performed by this repair round, not yet independently confirmed:
  - The baseline `Files`/`Creates` rule was re-run after the edits: `Files` paths
    all resolve under `git cat-file -e HEAD:<path>`, `Creates` paths all do not.
    The four newly named artifacts — `apps/macos/AgentDeckAppTests/`,
    `apps/macos/AgentDeckWidgetTests/`, `scripts/check-widget-sandbox.sh`, and the
    widget catalog — were each confirmed absent from `HEAD`, so they are correctly
    `Creates` entries.
  - The claims R10-F1 rests on were read back rather than taken from the finding:
    `apps/macos/Package.swift` declares one test target, `AgentDeckSharedTests`,
    with `dependencies: ["AgentDeckShared"]` and `path: "AgentDeckTests"`, and its
    header states the package exists only to exercise `AgentDeckShared` without
    Xcode; `project.pbxproj:191-207` holds one
    `com.apple.product-type.bundle.unit-test` target with a single dependency. Both
    confirm the directory cannot host app or widget tests.
  - `scripts/check-privacy.sh` was read in full to confirm R10-F2's characterization
    before writing the clarification: it enumerates tracked and untracked files and
    greps four credential-literal patterns. It is unrelated to process capability,
    as the finding said.
  - `project.pbxproj` is now claimed in four places that agree with each other, and
    `Package.swift` in one.
  - Verification level L0: documentation only. `make check-whitespace`,
    `bash scripts/check-topic-docs.sh`, and `git diff --check` all exit 0. No
    product test was run and none is claimed.
  - Not claimed: that no further unowned artifact exists. Round 10 found R10-F1 by
    reading the build system's target table and asking which task extends it — the
    reverse direction from the path list — and R10-F2 by asking whether the artifact
    a task names can actually perform what the task claims. Both questions can be
    asked again of the two new test targets and the new script; that is the
    Re-review's to do, and the mechanical baseline check still cannot ask either.
- Evidence: `git hash-object docs/topics/desktop-app/tasks.md` ->
  `34912e6f518150ed6d2d0cec6038a41762d347ac`; baseline `Files`/`Creates` check
  consistent across tasks 3–7 after the edits; `apps/macos/Package.swift` and
  `project.pbxproj:191-207` read back as above; `scripts/check-privacy.sh:1-45`
  read in full; `make check-whitespace`, `bash scripts/check-topic-docs.sh`, and
  `git diff --check` -> exit 0. One document changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md / Round 11
```

## Round 12 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/tasks.md` blob
  `34912e6f518150ed6d2d0cec6038a41762d347ac`（Round 11 声称的修复后 blob，一致）;
  `docs/topics/v0-5-0-contract/tasks.md` blob
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`（自 Round 8 未变）。
- Reviewer: claude-code（独立复评）
- Method: 单 agent 有界复评。先核对 R10-F1、R10-F2 的关闭并自行重跑基线机械核对；
  再执行 Round 11 明确留给复评的两问，且沿它自己指出的方向——**从构建系统反查**，
  而不是沿清单正查：新增的两个测试目标要真正被执行，除 target 定义外还需要什么；
  新增的 widget 扩展目标要真正随宿主交付，除 widget target 定义外还需要什么。
- Scope: R10-F1、R10-F2 的关闭状态；tasks 3–7 的 `Files`/`Creates` 基线一致性；
  两个新测试目标与 widget 扩展目标在现有构建系统中的可达性。
- Evidence: 自行重跑基线核对——38 条 `Files` 全部命中 HEAD，12 条 `Creates`
  （含四项新命名产物）全部落空，无违反；
  `apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme:26-45`；
  `scripts/test-macos-app.sh:8-20`；
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj:156-175`；
  `apps/macos/Package.swift:24-28`；`scripts/check-privacy.sh:1-45`；
  `Makefile:101-102,118`；`grep -n -i 'scheme' tasks.md architecture.md` 无匹配。
  `make check-whitespace`、`bash scripts/check-topic-docs.sh`、`git diff --check`
  均 exit 0。L0 文档复评，未运行产品测试。

### Round 10 发现的关闭判定

- **R10-F1 已关闭。** 选择的是 `architecture.md:702` 复数表述支持的那条路：task 4
  `Creates` `AgentDeckAppTests` 目标与 `apps/macos/AgentDeckAppTests/` 源码，task 5
  `Creates` `AgentDeckWidgetTests` 与 `apps/macos/AgentDeckWidgetTests/`；
  `project.pbxproj` 的分派由三类更正为四类，测试目标 hunk 各归其主；两个 task 都写明
  测试**不**放进 `apps/macos/AgentDeckTests/` 并给出理由（该目录经
  `path: "AgentDeckTests"` 绑定 `AgentDeckSharedTests`，只链接 `AgentDeckShared`），
  同时保留各自已列的既有 Shared 测试文件——这个区分是对的，旧措辞的问题不是含糊而是
  点名了一个编译不了那些源码的目录。`Package.swift` 被点名归属"本 topic 无 task 改动"
  并说明理由与日后认领条件，这正是发现所要的：问题在于零次出现，不在于零次改动。
- **R10-F2 已关闭。** 证明被诚实地拆为静态与运行时两半：静态半由本 task 新建的
  `scripts/check-widget-sandbox.sh` 承担，三项断言（entitlements 只授予 App Group、
  widget 源码不引用数据库/凭据/客户端配置/session 路径、widget target 不链接可触达
  它们的模块）均可由静态检查判定；运行时半明写为 macOS 26 上扩展真实运行的手工验收，
  理由是沙箱拒绝只能从运行进程内部观察。`check-privacy.sh` 保留在 `Files` 中但其真实
  职责被写清，L3 的"privacy checks"改为指这两半。这处修复没有用一个新脚本笼统覆盖
  发现，而是承认了脚本能证明什么、不能证明什么。

### 🔴 严重问题——必须修复

[`docs/topics/desktop-app/tasks.md:134,138-139,191,196`] R12-F1：两个新测试目标没有
进入共享 scheme 的路径，而 scheme 是本项目 `xcodebuild test` 的唯一入口，且它不属于
任何 task。
- 行为风险：`scripts/test-macos-app.sh:8-20` 是本项目跑 macOS 测试的命令，它执行
  `xcodebuild -project … -scheme AgentDeck … test`；共享 scheme
  `AgentDeck.xcscheme:26-45` 的 `<Testables>` 块中只有一个
  `<TestableReference>`，指向 `AgentDeckSharedTests`。`xcodebuild test` 跑的是
  scheme 的 testables，不是工程里所有 unit-test 目标。于是 tasks 4/5 新建的
  `AgentDeckAppTests` 与 `AgentDeckWidgetTests` 会被建出来却从不执行——设置窗口、
  菜单栏项、timeline provider、App Intent 与四类 widget 视图的测试全部静默不运行，
  而命令仍然绿。这是最坏的失效形态：与"通过"完全无法区分。scheme 文件在 HEAD 中受
  版本控制，却在 `tasks.md` 与 `architecture.md` 中零次出现（`grep -n -i 'scheme'`
  无匹配），也不在任何 `Files`/`Creates` 中；它与 `project.pbxproj` 是两个文件，
  task 4/5 认领的 test-target hunk 覆盖不到它。这与 R10-F1 同型，只是外移了一层：
  上一轮补齐了测试目标的归属，这一轮缺的是让它们真正被执行的那个产物。
- 证据：`apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme:26-45`
  （单一 testable `AgentDeckSharedTests`，`BlueprintIdentifier`
  `A40000000000000000000003`）；`scripts/test-macos-app.sh:8-20`；
  `tasks.md:134,191`（两个 task 的 `project.pbxproj` hunk 认领，均未提及 scheme）；
  `grep -n -i 'scheme' docs/topics/desktop-app/tasks.md docs/topics/desktop-app/architecture.md`
  无匹配。
- 💡 有界修复：把
  `apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme` 写入
  tasks 4 与 5 的 `Files`，各自认领自己那一个 `<TestableReference>` 条目，并沿用已有
  的"后落地者 rebase、整文件重生成即缺陷"约定——该文件与 `project.pbxproj` 一样会被
  Xcode 整体改写。同时在两个 task 里写明验收条件：`bash scripts/test-macos-app.sh`
  的输出中必须出现本 task 的测试目标，否则测试未被执行。

[`docs/topics/desktop-app/tasks.md:124,225-229`] R12-F2：widget 扩展要随宿主 App 交付
所必需的宿主侧 hunk，按四类分派归 task 4，而 tasks 4 与 5 被明确声明互不依赖。
- 行为风险：macOS app extension 必须由宿主 App 的 target 通过 `PlugIns` 复制阶段嵌入，
  并需要宿主对扩展 target 的 `PBXTargetDependency`。当前 `AgentDeck` app target
  （`project.pbxproj:156-175`）的 `buildPhases` 只有 `Sources`、`Frameworks`、
  `Resources`、`Embed Frameworks`、`Embed AgentDeck Helper`，没有嵌入扩展的阶段，
  `dependencies` 也只有一项。task 5 交付 widget 时必须新增这两者，但按
  `:225-229` 的四类分派，app target 的 hunk 是 task 4 的；而 `:124` 又写明
  "It does not depend on task 5, and task 5 does not depend on it"。三条陈述无法同时
  成立：要么 task 5 去改 task 4 的 hunk（正是四类分派要防的重叠），要么 widget 永不
  被嵌入——那么 task 6 打包的 App 里没有 widget，而 task 6 只认领签名与打包 build
  settings，也补不上这个阶段。
- 证据：`apps/macos/AgentDeck.xcodeproj/project.pbxproj:156-175`（app target 的
  buildPhases 与 dependencies）；`tasks.md:225-229`（四类分派）、`:124`（互不依赖的
  声明）；task 6 的 `Files` 将其 `project.pbxproj` 份额限定为"signing and packaging
  build settings only"。
- 💡 有界修复：在四类分派中为"宿主嵌入扩展所需的 app-target hunk（`PlugIns` 复制阶段
  与对 widget target 的 target dependency）"指定唯一 owner。归 task 5 最贴合它的交付
  边界——嵌入是 widget 能被使用的前提——并需把该 hunk 明确从 task 4 的 app-target
  份额中排除；随后修正 `:124` 的互不依赖声明，改为在 `project.pbxproj` 上存在一处
  单向的落地顺序约束，而不是笼统的相互独立。

### 🟡 建议改进——推荐

[`docs/topics/desktop-app/tasks.md:196,204-214`] R12-F3：新建的
`scripts/check-widget-sandbox.sh` 没有说明它是否进入任何聚合门禁。
- 证据：仓库既有的 `scripts/check-privacy.sh` 有 `Makefile:101-102` 的目标，并被
  `Makefile:118` 的 `release-verify` 纳入。新脚本只作为 task 5 自身 L3 证据被直接
  调用是成立的，但 task 6 的 L4"expanded aggregate release gate"是否应包含它，
  两个 task 都没有说。`Makefile` 是 task 6 的文件，而脚本是 task 5 创建的，跨 task
  的接线不写清就容易两边都不做。
- 💡 有界改进：在 task 5 写明该脚本由本 task 直接调用作为 L3 证据，并在 task 6 的
  聚合门禁一条中明确它是否纳入 `release-verify`；无论纳入与否，写下来即可。

### 🟢 优点

- R10-F2 的修复把"脚本能证明什么"与"只有运行时能证明什么"分开，并主动把运行时那半
  标为手工验收，而不是用一个新脚本盖住发现。这比发现本身要求的更诚实。
- `Package.swift` 被点名为"无 task 改动"并附理由与日后认领条件，正确理解了发现的
  要点是零次出现而非零次改动。
- `project.pbxproj` 由三类改为四类时，同时说明了"测试目标 hunk 恰好是三类都不覆盖的
  那一类，这正是缺口不可见的原因"——把修复与失效机制绑在一起，而不是只加一行。
- Round 11 再次明确声明未主张不存在其他无主产物，并指出机械核对无法提出这两问。
  R12-F1 与 R12-F2 正是沿该提示反查得到的。

### 📝 摘要

R10-F1 与 R10-F2 经独立核对确认关闭，`Files`/`Creates` 的提交基线规则由本轮独立重跑
的 38+12 条核对确认无违反。剩余问题仍在同一方向上外移一层：R12-F1 指出让两个新测试
目标真正被执行的共享 scheme 无人认领，后果是测试建得出、跑不着且命令仍绿；R12-F2
指出 widget 扩展被宿主嵌入所需的 app-target hunk 归属与 tasks 4/5 互不依赖的声明相
矛盾。R12-F3 是新脚本的门禁接线未言明。七-task 方向、依赖图、wire owner、字符串目录
边界、发布 authority 与版本计数均无新异议。

- Verdict: FAIL — `Documents` 矩阵中 `tasks.md` 的 `Review` 单元格保持未勾选。
  本轮对被评审文档的唯一改动是 `tasks.md` 的当前评审状态段落（裁定后的状态同步，
  判定依据取自评审前的 blob `34912e6f518150ed6d2d0cec6038a41762d347ac`）；未改动任何
  task 定义、矩阵单元格、产品代码、测试或配置，未关闭任何 gate，不授权提交或推送。

#### 📌 下一步

```text
修复：desktop-app / reviews/tasks.md / R12-F1 R12-F2 R12-F3
```

## Round 13 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 12's three open findings.
  `docs/topics/desktop-app/tasks.md` is now blob
  `8727b3da533f37344fbebd2bc29eb44726f0e329`, from
  `34912e6f518150ed6d2d0cec6038a41762d347ac` — the blob Round 12 judged. HEAD is
  still `58fe5d300c5af572adef81a69a856a6aef9cea56`. No other file changed; no code,
  test, configuration, or fixture.
- Reviewer: claude-code (repair round for Round 12's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: R12-F1, R12-F2, R12-F3. Round 12 confirmed R10-F1 and R10-F2 closed and
  re-ran the baseline check itself; neither is revisited here.

- Round 12 findings, dispositions:
  - **R12-F1** the two new test targets had no path into the shared scheme ->
    **Fixed.** `apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme`
    is now in tasks 4 and 5 `Files`, each claiming its own `<TestableReference>`
    entry and nothing else in the file, under the same rebase and
    no-whole-file-regeneration rule `project.pbxproj` already carries — Xcode
    rewrites the scheme wholesale too, so leaving it unclaimed made it an
    unattributed side effect of opening the project.
    - Both tasks now state the mechanism rather than just the file: `xcodebuild test`
      executes the scheme's `<Testables>`, not every unit-test target in the project,
      so a target added to the project but not the scheme builds, never runs, and
      leaves `scripts/test-macos-app.sh` green. That is the worst failure shape
      available — indistinguishable from passing — which is why each task also gained
      an acceptance condition naming the observable: the script's output must name
      the task's test target and report its test count. A target's existence is not
      evidence it ran.
  - **R12-F2** the host-side embedding hunks were assigned to task 4 while tasks 4
    and 5 were declared mutually independent -> **Fixed** by giving the two hunks to
    task 5 and correcting the independence claim rather than deleting it.
    - Task 5's `Files` now claims the `PlugIns` copy phase and the app target's
      `PBXTargetDependency` on the widget target, with the reason: an extension
      ships only if the host copies and depends on it, the current `AgentDeck` target
      has neither, and a widget nobody embeds is not a widget. Task 4's app-target
      share explicitly excludes those two hunks; task 6 could not have supplied them
      because its `project.pbxproj` share is signing and packaging build settings.
    - The `:124` statement is corrected to the distinction it was missing. The two
      tasks remain independent *behaviorally* — neither surface reads the other, and
      either can be implemented and reviewed first — while sharing one file where
      task 5's hunks land inside the app target. That is a landing-order constraint
      resolved by the rebase rule, not a dependency. Deleting the independence claim
      would have been the wrong fix: it is true, and the dependency graph the earlier
      rounds settled depends on it.
    - The four-way `project.pbxproj` split is restated with the exclusion in it, so
      the split and the exception cannot drift apart.
  - **R12-F3** the new script's gate wiring was unstated -> **Fixed**, and stated in
    both places the finding identified as the seam. Task 5 says it invokes
    `scripts/check-widget-sandbox.sh` directly as its own L3 evidence and that the
    aggregate wiring is task 6's; task 6 gained a bullet owning that wiring as a
    `Makefile` target reached by `release-verify`, the shape
    `scripts/check-privacy.sh` already has. Both bullets say why it is written twice:
    a script one task creates and another wires is exactly where neither does it, and
    a static assertion that runs only during the task that wrote it is not a gate.

- Verification performed by this repair round, not yet independently confirmed:
  - The claims each fix rests on were read back from the build system, not taken
    from the finding: `AgentDeck.xcscheme:31-42` holds a single
    `<TestableReference>` whose `BlueprintIdentifier` is
    `A40000000000000000000003`, the `AgentDeckSharedTests` target;
    `scripts/test-macos-app.sh:10-19` runs `xcodebuild -scheme AgentDeck … test`;
    `project.pbxproj:156-175` shows the `AgentDeck` app target's build phases ending
    at `Embed AgentDeck Helper` with a single `dependencies` entry, so neither
    embedding hunk exists yet; `Makefile:101-102,118` shows the target-plus-
    `release-verify` shape the new wiring copies.
  - The scheme was confirmed present in `HEAD`, which is why it belongs in `Files`
    rather than `Creates`.
  - The baseline `Files`/`Creates` rule was re-run after the edits and reports
    consistent across tasks 3–7.
  - `AgentDeck.xcscheme` is now claimed in exactly two `Files` entries plus one
    shared-convention bullet; the embedding hunks in exactly one owner plus one
    explicit exclusion; `check-widget-sandbox.sh` in one creator and one wirer.
  - Verification level L0: documentation only. `make check-whitespace`,
    `bash scripts/check-topic-docs.sh`, and `git diff --check` all exit 0. No
    product test was run and none is claimed — in particular, that
    `scripts/test-macos-app.sh` would name a new test target is an acceptance
    condition written for the implementing task, not something this round observed.
  - Not claimed: that no further build-system artifact is unowned. Three rounds have
    now found one each by walking outward from the previous fix — target, then
    scheme, then embedding phase — and the same walk continues past this round:
    whether the widget target needs its own scheme or build-configuration entries,
    and whether the app test target needs a host application setting, are questions
    of the same shape that this repair has not asked. That is the Re-review's, and
    the mechanical baseline check still cannot ask any of them.
- Evidence: `git hash-object docs/topics/desktop-app/tasks.md` ->
  `8727b3da533f37344fbebd2bc29eb44726f0e329`;
  `git cat-file -e HEAD:apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme`
  succeeds; `AgentDeck.xcscheme:31-42`, `scripts/test-macos-app.sh:10-19`,
  `project.pbxproj:156-175`, and `Makefile:101-102,118` read back as above; baseline
  `Files`/`Creates` check consistent; `make check-whitespace`,
  `bash scripts/check-topic-docs.sh`, and `git diff --check` -> exit 0. One document
  changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/tasks.md / Round 13
```

## Round 14 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/tasks.md` blob
  `8727b3da533f37344fbebd2bc29eb44726f0e329`（Round 13 声称的修复后 blob，一致）;
  `docs/topics/v0-5-0-contract/tasks.md` blob
  `9213c4f0013277c16c53a992b8012bcc1147e0a1`（自 Round 8 未变）。
- Reviewer: claude-code（独立复评）
- Method: 单 agent 有界复评。先核对 R12-F1～R12-F3 的关闭并自行重跑基线机械核对；
  再回答 Round 13 自己点名留下的两问——widget target 是否需要自己的 scheme 或
  build-configuration 条目，app 测试目标是否需要 host application 设置——并沿同一
  方向做一次**收尾扫描**：把交付所需的构建系统产物列全，逐项问它是否落在某个
  owner 或某条显式的"无 owner"决定上。这是判断外扩是否终止的唯一方法。
- Scope: R12-F1～R12-F3 的关闭状态；tasks 3–7 的 `Files`/`Creates` 基线一致性；
  构建系统产物归属的完备性；`ux/settings.md` 四项偏好与 task 4 交付面的对应。
- Evidence: 自行重跑基线核对——39 条 `Files` 全部命中 HEAD，12 条 `Creates` 全部
  落空，无违反；`AgentDeck.xcscheme:5-8`（`buildImplicitDependencies = "YES"`）、
  `:26-45`（唯一 testable）；`project.pbxproj:156-175`（app target 的 buildPhases 与
  dependencies）、`:367-420`（各 target 自带 `PRODUCT_BUNDLE_IDENTIFIER`：
  `.shared`、`.tests`；`AgentDeckSharedTests` 为 `BUNDLE_LOADER = ""` 的无宿主测试
  包）、`:340,355`（app target 的 `baseConfigurationReference` 指向共享 xcconfig）；
  `apps/macos/Config/AgentDeck.xcconfig:1-4`；`scripts/test-macos-app.sh:8-20`；
  `Makefile:101-102,118`；`ux/settings.md:71-74`。
  `make check-whitespace`、`bash scripts/check-topic-docs.sh`、`git diff --check`
  均 exit 0。L0 文档复评，未运行产品测试。

### Round 12 发现的关闭判定

- **R12-F1 已关闭。** `AgentDeck.xcscheme` 进入 tasks 4 与 5 的 `Files`，各自只认领
  自己那一个 `<TestableReference>`，并沿用 `project.pbxproj` 已有的 rebase 与
  "整文件重生成即缺陷"约定，理由写明——Xcode 同样会整体改写该文件，不认领它就等于
  把它留成"打开工程的副作用"。更重要的是两个 task 都写出了机制而不只是文件：
  `xcodebuild test` 执行的是 scheme 的 `<Testables>` 而非工程中所有测试目标，只进
  工程不进 scheme 的目标会"建得出、跑不着、命令仍绿"。两个 task 各自新增了可观察的
  验收条件——`bash scripts/test-macos-app.sh` 的输出须点名本 task 的测试目标并报出
  用例数。把"目标存在"与"目标运行过"区分开，正是该发现的要害。
- **R12-F2 已关闭，且修法比发现建议的更准确。** 两个嵌入 hunk（`PlugIns` 复制阶段与
  app target 对 widget target 的 `PBXTargetDependency`）归 task 5，task 4 的
  app-target 份额显式排除它们，四类分派连同该例外一并重述，使分派与例外无法各自漂移。
  `:124` 的互不依赖声明没有被删掉而是被补上了它缺的那个区分：两个 task 在**行为上**
  仍互不依赖（互不读取对方、任一可先实现先评审），共享的只是一个文件上的落地顺序
  约束，由 rebase 规则解决。删掉该声明会是错的修法——它是真的，且前几轮定下的依赖图
  依赖它。
- **R12-F3 已关闭。** task 5 写明直接调用脚本作为自身 L3 证据、聚合接线属 task 6；
  task 6 新增一条认领该接线，形态与 `check-privacy.sh` 现有的
  `Makefile` 目标加 `release-verify` 一致（`Makefile:101-102,118`）。两处都写明了为何
  要写两遍：一个 task 创建、另一个 task 接线的脚本，正是两边都不做的接缝。

### Round 13 留给复评的两问

两问都落回已被认领的范围内，这也是本轮判断外扩已终止的依据：

- **widget target 无需自己的 scheme。** 共享 scheme 的 `BuildAction` 已设
  `buildImplicitDependencies = "YES"`（`:5-8`），而 app target 对 widget target 的
  依赖与 `PlugIns` 阶段现已归 task 5，因此 widget 随 app 一并被构建。它的
  build-configuration 条目（含自己的 `PRODUCT_BUNDLE_IDENTIFIER`）属于
  "widget target 的 hunk"，已是 task 5 的。工程现状支持这一读法：
  `AgentDeckShared` 与 `AgentDeckSharedTests` 各自在自己的 build config 里声明
  `com.kitdine.agentdeck.shared` 与 `.tests`，只有 app target 从共享 xcconfig 取
  标识符，所以新目标的标识符不需要动 task 6 的 `AgentDeck.xcconfig`。
- **app 测试目标的 host 设置也已被认领。** 现有 `AgentDeckSharedTests` 是
  `BUNDLE_LOADER = ""` 的无宿主测试包；链接 app 的 `AgentDeckAppTests` 需要
  `TEST_HOST` 与 `BUNDLE_LOADER`，而它们位于该目标的 `XCBuildConfiguration`，正是
  task 4 认领的"`AgentDeckAppTests` test-target hunks"。task 6 的份额限定为签名与
  打包 build settings，不与之重叠。

收尾扫描把交付所需的构建系统产物列全后，每一项都有唯一归属或一条显式的无 owner
决定：app target 与 `AgentDeckAppTests`（task 4）、widget target 与
`AgentDeckWidgetTests` 与两个嵌入 hunk（task 5）、scheme 的两个 testable（tasks 4/5
各一）、签名与打包 build settings 与 `AgentDeck.xcconfig` 与 `Makefile`（task 6）、
两份 entitlements（task 2 已交付 / task 5）、两个字符串目录（tasks 4/5）、
`Package.swift`（明示本 topic 无 task 改动，附日后认领条件）。前三轮各自向外走一步
找到一个无主产物——目标、scheme、嵌入阶段——这一步没有再找到第四个。

`ux/settings.md:71-74` 的四项偏好与 task 4 的交付面逐项对应，且都不需要额外产物：
登录项走 `SMAppService.mainApp`（不安装守护进程、无额外 bundle 或 entitlement），
周期刷新落在 task 4 已认领的 `EmbeddedHelperRunner.swift` coordinator hunk，
两项菜单栏偏好存于应用自己的 `UserDefaults`。

### 🟢 优点

- 三轮修复都没有停在"补一行归属"，而是每次把修复与失效机制绑在一起：测试目标 hunk
  是四类分派中"三类都不覆盖的那一类"、scheme 不认领就成为"打开工程的副作用"、
  "目标存在不是它运行过的证据"。这些句子使同类缺口在下一次可被认出，而不只是这一处
  被堵上。
- R12-F2 的修法拒绝了删除互不依赖声明这条更省事的路，转而补上它缺的区分。行为依赖
  与文件落地顺序是两件事，混为一谈会让已定下的依赖图失去依据。
- 每一轮修复轮都明确写出自己**未**主张什么，并点名下一轮该问什么。R12-F1 与 R12-F2
  都是沿上一轮的这类提示查到的；本轮沿同样的提示查下去没有再发现新缺口，这个"查不到"
  比前几轮的"查得到"更能说明外扩已经收敛。
- 七个 anchor 的分解本身自 Round 6 起未再被动摇：三视角挑战未发现缺失的第八个产品
  task，此后八轮全部落在 ownership、依赖、authority 与可执行性上。

### ⚪ 记录在案，非本主体的发现

`ux/settings.md:103` 写作 "Three groups — General, Menu bar — each with a title"，
列出的却只有两个组名。该文档已通过自己的评审，其 `Review` 单元格与本次裁定无关，
本轮不据此改动任何单元格，也不作为对 `tasks.md` 的发现；记在此处仅为不遗失。若要
处理，它属于 `reviews/ux-settings.md` 的下一轮。

### 📝 摘要

R12 的三项发现全部关闭，且都以"修复加机制"的方式关闭；`Files`/`Creates` 的提交基线
规则由本轮独立重跑的 39+12 条核对确认无违反；Round 13 点名留下的两问均落回已被认领
的范围，收尾扫描未发现第四个无主产物。七-task 分解、依赖图、两个 wire owner、字符串
目录边界、测试目标与 scheme 归属、嵌入 hunk 归属、发布 authority 与版本计数现已互相
一致且可派发。

- Verdict: PASS — `Documents` 矩阵中 `tasks.md` 的 `Review` 单元格已勾选，本 topic
  六份文档的文档集就此完整。

### 证据门禁：VERIFIED

按 `.agent-instructions/evidence.md`，文档边界在 `Verdict: PASS` 时被跨越。本内容状态
的 CEv1 记录已按库中现行模型写入并复查：

- `content_state` `urn:ce:agent-deck:state:candidate:ba90be3e…`，其
  `subject_digest` 由 `printf '%s' 'head=<head>;document=<blob>' | shasum -a 256`
  得出，绑定 HEAD `58fe5d3` 与 blob `8727b3da`；
- `criterion` `urn:ce:agent-deck:criterion:desktop-app-tasks:independent-review-pass`
  （`required: true`），经 `requires` 挂在 WorkUnit
  `urn:ce:agent-deck:work-unit:desktop-app-tasks` 上；
- `evidence` `urn:ce:agent-deck:evidence:desktop-app-tasks:rereview-round-14:ba90be3e…`
  （`outcome: 'pass'`），经 `satisfies` 指向该 criterion、经 `observed_at` 指向该
  content_state。

以 profile 定义的门禁查询复查 `desktop-app:tasks.md`：`VERIFIED`。

一处更正必须留在记录里：本轮初次裁定时曾写作
`COMPLETION_EVIDENCE_BLOCKED`。那个结论是错的——它出自一条被运行环境拒绝的
写入语句，而非缺少写权限；换成 profile 的幂等 upsert 模板即写入成功。更早的一版
证据还用了已于 2026-08-17 退役的大写 `Evidence` 单节点写法，`outcome` 写作 `PASS`
而非门禁实际匹配的小写 `pass`，且未建 `content_state` 与任何关系，因此那些节点对
门禁不产生任何作用；它们已被删除，本节所述记录是替代它们的正式记录。

- 本轮对被评审文档的改动仅两处状态同步：`Review` 单元格由 `[ ]` 改为 `[x]`，
  以及当前评审状态段落追加本轮裁定。未改动任何 task 定义、产品代码、测试或配置。

#### 📌 Task checkpoint

`tasks.md` 是本 topic 的 `ad-desktop-doc-tasks-design` 任务，已 Review PASS，因此
到达提交检查点。检查点是提醒，本身不构成 commit 或 push 授权。

- 目标 task：`文档：desktop-app / tasks.md`
- 建议纳入范围：`docs/topics/desktop-app/tasks.md`、
  `docs/topics/desktop-app/reviews/tasks.md`、
  `docs/topics/v0-5-0-contract/tasks.md`（T6-F6 的跨 topic 计数修正）、
  `docs/README.md`（本 topic 的阶段行，文档集完成后需同步）
- 排除的脏工作区内容：本 topic 其余文档与其评审记录、`apps/macos/**` 与
  `internal/**` 的未提交在制品、`AGENTS.md` 与 `scripts/**` 的改动——它们分属其他
  task 或其他 topic，按 hunk 归属判断，不随本 task 提交
- 验证证据：`make check-whitespace`、`bash scripts/check-topic-docs.sh`、
  `git diff --check` 均 exit 0；L0 文档改动，无产品测试要求
- 未决前置：上文的 `COMPLETION_EVIDENCE_BLOCKED`。提交前应先闭合该门禁，或明示
  接受文件证据回退

#### 📌 下一步

```text
开发：desktop-app / presentation-period-scoping
```
