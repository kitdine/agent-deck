---
status: active
topic: switch-effectiveness-boundary
subject: architecture.md
---

# Review log — switch-effectiveness-boundary / architecture.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`;
  `architecture.md` blob `98dc4e913a2927120cb8a3d5d873b99b9855e8d5`.
- Reviewer: Codex.
- Method: Formal design/contract review against the approved requirements and
  current provider/config-change/usage-route source. CodeGraph verified the
  available credential-presence metadata and the route-resolution call paths.
  No architecture-specific checker exists; the topic document-set checker does
  not validate contract semantics.
- Scope: `architecture.md`'s three contracts, corrected v0.6.0 premise, and
  verification boundary. `tasks.md` was inspected only for cross-target
  consistency.
- Findings:
  - [P1] A-F1 — The opening premise says `official` is the only selection that
    carries neither owned field and reduces the switch to one delete branch,
    contradicting the immediately following `official --via` row where the
    endpoint is written and only the credential is deleted -> open; state the
    three field shapes directly and derive the discriminant only from credential
    deletion.
  - [P1] A-F2 — Contract 3's title and the v0.6.0 correction still describe an
    unknown route and a resulting quality even though the repaired contract says
    no `ConfigChange` route exists -> open; rename the contract and describe the
    retained prior route's quality semantics without inventing a nonexistent
    route.
- Cross-target attribution: `tasks.md`'s `real-session-acceptance` step 5 still
  asks for an unknown route. That defect belongs to `tasks.md`; it is not a
  third finding against `architecture.md`.
- Evidence: `WriteClaudeConfig` has independent endpoint and credential branches;
  `official --via` writes `ANTHROPIC_BASE_URL` while deleting
  `ANTHROPIC_AUTH_TOKEN`. `ProviderSnapshot.Credential` is the persisted
  credential name, so the asynchronous reconcile can safely determine
  credential presence without plaintext. `architecture.md:31`, `:119`, and
  `:228-229` retain the contradictory former design. Broad checks stopped after
  these decisive document blockers.
- Verdict: REOPEN

### 📋 Architecture Design Review

📊 总体评分：6/10

✅ 评审结论：FAIL

#### 🔴 严重问题——必须修复

[`architecture.md:31`](../architecture.md) A-F1：开头前提把 `official` 简化为
“两个字段都不携带”的单一 delete branch，与紧随其后的
`official --via` 表格矛盾。
- 行为风险：`official --via` 会写 endpoint 但删 credential；实现者若按
  前提的“整个分支写/删”模型实现，会在 wrapper 方向选错
  effectiveness/route 判别。
- 证据：`WriteClaudeConfig` 对 endpoint 和 credential 使用两个独立分支；
  本文档的 wrapper 行也明确是 endpoint written / credential deleted。
💡 有界修复：将前提改成 direct official、official via wrapper、custom
三种字段形状，只从 `Credential == ""` 推导判别条件，不再声称
official 两个字段都不携带。

[`architecture.md:119`](../architecture.md) A-F2：Contract 3 标题仍是“an
undeterminable route is unknown”，v0.6.0 段落仍要求决定“such a route”的
quality，但合同正文已改为完全不写 `ConfigChange` route。
- 行为风险：后续实现或 v0.6.0 设计可能为一个不存在的 route 分配
  quality，或重新引入已被 requirements Re-review 否定的 unknown write。
- 证据：同文档 Contract 3 正文明确“records no `ConfigChange` route at
  all”，且 `Why not unknown` 已说明 unknown 会丢弃正确的旧路由。
💡 有界修复：将 Contract 3 重命名为 unadopted switch 不写 route，并把
v0.6.0 段落改成“保留的前序 route 使用哪个已存 quality”，而不是
“新 route 的 resulting quality”。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- advisory 与 attribution 共用 credential-deletion 判别，并明确覆盖
  `official --via` 的混合状态。
- 不写 route 的新合同正确复用了现有 prior-route/session-start timeline
  解析，没有新增 schema、quality 或历史重解释。
- 真实 Claude session 验收与自动化证据的边界划分清楚。

#### 📝 总结

评审绑定 HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf` 与
`architecture.md` blob `98dc4e913a2927120cb8a3d5d873b99b9855e8d5`。主要合同已经
转向正确的 no-route 设计，但前提和两个标题/结论仍保留旧的
unknown-route 模型；在 A-F1、A-F2 关闭前不能 PASS。

#### 📌 下一步

```text
修复：switch-effectiveness-boundary / reviews/architecture.md / A-F1 A-F2
```

## Round 2 — 2026-08-17

- Reviewed state: HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`;
  `architecture.md` blob `fd0919372bab8a37b2d7e9f0c6205603d40cefe0`.
- Reviewer: Codex, independently evaluating the changed candidate after the
  repair process was manually cancelled before it could write its own round.
- Method: Finding-by-finding design/contract Re-review. Reused the unchanged
  current-source evidence from Round 1 and re-read only the A-F1/A-F2 sections
  plus their approved-requirements context.
- Scope: A-F1, A-F2, and new blocking regressions caused by their repair.
- Finding dispositions:
  - **A-F1 — CLOSED.** The premise now lists the three field shapes separately:
    custom writes endpoint and credential, direct `official` deletes both, and
    `official --via` writes the endpoint while deleting the credential. It
    explicitly rejects a shared write/delete-branch model and derives billing
    effectiveness only from credential deletion.
  - **A-F2 — CLOSED.** Contract 3 is renamed to say a credential-deleting
    switch records no route. The v0.6.0 correction now states that no resulting
    `ConfigChange` route exists, the retained prior route supplies its existing
    provider/multiplier/quality, the session-start fallback remains unchanged,
    and this topic assigns no new quality.
- Cross-target attribution: `tasks.md:99` still says "the resulting quality
  decision". Whether that wording accurately refers to retained evidence
  sources belongs to the upcoming `tasks.md` review; it is not an open finding
  against this architecture.
- Evidence: `architecture.md:30-42`, `:128-183`, and `:233-242` now agree with
  approved `requirements.md:113-158`. Current source evidence remains unchanged:
  endpoint and credential writes are independent, `ProviderSnapshot.Credential`
  exposes only the credential name, and a missing later route preserves the
  prior session route. Completion Evidence Profile v1 gate
  `switch-effectiveness-boundary:architecture.md` is `VERIFIED` for this
  HEAD-plus-blob state, with evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:architecture.md:rereview-round-2:fd0919372bab8a37b2d7e9f0c6205603d40cefe0`.
- Verdict: PASS

### 📋 Architecture Independent Re-review

📊 总体评分：9/10

✅ 复评结论：PASS

#### 🔴 严重问题——必须修复

无。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- A-F1 已关闭：三种字段形状与 credential-deletion 判别条件一致。
- A-F2 已关闭：Contract 3、`Why not unknown` 和 v0.6.0 更正段落
  现在只描述 no-route 以及保留的已存 evidence source。
- 合同无需明文 credential、新 schema、新 quality 或历史重解释。

#### 📝 总结

A-F1、A-F2 均已在 blob `fd0919372bab8a37b2d7e9f0c6205603d40cefe0`
中关闭，没有新的 architecture finding。`tasks.md:99` 的措辞已归属到
下一份文档的评审，不作为延后本目标 finding 的借口。

#### Task checkpoint

- Task：`switch-effectiveness-boundary / architecture.md` @ HEAD
  `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf` + blob
  `fd0919372bab8a37b2d7e9f0c6205603d40cefe0`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅提交 `architecture.md` 与本评审记录；状态文档只在能
  安全隔离该 Task hunk 时纳入，排除未评审的 `tasks.md` 行为内容和其他 dirty work。
- 推送建议：在获得明确 commit/push 授权、检查目标分支与远端，且
  已推送的提交精确包含该 Task 后再推送；本 checkpoint 不执行也不授权交付。

## Round 3 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `f5465dc5096cc907b6953083456a1530d9bc514b`.
- Reviewer: Codex
- Method: Formal design/contract review against the newly approved requirements
  and current source, using the project Review dimensions. CodeGraph resolved
  the advisory, provider-selection, configuration-write, asynchronous
  reconciliation, session-route, provider-timeline, and pricing call paths.
  No architecture-specific checker exists; the topic document-set checker does
  not validate contract semantics.
- Scope: the three architecture contracts, prior-authentication classification,
  cross-topic attribution premise, verification boundary, and every current-code
  premise the document names. `tasks.md` was read only as downstream context.
- Findings:
  - [P1] A3-F1 — Contract 1's no-credential advisory says a running session
    keeps “the authentication and configuration it started with”. That is false
    after the supported `no key -> first key A` transition: a later removal
    leaves the process on A, not on its startup `official` authentication. It
    also contradicts the architecture's `official --via` premise, where the
    endpoint may change while the old token remains -> open; state only the
    guaranteed boundary: key removal does not re-authenticate an already-keyed
    session, adoption of other configuration changes is not established, and a
    restart is required to guarantee the new selection.
  - [P1] A3-F2 — The prior-state rule says “a prior `official` route or
    session-start timeline entry supplies the no-key side”, without classifying
    the timeline snapshot's credential. A session can lack a route while its
    session-start snapshot is a custom provider with a non-empty credential;
    treating that entry as no-key misclassifies `key A -> key B` as the first-key
    exception and records the unadopted B route -> open; specify the exact
    ordered classifier: latest known route first, then `ProviderSnapshotAt` at
    session start, with non-empty `Credential` proving keyed, empty/official
    proving no managed key, and missing/unknown/conflicting evidence remaining
    indeterminate. Name the owner/API that obtains session-start time and keeps
    classification with the conditional write.
  - [P2] A3-F3 — The opening audit statement binds every existing-code claim to
    old `main` HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`, while this
    review is against HEAD `56097366fa7fa4c275750a03387346d98f51dc57` and some
    cited locations have moved -> open; bind the source premise to the current
    reviewed HEAD or defer exact content identity to the review record while
    retaining current per-symbol locations.
