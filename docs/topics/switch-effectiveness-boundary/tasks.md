---
status: active
created: 2026-08-17
updated: 2026-08-26
---

# Switch Effectiveness Boundary — Tasks

This file is the only status authority for this topic.

## Task breakdown

### 1. `hook-delivery-ledger`

Create the one accepted-Hook operation shared by Codex and Claude, as a storage
boundary that does not change any route this build already writes.

- Contract: [`architecture.md`](architecture.md) Contract 0 (admission, the
  `route_effect` policy, and the meaning of `advance`) and Stream 1.
- Keep client-specific code at the raw adapter boundary only. Normalize into one
  `HookDelivery` input and call shared `usage.Service.RecordHookDelivery`
  **only after the full ordered admission sequence passes**: bounded read,
  `usagehook.ParseEvent`, store/home availability, `managedClaudeConfigChange`
  for a Claude `ConfigChange`, and `validHookTranscript` for a `SessionStart`
  from either client. A rejected delivery stays fail-open and silent and writes
  neither stream.
- Add `usage_session_observations`, its
  `(client, session_id, observed_at)` lookup index, and a unique `delivery_id`
  index as migration 19, using the exact DDL, column set, and encodings in
  Stream 1. Existing tables and columns do not change.
- Persist one privacy-bounded observation per committed delivery. Store common
  fields for every event and nullable event-only fields — NULL means "not
  applicable", never `''` or `0`; use one `route_effect` enum: `advance`,
  `retain`, `unknown`, or `none`.
- **Route behavior this task ships is exactly today's behavior, re-expressed
  through `route_effect`.** Task 1 emits only `advance`, `unknown`, and `none`,
  derived from facts the current handler already has: non-compact `SessionStart`
  with a completed selection is `advance`; compact starts, `SessionEnd`, and any
  other accepted non-boundary event are `none`; a matched managed `ConfigChange`
  is `advance`; a mismatch is `unknown`. Task 1 introduces no prior-state
  classifier and never emits `retain`, so it neither contains part of Task 3 nor
  regresses `ConfigChange` routing: after Task 1, every route row this build
  writes today is still written, with the same values. This mapping is the body
  of the ordered transaction's classify step; Task 3 replaces that body with the
  prior-state classifier that turns rotation/removal/indeterminate changes into
  `retain`, which is the only route-behavior change in this topic. The step's
  position, the duplicate guard around it, and the observation/route write order
  are Task 1's and do not move.
- `advance` and `unknown` write through the unchanged consecutive-identical
  no-op rule in `recordSessionRoute`; they guarantee the session's resolved
  route, not a new row.
- Generate one opaque `delivery_id` per accepted delivery, reuse it only across
  an internal retry of that store operation, and accept a new row for every later
  transport delivery. Mtime is diagnostic only and never suppresses insertion.
- Apply one privacy allowlist to every client: never store credential values,
  endpoints, settings paths, prompts, responses, or transcript content.
- Perform the whole operation in one atomic transaction, in the executable order
  Stream 1 fixes — Task 1 **owns this ordered skeleton** and Task 3 later fills
  one step of it:
  1. Take the per-attempt settings snapshot for a config event, before `BEGIN`.
  2. `BEGIN`, then the duplicate-delivery guard: if `delivery_id` already exists,
     the whole operation is a no-op — classify nothing, insert nothing, skip the
     route write, commit, return success.
  3. Classify on this same transaction and compute `route_effect` plus the
     observation's classifier fields. In Task 1 this step is the fixed
     today's-behavior mapping below; Task 3 replaces its body with the
     prior-state classifier without moving the step.
  4. Insert the observation carrying that computed `route_effect`, still
     conditionally on `delivery_id` not existing, so the guard has a
     storage-level backstop rather than a check-then-act race.
  5. If that insert affects zero rows, take the same no-op outcome as step 2:
     skip the route write, commit, return success.
  6. Only on a one-row insert, apply `route_effect` and perform the zero-or-one
     route write.

  Classification precedes the observation insert because `route_effect` is
  `NOT NULL` and carries the classifier's result. All applicable rows commit or
  none does; there is no client-specific transaction or partial-state recovery
  protocol.
- Cover every currently accepted event: Codex `SessionStart`; Claude
  `SessionStart`, `ConfigChange`, and `SessionEnd`; all supported sources,
  compact/no-route behavior, internal retry, external replay, and future-adapter
  extensibility.
