---
status: active
created: 2026-09-08
updated: 2026-09-10
---

# Subscription Quota — Architecture

Every field the three surface documents request is provisioned below or refused
with a reason. A refusal is a real answer, and the surface adopts it rather than
inventing a substitute.

Scope: the quota domain, its two client adapters, the probe schedule, the alert
evaluator, storage, the desktop wire addition, and the CLI surface. Everything
outside those is unchanged.

## C0 — The boundary that shapes everything else

AgentDeck never reads a credential and never constructs an authenticated vendor
request. Both routes invoke a client the user has already signed in, as a child
process, and read what it prints.

This is not an implementation preference; it is what makes the topic compatible
with `docs/specs/cli-design.md:54-55`. It is enforced in three places rather
than trusted once:

1. No package added by this topic imports a keychain API or opens
   `~/.claude/.credentials.json`, `~/.codex/auth.json`, or any file under a
   credential directory.
2. No package added by this topic constructs an HTTP client. The one existing
   network path stays where it is, in `internal/usage/price_update.go`.
3. A test asserts both, over the topic's own packages, so the property survives
   a later change that would otherwise quietly reintroduce it.

`requirements.md` clause 15 is that test.

## C1 — The gate

**Contract.** A client is probed only when quota reading is on *and* its current
provider is `official`. The two conditions are ordered, not interchangeable:
`quotaProbe` off suppresses every probe for both clients regardless of provider,
and the provider check runs only for clients that survive it. `requirements.md`
clauses 1 and 3 are these two conditions, and they carry different reasons —
`probe_disabled` for the switch, `not_official` for the provider — because a
user who turned reading off is owed a different sentence than a user on a relay.

Resolution uses `provider.Service.Current` (`internal/provider/service.go:692`),
which returns the latest completed selection per client without decrypting
credentials, compared against `OfficialProviderName` (`:118`).

**A recorded selection is not an observed one.** `Current` returns AgentDeck's
record of the last switch it performed. `internal/usage/routes.go` separately
carries `config_matched`, `observed_provider`, and `conflict_scan` derived from
Hook deliveries — what the client is actually configured with. These can
disagree, when a user edits a client config outside AgentDeck.

The gate uses the **recorded** selection, and this is a deliberate choice with a
stated cost: the recorded value is always available, needs no Hook delivery, and
cannot itself fail, whereas the observed value is absent for a client that has
not delivered a Hook event since the change. Using the observed value would make
the gate silently stop working for users without Hook integration.

The cost is that a user who edits a client config by hand may be probed when
they should not be, or not probed when they could be. That is mitigated, not
solved: when an observed provider is available and disagrees with the recorded
one, the probe is suppressed and the client reports `not_official`. Suppressing
on disagreement is the safe direction — it never probes an account the product
is not sure it should.

**Refused:** re-deriving the provider from the client's own configuration files.
That is a second parser for a contract another topic owns.

## C2 — The Codex adapter

**Transport.** `codex app-server`, JSON-RPC over stdio, as a child process.

**Handshake.** Three steps, in order, before any request is answered:

```text
→ {"method":"initialize","id":1,"params":{"clientInfo":{…}}}
← {"id":1,"result":{…}}
→ {"method":"initialized"}
→ {"method":"account/rateLimits/read","id":2}
← {"id":2,"result":{…}}
```

Writing both requests and closing stdin returns only the initialize result. The
adapter therefore keeps stdin open and reads until the matching `id` arrives or
the deadline expires; it must not treat process exit as an empty answer.

**Field mapping.** The response is mapped, not copied. Vendor names are
camelCase and the domain uses snake_case, and the domain deliberately carries
fewer fields than the vendor returns:

| Vendor | Domain | Note |
| --- | --- | --- |
| `rateLimitsByLimitId[k].primary/.secondary` | `windows[]` | Each becomes one window; `limitId` plus `primary`/`secondary` forms the stable key |
| `rateLimitResetCredits.credits[].title/.status/.grantedAt/.expiresAt` | `reset_allowance.credits[]` | The disclosure detail |
| `rateLimitResetCredits.credits[].id/.description` | — | **Refused**: an account-scoped identifier, and vendor copy |
| `.windowDurationMins` | `window_minutes` | Drives the window label |
| `.usedPercent` | `used_percent` | |
| `.resetsAt` | `resets_at` | Epoch seconds, kept as an instant |
| `limitName` | `label` | Optional; absent on the main limit |
| `planType` | `plan` | **Opaque.** Displayed, never branched on |
| `rateLimitResetCredits.availableCount` | `reset_allowance.remaining` | |
| `accountId` | `account_id` | Isolation key only; never displayed |
| `credits` | `billing` | |
| `spendControlReached`, `rateLimitReachedType`, `individualLimit`, `rateLimitUpsell` | — | **Refused** for this topic |

**The window count is not fixed, and the adapter must not assume the sampled
shape.** The one response this topic observed came from a `prolite` account,
which returned `primary` only on its main `codex` limit — `requirements.md`
records that observation and no other. Whether another plan returns both a
`primary` and a `secondary` on the main limit **has not been observed here**;
it is an assumption, and it is named as one rather than stated as fact.

The mapping is written so that the assumption does not matter: every present
`primary`/`secondary` under every `limitId` becomes a row, and nothing reads a
count or a slot. Writing it against the single observed response would have
produced a contract in which a main limit can have only one window — a contract
that breaks on the first account shaped differently, which is the risk the rule
avoids regardless of which plans turn out to be shaped that way.

The specimen's `codexPlus` fixture is constructed from this assumption, not
recorded from a vendor response, and its own comment says so.

**`reset_allowance.total` is null, with reason `not_reported`.** The response
lists currently visible credits; a spent or expired credit was not observable on
the sample account. Deriving a total from `availableCount` would be inventing a
number. `requirements.md` open question 1 owns revisiting this.

**Refused fields** are refused because they describe refusal and upsell states
this topic does not present. Adding them later is additive.

**Failure classification.** Spawn failure, non-zero exit, deadline, malformed
JSON, and a well-formed response missing `rateLimits` are distinct outcomes, and
the last one is `not_reported` for the affected fields rather than a probe
failure — an account without plan limits is not an error.

## C3 — The Claude status-line adapter

**Contract.** When the user consents, AgentDeck registers a `statusLine` command
in `~/.claude/settings.json`. Claude Code invokes it on each status-line refresh
of a live interactive session, passing a JSON payload on stdin that carries:

```json
{"rate_limits": {
   "five_hour": {"used_percentage": 0.0, "resets_at": 0},
   "seven_day": {"used_percentage": 0.0, "resets_at": 0}}}
```

The command captures those fields and returns. It is a capture point, not a
request: AgentDeck cannot ask for this payload, only receive it.

**`window_minutes` is supplied locally, not by the vendor.** The payload carries
no window length, and neither does the prose route, but the menu bar needs one
because the length is what produces the label. Rather than refuse the field and
leave the surface without a label, the adapter maps the two fixed window names
to their lengths — `five_hour` to `300`, `seven_day` to `10080` — as a local
constant.

This is safe precisely because it is not an inference from data: the names
themselves state the lengths, so the constant cannot drift from the payload the
way a derived number could. It is recorded as a local constant rather than
presented as a vendor field so that a future payload carrying a real length
replaces it rather than silently agreeing with it. If Claude ever reports a
window whose name is not one of these two, it is `not_reported` for the length
and the surface labels it by name alone.

**Chaining is mandatory when a command already exists.** The registered command
reads stdin once, persists the quota fields, then invokes the user's prior
command with the same stdin and passes its stdout through unchanged. Three
properties are required and tested:

1. The prior command's stdout reaches Claude Code byte for byte.
2. A failure in the prior command does not blank the status line beyond what
   that command's own failure would have caused; AgentDeck's wrapper adds no new
   failure mode.
3. A failure in AgentDeck's capture does not prevent the prior command running.

The capture path therefore writes after invoking, or writes in a way that cannot
abort the chain.

