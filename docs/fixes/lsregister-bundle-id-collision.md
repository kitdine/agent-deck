---
status: active
created: 2026-09-07
---

# 缺陷：分发测试用生产 bundle identifier 造 fixture，污染 LaunchServices 并打停全部出货 widget

## 现象

2026-09-06 深夜起，本机 13 个 AgentDeck widget 实例全部无法刷新：通知中心里渲染为
黑块或停在旧内容，四个 kind 全中，24 小时内 6803 次失败、0 次成功。

WidgetKit 逐字给出的拒绝理由：

```
WidgetKit.WidgetArchiver.ValidationError.bundleStubNotSupported(
  underlyingError: SimpleError.message(
    "Bundle version did not match; LaunchServices DB may need to be rebuilt"))
-> CHSErrorDomain 1050 timelineReloadFailed，重试退避到 1 小时
```

排查排除了两类看起来更像的原因：App Group 快照持续在写且内容正确，widget 扩展本身
也能成功生成部分 timeline，因此不是数据缺失、也不是 provider 崩溃。真正的分歧在
宿主 bundle 的元数据：LaunchServices 中 bundle id 为 AgentDeck 的注册有 44 条，
CFBundleVersion 互相冲突，WidgetKit 校验归档记录的 host 版本时拿到的不是真实安装的
那一个，整份归档因此作废。

把 58 条非 `/Applications` 的 AgentDeck 注册逐条 `lsregister -u` 反注册后重启
`chronod`，AgentDeck 的注册条目从 44 条降到 1 条，13 个实例全部 reload succeeded，
失败归零。

本次修复期间又测得一个更直接的量：在注册基线干净（生产 id 恰好 1 条）的机器上跑一次
`scripts/test-macos-distribution.sh`，fixture 会新增 **7 条**注册。修复前这 7 条落在
生产 bundle id 上，即每跑一次分发测试就往出货 app 的注册记录里塞 7 条版本冲突条目。

## 根因

`scripts/test-macos-distribution.sh` 的 `make_bundle`（HEAD `1c5d464` 的 :158，
:170 与 :182 两处 `CFBundleIdentifier`）用生产标识写 fixture 的 `Info.plist`：

```
CFBundleIdentifier = com.kitdine.agentdeck            版本 1.2.3 / build 37
CFBundleIdentifier = com.kitdine.agentdeck.widget
```

这些 fixture 是真实 bundle，被真实的 `scripts/package-macos-app.sh` 打进 DMG，
section 6（HEAD :382）再用 `hdiutil attach` 挂载它。挂载即触发 LaunchServices 注册；
`-nobrowse` 挡的是 Finder 显示，不是注册。脚本的 `trap rm -rf` 删掉临时目录，却不做
任何反注册，于是每跑一次就留下一组悬空条目，版本号是脚本自己编的 1.2.3 / build 37。

被测脚本对标识本身没有任何依赖：`scripts/package-macos-app.sh` 全文不读
`CFBundleIdentifier`，它只用 `AGENTDECK_APP_GROUP`（默认
`N2FZ2FNRTU.group.com.kitdine.agentdeck`）签 widget 的 entitlements。也就是说
fixture 借用生产标识从来没有换来任何覆盖面，只换来了对系统状态的破坏。

## 修复边界

改动只在 `scripts/test-macos-distribution.sh`：

- 新增四个常量（:166 起）：`fixture_bundle_id=com.kitdine.agentdeck.disttest`、
  `fixture_widget_bundle_id=com.kitdine.agentdeck.disttest.widget`，以及
  `shipping_bundle_id` / `shipping_widget_bundle_id` 两个只作为禁止值存在的常量。
  同时把「挂载即注册、`-nobrowse` 不挡注册、`trap` 不反注册」这条机制写进注释，
  因为它是这段代码唯一的存在理由，而它从代码本身看不出来。
- `make_bundle` 的两处 `CFBundleIdentifier`（:185、:197）改用 fixture 常量。
- 新增 `assert_fixture_identifier`（:225），`make_bundle` 每写完一个 bundle 就用
  `plutil` 把标识读回来校验：等于生产标识则带机制说明退出 1，不等于 fixture 标识
  也退出 1。断言点在 `make_bundle` 内，早于任何打包与挂载，所以判定为真时不会有
  任何注册发生——这一点由下面的 RED 实测坐实。

