## Beads Agent Coordination

Read this file when the active request requires Beads coordination. Beads owns
dispatch, dependency readiness, assignment, and handoff. Repository documents own
requirements and review verdicts; CEv1 owns evidence for defined content states.
Do not infer authority or completion in one system from another system's label.

## Local deployment

Use the operator's existing Beads deployment. The current installation's wrapper
is `$HOME/.local/state/agentdeck-beads/bin/agentdeck-bd`; its state lives outside
this repository. If that binding is unavailable, resolve the operator's actual
installation rather than guessing from old Hook output, provisioning a new store,
or changing services as part of an ordinary task.

Every agent read or write must identify its actor. Use `codex` or `claude-code`;
do not substitute the human operator or fabricate an actor identity.

```bash
beads_cli="$HOME/.local/state/agentdeck-beads/bin/agentdeck-bd"
env BEADS_ACTOR=codex "$beads_cli" ready --label agent-task --json
env BEADS_ACTOR=claude-code "$beads_cli" list --status in_review --json
```

Use the actor-qualified wrapper for all command examples below. Subcommand names
in prose are not permission to use bare `bd`. Keep any existing Dolt/UI listeners
loopback-bound; do not expose them or change network configuration to make a task
or check succeed.

## Automatic backup warnings

A mutation may persist successfully and then report `auto-backup failed`.
Treat the primary write and its backup side effect separately:

1. Read back the exact intended task change or comment.
2. If it persisted, report the backup warning once without replaying the write,
   changing permissions, or requesting approval merely to silence the warning.
3. If it did not persist, diagnose that failure before retrying. Escalate backup
   work only when successful backup is an explicit requirement.

User authorization, sandbox write access, and the backup implementation are
different concerns. A backup warning does not revoke an already authorized phase
or prove that its primary operation failed.

## Scoped task procedure

1. Resolve the active user scope, applicable workflow, and authoritative work
   product before consulting dispatch. An advisory read-only audit does not become
   a formal phase or a task claim because a Hook mentions another dirty path.
2. Find the exact existing task by current subject, anchor, labels, and relations.
   Read its details, relevant dependencies, and comments. Do not claim an arbitrary
   ready item or create a duplicate because a remembered ID is absent.
3. A real user stage command may resolve its matching Design or Development
   authorization Gate under `AGENTS.md`. Do not resolve another task's Gate, a
   later stage's Gate, or infer phase authority from `ready` or Gate status alone.
4. Resolve ownership before claiming. Use the supported atomic claim operation
   for an available task; an existing assignment requires a verified handoff or
   separately authorized takeover. Do not overwrite an unknown active owner with
   `--assignee` merely because claiming failed.
5. Preserve the phase's intended status when claiming or transferring ownership.
   The current CLI documents that `--claim` sets `in_progress`; do not leave a
   review task there simply because the ownership operation succeeded. Complete
   the authorized status adjustment and verify assignee and status before doing
   the phase's work. Do not assume claim/status options compose atomically unless
   the installed behavior has been verified.
6. After a pause that makes ownership uncertain, recheck the exact task. An actor
   name identifies a client role, not a unique session; another Codex or Claude
   session can use the same actor. Use handoff context and a session/correlation
   reference when necessary to distinguish active work. An old `updated_at` is a
   reason to investigate, not proof that an owner is dead or a claim is free.
7. Record a concise handoff when leaving unfinished work or transferring ownership.
   Do not record credentials, private prompts, raw session content, or sensitive
   paths. Preserve enough context for the next actor to locate authoritative work.

Do not maintain a static task-ID map. If a coordination step only partly succeeds,
read back the result and complete the missing part; do not repeat successful
comment creation or describe several CLI calls as one atomic transaction.

## Workspace handoff

When work occurs outside the canonical `main` workspace, task handoff records
reference the verified workflow workspace ID and branch. Resolve the local path
through that binding and the live Git worktree inventory; do not maintain a
second independently updated absolute-path registry in Beads.

