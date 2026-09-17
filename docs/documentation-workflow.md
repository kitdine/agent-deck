---
status: active
updated: 2026-09-08
---

# Documentation Workflow

This document owns documentation naming, topic structure, lifecycle, readiness
matrices, size policy, and status vocabulary. The shared `development-workflow`
Skill owns phase commands, review execution and presentation, completion
receipts, and next instructions. This document supplies project-specific
prerequisites and artifacts without defining a second phase protocol.

Use [Project Status](status.md) for integrated project and release state,
[Roadmap](roadmap.md) for later planning and backlog, and [Documentation](README.md)
for stable navigation. [Review Records](../.agent-instructions/review-records.md)
owns record locations and metadata; [Evidence](../.agent-instructions/evidence.md)
owns exact-content-state gates; [Beads](../.agent-instructions/beads.md) owns
coordination. A review verdict, evidence result, and dispatch status are distinct.

## Naming Convention

- Use lowercase kebab-case feature-topic names without date or version prefixes.
  Version-contract topics are the explicit exception: `vX-Y-Z-contract`.
- Keep each feature topic under `docs/topics/<topic>/` using the structure below.
  Use the record naming rules in Review Records instead of inventing another
  review directory or naming scheme.
- `docs/specs/` holds guaranteed product contracts, not unfinished designs.
  Existing living authorities such as `cli-design.md` and `cli-manual.md` retain
  their stable names.
- Keep a follow-up inside its unfinished topic when its goal and acceptance
  boundary remain the same. Use a dated `## Follow-Up — YYYY-MM-DD` subsection
  when useful. A distinct goal or acceptance boundary becomes a separate topic.
- Keep topic-local candidates in that topic's Backlog or Future Feature Ideas;
  repository-wide candidates belong in `docs/roadmap.md`.

Use frontmatter appropriate to the document; do not invent historical dates:

```yaml
---
status: active | reference | historical
created: YYYY-MM-DD
updated: YYYY-MM-DD   # material change to a living document
retired: YYYY-MM-DD   # archived documents only
version: N            # versioned specifications only
---
```

## Document Lifecycle

| Directory | Purpose | Lifecycle |
| --- | --- | --- |
| `docs/topics/<topic>/` | One coherent behavior change | Keep active through its required reviews, evidence gates, contract reconciliation, and applicable delivery boundary; retire the topic as a whole. |
| `docs/fixes/` | One Lane A repair and its review history | Keep active while open; retire under Fix records after its authorized commit and required evidence finalization. |
| `docs/specs/` | Guaranteed product contracts | Maintain in place; reconcile stable topic contracts at the topic's closure boundary. |
| `docs/archive/` | Historical topics and supporting records | Preserve history; consult it for provenance, not as the default current authority. |
| `docs/README.md` | Stable navigation | Update for documentation topology or authority changes. |
| `docs/status.md` | Integrated project and release state | Update at the authorized integration/version-assembly boundary; do not mirror unmerged topic task progress. |
| `docs/roadmap.md` | Later direction, backlog, and withdrawals | Update planning decisions; do not mirror review rounds or execution detail. |

### Status ownership across worktrees

An unmerged topic has one execution-status authority: its own `tasks.md` in
the selected worktree. Full findings and verdicts belong to its review records;
Beads carries dispatch and cross-worktree handoff; CEv1 binds evidence to exact
content states. These responsibilities do not require a second progress log.

During topic design, implementation, review, repair, acceptance, or task delivery:

- Update only the topic's applicable matrix, review record, evidence and dispatch.
- Do not update `docs/status.md` in either the feature worktree or the `main`
  checkout merely because a task starts, passes, is committed, or is handed off.
- Do not create a companion `main` status commit or a status-only PR for that
  task. A Task checkpoint covers its owning worktree and does not imply those
  extra delivery actions.

`docs/status.md` describes the integrated project/release baseline. Reconcile
its summary and pointers once in the authorized topic-integration or version-
assembly change, with the relevant product state. Explicit project planning,
governance, or release changes may update their own global authorities, but are
not implied by a topic phase or a handoff reminder.

A feature worktree inherits global documents from its base. Their older topic
summary is not a stale-handoff defect: consult the topic matrix and Beads for
unmerged work. Preserve historical review/evidence facts; a later policy decision
can supersede their pending delivery advice without rewriting the old rounds.

### Topic structure

A feature topic owns one coherent behavior change. Active version membership is
owned by the version-contract topic and projected in `docs/status.md`; later
version direction and unscheduled planning belong in `docs/roadmap.md`.
See [Branching](../.agent-instructions/branching.md). Changing a proposed release
assignment does not rename the feature topic or invalidate its review by itself.

