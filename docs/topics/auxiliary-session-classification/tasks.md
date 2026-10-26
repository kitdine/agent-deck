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
| ux/menubar.md | [ ] | [ ] |
| ux/settings.md | [ ] | [ ] |
| ux/widget.md | [ ] | [ ] |
| architecture.md | [ ] | [ ] |
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
当前 requirements 最新独立复评为 PASS，Review 已勾选；文档完整性证据门禁仍开放。
CLI framework 最新独立复评为 PASS，文档完整性证据门禁仍开放，见 [reviews/ux-cli.md](reviews/ux-cli.md)；
architecture 后仍需 final-surface reconciliation，不代表最终表面完成。
评审与修复交接见 [reviews/requirements.md](reviews/requirements.md)。
未合并主题状态只在本文件及 Beads 维护，不更新 main 的 docs/status.md。
