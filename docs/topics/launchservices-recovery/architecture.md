---
status: active
created: 2026-10-04
updated: 2026-10-04
---

# LaunchServices Recovery — Architecture

Complete candidate for independent batch review, not implementation approval.
Inputs: [requirements.md](requirements.md), [CLI framework/final](ux/cli.md),
and the approved exception in [CLI contract](../../specs/cli-design.md).

## Current interfaces and proposed integration

`internal/doctor/doctor.go` owns Check, Report.add and Service.Check. It returns
early for missing state, unreadable database and unreadable extension inventory.
Append the new check immediately before its final normal return; do not move or
modify earlier checks. `cmd/agentdeck/main.go` owns renderDoctorText and the
existing JSON envelope. `internal/desktop/desktop.go:healthSnapshot` explicitly
copies allowlisted fields; it does not marshal doctor.Check. Preserve that DTO.

Add package-local acquisition and pure classification in `internal/doctor`;
Darwin and other-platform adapters use build tags. A Service injection seam
accepts context/full and returns registration details for deterministic tests.
Default production acquisition uses the native adapter. No dependency, database,
cache, config, flag, daemon, compiled helper or distribution change is required.

## Wire contract and classification

Check name `launchservices`, resource `launchservices_registration`, code and
reason one of `launchservices_consistent`, `launchservices_conflict`,
`launchservices_stale`, `launchservices_unknown`, `launchservices_not_applicable`.
Consistent/not-applicable status is ok; others warning. No Count, Recovery,
ActionKind, DiagnosticCommand or ManualPrerequisite field is set. The local
renderer supplies bounded explanatory guidance; desktop receives no action.
Add the resource and all five reasons to DesktopWire's explicit safe recognition
allowlists. Recognized consistent/not-applicable keep status ok; recognized
conflict/stale/unknown keep warning. No action is enabled. Other unknown tokens
retain the existing fail-closed decoder. This minimal consumer adaptation is
required: leaving the new tokens unknown coerces even ok to warning and lets
MenuBarViewModel synthesize a warning despite zero report problems. No new GUI
layout or copy is introduced. Tests decode complete snapshots and verify notice
and detail rows for all five states, preserving counts and excluding actions.

Optional `registration_details` on doctor.Check contains `applicable` (bool),
`applicability_reason` (optional unsupported_platform/gui_host_absent), `scope`
(`returned_host_urls_and_targeted_widget_matches`), `complete` (bool), `host`
and `widget`. Both sources contain `source`, `complete`, optional `reason` and
`entries` (always array). Names are host_application_urls/widget_pluginkit.
Each entry contains `path`, `canonical` (bool), `state`, optional `version` and
`build`. Entry states: canonical, matching_build, different_build, missing,
unreadable_metadata, invalid_metadata. No raw stdout/stderr or OS errors escape.

Source reasons: timeout, cancelled, control_failed, enumeration_failed,
unknown_format, output_limit, entry_limit, empty_result, unreadable_metadata,
invalid_metadata, canonical_missing. First source failure remains its stable
reason; entries already acquired remain available. A missing installed host is
not-applicable only for genuine not-exist; denied/invalid metadata is unknown.
Absent installed Widget is unknown. Non-Darwin is not-applicable without probing.

Resolve absolute normalized paths and filesystem symlinks for live entries;
deduplicate normalized resolved identities and sort lexically. Missing returned
paths are stale evidence, not canonical aliases. Compare both version and build
against each source's readable canonical bundle with its expected bundle ID.
Nonempty string ID/version/build are required. Two same-build copies are not
conflicts. Both sources must be complete, nonempty, contain the canonical entry,
and pass canonical metadata validation before consistent/conflict/stale can
win. Incomplete wins unknown while preserving positive observations.

## Native sources and limits

Host: `/usr/bin/osascript -l JavaScript` with a fixed AppKit/Foundation script
uses public NSWorkspace URLsForApplicationsWithBundleIdentifier. Same invocation
enumerates Finder and verifies a readable com.apple.finder bundle as a positive
control. Target host ID is com.kitdine.agentdeck. JXA count is explicitly Number
converted (the bridge returns a string). Unsupported API is enumeration_failed,
never empty-complete. The plural API requires macOS 12 or newer.

