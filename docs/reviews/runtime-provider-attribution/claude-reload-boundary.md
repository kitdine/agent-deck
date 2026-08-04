---
status: active
plan: runtime-provider-attribution
task: claude-reload-boundary
---

# Review log — runtime-provider-attribution / claude-reload-boundary

## Round 1 — 2026-08-04

- Reviewed state: `HEAD` `19f2f557d8035758765d85b75a3ff0954edcf172`; reviewed product/test file SHA-256 values: `cmd/agentdeck/main.go` `f35066975daaa22e1b0a0bf556106774e81fc77b411858f42f940702582605ad`, `cmd/agentdeck/claude_reload_test.go` `b18522050636ba44824e51084c993de7c27d53059c933d1fa7eb79010ababf38`, `internal/provider/config.go` `b0e616b1f5ec5b5dfc7c74ee24428c20ff543e8876d3c4bb38f75fff260d47eb`, `internal/provider/config_test.go` `5ef2ddab60ad93755d5184fc2cb18ba688ad97972a6909ccfb830f2b7078589a`, `internal/usage/routes.go` `69d78e1ffb8508ba376784940d74bbb107da71afe1a039753e6fb2af01c9bc9f`, `internal/usage/routes_test.go` `e5e7b9ad95c763a0f2bce272797a56342a351fb20b6b0dde6908e3f453df7222`, `internal/usagehook/event.go` `91f4e34fdd9b291cd8aedbd873c1e7a6de3d0ef07dba2798e4bc0ddf18c9607e`, and `internal/usagehook/event_test.go` `b281783f0d1497f4d67a41107680031e0cf2f9edd36368b41d0ad50cf586ef5a`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: Claude `ConfigChange` input boundary, managed user-settings-path filtering, completed-provider snapshot reconciliation, custom/official/wrapper matching, estimated matched/unknown route persistence, and regression coverage.

### Findings

**M1 — transient unreadable or incomplete settings file bypasses the advertised reconciliation window and loses the provider boundary.** `cmd/agentdeck/main.go:2735-2738` returns immediately when `provider.ClaudeConfigMatchesSnapshot` cannot read or parse `~/.claude/settings.json`; only a clean mismatch reaches the remaining two 25 ms attempts. A Hook delivered while the settings file is temporarily absent or incomplete therefore writes neither the matched boundary nor the required stable-mismatch `unknown` boundary, although a subsequent retry would observe the completed configuration. This violates task acceptance that a matched user-settings reload changes later attribution and makes the three-attempt reconciliation ineffective for the most timing-sensitive state. Keep hook failure fail-open: retain the last read/parse error and return without a write only if all attempts fail to inspect the file; otherwise retry errors and mismatches, recording matched or `unknown` only after a successful final observation.

**Required regression coverage:** extend `cmd/agentdeck/claude_reload_test.go` so the first inspection sees malformed or missing managed settings, the injected reconciliation sleep replaces it with the matching configuration, and the assertion proves exactly one selected-provider `ConfigChange` route (not `unknown`). Preserve the existing stable-mismatch and ignored-path/source assertions.

### Evidence

- Full-context diff and CodeGraph call-path review of `runUsageHookEvent -> reconcileClaudeConfigChange -> ClaudeConfigMatchesSnapshot -> RecordClaudeConfigChange -> sessionRouteAt`.
- Input parser accepts only known Claude `ConfigChange` sources; command handling further limits writes to `user_settings` at `~/.claude/settings.json`. Configuration contents and event path are not persisted.
- Existing development evidence remains applicable to the reviewed product/test content: targeted provider/usage/hook/CLI vendored tests and `go test -mod=vendor -count=1 ./...` passed; `git diff --check` passed. No duplicate suite was run during this unchanged-state review.

- Verdict: REOPEN

## Round 2 — 2026-08-04

- Reviewed state: `HEAD` `19f2f557d8035758765d85b75a3ff0954edcf172`; changed since Round 1: `cmd/agentdeck/main.go` SHA-256 `3e7da2a2a2149f9ef81ce2b4aaf5c168bd87e131de6f2237cbbfe5edb13e2e01` and `cmd/agentdeck/claude_reload_test.go` SHA-256 `4c2919095b83cf512f56c0a90d3f5c1b43f6c8bc5f3abe12af03619331327f61`. The other six reviewed product/test files retain the Round 1 hashes.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: Round 1 M1 retry semantics, its deterministic regression, and adjacent matched/stable-mismatch route persistence.

