---
status: active
created: 2026-09-09
updated: 2026-09-10
---

# Snapshot Performance — Requirements

## Purpose and origin

Reduce the work needed to refresh AgentDeck's desktop data while preserving the
meaning of every value, attribution decision, availability flag and failure.
This is the `ad-snapshot-performance` planning entry selected by the
[v0.6.0 contract](../v0-6-0-contract/tasks.md). Its origin is the performance
portion of `ad-bug-widget-refresh-stale`; that bug also has refresh and UI work
owned elsewhere. This topic does not close that entire origin bug.

On 2026-09-10 the user requested a complete topic review and replan, accepted
replacement of all topic implementation code, and rejected parallel old/new
execution plans. This revision replaces the original foreground-only scope.
Historical reviews and commits retain their original meaning; they do not approve
this revised document set. [Tasks](tasks.md) is the only execution/status plan.

## Confirmed decisions

- One scan engine serves CLI and App for each canonical state root. Every scan
  processes both usage and sessions. Scope selects foreground waiting/display.
- An on-demand detached worker continues after its subscriber leaves and exits
  when accepted work is terminal. No installed permanent daemon or launchd job.
- New `agentdeck scan [--scope usage|session]`; no scope waits for both domains.
  Legacy scan commands are adapters. Read-only/no-scan commands never start work.
- A later caller joins only a round whose observation covers its request;
  otherwise a finite follow-up round is queued. Never certify new input from an
  old completed-domain flag. One active scan round per state root.
- App refresh displays actual waiting/checking/processing/statistics stages;
  previous data remains available under existing partial/error rules. Closing
  the popover detaches, without cancelling the global scan or reopening the UI.
- Preserve source semantics, ownership, exact costs, approved session text,
  independent domain outcomes, recovery, schema refusal and read-only snapshot.
- Derived reuse is private, bounded and disposable. Performance includes actual
  client/helper/worker startup, required work and final decoded snapshot. A quick
  foreground return does not establish a faster completed global cycle.
- Replacement of the old coordinator is allowed. Fixing or delivering the old
  shared-ingestion candidate is not an entry prerequisite for the replacement.
  Existing defects remain acceptance cases, not implicitly repaired findings.
- Measure only isolated synthetic or private copied data. Existing live state,
  credentials, unrelated code, Git history and other topics are not reset.
- Retain the performance targets below. The prior design-readiness relaxation
  did not waive them; this replan does not promise they have been achieved.

## Performance targets and their current status

| Scenario | Target | Measurement boundary |
| --- | --- | --- |
| Unchanged complete refresh | Wall time ≤1 s; combined client/helper/worker CPU ≤0.5 s; peak executing helper/worker RSS ≤100 MiB | Source checks through complete decoded snapshot, including process starts and worker completion |
| First complete import | Wall time ≤10 s | Existing executable, empty indexes/cache, full representative source import, aggregation and complete decoded snapshot; include initialization needed by the commands |
| Full recomputation after invalidation | Wall time ≤10 s | Required refresh work and reconstruction of a complete snapshot from committed inputs; identify source-import work separately without omitting it from the total |

These are targets to verify, not observed guarantees. Design readiness does not
assert that an implementation meets them. Implementation must report the actual
result and any remaining gap; an over-target observation must not be labeled a
passing measurement. Any later change to these targets requires an explicit
recorded decision, not a silent threshold change in a benchmark.

Record all measured samples, median, P95, maximum, CPU and RSS. Include worker
CPU after client exit; report aggregate round cost once, every process peak RSS,
and simultaneous aggregate RSS separately. Do not substitute foreground latency
for either complete wall time or whole-round CPU. Do not discard the
first invocation or use P95 alone to claim that every sample meets a limit. State
whether executable/OS caches were warm and whether external load invalidates a
comparison. Measurements from different clocks or data states are not paired
performance evidence.

## Scope

1. Unified explicit/implicit scan and maintenance coordination, worker process,
   election, finite rounds, subscriptions, durable outcomes and cancellation.
2. Plan-first shared ingestion, bounded memory, deterministic domain publication,
   real SQL batching and cold/incremental/unchanged correctness.
3. Domain checkpoints, generation invalidation, derived aggregation/cache,
   read-only snapshot and cross-process reuse.
4. CLI/App progress, scoped waiting, helper streaming and existing partial/error
   presentation. CLI remains English; App follows existing English/Chinese copy.
5. Complete-cycle correctness/resource/performance acceptance, obsolete scan-path
   removal and reconciliation of stable contracts.

Out of scope: refresh interval changes; Widget timeline/reload policy; unrelated
stale/aging copy and controls; subscription/quota; price meaning; new retained
source shapes; service installation; release or App installation. Code rewrite
permission is not permission to delete user databases or discard unrelated work.

[CLI scan UX](ux/cli-scan.md) and [menu-bar scan UX](ux/menubar-scan.md) own the
changed presentation contracts. The shared [product prototype](../../../prototype/README.md)
remains the specimen authority. UX is applicable; missing specimen validation
must not be hidden behind n/a or a backend test result.

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
