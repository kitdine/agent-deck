---
status: historical
retired: 2026-11-27
created: 2026-11-17
---

# 缺陷：刷新后保留已结束扫描和旧健康警告

主载体：`ad-bug-serving-health-stale-after-refresh`；同源关联：`ad-bug-health-self-scan-lock`；版本 v0.7.5；Lane A。
两项使用同一候选和单一写者，分别保留验收及 CEv1 边界，不新开重复问题。
工作区 `agent-deck.fix.serving-health-stale-after-refresh`，分支 `fix/serving-health-stale-after-refresh`。

## 现象

既有 issue 观察到 Refresh 后菜单栏仍显示扫描正在运行及失败健康检查，而独立 doctor/v1 snapshot 已健康。发布快照携带先前的 scan_lock=warning/lock_live，锁释放或扫描结束不必然产生新数据发布。
关联 issue 还记录扫描发布者把自己持有的 scan.lock 误当作另一个扫描。两项必须联合验证，不能只消除已完成警告而隐藏真正的其他进程锁。

## 根因

候选针对两条路径：doctor 对当前进程锁的判断，以及 Go serving 读取时对持久健康记录中易变 lock 行的重新计算。主控核对 production diff，确认现候选通过 LockCheck、refreshLockHealth 与 current-process token 判断实现上述处理。
当前记录不预先断言 native 直接文件恢复、PID/nonce 身份和全部旧快照路径均已覆盖；这些属于独立评审和验收范围。

## 修复边界

- 正常 Refresh 清除已结束扫描和已解决的 lock 警告；真实其他进程锁仍显示。
- 只更新易变锁状态；其他真实健康错误与摘要计数保持正确。
- 覆盖旧 persisted snapshot、Go helper 及 native 直接文件读取/恢复路径，不把某一 API 读数健康当作 App 已恢复。
- caller-held lock 与 foreign lock 的区分必须根据实际所有权；检查 PID 复用、伪造 token、陈旧元数据、路径别名和缓存旁路。
- 沿用迁入的八文件候选；如发现 native 旁路缺口，先记录 finding，再在同一行为边界内明确扩展相关原生代码与测试。
- 用户随后已明确授权评审/gate通过后按issue提交、推送origin、创建PR，并要求Codex review；无merge/release/deploy/安装授权。旧v0.7.0工作树暂存区保持原样。

## 验证

基线 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。
主控直接核对下列工作文件 blob 与既有来源记录一致；新工作树中是 5 个 tracked 修改和 3 个新测试文件，旧 staged tree 仅是来源身份，不是新候选的暂存状态。

| 路径 | Blob |
| --- | --- |
| cmd/agentdeck/acceptance_test.go | 4127b1c086a1e2a9e9f4cf240e57cf8362cabd84 |
| internal/desktop/desktop.go | acc928bfa3d4ff01b93cf0f7a38b7c71266ec273 |
| internal/desktop/serving_cache.go | c3b81206f5523f3ecbe4aa8a468fb2fe67d21598 |
| internal/desktop/serving_health_test.go | 205c29dac63a9bfeaa0586e3d5b719d50897d3ee |
| internal/doctor/doctor.go | 2ad22bfc21c751dd476fa16afd0c24ad442991f6 |
| internal/doctor/self_lock_test.go | 7cef2b7bb1b48a354f882e9fa3a15ec37d774383 |
| internal/scanruntime/serving_health_test.go | 5a5790ac4e58e4f35a16b1c6bb18d4a2a1caacaf |
| internal/store/store.go | f57710c57221961a2ea7acd2ecc4f16e25800e6b |

现有回归入口：TestServingPublicationIgnoresItsPublishersOwnScanLock、TestServingReadReEvaluatesLockRowsOfAPersistedPublication、TestRoundPublicationDoesNotReportItsOwnScanLock、TestLockRowsDistinguishTheCheckingProcessFromOtherOwners，以及 cmd acceptance fixture。

旧开发交接报告曾称 Go 全仓、相关 race/vet 通过；完整原始命令证据和旧 toolchain 尚未恢复，因此不直接把这些报告注册为当前通过证据。本轮按最小缺口取证；一旦有决定性阻塞，停止广泛验证，待最终修复候选再跑所需范围。
当前环境为 Go1.27.1 darwin/amd64，Xcode/Swift 工具链可用。构建不等于真实 UI 验收；原生验收先绑定隔离 fixture、实际 App 身份、进程与窗口，不能误操作安装 App。
两项 fix gate 均保持未完成，独立评审尚未给出正式结论。

