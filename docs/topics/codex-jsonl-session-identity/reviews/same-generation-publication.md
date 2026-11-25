---
status: active
topic: codex-jsonl-session-identity
subject: same-generation-publication
---

# 同代发布评审

## Round 1 — 2026-11-20，私有发布文件耐久性

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

**CJSI-PUB-R1-F1。** 文件初始化时 `Fstat` 失败会直接按临时名称调用 `Unlinkat`。延迟清理又在 `Fstatat` 核对 dev/inode 后单独按名称删除。两步之间若目录项改绑，删除操作会作用于替代对象。持有目录描述符可固定父目录，目录项的绑定仍可变化。现有重绑测试发生在核对之前，无法覆盖核对与删除之间的窗口。无法保证精确对象身份时，失败路径应保留未封存项，交由具备占用和回收权威的协调器处理。

#### 🟢 通过的范围

私有目录、文件大小和身份门禁通过检查。顺序为完整写入、文件同步、关闭、排他安装、目录同步及重新打开并核对内容和 inode。先到的同身份写者保持已安装内容，后到写者得到碰撞结果。同步错误返回零凭证，目录同步失败可留下未封存文件。发布协调、元数据封存及公开同票读写由后续 Task 8 路径负责。

#### 📝 摘要

独立冷评审为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-slot-durability-review-8a0fb4d1`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。候选产品 `8a0fb4d1d3105213ebaa3c07c37511c8427e4c200c26b31cd99edbaa05d1e0ad`、Go `67ffc2c7ae61e99f820aae5b6044fb80aa91a22eddae7c2f3f2a616e6b8f58b8`，补丁 `d2164b7388081a9b773523ff2e8331e4a8a2214c2f2063b7168082e486da0303`。主控核对两文件的严格回放、文件模式和完整清单。普通和 race 日志分别有 22 和 11 条通过，均涵盖 10 项文件子用例，无跳过。amd64 为本机执行，arm64 为构建验证。通过范围保留为局部证据。

## Round 2 — 2026-11-20，未封存文件清理复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-PUB-R1-F1 CLOSED。** 失败路径关闭属于本次写入的描述符，保留未封存文件供协调器在核实归属后回收。文件辅助函数不按名称删除目录项。已占用的目标含悬空符号链接，在建立暂存前返回碰撞。排他安装仍拒绝在文件同步后才出现的占用。原实现删除了同步失败或迟到碰撞后的暂存文件，两条新版断言在原实现失败，在修复候选的普通和 race 均通过。碰撞夹具同时核对先到对象的 inode、内容、失败方的零凭证及保留的暂存内容。

#### 📝 摘要

独立冷复评为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-slot-cleanup-r1-rereview-9d21fb14`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `9d21fb140541335a46034cf7c801a8510345259f24388d321022182028a2b358`、Go `412f98bb9d38d6ea725a933d949cfc9abf336e96cc8c8734f6f46bf874a932ca`，增量 `9cda2589dd3ed970de796d34d4a6fcc5a9c269476013df511bb8b5dfdcad9feb`。主控精确重放两个后像与模式。普通 22 条、race 11 条通过，零跳过，vet、gofmt、Darwin 两架构构建通过。

额外普通和 race 探针使用隔离受管 core。独立 SQLite 连接在文件和目录的同步回调分别暂停时，各完成一次 `BEGIN IMMEDIATE` 至提交，最后可读取两次写入。实际 scanruntime 锁交接、单根并发名额、私有元数据封存和公开同票发布仍待验证。

## Round 3 — 2026-11-20，私有封存准入

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

**CJSI-PUB-R3-F1（P2）。** 借用 core 写事务时，主查询按主键定位目标试次，重复身份检查却按 `slot_identity` 遍历其他历史试次。受管 schema 37 缺少该键的索引。隔离的受管 SQLite 夹具插入 4,096 条无关退役试次，实际封存 SQL 的 `EXPLAIN QUERY PLAN` 输出包含 `SCAN other`。合法的首次封存和精确重试均需要完成该查找，成本随保留历史增长。修复应使含退役历史的身份检查使用有界索引查找，并继续拒绝冲突身份。

