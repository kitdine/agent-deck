---
status: active
created: 2026-08-30
updated: 2026-09-07
---

# Schema Version Signal — Tasks

This file is the only status authority for this topic.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar-schema-signal.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

The document set is unchanged. Requirements, architecture, and the final UX
surface have passed review. This decomposition passed Round 1 review on 2026-09-07; see
`reviews/tasks.md` for evidence and the completion gate. Implementation still
requires its own development-stage authorization.

Why each row exists, against the review question that justifies it:

- `requirements.md` — one boundary question for one coherent behavior change.
- `ux/menubar-schema-signal.md` — the menu-bar Health panel, the four tab
  badges, and the footer are user-visible states with no presentation rule for
  this condition today. `requirements.md` names this surface by path, so the
  audit requires the row.
- `architecture.md` — the error carrier, the stable code, the two version
  fields, and the precedence over `state_busy` are contracts, and "are the
  contracts specified" is one question.
- `tasks.md` — one decomposition.

No second `ux/` row is declared. The CLI renders this condition through the
error-code contract and `docs/specs/cli-design.md`, not through a surface
document, matching how `cli-error-classification` handled the same question. The
widget extension reads the same snapshot as the menu bar and its presentation is
decided inside `ux/menubar-schema-signal.md`; if that document concludes the
widget needs its own state set, this matrix gains a row and returns here for
re-ratification. It concluded on 2026-09-06 that it does not: decision D8 keeps
the widget's existing `Data unavailable` state, on the ground that
`WidgetDesktopSnapshotV1` decodes no `health` at all, so presenting the cause
would mean extending the widget projection. No row is added.

## Design basis

Inputs are sufficient for decomposition: [requirements](requirements.md),
[architecture](architecture.md) C1–C6, and the reviewed final
[menu-bar surface](ux/menubar-schema-signal.md) D1–D9. The final surface and its
prototype were delivered in `66924c4`; the architecture passed Round 2 and the
surface passed Round 6. The review history remains in `reviews/`.

This plan covers acceptance items 1–9. It adds no migration, schema-version bump,
downgrade reader, updater, release assignment, session-index migration, or widget
state. The existing prototype is the presentation reference, not work to rebuild.
The source baseline for this decomposition is HEAD `66924c4`.

The final UX owns notice suppression: retain `sessions_unavailable`, suppress
only `provider_unavailable` and `usage_unavailable` plus the partial notice.
Architecture's older explanation is already carried by
`ad-bug-arch-sessions-unavailable-not-a-consequence`; it is not an implementation
instruction to suppress an independent warning. This plan does not close that
carrier or rewrite its architecture record.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `core-schema-contract` | [ ] | [ ] |
| 2. `hook-refusal-lifecycle` | [ ] | [ ] |
| 3. `desktop-schema-wire` | [ ] | [ ] |
| 4. `menubar-schema-presentation` | [ ] | [ ] |
| 5. `schema-signal-acceptance` | [ ] | [ ] |
| 6. `contract-reconciliation` | [ ] | [ ] |

Execute in order. Each task is independently reviewable and must leave its
selected checks passing; later tasks are not excuses for a failing intermediate
state. Shared files are edited sequentially, not assigned to competing tasks.
Implementation dispatch tasks are created only after this document passes review.
Each task still needs the user's development-stage authorization.

### 1. `core-schema-contract`

**Result:** core-store refusal, CLI classification, and doctor report agree on
`schema_ahead`, preserve the version pair, and distinguish incomplete checks.
C1–C4 and C6 travel together because changing the sentinel alone would break
existing doctor/CLI expectations and leave the signal split across consumers.

**Files:** `internal/store/store.go`, `internal/store/migrations.go`,
`internal/store/store_test.go`; `internal/doctor/doctor.go`,
`internal/doctor/doctor_test.go`; `cmd/agentdeck/main.go`,
`cmd/agentdeck/main_test.go`; `internal/desktop/desktop.go` and
`internal/desktop/desktop_test.go` for the direct check-field projection.
Focused additional test files may live in these same packages.

- Add `ErrSchemaAhead` and `SchemaAhead{Stored, Supported}`; use them at both
  future-version comparisons. Preserve `errors.Is` and `errors.As` through
  wrapping. Leave all metadata-damage `ErrUnknownSchema` sites unchanged.
