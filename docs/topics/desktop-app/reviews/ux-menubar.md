---
status: active
topic: desktop-app
subject: ux/menubar.md
---

# Review log — desktop-app / ux/menubar.md

## Round 1 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/ux/menubar.md` blob
  `f61bce676626066e60e5f80744baded3b288668f`; referenced
  `ux/prototype/interactive-v7/` fingerprint
  `0f41b84456d2c03a429be9d5e88c6c1b451d95c882074a6ee7555e4dbed3698f`
- Reviewer: Codex
- Method: Single-agent formal design/contract Review, supplemented by a current
  Product Design audit in a headed `agent-browser` session at the document's
  declared 420 × 760 pt surface. The audit exercised all four panels, health
  detail, provider menu and confirmation, empty/aged/partial/unavailable states,
  both languages, both appearances, keyboard focus cycling, console errors, and
  scoped axe-core 4.12.1 WCAG A/AA checks. Candidate findings were independently
  checked against the repository contract before entering this record. A mock
  attribution-arithmetic candidate was rejected because prototype values are
  illustrative and is not a finding.
- Scope: `docs/topics/desktop-app/ux/menubar.md` and its referenced
  `ux/prototype/interactive-v7/` specimen, checked against the approved
  requirements boundary and current task decomposition. Broad review stopped
  after the decisive M1-F1 reproducer.
- Findings:
  - [P1] M1-F1 — `ux/menubar.md:712-716,783-785` requires semantic colors and
    states that both Light and Dark appearances keep every status legible, but
    the referenced prototype fails the scoped axe `color-contrast` rule in both
    shipped appearances: 22 nodes in Light and 18 in Dark. Affected nodes include
    freshness text, selected and unselected client tabs, hero annotations,
    notice copy, statistic labels, and list metadata. -> Adjust only the
    prototype's semantic color tokens for those roles until the scoped WCAG A/AA
    audit reports no contrast violations in either appearance, then perform the
    document's manual Increase Contrast and Light/Dark legibility checks. Record
    their observed results in the next review round.
- Evidence: `git rev-parse HEAD` ->
  `58fe5d300c5af572adef81a69a856a6aef9cea56`; `git hash-object
  docs/topics/desktop-app/ux/menubar.md` ->
  `f61bce676626066e60e5f80744baded3b288668f`; normalized prototype fingerprint
  -> `0f41b84456d2c03a429be9d5e88c6c1b451d95c882074a6ee7555e4dbed3698f`;
  headed browser captures confirmed the visible contrast loss; scoped axe-core
  4.12.1 -> Light: 1 serious violation over 22 nodes, Dark: 1 serious violation
  over 18 nodes, with 17 rules passing in each appearance; provider confirmation
  focus cycled Cancel -> Confirm -> Cancel; browser errors were empty and the
  console contained only Vite/React development messages. The calendar's
  incomplete `aria-prohibited-attr` result requires manual verification and is
  not recorded as a finding. `bash scripts/check-topic-docs.sh`,
  `make check-whitespace`, and `git diff --check` -> exit 0.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`ux/menubar.md:712`] M1-F1：引用原型的 Light 与 Dark 外观均未达到最低文字
对比度。
- 行为风险：freshness、client tabs、hero 注释、notice、stat 标签和列表元数据中的
  次要信息对低视力用户不可可靠辨认；该 specimen 不能作为文档声明的可访问实现基线。
- 证据：同一 headed 运行中，scoped axe-core 4.12.1 在 Light 报告 22 个节点、Dark
  报告 18 个节点违反 serious `color-contrast`；截图也确认 Light 的次要文字明显偏淡。
💡 有界修复：只调整原型中这些语义角色的主题颜色，直到两套外观的 scoped WCAG
A/AA 检查不再报告对比度失败，再人工执行 Increase Contrast 与 Light/Dark 可读性
清单并记录结果。

### 🟡 建议改进——推荐

无。`.calendar` 的 ARIA 检查目前只是 incomplete，证据不足，不记 finding。

### 🟢 优点

- 四个面板保持一致层级，header、filters、scrolling content 与 footer 稳定。
- health detail 有明确返回路径；provider menu 区分当前、可用与禁用选项，并在切换前
  要求确认。
