---
status: active
plan: project-attribution
task: project-identity
---

# Review log — project-attribution / project-identity

## Round 1 — 2026-07-28

- Reviewed state: base `5e8f8db`, uncommitted working tree carrying the
  `project-identity` implementation, tests, and plan update.
- Reviewer: user-provided review instruction.
- Scope: the shared cleaned full-path identity, base-name wire encoding,
  nameless-directory behavior, package dependency direction, targeted tests,
  and the task's Dev completion note.

### Findings

- **[P2] Bare `+` is decoder-dependent.** `url.PathEscape` leaves plus signs
  unescaped. A path-style decoder preserves them, but a form-style
  `unquote_plus` decoder turns them into spaces. Encode each remaining `+` as
  `%2B` after the existing escape step, and protect `my+project` and `c++`
  without changing the cleaned full-path identity.
- **[P3] The nameless-directory table omitted `..`.** The implementation already
  returns no attribution for it, but the acceptance test did not pin that
  behavior.
- **[P3] The new package dependency was not documented as a design choice.**
  Record that `internal/provider` intentionally imports `internal/session`
  because the task exported the existing owner rule rather than lifting it.
  Defer moving the rule into a lower-level package unless later tasks make the
  dependency spread.

**Verdict: REOPEN.** One P2 and two P3 findings require a scoped fix. Per the
task instruction, the plan's Dev cell remains checked while review remains open.

## Round 2 — 2026-07-28 (fix round, recorded by the implementer)

Not a review pass. This section records the scoped changes for the requested
re-review.

- **[P2] Addressed at the wire boundary.** `ProjectWireValue` still calls
  `url.PathEscape`, then replaces every bare `+` with `%2B`. `ProjectIdentity`
  remains the session package's cleaned full path. The test table now covers
  `my+project` → `my%2Bproject` and `c++` → `c%2B%2B`, rejects any bare plus,
  and retains the existing `url.PathUnescape` round-trip assertion.
- **[P3] `..` is now an explicit nameless-directory case.**
- **[P3] The Dev note records the intentional
  `internal/provider` → `internal/session` dependency and the condition for
  moving the rule down later; no package structure changed in this fix.

### Mutation evidence

After the fix passed, the `%2B` replacement was temporarily changed to the
compilable no-op `strings.ReplaceAll(escaped, "+", "+")`.
`go test -mod=vendor ./internal/provider -run TestProjectWireValue` then failed
both plus cases on behavior: `my+project` remained `my+project`, and `c++`
remained `c++`. Restoring `%2B` made the targeted tests pass.

### Evidence

- `go test -mod=vendor ./internal/provider -run TestProject` — exit 0.
- `go test -mod=vendor ./...` — exit 0, 16 packages.
- `go vet -mod=vendor ./...` — clean.
- `gofmt -l internal/provider/project.go internal/provider/project_test.go
  internal/session/session.go` — no output.

Next workflow: `进入复评:project attribution / project-identity`.

## Round 3 — 2026-07-28 (re-review)

- Reviewed state: base `5e8f8db`, uncommitted working tree after the Round 2
  fix.
- Reviewer: Codex, using a fresh source and test review rather than accepting
  the Round 2 claims at face value.
- Scope: closure of all three Round 1 findings, the original acceptance
  criteria, and regressions plausibly introduced by the plus-encoding fix or
  package dependency choice.

### Round 1 findings

- **[P2] Closed.** `ProjectWireValue` applies `url.PathEscape` first and then
  converts every remaining bare plus to `%2B`. Fresh targeted tests pass for
  `my+project` and `c++`, reject raw pluses, and retain the path-decoder
  round trip. An independent `unquote` / `unquote_plus` probe decoded both wire
  fixtures identically.
- **[P3] Closed.** `..` is present in the nameless-directory table and the fresh
  targeted run exercises it.
- **[P3] Closed.** The Dev note identifies the
  `internal/provider` → `internal/session` dependency as intentional, explains
  export over lift, and records the condition for moving the rule lower later.
  The dependency remains one-way; `internal/session` does not import
  `internal/provider`.

### Acceptance and regression review

- `ProjectIdentity` still delegates directly to `session.NormalizeProject`, and
  the synthetic session test compares it with the project actually stored by a
  session scan. The cleaned full-path identity did not change.
- The wire value still uses only `filepath.Base(identity)`, so the full machine
  path is not disclosed. Empty, root, `.`, and `..` values attribute nothing.
- Encoding order is safe for literal percent sequences: the initial path escape
  protects `%` before the remaining raw pluses are replaced. Newline, quote,
  non-ASCII, spaces, and pluses all remain covered by exact expected values and
  round-trip assertions.
- The added import creates no package cycle, and the extra replacement is
  linear in the already bounded project-name value.
- No new finding at P1, P2, or P3 was found.

### Evidence

- `go test -count=1 -mod=vendor ./internal/provider -run TestProject` — exit 0.
- Python `urllib.parse.unquote` / `unquote_plus` probe over `my%2Bproject` and
  `c%2B%2B` — 2/2 decoded identically.
- Round 2's `go test -mod=vendor ./...` (16 packages) and
  `go vet -mod=vendor ./...` results were reused after a fresh status and raw
  diff check showed the relevant code, tests, dependencies, and toolchain
  unchanged.

**Verdict: PASS.** All three Round 1 findings are closed at their source, the
original acceptance criteria still hold, and no new P1–P3 issue was found.
