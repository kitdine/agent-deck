---
status: active
topic: codex-jsonl-session-identity
subject: scoped-usage-aggregation
---

# 作用范围用量汇总评审

## Round 1 — 2026-11-20，纯区间归约切片

### 📋 独立评审

📊 本轮范围评分 9/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🟡 需修复的问题

**CJSI-AGG-R1-F1**（P2）。`reduceScopedIntervals` 在检查 client 是否相交前要求 Codex 区间账本可读。只查询 Claude 且窗有效时，nil 成员应得到零可分配区间及完整覆盖，旧候选实际报错。独立合成用例先确认 Codex nil 拒绝，再只改 Claude client 复现错误。实际公开影响由后续读者接线决定。修复应先校验窗，再排除明确不相交的客户端，最后为 Codex 与 All 检查账本可用性。

#### 🟢 通过范围

已准入可读输入上，计量按真实残量归约，不按比例分配。边界、冲突重复、显式零与缺失、整数和成分存在性有针对性证据。未经证明的模型/provider 区间不生成行或额外调用，保留未过滤份额分母。

#### 📝 摘要

Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-pure-codex-review-054e8ce2`。启动只收到本轮候选简报。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `054e8ce2305b3651a8217ea0af9c2038ebd2d0bdc7b9c6dc3bba379779fc9c07`、Go `45f21653c6c7839f902c2de31df2cd253f6bc75385b9aff4e915695201a3d9f0`。主控在作者额度中断后保留两路径原始后像，核实原定向锚点 54 条、race 5 条及 vet。独立评审另以最小 Go overlay 实测 F1。数据库、cache invalidation 与既有读面接线仍开放。

#### F1 修复候选补证

先校验半开窗，再对无法含有 Codex 区间的 client 返回零贡献。Codex 及 All 仍检查已准入成员是否可读。新增用例在旧入口实际失败，修复后确认 Claude nil 得完整空集合，空窗仍拒绝。两路径增量 `960978410badec8703e389f3e029b8581b8e9ddf8a31d935b65e4879552a4f74` 重放到产品 `f7b740905a402305a28a904423ab499aecd0b78871a398e86c837b7581ebccc8`、Go `3f2347d40a616b9a361496ec2a151f17a6ece726bbbcc865c1c1b6a2d8719260`。主控核对零 fuzz、零 offset 及模式，普通 55 条、race 6 条通过且无跳过，vet 干净。正式冷复评待完成。

## Round 2 — 2026-11-20，客户端隔离冷复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-AGG-R1-F1 CLOSED。** 窗校验先于不相交 client 的短路。Claude 而账本不可用的合法窗返回零贡献、无会话、完整覆盖。无效窗仍拒绝。Codex 与 All 仍要求所选账本可读。差异在成员循环前结束，沿用窗口、残量、存在掩码与份额分母的回归证据。

#### 📝 摘要

Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-cold-r1-rereview-f7b74090`。通用项目背景之外未收到候选作者对话或实施轨迹。复评核对产品 `f7b740905a402305a28a904423ab499aecd0b78871a398e86c837b7581ebccc8` 的作用范围、前后候选和绑定原始输出。新断言在原候选失败，修复后普通 55 条与 race 6 条全过。此轮关闭纯归约切片发现。持久事实读取、cache invalidation 和公开调用方继续按原分工。

## Round 3 — 2026-11-20，私有联合汇总冷评审

### 📋 独立评审

📊 本轮范围评分 10/10

评审结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 通过范围

同一 `(codex,C)` 的请求与残量各贡献 10 tokens，汇总为 20 tokens、1 次调用、1 个会话。请求的已知目录价和供应商小计分别仍为 10 和 20，完整金额为空，紧凑视图标记成本不完整。区间不生成额外调用、时间戳、计价或信号。模型和 provider 过滤只在请求侧选行，份额分母保留同窗的未分配残量。半开边界、显式零和缺失选项、溢出拒绝、异客户端与不相交窗口保持核对后的旧请求结果。

#### 🟡 组合验收条件

