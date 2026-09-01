---
status: historical
topic: cli-error-classification
subject: architecture.md
retired: 2026-09-01
---

# Review log — cli-error-classification / architecture.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/architecture.md` blob `adbbe840b00d2573a607d8764577e1eaef95b385`
- Reviewer: Codex
- Method: Single-agent document-contract review using the `development-workflow` design/contract dimensions; reused unchanged current-code evidence from the requirements review, used CodeGraph for the provider-use and store error paths, and performed one bounded raw read when two CodeGraph queries both deferred the same `UseCredential` branch. No implementation-scoring tool was applied to the architecture document.
- Scope: `docs/topics/cli-error-classification/architecture.md`, checked against the approved requirements contract, current provider/store/backup/session/CLI error paths, existing typed errors and error-code mapping, and the topic task boundary.
- Findings:
  - [P1] A1-F1 — `architecture.md:27-48` names four concepts and says “new typed errors” will be mapped, but never specifies the exact stable codes, concrete error identities, package ownership, `errors.Is`/`errors.As` matching contract, or cause/message behavior. The omission is unsafe in the current code: `store.Error.Error()` renders `Code + ": " + Err`, so using the existing wrapper around `sql.ErrNoRows` would reproduce the forbidden storage text, while independent task implementations can invent incompatible codes or types. -> Add an exact catalogue for provider, credential, backup archive, and session: stable `error.code`, owning package/type or sentinel, construction boundary, matching rule, permitted cause/unwrapping, and redacted `Error()` behavior; then map each catalogue entry in `errorCode`.
  - [P1] A1-F2 — `architecture.md:35` classifies “archive path absent or not a readable archive” as one backup not-found condition without separating filesystem absence, other `os.Open` failures, and an opened archive that fails authentication/tar/manifest validation. Current `readEncrypted` returns the raw `os.Open` error but maps authentication and archive-structure failures to the already stable `invalid_backup`; the proposed sentence can therefore reclassify invalid archives as not-found or leave non-absence errno leakage undecided. -> Specify the exhaustive backup classification matrix, preserving `invalid_backup` for authentication/format/content failures, assigning the new not-found code only to the chosen absence condition, and deciding the sanitized code/message for every other open/read failure without exposing the path or errno.
  - [P1] A1-F3 — `architecture.md:45-64` says new errors follow `extension_not_found`, “where the error text is the stable code,” and then gives one generic “names the missing thing” rule. The approved requirements instead place the stable machine code in `error.code`, do not require new messages to repeat it, preserve `extension_not_found: <id>` only as a compatibility exception, keep the existing session sentence, and allow no caller-supplied identifier for backup. The architecture therefore cannot determine the exact constructors or regression assertions without contradicting its upstream contract. -> Make the approved Message identity table normative for the implementation mapping, state explicitly that new `error.message` values need not repeat `error.code`, preserve the extension exception and session text, and define the technical construction/assertion for every row including backup's identifier-free message.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; `git hash-object docs/topics/cli-error-classification/architecture.md` -> `adbbe840b00d2573a607d8764577e1eaef95b385`; CodeGraph and the bounded `UseCredential` read confirmed missing providers propagate `ProviderByName`'s bare `sql.ErrNoRows`, while named credential selection currently returns `ErrInvalidProvider`; `store.Error:133-145` exposes a wrapped cause in its text; unchanged focused evidence confirms `readEncrypted` returns raw `os.Open` failures, wraps authentication/archive failures with `ErrInvalidArchive`, and `errorCode` maps that sentinel to `invalid_backup`. Approved `requirements.md:62-72,89-115,124-135` fixes code placement and per-row message identity. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after this round was recorded.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：5/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:27`] typed error 与 stable code 合同没有落到可实现的类型、值和匹配语义。
- 行为风险：两个实现 task 可以分别发明不兼容的 code/type；若直接复用现有 `store.Error` 包装 `sql.ErrNoRows`，其 `Error()` 会再次把 storage sentinel 拼进 JSON message。
- 证据：`:27-48` 只写“typed error”和“new codes”，没有枚举值、类型、owner、`errors.Is/As` 或 cause 规则；当前 `store.Error:133-145` 输出 `code: cause`。
💡 有界修复：为 provider、credential、backup、session 增加 exact catalogue，逐项定义 code、类型/哨兵、owner、构造边界、匹配/unwrap 和 redacted message，再给出 `errorCode` 映射。

[`architecture.md:35`] backup absence、I/O unreadable 与既有 invalid archive 被合并。
- 行为风险：错误密码、损坏 tar/manifest 可能被误报为 not-found，或非 absence 的 `os.Open` errno 继续泄漏。
- 证据：当前 `readEncrypted` 直接返回 `os.Open` 错误，但 authentication/archive 失败包装 `ErrInvalidArchive`；`errorCode` 已将其映射为 `invalid_backup`。
💡 有界修复：增加完整分类矩阵，明确 absence、其他 open/read failure、authentication、format/content 各自的 code/message，并保留 `invalid_backup` 的既有语义。

[`architecture.md:45`] message 构造规则与已批准 requirements 不一致。
- 行为风险：照 extension pattern 实现会让新 message 重复 stable code，并遗漏 session 既有句子、extension 兼容例外和 backup 无 identifier 的隐私规则。
- 证据：`:45-64` 以 “error text is the stable code” 作为新模式；已批准 requirements 明确 stable code 位于 `error.code`，新 message 无须重复，并逐行固定 message identity。
💡 有界修复：让 requirements 的 Message identity 表成为技术映射的规范输入，逐行定义 constructor/assertion，明确 extension 仅为兼容例外。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 根因陈述经当前源码验证：provider lookup 的 bare `sql.ErrNoRows` 确实穿透到 CLI fallback。
- 领域而非 CLI 字符串检查拥有分类决定，是正确的依赖方向。
- 规格与手册必须在改变 error values 的 task 内同步，避免实现与文档分阶段漂移。

### 📝 摘要

评审对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439` 与 `architecture.md` blob `adbbe840b00d2573a607d8764577e1eaef95b385`。根因和大方向正确，但 exact type/code contract、backup 分类矩阵及 message construction 都仍需实现者自行决定，并且当前文字会与已批准 requirements 或既有 `invalid_backup` 行为冲突。因此本轮为 FAIL/REOPEN；修复范围仅为 A1-F1、A1-F2、A1-F3。

