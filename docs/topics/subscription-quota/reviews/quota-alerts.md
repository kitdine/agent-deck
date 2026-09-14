---
status: active
topic: subscription-quota
subject: quota-alerts
---

# Quota Alerts Review

## Round 1 — 2026-09-13

## 📋 quota-alerts 实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

被评审内容状态：`head=0d5bf9a1eee0f7de7ad12d4a97d388dfd320ef12`，任务 5 的 11 个未提交路径，
manifest sha256 `90019db68acc910f393088e5bb8ed5cb203ed6890fce62f14f3983339fe4a2b5`
（`head=` 行加按路径排序的 `<git hash-object> <path>` 行，本记录自身除外；该值已包含本轮 tasks.md
的状态同步，同步前实现交付态为 `b33aeb21…`，差异只在 tasks.md）。

评审方：claude-code（主会话，默认模型层级；未参与本任务实现）。方法：对照 tasks.md 任务 5、
architecture.md C9/C10（含本任务新增的三条 operator 决策）逐行审读 `alerts.go`、
`notifier_darwin.go`、两份测试、migration 26 及其波及面；关键判断全部由 `go test -overlay`
注入的只读复现测试得出，复现文件只在会话 scratchpad，工作区未被修改。

### 🔴 严重问题 — 必须修复

**QA-R1-F1 — 高：评估器不判断存储的窗口 occurrence 是否已经结束，陈旧数据会发出过期提醒，超过
31 天后每次评估都重复提醒。**

位置：`internal/quota/alerts.go:111-128`（阈值与 reset 两个分支都不看 `now` 与 `ResetsAt`）、
`internal/quota/alerts.go:100` 与 `:224-227`（按 `instance_unix < now-31d` 清理台账）。

根因是一处：评估器把"库里最新的窗口"当作"当前 occurrence"，但探测失败时 C9 保留最后一次好数据，
客户端不再使用时也不会有新观测，此时存储的 `resets_at` 早已过去。两个症状：

1. **重复提醒（违反"alerts that do not repeat"）。** 台账按 `instance_unix`（即该陈旧的
   `resets_at`）清理。一旦它早于 `now - 31d`，每次评估都会先删掉刚写入的记录，再判定"未发送"
   而重新投递、重新写入，下一次评估又删掉。

   ```
   REPRO QA-1 evaluate at t0 (fresh) -> new notifications = 1
   REPRO QA-1 evaluate at t0+1h (same occurrence) -> new notifications = 0
   REPRO QA-1 stale evaluation 1 at 2026-10-20T10:00:00Z (resets_at was 2026-09-10T13:00:00Z, no new observation) -> new notifications = 1
   REPRO QA-1 stale evaluation 2 at 2026-10-20T10:05:00Z ... -> new notifications = 1
   REPRO QA-1 stale evaluation 3 at 2026-10-20T10:10:00Z ... -> new notifications = 1
   REPRO QA-1 stale evaluation 4 at 2026-10-20T10:15:00Z ... -> new notifications = 1
   ```

   现实路径：Codex CLI 被卸载或持续探测失败，窗口停在 92%；任务 6 接线后评估器随每次刷新运行，
   40 天后起每个刷新间隔弹一次"Codex 5-hour window at 92%"，直到用户关掉提醒。

2. **对已结束的 occurrence 发出提醒。** 在窗口 `resets_at` 过去 47 小时后首次开启提醒：

   ```
   REPRO QA-2 at 2026-09-12T10:00:00Z, window resets_at 2026-09-10T11:00:00Z (ended 47h ago)
       -> delivered "Claude quota" / "5-hour window at 95% (alert threshold 90%)"
   ```

   该窗口实际早已重置，提醒给出的是一个过去的数字。决策 3 的"alerts switched on while a window
   already sits above a threshold notify once for that occurrence"成立的前提是 occurrence 仍在进行；
   对已结束的 occurrence 它不成立，C10 也没有这样授权。

- 行为风险：提醒功能恰恰在数据不可信时变成持续打扰；这是本任务 Result 行"opt-in alerts that do
  not repeat"的直接反例。
