---
status: active
topic: health-recovery
subject: menubar-health-presentation
---

# Health Recovery Menubar Health Presentation Review

## Round 1 — 2026-09-24

## 📋 health-recovery / menubar-health-presentation 评审

📊 总体评分：5/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**MHP-R1-F1** [apps/macos/AgentDeckApp/MenuBarViewModel.swift:520] 只要存在一个可识别的恢复检查，`notices` 就完全跳过原有的健康问题汇总，即使其他未分类检查仍为 error。
- 行为风险：例如同时有 `extension_stale_inventory` warning 和 `provider_configuration` error 时，顶层通知条只呈现库存过期的 warning，原本基于 `health.errors > 0` 的 error 提醒消失；用户可能误以为健康问题仅限可同步库存。详情页仍有错误行，但用户需要先打开它才能发现更严重的问题。
- 证据：`recoveryChecks` 非空后进入第 521–524 行的分支；第 525 行的 `else if` 使汇总行不再执行，且分支没有计算剩余未分类检查。新增 `testHealthRecoveryActionsAreClassifiedAndCopyDoesNotChangeHealth` 的四项输入均为可识别恢复原因，没有覆盖“可识别 warning + 其他 error”。现有 UX 仅在两个健康问题都已作为独立原因呈现时抑制无信息量的 generic 行（`ux/menubar-health-recovery.md` 的 combined 场景）。
- 💡 修复：仅在所有不健康检查都已由分类通知或独立 schema 通知表示时抑制 generic 行；对剩余未分类错误保留相应 severity 的通知，并以混合 warning/error 用例锁定通知集合和顺序。

### 🟡 建议改进（本轮同样必须关闭）

**MHP-R1-F2** [apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift:197; apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift:690] Task 4 要求的健康复制按钮原生焦点保留和 280/420 pt 具体布局尚无有效验收断言。
- 行为风险：复制后 `Copied` 状态重绘可能转移键盘焦点；280 pt 下按钮未填满堆叠行或命令截断时，现有测试仍会通过。焦点与 VoiceOver 风险不能从模型计时器或 PNG 大小推断为通过。
- 证据：ViewModel 测试直接调用 `copyHealthAction`，断言剪贴板、健康数据不变和 1.7 秒后状态清除；没有点击挂载视图中的 Button 或断言 `NSWindow.firstResponder`。新增 Chrome 测试只断言 `fittingSize.width <= width + 1` 与 PNG 字节数大于 2,000，没有断言 420 pt 同行、280 pt 堆叠/按钮宽度或命令换行。交接明确记载曾尝试的 NSButton 焦点测试无法定位 SwiftUI 控件并已移除；Task 4 Verification 明列“Focus retention and 1.6-second timer tests”及两种宽度的布局验证。
- 💡 修复：在实际挂载的健康复制控件上建立可稳定定位的原生/无障碍测试接缝，断言复制前后焦点、可访问标签/状态与 1.6 秒反馈；对 280/420 pt 的实际布局几何或经人工审阅的渲染图给出可复核证据。保留已有模型计时器用例，不将其视为焦点测试。

### 🟢 优点

- 动作构造验证 `reason`、`resource`、精确命令与七项安全前提键的组合；`retry`、未知键及不匹配命令均不提供复制按钮，菜单栏不执行恢复命令。
- 中英文 String Catalog 包含健康原因、七项安全前提、按钮、effect 和未回滚文案；复制后的 1.6 秒模型状态与健康快照不变已有测试。

### 📝 总结

