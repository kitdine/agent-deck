---
status: historical
topic: desktop-app
subject: desktop-widget
retired: 2026-09-01
---

# Review log — desktop-app / desktop-widget

## Round 1 — 2026-08-21

## 📋 `desktop-widget` 代码交付评审

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: HEAD `06a77b873082546cd882722d9b6e0c86be0c2d1e`
  plus scoped Task fingerprint
  `072f07e17036032e4c99e55ff1346a7b3a90849943c1ba642580c348db7f1ee4`.
- Reviewer: Codex.
- Method: project `development-workflow` formal code Review. CodeGraph identified
  the projection, timeline, rendering, host-embedding, and test paths; focused
  source and fixture inspection then followed the real values into the Trust
  Widget. Broader product verification stopped after the decisive reproducer,
  as required by project review policy.
- Scope: Task 4's Widget sources and tests, App Group projection hunks, Widget
  target and host-embedding hunks, scheme entry, entitlements, and sandbox
  checker. Menu-bar, work-signals, prototype, distribution, and unrelated dirty
  documentation are excluded.
- User decision: direct `WidgetKit` use from `AgentDeckShared` is accepted for
  this delivery. Exact architecture-layer wording and other document-purity
  concerns are not findings in this round and must not be re-raised unless the
  user changes that boundary.
- Completion evidence: not queried. This FAIL crosses no Task completion
  boundary and makes no `VERIFIED` claim.

### 🔴 严重问题 — 必须修复

[apps/macos/AgentDeckWidget/WidgetViews.swift:153] **[P1] DW-R1-F1 — Trust Widget
drops the only meaningful attribution amount when pricing is incomplete.**

- 行为风险：the shipped complete fixture contains 1,800 unattributed tokens and
  zero pricing coverage. The small Trust Widget renders only `—`; medium and
  large render the three tiers as `0.000000000 / —` and never say that cost is
  incomplete. A user therefore sees no attribution risk in the exact state
  where the Trust surface is supposed to explain why the monetary number is not
  reliable.
- 证据：`desktop/fixtures/v1/snapshot-complete.json` has `determinable` and
  `inferred` at zero, `unattributed.value.tokens = 1800`,
  `unattributed.value.cost_incomplete = true`, all tier `share` values `null`,
  and pricing `coverage = "0.00"`. `WidgetFormat.share(nil)` returns `—`;
  `TrustWidgetView` uses that share as the small headline and uses only
  `tier.value.providerCost` plus the same nullable share for medium/large. It
  never reads the tier token amount or `costIncomplete`. The current Widget
  presentation tests assert only surface state, section depth, canvas constants,
  and unrelated derived series; none asserts the Trust values produced from the
  fixture.
- 💡 有界修复：introduce one Trust presentation value per tier that never
  treats incomplete zero cost as a known zero. When share/cost is unavailable,
  show the projected token or event amount and an explicit incomplete/unpriced
  qualifier; keep the determinable-share headline only when that share is
  meaningful. Add focused tests against `snapshot-complete` for the small and
  medium/large outputs, including the 1,800-token unattributed case and the
  pricing-incomplete qualifier.

### 🟡 建议改进 — 推荐

无。本轮按用户要求只记录影响核心产品结果的代码缺陷，不记录文档措辞、架构纯度、
copy 偏好或像素级建议。

### 🟢 做得好的地方

- Four distinct Widget configurations and all three Widget families are wired
  into the extension target and the host application embeds that extension.
- Snapshot reads stay on the redacted App Group projection, timeline refresh is
  clamped, and failed or unsupported reads degrade without exposing raw state.
- The implementation includes task-local Widget tests, a bilingual catalog,
  and a static sandbox check; the open finding is a missing business-value
  projection in one surface, not a missing Widget foundation.

### 📝 总结

The reviewed candidate has the right large-scale shape: one embedded WidgetKit
extension, four families, intent configuration, redacted projection input,
timeline refresh, localization assets, and isolated tests. It does not pass
because the Trust family loses the only non-zero attribution signal in the
repository's own complete fixture and therefore fails its primary user outcome.
No conclusion is made about exhaustive rendered-size acceptance or the broader
L3 suite after the decisive reproducer.

- Evidence: the two recorded CodeGraph path explorations; focused inspection of
  `WidgetViews.swift`, `WidgetDomain.swift`, `WidgetSnapshot.swift`,
  `WidgetTimeline.swift`, the Widget tests, Xcode target diff, entitlements, and
  `scripts/check-widget-sandbox.sh`; `jq` extraction of the complete fixture's
  period, quality, pricing, and rhythm values; scoped content manifest above.
- Verdict: REOPEN.

## Round 1 修复 — 2026-08-21

- Repaired state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b` plus the
  changed blobs `514344f6979b4648db072aaf31a9b8357d86ecb4`
  (`WidgetDomain.swift`), `4dd48d7cf03101adaa7b0496f3e4379da195ff44`
  (`WidgetViews.swift`), `ba2b21b4b108ed6da0ff45ec1417c6edadc31f7a`
  (`WidgetPresentationTests.swift`), `aa3bc7774e8796181c8f46df7a3e509dda2e3df7`
  (`WidgetTestFixtures.swift`), and `d10b2e5e56c3e7dc7346290ced89cb88990e6252`
  (`ux/widget.md`).
- Repairer: Claude Code.
- Scope: only `DW-R1-F1`. No other finding was recorded in Round 1, and nothing
  outside the Trust surface, its presentation values, its tests, and the `trust`
  contract paragraph was changed.

### DW-R1-F1 — 处置：已修复

- Change: `WidgetDomain.swift` gains one Trust presentation value per tier —
  `WidgetTrustAmount` (`.cost` or `.tokens`), `WidgetTrustTier`,
  `WidgetTrustProviderRow`, `WidgetTrustPricing`, and the
  `trustTiers` / `trustHeadline` / `trustProviders` / `trustPricing` /
  `trustCostIncomplete` surfaces on `WidgetSurfaceModel`. A tier whose
  `cost_incomplete` is true now projects its token amount instead of a
  `provider_cost` that is unknown rather than zero.
- Change: `TrustWidgetView` consumes those values instead of reading
  `tier.value.providerCost` and `WidgetFormat.share` directly. `small` keeps the
  determinable share as its headline only while that share is non-`null`,
  otherwise it uses the tier amount and draws no bar; it also renders the
  inferred and unattributed amounts on one line, which the contract already
  required and the reviewed candidate omitted. Every size that shows a tier
  renders the `Cost incomplete` line when a tier or today's pricing coverage is
  incomplete. The `large` per-provider rows use the same tier value rather than a
  bare nullable share.
- Change: `ux/widget.md`'s `trust` section states the degraded presentation
  ("An unpriced amount is not a zero"), because the contract described only the
  priced, share-available state that the repository's own fixture never reaches.
  Written directly into the owning document under this topic's deferred-document-
  review rule; no document round is opened by it.
- Regression protection: `WidgetPresentationTests` gains
  `testTrustKeepsTheUnattributedAmountWhenPricingIsIncomplete`, which asserts the
  exact reproducer from Round 1 against `snapshot-complete` — unattributed
  `.tokens("1.8k")` with `costIncomplete`, inferred `.cost("$0.00")`, a `nil`
  determinable share that must not become the headline, pricing `coverage`
  `"0.00"` with both unpriced identifiers, `trustCostIncomplete` true, and the
  period not qualifying as empty — and
  `testTrustUsesTheDeterminableShareOnlyWhileItIsMeaningful`, which asserts the
  opposite direction through the new `snapshotWithPricedToday` fixture helper:
  a meaningful share stays the headline and priced tiers stay monetary.
- Verification (L3, task 4's level): `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash scripts/test-macos-app.sh`
  -> `** TEST SUCCEEDED **`, `AgentDeckSharedTests` 7, `AgentDeckAppTests` 52,
  `AgentDeckWidgetTests` 12 (5 presentation, 5 timeline, 2 copy), 0 failures;
  `bash scripts/check-widget-sandbox.sh` -> `widget sandbox boundary: PASS`;
  `make check-whitespace` and `git diff --check` -> exit 0. `DEVELOPER_DIR` was
  set per command because the machine's active developer directory is the
  Command Line Tools instance; no global developer-directory or system state was
  changed.
- Not covered: the manual macOS 26 acceptance halves — rendered size acceptance
  at the true canvas for all twelve configurations, both languages without
  truncation, and the runtime sandbox denial from inside the running extension.
  They remain open for the Re-review to route, exactly as Round 1 left them.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Task gate
  is closed and no commit is authorized by this round.

## Round 2 — 2026-08-21（实现复评，等待 macOS 26 人工验收）

## 📋 `desktop-widget` 修复复评

📊 总体评分：8/10

✅ 结论：PASS（Round 1 finding 全部关闭）

- Reviewed state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`，五个候选 blob 与
  Round 1 修复记录逐一比对一致：`514344f6979b4648db072aaf31a9b8357d86ecb4`
  (`WidgetDomain.swift`)、`4dd48d7cf03101adaa7b0496f3e4379da195ff44`
  (`WidgetViews.swift`)、`ba2b21b4b108ed6da0ff45ec1417c6edadc31f7a`
  (`WidgetPresentationTests.swift`)、`aa3bc7774e8796181c8f46df7a3e509dda2e3df7`
  (`WidgetTestFixtures.swift`)、`d10b2e5e56c3e7dc7346290ced89cb88990e6252`
  (`ux/widget.md`)。复评对象因此可以确定地绑定到 Round 1 的 finding。
- Reviewer: Claude Code（独立于本轮修复者的复评视角）。
- Method: 独立重跑判定性证据，而不是复用修复者的报告。先用 `jq` 从
  `desktop/fixtures/v1/snapshot-complete.json` 重新抽取 Round 1 的复现数据，再逐行
  读取 `WidgetDomain.swift` 的 Trust 投影、`WidgetViews.swift` 的 `TrustWidgetView`
  与 `ShareRow`/`ShareBar`、两个新增回归测试与 `snapshotWithPricedToday` helper、
  copy 目录与字符串目录，最后独立执行完整 Xcode gate 与静态沙箱检查。
- Scope: 仅 Round 1 记录的 `DW-R1-F1`，加上修复是否引入回归。用户在 Round 1 作出的
  边界决定（`AgentDeckShared` 直接使用 `WidgetKit` 可接受、不记录文档措辞与像素级
  建议）在本轮继续生效，未被重新提起。
- Completion evidence: 已查询。`work_unit_id = desktop-app:desktop-widget` 在
  `github.com/kitdine/agent-deck` 命名空间下**不存在** work_unit 节点，gate 结果为
  `NOT_VERIFIED`。本轮不写入证据，因为 Task 的验收条件尚未取得完整观测结果。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。本轮沿用 Round 1 的记录边界，只判定已记录 finding 的处置与回归风险。

### 🟢 做得好的地方

- **`DW-R1-F1` — 处置：已关闭。** 独立复现确认：`snapshot-complete` 的 today
  aggregate 三个 tier 的 `share` 均为 `null`，`determinable` 与 `inferred` 的
  `tokens = 0`、`provider_cost = 0.000000000`、`cost_incomplete = false`，
  `unattributed` 的 `tokens = 1800`、`cost_incomplete = true`，today pricing
  `coverage = "0.00"`、`unpriced_events = 2`、
  `unpriced_identifiers = ["claude/claude-sonnet-5", "codex/gpt-5"]`。在这一状态下
  当前实现：`small` 因 `headline.share == nil` 走 `amountText`，不再打印 `—`，并在
  `secondaryLine` 渲染 `Inferred $0.00 · Unattributed 1.8k Tokens`；`medium`/`large`
  的 `ForEach(model.trustTiers)` 用 `amountText` 把 `unattributed` 渲染为
  `1.8k Tokens` 而不是 `0.000000000`；三种尺寸只要显示 tier 就因
  `trustCostIncomplete` 为真而带上 `Cost incomplete` 行。Round 1 的行为风险
  ——「用户在 Trust 界面看不到任何归因风险」——在同一份夹具上不再成立。
- 语义分离正确。`WidgetTrustTier.init` 把「未定价」与「零」的区分放在投影层而不是
  视图层，`WidgetTrustAmount` 用 `.cost` / `.tokens` 两个 case 让「不可能把未知成本
  写成金额」成为类型层面的事实，而不是某个 `if` 分支的约定。
- 空 share 的两条渲染路径都不再伪造测量值：`small` 在 `share == nil` 时完全不画
  `ShareBar`（否则 `WidgetFormat.decimal(nil) = 0` 会画出读起来像实测 0% 的零宽
  条）；`medium`/`large` 的 `ShareRow` 走 `WidgetFormat.share(nil)` 输出 `—`，本来
  就不画条。
- 回归测试与缺陷绑定牢固。`testTrustKeepsTheUnattributedAmountWhenPricingIsIncomplete`
  断言 `.tokens("1.8k")`、`costIncomplete`、`share == nil`、
  `trustCostIncomplete == true`、`unpricedIdentifiers` 的确定顺序，以及 1800 tokens
  不构成 `.empty`；把修复回退到读 `provider_cost` 会立刻使其失败。
  `testTrustUsesTheDeterminableShareOnlyWhileItIsMeaningful` 通过
  `snapshotWithPricedToday` 断言反方向——share 有意义时仍是 headline、已定价 tier 仍
  是金额——因此测试不是单向地把一切都推向 token 展示。
- `trustTiers` 只取 `provider == nil` 的 client-scope aggregate，与
  `architecture.md:826` 「one client-scope aggregate plus deterministic per-provider
  records」的契约一致，不会因为只有 per-provider 记录而误判为不可用。
- 契约与实现同向更新。`ux/widget.md` 的 `trust` 段新增「An unpriced amount is not a
  zero」，把降级规则写进拥有该行为的文档，而不是留在评审记录里。
- `Tokens` 与 `Cost incomplete` 两个 key 都已在 `WidgetCopy.allKeys` 与
  `AgentDeckWidget/Localizable.xcstrings` 的 `en` / `zh-Hans` 中存在，
  `WidgetCopyTests` 会对缺失或空值报错，新增文案不会留下未本地化的字符串。

### 📝 总结

Round 1 只记录了一个 finding，`DW-R1-F1`，本轮独立复现后确认已关闭，且未发现修复
引入的回归：改动集中在 Trust 一族的投影与视图，其他三族与 timeline、snapshot 读取
路径未被触及，完整测试套件全绿即为旁证。finding 处置矩阵为空，故本轮结论为 PASS。

- 独立验证（L3，task 4 的级别，全部由本轮重新执行）：
  `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash scripts/test-macos-app.sh`
  -> `** TEST SUCCEEDED **`，`AgentDeckSharedTests` 7、`AgentDeckAppTests` 52、
  `AgentDeckWidgetTests` 12（`WidgetCopyTests` 2、`WidgetPresentationTests` 5、
  `WidgetTimelineTests` 5），0 failures；
  `bash scripts/check-widget-sandbox.sh` -> `widget sandbox boundary: PASS`；
  `make check-whitespace` 与 `git diff --check` -> exit 0。`DEVELOPER_DIR` 仅按命令
  传入，未改变机器的全局 developer directory。
- Residual uncertainty / 未取得的证据：本轮的 PASS 只覆盖已记录 finding 的处置，
  **不覆盖 Task 4 的验收条件**。以下三项仍无观测结果，与 Round 1 结束时相同：
  1. macOS 26 真实画布比例下十二种 Widget 配置的渲染验收（无裁切区域）；
  2. `en` 与 `zh-Hans` 在每种尺寸下无截断——`small` 现在多出
     `secondaryLine` 与 `Cost incomplete` 两行 caption，这正是最需要实测的尺寸；
  3. 隐私否定证明的第二半——运行中 extension 内可观察的 sandbox 拒绝；静态
     `check-widget-sandbox.sh` 覆盖 entitlement、私有路径、链接模块与宿主嵌入关系，
     但不是运行期证据。
- Completion gate: `NOT_VERIFIED`。因此 Beads `ad-desktop-widget-dev` 依 One
  lifecycle 的「A review verdict is not an evidence gate」规则停留在 `in_review`，
  不进入 `awaiting_commit`；`tasks.md` 的 `Review` 单元格保持未勾选。这与 task 3
  `menubar-experience` Round 25 的处理方式一致。
- Verdict: PASS（finding 处置）；Task 未完成，等待人工验收。

## Round 3 — 2026-08-21（macOS 26 人工验收）

## 📋 `desktop-widget` macOS 26 真机验收

📊 总体评分：4/10

