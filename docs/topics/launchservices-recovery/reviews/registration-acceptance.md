---
status: active
topic: launchservices-recovery
subject: registration-acceptance
---

## Round 1 — acceptance boundary remains open

Verdict: FAIL
Completion gate: NOT_VERIFIED

Code review R3 PASS and its exact receipt are in [implementation](implementation.md).
ACR-F1 -> open, carrier: `ad-launchservices-recovery-registration-acceptance`.
Representative native App/client refresh and clipboard/UI have not been accepted.
Two-file fixture performance and read-only native doctor are narrower evidence.
The unchanged HEAD source harness reproduces the same existing pasteboard nil
assertions; this excludes an introduced regression but does not prove native UI.
Hosted App test IDs can affect user-session LaunchServices even with temporary
HOME; permission for that action is pending. No production IDs are registered.

## Round 2 — authorized native hosted acceptance follow-up

Verdict: FAIL
Completion gate: NOT_VERIFIED

The real user explicitly approved the shown native test-ID registration and
limited cleanup at 2026-10-04 02:57 UTC. No new OS privilege/security-setting
request or production registration operation occurred. Actual Xcode/xcresult
result: 140 total, 139 passed, 1 opt-in schema matrix skipped, 0 failed. Native
clipboard and new LaunchServices five-state/no-action tests pass. The 45 copied
Swift source/test identities match the worktree; only candidate product IDs/app
group configuration were different. Report and xcresult are retained in
`/tmp/agentdeck-ls-candidate/hosted-native`; `hosted-evidence.json` binds log hashes.

Limited cleanup: baseline had no exact test IDs. Only the App and its embedded
Widget were newly registered. Widget-only unregister failed -10814; host
unregister succeeded, after which six exact ID queries, Widget enumeration and
LaunchServices dump returned baseline. Final checks after deletion also have no
test records. Seven production path/version records match baseline; no owned
test/helper process remained. Only the two exact self-created test roots were
deleted after preserving report/xcresult/configuration evidence.

ACR-F1 remains -> open, carrier: `ad-launchservices-recovery-registration-acceptance`.
Native clipboard and fixture rendering uncertainty is resolved within the hosted
test scope, and the authorization prerequisite is fulfilled. Representative
native App/client refresh wall/CPU/RSS remains unmeasured. A representative
isolated corpus and measurement of actual App client through worker, snapshot
publication and decode are the remaining prerequisites. Go fixture refresh does
not include that client. No causal collision, Widget timeline or recovery PASS.

## Round 3 — 2026-10-04 — measured performance and delivery decision

Verdict: FAIL
Completion gate: NOT_VERIFIED

Reviewer: Codex main, direct source/log/identity inspection; this diagnostic
round preserves independent R3 code review rather than issuing a new code PASS.
Reviewed product: HEAD `24623ec8bf0e172bf8e630ebe203163e76634403`,
R3 patch SHA-256
`104c3cff07437ee76a44856611f2b2f7564f3ac42c7fca64a8860440d24a40c7`.
The 24 product/test identities match the reviewed R3 manifest. Performance
measurement used the actual native App client, owned helper/worker, snapshot
decode and isolated publication, with a wholly synthetic 2,200-file,
2,343,100,564-byte corpus. Eight logical row digests agree across cached runs.
The HEAD comparison uses the same helper build metadata, unchanged Swift inputs
and harness. These are bounded diagnostic observations, not a completed
20-sample acceptance matrix or a statistically established regression verdict.

### Performance result — failed targets remain failed

The original targets in [snapshot-performance requirements](../../snapshot-performance/requirements.md)
remain unchanged: complete cold import/recomputation wall time <=10s; unchanged
refresh wall time <=1s, combined client/helper/worker CPU <=500ms and executing
helper/worker RSS <=100MiB. CPU/RSS budgets apply to unchanged refresh; cold
~810MB RSS is disclosed without inventing a cold RSS threshold.

| Unchanged diagnostic sample | HEAD wall | Candidate wall | HEAD CPU lower bound | Candidate CPU lower bound |
| --- | --- | --- | --- | --- |
| First | 2,123.756ms | 4,242.074ms | 779.156ms | 1,364.922ms |
| Second | 1,604.229ms | 1,828.296ms | 571.677ms | 947.026ms |

Both variants exceed unchanged wall and CPU targets. Native cold refresh timed
out at 120s on both variants; the HEAD worker completed at 196,873ms, while
candidate workers completed at 188,139–198,199ms. This occurs before snapshot
and the doctor probe. Existing ingestion and derived aggregation account for
the cold duration. Scanner repair is explicitly outside this delivery.

The candidate snapshot stage adds an observed 201–222ms. Second-sample waited
child CPU increases by about 333ms. These costs remain disclosed; there is no
claim of absolute absence of regression. About 86% of the first wall difference
is in the common scan command, including 1,588ms of additional unsegmented
worker time. Existing logs cannot distinguish DB-open, planning and close.
An unchanged ~1s worker-idle/lock-release wait also affects both variants.
Two sequential cached samples, a live business clock and finite resource
sampling do not establish formal paired timing, P95 or complete-lifecycle RSS
PASS. No additional scanner investigation or refactor is included.

