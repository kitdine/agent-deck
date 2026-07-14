# Local AI Provider Session Cost Tracking Implementation Plan

**Status:** historical, superseded by
`docs/plans/2026-07-13-agentdeck-cli.md`

The legacy implementation and review remediation remain available in
historical commit `3fcc121` but have been removed from the current tree after
the AgentDeck replacement passed independent review. This plan remains a
historical contract and is not the current execution tracker.

**Specification:**
`docs/specs/2026-07-13-ai-provider-session-cost-design.md`

**Goal:** Add private, incremental, session-aware local cost reporting for
Codex and Claude without changing or uploading their session content.

## Scope Boundaries

- Use Python standard library, Bash, JSON, and SQLite only.
- Do not add custom model pricing, provider usage APIs, a daemon, or a GUI.
- Do not modify Codex or Claude session files.
- Do not store prompt, response, tool, attachment, or credential content.
- Keep exact wrapper attribution and estimated file-only attribution visibly
  distinct.
- Do not commit, push, install into the user's home directory, or refresh the
  official price snapshot without the authorization required for that action.

## Task 1: Lock Calculation and Attribution Contracts

- [ ] Add table-driven tests for OpenAI uncached input, cached input, output,
      and provider multiplier calculation.
- [ ] Add table-driven tests for Anthropic input, output, 5-minute cache write,
      1-hour cache write, cache read, and provider multiplier calculation.
- [ ] Cover unknown models, missing cache TTL detail, invalid multipliers,
      decimal rounding, and historical multiplier `1`.
- [ ] Cover one logical session with two exact runs using different providers
      and one estimated session that remains assigned by its first timestamp.

## Task 2: Add the Versioned Official Price Catalog

- [ ] Define the normalized catalog schema under `config/` with source URLs,
      retrieval time, effective date, currency, aliases, and decimal prices per one
      million tokens.
- [ ] Implement an explicit updater that downloads only the approved official
      OpenAI and Anthropic sources, validates required models and components, and
      atomically writes the normalized snapshot.
- [ ] Add a reviewed initial snapshot for model IDs present in supported Codex
      and Claude logs.
- [ ] Keep tests offline with synthetic catalog fixtures.

## Task 3: Build the Private SQLite Store

- [ ] Add schema creation and versioned migrations for source files, provider
      selections, logical sessions, runs, usage events, and model prices.
- [ ] Create the database and sidecars with owner-only permissions.
- [ ] Implement transaction-safe upserts, source-row replacement, and targeted
      recalculation using parameterized statements.
- [ ] Add tests for a new database, repeated migration, interrupted transaction,
      and invalid schema version.

## Task 4: Implement Incremental Codex Import

- [ ] Discover active and archived Codex rollout JSONL files.
- [ ] Normalize `last_token_usage`, cached input, output, model, turn identity,
      logical session ID, and timestamp without retaining content.
- [ ] Replace repeated snapshots for a turn instead of summing them.
- [ ] Track file identity and byte cursors; rebuild rows for truncated or
      replaced sources.
- [ ] Test appended lines, incomplete final lines, archive moves, malformed
      records, cumulative totals, and duplicate snapshots.

## Task 5: Implement Incremental Claude Import

- [ ] Discover main and subagent Claude project JSONL files.
- [ ] Normalize assistant usage by message ID and ignore synthetic zero-usage
      placeholders.
- [ ] Preserve 5-minute and 1-hour cache-write tokens separately and retain the
      aggregate only for diagnostics.
- [ ] Replace repeated message snapshots instead of summing them.
- [ ] Test appended lines, missing TTL breakdown, malformed records, duplicate
      snapshots, subagent files, and unknown models.

## Task 6: Record Provider Selections and Multipliers

- [ ] Add optional `cost_multiplier` validation with default `1` to shared
      provider configuration handling.
- [ ] Record timestamp, client, provider, and multiplier after each successful
      mode selection, including a successful no-op selection.
- [ ] Keep existing switch behavior and credential redaction unchanged.
- [ ] Test that running sessions are not reassigned by a switch and that no key
      value reaches SQLite or command output.

## Task 7: Add Exact Run Wrappers

- [ ] Add `ai-provider-run codex` and `ai-provider-run claude` wrappers that
      resolve the active provider, snapshot the multiplier, capture file offsets,
      own the launched process lifetime, and close the run on exit.
- [ ] Bind a resumed logical session to a new run without rewriting its earlier
      run attribution.
- [ ] Detect ambiguous overlapping client lifetimes and downgrade to
      `estimated` instead of claiming exact attribution.
- [ ] Test new sessions, explicit resume, child failure, interruption, and
      overlapping instances with fake client commands and temporary homes.

## Task 8: Implement Fallback Attribution

- [ ] Assign unbound logical sessions to the latest provider selection at or
      before their first timestamp.
- [ ] Mark every file-only assignment `estimated` and expose the accuracy
      warning in text and JSON output.
- [ ] Assign sessions with no eligible selection to `historical` with multiplier
      `1`, never to the provider active during scanning.
- [ ] Recalculate only affected sessions when selection history is imported or
      corrected.

## Task 9: Add Usage Commands and Reports

- [ ] Add `scan`, `summary`, `sessions`, `diagnose`, and `rebuild` commands with
      date, client, provider, JSON, and no-scan options defined by the specification.
- [ ] Report raw component tokens, official base cost, multiplier, final cost,
      unpriced tokens, session/run counts, and attribution quality.
- [ ] Keep money as decimal strings in JSON and produce deterministic output for
      tests.
- [ ] Ensure failures identify the source path and reason without printing the
      source JSON line.

## Task 10: Integrate Documentation and Verification

- [ ] Update the provider example, command documentation, documentation index,
      and project-required verification commands.
- [ ] Run the targeted cost/parser/database tests with synthetic homes.
- [ ] Run the complete project test suite and Python compilation checks.
- [ ] Scan tracked files and test output for synthetic or real credential leaks.
- [ ] Verify an isolated end-to-end flow: switch provider, start through the
      wrapper, append synthetic client usage, rescan, and confirm exact cost output.
- [ ] Remove generated caches before delivery and record any unrun verification
      or remaining attribution limitations.

## Completion Gate

Development is complete only when the specification's ten acceptance criteria
are covered by fresh verification. Implementation completion and independent
review remain separate workflow stages. Commit, push, installation, and runtime
home-directory changes require their own authorization.

## Review Remediation (2026-07-13)

- Exact attribution now uses wrapper-bound stable event keys and persisted file
  ranges; ordinary scans never inherit an old exact run by session ID.
- Run start, binding, and end use short transactions with a cross-process active
  run constraint and stale-wrapper recovery.
- Rebuild rescans transactionally and restores exact bindings after import.
- Historical price versions are retained and selected by event timestamp.
- Source prefix hashes detect same-inode rewrites before the byte cursor.
- Summary, sessions, and diagnose expose attribution quality, multiplier/run
  details, unpriced tokens, and the estimated-attribution warning.
- Regression coverage includes interrupted wrappers, active-run contention,
  stale-run recovery, database and sidecar permissions, invalid schema versions,
  same-inode rewrites, historical price lookup, and Claude TTL-only usage.
- The obsolete `tests/test-codex-provider-mode.sh` suite was migrated to
  `tests/test-ai-provider-mode.sh` during legacy development. That replacement
  coverage was retained until the AgentDeck contracts passed independent review,
  then removed with the rest of the superseded legacy fixtures.
