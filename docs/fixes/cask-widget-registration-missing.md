---
status: active
created: 2026-10-01
---

# 缺陷：正常 Cask 安装后 Widget 未注册且配置化 timeline 不可用

Beads: `ad-bug-cask-widget-registration-missing`。Lane A：恢复已有 App/Widget
安装契约；用户于 2026-10-01 明确授权完成修复及独立评审。未授权 Git 交付。
基线：`release/v0.6.x` / `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`。
工作区：`agent-deck.fix.cask-widget-registration-missing`。
串行工作区规则来源：`59aa33a33b3568bd3d6e7e9840103086cf0ccfd8` 的
`.agent-instructions/branching.md#serial-lane-a-fix-workspace-entry`；补丁线已有
AGENTS、Project Rules、Beads、Evidence 和 Fix records 规则继续适用。
本修复不把新版 main 合入补丁线，不修改治理或共享 Hook/runtime。

## 现象

v0.6.0/build 21 正常 Cask 安装后 PlugInKit 返回 `(no matches)`，chronod
报告 unknown extension。原始文件位于
`/private/tmp/agentdeck-v060-homebrew.e98ZrJ/pluginkit-normal-install.txt`，
SHA-256 为 `03b074af4f60e77a0e11af5957bbe98572155ac20c908c5bbded872aa33de52a`。
手工注册后的五种配置化 Widget 请求仍失败；这不是正常安装成功证据。

## 根因

旧 Cask 没有 nested extension 注册生命周期，Release CI 只验证目录存在。
当前已安装包签名通过，版本及 helper commit 与基线一致；手工注册能让扩展启动，
因此不能把未知扩展故障归因为版本错误或拒绝签名。

另一个已复现的失败链：`AppIntentsArchivingError 1000 / Failed to create
LinkAction` → `Unable to get LNAction from intent` → `No AppIntent in
timeline(for:with:)` → `CHSErrorDomain 1101 / Returned view collection was
either nil or empty`。历史恢复日志中前三项各出现 212 次；2026-10-01
08:00:32 EDT 的 Quota 请求复现同一链条。安装包只有扩展包含三个配置 Intent
元数据，序列化配置的描述却指向宿主 App。provider 源码始终返回一个 entry。
同一份配置 Intent 定义现已编译进宿主与扩展。候选的已安装路径原生请求
成功，五种已配置 Widget 的归档均被系统以非空 entry 接收，原 AppIntent 失败链
不再出现在这些成功请求中。注册与 Intent 发现是本修复的两个独立必要边界。

## 修复边界

- 正式 App 首次启动时在后台为自身 nested Widget 执行 PlugInKit 注册，成功后刷新
  timeline。Debug/XCTest 不注册，后台命令设 5 秒终止边界。当前 Homebrew install-step
  sandbox 禁止注册服务的 mach-lookup；真实 postflight 实验已失败并回滚，故不采用该方案。
- Cask 使用现有 uninstall script，在移除 App 前退出宿主并只反注册安装路径；路径
  缺失时无操作，不使用 sudo。
- 宿主 target 编译既有 `WidgetIntents.swift`；不改变配置 ID、参数或界面。
- 构建和 Cask CI 校验宿主与扩展的配置 Intent 元数据；安装 CI 校验唯一且精确的
  PlugInKit 路径，拒绝目录存在但没有注册、连接失败或竞争副本。
- 执行真实 Homebrew uninstall artifact 和 shell 脚本的隔离回归，仅替换 PlugInKit
  外部命令，不注册测试副本。
- 不新增 doctor API，不清空缓存、不使用 `--zap`，不删除用户数据库或凭据。

## 验证

- Failure-first：旧模板在真实 Homebrew loader/step runner 回归中以
  `missing Widget uninstall registration lifecycle` 失败；已发布安装包在 Intent 元数据检查中
  以宿主缺少 `ClientPeriodWidgetIntent` 失败。
- 新模板回归已通过：含空格的 appdir、重复卸载、缺失扩展、反注册失败传播。
- Debug App 构建通过，宿主与扩展均包含三个配置 Intent；没有运行 Go 全套，因为
  Go 产品代码、依赖及契约未变。
- `scripts/test-macos-distribution.sh` 通过；日志
  `/private/tmp/agentdeck-cwr-distribution.log`。