```text
docs/topics/<topic>/
  requirements.md        goals, non-goals, acceptance boundary
  ux/<surface>.md        interaction design for each applicable surface
  architecture.md        contracts and development design
  tasks.md               Documents and Tasks matrices
  reviews/<name>.md      review history for each subject
```

#### Why these documents, and how many

A document earns its own review when it answers a distinct question against
specific evidence. Combine documents with the same question and evidence; split
only when genuinely independent subjects require different judgments.

| Document | Review question | Evidence |
| --- | --- | --- |
| `requirements.md` | Is the boundary decided? | Goals, non-goals, acceptance, and supported premises |
| `ux/<surface>.md` | Is the declared design stage complete for every relevant state? | Copy, presentation rules, data requirements, and rendered specimens |
| `architecture.md` | Are contracts specified and current-code claims verified? | Contract definitions, source locations, and field provisioning decisions |
| `tasks.md` | Does the decomposition cover the approved documents without gaps or added scope? | The document set, task scopes, and verification levels |

Use one requirements document and one task matrix per feature topic, one UX
document per applicable surface, and one architecture document unless independent
contract domains justify a split in `tasks.md`. Explicitly mark non-applicable
document kinds in the Documents matrix rather than silently omitting them.

A version-contract topic uses `tasks.md` to select and reconcile feature topics.
It does not originate feature requirements, UX, or architecture; route new
product decisions to their owning feature topic.

#### The specimen requirement

A UX document includes rendered specimens of the states it defines and the
rules needed to evaluate them. For product surfaces, use the shared
[Product Prototype](../prototype/README.md) as the design authority; cite its
relevant surface and state instead of creating a competing prototype or an
unrelated hand-drawn substitute.

State what each specimen proves. Terminal specimens can closely represent
character output. GUI specimens establish hierarchy, copy, state coverage, and
narrow-bound wrapping or truncation; they do not establish native typography,
Dynamic Type, accessibility order, or runtime correctness. Keep the applicable
manual acceptance requirements in the task scope.

If a review depends on a specimen, include its content identity in the reviewed
state as required by Evidence. A document-only fingerprint cannot establish the
state of an independently changing specimen.

#### When the set is decided, and by whom

The Documents matrix in `tasks.md` is the sole declaration of the topic's document
set. The author proposes it; the `tasks.md` reviewer ratifies it against the
requirements, surfaces, contracts, and implementation scope.

- Declare required but unwritten documents with Draft unchecked. Do not create
  empty files merely to fill rows.
- Mark non-applicable kinds explicitly. Update the declared set when a newly
  identified surface or contract domain changes it.
- Changing the set returns `tasks.md` to review. Other documents retain their
  verdicts only if the change does not invalidate their content or assumptions;
  assess affected subjects instead of resetting every review mechanically.
- Authoring a newly required document is separate design work under the applicable
  phase authorization; a matrix row does not authorize implementation of it.

Run `bash scripts/check-topic-docs.sh` when reviewing `tasks.md`. The checker
compares declared rows, on-disk topic documents, and local `ux/<surface>.md`
references across topic-root and UX Markdown files. It excludes review and
prototype files from the on-disk document inventory and excludes recognized
cross-topic surface references. It also flags files shorter than ten lines as
possible stubs. These are structural checks, not proof of semantic completeness.

The checker scans all active topics and currently has no topic-selection flag.
Attribute its findings to their topics; report unrelated gaps without repairing
or claiming unrelated work. Missing drafts are expected during design but must
be resolved before the document set can pass review. Do not add filler to evade
a checker result or silently waive a required check; report any mismatch between
a valid lifecycle case and the checker for scoped resolution.

Keep this check in the documentation workflow, not in product build or release
aggregates. Its exit codes are 0 for clean, 1 for gaps, and 2 for harness failure.

#### Creating a topic

Record the topic's origin in `requirements.md`: a narrowed roadmap theme, backlog
candidate, another review's finding, a measured defect requiring a new contract,
or a direct user request. A repair that restores an existing contract is Lane A
and follows Fix records instead of creating a topic.

Use `设计：<topic>` for a new topic. The Skill's initialization command adapts
project guidance; it does not create a product topic. Follow its current command
matching rules, without duplicating the parser specification here.

