---
status: active
created: 2026-09-28
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
- 独立评审、最终内容状态 CEv1 和同 SHA Release Preflight 待本修复交付后完成。
