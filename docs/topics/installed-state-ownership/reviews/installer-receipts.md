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
- 当前证据 ContentState：`installed-state-ownership:installer-receipts:ci-cycle4-20261111`，见下方 CI cycle 4；复评、validation 补齐和既有提交态 CI 的原始门禁仍保留为历史观察。
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

## CI evidence follow-up — 2026-11-11

本节执行用户在本地提交之后另行授权的“推送，进行ci验证”，不是新的修复或独立复评。Round 4 PASS、已关闭发现和 validation 通过事实保留各自原范围；当前完成门禁单独反映实际推送 SHA 的 CI 结果。

### 推送与内容身份

- 已核验的 SSH 签名提交 `5804e5cfaad0d03566dfdadefdf46d6f29507282`，树 `cbde46006cf3d05778bdd420bbee9eb8ee509ec2`，标题 `fix(installation): harden cask fixture and widget cleanup`，包含已复评安装修复、validation 测试修正及相关记录，共 10 个文件。其 49 项范围内容与 Validation follow-up 候选一致，原七项通过证据已显式绑定到不可变交付目标 `installed-state-ownership:installer-receipts:delivery-5804e5cf`。
- 用户授权后，`origin/feature/installed-state-ownership` 从 `fc290163d2bb7ee0776854e7bc91737f0553867f` 推进到该提交；推送后工作区干净、分支与远端同步。两条 push 工作流均确认 `headSha` 为 `5804e5cfaad0d03566dfdadefdf46d6f29507282`，不是先前失败的 `fc290163`。

### 实际 CI 观察

- **real-brew：failure。** 工作流 `.github/workflows/cask-real-brew.yml`，run `43006525357`，job `130475261732`；job 在 UTC `2026-11-11T10:47:21Z` 至 `2026-11-11T10:51:54Z` 执行。runner 为 `macos-26-arm64` 镜像 `20261102.0473.1`，macOS 26.7.2（25G332），Go 1.26.0 darwin/arm64。
- `Check cask fixture preparation` 成功，`python3 -B scripts/test-cask-real-brew-fixture.py` 的 2 项测试通过；`Build the fixture App and widget` 也成功。这些成功不等于真实 Homebrew 验收通过。
- `Exercise the real Homebrew installer` 执行 `bash scripts/test-cask-real-brew.sh --ci apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app`，退出 1。`prepare_fixture_app` 在 `scripts/cask_real_brew_fixture.py:16` 调用临时 App 的 `Contents/Helpers/agentdeck completion bash` 时得到 exit 1；驱动在 `run` 的第 41 行抛出 `AssertionError: Unexpected exit 1`。helper 输出 `state_ownership_refused`，`resource: state_root`，`reason: real_root_unresolved`。
- 此次失败发生在测试 App 的补全生成阶段，尚未进入 `lifecycle` 或任何真实 brew 安装场景。安装、升级恢复、注销、损坏材料修复、身份绑定竞争及 I4/I5 完整 brew 链均不能记为通过；本轮只记录失败层与原始诊断，不推断根因，也不把夹具单元测试成功当作该调用成功。
- **常规 CI：cancelled。** run `43006525312` 在第 1 轮新推送后按既有 concurrency 配置自动取消；其中 classify、documentation 成功，Go `Verify` 步骤被取消，不能记作 Go 成功或失败。desktop job `130475660245` 已于 UTC `2026-11-11T10:55:11Z` 失败：Swift 构建/单元测试成功，但 `Check desktop distribution in isolation` 拒绝 Homebrew 的两条 classic flight 弃用警告；原始输出、判别及处理见下方 CI cycle 2。该结果属于 `5804e5cf`，不是后续修复 SHA 的运行结论。

### 证据与交接

