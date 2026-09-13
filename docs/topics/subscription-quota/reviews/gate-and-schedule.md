---
status: active
topic: subscription-quota
subject: gate-and-schedule
---

# Gate And Schedule Review

## Round 1 — 2026-09-12

## 📋 gate-and-schedule 实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 18 个未提交路径，
manifest sha256 `dde5d80454fc612e1be251973044fb2eb220c62cda74739ebb4819bc55129b43`。
本轮所有结论均由 `go test -overlay` 注入的只读复现测试得出，工作区未被修改。

### 🔴 严重问题 — 必须修复

**GS-R1-F1 — 高：切换到 `official` 之后，一条早于该切换的 Hook 观测会无限期压制探测。**

位置：`internal/desktop/desktop.go:336-343`（`quotaObservedOfficial`）、
`internal/usage/routes.go:403-415`（`LatestObservedProvider`）、`internal/quota/gate.go:36-38`。

`LatestObservedProvider` 取该 client 最近一条非空 `observed_provider`，只按 `observed_at DESC`
排序，不限定它是否晚于当前 recorded selection。`provider.CurrentSelection` 带 `SelectedAt`
（`internal/provider/service.go:103`），`usage_session_observations` 带 `observed_at`，两个时间戳
都在库里，但 `quotaObservedOfficial` 从不比较它们。

复现（`REPRO F1`）：先写入一条 `SessionStart` 投递，观测 provider 为 `relay`；随后记录一次
严格更晚的 `official` 选择；再以 reading 打开分别跑一次 background 和一次 manual 刷新。

```
REPRO F1 recorded selection = "official" (selected_at 2026-09-12T09:08:49.918153Z);
         latest observed provider = "relay" (known=true), recorded BEFORE the switch
REPRO F1 -> codex probes after one background AND one manual refresh = 0; envelope written = false
REPRO F1 control (observation agrees) -> codex probes = 1
```

对照组把观测改成一致，同一路径探测次数为 1，说明"分歧"本身就是唯一原因，与探测接线无关。

后果：用户在 AgentDeck 内切到 `official` 后，只要该 client 还没有新的 `SessionStart` 投递到达，
网关就持续按 `not_official` 压制，background 与 manual 都无效，功能静默失效且不给理由。

C1 选择 recorded 而非 observed 的理由，原文是"Using the observed value would make the gate
silently stop working for users without Hook integration"。当前实现把同一种静默失效原样搬给了
"有 Hook 集成、但最近一次投递早于本次切换"的用户——恰恰是设计明确想避免的那类故障。

修复方向（择一，需与 operator 确认）：交叉校验只采纳不早于当前选择 `SelectedAt` 的观测，其余
情形按 `observedKnown=false` 处理；或修订 C1 明确写出该时间边界并留痕。

### 🟡 改进建议 — 建议处理

**GS-R1-F2 — 中：手动刷新无法越过退避，最长可被锁死一小时。**

位置：`internal/quota/scheduler.go:132-143`（`due`，退避检查在 `TriggerManual` 判断之前）。

C9 表格对 user-initiated refresh 的原话是"The interval does not gate a refresh the user asked
for; the reading switch and the provider gate still do."——枚举了仍然设卡的两项，未包含退避。

复现（`REPRO F2`）：连续失败使退避按 5m→10m→20m→40m→1h 增长后，在整个窗口内发起 5 次手动刷新。

```
REPRO F2 failure 5 at 2026-09-10T11:15:00Z -> backoff window = 1h0m0s (until 2026-09-10T12:15:00Z)
REPRO F2 -> manual refreshes refused: 5 of 5 attempts spread across the 1h0m0s backoff window;
         subprocess invocations unchanged at 5
REPRO F2 -> a manual refresh one instant after the window runs: invocations = 6 (was 5)
```

窗口结束后立即放行，确证退避是唯一原因。这是对留白契约的单方面裁决：用户遇到一次瞬时失败后，
明确要求重试也无法执行，且界面上没有任何可操作的解释。该裁决未进入 tasks.md 已记录的四项
operator 决策，属于未留痕的收窄。

修复方向：让 manual 越过退避（`single-flight` 仍保留），或把该裁决写入 C9 与 tasks.md 并获批。

**GS-R1-F3 — 中：single-flight 既不"加入"运行中的探测，在生产进程模型下也不生效。**

位置：`internal/quota/scheduler.go:93`（包级 `sync.Map`）、`:116-119`（发现已在飞行则直接返回）。

C9 的原话是"A refresh arriving while a probe is running joins the running probe rather than
starting a second."

复现（`REPRO F3`）：让 fake 探测阻塞 1 秒，在其飞行途中发起第二次刷新。

