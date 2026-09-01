---
status: historical
topic: cli-error-classification
subject: requirements.md
retired: 2026-09-01
---

# Review log — cli-error-classification / requirements.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `2a528bbc5da009d29d1a17f0f4d530b16dabc1e2`
- Reviewer: Codex
- Method: Single-agent document-contract review using the `development-workflow` design/contract dimensions; CodeGraph located the current CLI error mapper and missing-target paths before focused source and contract inspection. No implementation-scoring tool was applied to the requirements document.
- Scope: `docs/topics/cli-error-classification/requirements.md`, checked against the topic status, current `errorCode` behavior, the provider, credential, backup, session, and extension missing-target paths, the existing CLI specification, and repository document-lifecycle rules.
- Findings:
  - [P1] R1-F1 — `requirements.md:62-64,78-80,93-97` gives the storage-text rule two incompatible scopes and leaves the backup message impossible to derive unambiguously. Goals prohibit storage and file-path text in every documented JSON error, while Non-Goals and Acceptance constrain work and regression coverage to the evidence table; an implementation can therefore either satisfy Acceptance while violating the stated Goal elsewhere or expand into the broad audit that Non-Goals excludes. For the backup row, the missing thing is the caller-supplied archive path, yet the same rule requires the message to name the missing thing, contain no filesystem path, and contain “nothing else”; the document does not decide whether a basename, resource kind, or no caller-supplied identifier is permitted. -> Choose table-only or contract-wide scope consistently across Goals, Non-Goals, and Acceptance, then specify the privacy-safe message identity allowed for each table row, especially `backup inspect`, while reconciling “nothing else” with the accepted `extension_not_found: <id>` target shape.
- Evidence: `git rev-parse HEAD` -> `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `2a528bbc5da009d29d1a17f0f4d530b16dabc1e2`; `bash scripts/check-topic-docs.sh` -> exit 0; CodeGraph and focused source inspection confirmed bare `sql.ErrNoRows` propagation for provider and credential lookups, a typed-but-unmapped session error, direct `os.Open` error propagation for backup inspection, the existing `extension_not_found` mapping, and the `runtime_error` fallback; focused specification search confirmed `runtime_error` is not currently documented. `make check-whitespace` and `git diff --check` -> exit 0 after the review artifacts were written.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：7/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:62`] 消息隐私规则的适用范围和 backup 身份表达未决定。
- 行为风险：实现者可以只覆盖证据表并满足 Acceptance，却违反覆盖全部 JSON 错误的 Goal；也可以被迫审计整个 CLI，从而违反 Non-Goals。对于 `backup inspect`，若消息命名缺失的 archive 参数就会包含文件路径，不命名又违反“names the missing thing”，因此测试和实现都必须自行发明合同。
- 证据：`requirements.md:62-64` 覆盖所有 documented JSON contract，`:78-80` 排除 unrelated `runtime_error` 的广泛审计，`:93-97` 又只验收表中命令；当前 `backup.Inspect` 直接传播 `os.Open(path)`，而文档未说明允许 basename、资源种类还是完全不带调用者标识。
💡 有界修复：在 Goals、Non-Goals、Acceptance 中统一选择“仅证据表”或“整个 JSON 错误合同”，并逐行定义允许的隐私安全身份表达，尤其明确 `backup inspect`；同时让“nothing else”与既有 `extension_not_found: <id>` 目标形状一致。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 问题证据把机器可判定的 code 与用户可读 message 分开，能解释为什么仅改善 session 文案仍不够。
- Goals、Non-Goals、Acceptance 都明确保留退出码、stderr、命令形状和 `extension_not_found` 兼容性。
- 当前实现前提经 CodeGraph、聚焦源码和规范检索验证；专用主题文档检查通过。

### 📝 摘要