**明确不改**：

- **不在 `trap` 里加 `lsregister -u`。** 原 issue 把两条写成「或」，用户裁定的是
  前者。隔离标识是无条件成立的，反注册则依赖脚本正常退出，是更弱的第二道防线，
  且要新决定清理时机与清理失败时的处理。fixture 现在仍会留下 `disttest` 的悬空
  条目，但它们与出货 app 不再争抢同一条记录，这是本次要解决的问题的全部。
- **不动 app group。** `N2FZ2FNRTU.group.com.kitdine.agentdeck` 是被测脚本的契约
  默认值，与 LaunchServices 的 bundle 注册无关；改它是改被测契约，不是隔离 fixture。
- **不碰根因二。** Xcode DerivedData 与历史 agent 临时目录里的 AgentDeck 副本同样
  在抢注册，那不在本脚本范围内，记录留在 `ad-bug-lsregister-bundle-id-collision`
  的描述里。
- **不做 doctor 的诊断与恢复路径。** 原 issue 建议范围的第 2 项已拆为
  `ad-bug-lsregister-collision-no-recovery-path`，lane 待用户裁定。
- **不修 section 4 的 Homebrew 弃用失败。** 见下，carrier 为
  `ad-bug-cask-preflight-deprecated`。

## 验证

L3（installer/build 行为，加 L2 的全量 Go 测试）。

**范围外的既有失败，先记在这里，因为它决定了下面的跑法。** 在仓库内直接运行
`bash scripts/test-macos-distribution.sh` 会在 section 4 退出 1：

```
Homebrew warned while loading the agentdeck-app cask:
Warning: Calling `preflight` is deprecated! Use `preflight_steps` instead.
  .../Casks/agentdeck-app.rb:27
```

根因是 `packaging/homebrew/agentdeck-app.rb.tmpl:27` 的 `preflight do` 被当前
Homebrew 标为弃用，而 section 4 刻意把加载期的任何 warning 判为失败。它先于本次改动
存在（模板最后一次改动是 `9a956b9`），也先于本次改动的位置——本次 diff 全部落在
:155 之后，section 4 结束于 :153。已登记为 `ad-bug-cask-preflight-deprecated`（P1，
它同时让 `make check-macos-distribution` 与 `release-verify` 对任何改动必红）。

因此 section 5–9 的验证在一份 scratchpad 副本上进行：内容与仓库文件逐字一致，只
删去 section 4 并把 `root` 固定回仓库路径。副本只用于跑，不进仓库。

**GREEN**：

```bash
bash <scratchpad>/verify/test-macos-distribution.sh
# exit=0
# macOS distribution packaging: PASS
# 35.8s
```

**注册面实测**，用 `lsregister -dump` 在三个时间点各取一次快照，按
`^identifier: +<id>$` 精确计数：

| identifier | 跑之前 | GREEN 之后 | 三次 RED 之后 |
| --- | --- | --- | --- |
| `com.kitdine.agentdeck` | 1 | 1 | 1 |
| `com.kitdine.agentdeck.widget` | 1 | 1 | 1 |
| `com.kitdine.agentdeck.disttest` | 0 | 7 | 7 |

生产标识的两条记录（`/Applications/AgentDeck.app` 及其 appex）在整个过程中一条未增，
fixture 制造的 7 条注册全部落在隔离标识上。这就是本次修复要保证的性质，而它是被测出来
的，不是被声称的。

**RED（三次变异，逐次在副本上改一个值，其余不动）**：

| 变异 | 结果 |
| --- | --- |
| `fixture_bundle_id` 改回 `com.kitdine.agentdeck` | exit 1，`a distribution fixture declares the shipping bundle identifier com.kitdine.agentdeck (.../AgentDeck.app/Contents/Info.plist)` |
| `fixture_widget_bundle_id` 改回 `com.kitdine.agentdeck.widget` | exit 1，同一条报文，路径指向 `AgentDeckWidget.appex/Contents/Info.plist` |
| 常量改成 `com.example.other` 而 heredoc 里写死 `...disttest`（模拟两者脱钩） | exit 1，`a distribution fixture declares com.kitdine.agentdeck.disttest (...), want com.example.other` |

