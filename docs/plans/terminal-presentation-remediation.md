---
status: active
created: 2026-08-11
---

# Terminal Presentation Remediation

This feature plan corrects terminal presentation defects reported during manual
acceptance of `v0.4.0-rc.3`. The published RC tags and releases are immutable.
The local branch now contains three unpushed implementation commits for Tasks
1-3 and an uncommitted Task 4 development candidate. Those implementation
results are retained, but the review process used for Tasks 1-3 was rejected by
the user and provides no valid Review or Test gate.

After all five tasks reach Review PASS and the exact resulting commit passes the
local and isolated-real-state gates below, that commit may proceed through a new
manual `release-preflight`. If the preflight succeeds, the user-authorized
delivery workflow may publish the next available `v0.4.0-rc.N` prerelease.
`v0.4.0-rc.4` is only the current expectation while `v0.4.0-rc.3` remains the
latest remote tag; execution must discover the next tag again immediately before
tag creation.

## Review reset — 2026-08-11

The user invalidated every existing PASS conclusion for this plan. Earlier
review files and test output remain only as audit history and diagnostic hints;
they cannot be cited to tick a gate, close a finding, skip a review surface, or
claim a Task or Plan complete.

Current baseline:

| Task | Retained implementation state | Authoritative next gate |
| --- | --- | --- |
| `session-show-activity-presentation` | Isolated review commit `1f026a3cada47dcd9493781a44cba263e0040480`, tree `af31e270b6e639af8302bf241e6c1d6cf782332b`; signed and not pushed | Completed — Reset Round R3 Review PASS, targeted tests PASS, CEv1 VERIFIED |
| `interactive-detail-language` | Full Reset repair candidate on the isolated review branch; exact tree recorded by CEv1 | Completed — independent Full Reset Review R1 and Full Reset Re-reviews R1-R2 closed every finding; targeted tests PASS; CEv1 VERIFIED |
| `usage-interactive-detail` | Full Reset repair candidate on the isolated review branch; exact tree recorded by CEv1 | Completed — independent Full Reset Review R1 and Full Reset Re-reviews R1-R3 closed every finding; targeted unit, PTY, and race tests PASS; CEv1 VERIFIED |
| `session-interactive-responsive-layout` | Post-PASS PTY harness repair candidate on the isolated review branch | Post-PASS Full Reset Review R3 FAIL repaired; Full Reset Re-review R4 PASS; atomic commit and commit-bound CEv1 pending |
| `terminal-contract-and-acceptance` | Living contracts and compiled-current-candidate acceptance recorded on the isolated review branch | Acceptance remains recorded, but Task 5 is paused for rebind and complete re-review after Task 4's new commit |

`origin/main` remains `5afc0a1` (`v0.4.0-rc.3`); local `main` is currently
`bbbe5be`. These identifiers describe the planning baseline only. Execution must
verify them again and stop on drift rather than forcing the worktree back to this
snapshot.

The three existing implementation commits must not be amended, rebased, reset,
or otherwise rewritten. Any repair and every replacement review record lands in
a later task-scoped commit. Task 4 dirty work must be preserved. In particular,
its changes to `terminal_detail.go` and `terminal_detail_test.go` overlap Task 2:
execution first assigns each hunk to one task and records a patch fingerprint.
Shared-renderer cleanup belongs to Task 2 when it defines the final shared
contract; the remaining Session consumer changes stay with Task 4. If that split
cannot be staged and verified safely, stop and report the conflict instead of
using destructive Git operations.

## Isolation worktree topology — approved 2026-08-11

All implementation, review, repair, verification, and delivery work resumes in
one isolated worktree. This section supersedes every earlier instruction to
continue on local `main`, add repair commits on top of its three draft commits,
reset local `main`, or cherry-pick the finished result a second time.

| Role | Ref or path | Contract |
| --- | --- | --- |
| Safety source | Current repository worktree on local `main` | Read-only source for the three draft commits, Task 4 dirty payload, this approved plan, and invalidated review history. Do not stage, commit, reset, clean, switch, amend, rebase, or otherwise mutate it during execution. |
| Remote baseline | Freshly fetched `origin/main` | Expected planning identity `5afc0a18142f7e08137349d39ad961bbc7315f4b`; verify at execution time and stop on unexplained drift. |
| Review worktree | `/private/tmp/agent-deck-terminal-presentation-remediation-v2` | The only worktree in which payloads, repairs, tests, review records, and task commits are written. If this path already exists, do not remove or reuse it until its ownership and state are proven. |
| Review branch | `review/terminal-presentation-remediation-v2` | Create directly from the verified `origin/main`; it becomes the sole candidate history and final delivery source. If the branch already exists, stop rather than reset or overwrite it. |
| Final candidate | Review branch `HEAD` after Task 5 and plan closure | One exact SHA, denoted `S`, binds local acceptance, CEv1, remote `main`, release preflight, RC tag, artifacts, Homebrew RC, and local installation. |

Local `main` is deliberately allowed to remain ahead, dirty, and later diverged
from `origin/main`; preserving it is not a delivery defect. Its cleanup or
reconciliation is a separate destructive/history operation and is not part of
this plan.

### Baseline and drift gate

Before creating the review worktree:

1. Fetch `origin/main` and tags, then record full object IDs for local `main`,
   `origin/main`, Tasks 1-3, and the annotated `v0.4.0-rc.3` tag target. Verify
   the expected draft chain is exactly `origin/main -> 11c9236 -> 25ea084 ->
   bbbe5be`; no abbreviated ID is accepted as the evidence identity.
2. Capture the safety source's complete status, tracked/untracked path list, and
   binary-capable diffs. Record separate SHA-256 fingerprints for the approved
   plan/index control payload, each Task 1-3 commit payload, Task 4 product/test
   payload, the shared-Detail overlap slice, and each invalidated review record.
   A hash is evidence of extraction only; inspect the corresponding path and
   hunk manifest as well.
3. Classify every dirty hunk before export. The plan and `docs/README.md` are
   execution-control payload. The three existing review records are task-local
   audit payload. Session browser/viewer code and tests are Task 4. Changes in
   `terminal_detail.go` and `terminal_detail_test.go` are split by behavior:
   shared model/renderer contract and its tests belong to Task 2; Session-only
   producer/consumer behavior belongs to Task 4. Changed terminal design specs
   are assigned to the task whose accepted contract they describe, or deferred
   to Task 5 when they reconcile combined final behavior.
4. Stop before creating or applying anything if the remote baseline, draft
   commit chain, dirty path set, payload fingerprints, branch name, worktree
   path, or hunk ownership differs from the recorded baseline and cannot be
   explained by this plan-only update. Never repair drift with reset, clean,
   stash-pop, amend, rebase, force, or a bulk checkout.
