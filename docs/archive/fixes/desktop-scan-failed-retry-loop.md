---
status: historical
retired: 2026-11-27
created: 2026-11-27
---

# 缺陷：桌面扫描每轮 scan_failed 后立即重跑

载体：`ad-bug-desktop-scan-failed-retry-loop`；v0.7.5；Lane A（用户 2026-11-27 确认）。
工作区：`agent-deck.fix.desktop-scan-failed-retry-loop`；分支 `fix/desktop-scan-failed-retry-loop`，
路径 `.worktrees/fix-desktop-scan-failed-retry-loop`，基线 `main` `33b0eb16edd710cf0c32805f0836e635309f63f8`。

## 现象

安装版 v0.7.0（`4ac56db`）在 2026-11-22 至 11-25 期间，桌面扫描每轮都以 usage 和 session
两域同时 `failed`、`error_code=scan_failed` 结束，两域 `duration_ms` 相同，App 随即发起下一轮，
scan-worker 持续占用 30%–100% 单核，约 3 天没有一轮完整成功。11-25 08:43 之后安装版没有再写扫描收据。

## 诊断（2026-11-27）

方法：对安装态三个 SQLite 库各做一次只读 `.backup`，转录目录用 APFS 克隆，
在隔离 HOME 和状态目录中分别构建未 strip 的 v0.7.0 与 `main` `33b0eb16`，
在 `serving.go` 的副本中加入阶段与原始错误探针，执行 App 实际使用的 `desktop bootstrap` 路径。

| 构建与输入 | 结果 | 两域耗时 |
|---|---|---:|
| v0.7.0，冷状态 | completed / completed | 699.1 s |
| main，冷状态 | completed / completed | 549.5 s |
| v0.7.0，热状态 | completed / completed | 14.2 s |
| main，热状态 | completed / completed | 12.2 s |

- 现场故障在固定快照上没有复现。第一次出错的阶段和原始错误仍未知，可能依赖来源变化、
  并发写入或时序条件。不能把修复归因于 `040d534`。
- 已证实的放大：全量 session 扫描在 `removeMissingSourcesExec → deleteSource` 中逐路径按
  FTS5 的 `UNINDEXED` 列 `source_path` 删除，每次都走虚表全扫描。冷状态下这一段 v0.7.0 约 502 s，
  main 约 359 s；热状态降到 0.3–0.4 s。
- 重跑放大机制（源码）：`DesktopSourceChangeMonitor` 以 0.1 s FSEvents 监听转录写入，
  `requestFullRefresh` 在轮次进行中合并待处理触发，`runFullSequence` 在当前轮结束后立即执行下一轮，
  没有冷却和失败退避；失败轮不提交，下一轮重做同样的工作。

## 修复边界

1. **失败退避（Swift）**：自动触发共用一个待处理槽和一个可取消计时；失败后按单调时钟指数退避，
   从 30 秒开始翻倍，上限 15 分钟，收到一轮完整成功的终态后重置。手动刷新不受退避限制，并复用进行中的工作。
2. **来源变化最小间隔（Swift）**：自动 `.sourceChange` 按轮次起始时间限制最小间隔 30 秒，
   冷却期内的写入合并为一个 dirty 标记，冷却结束仍交付最后一次变化；与失败退避取较晚的时间。
3. **FTS 批量清理（Go）**：`removeMissingSourcesExec` 一次收集本轮失效路径对应的 FTS rowid，
   再按 rowid 删除，与 metadata、来源登记和 generation 更新保持在同一事务，保留 OTel 和特殊保留身份分支。
4. **保留原始错误（Go）**：在 serving 的批处理和收尾边界（含快照稳定后的刷新）包装阶段名，写入 worker 私有诊断日志
   （大小有上限并轮换，权限 0600）；公开结果和 receipt 的分类码契约不变。2026-11-27 主控更正：日志只写可枚举的结构化字段
   （round、阶段、domain、分类码、错误链类型名、SQLite 主码与扩展码名、文件操作与 errno 名、取消与超时哨兵），不写任何错误原文，
   以免规则遮蔽漏出路径或转录内容。

