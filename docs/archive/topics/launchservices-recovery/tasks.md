---
status: historical
created: 2026-10-03
updated: 2026-10-04
retired: 2026-10-04
---

# LaunchServices Recovery — Tasks

Origin: `ad-bug-lsregister-collision-no-recovery-path`, P2, Lane B option 2.
The operator selected v0.6.5 and approved the single bounded primary-version
exception delivered by PR #40. This feature supplies read-only diagnosis and
manual safety guidance; it never modifies system registrations or user data.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

The terminal surface covers ordinary doctor text/JSON and the existing safe
desktop-health projection. There is no new GUI layout, recovery button, native
footer/icon or full-screen TUI. The existing desktop count and unknown-value
fail-closed presentation remain authoritative; raw registration paths stay out
of desktop wire data. `ux/cli.md` first defines the framework, then reconciles
with architecture before its final approval. Required unwritten documents have
unchecked Draft cells; no empty placeholders are created.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| registration-acquisition | [x] | [x] |
| doctor-registration-surfaces | [x] | [x] |
| safe-health-projection | [x] | [x] |
| registration-acceptance | [x] | [x] |

The coherent design and implementation batches passed independent review and
applicable exact-state gates. Product delivery completed through PR #42 at
`f84e58a9ed8085b37d8bea605070ead5d837fc5f`, with reviewed signed source
`4f1283ad19cdbfdc49344f51aac53dd4f81fb190` and identical merge tree
`7aa9b7485b1b3e9c38a73fe77e302c0651e99af6`. Final source and postmerge CI
passed. The source-normalization repair passed targeted independent re-review;
the remote P2 was resolved with no new findings. Four immutable Task gates are
VERIFIED 3/3; the product merge Topic gate is VERIFIED 2/2. Those results cover
their named states, rather than automatically certifying later document changes.
Registration acceptance uses the explicit delivery-only risk disposition;
original whole-refresh performance remains FAIL.

### registration-acquisition

Files: internal/doctor/launchservices*.go and focused tests. Implement distinct
host/appex acquisition, control, bounded metadata, pure classification, shared
deadline and child ownership as specified in architecture. L3: failure-first
diagnostic test; parser/source/control/empty/truncation/metadata/aliases/builds/
missing paths; startup/output/cancellation/wait/reap; relevant race/vet and both
macOS CLI builds. Fixtures never register production bundle IDs.

### doctor-registration-surfaces

Files: internal/doctor/doctor.go, cmd/agentdeck/main.go, adjacent tests and
prototype/src/launchservices.js. Integrate one check on the normal return and
local text/JSON details/guidance. L2: original checks/early returns/partial/
checks_skipped/counts/exit/envelope regression, all diagnostic states, control/
Unicode/non-TTY/no-stdin/failing writer. Run scripts/run-go-test.sh ./... once
after the final relevant change. Release disclosure uses the approved bounded
exception; no other public semantics change.

### safe-health-projection

Files: internal/desktop/desktop_test.go,
apps/macos/AgentDeckShared/DesktopWire.swift,
apps/macos/AgentDeckTests/DesktopWireTests.swift and
apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift. Add minimal recognition
allowlists for the new resource and five reasons; do not alter GUI layout.
L3 privacy: explicit DTO excludes paths/builds/raw details;
new resource has no copyable action; existing count and unknown-value handling
remain valid. Assert a future unknown resource keeps warning/count and does
not discard the entire health snapshot or expose a copy action. No native
GUI/footer implementation.
Recognized consistent/not-applicable retain ok, zero warning notices and no
actions. Recognized conflict/stale/unknown retain warning/count and no actions.
Unknown future resource/reason tokens preserve the prior fail-closed behavior.

### registration-acceptance

