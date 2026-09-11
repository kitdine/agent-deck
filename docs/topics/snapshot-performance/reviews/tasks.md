---
status: active
topic: snapshot-performance
subject: tasks.md
---

## Round 1 — 2026-09-09

## 📋 任务分解评审

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 推荐

无开放 finding。

### 🟢 优点

- 先固化完整输出/逻辑数据基准，再实现共享解析、代次和缓存，最后验收完整周期。
- 原始偏移、机器身份、分域失败、恢复、未来 schema、时间/价格失效及隐私均有任务承接。
- 未将 10 秒目标的设计阶段放宽当作交付豁免；最终验收承担未达标处置。

### 📝 总结

Reviewer: Codex；session 01a085b3-3e60-7ae2-9524-3a1316aec423。
Method: 单一主评审角色的需求到任务追踪、依赖图检查和 L0 文档检查；无子代理。
本会话只同步了先前文档评审状态，未起草分解；本轮不宣称独立冷上下文评审。
Scope: tasks.md 文档集声明、六项任务的边界、先后关系、验证分配和交接。
Workspace: agent-deck.snapshot-performance / feature/snapshot-performance。
Task: ad-sp-doc-tasks-design；WorkUnit: snapshot-performance:tasks.md。

Reviewed state: HEAD f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f；
tasks.md blob 12bb0f670737275da06374f466ace3cf3263c7cc；
SHA-256(head=<HEAD>;document=<blob>) =
3cfac2d2af561783f4dae5d570ecbe1acde5248e77a9b2f411f1748a2fcb9b83。
评审输入 blob e5037d6335151a7c2db3cf192d462427cc36e6d6；本轮只更新 Review
勾选和交接，任务定义不变。最终证据绑定同步后的 tasks.md，而非旧输入指纹。

Evidence:

- 复用 requirements.md / architecture.md Round 1 PASS 及其 VERIFIED 文档门禁。
  两文档内容未变，分别对应 blob 01a3a52943757a73873d27f78cea683741ccdd0e、
  723f3743f6c0ec1d04a4e6068f88e48790b37acf；未重跑已完成的源码/契约核对。
- Task 1 承接固定语料、参考结果、业务时钟和完整样本/资源计量；Task 2 承接
  共享读取解析、顺序、身份、分域提交与冷导入优化；Task 3 承接 schema、
  epoch/revision、源签名和恢复；Task 4 承接 DTO、Summary、价格 availability、
  有界私有发布、失效和回退；Task 5 承接两 helper 的完整未变化路径；Task 6
  承接最终差分、资源/性能验收及稳定契约、desktop-refresh 交接。
- 声明依赖 1→2、1→3、2/3→4、2/3/4→5、1–5→6 无环。
  Task 3 明确与 Task 2 协调源签名接口；此处不授权并行代理或并行实施。
- 无新增表面/交互，UX n/a 与两份已批准设计一致；Documents 声明的三份
  文件均存在、Draft 完成且各有匹配评审记录。六项 Dev/Review 均保持未完成。
- L0/L2/L3 分配与项目矩阵相符；并发、迁移、恢复、权限及真实 helper 验收
  分配给对应任务。未把文档评审当成运行时测试，不要求本轮跑产品套件。
- bash scripts/check-topic-docs.sh exit 0；新增记录及状态同步后再进行最终
  文档集、空白和相对链接检查亦全部通过；make check-whitespace、
  git diff --check exit 0，主题所有 Markdown 的未跟踪空白/相对链接检查通过。
  该检查器验证结构，不替代语义审查。

完成门禁：VERIFIED
固定 gate-status.cypher 查询最终指纹对应 ContentState，三项必需准则均有
有效 pass evidence；缺失、失效、未决影响列表均为空。节点批次创建 4/1/3 个，
关系批次创建 3/6 条，数量与提交一致；关系 preflight 全部 ok，最终门禁回读
确认准则及证据目标绑定。不关闭包含本任务的功能主题。
整个 snapshot-performance 仍有六项实现工作，原始刷新问题也仍有其他主题的责任。

