---
status: active
topic: installed-state-ownership
subject: installation-identity
---

# Installation Identity — Review

## Round 1 — 2026-11-06

### 📋 installation-identity 独立评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-ID-R1-F1 · P1 · 大小写路径别名绕过受保护入口判定

- **位置**：`internal/platform/state.go:141-145`，以及调用它的
  `internal/platform/state.go:172-176`。
- **状态**：open；归属本任务，修复载体为 Beads
  `ad-installed-state-ownership-installation-identity`。
- **问题**：`protectedPath` 仅按区分大小写的字符串相等/前缀判断；
  `walk` 对普通目录保留用户输入的拼写。当前 macOS 的大小写不敏感文件系统中，
  `account/.AGENTDECK` 与 `account/.agentdeck` 指向同一目录，
  但前者不匹配保护集合。解析 `account/.AGENTDECK/sentinel` 返回成功，
  并已对受保护目录内的 `sentinel` 调用 `Lstat`。
- **行为风险**：本任务提供的路径原语不能可靠判断“指向真实状态目录”，
  且已经越过“子路径连元数据也不得访问”的边界。任务 2 若直接依赖它，
  大小写别名将被当作隔离目录，跳过真实根的身份及所有权会话分支。
  这里没有声称任务 2 的 CLI 守卫已经实现或已经做过 CLI 端到端验收。
