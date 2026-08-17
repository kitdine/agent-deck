---
status: active
created: 2026-08-17
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
provider a session is actually using. Both claims are true in one direction
only.

`WriteClaudeConfig` writes the same pair of fields in both directions, differing
only in whether it sets or deletes them
(`internal/provider/config.go:654-663`):

| Direction | `ANTHROPIC_BASE_URL` | `ANTHROPIC_AUTH_TOKEN` | Reaches a running session |
| --- | --- | --- | --- |
| Subscription -> API key (a custom provider) | written | written | Yes |
| API key -> `official`, direct | deleted | deleted | **No** |
| API key -> `official --via` wrapper | written | deleted | **Not for billing** |

Writing a credential into the environment a running client reads is picked up.
Deleting one is not: the session authenticated with that token at startup and
negotiated its capabilities then, and removing the key from a file does not
return the process to subscription authentication. The user reported this from
real use on 2026-08-17, and the mechanism above is the reading that explains it.

The third row is a mixed state and is called out separately because it is the one
a single "direction" reading gets wrong. A `--via official` selection writes an
endpoint and deletes the credential (`ConfigMatchesOfficialWrapper`,
`internal/provider/config.go:109-141`, checks exactly that shape), so the running
session may reach a new endpoint while still presenting the old token. The
endpoint change is observable and the credential change is not. Since it is the
credential that determines what is billed and at whose rate, this row is treated
as not reaching the session, the same as the direct row.

**The discriminant is therefore whether the switch deletes the credential**, not
which provider was selected. Keying on the provider name would put the wrapper row
on the wrong side.

Two things in the product assert otherwise.

**The advisory is unconditional.** `claudeRestartAdvisory`
(`internal/provider/service.go:808`) states that a running client reads its
settings live, so the switch can reach a session mid-conversation. Every Claude
switch prints it, including the direction where it is false — where the switch
reaches nothing until the session restarts. `docs/specs/cli-manual.md:283-285`
documents the same claim in the same unconditional form.

**Attribution treats a file as evidence of a process.**
`ClaudeConfigMatchesSnapshot` (`internal/provider/config.go:145-153`) compares
only the fields on disk. Switching back to subscription deletes both keys, so
`ConfigMatchesOfficialClaude` returns a match, `RecordClaudeConfigChange`
records `matched = true`
(`cmd/agentdeck/main.go:2947`, `internal/usage/routes.go:45-54`), and the route
carries `official` with its multiplier. The running session is still billing
against the API key. Every subsequent event in that session is priced at the
subscription rate while the money is spent at the other one.

The information needed to price them correctly was already there. The route
recorded when the session switched *to* the API key names the provider it is
still using; the new route overwrites that answer with a provider the session
never adopted. Nothing became unknowable — a correct attribution was replaced by
an incorrect one.

Nothing on disk can detect the discrepancy. The credential lived in
`ANTHROPIC_AUTH_TOKEN`, which AgentDeck owns and has just deleted;
`ClaudeCredentialConflicts` (`:173-191`) detects only `env.ANTHROPIC_API_KEY`
and `apiKeyHelper`, the two sources AgentDeck does not own. There is no residue
to find — which is why the fix derives the provider from the recorded selection
rather than trying to observe it.

The reverse direction is unaffected: the switch takes effect, and disk and
process agree.

Codex is out of scope because it activates no configuration without a restart,
so its advisory — that AgentDeck cannot update configuration already loaded by a
running client (`internal/provider/service.go:807`) — is directionless and
correct as written.

## Goals

- **A switch's effectiveness claim is direction-aware.** A Claude switch states
  what actually happens to a running session for the direction it just
  performed, and never asserts live activation for a direction that does not
  have it.
- **Disk state is not evidence of process state.** Reconciling the managed
  settings file proves what the file says. A switch that the running client
  cannot have adopted does not become that session's route merely because the
  file now describes it.
- **A session keeps the attribution it authenticated under.** The provider a
  running session bills at, after a switch that cannot reach it, is the one it
  started with — which AgentDeck already recorded, as that session's own route or
  in the provider timeline. Attribution continues to resolve to it until the
  session restarts; the hierarchy is stated below. It is not unknown, and
  reporting it as unknown would discard a correct answer the store already holds.
- **The invalid premise is corrected wherever it was written down.**
  `usage-attribution-precision`'s architecture derives `exact` for Claude
  `ConfigChange` routes from "Claude activates immediately"
  (`docs/topics/usage-attribution-precision/architecture.md:53-55`). That
  document has never been reviewed, so the correction is made now, before its
  review or any work derives from it.

## What a credential-deleting switch attributes to

A switch the running session cannot have adopted leaves that session's routing
unchanged, so the requirement is that attribution keeps resolving to what was
already in effect. That is achieved by **recording no route**, and it needs no new
stored representation and no new read rule: the existing resolution order already
produces the right answer once the misleading row is not written.

