# 会话索引布局围栏评审

## Round 1 — 2026-11-18

评审结论：PASS

Claude Sonnet 5.5 的独立评审建议两文件交接 PASS。评审匹配 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和候选 `f39b3a861ad582d31b45fee110f5f392c184ffe09fde98fe0377dc411810a0c7`。T3 运行配置为 medium。评审重跑定向测试，核对了选择状态、固定 locator、围栏内容、文件身份与只读事务内的 epoch、版本和 seal。

- **CJSI-LAYOUT-R1-F1 CLOSED。** SQLite `BUSY` 和 `LOCKED` 通过既有 `ErrStateBusy` 报告。隔离库持排他事务时，resolver 返回繁忙且不给出 locator。无法证明的选择继续拒绝。
- **CJSI-LAYOUT-R1-F2 CLOSED。** 在读取 seal 的事务内分别替换私有索引和根目录。根目录用例保留两个子文件的 inode，单独验证根身份。禁用对应的最终身份比较时，两项用例实际失败。恢复比较后通过。
- **CJSI-LAYOUT-R1-F3 CLOSED。** 从未建库及已清除状态即使存在最终名的私有索引，也不读 seal 或返回 locator。原暂存与缺围栏场景仍覆盖。

主控完成上述小修并核对两文件新指纹 `029622a458bcd689b61a3d28b4ad45d7da5ad8142b3b3daff00b9760c71d3a16`。`scripts/run-go-test.sh ./internal/store -count=1`、该夹具的定向 `-race`、`go vet -mod=vendor ./internal/store` 与文件格式检查通过。schema 35 夹具验证两条旧打开入口均收到 `SQLITE_NOTADB`，随后围栏内容及私有选择仍可验证。

完成门禁：NOT_VERIFIED

Task 2 仍在开发。生产打开入口须绑定实际选中状态与打开描述符，迁移须提供正式版本、seal 读取及中断恢复。真实旧版 CLI 和迁移顺序的验收依原 L3 计划执行。

