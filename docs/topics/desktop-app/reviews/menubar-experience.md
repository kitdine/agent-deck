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

📊 总体评分：9/10

✅ 结论：PASS

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

## Round 8 — 2026-08-17

- Reviewed state: repair of Round 7's two open findings, plus the surface
  redesign the user's design review required. Resulting blobs at HEAD:
  `architecture.md` `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`,
  `ux/menubar.md` `af8bdff81c344086fa399d1617605f15f3e2306e`,
  `ux/widget.md` `c317583f8ab9a521c9b49d51c16462f4cb1319fe`.
- Reviewer: claude-code (repair round — an independent Re-review is still
  required before any Document cell may be ticked)
- Scope: R3-F3 and R7-F1, and the design findings raised directly by the user
  against the rendered prototype.

- Round 7 findings, dispositions:
  - **R3-F3** terminal states do not retain the retry target -> **Fixed.** Round
    6 defined retry as atomic "for the same target" while the state enum was
    `failed(code)` and `indeterminate`, neither of which carries one — the rule
    was unimplementable from the state alone. Every non-idle state now carries
    the complete resolved option `opt = (client, provider, credential?,
    via_wrapper)`, the same tuple the canonical invocation takes, and retry reads
    its target from the state rather than from presentation. The reason is
    recorded: the window may be closed and the option list re-derived between the
    failure and the retry, so the only reliable source is the controller itself,
    and the view model observes the controller rather than the reverse.
  - **R7-F1** in-progress copy applied to terminal states -> **Fixed.** The
    overlay was keyed on "while the controller is not `idle`", and `failed` and
    `indeterminate` are also not idle, so `Switch in progress` would have
    appeared beside a switch that had already finished and failed. The overlay
    now applies in `inFlight` alone; terminal states present their own result
    with retry and dismiss. Both documents carry the correction.

- Design findings raised against the prototype, dispositions:
  - **Close button on the popover** -> **Fixed.** A `MenuBarExtra` popover is
    dismissed by clicking away and has no title bar or close control. The board
    had drawn window chrome that does not exist on this surface. Removed.
  - **Settings and Quit in the reading surface** -> **Fixed.** Moved to the `⌄`
    footer menu with provider switching, all three being rare deliberate acts.
  - **Provider switching leading the surface** -> **Fixed.** Demoted to that
    footer menu. Usage is what the surface is for.
  - **Widget set was arbitrary** -> **Fixed.** Seven cards had been listed one
    per interesting field, which answers nothing about why seven. The set is now
    derived from the four questions the data can answer — magnitude,
    composition, trust, rhythm — which closes it: a fifth widget requires a fifth
    question. Size selects depth rather than subject, each family a superset of
    the one beneath, giving twelve configurations and a natural Dynamic Type
    degradation.
  - **Popover had no derived sectioning** -> **Fixed.** Its body is the same four
    kinds in the same order, so learning one surface teaches the other. Health
    and warnings are not a fifth section; they qualify the other four and render
    as the banner strip.
  - **Too little of the available data was rendered** -> **Fixed upstream.**
    `usage stats` already returns buckets, models, clients, providers with
    attribution, cache components, activity, peak, coverage and unpriced models,
    and the terminal report already draws a 7×24 heatmap. The App Group
    projection is extended to carry them with stated bounds — 90 buckets, 12
    models, 12 unpriced identifiers.

- Process finding recorded against `docs/README.md`, not this topic: the
  progression reviewed `ux/` and `architecture.md` in parallel, which let the
  projection be treated as fixed and the surface trimmed to fit it. The surface
  now leads and the contract answers, provisioning or refusing each requested
  field with a stated ground.
- Evidence: `make check-topic-docs` and `make check-whitespace` pass. No product
  code, test, or configuration was changed; this remains a contract-document
  repair.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 9 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`;
  `docs/topics/desktop-app/architecture.md` blob
  `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`;
  `docs/topics/desktop-app/ux/menubar.md` blob
  `af8bdff81c344086fa399d1617605f15f3e2306e`;
  `docs/topics/desktop-app/ux/prototype/desktop-surfaces.html` blob
  `1081e56bff7f3acbd4a06e7561bbe5f93d41a57a`. Both contract blobs are
  byte-identical to the ones Round 8 declared, so its repair diff needed no
  re-derivation.
- Reviewer: claude-code (independent of Round 8's repair only in pass, not in
  authorship — the same agent wrote that repair, so every disposition below is
  re-derived from the documents rather than read off the repair note)
- Method: Single-agent bounded Re-review. Re-derived R3-F3 and R7-F1 from the
  controller contract and the UX switch flow on both sides, then checked the
  Round 8 redesign for the failure mode a redesign produces — prose and
  prototype moving while the artifacts that illustrate them stay behind.
- Scope: R3-F3 and R7-F1; the design findings Round 8 closed; regression on
  R3-F1, R3-F2, R3-F4, R3-F5, R3-F6 and R5-F1; and R5-N1, still non-blocking.

#### 🔴 严重问题 — 必须修复

**R9-F1 — `docs/topics/desktop-app/ux/menubar.md:220-341`: 全部 text specimen
仍是被 Round 8 废弃的旧结构。**

- 处置：**新发现。** Round 8 重设计了 popover：body 变为 magnitude、
  composition、trust、rhythm 四段（`:170-180`），窗口 chrome 被删除因为
  `MenuBarExtra` popover 无标题栏，Settings 与 Quit 移入 `⌄` footer 菜单
  （`:182-189`）。散文和 prototype 都照此改了，四个 specimen 一个没改。
- 行为风险：这不是过时插图。`:209-212` 明确 specimen 的作用是"评审按 blob hash
  引用契约，读者需要内联的状态"——它们是被指定的评审入口。实现者按 specimen
  搭出的是 `PROVIDER / USAGE / COST / RECENT SESSIONS / HEALTH` 五段加一个
  `✕` 关闭按钮和 footer 里的 `Settings…`、`Quit`，与散文要求的四段式加 footer
  菜单没有一处对应。同一份文档对同一个界面给出两种互斥结构，实现者必须自己选
  一个，而 specimen 是更具体的那个。
- 证据：`:222`、`:250`、`:262` 画出 `⋯ ✕`，而 `:187-189` 说这个界面没有 chrome；
  `:241` 把 `Refresh Settings… Quit` 平铺在 footer，而 `:186-189` 要求它们进
  `⌄` 菜单；`:224-238` 的段落名与 `:175-180` 的表格无一致项；四个 specimen 全
  无 period switcher、client tabs、trend chart、trust 行或 7×24 grid，而这些是
  `:746-770` 声明为必需字段的元素。对照 prototype `1081e56b`：`:591-639` 是四
  段式，`:661` 与 `:678-679` 明确记录了 Settings/Quit 进 footer 菜单和 popover
  复用同四段——prototype 跟上了，text specimen 没有。
- 💡 有界修复：把四个 specimen 重画为四段式 body 加 client tabs 加 footer，删除
  `✕` 与平铺的 `Settings…`/`Quit`；或者，若认为 prototype 已足以承载这些状态，
  则删除 specimen 并改写 `:209-212` 说明状态索引改由 prototype 承载。二者都可
  接受，同时保留两套互斥结构不可接受。

**R9-F2 — `docs/topics/desktop-app/ux/menubar.md:754`: period switcher 声称
`buckets` 可按 day/week/month 分组，投影只提供日桶。**

- 处置：**新发现。** 该行位于 Data requirements 表，表头（`:748-750`）声明
  "These are provisioned as of `architecture.md`'s 2026-08-17 revision"。week 与
  month 未被提供。
- 行为风险：`architecture.md:444` 只投影"a bounded daily series: at most 90
  buckets"，`:467-472` 又把该上界定为契约而非实现细节。实现者要么把 week/month
  切换器做成读不到数据，要么在 Swift 侧按日桶二次聚合——而
  `requirements.md:134-135` 明确禁止宿主端再聚合。两条路都违反已通过评审的上游
  边界。
- 证据：`ux/menubar.md:754` 断言已提供；`architecture.md:438-456` 的投影清单无
  周桶或月桶，`:458` 起的新增理由也未提及；`requirements.md:76-77`（Round 5
  PASS）授权的时间粒度止于日桶与 7×24；`ux/widget.md:85-86` 的 period 只有
  `today`/`7d`/`30d`，同一产品的另一个界面没有 week/month。
- 💡 有界修复：把该行改为切换器实际需要的东西——`today`/`7d`/`30d` 三个期间选择
  加 ≤90 日桶的趋势序列，与 `ux/widget.md:85-86` 及 `requirements.md:76-77` 对齐；
  若确实要 week/month，则需先重开 `requirements.md` 与 `architecture.md`，因为
  投影必须先承载这些桶。本轮不作此推定：四个有界问题中没有一个是日趋势与 7×24
  回答不了而 week/month 能回答的。

#### 🟡 建议改进 — 推荐

**R5-N1 — `docs/topics/desktop-app/ux/menubar.md`: selection 文案的重复半句。**

- 处置：仍未处理，非阻塞。Round 8 的 repair scope 是 R3-F3、R7-F1 与用户提出的
  设计发现，未含此项，保持 scope 正确。
- 证据：语义仍由后一句和 architecture contract 唯一确定，不改变本轮 verdict。
- 💡 有界改进：在获得修复授权时随附近改动一并删除。

#### 🟢 优点

- **R3-F3 已关闭。** 状态形状现在是 `inFlight(opt)`、`succeeded(opt)`、
  `failed(opt, code)`、`indeterminate(opt)`，`opt` 是与 canonical invocation 同
  一个 `(client, provider, credential?, via_wrapper)` 元组
  （`architecture.md:903-913`）。transition 表（`:946-956`）写明 retry 从
  `failed|indeterminate` 原子转入 `inFlight(opt)`，且 `opt` 取自状态本身；
  `:958-961` 记录了理由——窗口可能已关闭、快照可能已替换、选项列表可能已重新推
  导，所以唯一可靠来源是 controller。这正是 Round 7 要求的所有权归属。
- **R7-F1 已关闭，且两侧一致。** `architecture.md:693-699` 把 overlay 限定为
  `inFlight` **only**，并明确记录了旧稿键在 "not idle" 会在已失败的切换旁显示
  `Switch in progress`；`ux/menubar.md:554-558` 用相同措辞说 overlay 属于
  `inFlight` alone，terminal 状态下 rows 回到快照所述。`:563-574` 为 failure 与
  indeterminate 各自定义了 `Retry`/`Dismiss`，并解释了为何不复用 `Cancel`。
- **Round 5 与 Round 7 已关闭的项无回归。** R3-F1 canonical invocation、R3-F2
  stream/envelope/exit 全分类、R3-F4 option 绑定、R3-F5 解码回退、R3-F6 truth
  table、R5-F1 wire ownership 均未被 Round 8 的重设计改动。
- **Round 8 关闭的设计发现，散文层面确实落地。** 四段式 body 由数据能回答的四个
  问题推导而来而非按字段罗列（`:170-173`），footer 明确不是第五段因为它不回答关
  于花费的问题（`:185-189`），投影扩展记录了准入理由——模型标识、日 token 总数、
  周内小时强度、归因计数都是事件的聚合而非内容（`architecture.md:458-465`），并
  把 90/12/12 三个上界定为契约（`:467-472`）。
- **投影扩展提供了 trust 与 rhythm 所需字段。** `architecture.md:446-453` 的
  7×24 grid、per-client/per-provider 归因计数、pricing coverage 与
  `ux/menubar.md:759-761` 一一对应。

#### 📝 总结

逐条处置：R3-F3 关闭；R7-F1 关闭，且 `architecture.md` 与 `ux/menubar.md` 两侧
措辞一致；Round 5 与 Round 7 已关闭的六项无回归；Round 8 的设计发现在散文与
prototype 中确实落地；R5-N1 仍非阻塞；新增 R9-F1 与 R9-F2 两项阻断。评审对象为
HEAD `8beacdb1` 与上述两个 blob，与 Round 8 声明逐字节一致、工作区干净。

两项新发现同源：Round 8 是一次重设计，散文与 prototype 都改了，而依附于旧结构的
两处产物没跟上。R9-F1 是四个 text specimen 仍画着五段式加窗口 chrome，而文档自己
（`:209-212`）指定 specimen 为评审的内联状态入口，因此实现者会读到两套互斥结构；
R9-F2 是 Data requirements 表把 week/month 分组列为"已提供"，而投影只有日桶，且
`requirements.md` 在 Round 5 PASS 时授权的时间粒度止于日桶与 7×24。两者都不是契
约歧义而是文档内部不一致，修复边界清晰，不涉及 architecture 的行为契约。

本轮为 FAIL，两个 Document gate 保持未关闭。残余不确定性：R9-F2 的有界修复假定用
户并不真的要 week/month 粒度；若要，需连带重开 `requirements.md` 与
`architecture.md`，因为投影必须先承载这些桶。

证据：`git rev-parse HEAD` -> `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`；两个
blob hash 与 Round 8 声明一致；`bash scripts/check-topic-docs.sh` -> exit 0；
`make check-whitespace` -> exit 0。未改动任何产品代码、测试、配置或被评审的契约。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / R9-F1 R9-F2
```

## Round 10 — 2026-08-17

- Reviewed state: repair of Round 9's two open findings, against
  `docs/topics/desktop-app/ux/menubar.md` blob
  `af8bdff81c344086fa399d1617605f15f3e2306e`, the exact blob Round 9 judged.
- Reviewer: claude-code (repair round for Round 9's FAIL — an independent
  Re-review is still required before the `ux/menubar.md` Document cell may be
  ticked)
- Scope: R9-F1 and R9-F2 as named in the repair command, plus R5-N1 and the
  prototype's unprovisioned `Month` tab, both explicitly requested for
  inclusion rather than left for a later round.

- Round 9 findings, dispositions:
  - **R9-F1** text specimens still drew the structure Round 8 abandoned ->
    **Fixed.** All four affected specimens (healthy, helper-unreachable,
    narrow-bound, and the partial/empty fragments) are redrawn onto the
    four-section body — client tabs, hero, period switcher, `TREND`,
    `MODELS`, `ATTRIBUTION`, `RHYTHM` — with provider state, the `⌄` actions
    menu, refresh, and the session link moved into the footer. The `⋯ ✕`
    title-bar row and the flat `Refresh Settings… Quit` footer row are
    removed; the healthy specimen's intro line now states explicitly that a
    `MenuBarExtra` popover has no chrome to draw. The switch-flow specimens
    (confirmation, in-flight, failed) were already footer-menu dialogs and
    needed no change. `ux/menubar.md:218-347`.
  - **R9-F2** period switcher claimed week/month grouping the projection does
    not carry -> **Fixed.** The Data requirements row now states what the
    switcher actually needs — `today`/`7d`/`30d` period selection backed by
    the daily `buckets` series — matching `ux/widget.md:85-86` and the
    daily/7×24 granularity `requirements.md` authorized at its Round 5 PASS.
    No `requirements.md` or `architecture.md` reopening was needed, because
    the bounded fix assumed the switcher, not the projection, was wrong.
    `ux/menubar.md:754`.

- Non-blocking and residual items, dispositions:
  - **R5-N1** selection copy duplicated and grammatically broken ->
    **Does not reproduce; closed as no defect found.** The finding's own
    evidence is `ux/menubar.md:389-392` at Round 5's reviewed blob
    `38221fe155c8ac3647124deb74914b222d437b24`. Reading that exact blob at
    those exact lines shows one sentence — "choosing one `option` opens
    confirmation, carrying that option's exact `(client, provider,
    credential?, via_wrapper)`. A candidate is never selected as a whole, and
    the row is never itself the commit." — with no repeated clause and no
    syntax break. `git log -p` shows this file has carried exactly this
    wording since the line was first committed; no intermediate revision ever
    held a duplicate. The finding was carried forward as open for four rounds
    (5, 6, 7, 8, 9) without any round re-reading the cited lines against the
    cited blob, which is how a misreading survives past the text it described.
    Current location: `ux/menubar.md:548-550`.
  - Prototype period tabs claimed a fourth `Month` option ->
    **Fixed.** `ux/prototype/desktop-surfaces.html:285` and `:374` both read
    `Today · 7 Days · 30 Days · Month`. `Month` is not a period
    `ux/widget.md:85-86` defines, not one the corrected Data requirements row
    provisions, and not one any projection field backs — it is the same
    overclaim R9-F2 found in the prose, just left uncorrected in the
    prototype when R9-F2 named only `ux/menubar.md:754`. Removed from both
    occurrences, leaving `Today · 7 Days · 30 Days`, matching the widget
    document and the repaired Data requirements row.

- Site checked and left unchanged, recorded for the Re-review to confirm
  rather than asserted as settled: `ux/prototype/desktop-surfaces.html:688`
  reads "`buckets` groupable by hour / day / week / month" inside the "The
  data was always there" callout. That line describes `usage stats`'s own
  capability, not the desktop projection or the period switcher.
  `docs/specs/cli-manual.md:488-489` states `group-by` accepts
  `auto|hour|day|week|month` for that command. Left as-is on that basis, but
  this repair round is the one that made the call, so it is not independent
  confirmation.

- Evidence: `make check-whitespace` passes; `bash scripts/check-topic-docs.sh`
  passes; `git diff --check` passes; no product code, test, or configuration
  was changed. This remains a contract-document repair.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No
  Document gate is closed and no commit is authorized by this round; both
  require the independent Re-review's own verdict.

Nothing remains open from this round by the repair owner's own assessment.
The Re-review should independently confirm every disposition above,
including R5-N1's non-reproduction and the `usage stats` line left
unchanged, and should still check for further `Month`- or week/month-shaped
residue elsewhere in the topic's documents; this repair checked only the
sites named above.

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md / Round 10
```

## Round 11 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewed state: HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`，工作区未提交；
  `docs/topics/desktop-app/ux/menubar.md` blob
  `bbe920a213b91fa061a68cad7593e1c0af671a52`；
  `docs/topics/desktop-app/architecture.md` blob
  `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`（与 Round 8/9 逐字节一致，Round 10
  未改动）；`docs/topics/desktop-app/ux/prototype/desktop-surfaces.html` blob
  `8a8c8e5d16acfa41206ac789429078e92baefe89`
