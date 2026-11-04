---
status: active
topic: auxiliary-session-classification
subject: ux/cli.md
---

# CLI 框架评审

## 2026-10-27 证据范围与协调勘误

用户授权修正 Beads 状态后，确认先前将整个文档集缺稿归入本前置文档
document-integrity 的阻塞理由过宽。依据 docs/documentation-workflow.md 的文档集规则，
设计期间允许声明未编写文档；完整集合须在 tasks.md 文档集评审前补齐。
本勘误不改变语义 PASS、不撤销原发现、不豁免文档集/CI 检查，也不新建语义评审轮次。
下面各轮原 BLOCKED 及提交建议作为当时记录保留；当前协调以本勘误的交付对象及 CEv1 新观察为准。

- 交付对象：`0e74d0c98a5403338d637e1eb0095c1df7c41de4`；正文 blob `6d1d8eed3e4b801a0534d33e72cb3e76e1cf19b9`，与已评审正文一致。
- 新 ContentState：`auxiliary-session-classification:delivered-ux-cli.md-b94f096ccb05e96e`；指纹 `b94f096ccb05e96ee26f2d402ed72fbc1c87a10cba8159f4631a87f4cd5c1005`。
- 交付目标查询结果：VERIFIED（2026-10-27）；missing criteria 与 unresolved impacts 均为空。
  此结果仅绑定上述交付对象的 framework，不宣称当前全主题或最终 UX 已完成。
- 复用来源：`auxiliary-session-classification:ux-cli-r2-f07fae2c606b8626:independent-review`。新观察绑定交付对象并通过 rolls_up 显式引用；
  提交只固化已评审内容与记录，不引入新的正文决策。标本按各自交付 commit 核验，未用当前其他表面改动冒充旧 manifest。
- 本文检查范围：格式、真实本地链接、评审记录、空白和已绑定标本完整性；不包含尚未设计的另一文档。
  完整文档集缺稿仍由 tasks.md 承接。检查结果及门禁以新目标 CEv1 记录为准。
- 协调边界：本次仅确认已交付 framework；整份 UX 文档仍待 architecture 后 final-surface reconciliation，不关闭任务。后续共享原型变化不能因本次绑定历史交付对象而自动获得最终表面认可。

历史段落中的“尚未提交/不得提交”和旧 BLOCKED 不再用作当前待评审队列依据；
所有已有历史事实和旧目标证据保持不变。

## Round 1 — 2026-10-26

## 📋 CLI 交互框架评审

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-CLI-R1-F1 — P2 — usage 命令的扫描行为没有与现有契约对齐（OPEN）**

位置：ux/cli.md 第 48–49、154 行，关联第 16 行“不改采集”。

- 行为风险：同一个 `usage signals --session-scope work` 调用，按现有入口必须先扫描，按本文无隐式 scan 的统一 oracle 却应直接读存储。实现者无法确定保留扫描/进度/失败语义还是引入新的 stored-only 行为；仅切换范围不应增加全量重扫的需求不能直接替代原命令扫描契约。
- 证据：cli-manual.md 第 438–439、494–498 行明确 stats 默认扫描且有 --no-scan，signals 始终尝试扫描且无 stored-only 模式。cmd/agentdeck/main.go:3889–3892 的 stats 仅在 statsNoScan 为真时跳过，3940 行 signals 无条件调用 runUsageScanRound；3950 行保留 scan_incomplete/partial。本文新增范围覆盖这两个入口，却只给出“无隐式 scan”的数据供应要求，未声明例外或有意变更，也未设计现有扫描中和扫描失败的输出。
- 💡 修复建议：按入口写清扫描矩阵：会话浏览、分类查询/修改和 TUI 切范围不触发扫描；usage 默认扫描与既有 --no-scan 的行为明确保留或显式提出有依据的变更。区分“范围过滤不额外触发扫描”与“命令本来不扫描”，保留或明确替代 stderr progress、scan_incomplete/partial，并补充对应标本/验收映射。不得在修复中默默增加新采集策略。

**ASC-CLI-R1-F2 — P2 — 全屏标本未兑现声明的最小尺寸及高度预算（OPEN）**

位置：ux/cli.md 第 114–119、159–161 行；prototype/src/AuxiliaryCli.jsx:5–14。