- **契约依据**：[需求 G3](../requirements.md#目标)；
  [物理解析与保护集合](../architecture.md#什么算指向真实状态目录)；
  [访问边界](../architecture.md#测试与验收的隔离)；
  [任务 1](../tasks.md#1-installation-identity)的物理路径原语要求。
- **直接证据**：独立 Go overlay 探针先用 `os.SameFile` 确认两种拼写是同一目录，
  再分别调用当前候选的 `pathResolver.walk`。
  正常拼写返回 `ErrProtectedStatePath`，子文件 `Lstat` 计数为 0；
  大写别名返回 `error=<nil>`，子文件 `Lstat` 计数为 1。
  `TestReviewCaseInsensitiveProtectedAlias` FAIL（0.01 秒），包运行 1.811 秒、退出码 1。
- **测试缺口**：`internal/platform/state_ownership_test.go:52-70` 的用例以及
  `Lstat` 屏障都复用了同一个区分大小写的 `protectedPath`；
  现有反例没有证明文件系统认为等价的不同拼写会被拒绝。

💡 **限定修复**：在访问下一层之前，按所在文件系统的实际路径等价规则识别
受保护入口及其祖先的别名，不能只靠区分大小写的字符串前缀。
同时保留大小写敏感卷上两个不同目录的合法行为，不能全局直接改成
`EqualFold`。补充入口、子路径和祖先拼写变化的回归用例，
独立确认目录等价，并断言拒绝发生在子路径元数据访问之前。

### 🟡 建议改进 — 推荐

无。本轮没有以“建议”名义延后开放发现。

### 🟢 优点

- 身份判定每次重读唯一回执，登记路径不随 opt 链接切换而改变。
- 回执读取有大小上限、`O_NOFOLLOW`、字段名及重复字段校验；
  写入要求排他锁，使用随机安装 ID、临时文件及原子重命名。
- 所有权锁与状态锁分离，续登在取得排他锁后重新检查条件。

### 📝 总结

**Reviewer**：Codex，独立冷上下文评审；没有参与该候选开发，没有委派。

**Method**：读取任务与批准契约、同工作树 CodeGraph 定位、核心实现及差异检查、
现有路径测试检查、临时账户中的 Go overlay 反例。
发现决定性阻断反例后停止扩大产品验证，未重跑全量套件。

**Scope**：任务 1 的 Resolver、回执、formula 续登、物理路径原语、
所有权锁、验收 Resolver 的编译隔离及 CLI/Makefile 接线。
后续任务的真实状态守卫、客户端配置守卫、安装器和 App 接入不在本轮内。

**Reviewed state**：

- Workspace：`agent-deck.installed-state-ownership`。
- Branch：`feature/installed-state-ownership`。
- HEAD：`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`。
- 实际执行探针的开发候选指纹：
  `b181231712704d82ee9ab0d1309c17de599369b65cfdee3fd38625b7deefa559`。
- 本轮状态同步后的指纹：
  `814d49f188b7e2f3039533daa2164a330cf92ac019b982b78acf28874be3f781`。
- 计算规则沿用既有 WorkUnit：SHA-256，输入为
  `head=<HEAD>` 加排序后的 `<path>=<git-blob>`，逐行以 LF 结束。
  范围是 Makefile、CLI main、任务 1 的 installation/platform 源码及测试、
  验收符号检查脚本、topic tasks.md。
- 两个状态间只有 tasks.md 的评审交接文字变化；
  14 个代码/测试/构建文件，以及 go.mod、go.sum、vendor/modules.txt、
  Go 测试包装器的 blob 均相同。产品代码、测试和配置未改动。
- 本轮关键源码 blob：state.go `f77d14eba155fd1ebfca7422d753cc5f2de72a97`；
  state_ownership_test.go `5316ffd08edc8c3a6e99e1c3b03c68b8debf5e92`。
- 环境：macOS 26.7.3，darwin/amd64；显式选择并核实 mise 的 Go 1.27.3。
  当前 shell 默认 Go 为 1.27.1，未用它执行本轮探针。

**Evidence**：

1. 进入评审时，15 个候选文件及 4 个依赖/包装器 blob 与开发 ContentState
   逐项一致。初始 CEv1 为 FAILED：5 项行为证据通过，完整验证失败，独立评审缺失。
2. 新的独立反例仅在临时账户和临时 HOME 中运行，以 overlay 给编译中的
   platform 包添加诊断函数；不改仓库源码或测试文件，也不访问真实账户状态。
   命令形态如下（占位符表示每次复现新建的临时材料，不是保留日志的引用）：

   ```sh
   env PATH=<Go-1.27.3-bin>:$PATH HOME=<temporary-home> \
     GOCACHE=/private/tmp/agent-deck-go-build \
     GOMODCACHE=/private/tmp/agent-deck-go-mod \
     scripts/run-go-test.sh -overlay <overlay.json> ./internal/platform \
     -run '^TestReviewCaseInsensitiveProtectedAlias$'
   ```

   最小复现逻辑：使用现有 `pathFixture(t)` 获得注入临时账户的 resolver；
   创建 `home/.agentdeck/sentinel`，确认
   `os.SameFile(os.Stat(home+"/.agentdeck"), os.Stat(home+"/.AGENTDECK"))`
   为 true；调用 `r.protectedPaths()`；用 `r.lstat` seam 统计
   `os.SameFile(info, sentinelInfo)` 的命中；依次对正常路径与大写别名调用
   `r.walk(path, protected, nil)`。观测为：
   `canonical: ErrProtectedStatePath, childStats=0`；
   `alias: nil, childStats=1`。
   这是当前实现的实际执行结果，不是仅对设计的协议模型。
3. 复用仍适用的身份、回执、锁、续登证据，以及原有完整验证失败证据。
   原有路径用例通过的事实保留，但不据此推断别名场景通过。
   新反例已登记在原始执行状态，并通过显式目标绑定用于最终状态。
4. 原有全量失败未被本轮修复、重跑或豁免：
   `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree` 在
   `batch_test.go:71` 报 `collector-000001.jsonl: empty trace`。
   开发证据记录了候选及未修改 HEAD 的同失败对照；
   两个 doctor 超时用例隔离后通过。
   这是独立的验证限制，不把它错误归因为本轮路径发现。
5. 本轮文档/评审记录结构、空白与 diff 检查结果见本轮收口条目。
   这些只证明记录结构，不改变语义评审结论或 CEv1 结果。

**Completion gate**：FAILED。

- WorkUnit：`installed-state-ownership:installation-identity`。
- ContentState：`installed-state-ownership:installation-identity:review-r1-20261106-814d49f188b7`。
- 最终固定 gate 查询：7 项必需条件中 4 项有适用通过证据；
  paths、review、validation 均有适用失败证据；
  `reason=null`，没有未解决 candidate impact 或 invalidated evidence。
- 本轮写入 9 个新节点并逐项读回；22 条关系预检全部为 ok，
  写入统计为 22，再查询上述目标门禁。没有覆盖历史观察。

**结论与剩余边界**：ISO-ID-R1-F1 阻断本任务通过；
Review 保持未勾选，任务返回修复。任务 1 尚未完成交付，
主题后续 8 项实现任务也没有完成；没有提交或推送。

**收口检查**：`python3 scripts/check-docs.py`、
`bash scripts/check-topic-docs.sh installed-state-ownership`、
`make check-whitespace` 均通过。最终用
`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/installation-identity.md`
及 `git diff --check` 核对记录字段与差异格式；这些仅为静态检查。

### 下一步指令

修复：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R1-F1

## Repair 1 — 2026-11-06

**修复范围**：仅 `ISO-ID-R1-F1`。由本候选的开发者 Codex 执行修复，未委派；
保留 Round 1 的历史 FAIL，修复结果交独立复评。

**发现处理**：`ISO-ID-R1-F1 -> repaired in candidate`，待复评确认。

- `state.go` 逐分量解析时，以 `os.SameFile` 比较入口与祖先的实际文件系统身份，
  在进入下一层前拒绝保护入口的别名，并把已识别的祖先还原成保护集合使用的路径。
  比较所需的入口及祖先元数据只在单次 walk 内复用，不读取目录内容；
  下一次 walk 重新取得元数据，覆盖同路径目录被替换的情形。
- 缺失的入口没有 inode 可比较。对疑似大小写别名，Darwin 适配器在最近的
  已存在祖先上查询 `pathconf(_PC_CASE_SENSITIVE)`，只在该文件系统不区分
  大小写时匹配别名。能力查询失败就拒绝解析；其他平台在能力不可判定时也拒绝
  该歧义路径。已存在的入口仍按对象身份比较，没有对所有路径统一 `EqualFold`。
- `state_ownership_test.go` 使用独立的 OS 身份判定作为元数据访问断言，
  不再用被测的 `protectedPath` 作为屏障。新增入口、子路径、祖先拼写变化、
  符号链接入口/中间节点/目标、缺失入口、缺失多级链接目标及目录替换用例。
  大小写敏感对照使用真实且不同的目录 inode 与注入的文件系统规则模型，
  确认不同目录仍可访问；这项对照不是挂载真实大小写敏感卷的验收。
- `state_case_darwin_test.go` 在当前实际卷上独立比较 `pathconf` 结果与
  `os.Stat`/`os.SameFile` 的名称查找结果，并检查缺失真实根的大小写别名。

**验证事实**：

1. 修复前，在 Round 1 的产品代码 `state.go` blob
   `f77d14eba155fd1ebfca7422d753cc5f2de72a97` 上运行新增反例：
   `.AGENTDECK` 入口错误返回成功；大写入口及祖先别名的子路径触发元数据访问断言。
   `TestProtectedPathFilesystemAliases` FAIL，复现评审发现。
2. 最终修复下，`scripts/run-go-test.sh -race ./internal/platform` 通过
   （Go 1.27.3，包耗时 14.222 秒）。上述别名用例在拒绝前的子路径元数据
   访问计数均为 0；原物理路径反例和所有权锁用例同样通过。
3. `make vet build-all check-install` 通过，包含两种 macOS 架构和普通/验收构建
   的 Resolver 符号隔离检查。身份、回执、续登及锁的实现没有改动。
4. 全量命令改为 `scripts/run-go-test.sh -p 1 ./...`，避免包间并行与构建负载
   干扰短超时用例。原有 `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree`
   仍在 `batch_test.go:71` 报 `collector-000001.jsonl: empty trace`
   （本轮 22.06 秒）；没有修改该用例、放宽期限或豁免完整验证条件。

**边界**：这些是路径原语的实际执行与独立访问断言；任务 2 的 CLI 守卫接入和
操作系统访问屏障验收仍由任务 2 承担。Review 未勾选，任务不关闭，不提交或推送。

**修复候选与收口**：HEAD 仍为
`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`；沿用原有候选指纹算法，
纳入新增的三个 platform 文件，指纹为
`c54eec773f1e5fcc287af656330e7bca6ab16f9dcdd9574382ecfaa3abf6288e`。
评审记录仍在该算法范围之外，避免记录内容与其自身摘要相互依赖。
关键 blob：state.go `f39a8c30c310517c5ac77ce3e8037624c340d193`；
state_ownership_test.go `56b6600cba3f47f4afd61e45f9003db6986af16d`。

全量回归已结束：27 个包通过，仅 cmd/agentdeck-perf 的上述基线用例失败；
doctor 包本轮通过。文档结构、专题文档、评审记录、空白及 diff 检查通过。
CEv1 目标为
`installed-state-ownership:installation-identity:repair-r1-20261106-c54eec773f1e`，
精确目标查询结果为 **FAILED**：5 项行为条件具有适用通过证据，validation
保留本轮失败，review 等待独立复评；没有未解决的候选影响或失效证据。
身份、回执、锁和续登按函数作用范围及不变 blob 做了显式 preserves 评估和
目标绑定的 roll-up；没有把 Round 1 的路径/评审失败重标为新候选的通过。
本轮新增 12 个节点、24 条关系，关系预检全部 ok，计数读回一致。

**下一步**：复评：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R1-F1


## Round 2 — 2026-11-06

### 📋 ISO-ID-R1-F1 独立复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-ID-R2-F1 · P1 · 缺失的受保护链接目标仍可通过 Unicode 等价拼写绕过

- **位置**：`internal/platform/state.go:223-235`；
  `walk` 在 `state.go:295-304` 对缺失路径使用该判断后返回可用路径。
- **处置**：新发现，open；归属本任务修复引入的缺失入口等价判断分支，
  载体为 Beads `ad-installed-state-ownership-installation-identity`。
- **问题**：`protectedMissingAlias` 只用 `strings.EqualFold` 比较缺失分量，
  它没有覆盖当前 macOS 文件系统的 Unicode 规范等价名称。
  临时真实入口 `account/.agentdeck` 指向尚不存在的 `caf\u00e9`；
  候选 `cafe\u0301/sentinel` 与它在操作系统中属于同一路径，
  但比较不匹配，甚至尚未进入文件系统能力查询就放行。
  这里的名称分别是 `caf + U+00E9` 与 `caf + U+0065 + U+0301`。
- **行为风险**：未安装调用者得到一个成功返回的“隔离”路径后，
  创建它会创建受保护真实目标及其内容，违反真实根缺失时仍须保护的边界。
  这是本任务路径原语的缺口；没有声称任务 2 的 CLI 守卫已经接入或完成验收。
- **独立实证**：
  1. 先在另一控制目录上用原生 `os.Stat` 和 `os.SameFile` 证明两种名称等价，
     此时受保护目标仍不存在。
  2. 计算 `P`，对分解形式的候选调用当前 `r.walk`：
     返回候选本身、`error=<nil>`。
  3. 只在临时夹具中创建被放行的别名目录，再对两种拼写执行
     `os.Stat`/`os.SameFile`：均存在且为同一 inode，
     证明被创建的就是受保护的链接目标。
  4. `TestRereviewMissingNormalizedAlias` FAIL；
     同次运行的原大小写反例 PASS。包耗时 1.002 秒、退出码 1。
- **契约依据**：[物理解析与保护集合](../architecture.md#什么算指向真实状态目录)
  覆盖根不存在和符号链接目标；[任务 1](../tasks.md#1-installation-identity)
  要求返回真正可使用的物理路径；[需求 G3](../requirements.md#目标)
  不允许未安装进程碰真实状态。

💡 **限定修复**：缺失保护入口的比较应覆盖所在文件系统实际采用的名称等价规则，
包括 Unicode 规范等价，并保留大小写敏感、规范化敏感文件系统上的不同名称。
不能仅把 `EqualFold` 当作文件系统等价判断；无法可靠确定时拒绝歧义路径。
补充缺失链接目标的组合/分解 Unicode 名称回归，用原生文件系统对照确认等价，
并确保解析拒绝后不会创建受保护目标。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ISO-ID-R1-F1 CLOSED**：独立重放原 `.AGENTDECK/sentinel` 反例，
  返回 `ErrProtectedStatePath`，受保护子文件 `Lstat` 计数为 0。
  原有大小写绕过已经修复，不再把该发现保留为开放项。
- 现有入口通过 `os.SameFile` 比较对象身份，祖先在进入下一层前还原；
  单次 walk 重新取得元数据，避免跨调用保留已替换目录的 inode。
- 新回归断言使用独立 OS 身份信息，不再复用被测字符串谓词作为访问屏障。
  大小写敏感卷对照的模拟边界也有明确标注。

### 📝 总结

**发现处置**：

| Finding | 本轮处置 | 依据 |
| --- | --- | --- |
| ISO-ID-R1-F1 | CLOSED | 原始大小写别名独立探针通过，子文件访问计数为 0 |
| ISO-ID-R2-F1 | open，新增 P1 | 缺失链接目标的 Unicode 等价别名被放行，原生创建对照确认同一目标 |

**Reviewer**：Codex，原独立评审者；未参与 Repair 1，未委派。

**Method / Scope**：仅复核 ISO-ID-R1-F1 的修复及相邻的入口等价判断：
同工作树 CodeGraph 定位、修复源码和新增测试检查、
临时账户中的两个独立 Go overlay 探针。得到新的决定性阻断反例后，
停止扩大产品验证，复用适用的既有证据。

**Reviewed state**：

- Workspace：`agent-deck.installed-state-ownership`。
- Branch：`feature/installed-state-ownership`。
- HEAD：`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`。
- 实际执行探针的 Repair 1 候选：
  `c54eec773f1e5fcc287af656330e7bca6ab16f9dcdd9574382ecfaa3abf6288e`。
- 本轮交接同步后的最终指纹：
  `4359143ce3e82bd4e152c89bdc644adc384ad32c4e43c72dd8e91dc5f2e18e23`。
- 沿用既有 HEAD 加排序 path/blob、LF 分隔的 SHA-256 算法。
  从修复候选到最终状态仅 tasks.md 的交接摘要变化；
  17 个代码/测试/构建文件及 4 个依赖/包装器 blob 均相同。
  评审记录不在该指纹范围内。
- 关键源码 state.go blob：`f39a8c30c310517c5ac77ce3e8037624c340d193`。
- 已核实工具链与原证据一致：Go 1.27.3，darwin/amd64，macOS 26.7.3。

**Evidence**：

- 进入本轮时，18 个候选文件及 4 个依赖/包装器逐项匹配修复 ContentState；
  初始 gate 为 FAILED：5 项行为证据通过，validation 失败，review 待独立复评。
- 新的独立运行使用临时 HOME、`pathFixture` 注入的临时账户和 Go overlay；
  没有修改仓库源码、测试或配置，也没有触碰真实账户状态。
  复现命令形态如下；占位符表示新建的临时材料，不引用已删除的日志：

  ```sh
  env PATH=<Go-1.27.3-bin>:$PATH HOME=<temporary-home> \
    GOCACHE=/private/tmp/agent-deck-go-build \
    GOMODCACHE=/private/tmp/agent-deck-go-mod \
    scripts/run-go-test.sh -overlay <overlay.json> ./internal/platform \
    -run '^TestRereview(OriginalCaseAlias|MissingNormalizedAlias)$'
  ```

  原反例输出：`got="" error=protected state path childStats=0`。
  新反例输出：`native equivalent names=true; target missing=true; error=<nil>`；
  后续临时目录创建对照确认 `same inode`。
  Unicode 分支的文件系统规则没有使用注入模型，等价性由本机真实文件系统确认。
- 复用身份、回执、锁、续登证据，保留其适用范围。
  修复的原有路径测试通过事实保留；新反例为另一项实际失败观察。
  新探针绑定其实际执行的 Repair 1 状态，再显式绑定最终状态。
- Repair 1 的全量 `scripts/run-go-test.sh -p 1 ./...` 失败仍保留：
  `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree` 在 `batch_test.go:71`
  报 `collector-000001.jsonl: empty trace`。本轮未重复执行、修复或豁免它。
- 这些是路径原语的隔离运行证据。没有宣称真实大小写敏感卷挂载验收、
  任务 2 CLI 接入或操作系统访问屏障验收已经完成。

**Completion gate**：FAILED。

- WorkUnit：`installed-state-ownership:installation-identity`。
- ContentState：`installed-state-ownership:installation-identity:rereview-r2-20261106-4359143ce3e8`。
- 固定 gate 查询得到 7 项必需条件中 4 项有适用通过证据，
  paths、review、validation 有适用失败证据；`reason=null`，
  无未解决 candidate impact 或失效证据。
- 本轮 9 个新节点已逐项读回；22 条关系预检全部 ok、写入计数 22，
  随后查询上述最终门禁；未覆盖历史观察。

**收口校验**：本轮追加内容使用
`python3 scripts/check-docs.py`、
`bash scripts/check-topic-docs.sh installed-state-ownership`、
`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/installation-identity.md`、
`make check-whitespace` 和 `git diff --check` 做静态检查。
其中本轮文档结构、专题文档和空白检查均通过。
静态结构检查不认证语义评审或 CEv1 门禁。

**结论与交接**：原发现关闭，但新发现仍阻断本任务；
Review 保持未勾选，Beads 返回 `in_progress` 等待修复。
没有产品修改、提交、推送或任务完成声明。

### 下一步指令

修复：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R2-F1

## Repair 2 — 2026-11-06

**修复范围**：仅 `ISO-ID-R2-F1`。由 Codex 执行，未委派；保留 Round 2 的
历史 FAIL 和 `ISO-ID-R1-F1 CLOSED`，本节不签发独立复评结论。

**发现处理**：`ISO-ID-R2-F1 -> repaired in candidate`，待独立复评。

- 缺失入口按分量生成 Unicode NFD 比较键，并分别应用大小写与规范化规则。
  比较前先排除任何一个明显不匹配的分量，避免把不相关路径因规则未知而拒绝。
  比较键不用于改写路径，不在符号链接解析之前规范化原始输入，也不使用 NFKC
  兼容规范化替代规范等价。
- Darwin 通过最近已存在祖先的 `statfs` 识别 APFS 的规范化不敏感语义；
  大小写语义仍由独立的 `pathconf(_PC_CASE_SENSITIVE)` 查询决定。
  不从大小写属性推断规范化属性。其他文件系统的规范化规则在本实现中不能
  确认时，拒绝需要该规则才能判定的歧义路径；现存入口继续使用原生对象身份。
- 新增原生 NFC/NFD 双向及多级缺失目标用例：先在独立控制目录中用
  `os.Stat`/`os.SameFile` 确认等价，保护目标保持不存在，再执行 walk 并断言
  拒绝后两种拼写的目标都未创建。大小写与规范化敏感性的四种组合分别建模，
  覆盖规范等价、同时改变大小写、仅改变大小写及规则未知的情形。
  规范化敏感、大小写敏感对照为规则模型，不宣称挂载了对应真实卷。
- 引入 Go 官方 `golang.org/x/text/unicode/norm`，固定 `x/text v0.37.0`，
  版本与当前 Go 1.27.3 工具链自身使用的版本一致。使用 `go mod tidy` 和
  `go mod vendor` 生成依赖记录；仅增加 x/text 的版本/校验项，已有 x/sys
  仍为 v0.44.0，因直接使用被 tidy 归为直接依赖。没有升级其他依赖版本。

**验证事实**：

1. 修复前的产品源码 state.go blob 为
   `f39a8c30c310517c5ac77ce3e8037624c340d193`。
   新增 `TestMissingProtectedTargetCanonicalAliases` 的三例均 FAIL：
   两个规范形式的双向替换、多级缺失目标都被放行；仅在失败夹具中创建被放行路径后，
   原生对象身份确认创建出的就是受保护目标。包耗时 1.036 秒。
2. 修复后的原生反例及名称规则组合测试通过；
   `scripts/run-go-test.sh -race ./internal/platform` 通过（14.602 秒），
   包含原大小写别名、目录替换、物理路径与所有权锁回归。
3. `go mod verify` 报告全部模块校验通过；vendor 中 x/text 的 13 个文件
   与固定版本模块缓存逐文件一致，`go list -mod=vendor` 报告 norm 来自 v0.37.0。
   已有 vendor 源码没有变化，原有文件仅 vendor/modules.txt 增加新包登记。
   下载校验使用临时 GOPATH/GOMODCACHE，没有关闭 checksum 校验或修改用户 Go 配置。
4. `make vet build-all check-install` 通过，包含两种 macOS 架构和
   普通/验收 Resolver 的符号隔离检查。完整 Go 回归与最终候选门禁在下方收口记录。

**边界**：修复及测试仍限于任务 1 的路径原语；任务 2 的 CLI 接入和操作系统
访问屏障验收没有由本次修复代替。Review 保持未勾选，不提交或推送。

**完整回归**：`scripts/run-go-test.sh -p 1 ./...` 已结束，27 个包通过，
仅 cmd/agentdeck-perf 的既有 `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree`
失败；仍在 `batch_test.go:71` 报 `collector-000001.jsonl: empty trace`
（本轮 20.82 秒）。没有修复该范围外用例、放宽期限或豁免完整验证条件。
由于本轮添加了规范化依赖，身份、回执、锁和续登在当前依赖组合下的全量测试
结果作为新观察记录，不将旧依赖状态下的观察直接重标到本候选。

**修复候选与门禁收口**：HEAD 仍为
`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`；沿用 HEAD 加排序 path/blob、LF 分隔的
SHA-256 算法，并纳入本轮依赖记录和 13 个新增 vendor 文件，共 34 个候选文件。
评审记录仍不在该指纹范围内。最终指纹为
`a4f22dff85a8ccea2ec8e5a5e012911607b7dc32dc3b84f830e684fbc5c157c0`；state.go blob 为
`f76ab12c343fecbe4a1a45a016579886c7d848c8`。

CEv1 目标为
`installed-state-ownership:installation-identity:repair-r2-20261106-a4f22dff85a8`。
精确目标查询结果为 **FAILED**：5 项行为条件有适用通过证据，validation
保留上述基线失败，review 等待独立复评；无未解决的候选影响或失效证据。
本轮新增 7 个节点、12 条关系，全部关系预检 ok，计数读回一致。
文档、专题、评审记录、空白与 diff 检查按本轮最终文本收口；这些检查不代替
独立复评，也不将整体任务声明为完成。

**下一步**：复评：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R2-F1


## Round 3 — 2026-11-06

### 📋 ISO-ID-R2-F1 独立复评

📊 总体评分：7/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-ID-R2-F1 · P1 · 仍开放：Stream-Safe NFD 比较键仍会漏掉文件系统等价名称

- **位置**：`internal/platform/state.go:234` 的快速筛选，以及
  `internal/platform/state.go:266-269` 的名称比较键。
- **处置**：still open，部分修复；沿用原发现 ID，不另建相同缺陷。
  原 `caf\u00e9` / `cafe\u0301` 样例已通过，但缺失保护目标的
  Unicode 等价名称仍可被放行。载体仍为
  `ad-installed-state-ownership-installation-identity`。
- **问题**：这里把 `norm.NFD.String` 的结果当作文件系统规范等价键，
  但该 API 同时执行 Stream-Safe 处理。超过 30 个连续 non-starter 时，
  它插入 U+034F Combining Grapheme Joiner；组合字符原始排列不同，
  插入边界也可能不同，导致原生文件系统认为等价的两个名称得到不同键。
  当前快速筛选据此判为不匹配，不再进入能力判断，最后返回可用路径。
- **源码依据**：本候选 vendored
  `golang.org/x/text/unicode/norm/normalize.go:501-503`
  明确说明超过 30 个 non-starter 时插入该字符；
  `composition.go:90-91` 声明 `GraphemeJoiner = "\u034F"`。
  这是对依赖 API 语义的使用问题，不是声称 x/text 的实现错误。
- **行为风险**：未安装调用者仍可得到实际指向缺失真实状态目标的路径，
  随后的创建会创建受保护目标。无需目录竞态、替换真实安装或超长文件名；
  下面两个合法名称都只有 63 字节。风险仍是任务 1 的保护路径原语不完整，
  不声称任务 2 CLI 守卫已实现。

**决定性复现**：

```go
a := "a" + strings.Repeat("\u0301", 30) + "\u0323"
b := "a\u0323" + strings.Repeat("\u0301", 30)
```

在临时账户中令 `account/.agentdeck -> root/a`，目标最初不存在。
先在独立控制目录创建名称 a，再以名称 b 查找，
`os.SameFile` 返回 true；随后执行当前候选的 `walk(root/b/sentinel, P)`：

- 两个输入长度：`63 / 63` 字节；
- 原生文件系统：`nativeSameFile=true`；
- 两个 NFD 比较键各新增 1 个 U+034F，`equalNFDKeys=false`；
- 路径原语：`refused=false`，返回成功；
- 仅在失败夹具中创建被放行的 b，随后 a、b 都存在且为同一 inode：
  `createdSameTarget=true`。

💡 **限定修复**：不要把 Stream-Safe 转换结果直接作为文件系统规范等价的
否定依据。使用与目标文件系统一致的比较，或在无法可靠比较该类序列时，
在快速“不匹配”放行之前拒绝歧义路径。补充上述 31 个组合字符的原生
等价对照和缺失目标拒绝测试，保留现有大小写/规范化敏感性区分。
不要简单删除全部 U+034F，因为实际输入也可能包含该字符。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 原 `café` 的 NFC/NFD 反例独立执行通过，拒绝后目标仍不存在；
  无关目录对照仍然可以使用。
- 大小写和规范化规则已分开；未知且相关的规范化规则会拒绝，
  比较键没有改写最终返回路径。
- 依赖改动限定为固定的 `x/text v0.37.0`；
  x/sys 的版本未变，原有 vendor 源码未改动。
- ISO-ID-R1-F1 保持 CLOSED；现有入口的对象身份判定路径未改变。

### 📝 总结

**发现处置**：

| Finding | 本轮处置 | 依据 |
| --- | --- | --- |
| ISO-ID-R1-F1 | 保持 CLOSED | 现有入口身份判定路径未变；当前修复证据保留大小写回归通过 |
| ISO-ID-R2-F1 | still open，部分修复 | 原短名称通过；原生等价的 63 字节名称仍被放行并创建同一保护目标 |

**Reviewer**：Codex，原独立评审者；未参与 Repair 2，没有委派。

**Method / Scope**：复核本发现的规范化修复、相邻判断和新增依赖语义；
同工作树 CodeGraph、精确源码/依赖差异检查、两个原生文件系统对照的
独立 Go overlay 探针。决定性反例成立后停止扩大产品验证。

**Reviewed state**：

- Workspace：`agent-deck.installed-state-ownership`；
  Branch：`feature/installed-state-ownership`。
- HEAD：`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`。
- 实际执行探针的 Repair 2 候选：
  `a4f22dff85a8ccea2ec8e5a5e012911607b7dc32dc3b84f830e684fbc5c157c0`。
- 最终交接状态指纹：
  `fd9e9c1f653949d2e563db9927de8e5b1337f696a9bea430a10aab5c2c3e8bd1`。
- 沿用 34 个候选文件的 HEAD 加排序 path/blob、LF 分隔 SHA-256 算法。
  两个状态之间仅 tasks.md 的交接摘要变化，其他 33 个候选文件及
  4 个依赖/包装器 blob 相同；评审记录仍不在指纹范围内。
- state.go blob：`f76ab12c343fecbe4a1a45a016579886c7d848c8`；
  norm/normalize.go blob：`4747ad07a839c12bf7d40fc68d450c54e4e1afc2`。
- 已核实 Go 1.27.3、darwin/amd64、macOS 26.7.3 与修复证据一致。

**Evidence**：

1. 34 个候选文件与 4 个依赖/包装器均匹配 Repair 2 的 ContentState；
   初始 gate 为 FAILED：5 项行为证据通过，validation 失败，独立复评缺失。
2. 独立探针在临时账户、临时 HOME 中运行，Go overlay 只改变本次编译输入，
   没有修改仓库源码、测试或配置，也没有访问真实账户状态。
   命令形态如下，占位符表示每次复现新建的临时材料：

   ```sh
   env PATH=<Go-1.27.3-bin>:$PATH HOME=<temporary-home> \
     GOCACHE=/private/tmp/agent-deck-go-build \
     GOMODCACHE=/private/tmp/agent-deck-go-mod \
     scripts/run-go-test.sh -overlay <overlay.json> ./internal/platform \
     -run '^TestR3NativeMissingTargetNormalization$'
   ```

   `original` 子例 PASS，`long-combining-sequence` 子例 FAIL，
   没有跳过；总测试 0.01 秒，包耗时 1.291 秒、退出码 1。
   原生等价性来自独立控制目录的 `os.Stat`/`os.SameFile`，
   不以被测 NFD 比较的结果作为判定依据。失败后的创建同样仅发生在临时夹具内。
3. 当前依赖组合下的身份、回执、锁、续登测试，以及
   `make vet build-all check-install` 和依赖来源/校验的既有事实仍适用；
   原有路径用例通过不等于本轮反例通过。新增失败观察绑定实际执行状态，
   再显式绑定最终交接状态。
4. 原有全量 `scripts/run-go-test.sh -p 1 ./...` 的失败未修复或豁免：
   `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree` 在 `batch_test.go:71`
   报 `collector-000001.jsonl: empty trace`。本轮未重复全量测试。
5. 这些是任务 1 路径原语的原生夹具证据；
   没有把任务 2 的 CLI 接入、真实账户运行或操作系统访问屏障验收算作完成。

**Completion gate**：FAILED。

- WorkUnit：`installed-state-ownership:installation-identity`。
- ContentState：`installed-state-ownership:installation-identity:rereview-r3-20261106-fd9e9c1f6539`。
- 固定查询：7 项必需条件中 4 项有适用通过证据；
  paths、review、validation 有适用失败证据，`reason=null`；
  无未解决候选影响或失效证据。
- 本轮 9 个新节点逐项读回一致；22 条关系预检全为 ok，
  写入统计 22，再查询最终门禁；没有覆盖历史观察。

**收口校验**：`python3 scripts/check-docs.py`、
`bash scripts/check-topic-docs.sh installed-state-ownership` 和
`make check-whitespace` 均通过。最终用
`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/installation-identity.md`
及 `git diff --check` 核验本轮记录字段与差异格式；
这些检查不认证语义评审或 CEv1 门禁。

**结论与交接**：ISO-ID-R2-F1 仍阻断本任务，Review 保持未勾选，
同一 Beads 任务交回修复。没有产品修改、提交、推送或完成声明。

### 下一步指令

修复：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R2-F1

## Repair 3 — 2026-11-06

**修复范围**：继续处理 Round 3 的 `ISO-ID-R2-F1`，不新建重复发现。
Codex 执行本轮修复，未委派；保留历史复评 FAIL 与 `ISO-ID-R1-F1 CLOSED`。

**发现处理**：`ISO-ID-R2-F1 -> repaired in candidate`，待独立复评。

- 快速筛选和正式名称比较统一使用 `canonicalNameKey`。
  已经规范化的输入保持原样；其他输入逐个字符使用现有 x/text 做规范分解，
  再按 canonical combining class 稳定排序完整的 non-starter 序列。
  不把整段输入交给会引入 Stream-Safe 分隔的转换，也不删除 U+034F。
- 真实输入中的 U+034F 保持为原有字符及排序边界；相同组合类别的字符保持
  原始相对顺序。覆盖前导组合字符、Hangul 分解、分解字符跨长序列边界和
  非法 UTF-8 字节边界。比较键仍不改写实际返回路径。
- 大小写、规范化敏感性及未知规则拒绝分支保持原约束。补充长序列在
  规范化敏感规则下保留不同名称的模型对照，没有增加或升级依赖。

**验证事实**：

1. 修复前 state.go blob
   `f76ab12c343fecbe4a1a45a016579886c7d848c8` 上，原短名称三例 PASS，
   31 个组合字符的双向长名称及含真实 U+034F 前缀的长名称三例 FAIL。
   原生控制证明等价；失败夹具内创建后确认是同一保护目标。包耗时 1.421 秒。
2. 修复后的六个原生等价用例均通过，拒绝后保护目标两种拼写都不存在。
   独立原生负向对照创建并确认“含真实 U+034F”和“不含该字符”是不同 inode，
   路径原语保留合法的不同名称；没有以删除全部 joiner 来换取通过。
3. 名称键的明确预期测试验证无额外 U+034F、保留实际字符及排序边界、
   相同组合类别稳定性、Hangul 和非法字节保持行为。
   最终 `scripts/run-go-test.sh -race ./internal/platform` 通过（14.384 秒）。
4. `make vet build-all check-install` 通过，包含两种 macOS 架构及
   普通/验收构建的 Resolver 符号隔离检查。x/text 版本、vendor 与依赖记录
   未变，沿用 Repair 2 已验证的来源与校验事实。

**边界**：本轮是路径原语修复，保留规范化敏感卷对照为规则模型的声明；
没有代替任务 2 的 CLI 守卫及操作系统访问屏障验收，不提交或推送。

**完整回归**：最终 `scripts/run-go-test.sh -p 1 ./...` 已结束，27 个包通过，
仅 cmd/agentdeck-perf 的既有 `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree`
失败，仍为 `batch_test.go:71` 的 `collector-000001.jsonl: empty trace`
（本轮 22.06 秒）。本轮未修复、放宽或豁免该范围外基线失败。

**修复候选与收口**：HEAD 为
`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`，仍使用既有 34 文件指纹算法，评审记录在范围外。
最终指纹为 `8415b0d3eb9c015893a0f7640394f6da3858cc44827a5e638dbed6fcabb2be1b`；
state.go blob 为 `f9ac9c219b2fe677aff8871312a017f462a493b4`。
与 Round 3 的目标逐项比较，仅 tasks.md、state.go 及两份路径测试文件变化；
依赖、vendor 源码和其他候选文件均一致。

CEv1 精确目标
`installed-state-ownership:installation-identity:repair-r3-20261106-8415b0d3eb9c`
查询结果为 **FAILED**：5 项行为条件具有适用通过证据，validation 保留当前
完整回归失败，review 等待独立复评；无未解决候选影响或失效证据。
本轮新增 7 个节点和 12 条关系，预检全部 ok，处理计数读回一致。
文档、专题、评审记录、空白与 diff 检查按最终文本收口，不签发任务完成声明。

**下一步**：复评：installed-state-ownership / reviews/installation-identity.md / ISO-ID-R2-F1


## Round 4 — 2026-11-06

### 📋 ISO-ID-R2-F1 独立复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。ISO-ID-R1-F1 与 ISO-ID-R2-F1 均已关闭；本轮没有新增发现。
任务完成门禁仍有独立的完整验证失败，见下方门禁及 Task checkpoint。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ISO-ID-R2-F1 CLOSED**：快速筛选和正式比较都使用 `canonicalNameKey`。
  它逐字符做规范分解，再按 combining class 稳定排列完整序列，
  消除了整段 Stream-Safe 转换插入 U+034F 的问题。
- 独立重放原短名称和 63 字节长序列的双向反例，均在访问子路径之前拒绝；
  含真实 U+034F 前缀的等价名称也正确拒绝。
- 原生负向对照确认真实 U+034F 造成的不同名称具有不同 inode，
  路径原语保留其合法访问及原始拼写；没有通过删除所有 joiner 换取通过。
- ISO-ID-R1-F1 保持 CLOSED。现有入口对象身份、大小写与规范化敏感性分离、
  原始路径不提前 Clean/改写的规则均保留。

### 📝 总结

**发现处置**：

| Finding | 本轮处置 | 依据 |
| --- | --- | --- |
| ISO-ID-R1-F1 | 保持 CLOSED | 对象身份判定未变；当前候选回归证据仍适用 |
| ISO-ID-R2-F1 | CLOSED | 独立原生等价/不等价对照全部通过，长序列键无额外 U+034F |

**Reviewer**：Codex，原独立评审者；未参与 Repair 3，没有委派。

**Method / Scope**：复核记录中剩余发现及其相邻边界，审查
`canonicalNameKey`、两个调用点和明确预期测试；
用原生文件系统作独立判定，再运行 Go overlay 探针。
复用当前候选的其他行为、race、vet、build、install 证据，
没有重复未变化的全量套件。范围仍是任务 1；
任务 2 的 CLI 守卫与操作系统访问屏障验收不在本轮内。

**Reviewed state**：

- Workspace：`agent-deck.installed-state-ownership`；
  Branch：`feature/installed-state-ownership`。
- HEAD：`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`。
- 探针实际执行的 Repair 3 候选：
  `8415b0d3eb9c015893a0f7640394f6da3858cc44827a5e638dbed6fcabb2be1b`。
- 最终状态同步后的指纹：
  `93216a708c2220a205928d34209b822ee0977e50e4fbde2457866cfa34d9b903`。
- 沿用 34 个文件、HEAD 加排序 path/blob、LF 分隔的 SHA-256 算法；
  从修复候选到最终状态仅 tasks.md 的 Review 勾选及交接文字变化。
  其余 33 个候选文件、4 个依赖/包装器 blob 均相同，评审记录在范围外。
- state.go blob：`f9ac9c219b2fe677aff8871312a017f462a493b4`。
- 工具链与环境已核实：Go 1.27.3、darwin/amd64、macOS 26.7.3；
  x/text v0.37.0、vendor 和依赖记录没有变化。

**Evidence**：

1. 独立探针 `TestR4IndependentCanonicalClosure` 的 5 个子例全部 PASS，
   没有跳过，总测试 0.02 秒，包耗时 1.160 秒，退出码 0。

   | 原生对照 | SameFile | walk 结果 | 子路径 Lstat | 目标状态 |
   | --- | --- | --- | --- | --- |
   | 原 café NFC/NFD | true | ErrProtectedStatePath | 0 | 两种拼写均不存在 |
   | 63 字节长序列正向 | true | ErrProtectedStatePath | 0 | 两种拼写均不存在 |
   | 同一长序列反向 | true | ErrProtectedStatePath | 0 | 两种拼写均不存在 |
   | 真实 U+034F 前缀加长序列 | true | ErrProtectedStatePath | 0 | 两种拼写均不存在 |
   | 真实 U+034F 排序边界的不同名称 | false | 成功，原始路径不变 | 0 | 两个目标均不存在 |

   5 例都确认比较键的 U+034F 数量与实际输入相同。
   等价性由独立控制目录的 `os.Stat`/`os.SameFile` 得出，
   不使用被测比较键作为 oracle。

2. 使用临时账户、临时 HOME 和 overlay 编译输入，未改仓库源码、测试或配置，
   未访问真实账户状态。复现命令形态如下；占位符表示新建的临时材料：

   ```sh
   env PATH=<Go-1.27.3-bin>:$PATH HOME=<temporary-home> \
     GOCACHE=/private/tmp/agent-deck-go-build \
     GOMODCACHE=/private/tmp/agent-deck-go-mod \
     scripts/run-go-test.sh -overlay <overlay.json> ./internal/platform \
     -run '^TestR4IndependentCanonicalClosure$'
   ```

3. 源码确认两处筛选使用同一个键函数；U+034F、Hangul、非法 UTF-8
   和相同 combining class 的顺序边界有明确预期测试。
   Repair 3 的平台 race（14.384 秒）、当前依赖组合下的回归、
   vet、双架构构建与 check-install 证据仍适用。
   本轮探针补充独立发现关闭证据，不将 5 个用例冒充全部路径覆盖。
4. 保留验证限制：Repair 3 的完整
   `scripts/run-go-test.sh -p 1 ./...` 有 27 个包通过，
   `cmd/agentdeck-perf` 的
   `TestBatchTemplateStartIsANoOpAndBindsTheSourceTree` 仍在
   `batch_test.go:71` 报 `collector-000001.jsonl: empty trace`。
   该基线失败未被本轮修复、重跑或豁免。
5. 未宣称真实大小写/规范化敏感卷挂载验收或任务 2 的 OS 访问屏障完成；
   相关敏感性对照继续按修复记录标为规则模型。

**Completion gate**：VERIFIED。

- WorkUnit：`installed-state-ownership:installation-identity`。
- 当前 ContentState（2026-11-07 验证补齐）：`installed-state-ownership:installation-identity:validation-20261107-f7dc34b18a7a`。
- 当前固定查询：7/7 必需条件有适用通过证据，`reason=null`；
  missing criteria、invalidated evidence、unresolved candidate impacts 均为空。
- **历史观察保留**：独立复评当时的 ContentState
  `installed-state-ownership:installation-identity:rereview-r4-20261106-93216a708c22`
  为 FAILED（6/7，缺 validation）；当时登记的 9 个节点及 23 条关系不变。
  本次补齐是阶段后的证据同步，不改写原始失败，也不重做或自签独立评审。
- 本次新目标登记 8 个节点、16 条关系，节点逐项读回一致，
  关系预检全部 ok、计数一致；当前验证与两份独立评审的范围复用见下方 Validation follow-up。
- 图中当前 Task 没有登记 containing WorkUnit，Topic 节点尚不存在；
  任务归属及主题仍有 8 项后续实现任务由 tasks.md 和 Beads 确定。
  本轮没有跨 Topic 完成边界，不查询或声称 Topic 门禁通过。

**收口与交接（2026-11-07 同步）**：Review 单元保持勾选，验证门禁已补齐；
Beads 为 `awaiting_commit`，等待另行授权的提交，任务尚未交付关闭。
`python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`
及 `make check-whitespace` 均通过；最终用
`python3 scripts/check-review-records.py docs/topics/installed-state-ownership/reviews/installation-identity.md`
与 `git diff --check` 核验本轮记录字段及差异格式。
这些静态检查不代替语义评审或 CEv1 门禁。
原 Round 4 复评没有产品修改；后续验证工具同步见下节。两阶段均未提交或推送。

Task checkpoint：`ad-installed-state-ownership-installation-identity`；content_state=`f7dc34b18a7a1e7babe30d05a4795f344f4430af17d3d790014d77b18ac47912`；gate=VERIFIED，等待授权提交。

提交建议：validation 缺口已关闭；经明确授权后按本任务边界提交：
installation/platform 实现及测试、CLI 注入、构建检查、依赖/vendor、
tasks.md、本评审记录及下方列明的六个验证工具文件。执行时核验范围、贡献者、提交正文/尾注及 SSH 签名；复用代码的贡献者须从来源提交及任务记录核实。

推送建议：门禁已 VERIFIED；完成授权提交及提交对象/签名核验，再经单独授权推送
`origin/feature/installed-state-ownership`。
已核实 origin 为 `https://github.com/kitdine/agent-deck.git`；
未来推送时仍须核验实际提交范围、远端与所需检查。本建议不执行或授权交付。

### 复评当时的后置前提（2026-11-07 已完成）

调查并修复 cmd/agentdeck-perf 的 TestBatchTemplateStartIsANoOpAndBindsTheSourceTree
空 trace 基线失败，补齐 installed-state-ownership / installation-identity 的 validation 门禁证据。

## Validation follow-up — 2026-11-07

本节处理真实用户明确请求的
“调查并修复 cmd/agentdeck-perf 的 TestBatchTemplateStartIsANoOpAndBindsTheSourceTree
空 trace 基线失败，补齐 installed-state-ownership / installation-identity 的 validation 门禁证据”。
它是已通过复评后的验证补齐，不是新的评审轮次，不修改历史 finding 处置。

**分类与置信度**：测量 harness 的完成同步缺陷，置信度高；
由当前沙箱无法枚举进程触发。生产扫描和 trace 内容本身能够正常完成。

**根因证据**：

1. 相同 Go 1.27.3、vendor 与隔离 HOME 下，只运行原失败测试：
   `scripts/run-go-test.sh ./cmd/agentdeck-perf -run '^TestBatchTemplateStartIsANoOpAndBindsTheSourceTree$'`。
   修复前仍在 `batch_test.go:71` 报
   `collector-000001.jsonl: empty trace`，测试耗时 31.36 秒、退出码 1。
2. `runCLIWith` 在等待 CLI 后调用 `waitForWorkers`。
   `workerRunning` 依赖 `/bin/ps -Ao command=`；
   当前沙箱中实际执行该命令得到 operation not permitted（exit 127）。
   `output` 丢弃执行错误并返回空字符串，后者被解释为没有 worker。
3. 独立诊断 overlay 保留同一份 collector 文件而不在失败处立即清理：
   `runCLI` 返回时为 0 字节、`workerRunning=false`；
   不修改源树、状态或该 trace，约 1.039596921 秒后同一 inode 自然变为
   5362 字节。此时读取到完整 lifetime，2 份 trace 通过样本读取，
   `read_bytes=0`、`decoded_records=0`。
   诊断测试通过（25.04 秒），证明读取发生在正常刷盘之前。
4. 产品使用 64 KiB 缓冲，`perftrace.Close` 先写 lifetime，再可能追加
   `trace.invalid`，最后 Flush/Sync/Close。因此只等“文件非空”或 lifetime
   本身不能证明尾记录已经完成。

**排除的解释**：

- 不把问题归因于 installation-identity：失败在此前未修改 HEAD 已复现，
  本次也直接定位到未改动的测量调用链。
- 不是 no-op 扫描多读了源数据：同一运行自然完成后两个计数均为 0；
  原断言和源树绑定核对保留，修复后的完整原测试全部通过。
- 不是 trace 永久损坏、Go 版本差异或单纯需要更大的等待上限：
  同一文件在原有 60 秒边界内自然成为有效 trace，
  修复前后使用相同 Go 1.27.3，未修改测试期限。

**修复权限与范围**：用户明确授权修复该测试的空 trace 基线失败并补齐门禁。
本轮只同步测量工具和 trace 记录器的既有完成协议；
installation-identity 的生产实现、测试、配置及依赖原样保留。
没有新增产品行为决策、修改真实账户状态、创建分支/工作树或执行 Git 交付。

**实际改动**：六个文件直接复用仓库提交
`55dcf411454b17322eea73b983a7360bb88e6352` 的对应内容：

| 文件 | 完整文件 blob |
| --- | --- |
| cmd/agentdeck-perf/run.go | c4c3467314d2501eef84555be9ad5182189308c5 |
| cmd/agentdeck-perf/run_test.go | fe43c43607f416eb72924c66fe7ccb4ac58650f2 |
| internal/perftrace/perftrace.go | 5c668dffcb1ef2ae3e952611942646435f37624f |
| internal/perftrace/perftrace_test.go | b649c7cffc4141e9957bd961f46a3f272ac94b96 |
| internal/perftrace/lock_unix.go | 0205735a68ac0d8fe4d728518853fe6da5cd14f2 |
| internal/perftrace/lock_other.go | 0abc8b03e457fee9142499eab0af18449bb56661 |

当前文件与该提交逐项 hash 比较完全一致；没有整批移入其余 origin-store 改动。
该提交仍由 `feature/auxiliary-session-classification` 引用，
原始评审可从该 Git 对象的
`docs/topics/auxiliary-session-classification/reviews/origin-store.md`
Round 4（2026-11-05）读取。

生产者从启用记录起持有独占文件锁，完整尾记录刷盘、同步并关闭后释放；
消费者在返回给读取器前，要求锁已释放且有 lifetime。
原 60 秒上限、trace 格式、读取器对无效尾记录的拒绝及 CLI 外部计时区间不变。
回归包含“已可见 lifetime 的 65536 字节前缀仍持锁”反例，避免只修空文件而提前接受尾部未完成的样本。

**当前验证**（Go 1.27.3，macOS 26.7.3，临时 HOME/fixture，vendor）：

| 检查 | 结果 |
| --- | --- |
| 原失败测试及三个完成屏障回归 | PASS；原测试 54.49 秒 |
| `scripts/run-go-test.sh -p 1 ./...` | PASS，28 个包；原测试 46.12 秒，cmd/agentdeck-perf 72.256 秒 |
| trace、perf runner、store 相关 race | PASS；包耗时分别 3.096、3.092、4.090 秒，无 DATA RACE |
| `make vet build-all check-install`，输出目录为临时目录 | PASS；Darwin arm64 与 amd64 构建成功 |
| 安装验收 Resolver 符号隔离 | production absent，acceptance present |

定向与 race 的可复现命令为：

```sh
scripts/run-go-test.sh ./cmd/agentdeck-perf ./internal/perftrace \
  -run '^Test(BatchTemplateStartIsANoOpAndBindsTheSourceTree|WaitForTraces.*|TraceLockIsHeldUntilCloseFinishes)$'
scripts/run-go-test.sh -race ./internal/perftrace ./cmd/agentdeck-perf ./internal/store \
  -run '^Test(WaitForTraces.*|TraceLockIsHeldUntilCloseFinishes|ConcurrentMarkersAndClose|WritableStoreTracesWrites|WritableStoreIsUntracedWhenRecordingIsOff)$'
```

运行时显式把 Go 1.27.3 放在 PATH 首位，使用临时 HOME、
`GOCACHE=/private/tmp/agent-deck-go-build`、
`GOMODCACHE=/private/tmp/agent-deck-go-mod`；
未同时运行完整回归与额外构建，以免引入已知短超时用例的资源干扰。
诊断 overlay、日志与临时二进制没有作为持久引用，必要事实已完整记录于此。

**独立评审复用，非自评**：

- 原 34 个任务候选文件中，仅 tasks.md 的范围/状态说明更新；
  其余 33 个及四个依赖/包装器 blob 与本记录 Round 4 相同。
  installation-identity 的独立复评 PASS 因此保持有效。
- 增加的六个完整文件与 `55dcf411` 相同；该提交中的 origin-store Round 4
  是未参与其实现的 Codex 独立复评，明确关闭 trace 完成协议发现 ASC-OS-R3-F1。
  只复用其六文件完成协议部分，不把 origin-store 的其他行为证据移入本任务。
- 该协议的已登记来源评审证据为
  `auxiliary-session-classification:origin-store:commit:55dcf411454b17322eea73b983a7360bb88e6352:independent-review:delivery`。
  它与本任务 Round 4 的 review 证据通过显式目标绑定 roll-up 共同支持当前 review 条件。
- 来源评审使用较早的 macOS 补丁版本，其历史运行结果未被重标为当前结果；
  本节的新定向、全量、race、vet、build/install 运行验证当前集成环境。

**当前内容与证据绑定**：

- HEAD：`2623e87a6a0f45e568644250cfc13e1c75ea4bfd`。
- 候选指纹：`f7dc34b18a7a1e7babe30d05a4795f344f4430af17d3d790014d77b18ac47912`。
- 算法继续为 HEAD 加排序 path/blob、LF 分隔 SHA-256；
  原 34 个文件加上用户授权补齐的六个验证工具文件，共 40 个。
  本评审记录仍在指纹范围之外，避免状态记录与自身摘要相互依赖。
- 当前目标：`installed-state-ownership:installation-identity:validation-20261107-f7dc34b18a7a`。
- 最终固定门禁查询为 7/7 VERIFIED，缺失条件、失效证据和未解决候选影响均为空；
  新增 8 个节点、16 条关系，逐项读回及关系预检/计数均一致。
  原始 FAILED ContentState 与观察保留，未改写为成功。
- 新的五项行为观察来自本轮完整回归；
  validation 来自本轮全部验证；
  review 来自上述两份独立评审的明确范围复用。

**残余边界与交接**：没有未解决的本次空 trace 失败；
未宣称新的真实用户性能矩阵、任务 2 OS 访问屏障、整个 Topic 完成或发布验收。
验证补齐时 Task 为 `awaiting_commit`；实际提交与任务关闭状态以 Beads 的交付记录为准。
代码与记录的交付范围及建议沿用上方更新后的 Task checkpoint。

**本地提交授权（2026-11-07）**：用户已授权本任务本地提交。交付准备仅把状态说明改为
由 Beads 跟踪，产品、测试、配置及依赖未变；不重跑已有效的产品验证。
实际提交对象、SSH 签名及提交态 CEv1 绑定在交付后登记，不预写尚不存在的提交 SHA。
