---
status: active
topic: subscription-quota
subject: wire-and-cli
---

# Wire and CLI Review

## Round 1 — 2026-09-13

## 📋 wire-and-cli 实现评审

📊 总体评分：7/10

✅ 评审结论：FAIL

被评审内容状态：`head=dbd119fe7c6fe4ae8e5485ce2b94fa243913100d`，任务 6 的 21 个未提交路径，
manifest sha256 `835f05b18848cb3604fde8129910a34be8e8b5ecd3cbedbc7b42c3625023f602`
（`head=` 行加按路径排序的 `<git hash-object> <path>` 行，本记录自身除外；已包含本轮 tasks.md 的状态同步）。

评审方：claude-code（主会话，默认模型层级；未参与本任务实现）。方法：对照 tasks.md 任务 6、architecture.md
C9–C12、requirements.md 第 6、7 条逐行审读 `internal/desktop/subscription.go`、`cmd/agentdeck/quota.go`、
`internal/quota/settings.go`、`DesktopWire.swift` 的解码器、各处 diff 与新增测试；与 `usagehook` 的
status-line 状态机、`quota.Scheduler.recordFailure` 的失败记账、usage hook 的命令构造逐一交叉核对；
关键判断由 `go test -overlay` 注入的只读复现测试得出，复现文件只在会话 scratchpad，工作区未被修改。

### 🔴 严重问题 — 必须修复

**WC-R1-F1 — 高：一次成功之后，手动刷新的探测失败在 wire 与 CLI 上都不可见；手动刷新遇到 prose 解析失败时，
上一次的数字被当作当前值继续展示，违反 requirements.md 第 6 条。**

位置：`internal/desktop/subscription.go:172`（`failed := hasRecord && rec.Failure != "" &&
rec.FailureAt.After(observedAt)`），与 `internal/quota/scheduler.go:203-219`（`recordFailure`：手动失败保留
envelope 原有的 `FailureAt`）交互。

任务 4 的 GS-R3-F1/GS-R4-F1 为保护退避链，规定手动失败只写 `Failure`，`FailureAt` 保持原值；一次成功后
`PutEnvelope` 已把它清空，所以手动失败后 `FailureAt` 为零。本任务的投影却以 `FailureAt` 晚于最新观测作为
"失败发生在最后一次成功之后"的唯一判据，零值永远不晚于任何观测，于是这次失败被整条吞掉。

```
REPRO WC-2 after manual failure: envelope failure="probe_failed" failure_at_zero=true observed_at=2026-09-13T21:00:08Z
REPRO WC-2 manual failure -> wire failure=<nil>
REPRO WC-2 control (failure_at=now) -> wire failure=probe_failed
```

同一序列下 CLI 文本输出为 `Codex, plan pro, via codex_app_server, observed …`，没有任何失败提示。对照组只把
`FailureAt` 改为当前时刻，失败即出现，确证判据是唯一原因。

- 行为风险：手动刷新是用户在 popover 里主动触发的动作（task 7 接线），失败后界面照常显示旧数字而不说明失败；
  若失败原因是 `parse_failed`，`subscription.go:179` 的"解析失败不再展示数字"分支同样被跳过，上一次的数字被当作
  当前值呈现——requirements.md 第 6 条原文 "No partial number is emitted, and the prior value is not
  re-presented as current"。`stale` 只按观测年龄计算，刚成功不久的数据不会被标陈旧，无法兜底。
- 证据：上方复现与对照。`TestBuildSubscriptionFailureStates` 直接写入带 `FailureAt` 的 envelope，从未经过
  scheduler 的手动失败路径，因此覆盖不到；`cmd/agentdeck/quota_test.go` 的刷新测试只驱动成功探测。
- 严重度记为"高"：触发条件是普通用户操作，且直接违反一条已评审的需求条款。

💡 修复方向：让投影能判断"失败晚于最后一次成功"而不依赖退避链的锚点。可选其一并写入 tasks.md 任务 6：
(a) 利用成功时 `PutEnvelope` 清空 `Failure` 的事实，以 `rec.Failure != ""` 判定 envelope 自身的失败晚于其
`ObservedAt`，再单独处理 Claude status-line 窗口比 prose 失败更新的情形；或 (b) 在 envelope 上另记一个与退避
无关的失败尝试时刻（涉及 task 1/4 的存储，需要 operator 确认）。补测试：经 `desktop quota-refresh --manual`
驱动"成功 → 手动失败"，分别断言 `probe_failed` 可见、`parse_failed` 不展示数字。

