---
status: active
topic: desktop-refresh
subject: refresh-integration-acceptance
---

# Refresh integration acceptance review

## Round 1 — 2026-09-20

## 📋 Refresh integration acceptance 独立评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**RIA-R1-F1（P1）— [`docs/specs/cli-design.md:2008`](../../../specs/cli-design.md#L2008)、[`docs/specs/cli-manual.md:881`](../../../specs/cli-manual.md#L881)：稳定规范把明确 BLOCKED、无 waiver 的安装态 Widget 验收写成已交付事实。**

- 行为风险：设计规范保证“macOS approves both host and Widget container access, and
  all fifteen configurations render data”，中文手册进一步保证“十五种配置均渲染真实
  数据”。用户和后续 release/integration 会据此把五种 kind × 三种尺寸的安装态读取、
  intent 与 WidgetKit render 当成已验证，而 Task 5 自己明确记录真实 WidgetKit callback
  与 installed intents 为 BLOCKED、无 waiver。
- 证据：`tasks.md` 第 293–295 行要求稳定规范保留 native limitations；第 303–311 行把
  granted callbacks 与 installed intents 列为 L3 native acceptance；第 328 行明确这些
  检查 BLOCKED 且 source/XCTest request dates 不能冒充实际 WidgetKit delivery。两份
  stable spec 的肯定句与该权威验收表不能同时成立。新 integration checker 只检查
  `fifteen configurations` / `十五种配置` 字样存在，并不能证明实际安装、回调或真实
  数据渲染。

💡 有界修复：在未获得安装/GUI authority、实际完成 native matrix 或用户明确 waiver
之前，把两份稳定规范收窄为已实现的五 kind/十五 configuration、共享 App Group contract
与自动化 render 覆盖，并逐字保留真实 WidgetKit callback、installed intent 与 OS timing
尚未验收的限制；更新 checker，使其要求 limitation 语义而不是未受支持的完成宣称。

