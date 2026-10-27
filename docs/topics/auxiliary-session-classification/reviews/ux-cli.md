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
