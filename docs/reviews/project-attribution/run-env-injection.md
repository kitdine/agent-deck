---
status: active
plan: project-attribution
task: run-env-injection
---

# Review log — project-attribution / run-env-injection

## Round 1 — 2026-07-28

- Reviewed state: base `5e8f8db`, uncommitted working tree carrying the
  `run-env-injection` implementation, tests, and Dev completion note.
- Reviewer: Codex, following the user-requested review workflow.
- Scope: completed-route resolution, child environment injection, Codex
  `env_http_headers` lifecycle and preservation, and regression coverage.

### Findings

- **[P2] A stale wrapper selection inherited a different endpoint's protocol
  declaration.** `RunProjectEnvironment` combined the completed selection's
  `ViaWrapper` flag with the provider's current `WrapperKind`, but did not
  compare the selection's endpoint with the current wrapper URL. Replacing
  Headroom wrapper A with Headroom wrapper B could therefore send a
  Headroom-specific header to the still-selected endpoint A.
- **[P2] Project mapping rewrites deleted unrelated header mappings.** Both
  enabled and disabled rewrites removed the complete inline field or sub-table
  before optionally writing only `X-Headroom-Project`, so mappings such as
  `X-Unrelated = "OTHER_ENV"` were lost.
- **[P2] Equivalent quoted TOML sub-table syntax failed normalization.** Table
  removal compared the raw header text with one unquoted spelling. Basic-quoted,
  literal-quoted, and whitespace-separated dotted paths survived; adding the
  inline field then failed validation with a table/value conflict.

**Verdict: REOPEN.** The task's Dev cell remains checked and Review remains open
until an independent re-review closes all three findings.

## Round 2 — 2026-07-28 (fix round, recorded by implementer)

This is not a review pass. It records the scoped changes submitted for
independent re-review.

- Completed-route attribution now also requires
  `snapshot.Endpoint == current WrapperURL`. Table-driven coverage proves stale
  custom and built-in routes leave both Codex and Claude environments untouched.
- Codex header rewriting now parses and copies the existing string mapping,
  adds, replaces, or removes only `X-Headroom-Project`, and renders the remaining
  mappings as one deterministic inline field. The cross-writer fixture covers
  enabled and disabled wrapper writes plus custom and built-in transitions while
  retaining `X-Unrelated`. When attribution is disabled and no project key is
  present, the unrelated inline field is left byte-for-byte untouched.
- Sub-table matching now parses each candidate table header and compares its
  semantic key path. Coverage includes basic-quoted, literal-quoted, and
  whitespace-separated dotted spellings, validates the resulting TOML, and
  checks both the canonical project mapping and unrelated mapping.

### RED evidence

- `go test -count=1 -mod=vendor ./internal/provider -run Project` failed before
  production changes.
- Four stale-route cases returned changed environments carrying project
  attribution.
- The cross-writer case retained only `X-Headroom-Project` and lost
  `X-Unrelated`.
- A disabled rewrite reformatted an unrelated-only inline field before the
  no-project-key no-op guard was added.
- All three equivalent sub-table spellings failed with
  `key env_http_headers should be a table, not a value`.

### GREEN and final verification

- `go test -count=1 -mod=vendor ./internal/provider -run Project` — exit 0.
- `go test -count=1 -mod=vendor ./internal/provider` — exit 0.
- `go test -mod=vendor ./...` — exit 0.
- `go vet -mod=vendor ./...` — exit 0, no diagnostics.
- `gofmt -l cmd/agentdeck/main.go internal/provider/config.go
  internal/provider/config_test.go internal/provider/service.go
  internal/provider/project.go internal/provider/project_test.go` — no output.
- `git diff --check` — exit 0.

Next workflow:
`进入复评并生成后续指令:project attribution / run-env-injection`.

## Round 5 — 2026-07-29 (re-review)

- Reviewed state: base `5e8f8db`, uncommitted working tree containing the
  Round 4 fix.
- Reviewer: Codex, using fresh targeted behavioral tests.
- Scope: closure of the remaining Round 3 finding and regression coverage for
  all Round 1 findings.

### Findings