## Round 1 repair — 2026-08-17

- Repaired state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/architecture.md` blob `83d763ac355ecb17a6959c8f63957c5b5a5d50e8`
- Repairer: Claude Code
- Authorized scope: A1-F1, A1-F2, A1-F3.
- Disposition:
  - A1-F1 — CLOSED. A new `Error catalogue` section defines one carrier type,
    `errdefs.NotFound{Code, Message, cause}`, in a new standard-library-only leaf
    package, chosen because `internal/backup` and `internal/session` do not import
    `internal/store` and this contract must not make them. Its `Error()` returns
    only `Message`, so redaction is structural rather than a review habit. A table
    gives each concept its code constant, `error.code` value, preserved cause and
    matching rule. Root cause now records why `store.Error` cannot be reused: its
    `Error()` renders `Code + ": " + Err` and would reintroduce the driver sentinel
    under a better label. The cause is deliberately kept in the chain because
    existing `session show` tests assert `errors.Is(err, sql.ErrNoRows)`.
  - A1-F2 — CLOSED, with one discovery recorded below. A `Backup classification
    matrix` splits absence (`backup_not_found`) from every other `os.Open` failure
    (`backup_unreadable`), and leaves `invalid_backup` with its current meaning and
    text for passphrase, decrypt, tar and manifest failures.
  - A1-F3 — CLOSED. A `Message construction` section makes the approved Message
    identity table normative, states that a new `error.message` does not repeat its
    `error.code`, gives each row its construction, preserves both existing
    `session show` texts and the `extension_not_found: <id>` compatibility shape,
    and records that shape as preserved rather than as the pattern new rows copy.
    `Code mapping` follows: `errorCode` gains one `errors.As` case, not five,
    placed so no currently mapped code changes.
- Discovery, reproduced against the current binary at this HEAD: the requirements
  evidence row for `backup inspect` does not measure the archive path.
  `backup inspect` reads the passphrase before opening anything, and with a
  character-device stdin `term.ReadPassword`'s ioctl fails with ENODEV, rendered on
  Darwin as `operation not supported by device` — the exact recorded string. The
  true absence path, reached with a piped passphrase, returns
  `open /tmp/nonexistent-xyz.adb: no such file or directory` and leaks the full
  caller-supplied path. The matrix therefore also classifies the CLI-boundary
  passphrase-read failure as `passphrase_unreadable`; without it the delivered
  change would leave that evidence row's message exactly as it is today and
  Acceptance could not be met by changing `internal/backup` alone. This is stated
  in the architecture rather than silently implemented: reconciling the
  requirements evidence cell is a requirements amendment this repair did not
  authorize, and the re-review should decide whether to raise one.
- Out of scope, not changed: `tasks.md` still lists task files that predate
  `internal/errdefs` and the CLI passphrase boundary. It is a separate review
  subject and its own round should reconcile the file lists.
- Verification: L0 for the document. Discovery evidence: `go build -mod=vendor
  ./cmd/agentdeck` -> exit 0, then both `backup inspect` invocations above run
  against a temporary state root. `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — 2026-08-17

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/architecture.md` blob `83d763ac355ecb17a6959c8f63957c5b5a5d50e8`
- Reviewer: Codex
- Method: Single-agent independent Re-review of A1-F1, A1-F2, and A1-F3. Inspected the exact repair diff, re-derived each original finding against the approved requirements and unchanged implementation, then used CodeGraph plus one focused call-site read to verify the newly recorded passphrase-before-open discovery.
- Scope: A1-F1, A1-F2, and A1-F3 in `docs/topics/cli-error-classification/architecture.md`, plus regressions and newly blocking contradictions caused by their repair.
- Findings:
  - [closed] A1-F1 — `architecture.md:26-92,170-182` now rejects the leaking `store.Error`, defines `errdefs.NotFound` with a redacted `Error()` and preserved cause, enumerates exact owner constants/codes/matching for the four original concepts, and gives `errorCode` one explicit `errors.As` mapping.
  - [closed] A1-F2 — `architecture.md:94-111` now separates filesystem absence, other `os.Open` failures, and the existing authentication/archive `invalid_backup` branches without changing the latter's semantics.
  - [closed] A1-F3 — `architecture.md:140-168` now makes the approved requirements table normative, keeps stable codes in `error.code`, defines each original row's message construction, and preserves session and extension compatibility shapes.
  - [P1] A2-F1 — `architecture.md:107,113-138,170-190` discovers that the recorded `backup inspect` errno occurs in CLI `readPassphrase` before the archive is opened, correctly says reconciling that evidence is a requirements amendment the topic has not authorized, but then unilaterally mandates a new `passphrase_unreadable` code. That failure is not not-found, is absent from the `errdefs.NotFound` catalogue and Message construction table, and makes “the five new codes above” inaccurate because it is a sixth new code. The design therefore leaves its carrier semantics, message, matching, exit behavior, owning implementation task, test assertion, and upstream acceptance authorization undecided. -> Reopen and repair the requirements boundary so the evidence row distinguishes passphrase input from archive absence and explicitly authorizes or excludes the CLI input failure; if authorized, add its complete type/carrier, code, message, exit-status, task ownership, test, and contract-impact entries without representing a non-not-found error as `NotFound`. If excluded, remove `passphrase_unreadable` from this architecture and make the accepted backup-not-found scenario supply a passphrase so it reaches `os.Open`.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; `git hash-object docs/topics/cli-error-classification/architecture.md` -> `83d763ac355ecb17a6959c8f63957c5b5a5d50e8`; the exact repair diff closes all three Round 1 findings. Current source confirms `backup inspect` calls `readPassphrase` at `cmd/agentdeck/main.go:2703` before `backup.Service.Inspect` at `:2707`; `readPassphrase:4398-4424` returns terminal ioctl errors raw, so the discovery is valid but belongs to an upstream scope decision. Approved requirements currently frame the evidence row as a missing archive and authorize only its per-row message identity. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after this round was recorded.
- Verdict: REOPEN

## 📋 复评报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`architecture.md:130`] 新增的 `passphrase_unreadable` 越过了已批准 requirements，且没有完整技术合同。
- 处置：新增阻断 A2-F1；A1-F1、A1-F2、A1-F3 均已关闭。
- 行为风险：实现会新增一个 requirements 未授权的稳定 code，并用 `NotFound` 表示非 not-found 输入故障；实现者仍需自行决定 message、matching、exit status、task owner 和测试，而 contract impact 还会漏记这个第六个 code。
- 证据：`:130-138` 同时要求新增 code 并承认需要 requirements amendment；`:53-87` 的 `NotFound` catalogue 没有该行；`:145-152` 的 message table 没有该行；`:189` 仍声称只有五个新 code。当前源码确认 passphrase error 发生在 `Inspect/os.Open` 之前。
💡 有界修复：先重开 requirements，明确 passphrase input failure 是纳入还是排除。若纳入，补齐独立 carrier/type、code、message、exit、task、test 和 contract impact；若排除，从架构删除该 code，并让验收场景显式提供 passphrase 后再测 archive absence。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A1-F1 已关闭：type/code/owner/matching/cause/redaction 与 CLI mapping 均已落成可实现合同。
- A1-F2 已关闭：absence、其他 open failure 和既有 `invalid_backup` 已分离。
- A1-F3 已关闭：message construction 与批准的 requirements 完全对齐。
- passphrase-before-open discovery 本身经当前源码与真实运行证据成立，并被诚实记录为上游问题而非静默实现。

### 📝 摘要

复评对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439` 与 `architecture.md` blob `83d763ac355ecb17a6959c8f63957c5b5a5d50e8`。Round 1 三个 finding 均已关闭；但修复发现的 CLI passphrase failure 需要上游 requirements 决定，当前架构却提前把它设计成第六个稳定 code 且未补齐合同，因此 A2-F1 阻断 PASS。本轮为 FAIL/REOPEN，唯一修复范围为 A2-F1。

