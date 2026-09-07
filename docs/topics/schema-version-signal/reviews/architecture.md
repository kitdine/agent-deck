---
status: active
topic: schema-version-signal
subject: architecture.md
created: 2026-09-07
updated: 2026-09-07
---

# Review — architecture.md

## Round 1 — 2026-09-07

- **Reviewed state**: HEAD `34bf55ec0d9c0c5d4a4d632a740adcdb2394202a`，文档 blob
  `e746773f012e5a966e8f94b7ebac734b64bdae5a`（未跟踪）。对照物：
  `requirements.md` blob `b0854d0057df1dfd5bd07f6cec57e5c523f7333f`、
  `ux/menubar-schema-signal.md` blob `bab2f89c3a65e64ed639e26addf4cb50bda3b10b`、
  `tasks.md` blob `7ea1ce47e35767bfb137a44417ade3596752dbce`。
- **Reviewer**: Claude Code。独立性：本会话未参与本文档的起草，对其内容是冷上下文，
  角色分离。如实记录较弱的一条局限：起草与评审同属 Claude Code 运行时与同一套项目
  指令，共享同样的盲区；本轮以逐条重测源码坐标对冲这一点。
- **Method**: 设计/契约类目标的维度评审——前提有效性、与当前实现的一致性、场景覆盖、
  决策完备性、内部矛盾、隐性耦合、假设依赖。逐条核对文档声称的源码与规范坐标（Go、
  Swift、`docs/specs/cli-design.md`），全部亲自读取而非采信；运行项目自带的文档集
  审计 `scripts/check-topic-docs.sh`；把 C1/C2 的决定逐一回代到仓库里既有的断言与
  `prototype/`。评审阶段对生产代码只读，未执行 Xcode 构建。
- **Scope**: `docs/topics/schema-version-signal/architecture.md` 全文 599 行，及其
  对 `requirements.md` 与 `ux/menubar-schema-signal.md` 的承接。不含 `tasks.md`
  （未写）、不含实现、不含 `cli-design.md` 的实际编辑。

📊 综合评分: 8/10

✅ 评审结论: FAIL

### 🔴 严重问题（必须修复）

**ARCH-R1-F1** — 契约把稳定码定为 `schema_ahead`，但被 surface 声明为设计真相的
`prototype/` 仍硬编码 `unknown_schema`，本文档既未在 Changes 里点名，也未把它列入
stage 7 的吸收清单。

- **位置**: `architecture.md` 的 C1、C2，以及「What existing behavior this changes,
  and what it does not」与「Where this document departs from `requirements.md`」两节。
  受影响物：`prototype/src/data.js:439`、`:454`，`prototype/src/Popover.jsx:841`、`:848`。
- **行为风险**: `prototype/README.md:3` 声明 prototype 是「产品全部界面的唯一设计
  真相」，而 `ux/menubar-schema-signal.md` 在其 Round 2 修复中正是把标本来源改成
  prototype 并写入 prototype-wins 条款后才通过评审的。C1 决定**不**提升
  `unknown_schema` 而启用新码 `schema_ahead`，C2 决定两个数字走 `count` 与
  `supported_count`。prototype 的两个 stage state 写的是
  `code: "unknown_schema", storedVersion: 999, supportedVersion: 23`，而 `Popover.jsx`
  的行展开与因由/恢复两行都以 `check.code === "unknown_schema"` 为谓词、以
  `check.storedVersion` / `check.supportedVersion` 取值。契约一旦生效，设计真相就在
  断言一个契约阶段刚刚否决的码，且已通过评审的 surface 的全部标本都是从这个状态渲染
  出来的。本文档写了 stage 7 的吸收清单——departures 两条加
  `provider_candidates_unavailable` 一条更正——唯独漏掉这一条，而这是三条里唯一会
  让已通过的标本失效的。
- **证据**:
  `rg -n 'unknown_schema' --glob '!docs/**' .` → 命中 `prototype/src/data.js:439`、
  `:454`、`prototype/src/Popover.jsx:841`、`:848`，以及 `internal/store/store.go:134`
  与四处测试断言（见 ARCH-R1-F2）。文档全文无一处出现 `prototype`。
  `sed -n '3p' prototype/README.md` → 「产品全部界面的唯一设计真相」。
