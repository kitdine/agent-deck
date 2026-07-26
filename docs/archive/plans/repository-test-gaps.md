---
status: historical
retired: 2026-07-26
created: 2026-07-23
---

# Repository test-gap closure

## Goal

Close the meaningful behavioral test gaps originally identified across every
first-party Go package at baseline
`94437ab70273d90ff01dd19e9f64a9b358e2c709`, now resumed from the
production-fixed baseline
`4f614d34d09260a52df6bd333f6dad26134e96ac`, without changing production code
or turning the work into a line-coverage target.

This historical plan preserves the complete audit and delivery record. Task
implementation and intermediate review history remain on
`audit/repository-test-gaps-new-baseline-20260724-r1`, whose frozen reviewed
head is `2571307d8410c2b4874bc1f8fb53fef91707c129`. The old audit branch,
old-baseline staged candidates, and failed first delivery remain immutable
evidence. Only independently reviewed new-baseline final test states were
projected to the replacement delivery branch; the audit branch was never
merged into `main`.

## Baseline and authorization

- Target: `main`
- Baseline commit: `94437ab70273d90ff01dd19e9f64a9b358e2c709`
- Baseline tree: `c47a26e665bd620073a3e58f306d27298343115c`
- Broad baseline: `go test -mod=vendor -count=1 ./...` — PASS
- Atomic coverage baseline: 80.5%
- Coverage profile SHA-256:
  `b63da1d23589ad5c492684f444c2ae841c40ac1714878d519fbbc153bed76922`
- Authorized history mode: `final-state`
- Authorized partial delivery: yes
- Authorized target fast-forward: only when the delivery-state resolver emits
  `fast-forward-target`
- Authorized plan retirement: only after the complete retirement gate passes
- Not authorized: production changes, push, tag, release, force operations, or
  unrelated changes

The approved authorization identity is:

| Evidence | SHA-256 |
| --- | --- |
| Runtime capabilities | `6ef645716270f1d160344718fb127d44d8ec5f0562aafdc1b66b5d4c6b4601ed` |
| Authority inventory | `9d12a81c833f2d8c7e5c7a35059f8d35f0228e00290df7742dd051b9bbd9816c` |
| Orchestrator package | `1884c850d324ad8d33b116cc7981e14ff3fb15e89fed1bcd8e5320537846a20a` |
| Subagent config | `ed2c3a677556c376802597935fbed39bccdb02ee70a53cbe224bc7a607a1ac9c` |
| Resolved subagents | `1e87f27349ee9d1b4721295bdf9b0433a5fccdd1d8dc950217dce9d4ce770921` |
| Capability config | `b7de5d9e7ae9fda6cca8202ac4221ab9559bf5c5048bcd5bff15d7650702acd0` |
| Capability resolution | `de7900dc120e264901abfec2b115d41ece9a482369b74c828fd22fcb60d84744` |
| Complete resolved config | `e7fdf0230fb8e2bc8aee049bf60328c704347c3ed1c5ad3a6cc22a1663a679f3` |

The same-baseline resume on 2026-07-23 re-probed the same three selected
model/effort pairs. Package, authority, role, and capability hashes remained
unchanged. The refreshed runtime evidence hash is
`efd551e92dbb960fc04c5c1faa5629be87665bc1d458fadcd7da87601a8f2cde`;
the refreshed complete resolved hash is
`5e8ea8c054d82226c5d3c53d9b099ff667a66ca4b7c923bfa229b99f70399a50`.

### New-baseline resume — 2026-07-24

