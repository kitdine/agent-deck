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
- 复评 ContentState：`installed-state-ownership:installer-receipts:rereview-r2-20261110-crZhBpODBdbecxgw`。
- 当前证据 ContentState：`installed-state-ownership:installer-receipts:repair-real-brew-20261111-aYGRWYmW`，见下方 Repair 2；Round 2 PASS 不覆盖此次修复。
- 完成门禁：FAILED

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

## CI evidence follow-up — 2026-11-10

本节执行用户授权的主题分支推送、real-brew CI 取证与回填，不是新的独立评审或修复。Round 2 的 PASS 仅保留其原有范围；复评当时的门禁为 NOT_VERIFIED、7/8，仅缺 real-brew。以上当前门禁字段同步到实际 CI 目标，不改写原始观察。

### 推送与内容身份

- 首次推送 `origin/feature/installed-state-ownership`，分支头为 `fc290163d2bb7ee0776854e7bc91737f0553867f`，树为 `dded841c3a48f12e52ba8aa05aa1bafe12cc9c29`。与已核验的远端 main 相比有 13 个提交，SSH 签名均有效；工作树入场干净。本轮未创建提交、改写历史、创建 PR、打标签或发布。
- 任务 5 的本地交付提交为 `62e2f239002899fd5d0ba4bf26ea9280c66ba827`，但本次 CI 实际运行于上述主题分支头。不能把运行结果直接记作只验证了任务 5 的原始提交树。
- 对两提交之间的路径和文档差异作范围核对：任务 5 的实现、测试、安装脚本、模板、工作流、Go 依赖与 CLI 标本均未改变。原 43 路径中只有 architecture.md 和 tasks.md 改变，内容是任务 6/7 的进度及 Debug App Group 平台前提更正，不改变任务 5 契约；额外原生测试驱动只属于任务 6。原七项本地验证/独立评审通过事实通过显式影响判断及目标绑定 roll-up 复用，不声称 Swift App 的新行为或任务 8 组合验收已完成。

### 实际 CI 观察

- 工作流 `.github/workflows/cask-real-brew.yml`，push run `42960152309`，job `130324498722`；job 在 UTC `2026-11-11T04:34:48Z` 至 `2026-11-11T04:39:19Z` 执行，结论 failure。环境为 GitHub 托管 `macos-26-arm64` 镜像 `20261102.0473.1`，macOS 26.7.2（25G332），Go 1.26.0 darwin/arm64。复用的历史本地 L3 结果仍属于其原 Go 1.27.3 环境，不冒充本次 runner 上的全量回归。
- `Build the fixture App and widget` 成功。随后执行 `bash scripts/test-cask-real-brew.sh --ci apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app`，退出 1。
- 首个 `lifecycle` 场景的隔离 `brew install --cask --appdir=<临时 appdir> agentdeck/test/agentdeck-app` 失败。Homebrew 在建立补全链接时报告 `AgentDeck.app/Contents/Resources/completions/agentdeck.bash` 的 symlink source 不存在。
- 同一次失败回滚中，清理 helper 的 `pluginkit -r` 返回 1，报告 `AgentDeckWidget.appex` 路径没有已登记插件；attempt 清理报告没有删除对象、已保留清理记录。这是错误输出事实，不是失败后的文件保全断言或完整回滚通过证据。
- 后续正常卸载、重装、升级恢复、材料缺失修复、身份绑定竞争及 I4/I5 完整 brew 链均未执行，不能记为通过。缺失补全资源和小组件注销失败的根因尚未诊断；本轮未修改产品、测试或配置，未在开发机运行真实 brew，也未重跑 CI。
- 同一 SHA 的常规 CI run `42960152203` 中，documentation job `130324568207` 的工具测试通过，但文档检查失败：`git diff --check 660c81586f4c13e04e66dbc05cde5331bf09acec fc290163d2bb7ee0776854e7bc91737f0553867f --` 报 `reviews/state-root-guard.md:124: new blank line at EOF`（完整路径为 `docs/topics/installed-state-ownership/reviews/state-root-guard.md`）。该文件不是本轮回填改动，未顺手修复；不能宣称常规 CI 通过。

### 证据与交接

