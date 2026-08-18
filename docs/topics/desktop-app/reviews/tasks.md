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
