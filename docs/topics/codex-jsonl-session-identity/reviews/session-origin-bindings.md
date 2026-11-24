---
status: active
topic: codex-jsonl-session-identity
subject: session-origin-bindings
---

# 会话起源与覆盖绑定评审

## Round 1 — 2026-11-20，逻辑绑定与选择切片

### 📋 独立评审

📊 本轮范围评分 7/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

**CJSI-ORIGIN-R1-F1**（P1）。已结算的 OTel 投影将首个有效事件写入 `session_metadata.first_at`。迁移建立的 base group 没有 `head_at` 或 `birth`，实体读者因而返回有效的 `Source.FirstAt` 和空的 `Birth`。`readEffectiveTarget` 将私有起点替换为空 `Birth`，为 09:05 的既有完整匹配生成 limited 输入，导致分类详情不能应用，core-only 列表仍可能报 matched。准备使用相同前后副本也会失去匹配。原测试模拟把 09:05 写入了 `Birth`。通过经认证的同票已结算投影保留 OTel-only 的首时刻，并在实际覆盖读回中验证一致性。原生副本继续排除未经验证的 `first_at`。

**CJSI-ORIGIN-R1-F2**（P1）。原生旧副本以已证 09:04 本地首请求作为 birth。新副本有 09:00 的已证头，且同一线程的唯一 09:04 请求仍有相同稳定响应 ID、时间和量纲。旧代际退役使保留副本检查失败，`requestContinues` 随后因旧证据源为原生而拒绝。旧 digest 变为 unmatched，违背确证连续时保持覆盖应用的合同。修复应使用实际证明过的旧原生首请求，仅在旧、新代际分别获准及现有身份、唯一性、分项和时间条件一致时迁移 digest。新增 SQLite 对照同时核对 revision、时间和收据保留。

#### 🟢 已核对的范围

已准入副本上，显示仍用 priority DESC、path ASC 胜者，birth 考虑所有合法副本。跨时区同一瞬时、撤销、缺席、父历史排除、保护后的 Work/Auxiliary/All 逻辑读和 stale set/clear 已有针对性证据。SQLite 锚点包含回滚及新开只读连接先查覆盖列表，再查详情、digest、revision、时间和收据。

#### 📝 摘要

Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task6-codex-ca94f0ef`。启动核对只收到通用项目规则和本轮简报。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `ca94f0efb2cd3221920e6528464fceebfda1266158ef6dd4f314e253a0a0926d`，Go `67b1e0b6f362b410598b301c4caaca98d0e9baf3d893c20ba7b31dffbef77b05`，五路径补丁 `5443ba54f56760340bd68a3410ca6ef8dff3b48e96cc04260bd5cef09a6bff5e`。主控核对 23 项校验和、五路径精确回放、实体 reader 字段及 binding 分支。独立评审核对源码和已有正常/race 的 45/46 条通过记录，两项发现来自源代码路径对照。Task 5/6 组合锚点分别 83 和 80 条通过，范围仍限联合编译与定向路径，CLI 尚未执行受管 Task 6 锚点。最终同票发布及公开调用方归 Task 8。

## Round 2 — 2026-11-20，绑定修复冷复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 组合验收条件

Task 8 须从同票 core 提供实际结算与首请求证明，将覆盖准备、读取和保护接入同票 CAS 与公开调用方。本文仅关闭已绑定的逻辑切片发现。

#### 🟢 已关闭发现

**CJSI-ORIGIN-R1-F1 CLOSED。** 实体迁移 SQLite 夹具的副本无 `HeadAt` 或 `Birth`，09:05 来自投影的 `FirstAt`。旧例先出现列表 matched、详情未应用，再在前后副本相同的准备时写成 unmatched。修复在同票要求已结算的规范标记及同一 UTC 瞬时，撤销或不符时丢弃结果。重开只读库后先查列表为 matched，详情应用 Auxiliary 覆盖，保留原 digest、记录与收据。原生副本仍从核实的头或 birth 推导。

**CJSI-ORIGIN-R1-F2 CLOSED。** 原本持有 09:04 首请求的原生代际退出，新的已证出生为 09:00。只有旧代际参与原 birth 且旧、新间有一个匹配本地请求时迁移 digest。错误代际、冲突、重复和无证明控制保留旧 digest 并报 unmatched。SQLite 实测保持用户值、revision、更新时间与收据，set/clear 仍胜出。

#### 📝 摘要

Codex `gpt-6.2-astra`，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task6-cold-r1-rereview-f526ffc2`。主题级通用背景之外，没有此修复候选的作者或已执行历史与裁定。产品 `f526ffc2dc1b2b4cc56f461e725a1c956c468196030a3060a2dde80b06b8d9c4`，Go `a2570e3261622a6e89f1ad7bce7ca305c4ff3ba6780efd9f7fbc3f01cbc259cc`，四路径增量 `d1610cc3b6e24180bb22fd60ccb25a89235d131003139c734d6b61f1a6776c0b`，主控独立核对 40 项校验和及四路径零 fuzz、零 offset 回放。逻辑锚点在普通和 race 各 80 条通过，实际迁移夹具两种模式均通过，vet 和格式正常。CLI 没有匹配的 Task 6 锚点。

