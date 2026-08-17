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
