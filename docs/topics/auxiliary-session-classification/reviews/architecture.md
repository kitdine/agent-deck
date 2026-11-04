---
status: active
topic: auxiliary-session-classification
subject: architecture.md
---

# 架构评审

## Round 1 — 2026-10-27

## 📋 分类架构与数据合同评审

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-ARCH-R1-F1 — P2 — 未匹配/已失去会话的覆盖没有可执行的撤销合同（OPEN）**

位置：architecture.md:139–150、197–202、216；关联 131–135 行及 ux/settings.md:69–72。

- 行为风险：恢复后 unmatched、原会话已不存在的覆盖仍会出现在 Settings 列表，但撤销动作可能被可见会话查找拒绝，形成不能清除的用户记录。实现者必须擅自放宽 not-found 规则或新增协议，才能满足已批准 UX。
- 证据：Settings 明确“原始会话已不存在或恢复后尚未匹配……允许删除该覆盖记录”，不是要求删除源会话。架构也要求保留旧覆盖供逐条撤销，但写入流程要求查询目标后 CAS，并统一规定“目标不可见沿用 session_not_found”。协议只定义按 session ID 的 classify --clear 和 classify list，没有说明 --clear 可独立定位已有覆盖、无需可见会话，或另给覆盖记录撤销 API；Settings 必须通过这些 Go 命令操作。
- 判别场景：core 存有 codex/archived-1 的 auxiliary 覆盖（revision=4、unmatched），sessions 无该 canonical 会话。classify list 能返回该覆盖；Settings 选择撤销。若按 §5 的可见目标规则处理，则 session_not_found；若直接以覆盖键写 tombstone，则绕过了未写明例外的统一规则。当前架构无法唯一决定合法行为、返回 effective/automatic 是否 unavailable，以及重复撤销的 no_change。
- 💡 修复建议：明确把“查询/设置可见会话分类”和“撤销已有用户覆盖记录”分成不同的目标解析规则，可复用 --clear 但必须写清例外。规定 unmatched/会话消失时按覆盖记录身份及 expected_revision/request_id 做 CAS/tombstone；不读回或暴露被 exclude 的源会话内容，不把无会话伪造成自动分类结果。明确不存在覆盖时的幂等结果、并发变化与请求读回语义，并补充 Settings/CLI 对应验收例。无需在本轮实现代码。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 用户持久覆盖与自动派生分开，回执、CAS、提交后权限错误和发布待完成有独立语义。
- 维护适配器限制版本、列白名单和只读边界，不把来源目录/模型名当作用途证据。
- 明确跨数据库读向量、epoch 与 token mismatch 的保守退化；保留旧 wire 字段及 App Group 聚合隐私边界。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与本架构设计。
- Method: 单评审者全文合同与四份已评审 UX 对照；CodeGraph 定位现有 SetSettings/backup 等路径，再核对相关当前源码。对未匹配覆盖构造最小合同反例；决定性阻塞后停止广泛验证，未运行产品测试。
- Scope: architecture.md 的设计决策及需求/UX 可实现性，不是产品实现或实际 observer 数据验收。未修改产品、测试、配置、原型或架构正文。
- Reviewed state: HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob `7b586840ec6cf60a7a37493441a371d0a333324e`。
- Fingerprint: `fbe4dda0c252bf9de18a04f1b2922485a67955b57a88176c09a9fcfb77349fe2`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: settings.md 的未匹配覆盖删除要求与 architecture §5/§7 直接对照；上述反例是合同演绎，不冒充未实现命令的运行输出。原型中的 archived-1 只是说明场景，不是用户数据。
- Evidence: 当前 SetSettings 在 commit 后执行 secureFiles，可能返回已持久化的权限错误；backup 通过 snapshot API 并对空目标恢复，与文档该部分边界一致。只读核验安装包 worker-service.cjs/SessionStore.js 的 SHA-256，分别匹配文档 `aed0b798e04c40e2c3600e2e5976f4eb470b84c96bc1ea48fea9201157a556f4` / `4ed2bf28cfc01b37589e92f2ace8f243f25dbef0aa84df07d5d1b4140e1907c7`；未访问真实日志、数据库或凭据。哈希一致不证明实际 SDK observer 启用、映射命中或覆盖率。
- Evidence: 记录及矩阵同步后 `python3 scripts/check-docs.py` 通过（包括空白、diff、本地链接、记录结构及文档集检查）。文档集成稿不等于 tasks 分解或 final UX 已通过。
- WorkUnit: `auxiliary-session-classification:architecture.md`；ContentState: `auxiliary-session-classification:arch-r1-fbe4dda0c252bf9d`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-ARCH-R1-F1 为 fail；文档完整性按实际检查结果记录，不因语义 FAIL 虚构结构失败。
- Residual uncertainty: 其余分类/适配器/并发/备份实现仍需独立隔离 fixture 与最终内容验证。本轮停止于确定缺口，不作全部架构细节无缺陷声明。

