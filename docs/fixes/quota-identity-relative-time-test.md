---
status: active
created: 2026-11-28
---

# 缺陷：额度身份卡用例的相对时间跨越取整边界

载体：`ad-31m5`；v0.7.5；Lane A（测试修复，属用户 2026-11-27 发布授权范围）。
工作区：分支 `fix/quota-identity-relative-time-test`，基线 `main` `bf42a375f82cf405950d47b5dce0fa115dd86987`（含 PR 88）。

## 现象

PR 87 合入 PR 88 后的 desktop 作业（head `6dedfcc6`）在
`MenuBarChromeTests.testQuotaIdentityCardsRenderAtNativeWidthsInBothLanguagesAndThemes` 失败：
`pro100-and-recovered zh-Hans 420 light` 一轮期望无障碍标签为"2分钟前读取"，渲染结果为"1分钟前读取"。该用例在 CI 上耗时 30.9 s。

## 根因

用例在开头取一次 `now`，把身份读取时间设为 `now - 60 s`；视图渲染与期望标签各自在之后调用 `Date()`。整个矩阵（两种语言 ×
两种主题 × 两组卡片 × 两种宽度）耗时数十秒，最后一轮时读取时间已过去约 90 s，正好在相对时间"1 分钟 / 2 分钟"的取整边界上，
渲染与断言分落两侧即失败。属既有测试时序缺陷，与 v0.7.5 改动无关。

## 修复边界

只改该用例：`now` 移入每次构造卡片的 `panels()`，每种语言与主题组合按自身渲染时间生成读取时间，年龄保持在约 60–70 s，
远离取整边界。产品代码、卡片内容与断言不变。

## 验证

本机隔离完整 `scripts/test-macos-app.sh`，见下方评审节。

## Review — Round 1

评审结论：PASS

完成门禁：VERIFIED

发现：0 P1 / 0 P2 / 0 P3。

独立方法：PR 89 上的 Codex 代码评审（`chatgpt-codex-connector`，与作者 Claude 不同 harness），评审提交 `1056095def861d702fc057f237e5386448a4cd67`，结论"未发现主要问题"。
本机隔离完整 `bash scripts/test-macos-app.sh` 退出 0：App 287 个用例（3 个跳过）、Widget 75 个，0 失败。
