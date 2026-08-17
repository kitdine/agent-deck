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
