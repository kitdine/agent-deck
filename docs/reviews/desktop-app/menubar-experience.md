---
status: active
plan: desktop-app
task: menubar-experience
---

# Review log — desktop-app / menubar-experience

## Round 1 — 2026-08-15

- Reviewed state: `docs/specs/menubar-experience-design.md`, uncommitted working
  tree; no menu-bar implementation exists yet
- Reviewer: claude-code
- Scope: the design contract only. Method limitation stated below.
- Method limitation: this round originated from a Dieter Rams UI audit, whose
  scoring anchors assume an implemented visual surface. Four of its ten
  principles had no measurable basis against an unimplemented contract, and its
  aesthetic principle scored 0 purely because no prototype exists, which its own
  rules treat as a redesign trigger. The numeric total is therefore not carried
  forward and the audit artifacts were removed. Only findings that survive
  independent verification against the repository are recorded here.

- Findings:
  - [P1] The design does not specify how the GUI invokes a provider switch.
    Verified: `agentdeck provider use` has no command-local quiet flag; only the
    global `--quiet` exists. The design names no success/error JSON envelope, no
    Swift operation owner, and no serialization, cancellation, double-submit, or
    result-lifetime rule. -> Specify the exact invocation and result contract.
  - [P2] `stale` and `offline` wording appears without a state definition, and
    the design does not distinguish a recurring update check from a manual one
    in user-visible copy. -> Define the interaction-state and copy matrix for
    both `en` and `zh-Hans`, including confirmation, success, failure, disabled,
    and focus behavior.
  - [P2] No implementation-ready visual contract: native semantic type and
    spacing choices, numeric width/height and narrow bounds, scrolling or
    collapse rules, section grouping, focus and disabled treatment, and contrast
    acceptance are unspecified. -> Add them, or state that they are deferred to
    the implementation task with an acceptance gate.
  - [P2] Partial-failure presentation is unspecified. When candidate discovery
    fails, currently selected provider routes should stay visible; `ready: false`
    needs a localized reason and disabled behavior; growing health and warning
    content needs a bound or collapse rule. -> Specify degraded presentation.
  - [Resolved] The design's opt-in update check contradicted the authoritative
    connectivity policy, which permitted network access only for a price update.
    `docs/specs/cli-design.md` and `AGENTS.md` now permit both, recording that
    the desktop check is opt-in, defaults off, sends no local state, and only
    opens the official release page. No design change required.

- Evidence: `agentdeck provider use --help`, `agentdeck --help`,
  `docs/specs/cli-design.md` connectivity constraint, `AGENTS.md` allowed
  connectivity, `docs/specs/menubar-experience-design.md` lines 33, 64, 162
- Verdict: REOPEN

## Round 2 — 2026-08-16

- Reviewed state: `docs/specs/menubar-experience-design.md`, uncommitted working
  tree
- Reviewer: claude-code (repair round, same owner — an independent Review round
  is still required before the plan's `Review` cell may be ticked)
- Scope: the four Round 1 findings, plus the UI, UX, and UED specification the
  design had been missing entirely.

- Round 1 findings, dispositions:
  - [P1] Switch invocation and result contract -> **Fixed.** Added a result
    envelope section: outcome is decided by the presence of `error` and by
    nothing else, `MenuBarSwitchOperation` owns one attempt per client, and
    serialization, double-submit impossibility, non-cancellation, and result
    lifetime are specified.
  - [P2] `stale` / `offline` wording and update-check copy -> **Fixed.** Added an
    interaction-state and copy table in both languages. The user-visible wording
    no longer says "stale"; it states when data was updated. `offline` names the
    helper rather than the network. Manual and automatic update checks read
    differently, and a silent automatic no-op is required.
  - [P2] Missing visual contract -> **Fixed.** Added a menu-bar item section and
    a visual contract with geometry (340 pt, 280 pt narrow bound, 560 pt height
    cap), semantic type styles, a 4 pt spacing scale, semantic status colors each
    carrying a symbol and a label, and density bounds for the two unbounded
    sections.
  - [P2] Partial-failure presentation -> **Fixed.** Candidate discovery failure
    with readable routes now keeps `available: true` and shows current routes,
    with switching visibly disabled instead of the section disappearing.
    `ready: false` candidates are listed with a localized reason.

- New findings, raised during repair:
  - [P1] `provider use` exits `0` on a failed switch while reporting
    `error.code: runtime_error`. Verified: `agentdeck --format json provider use
    nonexistent-xyz --client codex` returns exit `0`, whereas `session show` with
    a missing id returns exit `1`. A consumer reading the exit status would treat
    the failure as success. -> Recorded in the design as a CLI prerequisite; the
    host is specified to ignore exit status entirely. The CLI defect itself is
    out of this task's scope and needs its own fix.
  - [P2] That same failure carries `sql: no rows in result set` in
    `error.message`, and `runtime_error` is not defined as a stable code in
    `docs/specs/cli-design.md`. -> The design forbids displaying or logging the
    raw message and requires a generic localized explanation beside the verbatim
    code.

- Evidence: `agentdeck --format json provider use nonexistent-xyz --client
  codex` (exit 0, `runtime_error`), `agentdeck --format json session show
  nonexistent-id` (exit 1), `agentdeck --format json provider current` envelope
  keys, `grep runtime_error docs/specs/cli-design.md` (no definition)
- Verdict: REOPEN — repair complete, awaiting independent review

Round 1's connectivity-policy finding was resolved in the authoritative documents
rather than in the design, and is closed. The two CLI defects found in Round 2 are
prerequisites recorded for their own fix; the design works correctly around them
without hiding them.