## Round 2 — 2026-11-18

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 静态评审限三个私有工作集文件，候选 `647c34ed254610f6aa5a3b7a616f91266abf08dcdd089c8009e17ac4c90e423e`，HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`。

- **CJSI-WORKSET-R1-F1。** ingest 接受 `a/b` 作为旧 ID，持久编码器拒绝同值并中止整个尝试。修复共用 ingest 的验证规则。
- **CJSI-WORKSET-R1-F2。** 持久 identity 接受非规范 `dev:ino` 拼写。修复以解析后的规范字符串回比。

主控核对修订后十七路径候选 `f47ea717e88c85d90422ce1a3dec9285bacf14491b5bb249147fe705d0a253e6`。草稿枚举、持久类型读回与清理按后续工作处理。

## Round 3 — 2026-11-18

复评结论：PASS

完成门禁：NOT_VERIFIED

PASS 范围为六个私有工作集文件。独立 Claude Sonnet 5.5 复核六文件指纹 `e8a4cdbfc28f9afddce4e1c14b0bd1df723593e9786123f817504db5c3b3cb6c`，配置档位 medium。主控重算 20 路径候选 `16507f93358a3d72687253e7d4ac00a180d3acf861c256c58e9926a756b57136`，核对实际类型化读回、损坏行、草稿恢复及退出状态。store 和 ingest 全包、目标 race 通过。独立评审另用隔离 overlay 检验最大白名单元数据、区间连续性、seal、规范编解码和草稿并发。

**CJSI-WORKSET-R1-F1 CLOSED。** 旧 ID `a/b` 通过完整读回与重放。**CJSI-WORKSET-R1-F2 CLOSED。** 封存身份按规范 `dev:ino` 比对。

生产协调器须找到会挡住同轮新尝试的其他绑定草稿，并按实际所有权和水位清理。Seal 失败后调用方清理暂存。归约须复验源字节并使用重新产生的记录。生产迁移、所有权接线、保留策略和发布 CAS 仍属 Task 2。

## Round 4 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限未注册的围栏安装与恢复核心。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，27 路径 `d3d52b1f41bb911cd00a2f25a93cb3ce7b47de49d8542b0e97554b5a61f6b368`，store 11 路径 `c0985764a4ae77ea717a1ae5fdff86e8b9c20369ba0175b07cf0f10a5a3c7f77`，四文件切片 `f80f05382a181240d07b61420dfc256d5a0e87436159f868f602eb97f5ee85ed`。评审和主控均复算。隔离 `TestCJSIFencedStateLayout` 及 recovery 定向测试、race、vet 和格式检查通过。

评审核对 online backup 中仅有已提交内容、可检测的活跃事务拒绝、先耐久围栏再验证旧入口拒开、随后清理 sidecar，以及 seal 与 journal 的中断恢复。未建索引场景保留无私有库的结果。当前 opener 仅作为隔离代理，真实旧版二进制覆盖由 Task 10 验证。

生产接线承接三条义务。LayoutReady 后的临时 legacy 快照须纳入退役、purge 和备份边界。支持的 Restore 须从目标已验证文件重绑 device/inode 和 epoch。正式维护入口须在围栏替换前停写并排空旧连接，隔离测试中的活跃事务探测不能替代该屏障。

下一段实现 core SQLite 的持久 journal 与初始 seal 读写适配。编号迁移须把 schema-ahead 和安装意图一同提交。生产路由、快照退役、Restore 重绑、旧二进制验收和同票发布均留在相应后续范围。

## Round 5 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 评审限持久 journal 和初始 seal 四文件切片，配置档位 high。评审与主控核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、四路径 `a008427829ce60a251f47fd090e13f4774a04ff827921d6e035229fa11024907` 和 store 15 路径 `76ee3a9013e8963c3178b461e6571afe2dbd620d2a0df1ed1371555792da5006`。定向 journal、seal 和 reopen adapter 测试通过，主控复核实际执行；评审另验证 race、vet 和格式。

隔离 SQLite 证据确认 schema-ahead 与初始 journal 可同事务提交或回滚、默认生产 DSN 实际为 FULL、全字段 CAS 在 24 个并发竞争者中只接受一个，以及无 checkpoint 的 WAL 在已提交子进程被终止后可重新打开并继续。畸形 phase 溢出和并发 CAS 已由独立探针验证，生产接线补入持久回归用例。初始 seal 随私有索引的 epoch 生效，重建与 Restore 需要协调重绑。

进程终止覆盖重启恢复。macOS 上的 FULL SQLite 提交默认采用普通 fsync，突然断电未验证。生产接线承接文件与目录 durability、先排空受管旧连接、退役 legacy 快照、schema 与 opener 路由，以及真实旧版二进制独立验收。Task 2 保持开发状态。

## Round 6 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 评审限生产准备适配器，配置档位 high。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，四文件 `b3def6d8a5151e2a5391d24456daf80ad2f29cde12ac7251a5d93e0ce16e3aa6`，store 17 路径 `0812860bec1fb9185278c24f2c483967d36473fcb54c61d3d8144a045a5e8f7d`。layout 选择集执行 82 项，81 项通过，WAL 子进程夹具按设计跳过。race、store 全包和 vet 通过。

schema 35 输入会被拒绝，schema 36 上的 37 初始化可随事务回滚，提交后才可续作。新 locator `session-index-v1.sqlite3`、兼容索引 epoch 保留、重建换代、初始 seal 与首次就绪都通过。存在私有读池租约时停在 PrivatePrepared 并返回繁忙。旧快照须在重新验证 Ready 后才按 inode、大小和 digest 退役，sidecar 或替换会阻止删除。从未建库的路径保持安全。新增越界 phase 与并发 CAS 测试可捕获 Round 5 所述回归。断电仍未实测。

当前 37 门禁与本版迁移目标一致。后续新增 core 版本时，须在升级前完成该布局，或为明确支持版本定义续作范围。注册阶段还须把物理根目录传入同事务迁移回调，并以实际 schema 36 迁移验证 35→36→37。受管停写排空、公开入口选择、Restore/purge、旧版二进制和同代发布继续由 Task 2 及后续验收承接。

## Round 7 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限受管 schema 37 注册。候选绑定 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、三路径 `f5c30d04ee05054aa9b3bfa5c14ba227b8c9df8fac7c05325cef0f6451f57277` 和 store 17 路径基线 `0812860bec1fb9185278c24f2c483967d36473fcb54c61d3d8144a045a5e8f7d`。实际 quota 36、物理根、同事务意图、旧上限、忙与取消、进程中断恢复、冷启动无私有索引、拒绝 38 均有直接证据。聚焦与 race、vet 通过。

- **CJSI-LAYOUT-R7-F1（低）** 受管冷启动打开 core 后未初始化 WAL，独立探针读到 `delete`。普通打开创建的 core 为 `wal`。修复须验证实际模式并以冷启动测试固定。待复评。
- **CJSI-LAYOUT-R7-F2（低）** 释放排空锁的用例验证了 Ready 和锁，删去受管 core 的 `Close` 后仍通过。须在 release 回调检查最后的受管连接已关闭，并以变异验证断言能拦住泄漏。待复评。

真实 quota 36 组合树的 store 全包仍失败。原 schema 35 夹具要求普通打开创建 35，组合树合法返回 36，断言在旧入口越过围栏之前中止。原 schema-35 源码的同一用例通过。保留这个区分及旧二进制验收义务。生产迁移仍依赖受管连接排空，公开入口、Restore/purge 和 serving 发布仍待接线。

## Round 8 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 定向复评通过，候选限隔离的三路径 `dd53736128800a00c8ff29fad8c81ad351110a9ac532d6485e4a1c78ba3a24c2`，HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`。主控聚焦、race 与两项变异测试结果已核对。此候选保留已评审的迁移注册 blob `eb476e7574c31ffa248a6b586bb2fd7bdf72af7b`。

- **CJSI-LAYOUT-R7-F1 CLOSED。** 受管 core 在排除期内初始化并读回 WAL 模式。独立复评以新只读连接验证冷启动模式，禁用初始化时该用例读回 `delete` 并失败。
- **CJSI-LAYOUT-R7-F2 CLOSED。** 排空锁释放回调在打开自己的验证连接前检查 WAL 和 SHM 已随受管句柄关闭。移除 `db.Close` 会同时触发两个 sidecar 断言。

本轮结论限两项发现。继续保留原 35 夹具的组合树失败及旧二进制义务。受管排空和公开入口仍待集成验收。

## Round 9 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限 SQLite 工作集接线的三个文件。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，提案 store 18 路径 `6b78c3a55964ed34899f3b860e36bc099e85b4a77586d18f2e93f3a5898881fd`，含 ingest/activity 的组合 38 路径 `d8d1123c3b70b4e949c5f2d28a18363b0d6ba502199d02d445751a02a6dec94b`。独立评审覆盖真实 SQLite 的封存绑定、多遍游标、重放、损坏与取消、无局部晋升及精确退休。store 全包通过，store 和 ingest 的 CJSI race 通过。对七项绑定字段、游标复用、连接泄漏和退休条件的变异均触发失败。

