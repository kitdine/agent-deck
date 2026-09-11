---
status: active
topic: subscription-quota
subject: codex-adapter
---

# Codex Adapter Review

## Round 1 — 2026-09-11

## 📋 codex-adapter 实现评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**CA-R1-F1 — 高：按非协议形状解析响应，真实响应的 plan、label、billing 全部丢失，fixture 也是这个错误形状。**

位置：`internal/quota/codex.go:212-218` 在顶层读取 `planType` 与 `credits`；
`internal/quota/codex.go:240-245` 在窗口对象里读取 `limitName`；
`internal/quota/codex.go:23-29` 的 `CodexResult` 没有 Billing 字段；
`internal/quota/codex.go:330-332` 在 `rateLimitsByLimitId` 缺失或为 null 时直接返回零窗口；
`internal/quota/testdata/codex_rate_limits.json` 把 `planType`、`credits` 放在顶层、`balance` 写成数字、
`limitName` 放进窗口。

- 行为风险：本机 `codex-cli 0.154.0` 用 `codex app-server generate-json-schema` 生成的
  `GetAccountRateLimitsResponse` 声明：顶层唯一必填的是 `rateLimits`（一个
  `RateLimitSnapshot`），`rateLimitsByLimitId` 是 `RateLimitSnapshot` 的映射；`planType`、
  `credits`、`limitName` 都是快照的字段，窗口（`RateLimitWindow`）只有 `usedPercent`、
  `windowDurationMins`、`resetsAt`；`credits.balance` 是 `string | null`。按此形状，
  每个真实响应都会得到：
  - `plan` 为空、原因 `not_reported`——C2 说 `planType` 映射为 `plan`，需求记录的样本
    有值 `prolite`（`requirements.md:71`）；
  - 所有窗口 label 为空——C11 要求按模型的限额行靠 label 区分（`architecture.md:521-524`）；
  - `credits` 映射为 `billing`（`architecture.md:107`）完全没有实现；
  - 旧后端或无分桶视图时 `rateLimitsByLimitId` 为 null，协议必填的 `rateLimits`
    带着主限额，结果却是零窗口，界面会显示为"未报告"。
  现有测试全部通过，因为 fixture 采用的正是同一个错误形状；架构要求 fixture 取自
  2026-09-08 记录的真实响应（`architecture.md:580-582`），仓库里没有原始响应，这份
  fixture 是按需求表格手工拼出来的。
- 证据：
  - schema：`GetAccountRateLimitsResponse.json` SHA-256
    `76bc91758269a89f57cd16c618b91c1fba76aca3e9d2b6186205e6f107d6b28c`（codex-cli 0.154.0；
    需求记录时为 0.153.4）。
  - 复现 `TestProbeCAParseSchemaShapedResponse`：按 schema 形状的响应，输出
    `plan="" planReason="not_reported" windows=[codex codex_bengalfox codex_bengalfox_secondary] labels=["" "" ""]`，
    `balance="12.50"` 无处承载。
  - 复现 `TestProbeCAParseRateLimitsWithoutByLimitID`：`rateLimits` 有主限额、
    `rateLimitsByLimitId` 为 null，输出 `windows=0 plan=""`。
  - 通过假 `codex` 走完整握手的成功场景，同样得到 `windows=3 plan=""`。

💡 修复范围：按协议形状解码——`plan` 与 `billing` 取自必填的 `rateLimits` 快照，
`balance` 按十进制字符串解析（缺失或无法解析时 `HasBalance=false`），各窗口的 label 取自
所属快照的 `limitName`；`CodexResult` 增加 Billing。把 fixture 改成协议形状（有真实抓取时
优先使用，账号 ID 替换），并断言 plan、label、billing。`rateLimitsByLimitId` 为 null 而
`rateLimits` 存在时是否把 `rateLimits` 按其 `limitId` 映射为窗口，C2 没有写明；修复方应先
停下来交由操作者决定，不得自行扩充契约。

**CA-R1-F2 — 中：传输层失败分类与 C2 不符，且没有任何测试覆盖。**

