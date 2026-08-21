---
status: active
topic: work-signals
subject: docs/topics/work-signals/requirements.md
---

# Review log — work-signals / requirements.md

## Round 1 — 2026-08-20
- Reviewed state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`, `requirements.md`
  blob `61dcc198f4b5f750a87ca15d69012ec2ec6d4f06` (untracked)
- Reviewer: claude-code
- Method: design/contract review. Every premise the document states about
  existing code was located in the repository rather than accepted from the
  document: `internal/activity/activity.go` package and `Detail` doc comments,
  the `usage_tool_calls` v13 migration in `internal/store/migrations.go:88-94`,
  `activity.Summary.ByTool`, `internal/desktop/desktop.go` period and client
  scoping, `apps/macos/AgentDeckShared/DesktopWire.swift` version guard and
  absent-family decoding, `apps/macos/AgentDeckApp/MenuBarPanelViews.swift`
  `WorkSignalPlaceholder`, and the prototype's `PENDING_CAPTURE` fixture. The
  document was also read against its own siblings — `architecture.md`,
  `tasks.md`, `ux/cli-work-signals.md`, `docs/topics/desktop-app/ux/menubar.md`,
  and `docs/README.md` — for internal contradiction.
- Scope: `docs/topics/work-signals/requirements.md` only. Findings that belong to
  a sibling document are attributed below and are not this document's to close.
- Findings:
  - [P1] Goal 2 authorizes "decide against a specific metric explicitly and
    remove it from the surface" with no named decider, no recorded-decision
    requirement, and no user-approval gate. The surface it would remove from
    (`../desktop-app/ux/menubar.md` and the prototype) is owned by a sibling
    topic, and Non-goals forbids changing the prototype, so exercising the clause
    produces a shipped surface that diverges from the specimen. This document's
    own history section (lines 27-30) records that a capability was cut from
    `v0.5.0` without asking on 2026-08-18 and that the cut was reversed; Goal 2
    re-opens that exact path as policy. `architecture.md` Decisions 1 and 4
    currently derive all five Workflow values, so the clause is latent rather
    than active. This is also the review question the dispatch record names
    (`边界是否已决定，有无 TBD`). -> Closed by the repair round below.
  - [P2] Goal 5's consequence clause states the wrong compatibility direction:
    "an older host decodes a payload lacking the new families as
    `available: false`". A host built before this topic carries no `work_signals`
    key in its Codable model and renders the hardcoded `WorkSignalPlaceholder`;
    it cannot decode an absent family as `available: false`. The property that
    holds — and that Acceptance item 6 and `architecture.md` Decision 5 both
    state correctly — is that a host with the new decoding reads a payload
    predating the families as `available: false`, while an older host ignores
    unknown keys entirely. `wire_version` must stay unchanged because
    `DesktopWire.swift:63` guards on `==`, not `>=`; that part of the goal is
    right. -> Closed by the repair round below.
- Attributed elsewhere, not this document's findings:
  - `tasks.md` line 164 repeats the same inverted compatibility statement ("a
    host built before it decodes a newer payload as `available: false`"). Owner:
    `ad-ws-doc-tasks-design`.
  - `ux/cli-work-signals.md` line 44 asserts `--period` and `--client` are
    "exactly the menu-bar panel's two filters", but the CLI accepts seven period
    values while `internal/desktop/desktop.go:480-487` emits only `today`, `7d`,
    and `30d`. The panel's set is a subset, so this document's Acceptance item 1
    reproducibility claim still holds; the "exactly" wording and the derivation's
    period coverage belong to `ad-ws-doc-ux-cli-design` and `ad-ws-doc-arch-design`.
- Non-findings, recorded so a later round does not re-raise them:
  - Every other topic's `requirements.md` carries a `Surfaces and contracts`
    section and this one does not. Goal 7 names both surfaces, Non-goals excludes
    the Widget, and `tasks.md` carries the split, so nothing is undecided. Not a
    defect.
  - Acceptance item 3 (unavailable, not zero) and `architecture.md` Decision 5's
    always-four activity kinds with `share: 0` are consistent: a kind with no
    work is empty, which `../desktop-app/ux/menubar.md:449` already distinguishes
    from unavailable.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0 (the project's document-set
  audit; it is the only checker shipped for this target class). No product test
  run: the review is read-only over an unimplemented design and no content state
  changed.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-20
- Repaired state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`,
  `requirements.md` blob `ad81b6a7d2704edc04961a282ab7427890639f03` (untracked)
- Repairer: claude-code
- Scope: the two open Round 1 findings, in `requirements.md` only. No sibling
  document was touched; the attributed findings in `tasks.md` and
  `ux/cli-work-signals.md` remain open against their own tasks.
- Findings closed:
  - [P1] Goal 2's unqualified removal clause -> replaced. The goal now states
    that all five Workflow values are derivable and cites
    `architecture.md` Decisions 1 and 4, then gates removal explicitly: it is a
    scope reduction of a `v0.5.0` feature and requires user approval, a recorded
    decision in `tasks.md`, and a change to the document owning the surface, and
    is never an implementation task's own call. The clause now points back at the
    "It is not a deferral" section, so the gate and the history that produced it
    read together.
  - [P2] Goal 5's inverted compatibility direction -> corrected to match
    Acceptance item 6 and `architecture.md` Decision 5: a host carrying the new
    decoding reads a pre-families payload as `available: false`, while a host
    built before this topic ignores the unknown families and keeps rendering the
    shipped `Not captured yet` form. The reason `wire_version` must not move was
    added, since `DesktopWire.swift:63` guards it with `==` and a raised version
    makes an older host reject the whole snapshot rather than degrade.