- Assertions this task owns:
  - **Admission regressions** in `cmd/agentdeck/hook_boundary_test.go`, extending
    its existing rejection cases: an out-of-root transcript, a transcript whose
    base name omits the session ID, a non-regular/symlink transcript, a
    `ConfigChange` on `project_settings`/`local_settings`/`policy_settings`/
    `skills`, and a `ConfigChange` on an unmanaged path each leave **zero**
    observation rows and zero route rows, and still exit fail-open.
  - **Cardinality**: a committed delivery leaves exactly one observation for its
    `delivery_id`; a delivery whose transaction fails leaves zero observations
    and zero routes; the invariant is `0 <= observations(delivery_id) <= 1`.
  - **Whole-operation retry**: re-running the store operation with the same
    `delivery_id` leaves one observation **and** the same route row count as the
    first commit — assert both streams, not the observation alone.
  - **Route non-regression**: for every event kind above, the route rows written
    after this task are identical to the rows the pre-task build writes for the
    same input, including the consecutive-identical no-op.
  - **Migration**: a fresh database reaches schema version 19 with the table, the
    lookup index, and the unique `delivery_id` index present, and with exactly
    the columns and declared types in the Stream 1 DDL; a database populated at
    version 18 with existing `usage_session_routes` and `usage_events` rows
    upgrades to the same shape with those rows byte-identical and
    `usage_session_routes`' schema unchanged. Both cases live in
    `internal/store/store_test.go`, which already owns the fresh-database and
    upgrade migration assertions.
- Files: `internal/store/migrations.go`, `internal/usagehook/event.go`,
  `internal/usage/routes.go`, `cmd/agentdeck/main.go`,
  `internal/store/store_test.go`, `internal/store/routes_test.go`,
  `internal/usage/routes_test.go`, `cmd/agentdeck/hook_boundary_test.go`, and
  `cmd/agentdeck/main_test.go`.
- Verification level: L2 — shared persisted Hook evidence and transaction
  behavior across both clients.

### 2. `switch-effectiveness-contract`

Make the Claude switch advisory state-aware and conservative, and state what the
settings-file reconcile proves.

- Contract: [`architecture.md`](architecture.md) contracts 1 and 2.
- Replace `claudeRestartAdvisory` with the two texts the contract names, selected
  by whether the reported selection contains a credential. The credential-present
  text must state both possible running-session states: only `no key -> first
  key` may apply live, while an already-keyed session keeps its current key.
  Thread the new selection shape into `SwitchAdvisories`; the signature change
  is internal and adds no flag to any command.
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

### 3. `effective-route-policy`

Stop overwriting a session's correct route with a switch that session cannot have
adopted.

- Contract: [`architecture.md`](architecture.md) Contract 3 and the shared
  `route_effect` policy from Contract 0.
- **This task owns the only route-behavior change in the topic.** Task 1 shipped
  the shared operation emitting `advance`/`unknown`/`none` at today's behavior;
  this task adds the prior-state classifier, introduces `retain`, and completes
  the shared policy. It changes `RecordHookDelivery`'s decision inputs, not its
  storage, transaction, or admission boundary, so the two tasks are separately
  reviewable and separately committable.
- Use one parsed settings snapshot per reconcile attempt: read the managed
  settings file once and derive both the match evaluation and the conflict scan
  from that same in-memory document, so a file change between two reads cannot
  pair a match from one state with a clean scan from another. A read or parse
  failure of that snapshot classifies `indeterminate` and writes no route.
- Replace the body of Task 1's classify step (step 3 of the ordered transaction)
  with the prior-state classifier. Read the prior route on that same
  transaction, so no decision commits from prior-route evidence read outside it,
  and produce `route_effect` plus `prior_state`, `conflict_scan`, and
  `conflict_sources` **before** the observation insert that carries them. This
  task moves no step: the duplicate-delivery guard stays ahead of
  classification, the observation insert stays ahead of the optional route
  write, and the zero-row no-op outcomes are unchanged. That is what keeps this
  task a decision change rather than a second transaction design.
- For a matched settings change, resolve the session's prior effective provider
  from its latest route or the provider timeline at session start, then confirm
  the candidate `no-key` against `ClaudeCredentialConflicts` on the settings path
  the reconcile already inspects. Record the new route only when that ordered
  classifier confirms `no key -> first key`.
- A matched `key A -> key B` replacement, credential deletion, or any transition
  whose prior state does not confirm the first-key exception records no route and
  retains the existing effective provider and multiplier. A settings mismatch
  keeps today's explicit `unknown` behavior.
- Implement one policy over normalized event facts: non-compact `SessionStart`
  with a completed selection is `advance`; compact/terminal/non-boundary events
  are `none`; confirmed first-key `ConfigChange` is `advance`; matched
  rotation/removal/indeterminate change is `retain`; managed mismatch is
  `unknown`.
- Keep every route decision inside shared `RecordHookDelivery`; raw adapters and
  `cmd/agentdeck` do not write routes or select storage behavior.