- WorkUnit 保持 `installed-state-ownership:installer-receipts`。本次目标为 `installed-state-ownership:installer-receipts:ci-fc290163-42960152309`，`real-brew` 的新观察为 `outcome: fail`，绑定实际运行的提交及树；原观察未重标或覆盖。
- 使用固定 gate-status 查询精确目标，返回 FAILED、`reason=null`；八项必需准则中七项有有效复用证据，real-brew 有适用失败证据，无失效证据或未决影响。当前完成边界保持开放，不查询或宣称 Topic 完成，来源缺陷不关闭。
- 按 tasks.md 的已批准证据时序例外，任务退回 `in_progress`，不因已推送或独立复评 PASS 关闭。首次真实安装的可行性检查未通过，后续须先处理批准设计中的对应可行性边界，不能静默更换 Homebrew 分派方案或把未验收范围转给任务 8；诊断、修复及新的 Git 交付均待相应授权。
- 以上保存必要命令、内容身份、结果及限制；run/job ID 仅作为取证来源标识，不依赖会过期的 CI 日志、临时文件或工件作为持久证据。本轮回填文件仍是本地未提交改动，不包含在已推送的 SHA 中。

## Repair 2 — 2026-11-11

本节响应用户对 run `42960152309` 的有界修复指令，仅处理上述两条 CI 观察，不是新的独立评审。为交接分别编号为 **ISO-IR-CI-F1**（补全资源缺失）和 **ISO-IR-CI-F2**（未登记扩展阻断回滚）；两者均为 **repaired in candidate，待独立复评及真实 CI**。原 Round 2 PASS、CI 失败和 ISO-IR-R1-F1 CLOSED 保留原范围与内容身份。

### 根因与契约边界

- **ISO-IR-CI-F1：CI 夹具缺少打包资源。** workflow 仅运行 `build-macos-app.sh`，随后复制 unsigned Debug App、替换验收 helper 并签名归档。补全只在 `package-macos-app.sh` 的打包阶段生成，因此该夹具缺少 cask 声明引用的三个补全源。生产打包脚本已有生成逻辑；本次不改生产打包或 cask 的补全路径。
- **ISO-IR-CI-F2：清理遗漏了“尚未登记”的正常状态。** `deregisterWidgets` 对所有磁盘上的 `.appex` 无条件执行 `pluginkit -r`。首个安装在显式 `Scenario.widget()` 登记前已因补全失败，系统也未发现这个临时扩展，故注销报告 `remove: no plugin at <path>`，并按既有失败保全边界阻断 App/回执删除。
- 分类为 CI 夹具缺陷和生产清理缺陷。日志中 helper 已执行、App 已发布、binary 已进入链接和逆向清理，排除“classic flight stanza 被拒绝”或“验收 helper 被 Gatekeeper 终止”作为这次最早根因；后续完整兼容性仍须真实 CI。无需改设计：仍在原排他锁与安装身份匹配后完成必要注销；精确路径没有登记时没有待注销对象。查询失败、不可识别的诊断（包括退出 0 的 `match: Connection invalid`）和已登记项注销失败继续非零返回，保留 App、回执和材料，不转为锁外兜底删除。

### 修复映射

- F1：把原 CI 夹具准备步骤提取为 `scripts/cask_real_brew_fixture.py`，直接使用替换后的验收 helper 生成 bash/zsh/fish 补全，验证非空、设置 `0644`，然后才签名。CI 同时运行该函数的隔离回归；空输出、纯空白和命令失败均禁止签名。原始 Debug App 不被修改。
- F2：清理先执行 `pluginkit -m -A -D -vv`，沿用现有注册检查器的 `Path =` 字段，逐字比较完整扩展路径；仅注销已登记的该路径。其他路径（含同前缀路径）不被注销，空结果及明确零登记结果允许继续同一条件清理。
- Go 回归通过真实 `Resolver.Unregister` 和临时记录工具运行七种状态，验证失败时 App/helper 字节、回执和不可变清单保持，成功时匹配对象删除。原实现七例均为 RED，修复后 GREEN；没有访问系统 PlugInKit 注册库。
- 真实 CI 增加 `completion-artifact-rollback`：刻意把 bash 补全源改成缺失文件，要求真实 brew 非零失败、App/回执清理完成、外部 helper/manifest 保留并有 cleaned 标记。这段新增真实链未在本机执行，不能用本地替身结果代替。
- 本地 cask 驱动的 PlugInKit 记录工具及断言同步为查询后注销；新增显式 `--local-only`，只跳过原有真实 brew formula 冲突检查，默认检查范围不变。本次使用该选项完成其余真实 helper / Ruby 声明驱动场景，没有调用 brew。

