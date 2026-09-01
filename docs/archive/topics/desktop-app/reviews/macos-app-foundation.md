---
status: historical
topic: desktop-app
subject: macos-app-foundation
retired: 2026-09-01
---

# Review log — desktop-app / macos-app-foundation

> **Path note (2026-08-16).** The approved contract this review judged,
> `docs/specs/macos-app-foundation-design.md`, is now the Foundation runtime
> section of `docs/topics/desktop-app/architecture.md`. The paths cited below
> identify the state that was reviewed and are left unchanged.

Round: 3
Reviewed candidate: `159ce24e112be734e8cd97adc50218818ba1b76da7876144d8931542b986d1a1`
Reviewer: Codex
Verdict: **PASS**

## Scope

This review compares the current first-round macOS foundation candidate with
the approved `docs/specs/macos-app-foundation-design.md` contract.

The candidate fingerprint covers the implementation, tests, build
configuration and scripts, and approved task design. It excludes this review
record and the plan/index status artifacts changed by the review itself.

Production code, tests, and configuration were read-only during Review.

## Blocking findings

### R1-F1: the real application does not own or start the refresh runtime

The approved design requires the application root to own one refresh
coordinator, inject its state, and initiate one non-blocking refresh after
startup.

The application entry in `apps/macos/AgentDeckApp/AgentDeckApp.swift:5` defines
only a placeholder `MenuBarExtra`. Its scene body renders static text through
line 13 and does not create a coordinator, expose refresh state, start a
refresh, or publish a successful snapshot to the App Group cache.

Impact: the helper runner and store exist as library code, but the shipped
application does not exercise the foundation data path. A successful build and
unit tests therefore do not prove the application can produce desktop state.

Required repair:

- add one application-lifetime coordinator at the real application root;
- start the initial refresh without blocking the main actor;
- publish only the latest non-cancelled result to in-memory state and cache;
- retain the last valid state on failure;
- add application-integration tests for launch refresh and failure retention.

### R1-F2: the App Group cache persists raw diagnostic text

The approved design defines the App Group file as a minimal,
presentation-safe projection and explicitly prohibits raw warning, error,
health-problem, and health-check text.

`apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:9` stores the envelope
warning array, and its initializer copies those warnings at line 21. The health
projection at lines 121-128 copies problem, warning, error, and check arrays
from the wire snapshot rather than reducing them to aggregate status, counts,
and allowlisted issue codes.

Impact: diagnostics that may contain paths or configuration details are written
to storage shared with every App Group member, exceeding the data required by
the future Widget and contradicting the repository privacy boundary.

Required repair:

- replace raw diagnostic strings and checks with aggregate counts and
  allowlisted presentation codes;
- keep detailed diagnostics transient and application-private;
- add negative privacy assertions proving prohibited strings and fields cannot
  appear in encoded cache data.

### R1-F3: private permissions are applied after cache publication

The approved persistence contract requires a private directory and a temporary
file with secure permissions before bytes are written and before atomic
publication.

`apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:171` creates the cache
directory without an explicit private mode. The store then performs an atomic
destination write and only afterwards applies `0600` to the published file at
approximately lines 178-180.

Impact: final permissions may be correct, but confidentiality depends on the
process umask and on a post-publication chmod. The published file can therefore
exist before the design's privacy invariant is established.

Required repair:

- create or verify the cache directory as `0700`;
- create the same-directory temporary file as `0600` before writing;
- flush and atomically replace the destination only after the secure write
  completes;
- verify the final regular, non-symlink file remains `0600`;
- prove a failed write preserves the previous cache.

### R1-F4: `generated_at` is accepted without timestamp validation

The approved wire boundary requires a malformed `generated_at` value to be
rejected.

`apps/macos/AgentDeckShared/DesktopWire.swift:44` begins envelope decoding and
line 48 decodes `generated_at` as an arbitrary `String`. The decoder validates
the envelope and data versions but does not parse or validate that string as an
RFC 3339 timestamp before returning the envelope.

Impact: malformed time data is accepted as valid state, so later freshness and
staleness decisions cannot rely on the shared runtime contract.

Required repair:

- validate the timestamp at the wire boundary while preserving its canonical
  serialized representation;
- reject malformed timestamps without replacing last-good state or cache;
- add valid, offset, fractional-second, and malformed timestamp coverage.

## Confirmed non-blocking behavior

Source inspection confirms useful foundation pieces that should be preserved:

- helper resolution is bundle-based and does not use a shell or `PATH`;
- helper execution has bounded stdout/stderr, timeout, cancellation, and
  termination handling;
- the wire decoder checks envelope and data versions and accepts valid partial
  output;
- logging emits fixed classification strings rather than raw helper output,
  paths, session content, or provider configuration;
- development records report an unsigned Xcode build and 10 isolated XCTest
  cases passing.

These facts do not close the four blocking findings.

## Verification disposition

- Independent source review against the approved design: **FAIL**.
- Candidate fingerprint:
  `669e755fa6dc859a756c0aec7abe5a9af87d475b726daf6c3dfe4fde1c1fa662`.
