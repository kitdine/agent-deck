---
status: active
created: 2026-09-09
updated: 2026-09-09
---

# Snapshot Performance — Tasks

This file owns the topic's document and implementation readiness. The topic is
selected by the [v0.6.0 contract](../v0-6-0-contract/tasks.md) and originates from
planning carrier `ad-snapshot-performance`.

Workspace: `agent-deck.snapshot-performance`; branch:
`feature/snapshot-performance`; design baseline:
`f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f`.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
| `ux/` | n/a | n/a |

No surface, copy, interaction or control changes are designed. UX remains with
the existing product prototype and the desktop-refresh topic. The user requested
all document drafts together; this does not self-approve their separate reviews
or authorize implementation before the document set passes.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `performance-contract` | [ ] | [ ] |
| 2. `shared-ingestion` | [ ] | [ ] |
| 3. `generation-checkpoints` | [ ] | [ ] |
| 4. `derived-snapshot-cache` | [ ] | [ ] |
| 5. `unchanged-refresh` | [ ] | [ ] |
| 6. `acceptance-and-reconciliation` | [ ] | [ ] |

### 1. `performance-contract`

Establish reproducible correctness and performance fixtures before changing the
runtime. Preserve a fixed corpus and reference outputs privately; repository
fixtures contain synthetic/approved data only.

- Boundaries: focused harnesses under `cmd/agentdeck`, `internal/desktop`, usage
  and session tests, plus the existing test scripts as needed.
- Include initial empty-index import, unchanged refresh and changed-input cases.
  Freeze business time/timezone for differential comparisons. Record hardware,
  toolchain, corpus digest/scale, process startup, CPU, peak RSS and every sample.
- Compare complete serialized output, stream integrity and all relevant logical
  rows, including tools, work signals, file links and approved documents. Internal
  `json:"-"` fields need their own assertions where reused by calculations.
- Distinguish measurement deadlines, parser errors and post-scan persistence
  failures. No partial import is a successful cold benchmark.
- Verification: L2 focused harness tests and affected package tests through
  `scripts/run-go-test.sh`; L0 documentation/fixture checks.

### 2. `shared-ingestion`

Depends on Task 1. Introduce coordinated source preparation and shared parsing
without changing either domain's observable semantics.

- Boundaries: `internal/usage`, `internal/session`, `internal/activity`, a small
  shared input/coordinator package, and `cmd/agentdeck/desktop.go`.
- Preserve domain ordering, duplicate ownership, machine identity, cumulative
  counters, pending turns, source offsets and partial-record continuation.
  Use bounded queues and private approved-record spooling rather than implicit
  cross-consumer ordering dependencies.
- Re-evaluate cold-import alternatives on the fixed corpus. Default to existing
  SQLite dependencies, write durability and recovery semantics. Adopt an
  optimization only with measured improvement and exact output/data equivalence.
- Keep the initial-import target at 10 seconds. The design-stage decision permits
  this task to investigate a better solution; it does not establish success or
  authorize changing the metric. Record unresolved performance gaps explicitly.
- Verification: L3; parser/scan suites, split/append/rewrite/rename/duplicate and
  cancellation cases, full Go regression, relevant race/vet/build checks.

### 3. `generation-checkpoints`

Depends on Task 1; coordinate the source-signature interface with Task 2.

- Boundaries: core/session schema ownership in `internal/store`, source registry
  updates in usage/session, source discovery, and owned backup/restore/rebuild
  entry points.
- Implement database epochs, coalesced dirty/revision invalidation, trigger
  coverage and trustworthy processed-source checkpoints. Bind parser/root
  selection and strong file signatures; old or incomplete markers cannot skip.
- Allocate the actual next migration version at implementation. Preserve
  read-only old-schema fallback and future-schema refusal.
- Verification: L3 migration and rollback tests; every covered mutation type,
  no-op writes, new table coverage, restore/rebuild, file replacement, missing
  session index and source changes during scanning; full Go and relevant races.

### 4. `derived-snapshot-cache`

Depends on Tasks 2 and 3.

- Boundaries: a focused cache codec/publisher, `internal/desktop`, reusable usage
  aggregations, and an optional desktop price-availability input to quick doctor.
- Implement the explicit summary/presentation/work-signal DTO, bounded private
  atomic publication, epoch/revision/timezone/time-boundary key and checksum.
  Avoid whole-event or cross-request per-event price retention.
- Reject missing, corrupt, oversized, obsolete, dirty, mismatched or expired
  entries. Check clock rollback and scheduled price boundaries. Keep live
  provider, schema, credential, lock, refusal and session reads outside the cache.
- Snapshot misses recompute without writes. A failed cache build/publication does
  not undo committed indexes or turn a source failure into success.
- Verification: L3 cache races, interrupted publication, reader/writer overlap,
  directory/file permissions and symlink handling; exact output comparison on
  hit/miss/invalidated paths and core/session independent failure tests.

### 5. `unchanged-refresh`

Depends on Tasks 2–4.

- Boundaries: `cmd/agentdeck/desktop.go`, source checkpoint/manifest helpers, and
  the existing macOS embedded-helper integration tests.
- Use a successful processed checkpoint, current parser/schema/index identity,
  strong current source signatures and a valid derived entry to skip unchanged
  work. Do not rely on post-scan fingerprints of unprocessed bytes.
- Preserve both helper commands, JSON fields and streamed output. Keep existing
  timeout and domain success/error meanings; include probe/build work in actual
  duration accounting rather than moving it outside the measurement.
- Verification: L3 append, same-size/mtime-preserving rewrite, rename, deletion,
  permissions, parser upgrade, index loss, midnight/DST and concurrent scan
  cases. Exercise the actual helper invocation/decode path and its error paths.

### 6. `acceptance-and-reconciliation`

Depends on Tasks 1–5.

- Run the complete differential and resource/performance matrix on the final
  content state. Report first invocation, all samples, median, P95, maximum,
  combined CPU and peak helper RSS. Do not hide cold work or failed samples.
- Verify the ≤1-second unchanged-cycle, ≤0.5-second CPU, ≤100-MiB warm RSS and
  retained ≤10-second initial-import/rebuild targets from requirements. Separate
  achieved values from gaps. A gap needs an explicit disposition before delivery
  can be claimed; it is not resolved by this design's readiness relaxation.
- Reconcile affected `docs/specs/cli-design.md` and `cli-manual.md` contracts,
  tests/fixtures and topic handoff. Record the actual capacity and limits for
  desktop-refresh; do not change refresh scheduling or global project status
  as an intermediate topic-progress update.
- Verification: L3 affected Go and macOS helper/wire acceptance, race/vet/build
  checks selected by actual changes, and L0 topic/link/whitespace checks.
  Release preflight, signing, publishing and installation are separate scopes.

## Current handoff

Requirements and architecture reviews passed: [requirements record](reviews/requirements.md)
and [architecture record](reviews/architecture.md). The decomposition and document
set also passed [tasks review](reviews/tasks.md). Implementation begins with
`performance-contract` only under a real user Development command. The initial-import
performance target remains to be verified during implementation under the user's
2026-09-09 design-stage decision. No production implementation or release is
claimed by the temporary research prototypes. Implementation dispatch starts
only after the document set and decomposition are approved.

Review order: requirements, architecture, then tasks. If an earlier review changes
the contract, reconcile affected later drafts before their review. Topic progress
stays here and in its review records; Beads provides cross-worktree coordination.
