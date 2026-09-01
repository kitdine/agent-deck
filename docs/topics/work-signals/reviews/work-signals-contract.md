---
status: active
topic: work-signals
subject: work-signals-contract
---

# Review log — work-signals / work-signals-contract

## Round 1 — 2026-08-31

- Reviewed state:
  - HEAD `9e5adde1dd45755d685beb93e14aa443a6682ef0`，工作区未提交
  - `docs/specs/cli-design.md` blob `5182cb94740b176e9ab1172932e716c5750e7b47`
  - `docs/specs/cli-manual.md` blob `5537cb830871a554f49231bba4dc96ea6f47a191`
  - `docs/topics/work-signals/tasks.md` blob
    `0012ac17a161c256f5152cf1e2f46928cac77fa8`
  - `docs/status.md` blob `94bdd98805a39f3cebbb86f7ba4a4970c038dcb7`
  - 与 Development 阶段记录的 CEv1 candidate
    `08d63448cd3bc99b779e83e5c080cd112eb3e8080043c2165dc5236e8c115472` 的
    `digest_recipe` 四个组成部分逐一相符（含 `docs/status.md#work-signals-row`
    行摘要 `7192fe82…`），即本轮评审的正是该证据绑定的内容状态。
- Reviewer: Claude Code，Development 路线关闭后的独立正式 Review。实现由 Codex
  产出，本次评审不共享其上下文与角色。
