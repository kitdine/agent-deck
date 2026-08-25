---
status: active
topic: desktop-app
subject: ux/settings.md
---

# Review log — desktop-app / ux/settings.md

## Round 1 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/ux/settings.md` blob
  `bec92a75a091fa43a3a43455d9ee7c06dc056bc9`; referenced
  `ux/prototype/interactive-v7/` manifest fingerprint
  `0243c0cd257e6fe1044827139d52f087c45d6ff7dcfccdf6ff3abbacdc1f2059`.
- Reviewer: Codex
- Method: Single-agent formal design/contract Review, supplemented by a current
  Product Design accessibility audit. The in-app Browser was unavailable, so
  the declared fallback used a headed Chrome session to capture and inspect the
  rendered Simplified Chinese Light-appearance settings window. The live DOM
  accessibility relationships were checked against the document, and CodeGraph
  supplied the current `SettingsWindow`, `Field`, and `Switch` source and call
  path. Broad appearance, language, and keyboard verification stopped after the
  two decisive accessibility-contract reproducers below.
- Scope: `docs/topics/desktop-app/ux/settings.md` and its referenced
  `ux/prototype/interactive-v7/src/Settings.jsx` specimen, checked against the
  approved four-preference requirements boundary and `menubar-experience` task.
- Findings:
  - [P1] S1-F1 — `ux/settings.md:173-174` requires each switch's explanatory
    line to be its accessible description, but `Settings.jsx:8-20,43-55` gives
    each switch only an `aria-label`; the hint has no ID and the switch has no
    `aria-describedby`. The current browser returned `describedBy: null` for
    both Launch at login and Periodic refresh. -> Give every field hint a stable
    ID and bind the corresponding switch with `aria-describedby`, then verify
    the accessibility tree exposes the localized consequence as its
    description in both languages.
  - [P1] S1-F2 — `ux/settings.md:177-178` requires the login-item failure to be
    announced when it appears, but `Settings.jsx:43-53` renders the conditional
    failure as an ordinary `<small>` with no `role="status"`, `role="alert"`, or
    live-region attribute. -> Give the failure line an appropriate localized
    status/live-region semantic while retaining its visible icon and text, then
    exercise the failure transition and verify one announcement without
    disabling the remaining controls.
- Evidence: `git rev-parse HEAD` ->
  `58fe5d300c5af572adef81a69a856a6aef9cea56`; `git hash-object
  docs/topics/desktop-app/ux/settings.md` ->
  `bec92a75a091fa43a3a43455d9ee7c06dc056bc9`; current prototype manifest ->
  `0243c0cd257e6fe1044827139d52f087c45d6ff7dcfccdf6ff3abbacdc1f2059`;
  headed current screenshot confirmed a complete four-preference window with no
  visible load failure; browser DOM inspection -> both switches have
  `aria-describedby: null`; browser error log -> empty; CodeGraph current source
  confirmed the unassociated hints and non-live conditional error; `bash
  scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check`
  -> exit 0.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：7/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`ux/settings.md:173`] S1-F1：引用 specimen 没有把 switch 的解释文字暴露为
accessible description。
- 行为风险：VoiceOver 用户只能听到偏好名称与开关状态，听不到“开启后会发生什么”，
  无法在操作前理解登录项和定时刷新的后果。
- 证据：当前浏览器中两个 switch 的 `aria-describedby` 均为 `null`；源码中的 hint
  没有 ID，`Switch` 只设置了 `aria-label`。
💡 有界修复：为 hint 建立稳定 ID，通过 `aria-describedby` 绑定相应 switch，并在中英
两种语言的 accessibility tree 中确认 localized description。

[`ux/settings.md:177`] S1-F2：引用 specimen 的登录项失败行不会在出现时自动宣布。
- 行为风险：系统拒绝登录项变更后，键盘或 VoiceOver 用户可能继续认为操作成功，除非
  主动重新浏览窗口内容。
- 证据：条件错误仅为普通 `<small>`；没有 `role="status"`、`role="alert"` 或
  `aria-live`。
💡 有界修复：为错误行增加合适的 status/live-region 语义，保留图标和文字，并验证
一次失败只宣布一次且其余控件保持可用。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 当前 specimen 清楚区分通用与菜单栏两组，四项偏好与批准后的 requirements 一一
  对应，没有重新引入 update check。