取值（30 秒起、翻倍、上限 15 分钟、成功重置；来源变化最小间隔 30 秒；手动刷新不受限）由用户 2026-11-27 确定。

第 1、2 项只改 Swift，先行实现。第 3、4 项原计划待 CJSI 合入后实现；2026-11-27 用户把 CJSI 移出 v0.7.5，
第 3、4 项改为直接在本分支（基线 `main` `33b0eb16`）实现。发布版本必须包含全部四项。

## 验证

- Swift（第 1、2 项）：候选 `git diff --binary --full-index 33b0eb16 -- apps/macos` SHA-256 `f1e8880e95e0a67319bdf15d5e6b95e4db5dd77f90a2d34d7afe2b1f9cc4a6ff`；隔离 `scripts/test-macos-app.sh` 扩展回归 AgentDeckSharedTests 224、AgentDeckAppTests 265（3 跳过）、AgentDeckWidgetTests 75 通过；8 种触发 × 4 种状态对照基线逐格核实（Round 4）。
- Go（第 3、4 项）：`./internal/session`、`./internal/scanruntime` 普通测试与 `-race` 通过（Round 6–8 评审方实跑）；FTS 批量清理在 15,000 份文档、2,900 个失效来源的合成索引上每次约 0.5–0.6 秒，逐路径实现约 58.7 秒；诊断日志只含结构化字段。
- 未执行：用真实快照重放冷启动耗时；现场失败的原始原因仍待下一次失败时从诊断日志读取。

## Review — Round 1

评审结论：FAIL

完成门禁：NOT_VERIFIED

发现：1 P1 / 2 P2 / 1 P3。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者（Codex/gpt-6.2-sol）不同 harness；只读，在临时副本中对照基线与候选运行隔离原生测试。被评候选为 `git diff --binary --full-index 33b0eb16 -- apps/macos`，SHA-256 `d5ebc4add869ed26af00219e2f3ba6f1f87a33d40c0a6483936d3bb46d70bb1b`（4 个 Swift 文件）。主控已核对 F1 所指代码：`requestFullRefresh` 只有 `.manual` 绕过期限。

- **DSR-R1-F1 / P1**：`EmbeddedHelperRunner.swift` `requestFullRefresh` 只让 `.manual` 绕过期限，用户切换供应商的 `.providerSwitch` 进入自动槽，失败退避期间最多推迟 15 分钟；槽中已有 `.sourceChange` 时也会被 30 秒冷却拖住，超过 `SwitchController` 10 秒的成功态窗口。
- **DSR-R1-F2 / P2**：设置页显式提交的刷新被当作自动触发。`SessionOriginSettingsController` 经 `AgentDeckApp.swift` 以 `.sourceChange` 请求，`QuotaSettingsController.drain()` 等待 `.periodic` 刷新，二者都会被冷却或退避推迟，后者还会阻塞后续额度设置写入。
- **DSR-R1-F3 / P2**：`requestFullRefresh` 的 `withCheckedContinuation` 不响应调用方取消；取消后仍挂起至多 15 分钟，残留的 `backgroundOnly: false` 使关闭后台自动更新时 `cancelBackgroundRefresh()` 无法取消计时器。
- **DSR-R1-F4 / P3**：测试缺口：真实 `DesktopRefreshClock.continuous` 的调度与取消未被执行；缺失败退避期间持续来源写入的防饿死、bootstrap 路径的取消/失败/成功记账、手动刷新复用自动轮次并唤醒等待者的用例；`PendingDesktopFullRefresh.merge` 未取 `recentLimit` 的最大值。

