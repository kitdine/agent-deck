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
  - [P2] A failed switch for an unknown provider reports `error.code:
    runtime_error`, which appears zero times in `docs/specs/cli-design.md` and
    `docs/specs/cli-manual.md`, so it is not a stable code a consumer can map.
    Its `error.message` carries the underlying storage text `sql: no rows in
    result set`. -> Recorded in the design as a CLI prerequisite; the host is
    forbidden from displaying or logging the raw message and shows the verbatim
    code beside a generic localized explanation. The CLI defect itself is out of
    this task's scope and needs its own fix.
  - [Withdrawn] An earlier draft of this round claimed `provider use` exits `0`
    on failure. That was a measurement error: `exit=$?` had read the exit status
    of a `head` process in a pipeline rather than of `agentdeck`. Re-measured
    without a pipeline, the command exits `1`, consistent with `session show`.
    The design text asserting otherwise was corrected in the same pass.

- Evidence: `agentdeck --format json provider use nonexistent-xyz --client codex`
  writes the envelope to stderr and exits `1`; `error.code` is `runtime_error`
  with message `sql: no rows in result set`; `grep -c runtime_error
  docs/specs/cli-design.md docs/specs/cli-manual.md` returns `0` for both;
  `agentdeck --format json provider current` envelope keys are `command`, `data`,
  `generated_at`, `partial`, `schema_version`, `warnings`
- Verdict: REOPEN — repair complete, awaiting independent review

Round 1's connectivity-policy finding was resolved in the authoritative documents
rather than in the design, and is closed. The two CLI defects found in Round 2 are
prerequisites recorded for their own fix; the design works correctly around them
without hiding them.