- Assert the classifier's own snapshot rule: a settings file whose conflict
  sources appear only after the match read still classifies `indeterminate`
  rather than promoting a false `no-key`, and the observation records
  `prior_state=indeterminate` with the matching `conflict_scan`.
- Leave `sessionRouteAt` and `priceForEvent` untouched and assert they never read
  the observation table: a session with observations but no new route must still
  price through its prior route.
- Cover that events after the suppressed write still resolve to the prior route
  and its multiplier, which is the whole point: the assertion is about what
  `sessionRouteAt` returns, not only about the absence of a row.
- Cover that a session started *after* the switch is attributed from
  `SessionStart`, and that a restart of the spanning session resolves to the new
  selection. Suppressing the write must not make the old route permanent — the
  regression this task could plausibly introduce is exactly that.
- Cover the cost consequence: an event in a session that spans key rotation or
  credential deletion is priced at the prior provider's multiplier, not the new
  selection's.
- Update the direct reconcile regressions in
  `cmd/agentdeck/claude_reload_test.go`: the first-key transition records its
  matched route; key rotation and credential deletion record none; a mismatch
  still records `unknown`/multiplier `1` until the attribution topic changes the
  unattributed boundary.
- Depends on tasks 1 and 2, which establish the shared operation and Claude
  effectiveness contract.
- Files: `internal/usage/routes.go`, `internal/provider/config.go`,
  `cmd/agentdeck/main.go`, `cmd/agentdeck/claude_reload_test.go`,
  `internal/provider/config_test.go`, `internal/usage/routes_test.go`, and
  `internal/usage/usage_test.go`.
- Verification level: L2 — it changes a persisted value that cost output reads.

### 4. `real-session-acceptance`

Confirm through real Codex and Claude lifecycle transport that the shared Hook
operation is client-neutral, then confirm Claude first-key addition, key
rotation, removal, and restart against provider-side evidence.

- Depends on tasks 1–3.
- Acceptance setup:
  - Use two dedicated custom-provider test credentials, A and B, whose
    provider-side audit log exposes distinct non-secret credential labels or key
    IDs and request IDs. Their providers or multipliers must also be
    distinguishable from each other and from `official`. Do not use a provider
    that cannot supply these discriminators.
  - Record the exact provider-audit command or dashboard query, its UTC time
    bounds, and only the request ID plus non-secret credential label/key ID in
    the review record. Never record an authorization header, credential value,
    prompt, or response. A successful response, model name, or `/status` alone
    does not prove which credential served the turn.
  - Set `AGENTDECK_BIN` to the locally built binary under test,
    `CODEX_SESSION_ID` and `CLAUDE_SESSION_ID` to the two real session IDs, and
    `STATE_DB` to the core database (normally
    `$HOME/.agentdeck/agentdeck.sqlite3`). After each marked turn, run
    `"$AGENTDECK_BIN" usage scan` and `"$AGENTDECK_BIN" session scan` before
    inspecting evidence.
- Procedure, using normal installed Hook transport — never manually feed a Hook
  payload to the binary:
  1. Start and resume a real Codex session. Confirm each accepted `SessionStart`
     delivery appends the common observation shape and each non-compact start
     leaves the session resolving to the current completed selection. Record the
     route row count before and after: a start whose selection differs from the
     preceding route appends one row; a repeated non-compact start on an
     unchanged selection appends **no** row while still appending its own
     observation with `route_effect=advance` — that is the preserved
     consecutive-identical no-op, not a defect. Trigger compact normally and
     confirm it appends an observation with `route_effect=none` and no route.
  2. Switch Codex provider mid-session without restarting. Confirm the running
     session retains its prior route; restart and confirm the next `SessionStart`
     observation and the session's resolved route use the current completed
     selection, and that the route row count increased by exactly one because the
     selection changed.
  3. Start a Claude session on `official` (subscription). Record its session ID.
  4. Switch that client to custom API-key credential A. Confirm the advisory says
     only a session that started without a key may adopt its first key live.
  5. Mark a UTC interval around one turn in the **already running** session. The
     provider audit must contain a request in that interval under the dedicated
     credential A. Run the evidence commands below and confirm the new
     `ConfigChange` route, the latest event's effective route, and its cost all
     use the custom provider and multiplier.
  6. Without restarting, switch to custom API-key credential B. Confirm the
     advisory says an already-keyed session keeps its current key. Record the
     `ConfigChange`-route count before sending another turn.
  7. Mark a second UTC interval around a turn in the same still-running session.
     The provider audit must still name credential A and contain no request for
     credential B. After both scans, confirm the `ConfigChange`-route count did
     not increase, there is no route for key rotation, and the latest event still
     resolves and prices through credential A's provider and multiplier.
  8. Still without restarting, switch to `official`. Confirm the advisory states
     restart is required. Record the route count, send another bounded turn, and
     confirm the provider audit still names credential A, the route count does
     not increase, and the latest event still resolves and prices through A.
  9. Restart the session, mark a fourth UTC interval around a turn, and run both
     scans. Confirm the custom provider audit contains no request in that
     interval, the new `SessionStart` route names `official`, and the latest
     event resolves and prices through that route.