#### 🟢 通过的范围

在调用方独立选定当前准入的前提下，封存对比完整提案、文件凭证和受管期望值。过期、换根、保护或覆盖变化以及不同凭证受原边界拒绝。首写更新一行，精确重试零行。陈旧 WAL 视图与回滚有实际 SQLite 结果。封存凭证以写事务提交为耐久边界。公开票据切换及生产证据组装仍由 Task 8 后续接线负责。

#### 📝 摘要

独立冷评审为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-slot-seal-review-234aea25`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `234aea25d9e8d01b440425533ce3fc8d61cc50afe6a3e8b8fb8c5bb546ae4a04`、Go `0760d2ce5ac78295e8aeb79ced1fbf4d9b9f975b8af1548f547809bbf4e2d78c`，增量 `0883c574e59c259ca4148be2d76c4940127476b722f41d749f6d8b9b266ae4b5`。主控精确重放两个后像、核查模式和两份全清单。普通 43 条、race 37 条通过，均无跳过。独立评审另执行了查询计划见证并核对 529 字节封存向量。

## Round 4 — 2026-11-20，封存身份索引复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-PUB-R3-F1 CLOSED。** 受管 schema 37 加入覆盖非 NULL `slot_identity` 的非唯一索引，范围包含退役试次。实际准入 SQL 的文本在提取成共用常量前后相同。在 4,096 条退役历史下，修复前的 `SCAN other` 变为按 `slot_identity` 的覆盖索引查找。使用真实参数的独立夹具在 draft 与 sealed 均确认同一索引，前者更新一行，后者返回原 seal 并更新零行。已验证同票不同试次及不同票相同试次号的退役身份冲突均被拒绝。拒绝不改写记录。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷复评，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-slot-index-r3-rereview-bfa9c85c`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `bfa9c85ceee01f9a080d61c5dae0268778d636c243b434f3e397d822d73bbc39`、Go `40ce3bf9e6094761dc34e5fe58d08c04f70d8120e13479d7a4264830bbb89321`，三路径补丁 `6b8d527d4ab54eff60f6f116a729f8d8c103ef9e8b90928475ad51f7b0aa941b`。普通和 race 各有 74 条通过，零跳过。真实参数与冷建 schema 37 由独立探针补验。升级、回滚及普通打开的 schema 上限控制通过，退役试次继续参与身份冲突校验。通过范围为私有封存准入中的索引修复。

## Round 5 — 2026-11-20，单根发布文件工作器

### 📋 独立评审

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 通过的范围