第三条变异是必要的：只断言「不等于生产标识」会放过 heredoc 与常量各写各的这类改动，
那正是这次缺陷最初发生的形状。

三次 RED 之后的注册快照见上表——`disttest` 仍是 7、生产标识仍是 1，说明拦截确实发生在
任何打包与挂载之前，RED 本身没有制造新的污染。

**L2**：

```bash
scripts/run-go-test.sh ./...
# exit=0，go test passed
```

本次没有 Go 代码改动，跑它是为了满足 L3 = L2 加相关检查的规定，不是因为它与改动相关。

## 授权说明

用户在会话中裁定本条为 Lane A 并直接授权修复：「如果登记了 则以 lane a 的方式 进行
直接修复」。据此没有铸造 `Authorize Development` Gate——事后补铸一个再由 agent 自行
关闭，只会伪造一次人工批准。这与 `docs/fixes/tasks-md-status-sync-false-claim.md`
的处置一致，规则与实做的分歧本身已有归属：`ad-lane-a-gate-vs-direct-instruction`。

评审未被豁免。用户裁定的范围是原 issue 建议范围的第 1 项，第 2 项已另行登记且 lane
待裁定，本次没有把它悄悄合并进来，也没有把它丢掉。

## Review — Round 1 — 2026-09-07

### 📋 评审报告：fix / lsregister-bundle-id-collision

📊 综合评分：9/10

✅ 评审结论：PASS

**被评审内容状态**：HEAD `1c5d4645d67e765d8ed4e0c1f4d2d0139c275dbe`，工作产物为
未提交的 `scripts/test-macos-distribution.sh` 与本文件，按 HEAD 加二者的 scoped blob
指纹绑定；指纹的具体取值记在 CEv1 的 `content_state` 节点上。

**评审人 / 独立性**：Claude Code（Opus 5），与开发不在同一会话，上下文中不含开发
过程的推理与中间结论。两点如实记录：会话启动时注入了前一会话的观察摘要（其中包含
本缺陷的排查结论），且 Beads actor 同为 `claude-code`。为抵消这一点，本轮的关键结论
不采信记录的自述，全部由本轮自行测量重得，见证据表。

**方法**：读改动与被测脚本全文；从仓库文件按行抽取 `make_bundle` 与
`assert_fixture_identifier` 构成隔离 harness，跑一次 GREEN 与四次变异；用仓库文件的
逐字副本（仅删 section 4、固定 `root`）跑 sections 5–9；在该次运行前后各取一次
LaunchServices 注册面快照；L0 与 L2。

**范围**：`scripts/test-macos-distribution.sh` 的本次改动，及本记录自身。不含 doctor
恢复路径（`ad-bug-lsregister-collision-no-recovery-path`）、Homebrew `preflight` 弃用
（`ad-bug-cask-preflight-deprecated`）、DerivedData 与临时目录副本抢注册（留在
`ad-bug-lsregister-bundle-id-collision` 描述里的根因二）。

**限制**：section 4 在本机必红，因此本轮与开发一样无法端到端跑仓库文件本身，只能跑
删去 section 4 的逐字副本。该失败先于本次改动存在，承载于
`ad-bug-cask-preflight-deprecated`，并由下面的 LS-R1-F2 记录其对本次防线的影响。

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

**LS-R1-F1**（低）[`scripts/test-macos-distribution.sh`:156-169] fixture 的注册条目
从不清理，每跑一次脚本 `com.kitdine.agentdeck.disttest` 增加 7 条，无上界。载体：
`ad-bug-disttest-fixture-registrations-linger`（P3，已创建并以 `discovered-from`
关联到本缺陷，其中记录了实测数字与两条可能做法）。

- 行为风险：本机 LaunchServices 数据库单调增长，且指向已删除的 `/var/folders` 路径；
  日后排查同类问题时是噪声。不会重现 widget 全停的故障，因为这些条目与出货 app 不再
  争抢同一 bundle id。