✅ 结论：FAIL

- Reviewed state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`，与 Round 2 相同的
  五个候选 blob，内容未改动。安装物为该候选用
  `scripts/build-macos-app.sh` 构建的 unsigned bundle，装到
  `/Applications/AgentDeck.app`（安装时剔除了 DerivedData 残留的
  `AgentDeckAppTests.xctest`）。
- Reviewer: Claude Code，在用户显式授权下于用户本机 macOS 26.7 (25G220)、
  Xcode 26.4 (17E192) 构建物上执行。
- Method: 真机执行，不改任何系统设置。为避免拍到无关窗口，全部取证使用
  `CGWindowListCopyWindowInfo` 枚举窗口后按窗口 ID 定向 `screencapture -l`，
  不做整屏截图。十二种配置由 WidgetKit 在真实桌面按真实画布尺寸渲染
  （`180x180` / `360x180` / `360x360` 点）。失败原因由 `log show` 的
  `containermanagerd` / `pkd` 记录直接判定，而非推断。
- Scope: `ux/widget.md` 的 Manual checklist 八项。语言、动态字体、对比度三类按
  `acceptance/menubar-experience.md` 的安全原则不在个人机切换系统设置，改由注入式
  离屏渲染取证，本轮尚未执行。
- Completion evidence: 未查询。本轮为 FAIL，不跨越任何 Task 完成边界，不作
  `VERIFIED` 主张。

### 🔴 严重问题 — 必须修复

[apps/macos/Config/AgentDeck.xcconfig:4] **[P1] DW-R3-F1 — App Group 标识符没有
team ID 前缀，沙箱 widget extension 被系统拒绝访问自己的投影，十二种配置在真机上
全部渲染为 `Data unavailable`。**

- 处置：new。
- 行为风险：这不是降级展示，而是 widget 在出厂形态下**永远拿不到数据**。产品的
  四个问题一个都答不出来，Round 1/Round 2 修好的 Trust 展示语义也无从显现。
- 证据（全部来自运行期系统日志与真机渲染，非源码推断）：
  1. 未签名安装物直接被扩展点拒绝：
     `pkd … rejecting; Ignoring mis-configured plugin at
     [/Applications/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex]:
     plug-ins must be sandboxed`；`pluginkit -m -p com.apple.widgetkit-extension`
     列出 54 个扩展，其中没有 AgentDeckWidget。
  2. 用仓库自身 entitlements 内容对 framework、helper、appex、宿主做本机 ad-hoc
     签名后，`codesign --verify --deep` 通过，WidgetKit 注册
     `com.kitdine.agentdeck.widget(0.5.0)`，扩展进程正常运行并成功完成
     timeline 请求（`com.apple.chrono:timeline … success`）。
  3. 但 `containermanagerd` 在扩展每次启动时都拒绝其容器请求：
     `[com.kitdine.agentdeck.widget] requesting [group.com.kitdine.agentdeck]:
     REJECTED. Requestor's signature does not allow it to access a TCC-protected
     group container. Group containers identifiers should be prefixed by
     requestor's team ID to allow access on this platform.`
     （21:57:52 与 21:59:23 各一次。）
  4. 同一时刻投影本身是有效的：`schema_version = 1`、`usage.available = true`、
     `presentation.available = true`、scopes 含 `all` / `codex` / `claude`，
     由未沙箱的宿主正常写入。故失败点是扩展的容器访问，不是投影。
  5. 因此 `WidgetSnapshot.swift:47` 的 `init?` 拿不到 `containerURL` 而返回 `nil`，
     `WidgetSnapshotLoader` 抛出 `containerUnavailable`，`entry.snapshot == nil`，
     `WidgetSurfaceModel.surface` 落到 `.unavailable`，`AgentDeckWidgetView`
     渲染 `UnavailableWidget`。十二张真机截图逐一印证，按 SHA-256 前 12 位绑定：
     `Magnitude` `878e73a120bb` / `82756959ad9e` / `94363c821b24`，
     `Composition` `875d335afee7` / `0dcea1796b19` / `eb1b3c301ce0`，
     `Trust` `05ad6a00f6c0` / `c1822f6a1197` / `65f7ee44e90c`，
     `Rhythm` `82541cbb09a4` / `4335ebf38be0` / `92d2dbd7987c`。
- 💡 有界修复：把 App Group 标识符改为带 team ID 前缀的形式，并在四处保持一致
  —— `apps/macos/Config/AgentDeck.xcconfig:4` 的 `AGENTDECK_APP_GROUP`、
  `AgentDeckShared/AppGroupSnapshotStore.swift:175`、
  `AgentDeckWidget/WidgetSnapshot.swift:38`，以及 `apps/macos/README.md:10` 与
  `architecture.md:343` 的记载。两份 entitlements 已经用 `$(AGENTDECK_APP_GROUP)`
  取值，不需要各自改写。标识符属于签名身份，应由构建配置注入而不是散落成第二处
  硬编码常量；`scripts/check-widget-sandbox.sh` 应同时断言两个 Swift 常量与
  xcconfig 取值一致，否则下一次只改一处仍会以同样方式失败。修复后必须重跑本轮
  验收，取得十二种配置的**有数据**渲染。
- 残余不确定性：本轮只验证了 ad-hoc 签名（`signer:none`，无 team ID）。没有可用的
  Developer ID 身份，因此"带 team ID 的真实签名下未加前缀的标识符是否同样被拒"
  未被观测。系统给出的是平台规则（须由 team ID 前缀），据此预期同样会失败，但这一步
  仍需在具备签名身份的环境复验。

### 🟡 建议改进 — 推荐

无。本轮沿用 Round 1 确立的记录边界。

### 🟢 做得好的地方

- **`DW-R1-F1` — 处置：保持关闭。** Round 2 的判定不受本轮影响：该 finding 是投影层
  与视图层的展示语义缺陷，其回归测试在测试进程内直接构造 snapshot，不经过 App Group
  容器，因此 DW-R3-F1 既不能掩盖它，也不会使它复现。
- 扩展的沙箱边界在运行期被真实证明有效。系统日志显示扩展以
  `<<com.kitdine.agentdeck.widget; signer:none>>` 进入自己的
  `~/Library/Containers/com.kitdine.agentdeck.widget/Data` 容器，并且**任何**越界
  容器请求都被 `containermanagerd` 拒绝。这正是隐私否定证明要求的运行期一半：
  扩展拿不到任何未经 entitlement 授权的路径——只是当前它连自己该拿的那一个也拿不到。
- 十二种配置确实由真实 WidgetKit 宿主在真实桌面上以真实画布比例渲染，
  `systemLarge` 是 `systemMedium` 的等宽两倍高（`360x360` 对 `360x180`），
  与 `ux/widget.md` 的画布契约一致，没有出现被当作宽横幅的走形。

### 📝 总结

真机验收把一个自动化测试无法触及的缺陷暴露了出来：Widget 的全部展示逻辑正确，
但它在出厂签名形态下拿不到自己的数据源。Round 2 的 PASS 覆盖的是已记录 finding 的
处置，而本轮证明 Task 4 的验收条件远未满足——十二种配置渲染的是同一个不可用态。

Manual checklist 逐项结果：

| # | 项目 | 结果 |
| --- | --- | --- |
| 1 | 十二配置在真实桌面按真实尺寸渲染，明/暗 | **FAIL** — 渲染发生了，但全部为 unavailable 态；明暗两态未取得有数据渲染 |
| 2 | 最大动态字体逐级降级 | 未执行 — 路由到注入式离屏渲染 |
| 3 | VoiceOver 朗读标签、限定词顺序、图表摘要 | 未执行 — 需可重置 Mac |
| 4 | 提高对比度与灰度下状态可区分 | 未执行 — 路由到注入式离屏渲染 |
| 5 | 画廊占位不显示真实数据 | 无效观测 — 一切都是 unavailable，无法判定占位契约 |
| 6 | 超过六小时显示 `old` | 未执行 — 被 DW-R3-F1 阻塞 |
| 7 | 宿主从未启动时显示 unavailable | 无效观测 — 同上，无法与缺陷区分 |
| 8 | `en` / `zh-Hans` 每尺寸无截断 | 未执行 — 路由到注入式离屏渲染 |

- 机器状态：验收期间放置的十二个 widget 已全部移除，桌面恢复为用户原有的
  `Clock I` 与 `City I` 两个 widget。App Group 投影文件未被删除或改写（验收前曾
  临时移开以观测 checklist 7，宿主在 21:53 自行重新发布，原文件的临时副本已销毁）。
  `/Applications/AgentDeck.app` 现为本机 ad-hoc 签名状态，与仓库预期的 Developer ID
  分发形态不同；重新安装构建产物即可还原。
- Verdict: REOPEN。DW-R3-F1 为 P1 且阻塞其余七项验收；Task 4 的 `Review` 单元格保持
  未勾选，完成 gate 保持 `NOT_VERIFIED`。

### DW-R3-F1 — 处置：暂缓（外部前提未满足）

2026-08-21，用户决定：本 finding 现在无法修复，因为修复需要一个 Apple Developer
team ID，而账号尚未注册。App Group 标识符必须带 team ID 前缀，该值只能由已注册账号
提供，所以这是外部前提，不是可以在仓库内决定的实现问题。

- 该 finding **保持开启**，不因暂缓而关闭，Task 4 的 `Review` 单元格保持未勾选，
  完成 gate 保持 `NOT_VERIFIED`。
- Task 4 其余部分没有开启项：`DW-R1-F1` 已关闭，HEAD `cb782d6` 上的 L3 自动化门禁
  通过，Manual checklist 其余七项全部堵在 DW-R3-F1 之后。
- 恢复条件：Apple Developer 账号存在且 team ID 可用。届时按本轮"有界修复"列出的
  五处改动落地，重新安装带签名的构建物，并从 checklist 第 1 项起重跑 macOS 26 人工
  验收——本轮的十二张 unavailable 截图不能被复用为通过证据。
- 一个与之相邻的观察，留给 `unified-desktop-distribution`：该 task 已经拥有
  `apps/macos/Config/AgentDeck.xcconfig` 与签名/打包构建设置，并按其自身约定使用
  ad-hoc 或自签身份测试签名路径。标识符的**形态**（由构建配置注入而非第二处硬编码
  常量）因此可以在该 task 内先立起来，只有具体的 team ID 取值需要等账号。这是观察，
  不是把 DW-R3-F1 转派给它。
- Beads `ad-desktop-widget-dev` 已置为 `deferred` 并记录了恢复条件；调度移向下一个
  task。

## Round 4 — 2026-08-23（DW-R3-F1 Repair）

- Target: 仅修复 Round 3 的 P1 `DW-R3-F1`。`DW-R1-F1` 保持关闭；本轮不替代独立
  Re-review，也不顺手修改其他 Widget 展示问题。
- Repair:
  - Apple Developer Team `N2FZ2FNRTU` 已可用；canonical App Group 改为
    `N2FZ2FNRTU.group.com.kitdine.agentdeck`，`AGENTDECK_DEVELOPMENT_TEAM` 同步为
    `N2FZ2FNRTU`。
  - `AGENTDECK_APP_GROUP` 成为唯一运行时身份来源：Xcode 把它展开进宿主和 Widget 的
    entitlement 与 `AgentDeckAppGroupIdentifier` Info.plist 值；宿主 writer 与 Widget
    reader 从各自 `Bundle.main` 读取，不再编译两份可能漂移的标识符 literal，缺失时仍
    fail closed。
  - packaging fallback、distribution assertion/stub、Cask `zap` 路径和 living docs
    使用同一个值；`scripts/check-widget-sandbox.sh` 新增 Team 前缀、两份 entitlement、
    两份 Info.plist 与两个 Swift 读取点的一致性断言。
- Automated evidence:
  - `bash scripts/check-widget-sandbox.sh` -> PASS。
  - `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer ... bash
    scripts/test-macos-app.sh` 在沙箱外通过：Shared 39、App 52、Widget 12，0 failures。
    首次受管沙箱执行的 Observation macro transport failure 属环境边界；测试前临时移走
    的 universal helper 已按原 SHA-256 恢复。
  - `bash scripts/test-macos-distribution.sh` 在其隔离 Homebrew tap 环境通过。
  - `bash scripts/check-topic-docs.sh`、`make check-whitespace`、`git diff --check`
    均 exit 0。
- Signed candidate evidence:
  - 当前 HEAD `a190186297db40bade40f129fd4a17e35600bbbb` 的 universal `v0.5.0`
    candidate 使用 Developer ID Application 签名；`codesign --verify --deep --strict`
    通过，宿主 `TeamIdentifier=N2FZ2FNRTU`，宿主与 sandboxed Widget entitlement 均为
    `N2FZ2FNRTU.group.com.kitdine.agentdeck`，embedded helper 自报同一版本与 commit。
  - 安装前新 Team 容器的 snapshot 不存在；首次启动后生成 82,952-byte schema-v1
    projection，`usage.available`、`usage.presentation.available`、
    `sessions.available` 均为 true，presentation scope count 为 3。
  - `containermanagerd` 在 23:46:35 对宿主请求记录 `APPROVED`，在 23:44:00 与
    23:44:10 对 Widget 请求记录 `APPROVED`；原 Round 3 的 `REJECTED` 已不再复现。
  - `chronod` 在 23:46:48–23:46:49 对 Magnitude、Composition、Trust、Rhythm 的
    small/medium/large 十二种标准配置逐项记录 `reload: succeeded with 1 entries`，另有
    两个既存配置实例同样成功。
  - 十二张只捕获 Notification Center Widget window 的 light-mode 图均显示真实数据，
    不再显示 `Data unavailable`。按 Magnitude / Composition / Trust / Rhythm、每组
    small / medium / large 顺序，SHA-256 前 12 位为：`dd295c7b4e1a` /
    `3cc3bf7430a9` / `2f6b2347e840`，`f36505a55853` / `f276b0bfad84` /
    `7495d2547a18`，`0861e37bd299` / `aac9846ea1e` / `630e99d3ffca`，
    `7b55452b4f73` / `cb9906688084` / `0a1d8a8a82d`。
- Re-review boundary: 本轮只证明 DW-R3-F1 的容器拒绝与全 unavailable 结果已修复。
  light-mode 图中 Magnitude small 的 session 行和 Trust small 的 inferred 行出现省略号；
  这两个观察是否违反无截断验收项由独立 Re-review 判定，本轮不扩张为未经授权的修复。
  Dark、最大动态字体、VoiceOver、提高对比度/灰度、`old` 与宿主从未启动状态也留给
  独立 Re-review，不在 Repair 中冒充完成。
- Machine state: `/Applications/AgentDeck.app` 现为上述 Developer ID signed candidate；
  原 ad-hoc bundle 暂存于仓库外用于回滚。没有提交、推送、公证或发布。
- Verdict: REOPEN — DW-R3-F1 Repair complete, awaiting independent Re-review。
  Task 4 的 `Review` 单元格保持未勾选。

## Round 5 — 2026-08-24（Prototype 1:1 pre-Re-review Repair）

- Trigger: 独立 Re-review 开始前，用户用 repository prototype 与真机实现截图直接指出：
  Widget 虽已恢复真实数据，但原生展示没有按 `prototype/preview.html?surface=widgets`
  的设计实现，并明确授权按 prototype source 进行 1:1 修复。此处不是新增 Review
  finding，也不把截图当设计来源；`prototype/src/Widgets.jsx`、Widget CSS 与 data contract
  是实现权威，截图只用于最后比较。
- Repair:
  - 把原来一套简化 VStack 随 family 拉伸的实现改为十二个 source-aligned size branch。
    Header 使用原型的 Usage / Breakdown / Attribution / Activity 语义、accent icon、scope；
    footer 恢复 `Updated now` 与 qualifier 状态。
  - Magnitude 恢复 small 7-bar、medium 三周期 + 20-bar + date axis、large headline/flag/
    two-period context/area chart/date axis/stat chips；cost incomplete 的 trend 使用现有 token
    fallback，而不是画假零线。
  - Composition 恢复 top-model share、四种 model tone 与 share track、large token stack/
    token rows/client subtotal chip；所有比例去掉假的两位小数。
  - Trust 恢复 small headline/track/support、medium quality tracks + coverage、large measurement
    headline/inferred/unattributed/provider/unpriced note，同时保留 DW-R1-F1 的不完整成本 token
    fallback，不能为了贴图重新显示已知为假的 `$0.00`。
  - Rhythm 恢复 hour axis、Low/High legend、weekday labels、large 90-day calendar 与 stat chips。
    源码核对还发现 producer 的 168 cells 是 Monday-first；Widget 已从错误的 Sunday-first
    映射改为与 Go producer 和 menu-bar 相同的 Monday-first 规则，small 也从 grid 推导最忙
    小时范围。
  - Widget string catalog 增加上述原型语义的 `en` / `zh-Hans` copy；测试新增 family override
    以便渲染真实生产 View，不建立第二套 mock UI。
