---
status: active
created: 2026-09-08
updated: 2026-09-09
---

# Subscription Quota — Requirements

Version membership is decided by the `v0-6-0-contract` topic's assembly list.
This topic is already selected there as the carrier `ad-subscription-quota`;
see `docs/topics/v0-6-0-contract/tasks.md`. Selection is not an implementation
decomposition, and it does not authorize a merge.

Promoted from the operator's **2026-09-05 v0.6.0 planning decision**
(`docs/roadmap.md:33`, with its scope paragraphs at `:35-45`), not from a
measured defect. The capability does not exist today, so this document records
a goal, the current limitation, and the source feasibility that was actually
verified — never a runtime measurement of behavior that has never run.

## Problem

AgentDeck can tell a user what they *spent* and cannot tell them what they have
*left*. Local cost estimation from JSONL usage records is the product's oldest
capability (`docs/specs/cli-design.md:28`), but a subscription user's real
constraint is not dollars: it is a rolling quota window that refuses work when
it fills. Nothing in the product reads that window.

The practical failure is ordinary and repeated: work stops mid-task because a
five-hour or weekly window filled, with no warning beforehand and no answer
afterwards to "when does this free up". A menu-bar application that is already
resident, already reading both clients, and already showing cost is the natural
place for that answer, and it is the one number it does not have.

### What already exists

| Fact | Source |
| --- | --- |
| The product reads local session records for both clients and derives cost | `docs/specs/cli-design.md:28` |
| It already walks `~/.codex/sessions` | `internal/session/session.go:247` |
| It already surgically edits `~/.claude/settings.json`, managing the top-level `hooks` key while preserving everything else | `internal/usagehook/config.go`, `topLevelValueSpan` / `insertTopLevelValue` |
| It already makes one kind of network request: `price update` fetching a public LiteLLM price table | `internal/usage/price_update.go` |
| It already resolves the current provider per client without decrypting credentials | `provider.Service.Current`, `internal/provider/service.go:692`; `OfficialProviderName` at `:118` |

The price-update path matters because it establishes that AgentDeck performs
network I/O at all. It does not establish a precedent for this topic: that
request is unauthenticated and fetches a public file. Reading a user's account
state is a different class of act, and this document does not borrow the
precedent to justify it.

## What the clients actually expose

Verified on the operator's machine on 2026-09-08 against `codex-cli 0.153.4`
and Claude Code `2.1.266`. These are observations, not assumptions. Field
availability is per-account and per-CLI-version; none of it is a published
contract, which is itself a requirement input rather than a footnote.

### Codex — an answering interface

`codex app-server` speaks JSON-RPC over stdio. The handshake must complete
before a request is answered: `initialize`, then the `initialized`
notification, then the request. Piping both lines and closing stdin returns the
initialize result and nothing else; this is why the route is a driven
subprocess rather than a pipe.

`account/rateLimits/read` returns, in shape:

| Field | Observed | Meaning for this topic |
| --- | --- | --- |
| `primary.usedPercent`, `.windowDurationMins`, `.resetsAt` | `79`, `10080`, epoch seconds | Weekly window: used share, window length, natural reset instant |
| `rateLimitsByLimitId` | keys `codex` and `codex_bengalfox` | Per-limit windows; the named limit carried its own 300-minute and 10080-minute pair |
| `planType` | `"prolite"` | Plan identification |
| `rateLimitResetCredits.availableCount` | `3` | **Official reset allowance** |
| `rateLimitResetCredits.credits[]` | each with `status`, `grantedAt`, `expiresAt`, `title`, `description` | Per-credit lifecycle |
| `credits` | `{hasCredits, unlimited, balance}` | Billing mode |
| `accountId` | an opaque identifier | Account isolation key |
| `spendControlReached`, `rateLimitReachedType`, `individualLimit`, `rateLimitUpsell` | present, unset in this sample | Refusal and upsell conditions |

`docs/roadmap.md:36-39` wrote the official reset allowance as conditional —
"any official total, used or remaining reset allowance **if it exists**". It
exists. `availableCount` is the remaining count, and each credit carries its
own status. A *total* and a *used* count are not directly returned; only
credits currently visible to the account are listed. That gap is a disposition
this topic must state, not a number it may compute.

The same call is the only source for `planType` and for the main `codex` limit.
The `rate_limits` object embedded in `~/.codex/sessions/**/rollout-*.jsonl`
carries only the limit for the model that session used and reports
`plan_type` as `null`. **Local session files cannot substitute for this call.**