### 🟡 改进建议 — 建议处理

**WC-R1-F2 — 低：CLI 文本在"有数字、上次探测失败"时输出 `last probe probe failed`。**

位置：`cmd/agentdeck/quota.go:121-123`（`"last probe "+quotaReasonPhrase(client.Failure)`）与 `:172-173`
（`probe_failed` 的短语就是 `probe failed`）。

`parse_failed` 不会走到这里（它不带数字），所以这个分支实际只会出现 `probe_failed`，拼出来必然是重复词。

- 证据：WC-2 对照组的文本输出首行 `Codex, plan pro, via codex_app_server, observed …, last probe probe failed`。
  `TestQuotaCommandRendersFiguresAsText` 只断言无失败的情形。
- 行为风险：C12 把文本形式定为 CLI 对这个问题的回答，这里的输出是可读性缺陷，不是措辞偏好。

💡 修复方向：改成不重复的短语（例如 `last probe failed`，或复用 `quotaReasonPhrase` 并去掉前缀），并在 F1 的
测试里断言该行。

### 🟢 做得好的方面

- **C11 的形状落实到位。** 节在 `WireVersion` 1 下追加；不带 `account_id`、`billing`；`tightest_window_key`
  在构建 payload 时解析一次、无窗口为 null；`windows[]` 保持厂商顺序；`attribution_confirmed` 按客户端给出。
  `TestBuildSubscriptionCarriesFiguresInVendorOrderWithTheTightestWindow` 连 JSON 里不出现账户与余额字段都查了。
- **阅读关闭的语义完整。** 全新安装下 `agentdeck quota` 报 `probe_disabled` 且不创建状态；已有状态时 gate 在读取
  存储之前生效，存储的观测保留但不展示。
- **关闭读取时的 status-line 恢复是安全的。** 只在 consent 为真或 `StatusLineStatus` 为
  `configured`/`modified`（AgentDeck 自己的命令，含被改动的）时恢复；`modified` 的判定来自
  `managedStatusLineCommand`，用户自己的 `statusLine` 报 `absent` 而不被触碰——已对照 `usagehook/config.go`
  `StatusLineStatus` 核实，并有 `TestDesktopQuotaSettingsLeavesAUsersOwnStatusLineAlone`。
- **命令构造与既有惯例一致。** `quotaAgentDeckCommand` 与 usage hook 的 `runUsageHookLifecycle` 相同
  （`agentdeck` 加可选 `--state-dir`），`RestoreStatusLine` 能认出同一条命令。
- **Swift 解码器与既有节的惯例一致。** 缺失节解码为 `.unavailable`（与 `workSignals`、`presentation` 同法），
  存在但非法的字段被拒绝，并校验 `tightest_window_key` 必须指向某个窗口。
- **设置存储的取舍有据。** 存储值不可解析时报错而不静默回落默认，理由（阅读开关是 kill switch）写在代码注释里。

### 📝 总结

C11 的 payload 形状、C12 的退出码约定、C9 的关闭读取恢复、C10 的评估器接线都已落地，测试覆盖了大部分契约条目。
问题集中在一处跨任务交互：任务 4 为保护退避链让手动失败不移动 `FailureAt`，本任务的投影却恰好以 `FailureAt`
判定失败的先后（F1），结果是用户手动刷新失败时既看不到失败、在解析失败时还会看到旧数字。F2 是同一路径上的
文本缺陷。两项都未被现有测试覆盖，因为测试从未经过 scheduler 的手动失败路径。

已排除的疑点（核实后不构成发现）：
- 成功结果写 stdout、错误信封写 stderr（`cmd/agentdeck/main.go:295-306`），`quota-settings` 在恢复失败时不会
  在 stdout 上产生两份 JSON。
- `BuildSubscription` 在 `provider.Service.Current` 出错时按"未选 official"处理，与已评审通过的任务 4
  `RefreshQuota`（`internal/desktop/desktop.go:319-322`）一致，不在本任务另计。
- 复现中 `reset_allowance.total` 为 0 源自复现器的零值假数据；真实适配器在 `internal/quota/codex.go:377` 总是
  设置 `TotalReason = not_reported`，store 往返保留该原因。