- Evidence commands, run after setting the variables above. Preserve redacted
  output for Codex steps 1–2 and Claude steps 5, 7, 8, and 9; preserve the
  ConfigChange route count immediately before steps 7 and 8 as well:

  ```bash
  "$AGENTDECK_BIN" usage scan
  "$AGENTDECK_BIN" session scan

  sqlite3 -readonly "$STATE_DB" -header -column \
    "SELECT client,session_id,observed_at,hook_event,source,route_effect,
            config_matched,prior_state,conflict_scan,conflict_sources,delivery_id
       FROM usage_session_observations
      WHERE (client='codex' AND session_id='$CODEX_SESSION_ID')
         OR (client='claude' AND session_id='$CLAUDE_SESSION_ID')
      ORDER BY client,observed_at,id;"

  sqlite3 -readonly "$STATE_DB" -header -column \
    "SELECT id,observed_at,provider,multiplier,hook_event,source,quality
       FROM usage_session_routes
      WHERE client='claude' AND session_id='$CLAUDE_SESSION_ID'
      ORDER BY observed_at,id;"

  sqlite3 -readonly "$STATE_DB" -header -column \
    "SELECT COUNT(*) AS config_change_routes
       FROM usage_session_routes
      WHERE client='claude' AND session_id='$CLAUDE_SESSION_ID'
        AND hook_event='ConfigChange';"

  sqlite3 -readonly "$STATE_DB" -header -column \
    "WITH latest_event AS (
       SELECT event_at,event_key
         FROM usage_events
        WHERE client='claude' AND session_id='$CLAUDE_SESSION_ID'
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
            WHERE client='claude' AND session_id='$CLAUDE_SESSION_ID'
              AND observed_at<=e.event_at
            ORDER BY observed_at DESC,id DESC
            LIMIT 1
         );"

  "$AGENTDECK_BIN" --format json session show "$CLAUDE_SESSION_ID" \
    --client claude --tokens --all \
    | jq '.data | {usage, latest_invocation: .invocations[-1]}'
  ```
- Steps 5 and 7 are the Claude controls: if first-key addition does not take effect, or
  key rotation does take effect, the state machine this topic assumes is wrong
  and the design must be reopened rather than the test adjusted.
- Record the observed evidence in this task's review record. Real credentials are
  used by the operator and never committed; the record names providers,
  directions, route values, and attribution, never a credential value.
- Files: this topic's review record only. No product change.
- Verification level: L3 — real credentials and real client behavior, which no
  automated suite in this repository covers.

Commit boundaries follow task boundaries. This topic does not authorize commits,
pushes, release publication, or installation.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight. This topic adds no user-visible surface: it changes an advisory,
adds a shared internal Hook observation ledger, and corrects effective-route
writes. These are contracts `architecture.md` owns, not a user-facing state set
or copy that a surface document would review.

All three required design documents have passed independent Re-review. The topic
is developable; `hook-delivery-ledger` is the next implementation task, and the
matching records under `reviews/` own the findings and evidence.

## Tasks

| # | Task | Dev | Review |
| --- | --- | --- | --- |
| 1 | `hook-delivery-ledger` | [ ] | [ ] |
| 2 | `switch-effectiveness-contract` | [x] | [x] |
| 3 | `effective-route-policy` | [ ] | [ ] |
| 4 | `real-session-acceptance` | [ ] | [ ] |

Development is blocked until this `tasks.md` passes review, per
`.agent-instructions/beads.md`: a draft task matrix is not a source of task
anchors.

`tasks.md` Review Round 1 (2026-08-17): **FAIL** on T-F1–T-F5. The manual
acceptance still expects an unknown route, the status matrix retains the old
task-2 anchor, quality wording still reflects the superseded design, the direct
reconcile regression test is absent from task 2, and the real-session proof has
no reproducible discriminator or inspection commands. Its `Review` cell remains
unchecked; see [`reviews/tasks.md`](reviews/tasks.md).

`tasks.md` Re-review Round 2 (2026-08-17) passed against historical blob
`719ff3988ebe14699fc39a8f891f5b4ee253866b`. The 2026-08-25 state-machine
correction supersedes that reviewed decomposition; implementation is blocked
until the current four-task matrix passes review.

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
