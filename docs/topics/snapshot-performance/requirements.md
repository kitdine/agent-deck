---
status: active
created: 2026-09-09
updated: 2026-09-09
---

# Snapshot Performance — Requirements

## Purpose and origin

Reduce the work needed to refresh AgentDeck's desktop data while preserving the
meaning of every value, attribution decision, availability flag and failure.
This is the `ad-snapshot-performance` planning entry selected by the
[v0.6.0 contract](../v0-6-0-contract/tasks.md). Its origin is the performance
portion of `ad-bug-widget-refresh-stale`; that bug also has refresh and UI work
owned elsewhere. This topic does not close that entire origin bug.

The user requested the complete document design together. Requirements,
architecture and decomposition are therefore drafted in this delivery, with
their separate reviews still required before implementation.

## Confirmed decisions

The user selected these boundaries during 2026-09-08/09:

- Optimize the complete snapshot generation chain, not only usage aggregation.
- The unchanged-refresh target includes source-change detection, both helper
  invocations, snapshot generation and stream decoding. Moving computation into
  `refresh-indexes` does not remove it from the measurement.
- Allow private, bounded, disposable derived caches. Keep `desktop snapshot`
  read-only and keep current output and failure semantics.
- Use isolated local copies of representative data for measurement. Never run
  a benchmark import against the operator's live state or expose transcript text
  or sensitive configuration in reports.
- Retain the aggressive targets below, including the first complete import.
- On 2026-09-09, the user explicitly removed proof of the 10-second target as a
  prerequisite for completing this design. Record the gap and re-evaluate better
  approaches during implementation. The target remains **10 seconds**; no larger
  replacement number or performance waiver was authorized.

## Performance targets and their current status

| Scenario | Target | Measurement boundary |
| --- | --- | --- |
| Unchanged complete refresh | Wall time ≤1 s; combined child-process CPU ≤0.5 s; peak helper RSS ≤100 MiB | Source checks through complete decoded snapshot, including process starts |
| First complete import | Wall time ≤10 s | Existing executable, empty indexes/cache, full representative source import, aggregation and complete decoded snapshot; include initialization needed by the commands |
| Full recomputation after invalidation | Wall time ≤10 s | Required refresh work and reconstruction of a complete snapshot from committed inputs; identify source-import work separately without omitting it from the total |

These are targets to verify, not observed guarantees. Design readiness does not
assert that an implementation meets them. Implementation must report the actual
result and any remaining gap; an over-target observation must not be labeled a
passing measurement. Any later change to these targets requires an explicit
recorded decision, not a silent threshold change in a benchmark.

Record all measured samples, median, P95, maximum, CPU and RSS. Do not discard the
first invocation or use P95 alone to claim that every sample meets a limit. State
whether executable/OS caches were warm and whether external load invalidates a
comparison. Measurements from different clocks or data states are not paired
performance evidence.

## Scope

1. Shared source discovery and trustworthy unchanged-input detection for usage
   and session indexes, including parser changes and index loss/rebuild.
2. Reduction of repeated reads, decoding, attribution/price resolution and
   aggregation within the snapshot path.
3. Bounded cross-process reuse of derived usage presentation, its summary and
   work-signal results, with explicit invalidation and safe publication.
4. A cacheable, successfully validated price-availability result for the desktop
   quick-health calculation. Current provider routes, credentials, locks, schema
   checks, refusal diagnostics and other live health inputs remain live reads.
5. Correct cold-import and incremental-import integration, cancellation,
   independent domain failures, recovery and benchmark coverage.
6. Stable-contract reconciliation and handoff to the later desktop-refresh work.

The following remain outside this topic: changing refresh intervals; Widget
timeline reload policy; stale/aging copy; new menu-bar or Widget controls;
subscription/quota behavior; changing cost/price meaning; changing supported
source shapes or retained content; installing a daemon; release or local app
installation. No new UX document or rendered specimen is required because the
existing surfaces and their interaction states remain unchanged. The
[product prototype](../../../prototype/README.md) retains ownership of them.

## Correctness requirements

- For the same committed inputs, clock, timezone and request, the optimized
  output must equal the reference output. Compare complete JSON, including
  omitted fields, null versus zero, arrays, ordering, decimal strings, warnings,
  partial flags and chunk integrity; do not compare only totals or row counts.
- Preserve event identity, duplicate-source ownership, cumulative-token deltas,
  turn boundaries, pending versus classified work, file-digest identity, route
  boundaries, price fallback and exact arithmetic.
- Preserve session source priority, approved-text filtering, original offsets,
  partial-record continuation, source mutation detection and persisted cursor
  ownership. Shared parsing must receive the same machine identity and parser
  context as the existing paths.
- A valid cache must describe the current committed index state for its declared
  clock/timezone window. Missing, malformed, oversized, obsolete or mismatched
  cache entries are misses, never zero-valued successful reports.
- New events, corrections, deletions, route/price changes, time boundaries,
  timezone changes, schema/parser changes, restore and rebuild must invalidate
  the applicable reuse. A file's size and mtime alone are insufficient to certify
  an unchanged session source.
- `snapshot` must not scan sources, migrate/create state, write cache files or
  counters, alter permissions, acquire a write lock, or use the network.
- Future-schema refusal and independent session availability retain the
  schema-version-signal contract. A cache cannot bypass an unsuccessful source
  database open or conceal a missing session index.
- Refresh errors continue to describe scan/checkpoint outcomes. Snapshot may
  still read the last committed indexes under the existing protocol, but a failed
  refresh must not be relabeled successful because cached data exists.
- Keep durable data and recovery guarantees. Disabling synchronous writes,
  omitting indexed documents, skipping classification, or losing file links is
  not a performance optimization.

## Resource and privacy boundaries

Cache only the allowlisted derived values needed by this snapshot. Do not cache
credentials, endpoint authentication material, raw tool arguments/results,
reasoning, complete transcripts, or live health/provider results. Session text
retention remains the existing approved-document contract.

Use private directories/files, bounded cache reads and one cache publisher per
state root. Keep source-reading and parsing buffers bounded; handle supported
large records without silently introducing a new rejection limit. Record cold
import peak memory separately; the measured warm-cache memory result does not
establish cold-import memory behavior.

## Acceptance and review boundary

Acceptance covers unchanged refresh, initial import, append, rewrite, removal,
rename, partial records, late route/price changes, time and timezone boundaries,
cache corruption, restore/rebuild, missing/future schemas, cancellation and
concurrent readers/writers. [Architecture](architecture.md) defines the cache and
checkpoint contracts; [Tasks](tasks.md) owns the verification assignments.

The research observations are discovery evidence, not acceptance of the future
implementation. The 10-second gap is explicitly carried into implementation;
it does not block review of this document set under the user's latest decision.
Implementation review must distinguish achieved behavior from unfulfilled
performance targets and obtain a disposition before claiming final delivery.
