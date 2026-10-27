---
status: active
created: 2026-10-26
updated: 2026-10-26
---

# 辅助会话识别与工作视图隔离：文档与任务

工作区：`agent-deck.auxiliary-session-classification`；分支：`feature/auxiliary-session-classification`。
来源：`ad-auxiliary-session-classification`；需求权威为 [requirements.md](requirements.md)。
本轮仅定义需求和文档集合，不是实现分解或交付授权。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| ux/menubar.md | [x] | [x] |
| ux/settings.md | [x] | [x] |
| ux/widget.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [ ] | [ ] |

CLI：选择范围、自动化兼容、分类查看和覆盖；菜单栏：列表/统计及范围切换；
Settings：默认范围和覆盖管理；Widget：聚合口径标签及持久化默认。
上述四个表面均有独立状态与验收，不能用某一表面的标本代替其他表面。
UX 按项目 prototype 权威提供标本，架构落实数据契约，再完成 UX 最终核对与任务分解。
本文件 Draft 未勾选表示尚未产生实现分解，不表示当前 Documents 声明缺失。
未编写的文档只声明于矩阵，不创建占位文件。文档集待 tasks.md 独立评审 ratify。

## Tasks

尚未分解；requirements → 各表面框架 → architecture → 各表面最终核对 → tasks。
每个实际文档有单独 Beads 文档任务；不建立实现任务，不以规划源记录 closed 代替产品验收。
requirements 已独立评审 PASS 并提交；本前置文档不再因后续文档缺稿停留在待评审队列。
CLI 最终表面已独立评审 PASS；菜单栏与 Settings framework 均已评审 PASS 并提交，仍待最终核对。
CLI Review 勾选适用于最终表面；其余表面 Review 勾选仅适用于已评审 framework，历史结论保留。
完整文档集检查归属本文件的分解/文档集评审；不会因本次状态修正而取消。
Beads 按当前剩余工作建立阻塞边：已完成框架的 UX 等待 architecture，tasks 等待架构和最终 UX。
架构与 UX 的框架输入用 related 关联并注明 framework 就绪条件，避免以整份 UX 最终关闭反向阻塞架构。
Widget framework 已独立评审 PASS，architecture 的四份框架输入已齐；原 architecture → Widget 阻塞边改为 related，
Widget 最终核对依赖 architecture，不建立双向 blocks。
2026-10-27 范围勘误后，requirements 与三个 UX 的各自已交付对象门禁为 VERIFIED；
requirements 协调已关闭，三个 UX 保持 in_progress、无受理人，标记待最终核对。
Widget framework 已提交于 `2510975`，PASS 与门禁见 [reviews/ux-widget.md](reviews/ux-widget.md)。
architecture 已通过并提交于 `c57102b`，见 [reviews/architecture.md](reviews/architecture.md)。
CLI 最终表面已独立评审 PASS，见 [reviews/ux-cli.md](reviews/ux-cli.md) Round 3，等待授权提交；其 Review 勾选适用于最终表面。
其余三份 UX 仍需架构后的最终核对，不将标本检查当作原生实现验收。
评审与修复交接见 [reviews/requirements.md](reviews/requirements.md)。
未合并主题状态只在本文件及 Beads 维护，不更新 main 的 docs/status.md。
