---
status: active
topic: installed-state-ownership
subject: state-root-guard
---

# state-root-guard — Review

## Round 1 — 2026-11-07

### 📋 独立实现评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-SRG-R1-F1 — P2：worker 接收阶段的所有权拒绝在 socket 边界丢失类型

- 位置：`internal/scanruntime/scanruntime.go:666-670`（新增接收阶段复核）、`:597-600`（拒绝响应）、`:294-296`（客户端恢复错误）。
- 状态：OPEN。
- 行为风险：客户端通过自身状态守卫后，worker 在接收请求时发现安装身份失效，例如旧 keg 可执行文件在此间被删除。`accept` 正确返回带 `Revoked: true` 的 `ErrStateOwnershipRefused`，但 `handle` 只发送 `err.Error()`，客户端用 `errors.New(first.Error)` 重建。结果不再满足 `errors.As`；CLI 的 `errorCode` 落入 `runtime_error`，`errorDetails` 丢失 resource/reason，文本也无法选择已批准的 revoked 恢复提示。这违反 Task 2 的长驻复核和结构化拒绝合同；拒绝本身仍然阻止本轮扫描，不是已经证实的状态访问绕过。
- 证据：隔离 Unix socket 诊断执行真实 `Client.Request` → `server.handle` → `accept`。客户端 `prepareStateRoot` 成功后，在服务端 `AcceptUnix` 返回与 `handle` 开始之间，将测试 Resolver 从已安装切换为 `executable_unresolved`。服务端使用临时账户、真实共享 flock 和真实 receipt journal；不访问操作员账户。客户端观察为：`error="This AgentDeck is not the installed program and cannot use the real state directory."`，`errors.As(error, *platform.ErrStateOwnershipRefused)=false`。错误码落点由 `cmd/agentdeck/main.go:659-721` 核实，details/文本分派由同文件 `errorDetails`、`renderCommandErrorText` 核实。
- 测试保护缺口：`internal/scanruntime/state_root_guard_test.go:12-32` 只 Marshal/Unmarshal `Result` 并调用 `ErrorFor`；接收拒绝没有 `Result`，因此这九个 reason/scope 组合不能覆盖实际缺陷。
- 💡 修复建议：为 worker 的拒绝响应传输可选的结构化所有权错误，在客户端还原 resource/reason/revoked；保持其他拒绝的既有协议行为。补充实际 Unix socket 接收拒绝回归及 CLI 映射断言，至少覆盖客户端已过守卫、服务端接收前失效的顺序。不要只增加 `Result` 序列化断言；不需要扩展安装器任务。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的方面

- 默认目录解析与显式真实根共用续登能力；Guard 返回物理路径，库边界默认拒绝未安装身份。
- 共享所有权会话覆盖只读池与进程生命期，开发材料列出了 80 个命令的三分支矩阵、62 个包入口及八种交错；屏障夹具包含正负对照，不能以 skip 代替运行。
- 开发 ContentState 的 81 个 subject、71 个变更路径与当前候选匹配，已有验证可按原内容状态复用。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra，独立评审角色；session `ses_e49d7f8e4ffeQOvNK2niDBXulY`。未参与本候选实现，无委派。
- Method：已批准 Task 2、架构合同与当前差异的静态追踪，工作区绑定的 CodeGraph 调用关系，针对未索引新增文件的源代码检查，以及临时 overlay Unix socket 诊断。产品、仓库测试和配置保持只读。
- Scope：`installed-state-ownership / state-root-guard`。重点检查状态解析、共享锁生命期、包入口的物理路径消费、CLI/doctor/Hook 分派和长驻 worker 复核。发现决定性阻塞反例后停止扩大验证；不把此轮记作全部实现已通过语义评审。
- Reviewed state：HEAD `c2484dee09405f578bb39037a6c5eac717ec8ad2`；开发目标 `installed-state-ownership:state-root-guard:dev-20261107-Gas5YUA6`；81 路径配方 SHA-256 `7aa794e97f86b9a7ffc792f68d66b19d0c66dfdd1d68d7ad2ca4eba1d8950fb4`。入口时用 `git hash-object` 按交接配方逐路径计算，结果精确一致。评审后仅 tasks.md 的状态摘要及本记录变化，产品候选未变。
- Evidence：入口标准 CEv1 gate 为 NOT_VERIFIED，十项 required criteria 中九项有有效开发证据，唯一缺项 independent-review；invalidated/unresolved 均为空。复用开发的 L3、命令矩阵、访问屏障和交错结果作为其原观察，不重跑全套。
- 新增诊断：通过规定 runner 加 Go `-overlay` 注入临时同包诊断，不写仓库测试。定向命令形状为 `scripts/run-go-test.sh -overlay <临时映射.json> ./internal/scanruntime -run '^TestReviewWorkerOwnershipTransport$/^before-accept$'`，exit 0，断言成功复现类型丢失。可复现步骤及期望错误详见 F1；临时源和日志不是持久引用。扩展探索中的 before-terminal 假设未复现（该诊断收到 typed refusal），不将它列为第二个发现，也不声称证明终态路径正确。
- 验证边界：这是合成账户和可控 Resolver 的错误传输诊断，不是新增的访问屏障或真实安装器验收。原开发材料披露的原生验证器 schema-pair 基线问题保持其原有归属。
- Completion gate：FAILED。最终状态 `installed-state-ownership:state-root-guard:review-r1-20261107-2aeddd37`，81 路径摘要 `2aeddd37863219ba2ef377b55f07b99ad671e275c3e558e91fd0b18191f339b0`；相对开发状态仅 tasks.md 摘要改变，blob 为 `fa2ade6a5c445b6245fd6700e4f6ff2a703bd3e8`。已写入并读回两个节点、两条通过 preflight 的关系；标准 gate 返回 applicable independent-review fail，invalidated/unresolved 为空。九项开发观察保留原目标，不为已失败的新边界制造 passing roll-up；最终状态因此亦列出九项尚未 target-bound 的开发 criteria，修复后按实际影响复用。
- 文档验证：评审记录结构检查、`make check-whitespace`、`git diff --check` 通过。`python3 scripts/check-docs.py` 被既有 `docs/topics/health-recovery/reviews/extension-inventory-recovery.md` 的 indented list continuation 链接格式阻断，不声称聚合文档检查通过，未修改该外部专题。
- 结论：一个 P2 合同缺陷仍开放，Review 保持未勾选；任务返回修复，不进入提交边界。