## Review — Round 1

完成门禁：NOT_VERIFIED

2026-11-17 格式补记：主控按本轮被评候选查询两项各自gate；未改变该轮FAIL与既有发现。

方法：冷上下文 Claude/Sonnet 5.5，requested effort=high；只读 source/diff、三个新测试、原生读取路径及隔离 overlay。主控核对 PID-only 判断与空 checks 重建摘要两处生产逻辑，记录正式裁决。被评候选为 `9064b54340037673b3f321f3c277cf68208e5dfe3c680465e836dd96f328c57e`。

### 📋 联合修复评审报告

📊 总体评分：5/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

**V075-SERVING-R1-F1 — 高：PID-only 判定会把非自持锁报告为健康。**
位置：`internal/store/store.go:943` 的 LockOwnedByCurrentProcess，以及 doctor 调用点。
当前函数只核对 token PID；同 PID、未由本进程获取的合法随机 nonce 也会被忽略。主控检查完整 token 的既有锁模型，确认“当前 PID”不等于“实际持有”。Review 的隔离探针以伪造同 PID token 复现 healthy；PID 活着时原锁回收不会移除该文件，因此可出现锁仍阻塞、健康却正常。
风险：PID 复用、遗留或伪造 token 掩盖真实争用；分类与自持判断的两次读取还可能不一致。
💡 修复：按实际成功获取和释放的完整 token、对应路径/文件身份识别自持锁，以同步保护登记生命周期；核对真实获取、释放、同 PID 不同 nonce、复制 token 和 foreign owner，不把仅 PID 相等固定成通过测试。

**V075-SERVING-R1-F2 — 中：空或稀疏 checks 会丢失已有健康摘要。**
位置：`internal/desktop/desktop.go` 的 refreshLockHealth。
函数仅从 checks 重建计数；Available=true 且保留非零 Problems/Warnings/Errors、Checks 为空时，循环没有记录，摘要会被清为 healthy/0。主控由当前代码确认该状态转换；Reviewer 的隔离探针观察相同结果。
风险：旧/稀疏快照内未被当前检查数组表达的真实健康问题被静默隐藏。
💡 修复：保守处理空 checks，只调整实际替换的 lock 行对应的计数差；非锁行及未表示的摘要不得清零。测试覆盖无 checks、非锁错误、稀疏 lock 行及真实 foreign owner，不推导未经定义的全新健康语义。

#### 🟡 建议改进 — 推荐

无其他已验证发现。原生直接恢复路径经 helper snapshot 进入 Go serving 刷新这一正向结论由 Reviewer 源检查支持，但不是实际 App 进程/窗口验收。

#### 🟢 优点

候选已覆盖发布者自锁、持久快照重读、foreign owner 以及摘要一致性的一般路径，未改写持久 serving 文件。原始八文件迁移身份已保留。

#### 📝 总结

`fix:serving-health-stale-after-refresh` 受 F1/F2 阻塞；`fix:health-self-scan-lock` 受 F1 阻塞。两项均 FAIL，保持同一写者与同一候选修复。决定性发现后停止广泛验证；最终相关 Go/race/vet 和隔离原生 Refresh 仍待取证。未操作已安装 App 或无关后台进程。

下一步指令：修复：fix / serving-health-stale-after-refresh / V075-SERVING-R1-F1, V075-SERVING-R1-F2

## 第1轮有界修复

F1修正：只有实际成功获取的完整token、规范路径及相同文件身份才算当前进程自持锁；登记生命周期用mutex保护，释放相应登记，分类与所有权共用一次有界文件观察。仅PID匹配、不同nonce、复制token或不同文件身份不能隐藏争用。

F2修正：没有checks时保留已有健康摘要；对实际表示的lock行只调整计数差，保留非锁问题及checks中没有表示的warnings/errors/problems。旧稀疏数据不被重建为healthy/0。

