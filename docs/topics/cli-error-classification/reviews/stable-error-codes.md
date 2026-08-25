---
status: active
topic: cli-error-classification
subject: stable-error-codes
---

# Review log — cli-error-classification / stable-error-codes

## Round 1 — 2026-08-25

- Reviewed state: HEAD `0ef6bfc78083d37069a19ce96059e5f92f0423c1`; scoped candidate `urn:ce:agent-deck:state:candidate:8b714377943941002ece0b7c68c99b48feeec96ba504f1e3066502279ee3f4ff`, derived from the ordered blobs of the task-owned code, tests, living specifications, approved topic contracts, vendored dependency metadata, and exact topic status row.
- Reviewer: Codex
- Method: Single-agent formal code-and-tests review. CodeGraph was used first for `errorCode`, `errorExitCode`, `isInputError`, and the JSON error-envelope path, followed by exact task diff inspection and focused source comparison. Existing unchanged implementation L2 and command-envelope evidence was reused; broad verification stopped after a decisive contract contradiction was confirmed. No multi-agent panel or implementation-scoring tool was used.
- Scope: `cmd/agentdeck/main.go`'s `errorCode` hunk; `cmd/agentdeck/error_code_test.go`; the task-owned `cmd/agentdeck/main_test.go` hunk; `docs/specs/cli-design.md`; `docs/specs/cli-manual.md`; the approved requirements, architecture, and task boundary.
- Findings:
  - [SEC-R1-F1][P1] `docs/specs/cli-design.md`'s new complete table records exit `1` for `unsupported_wire_version` and `invalid_recent_limit`, while the same specification and implementation classify both as input errors with exit `2`; the mapping test omits both existing rows. -> OPEN
- Evidence: `docs/specs/cli-design.md:1734-1735` says both codes are input errors with exit `2`, while `docs/specs/cli-design.md:2084-2085` says exit `1`. `cmd/agentdeck/main.go:374-378,407-409` returns exit `2` for both through `isInputError`. `cmd/agentdeck/error_code_test.go:17-57` contains no assertion for either desktop error. The initial CEv1 query was `VERIFIED`; append-only Review evidence superseded the two disproved living-spec pass records, and the final query is `FAILED` with the other three criteria still valid. No Go test was rerun for this source-decisive finding.
- Completion gate: FAILED
- Verdict: REOPEN