工作器通过借用根描述符相对打开 `.`，检查其与已准入物理目录具有同一 inode、当前用户所有及 `0700`。独立打开的描述符持有非阻塞排他锁，保持到文件写入、两次同步与读回核验完成。等待者超时返回零凭证，原根仍拒绝同进程和跨进程第二个写入，另一根可推进。取消后即使调用者清空缓冲区、关闭原句柄或替换路径，工作器仍使用原根和已复制的字节。完成或失败释放占用，失败路径保留未封存项。**CJSI-PUB-R1-F1** 的关闭仍适用。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-slot-worker-review-fd458b07`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `fd458b076c928c7473912e4043063f6c540a7fe9af8c09963b72b11df1759b6b`、Go `4d8295c4dab1584619f6dfa5d43d8ac2bab1009c588404aeadd667911080f37c`，补丁 `428d1edb833e75f19a2b02f36d006be306491ab53c2dd7854c57fe6b9d8052f9`。三个后像精确重放。普通和 race 各 38 条通过，包含原耐久用例。另一个受控夹具在文件及目录同步各暂停时验证同一调用者句柄的竞争，并成功取得实际 maintenance 和 state 锁。通过范围为独立文件工作器。scanruntime 生产调用仍要释放阻塞锁及 SQLite 快照，并遵循实时试次与开票门禁。

## Round 6 — 2026-11-20，草案登记与冻结

### 📋 独立评审

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 通过的范围

登记先取得 SQLite 写入权，再核对当前试次、受管根及围栏、基准票据、代际、依赖和预留身份。新记录为私有 draft，聚合 diff、session seal 及文件凭证均为空。重新打开后精确重试更新零行。竞争写者、过期、退役重复身份及陈旧 WAL 快照被拒绝。

冻结核对同票 session seal 及完整受影响 group 头。查询限定当前票据和试次，并以预期数量加一限制成员枚举。缺少、额外、替换或摘要/状态不符均拒绝并回滚。已冻结试次可进入既有文件凭证准入。由真实 usage Stage 执行的隔离夹具初写两个组，冻结前同组重试成功，冻结后拒绝两组。夹具提供空来源、工作集以及旧行集合。完整成员及来源证明仍由调用者提供并保持到冻结。公开表在此步骤的前后读回一致。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-publication-prepare-review-2de35d54`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `2de35d5466db620a0a2bf59e08b3ea2a0510c072e5ab881df3d0b93e887535e2`、Go `abfb6dbdd7fe1694818f80b7c8cc130d72f583235dfa6c6dbb851e9887f54eab`，增量 `bed75cedceadc6a5e1c7639785326927f670d0e80b0c1a01336a80fd37069abb`。严格重放两个后像和文件模式。独立评审核对 210 条产品和 196 条 Go 清单，复用普通及 race 各 33 条通过的记录和独立进程 Stage 夹具。当前通过私有登记和冻结切片。实际锁交接、提交和公开配置继续由协调器验证。

## Round 7 — 2026-11-20，发布协调器联调锁域

### 📋 主控联调检查

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

**CJSI-PUB-R7-F1。** 文件工作器在根目录 inode 上持排他 flock，而普通 Store 准入在同一目录上取共享锁。真实 desktop 文件工作器暂停同步时，两类项目锁及独立 SQLite 写事务可完成，普通根准入返回 `ErrStateBusy`。主控将真实 peer 的检查改为要求准入成功，实际复现失败。另外，在隔离受支持版本的普通 Store 打开后，前序工作器甚至在进入同步之前就被同一锁域挡住。单根文件工作器占用须使用独立锁域，继续以物理根和拥有的对象绑定写入。

#### 📝 摘要

这个联调观测由 Codex `gpt-6.2-astra` 实现候选先报告，主控通过真实受管 SQLite 和已有文件 peer 核对。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，epoch 21，候选产品 `e4ee9d3d9e9bba91f349aa65e8b56a4ea628aaecbd057a3ba5884a7a81782b20`、Go `bac093e5516b13c3d17ffff0847cc3c4875b2602ab9b4434a5b9ffeb20d94915`，增量 `8b9a9d8b2e35879cfbd8fc37a1ebaba0626126b08a7c67dad093bbde9a7034ba`。原实际 peer 的普通及 race 各 18 条通过记录包含明确的冲突观测。当时测试预期该忙错误，因此通过记录不满足生产 Store 并发验收。

## Round 8 — 2026-11-20，最终 core 事务独立评审

### 📋 独立评审

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 通过的范围

