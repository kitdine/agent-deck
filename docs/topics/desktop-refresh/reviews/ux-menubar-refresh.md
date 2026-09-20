---
status: active
topic: desktop-refresh
subject: ux/menubar-refresh.md
---

# Menu-Bar Refresh UX Review

## Round 1 — 2026-09-19

## 📋 Menu-Bar Refresh surface framework 评审

📊 总体评分：8.8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

**[MR-R1-F1] `prototype/src/Popover.jsx:1566` 中文 Retry 的 accessible name 使用英文句点，与文档声明的中文合同不一致。**

- 严重度：P2。
- 行为风险：中文 VoiceOver/浏览器辅助功能树读出 `刷新失败. 重试`，而 `ux/menubar-refresh.md:173` 明确要求 `刷新失败。重试`。当前 `checks.json` 只保存英文 `Refresh failed. Retry`，所以内容绑定的自动化证据无法发现中文合同回归。
- 证据：本轮重新启动当前 worktree prototype，在 `refresh=firstFailure&lang=zh&theme=dark&width=420&tab=usage` 捕获的 accessibility tree 显示 button name 为 `刷新失败. 重试`；源码用 ```${dict.refreshFailed}. ${dict.retry}``` 对两种语言硬拼英文句点。文档和源码位置分别为 `docs/topics/desktop-refresh/ux/menubar-refresh.md:173` 与 `prototype/src/Popover.jsx:1566`。
- 处置：OPEN。

💡 有界修复：由中英文资源分别提供完整 Retry accessible label，或提供本地化连接符，确保中文精确为 `刷新失败。重试`、英文精确为 `Refresh failed. Retry`；把两种语言的名称都加入 browser verification，更新 `checks.json`、相关 specimen 与 manifest 内容哈希。不要改动可见 Retry 文案、状态层级或刷新交互。

### 🟡 建议改进——推荐

无；除 `MR-R1-F1` 外不记录偏好性改写。

### 🟢 优点

- 成功数据年龄和刷新 attempt 独立呈现；刷新中与失败后都保留旧数据和原始年龄，没有制造“刚刚更新”。
- 280 pt 下动作正确压缩为图标，同时保留完整交互目标；失败 notice 仍完整换行，未挤掉数据内容。
- first-use failure、App Group publication failure 和 recovery 的信任边界清晰；存储失败没有降级菜单栏已接受的新数据。
- 键盘 Retry 在 running 和 success 后保持焦点，running 阶段暴露 disabled 语义，恢复后仅清除匹配的 refresh notice。
- 数据需求逐项交给 architecture 处置，没有提前发明 wire 字段或 scheduler 实现。

### 📝 总结

- Reviewed state：HEAD `4e203c527ca0d836b95da0a6e71da95ca4ac2846`；`ux/menubar-refresh.md` blob `717bbb4b2be7cffb0aeec4a34ed9640ae27b5755`；prototype manifest SHA-256 `3cd6b1af5123c9a8323c9a3683960f66765ed4e0407f363280b4408f63886175`；内容指纹 `fede44b50e5676ce60345af0a2721e61022d74d5ceb48bc76fdc1b3977ba3db9`，配方为 `SHA-256(head=<HEAD>;document=<blob>;prototype=<manifest_sha256>)`。
- Reviewer：Codex。
- Method：设计/契约审查 + Product Design combined UX/accessibility audit。使用当前 worktree 启动 Vite，在隔离 agent-browser session 中重新捕获并检查六个规定状态；重新执行键盘 Retry/focus/recovery；检查当前源码 diff、requirements、manifest 和 live Beads。未委派，不声称原生 VoiceOver 或 AppKit 验收。
- Scope：`docs/topics/desktop-refresh/ux/menubar-refresh.md`、其六个共享 prototype specimen、manifest/checks，以及生成这些状态的 `prototype/` 修改。未评审 architecture、Widget UX、产品 Swift 实现或任务分解。
- Findings：`MR-R1-F1` OPEN；无其他 finding 或外带项。
- Audit steps：1) running/prior — healthy；2) wake/narrow — healthy；3) retained-data failure/narrow — healthy；4) first-use failure/Chinese — finding `MR-R1-F1`；5) Widget publication failure — healthy；6) recovery/narrow — healthy。当前捕获保存在 `/private/tmp/agentdeck-menubar-refresh-review.6kVRif/`，正式 subject specimen 仍由 manifest 绑定在 `ux/prototype/menubar-refresh/`。
- Evidence：`npm --prefix prototype run build` 通过（4583 modules）；`make check-whitespace` 与 `git diff --check` 通过；manifest 的 12 个文件哈希全部匹配，manifest SHA-256 与文档声明一致；本轮键盘验证确认 running 时 `aria-disabled=true`、焦点保留，成功后 `aria-disabled=false`、焦点仍保留且 failure notice 清除。仓库内旧 `checks.json` 的 axe 结果是内容绑定辅助证据；本轮截图不能证明原生 VoiceOver、Dynamic Type、AppKit focus、真实睡眠或 Increase Contrast。
- Completion gate：FAILED。固定 CEv1 gate 对 `desktop-refresh:ux/menubar-refresh.md` 和精确 ContentState `urn:agent-deck:content-state:desktop-refresh:ux-menubar-refresh:4e203c5:717bbb4:3cd6b1a` 回查：L0 pass，UX 与 formal review 两项由 `MR-R1-F1` 的 target-bound fail evidence 反证；无缺失、blocked、malformed、失效证据或未决 candidate impact。