5. Only after the gate is recorded, create
   `review/terminal-presentation-remediation-v2` at the exact verified
   `origin/main` and attach the fixed isolated worktree. Confirm the new
   worktree is clean and its `HEAD` equals the recorded baseline before applying
   the first payload.

### Payload migration contract

Migration is content-level and serial. The old Task 1-3 commits are immutable
sources, not commits in the final legal history. Export their exact diffs with
path manifests, apply only the current task's payload to the review worktree,
then form a new atomic task commit only after the reset-era review loop passes.
Do not cherry-pick an old commit as a recorded commit and do not reuse its old
review/status conclusion.

1. **Control payload and Task 1** — apply the approved plan/index snapshot plus
   Task 1's production/test hunks from `11c9236`, and import its review record as
   invalid audit history. Perform the complete reset review loop and create the
   new Task 1 commit.
2. **Task 2** — apply the shared Detail production/test hunks from `25ea084`
   plus the classified shared-renderer slice extracted from the dirty Task 4
   draft. Import the invalidated Task 2 review record, complete review/repair/
   re-review, and commit. This commit freezes the shared Detail contract.
3. **Task 3** — only after that freeze, apply the Usage production/test hunks
   from `bbbe5be` and its invalidated review record. Complete the full loop and
   create the new Task 3 commit.
4. **Task 4** — apply the fingerprinted remainder of the dirty Session payload,
   including only its owned code, tests, and contract hunks. Verify that the
   applied diff equals the exported manifest and that no control, Task 2, or
   unrelated hunk leaked into it. Complete the full loop and create its first
   legal task commit.
5. **Task 5** — develop and review final contract/acceptance material on top of
   the four newly legal commits, then close and archive the plan in its own
   reviewed commit boundary defined below.

At each import boundary, the review worktree must be clean before applying the
next task. A conflict, unexpected dependency, changed fingerprint, or unsafe
hunk split is a stop condition; it is not permission to import all remaining
draft changes or rewrite either history.

## Execution authorization checkpoint

This revision records the approved topology but does not execute it. Updating
this plan and `docs/README.md`, followed by documentation-only validation, is the
entire authorized scope of the current session. Creating the branch/worktree,
exporting or applying any payload, changing product code or tests, reviewing,
repairing, committing, pushing, dispatching workflows, publishing an RC,
changing Homebrew, or installing locally begins only after the user confirms the
authoritative new-session execution instruction below.

## Goal

Make three related surfaces readable and information-dense without becoming
empty, gray, stretched, or unlabeled:

1. ordinary `session show --activity` text output;
2. selected-row Detail in `usage stats --interactive`;
3. the root `session --interactive` browser and its nested/direct Session detail
   viewer.

The result must preserve JSON values and shape, pagination data, pricing and
attribution, session-source privacy, read-only source logs, and the existing raw
terminal lifecycle.

## Scope

### Ordinary `session show --activity`

- Replace both extremes that failed acceptance: the old seven-line field dump
  and the draft's dense unlabeled single-line record.
- Render each safe activity call with a stable ordinal and explicit field
  labels. At wide widths use a bounded table; at standard widths use a compact
  two-line record; at narrow widths stack labeled fields.
- Omit optional `unknown`, `unavailable`, empty, and redundant values. Keep an
  explicit empty/safe-metadata state when a call has no displayable safe fields.
- Prefer `DURATION` as the completion boundary. Show `COMPLETED` only when a
  duration is unavailable and a valid completion time adds information.
- Preserve the Activity aggregate, result order, shown/total counts, next-page
  command, active display-zone naming, control sanitization, and JSON contract.

### Usage and Session interactive Detail

- Use one structured Detail model. Producers provide label, value, semantic
  role, priority, and optional long-note status; the renderer must not split
  concatenated `LABEL VALUE` strings or infer meaning from keyword searches.
- A Detail card exists only when at least one non-redundant supplementary field
  or required warning/note exists. No empty title or blank reserved region.
- Optional zero token/cache components, empty values, unavailable optional
  activity fields, and values already carried by the selected row are omitted.
  A zero or unavailable value that changes meaning, such as a required warning
  or pricing state, remains explicit.
- Labels remain visible in color and no-color modes. Values use semantic roles:
  token/capacity cyan, cost yellow, session/event magenta, success/complete/
  priced green, partial/unpriced/stale warning yellow, error/failed red, and
  neutral time/duration/text information color.
- When two field cells retain useful minimum widths, use a two-column compact
  grid. Otherwise use one aligned label/value column. Long approved text and
  notes span the card and wrap by visible cells.

### Session responsive geometry

- Session content uses a left-aligned readable canvas of at most 120 visible
  cells. The alternate screen may cover the terminal, but record columns never
  consume physical right-edge whitespace merely because it exists.
- Client, Session, Model, Project, and Last Activity have bounded minimum and
  maximum widths. Project may flex only inside its bound; Last Activity follows
  the preceding field instead of being pushed to the terminal's last column.
- At 80 cells and above, use bounded columns. At 48-79 cells, render a complete
  item as a two-line left-aligned record; do not use space-between alignment.
- Root browser and nested detail calculate body line capacity after actual title,
  tabs/header, advisory, status, and help rows. Complete rendered records, not a
  fixed item constant, consume that capacity.
- Remove fixed `sessionViewerPageLimit = 20` behavior. A nested section acquires
  enough bounded rows for the current body capacity. The selected Detail consumes
  vertical lines only when stacked; side-by-side Detail does not. When Detail is
  absent, rows immediately receive the full body capacity.
- Keep acquisition page and visual viewport separate. Arrow navigation can
  scroll all acquired rows; PageUp/PageDown moves by the current acquisition
  capacity without skipping rows.
- Preserve selected stable identity and absolute ordinal across resize. Recompute
  page, selected index, and viewport for the new capacity; use ordinal fallback
  only if the identity disappeared after a data refresh.
- Status reports the absolute visible/acquired range, total, and selected ordinal
  rather than relying only on a page number whose size changes with terminal
  height.

### Usage visual baseline

- Preserve Usage's compact row label, bar, and value behavior as the visual
  baseline. This plan does not stretch Usage to match the old Session layout.
- Change Usage outside the Detail region only when required to prevent an empty
  card from reserving height or to preserve selection/viewport on resize.

## Observable layout contract

### Ordinary Activity

At a wide readable canvas:

```text
ACTIVITY
  #  STARTED                   TOOL       MODEL        STATUS     DURATION
  1  2026-08-11 10:30:00 PDT  Read       claude-opus  complete   1.24s
```

At a standard width:

```text
CALL 1  Read  complete
  STARTED  2026-08-11 10:30:00 PDT  MODEL  claude-opus  DURATION  1.24s
```

At a compact width:

```text
CALL 1
  TOOL      Read
  STATUS    complete
  STARTED   2026-08-11 10:30:00 PDT
  MODEL     claude-opus
  DURATION  1.24s
```

These examples fix information order and labeling, not literal spacing. Every
line must fit its selected visible width after ANSI removal and terminal-cell
measurement.

### Interactive Detail

```text
DETAIL - TOKENS
  INPUT TOKENS       1.24M      OUTPUT TOKENS      318K
  CACHE READ         820K       PRICING STATUS     complete
```

`complete` remains explicit in no-color mode and uses the success role in color
mode. If every supplementary field is omitted, `DETAIL - TOKENS` is not emitted
and no body lines are reserved for it.

### Session browser

Wide and standard layouts keep bounded columns inside the readable canvas:

```text
> claude  session-id  claude-opus  project-name  2026-08-11 10:30 PDT
```

Compact layout uses only left alignment:

```text
> claude / session-id
  claude-opus - project-name - 2026-08-11 10:30 PDT
```

## Terminal matrix

| Mode | Geometry | Required behavior |
| --- | --- | --- |
| JSON | Any | Existing byte-shape/value contract; no ANSI, cursor control, prompts, or geometry-dependent omission. |
| Redirected text | Actual `COLUMNS`, otherwise existing deterministic fallback | Responsive ordinary Activity, copyable labels, no raw mode or cursor control. |
| TTY ordinary text | Live width | Same Activity information contract with optional decorative semantic color only. |
| Interactive color | TTY stdin/stdout, usable `TERM`, at least 48x10 | Responsive full-screen viewer, semantic roles, dynamic width and height. |
| Interactive no-color | Above plus `NO_COLOR` or `--no-color` | Same labels, order, values, warnings, marker, selection, and viewport without ANSI. |
| Unsupported interactive | Non-TTY, non-text format, `TERM=dumb`, or below 48x10 | Reject before acquiring raw mode or alternate-screen ownership. |

Required geometry fixtures are 48x10, 60x18, 80x24, 100x24, 120x32,
140x32, and 180x40. Coverage includes CJK, emoji, combining marks, long model
and project values, ANSI/control injection, and an unusually long approved note.

## Interaction and resize state

Each interactive section retains independent acquisition position, selection,
viewport, and content state. Resize does not silently reset section or selection.

```text
section
  acquisition start and limit
  selected stable identity and absolute ordinal
  viewport start and visible capacity
  loaded | empty | warning | partial | stale | unavailable | error
```

The renderer computes:

```text
body lines = terminal rows - actual chrome lines
detail lines = 0 when absent or side-by-side
list lines = body lines - stacked detail lines - required separator
visible rows = complete rendered records fitting list lines
acquisition limit = complete rows fitting the full body line budget
```

The acquisition limit stays stable while selection moves inside one geometry.
Changing selection therefore does not reload data. Resize may change the limit
and causes at most one anchored reload.

## Accessibility, privacy, and fallbacks

- Meaning never depends on color, horizontal position, or an icon.
- Selection always has a textual `>` marker. Warning, partial, unpriced,
  unavailable, and error states always have words in addition to color.
- Visible width is measured in cells after sanitization and independently of
  ANSI. No line may expose untrusted terminal control sequences.
- Activity remains limited to approved safe metadata. Tool arguments, results,
  commands, environment, hidden reasoning, prompts, credentials, and private
  source paths remain excluded.
- Ordinary text remains the screen-reader/copyable fallback; JSON remains the
  machine-readable fallback.
- Existing Escape, q, Ctrl-C, EOF, cancellation, SIGWINCH, input-reader release,
  raw-mode restoration, cursor restoration, and alternate-screen restoration
  contracts remain unchanged.

## Current draft reconciliation

The first development task begins by classifying each uncommitted hunk against
this plan. It must not perform a bulk rollback.

- Keep as design intent, then rework as necessary: optional-field filtering,
  shared Detail rendering, semantic colors, empty-card suppression, and their
  focused tests.
- Replace: string-based Detail parsing, keyword-based color inference, elastic
  Session value/right-edge padding, fixed 20-row nested paging, and the draft's
  one-line ordinary Activity format.
- Preserve: unrelated user work and all established JSON/privacy/lifecycle
  behavior.
- Documentation that currently describes the draft as implemented must be
  reconciled only when the corresponding task reaches its contract gate.

The currently reported
`TestSessionBrowserPTYOpensNavigatesAndReturnsFromStructuredDetail` timeout is
not accepted as a green baseline and must not be hidden by increasing a timeout.
If still reproducible when execution starts, diagnose its readiness oracle. PTY
visual assertions that cross styled label/value boundaries must normalize ANSI
and intentional padding or assert separate stable semantic markers while still
preserving exact lifecycle assertions.

## Tasks

### `session-show-activity-presentation`

Deliver the approved ordinary `session show --activity` responsive grammar.

Implementation scope:

- Rework Activity rendering in `cmd/agentdeck/session_show_text.go` without
  changing result acquisition or JSON DTOs.
- Replace the draft's dense single-record behavior with the wide, standard, and
  compact layouts above.
- Add field-selection helpers that distinguish absent optional data from required
  semantic state.
- Strengthen `cmd/agentdeck/session_show_text_test.go`, relevant CLI coverage in
  `cmd/agentdeck/main_test.go`, and display-time coverage without binding tests to
  incidental padding.

Acceptance criteria:

- Every safe call remains distinguishable and explicitly labeled at 48, 80, 100,
  120, and 180 visible cells.
- Increasing width never pushes a value to a remote edge or collapses all labels
  into an ambiguous sentence.
- Unknown optional fields and redundant `COMPLETED` values disappear; required
  empty state, ordering, totals, pagination, and display zone remain correct.
- Text contains no control injection or overflow after ANSI removal; JSON output
  is unchanged for the same result.