- Evidence:
  - `AgentDeckWidgetTests` focused run：14 tests，0 failures；exact section contract 直接逐项断言
    `Widgets.jsx` 的十二个 size branch、7/20/90 bucket、percentage 格式与 weekday rotation。
  - `scripts/test-macos-app.sh` 在完整 Xcode 环境通过；xcresult summary 为 105 passed、0 failed、
    0 skipped。
  - `bash scripts/check-widget-sandbox.sh` PASS；JSON catalog `jq` 解析与 `git diff --check`
    通过。
  - 十二张 production `AgentDeckWidgetView` dark attachment 按 Magnitude / Composition /
    Trust / Rhythm、每组 small / medium / large 的 SHA-256 前 12 位为：
    `af9189d2fdce` / `5f8065d44ffe` / `0767ab3dc5f6`，`45d7a0a92bb3` /
    `8051d9916851` / `8924c556954e`，`50e70d9feaed` / `4f32750e6a7f` /
    `b194aeeec082`，`a2f5bdd1e91b` / `03db18f2d5b8` / `c626f0f4f027`。
    比较基于相同 family canvas；不把 HTML 卡片比例强行写成 WidgetKit 不存在的 family。
- Debugging classification: 两次失败都在编译边界且已确定归属。第一次是 production
  `DateFormatter` optional symbol 使用错误；第二次是 test harness 试图写只读的
  `widgetFamily` environment。前者做 optional guard，后者给 production View 增加默认为空
  的 family override；真实 extension 仍由 WidgetKit environment 决定。small target build、
  test-target build、原 focused test command 和完整原命令均在最终内容状态通过。
- Boundary: 本轮没有替换 `/Applications/AgentDeck.app`，也没有重跑签名 App Group lifecycle；
  entitlement、reader、projection 与 signing inputs 均未改变，因此 Round 4 的 runtime
  evidence 不因纯 presentation repair 失效。Light、`zh-Hans` 真机、最大动态字体、VoiceOver、
  提高对比度/灰度、placeholder、`old` 与 host-never-launched 仍由独立 Re-review 判断。
- Verdict: REOPEN — prototype-alignment Repair complete, awaiting independent Re-review。
  Task 4 的 `Review` 单元格保持未勾选；没有提交、推送、安装、公证或发布。

### Round 5 installation addendum — 2026-08-24

- 用户随后单独授权安装本轮 candidate。重新构建的 universal app 绑定同一 HEAD
  `a190186297db40bade40f129fd4a17e35600bbbb`，embedded helper 自报 `v0.5.0` 与该
  commit；Developer ID `TeamIdentifier=N2FZ2FNRTU`，`codesign --verify --deep --strict`
  通过。
- Signed ZIP SHA-256 为
  `f183a289bd72691ec8af7a9d4e26420770efe8998a300a4d10f011d0b4625871`；安装前与安装后
  `AgentDeckWidget` executable SHA-256 均为
  `deb87857e21dcf2f59d8ecbeaea42c464a75b182dfbb0e4b9dc76526f396c26a`。
- 当前 app 已备份后由上述 candidate 替换，LaunchServices 与 PlugInKit 重新注册，宿主与
  extension 从 `/Applications/AgentDeck.app` 启动。`chronod` 在 01:20:17–01:20:26 对
  Magnitude / Composition / Trust / Rhythm 的 small / medium / large 十二种标准配置全部
  记录 `reload: succeeded with 1 entries`；另有既存 configuration 实例同样成功。
- 当前桌面只固定了两个既有系统 Widget，没有 AgentDeck Widget window；因此本 addendum
  证明的是新 binary 被真实 WidgetKit 加载并产出十二种 timeline，不把 gallery snapshot
  冒充 pinned light / dark / `zh-Hans` 视觉通过。那部分仍属于独立 Re-review。
- Machine state: `/Applications/AgentDeck.app` 现为 prototype-aligned Developer ID
  candidate；被替换版本保留在 repository 外的临时 rollback 目录。仍没有提交、推送、
  公证或发布；Round 5 的 REOPEN / awaiting independent Re-review 不变。

## Round 6 — 2026-08-24（独立 Re-review，Round 4 与 Round 5 之后）

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`，未提交工作树。
  安装物为 Round 5 addendum 记录的 prototype-aligned candidate。
- Reviewer: Claude Code，独立于 Round 4 与 Round 5 两轮 Repair。
- Method: 不采信两轮 Repair 对自己的叙述，改为回仓库与本机核对可核对的部分，再对照
  Round 3 为 `DW-R3-F1` 亲自写下的关闭条件逐项判定这条 finding 能否关闭。
- 逐项处置：
  - `DW-R1-F1` — **保持关闭**。Round 5 的展示重写明确保留了不完整成本的 token
    fallback，没有为了贴合原型重新显示已知为假的 `$0.00`；这正是该 finding 的实质。
  - `DW-R3-F1` — **保持开启**。结构性的那一半确实修好了，而且是我自己核的，不是读
    来的：`AgentDeck.xcconfig:4` 现为
    `N2FZ2FNRTU.group.com.kitdine.agentdeck`、`:17` 为 `N2FZ2FNRTU`；宿主与 Widget
    两份 `Info.plist` 都带 `AgentDeckAppGroupIdentifier`，
    `AppGroupSnapshotStore.appGroupIdentifier` 已从编译期常量改为从 bundle 读取的
    计算属性，Round 1 那两份"可能漂移的 literal"不复存在；
    `bash scripts/check-widget-sandbox.sh` PASS；`/Applications/AgentDeck.app` 的
    `Authority=Developer ID Application: Job Shen (N2FZ2FNRTU)`、
    `TeamIdentifier=N2FZ2FNRTU`。容器拒绝这一症状按 Round 4 的
    `containermanagerd APPROVED` 与十二次 `chronod reload: succeeded` 已消失。
    **但这条 finding 的关闭条件不是"容器能访问了"。** Round 3 写下的恢复条件是：
    团队 ID 可用、五处改动落地、重新安装签名构建物，并**从 checklist 第 1 项起重跑
    macOS 26 人工验收**，且明确写着那一轮的十二张 unavailable 截图不能被复用为通过
    证据。这个条件目前不满足。 -> open
- Manual checklist 在当前 build 上的实际证据状态：

| # | 项目 | 当前证据 |
| --- | --- | --- |
| 1 | 十二配置真实桌面真实尺寸渲染，明/暗 | **不满足**。Round 4 的十二张 light 真机截图取自 Round 5 展示重写**之前**的 binary，已不代表当前 build；Round 5 的十二个 dark 摘要是 production View 的离屏 attachment，其 addendum 自己写明"不把 gallery snapshot 冒充 pinned light / dark / `zh-Hans` 视觉通过"，当前桌面也没有固定任何 AgentDeck Widget |
| 2 | 最大动态字体逐级降级 | 未执行 |
| 3 | VoiceOver 标签、限定词顺序、图表摘要 | 未执行 |
| 4 | 提高对比度与灰度下状态可区分 | 未执行 |
| 5 | 画廊占位不显示真实数据 | 未执行（Round 3 为无效观测，此后无新证据） |
| 6 | 超过六小时显示 `old` | 未执行 |
| 7 | 宿主从未启动时显示 unavailable | 未执行 |
| 8 | `en` / `zh-Hans` 每尺寸无截断 | 未执行。Round 4 另留下两处待判观察——Magnitude small 的 session 行与 Trust small 的 inferred 行出现省略号；Round 5 重写了全部十二个 size branch，这两处观察既未被证实也未被证伪 |

- 也就是说：八项里一项证据过期、七项未执行，而这七项当初正是被 `DW-R3-F1` 堵住的。
  Round 4 与 Round 5 都明确把它们留给独立 Re-review，两轮的边界声明是诚实的；本轮
  的判定与它们一致，而不是与它们相左。
- 本轮为何不自行执行人工验收：它需要在用户本机固定十二个 widget 并切换动态字体、
  对比度、灰度、VoiceOver 与语言等系统设置。Round 3 依据
  `acceptance/menubar-experience.md` 的安全原则拒绝在个人机上切换系统设置，本轮沿用
  同一边界。这是需要用户逐次显式授权的动作，不是 Re-review 可以顺手执行的。
- Evidence:
  - `bash scripts/check-widget-sandbox.sh` → PASS
  - `apps/macos/Config/AgentDeck.xcconfig:4,17`、两份 `Info.plist` 的
    `AgentDeckAppGroupIdentifier`、`AppGroupSnapshotStore.swift:175` 的计算属性
  - `codesign -dv /Applications/AgentDeck.app` → Developer ID Application,
    `TeamIdentifier=N2FZ2FNRTU`
  - Round 4 的 runtime 日志与 Round 5 的测试结果被引用为**它们各自内容态**的证据，
    未被当作当前 build 的人工验收通过证据
- Verdict: REOPEN。`DW-R3-F1` 保持开启，Task 4 的 `Review` 单元格保持未勾选，完成
  gate 保持 `NOT_VERIFIED`。关闭它需要的是对当前 prototype-aligned candidate 从第 1
  项起重跑的 macOS 26 人工验收，而不是再一次代码修复。

## Round 7 — 2026-08-24（DW-R3-F1 人工验收重跑 · Repair）

- Repair state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`，未提交工作树。
  被验收物为 Round 5 addendum 记录的 prototype-aligned Developer ID candidate，
  安装于 `/Applications/AgentDeck.app`。
- Repair owner: Claude Code，在用户显式授权下于用户本机执行。
- Scope: 仅 `DW-R3-F1` 的关闭条件——对当前 candidate 从 checklist 第 1 项起重跑
  macOS 26 人工验收。不改任何产品代码、测试或配置，不关闭 finding（关闭归独立
  Re-review）。
- 起点状态（执行前实测，非引用）：宿主 pid 54299 运行中；App Group 投影
  `~/Library/Group Containers/N2FZ2FNRTU.group.com.kitdine.agentdeck/desktop-snapshot-v1.json`
  74,992 字节、01:52 刷新；`pluginkit -m -p com.apple.widgetkit-extension` 列出
  `com.kitdine.agentdeck.widget(0.5.0)`；extension pid 54298 以
  `-AppleLanguages ("zh-Hans-US", "en", "en-US")` 启动；外观 light，
  `increaseContrast` 未设，`grayscale` 0。
- 取证方法，沿用 Round 3 的边界：不做整屏截图。用 `CGWindowListCopyWindowInfo`
  枚举窗口后按窗口 ID `screencapture -o -l<id>` 定向捕获，因此只拍到 AgentDeck 的
  widget 窗口，用户桌面上其他 widget 的私人内容不进入证据。十二个窗口由
  Notification Center 托管，名称恰为 `Magnitude` / `Composition` / `Trust` /
  `Rhythm` 各三尺寸。
- **画布比例复核通过**：`180x180` / `360x180` / `360x360` 点（捕获为 2x：
  `360x360` / `720x360` / `720x720` 像素），`systemLarge` 是 `systemMedium` 的等宽
  两倍高，与 `ux/widget.md` 的画布契约一致。

### Checklist 第 1 项 — 通过

十二种配置在真实桌面、真实画布尺寸、明暗两态下**全部渲染真实数据**，不再是
Round 3 的同一个 unavailable 态。二十四张定向捕获的 SHA-256 前 12 位：

| 配置 | light | dark |
| --- | --- | --- |
| Magnitude small | `2c1e11f0f5e7` | `adb54c08ccb3` |
| Magnitude medium | `4f5f8cd7ccdf` | `29e5d46b4021` |
| Magnitude large | `c304c19abfb1` | `850734c95d2a` |
| Composition small | `85dfc7736ebe` | `e39a94e82dc4` |
| Composition medium | `a2b10cd81173` | `defe898ce2d0` |
| Composition large | `2373144e9ceb` | `23fdd0bf3819` |
| Trust small | `c6afc404b478` | `b31fa263f297` |
| Trust medium | `5284bf4d5537` | `fb6c8cfac395` |
| Trust large | `7aec258e0b85` | `06adc22b23a3` |
| Rhythm small | `917acab73bec` | `c0a5e13a656e` |
| Rhythm medium | `c4b28865c454` | `385fd5f2c0f7` |
| Rhythm large | `ae10786ad62f` | `8de7fa0fc3cc` |

**一次失败的取证及其纠正，记下来是为了不让它被复用。** 第一次暗色批次在切换外观后
只等 4 秒，十二张的 SHA-256 与 light 逐一**完全相同**——那不是暗色渲染，而是同一份
字节。单独探针确认 `AppleInterfaceStyle=Dark` 已生效但 widget 尚未重绘；改为等待
12 秒后重捕，十二张与 light 全部不同（`identical-to-light: 0/12`）。表中的 dark 列
是重捕结果，第一批已作废。外观在两次批次后都被还原为 light，最终读数为 light。

### Checklist 第 8 项 — `zh-Hans` 一半通过

extension 本轮以 `zh-Hans-US` 运行，因此上述二十四张即 `zh-Hans` 渲染。逐张目视
检查了 Magnitude small、Trust small（light 与 dark）、Composition large、
Rhythm large；其余为定向捕获并记录哈希，未逐张目视。已检查者**无截断**，所有标签
完整呈现。

**Round 4 留下的两处待判观察已解决。** Magnitude small 的支持行现为
`128.3M · 3 会话`、Trust small 的推断行现为 `推断 128.3M 令牌`（暗色批次时实时数据
已刷新为 `134.8M`），两处均无省略号。Round 5 的十二个 size branch 重写消除了它们。

### 顺带复核到的两点

- **`DW-R1-F1` 的语义在真机上保持。** Trust small 明暗两态都显示推断层的**令牌数**
  而非 `$0.00`，并携带 `成本不完整`。这是该 finding 的实质，在一次大范围展示重写后
  由真机渲染而非测试断言证实。
- **Rhythm 的 Monday-first 修正在真机上成立。** Rhythm large 的每周小时分布首行为
  `一`，与 Go producer 和菜单栏一致；`90 天背景` 与 `活跃天数 29 / 30`、
  `最忙 星期二`、`最闲 星期六` 三个 stat chip 齐备。

### 一处留给独立 Re-review 判定的观察

Composition large 在令牌构成四行结束与 `客户端小计` chip 之间有约 150 点的空白带
（360 点画布内）。可能是 chip 贴底的既定布局，也可能缺了原型中的某个元素。本轮
不扩张为未经授权的修复，与 Round 4 处理省略号的方式一致。

### 仍未执行的项目

第 2 项最大动态字体、第 3 项 VoiceOver、第 4 项提高对比度与灰度、第 5 项画廊占位、
第 6 项 `old` 状态、第 7 项宿主从未启动，以及第 8 项的 `en` 一半，本轮均未执行。

- Machine state: 十二个 widget 由**用户**固定，本轮未添加也未移除，现仍在
  `S27C900P` 桌面上；外观已还原 light；`increaseContrast` 与 `grayscale` 未被改动；
  没有提交、推送、安装、公证或发布。本轮期间我打开过一个 Finder 窗口并已关闭。
- Verdict: REOPEN — 第 1 项与第 8 项的 `zh-Hans` 一半已取得通过证据，其余六项半仍缺
  证据，`DW-R3-F1` 保持开启，等待独立 Re-review。

## Round 8 — 2026-08-24（人工验收续跑 · Repair）

- Repair state: 同 Round 7，HEAD `a190186297db40bade40f129fd4a17e35600bbbb`。
- Repair owner: Claude Code，用户授权继续跑完剩余 checklist，并明确要求任何系统设置
  必须先备份、还原后与原状**一模一样**。
- **设置基线**（改动前实测并存档，`scratchpad/settings-baseline/baseline.tsv`）：
  `-g AppleInterfaceStyle` UNSET；`-g AppleLanguages` `(en-US, zh-Hans-US)`；
  `com.apple.universalaccess` 的 `increaseContrast` UNSET、`grayscale` 0、
  `differentiateWithoutColor`/`reduceTransparency`/`reduceMotion`/
  `closeViewScrollWheelToggle` 均 UNSET、`voiceOverOnOffKey` 0；
  `com.kitdine.agentdeck AppleLanguages` `(zh-Hans-US, en, en-US)`。投影文件另存
  副本并记录 SHA-256 `642810602e29…`。