- 证据：本轮实测。运行前 `disttest` 7 条，跑一次 sections 5–9 后 14 条，`+7`；同一
  两次快照里 `com.kitdine.agentdeck` 与 `com.kitdine.agentdeck.widget` 各恒为 1 条。
- 归属：属于用户裁定明确排除的范围（原 issue 建议范围第 1 项的两个备选中，用户取
  标识隔离而非 trap 反注册），指向的是本脚本先于本次改动就有的行为，不是本次改动引入
  的缺陷，因此不阻断本目标。
- 💡 有界改法：在 `trap` 里对本次 fixture 的挂载点执行 `lsregister -u`，或让
  fixture DMG 不经过 `hdiutil attach` 校验。两条都记在载体里，本次不做。

**LS-R1-F2**（中低）[`scripts/test-macos-distribution.sh`:225] 本次新增的防线在项目
自身的门禁中当前不可达。

- 行为风险：`make check-macos-distribution` 与 `release-verify` 会在 section 4 先行
  退出 1，永远走不到 section 5 的 `assert_fixture_identifier`。也就是说，这道断言现在
  只在有人手工绕开 section 4 时才会执行；在 Homebrew 弃用被处理之前，它不构成门禁级
  保护。
- 证据：section 4 的 `grep -Eiq 'deprecat|^warning'` 判定位于 :137–141，早于常量与
  断言所在的 :156 起；`packaging/homebrew/agentdeck-app.rb.tmpl:27` 仍是 `preflight do`，
  该文件最后一次改动是 `9a956b9`（2026-09-02），先于本次改动。
- 归属：指向的是 Homebrew 弃用这一先存缺陷，不是本次改动的缺陷；本次改动无法也不应
  顺手改模板。
- 💡 有界改法：不在本次范围内。载体 `ad-bug-cask-preflight-deprecated`（P1，已存在）
  修复后，本防线自动进入门禁，无需再改本次代码。

### 🟢 优点

- **断言点的位置是被证明的，不是被声称的。** `assert_fixture_identifier` 在
  `make_bundle` 内、写完 plist 之后、任何打包与挂载之前。本轮四次变异全部在
  `exit 1` 时未产生任何注册，注册面快照可证。
- **等值断言是全覆盖的那一条。** `want` 恒为 fixture 标识，因此任何偏离——包括把
  widget plist 写成 app 的出货标识这种交叉错配——都会被第二个分支拦下；
  `shipping_*` 两个常量只承担把最危险的那种偏离说清楚的职责。本轮 M4 变异验证了
  这一点：报文落在 `want` 分支，但仍然 `exit 1`。
- **断言的覆盖面完整。** 脚本内四处 fixture bundle（`AgentDeck.app`、`StubAgentDeck.app`、
  `missing_keychain`/`case`/`missing_widget` 各例）全部经由 `make_bundle` 生成，
  `AgentDeck-mismatched.app` 由已断言的 bundle `cp -R` 而来，没有绕过断言的路径。
- **注释写的是机制而不是意图。** 「挂载即注册、`-nobrowse` 不挡注册、`trap` 不反注册」
  这条链从代码本身看不出来，而它是这段代码存在的全部理由。
- **记录的边界是负向写死的。** 四条「明确不改」各自给了理由与去处，范围外的 Homebrew
  失败在开发轮就已登记为独立缺陷而不是被绕过或吞掉。
- **记录里的数字经得起复算。** 本轮独立重得的注册基线与 fixture 增量与记录逐项一致。

### 📝 总结

被评审内容是 `scripts/test-macos-distribution.sh` 的 39 行新增与本 fix 记录。Lane A 的
唯一评审问题——改动是否让实现满足既有契约，且契约一旦失守回归是否会失败——两侧都由本轮
自行测量回答：GREEN 侧，逐字副本跑通 sections 5–9 且生产标识注册数不变；RED 侧，四次
变异（app 标识回退、widget 标识回退、常量与 heredoc 脱钩、widget plist 交叉写成 app 出货
标识）全部 `exit 1` 且发生在任何注册之前。记录中被核对的定位（HEAD `1c5d464` 的 :158、
:170、:182，改动后的 :166、:185、:197、:225，挂载点 :382，section 4 边界）逐项属实，
`scripts/package-macos-app.sh` 确实全文不读 `CFBundleIdentifier`，因此借用生产标识从未
换来覆盖面。