- 💡 **有界改法**: 两处补写，不改 prototype 本身。其一，在「Changes」加一项，点名
  上述四处坐标，说明谓词与字段名都被 C1/C2 改变；其二，在 departures / stage 7 吸收
  清单加一条，写明 stage 7 必须把 prototype 的码与两个字段名同步到新契约、并重渲染
  surface 的标本。

### 🟡 建议改进（推荐）

**ARCH-R1-F2** — 「What existing behavior this changes」只点名两处 `future` 回归
用例，实际有四处既有断言会因 C1 直接变红，其中两处在文档从未提及的 `internal/store`。

- **位置**: `architecture.md` 的「What existing behavior this changes」与
  「Verification expectations」。
- **行为风险**: 文档说「the rest extend cases that exist」，而 stage 8 会照这份清单
  划任务边界与验证层级。C1 让 `store.go:201-202` 与 `migrations.go:316-317` 返回
  `*SchemaAhead`，其 `Unwrap()` 返回 `ErrSchemaAhead` 而非 `ErrUnknownSchema`，因此
  下列断言全部转为失败，且没有一条被文档提及：
  - `internal/store/store_test.go:220` — `OpenReadOnly` 对 `CurrentSchemaVersion+1`
    断言 `errors.Is(err, ErrUnknownSchema)`
  - `internal/store/store_test.go:460` — `TestMigrationsRejectUnknownNewerSchema`
  - `internal/doctor/doctor_test.go:178` — `version=99` 用例断言
    `store.ErrUnknownSchema.Code`
  - `internal/doctor/doctor_test.go:334` — `hasCode(report, store.ErrUnknownSchema.Code)`

  文档点名的是 `internal/doctor/doctor_test.go:349` 与
  `cmd/agentdeck/main_test.go:1461` 两条。`store_test.go:1071` 与 `:1202` 属元数据
  损坏站点，按 C1 不变，已核实。
- **证据**: 上述四处逐条读取；`store_test.go:213` 与 `:457` 分别插入
  `CurrentSchemaVersion+1`，确认命中的是 C1 改动的两个比较站点，而非元数据站点。
- 💡 **有界改法**: 把四处补进「Changes」，并在「Verification expectations」的 L2 一条
  里点出 `internal/store` 同在改动面内——当前该条只说「store, doctor, and CLI
  contract work」，读不出既有断言要改。

**ARCH-R1-F3** — `hook-refusals.json` 的唯一清除触发是「下一次成功的 Hook 投递」；
投递停止时该记录与它的 doctor 警告永久留存，且按 C6 没有任何恢复路径。

- **位置**: `architecture.md` C5 的 **Cleared** 条款与 C6 末段。
- **行为风险**: 用户升级二进制后，若不再产生 Hook 投递——关闭 hook、卸载客户端、换
  工具——记录不会被清除。`doctor` 只读该文件（C5 的「The doctor check」），不清除它，
  于是 `hook_deliveries_dropped` 永久出现在报告里；`Report.add`
  （`internal/doctor/doctor.go:481-489`）对任何非 `ok` 检查递增 `Problems`，因此
  `health.problems` 永久 ≥ 1，菜单栏按 surface 的 D3 永久渲染健康计数条。而 C6 明确
  该检查不带 `recovery_command`、其恢复「就是同一次升级」——升级恰恰不清除它。这是
  本 topic 要修的那类缺陷的镜像：一个永久状态被呈现成一个用户无法处理的告警。
- **证据**: C5 全文只有一条清除路径；C6「The same rule applies to C5's
  `hook_deliveries` check: its recovery is the same upgrade」；`Report.add` 的计数
  行为已读取核对。
- 💡 **有界改法**: 三选一并写进 C5——给记录第二个与投递无关的清除条件（例如任何成功
  打开 store 的路径，或按 `last_at` 的年龄上界）；或明确写下这是被接受的界限，并说明
  用户如何自行清除；或让该检查在数据库已可打开时降级/消失。

### 🟢 优点

