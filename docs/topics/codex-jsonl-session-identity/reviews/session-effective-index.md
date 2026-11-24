---
status: active
topic: codex-jsonl-session-identity
subject: session-effective-index
---

# 会话有效索引评审

## Round 1 — 2026-11-20，C2 读者与正文消费切片

### 📋 独立评审

📊 本轮范围评分 6/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

**CJSI-SI-R1-F1**（P1）。重新打开 viewer 页面时，`internal/session/cjsi_effective_reader.go` 的 `DocumentsPage` 只比较选中的路径。同一来源的票据或详情序列变化时仍会按新序列继续分页。独立 SQLite WAL 复现从旧票取得第一页 `AB`，关闭后切换新票，再以同一来源路径读取到第二页 `BC`。拼接结果 `ABBC` 重复 `B` 并漏掉 `X`。现有 viewer 测试注入旧视图错误，只验证错误传播。修复需携带期望的票据与详情序列身份，在重开页时复验选中来源及保护，使用原刷新或错误路径拒绝拼接。回归应实际通过读者和两个票据读取，而非注入预期错误。

**CJSI-SI-R1-F2**（P1）。`internal/session/cjsi_native_material.go` 的 `StageCodexSegment` 在 `ReduceWorkset` 内读取正文并加入文档。归约结束后才绑定独立捕获与封存证据和当前权威选择。等长正文替换的独立覆盖见证执行两次文档追加，再因摘要不合被拒，selection 复验未到达。结果最终为空，连接顺序仍早于 C1 双消费冻结要求。归约阶段应只暂存无正文的记录与段引用。通过两项复验后，才按本地线程和 turn、段范围、span 与材料类型连接正文。回归要在拒绝时断言零次正文连接，保留已接受材料、继承与未解析输入的现有测试。

#### 🟡 建议改进

无另行发现。

#### 🟢 通过的范围

固定一次 SQLite 读事务时详情为 `AXBC`，两条分页返回 `AX`、`BC` 和空页，total 均为 4。FTS5 等时命中按稳定 ID 返回 `ABCX`，选中替换在零 MATCH 命中时仍遮蔽已被替换的底表项。活动读取沿用已选票据、来源及合成夹具提供的保护/范围证明，六个定向旧路径回归通过。改用稳定 ID 排详情会产生 `ABCX`，放宽材料证据至仅比较进度会接收替换正文。

#### 📝 摘要

独立 Codex Astra `gpt-6.2-astra` 冷评审，T3 配置 `reasoningEffort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，C1 底座 `77e46aa5f8fc50ad618104d7c21a948c771ed4f2bfb41f1f617e4b19614e6e81`。七路径切片的产品清单 `4b38d552c6580f8d181c7f934343718ee0fc6b011efe2d680fe3df6e3b92d7d8`，Go 清单 `3a7ec8014b7276a65d8cb16b075558c01cbad2ae516caae468bf5d14b329f8d2`，补丁 SHA-256 `0c0e3846e5f143319496bf63121b3dc931a1194a844048549b8eea17bcf246e7`。独立评审核对七个前后像及变异和回执，主控核对对应实现的顺序与重开校验。两项 P1 在本 Task 5 修复范围内。

`EffectiveCopies(ctx, client, id)` 在调用方已验证的固定视图中，按优先级降序、路径升序返回合格副本，第一项为选中项。副本的来源身份、前缀摘要、parser、first_at、已证 head/birth 是 Task 6 的逻辑输入契约。底层 Store 准入、候选持久化、最终发布、compact 与 Restore 的完整验收仍归原阶段。

## Round 2 — 2026-11-20，分页与正文消费复评

### 📋 独立复评

📊 本轮范围评分 7/10

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

**CJSI-SI-R2-F1**（P1）。新增的旧路径全库 epoch/revision 校验会在无关会话写入或零来源扫描后使命中文档未变的旧 viewer、browser 和 CLI 页面读失败。旧 view 只选一次，出错后终止。恢复旧读取与元数据既有行为，把严谨票据、保护和详情序列检查保留在已准入的新索引视图。用实际旧路径的无关写入及零来源扫描证明兼容。

#### 🟡 需修复的问题

**CJSI-SI-R2-F2**（P2）。`TestCodexCanonicalProjectAttribution` 在新候选全包失败。不同库的旧 `List` 结果即使逻辑数据相同，也因各自新增的私有绑定不再相等。恢复旧元数据语义并在最终状态通过 session 全包。

#### 🟢 已关闭发现

**CJSI-SI-R1-F1 CLOSED。** 新索引重开比较完整票据、保护、seal、来源和详情序列。真实 SQLite 重开拒绝 AB+BC，外层拒绝时不返回分页内容。固定票据详情 AXBC、等时搜索 ABCX 保留。旧路径新增回归由 R2-F1 和 R2-F2 跟踪。

**CJSI-SI-R1-F2 CLOSED。** sink 暂存无正文引用。归约之后先比对独立字节证明，再复验当前 selection，随后按本地线程、turn、segment、span、kind 连接。摘要不符与过期选择均记录零正文读取。获准路径连接两个本地正文，排除行为保持成立。

#### 📝 摘要

独立 Claude Code Opus `claude-opus-5-5`，T3 配置 `effort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，起点 `4b38d552c6580f8d181c7f934343718ee0fc6b011efe2d680fe3df6e3b92d7d8`，146 路径产品 `32917f46e0bc24bd287ed7c1876f8fe37e2d35ceb539704712f76aaa4766e37f`，Go `55e3dd30b359c1db478cfc30ad68cfa833e628b86e1aceefbb5917b2b61056f9`，六路径补丁 `3bab1e965f638da326f125f7496fdf6d6074b2e2bafb9f406a8decbd3956310f`。两条补丁路线复算吻合。e17 对照无两项新回归。独立 sessionorigin、desktop 全包和 311 个 CLI 相关检查通过，session 全包有 R2-F2。主控核对了新增旧路径 global stamp 的检查位置。

