---
status: active
created: 2026-07-14
updated: 2026-08-24
---

# AgentDeck Documentation

This file is the documentation index and the concise authority for current
release and execution status. Code, tests, configuration, and Git history remain
the source of truth when they disagree with documentation. Historical execution
detail belongs in [the archive](archive/README.md), not in this index.

**What this file owns, and what it must not absorb.** The scope here is
cross-topic: which release is out, which topics exist and which version each
belongs to, roughly how far each has got, what is on the roadmap, what was
withdrawn, and where every authoritative document lives. Everything *inside* a
topic — its per-document and per-task `Draft`/`Review` cells, its round history,
its findings and their dispositions, its verification evidence — belongs to that
topic's own `tasks.md` and `reviews/`, and `tasks.md` is the only status authority
for its topic. So:

- A single document's or task's `Review` cell changing is topic-internal. It does
  not change this index, because a topic's stage is what this index states.
- A repair round changes no status at all — the cell reads the same before and
  after — so it updates its review record and nothing here.
- Update this file when a topic's stage changes, when version membership changes,
  when a candidate is withdrawn or added, when a release ships, or when the
  document set itself changes.
- Never restate a finding, a round number, or a verdict here. If a fact is worth
  keeping and is not cross-topic, the right place already exists.

That distinction was learned the expensive way: the Active Development prose below
had grown into a forty-line run-on of nineteen review rounds, duplicating two
other documents and going stale inside them — a copy asserted a prototype
self-check count that a later round had already changed. It was collapsed to
topic-level state on 2026-08-19.

## Current State

### Release

