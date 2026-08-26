---
status: active
created: 2026-08-17
updated: 2026-08-26
---

# Switch Effectiveness Boundary — Requirements

`v0.5.0` has selected this topic. Version membership is decided by a
`vX-Y-Z-contract` topic's assembly list, not here; the
[`v0.5.0` contract topic](../v0-5-0-contract/tasks.md#assembly-list) records the
selection and the reason. See `.agent-instructions/branching.md` for how a
selected topic's branch reaches a tag.

## The defect

AgentDeck tells the user that a Claude provider switch reaches a running session
without a restart, and it treats the managed settings file as proof of which
provider a session is actually using. The first claim is true for exactly one
state transition: a session that started without an API key may adopt its first
API key from the live settings change. It is false once that session already
holds a key, and it is false when a key is removed.

`WriteClaudeConfig` derives the same pair of owned fields from every new
selection, setting or deleting each independently without knowing the running
session's prior authentication state
(`internal/provider/config.go:654-663`):

| Running-session state | New selection | Managed fields | Reaches the running session |
| --- | --- | --- | --- |
| Started without an API key | first custom API key | endpoint and credential written | **Yes** |
| Already authenticated with API key A | custom API key B, including another custom provider | endpoint and credential written | **No** |
| Already authenticated with an API key | `official`, direct | endpoint and credential deleted | **No** |
| Already authenticated with an API key | `official --via` wrapper | endpoint written; credential deleted | **Not for billing** |
| Any other running state | endpoint-only or other configuration change | varies | **No supported live-effect claim** |

The first row is the sole live-update exception. It was observed in real use on
2026-08-17. The key-rotation row was separately confirmed in real use on
2026-08-25: once a session already holds an API key, writing another key does
not replace the credential used by that process. Deleting a key likewise does
not return the process to subscription authentication. In all three cases the
session keeps the authentication it already negotiated until restart.

The `official --via` row is a mixed state and is called out separately because
it is the one a single "direction" reading gets wrong. That selection writes an
endpoint and deletes the credential (`ConfigMatchesOfficialWrapper`,
`internal/provider/config.go:109-141`, checks exactly that shape), so the running
session may reach a new endpoint while still presenting the old token. The
endpoint change is observable and the credential change is not. Since it is the
credential that determines what is billed and at whose rate, this row is treated
as not reaching the session, the same as the direct row.

**The discriminant is therefore the running session's transition, not whether
the new configuration merely contains a credential.** A matched write may
produce the one effective `no key -> first key` transition, or it may be an
unadopted `key A -> key B` replacement. Keying only on the new provider or on
`config.Credential == ""` cannot distinguish them. When prior session evidence
does not prove the live-update exception, restart semantics apply.

Two things in the product assert otherwise.

**The advisory is unconditional.** `claudeRestartAdvisory`
(`internal/provider/service.go:808`) states that a running client reads its
settings live, so the switch can reach a session mid-conversation. Every Claude
switch prints it, including the direction where it is false — where the switch
reaches nothing until the session restarts. `docs/specs/cli-manual.md:283-285`
documents the same claim in the same unconditional form.

**Attribution treats a file as evidence of a process.**
`ClaudeConfigMatchesSnapshot` (`internal/provider/config.go:145-153`) compares
only the fields on disk. Replacing a key or switching back to subscription can
make the file match the new selection, so `RecordClaudeConfigChange` records
`matched = true`
(`cmd/agentdeck/main.go:2939`, `internal/usage/routes.go:45-54`), and the route
carries the new provider and multiplier. The running session may still be
billing against the prior API key. Every subsequent event can then be priced at
the new provider's rate while the money is spent at the old one.

The information needed to price them correctly was already there. The route
recorded when the session first obtained the API key names the provider it is
still using; a later key replacement or removal overwrites that answer with a
provider the session never adopted. Nothing became unknowable — a correct
attribution was replaced by an incorrect one.

Nothing on disk can detect the discrepancy. Key rotation overwrites
`ANTHROPIC_AUTH_TOKEN`; key removal deletes it. Either way the old value held by
the running process leaves no file residue. `ClaudeCredentialConflicts`
(`:173-191`) detects only `env.ANTHROPIC_API_KEY` and `apiKeyHelper`, the two
sources AgentDeck does not own. The fix therefore derives the effective provider
from session evidence rather than treating the new file as process state.

Only the first-key direction is unaffected: when session-start evidence shows
there was no managed API key, the matched `ConfigChange` may record the new
custom-provider route because disk and process agree.

Codex's **reload behavior** is unchanged because it activates no configuration
without a restart, so its advisory remains correct
(`internal/provider/service.go:807`). Codex is nevertheless in scope for the
shared Hook operation contract below: once either client delivers an accepted
Hook event, AgentDeck must normalize, persist, classify, and commit it through
the same pipeline. Raw payload fields and the runtime fact used to classify
`route_effect` may differ by event; storage, transaction, privacy, replay, and
resolver rules may not differ by client.

## Goals

- **Hook operations are client-neutral after normalization.** Codex and Claude
  adapters may accept different raw event sets, but every accepted delivery gets
  one `delivery_id`, one normalized observation, one shared `route_effect`
  classification, and one atomic persistence operation. No client owns a
  separate observation table, transaction path, privacy rule, or retry contract.
- **A switch's effectiveness claim is state-aware and conservative.** A Claude
  switch states that only `no key -> first key` may apply live. It requires a
  restart for key replacement, key removal, and every other transition whose
  adoption by the running process is not established.
- **Disk state is not evidence of process state.** Reconciling the managed
  settings file proves what the file says. A switch does not become a session's
  route merely because the file now describes it; the caller must also prove
  that the session was in the one state that can adopt the change live.
- **A session keeps the latest effective attribution it adopted.** After a switch
  the running session cannot adopt, attribution continues to resolve first to
  that session's latest prior effective route. Only when the session has no route
  does it resolve through the provider timeline at session start, followed by the
  existing no-coverage fallback. This hierarchy, stated below, preserves a
  first-key route across a later key rotation instead of incorrectly returning to
  the provider at session start.
- **The invalid premise is corrected wherever it was written down.**
  The earlier `usage-attribution-precision` architecture derived `exact` for
  every Claude `ConfigChange` route from "Claude activates immediately". Its
  current draft now limits effective live routes to `no key -> first key` and
  keeps the session-start fallback for unadopted changes. That document has
  never been reviewed, so the correction is made before any work derives from
  the invalid premise.

## What an unadopted switch attributes to

A key replacement, key removal, or other switch the running session cannot have
adopted leaves that session's routing unchanged. Attribution therefore keeps
resolving to what was already in effect. That is achieved by **recording no
effective route**, and effective-route resolution needs no new representation:
the existing resolution order already produces the right answer once the
misleading row is not written. Only a matched `no key -> first key` transition
records a new `ConfigChange` route.

**Recording no route is not discarding the observation.** Every accepted Hook
delivery from either client is a fact AgentDeck observed and is persisted through
the same operation. The two stored streams remain separate: an append-only
observation record, which nothing prices from, and the effective-route history,
which advances only when the normalized event establishes a route effect. The
shared operation is client-neutral; event-specific facts decide `route_effect`,
not a separate Codex or Claude storage path. `architecture.md` owns the normalized
envelope, representation, ownership, ordering, idempotency, privacy, and route
effects for both.

`sessionRouteAt` returns the most recent route at or before an event's time
(`internal/usage/usage.go:2504-2510`), and `priceForEvent` falls back to the
provider timeline at session start when a session has no route at all
(`:2622-2634`). With nothing written, an event resolves in this order:

| Evidence available | Resolves to | Quality |
| --- | --- | --- |
| A prior route for this session — from `SessionStart`, or from the effective `ConfigChange` that introduced its first API key | that route's provider and multiplier | as that route was stored, `estimated` |
| No route for this session, but the provider timeline covers its start | the selection in effect at session start | `estimated` |
| No route and no timeline coverage | `unknown`, multiplier `1` | today's `historical` fallback |

Rows two and three are pre-existing behavior reached today by any session without
a route; this topic neither creates nor changes them. Row one is the case the
defect corrupts, and it is the only case the fix touches.

Row two matters for the "no prior route" gap being real rather than theoretical:
`RecordSessionRoute` writes nothing when no completed selection exists
(`internal/usage/routes.go:33-34`) and skips `compact`
(`:29`). A session can therefore reach an unadopted switch with no route
of its own — and the timeline fallback then resolves to the selection at its
start, which is the same answer "keep what you authenticated under" asks for.

**Consequences for cost output, which is what makes this requirement satisfiable
in `v0.5.0`:**

- Events in a session spanning an unadopted key replacement or removal are
  priced at the prior provider's multiplier, because that is the route they
  continue to resolve to.
- No new quality value is introduced and no `usage_session_routes` column
  changes. One additive migration creates the append-only observation table; it
  alters no existing table, and no pricing path reads it.
- No stored row is reinterpreted. The rows that exist keep their meaning; the fix
  is that one misleading row stops being written. That is why this does not cross
  into the historical-recomputation non-goal below: events already recorded under
  the wrong multiplier keep their recorded attribution, and only events after the
  fix ships resolve correctly.
- The `unknown`/multiplier-`1` question is untouched. Whether an unattributed
  route may carry a multiplier at all belongs to
  [`usage-attribution-precision`](../usage-attribution-precision/tasks.md), and
  this topic reaches that state no more often than today.

## Non-goals

- **Changing Claude's own reload behavior.** It is not ours to change, and this
  topic states the boundary rather than working around it.
- **Making the switch take effect in an unsupported transition.** Terminating or
  signalling a running Claude process to force re-authentication is not in
  scope: AgentDeck does not manage client processes, and doing so would end a
  user's conversation.
- **Recomputing historical attribution.** Events already recorded under the
  wrong multiplier stay as they are. Quality-value redesign and backfill belong
  to [`usage-attribution-precision`](../usage-attribution-precision/tasks.md) in
  `v0.5.0`; this topic stops the defect from producing new wrong data and
  corrects that topic's invalid premise.
- **Changing Codex reload behavior.** Codex remains restart-only. Its accepted
  Hook deliveries do participate in the shared observation/effective-route
  operation, so Hook persistence itself is not a Codex non-goal.
- **Observing which credential a running process holds.** No supported mechanism
  exists. The design does not need one: it derives the answer from the previous
  selection instead of observing the process.

## User-visible surfaces

This topic adds no new surface. It changes one stderr advisory, suppresses one
misleading route, and adds one internal cross-client Hook observation table no
command renders, all of which `architecture.md` owns as contract changes. There
is therefore no `ux/<surface>.md`, and `tasks.md` states that row as a decision
rather than omitting it.

## Contracts in scope

Yes. Four, all specified in [`architecture.md`](architecture.md):

- the client-neutral accepted-Hook pipeline and normalized observation contract;
- the Claude switch advisory text, which is a documented stderr contract in
  `docs/specs/cli-manual.md` and `docs/specs/cli-design.md`;
- what `ClaudeConfigMatchesSnapshot` proves, and what a caller may conclude from
  it;
- whether a `ConfigChange` route is recorded for the one supported live
  transition or suppressed for an unadopted switch.

## Acceptance boundary

- A Claude switch **to** a custom provider states that only a session which
  started without an API key may adopt its first key live. It also states that a
  session already authenticated with a key keeps that key until restart.
- A Claude switch that removes a credential or changes other configuration
  states that restart is required. `official` direct and `official --via` are
  both on this side for billing.
- Neither advisory prints a credential value, and a switch that already
  succeeded is never failed by advisory generation.
- A Hook delivery is **accepted** only after the whole ordered admission
  sequence passes: bounded read, `usagehook.ParseEvent` wire validation, store
  and home availability, the managed-path/`user_settings` scope check for a
  Claude `ConfigChange`, and the transcript-scope check for a `SessionStart`
  from either client. Nothing rejected is normalized or persisted. Rejection
  stays fail-open and silent — the client is never blocked, and **neither** the
  observation stream nor the route stream is written.
- Every accepted Codex or Claude Hook delivery is normalized and persisted in
  one atomic transaction, without client-specific storage or transaction
  behavior. Common fields are always present; event-only fields are nullable or
  `n/a`. No observation carries a credential value, endpoint, settings path,
  prompt, response, or transcript content.
- Cardinality is stated over successful commits, not attempts: a delivery whose
  transaction commits leaves exactly one observation for its `delivery_id` plus
  zero or one route row; a delivery whose transaction fails leaves zero rows in
  both streams and is dropped fail-open, with no pending marker, cross-process
  retry, or recovery protocol. An internal retry reusing the same `delivery_id`
  is a whole-operation no-op: it adds neither a second observation nor a second
  route. The invariant is `0 <= observations(delivery_id) <= 1`.
- The shared classifier records one `route_effect`: `advance`, `retain`,
  `unknown`, or `none`. Only `advance` or the pre-existing mismatch `unknown`
  effect may write an effective-route row, and both write through the unchanged
  consecutive-identical no-op rule: `advance` guarantees the session resolves to
  the advanced selection, not that a new row was appended. An observation never
  changes event pricing by itself.
- A matched `no key -> first key` transition records the new custom-provider
  `ConfigChange` route. A matched `key A -> key B` replacement and a
  key-removal switch record no route, because the running session keeps
  the prior credential. A settings mismatch retains its existing explicit
  unknown handling; this topic does not turn an unrecognized external edit into
  a known provider.
- Events in a session that spans such a switch resolve by the hierarchy above: to
  the session's prior route when it has one, and otherwise to the provider
  timeline at session start. Neither path names the newly selected provider.
- After that session restarts, its events resolve to the new selection, because
  `SessionStart` writes a fresh route from it. `startup`, `resume`, and `clear` all
  reach that path (`internal/usagehook/event.go:88`), so suppressing the
  `ConfigChange` write does not make the old route permanent.
- A `ConfigChange` route after a custom-provider switch is accepted only for the
  observed `no key -> first key` transition, not for key rotation.
- A session that started after the switch is attributed normally in both
  directions: `SessionStart` reads the completed selection
  (`internal/usage/routes.go:22-40`), which is unaffected by this defect.
- `usage summary` and session cost output price those events at the multiplier of
  the provider the session is actually billing at, not the newly selected one. No
  new unattributed or unknown bucket appears, and no existing one grows: nothing
  became unattributable, and the fix removes a write rather than adding a state.
- The attribution architecture no longer derives `exact` for every Claude
  `ConfigChange` route from immediate activation. It names the sole first-key
  exception and preserves session-start fallback for all unadopted changes.
- **Verified on a real Claude session across first-key addition, key rotation,
  key removal, and restart.** The defect was
  found in real use and its mechanism is a claim about a running process, so
  automated tests over the settings file cannot confirm the fix. The manual
  procedure and what it must show belong to `tasks.md`; a passing unit test is
  not acceptance evidence for this topic's central claim.
