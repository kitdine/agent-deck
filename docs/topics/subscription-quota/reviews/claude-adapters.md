---
status: active
topic: subscription-quota
subject: claude-adapters
---

# Claude Adapters Review

## Round 1 — 2026-09-11

## 📋 claude-adapters 实现评审

📊 总体评分：4/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CLA-R1-F1 — 高：关闭状态栏通路不能恢复原配置，且在 Claude 的 settings.json 里留下 AgentDeck 自己的键。**

位置：`internal/usagehook/config.go:874` 定义 `agentdeckStatusLinePrior` 键；
`internal/usagehook/config.go:962` 在注册时把旧值写进 `~/.claude/settings.json`；
`internal/usagehook/config.go:1023-1025` 在没有旧值时把 `statusLine` 写成 `null`，且恢复从不删除
`agentdeckStatusLinePrior`；`internal/usagehook/config_test.go:579` 把"恢复为 null"当作期望行为。

- 行为风险：授权文案承诺的是"写入 ~/.claude/settings.json 的 statusLine，并串接你已有的命令；
  关闭后恢复原配置"（`ux/settings-quota.md:88`、`:128`），C3 也写"disabling restores it"
  （`architecture.md:188`）。实际是：
  - 注册时额外写入一个不属于 Claude Code 的顶层键，超出了用户同意的写入范围；
  - 原本没有 `statusLine` 的用户关闭后得到 `"statusLine": null`，而不是回到"没有这个键"；
    Claude Code 是否接受 `null` 的 statusLine 没有核实过；
  - 无论有没有旧值，`agentdeckStatusLinePrior` 都永久留在另一个工具的配置文件里。
- 证据：复现 `TestProbeCLARestoreWithNoPriorLeavesResidue`：原文件
  `{"theme": "dark"}` 注册后再恢复，得到
  `{"theme":"dark","statusLine":null,"agentdeckStatusLinePrior":null}`，`identicalToOriginal=false`；
  `TestProbeCLARestoreWithPriorLeavesPriorKey`：有旧命令 `ccstatusline` 时恢复后
  `priorKeyStillPresent=true`。

💡 修复范围：把旧值记录在 AgentDeck 自己的状态里（例如 `~/.agentdeck` 下的状态文件或核心库），
使 `~/.claude/settings.json` 中唯一被写入的键是 `statusLine`；运行时的 `quota capture` 从同一处读取
要串接的命令。恢复时，原本没有该键就删除 `statusLine`（需要为顶层键增加删除操作），原本有值就
写回原值。改掉把 `null` 当期望的测试，并补"注册再恢复后文件与原文件语义一致"的测试（有旧值、无
旧值两种）。若修复方认为必须把旧值留在 Claude 的文件里，这会改变授权文案承诺的写入范围，应先
交由操作者决定。

**CLA-R1-F2 — 高：AgentDeck 自己的条目被改动过时，关闭后 AgentDeck 的命令仍然装着。**

位置：`internal/usagehook/config.go:1014-1019` 只要当前 `statusLine` 与注册时写入的值不完全相同，
就什么都不写直接返回 `restore_incomplete`；`internal/usagehook/config_test.go:593` 断言这时
"保持不动"。

- 行为风险：C3 规定这种情况下"disabling removes AgentDeck's command and reports that a manual check
  is needed"（`architecture.md:190-193`）；设置窗口的"恢复不完整"文案写的是"已移除 AgentDeck 写入的
  statusLine，但原有配置在此期间被改过，没有自动还原"（`ux/settings-quota.md:132`、`:157`）；C9 要求
  关闭读取时注销已安装的通路（`architecture.md:380`）。现状下，用户或 Claude Code 只要给 AgentDeck
  的条目加过一个字段（例如 `padding`），关闭后 `agentdeck quota capture` 仍会在每次状态栏刷新时运行，
  界面却告诉用户已经移除。
