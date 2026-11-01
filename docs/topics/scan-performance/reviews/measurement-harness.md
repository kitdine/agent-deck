---
status: active
topic: scan-performance
subject: measurement-harness
---

# measurement-harness 评审记录

## Round 1 — 2026-10-24

### 📋 测量工具实现评审

📊 总体评分：4/10

✅ 评审结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；独立评审角色，本轮未参与实现。
- Method：工作区绑定、契约对照、CodeGraph 调用关系、源码检查及隔离合成 trace 的黑盒复现；未委派。
- Scope：Task 1 `measurement-harness`，重点覆盖刺激、运行单元、终点正确性、统计及正式批次能力。
  在获得决定性阻断复现后停止扩展验证；不是对整个 topic 或全部产品代码的重新评审。
- Reviewed state：HEAD `b7652bdf6f64a7b3110df6be01eb9bfd90baa02a`；
  candidate fingerprint `d9dd5ad18edc5d0ff71c2a5414f47714134c8d7cc80533a02f49d1ad9085abb0`。
  配方：取 `git diff --name-only HEAD` 与 `git ls-files --others --exclude-standard` 的去重路径，
  排除 `docs/`、`apps/macos/.build/`；按路径排序，每项为 `path:<git hash-object>`，已删除项为
  `path:deleted`，以 LF 连接且无末尾 LF，再取 SHA-256。共 103 路径：66 存在、37 删除。
  该身份包含交接中保留的旧 pipeline 候选，是周围候选身份，不扩大本次任务的交付范围。
- 规范依据：`tasks.md` §1；`performance-evaluation.md` 的原则、刺激、样本与统计、逐项可测性判定、P06 空闲窗口。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### MH-R1-F1 — P1：未核对 oracle，错误数据帧也被统计为成功

位置：`cmd/agentdeck-perf/stats.go:504-542`、`cmd/agentdeck-perf/run.go:710-724`。

- 行为风险：P10 只要求新帧的 tokens 不同于写入前的 tokens；没有核对本次追加后的期望值、金额与日期摘要。
  错误减少、其他 publication 的变化或先前写入迟到都可能被当成本次正确更新。
  P01 等通用区间也直接使用 `pairIntervals(..., nil)`，没有契约要求的 hero/oracle 核对。
- 证据：隔离 trace 中写前 tokens=100，写后 tokens=99，间隔 1 s；`stats <dir> P10` 输出
  `tool_invalid=0, n=1, failures=0, p95_ms=1000`。`runAppendCLI` 将 append 的 JSON 结果丢弃，
  未保留 `appendResult` 供逐样本正确性核对。源码定向搜索没有发现 oracle 的消费实现。
- 💡 有界修复：在计时窗口外建立并保留每个样本的期望 hero 摘要；P10 按写入序号关联累计期望值，
  仅匹配本次期望数据的帧结束区间，错误内容计为产品失败，oracle 运行失败计为工具无效。
  为错误下降、前次写入迟到与 token 相同但其他 hero 字段错误补充反例。
- Disposition：OPEN。

#### MH-R1-F2 — P1：正式批次仍执行固定的 pilot 编排

位置：`cmd/agentdeck-perf/run.go:49,84-91,594-703`。

- 行为风险：`--samples` 只改变外层运行单元数量，无法切换单元内部的正式数量与窗口；工具不能按契约完成正式批次。
- 证据：P03 固定每维 20 次，P06 固定 300 s，P07 固定 20 次增量，P10 固定 20 次写入。
  契约分别要求每维 100、每臂 3×1800 s、3 次全量加一个 100 次累积增量序列、一个 100 次累积写入序列。
  例如 P10 `--samples 5` 会在五次 App 启动前重置 state，不能替代同一副本内连续 100 次写入。
  P09 每个所谓 sample 实际运行 B0/S0 各两次，`--samples 20` 得到 40 对而不是规定的 20 对。
  P06 也仅等待 120 s，没有要求此时无存活 helper，或 7 min 后仍存活即产品失败的前置判断。
- 💡 有界修复：显式区分 pilot/formal 编排，将运行单元数、单元内样本数、窗口长度与累积序列独立建模；
  按契约验证实际数量、顺序和重置边界，并实现 P06 helper 起点条件。不得靠多跑 pilot 后拼接正式样本。
- Disposition：OPEN。

#### MH-R1-F3 — P1：相对门混合 P03 维度，且无法判定 P09

位置：`cmd/agentdeck-perf/gate.go:100-124,160-186`。

- 行为风险：一维明显退化可被其他维度改善掩盖；P09 没有可执行的中位 wall bootstrap 门。
- 证据：基线 client/period/panel 分别恒为 1000/1000/100 ms，候选为 100/100/200 ms，
  每维每运行单元 20 次、每臂 2 个单元。`gate --condition P03 --k 1` 输出 `pass`，
  ratio 及 95% 区间均为 0.2，尽管 panel 的比值为 2。
  `runStats` 的每维摘要不能修正 `armValues` 的混合门。
  相同合法 trace 目录调用 `gate --condition P09 --k 0.7` 退出 1：`P09 has no relative latency gate`；
  入口虽选了 q=0.5，提取器既不支持 P09，也不读取其 `cli-runs.json`。
- 💡 有界修复：P03 按维度独立取样、阈值与配对 bootstrap，所有必需维度通过才通过；
  P09 从真实 ABBA CLI 单元提取 wall 并执行对应中位数门，校验两臂数量与配对关系。
- Disposition：OPEN。

#### MH-R1-F4 — P1：重建层没有证明点击发生在全量重建期间

位置：`cmd/agentdeck-perf/run.go:575-590`。

- 行为风险：后台重建未启动、提前失败或在点击前结束时，P01-rebuild 仍可产生“有效”空闲点击结果；
  不能据此声称重建负载下的测量有效，也没有与该重建对应的 P07 资源证据。
- 证据：重建 goroutine 忽略 `MkdirTemp` 和 `command.Run` 的失败，不传入 trace 目录；
  主驱动仅等待 usable+3 s 后点击两次，没有重建起止握手或重叠检查。
  `launchApp` 只等待主 HOME 的 scan worker，未等待这个独立 state root 的 worker。
- 💡 有界修复：记录并检查重建的实际起止与退出结果，保证每个被计入重建层的点击区间与重建重叠；
  纳入资源 trace，等待对应 worker 退出后再清理 state；失败或无重叠不得生成重建层有效样本。
- Disposition：OPEN。

#### MH-R1-F5 — P2：已触发替代点击刺激，但实现与 E0 仍未完成切换

位置：`apps/macos/AgentDeckApp/PerfDriver.swift:148-175`、`cmd/agentdeck-perf/stats.go:269-271`。