评审对象为 HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814` 与 `requirements.md` blob `2a528bbc5da009d29d1a17f0f4d530b16dabc1e2`。现状证据、机器错误码目标及兼容边界基本可靠，但消息隐私规则在全局与表内范围之间冲突，且缺失 backup 的可表达身份合同，实现者仍需自行决定产品边界。因此本轮为 FAIL/REOPEN；修复范围仅为 R1-F1。

## Round 1 repair — 2026-08-17

- Repaired state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `7445d5a4a875035ce677afd5167490c1cba6ec83`
- Repairer: Claude Code
- Authorized scope: R1-F1 only.
- Disposition:
  - R1-F1 — CLOSED. Scope is now table-only and consistent in all three places:
    the Goals privacy rule is bound to the evidence-table commands and says
    explicitly that messages outside the table are out of scope, Non-Goals states
    the same boundary instead of the broader "leaked storage text", and Acceptance
    already verified only that table. A new `Message identity` section decides the
    permitted identifier per row: caller-supplied provider name, caller-supplied
    credential reference, none for `backup inspect` (its only identifier is a
    filesystem path, and a basename is still caller-supplied path text),
    `session show`'s existing text, and the unchanged `extension_not_found: <id>`.
    "Nothing else" is restated as "the code plus at most that one identifier",
    which the existing `extension_not_found: <id>` shape satisfies and the new
    rows follow. Acceptance gained a matching bullet.
- Out of scope, not changed: `architecture.md`'s Message rule repeats "names the
  missing thing and nothing else" without the per-row identity table. It is a
  separate review subject and was not touched under this finding; its own round
  should reconcile it with the new section.
- Verification: L0. `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — 2026-08-17

- Reviewed state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `7445d5a4a875035ce677afd5167490c1cba6ec83`
- Reviewer: Codex
- Method: Single-agent bounded Re-review of R1-F1. Inspected the exact requirements repair diff, re-derived both halves of the finding from the repaired document, and reused the unchanged Round 1 implementation evidence. No implementation-scoring tool was applied to the requirements document.
- Scope: R1-F1 in `docs/topics/cli-error-classification/requirements.md`, plus any regression or newly blocking contradiction caused by its repair.
- Findings:
  - [closed] R1-F1 — `requirements.md:62-66,80-83` now consistently limits the privacy and storage-text rule to the evidence-table commands. `requirements.md:93-106,122-123` decides the permitted identifier per row, explicitly gives `backup inspect` no identifier, and reconciles the existing `extension_not_found: <id>` shape with the repaired “nothing else” rule.
  - [P1] R2-F1 — `requirements.md:62-64,98,101-103` gives `session show` two incompatible message contracts. The general rule says every in-scope `error.message` carries its stable code and the summary defines the shape as “the code plus at most this one identifier”; the row-specific rule instead requires the unchanged text `no session "<id>" is known`, which contains the identifier but not a stable code. Preserving that already-approved message violates the general rule, while prefixing it with a new code violates the row and the document's premise that the current message is already good. -> Decide whether the stable code belongs only to `error.code` or must also be repeated in every `error.message`, then make Goals, the complete per-row message shape, and Acceptance agree; preserve the session text only if the general message rule explicitly permits it.
- Evidence: `git rev-parse HEAD` -> `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `7445d5a4a875035ce677afd5167490c1cba6ec83`; the exact repair diff confirms the table-only scope and per-row identity additions; focused inspection of `requirements.md:55-70,87-106,115-125` proves the new session-message contradiction. Round 1 source and specification evidence remains reusable because the repair changed only this requirements document. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after this round was recorded.
- Verdict: REOPEN

## 📋 复评报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:63`] `session show` 同时被要求携带 stable code 和保留不含 code 的现有 message。
- 处置：新增阻断 R2-F1；R1-F1 已关闭。
- 行为风险：保留 `no session "<id>" is known` 会违反通用 message 规则；改成带 `session_not_found` 前缀又会违反逐行合同和“现有文案已经正确”的前提，因此实现与回归测试必须自行选择相互排斥的合同。
- 证据：`requirements.md:62-64` 要求 message 携带 stable code，`:98` 要求保留现有纯文本，`:101-103` 再次把通用形状定义为 code 加至多一个 identifier。
💡 有界修复：决定 stable code 只属于 `error.code`，还是也必须在每条 `error.message` 中重复；随后统一 Goals、逐行完整 message 形状和 Acceptance。只有通用规则明确允许时才保留 session 现有文本。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 的范围冲突已关闭：Goals、Non-Goals 和 Acceptance 现在都只覆盖证据表。
- backup 的隐私选择已经明确为不携带任何调用者标识，basename 也不会被误当作安全信息。
- provider、credential、session、extension 的允许标识已逐行列出，`extension_not_found: <id>` 的既有形状得到保留。