### 验证与剩余边界

- 候选 HEAD 为 `fc290163d2bb7ee0776854e7bc91737f0553867f`，48 路径 SHA-256 为 `5759dae9869e2fc8d660d57a2c7697f19f6ae6bde52385bd40d8afe1a13ada3e`。沿用 CI 的 45 路径集合，加入新 Go 回归、Python 夹具函数及其测试；按 `head=<HEAD>` 加 LF、排序后的 `<path>=<blob>` 各加 LF 计算。修复评审载体不纳入该集合，必要事实直接存于本记录。CEv1 新建 12 个节点、22 条关系，节点与关系端点逐条读回一致；固定 gate-status 查询返回 FAILED，五项通过，validation 失败，independent-review/real-brew 未验证，失效证据与未决影响列表均为空。
- Go 1.27.3 darwin/amd64。新增七场景 RED/GREEN；`python3 -B scripts/test-cask-real-brew-fixture.py` 两个顶层测试（含三个拒绝变体）通过。`scripts/run-go-test.sh -race ./internal/installation ./internal/platform` 的 73 个顶层测试通过。
- `scripts/run-go-test.sh ./...` 退出 1：27 个包通过，`internal/scanruntime` 的 `TestServingJoinedRoundPublishesBeforeHistory/local=false/appFirst=false` 在 `serving_test.go:479` 因长度为 0 的切片索引 panic。全程有 1,583 个顶层测试通过，不能声称全量 Go 通过。该目录无本次改动，本次按用户边界未诊断、修复或重复运行无关失败；`cmd/agentdeck` 与 `internal/installation` 包均通过。
- `scripts/run-go-test.sh -tags agentdeck_acceptance ./cmd/agentdeck -run '^TestInstall(InterleavingI[45]|BoundCleanupCompleteMaterials)'` 通过，覆盖受清理影响的 I4、I5 和完整材料身份绑定保护。I1–I3 的发布/登记实现、相关测试及契约未变，保留其原状态的通过事实；claim 标本、formula 入口与模板也未变，按范围判断复用。
- `make vet build-all check-arm64-size check-install`、`bash scripts/test-cask-migration.sh --local-only` 通过。shell 语法、inline Python 语法及 real-brew 入口在开发机退出 2 的保护检查通过。构建为当前源的双架构静态证据，不是安装或原生运行验收。
- 原始失败命令涉及真实 brew，受用户明确限制，本次未重跑；真实 PlugInKit 行为也仍须 CI。常规 CI 的 `state-root-guard.md` EOF 空行问题未改。本次没有 Git 交付，也没有运行 release/distribution 中调用真实 brew 的套件。

WorkUnit 为 `installed-state-ownership:installer-receipts`，修复目标为 `installed-state-ownership:installer-receipts:repair-real-brew-20261111-aYGRWYmW`。门禁保持 **FAILED**：validation 记录实际全量 Go 失败，independent-review 与 real-brew 均未取得修复候选的通过证据。其余五项局部证据不代表 Task、Topic 或来源缺陷已完成。Beads 交回 `in_review`、释放修复所有权；Review 未勾选。本记录保存必要事实，不依赖临时日志或过期 CI 工件。

下一步指令：复评：installed-state-ownership / installer-receipts / ISO-IR-CI-F1、ISO-IR-CI-F2

## Round 3 — 2026-11-11

## 📋 ISO-IR-CI-F1、ISO-IR-CI-F2 独立复评

📊 综合评分：7/10

✅ 复评结论：FAIL

