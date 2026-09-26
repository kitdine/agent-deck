---
status: active
created: 2026-09-26
---

# 缺陷：Homebrew cask 预检语法阻断 RC1 发布验证

## 现象

`scripts/test-macos-distribution.sh` 在 Homebrew 加载渲染出的
`agentdeck-app` cask 时拒绝弃用告警：`Calling \`preflight\` is deprecated!
Use \`preflight_steps\` instead.`。该脚本属于 `make release-verify`，因此
v0.6.0-rc.1 的技术预检不能通过。2026-09-26 在当前 main 与 Homebrew 7.0.6
上复现；失败输出包含 `Warning: Calling \`preflight\` is deprecated! Use
\`preflight_steps\` instead.`。测试创建的临时 tap 已由脚本清理。

## 根因

`packaging/homebrew/agentdeck-app.rb.tmpl` 使用 `preflight do` 执行冲突 CLI
公式检查。当前 Homebrew 仍可加载它，但会发出弃用告警；发行测试正确地把
加载期告警作为失败。Homebrew 的 `preflight_steps` 是受限的声明式 DSL，不能
直接搬入原来的任意 Ruby 循环与异常，需要用路径守卫和失败的安装步骤表达
同一合同。

## 修复边界

按操作者 2026-09-26 的 Lane A 决定，仅替换 cask 预检语法及其直接测试。
`agentdeck` 和 `agentdeck-rc` 两个 CLI 公式都必须被拒绝，输出可读迁移指引；
Homebrew 拒绝安装后不能留下 Caskroom 收据、App 或命令链接，也不能触及
`~/.agentdeck`。不改变 Homebrew 发布渠道、应用安装内容或版本范围。

## 验证

原模板的 `bash scripts/test-macos-distribution.sh` 退出 1，加载期逐字报告
`Calling \`preflight\` is deprecated`。替换为 `preflight_steps` 后，同一
脚本 PASS，且其临时 Homebrew tap 已清理。`bash scripts/test-cask-migration.sh`
在本工具外层沙盒下先被 `sandbox-exec: sandbox_apply: Operation not permitted`
阻止，未进入产品断言；在许可的非沙盒环境以隔离 HOME、Cellar、Caskroom 和
应用目录重跑后输出 `cask migration and mutual exclusion: PASS`，覆盖冲突
公式拒绝、迁移文案与无残留回滚。发行脚本输出 `macOS distribution packaging:
PASS`。这些本地原始日志是临时诊断文件，不作为仓库内可重放证据；上面的命令、
环境边界和可观察结果是本修复记录的验证摘要。

## Review — Round 1 — 2026-09-26

📊 总体评分：7/10

❌ 评审结论：FAIL

Reviewer: GitHub Codex independent PR review of `d47812c`; method: scoped
inspection of the Lane A fix record and cask change in PR #11. This round found
no cask behavior defect, but the record itself needs two corrections.

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

- `CASK-R1-F1` ([PR comment](https://github.com/kitdine/agent-deck/pull/11#discussion_r4111740798)):
  an empty numbered review heading claimed a round before any review occurred.
  The present round records the actual independent review; its verdict remains
  FAIL until a later independent re-review accepts the repair.
- `CASK-R1-F2` ([PR comment](https://github.com/kitdine/agent-deck/pull/11#discussion_r4111740816)):
  temporary host-local log paths were presented as retained evidence although
  they do not travel with the repository. The current candidate records the
  observed RED/GREEN output and test environment without claiming portable raw
  logs; this correction also awaits independent re-review.

### 📝 总结

Reviewed state: PR #11 head `d47812c747c2b66e4c97a6f8b88a51de46b727b5`.
The cask behavior and direct regression checks remain unchanged in this repair.
Completion gate: NOT_VERIFIED; the Lane A review criterion awaits re-review and
the exact committed-content gate has not yet been recorded.

## Review — Round 2 — 2026-09-26

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer: GitHub Codex independent PR #11 re-reviews at `688c87e` and
`7c486b2`, with maintainer verification of the exact fix record and unchanged
cask/test blobs. Method: compare both Round 1 findings with the repaired record,
check the final PR comments, and reuse the local Homebrew migration and
distribution results plus four successful final-head CI jobs.

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

- `CASK-R1-F1` CLOSED: Round 1 is now the actual failed independent review, not
  an empty stub; this Round 2 is appended only after the later re-reviews.
- `CASK-R1-F2` CLOSED: the record keeps command, environment and observed
  RED/GREEN output, and explicitly labels local raw logs as temporary.
- The final `7c486b2` Codex review reports no major issues and adds no inline
  suggestions. The two Round 2 suggestions concerned other documents and were
  repaired without changing the cask behavior or this record's evidence.

### 🟢 优点

The structured `preflight_steps` checks both CLI-only formula channels and
retains readable migration guidance. The real isolated Homebrew migration
test exercised refusal and rollback; no installed user state was modified.

### 📝 总结

Reviewed state: PR #11 head `7c486b246e855991a06f4667e169b48ed2e5f0e6`;
cask template and direct test are unchanged since signed `55d86db`, and this
record's pre-review blob is `09dfd075fd1e83df726527c9408287f7075c63f5`.
The result is a review PASS for the scoped Lane A repair. Completion gate:
VERIFIED (4/4) for candidate
`fix:cask-preflight-deprecated:state:c5ffd28dd704a12fb9a2e7f287a0643d3db0fb77a340302b10b29455002f9264`.
The fixed-template query has no missing, invalidated or unresolved item. This
gate receipt changes the record blob, so a final target-bound preservation
assessment is still required before the Beads bug can close.