Start from a concrete problem or user-requested outcome. For a defect, record
the observed behavior and how it was measured. For a new capability, record the
user's goal, the current limitation, and available supporting evidence; distinguish
unverified assumptions from observations. Do not invent runtime measurements for
behavior that does not exist. Define affected surfaces, contracts, exclusions,
and acceptance before approving the requirement boundary.

#### The progression from a topic to development

The table defines artifact dependencies, not new workflow phases. Its completion
column describes the local design or review result; use the Skill, Evidence, and
Beads authorities for mandatory synchronization, gates, and checkpoints.
A local PASS alone does not mean a task is committed or a topic is complete.

| Stage | Command | Produces | Finished when |
| --- | --- | --- | --- |
| 1. Boundary | `设计：<topic>` | `requirements.md` and `tasks.md` with the Documents matrix | Boundary and intended document set are declared |
| 2. Boundary review | `评审：<topic> / requirements.md` | Full review record | Current boundary passes review |
| 3. Surface framework | `设计：<topic> / ux/<surface>.md` | Layout, hierarchy, states, specimens, and data requirements | A reviewer can assess the framework and requested fields |
| 4. Framework review | `评审：<topic> / ux/<surface>.md` | Full review record identifying framework scope | The framework and its data requirements pass review |
| 5. Contract | `设计：<topic> / architecture.md` | Contracts provisioning each requested field or refusing it with a reason | Every requested field has a written disposition |
| 6. Contract review | `评审：<topic> / architecture.md` | Full review record | Current contract passes review |
| 7. Surface final | `设计：<topic> / ux/<surface>.md` | Surface reconciled with the settled contract | Dependencies, copy, states, and specimens agree with that contract |
| 7a. Final surface review, when required | `评审：<topic> / ux/<surface>.md` | Review of material changes, or documented reuse of valid evidence | The final surface has a valid review for the content used by decomposition |
| 8. Decomposition | `设计：<topic> / tasks.md` | Tasks matrix | Tasks cover the approved documents and name files and verification levels |
| 9. Decomposition review | `评审：<topic> / tasks.md` | Full review record and document-set check | Decomposition and document set pass review |
| 10. Development | `开发：<topic> / <task-anchor>` | Approved implementation and proportionate verification | Follow the Tasks matrix and the Skill's completion and review contracts |

A topic without a user-visible surface skips the surface stages. If final-surface
reconciliation changes reviewed behavior, copy, contracts, or specimens, clear
that surface's current Review approval and review the affected content before
decomposition. Do not carry a framework PASS over material final-surface changes.
If content and assumptions remain valid, document the impact assessment and reuse
applicable evidence instead of automatically adding a broad review round.

The Documents matrix exists from topic creation. Populate the implementation
Tasks matrix during Decomposition, after its inputs are ready. Document dispatch
tasks may already exist; they are not implementation task decomposition.

##### Why the surface leads the contract

The surface states what data it needs; architecture must provision each requested
field or refuse it with an explicit reason such as privacy, cost, or availability.
An existing omission is a constraint to evaluate, not sufficient justification
for rejecting the requirement. The final surface must reflect the settled answer.

Repair and re-review name the applicable review record and finding scope:

```text
修复：<topic> / reviews/<record>.md / <finding ids>
复评：<topic> / reviews/<record>.md
```

Map each record to its subject and its own history before choosing findings or
round numbers. The example scope does not create a new route or authorize a
later phase; use the Skill's matching and next-instruction contracts.

Implementation may update documentation required by its approved scope. It must
not silently invent an unresolved product or architecture decision. If a task
exposes an undeclared surface or contract, revise the document set and return the
new decision to its authorized design/review boundary.

#### Why the progression is ordered that way

Requirements define scope, the surface declares data needs, architecture settles
contracts, and decomposition covers the approved result. Readiness follows these
dependencies, not a fixed number of documents or repeated restatements of stage
numbers. Do not use a prior verdict after its relevant premises have changed.

### Fix records

A Lane A defect repair uses one `docs/fixes/<slug>.md` file containing the
observation, verified cause, repair boundary, verification, and review history.
[Beads](../.agent-instructions/beads.md) owns lane selection. This file's review
question is whether the change restores the existing contract and whether its
regression protection detects the original defect.

Retain the established fix-carrier layout:

```markdown
---
status: active
created: YYYY-MM-DD
---

# 缺陷：<one-line observed symptom>

## 现象
## 根因
## 修复边界
## 验证
## Review — Round N
```

This is a carrier example, not a report template. Append the full report using
the current Skill format and [Review Records](../.agent-instructions/review-records.md)
metadata, preserving earlier rounds and explicit finding dispositions.

