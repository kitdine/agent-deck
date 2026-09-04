---
status: historical
created: 2026-08-06
updated: 2026-08-20
retired: 2026-09-01
---

# Native macOS Desktop App — Tasks

This file is the only status authority for this topic.

## Task breakdown

### 1. `desktop-wire-contract`

- Finalize the `v0.4.0` session DTO dependency and define the versioned desktop
  snapshot request/response contract.
- Keep Go aggregation, authorization, privacy filtering, partial results, and
  warnings authoritative.
- Add Go fixtures and Swift decoding fixtures from the same canonical examples.
- Preserve the no-network boundary this command established: the snapshot reaches
  no network, and neither does any other desktop surface. The desktop update check
  is withdrawn from this version, so there is no connectivity behavior to document
  — what a later task must not do is add the first outbound request. This bullet
  replaces a delivered instruction to document update-check connectivity "when
  implementation makes it real"; it never became real. Task 1's implementation and
  its Review PASS are untouched by the correction.
- Verification level: L2 because this adds a stable JSON/exit-code contract.

### 2. `macos-app-foundation`

- Contract: [`architecture.md`](architecture.md#foundation-runtime) — approved.
- Add the Xcode project, macOS 26 targets, bundle identifiers, entitlements,
  shared Swift layer, helper runner, App Group snapshot store, OSLog policy, and
  unsigned local build path.
- Prove the host executes only its embedded helper and handles timeout,
  cancellation, unsupported wire version, partial data, and helper failure.
- Add Swift unit tests without reading real AgentDeck or client state.
- Verification level: L3 for new build and application boundaries.

### 3. `menubar-experience`

- Contracts: [`architecture.md`](architecture.md#presentation-gaps-raised-by-the-reviewed-surfaces)
  for the additive wire families, [`ux/menubar.md`](ux/menubar.md#sections) for
  the reading surface and presentation state, and [`ux/settings.md`](ux/settings.md)
  for the settings window.
- **One task, several wire owners.** The contract provisions filtered presentation in
  different places, and they have different producers, different DTOs, and
  different decoders. Treating them as one object would
  either duplicate the session schema under usage or leave two owners fighting over
  `data.sessions`:

| Sub-change | Wire location | Go producer | Swift decoder |
| --- | --- | --- | --- |
| `quality` and `pricing` move from client scope to the `Client` × `Period` product | `data.usage.presentation.scopes[].quality.items[]` and `…pricing.items[]` | `internal/usage/presentation.go` | `DesktopUsageQualityV1` / `DesktopUsagePricingV1` in `apps/macos/AgentDeckShared/DesktopWire.swift` |
| `sessions` gains per-period grouping and per-period counts beside its bounded recent list | `data.sessions.periods.items[]`, with `data.sessions.items[]` unchanged | `internal/desktop/desktop.go` (`SessionsSnapshot`) | `DesktopSessionsSnapshotV1` in the same file |
| Today's hourly trend plus plotted/hover values | `data.usage.presentation.scopes[].hourly`, compact `daily` values, and the packed `rhythm` values | `internal/usage/presentation.go` | `DesktopUsageHourlyV1`, compact display values, and rhythm compatibility decoding in the same file |

- The bounded recent list stays a *recent* list. The panel's statistics come from
  `sessions.periods.items[]`; the host never derives them from the recent rows.
- **Merged by explicit user decision on 2026-08-20.** The earlier decomposition
  separated `presentation-period-scoping` from `menubar-experience`; implementation
  showed that the surface cannot be reviewed atomically without the producer, DTO,
  fixture, and decoder changes it consumes. The old task therefore becomes a
  delivered slice of this task rather than a separate live task.
- These changes are additive and MUST NOT raise `wire_version`; a payload without
  them decodes as `available: false`.
- Depends on tasks 1 and 2, both already Review PASS, so it is startable now.
- **File lists below are stated against the commit baseline**, not against
  whatever happens to be in the working tree. A path that exists only as
  uncommitted work in progress is a file this task creates, because the task's
  commit is what first puts it under version control. Checking the lists against a
  dirty tree hides exactly that distinction.
- Files, existing at the current commit baseline after the delivered producer
  slice: `internal/usage/presentation.go`,
  `internal/usage/presentation_test.go`,
  `internal/desktop/desktop.go` (`SessionsSnapshot`, provider candidates, and
  their builders), `internal/desktop/desktop_test.go`,
  `internal/desktop/fixtures_test.go`, all four canonical/legacy fixtures plus
  `desktop/fixtures/v1/verify.swift` and `README.md`,
  `apps/macos/AgentDeckShared/DesktopWire.swift` (quality, pricing, sessions,
  hourly, compact plotted values, rhythm compatibility, and provider-switch
  DTO/decoder hunks), `apps/macos/AgentDeckTests/DesktopWireTests.swift`,
  `apps/macos/AgentDeckTests/FixtureSupport.swift`,
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json`,
  `scripts/test-macos-app.sh` (**the fixture-argument list only** — every other
  part of that script is task 5's, where it is listed unqualified), and
  `apps/macos/AgentDeckVerification/main.swift` (fixture, presentation,
  legacy/empty-client, and embedded-helper invocation assertions).
- **The fixture set is this task's evidence, so what generates and runs it is
  this task's too.** The four paths above joined the list after the first review
  round, because closing that round's findings needed them and an undeclared path
  cannot be part of the task's atomic commit:
  - `internal/desktop/fixtures_test.go` generates the canonical fixtures from the
    real producer and pins them byte-for-byte. It is what makes the fixtures
    evidence about the producer rather than illustrations beside it; without it a
    fixture can satisfy every decoder while being a payload the producer cannot
    emit, which is what the first round found.
  - `desktop/fixtures/v1/snapshot-empty-client.json` is the payload for the empty
    concrete-client state `architecture.md` requires, and it is the only fixture
    in which both Swift decoders see that state.
  - `desktop/fixtures/v1/README.md` states the regeneration command and what each
    of the four fixtures represents. A generated fixture set whose regeneration
    command is undocumented gets regenerated by hand, which is the defect it was
    introduced to remove.
  - `scripts/test-macos-app.sh`'s fixture-argument list registers the two new
    fixtures with the command that actually runs the standalone gate. **A gate
    that is not named in the command that runs it does not run** — the same
    failure mode the menu-bar subsection records for a test target missing from the
    scheme, and it
    is equally indistinguishable from passing.
  - `apps/macos/AgentDeckVerification/main.swift` is the in-repo Swift path that
    `scripts/test-macos-app.sh` executes, and it is where the production decoder
    reads all four fixtures. `verify.swift` alone does not satisfy "both Swift
    decoder paths", because it carries its own DTO layer rather than
    `AgentDeckShared`'s. Its other hunk — the embedded-helper invocation count —
    belongs to the menu-bar subsection below.
- The producer and decoder sub-boundaries above remain explicit even though they
  now land in one Task. Different owners explain the file/hunk split; they no longer
  imply separate review, evidence, or commit checkpoints.
- [`reviews/presentation-period-scoping.md`](reviews/presentation-period-scoping.md)
  preserves the review history of the already committed producer slice. Commit
  `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7` is reusable partial evidence, not a
  PASS for this merged task. The combined task's current review history remains
  [`reviews/menubar-experience.md`](reviews/menubar-experience.md).

#### Menu-bar and settings surface

- Contracts: [`ux/menubar.md`](ux/menubar.md#sections) for the reading surface,
  [`Presentation state`](ux/menubar.md#presentation-state) for its state model,
  [`ux/settings.md`](ux/settings.md) for the settings window, and
  [`architecture.md`](architecture.md#menu-bar-wire-contract-extension) for the
  additive `usage.presentation` and `provider.candidates` objects, the switch
  command surface, its result envelope, and switch operation ownership.
- Implement the reading surface exactly as the Sections contract defines it: the
  client and period filters, the four filtered panels, the unfiltered rhythm
  block below them, the notice strip and its health detail, and the provider
  footer.
- **Both filters govern every filtered panel.** A panel that ignores one of them
  is a defect, not a simplification; this is the correction the reviewed
  prototype exists to record.
- The Sessions panel ships its statistics, per-project rows, and recent-session
  rows, **and it ships all three work-signal modules** — real headings, real
  layout, and the explicit `Not captured yet` state, which is what the shipped
  build renders today. The values behind them come from the sibling
  [`work-signals`](../work-signals/tasks.md) topic; that topic's wire additions
  are additive, so this task's host renders the pending state against an older
  payload and the captured state against a newer one without being rebuilt.
- Implement the settings window as its own window with the four preferences,
  their defaults, and the login-item refusal presentation.
- Implement the menu-bar item, its value and scope modes, and its right-click and
  double-click menu carrying Settings and Quit. The popover carries no
  application exits.
- Deliver no update check anywhere: no menu item, no preference, no copy, no
  network request.
- Derive presentation from the coordinator surfaces and orthogonal qualifiers in
  the Presentation state contract; do not introduce a second state machine.
- Verify VoiceOver, keyboard navigation, reduced motion, high contrast, locale,
  narrow layout, and appearance changes on macOS 26.
- Ship English and Simplified Chinese user-visible strings for both surfaces.
- Depends on tasks 1 and 2, both already Review PASS. It does not depend on task 4
  (`desktop-widget`) for any behavior:
  neither surface reads the other, and either can be implemented and reviewed first.
  What the two do share is one file — task 4 adds the two hunks that make the app
  target embed the widget, and those land in the app target this task otherwise owns.
  That is a landing-order constraint on `project.pbxproj`, resolved by the rebase
  rule below, not a behavioral dependency in either direction.
- Files, existing at the commit baseline:
  `apps/macos/AgentDeckApp/AgentDeckApp.swift`,
  `apps/macos/AgentDeckApp/Info.plist`,
  `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` (the refresh-coordinator
  and presentation-state hunks only; the process and capture contract is task 2's),
  `apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift`,
  `apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift`,
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj` (the app target's hunks **and
  the `AgentDeckAppTests` test-target hunks** it adds, but **not** the two hunks
  that embed the widget extension — those are task 4's, listed there),
  `apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme` (its
  own `<TestableReference>` entry only),
  `internal/desktop/desktop.go` (the `ProviderCandidate`,
  `ProviderCandidateCredential` and `ProviderSwitchOption` types, the
  `ProviderSnapshot.Candidates` field, and `providerCandidates`,
  `providerCandidateClients`, `providerClientLess`, `providerCandidateOptions`
  and `providerOptionReason`; the `SessionsSnapshot` hunks belong to the wire
  subsection above), `internal/desktop/desktop_test.go` (the candidate and option-reason
  tests only), `apps/macos/AgentDeckShared/DesktopWire.swift` (the
  `DesktopProviderCandidateV1`, `DesktopProviderCredentialV1` and
  `DesktopProviderSwitchOptionV1` DTOs, the `candidates` field and its decoder,
  and `ProviderUseEnvelopeV1` with `decodeProviderUseEnvelopeV1`; the quality,
  pricing, hourly, rhythm, compact-value, and sessions DTOs belong to the wire
  subsection above),
  `apps/macos/AgentDeckTests/DesktopWireTests.swift` (the candidate assertions
  and the `candidates`/`options` malformed-family cases only),
  `apps/macos/AgentDeckTests/FixtureSupport.swift` (the multi-behaviour recorder
  that the sequential index-refresh tests need),
  `apps/macos/AgentDeckVerification/main.swift` (the embedded-helper invocation
  assertions only; its fixture and presentation/legacy/empty-client assertions
  belong to the wire subsection above),
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json` and
  `desktop/fixtures/v1/snapshot-complete.json`,
  `desktop/fixtures/v1/snapshot-partial.json`,
  `desktop/fixtures/v1/snapshot-empty-client.json`.
- **The wire objects this task's own contract names are this task's to build.**
  Its `Contracts:` line above owns `provider.candidates`, the switch command
  surface, its result envelope, and switch operation ownership; a task cannot own
  a wire contract and disown the producer, DTOs and fixtures that realize it. The
  paths above were missing from this list until the first implementation review,
  which is what left the switch surface with no declared owner at all.
- **The three current fixtures and the command-contract golden are generated,
  not edited.** `internal/desktop/fixtures_test.go` regenerates the fixtures from
  the combined producer under `AGENTDECK_UPDATE_FIXTURES=1`, and
  `UPDATE_AGENTDECK_GOLDEN=1` regenerates the golden. Because the merged task owns
  the presentation, sessions, and `provider.candidates` contributions together,
  its final relevant edit regenerates each indivisible artifact once before review.
- **A test target that is not in the scheme does not run.**
  `scripts/test-macos-app.sh` invokes `xcodebuild -scheme AgentDeck … test`, and
  `xcodebuild test` executes the scheme's `<Testables>`, not every unit-test target
  in the project. Adding `AgentDeckAppTests` to the project without adding it to the
  scheme produces a target that builds, never executes, and leaves the command
  green — a failure indistinguishable from passing. Acceptance for this task
  therefore includes: `bash scripts/test-macos-app.sh` output names
  `AgentDeckAppTests` and reports its test count.
- Creates: `apps/macos/AgentDeckApp/Localizable.xcstrings` and
  `apps/macos/AgentDeckApp/Assets.xcassets/`, both currently uncommitted work in
  progress that this task commits; the settings window and menu-bar-item Swift
  sources under `apps/macos/AgentDeckApp/`; and the `AgentDeckAppTests` Xcode
  unit-test target with its sources under `apps/macos/AgentDeckAppTests/`;
  `acceptance/menubar-experience.md` is this task's safety-routed acceptance
  runbook and evidence handoff template.
- **Its tests do not go in `apps/macos/AgentDeckTests/`.** That directory is bound
  by `path: "AgentDeckTests"` to the `AgentDeckSharedTests` target, which depends on
  `AgentDeckShared` only and cannot see `AgentDeckApp/`. The two existing files
  listed above stay there because what they test — the refresh coordinator and the
  helper runner — lives in `AgentDeckShared`. Anything testing the settings window or
  the menu-bar item needs a target that links the app, which is why this task adds
  one. `architecture.md`'s Build and configuration section says the project defines
  isolated unit-test *targets*, plural; this is one of them.
- The app's string catalog is this task's exclusively. Task 4 creates and owns the
  widget's own catalog; neither task edits the other's.
- Owns both declared shares of `apps/macos/AgentDeckShared/DesktopWire.swift`: the
  wire subsection's presentation/session DTOs and compatibility decoding, plus the
  menu-bar subsection's provider-switch DTOs. The envelope, snapshot, usage, health,
  and route foundation types remain task 2's. It does not own
  `AppGroupSnapshotStore.swift`; that is task 4's projection boundary.
- Verification level: L3 including rendered and interactive acceptance.

### 4. `desktop-widget`

- Contracts: [`ux/widget.md`](ux/widget.md#surface-and-qualifiers) for the
  surface/qualifier model and [`Timeline`](ux/widget.md#timeline) for WidgetKit
  entry-point behavior.
- Add WidgetKit timelines and App Intent configuration backed only by the
  redacted App Group snapshot.
- Build and judge every size at its true canvas proportion — `systemLarge` is
  `systemMedium`'s width and roughly twice its height, not a full-row band. The
  reviewed prototype's first draft got this wrong and hid seven clipped regions.
- Honor the caption size bound and the fixed-height grid cell rule in
  [`ux/widget.md`](ux/widget.md#what-a-size-can-actually-hold); when content and
  canvas conflict, drop the least load-bearing element rather than shrinking type
  or clipping.
- Derive every figure from the same bounded series and 7×24 grid the popover
  reads, so the two surfaces cannot state contradictory facts.
- Implement the surface, qualifier, and timeline entry-point behavior from those
  contracts; do not introduce a parallel state vocabulary.
- Prove the Widget cannot read AgentDeck databases, credentials, client config,
  or raw session sources.
- Ship English and Simplified Chinese strings for every widget-visible element:
  the copy table in [`Copy`](ux/widget.md#copy) and the `Client` App Intent
  parameter values, whose `en`/`zh-Hans` pairs `ux/widget.md` fixes. Acceptance
  condition 8 there — both languages render without truncation or clipping at every
  size — is this task's to satisfy, and it is the reason the widget owns its own
  catalog rather than borrowing the app's.
- Depends on task 2 for the App Group snapshot store and on task 3 for the
  period-scoped data the projection carries. It consumes task 3's producer through
  that projection rather than by reading the wire itself, which is why the
  dependency is real even though no Go file is shared.
- Files: `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift` (the projection
  hunks that add the period-scoped fields; the atomic-write and fail-closed
  behavior is task 2's and stays as reviewed),
  `apps/macos/AgentDeckTests/AppGroupSnapshotStoreTests.swift`,
  `apps/macos/AgentDeckWidget.entitlements`,
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj` (the widget target's hunks, the
  `AgentDeckWidgetTests` test-target hunks, **and the two app-target hunks that
  embed the extension** — the `PlugIns` copy phase and the app target's
  `PBXTargetDependency` on the widget target),
  `apps/macos/AgentDeck.xcodeproj/xcshareddata/xcschemes/AgentDeck.xcscheme` (its
  own `<TestableReference>` entry only).
- **Embedding is this task's, not task 3's.** A macOS app extension ships only if
  the host application target copies it through a `PlugIns` phase and depends on its
  target, and the current `AgentDeck` target has neither — its build phases stop at
  `Embed AgentDeck Helper` and it carries one dependency. Those two hunks live in the
  app target but belong to the delivery that needs them: a widget nobody embeds is
  not a widget. Task 3's app-target share explicitly excludes them, and task 5
  cannot supply them because its `project.pbxproj` share is signing and packaging
  build settings only.
- **A test target that is not in the scheme does not run**, exactly as stated in
  task 3. Acceptance for this task includes: `bash scripts/test-macos-app.sh` output
  names `AgentDeckWidgetTests` and reports its test count.
- Creates: the `AgentDeckWidget/` target sources — timeline provider, App Intent
  configuration, and the four widget families at all three sizes;
  `apps/macos/AgentDeckWidget/Localizable.xcstrings` for the strings above; the
  `AgentDeckWidgetTests` Xcode unit-test target with its sources under
  `apps/macos/AgentDeckWidgetTests/`; and `scripts/check-widget-sandbox.sh` for the
  static half of the negative privacy proof below.
- **Its tests do not go in `apps/macos/AgentDeckTests/`** either, for the same
  reason as task 3: that directory belongs to `AgentDeckSharedTests`, which links
  only `AgentDeckShared`. `AppGroupSnapshotStoreTests.swift` stays there because the
  store is Shared code; the timeline provider, the App Intent, and the four families
  need a target that links the widget.
- The negative privacy proof has two halves, and only one of them is a script:
  - **Static**, this task's `scripts/check-widget-sandbox.sh`: the widget target's
    entitlements grant the App Group and nothing else; no widget source references
    an AgentDeck database, credential store, client-configuration, or raw session
    path; and the widget target links no module that could reach one.
  - **Runtime**, a manual acceptance step on macOS 26 with the extension actually
    running: those reads fail rather than merely being absent from the source.
    Stated as manual because a sandbox denial is observable only from inside the
    running extension.
  - `scripts/check-privacy.sh` is listed above for a different job and does not
    contribute to this proof. It enumerates repository files and greps for
    credential *literals*; it says nothing about what a process can open. The L3
    level's "privacy checks" means the two halves here, not that script.
  - `check-widget-sandbox.sh` is invoked directly by this task as its own L3
    evidence. Whether it also joins the aggregate release gate is task 5's to wire,
    and task 5 says it does — recorded in both places because a script created by one
    task and wired by another is exactly the seam where neither does it.
- The widget gets **its own** string catalog rather than a share of task 3's.
  `apps/macos/AgentDeckApp/Localizable.xcstrings` stays task 3's exclusively, and
  its target does not include the widget's strings. A second catalog is the cheaper
  boundary here: the alternative is two tasks editing one catalog, which is the
  overlap the `Files` lists exist to prevent, and an extension target loading the
  containing app's catalog is not something either target's membership gives it for
  free. Strings shared between the two surfaces, if any turn out to be identical,
  stay duplicated rather than hoisted — a shared catalog would recreate the
  overlap for the sake of a handful of words.
- Shares `project.pbxproj` with tasks 3 and 5, in four shares: task 3 owns the app
  target and the `AgentDeckAppTests` target **except** the two embedding hunks above;
  task 4 owns the widget target, the `AgentDeckWidgetTests` target, and those two
  embedding hunks; task 5 owns the signing and packaging build settings. Whichever
  lands second rebases rather than reformats, and a whole-file regeneration by any of
  them is a defect.
- Shares `AgentDeck.xcscheme` with task 3 on the same terms: one
  `<TestableReference>` entry each, nothing else in the file, same rebase rule.
  Xcode rewrites this file wholesale just as it does `project.pbxproj`, so it needs
  the same convention rather than being left as an unclaimed side effect of opening
  the project.
- `apps/macos/Package.swift` is changed by **no task in this topic**, and that is a
  decision rather than an omission. The package exists only to exercise
  `AgentDeckShared` without Xcode, as its own header says; it cannot host an
  application or app-extension test bundle, so the two new test targets belong to
  the Xcode project and the manifest stays scoped to Shared. A later task that needs
  a Shared-only test target may extend it and must then claim it explicitly.
- Verification level: L3 including the extension sandbox and privacy checks named
  above.

### 5. `unified-desktop-distribution`

- **This task builds and tests the distribution automation. It performs no real
  release action.** Signing with a development-team certificate, notarizing with
  Apple, uploading assets, opening a tap pull request, and publishing are each
  outside what a development phase may do here — the certificate, the notarization
  credential, the tap write, and the publication decision are all authority this
  topic does not hold, and the version contract reserves the release decision
  anyway. Writing the task as "sign, notarize, publish" made it literally
  uncompletable, which is worse than making it smaller: an implementer either stops
  and asks, or reaches past the ceiling.
- Implement: the universal helper and full App bundle build, nested-code signing
  **invocation**, notarization and stapling **invocation**, direct-download asset
  assembly, the `agentdeck-app` Cask rendering, and Formula-to-Cask migration and
  mutual-exclusion behavior.
- Test all of it in isolation, with no external state: ad-hoc or self-signed
  identities for the signing path, a stubbed notarization response for the stapling
  path, a local tap fixture for the Cask render and pull-request body, and a
  temporary `HOME` and prefix for every install path. A test that needs the real
  Developer ID, the real Apple service, or the real tap is out of scope for this
  task by construction.
- Preserve CLI-only Formula archives and tests.
- Verify against locally built artifacts: fresh Cask install, upgrade, uninstall,
  direct DMG installation, completion loading, state preservation, Gatekeeper
  assessment of a locally signed bundle, arm64, and Intel behavior.
- There is no user CLI link to verify: `architecture.md`'s Direct download section
  ships no link action, and the command line stays the Formula's and the Cask's to
  provide.
- Real signing, notarization, upload, tap pull request, and publication belong to
  the separately authorized exact-SHA release workflows, which run on a commit that
  already carries preflight evidence. Task 5 is what makes those workflows
  possible; it is not permitted to run them, and its Review PASS is not a release
  decision.
- Depends on tasks 3 and 4, because it packages the app and the widget it signs.
- Files: `scripts/build-macos-app.sh`, `scripts/test-macos-app.sh`,
  `scripts/release-archive.sh`, `scripts/verify-release-artifacts.sh`,
  `scripts/render-homebrew-formula.sh`, `scripts/update-homebrew-tap-pr.sh`,
  `scripts/test-release-distribution.sh`, `scripts/test-install.sh`,
  `scripts/manage-install.sh`, `scripts/test-completion-install.sh`,
  `packaging/homebrew/agentdeck.rb.tmpl`, `.github/workflows/release.yml`,
  `.github/workflows/release-preflight.yml`, `.github/workflows/ci.yml`,
  `apps/macos/AgentDeck.xcodeproj/project.pbxproj` (signing and packaging build
  settings only), `apps/macos/Config/AgentDeck.xcconfig`, `Makefile`, and
  `scripts/check-release-workflow.rb` plus
  `scripts/check-release-preflight-workflows.rb`. **The two workflow checkers
  joined this list during implementation** and are not scope creep: they are what
  makes a workflow assertion falsifiable, and a desktop job added without them
  would be the one release path nothing checks. `release-archive.sh`,
  `verify-release-artifacts.sh`, `test-install.sh`, `manage-install.sh` and
  `test-completion-install.sh` were listed but needed no change — the CLI-only
  path they own is preserved rather than modified, which is what this task
  promised about them.
- Creates: `packaging/homebrew/agentdeck-app.rb.tmpl` for the Cask,
  `scripts/render-homebrew-cask.sh` that renders it, `scripts/package-macos-app.sh`
  for signing, artifact assembly, notarization and stapling, and the two test
  scripts `scripts/test-macos-distribution.sh` and
  `scripts/test-cask-migration.sh`. `architecture.md`'s structure sketch named
  `notarize-macos-app.sh` and `test-desktop-distribution.sh`; the delivered names
  and the reasons they differ are recorded there rather than left as drift.
- **How the untestable parts are tested.** A Developer ID certificate, the Apple
  notarization service, and the real tap are all authority this task does not
  hold, so each is replaced by something observable: signing runs ad-hoc, or
  against a recording `codesign` stub whose log *is* the inside-out order
  assertion; notarization and stapling run against stubs whose recorded
  invocation is asserted; the tap is a local bare repository with a stub `gh`.
  A stub cannot establish that Homebrew itself behaves as the cask declares, so
  that question is split in two. Whether Homebrew *accepts* the rendered cask is
  answered by Homebrew: `test-macos-distribution.sh` loads each rendered cask
  through a throwaway tap it creates and removes inside the local Homebrew
  prefix, and requires the load to succeed with no deprecation warning. Whether
  the *declared* artifact set installs, upgrades, uninstalls and excludes the CLI
  formulae correctly is `test-cask-migration.sh`, which applies those
  declarations through a local installer and asserts nothing about Homebrew's
  implementation. Both boundaries are stated in the two files' headers.
  Real `brew install --cask` against a published DMG is a step of the release
  workflow, which this task builds and is not permitted to run.

  **Verification prerequisite.** The load check makes Homebrew a prerequisite of
  `make check-macos-distribution`, and therefore of `release-verify`. It is a
  hard requirement, not a skip: a load check that quietly disappears when `brew`
  is missing reads exactly like one that passed. It installs nothing, reads and
  writes only its own `agentdeck-fixture` tap, and refuses to run when a previous
  run left that tap behind.
- Wire task 4's `scripts/check-widget-sandbox.sh` into the aggregate gate, as a
  `Makefile` target reached by `release-verify` — the same shape
  `scripts/check-privacy.sh` already has. Task 4 creates the script and runs it
  directly as its own L3 evidence; this task owns `Makefile`, so the wiring is this
  task's. It is written in both places because a script one task creates and another
  wires is exactly the seam where neither does it, and a static sandbox assertion
  that runs only once during the task that wrote it is not a gate.
- **Unverified from here: the runner label.** The desktop jobs in `ci.yml`,
  `release.yml` and `release-preflight.yml` request `macos-26`, because the
  surfaces need the macOS 26 SDK and no earlier image carries it. Whether GitHub
  offers that label to this repository was not verified — it cannot be, without
  dispatching a run — so the first CI run on a branch carrying these workflows is
  what confirms it. If the label is unavailable the desktop jobs fail to start,
  which is visible and recoverable; the alternative, pinning an older image, would
  produce a green CI that never builds the app.
- Verification level: L4 through an expanded aggregate release gate, run against
  locally produced artifacts.

### 6. `desktop-app-contract`

- Reconcile **this topic's** delivered behavior into the living specs and manual:
  the wire contract, the menu-bar app, **the settings window and its four
  preferences**, the widget, packaging, and distribution behavior actually
  delivered. The settings surface is a separately reviewed contract
  (`ux/settings.md`) and was missing from this list, which is how a whole surface
  reaches release without entering the living specification.
- **Reconcile this topic's own document set first, then review it once.** Bring
  `requirements.md`, `architecture.md`, the three `ux/` documents, and this file
  into agreement with the final prototype and the shipped implementation, then
  submit the reconciled set for the single document review this topic deferred
  (see *Document review is deferred to one closing pass*). This precedes the
  living-spec reconciliation above, because a specification written from a
  document set that still disagrees with the build carries the disagreement into
  `docs/specs/`.
- **Every prior task's Review PASS is a precondition, not work this task does.**
  Task 6 cannot close its own review, and it cannot close anyone else's: a review
  record is closed by the independent Review that passes it. What task 6 does is
  verify that the records are already closed and that the document set is
  consistent with what shipped, then report any that are not as a blocker rather
  than ticking them.
- Task 6's own Review is likewise independent. Reaching the end of this task means
  the reconciliation is ready to review, not that the topic is closed.
- Confirm the app, CLI, wire-contract, and package identities agree with each
  other. **Reuse task 5's evidence rather than re-deriving it:** identity is an
  artifact property that task 5 established at L4 against a specific content state.
  If the tree is unchanged since that run — verified by status, diff, and the same
  commit or tree hash, not by assumption — task 6 cites it and runs no build. If
  anything relevant changed, the identity check is task 5's to redo at its own
  level, not something task 6 re-asserts under an L2 gate it cannot support.
- **This task does not raise the specification version, run technical preflight,
  choose a release channel, or write release notes.** The version-wide raise is
  owned by the [v0.5.0 contract closure](../v0-5-0-contract/tasks.md). Preflight
  and any RC or stable publication remain separate, explicitly authorized
  workflows.
- Depends on tasks 1 through 5, each at Review PASS.
- Files, existing at the commit baseline: `docs/specs/cli-design.md`,
  `docs/specs/cli-manual.md`, `docs/status.md` (the topic's stage row, **and the
  `desktop-app` deferral paragraph**), and — for the deferred reconciliation
  above — this topic's own `requirements.md`, `architecture.md`,
  `ux/menubar.md`, `ux/settings.md`, `ux/widget.md`, and `tasks.md`. It also owns
  **the deferral note at the foot of each of the six document records under
  `reviews/`**, which is the note's own bookkeeping — the pass's task number and
  whether the set has been submitted — and never a round, a verdict, or a
  finding, which stay the reviewer's. The list originally omitted both, and the
  omission was not neutral: it left seven documents naming a task 7 that the
  2026-08-18 re-cut had renumbered to 6, in the one task whose subject is making
  this topic's documents agree with each other.
- Creates: `docs/topics/desktop-app/reviews/desktop-app-contract.md`, when this
  task's first review round runs. The deferred document review's round is **not**
  written there: it appends to each reconciled document's own record under
  `reviews/`, which is where that document's history already lives.
- Changes no product code, test, or configuration. If reconciliation reveals that
  the specification and the delivered behavior disagree, that is a finding against
  the task that delivered it, not a fix task 6 applies to the code.
- Verification level: L0. It is a documentation change plus a reuse of task 5's
  unchanged exact-state evidence, so it requires the documentation and discovery
  checks and no product test run. The previous L2 claim implied a contract test
  this task neither changes nor can produce.

## Document review is deferred to one closing pass (2026-08-20)

By user instruction on 2026-08-20, this topic runs **no document review rounds
while its tasks are being implemented**. Document review is not cancelled — it is
deferred to a single closing pass after every implementation task is done.

**During implementation.** A change to this topic's behavior, contract, or
surface is written **directly into the document that owns it** — this file for
task state, `architecture.md` for the boundary, the `ux/` document for the
surface, `acceptance/` for manual acceptance — in the same action that makes the
change. The record is updated to state what is true now. No round is opened, no
verdict is recorded, and no `Review` cell moves.

**At the end.** Once tasks 1 through 5 are implemented and reviewed, the whole
document set is reconciled against the final prototype and the shipped
implementation in one pass, and *that* set is reviewed once. It is scoped as a
bullet on task 6, which is already the topic's documentation-only closing task.
Reviewing the set once, against something that exists, is the only review of it
worth running.

Why: nineteen prose rounds passed a document set that a single reviewable
prototype then falsified, and the twenty-plus rounds that followed consumed two
days without changing what the product does. The prototype and the shipped
implementation are the specimens this topic is judged against, and they are still
moving. A document reviewed against a moving implementation is re-reviewed every
time the implementation moves, which is what happened.

**Task review is unaffected.** What is deferred is *document* review — prose
judged against prose. Every task still gets an independent review before it is
  done, because that judges code against a contract. The live task records under
  [`reviews/`](reviews/) — `desktop-wire-contract`, `macos-app-foundation`, and
  `menubar-experience` — remain working documents. `presentation-period-scoping`
  preserves the exact-state history of the producer slice merged into
  `menubar-experience`; it is no longer a separate live task.

The six **document** records under [`reviews/`](reviews/) — `requirements`,
`architecture`, `ux-menubar`, `ux-settings`, `ux-widget`, and `tasks` — are kept
and each carries a note pointing at this section. They are paused, not closed:
the closing pass appends its round to them.

**Completion evidence.** A `document` boundary is crossed when a review reaches
`Verdict: PASS`, so this topic records no new `document` evidence until the
closing pass passes — at which point it records the whole set against one content
state, which is both cheaper and more truthful than six boundaries bound to six
states that no longer exist. Nothing already recorded was invalidated by
deferring: the store holds one `document` WorkUnit for this topic,
`desktop-app-ux-settings`, still `active`, and `ux/settings.md` is untouched;
`menubar-experience`'s four criteria bind builds, wire behavior, and manual macOS
acceptance, none of them a document's content. Checked 2026-08-20.

Three consequences bind the rest of this file:

- The `Review` cells below retain the **suspended historical values** from before
  deferral. They do not report the closing pass.
- The dated `Closing review` column is the closing-pass result: `[x]` means the
  document's own outcome is PASS, `[ ]` means REOPEN, and `—` marks a row outside
  the six-document closing set.
- The per-document "Current … review" sections and the round history below record
  the earlier process; the closing pass appends its own round to each of the six
  document records.

## Documents

| Document | Draft | Review | Closing review (2026-08-23) |
| --- | --- | --- | --- |
| requirements.md | [x] | [x] | [x] |
| architecture.md | [x] | [x] | [x] |
| ux/menubar.md | [x] | [x] | [x] |
| ux/settings.md | [x] | [x] | [x] |
| acceptance/menubar-experience.md | [x] | [ ] | — |
| ux/widget.md | [x] | [x] | [x] |
| tasks.md | [x] | [x] | [x] |

### Why every Review cell is unticked again (2026-08-18)

The set was green. It is not any more, and the reason is worth stating plainly
rather than burying in a round note.

A reviewable prototype was built for the menu-bar and widget surfaces — a running
application at the contract dimensions, in both appearances and both languages,
with every degraded state addressable. Reviewing **it** rather than a text sketch
surfaced defects that the prose had passed:

- the client and period filters governed only two of the four sections beneath
  them, with no disabled state and no explanation for the other two;
- the widget board drew `systemLarge` as a wide full-row band. At the true
  proportion — `systemMedium`'s width, roughly twice its height — seven regions
  clipped, including a whole statistics strip;
- the stated geometry was 340 × 560 pt while the shipped host used 420 × 760 pt,
  and nobody had noticed because the document stated geometry only as numbers;
- health expanded inline above the footer, covering data and deforming the footer
  at once;
- three modules were specified whose data no field in the projection supplies.

Those are the defects the specimen requirement in `docs/documentation-workflow.md` exists to
catch, and they were caught. Acting on them changed the content of every document
in the set, and **evidence binds to a content state, not to a file name** — a
`PASS` recorded against the previous text says nothing about this one. So every
cell is unticked and the set is re-reviewed in order.

What survives unchanged, and is not being re-litigated: the presentation
surface-and-qualifier state model, the provider switch flow end to end, the
privacy and logging boundaries, the four-question derivation of both surfaces,
and the size-is-depth rule for widgets. The changes are to structure, geometry,
and data provisioning — not to the model underneath them.

The prototype is [`prototype/interactive-v7/`](prototype/interactive-v7/) and is
cited by all three surface documents. It is the only prototype retained: the
earlier iterations and the static `desktop-surfaces.html` were removed once this
one superseded them, so nothing in the tree can be mistaken for a current
specimen. The round history below cites the removed HTML page; those citations
resolve through Git history, which is where a superseded specimen belongs.

### Review order

The order follows the dependency, not convenience: the boundary decides what the
surfaces may contain, the surfaces decide what contracts must provision, and the
decomposition is judged against all of them.

1. `requirements.md` — the boundary moved: update check withdrawn entirely, work
   signals moved to Backlog, a third surface added.
2. `ux/menubar.md`, `ux/settings.md`, `ux/widget.md` — the three surfaces. Any
   order among them; they do not depend on each other.
3. `architecture.md` — judged against the surfaces, especially the three
   provisioning rulings.
4. `tasks.md` — judged against all of the above, last.

### Current requirements review

Requirements Review Round 6 (2026-08-18): **FAIL**. The revised boundary names
the settings window and constrains the quality of preferences it exposes, but it
does not authorize a user outcome or the four-preference set that
`ux/settings.md` and `menubar-experience` require. An empty window with no
preferences would still satisfy the written acceptance boundary. R6-F1 is
recorded in [`reviews/requirements.md`](reviews/requirements.md).

Independent Re-review Round 7 (2026-08-18): **PASS**. Goals and Acceptance now
authorize exactly launch at login, periodic refresh, menu-bar value, and
menu-bar scope, matching `ux/settings.md` and `menubar-experience` one-to-one.
R6-F1 is closed and the requirements Review cell is ticked.

### Current menu-bar document review

`ux/menubar.md` Review Round 1 (2026-08-19): **FAIL**. A headed review of the
current referenced prototype closed the interaction, state, localization, and
layout questions but found M1-F1: scoped axe WCAG A/AA checks report serious
text-contrast failures in both Light and Dark appearances. The finding is
recorded in [`reviews/ux-menubar.md`](reviews/ux-menubar.md); the document's
Review cell remains unticked. Prototype mock arithmetic is explicitly not a
finding.

Independent Re-review Round 3 (2026-08-19): **PASS**. M1-F1 is closed on the
same contract blob and repaired specimen: current Light and Dark captures keep
the named roles legible, and an independent foreground/background check found
no ratio below 4.5:1 across 292 affected role instances. The exact content state
has a non-vacuous `VERIFIED` CEv1 gate, so the `ux/menubar.md` Review cell is
ticked. Surface-document review continues with `ux/settings.md`.

### Current settings document review

`ux/settings.md` Review Round 1 (2026-08-19): **FAIL**. S1-F1 is that the
referenced specimen leaves both switch hints as unrelated visible text instead
of exposing them as the controls' accessible descriptions. S1-F2 is that the
conditional login-item failure is not a status or live region and therefore is
not announced when it appears. Both findings are recorded in
[`reviews/ux-settings.md`](reviews/ux-settings.md); the Review cell remains
unticked pending repair and independent Re-review.

Independent Re-review Round 3 (2026-08-19): **PASS**. Current `zh` and `en`
accessibility trees expose every localized description; the refusal leaves the
switch off with one announced status and no disabled sibling control or second
dialog, and the next successful attempt clears it. S1-F1 and S1-F2 are closed,
the exact content state has a non-vacuous `VERIFIED` CEv1 gate, and the
`ux/settings.md` Review cell is ticked. Surface-document review continues with
`ux/widget.md`.

### Current widget document review

`ux/widget.md` Review Round 14 (2026-08-19): **FAIL**. W-F12 is that the
current referenced specimen contradicts the normative `magnitude` size-depth
table: medium omits each period's token value and renders 30 rather than 20
bars, while large omits the peak date. The finding is recorded in
[`reviews/ux-widget.md`](reviews/ux-widget.md); the Review cell remains unticked
pending repair and independent Re-review.

Independent Re-review Round 16 (2026-08-19): **PASS**. W-F12 is closed on the
current specimen: medium carries cost and tokens for all three periods, exactly
20 bars and the matching date axis; large carries the localized peak date with
no chip clipping. The exact content state has a non-vacuous `VERIFIED` CEv1
gate, so the `ux/widget.md` Review cell is ticked. Document review continues
with `architecture.md`.

### Current architecture document review

`architecture.md` Review Round 1 (2026-08-19): **FAIL**. A1-F1 is that the
top-level desktop wire section leaves the already-selected dedicated snapshot
command as a future decision while the same document and current code require
`desktop snapshot` wire v1. A1-F2 is that Direct download adds an optional
Settings CLI-link action outside the approved four-preference surface and task
boundary. Both findings are recorded in
[`reviews/architecture.md`](reviews/architecture.md); the Review cell remains
unticked pending repair and independent Re-review.

Independent Re-review Round 3 (2026-08-19): **PASS**. A1-F1 and A1-F2 are
closed: the dedicated `desktop snapshot` wire-v1 contract is fixed and
implementation-aligned, while Direct download explicitly ships no Settings
CLI-link action and task 6 agrees. The previously unreviewed remainder of the
current architecture produced no new blocker; the exact content state has a
non-vacuous `VERIFIED` CEv1 gate. The `architecture.md` Review cell is ticked,
and document review continues with `tasks.md`.

### Current tasks document review

`tasks.md` Review Round 6 (2026-08-19): **FAIL**. T6-F1 assigns period-scoped
sessions to the wrong wire owner; T6-F2 leaves tasks 3–7 without required file
ownership; T6-F3 leaves the current dependency graph incomplete; T6-F4 mixes
distribution implementation with separately authorized publication; T6-F5
leaves the final contract task's settings, Review ownership, and evidence reuse
incomplete; T6-F6 leaves the version contract at six desktop tasks while this
matrix has seven; and T6-F7 retains withdrawn update-check scope in task 1. The
findings are recorded in [`reviews/tasks.md`](reviews/tasks.md); the Review cell
remains unticked pending repair and independent Re-review.

Independent Re-review Round 8 (2026-08-19): **FAIL**. Six of Round 6's seven
findings are confirmed closed; T6-F2 is only partly closed, and three findings
remain — widget localization has no owning task, task 3's `Creates` claim does
not hold against the committed baseline, and the matrix preamble still states a
six-task range. They are recorded in [`reviews/tasks.md`](reviews/tasks.md); the
Review cell stays unticked pending repair and a further Re-review.

Independent Re-review Round 10 (2026-08-19): **FAIL**. All three Round 8 findings
are confirmed closed and the commit-baseline rule for `Files`/`Creates` now holds
mechanically, but two findings remain — the test-target growth tasks 4 and 5 need
belongs to no task, and task 5's extension-sandbox proof names no artifact that
can perform it. They are recorded in [`reviews/tasks.md`](reviews/tasks.md); the
Review cell stays unticked pending repair and a further Re-review.

Independent Re-review Round 12 (2026-08-19): **FAIL**. Both Round 10 findings are
confirmed closed, but the same reverse reading of the build system found two more
one level out — the shared scheme that decides which test targets `xcodebuild
test` actually runs belongs to no task, and the app-target hunk that embeds the
widget extension is assigned to task 4 while tasks 4 and 5 are declared mutually
independent. They are recorded in [`reviews/tasks.md`](reviews/tasks.md); the
Review cell stays unticked pending repair and a further Re-review.

Independent Re-review Round 14 (2026-08-19): **PASS**. R12-F1, R12-F2 and R12-F3
are closed, the commit-baseline `Files`/`Creates` rule holds under an independent
re-run, and the two questions Round 13 left open both resolve into territory a
task already owns. The `tasks.md` Review cell is ticked and the Documents matrix
is complete. The `completion-evidence/v1` gate for this
exact content state is `VERIFIED`; the record in
[`reviews/tasks.md`](reviews/tasks.md) carries the criterion, content state, and
evidence identifiers, and corrects an earlier `BLOCKED` claim that was wrong.


### Round history for the previous content state

Everything below this line was recorded against the pre-2026-08-18 text. It is
kept because a reader following a citation needs it, and because the findings it
closed are findings this revision must not reintroduce. It is not evidence for
the current content.

`requirements.md` Review Round 1 (2026-08-17): **FAIL**. The boundary still
limits the named menu-bar outcome to current-day usage while the drafted
surfaces require bounded historical analytics, and it gives the Widget no
functional user-visible acceptance outcome. Both findings are recorded in
[`reviews/requirements.md`](reviews/requirements.md). Its `Review` cell remains
unchecked. Re-review Round 2 closed both original findings but found R2-F1: the
new prohibition on every other `breakdown` also forbids the non-temporal
breakdowns required by the authorized composition and trust questions. Re-review
Round 3 narrowed R2-F1 to one omission: the repaired authorization lists every
required non-temporal dimension except provider. Round 4 (2026-08-17) closed it —
`runtime provider` is authorized in both Goals and the Acceptance boundary, and
attribution quality is stated as reportable per client and per runtime provider,
matching what `ux/menubar.md`, `ux/widget.md`, and `architecture.md` all
already require. Re-review Round 5 (2026-08-17) independently confirmed R2-F1
closed and found no regression on the two original findings: **PASS**, `Review`
cell ticked, and the remaining four documents are unblocked. Round 5 also swept
the half of the scope prohibition Round 2 left unchecked and recorded R5-F1
against `ux/menubar.md:754`, whose period switcher asks for week and month
grouping that this requirement does not authorize and the projection does not
provision. It is attributed to that document and closes in its own review, so it
does not reopen this one.

`ux/menubar.md` and `ux/widget.md` are both drafted as of 2026-08-16.
`ux/menubar.md` now carries rendered specimens for healthy, loading, retained
offline, partial with incomplete pricing, the switch confirmation and its
in-flight and failed states, the 280 pt narrow bound, and empty — the readiness
condition it previously failed while stating geometry only as numbers.
`ux/widget.md` is new: it specifies both widget families, the App Intent
configuration, the surface/qualifier table over cache presence, version support
and age, copy in both languages, timeline construction, and the negative
privacy assertions.

Both remain unreviewed, so neither surface may enter development yet. The
document set is audited by `bash scripts/check-topic-docs.sh`, which compares this matrix
against the files on disk and against the surfaces `requirements.md` names.

The foundation runtime contract in `architecture.md` was reviewed and approved
under the previous per-task-design convention; that history is in
[`reviews/macos-app-foundation.md`](reviews/macos-app-foundation.md). The
menu-bar contract failed independent Design Review Round 3 on six blocking
findings recorded in
[`reviews/menubar-experience.md`](reviews/menubar-experience.md); those findings
span both `ux/menubar.md` and the menu-bar section of `architecture.md`, so the
`Review` cell for each stays unticked until the repair passes.

## Tasks

This matrix predates the staged progression, so it was written before
`architecture.md` and `ux/menubar.md` existed in reviewable form — the early
decomposition the progression now forbids. It is not being rebuilt from scratch:
  tasks 1 and 2 are delivered and independently reviewed, and discarding a
decomposition that already produced verified work would cost more than it
corrects.

Instead it was re-derived once the design existed, in the 2026-08-18 pass recorded
under **What changed in the decomposition (2026-08-18)** below. Tasks 1 and 2
entered that pass as fixed inputs — their anchors, boundaries, and evidence stayed
as they are — and the remaining work was re-derived from the reviewed specification.
That pass initially split `presentation-period-scoping` from
`menubar-experience`; the explicit 2026-08-20 user decision merged them again
after implementation showed that the UI and its producer/decoder boundary need
one reviewable Task.
A task whose scope the specification did not support was dropped or re-cut in that
pass, which is the point of decomposing after the design exists.

| Task | Dev | Review |
| --- | --- | --- |
| 1. `desktop-wire-contract` | [x] | [x] |
| 2. `macos-app-foundation` | [x] | [x] |
| 3. `menubar-experience` | [x] | [x] |
| 4. `desktop-widget` | [x] | [x] |
| 5. `unified-desktop-distribution` | [x] | [x] |
| 6. `desktop-app-contract` | [x] | [x] |

### What changed in the decomposition (2026-08-18)

Tasks 1 and 2 are delivered, independently reviewed, and untouched. The rest was
re-cut against the reviewed surfaces:

| Change | Reason |
| --- | --- |
| `presentation-period-scoping` merged into `menubar-experience` as task 3 | Attribution, sessions, hourly presentation, fixtures, decoders, and the UI that consumes them form one reviewable delivery boundary. The earlier producer commit remains partial evidence, while the combined Task stays open until its remaining app work and independent Review pass |
| `menubar-experience` re-scoped | It delivers the period-scoped producer/decoder work plus four filtered panels, an unfiltered rhythm block, a notice strip with health detail, the settings window, and the item's own menu. It no longer delivers an update check. It **does** deliver the three work-signal modules in their uncaptured form; only the data behind them moves to a sibling topic |
| `desktop-widget` re-scoped | Same twelve configurations, but built and judged at the true canvas proportion, with the caption-size bound and fixed-height cell rule as acceptance conditions |
| Work signals implemented by a sibling topic, not dropped | The three modules stay specified in `ux/menubar.md` and stay rendered by `menubar-experience` in their `Not captured yet` form. The data behind them — a classifier over raw session logs, with its own extraction, storage, and privacy analysis — is a usage-domain capability and is delivered by [`work-signals`](../work-signals/tasks.md), which also replaces the pending cards with real values. Both topics are in `v0.5.0`. **Corrected 2026-08-20:** the 2026-08-18 wording said the data was "refused" and put the capability in Backlog, which removed a committed feature from the version without asking |
| Update check removed everywhere | Withdrawn from the version. With it gone the desktop app makes no network request at all, which is a boundary simplification, not only a scope cut |

No task was dropped for being inconvenient, and none was added that the reviewed
surfaces do not require.

### Task round history

`desktop-wire-contract` Review Round 1 (2026-08-13): **FAIL**. The `Review` cell
remained unchecked pending the bounded filesystem-contract and
documentation-index Repair recorded in
[`reviews/desktop-wire-contract.md`](reviews/desktop-wire-contract.md).

`desktop-wire-contract` Re-review Round 2 (2026-08-13): **PASS**. Both Round 1
blockers are closed and the `Review` cell is synchronized.

`macos-app-foundation` development (2026-08-13): **COMPLETE**. The unsigned
Xcode build embeds the AgentDeck helper and shared framework; 10 isolated
XCTest cases passed.

`macos-app-foundation` Re-review Round 3 (2026-08-14): **PASS**. R2-F1 is
closed, all earlier findings remain closed, and 19 XCTest cases pass. Task 3
`menubar-experience` is the next task.

Menu-bar design Review Round 3 (2026-08-16): **FAIL** on six bounded contract
findings. Round 4 repaired all six and recorded the post-migration blob mapping.
Independent Re-review Round 5 (2026-08-16): **FAIL**. R3-F1, R3-F4, R3-F5, and
R3-F6 are closed; R3-F2's transport matrix and R3-F3's retry transition remain
open, and R5-F1 newly identifies conflicting ownership of the dynamic
`switch_in_flight` reason.

Round 6 (2026-08-16): repair complete, `REOPEN` pending independent Re-review.
The transport matrix is now total by construction with an explicit catch-all,
the controller carries a complete transition table making retry and dismiss
bounded exceptions to the non-idle refusal, and `switch_in_flight` is removed
from the wire and respecified as a host-only presentation overlay. Consequential
UX repairs followed: `Cancel` on a finished failure became `Dismiss`,
`indeterminate` was aligned to the same two actions as `failed`, and three
manual checklist items were added. R5-N1 was recorded but not authorized, and is
untouched.

Independent Re-review Round 7 (2026-08-16): **FAIL**. R3-F2 and R5-F1's
wire-ownership defect are closed, but R3-F3 remains open because terminal states
do not retain the complete credential/wrapper target required by same-target
retry. R7-F1 newly records that the architecture applies `Switch in progress`
to every non-idle terminal state while the UX limits it to `inFlight`.

Round 8 (2026-08-17): repair complete, `REOPEN` pending independent Re-review.
Both findings are closed — every non-idle controller state now carries the
complete resolved option so retry reads its target from the state, and the
overlay applies in `inFlight` alone. The same round absorbed a design review of
the rendered prototype: the popover lost invented window chrome, Settings, Quit
and provider switching moved into the footer menu, and both surfaces were
rederived around the four questions the usage data answers — magnitude,
composition, trust, rhythm — with widget size selecting depth rather than
subject. `architecture.md`'s App Group projection was extended to carry the
fields those surfaces asked for.

Independent Re-review Round 9 (2026-08-17): **FAIL**. R3-F3 and R7-F1 are both
closed, and the six findings Rounds 5 and 7 closed show no regression. Two new
blockers come from the redesign itself: Round 8 moved the prose and the
prototype to the four-section body and the footer menu, and two artifacts tied
to the old structure did not follow. R9-F1 is that all four text specimens in
`ux/menubar.md` still draw the five-section body with window chrome and a
flat `Settings…`/`Quit` footer, which the document itself designates as the
inline review entry point, so the file offers implementers two mutually
exclusive structures. R9-F2 is that the Data requirements table lists week and
month bucket grouping as provisioned when the projection carries only a daily
series, and `requirements.md` authorizes no granularity beyond it.

Independent Re-review Round 11 (2026-08-17): **FAIL**, with no serious finding.
Round 10 closed both of Round 9's blockers — the four specimens are redrawn onto
the four-section body with the window chrome and the flat footer gone, and the
Data requirements row now asks for the `today`/`7d`/`30d` selection the daily
`buckets` series actually backs. R5-N1 is closed as not reproducing, verified
back at the blob it cited; the prototype's `Month` tab is gone; the `usage
stats` capability callout was independently confirmed accurate and correctly
left alone; and a topic-wide sweep found no other week/month claim.
`architecture.md` is unchanged and carries no open finding. Two new non-blocking
findings keep the round from passing, both of the same kind as R9-F1 and both
inside the artifact Round 10 redrew: R11-N1, thirteen specimen frame lines one
column wider than their border, a regression against the previous blob's uniform
widths; and R11-N2, the empty specimen printing `No activity today` where the
copy table fixes `No local activity today`. Under the no-deferred-findings gate
adopted this day, a minor finding blocks `PASS`, so both Document cells stay
unticked; the repair is about fourteen lines and touches no behavior contract.

Independent Re-review Round 13 (2026-08-17): **PASS**. Round 12 closed both:
every specimen frame line re-measures at exactly 46 or 36 display columns —
the two rows carrying annotations outside the box excluded, as R11-N1 itself
excluded them — and the empty specimen now reads `No local activity today`,
the string the copy table fixes for a current, issue-free surface. The repair
also corrected the one same-kind row R11-N1 had not named. No new finding, and
no regression in the four-section structure, the Data requirements row, or the
other fixed copy. Both `architecture.md` and `ux/menubar.md` therefore have
every finding closed since Round 1, and both Document cells are ticked. Their
CEv1 Document gates re-query as `VERIFIED` against this exact uncommitted
candidate state, which must be re-recorded against the Git tree once an
authorized commit exists. Each of these documents is a task in its own right,
so both now sit at `awaiting_commit`: the work product exists and has passed,
and only an authorized commit closes them. What is committable is the two
documents, this file, `docs/status.md`, and the review record — not the
`menubar-experience` implementation anchor, which still has no implementation.

`ux/widget.md` Review Round 1 (2026-08-17): **FAIL**, recorded in
[`reviews/ux-widget.md`](reviews/ux-widget.md). Three findings are local
alignment work, but W-F1 is not: the widget's `Period` parameter and its
three-period comparison need per-period aggregates the App Group projection
does not carry, and a widget has no second way to get them — it cannot invoke
the helper, and deriving them in Swift is what `requirements.md` and
`ux/menubar.md:55` both forbid.

That finding reopens two documents this topic had already closed. The same
projection gap sits under `ux/menubar.md:754`'s period switcher, whose row
claims `today`/`7d`/`30d` are provisioned; they are not, and Re-review Round 13
passed that row by checking it against three documents that all repeat the same
unprovisioned claim instead of against the projection itself. Round 14 of
[`reviews/menubar-experience.md`](reviews/menubar-experience.md) withdraws that
`PASS`. Both `Review` cells are unticked again and both CEv1 gates are back to
`FAILED`. Commit `10ce01e` is not reverted: the repairs it carries are real, and
only the gate-closing conclusion drawn from them was wrong.

`ux/widget.md` Re-review Round 3 (2026-08-17): **FAIL**. Round 2 took the
user's chosen path and extended the App Group projection to carry per-period
totals and per-period model shares, plus the two rhythm-day fields, and it
also corrected `ux/menubar.md:754`'s mechanism wording rather than letting the
extension merely make the old sentence true. W-F1 through W-F4 are closed on
the elements they named. Two same-source residuals keep the document open:
W-F5, `composition` large's per-client subtotals are still single-period while
`composition` accepts a `Period`; and W-F6, the two new rhythm fields are
provisioned over a 90-day window while `rhythm` displays a 30-day one. Both are
the recurring shape in this topic — the repair answered the finding's line
numbers instead of the set of elements the same decision governs.

`architecture.md` and `ux/menubar.md` were both edited by that repair, so on
top of being reopened by Round 14 they now carry content states no re-review
has judged. Their independent re-review should run after W-F5 and W-F6 close,
since W-F5's fix most likely lands in `architecture.md` again.

`ux/widget.md` Re-review Round 5 (2026-08-17): **FAIL**. W-F5 and W-F6 are
closed, and closed well — the per-client bullet was split by consumer rather
than changed uniformly, because `composition` takes a `Period` and `trust` does
not, and the rhythm sentence that made the window ambiguous was rewritten
rather than just renumbered. Two new findings: W-F8 (blocking) — `trust` shows
per-tier **amounts** at every size while the projection provisions per-tier
**counts**, and the document's own Data requirements row asks only for counts,
so the target contradicts itself; and W-F7, a citation to an `architecture.md`
revision ordinal that does not exist by that file's own numbering.

W-F8 is the third appearance of one problem: a displayed element whose shape
the projection does not carry. `Period` exposed the first, the shared bullet
the second, and `trust` — governed by neither — the third. The convergent fix
is to map every Data requirements row one-to-one onto a projection bullet and
check the field's *shape*, not its name; "attribution counts" matches
"attribution counts" by name while money and cardinality are different data.

`ux/widget.md` Re-review Round 7 (2026-08-17): **FAIL**. W-F7 and W-F8 are both
closed — the quality tiers now carry `(cost, tokens, count, share)`, the same
shape as the projection's other per-dimension breakdowns, and both documents
now share one explicit revision sequence instead of two disagreeing ordinals.
Running the row-to-bullet shape mapping Round 5 prescribed then found W-F9 on
the one governing dimension no round had swept: `Client` takes `all`, `codex`,
or `claude` on every widget, while the projection keys only three things by
client. At `Client = codex`, `composition` and `rhythm` have no data at any
size, and `magnitude` keeps its cost and tokens but loses its chart, `avg/day`,
`peak`, and session count.

`ux/widget.md` Re-review Round 9 (2026-08-17): **FAIL**, with no serious
finding. W-F9 is closed, and closed most thoroughly of the series: the two
cross terms are provisioned as products rather than as single dimensions, the
ceilings were restated per scope with truncation held inside each scope so a
busy client cannot eat another's budget, and the table gained a `Varies by`
column that turns the next check of this kind from reading the document into
reading one column. The choice between the two paths was made after measuring
the cost (906 entries against 309) rather than deferred a second time. A
thirty-six cell enumeration of `Client` x `Period` x kind x size found no cell
outside the projection. What keeps the document open is W-F10: `Cost
incomplete` is a label this document displays and specifies, yet it has no Data
requirements row and no stated client scope, and the same repair left
`architecture.md` describing per-period totals in two overlapping bullets where
only the unscoped one mentions pricing completeness. One table row and one
bullet merge close it.

Recorded against `architecture.md` rather than this document: its
sixth-revision paragraph says nine bullets gained a client scope, while five
did and its own following sentence names six things. That closes in
`reviews/menubar-experience.md`, whose gates have been open since Round 14.

`ux/widget.md` Re-review Round 11 (2026-08-17): **FAIL**. W-F10 is closed by
merging the two per-period totals bullets into one client-scoped cell that
carries counts, session count, pricing completeness, and cost strings together,
so `Cost incomplete` now qualifies the number beside it; the table gained the
matching row. The repair also removed the aggregate session availability/count
bullet after checking for consumers, and that check holds independently — the
projection is read by the widget alone, the menu bar reads the wire snapshot.
W-F11 keeps the document open: the timeline's refresh-after reads "the
projection's next suggested refresh time", which the projection does not carry;
that field lives in the wire snapshot, and the projection list is a *may contain
only* enumeration, so the timeline's stated input is not merely absent but
disallowed. One bullet and one row close it.

W-F10 and W-F11 came from the same sweep at different breadths — Copy table and
widget bodies the first time, Timeline and Accessibility added the second. The
completeness test for the Data requirements table is therefore "every place this
document says it reads the projection has a row", not "every visible element has
a row". No section that specifies reading remains unswept.

`ux/widget.md` Re-review Round 13 (2026-08-17): **PASS**, and its `Review` cell
is ticked. W-F11 closed by projecting the next suggested refresh time beside
`generated_at` — the scalar the wire snapshot already carries — so the timeline
has the baseline its clamp needs without a new refresh policy. All eleven
findings are closed with no regression, and a full pass over every section that
specifies reading the projection found nothing further.

Six of those eleven were one problem: a displayed field the projection did not
carry. What ended it was not effort but the test getting wider each round —
from the named lines, to the element set one decision governs, to a row-by-row
shape mapping, to the full `Client` x `Period` x kind x size enumeration, to
every place the document says it reads the projection. `architecture.md` was
revised seven times across those rounds, all driven by this document.

`architecture.md` and `ux/menubar.md` stay unticked. None of those seven
revisions has been independently re-reviewed, and both documents have been
reopened since `reviews/menubar-experience.md` Round 14. They should be
re-reviewed together, because they bind to the same projection contract.

`menubar-experience` Re-review Round 15 (2026-08-17): **FAIL**. What Round 14
reopened is not closed — it was answered on the wrong side of a boundary. The
thirteen widget rounds extended the App Group projection seven times, but the
menu bar does not read the projection: that cache is written by the host and
read by the widget, while the menu bar renders the wire snapshot, whose contract
says only "bounded usage summary and pricing completeness" and whose menu-bar
extension adds nothing but `provider.candidates`. Ten of the fourteen rows in
`ux/menubar.md`'s Data requirements table have no field in the contract they
actually read, and the row Round 14 named now cites the projection by name.
M-F2 and M-F3 are the two cross-target items this record already owned: the
period switcher's governed scope is unstated where the client tabs' is explicit,
and `architecture.md`'s sixth-revision paragraph still miscounts its own bullets.

No regression: `architecture.md` changed only inside the projection section, so
the switch surface, envelope, ownership and transition table this record cleared
at Rounds 7 through 13 are byte-identical, and `ux/menubar.md` changed by one
line.

The lesson differs from the widget record's. There the test was "every place the
document says it reads the projection has a row." Here the prior step failed:
before checking whether a row is provisioned, establish which contract the
surface reads. The two surfaces share a topic, a vocabulary, and a four-section
structure, but not a data path.

Repair Round 16 (2026-08-17) closes M-F1 through M-F3 in the candidate documents.
The wire now carries additive `usage.presentation` analytics produced once by Go
for the complete `Client` × `Period` product, with explicit bounds, partial-family
availability, quality/cost shape, daily and rhythm data, pricing coverage, and
per-client subtotals. The host renders the menu bar from that wire object and
only copies its allowlisted values into the widget projection; it does not read
the projection back or aggregate in Swift. The period switcher now explicitly
governs Magnitude and Composition, while Trust stays current-period and Rhythm
stays last-30-days. The sixth-revision note no longer makes a stale bullet-count
claim. Both Document `Review` cells and their CEv1 gates remain open until an
independent re-review closes the findings.

Independent Re-review Round 17 (2026-08-17): **FAIL**, with no serious finding.
M-F1, M-F2 and M-F3 are all closed, and M-F1 was closed on the correct side of
the boundary — the menu bar was not pointed at the widget's cache to fit repairs
already made; the wire snapshot gained the analytics instead, with Go
aggregating once and Swift-side summing, regrouping, and share calculation
forbidden. All ten previously unprovisioned rows map to a named field with a
bound, and the claim that the extension is additive without raising
`wire_version` was verified against `internal/desktop/desktop.go`. The R9-F2
line — granularity, then who produces the three periods, then which contract
they come from — is closed.

M-F4 keeps the record open: M-F2's new sentence says both fixed-window sections
state their window in their heading, and the ATTRIBUTION specimen does not.
That is this record's third instance of prose moving while the specimen
attached to it stays behind, after R9-F1 and R11-N2. One specimen cell or one
sentence closes it.

Repair Round 18 (2026-08-17) closes M-F4 in the candidate UX document. The Trust
row now says its amounts and pricing coverage are current-period, and the
ATTRIBUTION specimen heading says `today`, while RHYTHM continues to say
`last 30 days`. The changed specimen row retains the established 46-column
width. Both Document `Review` cells and CEv1 gates remain open until independent
re-review.

Independent Re-review Round 19 (2026-08-17): **PASS**. M-F4 closed, and all
seventeen findings on this record are closed with no regression, so both
`architecture.md` and `ux/menubar.md` have their `Review` cells ticked and their
CEv1 gates `VERIFIED`. R9-F2 is finally closed through M-F1 rather than through
one more repair on the widget's side of the boundary.

Nineteen rounds are worth two notes. Keeping `architecture.md` and
`ux/menubar.md` as one review subject was right: nine of the seventeen findings
span both documents, and reviewed apart each side reads as self-consistent while
the contract and the surface disagree. And the record kept failing in two
shapes — prose moving while the specimen or copy attached to it stayed behind
(R9-F1, R11-N2, M-F4), and checking a "provisioned" claim against the wrong
contract (R9-F2, three times). The defences are to re-read what a prose change
drives, and to establish which contract a surface reads before checking any row.

Three documents have now passed. `tasks.md` is the last, and it is reviewed last
by design: the task matrix stays a draft until the documents it rests on pass.

`tasks.md` Review Round 1 (2026-08-17): **FAIL**, recorded in
[`reviews/tasks.md`](reviews/tasks.md). The two status matrices, the verification
routing, task 6's release boundary and the dispatch prerequisites all hold. What
fails is task 3's own description, which was written before the Round 8 redesign
and still asks for the five-section surface and the six-state model that
`ux/menubar.md` replaced (T-F1), and which names only `provider.candidates` where
the menu-bar wire extension now also defines `usage.presentation` — the larger,
unimplemented object, whose owner and its effect on task 1's closed review are
left unstated (T-F2). T-F3 and T-F4 are the same cause at smaller scale: the
task 3/task 4 independence claim and a stale "reopened" sentence were not re-read
when the contracts changed.

Re-review Round 3 (2026-08-17): **FAIL**, with no serious finding. T-F1 through
T-F4 are all closed, and closed by pointing at the contracts rather than by
updating the restatement — task 3 now cites `ux/menubar.md#sections`,
`#presentation-state`, and the wire extension, owns both additive wire objects
with their fixtures and decoder tests, and no longer repeats document status.
T-F5 keeps the record open: task 4 restates the vocabulary `ux/widget.md`
retired — `stale age`, `unavailable-host`, and timeline entry points described
as states — and is the only in-flight task with no contract pointer at all.

Round 1 missed it because the contract comparison was run against task 3 only.
That omission is itself the lesson this topic keeps relearning: a check worth
running is worth running against every object of its kind, not just the one that
exposed the problem.

Re-review Round 5 (2026-08-17): **PASS**, and the `Review` cell is ticked. T-F5
closed the same way task 3 was: a `Contracts:` pointer to
`ux/widget.md#surface-and-qualifiers` and `#timeline`, and the state/entry-point
enumeration replaced by "implement from those contracts, and do not introduce a
parallel state vocabulary". All four anchors this file cites were verified to
exist, a sweep of the whole Task breakdown found no remaining retired
vocabulary, and T-F1 through T-F4 show no regression.

All five of this topic's documents have now passed. The five findings on this
record were one defect wearing five faces — a task description restating a
contract, and nobody re-reading the restatement when the contract changed — and
they close by the same move: replace the restatement with an anchor and keep
only what the contract does not carry, namely the task boundary and its
verification level. What must be re-checked after a future contract rewrite
shrinks from every bullet to whether a heading was renamed.

This is the topic's third instance of one shape: R9-F1 was a specimen left behind
by prose, M-F4 a heading left behind by prose, and this is the dispatch
instruction left behind by the contract. A task description reads like a to-do
list rather than document content, which is exactly why it gets missed when a
contract it restates is rewritten.

`tasks.md` Repair Round 2 (2026-08-17) closes T-F1 through T-F4 in the candidate
document. Task 3 now delegates presentation shape and state semantics to the
approved UX anchors, owns the Go/fixture/Swift-decoder delivery for both additive
wire objects without reopening task 1, and no longer repeats stale document
status. Task 4 now depends on task 3's `usage.presentation` producer as well as
the foundation. The `tasks.md` `Review` cell remains unchecked pending
independent re-review.

`tasks.md` Repair Round 4 (2026-08-17) closes T-F5 in the candidate document.
Task 4 now points directly to `ux/widget.md`'s Surface and qualifiers and Timeline
contracts, and its delivery item requires those surface, qualifier, and timeline
entry-point behaviors without restating or inventing state vocabulary. The
`tasks.md` `Review` cell remains unchecked pending independent re-review.

The target was documents, not task 3, which has no implementation. Task 3 stays
blocked until this topic's remaining documents pass, since a task matrix is a
draft until they do.

`menubar-experience` development (2026-08-19): implementation complete, `Dev`
**unticked**. The app target now carries the view model that is the only
converter from coordinator state to presentation values, the reading surface
with its client and period filters, four filtered panels, the unfiltered rhythm
block, the notice strip and its health detail, the provider footer and switch
flow, the settings window with its four preferences, the menu-bar item with its
value and scope modes and its own right-click and double-click menu, an `en` and
`zh-Hans` string catalog covering every shipped key, and the `AgentDeckAppTests`
target wired into `AgentDeck.xcscheme`. The withdrawn update check is gone from
the code, the menu, the preferences, and the copy, and the app target now makes
no network request. The cell stays unticked because this task's selected
verification level is L3 and its runtime half cannot run here: this machine has
Command Line Tools only, so `xcodebuild` is absent and
`bash scripts/test-macos-app.sh` takes its non-Xcode branch, which cannot name
`AgentDeckAppTests` or report a test count. What did pass is recorded with the
task's Beads handoff; what remains is stated there as the exact blocked command.

`desktop-widget` Review Round 1 (2026-08-21): **FAIL**. The large-scale Widget
delivery shape is present — four embedded WidgetKit families, intent
configuration, a redacted App Group reader, timeline refresh, localization,
isolated tests, and the static sandbox boundary — but DW-R1-F1 is release
blocking. The complete fixture carries 1,800 unattributed tokens with incomplete
pricing, while the Trust Widget reads only zero cost and nullable share, so its
small surface prints `—` and its deeper surfaces print zero/unknown instead of
the available attribution amount and incomplete status. The `Review` cell stays
unchecked. Direct `WidgetKit` use from `AgentDeckShared` is explicitly accepted
by the user and is not a finding. Broad product verification stopped after the
decisive reproducer; the exact candidate and repair boundary are recorded in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md).

`desktop-widget` Re-review Round 2 (2026-08-21): **PASS** over the recorded
findings. DW-R1-F1 is closed and no regression was found. The `Review` cell
stays unchecked because this task's own acceptance conditions have no observed
results: rendered acceptance of the twelve configurations at the true canvas,
`en` / `zh-Hans` without truncation at every size, and the runtime half of the
sandbox proof. The completion gate for `desktop-app:desktop-widget` is
`NOT_VERIFIED`, so the task stays in review rather than reaching a commit
checkpoint. Round content is in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md).

`desktop-widget` macOS 26 manual acceptance Round 3 (2026-08-21): **FAIL**. The
twelve configurations do render on a real desktop at true canvas, but all twelve
render the unavailable state: the sandboxed extension's App Group request is
rejected by `containermanagerd` because the identifier carries no team-ID prefix,
so the widget cannot reach its own projection in the shipping signing form.
DW-R3-F1 is P1 and blocks the other seven checklist items. `DW-R1-F1` stays
closed. The `Review` cell stays unchecked and the completion gate stays
`NOT_VERIFIED`. Round content is in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md).

DW-R3-F1 is **parked** (2026-08-21, user decision): the fix needs an Apple
Developer team ID that does not exist yet, which is an external prerequisite
rather than a code decision. The finding stays open, task 4 stays unreviewed, and
its Beads task is `deferred` with the resume condition recorded. Dispatch moves
to task 5.

DW-R3-F1 Repair resumed (2026-08-23): the user supplied Team `N2FZ2FNRTU` and
confirmed a matching Developer ID Application identity. The source candidate now
uses `N2FZ2FNRTU.group.com.kitdine.agentdeck` through one xcconfig-to-entitlement
and Info.plist injection path; the host writer and Widget reader no longer compile
duplicate identifier literals. Shared 39/39, App 52/52, Widget 12/12, the Widget
sandbox gate, and the isolated distribution test pass. A current-HEAD `v0.5.0`
universal candidate is Developer ID signed with matching host/Widget entitlements.
After explicit authorization, that candidate replaced the installed app. The
new container did not exist before launch; the host then published a schema-v1
projection with usage, presentation, and sessions available. `containermanagerd`
approved both host and Widget requests, `chronod` successfully produced all
twelve kind-by-size timelines, and twelve privacy-bounded window captures show
data instead of `unavailableSurface`. Repair is complete; the finding and
`Review` cell remain open for independent Re-review.

`desktop-widget` prototype-alignment Repair (2026-08-24): before independent
Re-review, the user found that the native implementation preserved the data
hierarchy but did not implement the repository prototype's actual per-size
compositions. `prototype/src/Widgets.jsx` and its Widget CSS are now the direct
source: semantic Usage/Breakdown/Attribution/Activity headers, scope and updated
footer, 7/20/90 trend treatment, large Magnitude area/axis/stats, Composition
share tracks/token stack/client chip, Trust quality/provider/unpriced layers, and
Rhythm axes/legend/Monday-first hour grid/90-day calendar each map to the matching
small/medium/large SwiftUI branch. The former monotonic-depth test was replaced
with exact source-section contracts; twelve dark render attachments and the full
105-test macOS gate pass. This candidate has not replaced the currently installed
signed app during the Repair itself. A separately authorized installation later
replaced it with the same-source Developer ID candidate; `chronod` successfully
reloaded all twelve standard kind-by-size timelines from the new extension.
No AgentDeck Widget is currently pinned on the desktop, so installed light and
`zh-Hans` visual acceptance remains for independent Re-review rather than being
inferred from gallery snapshots.

`desktop-widget` DW-R11 Repair Round 12 (2026-08-24): the four Round 11 findings
are repaired in the candidate. Every Widget metric is now an AX label/value
element, charts and categorical tracks carry summaries, decorative SF Symbols
are hidden, and Trust provider names precede their values. A standalone
WidgetCenter reload hook plus byte-safe backup/restore procedure makes checklist
items 6 and 7 executable. Composition large uses bounded spacing before client
subtotals. Grayscale evidence is a physical-display observation, and production
View tests now run under explicit Xcode test languages; fixing WidgetCopy's
resource-bundle ownership made the twelve `zh-Hans` attachments actually render
Chinese. The final explicit-English macOS suite passes Shared 39/39, App 52/52,
and Widget 17/17; the sandbox gate and reload-hook check pass. No installed app,
projection, system setting, or real timeline was changed. The `Review` cell and
completion gate remain open for independent Re-review of DW-R11-F1 through F4
and the still-open DW-R3-F1.

`desktop-widget` Repair Round 18 (2026-08-24): DW-R15-F1 and DW-R15-F2 are
repaired in the candidate. The misleading Round 14 attachment directory was
renamed, and the current candidate has independently located 12-image `en` and
`zh-Hans` evidence sets. Trust large now always anchors its pricing summary:
unpriced details when incomplete, pricing coverage when complete. The full
macOS suite passes 108/108, the Widget sandbox gate passes, and a Developer ID
signed universal `0.5.0 (2)` candidate is installed. The DW-R3-F1 unified run
obtained current-build Light/Dark evidence for all twelve windows and a valid
six-hours-old observation for all twelve, then restored the projection bytes,
mode, appearance, and running host. It remains incomplete: Accessibility TCC
blocks Dynamic Type, AX/VoiceOver, contrast, and gallery automation; physical
grayscale still needs a human observer; managed approval separately refused
moving the real projection for host-absent acceptance. The `Review` cell and
completion gate remain open; exact evidence and recovery state are in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md) Round 18.

