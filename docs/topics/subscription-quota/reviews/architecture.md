---
status: active
topic: subscription-quota
subject: architecture.md
---

# Architecture Review

## Round 1 — 2026-09-10

## 📋 订阅额度架构评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SQ-ARCH-R1-F1 — 中：读取关闭时，已注册的 statusLine 该怎么办没有答案

- 位置：`architecture.md:294-310`（C9 调度表第一行与「Turning reading off retains storage」）；关联 `ux/settings-quota.md:42-50`、`requirements.md:324-334`（clause 1、2）。
- 行为风险：C9 只回答了「从未注册过」的情形。真正会发生的是另一种：用户先同意并注册了状态栏通路，之后把「读取额度」关掉。此时两条约束互相拉扯——clause 1 与设置文档都说关闭状态下**不写** `~/.claude/settings.json`，所以不能去注销；而不注销就意味着 Claude Code 仍在每次状态栏刷新时调用 AgentDeck 的命令，把 payload 推给它。文档从未说这些被推来的观测是丢弃还是照存。实现者必须自己在三种做法里选一种：照样注销（违反「不写」）、留着但丢弃捕获、留着且继续入库（那么「读取关闭」这句话在用户看来就是假的——开关说没在读，另一个工具的配置里还装着 AgentDeck 的命令并且一直在跑）。
- 证据：C9 第一行写的是「the status-line route is not registered」。这句话有两种读法，两种都留下缺口。读作状态断言（该通路处于未注册状态），则与「nothing is written to `~/.claude/settings.json`」矛盾，因为把已注册的注销掉就是一次写入。读作动作断言（关闭期间不执行注册动作），则已注册的命令继续存在并继续被调用，而「捕获到的数据怎么处理」全文没有一句。`ux/settings-quota.md:45-47` 的「nothing is read by any trigger」也覆盖不到它——状态栏捕获不是 AgentDeck 触发的，是 Claude Code 推过来的。全文检索 `statusLine`、`not_consented`、C3、C9 均无该转换的处置。
- 💡 有界修复：在 C9 或 C3 增加一条转换规则，明确「读取关闭且状态栏通路此前已注册」时：命令是否保留在文件里、被推来的 payload 是丢弃还是入库、以及用户在设置里看到的开关状态是什么。三选一并写出理由即可，不需要改 clause 1 或设置文档的措辞——它们只是没覆盖这一格。

#### SQ-ARCH-R1-F2 — 中：陈旧判据被 C5 认领了，但没有给出判据

- 位置：`architecture.md:221-224`。
- 行为风险：C5 明确把这条规则收进架构（"**Staleness** is a function of the window it describes, not a global constant"），却只给了方向性的一句「五小时窗能容忍的绝对年龄比周窗短」，没有函数、没有系数、没有任何一个具体数值，也没有一个可算的例子。`stale` 是用户可见的状态，三个界面都渲染它，`requirements.md` clause 9 还把它写成可失败的验收项（"A figure older than its allowed age is presented as stale"）——而「allowed age」在本仓库任何一处都没有定义。两个实现者会选出不同的阈值，而 clause 9 无法判定谁对。
- 证据：全仓检索 `stale` 后确认：`ux/menubar-quota.md:245,337`、`ux/widget-quota.md:197-205` 都只把它当作 payload 送来的既成标志渲染；`prototype/src/data.js:575,631,726-730` 在夹具里直接写死 `stale: true/false` 并配一个任意的 `observed_at`，没有任何推导；`requirements.md` clause 9 只声明后果不给判据。C5 是唯一自称拥有该规则的地方。架构自己的 Verification 表（`:424-436`）也没有任何一行覆盖陈旧计算——L1/L2 十行里没有 staleness。
- 💡 有界修复：在 C5 给出可计算的判据，例如「阈值 = f(window_minutes)」的具体形式与两个窗口长度下的取值，并在 Verification 表补一行 L1 覆盖它。不需要改界面文档——它们要的就是一个算好的布尔值。

