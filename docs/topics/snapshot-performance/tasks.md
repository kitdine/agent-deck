---
status: active
created: 2026-09-09
updated: 2026-09-13
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
| 2. `snapshot-computation-reuse` | [x] | [x] |
| 3. `scan-experience-acceptance` | [x] | [x] |

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
- Current-round completion follows the user's explicit
  [current-state disposition](#task-3-current-state-completion-decision--2026-09-13).
  The named residual gaps are accepted/deferred for this development boundary,
  not represented as passing measurements or completed manual checks.

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

All three tasks reached their development handoff. Tasks 1 and 2 passed independent
review and were delivered in signed local commits; Task 3 reached that boundary
by the user's explicit current-state disposition. Its
[Round 2 re-review](reviews/scan-experience-acceptance.md#round-2--2026-09-13)
returned PASS for the repaired candidate
`9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0`, and the
same task is included in the user-authorized local Task 3 delivery commit under
the explicit delivery acceptance exceptions recorded below. Its Review cell is checked; the Task acceptance gate
is VERIFIED under those exceptions, not a claim that the original technical
targets or missing manual checks passed.
Topic development handoffs are 3/3; task review is 3/3. The commit containing this
handoff is Task 3's local delivery boundary; its immutable identity and verification
are recorded in CEv1 and Beads after signing. The topic is not integrated, pushed,
released or retired. Continue from the
review record in workspace `agent-deck.snapshot-performance`; the accepted
optimization/evidence disposition below remains effective. Integration and
retirement remain open; local delivery does not authorize push or assembly.

### Task 3 delivery acceptance exceptions — 2026-09-13

The user explicitly confirmed: "确认 snapshot-performance / scan-experience-acceptance 已列明的性能、最终 20 样本及 V01-V19/native 证据缺口可作为本次交付验收例外，并据此同步验收门禁，保留原始失败和未验证记录。"

This extends the earlier development-only decision to this delivery acceptance
boundary for candidate `9762c53ea4fba19e3493d6ad1852e216149c1bca271b95a87a65362abd08aaa0`.
It accepts the previously named cold-import and unchanged-CPU shortfalls, defers
the final 20-sample campaign, and accepts the incomplete V01-V19 and real-helper/
manual native proof. No new measurements or manual checks are asserted.

CEv1 appends explicit user-disposition observations for the three affected
criteria and supersedes their previous active acceptance evaluations for this
candidate. Original fail/not_verified observations, measurements and review
history remain intact. The other three passing criteria are reused. The resulting
Task gate is VERIFIED by accepted exceptions; original technical outcomes remain
fail/not_verified. Round 2 PASS is unchanged; no new review round is created.
See [acceptance synchronization](reviews/scan-experience-acceptance.md#delivery-acceptance-exceptions--2026-09-13).

This grants no commit, push, integration, release, installation, CGO adoption or
further optimization authority. Task 3 waits for its authorized commit; the topic
is not closed or retired by this decision.

The user's full replan replaces the prior code-first repair/delivery sequence.
The current six-document set, including the shared UX specimens, passed
[document-set re-review](reviews/tasks.md#round-4--2026-09-11).
The document Review cells reflect the corresponding current records. Completion
evidence and delivery checkpoints are recorded there separately from the verdict.
The outstanding technical acceptance gaps are now explicitly accepted/deferred
for Task 3's current development boundary as recorded below. Their original
measurement and verification outcomes remain unchanged.
Task 1 full delivery-scope re-review passed with a VERIFIED completion gate; see
[runtime review](reviews/unified-scan-runtime.md#round-4--2026-09-12).
It was delivered in signed local commit `7ea8dc3e` without push.
Task 2 implementation and L3 verification reached
[Round 1 review](reviews/snapshot-computation-reuse.md#round-1--2026-09-12),
which returned FAIL; [Round 2 re-review](reviews/snapshot-computation-reuse.md#round-2--2026-09-13)
closed its repair and returned PASS. It was subsequently delivered in signed
local commit `e1311ce1` without push. The reviewed 36-file candidate fingerprint is
`a9627671b780b12bc624d1677b0dd67f1459f5e8fabdc3f5a756a090ee9f4be0`,
and its completion gate is VERIFIED. The refreshed representative
worker-accounted report used 2,200 isolated JSONL
files / 2,343,100,564 bytes with corpus SHA-256
`84a0c345e0f8c505134dfe81a3a27d7b785a231ed9a7fc8beeaa26a6baa110d8`.
All three complete samples produced snapshot SHA-256
`db82eb84d1da0260204473d23fc4cfad4905ff2f54fb8fa2eab8dd0ff873f10e`
and logical-row SHA-256
`b1e1be6961abe5e8924774b8b5f40ad94584e68e934f5856b4217b7322560507`:
cold import took 59.054 s wall / 88.528 s CPU / 213.0 MiB peak RSS;
full recomputation took 6.181 s / 7.183 s / 195.3 MiB; unchanged refresh took
5.791 s / 6.754 s / 195.8 MiB. Only full recomputation met its 10-second wall
target. The complete private report SHA-256 is
`5f6af73fcd43b270afa1fef87b72c50c533544e008879bb2acaa5b0412c61b89`.
Cold and unchanged targets remain unmet and are not passing measurements; Task 3
owns the final 20-sample campaign and explicit disposition before topic delivery.
Each task contains its investigation, implementation and verification and needs
no further task decomposition.
No old-code repair is a prerequisite. Git delivery requires a new, scoped
checkpoint for the chosen content and separate authorization; the old document
commit request must not be expanded into delivery of rewritten code.

### Task 3 current-state completion decision — 2026-09-13

Authority: the user's explicit instruction, "task 3 就保留目前现状为完成，记录原因等内容，后续可以继续优化 然后整理整体topic进度，给出下一步指令".
This supersedes the earlier stop-only instruction and the pending CGO adoption
decision for the current iteration. Accept the existing pure-Go implementation
as Task 3's completed development result. No additional optimization, benchmark
campaign or CGO adoption is required before its review handoff.

The accepted implementation remains the 48-file candidate at HEAD `e1311ce1`,
fingerprint `ed6ff6c0a4a77e372e8545c3350541dd5b89265fd0cb2423f4d8888cd4b78755`.
This status/decision update changes no product code, tests, dependencies or build
configuration. CGO remains an isolated experiment and is excluded from the
accepted candidate. Existing exact-state product evidence is reused.

The disposition explicitly carries the following known limits; accepting the
current result does not assert that any missing check ran or any target passed:

| Known limit | Current evidence | Current-round disposition |
| --- | --- | --- |
| Cold import <=10 s | Final pure-Go n=3 median 37.170 s, max 37.850 s; 0/3 met target | Accept current behavior; retain <=10 s as a future optimization goal |
| Unchanged combined CPU <=0.5 s | 2/3 met target; max 0.610 s; wall and RSS met targets in all three samples | Accept current CPU variance; retain the original budget for follow-up |
| Final-content 20-sample campaign | Final candidate has 3 samples/scenario; earlier 20-sample campaign belongs to older code | Defer the final 20-sample acceptance campaign; do not relabel historical evidence |
| Complete V01-V19 and real-helper/manual native acceptance | Automated Go/native checks and differential output pass within their recorded scope; full scenario-specific/manual proof remains incomplete | Accept/defer the named evidence gap; do not claim blanket native or V01-V19 PASS |
| CGO trial | No established complete-cycle benefit; driver-specific error and FTS checks remain failing | Do not adopt it in this iteration; preserve isolated evidence only |

Engineering rationale: the candidate delivers the CLI/App progress and snapshot
reuse work, retains the measured pure-Go improvements, and has passing scoped
L3 and differential evidence. Further cold gains require additional pipeline/
memory-admission work; the isolated driver swap did not justify adoption.
The user chose to finish this iteration at that known state and leave further
optimization possible later. This is an explicit scope/risk disposition, not a
lowered benchmark, fabricated technical PASS, or approval to weaken durability.

The Dev cell is checked; Review remains unchecked. Beads moves to `in_review`,
not `closed` or `awaiting_commit`. The original technical CEv1 gate for `ed6ff6c0a4a7`
remains FAILED on its measured/unverified claims; this approval is a separate
development-completion decision. Independent review must evaluate the candidate
and this accepted exception boundary, then reconcile the applicable completion
evidence before delivery. Do not mechanically reopen development solely because
an explicitly accepted/deferred item retains its original failing or unverified
technical result. No Review PASS, commit, push or topic closure is implied.

Non-blocking follow-up options, not newly dispatched tasks or an active plan:

- Replace conservative whole-file-times-64 admission with properly accounted
  reader/reducer/publication budgets and investigate pipeline overlap.
- Revisit cold-import and unchanged-CPU targets on a representative, controlled
  environment, retaining every failure and a predeclared 20-sample campaign.
- Complete the remaining scenario-specific real-helper/manual native evidence.
- Reconsider alternative SQLite drivers only if new evidence justifies their
  compatibility and build costs; no automatic CGO continuation is authorized.

### Task 3 pre-optimization checkpoint (historical)

Task 3 initially produced a 39-file candidate at HEAD `e1311ce1`, fingerprint
`306840dcdc93ec22d2cd37ec3900169f9dfe6c1130548135b97565a2255312d8`.
This checkpoint and its campaign predate the user's subsequent optimization
request; they do not identify the current implementation or its performance.
At that checkpoint the Dev cell was unchecked pending the original targets and
integrated acceptance; no user disposition had yet been granted.

- CLI and App now consume one versioned aggregate progress stream from the same
  worker. Text/JSON progress stays on stderr; explicit NDJSON carries monotonic
  safe events and one frozen result. Scoped callers detach without trailing
  output or cancelling the global obligation.
- The App consumes events while the helper is running, retains previous data,
  atomically publishes only after successful scan plus snapshot validation, and
  rejects malformed/truncated/unknown/out-of-order streams. English/Chinese,
  420/280 widths, accessibility3, keyboard refresh, first use, failure and
  retained-data states have automated Xcode App coverage. The rendering tests
  use a stub host; this is not evidence of a full real-helper popover lifecycle
  or manual keyboard/VoiceOver acceptance.
- V01-V06 and V10-V13/V17-V18 are covered by worker election, receipt,
  follow-up, detach, maintenance, bounded transport and protocol tests; V07-V09
  and V14-V16 by final usage/session/ingest/store/backup/cache differential and
  mutation tests; V19 by the native helper/coordinator/view-model/rendering suite.
  These are package-level coverage pointers, not a scenario-by-scenario proof
  that the final CLI/App topology passed every V01-V19 condition.
  Full vendored Go, the affected race net, vet, darwin builds, arm64 size and
  full Xcode build/tests pass. The unchanged shared prototype manifest remains
  `cf26b5d5f7afd30bf57eca9f4e7fa6c373eaec0b786e2be5eb02c7de973def8f`.
- The final detached-worker campaign used 2,200 files / 2,343,100,564 bytes,
  corpus SHA-256
  `84a0c345e0f8c505134dfe81a3a27d7b785a231ed9a7fc8beeaa26a6baa110d8`.
  All 60 samples completed with one snapshot digest and one logical-row digest.
  Cold import median/P95/max was 45.080/46.522/48.655 s: 0/20 met 10 s.
  Full recomputation was 4.506/4.805/4.929 s: 20/20 met 10 s. Unchanged was
  1.106/1.272/1.279 s: 0/20 met 1 s wall and 0/20 met 0.5 s combined CPU, while
  20/20 met 100 MiB peak RSS. Report SHA-256 is
  `83db86973cb7100af09c8a88bfd2c7b71303453786ab1f682d7f1bfe6ea4b427`.
  The first campaign attempt was retained separately after the outer Go runner's
  10-minute timeout stopped it at 11 completed cold samples; it was not merged
  into or hidden by the successful 60-sample campaign.
- The user chose further optimization in this Task. The historical request for
  a decision is resolved by that direction, not by waiving any target.

### Task 3 first optimization checkpoint — 2026-09-13

The user authorized another optimization round, including departures from the
current implementation design. This remains Task 3, not a new task matrix or
permission to waive targets, modify live data, commit or push.

That round's 44-file implementation/contract candidate at HEAD `e1311ce1` has
fingerprint `b05c782b02e3fabe81256319f5c8feb65f179f6d27941a692caa9ec5e5c60aaf`.
The recipe remains SHA-256 of newline-delimited `head=<HEAD>` followed by sorted
`<git hash-object --no-filters>  <path>` entries; topic status and review records
are excluded. The shared prototype is unchanged from the checkpoint above.

Retained changes (implementation details in architecture section 8.1):

- Session scans prove the complete unchanged source set with one registry query
  and revalidate captured file generations. Proven no-write scans avoid two full
  visible-document materializations and per-source SQL planning.
- Session FTS insertion uses bounded 64-row / 1 MiB text batches within the
  original atomic source transaction. Usage orphan recovery subtracts the
  registered path set before reading orphan session identities.
- A same-length rewrite with restored mtime must not pass a weaker prefix/suffix
  anchor fallback after ctime changes. New session and usage regressions mutate
  content beyond those anchors and assert updated search/token results. The
  session reproducer failed before repair; both now pass. Batch rollback restores
  the exact previous index after an injected metadata failure.
- The isolated end-to-end test's Claude stub no longer rewrites its Codex source:
  doing so correctly invalidates the earlier exact-run binding under the new
  ctime check. Provider-attribution assertions are retained and pass.

Rejected experiments: memory-only SQLite scratch storage, extra composite turn
indexes, plain event inserts and transition coalescing did not establish a
repeatable cold-import benefit. They are removed; source transaction boundaries,
UPSERT ownership behavior, constraints and durability are preserved. Private
diagnostic reports retain failed samples, including a worker startup timeout in
the rejected extra-index/plain-insert candidate. No failed sample is erased or
merged into the final candidate's results.

Final-code verification passed:

- Full Go regression: `/private/tmp/agentdeck-opt-round-full-final.log`, SHA-256
  `5c790fb72dcc447b740e61e791b8592fe517b548a69f22211b429debe650a79c`.
- Race: session, usage, scanruntime, store, desktop and cmd/agentdeck;
  `/private/tmp/agentdeck-opt-round-race-final.log`, SHA-256
  `abdfcd3d63169736820b8bd38d15f69630d4617f8cf782ea1612e3e813fd67d4`.
- Vet and darwin arm64/amd64 builds passed; arm64 is 13,879,362 bytes, under
  26,214,400. Artifacts: `/private/tmp/agentdeck-opt-round-verified-bin`.
- Reused automated Xcode evidence: the 13 changed native source/test blobs
  match the prior manifest, and this optimization does not change Swift,
  localization, helper wire format or build configuration. The original log is
  `1789297394_make_test-macos-app.log` in the private RTK tee directory, SHA-256
  `c04e61a47d4241215c2df6be0559fd0bf894f337e1d215bdde50eace34932f57`.
  This reuse covers the actual automated tests, including stub-host native-width
  rendering and malformed stream checks; it does not fill the manual/real-helper
  acceptance gaps described above.

Final-code diagnostic measurement (predeclared 3 samples per scenario, sequential
after race tests, not the required 20-sample acceptance campaign):

| Scenario | Wall median / max | Max combined CPU | Max peak RSS | Within original targets |
| --- | --- | --- | --- | --- |
| Cold import | 61.953 / 62.207 s | 93.141 s | 215,801,856 B | 0/3 |
| Full recomputation | 5.068 / 5.472 s | 5.432 s | 184,889,344 B | 3/3 |
| Unchanged refresh | 0.473 / 0.482 s | 0.486 s | 35,082,240 B | 3/3 |

All 9 samples completed. The report includes every sample and its failure/target
flags: `/private/tmp/agentdeck-opt-round-verified-measure.json`, SHA-256
`db71c304747ad5ece69592c9af4aaace39ee923495a9eb52fb748fc44e6a5151`.
The harness passed because it completed and preserved its measurements; this is
not a passing performance gate. No meaningful P95 claim is made from n=3.
Environment: Go 1.27.1, darwin/amd64, 12 logical CPUs, fixed business time and
UTC; OS cache and external load uncontrolled. The unchanged 2,200-file /
2,343,100,564-byte corpus has SHA-256
`84a0c345e0f8c505134dfe81a3a27d7b785a231ed9a7fc8beeaa26a6baa110d8`.
Every sample matches the same-session old-binary control's snapshot SHA-256
`7e3544e6af5bd2639175411d534434dc93fa3e7fbfec24601b61f0ebde050e5e`
and logical-row SHA-256
`b1e1be6961abe5e8924774b8b5f40ad94584e68e934f5856b4217b7322560507`.

The single old-binary control's unchanged cycle was 1.195 s wall / 1.593 s CPU /
76,079,104 B RSS. The final three-sample unchanged medians are 0.473 s wall /
0.485 s CPU / 34,304,000 B RSS (observed reductions of approximately 60% / 70% /
55%). This is a small uncontrolled comparison, not final acceptance or an
isolated attribution to each optimization. Cold final median 61.953 s is slower
than that control's 54.895 s and the earlier selected candidate's 51.744 s;
no repeatable cold gain is established. The CPU profile still motivates a
subsequent cold reduction/publication redesign inside this Task, preserving
atomicity and differential proof; it does not authorize durability weakening.

At that checkpoint Dev/Review remained unchecked: cold import still needed the original <=10 s target,
the final content needs the prescribed 20-sample acceptance campaign, and the
integrated V01-V19/native real-helper lifecycle needs scenario-specific evidence.
The previous package-level/native PASS claims are not promoted into those missing
acceptance results.

That checkpoint's Task CEv1 gate is `FAILED`: performance has an applicable failing
observation; integrated V01-V19 and native real-helper/manual acceptance remain
`not_verified`. CLI progress, reconciled contracts and scoped L3/automated-native
checks have applicable passing evidence. Native reuse has an explicit
scope-preserving assessment; there are no unresolved candidate impacts. Beads
remained `in_progress`; no Task/topic completion, independent review, commit or
push was claimed. L0 whitespace/topic-document/diff checks passed; this status
projection does not change the implementation fingerprint above.

### Task 3 cold-publication continuation and native trial — 2026-09-13

The user renewed Development authority and separately approved an isolated CGO
trial, with adoption to be decided after verification. The pure-Go worktree is
still the implementation candidate; no native driver, dependency or build-tag
change has been copied into it. The current 48-file fingerprint is
`ed6ff6c0a4a77e372e8545c3350541dd5b89265fd0cb2423f4d8888cd4b78755`
at HEAD `e1311ce1`, using the same recipe and exclusions above.

Retained pure-Go changes:

- Transaction-local absent-key proof and bounded cold-source bulk publication.
  Sequential-reference tests compare all event/tool/file-link columns, duplicate
  logical-change counts, restart/completion semantics and ordering. Existing
  keys fall back before writes; a late injected failure rolls back every batch.
- Ordinary record lines borrow the reader buffer until JSON decoding completes;
  oversized records and retained partial tails retain their previous semantics.
  A 1 MiB read-loop microbenchmark (100 iterations) changed from 1,025 allocations
  / 1,059,202 B per iteration to 1 allocation / 10,518 B. This is allocation
  evidence, not a complete-cycle throughput claim.
- INSERT OR ROLLBACK and a fixed four-worker record decoder were evaluated then
  removed: single-sample results did not justify retaining the extra policy or
  concurrency. Their private reports remain as discarded-candidate evidence.

Final pure-Go full regression and affected race selection pass:
`/private/tmp/agentdeck-cold-selected-full.log` SHA-256
`f2c136dcfa9585b6fd2b3880edcbe10e47ea06c90e1dee5473ceb4a3c4e14c29`;
`/private/tmp/agentdeck-cold-selected-race.log` SHA-256
`08aaf4e504d6b032e7ab9221608e3531f48fd68a53bb9bc5b8572145ba4b4bf1`.
The race selection covers Scan/Source/Ingestion/Cold/RecordLine/Coordinator/
Snapshot/Derived tests in usage, ingest, session, scanruntime, store, desktop and
cmd/agentdeck. Vet, darwin arm64/amd64 and arm64 size pass (13,895,954 B).
The unchanged automated native App evidence remains reusable with its previously
recorded limits; no manual or real-helper acceptance is inferred.

The private CGO trial is `/private/tmp/agentdeck-native-sqlite.YE0M8t`, based on
the same product code and pinned `github.com/mattn/go-sqlite3 v1.14.52`. It changes
store-owned core/session connections and online backup; existing direct raw
doctor/test connections still use modernc. SQLite versions are 3.53.2 (modernc)
and 3.53.4 (CGO), so the comparison is not an isolated driver-only attribution.
Both paths were checked for WAL, synchronous FULL, foreign keys ON, busy timeout
5000 and FTS5. The adapter overrides the native driver's weaker NORMAL default.
Both native darwin architectures build and satisfy the size limit; the first
arm64 artifact is 15,420,258 B and links only Apple system libraries/frameworks,
not a separately installed SQLite dylib. This is build evidence, not execution
evidence for arm64 or adoption approval.

CGO compatibility remains incomplete. The original FULL-durability suite failed
on a connection-open lock race and five driver-specific error assertions.
Instrumented repetition localized the lock race to native connection setup
before the source's busy timeout was applied (6/20 failures). Applying the same
timeout in the native DSN before opening passes 20/20 and removes that failure
from the refreshed broad suite. Original driver-specific assertions remain
unchanged and failing. Separate native-error diagnostics preserve the joined
provider error cause and prove exclusion/rebuild atomicity; two FTS insert-failure
diagnostics still fail the expected extended-code check before their complete
state comparisons, so those cases are not claimed verified. No failing test was
waived, relabelled or removed from the original suite.

The refreshed native broad log is `/private/tmp/agentdeck-native-full-adapted.log`,
SHA-256 `24662914d7b704a488473237fb3195b3b27ad0f148512ef10540e1b71971ef25`.
Five original tests plus two additional FTS diagnostic tests fail; it is not a
passing replacement candidate. The lock reproducer/fix logs are
`/private/tmp/agentdeck-native-lock-repro.log` and
`/private/tmp/agentdeck-native-lock-fixed.log`.

The sequential pure-Go/CGO diagnostic comparison used three complete samples per
scenario per implementation (18 total), not a 20-sample acceptance campaign:

| Implementation | Cold wall median / max | Recompute wall median / max | Unchanged wall median / max | Unchanged CPU target |
| --- | --- | --- | --- | --- |
| Current pure Go | 37.170 / 37.850 s | 6.448 / 7.868 s | 0.489 / 0.631 s | 2/3 <=0.5 s; max 0.610 s |
| Isolated CGO, FULL | 55.498 / 58.065 s | 6.076 / 6.104 s | 0.610 / 0.627 s | 0/3 <=0.5 s; max 0.615 s |

Every sample completed and matches the prior snapshot and logical-row digests.
All unchanged wall/RSS samples meet 1 s / 100 MiB; cold remains 0/3 <=10 s in
both variants. Pure-Go cold max CPU/RSS is 53.874 s / 207,060,992 B; CGO is
85.576 s / 208,723,968 B. CGO cold samples were 34.746, 58.065 and 55.498 s;
the large variation and uncontrolled OS cache/external load prevent a universal
driver ranking or clean attribution. No meaningful P95 claim is made for n=3.
The repeated measurements support keeping the source-batch optimization; they
do not establish a repeatable CGO advantage or satisfy the original targets.

- Pure-Go report: `/private/tmp/agentdeck-cold-selected-3.json`, SHA-256
  `116c208982d25ce457d6f36c2289d8a8096a70addc8b946b350596cf4e116aef`;
  measured executable SHA-256
  `822cdaa3c0b16aaaf169c30ce21247903f979de3639ae92f92f9a5da23cbc975`.
- CGO report: `/private/tmp/agentdeck-native-adapted-3.json`, SHA-256
  `daaf549d35dc723cecc2ca25d88880ba1fc8a1aba510f3de50cfc73db3a67f47`;
  measured executable SHA-256
  `1367a87fdcb309dc918ace029dc9f7665f4be417f3ed33ce83be75c7cb3a980c`.

Recommendation at the user-requested adoption decision: do not merge this CGO
trial into the candidate. It has no established complete-cycle advantage, still
requires error-contract/FTS compatibility work, and adds C-toolchain/build-tag
obligations. Retain the isolated sources and logs as evidence, not shipped code.
Only the isolated experiment was authorized; CGO adoption was not authorized.
A possible later pure-Go opportunity is the shared reduction/publication pipeline and
its conservative whole-file-times-64 admission, not another unproven driver swap.
At that checkpoint the work and V01-V19/manual/native acceptance remained inside
Task 3; no fourth task matrix, target waiver, Dev/Review completion, commit or push
had been recorded.
The technical `ed6ff6c0a4a7` Task CEv1 gate is `FAILED`: performance fails, while
real-helper/manual native and integrated V01-V19 acceptance remain
`not_verified`. CLI, stable-contract reconciliation and scoped L3/automated
native evidence pass, with no unresolved candidate impacts. At this historical
checkpoint Beads remained `in_progress`; the current-state completion decision
above supersedes that administrative status.

The user's earlier stop-only instruction, "不再继续优化", stopped further optimization.
Preserve the current pure-Go candidate, isolated CGO sources and all evidence;
do not merge CGO or start another optimization/measurement round. This instruction
does not waive the original targets, establish acceptance, authorize delivery or
complete Task 3 at that point. The later explicit current-state completion
decision above now governs the development result and review handoff. The
optimization suggestions remain historical options, not active instructions.

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