- WorkUnit 为 `installed-state-ownership:installer-receipts`，本次 `real-brew` 新失败观察为 `installed-state-ownership:installer-receipts:evidence:real-brew-ci-43006525357`，直接绑定上述提交态 ContentState；没有重标历史观察或将 CI 结果绑定到后来的文档回填内容。
- 新增 1 个 evidence 节点及 3 条关系，完整节点载荷和关系端点已读回，关系预检全部 ok，实际新增计数与提交数一致。新失败观察显式 supersede 该提交态原有的 `real-brew: not_verified` 占位；原占位仍保留，不能再作为当前适用证据。
- 固定 gate-status 查询该精确目标返回 **FAILED**、`reason=null`；8 项必需准则中 7 项仍有有效通过证据，只有 real-brew 有适用失败证据，未决影响列表为空。原七项本地/复评证据没有变化，不因推送重复运行；其原工具链和有界覆盖不扩展为本次 CI 全部通过。
- 按 Task 5 已批准时序例外，CI 失败后任务退回 `in_progress`，保持未分配，等待相应诊断/修复授权；Round 4 独立复评结论不由本次取证擅自改写。Task、Topic 和来源缺陷保持开放，不把缺失真实验收转给任务 8。
- 本轮只推送既有提交并回填结果，没有新增提交、产品/测试/配置修改、CI 重跑、手工 workflow dispatch、开发机真实 brew、PR、标签或发布。本节及 tasks.md 的回填是本地未提交文档，不包含在已推送 SHA 内；必要检查事实已直接保存，run/job ID 只作来源标识，不依赖会过期的日志或工件。

## Repair 4 — 2026-11-11

本节是用户授权“修复、提交、推送、验证，最多 3 轮”的第 1 轮修复。用户随后明确选择“不授权委派”，接受完成诊断和本地修复后在独立复评边界暂停。因此本轮不自行签发复评 PASS，不越过该边界提交或推送；第 2、3 轮尚未开始。

### 根因与判别实验

- **分类：CI 测试夹具缺陷，置信度高。** `test-cask-real-brew.sh` 的 `base_env` 清除全部验收账户变量；共享归档的 `prepare_fixture_app` 在任何 `Scenario` 创建账户之前执行。替换后的 helper 带 `agentdeck_acceptance`，其 `acceptanceAccountHome` 必须从显式验收变量取得绝对且存在的账户目录。原准备函数执行 `completion` 时没有提供该目录，`GuardStateRoot` 因无法确定受保护入口而按既定契约以 `real_root_unresolved` 拒绝。这不是产品应放宽的未安装访问路径。
- 用一次 `go build -mod=vendor -trimpath -tags agentdeck_acceptance -o <临时 helper> ./cmd/agentdeck` 得到同一个程序，在相同临时 HOME 下作两个子进程对照：没有验收账户变量时，`completion bash` 为 exit 1、stdout 0 字节，诊断与 CI 完全一致；仅加入指向独立临时账户的 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 后为 exit 0、stdout 75,243 字节。两次均没有在该临时 HOME/账户中产生文件。
- 同一二进制仅改变账户变量即可切换结果，说明无需改动产品守卫、签名、Homebrew 分派或超时来解决这一最早分歧。该拒绝也可在本地工具链复现，不支持把它归因于 CI 的 Go 1.26.0 编译差异；这不预判尚未执行的真实安装或其他 CI 失败。

### 修复与回归

- `scripts/cask_real_brew_fixture.py` 仅为补全 helper 子进程创建短生命周期的临时 HOME 和独立临时账户，均置于当前 fixture 版本目录的私有子目录。环境复制自调用者提供的 `base_env`，清除继承的 `AGENTDECK_ACCEPTANCE_*` 和 `HOMEBREW_AGENTDECK_*`，再仅设置本次账户。归档准备不能复用后续某个安装场景的账户，也不修改全局 `base_env`。
- 真实 helper 继续生成 bash、zsh、fish 补全，保留非空检查和 `0644` 权限，然后才执行签名。临时 HOME/账户在签名前清理，不被打入共享 App 或归档。`test-cask-real-brew.sh` 只增加对该函数的显式 `env=base_env` 传递；产品 Go、cask 模板、安装/卸载协议和实际场景断言均未修改。
- 新增 `test_real_helper_uses_private_completion_environment`：在原准备函数上先 RED，实际复制到临时 App 的验收 helper 在 `prepare_fixture_app` 第 16 行执行 `completion bash` 时以 exit 1 失败，与 CI 失败层一致；修复后真实 helper 的三种 shell 输出均成功。该用例同时保留缺失账户的拒绝对照、检查 HOME/账户分离、继承的修复/失败注入变量未泄漏、临时账户及 HOME 在每次调用后没有新增文件、临时目录按期清理、原始 Debug fixture 不变。
- 原夹具回归继续覆盖三个补全源、权限、签名前生成，以及空输出/纯空白/命令失败禁止签名；另验证显式 `base_env` 中无关标记保留、修复变量被过滤且调用者字典不被修改。测试替换的只有 `ditto` 与 `codesign`，真实 Go helper 不被 mock；未执行 Homebrew 或系统注册。