`EffectiveCopies` 的逻辑合同保持可用。实体 Store 的严格可比较 stamp 和逐行身份查询继续走实际组合验收。

## Round 3 — 2026-11-20，旧读取兼容性复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 建议改进

无新增发现。

#### 🟢 已关闭发现

**CJSI-SI-R2-F1 CLOSED。** 实际旧 viewer 和 browser 先读 `AB`，无关会话写入及零来源扫描两种情况下分别沿用原选择读取第二页 `C`，总数仍为 3。持有旧选择的整组读取和实际 CLI `session show --all` 均返回 `ABC`。未修候选的六次持有选择读取返回 `sql.ErrNoRows`，e17 对照和修复候选均通过。同会话改由更高优先级副本胜出时，旧选择的分页和整组读取均拒绝并返回空结果。新选择只读出新副本。旧路径在同一读事务核对来源赢家、读取总数与页面。

**CJSI-SI-R2-F2 CLOSED。** 旧 `Metadata` 恢复跨库逻辑相等。原样保留的 `TestCodexCanonicalProjectAttribution` 在未修候选失败，修复后增量和重建列表的整值比较通过。

R1 的关闭证据继续有效。新索引使用私有 `selectedDetail` 校验完整票据、保护、seal、来源和序列。SQLite WAL 用例确认过期续页被拒，回调后撤销保护时页面及分页均为空。固定票据详情顺序仍为 `AXBC`，等时搜索为 `ABCX`。正文连接实现保持 blob `fe9a7a6a9a6b5ae5c1029ac29c75a4683f948411`，摘要不符或选择过期时均为零次正文查找。

#### 📝 摘要

OpenCode Pro `openai/gpt-6.2-astra` 冷上下文独立复评，T3 请求 `v075-e20-task5-r2-rereview-opencode-4d5fb4cc`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，复评起点 `32917f46e0bc24bd287ed7c1876f8fe37e2d35ceb539704712f76aaa4766e37f`。五路径增量补丁 `02332f291eb709e7f9b7aac3628b5fe25c1ac89cac6ea20a22d3ee28244b2851`，最终 146 路径产品清单 `4d5fb4cc52f1e606e69a533447cb008271717c59809dbf06bd92c71aed589227`，132 路径 Go 清单 `8676e2f5b4221bd3e069966e3341e0bbfef7d274e1040b50606f26a91f093a5d`。独立复评重放增量补丁并比较 3,726 个源码条目。主控核对 13 项交付校验和、正文及对照测试 blob、读者入口和测试结果。

