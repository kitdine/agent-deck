---
status: active
topic: switch-effectiveness-boundary
subject: tasks.md
---

# Review log — switch-effectiveness-boundary / tasks.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `8a13af7155f5a5303b76bfd70ac21cb918ecedb7`;
  `tasks.md` blob `322472a9f7b8de6e95383171640c4f4b967411a3`.
- Reviewer: Codex.
- Method: Formal task-decomposition review against the approved requirements
  and architecture. Reused the current document-set checker result and
  inspected direct affected-test ownership for the config-change reconcile.
- Scope: task anchors, dependencies, file/test ownership, verification levels,
  real-session acceptance procedure, and Documents/Tasks status matrices.
- Findings:
  - [P1] T-F1 — `real-session-acceptance` step 5 still requires an unknown
    `ConfigChange` route, contradicting the approved no-route contract -> open;
    require absence of the new route and continued resolution to the prior
    custom provider/multiplier before restart.
  - [P1] T-F2 — The Tasks matrix still names task 2
    `undeterminable-route-quality` while the task breakdown renamed it
    `unadopted-switch-no-route`, and the Documents prose still says architecture
    is next after its PASS -> open; synchronize the sole dispatch/status
    authority with the reviewed decomposition and current document order.
  - [P1] T-F3 — The document-set rationale and task 4 retain the superseded
    stored/resulting-quality model even though the approved contract writes no
    route and assigns no new quality -> open; describe suppression of the
    misleading route and leave only redesign of the retained evidence sources'
    quality to the v0.6.0 topic.
  - [P1] T-F4 — Task 2 omits
    `cmd/agentdeck/claude_reload_test.go`, the existing direct test owner for
    `reconcileClaudeConfigChange` and its current matched/unknown behavior ->
    open; include and update those tests in the task's Files and coverage.
  - [P1] T-F5 — The central real-session procedure says to confirm the serving
    credential "from the client's own behavior" without naming a reproducible
    signal or the commands/queries that prove route and attribution -> open;
    define the credential/provider discriminator and the exact redacted route,
    event-attribution, and cost checks to record.
- Evidence: `tasks.md:77-79`, `:99`, `:121`, `:128`, and `:148` contradict the
  approved no-route architecture or the document's own task anchors/status.
  `cmd/agentdeck/claude_reload_test.go:15-215` directly exercises
  `reconcileClaudeConfigChange`, including the unknown-route expectation task 2
  must change, but is absent from `tasks.md:56-57`. The previously run
  `scripts/check-topic-docs.sh` passed for this unchanged candidate; it audits
  document-set presence and cannot falsify these semantic defects. Broad
  verification stopped after the decisive blockers.
- Verdict: REOPEN

### 📋 Tasks Decomposition Review

📊 总体评分：4/10

✅ 评审结论：FAIL

#### 🔴 严重问题——必须修复

[`tasks.md:77`](../tasks.md) T-F1：真实会话验收仍要求 unknown route，与已通过的
no-route 合同直接矛盾。
- 行为风险：正确实现会被手工验收判失败，或实现者为让验收通过而
  重新写入被 requirements/architecture 明确否定的 unknown route。
- 证据：Contract 3 要求 credential-deleting switch 不写任何
  `ConfigChange` route，并让 prior route 继续决定 multiplier。
💡 有界修复：将步骤 5 改为确认没有新 `ConfigChange` row，事件仍归属
前序 custom provider/multiplier；重启后再由 `SessionStart` 切到 `official`。

[`tasks.md:148`](../tasks.md) T-F2：Tasks 矩阵仍使用旧 anchor
`undeterminable-route-quality`，与正文的 `unadopted-switch-no-route` 不同；
文档状态又仍说 architecture 是下一份。
- 行为风险：Beads dispatch、开发命令和评审记录会指向不同 Task，而且唯一
  状态权威对当前 document gate 给出错误顺序。
- 证据：`tasks.md:36` 和 `:148` 的 anchor 不同；`:138-141` 已记录
  architecture PASS，`:128` 却仍说它是 next。
💡 有界修复：将矩阵 anchor 与正文统一，并把文档顺序更新为
`tasks.md` 待评审。

[`tasks.md:99`](../tasks.md) T-F3：task 4 与 Documents 理由仍使用旧的
stored/resulting-quality 模型。
- 行为风险：开发可能把一个不存在的新 route quality 纳入 v0.6.0 修改，
  扩大范围或恢复 unknown write。
- 证据：architecture 明确 no new route/no new quality；`tasks.md:121` 仍声称主题
  改变“quality of a stored route”。
💡 有界修复：把主题行为改为“suppress one misleading route”，task 4 只将已存
evidence source 的未来 quality redesign 留给 v0.6.0。

[`tasks.md:56`](../tasks.md) T-F4：task 2 未列出直接覆盖 reconcile 的
`cmd/agentdeck/claude_reload_test.go`。
- 行为风险：任务可以新增邻近测试却遗留旧的 matched/unknown 回归期望，
  或直到广泛套件才发现直接测试失败。
- 证据：`cmd/agentdeck/claude_reload_test.go:15-215` 调用
  `reconcileClaudeConfigChange`，并断言当前 unknown-route 行为。
💡 有界修复：将该文件加入 task 2 Files，明确更新 direct reconcile
回归覆盖。

[`tasks.md:72`](../tasks.md) T-F5：中央真实会话验收没有定义可重复的 credential/provider
判定信号与检查命令。
- 行为风险：“from the client's own behavior”无法让独立审核者判定同一轮运行
  到底是真正验证了凭据方向，还是仅看到请求成功。