调用方须从同一快照提供符合权限、发布覆盖和初始 scope 的全部有效请求，再应用模型或供应商过滤。直接复用已带 `options.Model` 的旧 Stats 查询会缩小份额分母。区间须按同一 scope 与会话归约成功后再交给联合汇总。公开 summary、stats、sessions、分区与缓存失效、桌面展示及 Task 8 发布仍按原分工。

#### 📝 摘要

Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-joint-reducer-review-a49c21f0`。启动只有通用规则和无关任务背景。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `a49c21f0f4293fe98210ca71e04c60eeaa132acf107fd32b14af5e5d895a7a6b`、Go `a549b4ea811f74bddfb574f1421de8a3339d15fa91ca343d4b7c29dbea13f781`、两路径增量 `592c19b88118f76b7d58179571129333d5c15b0f0b9e77bf6a272f5256bfde24`。主控核对七项校验、前像缺席、精确模式与重放。独立评审核对源代码和已绑定测试回执，Go 1.27.1 普通 64 条、race 13 条、无跳过，vet 通过。PASS 适用于私有联合归约切片。

## Round 4 — 2026-11-20，选定区间读者冷评审

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

**CJSI-AGG-R4-F1**（P2）。`decodeIntervalRow` 的校验接受缓存累计下界未测、上界为 5 时的残余缓存 6。两端已测时，原代码还会接受 26 个输入全是缓存、残余 6 输入中只有 3 缓存的区间。这会让余下 20 个输入承担 23 个缓存命中。reasoning 与输出的相同矛盾同样能通过。独立 SQLite 证据和主控红测均证实失败。

**CJSI-AGG-R4-F2**（P2）。SQL 原查询正确使用 ticket 和 member_id，旧夹具缺少同线程同 attempt 的异票行及同组选中成员的不同键值，故未能区分漏 ticket 或漏 member_id 的查询。补证应在合成库中使两个独立变体分别失败。

冷启动只有简报提供的候选背景。Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，`v075-e21-cjsi-task7-interval-reader-review-44f607c7`，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `44f607c76f36821894a94f03965de5b0716127a1d8e16d1a56c55c84445401c7`，Go `ff1bb579f4a3f18db53d7a271f0ec66ff9c4378a8f3108ce89112e3a71967094`。已有普通 73 条与 race 24 条通过，无跳过。额外合成见证暴露剩余阻断发现。准入竖切片及上游授权仍由后续集成验证。

## Round 5 — 2026-11-20，区间失效范围冷评审

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

**CJSI-AGG-R5-F1**（P2）。`residualFitsRange` 会接受跨度 10、实测子项 8、余量 9 和子项 6 的记录。扣除余量后，1 个剩余主项无法包含 2 个剩余子项。缓存命中与 reasoning 均可复现。将两侧设成同一错误记录时，旧规划器返回有效空计划，和真正的无变化无法区分。主控用独立负向 fixture 复现了插入、相同两侧和撤回的错误接受。

冷启动未注入此候选的作者历史、执行历史或原有裁定。Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，`v075-e21-cjsi-task7-dirty-ranges-review-7f63427c`。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `7f63427c919cb927a2eeedfbd4af1fa9b744a8f67c91c1f9cfde84a29c4d5d67`，Go `6b4c674c0a5caaba2560b3891fb4defcc8e3f948dfb77825a56e03020d5fe5fe`。已有普通 69 条与 race 7 条通过，无跳过。删除最后一项、分离 scope 与紧凑日期范围的有效输入证据保留。真实持久化及 Task 8 准入仍是独立的集成事项。

#### Round 4/5 修复候选补证

读者和规划器共享同一余量校验。已观察的缓存与 reasoning 要在整个区间和扣除已归约请求后的两段都可行。累计下界未测时，以其主项约束剩余子项的最低可行量，累计上界仍明确限制已测余量。未测的余量分项继续作为未知。无效事实在构造结果和比较 OLD/NEW 前被拒。主控按同一失败基线验证了真实 SQLite 合成数据及两侧相同的无效计划。

当前 SQL 保持完整键。扩充夹具让同一线程同 attempt 的另一票据和同组选中区间的两个不同 member_id 同时存在。仅移除 ticket 过滤时实际多读一行，仅移除 member_id 连接键时两个成员实际交叉连成四行。两个独立变体均在目标断言处失败。

组合前像产品 `38092ccd3732635a5680a01956493d5881ea42109152321340ab4aa4349ff190`，修复产品 `0cb8817f6e60d84c06072e56182e2b13561607a092ee93b97ffbeae49a0ddc0f`、Go `193b5f02d1e77acd24761f88ae61f91718fe108c6456852401d59fe5d08c3f48`。五路径增量 `ce2f2a3c79577f0806a289ea5e680264eea91f5fbacb3894c7c6b166a4267282` 精确重放，普通与 race 各 99 条通过、无跳过，构建、vet 与格式核对均通过。待新独立复评关闭三个发现。

## Round 6 — 2026-11-20，共享余量校验冷复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

- **CJSI-AGG-R4-F1 CLOSED。** 选定读者先校验同一跨度及扣除余量后的可行量，再返回成员或汇总。已测上界独立约束已测余量。两端已测且余量未测时保留未知，以及余量与剩余量恰好可容纳的等号控制均通过。独立复评核对了冻结账本中原累计端点与归约余量的含义。
- **CJSI-AGG-R4-F2 CLOSED。** 原完整查询仍通过。只移除 ticket 过滤会把同线程同 attempt 的另一票行并入结果；只移除 member_id 连接键会把两个成员交叉连成四行。各自的新夹具负向变体均命中指定断言。
- **CJSI-AGG-R5-F1 CLOSED。** 规划器在比较 OLD 和 NEW 前分别校验事实。缓存与 reasoning 均拒绝剩余子项超出剩余主项，覆盖由空到坏值、相同坏值、撤回三种输入。边界可行值仍产生四类范围。

#### 📝 摘要

Codex `gpt-6.2-astra`、T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-shared-residual-rereview-0cb8817f`。简报之外未注入该候选的作者、执行或既往裁决。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`，产品 `0cb8817f6e60d84c06072e56182e2b13561607a092ee93b97ffbeae49a0ddc0f`（168 路径）、Go `193b5f02d1e77acd24761f88ae61f91718fe108c6456852401d59fe5d08c3f48`（154 路径），增量 `ce2f2a3c79577f0806a289ea5e680264eea91f5fbacb3894c7c6b166a4267282`。逐项核对了有效清单与五路径内容和模式。复用 Go 1.27.3 的普通、race 各 99 条通过（无跳过）及 build、vet、gofmt。主控另以枚举可行分配的临时见证核对 3388 种小计数组合。持久缓存、公开读路径与 Task 8 发布仍按原分工。

## Round 7 — 2026-11-20，区间失效戳 UTC 日界复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-AGG-R7-F1 CLOSED。** 规划器接收的业务发生时刻均为非零，转换为 UTC 日范围后，合法起点或开区间终点可能等于 Go 的零值时刻 `0001-01-01T00:00:00Z`。原写入器误拒两个范围。修复按严格递增的签名 UTC 日编号验证持久边界，读者按时间顺序验证半开查询窗，保留 epoch、票据、scope 以及未来票拒绝条件。业务事件时刻仍经原校验。

两个回归将 01:00 和 02:00 的实际输入送给规划器。每次生成四类范围，借用事务先推进票据 1→2，再写入并提交。新事务验证四条 SQLite 记录存储确切日期和票据。四类范围查本日均返回票据 2，只接触午夜的相邻日均返回零。两例分别覆盖零值时刻为起点和为开区间终点。原候选在写入处失败，修复后普通和 race 均通过。

#### 📝 摘要

独立冷复评为 Codex `gpt-6.2-astra`，T3 配置 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-stamp-zero-boundary-review-1ad66a65`，authority epoch 21。候选作者、执行历史与往前裁定仅来自本轮简报。话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。候选产品 `1ad66a65184065626cab0617b614dc566118e86375b9ffd8ae624f30e9ce9459`、Go `b81d57e6bc6cf91b246e4df2bad707f0e69d113bd9b66f0f99cd74102f0a2d46`。两路径增量 `c54026d045ecc390d05c9a9576978aede09f58501e1b6a7a7593d0252fefa9de`。主控精确重放两文件，核对模式、全清单与红绿日志。普通 29 条、race 6 条通过，零跳过，build、vet、gofmt 通过。通过范围为 UTC 日界修复。区间戳原始适配器审查、公开缓存及发布接线分别处置。