- Reviewer: claude-code（与 Round 10 修复同一 agent，因此每条处置均从文档本身重新
  推导，而不是照读修复说明；每条“已修复”均回到被引用的确切 blob 与行号复核）
- Method: 单 agent 有界复评。先按 Round 9 的两条 finding 重新推导 specimen 结构与
  Data requirements 行，再独立复核 Round 10 主动纳入的两项残余项与它自陈"已检查但
  未改动"的一处，最后按 Round 10 交接要求做 week/month 形状的全 topic 残留扫描。
  重设计轮次的典型失效模式是产物落后于散文，因此本轮把 specimen 与它所引用的
  copy 表逐串比对，而不只看结构。
- Scope: R9-F1、R9-F2；R5-N1；prototype `Month` 页签；`usage stats` callout；
  R3-F1～R3-F6、R5-F1、R7-F1 的回归检查
- Evidence: `git rev-parse HEAD`、`git hash-object` 三个 blob 如上；
  `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审的契约。
- Completion evidence: 兼容的 Neo4j `completion-evidence/v1` provider 中
  `desktop-app:architecture.md` 与 `desktop-app:ux/menubar.md` 两个 WorkUnit 的最新
  记录均为 `FAILED`，且 `ux/menubar.md` 的内容状态已在 Round 10 后改变，没有任何
  证据覆盖当前候选状态。两个 Document gate 均未关闭。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

**R11-N1 — `docs/topics/desktop-app/ux/menubar.md:225-347`: 重绘后的 specimen
边框宽度不齐，13 行比框宽多一列。**

- 处置：**新发现，非阻塞。** 由 Round 10 的重绘引入。
- 证据：healthy specimen 的上下边框（`:226`、`:253`）为 46 列，而其中的空行
  （`:228`、`:231`、`:233`、`:237`、`:241`、`:245`、`:248`）为 47 列；
  helper-unreachable（`:273`、`:277`）、loading（`:260`、`:262`）、narrow-bound
  （`:331`、`:335`，框宽 36 对 37）与 empty 片段（`:345`）同样如此。对照 Round 9
  评判的 blob `af8bdff8`：其全部 70 行框线恰为 46 列、10 行为 36 列，无一例外
  （两行 63/77 列是 `← both disabled` 这类框外注解，不属框线）。因此这是本轮
  被修复文件自身的回归，而不是既有约定。
- 行为风险：无契约歧义。specimen 明确声明不承载比例与换行（`:214-216`，那些由
  prototype 承载），因此右边缘参差只损害它作为"评审内联入口"（`:209-212`）的可读
  性——而这正是 R9-F1 认定它必须成立的作用。
- 💡 有界改进：把这 13 行的行尾多余空格删掉，使每个 specimen 内所有框线等宽。

**R11-N2 — `docs/topics/desktop-app/ux/menubar.md:346`: empty specimen 写的
`No activity today` 不是 copy 表固定的字符串。**

- 处置：**新发现，非阻塞。** 早于 Round 10（`af8bdff8:340` 同样如此），但
  Round 10 正是重绘该 specimen 的一轮，重绘时未回读它所展示的 copy。
- 证据：`:508` 把 `empty`（current）的 `en` 文案固定为 `No local activity today`
  / `今天没有本地活动`，`:509` 把带任何 freshness/reachability qualifier 的形态固定
  为 `No activity in this snapshot`；`:516-522` 说明这两种形态的存在理由正是
  "local/today" 的断言边界。specimen 显示的 `No activity today` 两者都不是。
- 行为风险：与 R9-F1 同类，只是尺度更小——同一份文档对同一处用户可见文案给出两
  个版本，而 specimen 是更具体的那个。实现者照 specimen 落地会丢掉 `local` 这个
  刻意保留的限定词，即 Round 4 第二项残余修复所建立的区分。
- 💡 有界改进：把 `:346` 改为 `No local activity today`，与 `:508` 对齐；该 specimen
  描述的是 current、无 issue 的表面，属 `:508` 而非 `:509` 的形态。

#### 🟢 优点

- **R9-F1 已关闭。** 四个 specimen 全部重绘为四段式：`:227` 是 client tabs，
  `:229-232` 是 magnitude hero 加 period switcher，`:234`、`:238`、`:242`、`:246`
  依次是 `TREND`、`MODELS`、`ATTRIBUTION`、`RHYTHM`，与 `:175-180` 的段落表逐行
  对应；footer（`:249-252`）承载 provider 只读状态、`⌄` 菜单、refresh 与 session
  链接，与 `:182-189` 一致。`rg '✕|Settings… Quit|Refresh Settings'` 在该文件返回
  零命中，窗口 chrome 与平铺的 `Settings…`/`Quit` 已彻底移除；`:221-223` 还把
  "`MenuBarExtra` popover 无 chrome 可画"写进了 specimen 前言，使结构选择自带理由。
  R9-F1 点名缺失的 period switcher、client tabs、trend chart、trust 行与 7×24 现在
  都在。narrow-bound 与 partial 片段也改用同一结构。
- **R9-F2 已关闭。** Data requirements 的 period switcher 行（现 `:760`）改为
  "`today`/`7d`/`30d` period selection, backed by the daily `buckets` series"，与
  `ux/widget.md:85-86` 的三个 period、`architecture.md:438-456` 只投影日桶、
  `requirements.md:75,137` 在 Round 5 PASS 时授权的"至多 90 日桶加 7x24"三处同时
  对齐。选择的是 R9-F2 建议的有界路径：改切换器而非重开投影，因此
  `requirements.md` 与 `architecture.md` 都未被改动，也不必重开。
- **R5-N1 不成立，已关闭。** 独立复核了 Round 10 的非复现结论：
  `git cat-file -p 38221fe1…`（Round 5 评判的确切 blob）第 389-392 行读到的是单独
  一句 "choosing one `option` opens confirmation, carrying that option's exact
  `(client, provider, credential?, via_wrapper)`. A candidate is never selected as
  a whole, and the row is never itself the commit."——没有重复半句，也没有语法
  中断。当前位置 `:548-550` 文本相同。该 finding 从 Round 5 起被携带了五轮而无人
  回到被引用的 blob 与行号复核，这正是 Round 9 之后新政策（不得把任何 finding 延后
  到 PASS）要防的情形；关闭方式是"无缺陷"，不是"已修复"。
- **prototype 的 `Month` 页签已移除。** `desktop-surfaces.html:285` 与 `:374` 现读
  `Today · 7 Days · 30 Days`，全文件再无 `Month` 命中，与修复后的 Data requirements
  行及 `ux/widget.md:85-86` 一致。
- **Round 10 声明"已检查但未改动"的一处，独立确认判断正确。**
  `desktop-surfaces.html:688` 的 "`buckets` groupable by hour / day / week / month"
  位于 "The data was always there" callout 内，该段句首即 "`usage stats` returns"，
  陈述的是 CLI 自身能力；`docs/specs/cli-manual.md:488-489` 确认 `group-by` 接受
  `auto|hour|day|week|month`。该句为真，且不声称桌面投影或 period switcher 拥有这些
  粒度，保留正确。
- **week/month 形状残留扫描无遗漏。** 对 `docs/topics/desktop-app/` 全目录扫描后，
  除评审记录与 `tasks.md` 的叙述性引用外，仅剩 `requirements.md:75,137` 与
  `architecture.md:461` 的 "hour-of-week"——那是 7×24 网格的维度名，不是桶粒度声明。
  没有第二处未被授权的 week/month 主张。
- **前轮已关闭项无回归。** `architecture.md` blob 与 Round 8 声明、Round 9 评判逐
  字节一致（`e23ccc7c`），Round 10 未触及该文件，因此 R3-F1、R3-F2、R3-F3、R3-F4、
  R3-F5、R5-F1、R7-F1 的关闭状态沿用 Round 9 绑定在同一内容状态上的证据，无需重跑。
  `ux/menubar.md` 侧的 R3-F6 truth table（`:120-152`）、R7-F1 的 `inFlight`-only
  overlay（`:553-564`）与 switch flow 的 `Retry`/`Dismiss`（`:569-591`）在重绘后
  文字未变，in-flight specimen（`:310-315`）仍展示 overlay 覆盖单行 reason。

#### 📝 总结

逐条处置：R9-F1 关闭，四个 specimen 已整体改画为四段式并删除不存在的窗口 chrome；
R9-F2 关闭，period switcher 的数据需求改为投影实际承载的三期间加日桶；R5-N1 以
"不复现、无缺陷"关闭，且该结论已回到被引用的 blob 独立验证；prototype 的 `Month`
页签已移除；`usage stats` callout 保留正确；全 topic 无其他 week/month 形状残留；
`architecture.md` 未改动，其全部 finding 保持关闭。新增两项非阻塞发现 R11-N1 与
R11-N2。

本轮为 FAIL。两项发现都不是契约歧义，行为风险都很低，但按本 topic 于 2026-08-17
采纳的门禁——任何未关闭的 finding（含 minor）都不得随 PASS 延后——它们必须先关闭。
两项同源，且与 R9-F1 同源：Round 8 与 Round 10 两次改动 specimen 时，都只对齐了
结构而没有回读 specimen 自身呈现的细节（框线宽度、文案串）。修复边界清晰，合计
14 行编辑，不涉及任何行为契约。

`architecture.md` 在本轮没有任何未关闭 finding，其 Document gate 之所以仍未关闭，
只是因为本记录同时覆盖两份文档而整体结论为 FAIL；它预计随下一轮 PASS 一并关闭，
不需要针对它的额外修复。

残余不确定性：R11-N1 的"框线应等宽"以该文件自身在 `af8bdff8` 之前各轮的一致做法
为准，本 topic 没有独立的 ASCII specimen 排版约定文档（`ux/widget.md` 不含框图）。
若用户认为框线参差不值得一轮修复，可显式豁免，该判断属用户而非评审。

证据：`git rev-parse HEAD` -> `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`；
`git hash-object` 三个 blob 如 Reviewed state 所列；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / R11-N1 R11-N2
```

## Round 12 — 2026-08-17

- Reviewed state: repair of Round 11's two open findings, against
  `docs/topics/desktop-app/ux/menubar.md` blob
  `bbe920a213b91fa061a68cad7593e1c0af671a52`, the exact blob Round 11 judged.
- Reviewer: claude-code (repair round for Round 11's FAIL — an independent
  Re-review is still required before either Document gate may be ticked; this
  round does not close either gate and authorizes no commit)
- Scope: R11-N1 and R11-N2 as named in the repair command.

- Round 11 findings, dispositions:
  - **R11-N1** specimen borders uneven width -> **Fixed.** Every blank-fill
    row in the wide (`340 pt`, 46-column) specimens carried one trailing
    space too many; the narrow-bound (`280 pt`, 36-column) specimen's two
    blank rows had the same defect at 37 columns. The `MODELS … Unavailable`
    row was off in the other direction. Trimmed the wide blanks from 45 to 44
    interior spaces, the narrow blanks from 34 to 33, and re-balanced the
    `Unavailable` row, then verified programmatically that every `┌`/`│`/`└`
    line inside `ux/menubar.md:225-347` is exactly 46 or 36 columns (the two
    annotated in-flight rows carrying `← both disabled` / `← overlay, not
    its own reason` outside the box are unaffected, matching R11-N1's own
    exclusion of them).
  - **R11-N2** empty specimen copy did not match the fixed string ->
    **Fixed.** `:346` read `No activity today`; the copy table at `:508`
    fixes the current, issue-free form as `No local activity today`. The
    empty specimen shows a current snapshot with no qualifier, so `:508`'s
    form applies, not `:509`'s snapshot-scoped `No activity in this
    snapshot`. Changed to `No local activity today`.

- Verification performed by this repair round, not yet independently
  confirmed: a script re-read every `┌`/`│`/`└` line in the specimen block
  and asserted each is 46 or 36 columns with no exception; this is the same
  check R11-N1's evidence describes doing by inspection, repeated here
  mechanically as a sturdier check for the Re-review to redo rather than
  trust.

- Evidence: `make check-whitespace` passes; `bash scripts/check-topic-docs.sh`
  passes; `git diff --check` passes; no product code, test, or configuration
  was changed. This remains a contract-document repair, and is 15 edited
  lines against R11's own estimate of 14.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No
  Document gate is closed and no commit is authorized by this round.

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md / Round 12
```

## Round 13 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：9/10

✅ 结论：PASS

- Reviewed state: HEAD `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`，工作区未提交；
  `docs/topics/desktop-app/ux/menubar.md` blob
  `5303e0d14556da181632f80ccc802b3f82c3a068`（Round 12 修复后）；
  `docs/topics/desktop-app/architecture.md` blob
  `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`；
  `docs/topics/desktop-app/ux/prototype/desktop-surfaces.html` blob
  `8a8c8e5d16acfa41206ac789429078e92baefe89`。后两个 blob 与 Round 11 评判的
  完全一致，Round 12 未触及。
- Reviewer: claude-code（与 Round 12 修复同一 agent；两条 finding 均以独立测量
  重新判定，而非采信修复轮自述的脚本结果）
- Method: 单 agent 有界复评。对 R11-N1 重新做一次机械测量而不是目视：按东亚宽度
  逐行计算 specimen 块内每一条框线的显示列宽；对 R11-N2 回到 copy 表判定该
  specimen 属于哪一种 `empty` 形态，再比对字符串。随后检查修复是否在别处引入回归
  ——重排空白最容易破坏的是 R9-F1 刚建立的结构与其他固定文案。
- Scope: R11-N1、R11-N2；R9-F1、R9-F2 的回归检查；R3-F1～R3-F6、R5-F1、R5-N1、
  R7-F1 的回归检查
- Evidence: `git rev-parse HEAD`、`git hash-object` 三个 blob 如上；
  `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审的契约。
- Completion evidence: 兼容的 Neo4j `completion-evidence/v1` provider 中
  `desktop-app:architecture.md` 与 `desktop-app:ux/menubar.md` 两个 Document
  WorkUnit 已按本轮的确切内容状态（HEAD 加各自 blob 指纹）记录并复查为
  `VERIFIED`。该 gate 与本 PASS 是两件事，且都不授权任何提交。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **R11-N1 已关闭，机械复核。** 对 `ux/menubar.md:225-350` 内每一条以 `┌`/`│`/
  `└` 起始的行按显示列宽重算，全部恰为 46 列（340 pt 规格）或 36 列（280 pt narrow
  规格），无一例外。唯二超出的两行是 `:312` 与 `:314`——它们的多出部分是框外注解
  `← both disabled` 与 `← overlay, not its own reason`，正是 R11-N1 证据本身排除的
  情形。Round 12 修复面覆盖了 R11-N1 未点名但同源的一处：`:290` 的
  `MODELS … Unavailable` 行原先偏向另一侧，现已与框宽一致；把同类缺陷一次改净而不是
  只改被点名的行，是正确的修复边界处理。
- **R11-N2 已关闭。** `:346` 现为 `No local activity today`。判定路径独立复核：该
  specimen 展示的是 current、无任何 freshness/reachability qualifier 的表面，因此适用
  `:508` 的 `empty`（current）形态，而非 `:509` 的 snapshot 限定形态；改后与
  `:508`、`:518` 三处一致。
- **R9-F1 无回归。** 重排空白后结构完好：`:227` client tabs、`:229-232` magnitude
  hero 与 period switcher、`:234`/`:238`/`:242`/`:246` 四段标题、`:249-252` footer
  的 provider 只读状态与 `⌄`/Refresh/Sessions，均与 `:175-189` 的段落表逐行对应。
  全文件仍无 `✕`、无平铺 `Settings…`/`Quit`。
- **R9-F2 无回归。** Data requirements 的 period switcher 行（`:760`）仍为
  `today`/`7d`/`30d` + 日 `buckets`；prototype blob 未变，`Month` 页签未回流。
- **其余 specimen 文案与 copy 表一致。** 逐串比对：`Loading…`(`:261` / `:497`)、
  `Cannot reach the AgentDeck helper`(`:281` / `:505`)、`Some data unavailable`
  (`:295` / `:507`)、`Switch in progress`(`:314` / `:557`) 全部命中固定串。未再发现
  R11-N2 同类的矛盾。
- **前轮已关闭项无回归。** `architecture.md` blob 与 Round 8 声明、Round 9 与
  Round 11 评判逐字节一致，R3-F1～R3-F5、R5-F1、R7-F1 的关闭证据按同一内容状态复用；
  `ux/menubar.md` 侧的 R3-F6 truth table（`:120-152`）、R7-F1 的 `inFlight`-only
  overlay（`:553-564`）、switch flow 的 `Retry`/`Dismiss`（`:569-591`）文字未变。
  R5-N1 已于 Round 10 以"不复现"关闭，本轮在 `:548-550` 复看仍为单句。

#### 📝 总结

逐条处置：R11-N1 关闭，specimen 框线全部等宽，且修复顺带改净了同源但未被点名的
一行；R11-N2 关闭，empty specimen 改用 copy 表固定的 `No local activity today`。
R9-F1、R9-F2 与 Rounds 5、7、9 已关闭的全部 finding 均无回归。本轮无新发现，
disposition 矩阵内没有任何未关闭项——minor 也没有——因此结论为 PASS。

`architecture.md` 与 `ux/menubar.md` 两份文档的 Document gate 随本轮关闭：前者内容
自 Round 8 起未变且其全部 finding 早已关闭（Round 9 与 Round 11 各确认一次），后者
在本轮闭合最后两项。两份文档的 `Review` 单元格随之勾选。

这两份文档各自就是一个 task——`.agent-instructions/beads.md` 的单一 lifecycle 覆盖
文档、task anchor 与测试，差别只在工作产物是 `.md` 还是代码，不在它经过的状态。
`development-workflow` 对 design/contract/process 目标不发出 Task checkpoint，那是
关于该 Skill 输出的约定，不是"文档未交付"的判断；文档的提交检查点由项目发出，
两个 Beads 文档任务因此进入 `awaiting_commit`，等待被授权的提交才 `closed`。
本轮通过的是文档，不是 `menubar-experience` 这个实现 anchor——它仍无任何实现，
其 `Dev`/`Review` 保持未勾选。

审阅过但判定不成立、记录以免下轮重复：specimen 中若干只出现一次的英文串
（`Cost incomplete`、`Section could not be read this refresh`、`Another operation is
using AgentDeck state.`）在文档内没有对应的 `zh-Hans` 条目。这不是缺陷：`:393-397`
的 Localization 规则把"每条用户可见串都本地化"交给 String Catalog，而 copy 表只固定
那些措辞本身承载真实性要求的串（surface、qualifier、update check）。段级
unavailable 标签与失败原因文案属派生内容，文档没有把它们声明为固定串，因此
specimen 使用示例文本不构成与固定串的矛盾——这正是 R11-N2 与本项的区别。

残余不确定性：本轮全部结论绑定在未提交的工作区内容上（HEAD 加 blob 指纹）。
一次授权提交之后，CEv1 证据需按不可变 Git tree 重新记录；在那之前 gate 绑定的是
候选状态而非提交状态。

证据：`git rev-parse HEAD` -> `8beacdb1a412fc4cbe59f84cbe76512ee2c41025`；
`git hash-object` -> `ux/menubar.md` `5303e0d1…`、`architecture.md` `e23ccc7c…`、
`prototype/desktop-surfaces.html` `8a8c8e5d…`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
评审：desktop-app / ux/widget.md
```

