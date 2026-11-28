---
status: active
created: 2026-11-28
---

# 缺陷：session 分页用例按原文匹配下一页参数，随临时路径长度换行而失败

载体：`ad-zlaa`；v0.7.5；Lane A（测试修复，属用户 2026-11-27 发布授权范围）。
工作区：分支 `fix/session-pagination-wrapped-next-page`，基线 `main` `bd18977e3613f19d4c2355d414cb07c5ce3d4a86`。

## 现象

v0.7.5 发布提交 `bd18977e` 上的 L4 `make release-verify`（隔离 HOME 与 TMPDIR）在
`TestSessionCLIListAndShowPaginationContracts` 失败：`session show` 文本中找不到 `--client 'codex'`。CI 与使用系统默认 TMPDIR 的本机运行通过。

## 根因

`NEXT PAGE` 字段按显示宽度换行，换行位置取决于临时 state 路径的长度。用例逐段匹配 `--client 'codex'`、`--page 2` 等原文，
TMPDIR 为 26 或 41 个字符时这些参数被拆到相邻两行（实测），默认 49 个字符时恰好不拆。`9d8d389f` 已对 activity 用例改用
`sessionShowNextPageMatches` 比对完整命令，本用例当时未覆盖。属既有测试缺陷，产品输出与 v0.7.0 一致。

## 修复边界

只改该用例：保留 `SHOWING`、`1-2 of 3`、`NEXT PAGE` 检查，下一页命令改用 `sessionShowNextPageMatches` 与独立写出的完整期望命令比对。
产品代码与 `session list` 断言不变。

## 验证

- 该用例在 TMPDIR 为 26、38、41、49 个字符时各运行 3 次均通过；修复前 26 与 41 个字符时失败。
- 变异：期望命令改为 `--page 3` 时用例失败。
- 短 TMPDIR（26 个字符）下完整 `cmd/agentdeck` 包通过。

## Review — Round 1

评审结论：PASS

完成门禁：VERIFIED

发现：0 P1 / 0 P2 / 0 P3。

独立方法：PR 90 上的 Codex 代码评审（`chatgpt-codex-connector`，与作者 Claude 不同 harness），评审提交 `b39ec1ac0fcd8c463f8554c2aae4343074543be8`，结论"未发现主要问题"。
作者验证：该用例四种 TMPDIR 长度各 3 次通过，`--page 3` 变异被检出，短 TMPDIR 下完整 `cmd/agentdeck` 包通过（165.8 s）。
