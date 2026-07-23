---
status: historical
created: 2026-07-22
retired: 2026-07-23
---

# Price Catalog Coverage Plan

**Specification:** `docs/specs/cli-design.md` ("Price Catalog": the LiteLLM
importer contract, the `bundled`/`litellm`/`official` provenance layers, and
"A model absent from both historical and current local catalogs remains
unpriced.")

**Goal:** Make a freshly installed AgentDeck price the models people actually
run, and make a model that upstream never prices — `gpt-5.3-codex-spark` is the
reported case — priceable out of the box rather than only after a hand-applied
local `price override`.

Promoted out of the `docs/README.md` backlog item "Broaden the bundled fallback
price catalog", which described the cold-start half of this problem. The
reported Spark case proved there is a second, permanent half that a bigger
bundled catalog alone does not fix.

## Measured Baseline

Measured 2026-07-22 against the real state database (48.6 MB, 15 distinct
models, 5.19 B tokens, `usage stats --period all --format json`) and against the
LiteLLM catalog at `main` (1.66 MB snapshot).

**Cold start — what a fresh install can price before any network call:**

| | Tokens | Share |
| --- | ---: | ---: |
| Priceable from the bundled catalog | 388,030,488 | **7.5%** |
| Not priceable until `price update` succeeds | 4,797,768,283 | **92.5%** |

`internal/usage/model-prices.json` ships exactly two models (`gpt-5.4`,
`claude-sonnet-4-6`). Thirteen of the fifteen models in real use are absent.

**After a successful `price update` — what remains permanently unpriced:**

| Model | Tokens | LiteLLM status |
| --- | ---: | --- |
| `gpt-5.3-codex-spark` | 69,406,886 | present **only** as `chatgpt/gpt-5.3-codex-spark`, which carries no cost fields at all |
| `codex-auto-review` | 84,905,101 | absent entirely (likely a pseudo-model, see Out of Scope) |

Every other model in use imports cleanly from LiteLLM once the network call
succeeds, including all `gpt-5.6-*`, `gpt-5.5`, `claude-opus-4-8`,
`claude-sonnet-5`, and the dotted `claude-haiku-4.5` / `claude-opus-4.8`
spellings that the existing `claude-` punctuation rule already normalizes.

**Why Spark is not a LiteLLM import bug.** LiteLLM's whole `chatgpt/` namespace
is a subscription-surface listing: all ten entries (`chatgpt/gpt-5.4`,
`chatgpt/gpt-5.3-codex`, `chatgpt/gpt-5.3-codex-spark`, …) carry zero cost
fields. Relaxing the importer's `openai`/`anthropic` provider filter to admit
them would import ten models with no prices and change nothing. The `openai`
namespace prices `gpt-5.3-codex` but has no `-spark` row. `price update` can
therefore never price this model, no matter how often it runs.

## What Is Not the Problem

- **The importer's provider filter.** It correctly rejects a namespace that has
  no prices. Do not widen it.
- **The `official` override path.** It already works — this is exactly how the
  gap was closed by hand on one machine. The defect is that it is a local,
  per-machine action that no fresh install inherits.
- **Text rendering.** `PRICED` / `UNPRICED` display belongs to
  `plans/usage-stats-readability.md`; this plan changes what data exists, not
  how it is drawn. The two plans do not overlap in files.

## Root Causes

**1. The bundled catalog is a two-model stub.** It was seeded once and never
grown, so cold start is 7.5% covered. There is no defined regeneration owner,
source, or cadence — the original backlog item's open question.

**2. There is no durable home for a vendor price that upstream lacks.** The
three layers are `bundled` (ships with the binary), `litellm` (network), and
`official` (hand-imported, local-only). A model upstream never prices can only
live in `official`, which does not ship. Nothing in the repository can carry it.

**3. Regenerating the bundled catalog would delete the fix.** This is the
constraint that shapes the design: if `model-prices.json` is regenerated from
LiteLLM to fix root cause 1, `gpt-5.3-codex-spark` is silently dropped again,
because LiteLLM has no priced row for it. A generated catalog and a curated
gap-fill must therefore be **separate inputs**, merged by the generator, with
the curated set never overwritten by regeneration.

**4. A changed bundled file does not fully reach an upgrading install.**
`importCatalog` uses `INSERT OR IGNORE` keyed on `version TEXT PRIMARY KEY` and
`PRIMARY KEY(catalog_version, model)` (`internal/store/migrations.go:44-45`,
`internal/usage/usage.go:498-505`). Verified against SQLite with the real DDL:

| Change to the bundled file, version string unchanged | Result on an existing install |
| --- | --- |
| New model added | inserted correctly |
| Existing model's price changed | **silently ignored — stale price kept** |
| File content changed | **`content_sha256` keeps describing the old file** |

So adding Spark alone would reach upgraders, but any repricing would not, and
the stored provenance hash becomes a lie either way. The version string must
change whenever the file's content changes. Nothing enforces that today, and
the current string `2026-07-13-openai-standard-v1` already misdescribes its own
contents by carrying an Anthropic model.

## Price Confidence for `gpt-5.3-codex-spark`

Every source that has an opinion agrees on **$1.75 / $14.00 / $0.175** per 1M
tokens (input / output / cached input), but the corroboration is weaker than the
agreement suggests, and the design must treat it accordingly:

| Source | Spark entry | Note |
| --- | --- | --- |
| models.dev | 1.75 / 14 / 0.175 | same figures **and same `release_date` (2026-02-05)** as `gpt-5.3-codex`; looks inherited from the parent model, not independently confirmed |
| codeburn `pricing-fallback.json` | 1.75 / 14 / 0.175 | gap-fills from models.dev — **not an independent source** |
| typingmind cost calculator | 1.75 / 14 / 0.175 | cites openai.com generically, no specific rate-card citation |
| OpenRouter API | **absent** | prices `openai/gpt-5.3-codex` only |
| LiteLLM (priced namespace) | **absent** | see baseline above |
| OpenAI rate card / API pricing page | **unverified** | both return HTTP 403 to automated fetch from this environment |

The figures are also exactly equal to the parent `gpt-5.3-codex`. That is
plausible for a Spark tier, but it is equally consistent with every aggregator
having copied the parent row. **Treat the number as unconfirmed until a human
checks OpenAI's published rate card.** Wikipedia gives Spark a 2026-02-12
release, a week after the 2026-02-05 date models.dev reuses for it, which is one
more reason not to trust the inherited row's metadata. Shipping this figure
without that check would make the bundled layer a silent guess, which the spec's
provenance contract forbids.

Superseded by `## Follow-Up — 2026-07-23: Spark Equivalent Estimate`: the figure
still ships as an explicitly marked equivalent estimate borrowing the vendor-
priced predecessor's rate, never as a confirmed Spark vendor rate. The
reasoning above is why it is disclosed as an estimate rather than presented as
a price.

## Tasks

Task content lives here; per-gate status lives in the matrix below.

1. **version-guard** — Enforce root cause 4 before any bundled data change
   ships. Derive or verify the bundled catalog's `catalog_version` against the
   embedded file's content hash so a content change without a version change
   fails loudly at build or test time rather than silently keeping stale prices
   on upgraded installs. Add a regression test that mutates a bundled price with
   an unchanged version and asserts the failure. **Prerequisite for tasks 2
   and 4** — without it, neither reliably reaches an existing install.
   **Completion note (2026-07-23):** Added `internal/usage/bundled_catalog.go`:
   `BundledCatalogVersionDigest` hashes the catalog's canonical JSON with
   `catalog_version` blanked (the field cannot feed a digest it carries) and
   object keys sorted, so the digest tracks semantic content — models,
   providers, prices, aliases, effective dates — and ignores byte-level
   formatting. `BundledCatalogVersionFor` stamps `<prefix>-g<digest12>`;
   `VerifyBundledCatalogVersion` rejects a catalog whose version omits or
   staledates that digest, naming the exact version string it should carry.
   The raw-file `content_sha256` recorded at import is a separate value and is
   unaffected by this normalization.
   Tests in `internal/usage/bundled_catalog_test.go`:
   `TestBundledCatalogVersionCarriesItsContentDigest` is the shipping guard on
   the embedded catalog; `TestBundledCatalogVersionGuardRejectsChangedPriceWithUnchangedVersion`
   is the regression the plan asks for — it mutates a bundled price, keeps the
   version, requires a `stale content digest` failure, then restamps and
   requires acceptance, so the guard demands a version bump rather than
   forbidding the edit;
   `TestBundledCatalogVersionDigestIgnoresFormattingButTracksContent` pins the
   normalization from both sides (reformatting and renaming the version do not
   move the digest; adding a model does).
   RED evidence is genuine rather than synthetic: the guard's first run failed
   against the then-current hand-written catalog with
   `bundled catalog version "2026-07-13-openai-standard-v1" carries no "-g"
   content digest`, i.e. the defect root cause 4 describes was live in the repo
   and the guard caught it. It went green once task 4's generator stamped a
   content-derived version.
2. **spark-gapfill** — Confirm the Spark rate against OpenAI's published rate
   card (a human step; the page is 403 to automated fetch — see Price
   Confidence), then add `gpt-5.3-codex-spark` to the bundled catalog as an
   `openai` model with `input` / `cached_input` / `output` and its real
   `effective_from`. Do **not** ship the figure if the check fails or cannot be
   made; record what was verified and by whom in the review file. Note that
   `ImportBundledCatalog` derives the catalog's effective date from the
   *minimum* model `effective_from` (`usage.go:477-482`), so a February date
   pulls the whole bundled catalog's effective date back — each model still has
   its own `modelEffective` gate in `priceAt`, so this is expected to be safe,
   but assert it with a test rather than assuming.
   *(Task statement as originally written; kept for the record. The "expected
   to be safe" assumption was wrong and the hedge earned its keep — the catalog
   date is also a precedence key, not only a gate. `ImportBundledCatalog` no
   longer derives it from the models at all; see the Repair section under this
   task.)*
   **Status note (2026-07-23): BLOCKED on the human rate-card check; price
   deliberately NOT shipped.** The verification gate was attempted and failed
   exactly as this plan predicted: `https://openai.com/api/pricing/` returns
   **HTTP 403** to automated fetch, and `https://platform.openai.com/docs/pricing`
   returns HTTP 200 but is client-rendered — the served HTML contains **zero**
   occurrences of "spark". No machine-checkable confirmation of the rate is
   obtainable from this environment, so per this task ("Do not ship the figure
   if the check fails or cannot be made") and the invariant "Regeneration never
   invents a price", `gpt-5.3-codex-spark` remains **unpriced**.
   What was built instead, so the price is a one-line change once a human
   confirms it: `gpt-5.3-codex-spark` is recorded in the `pending` array of
   `internal/usage/price-gapfill.json` (task 3's curated input) carrying the
   reported 1.75 / 0.175 / 14 figures, what the check is blocked on, why the
   apparent multi-source agreement is not corroboration (every source traces
   back to models.dev, which reuses the parent `gpt-5.3-codex` row including
   its 2026-02-05 `release_date`, while Wikipedia dates Spark to 2026-02-12),
   and the exact steps to ship. `TestBundledGapfillMeetsCurationBar` fails if
   Spark is ever neither shipped nor tracked as pending, so the gap cannot be
   silently forgotten, and `TestBundledCatalogPricesRealModelsWithoutNetwork`
   asserts Spark is still unpriced — that assertion fails loudly if a price
   appears without the coldStartModels fixture and this plan being updated
   together.
   The effective-date hazard this task flags was addressed under task 4 rather
   than assumed: generated bundled models carry the stable
   `BundledCatalogEffectiveFrom` rather than generation time, precisely because
   `ImportBundledCatalog` derives the catalog effective date from the minimum
   model `effective_from`. A curated February date would therefore be safe for
   the catalog date, but the reverse hazard proved real and was caught by
   existing tests — see task 4's note.
   **To ship:** confirm the rate on OpenAI's published rate card, move the
   object from `pending` into `entries` with `source_url`, `verified_by`,
   `verified_on`, and `effective_from`, run `make prices-regen
   LITELLM_COMMIT=<sha>`, flip Spark's `wantPriced` to `true` in
   `coldStartModels`, and record who verified what in
   `docs/reviews/price-coverage/spark-gapfill.md`.
   **Completion note (2026-07-23, reopened under the equivalent-estimate
   contract).** The BLOCKED note above is history: it is the correct outcome of
   the old "vendor rate or nothing" contract, which the Follow-Up section
   replaced. Spark now ships as an explicitly marked estimate, not as a claimed
   OpenAI rate.
   Data: `gpt-5.3-codex-spark` moved from `pending` into `entries` in
   `internal/usage/price-gapfill.json` as `kind: equivalent_estimate` with
   `basis_model: gpt-5.3-codex`, `1.75` / `0.175` / `14` per 1M tokens,
   `effective_from` at its 2026-02-12 release, and a `pricing_note` stating in
   full that the value is an equivalent token cost, not an OpenAI price for
   this model and not what a Pro subscription bills.
   Schema (`price_gapfill.go`): `GapfillEntry` gains `kind`, `basis_model`, and
   `pricing_note`; `PriceKind()` defaults an absent kind to `vendor_rate` so no
   existing entry changes meaning. `ParseGapfill` requires an estimate to name
   a basis model other than itself and to carry a note, and rejects a
   `vendor_rate` that wears estimate metadata — a mislabelled row cannot borrow
   the disclosure without the constraints.
   The bar that actually prevents an invented figure lives where the upstream
   catalog is in hand: `validateGapfillEstimates`, called by
   `GenerateBundledCatalog` before the merge, requires the basis model to exist
   in the generated upstream catalog, to belong to the same vendor, and to
   carry **exactly** the rate the estimate claims (compared as decimals, so
   `1.75` and `1.750000000` agree). If upstream ever reprices `gpt-5.3-codex`,
   regeneration fails and names both figures rather than letting the estimate
   drift into an unsourced number.
   Disclosure (no migration, per the approved contract): `EffectivePrice` gains
   `price_kind` / `basis_model` / `pricing_note`, all `omitempty`, and
   `markEstimatedPrices` derives them from the compiled gap-fill plus the
   provenance of the components that actually won. A row is marked only while
   at least one effective component still comes from the `bundled` layer, so a
   fresher `litellm` or `official` catalog covering every component retires the
   marker automatically. `price list` text appends `(ESTIMATED)` to the model
   and prints an `ESTIMATED PRICES` block naming the basis and the note;
   `usage stats`, summaries, and sessions are untouched.
   Merge consistency: `MergeGapfill` now renders curated values through the
   same `money(decimal(...))` the upstream conversion uses, so the artifact and
   `price list` cannot show `1.75` beside `1.750000000` for the same unit.
   Regenerated from the same pinned LiteLLM commit
   `f2479cc704f6e63d5510929d30ce8e11ffe43467`: **111 → 112 models**, version
   `bundled-litellm-merged-gc972a7bce902` → `-g32b44e2b8128`. Spark's model row
   carries its own 2026-02-12 release date, while the catalog's own effective
   date stays the stable `BundledCatalogEffectiveFrom` (capped by the current
   clock so it is never future-dated) — the two are deliberately independent,
   because the catalog date is a precedence key among same-layer catalogs and
   the model date is what gates history. The hazard this task flagged is
   therefore asserted rather than assumed, from both sides:
   `TestBundledCatalogDateIsIndependentOfItsCuratedModelDates` pins that a
   curated early date does not move the catalog date, and
   `TestNewerBundledCatalogOutranksAPreviouslyInstalledOne` pins the upgrade
   consequence that made the coupling matter. `price history` ordering is
   unchanged, and usage predating the catalog date still prices through the
   current-price fallback — measured, not assumed: a Spark event dated
   2026-03-01 prices at `15.750000000` for 1M input plus 1M output.
   The first implementation of this task did derive the catalog date from the
   minimum model date; see the Repair section below for why that was wrong and
   what replaced it.
   Tests: schema table cases for every estimate rule
   (`TestParseGapfillEnforcesCurationBar`), the basis contract from both sides
   (`TestValidateGapfillEstimatesRequiresARealVendorPricedBasis`,
   `TestGenerateBundledCatalogGuardsEquivalentEstimates`), the shipped-artifact
   guard (`TestBundledCatalogEstimateBorrowsItsBasisModelRate`), fresh-install
   pricing (`coldStartModels` flips Spark to `wantPriced: true`;
   `codex-auto-review` stays unpriced), the disclosure and its absence on a
   published rate (`TestPriceListDisclosesTheEstimateAndItsBasis`,
   `TestPriceListMarksEstimatedPricesAndNamesTheirBasis`), the automatic exit
   (`TestFresherUpstreamPricingRetiresTheEstimateMarker`, covering both full
   and partial upstream coverage), and the boundary
   (`TestUsageOutputNeverCarriesTheEstimateDisclosure`, which fails if stats or
   summary JSON ever carries `price_kind`, `basis_model`, `pricing_note`,
   `equivalent_estimate`, or `ESTIMATED`).
   RED evidence: removing the bundled-provenance gate made the fully-upstream
   case still report `estimate marker = true`; dropping the
   `validateGapfillEstimates` call accepted both a nonexistent basis and a
   drifted rate; removing the `basis_model` requirement accepted an estimate
   with none. All three restored and re-verified GREEN.
   The `price.list` JSON contract fixture
   (`cmd/agentdeck/testdata/phase7/gui-json-contract.json`) was regenerated:
   the change is purely additive — the estimated row's shape is new, the
   published-rate shape is unchanged, and no other command's contract moved.
   **Repair (2026-07-23, round 1):** closed all findings.
   [P1] The curated 2026-02-12 `effective_from` reached a place nobody looked:
   `ImportBundledCatalog` dated the *catalog* at its earliest model, and
   `priceLayerBefore` ranks same-layer catalogs by that date. Installed
   catalogs are outranked, never deleted, so this build's catalog (dated
   2026-02-12) lost to the v0.1.0 stub `2026-07-13-openai-standard-v1` on every
   model they share — the exact "upgrade silently keeps stale prices" failure
   task 1 exists to prevent, reintroduced through a data field. Reproduced with
   the real shipped stub before fixing: `gpt-5.4`'s three components all came
   back from the retired stub, and the counterfactual (regenerate with no
   curated entry → the byte-identical `c4231e75…` catalog) had the new catalog
   winning, so the cause was this task's data, not pre-existing behavior.
   Fixed by decoupling the two dates: the catalog's own effective time is
   `BundledCatalogEffectiveFrom`, still capped by the clock so it can never be
   future-dated, while each model keeps its own `effective_from`. The catalog
   date is a precedence key; the model date describes history. Spark therefore
   keeps its honest 2026-02-12 release date.
   [P2] `markEstimatedPrices` keyed the disclosure on `SourceKind == "bundled"`,
   so a price served by *any* bundled catalog would wear this binary's estimate
   note — and a database can hold bundled catalogs from several releases. Now
   matched against the compiled catalog's own `catalog_version`
   (`compiledBundledCatalogVersion`).
   [P2] Removed the unreachable alias index in `bundledEstimateEntries`: price
   rows are keyed on `model_prices.model`, so an alias can never be the model
   of an effective price. The comment now says so rather than the code
   implying a lookup that cannot happen.
   [P2] Corrected the re-measurement's method description above — this round
   never touched the real database (it reused the already-frozen snapshot), and
   its importer is a script reproducing `ImportBundledCatalog` rather than the
   product path, with byte-identical catalog reproduction plus exact figure
   reproduction as the equivalence evidence.
   [P3] `verified_by` for an estimate is now defined in the spec as naming
   whoever approved the estimate and its basis, which may be a project role —
   what an estimate attests to is a disclosed derivation, not a figure read off
   a vendor rate card. Also: wrap width became the documented
   `estimatedPriceNoteWidth`; `MergeGapfill`'s verbatim fallback for an
   unparseable value is explained; the `"ESTIMATED"` assertion documents why it
   is case-sensitive (summary legitimately carries `counts.estimated`, the
   unrelated multiplier quality); and `markEstimatedPrices` documents that
   failing loudly on an unparseable embedded input is deliberate.
   Tests: `TestNewerBundledCatalogOutranksAPreviouslyInstalledOne` is the
   upgrade regression (previous release's bundled catalog installed first with
   a conflicting price, then `ImportBundledCatalog`; every component must come
   from this binary's catalog).
   `TestBundledCatalogEffectiveDateFollowsItsEarliestModel` became
   `TestBundledCatalogDateIsIndependentOfItsCuratedModelDates`, asserting the
   catalog date is the stable constant *and* that the curated model row still
   carries 2026-02-12. `TestAnotherBundledCatalogsPriceIsNotMarkedAsThisEstimate`
   covers the version check. `TestOfficialOverridesRetainCatalogComponentsAndProvenance`
   reverted to its original `history[0].SourceKind == "official"` assertion,
   which the fix restores.
   RED: reverting the date fix failed both new P1 tests, naming the retired
   catalog and the 999 price it was still serving; dropping the version check
   failed the estimate-attribution test with `bundled-other-release` shown as
   the source. Both restored and re-verified GREEN.
   Verification (L3; catalog bytes unchanged at `d69fc03c…`, gap-fill unchanged
   at `1da8e3cf…`): `gofmt -l` clean, `go vet -mod=vendor ./...` clean,
   `go test -mod=vendor ./...` all packages ok,
   `make check-prices-reproducible` matches the recorded pin, both darwin
   cross-builds succeed, `make check-arm64-size` passes at 12,213,842 bytes
   against the 26,214,400 gate. The coverage figures were re-imported and
   re-measured under the repaired catalog-date rule and came back identical.
   **Repair (2026-07-23, round 2):** closed the three findings from the
   read-only review of the repaired state. None was a contract deviation.
   [known boundary, documented not changed] **The disclosure travels with the
   binary, not with the database.** Price rows live in `agentdeck.sqlite3` and
   move with a portable backup (`internal/backup/backup.go` packs `coreName`
   whole); the marker, basis, and note come from the running binary's compiled
   gap-fill. Demonstrated rather than argued: a binary built with an empty
   `entries` array, reading the same database, prints

   ```
   | openai   | gpt-5.3-codex-spark | 1.750000000 | 0.175000000 | 14.000000000 | - | - | - |
   ```

   with no marker and no `ESTIMATED PRICES` block, and its JSON row carries
   only `model`, `prices`, `provenance`, `provider`, `unit`. That is the
   accepted cost of the approved "derive it, no migration" contract — storing
   the disclosure is exactly the migration the contract avoids — so it is now
   written down in the spec (`### Curated Gap-Fill`, version 14) instead of
   being fixed. Re-open it as its own design change if it ever stops being
   acceptable.
   [output] The `(ESTIMATED)` suffix was polluting the MODEL cell, which is the
   value a user passes back to `price list <model>`; copying it from the table
   produced a name that does not resolve. The marker moved to its own `EST`
   column carrying `*`, and the note block below now leads with the same `*`.
   The column is added **only when a listing actually contains an estimate**,
   so an ordinary listing keeps byte-for-byte the table it always had.
   [robustness] `statsWrap` breaks on spaces and never splits a word, and it is
   shared with the usage stats renderer, so hard-folding there would change
   unrelated output. The constraint is enforced on the data instead:
   `estimatedPriceNoteWidth` documents that a `pricing_note` must not contain a
   token longer than the note width, and
   `TestEstimatePricingNotesFitTheDisclosureWidth` checks every shipped
   disclosure token — a URL dropped into a note is the realistic way this
   breaks, and it now fails a test rather than overflowing a line.
   Tests: `TestPriceListMarksEstimatedPricesAndNamesTheirBasis` now asserts two
   separable things through a new `priceTableRow` cell parser — the row's last
   cell is the `*` marker, and the MODEL cell equals exactly
   `gpt-5.3-codex-spark` — plus that a listing with no estimate grows no `EST`
   column.
   RED: suppressing the marker column failed with `estimated row carries no "*"
   marker in the EST column`; restoring the old `(ESTIMATED)` suffix failed with
   `no table row for model "gpt-5.3-codex-spark"`, i.e. each assertion catches
   its own defect rather than both firing on any change. Both restored, GREEN.
   Rendered result:

   ```
   | PROVIDER | MODEL               | INPUT       | ... | WRITE 1H | EST |
   | openai   | gpt-5.3-codex-spark | 1.750000000 | ... | -        | *   |

   ESTIMATED PRICES
   * gpt-5.3-codex-spark (openai): equivalent estimate based on gpt-5.3-codex. ...
   ```

   Verification (L2): `go test -mod=vendor ./internal/usage/ ./cmd/agentdeck/`
   ok, `go test -mod=vendor ./...` ok, `gofmt -l` clean, `go vet ./...` clean.
   The embedded artifacts are untouched — `model-prices.json` `d69fc03c…` and
   `price-gapfill.json` `1da8e3cf…`, both reconfirmed by SHA-256 after the
   experiment that temporarily emptied the gap-fill to build the comparison
   binary — so `check-prices-reproducible` and the cross-builds were not re-run.
   Real-binary output for `price list gpt-5.3-codex-spark`, the full 112-model
   listing, and a listing with no estimate were all rendered and inspected.
3. **gapfill-contract** — Establish the curated gap-fill as a first-class,
   separate input (root cause 3) so regeneration cannot wipe it: a small
   hand-maintained file carrying, per entry, the model, provider, prices,
   `effective_from`, and a real vendor `source_url`, merged into the generated
   catalog by task 4's generator. Keep these entries in the **`bundled`** layer,
   not `official`: `bundled` loses to a fresher `litellm` catalog on
   `catalogEffective`, so if upstream ever starts pricing Spark, upstream wins
   automatically and the curated row stops mattering — whereas an `official`
   entry would outrank upstream forever and freeze a stale price. Document the
   layer choice and the curation bar in `docs/specs/cli-design.md` (spec version
   bump + changelog row).
   **Completion note (2026-07-23):** Added `internal/usage/price-gapfill.json`
   (the curated input, embedded) and `internal/usage/price_gapfill.go`
   (`GapfillEntry`, `GapfillPending`, `ParseGapfill`, `BundledGapfill`,
   `MergeGapfill`). The file carries its own `comment` and `curation_bar` text
   so the layer choice and the bar travel with the data, not only in the spec.
   `ParseGapfill` **enforces** the curation bar rather than documenting it: a
   shipped entry requires an `https` vendor `source_url` with a host, a
   non-empty `verified_by`, a `verified_on` parseable as `YYYY-MM-DD`, an
   RFC3339 `effective_from`, an `openai`/`anthropic` provider, and at least one
   component whose value parses through the same `decimal` used for real
   prices. Duplicates are rejected, as is a model appearing in both `entries`
   and `pending`; a `pending` item must say what it is `blocked_on`.
   `MergeGapfill` overlays curated entries onto the generated models and is the
   single merge point task 4's generator calls, so a curated row survives
   regeneration by construction (root cause 3) rather than by convention. It
   copies rather than mutates its input, asserted by
   `TestGapfillEntriesMergeIntoBundledLayer`, which also pins that a curated
   entry wins over an upstream row for the same model — the only reason to
   curate one.
   Entries land in `bundled`, never `official`, per the plan's reasoning; the
   rationale is recorded in the Go doc comment, the JSON `comment` field, and
   the spec, so a future contributor cannot "promote for convenience" without
   contradicting three places.
   Spec: `docs/specs/cli-design.md` gains a `### Curated Gap-Fill` subsection
   under Price Catalog covering the merge, the layer choice and why, and the
   enforced bar including "an unconfirmed rate stays unpriced"; version bumped
   10 → 11 with a changelog row.
   Verified: `TestBundledGapfillMeetsCurationBar` (the shipped file parses and
   any entry cites a real vendor URL) and `TestGapfillEntriesMergeIntoBundledLayer`.
   **Repair (2026-07-23):** closed both Round 1 findings.
   [P1] `ParseGapfill` accepted any HTTPS host and any component name, so an
   aggregator URL or a component that prices nothing could satisfy the bar —
   and with zero shipped entries the existing loop was vacuous, so nothing
   caught it. Added `gapfillVendorHosts` (openai → `openai.com`, anthropic →
   `anthropic.com`) enforced through `vendorHostMatches`, which accepts the
   domain or a true subdomain and rejects lookalikes such as `notopenai.com`,
   and requires the host to match the entry's *own* provider. Added
   `gapfillSupportedComponents`, the exact set `priceAt` reads
   (`input`, `output`, `cached_input`, `cache_read`, `cache_write_5m`,
   `cache_write_1h`); an unsupported component is now rejected outright rather
   than counted toward "at least one price", and at least one supported
   component is required.
   [P2] `MergeGapfill` documented that it copies but `aliases := e.Aliases`
   shared the caller's backing array, so `sort.Strings` reordered the caller's
   entry in place; the old assertion only checked the generated map, so it
   stayed green. Now clones before sorting.
   Tests: `internal/usage/price_gapfill_test.go` adds a 19-case table
   (`TestParseGapfillEnforcesCurationBar`) built by mutating one field of a
   valid entry at a time — accepting the vendor domain, a vendor subdomain, and
   the anthropic equivalent, while rejecting models.dev, OpenRouter, the
   `vendor.example` placeholder the old test itself used, a provider/domain
   mismatch, a lookalike host, plain HTTP, an arbitrary component alone, an
   arbitrary component alongside a real one, and each missing verification
   field. `TestParseGapfillRejectsDuplicateAndContradictoryModels` covers
   duplicates, a model both shipped and pending, and a pending entry with no
   reason. `TestMergeGapfillDoesNotMutateItsInputs` asserts the caller's
   aliases order, the whole entry, and the generated map are unchanged.
   RED/GREEN: reverting the alias clone failed the mutation assertion with
   `reordered the caller's aliases in place`; disabling the vendor-domain and
   component checks failed exactly the 7 subtests that target them (the
   accept-cases stayed green, confirming the table discriminates rather than
   just failing). Both restored and re-verified GREEN.
   **Repair (2026-07-23, round 2):** closed the remaining partially-closed
   finding. The component allowlist was global, so an `openai` entry carrying
   only `cache_read`, or an `anthropic` entry carrying only `cached_input`,
   still passed — those names exist in `priceAt`, but in the *other* provider's
   branch, so such an entry would price no token while looking curated. Split
   `gapfillSupportedComponents` by provider, mirroring `priceAt` exactly
   (openai: `input`, `output`, `cached_input`; anthropic: `input`, `output`,
   `cache_read`, `cache_write_5m`, `cache_write_1h`), rejected any component
   not supported for the entry's own provider, and made the error name that
   provider and its real set. `sortedSupportedComponents` is provider-aware so
   the message cannot drift from the check.
   Added 6 table cases: openai-with-`cache_read` and anthropic-with-
   `cached_input` rejected both alone and mixed with valid components, plus a
   positive case per provider carrying its full supported set.
   RED: temporarily restoring the global (union) allowlist failed exactly the
   4 mismatch cases while both full-set positives stayed green, confirming the
   new cases test the provider split rather than the component names. Restored
   and re-verified GREEN.
4. **bundled-regen** — Close the cold-start gap (root cause 1). Add a
   release-time regeneration step that rebuilds `model-prices.json` from a
   pinned LiteLLM commit for both vendors, merges task 3's curated entries over
   it, and emits a content-derived version string that retires the misleading
   `2026-07-13-openai-standard-v1` name. Define in the spec who runs it, from
   which pinned source, and on what cadence — a generated, reviewed artifact,
   never a hand-edited file. Keep the existing provenance and immutability
   contract intact.
   **Completion note (2026-07-23):** Added `GenerateBundledCatalog`
   (`internal/usage/bundled_catalog_gen.go`) and the thin network shim
   `tools/genprices`, wired as `make prices-regen [LITELLM_COMMIT=<sha>]` and
   `make check-prices-reproducible`. The generator **reuses `liteLLMCatalog`**,
   the same conversion the network `price update` path uses, so the bundled and
   litellm layers cannot drift in how an upstream row becomes a price; all
   logic lives in package `usage` and is unit-tested without network, leaving
   the command as fetch-and-write only. It merges task 3's curated entries via
   `MergeGapfill` and stamps task 1's content-derived version, then re-verifies
   its own output through `VerifyBundledCatalogVersion` before returning.
   Regenerated from pinned LiteLLM commit
   `f2479cc704f6e63d5510929d30ce8e11ffe43467`: **2 models → 111 models**
   (both vendors), version `2026-07-13-openai-standard-v1` →
   `bundled-litellm-merged-g<digest>`, retiring the misleading old string that
   named only OpenAI while carrying an Anthropic model.
   Two real defects were found by tests during this task, not assumed away:
   (1) The first generator stamped `effective_from = generation time`, which
   `liteLLMCatalog` does correctly for a network fetch but is wrong for a
   bundled fallback — it left every event older than the build unpriced,
   defeating the very cold-start goal of this plan. Caught by pre-existing
   `TestSummarySessionsAndEventTimeCatalogSelection` and
   `TestOfficialOverridesRetainCatalogComponentsAndProvenance`. Fixed with the
   documented `BundledCatalogEffectiveFrom` constant, held at the date the
   bundled layer was established so regeneration is idempotent and never
   silently re-dates prices; a fresher litellm/official catalog outranks it on
   `catalogEffective` regardless, so a stable bundled date can never beat real
   upstream data. This is the same `ImportBundledCatalog`-minimum-date
   interaction task 2 flags, verified rather than assumed.
   (2) The first generator emitted a wall-clock `retrieved_at`, making the
   artifact differ on every run. Caught by `genprices -check`, which exists
   precisely to prove the committed file matches its inputs. Fixed by dropping
   wall-clock from the artifact entirely: the pinned commit SHA identifies the
   upstream snapshot exactly, so generation is now a pure function of (LiteLLM
   bytes, commit, gap-fill) and `make check-prices-reproducible` is a real
   guard that a hand-edit cannot pass. It self-discovers the pinned commit from
   the file's own `generated_from`, so it needs no separately maintained pin.
   Provenance contract kept intact: an existing test requires the bundled
   file's `sources` to be exactly one `bundled://agentdeck/model-prices.json`
   sentinel (the value `ImportBundledCatalog` records as `source_url`). The
   first generator put the LiteLLM URL there and the test caught it; upstream
   identity now lives in a separate `generated_from` field, so this artifact's
   own identity and what it was built from stay distinct.
   Spec: `### Bundled Catalog` documents the generated-artifact rule, the
   pinned source, the release-preparer ownership and release-time cadence, the
   reproducibility check, the stable effective date and why, and the
   content-derived version invariant.
   L3 verification (build/release-time artifact): `GOOS=darwin GOARCH=arm64`
   and `GOOS=darwin GOARCH=amd64` cross-builds both succeed with the larger
   embedded catalog, and `make check-arm64-size` passes at 12,179,714 bytes
   against the 26,214,400-byte gate (13.38 MiB headroom), so a 34 KB catalog is
   comfortably within budget. `make check-privacy` passes.
   **Repair (2026-07-23):** closed both Round 1 findings.
   [P1] The completion note claimed the generator was unit-tested without
   network, but no test called `GenerateBundledCatalog` — the tests inspected
   the already-generated artifact, so a regression in any of its
   transformations would have stayed green until someone ran a network
   regeneration. That claim was wrong and is now true: added
   `internal/usage/bundled_catalog_gen_test.go` with a synthetic upstream
   fixture carrying the real document's traps (both direct vendors, an
   `azure/` channel row, a cost-less `chatgpt/` row, and an openai row missing
   cached-input cost). It pins the provider filter and exact per-million
   conversions for both vendors, the stable `effective_from`, the
   `sources`-versus-`generated_from` split including the absence of any
   wall-clock field, curated merge and curated-wins-over-upstream, byte
   determinism across two generations, the content-derived version and that a
   changed curated input moves it, rejection of unpinned/short/non-hex
   commits, and rejection of malformed or direct-vendor-free upstream.
   RED/GREEN: reintroducing generation-time `effective_from` failed
   `TestGenerateBundledCatalogUsesStableEffectiveDate` with the real dates in
   the message; restored and re-verified GREEN.
   [P2] `make check-prices-reproducible` did not self-discover the recorded
   commit as claimed — it shelled out to `python3` to read
   `generated_from[0].commit_sha`, adding an undeclared runtime dependency to a
   Go release tool, and `genprices -check` without a commit resolved current
   `main`, i.e. compared the artifact against inputs it was not built from.
   Added `RecordedLiteLLMCommit` in package `usage`: check mode now reads the
   pin from the artifact's own `generated_from`, and fails loudly on missing,
   malformed, ambiguous (more than one litellm origin), or invalid-SHA
   metadata rather than falling back to a different pin. The Makefile's
   `python3` subprocess is gone; the target is now plain `genprices -check`.
   `TestRecordedLiteLLMCommit` covers the happy path on both a generated
   fixture and the shipped artifact plus all six failure modes, with no
   network. Also corrected the tool comment's `COMMIT=` to the real
   `LITELLM_COMMIT=` Make variable.
   **Repair (2026-07-23, round 2):** closed the remaining partially-closed
   finding — Round 1 had asked for a no-network test of the *actual* check
   path, and the previous repair only covered the `RecordedLiteLLMCommit`
   helper, leaving the check-mode wiring itself unprotected.
   Added a `fetcher` seam in `tools/genprices` (`func(ctx, url, headers)
   ([]byte, error)`), with `httpFetcher` wrapping the real client in `main()`
   and `run`/`latestCommit` taking it as a parameter. That is the whole
   production change; no behavior moved.
   Added `tools/genprices/main_test.go`, which drives the real
   `run(..., check=true)` against temporary artifact and gap-fill files with a
   recording fake fetcher, and asserts: the pin comes from the artifact's own
   `generated_from` when no commit is passed; the latest-main endpoint is
   **not** requested; exactly one fetch is made, for the recorded commit's
   snapshot URL; a byte-identical artifact passes; a hand-edited one fails with
   the regeneration command naming the recorded pin; and an artifact whose pin
   is missing, malformed, unpinned to a branch name, empty, or ambiguous fails
   **before any request at all** (the fake fails the test on any call). Two
   counterpart tests keep the check-mode behavior honest rather than
   tautological: an explicit commit is fetched and still skips latest-main
   resolution, and write mode *does* resolve latest main and records the
   resolved pin — so check mode's difference is deliberate, not a broken
   resolver. A final case proves a gap-fill failing the curation bar is
   rejected before the network is touched.
   RED: disabling check mode's pin-discovery branch (so it fell back to
   latest-main) failed the pin test, the mismatch test, and all five
   before-any-network cases. Restored and re-verified GREEN.
   The embedded catalog is byte-identical after this repair
   (`c4231e75…01876dc24`, `make check-prices-reproducible` still passes), so
   the generator's output contract is unchanged; cross-build, size, and
   privacy checks were re-run anyway because `internal/usage` is compiled into
   the binary — all pass, arm64 at 12,179,730 bytes.
5. **coverage-test** — Add a test asserting that a fresh install with **no
   network** prices a representative set of currently-real models (the fifteen
   from the baseline are the natural fixture), so the catalog cannot silently
   decay back toward a stub. Re-measure the cold-start percentage from the
   baseline table and record the new figure in this document.
   **Completion note (2026-07-23):** Added
   `internal/usage/bundled_coverage_test.go`.
   `TestBundledCatalogPricesRealModelsWithoutNetwork` opens a fresh store,
   calls **only** `ImportBundledCatalog` (no `UpdateLiteLLM`, no
   `ImportOfficialOverrides`, no network), inserts one event per model in the
   `coldStartModels` fixture, and asserts each model's expected priced state.
   The fixture carries the baseline's real models plus a `wantPriced` flag and
   a `why` string, so the two deliberate exclusions are asserted rather than
   merely absent: `gpt-5.3-codex-spark` and `codex-auto-review` must stay
   unpriced, and the test fails with an explanatory message if either starts
   pricing without the fixture and this plan being updated together. Each
   event only carries components its client actually reports, so an unpriced
   *component* cannot masquerade as an unpriced *model*.
   It also covers the two dotted spellings in real use, `claude-haiku-4.5` and
   `claude-opus-4.8`, which have no catalog rows of their own and must price
   through the existing `claude-` punctuation rule — verified, not assumed.
   `TestBundledCatalogIsNotAStub` guards the other direction: at least 50
   models, both vendors present, and the generated version prefix intact.
   **Re-measured cold start — controlled A/B (2026-07-23).** The first pass
   compared the new catalog's observation against the `## Measured Baseline`
   7.5% figure, which was captured earlier against a different database state;
   review reopened it because the input data had changed too, so the delta was
   not attributable to the catalog. Replaced with a paired measurement in which
   **only the embedded catalog differs**.

   Method: one frozen snapshot of the real state database, SHA-256
   `2434b8cad72915de1415f2f0ea0bc585f1499f2b164321058e9b011486a66467`,
   reverified byte-identical before each side's run and unchanged afterward.
   The real `~/.agentdeck/agentdeck.sqlite3` was never opened — only copied
   from — and no real session source was scanned (`--no-scan` throughout).
   Each side: a fresh copy of that snapshot, `DELETE FROM model_prices; DELETE
   FROM price_catalogs;` to strip every layer, then `ImportBundledCatalog` from
   a binary embedding that side's catalog, then
   `env NO_COLOR=1 <shared-binary> --state-dir <side> --format json usage stats
   --period all --no-scan`. Stats runs from **one shared binary** for both
   sides (with `--no-scan` it imports nothing, so it is catalog-neutral), and
   the importers differ *only* in the embedded `model-prices.json`.

   | Side | Catalog | SHA-256 of catalog | Models |
   | --- | --- | --- | --- |
   | old | `2026-07-13-openai-standard-v1` (recovered via `git show HEAD:internal/usage/model-prices.json`) | `4bb69e84…c741b93fe` | 2 |
   | new | `bundled-litellm-merged-gc972a7bce902` | `c4231e75…01876dc24` | 111 |

   Both sides measured the **same 5,259,503,075 tokens**, which is the check
   that this is a controlled comparison rather than two observations:

   | | old catalog | new catalog |
   | --- | ---: | ---: |
   | Fully priced (all components) | 388,030,488 — **7.4%** | 4,922,490,150 — **93.6%** |
   | Model matched by the catalog | 388,030,488 — **7.4%** | 5,097,088,027 — **96.9%** |

   On this one fixture, replacing the catalog takes cold-start coverage from
   **7.4% to 93.6%** of tokens fully priced. That 7.4% is this snapshot's
   like-for-like figure; the 7.5% in `## Measured Baseline` was measured
   against an earlier database state and is **not** directly comparable — the
   two are close only by coincidence of which models grew.
   The 3.1% still carrying `unknown_model` on the new side is exactly the two
   models this plan leaves unpriced on purpose: `codex-auto-review` (out of
   scope) and `gpt-5.3-codex-spark` (task 2's unconfirmed rate). The gap
   between 96.9% matched and 93.6% fully priced is entirely `claude-haiku-4.5`
   and `claude-opus-4.8` reporting `missing_components:
   [cache_creation_tokens]` — their models *are* matched and priced by the
   bundled catalog, and that component gap is the separate token-classification
   concern this plan lists as out of scope. So no part of the residual is a
   catalog-coverage failure.

   **Re-measured after task 2's Spark estimate (2026-07-23), same input.**
   This run **did not touch the real `~/.agentdeck/agentdeck.sqlite3` at all**;
   it reused the already-frozen snapshot from the A/B above, SHA-256
   `2434b8cad72915de1415f2f0ea0bc585f1499f2b164321058e9b011486a66467`,
   reverified byte-identical before and after, with `--no-scan` throughout.
   Only the catalog differs between sides: each side strips every layer from a
   fresh copy of that snapshot and imports one catalog, then one shared binary
   renders `usage stats --period all --no-scan --format json`.

   **Method difference from the A/B above, stated plainly:** that run built one
   binary per side and used the product's own `ImportBundledCatalog`. This run
   used a script that reproduces the same import (bundled kind, the
   `bundled://agentdeck/model-prices.json` sentinel, the file's SHA-256, the
   catalog date rule, and each model's own `effective_from`) and one shared
   stats binary. The equivalence evidence is that the no-estimate side is not a
   re-derived approximation: regenerating with an empty `entries` array
   reproduces catalog `c4231e75…01876dc24` **byte for byte** — the same
   artifact the A/B measured — and the script-imported side returns that
   artifact's recorded figures exactly (4,922,490,150 / 5,097,088,027). A
   faithless importer would not land on both numbers. Anyone re-running this
   should still prefer per-side binaries; the script is a convenience, not the
   contract.

   | Side | Catalog | Models | Fully priced | Model matched |
   | --- | --- | ---: | ---: | ---: |
   | without the estimate | `…-gc972a7bce902` (`c4231e75…`) | 111 | 4,922,490,150 — **93.6%** | 5,097,088,027 — **96.9%** |
   | with the estimate | `…-g32b44e2b8128` (`d69fc03c…`) | 112 | 5,000,000,097 — **95.1%** | 5,174,597,974 — **98.4%** |

   These figures are bound to the **post-repair** catalog-date rule: after the
   review fix pinned the bundled catalog's own effective date to the stable
   constant, both sides were re-imported with that rule (catalog effective
   `2026-07-13` on both) and re-measured, and every number below came back
   identical. That is expected — each side holds exactly one catalog, so the
   catalog date is not a precedence key there, and events predating it still
   price through the current-price fallback — but it was re-run rather than
   argued.

   Both sides measured the same 5,259,503,075 tokens. The delta is exactly
   `gpt-5.3-codex-spark`'s 77,509,947 tokens moving out of `unknown_model`;
   `codex-auto-review` (84,905,101 tokens) stays UNPRICED, and
   `claude-haiku-4.5` / `claude-opus-4.8` still report only
   `missing_components: [cache_creation_tokens]`, the out-of-scope
   token-classification gap. Spark's token count differs from the 69,406,886 in
   `## Measured Baseline` because that figure came from an earlier database
   state; within this snapshot both sides see the same 77,509,947.

## Out of Scope

- **`codex-auto-review` (84.9 M tokens, unpriced).** It is absent from every
  pricing source checked and is most likely a Codex-internal pseudo-model rather
  than a billable one. It needs classification — suppress it, or attribute it to
  its real underlying model — not a price. That is a usage-attribution question;
  open a separate plan if it matters.
- **The dotted-name `cache_creation_tokens` gap.** `claude-haiku-4.5` and
  `claude-opus-4.8` report `missing_components: [cache_creation_tokens]`. Their
  models *are* matched and priced; this is a token-classification issue in
  event parsing, not a catalog-coverage one. Separate concern, separate plan.

## Follow-Up — 2026-07-23: Spark Equivalent Estimate

The original task treated every model without a published vendor rate the same:
leave it unpriced. That is too broad. `gpt-5.3-codex-spark` is not absent or
unreleased; it is a real Pro subscription-only successor that replaces
`gpt-5.3-codex`, but OpenAI does not expose it through the API or publish a
separate token rate. The product decision is therefore to ship an explicitly
marked **equivalent cost estimate**, while continuing to leave absent,
unreleased, or unidentified models unpriced.

Task 2 is reopened for development with this accepted contract:

- Move `gpt-5.3-codex-spark` from `pending` to a curated entry priced at
  `$1.75` input, `$14.00` output, and `$0.175` cached input per one million
  tokens.
- Mark it `equivalent_estimate`, name `gpt-5.3-codex` as `basis_model`, and
  state that the value estimates equivalent token cost rather than the user's
  actual Pro subscription invoice.
- Extend the gap-fill validation contract without weakening normal
  `vendor_rate` entries. An estimate is allowed only for a real released
  subscription-only model, with a named vendor-priced basis model and a
  non-empty explanation.
- Do not add a database migration. `PriceList` derives the estimate marker from
  the compiled gap-fill and the winning component provenance.
- Keep `usage stats`, summaries, sessions, and their JSON output unchanged.
  Only `price list` text and JSON expose `ESTIMATED`, the basis model, and the
  note.
- Show the marker while any effective component still comes from the estimated
  bundled entry. A fresher upstream or official catalog that supplies all
  components removes the marker automatically.
- Update the cold-start fixture and measurement: Spark becomes intentionally
  priced, while `codex-auto-review` remains an explicit unpriced pseudo-model.
- Add regression coverage for schema validation, bundled generation, fresh
  install pricing, text/JSON price-list disclosure, unchanged stats output, and
  upstream replacement of the estimate.

Delivered 2026-07-23 — see task 2's completion note for what was built, the
re-measured cold-start figures under `## Tasks` item 5, and the spec's version
12 changelog row. This section supersedes the "unconfirmed, therefore unpriced"
guidance in `## Price Confidence`, which remains as the record of why the
figure could not be shipped as a vendor rate.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | version-guard | ✅ | ✅ |
| 2 | spark-gapfill | ✅ | ✅ |
| 3 | gapfill-contract | ✅ | ✅ |
| 4 | bundled-regen | ✅ | ✅ |
| 5 | coverage-test | ✅ | ✅ |

Task 2 is no longer blocked on a nonexistent Spark API rate card. The accepted
follow-up above authorizes an explicitly disclosed equivalent estimate based on
the predecessor model; it does not reclassify that estimate as an official
Spark API price.

Done: **5/5 reviewed.** Task 2 shipped under the approved equivalent-estimate
contract and passed review on 2026-07-23 (round 6 in
`docs/reviews/price-coverage/spark-gapfill.md`), which verified the admission
bar against the real generator, upgrade precedence across three coexisting
bundled catalogs, disclosure scope across eleven commands in both formats, and
that the `usage.stats` / `usage.summary` contracts are byte-identical to `HEAD`.
That round was performed in the same session as the implementation, at the
user's direction; the caveat is recorded with the round. The
implementer ticks **Dev** once a task is built and its targeted
verification passes; an independent reviewer ticks **Review** once findings are
closed, recording the round in `docs/reviews/price-coverage/`. A task is done
only when Review is ticked.

Sequencing: task 1 gates tasks 2 and 4. Task 3 defines the input format that
task 4's generator consumes, so it lands before task 4. Task 2 may ship ahead of
tasks 3 and 4 as the direct fix for the reported model, provided task 1 is in.

## Invariants

- **A bundled content change always carries a new catalog version string.**
  `INSERT OR IGNORE` is keyed on the version, so reusing a string silently keeps
  stale prices and a stale `content_sha256` on every existing install. This is
  what task 1 enforces; do not relax it in later tasks.
- **Curated gap-fill entries stay in the `bundled` layer.** Never promote them
  to `official` for convenience — that would permanently outrank a future
  upstream correction. See task 3.
- **Regeneration never silently invents a price.** A normal curated entry
  carries a confirmed vendor rate. A released subscription-only model may use
  an explicitly marked equivalent estimate only when it names the
  vendor-priced predecessor or basis model and explains that the result is not
  an actual subscription invoice. Absent, unreleased, or unidentified models
  stay unpriced.
- **The importer's `openai`/`anthropic` provider filter stays as it is.** The
  `chatgpt/` namespace carries no prices; admitting it buys nothing.

## Starting a task

Turn any row of the Status matrix into a scoped development instruction through
its anchor — no fresh prompt needs to be written by hand:

> **进入开发:price 覆盖率 / `<task-anchor>`**
> 阅读 `AGENTS.md`、本 plan `## Tasks` 中 `<task-anchor>` 一条及它命名的文件、本
> plan 的 `## Invariants` 与 `## Required Verification`,以及 `docs/README.md` 的
> 验证路由。只在该 task 的范围内实现并自测。完成后在 `## Status` 勾上该行的
> `Dev`,把命令与结果记进该 task 的完成注记;评审留痕到
> `docs/reviews/price-coverage/<task-anchor>.md`。

Example — `spark-gapfill`: 阅读 `internal/usage/model-prices.json`、
`internal/usage/usage.go` 的 `ImportBundledCatalog`/`importCatalog`/`priceAt`,
以及本 plan 的 `## Price Confidence` 一节;先完成费率人工核对再落数据,并为
bundled catalog 生效时间下移加一条测试。

## Required Verification

L2 for tasks 1-3 and 5: the bundled catalog is a persisted-data and provenance
contract, so run targeted `go test ./internal/usage` plus
`go test -mod=vendor ./...` once after the final edit.

L3 for task 4: it adds a build/release-time generation step, so add the relevant
cross-build check for the embedded artifact on top of L2. `release-verify` (L4)
is not required unless the release artifact itself is being validated.

No credential, concurrency, or migration-execution surface is touched by any
task. Commands are listed in `AGENTS.md` under "Testing and Verification".