- Reviewed state：`feature/health-recovery` HEAD `581a855a5f5400b479afbb716ed5a6662ba78dc6` + Task 4 八个修改文件；`sha256(git diff --binary 581a855a... -- <本任务八个文件>) = 3788c26334db9651e8a8243e857718f5494c714f4ec9329adccc546c4e3c6491`。并行的 `.agent-instructions/` 与 Hook 修改不计入本候选指纹。
- Reviewer：codex。Method：对照 Task 4、菜单栏 UX 和既有通知行为审读 SwiftUI/ViewModel、复制/本地化与原生测试 diff，构造混合检查路径，并核对焦点与布局断言。Scope：Task 4 菜单栏实现与测试；产品代码、测试、配置只读。
- Evidence：Beads `ad-hr-menubar-health-presentation-dev` 已由实现者交接到 `in_review`，当前八文件指纹与交接相同；上述源码分支和测试断言给出可证伪的缺口。交接报告隔离 macOS 套件通过，但明确保留焦点/VoiceOver 原生风险；发现阻断项后未重复扩大运行套件。
- 结论依据：F1 为用户可见的错误级别漏报；F2 为任务明确要求却尚未验证的原生交互与排布。Task 4 Dev 保持 `[x]`，Review 保持 `[ ]`。
- Completion gate：NOT_VERIFIED；CEv1 WorkUnit `health-recovery:menubar-health-presentation` 的五项必需准则，对 ContentState `urn:ce:agent-deck:content-state:health-recovery-menubar-health-presentation:review:3788c26334db9651e8a8243e857718f5494c714f4ec9329adccc546c4e3c6491` 的门禁查询为 0/5。本轮 FAIL 不写通过证据。

### 下一步指令

`修复：health-recovery / reviews/menubar-health-presentation.md / MHP-R1-F1、MHP-R1-F2`

## 修复记录 — 2026-09-24

本节记录 Round 1 两项发现的修复候选，不改变 Round 1 的 FAIL 结论；是否关闭发现由独立复评决定。

- **MHP-R1-F1**：分类健康通知继续逐项显示；仅对未被分类通知或独立 schema 通知覆盖的问题保留通用健康通知，并从剩余检查及未被覆盖的错误计数确定严重级别。混合 extension_stale_inventory warning 与 provider_configuration failed 的回归测试断言通知顺序、warning/error 严重级别及剩余问题数。
- **MHP-R1-F2**：健康复制按钮改为具有稳定 health.copy.<row-id> 标识的原生 NSButton，复制反馈更新同一控件。挂载窗口测试实际点击该按钮，断言点击前后及 1.6 秒反馈结束后的第一响应者、控件身份、可访问标签和状态。宽度驱动布局在 280 pt 垂直堆叠并铺满按钮行，在 420 pt 将命令与按钮并排；中英文几何断言及四张渲染图覆盖两种宽度。

验证：隔离 HOME 的 bash scripts/test-macos-app.sh 通过（Shared 81、App 127，其中 1 项条件跳过、Widget 45）；make check-whitespace 与 git diff --check 通过。测试结果和四张布局附件保存在 apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.09.24_08-32-24--0700.xcresult。渲染图已逐张审读，命令、换行和按钮未见截断；真实 VoiceOver 语音播报仍需原生人工验收，自动测试只证明可访问标签/状态及通知调用。

候选状态：feature/health-recovery HEAD 581a855a5f5400b479afbb716ed5a6662ba78dc6；Task 4 八文件的 git diff --binary SHA-256 为 395a1fd2d1e0155da274d3847e87fbf7c3257b5576a7fe6680b250b425c93a81。并行规则与 Hook 改动仍不在本任务范围内。两项发现均已提交复评候选，Review 矩阵保持 [ ]，未自行签发 PASS。

### 下一步指令

`复评：health-recovery / reviews/menubar-health-presentation.md`

## Round 2 — 2026-09-24

## 📋 health-recovery / menubar-health-presentation 复评

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题（必须修复）

**MHP-R2-F1** [apps/macos/AgentDeckApp/MenuBarViewModel.swift:529] 修复后的剩余健康通知把“已由分类行呈现的 error”只按字面状态 `failed` 计数，而生产 doctor 的错误检查状态为 `error`。
- 处置：新增，仍开放。Round 1 的“分类 warning + 未分类 error 被漏报”路径已修复；反向混合状态出现错误严重度。
- 行为风险：`extension_inventory_unreadable` 的已分类 `error` 与另一个未分类 `warning` 并存时，`remainingCount > 0`，但 `representedErrors == 0`，`health.errors > representedErrors` 将仅包含 warning 的剩余通用通知错误地标为 error。顶层通知重复强调已分类错误，误导用户判断剩余问题的严重程度。
- 证据：`representedErrors` 仅筛选 `$0.status == "failed"`，而 `internal/doctor/doctor.go:588-596` 只有 `Status == "error"` 才增加 `Errors`；`healthSeverity` 本身把所有非 `ok`/`warning` 状态映射为 error。新增混合用例把 `provider_configuration` 人工设为 `failed`，只验证“分类 warning + 未分类 error”，没有使用生产状态 `error` 验证反向组合。
- 💡 修复：按生产 doctor 的 `error` 状态核算已呈现错误，并让剩余通知的严重度由未被表示的检查决定；补充“已分类 error + 未分类 warning”及 schema error 与 warning 的聚焦用例，断言剩余通知数量和 severity。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- MHP-R1-F1 的原始漏报路径已关闭：分类通知与未分类问题分开统计；“库存 warning + 其他 failed/error”现在保留额外通用通知。新回归测试验证了两条通知的顺序与 warning/error 呈现。
- MHP-R1-F2 已关闭：健康复制控件改为可定位且保持身份的原生 NSButton。保存的 XCTest 结果显示实际点击、第一响应者、可访问标签/反馈和 1.6 秒清除测试通过；280/420 pt 的中英文布局测试通过，四张附件经本轮逐张审阅，命令与按钮未见截断。真实 VoiceOver 语音播报仍属 Task 5 原生人工验收风险，不能由这些自动化结果冒充。