- 默认状态与文档一致：登录项和定时刷新关闭，菜单栏显示成本且统计全部客户端。
- Light 中文窗口在当前截图中没有裁切或加载错误；dialog 名称、radio group 名称和
  selected state 均进入了 accessibility tree。

### 📝 摘要

评审对象为 HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`、
`ux/settings.md` blob `bec92a75a091fa43a3a43455d9ee7c06dc056bc9` 与 prototype
manifest `0243c0cd257e6fe1044827139d52f087c45d6ff7dcfccdf6ff3abbacdc1f2059`。
四偏好边界、视觉层级、默认值和基础语义整体完整，但 S1-F1 与 S1-F2 使引用 specimen
无法兑现文档明确写下的 switch description 和动态失败宣布合同，因此本轮
FAIL/REOPEN。由于决定性 finding 已成立，Dark、English、完整键盘路径与其他手工清单
没有在本轮继续执行；它们不是 PASS 证据，也不是新增 finding。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-settings.md / S1-F1 S1-F2
```

## Round 2 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 1's two open findings. `ux/settings.md` is
  unchanged and still blob `bec92a75a091fa43a3a43455d9ee7c06dc056bc9` — both
  findings were against the referenced specimen, not the contract text. HEAD is
  still `58fe5d300c5af572adef81a69a856a6aef9cea56`. Edited specimen files, by
  `sha256`:
  - `src/Settings.jsx` ->
    `8dad7038e4142b7c6807cc234e8a169bb4b8bdd12fcf4d1f625ba46b05444e64`
  - `src/i18n.js` ->
    `1bc5f0645cb9a09858ff8c5a6d06d52dbad8eed1d7d7ad6dd1205d11495a65c1`
  - `src/probe.js` ->
    `5f1ec5735de3e845edfafa5172240ee320a6038d6600b5240b053f0c57e4ebce`
  - `src/styles.css` ->
    `3ed22659df721915f9c2645dec416ae9a809901d9f1388c8223327c22fa0f75e`
  - `README.md` ->
    `679f17ae9d4b4fbfb476115d5b9f94e01e997c699f13377b1f901481c4216f1c`
- Reviewer: claude-code (repair round for Round 1's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: S1-F1 and S1-F2 as named in the repair command. No product code, test,
  configuration, or contract text was changed.

- Round 1 findings, dispositions:
  - **S1-F1** switch hints are not the accessible description -> **Fixed.**
    Added a language-independent ID helper (`settings-<key>-hint`), put that ID
    on each field's hint, and bound the control with `aria-describedby`. The
    finding named the two switches; the two mode selectors carry the same kind
    of consequence line and the instruction was to give *every* field hint a
    stable ID, so both radio groups are bound as well rather than left holding
    an ID nothing points at. `ux/settings.md:174-175` requires the group to
    carry the preference name and each option its selected state, which is
    unchanged; a description is additive there, and this is recorded here
    explicitly so the Re-review can judge that call rather than discover it.
  - **S1-F2** the login-item failure is not announced when it appears ->
    **Fixed**, and the fix is placement, not just an attribute. A `role="status"`
    put on the conditionally-rendered `<small>` would enter the accessibility
    tree at the same moment as its text, which is the case screen readers
    routinely miss. Instead each field now renders a persistent
    `.settings-error` container with `role="status" aria-live="polite"`, empty
    until a failure exists, so the region is already in the accessibility tree
    when the text arrives. Exactly one such region covers the failure and it is
    not nested inside another, so one failure produces one announcement. The
    warning icon stays and is `aria-hidden`, so the announcement is the localized
    sentence rather than the sentence plus an icon node — `ux/settings.md:177`
    wants text *and* icon visually, and the text is what carries the meaning.
    `.settings-label` moved from `gap: 3px` to `margin-top: 3px` on its `small`
    children, because a container `gap` would give the empty region a 3px band;
    the spacing of every other field is unchanged by construction (3px before,
    3px after) and was confirmed visually.

- Made reachable in order to verify S1-F2 at all: the specimen had no path to
  the failure state — `loginItemError` was initialized to `null` and never set,
  and the copy table's refusal string was not in `i18n.js`. S1-F2's own
  verification clause requires exercising the failure transition, so the switch
  now follows the specimen's established simulated-failure pattern (the refresh
  control already fails the first attempt and succeeds the second): the first
  attempt to enable is refused, a further attempt succeeds. That walks all four
  clauses of `ux/settings.md:85-92` — the switch stays at the real status,
  the line appears, nothing else is disabled and no modal appears, and the line
  clears on the next successful change rather than on a timer. The stored state
  is a boolean (`loginItemRefused`) with the string resolved at render, so the
  failure line follows a language switch instead of freezing in the language it
  appeared in; `loginItemError` was renamed accordingly, and it had no other
  reader. Added `loginItemRefused` to both catalogs with the copy table's exact
  strings (`ux/settings.md:160`): `Could not change the login item` /
  `无法修改登录项`.