- Evidence: current CodeGraph source shows `SwitchAdvisories` lacks per-session
  state (`internal/provider/service.go:780-805`), custom provider definitions
  require a credential reference (`internal/provider/provider.go:39-43`),
  `ProviderSnapshot.Credential` carries the non-secret credential name
  (`internal/store/providers.go:86-94`), and `usage_session_routes` stores no
  credential field (`internal/usage/routes.go:56-78`). Current reconciliation
  receives the session ID and current snapshot but has no specified prior-state
  classifier (`cmd/agentdeck/main.go:2922-2952`). Broad verification stopped
  after the decisive contract blockers. L0 review-record checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A3-F1, A3-F2, and A3-F3 leave the current
  architecture criterion open; no CEv1 evidence was written for this state.
- Verdict: REOPEN

## 📋 Architecture 设计评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:74`](../architecture.md#contract-1--the-switch-advisory-is-direction-aware)
A3-F1：no-credential advisory 错误承诺 session 会保留启动时的 authentication 和
configuration。
- 行为风险：`official -> first key A -> remove key` 会保留 A，而不是回到启动时的
  `official`；`official --via` 又可能热更新 endpoint，因此该 copy 在两个既定场景中都
  会向用户陈述错误运行态。
- 证据：本文状态表和已通过 requirements 都把 first-key 视为可采纳 route，并把
  wrapper 定义为 endpoint/token 混合态。
💡 有界修复：copy 只陈述可保证事实——删 key 不会让 already-keyed session
重新认证，其他配置是否被采纳未建立，必须重启才能保证新 selection 完整生效。