允许按精确输入采纳三个文件。Retire 成功表示精确绑定的记录已退役，调用方单独保有封存凭证。Seal 返回错误且实际提交时须按绑定清理。全仓先前的 quota 夹具超时、生产迁移及来源发布仍属于各自验证边界。

## Round 10 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限诊断准入三路径提案 `fa62cbdfa5cf7b0c7264e2c8e0b842df8f7a35fddda224e8aab961d5d2d5ea06`，HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`。实际 `CheckHealth` 准入、退出、取消、别名、其他根、缺失及部分 schema 行为均有隔离证据。五项定向、race、调用方检查和旧直连负向对照已核对。

- **CJSI-LAYOUT-R10-F1**。带 `agentdeck_acceptance` 标签的公开入口验收缺少 `store.OpenSessionDiagnosticsReadOnly` 行及 barrier probe。组合前提里的 `store.CloseLegacyReadPools` 也缺登记。实际两者均先执行 `GuardStateRoot`，共享 inventory 文件应包含两条受管入口和相应探针。候选下一轮补齐登记并保留诊断逻辑。

公开启用、完整接线和旧版二进制验收仍属后续。诊断对缺失路径给出该时刻的缺失观测，对非普通或多硬链接的 SQLite 文件沿现行受管 Store 规则拒绝。

## Round 11 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

**CJSI-LAYOUT-R10-F1 CLOSED。** 四路径提案 `fd82589209ba242d15371b6b01c173dcca05b461d2dc6e30dcae5a559f0f4387`，overlay `b0aef526e325cc5ddefd1fcd3844fe93d775eaa162803f0399647486ef3b78c5`。独立评审用实际工作树输入的非 Git 合成 fixture 验证带 `agentdeck_acceptance` 标签的 inventory 及两个新受管入口的 barrier 子用例，均通过。两个子用例均证明受保护访问之前拒绝，且各自对照通过。还原旧 inventory 会准确复现两条漏登记。沿用 Round 10 范围内的健康、race 和旧直连负向证明。组合采纳和 CLI 启用仍待排空验收。

## Round 12 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 评审限受管根排空五路径 `50ac8c75629011e9c1b169353a1bd6bd2721b5f8b1c3754ee70b58dc9db78319`。聚焦 56 项、race 39 项、scanruntime、vet 与 trace 定向测试通过。全 Store 和 backup 在该组合中仍有 13 个顶层失败。撤掉排空候选的对照得到相同失败集合，覆盖旧 schema 35 夹具与 quota 旧表引用。对照结果独立于本切片。

- **CJSI-LAYOUT-R12-F1**。现有写入方排空测试只断言 busy。去除 `OpenSessions` 或 `openAtExistingRoot` 准备文件前的准入时，连接级 pin 仍给出 busy，测试漏掉了先被创建的零字节 `-journal`。EX 租约期间对所有被拒 opener 核对目录项名称、大小、mode 和 mtime。测试专用补丁已准备，等待独立验证对应两种变异。

根身份复核和逻辑读租约仍用于一致性保护。已发布同路径的符号链接或多重硬链接会按受管入口规则被拒。旧版和外部连接的操作性交接、完整组合和迁移启用仍待验收。

## Round 13 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

**CJSI-LAYOUT-R12-F1 CLOSED。** 五路径提案 `8e52a3b0e42b6acca88b9a346fcabf0c2f29e06e26bc4249515e3a06c1974c81`，overlay `89ef81fa73a2b8d62411cb446064a37859de3af55583ae6ad86fec504fa619d6`。四个生产 blob 与 Round 12 相同。独立 Claude Opus 5.5 验证新目录快照断言，定向通过，排空 race 8/8。只绕过 `OpenSessions` 的提前准入时，测试检出零字节 `sessions.sqlite3-journal`；只绕过 `openAtExistingRoot` 的提前准入时，检出零字节 `agentdeck.sqlite3-journal`。两次旧测试对照均通过。主控复核定向、race 和变异收据。

次阶段使用单一隔离组合候选，接入实际 quota schema 36 迁移、匹配的备份消费方、审定注册、排空与诊断。原 schema 35 夹具对照及旧二进制验收义务延续。组合验证和迁移启用尚待完成。

### 组合验证结果

隔离组合 68 路径 `40b2d7a04f460503de2924ebdc19679edf81d3135363578054bef16dffe18ec2`，overlay `151979e802d9e4ab0d0ecdaa7645f4ede9b8a3775b5d21d9a3f979778286f569`。以实际 SAI 36 quota 迁移及备份消费方验证后，原 13 个 Store／backup 失败均通过。旧版夹具显式执行真实前 35 个迁移，继续断言 35、两个旧 opener 的 `SQLITE_NOTADB`、围栏完整和私有定位。原 35 源码对照也通过。session／doctor、带标签 inventory 和两条 barrier、聚焦 race、scanruntime 与 vet 各自通过。原生旧二进制仍由 Task 10 验收。

组合验证发现新的生产互锁问题。`TestAlertGenerationSnapshotAndACK` 在 SAI R8 对照通过，在组合候选以 `quota delivery guard busy` 失败，普通 schema 36 且未迁移时即可复现。Store 长持物理目录共享 flock，额度投递在同一 inode 取独占 flock。修复需使排空准入与额度投递锁兼容，同时保持两侧原有生命期、物理身份、排除及 Go／Swift 投递顺序。合并和启用前需验证修复及当前 main 的 read timeout、Store 和 inventory 接线。