- Reviewer：OpenCode（GPT-6.1 Sol），未参与 Repair 2 的产品实现；本会话此前只执行推送与 CI 证据回填，本轮为只读独立复评角色，未委派。
- Method：核对修复候选全部 48 个 blob，检查补全夹具、签名前顺序、工作流和查询后注销的调用路径；以临时 Go overlay 在真实 `Resolver.Register` / `Unregister` 与所有权锁上增加反例。生产、测试和配置文件均未修改；仅更新本评审载体、任务状态与证据/协调记录。
- Scope：用户指定两项 CI 修复及其清理消费者。发现决定性新阻断后不扩大验证；不重跑已有局部检查或全量 Go，不诊断范围外 `scanruntime` 失败，不运行真实 brew 或系统 PlugInKit。
- Reviewed state：HEAD `fc290163d2bb7ee0776854e7bc91737f0553867f`；入场 48 路径指纹 `5759dae9869e2fc8d660d57a2c7697f19f6ae6bde52385bd40d8afe1a13ada3e` 与 Repair 2 完全一致。状态同步后为 `6abc2f86f0f497903a4c93f856f05e3fd16220c6f522264d311f02496f0f4827`，只有 tasks.md 状态投影改变，其 blob 为 `f05dd62e0c25247d8efb657bcc99b04e8b8a4652`。沿用 HEAD 加 LF、排序 path/blob 加 LF 的 SHA-256 配方，评审载体不纳入集合。
- WorkUnit：`installed-state-ownership:installer-receipts`。
- ContentState：`installed-state-ownership:installer-receipts:rereview-r3-20261111-gR7B5Ep3`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

**ISO-IR-R3-F1 · P2 · OPEN — 混合查询输出会把 PlugInKit 失败当成“没有登记”的证明**

位置：`internal/installation/install.go:95-113`，随后删除发生在 `:618-632`。新增回归集合为 `internal/installation/widget_cleanup_test.go:12-23`。

- **处置：新发现。** 原始未登记扩展的清理缺陷已修复，但新引入的查询解析会忽略与正常记录同时出现的错误诊断。现有七例只覆盖单独的 `match: Connection invalid`，没有覆盖混合输出。
- **行为风险。** `CombinedOutput` 合并 stdout/stderr；任一 `Path =` 行或零登记汇总都会令 `recognized=true`，其他诊断行不再影响结果。当目标未出现在部分列表中时，代码跳过注销并继续删除当前 App 和回执，即使查询是否完整、目标是否真的没有登记尚未证实。这破坏了该修复声明保留的查询失败保全边界，可能留下系统登记却丢失可重试清理所需对象。
- **证据。** Go 1.27.3 darwin/amd64 下，只运行一次 `TestReviewWidgetQueryPartialDiagnostic`。夹具通过实际 `Register` 建立临时 cask App、回执和清单，再将 `pluginkitTool` 指向只记录/返回结果的临时脚本；`Unregister --remove` 仍执行真实持锁清理。脚本的查询均退出 0，在 stderr 写入 `match: Connection invalid`，stdout 分别返回另一条相近扩展路径和零登记汇总：

  | 查询 stdout | 预期 | 实际 |
  | --- | --- | --- |
  | `Path = <目标扩展>.other` 与 `(1 plug-in)` | 返回查询错误，保留 App/回执/清单 | `outcome=removed`、`err=nil`，helper 与回执均已删除 |
  | `(0 plug-ins)` | 同上 | 同上 |

  两例均未执行 `-r`。测试明确输出 `helper_exists=false receipt_present=false`，runner 退出 1。这是合成查询响应经过真实清理路径的回归证明，不宣称已从真实系统 daemon 观察到相同组合。
- **💡 有界修复。** 只有完整且成功的查询才能证明目标没有登记；不得因为出现一条可识别记录就吞掉失败诊断。分离或严格处理诊断通道，并为上述混合输出补回归，断言非零失败、App/helper 字节、回执与不可变清单保持，且不执行注销/删除。保留干净空结果、完整其他路径列表、精确目标注销以及注销失败保全的既有语义；不改变所有权锁或身份绑定协议。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ISO-IR-CI-F1 CLOSED。** `prepare_fixture_app` 使用替换后的验收 helper 生成 bash/zsh/fish 补全，检查非空并设置 `0644` 后才签名；真实 CI 调用同一函数并运行隔离回归。三个路径与生产 cask/打包声明一致，原 Debug App 不被修改。关闭的是本地夹具缺陷，不是宣称真实 brew 已通过。
- **ISO-IR-CI-F2 CLOSED（原始状态）。** 从未登记的扩展现在可以在干净空查询或明确零登记结果下继续清理；精确路径已登记时仍调用 `-r`，查询非零或单独不可识别诊断、注销失败仍阻止删除。混合查询失败的新增缺口由 ISO-IR-R3-F1 单独承接。
- **ISO-IR-R1-F1 保持 CLOSED。** 本次未改动绑定身份传递与锁内核对，完整/缺失清理材料的严格绑定分支保持原结构。

### 📝 总结