ACR-F1 -> open, carrier: `ad-launchservices-recovery-registration-acceptance`.
Representative measurements are now available but do not satisfy the original
performance targets. Earlier FAIL rounds and raw failures remain preserved.

### Explicit limited delivery decision

At 2026-10-04 06:18 UTC the parent explained that doctor/code review passed,
overall refresh performance acceptance failed on the original baseline, and
proposed accepting that known limit before committing, opening a PR and merging
after CI/review. The real user replied **“同意”** at 06:19 UTC.
Source: parent thread `01a0f0e6-40aa-726b-80da-0cbfd4329006`, assistant message
`Sentinel_11b8bb3cdd3081918edbcc110458d118`, user message
`Sentinel_f93f96a34d948191974482997372d345`; delegated authorization includes
the observed 201–222ms snapshot cost and first-cache variation. The user also
said **“不要扩大范围”** at 06:06 UTC, message
`Sentinel_9a22ea582ba48191809db2fbad1a9a39`.

This is an explicit delivery-only risk acceptance for this R3 doctor/manual
guidance candidate. It permits normal signed feature delivery to
`release/v0.6.x`, subject to exact-head independent review and CI. It does not
lower targets, turn a performance failure into PASS, resolve the scanner
limitation, authorize scanner work, or cover future candidates. The bounded
verification disposition must be recorded separately from raw performance
results. Final disposition and its exact-state gate remain pending below.

### Preserved evidence and operational limits

All raw measurements, first/failing samples, source identities, xcresults and
logs remain under `/tmp/agentdeck-ls-candidate/native-performance`.
`startup-gap-diagnosis.json` SHA-256:
`9b7ca89420a08707d3f2c7f0775aaaf1755cf4cc954bd6dd774012854ac1ac89`.
`threshold-reconciliation.json` SHA-256:
`f755b8b365f816056b7ec1d6263ec0911e41caf3d5627a40a05217a28cd42996`.
Corrected derived files preserve originals and apply RSS budgets only to
unchanged refresh; no measurement was rewritten or repeated.

`cleanup-receipt.json` SHA-256:
`b34dae9236b049c6048464737cbe1460dcefd6ef69cf68192eebfdf6fa31e4f1`.
The initial restricted queries could not prove the real user-session registry
baseline; that limitation is retained. Subsequent live user-session checks
identified only the authorized test App/embedded Widget. Widget-only unregister
failed -10814; host unregister succeeded. Six exact test IDs and Widget/dump
queries then had no test records; seven production path/version records matched
the earlier verified native round. No owned process remained. The two exact
self-created test roots were removed after preserving evidence. No production
unregister, reset/rebuild, security change or recovery was performed.

Functional evidence: final Go suite, scoped race/vet, both Darwin builds and
arm64 size passed; authorized hosted native tests had 139 pass, 1 opt-in schema
skip and 0 failures. Earlier narrower harness failures remain historical.
Collision causality, WidgetTimeline and actual recovery remain unverified.

## Round 4 — 2026-10-04 — independent limited delivery disposition

Verdict: PASS
Completion gate: VERIFIED

Reviewer/method: cold read-only `/root/limited_delivery_review`; main verified
its exact hashes and source/evidence claims. Scope is the explicit delivery
decision and evidence reuse. The unchanged R3 code review is reused, not rerun.
Reviewed inputs: Round 3 record SHA-256
`aec4227017a160e3d15753abd6218212413a2452dfa4894efc24b8f5bef588e5`,
tasks SHA-256 `b274081512f8e21ab1b9a98b7e08955c5ed819389db2b437b815f1c4c3b8c8d2`,
requirements SHA-256
`065dffa81298e40da7135240ed25504d85f4918632ed4b8be97c5705014b913e`,
and unchanged 24-file R3 product/test identity. R3 patch SHA-256 remains
`104c3cff07437ee76a44856611f2b2f7564f3ac42c7fca64a8860440d24a40c7`.

ACR-F1 CLOSED by the real user's explicit limited delivery decision at
2026-10-04 06:19 UTC, with source and scope retained in Round 3. This closes the
feature's delivery objection without claiming that the original performance
targets passed or the existing scanner limitation was solved. The snapshot
cost and first-cache uncertainty remain disclosed. There is no residual finding
against this bounded disposition and no authorization for future exceptions.

The reviewer verified all 24 current product/test identities, the R3 patch and
referenced diagnostic/log hashes; no product tests or OS mutations were run.
Normal precedent is the existing snapshot-performance delivery-acceptance
exception. The new CEv1 observation must explicitly mean approved exception,
supersede the old acceptance evaluation, and retain failed observations/logs.
It must never roll up failed measurements as passing tests or change criteria.
An actual exact-target gate remains pending; signed-head review and CI are
separate delivery prerequisites. Evidence and scope-review receipt are retained
in `/tmp/agentdeck-ls-candidate/limited-delivery-independent-review.md`.

Exact-state gate follow-up: all three required acceptance criteria returned
VERIFIED through the approved delivery exception. Raw performance evidence
remains fail and is explicitly superseded only as a delivery-acceptance
evaluation; no failed observation is rolled up as passing tests. Standard MCP
read-back matches32nodes/60relations; receipt:
`/tmp/agentdeck-ls-candidate/delivery-decision-gates.json`. Final record/status
metadata is bound by a separate preservation roll-up before commit.
