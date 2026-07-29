---
status: historical
plan: project-attribution
task: headroom-wrapper-kind
---

# Review log — project-attribution / headroom-wrapper-kind

## Round 1 — 2026-07-28

- Reviewed state: base `8c053c9`, uncommitted working tree carrying the
  `headroom-wrapper-kind` implementation, its three new test files, the plan, and
  the docs index.
- Reviewer: Claude Opus 5.
- Scope: migration 16 and `CurrentSchemaVersion`; `store.Provider.WrapperKind`
  and its read path; `SetProviderWrapper` / `SetOfficialWrapperURL` signature
  changes and both storage paths; `NormalizeWrapperKind`,
  `reportedWrapperKind`, `Service.SetWrapper`, `storedProvider`,
  `officialDefinition`; the `--kind` flag, its intent guard, and the
  `wrapperCell` renderer; all three acceptance criteria; the additive-reporting
  claim across every provider surface in both formats. Findings were derived
  from source and from probes against built binaries, not from the task's
  completion note — that note's own atomicity claim is one of the findings.

### Acceptance criteria

All three hold. Each was reproduced independently of the committed tests.

**1. An existing database opens with every wrapper reading back as `plain`.**
Verified on the real upgrade path rather than the v6 SQL fixture the committed
test replays. A binary built from `HEAD` (via `git archive` into a scratch
directory — no worktree, no change to the working tree) created a state
directory at schema 15 with a wrapper on both a stored provider and the built-in
one. The current binary then opened it: schema 15 → 16, both wrapper URLs
preserved verbatim, `wrapper_kind` `NULL` in the row and absent from
`provider show --format json` for both providers.

**2. A `plain` wrapper produces byte-identical behavior to today on every
command.** Verified by differential execution. The same sequence (`provider
add`, two undeclared `set-wrapper` calls) ran against the `HEAD` binary and the
current binary in separate state directories, then every provider reporting
surface was captured in both formats: `provider list`, `provider show example`,
`provider show official`, `provider status`, `provider status example`,
`provider current`. After normalizing timestamps, **the diff is empty**.

Corroborated by evidence the implementer did not cite: `TestIsolatedEndToEndFlow`
passes against the committed `cmd/agentdeck/testdata/phase7/gui-json-contract.json`,
which declares `wrapper_url` and `wrapper_events` but no `wrapper_kind`. That
golden is a whole-envelope comparison over a flow that calls
`provider set-wrapper`, so it pins the additive guarantee independently.

**3. `--clear` removes the declaration along with the URL.** Verified by direct
SQLite inspection on both storage paths, which is where they could diverge. After
declaring `headroom` on a stored provider and on `official` and clearing both:
the providers row holds `NULL` in both columns, and
`SELECT count(*) FROM settings WHERE key LIKE 'official.wrapper%'` returns `0`.
Re-setting a URL afterwards does not reattach the previous declaration.

Additive reporting reaches every surface claimed: with a declaration present,
`wrapper_kind` appears in `provider show`, `list`, and `status` JSON and as a
`(headroom)` annotation in the matching text; with none, it is absent from all.

### Findings

- **[P2] A URL-only `set-wrapper` silently drops an explicit `--kind headroom`.**
  Reproduced: after `set-wrapper example --url https://old-proxy.example --kind
  headroom`, a plain `set-wrapper example --url https://new-proxy.example` — an
  ordinary "the proxy moved" edit — leaves the declaration at `plain`, with an
  unchanged success message. The only signal is an annotation that stopped
  appearing. This contradicts the sibling command on the same noun:
  `provider update` is a partial patch (`cli-manual.md` states 未指定字段保持不变,
  confirmed by probe — an endpoint survived a `--multiplier`-only update), so a
  user has no reason to expect `set-wrapper` to be a full replace. Scoped by this
  plan itself: `run-env-injection` reads the declaration to decide whether to
  attribute a launch, so once it ships this flow silently disables attribution
  with no error and no advisory. Not a P1 — it breaks none of the three stated
  acceptance criteria — but it must settle before a task depends on the
  declaration surviving. → Either preserve an existing declaration when `--kind`
  is omitted, matching `provider update`; or keep replace semantics and state
  them in the mutation text and the manual so the loss is visible when it
  happens.
