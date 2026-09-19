---
status: active
topic: subscription-quota
subject: quota-domain
---

# Quota Domain Review

## Round 1 — 2026-09-10

## 📋 quota-domain 实现评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**QD-R1-F1 — 高：存储按到达顺序覆盖，C5 的来源优先级在持久层失效，且会伪造 reset。**

位置：`internal/quota/store.go:106-116` 的 upsert 无条件覆盖同键行；
`internal/quota/state.go:56-71` 的 `SelectObservation` 没有被 `Store` 使用；
`internal/quota/store.go:178-208` 的 `Windows` 不返回 `source` 与 `observed_at`。

- 行为风险：Claude 两条路由写同一个 `(client, window_key)` 行（C8）。
  等龄的 prose 读数晚于 status-line 到达时会替换其 `source`；较旧的读数晚到时
  会把行回退到旧时刻，且若其百分比较低会被当作 reset，写入伪造的
  `observed_reset_at`（任务 5 的 reset 通知也由此驱动）。由于读路径不返回来源和
  观测时刻，C5 要求的"来源原样到达 payload"也无法从存储得到。
- 证据：`TestProbeEqualAgeProseOverwritesStatusLine` 输出
  `stored source=claude_prose`；
  `TestProbeOlderObservationArrivingLaterRegressesAndFakesReset` 输出
  `stored observed_at=2026-09-10T10:00:00Z resetObserved=true`（先记录 10:10 的
  50%，后记录 10:00 的 45%）。契约见 `architecture.md:238-243`、
  `tasks.md:129-131`。
- 测试保护缺口：`SelectObservation` 只作为纯函数测试，存储路径没有乱序与等龄用例。

💡 修复范围：`Record` 对同键已存观测应用 C5——比已存更旧的读数不覆盖，等龄时保留
status-line；只有被接受的读数才参与 `ApplyObservation`。持久化并在读路径返回每个
窗口的 `source` 与 `observed_at`。补充"等龄 prose 晚到""旧读数晚到"两个存储用例。

**QD-R1-F2 — 中：跨路由精度差异会被判为 reset。**

位置：`internal/quota/state.go:45` 用严格 `<` 比较任意来源的前后两次读数。

- 行为风险：prose 路由只给整数百分比（`requirements.md:105-106`），status-line
  载荷是 JSON 数值（`architecture.md:146-149` 样例为 `0.0`），两者共用同一 Claude 键。
  status-line 的小数读数之后紧跟 prose 的取整读数，就会产生伪造的
  `observed_reset_at` 和 reset 通知。真实 status-line 是否带小数未在需求中记录；
  缺陷在于比较规则对文档已写明的表示差异没有任何防护。
- 证据：`TestProbeCrossRoutePrecisionFakesReset`，22.4% 之后 5 分钟记录 22%，
  输出 `resetObserved=true`。

💡 修复范围：来源不同时，只有下降达到较粗路由的粒度（至少 1 个百分点）才判定为
reset；同来源保持严格比较。在代码注释中引用 C6 说明该规则，并加测试：跨路由
22.4→22 不是 reset，跨路由 80→5 仍是 reset。若修复方认为该规则需要修改
`architecture.md`，应停下来交由操作者决定，不得自行改动契约。

**QD-R1-F3 — 中：`Windows` 按 key 字母序返回，丢失厂商顺序且无法恢复。**

位置：`internal/quota/store.go:181` 的 `ORDER BY window_key`；表中没有保存顺序的列。

- 行为风险：C11 要求 `windows[]` 保持厂商顺序（主限额在前，再到按模型的限额），
  并明确拒绝排序（`architecture.md:495-500`、`tasks.md:281`）。存储是唯一的
  持久来源，按字母序会把主限额的 `codex_secondary` 排到按模型限额之后，
  任务 6 无从恢复。
- 证据：`TestProbeWindowsLosesVendorOrder` 按
  `[codex codex_secondary codex_bengalfox codex_bengalfox_secondary]` 记录，
  返回 `[codex codex_bengalfox codex_bengalfox_secondary codex_secondary]`。

💡 修复范围：随每个窗口持久化其厂商序号，读路径按序号排序；用上面的四键顺序加测试。

**QD-R1-F4 — 中：没有持久化每个客户端的 envelope（C7）。**

位置：`internal/quota/store.go:25-43` 只有 `quota_windows`；`Envelope`
（`internal/quota/model.go:205-228`）没有任何存取路径。

