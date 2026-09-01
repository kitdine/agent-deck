---
status: historical
created: 2026-08-17
updated: 2026-08-26
retired: 2026-09-01
---

# Switch Effectiveness Boundary — Architecture

Four contracts, one corrected premise. Every claim below about existing code
names its current file and symbol location. The exact HEAD and architecture blob
audited for each content state live in [`reviews/architecture.md`](reviews/architecture.md),
so this living design does not bind later readers to one historical source tree.

## Contract 0 — one accepted-Hook operation for every client

Codex and Claude expose different raw event sets today, but that difference ends
at the adapter boundary. `usagehook.ParseEvent` already returns one bounded event
shape with client, session, event name, source, and optional paths. Wire parsing
is only the first admission check, not acceptance itself: a delivery is
**accepted** only after the whole ordered admission sequence below passes. Every
accepted delivery then enters one client-neutral operation:

```text
raw client Hook
  -> client adapter parses the wire payload           (admission 1-2)
  -> semantic path/scope validation for this source   (admission 3-5)
  -> accepted: normalized HookDelivery + fresh delivery_id
  -> read the settings snapshot this attempt will classify from (config events)
  -> BEGIN
       -> duplicate-delivery guard on delivery_id
       -> classify route_effect from normalized facts + prior-route evidence
       -> append one privacy-bounded observation carrying that route_effect
       -> append zero or one effective-route row
     COMMIT (both rows or neither)
```

Classification precedes the observation insert, because the observation row
carries the classifier's result in `NOT NULL route_effect`. The executable
step-by-step order, including the duplicate guard, is fixed under *Whole-operation
idempotence*.

### Admission — what "accepted" means, in order

Admission is ordered and total. Every check below runs *before* a `HookDelivery`
is constructed, so nothing rejected can reach normalization, the observation
stream, or the route stream. The sequence is the one the current handler already
performs (`cmd/agentdeck/main.go:2884-2914`); this contract names it and forbids
reordering it, rather than introducing a new boundary:

1. **Bounded read.** Input longer than `usagehook.MaxEventBytes`, or unreadable,
   is rejected.
2. **Wire shape.** `usagehook.ParseEvent` must accept the client, event name,
   source, and optional paths.
3. **Runtime prerequisites.** The store must open and the home directory must
   resolve. A failure here rejects the delivery; it never partially admits one.
4. **Source scope.** A Claude `ConfigChange` is admitted only when
   `managedClaudeConfigChange` holds — `source == "user_settings"` and the
   cleaned `file_path` equals the managed `~/.claude/settings.json`. Every other
   `ConfigChange` source (`project_settings`, `local_settings`,
   `policy_settings`, `skills`) parses but is **not** admitted.
5. **Transcript scope.** A `SessionStart` delivery from either client is admitted
   only when `validHookTranscript` holds — the base name contains the session ID,
   the path is a regular non-symlink file, and it resolves inside the client's
   own session root.

Per-source admission outcome, exhaustively:

| Client and event | Admission requirement | Rejected effect | Accepted effect |
| --- | --- | --- | --- |
| Codex `SessionStart` | 1-3, 5 | no observation, no route | one observation; `route_effect` per policy |
| Claude `SessionStart` | 1-3, 5 | no observation, no route | one observation; `route_effect` per policy |
| Claude `ConfigChange` | 1-4 | no observation, no route | one observation; `route_effect` per policy |
| Claude `SessionEnd` | 1-3 | no observation, no route | one observation; `route_effect=none` |

`SessionEnd` keeps admission 1-3 only, matching today's handler: Claude's
contract gives it no required source, and this contract does not invent a new
path requirement for an event that writes no route.

Rejection is **fail-open and silent**: the handler returns success to the client,
writes nothing to either stream, and never blocks the client from starting,
resuming, or exiting. That is the existing Hook contract
(`cmd/agentdeck/main.go:2886-2894`), and the observation stream does not weaken
it. `cmd/agentdeck/hook_boundary_test.go` already pins the rejection cases; the
observation ledger extends those assertions to "neither stream was written".

The normalized input includes `client`, `session_id`, `hook_event`, `source`,
`delivery_id`, the completed selection when one exists, and nullable event facts
such as `config_matched`, prior authentication state, conflict scan, and settings
mtime. Fields that do not apply are NULL/`n/a`; they do not select another
handler or table.

One policy maps normalized facts to `route_effect`:

| Normalized event fact | `route_effect` | Effective-route result |
| --- | --- | --- |
| non-compact `SessionStart` with a completed selection | `advance` | append the loaded selection |
| compact start, `SessionEnd`, or another accepted non-boundary event | `none` | observation only |
| matched first-key `ConfigChange` confirmed by the classifier | `advance` | append the adopted selection |
| matched key rotation/removal or indeterminate change | `retain` | observation only; keep prior route |
| managed settings mismatch | `unknown` | preserve the explicit unknown route behavior |

**What `advance` means.** `advance` is *attempt the route write under today's
idempotence rule*, not *append a row unconditionally*. `recordSessionRoute`
(`internal/usage/routes.go:56-78`) suppresses a row whose
provider/multiplier/via_wrapper/hook_event/source equals the immediately
preceding row for that session. This contract **preserves that no-op
deliberately**: it is what keeps a repeated `startup`/`resume` on an unchanged
selection from inflating the route history, and replacing it is out of scope for
this topic. So `advance` guarantees an *effective-route state*, not a row count:
after the commit, `sessionRouteAt` resolves the session to the advanced
selection. A consecutive identical advance leaves the row count unchanged and is
still `route_effect=advance` in the observation stream — the observation records
the delivery's decision, the route table records the state. `unknown` writes
through the same rule.

Current transport capability is data, not a second operation: Codex currently
accepts `SessionStart`; Claude accepts `SessionStart`, `ConfigChange`, and
`SessionEnd`. Future accepted event kinds use the same operation. There is one
observation table, one delivery-identity rule, one transaction contract, one
privacy allowlist, and one resolver-isolation rule for every client.

## The Claude asymmetry this design is built on

`WriteClaudeConfig` (`internal/provider/config.go:638-671`) owns exactly two
fields and evaluates each one independently:

```go
if config.Endpoint == "" {
        delete(env, "ANTHROPIC_BASE_URL")     // :655
} else {
        env["ANTHROPIC_BASE_URL"] = ...       // :657
}
if config.Credential == "" {
        delete(env, "ANTHROPIC_AUTH_TOKEN")   // :660
} else {
        env["ANTHROPIC_AUTH_TOKEN"] = ...     // :662
}
```

Each field is written when its provider configuration value is present and
deleted when that value is absent. That describes the file, not whether a
running process adopts the change. Real-session observations establish this
state machine:

| Prior running-session authentication | New file shape | Live effect |
| --- | --- | --- |
| no managed API key | custom endpoint and first credential written | the sole supported live-update transition |
| API key A | custom endpoint and credential B written | no key replacement until restart |
| API key | direct `official`; endpoint and credential deleted | no return to subscription until restart |
| API key | `official --via`; endpoint written and credential deleted | endpoint may change, but not billing identity |
| any other state | endpoint-only or other configuration change | no supported live-effect claim |

The asymmetry is not a generic write/delete rule. Claude watches the settings
file, but a process that has already authenticated keeps that authentication.
Introducing the first key to a session that started without one is effective;
replacing or removing an existing key does not cause re-authentication. The
first two rows both write a credential, so `config.Credential != ""` is not a
sufficient discriminant.

The wrapper row remains a mixed state. Its endpoint change may be observable
while the process keeps presenting its old token. Billing identity follows the
credential, so attribution treats it like every other unadopted change and
preserves the prior route until restart.

**Route reconciliation must compare the new matched selection with an ordered,
three-state classification of the session's prior effective evidence.** The
classification is `keyed`, `no-key`, or `indeterminate`. `no-key` states that no
credential source this project models was in effect for the session — it is a
classification bounded by AgentDeck's recorded credential-source boundary, not a
proof about which authentication the process negotiated. It is resolved in this
order:

1. `usage.Service` queries the latest session route at the reconciliation time
   through `sessionRouteAt` (`internal/usage/routes.go:82-87`). A recognized
   custom-provider route is `keyed`; a recognized `official` route is a
   *candidate* `no-key` that step 3 must still confirm. An empty or `unknown`
   route is `indeterminate` and does not fall through to older timeline
   evidence.
2. Only when no session route exists, the service reads
   `usage_sessions.first_at` for the Claude session and calls
   `Store.ProviderSnapshotAt` (`internal/store/providers.go:823-824`) at that
   instant. A non-`Official` snapshot with a non-empty
   `ProviderSnapshot.Credential` is `keyed`; an `Official` snapshot with an
   empty credential is a *candidate* `no-key`. A missing or invalid start time,
   no snapshot, or any other contradictory snapshot shape is `indeterminate`.
