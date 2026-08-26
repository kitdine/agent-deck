---
status: active
topic: switch-effectiveness-boundary
subject: requirements.md
---

# Review log — switch-effectiveness-boundary / requirements.md

## Round 1 — 2026-08-17
- Reviewed state: HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`; `requirements.md` blob `c752dca224b55730b0dc210125236f2d5182ec03`
- Reviewer: Codex
- Method: Formal design/contract review using the `development-workflow` Review dimensions; CodeGraph located the current provider-switch and usage-attribution paths, followed by focused source and contract inspection. No requirements-boundary checker exists; `scripts/check-topic-docs.sh` audits the document set during `tasks.md` review and was outside this target.
- Scope: `docs/topics/switch-effectiveness-boundary/requirements.md`; premise checks against the current Claude configuration writer, advisory, config-change route capture, price resolver, session-cost presentation, and the draft `usage-attribution-precision` boundary.
- Findings:
  - [P1] R1-F1 — The acceptance boundary requires undeterminable events to stop contributing subscription-rate cost, but the topic neither owns nor identifies a representation/read rule that can satisfy that outcome without violating its historical-recomputation and quality-redesign non-goals -> open; decide the v0.5.0 storage/read boundary or narrow the promised outcome and its version ownership.
  - [P1] R1-F2 — The requirement concludes that every credential-deleting `ConfigChange` route is unknowable without evaluating the same session's preceding `SessionStart` or route evidence, and it does not distinguish direct `official` from the mixed endpoint/token state of `official --via` -> open; define the evidence hierarchy and the exact cases that preserve a prior route versus become unknown.
- Evidence: `codegraph explore` confirmed `WriteClaudeConfig`, `ClaudeConfigMatchesSnapshot`, `RecordClaudeConfigChange`, and their call paths. Focused source inspection confirmed that unknown routes are stored as `("unknown", "1", false)` with quality `estimated` (`internal/usage/routes.go:45-78`), the price resolver adopts that multiplier (`internal/usage/usage.go:2617-2621`), summary and session invocation output still calculate and expose the resulting cost (`internal/usage/usage.go:3130-3171`; `internal/usage/session_usage.go:109-132`), and `SessionStart` plus `ConfigChange` routes share a session-specific history (`internal/usage/routes.go:22-53`). The draft architecture explicitly leaves multiplier `1` untouched for this topic (`architecture.md:165-169`). No tests were run after the decisive document blockers.
- Verdict: REOPEN

## 📋 需求文档评审报告

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:100`](../requirements.md#non-goals) R1-F1：验收边界要求不可确定的事件不再产生 subscription-rate 成本，但本主题没有拥有或指定可实现该结果的持久化/读取规则。
- 行为风险：实现者要么继续把 `unknown/1/estimated` 计入成本而违反 `requirements.md:146-148`，要么改变旧路由的读取解释而跨越 `requirements.md:100-104` 为 `v0.6.0` 保留的边界。
- 证据：当前 `RecordClaudeConfigChange` 把不匹配路由写为 `unknown/1/estimated`，`priceForEvent` 仍采用其 multiplier 并计算 summary/session cost；同主题 `architecture.md:165-169` 又明确不修改 multiplier `1`。
💡 有界修复：在需求中决定一个 `v0.5.0` 可实现的新路由表示与读取规则，保证新的不可确定事件不进入真实支出，且说明是否会重解释历史行；或明确缩小本主题的验收承诺并把该结果连同依赖关系移交 `v0.6.0`。

[`requirements.md:57`](../requirements.md#the-defect) R1-F2：“没有支持的机制”与“切换到 `official` 后一律 unknown”忽略了同一 session 已有的 `SessionStart`/前序 route 证据，也未覆盖 `official --via` 的端点已变但 token 未变的混合状态。
- 行为风险：对 direct `official` 可能丢弃已有的会话级路由证据，造成不必要的归因降级；对 `official --via` 又可能把混合状态错当成普通 direct switch。
- 证据：`RecordSessionRoute` 在 `SessionStart` 按 session ID 写入路由，`ConfigChange` 也带同一 session ID，而 `sessionRouteAt` 可查询事件前最近路由；需求自身 `requirements.md:143-145` 也承认 `SessionStart` 路由。
💡 有界修复：把 credential-deleting switch 拆成 direct `official` 与 `official --via`，定义“存在可用前序 session route 时延续哪个路由、何时因混合/缺失证据记为 unknown”的层级；若前序 route 不能作为证据，需在需求中给出可验证的理由。

### 🟡 建议改进——推荐

无。

### 🟢 优点

需求正确隔离了磁盘配置与运行中进程状态，保留 Codex 边界，禁止凭据泄漏，并把真实 Claude session 双向验证列为不可被单测代替的验收条件。

### 📝 总结

本轮评审绑定 HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025` 与 `requirements.md` blob `c752dca224b55730b0dc210125236f2d5182ec03`。方向性缺陷的核心观察成立，但成本输出的可实现边界与路由证据层级仍未决定，因此结论为 FAIL/REOPEN。本轮未执行真实 Claude session 实验；该运行时不确定性已被保留为后续验收门槛，不影响上述两个文档阻断项。

## Round 2 — 2026-08-17

- Reviewed state: repair of Round 1's two blocking findings. HEAD
  `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`; `requirements.md` blob
  `1b365117c57481dd1463576d71b6211e351d3f85`, and `architecture.md` blob
  `98dc4e913a2927120cb8a3d5d873b99b9855e8d5` for the consequential changes the
  repair required there.
- Reviewer: claude-code (repair round — an independent Re-review is required
  before the `Review` cell may be ticked)
- Scope: R1-F1 and R1-F2 as named in the repair command.

- Round 1 findings, dispositions:
  - **R1-F1** the acceptance boundary promised that undeterminable events stop
    contributing subscription-rate cost without owning a representation or read
    rule that could deliver it -> **Fixed.** The finding was correct and the
    premise under it was wrong: the draft assumed a new state had to be
    represented. Nothing new is represented. A new section, "What a
    credential-deleting switch attributes to", states that the existing
    resolution order already produces the right answer once the misleading row is
    not written — `sessionRouteAt` returns the most recent prior route
    (`internal/usage/usage.go:2504-2510`), and `priceForEvent` falls back to the
    provider timeline at session start when a session has none
    (`:2622-2634`). The section states the cost consequence explicitly: no new
    quality value, no column change, no migration, and no stored row
    reinterpreted, which is what keeps the promise inside the
    historical-recomputation non-goal rather than across it. The
    `unknown`/multiplier-`1` question is named as untouched and left to
    `usage-attribution-precision`, since this topic reaches that state no more
    often than today.
  - **R1-F2** the draft concluded every credential-deleting `ConfigChange` route
    was unknowable without evaluating the same session's prior route evidence,
    and did not separate direct `official` from the mixed `official --via` state
    -> **Fixed, and it invalidated the contract the draft was built on.** The
    finding is right that prior route evidence exists, and following it changes
    the answer rather than refining it: the provider is not unknowable, it is the
    selection the session started under. The route is therefore **not recorded at
    all** instead of recorded as `unknown` — suppressing the write is what makes
    the prior route continue to govern, and writing `unknown` would have replaced
    a correct attribution with an absent one at multiplier `1`, which the price
    resolver adopts verbatim. The evidence hierarchy is now a table with three
    cases and the quality each carries, and the "no prior route" case is shown to
    be reachable rather than theoretical (`internal/usage/routes.go:29,33-34`).
    The defect table is split into three rows so `official --via` is stated
    separately, with the ground for treating it like the direct row: the endpoint
    change is observable and the credential change is not, and billing follows
    the credential. The discriminant is named as credential deletion rather than
    provider name, since keying on the name puts the wrapper row on the wrong
    side.

- Consequential changes outside the two findings, all required by them:
  `architecture.md`'s contract 3 was rewritten from "record the unknown route" to
  "record no route", its `### Why not unknown` section records the superseded
  reasoning and why it was wrong, the wrapper row's verdict was aligned to
  `Not for billing`, and `tasks.md`'s task 2 was renamed
  `undeterminable-route-quality` -> `unadopted-switch-no-route` with its
  coverage restated around what `sessionRouteAt` returns rather than around a
  stored value. The Goals, Non-goals, User-visible surfaces, Contracts in scope,
  and Acceptance boundary sections were reworded wherever they asserted the
  unknown-route design.
- Attribution: the user identified the R1-F2 defect in conversation before this
  record existed, and Codex's Round 1 reached the same finding independently. The
  wrapper-state half of R1-F2 was Codex's alone.
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0; `make check-whitespace`
  -> exit 0; `git diff --check` -> exit 0. Every code claim added by this repair
  was read at the cited line on this HEAD. No product code, test, or
  configuration changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 3 — 2026-08-17

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`;
  `requirements.md` blob `1b365117c57481dd1463576d71b6211e351d3f85`.
  Consequential consistency was checked against `architecture.md` blob
  `98dc4e913a2927120cb8a3d5d873b99b9855e8d5` and `tasks.md` blob
  `0a223ec68b42691ad9acd8e7e4272010b35e6cf2`.
- Reviewer: Codex, independently re-reviewing claude-code's Round 2 repair.
- Method: Finding-by-finding design/contract Re-review. Reused the unchanged
  Round 2 document checks, then used focused CodeGraph source inspection for
  the repaired prior-route, timeline-fallback, and restart/`SessionStart`
  premises. Cross-target defects were attributed to their owning documents
  rather than carried as false findings against `requirements.md`.
- Scope: R1-F1 and R1-F2, their consequential contract statements, and new
  blocking regressions caused by the repair.
- Finding dispositions:
  - **R1-F1 — CLOSED.** The repaired requirement introduces no unknown route,
    quality value, schema field, migration, or historical reinterpretation. A
    credential-deleting switch writes no `ConfigChange` route, so the existing
    resolver continues using the same session's prior route or, when none
    exists, the provider timeline at session start. New events therefore retain
    the provider multiplier the running session is still billing at while old
    stored rows keep their meaning.
  - **R1-F2 — CLOSED.** The direct `official` and `official --via` cases are
    stated separately, credential deletion is the discriminant, and the full
    evidence hierarchy now decides prior route, timeline fallback, and the
    pre-existing no-coverage fallback. The requirement no longer discards a
    known prior route as unknown.
- Cross-target attribution:
  - `architecture.md` still titles Contract 3 "an undeterminable route is
    unknown" although the repaired body specifies that no route is written.
    This belongs to the next `architecture.md` review and does not reopen
    `requirements.md`.
  - `tasks.md`'s `real-session-acceptance` step 5 still asks the operator to
    confirm an unknown route, contradicting the repaired no-route contract.
    This belongs to `tasks.md` and must close in that document's own review.
- Evidence: CodeGraph confirmed `readPriceResolver.sessionRouteAt` returns the
  latest prior route, `RecordSessionRoute` writes completed selections only at
  `SessionStart`, and Claude `startup`, `resume`, and `clear` are valid
  `SessionStart` sources. Round 2's `scripts/check-topic-docs.sh`,
  `make check-whitespace`, and `git diff --check` results remain reusable for
  the unchanged candidate. Completion Evidence Profile v1 gate
  `switch-effectiveness-boundary:requirements.md` is `VERIFIED` for the reviewed
  HEAD plus requirements blob, with evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:requirements.md:rereview-round-3:1b365117c57481dd1463576d71b6211e351d3f85`.
- Verdict: PASS

### 📋 需求文档独立复评

📊 总体评分：9/10

✅ 复评结论：PASS

#### 🔴 严重问题——必须修复

无（`requirements.md` 自身）。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- R1-F1 已关闭：“不写新 route”让现有 session route / session-start
  timeline 自然继续生效，无需新质量值、migration 或历史重解释。
- R1-F2 已关闭：direct `official` 与 `official --via` 已分开说明，
  credential deletion 是判别条件，路由证据层级完整且可实现。
- 真实 Claude session 双向验收仍保留为后续 runtime gate，没有被
  磁盘文件单测替代。

#### 📝 总结

R1-F1、R1-F2 均已在精确 HEAD+blob 内关闭，无新的
`requirements.md` finding。`architecture.md` 的旧标题与 `tasks.md` 的
unknown-route 验收步骤已按所有权归属到后续文档评审，不伪造为
本目标的未关闭 finding。

#### Task checkpoint

- Task：`switch-effectiveness-boundary / requirements.md` @ HEAD
  `10ce01e790d5330e632da081cfa681f36cb9e086` + blob
  `1b365117c57481dd1463576d71b6211e351d3f85`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅纳入 `requirements.md`、本评审记录与
  `tasks.md`/`docs/README.md` 的该 Task 状态 hunk；排除尚未评审的
  architecture/tasks 行为内容与其他 dirty work。
- 推送建议：只在上述 Task 边界能被安全隔离、获得明确 commit
  与 push 授权、并核对目标分支/远端后推送；本 checkpoint 不执行也不授权交付。

## Round 4 — 2026-08-25

- Reviewed state: HEAD `6ec680adcb9ab65fa05622140100b4e6cdba57cf`;
  `requirements.md` blob `02ce6a26bc02d53e3e515529b6fa10269f0c5a1f`.
- Reviewer: Codex
- Method: Formal design/contract review using the `development-workflow` Review
  dimensions. CodeGraph resolved the current configuration-write,
  reconciliation, session-route, pricing-fallback, and Claude lifecycle paths;
  focused source and living-contract inspection checked the exact premises and
  cross-topic boundary. No requirements-boundary checker exists;
  `scripts/check-topic-docs.sh` ratifies the document set during `tasks.md`
  review and does not judge this target.
- Scope: `docs/topics/switch-effectiveness-boundary/requirements.md`; premise and
  scenario checks against the current Claude configuration writer and advisory,
  `ConfigChange` reconciliation, route ordering, provider timeline snapshots,
  session-start handling, wrapper semantics, and the current
  `usage-attribution-precision` draft.
- Findings:
  - [P1] R4-F1 — The Goals section says an unadopted switch keeps the provider
    the session “started with”. That is false for the newly added sequence
    `no key -> first key A -> key B`: the session started on `official`, adopted
    A live, and must keep A across the unadopted rotation. The document's own
    defect explanation and route hierarchy correctly give the latest prior
    effective route precedence, so the goal contradicts both of them -> open;
    define the retained provider as the latest adopted effective session route,
    using the provider timeline at session start only when no route exists.
  - [P2] R4-F2 — Two audit references no longer identify the facts they claim:
    the mixed `official --via` state is the fourth table row, not the third, and
    `cmd/agentdeck/main.go:2947` is the retry sleep rather than the matched
    `RecordClaudeConfigChange` call, which is currently at line 2939 -> open;
    name the wrapper row directly (or correct its ordinal) and update the call
    citation to the current source location.
- Evidence: CodeGraph source for `WriteClaudeConfig`,
  `ClaudeConfigMatchesSnapshot`, `reconcileClaudeConfigChange`,
  `RecordClaudeConfigChange`, `RecordSessionRoute`, both `sessionRouteAt` /
  `priceForEvent` paths, `ProviderSnapshot`, and Claude `SessionStart` parsing.
  Focused source inspection confirmed the read-time route precedence at
  `internal/usage/usage.go:2617-2634`, provider-timeline credential snapshots at
  `internal/store/providers.go:819-950`, and the matched reconciliation call at
  `cmd/agentdeck/main.go:2939`. The current
  `usage-attribution-precision` requirements and architecture agree that only
  `no key -> first key` adds a live route. L0 record checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — R4-F1 and R4-F2 leave the current document
  criterion open; no CEv1 evidence was written for this content state.
- Verdict: REOPEN

## 📋 需求文档评审报告

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:111`](../requirements.md#goals) R4-F1：Goals 把未采纳切换后的
有效 provider 写成 session “started with”的 provider，与新增的 first-key 后再换 key
场景及本文自己的路由层级冲突。
- 行为风险：在 `official -> first key A -> key B` 中，实现者可能错误回退到 session-start
  的 `official`，而不是继续使用已被运行中进程采纳并已记录的 A route，重新造成错误
  multiplier 与成本归因。
- 证据：本文 `requirements.md:79-83` 已说明 first-key route 记录 A，
  `requirements.md:127-143` 又规定 prior route 先于 session-start timeline；当前
  `sessionRouteAt` 与 `priceForEvent` 也按此顺序解析。
💡 有界修复：把该 Goal 改为保留“最后一个已采纳的有效 session route”；仅在 session
没有 route 时才使用 session-start provider timeline，并保留无覆盖时的既有 fallback。

### 🟡 建议改进——推荐

[`requirements.md:44`](../requirements.md#the-defect) R4-F2：wrapper 混合态的行号与
matched reconciliation 的源码行号均已失真。
- 行为风险：读者会把 direct `official` 的第三行误当成 endpoint/token 混合态；同时
  `main.go:2947` 无法审计“matched route 被写入”的事实，因为该行当前只是 retry sleep。
- 证据：表格中 `official --via` 是第四行；当前调用位于
  `cmd/agentdeck/main.go:2939`。
💡 有界修复：直接按名称引用 `official --via` 行（或改成第四行），并把调用引用更新为
当前位置。

### 🟢 优点

- 新草稿正确把唯一可能热更新的状态限定为 `no key -> first key`，不再把所有 custom
  switch 视为立即生效。
- prior route、session-start timeline 与无覆盖 fallback 的层级清楚，无需 schema
  migration 或历史重解释。
- settings mismatch、凭据隐私、Codex 非目标以及真实 Claude session 验收边界均有明确
  约束，并与当前 `usage-attribution-precision` 草稿一致。

### 📝 总结

本轮绑定 HEAD `6ec680adcb9ab65fa05622140100b4e6cdba57cf` 与当前未提交
`requirements.md` blob `02ce6a26bc02d53e3e515529b6fa10269f0c5a1f`。新增状态机方向
基本正确，但 Goals 对 first-key 后再换 key 的 retained-provider 定义仍会导向错误实现，
并有两处可审计引用失真，因此结论为 FAIL/REOPEN。真实 Claude session 仍是后续实现
验收门禁；本轮未用磁盘文件测试替代该运行时证据。

## Round 5 — 2026-08-25

- Reviewed state: repair of Round 4's two authorized findings. HEAD
  `6ec680adcb9ab65fa05622140100b4e6cdba57cf`; `requirements.md` blob
  `c762becde8c1480739ae6111f8faa5832bd3dce3`.
- Repairer: Codex (repair round — an independent Re-review is required before
  the `Review` cell may be ticked again).
- Scope: R4-F1 and R4-F2 only.
- Round 4 findings, dispositions:
  - **R4-F1** the Goals section returned every unadopted switch to the provider
    at session start, contradicting the `no key -> first key A -> key B`
    sequence and the document's own route precedence -> **Fixed.** The Goal now
    preserves the latest prior effective session route first, uses the provider
    timeline at session start only when no session route exists, and retains the
    existing no-coverage fallback. It explicitly states that a first-key route
    survives a later unadopted rotation.
  - **R4-F2** the wrapper mixed-state paragraph called `official --via` the
    third table row and cited the matched reconciliation call at stale line 2947
    -> **Fixed.** The paragraph now names the `official --via` row directly, and
    the matched `RecordClaudeConfigChange` citation points to
    `cmd/agentdeck/main.go:2939`.
- Evidence: CodeGraph and focused source inspection confirmed the matched call
  at `cmd/agentdeck/main.go:2939`. Focused document inspection confirmed the
  prior-route-first hierarchy already stated below the Goal and the absence of
  the two stale references from the repaired requirements candidate. No product
  code, test, configuration, architecture, task decomposition, or unrelated
  topic document changed in this repair.
- Completion gate: NOT_VERIFIED — Repair closes the named findings but does not
  grant an independent review verdict or create completion evidence.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 6 — 2026-08-25

- Reviewed state: HEAD `6ec680adcb9ab65fa05622140100b4e6cdba57cf`;
  `requirements.md` blob `c762becde8c1480739ae6111f8faa5832bd3dce3`.
- Reviewer: Codex, independently re-reviewing the Round 5 repair; this workflow
  turn did not author that repair.
- Method: Finding-by-finding design/contract Re-review under the
  `development-workflow` REREVIEW dimensions. The unchanged HEAD-bound
  CodeGraph/source evidence from Round 4 was reused; focused inspection checked
  the repaired requirements text and the exact current matched-reconciliation
  citation. No requirements-boundary checker exists; the topic document-set
  checker belongs to `tasks.md` review.
- Scope: R4-F1 and R4-F2, their repaired requirements statements, and new
  blocking regressions caused by those edits.
- Finding dispositions:
  - **R4-F1 — CLOSED.** The Goal now resolves an unadopted switch first through
    the latest prior effective session route, then through the provider timeline
    at session start only when the session has no route, followed by the existing
    no-coverage fallback. It explicitly preserves first-key route A across a
    later unadopted rotation to key B, matching the document's route hierarchy
    and current `sessionRouteAt` / `priceForEvent` precedence.
  - **R4-F2 — CLOSED.** The mixed-state paragraph now names the
    `official --via` row rather than using the wrong ordinal, and the matched
    `RecordClaudeConfigChange` call is cited at current
    `cmd/agentdeck/main.go:2939`.
- New blocking findings: none.
- Evidence: focused inspection of `requirements.md:29-51,69-117`; current
  `cmd/agentdeck/main.go:2939`; reused unchanged-HEAD route-precedence evidence
  at `internal/usage/usage.go:2617-2634`; targeted stale-reference scan; L0
  final-state checks: stale-reference scan -> no matches;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0. CEv1 evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:requirements.md:rereview-round-6:c762becde8c1480739ae6111f8faa5832bd3dce3`
  satisfies the document criterion for the exact reviewed state.
- Completion gate: VERIFIED
- Verdict: PASS

## 📋 需求文档独立复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R4-F1 已关闭：first-key route 在后续未采纳 key rotation 中保持最高优先级，只有无
  session route 时才回退 session-start timeline。
- R4-F2 已关闭：wrapper 混合态与 matched reconciliation 调用均由当前、可审计的名称
  和位置标识。
- 修复没有扩张 requirements 边界，也没有改变 schema、历史解释、Codex 非目标或真实
  Claude session 验收门禁。

### 📝 总结

R4-F1、R4-F2 均在 HEAD `6ec680adcb9ab65fa05622140100b4e6cdba57cf` +
`requirements.md` blob `c762becde8c1480739ae6111f8faa5832bd3dce3` 中关闭，无新
blocking finding，复评结论为 PASS。剩余运行时不确定性由后续
`real-session-acceptance` Task 按既有契约验证，不是 requirements 文档的开放 finding。

### Task checkpoint

- Task：`switch-effectiveness-boundary / requirements.md` @ HEAD
  `6ec680adcb9ab65fa05622140100b4e6cdba57cf` + blob
  `c762becde8c1480739ae6111f8faa5832bd3dce3`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅纳入完整的已评审 `requirements.md`、本 requirements 评审记录、
  `tasks.md` 的 requirements Review cell/当前状态 hunk，以及 `docs/status.md` 的
  Switch Effectiveness Boundary 状态 hunk；排除尚未评审的 architecture/tasks 行为
  内容、其他 topic、roadmap 与无关 dirty work。
- 推送建议：目标分支与远端尚未解析；仅在获得明确 commit 与 push 授权、按上述 Task
  边界形成并核验签名提交后推送。本 checkpoint 不执行也不授权交付。