Authorization package
`bbf49cd178e1223c0b10ee59ea60f13f3c2e80818d63aa2b2f4a666b861e0710`
was explicitly approved. Its resolved configuration SHA-256 is
`0fc1e5b645e99297b8c8de8582be4ee9f28c4e6cde2821ed40c035739d80b432`.
Replacement authorization package
`9f8b6bcffb1ced906dda50bcf730ecea151dbcb91615b36ac6d2ebee4208fb4f`
was also explicitly approved and binds the r1 replacement audit mapping; the
original package remains the base authorization.
Ledger-count delta authorization package
`392fe2749a0db9c1ada5b945f06a1243a84d64878ed086601576447cc4123a19`
was explicitly approved. It corrects the deterministic module ledger from 15
to 16 while preserving the exact 15 tasks and every existing ownership, path,
commit, and delivery boundary.
Genprices delivery-review repair composite package
`d4860a69db52f3b081b6e44cab7899b50446addd172e86c1cc907dee87b39292`
was explicitly approved. It retains the failed delivery evidence, binds the
reviewed check-mode artifact-preservation repair, and permits replacement
delivery only after repaired Aggregate Review Round 3 passes and its audit
state is recorded.
The resumed workflow retained the original ledger and task ownership. All four
affected ledger entries received fresh focused/package evidence and independent
PASS review; Audit Aggregate Review Round 3 passed. Four fresh replacement
delivery commits then passed independent review and complete delivery
verification passed. The archive, resolver, retirement, and target gates remain
governed by the approved composite authorization and may proceed only after the
pending reviews and resolver actions.

```text
original_baseline: 94437ab70273d90ff01dd19e9f64a9b358e2c709
production_fix_commit: 4f614d34d09260a52df6bd333f6dad26134e96ac
production_fix_delivery_evidence: signed linear local-main fast-forward; no push
old_audit_integration_head: f9cb5ca35d527c80b484e835f9bec185b11b9bf8
old_reviewed_audit_integration_head: 5b68942b664cf538a52daf153e0b0a466ad473a1
resumed_baseline: 4f614d34d09260a52df6bd333f6dad26134e96ac
resumed_audit_integration_head: 6eccd59086de53c295ed411c320cffec887d151c
affected_evidence_rerun: full test PASS; full race PASS; vet PASS; atomic
  coverage 81.2%, profile 5fac53b877bbdc8dc9c03e62e6fc9f626b2060c7924c5122c902720bb893ccc1
prior_safe_tasks_reprojected_and_reverified: the 11 safe tasks are already
  delivered in target history; none is reprojected
resolution: resolved-by-production-fix
reauthorization_identity: 0fc1e5b645e99297b8c8de8582be4ee9f28c4e6cde2821ed40c035739d80b432
```

The initial reconstruction attempt `8651b72` reproduced the reviewed tree but
not the exact frozen commit message. It and its branch/worktree are retained as
immutable failure evidence. The authorized replacement audit branch
`audit/repository-test-gaps-new-baseline-20260724-r1` and worktree
`/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/audit-replacement-r1`
are the authoritative integration mapping; no failed commit is reused.

## Module ledger

| Module | State | Task or protecting boundary |
| --- | --- | --- |
| `cmd/agentdeck` | adequately-protected | `cli-stable-error-code-matrix`, `cli-extension-backup-text-contracts` (PASS) |
| `internal/activity` | adequately-protected | `activity-read-details-resilience` (PASS) |
| `internal/backup` | adequately-protected | `backup-invalid-archive-no-target-mutation` (PASS) |
| `internal/buildinfo` | adequately-protected | development defaults and text/JSON version identity tests |
| `internal/credentialvault` | adequately-protected | `vault-malformed-inputs-fail-closed` (PASS) |
| `internal/doctor` | adequately-protected | `doctor-state-diagnostics-contract` (PASS) |
| `internal/extension` | adequately-protected | atomic scan, symlink lifecycle, adoption, and recovery tests |
| `internal/output` | adequately-protected | `output-terminal-boundaries` (PASS) |
| `internal/platform` | adequately-protected | `platform-private-state-and-machine-errors` (PASS) |
| `internal/provider` | adequately-protected | `provider-cross-client-failure-isolation` (PASS) |
| `internal/providermeta` | adequately-protected | `providermeta-canonical-boundaries` (PASS) |
| `internal/session` | adequately-protected | `session-index-transition-atomicity` (PASS) |
| `internal/store` | adequately-protected | `store-migration-atomic-rollback` (PASS) |
| `internal/usage` | adequately-protected | `usage-price-refresh-failure-boundaries` (PASS) |
| `internal/watch` | adequately-protected | `watch-lifecycle-and-release` (PASS) |
| `tools/genprices` | adequately-protected | `genprices-network-commit-errors` (PASS) |

