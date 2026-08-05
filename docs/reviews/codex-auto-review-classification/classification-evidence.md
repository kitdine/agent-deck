---
status: active
plan: codex-auto-review-classification
task: classification-evidence
---

# Review log — codex-auto-review-classification / classification-evidence

## Round 1 — 2026-08-04

- Reviewed state: worktree on `63c9b97`;
  `docs/plans/codex-auto-review-classification.md` blob
  `e3a0a7eb5ee691cf1345803c0c822a1248f79c84`, `docs/README.md` blob
  `13c53a9a10f06a4a500e93115364b8a4d4e77662`. Both files uncommitted.
- Reviewer: Claude Opus 5 (independent review pass)
- Scope: the task's four acceptance criteria and its declared L0-plus
  verification — the recorded Evidence Baseline probe block
  (`docs/plans/codex-auto-review-classification.md:77-201`), the Status matrix
  (`:307-318`), and the `docs/README.md` roadmap row, plan-index entry, and
  Backlog reinstatement. No product code is in scope; this task ships none.

### Acceptance criteria

**1. "The recorded evidence is reproducible from the stated method." — PASS.**
The documented aggregate query was re-run verbatim against the real local state
database. Every recorded figure reproduced exactly: 2,194 target events, 93
sessions, 2,136 distinct event IDs; 138,434,245 input, 123,631,488 cached
input, 197,602 output, and zero for cache read, cache creation, five-minute
cache write, and one-hour cache write; 1,934 `with_other_session`, 0
`with_other_event_id`; 735 / 1,545 / 1,933 / 1,933 for the 1s / 5s / 60s / 300s
windows; adjacency pairs `gpt-5.4` 103, `gpt-5.4-mini` 11, `gpt-5.5` 402,
`gpt-5.6-luna` 19, `gpt-5.6-sol` 833, `gpt-5.6-terra` 339; 0 local price rows.
The `immutable=1` caveat holds in practice — the re-run created no WAL or SHM
sidecar in `~/.agentdeck/`.

**2. "The selected branch follows from the evidence rather than from the earlier
'most likely a pseudo-model' assumption." — PASS.** The record explicitly
declines to let the retired plan's assumption stand in for proof, and states
what each branch would still require: Branch A needs authoritative
billing/account evidence of non-billability, Branch B needs an authoritative
billing mapping or matched account-level charge evidence. Absence from every
pricing catalog is correctly treated as non-evidence of non-billability rather
than as support for Branch A.

**3. "No session content of any kind appears in the plan, the repository, or the
command output." — PASS.** The probe emits aggregates only. A scan of the plan
for filesystem paths, session identifiers, and UUID-shaped strings is clean;
the query embeds `$HOME` unexpanded rather than a real path. `git status
--untracked-files=all` shows only the two intended modified documents, so the
probe left no output file behind. The model names that do appear
(`gpt-5.6-sol` and siblings) are catalog slugs, not session data.

