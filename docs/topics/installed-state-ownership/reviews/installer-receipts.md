---
status: active
topic: installed-state-ownership
subject: installer-receipts
---

# Installer Receipts — Review

## Round 1 — 2026-11-10

## 📋 Task 5 独立评审

📊 综合评分：6/10

✅ 评审结论：FAIL

- Reviewer：Codex，本会话为独立评审角色，未参与候选实现，未委派。
- Method：核对批准任务与架构、CodeGraph 定位调用路径、检查实际源代码，并在临时账户中通过验收版真实 CLI 复现。生产代码、测试、配置保持只读。
- Scope：Task 5 的安装发布、attempt 清理、身份绑定修复与安装器接入。发现决定性阻断后停止扩大验证。
- Reviewed state：HEAD `88cfbfbbfe1df89d59fdd2302106b89ba465b43f`，43 路径入场指纹 `fd4cce23b4d32b28b37b015ce65924e757eedd96ba1d44129e5296f0cfa69b0d`，与开发 ContentState 全部 blob 一致。状态同步后指纹 `c8f758600cb1e20368b91036ff52b05a9a3820bdba4421a21f0665943cb4651e`，同一配方和路径集合，仅任务状态投影改变。
- WorkUnit：`installed-state-ownership:installer-receipts`。
- ContentState：`installed-state-ownership:installer-receipts:review-r1-20261110-JztJk8AtlEwiN45d`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

**ISO-IR-R1-F1 · P2 · OPEN — 完整清理材料分支丢失身份绑定修复的严格拒绝语义**

位置为 `internal/installation/attempt.go:443-459`，调用的 `internal/installation/install.go:509,532-558`，以及传播退出码的 `packaging/homebrew/agentdeck-app.rb.tmpl:181-195`。

- 行为风险。用户已经捕获原 installation ID 并发起绑定修复，但回执随后缺失或损坏时，材料完整的 cleanup 返回成功。Homebrew 因此可以继续卸载记录及后续步骤，尽管本次绑定身份已经无法验证。目标 App 当前被保留，后续 register 仍会拒绝无回执的现存对象，但卸载阶段的非零拒绝边界已失效。
- 证据。`CleanupAttempt` 在材料完整分支将原 ID、清单和 `--authorized-repair` 传给普通 `Unregister`，只把 `skipped_superseded` 转为失败。`Unregister` 仅以“授权且清单参数为空”识别严格绑定检查，因此本分支在回执缺失/损坏时返回 `skipped_unverified`。架构“cask 无主判定与授权修复”要求有绑定时每次卸载均在锁内核对原身份，无法验证时非零拒绝。
- 实测。验收版 CLI 经 `attempt prepare`、`register --require-unowned` 安装临时 cask App，然后由该 App 的 `repair-identity` 捕获真实 ID。保持 locator、manifest、保留 helper 完整，分别移除临时回执和写入无效回执，再调用 `attempt cleanup --authorized-repair --expected-installation <捕获 ID>`。两例均输出 `skipped unverified`，退出码为 0，App 存在。预期分别为 `receipt_missing`、`receipt_invalid` 和非零退出。
- 有界修复。将严格绑定模式显式传递到持锁执行层，使材料完整与材料缺失两个分支都在同一次排他锁内验证预先捕获的身份。保留无绑定授权修复的容忍语义及普通旧清单的幂等跳过语义。补充材料完整情况下回执缺失/损坏，以及捕获 A 后同路径后继 B 丢失/损坏回执的 CLI 回归保护。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

发布和条件删除集中于 helper 的排他锁范围。attempt 清单和保留 helper 存放在 Caskroom 外，回滚入口携带不可变 attempt ID。上述结构在本次检查的代码中可直接追踪。

### 📝 总结

本轮存在一个可由真实 CLI 稳定复现的契约缺陷，结论 FAIL。开发候选的 43 个路径已逐一核对，既有本地 L3 证据仍是该候选的历史检查事实，其测试通过不能覆盖本轮新增反例。入场门禁为 NOT_VERIFIED，缺 independent-review 和 real-brew。本轮失败证据绑定状态同步后的候选，完成门禁为 FAILED。

真实 Homebrew CI 尚未执行，遵循 Task 5 的授权提交、推送后取证例外。本轮复现仅运行临时账户中的 helper 命令，未执行 Homebrew 暂存 helper、真实安装或系统注册。发现阻断后未继续全量检查，因此本轮不宣称其余安装路径已完成全面审查。

#### Evidence

