---
status: active
created: 2026-10-03
updated: 2026-10-04
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

The coherent design batch passed independent scoped re-review and its document
gates. The approved local implementation and test work is produced. Cold implementation R3 PASS and exact-state gates verify the first three tasks.
Registration acceptance has a reviewed explicit delivery-only risk disposition.
Raw overall-refresh performance remains failed; task gates are VERIFIED through
explicit delivery acceptance and preserved R3 evidence. Authorized Git/PR
delivery remains pending.

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
implementation review and scoped CE gates precede completion. Delivery remains
with the parent; this session performs no commit/push/PR/merge/release.

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

## Current handoff

The unchanged R3 product/test candidate has independent code review PASS and
reviewed user acceptance of the disclosed overall-refresh performance limit.
[Registration acceptance](reviews/registration-acceptance.md) preserves the
original failed targets, native baseline/candidate measurements, first-cache
variation, added snapshot cost and the user's delivery-only decision. The
performance result remains FAIL; scanner repair and a future waiver are outside
this scope. Task gates are VERIFIED, with acceptance explicitly based on the user-approved
delivery exception. Final metadata synchronization reuses this exact evidence;
performance is not marked passing.

Final Go suite, scoped race/vet, both Darwin builds and arm64 size passed for
R3. Authorized hosted native tests had 139 pass, one opt-in schema skip and no
failures; source/test identities and existing logs were verified for reuse.
Earlier narrower harness failures and all raw performance measurements remain
in the acceptance/implementation records and preserved local evidence. No new
product/native test matrix is required for this record-only disposition.

Read-only doctor diagnostics and manual advice preserve host/appex separation,
500ms/1.5s total probe budgets,64-entry/512KiB limits, conservative unknowns and
path-free desktop health. No automatic recovery, production registration change,
WidgetTimeline or collision-causality acceptance is claimed. Authorized temporary
test registrations/files were cleaned; production entries were unchanged.

The real user authorized signed logical delivery of this coherent candidate,
ordinary feature push and a Draft PR into `release/v0.6.x`, followed by exact-head
CI and independent review before normal merge. Delivery and origin closure
remain pending; unrelated P3 and aggregate release work stay open. After this
task finishes, provide a clean handoff for the parent to arrange subsequent
local CLI work; do not start another product task or CLI here.
