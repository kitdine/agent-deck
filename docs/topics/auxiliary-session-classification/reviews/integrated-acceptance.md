---
status: active
topic: auxiliary-session-classification
subject: integrated-acceptance
---

# integrated-acceptance 评审记录

## Round 1 — 2026-11-11

## 📋 Task 9 integrated-acceptance 独立评审

📊 总体评分：5/10

✅ 结论：FAIL

- Reviewer: Claude Code（Beads actor `claude-code`）。本会话为冷上下文，未参与任务 9 的实现；
  开发交接同样以 `claude-code` 记录，属于同一客户端的另一会话，独立性仅来自冷上下文与评审角色，特此说明。
- Method: 工作区绑定与候选指纹复算、开发交接与 Beads 记录核对、逐文件 diff 审查、
  对照已批准的 architecture / ux/cli 与 scan-performance 测量契约、读取测量工具源码、CEv1 工作单元查询、
  直接运行 Go 回归；未委派，未修改产品代码、测试或配置。
- Scope: 任务 9 候选的全部 14 个路径：联合验收用例、SQLite 锁修复、FTS 内容表触发器移除与显式推进、
  菜单栏标签栏判定与 Widget 批次视图复用，以及交接中记录的性能矩阵结论。
  未重跑性能矩阵与原生测试（见 Evidence 的限制）。
- Reviewed state: HEAD `a2c10771f05c5f681f3a091c617f9dd97e821621`，候选指纹
  `5ce1f4d6b1af0511010ccade12b602e973c221e12d26247c4610d4ae135eb17f`，与开发交接一致。
  算法为以下有序 `path + 空格 + blob + 换行` 字节串的 SHA-256；不含 tasks.md 与评审记录。

| Path | Git blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarViewModel.swift | b294455c17d16b3f1daebe1ca1758f9b200dc691 |
| apps/macos/AgentDeckApp/SessionScopeController.swift | 1b4353386bc54f7cd7f96b223c62f3a22a022411 |
| apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | 16ce7b047cb1b2ce24511c89e998fec9f4f6e04f |
| apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift | ffac93163e34f55b1d509bb94b3c0e34c6f43038 |
| apps/macos/AgentDeckShared/WidgetSessionScopeBatch.swift | f50a235bbeaaedbc433377dd85442cb6e438174b |
| apps/macos/AgentDeckTests/AppGroupSnapshotStoreTests.swift | d7b2f893ccb3e72e1c0661002effcfcaa001513c |
| cmd/agentdeck/session_origin_acceptance_test.go | efb4ee95c29df6af6028f08070e2dfbced7b186c |
| internal/session/ingestion_sql_test.go | 4266ba2696f06ebcfbd30d7d4cfdb8f41da127f8 |
| internal/session/search_text_revision_test.go | 130c308fc30dc70e63e28e87fa74ff945e4fdb09 |
| internal/session/session.go | 1b43ab611171d079ec834ab650b715cdf3a54ef5 |
| internal/sessionorigin/selection_test.go | 1dbdebe9d7bc38e979676de069bcf08e20a12348 |
| internal/store/serving_generation.go | 2dddc408df265ffe8a5dfbd5090372c420f66b9d |
| internal/store/sqlite_lock_unix_test.go | 43b33ef9dd5cb657601d1581153cf3d315be2d92 |
| internal/store/store.go | ec078fa9a30bfc018c3ccc7a61e9a6d9f70cd091 |

### 🔴 严重问题 — 必须修复

**ASC-IA-R1-F1 — P1：移除 FTS 内容表触发器使实现偏离已批准的架构 §6，并重新打开 ASC-SP-R3-F1 关闭的保证。状态：OPEN。**

位置：`internal/store/serving_generation.go:47-57`、`internal/session/session.go:1587-1606` 及各前缀调用点；
未随之更新的 `internal/session/scope_snapshot.go:36-38`。

- 行为风险：候选删除 `session_documents_content` 上的三个触发器，改由 session 包的产品写入路径显式推进
  serving revision。绕过该包对 `session_documents` 的任何写入（直接 SQL、后续新增的写入点）不再推进 revision，
  搜索选择可沿用旧向量与旧 publication 身份。保证从结构性（触发器）变为依赖约定（每个写入点记得加前缀）。
- 合同依据：`architecture.md:265-266` 写明「每个依赖的 INSERT/UPDATE/DELETE（含直接 SQL）都须推进 revision；
  旧 dirty 位不能替代这一保障。sessions 侧……FTS 输入如被读取则补齐」，第 192 行要求读取 FTS 输入时在同一 migration
  补齐 trigger。该文档当前为 Round 5 PASS 且未被修改。`reviews/scope-projections.md` 的 ASC-SP-R3-F1
  正是「直接 SQL 修改 FTS 不推进 serving revision」，其关闭依据就是这三个触发器。
  `tasks.md` 规定跨任务合同改变须先核对已批准架构，任务 9 不得借验收扩张或改变产品定义。
- 证据：`rg` 全仓非测试代码中 `session_documents` 的写入共 10 处，均在 `internal/session/session.go`，
  候选已为它们加显式推进；架构文本与候选行为逐字对照不一致；`scope_snapshot.go:36-38` 的注释仍声明
  内容影子表带触发器、直接 SQL 也推进 revision，与候选相反。
  开发交接本身也写明「此项改变了任务 3 已评审的合同，须复评」。性能动机（P09 比值 1.115 降到 1.030）属实，但不替代合同变更的批准。

💡 有界修复：这是一个需要用户决定的合同取舍，二选一后再复评。
(a) 接受收窄：经 architecture.md 的设计修订与文档评审，把 §4/§6 的保证改为「产品写入路径」，说明直接 SQL 不再覆盖及其理由，
同步 `scope_snapshot.go` 注释，并在 `reviews/scope-projections.md` 记录 ASC-SP-R3-F1 保证范围的变化；
(b) 保留合同：换一种不在 FTS 内容表上挂触发器、但仍对任何写入生效的结构性机制。
不要在未改架构文档的情况下保留当前实现。

**ASC-IA-R1-F2 — P1：P03「切换期间不得有 helper」的行为要求在最终候选上未满足，交接中的「产品失败 0」与测量契约不符。状态：OPEN。**

位置：`apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:2660-2667`（发布后九次批次读取）、
`apps/macos/AgentDeckApp/SessionScopeController.swift:113-125`（批次未到位或非默认范围时仍按需读取）；
`tasks.md` 任务 9 的 2026-11-11 两条交接。

- 行为风险：scan-performance 的 `requirements.md:235` 把 P03「期间不得有 helper/扫描/OTel 等待」定为行为要求；
  `performance-evaluation.md:292` 规定「期间 helper/扫描/OTel 打点必须为零，非零为产品失败」；
  测量工具 `cmd/agentdeck-perf/run.go:323` 在 `backgroundMarkers != 0` 时直接判该运行失败。
- 证据：交接记录首轮正式矩阵中候选臂每个运行单元约 24 次 `desktop session-scope` 启动（基线 0），却写「全部条件工具无效 0、产品失败 0」；
  修复后的复测仍记录 18 条背景打点，来源为发布后 Widget 批次自身的九次读取与批次到位前的少量读取。
  按契约这两种情况都是非零，最终候选没有一份背景打点为零的 P03 正式批次。
  每次发布后顺序启动九个 helper（交接自述约 2.3 秒后台 CPU，未测量资源影响）与此直接相关。
- 说明：批次读取是任务 7 已评审的设计，其成本当时明确「归入任务 9 性能预算」，所以裁定落在本任务。

💡 有界修复：让 P03 的测量窗口内不出现 helper 启动（例如批次读取不与已恢复 publication 后的切换重叠，
或九个组合不再各起一个 helper 进程），随后在最终内容上跑一次 P03 正式批次并如实报告背景打点与产品失败计数；
若认为该行为要求不再适用于带批次的发布，须由用户明确决定并改写对应要求，不能只在交接里改口径。

**ASC-IA-R1-F3 — P1：性能证据未绑定最终候选，且缺少测量契约与任务 9 明文要求的记录项。状态：OPEN。**

位置：`tasks.md` 任务 9 的 2026-11-11 三条交接；CEv1 工作单元 `auxiliary-session-classification:integrated-acceptance`。

- 行为风险：`performance-matrix` 准则要求在实际内容上按测量契约给出 scan、无变化、增量、恢复与前台延迟的结果。
  现有数字分属三个不同内容状态，无法作为同一候选的证据，也无法由评审绑定进 CEv1。