修复：auxiliary-session-classification / reviews/architecture.md / ASC-ARCH-R1-F1

## Repair candidate — 2026-10-27（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-ARCH-R1-F1，未委派，未修改产品、测试、配置或 UX 标本。
- ASC-ARCH-R1-F1 → repaired in candidate，待独立复评确认关闭。§5 分开查询/--set 的可见会话解析
  与 --clear 的 core 覆盖键解析；§7 明确复用 --clear 的例外及列表提供 revision，不引入源读取权限。
- --clear 要求明确 client/session_id，可处理 unmatched、源消失、purge-index 后或已 exclude 的目标。
  有效覆盖写 tombstone 并加版本；已有 tombstone 或从未存在记录在 CAS 成功后 no_change。
  core 读取失败不等于没有覆盖；新请求先 CAS 再判幂等；相同请求重放原回执，不删除其后新建的覆盖。
- clear 回执 automatic/effective=null、classification_available=false、reason=not_queried，
  不伪造无会话的自动分类。Settings 另行查询仍可见会话或刷新覆盖列表，旧回执不能覆盖更高版本。
  未确认提交仍按 request_id 读回，不以 session_not_found 推断撤销成功。
- 判别场景推演：archived-1/unmatched/revision4 且 sessions 无记录，clear expected4/requestA
  得到 tombstone5；新 requestB/expected5 为 no_change5；他人 set6 后重放 requestA 仅返回原5回执，
  不清除6。另列不存在键、exclude、并发修改和提交结果未知，共七条 Settings/CLI 验收案例。
  这是合同一致性推演，不是未实现命令的运行测试。
- Candidate identity：HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob
  `5b5a3b5694827900a9380e3879058a6baca4919a`。
- 验证：与 Settings 的“未匹配覆盖可逐条撤销”要求对照，并核对全文 --clear、session_not_found、
  no_change、请求读回及 UI 版本约束的一致性。`python3 scripts/check-docs.py` 通过；未运行产品测试。
- 原 Round 1 FAIL/OPEN 和原目标 FAILED 保留，新候选不自发声明 Review PASS 或 CEv1 VERIFIED。
  tasks.md 仅记录待复评，最终 UX 核对仍等待架构独立评审通过。

## Round 2 — 2026-10-27

## 📋 架构修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-ARCH-R1-F1 — CLOSED**：§5 区分可见会话查询/设置与 core 覆盖撤销。
--clear 要求明确 client/session_id，以 core 记录或 tombstone 为目标，无 sessions/源日志前置条件；
§7 的命令例外、列表键/revision 和 §9 七条验收例一致，Settings 的 unmatched 删除要求已有协议落点。

### 📝 总结