- 证据：上方两段复现；现有测试的时钟从不越过 `resets_at` 太远，`TestEvaluateAlertsPrunesExpiredNotices`
  只断言清理后行数为 0，不驱动清理后的下一次评估。

💡 修复方向：评估时以 `now` 界定当前 occurrence——`ResetsAt` 已早于 `now`（可带同一容差）的窗口
不发阈值提醒，reset 提醒同理；清理条件改为与之一致，保证凡是评估器仍可能匹配的实例都不会被删。
补两条测试：陈旧窗口在 31 天前后都不提醒；occurrence 结束后首次开启不提醒。

**QA-R1-F2 — 中：reset 提醒按 `observed_reset_at` 去重，而不是按 occurrence 去重，同一 occurrence
内一次小幅回落就会再提醒一次。**

位置：`internal/quota/alerts.go:123-127`（`notifyOnce(..., w.ObservedResetAt, 0, now)`）与
`internal/quota/state.go:83-90`（同源任意幅度的下降都会把 `observed_reset_at` 前移到本次观测时刻）。

C10 原文："Reset notice fires **once per window occurrence** when a reset is observed"。实现把台账
实例设为 `observed_reset_at`、容差 0；而 C6 的 `ApplyObservation` 对同源下降没有下限，真实重置之后、
同一 occurrence 内的任何回落（厂商修正、状态栏 JSON 数值抖动）都会刷新 `observed_reset_at`，于是
得到一个"新实例"。

```
REPRO QA-3 t0 before reset                  used= 50.0 resets_at=15:00 observed_reset_at=00:00 -> reset notices so far = 0
REPRO QA-3 real reset (occurrence r2)       used=  3.0 resets_at=20:00 observed_reset_at=15:01 -> reset notices so far = 1
REPRO QA-3 rises in same occurrence         used= 12.0 resets_at=20:00 observed_reset_at=15:01 -> reset notices so far = 1
REPRO QA-3 drops 0.5pt, same occurrence r2  used= 11.5 resets_at=20:00 observed_reset_at=15:40 -> reset notices so far = 2
```

`resets_at` 始终是 20:00，属于同一 occurrence，却发了两次"has reset"。

- 行为风险：用户收到内容为假的"已重置"通知；与本任务 Result 行和 C10 的去重键直接冲突。
- 证据：上方复现。`TestEvaluateAlertsResetNoticeOncePerObservedReset` 在重置后只让用量上升
  （3→6），从不在同一 occurrence 内下降，因此覆盖不到；测试名本身也反映了实现选择的是
  "per observed reset"而非契约的"per occurrence"。

💡 修复方向：reset 提醒的台账实例改为该窗口当前 occurrence 的 `resets_at`（沿用阈值提醒的 15 分钟
容差），`observed_reset_at` 只用来判断"本 occurrence 内观测到过重置"；无 `resets_at` 时的处理需写明。
补一条"重置后同一 occurrence 内回落不再提醒"的测试。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **关闭即不评估，而不只是不投递。** `EvaluateAlerts` 在 reading 或 alerts 关闭时第一行就返回，测试
  传入 `nil` store 作为"没有任何读取"的证明，这比断言通知数为 0 更强。
- **投递注入安全。** `OSANotifier` 用固定的 `on run argv` 脚本，标题与正文只作为 argv 传入；测试用含
  引号与 `$(touch pwned)` 的正文验证参数逐字到达。
- **"投递成功后才记账"** 让失败的投递在下一次评估重试，且 `TestEvaluateAlertsRetriesAFailedDelivery`
  断言了"恰好重试一次然后去重"。
- **跨路由精度容差** 有真实依据（prose 精确到分钟），测试同时覆盖了容差内合并与容差外分开。
- **不携带账户标识** 同时检查原始 ID 与 digest，比只查原值更严。
- **越界决策如实留痕。** migration 26、容差、at-or-above、无调用方四项都写进了 tasks.md 与 C10；
  schema 25→26 的波及面（两份 fixture 只有 `count` 变化、schema-12 升级测试补 `DROP TABLE`）已逐一核对。

### 📝 总结

