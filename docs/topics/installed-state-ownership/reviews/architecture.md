---
status: active
topic: installed-state-ownership
subject: architecture.md
---

# Architecture Review

## Round 1 — 2026-11-05

### 📋 架构契约评审报告

📊 总体评分：4/10

✅ 评审结论：FAIL

- Reviewer: Codex；独立于本批文档作者的当前评审会话，未委派。
- Method: 设计契约评审、原始缺陷交叉核验、所需源码调用路径核验及临时目录内的设计规则反例；未执行真实安装、真实数据操作或产品实现。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。
- Scope: 运行时身份、回执权威、所有权切换、路径规范化、Hook 诊断和四份设计之间的契约。
- Reviewed state: blob `597b1e05e1889e9708a6f14f2c046875afc21dbe`；fingerprint `fd77af7d3a61628b53e22fcbf1cf2891ee352308b086f07fb05bfd72ab425636`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:architecture.md`。
- Target ContentState: `installed-state-ownership:arch:review-20261105-a0fXv0-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 查询返回本 WorkUnit / ContentState 的 FAILED；review 失败证据 applicable=true，L0 通过证据有效；invalidated_evidence 与 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 全部 exit 0；主题集合检查也为 exit 0。静态检查不代表语义评审通过。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R1-F1 — P1 — 每个 keg 都能自证拥有者，不能保证唯一安装身份。**

