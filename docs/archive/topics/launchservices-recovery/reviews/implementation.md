---
status: historical
topic: launchservices-recovery
subject: implementation
retired: 2026-10-04
---

## Round 1 — 2026-10-04

## 📋 Independent cold implementation review

Verdict: FAIL
Completion gate: NOT_VERIFIED

Reviewer: cold read-only `/root/cold_implementation_review`, no inherited turns,
product/status/CE/Beads writes, nested CLI or delegation. Main independently
verified source branches, preserved identity receipts and the path failure-first
result. This is implementation review; prior design PASS is separate.
Reviewed candidate: HEAD `24623ec8bf0e172bf8e630ebe203163e76634403`;
33-file `/tmp/agentdeck-ls-candidate/final-manifest.json` (original frozen state);
patch SHA-256 `ed58f056880a8ffe6ce2e0011f4d7a1887263faf35f4355ed5fd56465fc1b490`.
All 33 SHA-256/Git blob identities and evidence log hashes matched directly.

### 🔴 严重问题 — 必须修复

ICR-F1 — P2: launchservices_darwin.go113 discards validated Widget paths when
PlugInKit's later text/count is incomplete. Retain acquired positive observations
under the remaining deadline/output allowance and preserve the parser failure.

ICR-F2 — P2: launchservices.go254 trims the entire Path line, changing a legitimate
trailing-space identity. Remove indentation without stripping the value. Main
failure-first receipt `review-path-failure-first.log` confirms the altered path.

ICR-F3 — P3: launchservices_darwin.go46 applies raw host URL count >=64 before
normalized dedup. Review's synthetic production-JXA execution reproduced 63
identical URLs => one complete identity, 64 => one entry incorrectly incomplete.
Apply the unique normalized limit within the owned bounded child.

### 🟡 改进建议 — 建议处理

Add explicit five-state rendered CLI/no-stdin/non-TTY coverage. Correct the
fixture corpus prose: final report is 2 files, 1850 bytes. Neither fixture refresh
nor bounded OS reads prove representative native App/client acceptance.

### 🟢 优点

Normal-path/early-return integration, conservative warning accounting, escaped
local evidence, owned child wait/reap and desktop DTO privacy are preserved.
Swift consumer action checks prevent a LaunchServices copy action.

### 📝 总结

FAIL until the three findings close under scoped independent re-review. Main
baseline harness used HEAD DesktopWire and ViewModel test source, unchanged App
sources/package/options, and reproduced the same clipboard nil assertions at
755/756; this failure is not introduced by this change. Native clipboard/UI
acceptance is still unproven. No Widget causality, timeline or recovery PASS.
Repair and verification continue; no Task/Topic completion or delivery asserted.

Repair verification discovered ICR-F4 — P2: the production NSError Ref[0]
dereference terminates JXA with segmentation fault for absent paths on this
host. Minimal attributes/error-ref probes reproduced it without source mocking;
ordinary-user execution reproduced the same result, excluding sandbox-only
failure. Replace the unsafe error reference with in-child access/errno: only
ENOENT is missing; permission/other uncertainty stays unreadable. Foundation
attributes request no error reference. Missing canonical and EACCES fixtures
separate absence from unreadability; this is a production bridge repair, not a
larger timeout or skip. The native limit fixture additionally verified real
Foundation NSURL arrays, normalization, aliases and 64 distinct identities.

ICR-F1/F2/F3 repairs: preserve parsed paths and parser reason, preserve trailing
spaces, count normalized identities before the source limit. Add CLI all-state
text/JSON no-stdin, dumb/non-TTY and 20-column tests. Native App permission scope
is pending; no production bundle registration or recovery is authorized here.

## Round 2 — scoped repair review

Verdict: FAIL
Completion gate: NOT_VERIFIED

The same cold read-only reviewer directly matched all 34 file SHA-256/Git blob
identities in `repair-r2-manifest.json`, HEAD unchanged, patch SHA-256
`06e1dd7de73bb1ce00a93a884b3fc356bad37b69b5e141015b55088ada31c7d5`.
ICR-F2/F3/F4 are closed: trailing-space parser identity, normalized unique host
limits, and safe absent/unreadable native metadata are supported by focused
fixtures. Five-state CLI/no-stdin coverage and corpus correction also pass.

Residual ICR-F1 — P2: an empty metadata result after timeout/output failure still
erases validated PlugInKit paths. Preserve bounded raw paths as uncertain
observations; do not infer canonical membership, existence, version or build.
Main's `review-metadata-failure-first.log` reproduces this across eight cases.
Repair now retains up to 64 distinct raw paths when metadata returns no entries,
keeps the earlier enumeration failure authoritative and always remains unknown.
The focused regression covers complete/incomplete enumeration and timeout,
output-limit, failed and empty metadata. Independent re-review is pending.

Full Go suite, scoped race, vet and both Darwin builds passed for the R2 repair
state. Final R3 checks follow the residual fix. Representative native App/client,
clipboard/UI and Widget acceptance remain unproven, separate from code review.

## Round 3 — scoped residual repair review

Verdict: PASS
Completion gate: NOT_VERIFIED

