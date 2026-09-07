---
status: active
created: 2026-09-07
---

# Schema Version Signal — Architecture

This is stage 5 of the progression in `docs/documentation-workflow.md`: the
contract stage. It answers every request
[`ux/menubar-schema-signal.md`](ux/menubar-schema-signal.md) filed under **Data
requirements** — provisioning it or refusing it with a stated ground — and it
specifies the contracts [`requirements.md`](requirements.md) declared in scope
but deliberately left undecided.

The boundary is `requirements.md`'s acceptance list. This document does not
revisit it; where a decision here narrows or reshapes one of its expectations,
that is stated in the open, under **Where this document departs from
`requirements.md`**.

## Source coordinates

Every line number below is at `34bf55e` (2026-09-07). `requirements.md` and the
surface document both state their coordinates at `8d283cd` (2026-09-05);
`git diff --stat 8d283cd..34bf55e -- '*.go' '*.swift'` is **empty**, so no
coordinate in either document has drifted and the two sets are interchangeable.
Every claim about existing code below names the file and line where it was read.

## What the condition is, in code

Five sites decide or report it today:

| Site | What it does |
| --- | --- |
| `OpenReadOnly` (`internal/store/store.go:200-202`) | Compares the stored version to `CurrentSchemaVersion` and returns the bare `ErrUnknownSchema` sentinel — the two numbers are discarded |
| `migrate` (`internal/store/migrations.go:316-317`) | Compares the same two numbers and wraps them into a message: `database version %d exceeds supported version %d` |
| `open` (`internal/store/store.go:207-249`) | Acquires the state lock **before** `migrate` runs, so a held lock returns `ErrStateBusy` and the comparison never happens |
| `errorCode` (`cmd/agentdeck/main.go:336-374`) | Lists `store.ErrStateBusy` but not `store.ErrUnknownSchema`, so the wrapped error falls to the `runtime_error` default at `:371-372` |
| `databaseCode` (`internal/doctor/doctor.go:474-479`) | Maps `ErrUnknownSchema` to the check code `unknown_schema`, with no count and no recovery |

`CurrentSchemaVersion` is `23` (`internal/store/store.go:24`).

### The sentinel already carries four conditions

This is the fact that decides the first contract below, and it is not visible
from either earlier document. `ErrUnknownSchema`
(`internal/store/store.go:134`) is returned from **five** sites, of which only
two are this topic's condition:

| Site | Condition | Has two version numbers? |
| --- | --- | --- |
| `store.go:201-202` | stored version exceeds supported | yes |
| `migrations.go:316-317` | stored version exceeds supported | yes |
| `migrations.go:366` | `missing schema metadata` | no |
| `migrations.go:413` | `expected one schema version row, found %d` | no |
| `migrations.go:418` | `missing version row` | no |

The last three are metadata damage. They have no supported/stored pair to
report, and their recovery is not "upgrade the binary" — upgrading changes
nothing about a database whose `schema_metadata` table is missing or holds two
rows. A code that covers all five would be a code whose documented meaning is
false for three of them, and whose new `supported_count` field would be absent
exactly when a consumer most needs to know why.

## C1 — The stable code is `schema_ahead`, a new value

**Decision.** The condition gets a new stable `error.code`, `schema_ahead`.
`unknown_schema` is not promoted.

**Ground.** Two reasons, in order of weight:

1. The section above: `unknown_schema` already means four things, three of
   which cannot satisfy the contract this topic is writing. Promoting it would
   publish a code whose documented meaning — "the database is newer than this
   binary; upgrade" — is wrong for three of its five producers.
2. `schema_ahead` completes a pair the product already has. `doctor` reports
   `schema_outdated` for a database *behind* the binary
   (`internal/doctor/doctor.go:82`); this is the same axis in the other
   direction, and naming it in the same vocabulary is what makes the two
   readable as one contract rather than as two unrelated failures.

`unknown_schema` keeps its current meaning for the three metadata-damage sites
and keeps reaching the CLI as `runtime_error`. That residual is **not** fixed
here: it is a different condition with a different recovery and no measured
report behind it, and inventing a code for it inside this topic would be
exactly the scope creep `requirements.md`'s non-goals refuse. It is recorded
under **Known residuals** below so that it is carried rather than forgotten.

### The carrier

The two version numbers must survive from the comparison site to three
consumers — the CLI error envelope, the `doctor` check, and the desktop wire —
so the sentinel gains a typed carrier beside it:

