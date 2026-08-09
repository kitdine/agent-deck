---
status: active
created: 2026-07-14
---

# AgentDeck Documentation

This is both the documentation index and the execution baseline. Decide what to
work on next from this file. Repository code, tests, configuration, and Git
history remain the source of truth when they disagree with any document.

## Current State (2026-08-06)

`v0.3.0` is the current stable GitHub release. Its annotated tag points to
`b940010c09caac1d4ea5687629f4d60756300f77`; the
[tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/31078492290)
passed and published a non-prerelease
[GitHub Release](https://github.com/kitdine/agent-deck/releases/tag/v0.3.0)
with Darwin arm64 and amd64 archives plus checksums. The stable Homebrew Formula
update remains open in
[homebrew-tap PR #12](https://github.com/kitdine/homebrew-tap/pull/12), so GitHub
publication is complete while stable Homebrew channel promotion is not yet
complete. The earlier `v0.3.0-rc.1` Cask-independent Formula RC validation and
tap merge remain release-candidate evidence, not proof that the stable Formula
has been promoted.

`v0.2.1` was an earlier stable release. Its annotated tag points to
`e722be82c617a1418cc533a0ea2cbed35b65ad06`; the
[tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/30754555537)
passed both release and Homebrew jobs, published a non-prerelease GitHub Release
with both Darwin archives and the checksum asset, and verified the stable
formula through an isolated install, formula test, and bash, zsh, and fish
completion loading. [Homebrew tap PR #8](https://github.com/kitdine/homebrew-tap/pull/8)
merged the checked formula into `Formula/agentdeck.rb`. The local channel
transition also completed: `brew uninstall kitdine/tap/agentdeck-rc` removed
`0.2.1-rc.2`, and `brew install kitdine/tap/agentdeck` installed stable `0.2.1`.
The installed binary reports `v0.2.1` at commit
`e722be82c617a1418cc533a0ea2cbed35b65ad06`; all three completion files are
present in Homebrew's standard paths. Local `brew test kitdine/tap/agentdeck`
could not complete because Homebrew's developer bundle cache under
`/usr/local/Homebrew/Library/Homebrew/vendor/bundle` is not writable; the
GitHub Homebrew job passed the corresponding formula verification.

`v0.2.0` was the previous stable release. Its annotated tag points to
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

`v0.2.1-rc.1` was the first opt-in release candidate. Its annotated tag
points to `aaab1c2076e3685dcd3c9d548c1c4bfccc4c0f6c`; the
[tag-triggered Release run](https://github.com/kitdine/agent-deck/actions/runs/30488817055)
passed both release and Homebrew jobs, published a GitHub prerelease with both
Darwin archives and the checksum asset, and verified the RC formula through an
isolated install plus bash, zsh, and fish completion loading.
[Homebrew tap PR #6](https://github.com/kitdine/homebrew-tap/pull/6) merged the
checked formula into `Formula/agentdeck-rc.rb` without changing the stable
formula. The local stable-to-RC transition also passed:
`brew uninstall kitdine/tap/agentdeck` removed `0.2.0`,
`brew install kitdine/tap/agentdeck-rc` installed `0.2.1-rc.1`, and
`brew test kitdine/tap/agentdeck-rc` succeeded. The installed binary reports
`v0.2.1-rc.1` at commit
`aaab1c2076e3685dcd3c9d548c1c4bfccc4c0f6c`; all three completion files load
successfully, and the released `shell-init` command emits the expected
project-aware wrapper.

The following RC1 design history records the issues resolved before stable
`v0.2.1` promotion. RC1 proved wrapper generation but neither
a coherent installation lifecycle nor an observable reason for the wrappers to
act: `shell-init` writes functions to stdout without installing or activating
them, Homebrew installs completion only, and nothing reports whether a client
currently routes through a wrapper declared `headroom` — the condition
`Service.RunProjectEnvironment` requires before any attribution is injected. A
design review on 2026-07-29 added that second gap to the plan, along with the
finding that an exported constant activation marker would misreport child
shells as active because environment variables are inherited while shell
functions are not. A second review round the same day settled three questions
about how invisible this can be made: `shell-init` cannot be deleted, because a
released managed block and already-configured RC users depend on it, so it
becomes hidden instead; package installation must not write the block, because
`brew uninstall` would leave an orphan that breaks every later shell startup and
no installer can activate a running shell anyway; and `provider use` should
announce attribution changes on both entering and leaving an eligible route,
which also corrects an advisory that still promises launches outside
`agentdeck run` are unattributed. A third round then settled how the wrappers
survive AgentDeck's own removal: the managed block guards on `agentdeck` being
on `PATH`, so a block left behind by `brew uninstall` is inert rather than
fatal — a Homebrew formula has no uninstall hook, unlike a cask, so nothing can
clean it up automatically. The resolver the wrappers call became the documented
`agentdeck shell env <codex|claude>` rather than a `provider current` parse,
because `CurrentSelection` carries no `wrapper_kind` and the wire encoding must
stay in Go. The same round also made the lifecycle commands cover every shell in
use by default rather than only the invoking one, since one person commonly uses
more than one shell; that capability belongs to the authorized command, not to
package installation. The negative-gate marker is confirmed as part of the
delivery, so a user with no Headroom route pays one `test -e` per client launch
instead of a fork. The plan also records why this work belongs to `v0.2.1`
rather than `v0.2.2`: `shell-init` has never shipped in a stable release, so
hiding it and moving the recommended path is an internal rearrangement only
before stable `v0.2.1` shipped it.

Writing startup files from `brew install` was re-examined and rejected again, but
on narrower grounds than before — two earlier objections are recorded as
retracted, since `post_install` does have a correct `$HOME` and neither approach
can activate a running shell. What stands is that an upgrade would silently undo
a deliberate removal, an uninstall would leave a block the user never agreed to,
and CI or container builds would be written to as well. The same convenience was then adopted at a
different moment: `provider use --via` configures the shell itself on the first
switch that makes a client eligible, because that is when the user's intent is
unambiguous and only users who need the integration are touched. It writes
nothing when the invocation is not interactive — a non-TTY stderr, JSON or NDJSON
output, `--quiet`, or `--no-shell-setup` — which is what keeps the CI objection
from returning through another door, and nothing after `agentdeck shell remove`,
which records the refusal until `shell setup` reverses it. `shell setup` therefore
stops being the ordinary path and becomes the explicit one, for configuring ahead
of any route, repairing a block, or following a non-interactive switch.

[The retired shell integration plan](archive/plans/shell-integration.md)
delivered the shell lifecycle, per-client eligibility reporting, route-change
advisories, switch-time setup, cross-shell verification, and living contract
migration. All eight tasks passed independent review; the stable contract now
lives in `docs/specs/cli-design.md` version 21 and
`docs/specs/cli-manual.md`, while implementation and review history live under
`docs/archive/`. This documentation closure was not release-readiness proof at the time:
L4 and real RC2 artifact installation gates have since passed in the
`v0.2.1` release workflow and Homebrew validation. The
`v0.2.2` hardening plans are no longer blocked by this plan.

Local validation of installed `v0.2.1-rc.2` at commit `886f0a8` found one
release-blocking quick-diagnostics defect: default `agentdeck doctor` enters
full-history price coverage through `PriceDiagnostics` and does not return in a
bounded time on the 93,982,720-byte real usage database. The
[retired doctor quick diagnostics plan](archive/plans/doctor-quick-diagnostics.md)
delivered only the `v0.2.1` quick/full boundary fix. The related full-doctor and
`usage sessions` N+1 pricing reads remain explicitly deferred to the `v0.2.2`
[retired usage pricing read scalability plan](archive/plans/usage-pricing-read-scalability.md).
The quick-boundary change was committed in `e722be8`; its L2 suite and Review
Round 2 passed after directly pinning full-mode `unpriced_models` and its result
code. Real-state acceptance passed on 2026-08-02 against the 93,982,720-byte
usage database: its before/after SHA-256 was
`2763ef03f3fff49f79955fe78eb19e31c96323fcb2b88a5fc15ffc024dd8624b`.
Quick `doctor` (12 checks) and `doctor --full` (18 checks) both used the
read-only database path and exited 0; the full check included price provenance
and unpriced-model coverage. Because `usage stats --no-scan` still opens a
writable store, baseline and current binaries compared isolated byte-identical
copies over a fixed all-history range. Their JSON matched after excluding only
the envelope's dynamic `generated_at` field, confirming the session-start
fallback did not change stats output. The installed stable Homebrew binary is
separately verified above.

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
on the same frozen snapshot went from 7.4% to 95.2% of tokens fully priced, and
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

Project attribution is delivered and independently reviewed under the
[CLI contract](specs/cli-design.md#project-attribution) and
[manual](specs/cli-manual.md#project-attribution). It lets a
user mark a wrapper URL as speaking Headroom's protocol and have
`agentdeck run` label a launch with the project it happened in, with an
installable shell helper and a documented settings-file recipe covering
launches AgentDeck does not make. All six tasks are delivered and independently
reviewed: `headroom-wrapper-kind` adds the explicit protocol
declaration, and `project-identity` shares the session parser's cleaned
full-path identity while exposing only a safely encoded base name on the wire;
`run-env-injection` attributes eligible AgentDeck-launched Codex and Claude
processes; `attribution-guidance` points successful Headroom wrapper actions
to the project-owned manual without changing JSON or quiet output; and
`shell-helpers` adds `agentdeck shell-init <bash|fish|zsh>`, which emits `codex`
and `claude` wrapper functions that resolve attribution through the same route
check that `agentdeck run` uses, so a helper sourced under a direct route or a
non-Headroom wrapper injects nothing. `attribution-contract` records the
shipped behavior in specification version 20.
AgentDeck writes no file it does not already own; the app questions sit in the
Backlog below.

Display timezone rendering is delivered and independently reviewed under the
[CLI contract](specs/cli-design.md)'s "Time Representation" rule and the
[manual](specs/cli-manual.md). Text meant for a person renders instants in the
machine's zone to the second and always names that zone; storage, JSON, and
NDJSON keep UTC RFC 3339, and an unparseable value renders unchanged rather
than failing a read. All three tasks are delivered and reviewed:
`display-clock` extracted the shared zone, timezone-name, and
RFC3339/`time.Time` rendering helpers; `provider-and-session-surfaces`
localized provider selection times, session bounds, and safe activity start
times, leaving `session search` unchanged because its result contract carries
no instant; and `backup-and-price-surfaces` localized backup and price
timestamps and completed the renderer sweep across price-list provenance,
usage session bounds, watch text, and the `usage stats --activity` model range.
`version`'s `UTC Build Time` stays UTC by decision, because it is immutable
build identity rather than a runtime instant.

## Release Roadmap (2026-08-06)

The repository had shipped four releases without a written rule for what a
version-number position means, and the patch position had drifted into being a
release counter: `v0.2.1` added the `agentdeck shell` command group and
relocalized every human-facing timestamp, which is minor-level content. The
[retired release versioning contract plan](archive/plans/release-versioning-contract.md) wrote
the position semantics into `docs/specs/cli-design.md`, and the next two
releases are scoped by that rule rather than by convenience.

**`v0.2.2` — patch. Read scalability and defect fixes; safe to downgrade from.**
No new command, flag, schema migration, typed error code, or user-visible number
change. **All nine tasks across its four plans were delivered and independently
reviewed by 2026-08-03; the plans and their review records retired to
`docs/archive/` that day.** See the retirement entry in
[the archive index](archive/README.md) for what each one delivered and where its
conclusions now live. The release itself is not tagged yet.

**`v0.3.0` — minor. Runtime attribution and pricing semantics.** Carries a
schema migration, two external client Hook contracts, a Codex trust step
AgentDeck cannot automate, changed cost numbers, and credentials an earlier
release cannot read.

**Every behavior task across its three plans was delivered and independently
reviewed by 2026-08-04; the plans and their review records retired to
`docs/archive/` that day.** See the retirement entry in
[the archive index](archive/README.md) for what each one delivered and where its
conclusions now live. The final contract task `v0-3-0-contract` passed
Round 3 re-review on 2026-08-04, so every task in the batch is complete.
The stable release was tagged and published on 2026-08-06 at commit
`b940010c09caac1d4ea5687629f4d60756300f77`. Its GitHub Release workflow passed;
stable Homebrew Formula promotion remains open in `kitdine/homebrew-tap#12`.

Two rules held across the `v0.3.0` batch. The specification version was raised
**once**, by `v0-3-0-contract` in the runtime attribution plan, which recorded
the batch's behavior changes as version 23; and both releases ship at least one
`-rc.N` validated against real local data, because both touch persisted data or
the pricing read path. `v0.3.0-rc.1` completed that release-candidate gate before
stable publication.

Its release notes must state both downgrade consequences, which the retired
contract task owned and this index now carries: credentials written by this
release are unreadable by `v0.2.x`, and cost/coverage numbers change for
existing data.

**`v0.4.0` — minor. Session experience, usage report presentation, and
desktop-facing data contracts.** Two lines share the release.

The first prioritizes visible `session scan` progress, a defined instant in
`session search`, a redesigned human-facing `session show`, invocation-level
token and cost detail, and an interactive session viewer. The same work must
leave bounded, stable DTO and JSON contracts suitable for the native desktop
client. This six-task line is now delivered and independently reviewed; its
current behavior is absorbed into the living CLI design and manual, while the
historical execution record is summarized by the archive index.

The second redesigns how the `usage` report family presents what it already
computes, across `usage stats`, `usage summary`, `usage sessions`, and
`usage diagnose`. The retired
[readability plan](archive/plans/usage-stats-readability.md) bounded how much
those reports print; this line addresses whether the printed values can be
compared and understood. It is owned by the active
[usage report presentation plan](plans/usage-report-presentation.md), which
defines six implementation and review gates. Its interactive mode reuses the
session viewer's terminal state machine, so its task 5 is blocked on
`interactive-session-viewer`.

Both lines land in one release, so the batch rule that held across `v0.3.0`
still holds: the specification version is raised **once**. Each feature plan
lands only its own contract text; the single raise, the release candidate, and
the release notes belong to the
[v0.4.0 release plan](plans/v0-4-0-release.md), which starts only after both
lines are fully reviewed.

**`v0.5.0` — minor. Native macOS desktop foundation.** Deliver the Swift 6,
SwiftUI macOS 26 menu-bar app, WidgetKit extension, embedded universal Go helper,
Homebrew Cask `agentdeck-app`, direct-download DMG, signing, notarization, and a
notification-only update check that opens the official download page. The active
[native desktop plan](plans/desktop-app.md) owns its six implementation and
review gates, the last of which lands its own contract text. The single
specification raise, the release candidate, and the release notes belong to the
[v0.5.0 release plan](plans/v0-5-0-release.md).

**`v0.6.0` — minor. Skills and Hooks lifecycle management.** Build the shared Go
lifecycle engine, Skills adapter, and third-party Hooks adapter with preview,
ownership, drift detection, atomic mutation, rollback, and doctor behavior. The
GUI is the primary interactive surface; thin deterministic CLI commands remain
for automation, diagnosis, and recovery. The specialized runtime-attribution
`usage hook` lifecycle remains separate. Skills and Hooks still require separate
approved plans before development.

**`v0.7.0` — minor. Plugins and MCP lifecycle management.** Reuse the reviewed
lifecycle engine for Plugins and MCP servers, including dependency, credential,
transport, source-authenticity, offline, and client-ownership boundaries. Each
native adapter still requires its own approved plan and independent review.

The `v0.2.2` batch was cut from what was planned on 2026-07-29. The credential
and pricing hardening plan then held six tasks; `key-id-derivation` and
`cache-creation-ttl-default` moved to `v0.3.0` with their evidence intact,
because the first makes newly sealed rows unreadable by `v0.2.1` and the second
changes user-visible cost numbers. Tasks 1, 3, and 4 never depended on them.
`session scan` progress output was considered for `v0.2.2` and declined; it
stays in the Backlog.

Pulling the runtime attribution work into `v0.2.1` was considered on 2026-07-29
and rejected: unlike `shell-init`, none of it has an expiring interface window —
the `usage hook` group and the session-route table are additions, and dropping
`one_active_usage_run_per_client` relaxes behavior rather than tightening it —
while it does carry a schema migration, two external client Hook contracts, and a
Codex trust step AgentDeck cannot automate. Two dependencies on `v0.2.1` were
recorded instead: its `shell-init` byte-identity acceptance is now pinned to the
post-shell-integration output rather than today's, and its `usage hook`
lifecycle must follow the `shell` lifecycle conventions that `v0.2.1` establishes,
since the two cannot share code but should not diverge. Two design findings from the credential plan matter outside
it: the affected
cache-creation events all carry a `cache_creation` object whose two TTL fields
are zero rather than a missing object, so dotted model spelling is a
coincidence and no implementation may branch on a model name; and treating such
a total as a five-minute write contradicts an explicit specification rule, so
that task rewrites a rule promised by the current specification rather than
fixing a defect, which is why it is a contract change recorded by the single
`v0.3.0` contract task rather than a patch.

## Documents

| Document | Purpose |
| --- | --- |
| [specs/cli-design.md](specs/cli-design.md) | What the system does and must keep doing: provider, credential, usage, pricing, session, backup, and distribution behavior. Currently version 23; see its changelog. |
| [specs/cli-manual.md](specs/cli-manual.md) | The implemented command surface, flags, and output shapes. |
| [plans/usage-report-presentation.md](plans/usage-report-presentation.md) | Active – 2/6 reviewed; Task 2 bar/detail semantics passed Round 4 after closing value-preservation and cross-row alignment findings. Render primitives passed Round 2 after independent multi-width byte-identical baseline verification. Stats layout, family alignment, interactive viewing, and contract text remain for `v0.4.0`. |
| [plans/v0-4-0-release.md](plans/v0-4-0-release.md) | Active — 0/3 done. Release plan, not a feature plan: the single `v0.4.0` specification raise, the release candidate, and release notes. Blocked until both `v0.4.0` feature plans are fully reviewed. |
| [plans/desktop-app.md](plans/desktop-app.md) | Active — 0/6 done. Native macOS 26 menu-bar app, WidgetKit extension, unified desktop package, Cask, direct download, and its own contract task for `v0.5.0`. |
| [plans/v0-5-0-release.md](plans/v0-5-0-release.md) | Active — 0/3 done. Release plan, not a feature plan: the single `v0.5.0` specification raise, the release candidate, and release notes. Blocked until the desktop plan is fully reviewed. |
| [reviews/](reviews/README.md) | Per-task review records that back each plan's ticked `Review` cell. |
| [archive/](archive/README.md) | Retired plans and superseded contracts. Not a starting point for new work. |

## Open Tasks Not Owned by a Plan

None.

## Backlog

Candidate work with no approved specification. Each item needs its own plan
before implementation starts; promote it out of this list at that point rather
than expanding the entry in place.

- [ ] Resolve the billing treatment of `codex-auto-review`. The 2026-08-04
      aggregate evidence probe proved it is a dedicated automatic-approval
      reviewer model/session label with independently reported token events,
      but neither official Codex source, adjacent-event correlation, nor public
      pricing catalogs establish whether its tokens are free, separately
      billed, or charged under another model. Reopen behavior work only with
      authoritative billing/account evidence; see
      [the retired classification plan](archive/plans/codex-auto-review-classification.md).

- [ ] Design complete lifecycle management for native Skills, Plugins, MCP servers,
      and Hooks. This is an umbrella capability, not an implementation task:
      promote it into four independently approved plans before development because
      each native adapter has different ownership, trust, credential, dependency,
      and rollback contracts. Define consistent `status`, `install`, `update`,
      `enable`, `disable`, `remove`, and `doctor` semantics while keeping Codex and
      Claude configuration as the source of truth. Every mutation must support
      preview, drift detection, atomic application, and safe rollback; must preserve
      unknown and non-AgentDeck-owned content; and must specify source authenticity,
      version pinning, dependencies, credential handling, and offline behavior.
      Existing `extension adopt` and `extension release` remain metadata-only rather
      than implying file ownership. An adapter stays read-only until its client
      exposes an unambiguous stable write contract. The Hooks plan must keep
      AgentDeck-owned runtime-attribution handlers distinct from arbitrary
      third-party hooks and must not generalize `usage hook` into unrestricted hook
      mutation.

- [ ] Confirm whether a Claude **app** picks up a project-scoped
      `.claude/settings.local.json` without a restart.
      The [CLI manual's Project Attribution
      section](specs/cli-manual.md#project-attribution) documents that file as a
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
- [ ] Add the ability to switch Claude subscription/account — analogous to the
      existing AI provider switching, but selecting a Claude account or plan
      rather than an API base URL and token. Not addressed by the
      provider-wrapper-routing plan: selecting `official` there returns a client
      to whatever login it already holds and deliberately never enumerates,
      selects, stores, or refreshes an account, plan, or OAuth token. This item
      is what would cross that line, so it needs its own plan and its own
      security review.
Plaintext and credential key bytes are not zeroed after use. That is an
accepted residual risk rather than a Backlog item: Go's copying garbage
collector makes wiping unreliable, and `credentialvault.Open` returns an
immutable `string`.

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

**A plan owns a feature; a release owns a version.** These are different scopes,
so they get different plans.

- A **feature plan** owns one coherent piece of product behavior. Its own
  contract task reconciles *what that plan delivered* with the living specs. It
  **never raises the specification version**.
- A **release plan** owns one version. It raises the specification version
  **exactly once**, validates the release candidate, and prepares release notes.
  It owns no product behavior and starts only after every feature plan in that
  version is fully reviewed.

The version raise describes a release, not a feature, so attaching it to a
feature plan makes whichever plan finishes last implicitly responsible for the
release. This project did that through `v0.3.0` — `v0-3-0-contract` lived inside
`runtime-provider-attribution` — and corrected it on 2026-08-06 for `v0.4.0` and
`v0.5.0`. The shipped `v0.3.0` history is not rewritten.

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