- 证据：
  - 只有 P09 在 14 路径候选上重测（20 对，比值 1.030）。P08、P02、P04、P01-repeat 的结果来自 9 路径候选之前、
    未含界面修复与触发器移除的内容；P03 与 P01-first 的「复测」在界面修复之后、触发器移除之前，且未说明是否为正式批次；
    P10 明确写「未重测」。`performance-evaluation.md:273-274` 规定产品改动后重测须绑定新的 content state。
  - 该契约 `performance-evaluation.md:279-281` 要求 CEv1 证据记录工具源码摘要、App/helper/framework 二进制 SHA-256、构建配置、
    语料身份与冻结清单聚合、锚定日与平移天数、工作副本与数据库模板身份、机器环境、样本数、统计量、失败与无效计数；
    任务 9 自身要求「记录采用的文档 blob、候选 SHA、工具链、输入身份和有效样本」。
    交接只给出截断的语料标识 `1d9ee56f…`、锚定日和多数条件的中位数与比值；未记录 scan-performance 三份文档的 blob、
    各条件样本数（仅 P09 写了 20 对）、p95/p99、二进制与工具摘要。CEv1 查询显示 9 项必需准则证据数均为 0。
  - 工具 pilot 曾大量无效；锁修复改变了两臂构建后，是否按契约重跑 pilot 并得到无效 0，交接没有记录。
- 说明：比值本身不判通过或失败（`performance-evaluation.md:245`），P09 余下约 3%、P10 约 4% 作为已知成本如实报告即可，不是本发现的内容。

💡 有界修复：在 F1、F2 确定最终内容之后，按 2026-11-11 用户收窄的五类条件，先跑 pilot（工具无效须为 0），
再对同一最终内容各跑一次正式批次；把契约要求的身份、样本数、统计量与失败/无效计数写入 CEv1，评审记录引用证据 ID 与关键数值。
原始运行目录若已删除，旧数字只能作为历史观察，不能补录为证据。

### 🟡 建议改进 — 推荐

**ASC-IA-R1-F4 — P2：claude-mem 来源不可读时，CLI 读取结果没有按合同给出 warning/partial；联合验收用例未断言 CLI envelope。状态：OPEN。**

位置：`cmd/agentdeck/session_origin.go:97-110`（`sessionSelectionMeta.fill`）、
`cmd/agentdeck/session_origin_acceptance_test.go:568-617`。

- 行为风险：`architecture.md:235` 与 `ux/cli.md:204` 规定「来源不可用是读取结果的 warning/partial，不改成伪成功分类」。
  自动化只看 envelope 时会把覆盖不完整的列表当作完整结果，与 AC7「不错误标记完成」及
  `recovery-joint-failure` 准则「never report an incomplete classification as complete」不符。
- 证据：`fill` 无条件置 `availability=available`，`cmd/agentdeck` 非测试代码没有任何位置消费 `sessionorigin.CoverageGapReasons`
  或把来源不可用转成 warnings/partial；开发交接的观察与此一致（`session list` envelope 为 available、partial=false、无 warning，
  仅逐项 reason_code 为 `origin_source_unavailable`）。验收用例在该场景只断言 `desktop session-scope` 的 `origin_coverage`
  与 `origin-source status`，没有断言 CLI 读取的 envelope。本条依据源代码审查，评审未另行执行复现。
- 归属：缺陷代码属任务 4 `cli-contract` 的交付，但由联合验收发现且直接影响本任务的 AC7 结论；
  用户已决定验收中发现的缺陷在本工作树内随任务 9 修复，故按本候选的未关闭发现处理。

💡 有界修复：带 session scope 的 CLI 读取在选定集合存在覆盖缺口原因时，于 envelope 给出带原因的 warning 并置 partial，
文本输出按 ux/cli.md 的 partial 状态标注；验收用例在来源不可读与恢复两步断言该 envelope。若用户认为逐项原因码已足够，
须改写上述两处合同文本后由文档评审确认。

**ASC-IA-R1-F5 — P2：显式推进 revision 的回归只覆盖部分写入点，交接中「去掉其中一处推进即失败」不成立。状态：OPEN。**

位置：`internal/session/search_text_revision_test.go:76-103`；未覆盖的写入点
`internal/session/session.go:904`、`1574`、`1772`、`1777`、`1787`、`1829`。

- 行为风险：F1 之后保证依赖每个写入点都带前缀，回归保护因此是该保证的唯一防线。漏掉的写入点被改坏时没有测试会失败。
- 证据：用例先删掉其余 revision 触发器，再验证插入、替换、移动来源、删除来源和 `Exclude("session")`。
  `replaceExec`（1574）在有文档时随后调用 `insertDocuments`，后者自带推进，所以单独去掉 1574 的前缀「replace」步仍然通过；
  `applySourceUpdate` 的排除分支（904）、`Exclude` 的 project/path/client 三个分支（1772、1777、1787）与
  `RebuildWithOptions`（1829）没有任何步骤触及。本条为静态推导，未做变异执行。

💡 有界修复：取决于 F1 的决定。若保留显式推进，为每个写入点补一步（replace 用空文档集验证删除侧），
或增加一条结构性检查：session 包内凡写 `session_documents` 的语句必须带该推进。若 F1 改回结构性机制，本条随之关闭并在复评中说明。

### 🟢 优点

- SQLite 锁丢失的根因分析与修复准确：`preparePrivateSQLiteFiles` 不再开关已存在的文件，
  `sqlite_lock_unix_test.go` 用子进程 F_GETLK 探测 `-shm` 的 dead-man-switch 字节，对 core、existing core 与 sessions 三条打开路径都成立。
  `prepareExistingPrivateSQLiteFile` 改用 lstat 与 `OpenExisting` 既有的常规私有文件检查一致，评审未将其视为缺陷。
- 五个联合验收用例走真实扫描与真实来源格式，对照手写 oracle，覆盖默认/覆盖/清除/重扫/停用/恢复链与事件、token、费用逐字节不变。
- `sessionScopePanelIsReady` 与完整面板模型的判定在六种状态下有一致性断言；批次视图复用的键与菜单栏的 publication 键同式，
  选择相等性包含 scope，不会把默认范围的批次视图用于临时范围。
- 开发交接如实记录了自己推翻的结论（触发器开销实验有误）、未重测项和合同变化，便于评审定位。

### 📝 总结

评审内容为 HEAD `a2c10771` 加上表 14 个路径、指纹 `5ce1f4d6…`。候选的功能联合验收与锁修复质量较好，
但有三项必须先解决：实现与已批准架构 §6 不一致（F1）、P03 行为要求未满足且记录口径有误（F2）、
性能证据未绑定最终内容且缺必需记录项（F3）；另有两项较小但同样须关闭的发现（F4、F5）。Round 1 FAIL。
产品代码、测试与配置在本轮保持只读。

Finding disposition：ASC-IA-R1-F1 OPEN；ASC-IA-R1-F2 OPEN；ASC-IA-R1-F3 OPEN；ASC-IA-R1-F4 OPEN；ASC-IA-R1-F5 OPEN。

Evidence（工具链 go1.27.3，darwin，全部数据库为临时隔离 fixture，未使用真实状态）：

- 候选指纹：按上表算法对工作树内容复算，得到 `5ce1f4d6…`，与交接一致。
- `scripts/run-go-test.sh ./...`：通过（退出 0）。
- `scripts/run-go-test.sh -race -timeout 40m ./internal/store/ ./internal/session/ ./internal/sessionorigin/...`：通过（退出 0）。
- `go vet ./...`：退出 0。对候选的 8 个 Go 文件执行 `gofmt -l` 无输出；`git diff --check` 退出 0。
  全仓 `gofmt -l cmd internal` 另列出 `cmd/agentdeck/usage_stats_viewer_test.go` 与 `internal/desktop/serving_cache.go`，
  两者与 HEAD 相同、不属本候选。
