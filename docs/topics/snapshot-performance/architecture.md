---
status: active
created: 2026-09-09
updated: 2026-09-09
---

# Snapshot Performance — Architecture

## Current baseline

Source baseline: `f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f` in workspace
`agent-deck.snapshot-performance`. Current relevant entry points are
`cmd/agentdeck/desktop.go`, `internal/desktop/desktop.go`, and the usage/session
readers. `EmbeddedHelperRunner` runs `refresh-indexes` before streamed `snapshot`.
The snapshot timeout is 30 seconds; index refresh has a separate 120-second
timeout. These are failure bounds, not performance targets, and are not raised
by this design.

`Presentation` reads 90 days of events and builds all presentation dimensions.
`loadWorkSignals` invokes `Signals` for three periods and three client scopes.
Those calls repeat event reads and price/route resolution. `PriceStatus` also
builds effective prices although quick doctor only consumes availability.
Session discovery and source checkpointing have their own correctness rules.

## Selected structure

Keep the existing Go/SQLite stack, CLI commands, JSON/chunk contract and surface
behavior. Do not adopt the research-only native driver, system-SQLite bulk
loader, reduced durability, process-global GC settings or experimental Go JSON
APIs as dependencies of this design.

Separate the implementation into three cooperating parts:

1. **Source preparation:** discover once, validate processed source signatures,
   and avoid scans only when the corresponding index and parser state qualify.
2. **Derived-data builder:** share request-local inputs and calculation work;
   build a bounded cache after authorized refresh writes.
3. **Read-only snapshot:** validate source databases and cache identity, reuse
   eligible derived parts, and calculate live parts using the existing behavior.

The cache is an accelerator. The authoritative facts remain the two databases
and their source registries. An unchanged fast path must prove both that the
sources are unchanged and that the indexes are still the indexes which processed
them. Cache validity alone does not prove ingestion freshness.

## Source preparation and shared parsing

Use the same source-root selection and exclusion rules as the existing usage and
session scanners. A shared inventory can serve both domains, but retain each
domain's source priority and processing/ownership semantics.

A processed signature includes logical path, file identity, size, mtime,
change-time and relevant mode, plus parser version. Persist change-time through
the appropriate source-registry migration; rows without a trustworthy signature
require validation/scan before becoming eligible for fast skip. On a filesystem
without a reliable identity/change-time signal, use the existing validated scan
path instead of asserting unchanged input.

Bind a domain's completed-input checkpoint to its database epoch, parser version,
root selection and digest of the actually processed signatures. Publish it only
after the domain's data/cursor commit succeeds and a final source check confirms
those signatures. A post-scan observation of newer bytes must not advance the
checkpoint past the bytes actually processed. Failed or cancelled persistence
leaves the fast path ineligible, even if parsing itself completed.

For changed inputs, share source reads and record decoding through a coordinator.
Default to four parsing workers, capped by available parallelism, and a 64 MiB
budget for queued encoded derived records. These are initial implementation
parameters, not measured optimal values. Keep stateful parsing ordered within a
source, including cumulative counters and pending-turn context. Pass machine
identity explicitly; omitting it loses file associations and changes work-signal
classification.

Each domain consumes its existing approved representation and retains its own
commit/checkpoint outcome. Do not force both domains into one artificial source
order or require them to consume a shared slot simultaneously. A coordinator
owns scheduling; if their required orders cause queued results to exceed the
budget, spool only approved derived records in a private temporary directory.
Never spool raw reasoning or unrestricted tool bodies. Clean owned spool files
on success, error and cancellation, with bounded cleanup of abandoned files.

Retain the existing parser's validation and supported-record behavior. Standard
decoding is the default; any selective decoder must pass malformed-number,
duplicate-key, invalid-UTF8, field-order and retained-content equivalence tests.
The design does not introduce a new record-size rejection threshold. Oversized
supported records take a single-source path; measure their memory separately.

## Cold import optimization policy

The cold target remains 10 seconds. It is not proven by the research and does
not block completing this design. Re-evaluate the alternatives on a controlled,
fixed corpus during Task 2 before incorporating them into the implementation.

The safe default is shared preparation with existing write and checkpoint
semantics. Evaluate prepared-statement reuse and bounded batches first. A
separate cold-import strategy may apply only after proving the relevant derived
indexes are empty; preserve providers, credentials, prices and unrelated state.
It must not be selected merely because a cache is absent.

If batch publication is chosen, retain unique/foreign-key constraints and
durability, preserve duplicate ownership and classification, and commit derived
rows with their source cursors. Rollback must leave a retryable state. Do not
reuse a cold-only assumption for append, rewrite, orphan recovery or rebuild.
Record actual gains before adding complexity. Rejected research variants are
not production requirements.

If the implemented path still exceeds 10 seconds, record the measured gap and
the next evidence-backed alternative in the task's existing record. Do not
change the target, disguise omitted work or claim performance acceptance. The
user's design-stage relaxation is not a future delivery waiver.

## Database generations and migration

Add a core generation record with a random database epoch, monotonic revision
and dirty flag. Core application-table mutations invalidate derived reuse;
exclude only the generation/checkpoint bookkeeping itself from its trigger set.
Register coverage during owned migrations and check coverage/version before
trusting cache metadata. A missing/incompatible marker disables reuse.

Coalesce invalidation while dirty: the first mutation after a clean checkpoint
increments revision and sets dirty; further dirty writes need not increment it
per row. Readers never accept a cache while dirty. Before a rebuild, the writer
briefly obtains normal write authority, increments revision and clears dirty.
Any subsequent committed mutation then invalidates that build. This avoids
pretending that a dirty revision alone identifies all intervening data states.

The builder records epoch/revision before reading and checks epoch/revision/dirty
after all derived computations. Publish only if unchanged and still in the
declared time window. A change after this check still causes readers to reject
the old revision. Do not hold the state write lock for the entire aggregation;
serialize cache publishers with a separate bounded lock and make one build
attempt per refresh rather than looping indefinitely under active writes.

Give the session index its own epoch for ingestion-checkpoint identity. Session
facts are read live, not stored in this derived cache. Missing/rebuilt sessions
must force the appropriate scan/read outcome independently of core validity.

Allocate the next available core migration version during implementation; the
baseline version is 23, but concurrent topics may consume the next number.
Follow the existing session-schema migration owner for session changes. Mint or
invalidate epochs on owned creation, restore and rebuild paths, including backup
restore. File replacement identity also invalidates reuse. Old-schema readers
must fall back without creating or migrating state; future schemas keep their
existing refusal and partial-snapshot behavior.

## Cache contents and publication

Use a versioned cache under the selected state root, with directory mode 0700
and file mode 0600. Limit an entry to 8 MiB; retain one active entry and one
publisher's temporary entry. Oversized results use the uncached path. Reject
symlinks, truncated/malformed data, checksum failures and unsupported versions.
Use a same-directory temporary write, complete write/sync and atomic rename;
never expose a partially written entry. Remove only owned abandoned temporaries.

The header binds cache-format/algorithm version, core epoch/revision, schema and
parser versions, request semantics, source database identity, creation time and
valid-until. Bind timezone identity and the UTC boundaries of the relevant local
calendar window, not merely the current offset or the string `Local`.

Validity ends at the earliest applicable local hour/day boundary or scheduled
price-effective boundary. Reject clock rollback before creation, timezone
changes and identity mismatches. A fixed TTL alone cannot establish validity.

The payload contains usage presentation, its summary, work-signal aggregates and
successfully validated price availability. Use a dedicated internal DTO:
`PresentationReport.Summary` has `json:"-"`, so a public-JSON round trip cannot
preserve that internal calculation field. Restore the summary explicitly or
reconstruct the final usage snapshot from the stored summary and presentation.
Do not retain a whole event list or a per-event price memo across requests.

Keep provider routes, credentials, locks, schema/open checks, refusal diagnostics
and session availability live. Desktop quick health may receive a validated
price-availability reader; ordinary `price status` and `doctor` keep their default
paths. Do not cache credentials or the complete health report. Price validation
errors are not successful cacheable availability results.

## Read and refresh behavior

`snapshot` first performs the normal core/session opening and compatibility
checks. It may read an eligible cache, then validate generation again before
using it in the response. If a generation changes during the read, discard reused
parts and use a fresh uncached calculation. Continuous writes must not cause an
unbounded retry loop. This does not create a cross-database atomicity guarantee.

On a miss, compute the current report without writing a cache. Preserve all
existing partial/error rules. On authorized `refresh-indexes`, a successful
unchanged checkpoint and valid derived entry allow skipping scans; missing
indexes, changed signatures, parser/schema changes or unreadable roots do not.
After required scans/checkpoints, attempt derived-cache construction.

Cache write failure does not discard successful index commits. Keep the cache
unavailable and let snapshot recompute. Refresh domain results continue to
include checkpoint-persistence success: a completed session scan followed by a
cancelled fingerprint write is not a successful completed domain refresh. Keep
diagnostic stage timings separate so this is not mistaken for parser failure.

Existing JSON fields and stream checksums remain unchanged. Do not add a public
cache toggle, a daemon, new freshness copy or longer timeouts. Report timings
through the existing refresh durations and benchmark artifacts; never put paths,
transcript text or credential material into diagnostics.

## Research observations — not implementation acceptance

The isolated corpus contained 1,720 files / 2,202,066,977 bytes. Measurements used
Go 1.27.1 on Intel i7-9750H, 12 logical CPUs and 32 GiB RAM. Production toolchain
requirements remain unchanged. Background load varied substantially; early
samples also crossed local midnight. These exploratory results are not a
controlled ranking or universal speed claim.

| Observation | Result / limitation |
| --- | --- |
| Complete snapshot on copied populated databases | 13.40, 22.67, 18.35 s; approximately 209–226 MiB RSS; no partial result |
| Stage probe | Usage 5.34 s, work signals 4.70 s, sessions 68 ms, health without vault 681 ms; one diagnostic sample |
| Original cold import | Usage exceeded a 90 s diagnostic deadline; session scan succeeded in another probe but later fingerprint persistence inherited cancellation |
| Shared decoding prototype | 28.37 s import plus 5.99 s snapshot; seven logical result tables matched the completed reference |
| Atomic-domain prototype | 21.80 s import plus 3.97 s snapshot; same seven-table comparison passed; not an adopted recovery policy |
| Incorrect compact prototype | Lost machine-identity-derived file links; its faster result was rejected |
| Request-local event/price memo | Fixed-clock JSON matched; 11.42 s to 5.69 s, but RSS rose to about 416 MiB; not a suitable retained-data design |
| Derived-cache CLI prototype | 20 cycles including two process starts and stream decoding: median 194 ms, P95 295 ms, maximum 1.990 s; CPU P95 209 ms, peak child RSS about 20 MiB |
| Cache guards | Malformed/checksum failures, expiry, epoch/revision/format mismatch rejected; complete serialized result matched recomputation |

The CLI outlier was its first measured invocation; its precise cause was not
established. Do not omit it when reporting the 1-second target. Cache tests above
do not yet prove all concurrent, timezone, future-price or source-rewrite cases.
Cold prototypes are limited to frozen, empty-index experiments, not incremental
or production acceptance. No cold prototype established the complete 10-second
target. The latest user decision carries that work into implementation.

## Verification and integration

Use full-output and logical-table differential checks against the unchanged
reference, then targeted failure/concurrency tests and the project L0–L3 matrix.
Benchmark source discovery, parsing, writes, cache construction, live health,
process startup and stream decoding separately as well as end-to-end. Keep
fixture construction, compiler time and actual command initialization distinct.

Update the affected stable contract/manual at topic closure, describing read-only
fallback and invalidation without presenting target numbers as achieved facts.
The desktop-refresh topic may consume this work only with its actual measured
capacity and remaining limits; it must not infer permission to increase polling
from a design document or a cache-hit microbenchmark.
