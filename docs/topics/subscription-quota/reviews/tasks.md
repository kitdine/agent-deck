---
status: active
topic: subscription-quota
subject: tasks.md
---

# Tasks Review

## Round 1 — 2026-09-10

## 📋 订阅额度分解评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-TASKS-R1-F1 — 中：Current handoff 与本文件自己的 Documents 矩阵互相矛盾

- 位置：`tasks.md:319-352`，对照同文件 `:15-22`。
- 行为风险：`docs/documentation-workflow.md:306` 规定 `tasks.md` 承载「concise current state and report pointers」，而这一节现在陈述的是一个已经不存在的状态，且与同一份文件上方的矩阵直接冲突。矩阵说六份文档里五份 Review 已勾选，散文说「the surface Review cells remain unchecked」（`:344-345`）、「Settings review requires repair」（`:350-351`）、「Other document reviews and implementation have not started」（`:352`）。一份文件里两处权威说法相反，读者无从判断哪一处是真的；而这一节恰恰是别人拿来判断「现在该做什么」的地方。
- 证据：本文件 `:20` 为 `| ux/settings-quota.md | [x] | [x] |`，`:21` 为 `| architecture.md | [x] | [x] |`。settings 的复评在 `reviews/ux-settings-quota.md` Round 6 判 PASS，随 `01c7234` 提交；architecture 的复评在 `reviews/architecture.md` Round 2 判 PASS，随 `5a0245f` 提交。menubar 与 widget 的 PASS 更早，分别随 `1577997`、`2999a34` 提交。散文里唯一仍然成立的是 `:324-325`「没有实现任务开始、分解通过评审前不创建实现派单」。
- 💡 有界修复：把 `:319-352` 改写为当前状态——六份文档已起草，requirements、menubar、widget、settings、architecture 五份评审通过并各自指向其记录，`tasks.md` 本身在评审中；保留仍然成立的 worktree 基线段与实现未开始的声明。不要在这里复制发现 ID、评分或修复指引（同一份 documentation-workflow 的表格禁止）。

#### SQ-TASKS-R1-F2 — 中：七个任务没有一个写出文件边界

- 位置：`tasks.md:105-278`（任务 1–7）。
- 行为风险：`docs/documentation-workflow.md:306` 把「file boundaries」列为 `tasks.md` 的评审判据之一，与 anchors 和验证级别并列。这里七个任务都有 `**Depends on:**`、`**Result:**`、条目和 `**Verification:**`，唯独没有文件边界。缺了它，「一个任务一次提交」和「共享文件按 hunk 分类」就只能在提交时临时判断——本主题已经为此付出过两次代价：`01c7234` 与 `5a0245f` 都必须手工构造一份只含本任务范围的 `tasks.md` blob 才能提交，因为没有任何文档说过哪些改动属于哪个任务。实现阶段涉及的是 Go 包、Swift 目标和 prototype 三类产物，边界不写下来时，任务 6 和任务 7 会在同一批文件上相遇。
- 证据：`rg -c '\*\*Files' docs/topics/subscription-quota/tasks.md` 无命中。同项目已通过评审并已合入的 `docs/topics/schema-version-signal/tasks.md` 则每个任务都有，紧跟在 `**Result:**` 之后，例如 `:92-97` 列出 `internal/store/store.go`、`internal/store/migrations.go`、`internal/store/store_test.go` 等，并补一句「Focused additional test files may live in these same packages」。本主题的任务里只有任务 3 在条目内顺带提到 `internal/usagehook/config.go`，那是引用而不是边界。
- 💡 有界修复：给七个任务各补一行 `**Files:**`，沿用 schema-version-signal 的写法与粒度。新建包写「new `internal/...`」，Swift 任务写目标与文件，任务 7 写清它与任务 6 在 prototype 与 wire 上的分界。不需要改任务划分本身。

#### SQ-TASKS-R1-F3 — 中：C5 的来源优先级没有落到任何任务

