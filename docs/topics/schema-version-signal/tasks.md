---
status: active
created: 2026-08-30
updated: 2026-09-08
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
`reviews/tasks.md` for evidence and the completion gate. Task 1 passed re-review
with its required evidence gate VERIFIED. See [its review record](reviews/core-schema-contract.md) for the Task checkpoint.
Task 2 passed review with its required evidence gate VERIFIED; see [its review record](reviews/hook-refusal-lifecycle.md) for the Task checkpoint. Task 3 passed review with its required evidence gate VERIFIED; see [its review record](reviews/desktop-schema-wire.md). Task 4 passed review with its required evidence gate VERIFIED; see [its review record](reviews/menubar-schema-presentation.md); Tasks 5–6 still require their own development-stage authorization.

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
| 1. `core-schema-contract` | [x] | [x] |
| 2. `hook-refusal-lifecycle` | [x] | [x] |
| 3. `desktop-schema-wire` | [x] | [x] |
| 4. `menubar-schema-presentation` | [x] | [x] |
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

#### Task 2 implementation handoff — 2026-09-08

- Implementer: Codex; workspace `agent-deck.schema-version-signal`, branch
  `feature/schema-version-signal`. Task 1 is delivered in `6cc1d6f`.
- `internal/hookrefusal` owns the seven fixed keys, 2 KiB read bound, 0600
  temporary-sibling replacement, live-version comparison and deletion. It adds
  no client payload, path or session ID to the record. Concurrent increments
  remain approximate; there is no state-lock acquisition or exact ledger claim.
- The Hook writes only after a typed schema-ahead open refusal and swallows
  diagnostic failures. Core open clears only after successful final lock
  release. Doctor reads before database open, suppresses upgraded history
  without mutation and emits no diagnostic-about-diagnostic warning.
- Tests cover both clients through the command entry point with empty stdout
  and stderr, supported/future/supported transitions, held-lock future refusal,
  supported state_busy without a refusal record, no new refused route, real
  successful routes, write failure, sequential/concurrent replacement, corrupt
  and absent records, read-only retention, failed lock-release retention,
  non-Hook successful clearing, deletion failure, doctor quick/full upgrade
  suppression, and encrypted backup member exclusion. These use synthetic
  fixtures, not installed clients, real session Hooks or user databases.
- Final product state: HEAD `6cc1d6f56428ea66b00a3a46944b5c47d0556044`,
  SHA-256 `d37d73a60f66e0dfa763cec9292522fa7820bf821c76081edad7c0468336ff35`.
  Recipe: `git diff --binary` for the seven changed tracked Go files, in sorted
  path order, followed by `git diff --no-index --binary -- /dev/null <path>`
  for `internal/hookrefusal/record.go`, then `record_test.go`; hash concatenated
  stdout. Status documents are excluded from this product/test fingerprint.
- Verification passed: targeted owner/store/doctor/CLI/backup tests, then
  `scripts/run-go-test.sh ./...`,
  `scripts/run-go-test.sh -race ./internal/hookrefusal ./internal/store ./internal/doctor`,
  and `make vet`. The final full suite includes the added supported-lock and
  backup-fixture assertions. Go 1.27.1 darwin/amd64, vendored dependencies,
  `GOCACHE=/private/tmp/agent-deck-go-build`; no dependency changes.
  Logs are retained in the local TMPDIR: targeted `agentdeck-go-test.X14O9U`,
  full `agentdeck-go-test.YNms5w`, race `agentdeck-go-test.AnuDWH`.
- CEv1 WorkUnit:
  `urn:ce:agent-deck:work-unit:schema-version-signal-hook-refusal-lifecycle`;
  target `urn:ce:agent-deck:state:implement:hook-refusal-lifecycle:6hf-2V_qpB0yYQHf`.
  Five required criteria cover the record, Hook, clearing, doctor/backup and L3
  verification. Fixed gate query confirms VERIFIED (5/5), with no missing,
  invalidated or unresolved evidence. Six state/evidence nodes and ten relations
  were confirmed; all ten relationship preflights passed. No review PASS or
  delivery is claimed. Review remains unchecked.
- Documentation checks passed: `make check-whitespace`,
  `bash scripts/check-topic-docs.sh`, and topic/canonical `git diff --check`.
  The full Go log SHA-256 is
  `775b846e73b05ae5be85e59819f9115584240d9e841df911398d6fe209a12619`;
  race log SHA-256 is
  `64189197b5a47b5e372e8d19f0c655b6f791118d873a8847df4dff451bb8a65d`.

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

#### Task 3 implementation handoff — 2026-09-08