- Reviewer: Codex，原独立评审会话，未参与修复。
- Method: 单评审者逐 finding 核对 §5/§7/§9、原 Settings 要求和原反例；不委派、不执行未实现命令，不修改产品、测试、配置或 UX。
- Scope: ASC-ARCH-R1-F1 及其身份解析、CAS、幂等/回执、错误和 UI 更新语义。未变更的源码事实沿用 Round 1 核对，HEAD 未变且无产品脏文件；不将本轮文档复评当作实现验收。
- Reviewed state: HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob `5b5a3b5694827900a9380e3879058a6baca4919a`。
- Fingerprint: `54920214c6ee71310902644ff66ff9a4ed1a02e2a433c57a6d4b714b998d225b`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: 原 archived-1/unmatched/revision4、sessions 无记录场景，现在以 clear expected4/requestA 写 tombstone5；automatic/effective=null、classification_available=false/not_queried，不伪造自动分类。被 exclude 的源同样可撤销用户记录，但不解除 exclude 或读取私有内容。
- Evidence: 新请求 expected5 重复 clear 得 no_change5；从未存在键 expected0 得 no_change0；core 读取错误不等于不存在。读取后记录改到5时 expected4 冲突，即使目标已是 tombstone 也先 CAS。旧 requestA 在他人 set6 后仅重放原5回执、不改6；UI 也不能用旧回执抹掉已知高版本。这些规则形成一致的合同链。
- Evidence: 提交结果未知按 request_id 查证，未确认前不删除列表项；无会话时保持分类不可用，可见会话需另行查询当前 automatic/effective。以上是设计合同推演，不是运行测试；七场景须落实为后续 L2/L3 证据。
- WorkUnit: `auxiliary-session-classification:architecture.md`；ContentState: `auxiliary-session-classification:arch-r2-54920214c6ee7131`。
- Completion gate: VERIFIED
- Gate scope: 本架构文档的独立复评和完整性检查；不认证维护适配器实际覆盖、跨库一致性实现、迁移/恢复或性能。
- Hierarchy: 架构通过后四份 UX 仍需 final-surface reconciliation，tasks.md 尚未分解/评审；本轮不跨越主题完成边界，不创建实现任务。
- Evidence: 记录与矩阵同步后的 `python3 scripts/check-docs.py` 通过，涵盖空白、diff、本地链接、评审记录及文档集合结构；不等于实现分解或最终 UX 的语义通过。
- Disposition: ASC-ARCH-R1-F1 CLOSED；旧 Round 1 FAIL 和旧目标 FAILED 保留，当前结果只绑定新候选。
- Residual uncertainty: 真实维护格式的隔离端到端验收、原生交互/发布和最终 scan-performance 集成仍未执行，必须由实现及其证据门覆盖。

Task checkpoint：ad-auxiliary-session-classification-doc-arch；content_state `54920214c6ee71310902644ff66ff9a4ed1a02e2a433c57a6d4b714b998d225b`；gate VERIFIED。
提交建议：经授权提交 architecture.md、对应评审记录及 tasks.md 的架构状态；不包含产品实现或最终 UX 修改。
推送建议：经授权提交并核验提交对象、Codex trailer 与 SSH 签名后，推送 feature/auxiliary-session-classification；remote/upstream 尚未解析。

设计：auxiliary-session-classification / ux/cli.md（final-surface reconciliation）

## Round 3 — 2026-11-03

## 📋 架构集成修订评审（批量评审：所有文档）

📊 总体评分: 6/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-ARCH-R3-F1**（P1）[architecture.md:127] §3.4 规则把支持表收窄为单一条目，并误称这是 scan-performance 登记的全部条目。
- 行为风险: `internal/telemetry/reconcile.go:57-60` 的 `supportTable` 有两条已实验验证的条目：
  `codex_exec/0.165.0/codex_exec/sse` 与 `codex-app-server/0.165.0/codex-tui/sse`（`SupportEntry.Name()` 形如
  producer/version/originator/transport）。按当前文字，TUI 会话（含标题生成）的带标记后台会话将得到 unknown/`origin_rule_unsupported`
  并留在 Work，与 scan-performance requirements 第 186–187 行“带标记会话同属辅助会话”及 Beads
  `ad-auxiliary-session-classification-origin-adapter-reducer` 2026-10-29 接口请求（明确包含 TUI thread title generation）相矛盾，AC1 的目的落空。