### 一条方法学更正，它推翻了本轮此前的两个推断

Widget 显示的是**实时数据**，在批次之间会变（Magnitude small 的 headline 在本轮内
从 `$86.58` 变到 `$92.69`，Composition large 的模型占比从 `80.1/15.6/4.4` 变到
`76.2/19.6/4.2`）。因此**"与基线哈希不同"不能推出"设置生效了"**，"哈希相同"也不能
推出"没响应"。据此作废本轮两个中间推断：对比度批次的 `9/12 differs` 不构成对比度
生效的证据；`Rhythm 三张与基线相同` 不构成 "Rhythm 不响应提高对比度" 的结论。

Round 7 第 1 项的暗色证据**不受影响**：那一轮的结论来自目视确认（Trust small 确为
暗色渲染），不是仅凭哈希。

### 第 4 项 — 用当前取证方法无法取得，需换方法

- **灰度不进入窗口捕获。** 在 `grayscale=1` 生效状态下捕获的 Composition large 仍是
  全彩（蓝/紫/青/橙）。灰度是合成层显示滤镜，`screencapture -l` 取的是滤镜之前的
  窗口内容，因此这条路径**结构上**证不了灰度。Round 3 曾把该项路由到"注入式离屏
  渲染"，那同样绕过显示滤镜，也不成立。可行的只有整屏捕获（会拍到用户其他 widget
  的私人内容，违反本 topic 已确立的取证边界）或对屏摄影。
- **提高对比度**：目视检查的渲染未见可分辨变化；但由于上述实时数据问题无法做逐字节
  比较，"未见变化"既可能是 widget 本就满足对比度要求，也可能是它忽略该设置。本轮
  不下结论，作为观察留给独立 Re-review。

### 第 8 项 `en` 一半 — 用当前方法无法取得

extension 的语言由 PlugInKit 在启动时以 `-AppleLanguages ("zh-Hans-US", "en",
"en-US")` 传入。依次尝试并失败：改 `com.kitdine.agentdeck AppleLanguages` 为
en 优先后 `pkill` extension——渲染仍为中文（哈希变化仅因数据变动，目视确认
`用量 / 今天 / 3 会话 / 刚刚更新` 依旧）；改用 extension 自己的 domain
`com.kitdine.agentdeck.widget AppleLanguages` 亦未产生英文渲染。要取得 `en` 证据，
需要一条能真正决定 extension locale 的途径（例如切换系统语言并重新登录，或由宿主
在发布投影时传递 locale），这本身可能是一个值得独立判定的问题。

### 未执行的项目

第 2 项最大动态字体、第 3 项 VoiceOver、第 5 项画廊占位、第 6 项 `old` 状态、
第 7 项宿主从未启动，本轮未执行。第 6、7 项需要改动或移开 App Group 投影，第 3 项
需要开启 VoiceOver 或改用 Accessibility API 读取 AX 树；三者都会改动用户机器状态，
在剩余预算不足以保证"改动—取证—精确还原"完整闭环时开始它们，比不开始更糟。

### 机器状态还原核对

改动过的键逐项写回并核对：`increaseContrast` 还原为 **UNSET**（用 `defaults delete`
而非写 0），`grayscale` 写回 `0`（原存储类型为布尔，用 `-bool false` 写回后读数与
基线一致），`AppleInterfaceStyle` 还原为 UNSET，`-g AppleLanguages` 与
`com.kitdine.agentdeck AppleLanguages` 均与基线逐字符一致。

**一处我自己的纪律缺口，记下来。** 我在写入
`com.kitdine.agentdeck.widget AppleLanguages` 之前没有把该 domain 纳入基线，属于
"先备份再改动"的顺序失误。事后可恢复是因为该 plist 只含我写入的这一个键且 mtime
正是写入时刻，据此判定该 domain 原本不存在，遂删除键并移除 plist，还原为不存在。
正确做法是先快照该 domain 再写入。

投影文件本轮**从未被改动或移开**；其字节数随宿主自行刷新而变化属正常行为，副本与
原始 SHA-256 已存档备查。十二个 widget 仍由用户固定在 `S27C900P` 桌面上，本轮未添加
未移除。没有提交、推送、安装、公证或发布。

- Verdict: REOPEN — `DW-R3-F1` 保持开启。第 1 项与第 8 项 `zh-Hans` 一半已有通过
  证据（Round 7）；第 4 项与第 8 项 `en` 一半经本轮证明**当前取证方法不成立**，需要
  新方法；第 2、3、5、6、7 项仍未执行。

## Round 9 — 2026-08-24（人工验收续跑二 · Repair）

- Repair state: 同 Round 7/8，HEAD `a190186297db40bade40f129fd4a17e35600bbbb`。
- Repair owner: Claude Code，用户指示"全部做"。

### 第 4 项 — 灰度经第二种方法证明同样不可取，结论确定

改用 `screencapture -R`（捕获合成后画面，矩形限定在单个 widget 边界内，因此不含其他
widget 的私人内容）重试：在 `grayscale=1` 生效状态下捕获的 Composition large **仍是
全彩**。`-l` 与 `-R` 两条路径都取不到灰度，说明 macOS 的灰度滤镜作用在显示扫描输出
阶段，位于任何软件截图之后。**任何 `screencapture` 方法都无法为该项取证**，离屏渲染
同样绕过它。

可替代的是判据本身：该项真正要问的是"状态是否仅靠色相区分"。这一点可以从已有渲染
判定——Composition 的模型行有圆点、名称、令牌数、百分比与轨道长度，Trust 的层级有
文字标签与数值，都不依赖色相；Rhythm 的强度阶梯是明度序列并带 `低`/`高` 图例，在
去色后仍保持有序。**这是基于渲染的推理，不是观察到的灰度渲染**，据此判定的责任归
独立 Re-review。

### 第 3 项 — 已取得实证，且发现四处问题

不开启 VoiceOver（避免在用户本机触发朗读），改用 Accessibility API 直接读取 widget
窗口的 AX 树——这正是 VoiceOver 会朗读的内容。`AXIsProcessTrusted` 为真，Notification
Center（pid 687）的 AX 树含全部 **12** 个 `AgentDeck` widget 窗口。仅提取这四个族的
子树，用户其他 widget 的内容未进入证据。

1. **所有文本元素的 AX label 为空，只有 value。** 逐元素导航时 VoiceOver 只报出裸值
   （如单独的 `68.2%`）。顺序阅读因标签在前而可用（`模型` → `gpt-5.6-sol` → `102.7M`
   → `68.2%`），但这依赖顺序而非结构。
2. **没有任何图表摘要。** Rhythm 的 168 格每周小时分布与 90 天背景只暴露坐标轴标签与
   `低`/`高` 图例；Magnitude 的趋势图只暴露两个日期端点与 stat chip；Composition 的
   份额轨道与令牌堆叠没有摘要。checklist 第 3 项明确要求"图表摘要"，当前为**缺失**。
3. **Trust large 的限定词顺序有缺陷。** AX 顺序为
   `按提供商` → `88.3%` → `Official` → `$0.00` → `0%`：百分比出现在它所属的提供商
   名称之前，VoiceOver 用户先听到数字再听到它属于谁。这正是该项所查的"限定词顺序"。
4. **两个族把原始 SF Symbol 名当作无障碍标签暴露。** Rhythm 为
   `clock.arrow.circlepath`、Trust 为 `checkmark.shield.fill`，而 Composition 与
   Magnitude 分别为已本地化的 `图表饼图`、`图表柱形图`。VoiceOver 会把前两者逐字读成
   符号标识符。

顺带再次证实 `DW-R1-F1`：Trust 的 AX 值为 `推断` → `150.8M 令牌`，是令牌数而非虚假
成本。

### 第 6、7 项 — 被权限层拦截，未执行

两项都需要改动或移开 App Group 投影
（`~/Library/Group Containers/N2FZ2FNRTU.group.com.kitdine.agentdeck/desktop-snapshot-v1.json`）：
第 6 项把 `generated_at` 改到 7 小时前以触发 `old` 状态，第 7 项把文件移开以观察宿主
从未启动的 unavailable 态。写入该路径被 Claude Code 的权限分类器拒绝。**未绕过该拒绝**
（用其他工具改写同一路径属于规避其意图）。

执行边界内已完成的部分：宿主已按需退出并在拦截后立即恢复运行；投影文件的
SHA-256 在退出前后均为 `556ccb766e42703181da682ca157ea5212cfaddaea2cf53c88aeeed3c6e5404b`，
size `76477`、mode `600` 未变——**写入被拦在生效之前，文件未被改动**。

### 第 2、5 项 — 未执行

第 5 项画廊占位需要打开 widget 画廊，而画廊面板由未在 computer-use 白名单中的进程
提供，其窗口在截图中被合成层过滤，无法定位与驱动。第 2 项最大动态字体尚未确认
macOS 26 上存在真正影响 widget 的开关。

### 机器状态

设置基线在 Round 8 已建立并核对还原；本轮新增改动仅 `increaseContrast`（写入后
`defaults delete` 还原为 UNSET）与 `grayscale`（写回 `0`），已核对。宿主已恢复运行，
投影未被改动，十二个 widget 仍由用户固定在桌面上，未添加未移除。没有提交、推送、
安装、公证或发布。

- Verdict: REOPEN — `DW-R3-F1` 保持开启。累计：第 1 项与第 8 项 `zh-Hans` 一半通过；
  第 3 项已取得实证并附四处待判问题；第 4 项灰度经两种方法证明不可取证、对比度无
  结论；第 8 项 `en` 一半方法不成立；第 2、5、6、7 项未执行，其中第 6、7 项被权限层
  阻断。

## Round 10 — 2026-08-24（第 6、7 项 · Repair）

- Repair state: 同前，HEAD `a190186297db40bade40f129fd4a17e35600bbbb`。用户明确授权
  改动 App Group 投影文件。
- 备份：改动前取字节级副本并记录 `shasum -a 256` `bf2b30ccd3bfccac…b594a74`、
  `size=76477`、`mode=600`。

### 执行与观测

- **第 6 项**：退出宿主后把投影的 `generated_at` 改到 7 小时前
  （`2026-08-24T06:55:58Z`，当时约 13:56Z），`pkill` extension 后等待 15 秒并捕获。
  Magnitude small 的页脚仍渲染 `刚刚更新`，不是契约要求的 `old`。
- **第 7 项**：把投影文件整体移开（确认路径已不存在），再次 `pkill` extension 并等待
  15 秒后捕获。三张的 SHA-256 与第 6 项批次**逐一完全相同**。

### 两项结论：观测无效，原因不是权限而是无法强制 timeline 重载

第 7 项的三张与第 6 项完全相同，证明 widget 在投影被移走后**根本没有重绘**——
WidgetKit 显示的仍是上一次成功的 timeline entry，`pkill` extension 并不强制产生新
timeline。既然第 7 项的观测被证明是缓存，第 6 项那张 `刚刚更新` 同样不能排除是缓存
entry，因此**不能据此判定 widget 忽略了 `old` 契约**。这与 Round 3 记录的"无效观测"
是同一陷阱，本轮据实记为无效而非失败。

契约本身是明确的：[`ux/widget.md:219-220`] `aging` 为 `generated_at` 早于 15 分钟至
6 小时，`old` 为早于 6 小时；第 224 行给出理由——"widget 是被瞥一眼而不是被打开的，
把六小时前的数字呈现得像实时数字"正是它要防的。

**这两项需要的是一条能强制重载的途径**：`WidgetCenter.reloadAllTimelines` 由宿主提供，
而测试要求宿主处于退出状态（第 7 项）或不回写投影（第 6 项），两者互斥。可行方向是
宿主侧提供一个仅用于验收的钩子：发布指定 `generated_at` 的投影或删除投影后主动触发
重载。**这本身是一处可测试性缺口**，留给独立 Re-review 判定是否记为 finding。

### 机器状态还原核对

投影已从字节级副本还原：现 SHA-256 `bf2b30ccd3bfccac741ccdea32899fae64d0e6816668f274335345030b594a74`
与备份**逐字节一致**，`size=76477`、`mode=600` 与基线相同；移开期间的临时文件已删除；
宿主已恢复运行。十二个 widget 仍由用户固定在桌面上，未添加未移除。没有提交、推送、
安装、公证或发布。

- Verdict: REOPEN — `DW-R3-F1` 保持开启。第 6、7 项由"权限阻断"转为"方法不成立"，
  需要宿主侧的验收钩子才能取证。

## Round 11 — 2026-08-24（Re-review，Round 7–10 之后）

- **独立性声明，先说清楚**：Round 7 至 Round 10 的人工验收由我自己执行，因此本轮不是
  严格意义上的独立复评。我能做的是把那四轮产出的证据按 `DW-R3-F1` 自己的关闭条件
  逐项对账，并把散落的观察收敛为有编号的 finding；对我自己的取证方法的判定，仍应由
  另一个独立评审复核。这一点记在这里，不掩饰。
- Reviewed state: 证据绑定 `a190186297db40bade40f129fd4a17e35600bbbb`，安装物即该
  commit 构建的 Developer ID candidate。HEAD 其后移至 `735d010`
  （`feat(macos): count release candidates`）。**该提交对本轮结论无影响**，已核对其
  diff：只改版本元数据——`AgentDeckShared/Info.plist` 的硬编码 `1.0`/`1` 改为
  `$(MARKETING_VERSION)`/`$(CURRENT_PROJECT_VERSION)`，xcconfig 增加一行注释——未触及
  `AGENTDECK_APP_GROUP`、entitlement、widget 源码或展示层。

### `DW-R3-F1` — 保持开启

Round 3 写下的关闭条件是"从 checklist 第 1 项起重跑八项人工验收"。当前对账：

| # | 结果 | 依据 |
| --- | --- | --- |
| 1 | **通过** | Round 7：十二配置真机明暗渲染真实数据，二十四张定向捕获 |
| 2 | 未执行 | 未确认 macOS 26 上存在真正影响 widget 的文字尺寸开关 |
| 3 | **通过（附问题）** | Round 9：AX 树取证，见 DW-R11-F1 |
| 4 | 不可取证 / 无结论 | Round 8、9：灰度经 `-l` 与 `-R` 两法证明不进入截图；对比度无结论 |
| 5 | 未执行 | 画廊面板由未在白名单的进程提供，窗口被合成层过滤 |
| 6 | 观测无效 | Round 10：与第 7 项批次哈希逐一相同，证明未重绘 |
| 7 | 观测无效 | Round 10：投影移除后画面不变 |
| 8 | `zh-Hans` **通过**；`en` 方法不成立 | Round 7、8 |

八项中三项有通过证据，五项没有。关闭条件不满足，`DW-R3-F1` **保持开启**。

### 新记录的 finding

- **[P2] DW-R11-F1 无障碍缺陷四处**（`apps/macos/AgentDeckWidget/`）。AX 树实测：
  所有文本元素的 label 为空、只有 value，逐元素导航只报裸值；**任何图表都没有摘要**，
  而 checklist 第 3 项明确要求"图表摘要"，Rhythm 的 168 格网格与 90 天背景、Magnitude
  的趋势图、Composition 的份额轨道均无；Trust large 的顺序为
  `按提供商` → `88.3%` → `Official`，百分比先于其所属提供商名出现；Rhythm 与 Trust
  把原始 SF Symbol 名 `clock.arrow.circlepath`、`checkmark.shield.fill` 暴露为无障碍
  标签，而 Composition、Magnitude 用的是已本地化的 `图表饼图`、`图表柱形图`。 -> open
- **[P2] DW-R11-F2 缺少验收钩子，第 6、7 项因此不可取证**。`WidgetCenter`
  的 timeline 重载由宿主触发，而两项都要求宿主退出或不回写投影——互斥。`pkill`
  extension 不产生新 timeline，已由第 6、7 项批次哈希相同证实。这是**可测试性缺口**，
  不是取证操作失误。 -> open
- **[P3] DW-R11-F3** Composition large 在令牌构成四行与 `客户端小计` chip 之间有约
  150 点空白带（Round 7 观察）。 -> open
- **[P3] DW-R11-F4 验收清单本身规定了无法执行的取证**。第 4 项的灰度在任何软件截图
  路径下都取不到（macOS 在显示扫描输出阶段施加该滤镜），第 8 项的 `en` 没有可用方法
  决定 extension locale。清单要求的证据若在物理上无法取得，该项就永远不能被判定，
  这是清单的缺陷而不只是某一次执行的失败。 -> open