- 证据：复现 `TestProbeCLARestoreIncompleteLeavesAgentDeckCommandInstalled`：给注册条目加上
  `"padding": 2` 后恢复，得到 `outcome=restore_incomplete`，`statusLine` 仍是
  `agentdeck quota capture`，`stillAgentDeckCommand=true`。

💡 修复范围：当前值仍是 AgentDeck 的命令（`managedStatusLineCommand` 为真）但与注册值不同时，移除
它、不写回旧值，报告 `restore_incomplete`；当前值已不是 AgentDeck 的命令时保持不动并报告
`restore_incomplete`。修改对应测试，分别断言两种情况下文件的最终内容。

**CLA-R1-F3 — 中：串接原命令之前先打开核心库，状态栏会被锁等待和迁移拖慢。**

位置：`cmd/agentdeck/quota_capture.go:56-62` 在调用 `quota.CaptureStatusLine`（它才运行原命令）之前
调用 `opts.openStore`；`opts.openStore` 进入 `store.Open`，先以最长 5 秒等待 `state.lock`，再执行迁移
（`internal/store/store.go` 的 `open`、`lockWait`）。

- 行为风险：C3 要求包装层不引入新的失败方式，"The capture path therefore writes after invoking, or
  writes in a way that cannot abort the chain"（`architecture.md:176-182`）。状态栏命令在每次刷新时
  运行；只要有另一个 agentdeck 进程持有 `state.lock`（写操作、扫描中的状态更新、另一个会话的同一
  捕获），用户原有状态栏的输出就会推迟最多 5 秒；首次升级后迁移也会在状态栏刷新路径里执行。
- 证据：复现 `TestProbeCLACaptureOpensStoreBeforeChaining`：原命令用 perl 打印开始时间，锁空闲时
  `prior command started 30ms after capture began`，另一个进程持锁时 `prior command started 5.03s after
  capture began`。现有测试没有覆盖原命令的开始时机。

💡 修复范围：先读取要串接的命令并运行它，原命令结束后再打开存储并记录；存储打开采用短等待或不等待
（捕获本来就是尽力而为），拿不到锁就放弃本次记录。补测试：持有 `state.lock` 时原命令的输出不被推迟。

**CLA-R1-F4 — 中：Claude prose 探测的失败无法区分 `probe_failed` 与 `parse_failed`。**

位置：`internal/quota/claude.go:36-40` 把进程启动失败、非零退出、超时都用 `fmt.Errorf` 包装返回，
解析失败也返回普通错误；`claudeProseCommandArgs` 注入点没有任何测试使用。

- 行为风险：C4 规定任一行匹配失败时"the whole client result is `parse_failed`"
  （`architecture.md:220-223`），C6 的封闭原因集区分 `probe_failed` 与 `parse_failed`
  （`architecture.md:321-325`）。调用方拿到的是同一种 `*fmt.wrapError`，只能靠解析错误文本来判断，
  任务 4 无法可靠地把"claude 没装/超时"与"输出格式变了"分开呈现。任务 2 的 Codex 适配器已有
  类型化的失败分类，Claude 这一路没有。
- 证据：复现 `TestProbeCLAProseFailureKindsAreIndistinguishable`，用临时目录中的假 `claude` 脚本：
  退出码 1、输出格式改变、可执行文件不存在，三者都得到 `errType=*fmt.wrapError`。

💡 修复范围：为 prose 探测提供类型化失败（启动失败、退出、超时映射 `probe_failed`，解析失败映射
`parse_failed`），并用注入的假进程测试每一种分类，不启动真实的 claude。

### 🟡 改进建议 — 建议处理

以下同样是本目标的未关闭发现，PASS 前必须关闭。

**CLA-R1-F5 — 低：stdin 超过 64KB 或读取出错时，原命令收到空输入。**

位置：`cmd/agentdeck/quota_capture.go:45-48`。

- 证据：C3 要求"invokes the user's prior command with the same stdin"（`architecture.md:171-172`）。
  复现 `TestProbeCLAOversizedStdinNotPassedToPrior`：71690 字节的 stdin，原命令 `wc -c` 收到 `0`
  字节。状态栏载荷通常很小，但一旦超过上限，AgentDeck 会改变原命令的输入。