[`architecture.md:56`](../architecture.md#the-asymmetry-this-design-is-built-on)
A3-F2：prior-state classifier 没有区分带 credential 与不带 credential 的
session-start timeline snapshot。
- 行为风险：缺少 session route、但从 custom key A 启动的 session 在切换 B 时可能被
  当成 `no key -> first key`，错误写入 B route 并覆盖真实 A multiplier。
- 证据：`ProviderSnapshot` 已有非明文 `Credential` 字段；custom provider 要求
  credential reference，而 `usage_session_routes` 自身不保存 credential presence。
💡 有界修复：完整规定 latest route -> session-start snapshot 的有序分类与
keyed/no-key/indeterminate 三态，并明确 session-start 时间读取、分类和条件写入的所有者。

### 🟡 建议改进——推荐

[`architecture.md:9`](../architecture.md) A3-F3：全局源码审计锚点仍指向旧 HEAD
`8beacdb…`。
- 行为风险：后续实现者无法判断行号和源码事实是否针对当前设计内容验证，历史与当前
  review state 会混在一起。
- 证据：本轮精确内容状态已是 HEAD `5609736…`，当前 reconcile 调用位置也与旧文档
  引用发生过漂移。
💡 有界修复：更新为当前 reviewed HEAD，或让 review record 持有 HEAD+blob，仅在正文
保留当前 symbol/line 位置。

### 🟢 优点

- file state 与 process state 的边界明确，settings mismatch 仍保留显式 unknown。
- no-route 设计正确复用 prior route/session-start fallback，没有引入 schema、quality 或
  历史重解释。
- 自动化验证与真实 Claude session 验收的证据边界清楚。

### 📝 总结

本轮绑定 HEAD `56097366fa7fa4c275750a03387346d98f51dc57` 与当前未提交
`architecture.md` blob `f5465dc5096cc907b6953083456a1530d9bc514b`。总体 no-route
方向成立，但 advisory copy 会错误描述已采纳 first-key/wrapper 状态，prior-state
classifier 又缺少 credential-aware 三态算法；加上旧源码锚点，当前合同仍要求实现者
自行发明关键语义，因此结论为 FAIL/REOPEN。

## Round 4 — 2026-08-25

- Reviewed state: repair of Round 3's three authorized findings. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `1af91b6b0c49af8b3c6b9baaf2285e301a06ee32`.
- Repairer: Codex (repair round — an independent Re-review is required before
  the `Review` cell may be ticked again).
- Scope: A3-F1, A3-F2, and A3-F3 only.
- Round 3 findings, dispositions:
  - **A3-F1** the no-credential advisory claimed a running process kept all
    startup authentication and configuration -> **Fixed.** The copy now states
    only the guaranteed boundary: removing a key does not re-authenticate an
    already-keyed session, adoption of other configuration changes is not
    established, and restart guarantees the complete new selection.
  - **A3-F2** the prior-state rule treated every session-start timeline entry as
    no-key -> **Fixed.** The contract now specifies an ordered
    `keyed`/`no-key`/`indeterminate` classifier: latest recognized route first,
    then `usage_sessions.first_at` plus `Store.ProviderSnapshotAt` only when no
    route exists. Credential presence and `Official` state decide the snapshot;
    missing, unknown, invalid, or contradictory evidence fails closed. The
    classification and conditional write are both owned by
    `usage.Service.RecordClaudeConfigChange` and its private helper.
  - **A3-F3** the living design bound every source premise to old HEAD
    `8beacdb1` -> **Fixed.** Exact HEAD-plus-blob identity remains in this review
    record, while the architecture keeps current symbol locations inline. The
    moved reconcile range now cites `cmd/agentdeck/main.go:2922-2952`.
- Evidence: current CodeGraph and focused source inspection confirmed
  `reconcileClaudeConfigChange` at `cmd/agentdeck/main.go:2922-2952`, the latest
  route query at `internal/usage/routes.go:82-87`, the existing
  `usage_sessions.first_at` lookup, and `Store.ProviderSnapshotAt` at
  `internal/store/providers.go:823-824`. A focused stale-text scan found none of
  the superseded advisory, classifier, old-HEAD, or reconcile-range wording. No
  product code, tests, requirements, task decomposition, or unrelated topic
  document changed in this repair.
- Completion gate: NOT_VERIFIED — Repair closes the named findings but does not
  grant an independent review verdict or create completion evidence.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 5 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `1af91b6b0c49af8b3c6b9baaf2285e301a06ee32`.
- Reviewer: Codex, independently re-reviewing the Round 4 repair; this workflow
  turn did not author that repair.
- Method: Finding-by-finding design/contract Re-review under the
  `development-workflow` REREVIEW dimensions. Focused current-content inspection
  checked all three repaired findings. A targeted CodeGraph pass then tested the
  repaired prior-auth classifier against every Claude credential source the
  current code can and cannot observe.
- Scope: A3-F1, A3-F2, A3-F3, regressions caused by their repair, and new
  blockers in the same prior-authentication decision boundary.
- Finding dispositions:
  - **A3-F1 — CLOSED.** The no-credential advisory now states only that removing
    a key does not re-authenticate an already-keyed session, other configuration
    adoption is not established, and restart guarantees the complete new
    selection. It no longer claims that all startup authentication or
    configuration remains in effect.
  - **A3-F2 — CLOSED as scoped.** The contract now specifies the requested
    route-first, timeline-second `keyed` / `no-key` / `indeterminate` classifier,
    checks `ProviderSnapshot.Credential`, fails closed on missing or contradictory
    evidence, and assigns classification plus the conditional write to
    `usage.Service.RecordClaudeConfigChange` and a private helper.
  - **A3-F3 — CLOSED.** The living architecture no longer binds source premises
    to historical HEAD `8beacdb1`; exact HEAD-plus-blob identity remains in the
    review record and current symbol locations remain inline.
- New blocking findings:
  - [P1] **A4-F1 — An `Official` timeline snapshot with an empty managed
    `Credential` does not prove the running session had no API key.** Claude may
    authenticate through `env.ANTHROPIC_API_KEY`, `apiKeyHelper`, or a credential
    exported in the process environment. The first two are AgentDeck-unowned
    conflict sources, and the last is explicitly invisible to file inspection.
    A completed `official` selection and its session route/timeline record only
    AgentDeck's managed choice, not the authentication the process actually
    adopted. The repaired classifier nevertheless promotes that state to
    `no-key`, so a session already authenticated through an unowned key can be
    misclassified as `no key -> first key` and receive a matched route it did not
    adopt -> open; either define a supported session-specific evidence source
    that excludes every effective credential source, or fail closed and suppress
    the matched route when actual prior authentication cannot be proved. If the
    latter changes the approved first-key promise, reopen and reconcile the
    requirements before this architecture can pass.
- Evidence: `ClaudeCredentialConflicts` sees only settings-file
  `env.ANTHROPIC_API_KEY` and `apiKeyHelper`, while its source comment states that
  a shell-exported credential is honored by Claude and invisible here
  (`internal/provider/config.go:170-190`). `ProviderSnapshot` carries only the
  managed selection's non-secret credential name
  (`internal/store/providers.go:86-94`). The current classifier at
  `architecture.md:67-76` checks neither unowned source nor a process-auth signal
  before declaring `Official + empty Credential` to be `no-key`. Broad
  verification stopped after this decisive contract blocker. L0 review-record
  checks: `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A4-F1 leaves the current architecture
  criterion open; no CEv1 evidence was written for this state.
- Verdict: REOPEN

## 📋 Architecture 独立复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:67`](../architecture.md#the-asymmetry-this-design-is-built-on)
A4-F1：`Official + empty managed Credential` 不能证明运行中的 Claude session 没有
API key。
- 处置：new/open。
- 行为风险：session 可能已通过 `env.ANTHROPIC_API_KEY`、`apiKeyHelper` 或 shell
  environment 中的 key 认证；分类器仍会把它当成 `no-key`，从而把后续 managed key
  错记为已采纳 first-key route，并覆盖真实 provider/multiplier。
- 证据：`ClaudeCredentialConflicts` 只检查 settings 文件中的两个 unowned source，源码
  明确说明 shell-exported credential 不可见；`ProviderSnapshot` 只描述 AgentDeck
  selection，不描述运行中 process authentication。
💡 有界修复：定义能排除所有有效 credential source 的 session-specific 证据；若没有
这种证据，则 prior authentication 必须保持 `indeterminate` 并 fail closed。若这会改变
requirements 的 first-key 承诺，先重开并同步 requirements。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A3-F1 已关闭：advisory 不再承诺保留全部 startup authentication/configuration。
- A3-F2 的原始缺口已关闭：route/timeline 顺序、credential-aware 三态与服务所有权已经
  明确。
- A3-F3 已关闭：正文与 review record 的源码状态职责分离清楚。

### 📝 总结

A3-F1、A3-F2、A3-F3 均在 HEAD `56097366fa7fa4c275750a03387346d98f51dc57` +
`architecture.md` blob `1af91b6b0c49af8b3c6b9baaf2285e301a06ee32` 中关闭；但
修复后的 classifier 把 managed `official` 状态误当成真实 no-key 证明，遗漏 Claude
仍可能采用的 unowned/unobservable credentials。A4-F1 直接破坏 first-key route 的
正确性，因此本轮结论为 FAIL/REOPEN。

## Round 6 — 2026-08-25

- Reviewed state: repair of Round 5's single authorized finding. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `c81d6aa93c3ebf8143a594478c2680d60c6832b6`.
- Repairer: Claude Code (repair round — an independent Re-review is required
  before the `Review` cell may be ticked again).
- Scope: A4-F1 only.
- Direction decision: the finding offered two bounded fixes. Branch (a), a
  session-specific evidence source excluding *every* effective credential
  source, was investigated and found unavailable: `ClaudeCredentialConflicts`
  reads only the one managed settings file
  (`internal/provider/config.go:164-191`), Claude transcript entries carry no
  authentication-source field (inspected live JSONL: `entrypoint`, `version`,
  `cwd`, `sessionId`, `userType`, and no `apiKeySource`), and reading
  `os.Getenv` was already rejected project-wide because AgentDeck's own process
  environment is not the client's
  (`docs/archive/plans/provider-wrapper-routing.md:678-693`). Branch (b), a full
  fail-closed suppression, would delete the approved first-key route promise
  (`requirements.md:92`, `:223`) and required reopening a passed requirements
  review. The user selected the third, in-scope reading: tighten the classifier
  with every observable unowned source and state the residual boundary
  explicitly, without changing `requirements.md`.
- Round 5 finding, disposition:
  - **A4-F1** the classifier promoted `Official` plus an empty managed
    `Credential` to `no-key`, so a session authenticated by a source AgentDeck
    does not own could be misclassified as `no key -> first key` -> **Fixed.**
    Steps 1 and 2 now yield only a *candidate* `no-key`, and a new step 3
    confirms it: the service calls `ClaudeCredentialConflicts`
    (`internal/provider/config.go:173-191`) on the path the reconcile already
    inspects, and any reported `env.ANTHROPIC_API_KEY` or `apiKeyHelper`, or any
    read or parse failure, downgrades the candidate to `indeterminate`. The
    contract states in the same section that steps 1 and 2 describe only
    AgentDeck's managed selection and therefore cannot, alone, distinguish an
    unauthenticated session from one authenticated elsewhere. `no-key` is
    redefined as a classification bounded by the project's recorded
    credential-source boundary rather than a proof about the process. A new
    boundary paragraph names what stays invisible — a shell-exported credential
    and any unmodeled Claude settings scope — attributes that boundary to the
    existing project-wide scope decisions rather than to this contract, and
    states that today's `official` routes are bounded identically. Contract 3's
    headline and the "does not suppress" bullet were realigned from `proves` to
    the classifier's confirmed `no-key`, and `## Verification` now requires
    tests for the unowned-source downgrade and the unreadable-settings case.
- Evidence: `ClaudeCredentialConflicts` returns the two settings-file conflict
  keys in a stable order and never a value
  (`internal/provider/config.go:164-191`); `reconcileClaudeConfigChange` already
  resolves and inspects `~/.claude/settings.json`
  (`cmd/agentdeck/main.go:2922-2952`), so step 3 adds no new path resolution or
  configuration surface. No product code, tests, `requirements.md`, `tasks.md`,
  or unrelated topic document changed in this repair. A focused stale-text scan
  found no remaining "proves `no-key`" wording. L0 review-record checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Residual accepted and stated, not hidden: a credential exported into the shell
  environment or parked in an unmodeled settings scope can still make a
  candidate `no-key` wrong. That residual is the project's existing documented
  credential-source boundary, under which today's `official` attribution is
  already bounded the same way; closing it would require widening AgentDeck's
  Claude settings-resolution model, which is outside this topic.
- Completion gate: NOT_VERIFIED — Repair closes the named finding but does not
  grant an independent review verdict or create completion evidence.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 7 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `c81d6aa93c3ebf8143a594478c2680d60c6832b6`.
- Reviewer: Codex, independently re-reviewing the Round 6 repair; this workflow
  turn did not author that repair.
- Method: Finding-focused design/contract Re-review under the
  `development-workflow` REREVIEW dimensions. The A4-F1 repair was checked
  against current configuration/conflict source, the archived scope decision,
  the living CLI detection-boundary contract, and temporal scenarios where a
  modeled unowned credential changes after session authentication.
- Scope: A4-F1, the explicit residual-acceptance decision, regressions caused by
  its repair, and new blockers in the same prior-authentication proof boundary.
- Finding dispositions:
  - **A4-F1 — CLOSED by the selected project-boundary decision.** The classifier
    no longer treats a managed `official` route or timeline snapshot alone as
    process-auth proof. It checks the two unowned sources the project models,
    fails closed on conflict/read/parse failure, and explicitly discloses that
    shell-exported credentials and unmodeled settings scopes remain outside the
    project-wide detection boundary. The archived provider-wrapper decision and
    living CLI manual confirm that boundary; the repair record attributes its
    acceptance to the user, so the same residual is not re-raised here.
- New blocking findings:
  - [P1] **A6-F1 — The current-file conflict scan cannot prove a modeled unowned
    credential was absent when the session authenticated.** A session can start
    under `env.ANTHROPIC_API_KEY` or `apiKeyHelper`, then have that field removed
    from the managed settings file without the already-running process
    re-authenticating. A later switch sees a recognized `official` route plus a
    current file with no conflict, so step 3 promotes the candidate to `no-key`
    even though the process remains keyed. Neither the route nor provider
    timeline stores the historical unowned-source presence. The classifier can
    therefore record a later managed key as a first-key route the process did not
    adopt -> open; require session-start/historical evidence that the modeled
    unowned sources were absent for this session, or keep the candidate
    `indeterminate` when that historical fact is unavailable. A current-file
    absence check alone is insufficient; if the required evidence needs new
    persistence or changes the approved no-schema/first-key boundary, reconcile
    requirements before this architecture can pass.
- Evidence: `ClaudeCredentialConflicts` reads only the current managed file
  (`internal/provider/config.go:173-190`); it has no historical input.
  `usage_session_routes` stores provider, multiplier, wrapper state, event,
  source, and quality but no unowned-credential presence
  (`internal/usage/routes.go:56-78`). Contract 2 states that a config match proves
  only current AgentDeck-owned file fields, while the approved state machine
  states that removing an existing key does not re-authenticate the process.
  Broad verification stopped after this decisive temporal blocker. L0
  review-record checks: `make check-whitespace` -> exit 0;
  `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A6-F1 leaves the current architecture
  criterion open; no CEv1 evidence was written for this state.
- Verdict: REOPEN

## 📋 Architecture 独立复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:78`](../architecture.md#the-asymmetry-this-design-is-built-on)
A6-F1：切换时读取“当前文件无 conflict”不能证明 session 认证时没有 modeled unowned
credential。
- 处置：new/open。
- 行为风险：session 可先通过 `env.ANTHROPIC_API_KEY` 或 `apiKeyHelper` 认证，再从文件
  删除该字段；运行中进程仍保持旧 key，但后续 scan 会把它分类为 `no-key`，错误记录
  first-key route。
- 证据：`ClaudeCredentialConflicts` 没有历史输入，session route/timeline 也不保存这些
  unowned source 的 session-start presence；删 key 不触发重新认证是本 topic 的既定前提。
💡 有界修复：增加能绑定该 session 启动/认证时刻的 modeled-source absence evidence；
没有该历史证据时保持 `indeterminate`。若需要新持久化或改变 no-schema/first-key 承诺，
先同步 requirements。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A4-F1 已按用户选择的既有 project-wide credential-source boundary 关闭。
- 当前修复正确检查可观察 conflict，并公开 shell/unmodeled residual，没有把它伪装成
  process-level proof。
- unreadable/invalid settings 已 fail closed，verification 也覆盖相应分支。

### 📝 总结

A4-F1 在 HEAD `56097366fa7fa4c275750a03387346d98f51dc57` +
`architecture.md` blob `c81d6aa93c3ebf8143a594478c2680d60c6832b6` 中按明确用户决策关闭。
但 step 3 只有当前文件视图，无法覆盖“modeled unowned key 在 session 认证后被移除”的
时间场景；A6-F1 仍可让 unadopted key 获得 first-key route，因此本轮结论为
FAIL/REOPEN。

## Round 8 — 2026-08-25

- Reviewed state: correction of Round 7 after the user's authoritative runtime
  and persistence clarification. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `c81d6aa93c3ebf8143a594478c2680d60c6832b6`.
- Reviewer: Codex
- Method: Re-evaluated the Round 7 scenario against the user's explicit
  invariant: every Hook-triggered fact is critical information that must be
  persisted, while a configuration observation and the route actually adopted
  by a running session are different state streams.
- Corrected finding disposition:
  - **A6-F1 — SUPERSEDED.** Round 7 treated a later hook as though the earlier
    key-mode determination legitimately left no durable fact. The user rejected
    that premise: key-mode detection itself must be stored. The runtime part of
    the user's correction is also decisive — without restart the session remains
    on key A; after restart, `SessionStart` adopts the then-current `official`
    selection. A6-F1 is therefore not retained as the authoritative defect.
- Replacement blocking finding:
  - [P1] **A8-F1 — The architecture has no persistence contract separating
    append-only Hook observations from the effective session route.** It says an
    unadopted matched change writes no route, but defines no other durable record
    for the Hook fact. Conversely, writing every observed selection directly to
    `usage_session_routes` is incorrect because `sessionRouteAt` treats the latest
    row as effective for pricing. The design therefore loses critical key-mode /
    selection history or corrupts effective attribution, depending on which
    interpretation an implementer chooses -> open; define two explicit streams:
    (1) every relevant Hook observation is appended with session, time, observed
    selection/key-mode/conflict evidence and no secret value; (2) effective-route
    state advances only when the session adopts a transition. Unadopted switches
    persist their observation but leave effective route A unchanged; a restart's
    `SessionStart` promotes the current selection (for example `official`) to the
    effective route. Specify storage ownership, schema/representation, ordering,
    idempotency, and resolver rules that ignore unadopted observations for cost.
    Reconcile the requirements statements that currently promise “recording no
    route”, no new stored representation, and no migration so that “no effective
    route row” is not misread as “discard the Hook event”.
- Evidence: `usage_session_routes` is currently an effective-route history and
  `sessionRouteAt` returns its latest row at or before the event
  (`internal/usage/routes.go:56-87`; `internal/usage/usage.go:2504-2510`). The
  current architecture suppresses the unadopted matched write but defines no
  append-only observation store. The approved requirements currently state that
  recording no route needs no new representation and no migration, which conflicts
  with the newly explicit persistence invariant. L0 review-record checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A8-F1 leaves the current architecture
  criterion open; no CEv1 evidence was written for this unchanged target state.
- Verdict: REOPEN

## 📋 Architecture 更正后评审结果

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:203`](../architecture.md#contract-3--only-an-effective-first-key-transition-records-a-matched-route)
A8-F1：缺少 Hook observation 与 effective route 的独立持久化合同。
- 行为风险：若“不写 route”等于不落库，会丢失关键 Hook/key-mode 历史；若把每次 observed
  selection 都写进 `usage_session_routes`，resolver 又会把未采纳的 `official` 或 key B
  当成 effective route，错误改变成本归因。
- 证据：当前 route resolver 把最近 route row 直接作为有效 provider/multiplier；设计只
  规定 unadopted change 不写 route，没有定义另一份 append-only Hook record。
💡 有界修复：持久化每个 Hook observation，但只有实际采纳的 transition 才推进
effective route；无重启时保持 A，重启的 `SessionStart` 才把当前 selection 提升为新的
effective route。明确 representation/schema、幂等、顺序、隐私字段和 resolver 过滤规则，
并同步 requirements 的 no-representation/no-migration 表述。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 用户明确了正确运行时不变量：没有重启时 session 始终保持 A。
- `SessionStart` 已是重启后推进 effective route 的自然边界。
- observation/effective 分离后，既能保留完整 Hook 事实，也不会让文件变化冒充进程状态。

### 📝 总结

Round 7 的 A6-F1 被更准确的 A8-F1 取代。当前真正 blocker 不是“当前 conflict scan
是否足够”，而是设计把“是否保存 Hook 事实”和“是否推进 effective route”合并成一个
route-write 决定。architecture、requirements 与 tasks 必须明确两类持久化语义后才能
进入 PASS。

## Round 9 — 2026-08-25

- Reviewed state: repair of Round 8's single authorized finding. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `0c91b38ccb82a4947b50ff4960d4486a6e8b4ffd`. Reconciled companions:
  `requirements.md` blob `046460d4d03f4ae0b710b2899a19e49389e6ff0e`,
  `tasks.md` blob `62065df2c32241c76814e409fc318f6f6c92542c`, and the
  requirements reopen record `reviews/requirements.md` blob
  `ac7811cadc3efa34648fa04e6432a6b818b62c5d`.
- Repairer: Claude Code (repair round — an independent Re-review is required
  before the `Review` cell may be ticked again).
- Scope: A8-F1 only, plus the requirements and decomposition reconciliation the
  finding explicitly requires.
- Round 8 finding, disposition:
  - **A8-F1** the architecture merged "persist the Hook fact" and "advance the
    effective route" into one route-write decision, so an implementer had to
    either lose the key-mode history or corrupt pricing -> **Fixed.** Contract 3
    gains a `### Two persistence streams` section that separates them and states
    why collapsing them is the false choice. Stream 1 is a new append-only
    `usage_session_observations` table with a specified column set — session,
    `observed_at`, hook event and source, `config_matched`, the observed
    selection, the `keyed`/`no-key`/`indeterminate` classification, a
    `clean`/`conflict`/`unavailable` conflict scan with its stable key names,
    `adopted`, and `semantic_key`. Privacy is stated positively and negatively:
    key names only, and no credential value, endpoint, settings path, prompt, or
    transcript content. Representation is one additive migration in the style of
    migration 17 (`internal/store/migrations.go:105-109`) with a
    `(client, session_id, observed_at)` index; idempotency is `semantic_key`
    dedup so a replayed hook appends nothing; ordering is `observed_at` then
    `id`, matching `sessionRouteAt`. Stream 2 keeps `usage_session_routes`
    unchanged in schema and meaning and advances only on `SessionStart` or an
    `adopted` `ConfigChange`, with the pre-existing mismatch `unknown` write
    untouched. The resolver rule is explicit: `sessionRouteAt` and
    `priceForEvent` are unchanged and must not join the observation table.
    Ownership is `usage.Service.RecordClaudeConfigChange`, both writes in one
    store transaction, observation first, so no route can exist without its
    justifying observation. The section also states what the stream is for and
    that this topic reads it nowhere.
  - Consequential wording realigned in the same document: the contract headline
    now says "records no route" is a statement about the effective-route stream
    only, and the quality paragraph now says there is no new *route* row and that
    the observation stream carries no quality column because nothing prices from
    it. `## Verification` gains the observation tests as a separate group,
    including the assertion that a session with observations but no new route
    still prices through its prior route.
- Requirements reconciliation (authorized by the finding): `requirements.md`
  no longer promises "no new stored representation" without qualification — the
  claim is now scoped to effective-route resolution, followed by an explicit
  "recording no route is not discarding the observation" paragraph naming the two
  streams and pointing at `architecture.md` for their rules. The consequence
  bullet's "no migration is required" becomes one additive migration that alters
  no existing table and that no pricing path reads. The acceptance boundary gains
  the observation requirement, including that an observation never changes what an
  event is priced at. The user-visible-surface statement now names the internal
  table while keeping "no new surface", because no command renders it.
- Decomposition reconciliation: `tasks.md` task 2 now owns the migration, the
  one-observation-per-hook rule with its privacy and dedup constraints, the
  single-transaction ownership, and the assertion that the resolver never reads
  stream 1; its file list gains `internal/store/migrations.go` and
  `internal/store/routes_test.go`. The `requirements.md` `Review` cell is
  unticked with a stated reason, because Round 6's PASS applies to the superseded
  blob, and the reopen is recorded as `reviews/requirements.md` Round 7 with
  verdict `REOPEN` and a `NOT_VERIFIED` gate so no stale PASS remains the file's
  last verdict.
- Evidence: `usage_session_routes` is created by migration 17
  (`internal/store/migrations.go:105-109`) and `sessionRouteAt` selects its
  latest row at or before the event (`internal/usage/routes.go:79-87`;
  `internal/usage/usage.go:2504-2510`), which is why the observation stream must
  be a separate table rather than extra rows or a flag on that one.
  `recordSessionRoute` already builds a `semantic_key` from its full tuple and
  suppresses an identical repeat (`internal/usage/routes.go:54-77`), so the
  observation dedup reuses an existing convention rather than inventing one.
  `ClaudeCredentialConflicts` returns key names only and never a value
  (`internal/provider/config.go:164-191`), which is the rule `conflict_sources`
  inherits. No product code, tests, or unrelated topic document changed in this
  repair. L0 review-record checks: `make check-whitespace` -> exit 0;
  `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — Repair closes the named finding but does not
  grant an independent review verdict or create completion evidence. The
  reopened `requirements.md` gate is also no longer satisfied by its Round 6
  evidence, which was bound to the superseded blob.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 10 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `0c91b38ccb82a4947b50ff4960d4486a6e8b4ffd`.
  The reconciled requirements prerequisite is independently PASS and VERIFIED at
  blob `046460d4d03f4ae0b710b2899a19e49389e6ff0e`.
- Reviewer: Codex, independently re-reviewing the Round 9 A8-F1 repair; this
  workflow turn did not author that repair.
- Method: Finding-focused design/contract Re-review under the
  `development-workflow` REREVIEW dimensions. Checked both persistence streams,
  schema meaning, privacy, resolver isolation, transaction behavior, ordering,
  and replay idempotency against current Hook and route source.
- Scope: A8-F1 and new blockers caused by its two-stream repair.
- Finding disposition:
  - **A8-F1 — STILL OPEN.** The repair correctly separates append-only
    observations from effective routes, specifies the observation columns and
    privacy exclusions, leaves route resolution isolated, and reconciles
    requirements/tasks. Two required parts of the finding remain contradictory
    or unsatisfied:
    1. **Replay idempotency is not implementable as specified.** `semantic_key`
       is the joined tuple of every column, including `observed_at`, while
       `observed_at` comes from the service processing clock. Replaying the same
       Hook later produces a different timestamp and therefore a different key,
       contradicting “a replayed hook appends nothing”. Removing time from that
       tuple without another stable event identity would instead collapse two
       legitimate identical configuration observations that occurred at
       different times. The current Hook envelope carries no stable event ID or
       event timestamp, so the architecture must choose and specify another
       durable identity/dedup rule rather than borrowing the route convention by
       assertion.
    2. **The transaction/crash contract is internally impossible.** The document
       requires observation and route writes “in one store transaction” but then
       says a crash between them may leave the observation without the route. An
       atomic transaction commits both or neither. If the product requirement is
       that the observation survives a route-write failure/crash, it requires a
       separately committed observation and an explicit recovery/idempotency
       protocol; if atomicity is intended, the stated partial durable state must
       be removed and failure means neither row exists.
- Evidence: the observation table contract includes `observed_at` in the
  `semantic_key` tuple (`architecture.md:291-314`). The Hook event currently
  carries session, name, source, transcript/config paths but no stable event ID
  or event timestamp (`internal/usagehook/event.go:11-16`). The ownership section
  simultaneously specifies one transaction and a crash-surviving first write
  (`architecture.md:334-340`). Broad verification stopped after these decisive
  contract blockers. L0 review-record checks: `make check-whitespace` -> exit 0;
  `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A8-F1 remains open for the current architecture
  state; no CEv1 evidence was written.
- Verdict: REOPEN

## 📋 Architecture A8-F1 独立复评

📊 总体评分：6/10

✅ 复评结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:309`](../architecture.md#two-persistence-streams) A8-F1：
`semantic_key` 包含处理时生成的 `observed_at`，无法去重 replay。
- 处置：still open。
- 行为风险：相同 Hook 重放会产生新时间和新 key，重复 observation；简单移除时间又会
  折叠两个合法但内容相同的独立 Hook。
- 证据：当前 Hook envelope 没有稳定 event ID/timestamp，architecture 也未定义替代身份。
💡 有界修复：定义稳定 event identity 或明确、不会吞掉合法重复事件的 dedup/replay
协议，并据此生成 `semantic_key`。

[`architecture.md:334`](../architecture.md#two-persistence-streams) A8-F1：同一 store
transaction 与“crash 后只留下 observation”不能同时成立。
- 处置：still open。
- 行为风险：实现者无法判断失败时应 atomic rollback，还是必须保证 observation 独立持久；
  recovery 与测试合同因此不确定。
- 证据：ACID 单事务只会两者都提交或都回滚。
💡 有界修复：选择并写明一种合同：atomic all-or-nothing；或 observation 先独立 commit，
再用可重放/恢复协议推进 effective route。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- observation/effective-route 表与读取路径已经正确分离。
- 隐私字段、additive migration、resolver isolation 和 requirements reconciliation 完整。
- 未采纳 observation 不再污染成本归因，restart `SessionStart` 仍是 route 推进边界。

### 📝 总结

两流模型已解决 A8-F1 的主要方向，但 replay identity 与事务失败语义仍要求实现者自行
选择互不兼容的行为。当前 architecture blob `0c91b38ccb82a4947b50ff4960d4486a6e8b4ffd`
尚未达到可实现合同，结论为 FAIL/REOPEN。

## Round 11 — 2026-08-25

- Reviewed state: repair of Round 10's two still-open parts of A8-F1. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `e6df4a513a691f2c1c350ed219e42192db0db365`; `tasks.md` blob
  `872f011c21bc1d26bc92929dea265a21e5bd4ae4`.
- Repairer: Claude Code (repair round — an independent Re-review is required
  before the `Review` cell may be ticked again).
- Scope: A8-F1's two remaining parts only. Round 10's confirmed parts — stream
  separation, column set, privacy exclusions, resolver isolation, additive
  migration, and the requirements/tasks reconciliation — were not reopened or
  rewritten.
- Round 10 finding, dispositions:
  - **A8-F1 part 1, replay identity — Fixed.** The contract no longer borrows the
    route convention by assertion. It states why it cannot: `recordSessionRoute`
    keys on the processing clock, and the hook envelope carries no event ID or
    event timestamp (`internal/usagehook/event.go:11-16`). Identity moves to the
    event being observed rather than the moment it was processed. A new
    `settings_changed_at` column records the managed settings file's modification
    time at reconcile, `observed_at` is removed from `semantic_key`, and
    `settings_changed_at` takes its place in an explicitly written tuple. The
    same hook delivered twice reads one mtime and appends once; two separate file
    writes carry different mtimes and both survive even when their conclusions
    match, which is the collapse Round 10 warned about. Two boundaries are stated
    rather than left open: an unreadable mtime puts `observed_at` back in the key
    and appends unconditionally, failing toward a duplicate rather than a missing
    row and already marked `unavailable` by `conflict_scan`; and two writes
    sharing one mtime tick with one conclusion collapse to one row, losing a
    duplicate rather than a fact. The migration paragraph gained the constraint
    that rule depends on: the additive migration also creates a unique index on
    `semantic_key`, because a no-op-on-conflict insert needs something real to
    conflict against. The divergence from `usage_session_routes` — whose
    `semantic_key` carries no constraint because `recordSessionRoute` guards a
    repeat with `NOT EXISTS` — is stated rather than left for an implementer to
    notice. `tasks.md` task 2 carries the same index.
  - **A8-F1 part 2, transaction semantics — Fixed.** The contradiction is
    removed by choosing atomicity outright. The section is retitled
    *Ownership and failure semantics of the write*: both rows commit or neither
    does, a failure returns an error and leaves the store unchanged, and there is
    no partially durable state and no recovery protocol. The alternative is
    stated and rejected with its reason — a separately committed observation
    would have to record an `adopted` value for a route that does not exist,
    which is a false statement about the effective stream, and repairing it would
    need a replay protocol for a row nothing reads. `adopted` therefore always
    describes a committed route.
- Verification reconciled in the same pass: the stale "a replayed identical hook
  appends nothing" assertion is replaced by four event-identity cases (same hook
  twice over an unchanged file; two writes with identical conclusions; an
  unreadable mtime; one shared mtime tick) plus the atomic-transaction case in
  the direction that can go wrong — a failing route write leaves no observation
  behind. `tasks.md` task 2 carries the same identity rule, the fail-toward-
  duplicate boundary, and the atomic contract, replacing its previous
  "deduplicate on `semantic_key`" and "observation first" bullets.
- Evidence: `Event` carries `ConfigPath`, `SessionID`, `TranscriptPath`, `Name`,
  and `Source` only (`internal/usagehook/event.go:11-16`), confirming no stable
  event identity exists in the envelope. `reconcileClaudeConfigChange` already
  resolves and stats-adjacent-reads that exact settings path
  (`cmd/agentdeck/main.go:2922-2952`), so `settings_changed_at` needs no new path
  resolution or configuration surface. `recordSessionRoute` builds its key from
  `s.now()` (`internal/usage/routes.go:54-56`), which is the borrowed convention
  the repair rejects. No product code, tests, `requirements.md`, or unrelated
  topic document changed in this repair. L0 review-record checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — Repair closes the named parts but does not
  grant an independent review verdict or create completion evidence.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 12 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `e6df4a513a691f2c1c350ed219e42192db0db365`.
  Requirements remain independently PASS and VERIFIED at blob
  `046460d4d03f4ae0b710b2899a19e49389e6ff0e`.
- Reviewer: Codex, independently re-reviewing the Round 11 A8-F1 repair; this
  workflow turn did not author that repair.
- Method: Finding-focused design/contract Re-review under the
  `development-workflow` REREVIEW dimensions. Rechecked replay identity against
  the Hook envelope and the user's explicit one-observation-per-Hook invariant,
  and checked the selected atomic transaction semantics for internal
  contradiction or missing recovery behavior.
- Scope: the two still-open parts of A8-F1 from Round 10 and regressions caused by
  their repair.
- Finding disposition:
  - **A8-F1 part 2, transaction semantics — CLOSED.** The architecture now chooses
    atomic all-or-nothing behavior unambiguously: observation and adopted route
    commit together or neither does, failure leaves the store unchanged, no
    partial durable state exists, and no recovery protocol is claimed. The
    rejected separate-commit alternative and `adopted` invariant are consistent.
  - **A8-F1 part 1, replay identity — STILL OPEN.** Moving identity from
    processing time to file mtime improves immediate replay handling, but file
    mtime identifies a file state, not a Hook occurrence. The contract begins by
    requiring exactly one row for every handled Hook, then explicitly collapses
    two independent writes that share an mtime tick and conclusion. Those are two
    distinct Hook facts under the user's authoritative invariant, even if their
    stored conclusions match. A delayed replay after a later file write also
    reads the later mtime/state rather than the event that originally triggered
    it, so the key is not stable for the observed event. Because the Hook envelope
    carries no event ID or event timestamp, the design cannot simultaneously
    suppress external replay and distinguish every legitimate identical event by
    inference from current file metadata -> open; choose a truthful delivery
    contract. Under the user's “every Hook fact is critical” decision, append one
    observation per delivery with an internal observation/delivery ID and accept
    possible replay duplicates (optionally mark probable duplicates without
    suppressing them). Idempotency may cover retries of the same in-process store
    operation only when that operation carries a stable ID. Alternatively, a
    stable source event identity must be added to transport before external replay
    suppression can be promised.
- Evidence: the contract requires exactly one row per handled hook
  (`architecture.md:285-287`) but allows two independent writes with the same mtime
  and conclusion to collapse (`:357-359`). It also acknowledges the Hook envelope
  has no event ID/timestamp (`:324-330`; `internal/usagehook/event.go:11-16`). The
  atomic write contract at `architecture.md:377-390` is now internally complete.
  Broad verification stopped after the decisive identity blocker. L0
  review-record checks: `make check-whitespace` -> exit 0;
  `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A8-F1 replay identity remains open; no CEv1
  evidence was written for the current architecture state.
- Verdict: REOPEN

## 📋 Architecture A8-F1 独立复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:324`](../architecture.md#two-persistence-streams) A8-F1：mtime
仍不是稳定 Hook event identity。
- 处置：still open。
- 行为风险：两个独立 Hook 可因共享 mtime tick 和相同结论被合并，违反“每个 Hook 事实
  都必须存储”；延迟 replay 又可能读取后来文件状态，无法保持原 event identity。
- 证据：Hook payload 没有 event ID/timestamp；文档同时要求每个 handled Hook 一行并允许
  两个独立写入折叠。
💡 有界修复：按用户不变量采用 one-observation-per-delivery，使用内部 observation/delivery
ID 并接受可能的 replay duplicate；只能标记疑似重复，不能丢弃。若必须去除外部 replay，
则 transport 必须先提供稳定 source event identity。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A8-F1 transaction/crash 部分已关闭：atomic commit/rollback 与 failure contract 唯一明确。
- 两个持久化流、隐私、resolver isolation、migration 和 requirements reconciliation 保持正确。
- mtime 不可用时 fail toward duplicate 的方向符合“不能丢 Hook fact”的原则。

### 📝 总结

Round 11 已关闭 transaction 语义，但 mtime 方案仍试图从没有 event identity 的 transport
推导 exact replay identity，并因此主动折叠独立 Hook。A8-F1 尚未完全关闭，当前
architecture blob `e6df4a513a691f2c1c350ed219e42192db0db365` 结论仍为
FAIL/REOPEN。

## Round 13 — 2026-08-25

- Reviewed state: repair of the replay-identity portion of A8-F1 left open by
  Round 12. HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `c9d36d402dba1dc82338d063319a4554df189f75`.
  Reconciled downstream `tasks.md` blob
  `cfa22cc0f59a7a5bf4ff0c711eea937aa016e530`.
- Repairer: Codex (repair round — an independent Re-review is required before
  the `Review` cell may be ticked again).
- Scope: A8-F1 replay identity only. Round 12's closed atomic-transaction
  disposition remains unchanged.
- Finding disposition:
  - **A8-F1 replay identity** used settings-file mtime as though it identified a
    Hook occurrence, collapsing independent same-tick deliveries and misbinding
    delayed replay to later file state -> **Fixed.** The contract now persists
    one observation per handled delivery, generates one opaque `delivery_id`
    when the handler accepts that delivery, and reuses it only across an internal
    retry of the same store operation. A later transport delivery, including a
    possible external replay, receives a new ID and appends another row.
    `settings_changed_at` remains diagnostic only and never suppresses insertion.
    External replay suppression is explicitly unavailable until transport
    supplies a stable source event ID or timestamp.
- Consequential synchronization: Task 2 now creates a unique `delivery_id`
  index instead of a `semantic_key` index, carries the one-row-per-delivery and
  accepted-replay-duplicate contract, and verifies the unique index during both
  fresh and upgrade migration paths. Requirements already require one persisted
  observation per handled Hook and needed no change.
- Evidence: Round 12's current-source evidence remains applicable: the Hook
  envelope exposes no event ID or timestamp
  (`internal/usagehook/event.go:11-16`). Focused inspection confirmed that
  architecture and Task 2 no longer derive identity from mtime, promise external
  replay suppression, or collapse same-tick deliveries. No product code, tests,
  requirements, or unrelated topic document changed in this repair.
- Completion gate: NOT_VERIFIED — Repair closes the named finding portion but
  does not grant an independent review verdict or create completion evidence.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 14 — 2026-08-25

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `c9d36d402dba1dc82338d063319a4554df189f75`.
  Requirements are independently PASS and VERIFIED at blob
  `046460d4d03f4ae0b710b2899a19e49389e6ff0e`; downstream consistency was checked
  against `tasks.md` blob `cfa22cc0f59a7a5bf4ff0c711eea937aa016e530`.
- Reviewer: Codex, independently re-reviewing the Round 13 repair; this workflow
  turn did not author that repair.
- Method: Complete finding-disposition Re-review under the
  `development-workflow` REREVIEW dimensions. Rechecked every open and previously
  closed architecture finding against the current content, with focused current
  source reuse for Hook transport, provider/config state, route resolution, and
  migration conventions.
- Scope: A8-F1's final replay-identity repair, all prior architecture finding
  dispositions for regression, and new blocking defects introduced by the
  repair.
- Finding dispositions:
  - **A8-F1 replay identity — CLOSED.** The contract now uses one opaque
    `delivery_id` generated once per accepted delivery and reused only by an
    internal retry of that same store operation. The unique constraint makes the
    internal retry idempotent. A later transport delivery, including an external
    replay with no source identity, receives a new ID and appends a row. Mtime is
    diagnostic only and cannot suppress insertion, so independent same-tick /
    same-conclusion deliveries survive.
  - **A8-F1 transaction semantics — CLOSED.** Observation and any adopted route
    commit atomically or neither does; failure leaves no partial durable state and
    requires no recovery protocol.
  - **A8-F1 stream separation — CLOSED.** Every accepted delivery produces an
    append-only privacy-bounded observation, while only `SessionStart` or an
    adopted first-key `ConfigChange` advances effective routes. Pricing reads
    routes only.
  - **Earlier architecture findings — remain CLOSED.** Advisory copy, ordered
    prior-state classification, credential-source boundary, source-state
    anchoring, no-route semantics, and retained-route quality have not regressed.
- New blocking findings: none.
- Evidence: `architecture.md:285-344` defines one observation per accepted
  delivery, `delivery_id` uniqueness, internal-retry-only reuse, external replay
  acceptance, and diagnostic-only mtime. `:346-385` preserves effective-route
  isolation and atomic failure semantics. `:449-473` specifies independent
  delivery, retry, privacy, resolver, classifier, and transaction tests. Current
  Hook source still exposes no stable source event identity, matching the chosen
  at-least-once delivery contract. L0 final-state checks:
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0. CEv1 evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:architecture.md:rereview-round-14:c9d36d402dba1dc82338d063319a4554df189f75`
  satisfies the architecture criterion for this exact state.
- Completion gate: VERIFIED
- Verdict: PASS

## 📋 Architecture 独立复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 每个 accepted delivery 都保留 observation，外部 replay 不再被错误 suppression。
- 内部 retry 通过稳定 `delivery_id` 与 unique constraint 幂等。
- Observation/effective-route、atomic failure、隐私和 pricing resolver 边界完整且一致。
- Requirements 当前 persistence boundary 已独立 PASS/VERIFIED。

### 📝 总结

A8-F1 及全部历史 architecture finding 均在 HEAD
`56097366fa7fa4c275750a03387346d98f51dc57` + architecture blob
`c9d36d402dba1dc82338d063319a4554df189f75` 中关闭，无新 blocker。当前合同可直接
指导 migration、transaction、observation、route 和 verification 实现，复评结论为 PASS。

### Task checkpoint

- Task：`switch-effectiveness-boundary / architecture.md` @ HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57` + blob
  `c9d36d402dba1dc82338d063319a4554df189f75`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅纳入当前 `architecture.md`、architecture 评审记录、`tasks.md` 的
  architecture Review/current-state hunk，以及 `docs/status.md` 的对应 topic 状态 hunk；
  排除仍待独立提交的 requirements Task、尚未评审的 tasks 行为内容、其他 topic、roadmap
  与无关 dirty work。
- 推送建议：目标分支与远端尚未解析；仅在获得明确 commit 与 push 授权、形成并核验
  上述 Task 边界的签名提交后推送。本 checkpoint 不执行也不授权交付。

## Round 15 — 2026-08-25

- Reviewed state: user-authorized client-neutral Hook-operation revision. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `ced65fe13f6f9d0b07f7b3a9f572943d374ed8d3`.
- Author: Codex — this is a design reopen record, not an independent review.
- Change: new Contract 0 defines one normalized `HookDelivery` pipeline for every
  accepted client event. Observation storage now covers Codex and Claude;
  `route_effect` replaces the Claude-specific adopted flag; shared
  `RecordHookDelivery` owns one transaction and optional route write. Client
  differences remain only in raw adapters and normalized runtime facts.
- Consequential state: requirements blob
  `3605f402d413811290e5f56dee4361c035321823` and tasks blob
  `646de4edc9b541dc7fff5c5b8dc540321d382efc` carry the same boundary.
- Completion gate: NOT_VERIFIED — Round 14 evidence is bound to the superseded
  blob and does not apply to this cross-client architecture state.
- Verdict: REOPEN — awaiting independent review of the current blob.

## Round 16 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `ced65fe13f6f9d0b07f7b3a9f572943d374ed8d3`.
  Requirements were reviewed at blob
  `3605f402d413811290e5f56dee4361c035321823`; downstream consistency was checked
  against final synchronized tasks blob
  `3cf880b8df1ab0cbec6500c164400c368a312974`.
- Reviewer: Codex, independently reviewing the client-neutral Hook-operation
  revision; this workflow turn did not author the reviewed architecture content.
- Method: Formal design/contract Review under `development-workflow`. CodeGraph
  traced parser/handler, route, provider-snapshot, conflict, resolver, and test
  paths; focused source inspection checked the current validators, independent
  settings reads, route idempotence, and migration convention. Broad verification
  stopped after decisive blockers; required topic/L0 checks were retained.
- Scope: Contract 0, Contract 3, both persistence streams, admission, failure,
  retry, concurrency, schema, and effective-route compatibility.
- Findings:
  - **[P1] A16-F1 — the adapter admission boundary bypasses existing semantic
    validation.** Contract 0 treats an accepted Hook as one admitted by its wire
    adapter (`architecture.md:16-20`, `:328-332`), but `ParseEvent` only validates
    the bounded JSON/event/source shape. The current handler subsequently rejects
    invalid or out-of-root transcripts and unmanaged Claude config changes
    (`cmd/agentdeck/main.go:2904-2913`, `:2955-2976`). Without an ordered admission
    contract, “normalize after ParseEvent” can persist rejected events and weaken
    the existing path boundary. Repair by naming all admission checks before
    `HookDelivery` creation, defining each supported source's accepted/rejected
    effect, and preserving fail-open rejection without observation or route.
  - **[P1] A16-F2 — first-key classification has a settings-file TOCTOU gap.**
    Steps 3-4 (`architecture.md:118-128`) combine file match and conflict scan, but
    current `ClaudeConfigMatchesSnapshot` and `ClaudeCredentialConflicts` read and
    parse the file independently (`internal/provider/config.go:145-191`). A change
    between those reads can pair a match from one file state with a clean conflict
    result from another and falsely promote `no-key`. Repair by defining one stable
    parsed settings snapshot (or an explicit stability check that downgrades drift
    to `indeterminate`) and by placing the prior-route read, classification, and
    optional route write under a concurrency rule that cannot commit from stale
    prior-route evidence.
  - **[P1] A16-F3 — the persistence and retry contract is not implementation
    complete.** The Stream 1 column table (`architecture.md:335-349`) omits the
    `id` later required by ordering and by the acceptance SQL, and leaves the
    encodings/constraints for nullable booleans, `prior_state`, `conflict_scan`,
    `conflict_sources`, and `route_effect` undefined. More importantly, uniqueness
    on observation `delivery_id` alone does not make the observation-plus-route
    operation idempotent: a retry that finds the observation already committed
    must also skip the route side. Repair with the concrete DDL/encoding contract
    and explicit conflict behavior for the whole operation, then require a retry
    assertion for both streams.
  - **[P1] A16-F4 — the exact-once promise contradicts the chosen failure
    semantics.** `architecture.md:328-329` and `:493-508` require one observation
    per accepted delivery, while `:405-417` deliberately commits neither row on
    failure and supplies no recovery. Repair by distinguishing successful commit
    cardinality from attempted delivery and carrying the same wording into
    requirements and verification.
  - **[P2] A16-F5 — `route_effect=advance` does not define compatibility with the
    existing route no-op rule.** Contract 0 and Stream 2 say each non-compact
    `SessionStart` appends/advances a route (`architecture.md:42`, `:389-395`),
    while current `recordSessionRoute` suppresses a consecutive identical
    provider/multiplier/wrapper/event/source row
    (`internal/usage/routes.go:56-78`). Repair by explicitly preserving that
    idempotent no-op or intentionally replacing it, and align the real-session
    assertion with the chosen meaning of `advance`.
- Evidence: `codegraph explore` over the current Hook and route call paths;
  focused reads of `internal/usagehook/event.go`, `cmd/agentdeck/main.go:2884-2976`,
  `internal/provider/config.go:145-191`, `internal/usage/routes.go:14-87`,
  `internal/usage/usage.go:2504-2634`, and migration 17/18 conventions in
  `internal/store/migrations.go`; final-state checks:
  `bash scripts/check-topic-docs.sh` -> exit 0, `make check-whitespace` -> exit 0,
  and `git diff --check` -> exit 0.
- Completion gate: NOT_VERIFIED — A16-F1 through A16-F5 leave the architecture
  WorkUnit unsatisfied; no CEv1 completion evidence was recorded for this blob.
- Verdict: REOPEN

## 📋 Architecture 独立评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:16`] A16-F1：accepted/admission 边界没有纳入现有 post-parse
transcript 与 managed-config 校验。
- 行为风险：统一管线可能持久化当前安全边界明确丢弃的事件。
- 证据：`ParseEvent` 返回后，`runUsageHookEvent` 仍在
  `cmd/agentdeck/main.go:2904-2913` 执行两类语义校验。
💡 有界修复：把全部 admission 校验放在 `HookDelivery` 创建之前，并逐 source 定义
拒绝后的 observation/route 结果。

[`architecture.md:118`] A16-F2：file match 与 conflict scan 来自两次独立文件读取，
没有稳定快照或并发降级规则。
- 行为风险：竞态可把不同文件状态拼成虚假的 `no-key`，从而写入错误 effective route。
- 证据：`ClaudeConfigMatchesSnapshot` 与 `ClaudeCredentialConflicts` 在
  `internal/provider/config.go:145-191` 独立读取 settings 文件。
💡 有界修复：用同一解析快照完成 match/conflict，或检测变化并降级
`indeterminate`；同时规定 prior-route 查询与写入的事务/串行化边界。

[`architecture.md:335`] A16-F3：新表 DDL 与整项 retry 幂等合同不完整。
- 行为风险：实现者必须自行发明 `id`、状态编码、NULL/constraint 规则；相同
  `delivery_id` 的重试仍可能再次执行 route 写入。
- 证据：列清单没有 `id`，但文档与 `tasks.md` 查询都按 `id` 排序；唯一约束只绑定
  observation。
💡 有界修复：给出完整 DDL/编码与整个 transaction 的 conflict/no-op 规则，并同时
断言 observation 与 route 的 retry 结果。

[`architecture.md:328`] A16-F4：每个 accepted delivery 恰好一条 observation 与
失败时事务零写入、无恢复不能同时成立。
- 行为风险：验收合同不可满足，失败实现无统一判定。
- 证据：`:328-329` 与 `:493-508` 对 exact-once 的要求和 `:405-417` 的失败合同直接
矛盾。
💡 有界修复：区分 delivery attempt 与 successful commit cardinality，并同步三份文档。

### 🟡 建议改进——推荐

[`architecture.md:42`] A16-F5：`advance` 是“尝试写入并允许连续相同 route no-op”
还是“每次追加一行”没有决定。
- 证据：当前 `recordSessionRoute` 在 `internal/usage/routes.go:62-78` 抑制连续相同行，
而新合同/验收写成每次 append/advance。
💡 有界改进：明确保留或替换现有 route idempotence，并让测试和实机会话步骤采用同一
语义。

### 🟢 优点

- client-neutral adapter/policy/store 分层方向清楚，observation 与 pricing route 的所有权
  也分开了。
- 隐私 allowlist、外部 replay 不错误去重、resolver 不读取 observation 的原则明确。

### 📝 总结

当前 architecture blob `ced65fe13f6f9d0b07f7b3a9f572943d374ed8d3`
已经给出统一操作的正确大方向，但 admission、并发快照、DDL/整项 retry 幂等、失败
cardinality 与 route `advance` 语义仍需要实现者自行决定，因此不能直接开发。

## Round 17 — 2026-08-26

- Reviewed state: repair of Round 16's five findings. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `b620adf14e53711334cc6dd038424a2947b04109`. Companion repairs land in
  `requirements.md` blob `64cbe359ff36fd249b96593c85fb70cf542854f6` and
  `tasks.md` blob `a6d6a6f46954f8b7fc72b00f4c76401f203789d6`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a verdict).