两项指定原始发现已关闭，但 ISO-IR-R3-F1 可稳定复现，结论 FAIL；任务 Review 继续未勾选并交回修复。本轮没有产品修复、提交、推送或 CI 重跑。

复用 Repair 2 中未变化的局部回归、race、I4/I5、构建与安装检查事实，不把它们扩大为混合输出或真实系统注销证明。原全量 Go 的 `TestServingJoinedRoundPublishesBeforeHistory/local=false/appFirst=false` 在 `serving_test.go:479` 的 panic 仍属于适用 validation 失败，未诊断、重跑或豁免；修复候选的 real-brew 仍无实际执行结果。此前推送 SHA 的失败不能替代本候选的 CI 观察，其他 CI 问题未修改。

#### Evidence

- 入场 48 路径按既定指纹配方核对成功。只读 overlay 在新建的临时映射中加入 `TestReviewWidgetQueryPartialDiagnostic`；可复现命令为 `scripts/run-go-test.sh -overlay <新建 review-overlay.json> ./internal/installation -run '^TestReviewWidgetQueryPartialDiagnostic$'`。用上述两种 stdout/stderr 与退出码实现查询脚本，再由真实注册/卸载夹具验证即可重建反例；不依赖当次临时路径或日志仍然存在。
- 最终目标新增 9 个 CEv1 节点及 23 条关系，节点完整载荷逐项读回，关系预检全部 ok，写入计数与逐端点读回均一致。四项局部准则通过，helper-protocol、independent-review 与 validation 有适用失败证据，real-brew 未验证；固定 gate-status 查询返回 FAILED、`reason=null`，无失效证据或未决影响。
- 本任务仍有任务 8/9 等未完成边界，未查询或宣称 Topic 完成，也未关闭来源缺陷。必要复现输入、实际删除结果、内容身份和限制已直接写入本记录，临时 overlay 不作持久引用。

#### 下一步指令

修复：installed-state-ownership / installer-receipts / ISO-IR-R3-F1

## Repair 3 — 2026-11-11

**ISO-IR-R3-F1 → repaired in candidate，待独立复评。** Round 3 的 FAIL、混合输出反例和原始文件删除结果保持历史范围。本节仅修复该发现，不重新评审或更改其他 CI/全量 Go 失败。

- 根因是把 stdout/stderr 合并后，以任一 `Path =` 行或零登记汇总作为整个查询可用的证明，因而吞掉同时出现的失败诊断。新增五例在原实现均 RED、错误返回 `removed`：其他路径/零登记汇总与 stderr 诊断混合、精确目标与 stderr 诊断混合，以及其他路径/零登记汇总与 stdout 的 `match:` 诊断混合。
- 查询改为独立捕获 stdout 与 stderr。非零退出或非空 stderr 在解析任何记录前即返回错误；stdout 中出现 `match:` 诊断也立即拒绝，不能被此前/后续的有效路径或零登记行掩盖。干净空结果、零登记汇总、其他完整路径列表、精确路径注销和注销失败保全的既有语义保持。
- 12 种状态均通过真实 `Resolver.Register` / `Unregister` 与原所有权锁，PlugInKit 仅为临时记录工具。失败例要求错误返回，App 主程序/helper 完整字节、扩展目录、回执和不可变清单保持，记录只能出现一次查询、不出现 `-r`；成功与精确路径对照继续保留。没有真实系统注册操作、锁外删除或身份绑定协议变更。
- Go 1.27.3：installation 包 39 项顶层测试通过，相关包 race 的 39 项顶层测试通过；增强 App/扩展保全断言后的最终 12 场景另作定向验证。受影响包 vet、双架构构建和 arm64 体积检查通过。
- 变更仅涉及查询错误通道、诊断拒绝与其测试。干净查询/注销路径已由本次正向对照覆盖，`Register`、attempt、绑定身份、模板、CI 夹具、依赖、标本和安装驱动未变，因此沿用 Round 3 绑定的 I4/I5、原安装脚本与夹具检查事实。本轮没有因阶段变化重跑这些未改变的范围。仍保留原全量 Go 中未修改的 `scanruntime` 测试 panic 为 validation 失败，不诊断、重跑或豁免；真实 brew 与修复候选独立复评仍无通过证据。
- 当前 WorkUnit 为 `installed-state-ownership:installer-receipts`，候选 ContentState 为 `installed-state-ownership:installer-receipts:repair-r3-f1-20261111-arX4dMMv`。沿用 48 路径指纹与 LF 配方，评审载体不入集合。完成门禁保持 **FAILED**：helper-protocol 已有本轮通过证据，原 validation 失败保留，independent-review 与 real-brew 未验证。历史失败不覆盖或改写，未查询 Topic 门禁，不关闭 Task 或来源缺陷。
- 最终 HEAD 为 `fc290163d2bb7ee0776854e7bc91737f0553867f`，48 路径 SHA-256 为 `04973be972b56c59e77881b0497de8edfe8fdc618aa72d5409abaa26cbe53b84`；与 Round 3 目标逐 blob 比较，仅 `install.go`、`widget_cleanup_test.go` 和 tasks.md 状态投影改变。最终增强断言的 12 场景已通过。15 个新 CEv1 节点完整载荷、31 条预检通过的关系及其端点均已逐项读回；固定 gate-status 返回 FAILED、五项通过，只有 validation 有失败证据，独立复评及 real-brew 未验证，失效/未决影响列表为空。
- Beads 交回 `in_review` 并释放修复所有权，任务 Review 保持未勾选。未在开发机执行真实 brew，未修复其他 CI 问题，未提交或推送。必要反例、输入和结果直接保存于本节，不依赖临时日志。

