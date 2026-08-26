---
status: active
created: 2026-08-17
updated: 2026-08-25
---

# Switch Effectiveness Boundary — Tasks

This file is the only status authority for this topic.

## Task breakdown

### 1. `switch-effectiveness-contract`

Make the Claude switch advisory direction-aware, and state what the settings-file
reconcile proves.

- Contract: [`architecture.md`](architecture.md) contracts 1 and 2.
- Replace `claudeRestartAdvisory` with the two texts the contract names, selected
  by whether the reported selection deletes the credential. Thread that fact into
  `SwitchAdvisories`; the signature change is internal and adds no flag to any
  command.
- Leave the `official` conflict advisories and `codexRestartAdvisory` unchanged,
  and keep the conflict notes ordered before the restart note.
- Record the boundary on `ClaudeConfigMatchesSnapshot` and at
  `reconcileClaudeConfigChange`: a match is a fact about the file, not about a
  running client.
- Update the stderr advisory contract in both living specifications, which
  currently document the live-activation claim unconditionally
  (`docs/specs/cli-manual.md:283-285`).
- Files: `internal/provider/service.go`, `internal/provider/config.go`,
  `cmd/agentdeck/main.go`, `cmd/agentdeck/provider_switch_advisories_test.go`,
  `internal/provider/switch_advisories_test.go`, `docs/specs/cli-manual.md`,
  `docs/specs/cli-design.md`.
- Verification level: L2 — the advisory text is a documented stderr contract.

### 2. `unadopted-switch-no-route`

Stop overwriting a session's correct route with a switch that session cannot have
adopted.

- Contract: [`architecture.md`](architecture.md) contract 3.
- A `ConfigChange` reconcile for a credential-deleting selection records no route.
  A credential-writing selection keeps today's behavior.
- Cover that events after the suppressed write still resolve to the prior route
  and its multiplier, which is the whole point: the assertion is about what
  `sessionRouteAt` returns, not only about the absence of a row.
- Cover that a session started *after* the switch is attributed from
  `SessionStart`, and that a restart of the spanning session resolves to the new
  selection. Suppressing the write must not make the old route permanent — the
  regression this task could plausibly introduce is exactly that.
- Cover the cost consequence: an event in a session that spans a
  credential-deleting switch is priced at the custom provider's multiplier, not
  the subscription one.
- Update the direct reconcile regressions in
  `cmd/agentdeck/claude_reload_test.go`: a credential-writing selection still
  records its matched route, while a credential-deleting selection records no
  route. Remove the current expectation that the latter writes
  `unknown`/multiplier `1`.
- Depends on task 1, which establishes the direction discriminant this task
  reuses.
- Files: `internal/usage/routes.go`, `cmd/agentdeck/main.go`,
  `cmd/agentdeck/claude_reload_test.go`, `internal/usage/routes_test.go`,
  `internal/usage/usage_test.go`.
- Verification level: L2 — it changes a persisted value that cost output reads.

### 3. `real-session-acceptance`

Confirm on a real Claude session, in both directions, that the shipped behavior
matches the contract. This is the task the topic exists for: its central claim is
about a process that authenticated before the file changed, and no test over
`settings.json` can observe that.

- Depends on tasks 1 and 2.
- Acceptance setup:
  - Use a dedicated custom-provider test credential whose provider-side audit
    log exposes a non-secret credential label or key ID and request ID. The
    custom multiplier must be distinct from `official` (for example, `2`). Do
    not use a provider that cannot supply this discriminator.
  - Record the exact provider-audit command or dashboard query, its UTC time
    bounds, and only the request ID plus non-secret credential label/key ID in
    the review record. Never record an authorization header, credential value,
    prompt, or response. A successful response, model name, or `/status` alone
    does not prove which credential served the turn.
  - Set `AGENTDECK_BIN` to the locally built binary under test, `SESSION_ID` to
    the Claude session ID, and `STATE_DB` to the core database (normally
    `$HOME/.agentdeck/agentdeck.sqlite3`). After each marked turn, run
    `"$AGENTDECK_BIN" usage scan` and `"$AGENTDECK_BIN" session scan` before
    inspecting evidence.
- Procedure, run against a locally built binary with a real Claude session:
  1. Start a Claude session on `official` (subscription). Record its session ID.
  2. Switch that client to a custom API-key provider. Confirm the advisory states
     the switch can reach the session mid-conversation.
  3. Mark a UTC interval around one turn in the **already running** session. The
     provider audit must contain a request in that interval under the dedicated
     custom credential. Run the evidence commands below and confirm the new
     `ConfigChange` route, the latest event's effective route, and its cost all
     use the custom provider and multiplier.
  4. Switch back to `official`. Confirm the advisory states the running session
     keeps its credential until restarted. Record the current
     `ConfigChange`-route count before sending another turn.
  5. Mark a second UTC interval around a turn in the same still-running session.
     The provider audit must again name the dedicated custom credential. After
     both scans, confirm the `ConfigChange`-route count did not increase, there
     is no route for the credential-deleting switch, and the latest event still
     resolves to the prior custom provider and multiplier. Its invocation cost
     must use that custom multiplier, not the subscription multiplier.
  6. Restart the session, mark a third UTC interval around a turn, and run both
     scans. Confirm the custom provider audit contains no request in that
     interval, the new `SessionStart` route names `official`, and the latest
     event resolves and prices through that route.