主控已恢复三项实际红灯：复制token所有权、doctor复制token、空checks清零；旧候选均失败。最终定向检查通过。Developer原先两次全仓命令未形成有效完成证据，已停止该不完整执行；主控随后直接执行 `scripts/run-go-test.sh ./...` 和 `scripts/run-go-test.sh -race ./internal/store ./internal/doctor ./internal/desktop ./internal/scanruntime`，两者均通过。原始 `make vet`、whitespace/diff以及两个Darwin架构构建均退出0，沿用未变范围，不重复未变套件。

主控重新核对当前九文件与上述最终执行的冻结身份一致：

| 路径 | Blob |
| --- | --- |
| cmd/agentdeck/acceptance_test.go | 4127b1c086a1e2a9e9f4cf240e57cf8362cabd84 |
| internal/desktop/desktop.go | 503920f998aa55ba14fc6831f8868807e923a915 |
| internal/desktop/serving_cache.go | c3b81206f5523f3ecbe4aa8a468fb2fe67d21598 |
| internal/desktop/serving_health_test.go | 4ab5d1688cc05e2db2d484bb3d3af1044b783c77 |
| internal/doctor/doctor.go | bfa5984606a9de5ff17a140e7d8340ae2ad2e1e2 |
| internal/doctor/self_lock_test.go | 227ed5b09961c3b0b884bfca2f4f7fade15e75aa |
| internal/scanruntime/serving_health_test.go | 5a5790ac4e58e4f35a16b1c6bb18d4a2a1caacaf |
| internal/store/store.go | 42e3f4f4aac99ca459bccf37c0b5515548265bac |
| internal/store/store_test.go | 92313298ada50f2fb06851f6520462ce4d3573ee |

按既有HEAD+有序九文件+go.mod/go.sum/vendor/modules.txt recipe，候选为 `08b422ce730cb8503939ca372281fe5e4b27191ea756541fdced65ddc091d9fd`；新增store测试明确计入，不以仅tracked diff或旧八文件身份代替。各项CEv1保留同一实现候选但各自criterion映射。

独立复评升级为Opus，仍重点核对稀疏/旧摘要、锁别名/完整所有权和native wire/checksum语义，不能凭已有全仓绿灯关闭finding。原生Refresh的实际App身份、进程/窗口及隔离fixture验收仍未完成；native读取经helper v2 snapshot进入Go链路的源码核对不能替代它。代码已冻结、旧Developer及不完整执行器已确认停止；只读评审不构成通过或交付。

## Review — Round 2

方法：冷独立 Codex，executedModel gpt-6.2-sol、reasoning effort=xhigh（运行时提供），只读。上一位 Opus 复评者的原始报告位于已失效的协调线程，无法回读，只留下"源码建议 PASS"的摘要，不作为正式结论；本轮为重新执行的独立复评。九个候选 blob 与上节一致。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 6/10。V075-SERVING-R1-F1 OPEN：PID-only 判定已修正，登记受 mutex 保护，所有权要求完整 token、路径与文件身份，分类与所有权共用一次读取；但仍有 R2-F1、R2-F2。V075-SERVING-R1-F2 OPEN：空 checks 在 Go 中保留摘要，计数有零下限，重复读取不累计扣减；但仍有 R2-F3、R2-F4。持久 body 的 SHA-256、快照身份与版本先校验，读取路径不回写持久快照；未改 schema、wire 字段、`v1:PID:nonce` token 格式或扫描调度。

- V075-SERVING-R2-F1 Medium：大小写不敏感卷上，经 `CaseRoot` 获取 `scan.lock`、经 `caseroot` 查询，父目录与锁文件为同一文件身份，分类却返回 `lock_live, owned=false`，doctor 显示重试警告。应按文件系统语义比较实际目录项，同时保留完整 token、文件身份及拒绝不同硬链接目录项的要求。
- V075-SERVING-R2-F2 Medium：锁文件在检查与打开之间被原子换成 FIFO 时，健康读取阻塞，直到有人打开 FIFO 写端；64 字节上限只约束打开后的读取。该窗口原存在于分类路径，本候选把它带入 helper v2 缓存恢复。应以 `O_NOFOLLOW|O_NONBLOCK` 打开，再校验描述符类型与身份，异常观察归为 `lock_owner_unknown`，释放路径复用同一有界打开。
- V075-SERVING-R2-F3 Medium：校验和有效的稀疏 fixture 中，摘要为两个问题、两个警告，checks 含同一 `scan_lock` 警告的两份副本，另一警告无对应行；锁消失后得到 `healthy/0/0`，应为 `degraded/1/1`。应每个锁名只观察、核算一次，对歧义旧行保守保留无法归因的计数。属构造的兼容性反例，未观察到历史发布者生成重复行。
- V075-SERVING-R2-F4 Medium：缺失或 `null` 的 checks 经 v2 读取后输出 `"checks":null`，Swift 把 checks 解码为必需数组，原生恢复会在安装快照前失败。应在 helper 输出时把 nil 规范化为 `[]`，保留摘要与持久字节，并补原生解码回归。Go 输出由探针验证，Swift 拒绝来自源码核对。

