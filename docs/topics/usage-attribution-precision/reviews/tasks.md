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