下一步指令：复评：installed-state-ownership / installer-receipts / ISO-IR-R3-F1

## Round 4 — 2026-11-11

## 📋 ISO-IR-R3-F1 独立复评

📊 综合评分：8/10

✅ 复评结论：PASS

- Reviewer：OpenCode（GPT-6.1 Sol），延续 Round 3 独立评审角色，未参与 Repair 3 实现，未委派。
- Method：核对修复 ContentState 的全部 48 个路径；检查查询通道分离、诊断拒绝的先后顺序、实际 `Unregister` 清理边界及 12 场景的断言；复用精确候选已登记的有效验证事实，不因进入复评重复运行产品测试。
- Scope：ISO-IR-R3-F1 的混合输出拒绝及受影响的注册/未注册清理分支。生产、测试及配置保持只读；范围外全量 Go 失败和真实 CI 取证不在本轮执行范围内。
- Reviewed state：HEAD `fc290163d2bb7ee0776854e7bc91737f0553867f`；入场 48 路径指纹 `04973be972b56c59e77881b0497de8edfe8fdc618aa72d5409abaa26cbe53b84` 与 Repair 3 完全一致。同步 Review/状态后指纹 `87d3c4b232a13964b2f2a13d8c5f2ce4366a64068fdd2e2e23b8c3c7fba31925`，只有 tasks.md 改变，其 blob 为 `308d82ee83b0240361c0aeae47b57a590f66340a`。沿用 HEAD 加 LF、排序 path/blob 加 LF 的 SHA-256 配方，评审载体不纳入集合。
- WorkUnit：`installed-state-ownership:installer-receipts`。
- ContentState：`installed-state-ownership:installer-receipts:rereview-r4-20261111-HloMiWBA`。
- 当前证据 ContentState：`installed-state-ownership:installer-receipts:validation-scanruntime-20261111-midnight`，见下方 Validation follow-up；复评当时的 FAILED 仍保留为历史观察。
- 完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。ISO-IR-R3-F1 CLOSED；本轮未发现新的范围内阻断。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ISO-IR-R3-F1 CLOSED。** `install.go` 使用 `query.Output()` 和独立 stderr 缓冲；非零退出或非空 stderr 在解析任何正常记录前返回错误。stdout 的全部行在执行 `-r` 前扫描，出现 `match:` 时立即拒绝，既不能被其他路径/零登记汇总掩盖，也不能因为此前已看到目标路径而提前注销。
- `TestWidgetCleanupRegistrationBoundary` 的五个混合诊断场景覆盖其他路径、零登记和精确目标与 stderr 组合，以及其他路径/零登记与 stdout 诊断组合。失败断言检查 App 主程序/helper 完整字节、扩展目录、回执、不可变清单以及只有一次查询的调用记录；不只是核对错误字符串。原始 Round 3 两种反例均有直接对应场景。
- 干净空查询、零登记汇总、其他路径、精确目标注销，以及查询非零/单独诊断/注销失败的正反对照仍保留。错误分支在删除回执与 App 之前返回，原所有权排他锁和身份绑定检查未变化。
- ISO-IR-CI-F1、ISO-IR-CI-F2 和 ISO-IR-R1-F1 保持 CLOSED，未把它们重新列为开放问题。补全夹具、attempt、绑定身份、模板、依赖及相关安装驱动均未被此次查询通道修复改变。