- 原型 `prototype/src/Cli.jsx` 确无 quota 样例，实现方"文本沿用 CLI 既有的英文平铺形式"的说明属实。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过（退出码 0，wrapper 报告 go test passed）；
`go vet ./...` 零输出；`gofmt -l` 仅剩 `cmd/agentdeck/usage_stats_viewer_test.go` 这处与本任务无关的预存漂移；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在本记录与 tasks.md 状态同步后 exit 0。
既有测试全绿不构成对 F1 的反证：没有测试经过手动失败路径。

残余不确定性：Swift 侧只做了实现方的 `swiftc -parse` 与 Foundation verifier 构建，XCTest 在本环境无完整 Xcode
无法运行，`DesktopWireTests.swift` 的新增用例未执行；`docs/specs/cli-design.md` 的对齐按 tasks.md 留给 task 7。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:wire-and-cli`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:wire-and-cli:r1:835f05b18848`；review 与 wire-cli-contract 为
fail，verification 为 pass）。

## Round 2 — 2026-09-13

## 📋 wire-and-cli 修复复评

📊 总体评分：8/10

✅ 复评结论：FAIL

被评审内容状态：`head=dbd119fe7c6fe4ae8e5485ce2b94fa243913100d`，任务 6 的 21 个未提交路径，
manifest sha256 `415155cecc645e4a2d8b05369a029ef65e1abf88b13130b739a751beb9e9e5d1`
（沿用 Round 1 配方；已包含本轮 tasks.md 的状态同步）。相对 Round 1，blob 变化的恰为
`internal/desktop/subscription.go`、`cmd/agentdeck/quota.go`、`cmd/agentdeck/quota_test.go`、`tasks.md` 四个
文件，与修复交接一致；其余 17 个路径逐字未变。

评审方：claude-code（主会话，默认模型层级）。方法：用 Round 1 的原始复现 WC-2 对当前内容重跑；对两处修复各做一次
overlay 变异负向对照；针对新判据新暴露的路径写复现 WC-3（手动与后台两种触发下的 prose 解析失败）；复现与变异
副本只在会话 scratchpad，工作区未被修改。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

**WC-R2-F1 — 中；新增，由 WC-R1-F1 的修复暴露：手动刷新遇到 prose 解析失败时，payload 的 `observed_at` 为
null，CLI 不再给出尝试时刻，requirements.md 第 6 条要求的"the observation instant of the failed attempt"缺失。**

位置：`internal/desktop/subscription.go:190-192`（解析失败分支以 `out.ObservedAt = timeText(rec.FailureAt)`
作为尝试时刻），与 `internal/quota/scheduler.go:217`（手动失败沿用 envelope 原有的 `FailureAt`）交互。

Round 1 修复把"失败晚于最后一次成功"的判据改为 `rec.Failure != ""`，手动失败因此能进入解析失败分支——这正是
WC-R1-F1 要的。但该分支仍从 `FailureAt` 取尝试时刻，而手动失败按任务 4 的 GS-R3-F1 不写 `FailureAt`：成功之后它
已被清空，从未成功时它本来就是零。于是分支给出失败原因、不给数字，却给不出尝试时刻。

```
REPRO WC-3 manual parse failure after success         -> failure=parse_failed observed_at=<nil> windows=[] | text: "Claude: no figures, output not recognized\n"
REPRO WC-3 manual parse failure, never succeeded      -> failure=parse_failed observed_at=<nil> windows=[] | text: "Claude: no figures, output not recognized\n"
REPRO WC-3 background parse failure, never succeeded  -> failure=parse_failed observed_at=2026-09-14T04:22:38Z windows=[] | text: "Claude: no figures, output not recognized (attempted 2026-09-14T04:22:38Z)\n"
```

对照组只把触发改为后台，尝试时刻即出现，确证手动触发不写 `FailureAt` 是唯一原因。

- 行为风险：用户手动刷新后看到"输出无法识别"，却不知道是哪一次尝试、是不是刚才这一次；在 Round 1 之前这条路径根本
  不可达（失败被整条吞掉），所以这是修复打开的新缺口，而非既有状态。requirements.md 第 6 条三个要素（原因、无数字、
  尝试时刻）现在满足前两个。
- 证据：上方复现与对照。`TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible` 断言了 `failure` 与窗口数，
  没有断言 `observed_at`，因此覆盖不到。
- 严重度记为"中"而非"高"：原因可见、没有错误数字，缺的是时刻；但它是已评审需求条款的明文要素。

