---
status: historical
created: 2026-10-03
updated: 2026-10-03
retired: 2026-10-04
---

# LaunchServices Recovery — Requirements

## Origin and supported premises

Origin: P2 `ad-bug-lsregister-collision-no-recovery-path`, split from the
already-delivered fixture-bundle-ID repair. Its historical incident reported
conflicting registered host builds and Widget archive rejection. That history
supports a discoverability problem; it is not fresh reproduction or acceptance.
The real operator selected Lane B option 2, v0.6.5, automatic full flow and the
precise primary-version exception delivered in PR #40. No later-version choice
or automatic recovery is pending.

Current doctor has read-only quick/full checks, a success report at exit 0 even
with warnings, explicit text/JSON renderers and safe desktop-health DTOs. It has
no OS-registration diagnosis. Its extension duplicate-ID check describes client
inventory, not LaunchServices. The installed host ID is
`com.kitdine.agentdeck`; its Widget ID is `com.kitdine.agentdeck.widget`, with the
supported Cask installation at `/Applications/AgentDeck.app` and its nested
`Contents/PlugIns/AgentDeckWidget.appex`.

Native feasibility receipts are preserved outside Git:

- The prior ordinary-user public application API returned four host URLs with a
  successful Finder control. The same managed-sandbox control failed. Four URLs
  alone prove neither conflicting versions nor Widget causality or recovery.
- That application-only API returned -10814 for the appex. It cannot establish
  Widget absence. A new target-limited ordinary-user PlugInKit match returned
  one canonical installed appex in 0.159 s; it establishes this data source's
  feasibility only.
- The prior real `lsregister -dump` timed out after 5 s. No further full dump,
  full OS-database scan or private daemon/log dependency is selected.

Receipts: `/tmp/agentdeck-doctor-v065/ls-public-ordinary-user.json`,
`launchservices-ordinary-user-dump.json`, and
`/tmp/agentdeck-doctor-v065-resume/widget-pluginkit-probe.json`.

## Goal

A user investigating a stale or black AgentDeck Widget can run the existing
`agentdeck doctor` or `agentdeck doctor --full`, discover bounded registration
evidence, distinguish observed risks from uncertainty, and obtain safe manual
next steps without reading private system logs. The feature improves diagnosis
and recovery discoverability; it does not perform or certify system recovery.

## Scope and observable behavior

1. On macOS with the supported installed AgentDeck app, inspect the host and
   appex through distinct sources appropriate to their identities. The host
   application API is never used as an appex absence test. Do not expand the
   extension-inventory resource or reuse `extension_duplicate_id`.
2. Compare returned registrations with installed bundle identity/build metadata.
   Distinguish the canonical installed copy, live matching copies, live copies
   with different builds, returned paths that are missing, malformed/unreadable
   metadata, and unknown/incomplete enumeration. Deduplicate normalized paths;
   an alias or two same-build copies alone is not a version conflict.
3. Report differing observed live versions as a possible registration conflict,
   with the supporting entries. Never claim that the OS selected the wrong
   host, that WidgetKit failed because of it, or that a Widget recovered.
   Observed stale paths are not an exhaustive stale-registration inventory.
4. A valid canonical-only or matching-copy result is consistent only within the
   declared returned-source scope. A failed positive control, timeout, denied
   read, unknown format, output limit or incomplete required source cannot yield
   an `ok` registration conclusion. Empty appex results are inconclusive rather
   than proof of missing registration. CLI-only/non-macOS installations do not
   acquire a spurious unhealthy GUI diagnosis; architecture defines explicit
   non-applicability without claiming OS health.
5. Quick and full use the same safety/classification semantics. Quick is bounded
   for normal health reads; full may use a larger bounded acquisition budget and
   expose additional evidence, not a broader authority or a system mutation.
   The OS-diagnostic acquisition/classification budget is at most 500 ms in quick
   mode and 1.5 s in full mode, including owned child startup/output/wait. Every
   child is reaped; cancellation and output-limit failure leave no diagnostic
   process running. Bound each source to 64 unique returned entries and 512 KiB
   output; hitting a limit is incomplete, never a clean result.
6. Add only the declared diagnostic check/codes, optional diagnostic details and
   their consequent warning/count/status effects. Native uncertainty is a
   diagnostic warning, not a fatal command error or failure of unrelated health
   sections. Do not alter existing checks, errors, database short-circuit/partial
   semantics or `checks_skipped`; record new-source completeness explicitly.