- Verification performed by this repair round, not yet independently confirmed:
  - **Real accessibility tree, not DOM attributes.** Round 1's evidence was
    `aria-describedby: null`, so this round checked the level above: CDP
    `Accessibility.getFullAXTree` over the live settings window. All four
    controls now expose the localized consequence as `description`, in `zh` and
    `en` and in both appearances — e.g. `switch "Launch at login"` ->
    `"Registered as a system login item; no background daemon is installed"`,
    `radiogroup "统计范围"` -> `"跟随面板筛选时，面板里选了 Codex，菜单栏也只显示
    Codex"`.
  - **Live region present before the failure.** The same tree shows four
    `status` nodes with `live=polite atomic=true` and empty content while no
    failure exists; after the refused attempt exactly one of them carries
    `"无法修改登录项"` / `"Could not change the login item"` and the other three
    stay empty. A fifth `status` in the tree is the popover's pre-existing trend
    readout, not part of this window.
  - **Failure transition, all four contract clauses.** After the refused
    attempt: `aria-checked` stays `false`; the line renders with its icon; no
    `button` in the window is `disabled`; `[role=dialog]` count stays 1, so no
    modal appeared. After the next attempt: the switch reads `true` and the line
    is empty. Three seconds later it is still empty, confirming it cleared on the
    change and not on a timer.
  - **Regression found and closed inside this repair.** The specimen's own
    interaction probe asserted `开关可切换` against the login-item switch, which
    the now-contract-correct refusal makes false. The plain toggle assertion
    moved to Periodic refresh, which has no refusal path, and ten assertions were
    added for the two findings: the four `aria-describedby` targets resolve to
    non-empty text, the live region exists and is empty before the failure, the
    switch stays off, the line appears with an icon, exactly one live region is
    non-empty, nothing is disabled and no modal appears, the retry succeeds, and
    the line clears. `probe=1` is `ALL PASS` over 49 assertions in all four
    appearance × language combinations.
  - **No contrast or layout regression.** Scoped axe WCAG A/AA over
    `.settings-window` in both appearances × both languages × clean and refused
    states — 8 runs, 0 violations of any rule, so the M1-F1 token work still
    holds with the new markup. `surface=widgets&measure=1` returns `NO OVERFLOW`
    in all four combinations.
  - Not exercised by this round, and not claimed: a real VoiceOver pass. The
    evidence above establishes that the description and the live region exist in
    the accessibility tree with the right values at the right time, which is what
    the two findings name; whether macOS VoiceOver speaks them is a runtime check
    this specimen cannot perform.

- Evidence: CDP `Accessibility.getFullAXTree` output as summarized above for
  `zh`/`en` × dark/light; failure-transition assertions before, during, and after
  the refusal; `probe=1` 49/49 `ALL PASS` ×4; scoped axe over `.settings-window`
  0 violations ×8; `measure=1` `NO OVERFLOW` ×4; `make check-whitespace`,
  `bash scripts/check-topic-docs.sh`, and `git diff --check` all exit 0.
  `ux/settings.md` is byte-identical to the blob Round 1 judged. Five specimen
  files changed, fingerprints listed above; `README.md` records the two semantics
  and the fact that only the accessibility tree — not DOM attributes — proves
  them, so the next change to `Settings.jsx` cannot regress this silently.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-settings.md / Round 2
```

## Round 3 — 2026-08-19（独立复评）

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/ux/settings.md` blob
  `bec92a75a091fa43a3a43455d9ee7c06dc056bc9`; referenced
  `ux/prototype/interactive-v7/` manifest fingerprint
  `e04e83899c3df575df3932a752f61f3f9e7063d125694b78e8afd412875d7434`;
  repaired `src/Settings.jsx` sha256
  `8dad7038e4142b7c6807cc234e8a169bb4b8bdd12fcf4d1f625ba46b05444e64`.
