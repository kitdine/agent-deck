---
status: active
created: 2026-08-02
---

# codex-auto-review Classification

Target release: `v0.3.0`.

The 2026-08-04 evidence probe was inconclusive about billing, so the behavior
task was dropped and the unresolved classification returned to the Backlog.

Promoted out of the `docs/README.md` Backlog on 2026-08-02. In the historical
2026-07-21 snapshot, `codex-auto-review` accounted for 84,905,101
`input_tokens + output_tokens` and was deliberately unpriced. The
open question is not what it costs — it is what it *is*, because "unpriced"
currently means two different things at once: a model AgentDeck should price and
cannot, and a model that has no price to have.

Contract text is **not** owned here. It is recorded by the single `v0.3.0`
contract task in
[the runtime provider attribution plan](runtime-provider-attribution.md), so the
release increments the specification version exactly once.

## Goal

- Establish, from evidence, whether `codex-auto-review` is a billable model, a
  Codex-internal pseudo-model, or a label attached to work billed under another
  model.
- Make `unpriced` mean one thing again: a model that should have a price and
  does not.
- Keep every recorded token visible. Whatever the classification, the tokens
  are real and must not silently disappear from usage reporting.

## Non-Goals

- No price for `codex-auto-review`, invented, estimated, or borrowed. The
  price-coverage plan settled that this is an attribution question, not a
  catalog-coverage one.
- No change to the price catalog, its generator, or its pinned LiteLLM commit.
- No change to how any other model is priced or classified.
- No re-parse or rewrite of stored events. Classification is applied at read
  time, like pricing.

## Evidence Baseline

Gathered on 2026-08-02 at `308feb0`.

**What the retired price-coverage plan established.** `codex-auto-review`
carries 84,905,101 tokens and is "absent entirely" from every pricing source
checked (`docs/archive/plans/price-coverage.md:45`), and was deliberately left
out of scope as "most likely a Codex-internal pseudo-model rather than a
billable one" (lines 759-762). That plan priced `gpt-5.3-codex-spark` through a
disclosed equivalent estimate and explicitly declined to do the same here.

**Snapshot reconciliation.** The 84,905,101 historical figure is the
`input_tokens + output_tokens` total from the retired plan's 2026-07-21
snapshot. The 2026-08-04 probe records the same two fields at 138,631,847, an
increase of 53,726,746 over the later observation window. Its separately
reported `cached_input_tokens` is a component, not an additional addend to
either comparison. The current probe uses an immutable main-database snapshot
and can omit live-WAL rows, so neither number is a live-current total; they are
dated, same-basis snapshots rather than contradictory magnitudes.

**What is pinned today.** `internal/usage/bundled_coverage_test.go` lists
`{client: "codex", model: "codex-auto-review", wantPriced: false}` in
`coldStartModels`, with the reason recorded inline. The cold-start guarantee
asserts that a fresh network-free install prices every model in that table
according to its `wantPriced` flag, so the current expectation is pinned by a
shipped test.

**A consequence that is easy to miss.** `codex-auto-review` is also used as the
generic *unpriced model* fixture across the CLI test suite:

| Site | Role |
| --- | --- |
| `internal/usage/usage_test.go:732,748` | Proves `Models[]` reports `UnpricedEvents` |
| `internal/usage/usage_test.go:2415,2878` | Unpriced event in coverage and stats fixtures |
| `cmd/agentdeck/main_test.go:1263,1270` | Proves the model-coverage table renders an unpriced row |
| `cmd/agentdeck/usage_stats_text_test.go:31,1197` | Proves the stats text model section renders it |

If it is reclassified as non-billable, those fixtures stop exercising the
unpriced-model path. Substituting a genuinely unpriced model name in them is
part of this work, not an incidental edit — otherwise the change silently
deletes coverage of the rendering path it claims not to affect.