- Probe the at-rest version only after acquisition returns `ErrStateBusy`,
  using read-only SQLite and no migration or permission change. If the probe
  cannot establish a future version, preserve the original lock error. Keep
  successful opens free of the additional probe; cancellation stays bounded.
- Map CLI `errorCode` and doctor `databaseCode`; ordinary CLI errors retain
  exit 1 and the existing code/message envelope, with upgrade prose in the
  typed error message. Do not invent extra error-envelope version keys.
- Add optional/omitted `supported_count` to the doctor check and Go desktop
  mirror, copying it through `healthSnapshot`. Populate both schema directions
  per C2.1; preserve zero/absent semantics for unrelated checks.
- Add unserialized doctor `Report.Partial` for missing-state and failed-open
  short circuits. Give doctor a command-specific envelope write path: JSON
  `partial: true`, `warnings: [checks_skipped]`; text names skipped checks and
  the upgrade recovery. Do not route doctor through the usage text renderer.
  Neither schema-ahead nor Hook recovery prose belongs in `recovery_command`.
- Update all six future-schema assertions enumerated in architecture, plus
  both older-schema expectations. Keep the two metadata-damage assertions and
  reverse-condition migration command intact. Cover quick/full output,
  missing state, healthy complete reports, JSON fields and text recovery.

**Verification: L3** (L2 contract work plus the affected lock/concurrency path).
Run targeted store/doctor/CLI/desktop tests, then the Go suite. Run the store
race tests for the lock/probe change. A held-lock future database must report
`schema_ahead`; supported, missing and malformed stores must retain the correct
lock/error behavior. Prove read-only probing did not modify the database or
create sidecars. Use synthetic state only.

### 2. `hook-refusal-lifecycle`

**Depends on:** Task 1. **Result:** a refused Hook delivery stays fail-open but
leaves C5's bounded record, and doctor presents it only for the specified lifetime.

**Files:** new `internal/hookrefusal/record.go` and `record_test.go`;
`internal/store/store.go` / `store_test.go`; `cmd/agentdeck/main.go` /
`main_test.go`; `internal/doctor/doctor.go` / `doctor_test.go`;
`internal/backup/backup_test.go` only for archive exclusion coverage.

The package `internal/hookrefusal` owns the record shape, read, atomic replacement,
clear, and live-condition comparison. It takes the state root and version values;
it does not import `store`, `doctor`, `usage`, or the CLI. All three callers use
that owner, avoiding an import cycle and duplicate JSON handling.

- Write only on `errors.As` identifying a schema-ahead store-open refusal in
  `runUsageHookEvent`. Use C5's fixed keys and one file, mode 0600, a temporary
  sibling plus rename; clean temporary files on failure. Do not take the state
  lock, store payloads/session identifiers, or add stdout/stderr output.
- Preserve bounded, best-effort behavior. Malformed, unreadable, missing or
  incompatible diagnostic records do not break Hook delivery or doctor.
  Concurrent updates retain C5's approximate-count limitation; do not promise
  an exact delivery ledger or reconstruct routes.
- Clear only after a read-write open actually succeeds, including final open
  failure handling such as lock release. A failed open must not clear it;
  diagnostic deletion failure must not make an otherwise usable open fail.
  Read-only callers never clear, chmod or repair the diagnostic.
- Place the Hook check after the lock check and before doctor opens the core
  database. Emit `hook_deliveries_dropped` with count only while recorded
  `stored` exceeds this binary's supported version; upgrade suppresses it
  without requiring deletion. It contributes to `health.problems` normally.
- Keep the record out of backup archives and restore expectations.

**Verification: L3.** Targeted owner/store/doctor/CLI tests plus the Go suite;
race checks on owner and touched store/doctor paths. Exercise both client Hook
routes with supported → future → supported fixtures, held-lock refusal,
write failure, corrupt/absent record, repeated writes, concurrent replacement,
successful non-Hook clearing, failed-open retention, read-only immutability,
and upgrade-without-write suppression. Assert exit 0, empty streams and no
route on refusal, a bounded private record, and a real route on success.

### 3. `desktop-schema-wire`

**Depends on:** Tasks 1–2. **Result:** producer-generated schema-ahead data reaches
Swift while legacy v1 payloads remain decodable.

