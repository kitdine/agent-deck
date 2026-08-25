# Beads Agent Coordination

Read this file only when a task requires Beads coordination. Repository plans,
contracts, review records, status documents, and evidence remain authoritative
for their respective concerns.

## Local deployment

The user-level Beads deployment coordinates Codex and Claude Code without
placing `.beads`, hooks, generated instructions, or database files in this
repository.

- State root: `/Users/jobshen/.local/state/agentdeck-beads`
- Required CLI wrapper:
  `/Users/jobshen/.local/state/agentdeck-beads/bin/agentdeck-bd`
- Human UI: `http://127.0.0.1:13308`
- Services: `com.kitdine.agentdeck.beads-dolt` and
  `com.kitdine.agentdeck.beads-ui`
- The Dolt and UI listeners must remain bound to `127.0.0.1`; do not expose
  them to LAN or public interfaces.

Every Beads CLI invocation must identify its actor. Use `codex` for Codex and
`claude-code` for Claude Code. The wrapper rejects an omitted actor:

```bash
env BEADS_ACTOR=codex /Users/jobshen/.local/state/agentdeck-beads/bin/agentdeck-bd ready --label agent-task --json
env BEADS_ACTOR=claude-code /Users/jobshen/.local/state/agentdeck-beads/bin/agentdeck-bd ready --label agent-task --json
```

`ready` covers work waiting to START. It cannot see a custom status, so work
waiting for a REVIEWER needs `list --status in_review`; see One lifecycle.

## Automatic backup warnings

Mutating `agentdeck-bd` commands may complete their primary database write and
then report `auto-backup failed`. The backup is an internal `bd` side effect,
not a separate Beads workflow action and not a new user-authorization boundary.

When this warning appears:

1. Read back the exact primary mutation: task status, assignee, dependency, or
   comment.
2. If the read-back matches, treat the primary operation as successful and
   report the backup warning once as a non-blocking operational risk.
3. Do not retry the mutation, change permissions, request sandbox escalation,
   or describe extra user authorization as necessary merely to silence the
   warning.
4. Escalate only when successful backup creation is itself an explicit task
   requirement or the primary mutation did not persist.

Keep the three authorities distinct in explanations: the user's workflow
instruction authorizes the Beads mutation; the Codex sandbox determines which
paths a command can technically write; and `bd` owns its internal backup side
effect. Use wording such as `primary write succeeded; the internal backup was
unavailable and did not block this task`, never `the user must authorize the
automatic backup`.

## Scoped task procedure

1. Resolve the repository workflow route and read its authoritative plan and
   contract before consulting Beads.
2. Resolve the selected task from current Beads state, then read it with `show
   --json`, its blockers with `blocked --json` or `dep tree`, and its comments.
3. A phase command accepted by the project workflow authorizes resolving only
   the matching `Authorize <Phase>: <task-anchor>` human Gate, where `<Phase>`
   is `Design` or `Development` — review has no Gate, see One lifecycle below.
   Never resolve a later Gate, a different task's Gate, or a Gate based only on
   `bd ready`.
4. Atomically claim the exact resolved task ID with `bd update <id> --claim`.
   Do not use an unfiltered ready claim when the user named a task anchor.
5. **Frozen, pending upstream.** `bd heartbeat` does not exist in the installed
   `bd`; see Installed-version limits below. Until it returns, treat a claim as
   live only while the same actor is demonstrably working: before continuing
   after a long command or pause, re-read the task and stop if `assignee` is no
   longer you. A claim whose `updated_at` is older than the current working
   session, with no comment explaining why, is stale rather than active — say so
   in a comment instead of assuming either way.
6. Write concise durable handoff comments before releasing a claim or ending
   with unfinished work. Never put credentials, raw session content, private
   prompts, or sensitive paths in Beads.

Do not keep a version-specific task-ID map here. Resolve task IDs from the live
store by anchor, labels, dependencies, and comments.

## Document work is dispatched too

A topic reaches development through six document stages before any task exists;
see the progression in `docs/documentation-workflow.md`. Beads carried only
`Development:` and `Review:` pairs per task anchor, so that entire span was
invisible to dispatch —
during it a `Development:` task sat claimed and `in_progress` while nothing was
being implemented, which is the opposite of what dispatch should report.