### Claude — two partial sources, no answering interface

Claude Code exposes no equivalent request. Four sources were checked:

1. **Session records.** A structured-key scan across every
   `~/.claude/projects/**/*.jsonl` for `rate_limit*`, `plan_type`, `quota*`,
   `resets_at` returned **zero** matches. (A file-level substring scan does
   produce hits; they are prose inside conversations, including this topic's
   own discussion. They are not fields.)
2. **`~/.claude/stats-cache.json`.** Holds locally computed activity —
   `dailyActivity`, `dailyModelTokens`, `modelUsage`, `totalSessions` — and no
   quota fields. It is the same category of data AgentDeck already derives.
3. **`claude -p "/usage"`.** Returns the real numbers, as **prose**:

   ```text
   Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)
   Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)
   ```

   `--output-format json` wraps that same string in `result`; no structured
   keys are added. Observed `total_cost_usd: 0`, `duration_api_ms: 0`,
   `num_turns: 0` — the slash command is local and **does not consume quota**.

   Its output mixes two origins, and they must not be presented as one: the
   percentages and reset instants come from the service, while the
   "what's contributing" section states on its face that it is
   "based on local sessions on this machine".

4. **The statusline payload.** Claude Code passes
   `rate_limits.five_hour` and `rate_limits.seven_day`, each with
   `used_percentage` and `resets_at` as epoch seconds, on stdin to the
   configured `statusLine` command. This is structured and needs neither
   network nor credentials. Two limits were confirmed: `claude -p` does **not**
   invoke a statusline, and `--output-format stream-json` does not carry the
   field, so the payload reaches only a live interactive session's status line.
   Anthropic issue #27915 requests that this become a declared part of the
   statusLine input, which is to say it is current behavior rather than a
   promised contract.

A fifth route exists and is **rejected**: the undocumented
`https://api.anthropic.com/api/oauth/usage` endpoint, reached with the OAuth
token in `~/.claude/.credentials.json`, returns the same figures on demand.
Published tools use it as their fallback. This topic does not, because reading
a client's stored credential is exactly the boundary the operator declined on
2026-09-08 and the one `docs/specs/cli-design.md:54-55` draws.

### The asymmetry, stated plainly

| | Codex | Claude |
| --- | --- | --- |
| Shape | structured JSON-RPC | structured via statusline; otherwise prose |
| On demand, no live session | yes | **no** — prose probe only |
| Plan identification | `planType` | none |
| Official reset allowance | remaining count and per-credit status | none |
| Per-model limits | yes | no |
| Reset instant | epoch | epoch via statusline; localized text via prose |
| Account identity | `accountId` | **none** — neither route reports one |

The two clients will not present the same fields, and the design must not
invent parity. An absent field is reported as unavailable with a reason.

### Named gap — Claude account identity

The last row of that table is a gap, not a missing field, because it is the one
absence that affects the *correctness* of every other Claude figure rather than
the completeness of one row. Codex returns an `accountId` that scopes what is
stored; Claude returns nothing equivalent through the status-line payload or
through `/usage`, and neither route is asked who is signed in.

The consequence is exact and cannot be engineered away without an identifier: a
user who switches Claude accounts continues to see the previous account's
figures until the next successful probe replaces them. Freshness bounds the
exposure in time; it cannot establish attribution, because an observation can be
recent and still belong to the account that was signed in a moment ago.

Two tighter routes were considered and declined. Watching
`~/.claude/.credentials.json` metadata as a change signal would put a new
dependency on the credential file this topic refuses to touch, and it would
invalidate on an ordinary token refresh as readily as on an account change.
Holding Claude observations only in memory would shorten the window in theory
and hardly at all in practice, because a resident menu-bar process rarely
restarts.

**The operator's 2026-09-09 decision is to accept the degradation and state it
on the surface.** The boundary this document then owns is what "state it" means:
the limitation is named where the Claude figures are, no surface or alert
asserts that a Claude figure belongs to a particular account, and the
acceptance item below can fail. `architecture.md` records the storage
consequence; it does not get to narrow this decision further.

## Goals

- **One place answers "what do I have left", for whichever client can say.**
  Codex reports through `account/rateLimits/read`; Claude reports through the
  statusline payload when it is available and through `claude -p "/usage"`
  otherwise. Each client's answer names its own source and freshness.

