---
status: active
topic: v0-5-0-contract
subject: tasks.md
---

# Review log — v0-5-0-contract / tasks.md

## Round 1 — 2026-08-31

- Reviewed state:
  - HEAD `a83ae2ba3fb0d8201043f7f8a5abcaff3b9fec2d`, working tree uncommitted
  - `docs/topics/v0-5-0-contract/tasks.md` reviewed blob
    `f8f78ab3f0f36f9aa48a06f076ff725a0917ba71`
- Reviewer: Codex, independently reviewing the existing version-contract task
  decomposition; production code, tests, and configuration remained read-only.
- Method: design/contract review. Premises and task boundaries were checked
  against `.agent-instructions/branching.md`, `docs/documentation-workflow.md`,
  the five selected topics' authoritative task matrices, and `docs/status.md`.
  The repository topic-document, whitespace, and diff checkers were executed.
- Scope: assembly membership and entry conditions, `assemble` and
  `v0-5-0-contract` task ownership/order/verification, document lifecycle,
  terminal delivery boundary, and the authorities those statements cite.

## 📋 v0.5.0 Contract Closure tasks.md 评审报告

📊 总体评分：6/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

[`docs/topics/v0-5-0-contract/tasks.md:60`]
**[R1-F1] Entry condition 要求每个 selected-topic task 都达到 Review PASS，
但 switch-effectiveness-boundary 的 Task 4 被 operator 明确 waiver 为
`n/a` / `n/a`，没有且不应创建 review record。**

- 行为风险：按本文字面，`assemble` 永远无法进入；满足权威 waiver 会违反这里的
  “every task ... Review PASS”，而满足这里又必须撤销用户已记录的 waiver。版本闭合
  的首个门禁因此不可判定。
- 证据：`docs/topics/switch-effectiveness-boundary/tasks.md` 的权威矩阵将
  `real-session-acceptance` 标为 `n/a` / `n/a`，并说明该 task 于 2026-08-26 被
  operator waiver、程序未执行、没有 review record；本文件却把该 real-session
  acceptance 列入必须 Review PASS 的 entry condition。
- 有界修复：在 entry condition 中明确识别该具名 operator waiver 为该 task 的终态，
  同时保留其“未执行、无 provider-audit evidence”的限制；不要虚构 PASS 或 review
  record，也不要改写 waiver。

[`docs/topics/v0-5-0-contract/tasks.md:94`、
`docs/topics/v0-5-0-contract/tasks.md:107`]
**[R1-F2] 两个任务没有满足 tasks.md readiness 所要求的完整 Files/creates 边界。**

- 行为风险：`assemble` 与 final contract 的实现者仍需自行猜测哪些文件属于各自原子
  commit，尤其 changelog、documentation index/archive lifecycle，以及 app、CLI、
  wire、Cask version identity 的权威来源；在并行 dirty worktree 中这会直接造成漏交付
  或跨 task 暂存。
- 证据：`docs/documentation-workflow.md` 要求每个 task 有 anchor、files 和
  verification level。本文件为 `assemble` 指向 branching 规则和 review record，
  为 final contract 提到两份 specs、index/archive 和 identity reconciliation，但未
  列出完整 `Files` / `creates`，也未命名各 identity authority；只有 final task 写了
  L2，assemble 仅把 verification 路由给 branching。
- 有界修复：为两项分别列出精确 `Files` / `creates`、共享文件的 hunk ownership、
  integration review/evidence artifacts，以及 app/CLI/wire/Cask identity authorities；
  保留 branching 按 merge class 选择验证的规则。

### 🟡 建议改进 — 推荐

[`docs/topics/v0-5-0-contract/tasks.md:11`、
`docs/topics/v0-5-0-contract/tasks.md:60`]
**[R1-F3] 开篇 ownership premise 只说 desktop-app fully reviewed 后开始，
而实际 entry condition 与 assembly list 要求五个 selected topics。**

- 行为风险：读者在最早的范围说明处会把 desktop 当作唯一前置，直到后文才遇到不同
  gate；两个都像规范性陈述，无法判断哪一个支配 dispatch。
- 证据：开篇只链接 native desktop topic；Assembly list 与 Entry condition 明确列出
  desktop、work-signals、CLI error classification、switch effectiveness、usage
  attribution precision 五条。