### 锁兼容实验

主控逐一核对实验组合的 70 个 blob、产物与测试日志摘要。组合指纹 `01211dab5a04d97ec9d3c7587a13625496ff7b5001cf40734c4da2a4d6f97c4f`，补丁 SHA-256 `7b5a88f65601befa2bdcefdba5fc5b92df4f082e097665f9a736d9eac5d817a5`。预建独立锁 inode 后，持有 Store 时的额度 ACK 和 Swift 投递通过。排空仍先拒绝新写者，并等待逻辑关闭后保留的 SQLite 连接。

`TestCJSIMarkerlessV36HeldReaderDelivery` 在既有 schema 36 根上先只读打开，并保留一个仍可查询的 SQLite 连接。随后写入口正常打开，额度对账以 `quota delivery guard busy` 失败，退出 1。改到独立 inode 会遗漏仍在原目录上的读者，继续保持原锁则阻塞额度投递。实验补丁保留作失败证据，待验证读先开的方案后再进行组合采纳。

主控在 Darwin 合成根实测。普通 POSIX 共享字节锁遇目录独占 flock 返回 `EWOULDBLOCK`，只读目录描述符上的 OFD 独占字节锁返回 `EBADF`。数据库 OFD 字节锁可与 SQLite 锁位并存，但更换文件 inode 后，原连接持锁时新 inode 仍可独占。因此按数据库上锁的方案还需覆盖所有受管替换入口，证明不会遗漏保留连接。实验组合的并发 CAS race 出现 `SQLITE_BUSY`，同一未修组合的 24 次定向运行也复现一次，最终组合仍需核对此用例。

### 只读先开设计核对

独立 Claude Opus 5.5 复核既有 68 路径组合和失败的 70 路径实验，T3 配置 `effort=max`。额外验证目录 OFD 读锁仍排斥 A5.7 的独占 flock。按主文件上锁会遗漏 purge 或 Restore 回滚后仍保留的旧连接。modernc v1.53.0 在同进程关闭额外主文件描述符时还可能释放 SQLite 的进程级锁。没有发现能同时保持只读先开、物理根排空和现行 A5.7 的最小 CJSI 单侧修复。

候选决策是由 quota helper 创建独立的 `quota-delivery.lock`，以物理根的扩展属性一次性绑定锁文件 inode。Go 和 Swift 在同一文件上互斥并复核绑定，CJSI 继续对物理根排空。主控在合成根验证了单次绑定、持目录共享锁时的守卫获取、Go／Swift 所需的跨进程互斥，以及拒绝同名替换的守卫。最新 GitHub 发布为 v0.7.0，其通知调用路径以及本地记录的 `origin/main` 路径均尚无 A5.7 目录守卫。独立守卫涉及 SAI C10、A5.7、首轮初始化预算和相应验收，待用户确认契约变更。

2026-11-19 用户批准独立守卫文件。SAI 负责同一 inode 的初始化、绑定、Go／Swift 校验和不可用分支。CJSI 组合沿用物理根排空候选，待独立守卫实现后重验只读先开、保留连接、投递顺序及当前主线接线。

主线本地观察点 `929e95233a2658d3ed956c58cd88b01c16d7190c` 的隔离 Go 拼装预检合并了原 SAI A1.5 组合与 CJSI 68 路径候选。保留 refresh 锁截止检查和主线 `createLockEntry`，合并读池超时、所有权登记与两侧锁测试。定向 Store 排空、读池和锁测试通过。结构化清单还发现已受管的 `desktop.Service.RefreshQuotaCycle` 登记被跨主题合并删掉，以及主线 `doctor.Service.LockCheck` 待补。还原已有登记并添加 doctor 直接拒绝探针后，带标签 inventory 和该探针通过。预检 119 个 Go 路径指纹 `e502d46038f85179ca9e48aefef803492e7ee01fc85ac09f4566191793578911`，供最终组合参考。独立守卫及后续 A1.5 复评修订尚未纳入该隔离预检。

在上述主线观察点，首次叠加独立文件守卫与 A1.5 近截止修订的 Go 组合指纹为 `db5bca7ad74857b3f385ea76ba2e5357471a0e8d88558dc0af3508ce60fb2f30`。合并了同一调度分支的闸门余量判断和守卫错误返回。`TestCJSIMarkerlessV36HeldReaderDelivery` 以真实 schema 36 的 SQLite 先只读打开。关闭 Store 和池后，保留连接依然可查询。真实 quota 守卫初始化、对账和身份写入成功。此后关闭写者，在持有投递文件锁时，根目录独占仍拒绝旧读者，关闭该连接后立即成功。探针 SHA-256 `b1b934205cb4d656261bd18ff7203f071debd85429aa0324447da26c69b15c53`。同组 `TestCJSILegacyDrainAdmission`、`TestCJSILegacyDrainRetainedConnections` 四个子用例及 `TestAlertGenerationSnapshotAndACK` 均通过。候选仍待独立守卫代码复评及完整组合验证。

## Round 14 — 2026-11-19，独立守卫组合复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 复评绑定 CJSI HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、tasks blob `20bede4a595555dcdc88b0559b63fe38190d6d87`、SAI 架构 blob `8aa550e4bd233f4f7f2eca67f9b389a05729c1e2`、上游 `929e95233a2658d3ed956c58cd88b01c16d7190c` 和 121 路径组合 `674f5dee092580c67b8e9b4a7c0e0cd8f2741eb4abfd5cc8f46ed0f07182773b`。T3 配置 `effort=max`。只读先开探针使用 Round 13 记录的原始内容，参与了 Go 包测试。