评估器的开关语义、投递安全与失败重试都做得扎实。问题集中在"occurrence"这个核心概念上：阈值提醒
没有用 `now` 界定 occurrence 是否仍在进行（F1），reset 提醒则干脆没有按 occurrence 去重（F2）。
两者都会让一个标榜"不重复"的提醒功能重复或失真，且现有测试恰好都绕开了触发路径。

F1 为高：陈旧数据在真实场景（探测持续失败、客户端不再使用）下必然出现，一旦越过 31 天就是每个刷新
间隔一次的持续打扰。F2 为中：需要同一 occurrence 内出现回落，频率较低，但通知内容是假的。

验证记录：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；`go vet ./...`
零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务无关的预存漂移；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 均 exit 0。工作区 11 个条目、HEAD 未变、
无复现文件泄漏。既有测试全绿不构成反证：两项发现都不在现有断言的覆盖范围内。

残余不确定性：真实通知投递（`osascript display notification` 从 CLI 进程发出时的可见性与权限）属
tasks.md 已命名的人工验收，本轮未执行、未计分。评估器尚无调用方（决策 4），F1 的"每个刷新间隔"
频率以任务 6 接线后的调用节奏为前提。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:quota-alerts`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:quota-alerts:r1:90019db68acc`；review、
alerts-contract、verification 三项均为 fail）。

## Round 2 — 2026-09-13

## 📋 quota-alerts 修复复评

📊 总体评分：8/10

✅ 复评结论：FAIL

被评审内容状态：`head=0d5bf9a1eee0f7de7ad12d4a97d388dfd320ef12`，任务 5 的 11 个未提交路径，
manifest sha256 `0ab35852f73d88cbb8b90e293bad4ad8cdd64ba732ca933e5fad915b526074b3`
（沿用 Round 1 配方；已包含本轮 tasks.md 的状态同步）。相对 Round 1，变更集恰为 4 个文件：
`architecture.md`、`tasks.md`、`internal/quota/{alerts.go,alerts_test.go}`，与修复交接声明一致；
`notifier_darwin*.go`、migration、`store.go`、fixtures 与 `main_test.go` 的 blob 逐字未变。

评审方：claude-code（主会话，默认模型层级）。方法：用 Round 1 的原始复现重跑，新增一条针对修复
副作用的复现，并对两处修复各做一次 overlay 变异负向对照（修复方本轮未做负向对照）；工作区未被修改。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**QA-R2-F1 — 中；新增，由 QA-R1-F2 的修复引入：窗口长度未知时，之后任何一个 occurrence 都会在没有
观测到下降的情况下发出"has reset"。**

位置：`internal/quota/alerts.go:136-141`（reset 提醒的台账实例改为 `w.ResetsAt`）与
`internal/quota/alerts.go:176-179`（`resetInCurrentOccurrence` 在 `WindowMinutesReason != ""` 或
`WindowMinutes <= 0` 时直接返回 `true`）。

`observed_reset_at` 是粘性的：只有新的下降才会改写它，否则一直保留。Round 1 的实现按
`observed_reset_at` 去重，所以同一个旧值永远只提醒一次；本轮改成按 `resets_at` 去重后，每个新的
occurrence 都是一个新实例。对长度已知的窗口，`resetInCurrentOccurrence` 用"`resets_at` 前推一个
窗口长度"挡住了旧的 `observed_reset_at`；对长度未知的窗口它放行一切，于是旧的 `observed_reset_at`
在后续每个 occurrence 都被当作"本次的重置"。

复现（`window_minutes` 未报告，与 Codex 适配器的 `ReasonNotReported` 路径一致）：

```
REPRO QA-4 occurrence 1                          used=  50 resets_at=09-10 15:00 observed_reset_at=(none)      -> reset notices = 0
REPRO QA-4 real reset -> occurrence 2            used=   3 resets_at=09-10 20:00 observed_reset_at=09-10 15:01 -> reset notices = 1
REPRO QA-4 occurrence 3, no decrease (3 -> 40)   used=  40 resets_at=09-11 01:00 observed_reset_at=09-10 15:01 -> reset notices = 2
REPRO QA-4 control (window_minutes=300), same sequence -> reset notices = 1
```