#### SQ-ARCH-R1-F3 — 中：`window_key` 从未定义，Claude 的窗口长度既没提供也没拒绝

- 位置：`architecture.md:255`、`:281`、`:330-331`（三处以 `window_key` 为键）；`:95-108`（C2 映射表）；`:362-365`（C11 字段清单）；关联 `ux/menubar-quota.md:332`。
- 行为风险：C7 的存储键 `(client, account_id, window_key)`、C8 的 Claude 存储键 `(client, window_key)`、C10 的告警去重键 `(client, window_key, threshold, window instance)` 全都以 `window_key` 为轴，而全文没有一句定义 `window_key` 是什么。C2 只为 Codex 描述了「`limitId` plus `primary`/`secondary` forms the stable key」，且没把它命名为 `window_key`；Claude 侧一字未提。同时 `window_minutes` 只出现在 C2 的 Codex 厂商映射里，而 `ux/menubar-quota.md:332` 把「window length」列为**两个客户端都需要**的字段，理由是「the length drives the label」——C3 的 statusline payload（`:137-141`）与 C4 的散文（`:186-189`）都不带任何时长。这条被请求的字段在架构里既没有被提供，也没有被带理由地拒绝，因此文档开篇「Every field the three surface documents request is provisioned below or refused with a reason」（`:9-11`）对它不成立。
- 证据：`rg -n "window_key|five_hour|seven_day|window_minutes"` 对 `architecture.md` 的全部命中只有上述几处，无定义句。标本里确实有答案——`prototype/src/data.js:635-636` 用 `key: "five_hour"/"seven_day"` 配 `window_minutes: 300/10080`，`:685-688` 用 `codex_primary`/`codex_bengalfox_secondary`——但标本是界面设计权威，不是 wire 与存储契约的权威；`tasks.md` 的 `quota-domain` 与 `wire-and-cli` 两个实现任务是照 `architecture.md` 写的。
- 💡 有界修复：在 C6 或 C7 用一两句定义 `window_key` 的构成（两个客户端各自的形式），并在 C3 处置 Claude 的 `window_minutes`——要么提供（由两个固定窗口名推出 300/10080 并说明这是本地常量而非厂商字段），要么带理由拒绝并说明菜单栏的窗口标签改用什么。与标本已有的取值对齐即可，不需要改标本。

### 🟡 建议改进 — 推荐

#### SQ-ARCH-R1-F4 — 低：C11 说「最紧窗口在 payload 里解析」，但列出的 payload 没有承载它的字段

- 位置：`architecture.md:362-365`（字段清单）与 `:367-370`（该论断）；关联 `ux/widget-quota.md:229-239`。
- 行为风险：C11 先声明「**Tightest-window resolution happens in the payload**, not in the widget」，理由是两处独立推导就是两次分歧机会。但紧接其上的字段清单里既没有「最紧窗口」这样的字段，也没有对 `windows[]` 顺序的任何约定。按清单实现，小组件只能自己按 `used_percent` 排一遍——正是这段话说要避免的做法；而它与 popover 是否一致，就退化成「两边记得用同一条规则」的约定，而不是结构上成立。小组件文档要的正是「sorted or sortable by used share」，架构给了后半句却宣称给的是前半句。
- 证据：`:362-365` 的枚举为 `applicable`+reason、`source`、`observed_at`、`stale`、`attribution_confirmed`、`plan`+reason、`windows[]`、`reset_allowance`+reason、`observed_reset_at`、`failure`，无最紧窗口字段、无排序约定。标本侧同样没有：`rg -n "tightest|highest|sort"` 在 `prototype/src/Widgets.jsx`、`Popover.jsx`、`data.js` 的额度部分无命中，`data.js` 里的 `sort` 全属其他界面。
- 💡 有界改进：二选一并写清楚——在字段清单里加上承载该解析结果的字段（例如最紧窗口的键），或改成「`windows[]` 按 used share 降序排列，最紧即首元素」并把该顺序写成契约。两者都能让「结构上一致」成立，措辞与实现二选一即可。