Cold reviewer directly matched HEAD and all 34 scoped SHA-256/Git blobs in
`repair-r3-manifest.json`; patch SHA-256
`104c3cff07437ee76a44856611f2b2f7564f3ac42c7fca64a8860440d24a40c7`.
ICR-F1 closed: failed metadata retains bounded raw identities as unknown;
canonical/version/build remain uninferred. Eight failure-first cases now pass,
metadata failure reason is retained and parser failure takes precedence.
ICR-F2 closed: trailing Path spaces are preserved.
ICR-F3 closed: alias-normalized unique host identities own the limit.
ICR-F4 closed: absent/unreadable metadata avoids unsafe NSError references.
No unresolved code finding; previous FAIL rounds remain part of this record.
Main checked exact identities, logs and the review branches directly.

Code review PASS is separate from registration-acceptance completion. Native
App/client refresh and clipboard/UI remain unverified; the acceptance task stays
open. No collision causality, Widget timeline or system recovery claim is made.

Native acceptance follow-up after R3: authorized hosted XCTest passed 139 tests,
with 1 opt-in schema matrix skipped and 0 failures; native clipboard and new
five-state/no-action cases pass. Production sources/tests are unchanged. Limited
test registration cleanup returned the exact six-ID baseline; the separate
acceptance record preserves the -10814 Widget-only unregister failure and the
successful host removal/final readbacks. Representative App/client refresh
performance remains open; this does not reopen the R3 code findings.

## Round 4 — 2026-10-04 remote PR review

Verdict: FAIL
Completion gate: NOT_VERIFIED

Reviewer: remote Codex; Method: independent GitHub PR review.
Reviewed state: signed commit `175e76c453e986b4cdea5a6253f4ee48a2a75670`,
tree `92fa271f8e6c2973ea15074d1694f8a9ba51c79e`, PR #42, review
`5404692861`, comment `4176485169`. Scope: feature diff against
`release/v0.6.x`; main independently verified the finding against the source.

ICR4-F1 — P2: `internal/doctor/launchservices.go` skips source deduplication
and lexical sorting when the canonical host is absent. Returned Widget entries
still reach local JSON, so OS enumeration order changes the output. In-scope
carrier: `ad-launchservices-recovery-registration-acquisition`. Repair only
normalizes both sources before applicability classification; it changes no
acquisition, budgets, metadata, safe desktop DTO, scanner or performance target.

Evidence: `review-not-applicable-failure-first.log` fails both the host-empty
and populated-source regression cases on the signed head plus test-only change.
Prior ICR-F1/F2/F3/F4 remain closed. The new finding remains open until targeted
independent re-review and exact-state verification; R3 PASS does not cover it.
Raw performance FAIL and the explicit delivery-only decision are preserved.

## Round 5 — 2026-10-04 scoped ordering repair re-review

Verdict: PASS
Completion gate: VERIFIED

Reviewer: `/root/limited_delivery_review`, independent cold read-only role.
Method: targeted two-file source/diff review, direct R3 manifest comparison and
completed verification-log readback; no product/status/evidence writes by reviewer.
Reviewed state: HEAD `175e76c453e986b4cdea5a6253f4ee48a2a75670` plus
`repair-r4-product-manifest.json`; two-file patch SHA-256
`d3311351e4730e9c22ff3164151938c76b8845cba0311cda7513cc8e492d35ec`.
Production SHA-256 `b24b2c561ba3a768e37befc6d3d03de6e166dd61f29a5ba433ef8fe4c8d1d2d6`;
test SHA-256 `07a9c6691a6d0e41148b95970dcda199158f94076dc7b26a7d59b264af608fa2`.
The other 22 R3 product/test inputs match their prior identities directly.

ICR4-F1 CLOSED: both source arrays are normalized before applicability;
duplicate paths collapse and lexical order is stable. Empty host evidence stays
an array, source completeness/reasons are preserved, and not-applicable remains
ok with unchanged applicability reason and no action. Failure-first regression
fails before the repair and passes after it for both populated-source and
host-empty/Widget-present cases, including reordered JSON equality.
ICR-F1/F2/F3/F4 remain closed; no new in-scope finding.

Verification: affected functional packages and ordinary-permission full Go
suite PASS; pure classification race PASS; existing CLI race PASS (578.911s),
desktop race PASS (77.589s), complete doctor race PASS in the normal-permission
rerun (52.210s); scoped vet PASS using the approved temporary Go cache. The
initial overall race result remains FAIL: one unchanged native JXA case hit its
1s deadline under concurrent suites, outside modified registrationCheck. Its
isolated race rerun passed at 0.86s with the same deadline. No DATA RACE report.
Restricted full Go socket-bind/helper failures and the initial native timeout
remain in raw logs; normal full Go and the isolated/full doctor race recoveries
are recorded separately. The redundant fresh CLI/desktop race run is pending;
its completion is not claimed here.

Main directly matched source/log fingerprints, source causal paths and completed
results in `ordering-repair-evidence.json`. Unchanged native acquisition, Swift
privacy/UI and budget inputs support scoped reuse; no new native registration,
performance campaign, recovery, WidgetTimeline or collision-causality PASS.
Original refresh performance FAIL, baseline and user delivery-only acceptance
remain unchanged. Exact repaired-state gates and new signed-head remote CI/review
are required before final delivery.

Main evaluated the exact repaired candidate CEv1 gates: all four Tasks VERIFIED,
three required criteria each, no missing criteria or unresolved impacts. The
normalization delta is directly covered; historical raw failures are retained.
Final record/Git binding uses explicit target-bound preservation.

After the scoped review, the ordinary-permission affected race run completed
PASS for all three packages with no DATA RACE. Main verified the complete log
and unchanged product manifest in `ordering-race-followup.json`. The initial
overall race FAIL and its isolated recovery remain retained separately.