- Scope: A16-F1 through A16-F5, as named in the repair command.
- Repair of A16-F1 — Contract 0 gains an *Admission* section that makes
  acceptance an ordered, total sequence completed **before** `HookDelivery` is
  constructed: (1) bounded read, (2) `usagehook.ParseEvent`, (3) store/home
  prerequisites, (4) source scope — `managedClaudeConfigChange`, which admits
  only `user_settings` on the managed `~/.claude/settings.json` and excludes
  `project_settings`, `local_settings`, `policy_settings`, and `skills` — and
  (5) transcript scope via `validHookTranscript`. A per-source table gives every
  currently accepted client/event pair its required checks and both its rejected
  and accepted effects; rejection is stated as fail-open, silent, and
  zero-write in both streams. The section names
  `cmd/agentdeck/hook_boundary_test.go` as the existing pin and forbids
  reordering the sequence, so the pipeline cannot persist an event the current
  boundary discards. `SessionEnd` is explicitly held at checks 1-3, matching the
  current handler rather than inventing a new path requirement for an event that
  writes no route.
- Repair of A16-F2 — Contract 3 now requires **one parsed settings snapshot per
  reconcile attempt**: a read-once provider entry point returns one in-memory
  parsed document plus its mtime, and both the match evaluation and the conflict
  scan are computed from that single document, so the two independent reads at
  `internal/provider/config.go:145-191` can no longer pair a match from one file
  state with a clean scan from another. The exported
  `ClaudeConfigMatchesSnapshot` and `ClaudeCredentialConflicts` keep their
  signatures for the advisory path; only the reconcile path moves to the
  snapshot form. A read or parse failure yields `indeterminate` and writes no
  matched route, and each of the three existing reconcile attempts takes a fresh
  snapshot. A *Serialization of the decision* paragraph places the prior-route
  read, the classification, and the optional route write on the same `*sql.Tx`
  as the observation insert, forbidding a commit from prior-route evidence read
  outside that transaction; it states this as a placement rule over the existing
  single-writer SQLite store, not a new concurrency primitive.
