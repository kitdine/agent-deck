---
status: active
created: 2026-11-18
---

# Codex JSONL 子会话身份与用量归属 — Architecture Review

## Round 1 — 2026-11-18

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、architecture.md blob `68c868e8cb79d991c4344009cc18602a7d350e53` 和指纹 `3ba86706a11d1670d83b83f5fb9d3a43a54fd518139680c801ea437aa5b46963`。输入需求 blob 为 `f5ebb617dd1acf54f3728712adc5f2f518d343e2`，对照 main 为 `a224b0b824d2cad451d3bd83153832d65506f0fe`。

Claude Sonnet 5.5 冷独立复评，请求 effort 为 medium，实际档位未单独观察。评审检查架构、需求与实际 Go/SQLite 入口。主控重算候选指纹，以工作树 CodeGraph 定位并核对触发器、发布者、分区版本、旧版打开和 FTS 查询源码。范围为设计核对。CEv1 返回 `missing_work_unit`。

评审结论：FAIL

完成门禁：NOT_VERIFIED

### 高优先级

- **CJSI-ARCH-R1-F1（高）**。现有 `telemetry_jsonl_retracts_otel` 在非 OTel Codex 事件插入后直接撤销旧 contribution 和 marker。`publishFacts` 也会单凭非空线程集合撤销，即使 events 为空。迁移明确移除该触发器，并用同事务的按线程重评取代所有隐式撤销。先记恢复收据，再发布和撤销。A5 覆盖裸 INSERT、仅头及尚未开放的候选。
- **CJSI-ARCH-R1-F2（高）**。现有分区触发器与 serving revision/coverage 表清单不包含新区间账、收据或开放票据，投影缓存还使用全局 parser 版本。定义公开区间与开放动作如何标记受影响的客户端、模型、日期和覆盖。未知模型使用内部识别，公开读面沿用需求。区分待开候选与已开放结果的 revision 行为，定义按来源的 parser/cache 身份。A4b/A6c 核对标脏及存量读面。
- **CJSI-ARCH-R1-F3（高）**。写 `session_sources` 或 `session_metadata` 会立即改变现有 revision 与覆盖，待开放差分需要独立候选结构。同时，`session.go` 的 List/Search/Details、`affected_view.go`、`scope_snapshot.go`、`session_origin_derive.go` 及 CLI/OTel 等入口各自读底表。给出实际访问清单、唯一有效视图和禁止业务读旁路的护栏。备份、诊断和受管迁移的原始访问须有明确边界。
- **CJSI-ARCH-R1-F4（高）**。FTS 搜索以 rowid 做最终排序，文档分页也按 rowid，独立候选库的默认 rowid 会冲突。明确稳定 ID 的分配、候选对底表的遮蔽键、合并后排序与总数、compact 的显式保留及 A6c 的等序断言。
- **CJSI-ARCH-R1-F5（高）**。旧 `OpenSessionsReadOnly` 只检查两张表存在，旧写路径会按结构自动建表和重建索引，core schema-ahead 或新版本记号本身不足以保护全部入口。选定兼容围栏，逐一说明可达的旧打开、自动重建及破坏性管理路径为何被拒或处于受支持保证内。补可运行的旧版读写和迁移失败用例。

### 其他待修订项

- **CJSI-ARCH-R1-F6（中）**。分别列出 usage summary/range/sessions、session usage、signals、stats、diagnose 和桌面分区入口的区间接法。区间只贡献 token 和合格会话计数，调用数仍按实际请求。定义字段存在性、reasoning、份额分母、分区 payload 版本及坏数据与真实零值。
- **CJSI-ARCH-R1-F7（中）**。列齐 usage 与 session parser 的比较、快路径、恢复和写检查点位置，集中按 client 与来源类型选目标。说明缓存键与收录后的版本联系，并验证 Claude 和合格 OTel 不因 Codex 格式提升重读。
- **CJSI-ARCH-R1-F8（中）**。用实际跨域输出依赖判定 usage-only 是否持有候选。明确持有边界、覆盖提示和组合扫描出口。验证普通追加可发布，跨域候选续作后清提示。
- **CJSI-ARCH-R1-F9（中）**。定义字段级 OTel 恢复收据，包括原 state/reason、观察与事件键、接管期、原 session/usage 范围、来源/模型/时间及准确复原计量所需元数据。保持既有状态枚举，按同一接管期幂等重评和恢复。
- **CJSI-ARCH-R1-F10（中）**。main 除预聚合 `signals.go` 还有 `internal/watch/watch.go` 的取消与释放差异。把该调用者列入对账，复用 `ErrTelemetrySourceIdentityPending`、`scan_pending` 和 `source_identity_pending`。
- **CJSI-ARCH-R1-F11（中）**。明确双库中途备份只保留哪些公开状态、待开差分与隐私、恢复时旧票、epoch 与派生证据重绑。为 backup/Restore、doctor 的新表健康判断与故障用例给出实际合同。
- **CJSI-ARCH-R1-F12（低）**。A1–A7 增补 OTel 撤销触发器、区间账与 revision、旧版拒绝、读取护栏及中途备份恢复断言。
- **CJSI-ARCH-R1-F13（低）**。把风险说明连到已确认的旁路清单，明确新增计数与现有 perftrace 记录的位置及语义。