### 📝 总结

- Finding disposition：MHP-R1-F1 的原始 warning/error 漏报场景关闭；MHP-R1-F2 关闭；新增 MHP-R2-F1 开放。无其他新增本任务发现。
- Reviewed state：`feature/health-recovery` HEAD `581a855a5f5400b479afbb716ed5a6662ba78dc6` + Task 4 八个修改文件；`sha256(git diff --binary 581a855a... -- <本任务八个文件>) = 395a1fd2d1e0155da274d3847e87fbf7c3257b5576a7fe6680b250b425c93a81`。本评审记录及并行规则/Hook 改动不计入指纹。
- Reviewer：codex。Method：逐项复核 Round 1 两项发现、通知计算、原生控件与 XCTest 断言，直接查询保存的 xcresult 并审阅 en/zh-Hans × 280/420 pt 四张渲染附件。Scope：Task 4 SwiftUI/ViewModel、本地化与测试；产品代码、测试与配置只读。
- Evidence：Beads `ad-hr-menubar-health-presentation-dev` 为 `in_review`，修复交接指纹与当前候选一致。`Test-AgentDeck-2026.09.24_08-32-24--0700.xcresult` 中健康复制焦点/反馈和双宽度布局用例均为 Passed；MHP-R2-F1 由当前源码状态比较与生产 `doctor.Report.add` 的 `error` 计数规则直接复现。发现阻断项后未重跑完整原生套件。
- 结论依据：原两项发现已关闭，但新剩余通知严重度错误仍属本任务改动。Task 4 Dev 保持 `[x]`，Review 保持 `[ ]`。
- Completion gate：NOT_VERIFIED；CEv1 WorkUnit `health-recovery:menubar-health-presentation` 对 ContentState `urn:ce:agent-deck:content-state:health-recovery-menubar-health-presentation:rereview:395a1fd2d1e0155da274d3847e87fbf7c3257b5576a7fe6680b250b425c93a81` 的门禁查询为 0/5。本轮 FAIL 不写通过证据。

### 下一步指令

`修复：health-recovery / reviews/menubar-health-presentation.md / MHP-R2-F1`

## Round 2 修复记录 — 2026-09-24

MHP-R2-F1 的修复候选现按生产 doctor 的 status: error 统计已由分类行或独立 schema 通知表示的错误；剩余通用通知的严重度由尚未表示的检查决定。原“分类 warning + 未分类 error”用例改用生产状态 error，并新增“分类 error + 未分类 warning”和“schema error + hook warning”两项回归测试，分别断言通知集合、顺序、剩余数量与严重级别。

隔离 HOME 的 bash scripts/test-macos-app.sh 通过：Shared 81、App 129（1 项条件跳过）、Widget 45；测试结果位于 apps/macos/build/DerivedData/Logs/Test/Test-AgentDeck-2026.09.24_22-31-50--0700.xcresult。make check-whitespace 与 git diff --check 通过。Task 4 八文件的 git diff --binary SHA-256 为 f44f85571442f54729c1a3fcc92d7cfa24bd2e6c8533456569c81bfe9aa0637e，HEAD 仍为 581a855a5f5400b479afbb716ed5a6662ba78dc6。