- 迁移回归通过。XCTest 首轮有一个未修改的刷新控件时序断言失败：1.6 秒成功
  反馈已回到 idle；日志 `/private/tmp/agentdeck-cwr-xctest.log`。需无构建负载的定向
  复现和最终验证，不修改或削弱该断言。
- Release 构建通过。真实本地 Cask 安装通过；未公证候选被 Gatekeeper 拒绝启动，
  因而不能证明已公证发布后的正常首次启动。本机缺少 `agentdeck-release`
  公证 profile。后续运行时验收先确认签名及二进制与本会话自建产物一致，再仅移除
  该开发候选的下载隔离标记；没有更改系统信任设置。这一环境差异不是公证或
  发布验收 PASS，最终发布的普通 Cask/Gatekeeper 验收仍归 v0.6.5 Release 边界。
- 本地实际注册及保留的五种配置化 timeline 验收通过。
  `/private/tmp/agentdeck-cwr-native-final.ndjson`（SHA-256
  `559990491c03e18f3de5269506d3030a929718adc93e042af869b59a5cd616a9`）记录
  Composition 14、Magnitude 15、Quota 3、Rhythm 14、Trust 11 次配置请求成功，
  系统明确接收 `1 entries`。四种 usage Widget 覆盖 small/medium/large；Quota
  覆盖当前已配置 medium，不声称未配置的 family 被实测。窗口内有两个没有 kind
  的 generic `timelineReloadFailed`，不能据此宣称日志完全无错误；原 LinkAction/
  No AppIntent 错误链不存在，具名配置请求均有成功接收。
- 真实 Cask 卸载执行 quit 和路径限定反注册后 PlugInKit 为 `(no matches)`；
  重装并启动后仅一个安装路径记录。证据为
  `/private/tmp/agentdeck-cwr-registration-uninstalled.txt`、
  `/private/tmp/agentdeck-cwr-registration-reinstalled.txt` 和
  `/private/tmp/agentdeck-cwr-lifecycle.log`。开发候选信任差异同上。
- 无并行重构建负载的定向测试通过；最终原命令 `scripts/test-macos-app.sh`
  退出 0、`TEST SUCCEEDED`，全部 suites 通过，包含 Widget 45 个测试。
  最终日志 `/private/tmp/agentdeck-cwr-xctest-final.log`，SHA-256
  `290683c015ceee383b2c754aa907b75ed5ffa295927768a586d492005706324d`。
  保留首轮失败，不据此声称已消除该既有时序测试的所有潜在 flake。
- 本地候选用既有 Developer ID 签名、跳过公证，只用于验证，不作为发布验收。

WorkUnit: `fix:cask-widget-registration-missing`。独立评审及完成门禁见以下轮次。


## Review — Round 1 — 2026-10-01

## 📋 独立修复评审

📊 总体评分：7/10

✅ 结论：FAIL

Reviewer: 独立冷上下文 Codex，`widget_independent_review`。
Method: 单一 operations/compatibility/tests lens，只读 diff、源码、Homebrew 实现及原始日志。
Reviewed state: HEAD `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`，冻结 candidate
fingerprint `142e958a8c9e1364d4b67ec478f88b1c4b93239327220eb0f3114806de977e92`，
由评审者独立复算。Scope: 本记录修复边界内的 App、Cask、构建、CI 和回归。
Completion gate: NOT_VERIFIED。

### 🔴 严重问题 — 必须修复

- **CWR-R1-F1 / P1 / OPEN** — 评审时实际注册和五种配置化 timeline 验收未完成。
  当时提供的日志仅有 metadata-change 通知，不能证明原 LNAction/空 collection
  故障消失。关闭要求：准确候选的 installed-path 注册和五种配置的系统接收结果，
  并区分本地开发信任环境与已公证 Cask 验收。

### 🟡 改进建议 — 推荐

- **CWR-R1-F2 / P2 / OPEN** — `AgentDeckAppTests/MenuBarChromeTests.swift:133`
  的首轮日志仍为 `idle != succeeded` / `TEST FAILED`。不可自动豁免为无关 flaky。
  关闭要求：保留断言，无重构建负载的 focused reproduction 及最终适用测试证据；
  失败持续则先分类再修复。本轮未证明其为变更导致的代码缺陷。

