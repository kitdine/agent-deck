---
status: active
created: 2026-08-13
updated: 2026-08-26
---

# Usage Attribution Precision — Architecture

Specifies the attribution resolution order, the per-client time semantics, the
unattributed boundary, and the resulting contract change. The measured baseline
and the decisions this design implements are in
[`requirements.md`](requirements.md).

## Current resolution order

Two functions in `internal/usage/usage.go` independently implement the same
four-step policy: `readPriceResolver.priceForEvent`, used by bounded range and
desktop presentation reads, and `Service.priceForEvent`, used by session and
legacy summary paths. Only their first step yields `exact`:

```text
1. usage_runs.exact = 1                  -> exact       (agentdeck run only; observed 0)
2. sessionRouteAt(client, sid, eventAt)  -> estimated   (persisted route quality)
3. timeline.SnapshotAt(client, sessionStartAt(event))
                                         -> estimated   (hardcoded)
4. fallthrough                           -> historical, multiplier "1", provider "unknown"
```

`usage_session_routes.quality` is `TEXT NOT NULL`, and
`recordSessionRouteConn` in `internal/usage/routes.go` stores `estimated` for
every accepted route. Both resolvers currently pass that stored value through.
Step 2 already matches session routes by event time. Step 3 applies
`sessionStartAt` to both clients, which is retained: a provider-timeline change
is global file/selection state, not proof that a running Claude session adopted
it. Claude's valid mid-session movement is represented only by an effective
`ConfigChange` route in step 2.

`usage_session_observations` is deliberately absent from both the current and
target resolution orders. The upstream switch topic now persists every accepted
Codex or Claude Hook delivery through one client-neutral operation, but an
observation records what AgentDeck saw and concluded — not what the process is
billing. This resolver consumes only the resulting effective-route stream.

## Target resolution policy

```text
1. usage_runs.exact = 1                          -> exact
2. effective route sequence for the session
     codex : the SessionStart route loaded by the process
     claude: the latest adopted route at or before the event time
                                                 -> exact / effective_route when
                                                    provider is known
                                                 -> estimated / ambiguous_route
                                                    when provider is unknown
3. timeline.SnapshotAt(session start time) for either client
                                                 -> estimated / timeline_snapshot
4. no timeline coverage at the positioned time
     no timeline entry exists for this client    -> unattributed / before_adoption
     timeline exists but has no positioned entry -> unattributed / coverage_gap
```

`exact_run` is the reason emitted by step 1. Together with the five reasons in
the table, the resolver therefore returns one closed reason vocabulary:
`exact_run`, `effective_route`, `ambiguous_route`, `timeline_snapshot`,
`before_adoption`, or `coverage_gap`.

Step 2 becomes `exact` because a known-provider route is an observation of the
provider and multiplier actually in effect, not a guess:

- **Codex** activates configuration only on restart. Its accepted Hook deliveries
  use the same observation transaction as Claude, while non-compact
  `SessionStart` produces `route_effect=advance`. The resulting effective route
  is the exact configuration the process loaded and will keep for its lifetime.
- **Claude** may adopt one live transition: a session that started without an
  API key can take its first key. That effective transition records a
  `ConfigChange` route. An already-keyed session does not adopt key rotation or
  key removal; those matched file changes record no route, so the prior route
  remains effective until restart. This state machine is owned by
  [`switch-effectiveness-boundary`](../switch-effectiveness-boundary/architecture.md).

The difference above is runtime evidence used by one shared policy, not separate
client storage, transaction, privacy, or resolver logic.

The provider timeline remains a session-start fallback for Claude as well as
Codex. Using its event-time selection would reintroduce the defect: the global
selection changes when the file changes even when the running session does not
adopt it. Event time applies only inside the session's effective route sequence.

A route is **ambiguous**, and stays `estimated`, when the positioned route
records `provider = unknown`. That behavior now lives in `classifyConfigChange`
inside `RecordHookDelivery` (`internal/usage/routes.go`): a managed-file mismatch
sets the route provider to `unknown`, the multiplier to `1`, and the effect to
`unknown` before `recordSessionRouteConn` persists the route.

### Read-time quality derivation

Both `readPriceResolver.priceForEvent` and `Service.priceForEvent` must call the
same policy helper (or prove equivalent results through one shared table of
cases). Step 2 derives quality from the positioned route: known provider is
`exact`; `provider = unknown` is `estimated`. It does not read the persisted
`usage_session_routes.quality` value as the verdict. The column and
`recordSessionRouteConn` remain unchanged for compatibility, existing rows need
no backfill, and `internal/usage/routes.go` is not a target file for this topic.

The returned attribution also carries its reason and a `spend_eligible` decision.
Quality and spend eligibility are independent: an `ambiguous_route` remains
`estimated`/`inferred`, but `provider = unknown` makes it ineligible for a
real-spend provider total. `before_adoption` and `coverage_gap` are likewise
ineligible. A known-provider effective route or timeline snapshot is eligible;
step 1 treats a missing provider or multiplier as invalid exact-run data rather
than silently applying `1`.

## Unattributed boundary

Step 4 currently conflates two different states. After `SnapshotAt` returns
`sql.ErrNoRows`, the resolver distinguishes them with a client-wide timeline
existence check; it does not infer the distinction from the event timestamp:

| State | Meaning | Reportable |
| --- | --- | --- |
| Before adoption | The client has no completed provider operation or usable provider selection anywhere in its timeline | Yes, reason key `before_adoption` |
| Coverage gap | The client has at least one timeline entry, but none can supply a snapshot at the session-start position | Yes, reason key `coverage_gap`; also a defect signal |

Neither may contribute to a real-spend total, and neither may silently use
multiplier `1` as if it were a known rate.

## Contract impact

`usage summary` keeps one quality count per event. Its JSON `counts` object
replaces `historical` with `unattributed` and otherwise retains `exact` and
`estimated`; warnings become `estimated attribution` and `unattributed
attribution`. It also gains an `attribution_reasons` object with all six reason
keys initialized to zero, so `before_adoption` and `coverage_gap` are separately
observable without inventing two more quality values. Text output prints the
same non-zero reason counts under an `ATTRIBUTION REASONS` section.

Provider-cost fields are real-spend fields. `known_provider_cost` sums only
priced events whose attribution is `spend_eligible`; `provider_cost` remains
available only when every event is both fully priced and spend-eligible.
`unattributed_catalog_base_cost` separately reports the calculable catalog-base
amount for `before_adoption` and `coverage_gap` events; it is `null` when any
such event has an unpriced component. The reason counts preserve the split of
that amount. An `ambiguous_route` is excluded from provider cost for the same
unknown-provider reason, remains counted under `estimated`, and is disclosed by
its own reason count rather than relabelled `unattributed`.

The existing desktop `Presentation` contract is another consumer. Its tier
mapping becomes quality-driven: `exact -> determinable`, `estimated -> inferred`,
and `unattributed -> unattributed`. Provider identity does not override that
mapping. Any tier containing a non-spend-eligible event reports incomplete cost,
and desktop provider totals exclude that event from real spend. Producer tests
and the checked-in `desktop/fixtures/v1/*.json` payloads must be regenerated with
`AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`, never edited by hand.

These JSON/text changes, the CLI manual, the CLI design contract, canonical
desktop fixtures, and the release-note input must be reconciled in the same
task that introduces observability.