- Codex implemented the producer-generated schema-ahead fixture, optional Swift
  `supportedCount`, Go privacy/availability checks, XCTest assertions, and both
  Swift fixture entry points. Workspace `agent-deck.schema-version-signal`,
  branch `feature/schema-version-signal`, HEAD
  `4285979ecec296ea8123090f2085161ef700e89d`.
- The fixture carries schema 99/supported 23 and two synthetic Hook refusals.
  Independent session-index availability is tested both ways. Existing producer
  fixtures remain unchanged; legacy SHA-256 remains
  `b4fc86e306b3ce557a744f4da2416faeb3e74b0fa60b95a354c7f116765400f2`.
  Legacy has an empty health-check list; a separate old-shape check without
  `supported_count` proves nil decoding without changing legacy bytes.
- Passed: fixture generation, reproducibility without the update flag, targeted
  producer/fixture/privacy tests, full vendored Go suite, and the standalone
  Swift verifier over all five fixtures. Go logs in local TMPDIR:
  `agentdeck-go-test.OpE7n7` (generation), `agentdeck-go-test.5LGGwu` (targeted),
  `agentdeck-go-test.gOXap9` (full, SHA-256
  `65d8e6925cb5b0ca861c9f64cbfe1b698c91e65fd252d2917e47b4e42ccb5733`).
  The final changes after the Go suite only correct Swift verifier assertions;
  unchanged Go evidence is reused. Swift 6.3.3, x86_64-apple-macosx26.0.
- Harness diagnosis: the standalone decoder mirror already referred to nonexistent
  `rhythm.cells` at HEAD. Its fixed-bound assertions now check the four serialized
  arrays directly, preserving 168-cell/empty-array checks; all five fixtures pass.
  The newly introduced nonempty-legacy assertion was corrected against the
  unchanged legacy payload, and missing-field decoding remains explicitly tested.
- **Initial blocking prerequisite (resolved below):** `bash scripts/test-macos-app.sh` builds the shared
  decoder and runs the CLT fallback, passes the new schema and old-shape decoding
  assertions, then fails at the pre-existing helper assertion:
  `expected two index refreshes followed by one snapshot read`.
  `EmbeddedHelperRunner.snapshot` actually performs one
  `desktop refresh-indexes` request and one `desktop snapshot ... --stream`
  request. The verifier still expects `usage scan`, `session scan`, then a
  non-stream snapshot. This is a test-harness mismatch, not evidence of a
  schema-wire failure. Updating those helper assertions exceeds Task 3's stated
  permission to register the new fixture in that entry point. Separate scoped
  repair authorization is required before rerunning this mandatory gate.
  Final failure log: `/private/tmp/agentdeck-desktop-schema-wire-macos.log`.
  CLT fallback establishes decoding only; XCTest/native UI acceptance was not run.
- Current content fingerprint:
  `47a97341a0680f75f8b23ea9a12c4f8b3ef05902a5f791549c5d707f0bab408f`.
  Recipe: SHA-256 of `head=<HEAD>` followed by sorted `;<path>=<git hash-object>`
  entries for the nine Task 3 changed files (eight tracked plus the new JSON),
  excluding tasks.md/status.md. This includes both Swift verifier corrections.
- At the initial blocked handoff, Task remained `in_progress`, Dev/Review unchecked; no completion, review PASS,
  commit or push. CEv1 target:
  `urn:ce:agent-deck:state:implement:desktop-schema-wire:F6Y6pjgFEZSajayT`.
  Producer and Swift compatibility evidence are available; the mandatory
  canonical-macOS verification criterion is blocked by the prerequisite above.
  Fixed gate query confirms BLOCKED: two criteria pass, verification remains
  blocked; four nodes and six relations were confirmed, six preflights passed.
  Whitespace, topic-document and both workspace diff checks pass.

#### Task 3 completion after supplemental authorization — 2026-09-08

- The user explicitly authorized synchronizing the verifier's obsolete helper
  assertions. They now require exactly `desktop refresh-indexes`, followed by
  `desktop snapshot --wire-version 1 --recent-limit 5 --stream`, with the existing
  flags and embedded-helper path assertion preserved. No runner behavior changed.
- `bash scripts/test-macos-app.sh` now passes: shared Swift compilation, five
  fixture checks and helper boundaries. Final log:
  `/private/tmp/agentdeck-desktop-schema-wire-macos-final.log`.
  This is CLT fallback decoding/helper evidence; native XCTest/UI was not run.
  SwiftPM emitted user-cache warnings but completed successfully.
- Reused the preceding targeted/full Go and standalone five-fixture Swift
  results: their producer, tests, fixtures, shared decoder, standalone verifier,
  dependencies and toolchain are unchanged by this helper-assertion-only update.
