---
status: historical
topic: work-signals
subject: work-signal-surface acceptance
retired: 2026-09-01
---

# `work-signal-surface` acceptance — 2026-08-31

## Candidate and environment

- Candidate: HEAD `7a1160a86e549a3ae3532bbfe8b782fdbfbfef82` plus the
  uncommitted Task 5 scope; the final scoped manifest is recorded only after
  task/status synchronization.
- Host: macOS 26 (`25G220`).
- Toolchain: Xcode 26.4 (`17E192`) selected per command with
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`.
- Safety: every view uses synthetic wire data. No real AgentDeck, provider,
  client, login-item, TCC, system language, appearance, or accessibility state
  was read or changed.

## Automated and rendered acceptance

| Check | Result |
| --- | --- |
| Captured summaries follow the selected Client × Period item | PASS — model tests cover `today/all` and `30d/codex` |
| Activity fixed order, one expanded row, subcategory order and zero-row omission | PASS |
| Workflow nullable values, measured zero, rework note and base-name-only top file | PASS |
| Tooling call order with `other` last, textual share and Top MCP | PASS |
| Empty scope versus legacy/unavailable families | PASS |
| `turn`, `partial`, and `none` cost-basis presentation | PASS |
| English and Simplified Chinese catalog inventory | PASS |
| 420 pt summary in Light and Dark | PASS — XCTest PNG attachments |
| 280 pt summary, Activity expanded detail, Workflow, Tooling, and legacy detail | PASS — English and Simplified Chinese XCTest PNG attachments visually inspected |
| Actual VoiceOver speech, TCC, or system accessibility-setting automation | NOT RUN — explicitly not required by operator decision |
| Complete App and Shared XCTest targets, explicit English locale | PASS |
| Repository-wide Go L2 | PASS |

The 280 pt refinement uses the prototype header with its disclosure chevron
when it fits and a title-preserving fallback without the redundant chevron when
it does not. Both variants remain buttons and expose the same detail action.
Shares are present as text as well as bars. The captured Activity detail uses a
native `DisclosureGroup`, so expanded state and parent-then-child ordering are
provided by the SwiftUI accessibility element rather than inferred from colour
or geometry.

## Accessibility acceptance boundary

Implemented and checked without system changes:

- summary cards are keyboard-focusable buttons;
- entering a detail assigns accessibility focus to its heading;
- Back returns keyboard and accessibility focus to the opening card;
- only one Activity category can be expanded, and its non-empty subcategories
  immediately follow it in view order;
- every share, cost, event count, and tool-call share has textual output.

The pure navigation regression proves the opening-card identity survives the
detail round trip. The rendered and model checks do not claim to be actual
VoiceOver speech or a real accessibility-focus traversal.

### Explicit non-execution disposition

By operator decision on 2026-08-31, actual VoiceOver reading, TCC changes, and
system accessibility-setting automation are **not performed and are not Task 5
completion requirements**. This acceptance passes on the safe structural,
textual, navigation-state, bilingual, appearance, and narrow-layout evidence
above. No claim is made that VoiceOver/TCC behavior was executed or observed.
