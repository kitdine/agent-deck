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

## Scoped task procedure

1. Resolve the repository workflow route and read its authoritative plan and
   contract before consulting Beads.
2. Resolve the selected task from current Beads state, then read it with `show
   --json`, its blockers with `blocked --json` or `dep tree`, and its comments.
3. A phase command accepted by the project workflow authorizes resolving only
   the matching `Authorize <Phase>: <task-anchor>` human Gate. Never resolve a
   later Gate, a different task's Gate, or a Gate based only on `bd ready`.
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
see the progression in `docs/README.md`. Beads carried only `Development:` and
`Review:` pairs per task anchor, so that entire span was invisible to dispatch —
during it a `Development:` task sat claimed and `in_progress` while nothing was
being implemented, which is the opposite of what dispatch should report.

Each document a topic's Documents matrix declares gets **one** task, whose
status walks its lifecycle:

```text
ad-<topic>-doc-<document>-design     文档：<topic> / <document>

open -> drafting -> in_review -> repairing -> in_review -> closed
```

One task, not a design/review pair. A document is one object and these are
stages of that object, so a second task duplicates it. The pair model also does
not survive contact with a real review: under the Repair and Re-review rule
below, one document that failed review three times would carry a design task, a
review task, three repair tasks and three re-review tasks — eight objects for
one file. `ux/menubar.md` reached eight rounds.

`drafting`, `in_review`, and `repairing` are custom statuses, registered once
with `bd config set status.custom "drafting,in_review,repairing"`. The built-in
set is `open, in_progress, blocked, deferred, closed, pinned, hooked`.

`closed` means the latest applicable review round is `Verdict: PASS` — not that
the document was written. A drafted but unreviewed document is `open`; a
document whose repair is complete and awaiting an independent re-review is
`in_review`.

A `round-N` label counts how many times review sent the document back. It
increments on every `REOPEN` and is never reset, so a document that keeps
bouncing is visible as a number rather than as a comment someone has to read.
Repair and Re-review are therefore status transitions on this one task rather
than new tasks; the rule below applies unchanged to task reviews, where the
implementation and its review are genuinely separate objects.

`<document>` is the matrix row flattened: `req`, `arch`, `tasks`,
`ux-<surface>`. Use topic-scoped IDs with no version segment, following the
`ad-clierr-*` precedent — a topic carries no version, and embedding one would
put version membership in a second place. A row marked `n/a` gets no task.

Ordering follows the review order, expressed as dependencies: every other
document task depends on the requirements document task, and the `tasks`
document task depends on every other one. A task anchor's `Development:` task
depends on the `tasks` document task, which is what makes "the specification has
not passed" a dispatch fact rather than something a reader must infer.

## State transitions and authority

Beads state transitions must preserve the project workflow boundaries. A Beads
task is a schedulable coordination record; its lifecycle does not own
requirements, phase state, review verdicts, or evidence.

Apply repository workflow and CEv1 transitions in their own authoritative
stores first, then close the corresponding Beads task last as a derived
coordination projection. Beads closure adds no completion gate.

- Close a Development coordination task only after the owning plan's `Dev`
  field is synchronized and every completion gate required for that transition
  is satisfied. Closing it does not grant Review.
- Close a Review coordination task only after the latest applicable independent
  review record says `Verdict: PASS`, the owning plan's `Review` field is
  synchronized, and every required completion gate is satisfied.
- If Review finds a blocker, do not close the Review task. Record the finding,
  release its claim, and create bounded Repair and Re-review tasks and Gates
  linked back to that Review task. The original Review task remains the
  downstream coordination gate and closes only after successful Re-review.
  This applies to **task** reviews, where the implementation and its review are
  separate objects with separate claims. A document task instead moves to
  `repairing` and increments its `round-N` label, because there is only one
  object; see `Document work is dispatched too`.
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
