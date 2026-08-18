---
status: active
topic: cli-error-classification
subject: tasks.md
---

# Review log — cli-error-classification / tasks.md

## Round 1 — 2026-08-18

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/tasks.md` blob `4f7c03f283a842c00614e876bbdd921cdfb63327`
- Reviewer: Codex
- Method: Single-agent document-decomposition review using the `development-workflow` design/contract dimensions; CodeGraph mapped the approved architecture's construction/mapping symbols to current callers and tests, followed by one focused search of existing CLI error assertions and JSON contract fixtures. No implementation-scoring tool was applied to the tasks document.
- Scope: `docs/topics/cli-error-classification/tasks.md`, checked against the approved requirements and architecture contracts, current provider/store/backup/session/CLI source boundaries, affected unit and command tests, JSON error-envelope fixtures, verification routing, and task-level commit boundaries.
- Findings:
  - [P1] T1-F1 — `tasks.md:13-24` cannot deliver the architecture's typed-error boundary. Task 1 omits the new `internal/errdefs` leaf package, does not name the `backup_unreadable` branch alongside not-found errors, and excludes `cmd/agentdeck/main.go` even though the approved architecture deliberately keeps session error construction in `sessionShowNotFound` there. Its generic “tests” also fails to assign the carrier redaction, `errors.As`/`errors.Is`, provider/credential conversion, backup absence/unreadable/`invalid_backup`, and both session-message regression checks. -> Expand task 1's outcome, files, and test ownership to include `internal/errdefs`, the two backup open-error branches, the CLI session constructor hunk in `cmd/agentdeck/main.go`, and the exact package/CLI tests that prove redacted `Error()`, preserved causes, matching, unchanged `invalid_backup`, and unchanged session text; state that task 2 owns only the later `errorCode` mapping hunk in the shared CLI file.
  - [P1] T1-F2 — `tasks.md:26-37,48-50` reduces task 2 to a negative storage-text assertion and calls the machine code the only observable change. The approved contracts require five exact new codes, per-row message identity (including no backup identifier and preserved session/extension shapes), unchanged exit status and existing `state_busy`/`invalid_backup` mappings, and real JSON envelopes for every evidence-table command with a supplied backup passphrase. Current tests demonstrate why the ownership must be explicit: `error_code_test.go` covers the mapping matrix, while `main_test.go` still expects `session show` to emit `runtime_error`; the generic phase-7 fixture pins mostly invalid-argument leaf errors rather than these missing-target scenarios. -> Make task 2's acceptance and test boundary enumerate the exact code/message/exit/compatibility matrix and end-to-end evidence-table commands, name the mapping and CLI-envelope tests it owns, and revise the `ux/` rationale to acknowledge observable JSON message changes while retaining the correct decision that no interactive UX document is required.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; `git hash-object docs/topics/cli-error-classification/tasks.md` -> `4f7c03f283a842c00614e876bbdd921cdfb63327`; approved `architecture.md` requires `internal/errdefs`, keeps session construction in `cmd/agentdeck/main.go`, defines provider/credential/backup/session constants, splits backup absent/unreadable, and maps all carriers through one `errors.As` case. CodeGraph located `sessionShowNotFoundError` only in `main.go`, found no direct covering test for that type, and mapped `errorCode` to `cmd/agentdeck/error_code_test.go`; focused inspection confirmed `main_test.go` still asserts `runtime_error` for missing session JSON and the phase-7 fixture does not exercise the required missing-target matrix. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0 after this round was recorded.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：6/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`tasks.md:13`] Task 1 无法交付 architecture 已批准的 typed-error boundary。
- 行为风险：实现者必须自行决定 `internal/errdefs`、backup unreadable、session CLI constructor 和 matching/redaction tests 属于哪个 task；task 1 即使按文字完成，也无法满足 architecture。
- 证据：`:20-22` 未列 `internal/errdefs` 或 `cmd/agentdeck/main.go`；当前 `sessionShowNotFoundError` 只在 CLI 文件中；architecture 同时要求 absent/unreadable 两个 backup branch 和结构化 cause/redaction。
💡 有界修复：补齐 task 1 的 outcome、文件和 exact tests，并明确共享 `main.go` 中 constructor hunk 属 task 1、`errorCode` mapping hunk属 task 2。

[`tasks.md:26`] Task 2 的 acceptance/test boundary 没有覆盖完整 JSON 合同。
- 行为风险：五个 code 可以映射正确但 message identity、exit status、旧 code 兼容或真实命令 envelope 仍回归；“唯一变化是 code”还会掩盖明确要求的 message 脱敏变化。
- 证据：`:28-36` 只要求负面 storage-text 断言；现有 `error_code_test.go` 只覆盖旧 mapping，`main_test.go` 仍期待 session `runtime_error`，phase-7 fixture 不覆盖这些 missing-target 场景。
💡 有界修复：枚举 exact code/message/exit/compatibility matrix 与五类真实命令场景，指定 mapping/CLI tests，并修正 `ux/` rationale 对 observable message changes 的描述。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 两个 implementation task 的先后关系和 commit boundary 清楚，避免 code mapping 先于领域错误类型。
- L2 适合 documented JSON contract 与 persisted-adjacent lookup path 的风险。
- Specs/manual 更新被放进改变 error values 的同一 task，没有留下事后文档漂移。

### 📝 摘要

评审对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439` 与 `tasks.md` blob `4f7c03f283a842c00614e876bbdd921cdfb63327`。任务顺序和验证等级合理，但 task 1 缺少新 carrier/session constructor/backup branch 所有权，task 2 缺少完整 code-message-exit/真实 CLI 验收矩阵，因此实现者仍需自行补设计。本轮为 FAIL/REOPEN；修复范围仅为 T1-F1、T1-F2。