- Method: 目标类别为 **contract**（文档尚未被任何实现"满足"，而是反过来把已交付
  行为回写进契约），因此按设计/契约维度评审，逐条把文档断言与仓库现状对照，而不
  接受文档的自述。schema 与 parser 版本对照 `internal/store/store.go`、
  `internal/store/migrations.go`、`internal/usage/usage.go`；隐私边界对照
  `internal/activity/extraction.go`、`internal/activity/classify.go` 与
  `usage_tool_calls` / `usage_tool_files` 的实际列；命令与 flag 对照
  `cmd/agentdeck/main.go` 的 Cobra 注册、`cmd/agentdeck/usage_signals_text.go`、
  `cmd/agentdeck/usage_stats_layout.go`、`cmd/agentdeck/session_show_text.go`；
  JSON 形状对照 `internal/usage/signals_report.go` 与
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json`；wire 家族对照
  `internal/desktop/desktop.go` 与 `apps/macos/AgentDeckApp/MenuBarViewModel.swift`、
  `MenuBarPanelViews.swift`。项目自带的文档集审计
  `scripts/check-topic-docs.sh` 与隐私审计 `scripts/check-privacy.sh` 作为本类目标
  的必需证据实际执行，未凭肉眼断言完备性。CEv1 门禁为只读查询。
- Scope: 本轮改动的四个文档（`cli-design.md`、`cli-manual.md`、`tasks.md`、
  `docs/status.md`）及其所断言的仓库行为。生产代码、测试与配置全程只读，工作区
  除本记录外未做任何写入。

## 📋 work-signals-contract 评审报告

📊 总体评分：8/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

[`docs/specs/cli-design.md:1162`、`docs/specs/cli-design.md:1167`、
`docs/specs/cli-manual.md:491`、`docs/specs/cli-manual.md:439`]
**[R1-F1] `usage signals` 会同步隐式扫描且没有 `--no-scan`，两份契约都没有回写这
一交付行为，反而继续把隐式扫描的命令集合枚举为"stats 与 summary"。**

- 行为风险：任务 6 的交付物就是"the new command and flags"的回写。设计文档在
  1162 行明确枚举"Progress covers the implicit scan performed by `usage stats`
  and `usage summary` as well as explicit `usage scan` and `usage rebuild`"，
  1167 行进一步把"同步扫描 + `--no-scan` 逃生口"写成这两个命令的成对性质；手册
  491 行是同一句的中文对应。这两处是**穷举式**的规范性枚举，读者据此判断某个
  usage 命令是否会在输出前阻塞扫描。`usage signals` 实际会阻塞扫描、会打印扫描
  进度、失败时会带 `partial: true` 与 `scan_incomplete`，却**没有** `--no-scan`
  可用。于是契约给出的结论与交付行为相反：一个在大历史上要等数分钟且无逃生口的
  命令，被文档归入"不隐式扫描"的一侧。手册 439 行的新增表行同样加剧了误读——
  同表的 `usage stats` 行以"默认扫描后输出"开头，`usage signals` 行以"直接读取
  持久化 Work Signals 派生"开头，逐行对读只会得出它不扫描的结论。这正是本仓库
  反复付过代价的"规范被复制成数据后与现实分叉"，而且它落在最早被下游引用的一层。
- 证据：`cmd/agentdeck/main.go:3234-3252` 中 `usage signals` 的 `RunE` 无条件执行
  `_, scanErr := s.Scan(ctx)`，随后把 `scanErr != nil` 转成 `partial` 与
  `scan_incomplete` warning；命令只注册了 `--period`、`--client`、`--kind`、
  `--sub`、`--activity` 五个 flag（3253-3257 行），没有 `--no-scan`。
  `cmd/agentdeck/main.go:3126` 的 `withUsage` 对**每个** usage 命令都设置
  `service.Progress = newUsageProgress(opts.stderr, opts.quiet)`，因此扫描进度同样
  覆盖 `usage signals`。对照 `usage stats`（3228 行注册 `--no-scan`）与
  `usage summary`，可见"扫描 + `--no-scan`"这一成对性质在 `usage signals` 上被
  单边打破，而两份契约都没有记录这个差异。检索确认两份文档的本轮新增文字中都没有
  任何 scan 相关表述：`git diff docs/specs/` 的新增行内无 `scan`/`扫描` 命中。
- 💡 有界修复：在 `cli-design.md` 1162 与 1167 两处枚举中加入 `usage signals`，
  并明确它同步扫描却**没有** `--no-scan`（如需保留该缺口，就把"为什么没有"写成
  一句契约，而不是留给读者推断）；在 `cli-manual.md` 491 行做同样的补充，并在 439
  行表行的第一列或约束列写明"默认扫描后读取持久化派生；无 `--no-scan`"。不要修改
  代码：本轮评审只判定文档未回写交付行为，`usage signals` 是否应当获得 `--no-scan`
  是新的产品决定，超出任务 6 的范围。

### 🟡 建议改进 — 推荐

[`docs/specs/cli-design.md:1042`] **[R1-F2] usage 报告家族的渲染器豁免枚举没有
纳入 `usage signals`。**

- 证据：该句写作"The usage report family (`usage summary`, `usage stats`,
  `usage sessions`, and `usage diagnose`) is explicitly excluded and follows the
  dedicated responsive section, bar, and continuation-line contract below"，用于
  决定哪些命令**不**走统一 ASCII grid。`cmd/agentdeck/usage_signals_text.go:13-32`
  显示 `usage signals` 使用 `statsDefaultWidth`/`statsMinWidth`/`statsMaxWidth`
  与 `usageTextPrimitives`，即同一套宽度感知 section primitives，而非 grid。枚举
  未更新，后续实现者或评审者据此会把 grid 契约错误地套到该命令上。手册第 30 行与
  943 行是概括表述（"usage 报告使用其专用的宽度感知 section/row primitives"），
  不含穷举，无需改动。
- 💡 有界改进：把 `usage signals` 加入 1042 行的括号枚举。

[`docs/specs/cli-manual.md:439`、`docs/specs/cli-design.md:1383`]
**[R1-F3] `--activity` 隐含 `--sub` 这一 flag 交互没有进入任何长期契约。**

- 证据：`cmd/agentdeck/main.go:3249` 传入
  `IncludeSub: signalsSub || signalsActivity != ""`，
  `internal/usage/signals_report.go:225-236` 据此才输出 `sub` 行；
  `docs/topics/work-signals/ux/cli-work-signals.md:82` 与 226-227 行确认这是设计
  意图（"Implies `--sub` when given a category"、"`sub` is present only under
  `--sub` or `--activity`"）。但 topic 文档会随 topic 归档退役，`cli-design.md`
  与 `cli-manual.md` 才是幸存契约，两者都只并列列出 `--sub` 与 `--activity`，未
  记录二者的蕴含关系。手册该表的"约束"列正是同表记录 flag 交互的位置（同列已写
  `--activity` 必须与 `--model` 同用）。
- 💡 有界改进：在手册 439 行的约束列补一句"`--activity` 隐含 `--sub`"，或在设计
  1383 段落的 `--sub`/`--activity` 描述处补同一事实。

### 🟢 优点

- **不预测、只读交付**。任务 6 明确要求"read it from the delivered migration, do
  not predict it here"，而对应的 Beads 描述里仍留着过时的"schema v19"。本轮交付
  写的是 v20/v21 与 parser 5/6，并逐条对得上
  `internal/store/migrations.go:141-195` 与 `internal/store/store.go:21`
  (`CurrentSchemaVersion = 21`)、`internal/usage/usage.go:42`
  (`usageParserVersion = 6`)。v21 段落对"为什么是 DROP + CREATE 而不是四条
  ALTER"的转述也与迁移文件里的注释一致。
- **隐私边界从绝对句改写成了可核验的两段式**，而且每一项都能落到具体列上：
  `usage_tool_calls` 的 `tool_kind`/`mcp_server`/`command_read`/`command_hint`、
  `usage_tool_files` 的 `path_digest`/`base_name`/`wrote`，`command_hint` 的取值
  确实只有 `testing`/`chore`/空（`internal/activity/classify.go:18-19`），
  digest 确实按机器身份加盐、base name 确实截断到 128 字节
  （`internal/activity/extraction.go:98-133`）。"transient read → reduce →
  persist"的顺序写法比旧的"什么都不留"更难被后续改动悄悄证伪。
- **JSON 与 wire 的字段清单逐字可核**。`signals_report.go:28-78` 的 `cost_basis`、
  六个 nullable workflow 字段、tooling 的 calls/groups/rows/top MCP 与文档列举完全
  一致；`internal/desktop/desktop.go:140-185`、`542-618` 证实三族 keyed items、
  `today|7d|30d` × `all|codex|claude`、Activity 四类固定顺序、Tooling ≤5 行的生产者
  上界，以及 `WireVersion = 1` 未被抬高。
- **桌面表面那段没有把"整体不可用"和"本族无数据"混为一谈**，与
  `MenuBarViewModel.swift:795-960` 的 per-family `uncapturedSections` 加 per-item
  查表、以及 `MenuBarPanelViews.swift:690-745` 的渲染分支一致。
- **审计工具真的跑了，而且失败被正确归属**。`scripts/check-topic-docs.sh` 只报告
  并发的、未跟踪的 `schema-version-signal` topic 的两个声明未写文件，Work Signals
  无任何 finding；该脚本没有 scoped 模式，因此这条无关失败被隔离而不是顺手修掉，
  这个判断是对的。

### 📝 总结

本轮评审的是 HEAD `9e5adde` 之上未提交的四文档改动（blob 见"Reviewed state"），
即任务 6 `work-signals-contract` 把已交付的 Work Signals 行为回写进
`cli-design.md`（版本 27）与 `cli-manual.md`。schema/parser 版本、隐私边界、
`usage stats` 默认区块、`usage signals` 的 flag 与 JSON 形状、
`session show --activity` 的 `SIGNALS` 行、wire-v1 三族与桌面 captured 渲染，逐条
与仓库现状对照后均属实，没有发现任何一条"文档声称而代码没有"的断言——这正是本类
目标最容易出错的地方，本轮没有出错。

判定为 FAIL 的原因是回写不完整而非不正确：`usage signals` 的隐式同步扫描与
`--no-scan` 缺口没有进入契约，而两份文档恰好都对它的同族命令做了穷举式枚举，于是
沉默变成了错误结论（R1-F1）。另外两条较轻的遗漏（渲染器豁免枚举、`--activity`
隐含 `--sub`）同属"新命令没有被登记进既有枚举"这一类，放在同一轮修复里成本最低；
按本项目"没有 finding 可以越过 PASS"的规则，它们同样必须在下一轮前关闭。

残余不确定性有二。其一，`usage signals` 是否**应当**拥有 `--no-scan`，本轮不作
判断：那是产品决定，任务 6 的义务只是如实记录当前行为。其二，Development 阶段已
在同一 candidate content state 上写入了五条 `outcome: pass` 证据（含 `cli-contract`
一条，其 check 声称 flag/envelope/availability 三者一致），本轮否证了其中的
`cli-contract` 部分；本轮未改写那些历史节点（改写会重写历史记录），修复必然产生
新的 content state，届时须在新状态上重新记录证据，旧 candidate 的证据不得复用。

### 下一步指令

修复：work-signals / reviews/work-signals-contract.md / R1-F1, R1-F2, R1-F3

## Round 1 — repair — 2026-08-31

- Repairer: Codex
- Scope: R1-F1, R1-F2, and R1-F3 only. Production code, tests, schema/privacy,
  JSON, wire, Session, desktop, and unrelated `schema-version-signal` work are
  unchanged.
- Repaired state: HEAD `9e5adde1dd45755d685beb93e14aa443a6682ef0`,
  working tree uncommitted; synchronized scoped candidate
  `714ed56df3f12f9cd5ed675f2c37448cdde64837b2fe2f11ac2fceb53ff1e9b6`.

### R1-F1 — CLOSED

Both surviving specifications now state that `usage signals` participates in
the synchronous implicit-scan and stderr-progress contract. CLI Design separates
the stored-only behavior explicitly: stats and summary provide `--no-scan`,
while signals has no `--no-scan` and always attempts a scan before reading its
persisted derivation. A failed scan returns the last committed signals with
`partial: true` and `scan_incomplete`. The Chinese manual records the same
behavior both in the command table and in Usage default-output rules. No command
or flag behavior changed.

### R1-F2 — CLOSED

CLI Design's exhaustive renderer-exemption list now includes `usage signals`
beside the other usage reports. This matches `renderUsageSignalsWithOptions`,
which uses the stats width bounds and `usageTextPrimitives` rather than the
shared ASCII grid.

### R1-F3 — CLOSED

CLI Design and the Chinese command row now state that `--activity` implies
`--sub`. This matches `IncludeSub: signalsSub || signalsActivity != ""` and
preserves the existing category/subcategory filtering behavior.

### Evidence

```text
current source and contract anchors
  -> s.Scan + scan_incomplete + five registered flags confirmed
  -> renderer uses stats widths and usageTextPrimitives
  -> IncludeSub is signalsSub || signalsActivity != ""
  -> repaired scan/renderer/flag statements present in both living specs