**4. "If the outcome is inconclusive, the plan says so and task 2 is dropped
rather than reinterpreted." — PASS.** The Status matrix marks task 2 `n/a —
task 1 inconclusive` on both `Dev` and `Review` with a pointer to task 1's
conclusion, exactly as `:314-318` prescribes, and `docs/README.md` reinstates
the unresolved item in the Backlog with the reopen condition attached.

### Link and format checks (L0)

- `git diff --check` -> PASS.
- All four external references return HTTP 200: the Auto-review manual and the
  three `openai/codex` blob URLs pinned at commit
  `5d89ab65dc9d4d0c55796c11df112b54157922b4`.
- The archive citations resolve: `docs/archive/plans/price-coverage.md:45`
  carries the 84,905,101 row, and its Out of Scope section carries the
  "most likely a Codex-internal pseudo-model" wording the task exists to test.
- The pinned expectation the record describes is present:
  `internal/usage/bundled_coverage_test.go:49` still holds
  `{client: "codex", model: "codex-auto-review", wantPriced: false}` with the
  inline reason at `:21`.

### Findings

- **[P2] The plan states two magnitudes for the same quantity, 63% apart, and
  reconciles neither.** The intro still reads "accounts for roughly 85 M tokens
  of real usage" (`:14-15`) and the Goal commits to "Keep the 85 M tokens
  visible" (`:31`), while the record this task added measures 138,434,245 input
  plus 197,602 output — 138,631,847 on the same basis. Both figures are
  individually correct point-in-time snapshots, which the plan never says:
  cumulative input+output for this model reached exactly 84,905,101 through
  2026-07-21, and the retired price-coverage plan records "Measured 2026-07-22".
  The gap is thirteen days of accumulation, not a measurement-basis difference.
  Two further details compound it: the Evidence Baseline header says "Gathered
  on 2026-08-02 at `308feb0`" while the 84,905,101 figure inside it dates from
  2026-07-22, and the probe block's own stale-snapshot caveat is about WAL
  visibility, so it does not cover this. The task's whole product is a durable
  evidence record that outlives the plan into the Backlog; shipping it with an
  unexplained 63% internal contradiction in its headline number means a future
  reopen carries the stale magnitude into its reasoning. Fix is
  documentation-only: date-qualify the 85 M references, or restate them at the
  measured current figure, and note that the two snapshots share a basis.

- **[P3] `docs/README.md:426` drops the status marker its siblings carry.** The
  two other `v0.3.0` plan-index entries end `active — 3/4 done` (`:424`) and
  `active — 2/2 done` (`:425`); the rewritten classification entry ends
  "evidence awaiting review" with no `active` marker and no done-count. The
  documentation convention asks that substantial documents be marked with a
  status, and the plan is still `status: active` in its own frontmatter.

- **[P3] `docs/plans/runtime-provider-attribution.md:311` now asserts something
  false, and its mirror sentence was corrected in this same change.** That line
  reads "three plans ship behavior changes in `v0.3.0`"; with this plan's
  behavior task `n/a`, only two do. The parallel claim in `docs/README.md:386`
  was updated here from "all three plans' behavior changes" to "the batch's
  behavior changes", so the asymmetry is a miss rather than a deliberate
  deferral. Mitigating: `:323` of that plan already anticipates this outcome
  ("or nothing if that plan concluded inconclusive"), and task 4
  `v0-3-0-contract` explicitly owns the final "`docs/README.md`, all three
  plans' status matrices, and the review records agree" sweep. The "all three
  plans" phrasings at `:317`, `:347`, and `:349` stay accurate, since all three
  plans still coordinate and retire together. Correct here or route explicitly
  to task 4; do not leave it unrecorded.

- **[nit] 2,194 target events carry only 2,136 distinct event IDs.** The record
  states both numbers without remarking on the 58 duplicates. Harmless to the
  verdict — the correlation counts that matter are per-event — but a reader
  reconciling the two columns will stop on it. One clause would settle whether
  duplicate event IDs are expected for this label.

### Evidence

- Re-ran the plan's documented aggregate query verbatim against
  `~/.agentdeck/agentdeck.sqlite3` with `mode=ro&immutable=1` and
  `PRAGMA query_only=ON` -> every recorded figure matched exactly.
- `ls ~/.agentdeck/` after the re-run -> no WAL/SHM sidecars.
- Snapshot reconciliation, same read-only aggregate form: cumulative
  `input_tokens+output_tokens` for `codex`/`codex-auto-review` by day ->
  84,905,101 through 2026-07-21 and 138,631,847 through 2026-08-04; first
  event day 2026-06-17.
- `git status --porcelain --untracked-files=all` -> only the two intended
  modified documents; no probe artifact.
- `git diff --check` -> PASS.
- `curl -o /dev/null -w %{http_code} -L` on all four external references -> 200.
- `grep -nE "/Users/|/home/|sess[-_]|rollout-|[0-9a-f]{8}-[0-9a-f]{4}-"` over the
  plan -> no match.

### Verdict

**REOPEN.** All four acceptance criteria pass and the inconclusive conclusion is
the right call on this evidence — the probe reproduces to the digit and leaked
nothing. The task is held open only for the P2: the document it exists to
produce contradicts itself on the number that motivates the whole plan. That is
a documentation-only fix. `Dev` reverts to unticked in the Status matrix;
`Review` stays unticked. The two P3s should close in the same pass, or the
second be routed to `v0-3-0-contract` on the record.

## Round 2 — 2026-08-04

- Reviewed state: worktree on `63c9b97`;
  `docs/plans/codex-auto-review-classification.md` blob
  `f460f6ddfa985e84219e6b9059eff5359d171acc`, `docs/README.md` blob
  `9028e6de2d5a519de959ae0ceedf8042f714ab60`,
  `docs/plans/runtime-provider-attribution.md` blob
  `3eb121bc2e510651c01a12f092ee75e04c900cef`. All uncommitted.
- Reviewer: Claude Opus 5 (independent re-review)
- Scope: closure of the three Round 1 findings and the nit, plus the two new
  technical assertions the fix introduced, plus a check for regressions in the
  evidence record. Product code remains out of scope; this task ships none.

### Round 1 findings

**[P2] Closed, and closed at the root cause.** The fix does not merely soften
the wording — it adds a dedicated **Snapshot reconciliation** paragraph
(`:56-63`) that names both observations, their shared basis, and the arithmetic
between them, and it re-anchors the two loose references: the intro now reads
"In the historical 2026-07-21 snapshot … 84,905,101 `input_tokens +
output_tokens`" (`:13-15`), and the Goal's normative commitment is now "Keep
every recorded token visible" (`:32-33`), which no longer pins a magnitude that
will keep drifting. Each figure independently verified:

- 84,905,101 is exactly the cumulative `input_tokens+output_tokens` for
  `codex`/`codex-auto-review` through 2026-07-21. The retired plan's table
  column is labelled only "Tokens" and never states a basis, so the basis is an
  inference — but a nine-digit exact match admits no competing basis, and I
  reproduced it independently.
- 138,631,847 = 138,434,245 + 197,602. Confirmed.
- The stated delta 53,726,746 = 138,631,847 - 84,905,101. Confirmed.

The claim that `cached_input_tokens` is "a component, not an additional addend"
is not just numerically plausible, it is enforced: `internal/usage/usage.go:461`
rejects any event with `cached_input_tokens > input_tokens`, and `:464`/`:467`
price `input_tokens - cached_input_tokens` and `cached_input_tokens` as the two
disjoint halves of input. Zero rows in the real database violate the invariant.
Adding this sentence was the right call — without it a reader could reasonably
have summed all three fields.

**[P3] Closed.** `docs/README.md` restores the status marker, now reading
`active — 1/1 Dev complete; re-review required`. That was accurate for the state
in which it was written; it needs one more update now that this round passes,
handled below.

**[P3] Closed.** `docs/plans/runtime-provider-attribution.md:311` now reads "the
three plans coordinate one release, while two ship behavior changes and the
classification plan may ship nothing when its evidence is inconclusive". This is
the stronger of the two options I offered — it fixes the false claim in place
rather than deferring it — and it keeps the surrounding rationale intact, since
the "three increments would collide" argument was always about the three
original contract tasks, not about three behavior deliveries. The "all three
plans" phrasings at `:317`, `:347`, and `:349` correctly remain, as they
describe coordination and retirement rather than behavior.

**[nit] Closed, and the explanation is verifiably correct rather than merely
plausible.** `:151-154` attributes the 2,194-vs-2,136 gap to `event_key` being
the stored row primary key. The live schema confirms it exactly:
`usage_events(event_key TEXT PRIMARY KEY, …, event_id TEXT NOT NULL, …)` — a
non-unique `event_id` alongside a distinct primary key. The added clause that
"the aggregate intentionally retains every row's stored components" also
forestalls the natural follow-up question of whether the token sums
double-count.

### Regression and new-issue check

- The probe still reproduces exactly against the live database: 2,194 / 93 /
  2,136 / 138,434,245 / 123,631,488 / 197,602, with `input+output` =
  138,631,847. The fix changed no recorded figure.
- No new external references were introduced; the previously verified links
  still return 200.
- `git diff --check` -> PASS. No stray or untracked probe artifacts.
- Leak scan for paths, session identifiers, and UUID-shaped strings -> clean.
  The reconciliation paragraph added only aggregates and arithmetic.
- The four acceptance criteria all still hold; nothing in the fix touched the
  probe method, the inconclusive conclusion, or the Status matrix semantics.

Observation, not a finding: the Evidence Baseline still opens "Gathered on
2026-08-02 at `308feb0`" while the section now carries 2026-07-21, 2026-08-02,
and 2026-08-04 material. In Round 1 this compounded the P2; it no longer does,
because every figure inside the section is now individually dated at the point
of use. Left as-is deliberately — the header marks when the baseline was
established, and rewriting it would gain nothing.

### Evidence

- Re-ran the aggregate probe against `~/.agentdeck/agentdeck.sqlite3` with
  `mode=ro&immutable=1` -> all recorded figures unchanged.
- Independent basis check: cumulative `input_tokens+output_tokens` through
  2026-07-21 -> 84,905,101, matching the historical figure exactly.
- `SELECT COUNT(*) … WHERE cached_input_tokens > input_tokens` -> 0.
- `.schema usage_events` -> `event_key TEXT PRIMARY KEY`, `event_id TEXT NOT
  NULL`, confirming the nit's explanation.
- `internal/usage/usage.go:461,464,467` -> the cached-input-as-component
  invariant and its pricing split.
- `git diff --check` -> PASS; `git status --porcelain --untracked-files=all` ->
  only the three intended documents plus this review record.
- `curl -o /dev/null -w %{http_code} -L` on the external references -> 200.

### Verdict

**PASS.** All three Round 1 findings and the nit are closed at the root cause,
not papered over, and the two technical assertions the fix introduced both hold
up against the schema and the pricing code rather than resting on the numbers
looking consistent. The evidence record is now internally coherent, correctly
dated, and reproducible. `Review` is ticked; task 2 stays `n/a` and the
unresolved classification stays in the Backlog.

Retirement is deliberately **not** performed here: this plan's archival is owned
by `v0-3-0-contract` in the runtime attribution plan (`:349`), which retires all
three `v0.3.0` plans and their review directories in one pass.