There are no proposed or approved exclusions and no unconfirmed module ledger
entries. The two adequately protected modules have no new task.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| `backup-invalid-archive-no-target-mutation` | ✅ | ✅ |
| `vault-malformed-inputs-fail-closed` | ✅ | ✅ |
| `store-migration-atomic-rollback` | ✅ | ✅ |
| `doctor-state-diagnostics-contract` | ✅ | ✅ |
| `usage-price-refresh-failure-boundaries` | ✅ | ✅ |
| `providermeta-canonical-boundaries` | ✅ | ✅ |
| `genprices-network-commit-errors` | ✅ | ✅ |
| `session-index-transition-atomicity` | ✅ | ✅ |
| `activity-read-details-resilience` | ✅ | ✅ |
| `watch-lifecycle-and-release` | ✅ | ✅ |
| `provider-cross-client-failure-isolation` | ✅ | ✅ |
| `platform-private-state-and-machine-errors` | ✅ | ✅ |
| `cli-stable-error-code-matrix` | ✅ | ✅ |
| `cli-extension-backup-text-contracts` | ✅ | ✅ |
| `output-terminal-boundaries` | ✅ | ✅ |

## New-baseline task integration

Task content was reconstructed from the production-fixed baseline, freshly
verified and reviewed, and integrated in the frozen order. No old-baseline task
or audit commit was reused.

| Task | Source task commit | Audit integration commit | Manifest | Review |
| --- | --- | --- | --- | --- |
| `usage-price-refresh-failure-boundaries` | `b8de77419943d32810ad6aef290a1f706a559185` | `af41d5840bc78c299be5ea3049c599567d993125` | `f6e2b01be165b42ff5806a14ff291daab6967295e1a91f8b351505bcfc524bf4` | PASS round 2 |
| `providermeta-canonical-boundaries` | `5f9d56f26d9b4ac57a284186d419bbc7e06f9c2c` | `5eed40fe3d55b5d26cc960b1d5a6803ee7c1cf69` | `2ee3d10d55196d569e5e320505492f4ada9f75ee315591f88e92b412b7a385b4` | PASS round 1 |
| `genprices-network-commit-errors` | `e825d24a00c18917ea6025ba1ab125a2a447b662` | `f65856d2f5da0569b76697bec13544f978985352` | `82e983adca576925eccb8832355a30d86ede4c88614fb8c4f4c50fede6d33972` | PASS round 3 |
| `session-index-transition-atomicity` | `3a6b7aa48f3f213a9b262fde024f2d44d912651d` | `2da75f63dcbe8cf7e829c97d5ddfabf6696ad028` | `e57a1574a7c2a52eea5427c7edd7e2bb65d95434a2a8b4cdc6eed58f726bc4cc` | PASS round 2 |

The repaired task-content audit head is
`f65856d2f5da0569b76697bec13544f978985352`. Repository-wide aggregate
verification passed against the earlier audit content
`926ed55239a75fa3962a23b7684f829c638d2842`; Aggregate Review Round 2 passed
at `1d617a16b5fbb3d5cf515c9ea0ff18b55bdbf8d2`. Aggregate Review Round 3 over
the repaired test state passed at
`aab3d418e69eb92559a76a46db125597ad48aaa4`.

## New-baseline aggregate verification — 2026-07-25

Verification ran against authoritative audit content
`926ed55239a75fa3962a23b7684f829c638d2842` (tree
`6cb8995f1f1bd851d9302ba031a6ebafa37a4a04`):

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go test -mod=vendor -count=1 ./...` — PASS
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go test -mod=vendor -race -count=1 ./...` — PASS
- `rtk lint env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go vet -mod=vendor ./...` — PASS
- Atomic `-covermode=atomic -coverpkg=./... -coverprofile=/private/tmp/agent-deck-test-gaps-final.cover ./...` — PASS; total statement coverage 81.9%; profile SHA-256 `d3e40f9897cb74b3ed00576ed444aa286f572a918a8961d9da28d634049e5211`
- `git diff --check 4f614d3..HEAD` — PASS; the exact diff is 21 authorized paths (17 audit documents and four test files).
- The audit worktree remained clean.

Audit Aggregate Review Round 1 at
`2b814b939018bcdf6e86176047ab691d8f9a9c5e` was BLOCKED by the corrected
15-versus-16 module-ledger authorization mismatch and the two stale status
statements repaired under approved delta package
`392fe2749a0db9c1ada5b945f06a1243a84d64878ed086601576447cc4123a19`.
Aggregate Review Round 2 passed at
`1d617a16b5fbb3d5cf515c9ea0ff18b55bdbf8d2`.