Reviewer 以 `-overlay` 隔离探针运行 `scripts/run-go-test.sh ./internal/store ./internal/desktop -run '^TestR2'`（退出 1，复现 F1–F3）与 nil checks 探针（退出 1，复现 F4）；未重跑全套、race 或 vet。临时文件与进程均已清理。

源码评审之外仍需的原生 Refresh 验收：在实际候选 App 中绑定 App 路径、bundle ID、PID、菜单栏窗口及嵌入 helper，Home、状态根与 App Group 均指向隔离 fixture；启动恢复时旧快照里已结束的 `scan_lock` 警告消失、非锁错误与摘要保留，恢复前后持久文件字节与 SHA-256 不变，校验和损坏的副本不被采用；点击 Refresh 后扫描者自持锁不变成 `lock_live`，终态后进度与已结束扫描提示消失、无关告警保留；另一隔离进程持锁时仍显示真实争用，释放后再 Refresh 争用消失，重复 Refresh 计数稳定。

进入第 2 轮有界修复，repair_count=2；修复 R2-F1 至 R2-F4 后连同 R1-F1、R1-F2 一并冷复评。

## 第2轮有界修复 — V075-SERVING-R2-F1 至 F4

repair_count=2。Developer 为 Claude Opus 5.5（请求 effort=high，实际 unknown），与 Round 2 的 Codex 评审者不同 harness。在上一轮九文件候选基础上修改，各项先在旧候选上取得真实 red。

- R2-F1：获取锁时登记父目录身份、创建时名称与锁文件身份，不再登记字符串路径；查询先打开父目录、再在其下打开锁文件，两个身份来自同一次解析。自持判定要求完整 token、PID、父目录身份、文件身份相同，且名称完全一致，或只差大小写且该文件链接数为 1，因此"另一个硬链接目录项"仍判为他人。red：`ClassifyLockOwnership(.../caseroot/scan.lock) = lock_live, owned=false`，doctor 对 `caseroot` 给出 `lock_live` 重试警告；新增 `internal/store/lock_entry_test.go`、`internal/doctor/lock_entry_test.go`（卷大小写敏感时 Skip，本机 APFS 实际执行）。
- R2-F2：新增 `openLockEntry`，以 `O_DIRECTORY|O_NONBLOCK` 打开父目录、`O_NOFOLLOW|O_NONBLOCK|O_CLOEXEC` 打开锁文件并 fstat 校验为普通文件；符号链接与非普通文件在分类中归为 `lock_owner_unknown`，在释放中返回 `ErrLockLost`。分类、所有权查询与 `Release` 共用此有界打开。red：分类与释放在目录项被原子换成 FIFO 时各自阻塞超过 2 s（测试自带防挂住）。
- R2-F3：`refreshLockHealth` 每个锁名只调用一次 `LockCheck`、只核算一次并只输出一行；重复旧行只扣除各份中最小的贡献，其余作为无法归因的计数保留，任一旧行状态未知时不扣减。red：重复 `scan_lock` 行的稀疏快照在锁消失后得到 `healthy 0/0/0`，应为 `degraded 1/1/0`；新增 `internal/desktop/serving_health_sparse_test.go`。
- R2-F4：`ReadOrBootstrapServingSnapshot` 返回前把 nil checks 规范化为 `[]`，摘要与持久字节不变。red：`served health.checks = null, want []`。Swift 新增 `testRestoreKeepsSparseHealthServedWithEmptyChecksAndRejectsNullChecks`，经真实 `EmbeddedHelperRunner.readServingSnapshot` 固定原生契约（`[]` 恢复成功，`null` 抛 `DecodingError.valueNotFound`）；Swift 生产代码未改，该测试为契约保护而非 red。