时序失败归因：`SwitchFlowTests/testConfirmationUsesExactlyTheResolvedOption` 与 `TelemetrySettingsControllerTests/testEnablingTheReceiverElsewhereStartsReceptionWithoutReopeningSettings` 在未打补丁的基线全量运行中同样失败或严重超时；跳过前置的 Settings 窗口套件后，基线与候选各 24/24 通过。结论为基线既有的宿主调度退化，非本补丁引入。

主控裁决（契约解释）：用户确定的"手动刷新不受限"按"用户显式发起的刷新"理解，包括手动刷新、供应商切换和设置页提交；只有 watcher、唤醒、定时等自动触发受退避与最小间隔约束。

## Review — Round 2

评审结论：FAIL

完成门禁：NOT_VERIFIED

发现：1 P1 / 1 P2，均为本轮引入的回归。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Codex 不同 harness；只读，在临时副本中运行隔离扩展回归和两项变异抽查。被评候选 SHA-256 `e6fec116aa2c600ffadf33c7c568f50181442fb0fb42042d682ad3795ec40f1b`（7 个 Swift 文件）。Decisions 推荐 Claude Opus，主控改派 AGY：Claude 5 小时额度留给 CJSI 修复，AGY 与作者不同 harness。主控已核对 `docs/topics/desktop-refresh/architecture.md` 第 124–129 行与 `AgentDeckApp.swift` 第 198 行。

- DSR-R1-F1 消除了期限阻塞，DSR-R1-F2 消除了设置提交被限流，DSR-R1-F3、DSR-R1-F4 关闭。
- **DSR-R2-F1 / P1**：`EmbeddedHelperRunner.swift` 把 `.providerSwitch` 移出 `requestsFollowUp`，并把基线用例 `testProviderSwitchDuringFullRefreshCreatesExactlyOneFollowUp` 换成断言复用进行中轮次的用例。这违反 desktop-refresh 架构：活动 generation 期间的手动刷新和供应商切换要设置一个跟进标记，供应商切换的跟进不能由切换前开始的扫描满足。扫描进行中切换供应商，菜单栏会停在旧供应商。
- **DSR-R2-F2 / P2**：`QuotaSettingsController` 在等待发布前释放写入锁，而 `AgentDeckApp.swift` 传入 `requiresFollowUp: false`；第一轮扫描进行中连续两次写入时，第二笔写入复用旧轮次，不再发布。

根因说明：R2 的派发说明由主控撰写，其中要求供应商切换"复用正在进行的轮次"，R1 的派发说明也要求手动刷新复用进行中的轮次，两处都与既有架构的跟进语义冲突。主控更正裁决：显式触发只绕过失败退避与来源最小间隔，其与活动轮次的关系保持架构既有语义（手动刷新和供应商切换设置一个跟进，重复合并；供应商切换的跟进不能由切换前开始的扫描满足）。

## Review — Round 3

评审结论：FAIL

完成门禁：NOT_VERIFIED

发现：0 P1 / 1 P2 / 1 P3。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Codex 不同 harness；只读，隔离副本扩展回归与两项变异抽查。被评候选 SHA-256 `9bb8464dfc1d812125b07813f20e79aade50ca5f605a9dc4900aa4c6d3adc371`（8 个 Swift 文件）。

- DSR-R2-F1、DSR-R2-F2 关闭；DSR-R1-F1 至 F4 保持关闭。
- **DSR-R3-F1 / P2**：R3 让 `.wake` 在活动轮次中 join，不再排跟进。`docs/topics/desktop-refresh/architecture.md` 第 126 行是周期刷新时代的旧写法；提交 `5a57b26b`（scan-performance Task 6）删除了固定周期全量刷新，并把主产品契约 `docs/specs/cli-design.md` 改为"唤醒或重新开启自动更新在活动轮次期间合并为一次跟进校对"，同时加了专门的测试与变异。去掉跟进后，唤醒和重新开启自动更新在扫描进行中会被静默丢弃，且没有周期刷新兜底。
- **DSR-R3-F2 / P3**：`runFullRefresh` 把立即跟进拆成新 `Task` 后提前返回，挂在 `await activeRefresh.value` 上的 `requestQuotaRefresh(manual:)` 在跟进轮的配额阶段开始前就返回。

