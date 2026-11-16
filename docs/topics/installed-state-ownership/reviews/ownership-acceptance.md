---
status: active
topic: installed-state-ownership
subject: ownership-acceptance
---

# Ownership Acceptance Review

## Round 1 — 2026-11-15

## 📋 任务 8 独立评审

📊 总体评分：6/10

✅ 评审结论：FAIL

Reviewer: Codex，独立于本候选开发；未委派。
Method: 契约、候选差异与隔离边界检查；真实验收 helper 和真实安装脚本的最小复现。出现决定性阻断后停止扩大验证。
Scope: `scripts/test-ownership-acceptance.sh`、Makefile 验收入口、`ownership_guard_acceptance_test.go` 的两处夹具改动及 Task 8 交接证据。生产代码、测试和配置只读；未运行真实 brew、注册或启动 App。

Reviewed state: HEAD `d3b965ffc9a06a239544f65154ebed5d6fe22703`；开发候选 ContentState `installed-state-ownership:ownership-acceptance:dev-20261115-lw8o7oEJ`；18 项清单 SHA-256 `8af49e511273769349dbd215d3587cc3b027674506c2b871c3c1d8586fecf20e`。逐项重算 Git blob 与清单摘要全部一致。评审后状态文档更新的 ContentState 见本轮末尾。

### 🔴 严重问题 — 必须修复

**ISO-OA-R1-F1 · P1 · OPEN — 验收安装链继承外部安装目标，不能保证临时目录隔离。**

位置：`scripts/test-ownership-acceptance.sh:174` 的 `manage()`，以及同文件直接设置 PREFIX 后调用 `manage-install.sh` 的各场景。