残留的不确定性有两处，都已归属到载体而不是留在本记录里：fixture 注册仍会以每次 +7 的
速度累积（LS-R1-F1），以及在 Homebrew 弃用被处理之前，这道新防线不在门禁路径上
（LS-R1-F2）。两者都指向本次改动之外，不构成本目标的未闭合发现。

**发现处置**：LS-R1-F1 -> `ad-bug-disttest-fixture-registrations-linger`；
LS-R1-F2 -> `ad-bug-cask-preflight-deprecated`。本轮无未归属发现。

### 证据

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| GREEN（隔离 harness，直接取自仓库文件的 `make_bundle` 与 `assert_fixture_identifier`） | `bash harness-green.sh` | exit 0；app plist 读回 `com.kitdine.agentdeck.disttest`，widget plist 读回 `com.kitdine.agentdeck.disttest.widget` |
| RED M1（`fixture_bundle_id` 改回出货标识） | 同上，改一个常量 | exit 1，`a distribution fixture declares the shipping bundle identifier com.kitdine.agentdeck (…/AgentDeck.app/Contents/Info.plist)` |
| RED M2（`fixture_widget_bundle_id` 改回出货标识） | 同上 | exit 1，同一条报文，路径指向 `AgentDeckWidget.appex/Contents/Info.plist` |
| RED M3（常量改 `com.example.other`，heredoc 写死 `…disttest`） | 同上 | exit 1，`a distribution fixture declares com.kitdine.agentdeck.disttest (…), want com.example.other` |
| RED M4（本轮新增：widget heredoc 交叉写成 app 的出货标识） | 同上 | exit 1，`a distribution fixture declares com.kitdine.agentdeck (…appex/Contents/Info.plist), want com.kitdine.agentdeck.disttest.widget`——落在 `want` 分支，仍然拦下 |
| GREEN（仓库文件逐字副本，仅删 section 4、固定 `root`；副本与仓库文件的差异经 `diff` 逐行确认只有这两处） | `bash verify-nosection4.sh` | exit 0，`macOS distribution packaging: PASS`，58.8s |
| 注册面（该次 GREEN 之前） | `lsregister -dump \| grep -oE '^\s*identifier:\s+com\.kitdine\.agentdeck[a-z.]*' \| sort \| uniq -c` | `com.kitdine.agentdeck` 1、`…disttest` 7、`…widget` 1 |
| 注册面（该次 GREEN 之后） | 同上 | `com.kitdine.agentdeck` 1、`…disttest` 14、`…widget` 1 —— 生产标识零增长，fixture 的 7 条全部落在隔离标识 |
| 断言覆盖面 | `rg -n 'make_bundle\|StubAgentDeck\|mismatched' scripts/test-macos-distribution.sh` | 六处 fixture bundle 全部经 `make_bundle`；`AgentDeck-mismatched.app` 由已断言 bundle `cp -R` |
| 被测脚本不依赖标识 | `rg -n 'CFBundleIdentifier\|bundle_id' scripts/package-macos-app.sh` | 零命中 |
| L0 | `git diff --check`；`make check-whitespace` | 均 exit 0，无输出 |
| L2 | `scripts/run-go-test.sh ./...` | exit 0，`go test passed` |
| L3 相关检查 | 上表的分发脚本 GREEN 与注册面对比 | 见上；section 4 无法运行，原因与载体见「限制」 |

**验证层级**：L3（build/installer 行为 = L2 加相关 install 检查），与记录 `验证` 一节
声明的层级一致，与 `.agent-instructions/project-rules.md` 的 L0–L4 矩阵一致。

**完成门禁**：VERIFIED —— `unit_kind: task`、`work_unit_id: fix:lsregister-bundle-id-collision`。
Lane A 无上层 topic 边界，该任务边界即最外层。