## Round 3 — 2026-11-24，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 6，基于 HEAD `bbb06da`（Task 1–5 已提交）。初版作者 OpenCode `openai/gpt-6.2-astra` high；用户随后禁止再委派 OpenCode，三轮修复改由 Codex `gpt-6.2-astra` high 承接。最终补丁 SHA-256 `7abb6b46db9cf36e219a6542f7fa8ecb405f682c85433e4ab173dc2e16084606`，13 个路径，均在 Task 6 所有权内。

- **重建首评（Codex `gpt-6.2-sol` high）FAIL。** CJSI-ORIGIN-R3-F1（P1）受管 `session classify list` 直接分页 `origin_overrides`，列出已受保护、有效副本为零的目标；R3-F2（P1）私库已提交保护而 core revision 未对齐时，旧绑定仍能 set 成功；R3-F3（P2）`SelectEffective` 结果无保护戳却可交给 `NewPublication`，撤销后缓存保留旧文档；R3-F4（P3）批量选择逐会话重开私库快照，单 ID 查询全表枚举。R1-F1/F2 未复发。
- **R1 修复与复评（AGY Gemini 4 Neon high）FAIL。** R3-F1..F3 closed；R3-F4 在 `Select`/`Lookup` closed。R3-F2 的划分被认可：set 在 core 提交前复验写事务协调头与完整私库快照戳，最终核验至 core `COMMIT` 的窗口由 Task 8 交接屏障覆盖。新发现 CJSI-ORIGIN-R4-F1（P1）Codex-only 的 matched 硬失败误用于 Claude，first_at 变化后受管读面全局失败；R4-F2（P2）`ResolveEffectiveBinding` 不带索引戳、`SelectEffective` 对 Codex matched 矛盾未 fail closed；R4-F3（P3）`ListOverrides` 全表扫描且 `limit=0` 未取默认 20；R4-F4（P3）确认无索引时列表无条件报错。
- **R2 修复与复评（AGY Gemini 4 Neon high，新会话）FAIL。** R4-F1..F3 closed；R4-F4 的无索引分支正确。新发现 CJSI-ORIGIN-R5-F1（P2）索引存在时 `ListOverrides` 静默丢弃未索引的恢复 unmatched 覆盖与已删 Claude 会话覆盖，违背 cli-design.md 与 auxiliary-session-classification 的 list 契约。
- **R3 修复与复评（AGY Gemini 4 Neon high，新会话）PASS。** R5-F1 closed：同票内区分受保护/排除目标（继续隐藏，不计 total）与确属未索引目标（列为 unmatched，core 持久行不变）；core mask 无法认证或索引缺少仍 matched 的 Codex 目标时整页 fail closed；`Select`/`Lookup` 只返回真实会话；跨 200 键分页正确。R4、R3、R1 各项无回退，本轮无新发现。
- **验证。** 复评者在 `bbb06da` + 补丁的隔离副本运行：入口 56 项、`internal/sessionorigin`+`internal/store` 579 项、`internal/sessionorigin` race、CLI 相关 34 项、实体夹具普通与 race 各 11 项均通过；gofmt、vet 干净；6 个变异全部被新回归拦截；两组对抗探针（368 条混合覆盖跨批分页、单文档保护与来源替换退役）通过。测试仅用隔离目录。
- **门禁与交接。** CEv1 WorkUnit `codex-jsonl-session-identity:session-origin-bindings` 在该补丁状态上独立评审、L3 验证、范围与隐私 3/3 满足。Task 8 须在受管命令入口传入同票 core、在最终事务调用 `PreparedOrigin.ApplyTx`，并以交接屏障覆盖私库最终核验至 core `COMMIT` 的并发窗口；实际 backup/restore/purge 命令由 Task 9 验收；大索引逐目标核验的性能尚未测量，由 Task 10 综合验收覆盖。
