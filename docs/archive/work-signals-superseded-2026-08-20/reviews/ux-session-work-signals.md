---
status: active
topic: work-signals
subject: docs/topics/work-signals/ux/session-work-signals.md
---

# Review log — work-signals / ux/session-work-signals.md

## Round 1 — 2026-08-20
- Reviewed state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`,
  `ux/session-work-signals.md` blob
  `36622be2881f403193fed21bf6404ad4959988f9` (untracked)
- Reviewer: claude-code
- Method: design review against three independent sources rather than against the
  document's own claims. Structure was checked against the prototype's
  `SignalDetail` and `SessionsPanel`
  (`docs/topics/desktop-app/ux/prototype/interactive-v7/src/Popover.jsx:402-570`);
  every copy string was checked against both catalogues in that prototype's
  `src/i18n.js`; field names and per-family availability were checked against
  `architecture.md` Decision 5; and the currently shipped surface was read in
  `apps/macos/AgentDeckApp/MenuBarPanelViews.swift:370-405` and
  `apps/macos/AgentDeckApp/DesktopCopy.swift:51,83-87`, because this document's
  implementation task replaces what that code renders. Ownership claims against
  `../../desktop-app/ux/menubar.md` were verified at the cited lines.
- Scope: `docs/topics/work-signals/ux/session-work-signals.md` only. Findings
  belonging to `architecture.md` or `tasks.md` are attributed and are not this
  document's to close.
- Findings:
  - [P1] The group-level `Not captured yet` badge has no captured-state binding.
    **Withdrawn in the same round, on the user's correction.** The finding rested
    on `architecture.md` Decision 5 giving each of the three families its own
    `available` flag, from which a mixed state — two modules captured, one not —
    was inferred, leaving a single group-level badge with no rule. The user
    confirmed the three families are produced together: `tasks.md` task 3 adds all
    three in one producer pass, so the mixed state is representable on the wire but
    not reachable in practice. With it gone, the remainder — that the document does
    not spell out that the badge disappears once values exist — is implied by the
    document's opening paragraph, which assigns the whole uncaptured treatment to
    `../../desktop-app/ux/menubar.md`, and by state-table row 1. Not a defect.
    Recorded rather than deleted so a later round does not re-derive it from the
    same contract wording.
    -> Withdrawn, not a finding.
  - [P2] "The last three rows are the only strings this topic adds" (line 104) is
    contradicted by the next sentence and by the prototype. The last three Copy
    rows are the pending banner, the attributed-cost note, and the kind-definition
    help; line 105 then states the pending banner "already exists in the prototype
    and is retained verbatim", which `i18n.js:65` and `i18n.js:263` confirm —
    `pendingHint` is present in both catalogues. Only **two** strings are new.
    Behavior risk: `tasks.md` task 4 instructs the implementer to "add only the
    three new strings the `ux` document names", so the wrong count is already
    propagating into the work instruction, and adding a third string means
    duplicating an existing key. 💡 Bounded remediation: change the count to two
    and name them, leaving the pending banner listed as retained.
    -> Open.
  - [P2] The kind-definition help string asserts a unit the design does not have
    for both clients. The Copy table ships `Turns that produced no tool call count
    as Conversation` / `未产生工具调用的轮次计为对话`, but `architecture.md`
    Decision 2 defines the classification unit as a turn for Codex and a
    tool-call group for Claude. For a Claude user the sentence names something the
    classifier does not operate on. Behavior risk: user-visible help text that is
    accurate for one client and not the other, in a product whose two clients are
    equal citizens. This is the copy's problem and therefore this document's; the
    unit definition itself is `architecture.md`'s and is already open there (see
    below). 💡 Bounded remediation: either reword the string so it does not name a
    unit — the surface only needs to say that work producing no tool call counts
    as Conversation — or make it conditional on the architecture closing its own
    finding by unifying the unit.
    -> Open.
- Attributed elsewhere, not this document's findings:
  - `tasks.md` task 4 repeats the "three new strings" count corrected above.
    Owner: `ad-ws-doc-tasks-design`.
  - `architecture.md` Decision 2's Claude grouping unit cannot represent a turn
    with no tool call, so `conversation` is unreachable for Claude. Already
    recorded against `ad-ws-doc-arch-design` in
    [`requirements.md`](requirements.md) Round 2 and not re-raised here; the P2
    above is about the copy, not about the unit.
- Non-findings, recorded so a later round does not re-raise them:
  - Every copy string in the table was matched character-for-character against
    both prototype catalogues, including `Read` / `读取`, `Edit` / `编辑`,
    `turns / edit` / `轮次/编辑`, and `Top MCP server` / `主要 MCP 服务`. The
    document is faithful to the specimen.
  - The summary lines, the four-cell metric grid, the fixed row orders, the
    inline `Most touched` and `Top MCP server` rows, and the `Back` control all
    match `SignalDetail`'s structure exactly.
  - Activity always rendering four kinds while Tooling omits an absent kind is
    asymmetric by design and the document states why; it matches
    `architecture.md` Decision 5, which bounds activity at exactly four records
    and tooling at "at most" four.
  - A Tooling module whose kinds are all absent is covered by the surface's
    general unavailable rule, which the document defers to and which
    `../../desktop-app/ux/menubar.md:687` states. Not a gap.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0 (the project's
  document-set audit; the only checker shipped for this target class).
  `make check-whitespace` — exit 0. `git diff --check` — clean. No product test
  run: the subject is an unimplemented design and no product content state
  changed. L0.
- Round outcome after the withdrawal: two open findings, both P2, both one-line
  corrections. Under the project's rule that no finding is deferred past `PASS`,
  the verdict stays `REOPEN`.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-20
- Repaired state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`,
  `ux/session-work-signals.md` blob
  `e0701fee4b2bdce178b626e8a95354a8569a0e7b` (untracked)