- 位置：`tasks.md:105-278`；对照 `architecture.md:236-243`。
- 行为风险：C5 有两半。陈旧判据那一半落在任务 1（`:116-117` 明确引用 C5）；**来源优先级那一半没有任何任务认领**——「每客户端取最新可用观测，同龄时状态栏路径优先于散文路径」，以及「所选 `source` 随 payload 传出，界面据以命名来源」。任务 3 建了 Claude 的两条路径但没说谁赢，任务 1 建了领域模型但没提优先级，任务 6 只说 wire 承载 `source` 而没说谁写它。于是「同龄时谁赢」这条规则会在实现时被临时决定，而 `requirements.md` clause 5 恰恰对它可判定：「the surface names which route produced the figure」。菜单栏的 Data requirements 也把 `source` 列为必需字段。
- 证据：`rg -n "precedence|newest|preferred"` 在 `tasks.md` 无命中；`C5` 在 `tasks.md` 仅出现一次，即 `:117` 的 `allowed_age`。十三个 C 条款在 `tasks.md` 中都至少被引用一次，所以缺口不是整条条款遗漏，而是 C5 内部只覆盖了一半。
- 💡 有界修复：在任务 1 或任务 3 增加一条，写明来源优先级与 `source` 的赋值归属，并在该任务的 `**Verification:**` 里加上同龄时状态栏胜出的用例。归到哪个任务由分解者决定——任务 1 拥有领域模型、任务 3 拥有两条 Claude 路径，两者都说得通，写下来即可。

### 🟡 建议改进 — 推荐

#### SQ-TASKS-R1-F4 — 低：「six failure states」是修复前的旧计数

- 位置：`tasks.md:32`。
- 行为风险：该句在论证 `ux/menubar-quota.md` 这一行为何存在时写「六种失败状态用户可见且今天没有呈现规则」。封闭原因集在 2026-09-09 的需求修复中加入 `probe_disabled` 后已是七个成员。这是本主题已经栽过两次的同一类失效——手工维护的计数在上游新增成员后腐烂（`SQ-SQ-R3-F1` 的 `18/18`、settings 的 `updated:` 戳），代价是这句话再也不能用来发现下一次漂移。
- 证据：`ux/menubar-quota.md:257-265` 的原因表有七行；`architecture.md:321-323` 的封闭集列出七个成员；`prototype/src/i18n.js:57-65` 也渲染七条。
- 💡 有界改进：改成七，或改成不随成员增减失效的说法（例如「封闭原因集的每个成员」）。只改这一句。

#### SQ-TASKS-R1-F5 — 低：跨主题引用指向了错误的行

- 位置：`tasks.md:271-273`。
- 行为风险：任务 7 引用 `docs/topics/v0-6-0-contract/tasks.md:164-165` 作为「读当时的规范修订版，不要预先指定修订号」这条规则的出处，但那两行是该文件的 `**Result:**` 行与一个空行；规则实际在 `:166-168`。读者按行号跳过去会落在别处。本主题在 `architecture.md` 上的引用逐条核验全部精确，这是唯一一处偏离。
- 证据：`docs/topics/v0-6-0-contract/tasks.md:164` 为 `**Result:** the integrated v0.6.0 behavior and version-level documentation agree.`，`:165` 为空行，`:166-168` 才是「Reconcile … Read the then-current spec revision; do not prescribe revision 29 or 30 …」。
- 💡 有界改进：把行号改为 `:166-168`，或改为按小节引用以免随该文件变动再次失准。

### 🟢 优点

- **十三个架构条款全部被引用，没有一条掉在地上**。`C0` 到 `C12` 在 `tasks.md` 中都至少出现一次并落到具体任务，`SQ-TASKS-R1-F3` 是条款**内部**的半条缺口，而不是整条遗漏——这个区别本身说明分解是按条款走查过的，不是凭印象写的。
- **依赖顺序与产物顺序一致且有约束力**：任务 1 是领域模型，2 与 3 各自依赖 1，4 依赖 2 和 3，5 依赖 1 和 4，6 依赖 1–5，7 依赖 6。没有循环，也没有「所有任务依赖所有任务」这种等于没写的依赖。
- **验证级别是按对象选的，不是按阶段套的**：解析器与领域规则给 L1，调度器、告警、状态栏串接、CLI 表面给 L2，Swift 界面给 L2 加人工验收。任务 1 的验证条目尤其具体——`allowed_age` 要在两个窗口长度、每个可选间隔（含下限接管的那一个）以及阈值两侧各一分钟处检查，这是能失败的描述而不是「测试覆盖 allowed_age」。
- **人工验收单独成表并指明归属任务**（`:280-293`），七项各有 owning task，且开宗明义「每项都需要一个显式结果——执行过，或由操作员豁免且豁免被记录」。这把「标本settle不了的事」从模糊的尾巴变成了可核对的清单。
- **「Findings repaired in this change」把车道决定记在了明处**（`:295-317`）：三个窄边界缺陷按操作员 2026-09-08 的指示就地修复，因此不建 bug 载体、不需要车道决定——决定是谁做的、什么时候做的都写下来了，而不是让读者以为有人擅自跳过了流程。同一节还诚实记录了量具加固后**立即发现本主题自己引入的第四个缺陷**（英文 tab strip 在 420pt 宽出 14px），并说明量具改动是通过「关掉一处修复、确认量具报出来、再打开」验证的——「A check that has never been observed to fail is not evidence」。
- **Documents 一节为每一行给出了存在理由**（`:24-43`），并显式处理了 `n/a`（`:45`）与「prototype 不占行」（`:46-48`，附 documentation-workflow 引用），而不是留给读者推断。
- 任务 6 明确把 `docs/specs/cli-design.md` 的对账推给任务 7 收口（`:247-248`），任务 7 又把「不预先指定修订号」的理由挂到 v0-6-0-contract 的 assemble 任务上——跨主题边界是写下来的，不是默认的。