```
REPRO F3 first (in-flight) probe took 1.022s; the refresh that arrived during it returned after 0s
REPRO F3 -> at the moment the second caller returned, an envelope existed = false;
         after the first probe finished = true
```

第二个调用者 0 秒返回、拿到的是探测前状态——既没有启动第二次探测（符合契约前半句），也没有
"加入"（违反后半句），其快照会渲染旧值。

更关键的是适用范围：该守卫只在进程内有效，而 `scheduler.go:84-87` 自己写明每次
`agentdeck desktop snapshot` 都是独立进程；manual 又不受 interval 约束，正是重复触发最可能
发生的路径。同时唯一的生产调用点 `RefreshQuota`（`internal/desktop/desktop.go:309-315`）顺序
遍历两个 client，进程内根本不存在并发，因此这个守卫目前在生产中从未生效过，只有测试会命中。

修复方向：使用跨进程锁，或明确承认降级、把"进程内尽力而为"写入 C9 并获批。

**GS-R1-F4 — 中：tasks.md 同时要求并推迟了"关闭读取时反注册状态栏"。**

位置：`docs/topics/subscription-quota/tasks.md:257-260`（范围条目，仍然要求该行为）、
`:270-274`（验收行，仍然要求"the unregister-on-off transition against a temporary settings file
including its restore-incomplete path"的 L2 覆盖）、`:298-303`（决策 3，把它推迟到任务 6）。

核实：`internal/quota` 与 `internal/desktop` 均不引用 `usagehook`（仅 `desktop.go:292` 的一句
注释提及），全仓没有任何针对该转换的测试。

后果：任务 4 的 Dev 勾选建立在一项其自身验收行仍然要求、却零覆盖的行为之上。本轮 CEv1 的
verification 判据逐字取自该验收行，因此该判据无法判定为满足——这不是评审额外加码，而是文档
自身尚未同步。

修复方向：把范围条目与验收行一并改为推迟到任务 6，并在任务 6 的范围与验收行补齐；或在本任务
内实现该转换。两者都需要 operator 确认，因为它改变已声明的任务边界。

**GS-R1-F5 — 低：网关算出的压制原因在唯一生产调用点被丢弃。**

位置：`internal/desktop/desktop.go:313`，`allowed, _ := quota.Allowed(...)`。

`Reason` 被丢弃，调度器在被压制时也不写任何信封，`quota_envelopes` 中没有压制原因。后果是
下游（任务 6）若按 C1 自行读 `provider.Service.Current`，会看到 `official`，无法重建"观测分歧"
这一压制路径，用户只会看到陈旧数据而没有理由。与 F1 同源，建议一并处理。

### 🟢 做得好的方面

- `gate.Allowed` 是纯函数、四个布尔入参，条件顺序与 C1 完全一致；7 个用例覆盖了优先级
  （reading off 压过 not official）、无选择等同非官方、以及分歧压制。
- 退避不引入计数列，而是用 `BackoffUntil.Sub(FailureAt)` 反推上一步长度
  （`scheduler.go:196-213`），并在成功时与 `Failure`/`FailureAt` 同一次写入清零，
  `store_test.go:698-738` 对往返与清零都有直接断言。
- `PutEnvelopeFailure` 扩参后，仓内全部 9 个调用点均已同步，无遗留旧签名。
- C0 静态断言独立于既有的仓库级网络导入测试，符合 C0"enforced in three places rather than
  trusted once"的要求。
- schema 24→25 的波及面处理干净：fixtures 仅 `count` 一个字段变化，已逐一核对；其余引用一律走
  `store.CurrentSchemaVersion` 常量，无硬编码版本号残留。
- 四项越界决策全部在 tasks.md 与 Beads 留痕（F4 的不一致是同步遗漏，不是未记录）。

### 📝 总结

网关本身（C1 的纯判定部分）与退避的持久化设计是这一轮里最扎实的部分，实现干净、测试到位。
问题集中在网关的**输入**与调度的**边界**上。

F1 是必须修复项：它让"切换到 official"这一最常见的入口在相当一段时间内静默失效，而这正是 C1
在选择 recorded selection 时明文想要避免的失败模式。F2、F3 都是在契约留白处做出的单方面收窄，
方向未必错，但都改变了用户可感知的行为且未留痕。F4 是文档自身的自相矛盾，它同时阻塞了
verification 判据的判定。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务
无关的预存漂移（任务 3 评审已记录在案）。既有测试全部通过这一事实本身不构成反证：上述五项
都不在现有断言的覆盖范围内。