occurrence 3 的用量从 3 升到 40，没有任何下降，却收到第二条"has reset — now at 40%"。对照组只有长度这一项
不同，结果为 1，确证长度未知是唯一原因。C10 原文要求 reset 提醒"driven by the observed decrease"，
tasks.md 任务 5 的范围条目同样写明"driven by the observed decrease"。

- 行为风险：长度未知的窗口在首次观测到重置之后，每个新 occurrence 只要第一次读数不低于上一次读数
  （两次探测之间用量上涨即可），就会收到一条内容为假的重置通知。
- 证据：上方复现与对照。`TestEvaluateAlertsResetNoticeOncePerOccurrence` 与
  `TestEvaluateAlertsIgnoresResetFromAnEarlierOccurrence` 都只用长度已知的 `claudeFiveHour`，覆盖不到。
- 严重度记为"中"而非"高"：只影响长度未报告的窗口，且每个 occurrence 至多一条。

💡 修复方向：长度未知时不能用"本 occurrence 起点"判定，改为要求本 occurrence 内确实发生过下降——例如
`observed_reset_at` 必须晚于该窗口台账中上一个 occurrence 的实例，或在无法界定时不发 reset 提醒并在 C10
写明；补一条长度未知、跨 occurrence 无下降不提醒的测试。

### 🟢 做得好的方面

- **QA-R1-F1 已闭合，Round 1 原始复现逐项转为 0。** 陈旧窗口在 40 天后连续 4 次评估新增提醒数
  均为 **0**（Round 1 为每次 1）；occurrence 结束 47 小时后首次开启提醒，通知数 **0**（Round 1 为 1）。
- **QA-R1-F2 已闭合。** 同一 occurrence 内回落 0.5 个百分点后，reset 提醒数仍为 **1**（Round 1 为 2）。
- **新测试确实保护修复，本轮独立做了变异负向对照：**
  - 把 `occurrenceOngoing` 改回只判断 `!ResetsAt.IsZero()`：
    `TestEvaluateAlertsIgnoresAnOccurrenceThatHasEnded` 的两个子测试都失败（4 条重复提醒；结束后
    首次开启发出 2 条阈值 + 1 条 reset）。
  - 把 reset 台账实例改回 `ObservedResetAt`、容差 0：`TestEvaluateAlertsResetNoticeOncePerOccurrence`
    失败（2 条 reset 提醒）。
- **修复位置选得准。** `occurrenceOngoing` 放在循环入口，同时覆盖阈值与 reset 两条路径；
  `alertNoticeRetention` 的注释写明了为何清理不再可能删掉仍需匹配的条目，并与 C10 同步。
- **范围克制，留痕完整。** 4 个文件全部落在两项发现的修复面内，C10 新增两条解读、tasks.md 新增
  Round 1 repair 记录。

### 📝 总结

Round 1 的两项发现都真实闭合，且本轮用变异对照证明新测试在修复被回退时会失败。

QA-R2-F1 是 QA-R1-F2 修复的边界遗漏：把去重键从 `observed_reset_at` 换成 `resets_at` 后，"这次重置
属于本 occurrence"这一判断的全部重量落在了 `resetInCurrentOccurrence` 上，而它对长度未知的窗口一律
放行。结果从"同一 occurrence 内重复提醒"变成"无下降的 occurrence 也提醒"，两者都违反同一句契约。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处无关预存漂移；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在 tasks.md 状态同步后重跑均 exit 0。
工作区 12 个条目、HEAD 未变，复现与变异副本只在会话 scratchpad 中。既有测试全绿不构成对 QA-R2-F1 的
反证：没有测试使用长度未知的窗口驱动 reset 提醒。

残余不确定性：Codex 适配器在真实响应中报告 `window_minutes` 缺失的频率未实测；真实通知投递仍属人工验收。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:quota-alerts`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:quota-alerts:r2:0ab35852f73d`；review、
alerts-contract、verification 三项均为 fail）。

## Round 3 — 2026-09-13