- `scripts/run-go-test.sh -race -timeout 40m -run '^TestIntegratedAcceptance' ./cmd/agentdeck/`：通过（退出 0）。
- CEv1：查询工作单元得到 9 项必需准则；写入本轮目标内容状态与 `independent-review` 的 fail 证据后复查门禁。
- 限制：未重跑性能矩阵（需冻结语料与数小时测量窗口）和 `make test-macos-app`；在得到决定性的合同与证据问题后未再扩大验证。
  F2、F3 中的测量数字取自开发交接，评审核对的是它们与测量契约、工具源码之间的一致性，没有独立复测。
  以上通过项只说明回归未被破坏，不构成任何准则的通过证据。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:integrated-acceptance:review-r1-5ce1f4d6b1af0511`。
完成门禁：FAILED。
失败证据对应 `independent-review`；其余八项尚无目标态通过证据。
该结论不关闭 Task 或 Topic，不授权提交、推送或修复。

下一步指令：`修复：auxiliary-session-classification / integrated-acceptance / ASC-IA-R1-F1 ASC-IA-R1-F2 ASC-IA-R1-F3 ASC-IA-R1-F4 ASC-IA-R1-F5`

## Round 2 — 2026-11-12

## 📋 Task 9 integrated-acceptance 独立复评

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewer: Claude Code（Beads actor `claude-code`）。本会话为冷上下文，未参与任务 9 的实现或修复；
  修复交接同样以 `claude-code` 记录，属于同一客户端的另一会话，独立性仅来自冷上下文与评审角色。
- Method: 候选指纹复算；Round 1 五项发现逐条对照当前内容；修复新增改动的逐文件 diff 审查
  （Go 批次/bootstrap、Swift 交接与收集器、CLI envelope、回归测试、架构文档修订）；
  对照已批准的 architecture、ux/cli、tasks.md 任务 3/7 合同与 scan-performance 测量契约；
  用 `go test -overlay` 做不改动工作树的变异执行；读取上一会话保留的正式批次原始输出复算 P03 背景打点
  并核对 CEv1 证据所记身份与数值；直接运行定向 Go 回归。未委派，未修改产品代码、测试或配置。
- Scope: 任务 9 候选的 26 个路径（下表）与 CEv1 中绑定该内容的两份证据。未重跑性能矩阵与 `make test-macos-app`
  （见 Evidence 的限制）。
- Reviewed state: HEAD `a2c10771f05c5f681f3a091c617f9dd97e821621`，候选指纹
  `b414adfdeae192a56567e775e5767ec666c4feb67880520400570de405178bc9`，与修复交接一致。
  算法同 Round 1：有序（LC_ALL=C）`path + 空格 + blob + 换行` 字节串的 SHA-256；不含 tasks.md 与全部评审记录
  （`reviews/scope-projections.md` 属评审记录，按此排除后才得到该指纹）。

| Path | Git blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarViewModel.swift | b294455c17d16b3f1daebe1ca1758f9b200dc691 |
| apps/macos/AgentDeckApp/SessionScopeController.swift | 1b4353386bc54f7cd7f96b223c62f3a22a022411 |
| apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | 16ce7b047cb1b2ce24511c89e998fec9f4f6e04f |
| apps/macos/AgentDeckShared/DesktopWire.swift | 05d84eb750fc041f87589276058dc9e11eb6bd89 |
| apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift | 49f00b6070503a5e2338aae65a703f04a0989efb |
| apps/macos/AgentDeckShared/WidgetSessionScopeBatch.swift | 9a99dca5931a035a1bd87dd427a14233284b5894 |
| apps/macos/AgentDeckTests/AppGroupSnapshotStoreTests.swift | ce6c953e4604b890f0ef5a1b967d67a43221b3f9 |
| apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift | 334fcfa6ce9aeb4ae1dfcecc20c6589bc661f94d |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | f1ca5c14d18c86c612f2aa0cb59ee4c5164da0d8 |
| cmd/agentdeck/desktop.go | d75df82bb079b295c93bc7702bee72a18ce21d97 |
| cmd/agentdeck/desktop_test.go | e381c642de2d97bcb5d7e7adf740d12bc7f9825a |
| cmd/agentdeck/main.go | 7db31fe75f31baef4aaf74ca09de1e3504f3e68e |
| cmd/agentdeck/session_origin.go | dd8e12a2edf2e7a1b1ae01e7f8fcab2fb1d54591 |
| cmd/agentdeck/session_origin_acceptance_test.go | 38d60f00e0b32e7a3b1e25ac948365d64ead8123 |
| cmd/agentdeck/source_coverage_test.go | ebd102b254df707c05e1a4c63dd54a7b6ae2ff1e |
| docs/topics/auxiliary-session-classification/architecture.md | df7eb6fba77025ba973e983b355f9213701fbdfd |
| internal/desktop/session_scope.go | 942547057793479168a528e3771ce2213acb46bc |
| internal/desktop/widget_summary_test.go | 4a9e9efd83f7902f8a564324b4a013ab637dd51b |
| internal/session/ingestion_sql_test.go | 4266ba2696f06ebcfbd30d7d4cfdb8f41da127f8 |
| internal/session/scope_snapshot.go | 6f37810ebbfc11efe542d43009d07980a895398d |
| internal/session/search_text_revision_test.go | 46a9509171d967c945541c8ea879983e8b5a2b19 |
| internal/session/session.go | 1b43ab611171d079ec834ab650b715cdf3a54ef5 |
| internal/sessionorigin/selection_test.go | 1dbdebe9d7bc38e979676de069bcf08e20a12348 |
| internal/store/serving_generation.go | 2dddc408df265ffe8a5dfbd5090372c420f66b9d |
| internal/store/sqlite_lock_unix_test.go | 43b33ef9dd5cb657601d1581153cf3d315be2d92 |
| internal/store/store.go | ec078fa9a30bfc018c3ccc7a61e9a6d9f70cd091 |

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

本节两项为 P2，但按本项目规则同样须在 PASS 前关闭。

**ASC-IA-R2-F1 — P2：交互式会话浏览器在分类覆盖不完整时没有任何提示，与同一选择的文本/JSON 输出不一致。处置：新增，OPEN。**

位置：`cmd/agentdeck/session_browser.go:29`（`newSessionBrowserPage` 只取 `UnknownCount`，丢弃 `snapshot.CoverageGaps`）、
`:449`（状态行）、`:478`（`sessionBrowserSummary`）；读取点 `cmd/agentdeck/main.go:2775`。

- 行为风险：claude-mem 来源不可读时，`session list` 文本现在在 Scope 行下输出
  `PARTIAL: classification coverage is incomplete (...)`，JSON 置 `partial=true` 并带原因；
  `session list --interactive` 对同一选择只显示 `N SESSIONS · Scope: work · M unknown`，
  用户无从知道这些 unknown 是因为来源读不到而不是确实未分类。三种表面给出的事实不同。
- 合同依据：`ux/cli.md` 状态表「partial｜保留已知记录，注明覆盖不完整」对全部 CLI 表面适用，
  第 102 行把 partial 放在 Scope 之后的固定位置，第 168 行规定浏览器前 4 行含状态行，
  第 194 行要求「browser/文本/JSON 必须共享事实，不能各自分类」。
- 证据：源码审查——浏览器页模型没有承载覆盖缺口的字段，渲染路径没有任何分支输出它；
  修复交接在「未改与限制」中自述「交互式会话浏览器没有覆盖缺口提示」。ASC-IA-R1-F4 的修复使文本/JSON 开始标注 partial，
  浏览器被留在旧行为，这个不一致是本候选造成的。评审未在 PTY 中另行复现。
- 归属：与 F4 相同——缺陷代码属任务 4 的交付，按用户「验收中发现的缺陷在本工作树内随任务 9 修复」的决定归本候选。

💡 有界修复：浏览器页模型带上所选集合的覆盖缺口原因，在既有状态位置按 ux/cli.md 的 partial 状态标注覆盖不完整
（窄宽度按该文档的摘要规则），并补一条帧断言；不改分类或计数。若用户认为浏览器不需要该提示，
须改写 ux/cli.md 上述文本并由文档评审确认，不能只在交接里列为限制。

**ASC-IA-R2-F2 — P2：两项用户已决定的合同变化没有同步进 tasks.md 的任务合同正文，已评审的任务 3、任务 7 条目与实现相反。处置：新增，OPEN。**

位置：`docs/topics/auxiliary-session-classification/tasks.md:194-195`、`:203`（任务 3）与 `:272-284`（任务 7）；
对应实现 `cmd/agentdeck/desktop.go:227`、`:297`，`apps/macos/AgentDeckShared/WidgetSessionScopeBatch.swift`。

- 行为风险：tasks.md 是已评审文档，任务条目是后续维护者与评审判断「实现是否符合计划」的依据。
  现在读任务 7 会得到「宿主按组合逐个请求、不新增多组合 wire」的生产路径，读任务 3 会得到「FTS 输入也在 trigger 依赖清单与直接 SQL 时钟内」，
  两者都与当前实现相反；真正的现行约定只存在于文件末尾的交接流水里。
- 证据：
  - 任务 7 第 272-277 行：「按……组合发起显式 Selection 只读请求……此处选定“宿主按组合请求”路径，不新增多组合 wire 字段」。
    候选新增 `desktop session-scope --batch`（envelope `desktop.session-scope-batch`，一次返回九份视图）
    和 bootstrap 流的 `session_scope_batch` 行，生产路径已是「bootstrap 进程随 publication 交付整批，缺失时一次 `--batch` 读取」；
    逐组合请求只剩给不支持整批的读取器。组合一致性规则（首个视图固定 generation、同一 as-of、整批丢弃与重试）仍成立，但载体变了。
  - 任务 3 第 194-195、203 行仍把「FTS 输入」列入 trigger 依赖清单与「直接 SQL 依赖时钟」验证；
    architecture.md §4/§6 已按用户 2026-11-11 的决定把 FTS 搜索文本改为产品写入路径的显式推进。
  - 修复交接自述「任务 3 条目中『含直接 SQL I/U/D』的文字未改」「任务 7 已评审的 helper 调用形态因此再次改变，须随本候选复评」。
    同一文件第 326-328 行对任务 9 测量范围的收窄已用带日期的修订说明就地同步，任务 3、7 没有照此处理。
- 说明：这与 Round 1 的 F1 是同一类问题（实现改了、已批准文本没改），只是这次决定已经有了，缺的是文本。

💡 有界修复：在任务 3 与任务 7 条目内各加一条带日期的修订说明（体例同第 326-328 行），写明用户决定、现行生产路径
（任务 7：bootstrap 交付整批、`--batch` 回退、逐组合读取仅作兼容；任务 3：FTS 搜索文本例外并指向 architecture.md §6），
保留原文不改写历史。tasks.md 因此需要恢复其 Review 确认；architecture.md 的 Review 目前同样是未勾选，见总结中的门禁说明。

### 🟢 优点

Round 1 发现的处置（逐条对照当前内容复核）：

- **ASC-IA-R1-F1 closed**：按用户 2026-11-11 的决定收窄合同。architecture.md §1 修订说明、§4 sessions 行与 §6 新增段落
  写明 FTS 搜索文本是 trigger 保证的唯一例外、理由（内容影子表上的 trigger 使 FTS5 逐次刷盘合并，P09 1.115→1.030）、
  保证范围与不受支持的写法；`serving_generation.go` 在可写打开时删除旧库的三个触发器；`scope_snapshot.go` 注释已同步；
  `reviews/scope-projections.md` 追加了不改写历史的保证范围变更说明。文本与代码逐项一致。
  该文档自身的 Review 单元格仍待恢复（见总结），其合同文本在任务 3 条目的副本归入 ASC-IA-R2-F2。
- **ASC-IA-R1-F2 closed**（修复加用户 2026-11-12 的明确决定）：切换客户端/时段复用批次视图，不再启动读取；
  批次由 bootstrap 进程在 `terminal` 行之后随 publication 交付，不另起 helper。
  评审从保留的正式批次原始打点独立复算：两臂各 4 个运行单元、每单元 75 次切换，窗口内 `helper.start`/`job.start`/`otel.*` 均为 0，
  两臂窗口前的 helper 启动数相同（各 5 次）。
  须如实说明的一点：打点为零是因为该计数只数进程启动。候选的 bootstrap helper 在四个运行单元里都延续到切换窗口开始后
  约 1.3–2.0 秒才退出，期间用掉 1.3–1.6 秒用户态 CPU（基线同一进程在首个运行单元约 0.03 秒；该单元里基线的扫描 collector 也有约 1 秒落在窗口内）。
  也就是说批次的计算仍与切换重叠，只是不再表现为新进程。最先执行、与这段重叠的 client 维度中位、p95、p99
  均不差于基线（66.29→66.19、83.2→81.3、99.7→99.5 ms），切换本身不等待任何 helper，行为要求的字面与测量契约的判定都满足；
  这一形态是用户选定的。
- **ASC-IA-R1-F3 closed**：最终内容上先 pilot（工具无效 0）再各跑一次正式批次，证据
  `auxiliary-session-classification:integrated-acceptance:repair-r1-b414adfd:performance-matrix:pass` 绑定本内容，
  记录了三份契约文档 blob、工具摘要与源码树、两臂 App/helper/framework SHA-256、构建配置、语料与工作副本身份、锚定日与平移天数、
  模板身份、机器环境、各条件样本数、中位/p95/p99 及区间、工具无效/产品失败/打断重跑计数。
  评审核对：契约 blob（`812d3f4c`、`cd5f3cdd`、`913a03fa`）与工具源码树（`e8bd8fa9`、`da9ce187`）等于 HEAD 且工作树未改；
  候选 App、helper、AgentDeckShared 与测量工具的 SHA-256 与证据所记一致，framework 构建于最后一次产品源码修改之后，
  构建时记录的 26 个 blob 与当前相同；P02、P04、P01、P03、P09、P10 的比较输出与 P08 计数和证据逐项相符。
  已知成本按实测记录、不以比值判定：P09 约 +3%，P10 约 +3%，P03 panel 的 p95/p99 晚一帧，P01-first 的 p95 为 111.8→135.4 ms（区间跨 1）。
- **ASC-IA-R1-F4 closed**：`sessionSelectionMeta.fill` 收集所选 client/时间窗的覆盖缺口原因，
  `writeResult` 与 `writeUsageEnvelope` 据此置 `partial=true` 并把原因码并入 `warnings`，文本在 Scope 行下输出 PARTIAL 行与检查命令，
  `availability` 保持 available。验收用例在来源不可读与恢复两步断言三种范围、claude/codex/无 client、search 与文本输出。
  浏览器表面的同类缺口另立为 ASC-IA-R2-F1。
- **ASC-IA-R1-F5 closed**：行为测试为 9 条带前缀的语句与 `insertDocuments` 各有一步能单独暴露它的验证，另有结构性检查。
  评审用 `-overlay` 对 10 个推进点逐一去掉推进后只跑行为测试，10 次全部失败，且各自失败在对应步骤
  （904 追加到已排除会话、942 移动来源、948 删除来源、1574 空文档替换、1772/1777/1782/1787 四种排除、1829 rebuild、`insertDocuments` 纯追加）；
  未变异时两条测试通过。Round 1 指出的六个漏点均已覆盖。

新增改动中做得好的地方：

- 交付的批次与读取的批次走同一套逐视图校验（selection、默认、generation、时间），不通过即回退到读取并保留完整重试预算；
  交付只是来源不同，不是校验上的捷径。未确认默认不产生批次。
- bootstrap 只在两域均 completed 且此前各行无解析失败时提前返回；未完成的轮次仍等退出码。批次行解析失败只算缺失交付，不把已完成的轮次判失败。
- 修复交接如实列出了自己未改的两处，本轮两项新发现都能由此直接定位。

### 📝 总结

逐项处置：ASC-IA-R1-F1 closed；ASC-IA-R1-F2 closed；ASC-IA-R1-F3 closed；ASC-IA-R1-F4 closed；ASC-IA-R1-F5 closed；
ASC-IA-R2-F1 OPEN（新增）；ASC-IA-R2-F2 OPEN（新增）。无回退。

评审内容为 HEAD `a2c10771` 加上表 26 个路径、指纹 `b414adfd…`。Round 1 的五项发现全部关闭，其中 F5 与 F2、F3 有评审独立取得的证据。
结论仍为 FAIL，原因是修复留下两处自述未处理的缺口：浏览器表面没有跟上 F4 的 partial 标注（R2-F1），
任务 3、7 的合同正文没有跟上用户已做的两项决定（R2-F2）。两项都小且边界清楚，不涉及性能矩阵，修复后无需重测性能，
但产品代码变化会产生新的内容状态，L3 与性能证据需按 Evidence 的复用规则做影响评估后才能用于新状态。

残余不确定性：
- F2 的重叠事实见上。若用户希望批次计算完全移出切换窗口，那是新的设计决定，不在本轮发现之内。
- bootstrap 调用提前返回后，helper 的尾段不再受调用方取消控制（交接已说明）；评审按只读批次、进程自行退出判断风险低，未做故障注入。
- 被暂缓的 publication 在释放时使用交付批次，其 as-of 是 bootstrap 结束时刻而非释放时刻；摘要自带该时间，评审未将其列为缺陷。
- `make test-macos-app` 与性能矩阵未由评审重跑，采用绑定同一内容的 CEv1 证据。

Evidence（工具链 go1.27.3，darwin；全部数据库为临时隔离 fixture，未使用真实状态）：

- 候选指纹：对工作树内容复算得 `b414adfd…`，评审结束时再次复算未变。
- 变异执行：`scripts/run-go-test.sh -overlay <变异副本> -run '^TestEveryProductWriteOfSearchTextAdvancesTheServingRevision$' ./internal/session/`
  共 10 次，均以对应步骤的断言失败；变异副本在工作树之外，工作树文件未改。
  不带 overlay 运行 `-run '^TestEvery(ProductWriteOfSearchText|SearchTextStatement)'` 通过。
- P03 背景打点复算：读取 2026-11-12 正式批次两臂共 8 个运行单元的 app/helper/collector 打点，
  按 `cmd/agentdeck-perf/stats.go` 的 `backgroundMarkers` 定义（首个 `select.intent` 至末个 `frame.vsync` 之间的
  `helper.start`、`job.start`、`otel.*`）计数，全部为 0；helper 进程寿命取各自的 `resource.lifetime` 记录。
  这些原始输出位于上一会话的临时目录，不作为可回溯引用，以上数字即本记录保留的全部事实。
- `scripts/run-go-test.sh -race -timeout 40m -run '^(TestIntegratedAcceptance|TestDesktopSessionScope|TestDesktopBootstrap)' ./cmd/agentdeck/`：通过（退出 0）。
- `scripts/run-go-test.sh -race -timeout 40m ./internal/session/ ./internal/desktop/ ./internal/store/ ./internal/sessionorigin/...`：通过（退出 0）。
- `go vet ./cmd/agentdeck/ ./internal/session/ ./internal/desktop/ ./internal/store/`：退出 0；`git diff --check HEAD`：退出 0。
- 全量 Go、全仓 vet 与 `make test-macos-app`（458 条，0 失败）采用 CEv1 证据
  `auxiliary-session-classification:integrated-acceptance:repair-r1-b414adfd:l3-verification:pass`，其内容状态与本轮相同。
- CEv1：写入本轮 `independent-review` 的 fail 证据
  `auxiliary-session-classification:integrated-acceptance:r2-b414adfd:independent-review:fail` 后复查门禁。
- 限制：未重跑性能矩阵与 `make test-macos-app`；R2-F1 依据源码审查与交接自述，未在 PTY 中复现。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:integrated-acceptance:repair-r1-b414adfdeae192a5`。
完成门禁：FAILED。
9 项必需准则中，`performance-matrix` 与 `l3-verification` 在该内容上有通过证据；`independent-review` 为本轮 fail；
其余六项（cross-surface-cohort、control-chain-usage-identity、incremental-full-equivalence、maintenance-adapters-end-to-end、
integration-no-rescan-write-bounds、recovery-joint-failure）尚无目标态证据。
文档门禁说明：architecture.md 在 Documents 矩阵中 Review 未勾选，其评审记录尚无覆盖当前 blob `df7eb6fb` 的 PASS 轮次；
本轮对其修订内容的核对结论记在 ASC-IA-R1-F1，不代替该文档自身的评审轮次。
该结论不关闭 Task 或 Topic，不授权提交、推送或修复。