- 行为风险：C7 规定存储"每个 key 的最新观测，加上每个客户端最新的 envelope"
  （`architecture.md:329-330`），任务 1 拥有持久化（`tasks.md:112-113`），后续任务的
  文件清单都不含它。缺少后，`plan`、`reset_allowance`（remaining 与 credits）、
  `failure`、`applicable` 以及客户端级 `source`/`observed_at` 都无处保存。C5 要求
  区分的"从未观测 / 失败 / 过期"三种状态（`architecture.md:280-283`）无法从存储
  得出：失败探测无法在不抹掉窗口的前提下记录原因；独立进程运行的
  `agentdeck quota` 也拿不到计划名和重置额度。
- 证据：源码审读；包内没有 envelope 表、写入或读取函数。

💡 修复范围：增加每客户端一行的 envelope 持久化（写入与读取）；`account_id` 仍只作
隔离键，不进入任何读出字段（C8）。Codex 账号变更时同时丢弃该客户端的 envelope。
加测试：失败读数保留窗口并记录原因；账号变更清空 envelope。

**QD-R1-F5 — 中：schema 绕开项目的版本化迁移。**

位置：`internal/quota/store.go:25-43` 用 `CREATE TABLE IF NOT EXISTS` 自建表。

- 行为风险：仓库所有生产表都在 `internal/store/migrations.go` 的版本化迁移中
  （迁移列表见 `:19`，执行入口 `bootstrapAndMigrate` 见 `:371`）。
  `IF NOT EXISTS` 无法演进已发布的表，而 F1、F3、F4 的修复本身就要求改 schema；
  一旦按现状发布，后续变更在用户数据库上会静默不生效。`tasks.md` 任务 2–7 的文件
  清单也没有任何任务负责接入。
- 证据：`rg 'CREATE TABLE' --type go internal` 除本包外只命中
  `internal/store/migrations.go` 和一个测试夹具。

💡 修复范围：把 quota 表注册为 `internal/store` 的下一个版本化迁移，删除
`EnsureSchema`。这会触及任务 1 声明文件清单（`tasks.md:110-111`）之外的
`internal/store`；修复开始前须请操作者确认修订该文件清单，不得无声扩大范围。

### 🟡 改进建议 — 建议处理

以下同样是本目标的未关闭发现，PASS 前必须关闭。

**QD-R1-F6 — 低：Claude 观测的 `account_id` 没有被结构性清空，对应测试不检验隔离行为。**

位置：`internal/quota/store.go:78`、`:117` 按调用方传入的 `AccountID` 建键；
`internal/quota/store_test.go:141-158`。

- 证据：`TestProbeClaudeAccountIDIsNotStripped` 以 `account_id=someone` 与空值
  各记录一次 `five_hour`，`Windows` 返回 2 行；C8 规定 Claude 键没有账号分量
  （`architecture.md:355-357`）。现有测试只记录一行并断言 1 行，有无账号隔离都会通过。

💡 修复范围：`Record` 对 Claude 强制 `AccountID = ""`（或拒绝非空值）；测试改为
带/不带账号各记录一次并断言只有一行。

**QD-R1-F7 — 低：`Envelope.AttributionConfirmed` 是可随意赋值的字段。**

位置：`internal/quota/model.go:215`；测试只覆盖函数
`internal/quota/model_test.go:29-36`。

- 证据：零值为 `false`，组装 Codex envelope 时漏赋值不会报错，却会错误声明
  "归属未确认"；注释写"always AttributionConfirmed(Client)"，但这只是约定。
  `tasks.md:126-128`、`:135-136` 要求每个 payload 都携带正确值。

💡 修复范围：改为由 `Client` 推导的方法（或唯一构造函数），并在 `Envelope` 上对两个
客户端各加一条测试。

**QD-R1-F8 — 低：读取已存 `observed_reset_at` 时吞掉错误，导致数据被静默清空。**

位置：`internal/quota/store.go:130-142`；对比同一列在 `Windows` 中会返回解析错误
（`internal/quota/store.go:199-202`）。

- 证据：`TestProbeCorruptStickyResetIsSilentlyErased` 把该列改成不可解析的值后做一次
  非 reset 记录，得到 `err=<nil> sticky.IsZero=true stored=""`。

💡 修复范围：在 `queryStored` 中一并读取该列，只查询一次并传播扫描/解析错误；加测试。

### 🟢 做得好的方面

- 三种 reset 语义落在三个独立字段上，类型层面不会混淆；原因集合是封闭的，并提供
  `Valid()`。
- `AllowedAge` 与 `Stale` 和 C5 公式、表格一致；测试覆盖两种窗口长度、5/15/30m
  三个间隔、下限接管的 30m 情形，以及阈值两侧各一分钟。
- Codex 账号变更在同一事务内先丢弃再写入；`ApplyObservation` 只在下降时判定 reset，
  不依据 `resets_at` 已过。