💡 修复范围：原命令始终拿到完整的原始 stdin；上限只约束捕获自己解析的部分。补对应测试。

### 🟢 做得好的方面

- prose 解析器严格且全有或全无，在指定时区而非本地时区解析重置时刻，处理了跨年与"8pm"这种
  没有分钟的形式；"what's contributing"一节被丢弃。测试覆盖了格式变化、缺行、未知时区与指定时区。
- 状态栏载荷的 `window_minutes` 始终来自本地常量，缺少 `rate_limits` 是 `not_reported`；原命令的
  stdout 直接接到输出上逐字节透传，捕获发生在原命令结束之后，原命令失败不影响捕获。
- 设置文件的改写复用现有的顶层键原语，保留了其他字段；幂等注册、识别已注册条目都有测试。
- 新命令保持隐藏，命令树测试与 GUI JSON 契约 fixture 同步更新；没有测试启动真实的 claude。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与实现。Method：单代理正式代码评审——
逐条对照 `tasks.md` 任务 3、`architecture.md` C3/C4/C9 与 `ux/settings-quota.md` 的授权与恢复文案，
审读全部新文件与 `internal/usagehook`、`cmd/agentdeck` 的 diff；用 Go `-overlay` 在 `usagehook`、
`quota`、`cmd/agentdeck` 三个包注入复现测试（假 `claude` 脚本，临时 settings 文件与状态目录）。未使用
子代理，未修改生产代码、仓库测试或配置。

Scope：`internal/quota/{claude,claude_test,statusline,statusline_test}.go`、
`internal/usagehook/{config,config_test}.go`、`cmd/agentdeck/{quota_capture,quota_capture_test,main,main_test}.go`、
`cmd/agentdeck/testdata/phase7/gui-json-contract.json`。注册与恢复由哪个任务调用（任务 4）不在本轮。

残余不确定：
- 状态栏载荷的 `used_percentage` 按 0-100 处理，实现已在注释中注明未经真实载荷核实。
- Claude Code 对 `null` 的 statusLine 与未知顶层键的处理没有核实；F1 不依赖这一点成立。
- 注册时插入新键会在原内容后拼出 `\n,` 这样的排版，来自已有的 `insertTopLevelValue`，JSON 仍合法，
  不记为本任务发现。

Reviewed state：branch `feature/subscription-quota`，HEAD
`e31a7715c5d46e5f3f55a968a80eb1c3294a1fa2`。manifest 为 `head=<HEAD>` 一行加 12 个路径按字母排序的
`<blob> <path>` 行（上述 Scope 与 `tasks.md`），SHA-256
`a4e6770082bb1f0bee1599f720e0a87aa093915a3d7e96f696251731c4f0c380`。关键 blob：
`config.go` `f97150f84c7ee74e6dab17946bd0f2d22199fa74`、
`quota_capture.go` `334b0dc0a81ef8e0ffbf54d10b4a0f55f0018696`、
`claude.go` `1cba4acfcf5911b5cb8c3b31f09ea27a3e00dbc5`、
`statusline.go` `eeacd09cfa2f615d583b148ae143790d394f9ec2`。复现与全仓测试运行时，只有 `tasks.md` 的
交接文字与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，0 个
  `--- FAIL`；日志 SHA-256 `4e1723f6ffa7e8a5981225b70fcf6819f0a4d6038cb31d04bc49ca20efc0a22b`。
  现有测试通过，但其中两个把 F1、F2 的错误行为当作期望。