3. A candidate `no-key` becomes `no-key` only when the managed settings file
   names no unowned credential source: any reported `env.ANTHROPIC_API_KEY` or
   `apiKeyHelper`, and any read or parse failure, downgrades the candidate to
   `indeterminate`. Steps 1 and 2 describe only AgentDeck's own managed
   selection, so neither alone can distinguish a session that started
   unauthenticated from one authenticated by a source AgentDeck did not write.
4. A matched route write is permitted only for confirmed `no-key` plus a
   consistent non-`Official` new selection with a non-empty credential. `keyed`
   and `indeterminate` both suppress the matched write; a settings mismatch
   retains the separate explicit-unknown behavior in Contract 3.

**One settings snapshot, not two reads.** `ClaudeConfigMatchesSnapshot`
(`internal/provider/config.go:145-152`) and `ClaudeCredentialConflicts`
(`:173-191`) each open and parse the settings file independently today. Two
independent reads let a file change between them pair a match derived from one
file state with a clean conflict scan derived from another, which would promote a
false `no-key` and write a route the session never adopted. This contract
therefore requires **one parsed settings snapshot per reconcile attempt**:

- The provider package gains a read-once entry point that reads the managed
  settings file a single time and returns an in-memory parsed document plus its
  observed `mtime`. Both the match evaluation and the conflict scan are computed
  from **that one document**; neither re-reads the path.
- The existing exported `ClaudeConfigMatchesSnapshot` and
  `ClaudeCredentialConflicts` keep their signatures and behavior for their
  current callers (the advisory path). The reconcile path uses the snapshot-based
  form so that match and conflict cannot disagree about which file state they
  saw.
- A read or parse failure of that single snapshot yields `indeterminate` and
  writes no matched route, exactly as a conflict-scan failure does today.
- The snapshot is per attempt. `reconcileClaudeConfigChange` already retries up
  to three times; each attempt takes a fresh snapshot, and the attempt that
  decides is the one whose snapshot produced the match.

**Serialization of the decision.** The prior-route read (steps 1-2), the
classification, and the optional route write happen inside the single
`RecordHookDelivery` transaction described under *Ownership and failure
semantics*, on the same `*sql.Tx` as the observation insert, in the executable
order fixed under *Whole-operation idempotence*: the duplicate-delivery guard
runs first, this classification second, the observation insert third, and the
optional route write last. A decision may never be committed from prior-route
evidence read outside that transaction. This is a
placement rule, not a new concurrency primitive: the store is single-writer
SQLite, so a transaction that reads the latest route and then appends is already
serialized against another writer's route append. The settings file is outside
that transaction, which is exactly why its stability is handled by the single
snapshot above rather than by the database.

**What this classification does not cover, and why that boundary is the
project's, not this contract's.** A credential exported into the shell
environment, or parked in a Claude settings scope this project does not model,
is invisible to every check above. That is the same boundary the `official`
conflict advisory already ships with — AgentDeck writes exactly one Claude
settings file and reads exactly that file back, and adopting Claude's full
settings-resolution order was rejected as a client behavior this project does
not track (scope decisions recorded in
`docs/archive/plans/provider-wrapper-routing.md`, documented for users in
`docs/specs/cli-manual.md`). Under that boundary every existing Claude
attribution is already bounded in the same way: a shell-exported key makes
today's `official` routes wrong too. This contract therefore does not widen the
boundary and does not claim to prove process authentication. It claims the
strongest classification the modeled evidence supports, fails closed everywhere
else, and leaves the unmodeled sources to the existing project-wide boundary
rather than silently treating a managed `official` state as proof of an
unauthenticated process.

Shared `usage.Service.RecordHookDelivery` owns observation persistence and the
optional route write. Its Claude `ConfigChange` policy uses a private prior-state
helper rather than asking `cmd/agentdeck` to pass a lossy boolean. Keeping
classification inside the shared operation prevents an adapter or caller from
bypassing the fail-closed policy without creating a client-specific transaction.
The advisory site still lacks per-session state, so its copy must state the
conditional rule rather than claim that the current switch reached every
running session.

## Contract 1 — the switch advisory is direction-aware

`SwitchAdvisories` (`internal/provider/service.go:780-805`) returns
`claudeRestartAdvisory` (`:808`) for every Claude switch. It can inspect the new
selection but has no per-session authentication state. It therefore uses two
conservative texts selected by whether the new selection contains a credential;
neither claims that every credential write took effect.

| New selection | Advisory |
| --- | --- |
| contains a credential | `restart running Claude sessions to guarantee this switch: only a session that started without an API key may adopt its first key live; a session already authenticated with a key keeps it until restart` |
| contains no credential | `restart running Claude sessions to guarantee this selection: removing a key does not re-authenticate a session that already holds one, and adoption of other configuration changes is not established until restart` |