- **[P3] The service never sends the store's "no declaration" value.**
  `NormalizeWrapperKind("")` returns `WrapperKindPlain`, so `Service.SetWrapper`
  always passes `"plain"`; the store's `kind == ""` → `DeleteSetting` branch and
  its `nullableString(kind)` `NULL` path are unreachable from the product. Two
  consequences: one logical state has two on-disk encodings (`NULL` for a
  pre-existing wrapper, `'plain'` for one written by this build, confirmed by
  SQL), and `TestSetOfficialWrapperURLReplacingAKindDropsThePreviousOne`
  exercises a call shape the CLI never makes, so it does not protect the real
  path. Reporting is correct either way because `reportedWrapperKind` maps both
  to `""`. → Hygiene: either let the service pass `""` for the default, or drop
  the unreachable store branch and retarget its test.
- **[P3] The completion note overclaims atomicity on the built-in path.** The
  note says clearing "clears the declaration in the same statement". True for a
  stored provider — one `UPDATE providers SET wrapper_url=?,wrapper_kind=?,…`.
  False for `official`: `SetOfficialWrapperURL` performs two independent
  operations with no surrounding transaction, while this same file uses `tx` for
  multi-statement work elsewhere. A failure between them orphans a declaration
  whose URL is gone — the state the plan's invariant says cannot exist. Practical
  risk is very low (two single-statement local SQLite writes); the finding is
  that the note asserts a guarantee the code does not make on one of its two
  paths. → Correct the note, or wrap the pair in a transaction and keep the
  claim.
- **[nit] The kind vocabulary is case-sensitive.** `--kind Headroom` is rejected;
  the error names the accepted values, so recovery is immediate. Recorded, no
  action required.

### Checked and clean

- Neither `INSERT INTO providers` statement names the wrapper columns, so a newly
  added provider gets `NULL` and reads back as the default.
- No `SELECT *` on `providers`, so the added column disturbs no positional scan.
- `ProviderByName` derives from `ListProviders`, so it picks the column up
  without a second query to keep in sync.
- `ErrInvalidProvider` reaches the input-error classifier at `main.go:322`, so an
  unknown protocol exits 2 — matching the flag-validation error beside it rather
  than being reported as a runtime failure.
- The migration adds a nullable column with no default and no backfill, so it
  cannot opt an existing wrapper into a protocol its owner never chose.
- The implementer's RED claim was re-derived rather than trusted: with
  `reportedWrapperKind` returning its argument unchanged,
  `TestUndeclaredWrapperIsReportedExactlyAsBefore`,
  `TestReportedWrapperKindHidesTheDefault`, and
  `TestSetWrapperWithoutAKindReportsNothingNew` all fail.

### Evidence

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — exit 0,
  16 packages.
- `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` — clean.
- `gofmt -l internal cmd` — clean; `git diff --check` — exit 0.
- `go test -mod=vendor ./cmd/agentdeck/ -run TestIsolatedEndToEndFlow` — PASS,
  so the golden envelope comparison ran and matched.
- Upgrade probe: `HEAD` binary wrote schema 15 with wrappers set; current binary
  read it back at schema 16 with `wrapper_kind` `NULL` and both URLs intact.
- Differential probe: six provider surfaces × text and JSON, `HEAD` binary vs
  current binary, timestamps normalized — empty diff.
- SQLite inspection of `providers` and `settings` across declare → clear →
  re-set on both storage paths.

**Verdict: REOPEN.** One P2, two P3, one nit; no correctness or security defect,
and all three acceptance criteria met. `Dev` unticked until the P2 closes.

## Round 2 — 2026-07-28 (fix round, recorded by the implementer)