### 🟢 优点

- 已有配置 Intent 共享，未改变 IDs、参数或 UI。
- 注册仅定位自身 nested extension；Debug 不注册，当前用户后台执行。
- Homebrew quit 在 script 前；upgrade/reinstall 保留 script；卸载不删除用户数据。
- 元数据与唯一 installed-path 检查通过；卸载回归执行真实 artifact 和 shell 脚本。

### 📝 总结

未发现已证明的 scoped 代码缺陷，但两个验收缺口未关闭，故 FAIL。
评审者否定了“宿主仍缺 Intent”、“带空格路径处理错误”、“反注册晚于 App 删除”
及“变更清空用户数据”的假设。5 秒发送 terminate 是软边界；未证明其被忽略。
本轮不把未验证的推测转成代码修复要求。独立报告来自该 reviewer 的初轮输出；
后续新增证据仅针对以上缺口进行一次选择性复评。

## Review — Round 2 — 2026-10-01

## 📋 选择性独立复评

📊 总体评分：9/10

✅ 结论：PASS

Reviewer: 同一独立冷上下文 Codex，`widget_independent_review`；主代理核对原始证据后记录。
Method: 唯一一次选择性跟进，仅审新增卸载 CI 断言和新增验收证据，复用首轮未变代码评审。
Scope: 本次 L3 修复；不扩大为公证、发行、Tag 或发布门禁。
Reviewed state: HEAD `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`；独立复算最终 fingerprint
`7c51f4e9f1646e639e4428e2e7b12df2b14c84a7a27d71e432883997d60eec3b`。
CEv1 scoped digest: `992fe8fd6f307aaf230b2262adac2fdb8d63c4880de1537744ab44032033b149`。
ContentState: `fix:cask-widget-registration-missing:candidate:992fe8fd6f307aaf230b2262adac2fdb8d63c4880de1537744ab44032033b149`。
该 ContentState 使用 `/private/tmp/agentdeck-cwr-content-manifest.txt` 的 UTF-8/LF
`head=<HEAD>` 加 lexical `path=<Git blob>` manifest；fix record 使用本轮追加前的冻结前缀，
不让随后追加的评审/门禁元数据形成循环指纹。
Completion gate: VERIFIED，四项 task criterion 全部满足，无 missing、invalidated 或 unresolved。
此结果只覆盖 scoped task，不代表 v0.6.5 Release VERIFIED。

### 🔴 严重问题 — 必须修复

无未关闭项。

- **CWR-R1-F1 → CLOSED（本地 L3 修复验收）**：独立解析 native 原始日志确认五种
  配置共 57 个非空接收结果；独立检查真实 Homebrew quit/script/removal、卸载
  `no matches` 和重装唯一 installed-path。原 LinkAction/No AppIntent 链为零。
  本地 Developer ID 候选的隔离标记处理已明确，不转换为公证发行验收 PASS。
- 已安装 host 与签名候选独立 `cmp` 完全相同，SHA-256
  `33d5f74faaf2bf7725d60b0778e191ff9dc0700e7170b499cf4fbd3cbaf6a3b7`；
  Widget 同样完全相同，SHA-256
  `9e6cfe8fd16fcfd4b7279b5255ee1a39c5fb045b49744038686712d6e809bd9b`。
  签名候选为 `apps/macos/build/widget-acceptance/release-candidate/AgentDeck.app`。
  unsigned Xcode 产物的 Mach-O 字节差异来自签名，不是运行了另一个 source candidate。

### 🟡 改进建议 — 推荐

无未关闭项。

- **CWR-R1-F2 → CLOSED**：原断言未修改，focused 和最终原命令都通过。
  独立核对 Shared 83、App 138、Widget 45 个测试零失败，日志 digest 匹配。
  App 中既有 opt-in `SchemaSignalAcceptanceTests.testSchemaSignalNativeMatrix`
  跳过，属于未修改的 schema-signal 验收面；不将该 skip 当作原生矩阵验收证据。

### 🟢 优点