- **坐标纪律经得起逐条复算。** 本轮亲自读取了文档声称的三十余处坐标，无一处漂移：
  `store.go:24`（`CurrentSchemaVersion = 23`）、`:134`、`:191`、`:200-202`、
  `:207-249`（acquire 在 :211、migrate 在 :239）、`migrations.go:317/366/413/418`、
  `doctor.go:23-29/52-55/64/66-70/82/474-479/481-489`、
  `desktop.go:204-212/214-220/252/254-255/284-286/294-310/305/758-771`、
  `main.go:301/336-374/337/361-362/363-364/371-372/1394-1401/2799/2940-2955/3654-3678/3684-3710/4430-4460/4442-4444/4453-4457`、
  `output.go:8-16`、`platform/state.go:43`、`backup.go:34-38`、
  `DesktopWire.swift:941-953`、`EmbeddedHelperRunner.swift:861-863`、
  `MenuBarSurfaceView.swift:555-571`、`cli-design.md:1243-1252/1805/1847-1848/2162-2184/2200-2220/2275-2282/2286-2291/2521/2523` 与
  `version: 28`。文档自称的坐标基准（`git diff --stat 8d283cd..34bf55e -- '*.go'
  '*.swift'` 为空）也复算为真。
- **C1 的理由是本轮无法在更早文档里找到的事实。** 「`ErrUnknownSchema` 有五个产出
  站点、其中三个是元数据损坏、没有可报告的版本对」——这条既决定了不提升旧码，也解释了
  为什么 `supported_count` 恰恰会在消费者最需要时缺席。这是契约阶段应该贡献而前两份
  文档给不出的东西。
- **`provider_candidates_unavailable` 不可达的判定是对的。** 复核
  `desktop.go:294-310` 确认 `loadProvider` 只在 `OpenReadOnly` 成功的 `else` 分支
  （`:256`）里调用，`:305` 因此在本条件下不可能触发。文档没有把它说成 surface 的
  缺陷，而是说成一条可以少一个成员的抑制列表，分寸准确。
- **C3 把代价放在失败路径上。** 「在锁获取失败处判定」既满足 acceptance item 4，又
  让成功路径零额外开销，并且对探针失败给了保守的回退（返回 nil、维持 `ErrStateBusy`），
  理由写在「A probe that cannot read the version has no standing to overrule the
  lock」一句里。
- **C2 的命名论证是罕见的诚实。** 明说 `supported_version` 在孤立看是更好的名字、在
  这里是更坏的名字，并给出理由（它暗示一个不存在的 `version` 兄弟键）。
- **departures 一节的存在本身。** 三条分歧写在明面上，并各自指定由哪个阶段吸收；
  C2.1 明确标为可单独否决，且写清了否决后 `cli-design.md:2275-2277` 仍必须改。这正是
  本轮能把 ARCH-R1-F1 定为「清单漏了一条」而不是「文档隐瞒了分歧」的原因。
- **兼容性归类可核实。** 「narrowing 属 MINOR」与 `cli-design.md:2222` 的既有裁定
  （「renaming a stable typed error code is MINOR」）一致，并正确指出 doctor 从未对
  本条件发出过 `runtime_error`，因此只加一行而不是两行。

### 📝 总结

被评审内容是 `docs/topics/schema-version-signal/architecture.md` 全文 599 行，处于
progression 的 stage 5（契约）。它把 requirements 留下的三个开放问题全部落地，把
surface 的四项数据请求逐条 provision 或 refuse，并给出六项契约与一份定位到行的
`cli-design.md` 编辑清单。坐标纪律与论证质量都高于本仓库同类文档的平均线，这也是
评分仍有 8 分的原因。

FAIL 的理由集中在同一处结构性弱点：文档对**自己改动的外沿**枚举不全。三条 finding
都是这一点的不同侧面——它枚举了要改的规范段落，却漏掉了被 surface 声明为设计真相的
`prototype/`（ARCH-R1-F1）；它枚举了要改的两条回归用例，却漏掉了另外四条会因同一次
改动变红的既有断言（ARCH-R1-F2）；它为一份新的磁盘契约写了清除规则，却没有覆盖清除
触发永不到来的那一支（ARCH-R1-F3）。三条都不动摇 C1–C6 的任何一项决定，修复面是补写
而不是重决。

残留不确定性两项，均不构成本轮 finding。其一，本轮未执行 Xcode 构建或 Swift 编译，
`supported_count` 的解码只在文档层面核对，实测留给实现阶段——这与 surface 文档的
verification 清单一致。其二，`prototype` 的 dev server 未运行，ARCH-R1-F1 的判定基于
源码读取而非渲染观察；由于该 finding 的实质是「文档漏列」，渲染证据不改变结论。