- Repair of A16-F3 — Stream 1 replaces the prose column list with the literal
  migration-19 `CREATE TABLE`/`CREATE INDEX` statements, including the
  previously missing `id INTEGER PRIMARY KEY` that the ordering rule and the
  acceptance SQL already assumed. An encoding table fixes every previously
  undefined value: nullable booleans as `1`/`0` with NULL meaning "not
  applicable", `prior_state` in {`keyed`, `no-key`, `indeterminate`},
  `conflict_scan` in {`clean`, `conflicted`, `unreadable`}, `conflict_sources`
  as `NOT NULL DEFAULT ''` comma-joined key names in
  `ClaudeCredentialConflicts` order, `route_effect` in {`advance`, `retain`,
  `unknown`, `none`}, and `settings_changed_at` as `NOT NULL DEFAULT ''`. A
  *Whole-operation idempotence* section replaces the delivery-ID unique index as
  the sole guard: the observation insert is conditional on the delivery ID not
  already existing, a zero-row insert makes the **entire** operation a no-op
  that skips the route write and commits, and only a one-row insert proceeds to
  the route write — so a retry can no longer leave one observation and two
  routes. The unique index is retained as the storage-level backstop.
- Repair of A16-F4 — a *Cardinality is a property of successful commits, not of
  attempts* paragraph now sits directly under the failure contract and states
  the invariant `0 <= observations(delivery_id) <= 1`: commit leaves one
  observation plus zero or one route, failure leaves zero rows in both streams
  and drops the delivery fail-open with no pending marker, cross-process retry,
  or recovery protocol, and two successful commits for one delivery ID are
  impossible by the whole-operation guard. The verification section and both
  companion documents carry the same wording.