Task checkpoint：ad-sp-doc-tasks-design；content_state 3cfac2d2af561783f4dae5d570ecbe1acde5248e77a9b2f411f1748a2fcb9b83；门禁 VERIFIED。
提交建议：单独授权后提交 tasks.md 和本评审记录；当前三份文档均未提交，如选择整个文档集交付，须同时明确前两文档任务及其评审记录的暂存和贡献边界。
推送建议：单独授权且完成提交、签名和远端核验后，推送候选 origin/feature/snapshot-performance；本轮未执行交付。

下一步指令：开发：snapshot-performance / performance-contract
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 2 — 2026-09-10

## 📋 整个 topic 文档体系重审与重规划

Checklist: 60/60 complete.
Incomplete: None. Checklist completion means the old plan was examined, not that
the replacement drafts or their missing UX specimens passed review.

📊 总体评分：3/10（重规划前的整套执行计划）

✅ 评审结论：FAIL

Plan verdict: REVISE. This is a review of the previous complete plan and a new
replacement draft, not approval of the newly authored documents.

### 🔴 严重问题 — 必须修复

**TOPIC-R2-F1 — 高：原六任务与 W0-W7 同时承担执行责任，没有唯一可领取分解。**
旧 tasks.md Tasks 仍是 performance-contract 到 acceptance-and-reconciliation；
worker supplement §14 同时给出 W0-W7，且 §1/§14 要求旧状态保持不变、后续再绑定。
因此“开发 W0”不是已有任务，也无法从旧矩阵判断 worker 完成了多少。
后果已发生：文档 PASS 的下一步建议指向没有 dispatch 的包。
修正：仅保留 tasks.md 的三个成果任务，把测量、调用盘点和局部实现步骤放进
各任务范围；旧分解转为历史。依据：项目 Documentation Workflow 的 Topic
structure/Status 与 Beads 的一任务一对象规则。这是仓库工作流事实，不依赖
外部平台假设。Disposition：已写入替代草案，待新文档集评审确认。

**TOPIC-R2-F2 — 高：需求仍排除新交互与后台运行，而补充设计已改变产品契约。**
旧 requirements Scope 声称表面/交互不变、UX 不需要；原 architecture 保持
foreground-only/无 daemon 叙述；补充设计引入 detached worker、scoped wait、
重连和 App 进度。仅靠跨文件 precedence 表不能让当前需求成为真实入口。
修正：更新 requirements，合并 worker 与缓存契约到 architecture，补充文件
降为历史；明确 on-demand worker 与 installed permanent daemon 的区别。
Disposition：已写入替代草案；历史单文档 PASS 不转移到新内容。

**TOPIC-R2-F3 — 高：新 CLI/App 状态没有对应 UX/specimen 验收责任。**
原 UX n/a 与补充 §10 的等待、导入、统计、失败、detach/reopen 不相容。
把原型工作推到后续实现却仍称整套设计通过，违反项目 specimen requirement。
修正：新增 ux/cli-scan.md 和 ux/menubar-scan.md 交互草案，列出状态、copy、
输出/线程生命周期、窄宽和可访问性边界；shared prototype 是唯一 specimen
来源。Disposition：OPEN；两份 UX 的 Draft 保持未勾选，必须完成实际原型
呈现与验证后才能称整个设计集就绪。不是通过更多后端单元测试来关闭。

**TOPIC-R2-F4 — 高：没有定义旧候选的退役与替代交付边界。**
新设计要求替换调度/生命周期，但 Task 2 仍被当作必须先修复、评审、提交的
生产成果，导致旧实现修复循环成为新实现前置。文档提交引用未提交代码时，
先前主评审也把先交付代码误当唯一方案。这是交付安排错误，不是用户扩大范围。
修正：用户已允许代码整体重写；旧实现不再要求先交付。旧 FAIL 与实际修复
历史保留，两个 defect reproducer 归入 unified-scan-runtime 验收，不冒充已修复。
旧未完成 dispatch 暂停，新 implementation dispatch 在新矩阵获批后建立。
Disposition：已写入替代草案与协调；新引擎仍须证明正常/取消/错误终结。