- 行为风险：已知不可稳定路由的刺激继续作为正式批次入口；忽略那一个样本会违背“整批作废”的契约。
- 证据：交接 comment `26b22998-cedd-54e5-8268-9e14fa188da2` 与 CEv1
  `scan-perf-mh-impl-20261024-pilot-validity` 明确记录 P01-rebuild 1/40 未路由。
  当前 click 仍只调用 `NSApp.postEvent`，P01 统计仍从 `input.event` 开始；
  `openAction` 虽用于启动条件，但没有实现 P01 的替代模式及配对路由差值报告。
- 本轮决定：按预先声明的触发条件，确认采用 `sendAction` 替代点击刺激，P01 起点改为 `input.action`；
  必须同时报告配对 `input.action − input.event` 路由段。该决定不豁免 E0，不将旧无效批次转成有效批次。
- 💡 有界修复：在同一事件构造位置接通替代模式及统计起点，保留并报告配对路由差值；
  修复上述工具缺陷后重跑所有受工具/构建变化影响的 pilot，E0 各条件工具无效须为 0。
- Disposition：OPEN（刺激选择已决定，实现与验证待修复）。

### 🟡 建议改进 — 推荐

无额外非阻断建议。

### 🟢 优点

- Trace 解析能拒绝明确的 driver/trace invalid 与缺失 lifetime 记录。
- 配对 bootstrap 已采用固定种子与 ABBA 分组，有可复用的统计基础。
- 实现交接如实保留了 1/40 未路由和 NOT_VERIFIED，没有把 E0 声称为通过。

### 📝 总结

本轮 FAIL：测量工具会把错误数据和分维退化判为成功，且尚不能执行完整的正式测量契约。
决定性反例出现后未重跑全量 Go、native 构建或真实语料 pilot；没有读取或重新冻结真实日志。
未修改产品代码、测试或配置。

Evidence：

- `go build -mod=vendor -o <private-output>/agentdeck-perf ./cmd/agentdeck-perf` 成功。
- 黑盒复现仅用隔离合成 JSONL：v=1、clock=CLOCK_UPTIME_RAW、合法角色头与 lifetime 尾；
  F1、F3 内已记录完整关键输入、命令形式及观察结果，可独立重建，不依赖临时产物。
- 使用 provider 固定 `gate-status.cypher` 查询历史交接状态 `scan-perf-mh-impl-20261024-state`：
  NOT_VERIFIED，缺 `pilot-validity`；其余六项图内有 passing evidence。
- 历史状态给出的 fingerprint 为 `fd813b376ca18cb47e1320d50989514749ef86746569ec81e985ee82027c32f3`，
  与本轮按上述 Git blob 配方重算值不一致，且历史状态没有逐路径清单可判定差异。
  因此历史构建/测试事实保留，但不直接认证本轮候选；这是内容绑定限制，不是那些测试失败的证明。
- 本轮观察绑定新 ContentState `scan-perf-mh-review-20261024-r1-state`；
  工具反例 Evidence ID：`scan-perf-mh-review-20261024-r1-tool-failure`。
  写入并回读后以固定 gate 模板查询该状态，返回 FAILED：`pilot-validity` 的当前反证适用，
  其余六项旧状态证据不匹配本轮 target；未将历史通过结果覆写为失败。
- 本任务没有独立的完整测量契约静态 checker；现有 `stats`/`gate` 是本次被测对象，不能替代语义评审。
- 记录检查：`python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/measurement-harness.md`
  与 `git diff --check -- docs/topics/scan-performance/tasks.md` 通过。
  `make check-whitespace` 因预先存在的 `apps/macos/.build/index-build/` 生成文件的尾空白/缺末尾换行失败；
  `python3 scripts/check-docs.py` 同样在此步骤终止，未取得其后续链接/文档集检查结果。
  该生成目录不属于本轮评审记录改动，保留未清理；不将检查失败描述为通过。

下一步指令：`修复：scan-performance / reviews/measurement-harness.md / MH-R1-F1–MH-R1-F5`。
工作区：`agent-deck.scan-performance`。本记录不授权提交、推送或后续任务。

## Repair candidate — 2026-10-24（Round 1 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围为 MH-R1-F1–F5。Round 1 的评分、Verdict 与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- MH-R1-F1 -> repaired in candidate：
  - **数据帧摘要。** 数据帧的 hero 摘要扩为 client/period 代码、日期标签的 yyyymmdd、金额字段的 FNV 摘要以及原有整数合计，见 `PerfFrameProbe.swift`、`MenuBarViewModel.perfDataFrame`。
  - **Oracle 来源。** `cmd/agentdeck-perf/oracle.go` 在计时窗口外，用产品 CLI 对样本源树做一次独立全量扫描（空 state root），再读取 `desktop snapshot`，得到各 client × period 的期望 hero，写入每个运行单元的 `oracle.json`。oracle 任一步失败都记为工具无效。
  - **P01/P03/P04 核对。** 每个结束帧都与 oracle 比对，不符计为产品失败。
  - **P10 预计算。** 运行前在“今天相关源文件”的镜像上，依次预先生成并写入每次追加记录，每次写入后扫描镜像，得到第 n 次写入后的期望值。镜像先要与全量扫描一致，运行后再用全量扫描核对最后一次期望值，两处任一不一致都记为 oracle 失败。
  - **P10 回放与判定。** 运行时把同一批记录按 `seq` 回放到工作副本。第 n 次写入只以 hero 等于期望值 n 的帧结束；显示更早期望状态的帧继续等待，其他内容（含错误下降、跳过本状态）计为产品失败。
  - **反例测试。** 见 `oracle_test.go`：tokens 下降、前次写入迟到、tokens 相同但 events/sessions 错、缺 oracle、写入超出 oracle、回放与原地写入逐字节一致。Swift/Go 摘要常数由 `PerfDataFrameDigestTests` 与 `TestFNVDigestMatchesTheAppProbe` 共同钉住。
  - **P11-first 不做 oracle 核对。** 契约 P11 行要求的是 partial 首批的数据帧。E0 中首帧（今日 1.1M tokens）与完整 oracle（259M）不同，属设计行为，因此撤回对它的等值核对。
- MH-R1-F2 -> repaired in candidate：
  - **编排拆分。** 新增 `schedule{Units, Inner, Window}`，每个条件各有独立固定的 pilot 与正式日程，`run --formal` 选择正式日程；删除只改外层数量的 `--samples`。
  - **正式日程。** P01 首击 100、重复 10×20、重建中 100×2；P02/P04/P11-first 各 100；P03 1×每维 100；P06 3×1800 s；P07 3 次全量重建，最后一次之后接 100 次增量；P08 3；P09 20 对；P10 1×100（间隔 6 s）；P11-full 5。
  - **P09 成对。** 每个运行单元恰为一对 B0/S0，奇数对先 B0、偶数对先 S0。
  - **两臂交错。** `run` 写出 `schedule.json`。两臂按“对”交错，奇数对 A B、偶数对 B A；奇数个单元时末对单独成组。
  - **按日程核对。** `stats` 按日程检查运行单元数、单元内数量（每维、写入数、CLI 运行标签与数量、P09 顺序、P06 窗口长度），不符即工具无效。
  - **P06 起点条件。** 驱动在 `model.usable` 后 120 s 请求开窗；工具确认 App 无子进程、也无该 state root 的 scan worker 后才放行。`model.usable` 后 7 min 仍有 helper 存活，则记 `driver.precondition_timeout{helper_alive}` 为产品失败，不延长等待。