Not a review pass. This section records what the fix round changed, so Round 3
can verify against a written claim rather than re-deriving intent.

- Reviewed state at fix time: base `8c053c9`, uncommitted working tree.
- Findings addressed: all four from Round 1.

- **[P2] Closed by removing the silence, not the semantics.** The user rejected
  the patch-semantics option on the grounds that `set-wrapper` is an atomic
  whole-wrapper set and this CLI's only partial-update verb is `provider update`.
  That is the decision recorded here. `Service.SetWrapper` now returns the
  non-default declaration a call replaced, and the command layer prints
  `advisory: wrapper kind reset to plain (was headroom); pass --kind headroom to
  keep it` on stderr. Silent cases: first set, `plain`→`plain`,
  `headroom`→`headroom`, and `--clear`. The dropped value is a return value, not
  a `DefinitionResult` field, so it cannot reach the JSON envelope. The mutation
  text is untouched, per the `cli-route-surface` precedent against echoing stored
  values there.
- **[P3-1] Closed by making the unreachable branch reachable.** `storedWrapperKind`
  persists the default as absence, mirroring `reportedWrapperKind` on the read
  side, so the two on-disk encodings collapse to one and the store's `kind == ""`
  path is now the shape the CLI actually produces. The existing store test was
  kept and its comment corrected rather than retargeted.
- **[P3-2] Closed in code.** `SetOfficialWrapperURL` performs its
  delete-then-insert pair in one transaction, matching
  `AddProviderWithCredential` in the same file. The Done note's "same statement"
  wording was corrected, since the stored-provider and built-in paths reach
  atomicity by different means.
- **[nit]** Left alone, as recorded.

Evidence: `go test -mod=vendor ./...` exit 0 across 16 packages; `go vet` clean;
`gofmt -l internal cmd` and `git diff --check` clean. The Round 1 differential
probe was re-run against the fixed build (six provider surfaces × text and JSON,
`HEAD` binary vs fixed binary, timestamps normalized) and remains an **empty
diff**. Storage encoding confirmed against the built binary: an undeclared
`set-wrapper` leaves `wrapper_kind` `NULL` and writes one settings row for the
built-in provider.

Open for Round 3 to judge: whether the advisory's trigger condition is exactly
"a non-default declaration was dropped" on every reachable path; whether the
pre-write read introduces any ordering change to the write path's errors; and
whether the transaction actually covers both rows on every branch.

## Round 3 — 2026-07-28 (re-review)

- Reviewed state: base `8c053c9`, uncommitted working tree after the Round 2 fix
  round.
- Reviewer: Claude Opus 5.
- Scope: the three Round 1 findings, judged closed or not; the three questions
  Round 2 left open; and a hunt for regressions the fix itself could have
  introduced, in particular the replacement of `SetSetting`/`DeleteSetting` with
  a transactional DELETE+INSERT pair.

### Findings from Round 1

- **[P2] Closed.** Semantics are unchanged and now verified on both storage
  paths: a URL-only call returns the declaration to the default for a stored
  provider and for the built-in one. The advisory fires exactly when a
  non-default declaration was dropped. Derived from source: `dropped` is set iff
  `previous != "" && previous != reportedWrapperKind(normalizedKind)`, where
  `previous` is the *reported* kind and so is non-empty only for a non-default
  declaration, and where clearing skips the read entirely so `previous` stays
  empty. Enumerating every reachable combination yields exactly the intended
  matrix, and an empirical run confirmed it: fires on `headroom`→default for both
  paths; silent on first set, `plain`→`plain`, declaring `headroom`,
  `headroom`→`headroom`, and `--clear`.
- **[P3-1] Closed.** `storedWrapperKind` persists the default as absence, so one
  logical state has one encoding. Confirmed against the built binary: an
  undeclared `set-wrapper` leaves `wrapper_kind` `NULL` and writes a single
  settings row for the built-in provider. The store branch Round 1 called
  unreachable is now the shape the CLI produces, so the existing store test
  protects a real path rather than a hypothetical one.