### 📝 总结

ISO-IR-R3-F1 已关闭，复评 PASS，任务 Review 已勾选。本轮复用 Repair 3 的 12 场景、installation 包及其 race 各 39 项顶层测试、受影响包 vet、双架构构建及体积检查；其命令、结果与原环境保持在 Repair 3/CEv1 的原观察中，没有重新标为本轮新执行结果。I4/I5、安装脚本与夹具沿用既有有界证据，不由局部查询测试推断完整系统验收。

复评 PASS 与任务完成门禁分开。复评当时，固定查询精确最终目标返回 FAILED、`reason=null`：六项准则通过；validation 仍有适用失败证据，real-brew 未验证；无失效证据或未决影响。原全量 Go 的 `TestServingJoinedRoundPublishesBeforeHistory/local=false/appFirst=false` 在 `internal/scanruntime/serving_test.go:479` 的 panic 未被本轮诊断、重跑、修复或豁免。真实 brew 未在本机执行，也没有修复候选的真实 CI 通过结果。

Task 5 的时序例外只允许 real-brew 在授权提交/推送后取证，不能豁免 validation。任务因此保留 `in_review`、释放 reviewer 所有权等待门禁补齐，而非进入 `awaiting_commit` 或关闭。任务 8/9 仍未完成，本轮不查询或宣称 Topic 门禁，也不关闭来源缺陷。未提交、推送或执行产品修复。

#### Evidence

- 按既定 48 路径及指纹配方核对修复候选成功；状态同步仅改变 tasks.md，产品和测试 blob 未变。原 helper-protocol 通过观察与其依赖有效，故不重跑同一套产品检查。
- 最终目标新增 9 个 CEv1 节点及 24 条关系；节点完整载荷逐项读回，关系预检全部 ok，写入计数与逐端点读回一致。新增独立复评通过观察不依赖旧的失败评审；原 validation 失败与 real-brew 未验证状态通过显式目标绑定保留，没有覆盖历史结果。
- 本记录直接保存修复机制、断言范围、内容身份与未解决前提，不引用临时校验脚本或会过期的 CI 工件。

Task checkpoint：`ad-installed-state-ownership-installer-receipts`，content_state `87d3c4b232a13964b2f2a13d8c5f2ce4366a64068fdd2e2e23b8c3c7fba31925`，gate `FAILED`。
提交建议：门禁 VERIFIED 后完成本任务修复、回归及必要记录的正常交付；当前须先补齐 validation，real-brew 仍按已批准时序例外另行授权 CI 取证交付，不包含其他未授权 CI 修复。
推送建议：门禁 VERIFIED 后完成 `origin/feature/installed-state-ownership` 的正常交付确认；当前缺 validation 与 real-brew。补齐 validation 后，按既定例外提交/推送 CI 候选仍须另行授权，并将真实结果绑定实际 SHA。

#### 下一步指令

调查并处理 `internal/scanruntime` 的 `TestServingJoinedRoundPublishesBeforeHistory/local=false/appFirst=false` 在 `serving_test.go:479` 的 panic，补齐 installed-state-ownership / installer-receipts 的 validation 门禁；不在本机执行真实 brew，不提交、不推送。本轮复评已完成，不重复复评。

## Validation follow-up — 2026-11-11

本节执行用户单独授权的 scanruntime panic 调查、处理及 validation 补齐，不是新的独立复评。Round 4 PASS 与当时的 FAILED 门禁保留原范围；上方当前门禁字段指向本次新目标。

### 根因与判别实验

