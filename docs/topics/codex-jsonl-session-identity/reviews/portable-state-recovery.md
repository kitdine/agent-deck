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

## Round 5 — 2026-11-25，9b-a 受管备份导出与恢复重绑首评

评审结论：FAIL

完成门禁：NOT_VERIFIED

9b-a 实现受管 core 的同票导出和受管 Restore 的 epoch 重绑与围栏。作者是 AGY `gemini-4-neon` medium，基线 `6d255ed`。独立冷评审由 Codex `gpt-6.2-sol` medium 执行，请求 `v075-e21-cjsi-rebuild-task9b-a-review-4c6c7900`。评审在交出最终结果信封之前被 OpenAI 内容过滤拦截，主控逐个复核了它留下的探针。变异 M1、M2、M5、M7、R1、R2 均被杀死。

- **CJSI-PSR-9BA-R1-F1（P1）**：Restore 遇到 `ErrTargetNotEmpty` 后进入回滚，会删掉并发属主写入的文件。rollback-locked 探针可复现。
- **CJSI-PSR-9BA-R1-F2（P2）**：旧版迁移中，未经校验的 portable 表会误触发 portable 导入分支。该用例在基线 PASS，在候选 FAIL。

## Round 6 — 2026-11-25，9b-a R1 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R1 候选 `8924bd46`，7 个路径，由同一位 AGY 作者修复。Claude `claude-opus-5-5` high 冷复评。

- CJSI-PSR-9BA-R1-F2 closed。
- CJSI-PSR-9BA-R1-F1 只关闭了一部分，剩余部分由下面的 R2-F1 取代。
- **CJSI-PSR-9BA-R2-F1（P2）**：回滚按名字删除预留文件。
- **CJSI-PSR-9BA-R2-F2（P2）**：测试钩子使用了非规范化的 TempDir。
- **CJSI-PSR-9BA-R2-F3（P2）**：身份证明没有测试覆盖，变异 MX1 存活。
- **CJSI-PSR-9BA-R2-F4（P2）**：收据里的正则实际执行了 0 个 backup 测试。
- **CJSI-PSR-9BA-R2-F5（P3）**：等锁行为没有测试，变异 MX2、MX3 存活。
- **CJSI-PSR-9BA-R2-F6（P3）**：`recordOwnedRestoreFile` 的错误被吞掉。

## Round 7 — 2026-11-25，9b-a R2 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R2 候选 `7d97b160`，7 个路径。AGY 周额度用完，作者改为 Codex `gpt-6.2-sol` high，15 个变异全部被杀死。评审先派给 OpenCode，被 OpenAI cyber_policy 拦截，改由 Claude Opus high 在原评审线程续评。

- CJSI-PSR-9BA-R2-F1 到 F6 closed。
- **CJSI-PSR-9BA-R3-F1（P3）**：恢复成功后留下 sidecar 预留文件。
- **CJSI-PSR-9BA-R3-F2（P3）**：变异 MY2 存活（sameRoot）。
- **CJSI-PSR-9BA-R3-F3（P3）**：变异 MY3 存活（创建身份）。
- **CJSI-PSR-9BA-R3-F4（P3）**：密钥身份取自 link 之后的 Lstat，需要改 credentialvault。

## Round 8 — 2026-11-25，9b-a R3 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R3 候选 `f0dbbabea968b2fa179d0a7d2fb4145aa13644acd535a1643929d4caccdcb852`，9 个路径。主控授权修改 credentialvault：新增未导出的 `observeNewKey` 和 `WithNewKeyObserver`，`InitializeNew` 签名不变，原有 vault 测试一行未改。Claude Opus 复评，backup、session、store、credentialvault 全量和物理 race 都退出 0。

- CJSI-PSR-9BA-R3-F1 closed：sidecar 按创建身份逐个清理，身份不符时整体回滚。
- CJSI-PSR-9BA-R3-F2、F3 closed：MY2、MY3 都被作者新增的用例杀死。
- CJSI-PSR-9BA-R3-F4 closed：身份取自临时文件描述符的 fstat，观察者为 nil 时行为与原来一致。
- **CJSI-PSR-9BA-R4-F1（P3）**：恢复成功后的 sidecar 清理改成按名删除（变异 MY5）时，没有测试能发现。

## Round 9 — 2026-11-25，9b-a R4 复评（接入恢复标记）

评审结论：FAIL

完成门禁：NOT_VERIFIED

R4 候选 `3d49f41d62ded57daaf15e063f852a64874827cdf09a5a6457d0d314a24d3aec`，叠在 Task 2b R2 之上，接入恢复标记。Mark 放在持有两把锁、空目录复验之后，第一次 O_EXCL 之前。Clear 之前先用 `ResolveCJSIRuntimeRootForRestore` 按同一锁世代校验；只有确认根里只剩两把锁和标记时才 Clear。作者的 Codex 运行在收尾时被 cyber_policy 拦截，产物已经完整。物理 race 和全量测试的退出码由主控核对，均为 0。Claude Opus 窄复评。评审没有找到会误清标记的路径；"同用户不守锁协议的进程在身份核对与 unlink 之间改名"这一竞态是 POSIX 固有限制，接受为已知限制，并写入交接。