整体 framework 已覆盖需求状态，视觉与交互主体成立；但目标自己的中文无障碍 copy contract 尚未满足，且当前自动化缺少对应语言断言，因此本轮不能 PASS。

### Repair candidate — 2026-09-19

- `MR-R1-F1 -> repaired in candidate.` `i18n.js` 现在分别提供完整的
  `Refresh failed. Retry` 与 `刷新失败。重试`，`Popover.jsx` 不再用英文句点拼接两种语言。
- `checks.json` 同时保存中英精确 accessible name；浏览器回查英文命中 1、中文全角
  句号命中 1、错误的 `刷新失败. 重试` 命中 0。两种失败态 scoped axe 均为 0
  violations；英文数据态保留既有 calendar incomplete，中文首次失败空态为 0 incomplete。
- Repair candidate：HEAD `4e203c527ca0d836b95da0a6e71da95ca4ac2846`；
  `ux/menubar-refresh.md` blob `23420054b81bf4880c4d253c3fc482feb48f7ae4`；
  prototype manifest SHA-256 `64083a6a4bad190debac0c96547d3648e59b83a3b0bf11c7115ca634783a5752`；
  内容指纹 `5f8b5af1362844557a12b44b6c4ec32a3732a374d4539c61a3261f2fd51a6805`。
- Verification：prototype production build PASS（4583 modules）；manifest 12/12 匹配；
  `make check-whitespace` 与 `git diff --check` PASS。相关中英失败标本已从修复后 source
  重新捕获；可见文案、层级、Retry 交互和生产 Swift 均未改。

Round 1 结论仍为 FAIL，Review 矩阵仍未勾选；以上处置只声明返修候选就绪，
`MR-R1-F1` 是否 CLOSED 及新候选门禁由独立复评决定。

## Round 2 — 2026-09-19

## 📋 Menu-Bar Refresh surface framework 复评

📊 总体评分：9.6/10

✅ 复评结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `MR-R1-F1 -> CLOSED`：中英文完整 Retry accessible label 现在由各自本地化资源拥有；当前浏览器 accessibility tree 精确返回 `刷新失败。重试` 与 `Refresh failed. Retry`。
- `checks.json` 将单一英文值升级为 `en` / `zh-Hans` 双语精确断言，修复了 Round 1 的保护缺口。
- 修复没有改变可见 Retry 文案、失败 notice、数据层级或交互；英文 280 pt 状态仍完整，中文 first-use failure 仍不制造数据或年龄。
- 键盘 Retry 后焦点继续保留；running 时 `aria-disabled=true`，成功后恢复为 `false` 并只清除匹配的 failure notice。

### 📝 总结

- Finding dispositions：`MR-R1-F1 CLOSED`；无 still-open、regressed、superseded 或新增 finding。
- Reviewed state：HEAD `4e203c527ca0d836b95da0a6e71da95ca4ac2846`；`ux/menubar-refresh.md` blob `23420054b81bf4880c4d253c3fc482feb48f7ae4`；prototype manifest SHA-256 `64083a6a4bad190debac0c96547d3648e59b83a3b0bf11c7115ca634783a5752`；内容指纹 `5f8b5af1362844557a12b44b6c4ec32a3732a374d4539c61a3261f2fd51a6805`。
- Reviewer：Codex。
- Method：finding-scoped 设计/契约复评 + Product Design combined UX/accessibility audit；从修复后源码重新捕获中文 first-use failure 和英文 retained-data failure，检查 accessibility tree、截图、键盘 Retry/focus/recovery，复用 Round 1 未受影响的四个状态证据。未委派，不声称原生 VoiceOver 或 AppKit 验收。
- Scope：`MR-R1-F1`、其双语断言、受影响 specimen/checks/manifest，以及新内容状态是否存在相邻回归。未评审 architecture、Widget UX、产品 Swift 实现或任务分解。
- Audit steps：1) 中文首次失败——healthy，accessible name 精确命中；2) 英文旧数据失败/280 pt——healthy，英文名称与窄宽层级未回归；3) 键盘 Retry/恢复——healthy，焦点、disabled 与 notice 清除均符合合同。
- Evidence：`npm --prefix prototype run build` 通过（4583 modules）；`make check-whitespace` 与 `git diff --check` 通过；manifest 的 12 个文件哈希全部匹配，manifest SHA-256 与修复记录一致。本轮临时截图保存在 `/private/tmp/agentdeck-menubar-refresh-rereview.lUl4FC/`，正式 subject specimen 仍由 manifest 绑定。
- Residual uncertainty：浏览器 specimen 仍不证明原生 VoiceOver、Dynamic Type、AppKit focus、真实睡眠或 Increase Contrast；这些保持为后续实现验收边界，不阻止 framework 文档 PASS。
- Completion gate：VERIFIED。固定 CEv1 gate 对 `desktop-refresh:ux/menubar-refresh.md` 和精确 ContentState `urn:agent-deck:content-state:desktop-refresh:ux-menubar-refresh:4e203c5:2342005:64083a6` 回查为 3/3；Round 1 fail evidence 保留在原 ContentState，不适用于本目标；无缺失、反证、blocked、malformed、失效证据或未决 candidate impact。

Round 1 的唯一 finding 已关闭，当前 framework 可进入 Task checkpoint；architecture 仍需在后续阶段处置文档列出的数据要求，final-surface reconciliation 也仍未执行。