### 🟡 改进建议 — 建议处理

无风格类 finding。没有借重规划扩张为 fsnotify、永久服务、自适应调度、
新 SQLite 依赖或更改刷新/Widget 策略。

### 🟢 保留的有效成果

- performance-contract 的 446a58f 与对应历史 Round 2 PASS 保留；完整周期、
  失败样本保留、逻辑核验独立计时的原则继续作为测试约束。
- 原 requirements 的数据正确性、价格精度、隐私、只读 snapshot 与性能目标保留。
- 原架构的 epoch/revision/dirty 算法、独立 checkpoints、8 MiB 私有缓存与
  时区/价格边界合并进入单一现行架构，不重新发明。
- worker 选择、全域执行/scope 等待、有限 round、兼容性协商、真实后台计量
  继续有效。废弃的是双重计划和错误状态继承，不是这些用户已决定的产品语义。

### 📝 总结

Reviewer：Codex 主代理。Method：ln-11-plan-reviewer；单代理、仓库证据先行；
执行模拟、新实现视角和取消/交付反例三种视角各做一次自查，未委派代理，
不声称冷上下文独立性。用户明确要求重规划，因而本轮在审查后改写产品设计
文档与必要协调；没有修改 AGENTS/Skill 或生产代码。Skill 的只读默认不限制
用户明确授权的规划改写。

Scope：四份原 topic 根文档，以及 requirements、architecture、tasks、
performance-contract、shared-ingestion、worker-scan-development-design 六份
review history；同时核对 Documentation Workflow、Beads、Project Rules、
prototype/README、go.mod 和实际 Go/Swift 验证脚本。复用当前会话已读完整契约
与源码证据，不因本轮开始重复运行产品测试或重做性能研究。

Reviewed baseline：HEAD 446a58f1f6716f257680879e5dbf3b61365c8cb2；10 个输入
文档冻结在 /private/tmp/agentdeck-topic-replan-20260910/，逐文件 SHA-256
在 input-manifest.json。原文和缺陷关系可复核；未删除未提交实现。

Policy evidence：

- docs/documentation-workflow.md Topic structure：一份 requirements、一份任务矩阵、
  按实际表面定义 UX；Specimen requirement：共享原型与原生证据区分。
- .agent-instructions/beads.md：未获批的分解不提前创建实现任务；deferred 仅暂停，
  不代表工作完成。旧公开历史和 gate facts 保留。
- 原型 README：CLI 输出为英文，App 中英；共享原型是唯一表面标本，420/280
  和真实 accessibility/runtime 检查有明确界限。
- go.mod：Go 1.26.0，modernc.org/sqlite v1.53.0；本轮不新增依赖、不修改版本。
- scripts/run-go-test.sh、scripts/test-macos-app.sh、scripts/build-macos-app.sh
  存在；执行能力、签名、机器环境和新引擎性能本轮不作已验证声明。
- Cross-topic overlap：对 schema-version-signal、subscription-quota、
  v0-6-0-contract 的 main...feature 已提交差异定向检查，internal/store 均无
  差异；schema-version-signal 的 CurrentSchemaVersion 仍为 23。不抢占未来
  migration 编号，任务 1 实际添加迁移前仍需核实届时的 schema owner。

Replacement plan：tasks.md 的 unified-scan-runtime → snapshot-computation-reuse
→ scan-experience-acceptance。没有 W0、研究任务或隐藏的二次分解；每个任务包含
自身的调研、实现、验证、移除旧路径与完成条件。最后一项承接整体验收，不再
建立另一套 acceptance 计划。技术参数由测量决定，不把它们变成新的用户审批
循环；改变产品语义或放宽性能目标仍需显式决定。

Subtraction ledger：移除第二套活跃执行计划与旧代码先交付前置；合并重复架构；
不引入永久服务/新依赖/通知监听；保留已验证语义与历史证据；生产旧路径的实际
移除由 unified-scan-runtime 完成，不能在文档阶段虚报已移除。