- Broad build and test verification was stopped after decisive source
  reproducers, as required by the project review policy.
- Previously recorded Development build/test evidence was not rerun and does
  not prove the newly approved application-integration, cache-minimization,
  pre-publication permission, or timestamp-validation requirements.

## Verdict

**FAIL.** The Task remains Development-complete but Review-incomplete. Repair is
limited to R1-F1 through R1-F4 and their proportionate regression coverage.
Repair does not authorize commit, push, signing, notarization, release, work on
later desktop tasks, or a PASS verdict without independent Re-review.

## 📋 macOS App Foundation Re-review — Round 2

📊 Overall score: 8/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:4] R2-F1: the App Group
cache reader accepts unsupported cache schema versions.

- Disposition: new.
- Behavior risk: a future Widget or application reader can treat an
  incompatible cache payload as valid instead of treating the cache as
  unavailable, so an additive or breaking cache-schema change can cross the
  version boundary without the required fail-closed behavior.
- Evidence: `AppGroupDesktopSnapshotV1` declares `schemaVersion` but relies on
  synthesized `Codable` decoding, and `AppGroupSnapshotStore.read()` directly
  returns `JSONDecoder().decode(...)` without comparing the decoded value with
  `AppGroupDesktopSnapshotV1.schemaVersion`. The 17-test Xcode run has no
  unsupported-cache-version case.

💡 Bounded remediation: reject any decoded App Group payload whose
`schema_version` is not `1` with a typed unavailable/unsupported-schema error,
and add regression coverage for unsupported and malformed cache payloads.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- R1-F1 is closed: the real application delegate owns one
  `DesktopRefreshCoordinator`, starts the initial refresh after launch, and
  injects that coordinator into the scene. Coordinator tests cover initial
  publication and last-good retention after helper and wire failures.
- R1-F2 is closed: the App Group projection stores aggregate fields and sorted
  allowlisted issue codes; raw warning, health-check, recovery-command, and
  unknown diagnostic strings are excluded and covered by negative assertions.
- R1-F3 is closed: the directory is established as `0700`, a same-directory
  temporary file is opened with `O_EXCL | O_NOFOLLOW` and mode `0600`, the file
  is synchronized before `rename`, the final file is verified, and replacement
  failure preserves the prior cache.
- R1-F4 is closed: envelope, data, and next-refresh timestamps are validated as
  RFC 3339 values while their serialized strings are preserved; offset,
  fractional-second, malformed, and last-good-retention cases pass.
- With `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`, the unsigned
  application build succeeded and `AgentDeckSharedTests` executed 17 tests with
  zero failures.

### 📝 Summary

The reviewed candidate is `HEAD b63721d` plus scoped fingerprint
`4eecd9fe44add15266f1eee97f3ed011f126b6652e2b62b161a045c918011bc4`, covering
the implementation, tests, Xcode/build configuration, build scripts, and the
approved design while excluding this review and plan/index status artifacts.
R1-F1 through R1-F4 are closed in that content state. R2-F1 is a newly blocking
contract failure, so the Task remains Review-incomplete even though the scoped
Xcode build and all 17 XCTest cases pass. No additional broad verification was
started after the decisive unsupported-schema source reproducer. Repair remains
limited to cache-version rejection and its proportionate regression coverage.

## 📋 macOS App Foundation Re-review — Round 3

📊 Overall score: 10/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- R2-F1 is closed: `AppGroupSnapshotStore.read()` compares the decoded cache
  schema with `AppGroupDesktopSnapshotV1.schemaVersion` and returns the typed
  `unsupportedSchemaVersion(Int)` error before exposing an incompatible
  snapshot.
- The new regression cases prove unsupported cache schema versions and
  malformed cache JSON both fail closed.
- R1-F2 and R1-F3 remain closed in the changed App Group store: the projection
  retains only aggregate fields and allowlisted issue codes, and publication
  still uses a `0700` directory, a synchronized `0600` temporary file, atomic
  replacement, final-file verification, and prior-cache preservation.
- R1-F1 and R1-F4 were outside the Round 2 Repair delta and their coordinator,
  launch, timestamp, and last-good-state regression cases remain green.
- The unsigned application build succeeded and `AgentDeckSharedTests` executed
  19 tests with zero failures under the full Xcode toolchain.

### 📝 Summary

The reviewed candidate is `HEAD a218a21` plus scoped fingerprint
`159ce24e112be734e8cd97adc50218818ba1b76da7876144d8931542b986d1a1`, covering
the implementation, tests, Xcode/build configuration, build scripts, and the
approved design while excluding this review and plan/index status artifacts.
The concurrent HEAD advance from `b63721d` to `a218a21` changed only repository
instruction and review-format documentation outside that scoped fingerprint;
it did not invalidate the source or test evidence.
R2-F1 is closed and R1-F1 through R1-F4 remain closed. No blocking, material,
or newly regressed finding remains in the reviewed Task boundary, so
`macos-app-foundation` passes Re-review Round 3.
