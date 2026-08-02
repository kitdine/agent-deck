---
status: historical
created: 2026-08-02
retired: 2026-08-02
---

# Doctor Quick Diagnostics

Target release: `v0.2.1`. This plan fixes only the released quick `doctor`
path. Deeper pricing-read scalability remains separate `v0.2.2` work in
`usage-pricing-read-scalability.md`.

## Goal

- Keep default `agentdeck doctor` bounded for a large real usage database.
- Align implementation with the manual: price-depth checks belong to
  `agentdeck doctor --full`.
- Preserve every full-mode diagnostic and every existing database, text, and
  JSON contract outside the quick report's deep-price check set.

## Non-Goals

- No optimization of `usage.PriceDiagnostics` or `usage sessions`.
- No deadline, progress output, pagination, schema, or persisted-data change.
- No change to full-mode `price_provenance` or `unpriced_models` results.
- No commit, release, Homebrew update, or local installed-binary replacement.

## Evidence Baseline

Gathered on 2026-08-01 through 2026-08-02 at `886f0a8`, the commit embedded in
the installed `v0.2.1-rc.2` binary:

- The real core database is 93,982,720 bytes. Quick `doctor` repeatedly ran for
  more than 30-60 seconds without producing a report.
- No `state.lock` or competing database owner was present. A direct read-only
  `PRAGMA integrity_check` returned `ok` in 3.35 seconds, so corruption and
  integrity-scan latency are not the cause.
- Provider status, credential listing, extension diagnostics, and the macOS
  machine-identity query completed independently in milliseconds.
- `internal/doctor/doctor.go:440` calls `PriceDiagnostics` in both modes.
  `internal/usage/usage.go:1746` loads every usage event, then
  `internal/usage/usage.go:3051` performs repeated attribution and price-table
  reads for each event.
- `docs/specs/cli-manual.md` already assigns usage, session, extension, and
  price-depth checks to `doctor --full`; no specification change is required.

## Task

### `quick-price-boundary`

Move only `PriceDiagnostics` and its `price_provenance` / `unpriced_models`
report entries behind the existing full-mode boundary. Quick mode still checks:

- state permissions and lock state;
- core schema and pending operations;
- provider credential/key/configuration health;
- incomplete usage runs;
- active price-catalog availability;
- session-index and extension health.

Add a regression fixture whose malformed historical event makes deep pricing
fail. Quick mode must complete without traversing it; full mode must still reach
the deep path and return the fixture error. This proves the boundary directly
without a timing-sensitive test.

Acceptance:

- Quick reports `prices` but neither `price_provenance` nor `unpriced_models`.
- Full mode preserves both checks and their current result codes.
- No production function outside `internal/doctor` changes.
- Targeted `internal/doctor` tests and `go test -mod=vendor ./...` pass.
- If the managed environment permits real-state access, a freshly built binary
  completes quick `doctor` against the same database; otherwise record that
  acceptance as unverified rather than substituting the installed RC2 binary.

Verification: L2 because the shared CLI report check set changes. Use targeted
`internal/doctor` tests followed by the full vendor suite.

Development evidence on 2026-08-02 at `886f0a8` plus the uncommitted scoped
diff:

- RED: `go test -count=1 -mod=vendor ./internal/doctor -run
  TestQuickCheckSkipsDeepPriceDiagnostics` failed before the production edit
  because quick mode traversed the malformed historical event and reported
  `usage/schema_incompatible`.
- GREEN: the same focused test passed; `go test -count=1 -mod=vendor
  ./internal/doctor` passed.
- L2: `go test -count=1 -mod=vendor ./...` passed.
- A temporary binary built successfully at
  `/private/tmp/agentdeck-quick-doctor`. Go emitted a non-fatal warning that its
  user module stat cache was not writable; the vendor build still exited 0.
- Real-state quick acceptance remains unverified. The managed approval reviewer
  rejected the read-only command with its own `Unknown parameter:
  input[58].namespace` error. The command was not retried through another path,
  and the installed Homebrew RC2 binary was not changed.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| `quick-price-boundary` | [x] | [x] |

Review Round 1 reopened development because the regression suite does not yet
pin the full-mode `unpriced_models` check and its `unpriced_models` result code.
The quick/full production boundary itself had no review finding.

Review Round 1 fix evidence on 2026-08-02:

- Added only a valid unpriced historical event to the existing full-doctor
  fixture and required result code `unpriced_models`; production code did not
  change.
- No behavioral RED was expected because the reviewed full-mode behavior was
  already correct and lacked direct regression protection.
- Both focused tests passed independently:
  `TestFullCheckReportsProblemsWithoutChangingDatabases` and
  `TestQuickCheckSkipsDeepPriceDiagnostics`.
- `go test -count=1 -mod=vendor ./internal/doctor` and
  `go test -count=1 -mod=vendor ./...` passed.
- An initial combined `-run` command did not enter Go testing because the shell
  interpreted its `|`; it was replaced by the two unambiguous focused commands
  above.

Review Round 2 passed on 2026-08-02. The added full-mode fixture and assertion
directly protect the `unpriced_models` check and its result code, closing the
only Round 1 finding without changing the reviewed production boundary.

## Final State

Delivered and independently reviewed. This historical plan has no starting
task; deeper pricing-read scalability remains in the active `v0.2.2` plan.