- 证据：步骤 3 未命名任何 provider-side/client-side discriminator，步骤 5 也未命名
  检查 route/event attribution/cost 的命令或查询。
💡 有界修复：定义不泄漏 secret 的真实 provider 判别信号，并列出记录 session
ID、route row、event attribution 和 cost multiplier 的精确命令/查询与时间顺序。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- 四个 Task 的主体分层正确：advisory/file-boundary、route/cost、real-session
  acceptance、v0.6.0 premise correction 各自具有独立交付边界。
- task 2 明确要求验证 prior route 的读取结果，而不是只断言没有新 row。
- 真实凭据不进入仓库、审核记录不记录 credential value 的安全边界正确。

#### 📝 总结

评审绑定 HEAD `8a13af7155f5a5303b76bfd70ac21cb918ecedb7` 与 `tasks.md`
blob `322472a9f7b8de6e95383171640c4f4b967411a3`。四任务主体边界可用，但状态
authority、真实会话验收和直接测试所有权仍保留旧的 unknown-route 模型或未决定证据。
所有五项在 PASS 前都必须关闭。

#### 📌 下一步

```text
修复：switch-effectiveness-boundary / reviews/tasks.md / T-F1 T-F2 T-F3 T-F4 T-F5
```

## Round 2 — 2026-08-17

- Reviewed state: HEAD `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8`;
  `tasks.md` blob `719ff3988ebe14699fc39a8f891f5b4ee253866b`.
- Reviewer: Codex, independently evaluating the repaired task matrix.
- Method: Finding-by-finding task-decomposition Re-review. Checked the five
  repaired scopes against the approved requirements/architecture, current
  direct reconcile tests, SQLite schema, and actual `usage scan`, `session scan`,
  and JSON `session show --tokens --all` command contracts.
- Scope: T-F1–T-F5 and new blocking regressions caused by their repair.
- Finding dispositions:
  - **T-F1 — CLOSED.** The real-session procedure now verifies that the
    credential-deleting switch adds no `ConfigChange` route, the route count
    remains unchanged, and subsequent events still resolve and price through
    the prior custom provider until restart.
  - **T-F2 — CLOSED.** The task-2 breakdown and Tasks matrix both use
    `unadopted-switch-no-route`; the Documents prose now records architecture
    PASS and names `tasks.md` as the final document awaiting review.
  - **T-F3 — CLOSED.** Task 4 now says a credential-deleting switch creates no
    resulting route and leaves any future redesign to the retained prior-route
    or session-start evidence sources. The document-set rationale says this
    topic suppresses one misleading stored route rather than changing its
    quality.
  - **T-F4 — CLOSED.** Task 2 explicitly includes and describes the required
    changes to `cmd/agentdeck/claude_reload_test.go`, including removal of the
    existing unknown/multiplier-1 expectation.
  - **T-F5 — CLOSED.** The acceptance setup requires a provider-side non-secret
    credential label/key ID plus request ID, rejects insufficient signals, fixes
    UTC bounds, and gives exact scan, route-count, route-resolution, and session
    cost commands. Current CLI/schema inspection confirms those commands and
    fields exist; invocation JSON exposes catalog and provider cost so the
    distinct multiplier can be checked without exposing a credential.
- Evidence: `scripts/check-topic-docs.sh`, `make check-whitespace`, and
  `git diff --check` pass against the repaired candidate. Current source confirms
  `usage_events.event_at/event_key`, all queried route columns,
  `session show --client --tokens --all`, and `.data.usage`/
  `.data.invocations` response fields. Completion Evidence Profile v1 gate
  `switch-effectiveness-boundary:tasks.md` is `VERIFIED` for the reviewed
  HEAD-plus-blob state, with evidence
  `urn:ce:agent-deck:evidence:switch-effectiveness-boundary:tasks.md:rereview-round-2:719ff3988ebe14699fc39a8f891f5b4ee253866b`.
- Verdict: PASS

### 📋 Tasks Independent Re-review

📊 总体评分：9/10

✅ 复评结论：PASS

#### 🔴 严重问题——必须修复

无。

#### 🟡 建议改进——推荐

无。

#### 🟢 优点

- T-F1/T-F3 已关闭：no-route 合同、真实会话验收和 v0.6.0 前提修正
  使用同一个 retained-evidence 模型。
- T-F2 已关闭：Task anchor、Documents 状态与开发顺序一致。
- T-F4 已关闭：直接 reconcile 回归测试已纳入 task 2 文件与覆盖边界。
- T-F5 已关闭：真实验收不再以“请求成功”代替凭据判定，并将
  provider audit、route row、event attribution 和 cost 证据绑定到同一 UTC 时序。

#### 📝 总结

T-F1–T-F5 均已在 blob `719ff3988ebe14699fc39a8f891f5b4ee253866b`
中关闭，没有新的 tasks.md finding。需求、架构和任务矩阵三份文档
现在均已 Review PASS，但四个实现 Task 仍未开始，因此本轮只关闭
`tasks.md` 文档 Task，不声称 topic 完成。

#### Task checkpoint

- Task：`switch-effectiveness-boundary / tasks.md` @ HEAD
  `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8` + blob
  `719ff3988ebe14699fc39a8f891f5b4ee253866b`
- Completion evidence gate：`VERIFIED`
- 提交建议：仅提交 `tasks.md` 与本评审记录；状态文档只在能安全隔离
  该 Task hunk 时纳入，排除 `docs/README.md` 中的其他 dirty hunk 和所有无关更改。
- 推送建议：获得明确 commit/push 授权，并检查目标分支、远端和提交范围后
  再推送；本 checkpoint 不执行也不授权交付。