### 下一步指令

修复：installed-state-ownership / state-root-guard / ISO-SRG-R1-F1

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Repair 1 — 2026-11-07

- Repairer：Codex；correlation `WxkADPnBdDDIRtTu`。真实用户仅授权修复 `ISO-SRG-R1-F1`；未委派、未提交、未推送。
- 接收状态：HEAD `c2484dee09405f578bb39037a6c5eac717ec8ad2`，81 个 subject 的逐路径 blob 与 Round 1 最终候选精确一致，SHA-256 `2aeddd37863219ba2ef377b55f07b99ad671e275c3e558e91fd0b18191f339b0`。原评审者已释放任务；通过 `in_progress` 与空负责人条件领取，保留既有产品候选。
- `ISO-SRG-R1-F1`：已实施修复，待独立复评确认。`wireResponse` 新增可选 `ownership_refusal`；`server.handle` 使用 `errors.As` 提取拒绝类型，同时保留既有 error 字符串；`Client.request` 在接收拒绝时优先恢复该类型。普通拒绝、正常 accepted/terminal 协议及旧 `Result` 序列化没有改变。
- 回归反证：`TestWorkerAdmissionOwnershipRefusal` 在真实 `Client.Request` → Unix socket → `server.handle` → `accept` 链上，对客户端守卫与服务端接收复核之间施加确定性身份切换。修复前 18 例均得到 `*errors.errorString` 并失败；修复后 18 例均保留 resource/reason/revoked。覆盖 executable_unresolved、receipt_other_executable、ownership_busy，usage/session/both，以及普通 decoder 和 serving reader。每例核验零扫描、无 round ID、真实 receipt journal 未变；有效拥有者和普通拒绝两项对照通过。测试使用进程已有的合成账户、真实共享 flock 与临时状态，不访问操作员数据，不宣称新增 OS 访问屏障验收。
- CLI 保护：新增 `TestWorkerOwnershipRefusalCLIContract` 的三种原因断言，核验包装错误仍映射 `state_ownership_refused`、exit 1、JSON details 严格为 resource/reason，并与已有 revoked 标本逐字比较恢复提示。实际 socket 类型传递由上一个回归证明；此 CLI 测试验证类型恢复后的映射，不冒充另一次端到端 CLI socket 验收。
- 验证：Go 1.27.3、vendor 模式、macOS 26.7.3。`scripts/run-go-test.sh ./internal/scanruntime ./cmd/agentdeck -run 'Test(WorkerAdmissionOwnership|WorkerOwnershipRefusalCLIContract|OwnershipRefusalSurvivesWorkerResultWire)'`、`scripts/run-go-test.sh ./...`（28 包）、`make vet build-all` 通过。受影响 scanruntime 包全量 `-race` 通过（170.509s），新增 CLI 回归也在 `-race` 下通过。
- 额外 CLI 竞态检查限制：`scripts/run-go-test.sh -race ./internal/scanruntime ./cmd/agentdeck` 的 cmd/agentdeck 包累计运行至默认 10 分钟上限，超时时 `TestSnapshotPerformanceContractSyntheticCorpus` 刚开始 3 秒，日志没有 data-race 报告。保持相同工具链、配置和超时，只将选择缩小为 `scripts/run-go-test.sh -race ./cmd/agentdeck -run '^TestSnapshotPerformanceContractSyntheticCorpus$'`，用例 5.56s、包 7.908s 通过。最早已证实的失败是包级时间预算耗尽；未修改产品、测试断言或超时来处理它，也不声称 CLI 全包竞态通过。本次新增 CLI 测试在原聚合运行中已通过，复用该观察。
- 文档：修复范围的 `check-docs.py` / `scoped_errors` 检查覆盖两个更新的 Markdown、三个 Go 修复路径的入链以及完整专题文档集；结构检查、`make check-whitespace`、`git diff --check` 通过。全工作区聚合仍报告既有 health-recovery 历史链接格式，文件 blob `1f08472758796aa4ea41f2ecf9c03f04c2c69409` 与 HEAD 完全相同；归属 `ad-bug-doc-check-historical-file-url-scope`，不宣称聚合通过。
- 证据边界：只改变拒绝响应构造与还原，以及两个回归测试文件。状态解析、包访问守卫、会话、续登、访问屏障与八种交错所验证的行为未受此次传输修改影响；revalidation 和 CLI 观察保留未受影响部分，并补充本次定向结果。此前 platform/store 等未修改并发包的竞态观察按其原输入复用。原生 schema-pair 限制沿用 `ad-bug-native-verifier-schema-pair-stale`，未扩展处理。
- 修复候选：`installed-state-ownership:state-root-guard:repair-r1-f1-20261107-WxkADPnB`；沿用原指纹配方并纳入新增 CLI 回归文件，共 82 个 subject，SHA-256 `27b2d4a8158e15386001677829a6e40a715c56fd014d95cd354ac929f1b77fd2`。独立复评尚未完成，Task 边界保持开放；Round 1 FAIL 与其原始证据保留，本节不出具独立 PASS。