**RIA-R1-F2（P1）— [`docs/topics/desktop-refresh/tasks.md:316`](../tasks.md#L316)：Task handoff 与 CEv1 证据没有绑定当前五路径内容状态。**

- 行为风险：当前候选若继续交付，会把另一组字节的 5/5 evidence 当成当前稳定规范、
  checker 与 acceptance table 的验收结果；任何发生在两状态间的合同变化都绕过了
  impact assessment 和 target-bound gate。
- 证据：CEv1 ContentState `ff07fe147d55…` 声明的 recipe 是五路径有序
  `path<TAB>git-blob` SHA-256，且 implementation handoff 也引用该 digest；对当前
  `Makefile`、integration checker、两份 stable spec 和 `tasks.md` 使用同一 recipe
  复算得到 `a29299f49e6c6060e3c84c51526a88e6b7c0631d62528b46947173ce3b70fe28`。
  因此现有五条 pass evidence 对当前内容状态均不可适用。

💡 有界修复：完成 `RIA-R1-F1` 后，以最终五路径字节重新生成 ContentState；对旧
`ff07…` evidence 做 scope-aware change/impact assessment，仅显式 roll-up 仍有效的检查，
对规范/limitation criterion 记录修复后证据，并查询精确最终 target gate。不要改写或
重标旧 observation。

### 🟡 建议改进 — 推荐

无。本轮只记录会阻止 Task 5 稳定合同与证据边界成立的缺陷。

### 🟢 优点

- acceptance table 对真实 wall-clock、WidgetKit、sleep/wake、VoiceOver、Increase
  Contrast 与 gallery 缺口使用 BLOCKED/no-waiver 口径，没有把 deterministic tests
  直接写成 native PASS。
- checker 对 legacy `reloadAllTimelines`、Widget unbounded `Data(contentsOf:)`、
  scheduler/timeline 常量与 shared bounded reader 提供了轻量结构门禁。
- 同一开发候选记录了 App 121（1 skip）、Widget 45/45、Go desktop normal/race、
  sandbox、topic docs、whitespace 与 diff checks 的成功结果；本轮 finding 不否定这些
  自动化检查本身。

### 📝 总结

- Finding disposition：`RIA-R1-F1 OPEN`；`RIA-R1-F2 OPEN`；无 CLOSED、
  SUPERSEDED 或 carried finding。
- Reviewed state：HEAD `7c8d1a2b20657fdd5e4465fb7ea05805a1e7cb0a`；五路径有序
  `path<TAB>git-blob` manifest SHA-256
  `a29299f49e6c6060e3c84c51526a88e6b7c0631d62528b46947173ce3b70fe28`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:refresh-integration-acceptance:review-r1:a29299f49e6c6060e3c84c51526a88e6b7c0631d62528b46947173ce3b70fe28`。
- Reviewer：Codex 主代理；Method：独立 integration/contract review，CodeGraph impact
  exploration + scoped diff/checker/source inspection，Blue-only，subagent rounds
  consumed：0；项目规则禁止未请求委派。
- Scope：Task 5 的 `Makefile`、integration checker、两份 stable spec 与 topic
  acceptance/status 更新，以及它们对 reviewed requirements/architecture/native
  limitation 的符合性。
- Evidence：复用 handoff 报告的自动化 PASS 作为其原 observed state 的历史事实；
  精确合同对照证明 stable spec 与 BLOCKED native rows 矛盾，当前五路径 manifest
  复算证明原 5/5 evidence 不能绑定当前候选。决定性 findings 成立后未重复宽泛测试。
- Completion gate：FAILED。当前 target 对 `stable-contract-reconciliation` 有适用
  fail evidence；其余 required criteria 没有已确认的当前态 target-bound evidence。
- Residual uncertainty：未执行任务已列为 BLOCKED 的安装/GUI/native 操作；没有
  user waiver，且 Review authority 不包含安装、系统睡眠或交互式无障碍操作。

### 下一步指令

修复：desktop-refresh / reviews/refresh-integration-acceptance.md / RIA-R1-F1 RIA-R1-F2

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — 2026-09-20

- `RIA-R1-F1 -> repaired in candidate.` English and Chinese stable specifications
  now guarantee only the implemented five-kind/fifteen-configuration contract and
  automated build/sandbox/localization/render coverage. They explicitly state that
  installed WidgetKit registration/container access, real callback timing,
  representative intents and real-data rendering remain native acceptance gaps
  until performed with install/GUI authority or covered by an explicit user waiver.
- The integration checker now requires those limitation phrases and fails if either
  prior overclaim (`macOS approves both...` / `十五种配置均渲染真实数据`) returns.
- `RIA-R1-F2 -> repaired in candidate.` The final five-path identity was recomputed
  after F1 as SHA-256
  `e9e241ed955ad1c492e81ee8f1fb0a5eb8847c3a9b0e716a251da69b6089996f`.
  CEv1 records a change from the reviewed `a29299f4…` state, explicit preserve/
  invalidate impact decisions, and new target-bound evidence; no old observation is
  overwritten or relabeled.
- Verification：`make check-desktop-refresh-integration`、
  `make check-widget-sandbox`、topic-docs、whitespace 与 diff checks PASS。Product
  source is unchanged, so the final App 121（1 skip）/Widget 45/45 and Go desktop
  normal/race results are reused only through the recorded impact assessment.

Round 1 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`RIA-R1-F1`、`RIA-R1-F2` 是否 CLOSED 与最终 target gate 由独立复评决定。

## Round 2 — 2026-09-20

## 📋 Refresh integration acceptance 独立复评

📊 总体评分：8/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**RIA-R1-F2（P1）— [`docs/topics/desktop-refresh/tasks.md:316`](../tasks.md#L316)：repair evidence 再次没有绑定当前五路径内容状态。**

- 处置：`RIA-R1-F2 STILL OPEN`。repair 建立了新 ContentState 和 impact lineage，
  但其 target 仍不是本轮实际读取的候选字节。
- 行为风险：若以当前 handoff 的 `VERIFIED` 继续交付，Review、stable specs、checker 或
  acceptance table 在 `e9e241ed…` 之后发生的任何变化都不会被 gate 观察；Task 5 的
  最终交付仍缺少精确内容身份。
- 证据：repair handoff、review repair candidate 与 CEv1 都引用五路径 digest
  `e9e241ed955ad1c492e81ee8f1fb0a5eb8847c3a9b0e716a251da69b6089996f`；本轮对当前
  `Makefile`、integration checker、两份 stable spec 和 `tasks.md` 按相同有序
  `path<TAB>git-blob` recipe 复算为
  `61cceb8f1327e122d9ff959b76e064ef6b818fd5b34801a0d3999c3255565790`。
  当前 digest 在 CEv1 中没有 ContentState，旧/new repair evidence 都不能自动适用。

💡 有界修复：冻结最终五路径后先复算并回读 manifest，再创建唯一对应的 ContentState；
只把 scope-compatible evidence target-bound roll-up 到该 state，查询精确 gate 后再写
handoff。任何随后会改变五路径 blob 的修复/status 写入必须发生在 fingerprint 之前。

### 🟡 建议改进 — 推荐

无。本轮只复核 Round 1 两项 finding。

### 🟢 优点

- `RIA-R1-F1 CLOSED`：English/Chinese stable specs 只保证五 kind/十五 configuration
  的实现与 automated build/sandbox/localization/render coverage，并逐项保留 installed
  WidgetKit registration/container、real callback timing、representative intent 与
  real-data rendering 的 native gaps，关闭条件明确为实际执行或用户 waiver。
- checker 要求两种语言存在 native-acceptance limitation，并显式拒绝
  `macOS approves both...` 与 `十五种配置均渲染真实数据` 旧过度声明。
- Product source 未变化；repair 记录的 integration、sandbox、topic-docs、whitespace
  与 diff checks 可作为其 observed state 的历史证据继续保留。

### 📝 总结

- Finding disposition：`RIA-R1-F1 CLOSED`；`RIA-R1-F2 STILL OPEN`；无 REGRESSED、
  SUPERSEDED 或 NEW finding。
- Reviewed state：HEAD `7c8d1a2b20657fdd5e4465fb7ea05805a1e7cb0a`；当前五路径有序
  `path<TAB>git-blob` manifest SHA-256
  `61cceb8f1327e122d9ff959b76e064ef6b818fd5b34801a0d3999c3255565790`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:refresh-integration-acceptance:rereview-r2:61cceb8f1327e122d9ff959b76e064ef6b818fd5b34801a0d3999c3255565790`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，CodeGraph
  current-state probe + scoped stable-spec/checker/source inspection，Blue-only，subagent
  rounds consumed：0；项目规则禁止未请求委派。
- Evidence：源码与 checker 精确核对关闭 `RIA-R1-F1`；当前 manifest 与 repair
  ContentState 对照证明 `RIA-R1-F2` 尚未关闭。未重复 product tests，因为当前阻塞是
  evidence target identity，而非这些测试在其原 observed state 的结果。
- Completion gate：NOT_VERIFIED。当前 target 已创建并回读，但五项 required criteria
  均无当前态 applicable evidence；这不否定 `e9e241ed…` 的历史 gate，只禁止把它用于
  当前态。
- Residual uncertainty：native acceptance rows 继续 BLOCKED/no-waiver；修复后的 stable
  specs 已诚实保留这些限制，因此它们不是本轮仍 open finding。

### 下一步指令

修复：desktop-refresh / reviews/refresh-integration-acceptance.md / RIA-R1-F2

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — Round 2 — 2026-09-20

- `RIA-R1-F2 -> repaired in candidate.` The final five-path manifest was frozen
  and re-read as SHA-256
  `61cceb8f1327e122d9ff959b76e064ef6b818fd5b34801a0d3999c3255565790`
  before this handoff. The existing Re-review Round 2 ContentState is the exact
  target; this review record is outside that manifest, so this handoff does not
  mutate the target bytes.
- CEv1 records the scoped change from `e9e241ed…` to `61cceb8f…`: automated L3
  and privacy/sandbox evidence are explicitly preserved with target-bound
  roll-ups and dependencies; integrated-path, stable-contract and native-
  accounting evidence were observed directly for the current target. No prior
  observation was overwritten or relabeled.
- Exact target gate: `VERIFIED`, with all five required criteria bound through
  `observed_at` to
  `urn:ce:agent-deck:content-state:desktop-refresh:refresh-integration-acceptance:rereview-r2:61cceb8f1327e122d9ff959b76e064ef6b818fd5b34801a0d3999c3255565790`.
- Verification: `make check-desktop-refresh-integration`,
  `make check-widget-sandbox`, topic-docs, whitespace and `git diff --check`
  PASS. Product/test sources did not change; native acceptance rows remain
  BLOCKED with no waiver.

Round 2 remains FAIL and the Task Review checkbox remains unchecked until an
independent re-review closes `RIA-R1-F2`.

## Round 3 — 2026-09-20

## 📋 Refresh integration acceptance 独立复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `RIA-R1-F2 CLOSED`：本轮复算的五路径 manifest 与 Repair Round 2 handoff、
  ContentState `rereview-r2:61cceb8f…` 完全一致；review record 不属于该 manifest，
  handoff 后没有再改变 target bytes。
- exact target gate 对五项 required criteria 均返回 applicable pass evidence；automated
  L3 与 privacy/sandbox 通过显式 preserves/roll-up 复用，integrated path、stable contract
  与 native-accounting 直接观察当前状态。旧 `ff07…` / `e9e241…` evidence 的 invalidation
  保持可见，不影响当前 gate。
- `RIA-R1-F1 CLOSED（保持）`：两份 stable spec 继续区分 automated coverage 与
  installed WidgetKit/native gaps；checker 继续要求 limitation 文案并拒绝旧过度声明。
- native acceptance table 继续把真实 wall-clock、WidgetKit callbacks/intents、sleep/wake、
  VoiceOver、Increase Contrast 与 gallery 标为 BLOCKED/no-waiver，没有转写为技术 PASS。

### 📝 总结

- Finding disposition：`RIA-R1-F1 CLOSED`；`RIA-R1-F2 CLOSED`；无 still-open、
  regressed、superseded 或 new finding。
- Reviewed state：HEAD `7c8d1a2b20657fdd5e4465fb7ea05805a1e7cb0a`；Review 状态同步后的
  五路径有序 `path<TAB>git-blob` manifest SHA-256
  `4947adc6e0de01e36dc8fb478848b29037c01153aaf7029fccab603f53a66db8`；ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:refresh-integration-acceptance:rereview-r3:4947adc6e0de01e36dc8fb478848b29037c01153aaf7029fccab603f53a66db8`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，CodeGraph
  current-state probe + scoped stable-spec/checker/CEv1 inspection，Blue-only，subagent
  rounds consumed：0；项目规则禁止未请求委派。
- Evidence：repair target `61cceb8f…` 的 exact gate VERIFIED 5/5。Round 3 只同步 Task
  Review 状态；scope-aware target-bound roll-up 把五项有效 evidence 绑定到最终
  ContentState，未重复 product tests。
- Completion gate：VERIFIED（5/5 required criteria；无 missing 或 unresolved；旧状态
  invalidation 保留且不适用于最终 target）。
- Residual uncertainty：Task 5 表内 native rows 继续 BLOCKED/no-waiver；稳定规范已诚实
  保留限制，本 PASS 只关闭 Task 5 的 integration/contract/evidence-accounting 边界，
  不把 blocked native rows 宣称为技术 PASS。

### Task checkpoint

Task checkpoint：`ad-dr-refresh-integration-acceptance-dev`；ContentState
`4947adc6e0de01e36dc8fb478848b29037c01153aaf7029fccab603f53a66db8`；gate
`VERIFIED`。

提交建议：提交 Task 5 的 `Makefile`、integration checker、两份 stable spec、
`tasks.md` 状态与 `reviews/refresh-integration-acceptance.md` Round 1–3 完整历史；
建议范围不授权提交。

推送建议：提交完成并验证签名、message、trailers、commit-bound CEv1 与 Task 5 实际
交付边界后，推送 `feature/desktop-refresh`；当前没有推送授权。

### Containing-unit prerequisite

`desktop-refresh` 当前没有可查询的 Topic/Plan WorkUnit、required criteria 或 CONTAINS
hierarchy，且 Task 5 尚未形成 commit-bound delivery state；因此本轮不伪造 Unit
completion checkpoint。Task 5 获得单独提交授权并完成 commit-bound gate 后，才跨越
topic completion boundary并建立/查询其权威 gate。

### 下一步指令

提交

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh
