---
status: active
created: 2026-11-17
---

# 缺陷：watch 轮内中断被报告为 runtime_error

协调载体：`ad-bug-watch-interrupt-mid-round-exit-1`；版本：v0.7.5；Lane A。
工作区：`agent-deck.fix.watch-interrupt-mid-round-exit-1`，分支 `fix/watch-interrupt-mid-round-exit-1`。
用户已授权自动开发、独立评审与修复；交付上限 none，未授权 Git 提交、推送、集成或安装。

## 现象

watch 在轮间收到调用者取消时正常结束；取消落在 Snapshot、Scan 或 fingerprint 持久化中时，原实现把包装后的 context cancellation 原样交给命令层，产生 runtime_error 与失败退出。
原 issue 的隔离信号实验记录 8 次轮内中断均 exit 1；本次回归不使用真实用户会话或状态。

## 根因

`Service.Run` 只在 timer/select 的 ctx.Done 路径返回 nil；`Poll` 返回的取消错误没有对应正常停止处理。
扫描锁释放错误可能与原错误通过 errors.Join 合并，修复不能把任一包含 cancellation 的复合错误一律当作成功。

## 修复边界

- 仅调整 watch 轮内取消识别、取消后的事件/检查点推进与既有锁释放路径。
- 独立业务错误、emit 错误和锁释放失败继续返回错误；保留调用者取消与内部独立失败的区别。
- 不改变扫描结果、持久化 schema、轮间间隔或公开命令参数。
- 生产代码：`internal/watch/watch.go`；回归：`internal/watch/watch_test.go`、`cmd/agentdeck/main_test.go`。
- 本记录不以开发者结论代替独立评审；尚未记录 Review PASS。

## 验证

候选基线 HEAD：`4ac56db32e13c4e925b648c620ba76fc588a7781`。
主控核对的代码/测试 blob：

| 路径 | Blob |
| --- | --- |
| internal/watch/watch.go | d15f209c7d02591864aa8b9fe4ed2262aa414314 |
| internal/watch/watch_test.go | c6ff620df035017f4d2d1c265fcc5425b6fa904b |
| cmd/agentdeck/main_test.go | 192bf15b6a2d29e8e75e4346414e6ff0e1d291a6 |

包含 HEAD、以上按路径排序的 blob 和 go.mod/go.sum/vendor/modules.txt 身份的候选 SHA-256 为 `aa6274fac20bdce0d2591ef62a483c88a8c2560cd6cbe323d294918a524388fe`；完整 recipe 以正式 CEv1 ContentState 为准。
环境：Go 1.27.1，darwin/amd64；Go 测试使用项目 wrapper。

主控从 T3 原始 command_execution 回读核对以下结果；临时测试日志已随 runtime 清理，不作为本记录的持久引用。

| 检查 | 观察结果 |
| --- | --- |
| 修复前 `./internal/watch -run 'TestRunStopsCleanlyOnMidRoundCancellation\|TestRunPreservesErrorsWhenCancelledMidRound'` | exit 1；正常取消组 9 个子用例失败，错误保留组通过；包括 snapshot、scan、post-scan、persist 和忽略 context 的路径 |
| 修复前 `./cmd/agentdeck -run TestWatchMidRoundCancellationExitsCleanly` | exit 1；session snapshot 与 extension fingerprint persist 两个取消路径均返回取消错误 |
| 最终候选 `-race ./internal/watch/...` | exit 0 |
| 最终候选 `./cmd/agentdeck -run 'Test.*Watch'` | exit 0 |
| 最终候选 `make vet`、`make check-whitespace` | exit 0 |

开发期间另有一次 lock 测试夹具断言失败，夹具修正后最终 watch race 回归通过；未将中间失败隐去或计为最终产品失败。
命令层当前只完成 watch 相关选择，未声明完整 cmd 包或全仓回归已运行；仍需独立评审决定受影响范围是否有额外缺口。
这些是隔离回归，未声明已安装 App、真实会话或真实进程信号验收。

实现来源为 Antigravity / Gemini 4 Hydrogen，请求 thought_level=medium；T3 未提供 executedModel，不能将请求配置当作独立执行证明。
独立评审和 `fix:watch-interrupt-mid-round-exit-1` 的最终候选 gate 均待完成。