- **Latest stable:** [`v0.4.1`](https://github.com/kitdine/agent-deck/releases/tag/v0.4.1)
  at commit `3b709a8fb09494a8d8fdd37ee154e3baedbce9ea`, published 2026-08-13.
  It is a patch on `v0.4.0`: Codex `cache_write_input_tokens` is backfilled into
  a new `cache_write_tokens` column and already-indexed Codex sources are
  re-scanned, so historical cache-write figures change on upgrade rather than
  staying at the migration default of zero.
- The [stable Release workflow](https://github.com/kitdine/agent-deck/actions/runs/31677864670)
  passed same-SHA preflight enforcement, version-specific artifact verification,
  GitHub publication, and Homebrew verification. The non-draft,
  non-prerelease release contains Darwin arm64 and amd64 archives plus checksums.
- [Homebrew tap PR #18](https://github.com/kitdine/homebrew-tap/pull/18)
  merged the reviewed stable `Formula/agentdeck.rb` update. The workflow verified
  `brew install`, `brew test`, and bash, zsh, and fish completions.
- Beads coordination was blocked by schema skew and is **recovered** as of
  2026-08-16. The accidentally published `bd` v1.2.1 had migrated the database
  from schema v53 to v65; the cursor was rolled back per the upstream runbook
  and `bd` now runs without an override, with all thirty issues intact. Work
  leases, `bd heartbeat`, and `bd reclaim` do not exist in the installed
  v1.2.2 and are frozen in `.agent-instructions/beads.md` pending an upstream
  release. The path migration and document-level dispatch are complete as of
  2026-08-17: 24 document tasks exist across the four topics, every open issue
  cites topic paths, and no open issue carries a stale claim. Eleven closed
  issues still cite the old paths and are left as written, because a closed
  dispatch record states where the work actually pointed while it ran.
- Exact-SHA [release preflight run 31676882544](https://github.com/kitdine/agent-deck/actions/runs/31676882544)
  succeeded for the `v0.4.1` commit. **No CEv1 Release boundary was recorded for
  `v0.4.1`**; the newest one is `v0.4.0`, `VERIFIED` for Git tree
  `4cf71848342b9b3ddf4d0739ae67b293f568d306`. `v0.4.1`'s tree is
  `6b2a7279e36adcc3048d9b98431a1bc8e77f983c` and has no boundary of its own.
- The previous stable, [`v0.4.0`](https://github.com/kitdine/agent-deck/releases/tag/v0.4.0)
  at commit `6b7663b51f22903445798dd7db637cbcaab1a422`, completed
  terminal-presentation remediation's five tasks including manual visual
  acceptance of `session show --activity`, Usage interactive, and Session
  interactive surfaces. Those records are historical and indexed by
  [the archive](archive/README.md#2026-08-12-retirement-terminal-presentation-remediation).

Install the stable Homebrew channel with:

```bash
brew install kitdine/tap/agentdeck
agentdeck version
```

### Active Development

| Topic | Version | Status | Purpose |
| --- | --- | --- | --- |
| [Native macOS Desktop App](topics/desktop-app/tasks.md) | `v0.5.0` | Review complete — 6/6 tasks reviewed with exact-state CEv1 Task gates VERIFIED; the containing desktop-app Plan gate is VERIFIED; tasks 4 `desktop-widget`, 5 `unified-desktop-distribution`, and 6 `desktop-app-contract` still await their authorized commit checkpoints | macOS 26 menu-bar app, settings window, WidgetKit extension, unified desktop distribution, Cask, and direct-download delivery. |
| [Work Signals](topics/work-signals/tasks.md) | `v0.5.0` | Active — all five design documents PASS after three combined rounds; prototype task done, implementation task 1 next; 0/6 implementation tasks | Activity classification, workflow metrics, and tool-call attribution, delivered on two first-class surfaces: the menu-bar `Sessions` panel's three pending modules and a new `agentdeck usage signals`. |
| [`v0.5.0` Contract Closure](topics/v0-5-0-contract/tasks.md) | `v0.5.0` | Active — 0/2 done | Version-wide specification raise and documentation reconciliation after every selected topic's tasks pass review. |
| [Usage Attribution Precision](topics/usage-attribution-precision/tasks.md) | `v0.6.0` | Active — 0/3 done | Per-client attribution time semantics, determinability-based quality, and an unattributed boundary that never enters a real-spend total. |
| [CLI Error Classification](topics/cli-error-classification/tasks.md) | `v0.5.0` | Active — all design documents PASS; implementation task 1 next; 0/2 implementation tasks done | Stable not-found codes, and no storage text in a documented JSON contract. Breaks the documented `runtime_error` value; announced in this version's notes. |
| [Switch Effectiveness Boundary](topics/switch-effectiveness-boundary/tasks.md) | `v0.5.0` | Active — all design documents PASS; implementation task 1 next; 0/4 implementation tasks done | A Claude switch to subscription does not reach a running session, so its advisory must say so and its route must not claim a multiplier the session is not billing at. |

**`v0.5.0` contains exactly the rows marked `v0.5.0` above**, plus the contract
closure that reconciles them. The count is deliberately not written here: it was
stated as a number twice and was wrong both times, once when a topic was
selected into the version and once when a topic's task count changed. The rows
are the list. The authoritative scope statement is
[`topics/v0-5-0-contract/tasks.md`](topics/v0-5-0-contract/tasks.md); per-task
state lives in each topic's own `tasks.md`, which is the only status authority
for that topic. A topic carries no version number of its own — membership is
decided here and in the contract topic, so a reschedule changes those two places
and nothing else.

The desktop topic's six documents were re-opened on 2026-08-18: a reviewable
prototype at the contract dimensions exposed structural defects that nineteen
rounds of prose review had passed, and acting on them changed the content of every
document, which unticks every `Review` cell because evidence binds to a content
state rather than to a file name. Three consequences are version-scope decisions
and are recorded as such — the desktop update check is withdrawn (see Withdrawn
Candidates), the three work-signal modules get their own topic, and the former
`presentation-period-scoping` producer slice is merged into
`menubar-experience` so the wire and the surface form one reviewable task.
Everything else
about that re-open — which defects, what survives unchanged, the dependency order
the set is re-reviewed in, and every round and finding — belongs to the topic and
lives in [`topics/desktop-app/tasks.md`](topics/desktop-app/tasks.md) and
[`topics/desktop-app/reviews/`](topics/desktop-app/reviews/).

Two corrections were made on 2026-08-20, both cross-topic and both recorded here
because they change version membership and process, not because they narrate a
round.

**Work signals are back in `v0.5.0`.** The 2026-08-18 re-open recorded the three
modules as moved to the Backlog and as "refused" by the desktop boundary. That
cut a committed feature out of the version without asking, and the stated ground
— that no field exists behind them — was only partly true: `internal/activity`
already extracts tool calls and `usage_tool_calls` has persisted them since
schema v13. The capability is restored as its own topic,
[`work-signals`](topics/work-signals/tasks.md), which supplies the data, turns
the panel's pending cards into real ones, and adds `agentdeck usage signals` so
the numbers are checkable from a terminal rather than visible only in a GUI. `menubar-experience` is unaffected: it
ships the three modules in their `Not captured yet` form, which stays a valid
state because the new wire families are additive.

**`desktop-app` defers document review to one closing pass.** By user
instruction, that topic runs no document review rounds while its tasks are being
implemented: changes are written directly into the process record that owns them.
Review is deferred, not cancelled — after every implementation task is done, the
whole set is reconciled against the final prototype and the shipped
implementation and reviewed once, scoped as a bullet on that topic's task 6. The
reason and the consequences are stated in
[`topics/desktop-app/tasks.md`](topics/desktop-app/tasks.md). As of 2026-08-23
the set is reconciled and submitted; the closing round has not run.

This is specific to `desktop-app`, whose documents were being re-reviewed against
an implementation that is still moving.

**`work-signals` reviews its five documents together, in one round, under one
verdict.** That too is by user instruction, and for a different reason: reviewing
documents that constrain each other one at a time does not converge. Its first
design pass ran fourteen single-document rounds before being discarded, six of
them repairing damage the order itself had caused. Whether this becomes the
project's general process is decided after that topic, not by it. Task review is
unaffected in both topics.

## Authoritative Documents

| Document | Status | Authority |
| --- | --- | --- |
| [Product Prototype](../prototype/README.md) | Active | The design truth for every user-visible surface: menu-bar panel, widgets, settings, and CLI output. A document that disagrees with it is repaired. |
| [CLI Design](specs/cli-design.md) | Active, version 26 | System, persistence, security, compatibility, and distribution contracts. |
| [CLI Manual](specs/cli-manual.md) | Active | Implemented commands, flags, output shapes, and interaction behavior. |
| [Archived Documents](archive/README.md) | Active index | Retirement history and pointers to historical topics, plans, and reviews. |

A topic's own documents are authoritative for that topic while it executes; see
its `tasks.md` in the table above. Review-record format lives in
`.agent-instructions/review-records.md`.

User-facing entry points are the [English README](../README.md) and
[Chinese README](../README_zh.md). Repository-specific development and
authorization rules live in [AGENTS.md](../AGENTS.md).

## Roadmap

Planned after `v0.5.0`. Re-planned on 2026-08-13; this table supersedes the
release sequence previously recorded in the desktop plan. Each version has a
Beads tracking epic and a blocked design task, and needs a bounded topic under
`docs/topics/` before development starts. Version themes are commitments of
sequence, not of scope detail.

| Version | Theme | Scope |
| --- | --- | --- |
| `v0.6.0` | Cost truthfulness | Attribution timeline precision, billing-mode detection, Codex cache-write pricing, `codex-auto-review` fallback classification, `unpriced` disambiguation, layered cost presentation, budget rules, menu-bar alerting. |
| `v0.7.0` | Subscription quota | Quota interface feasibility, in-app network quota lookup, allowance-window and reset modelling, quota alerting, automatic update download, prerelease channel selection. |
| `v0.8.0` | Boundary consolidation and Linux | Versioned client adapter contract, Linux machine identity, de-darwin PTY tests, Linux CI matrix and release artifacts. |
| `v0.9.0` | Observability completion | Extension enabled state, cross-client duplication and drift, source authenticity, structured session search filters, wrapper health probing, richer desktop session window. |
| `v1.0.0` | Multi-device and trust | Device dimension, backup merge import, read-only aggregation views, CLI archive signing and notarization. |

Direction decisions that shape this roadmap:

- Client breadth stays at Codex and Claude. An explicit versioned client adapter
  contract is extracted so a later out-of-process plugin model can add clients
  externally; no third client is added in-tree.
- Each machine remains its own authoritative store. Cross-device support is
  read-only aggregation, never bidirectional synchronization.
- Proactive behavior — alerting and scheduled evaluation — is hosted by the
  menu-bar app. No daemon, LaunchAgent, or network listener is introduced, and
  alert rules stay in Go.
- The CLI targets macOS and Linux; the GUI stays macOS-only. Capability layering
  is explicit rather than accidental.
- Cost has three coexisting dimensions. Third-party API with a multiplier and
  official API are real spend computed locally; official subscription is quota,
  requires network access, and is therefore handled inside the app. Equivalent
  API cost is retained as a reference baseline for every mode.
- Extension work is bounded to cross-client observability. Each client already
  owns its own management surface; no tool reports the cross-client view.

## Backlog

These candidates have no approved implementation plan and no version. Promote
each into a bounded plan before development; do not expand an active plan
opportunistically. Candidates that now carry a version live in the Roadmap above.

- [ ] Revisit ChatGPT app project attribution only if the app exposes a stable,
  reachable project configuration surface.

## Withdrawn Candidates

Recorded so they are not rediscovered as gaps. Reopen only if the stated reason
stops holding.

- **Desktop update check.** Withdrawn from `v0.5.0` entirely on 2026-08-18 — no
  menu item, no preference, no copy, no network request. The desktop app is
  installed through the Cask or a direct download, both of which already carry an
  upgrade path, so an in-app check would add the product's only outbound request
  to duplicate one.
- **Homebrew core submission.** Not important to this project; the personal tap
  already serves stable and release-candidate channels.
- **Claude subscription/account switching.** Technically reachable — the login
  state is a single per-system-user macOS Keychain entry — but withdrawn: OAuth
  refresh tokens rotate server-side so a saved snapshot silently expires and
  cannot be validated offline, persisting another product's credential
  contradicts this project's no-plaintext-credential rule, cross-application
  Keychain access is hard to justify in the trust model, and a failed write
  leaves the user unable to authenticate with no rollback path.
- **Extension mutation lifecycle.** The preview/plan/apply/ownership/rollback
  engine and its GUI, previously planned as two whole releases, chase each
  client's evolving extension format and duplicate management surfaces the
  clients already ship. Replaced by cross-client observability in `v0.9.0`.

## Known Residual Risk

- Plaintext credential values and derived key bytes are not reliably zeroed
  after use. Go's copying garbage collector and immutable `string` values make a
  complete wipe guarantee unavailable; this remains an accepted residual risk.

## Naming Convention

- Use lowercase kebab-case topic names, with no date prefix and no version
  number.
- A topic is a directory: `docs/topics/<topic>/`, with the fixed document names
  in the Topic structure section above.
- Review records live inside the topic and are named after what they review:
  `docs/topics/<topic>/reviews/requirements.md`,
  `.../reviews/ux-<surface>.md`, `.../reviews/architecture.md`,
  `.../reviews/tasks.md`, and `.../reviews/<task-anchor>.md`.
- `docs/specs/` holds contracts, not designs. A file there describes behavior the
  product guarantees, not how a topic intends to build it.
- Established living authorities such as `cli-design.md` and `cli-manual.md` keep
  their stable names.
- A follow-up that remains part of an unfinished plan uses a dated
  `## Follow-Up — YYYY-MM-DD` subsection. Work with a distinct goal or acceptance
  boundary gets a new plan.
- Unscoped plan-local ideas belong in that plan's `Backlog` or
  `Future Feature Ideas` section. Only repository-wide candidates belong in this
  index's Backlog.

Use frontmatter appropriate to the document:

```yaml
---
status: active | reference | historical
created: YYYY-MM-DD
updated: YYYY-MM-DD   # when a current document materially changes
retired: YYYY-MM-DD   # archived documents only
version: N            # versioned specifications only
---
```

## Document Lifecycle

| Directory | Purpose | Lifecycle |
| --- | --- | --- |
| `docs/topics/<topic>/` | One coherent behavior change, from requirements through tasks | Keep `active` until every required gate passes; then archive the whole directory. |
| `docs/specs/` | Current product and interaction contracts only | Revise in place while authoritative. Receives a topic's stable contracts after its last task passes review. |
| `docs/archive/` | Retired topics and superseded material | Preserve history; never use as the starting point for new work. |
| `docs/README.md` | Current status and navigation | Keep concise and update in place; do not duplicate historical narratives. |

### Topic structure

A topic owns one coherent behavior change and carries no version number. Version
membership is decided by a `vX-Y-Z-contract` topic and recorded in the Roadmap
above; nothing about a topic changes when its target version does.

```text
docs/topics/<topic>/
  requirements.md        goals, non-goals, acceptance boundary
  ux/<surface>.md        interaction design, one file per user-visible surface
  architecture.md        development design, contracts, boundaries
  tasks.md               task breakdown and the status matrices
  reviews/<name>.md      one record per document and per task
```

#### Why these documents, and how many

A document exists to be reviewed as its own artifact — that is the reason a
topic keeps its design next to its tasks instead of scattering it. So a document
earns its existence by having a **distinct review question**, answered against
different evidence:

| Document | Review question | Evidence |
| --- | --- | --- |
| `requirements.md` | Is the boundary decided? | Goals, non-goals, acceptance; no TBD |
| `ux/<surface>.md` | Does every user-visible state have a presentation rule and copy? | The state set, in every shipped language |
| `architecture.md` | Is every new contract fully specified, and every claim about existing code located? | Contract text and the code it names |
| `tasks.md` | Does the decomposition cover the others, with nothing missing and nothing beyond? | The other documents |

That gives the test for splitting or merging: **two candidate documents that
would be reviewed by asking the same question against the same evidence are one
document, and one document that needs two unrelated questions to judge complete
is two.** Apply the test to content, not to length.

The count follows from the same test, which is why `ux/` is plural and
`architecture.md` is not:

- One `requirements.md`, because a topic is one coherent behavior change and so
  has one boundary question.
- One `tasks.md`, because there is one decomposition.
- One `ux/<surface>.md` **per surface**, because reviewing one surface says
  nothing about whether another is complete — each has its own state set and
  copy.
- One `architecture.md`, because "are the contracts specified" is a single
  question. Split it only for genuinely independent contract domains, and argue
  the split in `tasks.md` rather than assuming it.

A `vX-Y-Z-contract` topic needs only `tasks.md`: it reconciles what other topics
already delivered and originates no requirement, surface, or architecture of its
own.

#### The specimen requirement

A `ux/<surface>.md` carries a rendered specimen of each state, not only rules
about it. Rules and specimen are one document because they are reviewed against
each other: a specimen with no rules cannot be checked, and rules with no
specimen can satisfy every stated condition while remaining illegible. A
geometry table of `340 pt` and `280 pt` does not let anyone see what the surface
looks like at either bound.

Be explicit about what a specimen settles. For a terminal surface it is close to
exact, because the specimen and the output share a medium. For a GUI it is an
approximation that settles hierarchy, copy, state coverage, and wrap or
truncation at the narrow bound, and settles nothing about real typography,
Dynamic Type, or assistive-technology order. Those need the manual acceptance
the topic's tasks already require; a specimen is presentation evidence and never
substitutes for runtime evidence.

The cost of omitting one is not hypothetical. An independent UI audit of the
menu-bar design scored it zero on aesthetics because no prototype existed, which
under that tool's own rules forced a redesign verdict its total contradicted.
The score was invalid, and the document was also genuinely missing the evidence
class the question needed.

#### When the set is decided, and by whom

The set cannot be fixed when the topic is created. A surface or contract domain
often becomes knowable only when a later task's scope is written, and pretending
otherwise is how a required document goes missing without anyone noticing.

So the set is a claim, and the `Documents` matrix in `tasks.md` is the only
place that claim lives. Do not also declare it in prose elsewhere; a second
writer is how the two drift apart.

- List a document that is required but **not yet written** as a row with `Draft`
  unticked. An empty row is the point — it makes the gap visible. Never commit
  an empty file to fill it.
- List a kind that does not apply as a row saying so, rather than omitting it
  silently, so a reader can tell "decided not to" from "forgot".
- Revise the set — the matrix rows — whenever a task's scope names a surface or
  contract domain it does not cover. Revising the set adds a row; it does not
  write the document. That is a later `设计：<topic> / <document>`.
- `bash scripts/check-topic-docs.sh` audits the result, and the `tasks.md` reviewer runs
  it, because that review is where the set is ratified. It compares three things
  that must agree — what the matrix declares, what exists on disk, and what the
  topic's own `requirements.md` names as a surface — so a required document that
  nobody remembered fails a check instead of waiting to be noticed. That third
  comparison only works because `requirements.md` names each surface by its
  path; a surface described in prose alone is invisible to it, which is how
  `ux/widget.md` stayed missing.
- It is a workflow tool, not a build step. It reads only `docs/topics/**`, so no
  code change can fail it, and putting it in `make verify` would fail a
  code-only CI run for a missing design document. Documentation obligations bind
  the phase that owns them, not the build.

The author proposes the set; **the `tasks.md` reviewer ratifies it**. This adds
no reviewer role, because `tasks.md`'s review question already asks whether the
breakdown covers the other documents with nothing missing — whether the set
itself is complete is that same question. It follows that changing the set
returns `tasks.md` to review, and only `tasks.md`; the other documents keep
their verdicts.

#### Creating a topic

A topic is promoted from one of five origins, and the origin belongs in
`requirements.md`'s opening so a later reader can tell why the work exists:

- a Roadmap version theme, narrowed to one coherent behavior change;
- a Backlog candidate;
- a finding recorded in another topic's review;
- a measured defect in released behavior;
- a direct request.

Promote with the design trigger, naming a topic that does not exist yet:

```text
设计：`<topic>`
```

Not `初始化`. That command is the workflow's project-initialization route, which
refreshes the managed guidance block in `AGENTS.md` and runs once per
repository; reusing the word for topic creation would make one trigger mean two
things. Creating a topic is design work anyway — `requirements.md` is the
boundary decision, which is exactly what the design phase produces.

**A topic starts from an observation, not from an idea.** Both topics promoted
so far open with measurement — one with a table of `error.code` and
`error.message` values captured from the released binary, the other with counted
attribution shares from the real local store. That is not a stylistic habit. The
review question for `requirements.md` is whether the boundary is decided, and a
boundary around a problem nobody has observed cannot be decided, only guessed.
So the minimum input is the observed behavior and how it was measured, the
surfaces and contracts it touches, and what is deliberately excluded.

#### The progression from a topic to development

A new topic is a name and an observation. Nothing else exists, so the useful
question is not which documents a topic may carry but what to do next. Each
stage below states what it produces and when it is finished.

| Stage | Command | Produces | Finished when |
| --- | --- | --- | --- |
| 1. Boundary | `设计：<topic>` | `requirements.md`, plus `tasks.md` holding only the Documents matrix | The boundary is decided and the intended document set is declared |
| 2. Boundary review | `评审：<topic> / requirements.md` | A review record | `Verdict: PASS` |
| 3. Surface framework | `设计：<topic> / ux/<surface>.md` | Layout, hierarchy, states, and a **data requirements list** naming the field each element needs | A reviewer can see the surface and read what it demands |
| 4. Framework review | `评审：<topic> / ux/<surface>.md` | A review record | `Verdict: PASS` on the framework and its requirements list |
| 5. Contract | `设计：<topic> / architecture.md` | Contracts and boundaries, provisioning each requested field or vetoing it with a stated ground | Every requested field is provisioned or refused in writing |
| 6. Contract review | `评审：<topic> / architecture.md` | A review record | `Verdict: PASS` |
| 7. Surface final | `设计：<topic> / ux/<surface>.md` | The surface absorbing any veto | No element depends on a field the contract refused |
| 8. Decomposition | `设计：<topic> / tasks.md` | The Tasks matrix | Every task has an anchor, its files, and a verification level |
| 9. Decomposition review | `评审：<topic> / tasks.md` | A review record | `Verdict: PASS`; the topic is now developable |
| 10. Development | `开发：<topic> / <task-anchor>` and its review | Code and tests | Per the Tasks matrix |

A topic with no user-visible surface skips stages 3, 4, and 7 and runs
requirements → contract → decomposition.

Stages 1 through 9 turn a topic into something developable; stage 10 builds it.
The boundary between them is `tasks.md` passing review — before that there is no
task to implement, and after it every task's scope is fixed.

##### Why the surface leads the contract

An earlier version of this table put `ux/` and `architecture.md` in one stage,
reviewed in parallel, on the reasoning that their questions and evidence are
independent. They are not, and the desktop topic proved it: the menu-bar surface
needed a daily series, model shares, and attribution counts that the App Group
projection did not carry, and rather than asking for them the surface document
recorded them as rejected — citing a contract written days earlier as though it
were a fact about the world. The result was a poor surface justified by a
constraint nobody had defended.

The dependency is real and it is a cycle: the surface cannot be finished without
knowing what data exists, and the contract cannot be sized without knowing what
the surface needs. What breaks the cycle is that the two sides are not
symmetric. **A contract can nearly always be extended to carry more derived,
non-sensitive data; a surface can never invent data that is not there.** So the
surface goes first and states what it needs, and the contract answers.

That gives architecture a specific obligation and a specific power. It must
provision each requested field or refuse it, and a refusal names its ground —
privacy, cost, or genuine unavailability — in writing, where a reviewer can
disagree with it. "The contract does not carry that" is not a ground; it is the
thing under discussion.

**Decomposition is stage 5, not stage 1.** A task is a unit of development work,
and what work exists is not knowable from a boundary alone; it is knowable from
the specification. Producing the Tasks matrix earlier would mean writing a claim
about documents that do not exist, which is exactly what its review question
asks about — "does the decomposition cover the others" — and exactly what the
evidence column names: the other documents. That is why `tasks.md` is reviewed
last, and it has to be produced last for the same reason.

`tasks.md` still exists from stage 1, because it is the topic's only status
authority and the Documents matrix has to live somewhere. Its Tasks matrix is
simply empty until stage 5.

The verbs are the workflow's, unchanged; no command is invented here. What
varies is the target after the colon, and one rule covers all of it: **the verb
is the phase, and what follows the colon is what that phase acts on.**

Repair and re-review take the same shape as the rest, and name the record rather
than the subject:

```text
修复：<topic> / reviews/<record>.md / <finding ids>
复评：<topic> / reviews/<record>.md
```

Naming the record, not the document or task, is what keeps the target
unambiguous. A record's findings can span more than one document — the menu-bar
round does — and a record's name can collide with a task anchor, so `修复：
<anchor>` alone cannot say whether the design or the implementation is being
repaired. A path can.

`开发` never writes a design document; it writes the code a reviewed document
already specified. When a task's scope reveals a surface or contract the matrix
does not cover, that is a `tasks.md` revision plus a later `设计：<topic> /
<document>` — not something the task absorbs.

#### Why the progression is ordered that way

The stage order is not a convention; it falls out of the review questions.

- **`requirements.md` first** because every other document's scope derives from
  its boundary. Findings raised against a surface or contract whose boundary is
  still open evaporate when the boundary moves, so that review is spent twice.
- **`ux/<surface>.md` before `architecture.md`**, because a surface states what
  data it needs and a contract answers. Reviewing them in parallel invites the
  contract to be treated as fixed and the surface to be trimmed to fit it, which
  is exactly the failure the desktop topic recorded.
- **`tasks.md` last** because its question is whether the breakdown covers the
  other documents, which cannot be answered — or even written — before they
  exist.

The Documents matrix therefore exists from the moment the topic does but is
ratified at the end. That is not a contradiction: it is a claim from the start
and a verdict at the finish, which is the same shape as `Draft` and `Review` on
every other row.

### Status

`tasks.md` is the only status authority for its topic, and it carries two
matrices because documents and tasks are different kinds of work:

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

`Draft` means the author asserts the document is complete enough to review. It is
not a formatting claim: a document may be marked ready for review only when it
meets its readiness condition. Each condition below is the author's side of the
review question that justifies that document existing at all, in Topic
structure above — the author asserts it, the reviewer asks it.

| Document | Ready to review when |
| --- | --- |
| `requirements.md` | Goals, non-goals, and acceptance boundary are stated; no TBD remains; it lists every user-visible surface **by its `ux/<surface>.md` path**, or says the topic adds none, and declares whether new contracts are in scope |
| `ux/<surface>.md` | Every user-visible state has a presentation rule, copy in every shipped language, and a rendered specimen; every element names the data field it needs; no placeholder remains |
| `architecture.md` | Every new contract is fully specified, every claim about existing code names where it was verified, and every field a surface requested is provisioned or refused with a stated ground |
| `tasks.md` | Every task has an anchor, its files, and a verification level, and the set covers the other documents' scope with nothing missing and nothing beyond it |

`tasks.md` appears in its own Documents matrix. Reviewing it asks whether the
breakdown is sound and complete against the other documents, and whether the
document set that matrix declares is itself complete — both are different
questions from whether any task's implementation is correct.

This index records only a coarse `X/N` rollup per topic and never duplicates
per-document or per-task status.

- A `Review` tick requires a review record whose latest applicable round is
  `Verdict: PASS`. A reopened finding returns that document or task to work.
- A topic reconciles its stable contracts into `docs/specs/` only after its last
  task passes review, not while it is still executing.
- A `vX-Y-Z-contract` topic begins only after its included topics pass review,
  raises the specification version exactly once, and ends at Review PASS.
  Preflight, release-channel selection, tagging, publication, and local
  installation remain separately authorized delivery stages.
- Retire a completed topic with one `git mv` of `docs/topics/<topic>/` to
  `docs/archive/topics/<topic>/`, set `status: historical` and `retired:` in each
  document, and add one concise entry to `docs/archive/README.md`. Reviews travel
  with the topic because they live inside it.
- Do not re-list individual archived files in this index. Link the archive index
  instead.

### Document size

Length is judged by what produced it, not by a line count. A long document is
correct when the length comes from recorded design argument — rejected
alternatives with their reasons, retracted judgements, measured baselines — and
wrong when it comes from accumulating unrelated work. Retired topics in the
archive include several documents past a thousand lines that were deliberately
never split.

Do not split a document to hit a size target. Split a topic only when it stops
owning one coherent behavior change.

## Status Vocabulary

- `active`: current authority, pointer, or unfinished execution plan.
- `reference`: delivered supporting design retained for consultation; living
  authorities take precedence if behavior evolves.
- `historical`: completed, superseded, or audit-only material under
  `docs/archive/`.
