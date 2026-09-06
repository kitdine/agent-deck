---
status: active
created: 2026-08-30
updated: 2026-09-05
---

# Schema Version Signal — Requirements

Version membership is decided by a `vX-Y-Z-contract` topic's assembly list, not
here. This topic is not yet in one; the selection and its reason belong in that
topic when it is made. See `.agent-instructions/branching.md` for how a selected
topic's branch reaches a tag.

Promoted from a **measured defect in released behavior**, reported 2026-08-30
from the installed menu-bar app: the Health panel showed `database 失败`, all
four tabs carried a warning badge, and the footer read `服务商不可用`, with
nothing on any surface naming what was wrong or what to do about it.

## Problem

The core database migrates forward and never backward. When a binary opens a
database whose schema version exceeds the version that binary supports, the
condition is well-defined and permanent until the binary is upgraded. The
product detects it correctly in every path. It reports it six different ways,
none of which tells the user what happened or what to do — and the sixth does
not report it at all, dropping data instead.

### How the condition was produced

Reproduced on the reporter's machine, not synthesized. `~/.agentdeck` reached
schema `21` because development builds of `main` ran against the real state
directory while migrations `19`, `20`, and `21` landed (`8703fed` 2026-08-26,
`cf539a2` 2026-08-27, `276986e` 2026-08-29). Both installed binaries predate
them:

| Binary | Provenance | `CurrentSchemaVersion` |
| --- | --- | --- |
| `/Applications/AgentDeck.app/Contents/Helpers/agentdeck` | pre-release build of `main` at `735d010` (2026-08-24), built 2026-08-25, stamped with the then-unreleased `v0.5.0` string | 18 |
| `/usr/local/bin/agentdeck` -> `Cellar/agentdeck/0.4.1` | released `v0.4.1` / `3b709a8`, built 2026-08-13 | 18 |
| working-tree build of `276986e` | `main` as of the 2026-08-30 measurement | 21 |

The first row is not the released `v0.5.0`, and reading it as one would invert
the argument. `735d010` is an ordinary `main` commit that predates every
`v0.5.0` release candidate (`v0.5.0-rc.1` is `40d52ab`, 2026-09-01); its version
string comes from the build's `buildinfo.Version` stamp, not from a tag.
Released `v0.5.0` is `acb8384` (2026-09-04) and supports schema `23`, so it does
not meet this condition against a schema-`21` database. The third row is a
measurement baseline rather than a moving pointer: `main` is `8d283cd` at schema
`23` as of 2026-09-05.

The database is healthy. The third-row build reports `schema: ok (count=21)` and
passes all twelve `quick`-mode checks against the same file. This is a
downgrade-read condition, not corruption, and not a defect in the migrations.

The condition is reachable without any development build. A released binary
reads a database migrated by a newer released binary whenever a machine runs two
installations, a user rolls back a release, or a shared state directory is
opened by an older copy. The observation above is one instance of a general
condition, which is why the fix is a signal contract rather than a change to how
development builds are run.

### Measured behavior

Measured 2026-08-30 with the installed helper from the first row above (the
pre-release build, supports 18) against the real store (version 21), via
`--format json`:

| Path | Reported | Names the cause? | Actionable? |
| --- | --- | --- | --- |
| `doctor` | `{"name":"database","status":"error","code":"unknown_schema"}` | Code only — no on-disk version, no supported version, no recovery | No |
| `provider list` | `{"code":"runtime_error","message":"unknown_schema: database version 21 exceeds supported version 18"}` | Message is complete and exact | Code is wrong; the message is unparseable by contract |
| `desktop snapshot` | `partial:true`, `warnings:["provider_unavailable","usage_unavailable"]`, cause present only inside `health.checks` | Warnings name symptoms, not the cause | No |
| `session list` | Returns real session data | Not applicable — `sessions.sqlite3` is a separate store and is unaffected | Misleading: part of the product works |
| any read-write path while the lock is held | `{"code":"state_busy","message":"state_busy: timed out waiting for state lock"}` | No — a transient condition masks a permanent one | Actively wrong |
| `usage hook event <client>` | exit `0`, empty stdout, empty stderr, no row written | **Nothing is reported at all** | No — indistinguishable from success |

Six reports, one condition. The user sees whichever one their surface happens
to hit — and on the sixth, sees nothing.

The Hook row was added 2026-09-04, measured separately from the other five and
by a different route: it was found while triaging a span of sessions that
silently carried no provider route, not from a user-visible symptom. It belongs
in this table because it is the same condition, but its consequence differs in
kind from the other five and that difference shapes what this topic has to
decide.

