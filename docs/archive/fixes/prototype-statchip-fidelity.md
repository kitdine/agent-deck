---
status: historical
created: 2026-10-02
retired: 2026-10-02
---

# 缺陷：原型 stat chip 缺少原生缩放，节律峰值显示两小时短格式

## 现象

Beads：`ad-bug-prototype-statchip-fidelity`。Lane A；本批仅此项与
`ad-bug-prototype-probe-pending-assertion`，工作区 `agent-deck.prototype-fidelity-batch`，
分支 `fix/prototype-fidelity-batch`，基线 `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`。

当前基线已有 280pt 四项网格变两列的布局，原报告的默认截断不再直接复现。
但峰值仍显示 `15–17h` / `15–17 时`，三行均无原生最小缩放行为；
最终回归探针放入隔离基线后得到 7 条预期失败，确认这两处仍在。

## 根因

`StatGrid` 仅对值设置 ellipsis，未实现原生 `StatChipRow` 的 label/value/note
最小缩放比例 `.72/.7/.72`。`RhythmBlock` 使用派生的多小时范围，而原生
`rhythmBlock` 取最大单元所在小时，`DesktopFormat.hourWindow` 用本地化 hour/minute
格式显示一小时；并列最大值保留第一个单元。

## 修复边界

原型 `StatGrid` 三行在各自原生最小比例内测量并缩小，监听容器尺寸、文字与字体
就绪变化。固定字距不会随字号缩小，因此用有限二分测量，避免宽度比留下截断；
最低字号仍放不下时保留 ellipsis，避免声称任意宽度均完整。

节律峰值改取第一个最大单元，并使用独立 `formatHourWindow`。英文保留原生
AM/PM 前的窄不换行空格，中文补齐小时位，支持午夜跨日。
既有 `formatHourRangeShort` 保留给 Widgets，避免扩大本项到 Widget 标本。
扩展 `contract=1` 的 Popover 分支并添加六项 formatter 单测；不改原生代码、
footer、两列布局或依赖。

## 验证

- 最终候选 HEAD：`4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`。
- 代码、测试、依赖及原生契约指纹：
  `34a4d7b5ca7cd27d8728abba5367c850e965b3e634b0cbe73f5fcb0196072116`。
- RED：以 `git archive <base> prototype` 导出隔离 fixture，仅放入最终 `contract.js`。
  `http://127.0.0.1:4187/?contract=1&lang=en&width=420&tab=usage` 得到 7 FAILED，
  失败为一小时峰值及 label/value/note 的缩放与最小比例；卡片数量和原布局完整性通过。
- GREEN：中英、420/280pt 的 Popover contract 全部通过。窄布局继续覆盖
  normal/empty/aged/partial/pending × en/zh × dark/light；矩阵结果由独立评审核对。
- `node --test prototype/src/i18n.test.js`：6/6 PASS；测试覆盖中英的 0、15、23 时。
- 独立 Swift/Foundation fixture 核对同一 `hour().minute()`：
  英文 `3:00 PM–4:00 PM`，中文 `15:00–16:00`，午夜/跨日与单测预期一致。
  这是 formatter fixture，不能替代原生 UI 验收。
- `npm run build --prefix prototype` 通过。L1 原型修改，无 Go/原生实现修改。
- 本机证据目录：`/tmp/agentdeck-prototype-fidelity-01a0fd06/`，
  `red-statchip-final-baseline.txt`、`green-statchip-*.txt`、`matrix-*.txt`、
  `hour-window-tests.txt`、`build.txt`、`content-state.json`；图像证据另列于评审记录。
- 浏览器证据只覆盖原型；菜单栏图标真机验收由用户 deferred，不标为通过。

## Review — Round 1

## 📋 stat-chip 原型修复独立评审

📊 综合评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 本轮必须闭合

`PSF-R1-F1`（P3，OPEN）：`prototype/src/contract.js` 新增 Popover 分支通过
`.rhythm .stat-grid`、翻译后的 label 及标签位置定位受测行，违反
`ln-12-delivery-reviewer` 的 VERIFY-4 稳定定位要求。样式或文案变化会造成量具误报，
削弱本项要恢复的原型回归信号。

证据：新分支的卡片、peak 与三行选择器直接绑定 CSS/翻译文字；现有渲染结果未发现
相应产品缺陷。修复限于增加仓库自有的 grid/chip/line 测试标识，改用标识定位，
保留本地化字符串为独立结果断言。依据：所用
评审技能 VERIFY-4（本地溯源：`/Users/jobshen/.codex/skills/ln-12-delivery-reviewer/SKILL.md`；2026-10-05 观察 SHA-256 `1e1eea1b9ad68fffdb90081958ce989a4e64b0c13cda615f91c734c3c2f0673e`，非追认历史哈希）。

### 🟢 优点

隔离基线能触发 7 条真实失败；最终中英/420/280pt 核心检查、6 项 formatter 测试
与构建通过，修复保持原生最小比例及一小时语义，未改原生或 Widgets 行为。

### 📝 总结