实际 core 写入权在领域变更前取得，核对原基准和完整有效的双 seal。应用已冻结差分后，公开头在同一事务选择 session 与 slot 并更新覆盖与失效戳。已打开试次的收据和最终行都有读回检查。受控独立见证在最终更新触发 SQLite CHECK 275 后验证 `Commit` 返回 `sql.ErrTxDone`，新连接见完整原像。旧读者在提交前后持有旧票，重开读到同票新 slot、session 和 17-token 事实。已核实恢复采用新连接并拒绝改绑收据或已被新头替代的尝试。实际来源、完整差分、真实文件及 session 生命周期资格由生产调用者证明。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-publication-open-review-8be1e02a`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `8be1e02af305f55479c1cb1f4e80aad037db27cf01ac4f9617ee63a6b8eaa274`、Go `53155a24fce2a280432876d0bbf75c2a30f55c122e4b37a89ae401ef1bf110c1`，增量 `06be1e13650e0c0657b62b6c29211acd21a230ed57889e78ba8823d08c33b2d0`。两个文件及完整清单、普通和 race 各 29 条通过记录及双架构构建核对。通过范围为最终 core CAS 组件。生产锁域、实际运行时证据和所有公开读入口仍待接线。

## Round 9 — 2026-11-20，冻结到开票协调器与锁域复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-PUB-R7-F1 CLOSED。** 工作器在已准入的目录描述符下打开持久的私有 `.cjsi-serving-worker.lock`，核对普通文件、空内容、所有者、0600、单链接和持锁后的目录项身份。这个零字节文件由同一物理根上的合作工作器保留，普通 Store 继续使用目录 flock。实际普通 Store 在文件及目录同步分别暂停时均成功打开并提交独立写入。受管 runner 的真实 desktop peer 在文件同步暂停时允许根准入及独立 SQLite 提交。等待者取消后，工作器持有独立占用至文件 I/O 及读回返回。

#### 🟢 通过的范围

runner 核对已冻结或精确已封存试次，关闭预检连接后写实际文件。受控调用在短事务之间分别提交私有封存和最终开票。过期或变化导致拒绝，已提交私有 seal 可供后续核验。最终提交结果不明时在新受管读连接确认原活跃头、试次和实际后像。旧读者保持旧票并见零条夹具事实，新读者读到双 seal、session 戳及 17-token 事实。来源与领域事实由夹具提供。公开 scanruntime 锁交接、真实来源和调用者证据仍待集成。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷复评，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-runner-lock-domain-review-5f9787ad`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。修复产品 `5f9787ad5c03a7f6941044fc2ed8318ea11ad117798ccad2c54f1014e77d3f99`、Go `96bfcf3e2fcd2cb679cb8107a6dd7ff9f5ac4eaa72ed9c9dfcf7b87a0a9f55e9`，补丁 `4624553f8c86f646cc2a1f30dfd6a6ff2eec688607caaf5d8d6e2704dbe26a96`。严格回放和双架构构建核查通过。桌面普通 32 条、race 21 条通过；受管 runner 普通和 race 各 18 条通过，各无跳过。复评复用真实文件 peer、完整 manifest 和测试日志，无新探针。

## Round 10 — 2026-11-20，正常会话列表与搜索分流

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 阻塞发现