A fix is ready for review when its record supports reproducing the observation,
locates the cause, defines the bounded repair, and provides a regression test that
fails without the fix. Preserve the project's existing failure-first requirement;
do not substitute an unsupported completion claim.

A failed review returns the fix to repair; only a subsequent review records its
new verdict. Completing the repair does not self-issue PASS. If a new product or
contract decision is necessary, re-triage to Lane B under its authority rather
than expanding the fix record into an unreviewed design.

Retire the delivered fix to `docs/archive/fixes/<slug>.md` after its authorized
commit and required evidence finalization. Set `status: historical` and `retired:`;
retain its own provenance and add no archive-index entry under the existing
fix-record convention. Do not rewrite older records to match a new format.

### Status

`tasks.md` owns the topic's document and implementation status. Preserve these
headings and matrix columns because project tooling consumes them:

```markdown
## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar.md | [x] | [ ] |

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `<anchor>` | [ ] | [ ] |
```

Draft asserts readiness for the document's declared design stage; Review reflects
its latest applicable review. For a staged UX document, identify framework or
final scope in the record. Before development, use the reviewed final scope,
not an earlier framework state. Do not add findings or round narratives to the
matrices.

| Document | Ready to review when |
| --- | --- |
| `requirements.md` | Goals, non-goals, acceptance, and supported premises are stated; no TBD remains; each surface is named by its local `ux/<surface>.md` path or declared absent; new contract scope is identified |
| `ux/<surface>.md` | The declared design stage covers relevant states, copy in shipped languages, rendered specimens, and each element's data requirements; final-stage content agrees with the settled contract |
| `architecture.md` | New contracts are specified, current-code claims are verified, and each requested field is provisioned or refused with a reason |
| `tasks.md` | The document set and task decomposition cover approved scope, with anchors, file boundaries, and proportionate verification levels |

`tasks.md` appears in its own Documents matrix. Its review checks the decomposition
and the completeness of the document set; it does not replace implementation
reviews. A required unresolved boundary decision prevents requirements approval.

Apply [Status ownership across worktrees](#status-ownership-across-worktrees)
before selecting a status target. Topic phase progress stays in its matrix;
integrated project summaries and pointers belong to `docs/status.md`. Apply the
Skill's status-summary contract: no finding IDs, descriptions, dispositions,
scores, or repair instructions in status-only summaries. Full reports remain in
their own records. A Review tick requires the latest applicable PASS, while
evidence gates remain separate under Evidence.

Reconcile stable contracts into `docs/specs/` at the topic's closure boundary,
after its last task passes review and applicable evidence gates are resolved.
Version-contract planning may record intended membership before all feature
work is finished; actual assembly and contract closure must follow Branching's
review and evidence prerequisites. A version-contract topic does not create
release authority. Preflight, tagging, publication, and installation retain
their separate authorization boundaries.

Retire a completed topic as a whole to `docs/archive/topics/<topic>/`, set
`status: historical` and `retired:` in its documents, and add one concise entry
to `docs/archive/README.md`. Reviews travel with the topic. The stable
`docs/README.md` links to the archive index rather than listing individual
archived files. Passing one document or committing one incremental change does
not retire an unfinished topic or close a broader audit.

### Document size

Keep the evidence and rationale needed to understand a document's decisions.
Length alone does not justify splitting a coherent topic or removing historical
evidence. Remove duplicate operational rules and unrelated material; preserve
useful historical context through existing records or precise Git references.

Historical explanations retained by the earlier form of this document are
available in commit `36e4b0e87f0ac0eda603e022afd52db26517a043`, file
`docs/documentation-workflow.md`. That snapshot preserves prior statements and
their context; it is not a claim that every historical observation was reverified
or that the snapshot is the current workflow contract.

### Entry document discipline

- `AGENTS.md` contains always-needed repository rules and conditional authority
  routing, not every detailed workflow rule.
- `docs/README.md` contains stable navigation and authority pointers.
- `docs/status.md` contains integrated project/release state and residual risk;
  `docs/roadmap.md` contains later planning, backlog, and withdrawals.
- Detailed rules have one routed owner. Topic status stays in `tasks.md` and
  full review history stays in its review record.

Before extending an entry document, determine whether every reader needs that
content. Otherwise update its routed authority and add only a concise pointer;
do not retain duplicate copies of the moved rule.

## Status Vocabulary

- `active`: current authority, pointer, or unfinished execution plan.
- `reference`: delivered supporting design retained for consultation; living
  authorities take precedence if behavior evolves.
- `historical`: completed, superseded, or audit-only material under `docs/archive/`.
