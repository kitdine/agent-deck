---
status: active
created: 2026-09-21
updated: 2026-09-22
---

# Health Recovery — Tasks

This is the topic-local execution and status authority for `health-recovery`.
Its planning carrier is `ad-health-recovery`; the retained defect origins are
`ad-bug-state-busy-recovery-guidance` and
`ad-bug-extension-stale-mcp-recovery`.

Workspace: `agent-deck.health-recovery`; branch `feature/health-recovery`;
creation base `a396c2f2158f579e70da3aaf9bd0bff084fc1f1a`. Design, implementation,
Git delivery, integration, retirement and release remain separate boundaries.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli-health-recovery.md | [x] | [x] |
| ux/menubar-health-recovery.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [ ] |

The document set keeps CLI and menu-bar interaction contracts independently
reviewable while one architecture owns shared reason/action semantics and state
transitions. Both origin bugs remain independent acceptance tracks; neither is
closed merely because this topic or its requirements are approved.

## Design progression

```text
requirements.md
   ├─> ux/cli-health-recovery.md ─────┐
   └─> ux/menubar-health-recovery.md ─┼─> architecture.md
                                      ├─> final UX reconciliation/review
                                      └─> tasks.md decomposition/review
```

- `requirements.md` owns scope, safety, action taxonomy, surfaces, exclusions
  and acceptance scenarios.
- `ux/cli-health-recovery.md` owns terminal text/JSON states, command semantics
  and the operator sequence for both lock and extension recovery.
- `ux/menubar-health-recovery.md` owns the native health-row hierarchy, labels,
  copied commands/manual prerequisites, bilingual copy and accessibility.
- `architecture.md` owns typed reason/action contracts, lock inspection and
  doctor read-only boundaries, extension inventory synchronization, ordering,
  compatibility and test seams.
- This file receives implementation tasks only after the documents above pass
  their applicable reviews. No implementation task or Development Gate is
  authorized by this initial matrix.

## Tasks

Implementation decomposition is intentionally not populated during boundary
design. The reviewed final CLI and menu-bar surfaces plus architecture determine
whether shared contract work and the two cause-specific recovery tracks form
two or more implementation tasks. Do not create implementation Beads tasks from
the planning carrier before this document passes decomposition review.

## Current handoff

The requirements boundary passed Review Round 1 and its signed delivered
document gate is VERIFIED. CLI framework Re-review Round 2 passed for content
state `urn:ce:agent-deck:content-state:health-recovery:ux-cli-health-recovery:7537f52:1d95ea77b6fff6ac58ad65424f1b546a4d9f262e7bd70a7e53978fc49a2fa7de`;
all five Round 1 findings are closed and the document gate is VERIFIED. The
document was delivered by signed commit `bc6df888edb9dd342129865184ff0ca426819675`;
its completed task state is unchanged. `ux/menubar-health-recovery.md` framework
Re-review Round 3 passed for content state
`urn:ce:agent-deck:content-state:health-recovery:ux-menubar-health-recovery:bc6df88:6a83f9435c75f17b4ebfb6769855e0be0a06099a8b51d73f222f709bad08a9c0`
and closed `MBHR-R1-F1`; it supersedes the unbound Round 2 entry, and the
document gate is VERIFIED; it was delivered by signed commit
`29694de77343bfa623ad04fa21c0e581564cda32` together with its shared prototype
changes. `architecture.md` Re-review Round 4 passed for content state
`urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:87b6094fbb40f55784ebd5a2f9deb2872613e2e9a22096a35afed7a43b9ceef7`;
every finding recorded in [`reviews/architecture.md`](reviews/architecture.md)
is closed and the document gate is VERIFIED. The document awaits separately
authorized Git delivery. Final UX reconciliation and `tasks.md` decomposition
are next.
Implementation remains blocked on its declared dependencies. Git delivery,
integration, retirement and release remain separate boundaries.

## Review boundary

The initial `tasks.md` review later ratifies the final document set and, during
decomposition, verifies that every reviewed requirement and architecture
contract maps to exactly one implementation owner with named files and
proportionate verification. The current draft declares documents only and does
not substitute for requirements review or authorize implementation.