- Repair of A16-F5 (P2) — a *What `advance` means* paragraph decides the
  question in favor of **preserving** the existing behavior:
  `recordSessionRoute`'s consecutive-identical suppression
  (`internal/usage/routes.go:56-78`) is kept deliberately, and `advance` is
  defined as an effective-route *state* guarantee rather than a row count. A
  consecutive identical advance appends no route row while still recording
  `route_effect=advance` in the observation stream. Stream 2, the "An `advance`
  observation therefore always describes a committed route" sentence, and the
  real-session assertions in `tasks.md` were aligned to that meaning.
- Consequential verification changes: the observation tests now assert the
  admission boundary from both sides (out-of-root and session-mismatched
  transcripts, and non-`user_settings`/unmanaged `ConfigChange` paths, write
  neither stream), the repeated-identical-start row-count rule, and the
  both-stream retry outcome; the migration cases now name schema version 19,
  the exact column set and encodings, the unique `delivery_id` index, and a
  populated v18 database whose existing rows must be untouched.
- Verification: `bash scripts/check-topic-docs.sh` -> exit 0. No product code
  changed in this round; all five repairs are contract text.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the architecture WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 20 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `98bbbeaa662247b87cc774cd42e70da4d28ec6cd`.
  Requirements remained at the independently passed blob
  `64cbe359ff36fd249b96593c85fb70cf542854f6`; downstream consistency was checked
  against final synchronized tasks blob
  `31e267b9a885b7b07fd2743a06404f70008cda33`.
