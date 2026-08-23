---
status: active
topic: desktop-app
subject: unified-desktop-distribution
---

# Review log — desktop-app / unified-desktop-distribution

## Round 1 — 2026-08-23

- Reviewed state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`, uncommitted
  working tree. Blob fingerprint of the fifteen reviewed files (the concatenated
  `git hash-object` list):
  `94bdd6f9329164b7be12969bbed59a3b7c6d8d9468a7837a90bf3dff0fd53002`.
- Reviewer: Claude Code (independent of the implementing session).
- Method: read the full diff and the five created files against `tasks.md`
  task 5 and `architecture.md`'s Distribution sections; re-ran the isolated
  gates; and independently exercised the rendered Cask against the locally
  installed Homebrew 6.0.18 and `notarytool`'s own help output, because the
  task's own tests assert the Cask's text rather than Homebrew's acceptance of
  it. No project checker exists for a Homebrew artifact's validity; `ruby -c`,
  which `render-homebrew-cask.sh` runs, checks syntax only.
- Scope: `scripts/package-macos-app.sh`, `scripts/render-homebrew-cask.sh`,
  `scripts/test-macos-distribution.sh`, `scripts/test-cask-migration.sh`,
  `packaging/homebrew/agentdeck-app.rb.tmpl`, `scripts/build-macos-app.sh`,
  `scripts/update-homebrew-tap-pr.sh`, `scripts/test-release-distribution.sh`,
  `scripts/check-release-workflow.rb`,
  `scripts/check-release-preflight-workflows.rb`, `Makefile`,
  `apps/macos/Config/AgentDeck.xcconfig`, and the desktop and cask jobs in
  `.github/workflows/{ci,release,release-preflight}.yml`.
  `scripts/check-widget-sandbox.sh` is task 4's file; only its `Makefile` wiring
  was reviewed here.
- Findings:
  - [P1] R1-F1 `packaging/homebrew/agentdeck-app.rb.tmpl:21` — the rendered Cask
    is invalid. Homebrew's `Cask::DSL::ConflictsWith::VALID_KEYS` is `[:cask]`
    and `assert_valid_keys` raises on any other key, so `conflicts_with formula:`
    makes the whole Cask unloadable, not merely inert. Loading the rendered
    stable Cask from a temporary tap returned
    `Error: Cask 'agentdeck-app' definition is invalid: 'conflicts_with' stanza
    failed with: Unknown key: :formula. Valid keys are: :cask`. Every declared
    behaviour of the Cask — app, binary links, completions, caveats, zap — is
    unreachable. -> open
  - [P1] R1-F2 `.github/workflows/release.yml:248-253` and
    `scripts/package-macos-app.sh:182` — the notarization profile is stored into
    the run-scoped keychain with `--keychain`, but `notarytool submit` is invoked
    with `--keychain-profile` and no `--keychain`, which reads the login
    keychain. The release job would fail at notarization with a credential it
    demonstrably wrote. No negative test or workflow checker covers the pairing.
    -> open
  - [P2] R1-F3 `scripts/test-macos-distribution.sh:39` (and the same shape
    throughout section 1) — the Cask assertions `grep` the file the renderer just
    wrote, so they restate the template instead of testing it. They pass on a
    Cask Homebrew rejects outright, which is how R1-F1 reached review green. The
    same blind spot applies to `test-cask-migration.sh`, whose local installer
    parses `conflicts_with formula:` itself. -> open
  - [P2] R1-F4 `scripts/package-macos-app.sh:174` versus `:182-186` — the ZIP is
    assembled before notarization and is never stapled, so the direct-download
    ZIP carries no ticket and fails first launch offline. `architecture.md:197`
    names only the DMG as the direct-download artifact, so the ZIP is also an
    artifact no contract declares. -> open
  - [P2] R1-F5 `packaging/homebrew/agentdeck-app.rb.tmpl:16` —
    `depends_on macos: ">= :tahoe"` is the deprecated string-comparison form and
    warns on every load:
    `Calling string comparison format for 'depends_on macos:' is deprecated!`.
    `MacOSRequirement.parse` already defaults to the `>=` comparator, so
    `depends_on macos: :tahoe` is the same requirement without the warning. -> open
- Consequential document repair (not a separate finding):
  `docs/topics/desktop-app/architecture.md:181-186` asserts that the
  `conflicts_with formula:` declaration makes "Homebrew refuse the second
  installation". That is false against current Homebrew and must be repaired
  with R1-F1 rather than left as the contract the fix is judged against.
- Evidence:
  - `bash scripts/test-macos-distribution.sh` → PASS (33s)
  - `bash scripts/test-cask-migration.sh` → PASS
  - `bash scripts/check-widget-sandbox.sh` → PASS
  - `bash scripts/test-release-distribution.sh` → exit 0
  - `ruby scripts/check-release-preflight-workflows.rb .github/workflows/release-preflight.yml .github/workflows/release.yml` → exit 0
  - `make check-whitespace` → exit 0
  - Rendered `agentdeck-app` v1.2.3 Cask loaded through Homebrew 6.0.18 in a
    throwaway tap → `CaskInvalidError` (R1-F1) plus the `depends_on` deprecation
    warning (R1-F5); tap removed with `brew untap`.
  - `/usr/local/Homebrew/Library/Homebrew/cask/dsl/conflicts_with.rb`
    (`VALID_KEYS = [:cask]`), `cask/installer.rb:236-250` (only `[:cask]` is
    enforced), `cask/dsl/depends_on.rb:110-113`
    (`MacOSRequirement.parse(args, comparator: ">=")`).
  - `xcrun notarytool submit --help` and `store-credentials --help`: `--keychain`
    is required on both sides to address a non-default keychain (R1-F2).
- Residual risk carried forward, not findings:
  - The `macos-26` runner label is unverified from here, as task 5 already
    records. The first CI run on a branch carrying these workflows confirms it.
  - Task 5 declares a dependency on task 4 `desktop-widget`, which is not at
    Review PASS (parked on an Apple Developer account). The packaging path is
    independently testable, so this review proceeded, but task 5's PASS does not
    substitute for task 4's.
  - The synthetic bundle in `test-macos-distribution.sh` has no `Frameworks`
    directory, so `package-macos-app.sh`'s framework-signing loop is never
    exercised in isolation; the real build's framework is signed only in the
    unstubbed local run.
- Verdict: REOPEN

## Round 2 — 2026-08-23 — repair of R1-F1 … R1-F5

- Repaired state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`, uncommitted
  working tree. Files changed by this round, with their post-repair
  `git hash-object` values:
  - `packaging/homebrew/agentdeck-app.rb.tmpl` `ecb3e191297532e50491b11fb14cbb40181c4776`
  - `scripts/package-macos-app.sh` `00d529fa8f0517257bfccfe1aa5dc1a3ec0c58aa`
  - `scripts/test-macos-distribution.sh` `ce04cf419ff2d99afcb919eab5a93b9889736db0`
  - `scripts/test-cask-migration.sh` `41a3de39108bbff60cb73c3db4eaaa6c95f31d99`
  - `scripts/check-release-workflow.rb` `176b5ffd291c57397b316e9f2d9b2d8902c9b5f2`
  - `.github/workflows/release.yml` `6446c8a8836a684f45a6011ee38b797e593b886b`
  - `docs/topics/desktop-app/architecture.md` `dc4f766533e22daee08a6bd4ecdaa1b7ac307516`