- MH-R1-F3 -> repaired in candidate：
  - **P03 按维度判定。** `gate` 对 client/period/panel 各自建组并独立做配对 bootstrap，任一维 fail 即 fail，全部 pass 才 pass；任一组缺某维即报错。
  - **P09 门。** 从同一批次的 `cli-runs.json` 取真实 ABBA 对的 wall 时间，执行 q=0.5 的中位门，并校验每对的标签与顺序。
  - **回归测试。** 复现评审反例（基线 1000/1000/100 ms、候选 100/100/200 ms），现在 panel 判 fail、比值 2；P09 门得到 p50 pass，乱序对被拒。
- MH-R1-F4 -> repaired in candidate：
  - **记录与握手。** 重建在独立 state root 运行，trace 写入样本目录，供 P07 RSS 使用。记录起止时间与退出结果到 `rebuild.json`。
  - **等待与清理。** `launchApp` 等待随行工作返回；重建等待自己的 worker 退出后才删除 state。
  - **判定。** `stats` 要求重建成功，且每次点击从 `input.action` 到帧（无帧时到超时）都落在重建区间内，否则工具无效。报告同时给出每个单元的重建 RSS 上界。
- MH-R1-F5 -> repaired in candidate：
  - **替代刺激。** P01 各层改用评审确认的 `sendAction` 替代刺激：在原位置构造同样的事件但不投递，起点改为 `input.action`。
  - **路由段报告。** 新增仅 pilot 的 `P01-routing`：同一进程内 20 对 event/action 点击，报告 `input.event → input.action` 路由段。该条件内的未路由点击记为 `input.unrouted`，计数报告，不作废样本。
  - **随修复暴露的问题。** 以下几处都在测试中复现过，因阻断替代刺激而一并修复：
    - **产品缺陷：** `handleClick` 对当前的非鼠标事件读取 `clickCount`，AppKit 抛出 ObjC 异常，异常穿过 Swift 栈导致崩溃，即先前在 `PerfFrameProbe` 观察者中看到的悬空执行器崩溃。改为只对鼠标抬起事件读取，真实点击行为不变。实现阶段把这些崩溃归因于启动时 `NSApp.activate`，这一判断撤回。
    - **驱动：** 结束与 `sendAction` 改为普通主队列块，不在 Task 作业内执行；退出前移除观察者。
    - **驱动：** 被忽略结果的上屏等待不再写 `driver.invalid`；action 点击只要求按钮已放置；open_action 保留“上屏”要求（启动期放宽会使 show 失效）。

修复目标：HEAD `b7652bdf6f64a7b3110df6be01eb9bfd90baa02a`；scoped candidate fingerprint
`2c5b38f2ec3a28d1b457608e99865d1a6752d1f486c361cf29084c504d61a6cc`（Round 1 配方，105 路径：68 存在、37 删除；
较 Round 1 新增 `cmd/agentdeck-perf/oracle.go`、`oracle_test.go`）。

E0（最终运行 09:39–10:15 EDT，冻结语料 `1d9ee56f`，锚定日 2026-10-24）：
- **运行状态。** 运行时源码状态为 fingerprint `91acbc607e5ed5b1ec262a964f502b07d59b3b43ff5a329f1bceca8d97630aad`，与修复目标只差 `run.go`、`stats.go` 两处 gofmt 空白，无 token 变化。
- **结果。** 14 个条件工具无效全部为 0：
  - P01-first 20/20，P01-repeat 1×20，P01-rebuild 20 单元共 40 次点击全部落在完成的重建内；
  - P01-routing 20 对，未路由 0，路由段中位 16.8 ms、p95 27.1 ms；
  - P02 20，P04 20，P11-first 20，P11-full 1；
  - P03 每维 20，后台打点 0；
  - P06 1×300 s，开窗时无 helper 存活；
  - P07 1+20，P08 每场景 2 次且计数一致，P09 3 对；
  - P10 20 次写入、失败 0（每次的金额摘要都不同，核对有实质内容）。
- **P11-first 环境无效批次。** 本次运行中第一批 P11-first（10:01–10:06）20/20 工具无效。原因：唯一带菜单栏的屏幕（S27C900P，主屏）上，状态栏按钮窗口停在 y=1890，即该屏上沿，处于收起状态，App 请求前台被拒，与菜单栏被隐藏一致。该批保留为无效批次，未改代码、未换构建，于 10:20 单独重跑该条件批次，20/20 有效。
- **工具外观察（不是门）：** P10 中位 5.66 s；P09 S0/B0 中位比 0.859；P04 中位 878 ms。

验证：
- **测试与构建。**
  - `go vet ./...`、`scripts/run-go-test.sh ./...` 通过；
  - agentdeck-perf/perftrace/store/ingest/scanruntime/usage 的 race 测试通过；
  - `bash scripts/test-macos-app.sh` 第二次运行 TEST SUCCEEDED、0 失败。第一次运行 `MenuBarChromeTests.testInitialPopoverPreparationDoesNotShowAWindowOrMoveFocus` 的窗口数断言失败一次，重跑未复现，根因未查明，如实保留；
  - Release 通用构建（x86_64 arm64）通过，Release AgentDeck/AgentDeckShared 的 `PerfDriver` 符号为 0、Debug 为 0、Perf 为 381；
  - gofmt 无差异；改动范围内 `git diff --check` 通过；
  - 已跟踪文件中旧 harness 名称只命中 `.agent-instructions/beads.md` 里的 Beads 任务 ID。
- **CEv1。**
  - E0 运行状态为 `scan-perf-mh-repair-r1-20261024-e0-state`，证据 `scan-perf-mh-repair-r1-20261024-e0-pilot-validity` 记为 pass。
  - 修复目标状态为 `scan-perf-mh-repair-r1-20261024-state`，其余六项证据 `scan-perf-mh-repair-r1-20261024-{clock-source, marker-coverage, driver-isolation, fixture-determinism, measurement-privacy, legacy-harness-removal}` 记为 pass。
  - pilot-validity 以 `scan-perf-mh-repair-r1-20261024-pilot-validity` 通过 `rolls_up` 引用 E0 证据，评估为 preserves（仅 gofmt 空白）。
  - 以固定 gate 模板查询目标状态，结果为 VERIFIED。这是证据状态，不代替复评结论。
