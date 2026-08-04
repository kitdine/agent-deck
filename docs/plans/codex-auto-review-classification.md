---
status: active
created: 2026-08-02
---

# codex-auto-review Classification

Target release: `v0.3.0`.

Promoted out of the `docs/README.md` Backlog on 2026-08-02. `codex-auto-review`
accounts for roughly 85 M tokens of real usage and is deliberately unpriced. The
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
- Keep the 85 M tokens visible. Whatever the classification, the tokens are real
  and must not silently disappear from usage reporting.

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

**What is not yet known**, and what task 1 exists to settle: whether
`codex-auto-review` events co-occur with a real model inside the same logical
turn, what token components they carry, whether they appear under sessions that
also carry billable events, and whether Codex documents the label anywhere.

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
| 1. `classification-evidence` | [ ] | [ ] |
| 2. `classification-behavior` | [ ] | [ ] |

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
