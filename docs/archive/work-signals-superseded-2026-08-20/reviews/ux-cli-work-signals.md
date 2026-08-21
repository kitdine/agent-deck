---
status: active
topic: work-signals
subject: docs/topics/work-signals/ux/cli-work-signals.md
---

# Review log — work-signals / ux/cli-work-signals.md

## Round 1 — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `c6a14a10e04b9b1f061ca0a274d5a4616a13d78e`
  (untracked)
- Reviewer: claude-code
- Method: design review against the shipped CLI rather than against the
  document's claims. Every flag was matched against the real `usage stats`
  registration in `cmd/agentdeck/main.go:3095-3106`; the panel-filter equivalence
  claim was checked against the projection that actually emits the panel's
  periods, `internal/desktop/desktop.go:475-503`; the JSON families and units
  were matched against `architecture.md` Decision 5; the schema version against
  `internal/store/store.go:20` and Decision 4; and `usage scan` and
  `session show --activity` were confirmed to exist. One finding attributed to
  this document by [`requirements.md`](requirements.md) Round 1 was re-judged
  here rather than inherited.
- Scope: `docs/topics/work-signals/ux/cli-work-signals.md` only.
- Findings:
  - [P1] Line 44's equivalence claim is false in both halves, and the document
    marks it as the one thing that "MUST NOT drift". It states `--period` and
    `--client` are "exactly the menu-bar panel's two filters, with exactly the
    same semantics". **Value sets differ:** this document's `--period` accepts
    seven values (`today`, `7d`, `30d`, `week`, `month`, `6m`, `all`), matching
    `usage stats`; the panel emits three, `today`, `7d`, `30d`
    (`internal/desktop/desktop.go:480-487`). **Semantics differ:** the panel
    assigns a session to a period by its *last event*
    (`desktop.go:498-503`, "A session belongs to a period when its last event
    falls inside it"), while the `usage` group filters *events* by `event_at`.
    Behavior risk: this is the sentence that carries `requirements.md`
    Acceptance item 1 — that a figure read in the app is reproducible from a
    terminal — so an implementer will build to it, and the two surfaces will
    disagree on any session straddling a boundary. `requirements.md` Round 1
    attributed this here; it is now this document's to close.
    💡 Bounded remediation: state the correspondence as it is — the panel's three
    periods are a subset of the CLI's seven, and the reproducibility guarantee
    holds for those three — and decide explicitly which bucketing rule the
    signals use, since the signal record is per turn, not per event.
    -> Closed by the Round 1 repair; confirmed at Round 2.
  - [P1] `--top` caps two lists, neither of which exists. The flag table (line 40)
    describes it as the "row cap for the tool-kind and file lists". The tool-kind
    list is bounded at four by `architecture.md` Decision 5, which is below any
    cap a user would set; and there is no file list at all — Decision 5's
    workflow family carries `top_file_base_name` and `top_file_count`, a single
    file, not a ranked list. Behavior risk: a documented flag with nothing to act
    on is the same defect the document itself rejects two sections later, where
    `--group-by` is turned down for producing "a flag that silently does nothing".
    💡 Bounded remediation: drop `--top` from the surface, or give the workflow
    family a real most-touched list in `architecture.md` first and cap that. Do
    not keep the flag as a no-op for symmetry with `usage stats`.
    -> Closed by the Round 1 repair; confirmed at Round 2.
  - [P2] The text output names a unit the design does not have for both clients,
    in four places. Lines 69-75 print the activity event count as `24 turns`, and
    line 76 prints `turns with no tool call count as Conversation`. Per
    `architecture.md` Decision 2 the classification unit is a turn for Codex and
    a tool-call group for Claude, so both readings are wrong for a Claude user.
    The sibling GUI document was repaired for exactly this on the same day and
    now states the rule without naming the unit; the wire field is `events`, not
    `turns`. Behavior risk: the two first-class surfaces would disagree in
    wording about the same number, which is what Decision 6 exists to prevent.
    💡 Bounded remediation: print the neutral unit the wire uses and reword the
    note the way `ux/session-work-signals.md` now does.
    -> Closed by the Round 1 repair; confirmed at Round 2.
  - [P2] The empty and unavailable states are specified for `text` only. The
    table at lines 117-122 gives a sentence and an exit code for each of the four
    conditions, but `--format json` is never bound to any of them, and
    `--module`'s effect on JSON is unstated — the flag is defined as selecting
    "which sections render", which is a text concept. Behavior risk: JSON is the
    scripted reader, so its shape in the states a script most needs to detect —
    nothing captured yet, scope empty — is exactly what is left to invent, and a
    script cannot distinguish "no signals" from "field absent" without it.
    💡 Bounded remediation: one row or one sentence binding each state to its
    JSON shape (`available: false` versus an empty `items`), and one sentence
    saying whether `--module` filters the JSON families or only the text
    sections.
    -> Open.
- Non-findings, recorded so a later round does not re-raise them:
  - All seven flags match the shipped `usage stats` registration in meaning and
    default where they overlap: `--period` default `7d` and its exact value list,
    `--from`/`--to`, `--client`, `--no-scan`, and `--top`'s "unset keeps the
    default cap, 0 shows every row" phrasing all correspond to
    `cmd/agentdeck/main.go:3095-3106`. `--module` is new and overloads nothing.
  - The JSON sample's field names, units, and nesting match `architecture.md`
    Decision 5 family for family, costs are decimal strings as elsewhere in this
    CLI, and the sample parses.
  - Schema v19, `agentdeck usage scan`, and `session show <id> --activity` all
    exist as cited — v19 is Decision 4's migration over the current v18
    (`internal/store/store.go:20`).
  - The "What this surface must not do" section is concrete and matches the
    storage guarantee in Decision 1 rather than restating it as an aspiration.
  - Rejecting `--group-by` with a stated reason is the right call and is
    recorded, not merely omitted.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0 (the project's
  document-set audit; the only checker shipped for this target class).
  `make check-whitespace` — exit 0. `git diff --check` — clean. No product test
  run: the subject is an unimplemented design and no product content state
  changed. L0.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-20
- Repaired state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `f0f94e49ca2dcba6a52712523d5c6afdd333c415`
  (untracked), from the reviewed `c6a14a10e04b9b1f061ca0a274d5a4616a13d78e`.
- Repairer: claude-code
- Scope: all four open Round 1 findings, in `ux/cli-work-signals.md` only. The
  user authorized repairing all of them (`全部问题都修复`). No sibling document
  was touched — one finding's full resolution needs `architecture.md` and is
  attributed there rather than reached into.
- Each finding's premise was re-verified against the code before repairing, not
  taken from the review record.
- Findings closed:
  - [P1] Line 44's false equivalence -> replaced with the correspondence as it
    actually is. Verified first: `internal/desktop/desktop.go:475-495` emits
    exactly `today`, `7d`, `30d`, and the comment at `498-503` states the session
    rule is `start <= last < end` over the last event, while the `usage` group
    filters events by `event_at`. Both halves of the finding hold. The document
    now states a **subset correspondence** — the panel's three periods are a
    subset of this command's seven, `--client` is an identity, and
    `requirements.md` Acceptance item 1's reproducibility guarantee is bound to
    those three periods, where it is exact. `week`, `month`, `6m`, and `all` have
    no panel counterpart to disagree with, which is why widening the flag set
    costs nothing.
  - [P1] `--top` capping two lists that do not exist -> the flag is **dropped**,
    not kept as a no-op. Verified: Decision 5 bounds the tooling rows at four,
    below any cap a user would set, and the workflow family carries
    `top_file_base_name` and `top_file_count` — one file, not a ranking. A
    paragraph records why there is no `--top`, so a later editor does not add one
    back for symmetry with `usage stats`. The JSON section's "never capped by
    `--top`" sentence was a second instance of the same claim and was rewritten
    rather than left dangling; the lesson from `ux/session-work-signals.md` Round
    2 — a fact stated in two places must be repaired in both — was applied here
    without waiting to be told again.
  - [P2] `turns` naming a unit only Codex has -> the activity count is now
    labelled `events`, which is the wire's own field name in Decision 5, and the
    Conversation note now reads `work with no tool call counts as Conversation`,
    matching the panel's repaired wording. A rendering rule records why, citing
    Decision 6: the two surfaces share field names, so their wording about the
    same number must not diverge.
  - [P2] Empty and unavailable states specified for `text` only -> a second table
    binds each of the four conditions to its `--format json` shape, and the
    `available: false` versus empty-`items` distinction is stated in one
    sentence, because that distinction is the only thing separating "never
    captured" from "a quiet week" for the reader that cannot see prose.
    `--module`'s effect on JSON is now stated: an unselected family is absent
    entirely, and the document says why key presence carries meaning in that one
    place — it echoes the caller's own argument rather than asserting anything
    about the data.
- Deliberately **not** changed, and why:
  - `Iteration depth`'s `turns / edit` note. It names the same unit the P2
    corrected elsewhere, but it is the prototype's own string
    (`i18n.js` `turnsPerEdit`) and the panel ships it verbatim. Changing it on
    this surface alone would create precisely the cross-surface divergence
    Decision 6 exists to prevent. It is stated in the document as the one
    exception and closes with the architecture's unit finding. The Round 1
    finding did not name it, so this is a disclosure rather than a silent
    extension of scope.
- New finding, attributed elsewhere and **not** repaired here:
  - [P1] `architecture.md` Decision 5 does not state a bucketing rule for the
    work-signal families. This document's reproducibility guarantee — the thing
    the P1 above was repaired to make honest — rests on both surfaces bucketing a
    signal by the same rule, and a signal record is per group, not per session
    and not per event, so neither existing rule applies to it by default. The
    document now states the requirement and the natural rule (a group falls in a
    period when its `started_at` does), and marks the decision as the
    architecture's. Owner: `ad-ws-doc-arch-design`. Repairing it here would have
    put a data contract in a surface document.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design; no product behavior is in scope.
- Dispatch: Beads `ad-ws-doc-ux-cli-design` was already `in_progress` from the
  Round 1 REOPEN and is moved to `in_review` in this action, with the
  disposition recorded as a comment.
- Verdict: REOPEN — repair complete, awaiting independent Re-review


## Round 2 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `f0f94e49ca2dcba6a52712523d5c6afdd333c415`
  (untracked) — the exact state the Round 1 repair recorded, confirmed with
  `git hash-object` rather than assumed.
- Re-reviewer: claude-code
- Method: each closed finding was re-judged against the current document, and
  every premise the repair rests on was re-located in the repository rather than
  accepted from the repair note. The document was then re-read end to end for
  regressions the repair could have introduced, and for second instances of each
  repaired fact.
- Findings re-judged:
  - [P1] Line 44's false equivalence — **closed, confirmed.** The claim is now a
    stated subset correspondence. Verified independently: the projection emits
    exactly `today`, `7d`, `30d` (`internal/desktop/desktop.go:480-487`), the CLI
    keeps the `usage` group's seven (`cmd/agentdeck/main.go:3095`), and the
    reproducibility guarantee is now bound to the three periods where it can
    hold. The `--client` identity claim also holds: the projection scopes by
    `all` / `codex` / `claude` (`desktop.go:489-492`) over the same field the
    flag selects.
  - [P1] `--top` capping lists that do not exist — **closed, confirmed.** The
    flag is gone from the table, which now carries six flags, and a paragraph
    records why it must not come back for symmetry. The second instance was
    repaired too: the JSON section no longer says "never capped by `--top`" but
    "Nothing caps the JSON". `rg -- '--top'` over the document returns one hit,
    the paragraph explaining its absence.
  - [P2] `turns` naming a Codex-only unit — **closed, confirmed.** The four
    activity lines now read `events`, matching Decision 5's field name, and the
    note reads `work with no tool call counts as Conversation`, matching the
    wording `ux/session-work-signals.md` was repaired to. A rendering rule states
    why, citing Decision 6.
  - [P2] Empty and unavailable states specified for `text` only — **closed,
    confirmed.** A second table binds all four conditions to their JSON shape,
    and the two states a script must separate are separated by the thing a script
    can test: `available: false` versus `available: true` with empty `items`.
    `--module`'s effect on JSON is stated, and the document itself flags that
    this is the one place key presence carries meaning — an exception it declares
    rather than leaves for a reader to trip over.
- Repair boundary: **respected.** No sibling document was modified. The
  bucketing gap the repair discovered was recorded and attributed rather than
  decided inside a surface document, which is the correct call — a bucketing rule
  is a data contract.
- Disclosed non-repair, judged: `Iteration depth`'s `turns / edit` note was
  deliberately left. It is the prototype's own `turnsPerEdit` string and the
  panel ships it verbatim, so changing it on one surface only would create the
  cross-surface divergence Decision 6 exists to prevent. It is not this
  document's defect; it closes with `architecture.md`'s open unit finding. The
  disclosure was the right way to handle it.
- New findings: none. The document was re-read in full, including the sections
  the repair did not touch.
- Attributed elsewhere, not this document's to close:
  - `architecture.md` Decision 5 states no bucketing rule for the work-signal
    families, which is what this document's reproducibility guarantee rests on.
    Owner: `ad-ws-doc-arch-design`. Recorded by the Round 1 repair and confirmed
    here against Decision 5, which carries the `Client` × `Period` product but no
    rule for which period a group falls in.
  - `architecture.md` Decision 2's classification unit differing per client.
    Owner: `ad-ws-doc-arch-design`.
  - `tasks.md` task 5 still says "the seven flags that document fixes" and
    "uncapped by `--top`"; the document now fixes six and has no `--top`.
    Owner: `ad-ws-doc-tasks-design`. Not corrected from this round, which is
    read-only with respect to other tasks' subjects.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **PASS**

## Round 3 — cross-document sweep — 2026-08-20
- State before: blob `f0f94e49ca2dcba6a52712523d5c6afdd333c415`, the state
  Round 2 passed. State after: `891a1aa47c866b8ec2e1a79123c96970570f6724`.
  HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4` throughout.
- Editor: claude-code
- Why this document was edited after passing: `architecture.md` Rounds 1 and 2
  rewrote Decision 2 after this document's Round 2 `PASS`, and lines 149-151
  justified labelling the activity count `events` by a per-client unit difference
  Decision 2 no longer has. `architecture.md` Round 2 and Round 3 both attributed
  the drift here. The label itself was and is correct; only its stated reason was
  false.
- Change made: the rendering rule now says `events` is the wire's own field name
  and that using it keeps both readers' wording identical for one number, noting
  that `turns` would also be accurate now. No other line changed.
- Consequences that must not be lost:
  - The `Review` cell for this document is **unticked**. Round 2's `PASS` was
    given to blob `f0f94e49`, and this is different content; a repairer's own
    edit cannot carry a prior verdict forward.
  - The CEv1 evidence recorded at Round 2 (`work-signals:ux/cli-work-signals.md`,
    content_state `4dc2962b…`, bound to blob `f0f94e49`) remains a true record of
    that state and is **not** rewritten. It no longer describes the current
    content, so the next `PASS` must record a new `content_state` and `evidence`
    and relate it to that one with `supersedes`, per
    `.agent-instructions/evidence.md`.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0.
- Verdict: REOPEN — content changed after `PASS`; awaiting independent
  Re-review of the new state.

## Round 4 — re-review, then repair — 2026-08-20
- State re-reviewed: blob `891a1aa47c866b8ec2e1a79123c96970570f6724`.
  State after the repair this round required:
  `b914c6b21b67e3798c95950bb71bce7bdaa70d82`. HEAD
  `9613498123f00b59d3d4b84fbff71e0f71d6ebd4` throughout.
- Re-reviewer and repairer: claude-code — the same actor, stated plainly.
- Findings, all four against the sweep's own edit to this document. The Round 3
  sweep corrected the false Decision 2 premise but left the conclusions that
  depended on it, which is the same defect class the sweep was run to end:
  - [P1] Lines 71-75 still said the bucketing rule "belongs to Decision 5, which
    does not yet state one" and that "an implementer has nothing to build
    against". Decision 5 states it — a turn belongs to the period its
    `started_at` falls in — and `architecture.md` passed its Round 3 with that
    rule in place. The sweep's keyword search missed this because the paragraph
    uses none of the retired vocabulary it searched for; a search for stale
    *wording* does not find a stale *claim*.
    -> Closed: the paragraph now states the rule Decision 5 decided and says the
    two documents agree.
  - [P2] Lines 148-149 repeated "the wire's own field name" in the bullet body
    after the bullet head had already said it.
    -> Closed: the body no longer repeats the head.
  - [P2] Lines 155-161 still called `turns / edit` "the one exception", left "as
    it is", closing "with `ad-ws-doc-arch-design`'s unit finding". That finding
    closed at `architecture.md` Round 2 and `architecture.md:114-115` states the
    label is accurate for both clients.
    -> Closed: it is now stated as the prototype's string, accurate for both
    clients under Decision 2, needing no qualification.
  - [nit] A line break left mid-sentence by the sweep's substitution.
    -> Closed.
- Verification after the repair: `rg` for `ad-ws-doc`, `does not yet state`,
  `one exception`, and `tool-call group` across all five product documents
  returns nothing.
- Consequence for the record: Round 2's CEv1 evidence is bound to blob
  `f0f94e49` and stays as the true record of that state. The current state is
  `b914c6b2`, so a `supersedes`-related evidence record is owed at the next
  `PASS`; it is not recorded here, because a repairer's own round is not the
  round that should record a completion gate.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0.
- Verdict: REOPEN — four findings raised and repaired in this round; the new
  state has not been judged by a round that did not write it.

## Round 5 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `b914c6b21b67e3798c95950bb71bce7bdaa70d82`
  (untracked), confirmed with `git hash-object` — the state Round 4 left and did
  not have judged by a round that did not write it.
- Re-reviewer: claude-code
- Method: Round 4's four findings were re-judged against the current text, and
  its verification claims were re-run rather than accepted. The document was then
  swept for the *class* of defect the last three rounds have each caught a
  further instance of — a downstream document not following an upstream
  definition change — using the concept rather than the retired vocabulary,
  since that is precisely what the earlier sweeps missed.
- Findings re-judged:
  - [P1] The bucketing paragraph claiming Decision 5 states no rule — **closed,
    confirmed.** It now says Decision 5 "now states it" and gives the rule: a
    turn belongs to the period its `started_at` falls in, half-open on the local
    calendar. Verified upstream at `architecture.md:265`, and the two documents
    do agree.
  - [P2] The repeated "the wire's own field name" — **closed, confirmed.** The
    bullet body no longer restates its head; it now explains why the field name
    is preferred over `turns` even though `turns` would also be accurate.
  - [P2] `turns / edit` described as "the one exception" pending an open finding
    — **closed, confirmed.** It is now stated as the prototype's own string,
    accurate for both clients under Decision 2, needing no qualification. Checked
    upstream: `architecture.md:113-115` says exactly that.
  - [nit] The mid-sentence line break — **closed, confirmed.**
- Round 4's verification claims, re-run independently: `ad-ws-doc`,
  `does not yet state`, `one exception`, and `tool-call group` each return no
  hits across the topic's five product documents. The claim was accurate.
- New finding:
  - [P2] The bucketing block still names the classification unit `group`, the one
    word `architecture.md` went out of its way to disambiguate. Lines 63, 64, and
    69 read "A signal record is per group", "a group falls in a period when its
    `started_at` does", and "a straddling group" — while the paragraph
    immediately beneath, added by the Round 4 repair, states the same rule as
    "a turn belongs to the period its `started_at` falls in". So the document
    uses two words for one concept, four lines apart. Upstream, `group` is no
    longer that concept's name at all: Decision 2 renamed the unit to `turn`
    in both clients, Decision 4 renamed the column `group_key` to `turn_key`, and
    Decision 4 adds a sentence warning that the tooling family's `groups` field
    counts tool kinds and "the two senses of 'group' are unrelated". This
    document then uses `groups` in that second sense at lines 127 and 220, so
    both senses are live in one file. Behavior risk: no rule is misstated and
    nothing would be misimplemented — the risk is the one the upstream sentence
    was written to prevent, a reader taking the classification unit for the tool
    grouping. The block also still reads as a proposal ("the natural rule is"),
    which is what it was when Decision 5 had not decided; it now describes a
    decided rule and should say so in the decided vocabulary.
    💡 Bounded remediation: say `turn` in the block, and drop the proposing
    voice now that the rule exists upstream.
    -> Closed by the Round 5 repair below.
- On the method, because this is the third consecutive round to find one more
  instance of one defect class: Round 3 swept by keyword and missed a stale
  *claim* that used none of the retired words; Round 4 swept by keyword and
  verified four retired phrases, and missed a retired *term* that is also an
  ordinary English word and a live field name in the same file. A keyword sweep
  cannot find either. What finds them is re-reading each downstream statement
  against the current upstream text, which is what this round did and what the
  next repair should do rather than adding a fifth keyword.
- Attributed elsewhere: none outstanding. Round 2's finding against `tasks.md`
  task 5 has already been acted on — line 140 now reads "There is no `--top`" —
  and `tasks.md` is itself unreviewed, so that was its own document's edit to
  make.
- CEv1: Round 2's evidence is bound to blob `f0f94e49` and remains the true
  record of that state. This round records none: the verdict is not a `PASS`, and
  the `supersedes`-related record Rounds 3 and 4 both note as owed is owed at the
  next `PASS`, against whatever state passes.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- Verdict: **REOPEN** — Round 4's four findings are genuinely closed; one P2
  keeps the cell unticked.


## Round 5 — repair — 2026-08-20
- Repaired state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `10394ff44d83d95085743757053b40daef65dabb`,
  from the re-reviewed `b914c6b21b67e3798c95950bb71bce7bdaa70d82`.
- Repairer: claude-code
- Scope: the one open Round 5 finding, in this document only.
- Method, following Round 5's instruction not to add a fifth keyword: the
  bucketing block was re-read against the current `architecture.md` rather than
  searched, and every occurrence of `group` in this file was then classified by
  sense rather than by string.
- Finding closed:
  - [P2] The bucketing block naming the classification unit `group` -> the block
    now says **turn** in all three places, matching the paragraph beneath it and
    `architecture.md` Decisions 2, 4, and 5. The proposing voice is gone with it:
    "the natural rule is that a group falls in a period" was the wording of a
    proposal made while Decision 5 had not decided, and it now reads "the rule is
    that a turn belongs to the period its `started_at` falls in", which is what
    Decision 5 states. The paragraph beneath no longer says "which now states
    it", since a decided rule does not need its recency announced.
  - A sentence was added naming the collision directly: the unit is a `turn`
    here and never a "group", because `groups` is live in this same document in
    the unrelated sense of a tool-kind count. `architecture.md` Decision 4
    carries the same warning; stating it on this side is what stops the next
    editor reintroducing the word.
- Verification by sense, not by string: `rg` for `group` in this document now
  returns ten hits and none of them names the classification unit — `usage group`
  (the command group, three hits), `--group-by` (the rejected flag, two hits),
  `groups` / `4 groups` (the tool-kind count, two hits), and three inside the new
  sentence that exists to distinguish them.
- Evidence: `bash scripts/check-topic-docs.sh` exit 0; `make check-whitespace`
  exit 0; `git diff --check` clean. L0 — a documentation-only change to an
  unimplemented design.
- Dispatch: Beads `ad-ws-doc-ux-cli-design` was `in_progress` from the Round 5
  REOPEN and is moved to `in_review` in this action.
- CEv1: none recorded. The `supersedes`-related record owed since Round 3 is owed
  at the next `PASS`, against the state that passes.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 6 — independent re-review — 2026-08-20
- Reviewed state: HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`,
  `ux/cli-work-signals.md` blob `10394ff44d83d95085743757053b40daef65dabb`,
  confirmed with `git hash-object`.
- Re-reviewer: claude-code
- Method: as Round 5 required, by reading each downstream statement against the
  current upstream text rather than by keyword. Every claim this document makes
  about `architecture.md` was located in the current `architecture.md` — the
  three families' field lists, `cost_basis`, the privacy boundary, Decision 6's
  `Filters` and `Bounds` rows (both rewritten by `architecture.md` Round 2 after
  this document was last repaired), and the bucketing rule. The two state tables
  were checked against each other, and every `group` in the file was classified
  by sense.
- Finding re-judged:
  - [P2] The bucketing block naming the classification unit `group` — **closed,
    confirmed.** The block says `turn` in all three places and the proposing
    voice is gone. Verified by sense rather than by string: of the eleven lines
    carrying `group` in this file, four are the `usage` command group, two the
    rejected `--group-by` flag, two the tooling family's tool-kind count, and
    three the new sentence that exists to keep those senses apart. **None** names
    the classification unit. Adding that sentence rather than only substituting
    the word was the right call: it is what stops the next editor reintroducing
    it.
- Cross-document check, which is what this round was set up to do:
  - Decision 6's `Filters` row was rewritten by `architecture.md` Round 2 into a
    subset correspondence. It now agrees with this document's lines 48-56 —
    `--client` an identity, `--period` a subset, the guarantee bound to the three
    shared periods. The two cite different line ranges for the same projection
    function, which is not a disagreement.
  - Decision 6's `Bounds` row was rewritten in the same round to state that
    nothing caps either reader and there is no `--top`. It agrees with this
    document, and the citation runs one way — the contract points here for the
    reasoning, and this document does not point back, so there is no loop.
  - The three families' field names, in the JSON sample, match Decision 5 field
    for field: activity's `kind`/`share`/`cost`/`events`/`cost_basis`, workflow's
    six values including its absence of `cost_basis`, and tooling's `calls`,
    `groups`, `share_of_cost`, `cost_basis`, `top_mcp_server`, `top_mcp_calls`,
    and bounded `rows[]`.
  - The privacy section matches Decision 1's storage guarantee rather than
    restating it as an aspiration.
  - The text state table and the JSON state table carry the same four conditions
    in the same order, and the pair a script must separate is separated by
    `available`.
- New finding:
  - [P2] The single-session `SIGNALS` line displays a session-level activity kind
    that no upstream decision defines. Line 248 renders
    `SIGNALS   Coding · 12 tool calls · 3 files · first edit 4m`. Three of those
    four values aggregate a session's turns unambiguously — a count, a distinct
    count, and a first occurrence. `Coding` does not: `usage_work_signals` holds
    one `activity_kind` per **turn** (Decision 4), a session has many turns, and
    no decision says how one kind is chosen from many. Most turns? Most cost?
    Most calls? The first? Each gives a different word on the screen for the same
    session. Behavior risk: this is the same shape as the turn-cost defect Round
    1 raised against `architecture.md` — a document consuming a quantity the
    contract never produces — and the consequence is the one that finding named:
    an implementer invents the derivation, and two implementers invent different
    ones. It is smaller in blast radius, one line of an auxiliary command rather
    than every cost figure, which is why it is P2 and not P1.
    💡 Bounded remediation: this document chooses whether the line carries a kind
    at all — dropping it leaves three values that need no rule — and if it keeps
    it, `architecture.md` owes the aggregation rule, as it owed the bucketing
    rule for the same reason. Do not decide the rule here.
    -> Open.
- Non-findings, recorded so a later round does not re-raise them:
  - The JSON sample shows one `activity` item while Decision 5 requires exactly
    four, and one `rows[]` entry while tooling allows up to four. The sample is
    illustrating field shape, which the sentence above it says, and single-entry
    samples are the ordinary convention. Not a contract statement and not a
    defect.
  - Round 5's repair note counts "ten hits" for `group` and attributes three to
    `usage group`; the file carries eleven such lines and four are `usage group`.
    The classification is right and no conclusion depends on the count. Recorded
    as an inaccuracy in the record, not a finding against the document.
- Evidence: `bash scripts/check-topic-docs.sh` — exit 0; `make check-whitespace`
  — exit 0; `git diff --check` — clean. No product test run: the subject is an
  unimplemented design and no product content state changed. L0.
- CEv1: none recorded; the verdict is not a `PASS`. The `supersedes`-related
  record owed since Round 3 remains owed at the next `PASS`.
- Verdict: **REOPEN** — the Round 5 finding is genuinely closed and the
  cross-document check is clean; one new P2 keeps the cell unticked.
