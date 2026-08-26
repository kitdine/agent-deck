---
status: active
topic: usage-attribution-precision
subject: tasks.md
---

# Review log — usage-attribution-precision / tasks.md

## Round 1 — 2026-08-26

- Reviewed state: HEAD `5f2189550348a5a3f65fca42d6b92e8d07b2b5ac` plus the
  uncommitted document blob `46877d60171e1dd584743174f991626170360011`
  (`requirements.md` `42bc63dc0d7890d2f2ffd1be57162fa0842dd46f`,
  `architecture.md` `fa92fc1d2b9d220855c69dd6b9b1ff97b380a964` reviewed in the
  same pass).
- Reviewer: claude-code, independently reviewing the design authored by
  `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal document Review under `development-workflow`. Each task's
  `Files` list was checked for completeness against the call sites that
  actually own the behavior the task changes, and each stated prerequisite was
  resolved against live repository state.
- Scope: the three-task decomposition, per-task file boundaries and
  dependencies, the prerequisite on the upstream switch topic, and the
  Documents/Tasks status matrices.
- Findings:
  - **[P1] T1-F1 — the file that owns the attribution counters and the
    existing quality vocabulary appears in no task.** `internal/usage/
    presentation.go` is absent from every `Files` list (tasks 1-3,
    `tasks.md:28,39-40,50-51`), yet it owns four things this topic changes:
    the `Counts` map initialization carrying the `"historical"` key (`:732`),
    the per-event counter increment `b.summary.Counts[attribution.quality]++`
    (`:740`), the warning emission `attribution.quality + " attribution"`
    (`:741-743`) that produces the very strings `architecture.md` lists as
    contract-affected, and the `determinable` / `inferred` / `unattributed`
    mapping (`:361-367`) that `requirements.md:110-111` describes as this
    topic's output. `internal/desktop/` and `desktop/fixtures/v1/*` are
    likewise absent although the fixtures encode those bucket names as a
    checked-in producer-output contract. A task whose `Files` list omits the
    code that must change is not a scoped boundary, and the omission has a
    known failure mode: the same class of gap closed a review as H3-F1 in
    `switch-effectiveness-boundary` earlier in this session, when a producer
    value moved and the canonical fixtures were not regenerated with it.
    Whichever task takes the rename must also name the regeneration step,
    since these fixtures are regenerated, never hand-edited.
  - **[P1] T1-F2 — task 2 is asked to implement a shape no document
    defines, and the Documents matrix denies that shape changes.** Task 2 must
    "replace `historical` with `unattributed` split into the before-adoption
    and coverage-gap states" and "Reconcile the JSON `counts` keys"
    (`tasks.md:33-36`), but `architecture.md`'s Contract impact specifies only
    the rename, never the split's observable form (A1-F5), and
    `requirements.md:110-111` describes three buckets where the architecture
    table makes four states reportable. The implementer therefore has three
    different pictures and no contract to satisfy. Compounding it, the
    Documents matrix note asserts that `usage summary` "keeps its existing text
    and JSON shape while the values and their labels change"
    (`tasks.md:64-66`) — which cannot hold if `counts` gains keys for the
    split, and which is the stated justification for the `ux/` row being
    `n/a`. Either the split is not surfaced in `counts` and the architecture
    must say where it is, or it is, and this note is wrong.
  - **[P2] T1-F3 — task 1's stated reason for reading `routes.go` does not
    match what the design needs from it.** Task 1 lists
    `internal/usage/routes.go` in `Files` while its prerequisite paragraph says
    "This task reads the effective-route stream only" (`tasks.md:26-28`). Under
    A1-F1 the open question is whether the promotion to `exact` is derived at
    read time or written at insert time; only the second requires touching
    `routes.go`, and it is the option `requirements.md:63-64` forbids. The file
    list therefore silently pre-commits to a mechanism the architecture has not
    chosen. Resolve A1-F1 first, then make this list follow from it.
- Verified, not findings:
  - Task 1's prerequisite — "`switch-effectiveness-boundary` tasks 1 and 3 have
    Review PASS" (`tasks.md:23`) — is satisfied as of HEAD `5f21895`: that
    topic's matrix records tasks 1-3 as `[x] [x]`, with the implementations
    committed at `8703fed` and `7db5618` and PASS rounds recorded under its
    `reviews/`.
  - The strict sequencing and the dependency edges (task 2 on 1, task 3 on 1
    and 2) are consistent with the resolution order the architecture defines.
  - Task 2 correctly carries both `docs/specs/cli-manual.md` and
    `docs/specs/cli-design.md`, matching `architecture.md:99-100`'s requirement
    that the contract documents be reconciled in the same task that changes the
    values.
- Evidence: in the reviewed blob `46877d60`, `tasks.md` contains zero
  occurrences of `presentation.go`, `desktop`, or `fixtures`. Searching the
  working file after this round now matches `desktop` and `fixtures` once each,
  but only inside the Round 1 narrative this review itself added; the finding is
  bound to the reviewed blob, not to the post-review file.
  `internal/usage/presentation.go:732,740,741,361-367`
  establish that file's ownership of the counters, warnings, and bucket names.
  `desktop/fixtures/v1/snapshot-complete.json` contains 18 occurrences each of
  `determinable`, `inferred`, and `unattributed`.
- Completion gate: NOT_REQUIRED — a document review round records no
  completion evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — 2026-08-26

- T1-F1 closed: tasks 2 and 3 now name `presentation.go`, desktop producer
  tests, canonical fixtures, and affected macOS presentation/Widget tests, with
  the producer regeneration command instead of hand editing.
- T1-F2 closed: architecture now owns a six-key reason object and explicit cost
  fields; task 3 implements that shape, and the Documents note no longer claims
  that JSON shape is unchanged.
- T1-F3 closed: task 1 removes `routes.go` from its file list and states that the
  persisted route-quality writer remains unchanged under read-time derivation.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 2 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus committed
  document blob `1651a86c465f42b049bd75992726c873ea247862` (`requirements.md`
  `0abe112cccb025e40e69ac6c531e93a3621f10be`, `architecture.md`
  `2174860af3646359e86f7cfaa8ace1611dbd1b14` re-reviewed in the same pass).
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; each `Files` list
  checked against the call sites that own the behavior its task changes, and
  each prerequisite resolved against live repository state.
- Scope: T1-F1 through T1-F3, and any newly blocking repair regression.
- Findings:
  - **T1-F1 — CLOSED.** Task 2 now names `internal/usage/presentation.go`,
    `internal/desktop/fixtures_test.go`, `internal/desktop/desktop_test.go`,
    `desktop/fixtures/v1/*.json`, and the affected macOS presentation/Widget
    tests, and carries the producer regeneration command with an explicit
    "never hand-edit them". Task 3 repeats the producer scope for the
    observability surface.
  - **T1-F2 — CLOSED.** Task 3 implements the six-key `attribution_reasons`
    object that `architecture.md` now defines, and the Documents-matrix note no
    longer claims the JSON shape is unchanged; it states that `usage summary`
    text/JSON and the desktop payloads do change, and justifies `ux/` as `n/a`
    on the ground that no interaction design is required, which is a different
    and sound claim.
  - **T1-F3 — CLOSED.** Task 1 no longer lists `internal/usage/routes.go`, and
    its prerequisite states that the persisted route-quality writer is
    unchanged, matching the read-time derivation the architecture chose. Only
    `routes_test.go` remains, for parity coverage.
  - **[P1] T2-F1 — no task owns the effect test A2-F1 requires.** Task 1 makes
    both read paths follow effective session state, and task 2 promotes
    known-provider routes to `exact`; neither is scoped to decide whether a
    positioned `ConfigChange` route recorded a transition that actually took
    effect. As written, task 2's "promote known-provider effective routes to
    `exact`" (`tasks.md:35`) is precisely the rule A2-F1 rejects, so following
    this decomposition produces the defect. The owning task also needs the
    prior-effective-state inputs named in its scope — the session's earlier
    route and the provider timeline at session start — even though both are
    already read, because the rule is what is missing, not the data. Once
    `architecture.md` states the rule, this file must place it in a task and
    add the acceptance coverage: the per-group classification recorded in
    `reviews/architecture.md` Round 2 is the reference fixture, and the
    regression must prove that the four removal-from-keyed rows stay
    `estimated` while the other twenty-seven `ConfigChange` rows and all 135
    `SessionStart` rows reach `exact`.
- Verified, not findings:
  - Task 1's prerequisite that `switch-effectiveness-boundary` tasks 1 and 3
    have Review PASS remains satisfied; those implementations are committed at
    `8703fed` and `7db5618`.
  - Task 3's addition of `internal/store/providers.go` is consistent with the
    client-wide timeline existence check `architecture.md` requires for the
    `before_adoption` / `coverage_gap` split; `LoadProviderTimeline` and
    `SnapshotAt` live there.
  - The dependency edges and strict sequencing still match the resolution order.
- Evidence: `tasks.md:29-30,43-47,61-65` are the repaired `Files` lists.
  `internal/store/providers.go:122,178` are the timeline entry points task 3
  will need. The A2-F1 classification and totals are in
  `reviews/architecture.md` Round 2.
- Completion gate: NOT_REQUIRED — a document review round records no completion
  evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — Round 2 — 2026-08-26

- T2-F1 closed: task 2 owns the legacy-effect classifier, its prior-state
  inputs, read-side route metadata changes, and the complete six-group reference
  regression. `recordSessionRouteConn` remains outside the change boundary.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 3 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus the
  uncommitted document blob `9f08908761ae0cc63a593190a5cbdbe94aa17fff`
  (`requirements.md` `647c0963d5a012b0bb513645042747461d921080`,
  `architecture.md` `6a43f5f3b4cf6e71042390265e542a647919dda2` re-reviewed in
  the same pass); subject digest
  `4225f43c57838ccc9b58ca754267a0221415d0f0e5c4e9cdeff96943dc6f6498`.
- Reviewer: claude-code, independently re-reviewing the Round 2 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; the Round 2 finding was
  checked against the repaired decomposition, and each `Files` list re-checked
  against the call sites its task must change.
- Scope: T2-F1, and any newly blocking repair regression.
- Findings:
  - **T2-F1 — CLOSED.** Task 2 now owns the rule and states it as the
    distinction Round 2 asked for: "Derive quality at read time from route
    effect, not provider presence." It names the three inputs the classifier
    needs — the existing `hook_event`, the preceding session route, and the
    provider timeline at session start — in both resolver paths, and states the
    promotion outcomes for each case. The acceptance obligation is explicit and
    checkable: the six-group table in `reviews/architecture.md` Round 2 is named
    as the fixture, a focused regression must reproduce the complete
    classification with its counts, and it must additionally prove that a
    blanket pre-cutoff exclusion would be wrong — which keeps the rejected
    coarse rule from reappearing as an implementation shortcut.
- Verified, not findings:
  - The task 1 / task 2 boundary stays coherent. Task 1 still reads the
    effective-route stream only, keeps the persisted writer unchanged, and does
    not list `routes.go`; task 2 takes the read-side change, and its `Files`
    entry carries the guard "(never `recordSessionRouteConn`)", which matches
    the architecture's read-side-only scope and the requirements Non-Goal.
  - Task 3 keeps `internal/store/providers.go` for the timeline existence check
    behind the `before_adoption` / `coverage_gap` split; `LoadProviderTimeline`
    and `SnapshotAt` are there (`internal/store/providers.go:122,178`).
  - The dependency edges, strict sequencing, and the `ux/` `n/a` justification
    are unchanged and still hold.
- Evidence: the repaired task 2 scope, acceptance paragraph, and `Files` list
  are at `tasks.md:33-60`; task 1's boundary is at `:13-31`. The four-row rule
  this task implements, and the measured classification it must reproduce, are
  in `reviews/architecture.md` Round 2 and were re-verified in Round 3 of that
  record, including the fifteen rows Round 2 left unresolved.
- Completion gate: VERIFIED — CEv1 gate `usage-attribution-precision:tasks.md`
  records this PASS against exact content state
  `4225f43c57838ccc9b58ca754267a0221415d0f0e5c4e9cdeff96943dc6f6498`.
- Verdict: PASS
