---
status: historical
retired: 2026-09-07
created: 2026-09-07
---

# Fix: Beads Stop diagnostics crossed the active session boundary

## Observation

During a read-only workflow installation audit, the repository Stop Hook
required the current session to create six implementation tasks belonging to
`schema-version-signal`. That work was outside the active user scope. A Hook
diagnostic cannot authorize taking over unrelated coordination work.

Tracking: `ad-bug-beads-stop-session-scope`. The user authorized repair after
two-client new-session acceptance. No unrelated task, review verdict, or CEv1
record is changed by this repair.

## Cause

`findings` scanned every topic's approved decomposition and live Beads lifecycle
text, and `main` called it on every Stop without a session or subject filter.
Git working-tree changes identify candidate content, not the session responsible
for it. A shared actor name is also insufficient to establish session ownership.

## Repair boundary

- Capture explicit scope on UserPromptSubmit by reusing the shared workflow
  command matcher; keep phase authority with that workflow.
- Key local scope by repository, runtime, and session, and reject stale turns.
  Clear it on unmatched task changes; retain it for exact continuations.
- Check only the selected subject or explicitly selected whole topic. Missing
  scope or unavailable parser never turns into a repository-wide blocker.
- Keep whole-repository checks behind an explicit non-blocking `--audit`.
- Deduplicate unchanged reports using note and relevant document content;
  report changed or resolved-then-recurring mismatches again.
- Register the observer in both repository client configurations. Preserve the
  existing Stop transports and authorization-wait behavior.

This restores the existing scope/authorization boundary. Ordinary prose with no
supported explicit scope is deliberately unclassified. The repair does not
infer authorship from file timestamps, dirty paths, or task assignees, and does
not claim that silence proves Beads or evidence completion.

## Verification

Pre-repair regression: the no-scope Stop test failed because `findings` was
called; the scoped-decomposition test exposed the missing scope interface.
The initial test-fixture syntax error was corrected before recording this RED
result; it is not evidence of the product defect.

