---
status: active
created: 2026-08-25
updated: 2026-09-02
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

- **Latest prerelease:** [`v0.5.0-rc.5`](https://github.com/kitdine/agent-deck/releases/tag/v0.5.0-rc.5)
  at commit `dd09acc96ad12b94e67a32bea30ba2007cfdd769`, tree `32773386`,
  published 2026-09-02. It is the fifth `v0.5.0` candidate and the first whose
  local `make release-verify` passed as a single command rather than as a
  CI/local split. [Release run 33658932481](https://github.com/kitdine/agent-deck/actions/runs/33658932481)
  succeeded on all four jobs and published six artifacts: the two CLI archives
  with checksums, and the universal DMG and ZIP with their own checksum file.
  Tap PRs [#24](https://github.com/kitdine/homebrew-tap/pull/24) for the
  `agentdeck-rc` formula and [#25](https://github.com/kitdine/homebrew-tap/pull/25)
  for the `agentdeck-app-rc` cask were merged as `cb31f5b` and `09a039a` after
  all three of their `sha256` values were compared against the published
  checksum files and against a locally recomputed hash of the DMG. Both rc
  channels are installable.
- The `v0.5.0` CEv1 Release boundary is `VERIFIED` for tree
  `32773386ad632a593e63d005ba57b04e7eebc83d`: both required criteria carry one
  `pass` record each — the aggregate L4 gate, and
  [exact-SHA preflight run 33657506153](https://github.com/kitdine/agent-deck/actions/runs/33657506153)
  with both jobs successful. Each supersedes the `rc.4` record it replaces.
- **`rc.5` exists because `rc.4` published cleanly and then failed in use.**
  The first three candidates each failed one step further along a publication
  path that had never run end to end: `rc.1` could not sign, `rc.2` was refused
  by Gatekeeper, `rc.3` collided two asset names. `rc.4` published all six
  artifacts and moved the failures one stage later still — into what a real
  install does with what was published, which found eight defects. Five are
  fixed in `rc.5`: the DMG shipped an unstapled bundle, the Cask exclusion guard
  refused and left a Caskroom receipt anyway, a leftover `state.lock` locked the
  CLI out permanently, determinable attribution was reported as inferred, and
  Claude startup sessions lost their route. Each carries a reviewed record under
  [`archive/fixes/`](archive/fixes/).
- **The notarization ticket now survives the Cask install, measured rather than
  inferred.** The stapled ticket for an app bundle is `Contents/CodeResources`,
  1800 bytes with the `s8ch` magic. Both published artifacts were checked, not
  only the one the Cask installs: it is present in the `rc.5` DMG's inner
  bundle, in the bundle unpacked from the published ZIP, and in the copy
  Homebrew placed in `/Applications`, and absent at that path in `rc.4`'s inner
  bundle. The DMG and ZIP checksums both match
  `AgentDeck_v0.5.0-rc.5_desktop_checksums.txt` and `xcrun stapler validate`
  passes on both. Ticket presence is a filesystem fact, so it depends on
  neither network state nor the Gatekeeper assessment cache. The `rc.4` baseline
  was reproduced live on the same machine first: its installed copy reported no
  stapled ticket while `spctl` still accepted it, because the machine was
  online. With Wi-Fi off and three Apple endpoints proven unreachable over real
  HTTP, the upgraded app launched and stayed running with no Gatekeeper alert.
  **That launch alone does not attribute the result to the ticket** — this
  machine had already assessed the signature online during the install, so a
  cache-free differential would need a machine that has never seen it. The
  acceptance record is `docs/fixes/staple-offline-first-launch.md` on
  `ad-verify-staple-offline-first-launch`. The notarization acceptance remains
  measured, and its record reached `PASS` at Round 5 with all six required
  completion criteria recorded `pass`; that round's partial independence is
  stated in the record itself.
- The `rc.5` scan-performance fix was confirmed on the real store, read-only:
  `usage summary --no-scan` completed in 2.28s over 96,874 events against the
  fix record's 2.4s and a pre-fix 9m18s.
- **The attribution fix is confirmed for Codex; Claude remains fail-closed
  pending better evidence.**
  Whole-history counts are the predicted shape — 58,758 `exact` /
  18,009 `estimated` / 20,193 `unattributed`, every one of the last from
  `before_adoption`, with `coverage_gap` at 0 — but that is an event count
  over all time and answers for no surface anyone sees. The surface the
  `attribution-determinability` record names is the menu bar's `today` panel,
  which shares by **cost**, not by event count. Measured 2026-09-02 20:32 PDT
  it reads 33.90% determinable, and split by client the two halves are
  nothing alike: Codex is 100.00% determinable (590 of 590 events), Claude is
  1.16% (22 of 803). Those 22 are the first `claude|startup` route this store
  has ever held, written at 2026-09-03T03:26:37Z by an `rc.5` interactive
  Claude Code session. Its task-owned acceptance is now recorded in
  [`claude-startup-route-live.md`](fixes/claude-startup-route-live.md): every
  invocation present in its timestamped acceptance snapshot reads determinable,
  and the desktop 7d determinable bucket matches that snapshot's session events,
  tokens, and cost. The sample remains one qualifying startup; the record
  reached `PASS` at Round 2 with all six required completion criteria recorded
  `pass`.
  Of 47 route-less Claude sessions in that snapshot, 46 are claude-mem
  `observer-sessions/` SDK sessions with one event each; the other two started
  before `rc.5` was installed and hold 675 events. Those events remain
  `inferred` in the released `rc.5` behavior under the reviewed fail-closed
  contract. The Lane A repair on `ad-bug-claude-no-route-quality`, which derives
  a process boundary from transcript timestamps, is committed at `be7da8b`
  after Re-review Round 7; two further Lane A repairs followed it on `main` —
  `5ff2e80` removes the unmanaged `ANTHROPIC_API_KEY` from Claude redacted
  backups, and `29039f9` admits `clear` and `fork` session starts and
  brand-new-project transcripts without weakening root containment. All three
  are on `main` only: none of them changes the released `rc.5` behavior, this
  release status, or the notarization acceptance, and reaching users needs a
  later candidate.
- The Cask exclusion guard was not exercised live, because reproducing it means
  installing the conflicting CLI-only formula; it rests on the real
  `brew install --cask` regression that passed in this commit's L4 aggregate.
- The published release notes were written before the release ran and therefore
  understate what is now verified. They are left as tagged, because the release
  body is published from the tag annotation and editing one without the other
  splits the record in two.
- Three of the eight `rc.4` findings are carried rather than fixed: the
  remaining hook-transcript admission edges, actionable recovery guidance for
  `state_busy`, and the schema-version signal — the one defect whose repair
  requires deciding new user-visible behaviour, which is why it is a topic
  rather than a fix.
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
| [`v0.5.0` Contract Closure](topics/v0-5-0-contract/tasks.md) | `v0.5.0` | Contract complete, reviewed, and committed at `a547362` — `cli-design.md` is at version 28, the five selected topics are retired under [`archive/topics/`](archive/topics/), and both contract Tasks have passed independent review. 2/2 implemented and reviewed. Five release candidates have since been published from `main`, the current one being `v0.5.0-rc.5`; stable channel selection and publication remain separate decisions. The specification stays at version 28 across the five Lane A fixes `rc.5` carries: only one of them touched `cli-design.md`, and it corrected an untrue paragraph about notarization submissions without changing the promise that paragraph supports, which the Changelog rule makes a correction rather than a contract change. The untracked schema-version-signal topic remains separate and unstaged | Version-wide specification raise and documentation reconciliation after every selected topic's tasks pass review. |
| [Schema Version Signal](topics/schema-version-signal/tasks.md) | unassigned | Active — boundary stage; `requirements.md` drafted and awaiting review; version membership not yet decided by a contract topic, so this row is deliberately not marked `v0.5.0`; the Documents matrix declares `ux/menubar-schema-signal.md` and `architecture.md` as required and unwritten, which `check-topic-docs.sh` reports as gaps until `tasks.md` review ratifies the set; 0/4 documents passed; `requirements.md` was widened on 2026-09-04 while in review, after a triage found a sixth reporting path — the Hook — and it is the one that reports nothing at all | One stable, actionable report when the core database's schema version exceeds the running binary's supported version, across `doctor`, the command paths, the desktop snapshot, and Hook delivery. Promoted from a measured defect: schema 21 on disk against two installed binaries supporting 18, reported six different ways. The sixth is silent: `usage hook event` exits 0 with empty streams and writes no row, which cost 19 Codex sessions their provider route between 2026-08-30 and 2026-09-01, unrecoverably. |

#### Retired into `v0.5.0`

These five topics reached their terminal reviewed state, were retired by the
`v0-5-0-contract` closure task on 2026-09-01, and now live under
[`archive/topics/`](archive/topics/) with every document marked
`status: historical`. They remain `v0.5.0` members: retirement moves the
documents, not the version membership.

| Topic | Version | Status | Purpose |
| --- | --- | --- | --- |
| [Native macOS Desktop App](archive/topics/desktop-app/tasks.md) | `v0.5.0` | Retired 2026-09-01 into `v0.5.0`; documents are historical under `archive/topics/`. Delivered — 6/6 tasks reviewed and committed; the immutable commit-tree CEv1 Task and Plan gates are VERIFIED at `0aefed1`; version-wide contract closure and any release action remain separate | macOS 26 menu-bar app, settings window, WidgetKit extension, unified desktop distribution, Cask, and direct-download delivery. |
| [Work Signals](archive/topics/work-signals/tasks.md) | `v0.5.0` | Retired 2026-09-01 into `v0.5.0`; documents are historical under `archive/topics/`. Complete — prototype task 0 and implementation tasks 1–6 are independently reviewed and committed. Final Task 6 is signed commit `a83ae2b` / tree `45094755`; its immutable Task gate is VERIFIED 5/5, and the `work-signals` Topic gate is VERIFIED 4/4 by rolling up all five Document gates and Tasks 0–6. Superseded Review failures remain immutable evidence; actual VoiceOver/TCC/system accessibility automation remains explicitly not run and not required. Push and the separate `v0.5.0` contract closure are not part of this checkpoint | Activity classification, workflow metrics, and tool-call attribution on two first-class surfaces: the menu-bar `Sessions` panel's three captured modules and `agentdeck usage signals`. |
| [Usage Attribution Precision](archive/topics/usage-attribution-precision/tasks.md) | `v0.5.0` | Retired 2026-09-01 into `v0.5.0`; documents are historical under `archive/topics/`. Complete — 3/3 tasks reviewed and committed at `9035b80` / tree `d3ffe3ac`; immutable Task and Topic CEv1 gates VERIFIED; v0.5.0 contract closure remains separate and not started; release blockers held: no determinable event is downgraded to `inferred`, and no unattributed event enters provider spend | Effective-session attribution semantics, determinability-based quality, and an unattributed boundary that never enters a real-spend total. |
| [CLI Error Classification](archive/topics/cli-error-classification/tasks.md) | `v0.5.0` | Retired 2026-09-01 into `v0.5.0`; documents are historical under `archive/topics/`. Complete — 2/2 tasks reviewed and committed; immutable Task and Topic CEv1 gates VERIFIED at `574a7ad` / tree `6d26f205`; v0.5.0 contract closure remains separate and not started | Stable not-found codes, and no storage text in a documented JSON contract. Breaks the documented `runtime_error` value; announced in this version's notes. |
| [Switch Effectiveness Boundary](archive/topics/switch-effectiveness-boundary/tasks.md) | `v0.5.0` | Retired 2026-09-01 into `v0.5.0`; documents are historical under `archive/topics/`. Active — all three design documents PASS; implementation tasks 1–3 PASS; `real-session-acceptance` waived by the operator 2026-08-26 (not executed, no review record); all 3 code-bearing tasks reviewed | Every accepted Codex or Claude Hook delivery uses one observation/transaction pipeline; effective-route effects remain event-specific, including Claude's sole live `no key -> first key` transition. |

**`v0.5.0` contains exactly the rows marked `v0.5.0` above**, in both the active
and the retired table, plus the contract closure that reconciles them. The count
is deliberately not written here: it was
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
lives in [`archive/topics/desktop-app/tasks.md`](archive/topics/desktop-app/tasks.md) and
[`archive/topics/desktop-app/reviews/`](archive/topics/desktop-app/reviews/).

Two corrections were made on 2026-08-20, both cross-topic and both recorded here
because they change version membership and process, not because they narrate a
round.

**Work signals are back in `v0.5.0`.** The 2026-08-18 re-open recorded the three
modules as moved to the Backlog and as "refused" by the desktop boundary. That
cut a committed feature out of the version without asking, and the stated ground
— that no field exists behind them — was only partly true: `internal/activity`
already extracts tool calls and `usage_tool_calls` has persisted them since
schema v13. The capability is restored as its own topic,
[`work-signals`](archive/topics/work-signals/tasks.md), which supplies the data, turns
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
[`archive/topics/desktop-app/tasks.md`](archive/topics/desktop-app/tasks.md). As of 2026-08-23
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