Workspace entry does not claim a product task or change its lifecycle. Claim and
handoff the task only when its authorized phase starts. Multiple sessions using
the same actor or workspace remain distinct; record a session correlation and
bounded collaboration scope when ambiguity matters. A workspace binding is not
a lease or exclusive ownership claim.

Beads is the cross-worktree dispatch view. Follow
[Status ownership across worktrees](../docs/documentation-workflow.md#status-ownership-across-worktrees);
a task handoff does not require a parallel global-status update or `main` commit.

## Document work is dispatched too

Each applicable document declared by a topic's Documents matrix has one task;
design, review, repair, and re-review are stages of that object. Do not create a
new task for each round. Follow [Documentation Workflow](../docs/documentation-workflow.md)
for artifact dependencies, including framework and final-surface review scope.

The established title forms are:

```text
文档：<topic> / <document>
任务：<task-anchor>
缺陷：<one-line observed symptom>
```

These titles identify different work products and are consumed by project
coordination tooling. Preserve their grammar. Resolve actual IDs from live state;
examples such as `ad-<topic>-doc-<document>-design`, `ad-<...>-dev`, and
`ad-bug-<slug>` illustrate naming, not lookup or creation authority.

Use stable topic-scoped IDs without release-version segments. Flatten document
paths consistently (`req`, `arch`, `tasks`, `ux-<surface>`) according to the existing
project mapping. An `n/a` document row gets no task.

Create document tasks when the topic's document set is declared. Create
implementation tasks only after the Tasks matrix is approved and the applicable
evidence/checkpoint conditions are satisfied. A draft decomposition may rename,
split, or remove anchors; it must not populate dispatch as if already approved.

Express prerequisite relationships from the authoritative progression. Document
tasks may exist before implementation decomposition; do not say that no task
exists during design. Keep one task per implementation anchor, not a separate
Development/Review pair.

## One lifecycle

Work-product tasks use this coordination lifecycle:

```text
open → in_progress → in_review → awaiting_commit → closed
           ↑______________|
             failed review
```

| Status | Category | Meaning for a work-product task |
| --- | --- | --- |
| `open` | active | Not started |
| `in_progress` | wip | Being produced or repaired |
| `in_review` | active | Awaiting or under review; also used while a passed review's required evidence gate remains open |
| `awaiting_commit` | wip | Review and applicable required gates passed; waiting for authorized delivery |
| `closed` | done | The task's authorized delivery boundary has been satisfied |

Authorization Gates and administrative disposition records are not product
implementation tasks. A Gate closes on the corresponding authorization decision;
that closure does not claim that code was committed or start a new phase.
Do not fabricate a product commit to justify closing a Gate or an explicitly
superseded coordination record.

Use dependencies for blockers rather than manually duplicating them in a
`blocked` status. `deferred` deliberately parks work. Do not repurpose `pinned`
as another workflow phase. Task descriptions state the work product and point at
this lifecycle; they do not copy status sequences or obsolete vocabulary.

For reviewer dispatch, query `list --status in_review` explicitly instead of
assuming `ready` includes the custom review queue. Use `ready` for work waiting
to start. The current custom configuration is
`in_review:active,awaiting_commit:wip`; verify it when setup or diagnostics require
it, not on every task. Reconfiguring statuses is a separately scoped setup action.

Moving finished work to `in_review` is a required handoff within its authorized
production stage and does not need another Review-authorization Gate. Actually
performing a review still requires its own real user command or an already
confirmed automatic workflow. Do not confuse dispatch readiness with authority
to run the next phase.

Design and Development authorization Gates remain user decision points. Create
one with the supported `create --id <name>-gate -t gate` subcommand and an ordinary
blocking dependency under the existing convention, rather than substituting the
async `gate create` mechanism. A Gate's human-facing description explains what
approval starts and unblocks; machine release details belong in a concise comment.
Do not infer that an evaluator closing a Gate grants broader business authority.

For a work-product task, review PASS alone does not mean delivered. Its authorized
commit must exist before it closes. A required NOT_VERIFIED, FAILED, or BLOCKED
evidence gate keeps the task in `in_review`.
When review and required gates pass, follow the Skill's Task checkpoint and move
to `awaiting_commit`. Every work-product task gets that checkpoint, including tasks whose
work product is a document; recommendations do not authorize commit or push.

Use a short coordination comment to explain the transition and link the applicable
review round, WorkUnit, and content identity. Such a comment is a historical
handoff pointer, not another maintained copy of findings, criteria, test output,
or current review/evidence status. The referenced authorities remain decisive.

New review records use PASS/FAIL; historical REOPEN records remain unchanged.
A `round-N` label counts review returns on that task, not the global review-round
number across different records. Never merge record histories just because their
round numbers or actors match. Preserve unrelated labels when changing round labels.

| Authorized work | Coordination result |
| --- | --- |
| Topic/document design | Create only justified document tasks; produce the subject at `in_progress`, then hand it to `in_review` when its declared stage is ready |
| Implementation | Claim the approved task; `in_progress` until its implementation and proportionate checks are ready for review |
| Review or re-review | Preserve `in_review` while reviewing; FAIL returns it to `in_progress`; PASS reaches `awaiting_commit` only after applicable required gates pass |
| Repair | Keep the same task; hand completed repair to `in_review` without self-issuing a review PASS |
| Authorized commit/delivery | Inspect the actual delivered boundary, then close only the matched task whose obligations are satisfied |

These are project coordination rules, not a second phase-command parser. The
Skill and `AGENTS.md` own command matching, authorization ceilings, and completion
receipts. A claim or a silent Hook never proves that a phase is complete.

### Commit-checkpoint contributor attribution

For an authorized commit, use durable task history to identify material
contributors to the exact staged scope:

1. Identify only the included work-product tasks at their delivery checkpoint.
   Do not scan unrelated tasks or infer authorship from the topic epic.
2. Read those tasks and their comments. Include supported Design, Development,
   Repair, or content-producing handoff contributions present in the staged
   files/hunks. Exclude review-only, dispatch-only, or evidence-query activity
   unless it also produced committed content.
3. Treat assignee and timestamps as supporting evidence, not complete authorship.
   Union material contributors across the included scope and avoid duplicate
   identities. Do not invent a human or model-specific identity from an actor name.
4. Before committing, show the existing checkpoint block:

   ```text
   Commit checkpoint contributors
   staged_tasks: <task ids>
   included: <actor> — <role> — <comment id or timestamp>
   excluded: <actor> — <reason> — <comment id or timestamp>
   trailers:
   Co-Authored-By: <established identity>
   unresolved: <none, or actor and missing evidence>
   ```

5. Use the established actor identities where applicable:

   ```text
   codex       -> Co-Authored-By: Codex <noreply@openai.com>
   claude-code -> Co-Authored-By: Claude <noreply@anthropic.com>
   ```

   Preserve supported historical attribution. Respect higher-priority runtime
   instructions; do not claim this repository file overrides them. If applicable
   identity requirements conflict, report their exact sources before committing
   rather than guessing or misattributing a contribution.
6. If material attribution is unresolved, stop before the commit and name the
   missing evidence. A current assignee is not a substitute for that evidence.

[Project Rules](project-rules.md) owns the mandatory Codex trailer, full commit
message, staged-scope, SSH-signature, and history-rewrite requirements. This check
does not add delivery authority. Close matched tasks only after the authorized
commit object and remaining delivery obligations have been verified.

## Bug lane

Propose the lane from the required decision, not diff size:

| Required work | Lane | Treatment |
| --- | --- | --- |
| Restore an existing contract without deciding new user-visible behavior | A | Bounded repair with a fix record and independent review |
| Decide a new code, state, output shape, or precedence rule | B | Feature topic with the applicable design/review progression |
| Defer the decision/work | C | Park the bug and record the planning candidate |

The user confirms a new lane choice. Reuse an already selected lane in the active
instruction or approved plan instead of asking again. Do not choose the cheaper
lane solely to reduce process, and do not keep using Lane A when the repair
reveals a new product or contract decision.

### Lane A in Beads

Use one `bug` task and one `docs/fixes/<slug>.md` carrier under Documentation
Workflow. Keep its ordinary work-product lifecycle and Development authorization
Gate; there is no design Gate because no new design is being approved.

Independent review is required. Its full report belongs in the fix record, not
only in a Beads comment. [Evidence](evidence.md) defines its task boundary as
`fix:<slug>` with no containing topic gate; absence of a topic does not waive
that task's required evidence gate.

The established project scopes remain:

```text
开发：fix / <slug>
评审：fix / <slug>
修复：fix / <slug> / <finding ids>
复评：fix / <slug>
```

These examples name the work product; the shared Skill still defines command
matching and stage behavior.

### Lane B and Lane C in Beads

Keep a Lane B bug as the origin record and create the topic's document tasks
under the normal convention. Link the origin from `requirements.md`.

Document/decomposition dependencies are planning prerequisites, not proof that
the defect was repaired. Once approved decomposition identifies the actual fix,
link the origin bug to its fix-delivery tasks or the applicable topic delivery
boundary. Do not close the bug because requirements or `tasks.md` were approved
or committed; close it only when the work that remedies the defect is delivered
under its required review/evidence and authorization conditions.

Lane C uses `deferred` and a Backlog candidate in `docs/roadmap.md`, with no Gate
for work that is not starting. Re-triage when it is promoted, using the current
contract and the user's decision; do not create a replacement bug merely to
restart its history.

## State transitions and authority

Apply repository and evidence obligations in their own stores; derive the task's
coordination transition afterward. Do not mirror a complete state machine or
criterion set into Beads, and do not create a CEv1 gate for a Beads-only change.

- Before `in_review`, verify the subject's actual readiness: document Draft,
  implementation Dev, or the standalone fix's readiness contract as applicable.
  Only gates required for that handoff apply; do not require a future review PASS
  as a prerequisite for dispatching its reviewer.
- Before `awaiting_commit`, verify the latest applicable independent PASS,
  corresponding Review state, and required evidence gates for the exact subject.
- A failed review returns the same task to repair. Release/transfer the reviewer's
  ownership through the supported handoff operation and increment the task's
  return counter; do not create separate Repair or Re-review tasks.
- Before closing a work-product task, verify its actual authorized delivery.
  Closing a Gate or administratively disposing of an erroneous record follows
  its own explicit decision and is not a product completion claim.
- Beads-only dependencies, assignment, comments, and handoff changes do not
  invalidate or query CEv1. A CEv1 result does not itself mutate Beads; the active
  workflow performs any required coordination transition.
- If a Hook or task record disagrees with repository authority, inspect only the
  exact relevant subject and history. Reconcile a confirmed in-scope mismatch
  under the active authority; do not claim another actor's unrelated work from
  a dirty-path signal or change a verdict to silence a warning.
- An ownership conflict or unavailable capability is a named coordination
  blocker. Do not infer a valid lease from a timestamp or repeatedly retry an
  unsupported claim/reclaim form.

## Installed-version limits

A read-only inspection on 2026-09-07 reported bd 1.2.2 and the custom status
configuration above. Its help documents `--claim` as setting the current actor
and `in_progress`, with same-actor idempotence; `heartbeat` and `reclaim` were not
listed as top-level commands. Help output is a capability clue, not an isolated
behavior test or proof of a lease contract.

Verify version-specific operations when they are needed or fail; do not poll
version/configuration on every turn or automatically change the deployment.
Until a supported ownership/lease mechanism is verified for this installation,
use explicit handoff and ownership checks. Do not invoke absent commands or use
a different binary against the live store merely to obtain newer features.

The historical migration incident and earlier deployment limits are retained in
commit `36e4b0e87f0ac0eda603e022afd52db26517a043`, file
`.agent-instructions/beads.md`. They are provenance, not a blanket claim about
all future versions. In particular, do not use the historically rejected v1.2.1
binary on this live store or perform migrations as incidental task coordination.

The convenience UI does not own workflow authority. Human board actions may
coordinate work when explicitly intended, but do not replace phase authorization,
review, evidence, or commit verification. Agent writes retain their own actor,
not the UI's human identity.

Beads grants no Git, release, installation, or deployment authority. Those actions
remain under the user's explicit request and the applicable project rules.