The first delivery branch then committed usage and providermeta before
genprices Delivery Task Review Round 1 returned `NEEDS-FIX`. The reviewed
manifest
`5138ed04b1a4b00ca67c9c4558acd29f7c9b7ccd680f2c8c4b7780a9fad7bab7`
did not protect byte-for-byte output preservation for four failed check-mode
paths. Failed delivery head
`725ab5aed94c3a38d7f9c8d7ebc8016e63569b33`, its worktree, and its staged
candidate remain immutable evidence.

The bounded repair adds exact pre-call snapshots and post-call byte comparisons
to every confirmed failure path. Focused `TestCheckMode` and complete
`tools/genprices` passed; repaired manifest
`82e983adca576925eccb8832355a30d86ede4c88614fb8c4f4c50fede6d33972`
received New-baseline Task Review Round 3 PASS. The verified signed mapping is
`e825d24a00c18917ea6025ba1ab125a2a447b662` ->
`f65856d2f5da0569b76697bec13544f978985352`.

Aggregate Review Round 3 reviewed
`aab3d418e69eb92559a76a46db125597ad48aaa4` (tree
`f0e423001e2a8dbd4f559264c53d42fc32594e14`) and passed. Full tests, full
race, vet, atomic coverage, exact 21-path scope, all four source-to-audit
manifests, messages, signatures, 16-module/15-task truth, review records, and
failed-evidence retention passed. Atomic statement coverage is 81.9%; the
fresh reviewer-run profile SHA-256 is
`bf63621bc88000f58c1b91df87f7d2feb1c7496940a4d373bf79eb3b887f622e`.
The separate orchestrator run produced the same 81.9% with profile SHA
`ae46b267169394045c513480dfaa2e01ca9ac19ffc40715d97a1874d6bc144fb`;
fresh-run counter ordering explains byte drift and is not treated as identity
equality.

The only residual uncertainty is that the genprices checks prove final
byte-for-byte preservation, not transient writes restored to identical bytes
or metadata-only changes. No blocker follows.

## Replacement final-state delivery — 2026-07-26

The failed first delivery remains frozen at
`725ab5aed94c3a38d7f9c8d7ebc8016e63569b33` with staged manifest
`5138ed04b1a4b00ca67c9c4558acd29f7c9b7ccd680f2c8c4b7780a9fad7bab7`.
It was not amended, reset, reused, cleaned, or merged.

The replacement branch
`delivery/repository-test-gaps-new-baseline-20260724-r1` starts at
`4f614d34d09260a52df6bd333f6dad26134e96ac` and contains exactly one final
signed commit for each remaining logical task:

| Task | Replacement delivery commit | Manifest | Delivery review |
| --- | --- | --- | --- |
| `usage-price-refresh-failure-boundaries` | `39650636fc92f884ecda5081f5d28ec22b583153` | `f6e2b01be165b42ff5806a14ff291daab6967295e1a91f8b351505bcfc524bf4` | PASS round 1 |
| `providermeta-canonical-boundaries` | `3968d703fc5ed94378fbb917c187543655a1ffbb` | `2ee3d10d55196d569e5e320505492f4ada9f75ee315591f88e92b412b7a385b4` | PASS round 1 |
| `genprices-network-commit-errors` | `02eec76513929fb321361858a00cc71d9ecad387` | `82e983adca576925eccb8832355a30d86ede4c88614fb8c4f4c50fede6d33972` | PASS round 1 |
| `session-index-transition-atomicity` | `7168079230adf8bb1fdf05b2d563f1f1782023e1` | `e57a1574a7c2a52eea5427c7edd7e2bb65d95434a2a8b4cdc6eed58f726bc4cc` | PASS round 1 |

Every replacement commit has the frozen parent, exact authorized message,
valid SSH signature, commit/review/audit manifest equality, and a clean
post-hook worktree. Production code is unchanged.

Complete delivery verification at replacement task head
`7168079230adf8bb1fdf05b2d563f1f1782023e1` passed:

- `go test -mod=vendor -count=1 ./...` — PASS
- `go test -mod=vendor -race -count=1 ./...` — PASS
- `go vet -mod=vendor ./...` — PASS
- atomic `-covermode=atomic -coverpkg=./...` — PASS, 81.9% total statement
  coverage; profile SHA-256
  `0ae5afc81ecbcae30fb747ea60b41f16e3570c1a3ea13722093660751627f54b`