- 复现 A（`usagehook`、`quota`）：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-cla-r1a.json
  -run 'TestProbeCLA' -v ./internal/usagehook/ ./internal/quota/`，exit 0；日志 SHA-256
  `c1bd20d73bbf7480c4d4f8d9b4769327d06d6da9fe80b9658de06ef96b6268e0`。
- 复现 B（`cmd/agentdeck`）：`-overlay <scratchpad>/overlay-cla-r1b.json -run 'TestProbeCLA' -v
  ./cmd/agentdeck/`，exit 0；日志 SHA-256
  `f275939af3823e1433e110f2b47a5f66dad35449e8c051564b1bf5c469cc3d64`。
- probe SHA-256：usagehook `cfa8af9f5a9d638f7e9e629a8262b70f74ddf3b2c6a50b2382fcb4baec3ae341`、
  quota `159da43043c8192e86d8dc51c65d5a05f02b5477ecf946a2cc1287edd5e49009`、
  cmd `50a20398b00bd1884b62996eb7c72830d058ecc2bbcb817f0b6415cf2d2279a1`。材料保存在本会话 scratchpad。
- 未单独复核 vet/gofmt。

完成门禁：FAILED。进入评审时 CEv1 中没有本任务的 WorkUnit，已依据 `tasks.md` 任务 3 事先写明的内容
建立 `urn:agent-deck:work-unit:subscription-quota:claude-adapters` 及三个必需标准 `adapter-contract`、
`verification`、`review`；本轮内容状态为
`urn:agent-deck:content-state:subscription-quota:claude-adapters:e31a771:a4e6770082bb`，三项均记为 fail。

Task 状态：Dev 保留实现方的勾选，Review 未勾选；Beads `ad-sq-claude-adapters-dev` 退回
`in_progress` 并释放认领。本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/claude-adapters.md / CLA-R1-F1 CLA-R1-F2 CLA-R1-F3 CLA-R1-F4 CLA-R1-F5

## Round 2 — 2026-09-11

## 📋 claude-adapters 修复复评

📊 总体评分：8/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

以下是本目标唯一未关闭的发现，PASS 前必须关闭。

**CLA-R2-F1 — 低；新增：stdin 上限被整个删除，留下一个无人使用且注释错误的常量。**

位置：`cmd/agentdeck/quota_capture.go:58` 无上限读取 stdin；
`cmd/agentdeck/quota_capture.go:54` 的注释称该上限"只约束捕获自己解析的部分"；
`internal/quota/statusline.go:13-16` 的 `MaxStatusLinePayloadBytes` 注释写它"约束
`CaptureStatusLine` 从 stdin 读取的量"。

- 处置：新增，由 CLA-R1-F5 的修复引入。
- 行为风险：`CaptureStatusLine` 已在本轮被拆成 `ChainStatusLine` 与
  `RecordStatusLinePayload`，注释指向的函数不存在了；常量在生产代码中没有任何引用，只有
  `cmd/agentdeck/quota_capture_test.go:192` 用它构造超长输入。两处注释都声称存在一个上限，
  而实际读取与解析都没有上限——状态栏命令每次刷新都运行，后续任务的读者据此会误判捕获会截断。
  上一轮给 CLA-R1-F5 的修复范围是"原命令始终拿到完整的原始 stdin；上限只约束捕获自己解析的
  部分"，现状是上限被整体移除。
- 证据：`rg MaxStatusLinePayloadBytes` 在生产代码中只命中常量定义与两处注释；
  `quota_capture.go:58` 为 `payload, _ := io.ReadAll(opts.stdin)`，
  `quota.ParseStatusLinePayload` 自身也不设上限。

💡 修复范围：二选一并使注释与之一致——要么让上限重新只约束捕获自己解析的部分（超过上限时跳过
本轮解析，原命令仍拿到完整 stdin），要么删除该常量并同时删掉两处提到它的注释；无论哪种，
`statusline.go` 的注释都不得再提到已被拆除的 `CaptureStatusLine`。补一条断言所选语义的测试。

### 🟢 做得好的方面

- **CLA-R1-F1 → CLOSED。** 旧值移到 AgentDeck 状态目录的 sidecar
  （`internal/usagehook/config.go:310-346`），`~/.claude/settings.json` 中唯一被写入的键是
  `statusLine`；没有旧值时恢复会删除该键（新增 `removeTopLevelValue`/`topLevelEntrySpan`）。
  复现 `TestProbeCLAR2RestoreReturnsFileToOriginal`：无旧值时
  `afterSetupExtraKeys=[] statusLinePresentAfterRestore=false byteIdenticalToOriginal=true`；
  `TestProbeCLAR2PriorRecordLivesInStateDir`：状态目录只多出
  `usagehook-statusline-prior.json`，权限 `-rw-------`；未配置 StateDir 时注册失败且不改动
  `settings.json`。
- **CLA-R1-F2 → CLOSED。** 恢复分三种情况：完全匹配则还原或删除；仍是 AgentDeck 的命令但被改过
  则删除该键并报告 `restore_incomplete`；已被换成别的命令则保持不动。对应测试
  `TestRestoreStatusLineWhenModifiedUnderneathRemovesAgentDeckCommand` 断言 AgentDeck 的命令不再留存。
- **CLA-R1-F3 → CLOSED。** `ChainStatusLine` 与 `RecordStatusLinePayload` 分离，
  `runQuotaCapture` 先串接再开库，且开库用 200ms 超时；`acquireNamedLockWithChecks` 会 select
  ctx，超时确实生效。复现 `TestProbeCLAR2CaptureTotalDurationUnderHeldLock`：锁空闲 10ms，
  持锁 210ms（Round 1 为 5.03s），两种情况原命令输出都正确。
- **CLA-R1-F4 → CLOSED。** 新增 `ClaudeFailureKind`/`ClaudeFailureError`，Start 与 Wait 分开分类，
  parse 映射 `parse_failed`，其余映射 `probe_failed`；5 个新测试用注入的假进程覆盖退出、格式变化、
  超时、启动失败与成功路径，未启动真实 claude。
- **CLA-R1-F5 → CLOSED（就其自身而言）。** 原命令现在拿到完整的原始 stdin，
  `TestRunQuotaCapturePassesFullOriginalStdinToPriorRegardlessOfSize` 断言超过 64KB 时原命令收到
  完整字节数；由此引入的上限缺失记为 CLA-R2-F1。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式复评——逐条核对
Round 1 五项发现，审读 `config.go`、`statusline.go`、`claude.go`、`quota_capture.go` 的改动与新增测试，
并核实 `store` 的锁等待是否遵守 ctx；用 Go `-overlay` 在 `usagehook` 与 `cmd/agentdeck` 注入 Round 2
复现。未使用子代理，未修改生产代码、仓库测试或配置。

逐项处置：CLA-R1-F1、CLA-R1-F2、CLA-R1-F3、CLA-R1-F4、CLA-R1-F5 CLOSED；新增 CLA-R2-F1（低）。

残余不确定（不计为发现）：
- 有旧值时恢复写回的是紧凑格式，语义相同但不是逐字节还原；`ux/settings-quota.md:309-310` 把
  "是否逐字节还原"列为人工验收项。
- 状态栏载荷 `used_percentage` 的 0-100 假设仍未用真实载荷核实。

Reviewed state：branch `feature/subscription-quota`，HEAD
`e31a7715c5d46e5f3f55a968a80eb1c3294a1fa2`；与 Round 1 相同的 12 路径 manifest，SHA-256
`962ed8a4ce4869869886671cd8a7b0fe8bd4eec0328b3bd2c352211ba1d2075f`。与 Round 1 相比变更的 blob：
`config.go` `d487d366aafbdb9a26499d5dac84a9d14efcbbe3`、
`config_test.go` `03f870e89cd4f387fed2bdea6b7996a6d1181668`、
`claude.go` `32dd0542d3df323fc863808faf81a9d3595c1509`、
`claude_test.go` `299340c437b6ae0f683b00da1d67264ebe62c21a`、
`statusline.go` `c1b2e3328feafb882abdf6f8bd146e9fb910317c`、
`statusline_test.go` `4e6875e57c0c5e9a09b9a52bcb238dc8097b2b71`、
`quota_capture.go` `ef1447fa6522cb65bfdff3a943ff2ffbb9567294`、
`quota_capture_test.go` `4173e5e0cbd9b5b576908886eea30f2b8b10f303`，以及 `tasks.md`。
`main.go`、`main_test.go` 与 GUI JSON 契约 fixture 本轮未变。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，0 个
  `--- FAIL`；日志 SHA-256 `229a4db1c7f6eb3c6a60db103504a32eaad2114fbcd4405c708ceeee09339988`。
- 复现 A（`usagehook`）：`-overlay <scratchpad>/overlay-cla-r2a.json -run 'TestProbeCLAR2' -v
  ./internal/usagehook/`，exit 0；日志 SHA-256
  `2b6dbbb69e59ff9fd6ed866ebecb07586ccdf551d9b1e1de13bcb7a4921abc2a`；probe SHA-256
  `a4586ac7f664d40a84dd5e14e4fa9df8846d4eb7d68a3750d9a85f2118f674ea`。
- 复现 B（`cmd/agentdeck`，在全仓测试结束后无负载运行）：`-overlay <scratchpad>/overlay-cla-r2b.json
  -run 'TestProbeCLAR2' -v ./cmd/agentdeck/`，exit 0；日志 SHA-256
  `46877c6643e82eedd8f5e80e1f3a66239220e6db97b2e7915224787151ce6672`；probe SHA-256
  `9bcf6b13516c15a9b022fb03a9e452d4b77fe68a68861e251a3125fa6fa058c7`。
- 本轮未单独复核 vet/gofmt。

完成门禁：FAILED。内容状态
`urn:agent-deck:content-state:subscription-quota:claude-adapters:e31a771:962ed8a4ce48`；
`verification` 记为 pass（全仓测试通过，Round 1 每项发现都有对应回归测试且断言正确行为），
`adapter-contract` 与 `review` 记为 fail。

Task 状态：Dev 保留勾选，Review 未勾选；Beads `ad-sq-claude-adapters-dev` 退回 `in_progress`。
本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/claude-adapters.md / CLA-R2-F1

## Round 3 — 2026-09-11

## 📋 claude-adapters 第二次修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **CLA-R2-F1 → CLOSED。** 采用上一轮给出的第一种修法：上限重新只约束捕获自身的解析。
  - `cmd/agentdeck/quota_capture.go:75-77` 在 `ChainStatusLine` 返回之后、开库之前检查
    `len(payload) > quota.MaxStatusLinePayloadBytes`，超限则跳过本轮记录，连库都不开；
  - `internal/quota/statusline.go:13-21` 的注释改为描述"只约束 `RecordStatusLinePayload` 的解析"，
    不再提及已被拆除的 `CaptureStatusLine`；`quota_capture.go:50-59` 的注释与之一致。
  - 复现 `TestProbeCLAR3OversizedPayloadChainsFullyButSkipsCapture`：正常载荷 134 字节时，原命令
    收到 134 字节且写入 2 个窗口；超限载荷 65670 字节时，原命令同样收到完整的 65670 字节，写入
    窗口数为 0。新增回归测试
    `TestRunQuotaCaptureSkipsPersistenceOnceStdinExceedsSizeLimitButStillChainsFullPayload`
    用不影响解析的前导空白构造超限载荷，从而把"因超限跳过"与"因解析失败跳过"分开，并同时断言
    这两条性质。
- Round 1 的五项与 Round 2 的一项均保持关闭；本轮改动只涉及 `statusline.go` 注释、
  `quota_capture.go` 的尺寸检查与注释，以及新增测试。
- `internal/quota/statusline_test.go:114-117` 仍出现 `CaptureStatusLine`，但那是描述 CLA-R1-F3
  拆分历史的小节标题，陈述的是"当初拆成了哪两个函数"，不是对当前行为的错误声明，不记为发现。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式复评——核对
CLA-R2-F1 的修法与两处注释，审读新增回归测试，并用 Go `-overlay` 注入独立复现，同时覆盖限内与
超限两种载荷。未使用子代理，未修改生产代码、仓库测试或配置。

逐项处置：CLA-R2-F1 CLOSED；CLA-R1-F1 至 CLA-R1-F5 仍为 CLOSED。本记录中没有未关闭的发现。

残余不确定（与前两轮相同，不计为发现）：有旧值时恢复写回的是紧凑格式，语义相同但非逐字节还原，
`ux/settings-quota.md:309-310` 将其列为人工验收项；状态栏载荷 `used_percentage` 的 0-100 假设仍未用
真实载荷核实。

Reviewed state：branch `feature/subscription-quota`，HEAD
`e31a7715c5d46e5f3f55a968a80eb1c3294a1fa2`；与前两轮相同的 12 路径 manifest，SHA-256
`e94cd0df713719aa18cac275a872c398fbc106ddf53ccc204ac4ed1125e74b13`。与 Round 2 相比变更的 blob：
`statusline.go` `5ba6a0de9e58106da0aa846a28a09dc17d36fec2`、
`quota_capture.go` `509f3a46a18be0d5ce7292c8600bfd0fa8d84d94`、
`quota_capture_test.go` `fe21dd10b9ac79bab094724978c52888e6ecb387`，以及 `tasks.md`。复现与全仓测试
运行时，只有 `tasks.md`（Review 勾选与交接文字）与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，0 个
  `--- FAIL`；日志 SHA-256 `e4396578e596d1a08fb5d793d6f9942a9ef99709d7f11eb48a7364bae7e8d22d`。
- `go vet -mod=vendor ./internal/quota/ ./internal/usagehook/ ./cmd/agentdeck/` 通过；
  `gofmt -l` 只列出 `cmd/agentdeck/usage_stats_viewer_test.go`，该文件与 HEAD 相同，属既有无关偏差。
- 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-cla-r3.json -run 'TestProbeCLAR3'
  -v ./cmd/agentdeck/`，exit 0；probe SHA-256
  `6d97990b84416579ff1d410957a58d19c132aaed1d428018ab8980cd8a8c3d27`，日志 SHA-256
  `fb310c57d3ab9748bb2e4f58f7f4a8f1ff393a12c2555e9bd30dbd71d8336458`。材料保存在本会话 scratchpad。