Files: topic task/review/acceptance records at their authorized boundaries.
L3: built quick/full ordinary-user reads against isolated AgentDeck state,
source/control/metadata comparison and cold/warm bounded timing; existing
snapshot refresh performance targets remain in force. Report fixture versus
native evidence and unperformed collision/timeline/system recovery. Independent
implementation review and scoped CE gates precede completion. The later explicit delivery authorization covers signed commits, ordinary
feature push, Draft PR and normal merge after exact-head CI and independent
review. No release or deployment is authorized.

## Working context and authority

- Workspace: `launchservices-recovery`, branch `feature/launchservices-recovery`.
- Base: verified patch-line `24623ec8bf0e172bf8e630ebe203163e76634403`.
- Delivery target: ordinary PR to `release/v0.6.x`; no direct shared-ref push,
  history rewrite, branch/worktree deletion, publication or deployment.
- Historical writer before the user-requested stop: local Codex session
  `01a101d7-cbe1-7692-b209-c1b6190fb27d`, gpt-6.1-sol/xhigh, existing provider.
  Independent reviewers are cold local read-only roles; they cannot write this
  matrix, review records, completion evidence, Beads or product content.
- No unregister/cleanup, daemon restart, real DB rebuild, user-data modification,
  new OS privilege, global runtime/provider change, unrelated P3 work or next
  batch. Existing commands/flags/exits/databases/persisted formats/check semantics
  remain fixed. Only the approved diagnostic addition/count effects are selected.

## Closure and retained boundaries

The approved read-only diagnosis and manual safety guidance were delivered by
PR #42 into `release/v0.6.x`. Eight scoped Beads work-product tasks and the origin
`ad-bug-lsregister-collision-no-recovery-path` are closed after direct delivery
readback. This closes diagnostic/recovery discoverability, without claiming
actual system recovery. The final remote review and PR/push CI are bound to
signed source `4f1283ad`; postmerge verify and desktop CI succeeded at `f84e58a9`.

This authorized document closure reconciles the stable [Doctor contract](../../../specs/cli-design.md#launchservices-registration-diagnosis),
the integrated [project status](../../../status.md) and whole-topic retirement.
The topic's requirements, UX, architecture, matrices and complete review history
travel together; historical content identities remain facts about their original
paths and states. Current review and evidence bindings cover the closure batch.

[Registration acceptance](reviews/registration-acceptance.md) retains failed
original targets, native baseline/candidate measurements, first-cache variation,
201–222 ms added snapshot cost and the user's 2026-10-04 06:19UTC delivery-only
risk acceptance. The user deferred performance repair on 2026-10-04 to
[issue #43](https://github.com/kitdine/agent-deck/issues/43), which does not block
this closeout. Performance remains FAIL, with no future waiver or scanner repair
included here.

Final R3 Go, scoped race/vet, both Darwin builds and arm64 size checks passed.
Hosted native evidence had 139 pass, one opt-in schema skip and no failures;
unchanged source/test identities were verified for reuse. The two-file ordering
repair has separate failure-first, full Go, affected race/vet and independent
review evidence. Earlier harness failures and all raw performance measurements
remain in the review histories and preserved evidence. This document-only
closure reuses unchanged product evidence and does not rerun a native campaign.

Diagnosis preserves host/appex separation, total 500 ms/1.5 s probe budgets,
64-entry/512 KiB source limits, conservative unknowns and path-free desktop health.
No automatic recovery, production registration change, WidgetTimeline or
collision-causality acceptance is claimed. Authorized temporary test
registrations/files were cleaned; production entries were unchanged.

Footer P3 `ad-bug-footer-routes-narrow-truncation` stays open. Native 280 pt
confirmation with real route names precedes its Lane A/B disposition and valid
workspace binding. Its closed schema-signal document relation is `relates-to`,
not a blocking dependency. No P3 design, implementation or acceptance occurred.
Main propagation, version assembly and release publication remain separately
scoped. After this closure, the parent arranges later local CLI work; this task
does not start another product writer or change runtime/model routing.