The deterministic ledger contains 16 adequately protected modules, 15 tasks,
no exclusions, no `needs-tests`, no `excluded-proposed`, no `unconfirmed`
entries, and no open or awaiting-human blockers.

Archive Review Round 1 reviewed 18-path manifest
`071cae8222a71e0d28543a951182fe3ae55daa90eef87ca28d504db87093a4b8`
and returned `NEEDS-FIX`: `docs/README.md` still dated the state snapshot
2026-07-24, and this plan incorrectly said pending resolver/retirement/target
gates had proceeded. Both findings were corrected within the approved archive
paths.

Delivery Aggregate Review Round 1 passed complete manifest
`ebcfa78172407b41835694d2251a908a7669d2a318ad395cc5d686eba57998d3`,
including fresh full test/race/vet evidence, but the two subsequent archive
wording corrections changed final content identity. Fresh Archive Review and
Delivery Aggregate Review Round 2 therefore reviewed the corrected candidate.

Archive Review Round 2 passed 18-path manifest
`e85249aaf94bc12af2fc0a924082aa345a735892edc7faba9510b23423895260`.
Delivery Aggregate Review Round 2 reviewed complete manifest
`b4f521fba6f9dd42dc84069951a21a71b01af14a718682e89a5b71f46fbbce99`
and returned `NEEDS-FIX`: the final-state, retirement, and replacement-delivery
dates still said 2026-07-25 although the four replacement commits and delivery
coverage artifact were created on 2026-07-26 Asia/Shanghai. All final-state
headings, replacement Delivery Review headings, and historical retirement
frontmatter are now consistently dated 2026-07-26; the 2026-07-25 audit repair
and Aggregate Review events remain unchanged.

Fresh Archive Review and Delivery Aggregate Review Round 3 are pending against
the date-corrected candidate. The delivery-state resolver, final documentation
commit, plan retirement, local target fast-forward, and authorized replacement
cleanup must not run until both Round 3 reviews return PASS.

## Prior audit aggregate verification

This section preserves the reviewed old-baseline partial-delivery evidence. It
does not constitute an aggregate PASS for the new-baseline audit branch.

- Reviewed audit integration head:
  `5b68942b664cf538a52daf153e0b0a466ad473a1`
- Full verification was run at
  `ec9f5b324f315a4f5f2c73a282255f85a75b5a07`. The later commits through the
  reviewed head changed documentation only, so the fresh Aggregate Reviewer
  confirmed exact-state reuse of the test, race, vet, and coverage evidence.
- `go test -mod=vendor -count=1 ./...` — PASS
- `go test -mod=vendor -race -count=1 ./...` — PASS
- `go vet -mod=vendor ./...` — PASS
- Atomic `-coverpkg=./...` suite — PASS; total statement coverage 81.3%
  versus the 80.5% baseline
- Audit coverage profile SHA-256:
  `2abaddf454c73cdc5051f0666de37dd3cc0ef28b78c969f30c36b3f937f689a6`
- `git diff --check 94437ab70273d90ff01dd19e9f64a9b358e2c709..HEAD`
  — PASS
- Aggregate review Round 1 at
  `aea8eb9063bc6fa29fbc2d38fbb946b63f573590`: NEEDS-FIX. Three review records
  named nonexistent audit commit SHAs and `docs/README.md` still reported
  `6/15` completed tasks.
- Reviewed integration repair:
  `df958ccf08673935c2edaeeb6dfed340a1fb3044` ->
  `5b68942b664cf538a52daf153e0b0a466ad473a1`, canonical manifest
  `6b7d9c418e79ab6da64c48e1b2fc5738d887c3fed67ec3e24cb798b8a3ee52ca`.
  Both commits have valid SSH signatures; the repair changed only the four
  reviewed documentation paths.
- Aggregate review Round 2 at
  `5b68942b664cf538a52daf153e0b0a466ad473a1`: PASS. Both Round 1 findings are
  closed; 11 independently reviewed tasks are eligible for the authorized
  partial delivery, while the four blockers below remain excluded from it.

## Resolved production blockers

