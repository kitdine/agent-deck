---
status: historical
created: 2026-08-11
retired: 2026-08-11
---

# RC3 Terminal UX Acceptance Remediation

This feature plan closes every issue found during manual acceptance of
`v0.4.0-rc.2`. The published `rc.2` tag and release remain immutable. After all
four tasks reach Review PASS, the resulting exact commit may proceed through a
fresh `release-preflight` and, if that preflight succeeds, publication as
`v0.4.0-rc.3`.

## Goal

Make Usage and Session terminal surfaces readable, lively, semantically
complete, and responsive without changing machine-readable JSON contracts,
privacy boundaries, or the explicit read-only interactive lifecycle.

## Scope and acceptance contract

- Usage interactive output uses a bright capability-aware semantic palette;
  populated frames are no longer dominated by gray tracks or gray primary
  values. Color remains decorative rather than the only carrier of meaning.
- Bars appear only when a value has a defined denominator or comparison basis.
  Overview KPIs and other absolute values never render meaningless empty tracks.
- Activity Heatmap is present for every valid Usage range, including hourly,
  short, empty, filtered, and partial-cost reports. The visible bucket unit is
  deterministic, explicit, and may adapt to the range and available geometry.
- Root `session --interactive` renders Claude models from indexed metadata,
  never leaves an unexplained blank model, and keeps source ownership rules.
- Session browser and detail viewer share Usage's title, tab, selection,
  warning, status, color/no-color, viewport, and responsive layout semantics.
- Documents expose selected approved visible text with bounded wrapping;
  Activity exposes safe call metadata; Tokens exposes every normalized
  invocation's token components, cost, coverage, unpriced components, and
  warnings without claiming an unreliable conversation-turn join.
- Ordinary `session show` has a bounded readable canvas at narrow, normal,
  wide, redirected, and explicit `COLUMNS` widths. It does not emit an
  unbounded table based on document length or a fixed 13-column invocation row.
- `NO_COLOR`, `--no-color`, non-TTY text, JSON, `TERM=dumb`, terminal cleanup,
  privacy allowlists, bounded lazy paging, and source-log read-only behavior are
  preserved.

## Non-goals

- No new TUI framework or dependency.
- No persistence of tool arguments, tool results, hidden reasoning, system or
  developer messages, credentials, source paths, or other private source data.
- No attempt to infer exact conversational turns from normalized usage events.
- No mutation or replacement of the existing `v0.4.0-rc.2` tag or release.

## Tasks

### `session-model-index`

- Read Claude assistant model identity from the real `message.model` shape while
  retaining fail-closed parsing and selected-source ownership.
- Bump the rebuildable session parser version so an explicit `session scan`
  repairs existing indexes after upgrade.
- Preserve empty metadata only when the selected source genuinely has no
  model-bearing record; the interactive presentation task renders that state as
  explicit `unknown`.
- Add parser, incremental re-read, List, Show, and source-priority regression
  coverage.
- Verification level: L2 persisted/index contract; targeted package tests during
  development, aggregate L2 after the final relevant task.
- Commit boundary: `fix: index Claude session models`.

### `usage-visual-system`

- Replace the gray-dominant palette with capability-aware bright semantic roles
  for brand, active tab, selection, primary metrics, client/model identity,
  success, partial/unpriced/stale, and failure.
- Remove unconditional bar tracks. Preserve share, peak, coverage, and cache-hit
  bars only when their basis is explicit.
- Generate Activity for every valid report and add a first-class `ACTIVITY`
  interactive section with explicit heatmap unit and legend.
- Keep no-color glyph/label equivalence and JSON value/shape invariance.
- Add pure renderer/state, width, no-color, short/empty/hourly range, heatmap,
  PTY lifecycle, and visual-role tests.
- Verification level: L3 interactive terminal behavior.
- Commit boundary: `feat: revitalize usage terminal visuals`.

### `session-interactive-experience`

- Rebuild the root browser with explicit headers, model/unknown state, project,
  last activity, stable selection, and responsive selected-session preview.
- Rebuild detail Overview, Documents, Activity, and Tokens as structured rows
  with section-local page, selection, viewport, summary, and selected detail.
- Add bounded document expansion and invocation detail while preserving privacy
  and lazy paging.
- Reuse the shared bright palette and semantic primitives, including
  capability/no-color fallbacks and durable status/warning states.
- Preserve browser-to-detail Escape behavior, direct-detail exit behavior,
  standalone Escape decoding, resize recovery, EOF/Ctrl-C/error cleanup, and
  alternate-screen restoration.
- Verification level: L3 interactive terminal behavior and privacy.
- Commit boundary: `feat: redesign session interactive experience`.

### `session-show-readability`

- Replace unbounded document and invocation ASCII grids with a bounded,
  responsive text hierarchy. Remove redundant identity columns and move
  secondary token/cost fields into adjacent continuation lines.
- Keep redirected text deterministic, copyable, color-free, and at or below its
  selected visible width; preserve JSON exactly.
- Reconcile `docs/specs/cli-manual.md`, the terminal-rendering contract, and
  `docs/README.md` with all delivered behavior.
- Run final aggregate L2/L3 verification and compiled-current-binary isolated
  HOME PTY acceptance for Usage and Session before Review PASS.
- Verification level: L3 aggregate terminal and shared CLI contract.
- Commit boundary: `fix: make session text output readable`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| `session-model-index` | [x] | [x] |
| `usage-visual-system` | [x] | [x] |
| `session-interactive-experience` | [x] | [x] |
| `session-show-readability` | [x] | [x] |

Each task follows Development -> Review -> Repair -> Re-review -> PASS. Review
records live under `docs/reviews/rc3-terminal-ux-remediation/`. A Review finding,
including a non-blocking or nit finding, reopens the task until it is fixed and
the next round records `Verdict: PASS`.

## Delivery after all tasks pass

1. Retire this plan and its review directory with the final task.
2. Verify commit messages, Codex trailers, SSH signatures, clean status, branch,
   remote, and exact commit range.
3. Push `main`.
4. Record or reuse only still-valid isolated-real-state evidence for the exact
   affected behavior, then dispatch manual `release-preflight` for the pushed
   SHA and evidence ID.
5. Require successful same-SHA preflight evidence before creating an annotated
   `v0.4.0-rc.3` tag.
6. Push the tag, wait for GitHub prerelease assets and checksums, then wait for
   the RC Homebrew formula PR and its verification.
7. Merge the verified Homebrew PR, upgrade the local `agentdeck-rc` formula,
   verify installed version/commit/completions and the affected real-data Usage
   and Session surfaces, then stop for human acceptance.

## Starting task

Start with:

```text
进入开发：`rc3-terminal-ux-remediation` / `session-model-index`
```

Read `AGENTS.md`, this plan, the Session Search contract in
`docs/specs/cli-design.md`, and verification routing. Tick `Dev` only after the
task's targeted verification passes. Record a formal review round before
ticking `Review` or committing.