## 📋 quota-alerts 修复复评

📊 总体评分：9/10

✅ 复评结论：FAIL

被评审内容状态：`head=0d5bf9a1eee0f7de7ad12d4a97d388dfd320ef12`，任务 5 的 11 个未提交路径，
manifest sha256 `a2cc01ecdfdbf9aa9b405f4d41194b116bae8fe475dee69ec5d25018729c3727`
（沿用 Round 1 配方；已包含本轮 tasks.md 的状态同步，同步前为 `176dda83…`，差异只在 tasks.md）。
相对 Round 2，修复声明的变更面为 `internal/quota/{alerts.go,alerts_test.go}` 与
`docs/topics/subscription-quota/{architecture.md,tasks.md}`。

评审方：claude-code（主会话，默认模型层级）。方法：用 Round 1、Round 2 的原始复现文件（QA-1 至 QA-4 及
长度已知对照组）经 `go test -overlay` 对当前内容重跑；独立做一次变异负向对照（只把新守卫改回
`return true`）；核对 Codex、Claude 两个适配器的窗口长度来源，判断这次收窄实际影响哪些窗口；
逐段比对 architecture.md C10 相对 HEAD 的新增文本。复现与变异副本只在会话 scratchpad，工作区未被修改。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**QA-R3-F1 — 低；新增：architecture.md C10 把四条解读统称为"Three readings"，并整体标为
"operator-approved"，其中后两条（包括本轮对 reset 提醒的收窄）并不是 operator 的决定。**

位置：`docs/topics/subscription-quota/architecture.md:471-472`（引导句），其下
`:488-503` 的 "Only an ongoing occurrence notifies" 与 "Reset notice, once per occurrence" 两条。

C10 新增段（相对 HEAD 共 41 行，全部由本任务引入）以 "Three readings of the above … (quota-alerts task,
operator-approved)" 引出，下面实际有四条。前两条（Same instance、Crossing）对应 tasks.md 任务 5 记录的
operator 决策 2、3；后两条是评审修复写进去的：第三条来自 QA-R1-F1，第四条来自 QA-R1-F2 与 QA-R2-F1。
其中 QA-R2-F1 的修复选择"窗口长度未报告时不发 reset 提醒"，收窄了 C10 原文 "Reset notice fires once
per window occurrence when a reset is observed"。这个方向是 Round 2 评审给出的两个选项之一，由修复方选定，
tasks.md 与 Beads 交接都如实写了"Of the review's two directions, the conservative one"——没有 operator
决定。C10 却把它归在 operator-approved 名下。

- 行为风险：代码行为本身没有问题（见下方闭合证据）。风险在契约文本：C10 是后续任务（task 6 接线、
  task 7 Settings 文案）与版本 assemble 评审读取的依据，读者会把"长度未知不发 reset 提醒"当成已经由
  operator 拍板的产品决定，而不是一个可以复议的防御性取舍；"Three" 与四条的不一致也让人无法判断哪一条
  是后加的。
- 证据：`git diff HEAD -- docs/topics/subscription-quota/architecture.md` 显示引导句与四条 bullet 同在
  一次新增中；tasks.md 任务 5 "Round 1 repair" 与 "Round 2 repair" 两段把后两条的来源写成评审发现，
  而不是 operator 决策。
- 严重度记为"低"：实现与测试正确，Codex 适配器的全部 fixture 与 requirements.md 的字段表都给出
  `windowDurationMins`，Claude 两条路由由 `ClaudeWindowMinutes` 为 `five_hour`/`seven_day` 提供固定长度，
  收窄只触及 Codex 未报告长度的窗口。

💡 修复方向：只改 C10 文本，二选一——(a) 引导句改为准确计数，并把后两条的来源写成对应的评审发现
（QA-R1-F1；QA-R1-F2、QA-R2-F1），不再标 operator-approved；或 (b) 请 operator 确认"长度未知不发
reset 提醒"这一收窄，确认后按实际来源标注。代码与测试无需改动。

### 🟢 做得好的方面