Development verification:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run 'Test(RenderSessionShowText|SessionShowActivityLines|SessionShowActivityReadsOnlySafeMetadata)'
```

Verification level: L2 human text/JSON command contract.

Proposed commit boundary after Review PASS:
`fix: make session activity output responsive`.

### `interactive-detail-language`

Deliver the structured Detail model and shared responsive card renderer.

Implementation scope:

- Replace the draft string parser in `cmd/agentdeck/terminal_detail.go` with an
  explicit structured field/note contract and semantic roles already provided by
  domain adapters.
- Render one/two-column Detail cards by visible-cell capacity; support wrapping
  notes, empty-card suppression, and no-color equivalence.
- Keep shared primitives command-layer only; add no TUI framework or dependency.
- Rewrite `cmd/agentdeck/terminal_detail_test.go` around observable roles,
  content, geometry, sanitization, and absence rather than ANSI byte adjacency.

Acceptance criteria:

- The renderer never derives a label or color by searching display text.
- Optional empty fields produce no rows; required warnings remain explicit.
- A card with no content consumes zero lines.
- Color and no-color frames contain the same semantic labels, values, order, and
  selection meaning at all required widths.

Development verification:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run 'TestRenderTerminalDetail'
```

Verification level: L1 shared renderer behavior.

Proposed commit boundary after Review PASS:
`refactor: structure interactive detail rendering`.

### `usage-interactive-detail`

Map Usage producers to the structured Detail contract while preserving Usage's
compact outer layout.

Implementation scope:

- Update `cmd/agentdeck/usage_stats_viewer.go` producer rules for Overview,
  Trend, Models, Clients, Providers, Cache, Coverage, and Activity as applicable.
- Omit optional zero cache/token components and redundant row values; preserve
  priced/unpriced, partial, warning, and error meaning.
- Reclaim Detail height immediately when the selected row has no card.
- Extend `cmd/agentdeck/usage_stats_viewer_test.go` and PTY tests for semantic
  colors, no-color, selection changes, empty Detail, resize, and geometry.

Acceptance criteria:

- Usage remains as compact and readable as the accepted baseline; no list or bar
  expands solely to fill a wide terminal.
- Selection changes the visible Detail content when supplementary data exists.
- Empty or redundant Detail never leaves a title or blank lower region.
- Warning and pricing completeness are not lost by zero/unknown filtering.

Development verification:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run 'TestUsageViewer'
```

On Darwin, also run the focused Usage PTY tests after the final task edit.

Verification level: L3 interactive terminal behavior.

Proposed commit boundary after Review PASS:
`fix: improve usage interactive detail`.

### `session-interactive-responsive-layout`

Apply the Detail language to Session and correct both width and height behavior
for root browser, nested viewer, and direct `session show --interactive` entry.

Implementation scope:

- Update `cmd/agentdeck/session_browser.go` bounded columns, compact two-line
  records, preview, dynamic complete-record viewport, and PageUp/PageDown step.
- Update `cmd/agentdeck/session_viewer.go` state from fixed page-limit assumptions
  to geometry-derived acquisition and viewport capacity with identity anchoring.
- Update `cmd/agentdeck/session_viewer_terminal.go` only where resize-driven
  anchored reload requires geometry input; preserve terminal ownership.
- Update `cmd/agentdeck/session_viewer_data.go` to emit structured Overview,
  Documents, Activity, and Tokens Detail fields with explicit semantic roles and
  domain-owned omission rules.
- Update Session unit, data, browser, CLI, and PTY tests, including the reported
  structured-detail readiness timeout if it remains reproducible.

Acceptance criteria:

- At 180 columns, Session records remain left aligned inside at most 120 visible
  cells; Last Activity is not placed at the physical terminal edge.
- 48-79-column items are complete two-line records with no space-between layout.
- Taller terminals display more complete rows and shorter terminals fewer; no
  nested section remains fixed at 20 rows.
- Detail absence returns all body lines to the list; stacked and side-by-side
  Detail never creates unexplained blank regions.
- Resize 60x18 -> 180x40 -> 80x24 preserves section and selected stable identity,
  keeps it visible, performs bounded reloads, and leaves no ghost cells.
- Browser Enter, detail Escape, root Escape/q, Ctrl-C, EOF, cancellation, errors,
  and state-lock release retain the existing lifecycle.

Development verification:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run 'Test(SessionViewer|RenderSessionViewer|SessionBrowser|RenderSessionBrowser|RunSessionViewer|SessionBrowserPTY)'
```

Verification level: L3 shared interactive terminal and bounded acquisition.

Proposed commit boundary after Review PASS:
`fix: make session interactive layout responsive`.

### `terminal-contract-and-acceptance`

Reconcile living documentation and prove the final combined behavior using the
compiled current binary.

Implementation scope:

- Reconcile `docs/specs/2026-08-06-terminal-rendering-design.md`,
  `docs/specs/2026-08-07-usage-interactive-viewer-design.md`, and
  `docs/specs/cli-manual.md` with delivered behavior; do not document an
  unimplemented draft.
- Update this plan, `docs/README.md`, and formal review records.
- Build one current binary and exercise ordinary Activity, Usage interactive,
  root Session browser, browser-to-detail return, and direct Session detail with
  synthetic isolated state and isolated copies of real local state.
- Record exact content identity, commands, terminal dimensions, color/no-color,
  resize path, and privacy/source-log observations.

Acceptance criteria:

- Every requirement in this plan maps to a unit, PTY, compiled-binary, or
  documentation oracle.
- Synthetic and isolated-real-state runs cover ordinary Activity plus both
  interactive viewers at narrow, standard, wide, short, and tall geometries.
- Real source logs remain byte-identical/read-only; credentials, credential key,
  private paths, commands, arguments, results, environment, prompts, and hidden
  content do not enter captured artifacts.
- Existing JSON results remain geometry independent and free of ANSI/cursor
  bytes.
- Required final verification and every review finding are closed.

Verification level: L4 release-candidate readiness after L3 acceptance.

Proposed commit boundary after Review PASS:
`docs: finalize terminal presentation remediation`.

## Task dependencies

```text
reset baseline and classify overlapping hunks
  |
  +-> session-show-activity-presentation reset review -------+
  |
  +-> interactive-detail-language reset review and freeze ---+--+
                                                               |
                                  +-> usage-interactive-detail reset review ---+
                                  +-> session-interactive-responsive-layout ---+--> terminal-contract-and-acceptance
```

Task 1 is independent, but it runs first to prove the reset process. Task 2 then
freezes the shared Detail contract. Tasks 3 and 4 are reviewed only after that
freeze so a later shared-renderer repair cannot invalidate their review evidence.
Execution remains serial: one task candidate, one complete review surface, one
repair set, one complete re-review, and one commit checkpoint at a time.

## Status