未改 SQLite schema、wire 字段集合、`v1:PID:nonce` token 格式、扫描调度、持久快照字节或 checksum 语义；锁的获取、回收与释放判定规则不变，`go.mod` 不变。F3 会把同名重复锁行合并成一行，`checks` 可能变短但字段集合不变。

Developer 报告的验证：`scripts/run-go-test.sh -count=1 ./internal/store ./internal/doctor ./internal/desktop ./internal/scanruntime ./cmd/agentdeck` 退出 0；同四包 `-race` 退出 0；`go vet` 五包退出 0；`gofmt -l` 改动文件无输出；`git diff --check` 退出 0；隔离 HOME 下 xcodebuild `EmbeddedHelperRunnerTests` 与 `DesktopWireTests` 60 tests 0 failures（排除需 `scripts/test-macos-app.sh` 夹具根的 `testNativeHelperAndGoReceiptDriverShareOwnershipLock`）。

已知剩余风险（未改，属锁协议本身）：获取锁时回收已死进程锁的 `reclaimLockFromDeadProcess` 仍以阻塞 `O_RDWR` 打开并读到 EOF；同目录存在硬链接别名时，用大小写不同的名称查询真锁会保守报为他人持有（只会多报争用，不会隐藏）。

主控核对修复后候选：

| 路径 | Blob |
| --- | --- |
| apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift | 11fd2c6a85310478670f83dfeb52532a9ba9c558 |
| cmd/agentdeck/acceptance_test.go | 4127b1c086a1e2a9e9f4cf240e57cf8362cabd84 |
| internal/desktop/desktop.go | 9eff4d2aa95131d5e5a06cdc4fecc94c2ce87702 |
| internal/desktop/serving_cache.go | eca58bd52ba24a62e273c612ac62ffbf9c581190 |
| internal/desktop/serving_health_sparse_test.go | 9245cc24bb86939b62b9699e9f61458497953118 |
| internal/desktop/serving_health_test.go | 4ab5d1688cc05e2db2d484bb3d3af1044b783c77 |
| internal/doctor/doctor.go | bfa5984606a9de5ff17a140e7d8340ae2ad2e1e2 |
| internal/doctor/lock_entry_test.go | 4d90e79f3b91f7c07de030455c6ea3bfca014407 |
| internal/doctor/self_lock_test.go | 227ed5b09961c3b0b884bfca2f4f7fade15e75aa |
| internal/scanruntime/serving_health_test.go | 5a5790ac4e58e4f35a16b1c6bb18d4a2a1caacaf |
| internal/store/lock_entry_test.go | 1a0cbf40830458f843f0f4a49fe6c6c35bdb72a8 |
| internal/store/store.go | 15d51ee73c3fdb58d593320bffbdf604b25bc7b9 |
| internal/store/store_test.go | 92313298ada50f2fb06851f6520462ce4d3573ee |

待冷独立复评；原生 Refresh 实机验收仍未执行。

## Review — Round 3

方法：冷独立 Codex，executedModel gpt-6.2-astra、reasoning effort=xhigh，只读；以 `-overlay` 与隔离 HOME/TMPDIR/GOCACHE 运行探针，系统沙箱禁止写仓库与访问真实状态。十三个候选 blob 与「第2轮有界修复」表一致。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 7/10。V075-SERVING-R2-F2 CLOSED：健康分类、自持查询与 `Release` 均经 `openLockEntry`，末端 `O_NOFOLLOW|O_NONBLOCK` 后对描述符检查普通文件，分类与释放单次最多读 65 字节，两个 FIFO 竞争测试 `-race` 通过；`reclaimLockFromDeadProcess` 的阻塞打开只在获取锁遇 `EEXIST` 的回收分支，不在本次健康读取链。V075-SERVING-R2-F4 CLOSED：缓存命中先验证原始 body 的 SHA-256 再于内存把 nil 转为 `[]`，缺失与 `null` 均通过且持久文件摘要不变；缓存缺失或损坏时 `BuildToday` 与完整 `Build` 均初始化 `[]`；Swift 契约测试经真实 `readServingSnapshot` 解码。V075-SERVING-R1-F1、R1-F2、R2-F1、R2-F3 仍 OPEN，原因见下列新发现；原 `CaseRoot/caseroot` 与 `2/2/0 → 1/1/0` 反例本身已通过。未发现 schema、wire 字段集合、token 格式或扫描调度改动；释放的错误分类与登记生命周期有行为变化。

