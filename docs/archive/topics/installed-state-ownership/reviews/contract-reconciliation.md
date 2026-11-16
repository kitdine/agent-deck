---
status: historical
topic: installed-state-ownership
subject: contract-reconciliation
retired: 2026-11-16
---

# Contract Reconciliation Review

## Round 1 — 2026-11-16

## 📋 Task 9 稳定契约与用户文档评审

📊 总体评分：7/10

✅ 评审结论：FAIL

Reviewer: Codex，独立于 OpenCode 实现者；未委派。
Method: 文档差异与任务要求对照，安装命令和构建配置源码核查，Make dry-run 静态复现；决定性阻断后停止扩大验证。
Scope: README.md、docs/specs/cli-design.md、docs/specs/cli-manual.md 的 Task 9 候选及交接。产品、测试、配置与被评审文档均未修改；仅更新本记录和 tasks.md。

Reviewed state: HEAD `a289f66b567b72ceee248c781717f9eea541f5e5`；开发 ContentState `installed-state-ownership:contract-reconciliation:dev-20261116`；8 项 SHA-256 `3c3f937310d6622d03d2771094057d7bc1ea76196dbe483bacc97ab3d5af28e1`。所有输入 blob 与摘要重算完全一致。

### 🔴 严重问题 — 必须修复

**ISO-CR-R1-F1 · P2 · OPEN — 本地 App 的构建示例与安装目标不一致。**

位置：`README.md:323–327`、`docs/specs/cli-manual.md:107–108`。

- 行为风险：README 声称先构建 Release App，却指导运行 `make build-macos-app`；手册给出同一命令链。`scripts/build-macos-app.sh:21` 的默认配置为 Debug，而 Makefile 的 MACOS_APP 默认指向 Release/AgentDeck.app，安装目标并不先构建 Release。干净工作区缺少待安装对象；有旧 Release 目录时可能装入旧产物，而不是刚构建的代码。
- 证据：`make -n build-macos-app install-macos-app` 输出先执行 `bash scripts/build-macos-app.sh`（没有 Release 配置覆盖），再执行 `scripts/install-local-app.sh install "apps/macos/build/DerivedData/Build/Products/Release/AgentDeck.app"`。Makefile:74–82 区分两个构建目标；`build-macos-release: build-all` 才设置 AGENTDECK_APP_CONFIGURATION=Release，并准备脚本要求的双架构 helper。该反例由当前脚本和 Make 展开直接确定，无需执行真实安装。

💡 修复范围：把 README 与手册的默认本地 App 构建/安装示例统一到实际生成安装目标的 Release 构建入口（现有 `make build-macos-release`），或给出完整、彼此一致的配置与 MACOS_APP 覆盖方式。检查两份示例的构建产物和安装输入一致；不改变产品构建默认配置、不执行真实安装。

### 🟡 建议改进 — 推荐

无额外发现。未完成的广泛文档审查不应解读为其余内容已独立确认无缺陷。

### 🟢 优点

候选明确区分安装身份和开发状态，保留 cask 文件生命周期限制、A5 用户豁免及安装元数据不可便携边界，没有将文档检查写成新的真实 App/brew 验收。

### 📝 总结

操作员可直接复制的命令链不能生成默认安装对象，因此 FAIL。Task 9 Review 保持未勾选；不跨越最终 Topic 门禁，不宣布主题或来源缺陷完成。

Evidence:

- 开发精确态固定模板 gate-status 为 NOT_VERIFIED 4/5，缺 independent-review；8 项内容指纹完全匹配。
- `make -n build-macos-app install-macos-app` 与上述源码确认 Debug/Release 错配；未构建、注册、启动或安装 App。
- 开发记录中的四项 L0 通过观察只证明结构/链接/格式，不能排除此命令语义缺陷。未变化的检查按原范围复用，正式评审记录和状态同步后执行文档/记录结构检查。
- 在决定性阻断后停止扩大验证，不重跑 Go、原生、Homebrew 或完整验收。

完成门禁：FAILED

最终 ContentState：`installed-state-ownership:contract-reconciliation:review-r1-20261116-Bi4iremb`；8 项 SHA-256 `4662714199badfcfdddb7a6b1906473128fc15a7a4b016a540c6de6df913fe51`。相对开发态只有 tasks.md 追加本轮交接，其他 7 项不变。operator-guidance 与 independent-review 记录失败，其余开发观察只按原范围复用；旧观察保留原内容态。

下一步：修复 ISO-CR-R1-F1 后独立复评。没有提交或推送建议，亦未执行 Git 交付。

## Repair 1 — 2026-11-16

修复者：OpenCode（GPT-6.1 Sol），未委派。范围仅为 ISO-CR-R1-F1 的默认本地
App 构建/安装示例与必要交接；本节不是独立复评，不修改原 Round 1 FAIL/FAILED。

### 发现与改动

`ISO-CR-R1-F1` → repaired in candidate，等待独立复评确认。