## Round 2 repair — 2026-08-17

- Repaired state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `architecture.md` blob `4042d7542c595b14b5b408717655b9051f80ac5f`; `requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`
- Repairer: Claude Code
- Authorized scope: A2-F1, including the requirements boundary its bounded fix names.
- Decision: the exclusion branch. `backup inspect`'s passphrase-input failure is
  not a not-found condition, happens in the CLI input layer before any archive
  lookup, and gets no code in this topic.
- Disposition:
  - A2-F1 — CLOSED. `passphrase_unreadable` is gone from the architecture: removed
    from the classification matrix, from `Code mapping`, and from the prose that
    mandated it. `errorCode` now states explicitly that nothing else is added and
    the excluded failure keeps falling through to `runtime_error`. "The five new
    codes above" is accurate again, because the five are `provider_not_found`,
    `credential_not_found`, `backup_not_found`, `backup_unreadable` and
    `session_not_found`, all of which are `NotFound` rows with full catalogue and
    message entries. No non-not-found error is represented as `NotFound`.
  - Upstream, in `requirements.md`: the evidence row now reads
    `backup inspect /tmp/nonexistent.adb`, passphrase supplied ->
    `open /tmp/nonexistent.adb: no such file or directory`, with a dated note
    recording why the original figure measured something else. A new Non-Goal
    excludes the passphrase-input failure by name, states it stays leaking as a
    known residual needing its own topic, and requires every `backup inspect`
    scenario in this topic to supply a passphrase. Acceptance's first bullet now
    says the scenario supplies one, so it exercises the archive lookup.