- Repairer: Claude Code, acting on the Round 1 findings only.
- Finding-to-change mapping:

  | Finding | Files changed |
  | --- | --- |
  | R1-F1 | `packaging/homebrew/agentdeck-app.rb.tmpl`, `docs/topics/desktop-app/architecture.md` |
  | R1-F2 | `scripts/package-macos-app.sh`, `.github/workflows/release.yml`, `scripts/check-release-workflow.rb` |
  | R1-F3 | `scripts/test-macos-distribution.sh`, `scripts/test-cask-migration.sh` |
  | R1-F4 | `scripts/package-macos-app.sh`, `scripts/test-macos-distribution.sh`, `docs/topics/desktop-app/architecture.md` |
  | R1-F5 | `packaging/homebrew/agentdeck-app.rb.tmpl`, `scripts/test-macos-distribution.sh` |

  No file outside this table changed, and no unrecorded issue was repaired
  opportunistically.
- Dispositions:
  - R1-F1 — fixed. `conflicts_with` now declares `cask:` alone, which is the only
    key Homebrew accepts. The formula half of the exclusion moved to a `preflight`
    block that aborts with `odie` when `agentdeck` or `agentdeck-rc` is present in
    `HOMEBREW_CELLAR`, and the abort message names the two-command migration
    rather than leaving the user with a link collision. This is a stronger
    guarantee than the invalid stanza claimed: it fires before any artifact is
    linked and says why.
  - R1-F2 — fixed. `package-macos-app.sh` reads `AGENTDECK_NOTARY_KEYCHAIN` and
    passes `--keychain` to `notarytool submit` alongside `--keychain-profile`;
    a keychain path that does not exist is refused in the preconditions, before
    anything is signed. `release.yml`'s packaging step sets that variable to the
    same run-scoped keychain `store-credentials` wrote to, and
    `check-release-workflow.rb` now asserts both sides of the pairing.
  - R1-F3 — fixed. `test-macos-distribution.sh` gained section 4, which copies
    each rendered cask into a throwaway `agentdeck-fixture/cask-fixture` tap under
    the real Homebrew prefix, loads it with `brew info --cask`, and requires the
    load to succeed, to emit no deprecation or warning line, and to enumerate
    every artifact class the cask declares. The fixture tap is removed by the
    script's `trap` as well as on the success path, and a leftover fixture from a
    previous run is refused rather than reused. `test-cask-migration.sh`'s local
    installer now reads the formula exclusion from the preflight declaration, and
    its parser is verified to still return both `agentdeck` and `agentdeck-rc`.
  - R1-F4 — fixed. The ZIP is now assembled after notarization, from the bundle
    `stapler staple` has already amended; the app bundle is stapled and validated
    in addition to the DMG, on the ticket the single submission issues against
    the shared code signature. `architecture.md`'s Direct download section now
    declares both artifacts and states the stapling contract instead of naming
    the DMG alone.
  - R1-F5 — fixed. `depends_on macos: :tahoe`, which `MacOSRequirement.parse`
    reads as the same `>=` requirement without the deprecation warning.
  - Consequential document repair — done. `architecture.md`'s Homebrew channels
    bullet no longer asserts that `conflicts_with formula:` makes Homebrew refuse
    the second installation. It now states that `conflicts_with` accepts `cask:`
    only, and that the formula exclusion is the cask's `preflight` refusal.