#### 前序原评审核对

原适配器独立评审请求 `v075-e21-cjsi-task7-interval-stamp-adapter-review-fc96a72d` 在产品 `fc96a72d8901b5f010bf493969e4c64fba65b199c575e3d085d77f2116b42608` 上独立复现 `0001-01-01` 业务事件归一化到零值午夜后被拒。该反例与 **CJSI-AGG-R7-F1** 一致，Round 7 已验证午夜作为起点及开区间终点各一例，并记录修复候选的独立 PASS。

## Round 8 — 2026-11-20，缓存时钟、编码及请求顺序

### 📋 独立评审

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 需修复的问题

**CJSI-AGG-R8-F1（P1）。** 桌面 v2 在柏林 2026-10-25 的 01:50 CEST 保存上一小时。原截止选中第二个 02:00（01:00Z）。第一个 02:10（00:10Z）重新准入仍命中旧数据，`ThroughHour=1`，实际应为 2。修复 v2 截止并验证重复小时。请求 `v075-e21-cjsi-task7-desktop-derived-cache-v2-review-b86fa98f`，产品 `b86fa98f2582e35bbf988c87c4cebf01f7dd17941775081b45a78c1cf1409717`，Go `948a507425b7aaee1c69ce785e81463740c395735f8c2c81cb6fae13453098d4`。