| Task | State | Dev | Review | Test | Acceptance |
| --- | --- | --- | --- | --- | --- |
| `session-show-activity-presentation` | Completed — Reset Round R3 Review PASS; CEv1 VERIFIED | [x] | [x] | [x] | N/A |
| `interactive-detail-language` | Completed — Full Reset Re-review R2 PASS; CEv1 VERIFIED | [x] | [x] | [x] | N/A |
| `usage-interactive-detail` | Completed — Full Reset Re-review R3 PASS; CEv1 VERIFIED | [x] | [x] | [x] | N/A |
| `session-interactive-responsive-layout` | In progress — post-PASS Full Reset Re-review R4 PASS; commit/CEv1 pending | [x] | [x] | [x] | N/A |
| `terminal-contract-and-acceptance` | Paused — Acceptance PASS; rebind/re-review required after Task 4 commit | [x] | [ ] | [ ] | [x] |

For Tasks 2-3, checked `Dev` means only that an imported development payload
exists; it does not certify correctness. For Task 4 it means only that a
complete uncommitted development candidate exists. Their previous Review and
Test checks remain reset. Task 1 is the exception recorded above: its fresh
Reset Round R3 review, targeted tests, signed commit tree, and CEv1 gate are
complete. For the first four tasks, `Test` means fresh targeted verification on
the exact candidate that receives the new legal PASS. For the final task,
`Test` includes the final aggregate local gate and `Acceptance` includes both
compiled-binary synthetic and isolated-real-state terminal validation.

## Reset review workflow

This workflow runs only in
`review/terminal-presentation-remediation-v2`'s isolated worktree. Its
"candidate" is the current task payload applied on the new legal branch, never
the safety-source `main` worktree. Step 7 means creating that task's complete
replacement commit on the review branch; it does not mean appending a
repair-only commit to any old Task 1-3 object. This isolation rule supersedes
conflicting wording below while the retained wording continues to define review
depth, finding closure, evidence, and commit inspection.

The reset applies to Tasks 1-4 and does not reuse the old PASS rounds:

1. **Baseline and ownership** - verify commits, dirty paths, task contract, test
   surface, and hunk ownership. Record the candidate content identity. Do not
   edit product code while establishing the baseline.
2. **Reset full review** - review the entire production, tests, documentation,
   dependency interaction, privacy, geometry, no-color, lifecycle, and failure
   surface owned by the task. This is a new Round R1 after reset, not a re-review
   of an old finding list. The reviewer may use old records only to construct
   adversarial cases.
3. **Repair all findings** - every in-scope finding is mandatory, including
   blocking, non-blocking, P0-P3, and nit findings. Repair production, tests, and
   contracts together where required. There is no "follow-up" bucket for known
   defects.
4. **Fresh targeted tests** - run the task's documented targeted verification on
   the repaired exact state and check the process exit code. A green command is
   development evidence, not a review verdict.
5. **Mandatory full re-review** - review the complete task again, not merely the
   repaired lines. Verify every prior finding and search for new findings. Any
   finding, regardless of severity, returns to step 3. Repeat Repair -> full
   Re-review until a new reset-era round records `Verdict: PASS` with no open
   findings.
6. **Gate update** - only after that PASS may the plan tick `Review`; tick `Test`
   only when the fresh targeted command is bound to the same candidate content.
   Query the configured CEv1 Task WorkUnit for that exact state and require
   `VERIFIED` before claiming the task complete.
7. **Commit checkpoint** - for Tasks 1-3, do not rewrite their existing
   implementation commits. Create one later task-scoped repair/review-status
   commit containing only newly required changes and replacement review records.
   If a task truly requires no product/test repair, its later commit contains the
   invalidation marker, new legal review record, plan/index status, and no
   fabricated code change. Task 4 receives its first implementation/review commit
   only after PASS. Inspect staged scope and the complete proposed English
   Conventional Commit message, include exactly
   `Co-Authored-By: Codex <noreply@openai.com>`, and verify commit tree, trailer,
   SSH signature, hook effects, and status. Commit still does not authorize push.

Existing review files must gain an explicit reset notice before any new round:

```text
## Review reset — 2026-08-11

All earlier PASS conclusions are invalid by user direction. Rounds above remain
historical audit data only and provide no current Review or Test gate.
```

New rounds start below that notice using `Reset Round R1`, `Repair after Reset
Round R1`, and `Reset Round R2` naming. A replacement PASS must enumerate the
complete reviewed scope, all open/closed findings, exact content identity, fresh
test commands with exit status, and residual risks. It cannot say only "old
findings remain closed."

Task execution order in the isolated review worktree is fixed:

1. Task 1 reset review loop and checkpoint commit;
2. Task 2 hunk classification, reset review loop, shared-contract freeze, and
   checkpoint commit;
3. Task 3 reset review loop against the frozen shared contract and checkpoint
   commit;
4. Task 4 reset full review of the current development candidate, repair loop,
   targeted PTY tests, and first task commit;
5. Task 5 development, normal full review/repair/re-review loop, acceptance, plan
   closure, and final task commit.

Task 5 follows the same standards, but because it has not been developed it starts
with Development before its first review.

An unrelated pre-existing failure is not waived: classify it, prove it is
unrelated, and keep release blocked while required verification is red. Expanding
into an unrelated repair still requires user authority. A failure caused by this
work or by stale assertions for the replaced terminal contract belongs to the
owning task and must be repaired before PASS.

## Local final verification

The reset invalidates prior task-test conclusions, but it does not authorize
repeated broad-suite runs. Each task runs its focused verification after its
final repair. After the final relevant edit, all five new legal reviews, and the
compiled-binary acceptance below, bind evidence to the exact final content state
and run the aggregate gate once:

```text
rtk test env -u VERSION GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod make release-verify
```

Do not run every component and then repeat it through `release-verify`. If the
aggregate fails, diagnose the smallest reproducer, reopen the owning task,
repair, run focused tests, perform another complete reset-era re-review, commit
the repair, invalidate stale content-bound evidence, and rerun the aggregate only
after the final repair. A failed aggregate can never be classified as
"non-blocking" for Plan or Release completion.

Before that aggregate, compiled-current-binary PTY acceptance must cover:

- synthetic isolated HOME/state with deterministic Usage, Session metadata,
  Documents, safe Activity, token/cost, partial/unpriced, empty, and warning data;
- isolated copies of real local state/session sources, never the live mutable
  database as a write target and never real credentials or credential keys;
- ordinary `session show --activity` at 48, 80, 100, 120, and 180 columns;
- Usage and Session interactive at 60x18, 80x24, 120x32, and 180x40;
- live resize, color/no-color, browser -> detail -> browser, direct detail,
  standalone Escape, q, Ctrl-C, EOF/error cleanup, and terminal restoration;
- source-log hashes before and after, captured artifacts mode `0600`, and removal
  of temporary acceptance data after evidence is recorded.