- 行为风险：实现会从与文字契约相反的 40 列可操作列表或无限高标本派生布局；100 组无横向溢出不能证明最小全屏、短 viewport 和退出帮助可见。
- 证据：文档规定 Browser 40 列必须 tooSmall；但组件第 7 行仅按 state 选择 sample，cols 仅用于第 14 行 width，未参与退化。选择 `?surface=cli&auxiliary=browser&auxCols=40` 的数据仍为 auxiliaryCli.js:27 的工作列表、选择行和 `enter open`；tooSmall 是第 30 行独立手选场景。组件没有 rows/height 或 viewport 限制，不能表示 48×10 与同宽不同高度。已查看保留的 scope-failed-48 截图，其浏览器容器并无 10 行终端边界；不会据此声称已证明短终端行为。
- 💡 修复建议：让标本对明确的 columns/rows 组合遵守声明的退化、固定行预算和 viewport；或为每个有效尺寸提供对应的静态完整终端帧，并禁止展示无效组合。至少覆盖 40 列拒绝、48×10、80×24、120×40，验证退出帮助及选中项的可见性；更新 manifest 与保留截图。仍只要求设计标本一致性，不要求本轮实现产品 PTY 行为。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 三范围、JSON 兼容、--all 与 scope 的区别明确；TUI 保持只读。
- 区分保存失败、提交结果未知、保存成功但发布待完成，避免误导性成功回执。
- 定义 unknown、隐私、读屏替代、输入退出清理以及后续 PTY oracle，并明确 framework 与最终架构核对的界限。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与本 CLI 框架设计。
- Method: 单评审者，使用 development-workflow 与 designing-terminal-experiences；只读需求、当前 CLI 契约、限定源码和共享标本，检查 manifest 并查看保留截图。未委派、未修复设计或产品。
- Scope: ux/cli.md 的 framework，以及所依赖的共享原型源文件与 manifest；不评审 final-surface 或产品实现。
- Reviewed state: HEAD `1d6ab3a12c06ace582d6ee21f4109bf82988561c`；document blob `f19e01c60210ba9122d68a322ae19d7185332400`。
- Specimen identity: manifest SHA-256 `34a20ebedac18ffa4ef633ee6235b748d5e82043137c0592a3d973e7ba62adc1`；其中三个源文件与三张截图的 SHA-256 全部与当前文件一致。
- Fingerprint: `783544d1b188e55c5bdfc7037833d8507820b7119b8f524b1f7f80f703677a68`，SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: CodeGraph 定位现有 usage 路径后，限定读取 main.go:3876–3958；与 cli-manual.md 对照确认扫描契约。原型源码直接证明 sample 与列宽独立且缺少高度维度；不将静态截图当作终端运行证据。
- Evidence: 六项 manifest 哈希校验通过。`python3 scripts/check-docs.py` 退出 1，报告 ux/menubar.md、ux/settings.md、ux/widget.md、architecture.md 四份声明未编写文档；该生命周期缺口与上述正文/标本发现分别记录，不填占位或删减矩阵。
- Evidence limitation: 使用 React 静态渲染复现 40 列的尝试在加载 esbuild 时 MODULE_NOT_FOUND，未产生渲染结果，未安装依赖。设计者的 100 组浏览器检查是其原有证据，本轮未复跑，不作为几何合同通过证明。无需运行无关产品测试。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；ContentState: `auxiliary-session-classification:ux-cli-r1-783544d1b188e55c`。
- Completion gate: FAILED
- Gate scope: independent-review 因 F1/F2 为 fail；document-integrity 因未完成下游文档集为 blocked。原型文件未提交，但其 retention 声明要求随主题历史保留；本记录自行保存必要观察，不依赖临时输出。
- Disposition: ASC-CLI-R1-F1 OPEN；ASC-CLI-R1-F2 OPEN。Review 保持未勾选，返回同一文档任务修复；不是对需求 Round 2 的重新判定。

修复：auxiliary-session-classification / reviews/ux-cli.md / ASC-CLI-R1-F1、ASC-CLI-R1-F2