**Registration** reuses the surgical editing already in
`internal/usagehook/config.go` — `topLevelValueSpan` and `insertTopLevelValue`
manage the top-level `hooks` key while preserving the rest of the file. The
`statusLine` key is the same class of operation on the same file. The prior
value is recorded before the write, and disabling restores it.

**Restore is best-effort and says so.** If the file changed since registration
such that the recorded prior value no longer matches what is present, disabling
removes AgentDeck's command and reports that a manual check is needed rather
than overwriting a value the user may have edited deliberately.

**This payload is not a contract.** Anthropic issue #27915 requests that it
become part of the declared statusLine input; today it is observed behavior in
`2.1.266`. The adapter therefore treats a missing `rate_limits` as
`not_reported`, never as a failure, and never assumes the key will be present.

## C4 — The Claude prose adapter

**Transport.** `claude -p "/usage"` as a child process. Observed
`total_cost_usd: 0`, `duration_api_ms: 0`, `num_turns: 0` — the slash command is
local and consumes no quota. `--output-format json` adds no structured keys; the
same prose arrives in `result`, so the adapter reads plain stdout.

**Parsing.** Two lines are extracted, each yielding a percentage and a reset
instant:

```text
Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)
Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)
```

The reset instant is a localized wall-clock string with a named zone, which must
be resolved against that zone rather than the local one — the two differ for any
user whose machine zone is not their account zone, and the resulting countdown
would be wrong by hours rather than obviously broken.

**All-or-nothing.** If either line fails to match, the whole client result is
`parse_failed` with the failed attempt's instant, and **no** partial figure is
emitted. A surface that shows the session window while silently dropping the
weekly one is worse than one that says it could not read the output.

**The "what's contributing" section is discarded.** It states on its face that
it is derived from local sessions on this machine, which is data AgentDeck
already computes itself and computes over a scope it controls. Presenting the
vendor's local derivation beside AgentDeck's own would create two answers to one
question.

**Shape-change detection.** The parser is strict, and a parse failure is
reported as such rather than degrading to a looser pattern. A looser pattern
that matches future copy by accident is the failure mode this rule prevents: it
would produce a number with no signal that the source changed.

## C5 — Source precedence and freshness

Per client, the newest usable observation wins, with one tiebreak: the
status-line route is preferred over the prose route at equal age, because it is
structured and carries epoch instants.

The chosen `source` is carried in the payload so the surface can name it. A
figure never loses its provenance on the way to a surface.

**Staleness** is a function of the window it describes, not a global constant. A
five-hour window tolerates a shorter absolute age than a weekly one before its
percentage is misleading, so the threshold scales with the window:

```
allowed_age(window) = max(window_minutes / 10, 2 × probe_interval)
stale                = age(observation) > allowed_age(shortest window on the client)
```

| Window | `window_minutes / 10` | Allowed age at the 5m interval | at 30m |
| --- | --- | --- | --- |
| 5-hour | 30 min | 30 min | 60 min |
| 7-day | 1008 min | 16 h 48 min | 16 h 48 min |

Both halves of the `max` earn their place. **The tenth** is the scaling itself:
a tenth of a window is roughly the point at which a percentage can have moved
enough to change what the user would do about it, and it produces sane figures
at both ends without a table of hand-picked constants — a weekly figure a few
hours old is still worth showing, a five-hour figure an hour old is not. **The
floor at twice the probe interval** stops the flag from firing on the product's
own cadence rather than on the data: at a 30-minute interval a 30-minute
threshold would mark almost every figure stale just before its next scheduled
refresh, which tells the user their configuration is broken when it is
behaving exactly as configured. Twice, because one interval leaves no room for
the jitter of a refresh that has not fired yet.

The flag is computed per client from its **shortest** window, so a card is
marked stale as soon as any figure on it is. A card carrying a fresh weekly
figure and a stale five-hour one is stale: the tighter window is the one the
user is about to hit.

This is a threshold, not a promise about accuracy. It says when the product
stops presenting a number as current; it does not claim a number under the
threshold is right.

**Never observed, failed, and stale are three states**, and the payload
distinguishes them: no observation at all, an observation attempt that failed
with a reason, and a successful observation that is too old. Each has its own
copy on every surface.