- README 与 CLI 手册均将 `make build-macos-app` 改为现有的
  `make build-macos-release`，随后仍为 `make install-macos-app`。
- 两处均说明 Release 入口先构建双架构 CLI helper，产物与默认 MACOS_APP 一致；
  `build-macos-app` 的默认 Debug 行为不改，只明确它不用于该 Release 安装链。
- 未修改 Makefile、产品、测试或配置；cli-design.md 的开发候选原样保留。
  没有顺带修复其他问题，也不把决定性阻断后尚未完成的广泛文档审查当作已经通过。

### 静态复现与定向回归

- 原命令 `make -n build-macos-app install-macos-app` 只展开
  `bash scripts/build-macos-app.sh`，没有 Release 覆盖，而安装输入为
  `apps/macos/build/DerivedData/Build/Products/Release/AgentDeck.app`。
  结合脚本 `configuration=${AGENTDECK_APP_CONFIGURATION:-Debug}`，确认原反例仍成立。
- 修复后用 Python 从 README「Install a Locally Built App」及手册
  「源码 CLI 与本地 App 安装」提取首个 Bash 代码块，分别断言
  `make build-macos-release` 的下一条命令为 `make install-macos-app`，
  这两个代码块均不再包含原 Debug 构建命令。
- 同一次回归执行 `make -n build-macos-release install-macos-app`，exit 0。
  断言输出同时含 GOARCH=arm64、GOARCH=amd64、
  AGENTDECK_APP_CONFIGURATION=Release，并将安装脚本输入规范化后与默认 Release
  输出路径比较，相等。Makefile 的 build-all 前置依赖及脚本
  `$derived_data/Build/Products/$configuration/AgentDeck.app` 与该结果一致。
- 全程为源码核对、文档断言与 Make dry-run；没有执行展开的 mkdir/go build/xcodebuild
  或安装命令，不声称 App 构建、安装、注册、启动或真实双 App 验收通过。

### 交接与证据边界

修复候选等待独立复评；Dev 保持勾选，Review 保持未勾选。A5 的明确豁免、
Cask/PlugInKit 已记录限制与其他原验证边界均保持，不跨越 Task 9 或 Topic 完成门禁。
L0 验证均通过：`python3 scripts/check-docs.py`、
`bash scripts/check-topic-docs.sh installed-state-ownership`、
`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/contract-reconciliation.md`、
`make check-whitespace`、`git diff --check`。这些结果不代替独立复评。

精确修复内容态为
`installed-state-ownership:contract-reconciliation:repair-r1-f1-20261116`，
HEAD `a289f66b567b72ceee248c781717f9eea541f5e5`，沿用原 8 项配方，
SHA-256 `8c3debe13bb64680738643b7a26203cc6c287345fa763b963d3aa2a2a085aed6`。
相对 Round 1 仅 README、手册及 tasks.md 变化，其余 5 项完全不变；
完整 subject_blobs 在该 ContentState，评审记录不进入自身指纹。
记录新 operator-guidance 与 L0 观察；contract-coverage 和
compatibility-portability 按原有限范围作影响判断及目标态 roll-up，
不将尚未完成的广泛独立审查伪作通过。8 个新节点、18 条关系均核对创建数与提交数，
关系预检全为 ok，节点属性及关系端点/类型读回一致。

固定模板查询该修复态结果为 NOT_VERIFIED，4/5 项有通过证据，仅缺
independent-review；invalidated/unresolved 均为空。原 Round 1 FAIL 及原内容态
FAILED 保持，不由修复者自行作出 PASS 或宣告 Task 9/Topic 完成。

```json
{
  "work_unit_id": "installed-state-ownership:contract-reconciliation",
  "target_content_state": "installed-state-ownership:contract-reconciliation:repair-r1-f1-20261116",
  "missing_criteria": ["independent-review"],
  "invalidated_evidence": [],
  "unresolved_candidate_impacts": [],
  "allowed_scope": "本轮仅 ISO-CR-R1-F1 修复、定向验证与交接；独立复评须另获用户阶段指令",
  "authority_ceiling": "无独立评审裁决、真实 App 构建/安装或 Git 交付授权"
}
```

## Round 2 — 2026-11-16

## 📋 Task 9 独立复评与主题内容边界

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer: Codex，独立于修复者 OpenCode；未委派。
Method / Scope: 原发现逐条复核、修复源码/Make 默认值与产物路径核对，并补完首轮中止的文档契约检查；只读审阅三份产出文档，复用同态 L0 和定向静态回归，不执行真实构建/安装。

### 🔴 严重问题 — 必须修复

无。ISO-CR-R1-F1 CLOSED：README 和手册均改为先 build-macos-release 再 install-macos-app，明确 Debug 入口不用于该 Release 安装链。Makefile 的 Release 入口依赖 build-all 并设置 Release 配置，与脚本产物和 MACOS_APP 默认路径一致。

### 🟡 建议改进 — 推荐

无新增发现。