- V075-SERVING-R3-F1 Medium：获取时先按路径创建文件、取文件身份并关闭，随后才按路径 `Stat` 父目录；父目录在两次解析之间更名或被替换时登记会混合两次解析的身份，查询端的 `openat` 无法修正。在大小写敏感卷上，持有期间仅改末端大小写的更名使新名仍满足 token、父目录、inode、`EqualFold` 与 `nlink==1`，查询新名返回 `owned=true`，而释放时查旧名得到 `ENOENT`。普通协议下他人新获取的锁 token 不同，未证明会隐藏他人锁；风险集中在保留原身份的更名、只剩一个名字的硬链接，以及 inode 复用且完整 token 相同。Go 在该平台的 `SameFile` 只比较设备号与 inode；未实测 inode 复用或大小写敏感卷。修复：以同一父目录描述符完成创建与登记；名字不完全相同时验证创建名仍从该目录解析到当前文件；无法证明自持时保留争用提示。
- V075-SERVING-R3-F2 Medium：checksum 有效的兼容 fixture（摘要 `1/1/0`，一条非锁 `warning` 与两条旧 `scan_lock/warning`，锁已消失）经真实 serving 读回 `blocked_tasks/warning, scan_lock/ok`、状态 `degraded`，摘要却为 `0/0/0`。属旧行与摘要归属有歧义的构造输入，未观察到当前发布者生成。另外计数先做 `int` 加法再取下限，接近 `MaxInt` 时不能保证非负。修复：先为保留下来的已知非锁 warning/error 预留摘要下限，再从剩余可归因计数中扣旧锁贡献并保留未由行表示的余额；在接受旧摘要的边界处理整数溢出。
- V075-SERVING-R3-F3 Medium：锁目录项变成 Unix socket 后，分类正确返回 `lock_owner_unknown`，但 Darwin 在 `openat` 阶段即返回 `operation not supported on socket`、到不了 `fstat`；实测 `Release` 返回该原始错误且自持登记保留（`registry retained=true`）。修复：打开失败时在同一父目录下不跟随链接确认类型，已是非普通文件则返回 `ErrLockLost` 并注销登记；权限或临时 I/O 错误保留可重试语义。

Reviewer 运行：13 个 blob 核对退出 0；`scripts/run-go-test.sh -race -overlay=… ./internal/store ./internal/doctor ./internal/desktop` 定向探针退出 1（doctor 两项与 desktop 四项既有回归通过，R3-F2 探针失败）；store 定向退出 1（四项既有回归通过，R3-F3 探针失败）；所选测试无 data race；临时文件与进程已清理。未扩大到五包全套、全量 race、vet 或 XCTest。

原生 Refresh 实机仍待验证：绑定候选 App 路径、bundle ID、PID、菜单栏窗口与嵌入 helper，HOME、状态根、App Group 均指向隔离 fixture；冷启动恢复含已结束 `scan_lock` 警告与非锁问题的旧快照，菜单栏提示、健康详情与摘要一致，缺失或 `null` checks 的非零摘要可恢复、损坏 checksum 被拒，恢复窗口内持久文件字节与 SHA-256 不变；实际点击 Refresh，扫描期间自持锁不出现 `lock_live`，终态后进度与"扫描正在运行"消失、无关警告与错误仍在；另一隔离进程持锁时显示真实争用，释放后再 Refresh 只清除该争用，连续两次刷新详情、摘要与终态稳定。

进入第 3 轮有界修复，repair_count=3。

## 第3轮有界修复 — V075-SERVING-R3-F1 至 F3