`desktop-widget` Repair Round 20 (2026-08-24): DW-R19-F1 and DW-R19-F2 are
repaired in the source candidate. The footer now renders one age-dependent
freshness value — `Updated <relative>` or `Last updated <relative>` — and keeps
only non-age qualifiers alongside it, with exact `en` / `zh-Hans` regression
coverage. Trust large no longer inserts a flexible spacer between provider rows
and the pricing summary; a production-view rendering shows the former roughly
160-point gap reduced to an explicit 10-point section gap. Widget tests pass
18/18 and the static sandbox boundary passes. The full scheme remains non-green
because one out-of-scope App test expects English provider-option copy while its
test host resolves Simplified Chinese; both aggregate attempts failed only that
same assertion. The installed signed build 2 predates this source repair. The
`Review` cell and completion gate remain open for independent Re-review; Round 20
details and exact scoped hashes are in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md).

`desktop-widget` Re-review Round 21 (2026-08-24): **FAIL**. DW-R19-F1 and
DW-R19-F2 are closed: the current scoped hashes still match Round 20, and an
independent focused XCTest run passed all four Widget copy tests plus the
production-view dark rendering test for all twelve configurations (5/5 total).
DW-R3-F1 remains
open because the installed signed Build 2 predates the Round 20 source repair:
items 2, 3, 4, 5 and 7 were never completed, while the user-visible footer and
Trust changes invalidate that installed candidate's item 1, 6 and 8 evidence for
the current source. The fixed Profile v1 gate query therefore reports eight
required criteria without current-candidate passing evidence and returns
`NOT_VERIFIED`. The `Review` cell stays unchecked, and the exact disposition is in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md) Round 21.