- 测试确定、使用内存 SQLite，不接触厂商，也没有凭据或传输代码。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与实现，实现由另一个会话
完成并交接。Method：单代理正式代码评审——逐条对照 `tasks.md` 任务 1 与
`architecture.md` C5–C8、C11，审读全部源码，并用 Go `-overlay` 注入复现测试；
未使用子代理，未修改生产代码、仓库测试或配置。

Scope：`internal/quota/` 下 6 个新文件，以及它们与任务 2–6 的衔接契约。适配器、
调度、告警、wire 与界面属于后续任务，不在本轮。

Reviewed state：workspace `.worktrees/subscription-quota`，branch
`feature/subscription-quota`，HEAD `9d96ee40cf7be17ad29ffe84b6ffdbd0a30091b3`。
候选 manifest：`internal/quota/` 按路径排序的 `<blob> <path>` 六行，其 SHA-256 为
`21605fbf09f0c9c5398a6cd395d3d6aeb657166767776427234e703ab944df49`。
关键 blob：`store.go` `2c99a481f1d48dcab3c462cd9f53709ecc4b71a0`、
`state.go` `051c366c65d293940fbc9fb707b9f2cfa4121d4d`、
`model.go` `75474ac7fe2fda46246fe507be522aebb49eb8a2`。
评审开始时 `tasks.md` blob 为 `a47772d64c07b61b60b5ca48b2b05aa10a83f5a6`（含实现方的
Dev 勾选与交接文字）。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/quota/...`：
  `ok`，20 个用例 PASS、0 FAIL；`go vet -mod=vendor ./internal/quota/` 通过；
  `gofmt -l internal/quota` 无输出；`make check-whitespace` exit 0；`git diff --check` 通过。
  这些结果证明现有测试通过，不能证明上述场景正确。
- 复现命令：`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1
  -overlay <scratchpad>/overlay.json -run 'TestProbe' -v ./internal/quota/`，exit 0，
  输出 7 条 `REPRO` 行，内容即上文各项证据。复现测试以 `t.Logf` 报告观测值，
  不以失败退出。本轮用它们测量行为，未把它们加入仓库。
- 复现材料保存在本会话 scratchpad（会话结束后可能被清理，修复时可按上文场景重建）：
  `review_probe_test.go` SHA-256
  `739e733649c7a4bdff1469cc4c46a00ac01781cc76b21b624c1d70eb83b871af`；
  日志 SHA-256 `fddca97b51071a8d632cd7fc9ce56d000cb8d6cda562c3ed92a1418f68d1b669`。
- 未运行全仓测试、race 或构建：包尚未被任何生产代码引用，而且已有决定性反例。

完成门禁：NOT_VERIFIED。CEv1 provider（Neo4j MCP）可用，但命名空间内没有
`subscription-quota` 任务级 WorkUnit，只有 6 个文档级 WorkUnit，属于
`missing_work_unit` 数据缺口。本轮没有写入 CEv1：协议规定不得在评审结论之后补造
验收标准，也没有使用文件回退。修复进入时须先按 `tasks.md` 任务 1 的 Verification
建立 `subscription-quota:quota-domain` WorkUnit 及其必需标准，再绑定修复后的内容状态。

Task 状态：Dev 保留实现方的勾选，Review 未勾选；Beads `ad-sq-quota-domain-dev`
退回 `in_progress` 等待修复。本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/quota-domain.md / QD-R1-F1 QD-R1-F2 QD-R1-F3 QD-R1-F4 QD-R1-F5 QD-R1-F6 QD-R1-F7 QD-R1-F8

## Round 2 — 2026-09-11

## 📋 quota-domain 修复复评

📊 总体评分：6/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**QD-R2-F1 — 高；新增（取代 QD-R1-F4 中"账号变更同时丢弃 envelope"的部分）：账号隔离只由窗口写入触发，envelope 写入不带账号。**

位置：`internal/quota/store.go:57-73` 的丢弃只在 `Record` 中执行；
`internal/quota/store.go:213-239` 的 `PutEnvelope` 不做任何账号检查；
`internal/quota/model.go:252-270` 的 `EnvelopeRecord` 没有账号字段。

- 处置：新增。
- 行为风险：
  - 同一次探测中，新账号若先写 envelope、后写窗口，新账号刚写入的 envelope
    会被紧随其后的丢弃删掉，客户端变成"有窗口、无 envelope"。
  - 新账号的探测若合法但没有 `rateLimits`——C2 规定这是 `not_reported`
    而不是失败（`architecture.md:135-138`）——就不会有窗口写入，丢弃永远不触发，
    旧账号的窗口会挂在新账号的 envelope 下继续显示，违反 C8
    （`architecture.md:342-345`）。
  - 结果依赖调用方的写入顺序，下一个任务无法从 API 看出这条约束。
- 证据：`TestProbeR2EnvelopeBeforeWindowsForNewAccountIsDeleted` 输出
  `envelope ok=false plan=""`；`TestProbeR2NewAccountWithoutWindowsKeepsPreviousAccountWindows`
  输出 `account B envelope (plan="plan-B") without windows -> account A windows still stored rows=1 used=80`。
- 测试保护缺口：`TestStoreAccountChangeDiscardsRatherThanMerges` 只覆盖
  "先写窗口"一种顺序，也没有覆盖无窗口的新账号。

💡 修复范围：让账号成为整个客户端写入的隔离键，不再依赖窗口写入。例如
`EnvelopeRecord` 携带 `AccountID`（只作隔离键，不出现在任何读出字段中，C8），
并由 `PutEnvelope` 执行与 `Record` 相同的丢弃检查；或提供一个把 envelope 与该次
全部窗口放在同一事务里写入的入口。补测试：先写 envelope 后写窗口、先写窗口后写
envelope、新账号无窗口，三种情况下旧账号的窗口和 envelope 都被丢弃，新账号的
数据都保留。

**QD-R2-F2 — 中；新增（取代 QD-R1-F4 中"失败读数保留窗口"的部分）：只写失败原因的 envelope 会清空最后一次成功的客户端状态。**

位置：`internal/quota/store.go:213-239` 的 `PutEnvelope` 整行覆盖；修复自己的
测试 `internal/quota/store_test.go:384` 正是这种写法。

- 处置：新增。
- 行为风险：一次暂时的探测失败会把 `applicable` 置为 false 且没有原因、清空
  `plan`、`reset_allowance` 和 credits、清空 `source`，并把 `observed_at` 置为零值；
  窗口却还保留着。界面可能把 Codex 卡片整个显示为"不适用"；缺失的字段也
  不带原因，违反 C6 的封闭原因集合（`architecture.md:321-325`）；这也和 C9
  "退避中的客户端仍显示最后的数字和真实时长"（`architecture.md:437-439`）相矛盾。
- 证据：`TestProbeR2FailureOnlyEnvelopeWriteErasesLastKnownFields` 先写入
  plan=pro、remaining=3、1 条 credit，再写入只含 `Failure` 的 envelope，输出
  `windows=1 applicable=false plan="" planReason="" remaining=0 hasRemaining=false credits=0 source="" observedAtZero=true failure="probe_failed"`。

💡 修复范围：提供只更新失败原因（及其尝试时刻）的写入，保留最后一次成功写入
的其余字段；成功的 `PutEnvelope` 清除失败原因。修正现有测试使用这个入口，并断言
失败后 plan、重置额度、applicable 与 source 都保持不变。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **QD-R1-F1 → CLOSED。** `Accept`（`internal/quota/state.go:43-52`）在持久化边界
  执行 C5 规则，`Record` 拒绝较旧读数和输掉等龄比较的读数，窗口读出时带
  `Source`/`ObservedAt`。复现：等龄 prose 晚到后，存储的来源仍是
  `claude_status_line`；较旧读数晚到时 `accepted=false reset=false`，存储保持
  10:10 的 50%。
- **QD-R1-F2 → CLOSED。** 来源不同时需下降至少 1 个百分点才判为 reset
  （`internal/quota/state.go:61`、`:80-85`）。复现 22.4→22 输出 `reset=false`；
  已有测试覆盖跨路由 80→5 仍判为 reset，以及同来源的小幅下降。
- **QD-R1-F3 → CLOSED。** 每个窗口都持久化 `vendor_order`，读出时按它排序。
  复现输出与厂商顺序一致。
- **QD-R1-F4 → SUPERSEDED by QD-R2-F1、QD-R2-F2。** envelope 持久化已经存在，
  失败时窗口也得到保留，原发现所说的"没有持久化"已不成立；但修复中新写的
  账号丢弃与失败写入各引入一个缺陷，分别由上面两项承接。
- **QD-R1-F5 → CLOSED。** 表迁入 `internal/store/migrations.go` 版本 24，
  `CurrentSchemaVersion` 升为 24；按任务 1 Files 说明，这次扩大范围已获操作者批准。
  `main` 仍是 23，组装时不会撞上版本号。受影响的升级测试和两个 fixture 只改了
  各自对应的那一处。
- **QD-R1-F6 → CLOSED。** `Record` 对 Claude 强制使用空账号；复现输出 `rows=1`，
  新测试会随隔离行为变化而失败。
- **QD-R1-F7 → CLOSED。** 改为 `Envelope.AttributionConfirmed()` 方法，字段已删除，
  两个客户端各有测试。
- **QD-R1-F8 → CLOSED。** `queryStored` 一次查询读出该列并传播解析错误；复现输出
  `err!=nil=true`，存储值没有被抹掉。
- 每条已关闭的发现都配有专门的回归测试；存储测试改为走真实的版本化迁移建库。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复，修复由另一个会话
完成并交接。Method：单代理正式复评——逐条核对 Round 1 八项发现，审读修复后的
全部源码与 diff，并用 Go `-overlay` 注入 Round 2 复现测试；未使用子代理，未修改
生产代码、仓库测试或配置。

Scope：`internal/quota/` 6 个文件；`internal/store/migrations.go`、
`internal/store/store.go`；`cmd/agentdeck/main_test.go`；两个 desktop fixture；
`tasks.md` 任务 1 的 Files 说明。适配器、调度、告警、wire 与界面仍属后续任务。

逐项处置：F1、F2、F3、F5、F6、F7、F8 CLOSED；F4 SUPERSEDED，由 QD-R2-F1、
QD-R2-F2 承接；新增 QD-R2-F1（高）、QD-R2-F2（中）。

Reviewed state：branch `feature/subscription-quota`，HEAD
`9d96ee40cf7be17ad29ffe84b6ffdbd0a30091b3`。manifest 为 `head=<HEAD>` 一行，加上
12 个路径按字母排序的 `<blob> <path>` 行，覆盖上述 Scope 中的产品与测试文件及
`tasks.md`；其 SHA-256 为
`e6d52cab740da501059cc77438f958280a33b245d6fa656c485bcd3d7afdf994`。
关键 blob：`store.go` `c435231609aab49d1770a889c5ddd9ad75c22391`、
`state.go` `459960dcc7f0bda97a5e4f521a3854f1a8f9c668`、
`model.go` `2e96673a5564ac0abdcde0c8cfd8f63c85572f0c`、
`migrations.go` `8da6021d3fbebe4002f7bef89f25b57a793f37ac`。
复现和全仓测试运行时，只有 `tasks.md` 与本轮同步前不同（交接文字），其余 11 个
blob 完全相同，因此证据适用于最终状态。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，
  21 个包 ok，0 个 `--- FAIL`；日志 SHA-256
  `73105d007af77528df41568324e748471904ac32a859ef129137bad6d76c441a`。
  schema 变更属于 L2 持久状态，按项目规则需要跑全仓。
- `go vet -mod=vendor ./internal/quota/ ./internal/store/ ./cmd/agentdeck/` 通过。
  `gofmt -l` 只列出 `cmd/agentdeck/usage_stats_viewer_test.go`，该文件与 HEAD
  相同，不属于本任务。
- Round 2 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-r2.json
  -run 'TestProbeR2' -v ./internal/quota/`，exit 0，输出 9 条 `REPRO` 行。
  前 6 条确认 Round 1 场景已修复，后 3 条即 QD-R2-F1、QD-R2-F2 的证据。
  probe SHA-256 `56c654bff0b240433c0c873e5b3df77cf51eac5e0ca5e146446411b2703b36fd`；
  日志 SHA-256 `354a837cc4afb2c2e6810e860cd4c2e5f8069a3e0fff29f533339d2660aad239`。
  材料保存在本会话 scratchpad，会话结束后可能被清理，可按上文场景重建。