The first text states both outcomes of a credential write because the command
cannot know which running sessions already hold keys. The second states only
the guaranteed boundary: key removal does not re-authenticate an already-keyed
session, other configuration adoption is unknown, and restart is what guarantees
the complete new selection. It does not claim the process kept every startup
configuration field.

Both begin with the same imperative. The user's action is identical in both
directions — restart the session — and only the reason differs; an advisory that
changed its opening verb would suggest the required action changed too.

Unchanged in this contract:

- The conflict advisories for `official`
  (`env.ANTHROPIC_API_KEY`, `apiKeyHelper`, `:797-802`) keep their text,
  ordering, and position before the restart note. They describe sources
  AgentDeck does not own, which is a different concern from effectiveness.
- Advisory generation never fails a completed switch (`:777-779`), and no
  advisory carries a credential value.
- `codexRestartAdvisory` (`:807`) is untouched. Codex activates nothing without
  a restart, so its note has no direction to distinguish.

`SwitchAdvisories` currently takes `(client, name, configPath)`. It needs the
credential presence of the selection being reported. Passing the resolved
`ClientConfig`, or a boolean derived from it at the one call site, are both
acceptable; `tasks.md` owns which, and the signature change is internal — no
command surface takes a new flag.

## Contract 2 — what `ClaudeConfigMatchesSnapshot` proves

`ClaudeConfigMatchesSnapshot` (`internal/provider/config.go:145-153`) reads the
settings file and compares the AgentDeck-owned fields. Its result is a fact
about the file and nothing else. Today's callers use it as though it were a fact
about the running client, which is the second half of this defect.

The function's behavior does not change. What changes is that the boundary is
stated, in its own doc comment and at the caller, so the next reader does not
re-derive the same wrong conclusion:

> A match proves the managed fields on disk equal the completed selection. It
> proves nothing about which credential a running client is currently
> presenting. After key replacement or removal, the two can differ indefinitely:
> a match proves the write completed, not that any running session
> re-authenticated.

This is a documentation-and-caller contract rather than a code change because
the function is correct for what it measures. Renaming it to imply a narrower
claim was considered and rejected: every caller would still have to decide what
to conclude, and the name would carry the reasoning instead of the reasoning
being written where the decision is made.

## Contract 3 — only an effective first-key transition records a matched route

`reconcileClaudeConfigChange` (`cmd/agentdeck/main.go:2922-2952`) currently
reconciles the file and calls Claude-specific
`RecordClaudeConfigChange(ctx, sessionID, snapshot, matched)`
(`internal/usage/routes.go:45-54`). The target replaces that ownership boundary
with shared `RecordHookDelivery`; the adapter supplies normalized facts and the
shared policy produces `route_effect`. The current branch below is the behavior
the Claude config policy must preserve:

```go
if !matched {
        return s.recordSessionRoute(ctx, route, "unknown", "1", false)   // :51
}
return s.recordSessionRoute(ctx, route, runtimeProviderName(snapshot.Name), ...)  // :53
```

Today every matching selection takes the second branch. That includes the valid
first-key transition, but also key rotation and key removal that the running
session did not adopt. `priceForEvent` then adopts the newly written provider
and multiplier verbatim (`internal/usage/usage.go:2617-2621`) for subsequent
events even when the running client still bills against the prior API key.

**A matched selection records a `ConfigChange` route only when the ordered
classifier above confirms a `no-key` prior state, making the change a
`no key -> first key` transition. Key rotation, key removal, and every other
matched but unadopted change record no route.** A settings mismatch
keeps today's explicit unknown branch; it is not reclassified as a known
transition by this contract.

"Records no route" is a statement about the effective-route stream only. Every
Hook observation is still persisted; the two are separate streams, specified
below.

For an unadopted matched change, the provider is not unknown. It is the
*previous* selection — the session is still presenting the authentication it
already held, and AgentDeck recorded that selection when the session started or
when its first key took effect. Writing `unknown` would discard information the
store already holds and would report a session as unattributable when it is
precisely attributable.

Recording nothing is what expresses that. `sessionRouteAt` positions an event
against the most recent route at or before its time
(`internal/usage/usage.go:2504-2510`):

```go
index := sort.Search(len(routes), func(i int) bool { return routes[i].observedAt.After(at) })
if index == 0 { return readSessionRoute{}, false }
return routes[index-1], true
```

