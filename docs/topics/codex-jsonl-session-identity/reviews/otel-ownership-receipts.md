---
status: active
topic: codex-jsonl-session-identity
subject: otel-ownership-receipts
---

# OTel 归属收据评审

## Round 1 — 2026-11-20，C2 收据消费切片

### 📋 独立评审

📊 本轮范围评分 7/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无 P1。

#### 🟡 需修复的问题

**CJSI-OTEL-R1-F1**（P2）。恢复规划允许已有 JSONL 事件和 `usage_sessions` 摘要并存。独立撤出回调若只移除有证事件，恢复的普通 INSERT 触发会话行唯一约束，整笔回滚，无法恢复 A5 的原 OTel 侧。旧夹具漏建真实发布者会写的摘要。修复在 Task 4 同一事务内复验并按明确归属处理完整旧摘要，再恢复原摘要。独立后续改动须拒绝覆盖并回滚。

**CJSI-OTEL-R1-F2**（P2）。恢复只复查准备时记录的事件键。独立探针在准备后增加本线程 JSONL 行且 revision 不变，应用成功并保留双侧。撤出后在同一写事务复查完整线程，在仍有 JSONL 行时拒绝恢复。Task 8 仍须让实际写者推进完整 revision。

#### 🟢 通过的范围

独立资格复核、仅已证请求接管、待核与部分态、先前收据及更晚盘点、原归因、邻居保护和失效观察等范围通过。已有 SQLite 故障夹具证明事务回滚。省略 overlap 或录入时间检查的变异均失败。

#### 📝 摘要

独立 Claude Code Opus `claude-opus-5-5`，T3 配置 `effort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，147 路径切片 `a7b2ac8a360f3a0a10aac280122f2fcac6c2df98bfc417db3bf74e80d62712d5`，133 路径 Go `0657351cf58b47a2d02441624a6bfbbc1c103061f28913c654c409730fc1d798`，四路径补丁 `e3bba2ea991da4575e7f549e6caa67aa6be83d2ecdb40697b790ec6d2bfb27a0`。组合 153 路径 `d41ac94e29846866e0a9e4553e18e811158e0cfbd9138fc3af3846464d1b97a6`。Go 1.27.3 组合定向 Ownership、Reconciliation 及 race、build、作用包 vet 均通过。独立探针复现上述两个缺口，主控核对了会话写入和旧行复验。

实际冷建触发器创建、结算入口接线、全量 store/telemetry 回归和 Task 8 同代事务依原任务负责。沿用 [任务 3](usage-ledger-replay.md) 的 A4/A5 适用条件。

## Round 2 — 2026-11-20，收据恢复复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 组合验收条件

Task 8 必须让实际依赖写者推进线程 revision，并在同一事务内复核后再撤出或恢复。独立探针中，准备前已有 OTel 计量会被拒绝。准备后出现新 OTel 计量、revision 正常推进时，操作在撤出前被拒，计量、摘要和接管期均保留。如果故意让该写者绕过 revision，失效释放能提交并删除摘要，新 OTel 事件、贡献和 marker 仍存在。该实验纠正修复回执中“所有迟到 OTel 均因数据库约束失败”的说明，供 Task 8 实际写者验收。

#### 🟢 已关闭发现

**CJSI-OTEL-R1-F1 CLOSED。** 独立撤出只删除事件时，事务按准备时保存的全行镜像校验并清理 JSONL 时期摘要，再恢复原 11 token、事件键、时间、run 绑定、摘要和 marker。摘要被并发修改时返回前置条件错误，撤出和收据更新同时回滚。失效释放清理无事件支撑且完全匹配的旧摘要。

**CJSI-OTEL-R1-F2 CLOSED。** 撤出后在同一写事务复查整个规范线程的非 OTel 事件。后插入的同线程 JSONL 行阻断恢复和失效释放，即使 fixture 冻结 revision。UUID 大小写变体同样被拒，其他 ID 严格相等。相邻 JSONL 和 OTel 数据保持原值。

#### 📝 摘要

OpenCode Pro `openai/gpt-6.2-astra` 冷上下文独立复评，T3 `variant=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，起点组合清单 `d41ac94e29846866e0a9e4553e18e811158e0cfbd9138fc3af3846464d1b97a6`。三路径补丁 `a485aa27a30ca1e562c01c2c73e51b06db4e52c942bef8ecdc6ec929092506f5`，产品 `e6c4c8ab0653399d2addfb21da9567a0733595d016668069cd941b30ecd49259`，Go `8abd0f3c11ceeb21cf2ba217e1e13c57b2e8f55a8d8f6deef7255490cea212c6`。独立复评重放三路径并比对组合清单，主控核对补丁、摘要镜像处理、线程复查和红绿日志。

