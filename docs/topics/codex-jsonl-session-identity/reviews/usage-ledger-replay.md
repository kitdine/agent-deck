---
status: active
topic: codex-jsonl-session-identity
subject: usage-ledger-replay
---

# 用量账组回放评审

## Round 1 — 2026-11-20，C2 账本切片

### 📋 独立评审

📊 本轮范围评分 9/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 建议改进

无阻断发现。旧镜像细目未知时，补充的副本证据可使该分量保守保留。

#### 🟢 通过的范围

parser 9 限于可准入的 Codex 明文，其他来源沿用已有版本。完整 stamp 依赖独立选择和捕获复验。计数保留 int64 和显式零的区别，区间只在观察端点分割。旧计量 20 加请求 10 的可能重叠、确证包含和独立情形分别得 10/1、20/1、30/2。未重放旧行整组保留，旧前置条件和准确 run 绑定受复验。SQLite 已提交旧小计始终为 60。

#### 📝 摘要

独立 Claude Code Opus `claude-opus-5-5`，T3 配置 `effort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，149 路径切片 `d6246ded957d118386eea646c6f3007d09390e65050a8aaf9b0f9d8e036180d9`，135 路径 Go `1bb1405a7aee9df5b83722ebe68b562530e3798136233ed2159af4c4c33eccab`。组合 153 路径 `d41ac94e29846866e0a9e4553e18e811158e0cfbd9138fc3af3846464d1b97a6`，139 路径 Go `dd7badcdcd726e4cb846cfbb0d9e0b15f5bcd5cc88c388e312159dfbe5f33ad9`。

组合独立 `TestCJSIUsageLedgerReplay` 顶层和 48 个子用例通过，build、作用包 vet 通过。移除整组 hold 或忽略 Basis 的两个变异触发回归。生产 `prepareSource`、`publishPrepared`、`publishFacts` 等接线、完整 Dev、Review 和 CEv1 仍待 Task 3 后续工作。

Task 8 分别应用账本和所有权条件。A4/A6a 判定可重放旧 JSONL 账组。A5 仍有安全已结算 OTel 且接管未决时，公开侧保持原 OTel，新 JSONL 提议等待授权换侧并报告覆盖不足。正式换侧使用同一原子发布事务。

## Round 2 — 2026-11-20，受管候选暂存

### 📋 独立评审

📊 本轮范围评分 7/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

**CJSI-USAGE-R2-F1**（P1）。`internal/usage/cjsi_ledger_prepare.go:125` 复制候选提供的旧行，`internal/usage/cjsi_ledger_staging.go:102` 只复核这一份。一个保留的 v8 调用未进入 `Hold` 或 `Retract`。带该旧行与省略该旧行的候选使用相同已登记 basis 及封存来源，得到相同 proposal stamp。真实受管 SQLite 用例将旧行的 output 增加 1，完整候选按预期拒绝，省略候选成功提交一个私有组且旧前置条件为零。使用草稿权威从同一写事务独立给出的完整事件键和行摘要集合，校验候选集合与所选集合完全一致，并逐行复核当前 SQLite 镜像。保留项也参与准入。

#### 🟡 需修复的问题

**CJSI-USAGE-R2-F2**（P2）。`internal/usage/cjsi_ledger_staging.go:68` 比较 attempt 与 publication head 的 core epoch。真实受管 SQLite 用例仅推进当前 `derived_snapshot_generation.epoch`，head 与 attempt 保持原值，暂存仍成功提交一个组。写事务应同时核对当前 generation singleton 和已选 core epoch，沿用私有索引头读取的等值条件。

#### 🟢 通过的范围

来源重放逐次核对实际封存工作集 round/attempt、认证有序记录和完整前缀，最终重读选择。已覆盖同字节 sibling、错误线程或 digest、过期草稿、工作集退役、精确整数和 NULL、引用/区间边界、后段失败回滚、篡改后重试拒绝、v36 普通入口。header-only 和旧账不能重放的组保持已提交 usage 与 OTel 原值。两项新增负例只观察到私有组提交。

#### 📝 摘要

OpenCode Pro `openai/gpt-6.2-astra` 冷上下文独立评审，T3 请求 `variant=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，起点 154 路径产品 `a070ba8d425facff1d73c5b5d6561a2d04186875da49589f366332df504bcf8b`。候选为 159 路径产品 `cba1426bee3dc0c6f1435d285d1b033bacd1728bef31e5acfd2755c205b37706` 和 145 路径 Go `274f3f748d0029b4529483ae506dcc4b8249137e52be9a84f209310a6605d4ad`，五路径增量补丁 `58a72ba951e4269d99ce1cc5771589971e00c4c80f2306baf8119daee9ea6ff8`。独立评审核对 12 项校验和，在 Go 1.27.3 darwin/amd64 下通过项目 wrapper 运行两项合成负例。主控比对精确清单、两处调用条件及结果。沿用该状态有效的运行和负对照。Task 3 的私有暂存须先修复并复评，最终同代提交归 Task 8。

