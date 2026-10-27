---
status: active
topic: auxiliary-session-classification
subject: tasks.md
---

# 实现分解与文档集评审

## Round 1 — 2026-10-27

## 📋 tasks.md 分解与文档集评审

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-TASKS-R1-F1 — P2 — Widget 按窗口/客户端的默认范围 summary 没有生产任务与文件边界（OPEN）**

位置：tasks.md 任务 5 `desktop-menubar`（范围 `internal/desktop/desktop.go` 与 Popover）、任务 7 `widget-summary`
（范围仅 AgentDeckShared/AgentDeckWidget 的 Swift 文件）；关联 architecture.md §7（App Group `session_scope_summary`，
“Magnitude small/large 消费各自窗口的 summary”）与 ux/widget.md:167（宿主把 CLI/core 选择投影转换为白名单 summary）。

- 行为风险：实现者无法从计划得知 Widget summary 的数据从哪里来。任务 5 只交付 Popover 的单一 `session_scope_view`
  （架构 §7：selection 的 scope/client/period 为一次选择，且是菜单栏临时范围）；任务 7 只写 Swift codec/发布/渲染，
  文件边界不含 `internal/desktop/desktop.go` 或 `DesktopWire.swift`。Widget 需要的是“已确认持久默认范围”在每个
  client × 窗口（small=today、large=30d）上的聚合，而不是 Popover 当前选择；若沿用单一 view，会把临时范围或错误窗口
  的数字发布给 Widget，正是 ux/widget.md 禁止的情况。不定义来源，任务 7 必须自行发明跨任务合同。
- 证据：现有 App Group 快照由 desktop wire 信封转换（AppGroupSnapshotStore.swift:26 `AppGroupSessionsSnapshotV1(envelope.data.sessions)`）；
  Widget 按 entry client 从 `usage.presentation.scopes` 取对应客户端（WidgetDomain.swift:162），各期 totals 带 `sessions`
  （DesktopWire.swift:394）。也就是说每个 client × period 的数字今天都来自 Go desktop 快照的多 scope 投影，新 summary
  同样需要 Go 侧按确认默认为每个组合生产聚合，或宿主按组合发起多次选择请求；计划两者都未指定，也未分配到任何任务。
- 💡 修复建议：在任务 5 或 7 中明确 Widget summary 的生产路径与所有者——例如 desktop 快照为每个受支持 client × Widget 窗口
  按已确认默认（未确认时不生成）additive 输出白名单聚合，并把 `internal/desktop/desktop.go`、`DesktopWire.swift`、相应 Go/Swift
  测试纳入该任务边界，写明与 Popover 临时 selection 的隔离和依赖顺序；若这需要新的 wire 合同而架构未定，先回到 architecture 决定，
  不在计划中虚构字段。

**ASC-TASKS-R1-F2 — P3 — 状态陈述称 Widget 最终候选“尚待授权交付”，与已交付事实不符（OPEN）**

位置：tasks.md:29。

- 行为风险：tasks.md 是主题状态权威；该句让读者以为 Widget 最终表面尚未提交，可能重复交付或误判开发前置条件。
- 证据：Widget 最终表面已提交于 `6818c682f96eaddcdab227ee96a0609bebbda4b9`（当前 HEAD），其 Beads 任务已关闭，
  CEv1 已交付状态 `auxiliary-session-classification:widget-final-delivered-76b083a2b7e3fdb5` 门禁 VERIFIED；
  同文件分解输入表所列 `ux/widget.md` blob `0e764dbe…` 正是 HEAD 中的已提交内容。
- 💡 修复建议：改为陈述四份 UX 最终表面均已提交（菜单栏 `27498c0`、Settings `8cb1a7d`、Widget `6818c68`，CLI `df75ce1`），或删除该句。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 文档集声明七份、无占位；`bash scripts/check-topic-docs.sh auxiliary-session-classification` 与 `python3 scripts/check-docs.py` 均退出 0。
- 分解输入表六个 blob 与 HEAD 中对应文件逐一相同（`git rev-parse HEAD:<path>` 与工作树 `git hash-object` 一致）。
- 任务 1–9 列出的现有 Go/Swift 文件与 `scripts/run-go-test.sh` 均存在；AC1–AC8 均有主任务与交叉证据映射，
  AC1 拒绝以全 unknown 代替真实格式适配器，AC8 正确依赖 scan-performance 最终集成而不复制未稳定预算。