`scripts/run-go-test.sh ./internal/session` 有 66 个顶层测试通过，作用范围内的 CLI 测试有 146 个顶层测试通过。两项命名锚点的普通与 race 运行各有 26 条通过记录，均无跳过。此轮通过旧路径修复及逻辑读契约复评。Task 6 可以使用有效会话副本的同票输入。Task 5 的实体 Store 读写接线、候选 seal 和完整验收继续开放。

## Round 4 — 2026-11-20，实体读者增量

### 📋 候选技术评审

📊 本轮范围评分 9/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟡 需修复的问题

**CJSI-SI-R4-F1**（P2）。在真实合成 SQLite 夹具中，旧 `Search` 执行 `{client text}:(A OR X)` 返回 `AX`。新 `CJSIReader.Search` 对普通 `A OR X` 和 `text:(A OR X)` 同样返回 `AX`，但带旧有列过滤时得到 `no such column: client`。底表 `session_documents` 和候选表 `cjsi_candidate_documents` 具有不同的 MATCH 列名集合。后者在 `internal/store/cjsi_index_schema.go` 定义，缺少 `source_path`、`client`、`session_id` 三个未索引列。两个分支按架构应执行相同原表达式。Task 2 的私有 DDL 修复须与 Task 5 的真实跨侧搜索回归一并验证。发布中的旧入口维持其现有语义。

#### 🟢 已核对的范围

真实 Store 夹具中七个入口均在取数后复核保护撤销，返回 `ErrCJSIIndexChanged` 和零输出。旧票不能续页，FTS 遮蔽与按稳定 ID 排序、详情 ordinal、原 payload 移动及 C1 读取范围保持原合同。`EffectiveCopies` 在同票返回合格副本及其已证实时间，区别已准入空库与从未建库。Task 8 持有生产入口和激活，写者与压实仍由 Task 5 后续验证。

#### 📝 摘要

话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。基底 `669fe60929a4ec48e1eb61aeef1ee6fc02fae96c959b72fe61e5f6282070b417`，产品 `9440f6ad86405668e88aac49151c1f591836864285bdef764c8371253fae0ac7`，Go `2d34071d227fe768a043bb3eae889235c053e91729dff205c8b2102625f4ea80`，五路径补丁 `12a619278a938697c93d414627dde8a2ad206d38124b6909e9283108f4f0c6b7`。主控核对 10 项校验和、零 fuzz 补丁回放和直接 SQL 见证输出。既有锚点普通及 race 各 38 条通过，旧入口验证分别有 197 和 242 条通过。原 Round 3 的结论适用于其旧入口和逻辑读范围。

请求 `v075-e21-cjsi-task5-physical-adapter-review-9440f6ad` 使用新 OpenCode 评审角色，配置 `openai/gpt-6.2-astra` 与 `variant=xhigh`。评审者在开始时报告收到简报外的作者摘要，独立性门槛仍开放。上文 FAIL 有实测负例支持，待修复候选另行接受冷复评。

#### F1 修复候选补证

两路径补丁 `e42f4d6af333c9652bc9f32cb358f8738eee17968c1d47363249845468ae391f` 从上述产品前像精确重放，结果为产品 `52ecbebc13c850abcfd295f172532da57a81405beffcb6f6c4ed5584b7f08679`（154 路径）及 Go `93118060707e75e74834cc039efa37a5ee77a69246ccf5a0ae154b1d733ba457`（140 路径）。Task 2 的候选 FTS 内容视图以 NULL 提供三个未索引列名，保留四列 payload 写入与选中成员的实际元数据。Task 5 的真实底表和候选夹具复跑三种过滤。旧布局均报缺列，修复后正向和排除过滤返回 `AX`，仅限未索引列时返回空集。

作用范围锚点普通 123 条通过，race 119 条通过，两个运行各跳过一个仅供子进程运行的 WAL 夹具。格式、相关 vet 和 macOS CLI 构建通过。首轮串行 Go 检查中 29 包通过，测试进程的 `XDG_CONFIG_HOME` 使两个 CLI 用例误读共享夹具。移除该变量并保留隔离 HOME 后，两条用例及完整 CLI、shellconfig 包通过，沿用其他包已有结果。候选冷复评、Task 2/5 组合门禁及公开接线仍待完成。

## Round 5 — 2026-11-20，guarded reader 冷复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 需修复的问题

无新增发现。

#### 🟢 已关闭发现

