---
status: historical
plan: price-coverage
task: spark-gapfill
retired: 2026-07-23
---

# Review log — price-coverage / spark-gapfill

## Round 1 — 2026-07-23

- Reviewed state: same price-coverage worktree identified by SHA-256
  `f1061aabdd19cbf7baaddc384d9ab38d2e76ffb715ec8995741e34e6fcaa91c9`.
- Reviewer: Codex
- Scope: vendor-rate confirmation gate, absence of an invented Spark price,
  pending-state durability, and the documented handoff for a human verifier.
- Findings:
  - [blocked as designed] No human has confirmed the reported
    `gpt-5.3-codex-spark` rate against OpenAI's published rate card. Automated
    access did not produce machine-checkable vendor evidence, so there is no
    `source_url`, `verified_by`, `verified_on`, or confirmed
    `effective_from` to review.
  - [closed safety condition] Spark is absent from the generated bundled
    catalog and remains unpriced in the cold-start test. It is explicitly
    tracked in `price-gapfill.json` under `pending`, with the reported figures,
    evidence weakness, blocker, and exact promotion steps. The implementation
    therefore obeys “do not ship the figure if it cannot be confirmed.”
- Evidence: read-only inspection of the pending entry, generated catalog
  metadata, cold-start fixture, plan evidence, and spec invariant. No vendor
  page was accessed in this review and no human verification is claimed.
- Verdict: BLOCKED

## Round 2 — 2026-07-23

- Reviewed state: repaired price-coverage worktree identified by SHA-256
  `1f9dc78452627729ef7e5d87ed789514468f3a04a494ad0f29c3bec9884982ed`.
- Findings:
  - [still blocked as designed] No human vendor-rate verification has been
    added, so there is still no reviewable `source_url`, verifier identity,
    verification date, or confirmed effective date.
  - Spark remains absent from the generated catalog, present under `pending`,
    and deliberately unpriced. No silent guess was introduced by the repairs.
- Verdict: BLOCKED

## Round 3 — 2026-07-23

- Reviewed state: explicit-file SHA-256
  `7d029feed3ac663eccaeb138bbe65afa4d42d4e98b898adc1822cc9c81e30ec9`.
- Findings:
  - [still blocked as designed] No human verification against an OpenAI vendor
    rate card has been recorded. The required `source_url`, `verified_by`,
    `verified_on`, and confirmed `effective_from` therefore remain unavailable.
  - Spark remains pending and deliberately unpriced; this review does not
    promote the reported figures or claim that they are official prices.
- Evidence: current plan, pending-state contract, and the unchanged human gate;
  no vendor-rate verification was performed or inferred.
- Verdict: BLOCKED

## Post-Round 3 product decision — 2026-07-23

- The BLOCKED verdict above accurately reflects the old contract but no longer
  defines the next implementation. The product owner clarified that Spark is a
  real released Pro subscription-only successor to `gpt-5.3-codex`, not an
  absent or unpublished model, and approved an explicitly disclosed equivalent
  estimate.
- Accepted estimate: `$1.75` input, `$14.00` output, and `$0.175` cached input
  per one million tokens, with `gpt-5.3-codex` named as the basis model.
- Required disclosure: no official Spark API rate exists; this is an equivalent
  token-cost estimate and not the user's actual subscription invoice. The
  disclosure belongs only to `price list` text/JSON, not usage stats output.
- Status: REOPEN FOR DEVELOPMENT. A new review round is required after the
  estimate metadata, generated catalog, output disclosure, replacement
  behavior, and regression tests are implemented.

## Round 4 — 2026-07-23 (re-review of the equivalent-estimate implementation)

- Reviewed state: uncommitted worktree carrying the equivalent-estimate
  implementation plus its round-1 repairs. Embedded artifacts pinned by hash:
  `internal/usage/model-prices.json` `d69fc03c…f31d70ee`,
  `internal/usage/price-gapfill.json` `1da8e3cf…4e2f0aa6`.
- **Independence limitation, stated up front:** this round was performed in the
  same session as the implementation and the repairs, not by a separate
  reviewer. It is therefore adequate as verification that the round-1 findings
  are closed, but it is *not* the independent review the plan's `Review` column
  requires. That cell stays unticked.
