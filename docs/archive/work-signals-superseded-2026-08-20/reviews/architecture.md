---
status: active
topic: work-signals
subject: docs/topics/work-signals/architecture.md
---

# Review log — work-signals / architecture.md

## Round 1 — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `architecture.md` blob `e5da848c076e94dacb1cf44852bb56cc23c1b2ac`
  (untracked)
- Reviewer: claude-code
- Method: contract review against the store and the parser rather than against
  the document's own claims. The source-log facts table was checked symbol by
  symbol in `internal/activity/activity.go` (`parseCodex` at 117-130,
  `parseClaude` at 132-, `Parser.turnID` at 70-102, the package and `Detail`
  doc comments at 1-24); the schema claims against
  `internal/store/migrations.go:39` (`usage_events`), `:91-93`
  (`usage_tool_calls`, v13) and `internal/store/store.go:20`
  (`CurrentSchemaVersion = 18`); and the projection claims against
  `internal/desktop/desktop.go:475-503` and
  `apps/macos/AgentDeckShared/DesktopWire.swift:63`. This document is reviewed
  last of the three it depends on, so the two findings earlier rounds attributed
  here were re-judged against the current text rather than inherited as stated.
- Scope: `docs/topics/work-signals/architecture.md` only.
- Findings:
  - [P1] Decision 2's Claude grouping unit cannot represent `conversation`, so a
    Claude user's activity mix is systematically wrong. Rule 4 classifies "the
    turn contains no tool call at all" as `conversation` (line 93), while the
    Claude unit is "the maximal run of tool calls between two consecutive
    assistant messages" (lines 104-108). Work that produced no tool call forms an
    empty run, so no group exists to carry the classification and the kind is
    unreachable for that client. Behavior risk: the Activity module renders all
    four kinds always, with shares that sum
    (`ux/session-work-signals.md`, `architecture.md` Decision 5's "exactly the
    four kinds, always all four"), so a client that can never produce one of them
    reports a redistributed mix as if it were a fact about the user's work — the
    same class of per-client bias Decision 2 explicitly rules out one paragraph
    earlier when it refuses failure rate as an input. Recorded against this
    document by `requirements.md` Round 2 and `ux/session-work-signals.md`
    Round 1; confirmed here against the current text.
    💡 Bounded remediation: define the Claude unit so it can be empty of tool
    calls — an assistant message and the run following it, rather than the run
    alone — or state a different `conversation` rule for Claude and say on the
    surface that the two clients count it differently.
    -> Open.
  - [P1] Decision 3's Codex rule attributes from a quantity that does not exist.
    The rule reads "a turn's cost is split evenly across the tool calls in that
    turn" (line 119), but no turn has a cost: `usage_events`
    (`internal/store/migrations.go:39`) carries `client`, `session_id`,
    `event_id`, `event_at`, `model`, token counts, `source_path`,
    `source_offset`, and `run_id` — no turn column and no tool column. The
    document's own facts table says exactly this ("no turn and no tool"), and
    Decision 3 opens by conceding cost "cannot be measured", then proceeds as if
    a per-turn total were available to divide. Nothing states how a turn acquires
    one — by the events whose `event_at` falls inside the turn's span, by even
    division of the session, or otherwise. Behavior risk: this is the whole
    Codex half of every cost figure on both surfaces, so an implementer invents
    the derivation, and two implementers invent different ones; `cost_basis:
    turn` would then label a number whose basis is undefined.
    💡 Bounded remediation: state how a turn's cost is obtained from
    `usage_events`, including what happens when a turn spans no event and when
    one event spans several turns, and keep `cost_basis: none` for the
    uncoverable case that already exists in the table.
    -> Open.
  - [P1] Decision 5 states no bucketing rule for the work-signal families. The
    table gives each family the `Client` × `Period` product "so both filters
    govern them exactly as they govern the rest of the panel", but never says
    which period a signal falls in. No existing rule transfers: a signal record
    is per group, while `internal/desktop/desktop.go:498-503` buckets a *session*
    by its last event and the `usage` group buckets *events* by `event_at`.
    Behavior risk: `ux/cli-work-signals.md` carries `requirements.md` Acceptance
    item 1 — a figure read in the app is reproducible from a terminal — and that
    guarantee rests entirely on both surfaces bucketing the same object the same
    way; without a rule here, a group straddling a period boundary can land in
    different periods on the two surfaces and the guarantee silently fails.
    Recorded against this document by `ux/cli-work-signals.md` Round 1's repair,
    which proposed `started_at` and correctly declined to decide it inside a
    surface document; confirmed here.
    💡 Bounded remediation: add the membership rule to Decision 5 — a group falls
    in a period when its `started_at` does is the natural one, and the storage in
    Decision 4 already carries `started_at` and indexes it — and state that it is
    deliberately not the session rule, since the two bucket different objects.
    -> Open.
  - [P2] Decision 2's rule table is not exhaustive, and the prose beneath it
    states a broader rule than the table does. Rule 4 requires "no tool call at
    all", so a turn of reads only matches none of the four rows under the stated
    "first match wins" evaluation; the prose then says such a turn "is classified
    as `conversation`" (lines 95-98). Both land in the same place, but they are
    different rules, and the table is the artifact an implementer codes.
    Behavior risk: a literal implementation leaves read-only turns unclassified,
    which is not a state any surface can render — every module assumes each group
    carries a kind.
    💡 Bounded remediation: make row 4 the default ("anything else"), and keep
    the read-only example in the prose as the illustration it is.
    -> Open.
  - [P2] Decision 3's Claude rule is ambiguous about what receives the cost.
    "Session cost is split across the session's tool calls in proportion to call
    count per group" (line 120) can be read as splitting over calls or over
    groups; read literally as over calls, the per-group weighting is redundant
    and the rule degenerates to an even per-call split. Since
    `usage_work_signals` holds one row per group (Decision 4), the unit that
    needs a cost is the group. Behavior risk: the two readings agree on a
    session's total but disagree on any per-kind figure whenever groups differ in
    size, which is exactly what the Activity module displays.
    💡 Bounded remediation: say the split is across groups in proportion to each
    group's call count, and state the tie-break for a session whose groups have
    equal counts if one is needed.
    -> Open.
- Non-findings, recorded so a later round does not re-raise them:
  - Every row of the source-log facts table is accurate. `parseCodex` does record
    output items with a hardcoded `"completed"` while only Claude's
    `tool_result.is_error` yields `"failed"`; `turn_id` is set only from Codex's
    `turn_context` and reaches no table; `function_call` arguments and
    `tool_use.input` are read by neither parser; and the package and `Detail`
    comments state the absolute boundary the document proposes to narrow.
    Refusing failure rate as a classifier input follows from the first of these
    and is the right call.
  - Decision 1's privacy boundary is complete as a contract: what is derived,
    what is kept, what must be proven absent by test, the per-client tool
    mapping, and the explicit acceptance of the residual bare-file-name exposure.
    Requiring the two stale comments to be updated in the same change is the
    correct scope.
  - Decision 4's migration is additive, `v19` is the right next number over the
    current `v18` (`internal/store/store.go:20`), the `parser_version` backfill
    follows the `v0.4.1` cache-write precedent that exists in the same file, and
    the empty-database outcome is defined.
  - Decision 5's additive-wire reasoning is correct and its consequence was
    verified in the host: `DesktopWire.swift:63` guards `wireVersion` with `==`,
    so not raising it is load-bearing rather than stylistic.
  - Decision 6's binding table is the reason the CLI document could be reviewed
    as a first-class surface rather than as a JSON dump.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0 (the project's
  document-set audit; the only checker shipped for this target class).
  `make check-whitespace` — exit 0. `git diff --check` — clean. No product test
  run: the subject is an unimplemented design and no product content state
  changed. L0.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-20
- Repaired state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `architecture.md` blob `c8b5a232a55fd1fd66a93e485ce599f4bf161422` (untracked),
  from the reviewed `e5da848c076e94dacb1cf44852bb56cc23c1b2ac`.
- Repairer: claude-code
- Scope: all five open Round 1 findings — three P1 and two P2 — in
  `architecture.md` only. The user authorized repairing all of them including
  the non-blocking ones (`全部修复 包括非阻塞`). No sibling document was touched.
- Findings closed:
  - [P1] Decision 2's unreachable `conversation` for Claude -> the classification
    unit is now **one object with two boundary markers** rather than two
    different objects. A turn is one assistant reply and the tool calls it made:
    Codex marks it with `turn_context`, Claude with the assistant message
    carrying a non-synthetic `model`, **together with the calls that follow it**.
    Anchoring Claude's turn on the message rather than on the run of calls is
    what makes an empty turn representable, which is what makes `conversation`
    reachable. The document now states why in those terms, naming the earlier
    definition and the bias it produced, so the fix is not silently reversible.
  - [P2] Decision 2's non-exhaustive rule table -> row 4 is now the default
    (`**Anything else.**`), naming the no-tool-call turn and the read-only turn
    together, and the table's preamble states that every turn carries exactly one
    kind. The prose beneath no longer states a second, broader rule; it explains
    why the default classifies a read-only turn as `conversation`, which is a
    definition and belongs in prose.
  - [P1] Decision 3 dividing a turn cost that did not exist -> the decision now
    opens by stating how a turn acquires one: **the sum of the `usage_events`
    rows whose `event_at` falls in the turn's span**, the span running from the
    turn's boundary marker to the next. Events are point-in-time rows, so none
    splits across turns. Both boundary cases the finding named are decided in a
    table rather than left open — a span containing no event yields
    `cost_basis: none` and borrows from no neighbour, and an event preceding the
    first turn boundary is attributed to the first turn, because discarding it
    would make a session's turns sum to less than the session.
  - [P2] Decision 3's ambiguous Claude split -> stated as a split **across
    turns**, weighted by each turn's tool-call count, then evenly within a turn.
    The document records why that reading and not the other: the unit that needs
    a cost is the turn, since `usage_work_signals` holds one row per turn and the
    Activity module sums per kind. It also notes that the two readings agree on a
    session total and diverge on every per-kind figure once turns differ in size,
    and that equal-count turns need no tie-break because equal weights divide
    evenly.
  - [P1] Decision 5's missing bucketing rule -> **a turn belongs to the period
    its `started_at` falls in**, on the local calendar, half-open as elsewhere.
    Decision 4 already carries and indexes that column. The decision states
    explicitly that this is *not* the session rule at
    `internal/desktop/desktop.go:498-503` and that both coexist because they
    bucket different objects, and it names the failure the rule prevents: the
    same turn landing in different periods on the two surfaces, which would
    break `requirements.md` Acceptance item 1 silently. It also states that
    `started_at` is a choice rather than an inevitability, and why it was chosen.
- Consequential changes made in the same pass, because leaving them would have
  meant the document used two words for one thing:
  - `usage_work_signals.group_key` -> `turn_key`, with a sentence recording the
    rename and what the key is in each client. The old name was correct only
    while Claude's unit was a run of tool calls.
  - Decision 4 now notes that the tooling family's `groups` field counts tool
    kinds, not turns — it is the prototype's `tool groups` label, and after this
    repair "group" would otherwise read as the classification unit it no longer
    is.
- Downstream, **not** repaired here and not defects:
  - `ux/session-work-signals.md` and `ux/cli-work-signals.md` were both repaired
    earlier today to avoid naming the classification unit, on the ground that it
    differed per client. It no longer differs, so those surfaces are now more
    conservative than they need to be — but conservative is not wrong: the
    wording remains true, and `ux/cli-work-signals.md`'s `turns / edit` exception
    is now positively correct rather than pending. Both documents declare a
    dependency on this finding closing; the dependency is satisfied by this
    repair and neither needs a wording change to be accurate.
- Extraction requirement added while closing the first P1, and stated because
  the repair is otherwise unimplementable: `parseClaude` already reads
  `message.model` and skips `<synthetic>`, so it sees every turn boundary — but
  it uses the value only to stamp a model onto tool calls and emits nothing for
  the message itself, so a turn that made no tool call produces no record today.
  Decision 2 now requires task 1 to emit a turn on the boundary rather than on
  the first call inside it. Without that, the new unit definition changes the
  prose and leaves `conversation` exactly as unreachable as before. Codex needs
  no equivalent change, since `turn_context` is already a distinct event.
- Final repaired state: `architecture.md` blob
  `cab22739f37d10b73da29ab4bafb709f5494d872`, superseding the
  `c8b5a232a55fd1fd66a93e485ce599f4bf161422` recorded above.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design; no product behavior is in scope.
- Dispatch: Beads `ad-ws-doc-arch-design` was already `in_progress` from the
  Round 1 REOPEN and is moved to `in_review` in this action, with the
  disposition recorded as a comment.
- Verdict: REOPEN — repair complete, awaiting independent Re-review


## Round 2 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `architecture.md` blob `cab22739f37d10b73da29ab4bafb709f5494d872` (untracked)
  — the final state the Round 1 repair recorded, confirmed with
  `git hash-object`. The intermediate `c8b5a232` the repair first named was not
  the state judged.
- Re-reviewer: claude-code
- Method: each closed finding was re-judged against the current text, and every
  premise the repair rests on was re-located in the repository rather than
  accepted from the repair note — including the extraction claim the repair
  makes about `parseClaude`, which is the load-bearing one. The document was
  then re-read end to end for regressions, and each repaired fact was searched
  for second instances, since a definition change propagates further than a
  wording fix.
- Findings re-judged:
  - [P1] Decision 2's unreachable `conversation` for Claude — **closed,
    confirmed.** The unit is now one object with two boundary markers, and
    Claude's turn is anchored on the assistant message *together with the calls
    that follow it*, so an empty turn exists and can carry `conversation`. The
    repair's enabling claim was verified rather than trusted:
    `internal/activity/activity.go:138` reads `message["model"]` and skips
    `<synthetic>`, and lines 142-155 emit records only for `tool_use` and
    `tool_result` — so the parser does see every boundary and does emit nothing
    for the message itself, exactly as stated. Requiring task 1 to emit a turn on
    the boundary is therefore not a nicety: without it the new definition changes
    only prose. Naming the earlier definition and the bias it produced makes the
    fix hard to reverse by accident.
  - [P2] Decision 2's non-exhaustive rule table — **closed, confirmed.** Row 4 is
    now `**Anything else.**`, naming the no-tool-call turn and the read-only turn
    together, and the preamble states that every turn carries exactly one kind.
    The prose beneath no longer states a competing rule.
  - [P1] Decision 3 dividing a turn cost that did not exist — **closed,
    confirmed.** A turn's cost is now defined as the sum of `usage_events` rows
    whose `event_at` falls in the turn's span. Verified against
    `internal/store/migrations.go:39`: `event_at` is a real column and events are
    point-in-time rows, so the "no event splits across turns" step holds. Both
    boundary cases are decided in a table — an eventless span yields
    `cost_basis: none` and borrows from no neighbour, and a pre-first-boundary
    event goes to the first turn so a session's turns sum to the session.
  - [P2] Decision 3's ambiguous Claude split — **closed, confirmed.** Stated as
    a split across turns weighted by tool-call count, then evenly within a turn,
    with the reason given and the divergence between the two readings named.
  - [P1] Decision 5's missing bucketing rule — **closed, confirmed.** A turn
    belongs to the period its `started_at` falls in, half-open on the local
    calendar. `usage_work_signals.started_at` exists in Decision 4 and is
    indexed. The decision states that this is deliberately not the session rule
    at `internal/desktop/desktop.go:498-503`, names the failure it prevents, and
    records that `started_at` is a choice rather than an inevitability.
- Consequential rename, judged: `group_key` -> `turn_key` is complete.
  `rg group_key` over the topic returns one hit, the sentence in Decision 4
  recording the rename. Disambiguating the tooling family's `groups` field in the
  same pass was necessary, not tidying — after this repair "group" would
  otherwise have read as the classification unit it no longer is.
- New findings, this document:
  - [P1] Decision 6's `Filters` row still asserts the exact equivalence that was
    reopened and repaired one level down. It reads "The CLI's `--period` and
    `--client` carry **exactly** the panel's two filter semantics. A figure read
    in the app is reproducible from a terminal by naming the same two values."
    That is the claim `ux/cli-work-signals.md` Round 1 P1 found false and its
    repair replaced with a stated subset correspondence: the projection emits
    three periods (`internal/desktop/desktop.go:480-487`), the CLI accepts the
    `usage` group's seven (`cmd/agentdeck/main.go:3095`). Behavior risk: this is
    the binding contract the surface document derives from, so the topic now
    holds two documents that contradict each other on its most load-bearing
    guarantee, and the one that is wrong is the one an implementer treats as
    authoritative. The repair invoked "a fact stated in two places must be
    repaired in both" for `--top` in a sibling document and did not apply it to
    this document's own upstream copy of the fact it had just seen corrected.
    💡 Bounded remediation: restate the row as the subset correspondence,
    matching the surface document, and bind the reproducibility guarantee to the
    three periods where it holds.
    -> Open.
  - [P2] Decision 6's `Bounds` row binds a flag that no longer exists. It reads
    "`--top` caps only text rows, never the JSON", but `ux/cli-work-signals.md`'s
    repair removed `--top` from the surface and recorded why it must not return.
    Behavior risk: smaller than the above — nothing is misderived — but the
    contract names a flag the surface refuses, so a reader reconciling the two
    finds a phantom.
    💡 Bounded remediation: state that nothing caps the projection or the JSON,
    and that `--module` is the only flag that changes which families appear.
    -> Open.
- Correction to the repair's downstream assessment, attributed rather than
  repaired: the repair states that the two surface documents are "now more
  conservative than they need to be — but conservative is not wrong", and that
  "neither needs a wording change to be accurate." That holds for the
  user-visible **strings**, which name no unit, and not for the documents'
  **prose about Decision 2**, which now asserts something false:
  - `ux/session-work-signals.md:136-137` states Decision 2 "classifies over a
    turn for Codex and over a tool-call group for Claude", and lines 141-147
    describe that finding as open and require the surface not to ship "before the
    architecture picks one". Decision 2 no longer says either thing and the
    finding is closed. Owner: `ad-ws-doc-ux-session-design`, whose `Review` cell
    is already ticked — this needs a new round, not a silent edit.
  - `ux/cli-work-signals.md:149-151` states the same false premise as the reason
    the activity count is labelled `events`. The labelling stays correct;
    its stated justification does not. Owner: `ad-ws-doc-ux-cli-design`, also
    already passed, and its CEv1 evidence is bound to blob `f0f94e49`, which is
    unchanged — the evidence is not invalidated, but the content it attests is
    now inaccurate about its own dependency.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **REOPEN** — the five Round 1 findings are genuinely closed, and two
  new findings in Decision 6 keep the cell unticked. Both are the same defect
  class the repair closed elsewhere, in the upstream copy it did not check.

## Round 2 — repair — 2026-08-20
- Repaired state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `architecture.md` blob `c933e68ec0492d40f59a3aca4b2750a212400ac4` (untracked),
  from the re-reviewed `cab22739f37d10b73da29ab4bafb709f5494d872`.
- Repairer: claude-code
- Scope: the two open Round 2 findings, in `architecture.md` only. Both premises
  were re-verified before repairing: `internal/desktop/desktop.go:480-487` emits
  three period names, `cmd/agentdeck/main.go:3095` registers the seven-value
  `--period`, and `ux/cli-work-signals.md` carries no `--top` and a paragraph
  saying why it must not return.
- Findings closed:
  - [P1] Decision 6's `Filters` row asserting exact equivalence -> restated as a
    **subset correspondence**, matching the surface document it binds:
    `--client` is an identity, `--period` is not, and Acceptance item 1's
    reproducibility guarantee binds to the three periods both surfaces share,
    where it is exact. The finding was right that this was the worse copy of the
    two: a surface document derives from the contract, so the contradiction sat
    in the document an implementer treats as authoritative.
  - [P2] Decision 6's `Bounds` row naming a removed flag -> now states that
    nothing caps either reader, that there is no `--top` because these families
    are already bounded, and that `--module` is the only flag changing which
    families appear. It points at `ux/cli-work-signals.md` for why the flag must
    not come back, rather than restating the reasoning in two places.
- Third instance found and closed in the same pass, unprompted. Round 2's
  criticism was that a fact repaired in a sibling document was left standing in
  this document's upstream copy, so the whole document was searched for further
  copies rather than only the two lines the findings named. Decision 5's
  bucketing rationale also promised, without qualification, that "a figure read
  in the app is reproducible from a terminal". It now binds that promise to the
  three shared periods and cites Decision 6's row, so the three statements of
  one fact — Decision 5, Decision 6, and the CLI surface — agree. `rg` over the
  document confirms no fourth copy: `--top` appears once, in the row that denies
  it; `group_key` appears once, in the sentence recording its rename.
- Attributed elsewhere, **not** repaired here — Round 2's correction to the
  Round 1 repair's downstream assessment stands, and both targets have a ticked
  `Review` cell, so each needs a round of its own rather than a silent edit:
  - `ux/session-work-signals.md` states Decision 2 "classifies over a turn for
    Codex and over a tool-call group for Claude" and describes that finding as
    open, requiring the surface not to ship "before the architecture picks one".
    Decision 2 no longer says either thing and the finding is closed. Owner:
    `ad-ws-doc-ux-session-design`.
  - `ux/cli-work-signals.md` gives the same false premise as its reason for
    labelling the activity count `events`. The labelling stays correct; its
    justification does not. Owner: `ad-ws-doc-ux-cli-design`. Its CEv1 evidence
    is bound to blob `f0f94e49`, unchanged, so the evidence is not invalidated —
    what it attests is simply now inaccurate about its own dependency.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design; no product behavior is in scope.
- Dispatch: Beads `ad-ws-doc-arch-design` was `in_progress` from the Round 2
  REOPEN and is moved to `in_review` in this action, with the disposition
  recorded as a comment.
- Verdict: REOPEN — repair complete, awaiting independent Re-review


## Round 3 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `architecture.md` blob `c933e68ec0492d40f59a3aca4b2750a212400ac4` (untracked)
  — the state the Round 2 repair recorded, confirmed with `git hash-object`.
- Re-reviewer: claude-code
- Method: the two closed findings were re-judged against the current text and
  their premises re-located in the repository rather than accepted from the
  repair note. The unprompted third closure was verified as a real instance
  rather than accepted as a claim, and the whole document was searched again for
  a fourth copy, since "repaired in one place, left standing in another" is what
  Round 2 reopened this document for.
- Findings re-judged:
  - [P1] Decision 6's `Filters` row asserting exact equivalence — **closed,
    confirmed.** The row now reads "A subset correspondence, not an identity",
    separates `--client` (identity) from `--period` (not), and binds Acceptance
    item 1's guarantee to the three shared periods. Both cited facts were
    re-verified: `internal/desktop/desktop.go:480-487` emits `today`, `7d`,
    `30d`, and `cmd/agentdeck/main.go:3095` registers the seven-value
    `--period`. The contract and `ux/cli-work-signals.md` now say the same
    thing.
  - [P2] Decision 6's `Bounds` row naming a removed flag — **closed,
    confirmed.** It now states that nothing caps either reader, that there is no
    `--top` because the families are already bounded, and that `--module` is the
    only flag changing which families appear, pointing at the surface document
    for the reasoning rather than duplicating it. This is consistent with
    Decision 5, whose bounds are producer-enforced in the derivation rather than
    by a flag.
- Unprompted third closure, verified rather than accepted: Decision 5's bucketing
  rationale did carry an unqualified copy of the same promise, and it now binds
  to the three shared periods and cites Decision 6's row. Searching the document
  again confirms no fourth copy — `rg` returns `--top` once, in the row that
  denies it; `group_key` once, in the sentence recording its rename; and
  `tool-call group` not at all, so the retired unit vocabulary is fully gone.
  Finding a third instance without being told is the behaviour Round 2 asked
  for.
- Regression check: no regression found. The facts table's "turn-level cost
  attribution is unavailable for Claude" still agrees with Decision 3's
  session-level Claude rule; Decision 4's `turn_key` description still matches
  Decision 2's two boundary markers; and the `Bounds` row's "nothing caps either
  reader" does not contradict Decision 5's bounded `rows[]`, because those bounds
  are producer-enforced rather than flag-enforced.
- Still open, attributed elsewhere and not this document's to close. Both targets
  have a ticked `Review` cell, so each needs a round of its own:
  - `ux/session-work-signals.md` states Decision 2 classifies over a tool-call
    group for Claude and describes that finding as open. Owner:
    `ad-ws-doc-ux-session-design`.
  - `ux/cli-work-signals.md` gives the same false premise as its reason for
    labelling the activity count `events`; the labelling stays correct, its
    justification does not. Owner: `ad-ws-doc-ux-cli-design`. Its CEv1 evidence
    is bound to blob `f0f94e49`, unchanged, so the evidence is not invalidated.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **PASS**
