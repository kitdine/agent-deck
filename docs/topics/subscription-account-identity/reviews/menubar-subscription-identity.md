---
status: active
created: 2026-11-19
---

# Menu bar subscription identity review

## Round 1 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Codex Astra 评审配置为 `reasoningEffort=ultra`。对象为 HEAD `424fe8a01ebad097a112323e5d3945918296443b` 上的 92 路径候选 `2c7d6c6bec34f313a31bb11a0d822fb1f4b5528893410d1bb6bfbe05d4725642`，审核范围为任务 5 和已批准的菜单栏 UX。评审核对作者 hosted App 回执，共 270 项，5 项跳过，0 项失败。

- **SAI-MENUBAR-R1-F1 · P1**。M10，在同一可见宿主窗口中超过 `current_until` 后，模型已判定身份过期，卡片仍显示当前登录和“与此卡额度为同一账号”。原有测试只向模型传入未来时间，以及检查调度数组。回归须在保持原卡片打开且快照不变时验证可见文案和合并后的 AX 名称均撤回确认。
- **SAI-MENUBAR-R1-F2 · P1**。280 pt 时，16 个宽字形组成的安全原值触发套餐胶囊固定宽度，挤掉额度来源和时间。回归须实际渲染宽原值与 `Pro 100` 两个对照，验证来源、读取时间和胶囊截断，紧凑套餐保持单行。
- **SAI-MENUBAR-R1-F3 · P2**。Q6，两侧均未探测时，当前 producer 发送含 `never_probed` 的身份对象，卡片渲染额外身份区。回归须验证隐藏该区域，另以已有额度、仅身份未读取的输入验证仍显示对应提示。

已核对 M1–M9、Q1–Q5、Q7 的相应代码与用例，以及中英文 29 组文案。额外证据边界为独立的身份观察与尝试时间、长邮箱专属后缀的识别，以及按来源 issue 分别验证可构建的交付候选。安装 App 的菜单栏窗口、系统朗读与通知验收由任务 7 负责。A4 文案对齐在架构记录 Round 10 单独裁决。

### Round 1 修复验证

主控修复候选为 HEAD `6c7b5d407cd23f135718b3121b29814160194612` 上的 92 路径 `4ec31b018e181a7cf9b54e351acfb19c5da1b27caf2e6e90ce7b826e52eead6c`。对比原评审候选，变更限 `MenuBarPanelViews.swift` 和 `MenuBarChromeTests.swift`。HEAD 间的两个提交只交付架构和任务分解文档与评审记录。

- **F1**。可见卡片在有效期结束时更新本地状态，按同一截止时间重新生成文案和 AX。测试保留打开的宿主窗口及原快照，验证先显示当前确认，到期后显示上次识别的登录和未确认归属。
- **F2**。来源、读取时间和胶囊共享标题行内宽，宽原值按空间省略。280 pt 渲染验证两种输入的来源与时间，以及 `Pro 100` 同一行内的完整文本。
- **F3**。仅在额度与身份均处于首次探测前时省略身份区域。已有额度的对照仍显示尚未读取身份。

原实现上三个回归均失败。修复后三个回归均通过。菜单栏与文案组合运行共 61 项，其中一项渲染矩阵因 OCR 对字体和断行的识别出现断言失败。改用独立顶级域、同一行的邮箱首尾以及规范化的简繁体识别后，在两种语言、两种主题、280／420 pt 重跑渲染矩阵通过。独立时间用例让额度读取、身份读取与身份尝试各用不同时间。其余 60 项的相关代码和断言保持原通过结果。独立复评与任务 7 实测仍待实际结论。

## Round 2 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Codex Astra 按 `reasoningEffort=ultra` 复评 HEAD `6c7b5d407cd23f135718b3121b29814160194612` 的 92 路径候选 `4ec31b018e181a7cf9b54e351acfb19c5da1b27caf2e6e90ce7b826e52eead6c`。核对五份验证回执，以及两次文档提交的 SSH 签名与范围。

