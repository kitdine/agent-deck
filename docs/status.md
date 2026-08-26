---
status: active
created: 2026-08-25
updated: 2026-08-25
---

# AgentDeck Project Status

This file is the concise authority for the current release and cross-topic
execution status. Topic-internal document/task cells, rounds, findings, and
evidence remain in each topic's `tasks.md` and `reviews/`. Active version
membership is decided by the applicable `vX-Y-Z-contract` topic; the Version
column below is its current execution-status projection. Later version direction
is recorded in `roadmap.md`.
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
| [Native macOS Desktop App](topics/desktop-app/tasks.md) | `v0.5.0` | Delivered — 6/6 tasks reviewed and committed; the immutable commit-tree CEv1 Task and Plan gates are VERIFIED at `0aefed1`; version-wide contract closure and any release action remain separate | macOS 26 menu-bar app, settings window, WidgetKit extension, unified desktop distribution, Cask, and direct-download delivery. |
| [Work Signals](topics/work-signals/tasks.md) | `v0.5.0` | Active — all five design documents PASS after three combined rounds; prototype task done, implementation task 1 next; 0/6 implementation tasks | Activity classification, workflow metrics, and tool-call attribution, delivered on two first-class surfaces: the menu-bar `Sessions` panel's three pending modules and a new `agentdeck usage signals`. |
| [`v0.5.0` Contract Closure](topics/v0-5-0-contract/tasks.md) | `v0.5.0` | Active — 0/2 done | Version-wide specification raise and documentation reconciliation after every selected topic's tasks pass review. |
| [Usage Attribution Precision](topics/usage-attribution-precision/tasks.md) | `v0.5.0` | Active — 0/3 done; keep the topic unchanged until development, then reconcile only attribution-related v0.5.0 Backlog items with it; release blocker: a determinable event must never be downgraded to `inferred` | Per-client attribution time semantics, determinability-based quality, and an unattributed boundary that never enters a real-spend total. |
| [CLI Error Classification](topics/cli-error-classification/tasks.md) | `v0.5.0` | Complete — 2/2 tasks reviewed and committed; immutable Task and Topic CEv1 gates VERIFIED at `574a7ad` / tree `6d26f205`; v0.5.0 contract closure remains separate and not started | Stable not-found codes, and no storage text in a documented JSON contract. Breaks the documented `runtime_error` value; announced in this version's notes. |
| [Switch Effectiveness Boundary](topics/switch-effectiveness-boundary/tasks.md) | `v0.5.0` | Active — all three design documents PASS; implementation tasks 1–3 PASS; `real-session-acceptance` waived by the operator 2026-08-26 (not executed, no review record); all 3 code-bearing tasks reviewed | Every accepted Codex or Claude Hook delivery uses one observation/transaction pipeline; effective-route effects remain event-specific, including Claude's sole live `no key -> first key` transition. |

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

## Known Residual Risk

- Plaintext credential values and derived key bytes are not reliably zeroed
  after use. Go's copying garbage collector and immutable `string` values make a
  complete wipe guarantee unavailable; this remains an accepted residual risk.