The following blocker records preserve the old-baseline diagnosis. Each
production defect was delivered separately to local `main`; no production code
is part of this test-only resumed workflow. Their tasks now have fresh
new-baseline test evidence and independent PASS review.

### `usage-price-refresh-permanent-validation-retried`

- Task: `usage-price-refresh-failure-boundaries`
- Classification: task-local production defect (resolved)
- State: resolved-by-production-fix
- Production fix:
  `571a0e3ba454e9789c0dae3932dc2e296bb684d8`
- Behavior: a valid catalog containing no accepted direct-provider records is
  a permanent validation failure and must return after one request without
  changing the existing price catalog.
- Expected: `no validated direct-provider records`, unchanged price state,
  one request.
- Actual: the error and state preservation are correct, but three requests are
  made.
- Reproduction:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go test -mod=vendor -count=1 ./internal/usage -run '^TestUpdateLiteLLMDoesNotRetryValidCatalogWithoutDirectProviders$'`
- Test manifest:
  `0a5ce2e960c348c48820bec0040e38337c9087c048efcbf297c51717ed1e0965`
- Production boundary: the fix was delivered before this resumed test-only
  workflow; production changes remain unauthorized here.
- Resume: satisfied by the approved `new-baseline` package; reconstruct the
  candidate, verify it, and obtain fresh review.
- Target eligibility: replacement delivery commit `39650636fc92f884ecda5081f5d28ec22b583153`
  passed Delivery Task Review Round 1 and complete delivery verification.

### `providermeta-non-decimal-rational-accepted`

- Task: `providermeta-canonical-boundaries`
- Classification: task-local production defect (resolved)
- State: resolved-by-production-fix
- Production fix:
  `e934f0042de5d7c7eeb945727b4fd655675d6efd`
- Behavior: provider multipliers accept only non-negative finite decimals;
  rational `a/b` syntax must be rejected.
- Expected: `NormalizeMultiplier("1/3")` returns
  `ErrInvalidMultiplier`.
- Actual: it returns `"0.333333333333", nil` because production passes the raw
  string to `big.Rat.SetString`.
- Reproduction:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/providermeta -run '^TestNormalizeMultiplierCanonicalBoundaries$'`
- Test manifest:
  `60c9a14b62adf9145a5cb1bdc0f22560691a754d3dd636b81b4934aa0377ae42`
- Production boundary: the fix was delivered before this resumed test-only
  workflow; production changes remain unauthorized here.
- Resume: satisfied by the approved `new-baseline` package; reconstruct the
  complete logical candidate, verify it, and obtain fresh review.
- Target eligibility: replacement delivery commit `3968d703fc5ed94378fbb917c187543655a1ffbb`
  passed Delivery Task Review Round 1 and complete delivery verification.

### `genprices-latest-commit-resolver-validation-001`

- Task: `genprices-network-commit-errors`
- Classification: task-local production defect (resolved)
- State: resolved-by-production-fix
- Production fix:
  `c4abf8700757c5429b6c24d139b077dde01a0183`
- Behavior: unpinned generation must classify malformed commit JSON at the
  resolver layer and reject non-SHA revisions before a catalog request.
- Expected: malformed JSON returns a `resolve latest LiteLLM commit` error;
  non-SHA `main` returns `response contained an invalid SHA`; each makes only
  the commit API request and leaves the existing output unchanged.
- Actual: malformed JSON is returned raw; `main` is logged as pinned and used
  for a catalog request before downstream generation rejects it. Output remains
  unchanged.