- **QA-R2-F1 已闭合。** Round 2 原始复现 QA-4 在当前内容上三步的 reset 提醒数均为 **0**（Round 2 为
  0 → 1 → 2，其中第 2 条是用量只升不降的假重置）；长度已知的对照组仍为 **1**，说明修复没有伤到
  正常路径。
- **QA-R1-F1、QA-R1-F2 保持闭合，没有回归。** QA-1 陈旧窗口 40 天后连续 4 次评估新增提醒均为 0；
  QA-2 occurrence 结束 47 小时后开启提醒为 0；QA-3 同一 occurrence 内回落 0.5 个百分点，reset 提醒仍为 1。
- **新测试确实保护修复，本轮独立变异对照确认：** 把 `resetInCurrentOccurrence` 的长度未知分支改回
  `return true`，`TestEvaluateAlertsNoResetNoticeWhenWindowLengthUnknown` 失败并给出两条 reset 提醒
  （`now at 3%` 与假的 `now at 40%`）；对真实文件 12 个 `TestEvaluateAlerts*` 全部通过。与修复方交接中
  自述的负向对照结果一致。
- **取舍方向正确、代价已核实。** "漏发而非误发"与"alerts that do not repeat"的目标一致；收窄面仅限
  Codex 未报告 `windowDurationMins` 的窗口，Claude 窗口不受影响，阈值提醒不受影响。
- **修复范围克制。** 代码只动守卫一行与注释，测试新增一条，文档同步到 C10 与 tasks.md。

### 📝 总结

逐项处置：QA-R1-F1 保持闭合；QA-R1-F2 保持闭合；QA-R2-F1 闭合（原始复现转为 0，对照组保持 1，变异
对照证明测试有效）；QA-R3-F1 新增、低、未闭合。

行为层面三项发现都已真实闭合，代码与测试在本轮没有新问题。唯一剩下的是 C10 文本把评审驱动的取舍记成
operator 决定，且计数与条目不符。按"PASS 前所有发现须闭合"的规则，本轮为 FAIL；修复只涉及 architecture.md。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过——21 个包 ok，0 条 FAIL，退出码 0，
新测试 PASS；`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处无关预存
漂移；`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在 tasks.md 状态同步后均 exit 0。
工作区 12 个条目（含本记录）、HEAD 未变。

残余不确定性：Codex 真实响应中 `windowDurationMins` 缺失的频率未实测，收窄的实际影响面以 fixture 与
requirements.md 字段表为依据；真实通知投递仍属人工验收，本轮未执行。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:quota-alerts`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:quota-alerts:r3:a2cc01ecdfdb`；review 为 fail，
alerts-contract 与 verification 为 pass）。

## Round 4 — 2026-09-13

## 📋 quota-alerts 修复复评

📊 总体评分：9/10

✅ 复评结论：FAIL

被评审内容状态：`head=0d5bf9a1eee0f7de7ad12d4a97d388dfd320ef12`，任务 5 的 11 个未提交路径，
manifest sha256 `553e87bbd86a985fb3f504690967976fe1e574e591a9db070ddd394f0f163c13`
（沿用 Round 1 配方；已包含本轮 tasks.md 的状态同步，同步前为 `b11dbddc…`，差异只在 tasks.md）。
相对 Round 3，只有 `architecture.md` 与 `tasks.md` 的 blob 变化；`alerts.go`、`alerts_test.go`、
`notifier_darwin*.go`、migration、`store.go`、两份 fixture 与 `main_test.go` 共 9 个文件与 Round 3
manifest 逐字一致，与修复交接"No Go files changed"相符。

评审方：claude-code（主会话，默认模型层级）。方法：逐文件比对 Round 3 与本轮 manifest 的 blob；
逐条审读 C10 修改后的引导句与四条 bullet 的来源标注，并与 tasks.md 任务 5 的决策列表、Round 1–3 repair
记录交叉核对；顺带核对 tasks.md 对 C10 的反向引用是否成立。Go 代码未变，Round 3 的复现、变异对照与
全仓测试证据按内容一致性复用，不重跑；文档检查重跑。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**QA-R4-F1 — 低；新增（Round 1 起即存在，前三轮漏检）：tasks.md 任务 5 称四条 operator 决策
"recorded in architecture.md C10 as well"，但决策 4 不在 C10，也不在 architecture.md 任何位置。**