## Repair candidate — 2026-10-26（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-CLI-R1-F1/F2，未委派，产品代码保持只读。
- ASC-CLI-R1-F1 → repaired in candidate：根据当前 cli-manual.md 的 usage 契约补充逐入口扫描矩阵。
  stats 保留默认扫描/--no-scan，signals 保留每次扫描及 scan_incomplete/partial；
  无隐式扫描只适用于会话/分类/TUI 范围切换。范围过滤不增加额外扫描轮次。
  添加 signalsScanning、statsScanning、signalsScanIncomplete、statsStored 四个标本，
  progress 单列 stderr，扫描中 stdout 为空；未引入新采集策略。
- ASC-CLI-R1-F2 → repaired in candidate：标本增加真实 columns/rows/selection 轴，
  <48 列或 <10 行自动拒绝列表；有效帧固定四行头部、两行底部，rows−6 行 viewport，
  选中身份始终可见。终端外框随实际字符列宽、固定行高和行数变化，不再以无限高容器代替短终端。
- Candidate：HEAD `1d6ab3a12c06ace582d6ee21f4109bf82988561c`；ux/cli.md blob
  `6d1d8eed3e4b801a0534d33e72cb3e76e1cf19b9`；manifest SHA-256
  `9e33688ab2d34901a5a82edc6c9fda92ffc83f4cda09f7d2cf209f6c69bb37f2`。
- 验证：`npm --prefix prototype run build` 成功。构建后在独立浏览器会话测试
  browser/scopeFailed/scopeMenu/loading/tooSmall × 40/48/80/120 列 × 8/10/24/40 行
  × 选中 1/8/12，共 240 帧，无失败。断言实际字符列宽、rows×20px 内容高度、精确行数、
  无水平/垂直溢出、拒绝态无可操作列表、有效列表的选中行与底部 quit 帮助可见。
  四个扫描状态逐一核对 stdout/stderr 和 scan_incomplete/--no-scan 表达，4/4 通过。
- 保留新增五张 r1fix 截图与内容哈希；原三张截图标为历史，不再作为新高度合同证据。
  已目视核对最小帧与扫描失败结果。浏览器标本不是产品扫描、真实 PTY 或持久化验收。
- `python3 scripts/check-docs.py` 仍退出 1，仅报告其余三份 UX 和 architecture.md 未编写。
  不填充占位、不删矩阵、不豁免门禁。原 Round 1 的 FAIL、OPEN 和目标 FAILED 保留；
  两条 finding 待独立复评确认关闭，新候选不宣称 CEv1 VERIFIED，Review 保持未勾选。

## Round 2 — 2026-10-26

## 📋 CLI 框架修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-CLI-R1-F1 — CLOSED**：ux/cli.md 新增逐入口扫描矩阵，保留 stats 默认扫描和 --no-scan、signals 始终尝试扫描，限定会话/分类/TUI 无扫描，明确过滤不额外增加轮次。stderr progress、quiet、取消、scan_incomplete/partial 及存储不可读错误沿用现有契约。四个新扫描状态与该决定一致。
- **ASC-CLI-R1-F2 — CLOSED**：auxiliaryFrame 根据列数、行数自动拒绝不足 48×10 的全屏；有效列表固定四行头部、两行底部，以 rows−6 计算 viewport，选中身份保持可见。组件设置字符列宽、rows×20px 高度及固定行高，已替代无限高标本；保留截图证明 40×10 拒绝与 48×10 的选中第 8 条、底部退出帮助。

### 📝 总结