**CJSI-PUB-R10-F1（P1）。** 普通 schema 36 core 无受管表时，缺少 `proveIndex` 会使旧版路径只检查 `sessions.sqlite3` 的 SQLite 文件头。即使这个旧文件名下的库已含 `cjsi_index_state`，仍可能进入可写的 `OpenSessions`。独立评审通过源代码定位，初次夹具未确认进入回滚日志分支。主控在两个各自独立的合成根中，确认 core 为 schema 36 且没有受管对象、两个数据库均为 DELETE 日志模式、头版本均为 1、三种 sidecar 均不存在。原候选的正常 `session list` 与 `session search` 分别返回四条结果，改写旧名索引，并留下 WAL 和 SHM。回归测试以退出码、空结果、文件哈希和目录条目验证拒绝。修复范围为现有索引的只读标记检查，以及两条正常命令的回归测试。修复待冷复评。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-managed-session-cli-read-review-2a200e83`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `2a200e83a68414c43823fa08a84df6c981c4eaba29ffbfc97bc3a7b34f006968`，Go `42db12b7062a87cc9e3e0842199fea7d076e0f48f67d902f0b74d48a7186b5c7`，补丁 `cd153bdbfa6b2fa77c1262634c8e8d716c469ed7768ec846542937c1cfdcae91`。复用正常 487 项通过及一项 fixture 跳过、race 38 项通过、Darwin 双架构编译。主控用仓库 wrapper 运行 `TestCJSILegacySessionCLIRefusesManagedIndex`，两个子测在原实现失败，退出 1，日志 SHA-256 `4ba41a7ea2693f783cf5158df824014ff95f6aa9126cf7effdc55fcf4e6bd334`。冷 WAL 的只读可用性由单独候选承担。

## Round 11 — 2026-11-20，冷热 WAL 只读适配器

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 阻塞发现

**CJSI-PUB-R11-F1（P1）。** 外部进程保持 WAL 可读时，受管 reader 若成为本进程第一个映射 SHM 的连接，会以 `readonly_shm=1` 创建按数据库 inode 共享的 Unix VFS SHM 节点。后续同进程普通写连接继承只读标志。独立回归中，受管 reader 在两个库均读到三条预期记录并正常返回。普通连接的 INSERT 在 reader 打开时和关闭后均返回 SQLite `SQLITE_READONLY (8)`。关闭受影响写连接后，新连接提交成功并查到该行。现有同进程测试先开 writer，因而漏掉 reader-first 顺序。仓库 wrapper 的隔离 Darwin amd64 探针退出 1，原始日志 SHA-256 `3be98b8b03f46b946b0fe5a35721ba91f06e89ea1967e3f56e665fa3e7e9e0dd`。主控复核 vendored Unix VFS 的共享节点和 WAL 写入拒绝路径。修复须兼顾两种打开顺序、源库及 sidecar 原字节、独立写入和整组读结果复验。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-cold-wal-adapter-review-codex-ae02bf47`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `ae02bf47cd69201671532ce7bd13ff04cb80b3ebef47611db945b979e1d1f26e`，Go `89c9336d45ba501725dff13fbe555d4a60136af80750bbaba623558e85051469`，补丁 `ccc7a6d9e3ff106bed4b0e82385e2fd43b4e4046eefe9da4384189003434be11`。主控严格回放四个文件并核对提交证据。直接 hook 的两个冷热组合各返回四条，原库和 sidecar 保持原样。已提交 WAL-only 行、损坏拒绝、变化检测有绑定夹具。普通 CLI 接入仍待该缺陷关闭。

## Round 12 — 2026-11-20，旧库只读分流复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-PUB-R10-F1 CLOSED。** 在正常 list 和 search 交给旧版可写路径前，对每个存在的旧名 SQLite 索引做只读 schema 检查。受管标记拒绝；缺失或损坏 core 仍要求适用的两张旧表正向证据。查询在共享迁移准入下读取一次一致 schema，减少无关旧库 DML 导致的误拒绝；不可变探针前后均检查 sidecar 缺失。实际混合根两条命令均拒绝且保持原文件与目录条目。正常旧库 scan 后的 list/search 及既有隐私用例在普通及 race 下通过。独立使用未打标签的 amd64 CLI 验证损坏 core 与仅一张旧表拒绝，输出为空，文件与目录不变。

#### 📝 摘要

T3 Codex `gpt-6.2-astra` 冷复评，`reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task8-legacy-dispatch-rereview-be5f212a`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `be5f212a5f7d665f5e3c9d191e848759e784f3b1e77c7249f66989ba54bed569`，Go `1e5dbc7a5fc837b9888b3274fabbfec32bd4a38ac9177b255a51056d26ee1b55`，补丁 `1eb56a21422338ebb9f62f91b259ec447a7401fca060a47d4a504f58a98a8b9f`。严格回放两文件。聚焦普通 37、race 35 条通过。完整命令与 Store 包 1,325 条通过、7 条跳过，日志 SHA-256 `42b4c96d8f301ffe0f28c5f29ee062bd810efdc846b4263cce7fca86cc73c924`。vet、gofmt、双架构构建通过。Task 8 整体门禁仍待生产接线与联合证据。

## Round 13 — 2026-11-25，重建单元 1 发布核心

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 阻塞发现