Each document a topic's Documents matrix declares gets **one** task, whose
status walks its lifecycle:

```text
ad-<topic>-doc-<document>-design     文档：<topic> / <document>
```

One task, not a design/review pair. A document is one object and these are
stages of that object, so a second task duplicates it. The pair model also does
not survive contact with a real review: under the Repair and Re-review rule
below, one document that failed review three times would carry a design task, a
review task, three repair tasks and three re-review tasks — eight objects for
one file. `ux/menubar.md` reached eight rounds.

## One lifecycle

One lifecycle covers every task — a document, a task anchor, a test, anything
else. What differs between them is the work product (`.md`, code, a test run),
not the states it passes through, so the status model has no
document-versus-development split:

```text
open ──→ in_progress ──→ in_review ──→ awaiting_commit ──→ closed
              ↑______________│
                (review sends it back)
```

| Status | Kind | Category | Meaning |
| --- | --- | --- | --- |
| `open` | built-in | active | Not started |
| `in_progress` | built-in | wip | Being produced — document, code, or anything else |
| `in_review` | custom | active | Awaiting or under review |
| `awaiting_commit` | custom | wip | Review passed; awaiting the commit checkpoint |
| `closed` | built-in | done | Committed |

**Never set `blocked` by hand.** Express blocking as a dependency and let it be
derived: a hand-set `blocked` is a second, silent record of the same fact, and
the two diverge the moment the blocker closes — the dependency graph says the
task is workable while the status still says it is not, and nothing brings it
back. `deferred` parks work indefinitely and is set deliberately. `pinned`
exists for work that never closes; nothing in this workflow qualifies, since
every task here ends at `closed`.

The custom pair is registered once:

```bash
bd config set status.custom "in_review:active,awaiting_commit:wip"
```

**`bd ready` never returns a custom status.** Its query hardcodes
`status IN ('open', 'in_progress')` (`internal/storage/sqlbuild/ready.go`), so
category has no bearing on it, and `internal/types/types.go`'s comment that
"active statuses appear in bd ready" states an intent the query does not
implement. Verified against bd 1.2.2 in an isolated repository: two unblocked,
unassigned tasks in an `active` custom status, and `bd ready` returned nothing.

Dispatch a reviewer with `bd list --status in_review`. Using `bd ready` for it
silently finds no work forever, which looks identical to there being none.

What category does control is default `bd list` visibility: `active` and `wip`
are listed, `frozen` and `done` are hidden. Same isolated check — `in_progress`,
`in_review` and `qa_testing` listed; `closed`, `on_hold` and `pinned` absent.
Both custom statuses must stay visible there, so both must be `active` or `wip`,
and bd draws no behavioural distinction between the two. `in_review` as `active`
and `awaiting_commit` as `wip` is therefore a semantic label — one is waiting for
someone to pick it up, the other is a waiting state of work already done — plus
the board's column colour, which is derived from category.

**Entering review needs no authorization.** Work that is finished moves to
`in_review` in the same action that finishes it. There is no `Authorize Review`
Gate: such a gate only records that nobody has started reviewing yet, which is
what the status already says. `Authorize Design` and `Authorize Development`
Gates remain — when work *starts* is the user's decision.

**A Gate's description is written for the user, not for an agent.** Its only
reader is the person deciding, so it states what approving starts, what it
changes that is observable from outside, what it is based on, and what it
unblocks — with the consequential part first, since the board truncates. Machine
release conditions belong in a comment. A description reading "Resolve only
after X Review PASS and explicit user Development authorization" tells the
decider nothing they can act on, and on an authorization Gate it is circular:
the approval it demands is the very approval being asked for.

`closed` means the work is delivered, not that it was produced or that review
passed. A `PASS` moves the task to `awaiting_commit`; the authorized commit is
what closes it. Collapsing `awaiting_commit` into `closed` is exactly what makes
"review passed" and "delivered" indistinguishable in dispatch.

**This includes document tasks**, and the commit checkpoint for one is the
project's to emit. The Skill deliberately emits no commit recommendation over a
document, contract, or process target — nothing was implemented, so it has
nothing to recommend — but that is a statement about the Skill's output, not
about whether the work is delivered. A passing document leaves the document
itself, its review record, and the `tasks.md` / `docs/status.md` status
synchronization to be committed, and CEv1 binds an uncommitted review candidate
to HEAD plus a blob fingerprint that must be re-recorded against the immutable
Git tree once an authorized commit exists. `awaiting_commit` is precisely that
interval. Closing a document at `PASS` would drop both the commit and the
evidence re-record on the floor.

**A review verdict is not an evidence gate.** `PASS` with a required completion
gate still `NOT_VERIFIED`, `FAILED`, or `BLOCKED` does NOT reach
`awaiting_commit` or `closed`: the task stays `in_review` and a comment records
the verdict and the open gate. The status name reads oddly for a few hours, and
that is the correct trade — the alternative asserts a completion the evidence
does not support. Do not add a status for this; the gate is CEv1's to answer and
mirroring it here would make Beads a second, stale evidence record.

Nothing moves a status by itself. Each phase command owns exactly one
transition on the task it names, and performing the command without the
transition is what made an entire day of document work invisible to dispatch:

| Command | Transition | Also |
| --- | --- | --- |
| `设计：<topic>` | create the topic's document tasks at `open` | — |
| `设计：<topic> / <document>` | `open` → `in_progress`, → `in_review` when the draft is complete | claim it |
| `开发：<topic> / <task-anchor>` | `open` → `in_progress`, → `in_review` when the implementation is complete | claim it |
| `评审：<topic> / <subject>` | stays `in_review` | claim it as the reviewer; on `PASS` → `awaiting_commit`, on `REOPEN` → `in_progress` and increment `round-N` |
| `修复：<topic> / reviews/<record>.md / <ids>` | stays `in_progress`, → `in_review` when the repair is complete | comment the disposition |
| `复评：<topic> / reviews/<record>.md` | as for `评审` | — |
| commit checkpoint | `awaiting_commit` → `closed`, after the authorized commit | — |

Repair and re-review are transitions on this one task, never new tasks. A
`round-N` label counts how many times review sent it back; it increments on
every `REOPEN` and is never reset, so a task that keeps bouncing is visible as a
number rather than as a comment someone has to read.

Derive the status from the command being performed, not from the last verdict
word in a review record. `REOPEN` marks the round that returns work to `Dev`,
but a repair round appends its own closing line to the same record, and those
lines read `Verdict: REOPEN — repair complete, awaiting independent Re-review`:
still-not-PASS, yet the work is finished and waiting for a reviewer. Read
literally, the record's last `REOPEN` says `in_progress` for a task that is
correctly `in_review`.

The comment matters as much as the status: a status says where the work is, a
comment says why it moved. Write it in the same action, not afterwards —
"afterwards" is reliably never.

**The same holds for a task anchor.** `menubar-experience` is one unit of work
and development, review, repair, and re-review are its stages, exactly as they
are a document's. An earlier version of this file kept a `Development:` /
`Review:` pair there, justified as "implementation and review are separate
objects with separate claims" — which is not a reason for two tasks, because a
changing owner is what `assignee` is for. The pair was inherited from the bulk
import and the justification written afterwards. One task per anchor:

```text
ad-<...>-dev     任务：<task-anchor>
```

`<document>` is the matrix row flattened: `req`, `arch`, `tasks`,
`ux-<surface>`. Use topic-scoped IDs with no version segment, following the
`ad-clierr-*` precedent — a topic carries no version, and embedding one would
put version membership in a second place. A row marked `n/a` gets no task.

Ordering follows the review order, expressed as dependencies: every other
document task depends on the requirements document task, and the `tasks`
document task depends on every other one. A task anchor's task depends on the
`tasks` document task, which is what makes "the specification has not passed" a
dispatch fact rather than something a reader must infer.

**Create a topic's development tasks only after its `tasks.md` passes review.**
The task matrix is what defines which anchors exist, and before `PASS` that
matrix is a draft: anchors get renamed, merged, split, or dropped. Tasks created
from a draft therefore assert work that may never exist, and the Gates created
alongside them ask the user to authorize it — which is how nineteen objects
(ten development tasks and nine `Authorize Development` Gates) came to sit in
dispatch for topics whose requirements document had not yet passed round 4. They
were deleted rather than repositioned, because dependency edges cannot fix an
object that should not have been created: without a passed document there are no
development tasks to order.

The same rule read forwards: `设计：<topic>` creates document tasks only. The
first command that may create development tasks is the one that follows the
`tasks.md` task reaching `awaiting_commit`.

## State transitions and authority

Beads state transitions must preserve the project workflow boundaries. A Beads
task is a schedulable coordination record; its lifecycle does not own
requirements, phase state, review verdicts, or evidence.

Apply repository workflow and CEv1 transitions in their own authoritative
stores first, then close the corresponding Beads task last as a derived
coordination projection. Beads closure adds no completion gate.

- Move a task to `in_review` only after the owning plan's `Dev` field is
  synchronized and every completion gate required for that transition is
  satisfied. Reaching `in_review` grants nothing: it reports that the work
  product exists and needs a reviewer.
- Move a task to `awaiting_commit` only after the latest applicable independent review
  record says `Verdict: PASS`, the owning plan's `Review` field is
  synchronized, and every required completion gate is satisfied.
- If review finds a blocker, the task does not reach `awaiting_commit`. Record the
  finding, release the reviewer's claim, move it back to `in_progress`, and
  increment `round-N`. Do not create Repair or Re-review tasks — there is one
  object, and repair and re-review are transitions on it.
- Close a task only after its authorized commit exists. `closed` is a
  delivery fact, not a review verdict.
- A Beads task and a CEv1 WorkUnit need not map one-to-one. A concise handoff
  comment may link stable task, WorkUnit, content-state, and evidence
  identifiers; do not copy criteria, raw evidence, test output, review
  verdicts, or CEv1 status into Beads.
- Beads dependency, claim, heartbeat, comment, handoff, and closure changes do
  not trigger CEv1 discovery, queries, invalidation, or upserts. CEv1 results
  do not mutate Beads.
- If Beads and authoritative repository state disagree, stop dispatch, inspect
  the exact repository state and Beads history, then reconcile only within the
  currently authorized workflow phase. Do not silently choose either side.
- **Frozen, pending upstream.** `bd reclaim` does not exist in the installed
  `bd`. A claim believed abandoned is taken over by re-reading the task,
  recording in a comment why the previous actor is judged not to be working,
  and then claiming it. When `reclaim` returns, never use `--any-replica` in
  this single-server deployment.

## Installed-version limits

`bd` v1.2.2 is the v1.1.2 code re-released under a higher version number. v1.2.0
and v1.2.1 were published accidentally on 2026-08-11 without release testing,
and running v1.2.1 once migrated this database from schema v53 to v65; the
cursor was rolled back to v53 on 2026-08-16 following the upstream runbook, and
the 12 migrations' additive tables remain in the database, unused.

The 1.2.x-only features are therefore absent: **work leases, `bd heartbeat`,
`bd reclaim`, the events journal, sync federation, the HTTP API server, and
provenance events.** Upstream states they return in a properly tested release.

Two consequences bind dispatch today. There is no lease, so two agents can hold
a claim simultaneously if one dies without releasing it — the comment discipline
above is the only guard. And a `bd` that predates the migrations will re-migrate
this database silently, so no v1.2.1 binary may touch it.

Clauses marked **Frozen, pending upstream** are unfrozen when a release restores
the command, not rewritten around it.

Bead Me Up, Scotty is a write-capable convenience UI, not a workflow authority.
Use it for visibility, comments, dependency inspection, and deliberate human
actions. Do not drag official phase tasks across columns, close them, delete
them, or approve their Gates as a substitute for an explicit workflow command.
Its human actor is `jobshen-human`; Agent writes must retain their own actor.

Beads does not authorize repository or external delivery actions. Commit, push,
tag, release, publication, PR creation, branch/worktree creation, installation,
and deployment retain their existing explicit authorization boundaries.