### 下一步指令

复评：installed-state-ownership / state-root-guard / ISO-SRG-R1-F1

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Round 2 — 2026-11-07

### 📋 限定发现独立复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。ISO-SRG-R1-F1 已关闭；本轮没有仍开放、回归或新阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的方面

- `ISO-SRG-R1-F1 CLOSED`：`wireResponse` 的可选 `ownership_refusal` 使用已有、无路径信息的 `ErrStateOwnershipRefused`。服务端在接收拒绝分支通过 `errors.As` 提取类型，保留原 error 字符串；客户端在未获 accepted 响应时优先返回该类型。resource、reason、revoked 均穿过实际 socket 编解码，CLI 的 `errors.As` 分派因而能恢复错误码、details 和 revoked 提示。位置：`internal/scanruntime/scanruntime.go:225-233,295-302,601-606`；`internal/platform/state_ownership.go:11-18,122-127`。
- 新回归在客户端 `prepareStateRoot` 完成、连接建立后，由 `AcceptUnix` 屏障在 `server.handle` 前切换身份。三种 reason、三个 scope、普通与 serving 两种 reader 的 18 例都要求具体拒绝字段，并同时要求零轮次、空 round ID、receipt journal 未变；有效拥有者与普通拒绝是两项对照。这里证明拒绝的传输与未启动工作，不把字节不变断言解释成 OS 访问屏障。位置：`internal/scanruntime/state_root_guard_test.go:57-194`。
- CLI 三种包装错误断言覆盖 `state_ownership_refused`、exit 1、严格的 resource/reason details 和现有 revoked golden。对应分派源码位于 `cmd/agentdeck/main.go:499-503,570-574,659-663,724-729` 与 `cmd/agentdeck/ownership.go:56-64`；测试位于 `cmd/agentdeck/ownership_error_test.go:15-52`。这与真实 socket 回归组成分层证据，不声称额外执行了端到端 CLI socket 验收。

### 📝 总结

