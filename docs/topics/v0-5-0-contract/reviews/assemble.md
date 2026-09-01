---
status: active
topic: v0-5-0-contract
subject: assemble
---

# Review log — v0-5-0-contract / assemble

This record holds every merge into `v0.5.0`, one round per merge, per
`.agent-instructions/branching.md`. The entry below is not a round: no merge was
performed, so there is no merge tree for a round to review. It records what the
task did instead, which the task definition requires be said rather than skipped.

## Merge log — 2026-09-01

- Performed by: Claude Code, executing `v0-5-0-contract` task 1 `assemble`.
- Repository state at classification: HEAD
  `86b57ea95a6b0134412287a994213cc2da1a0d31` on `main`, tree
  `760ea9a3c5c4e6a70b4a5e830d1c98d72746f178`.
- **Outcome: nothing to merge.**

### Classification

`.agent-instructions/branching.md` classifies a merge before it happens, and the
class decides how much work follows. Classification here found no merge to
classify: **no `feature/<topic>` branch exists for any topic in the assembly
list**, locally or on the remote.

| Ref | Object | Relation to this task |
| --- | --- | --- |
| `refs/heads/main` | `86b57ea` | The line being assembled |
| `refs/heads/review/terminal-presentation-remediation-v2` | `6b7663b` | Not in the assembly list; a completed 2026-08-12 review branch already contained in `main` |
| `refs/remotes/origin/main` | `f5935b6` | Behind local `main`; the remote carries no other head |

`git ls-remote --heads origin` returns exactly one ref, `refs/heads/main`. There
is no `feature/desktop-app`, `feature/work-signals`,
`feature/cli-error-classification`, `feature/switch-effectiveness-boundary`, or
`feature/usage-attribution-precision` to classify or merge.

This is the state `.agent-instructions/branching.md` itself describes: "`main` is
not protected yet. Until it is, `v0.5.0` development continues on `main` directly
and `main` is in effect that version's feature line." The five selected topics
were developed on `main`, so their content reached the version line by ordinary
commit rather than by assembly.

### The outcome is proven, not assumed

A missing branch alone would be equally consistent with content that never
landed. Each selected topic's delivered content is therefore checked to be in
`main`'s ancestry at the classified HEAD:

| Topic | Delivering commit | `git merge-base --is-ancestor <commit> HEAD` |
| --- | --- | --- |
| [`desktop-app`](../../../archive/topics/desktop-app/tasks.md) | `0aefed1` — `docs(macos): close desktop app topic` | ancestor |
| [`work-signals`](../../../archive/topics/work-signals/tasks.md) | `a83ae2b` — `docs(work-signals): reconcile delivered signal contract` | ancestor |
| [`cli-error-classification`](../../../archive/topics/cli-error-classification/requirements.md) | `574a7ad` — `feat: add stable CLI error codes` | ancestor |
| [`switch-effectiveness-boundary`](../../../archive/topics/switch-effectiveness-boundary/tasks.md) | `7db5618` — `fix(usage): retain effective routes across Claude switches`; `5f21895` — `docs(switch-effectiveness): waive the real-session acceptance task` | ancestor |
| [`usage-attribution-precision`](../../../archive/topics/usage-attribution-precision/tasks.md) | `9035b80` — `feat(usage): expose attribution observability` | ancestor |

All five topic directories are present at HEAD under `docs/topics/`. Membership
of each topic is whole, per the assembly list; no topic is partially present.

### Integration review scope

**None, and the reason is that there is no intersection to review.** Integration
review covers content a merge newly produced — files both sides touched,
consuming call sites across a changed interface, and lines written to resolve a
conflict. No merge occurred, so none of those three sets exists.

Out of scope, and covered elsewhere: every selected topic's own behavior, which
carries task-level review records under each topic's `reviews/` directory. This
task re-reviews none of it, per the rule that a second verdict on the same
content competes with the first.

### Integration evidence

**No `unit_kind: integration` record was written, because there is no merge tree
to bind one to.** The task's bullet requires integration evidence "bound to the
merge tree"; a record bound to something that is not a merge tree would assert an
integration that never happened. The evidence recorded instead is this task's own
`unit_kind: task` WorkUnit `v0-5-0-contract:assemble`, whose criteria include this
absence and its ground, so the gate answers a question that was actually asked.

A later sync or a topic developed on a real `feature/*` branch would append a
`## Round 1` here with the merge tree, both parent trees, the merge type, and the
integration verdict — the mechanism is unused, not absent.

### Verification

`.agent-instructions/branching.md` routes verification by merge class. No merge
occurred, so none of the three classes applies and no product verification is
triggered: no tree changed, no code was written, and no conflict was resolved.
Verification is therefore the documentation level this task's own change sits at.

```text
git branch -a --format='%(refname) %(objectname:short)'
  -> main, review/terminal-presentation-remediation-v2,
     origin/HEAD, origin/main; no feature/* ref
git ls-remote --heads origin
  -> f5935b6 refs/heads/main only
git merge-base --is-ancestor <commit> HEAD, for all five delivering commits
  -> exit 0 in every case
git ls-tree --name-only HEAD docs/topics/
  -> all five selected topics plus v0-5-0-contract
bash scripts/check-topic-docs.sh
  -> only the unrelated untracked schema-version-signal topic; no
     v0-5-0-contract finding
make check-whitespace
  -> PASS (exit 0)
git diff --check
  -> PASS
```

### Delivery boundary

This task's candidate changes are only what `tasks.md` assigns it: its own
`Tasks` matrix row, this record, and the single `docs/status.md` Contract
Closure row carried to *assembly complete, final contract pending*.

They are working-tree changes and nothing more. **The Git index remained empty
throughout: nothing was staged, committed, or pushed.** Development carries no
staging or commit authority, so isolating the index is the later commit
checkpoint's work rather than this task's, and this record asserts no index
state for that checkpoint to rely on.

The concurrently dirty `docs/topics/work-signals/tasks.md` hunk, the Work
Signals and Schema Version Signal status rows, and the untracked
`docs/topics/schema-version-signal/` topic belong to other tasks and were left
untouched. The evidence bound for this task excludes them by construction rather
than by staging: its `status-scoped` component is `docs/status.md` at HEAD with
only this task's Contract Closure row applied, which is a computed blob that was
never written to the index.

### 下一步指令

评审：v0-5-0-contract / assemble

## Round 1 — 2026-09-01

- Reviewed state:
  - HEAD `86b57ea95a6b0134412287a994213cc2da1a0d31` on `main`, tree
    `760ea9a3c5c4e6a70b4a5e830d1c98d72746f178`
  - Development candidate
    `urn:ce:agent-deck:state:candidate:f862e3f51498c4c2eb6aaf9ec6aee4c852943b80fa891f01927780cc50d86492`
  - Candidate recipe: tasks blob `ee03f5f5...`, reviewed assemble-record blob
    `ba63ba4a...`, and scoped status blob `b47263c3...`
- Reviewer: Codex, independently reviewing Claude Code's `assemble`
  Development result; production code, tests, configuration, Git refs, and Git
  history remained read-only.
- Method: process/task review using the `branching.md` merge-classification and
  intersection-only dimensions. Local and remote refs, every selected-topic
  delivering commit, entry matrices, task-scoped diff, the Git index, CEv1
  criteria/evidence, and the absence of integration WorkUnits were checked
  directly rather than accepted from the Development record.
- Scope: the nothing-to-merge classification and proof, integration review and
  evidence boundary, `reviews/assemble.md`, the `assemble` Tasks-matrix row,
  and the single Contract Closure status-row transition. Selected-topic product
  behavior was excluded because its own task reviews already cover it.

## 📋 v0.5.0 Contract Closure assemble 评审报告

📊 总体评分：9/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[`docs/topics/v0-5-0-contract/reviews/assemble.md:116`]
**[R1-F1] Delivery boundary 把未发生的 Git staging 写成已发生事实。**

- 行为风险：该段说 “This task staged only what `tasks.md` assigns it”，但当前
  Git index 为空，三个 Task work products 都是 unstaged。后续 commit checkpoint
  会据此误以为 index scope 已经隔离并检查过，而且 Development 阶段没有 staging
  或 commit 授权；评审记录因此断言了一个未发生的交付状态转换。
- 证据：`git diff --cached --stat` 无输出；`git status --short` 对
  `docs/status.md`、`docs/topics/v0-5-0-contract/tasks.md` 显示 ` M`，对本记录显示
  `??`，没有 staged entry。
💡 有界修复：只把 “This task staged only” 改为 “This task's candidate changes
only” 或等价准确表述，并明确 Git index remained empty；保留现有三项 owned scope、
其他 dirty work 排除项与 delivery ceiling，不改 classification、ancestry、CEv1 或
任何产品文件。

### 🟢 优点

- `git branch -a` 与 `git ls-remote --heads origin` 独立复现：没有任何 selected
  topic 的 `feature/*` ref，远端只有 `main`。
- 六个 selected-topic delivering commits/waiver commits 都是 HEAD ancestors；
  五份 HEAD task matrix 也处于规定终态，因此 missing branch 没被错误当成 delivered
  content 的替代证明。
- 没有 merge tree、conflict、intersection 或 consuming-call-site change，因而不
  创建 `unit_kind: integration` WorkUnit 是正确的；graph 查询也确认该空集。
- Development candidate 的 tasks/record/status scoped components、composite
  digest `f862e3f5...` 与四条 supersedes 关系均可重算，其他 dirty work 没进入该
  candidate。

### 📝 总结

Nothing-to-merge 的实质判断、范围和 evidence model 均正确，唯一 finding 是记录把
unstaged candidate 误写成 staged。被评状态为 HEAD `86b57ea...` 加 candidate
`f862e3f5...`；这条审计真实性缺陷虽不影响产品或 ancestry 结论，但不能留到 PASS
之后，因此本轮判定 FAIL。决定性证据出现后未运行无关产品套件；残余不确定性仅是
远端 ref 未来可能变化，本轮已用实时 `ls-remote` 绑定当前判断。

### Evidence

```text
git branch -a --format='%(refname) %(objectname:short)'
  -> main, contained review branch, origin/HEAD, origin/main; no feature/*
git ls-remote --heads origin
  -> f5935b6 refs/heads/main only
git merge-base --is-ancestor <commit> HEAD
  -> selected delivery/waiver commits and unrelated review branch all exit 0
git ls-tree --name-only HEAD docs/topics/
  -> all five selected topics plus v0-5-0-contract
candidate component and digest reconstruction
  -> ee03f5f5 / ba63ba4a / b47263c3 -> f862e3f5
CEv1 supersedes and integration WorkUnit queries
  -> four current pass records supersede 912c8786; integration WorkUnit set empty
git diff --cached --stat
  -> empty
```

- Completion gate: FAILED — exact target state
  `urn:ce:agent-deck:state:candidate:159496c97f3dca9f5ddd86418385b4815d0ea188a851e0d9c7a2e06c0b9d5439`
  has 3/4 required criteria passing and one disproving R1-F1 evidence record.
- Verdict: REOPEN

### 下一步指令

修复：v0-5-0-contract / reviews/assemble.md / R1-F1

## Repair Round 1 — 2026-09-01

- Repaired state: HEAD `86b57ea95a6b0134412287a994213cc2da1a0d31`, working tree
  uncommitted; this record and the two status projections changed, nothing else.
- Repairer: Claude Code, acting on Round 1's single finding. No product code,
  test, configuration, Git ref, Git index entry, or CEv1 record was touched, and
  the classification, ancestry, integration-scope, and evidence sections are
  byte-identical to the reviewed state.
- Scope: the `Delivery boundary` section of this record, plus the `tasks.md` and
  `docs/status.md` notes that project this round.

### Finding disposition

| Finding | Disposition | Change |
| --- | --- | --- |
| R1-F1 | Closed | `Delivery boundary` no longer asserts staging; it states the empty index |

**[R1-F1] The record asserted a Git staging that never happened.** The finding is
correct and the sentence was wrong in the way that matters most for a review
record: it described a delivery state transition, in a phase that has no
authority to perform one. `git diff --cached --stat` is empty; the three work
products are unstaged (` M` for `docs/status.md` and
`docs/topics/v0-5-0-contract/tasks.md`, `??` for this record). A later commit
checkpoint reading "This task staged only what `tasks.md` assigns it" would have
believed the index was already isolated and checked.

The bounded fix is taken as written. The sentence now reads "This task's
candidate changes are only what `tasks.md` assigns it", and the section states
explicitly that the Git index remained empty and that nothing was staged,
committed, or pushed — with the reason, which is that Development carries no
staging or commit authority, so isolating the index belongs to the commit
checkpoint and this record asserts no index state for it to rely on.

The three owned scope items, the excluded dirty work, and the delivery ceiling
are all preserved. One clarification was added inside the same finding's subject
rather than beyond it: the exclusion of the other tasks' dirty work is achieved
**by construction, not by staging** — the bound `status-scoped` component is
`docs/status.md` at HEAD with only this task's Contract Closure row applied, a
computed blob that was never written to the index. Round 1 verified that
construction independently; saying so is what keeps the corrected paragraph from
implying the exclusion needed an index to be real.

### Verification

```text
git diff --cached --stat
  -> empty, re-confirmed after the repair; the index was never written
awk '/^### Delivery boundary$/,/^### 下一步指令$/' \
  docs/topics/v0-5-0-contract/reviews/assemble.md | rg -n 'staged'
  -> 1 hit, and it is the negation: "nothing was staged, committed, or pushed".
     The assertion is gone from the section that made it. The old sentence
     survives elsewhere in this file only inside quotation marks -- Round 1's
     finding, Round 1's bounded fix, and this round restating it above. A
     repaired record keeps those; it is the live assertion that had to go. No
     whole-file count is recorded here, because any line quoting the defect
     changes that number, including this one.
make check-whitespace
  -> PASS (exit 0)
git diff --check
  -> PASS
bash scripts/check-topic-docs.sh
  -> only the unrelated untracked schema-version-signal topic
```

The classification, ancestry, integration-scope, and integration-evidence
sections were not edited, so Round 1's verification of them carries forward
unchanged. No merge class applies and no product surface changed, so no L1+ run
is triggered.

- Completion gate: not queried in this phase — Repair records disposition, and
  the gate belongs to the Re-review round that judges this content state. Round
  1's `FAILED` result at candidate `159496c9…` stands as immutable evidence
  against that superseded state.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

### 下一步指令

复评：v0-5-0-contract / reviews/assemble.md

## Round 2 — 2026-09-01

- Reviewed state:
  - HEAD `86b57ea95a6b0134412287a994213cc2da1a0d31` on `main`, working tree
    uncommitted
  - Repaired assemble subject blob
    `1b4df78aaf28370d5feaff74a65b3c83d9629876`
  - Superseded Round 1 target state
    `urn:ce:agent-deck:state:candidate:159496c97f3dca9f5ddd86418385b4815d0ea188a851e0d9c7a2e06c0b9d5439`
- Reviewer: Codex, independently re-reviewing Claude Code's Repair Round 1;
  production code, tests, configuration, Git refs, Git history, Git index, and
  CEv1 records remained read-only during finding disposition.
- Method: finding-by-finding process/task Re-review. The live Delivery boundary
  was checked against `git diff --cached`, section-scoped text inspection, and
  the current working-tree status. Round 1's unchanged ref, ancestry,
  integration-scope, and integration-evidence results were reused against the
  same HEAD and byte-identical sections.
- Scope: R1-F1 closure and regressions introduced by its bounded Repair. The
  selected topics' already reviewed product behavior remained out of scope.

## 📋 v0.5.0 Contract Closure assemble 复评报告

📊 总体评分：10/10

✅ 判定：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R1-F1 已关闭：live Delivery boundary 只描述 candidate changes，并明确 Git
  index 始终为空，nothing was staged、committed or pushed；不再为后续 commit
  checkpoint 断言一个虚假 index state。
- 三项 owned scope、其他 dirty work 排除项和 delivery ceiling 均被保留；
  `status-scoped` 仍以 HEAD 加单一 Contract Closure row 的构造隔离其他任务内容，
  不依赖 staging。
- Classification、selected-topic ancestry、empty integration scope 与不创建
  `unit_kind: integration` evidence 的内容保持 byte-identical，Round 1 的独立证据
  继续有效。

### 📝 总结

R1-F1 已关闭，没有回归或新增 finding。被复评 subject 为 HEAD `86b57ea...` 加
repaired blob `1b4df78a...`；Task matrix 与 status projection 已同步到 assemble
Review PASS。残余不确定性仅为远端 ref 未来可能变化，它不影响 Round 1 已绑定并在
本次内容状态未改动的 nothing-to-merge 判断；本轮没有产品/runtime 变更需要 L1+
验证。

### Evidence

```text
git diff --cached --stat
  -> empty
Delivery boundary section-scoped rg 'staged'
  -> one hit: the negation "nothing was staged, committed, or pushed"
git status --short
  -> task products remain unstaged; unrelated dirty work remains separate
reused Round 1 evidence
  -> refs, ancestry, matrices, integration WorkUnit empty set, and candidate
     construction remain valid at unchanged HEAD and unchanged sections
make check-whitespace
  -> PASS
git diff --check
  -> PASS
```

- Completion gate: VERIFIED — exact target state
  `urn:ce:agent-deck:state:candidate:476430b549b48d2b604150823958513f606003e7f60b3ef09858969841e3c864`
  has passing evidence for all 4/4 required criteria.
- Verdict: PASS

### Task checkpoint

Task checkpoint：Task `ad-v050c-assemble-dev` / WorkUnit
`v0-5-0-contract:assemble`，内容状态为 HEAD `86b57ea...` 加 candidate
`476430b5...`，CEv1 gate `VERIFIED` 4/4。

提交建议：仅提交 `assemble` Task 的 `reviews/assemble.md`、
`docs/topics/v0-5-0-contract/tasks.md` 中 Task 1 matrix/status hunks，以及
`docs/status.md` 的 Contract Closure row hunk；排除
`docs/topics/work-signals/tasks.md`、Work Signals/Schema Version Signal status
hunks、`docs/topics/schema-version-signal/**` 与 final contract Task 的任何内容。
建议不构成提交授权。

推送建议：如后续明确授权并完成 signed commit、完整 message/trailer/tree/signature
检查，可从当前 `main` 推送到 `origin/main`；建议不构成推送授权。

### 下一步指令

开发：v0-5-0-contract / v0-5-0-contract