## Review — Round 1

日期：2026-11-17。独立来源：Claude harness，Sonnet 5.5，requested effort=high；冷上下文只读评审。主控核对了实际生产错误传播与候选 blob，裁决由主控记录。

### 📋 评审报告

📊 总体评分：5.8/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

**V075-WATCH-R1-F1 — 高：包装的复合错误被当成纯取消吞掉。**
位置：`internal/watch/watch.go` 的 `isCancellationOnly` 与 `Run`。
`Poll` 将 Source 返回的 `errors.Join(realError, ctx.Err())` 用 `fmt.Errorf("scan ...: %w", err)` 再包装；判断函数只递归多子错误，遇到单子包装直接调用 `errors.Is`。只要任一后代是取消便返回 true，使真实错误被转换成成功退出。
风险：源读取、扫描或指纹持久化的真实故障与取消同时发生时错误丢失。
证据：主控检查当前函数及 Poll 包装路径，确认 `errors.Is` 的任一匹配语义不满足“所有叶子均为取消”的要求；Reviewer 的隔离 overlay 还复现了 Join、多个 `%w` 及包装 Join 三种场景。正式回归将在修复前针对冻结候选执行并记录。
💡 有界修复：完整遍历单子与多子错误树，仅取消叶子可转正常退出；补 Snapshot/Scan/PostScan/Persist 复合错误回归，保留原有纯取消及锁释放错误用例。

#### 🟡 建议改进 — 待核对事项

Reviewer 的 **V075-WATCH-R1-F2** 提醒中断可能丢弃已经完成源的缓冲事件。当前接口按 Poll 整轮返回事件，原有错误路径同样丢弃本轮事件，未定义每个已提交源必有一条持久送达事件的保证。本轮不新增分源流式交付契约；此项作为被审查的行为边界保留，复评须核对它是否违反既有保证，不能仅因“低优先级”跳过实际缺陷。

#### 🟢 优点

取消与锁释放的路径覆盖较完整，已区分独立 emit 错误，并提供了实际修复前失败的回归。

#### 📝 总结

被评代码指纹 `aa6274fac20bdce0d2591ef62a483c88a8c2560cd6cbe323d294918a524388fe`；F1 未关闭，不能通过。完整 cmd、真实进程信号和 sqlite 中断验收未由本轮复核；已有精确候选回归继续有效。

下一步指令：修复：fix / watch-interrupt-mid-round-exit-1 / V075-WATCH-R1-F1

## 修复 — V075-WATCH-R1-F1

2026-11-17 主控按运行时“小评审问题由主控修正”的要求执行有界修复。新增现有 Source 生命周期中的 Snapshot/Scan/PostScan/Persist 四阶段与 Join/多个 %w/包装 Join 三种组合，共 12 个回归；修复前全部失败，Run 均误返回 nil。
生产修正使单子 Unwrap 和多子 Unwrap 都递归遍历，仅叶子等于调用者 context 的取消错误时转正常结束；真实错误叶子不再因兄弟取消而被吞。

最终代码/测试候选指纹：`ebea8021051e46f87d32a76ea96511dff3f202c08a58330600b24fdcf2db0264`。
`internal/watch/watch.go` blob：`7e20423cb7cd77264867e7b1ad7f60a1978424e4`；
`internal/watch/watch_test.go` blob：`6ceeccc3903241c2cc85bc06982b8936946bca0f`；
cmd 测试仍为 `192bf15b6a2d29e8e75e4346414e6ff0e1d291a6`。
新的 CEv1 ContentState 保存完整路径排序与依赖 recipe；上方初版身份保留作为历史。

修复后 `scripts/run-go-test.sh -race ./internal/watch` 通过；
`scripts/run-go-test.sh ./cmd/agentdeck` 完整包通过；
`make vet`、`make check-whitespace`、`git diff --check` 均通过。
主控直接观察 wrapper 的真实退出码与完成输出。F1 已实现修正，仍等待独立复评；F2 的既有事件契约边界一并交复评核对。没有提交、推送或真实状态变更。

## Review — Round 2

方法：冷上下文 Claude/Sonnet 5.5，requested effort=high；只读检查及仓库外 overlay 对照。主控核对生产路径与原有整轮返回语义；正式结论如下。

### 📋 复评报告