`cmd/agentdeck/main.go`'s `runUsageHookEvent` returns `nil` when
`opts.openStore(ctx)` fails. That is deliberate fail-open — Hook delivery must
never stop a client from starting, resuming, or exiting — and it is correct for
a transient or malformed input. Against schema excess it is not transient: the
condition holds for every invocation until the binary is upgraded, so fail-open
turns a permanent, well-defined condition into continuous silent data loss.

Measured 2026-09-04 as a three-way control, using a current `main` build against
a throwaway state directory whose recorded version was moved and restored around
one otherwise identical Hook delivery:

| Store version | exit | stdout | stderr | route written |
| --- | --- | --- | --- | --- |
| `23` (supported) | 0 | empty | empty | **yes** |
| `999` (exceeds) | **0** | **empty** | **empty** | **no** |
| `23` again | 0 | empty | empty | **yes** |

The observed cost of that silence: **19 Codex sessions between
`2026-08-30T14:09` and `2026-09-01T18:28`** hold no provider route, including
sessions of 306, 277, 228 and 153 events. The span closes at the first session
after `rc.4` — which supports 21 — was installed on `2026-09-01T19:58`. Those
routes cannot be reconstructed: a route is an observation, and synthesising one
from a timeline is manufacturing evidence, a boundary
[`claude-no-route-quality`](../../fixes/claude-no-route-quality.md) already
established. The loss is permanent for that span.

This row does not by itself decide the fix. It states a condition the signal
contract has to answer that the other five do not: **a path whose whole purpose
is to not fail still has to be able to report that it is not functioning.**
Whether that answer is a persisted delivery record, a `doctor` check that can
see the Hook path, or something else is a design question, not a requirement.

### Four distinct defects

Source coordinates in this section and below are line numbers at `8d283cd`
(2026-09-05), each stated with the symbol that owns it so that later drift
misplaces nothing.

1. **The only path with a stable code discards the detail; the only path with
   the detail discards the code.** `OpenReadOnly`'s version check
   (`internal/store/store.go:200-202`) returns a bare `ErrUnknownSchema` from
   the read-only open, so `doctor` renders `unknown_schema` with nothing else.
   `migrate`'s version check (`internal/store/migrations.go:316-317`) wraps the
   same condition with both versions — and that path surfaces as `runtime_error`,
   because the wrapped error is not in `errorCode`'s list. The two halves of one
   usable message exist in the codebase and never meet.

   `runtime_error` here is the same class of defect
   [`cli-error-classification`](../../archive/topics/cli-error-classification/requirements.md)
   closed for not-found conditions: an undocumented fallback where a stable code
   is required. That topic is Complete with its CEv1 gates VERIFIED and its
   scope was the not-found evidence table, so this condition was never in it.
   This topic does not reopen it.

2. **A permanent condition is reported as a transient one.** Schema-version
   excess is decided before any lock matters, but the read-write paths acquire
   the state lock first, so a held lock returns `state_busy` and the real cause
   never surfaces. `state_busy` invites a retry that cannot succeed.

3. **`doctor` short-circuits and still calls the report complete.**
   `Service.Check` (`internal/doctor/doctor.go:66-70`) returns immediately after adding the `database` error, so
   the remaining checks never run — and the emitted JSON carries
   `partial:false`, asserting a complete report. Both observed doctor payloads
   show three checks each, where a healthy run shows twelve in `quick` mode and
   eighteen with `--full`.

4. **The one path that reports nothing also loses data.** The first three
   defects are all about *how* the condition is worded; this one is that it is
   never worded at all. `runUsageHookEvent` returns `nil` on a failed
   `openStore`, so every Hook delivery exits 0 with both streams empty and no
   row written. Nothing downstream records that a delivery was attempted and
   dropped, which is why the 19-session span above was found by reconciling
   sessions against routes rather than by anyone seeing an error.

   This is not the fail-open design being wrong. Fail-open is right for a
   malformed payload or a transient lock: the client must still start. It is
   wrong here only because schema excess is **permanent** — it holds for every
   subsequent invocation until the binary is upgraded — so the same rule that
   correctly swallows one bad event silently swallows every event for days. The
   distinction this topic has to draw is between a delivery that failed and a
   condition that will fail every delivery.

### The asymmetry that shows the intent

`doctor` already treats the reverse condition well. A database *behind* the
binary is a `warning` carrying both the version and the recovery command:

```go
report.add(Check{Name: "schema", Status: "warning", Code: "schema_outdated",
    Count: version, Recovery: "agentdeck state migrate"})
```