- 位置：[architecture.md](../architecture.md#安装身份判定) 第 48 行；安装方式表第 141 行。
- 行为风险：保留的旧 keg、不同 prefix 或 stable/rc formula 仍可直接启动并访问、迁移真实状态；“只有一个拥有者”无法成立。
- 证据：没有用户回执时，两条可执行文件路径 `Cellar/agentdeck/1/bin/agentdeck` 和 `Cellar/agentdeck/2/bin/agentdeck` 各自读取相邻 keg 回执。两个回执的 channel 均为 homebrew_formula 且 executable 分别等于各自 self，因此两个 Resolve 都返回 Installed=true。判定完全没有检查统一的当前安装选择。当前 formula 模板的 `install` 也只安装当前 keg 的 bin；cask 的 formula 冲突 preflight 不参与两个 formula/keg 之间的判定。
- 💡 修复建议：定义一个所有安装方式共同遵守的当前拥有者选择权威；keg 回执只能证明该文件曾安装，不能单独授予当前所有权。明确旧 keg、不同 prefix、stable/rc、用户回执过期及安装迁移的判定和测试。
- Disposition: OPEN。

**ISO-ARCH-R1-F2 — P1 — 永久缓存身份使接管后旧进程继续拥有真实状态。**

- 位置：[architecture.md](../architecture.md#安装身份判定) 第 53 行；第 99、142–143、164、172 行。
- 行为风险：A 进程或长驻 worker 在启动时缓存 Installed=true；B 经 TAKEOVER 接管后，新 B 和旧 A 同时拥有权限。旧 App 还会继续使用缓存的真实 state_root 直接读文件。
- 证据：按文档顺序模拟“回执=A → A缓存true → 回执=B → B缓存true”，A/B 均为 true；GuardStateRoot 对已安装直接放行。文档未定义撤销、切换的串行化、已有句柄/worker/直接 Swift 文件读取如何收敛，也未要求接管先停止旧拥有者。一次原子回执重命名不能撤销进程缓存。
- 💡 修复建议：定义安装所有权的生命周期和安全交接边界，包含已有进程、数据库句柄、worker 与 App 文件探测。可以选择经验证的停机交接或可撤销的所有权机制；必须明确旧持有者停止访问后新持有者才能开始，并为并发安装及接管失败规定结果。
- Disposition: OPEN。

**ISO-ARCH-R1-F3 — P1 — 真实状态根本身是符号链接时，保护路径与候选路径不在同一规范空间。**

- 位置：[architecture.md](../architecture.md#什么算指向真实状态目录) 第 89–95 行。
- 行为风险：用户将 `account/.agentdeck` 迁移到另一磁盘并以符号链接保留入口时，未安装进程即使用 `--state-dir account/.agentdeck` 也会被放行。
- 证据：保护路径定义是 `EvalSymlinks(account)/.agentdeck`，只解析 account；候选路径却解析完整最长已有前缀。在临时夹具 `account/.agentdeck → moved-state` 下，保护路径仍为 `account/.agentdeck`，候选变成 `moved-state`，既不相等也无包含关系，按文档判定 refused=false。
- 💡 修复建议：以一致规则解析真实根和候选，并处理真实根尚不存在、其自身/祖先符号链接、子路径及解析失败。增加真实根本身为链接的反例，不能仅测试另一个链接指向正常真实根。
- Disposition: OPEN。

**ISO-ARCH-R1-F4 — P2 — 安装拒绝不能直接沿用 schema_ahead 的 Hook 拒绝记录。**

- 位置：[architecture.md](../architecture.md#只读和-hook-路径) 第 154 行。
- 行为风险：实现者要么把安装拒绝写进受保护真实状态（违反 G3），要么伪造 schema_ahead 数据/改变既有记录契约；按现有机制则不会记录该原因。
- 证据：`internal/hookrefusal/record.go:30-54` 只接受 code=schema_ahead 且 stored>supported；`Write:66-100` 要求有效 schema 差，并在传入状态根内创建临时文件后重命名为 hook-refusals.json。`cli-design.md` Doctor 一节固定了这一结构和语义。未安装拒绝既没有 schema 差，也不得为了记录诊断而读写真实根。
- 💡 修复建议：保留既有 schema_ahead 机制，明确安装拒绝仅通过安全的 installation 诊断展示，或另行定义隔离位置上的记录及消费者；不能宣称现有拒绝记录已覆盖新原因。
- Disposition: OPEN。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

将守卫放到包边界；不使用版本号/编译标记授权；区分用户回执与 Homebrew 沙箱限制；安装失败与 Debug App Group 人工前提有明确记录。

#### 📝 总结

三个 P1 直接违反 G2/G3/G7 与 A1/A6；P2 违反 Hook 诊断现有契约。另一个 doctor 协调问题由 [CLI 评审](ux-cli.md) ISO-UX-R1-F1 承载，同轮修复需同步架构，不重复编号。全部仍 OPEN，故 FAIL。

Evidence: 只在 Python TemporaryDirectory 中构造路径及纯规则模型，输出：
- 真实根为符号链接：`guard refuses = False`；
- 没有用户回执的两个保留 keg：`accepted retained kegs = 2`；
- 回执 A→B 后：`cached A/B Installed = True True`。

这些是按设计逐步执行的反例，不是尚未实现的 Go 守卫运行测试。复现方法已在各发现中完整记载，临时目录在退出时删除，无真实用户数据或安装修改。CodeGraph 取得当前迁移链、doctor 与 hookrefusal 源码；直接核对 formula/cask 模板和稳定 CLI 契约。已有决定性反例后停止广泛验证，未运行真实 brew 安装、macOS 构建、全量 Go 测试或原型验收。L0 最终结果另行登记到 CEv1。

修复范围：ISO-ARCH-R1-F1–F4，并同步 CLI 评审中的 doctor 路由决策。

## Repair 1 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-ARCH-R1-F1 至 F4，以及 ux-cli 记录中 ISO-UX-R1-F1 需要的架构部分。只修改 `architecture.md`。
- 用户决定（2026-11-05，修复期间由用户在会话中明确选择）：F1 采用"formula 安装后显式认领"，F2 采用"随时复核 + 停机交接"。F3 和 F4 不需要新的产品决定，由修复者在已批准的原则内处理。
- Beads：`ad-installed-state-ownership-doc-arch` 由 claude-code 领取修复。

### ISO-ARCH-R1-F1 处置：已修复，待复评

- [安装身份判定](../architecture.md#安装身份判定)：删除 keg 回执。唯一权威是用户回执，程序附带的任何标记都不能授予所有权。
- 回执的 `executable` 可以是一个稳定入口，在判定时才解析：formula 登记 `<prefix>/opt/<formula>/bin/agentdeck`。这样升级后不需要重新认领；保留下来的旧 keg 不是 opt 指向的目标，所以不是拥有者。
- 新增 [formula 认领](../architecture.md#formula-认领)：`install claim` 只能认领 opt 当前指向的 keg；另一个程序已是拥有者时需要 `--takeover`，并满足交接前提。
- 对照评审反例：两个保留的 keg 不再各自读取相邻回执，只会和唯一的回执比较。最多只有一个程序能解析成回执登记的路径，所以"两个都为已安装"的结论不再成立。不同前缀、stable 与 rc 同理：哪个是拥有者，取决于用户对谁执行了认领。

### ISO-ARCH-R1-F2 处置：已修复，待复评

- 删除"在进程内缓存，只算一次"。[状态访问守卫](../architecture.md#状态访问守卫)在每次打开或加锁真实目录时都重新判定。
- 新增 `RevalidateOwnership`：长驻进程（`watch`、telemetry 监听、desktop bootstrap 流、scan worker 的每一轮）在每轮开始前复核，失败时释放句柄和锁、以 `state_ownership_refused` 退出。
- 已在进行中的单次操作与新拥有者的打开和迁移，由现有的 `state.lock` 串行（`store.open` → `AcquireLock`）。
- 新增 [交接前提](../architecture.md#交接前提)：接管前用 `lsof -t` 检查旧拥有者的可执行文件和 App 主程序，有进程运行就拒绝，回执保持不变。
  - 用户选项中原本写的是"检查真实目录里的锁是否被持有"。修复时改为检查旧拥有者的进程，原因有二：接管方在写入回执之前仍是未安装进程，按 G3 不能读取真实目录里的锁文件；而"旧拥有者没有任何进程在运行"更严格，同样排除了正在持锁的进程。这项调整在用户所选方案的意图之内，请复评时确认。
- App 改为每一轮刷新前查询 `state-location`，不跨轮缓存。
- 对照评审反例："回执=A → A 判定为已安装 → 回执=B → B 判定为已安装"之后，A 的下一次守卫会重新读回执，得到 `receipt_other_executable` 而被拒绝；而在 A 仍有进程运行时，接管本身就会被拒绝。

### ISO-ARCH-R1-F3 处置：已修复，待复评

- [什么算指向真实状态目录](../architecture.md#什么算指向真实状态目录)：真实目录和候选目录改用同一个 `canon` 规范化函数。受保护集合同时包含 `canon(account/.agentdeck)`（真实目录本身是链接时，这一步会跟随到目标）和词法入口。规范化失败时一律拒绝，原因为 `real_root_unresolved`。
- 架构中列出了必须覆盖的反例，包括评审给出的情形：真实目录本身是链接，候选目录写成链接入口或链接目标。
- 对照评审反例：`account/.agentdeck → moved-state` 时，受保护集合包含 `moved-state`，候选目录也规范化为 `moved-state`，两者相等，因此拒绝。

### ISO-ARCH-R1-F4 处置：已修复，待复评

- [只读和 Hook 路径](../architecture.md#只读和-hook-路径)：删除"可以在 schema-version-signal 的 Hook 拒绝记录中看到"这一说法。明确 `hook-refusals.json`（`internal/hookrefusal/record.go`）保持原样、不扩展，也不新增其他拒绝记录。
- 安装拒绝只通过两个只读诊断展示：`installation` 检查，以及新增的 `managed_command_owner` 检查。后者只读地比对受管 Hook 和 statusline 的绝对路径命令是否指向当前拥有者，识别方式沿用 `managedAgentDeckExecutable`（`internal/usagehook/config.go:1009`），因为接管之后，正是这类命令会被静默拒绝。

### ISO-UX-R1-F1 的架构部分

见 [doctor](../architecture.md#doctor)："状态目录被拒绝时，doctor 输出部分报告"。处置记录在 [CLI 评审](ux-cli.md)。

### 随修复新增：测试与验收的隔离

这部分是为了承接 tasks 记录中的 ISO-TASKS-R1-F1 和 F2，在架构中新增 [测试与验收的隔离](../architecture.md#测试与验收的隔离)：

- 包内测试使用未导出的注入字段；
- 端到端验收使用 `agentdeck_acceptance` build tag 构建的验收二进制；
- 生产产物只做静态的符号检查；
- 用 `sandbox-exec` 的访问即终止配置证明"没有访问"，每次先做阴性对照。

这个配置已在本机临时目录验证：直接访问、子进程 `stat`、经 `..` 和经符号链接访问时，进程都以 exit 137 终止；访问其他目录不受影响；验证用的临时目录已删除。

### 验证

结构检查同 [需求记录](requirements.md#验证) 中的 Repair 1 验证。架构中新引用的源码位置都在 HEAD 上核对过：

- `newDoctorCommand` 位于 `main.go:3498-3517`；
- `managedAgentDeckExecutable` 位于 `usagehook/config.go:1009`；
- `acquireNamedLock` 以 `O_EXCL` 加 token 的方式创建锁文件，位于 `store.go:714-730`；
- `doctor.Report.Checks` 的 JSON 名称为 `checks`。

Homebrew 的 formula 与 cask 沙箱约束沿用设计阶段的核对结果。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `3c18bba003ee735aefdea983874ad71d7c3a4f9c`；指纹 `0179ac4aa267c2626d13957c8abe400ddde3e7ce4eb4e0b242e462e54b12895c`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 2 — 2026-11-05

### 📋 架构复评报告

📊 总体评分：5/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code 的 Repair 1 候选，未委派，未修改设计正文、产品代码、测试或配置。
- Method: 按原发现逐项处置；复用相同 HEAD 的源码事实，并检查修复新增的契约。设计反例与生产运行验收严格区分。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `3c18bba003ee735aefdea983874ad71d7c3a4f9c`；fingerprint `0179ac4aa267c2626d13957c8abe400ddde3e7ce4eb4e0b242e462e54b12895c`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 对本轮精确目标返回 FAILED；本轮 review/L0 证据均适用，旧状态证据 target_matches=false；无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` exit 0，覆盖未跟踪 Markdown 的链接、记录解析及内部 whitespace/diff/topic 检查；显式主题集合检查也为 exit 0。
- Scope: ISO-ARCH-R1-F1–F4、doctor 修复和新增隔离验收契约；重点检查所有权交接及路径规范化。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R1-F2 — P1 — STILL_OPEN：复核与接管之间仍无同步边界，依赖的 state.lock 生命周期也不成立。**

- 位置：[architecture.md](../architecture.md#状态访问守卫) 第 133–142 行、[交接前提](../architecture.md#交接前提) 第 234–241 行；tasks.md 第 92–97 行。
- 行为风险：接管完成后，旧进程仍可能使用已经通过守卫或已打开的句柄访问真实状态，违反 G9/A9。轮次复核只限制下一轮，不能撤销已经开始的一轮。
- 证据：允许的时序是 B 的 lsof 未发现 A → Hook 启动 A → A 的守卫读到旧回执并成功打开 → B 写入新回执 → A 继续单次操作。第 239 行仅声称“新进程会读到新回执”，没有处理 A 在写入前已通过守卫的情况。原源码 `internal/store/store.go:376-399` 在 openAtExistingRoot 返回时 defer 释放锁，返回的 Store 不保有该锁；第 331–346 行的 OpenReadOnly 也没有 AcquireLock。因此第 142 行“单次操作持有 state.lock，与新拥有者天然串行”是错误的当前实现前提，且锁即使延长也不能自行解决所有只读/Swift 文件访问和交接写回执的竞态。
- 💡 修复建议：给回执切换、访问资格检查及已有访问的生命周期定义同一可验证的同步/停机边界，覆盖读取、写入、worker 与 App；不能只把 lsof 与写回执相邻执行。明确当前锁是否需要变更及范围，并用受控交错测试覆盖上述顺序。保留用户选择的“随时复核 + 停机交接”，不要把目标降成“最多再执行一轮”。
- 已修复部分：删除永久缓存、长驻每轮复核、App 不跨轮缓存。上述改进不构成整个旧发现的关闭。

**ISO-ARCH-R2-F1 — P1 — NEW / OPEN：先 Clean 再解析符号链接会把实际指向真实根的路径判到根外。**

- 位置：[architecture.md](../architecture.md#什么算指向真实状态目录) 第 101–116 行。
- 行为风险：GuardStateRoot 只返回 error，调用者仍可把原始路径交给文件系统；词法归一化后的安全判断与真实访问目标不一致，可能越过保护。
- 证据：临时夹具中 `outside/jump → account/inner`，真实根为 `account/.agentdeck`。原始路径 `outside/jump/../.agentdeck/sentinel` 经操作系统解析实际读取真实根里的 sentinel；Go 1.27.3 探针的 ReadFile 返回 `protected <nil>`。同一输入先 Clean 后变成 `outside/.agentdeck/sentinel`，EvalSymlinks 尝试的也是 outside 路径并报不存在；按本设计最长已有前缀加回剩余分量的规则，该不存在后缀被拼回根外路径，既不匹配物理保护根，也不匹配词法保护根。比较 candidate 的 Clean 形式不能补救，错误正发生在 Clean。
- 💡 修复建议：在消除 `..` 前保留并解析符号链接的物理路径语义，或以等价且明确拒绝歧义路径的方式保证判定目标与实际操作目标一致。加入“链接后接 `..` 返回真实根”的组合反例；分别处理缺失尾部、链接循环和失败关闭。
- 反例只使用本轮临时目录，没有读取用户真实状态。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

- **ISO-ARCH-R1-F1 — CLOSED。** 第 44–54、224–228 行移除 keg 自证，所有进程比较同一份用户回执；稳定 opt 入口在判定时解析，保留旧 keg 不再各自成为拥有者。并发切换属于仍开放的 F2。
- **ISO-ARCH-R1-F3 — CLOSED。** 旧反例 `account/.agentdeck → moved-state` 的入口和目标现在都纳入保护集合，第 111、123 行明确覆盖。新 R2-F1 是另一种组合路径缺陷，不把已修好的旧反例继续标成失败。
- **ISO-ARCH-R1-F4 — CLOSED。** 第 252–260 行明确不写安装拒绝记录、不扩展 schema_ahead 结构，改用只读 installation/managed_command_owner 诊断。

#### 📝 总结

原 4 项发现中 3 项关闭、F2 仍开放；新增 ISO-ARCH-R2-F1。doctor 入口矛盾已随 CLI 修复；新增访问屏障与规范化过程的矛盾由 [任务复评](tasks.md#round-2--2026-11-05) 的 ISO-TASKS-R1-F2 承载，架构对应段落须一并调整。

Evidence: CodeGraph 在该 worktree 定位 OpenReadOnly 与 managedAgentDeckExecutable；复用同一 HEAD 下已经读取的 openAtExistingRoot 锁释放源码。Go 标准库探针固定使用 Go 1.27.3，验证原始路径与 Clean 后解析的差别；探针和缓存随 TemporaryDirectory 清理。首次默认 go 调用有 1.27.1/1.27.3 工具链混用，未产生验证结果；固定工具链后构建成功。当前环境的嵌套 sandbox-exec 返回 `sandbox_apply: Operation not permitted` / exit 71，未观察到修复记录所称的 SIGKILL；不将这次不可执行当作产品失败或通过。本轮无产品测试、真实安装或真实状态操作。

修复范围：ISO-ARCH-R1-F2、ISO-ARCH-R2-F1，并同步任务记录中仍开放的访问屏障约束；其余旧发现保持关闭。

## Repair 2 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-ARCH-R1-F2（复评仍未关闭）、ISO-ARCH-R2-F1，并同步 tasks 记录中 ISO-TASKS-R1-F2 对应的架构部分。只修改 `architecture.md`。
- Beads：`ad-installed-state-ownership-doc-arch` 由 claude-code 领取修复。
- 保留用户选择的"随时复核 + 停机交接"，没有把目标降成"最多再执行一轮"。

### ISO-ARCH-R1-F2 处置：已修复，待复评

复评指出两点：检查和写回执之间没有同步边界；我在 Repair 1 中依赖的"单次操作持有 `state.lock`"在现有代码里不成立。第二点已在 HEAD 上核对属实：`openAtExistingRoot` 在返回时就释放了锁（`store.go:376-399`），`OpenReadOnly`（`:331`）完全不取锁。

修复：新增 [所有权锁与交接](../architecture.md#所有权锁与交接)，作为唯一的同步边界。

- 锁文件为 `account/Library/Application Support/AgentDeck/ownership.lock`，位于真实状态目录之外，用 `flock(2)` 咨询锁，进程退出时由内核释放。
- **共享锁**：已安装进程在第一次访问真实状态之前取得，在锁内判定身份，一直持有到对真实状态的全部访问结束。覆盖的访问方包括单次命令、Hook、`OpenReadOnly`、scan worker 和长驻进程（按轮或跨轮持有）。App 在整个运行期间持有，因为需求 G9 写明"已安装的 App 仍在运行时，接管会被拒绝"。
- **排他锁**：所有写回执的路径（cask、`make install`、本地 App 安装、`install claim`）都通过新程序的隐藏命令 `install register`，在排他锁内检查并写入回执；5 秒内取不到排他锁就以 `owner_running` 拒绝。安装脚本自己不写回执，因为 macOS 没有自带 `flock(1)`。
- 删除 Repair 1 中"`lsof` 检查后紧接着写回执"的做法，以及"`state.lock` 串行"的说法；`state.lock` 的语义保持不变。

对照复评给出的时序（B 检查时没看到 A → Hook 启动 A → A 读到旧回执并打开 → B 写新回执 → A 继续）：

- A 先拿到共享锁：B 的排他锁要等 A 的整个会话结束，超过上限就失败；
- B 先拿到排他锁：A 在取共享锁这一步等待，之后读到新回执，在访问之前被拒绝。

无论哪种顺序，读路径、写路径、worker 和 App 都被同一把锁覆盖。

残余风险：macOS 的 `flock` 没有写者优先，Hook 调用频繁时，接管可能拿不到锁而以 `owner_running` 失败。结果只会是拒绝接管，不会出现两个拥有者。已写入风险表。

本修复没有改动需求。G9 原文"真实状态的锁仍被持有、或已安装的 App 仍在运行时，接管会被拒绝"，由所有权锁和 App 全程持锁这两点满足。

### ISO-ARCH-R2-F1 处置：已修复，待复评

修复：[什么算指向真实状态目录](../architecture.md#什么算指向真实状态目录) 改为按 POSIX 物理语义逐分量解析的 `walk`：

- 在解析符号链接之前不做词法 `Clean`；
- `..` 作用在已解析的物理前缀上，结果与操作系统一致；
- 每取一个分量，先与受保护集合 `P` 比较，命中就立即拒绝，不再访问它；
- 遇到缺失路径之后，剩余部分不能再有 `..`；链接展开次数有上限，超过按链接循环处理；出错一律拒绝（新增原因码 `state_path_unresolved`）。

`GuardStateRoot` 改为返回物理路径，调用者此后只能使用这个返回值，所以判定的路径就是实际访问的路径。

对照复评反例：`outside/jump → account/inner`，候选为 `outside/jump/../.agentdeck/sentinel`。解析时，`jump` 先变成物理路径 `account/inner`，`..` 得到 `account`，下一个分量 `account/.agentdeck` 命中 `E`，在访问之前就被拒绝。架构的反例清单已加入这一条，以及缺失路径之后有 `..`、链接循环两条。

`P` 的计算也改用同一套物理解析：入口 `E`，加上它的符号链接链上的每个入口和最终目标 `T`。计算时对这些入口只做 `Lstat` 和 `Readlink`。

### ISO-TASKS-R1-F2 的架构部分

见 tasks 记录的 Repair 2。架构中的 [测试与验收的隔离](../architecture.md#测试与验收的隔离) 按以下几点改写：

- 明确区分"允许读取入口元数据"和"禁止访问入口之下的内容"；
- 屏障配置增加 `(allow file-read-metadata (literal …))`；
- 增加正向对照，证明守卫能在屏障内完成判定；
- 嵌套沙箱返回 exit 71 时，测试按失败处理；这类测试要在没有外层沙箱的环境中运行。

### 其他同步

- [doctor](../architecture.md#doctor) 和 [App 与小组件](../architecture.md#app-与小组件)：App 改为全程持有共享锁，但仍然每轮查询 `state-location`。
- 兼容性一节写明：新增 `install_claim_refused` 错误码和两条隐藏命令；`installation.json` 与 `ownership.lock` 不属于状态目录，也不在便携备份范围内。
- 原先的"交接前提"一节已被替换。在 [所有权锁与交接](../architecture.md#所有权锁与交接) 中保留了 HTML 锚点 `交接前提`，这样 Repair 1 和 Round 2 记录里指向它的历史链接仍然有效，无需改动这两份记录。

### 验证

- `make check-whitespace`、`git diff --check`、`python3 scripts/check-docs.py`（读取它自己的退出码）、`bash scripts/check-topic-docs.sh installed-state-ownership`：都是 exit 0。`check-docs.py` 在本轮先发现了这个失效锚点，加上兼容锚点后通过。
- 另外核对了四份文档、全部评审记录、原型 README 中的本地链接和锚点，全部有效。
- **访问屏障可行性**：2026-11-05 在本机、没有外层沙箱的环境中，用夹具 `acct/.agentdeck → moved` 验证：
  - 对入口 `lstat`、`readlink`，对目标 `stat`，都放行（exit 0）；
  - 列出目标目录、`stat` 目录内文件、经入口读取、在目录内写入，进程都被终止（exit 137）；
  - 访问无关目录不受影响。

  这只验证了屏障配置本身，守卫尚未实现，所以没有声称做过正向对照。验证用的临时目录已删除。
- 新引用的源码位置都在 HEAD 上核对过：`OpenReadOnly` 从 `store.go:331` 开始，经 `acquireReadOnlySQL` 打开且不取 state 锁；`openAtExistingRoot` 中 `defer` 释放锁的代码在 `:386-399`。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `26e2708992df7643dd9a3ce5c1ab87584d73f5f7`；指纹 `2a836c6251aad9d3cd6f404eb73845b66f43c10acbed6eda130f22ca03a795dd`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 3 — 2026-11-05

### 📋 架构复评报告

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 claude-code Repair 2，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `26e2708992df7643dd9a3ce5c1ab87584d73f5f7`；fingerprint `2a836c6251aad9d3cd6f404eb73845b66f43c10acbed6eda130f22ca03a795dd`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r3-20261105-l2xNcW-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮精确态查询为 FAILED；review 失败证据与 L0 通过证据均适用，历史状态未被覆盖；invalidated_evidence 和 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0。前者含空白、diff、链接、文档集合和记录结构检查；结构通过不代表语义通过。
- Method / Scope: 复查 ISO-ARCH-R1-F2、ISO-ARCH-R2-F1，以及新所有权锁、App 持锁和入口元数据边界。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R1-F2 — P1 — STILL_OPEN：formula 的 opt 升级可绕过排他锁转移实际所有权。**

- 位置：[architecture.md](../architecture.md#安装身份判定) 第 48–55 行、[所有权锁](../architecture.md#所有权锁与交接) 第 176–185 行、第 298、320 行。
- 行为风险：旧 formula 进程仍持有访问会话及真实状态句柄时，brew upgrade 改变 opt 指向；新二进制可同时取得共享锁并被 Resolve 认作新拥有者，旧访问未结束就发生所有权切换。
- 证据：本方案锁住的是 register/claim 对回执的写入；formula 安装/升级不调用 register，回执登记的 opt 链接却会随升级改变目标，并在每次 Resolve 时重新解析。临时夹具以真实 flock 取得 A 的共享锁，在没有排他锁的情况下将 opt 从 keg A 原子切到 keg B；B 的独立 fd 仍成功取得共享锁且解析为当前拥有者。输出的 `old_session_still_holds_shared_lock`、`old_was_owner_at_session_start`、`opt_changed_without_exclusive_lock`、`new_session_acquired_shared_lock`、`new_resolves_as_owner` 五项均为 true。
- 💡 修复建议：把能改变回执所指对象的所有事件纳入同一个交接边界，包括 formula 升级/链接变更；或固定不可变安装对象并通过明确受锁保护的升级登记切换。不能仅保护 JSON 文件写入，却把可由其他安装操作改变的链接目标作为实时权威。补充“旧只读/写会话持锁期间 opt 切换”的受控测试。
- 已修复部分：普通 register/claim 的共享/排他互斥、访问句柄全生命周期及旧 state.lock 前提均已纠正；旧发现的完整生命周期要求仍未满足。

**ISO-ARCH-R3-F1 — P2 — NEW / OPEN：App 启动顺序让未安装的 Debug App 也持续持有真实所有权锁。**

- 位置：[App 与小组件](../architecture.md#app-与小组件) 第 358–363 行；[tasks.md](../tasks.md#6-app-state-location) 第 188–189 行。
- 行为风险：从 DerivedData 启动、最终只使用隔离状态的 Debug App 仍会阻塞正式安装、卸载或接管；用户没有已安装访问会话，也会收到 owner_running。这与 Debug App 独立和“仅真实状态拥有者的会话阻止交接”的边界不一致。
- 证据：当前明确顺序是 App 启动就取同一个 account 下的共享锁并持有到退出，然后才调用 helper 的 state-location 得知 installed=false。没有非拥有者释放或跳过持锁的分支。排他锁与共享锁互斥，所以即使后续只访问隔离目录，register 也必定等到超时。任务 6 直接继承这一无条件顺序。
- 💡 修复建议：明确只有通过锁内身份复核、确实服务于真实状态的 App 才保持长期共享会话；未安装 App 只能短暂参与必要的身份检查，确认不拥有真实状态后释放，使用隔离路径。补上“Debug App 单独运行时安装/接管仍可完成”和“已安装 App 仍阻止接管”的区分测试。
- 这是新增所有权锁的访问者范围问题，不要求修改 Debug bundle/App Group 的既定选择。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-ARCH-R2-F1 — CLOSED。** 第 100–126 行改为逐分量物理解析，链接展开前不 Clean；碰到保护入口先拒绝，调用者必须使用返回的物理路径。旧 `outside/jump/../.agentdeck` 反例按新规则在 E 处拒绝，原先判定路径与操作路径不一致的问题关闭。

ISO-ARCH-R1-F1/F3/F4 保持 CLOSED。新锁在真实状态根之外，普通 register/claim 与读写会话使用同一锁，正确撤回了依赖 state.lock 的错误前提。入口 metadata 与内容读取也已分开，原屏障矛盾由任务记录关闭。

#### 📝 总结

本文件上一轮两项开放发现中，R2-F1 关闭，R1-F2 仍开放；新增 R3-F1。只证明互斥锁本身正确不足以覆盖没有取锁的 opt 变更和不应持锁的 App。无需重跑不相关产品全套检查。

Evidence: 仅用临时 Cellar/opt、两个真实 flock 文件描述符构造协议反例；未执行 brew upgrade 或操作本机安装。首次夹具未归一化 macOS 临时路径别名，身份比较无效，未采用其结果；将夹具根及双方可执行路径均 resolve 后，五项结果全部为 true。两个夹具均在退出时清理。App 问题由文档规定的执行顺序和同一把锁的互斥语义直接确定，不声称已运行未来 Swift 实现。

修复范围：ISO-ARCH-R1-F2、ISO-ARCH-R3-F1；同步受影响的安装、升级和 App 任务。保持其余已关闭发现。

## Repair 3 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-ARCH-R1-F2（仍未关闭）、ISO-ARCH-R3-F1。只修改 `architecture.md`。
- 用户决定（2026-11-05，在本次修复中由用户明确选择）：formula 升级后，采用**受锁保护的自动续登**，不要求每次升级后手动认领。
- Beads：`ad-installed-state-ownership-doc-arch` 由 claude-code 领取修复。

### ISO-ARCH-R1-F2 处置：已修复，待复评

根因与复评一致：Repair 2 让回执登记 opt 链接并在判定时实时解析，而 opt 的指向可以被 `brew upgrade` 在锁外改变，于是"回执所指的对象"可以不经排他锁就被换掉。

修复的原则是：**回执所指的对象，只能在所有权排他锁内被改变。**

- [安装身份判定](../architecture.md#安装身份判定)：回执只登记写入时已经解析好的不可变真实路径；判定时不再解析任何链接，直接与 `self` 比较。formula 登记 keg 内路径 `<prefix>/Cellar/<formula>/<version>/bin/agentdeck`。这样 opt 切换不再改变回执的含义。删除被登记的文件，只会让回执过期、减少拥有者，不会产生新的拥有者。
- [安装回执](../architecture.md#安装回执)：任何会替换"已登记路径上的文件"的安装操作，都按"撤销（排他锁内的 `install unregister`）→ 替换 → 重新登记"进行。覆盖的情形有 `make install FORCE=1`、覆盖已登记的同一个本地 App、cask 升级。cask 升级依赖 Homebrew 先执行旧版本的 uninstall 步骤；这一点列为必须在真实 brew 回归测试中验证的交付条件，验证失败就回到设计。
- 新增 [formula 升级续登](../architecture.md#formula-升级续登)（用户决定）：新 keg 首次访问真实状态时，若四个条件全部满足，就在排他锁内续登。四个条件是：回执为 formula、同一前缀、同一 formula、旧 keg 已被删除，并且自己是 opt 当前指向的 keg。在锁内要重读回执、再次确认条件后才写入。
  - 旧 keg 仍存在时不续登，需要 `install claim --takeover`；
  - 跨 formula、跨前缀时也不续登。

对照复评的五项反例（旧会话仍持有共享锁、旧会话开始时是拥有者、opt 在锁外被切换、新会话取得共享锁、新会话被判为拥有者）：

- 在新设计下，opt 切换不改变回执；旧 keg 仍存在，B 不满足续登条件，所以第五项"新会话被判为拥有者"不成立，B 在访问之前被拒绝。
- 如果旧 keg 被删除，B 要等到 A 的会话结束、拿到排他锁才能续登；删除之后，A 的残留进程在下一次过守卫时以 `executable_unresolved` 被拒绝。

[tasks.md](../tasks.md#2-state-root-guard) 中，这个反例已列为任务 2 的第 6 条交错测试，续登和替换分别为第 7、8 条。

### ISO-ARCH-R3-F1 处置：已修复，待复评

- [所有权锁与交接](../architecture.md#所有权锁与交接) 的访问方表格把 App 拆成两行：只有服务于真实状态的 App 才持有长期会话；未安装的 App（例如 Debug App）不取锁。
- [App 与小组件](../architecture.md#app-与小组件) 改为以下顺序：
  1. 先查询 `state-location`，不持锁；
  2. 只有返回 `installed: true` 并且 `state_root` 指向真实状态时，才取共享锁，并在锁内再查询一次，以防取锁前所有权已经转移；确认后持有到 App 退出；
  3. 之后任何一轮发现条件不成立，就立即释放锁，改用隔离目录。

结果是：Debug App 从不阻塞安装、卸载或接管；已安装的 App 仍然阻止接管，满足需求 G9。Debug bundle ID 和 App Group 的既定选择没有改动。

### 验证

- `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`make check-whitespace`、`git diff --check`：都是 exit 0。
- 新引用的源码位置已在 HEAD 上核对：缺失目录短路位于 `internal/doctor/doctor.go:72-76`，设置 `Partial=true` 后返回。
- 本轮的设计改动都是文档层面的约束，没有运行任何产品行为；续登、替换、交错测试都写进了任务要求，没有声称已执行。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `42d6748c1a30d9a184582058ce2f4de2d255f5e1`；指纹 `4965a0bbea24eb2e1140e32441d64c7042febd7c320f57206981115eb59f4bf2`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 4 — 2026-11-05

### 📋 安装交接与续登复评报告

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code 的 Repair 3，未委派，未修复设计、产品代码或标本。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `42d6748c1a30d9a184582058ce2f4de2d255f5e1`；fingerprint `4965a0bbea24eb2e1140e32441d64c7042febd7c320f57206981115eb59f4bf2`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r4-20261105-QC3iyb-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮固定 gate-status 对精确目标返回 FAILED；本轮 review 与 L0 证据均适用，无失效证据或未决影响。历史内容状态和结论保留。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者覆盖空白、diff、链接、文档集合和记录结构。
- Method / Scope: ISO-ARCH-R1-F2、ISO-ARCH-R3-F1，以及 Repair 3 新引入的撤销/替换/重新登记与自动续登顺序。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R4-F1 — P1 — NEW / OPEN：撤销后释放排他锁，不能保证替换期间没有新拥有者。**

- 位置：[安装回执](../architecture.md#安装回执) 第 301–309 行；任务 2 第 8 条交错测试、任务 5 的原地替换要求。
- 行为风险：两个安装器并发操作同一安装路径时，第一个安装器可能在第二个已经登记、且正在使用真实状态之后覆盖该路径，重新出现“回执不变但实际安装对象改变”的未串行切换。
- 证据：协议由暂存新程序运行 unregister 后结束该进程，排他锁随之释放；文件替换发生在下一步，register 才再次取锁。具体允许时序：安装器 A 撤销 P 并释放锁 → 安装器 B 在无回执窗口安装 P、在排他锁内登记 P → B 的客户端取得共享锁并打开 → A 执行自己的替换步骤，未持排他锁。临时 flock/文件句柄模型得到：替换时回执存在且仍登记 P，B 会话仍持共享锁，B 已打开句柄读到内容 B，新路径读到内容 A。第 306 行“此时没有回执，没有任何进程是拥有者”在这个合法交错中不成立。
- 💡 修复建议：把撤销、覆盖/删除文件及最终登记放在同一个持续有效的交接事务或等价保留机制中，阻止其他登记者在中间重新取得 P；明确失败清理和安装器间的排他关系。补充两个安装器针对同一路径的受控交错，不能仅测试单个安装器面对旧共享会话。
- 这是 Repair 3 新增替换协议的缺陷；旧 opt 反例的处置见下文，不把已修好的旧反例继续标为失败。

**ISO-ARCH-R4-F2 — P2 — NEW / OPEN：默认状态目录在续登之前被选为隔离根，自动续登入口不可达。**

- 位置：[状态目录解析](../architecture.md#状态目录解析) 第 90–103 行、[状态访问守卫](../architecture.md#状态访问守卫) 第 153–157 行、[formula 升级续登](../architecture.md#formula-升级续登) 第 338–351 行。
- 行为风险：升级新 keg 的普通命令不带 --state-dir 时，仍会使用空的隔离状态；用户选择的“无需再次认领、自动续登后继续使用原数据”不能按当前调用顺序发生。
- 证据：旧 keg 已删除、回执保留且续登四条件成立时，Resolve 先得到未安装。未传 override 的 StateRoot 按表选择 `$HOME/.agentdeck-dev`；GuardStateRoot 对不命中 P 的路径直接返回，不判定身份、不取锁。续登只被放在“路径命中 P，且自己不是拥有者”的分支。按这些分支执行的模型结果为 `four_renewal_conditions_met:true, resolved_installed:false, default_root:".agentdeck-dev", renewal_attempted:false`。显式指向真实根能触发，不等于默认调用会触发。
- 💡 修复建议：明确续登资格检查、排他续登、默认根选择之间的顺序，保证无 --state-dir 的符合条件新 keg 能走到续登；同时保留显式隔离 override 的意图。增加普通默认命令的端到端场景，而不只测试直接调用 GuardStateRoot 或显式真实根。
- 这是新自动续登功能的连接缺口，不要求恢复不受锁保护的 opt 权威。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

- **ISO-ARCH-R1-F2 — CLOSED。** 本轮把 formula 回执改为写入时固定的真实 keg 路径，opt 切换不再改变回执含义；旧 keg 删除后的切换明确等待排他锁，并在锁内重读条件。Round 3 的“opt 在旧共享会话期间切换，新 keg 直接被认作拥有者”反例已被排除。新替换协议的问题使用 R4-F1 单独记录。
- **ISO-ARCH-R3-F1 — CLOSED。** 第 413–418 行先查询身份，只有 installed=true 且服务真实根才取得长期共享锁，并在锁内再次确认；条件不成立时不取锁或释放。任务 6 第 200–205 行明确覆盖 Debug App 不阻塞安装和已安装 App 阻止交接，原发现关闭。
- 更早的路径解析、单回执和 Hook 记录发现均保持 CLOSED。

#### 📝 总结

上一轮本文件两项开放发现均已关闭；新增 R4-F1（P1）、R4-F2（P2）未关闭，故 FAIL。用户选择的受锁保护自动续登保留，当前缺陷在事务跨度和入口顺序。

Evidence: 临时模型使用真实 flock、回执文件和原子路径替换，完整时序及输出已写入发现；默认根模型逐条执行文档规则。模型只验证设计逻辑，不是未来产品实现测试。临时目录在脚本退出时清理，未触碰用户状态或安装。已有决定性反例后不继续扩大产品验证。cask 升级回调顺序仍是文档明确列出的实现阶段交付条件，没有将尚未执行的真实 brew 验证记为 PASS。

修复范围：ISO-ARCH-R4-F1、ISO-ARCH-R4-F2，并同步受影响的任务。

## Repair 4 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-ARCH-R4-F1、ISO-ARCH-R4-F2。修改 `architecture.md`，并同步 `tasks.md`。已通过的 `requirements.md` 和 `ux/cli.md` 都没有改动：本轮没有新增原因码，也没有改变 CLI 表面。
- Beads：`ad-installed-state-ownership-doc-arch` 由 claude-code 领取修复。

### ISO-ARCH-R4-F1 处置：已修复，待复评

确认属实：Repair 3 的"撤销"由一个暂存进程执行，进程结束时排他锁随之释放；在替换之前，另一个安装方式可以登记同一路径，并开始使用。

修复：把"撤销"改为**预留**，让保留状态跨越整个替换过程，而不要求排他锁跨步骤持有。见 [安装回执](../architecture.md#安装回执) 和 [回执格式](../architecture.md#回执格式)。

- 回执文件新增 `state` 字段，取值为 `installed` 或 `reserved`。预留记录带有 `reserved_until`，为预留后 10 分钟。
- 流程变为"预留 → 替换 → 完成"：
  - 预留：在排他锁内把回执改写为预留。要等旧会话全部结束才能拿到排他锁，所以这一步同时完成了交接。
  - 替换：预留期间没有任何拥有者，判定为 `receipt_missing`。其他登记同一路径的请求，一律拒绝，`--takeover` 也不行；登记别的路径时，没有 `--takeover` 以 `owned_by_other` 拒绝；有 `--takeover` 且被预留的路径仍存在时，以 `owner_running` 拒绝；被预留的路径已不存在时，`--takeover` 可以清除预留。
  - 完成：`install register` 在锁内确认预留仍有效、路径和安装方式一致，再改写为正式回执；预留不在了，登记就失败，不会覆盖他人的回执。
- 失败清理：替换失败时，用 `install unregister` 删除自己的预留；安装方式崩溃时，预留到期自动失效。
- cask 的 uninstall 步骤分不清是升级还是卸载，所以一律执行预留。真正卸载时，被预留的路径已不存在，其他安装方式可以用 `--takeover` 立即清除；否则 10 分钟后过期。
- 没有新增原因码，沿用 CLI 契约中已有的 `receipt_missing`、`owned_by_other`、`owner_running`，所以 `ux/cli.md` 保持不变。

对照评审的反例时序（A 撤销后释放锁 → B 在空窗中登记同一路径并开始使用 → A 替换），在新协议下：B 的登记在锁内看到 A 的有效预留，被拒绝；在 A 完成之前，这条路径上没有任何拥有者会话。

用真实 `flock` 和回执文件在临时目录中做了模型验证，按评审时序逐步执行，输出如下：

```text
{"owner_during_reservation": false, "B_register_same_path": "refused:same_path_reserved", "B_register_same_path_takeover": "refused:same_path_reserved", "B_other_path_no_takeover": "refused:owned_by_other", "B_other_path_takeover_P_exists": "refused:owner_running", "A_complete": "completed", "owner_after_complete": true}
```

这个模型只验证了设计规则，不是未来产品实现的测试；临时目录已删除。

### ISO-ARCH-R4-F2 处置：已修复，待复评

确认属实：Repair 3 只在守卫的"命中真实状态"分支里续登，而不带 `--state-dir` 的普通命令在此之前就已经选了隔离目录，根本走不到续登。

修复：见 [状态目录解析](../architecture.md#状态目录解析)。没传 override 时，`StateRoot` 按以下顺序执行：

1. `Resolve`；
2. 未安装时，检查续登条件（只读回执、opt 链接和自身路径，不访问 `P`）；
3. 条件成立，就执行续登，然后重新判定：成功就使用真实默认目录；失败就返回 `state_ownership_refused`，不悄悄改用空的隔离目录；
4. 条件不成立，才使用隔离目录。

[formula 升级续登](../architecture.md#formula-升级续登) 的触发时机改为两个入口，共用同一个续登函数：

- 默认目录解析：普通命令、Hook，以及不再传 `--state-dir` 的 App helper，都走这个入口；
- 显式指向真实状态时的守卫分支。

显式指向非真实目录时不续登，以尊重隔离意图。

对照评审的模型（`four_renewal_conditions_met:true, resolved_installed:false, default_root:".agentdeck-dev", renewal_attempted:false`）：在新顺序下，第 2 步检查条件成立，第 3 步执行续登，默认目录随之变为真实目录，不再出现"满足条件却落到 `.agentdeck-dev`"的情况。[任务 2](../tasks.md#2-state-root-guard) 的第 11 条交错测试，用一条不带 `--state-dir` 的普通命令做端到端验证。

### 验证

- `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`make check-whitespace`、`git diff --check`：都是 exit 0。
- `requirements.md`（`2c4f3e37…`）和 `ux/cli.md`（`5e427a11…`）的 blob 与它们通过时相同。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `2864e14e0c970bc996e7f5fb8779a034b7b649cd`；指纹 `36ac258191a5b231cdde1e8534d1d2290d6a299c1f4b195836602d8f79132297`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 5 — 2026-11-05

### 📋 预留事务与默认续登复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code Repair 4，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `2864e14e0c970bc996e7f5fb8779a034b7b649cd`；fingerprint `36ac258191a5b231cdde1e8534d1d2290d6a299c1f4b195836602d8f79132297`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r5-20261105-0wyFKc-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮固定 gate-status 对精确目标返回 FAILED；review 失败与 L0 通过证据均适用，历史状态保留，无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者含空白、diff、链接、文档集合和记录结构检查。
- Method / Scope: ISO-ARCH-R4-F1/F2，预留记录、完成/取消资格、到期恢复及默认根选择。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R4-F1 — P1 — STILL_OPEN：预留没有可区分的事务身份，过期也没有隔离仍能写入的旧安装器。**

- 位置：[回执格式](../architecture.md#回执格式) 第 75–89 行；[安装回执](../architecture.md#安装回执) 第 318、322–336 行。
- 行为风险：预留并未结构性保证“原安装器完成前，别人不能重新登记并访问该路径”；原地替换仍可覆盖已有活跃拥有者。单纯检查最终登记失败不能撤销已发生的文件覆盖。

**1. 完成与竞争请求不可区分。**

- 第 325 行要求其他安装器登记同一路径一律拒绝，第 326 行却允许“预留有效、路径和 channel 一致”的 register 完成。两者调用的都是 `install register --channel make --executable P`。
- 预留记录只含 state/path/channel/timestamps，没有事务持有者或代际凭据；命令也不携带预留身份。两个同时安装同一 channel、同一路径的安装器，在协议层给出的输入完全相同，无法一个放行、一个拒绝。
- 取消也只按路径删除（第 318、334 行），同样没有“这是自己的预留”的可验证条件。Repair 4 模型把 A_complete 和 B_register 当作不同操作，但没有定义实际命令如何获得这种区别。
- 独立模型确认 `A_completion_and_B_competing_request_identical=true`，`receipt_has_transaction_owner_or_token=false`，`completion_predicate_path_channel_equal_for_both=true`。

**2. 时间到期不能证明原安装器不再执行。**

- 第 83、335 行规定 10 分钟后按不存在处理；第 336 行只要求最终完成时检查预留，没有要求旧安装器在预留被回收后停止实际替换。
- 合法时序：A 预留后暂停在替换前，超过 10 分钟但没有崩溃 → B 按过期规则登记 P，并启动持共享锁的会话 → A 恢复，执行第 2 步的文件替换。A 后续 register 即使失败，B 正在使用的安装文件已经被覆盖。
- 虚拟时钟推进到 601 秒的临时 flock/文件模型得到：B 已在过期后登记同一路径，B 的共享锁仍持有，回执仍匹配被替换路径；旧句柄内容为 B-active-image，新路径为 resumed-A-image。未真实等待 10 分钟，也未操作本机安装。
- “到期只用于崩溃恢复”没有附带区分崩溃与暂停的机制，不能作为安全保证。任务中的过期测试只覆盖 A 已崩溃，遗漏仍能恢复执行的情况。

💡 **限定修复建议**：定义跨 reserve/register/unregister 的同一事务身份与完成、取消资格；并保证预留被回收后，旧安装器不能继续发布或覆盖目标文件。实际文件发布与有效预留校验必须受同一可执行互斥边界保护，或采用能提供等价保证的方案。增加“同 channel/同 path 的两个安装器”和“预留到期后原安装器恢复”的交错测试。仅延长 TTL、增加完成后的检查或让模型预先知道谁是 A，均不足以关闭该发现。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-ARCH-R4-F2 — CLOSED。** 第 103–116 行明确将续登资格检查和排他续登放在默认根选择之前：满足条件时先续登、再重新判定并选真实根；失败返回 state_ownership_refused，不退回空隔离根；显式隔离 override 不触发续登。任务 2 第 11 条和任务 8 第 6 条明确使用不带 --state-dir 的普通命令验证。按新顺序执行的合同模型得到一次续登尝试及默认根 .agentdeck，旧不可达路径已消除。

此前已关闭的 opt 身份、App 持锁、路径解析及 Hook 记录问题保持关闭。本轮未新增发现编号。

#### 📝 总结

上一轮 2 项中，R4-F2 关闭，R4-F1 仍开放。现有预留能表达“这里正在替换”的意图，但尚不能区分预留持有者、拒绝竞争完成或阻止过期持有者继续改文件，故 FAIL。

Evidence: 直接核对回执允许字段、register/unregister 的参数与判定条件；使用临时回执/可执行文件占位、真实 flock 和虚拟时钟验证允许的交错。模型只验证设计规则，不是未来产品实现测试；所有临时文件在退出时清理。已发现决定性反例后停止扩大验证，没有改产品代码或运行真实安装。最终 L0 与精确态门禁在收口字段登记。

修复范围：ISO-ARCH-R4-F1 的事务身份、有效期回收与文件发布边界，并同步受影响任务和必要的用户可见恢复语义。

## Repair 5 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-ARCH-R4-F1（仍未关闭）。修改 `architecture.md`，并同步 `tasks.md`。已通过的 `requirements.md`、`ux/cli.md` 都没有改动：隐藏命令仍叫 `install register` / `install unregister`，错误码和原因码不变。
- Beads：`ad-installed-state-ownership-doc-arch` 由 claude-code 领取修复。

### ISO-ARCH-R4-F1 处置：已修复，待复评

确认属实，两点都成立：

1. 完成登记和竞争登记的命令、参数完全相同，预留又没有事务身份，协议无法区分两者；
2. 预留到期并不能证明原安装器已经停止，暂停后恢复的原安装器仍会改写目标文件。

**修复方向：取消跨步骤的中间状态，而不是给它补身份和凭据。** 新规则：在可能被登记或已被登记的路径上创建、替换或删除文件，只能由**当前持有所有权排他锁的那个进程**来做，并且同一个进程在释放锁之前更新回执。见 [安装回执](../architecture.md#安装回执)。

- `install register` 新增 `--source`。暂存的新程序在**一次连续持有**的排他锁内，依次完成：检查回执 → 原子发布（文件用 `rename(2)`；App bundle 用 `renamex_np(RENAME_SWAP)` 交换后删除旧的）→ 写回执。任何失败都不改目标路径，也不改回执。
- `install unregister` 新增 `--remove`，在同一次持锁内删除回执和文件。
- Repair 4 的预留记录、`state` 字段、`install reserve` 和 10 分钟过期，全部删除。回执格式恢复为只有一种记录。
- **cask 不再使用 `app` 标签**，因为 Homebrew 会在我们的进程之外移动 App，无法纳入这把锁。改为 install step 以 `--source` 发布 App；uninstall 步骤先退出 App，再执行 `install unregister --remove`。不带 `app` 标签的 cask 能否正常安装、升级、卸载、注册小组件，以及升级时 uninstall 是否先于 install step 执行，都列为真实 brew 回归测试中的交付条件；任一项不成立，cask 部分就回到设计重新决定。
- 唯一允许在锁外发生的改动，是 Homebrew 删除旧的 formula keg。删除只会让回执过期、减少拥有者，不会产生新的拥有者；这一点此前已经关闭的 ISO-ARCH-R1-F2 处置中已有说明。
- 名称、错误码、原因码都没有变化，所以已通过的 CLI 契约不受影响。

**对照两个反例：**

1. **相同参数的两个安装器**：两者都必须先取得排他锁，后进入的只能在前一个完成发布和登记、释放锁之后才动手；它看到的是正式回执，覆盖也只发生在它自己持锁、没有任何会话的时候。所以不需要区分身份。
2. **暂停后恢复的安装器**：发布只发生在持锁期间。持锁时被暂停，锁一直被它持有，其他安装器和客户端会话都进不来；还没取锁时被暂停，恢复后必须先取锁，会等到别人的会话结束。没有任何基于时间的回收，所以不存在"过期后别人已登记、旧安装器又改了文件"的窗口。

用真实 `flock`（不同文件描述符）、回执文件和虚拟时钟在临时目录中做了模型验证，输出如下：

```text
{"B_identical_args_enters_while_A_holds": false, "client_session_while_A_holds": false, "A": "published", "client_session_while_B_publishes": false, "B_after_A": "published", "other_channel_same_path": "refused:owned_by_other", "P_content_after": "B-image", "after_601s_B_can_lock": false, "after_601s_client_can_session": false, "resumed_A": "published"}
```

这个模型只验证设计规则，不是未来产品实现的测试；临时目录已删除。

[任务 2](../tasks.md#2-state-root-guard) 的交错测试第 8 至 10 条已按新规则重写：第 9 条为相同参数的两个安装器，第 10 条为暂停跨越 601 秒、锁外暂停和崩溃三种情形；任务 5 也同步了在锁内发布和 cask 的验证条件。

### 验证

- `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`make check-whitespace`、`git diff --check`：都是 exit 0。
- `requirements.md`（`2c4f3e37…`）和 `ux/cli.md`（`5e427a11…`）的 blob 与它们通过时相同。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `d3cdf4bf437d64e89244f8dee3424e0714603260`；指纹 `cf6fe86211ed698a8149bd609bd722f8e2d893f52bc78618b1e709c41ecf4650`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 6 — 2026-11-05

### 📋 连续持锁发布与卸载链复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 claude-code Repair 5，未委派，未修复设计、产品代码或测试。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `d3cdf4bf437d64e89244f8dee3424e0714603260`；fingerprint `cf6fe86211ed698a8149bd609bd722f8e2d893f52bc78618b1e709c41ecf4650`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r6-20261105-ekfE00-state`。
- 完成门禁：FAILED
- Gate evidence: 固定 gate-status 对本轮精确目标返回 FAILED；review 失败与 L0 通过证据均适用，历史状态保留，无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者含空白、diff、链接、文档集合和记录结构检查。
- Method / Scope: ISO-ARCH-R4-F1；连续持锁发布、暂停/恢复、cask 完整卸载链及相关验收条件。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- External source identity: Homebrew/brew HEAD `944ccca3ed69484a9d762bec64f00f334db52182`；`Library/Homebrew/cask/artifact/abstract_uninstall.rb` 当前 blob `ec5c843420d6f81ee333d63b08c622e6a235ffe5`。必要事实在本记录中自包含记载，不依赖临时输出。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R4-F1 — P1 — STILL_OPEN：cask 的 uninstall delete 兜底在锁外删除，绕过刚建立的发布/删除互斥边界。**

- 位置：[architecture.md](../architecture.md#安装回执) 第 305、326、335–342 行，尤其第 339 行。
- 行为风险：旧 cask 卸载已经释放所有权锁后，另一个安装方式可以成功发布并登记同一路径的 App；随后 Homebrew 的兜底删除会删掉这个新拥有者的 App，即使新拥有者已经持有共享会话。结果是新安装被删除、回执仍保留、后续进程无法解析已安装可执行文件。
- 证据一，设计内部存在绕行：第 305 行要求所有创建/替换/删除都只能由持有排他锁的进程执行；第 339 行却保留独立的 `uninstall delete:`，没有进入 `install unregister --remove`。
- 证据二，实际 Homebrew 分派顺序：上述固定源码的 `ORDERED_DIRECTIVES`（第 23 行起）把 `:script` 放在 `:delete` 之前；`uninstall_script`（第 633 行）执行脚本后返回，且有独立的 1 秒等待；`uninstall_delete`（第 671 行）另行执行 xargs/rm 递归删除路径，没有 AgentDeck 的所有权锁或回执归属校验。它不是同一个 helper 进程里的“若仍属于本安装才删除”。
- 证据三，临时模型复现完整顺序：A 在排他锁内注销并删除旧 App → A 释放锁 → B 在排他锁内安装同一路径、登记 channel=local_app → B 开启共享会话 → A 的后续 delete 兜底在锁外执行。观察为 `B_session_still_holds_shared_lock=true`、`B_app_removed=true`、`B_receipt_retained=true`，已打开镜像仍为 local-B。
- 💡 限定修复建议：移除对可被其他安装接管路径的锁外 delete 兜底；让完整卸载、失败回滚和损坏安装清理都经过同一互斥边界并核验当前安装归属，不能在 helper 返回后再无条件删除该路径。对完整 Homebrew 卸载链加入“A helper 结束后，B 同路径发布并登记，再继续 A 清理”的交错测试。若保留任何 fallback，必须证明它不可能删掉后来的拥有者。
- 本项仍是原发现的“全链文件变更须受持续有效互斥保护”，不为同一保证的残留绕行新增编号。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

主 `install register --source` / `unregister --remove` 的连续持锁方案已闭合上一轮两个反例：

- 相同参数的两个安装器由排他锁串行，不再需要预留身份。
- 安装器暂停时锁不因时间失效；锁外暂停者恢复后必须先取锁。预留、TTL 和“过期后旧安装器继续写”的窗口均已删除。

独立真实 flock 模型在 A 持锁期间，B 的排他请求和客户端共享请求都失败；虚拟时钟推进 601 秒后结果不变；A 释放后 B 才能取得排他锁。ISO-ARCH-R4-F2 和更早已关闭的发现保持 CLOSED。

#### 📝 总结

ISO-ARCH-R4-F1 主流程已修复，完整 cask 卸载链仍有明确锁外删除路径，因此整体仍 OPEN / FAIL；无新增发现编号。

Evidence: 只读核对 Homebrew 当前源码、提交和 blob；临时目录中用真实 flock、App 目录/回执模型验证互斥和后续删除。没有实际执行 brew install/uninstall/upgrade，也未触碰用户安装。临时夹具在退出时清理。发布后故障恢复及不含 app 标签的 cask 安装/升级/小组件行为仍是任务中明确的实现阶段验收条件，本轮没有把尚未运行的条件记成 PASS。取得决定性反例后不扩大产品验证。最终 L0 和精确态门禁在收口字段登记。

修复范围：ISO-ARCH-R4-F1 的 cask 锁外删除兜底及完整清理链；同步对应卸载/回滚交错测试。保留已经成立的连续持锁主流程。

## Repair 6 — 2026-11-06

- Implementer：OpenCode（GPT-6 Astra），修复角色，未委派，不自判复评 PASS。
- Scope：ISO-ARCH-R4-F1 在 Round 6 中剩余的 cask 锁外删除及完整清理链；修改 `architecture.md`，同步 `tasks.md` 和两个修复记录。
- Workspace：`agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Beads：原文档任务 `ad-installed-state-ownership-doc-arch`、`ad-installed-state-ownership-doc-tasks`；核验上一轮无受让人的修复交接后由 opencode 接受。

### ISO-ARCH-R4-F1 处置：已修复，待复评

确认 Round 6 反例成立：设计保留的 `uninstall delete:` 不在 helper 的持锁区间。重新只读核对固定的 Homebrew/brew 提交 `944ccca3ed69484a9d762bec64f00f334db52182` 中 `Library/Homebrew/cask/artifact/abstract_uninstall.rb`：`ORDERED_DIRECTIVES` 先执行 `script`，后执行 `delete`；前者返回后，后者独立调用 xargs/rm。仓库当前 cask 模板仍是实现基线，尚未实现本主题的新协议，本轮修复对象是架构设计。

修复映射：

1. [cask 的完整安装与卸载链](../architecture.md#cask-的完整安装与卸载链) 移除最终 App 路径的 `uninstall delete:` 兜底；同一约束覆盖 zap、postflight、失败回滚和强制跳过脚本后的剩余步骤。私有暂存清理、链接删除不能跟随到最终 App。
2. [卸载与失败清理](../architecture.md#卸载与失败清理) 将清理统一到持续持有排他锁的 `unregister`，在锁内同时比较原安装的 ID、channel 和 executable。不同 ID 即跳过，包含同 channel、同路径、同版本重装；小组件注销也在这次校验之后执行。
3. [回执格式](../architecture.md#回执格式) 增加每次登记新生成的 `installation_id`；发布前持锁写入本次独占创建的清理清单，安装方式保存原 ID 到卸载/回滚结束。清理时不能从当前回执刷新旧清单。此 ID 只标识清理对象，不授权真实状态访问，不重新引入预留、TTL 或跨步骤发布。
4. 损坏最终 helper 时使用私有保存的清理 helper，仍走同一校验。清单、回执或可信 helper 不足时保留目标、报告未完成，通过重新安装恢复。撤销后删除失败保留残余与恢复材料，不在释放锁后补删；延迟回滚也不能恢复旧 App 覆盖新安装。
5. [任务 1、2、5](../tasks.md) 同步 ID 校验、完整卸载交错、延迟回滚、损坏安装和强制路径验收。原连续持锁主流程、默认续登及其他已关闭发现不变。

### 验证与边界

使用临时目录中的 App 占位文件、回执、清单和真实 `flock`（独立文件描述符）执行设计模型。模型结果：

| 场景 | 观察 |
| --- | --- |
| 旧协议：A 清理返回 → B 安装并持共享锁 → A 的锁外 delete | 复现：B 的 App 被删除，B 回执仍在，已打开句柄仍读到 B |
| 新协议：同一交错，B 分别为 local_app 和 homebrew_cask | 剩余私有清理不改变 B 的 App/回执；旧清理重试在 B 持锁时被阻止，B 退出后按旧 ID 跳过 |
| A 成功登记，B 同 channel 覆盖后再执行 A 的失败回滚 | 跳过 A 的清理，B 的 App/回执和小组件清理调用记录不变 |
| 最终 helper 被删除，但原回执、清单仍匹配 | 清理可完成；再次调用为幂等成功 |
| 回执缺失、回执损坏或清单缺失 | 拒绝清理，保留目标，不执行小组件注销 |
| 撤销回执后注入删除失败 | 保留残余和清单；旧清理不能补删，重新安装产生新 ID 后才由新清单完成清理 |

11 个模型断言全部通过；使用固定相同时间戳并分别生成安装 ID，未以时间或 channel 差异代替安装实例判定。模型夹具退出时清理。这是设计规则及互斥原语验证，不是尚未实现的产品、真实 Homebrew 或小组件验收；任务 5 中的真实 brew 交付条件仍待实现阶段执行。

`python3 scripts/check-docs.py` exit 0，包含文档链接、主题集合、记录结构、空白和 diff 检查。结构检查不签发语义 PASS。`requirements.md` blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`、`ux/cli.md` blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a` 与本轮输入相同：公开命令和标本契约未改；隐藏清理复用既有 receipt 原因码。

### 候选与交接

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`architecture.md` blob `9a5e5a349489c7dacbeefb68ee327c19e7ba7861`；指纹 `6d1e0be4912cf4c0e3b1ec2a04a5b69c791406c075fca5b10d26a6cc788421a3`，配方仍为 SHA-256 of `head=<HEAD>;document=<blob>`。

本轮只交接修复候选，不关闭文档完成边界。Review 保持未勾选，Round 6 的历史 FAIL / FAILED 保留；未为新候选签发复评或 CEv1 通过结论。产品实现、真实安装及 Git 交付均未执行。

## Round 7 — 2026-11-06

### 📋 架构复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；独立复核 OpenCode 的 Repair 6，未委派，未修复架构或产品内容。
- Method: 按 Round 6 的完整卸载链反例检查协议，并用临时文件、真实 flock 和定向故障注入独立验证清理判定。
- Scope: 本架构文档及 ISO-ARCH-R4-F1 的剩余 cask 删除/清理链；相关文档只用于依赖核对。本轮不签发 tasks.md 的复评结论。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: architecture.md blob `9a5e5a349489c7dacbeefb68ee327c19e7ba7861`；fingerprint `6d1e0be4912cf4c0e3b1ec2a04a5b69c791406c075fca5b10d26a6cc788421a3`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:architecture.md`；Beads: `ad-installed-state-ownership-doc-arch`。
- Target ContentState: `installed-state-ownership:arch:rereview-r7-20261106-fxmFb9-state`。
- 完成门禁：VERIFIED
- Gate evidence: 固定 gate-status 对本轮精确目标返回 VERIFIED；review/L0 两项必需标准都有适用通过证据，missing_criteria、invalidated_evidence、unresolved_candidate_impacts 均为空，历史失败状态保留。
- L0 results: `python3 scripts/check-docs.py` exit 0，包含空白、diff、链接、主题集合和记录结构检查。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-ARCH-R4-F1 — CLOSED。**

- [完整清理链](../architecture.md#cask-的完整安装与卸载链) 第 353–367 行移除最终 App 的 uninstall delete 兜底，将 zap、postflight、失败回滚、强制跳过脚本和私有 Caskroom 清理一并约束；后续链接/暂存清理不能跟随到最终 App。原先在 helper 返回后由 Homebrew 独立删除新拥有者的路径不再被协议允许。
- [清理归属](../architecture.md#卸载与失败清理) 第 339–347 行固定原安装的 installation_id、channel、executable 和限定的 remove_root。清理在连续排他锁内比较原清单与当前回执，不允许从新回执刷新旧 ID；同路径、同 channel、同版本的新安装仍因 ID 不同而被跳过。
- 同一校验保护小组件注销和目标删除。损坏 helper 可以使用私有保存的可信清理 helper；缺失/损坏回执或清单不能获得删除权。撤销后删除失败保留残余和恢复材料，不释放锁后追加兜底删除，也不在延迟回滚时恢复旧 bundle 覆盖新安装。
- 以上补全保留了连续持锁发布、无 TTL 和无跨步骤预留的主流程，未恢复已被否定的方案。

本记录此前的 ISO-ARCH-R1-F1/F2/F3/F4、ISO-ARCH-R2-F1、ISO-ARCH-R3-F1、ISO-ARCH-R4-F2 均保持 CLOSED。对本轮变化检查后，没有新增架构发现。

#### 📝 总结

本轮唯一开放项关闭，架构 PASS。该结论是设计契约通过，不是产品实现、真实 Homebrew 安装/升级/卸载或小组件验收通过；架构中列出的真实 brew 条件和任务中的实现验证继续有效。

**Evidence：** 独立设计模型使用本轮创建的临时 App/回执/清单、真实 flock 和确定的互异安装 ID，覆盖 8 组场景：

| 场景 | 直接观察 |
| --- | --- |
| A 清理返回后，B 以 local_app 登记同一路径 | B 持共享锁时旧清理为 busy；B 退出后旧 ID 为 skipped_superseded，B 的 App/回执/小组件调用记录不变 |
| 同上，但 B 与 A 都为 homebrew_cask | 结果相同；不依赖 channel、版本或时间差来区分实例 |
| 最终 helper 丢失，但原清单与回执匹配 | 清理完成；再次调用为幂等成功 |
| 回执缺失、目标仍在 | unverifiable，保留目标，不注销小组件 |
| 回执损坏 | 同上 |
| 清单缺失 | 同上 |
| remove_root 越过允许的文件/App 根 | 拒绝，保留目标，不注销小组件 |
| 撤销后注入删除失败，再重装新 ID | 保留残余；旧重试不能兜底删除；重装后旧 ID 跳过，只有新清单可以清理 |

全部定向断言通过，脚本 exit 0；临时夹具在退出时删除。模型验证的是书面协议与互斥原语，不替代未来产品测试。Round 6 已核对的 Homebrew script/delete 分派事实作为原失败背景复用；本轮没有重新执行旧的锁外删除，也没有运行真实 brew 操作。

依赖核对：requirements.md blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`、ux/cli.md blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a` 与此前通过时一致；原型 manifest 仍为 `0301083cf12950bc868a91533819b85834b35b62d943477b8f7a870515b3b65f`。tasks.md 在本轮仅更新架构 Review 勾选和状态摘要（输入 blob `ba007c729685812747372b2a78ae2f8b93b0eac7` → 状态同步后 `7f70f0d462f78cdd891d66a705dc4f0f0b67259d`），任务行为定义未改，不影响本架构的内容身份。任务分解仍待独立复评。

Task checkpoint：ad-installed-state-ownership-doc-arch；content_state `6d1e0be4912cf4c0e3b1ec2a04a5b69c791406c075fca5b10d26a6cc788421a3`；gate VERIFIED。
提交建议：经明确授权提交 architecture.md、本评审记录及必要状态片段；可与其他已通过文档成批交付。当前尚未执行提交。
推送建议：目标 origin/feature/installed-state-ownership；需显式推送授权及对应提交对象、签名、范围检查。当前尚未执行推送。