**CJSI-AGG-R8-F2。** 请求缓存 v2 从解码值重算校验和。输入额外的 `warnings:[]` 后同一校验和成立，命中返回了非 nil 空数组及非规范字节。未知 JSON 字段同样可被解码忽略后继续保留在返回的旧字节中。独立反例验证前一情况；公开存储或显示未在此夹具验证。v2 应只接受规范编码，其他内容从准入账本重建。请求 `v075-e21-cjsi-task7-projected-cache-v2-review-a16c92ca`，产品 `a16c92ca9ce5a712b0868cc7cd2af61531a06cba33fe762ff4d13b0241647698`，Go `2e2865c054105bcb7ed1d93d9496aa685d103d2db696bbe399557bf877479aae`。

**CJSI-AGG-R8-F3。** 汇总桥保持准入请求的顺序。独立评审在归约前反转请求，现有夹具仍通过。旧汇总按首次观察顺序累计警告。增加等时、反键序写入、差异警告及有价与无价请求的断言，并使反转变体在新断言处失败。请求 `v075-e21-cjsi-task7-same-snapshot-review-30c74063`，产品 `30c74063afae1374e03e7a607f2dd33c78ddd51da677c49f4b0e92847fdbc49b`，Go `29494e74510e72242355fa86fec0271bf164085b9d4f1abfbd4d03dd2b3f8c63`。

#### 📝 摘要

三个评审均为 Codex `gpt-6.2-astra` 冷评审，T3 `reasoningEffort=xhigh`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。前两项分别为隔离运行的实际缓存反例，第三项独立执行了测试保护变体。各评审的输入、原日志、测试及重放绑定均已核对。其他已验证的局部行为继续作为可复用证据。最终公开接入仍由 Task 8 负责。

## Round 9 — 2026-11-20，缓存及请求顺序复评

### 📋 独立复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 仍需修复的问题