根因说明：Round 2 的主控裁决引用了 desktop-refresh 架构文档中已被 `cli-design.md` 取代的唤醒语义。主控再次更正：刷新合并语义以 `docs/specs/cli-design.md` 现行条款和基线 `33b0eb16` 的行为为准；本修复只增加失败退避与来源最小间隔，其他语义一律保持基线。

## Review — Round 4

评审结论：PASS

完成门禁：NOT_VERIFIED

范围：Swift 部分，修复边界第 1、2 项；Go 部分未完成，整体门禁未关闭。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Codex 不同 harness；只读，在临时副本中扩展回归与两项变异。
被评候选：`git diff --binary --full-index 33b0eb16 -- apps/macos`，SHA-256 `f1e8880e95e0a67319bdf15d5e6b95e4db5dd77f90a2d34d7afe2b1f9cc4a6ff`，7 个 Swift 文件；`QuotaSettingsController.swift` 与基线逐字节一致。

- DSR-R3-F1 关闭：`.wake` 恢复跟进语义，仍受失败退避约束；基线用例 `testWakeAloneDuringScanStillGetsItsReconciliation` 逐字节恢复，变异 M1 被 3 个唤醒用例检出。
- DSR-R3-F2 关闭：到期跟进在原活动 Task 内执行，`requestQuotaRefresh(manual:)` 的完成点与基线一致，变异 M2 检出。
- DSR-R1-F1 至 F4、DSR-R2-F1 至 F2 保持关闭。
- 作者给出的 8 种触发 × 4 种状态对照表 32 格逐一核实：差异只来自新增的失败退避与来源最小间隔，其余与基线一致。
- 隔离扩展回归：AgentDeckSharedTests 224/224，AgentDeckAppTests 265 通过、3 按入口条件跳过，AgentDeckWidgetTests 75/75，Settings 重启两阶段通过；排除的两个 Settings 窗口测试类属于 `ad-6d5i`。

修复边界第 3、4 项（Go：FTS 批量清理、原始错误日志）尚未实现，本缺陷整体未完成。

## Review — Round 5

评审结论：FAIL

完成门禁：NOT_VERIFIED

Go 部分（修复边界第 3、4 项）首轮评审。独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Claude/Opus 不同 harness；派给 Codex 时 T3 报 Codex provider 不可用（app-server 状态检查超时），改派 AGY。只读，隔离副本运行普通测试与 race、两项变异。被评候选 `git diff --binary --full-index 33b0eb16 -- . ':!apps/macos' ':!docs'`，SHA-256 `adbbd39c73315e7fb7f60e74d8eb91dc7fa7897ff04308f16024ba9c1f2caa5c`。

- 通过项：FTS 批量清理与旧实现等价，rowid、OTel 与替换文档保留、单事务回滚正确；revision/generation 推进次数变化后，Go 与 Swift 全部读方只判断相等、变化或单调，无依赖逐次计数；`./internal/session`、`./internal/scanruntime` 普通测试与 `-race` 通过，无竞争告警。
- **DSR-GO-R1-F1 / P2**：完整快照稳定后的刷新走 `scanruntime.go` `executeProduction`，其中 13 处失败点没有阶段名，也不写诊断日志。
- **DSR-GO-R1-F2 / P2**：`diagnosticPath`、`diagnosticSlashToken` 在空白与 `()[]{},;:` 处截断，未从 `*fs.PathError`、`*os.LinkError` 取路径，多空格或括号、逗号的路径会泄漏中间目录名与 `.jsonl` 文件名片段；`entry.SQLiteError` 未经遮蔽。
- **DSR-GO-R1-F3 / P3**：超限轮换时先 rename 再收紧权限，预置 0644 的旧日志轮换为 `.1` 后仍是 0644。