完成门禁：NOT_VERIFIED（新文档集和新任务均未验收；UX specimen 仍缺）。
旧 CEv1 PASS/FAIL 只针对旧内容，不批量改写为新计划完成。此次 scope 是用户
要求的全 topic 重审和规划，不执行旧下一步指令，也不开始任何代码重写。

Final document validation：check-topic-docs、whitespace、diff-check 通过；42 个
topic source-document 相对链接/锚点有效；现行任务矩阵唯一且恰有三行任务；
新文档 Review 均未勾选；23 个生产/测试 blob 与入口候选完全相同。旧五个未完成
实现 dispatch 已 deferred，四个文档任务重新进入起草，两份新增 UX 文档任务
明确保留 specimen 缺口；未预先创建三个新实现任务。


## Design handoff — 2026-09-11

The shared product prototype now supplies the previously missing CLI/menu-bar
scan specimens. Both UX Draft cells are checked; every revised document Review
cell remains unchecked. TOPIC-R2-F3 has been addressed in the design candidate,
not independently closed by a review verdict. Current specimen source, screenshots
and browser checks are linked from the UX documents. The 2026-09-10 audit above
remains historical; its then-missing specimen is not the current handoff blocker.

Browser evidence: menu-bar 14 × 2 and CLI 11 × 2 assertions pass; 128 display/state
combinations pass; actual browser status has polite/atomic semantics; scoped new
scan-region axe audits have no violations/incomplete items. Build passed. Native
worker/IPC/Swift/accessibility/performance are not claimed. Next scope is the
complete revised document set with the shared prototype, not old-code repair or
an implementation task. No review PASS, commit or push was performed.

## Round 3 — 2026-09-11

## 📋 现行设计文档集评审（含共享原型）

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**TOPIC-R3-F1 — 高：合并架构时丢失仍被任务依赖的缓存契约。**

位置：`architecture.md:480–499`、`tasks.md:94–96`。
现行架构称使用 “the existing architecture”、保留 “the existing allowlist” 和
“the original derived-cache contract”，但这些段落现在指回已被替换的文档本身。
末尾只恢复了 generation 算法，没有恢复 payload/DTO 和缓存读取有效性契约。
Task 2 仍明确要求实现 architecture 的 allowlisted DTO、Summary 和时间/价格边界。

- 行为风险：实现者必须重新决定缓存哪些报告、怎样恢复内部 Summary、如何构成
  header/时间窗口以及读取时如何复核代次。公共 JSON 往返会丢失 Summary；只按
  当前 UTC offset 或固定 TTL 缓存也不能保证跨时区、小时和未来价格生效点的正确性。
  需求中的“精确等价”目标并不能替代这些已做过、如今缺失的实现契约。
- 证据：`git show HEAD:docs/topics/snapshot-performance/architecture.md` 的
  146–195 行明确规定 0700/0600、单 active 加单临时 entry、完整写入/sync/rename、
  header 身份、timezone identity 与本地窗口 UTC 边界、最早 hour/day/price 截止点、
  时钟回退拒绝、payload 白名单、专用 DTO/Summary 和 read 后 generation 复核。
  当前现行文档没有对应定义；历史 supplement §9 也只是同样的引用，且其开头
  已明确取消当前权威。`internal/usage/presentation.go:21` 当前仍为
  `Summary Summary` 配置 `json:"-"`，证实丢失 DTO 决策具有实际语义后果。
- 💡 修复建议：在现行 `architecture.md` 内恢复并与新 worker 路径协调上述缓存
  契约，明确字段白名单、内部 Summary、header/有效期、私有原子发布和读取复核；
  删除悬空的自指引用。核对 Task 2 的每个缓存要求都指向实际现行定义。可复用
  已提交旧架构中的有效设计，不恢复旧执行计划，不要求先修旧 Go 候选。
- Disposition：OPEN。Owner：`ad-sp-doc-arch-design`；依赖该契约的文档集/
  分解由 `ad-sp-doc-tasks-design` 跟踪本轮。两个任务共用此 finding，不复制缺陷。

### 🟡 改进建议 — 推荐

无额外 finding。发现上述可确定的设计阻断后，按 Review 的 stop-broad-verification
规则停止扩大验证；没有用未审完的部分推导 PASS。