#### SQ-ARCH-R1-F5 — 低：Plus 账号的窗口形状是未观察到的事实，却写成了已观察

- 位置：`architecture.md:110-116`。
- 行为风险：这一段把「a Plus account carries both a 5-hour `primary` and a 7-day `secondary` there」当作既成事实陈述，用来论证映射必须与采样形状无关。设计结论本身是对的且更安全，但这句断言在本主题的证据里没有来源：`requirements.md:65-76` 记录的唯一一次观察是 `prolite` 账号，`planType` 只观察到一个值（`requirements.md` 开放问题 2 明说「One value was observed. The set is unknown」）。本文档在别处对「观察到的 / 没观察到」极为克制——`:118-121` 的 `not_reported` 特意写明「was not observable on the sample account」，C4 逐条写明观察值——唯独此处失了这个标准。而由这句话派生的 `codexPlus` 夹具已经进了标本，后续读者容易把它当成录到的响应。
- 证据：`rg -n "Plus|plus"` 在 `requirements.md` 无任何 Plus 观察记录；`prototype/src/data.js:680-681` 的注释反而是准确的——「采样账号是 prolite，主限额只回了 7 天窗」，明说该夹具是为避免写死采样形状而构造的，并未声称观察到 Plus。标本比架构更谨慎。
- 💡 有界改进：给这句话加上来源标注或降级为假设（例如「Plus 账号预计在主限额上同时返回 5 小时与 7 天窗口；这一点未在本主题中观察到，映射因此不依赖任何特定形状」）。设计与夹具都不必改。

### 🟢 优点

- **每一处仓库引用都对得上，没有一个是凭印象写的。** 逐条核过：`docs/specs/cli-design.md:54-55` 确为 Non-Goals 中的凭据条款；`provider.Service.Current` 确在 `internal/provider/service.go:692`，且其文档注释「without reading or decrypting credential values」与 C1 的论证一字不差；`OfficialProviderName` 确在 `:118`；`internal/desktop/desktop.go:21` 确为 `WireVersion = 1`，`:38` 确为不匹配即拒；`config_matched`/`observed_provider`/`conflict_scan` 确在 `internal/usage/routes.go:355-356`;`topLevelValueSpan`/`insertTopLevelValue` 确在 `internal/usagehook/config.go:702,750`。C0 第 2 条「唯一一条既有网络路径」也成立——`rg -l "net/http"` 在 `internal/` 与 `cmd/` 下（排除测试）只命中 `internal/usage/price_update.go` 一个文件。一份架构文档里全部外部引用都可核验，这不是常态。
- **C1 那个「记录值 vs 观察值」的取舍写法值得保留。** 它先说选了哪个，再说为什么（记录值总是有的、不需要 Hook 投递、自己不会失败），然后把代价明说（手工改配置的用户可能被误探或漏探），最后给出缓解方向与选择该方向的理由（有分歧时抑制，因为「never probes an account the product is not sure it should」）。取舍、代价、缓解、以及缓解为何朝这个方向——四样齐全。
- **拒绝被当作答案来写。** C2 的映射表把每个不采纳的厂商字段单列一行并给理由（账号级标识符、厂商营销文案、本主题不呈现的拒付与推销状态），C4 明确丢弃「what's contributing」并说明理由是「两个答案回答同一个问题」，C7 把不存历史写成一次有意识的拒绝而不是遗漏。开篇那句「A refusal is a real answer」在正文里确实被贯彻了。
- **C4 的 all-or-nothing 与严格匹配是对的方向。** 「一条解析失败则整个客户端结果为 `parse_failed`，不发部分数字」，以及「宁可报告解析失败也不放宽模式」，理由都写在旁边——放宽的模式会在厂商改文案时凑巧匹配上，产出一个没有任何变化信号的数字。这正是上一份文档（`ux/settings-quota.md`）两轮 FAIL 的同一类失效，架构在这里主动挡住了。
- **C8 把 Claude 归因缺口做成了字段而不是备忘。** `attribution_confirmed` 随每个 payload 走，理由写得很准：「a surface that forgets to render it is a visible omission rather than an invisible one」。同一段还把「新鲜度不能替代归因」单独点出来，堵住了最容易犯的那个替代。
- 七成员原因集在三处完全一致：`architecture.md`、`ux/menubar-quota.md` 与 `prototype/src/i18n.js:57-65` 逐个比对无差异，`not_reported`/`not_official`/`never_probed`/`probe_failed`/`parse_failed`/`not_consented`/`probe_disabled` 均可渲染。
- `docs/specs/cli-design.md:53`（「query provider billing, subscription, or usage APIs」）看似与本主题冲突，实则已由 `requirements.md:287-302` 的 Contract changes 一节认领并要求修订，`tasks.md:231,254` 把该修订派给任务 7 收口。架构引用 `:54-55` 只谈凭据边界是正确的分工，不是漏掉了 `:53`。

