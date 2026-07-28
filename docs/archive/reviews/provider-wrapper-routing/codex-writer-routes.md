---
status: historical
plan: provider-wrapper-routing
task: codex-writer-routes
---

# Review log — provider-wrapper-routing / codex-writer-routes

## Round 1 — 2026-07-27

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- internal/provider/config.go internal/provider/config_test.go` =
  `290425d700a1ab41…` (the same file pair carries `claude-writer-routes`,
  reviewed separately in this directory; only the Codex writers and the
  extracted helper are in scope here).
- Reviewer: Claude Opus 5 (read-only pass, independent of the implementation
  session)
- Scope: `internal/provider/config.go` — the new `rewriteCodexCustomTable`
  helper (`:114-196`), the rewritten `WriteOfficialCodexConfig` (`:198-241`),
  the new `WriteCodexWrapperConfig` (`:243-299`), the two new regexps
  (`:84,86`), and the five new `WriteCodexWrapperConfig` tests. Behavior
  checked against `docs/specs/cli-design.md` "Owned Client Configuration
  Fields", and against the pre-refactor `WriteOfficialCodexConfig` for the
  claimed behavioral equivalence.
- Findings:
  - [P2] **The refactor is not behavior-preserving, contrary to the task
    note.** The old loop reset its owed-`name` state on entering the custom
    table (`customNameSeen = false` beside `customTableSeen = true`); the
    extracted helper dropped that reset, and neither caller's `flush` closure
    restores it — both set `nameSeen = true` and never clear it. The helper's
    own doc comment states the contract that is being broken: "it runs once
    per occurrence of the custom table (its owed-field state must reset
    itself)". Consequence on an array-of-tables source, which
    `tomlTablePattern` matches (`\[\[?…]]?`) and `toml.Unmarshal` accepts:

    ```
    in   model_provider = "custom" / [[model_providers.custom]] /
         base_url = "https://a.example/v1" / [[model_providers.custom]] /
         base_url = "https://b.example/v1"
    old  …[[…]] name = "official" [[…]] name = "official"
    new  …[[…]] name = "official" [[…]]            <- second element empty
    ```

    Verified by probe against the current tree; the old result is derived from
    the removed reset. `WriteCodexWrapperConfig` inherits the same asymmetry
    (both elements get `base_url`, only the first gets `name`). Codex's real
    config uses a plain `[model_providers.custom]` table, so this is
    pathological input rather than a live user path, and no test covers it —
    which is exactly why it slipped through. -> Either restore the per-
    occurrence reset in both `flush` closures (matching the documented
    contract), or, if array-of-tables is deliberately out of scope, delete
    that sentence from the doc comment and say so; add a test pinning
    whichever behavior is chosen, and correct the task note's "pure refactor"
    claim.
  - [P3] `WriteCodexConfig` (the direct, credentialed path) still rewrites
    the whole file through `toml.Marshal`, destroying comments and ordering,
    while `WriteCodexWrapperConfig` preserves them line by line. The spec says
    AgentDeck "never writes, clears, or reorders any other field" for both.
    The divergence is pre-existing, not introduced here, but this task is the
    first time two Codex writers with opposite fidelity sit side by side, and
    a user switching the same provider direct vs `--via` now gets a very
    different diff in `config.toml`. -> Not a blocker for this task; record
    the gap so `route-composition` or a follow-up decides whether
    `WriteCodexConfig` should move onto the shared helper.
  - [nit] `flush` is documented as running "once per occurrence of the custom
    table", but its two call sites are "on leaving the table" and "at end of
    file"; when the table is absent it does not run at all (`ensureTable`
    does). The sentence's third clause ("or because the table was absent") is
    wrong. -> Tighten the comment.
- Not defects (checked and dismissed):
  - CRLF sources round-trip correctly, including the line ending on flushed
    fields: a `\r\n` fixture with a bearer token produced
    `name = "official"\r\n` + `base_url = "…/v1"\r\n` with `\r\n` throughout.
  - `fmt.Sprintf("%q", name)` produces a valid TOML basic string for the
    quoting cases that matter (`we"ird\name` → `"we\"ird\\name"`); provider
    names are validated upstream regardless.
  - `base_url` is built as `strings.TrimRight(endpoint, "/") + "/v1"`,
    byte-identical to `WriteCodexConfig:106`, so direct and wrapper routes
    agree on the `/v1` convention and pair correctly with
    `NormalizeWrapperURL` stripping a trailing `/v1` before storage.
  - The line-ending bug the task note describes (a dropped line still
    contributing its newline) is genuinely fixed: `result` gains an ending
    only when `onLine` returns a non-nil replacement, and the pre-existing
    `TestWriteOfficialCodexConfigPreservesInsertionBoundaries` passes.
  - `requires_openai_auth` and `wire_api` survive because no wrapper pattern
    matches them and unhandled lines fall through to a verbatim append;
    asserted by two of the new tests.
  - Flush placement puts owed fields after any trailing blank line inside the
    custom table, so they visually attach to the following table header. This
    is unchanged from the old `appendMissingCustomName` placement, cosmetic,
    and not introduced here.
  - A source without a trailing newline gains one; same as before the
    refactor.
  - Atomic-replace failure leaves the original bytes intact, and the
    temporary file is chmod-ed before the write — covered by
    `TestWriteCodexWrapperConfigFailureLeavesOriginalBytes`.
- Evidence (run by this review against the reviewed state):
  - `go test -mod=vendor ./internal/provider/ -run Codex -v` → 15 tests pass,
    including all five pre-existing `WriteOfficialCodexConfig` tests and the
    five new `WriteCodexWrapperConfig` tests.
  - `go vet -mod=vendor ./...` → clean; `gofmt -l` clean on both changed
    files; `git diff --check` clean.
  - Six throwaway probe tests (added, run, removed; working tree confirmed
    unchanged afterwards) covering array-of-tables, CRLF, quote-bearing
    names, a source with no trailing newline, and a custom table that is not
    the last table and owes both fields.
- Verdict: REOPEN — the new wrapper writer is correct and well covered for
  every realistic input, but the accompanying refactor changed
  `WriteOfficialCodexConfig`'s behavior on a valid-TOML input class while the
  task note asserts it did not, and the helper's own doc comment describes the
  contract that was dropped. Returned to `Dev` for the P2. The P3 and the nit
  may be closed here or explicitly deferred with a recorded decision. Next
  pass is Round 2 in this file.

## Round 2 — 2026-07-27

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- internal/provider/config.go internal/provider/config_test.go` =
  `bdfbf4d592ba4b04…` (Round 1 reviewed `290425d700a1ab41…`; the intervening
  `3360722430769fe2…` was the `claude-writer-routes` Round 2 state, so this
  hash covers both tasks' fix rounds and only the Codex side is judged here).
- Reviewer: Claude Opus 5 (re-review pass; independent re-verification of each
  Round 1 finding plus a regression sweep of the shared helper)
- Scope: the Round 1 finding set, the fix-round edits to
  `rewriteCodexCustomTable` (new `onEnter` parameter and rewritten doc
  comment), both callers' `onEnter` closures, the two new array-of-tables
  regression tests, and the `codex-writer-routes` plan note.
- Round 1 finding status:
  - [P2] refactor not behavior-preserving — **fixed**. `rewriteCodexCustomTable`
    now takes `onEnter func()` and invokes it at `config.go:158`, inside the
    table-header branch, after `table` is reassigned and before any of that
    occurrence's lines reach `onLine`. Ordering is correct: the previous
    occurrence's `flush` runs at `:153` against the *old* `table` value, so
    owed fields are emitted before the state is reset, not after.
    `WriteOfficialCodexConfig` resets `nameSeen`; `WriteCodexWrapperConfig`
    resets `nameSeen` and `baseURLSeen`. Both new tests reproduce the Round 1
    probe exactly and pin per-occurrence output. The task note now carries a
    fix-round section stating what the "pure refactor" claim got wrong.
  - [P3] `WriteCodexConfig` vs `WriteCodexWrapperConfig` fidelity divergence —
    **deferred with a recorded decision**, which is what Round 1 permitted.
    The plan note assigns the decision to `route-composition` (task 4) and
    names both outcomes (move `WriteCodexConfig` onto the shared helper, or
    record a further deferral). Nothing about this gap worsened in this round.
  - [nit] wrong third clause on `flush`'s doc comment — **fixed**. The comment
    now documents `onEnter`'s per-occurrence role explicitly, and describes
    `flush` as running "on leaving it for another table or at end of file; it
    does not run for a table that was never present, which is what
    ensureTable is for". That matches the code at `:153`, `:191`, and `:194`.
- Regression sweep (throwaway probe, added → run → removed; working tree
  hash confirmed identical afterwards). Nine shapes across both writers:
  - array-of-tables where the *second* element already has a `name` → both
    rewritten to `"official"`, no duplicate flushed line;
  - three occurrences with different owed states (one owes `name`, one has a
    `name`, one has only a bearer token) → each gets exactly one `name`;
  - CRLF array-of-tables → per-occurrence flush emits `\r\n` throughout;
  - wrapper on an array-of-tables where the first element is complete and the
    second is empty → first rewritten in place, second flushed both fields;
  - single custom table with a bearer token → bearer and `base_url` dropped,
    `wire_api` and the leading comment survive, `model_provider` rewritten;
  - source with no trailing newline and two occurrences → both flushed;
  - wrapper with no custom table at all → `ensureTable` path, unchanged.
  Every result matches the pre-refactor behavior the Round 1 finding derived.
- Not defects (checked and dismissed):
  - Two *singular* `[model_providers.custom]` tables in one file never reach
    the rewrite: the upfront `toml.Unmarshal` guard rejects them with
    `table custom already exists`, so the flush-per-occurrence logic is only
    ever exercised on the array-of-tables form the tests now cover.
  - When `model_provider` is absent, the prepended
    `model_provider = "custom"` line lands *above* a leading `# comment`,
    detaching it from whatever it described. Pre-existing (the old loop
    prepended identically) and cosmetic; out of scope for this task.
- Evidence (run by this round against the reviewed state):
  - `go test -mod=vendor ./internal/provider/ -run Codex -v` → 17 tests pass,
    including the two new array-of-tables regressions and all five
    pre-existing `WriteOfficialCodexConfig` tests.
  - `go test -mod=vendor ./...` → every package `ok`.
  - `go vet -mod=vendor ./...` → clean; `gofmt -l` clean on both changed
    files; `git diff --check` clean.
- Verdict: PASS — the P2 is closed by a mechanism that matches the contract
  the helper documents, with tests that fail if the reset is dropped again;
  the nit is closed; the P3 is deferred onto a named task with both outcomes
  spelled out. No new defect was introduced, and the wrapper writer's
  realistic-input coverage from Round 1 is intact.