### 📝 摘要

R1-F1 已关闭。复评对象为 HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814` 与 `requirements.md` blob `7445d5a4a875035ce677afd5167490c1cba6ec83`。修复完整解决了原有范围和 backup 身份问题，但新增的通用 message 公式与 `session show` 逐行规则不能同时成立，因此 R2-F1 仍使需求边界无法直接实现，本轮为 FAIL/REOPEN。剩余不确定性仅限 stable code 是否应重复进 `error.message`；修复范围仅为 R2-F1。

## Round 3 — 2026-08-17

- Reviewed state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `7445d5a4a875035ce677afd5167490c1cba6ec83`
- Reviewer: Codex
- Method: Single-agent independent Re-review of R2-F1. Compared the current requirements blob and review history with Round 2, found no intervening repair content, and re-evaluated the cited clauses directly rather than carrying the disposition forward by assumption.
- Scope: R2-F1 in `docs/topics/cli-error-classification/requirements.md`, plus any regression or newly blocking contradiction in the unchanged target.
- Findings:
  - [still open] R2-F1 — `requirements.md:62-64` still requires each in-scope `error.message` to carry its stable code, while `requirements.md:98` still requires `session show` to keep `no session "<id>" is known` and `requirements.md:101-103` still defines the general shape as the code plus at most one identifier. The reviewed blob is identical to Round 2 and no repair round exists, so the two message contracts remain mutually exclusive. -> Decide whether the stable code belongs only to `error.code` or must also be repeated in every `error.message`, then make Goals, the complete per-row message shape, and Acceptance agree; preserve the session text only if the general message rule explicitly permits it.
- Evidence: `git rev-parse HEAD` -> `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `7445d5a4a875035ce677afd5167490c1cba6ec83`, byte-identical to Round 2; the exact current diff contains no later requirements repair, and the review history contains no R2-F1 repair round. Round 2's focused clause evidence therefore remains current. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after this round was recorded.
- Verdict: REOPEN

## 📋 复评报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:63`] `session show` 的 stable-code/message 合同仍然互斥。
- 处置：R2-F1 仍未关闭；未发现 repair content、回归或新的 finding。
- 行为风险：保留 `no session "<id>" is known` 会违反通用 message 规则；添加 stable-code 前缀又会违反逐行合同，因此实现和测试仍需自行选择合同。
- 证据：当前 blob 与 Round 2 相同；`:62-64`、`:98`、`:101-103` 的冲突文字均未变化。
💡 有界修复：决定 stable code 只属于 `error.code`，还是也必须在每条 `error.message` 中重复，并统一 Goals、逐行完整 message 形状和 Acceptance。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 保持关闭，表内范围、backup 隐私选择和逐行 identifier 上限均未回归。
- 本轮没有把未变化的 R2-F1 误报为已修复，也没有扩张到 architecture 或实现范围。

### 📝 摘要