- Repairer: claude-code
- Scope: the two open P2 findings, in `ux/session-work-signals.md` only. The
  withdrawn P1 needed no change. No sibling document was touched.
- Findings closed:
  - [P2] The "three new strings" miscount -> the paragraph now states that this
    topic adds **two** strings, names them, and explains that the pending banner
    is listed because it is retained verbatim rather than because it is new.
    Verified against the prototype: `pendingHint` is present in both catalogues
    at `i18n.js:65` and `i18n.js:263`, so two is the correct count.
  - [P2] The kind-definition help naming a unit that only holds for Codex -> the
    string is now `Work that produced no tool call counts as Conversation` /
    `未产生工具调用的工作计为对话`, which states the rule without naming the unit
    it is evaluated over. A short paragraph records why, so a later editor does
    not "improve" it back to "turns". The string is one of this topic's own two
    additions and appears in neither prototype catalogue, so no prototype
    divergence is introduced.
- Not changed, and why: the underlying asymmetry — `architecture.md` Decision 2
  classifying over a turn for Codex and a tool-call group for Claude — is still
  open against `ad-ws-doc-arch-design` and is not this document's to close. The
  repair makes the copy correct under either resolution rather than betting on
  one.
- Downstream, not repaired here: `tasks.md` task 4 still says "add only the three
  new strings the `ux` document names". It is attributed to
  `ad-ws-doc-tasks-design` and correcting it from this round would close a
  finding against a task no reviewer has judged.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design; no product behavior is in scope.