`sessionRouteAt` returns the most recent route at or before an event's time
(`internal/usage/usage.go:2504-2510`), and `priceForEvent` falls back to the
provider timeline at session start when a session has no route at all
(`:2622-2634`). With nothing written, an event resolves in this order:

| Evidence available | Resolves to | Quality |
| --- | --- | --- |
| A prior route for this session — from `SessionStart`, or from the `ConfigChange` of the earlier switch *to* the API key | that route's provider and multiplier | as that route was stored, `estimated` |
| No route for this session, but the provider timeline covers its start | the selection in effect at session start | `estimated` |
| No route and no timeline coverage | `unknown`, multiplier `1` | today's `historical` fallback |

Rows two and three are pre-existing behavior reached today by any session without
a route; this topic neither creates nor changes them. Row one is the case the
defect corrupts, and it is the only case the fix touches.

Row two matters for the "no prior route" gap being real rather than theoretical:
`RecordSessionRoute` writes nothing when no completed selection exists
(`internal/usage/routes.go:33-34`) and skips `compact`
(`:29`). A session can therefore reach a credential-deleting switch with no route
of its own — and the timeline fallback then resolves to the selection at its
start, which is the same answer "keep what you authenticated under" asks for.

**Consequences for cost output, which is what makes this requirement satisfiable
in `v0.5.0`:**

- Events in a session spanning a credential-deleting switch are priced at the
  custom provider's multiplier, because that is the route they resolve to.
- No new quality value is introduced, no `usage_session_routes` column changes,
  and no migration is required.
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
- **Making the switch take effect in the unsupported direction.** Terminating or
  signalling a running Claude process to force re-authentication is not in
  scope: AgentDeck does not manage client processes, and doing so would end a
  user's conversation.
- **Recomputing historical attribution.** Events already recorded under the
  wrong multiplier stay as they are. Quality-value redesign and backfill belong
  to [`usage-attribution-precision`](../usage-attribution-precision/tasks.md) in
  `v0.6.0`; this topic stops the defect from producing new wrong data and
  corrects that topic's invalid premise.
- **Codex behavior.** Unchanged, per the scope note above.
- **Observing which credential a running process holds.** No supported mechanism
  exists. The design does not need one: it derives the answer from the previous
  selection instead of observing the process.

## User-visible surfaces

This topic adds no new surface. It changes the text of one existing stderr
advisory and suppresses one stored route, both of which
`architecture.md` owns as contract changes. There is therefore no
`ux/<surface>.md`, and `tasks.md` states that row as a decision rather than
omitting it.

## Contracts in scope

Yes. Three, all specified in [`architecture.md`](architecture.md):

- the Claude switch advisory text, which is a documented stderr contract in
  `docs/specs/cli-manual.md` and `docs/specs/cli-design.md`;
- what `ClaudeConfigMatchesSnapshot` proves, and what a caller may conclude from
  it;
- whether a `ConfigChange` route is recorded at all for a switch the running
  client cannot have adopted.

## Acceptance boundary

- A Claude switch **to** a custom provider prints an advisory that states the
  switch can reach a running session mid-conversation and may reset its
  negotiated capabilities.
- A Claude switch that **deletes the credential** — `official` direct and
  `official --via` alike — prints an advisory that states a running session keeps
  the credential it authenticated with and must be restarted for the switch to
  apply to it.
- Neither advisory prints a credential value, and a switch that already
  succeeded is never failed by advisory generation.
- A credential-deleting switch records no `ConfigChange` route. This holds for
  both the direct and the `--via` wrapper selection, because both leave the
  running session presenting the old credential.
- Events in a session that spans such a switch resolve by the hierarchy above: to
  the session's prior route when it has one, and otherwise to the provider
  timeline at session start. Neither path names the newly selected provider.
- After that session restarts, its events resolve to the new selection, because
  `SessionStart` writes a fresh route from it. `startup`, `resume`, and `clear` all
  reach that path (`internal/usagehook/event.go:88`), so suppressing the
  `ConfigChange` write does not make the old route permanent.
- A `ConfigChange` route recorded after a switch to a custom provider continues
  to record that provider, because that direction is observable.
- A session that started after the switch is attributed normally in both
  directions: `SessionStart` reads the completed selection
  (`internal/usage/routes.go:22-40`), which is unaffected by this defect.
- `usage summary` and session cost output price those events at the multiplier of
  the provider the session is actually billing at, not the newly selected one. No
  new unattributed or unknown bucket appears, and no existing one grows: nothing
  became unattributable, and the fix removes a write rather than adding a state.
- The `v0.6.0` attribution architecture no longer derives `exact` for Claude
  `ConfigChange` routes from immediate activation, and states the direction
  dependency instead.
- **Verified on a real Claude session, in both directions.** The defect was
  found in real use and its mechanism is a claim about a running process, so
  automated tests over the settings file cannot confirm the fix. The manual
  procedure and what it must show belong to `tasks.md`; a passing unit test is
  not acceptance evidence for this topic's central claim.