📊 总体评分：7.5/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

**V075-WATCH-R2-F1 — 已成功提交的整轮完成事件被后置取消检查丢弃。**
位置：`internal/watch/watch.go`，持久化并更新指纹后的取消检查，以及 `release()` 成功后的取消检查。
风险：完整提交的唯一源或整轮已经完成，指纹已前进，但消费者没有 `scan_completed`；下次不会重扫补发。
证据：Reviewer 对 HEAD 与候选做相同 overlay 对照：PersistFingerprint 成功后取消、以及 release 内取消两种场景，HEAD 都返回成功并发出一个真实完成事件，候选返回成功但事件数为零；主控核对两处新增 return nil 的路径和 Run 的调用次序，确认存在这条回归。
💡 有界修复：删除完整持久化后与成功释放后的后置取消检查；保留开始下一步工作前的取消检查。修正把真实完成事件误称为 fake 的测试断言，补完整提交后取消及 release 时取消的回归。保持 Poll 批签名和非取消错误语义。

#### 🟡 建议改进 — 处置

**V075-WATCH-R1-F2 → CLOSED（既有批语义，非本次缺陷）。** HEAD 对整轮错误返回 `(nil, err)`，会丢弃先前源的缓冲事件；规约没有中断时逐源持久送达保证。新增分源流式返回需要另行设计，本次不扩展。此判断只适用于“仍有后续工作而整轮未完成”的情况；完整提交后取消的新增回归单列为 R2-F1，不以该处置豁免。

#### 🟢 优点

**V075-WATCH-R1-F1 → CLOSED。** 单子与多子错误完整遍历；12 个组合用例对旧 errors.Is 判断全部失败，对修复通过。Reviewer 的叶子探针覆盖纯取消、错误类型不一致、空 unwrap 及调用者尚未取消；真实 modernc sqlite 查询取消返回 ctx.Err 的叶子形态亦得到独立检查。

#### 📝 总结

被评代码指纹 `ebea8021051e46f87d32a76ea96511dff3f202c08a58330600b24fdcf2db0264`。旧 F1 已关闭，F2 已按既有语义明确处置；新增 R2-F1 必须修复。独立 watch race 与 cmd Watch 定向回归通过；真实进程信号取证仍单独进行，不作为虚构的通过证据。累计修复轮次 1；本次将进入第 2 轮有界修复。

下一步指令：修复：fix / watch-interrupt-mid-round-exit-1 / V075-WATCH-R2-F1

## 修复 — V075-WATCH-R2-F1

主控将已成功落盘和已成功释放锁之后的两处后置取消检查移除，保留开始各步骤前的取消检查；没有修改 Poll 的整轮返回签名或增加分源投递契约。
先修正原测试的完成判据并增加 release 内取消场景：修复前恰好两例失败，均已持久化但 emits=0；其余未完成取消用例通过。
修复后 `scripts/run-go-test.sh -race ./internal/watch`、`scripts/run-go-test.sh ./cmd/agentdeck -run 'Test.*Watch'`、`make vet`、`make check-whitespace` 和 `git diff --check` 均通过。
此前完整 cmd 包已通过；本次只重跑实际变化的 watch 子集，不把旧完整包结果冒称为新候选上的整包新执行。

最终候选：`0278da19870d20983c92f7126464649fc1ddef83f5fc57bcae0e956b0c6bb0f0`；
watch.go blob `9371f44d25f82b79f2b185449b8c1b645f57cdba`；
watch_test.go blob `578226022ffd401c5e100e7d9b7e9766231ee110`；
cmd/main_test.go 未变，仍为 `192bf15b6a2d29e8e75e4346414e6ff0e1d291a6`。
完整 recipe 保存于 CEv1 ContentState。第二轮有界修复已实现，正式复评仍待完成。

## 隔离真实信号验收