**CJSI-AGG-R8-F1。** 新版产生的柏林重复小时缓存能在第一个 02:00 按时失效，且 Pacific/Chatham 的 03:45 变更可提前截断。原 v2 写入器已存的不正确期限仍可能与第二次准入的期限相等，因而把 01 时数据带入第一次 02:10。另在柏林 2040-12-31 13:10，Go 1.27.1 的 `ZoneBounds` 返回已过去的 00:00Z，候选把正常新键误判过期。复评通过实际 v2 构造和读取在同一见证中复现两个边界。须使新读者拒绝旧算法条目，并且只以处于未来的时区变更截断。

#### 🟢 已关闭发现

**CJSI-AGG-R8-F2 CLOSED。** 仅 v2 接受经校验的规范字节。同一个有效校验和加入空警告数组或未知字段均触发一次准入账本读取，返回与冷建同样的完整行、版本及字节。规范命中仍为零读取。

**CJSI-AGG-R8-F3 CLOSED。** 同时刻 Claude 事件以相反键序插入。实际读者按时间与键排序，先发默认缓存 TTL 警告，再发无归属警告。实际数额为已知目录 5、已知供应方 10，两项完整费用缺失。一有价一无价。仅反转归约输入的夹具在警告顺序处失败，产品桥接保持原代码。

#### 📝 摘要