位置：`docs/topics/subscription-quota/tasks.md:498-499`（引导句），对应 `:521-526` 的决策 4
"No caller yet"。

决策 1（去重台账）对应 C10 的 ledger 段，决策 2、3 对应 C10 的 Same instance、Crossing 两条；决策 4 的
两项内容——`EvaluateAlerts`/`OSANotifier` 尚无调用方、由 task 6 随 `RefreshQuota` 接线，以及通知文本
暂为英文、本地化留给负责文案的 surfaces 任务——在 architecture.md 中检索 `caller`、`RefreshQuota`、
`EvaluateAlerts`、`English` 均无命中。与 QA-R3-F1 同类：交叉引用声称的记录位置不成立。

- 行为风险：代码无影响。task 6、task 7 的实现方按引导句去 C10 找"谁调用评估器""通知文案归谁"时找不到，
  可能重复决定或遗漏接线与本地化；assemble 评审也无法从 architecture 侧核对这两项约束。
- 证据：上方检索；`git diff HEAD -- docs/topics/subscription-quota/architecture.md` 的 48 行新增中没有
  决策 4 的内容。
- 为什么前三轮未提：Round 1–3 聚焦评估器行为与 C10 自身文本，没有反向核对 tasks.md 的"as well"声明；
  本轮核对 QA-R3-F1 的来源标注时才顺带发现。它不是本轮修复引入的回归。

💡 修复方向：文本二选一——(a) 把引导句改成准确范围（决策 1–3 同时记录在 C10，决策 4 只记录在此）；
或 (b) 在 C10 补一句决策 4 的内容并标 operator-approved。代码与测试无需改动。

### 🟢 做得好的方面

- **QA-R3-F1 已闭合。** C10 引导句改为"Four readings"，并写明前两条是 operator 决策、后两条是修复评审
  发现时加入、可复议的防御性取舍；四条 bullet 各自标注来源：Same instance、Crossing 为
  operator-approved，Only an ongoing occurrence notifies 为 QA-R1-F1，Reset notice, once per occurrence
  为 QA-R1-F2 与 QA-R2-F1，并单独点明"长度未知不发 reset 提醒"由修复选定、非 operator 决定。来源与
  tasks.md 决策 2、3 及 Round 1/2 repair 记录逐一对得上。
- **修改只动文本，范围克制。** bullet 正文除重排行宽外未改；Go 文件 blob 与 Round 3 完全一致。
- **QA-R1-F1、QA-R1-F2、QA-R2-F1 保持闭合。** 代码与测试内容与 Round 3 逐字一致，Round 3 的复现
  （QA-1/2/3/4 为 0/0/1/0，对照组 1）、变异对照与全仓测试结论直接适用。

### 📝 总结

逐项处置：QA-R1-F1 保持闭合；QA-R1-F2 保持闭合；QA-R2-F1 保持闭合；QA-R3-F1 闭合；QA-R4-F1 新增、
低、未闭合。

评估器行为在 Round 3 已全部达标，本轮修复也准确闭合了 C10 的来源问题。剩下的 QA-R4-F1 是 tasks.md
的一句交叉引用不实，按"PASS 前所有发现须闭合"的规则本轮为 FAIL；修复只涉及一处文本。