- Falsifiability of the new coverage, checked rather than assumed. With the
  Round 1 template restored into the fixture tap, `brew info --cask` exits 1 with
  `Cask 'agentdeck-app' definition is invalid: 'conflicts_with' stanza failed
  with: Unknown key: :formula`, and the warning grep matches
  `Calling string comparison format for 'depends_on macos:' is deprecated!`.
  Both new assertions therefore fail on the defects they were added for; the
  fixture tap was removed afterwards.
- Evidence, all after the final edit:
  - `bash scripts/test-macos-distribution.sh` → `macOS distribution packaging: PASS`
  - `bash scripts/test-cask-migration.sh` → `cask migration and mutual exclusion: PASS`
  - `bash scripts/check-widget-sandbox.sh` → `widget sandbox boundary: PASS`
  - `bash scripts/test-release-distribution.sh` → exit 0
  - `ruby scripts/check-release-workflow.rb .github/workflows/release.yml` → exit 0
  - `ruby scripts/check-release-preflight-workflows.rb .github/workflows/release-preflight.yml .github/workflows/release.yml` → exit 0
  - `make check-whitespace` → exit 0
  - `$(brew --repository)/Library/Taps` contains no `agentdeck-fixture` entry
    after the run.
  - No Go or Swift source changed in this round, so the aggregate L4 gate's Go
    stages are unaffected; `make release-verify` was not rerun here and belongs to
    the re-review or delivery decision.
- Residual risk, unchanged by this round: the `macos-26` runner label is still
  unverified from here; task 4 `desktop-widget` is still not at Review PASS; and
  the synthetic bundle still has no `Frameworks` directory, so the
  framework-signing loop remains unexercised in isolation.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 3 — 2026-08-23 — independent re-review of Round 2