Claude Opus 5.5 在原 Worker 线程接续，实际 effort 未暴露。主控核对 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781` 和 13 个文件。变更四个 blob：`internal/desktop/desktop.go` 为 `ac1e3c39f97920c249c116d1f83781a1b3c56d7f`，`internal/desktop/serving_health_sparse_test.go` 为 `73c0fb5b29a8d5bdb64d5714a4278b0db0ed359e`，`internal/store/lock_entry_test.go` 为 `f7025c6d22627f24fb2223ed12307b902c2fffa0`，`internal/store/store.go` 为 `1a68c5a1d84626f7539163d130929f2e93e0aef8`。其余九个与第 2 轮相同。排序后的 13 个 `路径 blob` 行 SHA-256 为 `31ef8bca605987115ed98479fc8019f03459554390c5f40c17dac328ba43ee12`。

- **R3-F1**：从同一父目录描述符排他创建和登记。临时替换父目录并硬链接时旧候选把真实锁判成他人、把替换目录别名判成自己，测试退出 1；修复后退出 0。大小写敏感卷上的改名用例受当前 APFS 行为限制而跳过。创建钩子默认为 nil。
- **R3-F2**：摘要至少覆盖可见的非锁及当前锁问题，旧摘要调整采用非负饱和。重复旧锁遮蔽警告、接近整数上限、负数三个反例在旧候选退出 1，修复后退出 0。
- **R3-F3**：同目录下打开失败后以不跟随链接的方式核对末端类型。socket 已替换时返回 `ErrLockLost` 并注销。旧候选返回 `operation not supported on socket` 且保留登记，退出 1。新候选通过，普通文件权限错误的可重试保护用例亦通过。

Developer 在隔离状态下运行四包 race、五包 vet、格式和 diff 检查均退出 0。五包常规 Go 测试首跑退出 1，doctor 的 LaunchServices 路径超时及 cmd 分页的临时路径折行各在后续对应整包运行退出 0。首跑失败没有在旧候选做相同环境对照，应保留为观察。原生 Refresh 待以最终候选执行。状态目录现在需能只读打开，常规受管 0700 状态根满足。大小写敏感卷复用、锁回收路径及临时测试钩子由冷复评判断适用性。

主控在同一候选运行 store/desktop 定向测试。目录登记、socket、可重试错误及三个摘要反例通过。大小写敏感卷上的改名用例跳过。

## Review — Round 4

冷独立 OpenCode `openai/gpt-6.2-astra`，实际 variant 未暴露。精确候选仍为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、十三文件指纹 `31ef8bca605987115ed98479fc8019f03459554390c5f40c17dac328ba43ee12`。主控核对候选与释放、摘要分支源码。Reviewer 以 CodeGraph 定位后，通过隔离 Go race 覆盖探针检查。

评审结论：FAIL

完成门禁：NOT_VERIFIED

同一候选的两个 WorkUnit 裁决均为 FAIL。CEv1 对各自本轮精确状态返回 `missing_target_state`。Reviewer 检查清单为 49/54；两处有决定性可运行反例。

- **V075-SERVING-R4-F1（待修，阻断两项）**：原父目录被移走后，替换目录中的同 inode 硬链接被分类为非自持。随后 `Release` 实测返回 nil，删掉替换目录别名，原目录锁仍保留且登记已注销。`releaseOnce` 关闭所打开的父目录并按路径重新解析删除，只校验了文件身份和 token。释放必须验证获取时的原目录项，证明不足时保留替换项并返回准确结果。反例中的同 inode 别名与原锁共享 token。
- **V075-SERVING-R4-F2（待修，阻断 serving-health）**：checksum 有效的旧快照为 `degraded`、`healthy=false`、摘要 `0/0/0`、两条旧锁 warning，且当前无锁。连续两次经真实 serving reader 都得到一条 `scan_lock/ok`，同一 publication 仍显示 `degraded`。计数相等的提前返回跳过状态更新。按实际更新行和保留摘要更新状态，同时保全无行的旧摘要及未表示问题。
- **R2-F1 仍 OPEN（保守误报）**：本机大小写不敏感卷有第二个硬链接时，原名 `scan.lock` 判自持、同目录项的 `SCAN.LOCK` 判他人；真别名正确判他人。doctor 以固定小写读取，常规路径未复现这项误报。

R1-F1 与 R3-F1 因释放路径 OPEN，R1-F2 因状态字段 OPEN。R2-F2 FIFO、R2-F3 原扣数反例、R2-F4 空 checks、R3-F2 三个算术反例及 R3-F3 socket 已 CLOSED。独立进程真实持扫描锁时保留 `warning/lock_live/retry`，释放后返回 `ok`。0300 目录的获取行为有变化，受管状态根 0700 的获取、回收和释放通过对照。未发现 schema、wire、token 或扫描调度变化。

定向 `-race -overlay` 退出 1，仅释放反例和零摘要状态反例失败，doctor 通过，未报告 race。原生测试仍需等待修复后候选。当前累计三轮有界修复，按编排预算暂停新的修复派发。

## 第四轮修复交接

2026-11-18，用户授权继续修复，并指定 Codex `gpt-6.2-astra` 的 `max` 档。T3 确认该执行模型及配置，累计修复轮次为四。HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，输入 13 文件 `31ef8bca605987115ed98479fc8019f03459554390c5f40c17dac328ba43ee12`，输出 15 文件 `6b68f2e42698861001a52bda407a66c6f68718f6ff1b4ba0db744f5e69a8454a`。主控复算输出，并核对原始红灯、定向 race 与六包日志。

- **V075-SERVING-R4-F1** 原候选错误删除替换目录的同 inode 别名并返回成功。新实现经获取时保留的父目录描述符核对并删除原锁，别名保留。定向测试也覆盖新持有者、socket、FIFO 及权限恢复后重试。
- **V075-SERVING-R4-F2** 原候选两次读出 `scan_lock/ok` 后仍报告 `degraded`。新候选在同一 publication 的两次读取中均报告 `healthy`，持久快照和 publication ID 保持一致。稀疏摘要、未表示问题和非锁警告的回归通过。
- **R2-F1** Darwin 通过已打开的目录描述符查询大小写策略。在当前大小写不敏感卷，两种不同拼写的本名均识别为自持，另一个硬链接仍识别为非自持。大小写敏感卷上的用例受实际测试卷条件限制跳过。策略未知及非 ASCII 名称仍保守处理。

修复前定向退出 1，修复后定向 race 23 项通过、1 项跳过。六包测试 819 项通过、8 项跳过、0 项失败。对应 vet、宿主 CLI build、Linux/amd64 交叉构建、格式和差异检查通过。`fstatat` 与 `unlinkat` 之间仍受常规系统调用的叶项竞态限制，需在独立复评中核对其与持锁及受管目录契约的关系。

本轮交接给 Round 5 独立复评，并以隔离 App 原生 Refresh 验收。

## Review — Round 5

评审结论：PASS

完成门禁：VERIFIED

2026-11-18，独立 Claude Sonnet 5.5 复评核对 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781` 与 15 文件候选 `6b68f2e42698861001a52bda407a66c6f68718f6ff1b4ba0db744f5e69a8454a`。`ad-bug-serving-health-stale-after-refresh` 与 `ad-bug-health-self-scan-lock` 两项均 PASS。R4-F1、R4-F2、R2-F1 及相关旧发现关闭。独立 race 探针覆盖获取、释放、并发分类、目录与叶项替换、同 token 归属、重复读取及稀疏摘要，均通过。

