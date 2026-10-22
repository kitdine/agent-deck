# P07 measurement tools

These tools provide instrumentation and synthetic calibration only. They do not
capture sources, open production databases, launch AgentDeck, grant data access,
or decide P07 acceptance. Task 3 remains open. Synthetic results cannot replace
representative frozen input, P01 non-regression, switching or native P10 evidence.

## Transaction boundaries

`scripts/p07-overlay` type-checks the workspace's production SQL users and writes
a Go overlay plus an application manifest to a new private directory. It changes
no product file. Ordinary builds do not import `scripts/p07measure`.

The overlay wraps `sql.DB.Begin/BeginTx`, `sql.Conn.BeginTx`, `sql.Tx.Commit` and
`Rollback`, raw `BEGIN IMMEDIATE` / `COMMIT` / `ROLLBACK` on pinned connections,
and autocommit SQL through DB/Conn statement APIs. It identifies handles as `core` or `session` from the
database filename at `sql.Open`, and propagates that identity to `sql.Conn`.
Unknown handles are retained as unknown and make a summary incomplete. The
manifest records original and instrumented source digests, every affected site,
and the coverage rule. Escaping observed SQL method values and unsupported method
expressions are rejected. Regenerate after changing the candidate; do not reuse
an overlay against a different source identity.

Each attempt has a random run identity, PID and increasing local transaction ID.
A begin marker is logged before invoking Begin. The monotonic duration starts
immediately before the real Begin call and ends immediately after the explicit
Commit or Rollback returns, including pool/lock wait, work and terminal-call time.
Default transactions are conservatively write-capable; explicit ReadOnly
transactions are reported separately. Successful commits and rollbacks enter the
write distribution, grouped by core/session. Errors and unknown domains are
retained separately. There is one terminal record per transaction: a deferred
Rollback after Commit cannot add a second sample. Raw failed Commit may leave
SQLite's transaction open; its eventual rollback determines that endpoint.

For a supported single-statement autocommit write, timing starts immediately
before `Exec/ExecContext` or `QueryRow/QueryRowContext`. Exec ends on its return;
`UPDATE ... RETURNING` ends only after the existing `sql.Row.Scan` returns, which
also consumes/closes its Rows. QueryRow's early return is never a terminal
sample. The original Row and call order are preserved: there is no eager Scan,
extra SQL, injected Begin/Commit, or forced close. An omitted Scan leaves an
unmatched begin; Flush exposes it as invalid. Repeated Scan cannot duplicate a
terminal sample. DB writes and pinned-connection autocommit writes are included;
statements in observed raw or Go transactions on a pinned connection are
suppressed so that only their enclosing explicit transaction is counted.

Only unambiguous single SQL statements are admitted to this autocommit
distribution. Quoted strings/identifiers and comments are parsed before checking
statement boundaries. Read statements are excluded. A bounded recognizer also
excludes read-only WITH queries: it checks every CTE declaration/body and a
terminal SELECT/VALUES, including recursive declarations, optional column lists,
multiple/nested CTEs and MATERIALIZED/NOT MATERIALIZED hints. Quoted identifiers,
literals, comments and balanced parentheses are distinguished. A write body or
terminal statement, malformed/ambiguous declaration, compound string or nesting
beyond 64 query levels remains unknown; WITH alone is never a read whitelist.
Other unknown SQL (including unsupported CTEs,
multi-statement strings and transaction-affecting PRAGMAs) creates an invalid
observation, rather than silently disappearing. Connection-local busy_timeout
and foreign_keys settings do not enter the data-write distribution. Deferred
prepared writes and mutating `Query/QueryContext` streams have no complete
terminal observation in this collector; their statement APIs explicitly record
coverage gaps. A run containing any such gap cannot produce `complete: true`;
resolve these gaps before attempting a representative acceptance claim.

Transaction logs and summaries use `p07-transactions/v2`, require the
`explicit-and-autocommit` process coverage declaration, and retain a boundary
kind per terminal record. Successful `implicit_commit` observations enter the
same domain distribution as explicit writes, with separate boundary counts.
The collector rejects v1 headers; old explicit-only calibration cannot be
reinterpreted as covering implicit writes.

Cancellation can cause `database/sql` to roll back internally without an observed
public terminal call. These records are invalid, even if later Rollback returns
ErrTxDone. An unmatched begin marker, missing terminal wrapper, malformed/truncated
log or duplicate identity cannot silently become a completed sample. Private
in-process harnesses must call `p07measure.Flush()` after joining workers; it
records pending spans as invalid and performs no database operation. A killed
process leaves unmatched begin markers. Combine all measured process logs before
checking coverage; no missing core/session stream may be assumed empty.