## C6 — The domain model

The three reset semantics are separate fields, not one field with a label:

| Field | Meaning | Source |
| --- | --- | --- |
| `windows[].resets_at` | This window reopens at this instant | Both clients |
| `reset_allowance.remaining` | Official resets the account may still spend | Codex only |
| `observed_reset_at` | AgentDeck saw `used_percent` fall | Derived locally |

**`window_key` identifies one window across observations**, and every store and
dedup key below is built on it, so it is defined here rather than assumed:

| Client | Form | Values seen |
| --- | --- | --- |
| Codex | the vendor's `limitId`, plus `_secondary` when the window came from the `secondary` slot | `codex`, `codex_secondary`, `codex_bengalfox`, `codex_bengalfox_secondary` |
| Claude | the payload's own window name, which is fixed at two | `five_hour`, `seven_day` |

The Codex form is derived rather than invented because `limitId` is already the
vendor's stable identifier for a limit, and the slot is the only thing that
distinguishes two windows under one limit. The `primary` slot takes the bare
`limitId` so the common single-window case has the shortest key. Claude's two
names are the vendor's own keys in the status-line payload, and the prose route
is parsed into the same two, so one client cannot produce two vocabularies for
the same window depending on which route answered.

A key is opaque to every surface: it is a join key for storage, alert
deduplication and reset detection, never a label. Labels come from
`window_minutes` and `label`.

`observed_reset_at` is derived by comparing a new observation against the stored
previous one for the same client, account, and window key. It is recorded only
when the used share decreases; a decrease is the only local evidence of a reset.
It is explicitly not inferred from `resets_at` passing, because a passed instant
proves only that the window *should* have reset.

Every absent field carries a reason from a closed set — `not_reported`,
`not_official`, `never_probed`, `probe_failed`, `parse_failed`,
`not_consented`, `probe_disabled`. The set is closed so that both surface
languages can render every member; a free-text reason would arrive
untranslated.

## C7 — Storage

The latest observation per `(client, account_id, window_key)`, plus the latest
per-client envelope. No time series.

This is a deliberate refusal of a capability the data would support. A quota
history is a separate goal with its own retention, privacy, and presentation
questions, and storing it "just in case" would decide those questions by
accident. `requirements.md` lists it as a non-goal.

One prior observation per key is retained, which is what `observed_reset_at`
needs — that is a working value, not a history.

## C8 — Account isolation

`account_id` from the Codex response scopes every stored Codex observation. When
a probe returns a different `account_id` than the stored one, the stored
observations for that client are discarded rather than merged, and the new
account starts from no observation.

Claude reports no account identifier through either route. Claude observations
are therefore scoped to the client alone. This is not the architecture narrowing
the requirement: `requirements.md` names the gap, records the operator's
2026-09-09 decision to accept it, and states the obligation that comes with
acceptance. If a user switches Claude accounts, the previous account's figures
are shown until the next successful probe replaces them.

Two consequences are structural rather than cosmetic, so they live here as well.
The stored Claude key is `(client, window_key)` with no account component, and
nothing in the domain, the wire, or the alert evaluator may assert an account
for a Claude figure — including a notification, whose content already carries no
account identifier for a different reason. And every payload carrying Claude
figures carries the attribution limitation as a field rather than leaving each
surface to remember it: `attribution_confirmed: false` for Claude, `true` for
Codex, so a surface that forgets to render it is a visible omission rather than
an invisible one. Freshness bounds the exposure in time; it cannot establish
attribution, and the two are not substitutes.

`account_id` is an isolation key and never reaches a surface, a log, or an
exported file.

## C9 — Probe scheduling

Every row below presumes C1 has already passed. With reading off, the scheduler
does not run at all, and no row applies.

| Trigger | Behavior |
| --- | --- |
| Any trigger, reading off | No probe. The scheduler is not started, no subprocess is spawned, and no status-line route is installed — see the transition below for how it gets that way. |
| User-initiated refresh | Quota is probed with the snapshot. Neither the interval nor backoff gates a refresh the user asked for; the reading switch and the provider gate still do. |
| Background snapshot refresh | Quota is **not** probed unless its own interval has elapsed. |
| Quota interval elapsed during background refresh | Probed once, then the interval restarts. |