Codex `gpt-6.2-astra` 冷复评，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-cache-and-order-review-051f39f1`，authority epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。候选产品 `051f39f1f435d5e1ccfb49ad708ba55c6b20ec8df0863b9487db86abc2cb0129`、Go `2e7cbeca10cf7a6c21fb8bfdb743e5c4250d6d12ea50d927a181935ce82e74b0`，补丁 `d8b4447bde49b80180bd556865df47fa371fbf149122654e0903668b4847cfb9`。独立评审匹配全部候选条目，复用普通和 race 各 21 条通过及三类反例。额外一个受控见证复现 F1 中的两个情况。主控原简报抄录的前序产品指纹有误，已复核实际前序为 `e4ee9d3d9e9bba91f349aa65e8b56a4ea628aaecbd057a3ba5884a7a81782b20`；原简报 Go 指纹 `bac093e5516b13c3d17ffff0847cc3c4875b2602ab9b4434a5b9ffeb20d94915` 与实际匹配。跨版本见证对应所给的前序 Go 内容。候选自身的过期误判独立于该抄录。

## Round 10 — 2026-11-20，桌面 v2 时钟后续复评

### 📋 独立复评

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🟢 已关闭发现

**CJSI-AGG-R8-F1 CLOSED。** 私有桌面 v2 使用新的算法版本，读者拒绝旧算法的缓存期限。用真实旧时钟方法构造可通过校验和的旧 v2 文档后，柏林第一个 02:10 的新读者返回零 payload 和未命中。新写入在第一个 02:00 前一纳秒可复用，之后失效。未来时区变更才参与截断。柏林 2040-12-31 的有效键可建立并在下一个本地小时到期。独立见证的六个控制包括柏林两个 02:00、Chatham 提前变更、Kathmandu 午夜和更早的日边界。v1 的原时钟、算法及编码继续通过旧路径验证。F2 和 F3 的已有关闭见 Round 9。

#### 📝 摘要

Codex `gpt-6.2-astra` 独立冷复评，T3 `reasoningEffort=xhigh`，请求 `v075-e21-cjsi-task7-cache-clock-r2-rereview-3f4e8d83`，epoch 21，话题 HEAD `5a1cd9159b564469b716a3338b5573adece90978`。产品 `3f4e8d83a47509fc14cd5515619c32c08a68d3f73d3a0ac1b2610e9bf550eef0`、Go `54bb9045fbcfa21f47c248d7a1ef2abb0de7d016afa83bfa20c04e971ece0346`，补丁 `77dc79b4ed3b747cda4dd7bea572d4cdc2ae2ed10780763939935537cc2713e0`。两个后像字节和模式精确重放，普通、race 各 14 条通过且零跳过，前序以相同 fixture 两条失败。独立探针补证截止算法。通过范围为 F1 修复，公开汇总及首开票接线归 Task 8。

## Round 11 — 2026-11-24，候选丢失后的重建复评

评审结论：PASS

完成门禁：VERIFIED

此前候选只存于 /private/tmp，主机重启后丢失。本轮在话题工作树内重建 Task 7，基于 `bbb06da`（与 `bbb06da..a0a13c2` 的新提交零重叠）。初版作者 AGY Gemini 4 Neon high（中断后续做），修复轮 1 同一 AGY 线程，修复轮 2、3 改由 Codex `gpt-6.2-sol` high。最终补丁 SHA-256 `d01de079890942c00cff622d6381e56e3b181caf22076342730ac95232a7cc62`，26 个路径，均在 Task 7 所有权内（`internal/usage/signals_metrics.go` 属“必要的已有 stats 呈现文件”，改动仅让原查询接受借用事务）。

- **重建首评（Codex `gpt-6.2-sol` high）FAIL。** CJSI-AGG-R9-F1（P1）Stats 桶计入查询窗外区间；R9-F2（P1）桌面与会话 helper 混读 live store 的不同代事实；R9-F3（P1）桌面 v1 缓存字节改变；R9-F4（P2）Stats 丢失模型活动与 detail_command；R9-F5（P2）未测缓存显示为实测零；R9-F6（P2）桌面汇总溢出仍成功；R9-F7（P3）CacheSessions/UnpricedModels 顺序不稳定；R9-F8（P3）合法系统时区被缓存键拒绝。历史 R1–R8 未复发。
- **R1 修复与复评（Claude Sonnet 5.5 high）FAIL。** R9-F1、F3–F8 closed；R9-F2 部分关闭。新发现 CJSI-AGG-R10-F1（P2）WorkSignals 缺同视图读者；R10-F2（P3）Stats 工具读者可选；R10-F3（P3）桶×区间复杂度；R10-F4（P3）Location 为空时缓存戳不绑定分桶时区且依赖墙钟；R10-F5（P3）ZoneSpans 无回归；R10-F6（P3）CacheSessions 主序与 LogicalInput 序无断言；R10-F7（P3）系统时区测试受宿主 TZ 影响。复评确认带 `--model/--provider` 过滤时 share 用未过滤分母是 A4b 需求行为（requirements.md:190），写入 Task 8 交接。
- **R2 修复与复评（AGY Gemini 4 Neon high）FAIL。** R10-F1、F2、F4、F5、F6 closed；生产代码无 P0–P2。新扫描线算法与旧穷举算法在 1,632 份报告（含 DST 切换、45 分钟偏移、空桶、半开边界）逐字节一致，400 天小时桶 × 2 万区间由 5.7 秒降至约 55 毫秒；v1 桌面缓存与请求分区 payload 与基线一致。首次 race 中取消用例 1.014s 超时判定为既有抖动，与本补丁无调用交集。测试侧新发现 CJSI-AGG-R11-F1（P3）`UpperAt == 桶边界` 结转被全窗区间掩蔽，变异可存活；R11-F2（P3）系统时区测试硬编码 `UTC+14:00` 冲突标签。
- **R3 修复与复评（AGY Gemini 4 Neon medium，新会话）PASS。** 相对 R2 只改两个测试文件。R11-F1 closed：单区间无掩蔽回归锁定端点结转，独立复做的变异使其失败；R11-F2 closed：冲突偏移断言移到固定 `Local -08:00` 夹具，UTC、Asia/Tokyo、Pacific/Kiritimati、`:/etc/localtime` 均通过。本轮无新发现。
- **验证。** 各轮复评在 `bbb06da`/`a0a13c2` + 补丁的隔离副本运行：三锚点非空、`internal/usage`、`internal/store`、`internal/desktop` 全包 `-p 1`、usage 与 desktop race、多时区测试均通过；gofmt、vet 干净；变异在各轮被对应回归拦截。测试仅用隔离目录。
- **门禁与交接。** CEv1 WorkUnit `codex-jsonl-session-identity:scoped-usage-aggregation` 在该补丁状态上独立评审、L3 验证、范围与隐私 3/3 满足。Task 8 须按 handoff 以同一借用事务接入演示、会话、Stats 与 WorkSignals 读者，并在同票开放中绑定缓存戳。