The affected subsystem command is:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest scripts.hooks.beads_consistency_test
```

Result: 59 tests passed. Coverage includes current-scope blockers for both
transports, absent scope, other sessions/runtimes/repositories, stale turns,
task switches, continuation, same-topic unrelated subjects, invalid paths,
content-bound deduplication, recurrence, and explicit non-blocking audit.
An additional isolated check exercised 20 scope-extraction cases against the
actual installed Codex and Claude shared workflow parsers; all passed.

Before repair, four real new CLI sessions exercised ordinary requests and an
explicit `claude-mem:how-it-works` invocation in both clients. All exited zero;
both loaded the explicit-only rule, neither ordinary request invoked a
claude-mem Skill, and explicit invocation remained available. Codex listed one
Fireworks entry; Claude had no pre-existing Fireworks registration and none was
added. The earlier local installation check verified three synchronized Skills,
removed Otty/TokenTracker registrations and preserved remaining configuration.

Live acceptance artifacts: `/private/tmp/agentdeck-cleanup-live-20260907/`.
Installation change/rollback index:
`/private/tmp/agentdeck-runtime-cleanup-20260907/applied.json`.
These artifacts are local and must be preserved through review.

Limits: Codex logged an MCP HTTP 409 and Claude reported a cancelled asynchronous
Stop Hook. These do not establish an all-MCP/all-Hook health PASS. The new scope
observer is verified through isolated event tests and actual-parser integration;
no post-repair live-client observer acceptance is claimed here.

## Review readiness

Implementation, scoped tests, Round 2 independent re-review, and post-repair
real-client observer acceptance passed. The fix was delivered in signed commit
`2d53d8e4c235f97ea037d7120bf5aae01188d99d`; its immutable Task gate is VERIFIED.
This record is retired after delivery and evidence finalization. No push was
performed.

## Review — Round 1 — 2026-09-07

## 📋 会话范围修复评审

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewer: Codex，独立评审角色；本会话未参与实现，未委派。
- Method: 当前差异、调用路径、既有测试及隔离反例检查。实现交接仅作为线索。
- Scope: Hook 脚本、聚焦测试、两端仓库注册、Toolchain 的观察器契约及本 Fix。
- Reviewed state: HEAD `eb564154682b888e93bc11b6ab35898b60aaf495`，
  六文件候选指纹 `4426defca9c18a9a02ad61f39a41f6993a7d74bdb365551784dba84518001d97`。
  配方为 `SHA256("head=" + HEAD + ";files=" + sorted_compact_json(path_to_sha256))`；
  六文件及逐文件 SHA-256 见交接的 `repair-content-state.json`，本轮已逐一核实一致。
- WorkUnit: `fix:beads-stop-session-scope`，无 containing topic。
- Completion gate: FAILED

### 🔴 严重问题 — 必须修复

**BSS-R1-F1 — 中等严重度 — OPEN：描述中的跨主题引用被当成任务归属。**

位置：`scripts/hooks/beads-consistency.py:630`，以及其后的
`by_record[("", anchor)]` 赋值和查找。

- 行为风险：任务描述只要含有 `docs/topics/alpha/` 就会被接受为 alpha
  的候选。同名 anchor 仍写入不含主题的键，后一个任务覆盖前一个任务。
  其他主题合理引用 alpha 的依赖契约时，当前会话会被要求迁移其他主题的
  Beads 任务，同时漏掉真正属于当前主题的任务，未恢复本 Fix 的授权边界。
- 证据：隔离临时仓库仅把 `docs/topics/alpha/reviews/build.md` 标为变更，
  内容为 `Verdict: PASS` 和 `Completion gate: VERIFIED`；scope 为
  `{"topic":"alpha","subject":"build"}`。模拟 `in_review` 返回以下顺序：

  | ID | Title | Description |
  | --- | --- | --- |
  | alpha-build | 任务：build | Implement docs/topics/alpha/tasks.md. |
  | beta-build | 任务：build | Implement docs/topics/beta/tasks.md; dependency contract: docs/topics/alpha/architecture.md. |

  其余 Beads 查询返回空数组。直接调用 `findings(root, deadline, scope=scope)`，
  得到唯一诊断：

  ```text
  docs/topics/alpha/reviews/build.md records Verdict: PASS, but its subject's task beta-build (alpha) is still `in_review` even though the completion gate is `VERIFIED`. Move it to `awaiting_commit`; see .agent-instructions/beads.md.
  ```

  `wrong_task_reported=true`，`correct_task_reported=false`，反例断言通过。
  原始结果：`/private/tmp/beads-stop-session-scope-review-reproducer.json`。
- 💡 限定修复：通过可验证的任务主题归属解析 `(topic, anchor)`；普通描述中的
  路径出现不能证明归属。无法唯一解析时不生成针对某个任务的状态迁移建议。
  增加跨主题同名 anchor、描述交叉引用及返回顺序变化的行为回归。

### 🟡 建议改进 — 推荐

无。上述问题属于本轮强制修复范围。

### 🟢 优点

会话状态键包含仓库、运行时和会话；缺少 scope 时不扫描全仓库。
显式 audit 与正常 Stop 分离，现有测试覆盖无范围及去重/复发行为。

### 📝 总结

已确定一个违反核心主题归属边界的反例，因此本轮 FAIL。生产代码、测试和
配置未修改。59 项回归及 20 个实际 parser 案例是实现交接中的既有结果，
六文件内容与交接哈希一致；本轮没有重跑这些检查，也不据此宣称所有运行时
行为已通过。按评审规则，决定性反例出现后停止广泛验证；修复后的真实
Codex/Claude observer acceptance 仍未执行，不能由模拟事件替代。

CEv1 已建立四项必需条件：范围隔离、回归保护、双端运行时验收和独立评审。
初始门禁为 NOT_VERIFIED；本轮反例记录为范围隔离失败。评审报告和状态同步
仅改变记录，未改变被复现的脚本、测试及注册，失败事实可绑定同步后的候选态。
任务 `ad-bug-beads-stop-session-scope` 退回修复；本 Fix 无 Topic 矩阵或 Topic 门禁。

下一步指令：`修复：fix / beads-stop-session-scope / BSS-R1-F1`

## Repair — BSS-R1-F1 — 2026-09-07

BSS-R1-F1 -> repaired in candidate; independent re-review pending. The Round 1
FAIL verdict and original counterexample remain historical evidence.

The repair removes description-path ownership inference and the topic-free
anchor lookup. An implementation candidate now resolves through a unique direct
`blocks` dependency on a task titled `文档：<topic> / tasks.md`, and that topic's
current Tasks matrix must contain the candidate anchor. When list output omits
dependencies, the Hook reads only the matching candidate's detail within its
existing deadline. An unavailable or mismatched detail yields no attribution.

Review candidates are keyed by `(topic, anchor)`. Multiple task identities or
contradictory statuses for that key yield no state-transition recommendation.
Ordinary references, multiple decomposition topics, or a missing matrix anchor
cannot establish ownership. No live task metadata or unrelated dependency graph
was rewritten to manufacture a match.

Failure-first verification reproduced four failing ownership tests against the
pre-repair candidate, including the recorded cross-reference failure. The final
affected-subsystem suite passed 65 tests. New coverage checks both return orders,
scoped and explicit-audit paths, description-only ambiguity, duplicate ownership,
conflicting decomposition dependencies, missing anchors, lazy detail resolution,
and unavailable or wrong-identity detail responses. Existing gate tests now
provide a verified decomposition association rather than a topic-free anchor.

Command: `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest scripts.hooks.beads_consistency_test`.
Previously verified parser behavior and client registrations are unchanged;
their evidence is reused, not presented as new runtime acceptance. Whitespace
and final diff checks are recorded with the repair evidence.

The final six-file candidate identity follows the Round 1 recipe and is retained
in `/private/tmp/beads-stop-session-scope-bss-r1-f1-state.json`. Repair evidence
is appended for that new state; prior failed observations are not overwritten.
Independent review and post-repair real-client observer acceptance remain
required before the WorkUnit can be completed. This repair does not issue PASS,
close the WorkUnit, commit, or push.

## Review — Round 2 — 2026-09-07

## 📋 BSS-R1-F1 修复复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Codex，独立复评角色；未参与修复，未委派。
- Method: 逐项核对 Round 1、修复交接、当前实现及行为断言；复用有效 CEv1 证据。
- Scope: BSS-R1-F1 的归属解析、调用方及相关回归，既有六文件修复边界不变。
- Reviewed state: HEAD `eb564154682b888e93bc11b6ab35898b60aaf495`，候选
  `078b725185e3bb90db6972242d2468dfece2faebe136da09d0e23ab290b77934`。
  已验证当前六文件与修复交接 manifest 全部一致；沿用 Round 1 指纹配方。
- WorkUnit: `fix:beads-stop-session-scope`；Beads: `ad-bug-beads-stop-session-scope`。
- Completion gate: NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。未发现本次修复引入的阻断问题。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**BSS-R1-F1 — CLOSED。** `implementation_subject` 在
`scripts/hooks/beads-consistency.py:602` 只接受唯一直接 `blocks` 分解文档关联，
并核对该主题矩阵确有 anchor；描述路径不再参与归属判断。
`findings` 按 `(topic, anchor)` 保存候选集合，只有唯一任务和状态才输出迁移建议。
因此原描述交叉引用反例不会再把 beta-build 当成 alpha 的任务。

已独立读取 `ReviewTaskOwnershipTest` 的断言：双返回顺序、scoped/audit、
仅描述引用、多个分解主题、重复任务、缺失 anchor、按候选 ID 加载 detail、
错误或不可用 detail 均有行为覆盖。原反例在无可验证归属时输出空诊断，
有效结构化关联的正例只输出 alpha-build，避免以全面静默掩盖错误匹配。

### 📝 总结

Round 1 唯一发现 BSS-R1-F1 已关闭，本轮 PASS；历史 FAIL 和失败证据保留。
CEv1 对修复候选返回范围隔离和回归两项可复用 passing evidence，
无 invalidated evidence 或 unresolved candidate impacts。
65 项测试已在修复阶段通过，本轮未因阶段变化重跑。
本轮新增的是独立复评及评审/状态文档的格式检查；产品代码、测试和注册未改。
最终同步候选及显式证据复用链由 CEv1 和
`/private/tmp/beads-stop-session-scope-rereview-r2-state.json` 记录。

剩余唯一门禁前提是修复后的真实 Codex/Claude observer acceptance：使用实际
仓库注册确认有范围与无范围事件的可归属诊断，区分模拟事件和真实客户端结果。
本轮未执行该验收，不以此前普通会话或 parser 测试替代。复评 PASS 不等于
WorkUnit 完成，任务保留 `in_review`；本 Fix 无 containing topic 门禁。

提交建议：门禁 VERIFIED 后，仅提交本 Fix 的脚本、测试、双端注册及相关文档
范围；先完成上述双端真实验收，再取得提交授权。
推送建议：门禁 VERIFIED 后，在上述验收完成、提交对象核验通过且取得推送
授权后执行；目标分支和远端尚未解析。

下一步指令：完成 fix / beads-stop-session-scope 修复后的真实 Codex/Claude
observer acceptance，并登记证据、查询 Task 门禁；保留本轮复评 PASS。

## Runtime acceptance — 2026-09-07

Round 2 PASS is preserved. No review was repeated and no Hook source, regression
test, shared parser, or registration was changed during acceptance.

The observed candidate was the synchronized Round 2 state
`021c72c38d7941e53df27fee6bacab6f358f3ada8471200c9f4dc14d47f2abe2`.
All six hashes matched its manifest before the runs and after acceptance.
Each client used two real new sessions: one explicitly selected alpha/tasks.md,
and one selected no scope. Both alpha and beta contained a synthetic approved
decomposition with an intentionally absent dispatch task.

| Client | Scoped session | Unscoped session | Observed behavior |
| --- | --- | --- | --- |
| Codex | `01a07f1a-c965-72e0-b70a-763cf1e529eb` | `01a07f1a-cd1f-7660-956d-caa665fa8064` | Alpha blocked once via stderr/exit 2; second Stop silent; unscoped Stop silent |
| Claude | `f71a4d9c-37d8-49d1-8949-2332aec794b2` | `0cb69b77-cb29-4d3e-83b8-e829d22c189f` | Alpha blocked once via JSON; second Stop silent; unscoped Stop silent |

All four client processes exited zero. Real UserPromptSubmit and Stop events,
session identities, output, and exit status were captured by a transparent PATH
recorder that forwarded event bytes and results unchanged. Only alpha appeared
in scoped diagnostics; beta and existing unrelated topics did not. Scripts and
registration definitions matched the reviewed bytes.

Claude used isolated repositories with the exact reviewed script and registration
files. Initial Codex isolated-directory attempts did not load project Hooks and
are excluded from the passing evidence. The documented `hooks/list` API confirmed
the fixtures had no project Hooks while the actual AgentDeck registrations were
enabled and trusted. Final Codex runs therefore used those actual registrations
with two temporary test topics in this repository. Both exact temporary files
were removed after verification; no existing topic or Beads task was reconciled.
Global trust settings and repository registrations were not modified.

Evidence: `/private/tmp/bss-observer-live-20260907/summary.json` records the four
sessions and event-log hashes. The same directory retains commands, raw client
events, transparent observer logs, effective Hook inventory, excluded attempts,
and the verifier. Unrelated MCP HTTP 409 and plugin asynchronous outcomes are not
an all-MCP/all-Hook health claim.

The runtime observation was appended to CEv1 at the observed Round 2 state; its
Task gate returned VERIFIED with no missing criteria or unresolved impacts.
Scope, regression, and independent review reused the existing valid evidence.
This acceptance/readiness update changes only the fix record within the six-file
candidate. Explicit target-bound roll-ups preserve the original observations for
the synchronized candidate; its manifest and final gate response are retained
in `/private/tmp/bss-observer-live-20260907/final-state.json` and `final-gate.json`.
Historical Round 2 gate status records what was known during that review; its
PASS is not replaced or reissued by this acceptance step.

## Delivery and retirement — 2026-09-07

Signed implementation commit: `2d53d8e4c235f97ea037d7120bf5aae01188d99d`.
Immutable delivery evidence: `/private/tmp/bss-delivery-20260907/implementation-gate.json`.
The Task gate returned VERIFIED for that commit. Retirement changes only this
record location/status and its current status pointer; review history is retained.