- Reproduction:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -v ./tools/genprices -run '^TestRunRejectsInvalidLatestCommitBeforeCatalogFetch$'`
- Test manifest:
  `2f2e9c4cff9cbec5678d8ebbc69a606e024e8ae2fae49638c8e0a52b339266d7`
- Production boundary: the fix was delivered before this resumed test-only
  workflow; production changes remain unauthorized here.
- Resume: satisfied by the approved `new-baseline` package; reconstruct the
  complete logical candidate, verify it, and obtain fresh review.
- Target eligibility: replacement delivery commit `02eec76513929fb321361858a00cc71d9ecad387`
  passed Delivery Task Review Round 1 and complete delivery verification.

### `session-index-atomic-transitions-and-source-boundaries`

- Task: `session-index-transition-atomicity`
- Classification: task-local production defect (resolved)
- State: resolved-by-production-fix
- Production fix:
  `3c80e4a9ad025375d337a7ef8f9cda065bc797f5`
- Behavior: failed ReplaceDocuments, Exclude, and Rebuild transitions preserve
  the complete session index; project/path exclusions remove only the exact
  matching source or project.
- Expected: no synthetic source leak, no partial exclusion or rebuild state,
  and a lower-priority fallback source remains searchable when only the higher
  source is excluded.
- Actual: first ReplaceDocuments failure leaves one synthetic source; failed
  Exclude commits its control row and document deletion; failed Rebuild leaves
  only the earlier rebuilt source; project/path exclusion deletes the fallback
  document while retaining its metadata.
- Reproduction:
  `rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -v ./internal/session -run '^(TestReplaceDocumentsFirstInsertFailureIsAtomic|TestReplaceDocumentsFailureIsAtomic|TestExcludeFailureIsAtomic|TestRebuildFailurePreservesIndex|TestExcludeExactBoundaries)$'`
- Test manifest:
  `cce7277dfd7006e3d3eb366360d7516822dedbc92488398d31574889f3262d0b`
- Production boundary: the fix was delivered before this resumed test-only
  workflow; production changes remain unauthorized here.
- Resume: satisfied by the approved `new-baseline` package; reconstruct the
  complete logical candidate, verify it, and obtain fresh review.
- Target eligibility: replacement delivery commit `7168079230adf8bb1fdf05b2d563f1f1782023e1`
  passed Delivery Task Review Round 1 and complete delivery verification.

## Frozen tasks

All tasks are Wave 1, have no shared fixture owner, and may write only the
listed test files.

| Task | Behavior at risk | Exclusive writable files | Final delivery title |
| --- | --- | --- | --- |
| `backup-invalid-archive-no-target-mutation` | Invalid encrypted archives must be authenticated and validated before a restore target is created, chmodded, or populated. | `internal/backup/backup_test.go` | `test(backup): protect restore target from invalid archives` |
| `vault-malformed-inputs-fail-closed` | Malformed key framing and ciphertext metadata must fail closed without key replacement or plaintext exposure. | `internal/credentialvault/vault_test.go` | `test(credentialvault): reject malformed key and ciphertext inputs` |
| `store-migration-atomic-rollback` | Incremental and bootstrap migrations must roll back successful earlier statements when a later statement or callback fails. | `internal/store/store_test.go` | `test(store): protect atomic migration rollback` |
| `doctor-state-diagnostics-contract` | Permissions, lock state, and database failures must be classified exactly without mutating persistent state. | `internal/doctor/doctor_test.go` | `test(doctor): protect immutable state diagnostics` |
| `usage-price-refresh-failure-boundaries` | Distinguish retryable truncated JSON from permanent malformed and semantic catalog failures, preserve cancellation identity and existing state, and freeze exact request counts. | `internal/usage/usage_test.go` | `test(usage): protect price refresh failure boundaries` |
| `providermeta-canonical-boundaries` | Protect blank defaults, ordinary decimals, twelve-place rounding, and fail-closed rejection of rational, exponent, radix, signed, padded, and incomplete multiplier syntax. | `internal/providermeta/metadata_test.go` | `test(providermeta): protect canonical multiplier boundaries` |
| `genprices-network-commit-errors` | Protect latest-commit error wrapping, invalid resolved and explicit pins, zero-network rejection, check-mode pinning, and unchanged output on failure. | `tools/genprices/main_test.go` | `test(genprices): protect commit resolution failure boundaries` |
| `session-index-transition-atomicity` | Protect atomic ReplaceDocuments, Exclude, and Rebuild failures, exact project and path source ownership, fallback visibility, rename and missing-source transitions, and FTS rollback. | `internal/session/session_test.go` | `test(session): protect atomic index transitions` |
| `activity-read-details-resilience` | On-demand activity reads must tolerate malformed/truncated JSONL, filter sessions, and preserve privacy. | `internal/activity/activity_test.go` | `test(activity): protect resilient safe activity parsing` |
| `watch-lifecycle-and-release` | Polling and foreground watch paths must stop promptly and release scan locks exactly once while preserving joined errors. | `internal/watch/watch_test.go` | `test(watch): protect foreground lifecycle cleanup` |
| `provider-cross-client-failure-isolation` | A failed selection for one client must not disturb another client's completed selection or lose journal failures. | `internal/provider/service_test.go` | `test(provider): isolate clients across selection failures` |
| `platform-private-state-and-machine-errors` | State roots must be private, filesystem failures ordered, and machine-identity failures stable across build tags. | `internal/platform/state_test.go`, `internal/platform/machine_darwin_test.go`, `internal/platform/machine_unsupported_test.go` | `test(platform): protect private state and identity failures` |
| `cli-stable-error-code-matrix` | Wrapped domain failures must retain stable JSON error codes and process exit classification. | `cmd/agentdeck/error_code_test.go` | `test(cli): protect stable error-code mapping` |
| `cli-extension-backup-text-contracts` | Extension and backup text output must expose required fields while JSON remains structured and separate. | `cmd/agentdeck/renderers_test.go` | `test(cli): protect extension and backup output contracts` |
| `output-terminal-boundaries` | Table cells must remove supported terminal controls and preserve Unicode border alignment on incomplete input. | `internal/output/table_test.go` | `test(output): protect ANSI and Unicode table boundaries` |

The doctor task deliberately does not assert that
`sessions.sqlite3-wal`/`sessions.sqlite3-shm` never appear; that known
documentation-versus-runtime question is already tracked in `docs/README.md`.
The session atomicity task is expected to be capable of exposing a production
defect. If it does, its RED evidence becomes a task-local blocker and production
code remains unchanged.

## Worktrees and delivery projection

- Audit:
  `audit/repository-test-gaps-new-baseline-20260724-r1` at
  `/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/audit-replacement-r1`
- Tasks:
  `test/repository-test-gaps-new-baseline/<task-id>` at
  `/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/task-<task-id>`
- Integration repair:
  `repair/repository-test-gaps-new-baseline-20260724` at
  `/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/integration-repair`
- Delivery:
  `delivery/repository-test-gaps-new-baseline-20260724` at
  `/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/delivery`
  is retained immutable failed-delivery evidence.
- Replacement delivery, eligible after repaired Aggregate Review Round 3 PASS:
  `delivery/repository-test-gaps-new-baseline-20260724-r1` at
  `/private/tmp/agent-deck-repository-test-gaps-20260724-new-baseline/delivery-r1`
  contains the four reviewed final task commits through
  `7168079230adf8bb1fdf05b2d563f1f1782023e1`.

Each candidate is staged from its exact owned path set, checked for unstaged
owned content with Git's filter-aware diff, bound with
`content-manifest.py --index`, and reviewed by a fresh read-only reviewer before
the primary orchestrator creates an SSH-signed task commit. The audit branch
receives reviewed task commits in frozen order. The delivery branch is rebuilt
from the authorized baseline and receives one independently reviewed final
commit per safe task, followed by one reviewed documentation commit.

If any task is blocked by a production defect, the separately authorized
partial-delivery path may project only the independently PASSed tasks. The plan
and review records then remain active on the retained audit branch. No partial
delivery retires this plan.

## Verification

Per task:

1. focused behavior test;
2. affected package test;
3. exact owned-path status and manifest checks;
4. fresh read-only task review.

After audit integration, run:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 \
  -covermode=atomic -coverpkg=./... \
  -coverprofile=/private/tmp/agent-deck-test-gaps-final.cover ./...
rtk git diff --check
```

The platform task additionally compiles/tests the non-Darwin build-tag path
with all artifacts under `/private/tmp`. L4 `make release-verify` is out of
scope because this is test-only L3 work, not release validation.

## Failure and review policy

- Reviewers are fresh and read-only for every round.
- The maximum automatic review count is three rounds per candidate.
- A Writer may repair only its owned test files.
- Production defects are recorded as task-local, dependency, or global
  blockers; production repair requires a separate workflow and authorization.
- No failing regression test is committed or integrated.
- Audit or aggregate repairs happen only on the authorized integration-repair
  branch.
- Any ownership, manifest, signing, hook, delivery-state, or target-identity
  uncertainty fails closed.

## Historical task-start recipe

This plan is retired and must not be used to start new work. The former task
recipe was: select a task ID from the Status table, bind its exclusive writable
files and verification policy, then require a fresh read-only review before
integration. All listed tasks completed that workflow. Any future gap requires
a new active plan and fresh authorization.