### 验证与交接

- 本轮定向验证为 `python3 -B scripts/test-cask-real-brew-fixture.py` 的 3 项顶层测试、`bash -n scripts/test-cask-real-brew.sh` 及嵌入 Python 驱动的 `ast.parse`。原始 real-brew 命令只允许在 GitHub 托管 macOS runner 执行，当前候选尚未提交/推送，不声称该命令或完整安装链已在修复后通过。
- WorkUnit 保持 `installed-state-ownership:installer-receipts`，新候选 ContentState 为 `installed-state-ownership:installer-receipts:repair-ci-43006525357-completion-env`。沿用 HEAD 加 LF、排序 path/blob 加 LF 的 49 路径指纹配方，评审载体不入集合。除上述三个 fixture/驱动文件及 tasks.md 状态投影外，其余范围内容与 `5804e5cf` 交付态相同。
- 原 Go 全量/race、安装 helper、身份绑定、I4/I5 局部与本地安装检查的输入未变，按原环境和范围复用；本轮刷新夹具的真实 helper 验证及 L0，不因阶段变化重复无关产品测试。Round 4 对原安装清理修复的 PASS 保留原范围，但不能替代本轮夹具修改的独立复评。
- 当前候选的独立复评和 real-brew 仍缺失，任务 Review 取消勾选并交接 `in_review`，不声称 Task、Topic 或来源缺陷完成。`5804e5cf` 的提交态 real-brew FAILED 仍是原事实，不能因本地回归通过而改写；常规 CI `43006525312` 仍等待原监控完成通知。
- 尚未提交、推送、重跑 CI、执行开发机真实 brew 或委派。必要复现、对照与修复事实直接保存在本节，不引用临时 helper 或日志作为持久证据。
- 最终 HEAD 为 `5804e5cfaad0d03566dfdadefdf46d6f29507282`，49 路径 SHA-256 为 `7eba17007b5fced759d0f27a7897a87003aa5ccf6e28a64763d7a9bd44f964fd`；实际差异只有前述四项范围路径，另有不入指纹的本评审载体。Go 1.27.3 darwin/amd64 下最终夹具回归 3 项通过（7.578 秒），脚本/嵌入驱动语法及文档 L0 均通过。
- 新增 9 个 CEv1 节点及 22 条关系，节点完整载荷、预检结果和关系端点逐项读回一致。固定 gate-status 对该候选返回 **NOT_VERIFIED**、`reason=null`、6/8 准则通过；仅缺 independent-review 和 real-brew，无当前适用失败、失效或未决影响。提交态原 CI 失败未覆盖或重标。

#### 下一步指令

在独立会话执行本轮夹具修复的复评；当前修复会话不能充当独立评审者。

```text
复评：installed-state-ownership / installer-receipts
WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership
```

## CI-cycle exception — 2026-11-11