- Round-1 findings, re-verified one by one:
  - [closed] **P1 catalog date coupled to curated data.** `ImportBundledCatalog`
    now takes `BundledCatalogEffectiveFrom`, capped by the clock; models keep
    their own dates. RED: reverting it fails
    `TestNewerBundledCatalogOutranksAPreviouslyInstalledOne` with the retired
    `2026-07-13-openai-standard-v1` still serving `gpt-5.4` at 999, and fails
    `TestBundledCatalogDateIsIndependentOfItsCuratedModelDates` on the date.
  - [closed] **P1 upgrade regression test** exists and is meaningful: it
    installs a previous release's catalog with a conflicting price *before*
    importing the compiled one and asserts every component's winning
    `catalog_version`.
  - [closed] **P1 affected assertions.** The renamed date test asserts both the
    stable catalog date and that the curated model row keeps 2026-02-12;
    `TestOfficialOverridesRetainCatalogComponentsAndProvenance` is back to its
    original `history[0].SourceKind == "official"`, which the fix restores.
  - [closed] **P2 estimate attribution.** Matching moved to the compiled
    catalog's own `catalog_version`. RED: relaxing it back to the layer check
    fails `TestAnotherBundledCatalogsPriceIsNotMarkedAsThisEstimate`, which
    reports `bundled-other-release` being disclosed as this binary's estimate.
  - [closed] **P2 unreachable alias index** removed, with the reason recorded.
  - [closed] **P2 measurement method description** now states that this round
    never touched the real database and that the importer is a script
    reproducing `ImportBundledCatalog`, with byte-identical catalog
    reproduction plus exact figure reproduction as the equivalence evidence.
  - [closed] **P3** wrap-width constant, `MergeGapfill` verbatim-fallback
    rationale, case-sensitivity note on the `ESTIMATED` assertion,
    loud-failure rationale on `markEstimatedPrices`, and the spec rule that an
    estimate's `verified_by` may name a project role.
  - [closed] **Spec sync.** cli-design v13 (frontmatter, changelog row) and the
    `docs/README.md` Documents table agree.
- Independent checks run this round, beyond re-running the suite:
  - The one behavior that changed direction between rounds was checked
    empirically rather than argued: a `gpt-5.3-codex-spark` event dated
    2026-03-01 — before the catalog's own 2026-07-13 date, after the model's
    2026-02-12 date — still prices, at `15.750000000` for 1M input + 1M output,
    through the current-price fallback. The repair did not strand pre-catalog
    Spark usage.
  - Coverage figures were re-imported and re-measured under the repaired
    catalog-date rule and came back identical (93.6% → 95.1% fully priced).
- New finding this round:
  - [medium, open] The task-2 completion note still describes the **pre-repair**
    behavior as current: it states the catalog's effective date "moves to
    2026-02-12", credits a test name that no longer exists
    (`TestBundledCatalogEffectiveDateFollowsItsEarliestModel`), and reports a
    `price history` ordering change whose assertion was reverted. The repair
    note two paragraphs later contradicts it. A reader who stops at the
    completion note gets a wrong description of shipped behavior.
- Verdict: **round-1 findings all closed; one new documentation-consistency
  finding open.** No code change is required. An independent review is still
  outstanding.

## Round 5 — 2026-07-23 (re-review of the output and documentation repairs)

- Reviewed state: uncommitted worktree. Embedded artifacts unchanged from
  round 4 and reconfirmed after this round's experiments:
  `model-prices.json` `d69fc03c…`, `price-gapfill.json` `1da8e3cf…`.
- Same-session self-review, as in round 4. Still not the independent review the
  plan's `Review` column requires; that cell stays unticked.
- Findings from the read-only review, re-verified:
  - [closed, documented not changed] **Disclosure travels with the binary, not
    the database.** Recorded in `### Curated Gap-Fill` (spec v14, changelog row,
    `docs/README.md` synced) and as a residual risk in the plan with the
    demonstration attached. Confirmed no migration was introduced:
    `internal/store/migrations.go` is untouched.
  - [closed] **MODEL cell no longer decorated.** The marker moved to an `EST`
    column carrying `*`. Verified by rendering: the estimated row's MODEL cell
    is exactly `gpt-5.3-codex-spark`; non-estimated rows in the same listing
    carry a blank `EST` cell and are otherwise unchanged; an anthropic-only
    listing (no estimate) grows no `EST` column at all; `--verbose` provenance
    is untouched. RED discriminates: suppressing the column fails only the
    marker assertion, restoring the old suffix fails only the cell-identity
    assertion.
  - [closed] **Long-token constraint enforced on the data.** RED-verified this
    round by appending a 118-character URL to the shipped note:
    `TestEstimatePricingNotesFitTheDisclosureWidth` fails naming the token, its
    length, and the width it would overflow. The note was restored byte-for-byte
    (`1da8e3cf…`) and the test returns GREEN.