- 修复到达正常正式 App 启动路径；没有借 Homebrew sandbox 禁止的服务调用发布必失败钩子。
- 同一配置定义为宿主和扩展生成元数据，保留既有配置；五种实际配置均被系统接收。
- 当前 Homebrew 的 uninstall 顺序、真实卸载/重装、失败先行回归和路径限定均有证据。
- 新 CI 同时检查安装后的唯一 registered path 和卸载后的 `no matches`。

### 📝 总结

初轮两个证据缺口关闭，新增卸载断言未引入 scoped correctness/safety 缺陷。
保留两个 kindless generic1050、Quota 未配置 family、opt-in 原生矩阵和既有时序测试
潜在 flake 的边界；不宣称全系统日志无错误或所有 family 都被实测。
普通公证发行包的 Cask/Gatekeeper 验收仍由 `v0-6-5-contract / v0-6-5-contract`
的 Release 准备边界承接，未豁免。本机当前是测试中安装的本地签名候选，非正式发布。

Task checkpoint: `fix:cask-widget-registration-missing`，上述 ContentState，L3 独立 PASS / CEv1 VERIFIED。
提交建议：仅本修复十个 scoped 文件；须有单独 Git 提交授权。
推送建议：授权后独立 `fix/cask-widget-registration-missing` PR 到 `release/v0.6.x`；
提交、推送、PR、合并和发布尚未执行。

冻结 fix record 前缀长度：8622 bytes；当前文件的此前缀必须与独立评审冻结内容一致。

## Review — Round 3 — 2026-10-01

## 📋 GitHub Codex Review

📊 总体评分：8/10

✅ 结论：FAIL

Reviewer: GitHub `chatgpt-codex-connector`，独立 PR code review。
Method: PR #23 current-head review；主代理运行决定性复现后采纳。
Reviewed state: `5b64468d2feb88d7d8339876dd747800dda4e996`。
Scope: 已交付到 PR 的同一 Widget 修复及相关 desktop-refresh 契约。
Completion gate: FAILED；此前该状态的 VERIFIED 被新失败证据纠正。

### 🔴 严重问题 — 必须修复

- **CWR-R3-F1 / P2 / OPEN** — `AgentDeckApp.swift:224` 在注册成功后直接调用
  `WidgetCenter.shared.reloadAllTimelines()`，绕过已有 semantic kind-scoped
  `WidgetTimelineReloader`。决定性复现：
  `bash scripts/check-desktop-refresh-integration.sh` 在该提交退出 1，报告
  `legacy unconditional Widget reload remains`。CI 的 desktop job 未运行这条
  既有检查，因而四个绿色检查没有覆盖该违约。
  GitHub finding: https://github.com/kitdine/agent-deck/pull/23#discussion_r4156254506
  修复要求：通过已有 adapter 请求全部 `AppGroupWidgetKind.allCases`，并让
  desktop CI 执行既有 integration checker。

### 🟡 改进建议 — 推荐

无其他建议。

### 🟢 优点

GitHub review 已绑定准确提交；push/PR 两轮 verify/desktop 共四项 CI 都通过，
但不能以此否定独立检查的明确失败。保留 Round 2 的历史结果，不重写其结论。

### 📝 总结

CWR-R3-F1 修复候选已把调用接入 `WidgetTimelineReloader.live`，使用 canonical
`AppGroupWidgetKind.allCases`；新增 CI step 运行原 checker，未修改 checker 的断言。
修复后 checker PASS，Release 编译验证正在执行。待新 head 独立 GitHub 复评后关闭
本项；当前不合并 PR。旧 native/Intent/卸载证据按未受影响边界保留，未宣称新 head
已完成复评或正常公证发行验收。


## Review — Round 4 — 2026-10-01

## 📋 GitHub finding 修复的独立确认

📊 总体评分：9/10

✅ 结论：PASS

Reviewer: 独立冷上下文 Codex，`startup_reload_repair_review`。
Method: 仅比较 HEAD `5b64468d2feb88d7d8339876dd747800dda4e996` 与 App/CI/record
三个 dirty 文件；直接项目评审，不启动完整审计或重复 broad suites。
Reviewed state: 独立复算 dirty-diff SHA-256
`c41e047ca53f62486cf14f26685e38f0bf84be14ad381dbbba399721a02f161b`。
Scope: CWR-R3-F1 的 kind-scoped adapter 和 CI wiring 修复；不扩大为发布验收。
Completion gate: VERIFIED，四项 scoped criterion；
ContentState: `fix:cask-widget-registration-missing:candidate:72ae03a811fb9a4e781a2d03e086ad6122a64284ddc047191967526b21aeba07`。
新 Release/integration/独立 review 证据与未变 native/installer 边界分开绑定；旧提交
的 regression/review 证据已失效，旧失败历史保留。
该状态使用 `/private/tmp/agentdeck-cwr-r4-manifest.txt`；fix record 第一 14569 bytes
为冻结前缀，随后追加的本轮评审/门禁元数据不形成循环指纹。