- 未运行 race：包内没有并发代码，跨进程写入属于任务 4 的调度范围。

完成门禁：FAILED。Round 1 所说的 WorkUnit 缺失已在本轮进入时补齐：依据
`tasks.md` 任务 1 在本结论之前已声明的内容，建立
`urn:agent-deck:work-unit:subscription-quota:quota-domain` 及三个必需标准
`domain-contract`、`verification`、`review`，并绑定内容状态
`urn:agent-deck:content-state:subscription-quota:quota-domain:9d96ee4:e6d52cab740d`。
其中 `verification` 记为 pass（目标测试与全仓测试通过），`domain-contract` 与
`review` 记为 fail（QD-R2-F1、QD-R2-F2 以及本轮 FAIL）。

Task 状态：Dev 保留勾选，Review 未勾选；Beads `ad-sq-quota-domain-dev` 退回
`in_progress` 等待修复。本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/quota-domain.md / QD-R2-F1 QD-R2-F2

## Round 3 — 2026-09-11

## 📋 quota-domain 第二次修复复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**QD-R3-F2 — 中；新增：原始 `account_id` 存进核心库，并随便携备份导出。**

位置：`internal/store/migrations.go:228`、`:245` 两张 quota 表都以明文保存
`account_id`；`internal/backup/backup.go:104-113` 把整个核心库快照
（`agentdeck.sqlite3`）读入备份归档，不剔除任何表。