下一步指令：`修复：auxiliary-session-classification / integrated-acceptance / ASC-IA-R2-F1 ASC-IA-R2-F2`

## Round 3 — 2026-11-12

## 📋 Task 9 integrated-acceptance 独立复评

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewer: Claude Code（Beads actor `claude-code`）。与 Round 2 为同一评审会话，未参与任务 9 的实现或任何一轮修复；
  修复交接同样以 `claude-code` 记录，属于同一客户端的另一会话，独立性仅来自评审角色与未参与修复。
- Method: 候选指纹复算并与 Round 2 表逐行比对变化；ASC-IA-R2-F1、ASC-IA-R2-F2 对照当前内容复核；
  浏览器范围行改动的 diff 审查与对照 ux/cli.md；用 `go test -overlay` 加一个工作树之外的探针测试，
  直接调用 `sessionBrowserSummary` 观察各宽度与计数下的实际输出；直接运行相关 Go 回归。未委派，未修改产品代码、测试或配置。
- Scope: 相对 Round 2 变化的四个路径（下表）与 tasks.md 任务 3、任务 7 的修订说明。其余 24 个路径的 blob 与 Round 2 表相同，沿用该轮结论。
- Reviewed state: HEAD `a2c10771f05c5f681f3a091c617f9dd97e821621`，候选 28 个路径，指纹
  `a2bb899e5d02b62c4526898fde960e6fe3855fb71a5cd78c85b893c64544d7c9`，与修复交接一致；算法同前两轮。