### 📝 总结

- 五项发现：`SQ-TASKS-R1-F1`、`SQ-TASKS-R1-F2`、`SQ-TASKS-R1-F3` 为中，`SQ-TASKS-R1-F4`、`SQ-TASKS-R1-F5` 为低，全部 OPEN。按本项目发现政策，低严重度同样阻断 PASS。Review 保持未勾选。
- 三项中等发现分属两类：F1 是状态陈述与自身矩阵矛盾，F2 与 F3 是分解的覆盖缺口——一个是每个任务都缺的横向字段，一个是某条款内部漏掉的半条。分解的骨架（任务划分、依赖、验证级别、人工验收归属）本身是好的，欠的是边界与一处规则归属。
- Reviewer：Claude Code（`claude-opus-5`，1M context）；Method：单主会话逐节评审，按 `docs/documentation-workflow.md:89,306` 为 `tasks.md` 规定的判据（文档集完整性、分解是否覆盖已批准文档而无缺口或越界、锚点、文件边界、验证级别是否相称）走查；十三个 C 条款逐条回查归属；与已通过评审的兄弟主题 `schema-version-signal/tasks.md` 对照任务写法；跨主题与跨文档引用逐条核验。非冷上下文独立评审，未委派，未使用低模型层级。
- Scope：`tasks.md` 全文。未修改产品代码、测试、配置、标本或任何被评审文档。
- Reviewed state：HEAD `5a0245f19f1ef0a354c3bb2a2bf5bd0956da87e8`；document blob `dcd3e4075226941e56d2856700bf7039c0f3952e`。
- Prototype：与 `architecture.md` 评审时字节一致，沿用 [architecture-r1 输入清单](evidence/architecture-r1/prototype-manifest.sha256)，清单 SHA-256 `72067191a79d6772dc7d4390fb9505cc3f520f09f61023cbda94b030de162c3b`；逐文件重算无差异，故本轮不另建证据目录。
- Content fingerprint：`94c4d3234dd905a18dc89a6b5bb95b8a0cb5f6517071c2d8c7af35462023b6ad`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-tasks-design`。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:tasks.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:tasks:5a0245f:dcd3e40:7206719`。本轮之前该 WorkUnit 与其 criterion 在库中不存在，按阶段授权以本主题既有三准则形状建立（`decomposition`、`review`、`l0`）后再记录证据。
- **项目自带的同类检查器**：`bash scripts/check-topic-docs.sh` 是 `docs/documentation-workflow.md:134` 为 `tasks.md` 评审**明确要求**运行的检查器，本轮实际运行，exit 0。它比对声明行、磁盘上的主题文档与 `ux/<surface>.md` 引用，并把不足十行的文件标为疑似占位；按其自身说明（`:139`）这些是结构检查，不证明语义完整。本轮三项中等发现均在其覆盖之外：它不判断状态散文是否与矩阵一致、不判断任务是否写了文件边界、也不判断某条架构条款是否被认领。这是工具边界，不是工具失效。
- L0：`make check-whitespace`、`git diff --check`、`scripts/check-topic-docs.sh` 全部 exit 0。
- 残余不确定性：本轮判断「分解是否覆盖已批准范围」的依据是五份已通过评审的上游文档与 `architecture.md` 的十三个条款；实现阶段是否还会暴露未声明的表面或契约，按 `docs/documentation-workflow.md:221` 属于届时修订文档集并让 `tasks.md` 回到评审的情形，不是本轮可以预判的。任务 7 的 Swift 表面与人工验收项无法在本轮验证，只核验其归属与描述是否可失败。
- 完成门禁：FAILED。criterion `decomposition` 记 `fail`（F1/F2/F3）、`review` 记 `fail`（五项发现未关闭）、`l0` 记 `pass`。
- 未跨越主题完成边界；不建议提交或推送。