#### 修复候选的复评见证

按原话题 HEAD 绑定产品 `fd56f79673686d7ecf6475415911d470a6c2844f7d345d4e80d563f05f689866`、Go `abe342e30c316be58c69b689b8ee5b510302017871c5a3c7d61e4038cbb42e08` 和补丁 `e8da5b486b502a7b8512365cdd2ad905b932cf36d4eae3afe9dd69a58bd00fc6`。主控核对 20 项校验和、3,742 个输入路径及两路径补丁回放。既有 48 个账本子例、8 个受管暂存子例和相应 race、vet 可沿用。

请求 `v075-e21-cjsi-task3-r2-independent-fd56f796` 的 OpenCode 复评者在真实受管 SQLite 测试中先存一组、三个成员及零条旧行动作。重试时省略选中行、分别修改任一选中旧行或只推进实时 core epoch 均被拒绝。只修改未选中行或恢复已登记值和 epoch 后允许复用。六张私有表与公开快照在各次调用前后逐列一致。主控核对了实际用例输出。作者原红例分别复现省略选中行成功和实时 core epoch 不一致时仍可暂存。复评者建议关闭 F1、F2。

复评者报告启动时收到了作者侧摘要。T3 启动记录确认新的独立角色线程，摘要投递情况仍待确认，因此正式复评和两项发现的处置仍开放。Task 8 继续拥有重启后重建选集及最终 CAS。

## Round 3 — 2026-11-20，私有暂存修复冷复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 需修复的问题

无新增发现。

#### 🟢 已关闭发现

**CJSI-USAGE-R2-F1 CLOSED。** `Stage` 在私有组写入或复用前比对独立登记的全部旧键与哈希，再在同一事务逐键读取现有行及 run 绑定。`OldImage.Precondition()` 包含绑定的值及存在性。缺项、多项、错误键或哈希、重复、畸形及未给定选集均被拒绝。无旧行时接受显式空集。旧实现同 stamp 省略保留项会成功写入一组，修复后仅完整登记集通过，逆序可复用且没有虚假的旧行动作。

**CJSI-USAGE-R2-F2 CLOSED。** 准入在组存在性检查及写入前要求当前 generation epoch、head 及 attempt 三者相等。只更新当前 epoch 的旧实现对照仍开放暂存；修复后首次提交及已有组复用均拒绝，恢复一致后可复用。晚期 SQLite 写入错误使六张私有表和公开快照保持原值。

#### 📝 摘要