| Path | Git blob | 相对 Round 2 |
| --- | --- | --- |
| cmd/agentdeck/session_browser.go | eb5c7de44d4f86bd3b9488c81279533cb12c6666 | 新增入候选 |
| cmd/agentdeck/session_origin.go | 699c5587da6388b39e7356d7738d502aad09a5a3 | 变化 |
| cmd/agentdeck/session_origin_acceptance_test.go | 8765ea02f4bff60fd65f3815ed622b0e914563cb | 变化 |
| cmd/agentdeck/session_origin_test.go | 5730b37a7e3ef1810d415d0828a0ee1b746dbce1 | 新增入候选 |

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

本节一项为 P2，按本项目规则同样须在 PASS 前关闭。

**ASC-IA-R3-F1 — P2：浏览器的 partial 标注在常见宽度下只剩一个词，在最小宽度下会被截断，原因与检查入口在浏览器里无处可见。处置：新增，OPEN。**

位置：`cmd/agentdeck/session_browser.go` 的 `sessionBrowserSummary`（三段式回退的最后一段 `statsFit(line+" · PARTIAL", width)`）；
回归 `cmd/agentdeck/session_origin_test.go` 的 `TestSessionBrowserScopeMenuStateMachine` 只用 2 条会话、1 个 unknown 的夹具。

- 行为风险：
  - 80 列是最常见的终端宽度。此时范围行为 `12 SESSIONS · Scope: work · 3 unknown · PARTIAL`：原因码与「classification coverage incomplete」
    都被省掉，浏览器里也没有别处给出原因或 `agentdeck session origin-source status`。
    同一个 PARTIAL 标签在 ux/cli.md 里也用于 scan_incomplete，用户分不出是哪一种不完整，也不知道去哪里查。
    完整语句在 work 范围、两位数会话时宽 111 列，要到这个宽度以上才出现。
  - 在浏览器支持的最小宽度附近，计数一大 PARTIAL 本身就被截掉：48 列下 `1500 SESSIONS · Scope: work · 300 unknown · PAR…`、
    `2232 SESSIONS · Scope: all · 1200 unknown · PAR…`；52 列下 `12000 SESSIONS · Scope: work · 4000 unknown · PARTI…`。
    千级会话、百级 unknown 在来源不可读时是正常规模（来源读不到时该来源的会话全部成为 unknown）。
    函数注释「so PARTIAL itself stays visible」与修复交接「PARTIAL 保留」在这个范围内不成立。
- 合同依据：`ux/cli.md` 状态表「partial｜保留已知记录，注明覆盖不完整」；第 159-160 行「保留标题/范围/警告最多 4 行……
  警告较长使用一行摘要并在只读详情显示完整信息」；第 194 行 browser/文本/JSON 共享事实。
  现实现把警告并进范围行而不是用警告行，较长时只留摘要词，完整信息没有任何去处。
- 证据：探针测试以 `usageTextPrimitives{}` 直接调用 `sessionBrowserSummary`，对 (会话数, unknown, 范围) =
  (12,3,work)、(1500,300,work)、(2232,1200,all)、(150,0,auxiliary)、(12000,4000,work) 在 48/52/60/80/120 列各取一次输出：
  120 列全部给出完整语句；48–80 列全部只有 `PARTIAL`；上述三处被截断。`statsFit` 超宽时从右侧截断并加 `…`，所以最右的 PARTIAL 最先丢。
  现有回归在 48 列断言的是 `2 SESSIONS · Scope: work · 1 unknown · PARTIAL`（46 列宽），刚好不触发截断。

💡 有界修复：让 partial 的陈述不依赖范围行的剩余宽度——按 ux/cli.md 第 159 行用警告行承载（该文档允许标题/范围/警告共 4 行），
或保持在范围行但让次要内容先让位、PARTIAL 与其含义最后让位；48 列时仍须完整显示状态词并能让用户得到原因
（缩短的原因语句、或 ux/cli.md 所说的只读详情）。补大计数夹具在 48 与 80 列的帧断言。分类、计数与文本/JSON 输出不变。
若用户认为 80 列下一个 `PARTIAL` 词已经足够，可由用户明确决定关闭该部分，但最小宽度下的截断仍须修。

### 🟢 优点

Round 2 发现的处置（对照当前内容复核）：

- **ASC-IA-R2-F1 closed**：浏览器页模型与状态带上 `coverageGaps`，与文本/JSON 同取自 `selectionCoverageGaps`（`sessionSelectionMeta.fill` 改为调用它），
  三种表面的事实来源已是同一个函数；范围行在三种范围、空集合与范围菜单帧都标注。Round 2 指出的缺陷是「没有任何提示且与文本/JSON 不一致」，这一点已消除。
  验收用例按 `--interactive` 的读取路径对三种范围在不可读与恢复两步断言。标注在窄宽度下的表现另立为 ASC-IA-R3-F1。
- **ASC-IA-R2-F2 closed**：任务 3 条目（tasks.md 第 205-209 行）与任务 7 条目（第 290-296 行）各有带日期的合同修订说明，
  内容与 architecture.md §4/§6 及当前实现一致（FTS 搜索文本例外；bootstrap 交付整批、`--batch` 回退、逐组合读取仅作兼容，组合一致性规则不变），原文保留。

修复交接没有把旧内容状态的证据直接挪用到新内容，而是明确留给复评判定，做法正确。

### 📝 总结

逐项处置：ASC-IA-R2-F1 closed；ASC-IA-R2-F2 closed；ASC-IA-R3-F1 OPEN（新增）。Round 1 的五项在 Round 2 已关闭，本轮涉及的路径没有使其回退
（`session_origin.go` 的变化只是把覆盖缺口的收集提成共享函数，F4 的文本与 JSON 断言仍通过）。

结论为 FAIL，只因一项：浏览器里新加的 partial 标注在 80 列下信息不足、在最小宽度下会被截断。修复范围限于 `sessionBrowserSummary` 及其帧断言。

证据复用的预判（供修复后参考，不构成本轮的门禁结论）：Round 2 到本轮的产品代码变化只有 `cmd/agentdeck` 的浏览器渲染与一个提取出的共享函数，
Swift、helper 协议、扫描、发布与存储路径的 blob 未变。性能矩阵各条件都不经过交互式浏览器，
`repair-r1-b414adfd` 的性能证据在做出影响评估记录后应可复用；L3 中的全量 Go 需要在新内容上重跑，原生测试的 Swift 与 helper 输入未变。

残余不确定性：
- 浏览器输出取自直接调用渲染函数，未在真实 PTY 中观察；颜色与截断后的样式未评。
- Round 2 记下的 F2 重叠事实、bootstrap 尾段不可取消、被暂缓 publication 的 as-of 三点没有变化。
- architecture.md 与 tasks.md 两份文档自身的 Review 确认仍未恢复，见下方门禁说明。