明文来源、既有公共读面、用户绑定及其他客户端范围已确定。修订应保留 R3 已通过的需求。若源码证明现有旧版兼容合同无法实现，须先提出具体冲突与可选方案。

### Round 1 修复交接

原 Codex Sol Worker 完成 F1–F13 的修订，T3 报告 `reasoningEffort=medium`。主控核对修订 SHA-256、已批准需求和工作树基线。修订包含显式 OTel 换侧与恢复收据、私有候选及公开 revision、有效索引查询与稳定 rowid、按来源版本、定向持有、可移植备份和断点恢复。

候选 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，architecture.md blob `c1d9fceb88d80dcc6511e1d15b02ba8111f72b46`。文档审计、空白和限定路径 diff 检查通过。旧版围栏依赖受管迁移窗口和旧连接排空。旧二进制拒绝、驱动行为和锁顺序由 L3 验收，十三项待独立复评。

## Round 2 — 2026-11-18

### 📋 JSONL 架构复评

📊 综合评分：8/10

✅ 复评结论：FAIL

### 🔴 必须修复

**CJSI-ARCH-R2-F1（中，承接 CJSI-ARCH-R1-F4）**。§7.2 规定新文档取更大的 `doc_id`，并保留可确证旧行的 ID。`DocumentsPage` 与 `ShowWithActivity` 当前都按 `rowid` 排列详情。若来源在旧行之间新增一条，详情会把新行放到末尾。主控用合成 SQLite 数据按 rowid 得到 `ABCX`，按来源位置得到 `AXBC`。搜索的 tie-break 和总数已有设计，详情的页序、`--all` 和 activity 读取仍缺顺序契约。明确按来源重解析后使有效详情顺序与冷建一致的内部规则，保留搜索和 compact 的既定边界。在 A6c 覆盖中段插入、详情全读和分页的顺序及总数。

### 🟡 其他改进

**CJSI-ARCH-R2-F2（低，新发现）**。§7.3 第 3 步把候选 serving slot 的写入、文件及目录 `fsync` 放在 core 写事务内，同时要求重 I/O 留在写锁外。组大小或事务取消无法单独限定同步文件 I/O 的等待。明确 slot 生成、持久化、seal/CAS 校验、开放和失败恢复各自的事务边界。验收需能证明 slot 写失败或等待时仍保留安全旧代。

### 🟢 已关闭

- **CJSI-ARCH-R1-F1、F2、F3** 已关闭。显式 OTel 换侧和先记收据、私有候选与公开 revision 分离、有效读层和原始 SQL 的访问边界均已说明。
- **CJSI-ARCH-R1-F5** 已关闭。旧库打开受围栏约束。复评以非 SQLite 头文件验证旧路径的 WAL pragma 和只读 schema 查询返回 NOTADB。受管升级仍须排空此前已打开的连接，旧二进制完整矩阵留到 L3。
- **CJSI-ARCH-R1-F6、F7、F8、F9、F10、F11、F12、F13** 已关闭。覆盖各用量入口、按来源选版本、定向持有与续作、可逆接管收据、目标 main 的 watch 行为、备份恢复及 doctor、A1–A7 映射和诊断计数。**CJSI-ARCH-R1-F4** 的详情顺序由 **CJSI-ARCH-R2-F1** 继续承接。

