---
status: active
created: 2026-08-13
updated: 2026-08-16
---

# Usage Attribution Precision — Architecture

Specifies the attribution resolution order, the per-client time semantics, the
unattributed boundary, and the resulting contract change. The measured baseline
and the decisions this design implements are in
[`requirements.md`](requirements.md).

## Current resolution order

`readPriceResolver.priceForEvent` (`internal/usage/usage.go`) resolves in four
steps, and only the first yields `exact`:

```text
1. usage_runs.exact = 1                  -> exact       (agentdeck run only; observed 0)
2. sessionRouteAt(client, sid, eventAt)  -> estimated   (quality hardcoded in recordSessionRoute)
3. timeline.SnapshotAt(client, sessionStartAt(event))
                                         -> estimated   (hardcoded)
4. fallthrough                           -> historical, multiplier "1", provider "unknown"
```

Step 2 already matches by event time. Step 3 applies `sessionStartAt` to both
clients, which is correct for Codex and wrong for Claude.

## Target resolution order

```text
1. usage_runs.exact = 1                          -> exact
2. route sequence for the session, positioned by client semantics
     codex : the SessionStart route in effect at session start
     claude: the latest route at or before the event time
                                                 -> exact when the positioned
                                                    route is unambiguous
                                                 -> estimated when ambiguous
3. timeline.SnapshotAt positioned by client semantics
     codex : session start time
     claude: event time
                                                 -> estimated
4. no timeline coverage at the positioned time   -> unattributed
```

Step 2 becomes `exact` because a route is an observation of the provider that
was actually in effect, not a guess:

- **Codex** activates configuration only on restart, and `RecordSessionRoute`
  writes on `SessionStart`. The recorded snapshot is therefore the exact
  configuration the process loaded and will keep for its lifetime.
- **Claude** activates immediately, and mid-session changes are separately
  recorded as `ConfigChange` routes. Positioning by event time inside that
  route sequence yields the provider actually in effect.

A route is **ambiguous**, and stays `estimated`, when the positioned route
records `provider = unknown` — which `RecordClaudeConfigChange` writes
deliberately when the managed settings file did not match a completed
selection.

## Unattributed boundary

Step 4 currently conflates two different states. They must be separated,
because only the first is genuinely unknowable:

| State | Meaning | Reportable |
| --- | --- | --- |
| Before adoption | Event predates any provider selection AgentDeck recorded | Yes, as an explicitly bounded bucket |
| Coverage gap | Timeline exists but has no entry at the positioned time | Yes, and it is a defect signal worth surfacing |

Neither may contribute to a real-spend total, and neither may silently use
multiplier `1` as if it were a known rate.

## Contract impact

`usage summary` exposes attribution counts in its JSON `counts` object and
emits `estimated attribution` / `historical attribution` warnings. Renaming
`historical` to `unattributed` and changing what `exact` counts are both
observable contract changes. The CLI manual, the CLI design contract, and the
release notes must be reconciled in the same task that changes the values.
