---
status: historical
created: 2026-10-02
retired: 2026-10-02
---

# 图标更新任务

独立 issue #29；用户已批准所选 artwork 和端到端交付边界。
基线 `release/v0.6.x` / `26662ba018444eb7758aba995fa678d47da5aa77`。
分支 `feature/ad-shared-stroke-icon`，workspace `agent-deck.ad-shared-stroke-icon`。
用户明确允许复用已释放串行 slot；前任 handoff
`50efa9ef-3bca-5783-8f8b-c5472d09208b`，此任务不是旧 Lane A Bug。

## Documents

| Document | Draft | Review | Notes |
| --- | --- | --- | --- |
| requirements.md | [x] | [x] | 已选 artwork、映射、验收边界合并记录 |
| ux/ | n/a | n/a | 用户已批准 ZIP 预览；无新交互设计 |
| architecture.md | n/a | n/a | 保留现有加载/编译契约，无新架构 |
| tasks.md | [x] | [x] | 单个不可拆分图标替换任务 |

## Tasks

| # | Task | Dev | Review | Verification |
| --- | --- | --- | --- | --- |
| 1 | ad-shared-stroke-icon | [x] | [x] | L1 PNG/模板/界面；现有 bundle 编译与相关测试 |

一个 PR 只交付这次品牌替换。历史资产记录保留。
未执行的显示验收不得记为通过。无发布、部署、安装或真实数据操作。

## 验收边界与后续

用户于 2026-10-02 明确延后系统菜单栏人工视觉验收（来源与原话见 requirements.md）。
此项未验证、不阻塞本轮交付；16px 仅记录离屏像素验证。
实际代码评审、CEv1 和 GitHub/CI 状态见 reviews/ad-shared-stroke-icon.md。
用户后续人肉检查若发现问题，按新的明确反馈处理；本任务不启动第二 issue。

## Source closure

签名实现提交 `ff1c03cf558b3a7b73022e1fc80b30ee56b8a453`；
该提交的文档门禁各1/1、Task4/4、topic closure1/1均经Neo4j MCP VERIFIED。
完整源码topic作为整体归档；PR远端Review/CI/merge及实际merge evidence仍由
issue29与Beads `ad-shared-stroke-icon` 跟踪，归档不是已合并或已发布声明。