- CJSI-PSR-9BA-R4-F1 closed：三个替换 sidecar 的用例在两种 TMPDIR 下都杀死了 MY5。
- **CJSI-PSR-9BA-R5-F1（P3）**：Mark 已经创建标记之后又报错时，标记会永久残留，目标根从此被封锁。
- **CJSI-PSR-9BA-R5-F2（P3）**：`clearMarker` 的标记 inode 核对和锁归属核对都没有测试，变异 MZ4、MZ5 存活。
- **CJSI-PSR-9BA-R5-F3（P3）**：收据在运行完成之前就生成了，内容过期。

## Round 10 — 2026-11-26，9b-a R5 与 R6 合并复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R5 候选 `9d174bb81ef783381a2dae56ec6036263422417a2ff989242e5c7c4af1486a5f`，用于修复 Round 9 的发现。R6 候选 `58905df28bb11d5eb58e327910c199360e0424f1aefc9d5e5dfa21bf672fa7ce` 修复一个集成缺陷：扫描之后 `backup create` 拒绝有效 slot，因为 `verifyManagedPublishedState` 拿 attempt 的基底 `session_revision` 去比较发布头的结果 revision。按契约，这两个值天然不相等。主控把 R6 增量并入同一轮复评。Claude Opus 复评。

- CJSI-PSR-9BA-R5-F1 closed：Mark 报错后，只有标记内容与当前两把锁的绑定完全一致，并且回滚后根里没有任何其他残留，才 Clear。半写的标记、被换了 inode 的标记、锁已失效时的标记都会保留。
- CJSI-PSR-9BA-R5-F2 closed：MZ4、MZ5 都被 `TestCJSIManagedBackupClearMarkerOwnership` 杀死，主控也核对了这一结果。
- CJSI-PSR-9BA-R5-F3 closed：收据记录的退出码和日志摘要与磁盘上的一致。
- R6 修复正确：slot 由 `(ticket, attempt)` 主键，加上 seal、epoch 和 opened 状态唯一确定；其余校验和 `ReadServingSlot` 的复验都还在；回归测试走的是真实发布流程。
- **CJSI-PSR-9BA-R6-F1（P3）**：作者的变异 MZ8 漏写了 `AND`，被杀死的原因是 SQL 语法错误。评审写的正确版本 MV0 被新测试杀死。
- **CJSI-PSR-9BA-R6-F2（P3，改动前就存在）**：`verifyManagedPublishedState` 的 seal、opened、epoch、遮蔽、时间范围五项绑定都没有测试，变异 MV1–MV5 存活。

## Round 11 — 2026-11-26，9b-a R7 主控核对

评审结论：PASS

完成门禁：NOT_VERIFIED

R7 只改测试文件。累计补丁 `9abf5da340725552cc92028f3accd3f849f032582ed23917168702151ca5ff24` 共 9 个路径，基线是已提交的 Task 2b `6718d8a`；增量 `bbbe644e388d4dd1666604513a5672364be11415c6b04374f7f60832e7b33ea4` 只有 1 个路径。本轮只关闭 Round 10 的两项 P3，由主控核对变异结果，没有另派冷复评。

- CJSI-PSR-9BA-R6-F1 closed：MZ8 换成语法正确的 MV0，MV0 在两种 TMPDIR 下都被 `TestCJSIManagedBackupAcceptsPublishedSessionRevision` 杀死。
- CJSI-PSR-9BA-R6-F2 closed：新增的 `TestCJSIManagedBackupValidatesUnreadableSlot` 覆盖两种情况：
  - 存在遮蔽或保护 revision 已推进时，结构完整的不可读 slot 仍然允许 core-only 备份；
  - seal、epoch、开放状态或时间范围任一损坏时，返回 `ErrCJSIIndexChanged`，且不生成归档。
  
  MV1–MV5 在两种 TMPDIR 下都被这个测试的行为断言杀死。主控检查过日志，确认没有构建错误或 SQL 语法错误。

验证：主控在 `6718d8a` 加 R7 累计补丁的隔离副本上（临时 HOME、XDG、TMPDIR，umask 077）跑了 backup、store、credentialvault 全量，以及带 `cjsi_adapter_fixture` 标签的 backup 物理 race，全部退出 0，没有 DATA RACE。

本单元修改了其他锚点拥有的文件，这些作为上游修改，在 Task 8 联合门禁时登记：
- Task 2 / Task 5 的 `internal/store/cjsi_index_schema.go`、`cjsi_layout_production.go`；
- credentialvault。

恢复标记 API 归 Task 2b（`fenced-state-layout.md` Round 25–28）。受管归档的完整恢复路径，以及 Task 2b 中 `state_root_identity_mismatch` 的恢复前提，由 Task 8 联合门禁和最终 L3 验证。