- 处置：新增。这个问题 Round 1 就已存在，但我在 Round 1 没有指出；QD-R1-F5 的修复方向
  （迁入 `internal/store`）使这一列落进了会被备份的核心库。
- 行为风险：C8 明确规定 `account_id` 只是隔离键，"never reaches a surface, a log,
  or an exported file"（`architecture.md:365-366`）。任何一次 `agentdeck` 便携备份
  都会把 Codex 账号标识写进导出的归档文件。
- 证据：源码审读——`Backup` 对核心库调用 `s.Core.Backup` 生成完整快照，再以
  `os.ReadFile` 放入 `entries[coreName]`，其间没有表级过滤；迁移 v24 的两张表都含
  `account_id TEXT`。

💡 修复范围：隔离只需要比较相等，不需要原值。在 `internal/quota` 内对 `account_id`
做单向摘要（例如带固定领域前缀的 SHA-256），存储和比较都用摘要，
`EnvelopeRecord`/`Observation` 读出的也只是摘要或不再读出；同步修改 v24 迁移中两列的
注释与语义（v24 尚未发布），并加测试断言库中不存在原始值。若修复方认为摘要仍算账号
标识，或倾向于改为让备份排除 quota 表（这会触及 `internal/backup`，超出任务 1 的文件
清单），应停下来交由操作者决定。