- Observations, not findings, recorded so a later reader does not rediscover
  them as defects:
  - The width test measures `len(token)` in bytes while `statsWrap` measures
    display width via `runewidth`. Identical for the ASCII notes the project
    defaults to, and conservative (stricter) for wider runes, so it cannot pass
    a note that would actually overflow.
  - That test re-creates the renderer's format string rather than calling it.
    If the rendered prefix ever changes shape, the test checks the old one. The
    tokens that matter are the note's own, so the exposure is small.
- Verification reused/run at this content state: `gofmt -l` clean, `go vet
  ./...` clean, `go test -mod=vendor ./...` ok, targeted
  `./internal/usage/ ./cmd/agentdeck/` re-run with `-count=1` after restoring
  the experiment, `git diff --check` clean. Embedded artifacts unchanged, so
  `check-prices-reproducible` and cross-builds were correctly not re-run.
- Verdict: **PASS.** All findings closed, no new defects, no regressions. The
  task is complete pending one independent review.

## Round 6 — 2026-07-23 (scoped review: admission bar, upgrade precedence, disclosure scope, stats contract)

- Reviewed state: uncommitted worktree; embedded artifacts `model-prices.json`
  `d69fc03c…` and `price-gapfill.json` `1da8e3cf…`, both reconfirmed unchanged
  after this round's probes. Full suite green at this exact state.
- **Independence:** performed in the same session as the implementation, at the
  user's explicit direction after the limitation was raised. Recorded here so
  the `Review` tick is read with that context.
- Method: live probes against real binaries rather than re-reading the diff.
- **Admission bar — does it actually stop an invented price?** Yes, on both
  paths. Editing the curated entry to claim `input = 9.99` while its basis model
  is priced at 1.750000000: the real generator (`tools/genprices`) exits 1 with
  `an equivalent estimate must carry the basis model's vendor rate, so
  re-confirm the estimate rather than letting it drift`, and
  `TestBundledCatalogEstimateBorrowsItsBasisModelRate` fails independently for a
  hand-edited gap-fill that was never regenerated. The artifact was not written
  (hash unchanged), so a failed regeneration cannot leave a half-updated
  catalog.
- **Upgrade precedence across three releases.** Seeded a database with the
  v0.1.0 stub (`gpt-5.4` at 111) and the 111-model interim catalog (at 222),
  both imported earlier, then let the current binary import its own. All three
  bundled catalogs coexist with equal `effective_from`; the current one wins
  every component (2.500000000 / 0.250000000 / 15.000000000, provenance
  `bundled-litellm-merged-g32b44e2b8128`). The tie-break is `imported_at`, so
  the newest import wins — a clock rollback or a restored backup could invert
  it, which is a pre-existing property of the layer ordering, not of this task.
- **Disclosure scope.** Swept 11 commands in JSON and 4 in text. Only
  `price.list` carries `price_kind` / `basis_model` / `pricing_note` /
  `equivalent_estimate`; `usage stats`, `usage summary`, `price status`,
  `price history`, `session list`, `doctor`, `extension list`, `provider list`,
  and `backup list` are clean in both formats.
  *Evidence-quality note:* the first sweep reported everything "clean" including
  `price list`. That was a harness artifact — the shell is zsh, which does not
  word-split unquoted expansions, so every multi-word command ran as one bogus
  argument and produced no output. Caught by the positive control (`price list`
  must not be clean) and redone with `eval`. A sweep without a positive control
  would have produced a confidently wrong pass.
- **usage stats contract.** Compared the committed golden against the current
  one structurally: exactly one command's contract changed, `price.list`;
  `usage.stats` and `usage.summary` are byte-identical to `HEAD`.
- **No migration** was introduced: `internal/store/migrations.go` untouched.
- Observations, not findings:
  - The `equivalent_estimate` path is now *more* mechanically constrained than
    the `vendor_rate` path, which still rests on human attestation with no
    machine check of the number. That asymmetry is defensible — an estimate has
    no vendor attestation to lean on — and the vendor_rate bar is task 3's
    reviewed contract, not this task's.
  - The `EST` column header is terse; the note block immediately below defines
    it, and the column only appears when something is estimated.
- Verification bound to this state: `gofmt -l` clean, `go vet ./...` clean,
  `go test -mod=vendor -count=1 ./...` all 15 packages ok.
- Verdict: **PASS — no blocking or medium findings.** Task 2's `Review` is
  ticked on this basis, with the independence caveat above.