**CJSI-SI-R4-F1 CLOSED。** 候选和底表 FTS 使用相同列名、列顺序及未索引声明，两侧运行原 `MATCH @query`，仅 `text` 进入索引。候选内容视图的三个组元数据列为 NULL，结果仍来自有效成员。实际读夹具的两条词分别来自底表和候选，三种列过滤在旧 DDL 报错，修复后依次得到 `AX`、`AX`、空集。外部内容插入和删除保持与 payload 的 doc_id 一致。

七个真实读入口在取数后遇到 protection revision 推进时均返回 `ErrCJSIIndexChanged` 和零结果。找不到索引和已准入空索引有独立结果。切票和序列推进拒绝旧页，搜索稳定 ID、详情 ordinal、有效副本、C1 的已发布范围及可见遮蔽继续符合合同。

#### 📝 摘要

Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task5-codex-r4-52ecbebc`。启动核对候选作者历史与先前裁定均未传入。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `52ecbebc13c850abcfd295f172532da57a81405beffcb6f6c4ed5584b7f08679`，Go `93118060707e75e74834cc039efa37a5ee77a69246ccf5a0ae154b1d733ba457`，补丁 `e42f4d6af333c9652bc9f32cb358f8738eee17968c1d47363249845468ae391f`。复评核对源码、补丁和运行凭证，复用普通 123 条、race 119 条及各自的子进程夹具跳过记录。主控与 Task 6 逻辑切片组合，产品 `331d727521cd64f76c4df37a5d2bd0c676668df9ae993c11587026ec499783f2` 的定向锚点有 83 条通过、race 80 条通过。

PASS 适用于实体读者和列过滤修复。Task 5 的候选写入、封存、压实及整项验收仍开放。公开入口与同票启用归 Task 8。

## Round 6 — 2026-11-20，私有候选写入与封存

### 📋 独立评审

📊 本轮范围评分 10/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 组合验收条件

调用方须借出已获准的私有写事务，并把实时核心 attempt、workset、合法成员和顺序绑定到 seal 和 sequence。写入器内所用认证值由调用方提供。压实、根与事务准入、耐久性、公开核心 CAS 及发布仍待各自阶段验收。

#### 🟢 通过范围

实际 SQLite 夹具成功保留旧 ID 1，并通过原触发器索引新 ID 8。提交私有候选前后公开选择仍返回 `ABC`，重复 attempt 被拒，已封存组与高水位保持原值。后段主键错误和最后 seal 触发器错误均终止整个私有事务，`Commit` 返回 `sql.ErrTxDone`。七张表为空，新词 FTS 命中为零，高水位还原为 7。独立 scratch 夹具另证实空移动只留下目标副本且没有文档，混合移动按 `A`、`stageword` 排序，移除位置和无资格正文不进入结果。

#### 📝 摘要

Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task5-private-stage-review-17f23e4f`。启动仅收到本轮候选简报，未注入作者及以往执行或裁定。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `17f23e4f02e9990becd526a37d8012ac56ab4d6c2e1cef90f6ad2374221efe35`，Go `95d5efdaf3d0ee58b8017b0e455acdd8284462bf74d6933318882b377f890f84`，两路径补丁 `acdea9714d20f2a5ae05d213ce045e41d096ab94509ce92952fa36dd3f25e579`。主控核对 19 项校验和、两个新增 blob 与模式及精确重放。Go 1.27.1 下完整 session 174 条通过，相关 race 63 条通过，Store、CLI 锚点及 vet、构建通过。

## Round 7 — 2026-11-20，私有索引压实

### 📋 独立评审

📊 本轮范围评分 9/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

**CJSI-SI-R7-F1（P2）。** 压实以 `doc_id + ceiling` 暂存保留文档的详情位置。实际 SQLite 夹具通过既有写入器封存并读出了 ID `9223372036854775806`、`9223372036854775807`，详情位置为 1、2，总数与 FTS 命中均为 2。压实时加法溢出，SQLite 3091 拒绝向强类型整数列写入 REAL。事务整体回滚，`Commit` 返回 `sql.ErrTxDone`，重新读取保留原视图。修复应使用与稳定 ID 大小无关的临时详情位置，并保留两文档回归。

#### 🟢 通过的范围