- **SAI-MENUBAR-R1-F1 CLOSED**。同一张卡片保持显示，跨过实际截止时间后，可见文案和 AX 均撤回同账号确认，显示上次识别的登录和未确认归属。快照替换的取消路径经源码核对。
- **SAI-MENUBAR-R1-F3 CLOSED**。额度与身份均未探测时省略身份区。已有额度而身份未读取的对照保留提示。
- **SAI-MENUBAR-R1-F2 OPEN，关联 SAI-MENUBAR-R2-F1**。280 pt 原图和对照验证了宽原值省略、来源及时间可读、`Pro 100` 完整单行。读取年龄的旧断言分别在整张卡片查找 `Read` 和 `ago`。到次日时，正确的 `Read yesterday` 会失败。若读取年龄丢失，重置行的 `6 hours ago` 又可满足旧断言。回归须区分读取与重置时间，在来源后检查完整的读取年龄，并允许正常换行。

A4 文案裁决沿用架构 Round 10 的 PASS。Claude 来源与其上的 Codex 叠加候选仍须分别验证；安装 App 验收由任务 7 覆盖。

### Round 2 修复验证

修复候选为同一 HEAD 上的 92 路径 `ea44c251b7ea86c7c2ac1911f477dd83f001e5637b36dbe9ce6a071912e3df8a`。相对 Round 2，唯一变更为 `MenuBarChromeTests.swift`，blob `f93f2befb28d4ded998bc6fbd71c9da9077a346a`。

280 pt 用例以测试开始时刻生成不同的额度读取时间和窗口重置时间。OCR 将字号导致的换行和标点差异归一为词序列，断言额度来源后有完整的读取年龄。宿主定向用例通过，覆盖宽原值及 `Pro 100`。另以同一词序列判据验证五个对照，接受换行的 `Read yesterday` 和正常的完整年龄，拒绝省略年龄、同龄重置行、以及脱离来源的读取行。两份测试回执均退出 0。独立复评待实际结论。

## Round 3 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

独立 Codex Astra 按 `reasoningEffort=ultra` 复评 HEAD `6c7b5d407cd23f135718b3121b29814160194612` 上的 92 路径候选 `ea44c251b7ea86c7c2ac1911f477dd83f001e5637b36dbe9ce6a071912e3df8a`。候选源文件和回执与主控逐项核对。与 Round 2 的差异为 `MenuBarChromeTests.swift`，blob `f93f2befb28d4ded998bc6fbd71c9da9077a346a`。

- **SAI-MENUBAR-R2-F1、SAI-MENUBAR-R1-F2 CLOSED**。280 pt 的两个实际渲染夹具具有不同读取和重置年龄。OCR 检查额度来源旁完整的 `Read` 和读取年龄，覆盖正常换行。宽原值显示可见前缀并截断，`Pro 100` 在同一行。独立复跑五个判据对照，验证“昨天”、完整年龄、年龄缺失、重置年龄干扰及来源与读取行分离。
- **SAI-MENUBAR-R1-F1、SAI-MENUBAR-R1-F3 沿用 CLOSED**。沿用 Round 2 对可见卡片实际到期、AX 撤回及 Q6 对照的复评。A4 文案沿用架构 Round 10 的裁决。

联合实现状态 `subscription-account-identity:menubar-subscription-identity:state:ea44c251b7ea86c7c2ac1911f477dd83f001e5637b36dbe9ce6a071912e3df8a` 通过 CEv1 5/5。门禁绑定本任务的 hosted XCTest、图像和 AX 树证据。按来源的交付候选编译与叠加验证遵循任务分解的交付顺序。安装 App 的菜单窗口、实际 VoiceOver 和通知实测由任务 7 验收。

### 来源切片准备