`desktop-widget` Repair Round 22 (2026-08-24): the user corrected the large
bottom-alignment requirement for the earlier DW-R11-F3 and DW-R19-F2 repairs.
Composition `ClientSubtotals` and both Trust pricing-summary branches now use a
collapsible spacer so their bottom information elements align immediately above
the footer; the existing 10-point minimum section gap remains when content grows.
The twelve-view dark rendering, a new two-view focused bottom-anchor rendering,
and the largest Dynamic Type rendering each pass their selected XCTest. The
`Review` cell and completion gate remain open for independent Re-review; exact
hashes and evidence are in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md) Round 22.

`desktop-widget` DW-R3-F1 unified Repair Round 23 (2026-08-24): the final source
candidate adds the missing largest-Dynamic-Type family degradation and replaces
the non-transporting interpreted reload request with an installed-host acceptance
mode that exits before refresh or projection publication. Developer ID signed
build 2 is installed and byte-matches the candidate. Checklist items 1, 6, 7,
and 8 have current-candidate observations; production-view evidence additionally
covers the item 2 family degradation and item 5 placeholders. The user made the
final acceptance decision that items 2–5 are not severe, must not receive further
real-environment execution, and are accepted for this candidate without it. The
full Widget target passes 22/22, the static sandbox and isolated distribution
checks pass, projection/settings/process state is restored, and exact evidence is
under the private Round 23 evidence root named in
[`reviews/desktop-widget.md`](reviews/desktop-widget.md). The exact candidate's
CEv1 gate is `VERIFIED` with 14/14 required criteria; the `Review` cell remains
open for independent Re-review.