复评对象仍为 HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814` 与 `requirements.md` blob `7445d5a4a875035ce677afd5167490c1cba6ec83`。该 blob 与 Round 2 完全相同，review 历史也没有 R2-F1 repair round；重新检查后，stable code 是否重复进 `error.message` 的冲突仍在。因此本轮为 FAIL/REOPEN，唯一修复范围仍是 R2-F1。

## Round 3 repair — 2026-08-17

- Repaired state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `9e36fbc03021b2f06a92e474fb1fa69ad8d50e01`
- Repairer: Claude Code
- Authorized scope: R2-F1 only.
- Decision: the stable code belongs to `error.code` alone. A message is not
  required to repeat it, because a machine consumer reads the code field while the
  message exists for a human reader, and duplicating the code into it only creates
  two places that can drift.
- Disposition:
  - R2-F1 — CLOSED. The general rule in Goals no longer requires a message to
    carry its code; it requires human-readable text naming the missing thing plus
    at most the row's permitted caller-supplied identifier. The Message identity
    section states the code-placement decision explicitly and its table now gives
    each row a complete message shape rather than only an identifier limit.
    `session show` keeps `no session "<id>" is known` and now satisfies the general
    rule as written instead of contradicting it. `extension_not_found: <id>` is
    recorded as a preserved shape whose leading code is compatibility, not the form
    new rows copy, which keeps it consistent with Non-Goals. Acceptance restates the
    same contract and says explicitly that no message must repeat its `error.code`,
    and the Goals regression-coverage bullet now names the message shape alongside
    the code and the storage-text assertion, so all three places state one contract.
- Out of scope, not changed: `architecture.md`'s Message rule still says only
  "names the missing thing and nothing else" and carries neither the per-row table
  nor the code-placement decision. It remains a separate review subject.
- Verification: L0. `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 4 — 2026-08-17

- Reviewed state: HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `docs/topics/cli-error-classification/requirements.md` blob `9e36fbc03021b2f06a92e474fb1fa69ad8d50e01`
- Reviewer: Codex
- Method: Single-agent independent Re-review of R2-F1. Inspected the exact repair diff, re-derived the code-placement rule across Goals, Message identity, and Acceptance, and swept the repaired clauses for regressions without re-running unchanged implementation checks.
- Scope: R2-F1 in `docs/topics/cli-error-classification/requirements.md`, preservation of R1-F1, and any newly blocking contradiction caused by the repair.
- Findings:
  - [closed] R2-F1 — `requirements.md:62-67,91-97,102-115,131-133` now makes `error.code` the sole required stable-code field and defines `error.message` as human-readable text that need not repeat the code. The complete per-row table preserves `session show`'s existing `no session "<id>" is known`, treats `extension_not_found: <id>` as a compatibility-preserved exception rather than a template, and carries the same rule into Acceptance.
  - [closed, no regression] R1-F1 — `requirements.md:62-67,82-85` still limits the privacy rule to the evidence table, while `requirements.md:102-115,131-133` still gives `backup inspect` no caller-supplied identifier and forbids path, errno, driver, and storage text.
- Evidence: `git rev-parse HEAD` -> `272c09ff0279cb21f0a94a4dba9f4c7c7927f814`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `9e36fbc03021b2f06a92e474fb1fa69ad8d50e01`; the exact repair diff removes both sides of the R2-F1 contradiction and strengthens regression coverage for the message identity rows. Round 1 implementation evidence remains reusable because the repair changes only this requirements document. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0. CEv1 document gate `cli-error-classification:requirements.md` -> `VERIFIED` for HEAD plus blob `9e36fbc03021b2f06a92e474fb1fa69ad8d50e01` after the missing WorkUnit, criterion, content state, and evidence lineage were recorded.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无（针对本文档）。

### 🟢 优点

- R2-F1 已关闭：stable code 的唯一强制位置是 `error.code`，`error.message` 不再承担重复机器码的职责。
- `session show` 的既有正确文案与通用规则、逐行完整 message 形状和 Acceptance 现在完全一致。
- `extension_not_found: <id>` 被明确记录为兼容性保留形状，而不是新 message 必须复制的模板。
- R1-F1 保持关闭，证据表范围和 backup 不回显路径的隐私边界没有回归。

### 📝 摘要