- **分类：测试夹具与断言缺陷。** 原夹具用 `now := time.Now()`，把首批事件设为 `now.Add(-time.Minute)`，却无条件假设它属于“今日”，随后直接索引 `Scopes[0].Periods.Items[0]`。
- 单独运行原子用例在当前时刻通过。用只改变该用例 `now` 的临时 Go overlay，对照固定本地 `2026-11-11 00:00:30` 与 `12:00:30`：午夜例在原 `serving_test.go:479` 稳定复现同一空切片 panic，退出 1；中午例通过。两例使用相同 Go 1.27.3、数据和同步屏障，未改产品或测试的调度/超时。
- 午夜例的首批事件是前一天 `23:59:30`。历史批次仍被屏障阻塞、coverage 为已提交 1/来源 2，因此今日尚无事件且覆盖不完整。`desktop.BuildToday` / `withholdUnprovenZero` 按契约将未知零值隐藏，令 usage unavailable 并清空 presentation。publication 本身已经产生、覆盖数检查也已通过；panic 来自测试对空 presentation 的未经检查索引。设计要求跨日不得把昨日 totals 标成今日，也不能在覆盖未证实时声称今日为零，故无需改变产品的隐藏语义。
- 相同调度下仅改变时钟即可切换失败/通过，排除了本次反例由 admission 顺序、编译工具链或等待预算不足导致的解释。没有增加 timeout、sleep、重试或跳过断言。

### 修复与验证

- 仅修改 `internal/scanruntime/serving_test.go` 的该用例：固定午夜后 30 秒和中午后 30 秒的时钟，将首批事件放在同一个 `now`；覆盖 worker/local × App/CLI 先到 × 两个时刻，共八种组合。读取 tokens 前检查 usage available、scope 和 period 存在，异常时报告完整视图和 warnings。仍要求首批 28 tokens、覆盖 1/2、历史处理前无 CLI terminal、App 退出不取消 CLI，以及最终历史覆盖完整。
- `scripts/run-go-test.sh ./internal/scanruntime -run '^TestServingJoinedRoundPublishesBeforeHistory$' -count=3` 通过：24 次子场景，包含所有午夜与中午组合。
- `scripts/run-go-test.sh ./...` 通过：28 个包、1,591 项顶层测试。此前 validation 中记录的 scanruntime panic 已由新的成功观察解决；原失败事实仍保留在原 ContentState，不改写为成功。
- `scripts/run-go-test.sh -race ./internal/scanruntime` 通过：47 项顶层测试；受影响包 `go vet -mod=vendor ./internal/scanruntime` 通过。工具链为 Go 1.27.3 darwin/amd64。
- Task 5 的安装产品、回归测试、CI 夹具、模板、依赖及配置均与 Round 4 候选相同；本轮新增的是原 48 路径之外的 scanruntime 测试修正及状态同步。因此复用原安装/夹具检查、I4/I5、安装包 race、vet、双架构与体积证据，并保留 Round 4 对安装修复的独立 PASS；不声称本轮产生了新的独立评审。全量 Go 与受影响并发包证据已刷新，其他既有验证不因进入本轮而重复执行。

### 当前证据与交接

- WorkUnit：`installed-state-ownership:installer-receipts`。新 ContentState：`installed-state-ownership:installer-receipts:validation-scanruntime-20261111-midnight`。沿用 HEAD 加 LF、排序 path/blob 加 LF 的指纹配方，并把本轮修改的 `internal/scanruntime/serving_test.go` 加入原 48 路径，形成 49 路径；评审载体仍不入集合。
- 最终 HEAD 为 `fc290163d2bb7ee0776854e7bc91737f0553867f`，49 路径 SHA-256 为 `d420b02bf5a8ac05873bcdf78f110e1479ce5f47ac1978a278f830e576252f11`。与 Round 4 逐 blob 比较，原 48 路径只有 tasks.md 状态投影改变，新增纳入的 serving_test.go 是唯一测试代码改动。16 个新节点完整载荷和 36 条预检通过的关系/端点均已读回；固定 gate-status 返回 NOT_VERIFIED、`reason=null`，validation 有当前适用通过证据、无当前适用失败证据，仅 real-brew 缺失，失效证据及未决影响列表为空。
- validation 的新通过证据绑定这个候选，不依赖此前的失败聚合。旧失败没有被覆盖或当作通过证据复用。其他六项已通过准则经逐路径范围核对后显式目标绑定复用；real-brew 保持未验证。
- 当前 Task 门禁为 **NOT_VERIFIED，7/8**，仅余 real-brew。按 Task 5 已批准的 CI 取证时序例外，Beads 转 `awaiting_commit` 并释放处理所有权；任务、Topic 与来源缺陷均未关闭。后续提交/推送及真实 CI 仍需要单独授权。
- 未在本机执行真实 brew、系统注册、提交或推送；未处理其他 CI 问题。诊断 overlay 和日志只用于本轮，必要输入、命令、结果与限制已直接保存于此，不以临时路径为证据引用。