- Reviewer: Codex，原独立评审会话；未参与修复。
- Method: 单评审者，逐 finding 文档/源码核对、独立字符帧断言、manifest 校验与三张保留截图目视检查。复用未变 HEAD 上的现有 usage 扫描源代码与 CLI Manual 核对；未重新运行产品测试或构建，未委派。
- Scope: CLI framework 的 F1/F2 及受影响标本。此 PASS 不是 final-surface，architecture 后仍须核对字段、协议和最终表面。
- Reviewed state: HEAD `1d6ab3a12c06ace582d6ee21f4109bf82988561c`；document blob `6d1d8eed3e4b801a0534d33e72cb3e76e1cf19b9`。
- Specimen identity: manifest SHA-256 `9e33688ab2d34901a5a82edc6c9fda92ffc83f4cda09f7d2cf209f6c69bb37f2`；8 项当前源文件/截图及 3 项历史截图均 MATCH。
- Fingerprint: `f07fae2c606b862666c704e9cb23ed78054263669147aee353240ae96162b747`；沿用 SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: Node 直接导入 auxiliaryCli.js，对五个全屏状态 × 四个列宽 × 四个行数 × 三个选择，共 240 帧检查精确行数、字符行宽、quit 提示、拒绝态无 task/enter open、有效列表选中项可见及 viewport 容量；全部通过。四个扫描状态核对扫描中 stdout 空、独立 stderr、scan_incomplete/last committed、--no-scan，全部通过。只覆盖合成字符标本，不是浏览器物理几何或产品 PTY 验收。
- Evidence: 目视检查 r1fix-browser-40x10.png、r1fix-browser-48x10.png、r1fix-signals-incomplete.png，与当前来源和矩阵一致。设计者原有 build/240 组浏览器几何检查保留为 Repair 证据；本轮未把它冒充独立运行结果。
- Evidence: 本轮记录和矩阵同步后，`python3 scripts/check-docs.py` 退出 1，仅报告四份下游文档未编写；评审记录解析、`make check-whitespace`、`git diff --check` 均退出 0。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；ContentState: `auxiliary-session-classification:ux-cli-r2-f07fae2c606b8626`。
- Completion gate: BLOCKED
- Gate scope: independent-review 为 pass；document-integrity 的剩余缺口为 ux/menubar.md、ux/settings.md、ux/widget.md、architecture.md。保留原轮 FAILED 历史，不将其迁移为新目标失败或抹除。
- Hierarchy: 主题矩阵仍有其他文档和后续分解未完成，本轮不跨越 Topic 完成边界。
- Residual uncertainty: 原生输入生命周期、真实扫描/持久化和最终架构供应尚未验收；框架明确要求的后续证据保持有效。

Task checkpoint：ad-auxiliary-session-classification-doc-ux-cli；content_state `f07fae2c606b862666c704e9cb23ed78054263669147aee353240ae96162b747`；gate BLOCKED。
提交建议：门禁 VERIFIED 后，经授权提交 ux/cli.md、共享原型相关改动、manifest/保留截图、评审记录与 tasks.md；前置为完成四份下游文档并通过文档完整性检查。
推送建议：门禁 VERIFIED 后，经授权推送 feature/auxiliary-session-classification；前置同上，remote/upstream 尚未解析，提交内容和 SSH 签名留待交付核验。

完成已声明的 ux/menubar.md、ux/settings.md、ux/widget.md、architecture.md，通过文档完整性检查后重算本 WorkUnit 门禁；保留本轮 framework PASS。

## Round 3 — 2026-10-27

## 📋 CLI 最终表面评审

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。原 ASC-CLI-R1-F1/F2 已关闭且无相关回归，未发现新增阻塞项。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- --clear 已与架构的 core 覆盖键合同一致；未匹配覆盖、tombstone/no_change、null 分类和独立读回不再被可见会话查询阻塞。
- 写结果未知保留 request_id，引导读原回执而非换 ID 盲重试；历史回执与当前覆盖状态明确区分。
- 桌面默认和来源管理命令补齐；配置路径不回传，配置成功不冒充 SDK 映射已验证；JSON 增量字段与旧集合形状分开。

### 📝 总结

