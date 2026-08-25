---
status: active
topic: desktop-app
subject: architecture.md
---

# Review log — desktop-app / architecture.md

## Round 1 — 2026-08-19

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/architecture.md` blob
  `65e6db60777be6f9ea54d514b0b1218d79afeeee`.
- Reviewer: Codex
- Method: Single-agent formal design/contract Review. The current architecture
  was checked against the already-PASS requirements and settings/task contracts.
  CodeGraph supplied the current dedicated desktop snapshot command, wire
  version, Swift/Go types, and macOS update-check implementation paths. Broad
  review of later projection, switch, and distribution details stopped after
  the two decisive contract contradictions below.
- Scope: current `docs/topics/desktop-app/architecture.md`, with focused
  cross-checks against `requirements.md`, `ux/settings.md`, `tasks.md`, and the
  current `desktop snapshot` implementation.
- Findings:
  - [P1] A1-F1 — `architecture.md:54-57` still says the first implementation
    must decide whether existing JSON commands suffice or a dedicated desktop
    snapshot command is necessary. That decision is already made: the same
    document's helper contract at `:383-425` assumes a versioned desktop
    snapshot command, while `cmd/agentdeck/desktop.go:12-54` defines
    `agentdeck --format json desktop snapshot --wire-version 1`, and
    `internal/desktop/desktop.go:20-50` fixes wire version 1 and the snapshot
    shape. -> Replace the obsolete decision placeholder with the selected
    command, discriminator, version, recent-limit ownership, and Swift runner
    invocation boundary so the architecture has one implementable contract.
  - [P1] A1-F2 — `architecture.md:156-161` designs an optional Settings action
    that creates `~/.local/bin/agentdeck`. The approved boundary says the
    settings window exposes exactly four preferences (`requirements.md:170-176`),
    `ux/settings.md:45-54` explicitly excludes every path control, and
    `menubar-experience` delivers only those four settings. No task owns this
    fifth control or its filesystem mutation. -> Remove the Settings link action
    from this topic and keep CLI exposure in the already-authorized Formula/Cask
    installation paths; any future manual link feature needs a separately
    approved requirement, surface contract, task, and mutation/rollback design.
- Evidence: `git rev-parse HEAD` ->
  `58fe5d300c5af572adef81a69a856a6aef9cea56`; `git hash-object
  docs/topics/desktop-app/architecture.md` ->
  `65e6db60777be6f9ea54d514b0b1218d79afeeee`; repository contract comparison as
  above; CodeGraph current source -> `newDesktopCommand` exposes
  `desktop snapshot`, `desktop.WireVersion == 1`, and the versioned Swift
  decoder/projection path. CodeGraph also found the current unreviewed app still
  contains `DesktopUpdateChecker`; that is implementation drift owned by the
  downstream `menubar-experience` delivery, not a reason to weaken the correct
  architecture withdrawal. `bash scripts/check-topic-docs.sh`, `make
  check-whitespace`, and `git diff --check` -> exit 0.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：6/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:54`] A1-F1：desktop snapshot 的核心命令选择仍被写成待决定，
而同一文档后文和当前实现已经固定专用 wire-v1 command。
- 行为风险：实现者可以继续组合旧 JSON 命令、另建第二条命令，或依赖当前专用命令；
  三条路线的 discriminator、版本、错误和 redaction 边界不同，architecture 无法作为
  单一实现 authority。
- 证据：`:54-57` 要求未来决定；`:383-425` 已假定 versioned desktop snapshot；
  `cmd/agentdeck/desktop.go:12-54` 与 `internal/desktop/desktop.go:20-50` 已实现
  `desktop snapshot` wire v1。
💡 有界修复：把待决定段替换为已选择的 command、wire discriminator/version、
`recent-limit` 与 Swift runner/decoder 边界。

[`architecture.md:156`] A1-F2：Direct download 段仍要求 Settings 创建 CLI link，
违反已批准的四偏好 surface 边界。
- 行为风险：实现该段会增加第五个 Settings 控件和新的用户文件系统 mutation；不实现
  又会违反 architecture，且当前 task 分解没有 owner、测试或 rollback 边界。
- 证据：requirements `:170-176` 限定恰好四项；`ux/settings.md:45-54` 排除 path
  control；`menubar-experience` 只交付四项设置。
💡 有界修复：从本 topic 删除 Settings CLI-link action，继续由 Formula/Cask 暴露 CLI；
未来若重提，另建 requirement、UX、task 与安全 mutation contract。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- Update notification 段已经明确撤销所有自动/手动检查和网络请求，与当前 requirements
  一致。
- App/embedded helper/Widget 的进程与 sandbox 边界清楚，禁止 secrets 和 raw session
  content 进入 OSLog/App Group projection。
- Bundle 签名顺序、Cask/Formula 所有权冲突和保留用户状态的分发边界已有明确规则。

### 📝 摘要

评审对象为 HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56` 与
`architecture.md` blob `65e6db60777be6f9ea54d514b0b1218d79afeeee`。A1-F1 留下
已经完成的核心 wire 决定，A1-F2 又引入 requirements/UX/tasks 均未授权的第五个
Settings action，因此本轮 FAIL/REOPEN。出现两个决定性 blocker 后，没有继续审查后续
App Group projection、switch state machine 和完整 distribution matrix；这些不是本轮
PASS 证据，也没有被记为额外 finding。

#### 📌 下一步

```text
修复：desktop-app / reviews/architecture.md / A1-F1 A1-F2
```

## Round 2 — 2026-08-19（修复轮）

- Reviewed state: repair of Round 1's two open findings.
  `docs/topics/desktop-app/architecture.md` is now blob
  `4bbf2139528372c5f03116c5c2ac34a726aa0b1b`, from
  `65e6db60777be6f9ea54d514b0b1218d79afeeee`. HEAD is still
  `58fe5d300c5af572adef81a69a856a6aef9cea56`. No code, test, configuration, or
  fixture changed; this is a contract-document repair.
- Reviewer: claude-code (repair round for Round 1's FAIL — an independent
  Re-review is still required before the Document `Review` cell may be ticked;
  this round closes no gate and authorizes no commit)
- Scope: A1-F1 and A1-F2 as named in the repair command, plus the one
  same-topic contradiction A1-F2's removal exposed, recorded below rather than
  left for a later round to rediscover.

- Round 1 findings, dispositions:
  - **A1-F1** the already-made wire decision was still written as a future one ->
    **Fixed.** The placeholder is replaced by the selected contract, stated as a
    table so each element is checkable against the implementation rather than
    inferable from prose: the command, the required `--format json`, the
    `--wire-version` and `--recent-limit` flags, the `command` discriminator, the
    envelope `schema_version`, and the `data.wire_version`. Also recorded, because
    a rejected alternative is the part of a decision that stops it being reopened:
    composing existing JSON commands was rejected because the menu bar needs one
    snapshot taken at a single instant, and stitching several outputs together lets
    provider state, usage, and health disagree inside one refresh with no single
    discriminator or version to reject on.
    - The two-version distinction is now explicit, because it is the detail most
      likely to be collapsed by a reader: `schema_version` describes the envelope
      every JSON command shares, `wire_version` describes this snapshot's shape,
      changing one does not force the other, and the decoder checks both.
    - **Recent-limit ownership**, which the finding asked for by name: Go owns the
      value and rejects rather than clamps; the Swift runner owns rejecting an
      out-of-range value before launch so an impossible request never becomes a
      process; neither side substitutes a different number, so both encoding the
      same bound is a contract requirement and a divergence is a defect.
    - **Swift runner invocation boundary**: argument array, helper embedded in the
      app's own bundle, never one found on `PATH`, indexes refreshed through that
      same helper before the read-only snapshot. The section deliberately does not
      restate capture limits, timeouts, typed failures, or decoder rejection rules
      — those already have exactly one home each in **Helper execution contract**
      and **Wire validation**, and the section says so. The `--recent-limit`
      default and range are likewise pointed at rather than copied, for the same
      reason.
  - **A1-F2** an unauthorized fifth Settings control -> **Fixed by removal.** The
    optional `~/.local/bin/agentdeck` link action is gone from Direct download,
    replaced by a positive statement of what a direct-download install does expose
    (GUI and Widget) and where the command line comes from instead (the Formula and
    the Cask, whose ownership conflict and migration path this document already
    specifies). The reasons are kept so the removal is not read as an oversight: it
    would be a fifth preference in a window the approved boundary limits to four,
    and the only user-filesystem mutation in the desktop surfaces, owned by no task
    and therefore carrying no acceptance, test, or rollback design. The conditions
    a future version would have to satisfy — the Formula/Cask, legacy-script, and
    unknown-file cases it must never overwrite — are retained as the bar for
    reintroduction rather than deleted, so the next proposal starts from them.

- Same-topic contradiction the removal exposed, repaired in the same pass:
  `tasks.md:113` had task 6 `unified-desktop-distribution` verifying an "optional
  user CLI link". With the action removed from the contract, that line asked a task
  to verify a feature no document defines — the same class of defect as A1-F2, one
  file over. The item is removed and replaced by an explicit statement that there
  is no link to verify. This is flagged rather than done silently because it edits
  a document outside the reviewed subject: `tasks.md`'s own `Review` cell is
  unticked, so no passing verdict is disturbed, and `ux/menubar.md:63` already
  listed the link action as out of scope, so the edit moves `tasks.md` toward the
  surface contract rather than away from it. If the Re-review judges this outside
  the repair's authority, it is a one-line revert.

- Verification performed by this repair round, not yet independently confirmed.
  Every element of the new table was read back against the implementation, since a
  contract stated as fact is only worth as much as that check:
  - command, `--format json` refusal, and both flags —
    `cmd/agentdeck/desktop.go:13,18,23-25,50-51`;
  - `command` discriminator `desktop.snapshot` — `cmd/agentdeck/desktop.go:47`
    and `apps/macos/AgentDeckShared/DesktopWire.swift:27`;
  - envelope `schema_version` = 1 — `internal/output/output.go:6` and
    `DesktopWire.swift:26`; data `wire_version` — `internal/desktop/desktop.go:47`;
    the decoder checking both — `DesktopWire.swift:57-58`;
  - wire version rejected rather than defaulted — `internal/desktop/desktop.go:21,37-39`;
  - recent-limit bound encoded identically on both sides —
    `internal/desktop/desktop.go:22-23` (`5`, `20`) and
    `EmbeddedHelperRunner.swift:159-161` (`5`, `1`, `20`), with Swift's pre-launch
    rejection at `:193-195` and Go's non-clamping rejection in `Request.Validate`;
  - argument array, embedded helper, indexes-then-snapshot order —
    `EmbeddedHelperRunner.swift:201-213`.
  - Verification level L0: documentation only, no code, test, configuration, or
    fixture touched, so no product test was run and none is claimed.
    `make check-whitespace`, `bash scripts/check-topic-docs.sh`, and
    `git diff --check` all exit 0.
  - Not addressed by this round, and not claimed as passing: the App Group
    projection, the switch state machine, and the full distribution matrix. Round 1
    stopped before them after the two blockers, and this repair did not reach them
    either. Round 1's note that the current app still contains
    `DesktopUpdateChecker` also stands unchanged — it is implementation drift owned
    by `menubar-experience`, not an architecture defect, and this repair leaves the
    withdrawal as written.
- Evidence: `git hash-object docs/topics/desktop-app/architecture.md` ->
  `4bbf2139528372c5f03116c5c2ac34a726aa0b1b`; the implementation cross-checks
  enumerated above; `make check-whitespace`, `bash scripts/check-topic-docs.sh`,
  and `git diff --check` -> exit 0. Two documents changed,
  `architecture.md` and `tasks.md`; no other file.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No Document
  gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/architecture.md / Round 2
```

## Round 3 — 2026-08-19（独立复评）

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/architecture.md` blob
  `4bbf2139528372c5f03116c5c2ac34a726aa0b1b`.
- Reviewer: Codex
- Method: Single-agent formal independent Re-review of Round 2. A1-F1 and
  A1-F2 were checked against the current Go/Swift command and decoder paths,
  approved requirements/settings contracts, and task decomposition. Because
  Round 1 stopped after the blockers, this round resumed review across the
  previously unreviewed Foundation, App Group projection, additive presentation
  schema, provider switch transport/operation ownership, and distribution
  boundaries. CodeGraph supplied current source and affected tests for the
  snapshot and switch paths.
- Scope: A1-F1/A1-F2 dispositions, their `tasks.md` alignment hunk, and the
  remainder of current `architecture.md` that Round 1 explicitly left
  unreviewed.
- Findings:
  - [P1] A1-F1 — **CLOSED.** The wire section now fixes one dedicated
    `agentdeck --format json desktop snapshot` contract, rejects the composed
    multi-command alternative, distinguishes envelope schema version from data
    wire version, and assigns recent-limit validation plus the embedded-helper
    Swift invocation boundary. Current `newDesktopCommand`, `desktop.Request`,
    `EmbeddedHelperRunner`, fixtures, and decoder tests match the table.
  - [P1] A1-F2 — **CLOSED.** Direct download now explicitly ships no CLI-link
    action, leaves command exposure to Formula/Cask, records why a fifth
    Settings preference and filesystem mutation are out of scope, and preserves
    the requirements for any future separately approved proposal. Task 6 now
    states that there is no link to verify.
  - Newly blocking findings: none.
- Evidence: current architecture blob as above; contract/source cross-check ->
  `cmd/agentdeck/desktop.go:12-54`, `internal/desktop/desktop.go:20-50`,
  `EmbeddedHelperRunner.swift` limits/argument array, shared Swift decoder and
  canonical tests agree with the selected wire table; requirements/settings/task
  comparison -> exactly four preferences and no path control. Resumed-scope
  review -> Widget Data requirements map to the App Group allowlist and bounded
  `usage.presentation` schema; the three menu-bar data gaps are explicitly
  provisioned or refused; provider switch uses the complete immutable option,
  canonical stream matrix, globally single-flight controller, and current
  retry/dismiss tests. The current app's `DesktopUpdateChecker` remains
  downstream implementation drift and is not evidence against the correct
  architecture withdrawal. `bash scripts/check-topic-docs.sh`, `make
  check-whitespace`, and `git diff --check` -> exit 0. CEv1 content state
  `d49e19279649005a0f05b21381b3c65a2722eac6f2aab8778aa0f5ea396f67aa`
  -> one required criterion, current passing evidence, no invalidation or
  unresolved impact, `VERIFIED`.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。A1-F1、A1-F2 均已关闭，恢复审查的其余范围没有新 blocker。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- Desktop wire 的 command、两层 version、limit ownership 和 Swift invocation
  现在形成单一可实现合同，且记录了被拒绝的组合命令路线。
- Direct-download/Settings/Formula/Cask 的所有权重新一致，不再存在无 task owner 的
  用户文件系统 mutation。
- App Group projection 与 Widget Data requirements 的 scope、bounds 和 privacy
  exclusions 对齐；menu-bar 的 period/client product 由 Go producer 计算。
- Provider switch 的 transport total classification、完整 option retention 和
  globally single-flight lifecycle 与当前测试路径一致。

### 📝 摘要

A1-F1、A1-F2 是 Round 1 的全部 findings，本轮已在 HEAD
`58fe5d300c5af572adef81a69a856a6aef9cea56` 与 architecture blob
`4bbf2139528372c5f03116c5c2ac34a726aa0b1b` 上独立确认关闭。Round 1 留下的
未审范围也已恢复检查，没有新阻断项，因此结论为 PASS。当前应用仍包含 update-check
实现是后续 implementation review/repair 风险，不改变本 architecture 的“无更新检查”
正确边界。

#### Task checkpoint

Task checkpoint：`desktop-app:architecture.md`（desktop-app architecture 文档交付）；
上述 exact content state 的 CEv1 门槛为 `VERIFIED`。

提交建议：以 architecture 文档交付为原子边界，候选范围包括 `architecture.md`、
A1-F2 对应的 task 6 单行对齐 hunk、本评审记录，以及 `tasks.md`/`docs/README.md` 的
本任务状态 hunk；排除产品代码、未评审 `tasks.md` 的其他内容和其他脏工作树。

推送建议：本轮不执行推送；若后续单独授权提交与推送，先确认 `main` 到
`origin/main` 的精确范围、暂存边界、commit 正文/共同创作者尾注/SSH 签名和 Hook
效果，再只推送该文档 Task 的 commit。

#### 📌 下一步

```text
评审：desktop-app / reviews/tasks.md
```

## Document review deferred to the closing pass (2026-08-20)

By user instruction, `desktop-app` runs no document review rounds while its tasks
are being implemented. Review is **deferred, not cancelled**: after every
implementation task is done, the whole document set is reconciled against the
final prototype and the shipped implementation and reviewed once, as a bullet on
task 6.

Until then, changes to this subject are written directly into the document that
owns it, and nothing is appended here. The closing pass appends its round to this
record. The reason and the two consequences are stated in
[`../tasks.md`](../tasks.md).

**Status 2026-08-23 — reconciled and submitted; the closing round has not run.**
Task 6 has brought the set into agreement with the shipped implementation, so this
record is now waiting for the single deferred review rather than waiting for
implementation to finish. The next thing appended below is that round.

## Closing document review — Round 1 (2026-08-23)

This is the single deferred document review this topic postponed on 2026-08-20.
One round, one verdict, over the whole set; this record carries the round's
common part plus this document's own outcome.

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint
  `5d5c576eeb75117bd6b329d807bb018c5dec7cea12b213a1c603256d3f5345e3`, computed as
  `shasum -a 256` over one `<git hash-object> <space> <line count> <space> <path>`
  line per document, in this order: `requirements.md`, `architecture.md`,
  `ux/menubar.md`, `ux/settings.md`, `ux/widget.md`, `tasks.md`. The order is part
  of the recipe.
- Reviewer: Claude Code, independent of every implementing and reconciling
  session.
- Method: the set judged against the shipped implementation rather than against
  itself, which is the reason the review was deferred to this point. Checkable
  claims were taken to the code: the settings defaults and the login-item refusal
  path to `DesktopPreferences.swift` and `SettingsWindowView.swift`; the four
  panels and the Breakdown cards' non-collapsibility to `MenuBarPanelViews.swift`;
  the `Not captured yet` modules to `DesktopCopy.swift`; the fixed 30-day rhythm
  window across `requirements.md`, `ux/menubar.md` and `internal/usage`; the
  widget's runtime state to `desktop-widget` Round 3. `scripts/check-topic-docs.sh`
  is the project's checker for this target class and was run.
- Set-wide verdict: **REOPEN**. Three findings, all located in two documents;
  four of the six documents carry none.
- This document's outcome: **PASS**, no findings.
  - Checked and correct: the App Group identifier section now sizes DW-R3-F1
    honestly, with a table of every site carrying the literal and the reason the
    fix is not configuration alone; the update-channel metadata claim is gone and
    the absence is explained rather than left blank; the Homebrew channel bullets
    match the delivered Cask, including that `conflicts_with` accepts `cask:` alone
    and the formula exclusion is the `preflight` refusal; the Direct download
    section declares both artifacts and the stapling contract; the release-secret
    table matches `.github/workflows/release.yml`; the `rhythm` wire family and its
    fixed 168-value ordering match `internal/usage/presentation.go`.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class; a verdict on set completeness must cite the tool that can
    falsify it)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Implementation cross-checks named under Method, each run against the working
    tree at this content state
- Residual, carried and owned elsewhere: task 4 `desktop-widget` remains at an
  open P1 (DW-R3-F1) parked on an Apple Developer team ID, so this set describes a
  surface whose runtime acceptance never passed; task 6's basis remains the user's
  explicit decision to proceed without task 4, recorded only by the implementing
  agent.
- Verdict: REOPEN

## Closing document review — Round 3 (independent re-review, 2026-08-23)

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint, computed with Round 1's six-document recipe
  **after this round's own status synchronization**, so the evidence binds to the
  state the verdict leaves behind rather than to the pre-tick one:
  `c42a6de1d707e6d79320032877996ad8aec10847cf58a45885eb3e63fa3bd2a7`.
  Round 2's repair candidate was `771b3375…cd10`; Round 1 was `5d5c576e…45e3`.
  Two of the six documents changed between Rounds 1 and 2 — `requirements.md` and
  `tasks.md` — which is exactly Round 2's declared scope; the four documents that
  passed Round 1 are byte-identical to their Round 1 blobs.
- Reviewer: Claude Code, independent of the repair round.
- Method: each finding re-verified against the documents themselves rather than
  against Round 2's account of the repair, and the four already-passing documents
  re-hashed to confirm the repair did not touch them.
- Disposition of every Round 1 finding:
  - CD1-F1 — **closed.** `requirements.md`'s Widget acceptance bullet now carries
    a `Known defect:` paragraph stating that the criterion is not met by the
    shipped build, naming the missing team-ID prefix, macOS 26's refusal, all
    twelve configurations rendering the unavailable state, and `DW-R3-F1` with its
    parked Apple Developer prerequisite — and it closes with "The criterion above
    remains the contract", which is the half that mattered. The disclosure now
    matches the shape the other four documents already used, so the set has one
    voice on this defect instead of two.
  - CD1-F2 — **closed.** The temporary code-over-contract rule now states why it
    survives task 6's initial reconciliation — task 4's open `DW-R3-F1` leaves the
    implementation and the document set in disagreement — and assigns the removal:
    task 6 owns a final reconciliation after task 4 closes that finding, before
    task 6 can reach Review PASS. A reader can now tell a deferral from an
    omission, which is what the finding asked for. Declaring the retention
    deliberate is the owner's decision to make, not a claim this reviewer can
    falsify, and the substantive requirement — the condition and the owner — is
    met.
  - CD1-F3 — **closed, and the repair is the stronger of the two available
    options.** The Documents matrix gained a dated `Closing review` column, and
    the deferral section was updated to say which column means what: the old
    `Review` cells "retain the suspended historical values from before deferral"
    and "do not report the closing pass", while the dated column "is the
    closing-pass result". The manual-acceptance row is marked `—` as outside the
    six-document set, which is correct — Round 1 reviewed six documents and that
    was not one of them. The closing pass now has falsifiable cells, and this
    round moved the two that were `[ ]`.
- New findings: none. The four documents that passed Round 1 are unchanged, and
  the two repaired documents introduced no defect this round could find.
- Status synchronization performed by this round, recorded here because it
  changed the fingerprint the evidence binds to: `requirements.md` and `tasks.md`
  moved to `[x]` in the `Closing review` column, and `tasks.md`'s next-action
  statement was corrected — it still directed the reader to run this review and to
  re-review task 6's record, both of which had already run.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Per-document blob comparison against Round 1 → only `requirements.md` and
    `tasks.md` changed
- What this verdict does and does not unblock: it closes task 6's R1-F1, whose
  whole content was that this review had not run. It does not let task 6 pass.
  The rule repaired under CD1-F2 now makes that explicit — task 4 must close
  `DW-R3-F1` before task 6 runs the final reconciliation that removes the rule —
  and `DW-R3-F1` is parked on an Apple Developer team ID that does not exist yet.
  That is where the topic stops, and it is an external prerequisite rather than
  work anyone here can perform.
- Set-wide verdict: **PASS**

### Round 3 addendum — completion-evidence bookkeeping (2026-08-23)

Recorded after the round, because the recording itself needed a correction and
hiding it would defeat the purpose of an audit trail.

- The six `document` boundaries this round crossed are now recorded and the gate
  answers `VERIFIED` for each, bound to content state
  `urn:ce:agent-deck:state:candidate:a5e3433a…f167` (HEAD `a190186` plus the
  post-synchronization set fingerprint `c42a6de1…d2a7`).
- **A mistake, disclosed rather than quietly fixed.** The first write used
  `MERGE` on work-unit ids that already existed under the store's *retired*
  capitalized `WorkUnit` vocabulary for four of the six documents —
  `architecture.md`, `ux/menubar.md`, `ux/widget.md` and `tasks.md`. Two things
  followed. The gate could not see them, because it filters on the current
  lowercase `work_unit` kind, so those four read `NOT_VERIFIED` while looking
  recorded. And an unconditional `SET` overwrote each of those historical nodes'
  `target_content_state` with this round's value; the prior values were not
  captured first and are **not** restored here, because guessing them from their
  latest evidence would put an invented fact into the record. `head` and
  `attributes_json` were added to those nodes rather than overwritten, the old
  vocabulary carrying neither.
- The forward fix creates four new current-model `work_unit` nodes under distinct
  ids ending `-doc`, pointing at the criteria this round already wrote, and
  leaves the historical nodes as history. `.agent-instructions/evidence.md` says
  to leave retired-vocabulary nodes as they are; it also says to inspect the store
  before writing, which is the step that was skipped.
- One pre-existing row still reads `NOT_VERIFIED`: `ux/settings.md`'s older
  `independent-review-pass` criterion, whose evidence binds to a content state
  from before the deferral. It is a leftover of the earlier process, not of this
  round, and is left for whoever reconciles that boundary.