make check-whitespace
  -> PASS
git diff --check
  -> PASS
bash scripts/check-topic-docs.sh
  -> only the same two out-of-scope, untracked schema-version-signal gaps;
     no Work Signals finding; the script has no scoped mode
work-signals:work-signals-contract gate at candidate 714ed56d…
  -> VERIFIED, 5/5
```

The Development repository-wide Go L2 evidence and Review privacy evidence are
reused because this Repair changes no production source, test, dependency,
fixture, configuration, toolchain, or privacy statement. CEv1 retains the old
`08d63448…` pass evidence and the Round 1 CLI fail as invalidated history; new
candidate evidence supersedes both and is the only valid lineage for all five
criteria.

- Completion gate: VERIFIED
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — independent re-review — 2026-08-31

- Reviewed state:
  - HEAD `9e5adde1dd45755d685beb93e14aa443a6682ef0`，工作区未提交
  - `docs/specs/cli-design.md` blob `de77284811c994c219cf828c1d05d8d1b729df52`
  - `docs/specs/cli-manual.md` blob `d63017a48451e9a8682b2da326d835c31525ef36`
  - `docs/topics/work-signals/tasks.md` blob
    `123a10e18adba71deda4d2a1357e0df1bfdce039`
  - `docs/status.md` blob `a1185047e698fc84d024496249dd4616e80fdd36`
  - 与 Repair 记录的 CEv1 candidate
    `714ed56df3f12f9cd5ed675f2c37448cdde64837b2fe2f11ac2fceb53ff1e9b6` 的
    `digest_recipe` 四项逐一相符（`docs/status.md#work-signals-row` 行摘要
    `2cbad6c4…` 亲自重算命中），即本轮复评的正是该证据绑定的内容状态。