Evidence（工具链 go1.27.3，darwin；全部数据库为临时隔离 fixture，未使用真实状态）：

- 候选指纹：对工作树内容复算得 `a2bb899e…`；与 Round 2 的 26 行比对，仅上表四个路径不同或新增。
- 探针：`scripts/run-go-test.sh -overlay <仅含一个新增测试文件的 overlay> -run '^TestZZProbeBrowserSummaryWidths$' ./cmd/agentdeck/`，
  探针文件在工作树之外，输出见 ASC-IA-R3-F1 的证据。
- `scripts/run-go-test.sh -race -timeout 20m -run '^(TestSessionBrowser|TestIntegratedAcceptanceUnreadableSourceDegradesAndRecovers)' ./cmd/agentdeck/`：通过（退出 0，13 条）。
- `go vet ./cmd/agentdeck/`：退出 0；四个变化的 Go 文件 `gofmt -l` 无输出；`git diff --check HEAD`：退出 0。
- CEv1：登记本轮内容状态并写入 `independent-review` 的 fail 证据后复查门禁。
- 限制：未重跑全量 Go、`make test-macos-app` 与性能矩阵；本内容状态上除本轮 fail 外没有其他证据。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:integrated-acceptance:repair-r2-a2bb899e5d02b62c`。
完成门禁：FAILED。
9 项必需准则在该内容上只有 `independent-review` 的 fail；其余八项尚无目标态证据（Round 2 内容状态上的两份通过证据不自动适用于本内容）。
文档门禁说明：architecture.md（blob `df7eb6fb`）与 tasks.md 在各自的评审记录里都还没有覆盖当前内容的 PASS 轮次；
tasks.md 的任务 3、7 修订说明已在本轮核对属实，但这不代替该文档自身的评审轮次。
该结论不关闭 Task 或 Topic，不授权提交、推送或修复。

下一步指令：`修复：auxiliary-session-classification / integrated-acceptance / ASC-IA-R3-F1`

## Round 4 — 2026-11-13

## 📋 Task 9 integrated-acceptance 独立复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Claude Code（Beads actor `claude-code`）。与 Round 2、Round 3 为同一评审会话，未参与任务 9 的实现或任何一轮修复；
  修复交接同样以 `claude-code` 记录，属于同一客户端的另一会话，独立性仅来自评审角色与未参与修复。
- Method: 候选指纹复算并与 Round 3 逐行比对；ASC-IA-R3-F1 对照当前内容复核，用 `go test -overlay` 加工作树之外的探针测试
  渲染各状态、各尺寸的完整浏览器帧；复读五个联合验收用例以确认它们各自支撑的证据准则；直接运行全量 Go、`cmd/agentdeck` 的 race 全量与 vet；
  对未重跑的原生测试与性能矩阵按 Evidence 的复用规则做影响评估。未委派，未修改产品代码、测试或配置。
- Scope: 相对 Round 3 变化的两个路径（下表）；其余 26 个路径的 blob 与 Round 3 相同，沿用前几轮结论。另含九项证据准则在本内容上的逐项判定。
- Reviewed state: HEAD `a2c10771f05c5f681f3a091c617f9dd97e821621`，候选 28 个路径，指纹
  `d4d02e77cbe4db40563b3106434cc292a41be6cd63ed308fa6b0d126bd64b473`，与修复交接一致，评审结束时复算未变；算法同前。

| Path | Git blob | 相对 Round 3 |
| --- | --- | --- |
| cmd/agentdeck/session_browser.go | f858057955d9e1f66b9672bf14cfb0b41112cca1 | 变化 |
| cmd/agentdeck/session_origin_test.go | 3b173068bacfafcf0757929bcaed5b267bf630ce | 变化 |

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

本候选范围内无。以下一项在本轮验证中观察到，但不属于本候选的改动，已转出，不阻塞本任务。

**ASC-IA-R4-F1 — P2（范围外，既有问题）：机器负载高时，带范围的会话读取与 `session classify --set` 会偶发以 generation_unavailable 失败。处置：-> open，载体 Beads `ad-bug-selection-verify-200ms-deadline`。**

位置：`internal/store/serving_generation.go` 的 `OpenServingGenerationReader`（固定 200 ms 的 context 期限，提交 `ab4f448`，scan-performance，已在 main）；
`internal/sessionorigin/selection.go` 的 `Reader.Select`（验证读向量时把该读取的任何错误都当作 `ErrGenerationUnavailable`，不重试；只有向量移动才重试）。

- 行为风险：状态健康、重试即成功的命令在高负载下退出 1，并提示用户运行 doctor。
- 证据：评审首次运行 `cmd/agentdeck` 的 race 全量时，`TestUsageSignalsAndStatsApplySessionScopeOnlyToWorkAnalysis` 在其 classify 步骤以该提示失败，其余 436 条通过。
  同一测试在 race 下单独重复 15 次全部通过；12 个 race 进程并发（12 逻辑核）时 96 次全部失败，
  工作树之外的插桩构建每次都打印底层错误 `serving generation unavailable: context deadline exceeded`。
- 归属：两处代码都不在本候选的 28 个路径的改动内（`serving_generation.go` 在候选中只改了触发器安装的几行），期限来自 scan-performance，验证逻辑来自任务 3 的已交付内容。
  按评审记录规则属于既有条件，转出而不阻塞。通道（Lane）未选，待用户分诊。

### 🟢 优点

- **ASC-IA-R3-F1 closed**：partial 改为范围行下方独立的警告行，范围行只剩计数与时区，计数再大也不挤占警告；
  有缺口时 viewport 的固定行数加一。评审渲染的帧（12000 会话、4000 unknown，另有 1500/300 与空集合）：
  - 48×10：`PARTIAL coverage: origin_source_unavailable`，状态词与原因码完整；
  - 80×10、80×24：`PARTIAL: classification coverage incomplete (origin_source_unavailable)`；
  - 120×24：上一句后接 `· Check: agentdeck session origin-source status`；
  - 两个原因时 80 列为 `PARTIAL coverage: origin_input_mismatch, origin_source_unavailable`，48 列从右截断第二个原因，PARTIAL 与第一个原因保留；
  - 范围菜单、范围加载失败、空 auxiliary、空集合加失败四种帧都带警告行，行数不超过终端高度，没有超宽行，行号与帮助行都在；
  - 无缺口时帧与修复前一致（少一行），47 列仍走 too small。
  Round 3 指出的两点——80 列只剩一个词、48 列被截断——都已消除。回归改用大计数夹具并逐字断言警告行、帧高与帮助行。
- 五个联合验收用例与九项证据准则的对应关系清楚，评审逐个复读后确认每项准则都有直接断言支撑（见 Evidence）。

### 📝 总结

逐项处置：ASC-IA-R3-F1 closed。本记录的全部发现现状：ASC-IA-R1-F1…F5 closed（Round 2），ASC-IA-R2-F1、ASC-IA-R2-F2 closed（Round 3），
ASC-IA-R3-F1 closed（本轮）。本候选范围内无未关闭、无回退的发现；范围外的 ASC-IA-R4-F1 已转出到 Beads `ad-bug-selection-verify-200ms-deadline`。

评审内容为 HEAD `a2c10771` 加 28 个路径、指纹 `d4d02e77…`。任务 9 的 Review 勾选。

残余不确定性（均已在前几轮说明，无变化，不构成发现）：
- Widget 批次的计算现在发生在 bootstrap helper 的尾段，会与紧随其后的切换重叠约 1.3–2.0 秒；测量契约的打点为零、切换延迟不差于基线，形态为用户选定。
- bootstrap 调用返回后 helper 尾段不受调用方取消控制；被暂缓 publication 的交付批次带的是 bootstrap 结束时刻的 as-of。
- 浏览器帧取自直接调用渲染函数，未在真实 PTY 中观察。
- 性能矩阵与原生测试未在本内容上重跑，采用下述影响评估后的复用。

Evidence（工具链 go1.27.3，darwin；全部数据库为临时隔离 fixture，未使用真实状态）：

- 候选指纹：复算得 `d4d02e77…`；与 Round 3 的 28 行比对，仅上表两个路径不同。
- 探针：`scripts/run-go-test.sh -overlay <仅含一个新增测试文件的 overlay> -run '^TestZZProbeBrowserFrames$' ./cmd/agentdeck/`，探针文件在工作树之外。
- `scripts/run-go-test.sh ./...`：通过（退出 0，29 个包）。
- `scripts/run-go-test.sh -race -timeout 40m ./cmd/agentdeck/`：第一次运行 1 条失败（见 ASC-IA-R4-F1，既有的 200 ms 期限在负载下超时，不属本候选），
  其余 436 条通过；定位根因后重跑通过（退出 0，438 条）。该失败测试在 race 下单独重复 15 次全部通过。
- `go vet ./...`：退出 0；两个变化的 Go 文件 `gofmt -l` 无输出；`git diff --check HEAD`：退出 0。
- 九项证据准则在本内容上的判定：
  - `cross-surface-cohort`、`maintenance-adapters-end-to-end`：`TestIntegratedAcceptanceEverySurfaceAgreesOnTheCohort`
    （真实格式的 Claude/Codex 日志、OTel 导出与 claude-mem fixture 经真实扫描；CLI、菜单栏视图、Widget 批次、Settings 读取对照手写 oracle；
    两条维护规则各自的 reason/rule_version；无价模型总费用保持未知）。
  - `control-chain-usage-identity`、`recovery-joint-failure`：`…ControlChainKeepsEveryEventAndCost`（默认/覆盖/清除/重扫/停用与启用/purge-index/备份恢复重绑，
    每步事件、token、费用与来源文件字节不变且无重新导入）与 `…UnreadableSourceDegradesAndRecovers`（来源不可读时各表面给出原因并标 partial，恢复后复原）。
  - `incremental-full-equivalence`：`…IncrementalEqualsFullRebuild`（追加与晚到 rollout 的增量结果等于同来源全量重建，晚到 rollout 撤销 OTel 标记且不重复计量）。
  - `integration-no-rescan-write-bounds`：`…ClassificationWritesAreBounded`（范围切换零写入；一次覆盖推进 revision 一次并只失效该会话分区；
    无变化扫描不重写分类行，启用来源每轮恰多一次）加上控制链用例的无重新导入断言；有界读取另见性能证据中的 P08 计数。
  - `l3-verification`：上列全量 Go、race 与 vet 为评审在本内容上直接取得。`make test-macos-app` 未重跑：
    相对已有通过证据的内容 `b414adfd`，本内容只变了 `cmd/agentdeck` 的四个文件（浏览器渲染、覆盖缺口收集函数的提取与其测试），
    九个 Swift 路径与 `desktop` 命令、`internal/desktop` 的 blob 均未变，原生测试不经过交互式浏览器或带范围的 CLI 读取；据此复用该证据的原生部分。
  - `performance-matrix`：复用 `b414adfd` 上的正式批次证据。产品代码的差异只有 `session_browser.go`（交互式 TUI 渲染）与
    `session_origin.go` 中 `selectionCoverageGaps` 的提取（调用者只有带范围的 CLI 读取与浏览器）；
    P01–P04、P08–P10 走的是 App、`desktop` 命令与扫描 worker，不调用这两处，扫描、发布、存储与 Swift 路径的 blob 未变。
    测量契约只在需要时要求重测；评审判断此差异不影响任何被测路径，不需重测。局限：被测 helper 二进制与本内容构建出的二进制字节不同。
  - `independent-review`：本轮 PASS。
- 限制：未重跑性能矩阵与 `make test-macos-app`；复用依据如上。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:integrated-acceptance:repair-r3-d4d02e77cbe4db40`。
完成门禁：VERIFIED。
九项必需准则在该内容上各有一份通过证据，写入并回读后按 WorkUnit 与目标内容状态复查得到。
证据 ID 为 `auxiliary-session-classification:integrated-acceptance:r4-d4d02e77:<准则>:pass`；其中 `l3-verification` 与 `performance-matrix`
以 rolls_up 关系指向 `…:repair-r1-b414adfd:` 下的原证据。
主题边界说明：任务 9 是本主题最后一个任务，但主题边界尚不能关闭——CEv1 中没有主题级 WorkUnit `auxiliary-session-classification`，
且 architecture.md（blob `df7eb6fb`）与 tasks.md 的任务 3、7 修订说明都还没有各自文档评审记录里的 PASS 轮次。
该结论不授权提交、推送或主题收尾。