- 证据: reconcile.go 第 50–55 行注释记录 2026-10-29 隔离实验：两种组合下后台请求（Memories 整理、TUI 标题生成）都在无 rollout 的会话中运行。
💡 修复: 以 `SupportEntry.Name()` 的精确字符串列出首版规则覆盖的条目，至少纳入上述两条；如有意排除某条，写明依据并取得用户决定。
同步 tasks.md 任务 2 验收。

**ASC-ARCH-R3-F2**（P1）[architecture.md:42, 125-133] 规则所需字段无法从声明的接口取得。
- 行为风险: §3.4 依赖 `support_entry` 白名单和以 `support_entry`+`settled_at` 作 source token，tasks.md 任务 2 写明“经
  `store.TelemetrySessionSourceKinds` 批量读取”；但该函数（`internal/store/telemetry.go:550-575`）只返回 `session_id → source_kind`，
  不含 support_entry 或 settled_at。实现者必须自行发明读取合同。
💡 修复: 定义只读 store 接口（返回 source_kind、support_entry、settled_at，沿用 200 一批、只读、不建库），说明它与 scan-performance
已交付接口的关系（扩展还是新增）以及归属任务；同步 §2 表格与 tasks.md 任务 1/2。

### 🟡 建议改进 — 推荐

以下同样是本轮开放发现，PASS 前必须关闭。

**ASC-ARCH-R3-F3**（P2）[architecture.md:44] §2 仍写“本工作树为 core schema 30”。
- 证据: 工作树已合入 `660c815`，`internal/store/store.go:26` 为 `CurrentSchemaVersion = 33`；与同文件第 26–30 行“main 能力已存在于本工作树”的说明矛盾。
💡 修复: 改为当前工作树 33，并保留“实施时按 main 清单分配下一个号”。

**ASC-ARCH-R3-F4**（P2）[architecture.md:103, 127] 新 reason_code 没有消费者合同。
- 行为风险: 架构只在 §3.4 命名 `codex_otel_background`，未定义 reason_code 枚举与未知码回退；菜单栏原型按码映射文案
  （`prototype/src/AuxiliarySessions.jsx:54` 仅有 task_child_unconfirmed/origin_missing/maintenance_origin），tasks.md 第 30 行却断言菜单栏/Widget
  “只消费通用原因码、无影响”。实现时菜单栏详情对新码的展示无合同可依。
💡 修复: 在 §3.3 定义 reason_code 枚举（含 claude-mem 规则的码与本码）及消费者对未知码的回退；或明确复用 `maintenance_origin` 由 rule_version 区分。
据此给出菜单栏影响结论。

**ASC-ARCH-R3-F5**（P2）[architecture.md:121-124, 283-285] 来源状态的读取合同缺失，内建来源行使其显性化。
- 行为风险: Settings 现须展示两行来源状态（claude-mem 与内建 Codex OTel），内建行要区分“core 可读/不可读”和“尚未开启 OTel 采集（无后台会话记录）”；
  §7 只写“Settings 通过 Go JSON 命令读/写”，没有任何命令或 DTO 提供来源状态（CLI 只有 origin-source set/disable）。§3.4 又声明不读 OTel 配置，
  “未开启 OTel”与“已开启但无记录”的判定来源未定义。
💡 修复: 定义来源状态读取命令/DTO（字段白名单、两类来源、安全 reason 枚举，含 OTel 未开启 vs 无记录的判定来源与是否读取 telemetry 状态），并与 ux/cli.md、ux/settings.md 对齐。

### 🟢 优点

- serving generation 复用的源事实核实无误：`internal/store/serving_generation.go:34` core 触发表 = derivedSnapshotGenerationTables + settings、quota_envelopes、
  domain_coverage_heads、source_domain_checkpoints；sessions 侧第 47 行含 session_sources/metadata/exclusions；读者 200 ms 截止（第 106 行）；
  `usage_derived_invalidations(kind,client,key)` 与分区键 (client, model, day) 见 `usage_partitions.go:25-33, 56-75`。