**Turning reading off unregisters an installed status-line route, as part of
that action.** This is the case the first row above describes as a state, and it
does not reach that state by itself: a user who consented, got the route
installed, and later turns quota reading off leaves AgentDeck's command sitting
in `~/.claude/settings.json`, invoked by Claude Code on every status-line
refresh of a live session. Three dispositions were possible and two are worse:

| Option | Why not |
| --- | --- |
| Keep it installed, keep storing what it pushes | The switch would be a lie. It says nothing is being read while a capture point runs on every refresh and fills the store. |
| Keep it installed, discard what it pushes | Honest about the data and dishonest about the footprint. Another tool's config still runs AgentDeck's command indefinitely, for nothing, and the user has no indication from the quota switch that it is there. |
| **Unregister, as part of turning reading off** | Chosen. |

**Turning the switch off is a user action, and the restore it performs is a
write that action authorises** — the same restore the status-line switch already
performs when turned off individually, described in C3 and in the settings
document's consent hint. `requirements.md` clause 1 constrains the **off state**
— while reading is off, nothing is written — and this write happens during the
transition into it, not while in it. No requirement wording needs to change for
this reading; the clause simply never addressed the transition.

Consequences, so an implementer does not have to derive them:

- The restore follows C3 exactly, best-effort included. If the recorded prior
  value no longer matches the file, AgentDeck removes only its own command and
  reports that a manual check is needed — the `restore incomplete` outcome the
  settings window already renders. Turning reading off can therefore surface
  that row, and it means the same thing there as it does on the status-line
  switch itself.
- The status-line consent flag goes to off with the route. Reading is the parent
  capability; consent to a route that cannot run is not consent that should be
  silently held for later. Turning reading back on leaves the route
  unregistered until the user consents again, which is the only reading under
  which the consent hint's promise about writing to another tool's file stays
  true.
- Stored observations are untouched by any of this. Unregistering is about the
  other tool's file; retention is about ours, and the next paragraph owns it.

**Specimen gap, named rather than left to be discovered.** The prototype does
not model this transition: turning the parent switch off leaves the status-line
switch rendering its previous `on` state, disabled. That contradicts the rule
above, which requires it to read off. The specimen is the design authority for
the settings window, so the correction belongs to `ux/settings-quota.md`'s own
pass rather than to this document; it is recorded here and on that document's
task so it cannot be lost between the two.

**Turning reading off retains storage.** Stored observations are not deleted;
they stop being read for display, and every field reports `probe_disabled`. This
is `requirements.md` clause 2, and it is the reason the switch is cheap to
toggle: turning it back on renders the retained observations at their true age
under the ordinary staleness rules, without a probe.

The interval is user-selected from 5m, 15m, 30m and defaults to 5m. The snapshot
cadence — approximately one minute for the menu bar — never drives a probe,
because that would spawn a subprocess per minute for data that changes on a
five-hour or weekly window.

**Backoff.** Consecutive failures back off geometrically from the configured
interval to a bounded maximum, and a success resets it. A backed-off client is
still displayed, with its last figure and its real age, or as never-observed.
Only a background failure advances this chain. A manual refresh is neither
gated by backoff nor a contributor to it (gate-and-schedule task, GS-R2-F1):
a manual failure is recorded, but leaves the background schedule's backoff
exactly where it already was, so repeated manual retries cannot push a
background probe out by pushing the same chain forward.

**Concurrency.** At most one probe per client in flight, per process. A
refresh arriving while a probe is running in the same process does not start
a second. This is an in-process guard, not a cross-process one: each trigger
runs as its own short-lived process, so it cannot itself observe or wait for
a probe another process happens to have in flight at the same moment — that
case is accepted as out of scope rather than solved with a new cross-process
lock, since the one production caller never issues concurrent probes for the
same client to begin with (gate-and-schedule task, GS-R1-F3).