## Round 14 — 2026-08-17（更正轮：Round 13 的 PASS 被推翻）

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`；
  `ux/menubar.md` blob `5303e0d14556da181632f80ccc802b3f82c3a068`；
  `architecture.md` blob `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`。内容与
  Round 13 判定的完全一致——本轮改变的不是内容，是对它的判断。
- 来源：`ux/widget.md` 的 Round 1 评审（`reviews/ux-widget.md`）。它的 W-F1 在
  `architecture.md` 的投影清单上撞到同一处缺口，因此顺带证伪了本记录 Round 13
  的一项前提。本条不改写 Round 13 已写下的任何内容。

- **被推翻的结论：`ux/menubar.md:760` 的 period switcher 行不成立。**
  该行称 `today`/`7d`/`30d` 的期间选择"backed by the daily `buckets` series"，
  并位于表头声明"These are provisioned as of `architecture.md`'s 2026-08-17
  revision"之下。回到投影清单核验：`architecture.md:438-456` 只有一组
  "aggregate usage totals" 与"a bounded daily series … plus **the period's**
  `peak` bucket and average"——单数的"the period"；全文件再无第二处 period 供给，
  `:385-396` 的 helper execution contract 也没有 period 参数。因此三个期间的
  hero 数值只能由 Swift 侧对日桶求和得到，而 `requirements.md:132-133` 与
  `ux/menubar.md:55` 都禁止在 Swift 侧再做一层聚合。
- **这正是 R9-F2 的实质，而 Round 10 的修复只处理了它的表象。** R9-F2 是"切换器
  声称的粒度投影没有供给"。把粒度从 week/month 收窄到 7d/30d 之后，"谁生成这三个
  期间"这个问题原样存在，而"backed by the daily buckets series"这句话恰好指向被
  禁止的那条路。
- **Round 13 的失职点，精确地说：** 它核对了该行与 `ux/widget.md:85-86`、
  `requirements.md:75,137` 的措辞一致——三处都写着 today/7d/30d，于是看起来自洽——
  但没有回到投影清单确认"provisioned"这个词成立。三份文档共同引用一个并不存在的
  供给，一致性检查因此全部通过。这与 R9-F2 当初能存活的方式是同一种。
- **后续处置：**
  - `ux/menubar.md` 与 `architecture.md` 的 Document gate 重开，`tasks.md` 的两个
    `Review` 单元格取消勾选，CEv1 两个 WorkUnit 改回 `FAILED` 并记录理由；
  - commit `10ce01e` 不回滚。它记录的修复本身为真（specimen 重画、`Month` 残留
    清除、框线与文案对齐），错的只是"因此可以关闭 gate"这一步；已提交的事实由
    Git 历史承担，本记录承担判断的更正；
  - 修复归口在 `reviews/ux-widget.md` 的 W-F1，因为根因在投影而不在任一 surface。
    投影一旦决定是否承载三个期间，本文档的 `:760` 行随之成立或需要重新设计。
- Evidence: `architecture.md:438-456`、`:385-396`；`requirements.md:132-133`；
  `ux/menubar.md:55`、`:760`；`ux/prototype/desktop-surfaces.html:591-622`
  把同一缺口画实（widget magnitude 同屏列出三个期间的 cost 与 tokens）。
- Verdict: REOPEN — Round 13 的 PASS 撤回，两份文档等待 W-F1 的决定后重新复评。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F1 W-F2 W-F3 W-F4
```

**附记（2026-08-17，非独立评审轮次）。** 上述"下一步"指向的修复已在
`reviews/ux-widget.md` Round 2 完成：`architecture.md` 的投影扩展为承载
`today`/`7d`/`30d` 三期间的 totals 与 per-period top-N model shares（用户在
W-F1 的两条有界路径间选择了"扩展投影"），`ux/menubar.md:760` 的 period
switcher 行随之改写，不再声称由日桶 Swift 侧求和支撑。这是本条被推翻结论的
实际修复，但不构成对本记录的独立复评——`architecture.md` 与 `ux/menubar.md`
的 Document gate 仍按 Round 14 保持重开状态，等待一次针对本记录（连同
`reviews/ux-widget.md`）的独立 Re-review 逐条核实后才能关闭。

## Round 15 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: HEAD `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8`（工作区对本记录
  的两份被评审文档无改动）；`docs/topics/desktop-app/architecture.md` blob
  `165dcc2b26926aeb53da9df362318f4183cd58ac`；
  `docs/topics/desktop-app/ux/menubar.md` blob
  `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`。
- Reviewer: claude-code
- Method: 单 agent 有界复评。先确认 Round 14 重开的实质是否已闭合，再对
  `architecture.md` 自 Round 13 判定状态（blob `e23ccc7c`）以来的全部改动确定影响
  范围，最后把 `ux/menubar.md` 的 Data requirements 逐行映射到**它实际读取的那份
  契约**上——这是本次复评的关键动作，因为 `reviews/ux-widget.md` 的十三轮全部围绕
  投影展开，而菜单栏读的不是投影。
- Scope: Round 14 重开的 period switcher 供给问题；`architecture.md` 七次修订对本
  记录已关闭 finding 的回归风险；归属本记录的两项跨目标记录（"Nine bullets" 编号、
  period switcher 的管辖范围）
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，两个 Document gate 保持 `FAILED`。

#### 🔴 严重问题 — 必须修复

**M-F1 — `docs/topics/desktop-app/ux/menubar.md:752-771`: 菜单栏的 Data
requirements 声称"已供给"，但它逐行指向的是 widget 的 App Group 投影，而菜单栏读的
是 wire snapshot；wire 契约里这些字段一个都没有。**

- 处置：**新发现，且它意味着 Round 14 重开的实质并未闭合，只是被移到了边界的另一
  侧。** `reviews/ux-widget.md` 的十三轮把投影从单期间扩到分期间、分 client scope、
  质量档带金额、rhythm 归约、next refresh time——那些修复对 widget 全部成立，但
  菜单栏一行都用不上。
- 行为风险：`architecture.md:713` 写明"`desktop-widget` reads only the
  presentation-safe App Group projection"，`ux/widget.md:11`、`:258` 同样；投影是
  host **写**给扩展**读**的缓存，菜单栏自身渲染的是 helper 返回的 wire snapshot。
  而 wire 契约（`architecture.md:59-68`）对用量只有一句"bounded usage summary and
  pricing completeness"，`## Menu-bar wire contract extension`（`:736-878`）只增加了
  `provider.candidates`。因此 Round 8 重设计后菜单栏要显示的四段内容，在它实际读取
  的契约里没有供给：实现者只能自行发明 wire 字段，或者让菜单栏去读扩展的缓存——
  后者与 `architecture.md:607` 的"application is the sole cache writer"及整个
  投影最小化立论冲突。
- 证据：`ux/menubar.md:754` 的表头声明"These are provisioned as of
  `architecture.md`'s 2026-08-17 revision"；同表十四行中，下列九行在 wire 契约与
  menu-bar wire extension 里都找不到对应字段——Magnitude hero 的 `totals` 与四个
  token 分量、Period switcher 的三期间、Trend chart 的 ≤90 日桶、`avg/day`/`peak`/
  cache-hit、Composition model rows 的 top-N shares、Composition token split、
  Trust quality rows、Trust coverage 的 ≤12 未定价标识符、Rhythm 的 7×24 网格、
  Client tabs 的 per-client 小计。只有 Footer provider state（`provider.routes`）、
  Switch menu rows（`provider.candidates[].options`）与 Freshness line
  （`generated_at`、`next_refresh_at`）三行有 wire 契约依据。
  最尖锐的一处是 `:760`：它是 Round 14 点名的那一行，修复后写成"backed by **the
  projection's** per-period totals"——把菜单栏的数据来源明确写成了它不读的那份契约。
  另外 `:765` 的 Trust 行写"attribution counts"，而 `:179` 与 specimen `:243-244`
  显示的是金额（`Determinable $11.90`），与 `reviews/ux-widget.md` 的 W-F8 同型，
  只是这一侧连供给方都还没有。
- 💡 有界修复：在 `architecture.md` 的 wire 契约（或 menu-bar wire extension）里
  写明菜单栏四段所需的 usage 结构——三期间 totals 与 token 分量、≤90 日桶与每期间
  `peak`/average、top-N model shares、按质量档的 `(cost, tokens, count, share)`、
  pricing coverage 与 ≤12 未定价标识符、7×24 网格、per-client 小计——并给出与投影
  同类的上界；然后把 `:754` 的表头与 `:760` 的机制描述改为指向 wire snapshot 而
  不是投影，`:765` 改为与展示形状一致的四元组。**不要**把菜单栏改成读投影：那会
  颠覆 `architecture.md:601-626` 的单写者与最小化设计。若 Go 侧计划让同一份聚合
  同时喂 wire 与投影，那正是应当在契约里写出来的事实，而不是留给实现者推断。

#### 🟡 建议改进 — 推荐

**M-F2 — `docs/topics/desktop-app/ux/menubar.md:177,182-183,238`: period switcher
管辖哪些 section 没有写明，而 client tabs 写明了。**

- 处置：**新发现，非阻塞。** 归属本记录，此前在 `reviews/ux-widget.md` 的 Round 3
  与 Round 5 已作为跨目标事项记录。
- 行为风险：`:182-183` 明确 client tabs "filter every section at once"；period
  switcher 只在 `:177` 作为 Magnitude 段的组成部分出现，没有对应句子。按位置读，
  它只管第一段；但 specimen `:238` 的 `MODELS` 行右侧标着 `today`，即 Composition
  段也带期间标签。实现者因此无法判断切到 `30 Days` 时 MODELS 与 ATTRIBUTION 是否
  跟随。这个问题与 M-F1 的供给决定相互影响，但它是呈现范围问题，不会被供给自动
  回答。
- 证据：`:177`、`:182-183`、`:238`、`:242`。
- 💡 有界改进：补一句与 client tabs 对称的声明，写明 period switcher 管辖哪些
  section；若只管 Magnitude，则说明 Composition 与 Trust 固定为当前期间，并让
  specimen 的段标签与之一致。

**M-F3 — `docs/topics/desktop-app/architecture.md:532`: "Nine bullets gained a
client scope" 与清单实际条数及该句自身的枚举都不符。**

- 处置：**仍未关闭。** 该项由 `reviews/ux-widget.md` Round 9 发现并明确归属本记录；
  其后的第七、第八次修订又合并、删除、新增了 bullet，偏差随之扩大。
- 行为风险：不影响任何字段语义，只影响修订记录的可核对性。
- 证据：`:532` 的"Nine"；实际在第六次修订中获得 client scope 的是五条（日序列、
  7×24 网格与其两个归约、model shares、新增的 per-period totals 与 session count、
  pricing coverage），而 `:534-538` 该句自身随后枚举的是六样东西。
- 💡 有界改进：把数字改为与清单一致，或改为不带计数的表述。

#### 🟢 优点

- **Round 13 判定的两侧一致性未被这七次修订破坏。** `architecture.md` 自 blob
  `e23ccc7c` 以来的全部改动集中在两个 hunk（`@@ -436,22` 与 `@@ -464,12`），都落在
  `### Presentation-safe App Group projection` 一节内。switch 命令面、result
  envelope、operation ownership 与 transition 表（`:736-1100`）逐字未动，因此
  R3-F1～R3-F6、R5-F1、R7-F1 的关闭状态可按同一内容状态复用，无需重跑。
- **`ux/menubar.md` 自 Round 13 以来只改了一行**（`:760` 的机制描述），specimen、
  copy 表、truth table、switch flow 全部保持 Round 13 判定时的状态；R9-F1 的四段式
  specimen、R11-N1 的框线、R11-N2 的 `No local activity today` 均无回归。
- **Round 14 的更正本身是对的，并且指向了正确的根因。** 它认定 R9-F2 的实质是
  "谁生成这三个期间"，这一判断在本轮得到进一步印证——修复方向没错，只是落点落在了
  widget 的投影上，而菜单栏读的是另一份契约。
- **投影侧的扩展质量很高**，且其立论（一次性归约、per-scope 上界、scope 内独立
  截断）可以直接复用到 wire 侧，M-F1 的修复因此有现成的形状可循，不需要重新设计。

#### 📝 总结

Round 14 重开的实质**未闭合**：投影被扩了七次，而菜单栏读的是 wire snapshot，
其契约对用量只有一句"bounded usage summary"。`ux/menubar.md` 的 Data requirements
表十四行里有十行在它实际读取的契约中没有对应字段，表头却声明"已供给"，
`:760` 更把数据来源明确写成了它不读的那份契约。这就是 R9-F2 的实质第三次以不同
形态出现：第一次是粒度（week/month），第二次是"谁生成三个期间"，这一次是"从哪份
契约取"。

值得记下的教训与 `reviews/ux-widget.md` 收敛时那条不同：那边的判据是"文档说要读的
每一处都有行"，而这里暴露的是更前一步的问题——**核对一行是否"已供给"之前，先要确定
这份 surface 读的是哪一份契约**。widget 与 menu bar 共处一个 topic、共用一套词汇、
甚至共用四段式结构，但数据路径完全不同：一个读 host 写出的 App Group 缓存，一个读
helper 返回的 wire snapshot。十三轮修复全部作用在前者，而本记录的两份文档一份是
后者的消费者、一份是两者的定义者。

本轮为 FAIL，两个 Document gate 保持未关闭。M-F1 的修复落在 `architecture.md` 的
wire 契约，形状可直接沿用投影侧已经论证过的那套；M-F2 与 M-F3 各是一处改动。

残余不确定性：M-F1 的修复规模取决于一个本轮未判定的问题——Go 侧是否让同一份聚合
同时喂 wire snapshot 与 App Group 投影。若是，wire 契约的补写主要是把既有事实写明；
若不是，则需要各自定义上界与截断规则。该决定属 `architecture.md`，不由本轮代为
选择。

证据：`git rev-parse HEAD` -> `9e2a5c43ccd07813fe9ac8991aaba8b3c876bdd8`；
`git hash-object` -> `architecture.md` `165dcc2b…`、`ux/menubar.md` `6c3bfed4…`；
`git diff e23ccc7c 165dcc2b` 的改动仅落在投影一节的两个 hunk；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / M-F1 M-F2 M-F3
```

## Round 16 — 2026-08-17（修复轮）

- Repair state: HEAD `c7331dbc7761093e3e7af0c19668e74bbfa2e945`，工作区未提交；
  `docs/topics/desktop-app/architecture.md` repair blob
  `75c96aed3524456b9cf0b682d029b1db49aefd33`；
  `docs/topics/desktop-app/ux/menubar.md` repair blob
  `a8e3b555b8f8e989bace08df8816a30ad6e9924d`。
- Repair owner: codex
- Scope: 只修复 Round 15 的 M-F1、M-F2、M-F3；未改动产品代码、测试、配置、
  prototype 或其他已关闭 finding 的合同面。

### Finding-to-change mapping

- **M-F1 repaired in the candidate.** `architecture.md` 的 menu-bar wire
  extension 现在给 `data.usage` 增加向后兼容的 `presentation` 对象。它以精确的
  `{available, items}` family 形状承载 `Client` × `Period` totals、四种 token
  分量、event/session counts、现有 cost tuple、每期间 average/peak/cache-hit、每期间
  top-N model rows、每 client scope 的 ≤90 日桶、current-period quality 四元组与
  pricing coverage、固定 30 日的 7×24 rhythm，以及每期间 per-client subtotals；同时
  固定数组上界、排序、missing-v1 解码和 malformed 类型规则。Go 只聚合一次，host 从
  wire 渲染菜单栏并把 allowlisted 值复制进 widget projection，不再存在菜单栏反读
  projection 或 Swift 二次归约的空间。`ux/menubar.md` 的 Data requirements 表已逐行
  指向该 wire 对象，Trust 行改为实际展示的 `(cost, tokens, count, share)` 形状。
- **M-F2 repaired in the candidate.** `ux/menubar.md` 现在明确 period switcher 同时
  管辖 Magnitude 与 Composition；Trust 固定 current period，Rhythm 固定最近 30 日，
  两者都不随 switcher 改变。这与 specimen 的 `MODELS … today`、Trust 无期间标签、
  `RHYTHM … last 30 days` 一致。
- **M-F3 repaired in the candidate.** 第六次 projection 修订改为不带陈旧数字的
  “usage families gained a client scope”，不再让标题数字、正文枚举和当前 bullet 数
  彼此矛盾。

### Verification and status

- Existing wire source was checked before writing: `internal/desktop/desktop.go` keeps
  wire v1 and the current flat usage summary, so `usage.presentation` is an additive
  extension rather than a silent replacement.
- `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。
- Compatible Neo4j `completion-evidence/v1` capability is reachable. Both Document
  WorkUnits were already `FAILED`; this repair does not claim an independent verdict,
  retarget them, or write `VERIFIED` evidence. `architecture.md` and `ux/menubar.md`
  remain unticked and await exact-state Re-review.