A database *ahead* of the binary produces `Check{Name: "database", Status:
"error", Code: "unknown_schema"}` — no `Count`, no `Recovery`.

For the recovery string, what is missing is the value and not the mechanism:
`Check` already carries `Recovery` (`internal/doctor/doctor.go:23-29`) and the
outdated path above proves the rendering works.

For the two version numbers the mechanism is missing too, and that is the harder
half. `Check` carries one `Count`, not two, and the `schema_outdated` path fills
it with the stored version alone (`internal/doctor/doctor.go:82`,
`Count: version`); no check emits the supported version at all. So this
asymmetry establishes the intent, not the sufficiency of the existing keys —
what has to be added is declared in Contract changes below.

## Goals

- One condition reports one way. Every path that can encounter schema-version
  excess reports a single stable, documented code, and that code is not
  `runtime_error`.
- The report carries the two numbers that make it diagnosable — the on-disk
  version and the version the running binary supports — through the documented
  contract, not only inside prose in a message string. The check payload carries
  one integer today, so this is a contract addition rather than a value fill;
  Contract changes below names the surfaces it reaches.
- The report carries a recovery the user can act on. For this condition the
  recovery is upgrading AgentDeck, which is categorically different from
  `schema_outdated`'s `agentdeck state migrate`; naming the wrong one is worse
  than naming none. `docs/specs/cli-design.md` forbids a recovery for this
  condition today, so this goal revises that rule rather than extending it —
  again, Contract changes below.
- A permanent condition is never masked by a transient one. Schema-version
  excess is reported in preference to `state_busy`.
- `doctor` does not assert completeness it did not deliver. A short-circuited
  report is marked as such.
- The desktop surfaces name the cause. A user reading the menu bar can tell that
  the app is out of date with its own database, rather than reading four
  unexplained unavailability badges. The cause *code* is already present in the
  snapshot payload, so naming it is a presentation obligation; the two version
  numbers are not present and reach the desktop only through the wire addition
  declared below — both subject to the contract stage confirming them.
- Regression coverage asserts the code, both version numbers, the recovery, and
  the precedence over `state_busy`, on a database fixture whose version exceeds
  the binary's.

## Non-goals

- **No forward compatibility.** An older binary will not gain the ability to
  read a newer database, in whole or in degraded read-only form. That requires
  declaring which parts of the schema are stable across versions, which is a
  much larger contract than this defect justifies. The binary refuses, and this
  topic is about how it refuses.
- **No downgrade migration.** No reverse migrations, no `state migrate --down`,
  no automatic rollback of a database to an older version.
- **No change to migration mechanics.** Migrations `19`–`21` are correct and are
  not revisited. `CurrentSchemaVersion` keeps its meaning.
- **No in-app updater.** Desktop update checking was withdrawn from `v0.5.0`
  deliberately (`docs/roadmap.md`, Withdrawn Candidates). Naming the recovery is
  not performing it, and this topic adds no outbound request.
- **No change to installation or packaging.** The Cask already binds the app and
  the CLI to one binary via
  `binary "#{appdir}/AgentDeck.app/Contents/Helpers/agentdeck"`, and its
  `conflicts_with` plus `preflight` already exclude a competing installation.
  That mechanism prevents an app/CLI split; it cannot prevent this condition,
  because a released binary can meet a newer database with no split involved.
  Beads `ad-etp` covered the installation question and was closed 2026-08-30.
- **No message-quality rules beyond this condition.** Other commands' messages
  are out of scope.
- **`sessions.sqlite3` is out of scope.** It is a separate store, was unaffected
  in the measurement, and carries its own versioning question. That its data
  still renders while the core store is refused is noted as a presentation
  input, not as a defect to fix here.

## Contract changes

This topic revises published contracts rather than only filling in values. Each
revision is named here so that no later stage can satisfy the acceptance
boundary while a contradicting rule stays standing.

**`docs/specs/cli-design.md` (version 28) is revised, not merely extended.** Its
Doctor section states today's rule twice, and both sentences contradict the
Goals above:

- `cli-design.md:2282` — "A schema newer than the binary remains an
  `unknown_schema` error."
- `cli-design.md:2289-2290` — the shared quick/full schema-state matrix, which
  fixes it as "a future schema reports `unknown_schema` **without a recovery
  command**".