## C10 — Alerts

Off by default. With alerts off, the evaluator does not run — the requirement is
that nothing is *evaluated*, not merely that nothing is delivered.

**Threshold crossing** fires when a window's used share crosses a configured
threshold upward. Deduplication is per `(client, window_key, threshold, window
instance)`, where the window instance is identified by its `resets_at`: a
threshold fires at most once per window occurrence, and the same threshold fires
again after the window resets. Repeated probes while a window sits above a
threshold produce no further notification.

**Reset notice** fires once per window occurrence when a reset is observed — the
same local evidence that sets `observed_reset_at`, not the passing of
`resets_at`.

Four readings of the above, fixed so an implementer does not have to choose.
Their sources differ: the first two are operator-approved decisions of the
quota-alerts task; the last two were added while repairing that task's review
findings, and are defensive choices open to revision rather than operator
decisions.

- **Same instance** (operator-approved). Two `resets_at` values within 15
  minutes of each other name the same occurrence. The routes report one
  occurrence at different precision — the prose route to the minute, the
  status-line payload and Codex to the second — so exact equality would notify
  twice in one occurrence; 15 minutes is far below the shortest window, so two
  real occurrences are never merged. A window with no `resets_at` has no
  identifiable occurrence and gets no threshold notice.
- **Crossing** (operator-approved). A threshold notice is due when the used
  share is at or above the threshold and none has been sent for that
  occurrence. The evaluator does not require having seen the previous figure
  below it: the status-line capture writes observations without passing
  through the evaluator, so an edge-triggered rule would miss those crossings.
  Alerts switched on while a window already sits above a threshold therefore
  notify once for that occurrence.
- **Only an ongoing occurrence notifies** (review finding QA-R1-F1). A stored
  window whose `resets_at` is already more than 15 minutes in the past
  describes an occurrence that has ended; it sends neither notice. When probes
  stop succeeding, the last good window stays stored (C9), and notifying from
  it would report a past figure — and, once its ledger entry is pruned, report
  it again on every evaluation.
- **Reset notice, once per occurrence** (review findings QA-R1-F2 and QA-R2-F1;
  the unknown-length narrowing below was chosen by the repair, not decided by
  the operator). A reset notice is deduplicated by the
  occurrence's `resets_at`, with the same tolerance, not by
  `observed_reset_at`: a small same-source drop later in the same occurrence
  moves `observed_reset_at` forward (C6) but is not a second reset. It is sent
  only when that `observed_reset_at` falls inside the current occurrence, so a
  reset observed while alerts were off does not notify after the window has
  moved on. A window without `resets_at`, or whose length is not reported,
  gets no reset notice: without a length the current occurrence cannot be
  bounded, and a sticky `observed_reset_at` from an earlier occurrence would
  otherwise notify again in every later one — a false notice, where skipping
  costs only a missed one.

Deduplication state is a small ledger of sent notices (`quota_alert_notices`),
recorded only after delivery succeeds so a failed delivery is retried, and
pruned once an occurrence ended more than 31 days ago. Because only ongoing
occurrences are evaluated, and an instance matches within 15 minutes, no
pruned entry can ever be needed again. It holds no account identifier and no
quota figure.

Notification content names the client, the window, and the figure. It carries no
account identifier.

**Delivery belongs to the app** (operator decision, 2026-09-16, from manual
acceptance). The helper does not post notifications. A notification posted from
a helper process through `osascript` is attributed to Script Editor: the user
sees the wrong sender, cannot switch AgentDeck's notifications off on their own,
and a disabled Script Editor still reports success, so a notice is recorded that
nobody saw. Instead:

1. `desktop quota-refresh` runs the evaluator and returns the notices that are
   due in its result's `alerts` array, without recording them. Each carries an
   opaque `id` naming its ledger key, the client, the kind, the used figure, the
   threshold for a threshold notice, and the window's vendor label and length
   so the app can name the window. It still carries no account identifier.
2. The app posts each notice through the user-notification service under its
   own bundle identity, using the notice `id` as the request identifier, and
   builds the title and body from its localized copy in the app's language.