### 下一步指令

修复：subscription-quota / reviews/tasks.md / SQ-TASKS-R1-F1 SQ-TASKS-R1-F2 SQ-TASKS-R1-F3 SQ-TASKS-R1-F4 SQ-TASKS-R1-F5

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 2 — 2026-09-10

## 📋 订阅额度分解复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-TASKS-R2-F1 — 中：把仅供原型评审的额度状态控件列入了产品实现任务

- 位置：`tasks.md:320`；对照 `ux/menubar-quota.md:17-54` 与同文件
  `tasks.md:50-80`。
- 处置：NEW。
- 行为风险：任务 7 的 Result 说 Swift 三个表面要匹配已批准文档，紧接着却把
  `visible quota-state control` 与菜单栏额度 tab、五列 tab strip 一并列为实现内容。
  额度状态控件实际是 `prototype` stage bar 的评审夹具，用来切换 Gate、Codex
  Plus、解析失败、陈旧等标本状态；`ux/menubar-quota.md:42-48` 明说 URL 仅是自动
  评审 deep link，而该可见控件属于原型 stage bar。`tasks.md` 自己也已经把它列在
  `## Prototype` 的既有设计产物里。照任务 7 的现文实现，会把测试／评审状态选择器
  带进真实菜单栏，新增一个上游文档没有批准的产品控件。
- 证据：`tasks.md:61` 把该控件归到 `prototype/src/Stage.jsx`、`App.jsx` 与
  `Popover.jsx`；任务 7 的 Files 边界 `:302-318` 又明确说 `prototype/` 只是被消费、
  不由任务 7 编辑。`ux/menubar-quota.md:7-10` 将产品范围限定为额度 tab 与第五个
  tab 迫使 tab strip 发生的变化，不包含任何运行时状态选择器。
- 💡 有界修复：从任务 7 的产品实现条目删掉 `visible quota-state control`；保留
  `## Prototype` 中对 stage control 的既有记录。不要为真实 App 新增该控件。

#### SQ-TASKS-R2-F2 — 中：小号 Widget 的 Claude 账号归属提示被降成仅辅助名称

- 位置：`tasks.md:323-326`；对照 `ux/widget-quota.md:58-108` 与
  `prototype/src/Widgets.jsx:520-527,601-628`。
- 处置：NEW。
- 行为风险：任务 7 要求小号 Widget 只把 Claude 的归属限制放进
  `accessible name`，会让绝大多数直接看数字的人看不到 `账号未确认 / account
  unconfirmed`。这正是已批准 Widget 文档明确否决的旧方案：需求 clause 11 要求
  在显示 Claude 数字的表面陈述限制，不能只让屏幕阅读器听到。文档要求三个尺寸
  都在屏幕上显示短式提示，原型也在 small 的 reset countdown 与 freshness 之间
  渲染同一个 `QuotaAttribution` 组件。
- 证据：`ux/widget-quota.md:71-85` 明说 `all three show it on screen`，并记录
  accessible-name-only 方案为何错误；`:91-95` 定义 small 的可见位置。当前原型
  `prototype/src/Widgets.jsx:520-525` 说明该行不可只塞进 title 或辅助名称，
  `:627` 在 small 分支实际渲染它。`tasks.md:75` 的 Prototype 表也已经写着
  `Visible Claude account-attribution line on all three widget sizes`，与任务 7 条目
  自相矛盾。
- 💡 有界修复：把任务 7 改成三个 Widget 尺寸都按
  `ux/widget-quota.md`／prototype 在屏幕上显示短式归属提示；small 的辅助名称可以
  同时携带它，但不能代替可见文本。

#### SQ-TASKS-R2-F3 — 中：Manual acceptance 漏接两项上游明确交给 tasks.md 的检查

- 位置：`tasks.md:344-357`；对照 `ux/menubar-quota.md:535-545` 与
  `ux/widget-quota.md:378-386`。