- **Findings**: ARCH-R1-F1（🔴，开放）、ARCH-R1-F2（🟡，开放）、ARCH-R1-F3（🟡，
  开放）。三条均指向本次评审目标自身的缺陷，按
  `.agent-instructions/review-records.md` 的分类以 FAIL 退回修复，不设外部 carrier。
- **Evidence**:
  - `scripts/check-topic-docs.sh` → exit 0（`architecture.md` 的缺口已解除，
    `tasks.md` 仍是预期中的缺口）。
  - `git rev-parse HEAD` 与 `git hash-object` 四份文档 blob，见 Reviewed state。
  - `git diff --stat 8d283cd..34bf55e -- '*.go' '*.swift'` → 空，坐标基准成立。
  - 三十余处源码与规范坐标逐条读取（见 🟢 第一条）。
  - `rg -n 'unknown_schema' --glob '!docs/**' .` → 8 处命中，构成 ARCH-R1-F1 与
    ARCH-R1-F2 的事实基础。
- **完成门禁**: NOT_VERIFIED。CEv1 查询
  （`MATCH (n:CEv1Node) WHERE n.kind='work_unit' AND n.work_unit_id STARTS WITH
  'schema-version-signal'`）返回两个 WorkUnit——`schema-version-signal:requirements.md`
  与 `schema-version-signal:ux/menubar-schema-signal.md`，均 `status: complete`。
  本文档的 document 边界 `schema-version-signal:architecture.md` 尚未建立，因为该
  边界在评审 `Verdict: PASS` 时才跨越。本轮为 FAIL，故不记录 pass 证据，也不创建
  WorkUnit。

## Round 2 — 2026-09-07（复评）

- **Reviewed state**: HEAD `34bf55ec0d9c0c5d4a4d632a740adcdb2394202a`（未变），文档
  blob `dd891436acbded2ff60e161f454cfffe5e9fd0c2`，599 行增至 709 行。Round 1 的
  基线 blob 为 `e746773f012e5a966e8f94b7ebac734b64bdae5a`。对照物三份未变：
  `requirements.md` `b0854d00`、`ux/menubar-schema-signal.md` `bab2f89c`、
  `tasks.md` `7ea1ce47`（复评时；本轮的状态同步在其后）。
- **Reviewer**: Claude Code，与 Round 1 同一会话（该会话未参与起草，也未参与本轮
  修复——修复由另一会话在 2026-09-07 11:11 完成并记录在
  `ad-svs-doc-arch-design` 的评论里）。角色分离成立：评审者不是修复者。较弱的
  局限同 Round 1：三方同属一个运行时与一套项目指令。
- **Method**: 逐条复核三条 finding 的当前状态，不采信修复自述。对修复新增的每一处
  坐标断言独立取证；对修复新增的规则做回代检验（清除触发、发出门槛、与 C5 既有
  段落和 C6、departure 2、Contract edits、Approval boundary 的一致性）；重跑
  `scripts/check-topic-docs.sh` 与 `make check-whitespace`。评审阶段对生产代码只读。
- **Scope**: `architecture.md` 全文 709 行。不含 `tasks.md`（未写）、不含实现、
  不含 `cli-design.md` 与 `prototype/` 的实际编辑。

📊 综合评分: 9/10

✅ 复评结论: PASS

### 上一轮发现的处置

**ARCH-R1-F1（🔴）closed。** 两处补写，均按 finding 给出的有界改法，未改 prototype
本身：

- 「What existing behavior this changes」新增一项「The prototype's two schema states
  and the row that renders them」，点名 `prototype/src/data.js:439`、`:454` 与
  `prototype/src/Popover.jsx:841`、`:848` 四处坐标，写出它们当前的 `code`、
  `storedVersion` / `supportedVersion` 取值与谓词形态，并引用
  `prototype/README.md:3` 的设计真相声明与 surface 的 prototype-wins 条款。
- 新增小节「What stage 7 must absorb」（`:540`），把 prototype 的码与字段名同步、
  以及 S1 两个宽度两种语言加 `schemaStacked` 的标本重渲染列为第 1 项，并明写
  「This is the only one of the three that invalidates something already
  reviewed」。原先散在 departures 与 refusal 两节的另两项吸收事项一并收进该清单，
  Approval boundary 改为指向它，且新增「the `prototype/` edit」到不被授权的清单。

本轮复核：四处 prototype 坐标逐条读取，与文档所写一致；`rg -n 'two absorptions'`
无命中，旧措辞未残留。

