---
status: historical
retired: 2026-09-04
topic: v0-5-0-contract
subject: v0-5-0-contract
---

# Review log — v0-5-0-contract / v0-5-0-contract

## Round 1 — 2026-09-01

- Reviewed state:
  - HEAD `96fb6c77ca5f2d47abea0e516a9b5578ac4e1c11`, working tree and
    index uncommitted
  - Development candidate
    `urn:ce:agent-deck:state:candidate:b4e593b784b484524fc4c290f2c8138889011a654ad49f27e104d121b56fc383`
  - Candidate manifest
    `bd5fc272f8b544671bde3b2557225d6ef28b339a4e58d8e9a042f92e174b0bdb`
- Reviewer: Codex, independently reviewing Claude Code's final contract
  Development candidate; production code, tests, configuration, archive
  contents, Git index, and Git history remained read-only.
- Method: contract/delivery review against the five registered CEv1 criteria,
  ratified Task 2 Files ownership, the combined/staged/unstaged diff, the
  version/specification changes, archive topology, Beads authorization record,
  and Development evidence. Broad link and L2 verification stopped after the
  decisive scoped-delivery contradiction; the existing exact-candidate evidence
  remains available for the later repaired state.
- Scope: single version raise and compatibility notice, specification
  reconciliation, entry condition and identity wiring, retirement of the five
  selected topics, consumer-link updates, task/status synchronization, and the
  boundary between explicitly authorized expansion and ratified Task Files.

## 📋 v0.5.0 Contract Closure final task 评审报告

📊 总体评分：6/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

[`docs/topics/v0-5-0-contract/tasks.md:196`、
`docs/topics/v0-5-0-contract/tasks.md:301`]
**[R1-F1] Final Task 的 ratified Files、实际 change set 与 CEv1
scoped-delivery criterion 三者不一致。**

- 行为风险：用户的显式授权允许 archive consumer 修复，但没有自动改写 Task 2 的
  权威 Files boundary。实际 candidate 修改了未列出的 `docs/roadmap.md`、
  `docs/topics/schema-version-signal/requirements.md`、`internal/usage/routes.go`
  和 `docs/topics/v0-5-0-contract/reviews/assemble.md`；tasks.md 的 Development
  summary 只列前三类，还漏掉 assemble review。后续 commit checkpoint 无法仅从
  plan 推导完整 scope，并可能把另一个未评审 topic 或 production file 当成原 Task
  固有内容。更直接地，CEv1 criterion 仍写 “Only this task's ratified Files are
  changed”，其 pass evidence 却承认 change set exceeds that list，因此
  `VERIFIED 5/5` 对当前 candidate 是自相矛盾的。
- 证据：`git diff HEAD --name-status` 返回上述四个非 ratified paths；Task 2 Files
  bullet 未列出它们；candidate `b4e593b7...` 的 scoped-delivery evidence 明文写
  “The change set exceeds the task's originally ratified Files list” 和 “gap worth
  a finding”，但 outcome 仍为小写 `pass`。显式用户授权记录在 Beads 和 Development
  summary 中，因此 finding 针对的是未同步的 contract/evidence boundary，不是对
  已授权编辑本身的异议。
💡 有界修复：只更新 Task 2 的 Files/hunk ownership 与 Development summary，完整
列出这四个已授权 consumer paths，说明 `internal/usage/routes.go` 仅改文档路径注释、
schema-version-signal 只改具名 link hunks、assemble review 只改五个 archive links，
并保留 pre-existing work-signals/tasks.md hunk 的独立归属。随后为修正后的精确
candidate 写入新的 scoped-delivery evidence 并 supersede 当前 pass；不得改写既有
CEv1 节点、扩大到其他内容、或执行 commit/push/release。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `cli-design.md` 的 diff 清楚显示 version 27 → 28、一个新 Changelog row，以及
  Error-Code Compatibility 段落完整列出旧 `runtime_error`、五个替代 code、条件和
  不变的 exit code。
- Archive candidate 使用 62 个 `git mv`，保留各 topic 的 review history；
  archive index 继续区分 switch-effectiveness 的 operator waiver 与 Review PASS。
- Development 在遇到 consumer gap 时先停下，并在获得用户显式授权后才完成额外
  link/comment 修复；scope expansion 与 pre-existing work-signals hunk 都没有被
  静默隐藏。
- L2、whitespace、diff 与 topic-doc evidence 已绑定 Development candidate；当前
  blocker 是 contract/evidence truth，而不是已发现的产品行为回归。

### 📝 总结

被评状态为 HEAD `96fb6c7...` 加 candidate `b4e593b7...` / manifest
`bd5fc272...`。版本合同与 archive 工作有充分实质进展，但 Task authority 仍不包含
四个已授权 consumer paths，且 CEv1 scoped-delivery pass 与自己的 criterion 直接
矛盾。该缺陷阻断可审计的 Task completion 与 commit scope，因此本轮判定 FAIL。
出现决定性 reproducer 后未继续扩大链接/L2 检查；三类 tag-derived identity 仍按
任务定义等待另行授权的 `v0.5.0` tag，不是本 finding 的一部分。

### Evidence

```text
git diff HEAD --name-status
  -> 62 archive renames plus final-task files and four authorized consumers
Task 2 Files list
  -> does not name roadmap.md, schema-version-signal/requirements.md,
     internal/usage/routes.go, or reviews/assemble.md
candidate b4e593b7 scoped-delivery evidence
  -> admits change set exceeds ratified Files and calls it a finding gap,
     but records outcome pass against an only-ratified-Files criterion
git diff HEAD -- docs/specs/cli-design.md
  -> one version raise, one Changelog row, one compatibility section
git diff --cached --stat / git diff --stat
  -> 62 base renames staged; content/link/frontmatter changes remain unstaged
```

- Completion gate: FAILED — exact target state
  `urn:ce:agent-deck:state:candidate:719f0f693062119f1d375ea501a5992f04225034ca6b73f3cbf3056c93ecdea1`
  has 4/5 required criteria passing and one disproving R1-F1 evidence record.
- Verdict: REOPEN

### 下一步指令

修复：v0-5-0-contract / reviews/v0-5-0-contract.md / R1-F1

## Repair Round 1 — 2026-09-01

- Repaired state: HEAD `96fb6c77ca5f2d47abea0e516a9b5578ac4e1c11`, working tree
  and index uncommitted. Only `tasks.md` and this record changed; the 62 archive
  renames, every re-stamped document, and every specification blob are untouched.
- Repairer: Claude Code. No product code, test, or configuration was touched in
  this round, no existing CEv1 node was rewritten, and nothing was staged beyond
  what `git mv` had already staged.
- Scope: Task 2's `Files` / hunk ownership and its Development summary, plus one
  new scoped-delivery evidence record at the corrected candidate.

### Finding disposition

| Finding | Disposition | Change |
| --- | --- | --- |
| R1-F1 | Closed | The ratified `Files` list now names all four consumers; new evidence supersedes the contradictory pass |

**[R1-F1] The contract, the change set, and the evidence disagreed.** The finding
is right, and the sharpest part of it is the one that is easy to miss: the CEv1
criterion reads *"Only this task's ratified Files are changed"*, while the
evidence recorded against it opened by saying the change set exceeds that list.
A `pass` whose own text contradicts its criterion is worse than a missing record,
because the gate reports `VERIFIED` while the thing it verified is false. The
fourth path, `reviews/assemble.md`, was absent from the Development summary's own
gap list as well, so even the disclosure was incomplete.

The repair fixes the contract rather than the wording of the evidence, which is
what the bounded remediation asked for. Task 2 gains a **Consumers of the moved
paths** bullet stating why they belong here at all — the side that moves a path
updates its citations, the same rule `.agent-instructions/branching.md` states
for a contract change — and then naming each with its exact hunk scope:

- `docs/roadmap.md` — the one `usage-attribution-precision` link.
- `docs/topics/schema-version-signal/requirements.md` — the two named
  `cli-error-classification` links only; the rest of that in-flight, unreviewed
  topic is untouched, and `tasks.md` in it was verified unmodified.
- `internal/usage/routes.go` — the two documentation-path comments only. The
  diff is `+2 -2`, both lines comments; no Go statement, signature, or test
  changed.
- `reviews/assemble.md` — the five archived-topic links in its ancestry table
  only; no round, verdict, finding, or disposition text changed.

A second bullet records what is deliberately **not** this task's: the
pre-existing uncommitted hunk in `work-signals/tasks.md`, authored by the closed
work-signals task 6 delivery sync, which travels into the archive with its
document and keeps its own authorship. The Development summary now lists all four
consumers instead of three, and no longer describes the boundary as a gap,
because the boundary now matches the delivery.

With the list corrected, the criterion is satisfiable as written: the four paths
are ratified `Files`, so "only ratified Files are changed" is true of this
candidate rather than admitted-false. New scoped-delivery evidence is recorded at
the corrected candidate and supersedes the contradictory record; the superseded
node is left exactly as it was, because it is what makes the correction auditable.

### Verification

```text
git status --porcelain | grep -v 'docs/archive/topics/'
  -> docs/README.md, docs/archive/README.md, docs/roadmap.md,
     docs/specs/cli-design.md, docs/status.md,
     docs/topics/v0-5-0-contract/reviews/assemble.md,
     docs/topics/v0-5-0-contract/tasks.md, internal/usage/routes.go,
     the untracked schema-version-signal topic, and this record.
     Every entry is now named by the ratified Files list.
git diff --unified=0 internal/usage/routes.go
  -> +2 -2, both changed lines are `//` comments carrying a document path
schema-version-signal file survey
  -> requirements.md edited; tasks.md verified unmodified
make check-whitespace / git diff --check
  -> PASS (exit 0) / PASS
```

L2 stays reusable from the Development round: this repair changed two Markdown
documents and no product surface, so `scripts/run-go-test.sh ./...` is not re-run.

- Completion gate: re-recorded in this round. New scoped-delivery evidence at the
  corrected candidate supersedes the contradictory pass at `b4e593b7…`; Round 1's
  `FAILED` result at `719f0f69…` stands as immutable evidence against that
  superseded state.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

### 下一步指令

复评：v0-5-0-contract / reviews/v0-5-0-contract.md

## Round 2 — 2026-09-01

- Reviewed state:
  - HEAD `96fb6c77ca5f2d47abea0e516a9b5578ac4e1c11`, working tree and
    index uncommitted
  - Repaired candidate
    `urn:ce:agent-deck:state:candidate:abb3e501f3e1151f6fd1990ef944886b5d627f64fcfe05052d65e347163fd31d`
  - Candidate manifest
    `642a2a2dce9e6e152e7afdd0894501c62d518862674271d9354690d33b390bda`
- Reviewer: Codex, independently re-reviewing Claude Code's Repair Round 1;
  production code, tests, configuration, archive contents, the untracked
  schema-version-signal topic, Git index, and Git history remained read-only.
- Method: finding-disposition Re-review against the repaired Task 2 Files list,
  actual Git object state, scoped-delivery evidence, and exact commit-boundary
  feasibility. Unchanged specification, archive, lifecycle, identity, and L2
  evidence was reused.
- Scope: R1-F1 closure, the four newly ratified consumer paths, pre-existing
  dirty-work attribution, and regressions introduced by the bounded Repair.

## 📋 v0.5.0 Contract Closure final task 复评报告

📊 总体评分：7/10

✅ 判定：FAIL

### 🔴 严重问题 — 必须修复

[`docs/topics/v0-5-0-contract/tasks.md:213`]
**[R2-F1] Task 2 把 untracked requirements file 描述成两个可独立提交的 link
hunks，但 Git 没有该文件的 baseline。**

- 处置：新增 blocking finding；R1-F1 本身已关闭。
- 行为风险：`docs/topics/schema-version-signal/requirements.md` 不存在于 HEAD，
  整个 schema-version-signal topic 仍为 untracked、unreviewed work。Task 2 的
  Consumers bullet 声称只拥有其中两个 link hunks、其余内容属于另一个 Task，但 Git
  无法对新增文件只提交两个 hunks：stage 该 path 会把整份未评审 requirements 一起
  加入 final-contract commit；不 stage 则该 consumer edit 没有随本 Task 交付。
  Repair 后的 scoped-delivery pass 因此仍不能证明可隔离的 commit boundary。
- 证据：`git cat-file -e HEAD:docs/topics/schema-version-signal/requirements.md`
  失败并明确报告 path 不在 HEAD；`git status --short
  docs/topics/schema-version-signal` 只返回整个目录 `??`。这不是授权问题——用户已
  授权编辑——而是未跟踪文件不存在可分离 hunk 的 Git 原子性问题。
💡 有界修复：从 Task 2 的 owned Consumers 与 candidate manifest 中移除该
requirements file，把当前两处正确 link edit 保留在工作树并明确归还
schema-version-signal Task，标为 “not this task and not to be staged”，与既有
work-signals hunk 采用同一归属规则。其余三个 authorized consumers 保持 Task 2
scope；为新 candidate 重写 scoped-delivery evidence 并 supersede `abb3e501...`，
不得丢弃或改写 untracked topic、archive/spec 内容或历史 evidence。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R1-F1 已关闭：Task 2 Files 与 Development summary 现在完整列出 roadmap、
  routes comments 和 assemble review，新的 evidence 也 supersede 了自相矛盾的
  pass。
- `internal/usage/routes.go` 精确限定为两个注释行，assemble review 限定为五个
  ancestry links；这三类 tracked consumer hunks 具备可交付的 Git baseline。
- Pre-existing work-signals hunk 被明确保留原 Task 归属，Repair 未修改 archive、
  specifications 或产品行为，其他四条 criterion evidence 可复用。

### 📝 总结

R1-F1 的 contract/evidence 矛盾已修正，但 Repair 将另一个 topic 的 untracked
requirements 当成可按 hunk 交付的 tracked consumer，留下新的 R2-F1。被复评状态为
HEAD `96fb6c7...` 加 candidate `abb3e501...` / manifest `642a2a2d...`；该缺陷会迫使
commit checkpoint 在“吸收整份未评审文件”和“漏交付 link edit”之间二选一，因此
本轮判定 FAIL，Task 与 Topic boundary 均保持开放。

### Evidence

```text
git status --short docs/topics/schema-version-signal
  -> ?? docs/topics/schema-version-signal/
git cat-file -e HEAD:docs/topics/schema-version-signal/requirements.md
  -> fatal: path exists on disk, but not in HEAD
Task 2 Consumers bullet
  -> claims ownership of two link hunks while the rest of the file is another
     unreviewed task's work
repaired scoped-delivery evidence
  -> records pass at abb3e501 but cannot make an untracked file hunk-isolatable
```

- Completion gate: FAILED — exact target state
  `urn:ce:agent-deck:state:candidate:fbf7820fce0f55f53a1b8724af52d0225811065578ed84da0352e974c6f3f178`
  has 4/5 required criteria passing and one disproving R2-F1 evidence record.
- Verdict: REOPEN

### 下一步指令

修复：v0-5-0-contract / reviews/v0-5-0-contract.md / R2-F1

## Repair Round 2 — 2026-09-01

- Repaired state: HEAD `96fb6c77ca5f2d47abea0e516a9b5578ac4e1c11`, working tree
  and index uncommitted. Only `tasks.md` and this record changed. The two link
  corrections in `docs/topics/schema-version-signal/requirements.md` are
  deliberately **left in place and left unstaged**, per the bounded remediation;
  nothing was reverted.
- Repairer: Claude Code. No archived document, specification, product file, or
  historical CEv1 node was touched.
- Scope: Task 2's Consumers ownership and its Development summary, plus new
  scoped-delivery evidence at the corrected candidate.

### Finding disposition

| Finding | Disposition | Change |
| --- | --- | --- |
| R2-F1 | Closed | The untracked consumer returns to its owning task; three tracked consumers stay |

**[R2-F1] An untracked file has no hunk to own.** The finding reproduces exactly:

```text
git cat-file -e HEAD:docs/topics/schema-version-signal/requirements.md
  -> fatal: path 'docs/topics/schema-version-signal/requirements.md'
     exists on disk, but not in 'HEAD'
git status --short docs/topics/schema-version-signal
  -> ?? docs/topics/schema-version-signal/
```

The previous repair treated all four consumers as one kind of thing. They are
not. Git can stage part of a *modification*, which is what makes "these two
comment lines and nothing else" a real boundary for `internal/usage/routes.go`.
It cannot stage part of an *addition*: a file absent from `HEAD` enters a commit
whole or not at all. So Task 2's claim to own two link hunks inside that file was
not a scope statement Git could honour — the commit checkpoint would have had to
choose between absorbing an entire unreviewed requirements document into the
final-contract commit and dropping the link fix. Neither is a boundary this task
can hold, and an evidence record asserting an isolable scope over it could not be
true however carefully it was worded.

The repair keeps the three tracked consumers — `docs/roadmap.md`,
`internal/usage/routes.go`, `reviews/assemble.md` — and states the reason they
qualify: each is tracked at the commit baseline. The fourth moves into the "not
this task's" bullet beside the pre-existing `work-signals/tasks.md` hunk, under
one stated rule: **an edit this task made does not become this task's to deliver
when the file belongs to someone else.** The edits stay in the working tree,
correct and unstaged, and belong to the `schema-version-signal` task that first
commits that file. The authorization to make the edit was real; the
deliverability is what is not this task's.

The Development summary now says three deliverable consumers and one returned,
instead of four.

### Verification

```text
git cat-file -e HEAD:docs/roadmap.md
git cat-file -e HEAD:internal/usage/routes.go
git cat-file -e HEAD:docs/topics/v0-5-0-contract/reviews/assemble.md
  -> all three succeed; every retained consumer has a commit baseline
git cat-file -e HEAD:docs/topics/schema-version-signal/requirements.md
  -> fails; the returned consumer does not, which is the finding
git status --short docs/topics/schema-version-signal
  -> still a single `??` for the whole topic: the link edits survive, unstaged
make check-whitespace / git diff --check
  -> PASS (exit 0) / PASS
```

L2 stays reusable: this round changed two Markdown documents and no product
surface, so `scripts/run-go-test.sh ./...` is not re-run.

- Completion gate: re-recorded. New scoped-delivery evidence at the corrected
  candidate supersedes the `abb3e501…` pass; Round 2's `FAILED` result at
  `fbf7820f…` stands as immutable evidence against that superseded state, and no
  earlier node was rewritten.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

### 下一步指令

复评：v0-5-0-contract / reviews/v0-5-0-contract.md

## Round 3 — 2026-09-01

- Reviewed state:
  - HEAD `96fb6c77ca5f2d47abea0e516a9b5578ac4e1c11`, working tree and
    index uncommitted
  - Repaired candidate
    `urn:ce:agent-deck:state:candidate:5004e922c5ed529b51f6acc7ce8e3db071ee57964d37988ff877b91a9504eaf4`
  - Candidate manifest
    `1494c4e59f098f69b249cbde7c58c7d52c90e6d8ec0d666e764b284fd974cb1a`
- Reviewer: Codex, independently re-reviewing Claude Code's Repair Round 2;
  production code, tests, configuration, archive contents, untracked
  schema-version-signal work, Git index, and Git history remained read-only.
- Method: finding-disposition Re-review against HEAD object existence, Task 2
  Files ownership, candidate manifest scope, supersedes relations, and the
  unchanged four criterion evidence sets.
- Scope: R2-F1 closure, R1-F1 regression check, the three retained tracked
  consumers, the returned untracked consumer, dirty-work attribution, and the
  final Task/Topic boundary.

## 📋 v0.5.0 Contract Closure final task 复评报告

📊 总体评分：10/10

✅ 判定：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- R2-F1 已关闭：Task 2 仅保留三个在 HEAD 中有 baseline 的 consumers；untracked
  schema requirements 被明确归还其自身 Task，正确 link edits 保留在工作树但不进入
  final-contract manifest 或 commit scope。
- R1-F1 保持关闭：Task authority、actual change set 与 scoped-delivery evidence
  现在一致，`5004e922...` 的五条 pass 均 supersede 上一候选且没有改写历史。
- Single raise/compatibility、spec reconciliation、archive lifecycle 与
  entry/identity evidence 均未被两轮 scope repair 改动，可按 exact content state
  复用。
- Pre-existing work-signals hunk 与 schema-version-signal topic 都保留原 Task
  归属，final Task checkpoint 可以从 ratified Files 推导可隔离的 staging scope。

### 📝 总结

R1-F1 与 R2-F1 均已关闭，没有回归或新增 finding。被复评状态为 HEAD
`96fb6c7...` 加 candidate `5004e922...` / manifest `1494c4e5...`；final Task 的
五条 completion criteria 均满足，Task Review cell 已勾选。该 Task 是 containing
topic 的最后一项，因此还需在本轮建立并查询 Topic WorkUnit，只有 Topic gate
`VERIFIED` 才形成 Unit completion checkpoint。

### Evidence

```text
git cat-file -e HEAD:<tracked-consumer>
  -> roadmap.md, internal/usage/routes.go, reviews/assemble.md all succeed
git cat-file -e HEAD:docs/topics/schema-version-signal/requirements.md
  -> fails; the file remains outside this Task
git status --short docs/topics/schema-version-signal
  -> ?? docs/topics/schema-version-signal/; edits retained and unstaged
candidate 5004e922 content state
  -> 71-file manifest explicitly excludes the untracked topic
CEv1 supersedes query
  -> all five repair-r2 evidence records supersede abb3e501
make check-whitespace / git diff --check
  -> PASS / PASS
```

- Completion gate: VERIFIED — exact Task target state
  `urn:ce:agent-deck:state:candidate:3308139a5842db9fe4d6cb67a0b4eda044c60f9ba0d0d97b954d4bc5e102e08a`
  has passing evidence for all 5/5 required criteria.
- Verdict: PASS

### Task checkpoint

Task checkpoint：Task `ad-v050c-contract-dev` / WorkUnit
`v0-5-0-contract:v0-5-0-contract`，内容状态为 HEAD `96fb6c7...` 加 candidate
`3308139a...`，CEv1 gate `VERIFIED` 5/5。

提交建议：提交 final Task ratified boundary，包括 62 个 archive renames 及其
historical/retired/link hunks、CLI Design v28、archive/docs/status/tasks/review
artifacts，以及三个 tracked consumers（roadmap 单一 link、routes.go 两个注释、
assemble review 五个 links）。明确排除整个 untracked schema-version-signal topic，
并按 hunk 保留 archived work-signals/tasks.md 中属于已关闭 Task 6 的 pre-existing
改动及其原作者归属。建议不构成提交授权。

推送建议：仅在后续明确授权、signed commit 对象及 immutable Task/Topic gates 均
验证后，才从 `main` 推送到 `origin/main`；建议不构成推送授权。

### Topic gate prerequisite

Containing WorkUnit `v0-5-0-contract` 已建立，状态为 `in_progress`，三条 required
criteria 已注册，但 `target_content_state` 尚不存在。精确 prerequisite 是当前 final
Task 的 separately authorized signed commit；它产生 immutable Topic Git tree 后，
才能重绑定 final Task evidence、写入 Topic roll-up evidence 并查询 Unit gate。当前不
形成 Unit completion checkpoint。

### 下一步指令

提交