- **未决。** `make check-whitespace` 仍会因未跟踪的 `apps/macos/.build/index-build/` 生成文件失败，不属于本次修复，未清理。

需复评确认的取舍：
- **单单元两臂。** P03/P10 的正式日程按契约每臂只有 1 个运行单元，配对 bootstrap 只有一组，区间退化为点估计。这是现有契约的结构问题，本次未改设计，请复评判断是否需要设计变更。
- **P11-first 的正确性核对。** 只检查终点帧到达，不比对完整 oracle，依据是 P11 行“partial 标记明示”的首批定义。帧记录目前不含 partial 标记本身。

## Round 2 — 2026-10-24

### 📋 MH-R1-F1–F5 修复复评

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；未参与本次修复，未委派。
- Method：修复交接与当前源码对照、定向 Go 回归、隔离合成 trace 黑盒反例。
- Scope：MH-R1-F1–F5 的闭环及修复直接影响的测量/统计路径；不修改产品、测试或配置。
- Reviewed state：HEAD `b7652bdf6f64a7b3110df6be01eb9bfd90baa02a`；Round 1 同一配方重算
  `79c6ef9a8df77e56a68b367bc7b759e0589f496efc0c15824f8f92d1e9c71e89`，105 路径。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### MH-R1-F1 — P1，仍未完全关闭：P11-first 的正确性核对被整体跳过

位置：`cmd/agentdeck-perf/run.go:742-745`、`cmd/agentdeck-perf/stats.go:371-386`。

- Disposition：STILL OPEN。原 P10 错误下降、迟到状态及 P01 hero 比对问题已修复；
  “每个样本在终点后核对正确性”的闭环仍缺 P11 首批。
- 行为风险：任意首帧都可能被当作今天的正确 partial 首批，无法发现错日期、错 client、错误总量，
  也不能确认契约要求的 partial 标记。partial 与全量 oracle 不相等，只能说明需要首批口径的核对，不能取消核对。
- 证据：`P11-first` 未启用 oracle；构造合法 app trace，`app.ready` 后 1 s 出现
  `frame.vsync{tokens:-999,client:3,period:2,day:19000101}`，无 oracle、无 partial 标记；
  `stats <dir> P11-first` 输出 `tool_invalid=0,n=1,failures=0,p95_ms=1000`。
  修复交接第 214 行也明确承认帧记录不含 partial 标记。
- 💡 有界修复：记录首批实际覆盖范围及渲染的 partial/日期/选择状态，以该覆盖范围的期望值在计时外核对；
  不要求 partial 总量等于完整历史总量。错误内容为产品失败，无法建立正确性 oracle 为工具无效。
  补充错日期、缺 partial 与部分结果错误的反例。

#### MH-R1-F2 — P1，仍未完全关闭：pilot/formal 边界和 ABBA 统计契约未受门禁约束

位置：`cmd/agentdeck-perf/run.go:51-60,216-260`、`cmd/agentdeck-perf/gate.go:112-149,228-264`。

- Disposition：STILL OPEN。固定 pilot 数量的问题已修复，正式日程、P06 helper 前置条件已落地；
  但 `gate` 不检查 schedule 的 Mode、不与该条件的正式日程核对，并把不足 ABBA 的末对当作完整重抽组。
- 行为风险：工具 pilot 可直接生成正式通过结论；P03/P10 各臂只有一个运行单元时，
  仅 A B 的两次运行被声明为 bootstrap 的一组，所有重抽值相同，不能声称满足契约规定的四单元 ABBA 重抽。
- 证据：两臂各一单元，`schedule.json={mode:pilot,units:1,inner:20}`；每维 20 次，
  基线恒 100 ms、候选恒 50 ms，合法 oracle。`gate --condition P03 --k 1` 退出 0，
  三维及总结果均 PASS，每维 groups=1、ratio=0.5、区间 [0.5,0.5]。
  `readSchedule` 只反序列化；`abbaGroups` 只读取候选臂的数量。正式单单元模式同样会走此分支。
  契约“工具 pilot 不产生门结果”和“每个重抽组 A、B、B、A 四个运行单元”没有被修复取舍替代。
- 💡 有界修复：门禁拒绝 pilot、两臂不一致及非批准正式日程；未满足四单元 ABBA 的结构不得输出 PASS。
  P03/P10 单运行单元与 ABBA 的契约冲突必须先取得明确设计决定，再实现被批准的统计方案；
  本轮不批准把两单元/单组点值默认为有效置信区间，也不授权自行增加样本。

### 🟡 建议改进 — 推荐

无额外非阻断建议。

### 🟢 优点与已关闭项

- MH-R1-F3 -> CLOSED：P03 按维度独立 bootstrap，原 panel 翻倍反例被拒；P09 已从 CLI 成对记录取 wall，
  校验标签/顺序并用中位门。`TestP03GateJudgesEachDimension`、`TestP09GateUsesThePairedCLIRuns` 通过。
  上述 pilot/ABBA 残余归 F2，不将混维缺陷重复保留为 F3。
- MH-R1-F4 -> CLOSED：重建传入 trace，保存实际进程区间及退出状态；所有点击必须落在成功区间内，
  随行 goroutine 完成并等待独立 state 的 worker 后清理；统计报告重建 RSS。
  `TestRebuildLayerRequiresClicksInsideACompletedRebuild` 验证成功、提前结束、退出失败三种情况。
- MH-R1-F5 -> CLOSED（刺激实现与报告）：P01 改为构造事件后 sendAction，统计从 input.action 起算；
  单独 routing pilot 配对报告被排除路由段。`TestRoutingPairsReportTheExcludedSegment` 通过。
  交接记录了 20 对未路由 0、路由中位 16.8 ms/p95 27.1 ms；该历史测量的当前状态适用性另见下方内容绑定限制，
  不因关闭刺激实现问题而宣称当前 E0 已通过。
- F1 的 P10 oracle 及 F2 的显式 formal 日程取得实质进展；回归覆盖包含错误下降、旧状态迟到、字段不符和 helper 不退出。

### 📝 总结

逐项处置：F1、F2 仍有上述必需修复；F3、F4、F5 的原缺陷关闭。未另增 finding ID。
本轮 FAIL，不以历史 VERIFIED 覆盖当前反例；没有重新运行冻结语料或全量构建。

Evidence：

- `scripts/run-go-test.sh ./cmd/agentdeck-perf -run 'Test(FrameHero|WriteToFrame|.*Gate|RebuildLayer|RoutingPairs|FormalSchedules|IdleWindowHelper)'`
  通过：10 项测试，包耗时 2.272 s。源码发生实质修复，本次定向验证不重复未受影响的全量套件。