`desktop-widget` independent Re-review Round 24 (2026-08-25): **PASS**. The
current source fingerprint and installed App / Widget / helper hashes still match
the Round 23 candidate. `DW-R3-F1`, corrected-requirement `DW-R11-F3`, and
`DW-R19-F2` are closed; all earlier findings remain closed and no new finding is
recorded. The exact candidate gate is `VERIFIED` at 14/14, and the final
review/status-synchronized uncommitted state is recorded separately in CEv1. The
`Review` cell is ticked and task 4 reaches its commit checkpoint. Task 6's
task-4 precondition is now satisfied and its final reconciliation follows below.

`desktop-app-contract` Review Round 1 (2026-08-23): **REOPEN** on three findings,
repaired the same day. R1-F1 (P1) recorded that the two halves of this task ran
in the wrong order; R1-F2 and R1-F3 are recorded with their repairs in
[`reviews/desktop-app-contract.md`](reviews/desktop-app-contract.md) Round 2.

**The ordering, corrected.** This task's second bullet puts the topic's own
document set first — reconciled, then reviewed once — because a specification
written from a set that still disagrees with the build carries the disagreement
into `docs/specs/`. The first development pass reconciled the set but wrote the
living-spec text without waiting for the deferred review, so the ordering was
inverted rather than observed. The set is now reconciled **and submitted**: the
deferral note in each of the six document records says so, and the closing round
is what those records are waiting for.

