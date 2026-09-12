---
status: active
created: 2026-09-09
updated: 2026-09-11
---

# Snapshot Performance — Tasks

This is the only current execution/status plan for the topic. The 2026-09-10
user request replaces the original six-task continuation and the supplemental
W0-W7 sequence. No W0 task, prerequisite research task or parallel work-package
matrix remains active. Topic membership remains the existing v0.6.0 selection.

Workspace: `agent-deck.snapshot-performance`; branch `feature/snapshot-performance`.
Historical delivered baseline: `446a58f1f6716f257680879e5dbf3b61365c8cb2`.
The user permits replacement of topic code, not deletion of user data, unrelated
work, branches or history. Current document approvals are recorded below;
implementation and Git delivery remain separate boundaries.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| ux/cli-scan.md | [x] | [x] |
| ux/menubar-scan.md | [x] | [x] |
| tasks.md | [x] | [x] |
| worker-scan-development-design.md | [x] | [x] |

The last row reviews only the supplement's historical/superseded designation;
its original content is retained for provenance, not as another design authority.
Both UX documents now include verified shared-prototype states, screenshots,
interaction checks and specimen identity. All six documents passed the current
document-set re-review, including the shared specimen. No current
Review cell is inherited from historical PASS. Native implementation acceptance
remains with the three tasks below.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `unified-scan-runtime` | [x] | [x] |
| 2. `snapshot-computation-reuse` | [ ] | [ ] |
| 3. `scan-experience-acceptance` | [ ] | [ ] |

### 1. `unified-scan-runtime`

Deliver a working global scan: CLI and App clients share one detached worker per
state root; both usage and session execute; scope only controls waiting; fresh
requests receive covered or finite follow-up rounds; all work reaches terminal
outcomes and the worker exits. This task includes the new engine, not merely a
worker shell around the failed old scheduler.

- Scope: `cmd/agentdeck` scan/desktop/implicit scan/maintenance adapters;
  `internal/ingest`, usage/session ingestion; state lock and source-registry
  owners; new narrowly scoped worker/client transport internals.
- Replace the current coordinator and duplicated orchestration. Retain domain
  parsing semantics and public compatibility adapters. Source planning precedes
  body admission; domain reduction is separate from deterministic publication.
  Transactions persist rows/cursors/context/source completion atomically.
- Implement election, private IPC, accepted receipts, finite observations,
  domain-state snapshots, idempotent reconnect, detached lifetime and shutdown.
  Maintenance participates in the same exclusivity contract. Never migrate or
  write state merely because a read-only snapshot or no-scan query is invoked.
- Within this task, map all callers/locks and record baseline stage costs before
  replacement; use the existing harness and corpus. This is an implementation
  activity, not a separate task or a demand to repeat prior measurements.
- Include semantic-preserving prepared/batch SQL and cold eligibility decisions
  in the engine. Retain optimizations only after differential proof and measured
  benefit; fixed bounded concurrency is sufficient initially.
- Completion: all owned callers use the same runtime; old live scheduler/bypasses
  removed; subprocess election/attach/detach/crash/version/maintenance cases pass;
  domain data, ownership, partial lines, priority, FTS and exact costs match the
  independent fixtures and verified reference. Both old skip-budget and in-flight
  cancellation reproducers pass against the replacement, including all waiters.
- Verification: L3 focused domain and subprocess tests; deterministic barriers for
  ordering/cancellation, affected race, full Go, vet and relevant cross-build.
  KEEP semantic goldens; UPDATE old scheduling tests to assert lifecycle behavior;
  ADD accepted-request/source termination coverage; DELETE obsolete scheduler-only
  assertions after their behavioral protection is retained.
- This task does not claim the final 1-second refresh target or native UI delivery.

### 2. `snapshot-computation-reuse`

Depends on unified-scan-runtime. Deliver complete cold/incremental/unchanged
snapshot computation over the new runtime, with safe source/index checkpoints,
generation invalidation and bounded derived reuse.