- 有界改进：让开篇引用 assembly list / entry condition，并写成所有 selected topics，
  不在两个位置维护不同的前置集合。

### 🟢 优点

- Assembly list 清楚区分版本 membership 与 topic ownership，并把 error-code break 和
  attribution release blockers 放在版本层。
- `assemble` 明确允许“nothing to merge”，且把 merge classification、intersection-only
  review 与 integration evidence 路由给 branching authority。
- final contract 与 later preflight/RC/publication 边界分离，commit、push、release、
  signing、installation 均未被任务文档静默授权。

### 📝 总结

被评状态为 HEAD `a83ae2b…` 加文档 blob `f8f78ab3…`。版本 membership 与两任务顺序
总体清楚，但 entry gate 与 operator waiver 互斥，且任务文件边界不足以在当前 dirty
worktree 中安全实施；开篇还保留了 desktop-only 的旧前提。三条 finding 均需在下一轮
前关闭，因此判定 FAIL。未继续扩大 branch/runtime 验证；最小 reproducer 已足以阻断
文档 PASS。

### Evidence

```text
selected topic task matrices
  -> desktop 6/6, work-signals 7/7, cli-error 2/2,
     attribution 3/3; switch tasks 1-3 PASS, task 4 n/a/n/a by operator waiver
bash scripts/check-topic-docs.sh
  -> only the unrelated untracked schema-version-signal missing documents;
     no v0-5-0-contract finding from the structural checker
make check-whitespace
  -> PASS
git diff --check
  -> PASS
```

- Completion gate: FAILED
- Verdict: REOPEN

### 下一步指令

修复：v0-5-0-contract / reviews/tasks.md / R1-F1, R1-F2, R1-F3

## Repair Round 1 — 2026-09-01

- Repaired state:
  - HEAD `a83ae2ba3fb0d8201043f7f8a5abcaff3b9fec2d`, working tree uncommitted
  - `docs/topics/v0-5-0-contract/tasks.md` repaired blob
    `d8e6539eaadf1b59bdc2fd635634bf9207f39408`
    (reviewed blob was `f8f78ab3f0f36f9aa48a06f076ff725a0917ba71`)
- Repairer: Claude Code, acting on the three Round 1 findings only. No production
  code, test, configuration, or other topic's document was changed.
- Scope: `docs/topics/v0-5-0-contract/tasks.md` alone. The authorities the
  repaired text now names were read to confirm each claim, and none was edited.

### Finding disposition

| Finding | Disposition | Change |
| --- | --- | --- |
| R1-F1 | Closed | Entry condition rewritten |
| R1-F2 | Closed | Both tasks given commit-baseline `Files` / `Creates` |
| R1-F3 | Closed | Opening premise routed to the assembly list and entry condition |

**[R1-F1] Entry gate versus the operator waiver.** `## Entry condition` no longer
demands Review PASS for every task. It now requires each selected topic's task to
have reached "its terminal reviewed state in that topic's own status authority",
states that terminal means Review PASS everywhere except the one recorded case,
and then names that case explicitly: `switch-effectiveness-boundary`'s
`real-session-acceptance` is `n/a` / `n/a` by the operator's 2026-08-26 waiver,
which *is* this gate's terminal state for that task. The bounded fix's second
half is kept intact rather than paraphrased away — the repaired text carries the
waiver's own limitation forward (procedure not executed, no review record exists
or should be created, standing operator experience rather than recorded
provider-audit evidence) and says the gate neither reconstructs it as a PASS nor
rewrites it. Switch effectiveness's other three tasks still require independent
Review PASS. Task 2's bullet at the old line 115 restated the same "every task …
independent Review PASS" claim and would have kept the contradiction alive behind
the repaired section; it now defers to `#entry-condition` instead of carrying a
second copy of the gate.

**[R1-F2] Task file boundaries.** Both tasks now carry the commit-baseline
`Files` / `Creates` form this repository already established in
`docs/topics/desktop-app/tasks.md`, together with the baseline rule itself, so a
path existing only as uncommitted work reads as a file the task creates.
`assemble` lists this `tasks.md` (its own matrix row) and `docs/status.md` (its
stage row), records `reviews/assemble.md` as what it creates, states that
integration evidence is a `completion-evidence/v1` record rather than a tracked
file, and states hunk ownership on the two files it shares with task 2. The
final contract task lists `docs/specs/cli-design.md`, `docs/specs/cli-manual.md`,
`docs/status.md`, `docs/archive/README.md`, `docs/README.md` under its stated
condition, this `tasks.md`, and the selected topics' documents the archive move
re-stamps; it creates `reviews/v0-5-0-contract.md`. The two ambiguities the
finding named are answered directly: the changelog is the `## Changelog` table in
`docs/specs/cli-design.md` raised with that file's `version:` frontmatter (the
repository has no `CHANGELOG.md`, and release notes belong to the separately
authorized release workflow), and the index/archive lifecycle is the
`git mv` + `status: historical` + `retired:` + `docs/archive/README.md` sequence
`docs/documentation-workflow.md` owns. Each of the four identity authorities is
named at its source rather than by a copied value.

**[R1-F3] Opening ownership premise.** The opening paragraph now says the topic
reconciles after every topic in the assembly list has reached the entry
condition's terminal state, links both sections, and states that the desktop
topic is one of the five rather than the gate. The prerequisite set is therefore
maintained in one place; the opening no longer states a competing one.

### Verification

```text
git hash-object docs/topics/v0-5-0-contract/tasks.md
  -> d8e6539eaadf1b59bdc2fd635634bf9207f39408
bash scripts/check-topic-docs.sh
  -> only the unrelated untracked schema-version-signal topic is reported;
     no v0-5-0-contract finding
make check-whitespace
  -> PASS (exit 0)
git diff --check
  -> PASS
```

Identity-authority claims were each verified against the repository rather than
asserted: `apps/macos/Config/AgentDeck.xcconfig:9`
(`AGENTDECK_MARKETING_VERSION = 0.5.0`), `Makefile:9-19`
(`VERSION_TAG := git describe --tags --abbrev=0`, `BUILD_LDFLAGS`,
`APP_VERSION`), `scripts/build-macos-app.sh:74-75`,
`internal/desktop/desktop.go:21` (`WireVersion = 1`),
`apps/macos/AgentDeckShared/DesktopWire.swift:84`, and
`scripts/render-homebrew-formula.sh:56` with
`packaging/homebrew/agentdeck.rb.tmpl:10`. The waiver text quoted by the entry
condition is `docs/topics/switch-effectiveness-boundary/tasks.md:345-357` and
its matrix row at line 393.

Verification stayed at documentation level because the repair changed one
Markdown file and no product surface; no L1+ run applies.

- Completion gate: not queried in this phase — Repair records disposition, and
  the gate belongs to the Re-review round that judges this content state.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

### 下一步指令

复评：v0-5-0-contract / reviews/tasks.md

## Round 2 — 2026-09-01

- Reviewed state:
  - HEAD `a83ae2ba3fb0d8201043f7f8a5abcaff3b9fec2d`, working tree uncommitted
  - Re-review candidate blob
    `d8e6539eaadf1b59bdc2fd635634bf9207f39408`
  - Final synchronized `docs/topics/v0-5-0-contract/tasks.md` blob
    `16d99383e47c0633a81fe7c9d13198bc91e1ec1b`; the only post-verdict change
    from the candidate is the current-state paragraph required by this round
- Reviewer: Codex, independently re-reviewing Claude Code's Repair Round 1;
  production code, tests, configuration, and task definitions remained
  read-only during finding disposition.
- Method: finding-by-finding design/contract Re-review. The exact Repair delta
  from Round 1 blob `f8f78ab3...` to candidate blob `d8e6539e...` was checked
  against the selected topics' live task matrices, the operator-waiver source,
  `.agent-instructions/branching.md`, the documentation lifecycle, the named
  release-identity authorities, and the sole Contract Closure row in
  `docs/status.md`. Broader semantic verification stopped after the decisive
  R1-F2 reproducer; the mandatory topic-document audit and L0 hygiene checks
  were still run.
- Scope: R1-F1, R1-F2, and R1-F3, plus regressions introduced by their bounded
  Repair. No merge, implementation, delivery, release, or runtime behavior was
  reviewed.

## 📋 v0.5.0 Contract Closure tasks.md 复评报告

📊 总体评分：8/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

[`docs/topics/v0-5-0-contract/tasks.md:130`、
`docs/topics/v0-5-0-contract/tasks.md:139`、
`docs/topics/v0-5-0-contract/tasks.md:189`]
**[R1-F2] 两项任务的 `docs/status.md` hunk ownership 仍不重叠，且依赖一个
实际不存在的 `assemble` 状态行。**

- 处置：仍未关闭。
- 行为风险：`assemble` 的 Files 声明要求它修改“this topic's stage row”，但紧接
  着的 ownership 声明又把 `v0-5-0-contract` row 交给 task 2；task 2 再排除由
  task 1 拥有的 `assemble` row。当前 `docs/status.md` 只有一条 Contract Closure
  topic row，没有 task-specific `assemble` row，因此实现者仍无法判断 Task 1 与
  Task 2 各自应暂存该共享行的哪个状态转换。并行 dirty worktree 下，原 finding
  所指出的跨 task 暂存风险仍然存在。
- 证据：`rg -n 'assemble|Contract Closure' docs/status.md` 只返回第 68 行的
  Contract Closure topic row；本文件第 131 行把该 topic stage row 列入
  `assemble`，第 140 行把 `v0-5-0-contract` row 列入 task 2，第 189–190 行又称
  task 2 应排除一个不存在的 `assemble` row。
💡 有界修复：明确 `docs/status.md` 只有一条 Contract Closure row，并按顺序内容
状态划分 ownership：Task 1 只拥有从当前基线到“assembly complete / final contract
pending”的该行 hunk，Task 2 只拥有从 Task 1 已提交基线到最终 contract closure 的
后续 hunk。删除 separate `assemble` row 的假设，不改其他 Files、identity authority
或 entry-condition 文本。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R1-F1 已关闭：entry condition 以 selected topic 自身 status authority 的终态为
  准，具名承认 `real-session-acceptance` 的 operator waiver，又明确不把 waiver
  伪装成 Review PASS，并完整保留“未执行、无 review record、无 provider-audit
  evidence”的限制。
- R1-F3 已关闭：开篇只链接 assembly list 与 entry condition，desktop 明确只是
  五个 selected topics 之一，不再形成 desktop-only 的竞争门禁。
- R1-F2 的其他部分已有实质改进：changelog、archive lifecycle、integration
  evidence、merge-class verification，以及 app/CLI/wire/Cask identity authorities
  都已定位到现有项目权威来源。

### 📝 总结

R1-F1 与 R1-F3 在 repaired blob `d8e6539e...` 中关闭；R1-F2 因唯一
`docs/status.md` Contract Closure row 的 ownership 自相矛盾而仍然开放。最终同步
内容状态为 HEAD `a83ae2b...` 加 blob `16d99383...`。这个矛盾直接阻断 task-scoped
implementation/commit isolation，因此本轮判定 FAIL；未将它降级为建议项，也未把
任何 finding 延后到 PASS。决定性 reproducer 出现后未扩大语义审查；残余不确定性仅
是未执行与该 finding 无关的广泛检查，且本次没有产品/runtime 改动需要 L1+ 验证。

### Evidence

```text
Round 1 blob -> repaired candidate diff
  -> R1-F1/R1-F2/R1-F3 的 bounded Repair 与状态段落
selected topic task matrices + switch waiver authority
  -> desktop 6/6, work-signals 7/7, cli-error 2/2,
     attribution 3/3; switch tasks 1-3 PASS, task 4 n/a/n/a by operator waiver
CodeGraph + named identity-source inspection
  -> internal/buildinfo/buildinfo.go owns Version; Makefile injects it;
     app, wire, Swift consumer, Cask template/render sources all exist
rg -n 'assemble|Contract Closure' docs/status.md
  -> one Contract Closure row; no assemble row
bash scripts/check-topic-docs.sh
  -> only unrelated untracked schema-version-signal gaps; no
     v0-5-0-contract finding from the structural checker
make check-whitespace
  -> PASS
git diff --check
  -> PASS
```

- Completion gate: FAILED — exact target state
  `urn:ce:agent-deck:state:document:15f43a34e472b527797c09bf3b26ad0eb5b56e9124a7c50b060764c561f5ccd7`
  has 2/3 required criteria passing and one disproving R1-F2 evidence record.
- Verdict: REOPEN

### 下一步指令

修复：v0-5-0-contract / reviews/tasks.md / R1-F2

## Repair Round 2 — 2026-09-01

- Repaired state:
  - HEAD `a83ae2ba3fb0d8201043f7f8a5abcaff3b9fec2d`, working tree uncommitted
  - `docs/topics/v0-5-0-contract/tasks.md` repaired blob
    `c63646fd4fdfb3318805c19f629e15544fc65837`
    (Round 2's reviewed state was `16d99383e47c0633a81fe7c9d13198bc91e1ec1b`)
- Repairer: Claude Code, acting on Round 2's single open finding. No production
  code, test, or configuration was touched, and no other topic's document was
  changed.
- Scope: R1-F2 only, and within it only the `docs/status.md` ownership
  statements the finding named. Round 2 recorded R1-F1 and R1-F3 as closed, so
  the entry condition, opening premise, changelog location, archive lifecycle,
  integration-evidence wording, merge-class verification routing, identity
  authorities, and every other `Files` / `Creates` entry were left exactly as
  Round 2 judged them.

### Finding disposition

| Finding | Disposition | Change |
| --- | --- | --- |
| R1-F2 | Closed | The single Contract Closure row split by content state, not by row |

**[R1-F2] `docs/status.md` hunk ownership.** The contradiction Round 2 reproduced
was real: `docs/status.md` carries one `v0.5.0` Contract Closure row and nothing
else for this topic, while task 1 claimed "this topic's stage row", task 2
claimed "this topic's rows", and task 2 then excluded an `assemble` row that has
never existed. Three statements over one row, one of them about a row that is not
there.

The repair takes the bounded fix as written. Task 1's hunk-ownership bullet now
opens by stating the fact both tasks depended on and neither said — that
`docs/status.md` carries exactly one row for this topic and that no
task-specific `assemble` row exists or is to be created — with the reason a
second row is not the escape hatch: it would put one topic's status in two
places, which is precisely what a single status row is the authority against.
The two tasks then divide that one row by **sequential content state rather than
by row identity**: `assemble` owns only the hunk carrying the row from its
baseline when the task starts to *assembly complete, final contract pending*,
and stages nothing past that step; every later state of the same row through
final contract closure is task 2's, together with the `Documents` matrix and the
version-identity and specification lines. Because `assemble` commits first, task
2 takes its hunk against the baseline `assemble` has already committed, so the
two tasks never hold a competing edit of the same line in the shared worktree —
which is the isolation the original finding asked for.

Both `Files` bullets were brought into agreement with that split rather than
left to restate it: task 1 names "the single `v0.5.0` Contract Closure row, in
the one content-state step the ownership bullet below assigns to this task", and
task 2 names "the same single Contract Closure row, in its later content state —
the hunk from the baseline `assemble` committed through final contract closure —
plus the version status lines; **not** the earlier step task 1 owns". The phrase
`assemble` row is gone from the file.

### Verification

```text
git hash-object docs/topics/v0-5-0-contract/tasks.md
  -> c63646fd4fdfb3318805c19f629e15544fc65837
rg -n 'assemble` row' docs/topics/v0-5-0-contract/tasks.md
  -> 3 hits, none of them an assumption: line 142 and line 254 are the two
     explicit denials this repair added, and line 248 is Round 2's status note
     describing the defect. The bullet that assumed the row -- task 2's
     "**but not** the `assemble` row task 1 owns" -- is gone.
rg -n 'assemble|Contract Closure' docs/status.md
  -> one Contract Closure row, unchanged by this repair
bash scripts/check-topic-docs.sh
  -> only the unrelated untracked schema-version-signal topic is reported;
     no v0-5-0-contract finding
make check-whitespace
  -> PASS (exit 0)
git diff --check
  -> PASS
```

The repair changed one Markdown file plus the two status projections this round
owns, and no product surface, so verification stayed at documentation level and
no L1+ run applies.

- Completion gate: not queried in this phase — Repair records disposition, and
  the gate belongs to the Re-review round that judges this content state. Round
  2's `FAILED` result for target state
  `urn:ce:agent-deck:state:document:15f43a34…` stands as immutable evidence
  against that superseded state.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

### 下一步指令

复评：v0-5-0-contract / reviews/tasks.md

## Round 3 — 2026-09-01

- Reviewed state:
  - HEAD `a83ae2ba3fb0d8201043f7f8a5abcaff3b9fec2d`, working tree uncommitted
  - Re-review candidate blob
    `c63646fd4fdfb3318805c19f629e15544fc65837`
  - Final synchronized `docs/topics/v0-5-0-contract/tasks.md` blob
    `a1d5489e548ebf40ed6263687c863e06598ee26a`; the post-verdict delta only
    ticks the Document Review cell and appends this round's current-state
    paragraph
- Reviewer: Codex, independently re-reviewing Claude Code's Repair Round 2;
  production code, tests, configuration, and task definitions remained
  read-only during finding disposition.
- Method: finding-by-finding design/contract Re-review. R1-F2's three repaired
  ownership statements were checked together against the one live Contract
  Closure row, task ordering and commit baselines. R1-F1 and R1-F3 were checked
  for regression against their unchanged entry-condition, waiver, and opening
  scope text. The mandatory topic-document audit and L0 hygiene checks were
  executed after status synchronization.
- Scope: R1-F2 closure plus regression checks for R1-F1 and R1-F3. No merge,
  implementation, delivery, release, or runtime behavior was reviewed.

## 📋 v0.5.0 Contract Closure tasks.md 复评报告

📊 总体评分：10/10

✅ 判定：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R1-F1 保持关闭：entry condition 继续把具名 operator waiver 作为
  `real-session-acceptance` 的终态，同时明确它不是 Review PASS，也没有
  provider-audit evidence 或 review record。
- R1-F2 已关闭：Task 1 与 Task 2 都以唯一 Contract Closure row 为对象，明确不
  创建 task-specific `assemble` row，并把 ownership 分成前后两个提交基线上的内容
  状态。Task 1 止于 *assembly complete, final contract pending*；Task 2 从该已提交
  基线推进到 final closure，Files 与 hunk-ownership 文字一致。
- R1-F3 保持关闭：开篇继续把 prerequisite set 路由到 assembly list 与 entry
  condition，desktop 只被识别为五个 selected topics 之一。
- Changelog、archive lifecycle、integration evidence、merge-class verification
  和 app/CLI/wire/Cask identity authorities 均保持上一轮已核验的明确边界。

### 📝 总结

Round 1 的 R1-F1、R1-F2、R1-F3 已全部关闭，Repair Round 2 没有引入回归或新
blocking finding。最终同步内容状态为 HEAD `a83ae2b...` 加 blob
`a1d5489e...`，Document Review cell 已勾选，topic 进入 developable 状态。残余不
确定性仅为工作区中另一个 untracked topic 的既有文档缺口；它不影响本 WorkUnit，且
本轮没有产品/runtime 改动需要 L1+ 验证。

### Evidence

```text
R1-F2 repaired ownership statements
  -> one Contract Closure row; no task-specific assemble row;
     task 1 and task 2 own sequential committed content states
R1-F1 / R1-F3 regression check
  -> entry condition, operator waiver, and five-topic opening premise unchanged
rg -n 'assemble|Contract Closure' docs/status.md
  -> one Contract Closure row, now projected as Developable
bash scripts/check-topic-docs.sh
  -> only unrelated untracked schema-version-signal gaps; no
     v0-5-0-contract finding from the structural checker
make check-whitespace
  -> PASS
git diff --check
  -> PASS
```

- Completion gate: VERIFIED — exact target state
  `urn:ce:agent-deck:state:document:5af6e1bf0b3a0394f3ca01823916195e27adefcf4fdd7aaec2121340fe6839fe`
  has passing evidence for all 3/3 required criteria.
- Verdict: PASS

### Task checkpoint

Task checkpoint：文档 Task `ad-v050c-doc-tasks-design` / WorkUnit
`v0-5-0-contract:tasks.md`，内容状态为 HEAD `a83ae2b...` 加 blob
`a1d5489e...`，CEv1 gate `VERIFIED` 3/3。

提交建议：仅提交该文档 Task 的 `docs/topics/v0-5-0-contract/tasks.md`、
`docs/topics/v0-5-0-contract/reviews/tasks.md` 与 `docs/status.md` 中 Contract
Closure row 的本 Task hunk；排除 `docs/topics/work-signals/tasks.md`、
`docs/topics/schema-version-signal/**` 及任何无关 dirty hunk。建议不构成提交授权。

推送建议：如后续明确授权并完成 signed commit、完整 message/trailer/tree/signature
检查，可从当前 `main` 推送到 `origin/main`
(`https://github.com/kitdine/agent-deck.git`)；建议不构成推送授权。

### 下一步指令

开发：v0-5-0-contract / assemble