## Round 1 repair — 2026-08-18

- Repaired state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; `docs/topics/cli-error-classification/tasks.md` blob `ae4e769c3887f090eaa600bece91342870f55f24`
- Repairer: Claude Code
- Authorized scope: T1-F1 and T1-F2.
- Disposition:
  - T1-F1 — CLOSED. Task 1's outcome now names the new `internal/errdefs` package
    and why it must stay standard-library-only, the five code constants in their
    owning packages, the provider/credential conversion at the `sql.ErrNoRows`
    returns, both `os.Open` branches with `ErrInvalidArchive` left untouched, and
    the session retype in `cmd/agentdeck/main.go`. Its file list adds
    `internal/errdefs/errdefs.go` and that CLI file. A new preamble states the
    shared-file split by hunk: task 1 owns the `sessionShowNotFoundError`
    constructor, task 2 owns `errorCode`, and neither may stage the other's hunk.
    The generic "tests" entry is replaced by per-package ownership covering carrier
    redaction, `errors.Is`/`errors.As`, the constructor-only cause, provider and
    credential conversion, the two backup branches, unchanged `invalid_backup`, and
    both preserved session texts.
  - T1-F2 — CLOSED. Task 2 gains a seven-row acceptance matrix of command, code,
    message identity and exit status, asserted against real command envelopes, with
    both `backup inspect` rows supplying a passphrase. It states the forbidden text
    classes, that exit statuses are unchanged, and that `state_busy`,
    `invalid_backup`, `extension_not_found` and `invalid_argument` must still
    resolve as they do today. Test ownership names the three concrete targets:
    extending `error_code_test.go`'s matrix, flipping `main_test.go`'s JSON
    `session show` expectation from `runtime_error` to `session_not_found` while
    keeping its exit-1 and no-`sql:` assertions, and a new command-level envelope
    test over the matrix. An explicit out-of-scope line carries the requirements
    Non-Goal forward: the passphrase-input failure keeps returning `runtime_error`,
    and no scenario may reach it.
  - The `ux/` rationale no longer claims the only observable change is a machine
    code. It now says four of the seven rows get a new `error.message`, that no row
    changes the interaction, and that the replacement text is fixed by the approved
    Message identity table rather than designed here — so the decision that no
    `ux/` document is required stands on an accurate premise.