- 处置：NEW。
- 行为风险：Manual acceptance 表覆盖了真实字体、五个 tab 的 VoiceOver、设置
  禁用态、WidgetKit 渲染、通知、同意流程与实时主题切换，但漏掉了两份已批准 UX
  文档明确写着交给 `tasks.md` 的检查：额度 bar 的 tone 对红绿色盲读者是否可区分，
  以及 Widget 内部的 accessibility reading order。实现任务即使完整执行现表也会在
  没有结果、没有 operator waiver 的情况下越过这两条上游验收边界。
- 证据：`ux/menubar-quota.md:537-544` 的四项 handoff 中前三项和第四项分别可在
  `tasks.md:351-352,357` 找到，唯独 colour-blind tone 检查无对应行；
  `ux/widget-quota.md:380-384` 的真实 WidgetKit／font／tint／refresh 已合并进
  `tasks.md:354`，但 accessibility reading order 无对应行。两份 UX 文档都明说这些
  项目属于 `tasks.md` manual acceptance。
- 💡 有界修复：在 Manual acceptance 表补两行并归给任务 7：一行检查额度 bar
  tone 对红绿色盲读者的可区分性（百分比文字仍是非颜色载体），一行检查 Widget
  内部 accessibility reading order。两项都沿用本表的规则：执行，或由 operator
  显式 waiver 并记录。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **SQ-TASKS-R1-F1 — CLOSED**：`tasks.md:383-398` 已与 Documents 矩阵一致：
  六份文档已起草，五份上游文档已通过，当前仅 `tasks.md` 等待复评；实现未开始，
  且 `4737076..main` 实际仍为十个提交，基线说明可复算。
- **SQ-TASKS-R1-F2 — CLOSED**：七个任务都已有 `**Files:**` 边界（`:110`、
  `:149`、`:174`、`:206`、`:244`、`:270`、`:302`），共享的
  `internal/desktop/desktop.go`、`cmd/agentdeck/main.go` 与
  `DesktopPreferences.swift` 还分别按 refresh/wire、capture/output、control-path/
  presentation 说明了 hunk 归属。
- **SQ-TASKS-R1-F3 — CLOSED**：C5 的来源选择已归 `quota-domain`（`:129-131`），
  同龄时 status-line 胜出并保留 `source` 的可失败用例已进入该任务验证（`:140-141`）。
- **SQ-TASKS-R1-F4 — CLOSED**：`ux/menubar-quota.md` 的存在理由改成
  `every member of the closed failure-reason set`（`:30-33`），不再复制会随成员变化
  而腐烂的数量。
- **SQ-TASKS-R1-F5 — CLOSED**：任务 7 改按 `v0-6-0-contract` 的稳定 task 2 小节
  引用（`:334-337`），不再把易漂移行号当规则身份。
- 文档集检查器仍确认声明矩阵、磁盘文件和主题内 UX 引用三方一致；七个任务的
  依赖图、C0–C12 归属和验证层级保持 Round 1 的优点。新发现均是检查器覆盖之外的
  语义回查，不是结构检查退化。

### 📝 总结

- Round 1 五项发现全部关闭：`SQ-TASKS-R1-F1` CLOSED、
  `SQ-TASKS-R1-F2` CLOSED、`SQ-TASKS-R1-F3` CLOSED、
  `SQ-TASKS-R1-F4` CLOSED、`SQ-TASKS-R1-F5` CLOSED；无回归、无替代链、无转出。
- 本轮新增三项中等发现并全部 OPEN：`SQ-TASKS-R2-F1` 把原型 stage control
  误列为真实产品内容，`SQ-TASKS-R2-F2` 与已批准 Widget 文档和原型相反地把 small
  归属提示降为仅辅助名称，`SQ-TASKS-R2-F3` 漏接两项上游 manual acceptance。
  三项都位于被评审分解自身，不能携带到别处后 PASS。
- Reviewer：Claude Code（`claude-opus-5`，1M context）；Method：独立 reviewer
  role，未参与 Round 1 repair；从 Round 1 记录与当前内容重建 finding disposition
  matrix，逐项复核五项修复，并把任务 7 与三份已批准 UX 文档、当前 prototype
  逐条回查，核对 C0–C12、七个 Files 边界、依赖、验证级别与全部
  `What the specimen does not settle` handoff。未委派，未使用低模型层级。
- Scope：`tasks.md` 全文、其 Round 1 五项发现，以及判定分解覆盖所需的已批准
  requirements／UX／architecture 和未变 prototype。未修改被评审文档、产品代码、
  测试、配置或标本。