- **Round 1 stale-wrapper route finding: closed.** Completed selections must
  still match the current Headroom wrapper endpoint before either client
  receives attribution.
- **Round 1 unrelated-mapping finding: closed.** Enabled and disabled rewrites
  preserve unrelated mappings across all three Codex writers.
- **Round 1/3 TOML semantic matching findings: closed.** Fully quoted outer and
  sub-table paths, quoted inline keys, dotted-key whitespace, and
  array-of-tables occurrences normalize without duplicate table or field
  definitions.

### Verification

- `go test -count=1 -mod=vendor ./internal/provider -run TestCodexProjectHeadersNormalizeQuotedOuterTablesAndInlineKeys`
  — exit 0.
- `go test -count=1 -mod=vendor ./internal/provider -run TestWriteCodexWrapperConfigResetsOwedFieldsAcrossArrayOfTablesOccurrences`
  — exit 0.
- `go test -count=1 -mod=vendor ./internal/provider -run Project` — exit 0.
- The Round 4 full-suite and vet results are reused because no product content,
  dependencies, toolchain, configuration, or generated files changed after
  those successful commands.

**Verdict: PASS.** All recorded findings are closed.

## Round 3 — 2026-07-29 (re-review)

- Reviewed state: base `5e8f8db`, uncommitted working tree after the Round 2
  fix.
- Reviewer: Codex, using fresh targeted tests and read-only behavioral overlays.
- Scope: closure of all three Round 1 findings and regression value of the new
  TOML syntax fixtures.

### Round 1 findings

- **[P2] Stale wrapper route: closed.** Custom and built-in selections now
  require the completed endpoint to equal the current wrapper URL for both
  clients.
- **[P2] Unrelated header mappings: closed.** Cross-writer enabled and disabled
  transitions preserve unrelated mappings, and a disabled no-project-key write
  leaves an unrelated inline field byte-for-byte untouched.
- **[P2] Equivalent TOML syntax: partially closed.** Parser-aware matching was
  used only while dropping the project-header sub-table. The shared custom-table
  writer still compared the raw table name with `model_providers.custom`, and
  the inline field matcher still accepted only a bare key.

### Remaining finding

- **[P2] Fully quoted outer paths and quoted inline keys still failed.** A
  `["model_providers".'custom']` table paired with a fully quoted project-header
  sub-table failed with `table custom already exists`. Basic-quoted and
  literal-quoted inline `env_http_headers` keys failed with
  `key env_http_headers already defined`.

**Verdict: REOPEN.** Review remains open pending one scoped semantic-matcher fix.

## Round 4 — 2026-07-29 (fix round, recorded by implementer)

This is not a review pass. It records the scoped changes submitted for another
independent re-review.

- `rewriteCodexCustomTable` now tracks whether the current table semantically
  resolves to `model_providers.custom`, rather than retaining its raw spelling.
  The same probe handles regular tables and the final array-of-tables element,
  preserving existing array writer behavior.
- Project-header line removal now parses the line as TOML and tests for the
  semantic `env_http_headers` key, covering bare, basic-quoted, and
  literal-quoted inline forms.
- `TestCodexProjectHeadersNormalizeQuotedOuterTablesAndInlineKeys` covers the
  fully quoted outer/sub-table path and both quoted inline key forms, validates
  the resulting configuration, and checks both the canonical project mapping
  and `X-Unrelated`.

### RED evidence

- The fully quoted path failed with
  `rewrite codex project headers: toml: table custom already exists`.
- Both quoted inline keys failed with
  `toml: key env_http_headers already defined`.

### GREEN and final verification

- `go test -count=1 -mod=vendor ./internal/provider -run Project` — exit 0.
- `go test -count=1 -mod=vendor ./internal/provider` — exit 0, including the
  existing array-of-tables tests.
- `go test -mod=vendor ./...` — exit 0.
- `go vet -mod=vendor ./...` — exit 0, no diagnostics.
- `gofmt -l cmd/agentdeck/main.go internal/provider/config.go
  internal/provider/config_test.go internal/provider/service.go
  internal/provider/project.go internal/provider/project_test.go` — no output.
- `git diff --check` — exit 0.

Next workflow:
`进入复评并生成后续指令:project attribution / run-env-injection`.