- Reviewer: Codex，独立最终表面评审，未参与本轮设计修改。
- Method: 单评审者比较当前 CLI 文档/标本与已通过 architecture §5/§7/§9；核对现有 Envelope/errorCode 来源；运行已有语义/帧检查并目视查看保留截图。未委派、未修改产品、测试、配置或标本。
- Scope: CLI final-surface reconciliation，不是产品命令、CAS/SQLite、真实 PTY 或维护来源格式验收。
- Reviewed state: HEAD `c57102bb5a641cf9bb6556941f63499d5c5217da`；CLI blob `e4651cca9dbdc40348a0624134778a327539eb3e`。
- Architecture dependency: blob `5b5a3b5694827900a9380e3879058a6baca4919a`，已交付 commit `c57102bb5a641cf9bb6556941f63499d5c5217da`；CEv1 delivered state `auxiliary-session-classification:arch-delivered-d6957d95a336e240`。
- Specimen identity: manifest SHA-256 `7c3aa502273a539c08cb50d5cb544829134753d8e88647940b74ef78e85c26d2`；8 项当前文件 SHA-256 全部 MATCH，历史 framework 文件不冒充当前内容。
- Fingerprint: `238ada644f8b104c1a876e4d91c6005633d9cf84b9881aa576722f60c7ab64db`；SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: `node prototype/tools/check-auxiliary-cli-final.mjs` 实测 14 semantic_checks、240 frame_cases、0 failures。检查含 clear revision5/no_change5/不存在0、回执重放、unknown request、unmatched 枚举、默认未确认和来源输出不泄漏路径；这是合成设计数据，不是产品运行。
- Evidence: source set/disable 的 JSON 只含来源/配置/启用/coverage 和失效标记；没有 data-dir。清除回执的 automatic/effective=null，classification_available=false/not_queried；旧请求读回与原清除 data 一致。读请求 outcome=unknown 可为 exit0，但文档要求自动化核对 outcome，不当作写入成功。
- Evidence: internal/output/output.go 的 schema_version=1、command/generated_at/data/warnings/partial/error 形状及 NewErrorWithDetails 与标本一致；main.go 的通用错误类别为 runtime_error。新增 session_selection 明确为拟实现的可加字段，不声称现有发布版已具有它。
- Evidence reuse: auxiliaryFrame 通过逐字比较与已交付版本一致，帧样式未变，复用 Round 2 几何与原扫描合同证据。目视核对 final-clear-unmatched-40.png、final-request-unknown-80.png：身份、请求、版本、未查询分类和未知结果恢复命令可见。设计者 60 组浏览器与构建结果仍标为原作者检查，本轮不冒充复跑。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；ContentState: `auxiliary-session-classification:ux-cli-final-r3-238ada644f8b104c`。
- Completion gate: VERIFIED
- Gate scope: 本最终表面的独立评审、文档完整性和架构对齐。新增 architecture-reconciliation 必需项用于最终表面边界；历史 framework 结果与其范围不被改写。
- Evidence: 本轮评审与矩阵同步后 `python3 scripts/check-docs.py` 通过，涵盖空白、diff、本地链接、记录结构及文档集合结构；不替代 tasks 分解评审。
- Hierarchy: 其余三份 UX 尚需最终核对，tasks.md 尚未分解/评审；不跨越 Topic 完成边界。
- Residual uncertainty: 最终生产输出仍需兼容 golden、真实 PTY、CAS/回执、SQLite 和来源适配器隔离验收。设计样例不能替代这些测试。

Task checkpoint：ad-auxiliary-session-classification-doc-ux-cli；content_state `238ada644f8b104c1a876e4d91c6005633d9cf84b9881aa576722f60c7ab64db`；gate VERIFIED（final-surface）。
提交建议：经授权提交 CLI 最终文档、共享 CLI 标本、检查脚本、manifest/新增截图、对应评审记录和矩阵状态。
推送建议：经授权提交并核验提交对象、Codex trailer 与 SSH 签名后推送 feature/auxiliary-session-classification；remote/upstream 尚未解析。

设计：auxiliary-session-classification / ux/menubar.md（final-surface reconciliation）

## Round 4 — 2026-11-03

## 📋 CLI 集成修订评审（批量评审：所有文档）

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

以下是本轮开放发现，PASS 前必须关闭。

**ASC-CLI-R4-F1**（P2）[ux/cli.md:40-60, 202-203] 没有读取来源状态的 CLI/JSON 入口。
- 行为风险: 命令表只有 `origin-source set/disable claude-mem`；内建 `codex-otel-background` 不可 set/disable，因此 CLI 用户与经 CLI bridge
  读取的 Settings（tasks.md 任务 6）都无法得到任一来源的 configured/enabled/available/reason。修订新增内建来源后，这一缺口在两行来源状态上都存在。
💡 修复: 待 ASC-ARCH-R3-F5 定义读取合同后，在命令表与 JSON 合同中加入只读来源状态入口（含两类来源、字段白名单、exit 码），并补对应 Specimen 说明。

**ASC-CLI-R4-F2**（P3）[ux/cli.md:59] 依赖未定的 reason_code。
- 行为风险: 文中固定显示 `codex_otel_background`；若 ASC-ARCH-R3-F4 选择复用 `maintenance_origin` 或改名，本处即与架构矛盾。
💡 修复: 按 ASC-ARCH-R3-F4 的结论同步原因码文字。