- Reviewer: Claude Code，与 Round 1 同一评审角色，Repair 由 Codex 完成。
- Method: 先把 Round 1 的三条 finding 逐条对当前内容重新证伪，再检查修复本身是否
  引入了新的错误断言，最后核对本轮内容状态所绑定的四个文件（含 `tasks.md` 与
  `docs/status.md` 行）是否自洽。隐式扫描命令集合这次不接受文档自述，而是穷举
  `cmd/agentdeck/main.go` 的 usage 命令树，确认 `s.Scan(ctx)` 只出现在 summary、
  stats、signals 三个 report 命令与显式 `usage scan`/`usage rebuild` 上。Round 1
  已核验的 schema/隐私/JSON/wire/Session/桌面段落通过 `git diff` 确认逐字未动，
  因此复用其证据而不重跑。`scripts/check-whitespace.sh`、`git diff --check`、
  `scripts/check-topic-docs.sh`、`scripts/check-privacy.sh` 重新执行。CEv1 门禁为
  只读查询。
- Scope: Round 1 的三条 finding、Repair 的两份规范改动，以及本轮内容状态绑定的
  `tasks.md` 与 `docs/status.md`。生产代码、测试与配置全程只读。

## 📋 work-signals-contract 复评报告

📊 总体评分：8/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[`docs/topics/work-signals/tasks.md:612`、`docs/topics/work-signals/tasks.md:713`]
**[R2-F1] 同一轮 Review 在本 topic 唯一的状态权威里被记了两遍，措辞不同。**

- 处置：新增。
- 证据：`tasks.md` 开篇写明"This file is the only status authority for this
  topic"，而现在第 612 行与第 713 行各有一段 `Task 6 Review Round 1
  (2026-08-31): **REOPEN** on R1-F1…`。两段内容不同：612 行那段没有记录
  Development 阶段 CEv1 candidate `08d63448…` 的 `cli-contract` 证据已被否证、
  不得复用这一事实，713 行那段有。其余每个任务（0 至 5）的每一轮都只有一段，
  `grep -n "^Task [0-9] \(Development\|Review\|Repair\|Re-review\)"` 可见 612 与
  713 是全文唯一的重复对，所以这不是本文件的写法约定，而是 Repair 轮引入的重复。
  这正是本仓库反复付过代价的"同一事实有第二个落点"：两段此后各自老化，读者无法
  判断哪一段是记录。
- 💡 有界改进：只保留一段。建议保留第 612 行那段——它与任务 2 至 5 的分组位置
  一致——并把 713 行那段独有的 CEv1 否证结论并入其中，然后删除 713 行整段。不要
  改动两段所引用的 finding 编号与判定。

[`docs/status.md:67`、`docs/topics/work-signals/tasks.md:373`]
**[R2-F2] `docs/status.md` 的实现计数与 topic 状态权威相互矛盾。**

- 处置：新增。
- 证据：`docs/status.md` 第 67 行写"6/6 implemented, 5/6 reviewed"，而
  `tasks.md` 第 373 行的矩阵是 `| 6. \`work-signals-contract\` | [ ] | [ ] |`。
  按该表既有惯例（"Task 1 Review Round 1 … Both task cells remain unticked until
  Repair and independent Re-review close the finding"），REOPEN 后两格保持未勾选
  直到 Re-review 关闭，所以矩阵是对的，实现任务 1–6 中已勾选的是 5 个。
  `docs/status.md` 自己在开头声明"Topic-internal document/task cells, rounds,
  findings, and evidence remain in each topic's `tasks.md`"，即它是投影而非权威，
  投影与权威不一致时错的是投影。该矛盾由 Repair 轮重写此行时引入：Round 1 结束
  时该行是"5/6 implemented, 5/6 reviewed"，与矩阵一致。