- Reviewed state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`, uncommitted
  working tree. Blob fingerprint of the same fifteen files Round 1 named:
  `e5b4d0931e8b355b1186ef3a2667fd76f0bb82609a8a308c2f55844fa88e1f85`
  (Round 1 was `94bdd6f9…d53002`). Six of the fifteen changed, and they are
  exactly the six Round 2 declared; `architecture.md` is the seventh declared
  file and lies outside the fifteen. No file outside Round 2's table changed, and
  no new untracked path appeared.
- Reviewer: Claude Code, independent of the repair round.
- Method: each Round 1 finding re-verified against the current content by the
  same mechanism that established it, not by reading the repair's account of
  itself. The Cask was loaded through real Homebrew 6.0.18 in a throwaway tap;
  the `preflight` replacement mechanism was traced through Homebrew's own source
  rather than assumed; the workflow checker's new assertions were mutated to
  confirm they can fail; the full evidence set was re-run.
- Disposition of every Round 1 finding:
  - R1-F1 — **closed.** `conflicts_with` now declares `cask:` alone. Loading the
    rendered v1.2.3 Cask from a throwaway tap succeeds (`brew info --cask`
    exit 0) and enumerates every artifact class: `AgentDeck.app (App)`, the
    helper binary, and all three completion links with their targets. The
    formula half moved to a `preflight` block, and that mechanism was verified in
    Homebrew's source rather than taken on trust:
    `Cask::Artifact::AbstractFlightBlock#abstract_phase` runs it as
    `Cask::DSL::Preflight.new(cask).instance_eval(&block)`; `Preflight` includes
    `Cask::Staged`, which includes `::Utils::Output::Mixin`, so `odie` resolves
    rather than hitting `DSL::Base#method_missing`; `HOMEBREW_CELLAR` is a frozen
    `Pathname`, so the `/` operand is valid; and
    `AbstractArtifact.sort_order` places `PreflightBlock` ahead of `App` and
    `Binary`, so the abort does fire before any artifact is moved or linked, as
    the repair claims.
  - R1-F2 — **closed.** `package-macos-app.sh` reads `AGENTDECK_NOTARY_KEYCHAIN`,
    refuses a non-existent path in the preconditions before anything is signed,
    and passes `--keychain` beside `--keychain-profile` on submit.
    `release.yml:261` sets it to `${{ runner.temp }}/agentdeck-signing.keychain-db`,
    the same path `store-credentials` writes to at line 252. Both halves of
    `check-release-workflow.rb`'s new pairing assertion were mutated and both
    rejected the mutation, so the assertion is falsifiable in both directions.
  - R1-F3 — **closed.** `test-macos-distribution.sh` section 4 loads each
    rendered cask through real Homebrew, requires the load to succeed, requires
    no deprecation or warning line, and enumerates the declared artifact classes.
    Section 5's stub stapler writes a `CodeResources.staple-marker` that the ZIP
    extraction then requires, so the ordering in R1-F4 is *tested* rather than
    asserted — the strongest single change in this round.
  - R1-F4 — **closed.** The ZIP is assembled after `stapler staple "$app"`, both
    artifacts are stapled and validated, and the stub-marker check above proves
    the order. `architecture.md`'s Direct download section now declares both
    artifacts and the stapling contract.
  - R1-F5 — **closed.** `depends_on macos: :tahoe`. The real load reports
    `Required: macOS >= 26` and emits no deprecation line, confirming both that
    the requirement is unchanged and that the warning is gone.
  - Consequential document repair — **closed.** `architecture.md:181-191` now
    states that `conflicts_with` accepts `cask:` only and that the formula
    exclusion is the `preflight` refusal.
- New findings:
  - [P2] R3-F1 `scripts/test-macos-distribution.sh:8-9` — the header still states
    the script "never touches the real tap, /Applications, or the user's Homebrew
    prefix", which section 4 now contradicts: it creates
    `$(brew --repository)/Library/Taps/agentdeck-fixture`, and the script hard-fails
    without `brew`. `test-cask-migration.sh:12-14` cross-references that boundary
    ("per the boundary stated at the top of this file"), so a reader following the
    reference is told the opposite of what happens. The added prerequisite also
    reaches `make check-macos-distribution` and therefore `release-verify`, and is
    recorded nowhere. The behaviour is correct and well guarded — a leftover
    fixture is refused rather than reused, and the `trap` removes it on abort;
    what is wrong is the file's own statement of its boundary. -> open
  - [P2] R3-F2 `scripts/test-cask-migration.sh:53-65` — `cask_conflicting_formulae`
    now parses the `preflight` declaration, and it does return both `agentdeck`
    and `agentdeck-rc` when run by hand against the rendered cask. But nothing in
    the file asserts that: section 2 plants only `agentdeck`, so a parser that
    silently lost `agentdeck-rc` would leave the suite green. This is the same
    class of unfalsifiable assertion R1-F3 removed from the sibling script. Round
    2's disposition additionally states the parser "is verified to still return
    both", a verification the file does not perform, so the record currently
    asserts coverage that does not exist. -> open