- Evidence commands, run after setting the three variables named above. Preserve
  their redacted output for steps 3, 5, and 6; preserve the route-count output
  immediately before step 5 as well:

  ```bash
  "$AGENTDECK_BIN" usage scan
  "$AGENTDECK_BIN" session scan

  sqlite3 -readonly "$STATE_DB" -header -column \
    "SELECT id,observed_at,provider,multiplier,hook_event,source,quality
       FROM usage_session_routes
      WHERE client='claude' AND session_id='$SESSION_ID'
      ORDER BY observed_at,id;"

  sqlite3 -readonly "$STATE_DB" -header -column \
    "SELECT COUNT(*) AS config_change_routes
       FROM usage_session_routes
      WHERE client='claude' AND session_id='$SESSION_ID'
        AND hook_event='ConfigChange';"

  sqlite3 -readonly "$STATE_DB" -header -column \
    "WITH latest_event AS (
       SELECT event_at,event_key
         FROM usage_events
        WHERE client='claude' AND session_id='$SESSION_ID'
        ORDER BY event_at DESC,event_key DESC
        LIMIT 1
     )
     SELECT e.event_at,r.observed_at AS route_observed_at,r.provider,
            r.multiplier,r.hook_event,r.source,r.quality
       FROM latest_event AS e
       LEFT JOIN usage_session_routes AS r
         ON r.id=(
           SELECT id
             FROM usage_session_routes
            WHERE client='claude' AND session_id='$SESSION_ID'
              AND observed_at<=e.event_at
            ORDER BY observed_at DESC,id DESC
            LIMIT 1
         );"

  "$AGENTDECK_BIN" --format json session show "$SESSION_ID" \
    --client claude --tokens --all \
    | jq '.data | {usage, latest_invocation: .invocations[-1]}'
  ```
- Step 3 is the control: if it fails, the asymmetry this topic assumes is not the
  real mechanism, and the design must be reopened rather than the test adjusted.
- Record the observed evidence in this task's review record. Real credentials are
  used by the operator and never committed; the record names providers,
  directions, route values, and attribution, never a credential value.
- Files: this topic's review record only. No product change.
- Verification level: L3 — real credentials and real client behavior, which no
  automated suite in this repository covers.

### 4. `attribution-premise-correction`

Correct the invalid premise in the `v0.6.0` attribution architecture.

- `../usage-attribution-precision/architecture.md:53-55` derives `exact` for
  Claude `ConfigChange` routes from immediate activation. State the direction
  dependency and point at this topic's contract 3: a credential-deleting switch
  creates no resulting `ConfigChange` route, so the retained prior session route
  or existing session-start timeline fallback remains the evidence source.
- Leave any future quality redesign for those retained evidence sources to that
  topic. This task suppresses the misleading route and removes a false premise;
  it does not assign a quality to a nonexistent replacement route.
- That document is unreviewed with no review record, so this is a draft
  correction: no verdict is reopened and no completion evidence is invalidated.
- Independent of tasks 1-3 and may run in any order relative to them.
- Files: `docs/topics/usage-attribution-precision/architecture.md`.
- Verification level: L0 — documentation, in an unreviewed draft.

Commit boundaries follow task boundaries. This topic does not authorize commits,
pushes, release publication, or installation.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [ ] |
| tasks.md | [x] | [ ] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight. This topic adds no user-visible surface: it changes the text of one
existing stderr advisory and suppresses one misleading stored route. Both are
contracts `architecture.md` owns, and neither has a state set or copy that a
surface document would review.

The review records under [`reviews/`](reviews/) preserve the exact reviewed
blobs. Additional real-session evidence on 2026-08-25 proved that an already-keyed
Claude session also ignores key rotation; only `no key -> first key` may apply
live. The current requirements now pass independent review. Architecture and
decomposition remain reopened on the corrected state machine, so review proceeds
architecture -> tasks before development.

## Tasks

| # | Task | Dev | Review |
| --- | --- | --- | --- |
| 1 | `switch-effectiveness-contract` | [ ] | [ ] |
| 2 | `unadopted-switch-no-route` | [ ] | [ ] |
| 3 | `real-session-acceptance` | [ ] | [ ] |
| 4 | `attribution-premise-correction` | [ ] | [ ] |

Development is blocked until this `tasks.md` passes review, per
`.agent-instructions/beads.md`: a draft task matrix is not a source of task
anchors.

`tasks.md` Review Round 1 (2026-08-17): **FAIL** on T-F1–T-F5. The manual
acceptance still expects an unknown route, the status matrix retains the old
task-2 anchor, quality wording still reflects the superseded design, the direct
reconcile regression test is absent from task 2, and the real-session proof has
no reproducible discriminator or inspection commands. Its `Review` cell remains
unchecked; see [`reviews/tasks.md`](reviews/tasks.md).

`tasks.md` Re-review Round 2 (2026-08-17): **PASS**. T-F1–T-F5 are closed
against blob `719ff3988ebe14699fc39a8f891f5b4ee253866b`; the required document
checker and L0 checks pass, and the completion-evidence/v1 document gate is
`VERIFIED`. The design document set is closed; implementation begins with
`switch-effectiveness-contract`.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
开发：`switch-effectiveness-boundary` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, every file it names, and
verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`reviews/<task-anchor>.md` before ticking `Review`.