- 💡 有界改进：把第 67 行的实现计数改回与矩阵一致的 `5/6 implemented`（本轮
  Re-review 判定为 REOPEN，矩阵两格继续保持未勾选），或在 Re-review PASS 的同一
  次同步里让两处一起变。两者取其一，不要两处各写各的。

### 🟢 优点

- **R1-F1 关闭，且修复的是契约而不是措辞。** `cli-design.md:1163` 与 `:1168-1175`
  现在把 `usage signals` 写进进度覆盖与同步扫描两处枚举，并明确它"deliberately
  has no stored-only mode"、扫描失败返回上次已提交数据并带 `partial: true` 与
  `scan_incomplete`；`cli-manual.md:439` 表行与 `:491-495` 记录同一契约，且约束列
  写明"没有 `--top` 或 `--no-scan`"。逐条对照代码属实：
  `cmd/agentdeck/main.go:3234-3257` 的 `RunE` 无条件 `s.Scan(ctx)`、只注册五个
  flag、把 `scanErr != nil` 转成 partial 与 `scan_incomplete`；`withUsage` 为每个
  usage 命令装 Progress。更重要的是，穷举整棵 usage 命令树后确认隐式扫描恰好只有
  summary、stats、signals 三个——修好的枚举现在是**完整**的，而不是补了一个漏了
  别的。
- **R1-F2 关闭。** `cli-design.md:1041-1042` 的渲染器豁免枚举已含
  `usage signals`，与 `renderUsageSignalsWithOptions` 使用 stats 宽度边界和
  `usageTextPrimitives` 而非 ASCII grid 一致。
- **R1-F3 关闭。** `cli-design.md:1392` 的"implies `--sub`"与
  `cli-manual.md:439` 约束列的"隐含 `--sub`"，对应
  `IncludeSub: signalsSub || signalsActivity != ""`。两份幸存契约都记住了这条
  会随 topic 归档消失的 flag 交互。
- **修复边界克制且可核。** `git diff` 显示两份规范相对 Round 1 只多了扫描、渲染器
  与 flag 交互三处文字，Round 1 已核验的 schema/隐私、JSON、wire-v1、Session、
  桌面段落逐字未动，因此那些证据可直接复用而不是重跑一遍走过场。生产代码、测试、
  fixture、配置零改动。
- **没有引入新的错误断言。** "扫描失败返回上次已提交数据"这条新写的行为在代码里
  成立：`s.Scan` 失败后命令继续读持久化派生并置 partial。

### 📝 总结

逐条处置：R1-F1 关闭，R1-F2 关闭，R1-F3 关闭；新增 R2-F1 与 R2-F2 两条。

本轮复评的内容状态是 HEAD `9e5adde` 之上未提交的四个文件（blob 见"Reviewed
state"），与 Repair 记录的 CEv1 candidate `714ed56d…` 的 recipe 四项逐一相符，
门禁在该 candidate 上 5/5 `pass` 即 VERIFIED。

判定为 FAIL 的原因不在契约本身——三条 Round 1 finding 都被正确关闭，修复既完整
又没有引入新的错误断言，本可以直接通过。问题出在审计轨迹：Repair 轮在改规范的
同时改了状态记录，结果 `tasks.md` 里同一轮 Review 有了两段不同措辞的记载
（R2-F1），`docs/status.md` 的实现计数与它自己声明为权威的矩阵对不上（R2-F2）。
两条都标为 🟡，因为都不影响交付的产品行为；但按本项目"没有 finding 可以越过
PASS"的规则，它们同样必须在下一轮前关闭——而且这类缺陷正是"低严重度不等于可以
带着走"的典型：两份记录一旦并存，此后每一轮都要重新判断哪一份是真的。