- 用户在“修复、提交、推送、验证，3 轮”之后明确澄清：“我意思是不评审 直接进行ci验证”。该真实用户决定取代上方 Repair 4 的独立复评暂停指令：当前 installer-receipts 及其相关 CI 阻断最多执行 3 轮“必要诊断/修复 → 本地验证 → 提交 → 推送 → CI”，全部 CI 通过则提前停止；不委派、不执行独立评审。
- 这是提交前评审顺序的明确用户例外，不是新的独立 PASS。任务 Review 保持未勾选，CEv1 的 independent-review 继续记录未执行；不擅改全局 required 准则，不据此宣称 Task 门禁 VERIFIED 或关闭 Task/Topic/来源缺陷。本次用户已明确授权三轮内的正常提交与推送，不再重复索取相同授权；无强推、发布或开发机真实 brew 授权。
- 第 1 轮使用 Repair 4 的同一产品/测试内容和已验证结果，只新增本例外及 tasks.md 的状态投影；没有因授权澄清重跑未变的产品检查。提交后将复用的有界验证事实显式绑定到实际不可变提交/树，CI 结果只认对应 SHA。旧候选 `7eba17007b5fced759d0f27a7897a87003aa5ccf6e28a64763d7a9bd44f964fd` 的原状态和原观察不重标。

## CI cycle 1 — 2026-11-11

- 第 1 轮正常签名提交并推送 `690e373dac328561c29db37e5117bd74f84cf9f2`，树 `f10397822d1f085d6ea32751b73211a852e10f4a`，父提交 `5804e5cf`。5 项范围文件、完整英文提交说明、OpenCode 贡献 trailer、预期树和实际 SSH 签名均核验；推送后工作区干净，远端主题分支同步。
- 不可变交付态为 `installed-state-ownership:installer-receipts:delivery-690e373d`。新增 9 个节点及 22 条预检通过的关系均已读回；推送时固定查询 NOT_VERIFIED、6/8，仅缺未执行的 independent-review 及尚未取得的 real-brew。评审跳过按用户 CI-cycle exception，不伪造 PASS。
- real-brew run `43011524396`、job `130492029687` 在 UTC `2026-11-11T11:23:19Z` 至 `2026-11-11T11:28:03Z` 执行，结论 failure，实际 SHA 为上述 `690e373d`。夹具回归及 App/widget 构建成功；补全准备已通过，脚本打印 `Running lifecycle`，实际 brew 安装返回成功并进入 `Scenario.widget()`，因此原缺失账户拒绝不再是本次最早失败。
- 新失败是 `bash scripts/check-macos-widget-registration.sh <临时已安装 App>` 退出 1：`Widget must have exactly one registration at <本次 extension 路径>`，登记查询返回 `(no matches)`。此前 `pluginkit -a` 返回 0；这不证明扩展已被发现，也不证明注销/升级或后续场景成功。现有输出不足以区分签名/权限、扩展元数据、宿主发现或登记机制问题，本轮不猜测根因。
- 同 SHA 的常规 CI run `43011524430` 仍由既有异步监控跟踪。后续候选推送可能按配置取消该旧运行；未收回的整项工作流不记作通过。

## CI cycle 2 — 2026-11-11

### 已定位的分发检查缺陷

- **分类：测试 harness 的警告格式处理缺陷。** 上述 `5804e5cf` 的 desktop 失败发生于 `test-macos-distribution.sh` 的已知警告白名单。Homebrew 在 `GITHUB_ACTIONS` 环境下通过 workflow annotation 输出警告，正文的换行转义为 `%0A`；本地普通输出则带 `Warning:`，还可能附 ANSI 色彩。旧链路只逐字允许两条无色的 `Warning: Calling …`，所以 CI 中已明确允许的 classic flight 警告仍被拒绝；不含 `deprecat` 的未知 annotation 警告反而可能漏检。
- 判别证据为原 CI 的警告拒绝输出、Homebrew 的 `opoo → GitHub::Actions.puts_annotation_if_env_set! → Annotation#to_s` 格式实现，以及直接运行真实 shell 检查段的局部回归：普通允许消息通过，带上下文的 GitHub annotation 和 ANSI 变体在原实现以同一“Homebrew warned while loading”失败，未知 annotation 则错误成功。不是 cask 被拒绝加载，也不需要改变已经批准的 classic flight 设计。
- 检查现在先去除 ANSI、识别 warning/error annotation，并按 Homebrew 的规则只解码一次换行/百分号，再逐行严格匹配原先的两条允许消息。未知警告、错误、带隐藏第二条 warning 的 annotation、变更措辞的弃用消息仍失败；没有关闭 GitHub 注解、开启测试豁免环境或放宽为任意弃用警告。回归执行实际 shell 检查段，不维护第二份白名单实现。