Swift 侧沿用本项目既定姿态：本机只有 Command Line Tools，`swiftc -parse` 语法通过，
`XCTest` 无法执行，`DesktopPreferencesTests.swift` 的实际运行属于待真实 Xcode 的人工验收。
本轮未把该限制计入评分。

## Round 2 — 2026-09-12

## 📋 gate-and-schedule 修复复评

📊 总体评分：8/10

✅ 评审结论：FAIL

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 19 个未提交路径，
manifest sha256 `2bb421a0a3466b0756a416c4f81c9164efe2cd37640dd0412bde422b8369eee4`。
相对 Round 1，变更集恰为 8 个文件：`architecture.md`、`tasks.md`、
`internal/desktop/{desktop.go,quota_test.go}`、`internal/quota/{scheduler.go,scheduler_test.go}`、
`internal/usage/{routes.go,routes_test.go}`。`gate.go`、`model.go`、`store.go`、`migrations.go`、
fixtures 与两个 Swift 文件的 blob 与 Round 1 完全一致，未被顺手改动。

本轮结论同样全部来自 `go test -overlay` 注入的只读复现测试；交接说明仅作为输入，未作为证据。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**GS-R2-F1 — 低；新增：手动重试会把后台退避推高，用户点刷新反而让自动探测变慢。**

位置：`internal/quota/scheduler.go` 的 `due`（手动放行）与 `recordFailure`/`nextBackoff`
（失败仍推进几何退避）之间的交互。

GS-R1-F2 的修复让 `TriggerManual` 越过退避——这是对的，也已写入 C9。但手动尝试失败后仍然走
`recordFailure`，照常把退避翻倍。于是手动刷新被豁免为退避的**消费者**，却仍然是退避的
**生产者**，而这后半个问题在本轮的决策记录里没有任何交代。

复现（`REPRO R2/F-new`）：interval 5m，maxBackoff 1h。

```
after one background failure -> backoff = 5m0s, background next allowed at 2026-09-10T10:05:00Z
manual retry 1 at +30s  -> backoff now 10m0s, background next allowed at 2026-09-10T10:10:30Z
manual retry 2 at +60s  -> backoff now 20m0s, background next allowed at 2026-09-10T10:21:30Z
manual retry 3 at +90s  -> backoff now 40m0s, background next allowed at 2026-09-10T10:43:00Z
manual retry 4 at +120s -> backoff now 1h0m0s, background next allowed at 2026-09-10T11:05:00Z
-> background probing was deferred from 10:05:00Z to 11:05:00Z purely because the user pressed
   refresh 4 times; delta = 1h0m0s
-> a background refresh at the ORIGINAL deadline 10:05:00Z: ran = false
```

两分钟内的四次点击，把原本 5 分钟后就会发生的后台探测推迟到一小时后——等于把约一小时的退避
增长压缩进两分钟。

影响有界，这也是本项只记为"低"的原因：手动刷新本身永远不受退避限制，用户始终有可用的逃生出口；
一次成功会清零退避。最坏情况是服务端恢复后、用户不再手动刷新时，后台多等最长 `MaxBackoff`。

但它与 GS-R1-F2 属于同一类问题：operator 刚刚就"手动与退避的关系"做出裁决，而这一裁决的镜像
另一半（手动失败是否应计入退避链）被留白且未留痕。Round 1 正是以这个理由判 F2 成立，本轮不宜
换一把尺子。

修复方向（择一，需 operator 确认）：`recordFailure` 对 `TriggerManual` 不推进退避链，只记录
失败；或保持现状，但在 C9 与 tasks.md 明确写下"任何触发源的失败都推进退避"，如同 F3 的处置。

### 🟢 做得好的方面

- **GS-R1-F1 已闭合，且边界正确。** 复现 Round 1 的完全相同场景：陈旧观测 `relay`
  （09:00）早于 `official` 选择（10:00），background 与 manual 分别探测 1 次和 2 次
  （Round 1 均为 0），reason 为空。两个对照都对：观测**晚于**选择且分歧时仍压制并报
  `not_official`；`observedAt == selectedAt` 的边界按"不陈旧"处理，`Before` 的严格比较是对的。
  修复位置也选得准——`LatestObservedProvider` 只多返回观测时刻、不对"陈旧"表态，判定留在
  `quotaObservedOfficial`，职责没有下沉到查询层。
- **GS-R1-F2 已闭合。** Round 1 的同一场景里 5 次手动刷新从 0 次执行变为 5 次全部执行，且
  C9 表格对应行已同步改为"Neither the interval nor backoff gates"。