位置：`internal/quota/codex.go:186-202` 的 `readJSONRPCResponse` 跳过无法解析的行；
`internal/quota/codex.go:148-159` 把 JSON-RPC error 响应的空 `result` 交给解析，得到
`malformed`；`internal/quota/codex.go:133-146` 把进程启动后的写入失败归为 `spawn`；
`internal/quota/codex.go:138`、`:150` 在读取没有出错时仍用 `%w` 包装 nil；
`internal/quota/codex_test.go:133-155` 只测解析函数和 Kind→Reason 映射。

- 行为风险：C2 要求 spawn 失败、非零退出、超时、malformed JSON、缺少 `rateLimits`
  是互相区分的结果（`architecture.md:135-138`），并分别映射到封闭原因集。现状是：
  - app-server 发出截断的 JSON 时，行被跳过，结果被报成 `exit` 或 `deadline`
    （`probe_failed`），`parse_failed` 这条路径在真实传输中永远到达不了——信封解码
    已经保证了 `result` 是合法 JSON；
  - 服务端明确返回错误（例如未登录）时反而报成 `malformed/parse_failed`，界面会说
    "无法读取输出"，而实际是探测失败；
  - 进程已启动、写入因进程退出而失败时报成 `spawn`；
  - 错误文本出现 `%!w(<nil>)`。
  `ProbeCodex` 没有任何测试；"不启动真实 codex"并不妨碍用注入的假进程验证分类。
- 证据：复现（假 `codex` 脚本放在临时目录并置于 PATH 首位，未启动真实 codex）：
  - 截断的 id 2 响应后退出：`kind=exit reason=probe_failed`；
  - 截断的 id 2 响应后保持运行：`kind=deadline reason=probe_failed`；
  - id 2 返回 JSON-RPC error：`kind=malformed reason=parse_failed err=... unexpected end of JSON input`；
  - 各失败场景的错误文本都含 `%!w(<nil>)`。

💡 修复范围：等待响应时遇到无法解码的行判为 malformed；带 `error` 成员的响应判为
probe_failed；启动后的写入失败判为进程退出；读取无错误时不包装 nil。为命令构造提供
包内可替换的注入点，用假进程测试握手、id 匹配和上述每种分类，不启动真实 codex。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- 握手顺序、保持 stdin 直到匹配 id、进程退出不当作空答复，都与 C2 一致；超时场景
  （从不应答）在截止时间内正确报为 `deadline`。
- 用 token 解码保留 `rateLimitsByLimitId` 的键顺序，并据此赋 `VendorOrder`，满足 C11；
  window_key 复用任务 1 的 `CodexWindowKey`。
- 拒绝的字段（`spendControlReached`、`rateLimitReachedType`、`individualLimit`、
  `rateLimitUpsell`、credit 的 `id` 与 `description`）在类型上就不存在，无法泄漏；
  `reset_allowance.total` 固定为 `not_reported`；`plan` 没有任何分支读取。
- 本任务没有改动命令、desktop wire、调度、告警或 Swift 文件。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与实现。Method：单代理正式代码
评审——逐条对照 `tasks.md` 任务 2 与 `architecture.md` C2、C11；用本机 codex 生成的
app-server 协议 schema 核对响应形状（只生成 schema，不读取账号、不联网）；用 Go
`-overlay` 注入复现测试，其中传输层场景使用临时目录中的假 `codex` 脚本。未使用子代理，
未修改生产代码、仓库测试或配置。

Scope：`internal/quota/codex.go`、`codex_test.go`、`internal/quota/testdata/` 下 4 个
fixture，以及它们与任务 1 domain 类型的衔接。

排除的疑点：
- 截止时间杀进程后子进程继续占用 stdout 导致卡死：本机 codex 是原生 Mach-O 程序
  （`/usr/local/bin/codex` → Caskroom 0.154.0），不是会再派生子进程的脚本启动器，前提不成立。
- 成功应答后 `cmd.Wait()` 等满超时：只在服务端不理会 stdin EOF 时出现；需求记录显示
  app-server 在 stdin 关闭后结束（`requirements.md:61-63`），不记为发现。