Probe the configured `completion-evidence/v1` capability once. Before each Task,
Plan, and Release completion claim, query the exact WorkUnit/content state. Record
new evidence idempotently and require `VERIFIED`; do not invoke the prohibited
`verification-before-completion` skill.

The final validation order is mandatory:

1. build the current candidate binary from the exact reviewed tree;
2. run synthetic isolated-HOME/state ordinary and PTY acceptance;
3. run isolated-copy real-state read-only validation and compare source hashes;
4. repair and re-review any issue found by either acceptance layer;
5. only on the final repaired/reviewed tree, run the single local
   `env -u VERSION ... make release-verify` aggregate;
6. query Task 5 and Plan CEv1 gates for the exact final state;
7. archive plan/reviews and create the final reviewed task commit;
8. verify the committed tree equals the accepted tree. If closure documentation
   changes behavior-bearing content or tests, rerun only the invalidated evidence.

## Plan closure

Plan closure occurs on the isolated review branch. Its closure commit is part of
final candidate `S`; no later documentation-only transplant is allowed.

After all five tasks have Dev, Review, Test, and applicable Acceptance gates:

1. move this plan to `docs/archive/plans/terminal-presentation-remediation.md`;
2. move `docs/reviews/terminal-presentation-remediation/` to the matching archive
   path;
3. set historical/retired frontmatter, add the archive reason to
   `docs/archive/README.md`, and remove the active plan row from `docs/README.md`;
4. include closure artifacts in the final task's reviewed atomic commit, or use a
   separate documentation commit only if the final task was already committed;
5. verify the plan is no longer advertised as active before delivery.

## Delivery workflow after Plan PASS

The following external actions are not implied by task completion. They are
authorized only when the user invokes the explicit full-flow instruction at the
end of this document.

### 1. Publish the exact review-branch SHA to `main` by fast-forward

- In the isolated worktree, require a clean
  `review/terminal-presentation-remediation-v2`, verify its complete task commit
  range from the recorded `origin/main` baseline, English messages, Codex
  trailers, SSH signatures, hook effects, and tree equality with the locally
  accepted and CEv1-verified candidate. Record its 40-character `HEAD` as `S`.
- Fetch immediately before delivery. Require the fetched `origin/main` to remain
  an ancestor of `S`, and require its identity to equal the baseline already
  reviewed for the candidate. Any remote drift is a stop condition: do not
  merge, rebase, force, or automatically replay the payload.
- Push the review branch directly to the remote main ref with a normal
  fast-forward refspec:

  ```text
  rtk git push origin review/terminal-presentation-remediation-v2:main
  ```

- Fetch once after the push and require
  `origin/main == review/terminal-presentation-remediation-v2 == S`. This is a
  ref update only; the commit and tree SHA must not change. Do not switch or
  reset local `main`, and do not perform a second cherry-pick, merge, squash,
  amend, rebase, or delivery commit.
- If branch protection rejects direct fast-forward push, stop and report the
  external blocker. Creating a PR or changing protection requires separate user
  direction because either may change the approved delivery path.

### 2. Dispatch same-SHA `release-preflight`

- Obtain the existing isolated-real-state completion-evidence URN bound to the
  exact pushed SHA.
- Dispatch `.github/workflows/release-preflight.yml` with the 40-character pushed
  SHA and that evidence ID.
- Wait for completion. Inspect the exact run, commit, L4 result, manifest, and
  uploaded commit-bound artifacts. A failure reopens the owning task; do not tag.

Example command shape, filled with verified values at execution time:

```text
rtk gh workflow run release-preflight.yml --ref main -f target_sha=<40-char-sha> -f real_state_evidence_id=<urn:ce:...>
rtk gh run list --workflow release-preflight.yml --commit <40-char-sha>
```

### 3. Publish the next RC prerelease

- Discover remote tags again and choose the next unused valid
  `v0.4.0-rc.<number>`; currently this is expected, not guaranteed, to be
  `v0.4.0-rc.4`.
- Prepare release notes with `Features`, `Improvements`, `Bug Fixes`, and `Tests`
  as applicable, including the exact commit and known limitations.
- Require successful same-SHA preflight evidence. Create an annotated tag through
  `make release-tag`, inspect its target and complete message, and push only that
  tag without force.
- Wait for `.github/workflows/release.yml`. Verify the GitHub Release is marked
  prerelease and contains arm64/amd64 archives plus checksums whose embedded
  version and commit match the tag.

Example command shapes:

```text
rtk test make release-tag TAG=<next-rc-tag> RELEASE_NOTES=<reviewed-notes-file>
rtk git show --show-signature --no-patch <next-rc-tag>
rtk git push origin <next-rc-tag>
rtk gh run list --workflow release.yml --commit <40-char-sha>
```

Never recreate, move, or replace an existing tag or published prerelease.

### 4. Complete Homebrew RC promotion

- Require the release workflow's Homebrew job to verify the RC formula and open
  the formula-specific PR against `kitdine/homebrew-tap` without changing the
  stable formula.
- Inspect the PR's `Formula/agentdeck-rc.rb` version, two archive URLs/checksums,
  completions, formula test, source branch, and checks. Merge only after all
  required checks pass and the formula matches the published RC assets exactly.
- If automatic PR creation failed after a valid GitHub prerelease, diagnose it;
  use the workflow's existing manual dispatch only for the same existing tag and
  never to recreate the GitHub Release.

### 5. Install and verify locally

- Inspect the currently installed stable/RC formulas before changing the local
  channel. Do not install stable and RC formulas together.
- After the verified Homebrew PR is merged, run `brew update`; install or upgrade
  `kitdine/tap/agentdeck-rc`, explicitly uninstalling the stable formula first
  only when the current local state requires a channel switch.
- Verify `agentdeck version` reports the exact RC tag and commit; verify arm64
  identity, formula test, bash/zsh/fish completion paths, and the three affected
  terminal surfaces against local read-only data.
- Stop for human visual acceptance. Do not infer stable-release authorization.
- If local acceptance fails, preserve state, record the reproducer, reopen the
  owning task, and require a new commit, push, preflight, and new RC number. Never
  replace the failed RC tag.

Homebrew uninstall removes Cellar-linked artifacts only and must not delete
`~/.agentdeck/` state. Rollback, if requested, is an explicit channel switch from
the RC formula back to the stable formula, not deletion of user state.

## Starting task in the isolated worktree

After the user separately confirms execution, begin with the baseline and drift
gate, create the fixed branch/worktree from the verified `origin/main`, and then
migrate Task 1. The first formal product phase is Task 1 Reset Round R1 inside
the review worktree. The safety-source local `main` remains read-only throughout.