### 内容状态更新 — 2026-11-13

本节只记录 Round 4 之后候选身份的变化，不改写上面各轮的结论与证据；它们仍绑定各自当时的内容。

architecture.md 的文档评审（[reviews/architecture.md](architecture.md) Round 6、Round 7）改正了 §6 中一句对触发器所占回退比例的表述。
该文件在本任务的 28 个候选路径内，blob 由 `df7eb6fb` 变为 `42986a3c5452e5400fca2635b560899604b1aa7e`，其余 27 个路径与 Round 4 表相同；
候选指纹因此由 `d4d02e77…` 变为 `024b6820cd8910ce4331c122291905a20f9f49a485cddb7b9ed9ed6b90604ec8`（算法不变）。
影响评估：只有一句文档措辞变化，产品代码、测试、配置与合同义务均未变，Round 4 的九项证据全部仍然适用，未重跑任何检查。
CEv1 已登记内容状态 `auxiliary-session-classification:integrated-acceptance:arch-r7-024b6820cd8910ce`，
九份证据 `…:r4-024b6820:<准则>:pass` 各以 rolls_up 指向 `…:r4-d4d02e77:` 下的原证据；写入并回读后复查，该 WorkUnit 在新内容状态上九项准则均有通过证据。
Round 4 的评审结论不变。


## Round 5 — 2026-11-13

## 📋 Task 9 稳定规格收尾独立评审

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Codex，本轮独立评审会话；未参与前一会话的稳定规格编写或产品实现，未委派。
- Method: 文档增量语义审查；工作区绑定、Beads 交接及精确 blob 核验；以本工作树 CodeGraph 定位后对照命令处理器、来源状态、FTS 写入与备份恢复源码；复核 CEv1 的目标绑定及证据沿袭。
- Scope: `docs/specs/cli-design.md`、`docs/specs/cli-manual.md` 的新增分类合同，以及 tasks.md 的稳定规格收尾交接。此前产品实现沿用 Round 4 及其 architecture Round 7 内容更新，不重做未变化的产品评审。
- Reviewed state: HEAD `f39f86aa1212de2c674667ee343f09c50dc11236`，tree `b1c9df4e54d832aa60fdc4547fe2d69a489935ae`；cli-design blob `7fe242acad4d4e0d428e400f83618bad075ad706`；cli-manual blob `907d221515ed8b1489d35f25ae3a1c0b528a405a`。
  ContentState: `auxiliary-session-classification:stable-specs:f39f86a-7fe242ac-907d2215`。沿用既有配方：HEAD 加上述两个稳定规格 blob；tasks.md 的状态/交接与本评审记录不进入产品候选身份。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无新增发现。此前范围外 ASC-IA-R4-F1 仍由 `ad-bug-selection-verify-200ms-deadline` 承载，本轮没有改变其归属或宣称修复。

### 🟢 优点

- 分类与消费统计边界清晰：unknown 留在 Work，范围只影响会话分析，不减去维护会话的 token/费用；覆盖与自动原因分离。
- CLI 文档与处理器一致：classify 的 set/clear/receipt/保留 ID、CAS、scope 默认值，以及 origin-source 的两项固定顺序和封闭原因枚举；clear 无需读取索引。
- `cmd/agentdeck/desktop.go:newDesktopSessionScopeCommand` 确认 JSON-only、显式 scope、client/period 默认值、batch 与显式 client/period 互斥、RFC3339 时间及 1–20 recent-limit。bootstrap 批次复用没有引入 Widget 的 CLI/SQLite 读取权限。
- `internal/session/session.go` 的 searchTextChanged 与写入推进路径、architecture §6 及 Round 4 的保证范围一致：FTS 内容不是任意外部 SQL 都受触发器保护。
- `internal/backup/backup.go` 的非便携表清理与恢复 unmatched 行为支持新增备份说明；分类控制 DTO 的隐私边界没有误覆盖既有 session 正文输出。
- CLI 的 coverage partial 与 source-status 的三种读取失败 partial 分开陈述；来源状态是已提交事实，不冒充插件实时健康检查。

### 📝 总结

本轮无未关闭的范围内发现，结论 PASS。此前 ASC-IA-R1-F1…F5、R2-F1/F2、R3-F1 的关闭状态不变；本轮文档没有撤销这些修复。
任务 9 Review 已勾选。产品代码、测试、依赖与配置均未修改，没有执行提交、推送、集成或发布。

Evidence:

- 实时 Git 状态仅有两份 specs 与 tasks.md 的文档变化；两份 specs 的 `git hash-object` 与上述 ContentState 完全相同。
- 手册参数与 `cmd/agentdeck/session_origin.go`、`cmd/agentdeck/desktop.go` 对照；来源状态 JSON 字段与 `originSourceStatus.MarshalJSON`、`originSourceStatuses` 对照；FTS 与备份边界按上述源码及已批准 architecture 核验。
- 入场 CEv1 固定模板查询：任务 NOT_VERIFIED 8/9，仅缺 independent-review；invalidated_evidence 与 unresolved_candidate_impacts 均为空。八项目标绑定 roll-up 的源码、测试与配置未变化，因此复用其既有证据及局限，不重跑 L3、原生测试或性能矩阵。
- 本轮 `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 均退出 0；结构检查不替代语义评审。最终门禁结果见下方字段。

残余不确定性沿用 Round 4：浏览器为渲染帧证据而非真实 PTY；原生与性能结论按已记录影响评估复用；P06/P07/P11 及 P01 rebuild 层不在已批准范围。没有新增真实用户状态、插件运行或 Widget 调度验收。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:stable-specs:f39f86a-7fe242ac-907d2215`。
完成门禁：VERIFIED。