- Reviewer: Codex
- Method: Single-agent formal independent Re-review of Round 2, supplemented by
  a current Product Design accessibility audit. The declared Chrome fallback
  captured and inspected the current Light Simplified Chinese clean state and
  Dark English refused state. Browser DOM relationships and CDP
  `Accessibility.getFullAXTree` output were checked before, during, and after
  the refused-login transition; CodeGraph independently supplied the current
  `SettingsWindow`, `Field`, `Switch`, and `Segmented` source and call paths.
- Scope: Round 1 findings S1-F1 and S1-F2 and regressions in their bounded
  accessibility semantics. The broader appearance/contrast and prototype probe
  evidence from Round 2 was reused only because the current manifest and all
  listed repair-file hashes match that round.
- Findings:
  - [P1] S1-F1 — **CLOSED.** Every field hint now has a stable,
    language-independent ID, and all two switches plus both radio groups point
    to their hint through `aria-describedby`. The live accessibility tree
    exposes the exact localized consequence as `description` for all four
    controls in both `zh` and `en`.
  - [P1] S1-F2 — **CLOSED.** Each field now owns one persistent
    `role="status" aria-live="polite"` container. In both languages the clean
    state contains four empty live regions; the first login-item attempt leaves
    the switch off, renders one localized warning with one icon, produces
    exactly one non-empty status, leaves every button enabled, and keeps one
    dialog. The next attempt turns the switch on and clears the status.
  - Newly blocking findings: none.
- Evidence: current accepted Light Chinese clean and Dark English refused
  captures; DOM -> four non-empty `aria-describedby` targets resolving to the
  localized hints; AX tree -> localized descriptions plus four persistent
  `live=polite`, `atomic=true` status nodes; refusal/retry probes in `zh` and
  `en` -> off/error/one status/no disabled/one dialog, then on/empty/zero
  non-empty status; browser error log -> empty; `bash
  scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check`
  -> exit 0. Round 2's exact-state evidence remains reusable: `probe=1` 49/49
  `ALL PASS` in all four appearance × language combinations, scoped axe over
  the clean/refused settings states -> 0 violations across 8 runs, and
  `measure=1` -> `NO OVERFLOW` ×4. A real
  VoiceOver speech pass remains an implementation/runtime acceptance item and
  is not claimed by this specimen review. CEv1 content state
  `45359869ce8a6ab2a0801327afb23b59a2cf2acda0a6faeaf638d274d5f6cd86`
  -> one required criterion, current passing evidence, no invalidation or
  unresolved impact, `VERIFIED`.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。S1-F1 与 S1-F2 均已关闭，没有新的阻断 finding。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- hint ID 与控件绑定覆盖了两个 switch 和两个 radio group，避免留下无人引用的
  description 节点。
- live region 在错误发生前已经存在；错误出现与成功清除都通过当前 AX tree 和状态转换
  独立确认，而不是只检查属性文本。
- 中英文错误和四项 description 都来自当前 catalog；Dark refused 与 Light clean
  截图均保持完整层级和可读性。

### 📝 摘要

S1-F1、S1-F2 是 Round 1 的全部 findings，本轮已在 HEAD
`58fe5d300c5af572adef81a69a856a6aef9cea56`、`ux/settings.md` blob
`bec92a75a091fa43a3a43455d9ee7c06dc056bc9` 和 prototype manifest
`e04e83899c3df575df3932a752f61f3f9e7063d125694b78e8afd412875d7434`
上独立确认关闭。没有新的阻断项，因此结论为 PASS。剩余不确定性仅是真实 macOS
VoiceOver 是否按预期发声；prototype 已证明 AX description/live-region 的值、时序与
状态正确，真实语音仍由 `menubar-experience` 的运行时手工验收承担。

#### Task checkpoint

Task checkpoint：`desktop-app:ux/settings.md`（`menubar-experience` 的设置窗口
文档交付）；上述 exact content state 的 CEv1 门槛为 `VERIFIED`。

提交建议：以设置窗口文档交付为原子边界，候选范围包括 `ux/settings.md`、
`Settings.jsx` 及 S1-F1/S1-F2 所需的 `i18n.js`、`probe.js`、`styles.css`、prototype
`README.md`，再加本评审记录与 `tasks.md`/`docs/README.md` 的本任务状态 hunk；这些
prototype 文件与其他 surface 共用，只有在能证明未纳入 `ux/widget.md` 未评审内容时
才应暂存，否则把提交 checkpoint 延后到共享文件可安全隔离时。