R1-F1 与 R2-F1 均已关闭。复评对象为 HEAD `272c09ff0279cb21f0a94a4dba9f4c7c7927f814` 与 `requirements.md` blob `9e36fbc03021b2f06a92e474fb1fa69ad8d50e01`。修复明确把 stable code 放在 `error.code`，并让 Goals、逐行 message 合同、session/extension 兼容形状和 Acceptance 使用同一规则；未发现新 finding 或剩余不确定性。本轮 PASS。

## Round 4 repair — 2026-08-17

- Repaired state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`
- Repairer: Claude Code
- Authorized scope: the requirements boundary named by `reviews/architecture.md`
  A2-F1's bounded fix. This document had already reached PASS and was committed;
  A2-F1 reopened it because the architecture review proved one evidence cell
  measured a different failure than the row claims.
- Disposition:
  - The `backup inspect` evidence row now reads "passphrase supplied" and
    `open /tmp/nonexistent.adb: no such file or directory`, the re-measured absence
    path. A dated note records that the original figure,
    `operation not supported by device`, was the Darwin ioctl error from reading a
    passphrase on a character-device stdin, produced before any archive lookup.
  - A new Non-Goal excludes `backup inspect`'s passphrase-input failure by name: it
    is not a not-found condition, it belongs to the CLI input layer, it still leaks
    an errno string after this topic ships, and it needs its own topic. It also
    requires every `backup inspect` scenario here to supply a passphrase.
  - Acceptance's first bullet now states that the `backup inspect` scenario supplies
    a passphrase, so it exercises the archive lookup rather than the excluded input.
- Verification: L0. `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 5 — 2026-08-17

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`
- Reviewer: Codex
- Method: Single-agent independent Re-review of the upstream boundary repair required by architecture finding A2-F1. Inspected the exact requirements diff, re-derived the backup scenario against the current CLI call order, and checked for regressions in every previously closed requirements finding.
- Scope: The `backup inspect` evidence row, passphrase-input Non-Goal, and matching Acceptance clause in `docs/topics/cli-error-classification/requirements.md`; preservation of R1-F1 and R2-F1.
- Findings:
  - [closed] A2-F1 upstream prerequisite — `requirements.md:33-50,94-103,140-147` now states that the in-scope backup scenario supplies a passphrase and therefore reaches archive lookup, records the re-measured missing-path output, and explicitly excludes the earlier CLI passphrase-input failure as a different-layer residual requiring its own topic. The evidence, Non-Goal, and Acceptance clauses use one boundary.
  - [closed, no regression] R1-F1 — the table-only privacy scope, backup identifier prohibition, and per-row message identity remain unchanged.
  - [closed, no regression] R2-F1 — stable codes remain owned by `error.code`; messages need not repeat them, and the session/extension compatibility decisions remain unchanged.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; `git hash-object docs/topics/cli-error-classification/requirements.md` -> `118c21dbba956b0eed832b20f66946a83bd28db4`; the exact diff is limited to the re-measured evidence, one explicit Non-Goal, and the matching Acceptance qualifier. Unchanged current source confirms `backup inspect` calls `readPassphrase` before `backup.Service.Inspect`, so supplying a passphrase is the necessary discriminator. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0. CEv1 document gate `cli-error-classification:requirements.md` -> `VERIFIED` for HEAD plus blob `118c21dbba956b0eed832b20f66946a83bd28db4` after new content-state and Round 5 evidence lineage were recorded.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无（针对本文档）。

### 🟢 优点

- Backup evidence 现在明确提供 passphrase，实际测量 archive absence，而不是前置 CLI input failure。
- 被排除的 passphrase-input errno 被诚实记录为不同层的已知 residual，没有被静默吞入 not-found topic。
- Evidence、Non-Goal 和 Acceptance 使用同一个可执行场景。
- R1-F1、R2-F1 保持关闭，无回归。

### 📝 摘要

复评对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439` 与 `requirements.md` blob `118c21dbba956b0eed832b20f66946a83bd28db4`。上游范围修复正确区分了 passphrase input 与 archive lookup，并选择排除前者；所有已批准 code/message/privacy 决定保持一致。未发现新 finding 或剩余不确定性，本轮 PASS。
