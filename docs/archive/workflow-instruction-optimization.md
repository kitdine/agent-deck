---
status: historical
created: 2026-09-07
retired: 2026-09-07
---

# Workflow-instruction optimization closeout

## Accepted boundary

The operator approved whole-document revisions to `AGENTS.md`,
`docs/documentation-workflow.md`, and the routed `project-rules.md`,
`evidence.md`, `toolchain.md`, `beads.md`, and `branching.md` authorities.
Review Records required no further change. These living files own the rules;
this record preserves the assessment and acceptance, not another rule set.

The component decisions remain: synchronize the selected shared Skills, retain
one Fireworks discovery entry, require explicit claude-mem Skill invocation,
remove Otty/TokenTracker hooks, and retain the other installed components.
The separately delivered [Beads observer fix](fixes/beads-stop-session-scope.md)
keeps its Round 2 PASS, real Codex/Claude acceptance, and immutable VERIFIED
gates for signed commits `2d53d8e` and `ad8b9fd`. None was re-reviewed or retested.

The operator selected Codex-only acceptance because Claude quota was insufficient;
Claude acceptance is deferred and does not block this closeout. After the
acceptance, the operator explicitly chose to stop before CEv1 recording, then
authorized necessary handoff and commits to leave a clean working tree. No push
was authorized.

## Advisory review and content identity

A separate read-only Codex review of the integrated rules found no blocking
conflict in ownership, triggers, authorization, evidence reuse, or verification
scope. It inspected installed shared Skills/Hooks, both user-level instruction
files, configured MCP capabilities, and current repository authorities. No
formal review phase, review round, or completion receipt was created.

Reviewed HEAD: `ad8b9fdb3b85c97b0c12c9b8cf54e2ab424de166`.
The seven-file SHA-256 map is:

| File | SHA-256 |
| --- | --- |
| `AGENTS.md` | `fd73fe0b5d1f304e97cc706b1af56d6be81d40cdd45c7061db4ff92f3d91729e` |
| `docs/documentation-workflow.md` | `d4b1c42c0c8842516e0dcc192c3212e7a80ef2fab0ad09430a7f95d5cb801369` |
| `.agent-instructions/project-rules.md` | `db4e4ad21f37496c597b8709fb4c53499fdeb0e741ccfe1f41f5146c9e93b932` |
| `.agent-instructions/evidence.md` | `1af7bd46705de33a6a855bf9d159e049243d7032672054939574fe001b155adf` |
| `.agent-instructions/toolchain.md` | `d5732b098202f1fd33860116bda517887a8b24db23cf8a2b59206e4dad1d434d` |
| `.agent-instructions/beads.md` | `6cd37af75ec95878397c69ab2c5d6b598a9f27a19c6487a5ca6211822772cc67` |
| `.agent-instructions/branching.md` | `82e4c0f10b695843a2d5cc56bcf0e45c4619a94dc7b9564cf247a256fe73a91d` |

The combined fingerprint is
`92826cd7cdbf19b4ca3d9cd4efdc7efa5cd2a39eae669f54b2204d7259eb0f4b`,
using `SHA256("head=" + HEAD + ";files=" + sorted_compact_json(path_to_sha256))`.
Delivery reuses these observations for unchanged scoped files; it does not
relabel the observed HEAD or claim a new immutable CEv1 gate.

## Codex representative acceptance

Twenty-six real new Codex CLI 0.153.4 sessions used GPT-6 Astra / medium.
A used the seven HEAD documents; B used the approved candidate. All five
scenario types passed their bounded functional and authority checks:

- Self-contained translation: correct answer, no tool call.
- Named-source lookup: correct endpoint diagnosis, no mutation or test run.
- Synthetic Go repair: only two allowed files changed; meaningful endpoint,
  singleton, reversed-range and integer-extreme regressions passed through
  `scripts/run-go-test.sh ./fixture`, without unrelated packages or `./...`.
- Evidence reuse: current file/log identities were checked, and valid tests
  were reused without rerunning them.
- Delivery preparation: no staging, commit, push or branch creation; proposed
  messages contained a subject, body and Codex trailer, while actual commit
  object and SSH-signature checks remained explicitly unexecuted.

Repair and delivery preparation each used five A/B pairs; the other scenarios
used one pair. The main agent checked source/test snapshots, actual commands,
logs, injected instruction content, model settings and state hashes rather than
accepting client summaries as proof. Instrumented workflow/handoff lifecycle
events returned successfully without starting a formal phase or forcing sync.

| Observed median | A | B |
| --- | ---: | ---: |
| Repair elapsed seconds, five samples | 106.267 | 85.921 |
| Repair input tokens, including cache | 143,530 | 119,064 |
| Delivery-preparation elapsed seconds, five samples | 82.400 | 66.587 |
| Delivery-preparation input tokens, including cache | 128,194 | 96,398 |

Across the 13 matched sessions per group, input tokens were 1,612,873 versus
1,356,798; elapsed seconds were 1,106.674 versus 934.927; tool calls were 42
versus 38. These are observations of this sample mix, not a universal speedup
or a measured bill reduction. Provider caches and live memory context were not
fully controlled, and generated test details varied while satisfying the same
contract. Earlier samples could appear in later memory context.

## Limits, evidence and cleanup

The record retains the initial pre-model sandbox startup failure, one malformed
candidate `sed` command and its corrected read, an unrelated CodeGraph result
for an exact file query, and MCP HTTP 409 startup messages. No all-MCP/all-Hook
health claim is made. Fixture observer registration was not captured; the
existing real-client observer acceptance remains separate. Hook timings cover
the instrumented shared handlers, not the complete plugin chain or model-only
latency. Actual Git delivery was outside the synthetic acceptance.

Raw local evidence remains under
`/private/tmp/agentdeck-codex-acceptance-20260907/`: `REPORT.md`, `summary.json`,
`metrics.json`, `manifest.json`, `runtime-manifest.json`, per-session commands
and events, test logs, and `snapshots/`. These local artifacts are retained for
inspection and are not repository-portable logs. The essential result and
candidate identity are preserved in this committed record.

The client persisted trust entries for the two temporary fixture directories.
Removing exactly those entries reproduced the initial configuration hash;
`cleanup-result.json` records successful restoration. The seven candidate
documents and unrelated production files were unchanged by acceptance.

The proposed CEv1 acceptance WorkUnit was absent (`NOT_VERIFIED`,
`missing_work_unit`). Automatic approval rejected the initial node write before
execution, and the operator chose to stop there. No new WorkUnit, criterion,
evidence node, relationship, or VERIFIED gate was created for this acceptance.
Local observations are not presented as an alternative CEv1 provider. The
operator-approved delivery preserves that decision and the existing Fix gates.

Further instruction edits require a new complete per-document proposal and
approval. Claude acceptance and any future CEv1 registration are separately
scoped follow-ups, not automatic continuation of this closed audit.