残余不确定性有二。其一，CEv1 在 candidate `714ed56d…` 上的 `predecessor-delivery`
证据声称"required task/status synchronization"已完成，本轮否证了这一部分；本轮
未改写该历史节点，修复会产生新的 content state，届时须在新状态上重新记录。其二，
R2-F1 保留哪一段是可选择的，本记录给出建议而非强制，只要求最终只剩一段且不丢失
CEv1 否证结论。

### Evidence

```text
git hash-object on the four bound documents
  -> de772848 / d63017a4 / 123a10e1 / a1185047, identical to Reviewed state
grep -n "usage signals" docs/specs/cli-design.md
  -> 1163 progress, 1168-1175 synchronous scan, 1041-1042 renderer exemption,
     1387-1392 flags and `--activity` implies `--sub`      <- R1-F1/F2/F3 closed
sed -n '437,441p;489,497p' docs/specs/cli-manual.md
  -> the same contract in the command row and the default-output rules
grep -n "s.Scan(ctx)" cmd/agentdeck/*.go
  -> main.go 3156 summary, 3192 stats, 3241 signals, 3261 explicit scan;
     desktop.go 150 is the menu-bar host, not a usage command
cmd/agentdeck/main.go:3234-3257
  -> unconditional scan, five flags, no --no-scan, scanErr -> partial +
     scan_incomplete, IncludeSub: signalsSub || signalsActivity != ""
bash scripts/check-whitespace.sh / check-privacy.sh; git diff --check
  -> exit 0
bash scripts/check-topic-docs.sh
  -> the two unwritten schema-version-signal documents only; no Work Signals
     finding. Unchanged from Development and Repair; the audit has no scoped
     mode, so that concurrent untracked topic stays isolated from this task.
```

- Completion gate: VERIFIED on candidate `714ed56d…` across all five criteria,
  and that gate result is not this round's verdict. This round records a `fail`
  on the `predecessor-delivery` criterion at that same candidate, following the
  Round 1 precedent of an appended fail node rather than a rewrite: the
  criterion's `pass` claims the required task and status synchronization was
  complete at `714ed56d…`, and R2-F1/R2-F2 disprove exactly that. The fail is
  bound to the re-reviewed state rather than to this round's post-synchronization
  blob, which the usual bind-to-the-final-blob rule would ask for: a `fail` says
  a named state did not hold, and the post-synchronization state was never
  re-reviewed. The repair produces a new content state that needs its own
  evidence.
- Round synchronization: `tasks.md` carries the Re-review Round 2 note in the
  Task 6 Development → Review → Repair chain, and the `docs/status.md` Work
  Signals row projects the REOPEN. Neither finding was repaired in this round.
  Writing that note shifted the duplicate block R2-F1 names from line 713 to
  line 738; the finding's line numbers remain correct for the bound blob
  `123a10e1…`, and the repair should locate both blocks by content.
- Verdict: FAIL

### 下一步指令

修复：work-signals / reviews/work-signals-contract.md / R2-F1, R2-F2

## Round 2 — repair — 2026-08-31

- Repairer: Codex
- Scope: R2-F1 and R2-F2 only. Both CLI specifications, production code,
  tests, dependencies, configuration, toolchain, and unrelated
  `schema-version-signal` work are unchanged.
- Repaired state: HEAD `9e5adde1dd45755d685beb93e14aa443a6682ef0`,
  working tree uncommitted; synchronized scoped candidate
  `8f870b32665a9fd46f126ae385fb226528a96fc90cf705206fff5847b3cf4a4a`.

### R2-F1 — CLOSED

`tasks.md` now contains exactly one `Task 6 Review Round 1` record, positioned
in the Task 6 Development → Review → Repair chain. The retained record now also
carries the removed duplicate's unique conclusion: Development candidate
`08d63448…` is disproven on `cli-contract` and must not be reused. The second
record was deleted in full; finding IDs and the `REOPEN` verdict are unchanged.

### R2-F2 — CLOSED

The Task 6 matrix remains `[ ] / [ ]`, following the existing REOPEN convention.
`docs/status.md` now projects that authority literally as `5/6 implemented,
5/6 reviewed`; neither source claims Development or Review completion before
independent Re-review PASS.

### Evidence

```text
rg '^Task 6 Review Round 1' docs/topics/work-signals/tasks.md
  -> exactly one result
Task 6 matrix / docs/status.md Work Signals row
  -> [ ] / [ ] and 5/6 implemented, 5/6 reviewed
make check-whitespace
  -> PASS
git diff --check
  -> PASS
bash scripts/check-topic-docs.sh
  -> only the same two out-of-scope, untracked schema-version-signal gaps;
     no Work Signals finding; the script has no scoped mode
work-signals:work-signals-contract gate at candidate 8f870b32…
  -> VERIFIED, 5/5
```