### 🟢 优点

- 内建来源 set/disable 为 exit2 且不改状态、以单会话 `--set work` 代替按来源整体关闭，与架构 §3.4 “内建、无配置”一致，避免第二个开关语义。
- 空项目与 `session show` 无转录的说明与 scan-performance 的会话索引投影（`otel:codex/<id>`、项目为空）一致。

### 📝 总结

- Reviewer: Claude Code（claude-code），独立评审会话。
- Method: 单评审者；读 diff 与命令/JSON 合同全文，对照架构 Round 3 与 Settings；未运行 CLI 原型检查（原型 CLI 文件本轮未修改）。
- Scope: ux/cli.md 全文；依赖 architecture.md（Round 3 FAIL）。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/cli.md blob `4bafecb2d0559be573a888a278f8226b1616223c`（未提交）。
- Fingerprint: `46974b3065f44c5a015a578c1b39772f4ea946b56eb72464eb0bb4d0c89911c1`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`scripts/check-whitespace.sh` 无输出。
- Disposition: ASC-CLI-R4-F1、ASC-CLI-R4-F2 开放。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；本内容状态未记录 pass 证据。
- Completion gate: NOT_VERIFIED

## Repair candidate — 2026-11-04（Round 4 后）

- Repairer：Claude Code（claude-code）修复会话；处理 ASC-CLI-R4-F1/F2，未委派。
- ASC-CLI-R4-F1 → repaired in candidate。命令表新增 `session origin-source status`；§7 补 JSON command、字段白名单、单项失败 exit0+partial、多余参数 exit2，并给出文本 specimen 与未知 reason 回退；注明浏览器 CLI 标本未覆盖、由任务 4 按合同验收。
- ASC-CLI-R4-F2 → repaired in candidate。原因码改为 `maintenance_origin` + 规则版本 `codex-otel-background-v1`，与架构 §3.3 一致。
- Candidate identity：HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/cli.md blob `7b2d21468d4de773c2da2c10edd0ac31af0d6aae`；依赖 architecture 候选 `d7c63db9b0933e7641564ff0a9711b31c7318d2f`。
- 验证：`npm run build` 通过；auxiliary CLI/Settings/Widget final 契约脚本 exit0（14/14/39 项）；浏览器 Settings 脚本 70 项、菜单栏 176 项 0 失败；Settings 五种 Codex 行状态逐一渲染核对。`python3 scripts/check-docs.py`、`scripts/check-topic-docs.sh`、`scripts/check-whitespace.sh` 见修复交接。未运行产品测试（无产品代码变更）。
- 原 FAIL/OPEN 与 NOT_VERIFIED 保留；本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 5 — 2026-11-04

## 📋 CLI 集成修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-CLI-R4-F1 — CLOSED**：命令表新增只读 `session origin-source status`。以下合同都已写明：JSON command、两项固定顺序、字段白名单、单项失败时 exit0 且 partial、多余参数 exit2、文本标本不打印路径/端点/token、未知 reason 的回退。文档也注明浏览器 CLI 标本未覆盖此命令，由任务 4 按合同验收。
- **ASC-CLI-R4-F2 — CLOSED**：Codex 后台会话改为 `maintenance_origin` 加规则版本 `codex-otel-background-v1`，与架构 §3.3 的枚举一致。
- 内建来源调用 set/disable 时 exit2 且不改状态，与架构 §3.4“内建、无配置”一致。

### 📝 总结

- Reviewer: Claude Code（claude-code），Round 4 评审会话；未参与修复。
- Method: 单评审者；读 diff，对照架构 §3.3/§3.4/§7 的修复候选；未运行 CLI 原型（`prototype/src/auxiliaryCli.js` 本轮未修改）。
- Scope: ASC-CLI-R4-F1/F2 及新增的 status 段落。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/cli.md blob `7b2d21468d4de773c2da2c10edd0ac31af0d6aae`（未提交）。
- Fingerprint: `053e798c443f98a9ec6624c6fa45b8b450a6e5985f9a49b353ee6ef31699e451`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`bash scripts/check-topic-docs.sh` exit 0；`bash scripts/check-whitespace.sh` exit 0。
- Dependency: 本文的 status 字段写为“按架构 §7”，只引用不复制。ASC-ARCH-R4-F1 只会收紧 available/partial 的取值，不改本文的命令、字段集合或退出码。架构修复后如果改动了 CLI 可见字段，需要对本文做一次影响核对。
- Disposition: ASC-CLI-R4-F1、ASC-CLI-R4-F2 CLOSED。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；ContentState: `auxiliary-session-classification:cli-r5-053e798c443f98a9`。
- Completion gate: VERIFIED
- Gate scope: 本文档的独立复评与完整性检查；不认证 CLI 实现或 PTY 验收。