- **CJSI-LAYOUT-R14-F1 OPEN**。组合中 `TestQuotaDeliveryIndependentDomains` 在 refresh core 仍打开时把根共享锁升级为独占锁。受管 Store 和其中的 SQLite 连接持有根共享 pin，主控在独立组合上复现 `EWOULDBLOCK`。复评运行组合 5/5 失败，SAI 单独候选 3/3 通过。这个断言与排空协议不相容，造成 `internal/quota` 包失败。
- **Round 13 投递互锁 CLOSED**。直接运行通过既有 v36 先只读打开、逻辑关闭后保留连接、排空等待期间的真实 refresh、绑定文件投递及释放顺序。关闭保留连接后，排空取得根独占锁，同时投递方仍能持有独立文件锁。把投递改回锁根目录或绕过读者 pin 的隔离变异都会使验证失败。Swift 在本轮核对了锁对象，真实 App 仍按原生验收执行。
- **CAS 测试及启用观察**。CAS 夹具同时附加 20 ms 和 5000 ms 的 SQLite pragma。实测 20 ms 生效，原夹具在 race 下失败 16/192，改为一个 5000 ms pragma 后失败 0/192。受管迁移候选的生产 DSN 也含重复值，当前实测 20 ms。C3 启用前需要明确生产等待策略并核对生效值。

组合的 Store、受影响 CLI、带标签的 ownership inventory、定向 race 与 vet 通过。生产迁移仍待 Task 8 的发起入口、读池协作和排空预算，Task 10 还需使用实际发布或准入的旧二进制核对围栏。

### 主控修复验证

后续隔离组合指纹为 `78646d324a7d255bd08e14484181d551a6a56d8a9e36624374351200efe583d9`，含 122 路径。将原只读先开探针列入清单。两份差异分别限测试文件，生产 blob 沿用本轮复评。

- 投递测试在 Store 打开时持根共享锁并对账。持有独立守卫时关闭 Store 和池，再取得根独占锁。另以只读投递方验证在根独占期间仍能取得同一文件守卫。组合锁测试、只读先开探针及告警 ACK 各通过五轮，SAI 单独组合的锁测试通过三轮。
- CAS 夹具明确只设置一次 `busy_timeout(5000)`，并断言实际设置。race 下八轮共 192 个竞争者完成，每轮一项成功、23 项返回旧状态。生产 DSN 意图留在 C3 启用判断。

122 路径 Go 组合的带标签入口清单、定向 race、vet 和 whitespace 检查通过。

全仓检查检出 Go 组合里的 Swift 读取器仍来自主线，支持 schema 35，而 helper 已发出 36。补入已评审 SAI 的九个 Swift 文件和 macOS 测试脚本。唯一重叠为主线追加的稀疏健康恢复用例，合并后同时保留该用例及 SAI 测试。跨层组合 132 路径指纹为 `92f3cd2747ce5e9526379a7020f82c3dcd68a930ac1c8a8d6ff1ef0e0761caa7`。schema 一致性用例和桌面刷新集成检查已通过。

补齐四个已评审 SAI 快照夹具后，跨层组合指纹为 `89b5b71fc642689d67741a70aeaf326e99721bee9e3665619fde0ac9a4d3c882`，含 136 路径。Go 的 schema 一致性、夹具可重现和字段边界检查通过。Swift wire 和嵌入 helper 定向测试合计 69/69，通过实际隔离 Go helper 和驱动器执行了所有权用例。最终组合 whitespace 检查通过。

**CJSI-LAYOUT-R14-F1 的修复验收通过。** 全仓 Go 运行在 122 路径组合上只有两个跨层断言失败，分别是 schema 35 读者与 schema 36 helper 不匹配，以及旧快照夹具与生产输出不匹配。最终组合复测了全部读入受影响文件的 Go 用例，三个顶层用例通过。两份清单的 122 个共同路径逐字节一致，其余 Go 回归、定向 race、vet 和入口清单结果继续适用。复评结论绑定原候选，后续隔离组合待正式采纳。

### 独立组合修复复评

独立 Claude Opus 5.5 对 122 路径 Go 候选 `78646d324a7d255bd08e14484181d551a6a56d8a9e36624374351200efe583d9` 和最终 136 路径组合 `89b5b71fc642689d67741a70aeaf326e99721bee9e3665619fde0ac9a4d3c882` 的定向复评为 **PASS**。主控核对相同的清单与运行回执。工作树 HEAD 为 `a224b0b824d2cad451d3bd83153832d65506f0fe`，上游基线 `929e95233a2658d3ed956c58cd88b01c16d7190c`。T3 请求 `effort=max`，实际推理档位未独立观察。

**CJSI-LAYOUT-R14-F1 CLOSED。** 复评逐一核对候选 blob。Go 修复改两个测试文件，并登记原先参与运行的 v36 只读先开探针。根共享期间真实对账能取得守卫。持守卫关闭 Store 和读池后可取根独占锁；继续保留一条专用 SQLite 连接时，根独占仍被拒。去掉 Store 关闭、读池关闭或物理连接 pin，以及让任一侧改锁目录，都会触发复评失败。组合锁、探针和世代 ACK 五轮共 15 项通过，race 三轮共 9 项通过。