残余不确定：
- 桌面 helper 调用 `ProbeCodex` 时以 PATH 查找 `codex`；DEBUG 验收分支把 PATH 设为
  `/usr/bin:/bin`（`apps/macos/AgentDeckApp/AgentDeckApp.swift:57-62`），生产环境的 helper
  环境变量来源本轮没有追到。可执行文件定位属于任务 4 接入桌面时的问题，不计为本任务发现。
- schema 来自 0.154.0，需求记录的是 0.153.4；两者的形状差异无法回溯核实。

Reviewed state：branch `feature/subscription-quota`，HEAD
`972952e6726c4630915f61155733567a184a23df`。manifest 为 `head=<HEAD>` 一行加 7 个路径按字母
排序的 `<blob> <path>` 行（`codex.go`、`codex_test.go`、4 个 fixture、`tasks.md`），SHA-256
`7a88d2aba5222a1a8d9f04562406656d3221e05b3d255566aee73d8b80c32358`。关键 blob：
`codex.go` `f3a8c2d2b40f0f8aed5bba5ee6d73d9af16ad502`、
`codex_test.go` `9ac15804c43f0ef6023516744e71571bcc6464be`、
`codex_rate_limits.json` `8638a9ebc72ce183462f9466cc04938ad8a676f9`。复现与全仓测试运行时，
只有 `tasks.md` 的交接文字与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，
  0 个 `--- FAIL`；日志 SHA-256 `79b567ca4c855c15069f54e0713e0a009639cd1fd3be0cc0b03c9ee411daba10`。
  现有测试通过，但它们只验证错误形状的 fixture。