What follows from that, and it constrains this task's own Review: the
living-spec text in `docs/specs/cli-design.md` and `docs/specs/cli-manual.md` is
**provisional to the deferred document review**. If that review changes the
topic's document set, the specification text derived from it changes with it.
Task 6 therefore cannot reach Review PASS before the deferred document review
has passed — the contract's ordering is restored by making the dependency
explicit and blocking on it, not by claiming the step happened.

`desktop-app-contract` development (2026-08-23): **COMPLETE**, and it ran with
task 4 still unreviewed. The task's own contract makes every prior task's Review
PASS a precondition; tasks 1, 2, 3 and 5 satisfy it, task 4 does not and cannot
until an Apple Developer team ID exists. That was reported as a blocker and the
user then decided explicitly to proceed without task 4. The deviation is
recorded here rather than absorbed silently, because task 6's Review has to
judge the reconciliation knowing the widget contract was written against a
surface whose runtime acceptance never passed.

What that cost is bounded and located. The widget's *specified* behavior is
unchanged and remains the contract; what could not be confirmed against a
working desktop is that the specification matches observed behavior, because
observed behavior is the unavailable state everywhere. Both documents that
carry a widget contract now say so at the point where a reader would otherwise
assume otherwise — `architecture.md`'s identifiers section and `ux/widget.md`'s
freshness table — and `docs/specs/cli-design.md` and `cli-manual.md` carry the
same disclosure rather than describing a widget that works.