## 📋 评审报告

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[SEC-R1-F1] [`docs/specs/cli-design.md:2084`](../../../specs/cli-design.md#output-and-errors) 的完整错误码表把 `unsupported_wire_version` 和 `invalid_recent_limit` 的 exit 写成 `1`。

- 行为风险：发布的机器契约与同一规范早先段落及真实 CLI 行为冲突；自动化调用者按新表实现时会错误处理两个 input-error 分支。
- 证据：`cli-design.md:1734-1735` 明确两者 exit `2`；`main.go:374-378,407-409` 也返回 `2`。当前 `error_code_test.go:17-57` 没有覆盖这两个既有 mapping，因此未能阻止错误表格进入候选。
💡 有界修复：只把完整表中的两行 exit 改为 `2`，并在 `TestWrappedErrorCodeAndExitCodeMatrix` 补上这两个现有错误的 code/exit 断言；不要改变生产 mapping。

### 🟡 建议改进 — 推荐

无。

## Round 1 repair — 2026-08-25

- Repaired state: HEAD `0ef6bfc78083d37069a19ce96059e5f92f0423c1`;
  `cmd/agentdeck/error_code_test.go` blob
  `293a022868a26d14fbbc1c0766d341eaf4d8f371`;
  `docs/specs/cli-design.md` blob
  `72bcbf74ef150ff43de9a604879d7899edaab0b2`.
- Repairer: Codex, with one independent read-only `gpt-5.6-terra` subagent used
  to confirm the bounded behavior and test route.
- Authorized scope: SEC-R1-F1 only.
- Disposition:
  - SEC-R1-F1 — CLOSED. The complete error-code table now records exit `2` for
    `unsupported_wire_version` and `invalid_recent_limit`, matching the earlier
    desktop contract and the unchanged `isInputError` behavior. The wrapped
    error-code matrix now asserts both existing codes and exit values.
- Out of scope, not changed: `errorCode`, `errorExitCode`, `isInputError`, the
  desktop command implementation, every other error-code table row, and all
  other findings or dirty worktree changes.
- Verification: `scripts/run-go-test.sh ./cmd/agentdeck -run
  '^TestWrappedErrorCodeAndExitCodeMatrix$'` -> exit 0;
  `bash scripts/check-topic-docs.sh` -> exit 0; `make check-whitespace` -> exit
  0; `git diff --check` -> exit 0. Existing desktop command tests already cover
  both real JSON envelopes with exit `2`; no unchanged broad suite was rerun.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

### 🟢 优点

- 一个 `errors.As` 分支覆盖五个新 typed not-found code，同时保留全部既有 mapping 的优先级。
- 七行真实命令 envelope 测试覆盖两个已提供 passphrase 的 backup 分支，并锁定 stderr、exit、message 与敏感信息边界。
- `cli-manual` 对本轮七个受影响命令及 passphrase-input residual 的描述与实现一致。

### 📝 摘要

评审对象是 HEAD `0ef6bfc78083d37069a19ce96059e5f92f0423c1` 上 scoped candidate `8b714377943941002ece0b7c68c99b48feeec96ba504f1e3066502279ee3f4ff`。实现 mapping 和七行命令 envelope 没有发现缺陷，但 living specification 的完整表与现有 exit contract 直接冲突，且对应既有 mapping 未被矩阵测试保护，因此 Round 1 为 FAIL / REOPEN。CEv1 已记录该反证并返回 `FAILED`；其余三个 required criteria 保持有效。

## Round 2 — 2026-08-25

- Reviewed state: HEAD `0ef6bfc78083d37069a19ce96059e5f92f0423c1`; repaired scoped candidate `urn:ce:agent-deck:state:candidate:1d41be7afa619d86e88f4dc55fd9797badf34ffa704845a2adb16c50fd4576a3`.
- Reviewer: Codex, with one user-authorized independent read-only subagent validating the bounded finding disposition.
- Method: Finding-by-finding independent Re-review of `SEC-R1-F1`. The current specification, mapping test, unchanged production exit path, repair blobs, and recorded targeted verification were compared directly. The exact repair evidence was reused because its test/spec blobs match the current candidate; no broad suite or unchanged test was rerun.
- Scope: `SEC-R1-F1`; the two repaired table cells in `docs/specs/cli-design.md`; the two added rows in `TestWrappedErrorCodeAndExitCodeMatrix`; unchanged `errorExitCode` / `isInputError` behavior; and direct repair-caused regressions.
- Disposition:
  - `SEC-R1-F1` — CLOSED. `unsupported_wire_version` and `invalid_recent_limit` now both record exit `2`, matching the earlier desktop contract and unchanged runtime behavior; the wrapped matrix asserts both stable codes and exit values through the common double-wrap loop.
- New findings: None.
- Evidence: `docs/specs/cli-design.md:1734-1735,2084-2085` now agrees on exit `2`; `cmd/agentdeck/error_code_test.go:42-43,50-57` asserts both code and exit after double wrapping; `cmd/agentdeck/main.go:374-378,407-409` remains unchanged. Current blobs are main `854786dc06290b3149cc58ded63ba619db26ad98`, test `293a022868a26d14fbbc1c0766d341eaf4d8f371`, and spec `72bcbf74ef150ff43de9a604879d7899edaab0b2`; the latter two exactly match the repair record. The recorded targeted matrix test and L0 checks passed. CEv1 candidate `1d41be7afa619d86e88f4dc55fd9797badf34ffa704845a2adb16c50fd4576a3` is `VERIFIED` for all four required criteria.
- Completion gate: VERIFIED
- Verdict: PASS

## 📋 复评报告

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `SEC-R1-F1` 的规范和测试两部分均按原边界关闭，没有修改已正确的生产 mapping。
- 新增矩阵行走与其他 mapping 相同的 double-wrap 路径，同时锁定 stable code 与 exit `2`。
- Repair 的 targeted test/L0 evidence 与当前精确 blob 一致，无需重跑未失效 broad suite。

### 📝 摘要

Round 2 对 repaired candidate `1d41be7afa619d86e88f4dc55fd9797badf34ffa704845a2adb16c50fd4576a3` 逐项复核了唯一 finding：`SEC-R1-F1` 已关闭，未发现有界修复引入的新阻塞。独立子代理给出相同结论，CEv1 Task gate 对四项 required criteria 返回 `VERIFIED`，因此 Re-review PASS。