- 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-ca-r1.json -run 'TestProbeCA'
  -v ./internal/quota/`，exit 0；probe SHA-256
  `a7069aaf4e96d6c610a17b5075ed36013ad256521f4afe1c64046f3ae2b6587b`，日志 SHA-256
  `e43cb3194ab0f47de7e336ade857e2d0689b803cf2211ca1daa17e9a8c4eba64`。材料与生成的 schema
  保存在本会话 scratchpad，会话结束后可能被清理，可用同一命令重新生成。
- 未单独运行 vet/gofmt：实现交接声称通过，本轮未独立复核。

完成门禁：FAILED。进入评审时 CEv1 中没有本任务的 WorkUnit，已依据 `tasks.md` 任务 2
事先写明的内容建立 `urn:agent-deck:work-unit:subscription-quota:codex-adapter` 及三个必需标准
`adapter-contract`、`verification`、`review`；本轮内容状态为
`urn:agent-deck:content-state:subscription-quota:codex-adapter:972952e:7a88d2aba522`，三项均记为 fail。

Task 状态：Dev 保留实现方的勾选，Review 未勾选；Beads `ad-sq-codex-adapter-dev` 退回
`in_progress` 并释放认领。本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/codex-adapter.md / CA-R1-F1 CA-R1-F2

## Round 2 — 2026-09-11

## 📋 codex-adapter 修复复评

📊 总体评分：8/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**CA-R2-F1 — 中；新增：回退到 `rateLimits` 的契约决定在代码、注释和记录之间互相矛盾，也没有写进持久位置。**

位置：`internal/quota/codex.go:390-398` 只在 `rateLimitsByLimitId` 缺失或为 null 时回退；
`internal/quota/codex.go:21-26` 的 `CodexResult` 注释写"absent, null, or empty"都回退；
`internal/quota/codex.go:342-347` 的 `parseCodexResult` 注释写"absent, null, or empty"时不返回窗口；
`internal/quota/codex_test.go:177-186` 的空对象测试所用 fixture 的 `rateLimits` 没有窗口，分不出是否回退。

- 处置：新增，由 CA-R1-F1 的修复引入。
- 行为风险：修复会话报告操作者批准了"`rateLimitsByLimitId` 为 null 时回退到必填的 `rateLimits`，
  `limitId` 缺失时 window_key 用占位 `codex`"。这个决定改变了 C2 表格所写的窗口来源
  （`architecture.md:97`），并决定了任务 5 告警去重、任务 6 wire 使用的 window_key。
  但它的范围目前有四种说法：交接文字只说 null；修复的 Beads 评论与 `CodexResult` 注释说
  缺失、null、空都回退；`parseCodexResult` 注释说三种情况都不返回窗口；代码实际只对缺失和
  null 回退。后端若返回 `"rateLimitsByLimitId": {}` 而 `rateLimits` 带着主限额，结果是零窗口，
  界面显示为未报告。决定本身只存在于 Beads 评论和代码注释里；`tasks.md` 中唯一的提及是每轮
  都会改写的交接段（本轮已被改写），占位 `codex` 从未在仓库文档中出现。
- 证据：复现 `TestProbeCAR2FallbackCases`：
  - `byLimitId null, rateLimits has primary -> windows=[codex] plan="plus"`
  - `byLimitId absent, rateLimits has primary -> windows=[codex] plan="plus"`
  - `byLimitId empty object, rateLimits has primary -> windows=[] plan="plus"`
  - `rateLimits absent, byLimitId has codex -> windows=[codex] planReason="not_reported"`

💡 修复范围：先确定决定的范围——修复会话记录的来源是它自己那一轮的 AskUserQuestion，
本轮无法核实原话，是否包含空对象需由操作者确认。然后在 `tasks.md` 任务 2 小节写入一段
持久说明（参照任务 1 的 QD-R1-F5 说明），记录决定内容、日期、来源、适用范围与占位
window_key；让代码、两处注释与测试都与之一致，并补一个 `rateLimits` 带窗口、
`rateLimitsByLimitId` 为空对象的 fixture 测试。是否修订 `architecture.md` C2 属于该文档
自己的评审，不在本次修复范围内。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **CA-R1-F1 → CLOSED。** 响应类型按协议 schema 重建：`plan` 与 `billing` 取自必填的
  `rateLimits` 快照，`balance` 按字符串解析，label 取自所属快照的 `limitName`，`CodexResult`
  增加 Billing；6 个 fixture 都是协议形状并断言 plan、label、billing。重跑 Round 1 复现：
  按 schema 形状的响应得到 `plan="prolite"`，按模型限额的两个窗口 label 为
  `GPT-5.3-Codex-Spark`；`rateLimitsByLimitId` 为 null 时得到 1 个窗口、`plan="plus"`；通过假
  `codex` 走完整握手得到 `windows=3 plan="prolite"`。
- **CA-R1-F2 → CLOSED。** 无法解码的行立即判为 malformed，JSON-RPC error 新增 `server_error`
  并映射为 probe_failed，启动后的写入失败判为 exit，不再包装 nil。重跑复现：截断行后退出或
  保持运行都得到 `malformed/parse_failed`；error 响应得到 `server_error/probe_failed`；不应答得到
  `deadline`；提前退出得到 `exit`；错误文本不再含 `%!w(<nil>)`。`codexCommandArgs` 注入点
  配合 7 个假进程测试覆盖了握手与每种分类，没有启动真实 codex。
- 超时与 C2 的其他要求（stdin 保持打开、拒绝字段不承载、`total` 固定 `not_reported`、
  plan 不分支）保持不变。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式复评——
审读修复后的 `codex.go`、`codex_test.go` 与全部 6 个 fixture，对照上一轮生成的 codex-cli
0.154.0 协议 schema（SHA-256 `76bc9175…b28c`，未重新生成）；用 Go `-overlay` 原样重跑
Round 1 复现，并加跑 Round 2 回退复现。未使用子代理，未修改生产代码、仓库测试或配置。

逐项处置：CA-R1-F1、CA-R1-F2 CLOSED；新增 CA-R2-F1（中）。

复现中需要说明的一点：Round 1 复现里"只等 stdin EOF"的场景本轮 3 次分别得到 exit、deadline、
exit——假脚本启动约 700ms，与 1s 超时相互竞争，属于测试构造的时序，不是缺陷。

Reviewed state：branch `feature/subscription-quota`，HEAD
`972952e6726c4630915f61155733567a184a23df`；manifest 为 `head=<HEAD>` 一行加 9 个路径按字母排序的
`<blob> <path>` 行（`codex.go`、`codex_test.go`、6 个 fixture、`tasks.md`），SHA-256
`139a3402e6e962ae8cddbcfbeb39879bf35d76cfcc652a39144e21306d2f3d60`。关键 blob：
`codex.go` `5ddd112cceb719be05ebfed6aff9dd0b8143474b`、
`codex_test.go` `ef53b01252cf1c58f4ae3f54304caa2ac4771067`、
`codex_rate_limits.json` `7c498601d5a14a99d5b83c0fd6ef61692299de8d`。复现与全仓测试运行时，只有
`tasks.md` 的交接文字与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，
  0 个 `--- FAIL`；日志 SHA-256 `7121cb3217af25d51d3257a2a38dcb6323df936b39a9644fba031dc65ca73e71`。
- `go vet -mod=vendor ./internal/quota/` 通过；`gofmt -l internal/quota` 无输出。
- Round 1 复现重跑：日志 SHA-256
  `c9f09c193a3d7f6a2b8e689086756da9d92a0925c76d341ab2aee23a2fac5b89`。
- Round 2 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-ca-r2.json
  -run 'TestProbeCAR2' -v ./internal/quota/`，exit 0；probe SHA-256
  `8118504c0f2138328dee368332c3a4c355b3d3eaa8b9c6a728eb8f13f9704ce1`，日志 SHA-256
  `09c35238c9fc55978e0955d1a4f2fa3bec35eab14e28fb7e9439e4bf65750937`。材料保存在本会话 scratchpad。