3. For each notice the service accepted, the app runs
   `desktop quota-alerts ack --id <id>`, which records the ledger entry. A
   notice the service refused — permission denied or not yet granted — is not
   acknowledged, stays due, and is offered again by the next refresh. Recording
   an unrecognized or malformed `id` is an input error and records nothing.

Delivery is therefore at least once per occurrence rather than exactly once: a
crash between posting and acknowledging offers the notice again, and the
repeated request identifier replaces the earlier notification instead of adding
a second one. Notification permission is requested when the user turns
`quotaAlerts` on, never at launch; a denied or disabled permission leaves the
switch on and is presented in Settings (ux/settings-quota.md). With alerts or
reading off the evaluator still does not run, and `alerts` is empty.

## C11 — Desktop wire

The snapshot gains one optional top-level section beside `provider`, `usage`,
`sessions`, and `health`:

```go
type Snapshot struct {
    WireVersion   int                   `json:"wire_version"`
    …
    Subscription  SubscriptionSnapshot  `json:"subscription"`
}
```

**`WireVersion` stays at 1.** `internal/desktop/desktop.go:21` sets it, and
`:38` rejects a mismatch. The addition is additive: an older consumer ignores an
unknown key, and a newer consumer reading an older payload sees the section
absent, which is indistinguishable from "not probed" and renders as such. This
matches how v0.5.0 added Work Signals to the same wire.

The section carries, per client: `applicable` and its reason, `source`,
`observed_at`, `stale`, `attribution_confirmed`, `plan` and its reason,
`windows[]`, `tightest_window_key`, `reset_allowance` and its reason,
`observed_reset_at`, and `failure`. It does not carry `account_id` or
`billing.balance`.

**`tightest_window_key`** names the window with the highest `used_percent`,
resolved once when the payload is built. It is null when the client has no
window. It exists because the next paragraph's claim would otherwise be
unbacked: without it, a widget picking "the window that will stop the work
first" has to derive it, and a derivation is what that paragraph says must not
happen twice.

Ordering was the alternative and was rejected. Making `windows[]` sorted by used
share would have carried the same information, but the popover deliberately
lists windows in the vendor's own order — the account's main limit first, then
per-model limits — and that order is part of the menu-bar design. A payload
cannot be sorted for one surface and unsorted for another, so the answer travels
as its own field and `windows[]` keeps the order the surfaces already agreed on.

**Tightest-window resolution happens in the payload**, not in the widget: it is
the `tightest_window_key` field above, and a surface reads it rather than
computing it. Both the popover and the widget must agree on which window is
tightest, and two independent derivations are two chances to disagree — the same
class of defect
as the provider footer contradicting the quota panel.

**Widget client projection is explicit.** Small and medium both receive one
configured client (`codex` or `claude`) through the Widget configuration/AppIntent
and project only that client. Small projects its window with the highest used
share; medium projects all of that client's windows. Neither falls back to the
other client when the configured one has no usable observation. Large projects
both clients, filtered to those with at least one usable window, in
Codex-then-Claude order, split into equal halves **of the full card height**,
each half centring its own content; a client with no usable window produces no
block; with a single client the block keeps its content height and centres rather
than stretching. Equal halves and a filled card are separate properties — a
content-sized container satisfies the first while failing the second.

**Widget window rows carry the vendor's window label when present.** The same
`label` the popover shows, for the same reason: with per-model limits, span alone
repeats and the rows become indistinguishable. The widget does not prefix rows
with the client name, because the block or frame header already carries it.

**No widget size projects the reset allowance.** It is popover-only: the count
is meaningful only alongside each credit's status and expiry, and no widget size
has room to open that detail. The wire still carries `reset_allowance` for the
popover and the CLI; the widget simply does not read it.

The prototype exposes both quota-state and small-client selection as visible
stage controls; their query parameters are only stable deep links for automation.