- 确认层的键盘焦点在取消与确认之间循环。
- empty、aged、partial、unavailable 状态可区分，`en` 与 `zh-Hans` 没有缺失 key。
- 当前浏览器运行没有应用异常或失败请求。

### 📝 摘要

评审对象为 HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`、`ux/menubar.md`
blob `f61bce676626066e60e5f80744baded3b288668f` 与 prototype 指纹
`0f41b84456d2c03a429be9d5e88c6c1b451d95c882074a6ee7555e4dbed3698f`。交互、
状态和布局结构整体成熟，但引用 specimen 同时违反文档自己的双外观可读性合同，故
本轮 FAIL/REOPEN。强制修复范围仅为 M1-F1；mock 数值准确性不属于本轮 finding。

## Round 2 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 1's single open finding. `ux/menubar.md` is
  unchanged and still blob `f61bce676626066e60e5f80744baded3b288668f` — the
  finding was against the referenced specimen, not the contract text. The
  repaired specimen is `ux/prototype/interactive-v7/`, whose only edited file is
  `src/styles.css`, now `sha256
  f946659bc2fbf899906642cd7a7987b3043fb8e06dae7fd7618027165f4cd374`. HEAD is
  still `58fe5d300c5af572adef81a69a856a6aef9cea56`.
- Reviewer: claude-code (repair round for Round 1's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: M1-F1 as named in the repair command. No product code, test,
  configuration, or contract text was changed, and no other Round 1 observation
  was reopened.

- Round 1 findings, dispositions:
  - **M1-F1** scoped axe `color-contrast` fails in both appearances ->
    **Fixed.** Reproduced first: the scoped audit on the default panel returned
    1 serious `color-contrast` violation over 10 nodes in each appearance, and
    the node list named exactly the roles Round 1 named — `.freshness`,
    `.clients > .active[role="tab"]`, `.active[role="tab"] > b`,
    `button[role="tab"]:nth-child(2|3) > b`, `.hero > div:nth-child(1|2) > span`,
    `.hero-note`, `.axis > span`, `.incomplete`, `.notice > span`. Three
    distinct causes, all in the token layer:
    1. **`--dim` never met AA in either appearance.** Dark `#6b7684` measured
       4.19:1 on `--bg` and 3.54:1 on `--surface-2`; light `#808b99` measured
       3.28:1 on `--bg` and 3.46:1 on `--surface`. This is `.freshness`, both
       hero annotations, `.hero-note`, `.axis`, and every stat-grid label.
       Retuned to `#7f8995` (dark) and `#676f7b` (light), each ≥ 4.6:1 against
       every surface those roles actually sit on, and each still a step below
       `--muted` so the three-tier text hierarchy survives.
    2. **The light appearance reused the dark appearance's semantic hues.**
       `--accent`, `--info`, `--good`, `--warn`, and `--bad` were defined once
       in `:root` and tuned for a `#0b0e13` ground, so on a white ground they
       measured 3.00–3.49:1, and `--warn`/`--bad` fell to 2.08–2.87:1 once they
       sat on their own `color-mix` tints — the `.notice` copy and `.incomplete`
       hero amount. The light block now carries its own set: `--accent #b24a0b`,
       `--info #2f67c4`, `--good #237843`, `--warn #74500e`, `--bad #a53937`,
       each solved against the worst background it can occupy, tints included.
       The `:root` set is unchanged and remains the dark appearance's values.
    3. **`opacity` was being used to de-emphasize already-marginal text.**
       `.segmented button b` at `opacity: 0.75` blended `--muted` into the
       groove and `.segmented.clients button.active b` at `opacity: 0.9`
       blended `#fff` into the fill. Both now carry an explicit color instead
       (`var(--dim)` and `#fff`), which keeps the count a step weaker than its
       label without reintroducing the failure.
    Separately, the selected client segment put `#fff` on `--accent`, which is
    3.16:1 — a failure in *both* appearances, not only light. Added
    `--accent-strong` (`#c4520c` dark, `#b24a0b` light) as the fill that carries
    white text, used by that segment and by `.dialog button.primary`, which had
    the same pairing. `--accent` itself keeps its role as the graphic fill for
    bars and glyphs, where no text sits.