With no new route written, every subsequent event in that session continues to
resolve to the route that was already in effect — the prior provider and its
multiplier, which is what the session is actually billing at. The absence of a
new route is not a gap; it states that nothing about this session's effective
routing changed.

A session may also have no route of its own, since `RecordSessionRoute` writes
nothing without a completed selection (`internal/usage/routes.go:33-34`) and skips
`compact` (`:29`). `priceForEvent` then falls back to the provider timeline at
session start (`internal/usage/usage.go:2622-2634`), which resolves to the
selection the session started under — the same answer. `requirements.md`'s
"What an unadopted switch attributes to" states the full hierarchy and
the quality each case carries; this contract only decides that no row is written.

This also removes the need for a quality decision. There is no new route row, so
there is no quality to assign; the observation stream below carries no quality
column for the same reason, because nothing prices from it. Nothing here touches
`usage-attribution-precision`'s redesign of that dimension.

The retained state does not persist indefinitely. A restart writes a fresh route from
`SessionStart` reading the completed selection
(`internal/usage/routes.go:22-40`), and `startup`, `resume`, and `clear` all
reach that path (`internal/usagehook/event.go:88`). So the old route governs
exactly the interval where the old credential is in effect, and the new selection
takes over at the boundary where the client actually adopts it. The advisory from
contract 1 is what tells the user that boundary is a restart.

What this deliberately does not attempt:

- **It does not verify that the running session is in fact still using the old
  credential.** No supported mechanism exists. The design rests on the mechanism
  in the state table above — replacement or deletion does not re-authenticate an
  already-keyed process — which is why `tasks.md` requires a real-session
  acceptance rather than treating unit tests as proof.
- **It does not distinguish a session that started before the switch from one
  that started after.** A `ConfigChange` hook fires for the session that observed
  the change, so by construction it predates the change. A session started after
  gets its route from `SessionStart`, which this defect never touched.
- **It does not suppress the route for a confirmed first-key transition.** That
  one credential-writing transition is the only one whose prior state the
  modeled evidence can classify, within the credential-source boundary stated
  above. A later credential write is key rotation and remains on the prior
  route.

### Two persistence streams

A Hook observation and the route a running session actually adopted are
different facts, and this contract keeps them in different stores. Collapsing
them into one route write is what forced the previous draft into a false choice:
writing every observed selection to `usage_session_routes` corrupts pricing,
because `sessionRouteAt` treats the latest row at or before an event as the
provider in effect; writing nothing at all discards the Hook fact, including the
key-mode determination the classifier just made.

**Stream 1 — observations, append-only.** Every accepted Codex or Claude Hook
delivery that commits appends exactly one row to one `usage_session_observations`
table, whether or not it advances a route. The row records what was observed and
what was concluded, never what was configured as a secret. An accepted Hook means
one delivery invocation that passed the full admission sequence in Contract 0;
without source identity, an external replay is another delivery.

The exact schema, added by migration 19 in the style of migration 17
(`internal/store/migrations.go:105-109`):

```sql
CREATE TABLE IF NOT EXISTS usage_session_observations (
  id INTEGER PRIMARY KEY,
  client TEXT NOT NULL,
  session_id TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  hook_event TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  config_matched INTEGER,
  observed_provider TEXT,
  observed_multiplier TEXT,
  observed_via_wrapper INTEGER,
  prior_state TEXT,
  conflict_scan TEXT,
  conflict_sources TEXT NOT NULL DEFAULT '',
  route_effect TEXT NOT NULL,
  settings_changed_at TEXT NOT NULL DEFAULT '',
  delivery_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS usage_session_observations_lookup
  ON usage_session_observations(client,session_id,observed_at);
CREATE UNIQUE INDEX IF NOT EXISTS usage_session_observations_delivery
  ON usage_session_observations(delivery_id);
```

Encoding is fixed so no implementer has to choose one:

| Column | Encoding | Applicability |
| --- | --- | --- |
| `client`, `session_id` | the session the hook fired for | always |
| `observed_at` | RFC3339Nano, from the same clock `recordSessionRoute` uses | always |
| `hook_event`, `source` | normalized event name and source; `source` is `''` when the client sends none | always |
| `config_matched` | `1`/`0`; NULL for a non-config event | config events only |
| `observed_provider`, `observed_multiplier` | the runtime provider name and multiplier strings `recordSessionRoute` uses; NULL when no completed selection was seen | when a selection exists |
| `observed_via_wrapper` | `1`/`0`; NULL exactly when `observed_provider` is NULL | with the selection |
| `prior_state` | `keyed`, `no-key`, or `indeterminate`; NULL when the classifier did not run | classified config events only |
| `conflict_scan` | `clean`, `conflicted`, or `unreadable`; NULL when no scan ran | classified config events only |
| `conflict_sources` | the reported key names joined by `,` in `ClaudeCredentialConflicts` order; `''` when none or not scanned; never NULL | config events |
| `route_effect` | exactly one of `advance`, `retain`, `unknown`, `none` | always |
| `settings_changed_at` | RFC3339Nano managed-settings mtime at reconcile, `''` when unavailable or inapplicable; diagnostic, never identity | config events |
| `delivery_id` | opaque internal ID generated once for this handled delivery | always |

Nullable columns are NULL — never `''`, never `0` — when the fact does not
apply, so a reader can tell "not applicable" from "observed as false/empty". The
two `NOT NULL DEFAULT ''` text columns (`conflict_sources`,
`settings_changed_at`) are set-valued rather than tri-state, which is why they
use an empty string instead.

No column holds a credential value, an endpoint, a settings-file path, a prompt,
or any transcript content. `conflict_sources` holds key *names* for the same
reason `ClaudeCredentialConflicts` returns names only: one of the two keys holds
a credential.

The migration is purely additive: it creates one table and two indexes and
changes no existing table, column, or row.

**Whole-operation idempotence, and the executable order inside the
transaction.** The unique index on `delivery_id` alone is not enough, because a
retry that finds the observation already present must also not repeat the route
side. The operation is therefore guarded as a whole. Because
`route_effect` is `NOT NULL` and holds the classifier's result, classification
must complete *before* the observation insert, not after it. `RecordHookDelivery`
executes exactly this order:

0. **Before the transaction**, for a config event, take the single parsed
   settings snapshot Contract 3 requires. It is a filesystem read, not a
   database read, and it is deliberately outside the transaction; its stability
   is handled by the snapshot rule, not by the store.
1. **BEGIN.**
2. **Duplicate-delivery guard.** `SELECT 1 FROM usage_session_observations WHERE
   delivery_id = ?`. If a row exists, this delivery ID was already committed: the
   whole operation is a no-op — classify nothing, insert nothing, **skip the
   route write**, `COMMIT`, and return success.
3. **Classify.** On this same transaction, read the prior-route evidence
   (`sessionRouteAt`, and the session-start provider snapshot when no route
   exists), combine it with the settings snapshot from step 0 and the normalized
   event facts, and compute `route_effect` together with the observation's
   `config_matched`, `prior_state`, `conflict_scan`, and `conflict_sources`
   values.
4. **Insert the observation**, carrying the `route_effect` computed in step 3.
   The insert stays conditional —
   `INSERT INTO usage_session_observations(...) SELECT ... WHERE NOT EXISTS
   (SELECT 1 FROM usage_session_observations WHERE delivery_id=?)` — so the
   guard in step 2 has a storage-level backstop rather than a
   check-then-act race.
5. **If that insert affected zero rows**, the same delivery ID was committed
   between steps 2 and 4. Treat it exactly like step 2: **skip the route write**,
   `COMMIT`, and return success. Steps 2 and 5 are the same no-op outcome reached
   from two different points; the classification performed in step 3 is simply
   discarded.
6. **Only when the insert affected one row**, apply `route_effect`: perform the
   zero-or-one route write through the unchanged consecutive-identical no-op
   rule.
7. **COMMIT.** Both rows, or the observation alone, or — on any error in steps
   2-6 — neither.

So an internal retry with the same delivery ID leaves exactly one observation
*and* the route table exactly as the first commit left it. The unique index
remains as the storage-level backstop for that invariant; it does not claim to
identify or suppress an external replay, which carries a different ID by
construction.

Ordering is `observed_at` then `id`, matching `sessionRouteAt`, so the two
streams can be read against one another in a diagnosis without a second time
model.

**Delivery identity and replay.** The normalized Hook envelope carries no event
ID and no event timestamp (`internal/usagehook/event.go:11-16`). File mtime
identifies a file state rather than the Hook occurrence: two independent
deliveries may share one tick, and a delayed replay may inspect a later state.
Neither mtime nor the observation tuple may therefore suppress a row.

The handler generates a fresh opaque `delivery_id` once when it accepts a
delivery, before reconciliation and before opening the store transaction. The
same in-process store operation retains that ID across an internal retry, so the
unique index makes that retry a no-op rather than a second observation. A later
transport delivery — including an external replay of the same source Hook — has
no stable source identity to correlate and receives a new ID. It appends another
row. Possible replay duplicates are accepted because suppressing one would risk
discarding an independent Hook fact.