### 📝 小结

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、architecture.md blob `c1d9fceb88d80dcc6511e1d15b02ba8111f72b46`、指纹 `f2247d802c0b67cdecb341a0944a645f2ac64d54411b10af63a64f6115e97db9`。T3 确认冷独立复评运行 `claude-sonnet-5-5`，报告请求选项 `effort=medium`，实际推理档位未独立观察。主控重算候选，以 CodeGraph 定位详情入口，核对 SQL 和架构条款，并完成上述最小合成复现。复用同候选文档审计、空白和限定路径 diff 检查。L3 旧二进制、持锁 I/O 故障和真实恢复尚属实施验收。

完成门禁：NOT_VERIFIED

精确候选查询返回 `missing_work_unit`。分解和实现继续等待架构通过。

下一步指令

修复：codex-jsonl-session-identity / architecture.md / CJSI-ARCH-R2-F1、CJSI-ARCH-R2-F2

### Round 2 修复交接

复用原 Codex Sol 6.2 Worker。T3 确认修复运行 `gpt-6.2-sol` 并报告 `reasoningEffort=high`。详情顺序在选中的来源组内以独立 ordinal 表达，旧 ID 保留。串行查阅、分页、压实和恢复共享同一候选顺序。候选派生计算与 slot 持久化移出最终 core 写事务。最终事务比较 seal 和输入修订后应用事实并开票。A6c 新增中段插入、分页、候选持久化等待和失败、CAS 冲突及重启断点的验收。

修订候选 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，architecture.md blob `a4f258224d7380cfbf7b018662793f8e3245b6af`，指纹 `8e9fabb8634a30cf1af1a39f905a98a621574c76c85d09a9d73b2f4c3a449ccc`。主控核对限定 diff、原需求 blob、文档审计、空白和限定路径 diff 检查。两项继续等待独立复评，真实产品异常注入属于后续 L3。

## Round 3 — 2026-11-18

### 📋 JSONL 架构复评

📊 综合评分：9.5/10

✅ 复评结论：PASS

### 🔴 必须修复

无。

### 🟡 其他改进

无。

### 🟢 已关闭

- **CJSI-ARCH-R2-F1 CLOSED，同时关闭 CJSI-ARCH-R1-F4。** §7.2 为选中来源的有效文档定义组内 `detail_ordinal`，由经过验证的源记录位置和文档发射顺序确定。CLI、viewer 和全量读共享同票的 metadata、total 和文档顺序。换票时重新选页，activity 仍使用同票范围核验。旧索引和旧归档保留安全旧序，重解析验证完整组。搜索保留 `doc_id` 与 tie-break。主控复用合成 SQL 对照 `ABCX` 和 `AXBC`，核对 A6c 对中段插入、分页空尾、压实及备份恢复后的断言。
- **CJSI-ARCH-R2-F2 CLOSED。** §7.3 在只读快照上生成同代候选，释放可能阻塞其他写者的锁及两库读快照后才写并同步私有 slot。文件和目录均持久化后登记 immutable seal。最终 core 写事务核对票据、来源、修订与 seal，再应用同一份差分并开票。主控用 CodeGraph 核对现有 serving 在不同连接读取两库的位置，并核对 A6c 的并发写入、旧版读取、迟到 attempt、CAS 冲突和重启断点。真实 `fsync` 屏障和恢复仍是实施验收。
- Round 1 其余十二项的关闭继续有效。限定修订未改变明文来源、现有公开读面、用户绑定或其他客户端边界。

### 📝 小结

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、architecture.md blob `a4f258224d7380cfbf7b018662793f8e3245b6af`、指纹 `8e9fabb8634a30cf1af1a39f905a98a621574c76c85d09a9d73b2f4c3a449ccc`。已批准需求 blob 为 `f5ebb617dd1acf54f3728712adc5f2f518d343e2`。独立复评通过 Antigravity 派发 Gemini 4 Argon，T3 报告请求选项 `thought_level=medium`。Worker 自报 Argon Medium，T3 未独立回报实际模型。主控核对回执、候选、两项设计修订和同态检查。

完成门禁：VERIFIED

CEv1 精确目标通过 4/4，覆盖契约、发布设计、格式检查和独立复评。四项证据及八条关系已读回。主题文档审计、评审记录结构、空白及主题限定 diff 检查均退出 0。

任务检查点 `ad-codex-jsonl-session-identity-doc-arch-design`，指定状态 `codex-jsonl-session-identity:architecture.md:state:8e9fabb8634a30cf1af1a39f905a98a621574c76c85d09a9d73b2f4c3a449ccc`。本架构及评审记录建议单独提交，再随该 issue 交付。推送按 v0.7.5 的逐 issue 授权执行。实施前按届时目标 main 核对迁移顺序。