- 使用 `git hash-object` 核对 CEv1 ContentState 保存的全部 43 个路径，差异数为 0。配方为 `head=<HEAD>\n` 加按路径排序的 `<path>=<blob>\n`，再计算 SHA-256。
- 使用现有 Go 1.27.3 的绝对路径和项目指定临时 GOCACHE 构建 `go build -mod=vendor -tags agentdeck_acceptance -o <临时 helper> ./cmd/agentdeck`，退出 0。最初默认命令混用了 Go 1.27.1 和 1.27.3 GOROOT，已改用一致工具链。构建出现用户模块统计缓存写权限提示，但最终构建成功。
- 复现步骤可在新的临时目录重建。设独立 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME`，建立同目录暂存 bundle 与最终 `AgentDeck.app` 路径，bundle 内放置上述验收 helper。调用 `install attempt prepare --attempt <32-hex> --token agentdeck-app --version 1.0.0 --executable <最终 helper> --locator <临时 Caskroom>/.agentdeck-attempt`，使用其输出 manifest 调用 `install register --channel homebrew_cask --executable <最终 helper> --source <暂存 bundle> --cleanup-manifest <manifest> --require-unowned`。调用最终 helper 的 `install repair-identity` 捕获 ID，再分别移除临时账户的 `Library/Application Support/AgentDeck/installation.json` 或将其写成 `invalid`，执行 `install attempt cleanup --token agentdeck-app --version 1.0.0 --executable <最终 helper> --locator <同一 locator> --authorized-repair --expected-installation <捕获 ID>`。两次实际退出 0，App 保留。
- `python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/installer-receipts.md`、`python3 scripts/check-docs.py`、`make check-whitespace` 和 `git diff --check` 均通过。首次文档检查误用了仅提供 `--base HEAD` 的参数组合，因缺少 `--head` 报 TypeError，改为脚本默认的工作树检查后通过。临时复现目录由脚本清理，必要事实已直接保存于本记录。

#### 下一步指令

修复：installed-state-ownership / installer-receipts / ISO-IR-R1-F1


#### Repair 1 — 2026-11-10

**ISO-IR-R1-F1 → repaired in candidate，待独立复评。** Round 1 的 FAIL 和原始反例保留为历史评审事实。

- 根因已通过新增真实 CLI 测试复现。完整 locator、manifest、保留 helper 下，原安装和同路径后继安装分别发生回执缺失、损坏，四例在修复前均错误地退出 0 并返回 `skipped_unverified`。
- `CleanupAttempt` 显式向持锁执行的 `Unregister` 传递内部 `--require-identity`。该参数必须配对 `--authorized-repair`，在同一次排他锁内先核对捕获的 ID/channel/规范 executable，匹配后才允许完整材料的条件删除；材料缺失时完成同样检查后保持只校验语义。原有空清单绑定调用继续兼容，非空清单仍须为绝对规范路径。
- 无绑定授权清理继续容忍不可验证状态，普通旧清单面对有效后继安装继续返回 `skipped_superseded`。CLI 新增四例覆盖原安装、后继安装与两种回执损坏，检查退出码、错误原因、完整 App helper 字节、回执和材料保持，并包含无绑定容忍、正常旧清单跳过以及有效原身份成功删除的对照。单元测试在 `unregister_locked` 钩子内损坏回执，确认核对发生在锁内。
- 同步隐藏命令的架构签名及 Task 5 接口说明。真实 Homebrew CI 加入相同四例，并要求失败时 Caskroom 记录也保持。旧保留 helper 若不识别新内部参数会非零失败，当前代码不会丢弃严格模式后重试。
- Go 1.27.3 验证通过。installation 包 38 个顶层测试，全量 Go 1,591 个顶层测试，installation/platform race 72 个顶层测试，I4/I5（I5 含 21 个子场景）和新增四个 CLI 场景均通过。`make vet`、`make check-install`、双架构构建及 arm64 体积检查、隔离 cask 迁移回归通过。CI 脚本 shell/Python 语法和本地入口拒绝检查通过。
- 复用入场候选的 I1–I3 通过证据，理由是本次改动限于清理请求的严格身份传递及 `Unregister`，`Register` 发布、登记、锁等待与崩溃路径未改。I4/I5 和清理消费者已在修复候选复验。首次开发整包验收的时间相关限制仍按开发交接保留。
- 修复候选 ContentState 为 `installed-state-ownership:installer-receipts:repair-r1-f1-20261110-fJoqKH2nIQ0AbeeX`，保存 HEAD、路径 blob 和工具链。43 路径指纹为 `c869205a7364a5d1a380a7cc6bcc8a5ce7b4b4208cfe4758ac0109c33d4367d2`，沿用开发与 Round 1 的路径集合，评审载体不纳入该集合。修复候选门禁已查询为 `NOT_VERIFIED`，6/8 准则通过，仅余独立复评及 `real-brew`，无未决影响或失效证据。真实 CI 按 Task 5 例外在授权提交、推送后执行。

复评：installed-state-ownership / installer-receipts / ISO-IR-R1-F1

## Round 2 — 2026-11-10

## 📋 ISO-IR-R1-F1 独立复评

📊 综合评分：8/10

✅ 复评结论：PASS

- Reviewer：Codex，延续 Round 1 的独立评审角色，未参与 Repair 1 实现。
- Method：逐路径核对修复 ContentState，审查参数传递、持锁校验、兼容分支与真实 CLI 回归断言，复用精确候选的有效测试证据。
- Scope：用户指定的 ISO-IR-R1-F1 修复及其清理消费者。此轮结论关闭该发现，全面真实 Homebrew 验收仍由 real-brew 准则约束。
- Reviewed state：HEAD `88cfbfbbfe1df89d59fdd2302106b89ba465b43f`，43 路径修复候选指纹 `c869205a7364a5d1a380a7cc6bcc8a5ce7b4b4208cfe4758ac0109c33d4367d2`，逐 blob 核对无差异。任务状态同步后指纹 `098a0e757f4e0f8d59f8afa06adc5a3286e2f5f5b36f657766ed88cf49633912`，仅 tasks.md 状态投影改变。
- WorkUnit：`installed-state-ownership:installer-receipts`。
- ContentState：`installed-state-ownership:installer-receipts:rereview-r2-20261110-crZhBpODBdbecxgw`。
- 完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。ISO-IR-R1-F1 CLOSED。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

`CleanupAttempt` 在完整材料和缺失材料两条绑定路径传递 `--require-identity`。CLI 绑定到 `UnregisterRequest.RequireIdentity`，`Unregister` 在 `WithExclusive` 内、读取清单及删除之前调用 `boundRepairCheck`。回执缺失、损坏和后继身份由该检查非零拒绝。缺失材料保持只校验，完整材料在身份匹配后继续条件删除，无绑定授权仍保留容忍行为。

`TestInstallBoundCleanupCompleteMaterials` 覆盖原安装/后继安装与 missing/invalid 四种组合，检查退出 1、具体原因、App、完整 helper 字节、回执和材料保持。对照覆盖有效原身份删除、无绑定容忍、普通旧清单跳过后继。`TestUnregisterRequiresIdentityUnderLock` 在锁内钩子损坏回执，能识别把身份检查提前到锁外的回归。

### 📝 总结

ISO-IR-R1-F1 已关闭，本轮未发现新的阻断。修复实际保留了绑定身份直到锁内判断，原反例已有真实 CLI 回归保护。旧保留 helper 无法识别新参数时会非零失败，调用方没有去掉严格参数后重试的路径。

入场精确门禁返回六项有效证据，缺独立复评和 real-brew，无失效证据及未决影响。本轮新增独立复评证据，六项修复证据按仅状态投影变化的范围判断复用到最终 ContentState。真实 Homebrew CI 尚未运行，门禁保留 NOT_VERIFIED，任务维持开放。按 Task 5 已批准的 real-brew 时序例外，协调状态进入 awaiting_commit。

#### Evidence

- `git hash-object` 核对修复候选全部 43 个路径，差异数 0。Go 版本现场确认为 `go1.27.3 darwin/amd64`。
- 复用 Repair 1 的四例真实 CLI RED/GREEN、锁内身份测试、I4/I5、installation 单元测试、全量 1,591 个 Go 顶层测试、相关 race、vet、安装、构建和 cask 迁移证据。源代码、测试、依赖和配置与这些证据一致，本轮未重复运行同一测试。
- 审查 `scripts/test-cask-real-brew.sh:400` 四场景，确认完整 brew reinstall 必须非零拒绝，并断言原 Caskroom 摘要保持。该检查目前仅有代码和既有语法证据，真实执行结果仍待 CI。
- Round 1 的整包 tagged CLI 超时历史保留，修复阶段只声明对应定向 tagged 测试通过。
- 本轮记录与任务状态通过评审结构、文档、空白及差异检查。

Task checkpoint：`ad-installed-state-ownership-installer-receipts`，content_state `098a0e757f4e0f8d59f8afa06adc5a3286e2f5f5b36f657766ed88cf49633912`，gate `NOT_VERIFIED`。
提交建议：门禁 VERIFIED 后完成正常任务交付，当前缺 real-brew。Task 5 的已批准时序例外允许另行授权先提交本任务实现、修复、测试及评审记录，以取得 CI 证据。
推送建议：门禁 VERIFIED 后完成正常交付确认，当前缺 real-brew。按同一例外另行授权将上述提交推送至 `origin/feature/installed-state-ownership`，运行 cask-real-brew 并回填提交树证据。

#### 下一步指令

提交并推送：installed-state-ownership / installer-receipts（按 real-brew 时序例外运行 CI 并回填证据）