Repair status: M-F1、M-F2、M-F3 已在候选文档中完成；Review verdict 仍为 FAIL，
等待独立复评。

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md
```

## Round 33 — 2026-08-20（独立实现复评）

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD
  `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`; current product-scope patch
  SHA-256 `c4629eb2a72fc5e94922e9edae99697d3f8baf048f4256d9321b071f8a4efdf3`.
- Reviewer: Codex
- Method: `development-workflow` REREVIEW，只复核 Round 31 的 R31-F1/F2/F3，
  再将它们的修复路径放回完整 Go/Xcode gate 中。CodeGraph 定位当前
  producer → decoder → ViewModel → chart/test 路径；源码与测试执行结果分别复核。
- Finding dispositions:
  - **R31-F1 — CLOSED.** 用户明确决定合并原 Task 3/4；`tasks.md`、
    `architecture.md`、`docs/README.md`、acceptance link、review history 与 Beads
    现在以合并后 Task 3 `menubar-experience` 拥有完整 producer-to-UI 边界。
  - **R31-F2 — CLOSED.** Go producer 通过 snapshot local `currentHour` 只输出
    `0...through_hour`；chart axis 从实际 bucket IDs 派生中点与右端 `Now`。
    上午 8 点、正午 12 点和日末 23 点的 model/axis tests 通过。
  - **R31-F3 — CLOSED.** Present-null、missing boundary、partial、out-of-range、
    duplicate、descending、post-boundary 和 unavailable-with-items 都被 decoder 拒绝；
    chart/chip 共用同一个 validated family，priciest-hour 忽略零 events 并在同价时
    选较早 hour。
  - **R33-F1 — CLOSED AS REVIEWER ERROR.** 用户指出本 topic 当前临时规则是
    契约与实现冲突时以代码为准。复评不应将已实现的 `.timedOut → .failing`
    报成生产缺陷；topic UX 文档和 stale XCTest 期望已直接与该行为对齐。
- Evidence:
  - `scripts/run-go-test.sh ./internal/usage ./internal/desktop ./cmd/agentdeck`:
    PASS; log `agentdeck-go-test.EbBDGX`.
  - `scripts/run-go-test.sh ./...`: PASS; log `agentdeck-go-test.0TRV7A`.
  - 评审结果纠正后，经授权的非沙箱聚焦 XCTest 1/1 PASS；原完整
    `scripts/test-macos-app.sh` 命令也通过：`AgentDeckSharedTests` 38/38，
    `AgentDeckAppTests` 51/51，`** TEST SUCCEEDED **`。Result bundle:
    `apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.08.20_22-51-34--0700.xcresult`.
  - `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and
    `git diff --check`: PASS.
  - CEv1 fixed-template gate query: Review findings are closed, while Task completion
    remains `BLOCKED` only on the separate required `manual-ux` criterion.

#### 🔴 严重问题 — 必须修复

无。R33-F1 是 reviewer 误用了本 topic 的临时权威顺序，不是产品 finding。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- R31-F1/F2/F3 都已在当前内容状态关闭，并有与每个失效模式对应的直接测试。
- Hourly family 保持 wire v1 additive compatibility；legacy missing 与 present malformed
  现在是两条明确路径。
- App tests 51/51 通过，hourly axis、decoder rejection、shared validity 与
  priciest-hour tie-break 都在完整 Xcode 运行中真正执行。
- Topic UX 契约和 Shared XCTest 期望已与保留的 `.timedOut → .failing`
  代码行为一致，完整 Xcode gate 恢复为 89/89 PASS。

#### 📝 总结

R31-F1/F2/F3 全部关闭，R33-F1 由用户指出为 reviewer 判断错误并关闭。
当前代码、topic 文档、XCTest 期望和完整 Go/Xcode gate 一致，所以本轮
Review verdict 直接更正为 PASS，无需另起 Repair 或重新复评这一 finding。
Task 3 Dev/Review 单元格均勾选；用户对最终 commit tree 明确确认
`manual-ux` 全部通过，CEv1 四项 required criteria 重新查询为 `VERIFIED`。
三笔已授权代码 commit 存在，因此 Beads task 可以关闭。

- Verdict: PASS

#### Task checkpoint

Task checkpoint：Task 3 `menubar-experience` at aggregate commit tree
`0b051d9e32afa6a2b188577ffb2f916547d91ce5` — Review PASS; CEv1 completion
gate `VERIFIED`.

提交建议：已按同一 Task 的三个 partial content states 完成：`9716dc7`
wire/data、`fa7eb1f` runtime/state、`f37328d` menu-bar app/resources/tests。

推送建议：当前未授权推送；提交保留在本地 `main`，后续需单独确认
分支、远端和 commit range。

## Round 32 — 2026-08-20（R31-F2 / R31-F3 修复）

Repair scope 仅限 Round 31 记录的 R31-F2、R31-F3。实现 identity 为 HEAD
`9613498123f00b59d3d4b84fbff71e0f71d6ebd4` 加当前 producer、Swift wire/host、
两组 Swift tests、canonical fixtures、GUI contract golden 与两份契约文档的有序
source manifest；该 manifest 的 SHA-256 为
`15059560d459988807f299e21d3890612c49efbee162f8e5ee1ad58f9ebc1b00`。

### Finding → change

- **R31-F2：CLOSED in Repair candidate.** `usage.presentation.hourly` 增加 producer
  声明的必填 `through_hour`，Go 只生成 `0...through_hour`；canonical 10:00 fixture
  因而是 11 桶而非 24 桶。ViewModel 不再以 `count == 24` 才采用 hourly，chart 的
  `Now` 永远落在 producer 边界的最右端；中点标签随当前小时变化，上午 8、正午 12、
  日末 23 分别验证为 `04:00`、`06:00`、`12:00`。
- **R31-F3：CLOSED in Repair candidate.** `DesktopUsageScopeV1` 自定义 decoder 只把
  field 缺失视为 legacy unavailable，显式 `null` 为 invalid wire；
  `DesktopUsageHourlyV1` 要求 `through_hour` 在 `0...23`，available family 必须精确
  包含 `0...through_hour`，从而拒绝 partial、越界、重复、降序、post-boundary 和
  unavailable-with-items。chart 与 priciest-hour 现在共用这一个已验证 family；chip
  只比较 `events > 0` 的 numeric display cost，并在同价时选择较早小时。

### Verification

- `env GOCACHE=/private/tmp/agent-deck-go-build AGENTDECK_UPDATE_FIXTURES=1 go test
  -mod=vendor ./internal/desktop`：PASS，三份 producer fixture 通过可重现性测试。
- `scripts/run-go-test.sh ./internal/usage -run
  'TestPresentation|TestEmptyConcreteClientScope'`：PASS。
- `scripts/run-go-test.sh ./...` 的首次 final-state run 只有
  `TestIsolatedEndToEndFlow` 因新增 `through_hour` 尚未写入 GUI contract golden 而失败；
  其余包 PASS。随后官方 `UPDATE_AGENTDECK_GOLDEN=1` 路径更新 golden，更新/非更新模式的
  E2E 均 PASS，最终 `scripts/run-go-test.sh ./cmd/agentdeck` 全包 PASS；未变包复用首次
  run 的同内容状态结果。
- `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
  scripts/test-macos-app.sh` 在宿主权限下成功 build；本范围的
  `DesktopWireTests` 11/11、`AgentDeckAppTests` 51/51 PASS，其中新增 malformed family、
  上午/正午/日末轴模型、zero-event/tie-break 与 unavailable-family tests 全部 PASS。
  aggregate 仍由既有 `DesktopRefreshCoordinatorTests` 一条范围外断言阻断：production
  把 `.helper(.timedOut)` 归为 `failing`，该测试仍期望 `offline`；hourly decoder、fixture
  或本轮 host 路径均不参与该分支，本 Repair 未越权修改它。

Verdict: REOPEN — R31-F2、R31-F3 repair complete，等待独立 Re-review；Task 3 的其余
L3/manual acceptance 与既有范围外测试状态不在本轮 disposition 内。

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md
```

## Round 31 — 2026-08-20（实现代码评审）

Checklist: 24/54 complete

Incomplete: `TRACE-1`, `TRACE-3`–`TRACE-9`; `DESIGN-1`, `DESIGN-2`,
`DESIGN-5`, `DESIGN-7`–`DESIGN-9`, `DESIGN-11`; `VERIFY-1`–`VERIFY-15` —
同一 hourly 路径存在两个已证实的用户可见正确性缺陷。原 Task 3/4 ownership
歧义已由用户明确决定合并，并在本轮后的文档修复中关闭；评审仍按项目规则在
决定性代码 blocker 后停止更广的 product verification。修复代码后，下一轮必须
补齐这些条目的实现、验收与门禁映射。

### 📋 独立实现评审 — desktop-app / menubar-experience

📊 总体评分：3/10

✅ 结论：FAIL

- Reviewed state: HEAD
  `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`; tracked Task-4-adjacent patch
  SHA-256 `87e9d0e34bedf269e725660c28ebd1ada8dfb9d4eeae783108051ff8656c9125`;
  current App/AppTests/acceptance source manifest SHA-256
  `4db3e0ec620afd45b4128c37ef2133d3bd7cfdbd618579f221d74cdea90ffb0b`.
- Reviewer: Codex
- Method: `development-workflow` 正式 REVIEW，以 `ln-12-delivery-reviewer` 作为补充
  checklist。CodeGraph 先定位 producer → JSON wire → Swift decoder → ViewModel →
  chart/test 路径，再用聚焦 diff 与权威文档验证。Independent review panel 为
  initial scope-scaled，消耗 1/2 轮：White/facts 和 API/compatibility 两个盲审
  lens；两者均只读，结论由 Blue 直接复核。
- Scope: 合并后的 Task 3 `menubar-experience` 当前未提交实现，及证明其用户结果所必需的
  `usage.presentation` hourly producer/decoder/consumer 路径。排除 Task 4 widget、
  Task 5 distribution、`work-signals` 和其他脏工作树内容。

#### 🔴 严重问题 — 必须修复

**[P1] R31-F1 — [`tasks.md:51`](../tasks.md) / [`architecture.md:815`](../architecture.md):
旧任务分解少覆盖 hourly wire 与其可交付 ownership；已由用户决定关闭。**
- 行为风险：旧文档把 producer/period-scoping 与 UI 拆成 Task 3/4，却没有给后续
  hourly、compact plotted values、rhythm hover values、session project rows、对应 Swift
  DTO/legacy validation 和 UI consumer 一个完整边界。这会让评审无法说明当前
  candidate 应由哪个 Task 原子交付；这是任务文档少做了范围声明，不是代码不得修改
  旧文件。
- 证据：`internal/usage/presentation.go:102-109,294-300,426-459` 生产固定 24 个
  hourly buckets；`DesktopWire.swift:275,430-451` 解码；
  `MenuBarViewModel.swift:452-486` 消费。但 [`architecture.md`](../architecture.md#additive-usagepresentation)
  的字段表只列 daily/rhythm，后续的“Presentation gaps”也未决定 hourly；
  旧 [`tasks.md`](../tasks.md) 将两个 Go 文件交给独立
  `presentation-period-scoping`，而旧 `menubar-experience` Files/Creates 未列它们。
  Round 30 已写下该缺口；用户随后明确决定“文档不对就改文档”，将两个 Task 合并。
💡 Disposition（2026-08-20）：**CLOSED by explicit user decision and documentation
repair.** `tasks.md` 现在以 Task 3 `menubar-experience` 一次性拥有 producer、Go tests、
fixtures/golden、Swift DTO/legacy decode 和 UI consumer；`architecture.md` 增加 hourly
bounds、local-hour、legacy/malformed semantics。原 commit `1bf1f76` 作为合并 Task 的
partial evidence 保留，不再是独立 completion boundary。

**[P1] R31-F2 — [`MenuBarPanelViews.swift:290`](../../../../apps/macos/AgentDeckApp/MenuBarPanelViews.swift):
固定 24 个小时桶把未来的一天末尾标成“现在”。**
- 行为风险：除一天结束前外，图表的最右桶都是 23:00，不是当前小时；中间的
  未来空桶还会压缩已观测趋势。这使时间轴与实际数据窗口同时误导用户。
- 证据：`presentation.go:453-459` 无条件生产 `0...23` 全部桶；
  `MenuBarViewModel.swift:452-476` 仅在数量等于 24 时把全部桶传给 chart；
  `MenuBarPanelViews.swift:290-296` 在最右端固定渲染 `Now` / `现在`。
💡 有界修复：在 [`ux/menubar.md`](../ux/menubar.md) 和 wire 契约中决定唯一语义：
要么 producer/UI 只给出截至当前本地小时的桶，要么保留 24 桶但把 `Now` 标在真实位置且不让
未来桶参与趋势形状；增加上午、正午和日末的渲染/模型验证。
💡 Repair disposition（Round 32）：**CLOSED in candidate**；采用只生成截至
`through_hour` 的方案，动态轴与上午/正午/日末模型测试见 Round 32。

**[P1] R31-F3 — [`DesktopWire.swift:275`](../../../../apps/macos/AgentDeckShared/DesktopWire.swift) /
[`MenuBarViewModel.swift:484`](../../../../apps/macos/AgentDeckApp/MenuBarViewModel.swift): malformed/partial
hourly 会绕过 invalid-wire 语义，并让 chart 与“最贵小时”互相矛盾。**
- 行为风险：optional synthesized `Codable` 把 field 缺失和显式 JSON `null` 都解为
  `nil`，且不验证 hour 范围、顺序和唯一性。例如 23 个桶会让 chart 回退到 daily，
  但 `usageChips` 仍对这 23 个桶做 `max`，生成看似可信的 priciest-hour。
- 证据：`DesktopUsageScopeV1.hourly` 是无自定义 decoder 的 optional field；
  `usesHourly` 只以 `count == 24` 决定 chart，而 `usageChips` 无条件对任意非空数组做 `max`。
  当前 tests 覆盖 missing 和 wrong-type string，未覆盖 `null`、越界/重复 hour 或 partial rows。
  合并后的 [`architecture.md`](../architecture.md#additive-usagepresentation) 允许 host
  只在完整有效 family 上做一次确定性选择，但当前实现没有共享 validity、非零 events
  过滤或相同 cost 下的明确 tie-breaking。
💡 有界修复：区分 legacy missing 与 present-null invalid（JSON `null` 是一个显式值，
参见 [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259)），验证固定范围/顺序/唯一性，并让 chart
与 chip 共用一个 validity decision；按 architecture 只比较有 events 的已观察 bucket，
并实现相同 cost 选择较早 hour 的确定规则。
💡 Repair disposition（Round 32）：**CLOSED in candidate**；严格 decoder、共享
family decision 与 priciest-hour 过滤/tie-break tests 见 Round 32。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- hourly 字段本身保持 additive；old decoder 可忽略，new decoder 可对缺失字段做
  legacy fallback，因此新增 family 本身不要求提高 `wire_version`。
- Go producer 而非 Swift host 生成原始小时聚合，incomplete-cost 标记也穿过
  producer → DTO → presentation 路径。
- 已有 focused producer test 和 missing-family fallback test；它们为修复后的契约补齐提供了
  可用基线。

#### 📝 总结

本轮评审的实现 identity 是上述 HEAD + patch/source-manifest 指纹。用户决定与文档修复
已关闭 R31-F1 的 Task boundary 歧义；hourly 仍存在错误的 `Now` 时间轴和不一致的
malformed/partial 处理，因此不能 PASS。项目规则要求决定性 blocker 后停止更广验证；本轮没有重跑
未改状态的 Go/Xcode 套件。CEv1 provider 已发现，但此 FAIL 不跨过 Task completion
boundary，所以未写入新证据；现有 manual-UX 证据仍是 `BLOCKED`。Beads 任务保持
`in_progress`；由于 completion blocker 尚在，本轮 reviewer claim / `in_review` 转移被安全
策略拒绝，本评审没有绕过该限制。

- Verdict: REOPEN

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / R31-F2 R31-F3
```

## Round 29 — 2026-08-20（原型一致性补充复评）

### 📋 实现复评 — desktop-app / menubar-experience

📊 总体评分：2/10

✅ 结论：FAIL

- Reviewed state: Round 28 安装后的同一个 unsigned candidate，安装位置为
  `/Applications/AgentDeck.app`；本轮未修改 product code、tests 或 configuration。
- Reviewer: Codex，以项目 `REREVIEW` 为主工作流，辅以
  Product Design screenshot audit 对已批准原型和已安装 App 做可视对照。
- Approved prototype evidence:
  - popover：`d47332b110d0…`（420×760）；
  - chart hover：`99c4c493cafc…`（420×760）；
  - provider menu：`1ca14f433887…`（420×760）；
  - Settings：`59f3abb2fad3…`（460×426）。
- Installed-app evidence: user-provided current popover/provider screenshot
  `e9ef148f127d…`（2114×3556）and Settings screenshot
  `0f704b5a24e0…`（920×718）。Round 26 status-item evidence
  `ce5489a4aabb…` remains applicable, and the user independently reports that
  the newly installed status item is still effectively iconless.
- Prototype visual contract observed in the accepted captures: warm orange
  `#f2650f` emphasis; near-black `#0b0e13`, `#141922` and `#1a202a` surfaces;
  restrained indigo/blue chart with an orange peak; compact card hierarchy; a
  24-hour bar chart whose bars expose time/cost/token detail; and a bounded,
  grouped Codex/Claude provider menu.
- Review correction: Round 28's statement **"New findings — none in static or
  automated review"** was too broad. That round proved source, asset, bundle and
  XCTest properties, but it did not compare the installed visual result against
  the approved prototype. Automated green evidence cannot close prototype
  fidelity. This round supersedes that visual conclusion, while preserving the
  valid test results.