Widget: fixed `/usr/bin/pluginkit -m -A -D -vv -i
com.kitdine.agentdeck.widget`. Parse only the verified targeted verbose grammar:
identifier(version) heading, known key/value rows including one Path per heading,
and final `(N plug-in[s])` count matching the records. Recognized rows include
UUID, Timestamp, SDK, Parent Bundle, Display Name, Short Name, Parent Name and
Platform. Unknown lines, repeated/missing path, count mismatch or stderr make
the source incomplete. Empty successful output is empty_result. Treat heading
versions as enumeration data only; Info.plist provides classification metadata.
Set LC_ALL=C and LANG=C only in owned child environments. A localized or future
unrecognized shape remains unknown_format; locale never permits weaker parsing.

Both scripts validate bundle metadata with bounded NSFileHandle reads and
NSPropertyListSerialization (XML/binary plist); no unbounded dictionary file
loading. Missing path is distinguished from read denial via filesystem errors.
Canonical paths are /Applications/AgentDeck.app and its nested
Contents/PlugIns/AgentDeckWidget.appex. Widget metadata uses a second fixed JXA
invocation with JSON paths as a data argument; no shell interpolation.

A single parent monotonic context starts before applicability/acquisition and
has 500ms quick / 1.5s full total budget. Reserve 25ms inside it for kill/wait;
subprocesses share its acquisition deadline, never receive fresh independent
budgets. Start host and Widget acquisition concurrently; widget metadata stays
within the same deadline. Join owned goroutines and wait every started child
before return. Fixed executables spawn no owned grandchildren. Tests exercise
overflow cancellation and reap; measured scheduling overshoot is a failure,
never silently excluded from the claimed budget. OS scheduling cannot provide
a hard real-time guarantee, so native timing evidence must report actual wall
time, cold startup and overshoot honestly.

Every possibly blocking filesystem operation (including canonical applicability,
stat, symlink/path resolution, Finder control metadata, bundle metadata and
network/external-volume paths) runs inside an owned terminable JXA child. Go
does no native-path Stat/EvalSymlinks/ReadFile before or around that deadline.
Goroutines own process launch/output/wait only; a join is not a filesystem I/O
timeout. Canonical host not-exist must come from the child and remains bounded.

Per source: fewer than 64 unique returned paths for completeness; reaching 64
is entry_limit. Bound combined stdout/stderr to 256KiB and metadata reads to
256KiB, hence at most 512KiB per source. Metadata allowance includes detection
bytes: reserve one byte from the remaining allowance, then read allowed+1,
never remaining+1. Filling that read means output_limit, including exact-cap
files conservatively; a detection byte is charged to the same allowance. Stop
before another read. Output writer returns an
error/cancels on overflow; Wait is always called. Context expiry/cancellation
produces unknown without fatal doctor errors or altered unrelated semantics.

## Feasibility and evidence boundaries

2026-10-04 ordinary-user read-only probes: Number-converted NSWorkspace returns
four host URLs and one Finder URL; targeted PlugInKit returns one canonical
Widget. Bounded NSFileHandle/NSPropertyListSerialization reads canonical XML
metadata (1624 bytes, ID com.kitdine.agentdeck, version0.6.0/build21).
Sandbox PlugInKit reports Connection invalid; it is an unknown-case observation.
These prove bridge/source feasibility, not production timing, binary-plist
regression coverage, a real conflict, root cause, actual Widget operation or
system recovery. No registration mutation occurred.

## Validation and field reconciliation

All requested CLI fields above are provisioned; raw output/action/timing fields
are explicitly refused as stated in UX. Optional details never reach desktop.
L3 validation includes failure-first tests, source/metadata/parser/classifier
fixtures, deadline/startup/output/wait/reap/cancellation, deterministic ordering,
aliases/same-build/different-build/stale/unknown, full Go core regressions,
relevant race/vet and both Darwin CLI architectures. CLI/desktop contract tests
cover original early returns/envelopes/counts, sanitized text, safe DTO and
unknown handling. Read-only ordinary-user native acceptance uses isolated state
and measures quick/full plus existing refresh targets. Unperformed timeline,
collision and recovery remain explicit limitations.