既有夹具证实普通替换、移动、空替换和单组删除的顺序、总数、稳定 FTS rowid、范围证明及覆盖状态。隔离测试覆盖后段 SQLite 错误及其他封存外键引用触发的回滚。生产规模的全索引校验成本、写锁时长和峰值内存仍待综合验收测量。实时根准入、耐久性、同票发布和回收由 Task 8 承担。

#### 📝 摘要

独立冷评审为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task5-compaction-review-79230c84`，authority epoch 21。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `79230c84d3ee2996555d176c210f5ee058683ed0657b9eacbdd55a6ae37bd5c6`，Go `81ba3f33e22d7669b5c4107d8c72a5fa896da2c2454982517dca02d36edfd455`，压实增量补丁 `c7ccd421b79d99ec257415c025371748aee739962ffb5f815079227a9e15320b`。主控核对独立反例及真实 SQLite 错误，并将其纳入该 finding 的修复回归。

## Round 8 — 2026-11-20，压实详情位置溢出复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟢 已关闭发现

**CJSI-SI-R7-F1 CLOSED。** 暂存位置由受影响范围的最大 ordinal 加各行原 ordinal 得到。迁移与候选写入生成连续位置，受组文档数约束。移动时同身份来源撤销使旧组也进入范围。暂存位置高于旧、新两侧且在原组内保持唯一，稳定 doc_id 保持原值。

同一回归在旧实现以 SQLite 3091 失败，修复后普通和 race 均通过。实际封存及压实 ID `9223372036854775806` 和 `9223372036854775807`。提交后重新读取仍为 ordinal 1、2，总数 2、FTS 命中 2，完整读视图一致。文档 ID 高水位保持 `MaxInt64`，选中 session ticket 已转移至底表。既有保留文档移动用例继续通过。

#### 📝 摘要

独立冷复评为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task5-compaction-r7-rereview-e1a851e4`，authority epoch 21。候选作者、执行历史及先前裁定仅来自本轮简报。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `e1a851e4df4a469b99fa2b7ab822a7f2d962138b1aef07d608fbb1bb3580c0a4`、Go `5008ac2416bd6110e4931afaf2852b02bfc03acf095df19cf51cfd17675cc854`。两路径补丁 `bb53a4c2ed18934b388a3ee2795a9608dcbf372530c0b6768c4cf1f32c78de7d`。主控精确重放补丁并核验模式及全部清单。普通 session 194 条通过，定向 race 42 条通过，均零跳过，build、vet、gofmt 通过。通过范围为本轮 finding。生产规模测量、Task 5 整体验收及 Task 8 发布继续在各自范围推进。

## Round 9 — 2026-11-23，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 5，作者 Codex `gpt-6.2-astra` high，最终补丁 SHA-256 `6d034e0bfe3a911a3c06494a6c70f2500eb8419c995293bee1d097bc2c7c7b56`，19 个路径，基于 HEAD `3b71a07`；同轮的 Task 1 跟进单独提交（见 native-source-proof Round 14）。用户批准第 4 轮补测与全量变异扫描；第 5 轮只补两处测试，由主控在用户出行期间决定。