- 任务 1/4/6 准确承接架构 §5 的“同 ID 回执 → CAS → no_change”顺序、clear 的 not_queried 回执与 ASC-SET-R3-F1 恢复语义。

### 📝 总结

- Reviewer: Claude Code，独立评审会话，未参与分解编写。
- Method: 单评审者；只读核对 tasks.md 与六份输入、architecture §4–8、documentation-workflow Status/文档集规则；
  核对输入 blob、文件路径存在性、AC 映射；追踪现有 Widget 数据来源（AppGroupSnapshotStore、WidgetDomain、DesktopWire）以检验任务 5/7 边界。
- Scope: tasks.md 文档集完整性与实现分解；不重判已通过的六份文档，不执行实现。
- Reviewed state: HEAD `6818c682f96eaddcdab227ee96a0609bebbda4b9`；document blob `8e1fcb437520135c90c809cdbe628ec723480f8e`。
- Fingerprint: `9f2c53b48e89a8e94443bc521a237963f22264cc7d0411c99a10cfd9fa27da60`，SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `bash scripts/check-topic-docs.sh auxiliary-session-classification` 退出 0；`python3 scripts/check-docs.py` 退出 0。
- WorkUnit: `auxiliary-session-classification:tasks.md`；ContentState: `auxiliary-session-classification:tasks-r1-9f2c53b48e89a8e9`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-TASKS-R1-F1、ASC-TASKS-R1-F2 为 fail；Review 保持未勾选，Beads 返回修复。
- Residual uncertainty: 若修复需要新增 desktop wire 字段，应先确认架构 §7 是否已足够，否则回到架构阶段。

修复：auxiliary-session-classification / tasks.md / ASC-TASKS-R1-F1, ASC-TASKS-R1-F2

## Repair candidate — 2026-10-27（Round 1 后）

- Scope: ASC-TASKS-R1-F1、ASC-TASKS-R1-F2；作者 Codex。保留 Round 1 FAIL/FAILED，待独立复评。
- ASC-TASKS-R1-F1 -> repaired in candidate：任务 7 拥有 Go producer/desktop 命令接点、
  DesktopWire decoder、宿主协调、App Group 转换/发布和 Widget 消费的完整边界；任务 5 明确提供
  可按显式 Selection 读取的接口。选定宿主按 client × period 请求已有 session_scope_view 的路径，
  使用已确认持久默认而非 Popover 临时范围；没有新增未声明的多组合 wire 字段。
  架构 §6 的 Selection/publication_id、已提交投影和同代发布约束，§7 的 session_scope_view 与
  App Group summary，以及 Widget 最终设计的宿主转换要求共同支撑此分工。指定代不满足时
  不静默换代；跨组合/默认变动丢弃候选、有界重试及整份旧快照回退均成为任务 7 的验收项。
  增加各客户端/窗口不同计数、默认 Auxiliary/临时 Work、默认更新与跨组合失败的端到端 oracle；
  查询/转换成本纳入任务 9，保留零额外采集与现有刷新预算。
- ASC-TASKS-R1-F2 -> repaired in candidate：删除“尚待授权交付”陈述，改为已提交于
  `6818c682f96eaddcdab227ee96a0609bebbda4b9`。直接检查 commit 与 HEAD 中 Widget blob
  `0e764dbee6e92d2a878508ac7151d54901ff06ca`，与分解输入一致；不重新宣称本轮进行了交付。
- Candidate identity: HEAD `6818c682f96eaddcdab227ee96a0609bebbda4b9`；tasks.md blob
  `b660b9af8f14c17ddcb8ff9e0d013c535cae6643`。
- Evidence: 文档结构/完整文档集检查、`make check-whitespace`、`git diff --check` 通过；
  `DesktopWire.swift` 实际路径确认在 AgentDeckShared。本轮只修改计划与修复记录，无产品/测试变更。