`fstatat` 和 `unlinkat` 的叶项间隙作为锁协议边界保留。协议内获取使用排他创建，回收要求持有者进程已退出。活锁仍受退出与回收规则保护。释放使用获取时的目录描述符，保持目录绑定。协议外直接替换活锁已破坏互斥，系统调用也无按 inode 比对后原子删除的接口。大小写敏感卷改名用例在当前大小写不敏感卷跳过，继续保留平台限制。

主控在使用独立状态目录和 App Group 的签名 Debug App 中实际触发 Refresh。外部 Go 进程持有扫描锁时，App 显示对应警告。释放后连续两次 Refresh 均清除该警告。使用同一签名 helper 恢复预置的旧警告快照时保留原文件摘要，校验和损坏的快照被拒绝。嵌入 helper 的 SHA-256 为 `cdd3dce3dc99f1e0de5f91d3b41a19f6fa653fe7e13dd1e546f9d71951a4f1a9`。

两项证据均绑定 15 文件候选 `6b68f2e42698861001a52bda407a66c6f68718f6ff1b4ba0db744f5e69a8454a`。`fix:serving-health-stale-after-refresh` 的失败先行、回归、独立复评及原生 Refresh 为 4/4。`fix:health-self-scan-lock` 的失败先行、回归及独立复评为 3/3。