### 小组件的下一步判别

- 小组件 `(no matches)` 保持 **未定位**。本轮只使同一个 `pluginkit -a` 输出 verbose 信息，并在原严格检查失败后读取这一个 fixture 的 host/extension 签名验证、签名权限和 Info.plist。原失败始终重新抛出，诊断失败也不覆盖它；不增加登记重试、sleep、宿主重登记、跳过或其他兜底。
- `verify_fixture_widget_registration` 的本地记录工具测试证明成功路径仍调用原精确路径检查器；add 拒绝和 checker 无匹配两类失败均保留原原因、捕获诊断、只执行一次 add。测试不触碰本机系统登记；真正的签名/登记差异需本轮托管 CI 的观测后再决定修复，不能把诊断增强宣称为小组件已修复。

### 验证与交付边界

- `python3 -B scripts/test-macos-distribution-cleanup.py`：最终 13 项通过（12.702 秒），包含原 8 项完整 EXIT 清理保护和新增 5 项警告格式/拒绝测试，后者还检查嵌套 annotation 和百分号不能被二次解码以隐藏警告。`python3 -B scripts/test-cask-real-brew-fixture.py`：5 项通过（7.312 秒），保留真实验收 helper 补全及失败前签名边界。两个 shell 脚本的 `bash -n` 和 real-brew 嵌入 Python 的 `ast.parse` 通过。
- 候选只改 CI 测试/诊断和必要记录；Go 产品、Go 测试、安装/清理协议、模板、依赖及分派契约未改变，原有效的有界 Go/安装事实继续按原环境复用，不重新执行全量 Go 或本机真实 brew。完整 `make check-macos-distribution` 和 real-brew 命令留给托管 CI，本地局部回归不替代它们。
- WorkUnit 仍为 `installed-state-ownership:installer-receipts`；第 2 轮交付态预留 ID 为 `installed-state-ownership:installer-receipts:ci-cycle2-20261111`，将在实际提交后绑定不可变 SHA/树。范围在原 49 项上加入本次修改的 `scripts/test-macos-distribution-cleanup.py`，共 50 项；评审载体仍不入集合。independent-review 按用户决定未执行，真实 CI 仍待本候选推送后取证；不宣称 Task/Topic 或来源缺陷完成。

### 第 2 轮实际结果

- 已正常签名提交并推送 `51f2a913246b2d7f15e068b331c84e9192dfbc29`，树 `5aaf6a74e9aa59840125739a606aabf1f3b08a32`，父提交 `690e373d`。完整暂存差异、7 项范围文件、英文说明/OpenCode trailer、实际树和 SSH 签名均核验；推送后工作区干净、主题分支同步。
- 第 1 轮常规 CI `43011524430` 已被新推送自动取消：classify/documentation 及其两项文档检查成功，desktop Swift 和 Go Verify 被取消，distribution 跳过；不记作全量 Go/原生或整个 CI 通过。
- 第 2 轮提交态已绑定到本节预留的 ContentState。连同第 1 轮 real-brew 失败，共新增 10 节点/25 关系，完整载荷、预检及端点读回一致；固定查询第 1 轮目标 FAILED、第 2 轮目标 NOT_VERIFIED（均 6/8），后者当时尚未取得 real-brew 结果，independent-review 按用户决定未执行，无新目标适用失败或未决影响。
- real-brew run `43014024489`、job `130500480350` 在 UTC `2026-11-11T11:41:33Z` 至 `2026-11-11T11:45:43Z` 执行，结论 failure，实际 SHA 为 `51f2a913`。夹具回归和 App 构建成功；仍在 lifecycle 的精确 widget 登记检查失败、返回 `(no matches)`。
- 新诊断显示：App 与 widget 均通过 `codesign --verify --deep --strict`，但都是 ad-hoc、`TeamIdentifier=not set`，权限显示没有任何 entitlement；Info.plist 中的宿主/扩展 ID 是各自的 Debug 身份，两者声明的 App Group 都是 `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug`，扩展点为 `com.apple.widgetkit-extension`。因此磁盘签名损坏和两份 Info 的 Group 不一致不是本次已观察到的问题；必需的 sandbox/Group 权限未被签入是一个可直接确认的夹具缺陷，不代表其他登记前提已经成立。
- 第 2 轮常规 CI `43014024384` 仍在异步跟踪。最终一轮推送可能按既有并发策略取消旧运行；未取得的结果仍不能记作通过。