推送建议：本轮不执行推送；若后续单独授权提交与推送，先确认 `main` 到
`origin/main` 的精确范围、暂存边界、commit 正文/共同创作者尾注/SSH 签名和 Hook
效果，再只推送该文档 Task 的 commit。

#### 📌 下一步

```text
评审：desktop-app / reviews/ux-widget.md
```

## Document review deferred to the closing pass (2026-08-20)

By user instruction, `desktop-app` runs no document review rounds while its tasks
are being implemented. Review is **deferred, not cancelled**: after every
implementation task is done, the whole document set is reconciled against the
final prototype and the shipped implementation and reviewed once, as a bullet on
task 6.

Until then, changes to this subject are written directly into the document that
owns it, and nothing is appended here. The closing pass appends its round to this
record. The reason and the two consequences are stated in
[`../tasks.md`](../tasks.md).

**Status 2026-08-23 — reconciled and submitted; the closing round has not run.**
Task 6 has brought the set into agreement with the shipped implementation, so this
record is now waiting for the single deferred review rather than waiting for
implementation to finish. The next thing appended below is that round.

## Closing document review — Round 1 (2026-08-23)

This is the single deferred document review this topic postponed on 2026-08-20.
One round, one verdict, over the whole set; this record carries the round's
common part plus this document's own outcome.

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint
  `5d5c576eeb75117bd6b329d807bb018c5dec7cea12b213a1c603256d3f5345e3`, computed as
  `shasum -a 256` over one `<git hash-object> <space> <line count> <space> <path>`
  line per document, in this order: `requirements.md`, `architecture.md`,
  `ux/menubar.md`, `ux/settings.md`, `ux/widget.md`, `tasks.md`. The order is part
  of the recipe.
- Reviewer: Claude Code, independent of every implementing and reconciling
  session.
- Method: the set judged against the shipped implementation rather than against
  itself, which is the reason the review was deferred to this point. Checkable
  claims were taken to the code: the settings defaults and the login-item refusal
  path to `DesktopPreferences.swift` and `SettingsWindowView.swift`; the four
  panels and the Breakdown cards' non-collapsibility to `MenuBarPanelViews.swift`;
  the `Not captured yet` modules to `DesktopCopy.swift`; the fixed 30-day rhythm
  window across `requirements.md`, `ux/menubar.md` and `internal/usage`; the
  widget's runtime state to `desktop-widget` Round 3. `scripts/check-topic-docs.sh`
  is the project's checker for this target class and was run.
- Set-wide verdict: **REOPEN**. Three findings, all located in two documents;
  four of the six documents carry none.
- This document's outcome: **PASS**, no findings.
  - Checked against the build: all four defaults match
    `DesktopPreferences.swift` — periodic refresh off, login item off, menu-bar
    value `cost`, menu-bar scope `allClients`; the login-item control reads
    `SMAppService.mainApp.status` and a refusal leaves the switch at the real
    status rather than the requested one; `requiresApproval` is carried as its own
    state and worded as waiting for approval rather than as a failure, which is the
    one case this document specifies in full.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class; a verdict on set completeness must cite the tool that can
    falsify it)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Implementation cross-checks named under Method, each run against the working
    tree at this content state
- Residual, carried and owned elsewhere: task 4 `desktop-widget` remains at an
  open P1 (DW-R3-F1) parked on an Apple Developer team ID, so this set describes a
  surface whose runtime acceptance never passed; task 6's basis remains the user's
  explicit decision to proceed without task 4, recorded only by the implementing
  agent.
- Verdict: REOPEN

## Closing document review — Round 3 (independent re-review, 2026-08-23)

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint, computed with Round 1's six-document recipe
  **after this round's own status synchronization**, so the evidence binds to the
  state the verdict leaves behind rather than to the pre-tick one:
  `c42a6de1d707e6d79320032877996ad8aec10847cf58a45885eb3e63fa3bd2a7`.
  Round 2's repair candidate was `771b3375…cd10`; Round 1 was `5d5c576e…45e3`.
  Two of the six documents changed between Rounds 1 and 2 — `requirements.md` and
  `tasks.md` — which is exactly Round 2's declared scope; the four documents that
  passed Round 1 are byte-identical to their Round 1 blobs.
- Reviewer: Claude Code, independent of the repair round.
- Method: each finding re-verified against the documents themselves rather than
  against Round 2's account of the repair, and the four already-passing documents
  re-hashed to confirm the repair did not touch them.