**Files:** `internal/desktop/fixtures_test.go`, `internal/desktop/desktop_test.go`;
new `desktop/fixtures/v1/snapshot-schema-ahead.json`, fixture `README.md`;
`apps/macos/AgentDeckShared/DesktopWire.swift`;
`apps/macos/AgentDeckTests/DesktopWireTests.swift`;
`apps/macos/AgentDeckVerification/` verifier entry point and
`scripts/test-macos-app.sh` only to register the new fixture in fallback checks;
`desktop/fixtures/v1/verify.swift` for its standalone decoder mirror.

- Decode `supported_count` as optional `Int`; do not raise desktop wire v1 or
  extend `WidgetDesktopSnapshotV1`.
- Add a deterministic fixture builder using the actual Go producer against a
  future-schema database and a synthetic Hook refusal. Register the new fixture
  in the producer reproducibility test and both Swift verification routes.
- Assert schema code, both numbers, Hook code/count, section availability,
  independent session warning behavior, and privacy-bounded keys. Keep
  `snapshot-legacy.json` byte-identical and verify the new field is nil there.
- Regenerate only producer fixtures whose output actually changes. A hand-edited
  schema payload cannot substitute for the producer fixture.

**Verification: L2.** Targeted producer/fixture tests and Go suite, followed by
shared Swift decoding and the canonical macOS test entry point. Regenerate with
`AGENTDECK_UPDATE_FIXTURES=1 scripts/run-go-test.sh ./internal/desktop
-run TestCanonicalFixturesAreReproducibleProducerOutput`; re-run the fixture
check without the update flag to establish reproducibility. The standalone
verifier must also accept all five fixture paths, including the unchanged legacy
payload. CLT fallback is decoding evidence only, never native UI acceptance.

### 4. `menubar-schema-presentation`

**Depends on:** Task 3. **Result:** implement the approved D1–D9 surface without
changing widget behavior, geometry, refresh-state semantics or recovery commands.

**Files:** `apps/macos/AgentDeckApp/MenuBarViewModel.swift`,
`MenuBarSurfaceView.swift`, `MenuBarPanelViews.swift`, `DesktopCopy.swift`,
`Localizable.xcstrings`; `apps/macos/AgentDeckAppTests/AppTestFixtures.swift`,
`MenuBarViewModelTests.swift`, `MenuBarChromeTests.swift`, `DesktopCopyTests.swift`.
All unqualified paths in this paragraph belong to the indicated App or AppTests
directory. The existing item controller consumes model badge/label values and
should not need a new glyph or a new Shared predicate.

- Implement the schema-code predicate, notice precedence, exact two-code
  suppression, independent-warning retention, and health-count threshold.
- Project `code`, `count`, `supported_count`; choose D4/D9 prose by stable code,
  not check name or localized status. Preserve missing-number semantics.
- Broaden each Health row's disclosure predicate to cause/recovery prose or a
  command. Render prose proportionally without copy buttons; preserve the
  command branch, expand/collapse control and default expansion. Extend the
  accessibility grouping with the order and operability specified by the UX.
- Add eight localized keys to `DesktopCopy.allKeys` and both shipped languages;
  enforce the existing no-update-check rule. Apply attributed body/footer/
  provider-popover copy, App-level badge and accessible label. Preserve tab
  marks and Shared `presentation.isBadged`.
- Cover no-refusal and refusal-recorded fixtures, each with and without the
  independent session warning; offline/failing precedence, aged state, reset
  after the next healthy snapshot, and `schema_outdated`'s copyable command.

**Verification: L1.** Affected view-model, chrome and copy tests through
`env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make test-macos-app`.
Validate native expanded/collapsed prose, no copy affordance, and both version
values. Compare with the existing prototype; do not edit it to make an
implementation discrepancy disappear. Native layout/accessibility acceptance
is completed in Task 5, not inferred from model tests.

### 5. `schema-signal-acceptance`

**Depends on:** Tasks 1–4 and their reviews. **Result:** state-bound evidence for
the complete producer-to-menu-bar path and the manual acceptance boundary.

**Files:** focused end-to-end regressions in `cmd/agentdeck/` and
`apps/macos/AgentDeckAppTests/` where not already covered; findings/evidence in
`reviews/schema-signal-acceptance.md` when its review occurs, plus this matrix.
No production changes are preauthorized by this acceptance task; a discovered
defect returns to its owning task for scoped repair.

Use a built test binary and isolated state root to exercise doctor quick/full,
a representative read-write command, desktop snapshot, and both Hook clients.
Verify codes, version pair, upgrade copy, held-lock priority, partial report,
refusal persistence, read-only preservation and successful-open clearing.
Inspect the actual generated JSON and stored synthetic record/route state.