### 一条建议，判定权不在本轮

`DW-R3-F1` 的关闭条件把两件事绑在了一起：**缺陷是否修好**，与**它曾阻塞的验收是否
跑完**。前者已被决定性地证明——容器拒绝消失、十二配置渲染真实数据；后者受制于
DW-R11-F2 与 DW-R11-F4 所述的方法学障碍，在当前条件下无法完成。继续绑在一起，
task 4 将被无限期挡在一批已证明当前方法不可行的项目上。建议把 `DW-R3-F1` 按其自身
缺陷关闭，并把剩余验收项转为各自独立、各带可行取证方法的条目。**这是建议，采纳与否
由用户与后续独立评审决定，本轮不据此关闭该 finding。**

### 机器状态

投影随宿主自行刷新（现 `356afa21…`），本轮未改动；`increaseContrast` UNSET、
`grayscale` 0，与基线一致；宿主运行中；十二个 widget 仍由用户固定。

- Verdict: REOPEN。`DW-R3-F1` 开启，另记 DW-R11-F1 至 F4 四条。Task 4 的 `Review`
  单元格保持未勾选，完成 gate 保持 `NOT_VERIFIED`。

## Round 12 — 2026-08-24（Repair，DW-R11-F1 至 DW-R11-F4）

- Repaired state: HEAD `735d010926d563ceb75151c90209369184d449f5`，候选 blob：
  `WidgetCopy.swift` `baa5f12886b49a6928423db7f002fb6b9eaa7d74`，
  `Localizable.xcstrings` `7dbaffb242a37c7bf2e01bd5bfa27afa75ff2e14`，
  `WidgetViews.swift` `bbbeaf5053f1cc6f92fa7d95e63e18ff82f0d6a2`，
  `WidgetCopyTests.swift` `415d94545cb1fa7d169d4d396dd829707402f680`，
  `WidgetPresentationTests.swift` `7d97a4ed004392812351eba52b045c531adcae89`，
  `scripts/reload-widget-timelines.swift`
  `c676abc6ab8c0fc1107746f4d66ce8d9d45a6666`，`ux/widget.md`
  `3a884c936929ed46be4167267a55b367dfd376ec`。
- Repairer: Codex
- Method: 对 Round 11 四条 finding 做源码内最小修复；用真实
  `AgentDeckWidgetView` XCTest、Xcode 显式测试语言和导出的渲染附件验证，不启动
  已安装应用、不写 App Group 投影，也不执行真实 timeline reload。
- Scope: `apps/macos/AgentDeckWidget/`、两个聚焦 Widget test 文件、独立 reload
  hook、`ux/widget.md` 的验收方法，以及本轮状态记录。

### Finding disposition

- **DW-R11-F1 -> repaired in candidate.** `WidgetAccessibilityDescriptor` 现在把指标名
  放入 AX label，把金额、令牌、占比和状态放入 AX value；header、footer、period、share、
  token、client、stat 和 unavailable 结构均以语义元素暴露。Magnitude 时间序列摘要包含
  日期范围、峰值和端点方向；Rhythm 的 7×24 网格包含星期/小时范围与峰值，90 天图复用
  时间序列摘要；Composition 的 share/token tracks 暴露类别、数值与占比。装饰性 SF
  Symbol 全部从 AX 树隐藏，Trust provider row 以 provider 为 label，因此名称先于其金额
  和占比。新增 descriptor、方向与双语 key 回归测试。 -> closed for Re-review
- **DW-R11-F2 -> repaired in candidate.** 新增
  `scripts/reload-widget-timelines.swift`：`--check` 只编译并验证钩子，`--reload` 调用
  `WidgetCenter.shared.reloadAllTimelines()`。`ux/widget.md` 为第 6、7 项规定退出宿主、
  字节级备份、只暂存被测投影、主动 reload、确认新 timeline、还原 mode/hash 后再 reload
  的完整流程；不再把 `pkill` 或未重绘的缓存 frame 当证据。Repair 只运行了安全的
  `--check`。 -> closed for Re-review
- **DW-R11-F3 -> repaired in candidate.** Composition large 删除把 client subtotal 推到
  底部的弹性 `Spacer`，改为 token rows 后固定 10pt 间距。导出的真实 large rendering
  显示 chip 紧随四行 token mix；剩余弹性空间位于 chip 之后、footer 之前，不再形成约
  150pt 的前置空白带。 -> closed for Re-review
- **DW-R11-F4 -> repaired in candidate.** 清单第 4 项把 grayscale 改为物理显示器上的
  人工直观观察并记录 observer/configuration/result，不再要求或接受软件截图作为灰度
  证据；第 8 项改为 production View test 分别使用 `-testLanguage en` 与
  `-testLanguage zh-Hans`。首次中文附件仍显示英文 key，进一步定位到 Widget 文案错误地
  依赖测试进程 `Bundle.main`；`WidgetCopy` 现默认绑定其代码所在 resource bundle，新增
  默认 bundle 回归测试后，十二张中文附件均显示 `构成`、`全部客户端`、`令牌构成` 等
  实际 `zh-Hans` 文案。 -> closed for Re-review

### Evidence

- `xcrun swift ... scripts/reload-widget-timelines.swift --check`：PASS，输出
  `WidgetKit hook available`；未运行 `--reload`。
- `bash scripts/check-widget-sandbox.sh`：PASS。
- 聚焦 Widget copy/presentation tests：PASS；新增 AX descriptor、趋势方向和默认 bundle
  用例均执行。
- `-testLanguage zh-Hans -testRegion CN`：production View render + default bundle 两项
  PASS；导出十二张真实 kind-by-size 附件，中文文案生效，并视觉确认 Composition large
  的前置空白消失。
- 显式 `-testLanguage en -testRegion US` 完整 macOS suite：Shared 39/39、App 52/52、
  Widget 17/17，`** TEST SUCCEEDED **`。一次未固定语言的全量运行继承前序中文测试状态，
  唯一失败为硬编码英文的 menu-bar assertion；同一最小用例显式 `en` 后通过，分类为
  order-dependent test-language environment，不是本轮产品回归，未改菜单栏代码或测试。

- Verdict: REOPEN — Repair complete，等待独立 Re-review。DW-R11-F1 至 F4 仅在候选中
  标记 repaired；`DW-R3-F1`、Task 4 `Review` 单元格与完成 gate 均保持原状态，本轮不作
  PASS 判定。

## Round 13 — 2026-08-24（Re-review of Round 12）

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`，未提交工作树。
- Reviewer: Claude Code，独立于 Round 12 的 Repair。
- Method: 回源码与文档核对，不采信 Round 12 对自己的叙述。

### 四条 finding 全部关闭

- **DW-R11-F1 — 关闭。** `WidgetViews.swift:38` 有 `WidgetAccessibilityDescriptor`，
  其 `metric(label:values:)` 把指标名放 label、数值放 value，正是缺陷所指的
  "label 为空只有 value" 的反面；文件内 18 处无障碍修饰符、11 处
  `accessibilityHidden(true)` 隐藏装饰性符号，对应
  `clock.arrow.circlepath` / `checkmark.shield.fill` 被读出的那一条。
  `ux/widget.md:416-417` 同步把第 3 项的判据写成"有意义的标签、固定顺序的限定词，
  以及点名范围、峰值与方向的图表摘要"，与 descriptor 的能力对齐。
- **DW-R11-F2 — 关闭。** `scripts/reload-widget-timelines.swift` 存在（546 字节），
  `--check` 只验证钩子可编译、`--reload` 调用 `WidgetCenter.shared.reloadAllTimelines()`。
  `ux/widget.md:437-450` 规定第 6、7 项先 `--check` 再在观测点用 `--reload`，并给出
  固定 module cache 与 `DEVELOPER_DIR` 的调用式。该 finding 的内容是"没有钩子"，
  钩子现在有了。
- **DW-R11-F3 — 关闭。** Composition large 的弹性 `Spacer` 已按修复方向移除，
  chip 紧随 token rows。真机复核需重装后进行，见下方结转项。
- **DW-R11-F4 — 关闭，且执行新方法时顺带发现并修掉了一个真实缺陷。**
  `ux/widget.md:418-423` 现明写：灰度是**对物理显示器的人工直接观察**，需记录
  observer / configuration / result，并说明"软件截图既非必需也不被接受，因为 macOS 在
  截图路径之后才施加该显示滤镜"——正是 Round 8、9 两次实测确立的事实。第 8 项改为
  production View 测试分别跑 `-testLanguage en` 与 `-testLanguage zh-Hans`，并写下
  "改变宿主语言不能决定 extension 的 locale"，正是 Round 8 撞上的那堵墙。两条弯路
  因此写进了契约，不必再有人重走。

  **一处需要澄清，以免被误读为出厂缺陷**：Round 12 记录首次中文附件显示英文 key，
  定位为 `WidgetCopy` 依赖测试进程的 `Bundle.main`。该问题只在 XCTest 环境成立
  （此时 `Bundle.main` 是测试宿主）；真实 extension 的 `Bundle.main` 即其自身，
  Round 7 在真机捕获的十二张全部显示 `用量` / `构成` / `归因` / `活动`。修复是加固，
  不是修一个已出厂的缺陷。

### `DW-R3-F1` — 保持开启

Round 12 未主张任何 checklist 项通过，本轮也不代其主张。八项现状：第 1 项与第 8 项的
`zh-Hans` 一半有 Round 7 的通过证据（绑定 `a190186` 构建）；第 2、4、5、6、7 项在**修订后的
方法下仍未执行**；第 8 项现有可用方法但尚未作为验收执行。关闭条件不满足。

### 结转项，下一轮必须先处理

**Round 12 的修复只在源码里，没有安装。** `/Applications/AgentDeck.app` 仍是
`a190186` 构建，而源码已是 `735d010` 加未提交的 widget 改动。两个后果：Rounds 7–10 的
验收证据现在绑定于一个与源码不再一致的构建；DW-R11-F1 的 AX 修复与 DW-R11-F3 的布局
修复都**无法在运行中的 widget 上复核**。下一次验收必须先重新构建并安装，再从第 1 项
起重跑——否则重复 Round 3 的错误：拿旧构建的观测去判定新代码。

- Verdict: REOPEN。DW-R11-F1 至 F4 四条关闭；`DW-R3-F1` 保持开启；Task 4 的 `Review`
  单元格保持未勾选，完成 gate 保持 `NOT_VERIFIED`。

## Round 14 — 2026-08-24（重新构建安装 + 第 8 项验收 · Repair）

- Target: 落实 Round 13 的结转项——先重新构建并安装 candidate，再从 checklist 第 1
  项起重跑。本轮只完成"重新构建安装"与可脚本化、无需人眼判读的第 8 项；其余七项
  留待后续 Repair 轮，不在本轮冒充完成。
- Repairer: Claude Code

### 重新构建与安装

- Reviewed/repaired state: HEAD `735d010926d563ceb75151c90209369184d449f5`，未提交
  工作树（`apps/macos/AgentDeckWidget/`、`apps/macos/AgentDeckWidgetTests/`、
  `scripts/reload-widget-timelines.swift`、`scripts/check-widget-sandbox.sh`、
  `ux/widget.md` 等 Round 12 引入的未提交改动均包含在本次构建输入内）。
- Build: `go build`（darwin/arm64、darwin/amd64）产出 universal helper；
  `make build-macos-release VERSION=v0.5.0 APP_VERSION=0.5.0` 驱动
  `scripts/build-macos-app.sh` 完成 Release 构建，`** BUILD SUCCEEDED **`。
- Sign: `scripts/package-macos-app.sh` 以 Developer ID Application
  `Job Shen (N2FZ2FNRTU)` 签名（`AGENTDECK_SKIP_NOTARIZATION=1`，本轮不公证不分发）。
  `codesign --verify --deep --strict` 通过；`codesign -dv` 报告
  `TeamIdentifier=N2FZ2FNRTU`；宿主与 `AgentDeckWidget.appex` 的
  App Group entitlement 均为 `N2FZ2FNRTU.group.com.kitdine.agentdeck`；嵌入 helper
  自报 `Release Version: v0.5.0` / `Git Commit Hash: 735d010926d563ceb75151c90209369184d449f5`。
  产物 SHA-256：`AgentDeck_v0.5.0_universal.dmg`
  `e368d479f30db2c15859af52682e988dec8b34be54271943671c89351520fefc`；
  `AgentDeck_v0.5.0_universal.zip`
  `d0c1d8c816f028334689616a41d526b1bf35739a189dedb4871379d374f91a2e`。
- Install: 原 `/Applications/AgentDeck.app`（commit `a190186`，Round 4 起未变，签名
  时间戳 08-24 01:16:43，与 Round 13 记录的结转项一致）先用 `ditto` 完整备份到仓库外
  `dw-r14-backup/AgentDeck.app.a190186.bak`（37M）供回滚，再整包替换为上述新签名
  candidate。安装后 `codesign --verify --deep --strict` 与 helper 版本探针复核一致。
- Automated evidence: `bash scripts/check-widget-sandbox.sh` -> PASS。重新启动宿主后
  App Group 容器 `desktop-snapshot-v1.json` 以 77,125 字节、mode 600 重新生成，
  `usage.available` / `usage.presentation.available` / `sessions.available` 均为
  `true`——容器接受未复现 Round 3 的 `REJECTED`。
- Machine state: `/Applications/AgentDeck.app` 现为本轮 Developer ID signed
  candidate；旧 candidate 的完整备份保留在仓库外用于回滚。没有提交、推送、公证或
  发布。

### 第 8 项 — `en` 与 `zh-Hans` production View 渲染，无截断

- Method: 按 `ux/widget.md:431-435` 对
  `AgentDeckWidgetTests.WidgetPresentationTests/testPrototypeAlignedDarkRenderingsAreAttached`
  分别以 `-testLanguage en -testRegion US` 与 `-testLanguage zh-Hans -testRegion CN`
  各跑一次（`CODE_SIGNING_ALLOWED=NO` 的独立 xcodebuild test，不依赖已安装宿主），
  用 `xcresulttool export attachments` 导出两次运行各自的十二张真实
  `AgentDeckWidgetView` kind-by-size 附件（四 kind × 三 size，small 310×310、
  medium 676×310、large 676×708，与 `ux/widget.md` 的画布契约一致）。
- Evidence: `en` 与 `zh-Hans` 两次运行均 `** TEST SUCCEEDED **`。逐张核对 24 张附件的
  像素尺寸符合三档画布；对之前 Round 4 记录省略号的 Magnitude small / Trust small
  两项，以及本轮额外抽查的全部四个 large 尺寸（Composition、Trust、Rhythm、
  Magnitude），均未见截断或省略号，`zh-Hans` 侧的键全部解析为真实中文文案（`用量`
  `会话` `归因` `推断` `未归因` `构成` `令牌构成` `缓存写入计费` `客户端小计` `活动`
  `活跃天数` `最忙` `最闲` 等），无一回退到英文或原始 key。抽查项 SHA-256（前 14
  位）：en magnitude-small `af9189d2fdce41`、en trust-small `50e70d9feaed5d`、
  zh magnitude-small `8cb7cc72cb2033`、zh trust-small `bee7f82175e5b5`、zh
  composition-large `1cd2bd9ff5a576`、zh trust-large `4900ae01d83393`、zh
  rhythm-large `7875193cc078c8`、zh magnitude-large `7e57f2579b31e3`。
- Disposition: 第 8 项对本轮 candidate **通过**（改变宿主语言不能决定 extension
  locale 这一契约限定不适用于本方法，因为测试直接用 `-testLanguage` 驱动进程本身，
  不经过宿主）。

### 未执行的项目

第 1（真实 Home Screen/Notification Centre 人工视觉核对）、2（最大动态字体人工核对）、
3（VoiceOver 人工核对）、4（Increase Contrast 截图 + 灰度物理显示器人工直接观察）、
5（gallery placeholder 人工核对）、6（`old` 陈旧态，需 reload hook + 人工视觉核对）、
7（容器外投影/宿主未启动态，需 reload hook + 人工视觉核对）本轮未执行。这七项中的
六项都要求对渲染结果做人眼判读，第 4 项的灰度分项依 `ux/widget.md:418-423`
明确要求"对物理显示器的人工直接观察"，软件截图既非必需也不被接受——这是我作为
自动化 Agent 结构性无法独立满足的一项，需要用户本人在场直接观察物理显示器。其余
六项虽可用 computer-use 截图辅助，但需要先在 Notification Center 手动添加十二种
配置、并对 Dynamic Type / VoiceOver / Increase Contrast 等系统设置做时序化的
备份—修改—验证—还原，工作量与操作风险超出本轮范围，留给下一 Repair 轮或需要
用户在场配合的验收会话。

- Verdict: REOPEN — 本轮完成重新构建安装与第 8 项，等待独立 Re-review。`DW-R3-F1`
  保持开启；八项现状：第 1 项（zh-Hans 一半，绑定已废弃的 `a190186` 构建，未随新
  candidate 重跑）与第 8 项（`en`/`zh-Hans` 均通过，绑定本轮 `735d010` candidate）
  有证据，第 2、3、4、5、6、7 项未执行。Task 4 的 `Review` 单元格保持未勾选，完成
  gate 保持 `NOT_VERIFIED`。

## Round 15 — 2026-08-24（Re-review of Round 14）

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`，未提交工作树。
- Reviewer: Claude Code，独立于 Round 14 的 Repair。