- Verification performed by this repair round, not yet independently confirmed:
  - Scoped axe-core WCAG A/AA sweep over 80 combinations — 2 appearances × 2
    languages (`zh`, `en`) × 5 states (`normal`, `empty`, `aged`, `partial`,
    `unavailable`) × 4 panels (`usage`, `breakdown`, `attribution`, `sessions`):
    **0 failing `color-contrast` nodes**, against 10 per appearance before the
    change.
  - Same audit over the sub-surfaces the panel sweep does not reach — health
    detail, provider menu, signal detail, the settings window, the widgets
    surface, and the states overview — in both appearances: 0 failing nodes in
    all 14 runs. The `.dialog` confirmation layer renders outside `.popover`, so
    the scoped selector does not cover it; its `button.primary` is covered by
    arithmetic (`#fff` on `#c4520c` = 4.61:1, on `#b24a0b` = 5.05:1) and by the
    interaction probe below, not by a scoped axe run. Stated rather than
    implied.
  - Manual acceptance items 4 and 5 (`ux/menubar.md:781-782`), observed results:
    **item 5 (Light/Dark switching keeps every status legible) — pass.** Headed
    captures at the declared 420 × 760 pt surface in both appearances show the
    freshness stamp, the `≈` incompleteness note, the event/session/project
    metadata line, the peak annotation, the axis labels, the stat-grid labels,
    and the amber health notice all plainly readable, and the selected client
    segment still reads as selected against the neutral period row below it.
    **Item 4 (Increase Contrast keeps every status legible) — legible, but the
    specimen does not respond to the setting.** `src/styles.css` contains no
    `prefers-contrast` or `forced-colors` query, so under Increase Contrast the
    specimen renders identically to its normal appearance. Since that normal
    appearance now meets AA in both appearances, "keeps every status legible"
    holds; "increases contrast" is not demonstrated by this specimen and remains
    a runtime obligation of the AppKit implementation.
  - No regression in the specimen's own self-checks: `probe=1` returned
    `ALL PASS` over 39 assertions in all four appearance × language
    combinations, and `surface=widgets&measure=1` returned `NO OVERFLOW` in all
    four.