CAS 夹具单次设置并检查 `busy_timeout(5000)`。复评八轮 race 共 192 个竞争者符合每轮一项提交、23 项旧状态的断言。主控把该测试的已评审 postimage `67f1bc511f9962e2a30eadb5ed486190b6778aad` 重放进 CJSI 工作树，独立运行该函数的 race 测试通过。生产等待策略由 C3 的 Task 8 启用入口核对。

最终组合在 Go 候选上增加 14 个 Swift、脚本和数据路径，复评确认 122 个原路径逐字节一致。三个读取这些输入的 Go 用例在最终状态通过，还原输入可复现原两条失败。最终 Swift 定向收据为 69/69。原 121 路径候选保留 FAIL 裁决。生产发起入口及协作关闭归 Task 8，真实旧构建围栏归 Task 10，安装 App 行为归联合原生验收。

## Round 15 — 2026-11-19，C1 版本感知定位入口

### 📋 定位入口独立评审

📊 本轮范围评分 8/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 需修复的问题

**CJSI-LAYOUT-R15-F1**（P3）。`internal/store/cjsi_layout_locate.go` 在校验围栏的记录身份前打开私有索引读取 seal。用同样的内容和不同 inode 替换 Ready 围栏后，定位函数返回 `errCJSIFenceInvalid` 和空位置，但原本为空的连接池留下了私有读连接。独立评审在迁移后的隔离状态根复现。修复要求在 resolver 之前校验记录身份，并保留解析后的围栏和根复检。

#### 🟢 通过的范围

定位在同一 core 事务读取 schema 版本和布局日志。未就绪和从未构建分支均保持关闭或缺席。已选中分支校验根和私有库身份、epoch 与 seal。定位与未来打开实际句柄之间的复验归 C3 的公共入口接线。

#### 📝 摘要

独立 Codex Astra `gpt-6.2-astra` 评审报告为 `CONCERNS`。T3 配置 `reasoningEffort=xhigh`。主控确认缺陷在本次定位改动范围内，裁决原候选为 FAIL。话题 HEAD 为 `5a1cd9159b564469b716a3338b5573adece90978`，组合基线为 `929e95233a2658d3ed956c58cd88b01c16d7190c`。被评审的 138 路径清单 SHA-256 为 `d67a9f40a733678e87212b3c586766246c27de3c155124165c2692f43631d388`，124 路径 Go 清单为 `b66a134d970ebbe1df3667e5227f4aaf651f723e50f5ae25bc71d066f114a8c0`。该定位测试原有 19 项通过，冷池探针补足了副作用验证。Task 2 的 CEv1 门禁查询返回 `missing_work_unit`。

## Round 16 — 2026-11-19，记录围栏预检复评

### 📋 围栏预检独立复评

📊 本轮范围评分 9/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 建议改进

无新发现。

#### 🟢 已关闭发现

**CJSI-LAYOUT-R15-F1 CLOSED。** Ready 定位在根和阶段确认后、根准入与 resolver 之前校验记录的围栏身份。复评逐项验证了两个清单及生产和测试差异。冷读池测试以同内容、不同 inode 的围栏触发 `errCJSIFenceInvalid`，验证空返回位置、私有读连接数为零以及目录条目保持一致。新测试在原生产 blob 上因保留读连接失败，修复后通过。仅移除预检的变异同样被该断言拦截。

#### 📝 摘要

独立 Codex Astra `gpt-6.2-astra` 复评，T3 配置 `reasoningEffort=xhigh`。候选延用 Round 15 的话题 HEAD 和组合基线。138 路径修复候选为 `5b30e16015f0a718b6e94d3e273bd45971fa497ac75cd4ed59fc570cd0a0f1d2`，124 路径 Go 候选为 `1c0796b47eb73dc17cb9f0b86613df12ecd034b179d811bedc7eb55d25735d20`。唯一变动为 `internal/store/cjsi_layout_locate.go` blob `a2a7bd053d8175bf7f2dbe524358f15d34edda80` 和 `internal/store/cjsi_layout_locate_test.go` blob `8c13755454f31102492c2c8d079f08df4d7b9d03`。主控复算清单和文件 blob，核对了 RED 失败行和 GREEN 计数。

`scripts/run-go-test.sh ./internal/store -run '^TestCJSIFencedStateLayout(Location)?$'` 有 42 项通过。定位测试 `-race -count=5` 有 100 项通过，包含五次新增断言，没有 race 报告。旧候选的全量 CJSI store 普通与 race 回执适用于未变化范围。解析后的围栏和根复检仍在。现有锚点覆盖 seal 读取期间的替换，预检与 resolver 捕获之间及最后一次复检前的两个外部时序窗口尚未定向触发。

此裁决绑定围栏预检修复。Task 2 的 CEv1 查询返回 `missing_work_unit`。Task 1 和 Task 2 的组合验证仍在准备，待实际组合候选关联接口证据和任务门禁后推进 C2。

## Round 17 — 2026-11-20，C1 工作集迁移

### 📋 受管迁移独立评审

📊 本轮范围评分 9/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 交接措辞

**CJSI-LAYOUT-R17-F1 CLOSED。** 此处明确排空导致的 `ErrStateBusy` 适用于已选中私有索引的准入和校验。合法的 Ready 且从未建索引状态可在读者准入前返回 `absent`。需要判断根当前能否执行工作的调用方使用自己的准入边界。裁决要求修正下游说明，已由本段落实。

#### 🟢 通过的范围