主控在两份 Task 4 准备树 `11ae22021fd74b32b7be0404ac7b72a05f1ae85b` 和 `2c7c65ed9ff2aa766f2a25a06b3e2219a48c4133` 上核对四个相同前像。独立重放共享层与 Codex 增量，验证三个结果树 `f2346be4a59998bf176795950128ce392ca09b43`、`1362ddc955298dba102dd85754592109509a6045` 和 `4e780523fe4bccdea56b384ea0a760140cf1c9f4`，零模糊、零偏移。最终四个文件均匹配 Round 3 后像。

三个叠加状态各通过 34 项宿主测试，分别为 23 个菜单栏用例与 11 个文案用例。共享层针对同一个打开的 Claude 卡片验证到期后可见文案和 AX 改标。Codex 补丁把这个测试替换为已复评的到期后 Codex 登录改标和关联撤回用例。复评需明确核对该替换对来源归属和组合回归保护的影响。

两个来源切片待独立复评。最终备签时按正式 Task 4 的树和实际编译输入重绑定。

## Round 4 — 2026-11-19，来源切片复评

复评结论：FAIL

完成门禁：NOT_VERIFIED

独立 Codex Astra 复核共享产物树 `f2346be4a59998bf176795950128ce392ca09b43`、组合树 `4e780523fe4bccdea56b384ea0a760140cf1c9f4` 和三个 34/34 的宿主回执。T3 配置 `reasoningEffort=ultra`。共享层通过，包括旧 Codex usage-only 计划回退、Claude 打开卡片到期时的可见文案与 AX 改标。Codex 生产增量的工作区标签、当前身份关联和套餐映射均符合分工。

**SAI-MENUBAR-R4-F1 OPEN。** Codex 补丁替换了共享的 `testOpenQuotaCardRelabelsExpiredSignIn`，组合只执行 `testOpenQuotaCardWithdrawsExpiredAssociation`。如果到期更新只对 Codex 调度，现有组合用例仍可通过，打开的 Claude 卡片会保留当前登录措辞。最小修复为原样保留首个方法并在 Codex 层新增第二个，同时在同一组合构建中选定两个用例，要求执行 2 项、零失败、零跳过。

Round 3 的同字节生产实现证据继续适用。两个来源的正式提交仍须绑定签名 Task 4 基线。

### R4-F1 修复与定向验证

主控保留共享 `testOpenQuotaCardRelabelsExpiredSignIn` 与已评审的 Codex `testOpenQuotaCardWithdrawsExpiredAssociation`，两个方法逐字节沿用各自回执的输入。修复后的 Codex 补丁保留共享方法，并增加 Codex 到期用例。三个生产后像均匹配 Round 3。相较旧组合，差异仅为测试文件恢复共享方法。修复组合树为 `c8c7f22628098a0210eae5a5e286f373db572322`，测试 blob 为 `57bd6cb29ddd6b2d2005bcf32f5f59a896aa00f9`。

全新组合构建只选两个到期用例，执行 2 项、零失败、零跳过。更新的组合目录含 24 项菜单栏与 11 项文案用例。独立复评提出的修正条件已由主控直接核对。正式交付仍按对应来源的签名 Task 4 树及编译输入绑定。

**SAI-MENUBAR-R4-F1 CLOSED（修复组合）。** 两个旧测试方法完整保留，原独立复评要求的同次执行已核对。Codex 叠加随后在其实际前置提交上绑定。

### 共享来源交付绑定

Task 4 已签名基线 `8c63d91b46649a45dd525d4fed1a4a1df3d6bc60` 与经测产品树 `11ae22021fd74b32b7be0404ac7b72a05f1ae85b` 的差异限 Task 4 的两个文档记录。共享补丁在签名基线四路精确前后像上无偏移、无模糊重放，结果树 `2dc85e228da2c2b7a0c3cd7e5e6412c233d3a159`。Round 4 的共享 PASS 和 34 项宿主证据适用。共享来源观测绑定 `subscription-account-identity:menubar-subscription-identity:state:b254a717ae899a2e4fb2bd24d22a0858f0ed621d21ad7db4e6ab5eca6f0d8ded`。对应 Codex 全组合证据须按实际 Codex 前置提交继续绑定。