- 残余不确定与 Round 1 相同：桌面 helper 的 PATH 能否找到 codex 属于任务 4；fixture 仍是按
  schema 构造而非真实抓取，仓库中没有原始响应。

完成门禁：FAILED。内容状态
`urn:agent-deck:content-state:subscription-quota:codex-adapter:972952e:139a3402e6e9`；
`verification` 记为 pass（目标测试与全仓测试通过、fixture 符合协议形状、分类有测试），
`adapter-contract` 与 `review` 记为 fail。

Task 状态：Dev 保留勾选，Review 未勾选；Beads `ad-sq-codex-adapter-dev` 退回 `in_progress`。
本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/codex-adapter.md / CA-R2-F1

## Round 3 — 2026-09-11

## 📋 codex-adapter 第二次修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **CA-R2-F1 → CLOSED。** 回退决定的范围现在只有一种说法：
  - `tasks.md` 任务 2 小节新增持久说明（`docs/topics/subscription-quota/tasks.md:176-203`），写明
    只有 `rateLimitsByLimitId` 缺失或为 null 时回退到 `rateLimits`，显式空对象 `{}` 不回退；
    `limitId` 缺失时 window_key 用占位 `codex`；注明日期、来源、对任务 5 与任务 6 的影响，以及
    修订 `architecture.md` C2 属于该文档自己的评审。
  - `CodexResult` 注释（`internal/quota/codex.go:21-30`）与 `parseCodexResult` 注释（`:346-352`）都
    改为同一范围；回退判断（`:395`）表达式未变。
  - `codex_rate_limits_empty.json` 现在在 `rateLimits.primary` 里带有数据，
    `TestParseCodexResultEmptyRateLimitsByLimitIDDoesNotFallBack` 断言零窗口，能区分"规则不回退"
    和"无物可回退"。
  - 复现 `TestProbeCAR2FallbackCases`：null 与缺失得到 `windows=[codex]`，空对象得到 `windows=[]`，
    缺少 `rateLimits` 得到 `planReason="not_reported"`，与记录的范围一致。
- CA-R1-F1、CA-R1-F2 保持关闭：Round 1 复现在无负载时重跑，结果与 Round 2 相同——按 schema 形状
  的响应得到 `plan="prolite"` 与按模型限额的 label；截断行得到 `malformed/parse_failed`；error 响应得到
  `server_error/probe_failed`；不应答得到 `deadline`；提前退出得到 `exit`；成功路径得到 3 个窗口。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式复评——审读