- **GS-R1-F3 的处置诚实。** 没有假装修好：`scheduler.go` 的注释与 C9 的 Concurrency 段落都
  明写这是进程内尽力而为、不是 C9 原文承诺的"joins the running probe"，并写明当前唯一生产
  调用点顺序遍历、该守卫在生产中根本不触发。承认缺口比粉饰更有价值。
- **GS-R1-F4 已闭合，且是"迁移"而非"删除"。** 任务 4 的范围条目与验收行都改成显式推迟，任务 6
  同时收到了对应的范围条目**和** L2 验收行，要求没有在两个任务之间蒸发。
- **GS-R1-F5 已闭合。** `RefreshQuota` 返回 `map[quota.Client]quota.Reason`，探针实测被压制的
  client 得到 `not_official`、放行的得到空值。
- 修复范围克制：8 个文件全部落在五项发现的修复面内，未夹带无关改动。

### 📝 总结

这是一轮干净的修复。五项发现全部真实闭合，其中 F1 这个高危项的修复不仅正确，边界条件和对照
场景也都站得住；F3、F4 选择了如实记录而不是粉饰，符合本项目对"决策必须留痕"的一贯要求。

唯一拦住 PASS 的是 GS-R2-F1：修复 F2 时只回答了"手动是否受退避约束"，没有回答"手动失败是否
推进退避"。后者同样改变用户可感知的行为，同样没有留痕。改动很小——要么一行判断，要么一句
决策记录。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务
无关的预存漂移；`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 均 exit 0
（architecture.md 改动后重跑确认）。既有测试全绿不构成对 GS-R2-F1 的反证：该交互不在任何现有
断言的覆盖范围内。

## Round 3 — 2026-09-12

## 📋 gate-and-schedule 修复复评

📊 总体评分：7/10

✅ 评审结论：FAIL

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 19 个未提交路径，
manifest sha256 `117d51bcde5e560763c7fd7499018ee686ee02b80c577beab1545dde346ddab4`。
相对 Round 2，变更集恰为 4 个文件：`architecture.md`、`tasks.md`、
`internal/quota/{scheduler.go,scheduler_test.go}`，与声明的修复面一致，其余 blob 逐字未动。
本轮结论同样全部来自 `go test -overlay` 注入的只读复现测试。

### 🔴 严重问题 — 必须修复

**GS-R3-F1 — 中；新增：手动失败会破坏后台退避链的推导，重复手动重试可把几何退避完全抵消。**

位置：`internal/quota/scheduler.go` 的 `recordFailure`（手动分支保留 `rec.BackoffUntil`，但
仍照常写入新的 `failureAt`）与 `nextBackoff`（用 `BackoffUntil.Sub(FailureAt)` 反推上一步长度）。

根因是 `FailureAt` 被复用为两种语义：既是"最近一次失败发生的时刻"，又是"当前退避步长的锚点"。
tasks.md 决策 2 明文选择了"由 `BackoffUntil` 与 `FailureAt` 共同反推，而不引入计数列"。本轮修复
只保留了 `BackoffUntil`、却让 `FailureAt` 继续前移，于是这个差值不再等于它本应度量的步长。

复现（interval 5m，MaxBackoff 1h）：

```
REPRO R3/A second background failure's backoff step:
    without a manual retry in between = 10m0s; with one = 6m0s

REPRO R3/B background failure 1 -> step = 5m0s
REPRO R3/B background failure 2 -> step = 10m0s
REPRO R3/B background failure 3 -> step = 20m0s
REPRO R3/B after one manual retry -> FailureAt moved to 10:36:00Z,
    BackoffUntil left at 10:35:00Z (now in the past); their difference = -1m0s
REPRO R3/B next background failure -> step = 5m0s
    (streak before the manual retry was 20m0s; doubling would give 40m0s)

REPRO R3/C cycle 1..5 -> background step was 5m0s, 5m0s, 5m0s, 5m0s, 5m0s
    (without any manual retry it would be 5m, 10m, 20m, 40m, 1h)