验证记录（覆盖上述内容状态）：`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在修复后
与本轮 tasks.md 状态同步后均 exit 0。Go 侧证据复用 Round 3（目标状态 `…:r3:a2cc01ecdfdb`）：全仓
`scripts/run-go-test.sh ./...` 21 个包 ok、0 条 FAIL；`go vet` 零输出；`gofmt -l` 仅无关预存漂移
`cmd/agentdeck/usage_stats_viewer_test.go`；复用依据是 9 个非文档文件 blob 逐字一致。

残余不确定性：同 Round 3——Codex 真实响应中 `windowDurationMins` 缺失频率未实测；真实通知投递仍属人工验收。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:quota-alerts`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:quota-alerts:r4:553e87bbd86a`；review 为 fail，
alerts-contract 与 verification 为 pass）。

## Round 5 — 2026-09-13

## 📋 quota-alerts 修复复评

📊 总体评分：10/10

✅ 复评结论：PASS

被评审内容状态：`head=0d5bf9a1eee0f7de7ad12d4a97d388dfd320ef12`，任务 5 的 11 个未提交路径，
manifest sha256 `9197ae81a9b24c3e1e3c45fe72a7c9623a4ccfdb6d84d21003ebeb7b82cb83ea`
（沿用 Round 1 配方；已包含本轮 tasks.md 的状态同步——Review 勾选与状态段，同步前为 `f65e7834…`，
差异只在 tasks.md）。相对 Round 4，只有 `tasks.md` 的 blob 变化；`architecture.md` 与 9 个非文档文件
均与 Round 4 manifest 逐字一致，与修复交接"tasks.md only"相符。

评审方：claude-code（主会话，默认模型层级）。方法：逐文件比对 Round 4 与本轮 manifest 的 blob；逐句核对
tasks.md 任务 5 新引导句对 C10 的定位声明是否与 architecture.md 实际内容相符；Go 代码与 architecture.md
未变，Round 3/4 的行为与测试证据按内容一致性复用；文档检查重跑。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **QA-R4-F1 已闭合。** `tasks.md:498-500` 现写"Decisions 1–3 are also recorded in architecture.md C10
  (the ledger paragraph, and the Same instance and Crossing readings); decision 4 is recorded only here"。
  逐项核对成立：决策 1 对应 C10 的 ledger 段，决策 2 对应 Same instance，决策 3 的阈值部分对应 Crossing；
  决策 4（无调用方、通知文本为英文）在 architecture.md 中确实不存在，现已如实声明只记录于 tasks.md。
- **修复范围最小。** 只改一句引导句并新增 Round 4 repair 记录；没有向 C10 添加未经 operator 确认的契约文本。
- **此前各项保持闭合。** QA-R1-F1、QA-R1-F2、QA-R2-F1 的代码与测试与 Round 3 逐字一致；QA-R3-F1 的
  C10 来源标注与 Round 4 逐字一致。

### 📝 总结

逐项处置：
- QA-R1-F1 已关闭（Round 2 起）。
- QA-R1-F2 已关闭（Round 2 起）。
- QA-R2-F1 已关闭（Round 3 起）。
- QA-R3-F1 已关闭（Round 4 起）。
- QA-R4-F1 已关闭（本轮）。

无新发现，记录中全部发现均已关闭，结论 PASS。

评估器行为：关闭即不评估、阈值按 occurrence 去重且跨 15 分钟容差合并、只有进行中的 occurrence 提醒、
reset 提醒按 occurrence 去重且只在本 occurrence 观测到下降时发出、长度未知窗口不发 reset 提醒、通知不带
账户标识、投递失败重试——均已由测试、复现与变异对照在 Round 2–3 证实，代码此后未变。

验证记录（覆盖上述内容状态）：`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在本轮
tasks.md 状态同步后均 exit 0。Go 侧证据复用 Round 3（目标状态 `…:r3:a2cc01ecdfdb`）：全仓
`scripts/run-go-test.sh ./...` 21 个包 ok、0 条 FAIL；`go vet` 零输出；`gofmt -l` 仅无关预存漂移
`cmd/agentdeck/usage_stats_viewer_test.go`；overlay 变异对照使长度未知测试失败；复用依据是 9 个非文档
文件 blob 逐字一致。

残余不确定性：Codex 真实响应中 `windowDurationMins` 缺失频率未实测，长度未知不发 reset 提醒的实际影响面以
fixture 与 requirements.md 字段表为依据；真实通知投递（`osascript display notification`）属 tasks.md 已命名的
人工验收，未执行、未计分。评估器尚无调用方，由 task 6 接线。

完成门禁：VERIFIED（CEv1 WorkUnit `subscription-quota:quota-alerts`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:quota-alerts:r5:9197ae81a9b2`；review、alerts-contract、
verification 三项均为 pass）。