- Reviewer: Codex, independently re-reviewing the Round 19 repair; this workflow
  turn did not author the reviewed architecture content.
- Method: Formal REREVIEW under `development-workflow`; focused comparison of
  A18-F1 against the repaired top-level pipeline, Contract 3 serialization rule,
  and whole-operation numbered sequence. Unchanged source evidence from Round 18
  was reused because the repair changed only contract ordering.
- Scope: A18-F1 and any new blocking regression introduced by its repair.
- Findings:
  - **A18-F1 — CLOSED.** `route_effect` is now computed before the observation
    carrying it is inserted. The transaction has one consistent executable
    order: duplicate guard, classification on the transaction, conditional
    observation insert, optional route write, and commit.
  - No new architecture finding.
- Evidence: the top-level pipeline at `architecture.md:23-40`, serialization
  rule at `:220-227`, and detailed sequence at `:496-540` agree. A duplicate hit
  or zero-row conditional insert skips the route side; a one-row insert applies
  the already computed effect; any error rolls back both streams.
- Completion gate: VERIFIED — CEv1 gate
  `switch-effectiveness-boundary:architecture.md` verified the exact
  `HEAD + architecture.md` state with subject digest
  `8469fd0746285a88eacdbde2620dfa1cc8a1a0dc5be57232a0804fb68ab9dba5`.