```go
// internal/store
var ErrSchemaAhead = &Error{Code: "schema_ahead"}

// SchemaAhead reports a database whose schema version exceeds the version this
// binary supports. It is permanent until the binary is upgraded.
type SchemaAhead struct {
    Stored    int
    Supported int
}

func (e *SchemaAhead) Error() string {
    return fmt.Sprintf("%s: database version %d exceeds supported version %d; upgrade AgentDeck to open it",
        ErrSchemaAhead.Code, e.Stored, e.Supported)
}

func (e *SchemaAhead) Unwrap() error { return ErrSchemaAhead }
```

`Unwrap` returning the sentinel is what makes `errors.Is(err, store.ErrSchemaAhead)`
true for callers that only need the classification, while
`errors.As(err, &ahead)` gives the two numbers to the callers that render them.
This is the shape `errorCode` already uses for `*errdefs.NotFound`
(`cmd/agentdeck/main.go:337`, `:361-362`), so it introduces no new idiom.

Both comparison sites return it: `store.go:201-202` replaces its bare sentinel,
and `migrations.go:316-317` replaces its `fmt.Errorf` wrap. The message text
above is the one that reaches the user through the JSON error envelope, because
`main.go:301` builds that envelope from `errorCode(err)` and `err.Error()`.

**The message carries the recovery.** That is deliberate and it is how
acceptance item 3 is satisfied on the CLI's read-write paths; see C6.

### Mapping

- `errorCode` (`cmd/agentdeck/main.go:336-374`) gains
  `case errors.Is(err, store.ErrSchemaAhead): return store.ErrSchemaAhead.Code`,
  placed beside the existing `store.ErrStateBusy` case at `:363-364`.
- `databaseCode` (`internal/doctor/doctor.go:474-479`) checks `ErrSchemaAhead`
  first and returns `schema_ahead`; its existing `ErrUnknownSchema` branch stays
  for the three metadata sites, and `database_unreadable` stays the default.

### Compatibility

This is the same class of change as the five narrowings `v0.5.0` already
recorded in `docs/specs/cli-design.md`'s Error-Code Compatibility section
(`:2200-2220`): a condition a consumer received as `runtime_error` now returns a
specific stable code, at an unchanged exit status of `1`. A consumer matching
`runtime_error` to detect a too-new database stops matching. Under the
release-position rules that section cites, that is MINOR.

It is one row, not two: `doctor` never emitted `runtime_error` for this
condition — it emitted the check code `unknown_schema`, which is a different
key in a different payload — so the compatibility statement covers the
read-write command paths only.

## C2 — Two numbers, carried as `count` and `supported_count`

**Decision.** The stored version stays in the existing `count`. The supported
version is a new integer key, `supported_count`, added to the check payload in
all three carriers.

```
doctor.Check          (internal/doctor/doctor.go:23-29)
  → desktop.HealthCheck (internal/desktop/desktop.go:214-220)
    → DesktopHealthCheckV1 (apps/macos/AgentDeckShared/DesktopWire.swift:941-953)
```

The three are a literal mirror: `healthSnapshot` (`internal/desktop/desktop.go:758-771`)
copies the five keys field by field, and the Swift type decodes them by name.
A field added to one and not the others does not reach the menu bar, which is
why the surface named all three.

**Ground for the names.** The surface asked for both numbers as integers and
took no position on shape. `count` already holds a schema version on the
`schema_outdated` path (`internal/doctor/doctor.go:82`, `Count: version`), so
the stored version belongs there; moving it would be a breaking change to a
key a shipped consumer reads. Given that, the second number's name has to pair
with `count`. `supported_version` would be the better name in isolation and the
worse name here: it implies a sibling `version` key that does not exist, and a
reader who goes looking for it finds the value in `count` instead. That
asymmetry is a puzzle handed to every future reader; `supported_count` has none.

**Wire version.** `supported_count` is additive to desktop wire version 1 and
does not raise it. `cli-design.md:2523` records that everything `v0.5.0` added
to that wire is additive, and the Swift type decodes it as `Int?`, so
`snapshot-legacy.json` — a v1 payload predating the field — still decodes. The
wire version is not a payload hash; it changes when a consumer that understood
the old payload can no longer understand the new one, which is not the case for
an optional integer.

### C2.1 — `schema_outdated` gains the same field

**Decision.** The `schema_outdated` check also sets `supported_count`.

**Ground.** `docs/specs/cli-design.md:2275-2276` already states that doctor
"reports `schema_outdated` with the stored and supported versions". The code
reports one: `internal/doctor/doctor.go:82` sets `Count: version` and nothing
else. The contract text has been false since it was written, and this topic is
rewriting the paragraph directly above it. Leaving the older half false while
making the newer half true would also produce the odd result that the
*forward* condition carries both numbers and the *backward* one — the condition
`cli-design.md` explicitly promises both for — carries one.

**This one is separable and may be refused.** It is the only decision in this
document that reaches a check outside the topic's condition. If review rejects
it, nothing else here changes; `cli-design.md:2276` must then be corrected to
claim only the stored version, because the sentence cannot be left standing
either way. Its cost is one expectation in each of the two existing
`schema12` regression cases (`internal/doctor/doctor_test.go:346` and
`cmd/agentdeck/main_test.go:1458`).

## C3 — Precedence over `state_busy`, decided at the lock's failure

**Decision.** `open` (`internal/store/store.go:207-249`) keeps acquiring the
lock first. When — and only when — acquisition fails with `ErrStateBusy`, it
reads the stored schema version from the database at rest and returns
`*SchemaAhead` instead if the version exceeds `CurrentSchemaVersion`.

```go
lock, err := acquire(ctx, stateRoot, lockWait)
if err != nil {
    if errors.Is(err, ErrStateBusy) {
        if ahead := schemaAheadAtRest(ctx, stateRoot); ahead != nil {
            return nil, ahead
        }
    }
    return nil, err
}
```

**Ground.** The obvious alternative is to probe before acquiring, which would
put an extra SQLite open and query on every read-write command including the
Hook handler, whose whole design budget is "cheap enough to run on every client
lifecycle event". Probing at the failure point costs nothing on the path that
succeeds, and acceptance item 4 asks only that a held lock not change *which*
condition is reported — not that the check move earlier.

`schemaAheadAtRest` is read-only and refuses to invent anything:

- if `agentdeck.sqlite3` does not exist, it returns nil — a fresh state root has
  no version to be ahead of;
- it opens with `mode=ro` (the same string `OpenReadOnly` uses at
  `internal/store/store.go:191`), creating no file, applying no migration,
  changing no permission;
- any error — missing table, unreadable file, damaged metadata — returns nil, so
  the original `ErrStateBusy` is what the caller sees. A probe that cannot read
  the version has no standing to overrule the lock.

The value it reads is safe to trust under a concurrent writer:
`schema_metadata.version` changes only inside a migration, and a read-only
connection observes committed state.

**What this reaches.** Every `openStore` caller
(`cmd/agentdeck/main.go:1394-1401`), which is every read-write command path,
plus `state migrate` — whose own comparison at `migrations.go:316-317` already
returns `*SchemaAhead` after C1, so both of its routes now agree.

`desktop snapshot` and `doctor` are unaffected here: both use `OpenReadOnly`
(`internal/desktop/desktop.go:252`, `internal/doctor/doctor.go:66`), which takes
no lock and already reaches the comparison.

## C4 — `doctor` stops claiming a complete report

**Decision.** `doctor.Report` gains an unserialized `Partial bool` field
(`json:"-"`), set on the short-circuit path at
`internal/doctor/doctor.go:66-70` and on the `state_missing` return at `:52-55`.
The `doctor` command sets the **output envelope**'s `partial` to it and adds
the stable warning `checks_skipped`.

**Ground for putting it on the envelope rather than in the payload.**
`internal/output/output.go:8-16` defines `partial` and `warnings` as envelope
keys, and `cli-design.md:1805` states that they "remain command-level state".
"Some checks did not run" is exactly command-level state. Adding a second
`partial` inside `data` would give one JSON document two fields with the same
name at different depths and no rule for which one a consumer should read.

**What has to be built.** `doctor` currently writes through `writeResult`
(`cmd/agentdeck/main.go:2799`, `:3654-3678`), which always calls
`output.New(...)` and never sets `Partial`. The existing envelope-aware writer,
`writeUsageEnvelope` (`:3684-3710`), cannot be reused: its text branch calls
`renderUsageTextWithOptions`, which does not render a doctor report. So doctor
needs its own envelope write — JSON through `output.New` with `Partial` and
`Warnings` set, text through the existing `renderDoctorText`
(`:4430-4460`) plus one added line naming the skip. This is a small, located
piece of work rather than a new mechanism, and calling it out here is what
keeps a later stage from discovering it as a surprise.

**Scope of the fix.** This makes the report *honest*, not complete. The
remaining checks genuinely cannot run: they need the `*store.Store` that
`OpenReadOnly` just failed to return. Acceptance item 5 asks only that the
payload not claim completeness it did not deliver, and reordering doctor so
that database-independent checks run after a database failure is a different
change with a different justification. It is not made here.

**Not mirrored to the desktop wire.** `desktop.HealthSnapshot`
(`internal/desktop/desktop.go:204-212`) does not gain a partial flag. Ground:
the desktop envelope is already `partial: true` under this condition — `warn`
sets it (`internal/desktop/desktop.go:284-286`) the moment `OpenReadOnly`
fails — and the surface document requested nothing here. A wire field with no
consumer is a field that will be wrong before anyone notices.

## C5 — A refused Hook delivery leaves a readable trace

This answers acceptance item 8, the one the requirements explicitly left as a
design decision.

**Decision.** When `runUsageHookEvent` (`cmd/agentdeck/main.go:2940-2955`)
fails to open the store *and the failure is `schema_ahead`*, it records the
refusal in one bounded file inside the state root, then returns `nil` exactly as
it does today. `doctor` reads that file and reports it as a check.

```
<state>/hook-refusals.json          mode 0600 (platform.FileMode, internal/platform/state.go:43)

{
  "schema_version": 1,
  "code": "schema_ahead",
  "stored": 21,
  "supported": 18,
  "first_at": "2026-08-30T14:09:11Z",
  "last_at": "2026-09-01T18:28:44Z",
  "count": 137
}
```

- **Written** by rename over a temporary file in the same directory, so a
  reader never sees a half-written record and no lock is involved — taking the
  state lock here would reintroduce exactly the `state_busy` masking C3 removes.
- **Any failure to write is swallowed**, like every other step in this handler.
  Fail-open is not weakened: the handler still writes nothing to stdout or
  stderr and still exits `0`. `cli-design.md:1243-1252` is amended to say that
  it may write this one record, because the current sentence — "unavailable
  state ... ends in success without output" — is what this topic found to be
  the wrong rule for a permanent condition.
