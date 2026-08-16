---
status: active
topic: desktop-app
subject: menubar-experience
---

# Review log — desktop-app / menubar-experience

> **Path note (2026-08-16).** Rounds 1–3 reviewed
> `docs/specs/menubar-experience-design.md`, which the topic migration split into
> two documents. Its wire-contract extension — `provider.candidates`, the switch
> command surface, the result envelope, and switch operation ownership — is now
> `docs/topics/desktop-app/architecture.md`, and its presentation, copy,
> localization, and accessibility contract is now
> `docs/topics/desktop-app/ux/menubar.md`. The paths and line numbers cited in
> the rounds below identify the state that was reviewed and are left unchanged.
> Of the six open Round 3 findings, R3-F1 through R3-F5 now land in
> `architecture.md` and R3-F6 lands in `ux/menubar.md`. This record therefore
> covers both documents until the repair passes.

## Round 1 — 2026-08-15

- Reviewed state: `docs/specs/menubar-experience-design.md`, uncommitted working
  tree; no menu-bar implementation exists yet
- Reviewer: claude-code
- Scope: the design contract only. Method limitation stated below.
- Method limitation: this round originated from a Dieter Rams UI audit, whose
  scoring anchors assume an implemented visual surface. Four of its ten
  principles had no measurable basis against an unimplemented contract, and its
  aesthetic principle scored 0 purely because no prototype exists, which its own
  rules treat as a redesign trigger. The numeric total is therefore not carried
  forward and the audit artifacts were removed. Only findings that survive
  independent verification against the repository are recorded here.

- Findings:
  - [P1] The design does not specify how the GUI invokes a provider switch.
    Verified: `agentdeck provider use` has no command-local quiet flag; only the
    global `--quiet` exists. The design names no success/error JSON envelope, no
    Swift operation owner, and no serialization, cancellation, double-submit, or
    result-lifetime rule. -> Specify the exact invocation and result contract.
  - [P2] `stale` and `offline` wording appears without a state definition, and
    the design does not distinguish a recurring update check from a manual one
    in user-visible copy. -> Define the interaction-state and copy matrix for
    both `en` and `zh-Hans`, including confirmation, success, failure, disabled,
    and focus behavior.
  - [P2] No implementation-ready visual contract: native semantic type and
    spacing choices, numeric width/height and narrow bounds, scrolling or
    collapse rules, section grouping, focus and disabled treatment, and contrast
    acceptance are unspecified. -> Add them, or state that they are deferred to
    the implementation task with an acceptance gate.
  - [P2] Partial-failure presentation is unspecified. When candidate discovery
    fails, currently selected provider routes should stay visible; `ready: false`
    needs a localized reason and disabled behavior; growing health and warning
    content needs a bound or collapse rule. -> Specify degraded presentation.
  - [Resolved] The design's opt-in update check contradicted the authoritative
    connectivity policy, which permitted network access only for a price update.
    `docs/specs/cli-design.md` and `AGENTS.md` now permit both, recording that
    the desktop check is opt-in, defaults off, sends no local state, and only
    opens the official release page. No design change required.

- Evidence: `agentdeck provider use --help`, `agentdeck --help`,
  `docs/specs/cli-design.md` connectivity constraint, `AGENTS.md` allowed
  connectivity, `docs/specs/menubar-experience-design.md` lines 33, 64, 162
- Verdict: REOPEN

## Round 2 — 2026-08-16

- Reviewed state: `docs/specs/menubar-experience-design.md`, uncommitted working
  tree
- Reviewer: claude-code (repair round, same owner — an independent Review round
  is still required before the plan's `Review` cell may be ticked)
- Scope: the four Round 1 findings, plus the UI, UX, and UED specification the
  design had been missing entirely.

- Round 1 findings, dispositions:
  - [P1] Switch invocation and result contract -> **Fixed.** Added a result
    envelope section: outcome is decided by the presence of `error` and by
    nothing else, `MenuBarSwitchOperation` owns one attempt per client, and
    serialization, double-submit impossibility, non-cancellation, and result
    lifetime are specified.
  - [P2] `stale` / `offline` wording and update-check copy -> **Fixed.** Added an
    interaction-state and copy table in both languages. The user-visible wording
    no longer says "stale"; it states when data was updated. `offline` names the
    helper rather than the network. Manual and automatic update checks read
    differently, and a silent automatic no-op is required.
  - [P2] Missing visual contract -> **Fixed.** Added a menu-bar item section and
    a visual contract with geometry (340 pt, 280 pt narrow bound, 560 pt height
    cap), semantic type styles, a 4 pt spacing scale, semantic status colors each
    carrying a symbol and a label, and density bounds for the two unbounded
    sections.
  - [P2] Partial-failure presentation -> **Fixed.** Candidate discovery failure
    with readable routes now keeps `available: true` and shows current routes,
    with switching visibly disabled instead of the section disappearing.
    `ready: false` candidates are listed with a localized reason.

- New findings, raised during repair:
  - [P2] A failed switch for an unknown provider reports `error.code:
    runtime_error`, which appears zero times in `docs/specs/cli-design.md` and
    `docs/specs/cli-manual.md`, so it is not a stable code a consumer can map.
    Its `error.message` carries the underlying storage text `sql: no rows in
    result set`. -> Recorded in the design as a CLI prerequisite; the host is
    forbidden from displaying or logging the raw message and shows the verbatim
    code beside a generic localized explanation. The CLI defect itself is out of
    this task's scope and needs its own fix.
  - [Withdrawn] An earlier draft of this round claimed `provider use` exits `0`
    on failure. That was a measurement error: `exit=$?` had read the exit status
    of a `head` process in a pipeline rather than of `agentdeck`. Re-measured
    without a pipeline, the command exits `1`, consistent with `session show`.
    The design text asserting otherwise was corrected in the same pass.

- Evidence: `agentdeck --format json provider use nonexistent-xyz --client codex`
  writes the envelope to stderr and exits `1`; `error.code` is `runtime_error`
  with message `sql: no rows in result set`; `grep -c runtime_error
  docs/specs/cli-design.md docs/specs/cli-manual.md` returns `0` for both;
  `agentdeck --format json provider current` envelope keys are `command`, `data`,
  `generated_at`, `partial`, `schema_version`, `warnings`
- Verdict: REOPEN — repair complete, awaiting independent review

Round 1's connectivity-policy finding was resolved in the authoritative documents
rather than in the design, and is closed. The two CLI defects found in Round 2 are
prerequisites recorded for their own fix; the design works correctly around them
without hiding them.

## Round 3 — 2026-08-16

### 📋 独立设计评审 — desktop-app / menubar-experience

📊 总体评分：4/10

✅ 结论：FAIL

- Reviewed state: HEAD `75f6f2eb99269b7ccac51f5d551d1f0462dea825`
  plus `docs/specs/menubar-experience-design.md` blob
  `c7e3f0c56b3052bc1ac6cb8f2760e68886f3d6ec`
- Reviewer: codex, independent of the Round 2 repair owner
- Scope: the Task 3 design contract against the current CLI, desktop wire,
  macOS foundation, App Group, logging, and verification surfaces
- Method: repository evidence plus one frozen three-perspective challenge round
  covering execution simulation, fresh implementation, and adversarial failure
  analysis
- Completion evidence: `NOT_VERIFIED` through phase-local fallback; the current
  environment advertises no compatible `completion-evidence/v1` provider or
  configured local store, and this FAIL does not claim a Task boundary complete

#### 🔴 严重问题 — 必须修复

**R3-F1 — `docs/specs/menubar-experience-design.md:130`: GUI switch 调用缺少全局 `--quiet`。**

- 行为风险：成功 switch 仍可把 effective endpoint 和 attribution/restart
  advisory 写入 stderr；这既违反了 transport 隐私目标，也把第一项 CLI
  prerequisite 错误描述为已经由 JSON mode 解决。
- 证据：`cmd/agentdeck/main.go:1460-1469` 在 `provider use` 成功后仍调用
  route/advisory reporters；
  `cmd/agentdeck/project_attribution_guidance_test.go:110-145` 明确断言普通
  JSON invocation 有 stderr，只有加入全局 `--quiet` 后 stderr 才为空。
- 💡 有界修复：把唯一规范调用固定为
  `agentdeck --quiet --format json provider use ... --no-shell-setup`，并测试
  exact arguments、成功 stdout envelope、成功 stderr 为空以及失败只在
  stderr 返回 envelope。

**R3-F2 — `docs/specs/menubar-experience-design.md:158`: switch result envelope 与当前 CLI/Swift 解码边界不一致。**

- 行为风险：当前 `provider.use` 成功和失败都使用 `data: null`，而现有
  Swift decoder 只接受 `desktop.snapshot` 和 snapshot-shaped data；现有
  runner 还会在非零退出时先抛错，导致 stderr error envelope 无法按设计
  解码。实现者必须自行决定 stream、exit 和矛盾 envelope 的优先级。
- 证据：`internal/output/output.go:8-36` 定义 nullable `Data`；
  `cmd/agentdeck/main.go:1470` 的成功结果没有 data payload；
  `apps/macos/AgentDeckShared/DesktopWire.swift:25-80` 固定 command 和 data
  类型；`EmbeddedHelperRunner.swift:224-236` 在 decode 前拒绝非零 exit。
- 💡 有界修复：定义独立的 `ProviderUseEnvelopeV1` 或等价类型，固定
  schema/command、nullable data、error-code-only retention，以及
  stdout/stderr/exit consistency matrix；双 envelope、未知 schema/command、
  截断或无有效 JSON 必须成为 opaque/indeterminate failure。

**R3-F3 — `docs/specs/menubar-experience-design.md:198`: operation ownership、序列化、取消和结果寿命互相冲突。**

- 行为风险：设计同时允许每个 client 一个 operation、又禁止任何 client
  并行 switch；所有 terminal result 的 refresh/10 秒清除规则又与 failure
  保留到 retry/cancel 冲突。窗口关闭不取消的承诺也没有应用生命周期
  owner，而当前 runner 在 task cancellation/timeout 时会 terminate helper。
- 证据：设计表格第 207-211 行与 flow 第 482-487 行冲突；
  `EmbeddedHelperRunner.swift:107-136` 的 cancellation handler 会 terminate
  正在运行的进程。
- 💡 有界修复：选择一个 app-owned、全局 single-flight controller；view
  只观察它。Success 在 refresh 或 10 秒后清除，failure 保留到
  retry/cancel。Timeout-after-launch 必须标为 indeterminate，并强制进行
  replacement refresh 和 health/recovery reconciliation。

**R3-F4 — `docs/specs/menubar-experience-design.md:89`: candidate 不能唯一决定 mutation target。**

- 行为风险：一个 candidate 同时携带多 client、多 credential 和 wrapper
  能力，却只有 provider 级 `ready`；flow 没有定义多 credential 或
  direct/via 的选择步骤，无法保证确认内容和实际 CLI 参数是同一个目标。
- 证据：设计第 89-122 行的 candidate shape、第 154-156 行的禁止推断规则，
  与第 468-474 行从 candidate 直接进入 confirmation 的流程不闭合；CLI
  的 `--credential` 和 `--via` 是独立语义选择。
- 💡 有界修复：由 Go 输出红acted、可执行的
  `(client, provider, credential?, via_wrapper, ready, reason_code?)` option；
  confirmation 必须显示并执行同一个 option。补齐多 credential、direct/via
  和所有 disabled reason 的双语 copy。

**R3-F5 — `docs/specs/menubar-experience-design.md:67`: wire v1 兼容仅证明了旧 decoder 读取新 producer。**

- 行为风险：若新 Swift model 直接增加 non-optional `candidates`，旧 v1
  payload 缺少该 key 时会 decode 失败；同时替换两个 canonical fixture
  会丢失该回归信号。
- 证据：当前 `DesktopProviderSnapshotV1` 没有 candidates，因此能忽略新
  unknown key；设计第 124-126 行只要求给 complete/partial fixture 都增加
  新字段，没有 legacy-v1-without-candidates case。
- 💡 有界修复：保持 wire v1，但规定 missing `candidates` 解码为 `[]`、
  present value 必须是数组、producer 始终编码数组；保留一个没有 candidates
  的 legacy v1 fixture，并验证新 Swift decoder 和旧字段消费者。

**R3-F6 — `docs/specs/menubar-experience-design.md:238`: 六个 presentation state 缺少完整组合与优先级。**

- 行为风险：`degraded(previous, timeout)` 同时是 stale/offline，
  `degraded(previous, invalidWire)` 同时是 stale/error，partial empty snapshot
  也可同时命中 partial/empty；badge、retained data、copy 和 accessibility
  value 因此需要实现者自行决定。
- 证据：state derivation 只显式允许 stale+partial；copy matrix 的 error
  又要求不显示数据，与 retained-snapshot-over-empty-surface 规则冲突。
- 💡 有界修复：定义一个穷尽 coordinator state/issue/previous-snapshot 的
  truth table。无 snapshot 才使用 loading/error surface；有 snapshot 始终
  保留内容；stale、partial、offline/error issue 作为正交 qualifier；empty
  只在非 partial、全部 section 可用且为空时成立，并固定 badge/copy 顺序。

#### 🟡 建议改进 — 推荐

无；本轮未闭环项都会改变唯一 mutation、wire 兼容或可观察 UI 行为，均为
必须修复项。

#### 🟢 优点

- Candidate discovery failure 保留 readable routes 的 wire 和 presentation
  行为已经明确，能力缺失会以 disabled affordance 可见而不是静默隐藏。
- Candidate 类型的禁止字段、App Group 手工投影和 fixed-classification
  OSLog 策略形成了正确的隐私方向。
- Geometry、typography、density、keyboard、VoiceOver、contrast、motion 和
  双语视觉验收边界足够具体。
- Unknown-provider `runtime_error`/raw SQL message prerequisite 被显式记录，
  host 只保留 code、丢弃 message 的临时规避方向正确。

#### 📝 总结

评审对象是上述未提交设计 blob，结论为 FAIL。候选降级、视觉尺寸和主要隐私
投影已经具备实现基础，但 switch transport、application ownership、exact
target、wire-v1 双向兼容和 presentation state algebra 仍需六项有界设计修复。
Task 3 的 `Dev`/`Review` 均保持未勾选；不得在这些契约闭合前进入实现。

## Round 4 — 2026-08-16

- Reviewed state: HEAD `3c18e02deb345fc7090680ccf3aaa194c5590492` plus
  `docs/specs/menubar-experience-design.md` blob
  `9c94aee650ca660efdc765191b230515ae7e6d6d`
- Reviewer: claude-code (repair round for Round 3's FAIL — an independent
  Re-review is still required before either plan cell may be ticked)
- Method: each Round 3 finding re-verified against the repository before
  accepting it, rather than adopting the described remedy. Design grew from 637
  to 796 lines.

- Round 3 findings, dispositions:
  - **R3-F1** GUI invocation missing `--quiet` -> **Fixed, verified.**
    `cmd/agentdeck/main.go:1609` and `:1638` both return early only on
    `opts.quiet`, and `project_attribution_guidance_test.go` asserts stderr is
    empty only once `--quiet` is present. The canonical invocation now includes
    it, with a required stream/exit matrix. The prior claim that `--format json`
    already suppressed advisories was wrong.
  - **R3-F2** envelope incompatible with the decoder -> **Fixed, verified.**
    `DesktopWireEnvelopeV1` fixes `command` to `desktop.snapshot` and declares
    `data` non-optional, while `provider use` emits `provider.use` with
    `data: null`; `EmbeddedHelperRunner` also guards on a zero exit before
    decoding, so a stderr envelope was unreachable. Replaced with a separate
    `ProviderUseEnvelopeV1`, an explicit indeterminate class, and a runner entry
    point that captures both streams and the status.
  - **R3-F3** ownership contradictions -> **Fixed, verified.** All three
    conflicts were real: per-client versus app-wide serialization, and a terminal
    result cleared on a timer while a failure was retained until dismissed.
    `EmbeddedHelperRunner`'s `withTaskCancellationHandler` does call
    `running.terminate()` on cancel. Replaced with an app-owned globally
    single-flight `SwitchController`, separated success and failure lifetimes with
    the reason stated, and an explicit rule that the switch path must not be
    cancellable.
  - **R3-F4** candidate cannot identify a target -> **Fixed.** A candidate is now
    a display grouping; Go expands `options`, each one exactly
    `(client, provider, credential?, via_wrapper, ready, reason_code?)` mapping
    one-to-one onto the invocation's arguments. Selection happens per option, and
    five fixed `reason_code` values carry localized copy.
  - **R3-F5** compatibility proven one direction only -> **Fixed.** Specified
    that a missing `candidates` decodes as `[]` and a non-array is invalid, and
    retained one legacy fixture without the field, since replacing every fixture
    would delete the only signal for the direction that can break.
  - **R3-F6** presentation states not exhaustive -> **Fixed.** The six names were
    never mutually exclusive. Replaced with one surface plus orthogonal
    qualifiers, an exhaustive truth table, a fixed qualifier order, and the rule
    that a retained snapshot is always shown — which removes the contradiction
    between the error copy and the retention rule. Copy, accessibility, badge
    policy, and the manual checklist were realigned.

- Evidence: `cmd/agentdeck/main.go:1608-1638`;
  `cmd/agentdeck/project_attribution_guidance_test.go:110-145`;
  `apps/macos/AgentDeckShared/DesktopWire.swift:25-46`;
  `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:107-115,224-236`;
  `internal/output/output.go:8-36`
- Verdict: REOPEN — repair complete, awaiting independent Re-review

Manual verification items grew to 20, covering the empty-stderr assertion, per
option rows, the legacy fixture, global single-flight refusal, indeterminate
timeout handling, and helper survival across window dismissal.

- Post-migration state mapping. This round's `Reviewed state` names the single
  pre-split document. The topic migration has since divided that text, so the
  repaired content a Re-review must judge is two blobs at HEAD
  `3a07618ac1f1b36151077e3343fe36775ea39b26`:
  `docs/topics/desktop-app/architecture.md`
  `83c9a882d586a953ba18abfd05aa20e782aaa066` carries R3-F1 through R3-F5, and
  `docs/topics/desktop-app/ux/menubar.md`
  `7a801bd0edce2321dd4b3148de2e0eb79f60bcdb` carries R3-F6. Verified present at
  `architecture.md:649`, `:687`, `:721`, `:757`, `:785`, `:810` and
  `ux/menubar.md:103`, `:130`.
- Still open, and not a Round 3 finding: `ux/menubar.md` carries no rendered
  specimen of any state, so its `Draft` is unticked against the specimen
  requirement adopted after this round. `ux/widget.md` is required and unwritten.
  Both are tracked in the topic's Documents matrix.