## Authoritative new-session execution instruction

The user may paste the following after approving execution. This is the only
executable new-session instruction in this document. Every older instruction
block below is retained only as audit history and grants no authority.

```text
进入完整流程开发：严格执行 docs/plans/terminal-presentation-remediation.md，尤其是 `Isolation worktree topology — approved 2026-08-11`、`Review reset — 2026-08-11`、`Reset review workflow`、`Local final verification` 和 `Delivery workflow after Plan PASS`。

授权范围与顺序：
1. 先只读核对 local main、fresh origin/main、三个本地 task commit、Task 4 dirty candidate、共享 Detail 重叠 hunks、计划/索引和旧 review records；记录完整 SHA、path/hunk manifest 与 SHA-256 fingerprint。任何无法解释的 drift、既存 branch/worktree 或不安全 hunk 拆分都停止，禁止 reset、clean、stash-pop、amend、rebase、bulk rollback、force push 或混入无关工作。
2. 从核验后的 origin/main 创建固定分支 `review/terminal-presentation-remediation-v2` 和固定隔离 worktree `/private/tmp/agent-deck-terminal-presentation-remediation-v2`。原 local main worktree全程只读，既不清理也不作为最终交付分支。
3. 按 Task 1、Task 2、Task 3、Task 4 顺序迁移 payload。Task 1-3 只导出并应用旧 commit 的 task-owned 内容，不直接沿用旧 commit 为最终提交；Task 4 仅应用 fingerprint 匹配且已完成归属的 dirty payload。旧 PASS/Test 全部无效，只作为审计历史。
4. 每个 task 在新分支严格执行：完整任务评审 -> 修复全部 findings -> fresh targeted tests 并核对 exit code -> 完整任务复评。所有 blocking、non-blocking、P0-P3 和 nit 必须全部关闭；发现任何新问题都重复修复和完整复评，直至新的合法 `Verdict: PASS`。
5. Task 2 先合入其旧 payload 与 dirty shared-Detail hunks，完成合法评审并冻结共享 Detail contract；Task 3 和 Task 4 只能在该冻结提交上迁移和评审。Task 4 视为 Dev 完成但从未合法评审。
6. 每个 Task 1-4 PASS 后，在 review branch 创建一个包含迁移 payload、修复、reset-era review 和状态的全新 task-scoped atomic commit。所有 commit 必须含 `Co-Authored-By: Codex <noreply@openai.com>`，并核验完整 message、SSH signature、tree、staged scope、hook effects、review worktree clean 和 safety-source fingerprints unchanged。中途不推送。
7. Task 1-4 全部新 PASS 后开发并评审 Task 5。使用当前编译二进制完成 synthetic isolated-HOME/state 普通输出与 PTY 验收、隔离真实本地数据只读验证、源数据 hash 前后对比；任何问题都回到所属 task 修复、fresh tests、完整复评和新提交。
8. 在最终修复且评审通过的精确树上只运行一次 `env -u VERSION GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod make release-verify`。失败必须诊断最小复现并重新进入所属 task，不能跳过、降级为 non-blocking 或靠增加超时掩盖。
9. 全部新 Task/Plan CEv1 gate VERIFIED 后在 review branch 归档 plan/reviews并提交。记录该精确 40 字符 HEAD 为 `S`，核验 committed tree 等于验收树。
10. 推送前 fresh fetch；仅当 origin/main 仍为已评审基线且可 fast-forward 到 `S` 时，直接执行 `git push origin review/terminal-presentation-remediation-v2:main`。推送后必须 `origin/main == review branch == S`。禁止二次 cherry-pick、merge、squash、amend、rebase、delivery commit、force push，也不要 reset/switch local main。保护分支拒绝时停止，不自行改走 PR。
11. 只针对精确 `S` 和有效 isolated-real-state evidence ID dispatch `release-preflight`；等待并核验同 SHA run、manifest 和 artifacts。失败不得打 tag，必须回到修复流程并形成新 SHA。
12. 同 SHA preflight 成功后，重新发现下一个未占用的 `v0.4.0-rc.N`，评审 release notes，创建并推送 annotated RC tag，核验 GitHub prerelease、arm64/amd64 archives、checksums、version 和 commit identity；禁止改写已有 tag/release。
13. 等待并核验 `kitdine/homebrew-tap` 的 `agentdeck-rc` PR。formula、tag、URLs、checksums、completions 和 checks 全部一致后才合并，禁止修改 stable formula。
14. Homebrew RC PR 合并后更新并切换本机到 `kitdine/tap/agentdeck-rc`，核验精确 RC version/commit、arm64、formula test、bash/zsh/fish completions，并用真实本地只读数据视觉验收 `session show --activity`、`usage stats --interactive`、`session --interactive`。完成视觉验收后停止，不授权 stable release。

全程遵守 AGENTS.md、RTK、CEv1、dirty-worktree、权限和证据复用规则，并核对每次工具调用 exit code。无关故障必须证明无关但 required gate 仍保持 red；权限、secret、保护分支、无法安全拆分 payload 或外部状态阻塞时停止并报告，不自行扩大授权。
```

## Exact commit semantics for the isolated branch

At every Task 1-4 checkpoint, create one new atomic commit on the review branch
containing that task's migrated implementation, all repairs, replacement review
record, and task-local status updates. The old Task 1-3 object IDs remain
immutable sources but are not ancestors of final `S`. If a task requires no
product/test repair, its new commit still carries the migrated payload,
invalidation marker, legal review record, and status; do not fabricate a code
change. Inspect staged scope and the complete English Conventional Commit
message, require `Co-Authored-By: Codex <noreply@openai.com>`, and verify tree,
trailer, SSH signature, hook effects, clean review worktree, and unchanged
safety-source fingerprints. No intermediate commit authorizes a push.

This section is later and more specific than the retained Reset-workflow step 7;
it replaces that step's obsolete repair-only wording.

## Superseded starting and pre-isolation execution instructions

Everything below this heading is audit history. It assumes execution on local
`main` or predates the isolated worktree decision and must not be used as an
instruction or authorization source.

Start with:

```text
重新进入评审：`terminal-presentation-remediation` / `session-show-activity-presentation`
```

Read `AGENTS.md`, this plan, the two terminal design specs, the three local task
commits, all existing review records, the current Task 4 dirty diff,
`docs/README.md` workflow conventions, and relevant tests. First verify the reset
baseline and append invalidation notices; then begin Task 1 Reset Round R1. Do not
bulk-revert the draft, rewrite history, or reuse any old PASS/Test conclusion.
Tick a gate only on new reset-era evidence and finish the full repair/re-review
loop before committing or moving to the next task.