The stable `error.code` table at `cli-design.md:2162-2184` carries no row for
this condition at all, so adding one there is a separate edit in a separate
section and does not touch either sentence. Both are rewritten in the same pass
to state the code, the two version numbers, and the upgrade recovery. Narrowing
`runtime_error` for one more condition also reaches the Error-Code Compatibility
section (`cli-design.md:2200-2220`), which already records five such narrowings
for `v0.5.0`; whether this one is a compatibility break of the same class is
stated there once `architecture.md` decides the code.

**The check payload gains a version number it cannot carry today.**
`doctor.Check` has a single `Count` (`internal/doctor/doctor.go:23-29`), so one
check can report the stored version or the supported version but not both, and
the `schema_outdated` path proves only the first. Acceptance item 2 therefore
puts a new contract field in scope, and it is not confined to the CLI:
`desktop.HealthCheck` mirrors the same five keys into the desktop snapshot
(`internal/desktop/desktop.go:214-220`), and Swift's `DesktopHealthCheckV1`
decodes them key by key
(`apps/macos/AgentDeckShared/DesktopWire.swift:941-953`). The addition is in
scope for those three surfaces plus the JSON contract in `cli-design.md`. What
it is named, whether it is one field or two, and whether it stays additive to
desktop wire version 1 are `architecture.md`'s to decide; that some addition is
required is decided here.

**The existing `future` regression expectations change with the contract.** Two
cases pin today's rule with an empty recovery and no count —
`internal/doctor/doctor_test.go:349` and `cmd/agentdeck/main_test.go:1461`, both
`{"future", "database", "error", "unknown_schema", "", 99, 0, false}`. They are
not left standing beside the new coverage acceptance item 9 requires: a build
that passes while both still assert the old expectations is a build in which the
contract was not actually revised.

## Acceptance boundary

This topic is complete when, on a state directory whose core schema version
exceeds the running binary's supported version:

1. `doctor`, the read-write command paths, and `desktop snapshot` each report
   the same stable code for the condition, and no path reports it as
   `runtime_error`.
2. The on-disk version and the supported version are both readable from the
   documented JSON contract without parsing a message string, through the
   contract addition declared above.
3. The recovery text names upgrading AgentDeck, and never
   `agentdeck state migrate`.
4. A held state lock does not change which condition is reported.
5. `doctor`'s payload does not claim to be complete when checks were skipped.
6. The menu-bar surfaces attribute their unavailability to this condition rather
   than presenting bare unavailability, to whatever extent the contract stage
   provisions.
7. `docs/specs/cli-design.md` carries this condition in its stable `error.code`
   table, **and** its Doctor section no longer states the no-recovery rule.
   Documenting the code beside the table does not satisfy this item on its own:
   the two sentences named in Contract changes are revised in the same pass.
8. A Hook delivery refused for this condition is distinguishable, after the
   fact, from one that succeeded. Hook delivery stays fail-open — the client
   must still start, resume, and exit — so this is not satisfied by making the
   Hook exit non-zero. What it requires is that the condition stop being
   invisible: a permanent, binary-wide condition must be recoverable from some
   surface a person or a later reconciliation can read, rather than leaving an
   unexplained span of route-less sessions. Which surface carries it is a design
   decision, not a requirement.
9. Regression tests cover items 1 through 5 and item 8 against a fixture
   database whose version exceeds the binary's, and the two existing `future`
   cases named in Contract changes assert the new expectations rather than the
   old ones.

Items 1, 2, 3, 4, 5, 7, 8, and 9 are decided within this topic. Item 6 depends
on the surface and contract stages and may be narrowed there with a stated
ground; it may not be dropped silently.

## Surfaces

This topic changes user-visible presentation on the menu-bar app, so it declares
one surface document:

- `ux/menubar-schema-signal.md` — how the menu-bar Health panel, the four tab
  badges, and the footer present this condition.

The CLI's rendering is governed by the error-code contract and
`docs/specs/cli-design.md` rather than by a surface document, matching how
[`cli-error-classification`](../../archive/topics/cli-error-classification/requirements.md)
handled the same question.

## Open questions

Recorded rather than silently decided; each is resolved by the stage named.

- Whether the stable code is a new value or a documented promotion of the
  existing `unknown_schema` string, and whether the read-only and read-write
  paths converge on one carrier. Resolved by `architecture.md`.
- Whether `desktop snapshot` gains an explicit cause field or the surfaces
  derive the cause from `health.checks`, which already carries it. Resolved by
  `ux/menubar-schema-signal.md` requesting it and `architecture.md`
  provisioning or vetoing it.
- Whether the widget extension, which reads the same snapshot, presents the
  condition or stays silent. Resolved by `ux/menubar-schema-signal.md`.