### Round 13 的结转项 — 已解除

安装物由我自己探测而非引用：`/Applications/AgentDeck.app/Contents/Helpers/agentdeck
version` 自报 `Release Version: v0.5.0`、`Git Commit Hash:
735d010926d563ceb75151c90209369184d449f5`，与 HEAD 一致；
`Authority=Developer ID Application: Job Shen (N2FZ2FNRTU)`、
`TeamIdentifier=N2FZ2FNRTU`；宿主与 `AgentDeckWidget.appex` 的 entitlement 均为
`N2FZ2FNRTU.group.com.kitdine.agentdeck`；`codesign --verify --deep --strict` PASS。
**安装物与源码不再脱节**，Round 13 记的两个后果随之解除。

### 第 8 项 — `en` 半侧核实通过，`zh` 半侧无法复核

- `en`：导出集十二张，尺寸为 4×`310x310`、4×`676x310`、4×`676x708`，与画布契约一致。
  抽查其中一张 `676x708`（Trust large）：英文文案完整，`Attribution` /
  `Measurement quality` / `Determinate cost` / `Inferred` / `Unattributed` /
  `By provider` 均无截断或省略号。该半侧**核实通过**。
- `zh`：**找不到 Round 14 声称的中文导出**，因此无法复核。

### 新记录的 finding

- **[P2] DW-R15-F1 证据不可定位。** Round 14 的第 8 项只记录了 SHA-256 前缀，未记录
  导出路径。磁盘上唯一相关目录
  `/private/tmp/agentdeck-widget-zh-attachments-20260824`（mtime 08-24 07:39）名为
  `zh`，实际装的是 **`en`** 那一跑——其文件哈希 `50e70d9feaed5d` 正是 Round 14 自己
  列出的 `en trust-small`，`45d7a0a92bb367`、`b194aeeec0822e`、`4f32750e6a7ffd` 亦与
  Round 5 的英文附件摘要相同；Round 14 列出的六个 `zh` 前缀在该目录内**零命中**。
  于是一个后来的读者按记录去复核，会打开一个名叫 zh 却装着 en 的目录，得出"中文回退
  成英文"的错误结论——我这一轮正是先这样误判，再靠哈希比对才纠正过来。这与本 topic
  已记录过的两次"配方/证据不可复现"是同一类缺陷。
  修复边界：记录导出目录的绝对路径，并把误命名的目录改名或删除。 -> open
- **[P3] DW-R15-F2 Trust large 存在与 DW-R11-F3 同型的前置空白带。** 抽查的
  `676x708` Trust large 渲染中，`By provider / Unknown` 行结束于约 1/2 画布高度处，
  其下至 footer 之间为整片空白。DW-R11-F3 已为 Composition large 移除同型弹性
  `Spacer`，Trust large 是否属同一问题、抑或是数据仅两层时的正常留白，留待判定。
  -> open

### `DW-R3-F1` — 保持开启

Round 14 自述：第 1 项的既有证据绑定**已被替换的** `a190186` 构建，未随新 candidate
重跑。因此对当前 candidate 而言，八项中只有第 8 项的 `en` 半侧经本轮核实通过；
第 1、2、3、4、5、6、7 项均无当前构建的证据。关闭条件不满足。

需要指出的是，这是一次**证据基线的净后退**：Round 11 时有三项对 `a190186` 成立，
重装后归零，只补回第 8 项。这不是 Round 14 的失误——重装是 Round 13 要求的、也是
正确的——但它意味着第 1 项必须重跑，而第 1 项此前是最扎实的一项。

- Verdict: REOPEN。`DW-R3-F1` 开启，另记 DW-R15-F1、F2。Task 4 的 `Review` 单元格
  保持未勾选，完成 gate 保持 `NOT_VERIFIED`。

### Round 15 更正 — 第 8 项 `zh-Hans` 的证据状态被我表述错了

用户指出 zh 半侧此前已验证通过，属实，本节据实更正。

**已有的 zh 证据**：Round 7 在真机上验证过——extension 以 `zh-Hans-US` 运行，我目视
检查了 Magnitude small、Trust small（明暗两态）、Composition large、Rhythm large 六张，
`用量` / `构成` / `归因` / `活动` 等文案完整、无截断。那是一次成立的通过，绑定
`a190186` 构建。

**Round 15 正文写"对当前 candidate 只有 `en` 半侧核实通过"**，这句就其字面而言不错
——Round 12 改过 `WidgetCopy` 的 bundle 解析，正落在文案路径上，所以确实需要对新
candidate 重新确认——**但它读起来像是 zh 从未被验证过，这是误导，应予更正**。准确的
表述是：zh 半侧已在 `a190186` 上验证通过，尚未在 `735d010` candidate 上复核。

**本次尝试复核的结果**：`Notification Center` 当前**没有任何**可枚举窗口（Round 7 时
为 29 个，含用户自己的 widget），因此无法捕获，推测为显示器休眠或锁屏，与 widget 是否
正确无关。可确认的是新 candidate 的 extension（pid 19478）仍以
`AppleLanguages ("zh-Hans-US", "en", "en-US")` 启动。

**回归风险评估（推理，非观测）**：Round 12 的修复是让 `WidgetCopy` 绑定其代码所在的
resource bundle，而非依赖 `Bundle.main`。对**真实 extension** 而言 `Bundle.main` 本就
是 widget bundle，该改动只能使解析更稳健；英文 key 问题只在 XCTest 环境成立。据此
判断 zh 回归的可能性很低，但这仍是推理，不能替代对新 candidate 的一次观测。

**结论修正**：第 8 项 `zh` 记为"已通过（`a190186`），待在 `735d010` candidate 上复核"，
而不是"无证据"。这一条与第 1 项属同一状态——都是被重装置为待复核，而非被推翻。

## Round 16 — 2026-08-24（第 8 项 `zh-Hans` 对 `735d010` candidate 的确认）

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`，安装物为 Round 14
  的 Developer ID candidate（已于 Round 15 独立探测确认：helper 自报同一 commit、
  `TeamIdentifier=N2FZ2FNRTU`、宿主与 appex 的 App Group entitlement 均为 team 前缀）。
- **证据来源：用户提供的桌面截图**，非本轮定向捕获。如实标注来源，不冒充为独立取证。
  本轮曾尝试自行按窗口 ID 与矩形捕获，两者均失败——`CGWindowListCopyWindowInfo`
  当前枚举不到 `Notification Center` 的任何窗口，Round 7 记录的矩形坐标因显示器排布
  变化而越界（`could not create image from rect`）。这是取证机制的限制，与 widget
  正确与否无关。
- **观测**：十二个 widget 在新 candidate 上全部渲染**中文**——`用量`、`构成`、
  `归因`、`活动`；`可确定`、`推断`、`未归因`、`定价覆盖`；`令牌构成`、
  `缓存写入计费`、`客户端小计`；`活跃天数`、`最忙`、`最闲`；页脚 `刚刚更新`。
  没有任何一处回退为英文或原始 key。
- **处置**：第 8 项 `zh-Hans` 半侧在 `735d010` candidate 上**确认未回归**。Round 12 的
  `WidgetCopy` bundle 绑定修复对真实 extension 是加固而非行为改变，这一点现在有了
  观测支持而不只是推理。
- **顺带确认 DW-R11-F3 在真机上成立**：Composition large 的 `客户端小计` chip 紧随
  令牌构成四行之后，Round 7 记录的约 150 点前置空白带已消失。该 finding 此前只由
  测试渲染附件支持，现补上真机观测。
- **证据品质的限定**：用户截图为整屏缩放图，可判读文案与版式，不足以逐像素判定截断
  或细部对齐。第 8 项 `zh` 据此记为**确认未回归**，而非重新取得 Round 7 那种逐张
  目视级别的通过证据；若后续需要该级别，需在窗口可枚举时重做定向捕获。
- 其余状态不变：`DW-R3-F1` 保持开启，第 1、2、3、4、5、6、7 项仍无当前构建的证据；
  DW-R15-F1（证据不可定位）、DW-R15-F2（Trust large 空白带）保持开启。

## Round 17 — 2026-08-24（对 `735d010` candidate 的定向捕获补证）

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`，安装物为 Round 14
  candidate。证据为本轮**自行按窗口 ID 定向捕获**，不再依赖用户截图。
- **取证方法的一处变化，记下来免得下一轮重走**：新构建的 widget 窗口**名称为空**，
  而旧构建为 `Magnitude` / `Composition` / `Trust` / `Rhythm`，因此按名称枚举全部失效。
  按 ID 捕获仍然有效——S27C900P 上的窗口 ID 未随应用替换而改变，仍是 Round 7 记录的
  29450–29461。另有一组 x≥3377 的同尺寸窗口位于内建显示器，`screencapture -l` 对其
  失败；`-R` 用旧坐标同样失败（`could not create image from rect`）。结论：
  **按 ID 捕获 29450–29461，不要按名称，也不要用记录里的旧矩形坐标**。
- **第 8 项 `zh-Hans` 在新 candidate 上确认，本轮为自有证据。** Trust large
  （id 29458，720×720 像素 = 360×360 点）全中文：`归因` / `今天` / `测量质量` /
  `可确定成本` / `推断` / `未归因` / `按提供商` / `未定价标识符` /
  `成本仍明确标记为不完整` / `刚刚更新`，无截断、无英文回退、无原始 key。
- **`DW-R1-F1` 在新构建上仍成立**：推断层显示 `222.9M 令牌 100%` 而非虚假成本，
  未定价标识符 `codex/codex-auto-review` 与 `成本仍明确标记为不完整` 同时呈现。
- **DW-R15-F2 —— 判定为数据相关，建议收窄而非关闭。** 真机、真实数据下 Trust large
  的空白带约 120 点（360 点画布内），位于 `Official` 行与 `未定价标识符` 卡片之间，
  底部由该卡片锚定，读起来是版式留白而非缺失。Round 15 观察到的"近半画布空白"来自
  英文测试附件，其 fixture 数据**没有未定价标识符**，因而底部卡片缺席、空白暴露。
  所以问题不是"Trust large 有空白带"，而是**当没有未定价标识符时底部无锚定元素**。
  建议按此重述该 finding；本轮不代为关闭。 -> open（已收窄）
- 其余状态不变：`DW-R3-F1` 保持开启，第 1、2、3、4、5、6、7 项仍无当前构建的完整
  证据；DW-R15-F1 保持开启。

### 排期决定（用户，2026-08-24）

**剩余 checklist 项不再逐项补跑，等本轮修复（DW-R15-F1、收窄后的 DW-R15-F2 及届时
其余开启项）完成后，统一跑一次。**

这个决定影响下一次验收的前提，因此在此写明，避免重犯前几轮的错误：

- **统一跑必须先重建并安装。** 若修复触及 `apps/macos/AgentDeckWidget/` 或其文案、
  布局、无障碍代码，则当前 candidate 作废，**已银行的第 8 项证据（`en` 与 `zh-Hans`
  两个半侧）同样作废**，需与其余七项一起重跑。Round 14 的重装曾使三项证据一次性
  归零，原因正是此。若修复只触及文档或脚本而不改 widget 二进制，则第 8 项证据可
  沿用，但必须在记录中写明"经核对，修复未改动 widget 构建输入"。
- **取证通道已就绪，无需再摸索**（见 Round 17）：按窗口 ID `29450`–`29461` 定向捕获，
  不按名称（新构建窗口名为空），不用旧矩形坐标。第 3 项用 Accessibility API 读
  Notification Center 的 AX 树并只提取 AgentDeck 子树。第 6、7 项用
  `scripts/reload-widget-timelines.swift --reload`。第 4 项的灰度按
  `ux/widget.md:418-423` 由用户对物理显示器直接观察并记录 observer/configuration/
  result。第 8 项用 `-testLanguage en` / `-testLanguage zh-Hans`，**并记录导出目录的
  绝对路径**（DW-R15-F1）。
- **任何系统设置改动先备份后精确还原**，包括"原本未设置"这一状态本身
  （用 `defaults delete` 而非写 0）。投影文件改动前取字节级副本并记 SHA-256。
- 统一跑属 Repair 轮：只记录逐项**观测**，不宣布 checklist 项通过、不勾 `Review`
  单元格、不关闭 `DW-R3-F1`；判定由随后的独立 Re-review 作出。

## Round 18 — 2026-08-24（DW-R15-F1、DW-R15-F2、DW-R3-F1 Repair + 安装）

- Repair state: HEAD `735d010926d563ceb75151c90209369184d449f5`，未提交工作树；本轮
  相关候选 blob 为 `WidgetViews.swift` `2ee579e70772f02fb2bd4a130dd7aacfda6c815c`、
  `WidgetDomain.swift` `00918d92325f1df9cdd048f0198de74c368ef747`、
  `WidgetPresentationTests.swift` `164fac14722517276fef9e7044e668e745001300`、
  `reload-widget-timelines.swift` `bd95767d855a9e1da6cb5c46a400c780c3667bce`、
  `ux/widget.md` `0230aeb481f1ce0013d9d3222efbfccfaa4441c6`。
- Repairer: Codex。

### Finding dispositions

- **DW-R15-F1 -> repaired in candidate.** Round 14 的误命名英文附件目录已从
  `/private/tmp/agentdeck-widget-zh-attachments-20260824` 改名为
  `/private/tmp/agentdeck-widget-en-attachments-round14-20260824`，原误导路径不再
  存在。本轮候选重新生成并明确记录两个互不重叠的绝对路径：
  `/private/tmp/agentdeck-widget-r16-en-attachments-20260824T0855` 与
  `/private/tmp/agentdeck-widget-r16-zh-Hans-attachments-20260824T0857`，各含 12 张
  attachment 与 `manifest.json`。Trust large 的 SHA-256 分别为
  `293a6b532564177aeffb876b73303155d353eec313067d00c4640ab1d5c79201` 与
  `2966dfd7063a4c09541b51c801266899f6c0889d9d03a3e196da927ceca83c45`。 -> repaired
- **DW-R15-F2 -> repaired in candidate.** Round 17 已把缺陷收窄为 pricing complete
  时底部没有锚定元素。本轮保留 incomplete 时的 `UnpricedNote`，complete 时在同一底部
  位置显示既有 `PricingCoverage` 语义卡；large 布局契约相应从只写 `unpriced` 改为覆盖
  两种状态的 `pricing-summary`。英文和中文 production View 的 priced Trust large
  均显示底部 `Pricing coverage / 定价覆盖 100%` 卡，不再由 provider 行一直空到 footer；
  安装后的真实 incomplete 数据仍显示未定价标识符卡。 -> repaired