- Disposition of every Round 1 finding:
  - CD1-F1 — **closed.** `requirements.md`'s Widget acceptance bullet now carries
    a `Known defect:` paragraph stating that the criterion is not met by the
    shipped build, naming the missing team-ID prefix, macOS 26's refusal, all
    twelve configurations rendering the unavailable state, and `DW-R3-F1` with its
    parked Apple Developer prerequisite — and it closes with "The criterion above
    remains the contract", which is the half that mattered. The disclosure now
    matches the shape the other four documents already used, so the set has one
    voice on this defect instead of two.
  - CD1-F2 — **closed.** The temporary code-over-contract rule now states why it
    survives task 6's initial reconciliation — task 4's open `DW-R3-F1` leaves the
    implementation and the document set in disagreement — and assigns the removal:
    task 6 owns a final reconciliation after task 4 closes that finding, before
    task 6 can reach Review PASS. A reader can now tell a deferral from an
    omission, which is what the finding asked for. Declaring the retention
    deliberate is the owner's decision to make, not a claim this reviewer can
    falsify, and the substantive requirement — the condition and the owner — is
    met.
  - CD1-F3 — **closed, and the repair is the stronger of the two available
    options.** The Documents matrix gained a dated `Closing review` column, and
    the deferral section was updated to say which column means what: the old
    `Review` cells "retain the suspended historical values from before deferral"
    and "do not report the closing pass", while the dated column "is the
    closing-pass result". The manual-acceptance row is marked `—` as outside the
    six-document set, which is correct — Round 1 reviewed six documents and that
    was not one of them. The closing pass now has falsifiable cells, and this
    round moved the two that were `[ ]`.
- New findings: none. The four documents that passed Round 1 are unchanged, and
  the two repaired documents introduced no defect this round could find.
- Status synchronization performed by this round, recorded here because it
  changed the fingerprint the evidence binds to: `requirements.md` and `tasks.md`
  moved to `[x]` in the `Closing review` column, and `tasks.md`'s next-action
  statement was corrected — it still directed the reader to run this review and to
  re-review task 6's record, both of which had already run.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Per-document blob comparison against Round 1 → only `requirements.md` and
    `tasks.md` changed
- What this verdict does and does not unblock: it closes task 6's R1-F1, whose
  whole content was that this review had not run. It does not let task 6 pass.
  The rule repaired under CD1-F2 now makes that explicit — task 4 must close
  `DW-R3-F1` before task 6 runs the final reconciliation that removes the rule —
  and `DW-R3-F1` is parked on an Apple Developer team ID that does not exist yet.
  That is where the topic stops, and it is an external prerequisite rather than
  work anyone here can perform.
- Set-wide verdict: **PASS**

### Round 3 addendum — completion-evidence bookkeeping (2026-08-23)

Recorded after the round, because the recording itself needed a correction and
hiding it would defeat the purpose of an audit trail.

- The six `document` boundaries this round crossed are now recorded and the gate
  answers `VERIFIED` for each, bound to content state
  `urn:ce:agent-deck:state:candidate:a5e3433a…f167` (HEAD `a190186` plus the
  post-synchronization set fingerprint `c42a6de1…d2a7`).
- **A mistake, disclosed rather than quietly fixed.** The first write used
  `MERGE` on work-unit ids that already existed under the store's *retired*
  capitalized `WorkUnit` vocabulary for four of the six documents —
  `architecture.md`, `ux/menubar.md`, `ux/widget.md` and `tasks.md`. Two things
  followed. The gate could not see them, because it filters on the current
  lowercase `work_unit` kind, so those four read `NOT_VERIFIED` while looking
  recorded. And an unconditional `SET` overwrote each of those historical nodes'
  `target_content_state` with this round's value; the prior values were not
  captured first and are **not** restored here, because guessing them from their
  latest evidence would put an invented fact into the record. `head` and
  `attributes_json` were added to those nodes rather than overwritten, the old
  vocabulary carrying neither.
- The forward fix creates four new current-model `work_unit` nodes under distinct
  ids ending `-doc`, pointing at the criteria this round already wrote, and
  leaves the historical nodes as history. `.agent-instructions/evidence.md` says
  to leave retired-vocabulary nodes as they are; it also says to inspect the store
  before writing, which is the step that was skipped.
- One pre-existing row still reads `NOT_VERIFIED`: `ux/settings.md`'s older
  `independent-review-pass` criterion, whose evidence binds to a content state
  from before the deferral. It is a leftover of the earlier process, not of this
  round, and is left for whoever reconciles that boundary.