- `go build -mod=vendor -o <private-output>/agentdeck-perf ./cmd/agentdeck-perf` 成功。
- 两个隔离合成 trace 反例的输入、命令与结果已写在 F1/F2，可独立重建，不依赖临时产物。
- 内容绑定限制：交接给出 `2c5b38f2ec3a28d1b457608e99865d1a6752d1f486c361cf29084c504d61a6cc`，
  本轮同配方重算为 `79c6ef9a8df77e56a68b367bc7b759e0589f496efc0c15824f8f92d1e9c71e89`。
  `run.go` 的 Git blob `318ce0253f1cf0c95b712c8653cf96bb5b7ef096` 与 `stats.go` 的
  `1340c7c0b75ccb3bb07d93d11b27627cd30a6c0f` 符合交接中两条局部身份，但不足以解释总指纹差异。
  修复需提供完整逐路径身份或明确配方差异，作范围化复用评估；无需仅为换轮次重跑所有测试。
- 本轮 ContentState：`scan-perf-mh-rereview-r2-20261024-state`；反例 Evidence：
  `scan-perf-mh-rereview-r2-20261024-tool-failure`。旧状态的测试与 pilot 记录保持原样。
  固定 provider gate 模板确认修复交接状态为 VERIFIED；本轮新状态写入并回读后为 FAILED，
  当前 `pilot-validity` 反例适用，其余历史通过证据不匹配本轮 target。
- 修复交接中的 XCTest 一次窗口数断言失败、重跑通过但根因未明，仍为历史不确定性，
  本轮没有把重跑描述为根因已解决，也不据此重复执行整套 native 测试。
- 本轮评审记录结构校验及范围内 `git diff --check` 通过；任务矩阵 Review 保持未勾选。
  全仓文档/空白检查仍沿用前轮已定位的 `.build/index-build/` 生成文件阻断，不重复未变化的失败检查。

下一步指令：`修复：scan-performance / reviews/measurement-harness.md / MH-R1-F1、MH-R1-F2`。
工作区：`agent-deck.scan-performance`。修复包含上述证据身份核对；不授权 Git 交付。

## Repair candidate — 2026-10-25（Round 2 后）

Repairer：Claude Code 主会话（Round 1 修复者）；范围为 MH-R1-F1、MH-R1-F2 及 Round 2 要求的证据身份核对。
Round 2 的评分、Verdict 与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

设计决定（用户，2026-10-24，回应 Round 2 F2 的契约冲突）：P03/P10 正式日程拆成 4 个运行单元。
P03 每臂 4 单元 × 每维 25，P10 每臂 4 条写入序列 × 25、每条从重新生成的工作副本开始；总样本量不变。
`performance-evaluation.md` 已同步：P03 行“每维 100，共 300（4 个运行单元 × 每维 25）”，
P10 行“100 次（4 条写入序列 × 25）”，重新生成次数“P07 1 次、P10 每臂 4 次”，并写明 pilot 与不完整 ABBA 不产生门结果。

- MH-R1-F1 -> repaired in candidate：
  - **首批记录。** 数据帧增加 `partial`（App 实际渲染的 `.partial` 限定词）、`committed`、`sources`、
    `coverage_complete`（serving header 的 usage coverage），见 `PerfFrameProbe.swift`、`MenuBarViewModel.perfDataFrame`。
  - **首批 oracle。** `computeBootstrapOracle`（`cmd/agentdeck-perf/oracle.go:210`）在计时外，用产品 CLI 对样本源树、
    空 state root 运行 `desktop bootstrap --format ndjson`，按发布顺序记录每次 publication 的覆盖范围、是否 complete、
    应显示的 partial 标记及各 client × period hero。发布顺序是确定的（今天相关源优先，每批 64）。
    日期不是锚定日、没有发布、最后一次发布不是 complete 或与全量扫描 hero 不一致，均为 oracle 失败，样本工具无效。
    同一计数会先以 partial、再以 complete 各发布一次，因此 coverage 身份包含 complete 状态。
  - **P11-first 核对。** `P11-first` 启用 oracle 与 bootstrap（`run.go:771`）。`sampleIntervals`（`stats.go:376-402`）
    按帧的 (committed, sources, coverage_complete) 找到对应发布，要求帧的 hero（含日期、client、period、合计、金额摘要）
    等于该发布对应选择的 hero，且 partial 标记等于该发布应显示的标记；不符或覆盖范围不存在为产品失败，缺首批 oracle 为工具无效。
  - **反例。** `TestFirstBatchFrameIsCheckedAgainstItsCoverage`：合法 partial 首批与合法 complete 帧通过；
    错日期、缺 partial 标记、partial 合计错误、未知覆盖范围记为产品失败；无 bootstrap oracle 记为工具无效。
    Round 2 的 `tokens:-999,client:3,period:2,day:19000101` 无标记首帧落入上述失败类。
- MH-R1-F2 -> repaired in candidate：
  - **门禁只认批准的正式日程。** `gateSchedule`（`gate.go:115`）拒绝 Mode 不是 formal 的批次（pilot 不产生门结果）、
    任一臂日程不等于该条件批准的正式日程，以及每臂单元数不足 2 或为奇数的日程；`abbaGroups` 读取两臂日程，
    每组恰为 2g-1、2g 两个单元（即 A B B A 四个运行单元），单元数与日程不符即报错。P09 同样经 `gateSchedule`。
  - **正式日程。** P03 `{Units: 4, Inner: 25}`、P10 `{Units: 4, Inner: 25}`（`run.go:754`、`run.go:867`）；
    `runP10` 每条写入序列前恢复干净工作副本，结束后标记为脏。其余已门控条件的正式单元数均为偶数，
    只报告不门控的 P11-full 除外。
  - **反例。** `TestGateAdmitsOnlyTheApprovedFormalSchedule`：Round 2 的 pilot 单单元批次、两臂不一致、
    非批准日程、奇数单元均被拒；并断言所有门控条件的正式日程单元数为偶数。
    `TestFormalSchedulesFollowTheContract` 及 P03（4×25）、P09（20 对、10 组）门测试已同步。
- **环境缺口（工具，随修复暴露）。** 本轮第一次 E0（2026-10-24）因 11:48:29 起锁屏而作废：
  锁屏遮挡全部窗口，`frame.blocked` 记录 occluded=1，P11-first 20/20 无效、P10 20/20 被误判为产品失败、P11-full 无效。
  这是测量环境缺口，不是产品行为。`sampleEnvironment` 现在记录 `screen_locked`（读取 `ioreg -n Root -d1` 的
  `CGSSessionScreenIsLocked`），锁屏即工具无效；运行单元结束时仍锁屏同样作废。`TestScreenLockedReadsTheConsoleSession` 覆盖。
  该作废批次未被引用为任何证据。

证据身份核对（Round 2 要求）：
- **指纹差异是配方偏差，不是内容差异。** Round 1 修复交接的 `2c5b38f2…` 由修复者脚本算出，该脚本有两处偏离 Round 1 配方：
  路径列表用 `git ls-files -m -d` 而非 `git diff --name-only HEAD`，且排序依赖 locale（未设 `LC_ALL=C`）。
  修正脚本后，对同一内容重算得到 `79c6ef9a…`，与 Round 2 复评值一致。Round 1 修复交接中的源码内容本身不受影响，
  但 `2c5b38f2…` 不应再作为内容身份引用。
- **本次修复目标。** HEAD `b7652bdf6f64a7b3110df6be01eb9bfd90baa02a`；Round 1 配方（路径取
  `git diff --name-only HEAD` 与未跟踪文件，排除 `docs/` 与 `apps/macos/.build/`，`LC_ALL=C` 字节序，
  每行 `path:<git blob>` 或 `path:deleted`，LF 连接、无末尾 LF，SHA-256）得到
  `ebde492c516501634abcfc3c1e039169201e0a748ac644e918fcadefbbe7c5ce`，105 路径（68 存在、37 删除）。
  E0 与 XCTest 前后各算一次，均为此值。逐路径清单：

<details>
<summary>105 路径的 Git blob 清单</summary>

```text
apps/macos/AgentDeck.xcodeproj/project.pbxproj:e9de1b6853f6b6c31be83fa594acba25ac3addd8
apps/macos/AgentDeckApp/AgentDeckApp.swift:03f47947269c013facaf8a59e41b556fba29b5c1
apps/macos/AgentDeckApp/MenuBarItemController.swift:396f4e63f31e0401fb4b5286b8696455d0d82341
apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:cecb3274909986eda0066ace1f7ed28f144d84ef
apps/macos/AgentDeckApp/MenuBarViewModel.swift:683219651333106e50e05df77c1ebb1cb8f7cb88
apps/macos/AgentDeckApp/PerfDriver.swift:1fbdcb0671343f82ef32966b0b51697b3b7c49b4
apps/macos/AgentDeckApp/PerfFrameProbe.swift:3fb6e44be2a87bb739ac8d2c427d0201ec906c52
apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift:889c11f934c2d6cee60ee052d3a35655f50d5619
apps/macos/AgentDeckAppTests/PerfClockTests.swift:af7285537c814844344e1630628b4073b91b412e
apps/macos/AgentDeckShared/DesktopLog.swift:6fc200f755098d2f2470f31db6a96b2d0e71ead7
apps/macos/AgentDeckShared/DesktopRefreshScheduler.swift:46822c07c9cc3bd53c0c098edb0da05e01f8b982
apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:0561377dc194545273ae0ef9a98bef2279816cc7
apps/macos/AgentDeckShared/PerfTrace.swift:a5c07a93ae42968aa162cea58f549c227341486f
apps/macos/AgentDeckTests/DesktopLogPolicyTests.swift:611300ae6ce535f1d6ccac6dbb923883cbfda26b
apps/macos/AgentDeckTests/PerfTraceTests.swift:34eb50a75fc72008ea84a88a8b80c13b811514b7
cmd/agentdeck-perf/append.go:9f4f6ab63d550c22b9340b25778b783359219d24
cmd/agentdeck-perf/append_test.go:dd69655d5f595fc69229d377cc558acae0150871
cmd/agentdeck-perf/batch.go:f2d9ca01db32bb4d915317ed79aa1bbafcba03cb
cmd/agentdeck-perf/batch_test.go:5be240c99b532ec8bbbcc5472d4dd4ce57972dbf
cmd/agentdeck-perf/content.go:eb9c129ecf328b6c2f8a03c642d563cde53b1bd2
cmd/agentdeck-perf/corpus.go:ec0fe6131fe99d74b206e5b0097279837330c078
cmd/agentdeck-perf/fixture.go:179817af5d985de886188193bf202e32ecbcdfa8
cmd/agentdeck-perf/fixture_test.go:7d6723b8ca8493c896d38238b876cb8144895d57
cmd/agentdeck-perf/gate.go:92b08053574f91a14581ac9f944c035bcf1db8d5
cmd/agentdeck-perf/gate_test.go:3cdc2e5bab3c096d9e08c59c679eb6328ead774e
cmd/agentdeck-perf/main.go:c18648ba56d8d6bdbe48bef933f5a2c739d1f6ff
cmd/agentdeck-perf/oracle.go:17b6b91926ac4de8eb2f6276af5d1efbafee6c76
cmd/agentdeck-perf/oracle_test.go:fc716ef66292e4f93f3f031f2ef317a6faf66485
cmd/agentdeck-perf/run.go:3aebe3081cd3a0a6e7e2139a40c64a81f9e6bc60
cmd/agentdeck-perf/shift.go:8841876ed419ccc69255be47cb9bf0d34a8e8a6f
cmd/agentdeck-perf/shift_test.go:9fbb55274527c487da2c73d904e7b7caf8c526a1
cmd/agentdeck-perf/sizes.go:be71010bdbfb680cd6b9bc5ad4c33848a02647d8
cmd/agentdeck-perf/sizes_test.go:8b0061f0b38b5a39a715161498fd687523337b3c
cmd/agentdeck-perf/stats.go:292ebe2119d3f246fb557c886ddff891e514c2cf
cmd/agentdeck-perf/stats_test.go:c436741b91d7837783c7a2bb4417d4cad8fc8e18
cmd/agentdeck-perf/template.go:7e42d51a194389efe263c73f5e338bd828743804
cmd/agentdeck/main.go:6c0da06c6981f0f49e3a044ec07a96264b308c65
cmd/agentdeck/perf_signal_test.go:5ef363ed72d0dce92acff64874e34d9b40245589
cmd/agentdeck/scan_runtime_test.go:45c9961a186a04973033a7f12fd9968d16a63c8b
internal/activity/classify.go:d170b764fb870e557a34386a4c72e82b3ad17f0e
internal/activity/scan_message_equivalence_test.go:e0f3983e2470b0ef6b31541898b0a51e891920cc
internal/ingest/ingest.go:12020ee72c3dff864700cb33d5aec30e9132b91b
internal/ingest/ingest_test.go:7561b443eaf6f6430a1c2fb8f9fe3ce6819248e8
internal/ingest/metrics.go:e613e331398e654f35007ec474903f286ec8fa86
internal/ingest/parallel_decode.go:961fb510378562384876f4532d58b6fb1bbad848
internal/ingest/parallel_decode_test.go:5585290f5da912833c083ee67418b93e4f45f072
internal/perftrace/clock_darwin.go:8963ef4e945334e061651773878b0d99ec7081b2
internal/perftrace/clock_other.go:d34c974edada061578370d97b0e77ca3c7ecf0e2
internal/perftrace/owner_other.go:2fbada86e503c9717d194acaa067e702189acebb
internal/perftrace/owner_unix.go:0a7191ea02e4af22703c52e351461ac12b5d1fb4
internal/perftrace/perftrace.go:40733c73a8c41edda7bd59a873a8f4866a6fcf3c
internal/perftrace/perftrace_test.go:adaaadb83e17445eaf15a66f6f3867645fe06d82
internal/scanruntime/profile_test.go:699438a86b965660247633784a23751ee7f219c6
internal/scanruntime/scanruntime.go:6226cfd1b71e0aaebea07fe2be9a4ee000877104
internal/scanruntime/serving.go:fb32d4a308958bd5fc90e6f05787686887aa0a61
internal/session/session.go:163f119ee5ac5c6b4e1aea3f0fbef5f11526e7bc
internal/store/perf_tx.go:5b8141ddda7a01cc7091b51ae3e74b8b3d7b4faa
internal/store/perf_tx_test.go:a8f665557aaab42bfcf8941ccd36ab588ba9ad5b
internal/store/session_source_index_test.go:1d3202eba1635074e0e97d22892b87fc489028b9
internal/store/store.go:13f0d53e35bf2101b1d36975e13c45c6e15a2d2a
internal/store/writable_pragmas_test.go:dbc349e63d2c48470107f5573c7be20fc140b7ce
internal/usage/orphan_cleanup_test.go:ebadedbcffae86ead3412b34072e2388bd659aac
internal/usage/presentation.go:19d9bb96f444170511727a550638d4a4a3480c50
internal/usage/profile.go:deleted
internal/usage/profile_test.go:deleted
internal/usage/projected_events.go:0120130dc5d6212db82952ad73c39576417646a9
internal/usage/projected_events_test.go:1c5a20f6b064372efa744c40b9e63fb415d5eb9c
internal/usage/session_rebuild_test.go:5aa1be36faf6ea30352d8c99cdb2eb235c3fe44a
internal/usage/usage.go:1775649ee958d3971903ad1e1fe8cead2d7b0251
scripts/build-macos-app.sh:97b1e3aac059883f72755680a5221899ec894627
scripts/p07-overlay/main.go:deleted
scripts/p07-overlay/main_test.go:deleted
scripts/p07_rss_support.py:deleted
scripts/p07_rss_support_test.py:deleted
scripts/p07measure/README.md:deleted
scripts/p07measure/attribution_test.go:deleted
scripts/p07measure/autocommit.go:deleted
scripts/p07measure/autocommit_test.go:deleted
scripts/p07measure/child_darwin.go:deleted
scripts/p07measure/child_other.go:deleted
scripts/p07measure/children.go:deleted
scripts/p07measure/children_test.go:deleted
scripts/p07measure/cte.go:deleted
scripts/p07measure/cte_test.go:deleted
scripts/p07measure/partition_regression_test.go.tmpl:deleted
scripts/p07measure/process.go:deleted
scripts/p07measure/session_cte_regression_test.go.tmpl:deleted
scripts/p07measure/transactions.go:deleted
scripts/p07measure/transactions_test.go:deleted
scripts/scan-performance-baseline.py:deleted
scripts/scan-performance-incremental-engine.go.tmpl:deleted
scripts/scan-performance-incremental.py:deleted
scripts/scan-performance-serving.py:deleted
scripts/scan-performance-serving_test.py:deleted
scripts/scan-performance-update-bounded-state.py:deleted
scripts/scan-performance-update-bounded.py:deleted
scripts/scan-performance-update-synthetic.py:deleted
scripts/scan_performance_baseline_test.py:deleted
scripts/scan_performance_p07.py:deleted
scripts/scan_performance_p07_test.py:deleted
scripts/scan_performance_serving_matrix.py:deleted
scripts/scan_performance_serving_matrix_test.py:deleted
scripts/scan_performance_switches.py:deleted
scripts/scan_performance_switches_test.py:deleted
scripts/scan_performance_update_bounded_test.py:deleted
```