Reviewed state：HEAD `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`，上述
`34a4d7b...6072116` scoped fingerprint。
Reviewer：`cold_prototype_review`（独立冷上下文只读输入），Codex 主 agent 核查并裁决。
Method：源码/原生契约与既有证据交叉核验；Scope：本项五个源码/测试文件及对应修复记录。
独立评审已确认 20 个窄矩阵文件存在，首轮尚未逐项核对；主 agent 已直接观察完整
矩阵运行 20/20 ALL PASS。修复前不扩大验证；后续只复评测试定位改动及矩阵证据。
Completion gate：NOT_VERIFIED（WorkUnit/证据关系待补齐）。
本轮先闭合 `PSF-R1-F1`，不重复已完成的产品验证或原生 deferred 验收。

## Review — Round 2

## 📋 stat-chip 原型修复独立复评

📊 综合评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 本轮必须闭合

`PSF-R1-F1`：CLOSED。Popover contract 新分支改用仓库自有的 root/grid/chip/line
测试标识，翻译后的峰值字符串仅作为独立结果断言，不参与定位。
`contract-hooks-en-420.txt` 的九条断言全部通过。

### 🟢 优点

| 验收项 | 实现及独立证据 | 结果 |
| --- | --- | --- |
| label/value/note 原生最小比例 | ScaledStatText 与 StatChipRow 的 `.72/.7/.72` 一致；四个核心契约 | PASS |
| 单小时峰值、并列取首个最大值 | 严格 `>` 选择及原生 hourWindow；中英午夜/15 时/23 时六项测试 | PASS |
| 窄状态、语言、主题矩阵 | 全部二十份文件：十六份各 9 PASS，四份 empty 各 7 PASS，无 FAIL | PASS |
| Widgets 保持既有短格式 | 原调用与 formatHourRangeShort 均保留 | PASS |
| 回归敏感性 | 隔离基线七条预期失败；当前契约、formatter 6/6 和构建通过 | PASS |
| 渲染可见性 | 新英文 420pt 深色及中文 280pt 浅色截图显示四项卡片和完整一小时峰值 | PASS |

### 📝 总结

Reviewer：`prototype_selective_rereview`，独立冷上下文、只读选择性后续轮，批次
2/2；主 agent 直接核查并裁决。Checklist：54/54 complete；Incomplete：None。
Method：工作区绑定的 CodeGraph，结果不适合时精确源码检查；差异、逐文件哈希、
全部二十份矩阵、契约/formatter/构建结果及截图像素检查。新 findings：None。

Reviewed state：HEAD `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`；
可独立重算的产品指纹
`7c8bd300976de22e9ab132964867ca134e61765497f01aaca7a6b66b97f6170d`。
配方为证据目录 `resume-product-manifest.json` 的 payload，按 UTF-8、排序键、
紧凑 JSON、无末尾换行计算 SHA-256；包含排序后的 11 项路径/哈希、HEAD 和
Node `v26.10.0`。旧清单逐文件身份均验证，旧聚合配方未保留，不声称重算旧聚合值。

四张旧 rhythm 截图均为空白，明确排除，不沿用其视觉 PASS。实际替代证据是
`resume-rhythm-en-420-dark.png` 与 `resume-rhythm-zh-280-light-final.png`，
主 agent 与独立 reviewer 均直接查看；中文截图将面板滚动到节律网格。
`candidate.diff` 早于原 formatter 清单，证据复用以逐文件哈希为依据。

Tests：UPDATE Popover contract 稳定定位；ADD 六个独立 formatter 期望；KEEP Widgets
短格式。空状态没有 note 行，七条断言属预期覆盖；无新依赖或额外套件。
Documentation：UPDATE 本记录；无新增 topic、全局状态或无关文档修改。
Completion gate：由 CEv1 WorkUnit
`urn:ce:agent-deck:work-unit:fix:prototype-statchip-fidelity`
保存候选、不可变提交及实际合并内容的目标绑定结果。
Residual risks：低于原生最小字号仍可能 ellipsis，符合修复边界；原型浏览器证据
不替代原生 UI，菜单栏图标真机验收仍 user-deferred。

## Delivery

本项 SSH 签名提交暨同批最终产品 head：
`5a96475d9e1cbe79f8f856b7a6c6e6d00d093721`。
[PR #33](https://github.com/kitdine/agent-deck/pull/33) 在精确 head 的 Codex Review
无发现及 CI `verify` / `desktop` 全部成功后转 ready，实际 merge commit 为
`951b16dcb036c8e2b24ca3b2fcb0a2e367324efa`。
实核 parents 为 `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` 与上述 head；
tree `2182087b84f63dc5497d975b104c7100668c8f95` 与已评审产品 tree 相同。
两项产品提交的 SSH 签名、完整消息及 Codex trailer 已验证；GitHub 报告实际 merge
PGP 签名 `verified: true, reason: valid`。
Completion gate: VERIFIED；仅指上述历史实际 merge `951b16dcb036c8e2b24ca3b2fcb0a2e367324efa` 的 CEv1 6/6。
此字段不补填此前评审候选的门禁，也不认证当前集成候选。
按用户明确约束，记录保持 active 直至实际合并；现在才归档，保留 FAIL、CLOSED
和后续 PASS 历史。原生菜单栏图标验收仍 user-deferred；本批没有发版或部署。