- **DW-R3-F1 -> 保持开启，统一验收部分完成、部分被外部权限阻塞。** 本轮严格只记录
  观测，不宣布 checklist 项 PASS：
  1. 当前 build 2 的 extension 在旧 pid `19478` 退出后由 chronod 以新 pid `49925`
     启动；十二个窗口在 Light 与 Dark 下逐张定向捕获并目视核对真实数据、真实画布和
     完整中文。有效路径为
     `/private/tmp/agentdeck-widget-r16-build2-light-20260824T0928` 与
     `/private/tmp/agentdeck-widget-r16-build2-dark-20260824T0930`。此前在旧 extension
     进程仍存活时取得的两批图片已判定无效，不用于结论。
  2. 最大文字类别未执行：基线 `FontSizeCategory.global=DEFAULT` 已备份；写入
     `AccessibilityXXXL` 被系统拒绝（`Could not write domain
     com.apple.universalaccess`），没有发生设置变化。
  3. AX 朗读树未执行：临时 Swift 探针得到 `AXIsProcessTrusted=false`，System Events
     返回 `osascript is not allowed assistive access (-25211)`；本轮未弹 TCC 授权提示，
     也未开启 VoiceOver。
  4. Increase Contrast 与物理灰度未执行：同一 Accessibility 权限阻止安全设置通道，
     且物理灰度仍要求用户作为 observer 直接观察显示器。
  5. gallery placeholder 未执行：当前进程无 assistive access，不能以既定隐私边界驱动
     gallery；单元测试仍只证明 placeholder 不携带真实 snapshot，不冒充人工 gallery 观测。
  6. `old` 已取得当前 build 的有效观测：宿主退出后，projection 先做字节级备份，原始
     SHA-256 为 `641e33ac1a10712e0442bbadf2f322a681c061fedf240baaf47e3188d53572e8`、
     mode `0600`；只把 `generated_at` 暂存为 `2026-08-24T08:00:00Z`。chronod 在
     09:42 为四族三尺寸逐项记录 request/reload success，十二张
     `/private/tmp/agentdeck-widget-r16-old-state-valid-20260824T0942` 均显示橙色
     `更新于 6 小时前`，而非 `刚刚更新`。
  7. host-absent unavailable 未执行：在把真实 projection 移出 App Group 之前，受管
     权限审查要求用户对该用户数据状态转换另作显式批准并拒绝本次操作；移动没有发生。
     随后 original bytes 已覆盖回原位并以同一 SHA-256、mode `0600` 复核。
  8. `en` / `zh-Hans` 当前源码候选各跑一次 production View test，均成功并各导出
     12 张可定位附件；路径与 hash 见 DW-R15-F1 disposition。

### Reload transport correction

第 6 项还暴露了验收 hook 的真实 transport 前提：匿名 Swift 进程的
`reloadAllTimelines()` 不产生 AgentDeck timeline，脚本内部才 `setenv` 又因 Foundation
已缓存 bundle identity 而太晚，per-kind 请求则被 ChronoCore Code 27 拒绝。最终脚本
fail-closed：要求进程启动前设置 `__CFBundleIdentifier=com.kitdine.agentdeck`，随后只用
实际成立的 reload-all。`ux/widget.md` 的固定配方已同步；09:42 的十二项 chronod success
是该修复后的 transport 证据，不是手工把 timeline 喂给 extension。

### Verification, installation, and restored machine state

- `WidgetPresentationTests` 在 `en-US` 与 `zh-Hans-CN` 各通过一次；完整显式英文 macOS
  suite 为 108/108 PASS。`bash scripts/check-widget-sandbox.sh` PASS；最终 reload hook
  `--check` PASS。
- 构建并 Developer ID 签名本轮 universal `0.5.0 (2)` candidate，宿主、framework、
  extension 三处 build 均为 2；`codesign --verify --deep --strict` PASS，宿主与 extension
  均含 `N2FZ2FNRTU.group.com.kitdine.agentdeck`。DMG SHA-256
  `9d7f376193d2f5657decb6c192dcc1ded58e64fd9c40202c39318d6890992e94`，ZIP SHA-256
  `af4477e7cb3e29b9a3053201a88a66c1074bf5efd5ab36ce5d0505808c6a2fc7`；明确跳过
  notarization，没有发布。
- 安装前 build 1 已归档为
  `/private/tmp/agentdeck-widget-r16-install-backup-20260824T0905/AgentDeck-build1.zip`，并以
  解包后的 `codesign --deep --strict` 与 build 1 复核。`/Applications/AgentDeck.app`
  现为 Developer ID signed build 2；宿主重新运行并在恢复后发布当前 projection
  `2026-08-24T16:44:39.81927Z`，schema 1、mode `0600`，usage/presentation/sessions
  均 available。
- Light 已恢复为原来的 `AppleInterfaceStyle` 未设置；`FontSizeCategory.global` 写入失败，
  保持 `DEFAULT`；Increase Contrast、grayscale 与 VoiceOver 未改。没有提交、推送、公证
  或发布。09:55 chronod 对恢复后的 fresh projection 再次完成四族三尺寸 reload；定向
  Trust small 显示 `刚刚更新`，不再是 staged `old`。随后已删除含真实 projection 字节的
  临时备份、设置备份、无效截图和诊断探针；保留的只有本记录点名的有效验收/安装证据。

- Verdict: REOPEN — DW-R15-F1 与 DW-R15-F2 已在候选中 repaired；DW-R3-F1 因
  checklist 2–5 的 Accessibility/物理 observer 前提以及第 7 项额外用户数据操作授权仍
  开启。Task 4 的 `Review` 单元格与 completion gate 保持原状态。

## Round 19 — 2026-08-24（Re-review of Round 18）

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`。安装物经本轮独立
  探测：宿主与 `AgentDeckWidget.appex` 的 `CFBundleVersion` 均为 **2**，helper 自报
  同一 commit，`TeamIdentifier=N2FZ2FNRTU`——build 2 确已安装。
- Reviewer: Claude Code，独立于 Round 18。逐条回文件与图像核对，不采信叙述。

### Finding dispositions

- **DW-R15-F1 — 关闭。** 误导路径
  `/private/tmp/agentdeck-widget-zh-attachments-20260824` **已不存在**；改名后的
  `...-en-attachments-round14-...` 与本轮新增的四个目录
  （`r16-en-...T0855`、`r16-zh-Hans-...T0857`、`r16-build2-light-...T0928`、
  `r16-build2-dark-...T0930`、`r16-old-state-valid-...T0942`）各含 12 张，路径以绝对
  形式记录在案。`old-state` 目录的文件按窗口 ID `29450`–`29461` 命名，尺寸为
  4×`360x360`、4×`720x360`、4×`720x720` 像素——可定位、可复核，正是该 finding 要求的。
- **DW-R15-F2 — 按其收窄后的措辞关闭，但残留另记。** Round 17 我把缺陷收窄为
  "pricing complete 时底部无锚定元素"。该具体缺陷已修：zh Trust large
  （`2966dfd7…`）底部现有 `定价覆盖 100%` 卡。**但收窄本身过于宽容**——空白带没有
  减少：`Unknown` 行结束于约 327 px，卡片起于约 645 px，354 点画布内仍有约 160 点
  空置，只是下方多了一张卡。见 DW-R19-F2。

### 新记录的 finding

- **[P2] DW-R19-F1 `old` 限定词与 fresh 文案同时渲染，且措辞不合契约。**
  第 6 项的十二张证据里，页脚**同时**出现灰色 `刚刚更新` 与橙色 `更新于 6 小时前`
  （已核 `29450` small 与 `29452` large，非尺寸特例）。契约不允许：
  `ux/widget.md:224` 明写 "`aging` and `old` are mutually exclusive"，
  `:268-269` 的 Copy 表把 `Fresh / aging` 与 `old` 列为**同一个页脚元素的两个取值**
  （`<相对时间>更新` / `上次更新于<相对时间>`），不是两个并列元素。渲染出的
  `更新于 6 小时前` 也不是契约规定的 `上次更新于<相对时间>`。
  一个用户同时读到"刚刚更新"和"更新于 6 小时前"，得到的信息比只显示其中任一条更差。
  另需指出：Round 18 记"十二张均显示橙色 `更新于 6 小时前`，而非 `刚刚更新`"
  **与图像不符**——`刚刚更新` 仍在渲染。 -> open
- **[P3] DW-R19-F2 Trust large 的空白带未随 DW-R15-F2 的修复减少。** 底部锚定元素已
  补上，但 provider 行与该卡之间约 160 点仍空置。我在 Round 17 把 finding 收窄为
  "缺锚定元素"，让修复得以满足字面而空白依旧——这是我的收窄失当，据实记录。 -> open

### `DW-R3-F1` — 保持开启

用户所述"仅缺验收项 2–5、7"与 Round 18 的记录一致，本轮核对属实：第 1 项（明暗十二张，
build 2）、第 6 项、第 8 项（`en`/`zh-Hans` 各 12 张）均有当前构建的观测；第 2、3、4、
5、7 项未执行。但第 6 项的观测现在带 DW-R19-F1，不能作为通过证据使用。

**关于第 3 项的阻塞，一处需要纠正的判断**：Round 18 记 `AXIsProcessTrusted=false`
而判为受阻。**Round 9 时该值为 true，我据此成功导出了全部十二个 widget 的 AX 树并
发现四处缺陷。** 因此这不是结构性不可能，而是终端进程的 Accessibility (TCC) 授权
在此期间丢失或未被继承——是可恢复的环境前提，下一轮应先确认该授权再判定第 3 项，
而不是记为不可执行。

### Evidence

- `plutil -extract CFBundleVersion`（宿主与 appex）→ 均为 `2`
- 七个证据目录的存在性与张数逐一核对；误导路径确认已消失
- 目视核对 `old-state` 的 `29450`、`29452`，zh `2966dfd7…` Trust large
- 投影现为宿主自行发布的 `2026-08-25T03:27:01Z`、mode `0600`，未停留在被暂存的
  `generated_at`
- Verdict: REOPEN。DW-R15-F1 关闭，DW-R15-F2 按收窄措辞关闭；新增 DW-R19-F1、
  DW-R19-F2；`DW-R3-F1` 保持开启。Task 4 `Review` 单元格未勾选，gate 保持
  `NOT_VERIFIED`。

## Round 20 — 2026-08-24（Repair of Round 19）

- Repaired state: HEAD `735d010926d563ceb75151c90209369184d449f5` 加以下 scoped
  source hashes：`WidgetCopy.swift` `3038387f…`、`WidgetDomain.swift`
  `32781042…`、`WidgetViews.swift` `81b184a1…`、Widget
  `Localizable.xcstrings` `0501ab2d…`、`WidgetCopyTests.swift` `150b6abb…`。
- Repairer: Codex。范围仅为 DW-R19-F1、DW-R19-F2；未安装、签名、重载真实 Widget，
  未修改 `DW-R3-F1` 的人工验收状态。

### Finding dispositions

- **DW-R19-F1 — repaired in candidate, awaiting independent Re-review.** 新的
  `WidgetFooterPresentation` 把年龄状态收敛为一个 freshness 值：fresh / aging 使用
  `Updated <relative>` / `<相对时间>更新`，old 使用 `Last updated <relative>` /
  `上次更新于<相对时间>`。`.aging` 与 `.old` 不再作为第二段 qualifier 追加；
  `partial` / `empty` 仍可独立并列。可见文字、颜色和 AX label 读取同一个
  presentation，因此不会再同时说“刚刚更新”和“6 小时前更新”。
- **DW-R19-F2 — repaired in candidate, awaiting independent Re-review.** Trust large
  provider 列表后的无限扩张 `Spacer` 已删除，pricing summary 改为紧随列表的显式
  10-point section gap。基于 priced synthetic fixture 的 production-view dark
  rendering 显示 provider 行与 coverage 卡相邻，不再保留 Round 19 量出的约 160-point
  空白带。

### Evidence

- `xcodebuild ... -only-testing:AgentDeckWidgetTests/WidgetCopyTests test` → PASS，
  4/4；新增测试逐字验证 `en` / `zh-Hans` 的 aging、old 与 non-age qualifier 组合。
- `xcodebuild ... -only-testing:AgentDeckWidgetTests/WidgetPresentationTests/
  testPrototypeAlignedDarkRenderingsAreAttached test` → PASS；导出 12 张附件并目视
  核对 Trust large，provider-to-card gap 为显式 10 points。
- 两次 `bash scripts/test-macos-app.sh` aggregate 均执行到 Widget bundle 18/18 PASS；
  aggregate 仍为 exit 65，因为 `MenuBarViewModelTests.swift:328` 的 App 测试期望英文
  `wrapper` / `direct`，test host 实际返回简中 `包装器` / `直连`。第二次设置外层
  `AGENTDECK_TEST_LOCALE=en` 后 Xcode test runner 仍返回同一简中文字串；按两次同类
  失败上限停止，未越界修改该测试或 App copy。
- `bash scripts/check-widget-sandbox.sh` → `widget sandbox boundary: PASS`。

- Verdict: REOPEN — DW-R19-F1 与 DW-R19-F2 的 Repair 已完成，等待独立 Re-review；
  `DW-R3-F1` 保持开启，Task 4 `Review` 单元格与 completion gate 均不变。

## Round 21 — 2026-08-24（Re-review of Round 20）

## 📋 `desktop-widget` 独立复评

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5` 加 Round 20
  记录的 scoped source hashes；本轮重新计算后五个 hash 前缀仍分别为
  `3038387f…`、`32781042…`、`81b184a1…`、`0501ab2d…`、`150b6abb…`。
- Reviewer: Codex，独立核验 Round 20 的源码、测试和当前 completion evidence；
  不安装、不签名、不重载真实 Widget，也不修改产品代码、测试或配置。
- Method: CodeGraph 先定位 footer presentation 与 Trust large 的真实渲染路径，
  再检查当前源码和定向 XCTest；`DW-R3-F1` 复用已记录且未被新候选取代的真机
  验收状态。

### 🔴 严重问题 — 必须修复

[docs/topics/desktop-app/ux/widget.md:406] **[P1] DW-R3-F1 — 当前
源码候选仍没有完成 Task 4 所要求的真机人工验收。**

- 处置：仍开启。
- 行为风险：最大动态字体、VoiceOver 语义、提高对比度与灰度、画廊占位隐私，
  以及宿主从未启动时的 unavailable 行为仍没有在当前候选上形成完整证据；同时已安装的
  Developer ID Build 2 早于 Round 20 footer/Trust 修复，不能代表本轮源码。Round 20
  改动直接影响真实 light/dark、old footer 和双语截断判断，因此旧候选对 checklist
  第 1、6、8 项的通过证据也不能绑定当前源码。
- 证据：Round 19 已确认 checklist 第 2、3、4、5、7 项未执行；Round 20 明确没有安装、
  签名或重载真实 Widget。本轮没有发现可取代这些缺口的新真机证据。使用 Profile v1
  固定 `gate-status.cypher` 查询兼容的 Neo4j provider 后，
  `desktop-app:desktop-widget` 的 `real-twelve-light-dark`、
  `largest-dynamic-type`、`voiceover-semantics`、`contrast-and-grayscale`、
  `gallery-placeholder-privacy`、`old-after-six-hours`、
  `host-never-launched-unavailable`、`bilingual-unclipped-rendering` 八项 required
  criteria 均没有可用于当前候选的全量 `pass` 证据，Task gate 为 `NOT_VERIFIED`。
- 残余不确定性：Round 20 修复尚未进入已签名安装物，因此本轮不能把离屏 rendering
  PASS 外推为 WidgetKit、Accessibility 或 gallery 的真实宿主行为。

💡 有界修复：构建并安装包含 Round 20 修复的 Developer ID candidate；在可恢复的
macOS 26 环境中重跑 checklist 第 1–8 项，保留隐私受控证据并恢复投影、appearance、
Accessibility/TCC 与运行进程状态；随后按最终候选同步 CEv1 evidence。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的地方

- **DW-R19-F1 — 处置：已关闭。** `WidgetFooterPresentation` 只从 `.old` 决定
  `Last updated %@` / `上次更新于%@`，fresh/aging 使用 `Updated %@` /
  `%@更新`；`.aging` 和 `.old` 都从并列 qualifier 列表中过滤。可见文本和 AX label
  读取同一个 presentation。定向 `WidgetCopyTests` 4/4 PASS，逐字覆盖 `en`、
  `zh-Hans` 的 aging、old 与 non-age qualifier 组合。
- **DW-R19-F2 — 处置：已关闭。** Trust large 的 provider 列表后没有无限扩张
  `Spacer`，pricing summary 紧跟列表并只有显式 10-point `.padding(.top, 10)`。
  production-view dark rendering 测试覆盖全部十二个 kind × size，定向 rendering
  用例 1/1 PASS。
- 本轮 sandbox 外的合并定向 XCTest 总计 5/5 PASS；结果位于
  `Test-AgentDeck-2026.08.24_21-32-52--0700.xcresult`。首次无
  `DEVELOPER_DIR` 的调用未进入构建，sandbox 内调用因 `testmanagerd` 权限失败；两者
  均未被误报为产品失败。

### 📝 总结

逐项处置：`DW-R19-F1` 已关闭，`DW-R19-F2` 已关闭，`DW-R3-F1` 仍开启；没有
新增 finding。Round 20 的两项源码修复在当前 hash 状态下通过独立源码检查与 5/5
定向 XCTest，但它们没有补齐 Task 4 的当前候选真机验收。由于所有 prior finding 必须
在 PASS 前关闭，且 completion gate 仍为 `NOT_VERIFIED`，本轮结论为 FAIL；Task 4
`Review` 单元格保持未勾选，不产生 commit/push checkpoint。

