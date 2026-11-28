---
status: active
created: 2026-11-18
---

# v0.8.0 版本契约

来源为 Beads `ad-v080` 和既有计划任务 `ad-v080-plan`。用户选择较短的发布范围，确认计价及历史重算设计的两项套餐补充后要求继续。工作区 `agent-deck.v0-8-0-contract` 使用 `feature/v0-8-0-contract`，创建基线 `a224b0b824d2cad451d3bd83153832d65506f0fe`。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [ ] | [ ] |
| 2. `v0-8-0-contract` | [ ] | [ ] |

## 产品成员

| Topic | 来源 | 本版接受边界 |
| --- | --- | --- |
| `cost-transparency` | `ad-cost-transparency` | 补全 API 价目、逐调用计算、Codex 套餐、历史重算和金额出处。 |
| `subscription-quota-conversion` | `ad-subscription-quota-conversion` | 新增功能。对同一会话给出金额，按账号及窗口显示可证明的消耗或条件估值。 |
| `claude-local-quota` | `ad-claude-local-quota` | 2026-11-26 用户加入的第二个新增功能。Claude 额度改为本地优先采集：高频读取 `.claude.json` 缓存，`claude -p /usage` 按需补位，界面与 Codex 对齐。v0.7.5 发布后开始，实现须等前两个成员合入 `main`。 |

会员积分换算与旧版规划的客户端适配、Linux 和其他候选留在原有范围。

装配分两批进行。第一批合入计价与额度换算：额度分支已通过合并提交包含计价分支，因此以额度分支为一个装配候选，先把 `main` 合入该分支以解决与 v0.7.5 的冲突，再经独立集成评审后发起 PR。第二批在 `claude-local-quota` 完成任务评审与门禁后单独装配。v0.7.5 成员维持独立版本，实际程序集成前核对采用的主线内容。

冲突解决按 [合并分类](../../../.agent-instructions/branching.md#merge-classification) 处理。已发布的 v0.7.5 迁移号固定，成员专题的迁移号顺延到其后；既有契约内的公共 wire 字段取并集。这两类调整属于该批的三方冲突解决，由该批独立集成评审覆盖。只有需要新增或改变产品、架构契约的冲突，才退回所属成员专题的设计与评审边界。

两批都按实际合并分类，作为追加轮次记入 `reviews/assemble.md`，写明父提交与结果标识。PR 以保留祖先的合并方式进入 `main`，不使用 squash 或 rebase。两批都合入 `main` 后，`assemble` 才算完成。

## 顺序与验收

计价先提供逐调用价卡、日期、完整性和资格。额度换算 topic `subscription-quota-conversion` 消费该结果，与一次测量的端点及相同请求集合绑定。v0.7.5 已于 2026-11-28 发布并位于 `main`，第一批在把 `main` 合入装配候选分支后的结果上复核其身份、JSONL 去重、schema、备份与原生投影。用户确认的具体费率和适用范围属于所属 topic 文档。

第一批的联合夹具至少证明旧请求按旧费率、同一调用不会因升级多计、两个套餐规则只在已证适用域生效、同一会话 API 等价 `$6` 与短窗 `4 pp` 和周窗 `2 pp` 在明确假设下形成约 `$150`、`$300`，以及无前后读数或同窗多来源时的观察说明。身份缺失夹具验证同源 Claude 手动测量只给带未验证账号条件的估值，验证 Codex 缺端点账号键时只给可证观察。针对实际目标内容做格式、回归、迁移/兼容和版本集成验证。评审及 CEv1 均按实际 ContentState 分别记录。

第二批针对第一批合入后的 `main`，复核额度样本的去重、账号归属、schema 迁移，以及桌面与 CLI 投影的兼容性。`claude-local-quota` 自身的需求、界面与架构归该专题文档所有，本契约不替它设定。

版本阶段用三个成员 topic 的审定内容和任务证据作为输入：第一批为 `cost-transparency` 与 `subscription-quota-conversion`，第二批为 `claude-local-quota`。版本收口以三者都已装配为前提。本轮授权范围为本地设计、实现、独立评审及验证。主题与版本 Git 交付按各自的动作授权及仓库门禁处理。