```

三段证据是同一根因的三个程度：夹在两次后台失败之间的一次手动重试把下一步从 10m 缩到 6m；
窗口之后的一次手动重试让差值变为负数，`nextBackoff` 的 `prev <= 0` 分支直接回退到 base，链条
重置为 5m；每轮一次手动重试则让后台步长永远停在 5m。

后果与 GS-R2-F1 方向相反、性质相同：一个持续失败的端点，只要用户还在点刷新，后台就会一直以
最小间隔反复探测，退避这项保护被完全抵消——而退避存在的理由正是避免对失败端点的高频重试。

本轮新增的 `TestSchedulerManualFailureDoesNotAdvanceBackgroundBackoff` 只断言"手动失败之后
`BackoffUntil` 未变"，从不驱动**下一次后台失败**，而破坏恰恰在那时才显现，因此它在链条已损的
情况下依然通过。这也是全仓 21 包全绿不构成反证的原因。

修复方向（需 operator 确认，因为它触及已记录的决策 2）：让手动失败只更新 `Failure`/`Reason`
而不动 `FailureAt`；或为退避链引入独立的锚点/计数字段，不再让 `FailureAt` 兼任两职——后者
等于重新审视决策 2"不引入计数列"的取舍。

### 🟡 改进建议 — 建议处理

**GS-R3-F2 — 低；新增：`recordFailure` 的文档注释残留了已被本轮推翻的旧描述。**

位置：`internal/quota/scheduler.go:192-193`。

```go
// recordFailure persists a failed probe attempt, advancing C9's geometric
// backoff from whatever the client's envelope already recorded.
// recordFailure persists a failed probe attempt. A manual failure records
// the failure itself but leaves backoff exactly as it already was, ...
```

旧句原样留在新句正上方，两句连续以 `recordFailure persists a failed probe attempt` 开头，且
前一句所断言的"advancing ... backoff"正是本轮明确取消的行为。注释因此同时声称手动失败会推进
退避、又声称不会。删掉前两行即可。

### 🟢 做得好的方面

- **GS-R2-F1 已闭合。** 手动失败不再推高 `BackoffUntil`：R3/B 中三次后台失败建立的 20m 窗口，
  在一次手动重试后终点仍停在原处；R3/C 五轮手动重试也从未把后台步长推大。Round 2 记录的
  "四次点击把后台探测从 5 分钟后推到一小时后"已不复现。
- **契约与决策同步到位。** C9 的 Backoff 段落新增了"只有后台失败推进退避链，手动既不受其约束
  也不向其贡献"，tasks.md 的退避范围条目同步改写，并新增了"Round 2 repair"决策记录段落，与
  Round 1 的记录并列。留痕这一项本轮做得完整。
- **修复范围克制。** 4 个文件全部落在这一项发现的修复面内，`gate.go`、`desktop.go`、
  `routes.go`、store/migrations、fixtures 与 Swift 文件的 blob 与 Round 2 逐字相同。
- `recordFailure` 通过新增 `trigger` 形参、并在 `runCodex`/`runClaudeProse`/`Run` 三处调用点
  逐层透传来获取触发源，没有借助包级变量或隐式状态，方向是对的。

### 📝 总结

本轮把 GS-R2-F1 真正修好了，契约与决策记录也补得完整。但修复选择了"保留 `BackoffUntil`、
照常前移 `FailureAt`"这条路径，而这两个字段在决策 2 里是被当作一对来反推退避步长的——只动其中
一个，推导就失真。结果是退避从"被手动重试推得过长"变成了"被手动重试压得过短"，在持续失败的
场景下等于没有退避。

这不是在提高标准：Round 2 判 GS-R2-F1 成立的理由是"手动与退避的关系被单方面改变且未经完整考虑"，
本轮的问题是同一处关系的另一个失真方向，且这次有明确的行为证据而不只是留白。

GS-R3-F2 是同一次改动留下的注释残留，删两行即可，单独不足以拦住 PASS。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务
无关的预存漂移；`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 均 exit 0。
工作区 20 个条目、HEAD 未变、无探针文件泄漏。

## Round 4 — 2026-09-13

## 📋 gate-and-schedule 修复复评

📊 总体评分：8/10

✅ 评审结论：FAIL

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 19 个未提交路径，
manifest sha256 `0406eef9898a6b569a4bc9675301a677f43dbd12308605311a103805c8bec203`。
相对 Round 3，变更集恰为 3 个文件：`tasks.md`、`internal/quota/{scheduler.go,scheduler_test.go}`。
`architecture.md` 本轮未动——其 C9 Backoff 文本（"Only a background failure advances this
chain"）本就正确，漂移只发生在实现侧，这个判断是对的。

### 🔴 严重问题 — 必须修复

**GS-R4-F1 — 中；新增：没有在先的后台失败时，手动失败把零值当作真实失败时刻写入信封。**

位置：`internal/quota/scheduler.go` 的 `recordFailure` 手动分支（`failureAt = rec.FailureAt`）
与 `internal/quota/store.go:381-388` 的 `PutEnvelopeFailure`。