任务 2 使用已批准的工作集 DDL。受管 core 37 在记录版本的同一事务校验 schema 36 的 quota 前置条件并安装布局日志和工作集。仅用量回放可使用 core 工作集，合法的从未建索引根保持私有库缺席。旧的公共打开入口仍报告 schema 37 超前。

编号升级的版本戳被中止时，已提交的 quota 36 得以保留，37 的布局日志和工作集一并撤销。最后一个索引的命名冲突会撤销本次 DDL，保留事先存在的对象，移除冲突后可重试。冷建和实际适配器验证通过，真实适配器在迁移后的 core 暂存、封存、读取及退役，并检查外键和键复用。

#### 📝 摘要

独立 Codex Astra `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，比较基线为 138 路径组合 `41a5b0a04868d25b8088a75d5b4e7e157a6300813cdf10310aa4c71daa70d89d`。本轮 139 路径候选为 `77e46aa5f8fc50ad618104d7c21a948c771ed4f2bfb41f1f617e4b19614e6e81`，125 路径验证输入为 `bbc5882191a6f91e041b91caf3331d32e572bf8f783de9683144d71d3125fb45`。两处变更为 `internal/store/cjsi_managed_migration.go` blob `8cec61965d91214f17290ea248f5e152d37f2585` 和新增 `internal/store/cjsi_checkpoint_workset_migration_test.go` blob `66ca43a88404c77934b135655b9af5202d5f11b8`。

`scripts/run-go-test.sh ./internal/store -run '^TestCJSICheckpointWorksetMigration$'` 的四个子用例通过。丢弃 DDL、提前到事务外及推迟到提交后均触发失败。Store `^TestCJSI` 普通与 race 各有 167 项通过、1 项 WAL 子进程夹具跳过。已有联合回放重跑五个场景均通过。独立评审和主控核对既有回执，验收限该组合与两处改动。Round 16 的围栏预检和生产者及适配器的既有裁决适用。

工作集是 C1 的候选暂存。任务 2 继续负责 C2 的已批准 core DDL 和旧 telemetry 触发器迁移。任务 8 负责生产选择、可用性准入、读者生命周期、公共入口和同代 CAS。任务 9 负责从导出中删去工作集暂存，任务 10 负责真正旧构建的联合验收。Task 2 的完整门禁保持未验证。C1 下一步是将来源双消费者的独立裁决绑定到此组合。

## Round 18 — 2026-11-20，C2 schema 与快照读取

### 📋 独立评审

📊 本轮范围评分 8/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 需修复的问题

**CJSI-LAYOUT-R18-F1**（P2）。`internal/store/cjsi_core_facts_schema.go` 的 `cjsi_usage_sources` 缺少来源证据的 C1 workset round 与 attempt。现有 `(ticket,attempt)` 标识发布候选。工作集编号须独立记录。仅 usage 的根可能没有私有索引，core 来源证据仍须能回指确切工作集 attempt。仅在未发布的受管 37 DDL 增加两个正整数列，并以区别于发布票据的值验证持久字段。

#### 🟢 通过的范围

普通打开保留 schema 36 上限。受管 core 37 与私有布局的 DDL、旧触发器撤除及初始 ready 发布具有事务边界。未建索引的根保持缺席。未提交选择和回调后撤销均拒绝读取。索引保持稳定 ID、独立 ordinal、按侧 MATCH 和零命中覆盖。接管期、逐观察收据字段和保护复核符合切片范围。

#### 📝 摘要

独立 Claude Code Opus `claude-opus-5-5`，T3 配置 `effort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，144 路径候选 `d424388af816fce8f714af0acc748558622b909b77023ae1158e6ba17f7094f3`，Go `a3fd4f3709f4a05dc587cc6da58d453206936b1e191c08d9441299e844b9ae95`。九路径补丁 SHA-256 `4ed7312dc737ef05c53118e8d649f51da6c08cda4c2343d630b4c69df0f054da`。普通 81 条、race 28 条通过，独立隔离锚点复跑 28 条通过，主控核实了 core 表与私有 range 持久字段的差异。C3 公共发起按原设计使用写入初始头的受管路径，同步保护与选择 revision，并遵循单一 ID 分配器。

## Round 19 — 2026-11-20，来源工作集编号复评

### 📋 独立复评

📊 本轮范围评分 9/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 建议改进

无新增阻断。

#### 🟢 已关闭发现

**CJSI-LAYOUT-R18-F1 CLOSED。** 未发布的受管 core 37 将正整数 `workset_round` 与 `workset_attempt` 持久化到 `cjsi_usage_sources`，与来源身份和前缀证明一同保存。发布复合主键及外键保持原合同。实际迁移后的强类型夹具写入发布 `(1,1)` 与工作集 `(7,2)`，通过同一查询确认四个值。旧 DDL 对该夹具报告缺少 workset_round。独立复评的定向 green 和 race 各有 6 条通过。

#### 📝 摘要

独立 Claude Code Opus `claude-opus-5-5`，T3 配置 `effort=xhigh`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，起始候选 `d424388af816fce8f714af0acc748558622b909b77023ae1158e6ba17f7094f3`，修复后产品 `4e3f6750dff8d0b84cc514fbc6a1e6581f6087b39885c74152c3f858e13f11cc`，Go `f5143d811a0c2e2018d9febdb1a36f53b74fae1cc024f62b5898ada0006743c0`。增量补丁 `1cff89e984a196fe70c466728d5c23cffa290b0c052fd28e53183806c61a5f34`。两个后像为 DDL `c4d509ed62283bf6211edc4185921b7be9604702` 和 typed fixture `20f1f5b02ac7a05562622dd8b82c0bbcd289960c`。独立复评对比清单、严格一次补丁重放和 red/green/race。Round 18 中其他已验证作用范围可复用。