💡 修复方向：手动失败需要一个与退避链无关的尝试时刻。Round 1 列出的方向 (b)——在 envelope 上另记失败尝试时刻，
`recordFailure` 对手动与后台都写入，退避只读 `FailureAt`——正好对应这里；它改动任务 1/4 的存储与 scheduler，
需要 operator 确认后写入 tasks.md 任务 6。补测试：手动解析失败（成功后、从未成功两种）断言 `observed_at` 非空且
等于该次尝试时刻，CLI 文本含 `attempted`。

### 🟢 做得好的方面

- **WC-R1-F1 已闭合。** Round 1 原始复现 WC-2 在当前内容上：envelope `failure=probe_failed`、`FailureAt` 仍为零，
  wire 上 `failure=probe_failed`（Round 1 为 nil），数字保留；手动解析失败不再展示旧数字（WC-3 三组 `windows=[]`）。
- **WC-R1-F2 已闭合。** 文本为 `last probe failed`，不再重复。
- **新测试确实保护修复，本轮独立做了变异负向对照：**
  - 把判据加回 `rec.FailureAt.After(observedAt)`：`TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible`
    失败（codex `failure:<nil>`）。
  - 把文本前缀改回 `"last probe "`：同一测试失败（输出含 `last probe probe failed`）。
- **测试走真实路径。** 新测试经 `desktop quota-refresh --manual` 驱动"成功 → 失败"，并用 `ClaudeFailureError`
  产生真实的 `parse_failed` 分类，而不是直接写 envelope——正是 Round 1 指出的覆盖缺口。
- **status-line 例外处理有据。** `supersededByStatusLine` 只在 status-line 窗口晚于 envelope 自身 `ObservedAt` 时
  压过 prose 失败，理由写在注释与 tasks.md 修复记录中；该路径的数据来自另一条独立路由，不属于"把旧值当作当前"。

### 📝 总结

逐项处置：
- WC-R1-F1 已关闭。
- WC-R1-F2 已关闭。
- WC-R2-F1 新增、中、未闭合。

Round 1 的两项发现都真实闭合，且变异对照证明新测试在修复被回退时会失败。WC-R2-F1 是同一交互的下一层：失败现在
可见了，但手动失败没有尝试时刻，因为任务 4 为保护退避链不让手动失败写 `FailureAt`，而投影仍从它取时刻。修复需要
一个与退避无关的时刻，涉及任务 1/4 的存储，应先经 operator 确认。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过（退出码 0，wrapper 报告 go test passed）；
`go vet ./...` 零输出；`gofmt -l` 仅剩无关预存漂移 `cmd/agentdeck/usage_stats_viewer_test.go`；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在 tasks.md 状态同步后 exit 0。既有测试全绿不
构成对 WC-R2-F1 的反证：没有测试断言手动失败的 `observed_at`。

残余不确定性：同 Round 1——XCTest 在本环境无法运行，`DesktopWireTests.swift` 未执行。