### 🟢 优点

- 三任务矩阵、旧 W0-W7 的历史定位和旧候选不必先交付的边界清楚。
- 全域执行与 scoped wait、有限观察轮次及后台资源计量已写入现行需求和架构。
- 两份 UX 文档引用了可定位的共享 specimen；本轮校验 manifest 全部 28 个条目
  的 SHA-256 均匹配，文档引用的 manifest 摘要也匹配。未将浏览器证据冒充原生验收。

### 📝 总结

Reviewer：Codex 主代理。Method：development-workflow REVIEW 的设计/契约维度，
单代理交叉引用核验、与提交版比较、定向源码确认；无委派，不声称冷上下文独立评审。
Scope：现行六文档与其共享原型依赖；补充文档只检查 superseded 定位。
本轮结论属于整个文档集，沿用本记录 Round 2 的整套审查关系；直接缺陷归属
architecture.md。其他文档仍未取得本候选的独立 PASS，不为它们制造通过记录。

Reviewed state：HEAD `446a58f1f6716f257680879e5dbf3b61365c8cb2`，
workspace `agent-deck.snapshot-performance`，branch `feature/snapshot-performance`。
以下 Git blob 覆盖现行六文档；tasks 的最终 blob 只增加本轮状态交接，未改变分解：

| Subject | Git blob |
| --- | --- |
| requirements.md | e106da96b4af57e648972e4528ad68fb0d4d675f |
| architecture.md | 7e3edb38a06b9d347370972f5305adbfcc84503f |
| ux/cli-scan.md | 0977c0600ab380ee0fb7cf3be7aba5245a4d4400 |
| ux/menubar-scan.md | a6bf1899b030de8f14b669d9d6ede4cdf8fc84ba |
| tasks.md（审查输入） | 768e660cd0c3c0efeb76f06eeedd25b825cc488f |
| tasks.md（状态同步后） | b21842ffc944b4eb2de5a29eefe4343fecc8379f |
| worker-scan-development-design.md | 95077f4f4238d307db95eae02fcfd16f364b8879 |

Specimen：`ux/prototype/scan/manifest.json` SHA-256
`cf26b5d5f7afd30bf57eca9f4e7fa6c373eaec0b786e2be5eb02c7de973def8f`；
它绑定共享源码、资源、包配置、六张截图及 checks.json，28/28 均匹配。
定向源码 `internal/usage/presentation.go` blob：
`7e6801dde6d2e128a3aa54478923938473c8fc57`。

Evidence：`bash scripts/check-topic-docs.sh` 通过；`make check-whitespace`、
`git diff --check` 通过；manifest 使用 `jq` 输出 sha256/path 后以
`shasum -a 256 -c` 校验全部通过。文档集合结构通过不消除语义缺口。
本轮没有运行 Go/Swift、性能实验或浏览器交互；历史浏览器报告仅确认所绑定内容
仍一致，未复述为本轮独立行为验收。未修改生产代码、测试、配置或原型。

历史 disposition：TOPIC-R2-F1/F4 的唯一分解及替代边界已体现在现行文档；
TOPIC-R2-F2 的 worker/UX 范围已对齐，但其“合并缓存契约”的缺口由 TOPIC-R3-F1
具体承接，不能笼统视为整套设计完成；TOPIC-R2-F3 的缺失 specimen 已有产物和
可验证内容身份，UX 独立评审仍待后续完成。本轮不改写旧内容的 PASS/FAIL 或证据。

完成门禁：NOT_VERIFIED。文档集仍有开放契约 finding，未跨过完成边界；本轮未
查询/写入 CEv1，也不把旧精确状态的 VERIFIED 迁移到当前草案。
六份文档 Review 均保持未勾选。架构和分解返回修复，其他四个文档任务保持
in_review 并释放本轮负责人。不创建新实现任务，不提交或推送。

下一步指令：修复：snapshot-performance / reviews/tasks.md / TOPIC-R3-F1
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Repair handoff — TOPIC-R3-F1 — 2026-09-11