- Dispatch: Beads `ad-ws-doc-ux-session-design` moved `in_progress` ->
  `in_review` in the same action, with the disposition recorded as a comment.
  The `round-1` label set by the Round 1 REOPEN is retained and not reset.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498`, `ux/session-work-signals.md` blob
  `e0701fee4b2bdce178b626e8a95354a8569a0e7b` (untracked) — the exact state the
  Round 1 repair recorded, verified with `git hash-object` rather than assumed.
- Re-reviewer: claude-code
- Method: the two closed findings were re-judged against the repaired text, and
  each premise the repair rests on was checked in the prototype independently
  rather than accepted from the repair note. The whole document was then re-read
  for regressions and for other instances of the *same* defect class, since a
  finding scoped to one line does not mean the fact is stated only once.
- Findings re-judged:
  - [P2] The "three new strings" miscount — **closed, confirmed.** The paragraph
    now states two, names them, and explains that the pending banner is listed as
    retained rather than new. Verified independently: `pendingHint` is present in
    both prototype catalogues (`i18n.js:65`, `i18n.js:263`), and neither
    `Cost attributed by session` / `成本按会话摊分` nor the kind-definition help
    appears anywhere in `i18n.js`. Two is the correct count.
  - [P2] The kind-definition help naming a Codex-only unit — **closed,
    confirmed.** The string is now `Work that produced no tool call counts as
    Conversation` / `未产生工具调用的工作计为对话`, which states the rule without
    naming the unit, and a paragraph records why so a later editor does not
    "improve" it back. **One correction to the repair's reasoning**, which does
    not reopen the finding: the repair claims the copy is "correct under either
    resolution" of the architecture's open unit defect. That is too broad. Under
    the current Decision 2 the string is still false for Claude — work with no
    tool call produces no group, so it is classified as nothing at all, not as
    `conversation`. What holds is narrower and sufficient: within the solution
    space `requirements.md` Goal 1 permits — every resolution must still classify
    into all four kinds — the string is correct. A resolution that dropped
    `conversation` for Claude would violate Goal 1 and this document's
    always-four-kinds rule, so it is not available.
- Repair boundary: **respected.** `tasks.md` task 4 still reads "add only the
  three new strings the `ux` document names" at line 114, unmodified. Correcting
  it from the repair round would have closed a finding against a task no reviewer
  had judged.
- New findings:
  - [P2] The document's own fidelity claim is false, and it is the **same defect
    the closed P2 corrected, in an earlier instance the Round 1 finding did not
    reach.** Line 23 asserts, without qualification, "Every element and every
    string below is taken from it" — `it` being the prototype. Two strings below
    are not: the attributed-cost note and the kind-definition help, which the
    repaired paragraph at line 104 now correctly identifies as this topic's own
    additions. The document therefore states the same fact two ways and they
    disagree. Behavior risk: this is the sentence that establishes why the
    document may be trusted as a faithful record of the specimen, so a reader
    verifying it against `i18n.js` searches for two strings that are not there
    and cannot tell whether the document or the prototype is wrong. The repair
    was correct not to fix this — the Round 1 finding was scoped to line 104 —
    but the class was not fully closed. 💡 Bounded remediation: qualify line 23
    to say every element and string is taken from the prototype except the two
    named additions, cross-referencing the Copy section.
    -> Open.
  - [P3] Line 119 introduces the state table as "the three module-specific
    bindings"; the table has five rows. The Accessibility section two headings
    later uses the same construction — "Three bindings specific to these
    modules" — and does have three, so this reads as a count that was not updated
    when rows were added. No decision changes and the table is self-evident.
    💡 Bounded remediation: state five, or drop the number.
    -> Open.
  - [P3] The kind-definition help's correctness depends on `architecture.md`
    Decision 2's open P1 being closed by unifying the classification unit, per
    the correction recorded above. The document explains why the string avoids
    naming a unit but does not record that the string is only true once that
    finding closes. A reader shipping this surface before the architecture is
    fixed would ship help text that is false for Claude users. 💡 Bounded
    remediation: one sentence noting the dependency, next to the existing
    explanation.
    -> Open.
- Attributed elsewhere, unchanged and not re-raised: `tasks.md` task 4's
  "three new strings" (owner `ad-ws-doc-tasks-design`), and `architecture.md`
  Decision 2's unreachable `conversation` for Claude (owner
  `ad-ws-doc-arch-design`).
- Non-findings from Round 1 were not re-derived; the copy-fidelity spot checks
  that section records were re-run on `Most touched` / `最常改动`,
  `Top MCP server` / `主要 MCP 服务`, and `turns / edit` / `轮次/编辑`, all of
  which match `i18n.js`.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **REOPEN** — one P2 open. The two Round 1 findings are genuinely
  closed; what keeps the cell unticked is that the first of them had a second
  instance upstream that the finding's scope did not cover.


## Round 2 — repair — 2026-08-20
- Repaired state: HEAD `9613498`, `ux/session-work-signals.md` blob
  `8ce84e560a07866ff5c8c28a0111991035c10c90` (untracked)
- Repairer: claude-code
- Scope: the three open Round 2 findings, plus one gap found while repairing
  them. The user authorized repairing everything in this document
  (`所有问题全部修复`), which is what extends the scope past the recorded
  findings; the extension is stated here rather than folded in silently.
- Findings closed:
  - [P2] Line 23's unqualified fidelity claim -> now reads "except the three
    additions the Copy section names", listing them, and states why the exception
    is enumerated: so the sentence stays checkable against `src/i18n.js`.
  - [P3] "the three module-specific bindings" over a five-row table -> the number
    is dropped rather than corrected. The Accessibility section's "Three
    bindings" was left alone; it has three and is right.
  - [P3] The kind-definition help's undeclared dependency on `architecture.md`
    Decision 2 -> a paragraph now records it: the string is false for Claude
    users under Decision 2 as written, the wording holds across every resolution
    `requirements.md` Goal 1 permits, and this surface must not ship before the
    architecture picks one. The Round 2 correction to the repair's earlier
    reasoning is adopted verbatim rather than argued with.
- Gap found and closed under the same authorization:
  - The Tooling detail dropped two fields the rest of the contract carries.
    `architecture.md` Decision 5's tooling family carries `groups` and
    `share_of_cost`, `cli-work-signals.md` renders both (line 90 text, line 152
    JSON), and the prototype has copy for both (`toolGroups`, `shareOfCost` in
    `i18n.js`) — but this document's Tooling section specified neither, so the
    panel would have shown two fewer numbers than the wire computes and than the
    terminal prints. Both are now inline rows with their source fields named.
    The prototype's `toolGroups` string is reused verbatim (`tool groups` /
    `工具组`); `shareOfCost` is **not**, because `of today's cost` /
    `占今日成本` predates the panel's period filter and would assert "today"
    under a `30d` scope. The period-bound half is dropped and the deviation is
    stated in the document at both the Tooling and Copy sections.