Reconciled in this task: `architecture.md` (the App Group identifier's runtime
refusal, and the removal of update-channel metadata task 5 never delivered);
`ux/widget.md` (the shipped-state disclosure); `docs/specs/cli-design.md` (the
GUI non-goal restated as a CLI-scope boundary, the signing/notarization
non-goal narrowed to the CLI archives, the withdrawn update check, a new
Desktop Application section, and the desktop release channel);
`docs/specs/cli-manual.md` (the same withdrawal plus the desktop application
and Cask installation sections); and `docs/status.md`'s stage row.

Verified current and left unchanged: `requirements.md`, `ux/menubar.md`, and
`ux/settings.md` already record the update-check withdrawal, the four
preferences, and the twelve widget configurations, because this topic writes
each change into the document that owns it as the change is made.

Identity is cited, not re-derived, exactly as this task's contract requires.
`package-macos-app.sh` refuses to sign unless the bundle's
`CFBundleShortVersionString` equals the tag without its `v` and the embedded
helper reports the exact tag, and the Cask's version is rendered from the same
tag, so app, CLI, and package identities agree by construction rather than by
inspection. The wire contract is pinned at `1` on both sides —
`DesktopSnapshotV1.wireVersion` in Swift against the Go contract. The evidence
is task 5's Round 5 `make release-verify` (exit 0) run against this content
state; task 6 ran no build.