## Authoritative new-session instruction after review reset

The user may paste the following after approving this revised plan. This is the
only executable new-session instruction in this document:

```text
进入完整流程开发：严格执行 docs/plans/terminal-presentation-remediation.md，尤其是 `Review reset — 2026-08-11`、`Reset review workflow`、`Local final verification` 和 `Delivery workflow after Plan PASS`。

授权范围与顺序：
1. 先只读核对 main/origin、三个本地未推送 task commit、Task 4 dirty candidate、共享 Detail 重叠 hunks 和旧 review records；保留现有实现，禁止 reset、amend、rebase、bulk rollback、force push 或混入无关工作。
2. Task 1-3 的全部旧 PASS/Test 结论无效，只能作为审计历史。按 Task 1 -> Task 2 -> Task 3 -> Task 4 顺序，对每个完整 task 从新的 Reset Round R1 开始评审；不得只检查旧 findings。
3. 每个 task 严格执行：完整任务评审 -> 修复全部 findings -> fresh targeted tests 并核对 exit code -> 完整任务复评。所有 blocking、non-blocking、P0-P3 和 nit 必须全部关闭；发现任何新问题都重复修复和完整复评，直至新的合法 Verdict: PASS。
4. Task 2 必须先完成重叠 hunk 归属、合法评审并冻结共享 Detail contract；Task 3 和 Task 4 只能在冻结状态上评审。Task 4 视为 Dev 完成但从未合法评审。
5. Task 1-3 不改写已有 commit；新的修复、reset-era review 和状态更新形成后续 task-scoped commit。Task 4 在新 PASS 后形成首个 task commit。所有 commit 必须含 `Co-Authored-By: Codex <noreply@openai.com>`，并核验完整 message、SSH signature、tree、staged scope 和 hook effects。提交不授权推送。
6. Task 1-4 全部新 PASS 后开发并评审 Task 5。使用当前编译二进制完成 synthetic isolated-HOME/state 普通输出与 PTY 验收、隔离真实本地数据只读验证、源数据 hash 前后对比；任何问题都回到所属 task 修复、fresh tests、完整复评和提交。
7. 在最终修复且评审通过的精确树上只运行一次 `env -u VERSION GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod make release-verify`。失败必须诊断最小复现并重新进入所属 task，不能跳过、降级为 non-blocking 或靠增加超时掩盖。
8. 全部新 Task/Plan CEv1 gate VERIFIED 后归档 plan/reviews，核验 committed tree 等于验收树，再无 force 推送 main。
9. 推送后，只针对精确 pushed SHA 和有效 isolated-real-state evidence ID dispatch `release-preflight`；等待并核验同 SHA run、manifest 和 artifacts。失败不得打 tag，必须回到修复流程。
10. 同 SHA preflight 成功后，重新发现下一个未占用的 `v0.4.0-rc.N`，评审 release notes，创建并推送 annotated RC tag，核验 GitHub prerelease、arm64/amd64 archives、checksums、version 和 commit identity；禁止改写已有 tag/release。
11. 等待并核验 `kitdine/homebrew-tap` 的 `agentdeck-rc` PR。formula、tag、URLs、checksums、completions 和 checks 全部一致后才合并，禁止修改 stable formula。
12. Homebrew RC PR 合并后，在本机安全切换或升级 `kitdine/tap/agentdeck-rc`，核验 version、commit、architecture、`brew test`、bash/zsh/fish completions，并用本机只读真实数据验收 `session show --activity`、`usage stats --interactive`、`session --interactive`。完成视觉验收后停止，不授权 stable release。

全程遵守 AGENTS.md、RTK、CEv1、dirty-worktree、权限和证据复用规则。每次工具调用检查 exit_code。若遇到无关故障、权限/secret/保护分支阻塞、无法安全拆分重叠 hunk 或需要扩大范围，停止并明确报告，不得绕过。
```

## Superseded pre-reset new-session instruction

The block below is retained only as audit history. It assumed the rejected Task
1-3 PASS conclusions and must not be executed or used as authorization.

The user may paste the following as the first instruction in a new session:

```text
严格执行 docs/plans/terminal-presentation-remediation.md 的全部内容。

本次明确授权在该计划定义的范围内依次完成：
1. 拆解并完成全部五个 task；从当前 dirty draft 逐 hunk 纠偏，禁止整轮回滚。
2. 每个 task 严格执行 开发 -> 评审 -> 修复 -> 复评 循环；所有范围内的 blocking、non-blocking、P0-P3 和 nit finding 必须全部关闭，直至正式 Review PASS。
3. 每个 task Review PASS 后按计划的原子边界提交；每个 commit 必须带 Co-Authored-By: Codex <noreply@openai.com>，并检查完整 message、SSH signature、tree 和 hook effects。提交不合并 task。
4. 完成编译当前二进制的 synthetic isolated-HOME/state PTY 验收、隔离真实本地数据只读验证，以及最终一次 env -u VERSION 的 make release-verify；任何失败都必须诊断、修复、复评并重新取得有效证据，不能跳过或靠增加超时掩盖。
5. 全部 Task/Plan gate VERIFIED 后，归档 plan/reviews，检查 main/origin/commit range，然后授权无 force 推送 main。
6. 推送成功后，授权针对精确 pushed SHA 和有效 isolated-real-state evidence ID dispatch 并等待 release-preflight；失败不得打 tag。
7. preflight 同 SHA 成功后，授权选择当时下一个未占用的 v0.4.0-rc.N，生成并评审 release notes，创建和推送 annotated RC tag，等待并核验 GitHub prerelease、archives、checksums 和 commit identity；禁止改写已有 tag/release。
8. 授权等待并核验 kitdine/homebrew-tap 的 agentdeck-rc PR；checks 与 formula/tag/assets/checksums 全部一致后合并，禁止修改 stable formula。
9. Homebrew RC PR 合并后，授权在本机安全切换/升级 kitdine/tap/agentdeck-rc，核验版本、commit、架构、brew test、bash/zsh/fish completions，并使用本机只读真实数据验收 session show --activity、usage stats --interactive、session --interactive。
10. 本机视觉验收后停止；本指令不授权 stable release、force push、历史改写、删除用户状态或修改无关工作。

全程遵守 AGENTS.md、RTK、CEv1、dirty-worktree、权限和证据复用规则。每次工具调用都检查 exit_code；不要把压缩输出或超时当成功。若遇到无关故障、权限/secret/保护分支阻塞或需要扩大范围，停止并明确报告，不得绕过。
```