`source_commit` is unchanged and remains commit-only. Unsupported transaction
control, including SAVEPOINT SQL outside an observed explicit transaction,
invalidates autocommit coverage; it is never counted as a complete write.
The overlay preserves SQL, transaction options, timeouts, context, errors and
retry behavior. JSON writes add observer cost; the calibration reports both
memory and private-file sinks. Logging errors stop the measurement run, never
turn into silent sample loss. Logs contain sites, identities and timings, without
SQL, arguments, source/database paths or error strings. Log output is exclusive
and mode 0600.

## RSS process set and clock

`scripts/scan_performance_p07.py rss` attaches to two explicit roles: engine and
collector. By default the collector is the sampler itself. Declare a containing
collector root when another controller must also be included. It samples the
union of those roots and recursively discovered children; overlapping roots and
threads cannot double count a PID. Roots are pinned by PID and birth time at
attachment. A previously observed descendant remains included after reparenting
while its birth identity matches. A reused PID cannot inherit membership; a new
descendant must be discovered again from a current member. Detaching a process
before it has ever been observed cannot be recovered by polling alone. The
overlay also observes typed `os/exec.Cmd` Start/Wait/Run/Output/CombinedOutput
calls. It reads the child's real BSD birth after Start and before Wait can reap
it, then records Wait-return exit evidence in the private barrier directory.
The collector binds these facts to an already admitted parent PID/birth. An
unbound identity or missing pre-cut exit proof still invalidates the frame.

The macOS backend uses scoped libproc child enumeration, BSD birth identity
(seconds and microseconds), and TASKINFO resident bytes. It does not inspect
command lines or enumerate unrelated process metadata. Process enumeration is
limited to 4096 children per member; a full buffer, permission error or
unsupported platform aborts instead of returning zero. Each frame sums current
resident bytes once per PID. Shared pages may be counted in each process's RSS;
this is a sum of RSS, not physical footprint or a sum of individual lifetime
high-water marks. The maximum is a **sampled aggregate maximum**. Peaks and
children shorter than the fixed interval can be missed.

The interval is fixed at **50 ms** on a monotonic deadline clock. The first poll
is immediate. Delayed deadlines are recorded and skipped, with no catch-up burst.
Each frame records polling time, lateness, PID membership and process gaps. The
summary includes missed ticks, incomplete frames, maximum observed gap, total
poll time and collector CPU/wall cost. Missing roots, unavailable resident
endpoints and observed topology races make the trace incomplete. RSS reads are
sequential within a frame; `acquisition_start_ns` through `cut_ns` gives the
acquisition span, with membership at its end. A bound EVFILT_PROC/NOTE_EXIT or
parent Wait-return observed at/before that cut can justify an exited member's
absence. A matching BSD SZOMB state or explicit ESRCH for a previously pinned
birth also records a kernel proof before the cut; an empty result with errno=0
does not. No zero RSS is inserted. Unknown child IDs and reused births remain
explicit gaps. Lifecycle records include PID/birth, parent identity, observer
timestamps and executable basename, never arguments. A trace marked
valid establishes collection integrity only, never performance acceptance.

A future authorized P07 harness must attach before releasing an engine start
barrier, include every collector/worker root, keep roots alive through an end
barrier and retain the entire measurement window. Otherwise unobserved startup
and terminal intervals must be declared. This tool adds no product barrier or
process-lifecycle behavior in ordinary binaries. Overlay barriers are opt-in.

RSS JSON encoding, writes and flushes run on one writer thread behind a bounded
256-record queue. Every overflow or write error invalidates the run; no old frame
is deleted. Drain cost, writer memory, CPU and any missed ticks remain included.
The end window waits for uncompleted business children as well as managed Go
workers; roots remain held until the final frame and writer drain. Late admitted
children get bounded same-frame discovery/RSS acquisition, with unresolved
membership still invalidating the frame. Checked-in dependency exec sites may
also be instrumented in private overlay copies; original vendor files stay intact.

Kernel `NOTE_FORK` events also trigger at most three same-frame subtree rescans,
even without a Go child ledger. New residents are birth-checked and counted once;
forks delivered during those reads trigger the next bounded pass. A late event
or exhausted closure bound is an explicit gap. Darwin supplies only the parent's
identity and may coalesce forks, so finding a live child or observing a known
child's exit cannot prove that every fork was covered: another descendant could
already have reparented. Such events retain `fork_membership_unresolved`, with
the first unresolved fact carried into later frames in bounded state. These
frames and the whole trace stay invalid, including when the visible census looks
stable. This intentionally prevents a completeness claim; it does not establish
a usable representative RSS acceptance channel or repair any historical trace.

Transaction records also carry public caller function names and Begin-call,
body and terminal-return phases. The three phases partition the original full
boundary; Begin-call includes pool/lock wait, while SQL waits inside the body
remain body time. This attribution does not assert isolated SQLite lock time.

## Reproduce synthetic verification

From the selected worktree:

```sh
scripts/run-go-test.sh -race ./scripts/p07measure ./scripts/p07-overlay
python3 -B -m unittest discover -s scripts -p scan_performance_p07_test.py -v
env GOCACHE=/private/tmp/agent-deck-go-build go vet ./scripts/p07measure ./scripts/p07-overlay
p07_root=$(mktemp -d /private/tmp/agentdeck-p07.XXXXXX)
env GOCACHE=/private/tmp/agent-deck-go-build go run ./scripts/p07-overlay -repo . -out "$p07_root/overlay"
env GOCACHE=/private/tmp/agent-deck-go-build go build -overlay "$p07_root/overlay/overlay.json" -o "$p07_root/agentdeck" ./cmd/agentdeck
```

The Go tests create only synthetic SQLite databases. The Python native test
creates a controlled engine/child/grandchild tree and uses the test runner as
collector. It checks membership and deduplication and prints synthetic RSS and
sampler overhead. It may require an execution channel permitted to inspect its
own test processes on macOS. Calibration is noisy, limited to this synthetic
workload, and has no representative-scale overhead or P07 passing threshold.

`partition_regression_test.go.tmpl` is a synthetic store regression injected
through the generated overlay only: prepare a temporary store, start a fresh
observer window, then call the actual `BeginUsagePartition` and
`PublishUsagePartition`. The original collector recorded 0 writes; the repaired
collector must record exactly two valid autocommit boundaries and preserve the
published revision/payload. The template never touches the real database.
After generating the overlay above, add this private test mapping and run it:

```sh
python3 -B - "$p07_root/overlay/overlay.json" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
value = json.loads(p.read_text())
repo = pathlib.Path.cwd()
value['Replace'][str(repo / 'internal/store/z_p07_partition_test.go')] = str(repo / 'scripts/p07measure/partition_regression_test.go.tmpl')
p.write_text(json.dumps(value) + '\n')
PY
scripts/run-go-test.sh -overlay "$p07_root/overlay/overlay.json" ./internal/store -run '^TestP07ImplicitPartitionTransactions$'
```

For the actual read-only session CTE, add a second virtual test mapping in the
same way: `internal/session/z_p07_cte_test.go` maps to
`scripts/p07measure/session_cte_regression_test.go.tmpl`. Run it with
`scripts/run-go-test.sh -overlay "$p07_root/overlay/overlay.json" ./internal/session -run '^TestP07SessionListCTEDoesNotPolluteWrites$'`.
It seeds two synthetic owners and two real synthetic writes (core/session),
then calls the actual product `session.List`. The winner must remain unchanged
and the read query must add zero log bytes; the two genuine writes stay complete.

## Interfaces for a later authorized run

Set `AGENTDECK_P07_TX_DIR` to an existing private (0700) output directory. Each
instrumented process creates its own exclusive 0600 log and process header, so
children can safely inherit this setting. For a single process only,
`AGENTDECK_P07_TX_LOG` selects one **new** JSONL file; never inherit that filename
into another instrumented process. The two settings are mutually exclusive.
Every process using the measured databases needs the overlay and its own log;
compiling an ordinary helper loses coverage. Attach
RSS to already-declared roots with `--engine-pid`, optional `--collector-pid`,
`--seconds` and `--output`. The CLI never spawns a workload. Summarize collected
transaction logs with:

```sh
python3 -B scripts/scan_performance_p07.py transactions <engine.jsonl> <other-process.jsonl>
```

The result retains rejected records and unmatched begins and uses nearest-rank
p95 and maximum of all completed write-capable transactions in each domain;
there is no per-source aggregation or sampling of transactions. Check its
`complete` field and execution/window provenance before using the distributions.
No representative P07 run is authorized by these instructions.

## WAL initialization and process sampling barriers

The exact single statement `PRAGMA journal_mode=WAL` remains a database
initialization write, labelled `operation=journal_mode_wal`, and enters its
core/session autocommit distribution once. Timing covers the original
ExecContext invocation through return, including pool and lock wait. The
vendored modernc Exec drains ROW results to DONE, then resets/finalizes before
return. Other journal modes, unknown PRAGMAs and compound statements fail closed.
Product SQL, retry, timeout, error and transaction behavior are unchanged.

The overlay adds ProcessStart/ProcessEnd around main and observes os.Exit.
Without AGENTDECK_P07_BARRIER_DIR these are no-ops. With a private barrier
directory, a helper waits before main and after cleanup until the collector
acknowledges a complete fixed 50 ms frame containing its pinned PID/birth.
These waits are outside measured SQL intervals. A 30 second missing observer
timeout aborts the process. The engine root must stay alive through the
collector final-frame acknowledgement. No shell sleep children are used.
All frames remain in the trace; missing endpoints, incomplete frames, missed
ticks or an unfinished window invalidate it. Report barrier overhead separately
from measured SQL and the sampled aggregate maximum; neither is acceptance.