### 🟡 改进建议 — 建议处理

以下同样是本目标的未关闭发现，PASS 前必须关闭。

**QD-R3-F1 — 低；新增：失败行的空账号被当成真实账号，导致同一账号的数据被丢弃。**

位置：`internal/quota/store.go:323-330` 的 `PutEnvelopeFailure` 在没有 envelope 行时
以默认 `account_id=''` 插入；`internal/quota/store.go:152-159` 的 `currentAccount`
优先读取 envelope 行的账号。

- 处置：新增，由 QD-R2-F1 的修复引入。
- 行为风险：客户端已有窗口但还没有 envelope 行时（例如某次成功探测写完窗口、写
  envelope 前出错），一次失败会写入一个"账号为空"的行。同一账号的下一次成功写入
  会被判为账号变更：该账号其他窗口被删、`observed_reset_at` 与上一次读数一起丢失、
  失败记录也被删除。窗口若恰在此时 reset，这次 reset 永远不会被识别，任务 5 的
  reset 通知随之漏发。C8 只在"探测返回不同的 account_id"时丢弃，而失败并不带账号。
- 证据：`TestProbeR3FailureRowWithoutEnvelopeDiscardsSameAccountWindows`——
  `acct-A` 记录 `codex` 90% 与 `codex_secondary`，失败写入产生
  `account_id=""`，随后同一 `acct-A` 记录 `codex` 5%，输出
  `accepted=true resetObserved=false stickyZero=true windows=[codex] envelopeStillPresent=false`。

💡 修复范围：让"尚无成功写入"的 envelope 行不参与账号判定——例如 `currentAccount`
跳过 `observed_at` 为空的 envelope 行并回退到窗口，或 `PutEnvelopeFailure` 在建行时
沿用窗口上已有的账号。补上面这个场景的测试：断言两个窗口都保留、90→5 被识别为
reset、失败记录保留到下一次成功的 `PutEnvelope`。

### 🟢 做得好的方面

- **QD-R2-F1 → CLOSED。** `discardOnAccountChange` 同时由 `Record` 与 `PutEnvelope`
  调用（`internal/quota/store.go:56`、`:281`），`EnvelopeRecord` 携带隔离用
  `AccountID`，Claude 强制为空。复现：新账号先写 envelope 再写窗口，得到
  `envelope ok=true plan="plan-B" windows=1 used=5`；新账号无窗口，旧窗口
  `rows=0`。新增的两个顺序测试和原有测试一起覆盖了三种写入顺序。
- **QD-R2-F2 → CLOSED。** 新增 `PutEnvelopeFailure` 只更新 `failure`/`failure_at`
  （`internal/quota/store.go:323-330`），`FailureAt` 与 `ObservedAt` 分离；成功的
  `PutEnvelope` 会清除失败。复现：成功后再失败，得到
  `applicable=true plan="pro" remaining=3 credits=1 source="codex" observedAt=2026-09-10T10:00:00Z failure="probe_failed"`。
  原先那种错误用法的测试已改用新入口，并补了"保留最后一次成功值"和"成功清除失败"
  两个测试。
- v24 迁移就地增加 `account_id`、`failure_at` 两列，没有另起版本；`main` 仍是 23，
  v24 未发布，就地修改成立。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式
复评——逐项核对 Round 2 的两项发现，审读修复后的 `store.go`、`model.go`、
`store_test.go` 与迁移 diff，并用 Go `-overlay` 注入 Round 3 复现测试；另外追查了
C8"不得进入导出文件"与备份实现的关系。未使用子代理，未修改生产代码、仓库测试或配置。

Scope：与 Round 2 相同的 12 个路径，外加只读核对 `internal/backup/backup.go`。