- Verdict: PASS

## 📋 Architecture 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A18-F1 已关闭：classifier output 在 observation insert 前产生，`NOT NULL
  route_effect` 不再依赖未来步骤。
- duplicate guard、conditional-insert backstop、双流原子性、retry no-op 和既有 route
  no-op 均保持完整。

### 📝 总结

当前 architecture blob `98bbbeaa662247b87cc774cd42e70da4d28ec6cd`
关闭了 A18-F1，未发现新的 architecture blocker；四个合同已可直接实现。

## Round 18 — 2026-08-26

- Reviewed state: HEAD `56097366fa7fa4c275750a03387346d98f51dc57`;
  `architecture.md` blob `b620adf14e53711334cc6dd038424a2947b04109`.
  Requirements were checked at blob
  `64cbe359ff36fd249b96593c85fb70cf542854f6`; downstream consistency was checked
  against final synchronized tasks blob
  `a81f3b8f8e9bf9bd594e29a361dcfe526b51f973`.
- Reviewer: Codex, independently re-reviewing the Round 17 repair; this workflow
  turn did not author the reviewed architecture content.
- Method: Formal REREVIEW under `development-workflow`; finding-by-finding
  comparison against Round 16, focused CodeGraph inspection of the current Hook,
  provider-config, and route paths, followed by direct transaction-contract
  consistency analysis. Broad verification stopped at the decisive reproducer.
- Scope: A16-F1 through A16-F5, their consequential transaction text, and any
  newly blocking regression.
- Findings:
  - **A16-F1 — CLOSED.** Admission is ordered before `HookDelivery` construction
    and preserves the current managed-config and transcript-scope rejection
    boundary with zero writes.
  - **A16-F2 — CLOSED.** Reconciliation uses one parsed settings snapshot per
    attempt, and prior-route classification plus the optional write are placed
    on the shared transaction.
  - **A16-F3 — CLOSED as to the recorded defect.** Migration 19 now has literal
    DDL, fixed field encodings, and a whole-operation duplicate-delivery guard.
  - **A16-F4 — CLOSED.** Cardinality is explicitly a successful-commit property;
    failed attempts leave both streams unchanged with no recovery protocol.
  - **A16-F5 — CLOSED.** `advance` preserves the existing consecutive-identical
    route no-op and guarantees resolved state rather than row growth.
  - **[P1] A18-F1 — the whole-operation sequence cannot persist its required
    observation.** `usage_session_observations.route_effect` is `NOT NULL` and
    the observation records the classifier result, but
    `architecture.md:488-500` requires the operation to insert the observation
    first and evaluate `route_effect` only after that insert affects one row.
    Contract 0 also orders observation append before classification, while the
    Claude decision must read and classify prior-route evidence inside this same
    transaction (`architecture.md:213-216`). The row therefore needs a value the
    specified sequence has not computed yet.
- Evidence: the literal DDL at `architecture.md:428-450` requires
  `route_effect TEXT NOT NULL`; the operation order at `:488-500` performs the
  conditional observation insert before route-effect evaluation; the top-level
  pipeline at `:27-31` repeats the same order. This contradiction is internal to
  the reviewed contract and is a decisive reproducer.
- Completion gate: NOT_VERIFIED — A18-F1 leaves the architecture criterion
  unsatisfied; no CEv1 completion evidence was recorded for this blob.
- Verdict: REOPEN

## 📋 Architecture 独立复评

📊 总体评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:488`] A18-F1：observation 行必须携带非空 `route_effect`，但合同
规定先插入 observation，插入成功后才计算 `route_effect`。
- 处置：新增阻断 finding。
- 行为风险：实现者只能写入未定义值、违反 `NOT NULL`，或自行改写事务顺序；三者都不
满足当前合同。
- 证据：DDL 在 `:444` 声明 `route_effect TEXT NOT NULL`；whole-operation 步骤 1
先插 observation，步骤 3 才 evaluate `route_effect`；`:213-216` 又要求 Claude
prior-route classifier 在同一事务内运行。
💡 有界修复：明确同一事务内的 duplicate-delivery guard、classification、observation
insert 与 optional route write 的可执行顺序，并同步顶层 pipeline；保留零行 retry
整项 no-op 和双流原子性。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A16-F1–A16-F5 均已按原 finding 关闭；admission、稳定 settings snapshot、DDL、失败
cardinality 与 `advance` no-op 的各自合同现在清楚。
- observation 与 pricing route 的隔离、隐私 allowlist 和现有 resolver 边界保持明确。

### 📝 总结

当前 architecture blob `b620adf14e53711334cc6dd038424a2947b04109`
关闭了五个既有 finding，但新增 A18-F1 是决定性的事务顺序矛盾；在 observation 能够
携带已计算 `route_effect` 之前，合同仍不可直接实现。

## Round 19 — 2026-08-26

- Reviewed state: repair of Round 18's blocking finding. HEAD
  `56097366fa7fa4c275750a03387346d98f51dc57`; `architecture.md` blob
  `98bbbeaa662247b87cc774cd42e70da4d28ec6cd`. The companion `tasks.md` repair is
  blob `e09f3b990c17e1b0073a12ea0da3451bbec67593`; `requirements.md` is unchanged
  at blob `64cbe359ff36fd249b96593c85fb70cf542854f6`, because the ordering defect
  was internal to the architecture contract and never reached the acceptance
  boundary.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a verdict).
- Scope: A18-F1 only, as named in the repair command. A16-F1 through A16-F5 stay
  CLOSED per Round 18 and were not reopened by this repair.
- Repair of A18-F1 — the contract no longer asks for a row before the value that
  row must carry. Classification now precedes the observation insert, and the
  duplicate-delivery guard is stated as its own step instead of being folded into
  the insert:
  - *Whole-operation idempotence* is rewritten as an executable, numbered order
    inside `RecordHookDelivery`: (0) take the per-attempt settings snapshot
    before `BEGIN`, since it is a filesystem read whose stability the snapshot
    rule owns; (1) `BEGIN`; (2) duplicate-delivery guard on `delivery_id` —
    an existing row makes the whole operation a no-op that classifies nothing,
    inserts nothing, skips the route write, commits, and returns success;
    (3) classify on the same transaction, computing `route_effect` together with
    `config_matched`, `prior_state`, `conflict_scan`, and `conflict_sources`;
    (4) insert the observation **carrying that computed `route_effect`**, still
    conditionally on `delivery_id` not existing so the guard keeps a
    storage-level backstop rather than a check-then-act race; (5) a zero-row
    insert takes the same no-op outcome as step 2, discarding the classification;
    (6) only a one-row insert applies `route_effect` to the zero-or-one route
    write; (7) commit both rows, the observation alone, or — on any error —
    neither.
  - The top-level Contract 0 pipeline at `:22-31` was reordered to match, and now
    shows the `BEGIN`/`COMMIT` boundary with the guard, classification,
    observation append, and route append inside it, followed by a sentence
    stating *why* classification comes first: `route_effect` is `NOT NULL` and
    holds the classifier's result.
  - Contract 3's *Serialization of the decision* paragraph now names the same
    executable order explicitly — guard, classification, observation insert,
    optional route write — so the two places that describe this transaction can
    no longer disagree.
  - Preserved unchanged by the repair: the zero-row whole-operation retry no-op,
    two-stream atomicity, the successful-commit cardinality invariant, the
    fail-open zero-write failure outcome, the unique `delivery_id` index as
    backstop, and the consecutive-identical route no-op.
- Verification: `bash scripts/check-topic-docs.sh` -> exit 0,
  `make check-whitespace` -> exit 0, `git diff --check` -> exit 0. No product
  code changed in this round; the repair is contract text.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the architecture WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.