- Finding dispositions:
  - **MEI-F1 — CLOSED, unchanged.** Task ownership remains correct.
  - **MEI-F2 — CLOSED, unchanged.** Existing model/state verification remains
    valid; this round found rendered-surface failures, not a regression in that
    closed finding.
  - **[P2] MEI-F3 — STILL OPEN.** Asset registration and compositor tests pass,
    but the newly installed status item still has no recognizable AgentDeck
    glyph at actual menu-bar size. Resource presence is not rendered acceptance.
  - **MEI-F4 — CLOSED.** The latest installed-app screenshot shows the header,
    client/period filters and panel selector in the visible popover; the prior
    shorter-display clipping symptom is no longer present in the supplied state.
  - **[P2] MEI-F5 — STILL OPEN / NOT RE-OBSERVED.** Source and bundle metadata
    are repaired, but this continuation supplied no current About-panel capture.
    Round 28 explicitly required that installed rendering before closure; the
    finding therefore remains open rather than being inferred closed.
  - **[P1] MEI-F6 — NEW: the installed App does not implement the approved
    visual language.** The popover and Settings use saturated macOS system
    blue/teal tint, blue selected controls, native semantic surfaces, larger
    type/spacing and different borders/weights. The approved prototype uses the
    explicit warm-orange and near-black palette above, restrained indigo chart
    color, compact dark cards and a visible header glyph. This is a whole-surface
    contract mismatch, not a subjective request for polish.
  - **[P1] MEI-F7 — NEW: Usage and rhythm visualization structure does not
    match the prototype.** The installed Today panel leaves a large unexplained
    blank region between the panel selector and chart, then renders a sparse,
    right-heavy series whose visible period reads like a 90-day distribution.
    The accepted prototype places a bounded 24-hour chart card directly below
    the notice strip. The installed lower "Last 90 days" area also collapses to
    a label/tiny mark instead of the prototype's legible calendar/rhythm block.
  - **[P1] MEI-F8 — NEW: chart-bar detail interaction is absent.** Every bar in
    the prototype is an interactive target and hover exposes the bucket's hour,
    cost and token/event detail. Current `TrendChart` renders non-interactive
    `RoundedRectangle` views with only accessibility label/value metadata; the
    installed bars expose no hover popover or equivalent visible detail.
  - **[P1] MEI-F9 — NEW: provider selection is structurally and visually wrong.**
    The installed menu expands into an oversized translucent, low-contrast, flat
    option list that extends far outside the compact popover, repeats raw rows
    and makes disabled entries difficult to read. The approved prototype uses a
    bounded menu grouped by Codex and Claude with concise current, ready and
    unavailable states. Current `FooterView` delegates the raw sections to a
    native borderless `Menu`; that implementation does not enforce the approved
    hierarchy, density, contrast or height.
- Launch-at-login disposition: the observed refusal remains contract-conformant
  for this unsigned local build and is not converted into a product finding.
  This does **not** prove the enabled path in a signed distribution; that path
  remains an explicitly environment-limited acceptance item rather than visual
  PASS evidence.
- Evidence limits: screenshots can prove visible color, spacing, clipping,
  hierarchy and the user-observed absence of hover feedback. They cannot prove
  VoiceOver, maximum text size or increased-contrast behavior without changing
  accessibility state; those cases remain governed by the separate acceptance
  runbook and must not be claimed tested here.
- Delivery-reviewer verdict: FAIL
- Verdict: REOPEN

Checklist: 49/54 complete

Incomplete: VERIFY-9 remains open for the installed About panel; the approved
prototype comparison additionally fails the visible glyph, visual-language,
chart-layout, chart-interaction and provider-menu checks recorded below.

#### 🔴 严重问题 — 必须修复

[`apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:1`] MEI-F6：popover 和
Settings 的整体色彩、字重、边框、间距与 header icon 不符合已批准
原型。
- 处置：新增，仍打开。
- 行为风险：产品失去原型已确定的身份、信息层级和状态强调语义；系统
  默认蓝色不是可接受的替代实现。
- 证据：原型 `d47332b110d0…` / `59f3abb2fad3…` 与安装版
  `e9ef148f127d…` / `0f704b5a24e0…` 直接对照；current views 仍依赖
  SwiftUI semantic/tint styling。
💡 有界修复：将原型 palette、surface、type、border、spacing 和 icon
规则落成 Task 4 单一 design-token layer，popover 与 Settings 共用；在相同
viewport/state 下输出 reference-versus-candidate screenshot 作为复评证据。

[`apps/macos/AgentDeckApp/MenuBarPanelViews.swift:100`] MEI-F7：Usage 图表前出现
大段空白，图表的时间结构和下方 rhythm/calendar 都不符合原型。
- 处置：新增，仍打开。
- 行为风险：用户无法在打开 popover 后立即理解 Today 消耗，也无法从
  下方模块读取 90-day rhythm。
- 证据：原型 `d47332b110d0…` 的紧凑 24-hour card 与安装版
  `e9ef148f127d…` 的 blank/sparse layout；current `TrendChart` 只按 bucket tokens
  比例排列 rectangles，未实现原型可见结构。
💡 有界修复：Today 状态固定为紧随 notice 的 24 个小时 bucket card，
消除无所属的空白，并按原型恢复可读的 90-day rhythm/calendar。

[`apps/macos/AgentDeckApp/MenuBarPanelViews.swift:100`] MEI-F8：柱图缺少原型已
定义的 hover/detail 交互。
- 处置：新增，仍打开。
- 行为风险：用户只能看高度，不能将任一柱与具体时间、cost 和
  tokens/events 对应，图表丧失基本可解释性。
- 证据：原型 hover 状态 `99c4c493cafc…`；安装版人工观察；
  `TrendChart:107-114` 只渲染 `RoundedRectangle` 及 accessibility metadata，无
  hover/focus/detail state。
💡 有界修复：每个 bucket 必须是有明确 hit target 的交互元素，
hover 和 keyboard focus 显示 hour/cost/tokens-or-events detail，同一信息通过
accessibility label/value 暴露；复评必须提供 hover 截图与交互测试。

[`apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:393`] MEI-F9：provider 菜单是一个
超长、低对比、平铺的系统列表，不是原型中有界且按 client 分组的选择器。
- 处置：新增，仍打开。
- 行为风险：菜单超出 popover/屏幕，选项层级和可用性不可扫读，disabled 行难以
  辨认，provider switching 主路径不可靠。
- 证据：原型 `1ca14f433887…` 与安装版 `e9ef148f127d…`；
  `FooterView:407-435` 将 sections 直接交给 native borderless `Menu`。
💡 有界修复：实现原型的 compact bounded menu，按 Codex/Claude
分组，每行只呈现一个清晰的 current/ready/unavailable 状态，且整个选择器
不能超出 popover 或 visible screen。

#### 🟡 建议改进 — 推荐

[`apps/macos/AgentDeckApp/MenuBarItemController.swift:55`] MEI-F3：安装版在实际
menu-bar size 下仍无可识别 AgentDeck glyph。
- 处置：仍打开；Round 27/28 的 asset/compositor 证据未通过 rendered gate。
- 证据：Round 26 `ce5489a4aabb…` 与当前用户观察。
💡 有界修复：以原型中的明细 icon 为视觉基准，在实际 1×/2×
status-item 尺寸下调整轮廓、占比和 template rendering，并提供安装后
normal/badged 截图；不得再以 asset lookup 成功代替视觉验收。

[`apps/macos/AgentDeckApp/Info.plist:1`] MEI-F5：About 的 metadata 修复尚未经新
安装版人工观察。
- 处置：仍打开，本轮没有用旧截图推断新 bundle 已通过。
- 证据：Round 28 的显式人工 gate 未完成。
💡 有界修复：打开新安装 bundle 的标准 About panel，确认 icon、
version 0.5.0、build 1 和 copyright 全部可见。

#### 🟢 优点

- MEI-F4 的较短屏 popover 尺寸修复在最新截图中已生效；header 和 filters
  不再被窗口顶部裁掉。
- Round 27/28 的 Xcode tests 仍是有效的非回归证据，但它们不再被解读为原型
  fidelity PASS。

#### 📝 总结

本轮已对每个旧 finding 重新处置：MEI-F1/F2/F4 关闭，MEI-F3 仍打开，
MEI-F5 因缺少新安装版 About 观察而仍打开。与批准原型的直接对照又产生
MEI-F6/F7/F8/F9：视觉语言、图表结构、柱图交互和 provider 选择器都不
符合已批准设计。因此 Task 4 不能 PASS，`Dev`/`Review` 保持未勾选，状态回到
repair-ready。

本轮也修正了评审方法：未来复评必须在相同 viewport 和可比状态下同时
检查 reference 与 candidate screenshot，逐项验收 icon、palette、typography、spacing、
chart structure/interaction 和 provider hierarchy。只跑 XCTest、查 Assets.car 或确认
SwiftUI control 存在，都不再足以宣布原型一致性。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / MEI-F3 MEI-F5 MEI-F6 MEI-F7 MEI-F8 MEI-F9
```

## Round 30 — 2026-08-20

- Repair owner: Codex
- Scope: only MEI-F3, MEI-F5, MEI-F6, MEI-F7, MEI-F8 and MEI-F9 from
  Round 29. No installation, commit, push, provider mutation, login-item change,
  release action or real-user-state probe was authorized or performed.
- Repaired state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7` plus the
  uncommitted Task 4 candidate. The final `Artwork` + `Assets.xcassets`
  manifest digest is
  `6c125812b0bce6bc0e96a2a8a3e4d88386cb339e256ddbe5ba916e3a469c89a4`.
- Method: repository contract/data tracing, local rendering of the approved v7
  prototype in an isolated `agent-browser` session, SwiftUI/AppKit candidate
  rendering over synthetic fixtures, and focused plus full Xcode gates.

- Finding dispositions:
  - **MEI-F3 — implementation repaired; installed-runtime gate BLOCKED.** The
    status-item template now uses the prototype's recognizable robot face rather
    than the deck/negative-space-A mark that remained unreadable at 18 pt. The
    same 18 pt mark appears in the popover header. Normal and badged 2×
    production-compositor attachments were visually inspected; the normal face
    remains recognizable and the badged state retains the face silhouette plus
    a separated warning triangle. Final attachment SHA-256 values are
    `81529c415cb7f01ddc3ff822853608cbd08054f9a46f7dc7d7d5ee5584bac1d8`
    (normal) and
    `80934a1d1123d40fbd9ae8380a72e2170fbb0371c3119810b1f38e338458caf9`
    (badged). Round 29 still requires a newly installed status-item screenshot;
    Repair does not infer that observation from a test renderer.
  - **MEI-F5 — built-bundle rendering closed; installed-runtime gate BLOCKED.**
    The unchanged standard About action was opened inside the built AgentDeck
    test host and captured from the real standard panel. The screenshot visibly
    contains the AppIcon, `AgentDeck`, `Version 0.5.0 (1)`, and the copyright;
    SHA-256
    `a8adae009dfba957dba6a61cc1a56986141ea69d91875c578fe27e76cc170285`.
    Round 29 explicitly asks for the newly installed panel, and this Repair did
    not have installation authority.
  - **MEI-F6 — implementation repaired; installed comparison pending.** Added
    one dynamic light/dark `DesktopVisualTheme` token layer carrying the
    approved orange accent, blue activity, near-black/white surfaces, borders
    and text hierarchy. Popover and Settings now share it; the header carries
    the robot mark, selected controls use orange rather than system blue, panels
    use compact bordered cards, and Settings uses the same surface/type rules.
    The 420×760 popover and 460 pt Settings candidate attachments are
    `d4d64b094ce455ebbc42ce48fdd64a360ba8170d4e361497a4724fa709e65b4c`
    and `95eaab60cdb54105273e5d6ab3b65ac11b60ea11158e3632176e5dd3ec8f99c5`.
    They were compared against the local approved prototype captures
    `72480913693a60d0f1be097c342ed4ebcb9b8073f9f5edcf7e6395583453256c`
    (normal) and
    `19ac20df2c7ade3dec11ecb96eb6b38d8962e3b8a4459951b175c286dbe9289b`
    (hover). Installed same-viewport evidence remains a Re-review prerequisite.
  - **MEI-F7 — honest daily/calendar repair applied; required 24-hour chart
    BLOCKED by the wire contract.** The Usage chart no longer renders all 90
    daily buckets when `today` is selected: it shows only the selected daily
    window, directly below the notice strip, inside a compact chart card. The
    rhythm section now renders the 7×24 intensity grid and the 90 daily values as
    an explicit 18-column calendar rather than compressing them into one tiny
    trend mark. However, the authoritative wire supplies only a bounded daily
    series and a 7×24 *relative-intensity* grid; it supplies no 24-item Today
    series. Creating the prototype's 24 hourly bars here would invent a
    distribution or mislabel 30-day rhythm intensity as today's activity.
  - **MEI-F8 — daily bucket interaction repaired; required hourly detail
    BLOCKED by the same contract gap.** Every available daily bucket is now a
    hit target; hover and keyboard focus show its real date, cost and tokens in
    a fixed readout, click pins/unpins it, arrow keys move the single chart focus,
    and accessibility exposes the same values. `TrendChartInteraction` tests the
    pin > hover > focused-bucket priority. The wire's rhythm cells contain only
    `(weekday, hour, intensity)`, so hour-specific cost/tokens/events cannot be
    displayed honestly until the producer contract changes.
  - **MEI-F9 — implementation repaired; installed comparison pending.** Replaced
    the native flat `Menu` with a custom 250 pt selector whose computed height is
    capped at 260 pt, scrolls within that bound, groups Codex and Claude, and
    gives every row exactly one current/available/unavailable status with higher
    disabled contrast. Its candidate attachment SHA-256 is
    `cdbccbb9bd17618bd15606daec0e1502440d85a7e1f73ea77872943ff7c45fdd`;
    the approved prototype provider capture is
    `09031a1f270b1fc09586bd9430abf95c0ce489d8527c8a26f890e717c8c6d2e1`.

- Contract blocker for MEI-F7/MEI-F8:
  `docs/topics/desktop-app/ux/menubar.md` assigns the trend chart the selected
  scope's ≤90-item `daily` series, and `architecture.md` defines each daily item
  as `(date, tokens, cost, sessions)`. `DesktopUsageRhythmCellV1` contains only
  `weekday`, `hour` and `intensity`. Supplying 24 truthful Today bars with
  hour/cost/tokens-or-events therefore requires an approved additive
  `usage.presentation` hourly family, Go producer/fixture changes, Swift DTOs,
  and a deliberate Task 3/Task 4 ownership decision. That is a new design and
  cross-task scope, not an authorized Task 4 visual Repair.

- Verification:
  - focused model/chrome Xcode tests: 26 tests / 0 failures;
  - `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
    scripts/test-macos-app.sh`: **TEST SUCCEEDED**; Shared 32/32 and App 41/41;
    final result bundle
    `apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.08.20_02-33-01--0700.xcresult`;
  - `make check-whitespace`, `git diff --check`,
    `bash scripts/check-topic-docs.sh`, and `jq empty` on the string catalog:
    PASS on the code state before this record append.
- Final source SHA-256: `MenuBarSurfaceView.swift`
  `9ab304d8c655851197e43cb662d0316c334e81b1abb274fd3ba16d3b74606fef`;
  `MenuBarPanelViews.swift`
  `0d0bf3cd05f4cda011fe49cec96da6b90b378a3b7086303816c6ffad3937af18`;
  `MenuBarViewModel.swift`
  `e2d1ba5a52234c009ffaaff4fe41ee415199f3f5066538b79740dc42bb1b3c3a`;
  `SettingsWindowView.swift`
  `7407f1be565af79d1b61ebc9f13881c40dda302126d4d0f1e1ed435cc0678055`;
  `DesktopCopy.swift`
  `9eeb5649f2b36fafd4a936865ecdde9b94aee8735751a1c4af1d5e1859d63193`;
  `Localizable.xcstrings`
  `1c7bd50a4970a70861be67fa91d2e412663eb1bbfa54dbab89523b8fe5401bfc`;
  `MenuBarChromeTests.swift`
  `b1873392aecd1a0bc66cfa19f48e918be1cb17833e85b8ca0111a1de9820a6fa`;
  `MenuBarViewModelTests.swift`
  `13d857d7c8230740092ffd23db078d85b0c30d0d09d91271013ec192f903dcd2`.
- Repair status: **WORKFLOW_BLOCKED**, not awaiting Re-review. F3/F5/F6/F9
  still need authorized installed-candidate observations, and F7/F8 need the
  approved producer/ownership prerequisite above. Task 4 remains `in_progress`;
  its `Dev` and `Review` cells remain unchecked.

## Round 17 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewed state: HEAD `c7331dbc7761093e3e7af0c19668e74bbfa2e945`（自 Round 15 的
  `9e2a5c43` 以来新增的唯一提交 `c7331db docs: approve switch effectiveness tasks`
  未触及 `docs/topics/desktop-app/`，经 `git diff --stat 9e2a5c4..HEAD --
  docs/topics/desktop-app/` 确认为空）。未提交工作区：`architecture.md` blob
  `75c96aed3524456b9cf0b682d029b1db49aefd33`、`ux/menubar.md` blob
  `a8e3b555b8f8e989bace08df8816a30ad6e9924d`，与 Round 16 声明一致。
- Reviewer: claude-code（Round 16 的修复由 codex 完成）
- Method: 单 agent 有界复评。M-F1 的核验方式与它被发现的方式对称——把
  `ux/menubar.md` 的 Data requirements **逐行**映射到它现在声明要读的那份契约
  （`data.usage.presentation`），确认每行都有对应字段与形状；M-F2、M-F3 各自回到
  被改动的文本核对；随后检查修复自身是否引入新的不一致，重点是散文与 specimen
  的关系——这是本记录出过 R9-F1 的地方。另外核实"additive、不抬 `wire_version`"
  这一断言在当前源码中成立，因为契约对既有实现的断言属必须验证项。
- Scope: M-F1、M-F2、M-F3 的处置；修复引入的回归；对既有 wire 实现的断言
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0；`internal/desktop/desktop.go` 只读检查。
  未改动任何产品代码、测试、配置或被评审的文档。
- Completion evidence: 本轮 FAIL，两个 Document gate 保持 `FAILED`。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

**M-F4 — `docs/topics/desktop-app/ux/menubar.md:190,249`: M-F2 新增的句子声称两个
固定窗口 section 都在标题里写出窗口，但 ATTRIBUTION 的 specimen 标题没有。**

- 处置：**新发现，非阻塞。** 由本轮修复自身引入，属本记录出过的 R9-F1 同类
  （散文与 specimen 对同一界面给出两种说法，而 specimen 是更具体的那个）。
- 行为风险：`:190` 写 "Their headings state those fixed windows"，主语是 Trust 与
  Rhythm 两段。specimen 里 `RHYTHM` 的右栏确实是 `7×24 · last 30 days`
  （`:253`），而 `ATTRIBUTION` 的右栏是 `is it real?`（`:249`）——一句提问，不是窗口。
  于是用户在切到 `30 Days` 之后，Magnitude 与 Composition 都跟着变，Trust 静止在
  当前期间却没有任何可见说明；实现者照 specimen 落地就会得到这个结果，照散文落地
  又会得到另一个。