## Review — Round 6

评审结论：FAIL

完成门禁：NOT_VERIFIED

Go R2 复评。独立方法：冷上下文 Codex/gpt-6.2-sol，reasoningEffort=high，与作者 Claude/Opus 不同 harness；只读，隔离副本运行普通测试与 race、两项变异。被评候选 SHA-256 `209137af2b1d0a772a07d8f4af314bf6d08cd2cf574c14ee9a9677fe464c6c3f`。

- DSR-GO-R1-F1、DSR-GO-R1-F3 关闭；FTS 批量清理、事务回滚与 revision 读方复核通过；两个包的普通测试与 race 通过。
- DSR-GO-R1-F2 仍未关闭：**DSR-GO-R2-F1 / P2**，SQLite 错误里的冒号、引号、换行或多词文件名使日志保留私有路径片段；**DSR-GO-R2-F2 / P2**，路径替换破坏转义引号，隔离稳态扫描复现目录名与 JSONL 原始 `timestamp` 值泄漏。
- 主控裁决：按规则遮蔽自由文本已两轮出现新泄漏形态，改为白名单，诊断日志不再写错误原文，只写结构化字段（见修复边界第 4 项更正）。

## Review — Round 7

评审结论：FAIL

完成门禁：NOT_VERIFIED

Go R3 复评。独立方法：冷上下文 Codex/gpt-6.2-sol，reasoningEffort=high，与作者 Claude/Opus 不同 harness；只读，隔离副本运行普通测试与 race、两项变异。被评候选 SHA-256 `8323760c4066d51c3b5f6ef891e94e4f276ed6571aef3ae78925515b58d277ba`。

- DSR-GO-R1-F2、DSR-GO-R2-F1、DSR-GO-R2-F2 关闭：日志只含结构化字段，原泄漏探针全部通过；两个包普通测试与 race 通过，无数据竞争。
- **DSR-GO-R3-F1 / P3**：恢复格式非法的私有 receipt 后，原始 `round_id` 被写入日志的轮次与 inventory 字段，可带出合成路径。
- **DSR-GO-R3-F2 / P3**：FTS 批量清理把旧索引路径中的非法 UTF-8 字节替换为 U+FFFD，三张表误删仍在使用的路径、保留应删的失效路径。

## Review — Round 8

评审结论：FAIL

完成门禁：NOT_VERIFIED

Go R4 复评。独立方法：冷上下文 Codex/gpt-6.2-sol，reasoningEffort=high；只读，隔离副本运行普通测试与 race、darwin/amd64 与 darwin/arm64 构建、两项变异。被评候选 SHA-256 `f497f095abd5fdd2dbd2b2d0ab75d4d09828434402c06cc4e2a578b6bad9c76b`。

- DSR-GO-R3-F2 关闭：FTS 批量清理按原始字节比较，`unhex` 由 vendored modernc.org/sqlite v1.53.0（SQLite 3.53.2）提供，两种发布架构可用；查询计划、事务与 revision 读方复核通过。
- **DSR-GO-R4-F1 / P3**：十进制分支接受 1–20 位数字，而时钟回退只产生规范的非负 int64（至多 19 位、无前导零），恢复的 receipt 可把 20 位数字写入日志 ID。
- **DSR-GO-R4-F2 / P3**：测试对随机 ID 做大写转换后期望 `invalid`，ID 恰好全为数字时会误报。

主控直接修复两项 P3：十进制分支改为解析为非负 int64 且与规范表示一致；测试改用含 a–f 的固定大写 ID，并增加全数字 24 位十六进制合法、20 位数字非法、前导零非法三个用例。`./internal/scanruntime`、`./internal/session` 隔离测试通过，`go vet` 与 `gofmt` 通过。修复后的 Go 候选 SHA-256 `34dc5136f094a56e83b065a49a3bc692e557bb66a32f0d191eac3793aecd7fba`，复评由 PR 上的 Codex 代码评审进行。