- **重建首评（AGY Gemini 4 Neon high）FAIL。** CJSI-SI-R9-F1（P1）无头旧单身份 Codex 源被拒；F2（P1）迁移来的 `otel:` 行 birth 永空、孤儿来源漏处理；F3（P1）迁移后干净 Claude 检查点被迫重放而半行反被放行；F4（P2）`Source.Birth` 为零时回退 `FirstAt`；F5（P2）显式 `thread_id` 无 `turn_id` 的本地消息被丢弃。
- **R1 复评（Claude Opus）FAIL。** R9-F1..F5 closed（作者指出复现 4 的 priority 硬编码与真实 Discover 不符，复评独立确认）。新发现 CJSI-SI-R10-F1（P2）旧 Codex 文件以 `synthetic`/`otel` 观察被复用；R10-F2（P3）缺 hint 拒绝无测试。
- **R2 复评（Claude Opus）FAIL。** R10-F1/F2 closed。CJSI-SI-R11-F1（P3）codex-jsonl 分支 `!complete` 守卫无测试。
- **R3 复评（AGY Gemini 4 Neon high）FAIL。** R11-F1 closed。CJSI-SI-R12-F1（P3）读后 `ingest.Validate` 复验无测试。用户批准第 4 轮并要求一次扫清同类缺口。
- **R4 修复与复评（AGY Gemini 4 Neon high）FAIL。** 全量变异扫描中发现并在 Task 5 自有路径内修复生产缺陷：CJSI-SI-R13-F1（整批 unchanged 把 `synthetic:replace-documents` 计为缺失文件）、R13-F2（OTel 活动分页绕过窗口校验）、R13-F3（可见消息 `event_at` 接线）、R13-F4（Claude 迁移前缀活动接线）、R13-F5（路径移动且原路径被另一 inode 复用触发唯一约束：不改 DDL，以替换 upsert 遮蔽旧身份，Task 8 约束写入 tasks.md）。复评关闭 R12-F1 与 R13-F1..F5，新发现 CJSI-SI-R14-F1（P3）`O_NONBLOCK` 无测试、R14-F2（P3）`SQL-1004` 被误判为等价。小数秒排序属既存缺陷，不在本 Task：-> open `ad-bug-session-search-fractional-second-order`。
- **R5 复评（Claude Opus medium）PASS。** R14-F1 closed：FIFO 替换的实库测试覆盖读回调与 `ActivityPage`，以有界超时解除阻塞，连续 5 次稳定，RM4 被杀。R14-F2 closed：裸快照拒绝回归杀死 `SQL-1004`，同类 `SQL-1002/1003/1005` 改判成立。相对 R4 只改两个测试文件，生产代码字节不变。本轮无新发现。
- **验证。** 变异清单 1048 项：1001 捕获、42 等价、5 不可达、0 未结案；评审者抽查等价理由与新变异全部成立。两锚点、`internal/session`/`ingest`/`activity` 普通与 race、106 项实库夹具、`internal/usage`/`store` 全包均通过；gofmt、vet 干净；测试仅用隔离目录。
- **门禁。** CEv1 WorkUnit `codex-jsonl-session-identity:session-effective-index` 在该补丁状态上独立评审、L2 验证、范围与隐私 3/3 满足。Task 6 绑定与 Task 8 同代开放、恢复 CAS 由后续 Task 联合验收。

## Round 10 — 2026-11-24，搜索小数秒排序修复评审

评审结论：PASS

完成门禁：VERIFIED

用户 2026-11-24 决定在 v0.7.5 内修复此前延后的四个 P3（重新分诊为 Lane A，只恢复既有契约）。作者 Codex `gpt-6.2-astra` high；独立评审 Claude Sonnet 5.5 medium（档位经 Decisions 评估）。补丁 SHA-256 `19b28f424247067037f8930b3526de62a01a8ad0e020da9ee763e2273cd95962`，9 个路径，基于 `230d688`（与 `bbb06da` 对这些路径无差异）。评审在 `bbb06da` + 补丁的隔离副本运行 `internal/ingest`、`internal/session`、`internal/store` 全包与 `cmd/agentdeck` 的 `TestSessionSearch*`，带 fixture 标签的读者用例通过；gofmt、vet 干净；新回归在旧代码上失败，四个变异均被拦截。

- **既存缺陷关闭（载体 `ad-bug-session-search-fractional-second-order`）。** 旧路径与 CJSI 读者共用的 `visitHits` 改为 `ORDER BY event_at COLLATE agentdeck_event_instant DESC`，按解析后的 RFC3339 瞬时比较到纳秒，同瞬时仍按 client、session、文档 ID 决胜；游标格式与分页不变，两条路径一致，符合 cli-design.md:2025-2027。
- **评审 P3 观察（已处置）。** CJSI-SI-R15-F1：collation 每次比较解析两个时间，大命中集下为 O(n log n) 次解析；`MustRegisterCollationUtf8` 同名重复注册会 panic，目前全仓仅一处注册。当前规模可接受，按评审最小契约保持现状，关闭；性能由 Task 10 综合验收观察。CJSI-SI-R15-F2：空 `event_at` 与无法解析的值现在同列末尾并按稳定决胜排序，此前无效值按字典序混在有效值之间——这是 cli-design.md:2027「无效值保持稳定顺序」所要求的可见变化，记录于此，关闭。CJSI-SI-R15-F3：读者排序的实体用例 `TestCJSIDocumentEventTimeParity` 受 `cjsi_adapter_fixture` 标签与 overlay 门控，默认运行不执行；默认运行由 `TestCJSISearchFractionalSecondOrder` 覆盖共享 SQL。已写入 tasks.md 的 source-issue-acceptance 最终验收命令，关闭。