两项收据锚点普通与 race 均 6 条通过。Store 与 Telemetry 共 224 个顶层测试通过，另有三个跳过。旧 F1 报唯一约束，旧 F2 错误接受未撤出的 JSONL 行，省略线程复查、放宽摘要保护、区分 UUID 大小写和保留旧摘要的四个控制均失败。此轮关闭收据恢复发现，正式接管、结算入口、实际 revision 写者、迁移与同代发布验收仍开放。

## Round 3 — 2026-11-23，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 4，初版作者 AGY Gemini 4 Neon high，R1、R2 修复由 Codex `gpt-6.2-astra` high 接手。最终补丁 SHA-256 `cbee6ba507bf1732f82c6b578663dee2fba8b574e5ee895167463307bae54b32`，11 个路径（含新增外部测试 `internal/store/cjsi_telemetry_projection_test.go`），基于 HEAD `119ec60`。

- **重建首评（Codex astra high）FAIL。** R1-F1、R1-F2 与 Round 2 组合条件未复发。CJSI-OTEL-RB-F1（P1）同一 UUID 大小写变体绕过冲突并重复入账；F2（P1）旧盘点越过当前盘点的同线程线索，仅 `update_pending` 时仍入账；F3（P1）schema 37 下 `SettleAndProject` 先提交 core 再拒绝；F4（P1）仅 `CatalogVersion` 的计价输入接管后无法释放；F5（P2）Claude 专属 checkpoint 的同名 `session_id` 阻断 Codex 晋升。
- **R1 复评（AGY Gemini 4 Neon high）FAIL。** RB-F1..F5 closed：统一线程匹配、晋升复核当前盘点与未核前缀、组合调用在 prepare 前拒绝、计价表示无损往返、只把可归属 Codex 的线索纳入阻断。新发现 CJSI-OTEL-R1R-F1（P2）冲突拼写计数漏 metered 的 `capture_only`；R1R-F2（P3）部分失效释放后 marker 与 session 写回已失效拼写。
- **R2 复评（AGY Gemini 4 Neon high）PASS。** R1R-F1 closed：拼写计数与冲突谓词一致，纯 `capture_only` 线程不进入状态机。R1R-F2 closed：部分释放按存活事件二进制最小 `session_id` 重算公开线程。上一轮 5 个与本轮 3 个新变异全部被杀；R0 五项、R1-F1/F2、Round 2 条件与合法路径（非 UUID 严格匹配、usage-only 结算、真实归因变化阻断释放、Codex 孤儿旧账保护）不回归。本轮无 finding；评审者另记一项观察：将释放时的 `min` 改为 `max` 只被其独立多存活探针杀死，仓库测试仅有单存活用例，生产逻辑已核对与 `syncTelemetrySession` 一致。
- **主控复验。** 评审时依赖的是 Task 3 未定稿的 `cjsiledger` 快照。Task 3 提交为 `e2b7f20` 后，主控在 `e2b7f20` + 本补丁的隔离副本上复验：11 个路径 SHA-256 与清单一致；gofmt、vet 干净；`^TestCJSIOTel` 定向、`internal/telemetry` 与 `cmd/agentdeck` 的 Telemetry 回归、`internal/store` 与 `internal/usage` 全包（-p 1）均通过。HOME、TMPDIR、GOCACHE 隔离。
- **门禁。** CEv1 WorkUnit `codex-jsonl-session-identity:otel-ownership-receipts` 在该状态上独立评审、L2 验证、范围与隐私 3/3 满足。实际 revision 写者、Task 5 物理投影与 Task 8 同代发布事务由后续 Task 联合验收。

## Round 4 — 2026-11-25，Task 8 单元 2a 上游修改登记

评审结论：PASS

完成门禁：NOT_VERIFIED

Task 8 单元 2a 经主控授权修改本锚：
- `internal/store/cjsi_otel_report_effects.go` 新增 `(*CJSIOTelChange).ReportEffects` 与 `VerifyReportVacated`，在借用事务中返回 Apply 将产生的事件、运行绑定与会话增删；
- 只读侧与 Apply 共用 Prepare 完成失效筛选后的同一 `edits`；
- `cjsi_telemetry_receipts.go` 与 `cjsi_telemetry_rows.go` 把 vacate 检查提取为共享的 `checkVacated`，新增导出的只读视图接口 `CJSIOTelReadView`。

Apply 与 vacate 的事务、SQL、错误和写入顺序不变。测试断言只读效果集与 Apply 后数据库的实际增删逐行相等。同代发布评审 Round 17–19（`same-generation-publication.md`）一并冷评审，最终补丁 SHA-256 `c7b44ccad9a4d16b9c2309dd47a2eb2b3b2715649b27462d9d89ae580fd2a6a9` 复评 PASS。复评者对照 `3e3afb5` 逐行核对：原有入口的行为、错误、写入与 SQL 不变，相关包全量测试通过。本锚此前的完成证据针对旧内容状态，新内容的证据由 Task 8 联合门禁与最终 L3 承担。