**CJSI-PUB-U1-R1-F1（P1）。** 依赖失效的试次不退役，后续重试被卡住。**CJSI-PUB-U1-R1-F2（P1）。** pending 覆盖了 partial 覆盖状态。**CJSI-PUB-U1-R1-F3（P2）。** 被替换根下的工作器锁在获取前未核对。**CJSI-PUB-U1-R1-F4（P2）。** 试次 BuiltAt 可越过 AcceptUntil。**CJSI-PUB-U1-R1-F5（P3）。** 撤回集合缺少完整性核对。

#### 📝 摘要

重建版单元 1 覆盖 store/scanruntime 发布核心、CAS、Task 6 屏障和 Task 5 移动与复用，基线 `f5d93d9`。作者 Codex `gpt-6.2-astra` xhigh。AGY `gemini-4-neon` high 冷评审，请求 `v075-e21-cjsi-rebuild-task8-unit1-review-8e3156fc`，补丁 `8e3156fc71253ddc0057986862e53f0a414efb293d7635c65ac230f042a97990`，37 个路径。上游授权修改：`cjsi_core_facts_schema.go` 部分索引 `cjsi_publication_attempts_slot_identity`；`cjsi_candidate.go` 的 `Withdrawals()` 与重启恢复入口；`cjsi_ledger_staging.go` 只读 `VerifyStaged`。这些分别在 Task 2、Task 3、Task 5 评审记录中补记。

## Round 14 — 2026-11-25，单元 1 第 1 轮修复复评

### 📋 独立复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

CJSI-PUB-U1-R1-F1 至 F5 全部关闭。

#### 🔴 新发现

**CJSI-PUB-U1-R2-F1（P2）。** 已封存试次越过 valid_until 后不退役，重试被卡住。**CJSI-PUB-U1-R2-F2（P3）。** 缺少孤立 delete 负样本，LEFT JOIN 变异存活。

#### 📝 摘要

AGY `gemini-4-neon` high 冷复评，请求 `v075-e21-cjsi-rebuild-task8-unit1-r1-rereview-5b02328f`，补丁 `5b02328f4dbc7c6f06136d301ce154c7546b46aac70a402af525b7e45ed5992f`。修复为第 1/3 轮，同一作者线程。

## Round 15 — 2026-11-25，单元 1 第 2 轮修复复评

### 📋 独立复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

CJSI-PUB-U1-R2-F1、F2 关闭。生产代码无 P0–P2；锁外窗口重验经核对无 TOCTOU。

#### 🔴 新发现

**CJSI-PUB-U1-R3-F1（P3）。** 窗口策略准入无测试保护：变异 X1（变窄窗口不退役）、X3（Open 截止不等于准入截止）、X4（过期后继被准入）、X5（时钟缺失不重验输入）均存活。**CJSI-PUB-U1-R3-F2（P3）。** 孤立 delete 的空身份哨兵无测试，X6 存活。

#### 📝 摘要

Claude `claude-opus-5-5` xhigh 冷复评，请求 `v075-e21-cjsi-rebuild-task8-unit1-r2-rereview-09e5f92e`，补丁 `09e5f92e1ae4eaf9c4d198126e0a23274032b98534b0ec2630b0a4bf7fdd3564`。修复为第 2/3 轮。第 3 轮修复限定在 store 测试。

## Round 16 — 2026-11-25，单元 1 第 3 轮测试修复复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-PUB-U1-R3-F1 CLOSED。** `cjsi_publication_retry_test.go` 新增四个用例：
- `TestCJSIPublicationNarrowerSealedWindow`：变窄窗口时退役已封存试次，保留原 seal 与 slot inode；
- `TestCJSIPublicationOpenRequiresAdmittedCutoff`：准入截止 ±1 分钟均拒绝，且 head 不变；
- `TestCJSIPublicationClockMissRevalidatesInput`：时钟缺失后重新验证输入；
- `TestCJSIPublicationClosedSuccessorKeepsDraft`：后继截止已过期时保留原预留。

分别杀死 X1、X3、X5、X4，每个变异只让对应用例失败，退出码 1。

