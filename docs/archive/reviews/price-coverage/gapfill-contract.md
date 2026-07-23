---
status: historical
plan: price-coverage
task: gapfill-contract
retired: 2026-07-23
---

# Review log — price-coverage / gapfill-contract

## Round 1 — 2026-07-23

- Reviewed state: same price-coverage worktree identified by SHA-256
  `f1061aabdd19cbf7baaddc384d9ab38d2e76ffb715ec8995741e34e6fcaa91c9`.
- Reviewer: Codex
- Scope: enforced curation bar, vendor provenance, valid price components,
  pending-state integrity, bundled-layer merge precedence, regeneration
  survival, input immutability, and test strength.
- Findings:
  - [P1] `ParseGapfill` does not enforce the plan's “real vendor rate-card”
    provenance invariant. It accepts any HTTPS URL with any host, regardless
    of the entry provider; the only constructed-entry test even uses
    `https://vendor.example/rates`. It also accepts any component name as long
    as its value parses as a decimal, so an entry containing only an ignored
    key such as `not_a_price` can satisfy the claimed “at least one price” bar
    while pricing no token component. The shipping file currently has zero
    entries, making `TestBundledGapfillMeetsCurationBar`'s entry loop vacuous.
    Enforce provider-matching vendor hosts and the supported component set,
    require at least one supported component, and add table-driven valid and
    invalid entry tests so the first promoted pending model cannot bypass the
    bar.
  - [P2] `MergeGapfill` is documented as copying rather than mutating its
    inputs, but `aliases := e.Aliases; sort.Strings(aliases)` sorts the caller's
    backing slice in place. The existing non-mutation assertion checks only
    the generated models map, not the `Gapfill` argument, so it remains green.
    Clone aliases before sorting and assert the original gap-fill entry and
    aliases are byte-for-byte/order unchanged after the merge.
- Evidence: read-only inspection of `price_gapfill.go`, the embedded JSON,
  gap-fill tests, generator merge call, spec contract, and completion note.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: repaired price-coverage worktree identified by SHA-256
  `1f9dc78452627729ef7e5d87ed789514468f3a04a494ad0f29c3bec9884982ed`.
- Reviewer: Codex
- Scope: closure of both Round 1 findings, strength of the new curation table
  and mutation RED, and newly introduced or remaining curation gaps.
- Findings:
  - [closed] Vendor provenance now requires the provider's exact vendor domain
    or a true subdomain, rejecting aggregators, placeholders, lookalikes,
    provider/domain mismatch, and non-HTTPS URLs. The new table exercises real
    non-empty entries instead of relying on the shipping file's empty array.
  - [partially closed] Arbitrary component names are now rejected, but the
    allowlist is global rather than provider-specific. An `openai` entry
    containing only `cache_read`, or an `anthropic` entry containing only
    `cached_input`, still passes `ParseGapfill`; those names exist somewhere in
    `priceAt`, but not in the branch used for that provider/client, so the
    accepted entry can still price no token component. Split the allowed set by
    provider (OpenAI: `input`, `output`, `cached_input`; Anthropic: `input`,
    `output`, `cache_read`, `cache_write_5m`, `cache_write_1h`), reject a
    component that is unsupported for that entry's provider, and add both
    mismatch cases to the table with a RED check.
  - [closed] `MergeGapfill` clones aliases before sorting. The new regression
    test checks the original aliases order, whole entry, and generated map,
    and the recorded mutation RED targets the original aliasing defect.
- Evidence: read-only inspection of the repaired parser, 19-case table,
  non-mutation test, merge path, and RED/GREEN record. `git diff --check` was
  clean before this review update.
- Verdict: REOPEN

## Round 3 — 2026-07-23

- Reviewed state: explicit-file SHA-256
  `7d029feed3ac663eccaeb138bbe65afa4d42d4e98b898adc1822cc9c81e30ec9`.
- Reviewer: Codex
- Scope: closure of the remaining provider/component mismatch finding, test
  discrimination, and newly introduced curation gaps.
- Findings:
  - [closed] `gapfillSupportedComponents` is now keyed by entry provider.
    OpenAI accepts only `input`, `output`, and `cached_input`; Anthropic accepts
    only `input`, `output`, `cache_read`, `cache_write_5m`, and
    `cache_write_1h`. `ParseGapfill` selects that provider-specific set before
    validating every component, so a name supported only by the other
    provider is rejected even when mixed with otherwise valid components.
  - [closed] The table covers both pure and mixed mismatches in both
    directions, plus each provider's complete legal set. The recorded mutation
    RED failed exactly the four mismatch cases while both positive cases stayed
    green, so the assertions distinguish the provider split from a global
    component-name check.
  - No new blocking or medium-severity finding was found in the repaired
    parser or its regression tests.
- Evidence: read-only inspection of `price_gapfill.go` and
  `price_gapfill_test.go`, the recorded RED/GREEN evidence, explicit-file
  content identity, and a clean `rtk git diff --check`. Product tests were not
  mechanically rerun because the repair evidence is recorded for this content
  and the re-review claim is limited to closure of the finding.
- Verdict: PASS