Only `predecessor-delivery` required new candidate evidence. Its new pass
supersedes both the Round 2 fail and the previous `714ed56d…` predecessor pass.
The other four criteria reuse their valid `714ed56d…` evidence because the two
specifications and every relevant product/test input are unchanged.

- Completion gate: VERIFIED
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 3 — independent re-review — 2026-08-31

- Reviewed state:
  - HEAD `9e5adde1dd45755d685beb93e14aa443a6682ef0`，工作区未提交
  - `docs/specs/cli-design.md` blob `de77284811c994c219cf828c1d05d8d1b729df52`
  - `docs/specs/cli-manual.md` blob `d63017a48451e9a8682b2da326d835c31525ef36`
  - `docs/topics/work-signals/tasks.md` blob
    `746522b410734603bd8c7ef61fcfa53557d6af82`
  - `docs/status.md#work-signals-row` 行摘要
    `4841d685781bf3578b770a0b3f2961dcfc483c57470fd755a41cec78c507b9b5`，亲自重算
    命中（`printf '%s'` 该行、不含换行）
  - 四项与 Repair 记录的 CEv1 candidate `8f870b32…` 的 `digest_recipe` 逐一相符，
    两份规范的 blob 与 Round 2 被评状态完全相同，即修复没有碰契约。
- Reviewer: Claude Code，与 Round 1/Round 2 同一评审角色，Repair Round 2 由 Codex
  完成。
- Method: 先把 R2-F1 与 R2-F2 逐条对当前内容重新证伪，再查修复是否引入新的错误
  断言或丢失被删段落的独有内容，最后核对 CEv1 的形状与门禁。CEv1 侧不接受修复
  自述：直接查 `predecessor-delivery` 上的新证据及其 `observed_at`/`satisfies`/
  `supersedes` 关系，确认是追加而非改写。`scripts/check-whitespace.sh`、
  `scripts/check-privacy.sh`、`git diff --check`、`scripts/check-topic-docs.sh`
  重新执行。Round 2 已核验的三条 R1 关闭与隐式扫描枚举，通过两份规范 blob 逐字
  未变确认，复用其证据而不重跑。
- Scope: R2-F1、R2-F2，Repair Round 2 的两处记录改动，以及本轮内容状态绑定的四个
  subject。生产代码、测试、fixture、配置全程只读。

## 📋 work-signals-contract 复评报告

📊 总体评分：9/10

✅ 判定：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **R2-F1 关闭，且合并方式正是 finding 所规定的那一种。** `tasks.md` 现在只有一段
  `Task 6 Review Round 1`（612 行），位于 Development → Review → Repair 链条内，
  与任务 0 至 5 的分组一致；`grep -n "^Task 6 "` 只返回 598/612/623/633/658 五段，
  每轮一段，重复对消失。被删那段独有的结论已并入保留段第 619-621 行：候选
  `08d63448…` 在 `cli-contract` 上被否证、不得复用。R2-F1 的有界改进写明「保留
  612 行那段并把 713 行那段独有的 CEv1 否证结论并入其中」，交付的正是这一条，
  finding 编号与 `REOPEN` 判定都没有被改动。
- **删除没有造成证据丢失。** 被删段落中除 CEv1 结论外的独有内容，是 Round 1 逐条
  核验清单（schema v20/v21 与 parser 5/6、逐列的隐私边界、`usage stats` 默认小节
  与其位置、signals 的 flag/envelope/JSON 字段名、`session show --activity` 的
  `SIGNALS` 行、`wire_version` 不变的 wire-v1 三族）。保留段以「schema/privacy、
  JSON、wire-v1、Session、captured-surface 各项断言均成立」概括同一事实，逐条明细
  仍在本记录 Round 1 的第 119 行与第 150 行——那里才是证据权威，`tasks.md` 是状态
  权威。两个落点各司其职，而不是同一事实的两份副本。
- **R2-F2 关闭，且选的是把投影对齐权威而不是反过来。** `docs/status.md` 的 Work
  Signals 行现在写 `5/6 implemented, 5/6 reviewed`，与 `tasks.md` 第 373 行
  `| 6. \`work-signals-contract\` | [ ] | [ ] |` 一致。矩阵两格在本轮 PASS 之前
  保持未勾选，符合该表既有的 REOPEN 惯例，两处都没有抢先宣称 Development 或
  Review 完成。
- **CEv1 是追加，不是改写。** `predecessor-delivery` 上的新 `pass` 绑在
  `8f870b32…`，并带两条 `supersedes` 分别指向 Round 2 的 `fail` 与前一个
  `714ed56d…` `pass`；`observed_at` 与 `satisfies` 端点齐备。Round 2 那条由
  `claude-code-rereview-r2` 写入的 `fail` 原样保留在图中，被取代而不是被抹掉——
  这正是让复用判断可审计的那条性质。另外四条 criterion 复用 `714ed56d…` 证据是
  成立的：两份规范 blob 逐字未变，本轮改动全部落在 `tasks.md` 与 status 行。
