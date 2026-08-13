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

| Plan | Status | Purpose |
| --- | --- | --- |
| [Native macOS Desktop App](plans/desktop-app.md) | Active — 0/6 done | macOS 26 menu-bar app, WidgetKit extension, unified desktop distribution, Cask, and direct-download delivery. |
| [`v0.5.0` Contract Closure](plans/v0-5-0-contract.md) | Active — 0/1 done | Version-wide specification raise and documentation reconciliation after all desktop tasks pass review. |

The next planned feature anchor is `desktop-wire-contract`; development has not
started. There are no open execution tasks outside an active plan.

## Authoritative Documents

| Document | Status | Authority |
| --- | --- | --- |
| [CLI Design](specs/cli-design.md) | Active, version 24 | System, persistence, security, compatibility, and distribution contracts. |
| [CLI Manual](specs/cli-manual.md) | Active | Implemented commands, flags, output shapes, and interaction behavior. |
| [Terminal Rendering Experience](specs/2026-08-06-terminal-rendering-design.md) | Active | Shared Usage and Session terminal framing, responsive geometry, accessibility, and lifecycle. |
| [Usage Interactive Viewer](specs/2026-08-07-usage-interactive-viewer-design.md) | Reference | Delivered Usage interactive layout, state, color/no-color, and PTY contract. |
| [Review Records](reviews/README.md) | Active | Review-record format and relationship to plan task gates. |
| [Archived Documents](archive/README.md) | Active index | Retirement history and pointers to historical plans and reviews. |

User-facing entry points are the [English README](../README.md) and
[Chinese README](../README_zh.md). Repository-specific development and
authorization rules live in [AGENTS.md](../AGENTS.md).

## Backlog

These candidates have no approved implementation plan. Promote each into a
bounded plan before development; do not expand an active plan opportunistically.

- [ ] Resolve `codex-auto-review` billing only when authoritative billing or
  account evidence establishes how its independently reported tokens are charged.
  See the [historical classification plan](archive/plans/codex-auto-review-classification.md).
- [ ] Design native lifecycle management for Skills, Plugins, MCP servers, and
  Hooks as separate ownership and security plans with preview, drift detection,
  atomic mutation, rollback, source authenticity, dependency, credential, and
  offline contracts.
- [ ] Verify through observed requests whether the Claude app picks up
  project-scoped `.claude/settings.local.json` without restart. Documentation
  inference is insufficient; see the CLI manual's
  [Project Attribution](specs/cli-manual.md#project-attribution) section.
- [ ] Revisit ChatGPT app project attribution only if the app exposes a stable,
  reachable project configuration surface.
- [ ] Design Claude subscription/account switching separately from API-provider
  switching, including account, OAuth, credential, and security boundaries.

## Known Residual Risk

- Plaintext credential values and derived key bytes are not reliably zeroed
  after use. Go's copying garbage collector and immutable `string` values make a
  complete wipe guarantee unavailable; this remains an accepted residual risk.

## Naming Convention

- Use lowercase kebab-case topic names.
- New dated design documents use
  `docs/specs/YYYY-MM-DD-<topic>-design.md`.
- New execution plans use `docs/plans/YYYY-MM-DD-<topic>.md`.
- Established living authorities such as `cli-design.md`, `cli-manual.md`, and
  existing active plans keep their stable names; do not rename them solely to
  adopt the dated convention.
- Review directories mirror the owning plan topic and use one file per task
  anchor: `docs/reviews/<plan-topic>/<task-anchor>.md`.
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
| `docs/specs/` | Current product and interaction contracts | Revise in place while authoritative. A supporting delivered design may become `reference`. |
| `docs/plans/` | Finite approved execution | Keep `active` until every required task gate passes; then archive. |
| `docs/reviews/` | Per-task review evidence matching active plans | Archive with the owning plan. |
| `docs/archive/` | Historical plans, reviews, and superseded material | Preserve history; never use as the starting point for new work. |
| `docs/README.md` | Current status and navigation | Keep concise and update in place; do not duplicate historical narratives. |

- A feature plan owns one coherent behavior change and reconciles that behavior
  into living specifications. It does not raise the specification version.
- A version contract plan begins only after its included feature plans pass
  review, raises the specification version exactly once, and ends at Review
  PASS. Preflight, release-channel selection, tagging, publication, and local
  installation remain separately authorized delivery stages.
- Each plan owns its task matrix. This index records only a coarse `X/N` rollup;
  it never duplicates per-task status.
- A Review tick requires a corresponding review record whose latest applicable
  round is `Verdict: PASS`. A reopened finding returns the task to development.
- Retire completed work with `git mv`: move the plan to `docs/archive/plans/`,
  move its review directory to `docs/archive/reviews/`, set `status: historical`
  and `retired:`, and add one concise entry to `docs/archive/README.md`.
- Do not re-list individual archived files in this index. Link the archive index
  instead.

## Status Vocabulary

- `active`: current authority, pointer, or unfinished execution plan.
- `reference`: delivered supporting design retained for consultation; living
  authorities take precedence if behavior evolves.
- `historical`: completed, superseded, or audit-only material under
  `docs/archive/`.