- Scope: `internal/store`, usage/session registry and generation owners,
  `internal/desktop`, aggregation/cache publisher/readers, owned restore/rebuild
  invalidation and the complete-cycle measurement harness.
- Finish epoch/revision/dirty coverage for every relevant writer. Distinguish
  source processing proof from derived-cache validity. Refuse wrong/old/future
  states appropriately; no cache can conceal missing sessions or failed refresh.
- Implement the architecture's [payload and internal DTO](architecture.md#91-payload-and-internal-dto),
  [header/time validity](architecture.md#92-header-identity-and-validity-window),
  [private atomic publication](architecture.md#93-private-bounded-and-atomic-publication)
  and [read validation/worker integration](architecture.md#94-read-validation-and-worker-integration).
  These sections explicitly own Summary, the 8 MiB bound and read-only fallback. Unify request-local
  calculations and eliminate duplicate reads rather than adding another cache
  for each existing caller. No raw transcript or live health/provider caching.
- Inside this task, measure the actual dominant cold/recompute/unchanged stages;
  select batch/reduction/cache changes from complete-cycle comparisons. Preserve
  failed samples and the original targets; no separate optimization task queue.
- Completion: append/rewrite/rename/delete/partial inputs, schema/parser changes,
  restore/index loss, clock/timezone/price changes and corrupt cache produce exact
  results; no snapshot writes; worker startup/background CPU/RSS included.
  Publish a complete performance report with every sample and remaining gaps.
- Verification: L3 migration/rollback/invalidation/read-only/differential/cache
  publication races plus actual worker harness. KEEP cost/ownership goldens and
  measurement-boundary regressions; UPDATE accounting for detached workers;
  MERGE redundant aggregation tests only if independent expected results remain.
- An over-target report is not a passing measurement. The task records it honestly;
  topic delivery remains blocked until task 3's acceptance resolves the targets.

### 3. `scan-experience-acceptance`

Depends on tasks 1 and 2 and completed UX/specimen review. Deliver the actual CLI
and App experience over the same runtime, then complete topic acceptance on the
final integrated content. Acceptance belongs to this task, not a fourth plan.

- Scope: CLI output/legacy result adapters, Swift helper event consumer, refresh
  coordinator and menu-bar view/model/localization; shared prototype and tests;
  stable CLI design/manual reconciliation and final performance harness.
- Show actual stages while the helper runs; atomic snapshot replacement; previous
  data on refresh failures; scope-specific foreground completion; no trailing
  background output. Closing the popover detaches without cancelling global work.
- Validate English CLI and English/Chinese App states, 420/280 layouts, keyboard/
  accessibility, first-use/previous-data/partial/failure/reconnect and malformed or
  slow streams. Use the shared prototype; no separate topic prototype copy.
- Completion: final shipped topology passes all architecture scenarios V01-V19,
  complete JSON/logical data comparisons and original cold/recompute/unchanged
  wall/CPU/RSS targets, or has an explicit user-approved disposition of a named
  remaining target. Document/prototype PASS is not native acceptance.
- Run a predeclared campaign of at least 20 complete samples per required scenario,
  including the first, report all failures, median/P95/max and environment. Heavy
  runs are sequential, isolated and never against live state. Unsupported/failed
  samples do not disappear from the denominator or become successful refreshes.
- Verification: final affected L3 regression/race/vet/build plus actual Swift
  helper/native checks from repository scripts; L0 docs/links. Reuse unchanged
  evidence; refresh only integration-affected checks. Release/sign/install remains
  separately authorized and is not claimed by this task.
- Remove temporary experiment switches and test-only legacy live bypasses;
  reconcile contracts and handoff to desktop-refresh without changing its policy.

## Historical task disposition

| Previous task | Disposition in this replan | Current owner of remaining work |
| --- | --- | --- |
| performance-contract | Delivered in 446a58f; keep historical PASS and reusable fixtures | Worker accounting updates belong to tasks 1–3 |
| shared-ingestion | Candidate not accepted for delivery; old repair route withdrawn | unified-scan-runtime owns replacement and both defect regression cases |
| generation-checkpoints | Superseded, not completed | Runtime source checkpoint in task 1; generation invalidation in task 2 |
| derived-snapshot-cache | Superseded, not completed | snapshot-computation-reuse |
| unchanged-refresh | Superseded, not completed | snapshot-computation-reuse, with App presentation in task 3 |
| acceptance-and-reconciliation | Superseded, not completed | scan-experience-acceptance |
| W0-W7 | Historical supplement coverage, never a second active task list | Incorporated in the three task scopes above |

Full historical findings remain in the shared-ingestion review record. A failed
candidate is not relabelled PASS, and no old WorkUnit evidence is relabelled as
new-engine acceptance. Paused old Beads tasks retain history; create the three
new implementation dispatch records only after this Tasks matrix is approved.
Until then the next work is completion/review of this revised document set.
Do not claim, repair, commit or push the superseded implementation by following
historical next-instruction text.

## Current handoff

The user's full replan replaces the prior code-first repair/delivery sequence.
The current six-document set, including the shared UX specimens, passed
[document-set re-review](reviews/tasks.md#round-4--2026-09-11).
The document Review cells reflect the corresponding current records. Completion
evidence and delivery checkpoints are recorded there separately from the verdict.
Native implementation and performance acceptance remain open.
Task 1 full delivery-scope re-review passed with a VERIFIED completion gate; see
[runtime review](reviews/unified-scan-runtime.md#round-4--2026-09-12).
It awaits separately authorized Git delivery; Task 2 has not started.
Then execute the three tasks above in order; a task contains its investigation,
implementation and verification and needs no further task decomposition.
No old-code repair is a prerequisite. Git delivery requires a new, scoped
checkpoint for the chosen content and separate authorization; the old document
commit request must not be expanded into delivery of rewritten code.

## Historical implementation and research evidence

Requirements and architecture reviews passed: [requirements record](reviews/requirements.md)
and [architecture record](reviews/architecture.md). The decomposition and document
set also passed [tasks review](reviews/tasks.md). Task 1 `performance-contract`
re-review passed and was delivered in signed commit `446a58f`:
[performance-contract record](reviews/performance-contract.md). Task 2
`shared-ingestion` completed the bounded `SI-R1-F1` repair from
[Round 1 review](reviews/shared-ingestion.md) on 2026-09-10. Round 2 closed that
finding but returned the task to repair for a new cancellation-lifecycle finding;
the review record owns its evidence and repair scope. The usage inventory now releases its coordinator participation for
unchanged sources before continuing, so a budget-filling unchanged source cannot
block a later changed source. Focused legacy/shared equivalence, the new admission
regression, cancellation preservation, race, full Go, vet, cross-build and size
checks pass. Its Review cell remains unchecked until re-review. Its fixed synthetic corpus covers
cold, unchanged and changed-input cycles; the producer reference compares the
complete stream, internal fields omitted from JSON, and relevant usage/session
logical tables.

The final representative baseline used 1,706 private JSONL files / 2,199,173,587
bytes (corpus SHA-256 `4e8bb4d539c0ae16c2b40511ee4200c00b4b618429ad2ac7a13fd23541e10ddf`),
Go 1.27.1 on darwin/amd64 with 12 logical CPUs, UTC and a fixed business clock.
The repaired end-to-end harness launches separate refresh and snapshot CLI
helpers and includes both command initializations and stream decode in the target
metric. One complete cold sample took 86.539 s wall / 147.332 s CPU / 347.2 MiB
peak RSS; one complete unchanged-refresh sample took 7.691 s wall / 8.222 s CPU /
175.9 MiB peak RSS. Logical-row verification ran outside those target metrics and
took 2.496 s / 2.899 s respectively. Both samples produced the same snapshot and
logical-row digests, but both remain over target; OS cache state and external load
were uncontrolled.
The private full report includes every sample and has SHA-256
`598057c029391a5b408019bde01c7577ce8d1854bc135d439260591f58d69343`.
No production optimization, release or performance waiver is claimed by Task 1.

Task 2 introduces `internal/ingest` as the shared source/event boundary. Source
roots are discovered concurrently. A source-ordered scheduler feeds a reader
pool sized from available CPUs and source count, with a 32-worker hard cap and
64 MiB admission budget. Each stable file generation is read once, JSON-decoded
once and emitted in ordered batches of at most 256 records / approximately 1
MiB. Each record carries type, byte offsets and source sequence; the same
immutable batch is sent through separate usage and session channels. Unbuffered
batch handoff provides backpressure without retaining whole-file raw content.

Usage retains path order, machine identity, cumulative state and per-source
transactions; sessions retain priority/path order, duplicate ownership,
cursor/partial state and per-source transactions. Unchanged consumers explicitly
skip a source; if one consumer exits, the other continues without waiting for
its acknowledgements. Device/inode, size, mtime and ctime validate the generation
before either domain commits. Unsupported identity environments use the legacy
domain path. No raw content is spooled or persisted by the coordinator.

Legacy/shared differential tests cover initial import, split partial append,
completed append, same-size rewrite, rename, duplicate ownership and
cancellation, including source registries and public logical rows. On the same
representative corpus, one complete Task 2 sample measured 89.046 s cold wall /
132.914 s CPU / 165.3 MiB peak RSS and 6.774 s unchanged wall / 7.568 s CPU /
175.6 MiB peak RSS. This is a single uncontrolled sample: compared with Task 1,
cold wall was about 2.5 s slower while cold CPU fell 14.4 s and peak RSS fell
about 181.9 MiB; unchanged wall improved by about 0.9 s. Snapshot digest stayed
`92ad3cf74ff3a9338a546d9a96172cf573260b4b4b946f225f13162f5059f975`;
the expanded eight-table logical digest is
`604d62baa2d5f32297d1da87decda9cc401325f6bab464d97fa0c24716252863`.
The private Task 2 report SHA-256 is
`a7b32e48be48b957237304454e0619422da550128602b7169f5742568774d232`.
The 10-second target remains unmet; per the user's 2026-09-09 direction, further
strategy discussion is deferred until this first implementation is reviewable.

Review order: requirements, architecture, then tasks. If an earlier review changes
the contract, reconcile affected later drafts before their review. Topic progress
stays here and in its review records; Beads provides cross-worktree coordination.

### SQL hotspot optimization — 2026-09-09

The user authorized the first profiling-driven batch: avoid new-source FTS
deletion and reduce usage event/tool SQL overhead. Session scans now check for
orphan projections once when unregistered source paths exist. Confirmed new
sources skip FTS deletion; rewrites and orphan recovery still remove old rows.
Unchanged inventories do not execute the orphan FTS query. Usage reuses seven
SQL statements within each source transaction, skips unchanged event UPDATEs
while preserving source-offset updates and ownership rules, and avoids empty
tool-file deletion for a newly inserted tool. Query-before-upsert is retained
where it supplies ownership and logical-change counts.

The corpus identity now includes archived sessions: 1,720 files / 2,202,066,977
bytes; SHA-256 `8b8750895129b955fba0263dc5e5080308cf587f28e073e60dfc302d9d6684b2`.
An initial SQL-optimized sample completed in 47.678 s cold wall / 71.254 s CPU /
163.8 MiB peak RSS, with usage/session scan spans of 38.204 / 37.001 s. Snapshot
and eight-table digests matched the preceding event-pipeline sample. Unchanged
wall was 8.924 s, slower than the earlier 6.774 s sample; no unchanged speedup
is claimed. These are uncontrolled single samples, not an isolated attribution
of the gain to either SQL change.

A subsequent final-content representative run timed out after 10 minutes during
the unchanged setup import. Its stack still showed a runnable JSON decoder;
the evidence does not establish a coordinator deadlock. The setup uses an
unbounded context and the existing harness did not persist its report before
the process-wide timeout. This failed run is not performance acceptance and
the earlier exact-state CEv1 VERIFIED result does not cover this new candidate.
The first completed report is retained privately at
`/private/tmp/agentdeck-snapshot-sql-optimized-report.json`; failed-run log:
`/private/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.mwLeSt`.
The new-source/rewrite/orphan and prepared-upsert ownership/no-op tests pass;
full Go regression passed before the final new-path-only orphan-query guard,
whose focused regression also passed. Final performance acceptance remains open.
Vet, darwin arm64/amd64 builds, arm64 size and L0 checks passed. A broader race
selection hit the existing watch test's migration deadline and then exhausted
the CLI package's three-minute run budget during a text-layout test; it is not
recorded as a passing suite.

### Continued hotspot optimization — 2026-09-10

Turn classification now materializes tool shapes once per source and joins
logical client/session/turn keys. Cross-source tool ownership remains valid;
pending and tool-less turns match the retained per-turn reference. The query
plan shows one tool-table scan, followed by indexed turn lookup. A synthetic
100-turn benchmark measured 32.65 ms for the old path and 9.58 ms for the new
path (three iterations each); this is not an end-to-end speedup claim.

A fresh cold CPU profile after this SQL change measured 39.64 s duration,
60.81 CPU seconds, including reader 16.38 s (file reads 9.44 s, JSON 6.41 s),
usage scan 10.87 s (classification 2.63 s), and session scan 2.79 s. Inclusive
CPU times overlap; they are not additive wall spans. This identified small
source-read syscalls as a remaining hotspot. Reader buffers now scale with file
size from 4 KiB to at most 1 MiB instead of always using 64 KiB. A counting-reader
test confirms a roughly 600 KiB source takes at most three reads including EOF,
with identical records delivered to both consumers.

The benchmark reuses the last successful cold state for unchanged samples,
eliminating the unbounded preparation import. Every measured sample is written
to the private report immediately; if no cold import completes, unchanged
samples are recorded as setup failures. No timeout limit was increased.

Final representative samples, all complete and with unchanged snapshot/eight-table
digests, are recorded in `/private/tmp/agentdeck-snapshot-reader-20260910.json`:

| Scenario | Wall samples | CPU samples | Peak RSS samples |
| --- | --- | --- | --- |
| Cold import | 39.559 s, 40.700 s | 63.624 s, 65.164 s | 169967616 B, 183119872 B |
| Unchanged refresh | 6.429 s, 6.918 s | 8.395 s, 8.759 s | 185274368 B, 185823232 B |

Immediately preceding reader-buffer changes, the batched-classification build
measured cold 48.128 / 56.670 s and unchanged 9.315 / 9.303 s. These uncontrolled
samples support the selected direction but do not isolate environmental effects.
The unchanged original performance targets remain unmet and are not waived.

Final full vendored Go suite, focused race coverage of ingestion/classification/
ownership/reporting, vet, darwin arm64/amd64 builds and arm64 size check passed.
This new complete run resolves the previous run's missing final-content
verification; the historical timeouts remain recorded above. The report digest
is `07f9c0d67d9d40a64b57481483a80f80a39cd783b924acc888219761fdfc0215`.
Task 2 remains pending independent review and separately authorized Git delivery.

### Snapshot stages and reader admission — 2026-09-10

The user authorized measurement of the remaining snapshot and admission
bottlenecks followed by bounded optimization. Snapshot stage measurements on
the isolated populated state found work signals at 4.266 / 3.636 s and usage
presentation at 3.438 / 2.906 s; session projection took 63 / 21 ms and health
7 / 4 ms. These wall spans identify the two dominant snapshot components.

`usage.SignalsBatch` now shares event/turn reads and price resolution across
the nine desktop period/client scopes, pricing each covered event once and
folding it into each applicable scope. Workflow/tooling readers retain their
existing semantics. Requests without an unfiltered option covering the union
fall back to individual queries so unrelated events cannot introduce errors.
All temporary data is request-local; no persisted cache, credentials or health
results are introduced. Presentation also parses each event's decimal costs
once and reuses immutable rational operands across its aggregation buckets.

An individual/batch/batch/individual paired run on the same database measured
6.613 / 2.862 / 2.954 / 5.599 s, with full report equality. The batch path's mean
was about 52% lower in that diagnostic. Final two-helper samples remained
complete with identical full snapshot/eight-table digests:

| Scenario | Wall samples | CPU samples | Peak RSS samples |
| --- | --- | --- | --- |
| Cold import | 46.444 s, 46.357 s | 73.626 s, 73.967 s | 183242752 B, 182489088 B |
| Unchanged refresh | 7.317 s, 6.091 s | 9.205 s, 7.913 s | 168579072 B, 181821440 B |

These cross-process samples do not establish a stable end-to-end improvement;
load and OS caches remain uncontrolled. Original target gaps remain open.

Opt-in ingestion metrics measured 35.540 s wall, 12 peak and 2.01 average open
readers, 33.010 s scheduler budget wait, 17.154 s summed decode time, 1.637 s
summed read time, and 50.720 / 0.323 s summed usage/session sends. Exactly
2,202,066,977 source bytes were read in 5,204 underlying reads. Worker durations
overlap and send order affects waiting attribution; they are not additive wall
spans or a proof that one consumer alone is slow. The admission policy is not
changed here: replacing whole-file admission with batch-byte credits requires
releasing credits only after both consumers finish processing, plus progress
tests across different source orders. Merely increasing reader limits would
trade waiting for unbounded live batches.

Diagnostic artifacts: `/private/tmp/agentdeck-signals-paired.json`,
`/private/tmp/agentdeck-ingestion-stage-current.json`, and final full-cycle
`/private/tmp/agentdeck-snapshot-reuse-final.json`. Opt-in profiling tests and
metrics remain available without changing normal command output.

Final full Go regression, focused race, vet, darwin builds, arm64 size and L0
checks passed. The complete-cycle report SHA-256 is
`da418fb70c398b82258ae10c6606a8356fa61047b0c2ca0ea02d2c63a566cf42`;
paired signal report SHA-256 is
`e806e23effe1f84ac964313509714bda345f8dc86cc482692478e925daeaf74c`.
Independent review and Git delivery remain pending.

### Batch-credit admission experiment rejected — 2026-09-10

The authorized follow-up replaced whole-file admission with per-worker batch
lanes, acknowledgements after both domain processors finish, cancellation
closure for unscheduled sources, and a legacy fallback for mismatched consumer
orders. Focused race tests covered duplicate acknowledgements, retaining credits
until both acknowledgements, consumer exit, normal batch bounds, concurrent file
admission, and cancellation. Large individual records were accounted separately
as oversized exceptions rather than claimed below the ordinary 64 MiB budget.

The measured change increased average open readers from 2.01 to 11.67 (peak 12)
and removed scheduler admission wait, but ingestion wall increased from 35.540
to 47.275 s. Summed usage-send waiting became 489.878 s across workers; this is
overlapping wait, not elapsed time. Final complete-cycle cold samples were
52.617 / 47.065 s, compared with the preceding 46.444 / 46.357 s; unchanged
samples were 6.085 / 6.050 s. Snapshot and eight-table digests remained identical.
These uncontrolled samples do not justify adopting the added scheduling
complexity: more open readers mostly waited for source-ordered consumers.

The candidate was rejected and all four modified production files were restored
to their exact pre-experiment blobs. The two experiment-only source/test files
were removed from the repository. Candidate sources remain under
`/private/tmp/agentdeck-rejected-batch-credits/`; diagnostic reports are
`/private/tmp/agentdeck-ingestion-batch-credits.json` and
`/private/tmp/agentdeck-snapshot-batch-credits-final.json`. Existing verified
product tests/builds apply to the restored code; only this historical record
changed. No new performance improvement or delivery is claimed.

The next experiment should separate source-local domain reduction from ordered
database publication. Both domains currently finish a source's transaction
before advancing to the next source, so more prefetch lanes alone cannot unlock
parallel reduction. Any such change must retain cursor/row atomicity and
duplicate ownership and use bounded, approved derived records between reducers
and writers. It is not implemented by this rejected admission experiment.