`tasks.md` 任务 2 新增说明、`codex.go` 注释与回退判断、空对象 fixture 与其测试；用 Go `-overlay` 同时重跑
Round 1 与 Round 2 复现。未使用子代理，未修改生产代码、仓库测试或配置。

逐项处置：CA-R2-F1 CLOSED；CA-R1-F1、CA-R1-F2 仍为 CLOSED。本记录中没有未关闭的发现。

复现过程说明：第一次与全仓测试并发运行时，传输层场景大多拖到超时并报 `deadline`；全仓测试结束后
在无负载下重跑，结果恢复为与 Round 2 一致。结合行号偏移只来自注释增长、回退判断表达式未变，判定
为负载造成的时序失真，不是回归。

残余不确定：
- 回退范围的来源是修复会话自己那一轮的 AskUserQuestion，本轮无法看到原话；`tasks.md` 已写明来源。
- 桌面 helper 的 PATH 能否找到 codex 属于任务 4；fixture 按协议 schema 构造，仓库中没有真实抓取的响应。

Reviewed state：branch `feature/subscription-quota`，HEAD
`972952e6726c4630915f61155733567a184a23df`；与 Round 2 相同的 9 路径 manifest，SHA-256
`efbdfca60ad0ef4bef12ed7fcff7a812fdd3f9841f2bfa4c1dd9db635a29c756`。与 Round 2 相比变更的 blob：
`codex.go` `083fbd42f62e740be63bfaf3f7b89b48fc36b199`、
`codex_test.go` `6d94683cd96978c214813d9bb4aaea4c2c22f6d1`、
`codex_rate_limits_empty.json` `674fc41075043f903b89083f9f414b4c7a4d3772`，以及 `tasks.md`。复现与全仓测试
运行时，只有 `tasks.md`（Review 勾选与交接文字）与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，21 个包 ok，0 个
  `--- FAIL`；日志 SHA-256 `43c7e3acd5331d5843ee8b2965d4de33906b11383a107c0bbb2105b28a4f1db7`。
- `go vet -mod=vendor ./internal/quota/` 通过；`gofmt -l internal/quota` 无输出。
- Round 1 与 Round 2 复现合并重跑（`overlay-ca-r3.json`）：日志 SHA-256
  `63ea440577645878f6063488fae9e8f1fd2c4119d95a01caf4e042a7c19be522`；无负载传输层重跑日志 SHA-256
  `3ea8606bbbf91dffd26316fcaa96241fe51e04ac350d1bba4297931e9aa0e1b6`。材料保存在本会话 scratchpad。

完成门禁：VERIFIED。WorkUnit `urn:agent-deck:work-unit:subscription-quota:codex-adapter` 的目标为
`urn:agent-deck:content-state:subscription-quota:codex-adapter:972952e:efbdfca60ad0`；`adapter-contract`、
`verification`、`review` 三项均有绑定此状态的 pass 证据。

Task 状态：Dev 与 Review 均已勾选；Beads `ad-sq-codex-adapter-dev` 流转为 `awaiting_commit`。本轮没有
commit 或 push。

Task checkpoint：`ad-sq-codex-adapter-dev`，content state
`urn:agent-deck:content-state:subscription-quota:codex-adapter:972952e:efbdfca60ad0`
（manifest `efbdfca6…c756`），门禁 VERIFIED。

提交建议：只提交任务 2 的交付范围，即 `internal/quota/codex.go`、`internal/quota/codex_test.go`、
`internal/quota/testdata/` 下 6 个 fixture、`docs/topics/subscription-quota/tasks.md` 与本评审记录；
暂存前核对 blob 与上方 manifest 一致。

推送建议：`feature/subscription-quota` 仍没有上游；提交后如需备份或协作，可推送为
`origin/feature/subscription-quota`。合并到 `main` 属于 `v0-6-0-contract` 的 `assemble` 任务。
提交与推送都需要单独授权。

下一步指令：开发：subscription-quota / claude-adapters