完成门禁：VERIFIED。WorkUnit `urn:agent-deck:work-unit:subscription-quota:claude-adapters` 的目标为
`urn:agent-deck:content-state:subscription-quota:claude-adapters:e31a771:e94cd0df7137`；
`adapter-contract`、`verification`、`review` 三项均有绑定此状态的 pass 证据。

Task 状态：Dev 与 Review 均已勾选；Beads `ad-sq-claude-adapters-dev` 流转为 `awaiting_commit`。
本轮没有 commit 或 push。

Task checkpoint：`ad-sq-claude-adapters-dev`，content state
`urn:agent-deck:content-state:subscription-quota:claude-adapters:e31a771:e94cd0df7137`
（manifest `e94cd0df…4b13`），门禁 VERIFIED。

提交建议：只提交任务 3 的交付范围，即 `internal/quota/{claude,claude_test,statusline,statusline_test}.go`、
`internal/usagehook/{config,config_test}.go`、
`cmd/agentdeck/{quota_capture,quota_capture_test,main,main_test}.go`、
`cmd/agentdeck/testdata/phase7/gui-json-contract.json`、
`docs/topics/subscription-quota/tasks.md` 与本评审记录；暂存前核对 blob 与上方 manifest 一致，
不要把 `cmd/agentdeck/usage_stats_viewer_test.go` 的既有 gofmt 偏差带进去。

推送建议：`feature/subscription-quota` 仍没有上游，`origin` 上也没有同名分支；提交后若需备份或协作，
可推送为 `origin/feature/subscription-quota`。合并到 `main` 属于 `v0-6-0-contract` 的 `assemble` 任务。
提交与推送都需要单独授权。

下一步指令：开发：subscription-quota / gate-and-schedule