Codex `gpt-6.2-astra`、T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task3-r2-codex-packet-fd56f796`。启动时只收到通用项目与流程背景，候选作者历史和先前裁定均未传入。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `fd56f79673686d7ecf6475415911d470a6c2844f7d345d4e80d563f05f689866`，Go `abe342e30c316be58c69b689b8ee5b510302017871c5a3c7d61e4038cbb42e08`，两路径增量补丁 `e8da5b486b502a7b8512365cdd2ad905b932cf36d4eae3afe9dd69a58bd00fc6`。独立复评核对已绑定的产品源码、前后对照、受管 SQLite race 日志及测试断言。主控复用已核对的 48 个账本和 8 个受管暂存子例，包括保留行及实时 epoch 锚点。

PASS 适用于本轮两个私有暂存发现。当前尚未注册整项 Task 3 的 WorkUnit 与精确候选状态，仍需受影响生产调用方交接和整项证据。Task 8 负责重启后重建选集、最终 CAS 及公开发布。

## Round 4 — 2026-11-23，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 3，作者 Codex `gpt-6.2-astra`，最终补丁 SHA-256 `dacfc808882b23cc0d4adaebdbd43d1f32d61a05851878672908976a0d656a74`，30 个路径，基于 HEAD `119ec60`（已含 Task 2 schema 37）。经用户批准共五轮评审（含第 4 轮修复）。

- **重建首评（AGY Gemini 4 Neon high）FAIL。** CJSI-USAGE-RB-F1（P1）缺失或无效 `total_token_usage` 不轮换 epoch，计数重置后新调用被当镜像吞并；F2（P1）重叠旧通知无包含原生请求时双计；F3（P1）`Stage` 用 DEFERRED 事务，复用路径并发 fail-open、首次暂存并发报 517；F4（P1）`TokenInterval` 恒不填模型，阻断 A4b；F5（P2）保留的已提交响应跨 ticket 失证；F6（P2）保留旧 `LegacyCall` 未比对 turn；F7（P2）run 冲突只查完全包含。
- **R1 复评（AGY Gemini 4 Neon high）FAIL。** F1、F2、F3、F5、F6 closed：epoch 在基线失效边界轮换；重叠检查前移；`Stage` 在首条校验前取得写锁；保留凭据沿已开放 ticket 链延续；`unchanged` 同时比对 turn。新发现 CJSI-USAGE-R1-N1（P1）余量键含可选分项致跨副本双计；N2（P2）F7 未闭环，`ExactRun` 部分重叠与跨副本错绑；N3（P2）区间模型未核对被包含原生请求。
- **R2 复评（AGY Gemini 4 Hydrogen high）FAIL。** N2、N3 closed。CJSI-USAGE-R2-N1-RESURRECT（P1）稀疏副本、跨副本拆分端点或跨辅助字段矛盾可复活已拒余量；CJSI-USAGE-R2-T1（P3）`CompleteEnd` 上界无测试。
- **R3 复评（Claude Opus high）。** R1-N1、R2-N1-RESURRECT、R2-T1 closed：余量按键统一发布，拒绝全局单调保留至终判，合并后逐维度重扣已证请求；12 组排列性质测试。新发现 CJSI-USAGE-R3-T1（P3）已拒组不挂旧别名、跨副本时间窗合并无测试保护；CJSI-USAGE-R3-O1（P3）副本仅缺时间戳却永久否决完整副本余量。用户批准第 4 轮修复。
- **R4 复评（AGY Gemini 4 Neon high）PASS。** R3-T1 closed：排列测试断言被否决镜像只留原生证明、时间窗取最早起点与最晚终点。R3-O1 closed：区分时间缺失与时间矛盾，缺时间副本仍参与计数校验但不提出也不否决窗口，全缺时间时保留已证调用并报 `UnallocatedTokens`，倒置与计数矛盾仍否决；评审者对放宽后的矛盾、全缺、倒置、单独存在与拆分交互全排列复现，均不双计、不发布矛盾余量。本轮无新发现。
- **验证。** `TestCJSIUsageLedgerReplay` 与 `internal/usage` 全包通过，作者 race 通过；gofmt、vet 干净；`cjsiledger` 导出 API 自 R1 起逐字节不变（Task 4 消费者兼容）；历轮复现与 26 个变异全部成立，其中 6 个为本轮评审者新设计。测试仅用 `t.TempDir()`。
- **门禁。** CEv1 WorkUnit `codex-jsonl-session-identity:usage-ledger-replay` 在该补丁状态上独立评审、L2 验证、范围与隐私 3/3 满足。真实 Task 4/7/8 的所有权换侧、聚合读取与最终发布 CAS 由后续 Task 联合验收。