Not done here, by contract: the specification version is not raised and no
changelog row is added. `docs/specs/cli-design.md`'s changelog rule requires
both when promised behavior changes, and this task's edits do change it — that
row is [`v0-5-0-contract`](../v0-5-0-contract/tasks.md) task 2's, which raises
the version exactly once across every topic in the version and lands on top of
the feature-contract text this task just wrote. Row 25's mention of an
"opt-in privacy-bounded stable-release update-check" is the historical record of
what version 25 promised and is deliberately left as written; the withdrawal
belongs in the new row.

Next action:

```text
复评：desktop-app / reviews/desktop-app-contract.md
```

The closing pass the whole topic deferred has since run and **passed** on
2026-08-23: CD1-F1, CD1-F2 and CD1-F3 are closed, all six documents carry `[x]`
in the `Closing review` column, and the round is recorded in each of the six
records. Task 6's R1-F1 dependency on it is therefore lifted.

What still stands between task 6 and Review PASS is the temporary
code-over-contract rule above: it is retained while task 4's open P1 `DW-R3-F1`
leaves the implementation and the document set in disagreement, and only task 4
closing that finding lets task 6 run the final reconciliation that removes it.
The Apple Developer prerequisite is now satisfied and task 4's Repair is active;
its signed candidate has passed real WidgetKit/App Group Repair acceptance, and
the subsequent prototype-alignment candidate now awaits independent Re-review.

