---
status: active
created: 2026-08-06
updated: 2026-08-17
---

# Native macOS Desktop App — Tasks

This file is the only status authority for this topic.

## Task breakdown

### 1. `desktop-wire-contract`

- Finalize the `v0.4.0` session DTO dependency and define the versioned desktop
  snapshot request/response contract.
- Keep Go aggregation, authorization, privacy filtering, partial results, and
  warnings authoritative.
- Add Go fixtures and Swift decoding fixtures from the same canonical examples.
- Document allowed desktop update-check connectivity and privacy behavior in the
  living specification when implementation makes it real.
- Verification level: L2 because this adds a stable JSON/exit-code contract.

### 2. `macos-app-foundation`

- Contract: [`architecture.md`](architecture.md#foundation-runtime) — approved.
- Add the Xcode project, macOS 26 targets, bundle identifiers, entitlements,
  shared Swift layer, helper runner, App Group snapshot store, OSLog policy, and
  unsigned local build path.
- Prove the host executes only its embedded helper and handles timeout,
  cancellation, unsupported wire version, partial data, and helper failure.
- Add Swift unit tests without reading real AgentDeck or client state.
- Verification level: L3 for new build and application boundaries.

### 3. `menubar-experience`

- Contracts: [`ux/menubar.md`](ux/menubar.md) for presentation, and
  [`architecture.md`](architecture.md#menu-bar-wire-contract-extension) for the
  additive `provider.candidates` section, the switch command surface, its result
  envelope, and switch operation ownership. Both were reopened by
  `ux/widget.md`'s W-F1; see the Documents matrix below.
- Implement provider, usage, cost, recent-session, warning, and health summaries.
- Add safe provider quick actions, refresh behavior, login-item preference, and
  newer-version notification that opens the official download page only.
- Define loading, stale, offline, partial, empty, and error states.
- Verify VoiceOver, keyboard navigation, reduced motion, high contrast, locale,
  narrow layout, and appearance changes on macOS 26.
- The `provider.candidates` extension is additive to task 1's delivered
  contract; it does not raise `wire_version` or reopen that task's review.
- Ship English and Simplified Chinese user-visible strings.
- Verification level: L3 including rendered and interactive acceptance.

### 4. `desktop-widget`

- Add WidgetKit timelines and App Intent configuration backed only by the
  redacted App Group snapshot.
- Define stale age, privacy redaction, placeholder, snapshot, timeline, and
  unavailable-host states.
- Prove the Widget cannot read AgentDeck databases, credentials, client config,
  or raw session sources.
- Verification level: L3 including extension sandbox and privacy checks.

### 5. `unified-desktop-distribution`

- Build the universal helper and full App bundle, sign nested code, notarize and
  staple the DMG, publish direct-download assets, render the `agentdeck-app`
  Cask, and add Formula-to-Cask migration and mutual-exclusion behavior.
- Preserve CLI-only Formula archives and tests.
- Verify fresh Cask install, upgrade, uninstall, direct DMG installation,
  optional user CLI link, completion loading, state preservation, Gatekeeper,
  arm64, and Intel behavior.
- Verification level: L4 through an expanded aggregate release gate.

### 6. `desktop-app-contract`

- Reconcile **this topic's** delivered behavior into the living specs and manual:
  the wire contract, menu-bar app, widget, packaging, and distribution behavior
  actually delivered.
- Close all review records for this topic and confirm the app, CLI,
  wire-contract, and package identities it produces agree with each other.
- **This task does not raise the specification version, run technical preflight,
  choose a release channel, or write release notes.** The version-wide raise is
  owned by the [v0.5.0 contract closure](../v0-5-0-contract/tasks.md). Preflight
  and any RC or stable publication remain separate, explicitly authorized
  workflows.
- Runs only after every other task in this topic has Review PASS.
- Verification level: L2 for contract state.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [ ] |
| ux/menubar.md | [x] | [ ] |
| ux/widget.md | [x] | [x] |
| tasks.md | [x] | [ ] |

`requirements.md` Review Round 1 (2026-08-17): **FAIL**. The boundary still
limits the named menu-bar outcome to current-day usage while the drafted
surfaces require bounded historical analytics, and it gives the Widget no
functional user-visible acceptance outcome. Both findings are recorded in
[`reviews/requirements.md`](reviews/requirements.md). Its `Review` cell remains
unchecked. Re-review Round 2 closed both original findings but found R2-F1: the
new prohibition on every other `breakdown` also forbids the non-temporal
breakdowns required by the authorized composition and trust questions. Re-review
Round 3 narrowed R2-F1 to one omission: the repaired authorization lists every
required non-temporal dimension except provider. Round 4 (2026-08-17) closed it —
`runtime provider` is authorized in both Goals and the Acceptance boundary, and
attribution quality is stated as reportable per client and per runtime provider,
matching what `ux/menubar.md`, `ux/widget.md`, and `architecture.md` all
already require. Re-review Round 5 (2026-08-17) independently confirmed R2-F1
closed and found no regression on the two original findings: **PASS**, `Review`
cell ticked, and the remaining four documents are unblocked. Round 5 also swept
the half of the scope prohibition Round 2 left unchecked and recorded R5-F1
against `ux/menubar.md:754`, whose period switcher asks for week and month
grouping that this requirement does not authorize and the projection does not
provision. It is attributed to that document and closes in its own review, so it
does not reopen this one.

`ux/menubar.md` and `ux/widget.md` are both drafted as of 2026-08-16.
`ux/menubar.md` now carries rendered specimens for healthy, loading, retained
offline, partial with incomplete pricing, the switch confirmation and its
in-flight and failed states, the 280 pt narrow bound, and empty — the readiness
condition it previously failed while stating geometry only as numbers.
`ux/widget.md` is new: it specifies both widget families, the App Intent
configuration, the surface/qualifier table over cache presence, version support
and age, copy in both languages, timeline construction, and the negative
privacy assertions.

Both remain unreviewed, so neither surface may enter development yet. The
document set is audited by `bash scripts/check-topic-docs.sh`, which compares this matrix
against the files on disk and against the surfaces `requirements.md` names.

The foundation runtime contract in `architecture.md` was reviewed and approved
under the previous per-task-design convention; that history is in
[`reviews/macos-app-foundation.md`](reviews/macos-app-foundation.md). The
menu-bar contract failed independent Design Review Round 3 on six blocking
findings recorded in
[`reviews/menubar-experience.md`](reviews/menubar-experience.md); those findings
span both `ux/menubar.md` and the menu-bar section of `architecture.md`, so the
`Review` cell for each stays unticked until the repair passes.

## Tasks

This matrix predates the staged progression, so it was written before
`architecture.md` and `ux/menubar.md` existed in reviewable form — the early
decomposition the progression now forbids. It is not being rebuilt from scratch:
tasks 1 and 2 are delivered and independently reviewed, and discarding a
decomposition that already produced verified work would cost more than it
corrects.

Instead, decomposition happens properly once the Documents matrix is green.
Tasks 1 and 2 then enter stage 5 as fixed inputs — their anchors, boundaries,
and evidence stay as they are — and tasks 3 through 6 are re-derived from the
reviewed specification rather than assumed from this list. A task whose scope
the specification does not support is dropped or re-cut then, which is the point
of decomposing after the design exists.

| Task | Dev | Review |
| --- | --- | --- |
| 1. `desktop-wire-contract` | [x] | [x] |
| 2. `macos-app-foundation` | [x] | [x] |
| 3. `menubar-experience` | [ ] | [ ] |
| 4. `desktop-widget` | [ ] | [ ] |
| 5. `unified-desktop-distribution` | [ ] | [ ] |
| 6. `desktop-app-contract` | [ ] | [ ] |

`desktop-wire-contract` Review Round 1 (2026-08-13): **FAIL**. The `Review` cell
remained unchecked pending the bounded filesystem-contract and
documentation-index Repair recorded in
[`reviews/desktop-wire-contract.md`](reviews/desktop-wire-contract.md).

`desktop-wire-contract` Re-review Round 2 (2026-08-13): **PASS**. Both Round 1
blockers are closed and the `Review` cell is synchronized.

`macos-app-foundation` development (2026-08-13): **COMPLETE**. The unsigned
Xcode build embeds the AgentDeck helper and shared framework; 10 isolated
XCTest cases passed.

`macos-app-foundation` Re-review Round 3 (2026-08-14): **PASS**. R2-F1 is
closed, all earlier findings remain closed, and 19 XCTest cases pass. Task 3
`menubar-experience` is the next task.

Menu-bar design Review Round 3 (2026-08-16): **FAIL** on six bounded contract
findings. Round 4 repaired all six and recorded the post-migration blob mapping.
Independent Re-review Round 5 (2026-08-16): **FAIL**. R3-F1, R3-F4, R3-F5, and
R3-F6 are closed; R3-F2's transport matrix and R3-F3's retry transition remain
open, and R5-F1 newly identifies conflicting ownership of the dynamic
`switch_in_flight` reason.

Round 6 (2026-08-16): repair complete, `REOPEN` pending independent Re-review.
The transport matrix is now total by construction with an explicit catch-all,
the controller carries a complete transition table making retry and dismiss
bounded exceptions to the non-idle refusal, and `switch_in_flight` is removed
from the wire and respecified as a host-only presentation overlay. Consequential
UX repairs followed: `Cancel` on a finished failure became `Dismiss`,
`indeterminate` was aligned to the same two actions as `failed`, and three
manual checklist items were added. R5-N1 was recorded but not authorized, and is
untouched.

Independent Re-review Round 7 (2026-08-16): **FAIL**. R3-F2 and R5-F1's
wire-ownership defect are closed, but R3-F3 remains open because terminal states
do not retain the complete credential/wrapper target required by same-target
retry. R7-F1 newly records that the architecture applies `Switch in progress`
to every non-idle terminal state while the UX limits it to `inFlight`.

Round 8 (2026-08-17): repair complete, `REOPEN` pending independent Re-review.
Both findings are closed — every non-idle controller state now carries the
complete resolved option so retry reads its target from the state, and the
overlay applies in `inFlight` alone. The same round absorbed a design review of
the rendered prototype: the popover lost invented window chrome, Settings, Quit
and provider switching moved into the footer menu, and both surfaces were
rederived around the four questions the usage data answers — magnitude,
composition, trust, rhythm — with widget size selecting depth rather than
subject. `architecture.md`'s App Group projection was extended to carry the
fields those surfaces asked for.

Independent Re-review Round 9 (2026-08-17): **FAIL**. R3-F3 and R7-F1 are both
closed, and the six findings Rounds 5 and 7 closed show no regression. Two new
blockers come from the redesign itself: Round 8 moved the prose and the
prototype to the four-section body and the footer menu, and two artifacts tied
to the old structure did not follow. R9-F1 is that all four text specimens in
`ux/menubar.md` still draw the five-section body with window chrome and a
flat `Settings…`/`Quit` footer, which the document itself designates as the
inline review entry point, so the file offers implementers two mutually
exclusive structures. R9-F2 is that the Data requirements table lists week and
month bucket grouping as provisioned when the projection carries only a daily
series, and `requirements.md` authorizes no granularity beyond it.

Independent Re-review Round 11 (2026-08-17): **FAIL**, with no serious finding.
Round 10 closed both of Round 9's blockers — the four specimens are redrawn onto
the four-section body with the window chrome and the flat footer gone, and the
Data requirements row now asks for the `today`/`7d`/`30d` selection the daily
`buckets` series actually backs. R5-N1 is closed as not reproducing, verified
back at the blob it cited; the prototype's `Month` tab is gone; the `usage
stats` capability callout was independently confirmed accurate and correctly
left alone; and a topic-wide sweep found no other week/month claim.
`architecture.md` is unchanged and carries no open finding. Two new non-blocking
findings keep the round from passing, both of the same kind as R9-F1 and both
inside the artifact Round 10 redrew: R11-N1, thirteen specimen frame lines one
column wider than their border, a regression against the previous blob's uniform
widths; and R11-N2, the empty specimen printing `No activity today` where the
copy table fixes `No local activity today`. Under the no-deferred-findings gate
adopted this day, a minor finding blocks `PASS`, so both Document cells stay
unticked; the repair is about fourteen lines and touches no behavior contract.

Independent Re-review Round 13 (2026-08-17): **PASS**. Round 12 closed both:
every specimen frame line re-measures at exactly 46 or 36 display columns —
the two rows carrying annotations outside the box excluded, as R11-N1 itself
excluded them — and the empty specimen now reads `No local activity today`,
the string the copy table fixes for a current, issue-free surface. The repair
also corrected the one same-kind row R11-N1 had not named. No new finding, and
no regression in the four-section structure, the Data requirements row, or the
other fixed copy. Both `architecture.md` and `ux/menubar.md` therefore have
every finding closed since Round 1, and both Document cells are ticked. Their
CEv1 Document gates re-query as `VERIFIED` against this exact uncommitted
candidate state, which must be re-recorded against the Git tree once an
authorized commit exists. Each of these documents is a task in its own right,
so both now sit at `awaiting_commit`: the work product exists and has passed,
and only an authorized commit closes them. What is committable is the two
documents, this file, `docs/README.md`, and the review record — not the
`menubar-experience` implementation anchor, which still has no implementation.

`ux/widget.md` Review Round 1 (2026-08-17): **FAIL**, recorded in
[`reviews/ux-widget.md`](reviews/ux-widget.md). Three findings are local
alignment work, but W-F1 is not: the widget's `Period` parameter and its
three-period comparison need per-period aggregates the App Group projection
does not carry, and a widget has no second way to get them — it cannot invoke
the helper, and deriving them in Swift is what `requirements.md` and
`ux/menubar.md:55` both forbid.

That finding reopens two documents this topic had already closed. The same
projection gap sits under `ux/menubar.md:754`'s period switcher, whose row
claims `today`/`7d`/`30d` are provisioned; they are not, and Re-review Round 13
passed that row by checking it against three documents that all repeat the same
unprovisioned claim instead of against the projection itself. Round 14 of
[`reviews/menubar-experience.md`](reviews/menubar-experience.md) withdraws that
`PASS`. Both `Review` cells are unticked again and both CEv1 gates are back to
`FAILED`. Commit `10ce01e` is not reverted: the repairs it carries are real, and
only the gate-closing conclusion drawn from them was wrong.

`ux/widget.md` Re-review Round 3 (2026-08-17): **FAIL**. Round 2 took the
user's chosen path and extended the App Group projection to carry per-period
totals and per-period model shares, plus the two rhythm-day fields, and it
also corrected `ux/menubar.md:754`'s mechanism wording rather than letting the
extension merely make the old sentence true. W-F1 through W-F4 are closed on
the elements they named. Two same-source residuals keep the document open:
W-F5, `composition` large's per-client subtotals are still single-period while
`composition` accepts a `Period`; and W-F6, the two new rhythm fields are
provisioned over a 90-day window while `rhythm` displays a 30-day one. Both are
the recurring shape in this topic — the repair answered the finding's line
numbers instead of the set of elements the same decision governs.

`architecture.md` and `ux/menubar.md` were both edited by that repair, so on
top of being reopened by Round 14 they now carry content states no re-review
has judged. Their independent re-review should run after W-F5 and W-F6 close,
since W-F5's fix most likely lands in `architecture.md` again.

`ux/widget.md` Re-review Round 5 (2026-08-17): **FAIL**. W-F5 and W-F6 are
closed, and closed well — the per-client bullet was split by consumer rather
than changed uniformly, because `composition` takes a `Period` and `trust` does
not, and the rhythm sentence that made the window ambiguous was rewritten
rather than just renumbered. Two new findings: W-F8 (blocking) — `trust` shows
per-tier **amounts** at every size while the projection provisions per-tier
**counts**, and the document's own Data requirements row asks only for counts,
so the target contradicts itself; and W-F7, a citation to an `architecture.md`
revision ordinal that does not exist by that file's own numbering.

W-F8 is the third appearance of one problem: a displayed element whose shape
the projection does not carry. `Period` exposed the first, the shared bullet
the second, and `trust` — governed by neither — the third. The convergent fix
is to map every Data requirements row one-to-one onto a projection bullet and
check the field's *shape*, not its name; "attribution counts" matches
"attribution counts" by name while money and cardinality are different data.

`ux/widget.md` Re-review Round 7 (2026-08-17): **FAIL**. W-F7 and W-F8 are both
closed — the quality tiers now carry `(cost, tokens, count, share)`, the same
shape as the projection's other per-dimension breakdowns, and both documents
now share one explicit revision sequence instead of two disagreeing ordinals.
Running the row-to-bullet shape mapping Round 5 prescribed then found W-F9 on
the one governing dimension no round had swept: `Client` takes `all`, `codex`,
or `claude` on every widget, while the projection keys only three things by
client. At `Client = codex`, `composition` and `rhythm` have no data at any
size, and `magnitude` keeps its cost and tokens but loses its chart, `avg/day`,
`peak`, and session count.

`ux/widget.md` Re-review Round 9 (2026-08-17): **FAIL**, with no serious
finding. W-F9 is closed, and closed most thoroughly of the series: the two
cross terms are provisioned as products rather than as single dimensions, the
ceilings were restated per scope with truncation held inside each scope so a
busy client cannot eat another's budget, and the table gained a `Varies by`
column that turns the next check of this kind from reading the document into
reading one column. The choice between the two paths was made after measuring
the cost (906 entries against 309) rather than deferred a second time. A
thirty-six cell enumeration of `Client` x `Period` x kind x size found no cell
outside the projection. What keeps the document open is W-F10: `Cost
incomplete` is a label this document displays and specifies, yet it has no Data
requirements row and no stated client scope, and the same repair left
`architecture.md` describing per-period totals in two overlapping bullets where
only the unscoped one mentions pricing completeness. One table row and one
bullet merge close it.

Recorded against `architecture.md` rather than this document: its
sixth-revision paragraph says nine bullets gained a client scope, while five
did and its own following sentence names six things. That closes in
`reviews/menubar-experience.md`, whose gates have been open since Round 14.

`ux/widget.md` Re-review Round 11 (2026-08-17): **FAIL**. W-F10 is closed by
merging the two per-period totals bullets into one client-scoped cell that
carries counts, session count, pricing completeness, and cost strings together,
so `Cost incomplete` now qualifies the number beside it; the table gained the
matching row. The repair also removed the aggregate session availability/count
bullet after checking for consumers, and that check holds independently — the
projection is read by the widget alone, the menu bar reads the wire snapshot.
W-F11 keeps the document open: the timeline's refresh-after reads "the
projection's next suggested refresh time", which the projection does not carry;
that field lives in the wire snapshot, and the projection list is a *may contain
only* enumeration, so the timeline's stated input is not merely absent but
disallowed. One bullet and one row close it.

W-F10 and W-F11 came from the same sweep at different breadths — Copy table and
widget bodies the first time, Timeline and Accessibility added the second. The
completeness test for the Data requirements table is therefore "every place this
document says it reads the projection has a row", not "every visible element has
a row". No section that specifies reading remains unswept.

`ux/widget.md` Re-review Round 13 (2026-08-17): **PASS**, and its `Review` cell
is ticked. W-F11 closed by projecting the next suggested refresh time beside
`generated_at` — the scalar the wire snapshot already carries — so the timeline
has the baseline its clamp needs without a new refresh policy. All eleven
findings are closed with no regression, and a full pass over every section that
specifies reading the projection found nothing further.

Six of those eleven were one problem: a displayed field the projection did not
carry. What ended it was not effort but the test getting wider each round —
from the named lines, to the element set one decision governs, to a row-by-row
shape mapping, to the full `Client` x `Period` x kind x size enumeration, to
every place the document says it reads the projection. `architecture.md` was
revised seven times across those rounds, all driven by this document.

`architecture.md` and `ux/menubar.md` stay unticked. None of those seven
revisions has been independently re-reviewed, and both documents have been
reopened since `reviews/menubar-experience.md` Round 14. They should be
re-reviewed together, because they bind to the same projection contract.

That is the fourth appearance, and it also bounds the problem: this document
has exactly three governing dimensions — `Client`, `Period`, and `rhythm`'s
window — and all three have now been swept. The next repair should close the
whole parameter space at once rather than one dimension per round, checking
`Client` × `Period` × kind × size and especially the cross terms, since a
per-client `composition` at `7d` needs model shares keyed by both and
provisioning either dimension alone does not produce it.

The target was documents, not task 3, which has no implementation. Task 3 stays
blocked until this topic's remaining documents pass, since a task matrix is a
draft until they do.

Next action:

```text
复评：desktop-app / reviews/menubar-experience.md
```

Task 1 was blocked on the `v0.4.0` session DTO contract; that dependency is now
satisfied. Task 2 consumes task 1. Tasks 3 and 4 depend on task 2 and may
proceed independently after the shared snapshot contract is fixed. Task 5
integrates tasks 2-4. Task 6 runs last within this topic, and in turn gates the
[v0.5.0 contract closure](../v0-5-0-contract/tasks.md).

Commit boundaries follow task boundaries. This topic does not authorize commits,
pushes, certificate creation, secret changes, release publication, Homebrew tap
changes, local installation, or external distribution.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
开发：`desktop-app` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the current release and
versioning contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`reviews/<task-anchor>.md` before ticking `Review`.