- **No credential is read, and no vendor account API is called by AgentDeck.**
  Both routes invoke the user's own already-authenticated client. AgentDeck
  never opens `~/.claude/.credentials.json` or `~/.codex/auth.json`, never
  holds a token, and never constructs an authenticated request. This is a
  boundary of the design, not an implementation convenience; Contract changes
  below reconciles it with the current specification wording.

- **Quota reading is opt-in, and off by default.** The capability has one user
  switch. Until it is on, AgentDeck reads no quota at all: no probe subprocess
  is spawned by any trigger, a user-initiated refresh included; the Claude
  status-line route is not registered and `~/.claude/settings.json` is not
  written; and no reminder is evaluated. Turning the switch off later keeps the
  observations already stored but stops displaying them — a number nothing is
  refreshing is not shown as though it were — and each surface says that reading
  is off, which is a third fact beside not-applicable and unavailable. Turning
  it back on shows the retained observations with their real age until the next
  successful probe replaces them. Retaining rather than erasing is the
  operator's 2026-09-09 decision: turning a reading off is not a request to
  forget what was already read.

- **Probing is gated on `official`.** A client whose current provider is not
  `official` is not probed at all: its quota belongs to whatever relay or
  wrapper is selected, and the built-in client's account figure would be
  actively misleading. Resolution uses `provider.Service.Current`, which does
  not decrypt credentials. This gate applies after the reading switch, not
  instead of it: `official` alone never authorizes a probe.

- **The three reset concepts stay separate, and each is labeled.** An official
  reset allowance (Codex `rateLimitResetCredits`), a natural window reset
  (`resetsAt` / `resets_at`), and a locally observed reset (AgentDeck noticing
  `usedPercent` fall) are three different facts. None substitutes for another,
  and a surface never renders one under another's label.

- **An unsupported field reports unavailable with a reason and a disposition.**
  `planType` for Claude, reset allowance for Claude, and a *total* or *used*
  reset count for either are not available. They render as unavailable naming
  why. They are never `0`, never blank, and never silently dropped from the
  surface — `docs/roadmap.md:40-42` and
  `docs/topics/v0-6-0-contract/tasks.md:64-65`.

- **Freshness is always visible, and stale is never shown as current.** Every
  displayed figure carries the instant it was observed. When the last probe
  failed, is older than its window allows, or never ran, the surface says so
  rather than showing the last good number as though it were now.

- **Account identity scopes the data where the client supplies one.** Codex
  returns an `accountId`: figures observed under one account are not shown under
  another, and a change of account invalidates the cache rather than blending
  with it. Claude supplies none through either route, so its figures are scoped
  to the client alone and carry a stated limitation instead of an implied
  guarantee — see Named gap above. No surface, alert, or export claims that a
  Claude figure belongs to a named account.

- **Probing is bounded and predictable.** With reading on, a user-initiated
  refresh probes quota together with the snapshot. What that refresh bypasses is
  the background interval, and only that: it does not bypass the reading switch
  or the `official` gate, so "the user asked for it" never becomes a route
  around a decision the user already made. Background refresh does **not** follow the
  snapshot cadence; quota has its own longer minimum interval, so an
  approximately one-minute snapshot does not become a one-minute subprocess
  spawn. Backoff, offline, expired, and authentication-failure outcomes each
  have a defined presentation.

- **Threshold and reset reminders exist, opt-in, default off.** With
  notifications disabled — the default — no reminder is evaluated or delivered.
  Enabled, a threshold crossing notifies once per window rather than on every
  probe, and a window reset notifies once.

- **Regression coverage asserts the boundary, not the vendor.** Tests assert
  the parsers against captured fixtures, the `official` gate, the three reset
  semantics, the unavailable-with-reason path, staleness, account isolation,
  probe interval enforcement, and reminder deduplication — without contacting a
  vendor in a test.

## Non-goals

- **No credential access and no direct vendor account API.** Named explicitly
  because a working route exists and was declined: `api/oauth/usage` with the
  stored OAuth token. Not "not yet" — not in this topic.
- **No reset action.** AgentDeck displays the remaining reset allowance. It
  does not spend one. Spending is an account-affecting action behind a vendor's
  own confirmation, and reading a count is not authority to consume it.
- **No account login, switching, or plan management.**
  `docs/specs/cli-design.md:1013` already states that selecting `official` is
  not account management; this topic reads what the logged-in account reports
  and changes nothing about it.
- **No plaintext credential persistence**, and no new credential storage of any
  kind. Nothing this topic reads is a secret.