**Reset-credit detail is a hover-opened side popover.** The summary row remains
in the Codex card; hover or keyboard focus opens a labelled dialog anchored to the
row, while the main panel stays fixed and visible. The side is computed from the
room actually available at open time — right when the panel's right edge plus the
popover width and gap fit the viewport, otherwise left when that fits, otherwise
overlaying the panel's right edge. The menu-bar icon can sit anywhere along the
screen, so a fixed side would open off-screen for some users. The popover carries
title, status, grant age and expiry per credit, and no account-scoped credit ID or
vendor marketing description.

## C12 — CLI

One command, `agentdeck quota`, with the project's existing text and `--json`
forms. It is the CLI's answer to the same question and the desktop's data
source, so it carries the whole section rather than a summary.

Error envelope follows the project's stable-code convention. The gate is not an
error: a client that is not `official` exits 0 with its `applicable: false` and
reason in the payload. A probe failure is likewise a payload state, not a
non-zero exit, because a partial answer — one client succeeded, one failed — is
the normal case and must be printable.

A non-zero exit is reserved for conditions that prevent producing a payload at
all, and reuses existing codes rather than adding new ones.

## Verification

| Level | What |
| --- | --- |
| L0 | Documentation, links, prototype build, the prototype's own gauge and contract board |
| L1 | Adapter parsers against captured fixtures — Codex JSON-RPC responses and `/usage` prose, including the malformed cases; the closed reason set; the three reset semantics |
| L1 | The gate, including the recorded-versus-observed disagreement suppressing a probe |
| L2 | Scheduler behavior — manual follows the snapshot, background respects the interval, backoff, single-flight |
| L2 | Reading off — no scheduler, no subprocess for any trigger including a manual refresh, no status-line registration, no alert evaluation, and stored observations retained but reported `probe_disabled` |
| L2 | Turning reading off while a status-line route is installed unregisters it through the C3 restore, clears the consent flag, surfaces `restore incomplete` when the file changed underneath, and leaves stored observations alone |
| L1 | `allowed_age` at both window lengths and at each selectable interval, including the floor taking over at 30m; the flag following the shortest window on the client; and the boundary — one minute either side of the threshold |
| L1 | `window_key` construction for both clients, including a Codex `secondary` slot and both Claude routes producing the same two keys |
| L1 | Claude `window_minutes` supplied from the window name, and `not_reported` for a name outside the two |
| L1 | `tightest_window_key` names the highest `used_percent`, is null with no windows, and `windows[]` keeps vendor order rather than being sorted |
| L1 | `attribution_confirmed` is false for every Claude payload and true for Codex |
| L2 | Alert deduplication across window instances, and the off-by-default no-evaluation property |
| L2 | Status-line chaining: prior stdout passthrough, prior-command failure isolation, capture-failure isolation, register and restore |
| L2 | Account change discards rather than merges |
| L1 | The no-credential property, asserted over this topic's packages |
| Manual | Everything the three surface documents list under "what the specimen does not settle" |

No test contacts a vendor. Every adapter test runs against a captured fixture,
and the fixtures are the responses recorded on 2026-09-08 in `requirements.md`,
with the account identifier replaced.

## Open contract questions

Carried from `requirements.md`, restated as contract decisions taken under
uncertainty:

1. `reset_allowance.total` is null with `not_reported` until an observation
   proves a total is derivable. Revisit only with evidence, not by inference.
2. `plan` is an opaque display string. No branch anywhere reads its value.
3. The status-line payload's presence is not assumed; absence is
   `not_reported`.
4. The `/usage` prose shape is strict-matched; a change is a reported failure,
   never a looser match.
5. Claude has no account identifier. The limitation is an accepted product
   decision recorded in `requirements.md`, carried in the payload as
   `attribution_confirmed: false`, and implemented in C8 — not worked around.
6. Whether any plan returns both a `primary` and a `secondary` on the main
   `codex` limit is unobserved. C2 states it as an assumption and the mapping
   does not depend on it; a real response either way changes nothing here.
7. `allowed_age` uses a tenth of the window with a floor at twice the probe
   interval. Both are chosen, not measured — no observation in this topic says
   where a percentage stops being useful. Revisit with a real complaint about a
   figure marked stale too early or too late, not by adjusting the constant to
   taste.