逐项处置：QD-R2-F1、QD-R2-F2 CLOSED；新增 QD-R3-F1（低）、QD-R3-F2（中）。
Round 1 的 F1–F3、F5–F8 仍为 CLOSED，F4 仍为 SUPERSEDED；本轮改动未触及它们的修复点，
Round 3 复现中 R2 场景的行为也与此一致。

Reviewed state：branch `feature/subscription-quota`，HEAD
`9d96ee40cf7be17ad29ffe84b6ffdbd0a30091b3`；与 Round 2 相同的 12 路径 manifest，
SHA-256 `6a31ee975fc66e4fe2da5529e28df1b010a929992f8a7ef79cfd19f287bf9559`。
与上一轮相比变更的 blob：`store.go` `f2f990cde04fe250068691cccc8941624eff9766`、
`model.go` `6bb982a9b5f1fed31772069fc88ae229ad824b3c`、
`store_test.go` `16835c8e5caa25a05400984689de194cdf8c9461`、
`migrations.go` `bc2ad546cd935570d5f13889d3ad24dd2ebeea7a`，以及 `tasks.md`。
复现与全仓测试运行时只有 `tasks.md` 与最终状态不同（交接文字），证据适用于最终状态。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，
  21 个包 ok，0 个 `--- FAIL`；日志 SHA-256
  `4a10dc407a6070b14a948bb70fc29b72f21ce05b33504bcd0b19b7dc3d644e0f`。
- Round 3 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-r3.json
  -run 'TestProbeR3' -v ./internal/quota/`，exit 0，输出 5 条 `REPRO` 行：3 条确认
  R2 场景已修复，2 条为 QD-R3-F1 的证据。probe SHA-256
  `51e9dd89629243a70d72ec19b71678561406d125537802a609f762081ccbada7`；日志 SHA-256
  `247da5c3faece654667567787f27804d45ce6d62b0eb2447d0784d24402dfd82`。
  材料保存在本会话 scratchpad，会话结束后可能被清理。
- QD-R3-F2 为源码证据，没有运行真实备份：不对真实状态目录做导出。
- 本轮未单独重跑 vet；构建与全仓测试已覆盖编译，gofmt 情况以修复交接为准，未独立复核。

完成门禁：FAILED。WorkUnit
`urn:agent-deck:work-unit:subscription-quota:quota-domain` 的目标已指向本轮内容状态
`urn:agent-deck:content-state:subscription-quota:quota-domain:9d96ee4:6a31ee975fc6`；
`verification` 记为 pass（全仓测试通过），`domain-contract` 与 `review` 记为 fail
（QD-R3-F1、QD-R3-F2 以及本轮 FAIL）。

Task 状态：Dev 保留勾选，Review 未勾选；Beads `ad-sq-quota-domain-dev` 退回
`in_progress` 等待修复。本轮没有 commit 或 push。

下一步指令：修复：subscription-quota / reviews/quota-domain.md / QD-R3-F1 QD-R3-F2

## Round 4 — 2026-09-11

## 📋 quota-domain 第三次修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 做得好的方面

- **QD-R3-F2 → CLOSED。** 新增 `accountDigest`（`internal/quota/store.go:46-52`），
  用带领域前缀、并包含 client 的 SHA-256 生成摘要；`Record`、`PutEnvelope`、
  `discardOnAccountChange` 与 `queryStored` 都只使用摘要（`:77`、`:85`、`:89`、`:128`、
  `:316`、`:328`、`:353`），`PutEnvelopeFailure` 不写账号列，`Envelope` 读出的也是摘要。
  v24 迁移注释写明了列的含义（`internal/store/migrations.go`）。复现：两张表都存 64 位
  摘要，且彼此一致；数据库打开和关闭时，扫描状态目录下所有文件（含 WAL），都找不到
  原始账号字节；换账号后隔离仍然生效。回归测试 `TestStoreAccountIDNeverStoredRaw` 在
  任一表写回原值时会失败。
- **QD-R3-F1 → CLOSED。** `currentAccount` 跳过 `observed_at` 为空的 envelope 行，
  改从窗口表取账号（`internal/quota/store.go:190-211`）。复现：`acct-A` 在失败占位行
  之后 90→5，结果为 `resetObserved=true`，两个窗口都保留，失败记录仍在；随后换成
  `acct-B` 仍会丢弃旧数据。回归测试
  `TestStoreFailureRowWithoutEnvelopeDoesNotDiscardSameAccountWindows` 断言了 reset、
  两个窗口和失败记录三项。
- Round 2 的三个场景在当前代码上仍然成立：新账号先写 envelope、新账号无窗口、
  成功后再失败。
- 四轮修复每一项都配有能在缺陷重现时失败的回归测试，存储测试都走真实迁移建库。

### 📝 总结

Reviewer：Claude Code 主会话（actor `claude-code`）；未参与修复。Method：单代理正式
复评——核对 Round 3 的两项发现，审读修复后的 `store.go`、`state.go`（仅注释变化）、
`model.go`、`store_test.go` 与 v24 迁移，并用 Go `-overlay` 注入 Round 4 复现测试。
未使用子代理，未修改生产代码、仓库测试或配置。

逐项处置：QD-R3-F1、QD-R3-F2 CLOSED。全部历史发现：QD-R1-F1、F2、F3、F5、F6、F7、F8
CLOSED；QD-R1-F4 SUPERSEDED，其承接项 QD-R2-F1、QD-R2-F2 均已 CLOSED；QD-R3-F1、
QD-R3-F2 CLOSED。本记录中没有未关闭的发现。

Reviewed state：branch `feature/subscription-quota`，HEAD
`9d96ee40cf7be17ad29ffe84b6ffdbd0a30091b3`；与前几轮相同的 12 路径 manifest，
SHA-256 `b38193f2086498d909256463455c8949c08356c34bad6a2a6d8344ea631f6722`。
与 Round 3 相比变更的 blob：`store.go` `e2ebe0d22658b5e6a762590116681d313dc9ac45`、
`state.go` `ca37b79edeb8a7c6c49074e47b360febc7b41726`、
`model.go` `49dd4c37c44913593e9a8a6c687ffbdaaa1a5703`、
`store_test.go` `4989c173e8a38744f09fb5194114d82f028aeffa`、
`migrations.go` `49eb486677592c93235955799627bee631368ee8`，以及 `tasks.md`。
复现与全仓测试运行时，只有 `tasks.md`（Review 勾选与交接文字）与最终状态不同。

Evidence：

- `GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：exit 0，
  21 个包 ok（含 `internal/quota`、`internal/store`、`cmd/agentdeck`），0 个 `--- FAIL`；
  日志 SHA-256 `49d00826df2961ded05a864ee8fd434f651a4ed29bd88532684eefc2b8bad356`。