7. Desktop receives only the existing safe health projection: status, counts,
   stable check/code/resource/reason and allowlisted action metadata. Raw bundle
   paths, probe output and build-entry details stay out of desktop wire data.
   Unknown values continue to fail closed without a copyable recovery action.

## Surfaces and manual guidance

The sole new design surface is [ux/cli.md](ux/cli.md): ordinary line-oriented
doctor text/JSON, including piped/non-TTY output. It also ratifies use of the
existing desktop safe health/count presentation; there is no new GUI layout,
button, setting, full-screen mode, native footer or icon change.

Text identifies the evidence scope and uncertainty before guidance. It gives a
bounded English explanation and inspected paths/builds with control characters
escaped. JSON retains stable keys/enums, preserves the success/error envelope
and omits executable `recovery_command` for this resource. No OS-changing action
is generated as a copyable health action. Neither format prompts or reads stdin.

Manual guidance must require the user to identify which copy is obsolete, check
the installed canonical host/appex and quit obsolete app copies normally before
any targeted unregister/removal. Explain how to act only on that verified obsolete
registration, protecting the canonical app and Widget; give no blanket reset,
wildcard cleanup, database rebuild, daemon restart or permission escalation.
Malformed/control-character paths never become executable guidance. A later
read-only doctor run confirms only the observed registration change; actual
Widget operation is a separate user check. Uncertain evidence gives inspection
steps rather than pretending an unregister target is safe.

## Compatibility and non-goals

The approved primary-version exception permits these v0.6.5 doctor/health
diagnostic outputs and warning counts to differ from v0.6.0. Fixed-output/count
scripts may need adaptation; release documentation must disclose the exception.
Commands/subcommands/flags, exit codes, databases, persisted formats and existing
check semantics do not change. General MINOR/PATCH rules remain in force for all
other subjects. Backport only the already-reviewed primary exception into this
patch-line feature, never newer main wholesale.

No automatic unregister/cleanup, daemon restart, real database rebuild, user-data
write, new OS privilege, network call, global provider/runtime/credential change,
P3/native footer/icon/whole-popover work, overall assembly, publication/deployment,
history rewrite or next batch. No new long-lived helper or daemon, diagnostic
cache file, saved OS-registration inventory, TUI dependency or command/flag.
Temporary test/native receipts remain outside Git. Existing refresh performance
targets remain; report measurements honestly, without a new implicit waiver.

## Acceptance and verification boundary

- Failure-first isolated tests prove old doctor lacks the selected diagnosis;
  focused behavioral regressions cover host/appex identity, alias deduplication,
  same-build copies, differing builds, missing paths, malformed metadata, empty
  results, control failure, truncation, timeout/cancellation and deterministic
  ordering. No fixture uses/registers a production bundle ID in the OS registry.
- CLI text and real success-envelope tests prove stable new keys/codes, preserved
  exit/envelope/original checks, warning accounting and safe manual guidance.
  Width/height/non-TTY, Unicode/control-character and structured-output cases
  exercise [ux/cli.md](ux/cli.md); stdin is never consumed.
- Desktop allowlist tests prove raw registration detail/path exclusion and no
  unsafe recovery action; existing safe health counts and decoding remain valid.
- Ordinary-user native acceptance runs the built implementation against an
  isolated AgentDeck state directory while reading actual OS registrations.
  Verify the source/control/installed metadata and meaningful classification,
  compare with the target-limited native receipts, and measure bounded completion.
  A managed-sandbox control failure is an unknown-case test, never health PASS.
  Live-only consistent data is not a real collision or recovery reproducer.
- Preserve real state/OS registration contents and do not execute any manual
  recovery step. Explicitly report unperformed system recovery and Widget
  timeline acceptance; do not label them passing from fixtures or enumeration.
- Requirements, terminal framework, architecture, final terminal surface and
  decomposition pass independent cold review before implementation. Each owning
  gate uses standard MCP CEv1, exact content and required Beads coordination.
  Final delivery requires a signed logical commit, ordinary feature PR, no
  finding on its final exact-head GitHub review, all required CI, verified merge
  commit and legal whole-topic closure/archive. Origin closes only when this
  diagnosis/guidance remedy is actually delivered, not when contracts pass.
