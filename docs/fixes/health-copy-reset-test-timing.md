---
status: active
created: 2026-11-27
---

# 缺陷：健康项复制用例在负载高的 CI 上时序失败

载体：`ad-58gc`；v0.7.5；Lane A（测试修复，属用户 2026-11-27 发布授权范围）。
工作区：分支 `fix/health-copy-reset-test-timing`，基线 `main` `470b0d749ba0e24b457fe386d0e45a4f5999f2b8`。

## 现象

PR 87（只改文档）的 desktop 作业连续两次在
`MenuBarViewModelTests.testHealthRecoveryActionsAreClassifiedAndCopyDoesNotChangeHealth` 失败；同一棵树在本机隔离
`scripts/test-macos-app.sh` 中通过（1.84 s），`main` push 的 desktop 作业也通过。

## 根因

`MenuBarViewModel.copyHealthAction` 在复制后起一个 1.6 秒的任务清除 `copiedHealthRowID`；用例固定等待 1.7 秒后断言已清除，
只给复位任务留 100 ms。负载高的 CI 主机上该任务排程晚于 100 ms 即失败。属既有测试时序缺陷，与 v0.7.5 改动无关。

## 修复边界

只改该用例：把固定等待换成以 50 ms 间隔轮询、上限 5 秒的等待，到期仍未清除即断言失败。产品代码与断言内容不变。

## 验证

本机隔离完整 `scripts/test-macos-app.sh`，见下方评审节。复位任务被移除时，用例在 5 秒上限后断言失败。


## Review — Round 1

评审结论：PASS

完成门禁：VERIFIED

发现：0 P1 / 0 P2 / 0 P3。

独立方法：PR 88 上的 Codex 代码评审（`chatgpt-codex-connector`，与作者 Claude 不同 harness），评审提交 `9a64d22041611a5064cddb501220109e4cd5592b`，结论"未发现主要问题"。
同一提交的 CI desktop 作业原生测试全部通过（仅因本记录缺评审轮次被文档门禁拦下）；本机隔离完整 `scripts/test-macos-app.sh` 退出 0。