## CI cycle 3 — 2026-11-11

### 已证实的签名夹具缺陷与修复

- **分类：CI 夹具签名缺陷。** `build-macos-app.sh` 明确以 `CODE_SIGNING_ALLOWED=NO` 构建 Debug App；此前准备函数只调用不带权限文件的 `codesign --force --deep --sign -`，没有执行真实打包脚本的权限解析和逐层签名。仓库 `AgentDeckWidget.entitlements` 明确要求 App Sandbox 和单一 App Group，而 `AgentDeck.entitlements` 要求宿主 Group、宿主不 sandbox。第 2 轮诊断证明这两份权限均未进入 fixture 签名。缺少必需权限的事实已确定，但尚不把它宣称为 `(no matches)` 的唯一根因。
- 修复仅在 cask 夹具准备中读取已有两份权限模板，从已构建的 host/widget Info.plist 取得一致且已解析的 Debug Group，以结构化 plist 写出临时权限文件。空值、未展开的变量或两份 Group 不一致，在任何签名前拒绝；不把默认 Release Group 硬编码到 Debug fixture，不修改模板或任何产品身份。
- 按既有打包契约依次签 helper、顶层 frameworks、widget、host；widget/host 各携带对应的解析后权限，使用 DER 权限编码，禁止用 `--deep` 重签子项。最后仅以 `--verify --deep --strict` 验证整个 bundle，临时权限文件随准备结束清理。嵌套签名失败不再签宿主，不归档未完成的 App。仍是 ad-hoc 测试身份，无 Developer ID、notarization、门户或凭据操作。
- 登记检查继续精确要求唯一目标路径，没有 sleep、重试、宿主重登记或错误兜底。失败诊断改为 `codesign --display --entitlements - --xml`：本机当前 codesign 手册明确区分默认的抽象显示与 XML；该变化只使读回格式确定，并避免旧 `:-` 参数的弃用警告。

### 验证与限度

- 新 inside-out 回归在原准备函数上 RED：实际记录只有一次无权限的整包 `--deep` 签名，而非所需的四层及权限。最终 `python3 -B scripts/test-cask-real-brew-fixture.py` 的 8 项通过（5.598 秒）：新增正确模板/Group/顺序、无效 Group 禁止签名、嵌套失败禁止宿主封印；原真实 helper 补全、空/失败输出禁止签名和严格登记失败保护保持。
- 一个仅作用于临时普通 Mach-O 文件的原生 codesign 对照，保持同一二进制与 ad-hoc 身份：不带权限文件的签名及 strict verify 均 exit 0，但 XML 读回没有 sandbox/Group；仅显式加入测试权限及 DER 编码后，两命令仍 exit 0，读回 `com.apple.security.app-sandbox=true` 和预设测试 Group，字典与输入完全相等。未执行这个二进制、未操作系统登记或 Homebrew；该对照只证明权限编码，不证明 App Group 可访问或插件登记成功。
- `bash scripts/check-widget-sandbox.sh` 通过，确认沿用的原生模板和 Debug/Release 身份边界未变。第 2 轮警告/EXIT 清理的 13 项输入及脚本未变，复用既有通过结果；没有重复全量 Go/Xcode 检查，产品 Go/Swift、安装协议、依赖和 cask 声明均未改动。完整托管 CI 是修复后真实登记和完整安装链的唯一新增运行验证，当前不得记作通过。
- WorkUnit 保持 `installed-state-ownership:installer-receipts`；最终一轮交付态预留 ID 为 `installed-state-ownership:installer-receipts:ci-cycle3-20261111`，提交后绑定实际 SHA/树。在上轮 50 项范围上加入实际消费的两份原生权限模板，共 52 项；评审载体仍不入集合。independent-review 按用户决定未执行，不制造 PASS/VERIFIED。
- 本轮是已授权三轮中的最后一轮，推送后收回常规 CI 与 real-brew 的实际结论。即使仍有失败，也不自动进行第 4 轮修复、提交或推送；保留精确失败及后续前提，Task、Topic 和来源缺陷保持开放。

### 最终提交及 real-brew 结果

- 最终签名提交并正常推送 `760d2141ba0976bf69d63eef5e2d1af17b10e308`，树 `eb2e8ef679032ebd232fd65710ea05b3d0f382cb`，父提交 `51f2a913`。4 项范围文件、完整英文说明/OpenCode trailer、预期暂存树、实际提交及 SSH 签名全部核验；推送后工作区干净且远端同步。三轮正常提交/推送已全部执行，没有独立评审。
- 第 2 轮常规 CI `43014024384` 最终为 cancelled：classify/documentation 成功，desktop job `130500966517` 整体 success，含 Swift 构建/单元测试、desktop distribution 和文档要求；Go Verify 于 UTC `2026-11-11T12:02:42Z` 被最终推送自动取消。该结果证实警告处理修复已在真实 CI 的完整 desktop 路径通过，但不是 Go 或最后 SHA 的全工作流通过。
- 第 2 轮 real-brew 失败及本轮不可变交付态共新增 10 个 CEv1 节点、25 条关系；完整载荷和预检后的端点读回一致。固定查询第 2 轮目标 FAILED、第 3 轮目标当时 NOT_VERIFIED（均 6/8），后者只缺按用户决定未执行的 independent-review 和尚待执行的 real-brew，无适用失败或未决影响。历史事实不覆盖。
- 最终 real-brew run `43016890483`、job `130510085144`，实际 SHA 为上述 `760d2141`；job 在 UTC `2026-11-11T12:02:27Z` 至 `2026-11-11T12:10:19Z` 执行，结论 **failure**。fixture 回归及 App/widget 构建均成功，随后原 real-brew 命令退出 1。
- 失败为 `subprocess.TimeoutExpired`：隔离 `brew upgrade --cask --appdir=<本次临时 appdir> agentdeck/test/agentdeck-app` 超过原定 180 秒。Traceback 位于嵌入驱动第 224 行的 `s.command('upgrade')`，经 `Scenario.command` 调用 `run`，不是之前的 widget `(no matches)` 错误。
- 核对该 SHA 的真实驱动顺序可知，在进入此 upgrade 之前，`brew install`、原精确路径唯一 widget 登记检查、CLI 与三种补全链接断言、同版本 `brew reinstall`、新 installation ID/attempt 断言及原清单/helper 不变断言均已返回通过。该进展是这次真实运行的先行断言结果，不从本地签名对照推断。升级完成、升级后的登记/卸载、后续恢复与竞争/延迟清理场景仍未取得通过证据。
- 当前公开失败输出没有包含 upgrade 子进程已捕获的中间 stdout、进程等待栈或超时时对象状态，不能据此把原因定为锁死、Homebrew 网络/调度或单纯预算不足。本轮不诊断更多根因、不增加 timeout/sleep、不跳过升级、不重跑 CI，也不进行第 4 轮修复或 Git 交付。
- 最终常规 CI run `43016888177` 仍待原异步监控回收；未宣称全量 Go、整个常规 CI 或本任务完成。本次实际结果回填只修改本评审载体与 tasks.md，是最终 SHA 之外的本地未提交记录，不更改已运行内容或追加推送。
- 最终 real-brew 失败新增 1 个 evidence 节点和 3 条关系，完整节点/端点及关系预检均已核验。固定 gate-status 对 `ci-cycle3-20261111` 返回 **FAILED**、`reason=null`：6/8 准则仍有有效通过事实，real-brew 有当前适用失败，independent-review 按用户决定未执行，未决影响为空。原 real-brew pending 占位显式 supersede 后保留，不覆盖历史状态或把先行场景通过当作整个准则通过。