- Final fingerprint `9b305fcd70ff0395b2f94bf6bab543e5644ac24454aa26d3a876f9fcf88156a5`,
  using the same nine-file recipe and HEAD as above. The old blocked state remains
  historical evidence, not the final candidate's gate result. Final target:
  `urn:ce:agent-deck:state:implement:desktop-schema-wire:lfplhKUqIpF5bKrK`.
- Implementation is ready for review; Dev checked and Review unchecked. No
  review verdict, commit or push is implied. The final gate and handoff are
  synchronized against this content state. Fixed CEv1 gate: VERIFIED (3/3),
  no missing/invalidated/unresolved evidence. Four nodes and eight relations
  were confirmed, including two explicit reuse links; all relation preflights
  passed. Final macOS log SHA-256:
  `80b5f27adca4fa700db94f3df29509b4c70b05007e1fe63c7807df07f8ec6bcb`.

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

#### Task 4 implementation handoff — 2026-09-08

- Implementer: Codex. Workspace `agent-deck.schema-version-signal`, branch
  `feature/schema-version-signal`; Task 3 delivered at `64956c6`.
- App-only schema predicate now drives the primary notice, two-code suppression,
  health-count threshold, attributed unavailable bodies/footer/provider popover,
  badge and accessible label. Offline/failing precedence, aged freshness,
  independent warnings and normal-snapshot reset remain explicit model tests.
  Shared refresh semantics, Widget projection, tab marks and geometry are unchanged.
- Health rows retain optional stable code and version/count fields. Schema cause
  and recovery are proportional caption prose; Hook refusals have count prose
  only. Missing numbers remain absent. Disclosure defaults open and exposes a
  native accessible DisclosureGroup representation whose label combines name,
  status and visible prose without duplicate speech; command copying remains a
  separate operable branch for schema_outdated and other command recoveries.
- Eight keys match the approved UX/prototype bilingual copy and join allKeys.
  Both-language catalog tests and the existing no-update-check rule pass.
- Verification: full Xcode 26.4 native build and test entry passed, 128 tests:
  Shared 40, App 66, Widget 22. Command:
  `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer AGENTDECK_TEST_LOCALE=en TEST_RUNNER_AGENTDECK_TEST_LOCALE=en make test-macos-app`.
  Controlled elevated execution was needed because nested sandbox-exec blocked
  Swift Observation macros. The initial source compile error in the disclosure
  branch was corrected before the final candidate.
- The first native suite in the system Chinese locale passed every new schema
  test but failed the pre-existing English-literal assertion in
  `testProviderWithMultipleReadyTargetsUsesOneRowAndASecondLevel` (wrapper/direct
  versus 包装器/直连). No unrelated assertion or product language policy was
  changed; the full final run pins its test locale to English. This is not a
  claim that the entire suite passes under arbitrary system languages.
- Native NSHostingView renderings cover expanded/collapsed states in both
  languages. Inspected `/private/tmp/agentdeck-schema-presentation/health-*.png`:
  expanded rows show 99 and 23, cause and upgrade prose, no copy button; collapsed
  rows retain just status and disclosure. Compared with prototype/src/Popover.jsx
  HealthDetail and i18n.js plus the approved Health specimens. This bounded row
  rendering is not the Task 5 full-surface layout/VoiceOver acceptance.
- Final HEAD `64956c603b9b9802b18c1ee9a728344baf161bbd`, fingerprint
  `cbfcf5df79bf3dd3e4ae169e2c3eb4c1a56504a8ea94b2673abcfcbf83f14913`.
  Recipe: SHA-256 of `head=<HEAD>` plus sorted `;<path>=<git hash-object>` for
  the nine App/AppTests files named in this task, excluding status documents.
  Final log `/private/tmp/agentdeck-menubar-schema-native-en.log`, SHA-256
  `e22f34d17abccf09fff247057d1831e0259dc070fb480bb6d5f91c3ade24aecd`.
  Native xcresult: `apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.09.08_06-47-21--0700.xcresult`.
- CEv1 WorkUnit
  `urn:ce:agent-deck:work-unit:schema-version-signal-menubar-schema-presentation`,
  target `urn:ce:agent-deck:state:implement:menubar-schema-presentation:di7gVes8KvamLBPG`.
  Four criteria cover notice policy, Health disclosure, copy/chrome and native
  checks. Implementation is ready for review; Review remains unchecked. No
  commit, push or Task 5 acceptance is implied. Fixed gate query confirms
  VERIFIED (4/4), with no missing, invalidated or unresolved evidence. Five
  state/evidence nodes and eight relations were confirmed; all preflights passed.
  Whitespace, topic-document and both workspace diff checks passed.

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
Tasks 1–4 have passed review and their evidence gates; two implementation tasks remain.
The next task is `schema-signal-acceptance` under its own development-stage authorization.