### 🔴 严重问题 — 必须修复

无未关闭代码项。

- **CWR-R3-F1 → CLOSED（独立确认的修复候选）**：注册成功后调用现有 Sendable
  `WidgetTimelineReloader.live`，传入全部五种 canonical kind；adapter 的
  `reloadTimelines(ofKind:)` identifier 与五个现有 `AppIntentConfiguration`
  精确一致。新 desktop CI step 执行现有 Make target 和未削弱的 checker。
  后续仍等待 GitHub 对新 head 的实际 review，不以本地确认代替该步骤。

### 🟡 改进建议 — 推荐

无新 CWR-R4-Fn findings。

### 🟢 优点

- 未修改注册成功/失败处理、当前用户边界、配置参数或卸载/用户数据契约。
- 复用既有 adapter 和 canonical kind owner，没有新增 reload 实现。
- 独立运行 `make check-desktop-refresh-integration` PASS、`git diff --check` 退出 0，
  并检查新 Release log 的 `BUILD SUCCEEDED`。

### 📝 总结

精确候选通过小范围独立确认。此前 Shared/App/Widget tests、Intent 发现、原生
entry/render 和安装/卸载证据仅对未变边界进行 scope-aware 复用，不宣称已经原生
执行这条新 reload 调用。正常公证 Cask/Gatekeeper 仍在 v0.6.5 Release 边界。
主代理负责绑定新证据并取得新 head 的 GitHub 复评、CI 和合并结果；旧提交的
Round 3 FAIL 和失败门禁保留，不覆盖历史。

## Review — Round 5 — 2026-10-01

## 📋 GitHub 复评及 CI readiness

📊 总体评分：8/10

✅ 结论：FAIL

Reviewer: GitHub Codex 对 `188f07417abad48fda94392132d7aea3a5267980` 完成 code review，
回复未发现主要问题；主代理按真实 CI 结果评估整体交付 readiness。
Method: 精确 head 的独立 GitHub code review，加 PR-run desktop 失败日志判别。
Scope: CWR-R3-F1 修复后的同一 PR；新增 CI checker 的运行依赖。
Reviewed state: `188f07417abad48fda94392132d7aea3a5267980`。
Completion gate: FAILED（remote desktop CI），不把本地验证或 code review PASS 当成 CI PASS。

### 🔴 严重问题 — 必须修复

- **CWR-R5-F1 / P2 / OPEN** — 当前 macOS runner 没有 `rg`。PR workflow
  `36879634057` / desktop job `110427799778` 在新 integration step 报告
  `scripts/check-desktop-refresh-integration.sh: line 20: rg: command not found`，
  assertion 退出 1；push 对应 job 同样失败。Swift 编译及 unit tests 已通过，
  不是 Widget repair 的编译故障。最小修复：在该 CI step 明确安装缺失的 ripgrep，
  不改变 checker、断言或产品依赖。

### 🟡 改进建议 — 推荐

无其他项。首次下载 job 日志因终端 escape 输出保护失败，使用 gh 的指定参数保存
原始日志并清理 ANSI 后定位；没有把日志读取限制当成产品缺陷。

### 🟢 优点

GitHub 复评已明确绑定 `188f07417a` 并无新 code finding；CWR-R3-F1 已关闭。
此前源码修复和 Release 检查保持有效，CI tooling prerequisite 单独处理。

### 📝 总结

CI step 已添加 `command -v rg` 缺失分支及 `brew install ripgrep`，仅此步骤设置
`HOMEBREW_NO_AUTO_UPDATE=1`，不升级已有工具、不删除断言。等待新 head 的 GitHub
复评和 actual CI 成功后关闭 CWR-R5-F1；当前仍不合并 PR。