- **No automatic updater**, per `docs/topics/v0-6-0-contract/tasks.md:65-66`.
- **No reconciliation against an invoice.** Quota share and money remain
  separate; a percentage is not a bill, and this topic adds no path between
  them.
- **No historical quota series.** The product stores the latest observation per
  client and account, not a time series. Charting quota over time is a distinct
  goal with its own storage and privacy questions.
- **No parity backfill for Claude.** Absent fields stay absent with a reason.
  AgentDeck does not estimate a plan type, derive a reset allowance, or
  reconstruct a per-model breakdown Claude does not report.
- **No probing of a non-`official` client**, including "just to see".

## Contract changes

`docs/specs/cli-design.md:45-55` currently states, under Non-Goals, that the
first release will not "query provider billing, subscription, or usage APIs"
and will not "select, store, or refresh a client account, plan, or OAuth token".

The operator's 2026-09-08 decision is that these routes **do not** cross that
boundary, because no credential is touched and every figure is obtained by
invoking the user's own local client. The wording nevertheless reads as a
prohibition on the capability itself, so this topic narrows it rather than
working around it. The revision must:

- keep the credential half intact and strengthen it — selecting, storing,
  refreshing, or reading a client account token remains prohibited, and this
  topic's own rejection of `api/oauth/usage` becomes an example of the rule
  rather than an exception to it;
- narrow the API half to what is actually excluded — AgentDeck issues no
  authenticated request to a vendor billing, subscription, or usage endpoint —
  while permitting the product to *invoke the user's authenticated client* and
  read what it reports;
- record that the resulting figures are the client's report, not AgentDeck's
  measurement, and that no vendor field read here is a published contract.

The specification is at `version: 28` in this worktree. The revision number is
deliberately not prescribed here: `docs/topics/v0-6-0-contract/tasks.md:164-165`
requires reading the then-current revision at contract-closure time, because
other v0.6.0 topics also edit this file.

Wire and surface contract additions belong to `architecture.md`; this section
records only the product-contract question that the requirement boundary
depends on.

## Acceptance boundary

The topic is complete when all of the following hold. Each is stated so that it
can fail.

1. **Reading is off by default.** On a fresh installation the reading switch is
   off, and in that state no probe subprocess is spawned for either client by
   any trigger, including a user-initiated refresh; `~/.claude/settings.json` is
   not written; and no reminder is evaluated. Every surface presents quota as
   reading-off with that reason — not as unavailable, not as not-applicable, not
   as zero, and not as a blank panel.
2. **Turning reading off retains without displaying.** Given stored
   observations, turning the switch off displays no figure from them and probes
   nothing, and does not delete them. Turning it back on displays those same
   observations with their true observation instants and the ordinary staleness
   treatment, until the next successful probe replaces them.
3. **Gating.** With reading on and a client's current provider set to anything
   other than `official`, no probe subprocess is spawned for that client, and its
   quota is presented as not applicable with that reason — not as unavailable,
   and not as zero.
4. **Codex fields.** With `official` selected for Codex, the surface shows the
   plan, the used share and reset instant for each returned window, and the
   remaining reset allowance. A window the account does not have is absent, not
   rendered at 0%.
5. **Claude fields, both routes.** With the statusline route active, the
   five-hour and seven-day windows show used share and reset instant from the
   structured payload. With it inactive or unconsented, the same two windows
   are obtained from `claude -p "/usage"`, and the surface names which route
   produced the figure.
6. **Prose-parse failure is a stated outcome.** Given `/usage` output whose
   shape does not match, the result is unavailable with a parse-failure reason
   and the observation instant of the failed attempt. No partial number is
   emitted, and the prior value is not re-presented as current.
7. **Three reset concepts.** The remaining reset allowance, the natural window
   reset instant, and a locally observed reset are distinguishable in the
   payload and are separately labeled on every surface that shows more than one.
8. **Unavailability carries a reason.** Every field the client did not supply
   renders with a reason from a closed set, and no such field ever renders as
   `0`, an empty string, or a silently omitted row.
9. **Staleness.** A figure older than its allowed age is presented as stale with
   its observation instant. A probe that has never succeeded is presented as not
   yet observed, distinctly from a probe that failed.
10. **Account isolation, Codex.** Observations recorded under one Codex
    `accountId` are not shown after the account changes; the cache is
    invalidated rather than merged.