TOPIC-R3-F1: repaired in candidate; awaiting document-set re-review. Round 3's
FAIL and OPEN disposition above remain the original review, not overwritten by
this repair. The current Review cells remain unchecked; no new implementation
or old-code repair is included.

Scope: architecture.md §9 and Task 2's references/current handoff in tasks.md.
The committed cache contract was restored into the current worker architecture:
explicit four-field allowlist and internal Summary; live exclusions; identity,
timezone/window and expiry header; 0700/0600 and 8 MiB limits; one active/one
publisher temporary entry, checksum and atomic publication; post-read generation
and time revalidation, read-only miss fallback and independent domain outcomes.
Worker core-cache building does not require session success, add a second scanner,
hold state write authority across aggregation or omit waited work from timing.
The generation algorithm now has an actual local anchor instead of a dangling
reference to an overwritten architecture. Task 2 links directly to §9.1–§9.4.

Validation: check-topic-docs.sh, make check-whitespace and git diff --check pass.
Four Task 2 anchors resolve. A focused contract-coverage check confirms the
restored decisions and absence of the three dangling self-reference phrases.
These checks support repair readiness, not independent review or runtime proof.
No Go/Swift test, browser replay or performance benchmark was needed for this
text-only contract restoration; prototype and production content were not edited.

Content: HEAD 446a58f1f6716f257680879e5dbf3b61365c8cb2;
architecture.md blob d3dc83eea3c47fbd1dfd1c2f87a52aa8ff53e78f;
tasks.md blob 915ae28a6edd202f2348e1dc8b167cda162683bd.
Workspace: agent-deck.snapshot-performance / feature/snapshot-performance.
Completion evidence: no new VERIFIED or document PASS claimed; the current
completion boundary remains open for the document-set re-review.

Next instruction: 复评：snapshot-performance / 全部现行设计文档（含共享原型）
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
## Round 4 — 2026-09-11

## 📋 tasks.md 现行设计复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无未关闭 finding。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

三任务独占 runtime、计算复用和最终体验/验收责任；Task 2 引用已恢复的缓存契约；实际 native/IPC/性能验收均留在实现任务，没有 W0 或旧代码先交付前置。

### 📝 总结

Reviewer：Codex 主代理。Method：development-workflow REREVIEW，单代理，
逐项 finding 处置、当前源码/合同交叉核验、共享原型浏览器观察；未委派，
不声称冷上下文独立性。Scope：tasks.md 及所依赖共享 specimen。

逐项 disposition：

- TOPIC-R2-F1 CLOSED：现行 Tasks 只有三个成果任务，W0-W7 已历史化。
- TOPIC-R2-F2 CLOSED：需求、worker、CLI/App 范围一致；遗漏的缓存部分现由 §9 完整恢复。
- TOPIC-R2-F3 CLOSED：两份 UX 及共享 specimen 已完成本轮评审，内容身份与验证边界明确。
- TOPIC-R2-F4 CLOSED：旧候选保持历史 FAIL，替换入口不要求先修/先交付旧实现；旧缺陷归入任务 1 验收。
- TOPIC-R3-F1 CLOSED：现行 architecture 恢复原有有效契约；未恢复旧双重计划。

本轮与本记录 Round 2/3 共享整套审查历史，各单文档记录的新轮次是本轮的
subject-specific 投影，不覆盖别的记录轮号。六文档逐项结论均为 PASS。

现状核对：cmd/agentdeck/desktop.go:139–207 确认当前旧 coordinator、独立结果和
session fingerprint 失败路径；internal/usage/presentation.go:21 的 Summary 仍
不在公共 JSON 中；usage/session 的 registry、partial line、source priority 和
anchor 由各域独立维护。EmbeddedHelperRunner.swift:346–505 当前先 refresh，
再 snapshot，process 收集输出后返回；新实时事件消费由任务 3 实现，未冒充已有能力。
这些是设计起点核验，不是对保留旧 Go 候选的交付评审。

跨文档推演覆盖 finite cutoff/持续追加、晚到请求、usage 成功/session 失败、
全部 subscriber 离开、commit 后 receipt 前崩溃、maintenance 锁顺序、不同版本与
state root、慢 subscriber、读取中 generation/时间变化及首次无数据。合同明确
拒绝无限追赶、错误域掩盖、snapshot 写入和后台资源漏计；未发现新的阻断设计缺口。

