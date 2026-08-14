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
5. Keep a live claim healthy with `bd heartbeat <id>` at least every two
   minutes while working. Before continuing after a long command or pause,
   re-read the task and stop if the claim is no longer owned by the same actor.
6. Write concise durable handoff comments before releasing a claim or ending
   with unfinished work. Never put credentials, raw session content, private
   prompts, or sensitive paths in Beads.

Do not keep a version-specific task-ID map here. Resolve task IDs from the live
store by anchor, labels, dependencies, and comments.

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
- `bd reclaim` may recover a genuinely expired local lease after checking that
  the previous Agent is no longer working. Never use `--any-replica` in this
  single-server deployment.

Bead Me Up, Scotty is a write-capable convenience UI, not a workflow authority.
Use it for visibility, comments, dependency inspection, and deliberate human
actions. Do not drag official phase tasks across columns, close them, delete
them, or approve their Gates as a substitute for an explicit workflow command.
Its human actor is `jobshen-human`; Agent writes must retain their own actor.

Beads does not authorize repository or external delivery actions. Commit, push,
tag, release, publication, PR creation, branch/worktree creation, installation,
and deployment retain their existing explicit authorization boundaries.