Task 3 后续候选写入必须从真实 C1 binding 保存工作集 ID，Task 8 保持 attempt 不复用并统一发布。字段隔离夹具的 PASS 已生效。实际写入、读者组合与完整 Task 2 验收继续按各 owner 执行。

## Round 20 — 2026-11-20，私有区间失效戳 schema

### 📋 独立评审

📊 本轮范围评分 10/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟢 通过的范围

受管 core schema 37 的冷建与回滚后升级均创建独立的 `STRICT, WITHOUT ROWID` 区间失效戳表。生产 DDL 与冻结 SQL 逐字节相同。六列主键含 scope kind，允许 aggregate-only 与合法空模型并存。同一迁移的后段收据表冲突会保留 schema 36、恢复旧触发器并撤销失效戳表，移除冲突后重试成功。普通 core opener 拒绝 schema 37，两条旧 session 打开路径在旧位置返回 `SQLITE_NOTADB`。

夹具验证全部五种 scope、正 epoch/ticket 约束、签名 UTC 日期范围、列类型与主键。非法 epoch/ticket 用例覆盖零值，严格正数约束另由实际 DDL 检查确认。Task 7 负责对外读缓存采用失效戳，Task 8 负责同事务发布及回滚。

#### 📝 摘要

独立冷评审为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task2-interval-stamp-schema-review-4fb10058`，authority epoch 21。启动只有通用工作区摘要，候选作者历史、执行历史和旧裁决仅按本轮简报核对。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。基线产品 `0cb8817f6e60d84c06072e56182e2b13561607a092ee93b97ffbeae49a0ddc0f`，候选产品 `4fb100585a88bafa0fbef3ba743d6603330efdacc31a39bc2c5465100d54f59b`、Go `aa519e66d49911179b93b0ce365e23df033bdf199961566e7e71d417a37fb1df`。两路径补丁 `808e3247f9c28cf3dcfd753226e3bad61cfedd0d60e06ac8344e6280865af7e7`，DDL 摘要 `e0d5c65f2473030bed20c1f8e2f438184d5819929c3f012ce092d2556ce033d2`。主控核对 20 项校验和，独立重放两个后像及模式，核对产品和 Go 清单。基线两个用例因缺表失败，候选普通和 race 各通过 50 条，vet 和 gofmt 通过。通过范围为区间失效戳的 schema 和迁移夹具。

## Round 21 — 2026-11-23，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 2，初版作者 AGY Gemini 4 Neon high，R1–R6 修复由 Gemini 与 Codex `gpt-6.2-astra`（max）完成。最终补丁 SHA-256 `01b3dc854163912a8c861efe6ddc3d2ffe92123f9da3fa67f41ed21264095737`，40 个路径，基于 HEAD `238dbf7`（已含 main 的 quota schema 36，CJSI 受管布局定为 37）。

- **评审历程。** 首评（Codex）FAIL：P1-1 CJSI 36 与 quota 36 版本冲突、P1-2 `session.CheckHealth` 绕过排空、P2-1 入口清单测试、P2-2 联合 store 测试。R1 关闭 P1-2、P2-1、P2-2。P1-1 改为“写 37 前必须证明真实 quota 36 结构”，R2–R5 依次被列缺失、复合主键缺失、名称大小写、空白等价 DDL 误拒、双引号 `"NULL"` 折叠、开放世界额外对象（触发器、FK 子表、view）击穿。R6 改为由真实迁移注册表在内存生成参考 36，按（类型，折叠名，所属表）对 table/index/trigger/view 做严格集合相等；同时修复 Task 5 发现的候选 FTS 列缺失回归 CJSI-SI-R4-F1。
- **独立复评。** Claude Opus 5.5 冷复评，与作者不同 harness。重生补丁逐字节一致，冻结副本不含 Task 3/4/5。P1-1 与 CJSI-SI-R4-F1 关闭，此前各项保持。13 个对抗变体全部被拒；11 个发行版新建库、逐跳升级链、`.dump` 重建与 VACUUM/ANALYZE 后的库全部升到 37 且幂等，无误拒。14 个历史二进制 SHA-256 核对一致。
- **验证。** `TestCJSIFencedStateLayout` 36/36、`TestCJSIQuota36PrerequisiteValidation` 66/66；`internal/store` 普通与 race 各 438 通过、3 跳过（与既往相同的环境性跳过），无 data race；vet 干净。HOME、XDG、TMPDIR、GOCACHE 全部隔离。
- **非阻塞 P3。** CJSI-LAYOUT-R6-F1：`internal/store/cjsi_quota36_schema.go:267-268` 整类跳过 `sqlite_` 前缀对象，含触发器与 view；仅能经 `PRAGMA writable_schema` 构造。修复契约：只放行 `sqlite_sequence`、`sqlite_stat1`–`sqlite_stat4` 及 sql 为空的 `sqlite_autoindex_*`，其余 `sqlite_` 对象返回 `ErrUnknownSchema`，补 36/37 拒绝与 ANALYZE 接受两条回归。留待后续 Task 触及 schema 校验时收口。
- **门禁。** CEv1 WorkUnit `codex-jsonl-session-identity:fenced-state-layout` 在该补丁状态上独立评审、L2 验证、范围与隐私 3/3 满足。实际写者、读者组合与同代发布由 Task 3–8 集成验证。