本轮修复让手动失败复用 `rec.FailureAt`，使锚点对只随后台失败移动——这个方向是对的。但当不存在
可继承的值时（客户端从未探测过，或上一次探测成功、`PutEnvelope` 已把 `Failure`/`FailureAt`/
`BackoffUntil` 一并清零），`rec.FailureAt` 就是零值；而 `PutEnvelopeFailure` 对 `failureAt` 是
**无条件** `Format`，零值被写成 `0001-01-01T00:00:00Z` 并作为真实时刻读回。

复现：

```
REPRO R4/D case 1 (never probed, one manual failure at 2026-09-10T10:00:00Z)
    -> stored ok=true Failure="probe_failed" FailureAt=0001-01-01T00:00:00Z BackoffUntil=0001-01-01T00:00:00Z

REPRO R4/D case 2 (success at 2026-09-10T10:00:00Z, then a manual failure at 11:00:00Z)
    -> Failure="probe_failed" FailureAt=0001-01-01T00:00:00Z ObservedAt=2026-09-10T10:00:00Z
       (last good figure retained=true)
REPRO R4/D -> the age a surface would render for that failure is now - FailureAt
              = 2562047h47m16.854775807s
```

案例 2 是普通操作序列：探测成功过一次，用户随后手动刷新且这次失败。信封于是同时带着一个真实的
`Failure` 与一个公元 1 年的 `FailureAt`，二者自相矛盾；换算出的时长是 `time.Duration` 饱和溢出
的约 292 年。

同一个文件已经有正确写法可循：`PutEnvelope`（`store.go:332-338`）对 `FailureAt` 与
`BackoffUntil` **两个字段都**做了 `IsZero` 守卫，只有 `PutEnvelopeFailure` 漏了 `failureAt`
（它对 `backoffUntil` 的守卫就写在同一条语句里）。读取侧 `Envelope`（`store.go:433`）本就用
`if failureAtText != ""` 容忍空值。所以修复是补上那一处守卫，读写两端都无需其他改动。

影响范围要说清楚，不夸大：`ObservedAt` 与最近一次好数据保留正确，C9 要求展示的"真实年龄"取自
`ObservedAt` 而非 `FailureAt`；`nextBackoff` 在 `BackoffUntil` 为零时走 `IsZero` 分支返回 base，
也不受影响。因此**当前没有任何界面会读错**——消费方是尚未实现的任务 5–7。这是一处静默的持久化
数据错误，而写入它的正是本任务。

修复方向：`PutEnvelopeFailure` 对 `failureAt` 补 `IsZero` 守卫，与同函数内 `backoffUntil` 的写法
一致。注意 `store.go` 属任务 1 的文件，但任务 4 的决策 2 已有先例touch 过它，若采纳需一并留痕。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **GS-R3-F1 已闭合，三个场景逐一复验。** 用 Round 3 的原始探针重跑：夹在两次后台失败之间的
  手动重试，下一步为 **10m0s**（Round 3 为 6m0s），与无手动重试的基线完全一致；20m 连败后窗口外
  一次手动重试，字段对差值仍为 **20m0s**、下一次后台失败为 **40m0s**（Round 3 重置为 5m）；
  每轮一次手动重试，五轮步长为 **[5m 10m 20m 40m 1h]**（Round 3 恒为 5m）。
- **修复方向选得准。** 把"两个字段必须同进同退"作为不变式，而不是给退避链另加锚点或计数列——
  既解决了问题，也没有推翻决策 2 的取舍，这一点在 tasks.md 的 Round 3 repair 记录里也写明了。
- **注释这次写对了。** `recordFailure` 的文档注释不仅删除了残留旧句（GS-R3-F2 闭合），还在
  手动分支内就地说明了"为什么 `FailureAt` 也必须不动"，把不变式写在最容易被误改的位置。
- **新增测试这次盯住了正确的位置。** 两个新测试都驱动到**下一次后台失败**并断言其推导出的步长，
  正是 Round 3 指出的旧测试盲区；`…StillDoubleCorrectly` 直接对齐了 REPRO R3/C 的五轮序列。
- **范围克制且判断准确。** 3 个文件全部落在这一处修复内，并主动判断 `architecture.md` 无需改动，
  理由成立。

### 📝 总结

这一轮把 GS-R3-F1 真正修好了，而且修在正确的抽象层次上——建立"锚点对同进同退"的不变式，并把
理由写在代码里。新增测试也补上了上一轮的盲区。