- 证据：`:185-190` 的新段落；specimen `:249` 与 `:253`；`:179` 的 Sections 表对
  Trust 也未写窗口；`:775-776` 的 Data requirements 两行则明确写了 current-period。
- 💡 有界改进：把 specimen `:249` 的右栏改为窗口说明（与 `RHYTHM` 行同构，例如
  `ATTRIBUTION` 右栏写当前期间），并相应调整 `:179`；或者把 `:190` 改为只声明
  Rhythm 的标题写窗口、Trust 的固定窗口由别处说明。两者都行，但散文与 specimen
  必须一致。

#### 🟢 优点

- **M-F1 已关闭，且修在了正确的一侧。** `architecture.md:743-793` 给 `data.usage`
  增加 `presentation` 对象，明确它是菜单栏四段与投影中用量值的**共同来源**，并写死
  "the menu bar never reads that projection back"。Round 15 指出的十行缺口逐行核对
  已全部落位：hero 与 menu-bar item 读 `scopes[].periods.items[].totals`（含四个
  token 分量、event/session 计数与既有 cost tuple）、period switcher 读三条 period
  记录、trend 读 `scopes[].daily.items[]`（≤90 升序日桶）、chips 读
  `average_per_day`/`peak`/`cache_hit_share`（producer 计算，明确禁止 host 重算）、
  model rows 读 `periods.items[].models[]`（≤12）、Trust 读
  `scopes[].quality.items[]` 的 `(cost tuple, tokens, count, share)` 四元组、
  coverage 读 `scopes[].pricing`、rhythm 读 `scopes[].rhythm`（168 格）、client tabs
  读 `client_subtotals.items[]`（≤6 = 3 期间 × 2 客户端）。`ux/menubar.md:752-780`
  的表头也已改写，明确"App Group projection 是下游 widget 缓存，永远不是菜单栏的
  数据源"，并逐行指向 wire 字段；Trust 行的形状同步改成四元组，与 `:179` 和
  specimen 显示的金额一致——`reviews/ux-widget.md` 的 W-F8 同型问题在这一侧一并
  关闭了。
- **它同时回答了 Round 15 遗留的那个未判定问题。** Round 15 明说规模取决于"Go 侧
  是否让同一份聚合同时喂 wire 与投影"。修复直接把答案写进契约：Go 聚合一次，host
  只做选择、格式化与向投影复制 allowlisted 值，且明确禁止 Swift 侧求和、重新分组、
  自算 share。这既关闭了本 finding，也让投影侧十三轮论证过的形状可以复用而不是
  重来。
- **family 级 `{available, items}` 与向后兼容规则是有备而来的。** 一个 family 不可用
  不擦除其他 section；`available: true` 且 `items` 为空是有效空结果，`false` 是产不出
  来——这正好接上 R3-F6 truth table 里 `partial` 与 `empty` 必须区分的那条。兼容性
  双向写明：producer 恒发、旧 decoder 忽略、新 decoder 把 legacy v1 的缺失对象解码为
  `available: false` 加空 family、类型错误则无效，并保留一份不含 `presentation` 的
  legacy fixture——与 R3-F5 当年确立的方向一致，没有把唯一能坏的方向删掉。
- **"additive、不抬 `wire_version`"这一断言经源码核实成立。**
  `internal/desktop/desktop.go:21` 的 `WireVersion = 1`、`:48` 的 `wire_version`
  字段与 `:69` 起的扁平 `UsageSnapshot` 均在，`presentation` 是并列新增而非替换。
  契约对既有实现的断言为真，不是推测。
- **M-F2 已关闭。** `:185-190` 写明 period switcher 同时管辖 Magnitude 与
  Composition，并给出 Trust 固定当前期间、Rhythm 固定 30 日的理由；与 client tabs
  的 `:182-183` 形成对称声明。剩下的只是标题呈现问题（M-F4）。
- **M-F3 已关闭。** `:532` 改为"The projection's usage families gained a **client
  scope**"，去掉了与清单和自身枚举都对不上的数字。
- **无回归，且改动面被严格限制。** 本次 `architecture.md` 只有两个 hunk
  （`@@ -529,7` 与 `@@ -740,6`）：一句改写加一节新增。投影一节、switch 命令面、
  result envelope、operation ownership 与 transition 表逐字未动，因此
  R3-F1～R3-F6、R5-F1、R7-F1 的关闭状态按同一内容状态复用；`ux/menubar.md` 的
  specimen、copy 表、truth table、switch flow 同样未动，R9-F1、R11-N1、R11-N2
  无回归。

#### 📝 总结

M-F1、M-F2、M-F3 全部关闭。M-F1 的关闭尤其干净：它没有把菜单栏改成读投影去迁就
已有的修复，而是在菜单栏实际读取的那份契约里补齐字段，并把"Go 聚合一次、两个
surface 各取所需"这条边界写死。R9-F2 三次换形态的那条线——粒度、谁生成、从哪份
契约取——到此闭合。

本轮为 FAIL，只因 M-F4：M-F2 的新句子声称 Trust 与 Rhythm 的标题都写出固定窗口，
而 specimen 里只有 RHYTHM 写了。这是本记录第三次出现同一形态（R9-F1、R11-N2、
M-F4）：**散文改了，而依附于它的 specimen 没跟上**。修复是一处 specimen 右栏或一句
散文，二选一。

残余不确定性：`presentation` 的数组上界（3 scopes × 3 periods × 12 models、≤90 日桶、
≤6 client subtotals）在文档层面自洽，但 wire payload 的实际体积同样未按序列化字节
测量——与投影侧留下的是同一项未决，需在实现任务里按真实 payload 复核。另外
`scopes[]` 固定为 `all`/`codex`/`claude` 三条，若将来支持第三个 client，该"恰好三条"
的措辞需要一并修订；本轮按当前产品范围判断，不作预留要求。

证据：`git rev-parse HEAD` -> `c7331dbc7761093e3e7af0c19668e74bbfa2e945`；
`git diff --stat 9e2a5c4..HEAD -- docs/topics/desktop-app/` 为空；
`git hash-object` -> `architecture.md` `75c96aed…`、`ux/menubar.md` `a8e3b555…`；
`internal/desktop/desktop.go:21,48,69` 确认 wire v1 与扁平 usage summary 仍在；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / M-F4
```

## Round 18 — 2026-08-17（修复轮）

- Repair state: HEAD `c7331dbc7761093e3e7af0c19668e74bbfa2e945`，工作区未提交；
  `docs/topics/desktop-app/architecture.md` blob
  `75c96aed3524456b9cf0b682d029b1db49aefd33`（本轮未改）；
  `docs/topics/desktop-app/ux/menubar.md` repair blob
  `a5475e84e77c65a78645a77ce83af36d70c305b5`。
- Repair owner: codex
- Scope: 只修复 Round 17 的 M-F4；未改动 M-F1～M-F3 的已关闭契约、产品代码、
  测试、配置或 prototype。

### Finding-to-change mapping

- **M-F4 repaired in the candidate.** `ux/menubar.md` 的 Sections 表现在把 Trust
  的 amounts 与 pricing coverage 都明确标为 current-period；healthy specimen 的
  `ATTRIBUTION` 标题右栏从 `is it real?` 改为 `today`。因此 M-F2 建立的规则——
  Magnitude 与 Composition 随 period switcher 改变，Trust 固定当前期间，Rhythm
  固定最近 30 日，且两个固定窗口都在 section 标题中可见——在散文、表格和 specimen
  三处一致。
- specimen 行改写后仍与同一框体的其他内容行保持 46 个显示列，没有重开 R11-N1；
  `MODELS … today`、`ATTRIBUTION … today` 与 `RHYTHM … last 30 days` 现在把三个
  section 的期间行为直接并列展示。

### Verification and status

- 定向宽度测量确认改写后的 ATTRIBUTION 行与相邻 specimen 内容行保持相同宽度。
- `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
  `git diff --check` -> exit 0。
- 本轮是 Repair，不是独立复评；两个 Document WorkUnit 继续保持 `FAILED`，
  `architecture.md` 与 `ux/menubar.md` 的 `Review` 单元格继续未勾选。

Repair status: M-F4 已在候选文档中完成；Review verdict 仍为 FAIL，等待独立复评。

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md
```

## Round 19 — 2026-08-17

### 📋 独立复评 — desktop-app / menubar-experience

📊 总体评分：9/10

✅ 结论：PASS

- Reviewed state: HEAD `c7331dbc7761093e3e7af0c19668e74bbfa2e945`；未提交工作区：
  `docs/topics/desktop-app/architecture.md` blob
  `75c96aed3524456b9cf0b682d029b1db49aefd33`（本轮未改，与 Round 17 判定一致）、
  `docs/topics/desktop-app/ux/menubar.md` blob
  `a5475e84e77c65a78645a77ce83af36d70c305b5`、
  `ux/prototype/desktop-surfaces.html` blob `8a8c8e5d…`（未改动）。
- Reviewer: claude-code（Round 18 的修复由 codex 完成）
- Method: 单 agent 有界复评。M-F4 回到散文、Sections 表与 specimen 三处交叉核对，
  并重跑框线宽度测量（改写的是 specimen 内容行，正是 R11-N1 出过问题的地方）。
  随后对本记录**全部十七项** finding 逐条复看当前内容状态，确认无一项残留。
- Scope: M-F4 的处置；R3-F1～R3-F6、R5-F1、R5-N1、R7-F1、R9-F1、R9-F2、R11-N1、
  R11-N2、M-F1～M-F3 的回归
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 兼容的 Neo4j `completion-evidence/v1` provider 中
  `desktop-app:architecture.md` 与 `desktop-app:ux/menubar.md` 两个 Document
  WorkUnit 已按本轮的确切内容状态记录并复查为 `VERIFIED`，绑定的是未提交候选状态
  （HEAD 加各自指纹），授权提交后需按 Git tree 重记。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **M-F4 已关闭，三处一致。** Sections 表 `:179` 的 Trust 行现在把 amounts 与
  pricing coverage 都标为 current-period；specimen `:249` 的 `ATTRIBUTION` 右栏由
  `is it real?` 改为 `today`。于是 `:185-190` 建立的规则在散文、表格、specimen 三处
  同时可读：`MODELS … today` 随 switcher 变，`ATTRIBUTION … today` 固定在当前期间，
  `RHYTHM … last 30 days` 固定在 30 日——切到 `30 Days` 时三者的差异才显现，而这正是
  该规则要让用户看见的东西。
- **改写没有重开 R11-N1。** 独立重跑显示列宽测量：`:225-360` 内全部框线恰为 46 或
  36 列，唯二例外仍是 `:319`、`:321` 两条框外注解行，与 R11-N1 当初的排除一致。
- **十七项 finding 全部关闭，无一残留。** R3-F1（canonical invocation 含
  `--quiet`）、R3-F2（stream/envelope/exit 全分类加 catch-all）、R3-F3（terminal
  state 携带完整 option 与原子 retry）、R3-F4（Go 展开 option，一对一映射调用参数）、
  R3-F5（missing `candidates` 解码为 `[]` 并保留 legacy fixture）、R3-F6（surface/
  qualifier truth table）、R5-F1（`switch_in_flight` 移出 wire，改为 host overlay）、
  R5-N1（不复现，已于 Round 10 以"无缺陷"关闭）、R7-F1（overlay 仅限 `inFlight`）、
  R9-F1（四段式 specimen）、R9-F2（本轮由 M-F1 最终闭合）、R11-N1（框线等宽）、
  R11-N2（`No local activity today`）、M-F1（wire `usage.presentation`）、
  M-F2（switcher 管辖范围）、M-F3（修订编号去掉陈旧计数）、M-F4（标题窗口一致）。
- **R9-F2 那条线到此真正闭合。** 它换过三次形态——粒度（week/month）、谁生成三个
  期间、从哪份契约取——每次都被当作已修复过一次。最终闭合方式是把菜单栏读的那份
  契约补齐，并写死"Go 聚合一次、两个 surface 各读自己的契约"，而不是让某一侧去迁就
  另一侧已有的修复。
- **`ux/widget.md` 的 Round 13 PASS 未被本记录的改动影响。** 独立核实：
  `architecture.md` 自 widget 通过以来的两处改动是 M-F3 的一句改写与新增的
  `## Menu-bar wire contract extension` 小节，投影一节逐字未动，而 widget 只读投影；
  两个文档的证据各自绑定在自己的内容状态上，互不失效。

#### 📝 总结

M-F4 关闭，本记录十七项 finding 全部关闭且无回归，disposition 矩阵内没有任何未关闭
项——minor 也没有——因此结论为 PASS。评审对象是上述 HEAD 与两个 blob。

这份记录走了十九轮。值得留下的判断有两条。其一，`architecture.md` 与
`ux/menubar.md` 始终被当作一个评审对象处理是对的：十七项里有九项跨两份文档，
分开评审会让"契约说 A、界面说 B"这类问题各自看起来自洽。其二，本记录反复栽在同一个
形态上——**散文改了而依附于它的产物没跟上**（R9-F1 的 specimen、R11-N2 的文案、
M-F4 的标题），以及**核对"已供给"时看错了契约**（R9-F2 三次）。前者的防线是每次改
散文都回读它所驱动的 specimen 与 copy；后者的防线是先确定 surface 读哪份契约，再逐行
核对字段。这两条已分别写进两份记录的总结，供后续文档参考。

残余不确定性：`usage.presentation` 与 App Group 投影的数组上界在文档层面自洽，但两者
的实际序列化体积均未测量，需在实现任务里按真实 payload 复核；`scopes[]` 固定三条的
措辞在支持第三个 client 时需要修订；`internal/desktop/desktop.go` 目前只实现扁平
usage summary，`usage.presentation` 属尚未实现的契约扩展，实现任务需一并交付其
fixture 与解码测试。

证据：`git rev-parse HEAD` -> `c7331dbc7761093e3e7af0c19668e74bbfa2e945`；
`git hash-object` -> `architecture.md` `75c96aed…`、`ux/menubar.md` `a5475e84…`；
specimen 框线宽度测量：`:225-360` 内除两条框外注解行外全部为 46 或 36 列；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 Task checkpoint

```text
Task checkpoint：desktop-app / architecture.md（blob 75c96aed）与
                desktop-app / ux/menubar.md（blob a5475e84），Re-review Round 19
                PASS，两个 CEv1 Document gate 均为 VERIFIED
提交建议：architecture.md、ux/menubar.md、reviews/menubar-experience.md、
          tasks.md、docs/README.md —— 按 hunk 排除并行工作的
          v0-5-0-contract/tasks.md 与 docs/README.md 中不属本 task 的行
推送建议：未解析 —— 当前分支 main，项目未授权在此提交或推送；两者均需显式授权
```

#### 📌 下一步

```text
评审：desktop-app / tasks.md
```

## Round 20 — 2026-08-19（首次实现评审）

### 📋 实现评审 — desktop-app / menubar-experience

📊 总体评分：4/10

✅ 结论：FAIL

- Reviewed state: HEAD
  `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`; uncommitted Task 4
  candidate fingerprint
  `5e9d78c222d00cf5a34d8e16d7ae90acf02c0b189018b09a3b8edac43902c8ce`.
- Reviewer: Codex
- Method: Initial implementation Review under `development-workflow`,
  supplemented by `ln-12-delivery-reviewer`. CodeGraph traced filter,
  presentation, switch, item/window and settings paths before focused diff
  inspection. The pass was Blue-only after the exact task-boundary reproducer
  became decisive; repository policy stops broader verification and an
  independent panel after a demonstrated P1 unless the workflow requires it.
- Scope: task 4's app/menu-bar implementation, provider-switch producer and
  decoders, refresh/switch state, settings, localization, Xcode target/scheme,
  tests, fixtures and current Files/Creates boundary. Task 5
  `AppGroupSnapshotStore.swift` and task 6 `scripts/build-macos-app.sh` were
  excluded.
- Findings:
  - **[P1] MEI-F1 — the implementation's core provider-switch and verification
    paths have no Task 4 ownership.** The authoritative task lists app sources,
    `EmbeddedHelperRunner` refresh/state hunks, two Shared test files, the app
    project/scheme hunks, catalogs/assets and App tests. It then explicitly says
    Task 4 does **not** own `AgentDeckShared/DesktopWire.swift`. The current
    implementation nevertheless modifies ten undeclared paths:
    `internal/desktop/desktop.go`, `internal/desktop/desktop_test.go`,
    `AgentDeckShared/DesktopWire.swift`, `AgentDeckTests/DesktopWireTests.swift`,
    `AgentDeckTests/FixtureSupport.swift`, `AgentDeckVerification/main.swift`,
    `cmd/agentdeck/testdata/phase7/gui-json-contract.json`, and the complete,
    partial and empty-client desktop fixtures. These are not incidental changes:
    they implement `provider.candidates`, executable switch options,
    `ProviderUseEnvelopeV1`, sequential helper behavior, and the two-index-plus-
    snapshot verifier that the task's own switch and refresh flow consumes. ->
    Assign every one of these exact files/hunks to Task 4, or relocate the
    provider-switch producer/DTO/fixture delivery to another explicitly named
    prerequisite and update dependencies. Preserve Task 3 period-scoping hunks,
    Task 5 projection work, and Task 6 build work as excluded boundaries.
- Evidence: task 4 definition at `tasks.md:124-201`; `git diff --name-status`
  showed all ten undeclared modified paths; focused diff tied them directly to
  `ProviderCandidate`, `ProviderSwitchOption`, `ProviderUseEnvelopeV1`,
  `providerCandidates`, sequential `behaviors`, and the three-invocation
  verifier. The development handoff itself claims these behaviors as delivered,
  confirming they are required Task 4 content rather than unrelated dirt.
  Required L3 execution remains unavailable: this machine has Command Line
  Tools only, so Xcode/XCTest and the macOS 26 manual checklist were not run;
  full Go L3 evidence was also not supplied. Those gaps do not change this FAIL
  verdict because the atomic-boundary P1 is already decisive.
- Delivery-reviewer verdict: FAIL
- Verdict: REOPEN

Checklist: 52/54 complete

Incomplete: VERIFY-7 — required full Go and Xcode/XCTest gates are not current;
outcome impact: none for this FAIL verdict; exact next action: run the selected
L3 gates after MEI-F1 is repaired and the final task content state is frozen.

Incomplete: VERIFY-9 — macOS 26 rendered, interaction and accessibility
acceptance is unavailable on this machine; outcome impact: the task cannot later
PASS without that external evidence; exact next action: execute and record the
task's manual checklist on macOS 26 after repair.

#### 🔴 严重问题 — 必须修复

[`docs/topics/desktop-app/tasks.md:168`] MEI-F1：Task 4 明确不拥有
`DesktopWire.swift`，但 provider-switch 的 producer、DTO、fixtures、golden、test helper
和 production verifier 共十个修改路径都未进入 Files/Creates。
- 行为风险：按权威文件表提交会漏掉核心 switch wire 与 verifier；按实际工作区提交会越过
  Task 边界并吸收其他任务内容，无法形成可审计、可回退的原子 commit。
- 证据：current diff 中的 `ProviderCandidate`、`ProviderSwitchOption`、
  `ProviderUseEnvelopeV1`、`providerCandidates`、sequential `behaviors` 与三次 helper
  invocation 都来自这些未声明路径。
💡 有界修复：为 Task 4 声明这些精确 files/hunks，或把整条 provider-switch
producer/DTO/fixture 路径迁入一个明确 prerequisite 并同步依赖；不得把 Task 3/5/6 hunks
顺带并入。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- 现有代码与测试意图覆盖 retained snapshot、qualifier 顺序、四 panel filter、unfiltered
  rhythm、producer-computed sessions、incomplete pricing、provider switch、menu-bar value/
  scope、privacy 和 health detail。
- app test target 与 scheme、英语/简体中文 catalog、settings 和 NSStatusItem surface 都已
  出现在候选实现中；这些内容在边界修复后仍需通过实际 Xcode 与 macOS 26 验收。

#### 📝 总结

Round 19 的 PASS 只批准设计/架构文档，本轮是首次实现评审。当前实现包含 Task 4 所需的
主要 surface 和 switch 机制，但其核心 wire/fixture/verifier 路径没有权威 owner，且任务
文本还明确否认其中一个关键文件的 ownership。这个矛盾在 commit 前必须解决，因此本轮
REOPEN。由于已有决定性 P1，未继续扩展静态审查或用不可运行的 L3 evidence 制造额外
finding；修复后复评必须重新核对边界，并完成 Xcode/XCTest 与 macOS 26 checklist。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / MEI-F1
```

## Round 21 — 2026-08-19（修复轮）

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25`. This round's
  only content change is `docs/topics/desktop-app/tasks.md`, blob
  `9ad9ada35202`. No product code, test, fixture, script, or generated artifact
  was touched.
- Repairer: Claude Code
- Method: `修复` under `development-workflow`, scoped to MEI-F1 alone. The
  finding offers two remedies and `tasks.md` carried two authoritative statements
  that contradict each other, so the choice was put to the user rather than
  decided here; the reasoning and the decision are recorded below.
- Scope: task 4's Files/Creates boundary and its ownership statements. Task 3's
  period-scoping hunks, task 5's projection work, and task 6's build work were
  left as excluded boundaries.