- 放弃自建 scope_data_revision、origin_mutations 不装 trigger 以保持 no_change 不发布、为 telemetry_sessions 显式补 trigger 而不依赖“恰好同事务”，
  都是有理由的结构性选择。
- `ServingWireVersion = 2` 只是 serving 头部；嵌套 payload 仍为 wire v1（`internal/desktop/serving_cache.go:23-58`），与本文“wire v1 旧字段不变”一致。

### 📝 总结

- Reviewer: Claude Code（claude-code），未参与修订的独立评审会话。
- Method: 单评审者；读修订 diff 与相关章节，逐条对照 store/telemetry/desktop 源码与 scan-performance 文档和 Beads 接口请求；不委派，不改产品代码、测试或配置。
- Scope: architecture.md 全文，重点 §1–§4、§6–§7、§9；依赖 requirements Round 3 PASS。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；architecture blob `b68b6a86f9b6ac102b69e196de031acccb5115aa`（未提交）。
- Fingerprint: `a26986c3a3736b1b77b30256be891f4bfbfcee34cafed1faa53980b895000c3c`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过、`scripts/check-topic-docs.sh` exit 0、`scripts/check-whitespace.sh` 无输出；结构通过不抵消上述语义发现。
- Disposition: ASC-ARCH-R3-F1…F5 全部开放，修复范围即本轮。
- WorkUnit: `auxiliary-session-classification:architecture.md`；本内容状态未记录 pass 证据。
- Completion gate: NOT_VERIFIED
- Residual uncertainty: 分区 payload 内 scoped membership 的实际写放大只能在实现与任务 9 测得。

## Repair candidate — 2026-11-04（Round 3 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-ARCH-R3-F1…F5，未委派；未改产品代码、测试或配置。
- ASC-ARCH-R3-F1 → repaired in candidate。§3.4 以 `SupportEntry.Name()` 精确列出支持表全部两条（`codex_exec/0.165.0/codex_exec/sse`、`codex-app-server/0.165.0/codex-tui/sse`）及各自后台请求，引用 reconcile.go 注释与 scan-performance 支持表实验；§2 新增 reconcile.go/config.go 事实行；§9 验收分条目覆盖。
- ASC-ARCH-R3-F2 → repaired in candidate。新增只读 `TelemetrySessionOriginMarkers(ctx, client, ids)`（source_kind/support_entry/settled_at，200 一批、空候选不查、不建表不写行）与 `TelemetryBackgroundSessionCount(ctx)`；既有 `TelemetrySessionSourceKinds` 不改；归属任务 1，规则调用归任务 2；§2 表格同步。
- ASC-ARCH-R3-F3 → repaired in candidate。§2 与 §4 改为当前工作树/main 均为 schema 33（store.go:26）。
- ASC-ARCH-R3-F4 → repaired in candidate。§3.3 定义七个分类 reason_code 封闭枚举与未知码回退；Codex 来源复用 `maintenance_origin`、由 rule_version 区分（原型 `maint-2` 已为 Codex maintenance_origin）；“无证据”码按 CLI JSON 与 Settings 标本取 `origin_metadata_missing`，菜单栏标本内部键 `origin_missing` 由任务 5 映射。菜单栏 UX 文档无需修改，Widget 不受影响。
- ASC-ARCH-R3-F5 → repaired in candidate。§7 新增只读 `session origin-source status`（`session.origin_source.status`）：两项固定顺序、字段白名单、claude-mem 六种与 Codex 三种无记录及 core_unavailable 的 reason 枚举、判定顺序、单项失败 exit0+partial；接收器开关只经 `telemetry.LoadConfig` 读 enabled（只读、缺文件即 disabled），§3.4 明确分类不读它；§4 新增 core `origin_adapter_runs` 摘要作为 claude-mem 状态来源（派生、不进备份）。
- Candidate identity：HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；architecture blob `d7c63db9b0933e7641564ff0a9711b31c7318d2f`。
- 验证：`npm run build` 通过；auxiliary CLI/Settings/Widget final 契约脚本 exit0（14/14/39 项）；浏览器 Settings 脚本 70 项、菜单栏 176 项 0 失败；Settings 五种 Codex 行状态逐一渲染核对。`python3 scripts/check-docs.py`、`scripts/check-topic-docs.sh`、`scripts/check-whitespace.sh` 见修复交接。未运行产品测试（无产品代码变更）。
- 原 FAIL/OPEN 与 NOT_VERIFIED 保留；本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 4 — 2026-11-04