`settings_changed_at` remains diagnostic evidence only. It may help a future
reader mark two rows as probable duplicates, but this contract neither stores
such a disposition nor uses it for insertion. An unreadable mtime leaves the
field empty and does not weaken delivery identity. If external replay
suppression becomes a requirement, the Hook transport must first supply a
stable source event ID or timestamp; inferring one from current file metadata is
explicitly rejected.

**Stream 2 — effective routes, unchanged.** `usage_session_routes` keeps exactly
today's schema and today's meaning: the provider a session is billing at from
that instant forward. It advances in only two ways. `SessionStart` writes the
then-current completed selection when the session starts or restarts
(`internal/usage/routes.go:22-40`), and any normalized delivery with
`route_effect=advance` writes its adopted selection under the unchanged
consecutive-identical no-op rule (`internal/usage/routes.go:56-78`), so the
session's resolved route is the advanced selection whether or not a new row was
needed. `retain` and `none` write no route. A settings mismatch keeps its pre-existing unknown write through
`route_effect=unknown`, which this contract does not reclassify.

**The resolver reads stream 2 only.** `sessionRouteAt`
(`internal/usage/usage.go:2504-2510`) and `priceForEvent` are not changed and
must not join `usage_session_observations`. An observation is evidence about a
file and a classification; it is never evidence about what a process is billing,
which is the whole defect this topic exists to fix. Cost output therefore cannot
regress on the observation stream, and the topic's no-recomputation non-goal
holds: no existing row is reinterpreted.

**Ownership and failure semantics of the write.**
`usage.Service.RecordHookDelivery` performs the observation and optional route
write for every accepted Hook in one store transaction. The transaction is
atomic: all applicable rows commit or none does. There is no client-specific
transaction, partially durable state, or recovery protocol. A failure returns an
error and leaves the store exactly as it was — the same failure mode a route
write has today, not a new one this contract introduces.

**Cardinality is a property of successful commits, not of attempts.** The
"exactly one observation per accepted delivery" promise is stated precisely as:

- An accepted delivery whose transaction **commits** leaves exactly one
  observation row for its delivery ID, plus zero or one route row.
- An accepted delivery whose transaction **fails** leaves zero observation rows
  and zero route rows. The handler is fail-open: it drops the delivery, returns
  success to the client, and does not retry across process boundaries, record a
  pending marker, or reconstruct the row later. There is deliberately no
  recovery protocol, because nothing in this topic reads the observation stream.
- Two successful commits for one delivery ID are impossible by the
  whole-operation guard above; an internal retry of the same operation is a
  no-op, not a second commit.

The observable invariant an implementation and its tests must satisfy is
therefore `0 <= observations(delivery_id) <= 1`, with `1` exactly when that
delivery's transaction committed. A dropped delivery is a missing diagnostic row,
never a wrong figure, because the pricing resolver never reads this stream. The
same wording is carried by `requirements.md`'s acceptance boundary and by
`tasks.md`'s Task 1 assertions.

Atomicity is chosen over a separately committed observation because the
alternative buys nothing here. An observation committed alone with
`route_effect=advance` would claim a route transition that did not commit;
correcting it would require a replay/recovery protocol for a row nothing reads.
An `advance` observation therefore always describes a committed route state —
either a row this transaction appended, or the identical row the no-op rule kept.

`cmd/agentdeck` routes every accepted normalized delivery into the shared service
operation. Its client adapters may populate different nullable facts, but the
command boundary does not decide which store, transaction, privacy rule, or
route write applies.

What the observation stream is for, and what it is not: it preserves the Hook
fact — the selection observed, the key-mode determination, and the conflict
evidence behind it — so that a later diagnosis, or
`usage-attribution-precision`'s redesign of evidence quality, can use a real
record instead of re-deriving a state that no longer exists. This topic reads it
nowhere. Adding a reader is a change that topic owns, not this one.

### Why not `unknown`

An earlier draft of this contract recorded the unknown route here, reasoning that
if we cannot observe which credential the process holds then the provider is
undeterminable. That conflates two different things: we cannot *observe* it, but
we can *derive* it, because an unadopted matched switch does not change what a
running session presents and the previous selection is recorded. Reporting
`unknown` would have replaced a correct attribution with an absent one and made
the topic's own goal — never present a figure at a confidence the evidence does
not support — fail in the other direction, by withholding a figure the evidence
does support.