- **Cleared** by the first read-write open that succeeds, not only by the next
  Hook delivery: `store.Open` returning without error deletes the record if one
  exists, so any write-path command clears it. The record's subject is "this
  binary cannot open this database", and the moment it can, the record is
  history. The read-only openers do **not** clear it — `doctor` and
  `desktop snapshot` are contractually read-only
  (`docs/specs/cli-design.md:2280`: doctor "never migrates, creates, chmods, or
  otherwise repairs state"), and deleting a state file is a repair.
- **Owned by one reader/writer.** Three call sites touch this file — the Hook
  handler writes it, a successful `store.Open` clears it, `doctor` reads it —
  so its encoding, its atomic write, its clearing, and its "is this record still
  live" comparison belong to one small owner that all three call, not to three
  copies of the same JSON handling. Which package holds it is an implementation
  choice for stage 8; that it is one and not three is a contract decision,
  because a second copy of the shape is how the two halves of `unknown_schema`
  drifted apart in the first place.
- **Bounded** by construction: one file, one object, fixed keys. It carries no
  session ID, no path, no client payload, so it adds nothing to the privacy
  surface `cli-design.md`'s desktop and hook sections already bound.
- **`count` is a lower bound.** Two clients delivering concurrently can both
  read the same prior count and write the same successor. A lock would fix that
  and is not worth its cost here: the number's job is to show that the span was
  not a single event, and `first_at`/`last_at` — which are monotone under the
  same race — are what actually delimit it.
- **Not part of a backup.** `internal/backup/backup.go:34-38` names the archive
  members explicitly (`manifest.json`, `agentdeck.sqlite3`, `credentials.json`,
  `sessions.sqlite3`); a new state file is not swept in.

**The `doctor` check.** A new check, placed after `checkLock`
(`internal/doctor/doctor.go:64`) and **before** the database open at `:66`,
so that it still runs on the short-circuit path — which is the only path where
it will ever have something to say:

```
{"name": "hook_deliveries", "status": "warning", "code": "hook_deliveries_dropped",
 "count": 137}
```

No `recovery_command`, for the reason in C6. Absent file, unreadable file, or
undecodable content all produce no check at all rather than a check about the
diagnostic itself.

**The check is emitted only while the record's condition still holds for this
binary**, that is, while `record.stored > store.CurrentSchemaVersion`. This is
what keeps the record from becoming the very defect this topic exists to remove.
Clearing is a write, and the only clearing trigger is a read-write open; a user
who upgrades and then stops producing deliveries — hooks turned off, client
uninstalled, tool switched — may never run one. Without this rule the record
and its warning would persist for good, `Report.add`
(`internal/doctor/doctor.go:481-489`) would keep `Problems` at 1 or more, the
menu bar would render the health-count notice by D3 forever, and C6 gives the
check no recovery command because its recovery *is* the upgrade — the upgrade
that, by itself, does not clear the file. A permanent warning a user cannot act
on is exactly the shape `requirements.md` was written against.

Reading `stored` against the running binary's `CurrentSchemaVersion` decides it
with no new state and no clock: the record already carries the number, and the
comparison is the same one C1 makes everywhere else.

**The limit this accepts, stated so a reviewer can reject it.** After the
upgrade the loss stops being *presented*. A user who never ran `doctor` while
the condition held never sees the span on a surface. Acceptance item 8 is still
met — it asks that the condition be "recoverable from some surface a person or
a later reconciliation can read", and the file remains on disk, dated and
counted, until a write-path command clears it. What is given up is a
notification after the fact, and the alternative to giving it up is a warning
that never goes away.

**Why this and not the alternatives.**

- *stderr on refusal* — not readable after the fact. Both clients discard a
  hook's stderr on a zero exit, which is the whole reason the 19-session span
  in `requirements.md` was found by reconciliation rather than by anyone seeing
  it.
- *a row in the core database* — the database is the thing that cannot be
  opened.
- *inferring it from `doctor`'s schema check alone* — `doctor` can already say
  "this binary cannot open this database". What it cannot say without a record
  is that deliveries were actually attempted and dropped, or when the span
  started. Acceptance item 8 asks for a delivery that was refused to be
  distinguishable from one that succeeded; a statement about the binary's
  capability is not that.
- *a per-delivery log* — unbounded, and it would carry session IDs. The span,
  not the enumeration, is what a later reconciliation needs.

**What it does not claim.** The record does not identify *which* deliveries were
lost, and cannot: the sessions it would name are the ones whose routes were
never written. It converts an unexplained span into a dated, counted, named
one. `docs/fixes/claude-no-route-quality.md`'s boundary stands — nothing here
reconstructs a route.

## C6 — `recovery_command` stays absent, and where the recovery lives instead

**Decision.** The `schema_ahead` check carries **no** `recovery_command`, as the
surface requested (D5). The recovery reaches the user through three channels
instead:

| Surface | Carrier |
| --- | --- |
| CLI read-write paths | The error message from `SchemaAhead.Error()`: `...; upgrade AgentDeck to open it` |
| `agentdeck doctor` text | One line rendered from the code, beside the check |
| Menu bar | App-side localized copy keyed off the code (`schemaSignalRecovery`), per D5 |

**Ground.** `recovery_command` is a command contract, not a prose slot.
`cli-design.md:2277-2280` describes it as "an explicit current-version state
command", `renderDoctorText` prints it verbatim after `recovery:`
(`cmd/agentdeck/main.go:4453-4457`), and the menu bar renders it monospaced
beside a *Copy recovery command* button (`MenuBarSurfaceView.swift:557-571`).
This condition has no command. Putting a sentence in that key would produce a
copy button that yields an untranslated English sentence in a Chinese UI, which
is the outcome D5 refuses.

`doctor`'s text output gains a recovery line derived from the code rather than
from the field. That is a rendering decision, not a contract extension: the
text renderer already makes formatting decisions the JSON does not carry (the
`count=` form at `cmd/agentdeck/main.go:4442-4444`). JSON consumers get the
stable code, whose documented meaning in `cli-design.md` names the recovery.

C5's `hook_deliveries` check also carries no `recovery_command`, for a related
but distinct reason: the upgrade that would be its recovery is also what stops
the check from being emitted at all (C5), so there is no state left for a
command to act on.

## Answers to the surface's data requirements

Each row of `ux/menubar-schema-signal.md`'s **Data requirements** table, and
each of its four open questions:

| Request | Verdict | Where |
| --- | --- | --- |
| A contractual condition predicate | **Provisioned.** `schema_ahead`, a documented stable code, present on the check whenever the condition holds | C1 |
| Both version numbers as integers on the check | **Provisioned.** `count` (stored) and `supported_count` (supported), in all three carriers | C2 |
| Envelope-level cause, so suppression is attribution rather than a hard-coded code list | **Refused.** Ground below. The surface's stated fallback applies | — |
| `recovery_command` absent for this check | **Provisioned as requested** | C6 |
| Nothing for clearing (S5) | **Confirmed.** No field. The check's absence from the next snapshot is the whole mechanism | — |
| Nothing for the widget (D8) | **Confirmed.** `WidgetDesktopSnapshotV1` is not extended | — |

### Why the envelope-level cause is refused

The request is that the snapshot state, once, that `provider_unavailable`,
`usage_unavailable`, `sessions_unavailable` and `provider_candidates_unavailable`
are consequences of this condition, so the app suppresses by attribution.

Three grounds, in order of weight:

1. **The rule is not the wire's to own.** The surface's own words for the
   fallback are that deriving it in the app "hard-codes a rule the wire owns".
   It does not. What the wire owns is the fact that a section could not be
   read; *"therefore do not also show a notice about it"* is a presentation
   decision, and D2 — which decides exactly that, and defends the trade it
   accepts — is where it belongs. Moving it into the wire would mean the wire
   deciding which notices the menu bar draws.
2. **It would need a taxonomy with one member.** An envelope-level cause has to
   define the relation "warning W is a consequence of cause C" for every future
   pair, and maintain it. This topic supplies one cause and one consumer. A
   classification contract that exists to express a single fact is a contract
   that will be wrong the first time a second cause appears.
3. **The derivation is exact, not heuristic.** The four codes are not guessed
   at: when `OpenReadOnly` fails, `internal/desktop/desktop.go:254-255` emits
   `provider_unavailable` and `usage_unavailable` unconditionally, and
   `loadSessions` (`:484`, `:494`) emits `sessions_unavailable` when the
   separate session store is also unreadable. The app matching the check code
   and suppressing that fixed set reproduces the producer's behavior exactly.

**One correction the surface should absorb at stage 7.**
`provider_candidates_unavailable` cannot occur under this condition.
`internal/desktop/desktop.go:305` emits it inside `loadProvider`
(`:294-310`), which runs only in the `else` branch after `OpenReadOnly`
**succeeded** (`:252-263`). Under
`schema_ahead` that branch is never entered. Suppressing a code that cannot
appear is harmless, so this is not a defect in D2 — but the list can lose a
member, and a reviewer of the surface should not have to rediscover why it was
there.

## Where this document departs from `requirements.md`

Stated plainly, because a later stage must not be able to satisfy the
acceptance boundary while one of these is quietly unresolved.

1. **The recovery is not carried by the existing `Recovery` field.**
   `requirements.md`'s asymmetry section reads: "For the recovery string, what
   is missing is the value and not the mechanism: `Check` already carries
   `Recovery` ... and the outdated path above proves the rendering works."
   C6 declines that mechanism for this check. The rendering it proves works is
   *command* rendering, and this condition has no command. Acceptance item 3 is
   still met — the recovery text exists, names upgrading, and never names
   `agentdeck state migrate` — but it is met in the error message, the doctor
   text line, and the app copy rather than in `recovery_command`.

2. **The topic adds a check, so `health.problems` is 2, not 1, whenever a Hook
   refusal has been recorded.** C5's `hook_deliveries` check is a `warning`, and
   `Report.add` (`internal/doctor/doctor.go:481-489`) increments `Problems` for
   any non-`ok` check. The surface's D3 makes the health-count notice appear
   when `problems > 1`, so the state the surface drew as S1 will in practice
   often render as S4 — primary notice, then "2 项检查未通过".

   This document does **not** ask the surface to suppress it. The Hook check is
   an independent fact by D2's own test: it reports that data was already lost,
   not that a section is currently unreadable, and a user looking at the panel
   while the condition holds needs to know the span happened. Its lifetime is
   bounded by C5 — the check is not emitted once this binary can open the
   database — so this changes S1 while the condition holds rather than
   permanently. S1's specimen and the D3 example both need stage 7 to absorb
   it.

3. **`schema_outdated` changes too, if C2.1 stands.** `requirements.md` scoped
   the field addition to this condition. C2.1 argues for filling it on the
   reverse condition as well, on the ground that `cli-design.md` already claims
   it. It is marked separable so review can take the other branch without
   disturbing anything else.

### What stage 7 must absorb

Stage 7 returns to `ux/menubar-schema-signal.md` with the contract settled.
Three items are waiting for it, listed here so that none of them has to be
rediscovered from prose elsewhere in this document:

1. **The prototype's code and field names, and the specimens rendered from
   them.** `prototype/src/data.js:439`, `:454`, and
   `prototype/src/Popover.jsx:841`, `:848` carry `unknown_schema` and
   `storedVersion` / `supportedVersion`. C1 replaces the value with
   `schema_ahead`; C2 names the wire keys `count` and `supported_count`, and
   these two state objects exist to stand in for that wire payload, so the
   names should follow it rather than keep a second vocabulary. After the edit,
   the surface's specimens — S1 at both widths in both languages, and
   `schemaStacked` — are re-rendered, because they were captured from the old
   state. This is the only one of the three that invalidates something already
   reviewed.
2. **`health.problems` is 2 whenever a Hook refusal has been recorded**
   (departure 2 above), so the state the surface drew as S1 renders as S4 while
   that record exists. Note the narrowing C5 now makes: once the binary can open
   the database, the check is not emitted, so this applies while the condition
   holds rather than forever.
3. **`provider_candidates_unavailable` can leave D2's suppression list**, per
   the correction under **Why the envelope-level cause is refused**. Harmless
   either way; it is listed so the surface's author does not have to re-derive
   why it was there.

## Contract edits

Each edit is located. They are one pass over `docs/specs/cli-design.md`
(currently version 28) plus one new version-history row; the file's own history
table is at `:2521` onward.

| Location | Edit |
| --- | --- |
| `:2162-2184` stable `error.code` table | Add a `schema_ahead` row: "The database schema version is newer than this binary supports." Exit `1` |
| `:2200-2220` Error-Code Compatibility | Add the narrowing row: this condition returned `runtime_error` in `v0.4.x` and `v0.5.0`, and returns `schema_ahead` from the version that ships this topic. Exit unchanged |
| `:2282` "A schema newer than the binary remains an `unknown_schema` error." | Rewrite: the condition reports `schema_ahead`, carries the stored and supported versions, and names the upgrade recovery. `unknown_schema` remains for missing or malformed schema metadata |
| `:2289-2290` quick/full schema-state matrix | Rewrite the last clause: a future schema reports one `database` check with code `schema_ahead`, `count` = stored, `supported_count` = supported, and **no** recovery command, with the recovery in the text line and the code's documented meaning |
| `:2275-2276` `schema_outdated` sentence | Make it true: either the check now carries both numbers (C2.1) or the sentence claims only the stored one |
| `:2266` Doctor section, after the sentence rewritten at `:2282` | State that a short-circuited report sets envelope `partial: true` and the warning `checks_skipped` |
| `:1847-1848` desktop snapshot health bullet | Add `supported_count` to the listed safe check keys |
| `:1243-1252` Hook handler paragraph | Amend the fail-open sentence: unavailable state still ends in success without output, except that a `schema_ahead` refusal writes the bounded `hook-refusals.json` record described in the Doctor section |
| Doctor section, new sentence | Document the `hook_deliveries` / `hook_deliveries_dropped` check, the file it reads, the two triggers that clear that file, and the rule that the check is emitted only while the recorded `stored` still exceeds this binary's supported version |
| History table at `:2521` | One new row, version 29, stating the narrowing, the additive check key, the doctor partial semantics, and the Hook refusal record |

The desktop wire fixtures under `desktop/fixtures/v1/` are producer-generated
and pinned by `TestCanonicalFixturesAreReproducibleProducerOutput`
(`desktop/fixtures/v1/README.md`), so this topic adds one:
`snapshot-schema-ahead.json`, regenerated with `AGENTDECK_UPDATE_FIXTURES=1`.
`snapshot-legacy.json` stays hand-written and unchanged — it is the proof that
`supported_count` is optional.

## What existing behavior this changes, and what it does not

**Changes.**

- **Six existing assertions, not the two `requirements.md` names.** Its Contract
  changes section names the pair pinned to today's doctor rule. C1 also moves
  what `errors.Is` matches at the two comparison sites, which reaches four more
  — two of them in `internal/store`, a package this document otherwise never
  mentions:

  | Assertion | Pins today | After C1 |
  | --- | --- | --- |
  | `internal/store/store_test.go:220-221` | `OpenReadOnly` at `CurrentSchemaVersion+1` (inserted at `:213`) returns `ErrUnknownSchema` | `ErrSchemaAhead`, with the two versions reachable through `errors.As` |
  | `internal/store/store_test.go:460` | `migrate` at `CurrentSchemaVersion+1` (inserted at `:457`) returns `ErrUnknownSchema` | same |
  | `internal/doctor/doctor_test.go:178` (`future_schema`) | check code is `store.ErrUnknownSchema.Code` at `version=99` | `schema_ahead` |
  | `internal/doctor/doctor_test.go:334` | `hasCode(report, store.ErrUnknownSchema.Code)` at `version=99` | `schema_ahead` |
  | `internal/doctor/doctor_test.go:349` | `{"future", "database", "error", "unknown_schema", "", 99, 0, false}` | `schema_ahead`, `count` 99, `supported_count` `store.CurrentSchemaVersion` |
  | `cmd/agentdeck/main_test.go:1461` | the same row | the same change |

  Two further `errors.Is(err, ErrUnknownSchema)` assertions are **unchanged**:
  `internal/store/store_test.go:1071` (`missing schema metadata`) and `:1202`
  (`expected one schema version row`). They pin the metadata-damage sites C1
  deliberately leaves alone, and they are what will prove the split is real
  rather than merely asserted — a change that turned them red would mean C1 had
  been implemented too broadly.

- **The prototype's two schema states and the row that renders them.**
  `prototype/README.md:3` declares the prototype 「产品全部界面的唯一设计真相」,
  and `ux/menubar-schema-signal.md` passed review only after being rewritten to
  index it and to carry its prototype-wins clause. It currently hardcodes the
  code C1 declines and field names C2 decides:
  `prototype/src/data.js:439` and `:454` set
  `code: "unknown_schema", storedVersion: 999, supportedVersion: 23`, and
  `prototype/src/Popover.jsx:841` and `:848` use
  `check.code === "unknown_schema"` as the predicate for both the row expansion
  and the cause/recovery pair. Left alone, the declared design truth would
  assert a code the contract stage just refused, and every specimen the surface
  passed review with was rendered from that state. This document does not edit
  the prototype — stage 7 owns it; see **What stage 7 must absorb**.

- Read-write command paths return `schema_ahead` where they returned
  `runtime_error`, and return it in place of `state_busy` when the lock is held.
- `doctor`'s envelope reports `partial: true` with `checks_skipped` on the two
  short-circuit paths.

**Does not change.**

- `CurrentSchemaVersion`, the migration sequence, and every migration's content.
  `requirements.md`'s non-goals own this.
- The `state_busy` code, its meaning, and every other condition that produces
  it. C3 changes which error one specific failed acquisition returns, not what
  `state_busy` means.
- `OpenReadOnly`'s signature, its no-lock property, and its use by `doctor` and
  `desktop snapshot`.
- Hook fail-open. Exit status, stdout, and stderr are byte-identical to today
  in every case, including this one.
- `sessions.sqlite3`, the widget projection, `presentation.isBadged`
  (`AgentDeckShared/EmbeddedHelperRunner.swift:861-863`), and desktop wire
  version 1.
- The three metadata-damage `unknown_schema` sites.

## Verification expectations

Per the L0–L4 matrix in `.agent-instructions/project-rules.md`. The
decomposition stage assigns these to anchors; they are stated here so that
`tasks.md` has something to cover rather than invent.

- **L2** for the store, doctor, and CLI contract work: it moves a persisted
  JSON contract and a stable error code, so targeted tests plus
  `scripts/run-go-test.sh ./...`. `internal/store` is inside that surface and
  not only a dependency of it — two of the six assertions listed under
  **Changes** live there, and both are `errors.Is` checks that C1 moves.
- The fixture the acceptance boundary names — a state directory whose recorded
  version exceeds `CurrentSchemaVersion` — is already the shape both existing
  `future` cases build, and the surface's own measured run used
  `schema_metadata.version = 999`.
- Item 4 needs a test that holds the state lock while the condition is present
  and asserts `schema_ahead` rather than `state_busy`. It and the item 8 set
  below are the new test shapes; everything else extends one of the six existing
  assertions enumerated under **Changes**.
- Item 8 needs four: a refused delivery writes the record and still exits `0`
  with empty streams; any successful read-write open clears it, not only a Hook
  delivery; `doctor` reports the check while `record.stored` still exceeds
  `store.CurrentSchemaVersion`; and `doctor` does **not** report it once that
  is no longer true, which is the assertion that protects against the permanent
  warning C5 rules out.
- **L1** for the app target's rendering of the new keys, per the surface's own
  verification list.
- Swift decoding of `supported_count` is covered by the new canonical fixture
  through the existing shared-fixture path, not by a hand-written payload.

## Known residuals

Carried, not fixed here, and not blocking:

- `unknown_schema` still reaches the CLI as `runtime_error` for the three
  metadata-damage sites (`internal/store/migrations.go:366`, `:413`, `:418`).
  Different condition, different recovery, no measured report behind it.
- `doctor` still runs no database-independent check after a database failure.
  C4 makes the report honest about that; making it fuller is a separate change.

## Open questions

None. The surface's four questions are answered above; the topic's three open
questions from `requirements.md` are all resolved — the stable code and the
carrier convergence in C1, the desktop cause field in the refusal above, and the
widget in D8, which this document confirms by not extending the projection.

## Approval boundary

Approval of this document authorizes it as the contract authority for this
condition: the stable code, the two version keys, the precedence rule, the
doctor partial semantics, the Hook refusal record, and the recovery carrier.
It does not authorize implementation, the `cli-design.md` edits themselves, the
`prototype/` edit named below, commit, push, or release. Stage 7 returns to
`ux/menubar-schema-signal.md` with the three items under **What stage 7 must
absorb**, and stage 8 decomposes.