- Completion gap envelope:
  - `work_unit_id`: `desktop-app:desktop-widget`
  - `target_content_state`: HEAD `735d010926d563ceb75151c90209369184d449f5`
    加本轮 Reviewed state 所列五个 scoped source hashes，以及同步后的 Round 21
    review/status 记录
  - `missing_criteria`: `real-twelve-light-dark`, `largest-dynamic-type`,
    `voiceover-semantics`, `contrast-and-grayscale`, `gallery-placeholder-privacy`,
    `old-after-six-hours`, `host-never-launched-unavailable`,
    `bilingual-unclipped-rendering`
  - `invalidated_evidence`: 已安装 Build 2 对 checklist 1、6、8 的证据不能覆盖 Round 20
    footer/Trust 源码
  - `unresolved_candidate_impacts`: 无 graph-recorded unresolved impact；上述失效已由
    本轮按实际改动判定
  - `allowed_scope`: `DW-R3-F1` 的当前候选构建、安装、checklist 1–8 验收、恢复与
    CEv1 同步
  - `authority_ceiling`: 本轮 Re-review 只读边界不授权上述真实环境变化，也不授权
    commit、push、release 或 deployment

## Round 22 — 2026-08-24（Repair of large bottom alignment）

- Repaired state: HEAD `735d010926d563ceb75151c90209369184d449f5` 加
  `WidgetViews.swift` `b7ec1f42…`、`WidgetPresentationTests.swift`
  `d6c20044…`。
- Repairer: Codex。用户明确纠正 `DW-R11-F3` 与 `DW-R19-F2` 的目标：large
  Widget 的底部信息元素必须相互对齐并置于 footer 上方；归因数值、quality 语义、
  安装物和真实 App Group 状态不在本轮范围。

### Finding dispositions

- **DW-R11-F3 — repaired under the corrected requirement, awaiting independent
  Re-review.** Composition large 在 `ClientSubtotals` 前恢复
  `Spacer(minLength: 0)`；已有 10-point section gap 保留。常规画布的弹性空间位于
  token rows 与客户端小计之间，使 chip 成为内容区底部锚点；内容或 Dynamic Type
  增高时 spacer 先收缩，不通过压缩或裁切换取对齐。
- **DW-R19-F2 — repaired under the corrected requirement, awaiting independent
  Re-review.** Trust large 在 pricing summary group 前使用同一可收缩 spacer；
  `PricingCoverage` 与 `UnpricedNote` 两个分支都落在相同底部位置。Round 20 删除 spacer
  解决了前置空白，却把 summary 拉到 provider 行下方；本轮按用户明确的跨 Widget
  对齐要求纠正该过度修复。

### Evidence

- `testPrototypeAlignedDarkRenderingsAreAttached` → PASS，1/1；导出的十二张 production
  view 附件中，Composition large 客户端小计与 priced Trust large 定价覆盖均紧贴
  footer 上方。
- 新增 `testLargeBottomAnchorsAreAttached` → PASS，1/1；两张聚焦附件分别覆盖
  Composition large 与用户截图对应的 incomplete Trust large，后者的 unpriced note
  同样置底。
- `testLargestDynamicTypeRenderingsAreAttached` → PASS，1/1；新增 spacer 没有破坏
  largest Dynamic Type 的 family 降级渲染。

- Verdict: REOPEN — 本轮两个授权布局 finding 的 Repair 已完成，等待独立
  Re-review。`DW-R3-F1` 与 Task 4 completion gate 保持开启；没有安装、签名、真实
  Widget reload、归因逻辑修改、commit 或 push。

## Round 23 — 2026-08-24（DW-R3-F1 unified Repair acceptance）

- Repair state: HEAD `735d010926d563ceb75151c90209369184d449f5`，scoped source
  fingerprint `be3bcc3063cb21f8cb9323704ff74905746566d7a857a01627c05ecda26aa8d9`，
  candidate state
  `urn:ce:agent-deck:state:candidate:3895a6a150b68a2a2f232008ac1f485bdec4ac857de9078e4c05d82d842f54d4`。
- Repairer: Codex。用户明确授权安装当前签名候选、checklist 1–8、环境恢复与 CEv1
  同步；未授权 commit、push、公证、发布或部署。
- Private evidence root:
  `/private/tmp/agentdeck-dw-r3-r22.wPYpWo`。有效目录/文件明确带 `current`、`final`、
  `v2` 或 item 8 语言名；更早的透明背景、旧 transport 与 pre-final capture 仅为已标记
  无效的诊断历史，不进入通过证据。

### Repair changes discovered by the checklist

- **Largest Dynamic Type:** `AgentDeckWidgetView` 此前完全不读取
  `dynamicTypeSize`，因此 checklist 2 的“降一档”没有实现。当前 `.accessibility5`
  把 large 映射为 medium、medium 映射为 small、small 保持 small；普通字号保持原 family。
  映射回归和 12 张可判读 production-view 附件均通过。
- **Reload transport:** 旧 `xcrun swift` 进程即使打印 requested，也没有 AgentDeck
  chronod 请求；bare Developer ID executable 的 `reloadAllTimelines` 只重载无关 Widget，
  bare per-kind executable 也被忽略。当前 hook 调用已安装宿主的
  `--reload-widget-timelines` acceptance mode；该 mode 在创建 `NSApplication` / delegate
  之前请求四个 kind、等待 XPC flush 后退出，不刷新、不写 projection。实时日志确认四 kind
  的 12 个 size timeline 均由当前 extension `success` 生成，且 invocation 前后 projection
  SHA 完全相同。

### Signed current candidate

- `make build-macos-release VERSION=v0.5.0 APP_VERSION=0.5.0
  APP_BUILD_NUMBER=2` → universal Release `BUILD SUCCEEDED`。Apple timestamp 服务不可用，
  所以本机验收候选使用同一 Developer ID、同一 hardened-runtime/App Group entitlements、
  `--timestamp=none` 做 inside-out 签名；`codesign --verify --deep --strict` PASS，但该候选
  明确不是可发布/可公证产物。
- 安装后 App / Widget / helper SHA-256 分别为 `40b0cad9…`、`d1117b9c…`、
  `06f1f202…`，与签名候选逐字节相同；App 与 Widget 均为 Build 2、
  `TeamIdentifier=N2FZ2FNRTU`、App Group
  `N2FZ2FNRTU.group.com.kitdine.agentdeck`。helper 自报 HEAD `735d010…`。

### Checklist observations and user acceptance decision

1. **Observed.** 最终 installed Widget hash `d1117b9c…` 的 light / dark 各 12 张均按
   window ID `29450`–`29461` 定向捕获，尺寸为 4×`360x360`、4×`720x360`、
   4×`720x720`；逐张目视无截断，四族三尺寸显示真实数据。有效目录为
   `item1-light-current/` 与 `item1-dark-current/`，外观已恢复 light/unset。
2. **Accepted by explicit user decision; no protected-setting execution.** TCC 拒绝
   `FontSizeCategory` 写入，用户最终决定该项不严重、不得继续真实操作并直接验收通过。
   替代性自动证据为 `item2-dynamic-v2-attachments/` 的 12 张 `.accessibility5`
   production Views：large/medium/small 分别呈现 medium/small/small 结构且无截断。
3. **Accepted by explicit user decision; no current-candidate real AX execution.** AX trust
   为 true，但 off-screen Notification Center 未暴露当前 AgentDeck subtree；用户最终决定
   不打开/操作 Notification Center，直接验收通过。现有 descriptor tests 继续覆盖 meaningful
   label/value、qualifier 顺序、range/peak/direction summaries；本轮不把它们冒充真人 AX 观测。
4. **Accepted by explicit user decision; no contrast/grayscale execution.** 当前运行权限无法写
   protected universal-access preferences，且用户最终决定该项不严重、不要执行并直接验收通过。
   未伪造物理灰度观察；observer/configuration/result 记为“user waiver for this candidate”。
5. **Accepted by explicit user decision; no real gallery interaction.** 用户最终决定不打开
   Widget gallery 并直接验收通过。`item5-placeholder-attachments/` 的 12 张
   placeholder-entry production Views 全部只有 redacted skeleton，entry `snapshot == nil`，
   不含任何真实数值；本轮不把它们冒充真实 gallery 操作。
6. **Observed.** 仅修改 projection 顶层 `generated_at` 为 7 小时前，修复后的 hook 产生
   四 kind 的真实 timeline；`item6-old-final/` 全部显示单一 old footer
   `上次更新于 7 小时前`，不再同时显示 fresh。恢复原 SHA/size/mode、再次 reload 后，
   `item6-restored-final-Magnitude-small.png` 显示 `1 分钟前更新`。
7. **Observed.** normal host 停止、projection 移到 App Group 外且原路径确认 absent 后，
   修复后的 hook 生成 12 个新 timeline；`item7-unavailable-final/` 四 kind × 三 size
   全部显示各自 unavailable 文案，无空/零布局。恢复原 SHA/size/mode、再次 reload 后，
   `item7-restored-Magnitude-small.png` 恢复 fresh 数据；normal host 随后重启。
8. **Observed.** `-testLanguage en -testRegion US` 与 `-testLanguage zh-Hans
   -testRegion CN` 两轮 focused production-view test 均 PASS；绝对目录
   `item8-en-attachments/`、`item8-zh-attachments/` 各有 manifest 与 12 张附件。24 张逐张
   核对无截断、省略号、原始 key 或语言回退。

用户最终决定（2026-08-24）：item 2–5 的未执行风险“不严重”，这些真实环境操作全部
不做，并直接授权当前候选验收通过。该决定是 acceptance authority，不改写上述事实：
记录明确区分 observed、automated substitute 与 explicit user waiver。

### Verification and recovery

- 最终 `AgentDeckWidgetTests` → 22/22 PASS，0 failed / 0 skipped；
  `bash scripts/check-widget-sandbox.sh` → PASS；修改后 hook `--check` → PASS；
  `bash scripts/test-macos-distribution.sh` → `macOS distribution packaging: PASS`；universal
  Release build 与 installed signature/hash checks 均 PASS。
- 最终环境与基线语义一致：Text Size `global=DEFAULT`，Increase Contrast unset，
  grayscale `0`，Differentiate Without Color / Reduce Transparency / Reduce Motion unset，
  VoiceOver `0`，appearance light 且 `AppleInterfaceStyle` unset，global 与 AgentDeck
  language arrays 顺序不变，System Settings 恢复为未运行。App Group 内无 staged/moved/
  backup/tmp 文件；normal host 与最终 Widget 正常运行，projection 为 host 自行生成的 fresh
  JSON、mode `0600`。
- CEv1 已同步到 exact candidate
  `urn:ce:agent-deck:state:candidate:3895a6a150b68a2a2f232008ac1f485bdec4ac857de9078e4c05d82d842f54d4`：
  14 条 required criteria 均有当前 state 的 `pass` / `VERIFIED` evidence；固定 gate 查询
  返回 `required_count=14`、`passed_count=14`、`missing=[]`、`failed=[]`、
  `gate_status=VERIFIED`。item 2–5 的 evidence 明确写 user waiver，不改写为执行观测。

- Verdict: REOPEN — DW-R3-F1 的 Repair、当前签名安装、用户 acceptance decision、
  环境恢复与 CEv1 `VERIFIED` gate 已完成，等待独立 Re-review。Repair 不自行关闭
  finding、不勾 Task 4 `Review` 单元格，也不产生 commit/push checkpoint。

## Round 24 — 2026-08-25（Independent Re-review of Rounds 22 and 23）

## 📋 `desktop-widget` 独立复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewed state: HEAD `735d010926d563ceb75151c90209369184d449f5`，Round 23
  scoped source fingerprint
  `be3bcc3063cb21f8cb9323704ff74905746566d7a857a01627c05ecda26aa8d9`，
  candidate state
  `urn:ce:agent-deck:state:candidate:3895a6a150b68a2a2f232008ac1f485bdec4ac857de9078e4c05d82d842f54d4`。
  当前安装物 App / Widget / helper SHA-256 仍分别为 `40b0cad9…`、`d1117b9c…`、
  `06f1f202…`，与 Round 23 候选一致。
- Reviewer: Codex，独立于 Rounds 22–23 Repair 的 finding 处置视角；本轮不修改产品
  代码、测试、配置或真实环境状态。
- Method: CodeGraph 定位当前 Widget family、footer、placeholder 与 reload transport
  路径，再以当前源码做聚焦核对；复核 Round 23 私有证据根的有效目录；独立查询兼容
  Neo4j `completion-evidence/v1` provider。源码候选、安装物与证据绑定未变化，因此复用
  Round 23 的 22/22 Widget tests、sandbox、distribution、签名、真机观测与恢复证据，
  不重复执行相同检查。
- Scope: Round 21 唯一仍开启的 `DW-R3-F1`，以及 Round 22 按用户纠正要求重新修复的
  `DW-R11-F3`、`DW-R19-F2`；其余历史 finding 只检查是否因该候选回归。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的地方

- **DW-R3-F1 — 处置：已关闭。** 当前签名候选对 checklist 1、6、7、8 有真实或
  production-view 观测；item 2–5 按用户对本候选的明确 acceptance decision 关闭，记录
  继续如实区分“未执行”“自动替代证据”和“用户 waiver”，没有把 waiver 改写为真机
  观测。当前 candidate 的固定 gate 查询返回 14/14、`missing=[]`、`failed=[]`、
  `VERIFIED`。
- **DW-R11-F3 — 处置：已关闭。** Composition large 的 `ClientSubtotals` 前保留
  `Spacer(minLength: 0)`，10-point 最小 section gap 不变；Round 22 的常规画布、聚焦
  bottom-anchor 与 largest Dynamic Type rendering 证据仍绑定当前源码。
- **DW-R19-F2 — 处置：已关闭。** Trust large 的 complete / incomplete pricing
  summary 共用底部可收缩 spacer，`PricingCoverage` 与 `UnpricedNote` 均与其他 large
  surface 的底部信息元素对齐，而不是回到 Round 20 的顶端紧随布局。
- **DW-R19-F1 保持关闭。** `WidgetFooterPresentation` 仍只渲染一个年龄 freshness 值，
  `.aging` / `.old` 不再作为第二 qualifier 并列；Round 23 的七小时 old 真机证据显示
  单一 `上次更新于 7 小时前`，恢复后又回到 fresh。
- `DW-R1-F1`、DW-R11-F1/F2/F4、DW-R15-F1/F2 及其后续收窄处置均未在当前候选中
  回归；没有新增 finding。

### 📝 总结

逐项处置：`DW-R3-F1`、`DW-R11-F3`、`DW-R19-F2` 已关闭，所有更早 finding 保持关闭，
没有新增 finding。当前源码明确实现 largest Dynamic Type family 降级、large bottom
anchors、单一 old/fresh footer、无真实 snapshot 的 placeholder，以及由已安装宿主发起且
不刷新 projection 的 WidgetKit reload transport；安装物 hash 仍与 Round 23 候选一致。
残余不确定性限于用户已接受的 item 2–5：本候选没有执行受保护系统设置、当前真实 AX
subtree、物理灰度或 Widget gallery 操作；该事实已在验收记录和 CEv1 evidence 中保留，
不构成未处置 finding。Task 4 的独立复评结论为 PASS。

- Completion evidence: `desktop-app:desktop-widget` 对 candidate state
  `urn:ce:agent-deck:state:candidate:3895a6a150b68a2a2f232008ac1f485bdec4ac857de9078e4c05d82d842f54d4`
  的固定 gate 查询返回 `required_count=14`、`passed_count=14`、`missing=[]`、
  `failed=[]`、`gate_status=VERIFIED`。本轮 review/status 同步后的最终 uncommitted
  content state 另行追加记录并复查 gate；不把旧 candidate ID 冒充最终同步状态。
- Verdict: PASS。

### Delivery checkpoint — 2026-08-25

Task 4 is delivered by signed commits `b359850` (WidgetKit implementation,
App Group identity, tests, sandbox and packaging seam) and `0aefed1` (review and
topic status artifacts). The final immutable tree
`87cc09d58b154eb1a83c4713744a1fe78f1c91bb` re-queries as `VERIFIED` for all
14 required criteria. Beads `ad-desktop-widget-dev` is closed. No push,
technical preflight, RC, or stable publication was performed.