剩下的 GS-R4-F1 是同一处改动的边界遗漏：修复只考虑了"有值可继承"的路径，没考虑"无值可继承"时
复用零值意味着什么，而写入层恰好对这个字段缺了同函数内其他字段都有的守卫。它当前不影响任何
界面，但会把一条自相矛盾的记录持久化下来，留给任务 5–7 去踩。补一处 `IsZero` 守卫即可。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务
无关的预存漂移；`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 均 exit 0。
工作区 20 个条目、HEAD 未变、无探针文件泄漏。既有测试全绿不构成对 GS-R4-F1 的反证：没有任何
断言覆盖"无在先失败时手动失败写入了什么时刻"。

## Round 5 — 2026-09-13

## 📋 gate-and-schedule 修复复评

📊 总体评分：9/10

✅ 评审结论：FAIL

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 19 个未提交路径，
manifest sha256 `64ae7086d93bca628a56de3b76b2e282ef507468d8cc8a07a285f4068d59f5ca`。
相对 Round 4，变更集为 4 个文件：`tasks.md`、`internal/quota/{store.go,store_test.go,scheduler_test.go}`；
`scheduler.go` 与 `architecture.md` 的 blob 未变，与修复说明一致。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**GS-R5-F1 — 低；新增：为 GS-R4-F1 新增的两个回归测试不保护这次修复。**

位置：`internal/quota/store_test.go` 的 `TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent`，
`internal/quota/scheduler_test.go` 的 `TestSchedulerManualFailureWithNoPriorFailureStoresNoFailureInstant`。

两个测试都经 `Store.Envelope()` 断言 `FailureAt.IsZero()`。但修复前写入的文本
`0001-01-01T00:00:00Z` 按同一布局解析回来本就是零值 `time.Time`，所以这一断言在修复前同样成立。

实证：用 overlay 把 `store.go` 替换为仅去掉该 `IsZero` 守卫的副本（其余逐字相同），同时运行原始列
探针作为对照：

```
[simulated pre-fix] REPRO R5/case1 never probed + manual failure -> raw failure_at="0001-01-01T00:00:00Z"
[simulated pre-fix] REPRO R5/case2 success then manual failure  -> raw failure_at="0001-01-01T00:00:00Z"
[simulated pre-fix] REPRO R5/pre-fix text parsed back IsZero=true
--- PASS: TestSchedulerManualFailureWithNoPriorFailureStoresNoFailureInstant (both subtests)
--- PASS: TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent
```

对照证明模拟确实复原了旧的原始列文本，而两个测试仍然通过——守卫被回退时它们不会报警。

修复方向：在同包测试里直接读原始列（例如 `SELECT failure_at FROM quota_envelopes WHERE client=?`）
并断言其为空字符串；调度器测试可复用同一断言。

### 🟢 做得好的方面

- **GS-R4-F1 已闭合，在存储层实测确认。** 两个案例（从未探测后手动失败、成功后手动失败）的原始列
  `failure_at` 均为 `""`、`backoff_until` 为 `""`；对照组后台失败照常写入
  `2026-09-10T10:00:00Z` / `10:05:00Z`，守卫没有误伤真实失败。
- 修复与同函数内 `backoffUntil`、以及 `PutEnvelope` 对两字段的既有写法完全一致，读取侧无需改动；
  注释写明了为何零值必须存为空。
- 范围克制并如实留痕：`store.go` 属任务 1 文件，tasks.md 新增 Round 4 repair 段落记录了这一点。
- Round 4 的三个退避场景复跑无回归：10m/10m、20m→40m、[5m 10m 20m 40m 1h]。

### 📝 总结

**评审方更正。** Round 4 记录中 GS-R4-F1 的影响描述有误：我当时写"作为真实时刻读回"、并以
约 292 年的时长佐证，但本轮实测修复前的文本解析回来 `IsZero=true`——`Envelope()` 返回的本就是零值，
那条时长对任何未检查零值的零 `FailureAt` 都成立，不能区分修复前后。缺陷真实存在，但只在持久化的
原始列文本及其与 `PutEnvelope` 约定的不一致上（原样导出数据的读者可见），按实际影响应为"低"而非"中"。
这一更正不改变 Round 4 的判定，因为该处不一致确实需要修。

GS-R5-F1 正是这一更正的直接推论：修复方按我 Round 4 的表述去写测试，于是断言落在了修复前后都成立的
Go 层零值上。评审表述的偏差是成因之一，这里如实记录。但交付的测试确实不保护修复，按本评审一贯尺度
仍记为发现。修改只需把断言换成原始列文本。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处无关预存漂移；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 均 exit 0。工作区 20 个条目、HEAD 未变、
无探针或模拟文件泄漏（修复前模拟仅经 overlay 注入）。

## Round 6 — 2026-09-13

## 📋 gate-and-schedule 修复复评

📊 总体评分：9/10

✅ 评审结论：PASS

被评审内容状态：`head=3e944523068abd36cb16319a28d169052477dbc6`，任务 4 的 19 个未提交路径，
manifest sha256 `0a168f461458ab54e6bfd4b169fee5a85db44890df36e4cd864cf713e6dd85e3`
（沿用 Round 5 配方：`head=` 行加按路径排序的 `<git hash-object> <path>` 行，本记录自身除外；
该值已包含本轮 tasks.md 的状态同步）。本轮修复声明的变更面为 `internal/quota/{store_test.go,scheduler_test.go}`
与 `tasks.md`，无生产代码改动；`store.go` 中 `PutEnvelopeFailure` 的 `failureAt` 守卫仍在。

评审方：claude-code（主会话，默认模型层级）。方法：逐条对照前五轮全部发现复核当前内容，
并用 `go test -overlay` 做独立负向对照；工作区未被修改。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **GS-R5-F1 已闭合，负向对照独立复现。** 本轮自行构造 overlay：把 `store.go` 替换为仅将
  `failureAt` 的 `IsZero` 守卫改回无条件 `Format` 的副本（`diff` 确认只有这 4 行→1 行差异）。
  在该副本下三处断言全部失败：

  ```
  --- FAIL: TestSchedulerManualFailureWithNoPriorFailureStoresNoFailureInstant/never_probed
      scheduler_test.go:267: stored failure_at="0001-01-01T00:00:00Z" backoff_until="", want both empty
  --- FAIL: TestSchedulerManualFailureWithNoPriorFailureStoresNoFailureInstant/after_a_success
      scheduler_test.go:287: stored failure_at="0001-01-01T00:00:00Z" backoff_until="", want both empty
  --- FAIL: TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent
      store_test.go:741: stored failure_at="0001-01-01T00:00:00Z" backoff_until="", want both empty
  ```

  对真实文件同一组测试全部 PASS。守卫一旦回退，测试会报警——这正是 Round 5 要求的保护。
- **断言落在正确的层。** `rawEnvelopeInstants` 直接 `SELECT failure_at, backoff_until` 读原始列文本，
  注释写明了为何经 `Envelope()` 读取无法区分 `""` 与 `0001-01-01T00:00:00Z`，把 Round 5 的盲区
  原因留在了最容易再犯的位置。
- **"成功后手动失败"子测试保留了业务断言。** 除原始列为空外，仍断言 `ObservedAt` 保留最近一次
  成功时刻，没有为了换断言层而丢掉原有保护。
- **范围克制。** 本轮只动测试与 tasks.md 修复记录，未夹带生产代码。

前五轮发现逐条复核（均针对当前内容重新确认，而非沿用旧结论）：

- GS-R1-F1 closed：`quotaObservedOfficial` 仍按 `observedAt.Before(selectedAt)` 丢弃早于选择的观测。
- GS-R1-F2 closed：`TriggerManual` 越过 interval 与退避，C9 表格一致。
- GS-R1-F3 closed：进程内 single-flight 的降级在 `scheduler.go` 注释与 C9 Concurrency 段落如实记录。
- GS-R1-F4 closed：任务 4 的范围与 Verification 行明确推迟 unregister-on-off，任务 6 承接范围与 L2 验收。
- GS-R1-F5 closed：`RefreshQuota` 返回每个 client 的压制原因。
- GS-R2-F1 closed：手动失败不推进退避链。
- GS-R3-F1 closed：手动失败不移动 `FailureAt`，退避步长推导保持 5m/10m/20m/40m/1h。
- GS-R3-F2 closed：`recordFailure` 注释无残留旧句。
- GS-R4-F1 closed：`PutEnvelopeFailure` 对零值 `failureAt` 存空文本。
- GS-R5-F1 closed：见上。

### 📝 总结

十项发现全部闭合，无遗留、无新发现。Round 5 唯一的阻塞项是测试不保护修复；本轮修复把断言移到
原始列文本，且独立负向对照证明守卫回退时三处断言都会失败。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，
退出码 0；`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处无关预存漂移；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在 tasks.md 状态同步后重跑均 exit 0。
工作区 20 个条目、HEAD 未变，overlay 副本只在会话 scratchpad 中。

残余不确定性：Round 5 内容状态只记录了整体 digest，未存逐文件 blob，因此"本轮只改了声明的 3 个文件"
无法逐字对比，依据是修复交接说明与本轮对相关代码的直接复核。Swift 侧沿用既定姿态：本机无完整 Xcode，
`DesktopPreferencesTests.swift` 的 XCTest 实际执行仍属待真实 Xcode 的人工验收，不计入评分。

完成门禁：VERIFIED（CEv1 WorkUnit `subscription-quota:gate-and-schedule`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:gate-and-schedule:r6:0a168f461458`）。