</details>

E0（2026-10-25 01:30–02:04 EDT，屏幕解锁，冻结语料，锚定日 2026-10-25，源码即上述修复目标，工具 pilot 日程）：
- **结果。** 14 个条件工具无效全部为 0、产品失败全部为 0：
  - P01-first 20，P01-repeat 1×20，P01-routing 20 对（未路由 0），P01-rebuild 20 单元共 40 次点击；
  - P02 20，P03 每维 20、后台打点 0，P04 20；
  - P06 1×300 s（开窗时无 helper 存活），P07 1 单元，P08 2 单元，P09 3 对；
  - P10 20 次写入，P11-full 1。
- **P11-first 首批核对生效。** 抽查的首个样本 oracle 含 36 次 publication；全部 20 个样本的首个数据帧都是
  committed 64 / sources 2232、partial=1、coverage_complete=0、日期 20261025，均与该覆盖范围的期望 hero 一致。
- **工具外观察（不是门）：** P11-first 中位 1239 ms；P10 中位 5.84 s；P09 S0/B0 中位比 0.847；P04 中位 864 ms。
- E0 是工具 pilot，按 F2 修复本身不产生门结果；正式 ABBA 批次不在本修复范围。

验证：
- **Go。** `go vet ./...`、`scripts/run-go-test.sh ./...` 通过；agentdeck-perf 与 perftrace 的 race 测试通过；gofmt 无差异。
- **macOS。** `bash scripts/test-macos-app.sh` 第一次运行时 `MenuBarChromeTests.testInitialPopoverPreparationDoesNotShowAWindowOrMoveFocus`
  失败 3 处：可见窗口集合多出 2 个、新增窗口 3 个而非 1 个、准备窗口可见；原样重跑 TEST SUCCEEDED（152 项、1 跳过、0 失败），
  Widget 套件 51 项 0 失败。该测试与 Round 1 修复交接记录的偶发失败相同；它比较测试宿主进程内全部 `NSApp.windows`，
  本修复未改动该测试或菜单栏窗口逻辑。根因未查明，如实保留，未另立 finding，也未在本修复中处理。
- **构建。** Release 通用构建通过，Release 与 Debug 的 `PerfDriver` 符号为 0、Perf 为 381。
- **旧 harness。** 已跟踪文件（除 `docs/`、`prototype/`）中旧 harness 名称只命中 `.agent-instructions/beads.md` 里的 Beads 任务 ID。
- **文档检查。** 本记录 `check-review-records.py`、`check-topic-docs.sh scan-performance`、范围内 `git diff --check` 通过。
- **CEv1。** 修复目标状态 `scan-perf-mh-repair-r2-20261025-state`（属性含上述逐路径清单）。七项证据
  `scan-perf-mh-repair-r2-20261025-{pilot-validity, clock-source, marker-coverage, driver-isolation, fixture-determinism, measurement-privacy, legacy-harness-removal}`
  记为 pass，均直接观测该状态；E0 即在该状态上运行，无 rolls_up。以固定 gate 模板查询为 VERIFIED。这是证据状态，不代替复评结论。