- `go vet -mod=vendor ./internal/quota/ ./internal/store/` 通过；
  `gofmt -l internal/quota internal/store` 无输出。
- Round 4 复现：`go test -mod=vendor -count=1 -overlay <scratchpad>/overlay-r4.json
  -run 'TestProbeR4' -v ./internal/quota/`，exit 0，输出 6 条 `REPRO` 行，内容即上文。
  probe SHA-256 `b15b9e1abd5ed04b177ea858e5f2c79efaa6df490caa2c40b68c415ef1794241`；
  日志 SHA-256 `d495ec4d2ce20a624223499e6d34e769cf9b777ce7563915f07113ef0df95cb2`。
  材料保存在本会话 scratchpad，会话结束后可能被清理。
- 残余不确定：账号摘要没有加盐；Codex 账号 ID 形如 UUID，摘要不可逆，C8 的
  "不得出现在导出文件中"只禁止原值。未执行真实备份导出，结论依据源码与状态目录的
  字节扫描。

完成门禁：VERIFIED。WorkUnit
`urn:agent-deck:work-unit:subscription-quota:quota-domain` 的目标为
`urn:agent-deck:content-state:subscription-quota:quota-domain:9d96ee4:b38193f20864`；
`domain-contract`、`verification`、`review` 三项均有绑定此状态的 pass 证据。

Task 状态：Dev 与 Review 均已勾选；Beads `ad-sq-quota-domain-dev` 流转为
`awaiting_commit`。本轮没有 commit 或 push。

Task checkpoint：`ad-sq-quota-domain-dev`，content state
`urn:agent-deck:content-state:subscription-quota:quota-domain:9d96ee4:b38193f20864`
（manifest `b38193f2…f6722`），门禁 VERIFIED。

提交建议：只提交任务 1 的交付边界，即 `internal/quota/` 下 6 个文件、
`internal/store/migrations.go`、`internal/store/store.go`、`cmd/agentdeck/main_test.go`、
`desktop/fixtures/v1/snapshot-complete.json`、`desktop/fixtures/v1/snapshot-empty-client.json`、
`docs/topics/subscription-quota/tasks.md` 与本评审记录；提交前确认暂存的 blob 与上方
manifest 一致，不暂存 `cmd/agentdeck/usage_stats_viewer_test.go` 的既有 gofmt 偏差。

推送建议：`feature/subscription-quota` 目前没有上游，`origin` 上也没有同名分支；提交后
若需要备份或协作，可推送为 `origin/feature/subscription-quota`。合并到 `main` 属于
`v0-6-0-contract` 的 `assemble` 任务，不在本主题内。提交与推送都需要单独授权。

下一步指令：开发：subscription-quota / codex-adapter
