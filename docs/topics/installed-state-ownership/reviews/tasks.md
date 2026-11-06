---
status: active
topic: installed-state-ownership
subject: tasks.md
---

# Tasks Review

## Round 1 — 2026-11-05

### 📋 任务分解评审报告

📊 总体评分：6/10

✅ 评审结论：FAIL

- Reviewer: Codex；独立于本批文档作者的当前评审会话，未委派。
- Method: 设计契约评审、原始缺陷交叉核验、所需源码调用路径核验及临时目录内的设计规则反例；未执行真实安装、真实数据操作或产品实现。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。
- Scope: 四份 Documents 声明、九个实现任务的目标映射、文件边界、依赖、验证等级和验收可执行性。
- Reviewed state: 初稿 blob `06f3d09b5e3dc93bba18e07fcfa0bff5fe3f053a`；收口仅新增本轮评审状态与记录指针，最终 blob `e15323ca54b64114c75d6e710354fd11d6c2cf33`。
- Final fingerprint: `adb393b5c134f1bfdfe49c3f76545a44e4ec421d302f03a2e04e32008aa91740`，SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:tasks.md`。
- Target ContentState: `installed-state-ownership:tasks:review-20261105-a0fXv0-final-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 查询返回本 WorkUnit / ContentState 的 FAILED；review 失败证据 applicable=true，L0 通过证据有效；invalidated_evidence 与 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 全部 exit 0；主题集合检查也为 exit 0。静态检查不代表语义评审通过。

#### 🔴 严重问题 — 必须修复

**ISO-TASKS-R1-F1 — P2 — 生产二进制的端到端隔离环境与仅测试注入点没有接通。**

