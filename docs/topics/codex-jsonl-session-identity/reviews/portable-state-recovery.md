---
status: active
topic: codex-jsonl-session-identity
subject: portable-state-recovery
---

# 可移植状态恢复评审

## Round 1 — 2026-11-20，备份详情顺序校验

### 📋 独立评审

📊 本轮范围评分 10/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟢 通过的范围

已解码的来源、客户端、会话分组与正数文档 ID 完整对应。每个组的 ordinal 唯一且连续，空组也须有映射头。AXBC 恢复后文档 ID 为 `1,4,2,3`。映射保留组校验标志和 ID 高水位，删除后高水位仍为 12。新版缺少或损坏映射会返回错误与零结果。已准入的旧版无映射时按每组 rowid 排序，标为未核实。

#### 📝 摘要

独立冷评审为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task9-portable-order-review-f66b4d0d`，authority epoch 21。启动时候选作者、执行历史及先前裁定仅来自本轮简报。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `f66b4d0d07bc4a3db2d13f43f895dbd713d46fa551a0d2708acb49adbfe48987` 共 192 路径，Go `2c6ebbfef6471a38fa546b8172c1932f7d6d75643b2f3d1f1d0e68951e5fd4c3` 共 178 路径。两路径补丁 `a749def94519c3611967c62a9d396d4a23f6709c0b3627c63ddb9a1336772da2`。主控对两个后像精确重放并核验全部清单和原始日志。备份包普通测试通过 90 条，定向 race 通过 27 条，均零跳过，vet 和 gofmt 通过。

通过范围为已解码的顺序和高水位校验。实际归档格式与提取、Task 8 同票准入、恢复重绑和公开分页仍待集成。

## Round 2 — 2026-11-25，重建 9a 备份与诊断纯校验

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

- **CJSI-PSR-9A-R1-F1（P2）。** `validatePortableSessionOrder` 未校验 `portable_session_order` 的会话键属于 `portable_session_groups`，也未校验键列为 TEXT，篡改归档可通过。
- **CJSI-PSR-9A-R1-F2（P2）。** Restore 对旧归档的 `sanitizeRestoredSession` 无测试保护，把它短路后测试仍全部通过。
- **CJSI-PSR-9A-R1-F3（P2）。** doctor 负控缺测试：`telemetry_jsonl_retracts_otel` 触发器残留的检查被删掉后测试仍通过；已开放 slot 的物理校验、`full=false`、`integrity_check` 失败与损坏围栏均未覆盖。
- **CJSI-PSR-9A-R1-F4（P2）。** `ErrStateBusy`/`SQLITE_BUSY` 被报为 `schema_incompatible`；`hasCJSILayout` 无 busy_timeout，可能回落为 `schema_ahead`。
- **CJSI-PSR-9A-R1-F5（P3）。** `validateCoreSnapshot` 未像 `preparePortableCore` 一样拒绝 schema≤36 核心归档中的 `cjsi_*` 对象。
- **CJSI-PSR-9A-R1-F6（P3）。** doctor 核心表清单缺 `cjsi_usage_interval_dirty_ranges`；`preparePortableSession` 以字符串拼接 SQLite URI。

#### 🟢 通过的范围

受管布局的实际接线（同票导出收据与覆盖、受管 Restore 的 epoch 重绑与围栏、受管 doctor 跨域检查、CLI/watch）推迟到 9b，符合 tasks.md C3 行“备份/诊断纯校验可先于 8 独立”的划分。旧入口对受管布局在写目标前拒绝，无副作用。doctor 只报计数，不泄露原文或路径。

#### 📝 摘要

重建版 9a 作者 Codex `gpt-6.2-sol` medium，基线 `3e3afb5`。AGY `gemini-4-hydrogen` high 冷评审，请求 `v075-e21-cjsi-rebuild-task9a-review-c873eff2`，补丁 `c873eff20485226a3a3de11fc47dd280708f5a1e41778d7d0f07b37fd7f25d46`，9 个路径。三包全量 163 项通过、1 项原生注册验收跳过；race、vet、gofmt 通过；作者 4 个变异被杀，评审变异中两个逃逸。

## Round 3 — 2026-11-25，9a 第 1 轮修复复评

### 📋 独立复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

CJSI-PSR-9A-R1-F1 至 F6 全部关闭。上轮逃逸的两个变异与作者新变异均被杀。

#### 🔴 新发现

- **CJSI-PSR-9A-R2-F1（P2）。** `COLLATE BINARY` 只加在一个 EXCEPT 上。篡改归档把列声明为 `COLLATE NOCASE` 后，GROUP BY 会把 `'A'`/`'a'` 合并，绕过每组 `detail_ordinal` 连续性；`session_metadata` 与分组表的大小写错配也能通过。两个探针均返回 `err=nil`。
- **CJSI-PSR-9A-R2-F2（P3）。** 布局探测错误与私有索引运维错误的透传分支无测试（两个变异逃逸）。同一根准入锁经公开 `Service.Check` 报 `database_unreadable`，经内部检查报 `state_busy`，诊断码不一致。

#### 📝 摘要

AGY `gemini-4-neon` high 冷复评，请求 `v075-e21-cjsi-rebuild-task9a-r1-rereview-97c0f7bb`，补丁 `97c0f7bba8535cf4e005bc1b183e6125a5dbedb0ebe8686279a03b95534f353a`，11 个路径，修复轮 1/3。三包全量 168 项通过、1 项跳过；race、实体物理测试及其 race、vet（含 tagged）、gofmt 通过。

## Round 4 — 2026-11-25，9a 第 2 轮修复复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

- **CJSI-PSR-9A-R2-F1 CLOSED。** 会话三元键的全部双向集合差与两处 GROUP BY 都显式使用 `COLLATE BINARY`。上轮两个探针现在均以 `invalid_backup: session document order` 被拒；去掉 `COLLATE BINARY` 的变异被杀。同文件及 `validateSessionArchive`、`scrubLegacySession`、`validateCoreSnapshot` 等集合比较未发现同类大小写绕过。
- **CJSI-PSR-9A-R2-F2 CLOSED。** `ErrStateBusy` 统一经既有 `databaseCode` 映射为 `database_unreadable`，与公开入口一致，未新增诊断码或 JSON 字段。经公开 `Service.Check` 的测试覆盖两个根锁竞争窗口、布局探测受阻与私有索引被锁。两个原逃逸变异与评审新变异均被杀。

#### 🟢 通过的范围

更早的 F1–F6 无回归。测试全部使用临时目录和隔离 HOME；断言基于返回值、归档字节、SQLite 行与诊断输出。无 P0–P3 新发现。

#### 📝 摘要

AGY `gemini-4-hydrogen` high 冷复评，请求 `v075-e21-cjsi-rebuild-task9a-r2-rereview-ac1adee2`，补丁 `ac1adee29bece9197d666087c682d4c1f58a843e1940b53934d6c893ac860703`，11 个路径，修复轮 2/3，作者 Codex `gpt-6.2-sol` medium。定向 27 项通过；三包 `-p 1` 全量 169 项通过、1 项跳过；race、实体物理测试及其 race、vet（含 tagged）、gofmt 均退出 0；作者 33 个变异全部被杀。Task 9 整体门禁待 9b 接线与 Task 8 联合验证后关闭，tasks.md 的 Task 9 行暂不勾选。