## Corrected premise in the attribution architecture

The earlier 2026-08-17 draft of
`docs/topics/usage-attribution-precision/architecture.md` read:

> **Claude** activates immediately, and mid-session changes are separately
> recorded as `ConfigChange` routes. Positioning by event time inside that route
> sequence yields the provider actually in effect.

This is the stated ground for promoting Claude `ConfigChange` routes to `exact`.
Immediate activation holds only for a session that started without an API key
and then obtained its first key. It does not hold for key rotation, key removal,
or other settings changes. Positioning by event time is therefore sound only
for routes whose transition was proven effective, not for every provider the
file names.

That document's `Review` cell is unticked and it has no review record, so this
is a draft correction rather than a reopened verdict. Its current passage now
states the transition dependency and points at this topic's Contract 3: only
`no key -> first key` creates a matched `ConfigChange` route. An unadopted
matched switch creates none. The retained prior route continues to supply its
already stored provider, multiplier, and quality (currently `estimated`); when
no session route exists, the unchanged session-start fallback applies. This
topic therefore assigns no new quality, while that topic remains free to
redesign the quality of the evidence sources that actually exist.

## Verification

L2 for the route contract: it changes a persisted attribution value that cost
output reads. L1 suffices for the advisory text alone, but the two ship
together, so L2 covers both. The observation stream ships in the same task and
stays at L2 rather than rising: it adds a table nothing reads, so its failure
mode is a missing or duplicated diagnostic row, not a wrong figure.

The additive migration carries its own two cases, because a migration is the one
part of this contract that runs against state a test did not create. A fresh
database must reach schema version 19 with `usage_session_observations`, its
`(client, session_id, observed_at)` lookup index, and the unique `delivery_id`
index present and with the exact column set and encodings tabulated above; and a
database already at version 18 with existing route and event rows must reach the
same shape without touching `usage_session_routes`, `usage_events`, or any
existing row. Migration failure must leave the store usable at its prior version
rather than half-applied, which is the existing transactional guarantee and is
asserted rather than assumed.

Automated tests can prove the file-level behavior — which advisory text each
direction produces, and which route a reconcile writes. They cannot prove the
claim this topic exists for, which is about a process that authenticated before
the file changed. `tasks.md` owns the manual procedure; `requirements.md`'s
acceptance boundary requires it, and it is not satisfiable by unit tests over
`settings.json`.

The observation tests are shared across clients and assert the split this
contract exists for: every accepted delivery whose transaction commits appends
exactly one observation, and a rejected or failed delivery appends none;
non-compact Codex and Claude `SessionStart` deliveries append an observation and
resolve the session to the loaded selection, a repeated identical non-compact
start appends a second observation while the route row count stays unchanged,
compact/terminal events append observations only, an adopted first-key change
appends one observation and route, and retained/indeterminate changes append
observations without routes. Admission is asserted from the other side too: an
out-of-root or session-mismatched transcript, and a `ConfigChange` on a
non-`user_settings` source or an unmanaged path, each append neither an
observation nor a route. No observation column ever holds a
credential value; pricing a session with observations but no new route still
resolves through the prior route, proving the resolver never reads stream 1.

Delivery identity gets its own cases, because incorrect deduplication silently
loses history: two external deliveries over one unchanged file append two rows
with different delivery IDs; two independent deliveries with the same mtime and
conclusion also append twice; an unreadable mtime still appends one row for the
delivery; and an internal retry of one store operation with the same delivery ID
leaves one row **and no second route row**, which is the assertion the unique
index alone would not force. The transaction case asserts the atomic contract in
the direction that can go wrong: a failing route write leaves no observation
behind, and a delivery whose transaction fails leaves both streams untouched.

The route tests cover every classifier branch: latest custom and latest
`official` routes; no-route sessions whose start snapshot is keyed or official;
unknown-route, missing-start, missing-snapshot, and contradictory-snapshot
cases; and the unowned-source downgrade — a candidate `no-key` whose settings
file names `env.ANTHROPIC_API_KEY` or `apiKeyHelper`, and one whose settings
file cannot be read or parsed, both classify `indeterminate`. Only a confirmed
`no-key` prior state plus a first-credential selection writes the matched route;
every keyed or indeterminate case writes none.

The shared-operation tests prove that raw client adapters converge before
persistence: equivalent normalized facts from either client produce the same
observation schema, delivery-ID handling, transaction boundaries, privacy
filtering, and `route_effect` application. Adding a future accepted client event
must require only adapter normalization plus a policy-table case, never another
store or transaction implementation.