## 📋 架构集成修复复评

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

以下为本轮新增开放发现，PASS 前必须关闭。

**ASC-ARCH-R4-F1**（P3）[architecture.md:330, 349-352] §7 来源状态的 `available` 与 `partial` 语义各有一处未定。
- 行为风险: claude-mem 的 `available` 只由“最近一次 origin_adapter_runs.outcome 为 ok/partial”决定。`disable` 使自动证据失效（ux/cli.md 命令表），
  但没有规定要清除运行摘要，所以已停用或本机配置不可读的来源仍可能输出 available=true，同时 reason 为 `source_disabled`/`source_config_unreadable`。
  这样 CLI 文本中的 `available` 和 Settings 的“元数据：可用”会把一个已不参与分类的来源显示为可用。
  另外，receiver.json 读取或解析失败（`otel_receiver_unknown`）是否算作“单项读取失败”、是否要把 envelope 置为 partial=true，也没有写明。
💡 修复: 写明 available 在 not_configured/disabled/config_unreadable/core_unavailable 下各自的取值（建议仅当配置启用且最近一次运行为 ok/partial 时才为 true），
以及 receiver=unknown 是否置 partial；同步 tasks.md 任务 4 的 status 验收。

### 🟢 优点

- **ASC-ARCH-R3-F1 — CLOSED**：§3.4 按 `SupportEntry.Name()` 精确列出两条条目，与 `internal/telemetry/reconcile.go:57-60` 一致；§9 codex-otel-source 按条目分别验收。
- **ASC-ARCH-R3-F2 — CLOSED**：新增只读 `TelemetrySessionOriginMarkers`，返回 source_kind/support_entry/settled_at；既有 `TelemetrySessionSourceKinds` 不改；接口归任务 1，调用归任务 2。
- **ASC-ARCH-R3-F3 — CLOSED**：§2/§4 改为 schema 33，与 `internal/store/store.go:26` 一致。
- **ASC-ARCH-R3-F4 — CLOSED**：§3.3 定义七个原因码的封闭枚举与未知码回退；Codex 来源复用 `maintenance_origin`，由 rule_version 区分。原型 `maint-2`（codex，maintenance_origin）与菜单栏既有文案相符，`origin_missing`→`origin_metadata_missing` 的映射归任务 5。
- **ASC-ARCH-R3-F5 — CLOSED**：§7 新增只读 `session origin-source status`，给出字段白名单、两套 reason 枚举与判定顺序；接收器开关经 `telemetry.LoadConfig`（`internal/telemetry/config.go:56`，只读，缺文件即 disabled）取得，§3.4 声明分类不读它。
- origin_adapter_runs 属于 §8 所说的“新派生表”，其行在 backup 的临时快照中清除，与 `internal/backup/backup.go` 现有在快照上清除本机表的做法同类。

### 📝 总结

- Reviewer: Claude Code（claude-code），Round 3 评审会话；未参与修复（修复由另一个 Claude Code 会话完成）。
- Method: 单评审者；读修复 diff，逐条对照 reconcile.go、config.go、telemetry.go、store.go、backup.go 与原型；不委派，不改产品代码、测试或配置。
- Scope: ASC-ARCH-R3-F1…F5，以及修复新增的 §3.3 枚举、§3.4、§4 origin_adapter_runs、§7 status 合同和 §9 三行验收。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；architecture blob `d7c63db9b0933e7641564ff0a9711b31c7318d2f`（未提交）。
- Fingerprint: `4ecc172e791c836905e7293e40e9702d3b3d48f9c67033373ff019558f2b3376`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`bash scripts/check-topic-docs.sh` exit 0；`bash scripts/check-whitespace.sh` exit 0。结构检查通过不能抵消上述语义发现。
- Disposition: ASC-ARCH-R3-F1…F5 CLOSED；ASC-ARCH-R4-F1 OPEN。
- WorkUnit: `auxiliary-session-classification:architecture.md`；本内容状态未记录 pass 证据。
- Completion gate: NOT_VERIFIED