- Evidence, reproduced against a build of this HEAD with a passphrase supplied:
  absent archive -> `open /tmp/nonexistent-xyz.adb: no such file or directory`;
  mode-000 file -> `open <path>: permission denied`; directory argument ->
  `invalid_backup: authentication`, confirming a directory is not a `backup_unreadable`
  row because `os.Open` succeeds on it. Both leaking rows justify keeping
  `backup_unreadable` alongside `backup_not_found`.
- Verification: L0 for both documents. `go build -mod=vendor ./cmd/agentdeck` -> exit 0;
  `bash scripts/check-topic-docs.sh` -> exit 0; `make check-whitespace` -> exit 0;
  `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 3 — 2026-08-17

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `architecture.md` blob `4042d7542c595b14b5b408717655b9051f80ac5f`; upstream `requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`
- Reviewer: Codex
- Method: Single-agent bounded Re-review of A2-F1. Inspected the exact exclusion-branch repair in both documents and re-read the latest applicable requirements review round and live Beads dependency state before deciding whether the repaired premise was authoritative.
- Scope: A2-F1 in `docs/topics/cli-error-classification/architecture.md`, including the upstream requirements decision that its bounded remediation explicitly required.
- Findings:
  - [closed in architecture, pending upstream authority] A2-F1 — `architecture.md:113-137,176-195` removes `passphrase_unreadable`, excludes the CLI input failure from the carrier and mapping, supplies a passphrase for every archive scenario, and restores the exact five-code contract. The matching `requirements.md` repair re-measures the evidence row and selects the exclusion branch. However, that new requirements blob's latest applicable record is `Round 4 repair` with `Verdict: REOPEN — repair complete, awaiting independent Re-review`; its Beads task is likewise `in_review`. The previous PASS and CEv1 evidence bind blob `9e36fbc...`, not the repaired blob `118c21d...`. Architecture cannot treat the new scope decision as approved until the requirements document independently passes.
- Evidence: `git hash-object docs/topics/cli-error-classification/architecture.md` -> `4042d7542c595b14b5b408717655b9051f80ac5f`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `118c21dbba956b0eed832b20f66946a83bd28db4`; exact diff confirms the architecture repair and requirements amendment agree. `reviews/requirements.md` latest round is the repair-only REOPEN, while live Beads `ad-clierr-doc-req-design` is `in_review`/round-4. The stale `tasks.md` Review check and `docs/README.md` PASS label were corrected to reflect that authority. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after synchronization.
- Verdict: REOPEN — blocked pending independent Re-review PASS of requirements blob `118c21dbba956b0eed832b20f66946a83bd28db4`

## 📋 复评报告

📊 综合评分：9/10

✅ 结论：BLOCKED

### 🔴 严重问题——必须修复

无（针对 architecture 修复内容）。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- A2-F1 的 exclusion branch 在 architecture 中完整实现：不再新增 `passphrase_unreadable`，五个 code、carrier、message table 和 contract impact 重新一致。
- Requirements repair 与 architecture 使用相同的 passphrase-supplied 场景，并诚实保留 CLI input errno 为独立 residual。
- A1-F1、A1-F2、A1-F3 保持关闭，无回归。

### 📝 摘要

Architecture 内容已关闭 A2-F1，但它依赖的新 requirements blob 尚只有 repair REOPEN，没有独立 PASS；旧 PASS/CEv1 绑定的是旧 blob，不能复用为新合同授权。因此本轮不能给出 architecture PASS，也没有新的 architecture repair finding。状态保持 REOPEN/in_review，等待先独立复评 requirements。

## Round 4 — 2026-08-18

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `architecture.md` blob `4042d7542c595b14b5b408717655b9051f80ac5f`; approved upstream `requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`
- Reviewer: Codex
- Method: Single-agent independent Re-review resumed after the Round 3 prerequisite blocker. Reused unchanged implementation and runtime discovery evidence, rechecked the exclusion branch against the now-PASS requirements Round 5, and swept all prior architecture findings for regression.
- Scope: A2-F1 in `docs/topics/cli-error-classification/architecture.md`, its upstream requirements premise, and preservation of A1-F1, A1-F2, and A1-F3.
- Findings:
  - [closed] A2-F1 — `architecture.md:113-137,176-195` excludes the passphrase-input failure, adds no `passphrase_unreadable` code/carrier/mapping, supplies a passphrase for every archive scenario, and keeps the contract at exactly five new codes. Upstream `requirements.md` blob `118c21db...` independently passed Round 5 and carries matching Evidence, Non-Goal, and Acceptance clauses, so the exclusion branch is now authoritative rather than provisional.
  - [closed, no regression] A1-F1 — the exact type/code/owner/matching/cause/redaction catalogue and one-case CLI mapping remain intact.
  - [closed, no regression] A1-F2 — backup absence, other open failures, and existing `invalid_backup` semantics remain separated.
  - [closed, no regression] A1-F3 — the normative per-row message construction remains aligned with the approved requirements, including session and extension compatibility shapes.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; `git hash-object docs/topics/cli-error-classification/architecture.md` -> `4042d7542c595b14b5b408717655b9051f80ac5f`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `118c21dbba956b0eed832b20f66946a83bd28db4`. Requirements Round 5 is PASS, its Review matrix cell is checked, its current-blob CEv1 evidence is VERIFIED, and its Beads task is `awaiting_commit`. Unchanged current source and Round 2 runtime evidence continue to prove passphrase-before-open and the two archive-open branches. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0. CEv1 document gate `cli-error-classification:architecture.md` -> `VERIFIED` for HEAD plus blob `4042d7542c595b14b5b408717655b9051f80ac5f` after current content-state and Round 4 evidence lineage were recorded.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无（针对本文档）。

### 🟢 优点

- A2-F1 已关闭：passphrase input 被明确排除，architecture 不再新增未经授权的 code 或错误 carrier。
- 五个 stable code、type catalogue、backup matrix、message construction、CLI mapping 与 contract impact 完全一致。
- Requirements Round 5 已独立批准相同边界，architecture 不再依赖 provisional premise。
- A1-F1、A1-F2、A1-F3 保持关闭，无回归。

### 📝 摘要

复评对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`、`architecture.md` blob `4042d7542c595b14b5b408717655b9051f80ac5f` 及已批准的上游 requirements blob `118c21dbba956b0eed832b20f66946a83bd28db4`。所有 architecture finding 均已关闭，前置 requirements blocker 已由 Round 5 PASS/CEv1 VERIFIED 解除；未发现新 finding 或剩余不确定性。本轮 PASS。