- Reviewer：Codex，本会话独立复评角色，correlation `c6x7hrej31uyKzAQ`；未参与原实现或 Repair 1，无委派。
- Method：核对原发现、批准的 Task 2 合同与修复交接；工作区绑定的 CodeGraph 调用链和当前源码检查；逐路径内容身份核验；检查回归的顺序屏障、类型/字段断言、隔离及对照；复用精确状态的既有工程证据。产品、测试、配置保持只读。
- Scope：只复评 `installed-state-ownership / state-root-guard / ISO-SRG-R1-F1` 及其直接传输、CLI 映射与回归保护。此次 PASS 关闭本记录唯一发现；不是对后续安装器、客户端配置、App 或跨面验收的认可，也不宣称重新开展了 Task 2 全面审计。
- 接收候选：HEAD `c2484dee09405f578bb39037a6c5eac717ec8ad2`；修复 ContentState `installed-state-ownership:state-root-guard:repair-r1-f1-20261107-WxkADPnB`；82 个 subject 的实际 Git blob 和 SHA-256 `27b2d4a8158e15386001677829a6e40a715c56fd014d95cd354ac929f1b77fd2` 与 CEv1 完全一致。Go `1.27.3 darwin/amd64`、macOS `26.7.3 (25G416)` 与修复观察一致。
- 修复范围核验：Round 1 与 Repair 1 的 subject 映射仅有 `scanruntime.go`、`state_root_guard_test.go`、新增 `ownership_error_test.go` 及 `tasks.md` 改变，无删除。Round 1 的两个未提交源码 blob 未保存在 Git 对象库，故没有声称恢复逐行旧差异；通过当前相关源码、原发现的明确位置和精确绑定的修复验证判定关闭。
- 传输核验：普通 `json.Decoder` 和 serving 的逐行 `json.Unmarshal` 均解入同一 `wireResponse`；接收复核在扫描/receipt 接受之前，失败直接返回。可选新字段不改变普通拒绝、accepted/progress/publication/terminal 的既有分支。资源与原因字段不含账户路径。
- Evidence：入口标准 `gate-status.cypher` 为 NOT_VERIFIED，十项 required criteria 中九项具有效、target-bound 的 passing evidence；唯一缺项 independent-review，invalidated/unresolved 均为空。按当前 82 路径和工具链复用 Repair 1 的 18 个 socket 例、两项对照、三项 CLI 映射、28 包 Go 回归、scanruntime 完整竞态、vet、双架构构建，以及已明确评估未受影响的会话、屏障与交错证据；本轮未重复这些已有效的产品检查。
- Reviewed state：状态摘要与 Review 矩阵同步后的目标为 `installed-state-ownership:state-root-guard:rereview-r2-20261107-1664b463`；沿用相同 82 路径配方，SHA-256 `1664b46398a269f55a94a65973d31440559511a0c3cdd45d727b9ef547a2c001`，HEAD 不变。相对 Repair 1 仅 `tasks.md` blob 变为 `60c100315cab2493eb12e5afdc05a8ae4da7a9db`；其余 81 个 subject 均未变。评审记录按既有配方排除，不递归嵌入自身指纹。
- 残余限制：Repair 1 的 CLI 全包 `-race` 达到默认 10 分钟包预算，不能记作通过；新增 CLI 回归已在该运行通过，超时时用例另行通过，scanruntime 全包竞态通过。既有聚合文档历史链接问题 `ad-bug-doc-check-historical-file-url-scope` 和原生 schema-pair 问题 `ad-bug-native-verifier-schema-pair-stale` 保持原归属；本轮没有扩大处理或为它们制造通过证据。
- 文档验证：本轮新增记录和状态摘要的 `check-docs.py / scoped_errors`、完整专题文档集及入链检查通过；`make check-whitespace`、`git diff --check` 通过。调用 scoped checker 时首次缺少 `scripts` 模块路径，补齐 `PYTHONPATH` 后该项通过；这不是文档或产品失败。门禁字段回填后运行 `python3 scripts/check-review-records.py --base HEAD docs/topics/installed-state-ownership/reviews/state-root-guard.md` 核对最终记录结构。
- Completion gate：VERIFIED。最终目标 `installed-state-ownership:state-root-guard:rereview-r2-20261107-1664b463` 的标准 `gate-status.cypher` 返回 10/10 required criteria 通过；missing、invalidated、unresolved 均为空。通过固定模板新增并精确读回 21 个节点；47 条关系全部 preflight 为 ok，写入与端点读回数量、内容一致。九项工程证据使用显式 status-only impact assessment 和 target-bound roll-up；本轮独立复评是新观察。Round 1 FAIL 及原始观察保留。
- 结论：原发现所述类型退化路径已被关闭，未发现该修复直接引入的阻塞问题。Task 2 的 Review 已勾选，尚无 Git 交付；任务 3–9 仍未完成，不跨越整个 Topic 的完成边界。

Task checkpoint：`ad-installed-state-ownership-state-root-guard`；content_state `1664b46398a269f55a94a65973d31440559511a0c3cdd45d727b9ef547a2c001`；gate VERIFIED。
提交建议：单独提交 Task 2 的现有实现、测试、状态摘要及本评审记录；提交前按项目规则核验暂存范围、贡献者、完整消息与 SSH 签名。此建议不授权执行提交。
推送建议：目标为 `origin/feature/installed-state-ownership`，当前分支未配置 upstream；Task 2 独立提交后，核验全部待推提交、签名、远端和上游差异，并取得单独推送授权。

### 下一步指令

任务 3 消费已交付的任务 2；先经单独授权完成上述提交检查点及 Beads 交付关闭，再执行：

开发：installed-state-ownership / client-config-guard

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