`desktop-app-contract` independent Re-review Round 7 (2026-08-23): **REOPEN** on
P1 `R7-F1`. Task 4's Team-prefixed runtime repair moved the delivered Widget
behavior, while the two living specs task 6 owns still said the Team ID did not
exist, the App Group lacked its prefix, and all twelve configurations rendered
unavailable. Repair Round 8 (2026-08-24) aligns both disclosures with the topic
documents: prerequisite satisfied, installed signed candidate has approved host
and Widget container access, all twelve configurations render data, and
independent Re-review still owns closure. Task 6 remains gated by task 4's
unchecked Review cell; this repair does not remove the temporary rule early.

`desktop-app-contract` independent Re-review Round 10 (2026-08-25): **FAIL** on
P1 `R10-F1`. Task 4's Round 24 PASS satisfies the former structural prerequisite,
but task 6 has not yet run the final reconciliation that its own temporary
code-over-contract rule requires: that active rule remains at the top of this
file, and both living specs still describe DW-R3-F1 as awaiting independent
closure. R1-F1/F2/F3, R3-F1 and R7-F1 remain closed; task 4 stays PASS. Task 6's
`Review` cell remains unchecked pending the bounded document-only repair recorded
in [`reviews/desktop-app-contract.md`](reviews/desktop-app-contract.md) Round 10.

`desktop-app-contract` Repair Round 11 (2026-08-25): **COMPLETE** for P1
`R10-F1`, awaiting independent Re-review. The task-4-triggered final
reconciliation removes the temporary code-over-contract rule and changes both
living-spec Widget disclosures from a pending-review boundary to the final
delivered facts. R1-F1/F2/F3, R3-F1 and R7-F1 remain closed; task 4 remains PASS,
and task 6's `Review` cell remains unchecked until independent Re-review.

`desktop-app-contract` independent Re-review Round 12 (2026-08-25): **PASS**.
P1 `R10-F1` is closed: the temporary rule remains absent, both living specs state
the final delivered Widget facts, and the task/topic status is reconciled. All
earlier findings remain closed. The exact review/status-synchronized candidate's
completion-evidence/v1 Task gate is `VERIFIED`; task 6 reaches its commit
checkpoint. The containing desktop-app completion gate remains a separate outer
boundary and is not inferred from this Task verdict.

`desktop-app` Unit completion checkpoint (2026-08-25): **VERIFIED**. The Plan
WorkUnit contains all six Task WorkUnits; each child has a non-vacuous `VERIFIED`
Task gate, and the Plan's three cross-Task criteria cover six-Task review/evidence
closure, cross-surface contract reconciliation, and the shared security, identity,
and distribution boundary. This closes the topic's development/review unit only.
Tasks 4, 5, and 6 still await their own authorized commit checkpoints; no commit,
push, preflight, release-channel decision, or publication is implied.

`desktop-app` delivery checkpoint (2026-08-25): **COMMITTED** as three signed,
feature-scoped commits: `7b743d0` (menu-bar usage presentation), `b359850`
(WidgetKit, App Group, and packaging boundary), and `0aefed1` (topic contract,
review history, and Unit checkpoint). The final immutable tree
`87cc09d58b154eb1a83c4713744a1fe78f1c91bb` has non-vacuous `VERIFIED` gates
for task 4 (14/14), task 5 (4/4), task 6 (4/4), and the Plan (3/3). Tasks 4–6
are delivered; no push, technical preflight, release-channel decision, or
publication was authorized or performed.

The previous actions, now complete:

```text
开发：desktop-app / unified-desktop-distribution
开发：desktop-app / desktop-app-contract
评审：desktop-app / desktop-app-contract
修复：desktop-app / reviews/desktop-app-contract.md / R1-F1, R1-F2, R1-F3
```

Task 1 was blocked on the `v0.4.0` session DTO contract; that dependency is now
satisfied. Task 2 consumes task 1. Task 3 depends on task 2. Task 4 depends on
tasks 2 and 3 because its App Group usage projection consumes the
`usage.presentation` producer task 3 delivers; it does not start as an
independent sibling. Task 5 integrates tasks 2-4. Task 6 runs last within this
topic, and in turn gates the
[v0.5.0 contract closure](../v0-5-0-contract/tasks.md).

Commit boundaries follow task boundaries. This topic does not authorize commits,
pushes, certificate creation, secret changes, release publication, Homebrew tap
changes, local installation, or external distribution.

`unified-desktop-distribution` Review Round 1 (2026-08-23): **FAIL**. The
universal build, inside-out signing, artifact assembly, the two isolated test
suites, the tap Cask channel, the workflow checkers, and the aggregate-gate
wiring are all present and their gates pass. R1-F1 is release blocking: the
rendered Cask declares `conflicts_with formula:`, a key current Homebrew rejects,
so the Cask cannot be loaded at all. R1-F2 is release blocking: the notarization
profile is stored in a run-scoped keychain that `notarytool submit` is never told
to read. Three further findings cover the Cask tests asserting their own
template, the unstapled direct-download ZIP, and a deprecated `depends_on macos:`
form. The `Review` cell stays unchecked. Round content is in
[`reviews/unified-desktop-distribution.md`](reviews/unified-desktop-distribution.md).

`unified-desktop-distribution` Re-review Round 5 (2026-08-23): **PASS**. All
seven findings from Rounds 1 and 3 are closed, no new finding was recorded, and
`make release-verify` exited 0 on this exact content state rather than being
reused from the pre-repair run. The `Review` cell is ticked. Task 6 remains
blocked on task 4, which is parked on an external prerequisite. Round content is
in [`reviews/unified-desktop-distribution.md`](reviews/unified-desktop-distribution.md).

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
开发：`desktop-app` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the current release and
versioning contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`reviews/<task-anchor>.md` before ticking `Review`.