### 🟢 做得好的部分

两份示例保持一致并解释默认值，修复没有改变构建默认配置。文档对 formula 首次认领/续登、显式接管、cask 文件生命周期限制、A5 豁免及不可便携安装材料的说明保留了已批准的边界。

### 📝 总结

原发现已关闭。补查安装/恢复、默认与显式状态、客户端写入守卫、CLI 错误与诊断输出、App 路径和便携边界，未发现新的范围内缺陷。Round 1 的失败与当时尚未完成广泛审查的限制保留为历史事实。

Reviewed state: HEAD `a289f66b567b72ceee248c781717f9eea541f5e5`；修复态 `installed-state-ownership:contract-reconciliation:repair-r1-f1-20261116`，8 项 SHA-256 `8c3debe13bb64680738643b7a26203cc6c287345fa763b963d3aa2a2a085aed6`。本轮逐项重算全部 blob 与摘要，完全一致。

Evidence:

- 修复态固定门禁 NOT_VERIFIED 4/5，仅缺 independent-review，无失效或未解决影响；复用该态文档提取断言、make dry-run 和 L0 检查，未重复执行同态检查。
- 修复后 README/手册代码块都使用 Release 入口。Makefile:74–82 与 build-macos-app.sh 默认配置、Release helper 前提、最终 App 路径的直接源码证明吻合。
- cmd/agentdeck/install.go 的 claim 用法、成功文本及 channel/takeover 数据与手册一致；main.go errorDetails 保持 resource/reason 与 install reason 的无路径输出；ownership.go 的 TTY/text/quiet、默认目录提示及 doctor/Hook 例外与文档一致。
- internal/doctor/managed_command.go 及现有测试中的 path_resolved、mismatch/recovery 对应文档边界；receipt.go 的字段校验与文档声明一致。App helper 路径/锁与测试隔离限制、cask 四条件放行和卸载/升级残余风险，复核已批准 architecture/requirements 和 Task 8 验收账本，不把它们改写为新增运行证明。
- 稳定契约 v34 的变更不指定产品发布版本；A5 继续是用户豁免，旧 cask 修复建议仍标为未验证；未引入内部 helper 恢复指令。
- 评审记录/状态更新后运行记录和文档结构检查。未修改产品、测试、配置或三份被评审文档。

完成门禁：VERIFIED

Task ContentState：`installed-state-ownership:contract-reconciliation:rereview-r2-20261116-jyK2jZY4`；8 项 SHA-256 `5e495c452668270891bb747b2d5e4f90751d4168c3dc68c7cb40f38c5e355796`。相对修复态只有 tasks.md 追加状态，其余 7 项不变；四项修复证据按其范围 roll-up，新增独立复评 PASS。Task 门禁 VERIFIED 5/5，missing/invalidated/unresolved 均为空。

Topic 内容边界：`installed-state-ownership:topic-final-20261116-5e495c45`。现存 Topic 缺少部分 contains 与全部 required 准则；依据 tasks.md 的九任务分解补齐全部直接子任务，按主题层面的三项要求建立汇总：已交付基础能力（任务 1–7）、最终跨面验收（任务 8，含明确 A5 豁免）、稳定契约闭环（任务 9）。不改写旧任务准则；Task 5 使用 acceptance-v3，Task 7 使用 acceptance-v2。

现有交付 gate 查询分别 VERIFIED：Task 1 为 7 项、Task 2 为 10 项、Task 3 为 5 项、Task 4 为 5 项、Task 5 acceptance-v3 为 8 项必需准则（返回 9 条有效观察）、Task 6 为 7 项、Task 7 acceptance-v2 为 6 项、Task 8 为 9 项。它们的原始 ContentState/观察保留：父级按既有范围复用，任务 5 当前安装协议及任务 8 的最终候选组合证据承接较早基础能力之后的实现变更，不能单独将早期证据宣称为所有后来代码的运行证明。HEAD 已含全部已交付代码，本轮只有文档/状态变化；当前契约依赖另绑定到已批准的 Task 9 内容清单。

Topic 固定门禁 VERIFIED 3/3，missing/invalidated/unresolved 均为空。本轮完成主题内容检查点，不是已提交、推送、集成、发布或归档。Task 9 保持 awaiting_commit；Topic 与来源缺陷交付协调仍开放。

Task checkpoint：ad-installed-state-ownership-contract-reconciliation；content_state=5e495c452668270891bb747b2d5e4f90751d4168c3dc68c7cb40f38c5e355796；gate=VERIFIED。
提交建议：用户授权后交付 README、cli-design、cli-manual、Task 9 评审记录及对应 tasks.md；不包含产品改动。
推送建议：origin/feature/installed-state-ownership，需单独授权，并验证实际提交范围、消息、贡献者、SSH 签名及远端状态。
Unit completion checkpoint：installed-state-ownership；Topic 内容门禁 VERIFIED 3/3；Task 9 本地交付与后续集成仍须分别授权。