## Repair candidate — 2026-11-04（Round 4 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-ARCH-R4-F1，未委派。
- ASC-ARCH-R4-F1 → repaired in candidate。§7 字段表：claude-mem available 仅在配置可读、已配置且启用、core 可读且最近运行 ok/partial 时为 true；原因表后逐一写明七个原因下 available 均为 false、reason=null 为 true；disable 不删运行摘要但 available 由配置判为 false。partial 只在三种读取失败时为 true：core 不可读、来源配置不可读、receiver.json 不可读或无效（receiver=unknown，无论有无记录）；其余已确认状态 partial=false，均 exit0。§9 source-status 验收同步。
- 影响核对：CLI 引用 §7 字段与枚举、Settings 按 available 显示“元数据可用/不可用”（sourceUnconfigured 等已为不可用），二者文本无需修改。
- Candidate identity：HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；architecture blob `416d63c922b9ea445a53a9774388efa13776af1c`。
- 验证：`python3 scripts/check-docs.py`、`scripts/check-topic-docs.sh auxiliary-session-classification`、`scripts/check-whitespace.sh` 均 exit0；未改原型或产品代码，未运行产品测试。
- 原 FAIL/OPEN 与 NOT_VERIFIED 保留；本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 5 — 2026-11-04

## 📋 架构 §7 来源状态语义复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-ARCH-R4-F1 — CLOSED**：
  - available 的取值已经写定。claude-mem 只有在配置可读、已配置且启用、core 可读，并且最近一次运行为 ok/partial 时才为 true；七个原因下都为 false，reason=null 时为 true。停用后残留的运行摘要也明确判为 false。
  - partial 的触发条件也写定了，恰好三种读取失败：core 不可读、来源配置不可读、receiver.json 读取或解析失败（无论是否有记录）。其余已确认状态都是 partial=false，所有情形都 exit0。
  - §9 source-status 的验收已同步更新。
- 判定顺序与 available、partial 的规则一致。配置不可读和 core 不可读同时出现时，reason 取先判定的 source_config_unreadable，partial 仍为 true；Codex 有记录但 receiver=unknown 时，reason=null、available=true、partial=true。没有自相矛盾的组合。

### 📝 总结

- Reviewer: Claude Code（claude-code），Round 3/4 评审会话；未参与修复。
- Method: 单评审者；读 §7 字段表、原因表、判定顺序与 partial 段落，逐一组合推演，并与 §9 验收对照；不委派。
- Scope: ASC-ARCH-R4-F1；其余章节相对 Round 4 未变。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；architecture blob `416d63c922b9ea445a53a9774388efa13776af1c`（未提交）。
- Fingerprint: `4299e9eae33f0e151fa5b6a4169864c5486da5eb9d3bcec2662d91585ff18a33`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`bash scripts/check-topic-docs.sh` exit 0；`bash scripts/check-whitespace.sh` exit 0。
- Downstream impact: 本轮修复把 receiver.json 读取失败定为“单项读取失败”，但 Codex 项在这种情况下 available 仍为 true。ux/cli.md:211 写的是“单项读取失败只置该项 available=false”，与之冲突。冲突应在 CLI 一侧修正，本文无需改动，已记为 ASC-TASKS-R5-F1。
- Disposition: ASC-ARCH-R4-F1 CLOSED。
- WorkUnit: `auxiliary-session-classification:architecture.md`；ContentState: `auxiliary-session-classification:arch-r5-4299e9eae33f0e15`。
- Completion gate: VERIFIED
- Gate scope: 本架构文档的独立复评与完整性检查；不认证实现。