- Consequential update: the added-strings count moves from two to three
  (attributed-cost note, kind-definition help, period-neutral share-of-cost
  label). This is the same count the closed Round 1 P2 corrected, so it was
  updated in both places it appears rather than only where it was first found.
- Not changed, and why: `architecture.md` Decision 2's unreachable
  `conversation` for Claude stays open against `ad-ws-doc-arch-design`, and
  `tasks.md` task 4's "three new strings" stays open against
  `ad-ws-doc-tasks-design` — its count now happens to read correctly, which is a
  coincidence of this repair and not a closure, since the strings it names are
  not the three this document now adds.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design.
- Dispatch: Beads `ad-ws-doc-ux-session-design` moved `in_progress` ->
  `in_review` in the same action, with the disposition recorded as a comment.
  The `round-N` label is retained and not reset.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 3 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498`, `ux/session-work-signals.md` blob
  `8ce84e560a07866ff5c8c28a0111991035c10c90` (untracked) — the exact state the
  Round 2 repair recorded, verified with `git hash-object`.
- Re-reviewer: claude-code
- Method: the three closed findings were re-judged against the repaired text.
  The scope extension — a gap the repairer found and closed under the user's
  `所有问题全部修复` authorization — was treated as unreviewed content and
  judged from its own sources rather than from the repair note: the prototype's
  `i18n.js`, `architecture.md` Decision 5, and `cli-work-signals.md`.
- Findings re-judged:
  - [P2] Line 23's unqualified fidelity claim — **closed, confirmed.** The
    sentence now excepts the three additions and names them, and states why they
    are enumerated: so the claim stays checkable against `src/i18n.js`. The two
    places that state the count — line 23 and the Copy paragraph — now agree,
    which is the property whose absence was the finding.
  - [P3] The five-row table introduced as "three bindings" — **closed,
    confirmed.** The number is dropped rather than corrected, which is the more
    durable fix: a count beside a table that grows is a defect waiting to recur.
    The Accessibility section's "Three bindings" was correctly left alone; it has
    three.
  - [P3] The kind-definition help's undeclared dependency — **closed,
    confirmed.** A paragraph now states that the string is false for Claude users
    under Decision 2 as written, that the wording holds across every resolution
    Goal 1 permits, and that the surface must not ship before the architecture
    picks one. Round 2's correction was adopted rather than argued with.
- Scope extension, reviewed as new content:
  - The Tooling detail's two missing fields — **accepted.** The gap was real and
    verified from three independent sources: `architecture.md` Decision 5's
    tooling family carries `groups` and `share_of_cost`;
    `cli-work-signals.md` renders both, in its text sample and in its JSON; and
    the prototype has copy for both at `i18n.js:78,80,276,278`. A field the
    projection computes that neither surface shows would have been a field that
    should not be on the wire, so adding the rows is the correct direction. Both
    new rows name their source field.
  - The `share_of_cost` label's deliberate deviation — **accepted, and the
    reasoning is right.** The prototype's `of today's cost` / `占今日成本`
    (`i18n.js:80,278`) is period-bound and predates this panel's period filter,
    so under a `30d` scope it would assert "today". Dropping the period-bound
    half is the only option that does not put a false word on the surface, and
    the deviation is stated at both the Tooling and Copy sections rather than
    left for a later reader to discover as a mismatch against the specimen.
  - The consequential count change from two to three — **correct.** Verified
    against both catalogues: `toolGroups` exists (`i18n.js:78,276`) and is reused
    verbatim, so it is retained rather than new; `Share of cost` / `占成本`
    appears in neither, so it is this topic's third addition. Both places stating
    the count were updated together, which is what the Round 1 finding asked for
    and what Round 2 found had not been done upstream.