- Evidence: reproduced-then-fixed axe counts as above (before: 1 serious
  violation over 10 nodes per appearance on the default panel; after: 0 over the
  full 80-combination sweep and the 14 sub-surface runs); `probe=1` 39/39
  `ALL PASS` ×4; `measure=1` `NO OVERFLOW` ×4; `make check-whitespace`,
  `bash scripts/check-topic-docs.sh`, and `git diff --check` all exit 0.
  `ux/menubar.md` itself is byte-identical to the blob Round 1 judged. Two files
  changed, both inside the specimen: `src/styles.css` (the fix) and `README.md`
  (the specimen's own record of it). Round 1's prototype fingerprint
  `0f41b844…` could not be reproduced because no recipe for it is recorded
  anywhere in the repository; this round therefore identifies the specimen by
  per-file sha256, checkable with `shasum -a 256`:
  `src/styles.css` ->
  `f946659bc2fbf899906642cd7a7987b3043fb8e06dae7fd7618027165f4cd374`,
  `README.md` ->
  `bea41a5ac441b71caaa6777cda30c59859682e7513b71cfa0baef1d1d6338953`.
- Specimen documentation, so the next token change cannot silently reintroduce
  M1-F1: the specimen's `README.md` now records the two-appearance token split
  and the `--accent-strong` role in its 视觉 section, and its 自检工具 section
  carries the scoped axe command, the 80-combination matrix, the sub-surface
  list, and the explicit note that `.dialog` falls outside the `.popover` scope.
  Round 1 had to derive that matrix from scratch; Round 2 leaves it written down.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-menubar.md / Round 2
```

## Round 3 — 2026-08-19（独立复评）

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/ux/menubar.md` blob
  `f61bce676626066e60e5f80744baded3b288668f`; referenced
  `ux/prototype/interactive-v7/` manifest fingerprint
  `0243c0cd257e6fe1044827139d52f087c45d6ff7dcfccdf6ff3abbacdc1f2059`;
  repaired `src/styles.css` sha256
  `f946659bc2fbf899906642cd7a7987b3043fb8e06dae7fd7618027165f4cd374`.
- Reviewer: Codex
- Method: Single-agent formal independent Re-review of Round 2, supplemented by
  a current Product Design accessibility audit. The in-app Browser was
  unavailable, so the declared fallback used a headed Chrome session at the
  contract's 420 × 760 surface. Current Dark and Light captures were inspected,
  then semantic locators independently measured the foreground/background
  pairs for every role named by M1-F1 across all four panels and the unavailable
  state. Round 2's broader axe evidence was reused only after the stylesheet
  hash and current manifest were confirmed unchanged.
- Scope: Round 1's single finding M1-F1 and regressions in the semantic roles it
  named. No product code, tests, configuration, or unrelated document question
  was re-reviewed.
- Findings:
  - [P1] M1-F1 — **CLOSED.** The repaired `--dim`, appearance-specific semantic
    colors, explicit client-count colors, and `--accent-strong` fill are present
    in the current stylesheet. The independent rendered-role check covered 292
    role instances across Light and Dark usage, breakdown, attribution, and
    sessions panels plus both unavailable states; it found no ratio below
    4.5:1. The minimum was 4.61:1 in Dark and 4.64:1 in Light. Current captures
    keep freshness, client segments, hero annotations, notice copy, chart axes,
    statistic labels, and list metadata visibly legible.
  - Newly blocking findings: none.
- Evidence: current headed Dark and Light captures inspected at 420 × 760;
  semantic-locator foreground/background measurement -> 292 affected role
  instances, 0 below 4.5:1, minima Dark 4.61:1 and Light 4.64:1; browser error
  log -> empty. Round 2's exact-stylesheet scoped axe evidence remains reusable:
  0 `color-contrast` nodes over the 80 appearance × language × state × panel
  combinations and 14 sub-surface runs. The stylesheet still has no
  `prefers-contrast` or `forced-colors` branch, so Increase Contrast renders the
  same already-AA colors; adaptive AppKit behavior remains a runtime acceptance
  obligation rather than an open document finding. CEv1 content state
  `27efa4a82460f5933432ad5ee4066e784fc1cdc19993a8ae5ae55188e2e8d938`
  -> one required criterion, current passing evidence, no invalidation or
  unresolved impact, `VERIFIED`.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。M1-F1 已关闭，没有新的阻断 finding。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- Round 2 把修复限定在语义 token 层，保留了三层文字层级和两种外观的
  视觉区分。
- 客户端选中态的白字与未选中计数都不再依赖透明度，实测最小对比度
  仍留有可复现的 AA 余量。
- 修复记录保留了 80 组 panel 扫描、14 组子界面扫描及 Increase Contrast
  的证据边界，本轮又用当前渲染状态独立复核了 finding 指向的角色。

### 📝 摘要

M1-F1 是 Round 1 的唯一 finding，本轮已在 HEAD
`58fe5d300c5af572adef81a69a856a6aef9cea56`、`ux/menubar.md` blob
`f61bce676626066e60e5f80744baded3b288668f` 和 prototype manifest
`0243c0cd257e6fe1044827139d52f087c45d6ff7dcfccdf6ff3abbacdc1f2059`
上独立确认关闭。当前渲染角色无低于 4.5:1 的测量值，没有新阻断项，
因此结论为 PASS。剩余不确定性只是 prototype 不模拟 macOS Increase Contrast
的自适应提升；它保持的基线颜色已达 AA，而真实 AppKit 响应仍由实现任务
的手工验收承担。

#### Task checkpoint

Task checkpoint：`desktop-app:ux/menubar.md`（`menubar-experience` 的文档
交付）；上述 exact content state 的 CEv1 门槛为 `VERIFIED`。

提交建议：以该文档交付为单一原子边界，候选范围包含已复评的
`ux/menubar.md`、其引用的 `ux/prototype/interactive-v7/`、本评审记录与
`tasks.md`/`docs/README.md` 的本任务状态 hunk；排除产品代码、其他未评审
文档及无关脏工作树。

推送建议：本轮不执行推送；若后续单独授权提交与推送，先确认
`main` 到 `origin/main` 的精确范围、暂存边界、commit 正文/共同创作者尾注/
SSH 签名及 Hook 效果，再只推送该文档 Task 的 commit。

#### 📌 下一步

```text
评审：desktop-app / reviews/ux-settings.md
```