**M1 remains partially fixed — a successful mismatch followed by transient read failures still loses the required `unknown` boundary.** `reconcileClaudeConfigChange` now retries an initial read/parse error and the new regression proves `error -> matched`. However, `lastConfigErr` describes only the final attempt: a successful mismatch clears it, but a later read/parse failure sets it again. For `mismatch -> error -> error`, the function returns the final error without recording `unknown`, even though one attempt successfully inspected the managed settings. Round 1 required returning without a write only when all attempts fail to inspect the file. Track whether any inspection succeeded independently from the last error; after attempts are exhausted, record `unknown` when at least one successful observation was a mismatch.

**Required regression coverage:** extend `cmd/agentdeck/claude_reload_test.go` with a deterministic `mismatch -> malformed/missing -> malformed/missing` sequence and assert exactly one `unknown` `ConfigChange` route. Also make the existing `error -> matched` test explicitly assert exactly one selected-provider route, so it cannot pass if an extra boundary is written.

- Verification: source and full-context diff provide a deterministic reproducer for the remaining branch. Per project review discipline, broad tests were stopped once the medium finding was decisive; the recorded review-fix targeted and full-suite PASS evidence was not rerun.
- Verdict: REOPEN

## Round 3 — 2026-08-04

- Reviewed state: `HEAD` `19f2f557d8035758765d85b75a3ff0954edcf172`; changed since Round 2: `cmd/agentdeck/main.go` SHA-256 `6df3084634ef5c4694aecce400265e8caefbb7fd0a8f9e25f43ada00f53601db` and `cmd/agentdeck/claude_reload_test.go` SHA-256 `75c340c5ee753e1132d65e4c01d86380fe7c02c2eb8a2d448fcd22a8cf64a171`. The other six reviewed product/test files retain their Round 2 hashes.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: Round 2 M1 product fix and both required deterministic regression assertions.

**M1 product behavior is fixed, but its required `error -> matched` regression assertion remains incomplete.** `reconcileClaudeConfigChange` now tracks `inspectedConfig` independently from the last read/parse error, so `mismatch -> error -> error` records `unknown` while three failed inspections still return without a write. The new complementary test explicitly proves exactly one `unknown` route. However, `TestClaudeConfigChangeRetriesTransientSettingsRead` still uses a single `QueryRow` and checks only the selected route fields; it does not assert the route count is exactly one as Round 2 required. An implementation that writes the selected route and then an extra boundary can still satisfy this assertion.

**Required closure:** add an explicit count assertion to `TestClaudeConfigChangeRetriesTransientSettingsRead` proving exactly one selected-provider `ConfigChange` route for the session, parallel to the count assertion in the complementary mismatch test.

- Verification: full-context source and focused line review at `cmd/agentdeck/claude_reload_test.go:152-158` decisively establish the missing assertion. Per project review discipline, broad tests stopped once the remaining required coverage finding was confirmed; recorded review-fix targeted and full-suite PASS evidence was not rerun.
- Verdict: REOPEN

## Round 4 — 2026-08-04

- Reviewed state: `HEAD` `19f2f557d8035758765d85b75a3ff0954edcf172`; changed since Round 3: only `cmd/agentdeck/claude_reload_test.go`, SHA-256 `afeb5e92fb787184896682ff65d0433cbbe405f88fc042e8b05a5b45a20ebb42`. Product logic and the other seven reviewed files retain their Round 3 hashes.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: Round 3 exact-one selected-provider route assertion, full M1 closure, and adjacent deterministic test behavior.
- Finding closure: `TestClaudeConfigChangeRetriesTransientSettingsRead` now asserts the route count is exactly one together with provider `custom`, multiplier `2`, and quality `estimated`. The complementary mismatch test continues to assert exactly one `unknown` route. The two tests now reject both missing and duplicate boundary writes across `error -> matched` and `mismatch -> error -> error`.
- New findings: none.
- Verification:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/provider ./internal/usage ./internal/usagehook ./cmd/agentdeck` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` -> PASS.
- Verdict: PASS