11. **Account attribution, Claude.** Where Claude's figures are shown, the
    surface states that their account attribution cannot be confirmed, beside
    the source and freshness that are already named there. No surface, alert,
    notification, or export names or implies an account for a Claude figure. The
    accepted consequence is stated so it can be checked rather than discovered:
    after a Claude account change with no intervening successful probe, the
    previous figures are still displayed, with their real age and this
    statement, and nothing claims they describe the account now signed in.
12. **Probe budget.** A user-initiated refresh probes quota with the snapshot,
    provided reading is on. A background snapshot refresh does not probe quota
    unless the quota interval has elapsed. Consecutive failures back off rather
    than retrying at the nominal interval.
13. **Reminders.** With reading off, or with notifications off — both are the
    default — no reminder is evaluated or delivered. With both on, a threshold
    crossing notifies once per window, a reset notifies once, and neither
    repeats while the condition persists.
14. **Statusline consent.** AgentDeck does not modify `~/.claude/settings.json`
    for the statusline route without explicit user consent, and consent is not
    reachable while reading is off. When consent is given and a `statusLine`
    command already exists, that command still runs and its output still reaches
    Claude Code unchanged; when the chained command fails, AgentDeck's wrapper
    does not blank or corrupt the user's status line. Disabling restores the
    prior configuration.
15. **No credential read.** No code path in the delivered topic opens
    `~/.claude/.credentials.json`, `~/.codex/auth.json`, or any keychain entry,
    and none constructs an authenticated vendor request. This is asserted, not
    merely intended.

Verification levels and per-task scope belong to `tasks.md`. Manual acceptance
that a specimen cannot establish — actual notification delivery, actual
menu-bar rendering under system appearance changes — is named in the task that
owns it.

## Surfaces

Three surfaces change. Each has its own document, and the shared
[Product Prototype](../../../prototype/README.md) is the design authority for
all three.

| Surface | Document | Why it is a surface |
| --- | --- | --- |
| Menu-bar popover | [`ux/menubar-quota.md`](ux/menubar-quota.md) | Quota becomes a fifth tab beside usage, breakdown, attribution, and sessions. Both clients, their windows, plan, and reset allowance need room the existing four tabs do not have. |
| Widgets | [`ux/widget-quota.md`](ux/widget-quota.md) | A fifth widget kind beside magnitude, composition, trust, and rhythm, at three sizes. Quota is the one figure worth seeing without opening anything. |
| Settings window | [`ux/settings-quota.md`](ux/settings-quota.md) | Reminders are opt-in and default off, thresholds are configurable, and the statusline route needs explicit consent. All three are settings, and consent in particular must be a deliberate act with visible scope. |

The CLI also gains a command, but it is not a designed surface in the UX sense:
its output shape is a contract question for `architecture.md`, and the
prototype's CLI page carries its rendered form.

## Open questions

These are recorded rather than resolved. None blocks the boundary; each is
named where it will be settled.

1. **Codex reset allowance totals.** Only currently visible credits are
   returned. Whether a spent or expired credit remains listed with a different
   `status` — which would permit a real used/total — was not observable on an
   account with three available and none spent. `architecture.md` must define
   the field as remaining-only until a contrary observation exists.
2. **`planType` values.** One value was observed. The set is unknown, so the
   surface treats it as an opaque label to display, never as an enum to branch
   on.
3. **Statusline payload stability.** Present in `2.1.266` for this account, and
   the subject of an open request to make it declared. Version and plan
   preconditions for its presence are unverified.
4. **`/usage` wording stability.** The parse target is human copy with no
   compatibility promise. Detection of a shape change, and what the product does
   the first time it changes, are `architecture.md`'s problem.
5. **Widget refresh versus probe interval.** Widgets refresh on a 3–5 minute
   target while quota probes are deliberately slower. Whether a widget may
   display a figure older than its own refresh period, and how it says so, is
   settled in `ux/widget-quota.md`.
6. **Where the Claude attribution statement lives on each surface.** The
   boundary requires the statement wherever Claude figures are shown; it does
   not choose a placement. The popover has room beside the source line, a
   large widget may not, and a small one certainly does not. Each surface
   document settles its own placement, and `ux/widget-quota.md` settles what a
   size that cannot carry the statement shows instead.
7. **Base currency of this branch.** This worktree was created from `4737076`,
   and `main` has since advanced by ten commits including the assembled
   `schema-version-signal` surfaces. The surface documents describe this
   branch's prototype state; integration reconciles them under the
   `v0-6-0-contract` topic's `assemble` task.
