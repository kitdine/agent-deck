# JSONL 来源证明评审

## Round 1 — 2026-11-18

### 📋 预冻结纯解析切片

📊 综合评分：9/10

✅ 评审结论：PASS（限定切片）

### 🔴 必须修复

当前切片无阻断项。以下接口事项在接入持久消费者前处理。

### 🟡 后续事项及处置

- **F1 · 回放范围。** 原实现在坏行、`compacted` 或已关闭且未归属的段上保留回放起点。在已验证来源内，`ReplayFrom` 现指向仍可通过追加获得证据的开放段，包含待配对的已证工具开始。无法从后缀补证的关闭段不拖住回放。尚未验证的来源另按来源状态处理。归约仍要求已验证的完整前缀，持久消费者须另处理迟到反证对既往记录的撤回。
- **F2 · 请求后消息边界。** 按 A2，在混合来源中，请求后的无 `thread_id` 消息继续等待本地右锚。即使裸 `turn_id` 仅有一个已证 context，也遵守该条件。显式本地请求及其独立可证的模型继续保留。测试包含唯一与复用 turn，补上本地 `task_complete` 后才归属回复。
- **F3 · 事件 ID 最小化。** turn、根 turn、call 和 response ID 拒绝含斜线的路径形值。`provider/model` 形模型和原有旧会话兼容沿原校验。测试核对路径哨兵不会进入序列化 proof。

### 🟢 已核对

独立复评确认来源冲突、直接父边、继承与显式请求、工具配对、计量存在性以及正文与证明的分离。候选里的四个 ingest 文件匹配约定。Claude Sonnet 5.5 运行完成。T3 记录请求 `effort=medium`，实际推理档位未独立观察。

### 📝 小结

复评 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，候选 `50ad394a721fd785289c91fbf0b9305052b3b55520a86ef60ab880531d18f69b`。独立 reviewer 的 ingest 测试通过。主控在同一范围内为 F1–F3 添加合成断言，在原实现复现失败，修订后 `scripts/run-go-test.sh ./internal/ingest -count=1` 和四个新增文件的空白检查通过。修订候选 `1553ed3ecc256dc7d4451805531a7fd5e600420ec143429f3fa3689731a49c02`，待随接线验收纳入完整锚点的独立评审。

完成门禁：NOT_VERIFIED

状态仍为开发中。C1 的来源代际、单行上限、持久断点恢复和活动读取由后续接线验证。现有纯切片约束与测试作为该范围的证据。

### 有界捕获交接 — 2026-11-18

候选 `a51a0ccb1a3e2f317b77e3d800e3399901d8a41dfc4c367fdc6ec3391724e745` 包含五个 ingest 路径，HEAD 为 `a224b0b824d2cad451d3bd83153832d65506f0fe`。主控核对四个交付 blob、保持原样的合成 fixture、实际测试记录及空白。捕获与纯来源证明两个命名用例和 ingest 全包测试通过。仅复用当前同一候选证据。

`ReadCodexJSONL` 使用 caller 的捕获终点和单行上限。超长完整行只给范围，形成不能跨越的解析边界。未终止行及未接受的回调不推进 `CompleteEnd`，`ReadEnd` 反映消费进度。保留普通逐行读取语义。底层读取中的取消需要调用方提供边界，读取块之间检查 context。

下一范围为同一描述符的来源与捕获前缀校验。把有界输入对应的身份、已证范围与摘要作为私有结果，先由后续发布方复验再接入。持久布局、旧库换代、完整有界工作集、activity 及实际发布仍在后续。锚点继续开发。

### 前缀捕获与复验交接 — 2026-11-18