On a fixture-driven native app, check 280/420 pt in `en` and `zh-Hans`, real SF
metrics, Dynamic Type, VoiceOver order/disclosure operation, footer fallback,
badge with both menu-bar value modes including Icon only, notice-to-Health
navigation and healthy-state reset. Use `AGENTDECK_TEST_WIDTH` and
`AGENTDECK_TEST_LOCALE`; never run a development migration on the real state root.

**Verification: L3 targeted acceptance.** Reuse exact-state package/suite evidence
from Tasks 1–4; run only missing cross-path and native checks. Record fixture,
binary/tree identity, toolchain, command and observation for each acceptance item.
No release-verify, installation, real authentication or user database mutation.
If native acceptance is unavailable, leave this task and its evidence boundary
open; a successful Foundation fallback is not a waiver.

### 6. `contract-reconciliation`

**Depends on:** Tasks 1–5 passing review with their required evidence.
**Result:** published contract text agrees with the implemented topic.

**Files:** `docs/specs/cli-design.md`, `docs/specs/cli-manual.md` only for affected
existing doctor/Hook examples, `docs/README.md` only for its spec-version pointer;
this topic's `tasks.md`, `reviews/contract-reconciliation.md` when reviewed, and
the topic row in `docs/status.md`.

Apply every architecture **Contract edits** row in one pass: stable error code,
compatibility narrowing, both formerly contradictory future-schema statements,
older-schema version pair, partial/checks-skipped semantics, desktop safe keys,
Hook fail-open plus bounded record, its clearing and presentation lifetime, and
the specification history entry (architecture plans revision 29 from baseline
28). Do not assign a product release version. If another authorized topic changes
the spec baseline first, reconcile the revision with that owner before editing;
do not overwrite or silently renumber its work.

State C5's clearing as successful read-write open and its presentation stop as
the recorded version no longer exceeding support; these are not two deletions.
Do not migrate the architecture's obsolete suppression explanation into the
stable contract. Reconcile only implemented guarantees, without changing the
reviewed design or absorbing the separately carried architecture finding.

**Verification: L0.** Topic/document links, whitespace and diff checks; audit the
contract edits against implemented behavior and the acceptance evidence map.
Do not rerun unchanged product suites for a documentation closure. Query each
newly crossed task/topic evidence boundary during its authorized closure; topic
completion, archive/version assembly and delivery follow their own authorities.

## Acceptance coverage

| Requirements item | Owning task(s) | Evidence boundary |
| --- | --- | --- |
| 1. One stable code | 1, 3, 5 | Store/CLI/doctor classification and generated desktop payload |
| 2. Both JSON numbers | 1, 3, 5 | Doctor plus desktop wire, Swift decoding, legacy compatibility |
| 3. Upgrade recovery | 1, 4, 5 | CLI/doctor text and localized prose; no recovery command |
| 4. Priority over lock | 1, 2, 5 | Held-lock fixture and unchanged fallback conditions |
| 5. Honest partial report | 1, 5 | Quick/full JSON and text short-circuit tests |
| 6. Menu-bar attribution | 3, 4, 5 | D1–D9 automated and native acceptance; D8 widget exclusion |
| 7. Stable contract reconciliation | 6 | All named contract locations, not just the error-code table |
| 8. Visible fail-open refusal | 2, 3, 4, 5 | Record lifecycle, doctor count and surface presentation |
| 9. Regressions including old expectations | 1–5 | Six future assertions, two older cases, metadata controls, held lock and Hook cases |

## Verification and workflow boundaries

Go commands use `scripts/run-go-test.sh`; set
`GOCACHE=/private/tmp/agent-deck-go-build`. L2/L3 Go tasks run the full Go suite
once at their final relevant state, not once per edit. Run only affected race
checks for L3. Every task also runs relevant L0 checks. Evidence may be reused
across review stages only with a compatible exact content state; an unrelated
HEAD change alone does not imply re-running everything.

Each Task has a corresponding `<topic>:<task-anchor>` evidence scope; actual
WorkUnits and atomic criteria are resolved under Evidence before implementation.
This design does not pre-create implementation dispatch or declare those gates
verified. The document review does not check any implementation Dev or Review cell.
The six implementation tasks remain pending; the next task is
`core-schema-contract` under its own development-stage authorization.