验证复用：specimen manifest 的 28 个文件在 Round 3 已逐项校验，本轮没有原型
修改且 manifest 摘要相同；保留 checks.json 的 128 组合、两语言共 50 项交互及
浏览器 AX/axe 证据。新增浏览器观察为 CLI 英文 11 项和菜单栏中文 14 项，均通过。
完整截图位于 /private/tmp/sp-r4-first-use-full.png、sp-r4-partial.png、sp-r4-cli.png。
短视口中居中首用卡在页面下方，完整截图内容可见；这不作为原生布局缺陷。

57 个相对文件链接存在；Task 2 四个缓存锚点对应当前 headings；
check-topic-docs.sh 通过。记录/矩阵的最终空白和 diff 检查另按 L0 收口。
本轮只更新 review/readiness/handoff 字段，未改行为设计、Go、Swift、测试或原型。

Reviewed state：HEAD 446a58f1f6716f257680879e5dbf3b61365c8cb2；
Git blob d1e561c25ac782423024128bf1a0f83442bce492；content_state e73b5ce53d8d795da4d95caddd52e39cdba62bee3e8bdf98b5e1941bac607fab。
配方：SHA-256(head=<HEAD>;document=<blob>;prototype=<manifest_sha256>)。
Specimen manifest SHA-256：cf26b5d5f7afd30bf57eca9f4e7fa6c373eaec0b786e2be5eb02c7de973def8f。
Workspace：agent-deck.snapshot-performance / feature/snapshot-performance。
Task：ad-sp-doc-tasks-design；WorkUnit：snapshot-performance:tasks.md。

完成门禁：VERIFIED。固定 gate-status.cypher 对本轮精确 ContentState 查询通过；
required criteria 为 3 项，missing/invalidated/unresolved 均为空。
本轮追加 37 个节点、58 条关系；关系预检 58/58 ok，实际创建数量匹配。
没有复写历史观察或把旧已交付实现的证据改成新引擎验收。

Task checkpoint：ad-sp-doc-tasks-design；content_state e73b5ce53d8d795da4d95caddd52e39cdba62bee3e8bdf98b5e1941bac607fab；门禁 VERIFIED。
提交建议：单独授权后提交 tasks.md、对应评审和 topic 矩阵；包含已审核的共享原型及其证据，不混入未评审 Go 候选。
推送建议：origin/feature/snapshot-performance（候选目标）；须单独授权，核验暂存范围、提交正文/署名/SSH 签名和实际远端配置。本轮未推送。
原生 worker/IPC/SQLite/Swift/VoiceOver/Dynamic Type 和完整性能指标不属于文档
PASS 的证明范围；任务 1–3 及 topic 完成边界仍开放。未提交、推送或启动实现。

最终交接：六个文档任务均为 awaiting_commit，已清空评审负责人并写入逐任务
checkpoint 评论。获批的三个实现对象为 ad-sp-unified-scan-runtime-dev、
ad-sp-snapshot-computation-reuse-dev、ad-sp-scan-experience-acceptance-dev，
均为 open/unassigned，parent 为 ad-snapshot-performance，按任务 1 → 2 → 3
建立 blocks 关系。首任务另由 ad-sp-unified-scan-runtime-dev-gate 保留开发授权
边界；后续任务仍各需真实用户开发指令。未创建第二套任务分解或启动实现。
最终状态链接共 59 个，均存在；最终 review/checkpoint 的 L0 空白与 diff 检查通过。
截图 SHA-256：first-use-full 639109417f1c31cb968fc14ed6e62c573dfb72c691d65c75b79faf1431ac8bbd；
partial 8765be283f568d5514fa3986d27917eb7217dc3bae6de9f5a2cc04b539815ed4；
cli 1bcd392f42ac13e737889dc0d62710df32501ae300dea440954ffba1580420c9。

下一步指令：开发：snapshot-performance / unified-scan-runtime
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance
