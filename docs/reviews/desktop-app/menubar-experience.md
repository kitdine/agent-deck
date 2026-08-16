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

The four open findings belong to the `menubar-experience` design task and are
left to its owner; this review changed no design content. Only the connectivity
policy conflict was resolved here, because it was a defect in the authoritative
documents rather than in the design.