- Attributed finding **withdrawn**, with the repair note's reasoning corrected:
  - `tasks.md` task 4's "add only the three new strings the `ux` document names"
    was attributed to `ad-ws-doc-tasks-design` in Round 1 as a propagated
    miscount. It is no longer a defect, and not for the reason the Round 2 repair
    gave. The repair reasoned that "the strings it names are not the three this
    document now adds" — but task 4 names no string at all; it refers to whatever
    this document names, so it was never a count of its own. It was wrong only
    while this document said two, and it is right now that this document says
    three. Left standing, it would send whoever reviews `tasks.md` to "fix" a
    correct sentence. Withdrawn here and reported to its owner; the owner decides,
    since a finding is closed by the review that owns it.
- Still open elsewhere, unchanged: `architecture.md` Decision 2's unreachable
  `conversation` for Claude (owner `ad-ws-doc-arch-design`). This document now
  declares its dependency on that resolution, which is the most it can do about
  a defect it does not own.
- Regression checks: the Copy table's two added rows match the Tooling section's
  two added inline rows; the state table is still five rows and `tasks.md` task 4
  still describes it as five; the Accessibility section is untouched and still
  correct; the CLI's `16.2% of cost` phrasing differs from the panel's
  `Share of cost`, which `architecture.md` Decision 6 explicitly permits — copy
  is the one property the two surfaces do not share.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **PASS**


## Round 4 — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/session-work-signals.md` blob
  `8ce84e560a07866ff5c8c28a0111991035c10c90` (untracked) — byte-identical to the
  state Round 3 passed, confirmed with `git hash-object`. **The document did not
  change; its upstream did.**
- Reviewer: claude-code
- Method: this round exists because `architecture.md` Round 1 and Round 2
  repairs rewrote Decision 2 between Round 3's `PASS` and now, and this document
  states facts about Decision 2 in prose. Every such statement was re-checked
  against the current `architecture.md` (blob
  `c933e68ec0492d40f59a3aca4b2750a212400ac4`, `PASS` at its Round 3). The
  document was then re-read in full for other statements the same upstream
  change could have falsified, and the copy table was re-checked against the
  prototype catalogues, since a passing round is not a reason to re-derive less.
- Scope: `docs/topics/work-signals/ux/session-work-signals.md` only.
- Findings:
  - [P1] Lines 135-139 state a premise about `architecture.md` that is now
    false. The paragraph reads "`architecture.md` Decision 2 classifies over a
    turn for Codex and over a tool-call group for Claude, so a string saying
    'turns' would be accurate for one client and not the other." Decision 2 no
    longer says that: it defines the unit as a **turn in both clients** — one
    object with two boundary markers — and states explicitly that "the surfaces
    may say `turn` to a user of either client". Behavior risk: this is the
    document's stated reason for a copy decision, so a reader checking the
    reasoning against the contract finds it contradicted and cannot tell which
    document is current; the wording it justifies is still fine, which is
    precisely what makes the false premise easy to leave standing.
    💡 Bounded remediation: restate the reason. The neutral wording is still
    defensible — the surface needs to state the rule, not the unit — but it must
    be justified by something that is true, and the two-different-units claim no
    longer is.
    -> Open.
  - [P2] Lines 141-147 describe a closed finding as open and impose a shipping
    gate that is already satisfied. The paragraph says the help string "is
    correct only once Decision 2's own open finding closes", that "Claude work
    that produced no tool call forms no group and is classified as nothing at
    all", and that "this surface must not ship before the architecture picks
    one". The architecture picked one: its Round 2 re-review confirmed the
    unreachable-`conversation` finding closed, and its Round 3 re-review passed
    the document with a `VERIFIED` CEv1 gate. Behavior risk: a stale blocker is
    worse than no blocker — it reads as an outstanding dependency, so an
    implementer either halts on a gate that has already opened or learns to
    discount the document's gates.
    💡 Bounded remediation: replace the paragraph with the resolved statement —
    Decision 2 fixed the unit, the string is true for both clients, and
    `iteration_depth`'s `turns / edit` is now positively correct rather than
    tolerated — or delete it, since the dependency it tracked no longer exists.
    -> Open.