**2026-08-04 classification probe.** The probe emitted aggregate counts only
and wrote no output file. Its output contained no session text, prompts, tool
arguments, session IDs, event IDs, timestamps, source paths, credentials, or
other per-session data. An ordinary SQLite `mode=ro` open was unavailable in
the managed environment. The successful bounded fallback queried the main
database with `immutable=1`; that avoids creating WAL/SHM sidecars but can omit
committed rows still present only in a live WAL. These figures are therefore an
aggregate, possibly stale snapshot rather than proof of live-current totals.

The exact aggregate query was:

```bash
rtk proxy env NO_RTK=1 sqlite3 -json \
  'file:'"$HOME"'/.agentdeck/agentdeck.sqlite3?mode=ro&immutable=1' \
  "PRAGMA query_only=ON;
   WITH target AS (
     SELECT * FROM usage_events
     WHERE client='codex' AND model='codex-auto-review'
   ), other AS (
     SELECT * FROM usage_events
     WHERE client='codex' AND model<>'codex-auto-review'
   ), adjacent AS (
     SELECT o.model, COUNT(*) AS pairs
     FROM target t JOIN other o
       ON o.session_id=t.session_id
      AND ABS(unixepoch(o.event_at)-unixepoch(t.event_at))<=5
     GROUP BY o.model
   )
   SELECT COUNT(*) AS target_events,
          COUNT(DISTINCT session_id) AS target_sessions,
          COUNT(DISTINCT event_id) AS target_event_ids,
          COALESCE(SUM(input_tokens),0) AS input_tokens,
          COALESCE(SUM(cached_input_tokens),0) AS cached_input_tokens,
          COALESCE(SUM(output_tokens),0) AS output_tokens,
          COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens,
          COALESCE(SUM(cache_creation_tokens),0) AS cache_creation_tokens,
          COALESCE(SUM(cache_write_5m_tokens),0) AS cache_write_5m_tokens,
          COALESCE(SUM(cache_write_1h_tokens),0) AS cache_write_1h_tokens,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id)) AS with_other_session,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id
                       AND o.event_id=t.event_id)) AS with_other_event_id,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id
                       AND ABS(unixepoch(o.event_at)-unixepoch(t.event_at))<=1)) AS within_1s,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id
                       AND ABS(unixepoch(o.event_at)-unixepoch(t.event_at))<=5)) AS within_5s,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id
                       AND ABS(unixepoch(o.event_at)-unixepoch(t.event_at))<=60)) AS within_60s,
          SUM(EXISTS(SELECT 1 FROM other o
                     WHERE o.session_id=t.session_id
                       AND ABS(unixepoch(o.event_at)-unixepoch(t.event_at))<=300)) AS within_300s,
          (SELECT json_group_object(model,pairs) FROM adjacent)
            AS adjacent_models_within_5s,
          (SELECT COUNT(*) FROM model_prices
           WHERE model='codex-auto-review'
              OR EXISTS (SELECT 1 FROM json_each(model_prices.aliases_json)
                         WHERE value='codex-auto-review')) AS local_exact_price_rows
   FROM target t;"
```

It returned 2,194 target event rows across 93 sessions and 2,136 distinct
event IDs. `event_key`, not `event_id`, is the stored row primary key, so the
58-row difference is expected from counting event rows versus distinct IDs;
the aggregate intentionally retains every row's stored components. Those
components were 138,434,245 input, 123,631,488 cached input,
197,602 output, and zero for cache read, cache creation, five-minute cache
write, and one-hour cache write. Of the target events, 1,934 occurred in a
session containing another model, but zero shared an event ID with another
model. Another-model events existed within 1, 5, 60, and 300 seconds for 735,
1,545, 1,933, and 1,933 target events respectively. The five-second join
produced aggregate target/other pairs for `gpt-5.4` (103), `gpt-5.4-mini` (11),
`gpt-5.5` (402), `gpt-5.6-luna` (19), `gpt-5.6-sol` (833), and
`gpt-5.6-terra` (339). Pair totals can exceed target-event totals because one
target event can have more than one nearby other-model event. The local price
table contained zero exact or alias rows.