- Limits: 未执行 Go/Swift 或端到端运行测试；这些是计划中的实现验收，非本次文档修复的通过声明。
  不自发记录独立 PASS、CEv1 VERIFIED、实现任务创建或 Git 交付。

## Round 2 — 2026-10-27

## 📋 tasks.md 分解与文档集修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-TASKS-R1-F1 — CLOSED**：任务 7 现在端到端拥有 Widget summary 生产链，文件边界纳入 `internal/desktop/desktop.go`
及测试、desktop 命令的显式 Selection 读取接点、`apps/macos/AgentDeckShared/DesktopWire.swift` 及解码测试与宿主发布协调器；
任务 5 只向其提供可按显式 Selection 请求的 view/decoder，并明确不以 Popover 当前 selection 作为 Widget 来源。
选定路径为宿主以已确认持久默认按 `client ∈ all/codex/claude × period ∈ today/7d/30d` 发起单选择请求——与
WidgetIntents.swift 中 `WidgetClient`（all/codex/claude）、`WidgetPeriod`（today/7d/30d）实际枚举一致；
依赖架构 §6 Selection 已含 publication_id，没有新增未声明的多组合 wire 字段。批次一致性（同默认版本、同 publication、
指定代不可用即整批丢弃、最多重试 2 次后保留旧快照 stale/无旧值 unavailable）与架构 §6 重试/回退规则一致；
验收加入各组合计数刻意不同、默认 Auxiliary 与 Popover 临时 Work 隔离、默认变更整体替换及混合代拒绝的联合 fixture，
批次成本归入任务 9 的发布成本。

**ASC-TASKS-R1-F2 — CLOSED**：tasks.md:29 改为 Widget 最终表面已提交于 `6818c682f96eaddcdab227ee96a0609bebbda4b9`，
并说明该提交包含输入表所列 Widget blob，与 HEAD 事实一致。

### 📝 总结

- Reviewer: Claude Code，原 Round 1 独立评审会话，未参与修复。
- Method: 单评审者；核对修复 diff、任务 5/7/9 文本、架构 §6/§7 与 Widget 实际 client/period 枚举；运行文档集与结构检查。
- Scope: ASC-TASKS-R1-F1/F2 及其对分解与文档集的影响；文档集七份，Round 1 已核对的输入 blob 与文件路径未变。
- Reviewed state: HEAD `6818c682f96eaddcdab227ee96a0609bebbda4b9`；修复候选 blob `b660b9af8f14c17ddcb8ff9e0d013c535cae6643`。
- Status sync: 复评后仅更新本文件 Documents 中 tasks.md 的 Review 勾选与“当前交接”两行状态陈述（逐行 diff 核对，无分解内容变化）；
  同步后 blob `a78549059325f78a17d37b4ea17157f348b42d88`，证据绑定此最终内容。
- Fingerprint: `eb5ab5ce53a2c3d0435b0c1734be50a37d7c032ab2675e1a05909d52b9e6aa4b`，SHA-256(`head=<HEAD>;document=<同步后 blob>`)。
- Evidence: `bash scripts/check-topic-docs.sh auxiliary-session-classification` 退出 0；`python3 scripts/check-docs.py` 退出 0；`git diff --check` 退出 0；
  评审记录经 `scripts/check-review-records.py` 校验。
- WorkUnit: `auxiliary-session-classification:tasks.md`；ContentState: `auxiliary-session-classification:tasks-r2-eb5ab5ce53a2c3d0`。
- Completion gate: VERIFIED
- Disposition: ASC-TASKS-R1-F1 CLOSED；ASC-TASKS-R1-F2 CLOSED。文档集七份 ratified。本记录引用的 ASC-SET-R1-F1、ASC-SET-R3-F1 是 ux/settings.md 的发现，非本记录发现，均已在 reviews/ux-settings.md Round 4 CLOSED；tasks.md 任务 6 仅把它们作为回归场景承接。
- Residual uncertainty: 任务 7 宿主批次请求的实际成本与指定代可得性需在实现与任务 9 中测得；本 PASS 不授权创建实现任务、开发或提交。
