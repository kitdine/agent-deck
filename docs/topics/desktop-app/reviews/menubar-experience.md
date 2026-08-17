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
  `a713e3286936becad0359956fb7362fa57a90f8e`:
  `docs/topics/desktop-app/architecture.md`
  `83c9a882d586a953ba18abfd05aa20e782aaa066` carries R3-F1 through R3-F5, and
  `docs/topics/desktop-app/ux/menubar.md`
  carries R3-F6, including the two residuals closed below. Verified present at
  `architecture.md:649`, `:687`, `:721`, `:757`, `:785`, `:810` and
  `ux/menubar.md:103`, `:130`, `:342`. The `ux/menubar.md` blob is stated in the
  final residual entry rather than here, because closing the second residual
  changed it after this line was first written.
- Residual closed during the 2026-08-16 record repair. Re-verifying R3-F6's
  table against its own qualifier definitions found `empty` listed on the
  `refreshing` retained-snapshot row but omitted from both `degraded` retained
  rows, while the qualifier table makes it hold whenever its condition does. A
  snapshot that was complete and had no rows stays empty when the next refresh
  fails, so the omission left exactly the kind of undefined combination R3-F6
  was raised about. Both rows now carry it, with the rule stated: `empty`
  describes the retained snapshot, not the refresh outcome, and never appears on
  an `errorSurface` row, which has no snapshot to describe.
- Second residual, surfaced by the first. Adding `empty` to the `degraded` rows
  put its copy — `No local activity today` / `今天没有本地活动` — beside
  `Cannot reach the AgentDeck helper`, which reads as a claim about today that
  the app cannot make while it is showing a snapshot it could not refresh. The
  qualifier now has two forms: it may claim the day only on a current,
  issue-free surface, and otherwise describes the snapshot
  (`No activity in this snapshot` / `此快照中没有活动`). Fixing a derivation
  table without re-reading the copy it drives is how a truthful rule produces an
  untruthful sentence. Recorded at `ux/menubar.md:342`, in blob
  `38221fe155c8ac3647124deb74914b222d437b24`, which supersedes the blob named in
  the state mapping above.
- Still open, and not a Round 3 finding: `ux/menubar.md` carries no rendered
  specimen of any state, so its `Draft` is unticked against the specimen
  requirement adopted after this round. `ux/widget.md` is required and unwritten.
  Both are tracked in the topic's Documents matrix.

## Round 5 — 2026-08-16

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: HEAD `e998cf29c5b351dfb8255fdc090de758ab666f6f`
  plus `docs/topics/desktop-app/architecture.md` blob
  `83c9a882d586a953ba18abfd05aa20e782aaa066` and
  `docs/topics/desktop-app/ux/menubar.md` blob
  `38221fe155c8ac3647124deb74914b222d437b24`
- Reviewer: codex, independent of the Round 4 repair owner
- Method: repository evidence, CodeGraph-first source location, focused contract
  inspection, and one decisive consistency check. Broad product verification
  stopped after the contract itself reproduced blocking ambiguities.
- Scope: every Round 3 finding against the post-migration architecture and
  menu-bar UX contracts, plus newly blocking contradictions exposed by those
  dispositions
- Evidence: `make check-whitespace` and `git diff --check HEAD~4..HEAD` passed;
  current CLI/output/runner source remained the comparison boundary for the
  repaired contract. No implementation or product test was changed.
- Completion evidence: the compatible Neo4j `completion-evidence/v1` provider
  records both exact document states as `FAILED`; neither Document gate is
  closed by this review verdict.

#### 🔴 严重问题 — 必须修复

**R3-F2 — `docs/topics/desktop-app/architecture.md:779-786`: switch
stream/exit matrix 仍未穷尽有效 envelope 落在错误 stream 的情况。**

- 处置：**仍未关闭。** 独立 `ProviderUseEnvelopeV1`、双 stream 捕获、未知
  schema/command 与双 envelope 处理已经补齐，但矩阵没有覆盖“stdout 上的
  error envelope + non-zero exit”或“stderr 上的 success envelope + zero
  exit”。这些输入既不是表中的 typed/indeterminate 行，也不是 malformed、
  truncated 或 no-valid-JSON 的 opaque 行。
- 行为风险：实现者必须自行决定一个格式有效但 transport 位置错误的 mutation
  结果是 typed、indeterminate 还是 opaque；不同实现可能对同一配置写入给出
  相反结论。
- 证据：`architecture.md:783-786` 的四行分类没有 catch-all transport
  violation；`architecture.md:779` 又要求由 envelope 而不是 exit status
  单独决定结果。
- 💡 有界修复：为每个“单一有效 envelope、stream、error presence、exit”组合
  给出穷尽分类，或增加明确的 catch-all：任何不匹配 canonical stream 的有效
  envelope 均进入指定的 indeterminate/opaque 类，且不得据此宣称成功或失败。

**R3-F3 — `docs/topics/desktop-app/architecture.md:828,834`: retry 生命周期与
non-idle 请求拒绝规则仍互相冲突。**

- 处置：**仍未关闭。** app-owned global single-flight、不可取消与 terminal
  result 的不同寿命已经明确；但 controller 在 `failed`/`indeterminate` 时并非
  `idle`，规则一方面拒绝任何 non-idle request，另一方面又要求状态保留到用户
  retry，并在 UX 中提供 retry。
- 行为风险：实现者必须自行发明 retry 是先清除到 `idle`、原子转入
  `inFlight`，还是被 serialization 拒绝；失败后的恢复行为因此不确定。
- 证据：`architecture.md:828` 与 `:834` 的规则直接冲突；
  `ux/menubar.md:401-403` 要求 failure 提供 retry。
- 💡 有界修复：写出 controller 的允许状态转换，并明确 retry/dismiss 是
  non-idle 拒绝规则的有界例外；retry 必须原子地从
  `failed|indeterminate` 转入 `inFlight` 或先执行一个明确的 reset transition。

**R5-F1 — `docs/topics/desktop-app/architecture.md:635-655`: wire-owned
`switch_in_flight` reason 无法由 snapshot producer 知道。**

- 处置：**新发现。** 设计要求 Go 生成完整 option、host 不组合 option，并把
  `switch_in_flight` 列为固定 `reason_code`；但 in-flight 是 app-owned
  `SwitchController` 的瞬时状态，不是 helper 生成 snapshot 时可知的 provider
  元数据。
- 行为风险：实现者必须在“host 不得组合 option”和“UI 必须禁用并解释全局
  in-flight”之间任选其一，或让已有 snapshot 暴露不可能保持同步的动态原因。
- 证据：`architecture.md:635` 把 option ownership 固定给 Go，`:652-655`
  又把 `switch_in_flight` 放入 wire reason；`:814-828` 则把对应状态固定给
  app-owned controller。
- 💡 有界修复：把 `switch_in_flight` 定义为 host-only presentation overlay，
  不修改 Go-resolved mutation tuple；或删除该 wire reason，并明确 controller
  全局禁用状态的 copy 与优先级。

#### 🟡 建议改进 — 推荐

**R5-N1 — `docs/topics/desktop-app/ux/menubar.md:389-392`: selection 文案有一段
重复且语法中断。**

- 处置：新发现，非阻塞。
- 证据：`A candidate is option's exact ...` 紧接着重新开始
  `A candidate is never selected ...`；后一句与 architecture contract 足以保留
  唯一语义。
- 💡 有界改进：删除重复的半句，保留 one-option/one-confirmation 规则。

#### 🟢 优点

- **R3-F1 已关闭。** canonical invocation 已固定包含全局 `--quiet`，并规定
  success stdout、空 stderr 与 exit `0` 的一致性。
- **R3-F4 已关闭。** candidate 只用于分组，Go-resolved option 把 client、
  provider、credential 与 wrapper route 一对一绑定到确认和命令参数。
- **R3-F5 已关闭。** missing `candidates`/`options` 解码为 `[]`，non-array
  无效，并保留 legacy-v1-without-candidates fixture。
- **R3-F6 已关闭。** surface/qualifier truth table 已覆盖 retained snapshot、
  degraded、empty 的组合；empty copy 也区分 current 与 non-current snapshot。

#### 📝 总结

Round 5 对 R3-F1～R3-F6 全部重新处置：F1、F4、F5、F6 关闭，F2 与 F3
仍未闭合，并新增一个会改变 readiness ownership 的阻塞项 R5-F1。评审对象是
上述两个 committed blobs；缺少 rendered specimen 与 `ux/widget.md` 仍是独立
readiness gap，不计入本轮六项 finding 的结论。由于三项 material contract
ambiguity 仍要求实现者自行决定 mutation transport、retry transition 或动态
readiness ownership，本轮结论为 FAIL。

## Round 6 — 2026-08-16

- Reviewed state: the repair targets Round 5's three blocking findings against
  `architecture.md` blob `83c9a882d586a953ba18abfd05aa20e782aaa066` and
  `ux/menubar.md` blob `38221fe155c8ac3647124deb74914b222d437b24`, the exact
  blobs Round 5 judged.
- Reviewer: claude-code (repair round for Round 5's FAIL — an independent
  Re-review is still required before either Document cell may be ticked)
- Method: each finding's contradiction reproduced in the current text before
  choosing a remedy, rather than adopting the suggested fix unread.
- Scope: R3-F2, R3-F3, R5-F1 as named in the repair command. R5-N1 is recorded
  but was not authorized and is untouched.

- Round 5 findings, dispositions:
  - **R3-F2** transport classification not exhaustive -> **Fixed.** The matrix
    is split into two conclusive rows — each requiring its envelope on its
    canonical stream, agreeing with the exit status, with the other stream empty
    — and an inconclusive set closed by a catch-all: any decodable envelope not
    matching a conclusive row is `indeterminate`, anything that does not decode
    is `opaque`. That makes the classification total by construction, so a valid
    envelope on the wrong stream can no longer fall outside the table. It is
    reported as neither success nor failure, and its `error.code` is not shown
    as an outcome, because the helper reached the point of emitting an envelope
    and may already have written the configuration. `architecture.md:823`.
  - **R3-F3** retry versus the non-idle refusal -> **Fixed.** The refusal rule
    governs *new* switches; recovery from a terminal state is a bounded
    exception. A complete transition table is added: `failed`/`indeterminate`
    accept `Retry`, which moves atomically back to `inFlight` for the same
    target, and `Dismiss`, which returns to `idle`. A request for a *different*
    target is still refused, so an unread failure cannot be silently abandoned.
    Retry is atomic rather than reset-then-start, because an observable `idle`
    between the two would be exactly the unguarded window serialization exists
    to prevent. `architecture.md:885`.
  - **R5-F1** `switch_in_flight` unknowable to the producer -> **Fixed by
    removal from the wire.** Every remaining `reason_code` states something the
    snapshot producer can know; in-flight is transient host state created after
    the snapshot was generated, and persisting it in the App Group cache would
    carry that staleness across launches. It becomes a host-only presentation
    overlay that disables option rows without altering their `ready`,
    `reason_code`, or arguments, shown in place of an option's own reason while
    it holds, with copy `Switch in progress` / `正在切换`. The dividing line is
    stated: Go decides what a switch *is*, the host decides whether one can be
    *started right now*. `architecture.md:657`, `ux/menubar.md:398`.

- Consequential UX repairs, required by the three above rather than raised
  separately:
  - The failure step offered "retry or cancel" while step 2's confirmation
    button and step 6's non-cancellable rule both use `Cancel` with different
    meanings. Renamed to `Retry` / `重试` and `Dismiss` / `关闭`, with the
    reason recorded: offering `Cancel` beside a finished failure implies an
    operation could still be called off, which step 6 forbids.
    `ux/menubar.md:409`.
  - Step 7 offered a different action set for `indeterminate` than for
    `failed`, contradicting the controller that treats both terminal states
    identically. Aligned to the same two actions.
  - The single-flight sentence was restated with the retry exception, so the UX
    and the transition table no longer disagree.
  - Manual checklist items 21 through 23 added: wrong-stream classification,
    retry atomicity with the different-target refusal, and the in-flight overlay
    reverting to snapshot values.

- Evidence: `make check-whitespace` passes; every new user-visible string is
  present in both `en` and `zh-Hans`; no product code, test, or configuration
  was changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

Still open and out of this repair's scope: R5-N1's duplicated sentence, the
absent rendered specimens, and the unwritten `ux/widget.md`.

## Round 7 — 2026-08-16

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `c2ff1623f2a509994f596f4730f7786fbeec8194`
  plus `docs/topics/desktop-app/architecture.md` blob
  `137834b9098560ad1168c758ace9467737d1db07` and
  `docs/topics/desktop-app/ux/menubar.md` blob
  `de7da5c7342d0f7f076618984807bccdf1772e41`
- Reviewer: codex, independent of the Round 6 repair owner
- Method: focused inspection of the committed repair diff and the final
  architecture/UX contracts. Broad product verification stopped after two
  contract-level reproducers decisively kept the document boundary open.
- Scope: R3-F2, R3-F3, and R5-F1, including Round 6's consequential UX changes;
  R5-N1 remains a recorded non-blocking improvement outside the repair scope
- Evidence: the exact committed state and line-level contradictions below;
  `make check-whitespace` and `git diff --check` pass after the review/status
  artifacts were synchronized. No product code, test, configuration, or reviewed
  contract was changed by this Re-review.
- Completion evidence: the compatible Neo4j `completion-evidence/v1` provider
  records both exact document states as `FAILED`; both Document gates remain
  open.

#### 🔴 严重问题 — 必须修复

**R3-F3 — `docs/topics/desktop-app/architecture.md:868-904`: terminal states
仍未保存 atomic retry 所需的完整 mutation target。**

- 处置：**仍未关闭。** Round 6 已明确 retry 是 non-idle refusal 的有界例外，
  且 transition 必须原子；但状态仍只有 `inFlight(client, provider)`、
  `failed(code)` 和无关联值的 `indeterminate`，而 retry 又要求重跑同一个
  `(client, provider, credential?, via_wrapper)` option。
- 行为风险：window dismiss 后由 app-owned controller 独立存活时，terminal
  state 无法从自身恢复 credential 与 wrapper route。实现者必须额外发明一个
  未入契约的 `lastTarget`，或退化为只按 client/provider 重建命令，破坏
  one-option/one-invocation 保证。
- 证据：`architecture.md:868-869` 的状态形状不携带完整 option；`:902` 要求
  same-target atomic retry；`ux/menubar.md:411` 和 `:556-558` 重复要求完全相同
  target，且 controller 明确不依赖 view lifetime。
- 💡 有界修复：让 `inFlight`、`failed`、`indeterminate`（以及需要展示 identity
  的 success）显式关联同一个 resolved option/immutable switch target，并让
  transition table 写出该值如何原样跨越 terminal state 与 retry。

**R7-F1 — `docs/topics/desktop-app/architecture.md:665-672`: host overlay 把
所有 non-idle terminal state 都错误显示为 `Switch in progress`。**

- 处置：**新发现。** R5-F1 的 wire ownership 已正确关闭，但 repair 把 overlay
  条件写成 controller `not idle`；controller 在 `succeeded`、`failed`、
  `indeterminate` 时同样 not idle。UX 与 checklist 则只在 `inFlight` 时显示
  `Switch in progress`，离开 in-flight 后恢复 snapshot reason。
- 行为风险：已经成功、失败或结果不确定的操作会继续被宣称为“正在切换”；若
  实现遵循 UX 而恢复 option rows，又会与 architecture 的 non-idle refusal 和
  disabled-row contract 不一致。
- 证据：`architecture.md:665-672` 使用 `not idle`/`returns to idle`；`:868-904`
  定义三个 terminal non-idle states；`ux/menubar.md:394-403` 与 `:559-563`
  把该文案和恢复条件限定为 in-flight。
- 💡 有界修复：按 controller state 穷尽 option-row availability 与 overlay copy。
  `inFlight` 才能显示 `Switch in progress`；对 `succeeded`、`failed`、
  `indeterminate` 分别定义是否禁用、何时恢复 snapshot reason，以及 truthful
  copy，不得用 in-progress 文案覆盖 terminal result。

#### 🟡 建议改进 — 推荐

**R5-N1 — `docs/topics/desktop-app/ux/menubar.md:389-392`: selection 文案的
重复半句仍在。**

- 处置：仍未处理，非阻塞；Round 6 正确保持了 repair scope。
- 证据：后一句和 architecture contract 已保留唯一行为语义，因此它不改变本轮
  verdict。
- 💡 有界改进：在获得相应修复授权时删除重复半句。

#### 🟢 优点

- **R3-F2 已关闭。** 两个 conclusive shape 加 decodable/opaque catch-all 使
  stream/envelope/exit 分类 total；wrong-stream envelope 明确进入
  indeterminate，并强制 replacement refresh。
- **R5-F1 原 finding 已关闭。** `switch_in_flight` 已从 wire reason 删除，Go
  继续独占 resolved mutation tuple，host 独占瞬时可启动性。
- **R3-F3 的原 refusal/retry 冲突已部分关闭。** transition table 明确 retry
  原子、dismiss 返回 idle、different-target request 必须先 dismiss；剩余 blocker
  仅是状态没有保存该 same target。
- Round 5 已关闭的 R3-F1、R3-F4、R3-F5、R3-F6 未见回归。

#### 📝 总结

Round 7 重新处置了 Round 5 的三项 blocker：R3-F2 与 R5-F1 原 finding 关闭；
R3-F3 的 transition 冲突虽已修复，但 same-target 数据所有权仍不完整，因此继续
保持 open；R7-F1 新发现 terminal state 被错误套用 in-progress overlay。
评审对象是上述两个 committed blobs。rendered specimen、`ux/widget.md` 与
R5-N1 仍是独立 readiness/improvement 项，不计入两项 material blocker。
由于实现者仍须自行决定 retry target 存储和 terminal-row presentation，本轮为
FAIL，两个 Document gate 保持未关闭。