Current official Codex evidence establishes the role of the label, but not its
billing treatment. The published Auto-review manual describes a separate
reviewer agent that receives a compact transcript plus the exact approval
request and decides whether the requested action should run:
<https://learn.chatgpt.com/docs/sandboxing/auto-review.md>. At official Codex
commit `5d89ab65dc9d4d0c55796c11df112b54157922b4`, an exact repository search
returned six `codex-auto-review` hits. The source defines it as the default
preferred automatic-approval review model, includes it as a model-catalog slug,
and builds a dedicated read-only guardian review session:

- <https://github.com/openai/codex/blob/5d89ab65dc9d4d0c55796c11df112b54157922b4/codex-rs/model-provider/src/provider.rs>
- <https://github.com/openai/codex/blob/5d89ab65dc9d4d0c55796c11df112b54157922b4/codex-rs/models-manager/models.json>
- <https://github.com/openai/codex/blob/5d89ab65dc9d4d0c55796c11df112b54157922b4/codex-rs/core/src/guardian/review_session.rs>

The exact source-count command was:

```bash
rtk proxy gh api \
  'search/code?q=%22codex-auto-review%22+repo%3Aopenai%2Fcodex' \
  --jq '{official_source_exact_hits: .total_count}'
```

No pricing source checked on 2026-08-04 had acquired the label. The bundled
catalog, curated gap-fill entries/pending list, local price table, and current
LiteLLM `main` catalog each returned zero exact or alias matches. The live
upstream check streamed the response without writing it to disk:

```bash
rtk proxy curl --fail --silent --show-error --location \
  https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json \
  | rtk proxy jq \
    '{litellm_exact_key_hits: (if has("codex-auto-review") then 1 else 0 end),
      litellm_alias_hits: ([to_entries[]
        | select((.value.aliases // []) | type == "array")
        | select(.value.aliases | index("codex-auto-review"))] | length)}'
```

**Result: inconclusive.** The evidence proves that `codex-auto-review` is a
dedicated automatic-approval reviewer model/session label with independently
reported token events. Same-session and adjacent-time correlation does not
show whether those tokens are free, separately billed, or charged under a
nearby model; absence from public and local price catalogs also does not prove
non-billability. Selecting Branch A or Branch B would therefore exceed the
evidence. Task 2 is `n/a`, no behavior ships, and the unresolved item returns
to the Backlog. Branch A would require authoritative billing/account evidence
that reviewer tokens are not separately billable. Branch B would require an
authoritative billing mapping or matched account-level charge evidence naming
the underlying model and attribution rule.

## Decision

The classification itself is decided by task 1's evidence. What is decided now
is the **shape of the outcome**, so task 2 is not open-ended.

Exactly one of two branches ships:

**Branch A — non-billable internal label.** If the evidence shows the tokens are
not separately billable, AgentDeck classifies the model as non-billable:

- its tokens remain in token totals and in the model list, so nothing
  disappears;
- it is excluded from `unpriced_models` and from the incomplete-cost warning set,
  so coverage percentages reflect only genuine pricing gaps;
- it renders with an explicit non-billable marker, not with a blank or a zero
  cost, because a zero would claim knowledge the classification does not have;
- the classification lives in curated data next to the existing gap-fill file,
  not in a code branch on a model name, so a future label needs a data change
  rather than a release.

**Branch B — billable under an underlying model.** If the evidence identifies the
real model that the work is billed as, the events are attributed to it at read
time, with the derivation disclosed exactly as an estimate is disclosed today,
and the label retained so the origin stays visible.

If task 1 settles neither — the evidence is inconclusive — the honest outcome is
to ship nothing, record the probe and its result in this plan, and return the
item to the Backlog. That is an acceptable, explicitly allowed outcome, not a
failure to be worked around by guessing.

Either shipping branch changes `usage stats` coverage output and the pinned
cold-start expectation, which is why this is `v0.3.0` and not a patch.