**ARCH-R1-F2（🟡）closed。** 「Changes」的第一项由「两处 `future` 用例」改写为六行
表格，逐条写明今天钉什么、C1 之后变成什么，并补出两条**不变**的断言与理由。本轮逐条
取证，六条与两条全部属实，且修复新增的三处细节坐标也成立：

- `internal/store/store_test.go:213` 确为 `INSERT ... VALUES (1, ?)` 带
  `CurrentSchemaVersion+1`，`:220-221` 为该断言；
- `internal/store/store_test.go:457` 确为同类插入，`:460` 为 `migrate` 断言；
- `internal/doctor/doctor_test.go` 的该用例名确为 `future_schema`（`:163`），
  其 `code:` 在 `:178`；
- 不变的两条：`store_test.go:1071` 属 `migrations.go:366` 的 `missing schema
  metadata`，`:1202`（`TestSchemaVersionRejectsMultipleRows`）属
  `migrations.go:413` 的 `expected one schema version row` ——文档对这两条的归类
  准确，且「变红反而说明 C1 实现过宽」这一用法是把它们当作实现宽度的探针，比单纯
  列出更有用。

「Verification expectations」的 L2 一条已补上 `internal/store` 在改动面内、六条里
有两条在那里；原「the rest extend cases that exist」一句同步改写为「everything else
extends one of the six existing assertions enumerated under **Changes**」。

**ARCH-R1-F3（🟡）closed，且闭合方式强于 finding 要求的最低限。** finding 给了三条
备选，修复取其一并补足另一条：

- **清除条款放宽**（C5「Cleared」）：由「下一次成功的 Hook 投递」改为「第一次成功的
  读写打开」——`store.Open` 无错返回即删除记录，任何写路径命令都清除。并写明只读
  打开者不清除的理由，引用 `docs/specs/cli-design.md:2280`（doctor「never migrates,
  creates, chmods, or otherwise repairs state」）；本轮已核对该行属实。
- **新增发出门槛**：检查只在 `record.stored > store.CurrentSchemaVersion` 时发出。
  这一条才是真正堵死 finding 所述场景的那一半——清除仍是写操作，升级后不再产生任何
  写路径命令的用户永远触发不到它，而发出门槛不依赖任何写操作、不引入新状态、不依赖
  时钟，只用记录里已有的数字做 C1 到处在做的同一个比较。
- **接受的界限明写**，并邀请评审拒绝：升级后损失不再被*呈现*，item 8 由磁盘上的记录
  满足。本轮接受该界限：`requirements.md` item 8 的原文是「recoverable from some
  surface a person or a later reconciliation can read」，并明说「Which surface
  carries it is a design decision」。

回代检验通过：`stored > CurrentSchemaVersion` 成立时 `OpenReadOnly` 必然失败，因此
C5 原有的「检查置于数据库打开之前、短路路径是它唯一有话可说的路径」在新门槛下依然
成立，两段不矛盾。C6 末段、departure 2、Contract edits 的 doctor 一行、Verification
的 item 8 四项测试（含「不再超前时**不**发出」这条负向断言）都已随之更新，没有留下
指向旧规则的段落。

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

**两处措辞被考虑并判定为不构成 finding，记在此处以便审计而非留作暗账**：

1. Contract edits 的 doctor 一行写「the two triggers that clear that file」，而 C5
   给出的是一条规则（成功的读写 `store.Open`）。可成立的读法是「Hook 处理器的成功
   打开」与「其他写路径命令的成功打开」两个触发点，而该行的作用正是要求
   `cli-design.md` 别把清除写成 Hook 专属。实质契约在 C5 里无歧义，实施者据 C5 落笔
   不会写错，故判为措辞而非缺陷。
2. Approval boundary 写「the `prototype/` edit named below」，而
   「What stage 7 must absorb」在其**上方**（`:540` 对 `:701`）。方向词有误，但紧接
   的下一句以小节名精确点名了它，读者不会找不到。同上，判为措辞而非缺陷。

按 `.agent-instructions/review-records.md` 与工作流 Skill 的取舍规则，「不是发现」与
「是发现但跳过」是两种判断，本轮取前者，并把理由写下来供后续轮次复核。

### 🟢 优点