- 行为风险：入口只覆盖 `PREFIX`，没有覆盖或清除调用环境中的 `BINDIR`、`DATADIR`。实际安装器 `scripts/manage-install.sh:661–663` 优先使用这两个变量，`install_agentdeck` 会在那里建立目录、发布验收程序并写安装清单。因此执行本验收入口可能写入调用者指定的真实安装目录；脚本后续对 `$temporary/prefix` 的断言无法阻止先前的写入，EXIT 清理也只覆盖自身临时目录。这违反 Task 8 的隔离边界，与安装器允许自定义目录本身无关。
- 决定性证据：从候选逐字提取 `manage()` 函数，以临时目录 A 为 `$temporary`，临时目录 B 为继承的 `BINDIR=B/bin`、`DATADIR=B/data`；运行真实 `manage install <验收二进制>`。结果 exit 0，`B/bin/agentdeck` 和 `B/data/install-manifest` 存在，`A/prefix/bin/agentdeck` 不存在。A、B 均位于本次独立临时根，复现后完整删除；未使用任何真实安装目标。
- 复现构建：Go 1.27.3，`go build -mod=vendor -tags agentdeck_acceptance -o <fixture>/acceptance ./cmd/agentdeck`；`HOME`、`AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 均在 A 内，`COMPLETION_SHELL=none`，清除 `FORCE`、`TAKEOVER`、`COMPLETION_RC` 后调用上述原函数。此复现只验证安装目标继承，不声称执行了完整跨面或访问屏障验收。

💡 修复范围：验收入口统一钉住安装目标环境，覆盖所有实际安装调用，包括未经过 `manage()` 的直接调用；明确控制会改变场景语义的继承变量。增加污染环境回归，断言外部目录/哨兵完全不变、所有安装产物只落在声明的临时夹具内。无需改变产品安装器的自定义目录契约。

### 🟡 建议改进 — 推荐

无额外发现。A5 的真实双 App 运行，以及 requirements.md 的 cask 范围修订与评审，是交接中已明确保留的完成前提，不作为本轮新发现，也不视为已豁免。

### 🟢 优点

交接明确区分真实安装脚本、cask 协议替身、原生夹具与生产静态证据；18 项候选内容指纹可复算，且没有以替身宣称真实双 App 验收完成。

### 📝 总结

隔离边界已有可重复的外部写入反例，本轮 FAIL。Task 8 Review 保持未勾选，交回同一任务修复；Dev 的历史交接保留。任务和主题均未完成，不建议提交或推送。

Evidence:

- 原始候选 gate-status 固定模板查询为 NOT_VERIFIED：9 项必需准则中 6 项有通过证据，缺 `independent-review`、`a5-app`、`acceptance-ledger`；invalidated/unresolved 均为空。
- HEAD、18 项 blob 与 SHA-256 全部匹配开发 ContentState。`git diff --name-only 98c0398 HEAD -- cmd internal apps/macos go.mod go.sum vendor scripts/test-cask-real-brew.sh` 无输出；未据此把先前 CI 扩大为新脚本的污染环境覆盖。
- 最小安装复现 exit 0，外部临时目录中的程序/清单存在，声明目标不存在。复现材料已清理，事实与方法完整保存在本记录中。
- 首次复现构建因 PATH 的 Go 1.27.1 与 GOROOT 的 1.27.3 混用失败；改用 GOROOT 下 1.27.3 的明确可执行路径，并设置临时 GOCACHE/GOMODCACHE 后完成复现。此环境问题不计作候选缺陷。
- 决定性复现后未重跑完整矩阵、race、双架构构建或真实 App/brew；历史通过结果保留其原始范围。

完成门禁：FAILED

评审后证据绑定：ContentState `installed-state-ownership:ownership-acceptance:review-r1-20261115-0jlCxNPr`，18 项清单 SHA-256 `4e793e71850701d1b2d29ed820530e79bc93c0f9eec234d0bdb0883862f0d4f1`。相对开发态仅 tasks.md 追加评审交接；17 项代码/测试/构建/安装输入全部不变。原始六项通过观察按原范围复用到此状态，独立评审失败单独记录，A5 与验收结论准则保持缺失。

下一步：修复 `ISO-OA-R1-F1` 后独立复评；A5 与需求文本前提仍保持开放。

## Repair 1 — 2026-11-15

修复者：OpenCode（GPT-6.1 Sol），未委派。本节是修复交接，不是独立复评，不修改 Round 1 的 FAIL/FAILED 历史结论。

### 发现与改动

`ISO-OA-R1-F1` → repaired in candidate，待独立复评确认。

- 确认原入口仅覆盖 PREFIX，而真实 `manage-install.sh` 优先消费继承的 BINDIR/DATADIR；原入口还会继承 FORCE/TAKEOVER，改变场景语义。
- 为所有 make/本地 App 安装、接管和卸载调用提供同一个 `install_env` 边界，包括原先绕开 `manage()` 的直接调用。环境固定到当前临时账户与安装夹具，BINDIR/DATADIR 不再由调用者控制，安装器的工作目录也放在该夹具内。LSREGISTER 和 COMPLETION_SHELL 为 none，COMPLETION_RC 固定为临时 HOME 下的路径；FORCE/TAKEOVER 默认 0，只接受场景显式开启。
- 新增污染环境回归入口 `scripts/test-ownership-acceptance-environment.sh`，并接入 `make check-ownership-acceptance`。回归通过真实完整脚本执行安装链，而非提取函数或替换安装器。污染目录及哨兵是临时兄弟夹具；运行前后比较整棵外部树，包含条目、文件摘要、模式、inode、uid/gid、mtime/ctime，并检查验收临时产物清理。新增“未显式 FORCE 的重装仍拒绝、回执不变”断言。
- 未修改产品安装器、Go 产品代码或原有 Go 夹具；原开发者的两处 Go 夹具改动保留。

### 验证

- 环境：Go 1.27.3 darwin/amd64，使用其匹配的 GOROOT；本机 `sandbox-exec` 的正负对照可用。全程仅临时账户/目录与验收二进制，没有真实安装目标、真实 brew 或 App 注册/启动。
- 修复前运行新增回归脚本，exit 1：场景 3 在继承 TAKEOVER=1 时意外 exit 0（预期 1），外部临时 BINDIR 下出现 agentdeck，DATADIR 下出现 cleanup 记录与 install-manifest；外部树比较同时失败。临时材料由 trap 清理。这是对原发现及场景污染的实际复现。
- 修复后 `make check-ownership-acceptance` exit 0：在同一污染环境下完整通过场景 1–8（含 6a）；未请求接管、持锁接管、停止后显式接管、未显式 FORCE 重装、显式 FORCE/TAKEOVER、make/本地 App 完整卸载链、formula 续登/多安装拒绝、cask 登记协议替身和损坏回执均满足断言。全部不访问注入真实状态的观察仍在访问屏障及正负对照下运行。
- 外部树快照完全一致；注册替身未执行、补全哨兵未变；所有安装产物落在声明的临时夹具内，结束后该夹具根为空。
- `bash -n scripts/test-ownership-acceptance.sh scripts/test-ownership-acceptance-environment.sh`、`python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/ownership-acceptance.md`、`make check-whitespace`、`git diff --check` 均通过。
- 改变的安装环境边界已重新执行完整串联；未变的 Go 叶子矩阵、A8、生产静态产物检查按开发交接和 Round 1 的原范围复用，不重跑无变化的全量 Go/race、原生或双架构构建。cask 文件动作仍是替身，绝不据此声称真实 Homebrew 或真实双 App 验收通过。

### 交接边界

当前修复候选仍需独立复评。A5 的真实双 App 运行或明确豁免、requirements.md 的 cask 范围文本修订及其评审继续开放；不将这些前提改为通过，不勾选任务 8 Review，不建议提交或推送。

### 精确内容态与证据

- WorkUnit：`installed-state-ownership:ownership-acceptance`。
- ContentState：`installed-state-ownership:ownership-acceptance:repair-r1-f1-20261115-gEbmL3KG`。
- HEAD：`d3b965ffc9a06a239544f65154ebed5d6fe22703`；19 项清单 SHA-256：`1705200f855655de492441befdd119845350ed37b1ddf259a8e37b8bf77b890e`。
- 沿用 `head=<HEAD>\n` 加按路径排序的 `<path>=<git-blob>\n` 配方；原 18 项增加污染环境回归入口，完整 `subject_blobs` 保存在该 ContentState。评审记录不进入自身指纹。相对 Round 1 内容态，15 项不变，三个原有输入变化及新增项如下；tasks.md 已完成本轮交接同步后才计算指纹。

  | 输入 | 修复候选 Git blob |
  | --- | --- |
  | `Makefile` | `283fbbfede8b0421ba9436cc4d86a741d2217115` |
  | `docs/topics/installed-state-ownership/tasks.md` | `0ba6f4a033250f95b8584d3f2dcbe307d27410b7` |
  | `scripts/test-ownership-acceptance.sh` | `04fbd9729b4e947558417cd66edb23dff1d3ab3b` |
  | `scripts/test-ownership-acceptance-environment.sh`（新增） | `ff35055e0f8d49e783612da285e4b69ec8bc7e00` |

- 记录新串联、channel 与 validation 观察；叶子矩阵、A8、生产静态三项分别作范围影响判断与目标态 roll-up，原始观察保持原身份。节点写入后逐项读回 11/11；23 条关系预检全为 ok，创建数与提交数均为 23，端点和类型读回完全一致。首次计数处理误将写工具的 counters 当作结果行，未重放节点写入，先读回确认再完成关系批次。
- 固定模板查询该精确目标后为 NOT_VERIFIED：9 项必需准则中 6 项已有通过证据，仍缺 `independent-review`、`a5-app`、`acceptance-ledger`，无失效证据或未解决影响。旧评审失败不被重写，也没有把修复者的验证充作独立复评。

```json
{
  "work_unit_id": "installed-state-ownership:ownership-acceptance",
  "target_content_state": "installed-state-ownership:ownership-acceptance:repair-r1-f1-20261115-gEbmL3KG",
  "missing_criteria": ["independent-review", "a5-app", "acceptance-ledger"],
  "invalidated_evidence": [],
  "unresolved_candidate_impacts": [],
  "allowed_scope": "ISO-OA-R1-F1 修复、定向验证和交接；独立复评须另有用户阶段指令",
  "authority_ceiling": "无独立评审裁决、真实 App/brew 操作或 Git 交付授权"
}
```

## Round 2 — 2026-11-15

## 📋 ISO-OA-R1-F1 独立复评

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer: Codex；独立于 OpenCode 修复者，未委派。
Method: 逐条核对原发现、当前源码、全部安装调用入口、污染环境回归断言与精确内容态证据。生产代码、测试和配置只读。
Scope: 仅 ISO-OA-R1-F1 及修复影响，不将真实双 App 或需求文本前提纳入已完成声明。

### 🔴 严重问题 — 必须修复

无。ISO-OA-R1-F1 CLOSED；没有新增阻断发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `scripts/test-ownership-acceptance.sh:174` 的 `install_env` 固定 HOME、账户、TMPDIR、PREFIX/BINDIR/DATADIR、APPDIR、链接及记录目录；所有 make/本地 App 安装、接管、卸载调用均经过该边界。原先绕过包装的直接调用也已转换。
- FORCE/TAKEOVER 默认 0，场景显式参数位于 env 默认赋值之后，保留需要的显式接管语义；未显式 FORCE 重装新增拒绝和回执不变断言。
- 新回归入口给完整验收脚本注入外部临时路径、FORCE/TAKEOVER、补全和注册污染，运行前后比较整棵外部树的条目、内容、模式、inode、属主及 mtime/ctime，并验证夹具清理；Makefile 的验收目标已调用该入口。没有以函数替身代替完整链。

### 📝 总结

ISO-OA-R1-F1 已关闭，复评 PASS。原始 Round 1 FAIL 与 Repair 1 历史保留。A5 和需求修订属于仍未满足的任务完成条件，PASS 不代表 Task 8 或主题完成。

Reviewed state: HEAD `d3b965ffc9a06a239544f65154ebed5d6fe22703`；修复 ContentState `installed-state-ownership:ownership-acceptance:repair-r1-f1-20261115-gEbmL3KG`，19 项 SHA-256 `1705200f855655de492441befdd119845350ed37b1ddf259a8e37b8bf77b890e`。逐项重新计算全部 19 个 Git blob 和整体摘要，完全一致。

Evidence:

- 固定 gate-status 查询修复态：NOT_VERIFIED，六项通过证据有效，缺 independent-review、a5-app、acceptance-ledger，无 invalidated/unresolved。
- 复核 Repair 1 的 validation/cross-surface/channel 观察：污染环境下旧脚本失败且发生外部写入；修复后 `make check-ownership-acceptance` exit 0，完整场景及外部树不变/清理断言通过。当前脚本与这些观察的内容态一致，复用其运行结果，不重复执行同态验收。
- 逐个核对全部 `manage-install.sh`、`install-local-app.sh` 调用，没有绕过 install_env 的入口；源码确认 env 的显式场景覆盖顺序和 FORCE 拒绝断言。未对原生/真实 Homebrew 生成新运行证据。
- 原叶子矩阵、A8、生产静态证据按修复态已有范围评估复用；本轮不重跑无变化的 Go/race/构建。

完成门禁：NOT_VERIFIED

最终 ContentState：`installed-state-ownership:ownership-acceptance:rereview-r2-20261115-n7SknmKZ`；19 项 SHA-256 `e558dfa9db3cb33e7a4f2022fb9843ff81ba0d07ae255c7518789f80a02edf4a`。相对修复候选仅 tasks.md 的复评状态同步变化，其余 18 项全部不变；六项原观察作目标态 roll-up，新增独立复评 PASS。仍缺 a5-app、acceptance-ledger，保持任务 in_review。

Task checkpoint：ad-installed-state-ownership-ownership-acceptance；上述最终内容态；gate NOT_VERIFIED。
提交建议：门禁 VERIFIED 后，经用户授权提交 Task 8 验收脚本、污染环境回归、Makefile、原 Go 夹具及相应任务/评审记录；前提为 A5 实测或明确豁免，以及 cask 需求修订和评审完成。
推送建议：门禁 VERIFIED 后，经用户另行授权推送 origin/feature/installed-state-ownership；同样先补齐 A5 与需求修订前提，再验证提交范围、消息、签名和交付检查。

下一步：补齐 A5 真实双 App 验收或明确豁免，并完成 requirements.md 的 cask 范围修订及独立评审；不重复本轮复评，不启动 Task 9。

## 验收前提补齐 — 2026-11-15

本节仅同步已完成复评之后的验收前提，不启动新评审轮次。Round 2 PASS / ISO-OA-R1-F1 CLOSED 保持；该轮原内容态 NOT_VERIFIED 是历史事实。

- 用户本次明确选择「明确豁免本次真实双 App 实测」：只豁免 A5 的真实双 App 同时运行验证，接受既有原生锁/helper、Debug 身份隔离和安装链替身证据；真实 App Group 容器、偏好域、小组件共存仍未新增实测，保留明确接受的风险。未启动/注册/安装真实 App，未运行真实 brew 或访问真实状态。
- 可行性核对：AgentDeckApp 的正式装配采用 NativeOwnershipSession.systemAccountHome（系统账户），AppGroupSnapshotStore 通过系统 App Group 获取容器；Debug 测试的临时 home/存储装配不等于真实正式身份双 App 验收。未通过仅改 HOME 宣称完成。
- 需求 cask 修订完成六处已批准调整，A2/A7 未改。用户明确授权独立冷上下文代理只读审查，主会话验证其报告、文件身份和架构依据后记录 requirements.md Round 7 PASS；文档门禁 VERIFIED 2/2。文档 ContentState `installed-state-ownership:req:cask-scope-review-r7-20261115-3515a41c`，fingerprint `3515a41c135c97ed6146413715e84add0963af5c131aa77a2c50519343b4c45b`。
- 最新 A1–A9 账本见 tasks.md「验收前提补齐」。A5 标为用户豁免，其他项使用原始运行证据与当前批准的范围；不把需求文本或豁免充作新增运行观察。

最终任务内容态：`installed-state-ownership:ownership-acceptance:prerequisites-20261115-d5cf70cf`；HEAD `d3b965ffc9a06a239544f65154ebed5d6fe22703`；19 项 SHA-256 `d5cf70cf61729e2ece1893d9cd0787f07e070cd0862028eb2d9d01901423be5b`。沿用已有 19 项配方，复评后仅 tasks.md 的前提、账本与状态变化，其余 18 项完全不变；需求文档及独立评审通过 depends_on 关系显式绑定，未偷偷扩充旧指纹配方。

最终固定模板查询结果为 VERIFIED 9/9，missing/invalidated/unresolved 均为空。七项有效复评态证据按相同运行内容和明确范围复用；新增 a5-app 用户豁免观察及 acceptance-ledger 核对观察。任务及文档修订的 Beads 状态为 awaiting_commit；未做 Git 交付，Task 9、Topic、来源缺陷仍开放。

Task checkpoint：ad-installed-state-ownership-ownership-acceptance；content_state=d5cf70cf61729e2ece1893d9cd0787f07e070cd0862028eb2d9d01901423be5b；gate=VERIFIED。
提交建议：用户授权后交付 Task 8 原候选、修复、任务账本与本评审记录；需求修订任务单独一个逻辑提交，按各任务拆分 tasks.md 状态片段。
推送建议：origin/feature/installed-state-ownership，需另获授权并验证实际提交范围、消息、贡献者、签名及交付检查；本次未提交或推送。