- **[P3-2] Closed.** The transaction covers both rows on all three branches —
  clear, set with a kind, set without one — because the `DELETE ... WHERE key IN
  (?,?)` always names both keys and sits inside the transaction, and the
  conditional inserts follow it. The Done note's wording was corrected to
  distinguish the one-statement and one-transaction paths.
- **[nit]** Left alone, as agreed.

### Round 2's open questions

1. **Is the advisory's trigger condition exactly "a non-default declaration was
   dropped" on every reachable path?** Yes, by the enumeration above, confirmed
   empirically on both storage paths.
2. **Does the pre-write read change the write path's error ordering?** No. The
   read runs after input validation and its error is discarded, so an unknown
   provider still surfaces `sql.ErrNoRows` from the write exactly as before, and
   invalid input is still rejected before any read. Confirmed: `set-wrapper` on a
   nonexistent provider still fails with the same error and prints no advisory.
3. **Does the transaction cover both rows on every branch?** Yes, per the reading
   above.

### Regressions hunted

- **The `SetSetting` swap.** Round 2 replaced a conditional UPSERT
  (`... ON CONFLICT DO UPDATE ... WHERE settings.value IS NOT excluded.value`)
  with an unconditional DELETE+INSERT. Checked specifically because that could
  have changed observable behavior: `settings` is
  `(key TEXT PRIMARY KEY, value TEXT NOT NULL)` with no `updated_at` column and
  no triggers anywhere in the migrations, so the two are observationally
  equivalent. `secureFiles()` now runs once after commit instead of once per
  statement, which is equivalent or stricter.
- **Advisory leaking into machine output.** Verified with clean separated
  capture rather than an inline redirection: stdout carries only the mutation
  text, stderr carries only the advisory, and under `--format json` the envelope
  contains no occurrence of "advisory", "headroom", or "reset" anywhere —
  `warnings` is `[]` and `partial` is `false`. `--quiet` suppresses the advisory
  while the drop still takes effect.
- **Advisory on a failed command.** Cannot happen: the command layer returns on
  error before calling `reportDroppedWrapperKind`.
- **Byte-identical acceptance criterion.** The Round 1 differential probe was
  re-run against the fixed build — six provider surfaces × text and JSON, `HEAD`
  binary vs fixed binary, timestamps normalized, 58 captured lines each —
  **empty diff**. The additive guarantee survives the fix.

### New observations

- **[nit] The pre-write read is heavier than it needs to be.** `s.Show` goes
  through `ListProviders`, which calls `ListProviderCredentials` for every
  provider. No ciphertext is read and neither the vault nor the key file is
  touched — the secret check is `EXISTS(SELECT 1 FROM credential_secrets …)` —
  and this read was already on the path before the fix, which now adds a second
  one. A targeted lookup of just this provider's wrapper kind would avoid it. Not
  a defect and not blocking.
- **Evidence-trail note.** The first attempt to re-run the differential probe
  reported a 5-line diff. That was an artifact: the probe script had been pruned
  from the scratch directory, so neither run executed and the only difference was
  the shell's error line number. Recreated and re-run correctly, the diff is
  empty. Recorded so a reader of this trail does not mistake the earlier number
  for a product signal.

### Evidence

- `go test -mod=vendor ./...` — exit 0, 16 packages, 0 `FAIL` lines.
- `go vet -mod=vendor ./...` — clean; `gofmt -l internal cmd` — clean;
  `git diff --check` — exit 0.
- Advisory trigger matrix run against the built binary on both storage paths.
- Separated stdout/stderr capture in text and JSON modes, plus a full-envelope
  scan for advisory text.
- Differential probe, `HEAD` vs fixed, empty diff.
- Storage inspection via `sqlite3` for the encoding and the settings row pair.

**Verdict: PASS.** All three Round 1 findings are closed at the root cause rather
than deferred, Round 2's three open questions are answered, and no new finding at
or above P3 was found. The plan's `Review` cell may be ticked.