- Non-findings, recorded so a later round does not re-raise them:
  - The Tooling detail's `tool groups` row remains correct after the upstream
    rename. `architecture.md` Decision 4 now says in as many words that the
    tooling family's `groups` field counts tool kinds and not turns, which is
    what this document renders.
  - The Workflow table's `turns / edit` note is now positively correct for both
    clients rather than an exception, per Decision 2's closing paragraph. It
    needs no change.
  - Every other statement in the document is about the surface, the prototype,
    or Decision 5, none of which the architecture repair touched. The copy table
    was re-matched against both prototype catalogues and still holds, including
    the two additions and the deliberate share-of-cost deviation.
  - The document's conservative help-string wording is now more cautious than it
    needs to be. That is not a defect: it is true under the current Decision 2,
    and only its stated justification is wrong.
- Process note, not a finding against this document: no CEv1 evidence node
  exists for `work-signals:ux/session-work-signals.md`, nor for
  `work-signals:requirements.md`. Both documents have a ticked `Review` cell
  from a `PASS` round that recorded no evidence, so neither has a completion
  gate to reuse or invalidate. `architecture.md` and `ux/cli-work-signals.md`
  both do.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: REOPEN — the `Review` cell ticked at Round 3 is untied by this round,
  because the content it passed now asserts something its contract contradicts.

## Round 4 — repair, inside a cross-document sweep — 2026-08-20
- Repaired state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/session-work-signals.md` blob
  `927635819fa2eb1e6b5a80cbfc56ac11c8ed9933`, from the reviewed
  `8ce84e560a07866ff5c8c28a0111991035c10c90`.
- Repairer: claude-code
- Scope and why it is wider than one document: the user stopped the
  document-at-a-time loop and asked for one sweep over the whole set
  (`直接一次性全部重新评审一遍`). The loop was the problem, not the documents —
  reviewing leaf documents while `architecture.md` was still being repaired meant
  each contract fix invalidated a sibling that had already passed. This round
  therefore repairs every stale statement in the topic's product documents at one
  content state, and the sweep's other halves are recorded in
  [`ux-cli-work-signals.md`](ux-cli-work-signals.md) and in `tasks.md` itself,
  which has no review record yet because it has never been reviewed.
- Findings closed, this document:
  - [P1] The false Decision 2 premise -> the paragraph now states that Decision 2
    classifies over a turn in both clients, that naming the unit would also be
    correct, and that the neutral wording is a choice rather than a workaround.
  - [P2] The stale shipping gate -> replaced. The paragraph records that the
    resolution landed and that `iteration_depth`'s `turns / edit` is now
    positively correct for both clients, instead of blocking on a finding that
    closed.
- Sweep result over the whole set, verified by search rather than by reading:
  `rg` for `tool-call group`, `seven flag`, `three new strings`, `picks one`, and
  `uncapped by` across `requirements.md`, `architecture.md`, `tasks.md`, and both
  `ux/` documents returns nothing. The retired vocabulary is gone from every
  product document.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 5 — re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`, blob
  `927635819fa2eb1e6b5a80cbfc56ac11c8ed9933`, confirmed with `git hash-object`.
- Re-reviewer: claude-code. **Not independent of the repair** — the same actor
  wrote the Round 4 repair. Stated rather than glossed, because the record should
  not imply a second pair of eyes it did not have.
- Findings re-judged:
  - [P1] The false Decision 2 premise — **closed, confirmed.** The paragraph now
    says Decision 2 classifies over a turn in both clients and that naming the
    unit would also be correct. Verified against `architecture.md` — the unit is
    "a **turn** in both clients", and line 114 states the surfaces may say `turn`
    to a user of either client.
  - [P2] The stale shipping gate — **closed, confirmed.** Replaced by the
    resolved statement; no gate remains, and `turns / edit` is described as
    positively correct, which `architecture.md:114-115` supports.
- Every other reference this document makes to the contract was re-checked, not
  only the two repaired paragraphs: line 73's bare-file-name storage guarantee
  against Decision 1, and line 88's `groups` / `share_of_cost` against Decision 5
  and Decision 4's note that `groups` counts tool kinds. Both hold.
- New findings: none.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. L0.
- Verdict: **PASS**