- 位置：[tasks.md](../tasks.md#8-ownership-acceptance) 第 185–193 行，以及第 51、59 行；[architecture.md](../architecture.md#安装身份判定) 第 43 行、测试影响第 132 行。
- 行为风险：任务 8 无法按已写步骤，在完全不触碰真实账户状态和回执的前提下，用标准生产构建验证已安装路径与真实状态迁移；实现者只能临时发明绕过入口、换用测试二进制或意外读到真实回执。
- 证据：任务要求使用 `make build-all` 的二进制和“临时账户主目录、临时回执”；但身份解析只认系统账户数据库的 HomeDir，不认 HOME、参数或环境变量，注入点又限定“仅测试可用”。`Makefile:39-42` 的 build-all 是普通 `go build ./cmd/agentdeck`，没有定义携带测试注入器的构建方式。临时 HOME/PREFIX/APPDIR 不能改变生产二进制所查询的系统账户主目录。
- 💡 修复建议：明确验收运行器：例如专用临时系统账户/隔离系统环境，或明确标注的测试专用可执行体并另列生产产物的有限验证。说明回执、账户身份和子进程如何同时隔离，保留“无生产绕过开关”和真实状态不被触碰的要求。
- Disposition: OPEN。

**ISO-TASKS-R1-F2 — P2 — 目录内容/mtime不变不能证明“没有读取真实状态”。**

- 位置：[tasks.md](../tasks.md#2-state-root-guard) 第 86 行；[requirements.md](../requirements.md#验收边界) A1。
- 行为风险：只读命令或 helper 在拒绝前打开并读取真实数据库，依然可满足文件清单、内容、mtime 不变；被称作结构性兜底的验收无法检出核心禁止行为。
- 证据：读取现有文件无需改变上述三项。当前 `doctor.Service.Check` 就在打开数据库前执行 `os.Stat`、锁诊断和 hookrefusal 读取；Guard 的验收必须证明拒绝在相应访问之前发生，而不是只证明最终没有写入。
- 💡 修复建议：保留不变性断言，并增加能观察读取/打开调用的故障注入或文件访问计数，证明被拒绝路径在首次受保护访问前短路；覆盖包直接调用、CLI 和子进程，清楚区分“无写入”和“无访问”的证据。
- Disposition: OPEN。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

显式声明四份文档和 n/a 理由；未提前创建实现任务；共享 main.go 的任务按顺序执行；安装器和 Debug 身份采用相应 L3 验证。

#### 📝 总结

Documents 声明与四份在盘初稿一致，`bash scripts/check-topic-docs.sh installed-state-ownership` 已以 exit 0 完成。文档集合结构完整不等于语义获批：同批需求、CLI 和架构均 FAIL，以上测试边界本身也未解决，所以分解不予批准，九个实现任务不进入开发。

Dependency coverage: 需求 blob `386f661f633da7c2d5bb765640228c0ec1c581f3`、CLI blob `2757f21e64e53cbcc734be5ca32a4cb51487bd81`、架构 blob `597b1e05e1889e9708a6f14f2c046875afc21dbe`。其发现分别由 [需求](requirements.md)、[CLI](ux-cli.md)、[架构](architecture.md) 评审记录承载。后续应按修复后的契约调整任务和验收，不在本轮修改任务内容。

Evidence: 核对 Makefile 的生产构建命令、架构的账户解析/测试注入约束和任务的端到端步骤；无真实账户、安装或数据库操作。收口仅在 tasks.md 增加状态指针，原任务内容和 Review 未勾选状态均保持。最终文档格式、链接、空白及记录解析的 L0 结果登记到 CEv1。

修复范围：ISO-TASKS-R1-F1–F2，并随上游文档修复对齐分解。

## Repair 1 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-TASKS-R1-F1、F2，并按上游文档的修复调整分解。只修改 `tasks.md`。
- Beads：`ad-installed-state-ownership-doc-tasks` 由 claude-code 领取修复。

### ISO-TASKS-R1-F1 处置：已修复，待复评

- 在架构中新增 [测试与验收的隔离](../architecture.md#测试与验收的隔离)，分为三层：
  - 包内测试：替换未导出字段，只在 `_test.go` 中可用；
  - 端到端验收：`agentdeck_acceptance` build tag 构建的验收二进制，`AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 同时隔离账户主目录和回执，scan worker、helper 等子进程继承这个环境变量；
  - 生产产物：只做静态验证，检查没有验收符号、构建命令中没有这个 tag。
- 生产代码仍然没有任何覆盖入口，验收 tag 不能用于发布构建、Makefile 的 `build` 和 `build-all`、App 打包或 Homebrew 模板。
- [任务 8](../tasks.md#8-ownership-acceptance) 改为使用验收二进制，并明确写出：生产产物的运行时行为由前两层覆盖，不能声称用生产产物做过运行时验收。符号检查加入任务 1 和 `check-install`。

### ISO-TASKS-R1-F2 处置：已修复，待复评

- 证明"没有访问"改用 `sandbox-exec` 的访问即终止配置：对注入的真实目录执行任何 stat、open、读或写，进程都会被 `SIGKILL`。
- 子进程继承这个配置；包级直接调用的测试，在同样配置下运行对应包的测试二进制。
- 每次运行先做一次阴性对照，证明屏障确实生效；屏障不可用时测试失败，不降级。
- 任务正文开头加了一条通用规则：只断言"文件不变"不能作为"没有访问"的证据。
- [任务 2](../tasks.md#2-state-root-guard) 的结构性回归测试，改为要求所有叶子命令和包级调用都在屏障下运行，断言没有进程被终止；文件不变的断言保留为补充。
- 任务 4 的 doctor 部分报告，同样在屏障下测试。
- 对照评审反例："`doctor.Service.Check` 在打开数据库前就执行 `os.Stat`"这种访问，在屏障下会直接终止进程，测试失败。另外，doctor 的拒绝路径已改为根本不调用 Service。

### 随上游修复调整的分解

- 任务 1：只认唯一回执、判定时解析 opt、不缓存，新增验收 tag 文件。
- 任务 2：守卫每次重新判定，新增长驻进程复核和所有权转移测试，规范化反例覆盖真实目录本身是链接的情形。
- 任务 4：新增 doctor 的部分报告和 `managed_command_owner` 检查，明确安装拒绝不写入 `hook-refusals.json`。
- 任务 5：formula 只改 caveats；新增 `install claim`；在 `TAKEOVER`、`install-macos-app`、`--takeover` 中加入交接前提检查。
- 任务 6：每轮查询 `state_root`，新增两条文案。
- 任务 8：验收顺序新增"旧拥有者仍在运行时拒绝接管"和 formula 认领两步。

任务的数量、名称和依赖关系都不变。

在任务 2 中，修复时也纠正了一处自己写错的要求：初稿写过"所有权转移之后的阶段也在屏障下观察"，但访问屏障不能在进程运行途中启用。改为：转移之后长驻进程不再开始新一轮，由轮次证据证明；之后新启动的旧可执行文件进程被拒绝，由屏障证明。

### 验证

结构检查同 [需求记录](requirements.md#验证) 中的 Repair 1 验证。`bash scripts/check-topic-docs.sh installed-state-ownership` 的结果为 exit 0，文档集合不变。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `d04793f37c5cb88475fa8af3d3df2e9d9ff8c6e5`；指纹 `ecc65b04a0a8bc8f5a2df1a9b68d7523b4eaf1277ed228faf7e908e34c45063c`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 2 — 2026-11-05

### 📋 任务分解复评报告

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code 的 Repair 1 候选，未委派，未修改设计正文、产品代码、测试或配置。
- Method: 按原发现逐项处置；复用相同 HEAD 的源码事实，并检查修复新增的契约。设计反例与生产运行验收严格区分。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `7a7e175dd25e24a12c082f53213b872626de8159`；fingerprint `c1a8dbfd49c2eb2fbfa3be2c94efe7f8e01f3bcc56934945c0d0b8c70ee6684e`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r2-20261105-G-VuYw-final-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 对本轮精确目标返回 FAILED；本轮 review/L0 证据均适用，旧状态证据 target_matches=false；无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` exit 0，覆盖未跟踪 Markdown 的链接、记录解析及内部 whitespace/diff/topic 检查；显式主题集合检查也为 exit 0。
- Scope: ISO-TASKS-R1-F1–F2，以及修复后的九个任务、四份文档声明和相应依赖。
- Candidate before status sync: blob `d04793f37c5cb88475fa8af3d3df2e9d9ff8c6e5`；fingerprint `ecc65b04a0a8bc8f5a2df1a9b68d7523b4eaf1277ed228faf7e908e34c45063c`。最终仅勾选已通过的需求 Review 并更新简短状态，任务行为内容未改。

#### 🔴 严重问题 — 必须修复

**ISO-TASKS-R1-F2 — P2 — STILL_OPEN：新增屏障连守卫必需的路径解析也禁止，无法按约定产生正常拒绝。**

- 位置：[tasks.md](../tasks.md#2-state-root-guard) 第 53、87–91、133 行；[architecture.md](../architecture.md#测试与验收的隔离) 第 190–203 行及 canon 第 101–117 行。
- 行为风险：实现者若照设计执行，守卫为识别真实根和符号链接所做的元数据查询就会触发终止，不能得到要求的 state_ownership_refused 或 doctor partial 报告；测试只能永远失败，或由实现者私自放宽“无访问”含义。
- 证据：canon 必须对 account/.agentdeck 和候选已有前缀执行 EvalSymlinks，并找出最长已有前缀。Go 1.27.3 的 filepath.walkSymlinks 在解析每个普通分量时调用 `os.Lstat(dest)`。屏障明确 deny file-read* / file-write*，并在第 200 行明言 stat 也触发 SIGKILL；任务 91 行则要求所有这些命令正常返回且不被终止。移除 doctor.Service 的 Stat 不能移除守卫自身的 Lstat。
- 💡 修复建议：明确被禁止的是状态内容访问、哪些路径身份元数据是否允许，并让需求、规范化算法和验收观测一致；若仍禁止全部元数据查询，就必须给出不依赖这些查询的可行守卫方案。用真正执行 canonicalization → guard → refusal 的对照验证，而非仅证明直接读文件会被杀死。不得把不可执行的屏障换成仅检查 mtime，也不得私自扩大允许访问范围。
- 已修复部分：放弃“文件不变即可证明未读取”，新增访问观测、子进程覆盖及阴性对照；但当前具体方案存在上述冲突，旧验收缺口未关闭。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-TASKS-R1-F1 — CLOSED。** 第 61、69–70、204、217–220 行明确引入独立 acceptance tag、临时账户变量与子进程传递；生产构建不带 tag，生产产物证据限定为静态检查，不再声称普通生产二进制可以通过测试专用字段改变系统账户身份。设计阶段的运行器边界已足够明确，其未来实现和构建验证仍属于对应任务。

任务 4 已承接 doctor 的安全部分报告，任务 5 已承接 formula 认领，任务 6 改为每轮查询 helper；未提前创建实现任务。

#### 📝 总结

原 2 项发现中 F1 关闭、F2 仍开放；无另增本文件发现。架构 F2 与 R2-F1 的修复会影响任务 1/2/5/8 的要求，由 [架构复评](architecture.md#round-2--2026-11-05) 承载；CLI 的共享标本由 [CLI 复评](ux-cli.md#round-2--2026-11-05) 承载，不在此重复编号。分解保持 FAIL，实现任务不进入开发。

Evidence: 主题文档集合检查 exit 0。代码与契约的矛盾由 Go 1.27.3 的 Lstat 实现及明确的屏障规则证明；当前嵌套 sandbox-exec 不可用，exit 71，未宣称复现 Repair 1 的 exit 137。这一执行限制不是失败判据，失败判据是可静态核验的设计约束互斥。测试运行器和生产测试尚未实现，不运行与本轮文档修复无关的产品全套检查。

对 Repair 1 的验证说明作事实补充：当前 HEAD 的 `scripts/check-docs.py:51-60` 通过 `git ls-files --others --exclude-standard` 纳入未跟踪文件；“不会检查未跟踪文件链接”的说法不适用于此内容状态。本轮使用当前检查器覆盖实际候选，不修改历史修复记录。

修复范围：ISO-TASKS-R1-F2，并按本轮架构及 CLI 仍开放项调整受影响任务；保持 F1 已关闭。

## Repair 2 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-TASKS-R1-F2（复评仍未关闭），并按架构和 CLI 的 Repair 2 调整任务 1、2、4、5、6、8。只修改 `tasks.md`。
- Beads：`ad-installed-state-ownership-doc-tasks` 由 claude-code 领取修复。

### ISO-TASKS-R1-F2 处置：已修复，待复评

复评指出：屏障禁止一切访问，连守卫识别真实目录所需的 `Lstat` 也禁止了，于是不可能观察到要求的拒绝结果。修复的方向是明确"禁止什么"，而不是放宽"没有访问"的含义：

- **禁止的对象。** 需求 G3 禁止的是真实状态目录里的内容。物理解析需要读取的，只有受保护集合 `P` 中每个入口自身的元数据（`lstat`、`stat`、`readlink`）。新的 `walk` 只要碰到 `P` 就先拒绝，不会再往下走，所以守卫只会用到这一类访问。见 [测试与验收的隔离](../architecture.md#测试与验收的隔离) 中的允许/禁止表。
- **屏障配置。** 保留 `deny file-read* file-write* (subpath …)` 和 `send-signal SIGKILL`，另加一条 `allow file-read-metadata (literal <入口>)`。入口之下的任何访问，包括 `stat`，仍然会使进程被终止。
- **增加正向对照。** 每次运行先用待测二进制真正执行一次"物理解析 → 守卫 → 拒绝"，必须以 `state_ownership_refused` 结束、不被终止，以此证明屏障允许的范围已经足够让守卫完成判定。阴性对照保留。
- **屏障不可用时测试失败。** 复评在嵌套沙箱中得到 `sandbox_apply` exit 71，而我在 Repair 1 中是在没有外层沙箱的本机上得到 exit 137，两者不矛盾。现在任务里写明：这类测试必须在没有外层沙箱的环境中运行，不可用时测试失败，不降级。

这个配置在本机、没有外层沙箱的环境中验证过：入口元数据放行，入口之下的访问被终止；详细结果见 [架构记录](architecture.md#repair-2--2026-11-05) 中 Repair 2 的验证。守卫尚未实现，所以正向对照只是写进了任务要求，并没有声称已经执行。

### 随上游修复调整的分解

- 任务 1：新增物理解析 `walk`，并要求用一个对照测试证明：同一路径先 `Clean` 会被误判，`walk` 则会拒绝；新增所有权锁的实现与测试。
- 任务 2：`GuardStateRoot` 返回物理路径，调用者只能使用这个返回值；新增所有权会话；把"所有权转移测试"改为 5 种受控交错测试，覆盖复评给出的时序、只读路径、跨轮和按轮持锁的长驻进程；删除对 `state.lock` 串行的依赖。
- 任务 4：doctor 的输出与原型中五个 `doctor_*` 状态逐字一致。
- 任务 5：新增 `install register` 和 `install unregister`；接管只以排他锁作为同步边界，删除 `lsof`，测试中改用持有共享锁的替身进程模拟旧拥有者。
- 任务 6：App 全程持有共享锁。
- 任务 8：第 4 步改为"旧拥有者有会话正持有共享锁"。

任务的数量、名称和依赖关系都不变。

### 对 Repair 1 验证说明的更正

Round 2 复评指出，`scripts/check-docs.py:51-60` 会通过 `git ls-files --others --exclude-standard` 把未跟踪文件纳入检查。我在 Repair 1 中写的"check-docs.py 不会检查未跟踪文件中的链接"不适用于当前 HEAD。我当时用来得出这个结论的探针（设计阶段）有误；本轮 `check-docs.py` 确实发现了一个未跟踪文件中的失效锚点，这也证实了评审的说法。Repair 1 的原文不改，以本更正为准。

### 验证

文档结构检查与 [架构记录](architecture.md#repair-2--2026-11-05) 中 Repair 2 的验证相同，均通过；`check-topic-docs.sh` 的结果为 exit 0，文档集合不变。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `192c97118444f0845b8f488953e11626d4ff79b3`；指纹 `707f63af7ca8f0d0e9f91dbdcab4689de96b706ce40dc8f3bcd2886e38c0b0fd`，配方同 Round 1。需求 blob 仍为 `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`，与 Round 2 PASS 时相同。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 3 — 2026-11-05

### 📋 任务分解复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 claude-code Repair 2，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `ad191437bf7c8af9c2a2d70ecafa59499b4c4b5c`；fingerprint `a27bb171859f5f43629448e573afaa8e2ab82bee70ea5d2f57bd18e98e419448`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r3-20261105-l2xNcW-final-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮精确态查询为 FAILED；review 失败证据与 L0 通过证据均适用，历史状态未被覆盖；invalidated_evidence 和 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0。前者含空白、diff、链接、文档集合和记录结构检查；结构通过不代表语义通过。
- Method / Scope: ISO-TASKS-R1-F2 及 Repair 2 对任务 1/2/4/5/6/8 的影响、输入文档批准状态。
- Candidate before status sync: blob `192c97118444f0845b8f488953e11626d4ff79b3`；fingerprint `707f63af7ca8f0d0e9f91dbdcab4689de96b706ce40dc8f3bcd2886e38c0b0fd`。
- Final state 仅更新当前评审状态；任务要求和矩阵勾选未改变。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

本轮不为同一跨文档缺陷另起编号，以下问题由对应记录持有完整事实和修复建议，并直接影响当前分解：

- **ISO-ARCH-R1-F2 — P1，仍开放。** [任务 1/5/8](../tasks.md#5-installer-receipts) 的 opt 实时解析、formula 只改 caveats、交错测试仅覆盖 register/claim，仍遗漏“旧会话持锁期间 formula 升级切换 opt”的交接路径。完整反例见 [架构 Round 3](architecture.md#round-3--2026-11-05)。
- **ISO-ARCH-R3-F1 — P2，新增。** [任务 6](../tasks.md#6-app-state-location) 第 188–189 行要求所有 App 启动即长期持锁，未区分未安装 Debug App；须按架构修复后的会话资格调整实现和测试。
- **ISO-UX-R3-F1 — P2，新增。** [任务 4](../tasks.md#4-installation-diagnostics) 第 143 行要求逐字实现五种 doctor 标本；其中两种 state_missing 标本的 partial 语义错误，完整事实见 [CLI Round 3](ux-cli.md#round-3--2026-11-05)。应在标本修复后更新验收基准。

这些是本任务分解已经采用的错误约束及遗漏路径，不是以别的文件“还没提交”为由阻断。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-TASKS-R1-F2 — CLOSED。** 第 53–58 行与架构第 243–269 行明确允许保护入口自身的 metadata 查询，仍禁止入口内部访问；增加真正执行解析/守卫/拒绝的正向对照，以及内部访问触发终止的阴性对照。不再要求既执行 Lstat 又禁止其所需权限，上一轮的约束互斥已消除。尚未实现的正向对照被准确保留为未来任务要求，没有宣称已执行。

ISO-TASKS-R1-F1 保持 CLOSED。新增五种受控交错覆盖普通 register/claim、只读和长驻会话；旧 state.lock 依赖已删除。

#### 📝 总结

本文件自己的历史发现全部关闭；但当前分解直接采用上述尚有缺陷的架构和原型要求，故不批准该候选，整体仍 FAIL。完整发现按各自载体管理，不复制成新的任务发现，也不恢复旧已关闭项。九个实现任务继续未启动，来源缺陷保持开放。

Evidence: 文档和原型内容身份已核对；上一轮嵌套 sandbox-exec 不可用的环境限制没有改变，本轮没有重复相同失败探测。该限制不阻断对修复后的设计约束一致性作判断；真实屏障和守卫的正向运行验收仍须在实现阶段按任务要求完成。共享标本检查器通过不能覆盖稳定 Doctor 契约的语义差异。最终 L0、主题集合检查和精确态门禁在收口字段登记。

修复范围：按 ISO-ARCH-R1-F2、ISO-ARCH-R3-F1、ISO-UX-R3-F1 同步任务 1/2/4/5/6/8 的受影响边界；不重做已经关闭的验收注入和元数据屏障设计。

## Repair 3 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：按 ISO-ARCH-R1-F2、ISO-ARCH-R3-F1、ISO-UX-R3-F1 三项的修复，同步任务 1、2、4、5、6、8。只修改 `tasks.md`。这三项的完整处置分别记录在 [架构记录](architecture.md#repair-3--2026-11-05) 和 [CLI 记录](ux-cli.md#repair-3--2026-11-05)。
- Beads：`ad-installed-state-ownership-doc-tasks` 由 claude-code 领取修复。

### 分解的同步

- **任务 1**：回执登记不可变真实路径，判定时不解析链接；新增"opt 切换不改变所有权"的直接回归测试（即 Round 3 反例）；新增续登条件的表驱动测试，覆盖四个条件全部满足，以及逐一缺少其中一个。
- **任务 2**：守卫在拒绝之前先判断是否续登；受控交错测试从 5 条增加到 8 条，新增：旧会话持锁期间 opt 被切换、旧 keg 删除后的续登、替换已登记文件时的"撤销 → 替换 → 重新登记"。
- **任务 4**：写明 doctor 两种短路（状态目录缺失、状态目录被拒绝）都是部分报告，各自使用自己的结尾一行；固定测试必须覆盖"未安装 + 状态目录缺失"这一组合。
- **任务 5**：认领时登记 keg 内的真实路径；新增"撤销 → 替换 → 重新登记"的要求和测试；在真实 brew 回归测试中，除了验证 cask 能写 `$HOME`，还要验证升级时旧版本的 uninstall 步骤先于替换执行。
- **任务 6**：App 只有服务于真实状态时才持有长期会话；新增"Debug App 单独运行时，安装和接管都能完成"与"已安装 App 阻止接管"的区分测试，以及锁内再次查询、失去所有权后释放锁的测试。
- **任务 8**：新增模拟 formula 升级（续登成功、保留旧 keg 时不续登）和未安装 App 替身不阻塞安装这两步。

任务的数量、名称和依赖关系都不变。

### 验证

文档结构检查同 [架构记录](architecture.md#repair-3--2026-11-05) 中 Repair 3 的验证，都通过；`check-topic-docs.sh` 为 exit 0，文档集合不变。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `e37c302b7d5eaebb0cc8dce130046291530b76cc`；指纹 `0a2d978daa3e6432ae6cf49e75850688593ef3b2f6dbc746f418e7ad1e3cbfd3`，配方同 Round 1。需求 blob 仍为 `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 4 — 2026-11-05

### 📋 任务分解复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code 的 Repair 3，未委派，未修复设计、产品代码或标本。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `bb7c82e57b121fd4c6c88f8cc3aeb0a4832af7cc`；fingerprint `5068961054ca186949093cef235e006c97e356f87a73f9a17426420f20bb8c1f`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r4-20261105-QC3iyb-final-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮固定 gate-status 对精确目标返回 FAILED；本轮 review 与 L0 证据均适用，无失效证据或未决影响。历史内容状态和结论保留。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者覆盖空白、diff、链接、文档集合和记录结构。
- Method / Scope: Repair 3 对任务 1/2/4/5/6/8 的同步、文档依赖和全部历史发现的保持情况。
- Candidate before status sync: blob `e37c302b7d5eaebb0cc8dce130046291530b76cc`；fingerprint `0a2d978daa3e6432ae6cf49e75850688593ef3b2f6dbc746f418e7ad1e3cbfd3`。
- Final state 只勾选通过的 CLI Review 并更新当前评审状态；任务行为内容未修改。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

完整发现由 [架构 Round 4](architecture.md#round-4--2026-11-05) 承载，不创建重复编号：

- **ISO-ARCH-R4-F1 — P1，新增。** 任务 2 第 8 条、任务 5 第 181 行逐字采用“撤销 → 替换 → 重新登记”，但交错测试只覆盖一个安装器与旧会话，未覆盖另一个安装器在撤销后的窗口重新登记。该分解必须随持续交接事务的方案补齐边界和测试。
- **ISO-ARCH-R4-F2 — P2，新增。** 任务 2 把续登放在守卫的真实根拒绝分支，任务 8 第 240 行只要求“首次访问时续登”，没有验证无 --state-dir 的默认调用。默认根已被选成隔离根时，该路径绕过续登，应按架构修复对齐解析顺序和验收。

这些缺陷已被本分解作为实现约束采用，因此候选不能批准。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-TASKS-R1-F1/F2 保持 CLOSED。上一轮继承的 ISO-ARCH-R1-F2、ISO-ARCH-R3-F1 和 ISO-UX-R3-F1 的原反例均随修复关闭：任务新增固定 keg 身份与续登条件测试、区分未安装/已安装 App 会话、将缺失状态目录的 doctor 恢复为部分报告。

#### 📝 总结

需求与 CLI 已通过；架构和采用其新协议的任务分解仍 FAIL。未创建或启动九个实现任务，来源缺陷保持开放。后续只修复新记录的两个边界，不能把已关闭历史项继续当作待办。

Evidence: 核对本轮任务文件和对应上游文档；独立模型反例及 CLI 正负检查由相应记录给出。最终状态同步不改变任务要求，目标指纹使用同步后的 blob。结构检查与精确态门禁在收口字段登记。

修复范围：随 ISO-ARCH-R4-F1、ISO-ARCH-R4-F2 对齐任务 1/2/5/8 的受影响定义和测试；保留已通过的需求和 CLI 契约。

## Repair 4 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：按 ISO-ARCH-R4-F1、ISO-ARCH-R4-F2 的修复，同步任务 1、2、5、8。完整处置见 [架构记录](architecture.md#repair-4--2026-11-05)。只修改 `tasks.md`。
- Beads：`ad-installed-state-ownership-doc-tasks` 由 claude-code 领取修复。

### 分解的同步

- **任务 1**：校验回执的 `state` 字段（`installed` / `reserved`）及各自允许的字段，预留期间判定为未安装、过期后按"不存在"处理；续登函数供两个入口共用。
- **任务 2**：
  - 写明续登的两个入口：默认目录解析在选择目录之前续登，失败时返回 `state_ownership_refused`、不退回隔离目录；以及守卫的真实根分支；
  - 交错测试从 8 条增加到 11 条：第 8 条改为"预留 → 替换 → 完成"；新增第 9 条，覆盖两个安装方式竞争同一路径，即 R4-F1 的反例，包括同一路径、别的路径、带或不带 `--takeover`、被预留路径已不存在这几种组合；新增第 10 条预留过期；新增第 11 条，用不带 `--state-dir` 的普通命令做端到端续登，即 R4-F2。
- **任务 5**：新增 `install reserve`；原地替换改为"预留 → 替换 → 完成"，替换失败时清除自己的预留；真实 brew 回归测试改为验证 uninstall 步骤中的 `install reserve` 先于替换执行、install step 完成同一份预留，并新增 cask 真正卸载后的预留处理测试。
- **任务 8**：模拟升级后，用不带 `--state-dir` 的普通命令验证续登，并确认读到原有数据。

任务的数量、名称和依赖关系都不变。

### 验证

文档结构检查同 [架构记录](architecture.md#repair-4--2026-11-05) 中 Repair 4 的验证，都通过；`check-topic-docs.sh` 为 exit 0。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `2e54f50c9d83cd0b054d2a53ace4d37f2368dd70`；指纹 `c5dc3e04dd566452c0bc4ed6ff83f9ed4d5fd55894dcebcbab42f0895fc9697e`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 5 — 2026-11-05

### 📋 任务分解复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code Repair 4，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `adb83792f4e460677d907eb111ecae339d9becb2`；fingerprint `9978a3b43f7c110f9352ee9e685e9713780d12e98cbd4ad3d33a06af55fa0392`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r5-20261105-0wyFKc-final-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮固定 gate-status 对精确目标返回 FAILED；review 失败与 L0 通过证据均适用，历史状态保留，无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者含空白、diff、链接、文档集合和记录结构检查。
- Method / Scope: Repair 4 对任务 1/2/5/8 的同步、预留交错与默认调用验收。
- Candidate before status sync: blob `2e54f50c9d83cd0b054d2a53ace4d37f2368dd70`；fingerprint `c5dc3e04dd566452c0bc4ed6ff83f9ed4d5fd55894dcebcbab42f0895fc9697e`。
- Final state 仅更新简短评审状态；文档勾选与任务行为定义未改。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R4-F1 — P1，仍开放，完整载体为 [架构 Round 5](architecture.md#round-5--2026-11-05)。**

- [任务 2](../tasks.md#2-state-root-guard) 第 9 条要求 B 的同路径登记失败、A 的完成成功，却没有给测试提供不同的事务身份输入；仅以模型角色 A/B 区分会把待证明的性质直接当作前提。
- 第 10 条只测试“预留后崩溃并过期”，遗漏“暂停超过期限后恢复执行替换”。旧执行者仍可能在新的拥有者活跃时覆盖文件。
- 任务 1 的回执 schema 和任务 5 的完成/取消/发布动作须随架构修复同步；需要测试实际协议可观察的事务资格和过期后的写入隔离。沿用同一发现编号，不复制成新的任务缺陷。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-ARCH-R4-F2 对本分解的影响已关闭：默认解析前续登、失败不退回隔离根、显式隔离不续登都有明确要求，且第 11 条与任务 8 使用默认命令验收。ISO-TASKS-R1-F1/F2 以及其他历史已关闭发现保持关闭。

#### 📝 总结

需求、CLI 保持通过；任务分解仍采用未闭合的预留协议，故 FAIL。剩余修复只对应 ISO-ARCH-R4-F1。没有创建或启动实现任务，没有关闭来源缺陷。

Evidence: 比对本轮 task blob 和上游架构规则；所需反例的完整输入、时序和观察在架构记录自包含记载。状态同步没有改动任务定义，最终 ContentState 绑定同步后的文档。L0 与门禁结果在收口字段登记。

修复范围：同步 ISO-ARCH-R4-F1 对任务 1/2/5/8 的受影响约束和交错测试，保留默认续登已关闭结论。

## Repair 5 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：按 ISO-ARCH-R4-F1 的 Repair 5，同步任务 1、2、5。完整处置见 [架构记录](architecture.md#repair-5--2026-11-05)。只修改 `tasks.md`。
- Beads：`ad-installed-state-ownership-doc-tasks` 由 claude-code 领取修复。

### 分解的同步

- **任务 1**：回执校验恢复为单一记录，删除 `state` 字段和预留记录。
- **任务 2**：交错测试第 8 至 10 条重写：
  - 第 8 条：在锁内发布；在"发布之后、写回执之前"暂停，断言锁外看不到中间状态；
  - 第 9 条：两个安装器以完全相同的参数竞争同一路径（评审第五轮反例 1），另测不同安装方式以 `owned_by_other` 拒绝；
  - 第 10 条：持锁暂停跨越 601 秒、锁外暂停、崩溃三种情形（评审第五轮反例 2）。
- **任务 5**：隐藏命令改为 `install register --source`、`install unregister --remove`，删除 `install reserve`；安装脚本既不写回执，也不把文件移到最终位置；新增"在锁内发布"的要求和失败注入测试；cask 改为由 install step 发布 App 的五项真实 brew 验证条件。

任务的数量、名称和依赖关系都不变。

### 验证

文档结构检查同 [架构记录](architecture.md#repair-5--2026-11-05) 中 Repair 5 的验证，都通过；`check-topic-docs.sh` 为 exit 0。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `604c1c4691a66bc2564d91428ab35dc454595dfb`；指纹 `f22d05e0620251048f4346a514fe63c254188d93e7836796d3fe2500fbf30a1b`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 6 — 2026-11-05

### 📋 任务分解复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 claude-code Repair 5，未委派，未修复设计、产品代码或测试。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `406c9d0b92885549d73109dce712592efae96207`；fingerprint `395bcae67e30cc07a1b0039ad6a03775130e9f75a39363ea404fe34419012a1a`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r6-20261105-ekfE00-final-state`。
- 完成门禁：FAILED
- Gate evidence: 固定 gate-status 对本轮精确目标返回 FAILED；review 失败与 L0 通过证据均适用，历史状态保留，无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者含空白、diff、链接、文档集合和记录结构检查。
- Method / Scope: Repair 5 对任务 1/2/5 的同步，特别是连续持锁和完整 cask 清理链。
- Candidate before status sync: blob `604c1c4691a66bc2564d91428ab35dc454595dfb`；fingerprint `f22d05e0620251048f4346a514fe63c254188d93e7836796d3fe2500fbf30a1b`。
- Final state 仅更新当前评审状态；矩阵勾选和任务行为内容未改。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。

#### 🔴 严重问题 — 必须修复

**ISO-ARCH-R4-F1 — P1，仍开放；完整事实由 [架构 Round 6](architecture.md#round-6--2026-11-05) 承载。**

- 任务 5 第 181、184 行要求全部删除受所有权锁保护，却引用仍保留 `uninstall delete:` 的架构。正常安装/卸载成功与 helper 内部交错测试，不能排除 Homebrew 在 helper 返回后的另一条删除路径。
- 按架构修复对齐任务 5 的完整卸载/失败清理范围，增加旧卸载链与新同路径安装之间的交错；检查的是全部 actor 的最终文件操作，不仅是 install helper 的内部锁。
- 不创建重复发现编号，不重开已经解决的事务身份、TTL 或默认续登问题。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

第 8–10 条已改为在一次持续持锁内发布和登记，覆盖相同参数、锁内/锁外暂停、崩溃；删除预留与有效期，正确承接了上一轮主要修复。任务 5 明确了失败注入与真实 brew 验证条件。历史 ISO-TASKS-R1-F1/F2、默认续登及 App 独立等关闭状态保持。

#### 📝 总结

当前分解仍采用有锁外删除兜底的 cask 方案，因此 FAIL；剩余项仍只有 ISO-ARCH-R4-F1。实现任务未创建或启动，来源缺陷保持开放。

Evidence: 核对 task blob、架构引用、Homebrew 的 script/delete 分派事实及最小交错模型。必要原始事实已收录架构记录，不引用临时文件。状态同步不改变任务内容；最终 L0 与精确态门禁在收口字段登记。

修复范围：同步 ISO-ARCH-R4-F1 对任务 5 及受影响清理/交错测试的要求，保留主发布流程已修复结论。

## Repair 6 — 2026-11-06

- Implementer：OpenCode（GPT-6 Astra），修复角色，未委派，不自判复评 PASS。
- Scope：同步 ISO-ARCH-R4-F1 的完整清理链修复；完整事实和验证见[架构 Repair 6](architecture.md#repair-6--2026-11-06)。
- Workspace：`agent-deck.installed-state-ownership` / `feature/installed-state-ownership`；Beads：`ad-installed-state-ownership-doc-tasks`。

### 分解的同步

- 任务 1：校验用于清理归属的 `installation_id`，同路径/channel/版本、相同时间戳的登记也生成新 ID；不改变运行时路径身份判定。
- 任务 2：保留连续持锁交错测试，并同步清理清单参数；新增第 12 条，覆盖旧 helper 返回后 B 同路径安装并开始会话、继续 A 全部清理与重复调用，包含同 channel 重装。
- 任务 5：发布前保存原安装清单，卸载/回滚必须在排他锁内按原 ID、channel 和路径条件清理；新增 helper 损坏、缺少清单、回执损坏、删除失败、延迟回滚与强制跳过脚本的要求。真实 brew 测试必须运行 helper 返回后的完整分派，并检查无最终 App 的路径式兜底删除。
- 文档矩阵保持架构与任务分解 Review 未勾选，简短状态改为修复候选待复评；任务数量、名称和依赖不变，未创建或启动实现任务。

### 验证与候选

`python3 scripts/check-docs.py` exit 0；架构记录中的隔离模型覆盖旧反例及新的条件清理规则，不替代未来产品或真实 brew 验收。

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`tasks.md` blob `ba007c729685812747372b2a78ae2f8b93b0eac7`；指纹 `306641ae533f418ddf643dfdace2e2d9d43e47a79efa8ace8c625bd44f9eb0d9`，配方仍为 SHA-256 of `head=<HEAD>;document=<blob>`。

Round 6 的历史结论保留；本轮只交接修复候选，不声明文档完成或新 CEv1 通过。来源缺陷保持开放。

## Round 7 — 2026-11-06

### 📋 任务分解复评报告

📊 总体评分：8/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 OpenCode Repair 6 的任务分解同步，未委派，未修复任务行为或产品内容。
- Method: 将九个任务的范围、依赖和验收要求映射到已通过的需求、CLI 和架构；复用架构 Round 7 的清理链处置，独立核对默认/显式状态目录的测试预期。
- Scope: 本任务分解及其本地历史；不重新签发其他文档的评审结论。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: 同步后 tasks.md blob `530b96d59fe67442229912dd3a8120c0f3361e6a`；fingerprint `b375d1a06c8d329b5c2a76f0aeeaa0dc7f2f3823bd8ae1f5498b54e1a44e37f1`。
- Candidate before status sync: blob `7f70f0d462f78cdd891d66a705dc4f0f0b67259d`；fingerprint `7b4c5f751262ffc07d090bb9dbd26b8048cb608a41e58abb7c19b65276e1b71e`。同步只更新任务分解的当前评审状态，任务 2 的断言原文和全部任务行为均未改。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r7-20261106-7MHYmt-final-state`。
- 完成门禁：FAILED
- Gate evidence: 固定 gate-status 对本轮精确目标返回 FAILED；新 review 失败证据与 L0 通过证据均适用，无失效证据或未决影响；历史状态保留。
- L0 results: `python3 scripts/check-docs.py` 与显式主题文档集合检查均 exit 0；结构通过不代表验收预期的语义正确。

#### 🔴 严重问题 — 必须修复

**ISO-TASKS-R7-F1 — P2 — NEW / OPEN：结构性回归测试把正常默认隔离调用也一律要求为拒绝。**

- 位置：[tasks.md](../tasks.md#2-state-root-guard) 第 103–104 行。
- 行为风险：符合已批准产品契约的实现会被任务验收错误地判失败，或实现者为了通过测试把默认开发运行也改成拒绝。该断言不能同时验证“默认隔离可用”和“显式真实根拒绝”。
- 证据：第 104 行要求分别使用“默认参数”和“--state-dir 指向注入真实目录”运行全部叶子命令，随后不分分支地断言“每条命令都以 state_ownership_refused 结束”，只列 doctor 例外。已通过 [architecture.md](../architecture.md#状态目录解析) 第 105、111、167 行规定：未安装且不满足续登条件时默认选择 .agentdeck-dev；不命中保护集合 P 的路径直接放行。注入真实账户主目录不会把这个默认隔离根改成真实根。
- 独立读取已批准的共享标本（ownership.js blob `b0d100100fc4caf9bf56dda156e6edd641d86fa4`）得到：`notice.command = go run ./cmd/agentdeck provider list`、`exit = 0`、stdout 为 `No providers.`、stderr 明确使用隔离目录；`refused_state` 则是显式真实根，exit 1 / state_ownership_refused。两种输入的正确结果本就不同。
- 同一任务第 103 行还要求 Hook/statusline 静默 exit 0；第 104 行的统一拒绝断言也必须保留这些已有命令例外，不能只列 doctor。
- 💡 限定修复建议：按输入和命令契约拆开预期。默认调用应验证选择隔离根、真实根无内容访问，并保留该命令的既定结果（例如 provider list 成功；客户端配置写入仍按其拒绝契约）；显式真实根调用验证访问前拒绝，同时保留 doctor 部分报告和 Hook/statusline 静默成功等既有例外。共用访问屏障和文件不变断言，不把所有默认命令都强制改成拒绝。修复任务定义即可，不改变已通过的架构或 CLI 行为。
- Disposition: OPEN。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

- **ISO-TASKS-R1-F1 — 保持 CLOSED。** 测试专用 acceptance tag、账户/回执注入与生产静态证据边界均明确。
- **ISO-TASKS-R1-F2 — 保持 CLOSED。** 访问屏障区分入口元数据与内部内容，保留正负对照和不可用时的失败处理；本轮新问题是命令预期混用，不恢复旧的“mtime 能证明无读取”错误。
- **继承的 ISO-ARCH-R4-F1 — 对本分解 CLOSED。** 已通过的架构 Round 7 内容为 blob `9a5e5a349489c7dacbeefb68ee327c19e7ba7861`，其精确态 gate VERIFIED。任务 1 增加每次安装独立 ID；任务 2 第 12 条覆盖旧 helper 返回后的同路径新安装及同 channel 重装；任务 5 覆盖原清单归属、小组件清理、缺损恢复、延迟回滚、完整 Homebrew 分派及强制路径。原清理链缺口已承接，不再保持开放。
- 此前继承的路径解析、默认续登、App 会话资格及 doctor partial 问题也保持关闭。文档集合声明为四份，未添加无关产品范围。

#### 📝 总结

旧发现均已关闭或保持关闭；新增 ISO-TASKS-R7-F1 未关闭，因此任务分解 FAIL。需求、CLI、架构的既有 PASS 保留。该结果没有批准实现分解，未创建或启动后续实现任务，也未关闭来源缺陷。

Evidence: `bash scripts/check-topic-docs.sh installed-state-ownership` exit 0，文档集合结构完整。独立 Node 检查读取实际标本并断言默认 notice 的 exit 0、隔离根提示和 stdout，以及显式真实根拒绝的 exit 1 / 错误码，全部得到预期值；这是契约矛盾的证据，不是运行了尚未实现的隔离 CLI。上游架构模型与门禁在同一未变 blob 上复用，没有重跑真实安装或产品测试。取得决定性矛盾后停止扩大验证。最终 L0 与门禁结果在收口字段登记。

修复范围：ISO-TASKS-R7-F1；拆分任务 2 第 104 行的输入/命令预期，保留已批准的默认隔离、拒绝及静默输出契约。

## Repair 7 — 2026-11-06

- Implementer：OpenCode（GPT-6 Astra），修复角色，未委派，不自判复评 PASS。
- Scope：ISO-TASKS-R7-F1；修改任务 2 的结构性回归验收要求、任务分解状态摘要及本修复记录。
- Workspace：`agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Beads：`ad-installed-state-ownership-doc-tasks`；核验 Round 7 的无受让人修复交接后由 opencode 接受。

### ISO-TASKS-R7-F1 处置：已修复，待复评

确认原断言与已批准契约矛盾：未安装且不满足续登条件时，默认根是 `.agentdeck-dev`，未命中 `P` 的路径放行；默认 `provider list` 可以成功，显式真实根才触发访问前拒绝。原文把两组输入的所有命令都要求为 `state_ownership_refused`，还遗漏 Hook/statusline 的静默返回。

[任务 2](../tasks.md#2-state-root-guard) 的修复映射：

1. 明确默认隔离组的夹具前提：未安装、不满足 formula 续登条件、默认隔离根不命中 `P`；“默认”只是不传 `--state-dir`，其余参数、stdin 和数据必须足以触发实际命令路径。每例独立，续登另用既有第 7、11 条夹具。
2. 默认调用保持命令契约：空 provider 的 `provider list` 为 exit 0 / `No providers.`，TTY 提示遵守 notice 的格式条件；doctor 在隔离根缺失时报告 `state_missing` 部分报告；写真实客户端配置仍按 `resource: client_config` 拒绝。Hook/statusline 的正常隔离处理不被误设为安装拒绝。
3. 显式真实根组拆成普通状态命令的 exit 1 / `resource: state_root`、doctor 的 exit 0 / 部分报告、Hook/statusline 的 exit 0 / stdout 与 stderr 为空且不写拒绝记录。
4. 全部叶子命令仍须有逐项预期和契约依据；不解析状态或有独立契约的入口不被强制套用普通状态命令错误，也不能无依据跳过。两组继续共用访问屏障、正负对照和真实目录不变断言；worker 与包级入口按各自目录分支和错误契约验证。

### 验证与候选

- 逐项对照已批准的 [CLI 状态矩阵](../ux/cli.md#状态矩阵)、[提示条件](../ux/cli.md#未安装提示)、[doctor 部分报告](../ux/cli.md#状态目录被拒绝时的部分报告) 和[架构的状态目录解析](../architecture.md#状态目录解析)：验收要求现在区分默认隔离、客户端配置拒绝、显式真实根拒绝及两类输出例外。
- `architecture.md` blob `9a5e5a349489c7dacbeefb68ee327c19e7ba7861`、`requirements.md` blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`、`ux/cli.md` blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a` 均保持输入状态。共享标本 blob 仍为 `b0d100100fc4caf9bf56dda156e6edd641d86fa4`，复用 Round 7 已记录的 notice/refused_state 检查结果；本轮没有新的产品运行证据。
- L0：`python3 scripts/check-docs.py` exit 0，覆盖文档链接、主题集合、记录结构、空白和 diff 检查；结构检查不签发语义 PASS 或 CE 门禁结论。

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；输入 `tasks.md` blob `530b96d59fe67442229912dd3a8120c0f3361e6a`；修复候选 blob `8979eb1663b7a60f7d57a5503a35447967d83110`；指纹 `32b4dd70126f36fa38754346a0a8fa1b36297ae8d3a848b2c2f1a62ebefa63c6`。配方仍为 SHA-256 of `head=<HEAD>;document=<blob>`。

任务分解的 Review 保持未勾选；上一轮 FAIL / FAILED 是其原内容状态的历史结论，本轮交接修复候选供独立复评，不签发新的 PASS 或 CEv1 完成结论。产品实现、后续实现任务登记及 Git 交付未执行。

## Round 8 — 2026-11-06

### 📋 任务分解复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；独立复核 OpenCode Repair 7，未委派，未修复任务行为或产品内容。
- Method: 对照已批准的状态目录/CLI 契约检查分支预期，复核历史发现处置和九个任务的需求覆盖，执行文档集合及 L0 检查。
- Scope: tasks.md 的分解、验收定义和文档集合；其他已通过文档作为未变输入复用，不重复其语义评审。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: 同步后 tasks.md blob `d6fe1e613293767e7159a1e884955b7291a66aec`；fingerprint `cc877a837a9dc4c92bfbe654ea72333b63158cff3960cc8b9a52419973dc72fb`。
- Candidate before status sync: blob `8979eb1663b7a60f7d57a5503a35447967d83110`；fingerprint `32b4dd70126f36fa38754346a0a8fa1b36297ae8d3a848b2c2f1a62ebefa63c6`。同步只更新本文件 Review、当前文档状态和分解批准状态；任务行为定义未改。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:tasks.md`；Beads: `ad-installed-state-ownership-doc-tasks`。
- Target ContentState: `installed-state-ownership:tasks:rereview-r8-20261106-6RZ5IS-final-state`。
- 完成门禁：VERIFIED
- Gate evidence: 固定 gate-status 返回本轮精确态 VERIFIED；review/L0 两项必需标准均通过，review 通过 depends_on 复用未变化的需求、CLI、架构通过证据；missing、invalidated、unresolved 均为空。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-TASKS-R7-F1 — CLOSED。** 任务 2 第 104–118 行不再对全部命令统一断言拒绝：

- 默认组明确为未安装、不满足续登条件、隔离根不命中 P；提供有效参数、stdin 和数据，避免用参数错误提前退出伪造覆盖。
- 默认普通状态命令保留其正常结果；provider list 明确为 exit 0 / No providers.，doctor 检查隔离根，真实客户端配置写入仍按自身契约拒绝。
- 显式真实根分别断言普通状态命令的 exit 1、doctor 的成功部分报告、Hook/statusline 的静默 exit 0。正常隔离 Hook 不被误标成安装拒绝。
- 不解析状态的入口和独立契约命令仍逐项说明预期，不无依据排除；共同的不变量是访问边界，而非退出码。worker、包级调用和续登使用各自适当的夹具。

**历史处置。** ISO-TASKS-R1-F1/F2 保持 CLOSED；采用的 ISO-ARCH-R4-F1 已由架构 Round 7 PASS/VERIFIED 关闭，并在任务 1/2/5 中完整承接安装 ID、原清单条件清理、完整 Homebrew 分派、同 channel 重装、损坏 helper 和延迟回滚测试。此前采用的路径解析、默认续登、App 会话资格和 doctor partial 问题均保持关闭。

**覆盖核对：**

| 已批准边界 | 分解承接 |
| --- | --- |
| G1 安装身份、回执与路径识别 | 任务 1 |
| G2/G3/G9 状态隔离、访问资格与交接 | 任务 2 |
| G4 客户端配置写入边界 | 任务 3 |
| G6/G8 安装诊断和 helper 状态路径 | 任务 4、6 |
| G7/G9 安装、续登、发布、清理归属 | 任务 5，复用任务 1/2 |
| G5 Debug App/Widget 独立身份 | 任务 7 |
| A1–A9 隔离验收、失败与恢复 | 任务 8，及各组件的定向验收 |
| 稳定产品契约同步 | 任务 9 |

四份 Documents 与在盘主题文档一致；菜单栏/Widget/Settings 的适用性声明保持原范围。代码任务的数量、名称、文件边界和依赖未被本轮改写。

#### 📝 总结

全部已记录的任务分解发现关闭，无新增发现，PASS。此时四份设计文档均通过；主题仍有九个未实施任务，不能宣布产品或整个主题完成。

Evidence: `bash scripts/check-topic-docs.sh installed-state-ownership` exit 0。直接检查修复后的七类输入/命令预期与独立命令规则；复用 Round 7 已核对的 notice/refused_state 数据和上游通过结论，不重复未变化的标本执行、架构模型或浏览器检查。requirements.md blob `2c4f3e37b8452ab2483908a98cc8be2a08366cf0`、ux/cli.md blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a`、architecture.md blob `9a5e5a349489c7dacbeefb68ee327c19e7ba7861` 和 ownership.js blob `b0d100100fc4caf9bf56dda156e6edd641d86fa4` 都保持通过时内容。未来真实 brew、原生 App、小组件、屏障和生产实现验证仍按相应任务执行，不计为本轮完成证据。最终 L0 与精确态门禁在收口字段登记。

调度同步：门禁满足后已登记 9 个实现任务和 9 个对应开发授权门。读回确认全部 open、无受让人，依赖与批准的矩阵一致（包含任务 3 相对任务 2 的顺序约束），批准文档的 ContentState/指纹元数据一致。首任务为 `ad-installed-state-ownership-installation-identity`，当前只等待其开发授权门。来源缺陷保持 open，并阻塞于 `ad-installed-state-ownership-contract-reconciliation`（该任务依赖任务 1–8），仅在实际修复交付及其条件满足后关闭。登记没有启动开发，也不是 Git 交付。

Task checkpoint：ad-installed-state-ownership-doc-tasks；content_state `cc877a837a9dc4c92bfbe654ea72333b63158cff3960cc8b9a52419973dc72fb`；gate VERIFIED。
提交建议：经明确授权将四份已通过主题文档、相关共享 CLI 原型、各评审记录和状态作为一致的文档批次交付。本轮未提交。
推送建议：目标 origin/feature/installed-state-ownership；需显式推送授权及对应提交对象、签名、范围检查。本轮未推送。
