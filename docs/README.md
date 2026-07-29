---
status: active
created: 2026-07-14
---

# AgentDeck Documentation

This is both the documentation index and the execution baseline. Decide what to
work on next from this file. Repository code, tests, configuration, and Git
history remain the source of truth when they disagree with any document.

## Current State (2026-07-28)

`v0.2.0` is now the current stable release. Its annotated tag points to
`8c053c9df53f9aad0797d3e9bf2d307fd203ab8a`; the tagged tree differs from
`v0.2.0-rc.2` only by the RC validation documentation commit, with no
product-code change after RC2. The
[tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/30423888201)
passed both release and Homebrew jobs, published a non-prerelease GitHub Release
with both Darwin archives and the checksum asset, and verified the stable
formula through an isolated install, `brew test`, and bash, zsh, and fish
completion loading. [Homebrew tap PR #5](https://github.com/kitdine/homebrew-tap/pull/5)
merged the checked formula into `Formula/agentdeck.rb`; the separate RC formula
remains unchanged. The local channel transition was also completed:
`brew uninstall kitdine/tap/agentdeck-rc` removed `0.2.0-rc.2`, and
`brew install kitdine/tap/agentdeck` installed stable `0.2.0`. The installed
binary reports `v0.2.0` at commit
`8c053c9df53f9aad0797d3e9bf2d307fd203ab8a`, and the bash, zsh, and fish
completion files are all present in Homebrew's standard paths.

v0.1.1 is published and installable through `kitdine/homebrew-tap`. Every
follow-up in the retired phase-one plan passed independent review, so there is
no outstanding review debt.

The v0.1.1 release exercised the automated Homebrew tap-PR flow end to end for
the first time: pushing the annotated tag ran the `Release` workflow to success,
published the GitHub Release, and opened `kitdine/homebrew-tap#1` as an
`agentdeck-v0.1.1` pull request (not a direct push) whose runner step verified
bash, zsh, and fish completion install. After that PR merged, a local
`brew reinstall kitdine/tap/agentdeck` upgraded to v0.1.1 with all three
completions present. This closes the previously open tap-automation
verification task.

The repository now defines an opt-in Homebrew release-candidate channel at
`kitdine/tap/agentdeck-rc`. Stable releases continue to update only
`agentdeck`; strict `vX.Y.Z-rc.N` tags render and install-test
`agentdeck-rc` before proposing a tap PR that leaves the stable formula
unchanged. The first live validation is complete: the annotated
`v0.2.0-rc.1` tag points to `1a7a8424a204b8c3413da08ac454122e1b3250cf`;
the [tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/30340790652)
published a GitHub prerelease with both Darwin archives and their checksum
asset; and [tap PR #2](https://github.com/kitdine/homebrew-tap/pull/2) added only
`Formula/agentdeck-rc.rb`.

The first real channel switch then exposed a Homebrew 6 trust interaction:
`conflicts_with "agentdeck"` loaded the uninstalled stable formula, which
direct RC installation had not trusted. CLI design v18 removed that DSL
declaration while retaining the explicit uninstall/install switch. The
[correction run](https://github.com/kitdine/agent-deck/actions/runs/30344631360)
passed and [tap PR #3](https://github.com/kitdine/homebrew-tap/pull/3) merged the
one-line formula correction without changing the stable formula. After
uninstalling stable, a real `brew install kitdine/tap/agentdeck-rc` installed
`agentdeck-rc 0.2.0-rc.1`; `brew test kitdine/tap/agentdeck-rc` passed, and the
bash, zsh, and fish completions were present under Homebrew's standard paths.

The installed release binary also exercised provider wrapper routing in an
isolated home: a wrapper input ending in `/v1/` stored as the normalized
endpoint, `provider use --via` wrote the wrapper route and reported
`via_wrapper: true`, and a subsequent direct switch restored the upstream
endpoint with `via_wrapper: false` while preserving unrelated Codex history
configuration. This closes the first-install and released-wrapper validation.
The second live validation is also complete: annotated `v0.2.0-rc.2` points to
`c3abda13467ebea4792f262057148bfea1c205f5`; the
[tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/30352721360)
published the GitHub prerelease and passed both release and Homebrew jobs; and
[tap PR #4](https://github.com/kitdine/homebrew-tap/pull/4) updated only
`Formula/agentdeck-rc.rb`. A literal `brew upgrade kitdine/tap/agentdeck-rc`
upgraded the installed formula from `0.2.0-rc.1` to `0.2.0-rc.2`; `brew test`
passed, and the installed binary reports release version `v0.2.0-rc.2`, commit
`c3abda13467ebea4792f262057148bfea1c205f5`. This closes the RC-to-RC upgrade
validation.

Delivered and reviewed: the Go CLI baseline; provider and credential
management; usage collection, pricing, and run attribution; local session
search; extension inventory; encrypted backup and device migration; unified
ASCII table output; machine-bound encrypted SQLite credential storage;
automatic LiteLLM price updates; active-log-safe usage rebuild; the usage stats
runtime provider dimension; a measured 5.40x mean cold-scan speedup; delayed
stderr progress with parser-reread context; stored-aggregate `--no-scan`
reporting for stats and summary; and GitHub release plus Homebrew tap
distribution.

Per-task history lives in
[the retired phase-one plan](archive/plans/agentdeck-cli.md). Read it only for
historical detail; it is not a current tracker.

Usage-stats text readability is delivered and independently reviewed: bounded
rankings, a recent 48-bucket trend window, the shared `--top` override, and
width-aware detail compaction reduced the controlled fixtures from 139 to 120
lines and from 832 to 142 lines. Measurement and review history lives in the
[retired readability plan](archive/plans/usage-stats-readability.md).

Price catalog coverage is delivered and reviewed: the bundled catalog is now a
generated, reproducible artifact rebuilt from a pinned LiteLLM commit, carries a
content-derived version so a price change cannot ship under a reused version,
and merges a curated gap-fill that regeneration cannot drop. Cold-start coverage
on the same frozen snapshot went from 7.4% to 95.1% of tokens fully priced, and
`gpt-5.3-codex-spark` ships an explicitly disclosed equivalent estimate rather
than an invented or absent price. Measurement, the estimate's rationale, and
review history live in the
[retired price-coverage plan](archive/plans/price-coverage.md).

High-value test coverage is delivered and reviewed: focused regression tests
now pin the evidenced high-risk gaps — store open/backup/scan-lock boundaries,
provider configuration persistence, usage run-state attribution, provider
backup redaction and config retention, credential-vault initialization and
recovery, and read-only `session.CheckHealth` diagnostics. All six tasks passed
review; tasks 4 through 6 had their round-1 findings repaired in the same
session after the implementer subagents hit a session limit, a caveat recorded
inline in each review round. One low-risk behavior deviation is carried into
the Backlog below. History lives in the
[retired test-coverage plan](archive/plans/test-coverage.md).

Repository-wide behavioral test-gap closure is complete and delivered on
`main` at `9bb88477c9655a08a0dfd26bb00e20d433db251e`. Fifteen independently
reviewed logical tasks protect all 16 deterministic first-party modules; there
are no exclusions, unconfirmed modules, remaining `needs-tests` entries, or
open/awaiting-human blockers. Production code was unchanged by the test-only
workflow.

Eleven safe task commits were delivered in the earlier authorized partial
delivery. After the four separately reviewed production fixes landed on local
`main`, the workflow resumed from baseline
`4f614d34d09260a52df6bd333f6dad26134e96ac`. The four reconstructed regression
tasks were independently reviewed on the audit branch and projected as fresh
signed replacement-delivery commits:

| Task | Replacement delivery commit |
| --- | --- |
| Price refresh permanent failures, cancellation, and request counts | `39650636fc92f884ecda5081f5d28ec22b583153` |
| Canonical provider multiplier syntax | `3968d703fc5ed94378fbb917c187543655a1ffbb` |
| Generator commit resolution and output preservation | `02eec76513929fb321361858a00cc71d9ecad387` |
| Atomic session index transitions and exact source ownership | `7168079230adf8bb1fdf05b2d563f1f1782023e1` |

The frozen reviewed audit integration head is
`2571307d8410c2b4874bc1f8fb53fef91707c129`. Aggregate Review Round 3 passed
with exact 21-path scope, all source-to-audit manifest/message/signature
mappings, 16-module/15-task truth, and immutable failed-evidence retention.
The failed first delivery was preserved at
`725ab5aed94c3a38d7f9c8d7ebc8016e63569b33` through final review and
retirement; it was not amended, reset, or reused. Its temporary recovery
worktree and ref were removed only after the final archive reached `main` and
the push was verified.

Complete verification at replacement task head
`7168079230adf8bb1fdf05b2d563f1f1782023e1` passed full tests, the full race
suite, `go vet`, and atomic repository coverage. Total statement coverage is
81.9%; the delivery coverage profile SHA-256 is
`0ae5afc81ecbcae30fb747ea60b41f16e3570c1a3ea13722093660751627f54b`.

The completed plan and all fifteen task review records are committed under
`docs/archive/`. Archive Review Round 1 found and closed two documentation-only
accuracy issues: the state snapshot date and wording that prematurely implied
pending gates had proceeded. Delivery Aggregate Review Round 1 passed its
candidate, but those corrections changed final content identity, so fresh
Round 2 reviews ran. Archive Review Round 2 passed; Delivery Aggregate Review
Round 2 found that the now-completed replacement work was still dated
2026-07-25 even though the commits and delivery coverage were created on
2026-07-26 Asia/Shanghai. The final-state, replacement-review, and retirement
dates are now consistently 2026-07-26.

Archive Review and Delivery Aggregate Review Round 3 both passed, and the
reviewed delivery-state resolver authorized the final documentation commit,
plan retirement, target fast-forward, and cleanup. Final commit
`9bb88477c9655a08a0dfd26bb00e20d433db251e` is present on both local `main`
and `origin/main`. After the verified push, all temporary task, repair,
delivery, and audit worktrees and refs were removed; the repository now has
only the `main` worktree and branch.

Provider wrapper routing is delivered and reviewed, and its plan retired on
2026-07-27. Every provider, including the built-in `official`, may now carry one
wrapper URL (`provider set-wrapper`), and `provider use --via` routes a single
switch through it without storing an attachment — so a compression proxy can
front a relay while still writing that relay's own credential, or front a
subscription without handing AgentDeck a token. `official` became selectable for
Claude as well as Codex; a completed Claude switch reports on stderr that
running sessions should be restarted, plus any credential source AgentDeck does
not own that would override an `official` selection; and the route reaches usage
attribution as reported metadata that never becomes a grouping key. All seven
tasks passed independent review, three of them after a reopen — including a
`doctor` config-drift regression caught end to end. History lives in the
[retired plan](archive/plans/provider-wrapper-routing.md).

Two `cmd/agentdeck` tests that had failed for months on any host west of UTC
were diagnosed and fixed in the same session: their fixtures sat at the start
of a UTC day while `usage stats --from/--to` resolve local dates, so the
local-day window opened hours later and dropped them. Both now pin the zone
through the display-neutral `displayLocation` seam, and two guards keep the
contract honest — that dates are read in the configured zone, and that the
configured zone defaults to the machine's.

A second active plan, [project-attribution](plans/project-attribution.md), lets a
user mark a wrapper URL as speaking Headroom's protocol and have
`agentdeck run` label a launch with the project it happened in, with an
installable shell helper and a documented settings-file recipe covering
launches AgentDeck does not make. Its first four tasks are delivered and
independently reviewed: `headroom-wrapper-kind` adds the explicit protocol
declaration, and `project-identity` shares the session parser's cleaned
full-path identity while exposing only a safely encoded base name on the wire;
`run-env-injection` attributes eligible AgentDeck-launched Codex and Claude
processes; and `attribution-guidance` points successful Headroom wrapper actions
to the project-owned manual without changing JSON or quiet output. Two tasks
remain, starting with `shell-helpers`. AgentDeck writes no file it does not
already own; the app questions sit in the Backlog below.

The first task in the older active plan,
[display-timezone](plans/display-timezone.md), is implemented and independently
reviewed. `display-clock` extracted the shared zone, timezone-name, and
RFC3339/`time.Time` rendering helpers without wiring them into a renderer, so
no command output changed. Storage and JSON remain UTC under
`specs/cli-design.md` v19. Next is `provider-and-session-surfaces`, which will
localize the provider and session timestamps a person reads.

## Documents

| Document | Purpose |
| --- | --- |
| [specs/cli-design.md](specs/cli-design.md) | What the system does and must keep doing: provider, credential, usage, pricing, session, backup, and distribution behavior. Currently version 18; see its changelog. |
| [specs/cli-manual.md](specs/cli-manual.md) | The implemented command surface, flags, and output shapes. |
| [plans/display-timezone.md](plans/display-timezone.md) | Render instants in the machine's zone in text only; storage and JSON stay UTC. `active — 1/3 done`. |
| [plans/project-attribution.md](plans/project-attribution.md) | Tell a Headroom-marked wrapper which project a launch belongs to, delivered through `agentdeck run` plus an installable shell helper. `active — 4/6 done`. |
| [reviews/](reviews/README.md) | Per-task review records that back each plan's ticked `Review` cell. |
| [archive/](archive/README.md) | Retired plans and superseded contracts. Not a starting point for new work. |

## Open Tasks Not Owned by a Plan

None.

## Backlog

Candidate work with no approved specification. Each item needs its own plan
before implementation starts; promote it out of this list at that point rather
than expanding the entry in place.

- [ ] Make the copyable
      `agentdeck session show <id> --client <client> --activity` commands emitted
      by usage statistics robust when the core usage database is newer than the
      separately purgeable session index. Reproduced on `v0.2.0-rc.2`: the
      selected Claude session, its usage events, safe tool calls, and source
      ownership were present in `agentdeck.sqlite3`, while `sessions.sqlite3`
      contained no metadata, documents, or source row for that session because
      its last scan predated the source log. `session show` therefore returned
      the raw `sql: no rows in result set` from its initial metadata query before
      on-demand activity parsing began. A plan must decide whether activity
      detail resolves through core usage state when available, the generated
      command synchronizes or verifies the session index, or the CLI reports an
      actionable stale-index error. Acceptance must cover a regression where
      usage state is newer than the session index and preserve the privacy
      contract: no tool arguments, results, command text, environment, or
      reasoning may be exposed.

- [ ] Add visible progress to `agentdeck session scan`. A real scan can walk
      enough Claude and Codex JSONL sources to leave an interactive terminal
      apparently idle until the single final `Completed session.scan.` line.
      Follow the established delayed usage-scan progress pattern: show processed
      and total source counts after a short anti-flicker delay, update in place
      on a TTY, remain deterministic on non-TTY output, honor `--quiet`, and
      never print source paths or session content. Keep the final scan summary
      after progress closes and cover cancellation and zero-source behavior.
- [ ] Redesign the human-facing `session show` text layout. The current metadata,
      approved documents, full activity summary, and activity rows form a dense
      stream that is difficult to scan even when the requested information is
      present. A plan must define a compact session header, clearly separated
      document/activity/token sections, stable column priorities for narrow and
      wide terminals, readable timestamps and durations, and explicit empty and
      partial states. JSON compatibility and the existing activity privacy
      boundary remain unchanged.
- [ ] Add an interactive session viewer with keyboard navigation instead of
      requiring a new shell command for every `--page` transition. It should let
      a user move up and down, page forward and backward, switch among session
      overview, approved documents, tool activity, and token-detail sections,
      and quit without changing source data. A plan must decide whether this is
      an explicit `--interactive` mode or the TTY default, preserve the current
      non-interactive `--page`/`--limit`/`--all` and JSON contracts for scripts,
      handle terminal resize and interruption, and avoid loading an unbounded
      session into the terminal UI.
- [ ] Expose invocation-level token details as part of session inspection.
      AgentDeck already retains each supported usage event in
      `agentdeck.sqlite3`, including its session, event time, model, input,
      cached-input, output, Claude cache-read/cache-creation/cache-write
      components, source ownership, and attribution inputs. Today
      `usage sessions` collapses those events into one session total, while
      `session show` exposes no token detail at all. Add a session-scoped view
      of each invocation or logical turn, with token components, model, time,
      pricing completeness, and attributable cost where valid, then make it
      available to the interactive session viewer. The design must define Codex
      cumulative snapshot/delta semantics, Claude cache components, event
      ordering, pagination, duplicate-source ownership, unavailable components,
      and the boundary between safe usage metadata and prohibited raw session
      content.

- [ ] Confirm whether a Claude **app** picks up a project-scoped
      `.claude/settings.local.json` without a restart.
      [project-attribution](plans/project-attribution.md) documents that file as a
      recipe the user applies themselves, but does not claim it reaches an app.
      The doubt is concrete: Claude Code's `env` block is injected when the
      process starts, and the `CLAUDE_ENV_FILE` available to `SessionStart`,
      `Setup`, `CwdChanged` and `FileChanged` hooks is sourced before each Bash
      tool command, so it reaches subprocesses rather than the layer that attaches
      request headers. Settle it by observing real requests through a proxy;
      documentation inference is not enough. The answer only changes what the
      manual promises — AgentDeck still would not write that file.
- [ ] Attribute ChatGPT **app** launches to a project. **Low priority, not
      implemented for now.** The reason is that the app exposes no provider
      configuration AgentDeck can reach, and Codex has no project-level
      configuration to scope a value to, so there is currently no mechanism to
      document or to build against — unlike the Claude app, where at least a
      candidate file exists. Revisit if the app gains a configuration surface.
- [ ] Classify `codex-auto-review`, which accounts for roughly 85 M tokens of
      real usage and is unpriced on purpose. It is absent from every pricing
      source checked and is most likely a Codex-internal pseudo-model rather
      than a billable one, so it needs classification — suppress it, or
      attribute it to its real underlying model — not a price. This is a
      usage-attribution question, not a catalog-coverage one; the price-coverage
      plan deliberately left it out of scope and a shipped test asserts it stays
      unpriced, so any change here must update that fixture too.
- [ ] Close the `cache_creation_tokens` gap on the dotted Claude spellings.
      `claude-haiku-4.5` and `claude-opus-4.8` report
      `missing_components: [cache_creation_tokens]`, which is why cold-start
      coverage reads 95.1% fully priced against 98.4% model-matched. Their
      models *are* matched and priced by the bundled catalog; this is a
      token-classification issue in event parsing, not a catalog one.

- [ ] Add the ability to switch Claude subscription/account — analogous to the
      existing AI provider switching, but selecting a Claude account or plan
      rather than an API base URL and token. Not addressed by the
      provider-wrapper-routing plan: selecting `official` there returns a client
      to whatever login it already holds and deliberately never enumerates,
      selects, stores, or refreshes an account, plan, or OAuth token. This item
      is what would cross that line, so it needs its own plan and its own
      security review.
- [ ] Implement a GUI, including a persistent menu-bar presence, as an
      alternative front end to the CLI.
- [ ] Address two defense-in-depth findings from the 2026-07-22 credential
      vault security review. Neither is exploitable today; take them the next
      time `internal/credentialvault/vault.go` is opened.
      (a) **Durability, higher priority despite lower likelihood**
      (`vault.go:244`): `os.Link` is not followed by a parent-directory
      `Sync()`, so the key file's contents are durable but its directory entry
      is not. A crash in that window, after SQLite has already committed
      ciphertext, leaves ciphertext with no recoverable key — and the design
      deliberately refuses to regenerate a key when encrypted rows exist, so the
      credentials are permanently lost. One `Sync()` on the state root closes it.
      (b) **Cryptographic hygiene** (`vault.go:181-182`): the persisted key ID
      is SHA-256 of the live AES key truncated to 16 bytes, which publishes a
      hash of the key and gives an offline oracle for verifying guesses at key
      material. Not exploitable against a 256-bit random seed, but avoidable:
      expand HKDF to 48 bytes and take bytes 32..48 as the ID so it is derived
      alongside the key rather than from it. Requires a key-version increment;
      existing ciphertext must keep verifying under version 1.
      Plaintext and key bytes are not zeroed after use. That is an accepted
      residual risk, not a task — Go's copying GC makes wiping unreliable and
      `Open` returns an immutable `string`.
- [ ] Address the remaining low-severity finding from the 2026-07-22 price
      update review, ideally folded into the next change that already touches
      `internal/usage/price_update.go`: `price_update.go:143-148` checks the
      byte-size cap before the HTTP status, so an oversized 5xx body is reported
      as non-retryable "response exceeds N bytes" instead of a retryable
      transient failure.
- [ ] Decide whether `agentdeck doctor`'s `session.CheckHealth` should avoid
      creating `sessions.sqlite3-wal`/`-shm` sidecars when it inspects the index
      (e.g. `immutable=1`/`nolock` handling, weighing the concurrent-watcher
      trade-off), or whether its "without creating, migrating, or changing it"
      doc comment should be corrected to describe sidecar creation as expected.
      Surfaced by the retired test-coverage plan (task 6): `CheckHealth` opens
      the WAL-mode index read-only yet still materializes both sidecars. The
      committed database bytes are unchanged and the sidecars are `0600` inside
      the `0700` state root, so no privacy or data-integrity boundary breaks —
      this is a doc-vs-behavior accuracy fix, not a security fix. A shipped test
      in `internal/session/doctor_test.go` pins the current sidecar behavior, so
      any change here must update that test too.

## Document Conventions

One shape for every document. The **directory decides what a document is**, the
**filename is just its topic**, and **frontmatter carries its attributes** —
none of that is encoded in the filename.

```yaml
---
status: active | historical
version: N            # specs only, raised with each contract change
created: YYYY-MM-DD
retired: YYYY-MM-DD   # archived documents only
---
```

Filenames are lowercase and hyphenated, with no date and no type suffix:
`docs/specs/cli-design.md`, `docs/plans/test-coverage.md`.

| Directory | Holds | Lifecycle |
| --- | --- | --- |
| `docs/specs/` | Contracts — what the system does and must keep doing | Revised in place; stays active as long as the system stands |
| `docs/plans/` | Execution — how and when finite work gets done | Retires to `docs/archive/` once delivered and reviewed |
| `docs/reviews/` | Per-task review records, mirroring `plans/` by topic | Archived alongside the plan it belongs to |
| `docs/archive/` | Retired documents, mirroring `plans/`, `specs/`, and `reviews/` | Historical; never a starting point |
| `docs/README.md` | This file: the document map and the execution state | Updated in place |

**Specs additionally carry a version and changelog.** Raise the version and add
a changelog row whenever promised behavior changes. A spec does not retire when
a feature ships — that is when it becomes most authoritative. Never record
execution state in a spec; "implemented", "awaiting review", and "in flight"
belong in this file.

**Plans are scoped work, not a permanent home.**

- One plan owns one coherent piece of work with its own goal, evidence,
  checklist, and verification level.
- Start a new plan when work has its own goal and acceptance criteria, even if
  it touches an already-delivered feature. Only append a follow-up section to an
  existing plan when that plan's work is still in flight and the follow-up is
  genuinely part of finishing it.
- Retire a plan once every task's final gate is ticked (all done): move it under
  `docs/archive/plans/`, move its `docs/reviews/<plan-topic>/` directory into
  `docs/archive/reviews/`, set `status: historical` and `retired:`, record why in
  `docs/archive/README.md`, and collapse it into one line of "Current State"
  above.
- Watch the size. A plan past a few hundred lines, or one whose sections no
  longer share a goal, should be split or retired rather than extended. An
  earlier convention said to keep appending dated follow-up sections to the
  existing plan; that grew one file to roughly 950 lines spanning phase-one
  bootstrapping through release automation, and it was retired on 2026-07-22
  for exactly that reason.
- End a plan with a **Starting a task** recipe: a single hit-method that turns
  any Status-matrix anchor into a scoped `进入开发` instruction, so a task can be
  handed to a developer (human or agent) without hand-writing a fresh prompt.
  The recipe names what to read (`AGENTS.md`, the task's own section and its
  named files, the verification routing) and what to do on completion (tick
  `Dev`, record evidence, leave the review trail). One recipe per plan, keyed by
  anchor, rather than a duplicated instruction under every task — per-task
  specifics already live in each task's section. A plan whose tasks are already
  governed by an equivalent execution contract satisfies this without a second
  section.

**Each plan tracks its own tasks in a status matrix; this file records only a
coarse rollup.** A plan's task content lives in its prose (a `## Tasks` list or
per-task sections); the matrix carries only status — one row per task, one
column per gate (`Dev` and `Review`, plus `Test` or `Acceptance` later as a
plan needs them), and a tick when that gate passes for that task. The
implementer ticks `Dev` when the task is built and its own targeted
verification passes; an independent reviewer ticks `Review` when findings are
closed, and reopens the task rather than ticking it when review finds problems.
Each `Review` tick is backed by a `Verdict: PASS` round recorded in
`docs/reviews/<plan-topic>/<task-anchor>.md`; see `docs/reviews/README.md`.
A task is *done* when its last required gate is ticked. This file records only
`active — X/N done` per plan, where N is the task count and X counts done
tasks. Never copy the per-task grid here — that duplicate is how a status
document and its checklists drift apart.

**Archiving means moving, not deleting.** Preserve content, set the frontmatter,
and record in `docs/archive/README.md` why the document was retired and where
its conclusions now live. Do not re-list archived files in this index.

## Status Vocabulary

Only two values, matching the frontmatter above:

- `active`: a current contract, or unfinished work.
- `historical`: superseded or completed material kept only for context. It
  lives under `docs/archive/`, never in `docs/specs/` or `docs/plans/`.