- **修复没有越界。** 三条 finding 都只补写文档，未改 `prototype/`、未改测试、未动
  C1–C6 的任何一项决定。ARCH-R1-F1 的有界改法说「不需要在本文档里改 prototype」，
  修复照办并在 Approval boundary 里显式声明该编辑未被授权。
- **ARCH-R1-F2 的闭合超出了「补四条」。** 它把两条**不变**的断言也写了进来，并给出
  它们的用途：变红即说明 C1 实现过宽。一个把「不该变的东西」也钉住的清单，比只列
  「要变的东西」的清单更能防止实现漂移。
- **ARCH-R1-F3 的闭合选了更强的那条路。** finding 列了三条备选，修复取「第二清除
  条件」并额外补上发出门槛；后者才是与写操作解耦的那一半，也是唯一能在「永不再有
  写路径命令」这一支上成立的。附带的负向测试（不再超前时不发出）把这条规则钉在了
  验证清单里。
- **新增的所有权边界是有价值的额外契约。** C5 的「Owned by one reader/writer」把写、
  清、读三个调用点收进一个所有者，理由直接引用本文档自己发现的教训——`unknown_schema`
  的两半正是因为形状被复制而漂移的。这是修复轮自己想到的，不在任何 finding 里。
- **坐标纪律在增量上保持。** 修复新增的每一处坐标（prototype 四处、六条断言加三处
  插入点、`future_schema` 用例名、`cli-design.md:2280`）本轮均独立取证，无一处漂移。

### 📝 总结

被复评内容是 `docs/topics/schema-version-signal/architecture.md` 修复后的 709 行。
Round 1 的三条 finding 全部闭合，且闭合方式与 finding 给出的有界改法一致或更强；未
发现回归，未发现修复引入的新缺陷。Round 1 的诊断是「文档对自己改动的外沿枚举不全」，
本轮确认外沿已补齐：设计真相 `prototype/` 进了 Changes 与 stage 7 清单，六条既有断言
（外加两条刻意不变的）进了表格，新磁盘契约的清除与发出两条规则合起来覆盖了原本没有
覆盖的那一支。

残留不确定性两项，均不构成 finding。其一，本轮未运行 prototype 的 dev server、未执行
Xcode 构建，`prototype/` 的判定基于源码读取——由于该 finding 的实质是「文档漏列」，
渲染证据不改变结论。其二，`cli-design.md` 与 `prototype/` 的实际编辑属 stage 7 与
实现阶段，本文档只授权契约本身，编辑质量将由各自阶段承接。

- **Findings**: 本轮无新发现。Round 1 的 ARCH-R1-F1、ARCH-R1-F2、ARCH-R1-F3 均
  **closed**，处置见上。无 carrier 需要设置。
- **Evidence**:
  - `git hash-object` 四份文档 blob，见 Reviewed state。
  - `scripts/check-topic-docs.sh` → exit 0；`make check-whitespace` → exit 0。
  - 修复新增坐标逐条读取：`prototype/src/data.js:439`、`:454`、
    `prototype/src/Popover.jsx:841`、`:848`、`prototype/README.md:3`；
    `store_test.go:213`、`:220-221`、`:457`、`:460`、`:1071`、`:1202`；
    `doctor_test.go:163`（`future_schema`）、`:178`、`:334`、`:349`；
    `main_test.go:1461`；`migrations.go:366`、`:413`；`cli-design.md:2280`。
  - `rg -n 'two absorptions' docs/topics/schema-version-signal/architecture.md`
    → 无命中，旧措辞未残留。
- **完成门禁**: VERIFIED（`document` 边界，绑定本轮状态同步后的候选内容态）。见下方
  门禁同步小节。

### Round 2 门禁同步

本轮触发的状态同步与证据记录，按 `.agent-instructions/evidence.md`「Record after the
round's own status synchronization, not before」的次序执行：先把 `tasks.md` 的
`architecture.md` Review 列勾选并更新其说明段落，再按同步后的 blob 计算内容态并写入
CEv1，最后复查门禁。

- WorkUnit：`schema-version-signal:architecture.md`，`unit_kind: document`。
- 内容态：HEAD `34bf55e` 加 `architecture.md`、`tasks.md`、本记录三份 blob 的
  scoped 指纹；具体取值记在 CEv1 的 `content_state` 节点上。
- 门禁复查结果：四条必需 criterion 各有一条 `outcome: pass` 证据，全部 `VERIFIED`。