### 📝 总结

- 五项发现：`SQ-ARCH-R1-F1`、`SQ-ARCH-R1-F2`、`SQ-ARCH-R1-F3` 为中，`SQ-ARCH-R1-F4`、`SQ-ARCH-R1-F5` 为低，全部 OPEN。按本项目发现政策，低严重度同样阻断 PASS。Review 保持未勾选。
- 三项中等发现是同一类：决策完整性。文档在「拒绝什么、为什么」上做得很好，在「实现者照此能否不发明契约」上还差三格——读取关闭与已注册状态栏的交叉、陈旧判据、以及窗口标识。这类缺口在架构层的代价高于在代码层：契约里少一行，会被下游任务、测试和界面各抄一遍。
- Reviewer：Claude Code（`claude-opus-5`，1M context）；Method：单主会话逐节评审，按设计/契约类目标的维度（前提有效性、与现有实现的一致性、场景覆盖、决策完整性、内部矛盾、隐藏耦合、假设依赖、规则与现实）判定；逐条核验文档对仓库源码与上游文档的每一处引用；与三份界面文档的 Data requirements 逐字段对账；与标本 `prototype/src/data.js`、`i18n.js` 对账。非冷上下文独立评审，未委派，未使用低模型层级。
- Scope：`architecture.md` 全文。未修改产品代码、测试、配置、标本或被评审文档。
- Reviewed state：HEAD `01c7234f4d99de2f602d9bddbd607d0db995c90c`；document blob `b7bec91363be04d0491007fc7c69485ece0635ef`。
- Prototype：[输入清单](evidence/architecture-r1/prototype-manifest.sha256)，沿用本主题逐文件 SHA-256 清单方法；清单 SHA-256 `72067191a79d6772dc7d4390fb9505cc3f520f09f61023cbda94b030de162c3b`。与 `ux/settings-quota.md` 交付时的标本字节一致。
- Content fingerprint：`81db1f8d41c11382af7591dc09b8fbb2dcf68dd7af2bed812e2461fae4265c71`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-arch-design`。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:architecture.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:architecture:01c7234:b7bec91:7206719`。本轮之前该 WorkUnit 与其 criterion 在库中不存在，按阶段授权以既有三准则形状建立（`architecture`、`review`、`l0`）后再记录证据。
- 项目自带的同类检查器：`scripts/check-topic-docs.sh` 是本项目为文档集提供的检查器，本轮实际运行，exit 0。它校验文档集一致性，不校验单份契约的决策完整性——本轮三项中等发现都在它的覆盖之外，这是工具边界而非工具失效。
- L0：`make check-whitespace`、`git diff --check`、`scripts/check-topic-docs.sh` 全部 exit 0。
- 残余不确定性：C2 的 JSON-RPC 握手序列、C3 的 statusline payload 形状与 C4 的散文格式都是对厂商行为的断言，只能对照 `requirements.md:57-134` 记录的 2026-09-08 观察核验，无法在本轮独立复现——本评审据其记录判定一致性，不据其判定厂商现状。C5 的陈旧阈值、C9 的退避系数属未定量项，其中前者已作为 F2 记录，后者因 clause 12 只要求「退避而非按标称间隔重试」而留有实现裕度，不记为发现。
- 完成门禁：FAILED。criterion `architecture` 记 `fail`（F1/F2/F3 三项决策完整性缺口）、`review` 记 `fail`（五项发现未关闭）、`l0` 记 `pass`。
- 未跨越主题完成边界；不建议提交或推送。

### 下一步指令

修复：subscription-quota / reviews/architecture.md / SQ-ARCH-R1-F1 SQ-ARCH-R1-F2 SQ-ARCH-R1-F3 SQ-ARCH-R1-F4 SQ-ARCH-R1-F5

WORKFLOW_WORKSPACE: agent-deck.subscription-quota

## Round 2 — 2026-09-10

## 📋 订阅额度架构复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **SQ-ARCH-R1-F1 — CLOSED**。C9 新增的转换段（`:380-424`）不只补了缺口，还把三种可能处置列成表并说明另外两种为什么更差：留着继续入库会让开关变成谎言，留着但丢弃则「对数据诚实、对足迹不诚实」——别人的配置里仍然常驻并空跑 AgentDeck 的命令，而额度开关不给任何提示。选定「关闭读取时一并注销」。与 clause 1 的张力也解开得干净：clause 1 约束的是**关闭态**（处于关闭期间不写），而注销发生在**进入该态的转换中**，因此无需改动需求措辞——文档明说「the clause simply never addressed the transition」，而不是把需求改成迁就实现。后续影响也写全了：复用 C3 的 best-effort 复原（因此 `restore incomplete` 也可能从这条路径浮现）、同意标志随通路一并清除（理由是「同意一个跑不起来的通路不该被静默保留」）、已存观测不受影响。
- **SQ-ARCH-R1-F2 — CLOSED，且给了判据之外的东西**。`:245-278` 给出 `allowed_age(window) = max(window_minutes / 10, 2 × probe_interval)`，配一张两个窗口 × 两个间隔端点的取值表，逐格复算无误（5 小时窗：300/10 = 30 分，5m 间隔取 30 分，30m 间隔取 60 分；7 天窗：10080/10 = 1008 分 = 16 时 48 分，两个间隔下均取该值）。更值得记的是它为 `max` 的**两半各自**给了理由：十分之一是缩放本身，能在两端都给出合理值而不需要一张手挑常量表；下限取两倍探测间隔是为了防止标志按产品自己的节奏而不是按数据触发——30 分间隔下若用 30 分阈值，几乎每个数字都会在下次刷新前被标为陈旧，等于告诉用户配置坏了，而它正按配置工作。最后还补了一句「这是阈值不是准确性承诺」，把它能说明什么与不能说明什么划开。
- **SQ-ARCH-R1-F3 — CLOSED，两半都补上了**。C6 新增 `window_key` 定义表（`:295-313`），两个客户端各给形式与取值，并说明 Codex 形式为何是派生而非发明（`limitId` 已是厂商稳定标识，槽位是同一 limit 下两窗口的唯一区分；`primary` 取裸 `limitId` 让常见的单窗口情形拿到最短键），以及 Claude 的两个名字取自厂商 payload、**散文路径解析进同样两个键**——这一句同时关掉了我在 Round 1 未列为独立发现的那个隐患：一个客户端不会因为哪条路径应答而产生两套窗口词汇。还补了一句「键对界面不透明，只是存储／告警去重／重置检测的连接键，标签来自 `window_minutes` 与 `label`」。C3 新增段（`:155-168`）把 Claude 的 `window_minutes` 作为**本地常量**提供而非厂商字段，理由是名字本身陈述了长度所以常量不会漂移，且记为本地常量是为了让未来真带长度的 payload 能替换它而不是与它「静默一致」；名字不在这两个之内时长度为 `not_reported`，界面只按名字标注。
- **SQ-ARCH-R1-F4 — CLOSED，且比我给的两个选项都好**。新增 `tightest_window_key` 进入 C11 字段清单（`:484`）并定义（最高 `used_percent`，构建 payload 时解析一次，无窗口时为 null）。关键是它**显式否决了排序方案并给了理由**（`:495-500`）：popover 有意按厂商顺序列窗口——账号主限额在前、按模型的限额在后——那个顺序是菜单栏设计的一部分，而一份 payload 不可能对一个界面排序、对另一个不排序，所以答案作为独立字段传递，`windows[]` 保持既有顺序。该顺序与标本一致：`prototype/src/Popover.jsx:615` 直接 `entry.windows.map(...)`，不排序。
- **SQ-ARCH-R1-F5 — CLOSED，并追到了传播链**。`:110-125` 改写为：本主题只观察到 `prolite` 一个账号，另一种套餐是否在主限额上同时返回两个槽位「**has not been observed here**; it is an assumption, and it is named as one rather than stated as fact」，随后说明映射写成与该假设无关因此假设成不成立都不影响契约。最后一句尤其到位——明说标本的 `codexPlus` 夹具是由该假设构造而非录自厂商响应，且夹具自己的注释就是这么写的。这正好堵上了我记录该发现的理由：未观察到的形状已经进了夹具。
- **验证表补了五行，与修复一一对应**（`:568-572`）：读取关闭时注销通路并清同意标志、`allowed_age` 在两个窗口长度与每个可选间隔下的取值含 30m 时下限接管与阈值两侧各一分钟的边界、两个客户端的 `window_key` 构造含 Codex `secondary` 槽位与 Claude 两条路径产出同样两个键、Claude `window_minutes` 及名字不在集合内的回退、`tightest_window_key` 及 `windows[]` 保持厂商顺序。开放问题也新增两条（6、7），其中第 7 条把两个常量明说为「chosen, not measured」，并给了修订规则——凭真实抱怨修订，而不是按口味调常量。
- **未改动的章节可以用算术确认，而不是靠断言**：全文 456 → 606 行，增量 150 完全由六节吸收（C2 +9、C3 +15、C5 +30、C6 +20、C9 +46、C11 +17、验证表 +5、开放问题 +8）。各节起始行的位移与之逐节吻合，因此 C0、C1、C4、C7、C8、C10、C12 与 Round 1 所评内容逐字未变，本轮无需重评。
- `updated:` 已随本次实质改动改为 `2026-09-10`，与文件 mtime `2026-09-10 06:50:47` 一致。

### 📝 总结

- 逐项处置：`SQ-ARCH-R1-F1` CLOSED、`SQ-ARCH-R1-F2` CLOSED、`SQ-ARCH-R1-F3` CLOSED、`SQ-ARCH-R1-F4` CLOSED、`SQ-ARCH-R1-F5` CLOSED。本轮无新增发现。
- **两项属于其他目标的观察，按评审规则记录并指明归属，不阻断本目标**：
  1. `ux/settings-quota.md` 的标本未建模「父开关关闭」这一转换——`prototype/src/Settings.jsx:88` 的 `set("quotaProbe")` 只合并 `quotaProbe`，`quotaStatusline` 保留旧值，因此关闭读取后状态栏开关仍渲染此前的 `on` 并置灰，与 C9 新规则要求的「读作 off」相矛盾。该缺口由 C9 `:418-424` 主动指出并已作为载体记入 `ad-sq-doc-ux-settings-quota-design`（2026-09-10 13:52 评论，本轮已读回确认存在）。需注意该任务已 `closed` 且其文档已随 `01c7234` 提交，因此这条载体位于一个不在任何队列中的任务上。
  2. `ux/menubar-quota.md:93-94` 仍把「a Plus account carries both a 5-hour and a 7-day window on the main limit」写成既成事实——正是 `SQ-ARCH-R1-F5` 在本文档内改掉的那句。架构的修复追到了 `codexPlus` 夹具，未追到这份同源文档。该文档同样已 PASS 并随 `01c7234` 提交。
  两项都需要用户决定处理车道（`.agent-instructions/beads.md` 的 Bug lane 由用户确认），本评审不代为开单、不重开已关闭任务、不改动其他目标的文档。
- Reviewer：Claude Code（`claude-opus-5`，1M context）；Method：单主会话逐项复评，五项发现逐条按当前内容核验；新增内容按设计/契约维度重评（前提有效性、内部矛盾、决策完整性、场景覆盖、假设依赖）；`allowed_age` 取值表逐格复算；改动范围以行数增量与各节起始行位移交叉核对；新声明对标本与兄弟文档逐条回查。非冷上下文独立评审，未委派，未使用低模型层级。
- Scope：`architecture.md` 全文。未修改产品代码、测试、配置、标本或任何被评审文档。
- Reviewed state：HEAD `01c7234f4d99de2f602d9bddbd607d0db995c90c`；document blob `d064326b0db266c053a201f9b5067e557189b95f`（Round 1 为 `b7bec913`）。
- Prototype：与 Round 1 字节一致，沿用 [Round 1 输入清单](evidence/architecture-r1/prototype-manifest.sha256)，清单 SHA-256 `72067191a79d6772dc7d4390fb9505cc3f520f09f61023cbda94b030de162c3b`；逐文件重算无差异，故本轮不另建证据目录。
- Content fingerprint：`66f70215a09b2d696b7b7873e4547e83d1911adb2c6c0f1c30842fc74bbfcccd`，SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Workspace：`agent-deck.subscription-quota`；branch `feature/subscription-quota`；Task：`ad-sq-doc-arch-design`。
- WorkUnit：`urn:agent-deck:work-unit:subscription-quota:architecture.md`；Target ContentState：`urn:agent-deck:content-state:subscription-quota:architecture:01c7234:d064326:7206719`。
- 项目自带的同类检查器：`scripts/check-topic-docs.sh` 本轮实际运行，exit 0。
- L0：`make check-whitespace`、`git diff --check`、`scripts/check-topic-docs.sh` 全部 exit 0。
- 残余不确定性：C2 的握手序列、C3 的 statusline payload 形状、C4 的散文格式仍只能对照 `requirements.md:57-134` 记录的 2026-09-08 观察核验，无法在本轮独立复现；`allowed_age` 的两个常量按其自身开放问题 7 的说法是选定而非实测，本轮据其自洽性与理由充分性判定，不据实测判定。C6 定义的 Codex `primary` 槽位取裸 `limitId`，而标本 `prototype/src/data.js:685-688` 的夹具键写作 `codex_primary`；因 C6 同段已明确「键对界面不透明、绝不作为标签」，标本该字段不在契约覆盖范围内，故不记为发现，在此记录以免后续读者重新「发现」它。
- 完成门禁：VERIFIED。criterion `architecture`、`review`、`l0` 三项均记 `pass`；新增一个 ContentState、三个 Evidence 与六个关系，关系 preflight 全部 ok，写入后已重新查询门禁确认；旧态观察未重标、未改写。
- Documents 矩阵中 `architecture.md` 的 Review 已勾选。主题仍有 `tasks.md` 未评审，未跨越主题完成边界。

### Task checkpoint

Task checkpoint：`ad-sq-doc-arch-design`；content_state `66f70215a09b2d696b7b7873e4547e83d1911adb2c6c0f1c30842fc74bbfcccd`；门禁 VERIFIED

提交建议：`docs/topics/subscription-quota/architecture.md`、`docs/topics/subscription-quota/reviews/architecture.md`、`docs/topics/subscription-quota/reviews/evidence/architecture-r1/`，以及本任务在 `docs/topics/subscription-quota/tasks.md` Documents 矩阵中的 Review 勾选。工作区内 `tasks.md` 的实现分解草稿属于 `ad-sq-doc-tasks-design`，尚未评审，不应混入本次提交。

推送建议：`origin/feature/subscription-quota`；该分支在 origin 上尚不存在，且提交与推送均需另行显式授权——评审阶段授权不含交付动作。

两项建议均为建议，不执行也不授权交付。

### 下一步指令

评审：subscription-quota / tasks.md

WORKFLOW_WORKSPACE: agent-deck.subscription-quota