完成门禁：FAILED（CEv1 WorkUnit `subscription-quota:wire-and-cli`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:wire-and-cli:r2:415155cecc64`；review 与 wire-cli-contract 为
fail，verification 为 pass）。

## Round 3 — 2026-09-13

## 📋 wire-and-cli 修复复评

📊 总体评分：10/10

✅ 复评结论：PASS

被评审内容状态：`head=dbd119fe7c6fe4ae8e5485ce2b94fa243913100d`，任务 6 的 28 个未提交路径，
manifest sha256 `5975112f91f4b8fb558fafd94fcd61775b229036907dae626c6a68789c7c7cb9`
（沿用 Round 1 配方；已包含本轮 tasks.md 的 Review 勾选与状态同步）。相对 Round 2，路径由 21 个增至 28 个：新增
`internal/quota/{model.go,store.go,scheduler.go,store_test.go,scheduler_test.go}`、
`internal/store/{migrations.go,store.go}`（schema 26 → 27）；blob 变化的还有 `subscription.go`、
`subscription_test.go`、`quota_test.go`、两份 desktop fixture 与 `tasks.md`。与修复交接列出的文件一致。

评审方：claude-code（主会话，默认模型层级）。方法：逐段审读存储、scheduler、投影三处 diff 与迁移 27；用 Round 2
的原始复现 WC-3 对当前内容重跑；对投影与 scheduler 两处修复各做一次 overlay 变异负向对照；核对迁移对既有行的
默认值、schema 计数的波及面（fixture、升级测试、Swift 侧）与主题文档中的 schema 版本记载。复现与变异副本只在会话
scratchpad，工作区未被修改。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **WC-R2-F1 已闭合。** Round 2 原始复现 WC-3 在当前内容上：手动解析失败（成功之后）`observed_at` 为该次尝试时刻，
  文本含 `(attempted …)`；手动解析失败（从未成功）同样给出时刻；后台对照组保持不变。Round 2 前两组均为 `null`。
- **修复没有触动退避链。** `recordFailure`（`internal/quota/scheduler.go:206-224`）只新增把 `attemptedAt` 写入
  `FailureObservedAt`，手动失败仍沿用原有的 `FailureAt`/`BackoffUntil`；GS-R3-F1 保护的步长推导不变，任务 4 的
  scheduler 测试全部通过。
- **新测试确实保护修复，本轮独立做了变异负向对照：**
  - 投影改回 `timeText(rec.FailureAt)`（`internal/desktop/subscription.go:197`）：
    `TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible` 失败（claude `observed_at:<nil>`）。
  - scheduler 改为不写真实尝试时刻（`scheduler.go:224` 传入 `failureAt`）：
    `TestSchedulerManualFailureWithNoPriorFailureStoresNoBackoffChainInstant/after_a_success` 失败，CLI 测试同时失败。
- **迁移对既有数据安全。** `ALTER TABLE quota_envelopes ADD COLUMN failure_observed_at TEXT NOT NULL DEFAULT ''`
  使升级前的行扫描为空串而非 NULL，`Envelope` 不会因旧行报错；`PutEnvelope` 在成功时与 `Failure`/`FailureAt`
  同一次写入清空该列。
- **波及面处理完整。** 两份 desktop fixture 的差异只有 Doctor `schema` 计数 26 → 27；schema-12 升级测试在全仓测试中
  通过；Swift 侧没有硬编码 schema 计数；tasks.md 的版本记载（任务 4 的 24 → 25、任务 5 的 25 → 26、本轮 26 → 27）
  前后一致；architecture.md 不列 envelope 存储字段，无需同步。
- **越界改动如实留痕。** 修复触及任务 1 的 schema 与任务 4 的 scheduler，tasks.md 任务 6 的 Round 2 repair 记录写明了
  operator 批准、改动文件与不变量，与任务 4 当年修改 `store.go` 的留痕方式一致。

### 📝 总结

逐项处置：
- WC-R1-F1 已关闭（Round 2 起）。
- WC-R1-F2 已关闭（Round 2 起）。
- WC-R2-F1 已关闭（本轮）。
- GS-R3-F1、GS-R4-F1 均已关闭：二者是任务 4 的发现，本记录各轮只作引用，处置见 `reviews/gate-and-schedule.md`。

无新发现，记录中全部发现均已关闭，结论 PASS。

wire 与 CLI 的契约条目在本内容状态上全部成立：C11 的追加节形状、`tightest_window_key` 与厂商顺序、C12 的退出码约定、
阅读关闭的 `probe_disabled`、C9 的关闭读取恢复、C10 的评估器接线；失败投影现在对手动与后台两种触发给出一致的原因、
数字取舍与尝试时刻，满足 requirements.md 第 6 条的三个要素。

验证记录（覆盖上述内容状态）：全仓 `scripts/run-go-test.sh ./...` 通过（退出码 0，wrapper 报告 go test passed）；
`go vet ./...` 零输出；`gofmt -l` 仅剩无关预存漂移 `cmd/agentdeck/usage_stats_viewer_test.go`；
`check-whitespace.sh` 与 `check-topic-docs.sh subscription-quota` 在 tasks.md 状态同步后 exit 0。

残余不确定性：
- operator 对存储改动的批准记录在 tasks.md 与 Beads 交接中，本评审会话未直接见证该批准。
- 升级前已存在的失败记录在下一次尝试之前 `observed_at` 为 null（旧行没有可恢复的尝试时刻），属一次性过渡状态。
- XCTest 在本环境无法运行，`DesktopWireTests.swift` 未执行。
- `docs/specs/cli-design.md` 的对齐按 tasks.md 留给 task 7。

完成门禁：VERIFIED（CEv1 WorkUnit `subscription-quota:wire-and-cli`，目标内容状态
`urn:agent-deck:content-state:subscription-quota:wire-and-cli:r3:5975112f91f4`；review、wire-cli-contract、
verification 三项均为 pass）。