**CJSI-PUB-U1-R3-F2 CLOSED。** `TestCJSIPublicationWithdrawalSet/orphan_sentinel_identity` 要求 `ErrCJSIIndexChanged`，杀死 X6。

#### 🟢 通过的范围

复评者在独立副本中逐字节核对 37 个路径，相对第 2 轮候选只有该测试文件变化，生产代码相同，第 15 轮对生产代码的结论继续有效。新断言基于数据库行、slot 文件 inode 和返回错误；时钟缺失场景由业务时间键确定性触发，无 sleep 或轮询；夹具使用临时目录。无 P0–P3。

#### 📝 摘要

Claude `claude-opus-5-5` high 冷复评，请求 `v075-e21-cjsi-rebuild-task8-unit1-r3-rereview-1a09ada1`，补丁 `1a09ada117ef998cf8813f2b663ba8b6da2ba6fbea4c52b12de67f9caafa5690`，作者 Codex `gpt-6.2-astra` xhigh 第 3/3 轮。验证结果：新用例通过；锚点 `TestCJSI(PublicationLifecycle|SlotDurability|ReaderConsistency)` 在 scanruntime 与 store 通过；store 包 `-p 1` 全量 655 项通过、3 项跳过，日志 SHA-256 `4957787933ee581610c3b7d3a3cbc46d726388ec733c30a9aa6e1788d715e617`；发布相关 race 37 个顶层用例通过、无 DATA RACE，日志 SHA-256 `505cf5f7694515f372ee2589b0adb666bfd7252f7d1555baf62ee560f0753858`；vet、gofmt 无输出。Task 8 整体门禁待单元 2（desktop、watch、ingest 指标、perftrace、CLI 接线）完成后联合验证。

## Round 17 — 2026-11-25，单元 2a 报告读者与派生缓存

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 阻塞发现

开票前的 proposed 路径没有执行真实写入路径的前置条件，真实 Apply 会拒绝的输入在这里被放行。每项都有评审探针复现。
- **CJSI-PUB-U2A-R1-F1（P1）。** 变更线程已在 `cjsi_usage_selection` 却被父组遗漏时不拒绝，脏范围被漏算，聚合戳停在旧票。
- **CJSI-PUB-U2A-R1-F2（P1）。** `RestoreOTel` 与恢复型变更不一致时仍放行；恢复前缺少与 Task 4 vacate 相同的残留检查，未撤回的 JSONL 与恢复的 OTel 双计（500 + 3）。
- **CJSI-PUB-U2A-R1-F3（P1）。** 请求覆写未登记的既有 `event_key`，`ApplySelectedRequests` 会拒绝同一输入。
- **CJSI-PUB-U2A-R1-F4（P2）。** 空变更组跳过 schema、layout、`accept_until` 与 revision 检查。
- **CJSI-PUB-U2A-R1-F5（P2）。** 父票 head 校验、`SelectedRanges` 守卫、父票 0 脏范围检查无负例，三个评审变异存活。
- **CJSI-PUB-U2A-R1-F6（P3）。** checkpoint 起始时刻推导重复实现，缺少一致性校验。

#### 🟢 通过的范围

四处授权上游修改未越界：Task 7 proposed 入口与目标聚合戳、`usage.go`/`providers.go` 只读 queryer 提取、`CJSIProposedView.FrozenDiff`、Task 4 `ReportEffects` 与 Apply 共用筛选。已开放路径与 Apply 的语义不变。洛杉矶跨日、价格、路由、工具归属与合法 OTel 部分恢复的 proposed 结果与 Open 后逐字段相等。desktop 编码在借用事务释放后进行，v1 缓存字节不变，缓存键绑定完整。

#### 📝 摘要