- Evidence, all against the current content state:
  - `bash scripts/test-macos-distribution.sh` → `macOS distribution packaging: PASS`
  - `bash scripts/test-cask-migration.sh` → `cask migration and mutual exclusion: PASS`
  - `bash scripts/check-widget-sandbox.sh` → `widget sandbox boundary: PASS`
  - `bash scripts/test-release-distribution.sh` → exit 0
  - `ruby scripts/check-release-workflow.rb .github/workflows/release.yml` → exit 0
  - `ruby scripts/check-release-preflight-workflows.rb .github/workflows/release-preflight.yml .github/workflows/release.yml` → exit 0
  - `make check-whitespace` → exit 0
  - Rendered v1.2.3 Cask loaded through Homebrew 6.0.18 in a throwaway tap →
    exit 0, no warning, all artifacts enumerated; tap removed with `brew untap`.
  - Two mutations of `release.yml` (dropping `AGENTDECK_NOTARY_KEYCHAIN`, and
    dropping `--keychain` from `store-credentials`) → `check-release-workflow.rb`
    rejected both.
  - `$(brew --repository)/Library/Taps` carries no `agentdeck*` entry after the
    suite ran.
  - No Go or Swift source changed since Round 1, so the L4 aggregate gate's Go
    stages remain bound to the same content; `make release-verify` was not rerun
    and belongs to the delivery decision once the two open findings close.
- Residual risk, unchanged: the `macos-26` runner label is still unverified from
  here; task 4 `desktop-widget` is still not at Review PASS; the synthetic bundle
  still has no `Frameworks` directory, so the framework-signing loop remains
  unexercised in isolation. Newly noted, not a finding: the `preflight` block's
  runtime behaviour (the `odie` path) is verified by source inspection and
  ordering, not by an executed Homebrew install, which stays the release path's.
- Verdict: REOPEN

## Round 4 — 2026-08-23 — repair of R3-F1, R3-F2