- Verification: L0. `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0; `git diff --check` -> exit 0.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — 2026-08-18

- Reviewed state: HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`; reviewed-content `docs/topics/cli-error-classification/tasks.md` blob `ae4e769c3887f090eaa600bece91342870f55f24`; post-verdict status-only blob `c962061a0b59fb498c8e4374c8d8de5463420796`
- Reviewer: Codex
- Method: Single-agent independent Re-review of T1-F1 and T1-F2. Inspected the exact task repair diff, mapped every repaired outcome/file/test row back to the approved architecture and current source/test locations, and checked the two task boundaries for overlap, omission, and independently stageable hunks.
- Scope: T1-F1 and T1-F2 in `docs/topics/cli-error-classification/tasks.md`, including task ownership, files, tests, acceptance, ordering, verification levels, and commit boundaries.
- Findings:
  - [closed] T1-F1 — `tasks.md:13-62` now assigns task 1 the standard-library-only `internal/errdefs` carrier, all five owner constants, provider/credential conversions, both backup open-error branches with `invalid_backup` preserved, the CLI session constructor hunk, and exact carrier/store/provider/backup/session tests. The preamble makes the shared `main.go` split explicit and keeps `errorCode` outside task 1.
  - [closed] T1-F2 — `tasks.md:64-107,115-123` now gives task 2 a seven-row real-command envelope matrix with exact code/message/exit behavior, negative leakage assertions, existing mapping compatibility, concrete mapping/session/command test ownership, supplied passphrases for both backup rows, and an explicit exclusion for the CLI passphrase-input residual. The `ux/` rationale now acknowledges message changes while correctly preserving the no-interaction-design decision.
  - [closed, no regression] Task ordering, L2 verification, per-task commit boundaries, and delivery-authority exclusions remain intact; the shared CLI file is explicitly staged by hunk.
- Evidence: `git rev-parse HEAD` -> `1a205e2a1afd1c258e97f95a253552a012e87439`; the reviewed-content blob `ae4e769c3887f090eaa600bece91342870f55f24` closes both Round 1 findings. Approved architecture blob `4042d754...` defines the same carrier, five codes, backup split, session construction, message rows, mapping, and passphrase exclusion. CodeGraph/current source locate the owned construction and mapping hunks in `cmd/agentdeck/main.go`, `errorCode` tests in `error_code_test.go`, session JSON expectations in `main_test.go`, and backup branches in `readEncrypted`. After the verdict, the only `tasks.md` change is its own Documents matrix Review cell `[ ]` -> `[x]`, producing final blob `c962061a0b59fb498c8e4374c8d8de5463420796` without changing the reviewed decomposition. `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and `git diff --check` -> exit 0. CEv1 document gate `cli-error-classification:tasks.md` -> `VERIFIED` for HEAD plus final blob `c962061a0b59fb498c8e4374c8d8de5463420796` after status-only preservation evidence was recorded.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- T1-F1 已关闭：carrier、owner constants、conversion branches、session constructor 和 exact tests 均有明确 task ownership。
- T1-F2 已关闭：七行 code/message/exit matrix、兼容断言和真实 CLI envelope tests 完整。
- `cmd/agentdeck/main.go` 的共享边界按 hunk 固定，两个 task 可以独立 staging/commit。
- `ux/` rationale 现在准确区分 observable message change 与 unchanged interaction。

### 📝 摘要

复评对象为 HEAD `1a205e2a1afd1c258e97f95a253552a012e87439`、reviewed-content blob `ae4e769c3887f090eaa600bece91342870f55f24` 与 verdict-only final blob `c962061a0b59fb498c8e4374c8d8de5463420796`。T1-F1、T1-F2 均已关闭，两个 implementation task 现在覆盖批准合同且边界可独立实现、验证和提交；未发现新 finding 或剩余不确定性。本轮 PASS。