## Repair candidate — 2026-11-04（Round 5 后，来自 ASC-TASKS-R5-F1）

- Repairer：Claude Code（claude-code）修复会话；因 tasks 评审发现 ASC-TASKS-R5-F1 修改本文件，未委派。
- ux/cli.md §7 原“单项读取失败只置该项 available=false”改为引用架构 §7：partial=true 仅限三种读取失败；available 按 §7 字段表，claude-mem 需配置启用且最近运行 ok/partial，Codex 只要 core 可读即 true，receiver=unknown 时仍 available=true（有记录 reason=null，无记录 otel_receiver_unknown）；各情形 exit0。
- Round 5 PASS 绑定旧 blob `7b2d21468d4de773c2da2c10edd0ac31af0d6aae`，不适用于本候选，需复评。
- Candidate identity：HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/cli.md blob `549477b1b3e47aae80b26756a27bde6cd44c1360`；依据 architecture blob `416d63c922b9ea445a53a9774388efa13776af1c`（Round 5 PASS）。
- 验证：`python3 scripts/check-docs.py`、`scripts/check-topic-docs.sh auxiliary-session-classification`、`scripts/check-whitespace.sh` 均 exit0；未改原型或产品代码，未运行产品测试。
- 本候选不自称 Review PASS 或 CEv1 VERIFIED，未写 pass 证据。未提交、未推送。

## Round 6 — 2026-11-04

## 📋 CLI 来源状态语义复评（ASC-TASKS-R5-F1）

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **ASC-TASKS-R5-F1（本文部分）— CLOSED**：ux/cli.md:210-213 不再自行规定 available，改为“按架构 §7 规则取值，CLI 不另立判定”。
  partial 只在三种读取失败时为 true；claude-mem 只有在配置启用且最近一次运行为 ok/partial 时才 available；Codex 只要 core 可读就是 available，receiver=unknown 时仍为 true（有记录时 reason=null，无记录时为 otel_receiver_unknown）；所有情形都 exit0。
  这与架构 §7（blob `416d63c…`，Round 5 PASS）和 ux/settings.md 五态表的“开关未知 available=true”三处一致。
- 在全文检索“单项读取失败”与 available/partial，没有其他与 §7 冲突的残留表述。第 204 行的 warning/partial 属于分类查询的来源不可用，与 status 命令无关。

### 📝 总结

- Reviewer: Claude Code（claude-code），Round 4/5 评审会话；未参与修复。
- Method: 单评审者；读修改段 ux/cli.md:207-224，对照架构 §7 字段表、原因表与 partial 段落，以及 Settings 五态表；未运行 CLI 原型（原型未修改）；不委派。
- Scope: status 段的 available/partial 表述；其余内容相对 Round 5（blob `7b2d214…`）未变。
- Reviewed state: HEAD `3ec8709a00b516b7e624da329ae178de6a7cde6b`；ux/cli.md blob `549477b1b3e47aae80b26756a27bde6cd44c1360`（未提交）。
- Fingerprint: `795243633f402267bda4bb7a113ce35ce670dd372a962f8309f32dc7ef5e0047`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `python3 scripts/check-docs.py` 通过；`bash scripts/check-topic-docs.sh` exit 0；`bash scripts/check-whitespace.sh` exit 0。
- Architecture dependency: blob `416d63c922b9ea445a53a9774388efa13776af1c`，ContentState `auxiliary-session-classification:arch-r5-4299e9eae33f0e15`，门禁 VERIFIED。
- WorkUnit: `auxiliary-session-classification:ux/cli.md`；ContentState: `auxiliary-session-classification:cli-r6-795243633f402267`。
- Completion gate: VERIFIED
- Gate scope: 本文档的独立复评、完整性检查与架构对齐；不认证 CLI 实现或 PTY 验收。