本记录仅交接修复候选；Round 2 的 FAIL 结论和 Task 4 Review [ ] 不在修复阶段改写。MHP-R2-F1 是否关闭、CEv1 门禁是否通过，仍由独立复评裁定。

### 下一步指令

`复评：health-recovery / reviews/menubar-health-presentation.md`

## Round 3 — 2026-09-24

## 📋 health-recovery / menubar-health-presentation 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（本轮同样必须关闭）

无。

### 🟢 优点

- MHP-R2-F1 已关闭：`representedErrors` 现在按生产 doctor 的 `status: error` 计数；剩余通用通知由未呈现检查及剩余错误数决定严重度。测试覆盖“分类 warning + 其他 error”“分类 error + 其他 warning”及独立 schema error + hook warning，断言通知顺序、数量和 severity。
- MHP-R1-F1 与 MHP-R1-F2 保持关闭。健康复制按钮的原生焦点、可访问标签/反馈和 1.6 秒恢复测试以及 en/zh-Hans 的 280/420 pt 布局测试在保存的 XCTest 结果中通过；前轮审阅的四张布局附件仍适用于未变的布局代码。

### 📝 总结

- Finding disposition：MHP-R1-F1、MHP-R1-F2、MHP-R2-F1 全部关闭；无仍开放、回归或新增的本任务发现。
- Reviewed state：`feature/health-recovery` HEAD `581a855a5f5400b479afbb716ed5a6662ba78dc6` + Task 4 八个修改文件，Review 状态同步后 `sha256(git diff --binary 581a855a... -- <八个文件>) = 343e6a4c632b02a84a5645270ac8c50a05bde48771732f46666349c79adbeed3`。评审记录及并行规则/Hook 改动不计入此指纹。
- Reviewer：codex。Method：逐项复核 Round 1/2 发现，核对生产 `doctor.Report.add` 的错误状态、ViewModel 通知计算与三组混合状态断言，并直接查询当前候选保存的隔离 XCTest 结果。Scope：Task 4 SwiftUI/ViewModel、双语复制和原生测试；产品代码、测试与配置只读。
- Evidence：当前八文件修复指纹 `f44f85571442f54729c1a3fcc92d7cfa24bd2e6c8533456569c81bfe9aa0637e` 与 Beads 交接一致；`Test-AgentDeck-2026.09.24_22-31-50--0700.xcresult` 的测试摘要为 254 passed、1 conditional skipped、0 failed；三组混合通知及焦点/布局测试节点均为 Passed。最终指纹与测试时相比仅多 `tasks.md` Review 勾选，产品代码、测试、配置、fixture 与原生环境未改变。`make check-whitespace` 与 `git diff --check` 在修复交接时通过；最终状态另行确认。
- 结论依据：混合健康状态的通知严重度符合生产错误词表，原生复制与布局证据仍有效。真实 VoiceOver 语音播报尚未由自动化证明，保留在 Task 5 原生验收风险中，不记作本任务技术 PASS。
- Completion gate：VERIFIED（5/5）。CEv1 WorkUnit `health-recovery:menubar-health-presentation` 对 ContentState `urn:ce:agent-deck:content-state:health-recovery-menubar-health-presentation:rereview:343e6a4c632b02a84a5645270ac8c50a05bde48771732f46666349c79adbeed3` 的精确目标门禁查询确认五项必需准则均有目标绑定的 pass evidence，无缺失、反证或未决候选影响；WorkUnit 已回读为 Review PASS、门禁 VERIFIED。此结果不代替提交授权或 Task 5 原生人工验收。

Task checkpoint：`ad-hr-menubar-health-presentation-dev`；内容状态 `343e6a4c632b02a84a5645270ac8c50a05bde48771732f46666349c79adbeed3`；门禁 VERIFIED。
提交建议：单独提交 Task 4 的八个 Swift app、String Catalog、测试及 `tasks.md` 文件与本评审记录；排除并行的 `.agent-instructions/` 和 Hook 改动，提交前核对贡献者、暂存范围、完整消息及 SSH 签名。
推送建议：取得单独推送授权、完成并验证签名提交后，确认远端目标，再推送 `feature/health-recovery` 到 `origin/feature/health-recovery`。

### 下一步指令

`开发：health-recovery / health-recovery-acceptance`

WORKFLOW_WORKSPACE: agent-deck.health-recovery
