---
status: active
created: 2026-07-14
updated: 2026-08-12
---

# AgentDeck Documentation

This file is the documentation index and the concise authority for current
release and execution status. Code, tests, configuration, and Git history remain
the source of truth when they disagree with documentation. Historical execution
detail belongs in [the archive](archive/README.md), not in this index.

## Current State

### Release

- **Latest stable:** [`v0.4.0`](https://github.com/kitdine/agent-deck/releases/tag/v0.4.0)
  at commit `6b7663b51f22903445798dd7db637cbcaab1a422`.
- The [stable Release workflow](https://github.com/kitdine/agent-deck/actions/runs/31664284248)
  passed same-SHA preflight enforcement, version-specific artifact verification,
  GitHub publication, and Homebrew verification. The non-draft,
  non-prerelease release contains Darwin arm64 and amd64 archives plus checksums.
- [Homebrew tap PR #17](https://github.com/kitdine/homebrew-tap/pull/17)
  merged the reviewed stable `Formula/agentdeck.rb` update. The workflow verified
  `brew install`, `brew test`, and bash, zsh, and fish completions.
- Exact-SHA [release preflight run 31607179658](https://github.com/kitdine/agent-deck/actions/runs/31607179658)
  and the `v0.4.0` CEv1 Release boundary are `VERIFIED/PASS` for Git tree
  `4cf71848342b9b3ddf4d0739ae67b293f568d306`.
- Terminal-presentation remediation completed all five tasks, including manual
  visual acceptance of `session show --activity`, Usage interactive, and Session
  interactive surfaces. Its plan and review records are historical and indexed
  by [the archive](archive/README.md#2026-08-12-retirement-terminal-presentation-remediation).

Install the stable Homebrew channel with:

```bash
brew install kitdine/tap/agentdeck
agentdeck version
```

### Active Development

| Topic | Status | Purpose |
| --- | --- | --- |
| [Native macOS Desktop App](topics/desktop-app/tasks.md) | Active — 2/6 tasks reviewed; menu-bar design Review FAIL | macOS 26 menu-bar app, WidgetKit extension, unified desktop distribution, Cask, and direct-download delivery. |
| [`v0.5.0` Contract Closure](topics/v0-5-0-contract/tasks.md) | Active — 0/1 done | Version-wide specification raise and documentation reconciliation after all desktop tasks pass review. |
| [Usage Attribution Precision](topics/usage-attribution-precision/tasks.md) | Active — 0/3 done | First `v0.6.0` topic: per-client attribution time semantics, determinability-based quality, and an unattributed boundary that never enters a real-spend total. |
| [CLI Error Classification](topics/cli-error-classification/tasks.md) | Active — 0/2 done | Unassigned to a version: stable not-found codes, and no storage text in a documented JSON contract. |

`desktop-wire-contract` reached Re-review Round 2 PASS.
`macos-app-foundation` reached Re-review Round 3 PASS after unsupported and
malformed App Group cache data were made fail-closed. The menu-bar contract
failed Design Review Round 3 on six decision-completeness findings and requires
bounded design repair before Development; the findings span the topic's
`architecture.md` and `ux/menubar.md`.

Usage attribution precision is planned but not started. It is independent of
the desktop topic and blocks the remaining `v0.6.0` cost items.

## Authoritative Documents

| Document | Status | Authority |
| --- | --- | --- |
| [CLI Design](specs/cli-design.md) | Active, version 25 | System, persistence, security, compatibility, and distribution contracts. |
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
Beads tracking epic and a blocked design task, and needs a bounded plan under
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

Only `requirements.md` and `tasks.md` are always required. The others are
required when they apply:

| Document | Required when |
| --- | --- |
| `ux/<surface>.md` | The topic changes or adds a user-visible surface |
| `architecture.md` | The topic introduces a contract, a boundary, or a cross-module change |

When one does not apply, say so in `requirements.md` — "this topic changes no
user-visible surface" — rather than committing an empty file. A
`vX-Y-Z-contract` topic needs only `tasks.md`: it reconciles what other topics
already delivered and originates no requirement, surface, or architecture of its
own.

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
meets its readiness condition.

| Document | Ready to review when |
| --- | --- |
| `requirements.md` | Goals, non-goals, and acceptance boundary are stated; no TBD remains; it declares whether a user-visible surface and new contracts are in scope |
| `ux/<surface>.md` | Every user-visible state has a presentation rule and copy in every shipped language; no placeholder remains |
| `architecture.md` | Every new contract is fully specified, and every claim about existing code names where it was verified |
| `tasks.md` | Every task has an anchor, its files, and a verification level, and the set covers the other documents' scope with nothing missing and nothing beyond it |

`tasks.md` appears in its own Documents matrix. Reviewing it asks whether the
breakdown is sound and complete against the other documents, which is a different
question from whether any task's implementation is correct.

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