单元 2a 由 OpenCode `openai/gpt-6.2-astra` xhigh 实现。实现中两次遇到跨锚点阻塞（开票前读取边界、OTel 只读效果集），经主控授权后继续。AGY `gemini-4-neon` high 冷评审，请求 `v075-e21-cjsi-rebuild-task8-unit2a-review-c2e76500`，补丁 `c2e7650002e4d2bd0924304eb9ae395c1e25801a6f536f7dd299e965681e5096`，21 个路径。四包全量、发布回归、race、vet、gofmt 通过；作者 6 个变异被复核杀死。

## Round 18 — 2026-11-25，单元 2a 第 1 轮修复复评

### 📋 独立复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

CJSI-PUB-U2A-R1-F1 至 F6 全部关闭。修复复用真实写入路径的判定，没有另写一份：Task 3 的 `requestIdentity`/`checkReplacement`、`verifyUsageStagingSchema`/`usageLiveAttemptSQL`、`checkpointStart`，以及 Task 4 的 `checkVacated`。复评者对照 `3e3afb5` 逐行核对，`ApplySelectedRequests`、`writeRequest`、`VerifyStaged`、`writeCheckpoints`、Apply 与 vacate 的行为、错误、写入和 SQL 均不变。冷评审探针全部通过，RM1–RM4 与作者 9 个新变异全部被杀。首次构建父票 0、仅撤回加删除、合法已登记覆写、洛杉矶跨日空组四个正向对拍未被新校验误拒。

#### 🔴 剩余发现

复评者报 PASS，但有 3 项 P3 未关闭，按规则退回修复：
- **CJSI-PUB-U2A-R2-F1（P3）。** 导出方法 `VerifyReportVacated` 缺少 nil 守卫，且签名暴露未导出接口。
- **CJSI-PUB-U2A-R2-F2（P3）。** 遗漏变更父组的拒绝断言只在 remove 子例执行，变异 R2M4 存活。
- **CJSI-PUB-U2A-R2-F3（P3）。** 缺少能杀死 R2M5（仅区间成员与残留 `otel:` 行）与 R2M1（替换元数据校验）的测试。

#### 📝 摘要

原作者 OpenCode 额度耗尽，修复改由 Codex `gpt-6.2-sol` high 接手。AGY `gemini-4-neon` high 冷复评，请求 `v075-e21-cjsi-rebuild-task8-unit2a-r1-rereview-0d1fc06e`，补丁 `0d1fc06e4d5e008b6cb116cc8425f24ea690d66f4cfde6fafb024b71f38897f1`，28 个路径，修复轮 1/3。

## Round 19 — 2026-11-25，单元 2a 第 2 轮修复复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

- **CJSI-PUB-U2A-R2-F1 CLOSED。** `VerifyReportVacated` 对 nil 视图返回前置条件错误。新导出的 `store.CJSIOTelReadView` 只含两个只读查询方法，不扩大写能力。
- **CJSI-PUB-U2A-R2-F2 CLOSED。** replace 与 remove 两个子例都断言遗漏父组被拒，R2M4 被杀。
- **CJSI-PUB-U2A-R2-F3 CLOSED。** 新测试杀死 R2M5 与 R2M1；四个正向对拍已纳入持久测试 `TestReportValidPublicationParity`。

#### 🟢 通过的范围

与上一候选相比，生产代码只有 nil 守卫、只读接口导出，以及从 `ApplySelectedRequests` 抽出等价的 `applyVerifiedRequests`，其余行为不变。抽查的旧变异仍被杀死。无新发现。

#### 📝 摘要

AGY `gemini-4-hydrogen` medium 冷复评，请求 `v075-e21-cjsi-rebuild-task8-unit2a-r2-rereview-c7b44cca`，补丁 `c7b44ccad9a4d16b9c2309dd47a2eb2b3b2715649b27462d9d89ae580fd2a6a9`，28 个路径，修复轮 2/3，作者 Codex `gpt-6.2-sol` high。usage、store、desktop、scanruntime 全量，单元 1 发布回归，vet（含 tagged）与 gofmt 均通过。作者 27 个变异全部被杀。Task 8 整体门禁待 2b 生产接线与联合证据。