## Tasks

### 1. `classification-evidence`

Determine what `codex-auto-review` is, and record the answer in this plan.

- Probe the real local Codex logs for: event count and token components; whether
  a `codex-auto-review` event shares a logical session and adjacent timestamps
  with events under a billable model; whether the label appears in Codex's own
  documentation or source; and whether any pricing source has added it since the
  price-coverage plan checked.
- The probe follows the precedent set by the cache-creation probe: **aggregate
  counts only**. No session text, prompt, tool argument, path, or credential may
  be emitted, recorded in this plan, or written to any file.
- Record the method, the aggregate results, and the resulting branch selection
  (A, B, or inconclusive) in this plan's Evidence Baseline, including what would
  change the answer.

Acceptance:

- The recorded evidence is reproducible from the stated method.
- The selected branch follows from the evidence rather than from the earlier
  "most likely a pseudo-model" assumption, which this task exists to confirm or
  overturn.
- No session content of any kind appears in the plan, the repository, or the
  command output.
- If the outcome is inconclusive, the plan says so and task 2 is dropped rather
  than reinterpreted.

Verification: L0 for the recorded document. The probe ships no code, but it
reads real local Codex logs, so the L0 format/link/diff checks do not cover its
two real risks. Satisfy them explicitly: state the exact commands or query in
the plan so the numbers are reproducible, and confirm from the emitted output
itself that only aggregate counts left the probe - no session text, prompt, tool
argument, path, or credential, in the plan, the terminal, or any file the probe
wrote.

### 2. `classification-behavior`

Implement the branch task 1 selected.

- Apply the classification at read time, in the shared pricing read path
  delivered by the `v0.2.2` scalability plan. Do not re-parse or rewrite stored
  events.
- Never branch on a model name in code. Branch A's classification is curated
  data; Branch B's mapping is curated data.
- Update `coldStartModels` in `internal/usage/bundled_coverage_test.go` to the
  new expectation, keeping the inline reason accurate.
- Substitute a genuinely unpriced model name in the four fixture sites listed in
  the Evidence Baseline, so the unpriced-model reporting and rendering paths stay
  covered.

Acceptance:

- Token totals for affected data are unchanged; only classification, coverage,
  and cost reporting change.
- `unpriced_models` no longer contains a model that has no price to have
  (Branch A), or contains it no longer because it is now attributed (Branch B).
- The unpriced-model reporting and rendering paths are still exercised by tests
  after the fixture substitution.
- The cold-start guarantee passes with the updated expectation and still fails if
  the bundled catalog decays.
- Release notes state that coverage percentages and the unpriced-model list
  change for existing data.

Verification: L2. Targeted `internal/usage` and `cmd/agentdeck` tests, then the
full vendor suite.

Prerequisite: task 1's branch selection, and `shared-read-resolver` in the
`v0.2.2` scalability plan (**satisfied on 2026-08-03**).

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `classification-evidence` | [x] | [x] |
| 2. `classification-behavior` | n/a — task 1 inconclusive | n/a — task 1 inconclusive |

Task 2 does not start before task 1 is reviewed, and does not start at all if
task 1 concludes inconclusive. In that case mark task 2's `Dev` and `Review`
cells `n/a` with a pointer to task 1's recorded conclusion, rather than leaving
them unchecked as if the work were still pending, and return the item to the
Backlog in `docs/README.md`.

Commit boundaries follow task boundaries: one commit per task.

## Starting a task

> 进入开发：`codex-auto-review-classification` / `<task-anchor>`

Read `AGENTS.md`, this plan's Evidence Baseline and Decision, the retired
price-coverage plan's Out of Scope section, `internal/usage/bundled_coverage_test.go`,
and the verification routing for the level the task declares. Tick `Dev` after
the task's own evidence passes; an independent reviewer records a PASS round
under `docs/reviews/codex-auto-review-classification/<task-anchor>.md` before
ticking `Review`.