本轮新增 2 个 evidence 节点、5 条关系，预检均为 ok，写入后回读数量与端点一致。固定 gate-status 模板按内到外查询：
任务 VERIFIED 9/9；Topic `auxiliary-session-classification` VERIFIED 2/2；均无 missing criteria、invalidated evidence 或 unresolved candidate impact。
新证据 ID 为本 ContentState 加 `:r5:independent-review:pass` 与 `:r5:stable-contract-reconciliation:pass`。
Topic 稳定合同证据 rolls_up 本轮独立评审；跨任务验收复用原有八项目标绑定证据。

Task checkpoint：`ad-auxiliary-session-classification-integrated-acceptance`；上述 ContentState；VERIFIED 9/9。
提交建议：建议在单独授权后提交两份稳定 specs、tasks.md 状态及本评审记录；本候选任务与 Topic 门禁均已 VERIFIED。
推送建议：待单独授权提交并核验提交对象及 SSH 签名，再按单独推送授权交付至 `origin/feature/auxiliary-session-classification`。

Unit completion checkpoint：`auxiliary-session-classification`；同一 ContentState；Topic 门禁 VERIFIED 2/2。评审与证据已满足收尾检查点，Git 交付和后续整体归档仍未执行；当前主题保持 active。

## Round 6 — 2026-11-13

## 📋 Task 9 CI follow-up 独立评审

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer: Codex，独立评审会话；未参与该候选实现，未委派。
- Method: 冻结提交的增量源码审查、工作树 CodeGraph 调用路径、历史验收与目标态证据复用评估；直接读取 GitHub 作业日志；独立运行文档工具回归。
- Scope: `f39f86a..74964a2` 的文档检查器与 Swift 修复，稳定 specs 的已评审 blob 对账，以及 Task/Topic 门禁。历史集成实现复用 Round 4/5，不重做无变化的性能矩阵。
- Reviewed state: HEAD `74964a2a0a8a4bade0ba6c1cea25e8d0f4d6667a`，tree `d64c7ae095089ae5d02d3555c1b21f02722d6e5f`，入场工作树干净。
  ContentState: `auxiliary-session-classification:integrated-acceptance:commit-74964a2a0a8a4bad`。
  本轮追加的评审与 tasks 状态沿用既有排除规则，不改变产品候选身份。

### 🔴 严重问题 — 必须修复

无。未发现本候选引入的可行动缺陷。

### 🟡 建议改进 — 推荐

无新增发现。ASC-IA-R4-F1 仍由 `ad-bug-selection-verify-200ms-deadline` 承载，不在本轮宣称修复。

### 🟢 优点

- 文档入站检查使用解析后的完整本地目标匹配；外部 URL 与不同目录同名文件不再误报。无法解析的语法仍走保守分支，全文检查仍拒绝不支持的缩进链接，真正受影响的本地目标仍失败。
- 动画测试使用真实工具栏 action，保持 active/key/visible 前置条件；包装原 `resizeWindow` 而非替换为假实现，断言第一个请求带动画，最终高度、标题和选中页正确。
- 默认闭包仍将相同 window/frame/animate 传给 `setFrame(_:display:animate:)`，`display` 仍为 true；尺寸计算、可见性判定、重入保护与补偿 refit 均未改变，诊断输出已移除。

### 📝 总结

本轮 PASS；ASC-IA-R1-F1…F5、R2-F1/F2、R3-F1 的关闭状态不变。Round 5 两份稳定规格的 blob 未变：
cli-design `7fe242acad4d4e0d428e400f83618bad075ad706`，cli-manual `907d221515ed8b1489d35f25ae3a1c0b528a405a`。
本轮不改产品、测试、配置，不执行提交、推送、集成、归档或发布。

动画验收的证据边界：

- 直接读取诊断 job `131769173790`：2026-11-13 08:09:28 UTC，window 70，`animate=true`、duration `0.34518178083084844` 秒，active/key/visible 均 true，`reduceMotion=true`；
  内容高从 148 到 466，仅一个 resize 回调，调用耗时 `0.0016253333333224873` 秒。它证明该环境没有观测到中间高度；没有系统设置 A/B 对照，不能把 Reduce Motion 的单因果解释提升为已实验证明。
- 本次 CI 只直接证明动画请求和最终布局，不能声称本次观测了动画帧。Settings Round 9 已记录活动窗口的真实中间高度与最终状态；
  相对本次修复前候选，生产路径仅提取默认转发闭包，动画参数、目标几何和同步调用顺序不变，因此该历史原生观察可按语义等价复用。
- ASC-SC-R8-F2 不重开，也不删除「随页切换并带动画」要求。本次请求断言补充了稳定的自动回归保护；原生过渡结论来自历史实测加本次影响评估，不能单靠 `requested.first == true` 证明。
  后续若改变默认闭包、动画参数、目标几何或调用顺序，须重新评估原生过渡证据。
- 本轮未执行变异、未切换用户的系统设置、未重新运行本地 macOS 全套。交接中的本地慢宿主失败原因仍未独立诊断；同一候选的远端完整套件通过，不能据此声称本地失败已修复。

Evidence:

- 独立执行 `python3 -B -m unittest discover -s scripts/ci -p v3_contract_test.py`：25 项通过，退出 0；其中受影响/不受影响入站链接及全文拒绝语义均有直接断言。
- CI run `43552367297`：classify、documentation、desktop、verify 全部 success。直接读取 desktop 与 verify 日志，均确认 checkout 为本轮完整 HEAD。
- desktop job `132317671261`：Xcode 26.6（17F113），macOS arm64；App 237 项、跳过 3、失败 0；Shared 163 项与 Widget 59 项均失败 0。
  动画用例通过（1.069 秒）；`SwitchFlowTests.testConfirmationUsesExactlyTheResolvedOption` 通过（0.222 秒）；主套件 `TEST SUCCEEDED`。
  随后的两个独立重启阶段各 1 项通过，耗时分别 1.260 与 1.008 秒；主套件中的阶段跳过不充作重启通过。
  App 构建、Widget sandbox、desktop refresh integration、distribution isolation 各步骤均 success。
- verify job `132317671304`：Go 1.26.0 darwin/arm64；`scripts/run-go-test.sh ./...`、`scripts/run-go-test.sh -race -p 1 -timeout=20m ./...` 与 `go vet -mod=vendor ./...` 均通过。
- 复用范围：六项 Go 集成行为与 performance-matrix 的产品、测试、输入及配置不受本次 Settings 缩放闭包和文档工具影响；性能路径 P01/P03 为菜单栏，不经过 Settings 切页。
  原七项证据及局限通过显式 preserves 与目标绑定 roll-up 复用。L3 使用本轮目标态 CI；独立评审使用本轮观察。
- 历史目标 `auxiliary-session-classification:stable-specs:f39f86a-7fe242ac-907d2215` 的固定模板查询仍为 VERIFIED 9/9；新目标入场缺 ContentState，NOT_VERIFIED，不能直接借用旧门禁。
- `python3 scripts/check-docs.py`、`python3 scripts/check-review-records.py --base HEAD docs/topics/auxiliary-session-classification/reviews/integrated-acceptance.md`、`make check-whitespace` 与 `git diff --check` 均通过。
  上述 CI 事实直接保留在本记录，不依赖临时日志或有保留期限的附件。

残余范围不变：浏览器为渲染帧而非真实 PTY；原性能证据保留原环境与样本局限；P06/P07/P11 与 P01 rebuild 层保持排除；没有新增真实用户数据、插件健康或 Widget 调度验收。

CEv1 WorkUnit: `auxiliary-session-classification:integrated-acceptance`。
Target ContentState: `auxiliary-session-classification:integrated-acceptance:commit-74964a2a0a8a4bad`。
完成门禁：VERIFIED。

新建 ContentState、change、八项 preserves 评估与十一项证据，共 21 个节点；55 条关系预检全部 ok。
写工具回传 nodes_created/relationships_created，随后按提交 ID 集合回读全部节点负载和关系端点，数量及内容均一致。
固定 gate-status 模板依次查询 Task 与 Topic：VERIFIED 9/9、VERIFIED 2/2，缺失、失效与未解决影响均为空。
证据前缀为本 ContentState 加 `:r6:`。历史节点未重标，未将旧门禁直接当作新目标结果。

Task checkpoint：`ad-auxiliary-session-classification-integrated-acceptance`；上述 ContentState；VERIFIED 9/9。
提交建议：仅提交本轮评审与 tasks 状态；产品及稳定 specs 已在历史交付提交中，不重做交付。
推送建议：本轮记录经单独授权提交及对象/SSH 签名核验后，再按单独授权推送 `origin/feature/auxiliary-session-classification`。

Unit completion checkpoint：`auxiliary-session-classification`；同一 ContentState；VERIFIED 2/2。
九个实现任务的层级已核实，其余八项 Beads 为 closed；本任务等待评审记录交付。主题保持 active，归档、集成与发布未执行。
