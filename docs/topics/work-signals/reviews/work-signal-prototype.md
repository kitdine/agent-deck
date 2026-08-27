---
status: active
topic: work-signals
subject: work-signal-prototype
---

# Review log — work-signals / work-signal-prototype

## Round 1 — 2026-08-27

- Reviewed state: HEAD `d168a1cbc78cfb34d2b7f5c6eeb9122ec47f36a6`, working tree
  clean. The task's content is the `prototype/` tree
  `2492f64ad79eb9f84c42b0373fdd5744c5bb331f`, delivered by `06a77b8` (the move
  and the work-signal changes) and `cb782d6` (the single-file export tooling
  added afterwards at the user's request), with `src/Popover.jsx`
  `c2768811` and `src/data.js` `c8c91621` carrying the findings below.
- Reviewer: claude-code, independently reviewing the implementation authored by
  `claude-code` in an earlier session; this workflow turn kept production code,
  tests, and configuration read-only.
- Method: Formal REVIEW under `development-workflow`, no external skill. The
  acceptance criteria require browser verification at two levels rather than a
  passing build, so the prototype was built, served, and driven in a real
  browser through `agent-browser`; every claim below was observed in the
  rendered surface or read from the source that produces it.
- Scope: the four Beads acceptance clauses — browser verification of the CLI
  page, the `--sub` subcategory expansion and the panel's expanded Activity row;
  `npm run build`; the pending-capture fixture retained and reachable; a pointer
  at the old path — plus the task record's own delivery claims in `tasks.md`
  (captured fixture shape, Tooling's dropped cost column, iteration depth
  becoming rework, the six CLI shapes).
- Findings:
  - **[P1] W1-F1 — the pending-capture fixture is retained but unreachable, so
    half of its acceptance clause is unmet and the task record asserts a
    rendering that cannot occur.** `PENDING_CAPTURE` is exported from
    `src/data.js:327`, imported by `src/Popover.jsx:20`, and selected by
    `signalsFor(state)` at `:481` when `state === "unavailable"`. Nothing can
    reach that branch. At `Popover.jsx:1113` the `unavailable` state replaces the
    entire scroll body with the `Notices` block and a
    `dict.status.unavailable` placeholder, so `SessionsPanel` (`:1128`) is never
    mounted in that state — and with it neither the `pending-flag` label at
    `:433`, nor `signalsFor`, nor `SignalDetail`'s `pending` at `:537`.
    Verified in the browser, not only in the source. On the popover surface the
    `Unavailable` state renders "Data could not be read / Open AgentDeck to
    refresh" with no Sessions content. On `?surface=states`, which mounts
    `<Popover embedded />` once per state for all five of `SURFACE_STATES`
    (`States.jsx:91-98`), `agent-browser get count ".pending-flag"` returns
    **0** across the whole board. There is no third route: `SessionsPanel` has
    exactly one call site.
    This matters more than a dead branch usually would, for two reasons. The
    acceptance clause is explicit that the fixture must be **retained and
    reachable**, and reachability is the half that fails. And `tasks.md` states
    as delivered that the pending fixture "is what the `unavailable` state
    renders" — a claim the code contradicts, in the document this project treats
    as the design truth for six downstream implementation tasks. Tasks 4 and 5
    have to build the not-yet-captured presentation; with no specimen of it they
    would invent one, which is the outcome moving the prototype to the
    repository root was meant to prevent.
    The cause looks like one state token doing two jobs: `unavailable` means
    "the snapshot could not be read" for the panel as a whole and "work signals
    were never captured" for this module, and the panel-wide meaning wins first.
    Separating them — a sixth surface state, or a per-module availability flag
    the Sessions panel reads independently of the panel-wide one — makes the
    retained fixture visible again. This review does not choose between them.
- Verified, not findings:
  - `npm run build` succeeds: 4581 modules, `dist/client` emitted, no warnings
    beyond an unrelated npm version notice.
  - The CLI page renders at `?surface=cli` and carries exactly six shapes —
    `stats`, `signals`, `kind`, `filter`, `session`, `empty` (`Cli.jsx:83-142`) —
    each as literal character output. Observed in the browser.
  - `--sub` expands correctly. `agentdeck usage signals --kind activity --sub`
    renders four categories and eleven subcategories, and every subcategory
    group sums to its parent in all three measures: shares 24/13/9/6 = 52,
    9/15 = 24, 8/4/3 = 15, 6/3 = 9, totalling 100; costs 2.74 / 1.27 / 0.79 /
    0.48; events 21 / 10 / 8 / 4. The same arithmetic holds in the fixture at
    `data.js:341-410`.
  - The panel's Activity row expands at both levels. The Sessions tab's
    `活动` row opens to the four categories, and `编码` opens to
    `新增 24% / 重构 13% / 测试 9% / 维护 6%` — the second level the task claims.
    Observed in the browser, with `[expanded=true]` on the category button.
  - Tooling carries no cost column, in the panel (`Bash 32 工具调用 39%`, …) and
    in the CLI `stats` shape alike, and the fixture's `tooling.rows` have only
    `calls` and `share`. Decision 4 is honoured.
  - Iteration depth became rework, in both catalogs: `retries: "返工"` /
    `"Rework"` with the note `改后验证再改` / `edit, verify, edit again`
    (`i18n.js:71-72,280-281`), rendered as such in the panel.
  - The old path carries a pointer: `docs/topics/desktop-app/ux/prototype/`
    holds only a `README.md` that names the new location and says why it moved.
    No second prototype copy exists anywhere under `docs/`, and
    `docs/README.md:35` lists the Product Prototype as an Active authority.
  - The out-of-scope single-file export from `cb782d6` is inert for this review:
    `preview.html` is git-ignored (`prototype/.gitignore:4`), `dist/` is
    untracked, and the addition is a `vite.config.mjs` branch plus
    `tools/build-single-file.mjs` with no new dependency. It changes how the
    specimen is viewed, not what it specifies.
- Evidence: `npm run build` in `prototype/`; `vite preview` on
  `http://localhost:4173`; `agent-browser` drove `?surface=cli`,
  `?surface=states` and the default popover, reading rendered text and
  accessibility snapshots at each step. `agent-browser get count ".pending-flag"`
  returned 0 on the states board. Source claims were checked against
  `Popover.jsx:20,433,481,537,641,1113,1128`, `States.jsx:91-98`,
  `data.js:327,341-410`, `Cli.jsx:83-142`, `i18n.js:71-72,280-281`,
  `main.jsx`, and `docs/topics/desktop-app/ux/prototype/README.md`.
- Completion gate: NOT_VERIFIED — no CEv1 WorkUnit exists for
  `work-signals:work-signal-prototype`; a query for that identifier returns no
  node of any kind. This is not `NOT_REQUIRED`: the project defines a task
  completion boundary for an implementation task, and the three
  `usage-attribution-precision` tasks each carry one. The gate is created and
  answered when the repair is re-reviewed; creating it now, over content this
  round reopens, would record a boundary for work that has not passed.
- Verdict: REOPEN

### Repair disposition — 2026-08-27

- W1-F1 closed: the panel-wide `unavailable` placeholder now applies only when
  the active tab is not `sessions`. The Sessions tab therefore mounts the
  existing `SessionsPanel` in that state, making `signalsFor("unavailable")`,
  the `PENDING_CAPTURE` fixture, the `pending-flag` heading label, and each
  detail view's pending banner reachable without adding a sixth state or
  changing the retained fixture. Usage, Breakdown, and Attribution retain the
  panel-wide unavailable placeholder.
- Evidence: `npm --prefix prototype run build` passes with 4581 modules. In an
  isolated real-browser session at `?surface=states&tab=sessions`, the five-state
  board contains exactly one `.pending-flag`; the unavailable specimen mounts
  three signal cards, and its Activity detail renders the pending banner plus
  the retained `58 / 21 / 12 / 9` fixture. The other four specimens contain no
  pending flag. At `?surface=states&tab=usage`, the unavailable specimen still
  contains one panel-wide unavailable placeholder and no signal cards.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 2 — 2026-08-27

- Reviewed state: HEAD `d168a1cbc78cfb34d2b7f5c6eeb9122ec47f36a6` plus the
  uncommitted `prototype/src/Popover.jsx` blob
  `d3ee7453e8b516c7b9fdf4ed976a82c406427c05`, the repair's only source change.
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`, no external skill. The
  acceptance standard for this task is browser verification, so the repaired
  prototype was rebuilt, served, and driven in a fresh `agent-browser` session;
  the finding below was read out of the rendered DOM, scoped to the exact panel
  that carries the pending flag, before it was traced back to source.
- Scope: W1-F1, and any consequence the repair introduces.
- Findings:
  - **W1-F1 — CLOSED.** `Popover.jsx:1113` now reads
    `unavailable && tab !== "sessions"`, so the Sessions tab mounts
    `SessionsPanel` in the `unavailable` state and the retained fixture is
    reachable. Verified in the browser rather than from the diff: on
    `?surface=states&tab=sessions` the five-specimen board contains exactly one
    `.pending-flag` and twelve `.signal-card`s — three cards on each of the four
    specimens that render the panel, `empty` still returning its own note. The
    `unavailable` specimen's Activity detail renders the pending banner
    「当前快照没有这些字段，需要新增采集」 above the four `PENDING_CAPTURE`
    categories at 58 / 21 / 12 / 9, with no subcategory expansion, which is
    exactly what an older snapshot should decode to. On
    `?surface=states&tab=usage` the same specimen still shows the panel-wide
    placeholder and no signal cards, so the other three tabs are unaffected.
  - **[P1] W2-F1 — closing W1-F1 this way makes the `unavailable` specimen
    assert, in one panel, both that the snapshot could not be read and that it
    was read.** `SessionsPanel` takes its content from
    `view = scope(client, period)` (`Popover.jsx:928`, `:412`), which is fixture
    data and is not filtered by `state`. Mounting it in the `unavailable` state
    therefore renders far more than the pending signal cards. Scoped to the panel
    that carries the pending flag, the rendered text is:
    `会话 4 · 均时长 59 分钟 · 项目 3`, then the three signal cards, then
    `按项目` with `headroom 2 会话 2时6分`, `ai-tools 1 会话 1时2分`,
    `codegraph 1 会话 48 分钟`, then `最近会话` with four named sessions
    carrying client, model and duration. Those are real numbers in the state
    whose panel-wide meaning is "the snapshot could not be read".
    The contradiction is not inferred, it is printed twice on the same board.
    The same specimen's Usage tab still says
    「数据读取失败 / 打开 AgentDeck 刷新」, while its Sessions tab lists four
    sessions by name. And the pending banner the repair relies on says
    「当前快照没有这些字段」 — a statement about a snapshot that *was* read and
    happens to lack the new fields. A snapshot cannot be simultaneously
    unreadable and readable-but-outdated; the repair now asserts both.
    The board states the project's own rule three sections below, in the widget
    degradation specimen: 「占位骨架绝不含真实数字」. The panel surface now
    breaks it.
    Round 1 named the cause — one state token doing two jobs — and offered two
    separations: a sixth surface state, or a per-module availability flag the
    Sessions panel reads independently of the panel-wide one. The repair took
    neither and widened the conflation instead: `unavailable` now means
    "unreadable" for three tabs and "readable, pre-subcategory" for the fourth,
    and `signalsFor` still keys the pending fixture to that same token. Either
    separation closes W1-F1 without this consequence, and a third option exists
    that the repair's own shape suggests — keep the tab bypass but have
    `SessionsPanel` render only the signal modules, with the session statistics,
    per-project rows and recent-session list suppressed the way the hero already
    suppresses cost and tokens to `—` at `:1040` and `:1061`. This review does
    not choose between them.
- Verified, not findings:
  - `npm run build` passes on the repaired source: 4581 modules, `dist/client`
    emitted.
  - The repair touched one file and two lines, and changed no fixture. The
    comment above `signalsFor` was updated to describe the new route rather than
    left stating the old one.
  - Round 1's other verified items were not re-run in full, having no dependency
    on this change; the CLI page, the `--sub` expansion and the panel's two-level
    Activity expansion are unaffected by a condition on the `unavailable`
    branch, and the build output is unchanged apart from the chunk hash.
  - The repair recorded its disposition in this record and commented it on the
    Beads task, but added no repair paragraph to `tasks.md`. Not a finding: the
    task remains `REOPEN`, so the matrix and the Round 1 narrative there are
    still the accurate status; the next passing round is what writes that
    paragraph.
- Evidence: `npm --prefix prototype run build`; `vite preview` on
  `http://localhost:4173`; a fresh worktree-scoped `agent-browser` session.
  `get count` on `?surface=states&tab=sessions` returned `.pending-flag` 1,
  `.signal-card` 12, `.unavailable` 1 (the widget-gallery specimen); on
  `?surface=states&tab=usage` it returned 0, 0 and 2. The W2-F1 text was read by
  scoping to `document.querySelector('.pending-flag').closest('.panel')`, so it
  is that specimen's panel and not a neighbour's. Source checked at
  `Popover.jsx:412,481,928,1040,1061,1113,1128`.
- Completion gate: NOT_VERIFIED — still no CEv1 WorkUnit for
  `work-signals:work-signal-prototype`. It is created and answered on the round
  that passes; recording a boundary over content this round reopens would assert
  a completion the evidence does not support.
- Verdict: REOPEN

### Repair disposition — 2026-08-27

- W2-F1 closed: `SessionsPanel` now suppresses its session-count statistics,
  per-project rows, and recent-session list when `state === "unavailable"`.
  The unavailable summary retains only the signal heading and its three cards;
  the existing pending fixture and detail views remain unchanged and reachable.
  Normal, aged, and partial summaries retain the complete session content, and
  the non-Sessions unavailable placeholder remains unchanged.
- Evidence: `npm --prefix prototype run build` passes with 4581 modules. In an
  isolated real-browser session at `?surface=states&tab=sessions`, the panel
  carrying the sole `.pending-flag` contains exactly three signal cards and no
  session-count, average-duration, project-count, per-project, recent-session,
  project-name, model, or session-duration text. Its Activity detail still
  renders the pending banner and the retained `58 / 21 / 12 / 9` fixture. A
  focused read of the normal specimen confirms its `4` sessions, `59`-minute
  average, `3` projects, three per-project rows, and four recent sessions remain.
  At `?surface=states&tab=usage`, the unavailable specimen still contains one
  panel-wide unavailable placeholder and no signal cards.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 3 — 2026-08-27

- Reviewed state: HEAD `d168a1cbc78cfb34d2b7f5c6eeb9122ec47f36a6` plus the
  uncommitted `prototype/src/Popover.jsx` blob
  `55a7b6c12b8ff1405b07227f0e0d94446640dd48`, again the repair's only source
  change.
- Reviewer: claude-code, independently re-reviewing the Round 2 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`, no external skill. Rebuilt,
  served, and driven in a fresh `agent-browser` session; this round read the whole
  `unavailable` specimen card rather than only the panel, because the previous
  round's finding is about what the specimen says as a whole.
- Scope: W2-F1, and any consequence the repair introduces.
- Findings:
  - **[P1] W3-F1 — the unavailable specimen still shows decoded work-signal
    values under a caption that says there is no snapshot. This is W2-F1's
    headline, unclosed; the remedy this review offered for it was wrong, and that
    is the reviewer's error, not the repairer's.**
    Round 2's repair did exactly what Round 2's third option prescribed:
    `SessionsPanel` now suppresses the session-count grid, the per-project card,
    and the recent-session card when `state === "unavailable"`. Verified — the
    panel carrying the sole `.pending-flag` now contains only
    `工作信号 / 待采集` and its three cards, with no session, project, model, or
    duration text anywhere in it, while the normal specimen keeps all of it.
    That option was mine and it does not resolve the finding it was attached to.
    W2-F1's headline was that the specimen asserts both that the snapshot could
    not be read and that it was read; the statistics were evidence for it, not
    the whole of it. Removing the evidence left the assertion standing, and
    reading the full specimen card instead of the panel makes it plain. Top to
    bottom, the `unavailable` card now reads:
    the board's own caption
    「没有快照，或缓存版本不受支持——一律不显示零，而是让用户去打开主程序」
    (`States.jsx:13`, in English at `:20`: "No snapshot, or an unsupported cache
    version — never render a zero, ask the user to open the app"); the hero with
    `全部 —`, `Codex —`, `Claude —` and 「打开 AgentDeck 以刷新」; the notice
    「数据读取失败」 (`i18n.js:116`); and then, three lines below that notice,
    `活动 编码 58%`, `工作流 首次编辑 2 分钟`, `工具 82 工具调用`.
    A specimen that says "no snapshot" cannot also display values decoded from
    one. Neither branch of the caption rescues it: under "no snapshot" there is
    nothing to decode, and under "unsupported cache version" the app has just
    declared it will not use that cache. The rule an implementer takes from this
    specimen is that an unreadable snapshot still renders work-signal numbers,
    and this prototype is the design truth for six downstream tasks.
    The two separations Round 1 named remain the ones that close it, and both are
    small. Add a sixth entry to `SURFACE_STATES` (`Popover.jsx:43`) for the
    pending-capture case and key `signalsFor` and the `pending-flag` on that
    instead of on `unavailable`; the board then renders it as its own specimen
    with its own caption, and `unavailable` goes back to being wholly
    unavailable. Or give the Sessions panel a module-level availability input
    that is independent of the panel-wide one. This review does not choose
    between them, and — having got the last option wrong — it does not propose a
    third.
- Verified, not findings:
  - `npm run build` passes: 4581 modules, `dist/client` emitted.
  - W2-F1's stated remedy is delivered exactly and completely. The three
    suppressions are conditioned on `state === "unavailable"` alone, so `normal`,
    `aged` and `partial` are untouched, and the browser confirms the normal
    specimen still carries its four sessions, 59-minute average, three projects,
    three per-project rows and four recent sessions.
  - W1-F1 stays closed: one `.pending-flag` and twelve `.signal-card`s across the
    board, the Activity detail still renders the pending banner over the retained
    58 / 21 / 12 / 9 fixture with no subcategory expansion, and the non-Sessions
    tabs keep the panel-wide placeholder.
  - The repair changed no fixture and no other component.
- Evidence: `npm --prefix prototype run build`; `vite preview` on
  `http://localhost:4173`; a fresh worktree-scoped `agent-browser` session on
  `?surface=states&tab=sessions`. The panel text was read by scoping to
  `document.querySelector('.pending-flag').closest('.panel')`, and the specimen
  text by walking up to the ancestor containing 「完全不可用」, so both are that
  specimen and not a neighbour's. Captions read from `States.jsx:13,20`; the
  notice string from `i18n.js:116`; `SURFACE_STATES` at `Popover.jsx:43`.
- Completion gate: NOT_VERIFIED — still no CEv1 WorkUnit for
  `work-signals:work-signal-prototype`, and this round does not create one.
- Verdict: REOPEN

### Repair disposition — 2026-08-27

- W3-F1 closed: the prototype now has a sixth, independent `pending` stage for
  a readable snapshot that predates the work-signal fields. `PENDING_CAPTURE`,
  the heading flag, and detail banners key only on `pending`; panel-wide
  `unavailable` again renders only its unreadable-snapshot placeholder. The
  Round 2 suppression workaround was removed because the separated state makes
  it unreachable and unnecessary.
- The state board and direct stage control both expose `pending` with distinct
  Chinese and English names and causes. The existing `normal`, `empty`, `aged`,
  `partial`, and `unavailable` meanings remain unchanged.
- Evidence: `npm --prefix prototype run build` passes with 4581 modules. In an
  isolated real-browser session, `?surface=states&tab=sessions` renders six
  specimens: exactly one pending flag on the separately captioned readable-old-
  snapshot specimen, three pending signal cards there, and no pending values or
  signal cards in the wholly unavailable specimen. The pending Activity detail
  retains its banner and `58 / 21 / 12 / 9` fixture. The direct English route
  `?state=pending&tab=sessions&lang=en` activates `Work signals pending`, renders
  `Not captured yet` and three signal cards, and carries no global read-failure
  message.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 4 — 2026-08-27

- Reviewed state: HEAD `d168a1cbc78cfb34d2b7f5c6eeb9122ec47f36a6` plus the
  uncommitted blobs `prototype/src/Popover.jsx`
  `caf4052929c46d9325e68393c4fc855c6028bf2f`, `prototype/src/Stage.jsx`
  `2eafd91d560c0f5c86fc48722045ff5b671f011f`, `prototype/src/States.jsx`
  `02fec9a2c083a828d501e391d997629456637da7`, and `prototype/src/i18n.js`
  `a300c3568df588c29407c6e0761e9f41269ab443`.
- Reviewer: claude-code, independently re-reviewing the Round 3 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`, no external skill.
  Rebuilt, served, and driven in a fresh `agent-browser` session; each specimen
  was read scoped to its own card, and the captured path was re-exercised to
  confirm the state split did not break it.
- Scope: W3-F1, and any consequence the repair introduces.
- Findings:
  - **W3-F1 — CLOSED.** The repair took the first of the two separations Round 1
    named. `SURFACE_STATES` (`Popover.jsx:43`) gains a sixth entry, `pending`,
    and `signalsFor`, the `pending-flag` heading label and `SignalDetail`'s
    `pending` all key on it instead of on `unavailable`. The tab bypass at
    `:1113` is back to a plain `unavailable`, and Round 2's three suppression
    guards inside `SessionsPanel` are gone, so `unavailable` is once again wholly
    unavailable and no workaround survives in the source.
    Verified in the browser, per specimen. The `unavailable` card now reads
    caption 「没有快照，或缓存版本不受支持」, hero `全部 — / Codex — / Claude —`,
    notice 「数据读取失败」, then the panel-wide 「打开 AgentDeck 以刷新」
    placeholder — with `0` signal cards and `0` pending flags inside it, on the
    Sessions tab. The contradiction Round 3 named is gone because the two
    meanings no longer share a token.
    The new `pending` specimen is coherent on its own terms: caption
    「快照可读，但早于工作信号字段——只保留待采集模块」 (`States.jsx:13`, English
    at `:18`), a hero showing real costs because the snapshot *is* readable,
    session statistics and per-project and recent-session rows present for the
    same reason, and only the work-signal modules flagged 待采集. Its Activity
    detail renders 「当前快照没有这些字段，需要新增采集」 over the retained
    `58 / 21 / 12 / 9` fixture with zero expandable rows — an old snapshot
    decoded, under a caption that says exactly that. There is no
    read-failure notice anywhere on it.
- Verified, not findings:
  - `npm run build` passes: 4581 modules, `dist/client` emitted.
  - The board is now six specimens and says so: `boardSubtitle` became
    「同一份界面在六种数据状态下的样子」 / "The same surface across six data
    states", and `Stage.jsx`'s direct control carries `pending` too. Across the
    board there is exactly one `.pending-flag` and twelve `.signal-card`s — three
    each on `normal`, `aged`, `partial` and `pending`, none on `empty` (its own
    note) or `unavailable` (its placeholder).
  - The captured path is intact. On the `normal` specimen the Activity detail
    still expands `编码 52%` into `新增 24% / 重构 13% / 测试 9% / 维护 6%`, four
    expandable rows, with no pending banner — so keying the fixture on a new
    token did not leak the pending presentation into the captured one.
  - The direct route works and is localized: `?state=pending&tab=sessions&lang=en`
    renders one flag reading "Not captured yet" over three signal cards, with no
    "Data could not be read" text anywhere on the page.
  - Round 1's other verified items are unaffected and were re-checked where
    cheap: the CLI page still carries its six shapes. `Cli.jsx` takes no `state`
    prop, so the state split cannot reach it.
  - The five existing state meanings are unchanged, and the repair touched no
    fixture.
- Evidence: `npm --prefix prototype run build`; `vite preview` on
  `http://localhost:4173`; a fresh worktree-scoped `agent-browser` session.
  Counts were taken with `get count`; each specimen's text was read by reducing
  to the largest element whose text starts with that specimen's caption, so no
  reading spans two cards. Source checked at `Popover.jsx:43,430,478-481,534,1113`,
  `States.jsx:13,18`, `Stage.jsx:1`, `i18n.js:196-206,409-419`.
- Completion gate: VERIFIED — the WorkUnit
  `work-signals:work-signal-prototype` was created on this passing round, as the
  three previous rounds each recorded it would be, and answers `pass` on all six
  criteria. Its content state is HEAD plus a manifest computed as the SHA-256
  over newline-terminated `<repository-relative-path>\t<git-blob-hash>` rows, in
  lexicographic order, for the seven files this task's review candidate spans:
  `docs/status.md`, `docs/topics/work-signals/reviews/work-signal-prototype.md`,
  `docs/topics/work-signals/tasks.md`, and the four changed `prototype/src/`
  sources. The resulting digest lives on the content-state node rather than in
  this sentence, because quoting it here would change the blob it is computed
  over. The bulk of the task's work product is already committed at `06a77b8`
  and `cb782d6`; the evidence is re-bound to the immutable tree once an
  authorized commit covers the remainder.
- Verdict: PASS