- Reviewed state：HEAD `5a0245f19f1ef0a354c3bb2a2bf5bd0956da87e8`；
  document blob `1f05918a5ff724875d23e3432e2abb196a72312a`（Round 1 为 `dcd3e407`）。
- Prototype：与 Round 1 字节一致，沿用
  [architecture-r1 输入清单](evidence/architecture-r1/prototype-manifest.sha256)，
  清单 SHA-256 `72067191a79d6772dc7d4390fb9505cc3f520f09f61023cbda94b030de162c3b`；
  本轮用 `shasum -a 256 -c` 逐文件确认全部 `OK`，不因阶段变化重跑 build／gauge／
  interaction probe。
- Content fingerprint：`0e8f013582e2701be0d59ef7b2fb046db01043e131a30eefa0a4a5526844e754`，
  SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；
  Task：`ad-sq-doc-tasks-design`。实时 Beads 读回为 `in_review`、assignee
  `claude-code`，且 Round 1 repair handoff `01a08bc9-def8-7dde-84cd-7f5ab1aadec7`
  指向同一 candidate blob。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:tasks.md`；Target
  ContentState：`urn:agent-deck:content-state:subscription-quota:tasks:5a0245f:1f05918:7206719`。
  首次 exact-target gate 查询为 NOT_VERIFIED／`missing_target_state`，三项既有 required
  criterion 均缺当前态证据，旧态观察因 target 不匹配未被复用。
- L0：`make check-whitespace`、`git diff --check`、
  `scripts/check-topic-docs.sh` 全部 exit 0；最后一项是本项目要求的 document-set
  checker，其结构边界不覆盖本轮三项语义发现。
- 残余不确定性：本轮没有执行真实 Swift／WidgetKit／VoiceOver／颜色感知验收；
  这正是 `tasks.md` 应完整分配而实现阶段再执行或由 operator waiver 的边界。本轮只
  判定分解是否完整、无越界且与批准设计一致。
- 完成门禁：FAILED。criterion `decomposition` 记 `fail`（三项新分解缺口）、
  `review` 记 `fail`（本轮三项发现未关闭）、`l0` 记 `pass`；新 ContentState、
  三项 Evidence 与六条关系按 CEv1 固定模板写入并在 relation preflight 通过后读回。
- `tasks.md` 的 Documents Review 保持未勾选；未创建任何实现派单，未跨越 Task 或
  Topic 完成边界，不建议提交或推送。

### 下一步指令

修复：subscription-quota / reviews/tasks.md / SQ-TASKS-R2-F1 SQ-TASKS-R2-F2 SQ-TASKS-R2-F3

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 3 — 2026-09-10

## 📋 订阅额度分解复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **SQ-TASKS-R2-F1 — CLOSED**：任务 7 的菜单栏条目现在只包含产品范围内的
  quota tab、五列 tab strip、窄边界 icon-only 规则与 reset-credit popover
  （`tasks.md:320-322`）；prototype-only 的额度状态选择器只保留在
  `## Prototype`（`:61,80`），并继续明确由 `prototype/src/Stage.jsx` 等标本文件
  承载，不再进入 Swift 产品任务。
- **SQ-TASKS-R2-F2 — CLOSED**：任务 7 现在逐尺寸写出可见 Claude 归属提示的位置：
  small 在 reset countdown 下方、medium 在 window rows 下方、large 在 Claude
  block header（`:323-327`）；small 的 accessible name 明确是可选的附加载体，
  不能替代可见行。这与 `ux/widget-quota.md:71-108` 和
  `prototype/src/Widgets.jsx:520-527,601-628` 一致。
- **SQ-TASKS-R2-F3 — CLOSED**：Manual acceptance 新增 colour-blind quota-bar tone
  与三个 Widget size 的 accessibility reading order 两行（`:354-355`），均归任务
  7，并受本表「实际执行，或由 operator 明确 waiver 并记录」的统一规则约束。
- Round 1 的五项发现保持关闭：Current handoff 与矩阵一致；七个任务均有 Files
  边界；C5 来源优先级及 equal-age 用例仍归 `quota-domain`；失败原因继续引用封闭
  集合而不复制计数；跨主题规则继续按稳定 task 2 小节引用。
- 状态同步只勾选 `tasks.md` Review 并更新 Current handoff；七个实现任务的 Dev／
  Review 均保持未勾选，没有把文档通过误写成实现完成。