- Not changed, and why: Non-goals, the Acceptance boundary, and the "What is
  already there" table were already consistent with both corrections, so nothing
  else needed to move. No task, matrix cell, or topic stage changed — a repair
  round changes neither.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design; no product behavior is in scope.
- Dispatch: Beads `ad-ws-doc-req-design` moved `in_progress` -> `in_review`
  in the same action, with the disposition recorded as a comment. The
  `round-1` label set by the Round 1 REOPEN is retained and not reset.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — independent re-review — 2026-08-20
- Reviewed state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`,
  `requirements.md` blob `ad81b6a7d2704edc04961a282ab7427890639f03` (untracked) —
  the exact state the Round 1 repair recorded, verified by `git hash-object`
  rather than assumed.
- Re-reviewer: claude-code
- Method: the two closed findings were re-judged against the repaired text, and
  every premise the repair rests on was located in the repository independently
  rather than accepted from the repair note. Sibling documents were re-read for
  regressions the repair could have introduced and for whether the attributed
  findings were touched.
- Findings re-judged:
  - [P1] Goal 2's unqualified removal clause — **closed, confirmed.** The goal
    now asserts all five Workflow values are derivable and gates removal behind
    user approval, a recorded decision in `tasks.md`, and a change to the
    document owning the surface, explicitly denying an implementation task the
    call. The assertion was verified rather than taken on trust: `files_touched`
    and `top_file_base_name`/`top_file_count` follow from Decision 1's
    `path_digest` and `base_name`; `edits_per_session` and `iteration_depth`
    follow from `usage_work_signals.write_calls` and its one-row-per-group
    shape; `first_edit_seconds` follows from that table's `started_at` against
    the first `tool_kind = 'edit'` row, with `usage_events.event_at` available as
    the session origin. The clause also now points back at the "It is not a
    deferral" section, so the gate and the 2026-08-18 history that motivated it
    read together — which is what makes the gate hard to erode later.
  - [P2] Goal 5's inverted compatibility direction — **closed, confirmed.** Both
    halves of the corrected statement were verified against the code, not against
    the repair note. `apps/macos/AgentDeckShared/DesktopWire.swift:63` guards
    `data.wireVersion == DesktopSnapshotV1.wireVersion` — an exact equality, so a
    raised version makes an older host throw `unsupportedWireVersion` and reject
    the whole snapshot rather than degrade, exactly as the goal now states.
    `apps/macos/AgentDeckApp/MenuBarPanelViews.swift:442` shows
    `WorkSignalPlaceholder` taking only `title`, `symbol`, and `tint` and holding
    no data field, confirming that an older host cannot represent
    `available: false` and simply keeps rendering the shipped uncaptured form.
- Repair boundary: **respected.** Both findings attributed elsewhere in Round 1
  are still present, unmodified, in their owning documents — `tasks.md`'s
  inverted compatibility sentence and `ux/cli-work-signals.md`'s "exactly the
  menu-bar panel's two filters". A repair that had silently corrected them would
  have closed findings against tasks no reviewer had judged.
- New finding, this document:
  - [P3] Goal 2 cites "Decisions 1 and 4" for the derivability of all five
    values, but `iteration_depth`'s denominator and `first_edit_seconds`' origin
    both rest on Decision 2's definition of the classification unit — a turn for
    Codex, a tool-call group for Claude. The citation is incomplete rather than
    wrong: the values are derivable, and no decision changes. Recorded, not
    blocking; fold Decision 2 into the citation whenever this document is next
    edited.
- New finding, attributed elsewhere and **not** this document's to close:
  - [P1] `architecture.md` Decision 2 contains a contradiction that would make
    `conversation` systematically unreachable for Claude. Rule 4 classifies "the
    turn contains no tool call at all" as `conversation`, but the Claude grouping
    unit is defined as "the maximal run of tool calls between two consecutive
    assistant messages" — a run that is empty when a turn made no tool call, so
    no group exists to carry the classification. The Activity module requires all
    four kinds to always render and their shares to sum, so a client that can
    never produce one of them reports a distorted mix as if it were a fact about
    the user's work. Owner: `ad-ws-doc-arch-design`. This does not block this
    document: Goal 1 states the requirement to classify into four kinds, which is
    correct; how the unit is defined so that it can is the architecture's
    question.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **PASS**

## Round 2 — repair — 2026-08-20
- Repaired state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`,
  `requirements.md` blob `678041252913870a5a368b187df78b4fe0ed08ba` (untracked),
  from the re-reviewed `ad81b6a7d2704edc04961a282ab7427890639f03`.
- Repairer: claude-code
- Scope: the one P3 Round 2 recorded against this document. No sibling document
  was touched; the P1 attributed to `architecture.md` remains open against
  `ad-ws-doc-arch-design`.
- Finding closed:
  - [P3] Goal 2's incomplete citation -> corrected to Decisions 1, 2, and 4, each
    named for what it supplies: Decision 1 the file identity, Decision 2 the
    classification unit that `iteration_depth` divides by and that
    `first_edit_seconds` measures from, and Decision 4 the storage for both. The
    reader who follows the citation now reaches everything the derivability claim
    rests on.
- **Effect on the Round 2 PASS.** The content state changed, so this is stated
  rather than glossed: the edit adds two decision references and one clause
  naming what each supplies, and alters no assertion, no gate, and no decision.
  It closes a finding Round 2 itself raised and judged non-blocking, so the PASS
  verdict holds for this state on the same grounds it was given — there is no new
  content for a re-review to judge. Had the repair changed a claim rather than a
  citation, the cell would have been unticked instead.
- Not changed, and why: the removal gate, the five-value assertion, and Goal 5
  were already correct and are untouched. No matrix cell and no topic stage
  changed — a repair round changes neither.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a citation correction in an
  unimplemented design.
- Verdict: PASS retained — the document's only open findings now belong to other
  documents.