候选 `2f2b11d3863fadd3b7bbd53117efd758e2a54c5845bf9089c4a770d74babc27c`，HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`。主控核对了七个文件、两个新增 blob 和实际回执。ingest 全包测试执行了来源证明、有界捕获、前缀代际三个用例并通过，复用当前候选证据。

捕获绑定同一描述符、源路径身份和实际读取字节摘要。末尾半行只参与所读范围摘要，最后完整行位置仍是重放边界。复验允许已捕获范围外的稳定追加或改写，并拒绝名称替换、截断或前缀变化。最终复验由发布调用方在其事务边界前执行，已有证明按已验证范围使用。

下一个开发范围为 activity 的已证事实消费。仅从已核准的来源和范围归约得到的本地工具开始建立明细，完成按证据中的开始位置配对。保留已有明细、汇总、分页及时间语义。ingest 的依赖图当前可供 activity 单向消费。实际选中票据、持久工作集与发布接线继续按 C1 和所属锚点处理。

### 已证 activity 交接 — 2026-11-18

主控匹配 10 文件的原切片 `f867f62866ca9564fd22bd39a567ae1b034995f1d05b769b10883e2b9d9e5337`。明细按本地工具开始的已证范围匹配完成，不跨段复用裸 call ID。无有效完成时保留 started，倒序时间的时长为空。

主控用合成输入复现了分页保留整批明细、丢失合格 `provider/model` 两项问题。现在保留页窗口及当前段待配对开始项，汇总仍覆盖整个已证范围。排序相同的明细按开始的位置排序。合格的命名空间模型保留，路径形值在解析和呈现时被清除。原通用 activity 读取路径继续沿用。

修订候选 `b5d5de8c54f2c05baa0e94b5623240dc006fc7a39bf59cd624a7b88abec145df`，相同 HEAD。两项反例在修订前失败，修订后 ingest 和 activity 全包及空白通过。该候选送一次独立的完整来源证明交接评审。实际选中来源、持久工作集与发布接线仍有独立验收。

## Round 2 — 2026-11-18

评审结论：FAIL

独立 reviewer Claude Sonnet 5.5 重算了相同 HEAD 和十文件候选，重跑 ingest、activity 和两包 `go vet`，均通过。T3 记录的请求档位为 medium，实际推理档位未独立观察。复评建议在合并交接范围内 PASS，包含一个接线前应解决的接口问题。

- **CJSI-NSP-R2-F1 · 同次捕获绑定。** 主控核对 `CodexDetails` 和 `CodexDetailsPage` 共用的输入校验。两个同线程、等长、同完整行边界的捕获可互换归约，现有数值比较无法区分。继续在任务 1 内收紧捕获到归约的类型边界，并验证同范围错配和无证明输入均被拒绝。
- 复评覆盖已有的来源证明、完整行限制、前缀校验和 activity 明细分页。真实来源选择、持久工作集和最终发布调用仍需对应任务的证据。

完成门禁：NOT_VERIFIED

任务仍处于开发中。正式任务复评和完成门禁在剩余任务 1 要求满足后处理。

## Round 3 — 2026-11-18

复评结论：PASS

**CJSI-NSP-R2-F1 CLOSED。** `CaptureCodexReduction` 从同一次成功捕获取得事实、归约与捕获证明。activity 的详情和分页先核对捕获身份，拒绝相同范围的另一份来源、同描述符的另一次捕获及未绑定输入。可迭代的证明字段按值复制。

作者在旧实现复现了两个入口的错配。主控重算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 的十文件候选 `2a4481e48532be9e73ee9b7daebdd2d76553f73fa992d000c35893f0127cd6df`，核对 `scripts/run-go-test.sh ./internal/ingest ./internal/activity -count=1` 的最终日志，新绑定、归约、详情和分页用例均通过。独立 reviewer Claude Sonnet 5.5 匹配同一候选，复评本项为 PASS。T3 记录的请求档位为 medium，实际推理档位未独立观察。复评的 Go wrapper 报告通过，管道未保留其退出码，测试证据使用作者的直接 wrapper 结果。

完成门禁：NOT_VERIFIED

本轮收口同次捕获绑定。锚点继续验证生产有界工作集、选中来源接入及其发布边界，未进入完整任务评审。

## Round 4 — 2026-11-18

评审结论：PASS

独立评审建议有界重放两文件交接 PASS。Claude Sonnet 5.5 的 T3 配置为 medium。评审匹配 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和候选 `00beccf880bb01da659b1d815e2fad3cec87c36a1969d65991e3b63a0b2de6de`。静态核对了从零验证连续完整行、行摘要和解析策略，以及字节预算、长行、半行、取消、跨步文件变化与暂存撤销。主控核实原十二路径合并指纹和 Go／race 日志中实际执行的用例。

主控在接口中注明，同一重放要求文件跨步稳定，即使追加位于固定捕获终点以外也会中止本次尝试。调用方成功 `Begin` 后登记 `defer Abort`，在未完成或回调 panic 时清理暂存。丢弃当前半行缓冲时先清零。新增跨步追加、非法预算和放弃后缓冲清零断言。源读错误沿既有 `ErrSourceChanged` 处理。后续持久工作集以重新验证、预算及轮次接续解决活跃追加的进度问题。

修订后的两文件指纹 `fecb90b239580a04cfc6ee0ab26a6e392a7022401c91ebb5dd612b3eba81e52a`，十二路径合并指纹 `e30b13d590d8c5fbaa8e6510be085598b62b84bcb879f85c6e876ffac8df55f4`。`scripts/run-go-test.sh ./internal/ingest -count=1`、定向 `-race`、`go vet -mod=vendor ./internal/ingest` 和范围内格式及空白检查通过。此前独立评审绑定原候选。上述小幅修订由主控核对实际测试。

完成门禁：NOT_VERIFIED

Task 1 继续开发。持久工作集、归约续作、选中来源对接与正式发布仍待实现和验收。

## Round 5 — 2026-11-18

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限五个已验证重放归约文件。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，22 文件候选 `b50ef0e474ab65d4401e7315d705a14c44f7d774cb7fbc781eccb5c10b316750`，切片 `14c4af2ed5d6debc2e6e0870e7a0ad28389901f3defb8369ad9636ebe2100955`，冻结 17 文件 `9d61e5e49214ba7636f1a53b27cf1087147df8cf3219e5d50acd388513b0aa7d`。T3 配置为 high。独立测试覆盖冷读与重放等价、完整行与半行、追加、摘要不匹配、范围与记录上限、取消、单次撤销，以及同一次捕获的证明。ingest 和 activity 全包、定向 race、vet 和格式检查通过。

**CJSI-REPLAY-R1-F1 CLOSED。** 新增 `TestCJSIVerifiedReplayReductionRejectsUnverifiedTerminalLine`。旧证明覆盖两条完整行，第二行的换行在新尝试前被改为空格。正常终态返回 `ErrSourceChanged`，撤销第一行暂存并拒绝归约。仅在隔离 overlay 移除终态守卫时，测试观察到 `done=true captured=true aborts=0 rows=1 err=<nil>` 并退出 1。含新用例的定向 race 退出 0。

测试增量后的 22 文件候选为 `2abac7d01a0358b495f1dfc4e6337383b6ce6c04896654a9cdb56267f4991cfc`，五文件指纹 `117338b2166f5bce91821b607be0f75d98c8ca0420d32ae69a1c0671c26c4417`，测试 blob `a62d7aebfea4766780a21cbab5e77ce6093e8aab`。代回旧测试 blob 重算获得评审原候选。调用方仍须完成实际来源选择、持久协调、大工作集归约、发布前源复验和同代发布。

## Round 6 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 通过四文件有界归约切片 `668222c456b05378431649018aecce57c67ade85ddeea617e90cacf29109eb50`。主控复算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 上的 ingest/activity 16 路径 `08ffa9e2ef758a423287445ca67e3ef4b0c6902bdd8e8f0245f0f39563c36b4c`。评审以 4,000 个合成种子对拍冷读和多遍重放。另用 124 个篡改与错误子用例验证同一已验证工作集摘要、范围连续性、拒绝不完整证据及输出撤销。项目自带 3,601 条记录的测试验证段和 turn 上限，主控核对 ingest/activity 包与定向 race 日志。峰值内存和生产磁盘成本仍待实际适配验收。

调用方承接暂存生命周期。归约失败必须撤销相应 attempt，并在中止或换遍时释放 reader。超过段预算的来源走明确的未完成状态。下一段接续选中来源的证明绑定、按需活动读取与发布前复验。生产发布和同代 CAS 仍由对应后续阶段接线。

## Round 7 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审选中来源及按需 activity，配置档位 high。评审与主控核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 下的七路径 `10213c4e25c47b242f7b695d1cf0012dbdab7379e74625573c4aed4556978b66`、ingest/activity 20 路径 `aaba8481a451992637d999f4f68630b3d2cfc8ec06d752c98c557006de2853e8`。两个包回归和定向 race 通过。评审合成探针确认路径、身份、范围、字节复验和选择复核，旧范围外追加保留旧半行状态。

- **CJSI-NATIVE-SELECTED-R1-F1（中）** `VisitSelectedCodexReduction` 将选择证据声明中的 `Version` 原样写入一次性捕获的 `checkpointVersion`。因此任意版本可让 `ReplayPrefix()` 产生本应只来自实际 checkpoint replay 的证据。独立探针用 generation 9 和 parser 777 验证可返回该版本，主控核对生产赋值和 `ReplayPrefix` 的返回条件。修复须保持选择版本经权威选择核验，且一次性读取无法生成 checkpoint replay 证明。待修复。
- **CJSI-NATIVE-SELECTED-R1-F2（低）** 硬链接同一 inode 的别名路径和访客执行期间同尺寸原地改写目前由生产检查拒绝。隔离变异显示对应检查缺少直接回归断言。补相应拒绝用例。待修复。

独立评审建议将 F1 留到持久接线前处理。主控将其纳入本切片修复，因为公开 `ReplayPrefix` 的证明承诺已经生效。CLI/session 消费、持久 provider、重试和发布 CAS 留在后续范围。

## Round 8 — 2026-11-19

复评结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 复评绑定 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、七路径 `948b2e15d2ab2741647b76edf1c7182b6d851ca72f9be5c333126f376f4706b8` 和 20 路径 `216fbd841df866789887728413f340943bb8f3b45e38f295446165db4188584f`。两个包全量和定向 race 通过。主控核对修复前假 replay prefix 的失败和修复后的实际执行。

**CJSI-NATIVE-SELECTED-R1-F2 CLOSED。** 同 inode 别名、访客期间同尺寸改写、末次核验中的改写和描述符关闭都有可击中对应变异的断言。

**CJSI-NATIVE-SELECTED-R1-F1 保持开放（低，测试缺口）。** 一次性读取已不再产生 replay 声明，且前后核验选择版本。真实 replay/workset 比较也是正确的，复评探针覆盖 parser 不符、generation 不符及缺失版本。但现有 `wrongPolicy` 测试传入的选择器仍返回原选择，版本校验被误删也会先由选择器拒绝。令选择器明确同意待拒绝的版本，并固定三种用例，证明拒绝来自真实证明。修复范围限测试。

## Round 9 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 复评限 Round 8 的版本比较断言，配置档位 high。主控与评审核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、七路径 `cdee7ba7b1bdf094c2d6e1f820bc1473fa14064a674f6c1a87a154512255e623` 和 20 路径 `938889899903902968a335c6a39091092ae39465b33fa5512dba2e3b19d797e9`。本轮只修改 `internal/ingest/codex_selected_test.go`，blob `76e8d76dcdd1b9ae65a0927bee435062049548ef`。

**CJSI-NATIVE-SELECTED-R1-F1 CLOSED。** 选择器同意待拒绝候选时，真实 workset 证明分别拒绝 parser 不符、generation 不符和未提供版本。匹配版本及范围外追加的正例通过。主控核对聚焦与 race 日志，忽略全部版本比较的变异使三项测试均失败。独立评审另验证每项断言能单独击中对应变异。沿用前轮已验证的一次性读取行为及 F2 关闭证据。

PASS 限选中来源证明与按需 activity 切片。Task 1 保持开发状态，下一步让持久工作集适配器消费此证明，并验证取消、恢复和 reader 退役。持久选择、生产入口及同代发布由 C1/C3 接线承接。

## Round 10 — 2026-11-19，生产者终止错误回归

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限候选 `ef3d1466ad553dd39d9953250b26436c6c1e5bebf1dd42efe479898cbfcbec09` 的单个测试文件。工作树 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。主控核对补丁 SHA-256 `ce7a2978e7a05f0d2491e5a49fb1b9b619a8a94dd9da9a46f2feb7e704829a28`、前像 `e3fb6e861ccaf7bd6e2bc072f90f864978f5b94e` 和候选后像 `25f5b33480c668e666e0f34b8babba5fdd47532d`。T3 配置 `effort=high`，实际推理强度未独立观察。

夹具在三条记录和两个段暂存完成后的末次读取注入退役错误。归约传播准确的错误并返回空结果，调用一次 Abort，暂存归零。把该读错误当作 EOF 的窄变异按预期失败，保留三条记录与两个段。独立复核校验变异条件及失败原因。ingest、activity、定向 race 和 vet 的有效回执通过。SQLite 适配器的取消重试、重启恢复和 reader 退役证据单独核对。

Task 1 保持开发状态。下一段 C1 验证绑定真实权威 selection、暂存归约复验失败时丢弃，以及 exact attempt 退役。正式同代 CAS 发布保留既有所有权与依赖。

## Round 11 — 2026-11-19，C1 隔离调用顺序

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 复评限 C1 隔离生产者与 SQLite 调用顺序。工作树 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，20 路径生产者 `ef3d1466ad553dd39d9953250b26436c6c1e5bebf1dd42efe479898cbfcbec09`，7 路径适配器 `0c367f25866d6165f36d61a2769c6d827609f7ddf7b39fbe57d63fb6b2a8f160`，联合复现包 SHA-256 `f12ed2e388cd65c17365b1d046003fe691a87f924e0deaa5dba0ec663c3571ab`。主控核对保存工件和真实五场景回执。T3 配置 `effort=high`，实际推理强度未独立观察。生产者测试采用 Round 10 已评审候选。

真实归约在复验前暂存 12 条记录与 4 个段。独立合成 SQLite 权威的匹配选中完整接收结果；暂存后 epoch 变化、producer 识别假摘要、权威行丢失和调用方报告 reader 活跃分别丢弃输出。适配器按 exact attempt 退役，旧游标和重新打开被拒绝，同轮 sibling 仍读取 12 行。活跃 reader 的判定来自调用方；在调用 Retire 前先排空，过早退役会使旧游标在下次读取时失败。

原始三项变异击中复验缺失、失败不丢弃和过早退役。独立复评补充的接受先行、清理失败不丢弃、权威回显、扩大退役范围及跳过退役五项变异均击中对应断言。joint、race 与 vet 回执适用。C1 的 producer/SQLite 接口证据可以复用；生产 selection 签发、claim 与 sealed attempt 的绑定、reader 的生产所有权及最终 C3 CAS 仍按批准的下游范围处理。

## Round 12 — 2026-11-20，C1 双消费见证与接口冻结

### 📋 双消费见证独立评审

📊 本轮范围评分 9/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 冻结条件

**CJSI-C1-DUAL-F1 CLOSED（冻结条件）。** Task 5 消费方必须在接收正文前将独立材料捕获的证据与封存的工作集前缀证据比对。比对覆盖文件身份、完整字节摘要、包括部分尾部的读取进度和行策略。重放通道单独证明 `CodexCheckpointVersion`，与材料捕获比对字节证据时仅将其 `Version` 归零。通过当前 selection 复验以及材料证据比对后，按同一 span、材料类型、当前线程与本地 resolved record 连接，且材料须在同一已证明本地 turn 的 segment 内。只接收用户提示和已证明的最终答复。Task 5 需保留同长度替换正文和非本地记录的拒绝用例，受影响暂存整体丢弃。派发以本段为实施约束。

#### 🟢 通过的范围

同一真实 SQLite 工作集归约产生一条子端请求和两个独立累计区间，同时供会话侧接收本地提示词和有右锚的最终答复。独立 SQLite 权威在重放前建立，复验时重新读取。epoch 过期、选择摘要错误、同长度材料替换均使两侧接收结果为零，调用一次 Abort，并在读者结束后退役同一个 attempt。持久工作集行及序列化证明均通过正文哨兵排除。

定向与 race 各四个场景通过，五项变异分别验证了选择复验、材料证据比对、区间、最终答复和正文持久化断言的敏感性。独立探针确认继承的父端点和提示词实际出现在生产者输出中。未过滤时为三个区间和三篇文档，按证据接受后各剩两个。可见的局部 commentary 缺少获准材料类型，工具 payload 亦未形成文档。两个消费方是本轮测试适配器。

#### 📝 摘要

独立 Claude Opus `claude-opus-5-5`，T3 配置 `effort=max`。话题 HEAD 为 `5a1cd9159b564469b716a3338b5573adece90978`，基线 `929e95233a2658d3ed956c58cd88b01c16d7190c`。双消费见证 blob `497f03cb3b1a642da0aa31562b2c86de73043c98`，回放包 SHA-256 `6d632c20d06decd731df7825794d4f39fcc42ed383995eac16781ffa4184935f`，实际执行的组合为 138 路径 `41a5b0a04868d25b8088a75d5b4e7e157a6300813cdf10310aa4c71daa70d89d`、Go 124 路径 `2051f2ad24e574aa62bc3e6142af7a2fd026eff28dc9e488961502f42678d327`。主控核对前后指纹、13 份校验文件及实际定向和 race 回执。

### C1 接口冻结

**C1 已冻结。** 冻结绑定 139 路径组合 `77e46aa5f8fc50ad618104d7c21a948c771ed4f2bfb41f1f617e4b19614e6e81` 和 Go 验证清单 `bbc5882191a6f91e041b91caf3331d32e572bf8f783de9683144d71d3125fb45`。来源见证运行在上节记录的 138 路径组合。后续差异限受管迁移安装既有工作集 DDL 和新增迁移测试。见证用相同 DDL 创建测试工作集，未执行该迁移入口。生产者 20 路径、适配器 7 路径和定位入口保持相同，来源证据适用于组合。实际初始化和回滚由 [布局 Round 17](fenced-state-layout.md#round-17--2026-11-20c1-工作集迁移) 的迁移见证覆盖。

- 原生 Codex 明文 JSONL 的用量 parser 从 8 分配到 9，会话从 6 分配到 7。启用以来源类别为范围，保留其他来源和公开 wire/archive 现有版本。core 顺序为 quota 36、CJSI 37，私有索引布局为 v1。发布组合仍需核对实际迁移写者和顺序。
- 工作集边界绑定 round、attempt、文件及来源身份、generation、parser、行上限和捕获范围。`ReduceWorkset` 产生暂存记录及段。调用方从受保护的 selection 取得 `CodexCurrentSelection`，运行 `VerifySelectedSource`，同时履行本轮材料校验义务，然后整体接受或撤销。调用方先等待读者完成，再按 exact attempt 退役。已证完整范围可保留，未确定尾部按 checkpoint 重放，失败和 partial 维持已有安全事实及覆盖说明。
- 布局读取在同一 core 快照取 schema 版本、布局 journal 和选中证据。36 及以下无 journal 可返回旧名。37 的 Ready 状态返回 `absent` 或经根、围栏、epoch、身份和 seal 校验的私有路径。pending、不一致和超前状态拒绝开放，37 路径不回退旧名。根排空的拒绝适用于已选中私有索引的准入。合法从未建库状态可在该准入前返回 `absent`，调用者按自身操作验证可用性。实际打开句柄时复核同一选择及身份。
- core 37 的工作集和布局日志同事务安装。工作集只作私有候选暂存。Task 2 保留 C2 所需域表与触发器迁移所有权。任务 8 保留真实 selection、读者所有权、公共入口启用和同代 CAS。任务 9 负责备份净化，任务 10 负责真实旧构建和综合交付验收。

C1 冻结开放 Task 3 和 Task 5 两路实施。Task 4 在 Task 3 的发布资格接口冻结后接入。这个检查点按组合和见证绑定，Task 1、Task 2 的完整任务门禁及签名交付各按实际候选处理。

## Round 13 — 2026-11-23，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 1，作者 Codex `gpt-6.2-astra` high，补丁 SHA-256 `a129113a0d4491bcddea6ec45959932ad598f65efd4e1d48b953e5097ab46180`，22 个路径（21 个 Go 文件与 1 个合成 JSONL 夹具），基于 HEAD `5a1cd91`。worktree 原有的 `internal/activity/activity.go` 改动经作者核对后整体保留。

- **独立评审。** AGY Gemini 4 Neon high 冷评审，与作者不同 harness。逐路径核对 22 个文件的 SHA-256 与 blob，与补丁逐字节一致。逐条核对 Round 1–12 的发现，均未复发。在隔离副本中做 9 项单点变异，全部被测试断言击杀。
- **验证。** 入口 `TestCJSINativeSourceProof` 非空执行（ingest 1 个顶层与 30 个子测试，activity 1 个顶层）。`internal/ingest` 50+95、`internal/activity` 20+45 在普通与 race 下全部通过，0 跳过；vet 与 gofmt 干净。隐私扫描 0 命中，正文仅在 `includeVisible` 下瞬态传递。
- **非阻塞 P3。** CJSI-NSP-R13-F1：`codex_workset_reduction.go:133-137` 未验证源分支未深拷贝记录、未包装 sink 错误，与已验证分支不一致 -> open `ad-bug-cjsi-workset-unverified-branch-copy`。CJSI-NSP-R13-F2：`codex_lex.go:123-127` 的导出 `ReadCodexJSONL` 在非空范围未拒绝 `visit == nil` -> open `ad-bug-cjsi-read-codex-jsonl-nil-visit`。两项按用户 2026-11-23 的延后决定以 C 通道载体跟踪。
- **门禁。** CEv1 WorkUnit `codex-jsonl-session-identity:native-source-proof` 在该补丁状态上独立评审、L2 验证、范围与隐私 3/3 满足。双消费见证使用内存工作集，生产 SQLite 选择与跨库发布由后续 Task 集成验证。