- **修复边界克制。** `git status --short` 只有那四份文档加两个无关未跟踪路径；
  生产代码、测试、依赖、配置零改动，与 Repair 记录的自述一致。

### 📝 总结

逐条处置：R2-F1 关闭，R2-F2 关闭；本轮无新增 finding。

Round 2 判 FAIL 的原因不在契约而在审计轨迹——同一轮 Review 有两段记录、投影与
权威计数矛盾。Repair Round 2 恰好只修这两处：合并重复段并保住其独有结论，把
`docs/status.md` 对齐到未勾选的矩阵，两份 CLI 规范一个字节都没动。契约部分因此
可以直接复用 Round 2 已经确认的结论，本轮只需证伪两条记录类 finding。

残余不确定性有二。其一，本仓库 CEv1 的 `digest_recipe` 是方法描述而非可直接重
哈希的原文：`8f870b32…` 与 `714ed56d…` 都无法由各自 recipe 文本的 sha256 复现，
task 5 的 recipe 更是直接写成散文。因此内容状态的绑定只能逐项核对（本轮四项全部
亲自重算命中），组合指纹本身不可独立验证。这不是本次修复引入的缺陷，是全库既有
属性，值得在版本收口时单独处理。其二，本轮 PASS 会勾选矩阵并改写状态行，从而
产生新的内容状态；证据按 `evidence.md` 的要求重绑到同步之后的最终 blob，
`8f870b32…` 上的证据被 `supersedes` 取代而非留作第二份活记录。

### Evidence

```text
git hash-object on the four bound subjects
  -> de772848 / d63017a4 / 746522b4, and the status row sha256 4841d685
     recomputed from the line itself                    <- all four recipe items
grep -n "^Task 6 " docs/topics/work-signals/tasks.md
  -> 598 / 612 / 623 / 633 / 658, one record per round  <- R2-F1 closed
grep -n "08d63448" docs/topics/work-signals/tasks.md
  -> 619 inside the single retained Review Round 1 record
docs/status.md Work Signals row vs tasks.md:373
  -> 5/6 implemented, 5/6 reviewed vs [ ] / [ ]         <- R2-F2 closed
CEv1 predecessor-delivery at 8f870b32
  -> pass, with observed_at + satisfies + two supersedes onto the Round 2 fail
     and the prior 714ed56d pass; the fail node is retained, not rewritten
CEv1 gate work-signals:work-signals-contract
  -> 5/5 pass; four criteria reuse 714ed56d because both specification blobs
     are byte-identical to the Round 2 reviewed state
bash scripts/check-whitespace.sh / check-privacy.sh; git diff --check
  -> exit 0
bash scripts/check-topic-docs.sh
  -> the two unwritten schema-version-signal documents only; no Work Signals
     finding, unchanged across every round of this task
git status --short
  -> the four documents plus the untracked review record and the unrelated
     schema-version-signal topic; no production, test, or configuration change
```

- Completion gate: VERIFIED — all five criteria re-bound to this round's final
  synchronized candidate, with the `8f870b32…` evidence superseded rather than
  left live. The Round 1 `cli-contract` fail and the Round 2
  `predecessor-delivery` fail remain immutable records of superseded states.
- Round synchronization: `tasks.md` ticks both Task 6 cells and carries the
  Re-review Round 3 note; the `docs/status.md` Work Signals row projects the
  PASS at `6/6 implemented, 6/6 reviewed`. The post-synchronization candidate is
  `5e728d29fe95ee38229689ec3173d48ad6febc037504e708f5939c56fd776744`, and unlike
  its predecessors its `digest_recipe` records the literal preimage, so the
  composite fingerprint reproduces from the recipe alone:

  ```bash
  printf '%s' 'head=9e5adde1dd45755d685beb93e14aa443a6682ef0;docs/specs/cli-design.md=de77284811c994c219cf828c1d05d8d1b729df52;docs/specs/cli-manual.md=d63017a48451e9a8682b2da326d835c31525ef36;docs/status.md#work-signals-row=d14fd7616e43be90cbd028f6d8801fd7177503c91dee3ae6da6b05af42798f9c;docs/topics/work-signals/tasks.md=ace043a5bc3232b49b35d1f5fac975157b42c9b5' | shasum -a 256
  ```

  The subject set stays the four this WorkUnit has used since Development;
  `reviews/work-signals-contract.md` is excluded, as in every prior candidate.
- Verdict: PASS

### 下一步指令

Task 6 已 PASS 且 Task 门禁 VERIFIED，建议提交并推送该任务，随后由提交后的树建立
topic 级闭合。提交与推送需要单独授权，本轮不执行。
