---
status: historical
created: 2026-09-28
retired: 2026-09-28
---

# 缺陷：隐私扫描将 cask/task 标识误认成密钥

## 现象

v0.6.0 RC1 的精确提交 `7c1dae1` 在 GitHub Release Preflight run
`36375779887` 中通过桌面作业，但 `make release-verify` 停在
`check-privacy`。扫描器报告三个文档含有禁止内容：
`docs/archive/fixes/cask-preflight-deprecated.md`、
`docs/fixes/lsregister-bundle-id-collision.md` 和
`docs/topics/snapshot-performance/tasks.md`。这些命中落在普通的 `cask-…`
路径、`ad-bug-cask-…` 任务 ID 和 `#task-…` 锚点中；不输出或保存任何疑似密钥值。

本修复的 Beads 任务为 `ad-bug-privacy-scan-token-boundary`，用户已确认 Lane A。

## 根因

`scripts/check-privacy.sh` 的 `sk-[A-Za-z0-9_-]{20,}` 没有左边界。
`grep -E` 从 `cask-` 和 `task-` 内部的 `sk-` 开始匹配，后面的长标识继续满足
长度要求。失败与文件中真实凭据无关；在独立临时 Git 仓库里只写入这两种标识，
原规则也会失败。预检的桌面作业成功，L4 停在静态扫描，未进入后续归档步骤。

## 修复边界

- 仅为 `sk-` 分支增加“行首或非字母数字/下划线”的左边界，保留其长度及字符集。
  `AKIA`、私钥头和 `ghp_` 三种检测保持原样。
- `scripts/test-check-privacy.sh` 用隔离临时 Git 仓库证明普通 `cask-…`、
  `task-…` 可通过，而独立的假 `sk-` token 仍被拒绝；测试源码不会包含
  连续的假密钥字面量。
- `Makefile` 的 `check-privacy` 目标运行这项回归，后续 Release Preflight
  不会只跑扫描器而漏掉边界测试。
- 不修改产品行为、用户数据、发布门禁或本次 RC 的真实状态决定。

## 验证

- 修复前：`bash scripts/test-check-privacy.sh` exit 1，报告普通标识被拒绝。
- 修复后：同一命令 exit 0，输出 `privacy token boundary PASS`。
- 修复后：`make check-privacy` exit 0，扫描当前完整仓库并运行边界回归。
- 独立评审和候选提交 CEv1 已在 Round 1 完成；PR 合并结果与同 SHA Release
  Preflight 待本修复交付后确认。

## Review — Round 1 — 2026-09-28

### 📋 隐私扫描 token 边界修复评审

📊 总体评分：9/10

✅ 评审结论：PASS

Reviewer: GitHub Codex connector（独立 PR 上下文）；Codex 主会话核对评审回执、
行内评论、差异和证据。Method: PR #14 的冷上下文代码审阅，加上针对预检失败路径的
直接复核。Scope: `Makefile`、`scripts/check-privacy.sh`、
`scripts/test-check-privacy.sh` 与本 fix 记录在提交 `e83097b` 的内容。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

无。

### 🟢 优点

- 回归用例先在旧扫描器上失败，再在修复后通过，同时证实独立假 token 仍会被拒绝。
- 左边界只影响 `sk-` 检测；其他三种秘密形态、输出边界和产品代码未改变。
- PR #14 的四项 `verify` / `desktop` 检查成功。GitHub Codex 对 `e83097b`
  的审阅完成，没有行内发现或需处置的建议。

### 📝 总结

Reviewed state: 签名提交 `e83097bb26fa4a8067e1632b3d2d83ee04c6b197`，tree
`30352e6d7ae8b7185d86956a9ce0e65616a523a8`；PR #14 目标 main 为
`7c1dae1ca0980b0507fdb655fc5c638e850ac833`。前述根因在临时 Git 仓库复现；
源仓库隐私扫描与边界回归通过。Review PASS 不宣称后续 Release Preflight 已通过。

Evidence: `bash scripts/test-check-privacy.sh` 在旧规则 exit 1、新规则 exit 0；
`make check-privacy`、`make check-whitespace`、两份脚本的 `bash -n` 及
`git diff --check` 均 exit 0。PR #14 的四项 CI 检查成功；GitHub Codex 审阅回执
对 `e83097b` 报告没有重大问题，PR 无行内评论或其他开放 finding。

Completion gate: VERIFIED (4/4)，WorkUnit `fix:privacy-scan-token-boundary`，
目标 `fix:privacy-scan-token-boundary:commit:e83097bb26fa4a8067e1632b3d2d83ee04c6b197`；
固定 gate 查询无 missing、invalidated 或 unresolved item。新增评审文字尚需以
最终文档提交状态显式绑定；提交与 PR 合并仍是独立交付步骤。