## Additional CI cycles — 2026-11-11

- 用户在第三轮 real-brew 升级超时后明确要求“再继续3轮”。该新授权取代上方“不得追加第 4 轮”的交付上限，允许继续整体第 **4–6 轮**，各轮仍按已明确的“不评审、不委派，修复/必要验证 → 正常提交 → 推送 → 精确 SHA 的 CI”执行；全部 CI 通过则提前停止。第 6 轮之后不再自动追加。
- 范围仍限 installer-receipts 及相关 CI 阻断，无强推、发布、凭据、门户或开发机真实 Homebrew/系统登记授权。旧轮次的内容身份、失败和未执行评审状态不覆盖；新授权不等同于旧 CI 通过。

## CI cycle 4 — 2026-11-11

### 升级超时的诊断缺口

- 第 3 轮的 upgrade 超时根因仍为 **未定位**。已确认的 harness 缺陷是：原 `run` 使用 `subprocess.run(..., stdout=PIPE, stderr=STDOUT)` 捕获输出，但未处理 `TimeoutExpired.output`；异常默认字符串只显示命令及预算，所以 CI traceback 丢失了升级过程中已经产生的输出。当前证据不足以修改安装协议或增大 180 秒预算。
- 新回归直接从真实 CI 驱动 AST 提取并执行 `run` 函数，记录 subprocess 边界，不复制第二套 wrapper。原实现对 bytes/str 的已捕获升级阶段标记均 RED，错误字符串中没有标记；空输出同样没有明确说明。成功/预期非零/非预期退出对照按原契约通过。
- 将执行边界提取为 `run_fixture_command`，原 CI `run` 只透传相同参数和 `base_env`。超时后保留命令、原 timeout 数值、已捕获 stdout/stderr，兼容 bytes/str 与非 UTF-8 内容；没有输出则明确报告。新 AssertionError 通过 `from` 保留原 TimeoutExpired 原因链；即使调用者预期非零退出，超时仍失败。
- 本轮只补齐诊断，不宣称 upgrade 已修复：没有增加 timeout、sleep、重试、改变 brew 参数/版本、成功判定或任何生命周期断言，也没有输出环境变量或读取凭据。下一次托管 CI 用原预算获取最早可见升级阶段，再按证据决定修复。

### 验证与交付

- `python3 -B scripts/test-cask-real-brew-fixture.py`：11 项通过（9.871 秒），包含原 8 项夹具/签名/登记保护及新增 3 项真实 wrapper 边界测试；覆盖 bytes、str、非法 UTF-8、空输出、预期失败仍拒绝 timeout，以及原环境、argv、显式/默认预算和退出语义。`bash -n scripts/test-cask-real-brew.sh` 通过，测试 AST 同时解析完整嵌入 Python。
- 第 3 轮常规 CI 监控曾因 GitHub API EOF 退出，不能据此判定 CI 失败。一次实际状态读回确认 run `43016888177` 仍为 in_progress，documentation 和 desktop job `130510569319` 已 success（含 Swift、sandbox、分发及文档要求），Go Verify 当时仍在运行；已重新挂接只读监控，不重跑 CI。
- 本轮 Go/Swift 产品、原生权限模板、安装协议、cask 声明、依赖及第 2 轮警告/EXIT 清理路径均未变，复用其原有有界证据。11 项局部回归不代替完整 real-brew 或常规 CI，之前的 upgrade 失败也不重标为本候选的新运行结果。
- WorkUnit 保持 `installed-state-ownership:installer-receipts`，新交付态预留 ID 为 `installed-state-ownership:installer-receipts:ci-cycle4-20261111`，提交后绑定实际 SHA/树；仍为 52 项范围，评审载体不入集合。independent-review 按用户决定未执行；真实验收等待本轮实际 SHA 的 CI，不制造 PASS/VERIFIED，不关闭 Task/Topic/来源缺陷。