- Findings and disposition:
  - **MEI-F1 — CLOSED.** All ten paths the finding named are now declared in task
    4, each scoped to its own hunks so no other task's share is absorbed:
    `internal/desktop/desktop.go` (the three candidate/option types, the
    `Candidates` field, and the five `provider*` functions — explicitly not the
    `SessionsSnapshot` hunks), `internal/desktop/desktop_test.go` (the candidate
    and option-reason tests), `AgentDeckShared/DesktopWire.swift` (the candidate,
    credential and switch-option DTOs, the `candidates` field and decoder, and
    `ProviderUseEnvelopeV1` — explicitly not the quality, pricing and sessions
    DTOs), `AgentDeckTests/DesktopWireTests.swift` (the candidate assertions and
    the `candidates`/`options` malformed-family cases),
    `AgentDeckTests/FixtureSupport.swift` (the multi-behaviour recorder),
    `AgentDeckVerification/main.swift` (the embedded-helper invocation assertions
    only — its four-fixture and presentation/legacy/empty-client assertions were
    declared by task 3 in that task's Round 4), the command-contract golden, and
    the complete, partial and empty-client fixtures.
  - **The contradiction the finding pointed at is removed.** Task 4's list said
    it owned none of `DesktopWire.swift` while its `Contracts:` line owned
    `provider.candidates`, the switch command surface, its result envelope, and
    switch operation ownership. That sentence now states what task 4 owns in that
    file and what it does not, and says plainly which revision was wrong. A task
    cannot own a wire contract and disown the producer, DTOs and fixtures that
    realize it.
  - **Which remedy, and why it was the user's call.** The alternative was to
    relocate the whole provider-switch producer/DTO/fixture delivery into a new
    prerequisite task, which is exactly the shape this topic used when it split
    `presentation-period-scoping` out of the UI task for being "a Go producer
    change with its own fixtures and decoders". That argument applies to
    `provider.candidates` without modification, so the two remedies were not
    equivalent bookkeeping: one edits a list, the other changes the
    decomposition, the matrix, the dependency graph and dispatch. The user chose
    to assign the paths to task 4, on the ground that task 4's `Contracts:` line
    already owns these objects and passed review at Round 19. The alternative and
    its rationale are recorded here so a later reader can see the decision was
    made rather than defaulted into.
  - **Generated artifacts are declared by both tasks on purpose.** The three
    fixtures and the command-contract golden cannot be split by hunk: they are
    regenerated wholesale by `internal/desktop/fixtures_test.go` under
    `AGENTDECK_UPDATE_FIXTURES=1` and by `UPDATE_AGENTDECK_GOLDEN=1`. Both tasks
    therefore list them, whichever commits second regenerates, and task 4's stated
    contribution to their content is the `provider.candidates` object and nothing
    else.
- Evidence: verification level L0, because only a status and ownership document
  changed. `make check-whitespace` clean; `git diff --check` clean;
  `bash scripts/check-topic-docs.sh` clean. A sweep confirms all ten paths named
  by MEI-F1 now appear in task 4's section. Nothing was relocated or deleted:
  `providerCandidates`, `ProviderUseEnvelopeV1` and the fixtures' `candidates`
  object are all still present in the tree. Task 3's 15 reviewed blobs and task
  4's 21 implementation artifacts were recomputed after this edit and are
  byte-identical, so no prior behavioural evidence is invalidated and none was
  rerun.
- Residual risk: unchanged from Round 20 and untouched by this repair. Round 20's
  `VERIFY-7` and `VERIFY-9` stay open — this machine has Command Line Tools only,
  so Xcode/XCTest and the macOS 26 rendered, interaction and accessibility
  checklist cannot be executed, and task 4 cannot reach PASS without that
  external evidence. `AgentDeckVerification/main.swift`'s invocation hunk is now
  declared by task 4, but the file is shared with task 3, so a commit checkpoint
  must stage it by hunk rather than whole.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.
## Round 22 — 2026-08-19（实现复评，外部证据阻塞）

- Reviewed state: HEAD
  `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`; unchanged implementation
  fingerprint
  `758d3d3ce2ce4ed6e7513b81073e911a456635684b114e796bd1d70ef7af09aa`;
  task-boundary blob `9ad9ada35202887d9008a6700ad01a5c1f0ab90b`.
- Reviewer: Codex
- Method: Independent `复评` under `development-workflow`, limited to MEI-F1
  and the required evidence that remained open in Round 20. The ownership text,
  ten changed paths and shared-hunk exclusions were checked directly; available
  L3 commands were then run without modifying product code, tests or
  configuration.
- Scope: MEI-F1 disposition, full Go/vet evidence, Xcode scheme-test
  availability, and the macOS 26 manual acceptance prerequisite.
- Finding disposition:
  - **MEI-F1 — CLOSED.** Task 4 now owns all ten provider-switch and verifier
    paths the finding named, with explicit hunk exclusions for Task 3 period
    DTO/session work and the Task 3 portion of `AgentDeckVerification/main.swift`.
    The former contradiction denying `DesktopWire.swift` ownership is gone.
    Generated fixtures and the golden are explicitly shared, regenerated
    artifacts with task 4's contribution bounded to `provider.candidates`.
  - **New findings — none.** The ownership-only repair did not change any
    implementation artifact, so Round 20's static candidate review remains
    applicable.
- Evidence: `scripts/run-go-test.sh ./...` PASS; `go vet -mod=vendor ./...`
  PASS; host OS `26.7`. `xcodebuild -version` failed because the active
  developer directory is `/Library/Developer/CommandLineTools`, not a full
  Xcode installation. No Xcode/XCTest result exists, and no item in the
  `ux/menubar.md` or `ux/settings.md` macOS 26 manual checklist has an observed
  result.
- Review status: **BLOCKED**. Exact prerequisites:
  1. Install or select a full Xcode developer directory, run
     `bash scripts/test-macos-app.sh`, and retain output that names
     `AgentDeckAppTests` and reports its test count.
  2. On macOS 26, execute and record every manual checklist item from
     `ux/menubar.md` and `ux/settings.md`, including VoiceOver, full keyboard
     access, Reduce Motion, Increase Contrast, Light/Dark, both locales, narrow
     layout, largest Dynamic Type, scrolling, status-item gestures, switch
     single-flight/failure, candidate-discovery failure, settings Escape and
     login-item refusal.

Checklist: 52/54 complete

Incomplete: VERIFY-7 — Xcode/XCTest scheme execution is unavailable until a
full Xcode developer directory is installed or selected.

Incomplete: VERIFY-9 — rendered, interaction and accessibility acceptance has
no observed manual results on macOS 26.

### 📋 复评状态 — desktop-app / menubar-experience

📊 评审状态：BLOCKED（不产生 PASS/FAIL 评分）

#### 🔴 严重问题 — 必须修复

无。MEI-F1 已关闭。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- Provider-switch producer、DTO、fixtures、golden、sequential helper 和 production
  verifier 现在都有明确 Task 4 owner，并按 hunk保护 Task 3 边界。
- full Go 与 vet 在当前实现状态通过；主机本身已是 macOS 26.7，剩余缺口是完整 Xcode 和
  尚未执行的人工验收，而不是新的代码 finding。

#### 📝 总结

MEI-F1 独立复核为 CLOSED，且没有新 finding。Task 4 仍不能获得正式 PASS：其 L3
acceptance 明文要求实际 `AgentDeckAppTests` scheme output 和 macOS 26 rendered/
interaction/accessibility observations，当前两者都不存在。评审保持在 `in_review`；提供这些
证据后应继续本轮复评并复用已通过的 Go/vet 与 unchanged implementation evidence。

#### 📌 继续条件

```text
复评：desktop-app / reviews/menubar-experience.md
```

## Round 23 — 2026-08-19（Xcode 证据恢复后的实现复评）

### 📋 实现复评 — desktop-app / menubar-experience

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: same implementation fingerprint and task-boundary blob as
  Round 22. No source, test, fixture, project or configuration content changed;
  only the Xcode evidence premise was corrected.
- Reviewer: Codex
- Method: Continuation of the active `复评` route after the user established
  that full Xcode is installed. The global `xcode-select` still points at
  Command Line Tools, so the required gate was run without changing system
  configuration by setting
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer` for that command.
- Finding dispositions:
  - **MEI-F1 — CLOSED, unchanged.** Round 22's ownership disposition remains
    valid.
  - **[P1] MEI-F2 — NEW.** The Xcode scheme discovers
    `AgentDeckAppTests`, but its module cannot compile under Swift 6 strict
    concurrency. `AppTestFixtures.swift` declares five static `[String: Any]`
    values — `defaultRoute`, `defaultCandidate`, `defaultSessionItem`,
    `healthyHealth`, and `failingHealth` — whose non-`Sendable` dictionary type
    is treated as shared mutable global state. `SwiftEmitModule` fails before
    any App test executes. -> Isolate the fixture owner or these values to
    `@MainActor` when all consumers are main-actor tests, or replace them with
    immutable `Sendable` typed fixture values. Do not suppress concurrency
    checks with `nonisolated(unsafe)` unless an independently reviewed external
    synchronization invariant actually exists.
- Evidence:
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild -version`
  -> Xcode 26.4 (`17E192`). The same environment running
  `bash scripts/test-macos-app.sh` built `AgentDeck.app` successfully and the
  test dependency graph named both `AgentDeckAppTests` and
  `AgentDeckSharedTests`. The test action then exited 65 with five exact
  concurrency-safety errors at `AppTestFixtures.swift:209,216,235,244,250`;
  testing was cancelled because `AgentDeckAppTests` failed to build. Round 22's
  full Go and vet PASS remain valid. The macOS 26 manual checklist was not
  started after this decisive P1.
- Delivery-reviewer verdict: FAIL
- Verdict: REOPEN

Checklist: 53/54 complete

Incomplete: VERIFY-9 — macOS 26 manual rendered, interaction and accessibility
acceptance remains pending; outcome impact: none for this FAIL verdict; exact
next action: execute it only after the Xcode test target compiles and passes.

#### 🔴 严重问题 — 必须修复

[`apps/macos/AgentDeckAppTests/AppTestFixtures.swift:209`] MEI-F2：五个静态
`[String: Any]` fixture 不是 `Sendable`，Swift 6 concurrency safety 阻止
`AgentDeckAppTests` module 编译。
- 行为风险：项目要求的 App XCTest 一个都没有执行；syntax-only evidence 掩盖了真实
  Xcode target failure。
- 证据：Xcode 26.4 scheme 已发现 `AgentDeckAppTests`，随后
  `SwiftEmitModule` 在 `:209/:216/:235/:244/:250` 报错并以 65 退出。
💡 有界修复：优先按实际 consumer 将 fixture owner/values 设为 `@MainActor`，或改用
不可变 `Sendable` typed fixture；不要用无证据的 `nonisolated(unsafe)` 绕过检查。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- App 本体在完整 Xcode 26.4 下构建成功。
- `AgentDeckAppTests` 已进入 scheme 和 dependency graph；问题是 test source 的真实编译
  缺陷，不再是 target 未注册或环境不可见。
- MEI-F1 ownership 修复与 full Go/vet evidence 保持有效。

#### 📝 总结

用户纠正了环境判断后，Xcode gate 从“不可运行”变成了可复现的实现失败。Task 4 当前
不能 PASS：App test target 在执行任何测试前就因 Swift 6 concurrency safety 编译失败。
本轮 REOPEN 仅要求修复 MEI-F2；修复后重新运行同一 Xcode gate，确认输出包含
`AgentDeckAppTests` 的实际 test count，再进入 macOS 26 手工验收。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / MEI-F2
```

## Round 24 — 2026-08-20（修复轮）

- Reviewed state: HEAD `f5935b6b91b0bfb6580c32d29f6cae15edb5ca25`. Changed files:
  `apps/macos/AgentDeckAppTests/AppTestFixtures.swift` and
  `apps/macos/AgentDeckTests/DesktopWireTests.swift` — both inside task 4's
  boundary as repaired at Round 21.
- Repairer: Claude Code
- Method: `修复` under `development-workflow`, scoped to MEI-F2. The finding's
  own acceptance is an executed Xcode gate, so every step was judged by running
  it rather than by static inspection.
- Scope: the `AgentDeckAppTests` module's compilability and the assertions that
  its first real execution exposed. No product code was changed.
- Findings and disposition:
  - **MEI-F2 — CLOSED.** `WireFixture` is now `@MainActor`, which is the
    finding's first remedy: every consumer is a main-actor test, so isolating the
    fixture owner is what makes the five `[String: Any]` payloads safe.
    `nonisolated(unsafe)` was not used, because no external synchronization
    invariant exists here for it to describe.
  - **A second compile failure was hiding behind the first.** With the
    concurrency errors gone, `SwiftCompile` reached
    `AppTestFixtures.swift:158` and reported "unable to type-check this
    expression in reasonable time" on the single heterogeneous dictionary literal
    that builds the whole envelope. It is now decomposed into named, explicitly
    typed sub-expressions — `subtotalItems`, `sessionPeriodItems`,
    `presentation`, `usage`, `sessions`, `provider`, `snapshot`, `payload`. This
    is part of MEI-F2 rather than a new finding: the finding's stated consequence
    is that no App test executes, and that stayed true until this was fixed too.
  - **Two real test defects, exposed by the first execution of these tests.**
    Both were in the tests, not the product:
    - `DesktopWireTests` asserted the complete fixture's built-in candidate
      offered three options; it offers four. Task 3's fixture repair seeds a
      current route for both clients, so the candidate covers codex and claude,
      each direct and through a wrapper. The bare count is replaced by the option
      identities, because a count hid *which* option changed.
    - `MenuBarViewModelTests` expected `No local activity today` and read
      `No activity in this snapshot`. The product was right: the model's clock
      sat 4.4 days after the fixtures' `generated_at`, so every derived state was
      additionally `aged`, and the contract says `empty` then describes the
      snapshot rather than the day. The clock now sits one minute after
      `generated_at`. Had the assertion been "corrected" to the observed value
      instead, the rule that distinguishes those two strings would have lost its
      only test.
- Evidence: `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
  scripts/test-macos-app.sh` -> `** TEST SUCCEEDED **`, exit 0.
  `Test Suite 'AgentDeckAppTests.xctest' passed` with **34 tests, 0 failures**,
  and `Test Suite 'AgentDeckSharedTests.xctest' passed` with **32 tests, 0
  failures**. That satisfies this task's stated acceptance condition — the gate
  output names `AgentDeckAppTests` and reports its test count — for the first
  time. Three gate runs were made: the first proved the concurrency errors gone
  and surfaced the type-check timeout, the second compiled and executed the
  module and surfaced the two assertion defects, the third is green.
  `make check-whitespace` and `git diff --check` clean. No Go file changed, so
  Round 22's Go and vet evidence is untouched.
- **Correction to a fact several rounds have relied on.** This machine does have
  Xcode: 26.4, build `17E192`, at `/Applications/Xcode.app`. Only `xcode-select`
  points at `CommandLineTools`, which is what earlier rounds — and this
  repairer's own handoffs — read as "Command Line Tools only". `VERIFY-7`'s
  Xcode/XCTest half was therefore never actually blocked, and is now satisfied.
  `VERIFY-9` remains genuinely outstanding: the macOS 26 rendered, interaction
  and accessibility checklist needs a person at the screen, and no toolchain
  discovery changes that.
- Residual risk: `VERIFY-9` is unexecuted, so this task still cannot reach PASS
  on this evidence alone. The App tests assert the view model rather than
  rendered SwiftUI output, so layout, VoiceOver order, contrast and the 280 pt
  narrow bound remain covered only by that manual checklist.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 25 — 2026-08-20（实现复评，等待手工验收）

- Reviewed state: HEAD
  `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7`; task-boundary blob
  `9ad9ada35202887d9008a6700ad01a5c1f0ab90b`; repaired test blobs
  `AppTestFixtures.swift` `a8fbad397428`, `DesktopWireTests.swift`
  `3a9090ef8ebe`, and `MenuBarViewModelTests.swift` `0e8e139ec3f6`.
- Reviewer: Codex
- Method: Independent `复评` under `development-workflow`, re-running the exact
  Xcode gate rather than reusing Round 24's repairer report. Full Xcode was
  selected for the command through `DEVELOPER_DIR`; no global developer setting
  was changed.
- Finding dispositions:
  - **MEI-F1 — CLOSED, unchanged.** Ownership remains complete.
  - **MEI-F2 — CLOSED.** `WireFixture` is main-actor isolated, the large payload
    literal is decomposed into typed subexpressions, and the two test assertions
    exposed by first execution now state stable option identities and use a
    fixture-aligned clock. No unsafe concurrency suppression was introduced.
  - **New findings — none.** The full App and Shared test suites passed.
- Evidence:
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
  scripts/test-macos-app.sh` independently returned exit 0 and
  `** TEST SUCCEEDED **`. `AgentDeckAppTests.xctest` executed **34 tests with 0
  failures**; `AgentDeckSharedTests.xctest` executed **32 tests with 0
  failures**. The app build, catalogs, app test target and scheme registration
  all completed under Xcode 26.4 (`17E192`). Round 22's full Go and vet PASS
  remain reusable because no Go file changed.
- Review status: **BLOCKED only on VERIFY-9.** The task's approved L3 contract
  requires observed results for the macOS 26 manual checklists in
  `ux/menubar.md` and `ux/settings.md`. Unit tests cover model behavior, not
  rendered SwiftUI geometry, VoiceOver reading order, focus visibility,
  contrast, truncation, scrolling or actual status-item gestures.
- Exact prerequisite: a person at the macOS 26 screen must execute and record
  PASS/FAIL for every menu-bar and settings manual item. Any FAIL becomes a new
  implementation finding; all PASS results allow this same re-review to finish
  without rerunning unchanged Go/Xcode evidence.

Checklist: 53/54 complete

Incomplete: VERIFY-9 — macOS 26 rendered, interaction and accessibility manual
acceptance has no observed results.

### 📋 复评状态 — desktop-app / menubar-experience

📊 评审状态：BLOCKED（Xcode/XCTest 已满足，仅等待手工验收）

#### 🔴 严重问题 — 必须修复

无。MEI-F1 与 MEI-F2 均已关闭。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- 完整 Xcode gate 独立通过，App tests 34/34、Shared tests 32/32。
- Swift 6 concurrency fix 使用 `@MainActor` 表达真实隔离，没有以 unsafe escape hatch
  隐藏问题。
- 任务 ownership、full Go、vet、Xcode build/test 和双语言 catalog 编译证据均已齐全。

#### 📝 总结

所有代码 finding 已关闭，且完整 Xcode 证据通过。Task 4 仍不能正式 PASS 的唯一原因是
approved L3 contract 明确要求人机交互与渲染观察，而这些结果尚未记录。任务保持
`in_review`，不应退回开发；完成 macOS 26 手工清单后继续同一复评即可。

#### 📌 继续条件

```text
复评：desktop-app / reviews/menubar-experience.md
```

## Round 26 — 2026-08-20（macOS 26 手工验收）

### 📋 实现复评 — desktop-app / menubar-experience

📊 总体评分：3/10

✅ 结论：FAIL

- Reviewed state: same implementation and test blobs as Round 25; installed
  unsigned local build at `/Applications/AgentDeck.app`, compiled from the
  current candidate with Xcode 26.4.
- Reviewer: Codex, using user-observed macOS 26.7 screenshots from the installed
  application and direct source/contract tracing.
- Method: The user executed the real status-item, popover, Settings and About
  paths. Four screenshots were inspected at original resolution and bound by
  SHA-256: status item `ce5489a4aabb` (222×252), popover
  `067354fcb5da` (2066×1208), Settings `e2c55bb3973b` (920×718), and About
  `9de6a2a401cc` (568×340). CodeGraph and focused source inspection tied the
  observed failures to their owning code.
- Finding dispositions:
  - **MEI-F1 and MEI-F2 — CLOSED, unchanged.** Ownership and Xcode tests remain
    valid.
  - **[P2] MEI-F3 — NEW: the installed menu-bar glyph is not a recognizable
    AgentDeck icon.** The status item shows the three tiny dots contained in the
    current 18×18 template asset beside the cost. The image loaded successfully;
    the defect is the asset's visible mark, scale and silhouette, not a missing
    resource lookup. -> Replace the template asset with a recognizable
    monochrome AgentDeck glyph whose artwork occupies the status-item canvas at
    1×/2×, and add a rendered status-item acceptance image for normal and badged
    states.
  - **[P1] MEI-F4 — NEW: the popover uses the wrong screen to calculate its
    height and is clipped off the top of a shorter secondary display.**
    `MenuBarGeometry.height` reads `NSScreen.main.visibleFrame`, while the
    popover is shown from whichever screen contains the `NSStatusItem`. In the
    observed secondary-screen run, header, client tabs, hero, period switcher
    and panel switcher are above the visible desktop; only the lower content and
    footer remain. The internal `ScrollView` cannot recover controls outside
    the popover window. -> Derive available height from the status item's
    actual screen before `popover.show`, size the popover to that screen's
    visible frame with margin, and keep overflow inside the content scroll view.
    Cover a secondary display shorter than the main display.
  - **[P2] MEI-F5 — NEW: About opens a metadata-empty standard panel.** The
    observed panel shows a generic placeholder icon and only `AgentDeck`, with
    no version, build or copyright. Task 4 owns the About action, `Info.plist`
    and `Assets.xcassets`; the current plist contains only
    `CFBundleDisplayName` and `LSUIElement`, and the catalog has no application
    icon set. -> Supply the Task 4-owned application icon and the stable About
    metadata this surface needs, or explicitly assign version/build metadata to
    the distribution prerequisite and suppress the About item until that
    metadata exists. The delivered About action must not expose an empty shell.
  - **Launch-at-login refusal — PASS, not a finding.** This installed bundle is
    an unsigned local build. `ux/settings.md` explicitly says such a build may
    not register and specifies the observed presentation: switch remains off,
    an inline warning appears, and the rest of Settings stays usable. The
    screenshot matches that contract.
- Evidence: installed-build screenshots above; `MenuBarItemController.swift:55`
  loads and renders the current `AgentDeckMenuBarIcon`; the asset catalog
  contains registered 18×18/36×36 template PNGs whose visible mark is the three
  dots; `MenuBarSurfaceView.swift:58-59` applies a fixed width/height and
  `MenuBarGeometry.height` uses `NSScreen.main` rather than the status item's
  screen; `Info.plist` has only display name and `LSUIElement`; the menu-bar UX
  contract requires a glyph and a standard About panel. Round 25's Xcode
  34/34 + 32/32 PASS remains valid but did not render these surfaces.
- Delivery-reviewer verdict: FAIL
- Verdict: REOPEN

Checklist: 54/54 complete

Incomplete: None — the manual checklist produced current failing evidence
rather than remaining unexecuted.

#### 🔴 严重问题 — 必须修复

[`apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:58`] MEI-F4：popover 高度按
`NSScreen.main` 计算，而窗口实际出现在较矮的副屏，顶部核心 controls 被裁到屏幕外。
- 行为风险：主界面的 header、filters、hero 和 panel switcher 不可见也不可达，popover
  无法完成其基本任务。
- 证据：截图 `067354fcb5da…` 与 `MenuBarGeometry.height` / `.frame(height:)` 路径。
💡 有界修复：按 status item 所在 screen 的 `visibleFrame` 定尺寸，并让 overflow 只发生在
内部 `ScrollView`。

#### 🟡 建议改进 — 推荐

[`apps/macos/AgentDeckApp/MenuBarItemController.swift:55`] MEI-F3：当前 template
asset 实际呈现为三颗小点，不是可识别的 AgentDeck glyph。
- 证据：截图 `ce5489a4aabb…`；资源已加载，问题在 artwork/silhouette。
💡 有界改进：替换为占满 18pt canvas 的清晰单色 glyph，并验证 normal/badged 两态。

[`apps/macos/AgentDeckApp/Info.plist:1`] MEI-F5：标准 About panel 只有通用占位图和应用名。
- 证据：截图 `9de6a2a401cc…`；bundle 缺 app icon、version/build/copyright metadata。
💡 有界改进：补齐 About 所需 icon/metadata，或在 distribution metadata 到位前隐藏 About。

#### 🟢 优点

- unsigned build 的 login-item refusal 状态与 contract 完全一致，Settings 其余 controls
  保持可用。
- 完整 Xcode tests、full Go 和 vet 仍通过；本轮失败来自真实 rendered behavior，正是手工
  L3 gate 应捕获的内容。

#### 📝 总结

macOS 26 手工验收不再是 blocker，而是产生了三个可复现 finding。MEI-F4 使主 popover
在较矮副屏不可用；MEI-F3 与 MEI-F5 分别破坏状态栏身份和 About surface。开机启动拒绝
是 unsigned local build 的正确失败呈现，不列为 defect。本轮 REOPEN，修复范围限定为
这三个 UI/rendering owner。

#### 📌 下一步

```text
修复：desktop-app / reviews/menubar-experience.md / MEI-F3 MEI-F4 MEI-F5
```

## Round 27 — 2026-08-20

- Repair owner: Codex
- Scope: only Round 26 findings MEI-F3, MEI-F4 and MEI-F5. No provider,
  snapshot, settings, widget, release or delivery behavior changed.
- Repaired state: HEAD `1bf1f7647e4aa2449c71b7d900b3f6c8208f97c7` plus the uncommitted
  Task 4 candidate. The final repair files are bound by SHA-256 in the evidence
  below; the complete `Artwork` + `Assets.xcassets` manifest digest is
  `6b9664cdb222ce849fc673dc9ec3c85864073727625ccd099076ba526001f885`.

- Finding dispositions:
  - **MEI-F3 — Fixed, awaiting independent rendered Re-review.** Replaced the
    three-dot 18×18/36×36 template PNGs with the contract's three stacked cards
    and negative-space `A`, generated from a checked-in SVG source. The
    production compositor is now testable with the exact 2× source asset;
    `MenuBarChromeTests` renders normal and badged production variants, asserts
    that the normal silhouette occupies at least 30×30 pixels of its 36×36
    canvas, and preserves both PNGs as `.xcresult` attachments. The badged
    compositor clears a transparent halo before drawing the alert triangle, so
    the black template symbol cannot disappear against the black base mark; the
    test also requires at least 24 opaque base pixels to be cleared. The final
    exported attachments were inspected at nearest-neighbor scale: normal keeps
    the full negative-space `A`, and badged keeps the deck silhouette while the
    separated warning triangle remains visible. The acceptance runbook names
    those attachments as the CI evidence.
  - **MEI-F4 — Fixed, awaiting independent multi-display Re-review.** Before
    `popover.show`, `MenuBarItemController` now reads
    `sender.window?.screen?.visibleFrame.height`, derives the bounded height with
    the existing 72 pt margin, and injects that same value into both the SwiftUI
    root view and `NSPopover.contentSize`. The fixed region's 40% bound derives
    from that injected height, so overflow remains in the existing internal
    scrolling region. A 600 pt secondary display produces a 528 pt popover while
    a 1,200 pt main display remains capped at 760 pt.
  - **MEI-F5 — Fixed, awaiting independent About-panel Re-review.** Added a full
    macOS `AppIcon` set generated from the same deck/`A` mark, selected it through
    the app target's asset-catalog build setting, and supplied
    `CFBundleIconName`, marketing version `0.5.0`, build `1`, and human-readable
    copyright metadata. The standard About action remains standard and now reads
    a complete built bundle rather than opening a metadata-empty shell.

- Test-failure diagnosis during repair: the first rendering test exposed that a
  hosted XCTest process does not resolve the app's named image catalog through
  `NSImage(named:)`, despite the installed app having loaded it and `assetutil`
  proving both renditions are in `Assets.car`. Replacing that optional lookup
  with Xcode's generated force-loading symbol made the test host crash before
  bootstrap. This was classified as a test-harness resource-context defect, not
  a missing asset; production keeps the fail-safe optional lookup and the test
  injects the exact checked-in 2× asset into the production compositor.

- Evidence:
  - `xcodebuild ... -only-testing:AgentDeckAppTests/MenuBarChromeTests test`:
    **TEST SUCCEEDED**, 3 tests, 0 failures.
  - `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
    scripts/test-macos-app.sh`: **TEST SUCCEEDED**;
    `AgentDeckSharedTests` 32 tests / 0 failures and `AgentDeckAppTests` 37 tests
    / 0 failures. Result bundle:
    `apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.08.20_01-07-05--0700.xcresult`.
  - `xcrun assetutil --info .../AgentDeck.app/Contents/Resources/Assets.car`
    lists `AgentDeckMenuBarIcon` at 18×18 and 36×36, and all ten `AppIcon`
    renditions from 16×16 through 1024×1024.
  - `make check-whitespace`, `git diff --check`, and
    `bash scripts/check-topic-docs.sh`: PASS.
  - Final source fingerprints: `project.pbxproj`
    `bf2c2e9f5647987bfa34dcd1e21c86b42b501089eb0211defdd5f3d9fa106f85`;
    `Info.plist`
    `677905266182cfd34816f30c7ed768cb07b8cff130ac1fa92e379bbf86355bd8`;
    `MenuBarItemController.swift`
    `74bf5898a4cc5175142097a1d9ee38e1137e3506ae8340f81cd543fec70370ca`;
    `MenuBarSurfaceView.swift`
    `3822febef2d3670c4b1e543a6cffb5d8c87f50bad787352de4e324e89c9efeb4`;
    `MenuBarChromeTests.swift`
    `3bcecdc48256160941913351075142649c3df1c95d9393d1375e88cb3d5861fd`.

- Remaining gate: an independent Re-review must inspect the new installed
  normal/badged status item, open the popover from the shorter secondary display,
  and open the standard About panel. Repair does not claim those runtime
  observations in advance.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. Task 4's
  `Dev` and `Review` cells remain unchecked.

## Round 28 — 2026-08-20（自动化复评与重新安装）

- Reviewed state: Round 27 final source fingerprints, unchanged after repair;
  Xcode-tested and installed unsigned candidate at `/Applications/AgentDeck.app`.
  Installed binary SHA-256 `3f209bfd77aa7ad82de7a600084b84604528c24cddca12860156a480788dace4`;
  installed `Assets.car` SHA-256
  `0a65eb6bbfbd1d2034bea0a3c15af45bd16c302ae4d04b6ce7634bdcdc952733`.
- Reviewer: Codex
- Method: Independent `复评` of MEI-F3/F4/F5 owning code, assets and bundle
  metadata, followed by the full Xcode gate and replacement installation. The
  prior installed bundle was moved to
  `/private/tmp/agentdeck-install-backup.Yk2U7t/AgentDeck.app`; the new bundle
  was installed and launched without changing global `xcode-select`.
- Finding dispositions:
  - **MEI-F3 — implementation and automated evidence CLOSED; rendered result
    pending user observation.** The checked-in icon now uses the deck/negative-
    space-A mark, fills the 36×36 canvas, and its normal/badged compositor tests
    pass. The final installed status item must still be visually confirmed.
  - **MEI-F4 — implementation and automated evidence CLOSED; multi-display
    result pending user observation.** Popover height is derived from
    `sender.window?.screen?.visibleFrame.height`, injected into the SwiftUI root
    and `contentSize`; the 600pt/1200pt test passes. The shorter real secondary
    display must still show header and filters.
  - **MEI-F5 — implementation and automated evidence CLOSED; About rendering
    pending user observation.** The installed bundle contains `AppIcon`, version
    `0.5.0`, build `1`, and copyright metadata. The standard panel must still be
    opened and inspected.
  - **New findings — none in static or automated review.**
- Evidence:
  `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
  scripts/test-macos-app.sh` -> `** TEST SUCCEEDED **`; Shared 32/32 and App
  37/37, including `MenuBarChromeTests` 3/3. The built menu and application
  artwork were independently viewed before installation. Installed bundle
  metadata contains `CFBundleIconName=AppIcon`,
  `CFBundleShortVersionString=0.5.0`, `CFBundleVersion=1`, and human-readable
  copyright.
- Review status: **BLOCKED only on independent installed-app observations.**
  Required user checks:
  1. normal and badged status-item glyph are recognizable at actual menu-bar size;
  2. on the shorter secondary display, the popover shows header, filters, hero,
     panel switcher and footer, with overflow scrolling internally;
  3. About shows the new application icon, version 0.5.0, build 1 and copyright.

Checklist: 53/54 complete

Incomplete: VERIFY-9 — the newly installed rendered candidate has not yet been
observed by the independent user after MEI-F3/F4/F5 repair.

### 📋 复评状态 — desktop-app / menubar-experience

📊 评审状态：BLOCKED（新版本已安装并启动，等待人工观察）

#### 🔴 严重问题 — 必须修复

无未关闭代码 finding；MEI-F3/F4/F5 等待 rendered confirmation。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- 修复后的完整 Xcode suite 通过，新增 chrome tests 覆盖 glyph、screen-specific height
  和 About metadata。
- 新 bundle 已安全替换安装，旧 bundle 可从临时备份恢复。

#### 📝 总结

三项修复在源码、资源、bundle metadata 和 Xcode tests 层面均关闭。正式 PASS 只等待用户
对刚安装版本的三个实际界面结果；若任一失败则产生新 finding，全部通过则继续同一复评。

#### 📌 继续条件

```text
复评：desktop-app / reviews/menubar-experience.md
```