- **未决。** `make check-whitespace` 仍会因未跟踪的 `apps/macos/.build/index-build/` 生成文件失败，不属于本次修复，未清理。

需复评确认：
- 首批核对以产品 CLI 的 bootstrap 发布为 oracle；它与 App 的发布共用批次规划，独立性来自空 state root 与计时外运行，
  并以最后一次发布等于独立全量扫描作交叉校验。
- 正式 ABBA 批次尚未运行；E0 只证明工具在 pilot 日程下有效。

## Round 3 — 2026-10-25

### 📋 MH-R1-F1、MH-R1-F2 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer：OpenCode / GPT-6 Astra；未参与修复，未委派。
- Method：源码与批准契约对照、逐路径身份核对、独立定向回归、CEv1 固定 provider gate 查询。
- Scope：F1/F2 剩余缺陷及修复影响的首批 oracle、帧标记、正式日程和 ABBA 分组。
- Reviewed state：HEAD `b7652bdf6f64a7b3110df6be01eb9bfd90baa02a`；105 路径指纹
  `ebde492c516501634abcfc3c1e039169201e0a748ac644e918fcadefbbe7c5ce`。
  本轮独立重算的全部路径与上方 manifest 完全一致，旧指纹差异已解释。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。F1/F2 均关闭，未发现新增阻断项。

### 🟡 建议改进 — 推荐

无新增建议。

### 🟢 优点与已关闭项

- **MH-R1-F1 -> CLOSED。** `oracle.go:210-274` 在独立空 state、计时窗口外采集 bootstrap 发布，
  按 committed/sources/complete 保存首批期望，最后发布与独立全量扫描一致才接受 oracle。
  `MenuBarViewModel.swift:357-375`、`PerfFrameProbe.swift:60-64` 记录实际选择、hero、覆盖范围及 partial 状态；
  `stats.go:371-403` 按覆盖范围选择期望，核对日期、总量与 partial，缺 bootstrap oracle 记工具无效。
  定向回归的七个样本中：有效 partial 与 complete 两个成功；错日期、缺 partial、错总量、未知覆盖范围四个产品失败；
  缺 bootstrap oracle 一个工具无效。不是把 partial 强行与全量总值相等比较。
- **MH-R1-F2 -> CLOSED。** `gate.go:115-181` 同时检查两臂的正式日程、目录数量和完整 ABBA 结构；
  拒绝 pilot、不一致与未批准日程，不再把末尾单 AB 对作为完整重抽组。
  P03/P10 按修复记录所载 2026-10-24 用户决定及当前性能契约采用每臂 4×25，总样本数不变，形成两个完整 ABBA 组。
  回归确认旧单单元 pilot 反例被拒，P03 正式 4×25 可进入分维门，P09 成对门保持通过。
- **MH-R1-F3/F4/F5 -> CLOSED（沿用 Round 2）。** 本轮范围未改变其原缺陷处置；分维与 P09 受日程修复影响，
  已补跑对应回归；重建重叠和替代刺激的已关闭结论继续有效。

### 📝 总结

全部既有 finding 已关闭。工具任务达到复评通过；此结论不表示后续正式性能目标已经达成。

Evidence：

- 本轮执行 `scripts/run-go-test.sh ./cmd/agentdeck-perf -run 'Test(FirstBatchFrame|GateAdmits|FormalSchedules|P03Gate|P09Gate)'`，
  五项测试全部通过，包耗时 1.681 s；断言内容经源码核对。
- 本轮 `check-review-records.py`、`check-topic-docs.sh scan-performance` 及评审/任务矩阵范围内
  `git diff --check` 均通过，任务矩阵 Review 已勾选。
- CEv1 WorkUnit `scan-performance:measurement-harness`，精确 target
  `scan-perf-mh-repair-r2-20261025-state`。固定 provider gate 回读七项 required criteria，
  与 tasks.md 一致，全部有效通过；missing/invalidated/unresolved 均为空，reason 为 null。
  复用 `scan-perf-mh-repair-r2-20261025-` 前缀的七项证据，不重标旧观察或新建重复状态。
- E0、全量 Go、race、构建及 native 测试复用上述精确状态证据；本轮没有重跑 E0 或正式批次。
  bootstrap oracle 与 App 共用产品批次规划，不是独立重实现业务逻辑；独立 state 与最终全量交叉校验
  足以覆盖本 finding 的首帧内容核对范围，不宣称证明扫描算法自身无缺陷。
- XCTest 窗口测试原样重跑通过但根因未明，保留修复交接中的不确定性；本轮不声称已修复该历史波动。
  全仓空白检查仍有已记录的 `.build/index-build/` 生成文件阻断，不改变或清理这些文件。

### Task 检查点

- Task：`measurement-harness`；内容身份如上；Review PASS，精确任务门 VERIFIED。
  所属 topic 仍有 `baseline-remeasure` 等七个任务，不跨越 Topic 完成边界。
- 提交建议：在获得授权后，将本任务实现、测试、必要契约、评审记录和任务矩阵作为逻辑提交；
  对混合工作树按 hunk 区分，排除旧任务的独立优化和生成产物，提交前确认贡献者归属。
- 推送建议：任务提交完成并核验后，再按明确授权推送 `feature/scan-performance`；本轮未提交或推送。
- 下一任务：`开发：scan-performance / baseline-remeasure`；工作区 `agent-deck.scan-performance`。

## PR #61 修复交接 — 2026-11-01

来源：[数值 UTC 偏移平移失败](https://github.com/kitdine/agent-deck/pull/61#discussion_r4432027295)，
评审目标 `c37829d14fde0b81e8b6ac3894ed11137d95e0e4`。用户授权修复、分笔提交推送，
并在当前提交全部 Actions 成功后再请求 Codex review。

修复前，合法 `+00:00` 被 `Z07:00` 格式化为 `Z`，长度检查失败，内容保留旧日期，
而目录与 mtime 已平移。新回归用合成工作副本确定复现该不一致；`Z` 与 `+08:00` 对照正常。
数值偏移分支现使用 `-07:00`，保留显式 UTC 偏移与小数位数。

`TestShiftCorpusPreservesNumericUTCOffsets` 核对有/无小数的 `+00:00`、`Z` 与非零偏移，
完整文件字节、字段长度、平移目录与 mtime 一致，4 条记录全部重写、unparsed 为 0。
该回归连续 3 轮 race 通过；测量工具整包 race 与 `make vet` 亦通过（Go 1.27.3）。

本轮只使用临时合成语料，不重新冻结或读取真实日志。历史测量是否涉及该形状，仍需按原
冻结身份及平移报告核对；现有评审记录未提供可直接判断该范围的 unparsed 聚合，不能
把本轮工具修复当作历史批次重新验证。旧 PASS 与精确状态证据保留原绑定；新候选待独立评审。