在上一候选 `ebea8021051e46f87d32a76ea96511dff3f202c08a58330600b24fdcf2db0264` 上，Developer 用隔离构建与 HOME/state-dir、合成 skill 进行真实 SIGINT/SIGTERM 取证：初始化 extension SQLite，删除其 watch fingerprint，持有外部 BEGIN IMMEDIATE 写事务，以 scan.lock 中的真实 watch PID 证明进程已进入轮内，再发送信号并释放 SQL 锁。
两种信号均 exit0，stdout/stderr 为空，无 runtime_error、无完成事件、fingerprint 未前进；scan.lock 释放，无遗留进程。
本次 R2 修正只改变成功提交后的事件返回；上述受阻 Scan 的取消路径、SQLite 驱动、命令信号接线、依赖和工具链均未变。最终状态通过明确的作用域影响评估复用该验收，不重标旧证据的 observed state。
此验收只覆盖 extension 域和受控 SQL 争用；usage/session 的取消由单元/命令回归覆盖，不声明全部域均已做真实进程信号实验。隔离材料已清理，不作为持久引用。

## Review — Round 3

方法：独立冷上下文 Claude/Sonnet 5.5，requested effort=high；只读源检查、仓库外 overlay 变异探针与定向 race/命令回归。主控核对返回的候选身份、终态无子操作、实际命令结果与下列关闭依据后记录正式结论。

### 📋 最终复评报告

📊 总体评分：9/10

✅ 评审结论：PASS

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无未关闭的范围内发现。Reviewer 的 V075-WATCH-R3-F1 是 R1-F2 同一既有批语义的多源观察，维持 CLOSED：整轮未完成时仍按既有 `(nil, err)` 路径丢弃缓冲事件；没有逐源送达保证。退出码恢复为正常取消，不引入分源事件交付设计，也不声称所有取消路径均不丢事件。

#### 🟢 优点及逐项处置

- **V075-WATCH-R1-F1 → CLOSED**：单子/多子错误树逐叶核对，12 个有判别力的复合错误用例继续通过，真实错误不再被取消吞掉。
- **V075-WATCH-R1-F2 → CLOSED**：既有整轮错误返回会丢弃未完成批次事件，规约没有逐源送达承诺；这是范围辨析，不是低严重度豁免。
- **V075-WATCH-R2-F1 → CLOSED**：成功持久化与释放后的两个检查已删除，测试区分已完成和未完成。Reviewer 将两个检查用外部 overlay 加回后，恰好两个对应子例再次失败，证明保护有效。
- **V075-WATCH-R3-F1 → CLOSED**：同 R1-F2 的既有多源批语义，无本修复新增的事件丢失发现。

Reviewer 独立执行当前候选 `-race ./internal/watch`、`./cmd/agentdeck -run 'Test.*Watch'` 与 diff 检查通过，三个代码/测试 blob 均与主控冻结身份一致。命令 SIGINT/SIGTERM 接线未变；真实信号的明确作用域复用论证与本轮生产 diff 一致。

#### 📝 总结

最终被评代码指纹 `0278da19870d20983c92f7126464649fc1ddef83f5fc57bcae0e956b0c6bb0f0`，两轮有界修复完成，无未关闭的范围内发现。当前完整 cmd 包与全仓测试未在本轮重跑；真实信号只覆盖隔离 extension/SQL 争用路径。独立评审 PASS 不等于 Git 交付，CEv1 gate 由主控随后查询。

Task checkpoint：`fix:watch-interrupt-mid-round-exit-1`；代码/测试候选如上。提交建议：三个 Go 文件及本 fix 记录，须先确认最终 gate VERIFIED 并取得提交授权；推送建议：当前无推送授权，待本任务授权提交、远端/CI 与交付检查。

## Task checkpoint — 2026-11-17

CEv1 最终目标 `fix:watch-interrupt-mid-round-exit-1:state:0278da19870d20983c92f7126464649fc1ddef83f5fc57bcae0e956b0c6bb0f0` 查询结果为 **VERIFIED，4/4**：failure-first、regression、signal-acceptance 与 independent-review 全部有效，无缺失、失效或未决影响。
本 A 修复无 containing topic gate。Task 保持 awaiting_commit；没有 Git 提交、推送、集成或安装，也未关闭原 Bug。
提交建议：冻结的三个 Go 文件及本 fix 记录，需另获授权并核对物料贡献、英文 Conventional subject/body、Codex trailer 与 SSH 签名。推送建议：尚无授权或远端交付动作，不把本地 PASS 视为可自动推送。
最终代码物料来源：Antigravity 的初版实现、Codex 的两轮有界修复与本记录；Claude 仅提供独立评审，不作为代码作者推断。