### 📝 总结

- 全部历史发现已关闭：`SQ-TASKS-R1-F1` 至 `SQ-TASKS-R1-F5`、
  `SQ-TASKS-R2-F1` 至 `SQ-TASKS-R2-F3` 均为 CLOSED；无回归、无替代链、无转出，
  本轮无新增发现。
- Reviewer：Claude Code（`claude-opus-5`，1M context）；Method：独立 reviewer
  role，未执行 Round 2 repair；按当前内容逐项复核三项发现，并回查已批准的
  menubar／Widget UX 与未变 prototype；复核 Round 1 五项 disposition、C0–C12
  归属、七个 Files 边界、依赖、验证级别及所有 manual-acceptance handoff。未委派，
  未使用低模型层级。
- Scope：`tasks.md` 全文、Round 1／2 八项历史发现，以及判断修复一致性所需的已批准
  UX 与 prototype。未修改产品代码、测试、配置或标本；仅在 PASS 后同步本文件自己
  的 Review 状态和 Current handoff。
- Reviewed state：HEAD `5a0245f19f1ef0a354c3bb2a2bf5bd0956da87e8`；入口
  candidate blob `bac749f533bcb31e5f24cbde839b1c5b4b9e8096`；仅同步审批状态后的最终
  document blob `964f87521156ff7df59f7a7e70a9b9d2f5be9cf2`。任务分解内容未因本轮改变。
- Prototype：与 Round 1／2 字节一致，沿用
  [architecture-r1 输入清单](evidence/architecture-r1/prototype-manifest.sha256)，
  清单 SHA-256 `72067191a79d6772dc7d4390fb9505cc3f520f09f61023cbda94b030de162c3b`；
  本轮再次用 `shasum -a 256 -c` 逐文件确认全部 `OK`，不因阶段变化重跑
  build／gauge／interaction probe。
- Content fingerprint：`bd03d1a662869927ba5427ba8d27bfa0828b11deb16538d0cdc48813398960a4`，
  SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；
  Task：`ad-sq-doc-tasks-design`。实时 Beads 在本轮开始时为 `in_review`、assignee
  `claude-code`；Round 2 repair handoff `01a08c00-4d82-7e57-8253-b6817b88ee71`
  指向入口 candidate blob。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:tasks.md`；Target
  ContentState：`urn:agent-deck:content-state:subscription-quota:tasks:5a0245f:964f875:7206719`。
  首次 exact-target gate 查询为 NOT_VERIFIED／`missing_target_state`，三项既有
  required criterion 均缺最终审批态证据，旧态观察因 target 不匹配未被复用。
- L0：`make check-whitespace`、`git diff --check`、
  `scripts/check-topic-docs.sh` 全部 exit 0；最后一项继续确认 Documents 矩阵、磁盘
  文档和本主题 UX 引用一致。
- 残余不确定性：尚未执行任何实现任务，也未运行 Swift／WidgetKit／VoiceOver／
  颜色感知的真实验收；这些已经完整归入任务 3、5、7 的 verification／manual
  acceptance，不属于本轮文档分解 PASS 所证明的运行时结果。
- 完成门禁：VERIFIED。criterion `decomposition`、`review`、`l0` 三项均记
  `pass`；新 ContentState、三项 Evidence 与六条关系按 CEv1 固定模板写入，relation
  preflight 全部 `ok`，写入后 exact-target gate 读回无 missing／invalidated／
  unresolved。
- Documents 矩阵中 `tasks.md` Review 已勾选；主题还有七个实现任务，未跨越 Topic
  完成边界。

### Task checkpoint

Task checkpoint：`ad-sq-doc-tasks-design`；content_state `bd03d1a662869927ba5427ba8d27bfa0828b11deb16538d0cdc48813398960a4`；门禁 VERIFIED

提交建议：仅提交 `docs/topics/subscription-quota/tasks.md` 与 `docs/topics/subscription-quota/reviews/tasks.md`；提交仍需单独显式授权，并在提交后把证据绑定到不可变提交内容态。

推送建议：`origin/feature/subscription-quota`；远程分支当前不存在，须先完成授权提交、验证不可变提交及签名，再另行取得推送授权。本轮不执行提交或推送。

### 下一步指令

开发：subscription-quota / quota-domain

WORKFLOW_WORKSPACE: agent-deck.subscription-quota