- Repaired state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`, uncommitted
  working tree. Files changed by this round, with their post-repair
  `git hash-object` values:
  - `scripts/test-macos-distribution.sh` `ac36f80e3672c41b5586bb9c83a64880e876dc07`
  - `scripts/test-cask-migration.sh` `e6a530e8d6285ba6be1336c22501e9f83930f8cd`
  - `Makefile` `295898b971ea03299cb45b70eb1babedfeecfe27`
  - `docs/topics/desktop-app/tasks.md` `0caac828948324145517539d0cc94e3504c1f1bd`

  `packaging/homebrew/agentdeck-app.rb.tmpl` is unchanged by this round and still
  hashes to `ecb3e191297532e50491b11fb14cbb40181c4776`, the Round 2 value. It was
  temporarily mutated for the falsifiability check below and restored to that
  exact blob.
- Repairer: Claude Code, acting on the Round 3 findings only.
- Finding-to-change mapping:

  | Finding | Files changed |
  | --- | --- |
  | R3-F1 | `scripts/test-macos-distribution.sh`, `scripts/test-cask-migration.sh`, `Makefile`, `docs/topics/desktop-app/tasks.md` |
  | R3-F2 | `scripts/test-cask-migration.sh` |

  No file outside this table changed, and no unrecorded issue was repaired
  opportunistically.
- Dispositions:
  - R3-F1 — fixed, in all four places the false boundary was stated or should
    have been recorded and was not:
    - `test-macos-distribution.sh`'s header no longer claims the script never
      touches the user's Homebrew prefix. It now states what section 4 actually
      does — creates `$(brew --repository)/Library/Taps/agentdeck-fixture`, loads
      the rendered casks out of it, removes it on the success path and through
      the `trap`, reads and writes no other tap, installs nothing — and states
      why a missing `brew` fails the run instead of skipping the section: a
      silently skipped load check is indistinguishable from a passing one. The
      runtime refusal says the same thing, since the person who hits it in CI
      reads the message and not the header: it names
      `make check-macos-distribution` and `release-verify` as what the
      prerequisite belongs to, rather than reporting only that `brew` is absent.
    - `test-cask-migration.sh`'s header no longer sends the reader to a boundary
      statement that contradicted it. It now says where Homebrew's own verdict
      is obtained, and that this file itself needs no Homebrew and writes only
      inside its temporary directories. The comment in
      `cask_conflicting_formulae` that cross-references it was reworded to match.
    - The `Makefile`'s `check-macos-distribution` comment said "Reaches no Apple
      service and no real tap", which was the same false claim one level up. It
      now records the Homebrew prerequisite at the target that carries it into
      `release-verify`.
    - `tasks.md` task 6's "How the untestable parts are tested" paragraph still
      described the pre-repair split, in which no test asked Homebrew anything.
      It now separates the two questions — whether Homebrew *accepts* the cask
      (the load check) and whether the *declared* artifact set behaves (the local
      installer) — and adds a **Verification prerequisite** paragraph recording
      that Homebrew is a hard requirement of `make check-macos-distribution` and
      therefore of `release-verify`, with the reason it is not a skip.
  - R3-F2 — fixed. `test-cask-migration.sh` section 2 now asserts the parser's
    output directly: `cask_conflicting_formulae` must read exactly
    `agentdeck agentdeck-rc` from the rendered cask, and each of the two is then
    planted on its own so the refusal is proven per channel rather than only for
    whichever is listed first. The pre-existing single-formula case is retained
    unchanged so section 3's migration path still starts from a planted
    `agentdeck`. Round 2's disposition claimed the parser "is verified to still
    return both" on the strength of a by-hand run; that verification is now
    performed by the file on every run, which is what the claim should have
    rested on. Round 2's text is left as written rather than edited after the
    fact — this round is where the gap is recorded and closed.
- Falsifiability of the new coverage, checked rather than assumed. With
  `agentdeck-rc` removed from the template's preflight list,
  `test-cask-migration.sh` fails with
  `cask must exclude both CLI formulae, read: [agentdeck ]` and exits 1 — the
  exact regression R3-F2 says the suite could previously not see. The template
  was restored to blob `ecb3e191…` and re-hashed to confirm it.
- Evidence, all after the final edit:
  - `bash scripts/test-macos-distribution.sh` → `macOS distribution packaging: PASS`
  - `bash scripts/test-cask-migration.sh` → `cask migration and mutual exclusion: PASS`
  - `bash scripts/check-widget-sandbox.sh` → `widget sandbox boundary: PASS`
  - `bash scripts/test-release-distribution.sh` → exit 0
  - `ruby scripts/check-release-workflow.rb .github/workflows/release.yml` → exit 0
  - `ruby scripts/check-release-preflight-workflows.rb .github/workflows/release-preflight.yml .github/workflows/release.yml` → exit 0
  - `bash scripts/check-topic-docs.sh` → exit 0, run because this round edits
    `tasks.md`
  - `make check-whitespace` → exit 0
  - `$(brew --repository)/Library/Taps` carries no `agentdeck*` entry after the
    suite ran.
  - No Go or Swift source changed in this round; three of the four changed files
    are comments and documentation, and the fourth adds test assertions only.
    `make release-verify` was not rerun here and belongs to the delivery decision.
- Residual risk, unchanged by this round: the `macos-26` runner label is still
  unverified from here; task 4 `desktop-widget` is still not at Review PASS; the
  synthetic bundle still has no `Frameworks` directory; and the `preflight`
  block's runtime `odie` path remains verified by source inspection and artifact
  ordering rather than by an executed Homebrew install.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 5 — 2026-08-23 — independent re-review of Round 4

- Reviewed state: HEAD `cb782d6980c6834578645053689ad0a68eaffe0b`, uncommitted
  working tree. Scoped fingerprint of the fifteen files Round 1 named —
  `git hash-object` of each, one `<hash> <path>` line per file in the Round 1
  order, then `shasum -a 256` of that list —
  `653a83edef8654e36c8edcb15e0f76b7e4bf2263f0c4ca0c6c0b429a40df084e`
  (Round 3 was `e5b4d093…e1f85`). Three of the fifteen changed —
  `Makefile`, `scripts/test-macos-distribution.sh`,
  `scripts/test-cask-migration.sh` — plus `tasks.md`, which lies outside the
  fifteen. That is exactly Round 4's declared set; nothing outside it changed and
  no new untracked path appeared.
- Reviewer: Claude Code, independent of the repair round.
- Method: both Round 3 findings re-verified against the current content by the
  mechanism that established them. The new parser assertion's falsifiability was
  reproduced without mutating the repository, by rendering the real cask into a
  scratch copy, removing `agentdeck-rc` from that copy's preflight list, and
  running the parser against both. The aggregate L4 gate was then run against
  this exact content state rather than reused from the pre-repair run.
- Disposition of every Round 3 finding:
  - R3-F1 — **closed**, in all four places the false boundary was stated or
    missing. `test-macos-distribution.sh`'s header now states that section 4
    writes inside the local Homebrew prefix, names the throwaway tap, says it is
    removed on both the success path and the `trap`, and says why a missing
    `brew` fails rather than skips. The runtime refusal carries the same
    information, naming `make check-macos-distribution` and `release-verify` as
    what the prerequisite belongs to, which is what the person who hits it in CI
    actually reads. `test-cask-migration.sh`'s header no longer points at a
    contradicting statement: it now says where Homebrew's verdict is obtained and
    that this file itself needs no Homebrew, and the `cask_conflicting_formulae`
    cross-reference was reworded to match. The `Makefile` comment above
    `check-macos-distribution` — which carried the same false claim one level up
    — now records the prerequisite at the target that carries it into
    `release-verify`. `tasks.md` splits the two questions the stub cannot answer
    and adds a **Verification prerequisite** paragraph. The target's composition
    is unchanged: `release-verify`'s prerequisite list is byte-identical to its
    Round 1 form, and the L4 run below reached every stage.
  - R3-F2 — **closed.** `test-cask-migration.sh` section 2 now asserts
    `cask_conflicting_formulae` reads exactly `agentdeck agentdeck-rc` from the
    rendered cask, then plants each of the two on its own so the refusal is
    proven per channel. Falsifiability confirmed independently and without
    touching the repository: the parser returns `agentdeck agentdeck-rc ` from
    the real render and `agentdeck ` from a scratch copy with `agentdeck-rc`
    removed, so the assertion's comparison fails on exactly the regression it was
    added for. Round 2's overstated claim is superseded by a check the file now
    performs on every run, and Round 4 recorded that rather than editing Round 2
    after the fact, which is the right disposal.
- New findings: none.
- Evidence, all against the current content state:
  - `make release-verify` → **exit 0**. Reached, in order: `check-whitespace`,
    `test-run-go-test.sh` self-test, the full vendored Go suite, the `-race`
    suite, `go vet`, the arm64 and amd64 release builds, the arm64 size gate,
    `test-install.sh`, `test-completion-install.sh`,
    `test-shell-integration-acceptance.sh`, `check-privacy.sh`,
    `check-widget-sandbox.sh` (PASS), `test-release-distribution.sh`,
    `test-release-preflight.sh`, `test-macos-distribution.sh` (PASS) and
    `test-cask-migration.sh` (PASS). Log:
    `scratchpad/release-verify.log`; Go log `scratchpad/l4-gotest.log`.
    This is the task's declared L4 level run against this content state, not
    reused from the pre-repair run, because Rounds 2 and 4 changed
    `package-macos-app.sh`, `release.yml`, the cask template and both test
    scripts.
  - The four component suites and both workflow checkers were additionally run
    standalone before the aggregate, with the same results.
  - `bash scripts/check-topic-docs.sh` → exit 0, run because Round 4 edited
    `tasks.md`.
  - Parser falsifiability, reproduced independently: `agentdeck agentdeck-rc `
    versus `agentdeck ` (see Method).
  - `$(brew --repository)/Library/Taps` carries no `agentdeck*` entry after the
    aggregate gate ran, and the working tree's file set is unchanged.
- Residual risk, carried forward and owned elsewhere:
  - The `macos-26` runner label is still unverified from here; the first CI run
    on a branch carrying these workflows confirms it. Task 5 records this.
  - Task 4 `desktop-widget` is still not at Review PASS. Task 5 declares a
    dependency on it, and this PASS does not substitute for task 4's.
  - The synthetic bundle has no `Frameworks` directory, so
    `package-macos-app.sh`'s framework-signing loop is exercised only by a real
    local build, not in isolation.
  - The `preflight` block's runtime `odie` path is established by Homebrew source
    inspection and artifact ordering, not by an executed `brew install`. That
    execution is the release workflow's `Verify cask install, completions, and
    uninstall` step, which this task builds and is not permitted to run.
- Verdict: PASS
