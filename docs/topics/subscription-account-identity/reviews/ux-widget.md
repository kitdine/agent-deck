---
status: active
created: 2026-11-17
topic: subscription-account-identity
subject: ux/widget.md
---

# 订阅账号与套餐辨识 — Widget UX Review

## Round 1 — 2026-11-17

对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、Widget 文档 blob `241de41619d4e7680764518ef3eb7f92234a2e08`。七个原型文件的有序 `路径 blob` 清单 SHA-256 为 `35a96456f08e12e1a654266b37b1cb7e184d05b9f7eb5d56d6c55c4be8a54962`，`git diff -- prototype` SHA-256 为 `6b977a7e24fafc902acba93a33f092083db00b09218f6c135ee8dc930f6e6edd`。

方法为冷独立 OpenCode `openai/gpt-6.2-astra`，实际 variant 未暴露。Reviewer 渲染中英文、两种舞台宽度、选端和三个尺寸，核对常规态和合成的长套餐，使用浏览器无障碍树和 axe 对照。主控核对文档和原型指纹，复核客户端标题、邮箱可见条件与次级文字样式。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 7/10。CEv1 对本轮精确目标返回 `missing_work_unit`。

### 发现

- **SAI-UX-R1-F1（高，待修）**：合成的无空格 64 字符未知套餐将 large 客户端块内容推出卡片约 84 px，百分比不可见，Claude 归属文字被裁。套餐用可收缩宽度和省略，优先完整保留数值、时间和归属。相关菜单栏证据见 [菜单栏 Round 1](ux-menubar.md)。
- **SAI-UX-R1-F4（中，待修）**：large 的两种 Codex 关联状态登录行完全相同，和旁边带未确认说明的 Claude 一起看容易被理解为已确认。未确认状态应在现有登录行内清楚表达，保留登录与额度的独立层级。
- **SAI-UX-R1-F7（中，待修）**：large 的无障碍树暴露带星号的字面邮箱，和菜单栏已定义的口语形不一致。复用已脱敏的口语形，视觉继续使用星号。本轮只取得了浏览器无障碍树。
- **SAI-UX-R1-F8（中，待修）**：深色 Claude 块新增的 9 px 邮箱文字 `#7f8995` 对背景 `#202131` 为 4.46:1。同块还有三条既有次级文字被 axe 标出。新行应达到 4.5:1，并用两个客户端可用的实际背景复测。
- **SAI-UX-R1-F10（中，待修）**：缺少可按状态、选端和尺寸打开的样例索引。应写明每个入口与可见预期，包括无身份、失效、过期、已恢复和长原值。

### 证据与修复边界

中英文 Widget 探针各 8/8 PASS，常规 contract 每组 24/24，Codex Plus 每组 36/36。量具 80/80 个常规组合无溢出，F1 来自额外的长原值实测。small 和 medium 未出现邮箱，large 只显示脱敏邮箱，三个尺寸均未显示组织；已知 Claude 失效仍保留 Codex 数值。复评时核对 `current_until`、套餐回退和 App Group 最小字段声明。视觉证据覆盖浏览器原型。

### Round 1 修复交接

原型增加 large 的内联关联提示、口语脱敏名称、到期省略和独立样例入口。A4 旧套餐回退、合成 64 字符展示名与隐私反例已同菜单栏一起验证。F4 的原生数据依赖写入 UX §7，当前架构 A11 仍排除 `quota_association`，需在原生实现前与允许列表对齐。

候选 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。Widget 文档 blob `4e1554ed158a51ba2e53d4ce6657eddcfd93ce68`。原型八文件指纹 `5d190386f6d06cb92d6c7cbcdcd60082d89d1509990caee864bc78616806ad9f`，与菜单栏相同。主控已核对生产构建、i18n、回退逻辑与脱敏断言。真实浏览器尺寸、对比度和无障碍树仍待本候选的独立复评。

## Round 2 — 2026-11-18

独立 OpenCode 复评核对上述 HEAD、文档 blob 和原型指纹，并以架构 blob `57cd0606281635186ec14ae1fab1fdf46c254376` 为对照。Widget 探针中英文各 17 PASS、1 FAIL，常规契约 24/24、Codex Plus 36/36。CEv1 返回 `missing_work_unit`。

评审结论：FAIL

完成门禁：NOT_VERIFIED

- **SAI-UX-R2-F3（高）**。`longPlan` 在 286 × 330 的 large 两种语言和两种选端均复现。容器可用宽 284 px，滚动宽 465 px，五个百分比和 Claude 归属越出 173.83 px。50 字符合成域名也能独立触发。须约束分栏与客户端块的最小宽度，并用实际百分比及归属的边界复验。
- **SAI-UX-R2-F4（中）**。Codex 不适用且 Claude 被丢弃时，large 仍写「尚未成功探测」。小中号在 `identity_key_reset` 时说登录已变，失效依据只证明旧额度无法关联。页脚应准确反映至少一个已失效的可用来源和实际原因。

R1-F4 的视觉和 AX 状态已通过，原生关联字段仍待架构 A11。R1-F7 在 Chromium AX 通过，R1-F8 新文字均达到 4.5:1，已有三个暗色节点同基线。R1-F10 样例覆盖通过。浏览器证据不包含原生 WidgetKit、VoiceOver 或 App Group I/O。

### Round 2 修复交接

large 分栏使用有宽度约束的单列网格。隔离 Chromium 中，64 字符套餐和 50 字符域名分别按宽度省略。两种语言的实际滚动宽与可用宽均为 284 px，五个百分比与 Claude 归属的最右边界留有 18 px。Codex 不适用时三个尺寸仍提示已知 Claude 失效。密钥重置显示「旧额度无法关联」。中英文 Widget 探针各 19/19。架构候选 A11 已列入 Codex 关联枚举，实际 App Group 读写待原生验收。

复评候选使用 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、Widget 文档 blob `5bf755a7e0b038d846d23b9c808984ac5ea5b355`、架构 blob `0e1fe0a0978952aa2ec28b64e9ec0d0a0e112ef3` 和原型八文件指纹 `f3777c6cf6a149fb64415f65468b40b112a373d2a44835484639916b382086a4`。两项发现待独立复评。

## Round 3 — 2026-11-18

Claude Opus 5.5 冷独立复评，请求 effort 为 high，实际档位未观察。核对上述 HEAD、文档 blob 和原型指纹。隔离 Chromium 测量该候选的中英文 Widget 与 AX 树。CEv1 返回 `missing_work_unit`。

评审结论：FAIL

完成门禁：NOT_VERIFIED

- **SAI-UX-R2-F3 CLOSED**。64 字符套餐、50 字符脱敏域名及 Codex Plus 六个百分比的 large 样例在 286 × 330 中均无溢出。卡片滚动边界为 284 × 328，百分比和 Claude 归属距右缘至少 18 px。
- **SAI-UX-R2-F4 CLOSED**。Codex 不适用且 Claude 失效时，三个尺寸正确区分登录变化与密钥重置。A11 的关联枚举和 D1 对账有效。
- **SAI-UX-R3-F1（中，待修）**。Widget UX §2 写成任一端失效即说明原因。原型在 Codex 仍有数据、Claude 失效时显示 Codex 的观测时间，且符合 §6 状态表。明确 §2 的失效原因页脚只用于两端都没有可用块的情况。

同一原型的 19/19 探针、生产构建与 i18n 证据可以复用。新发现是设计文档口径不一致。原生 WidgetKit 与 App Group I/O 留待实现阶段。

### Round 3 修复交接

主控只修订 §2 large 页脚的一句。有可用块时保留观测时间，两端都无块且存在已知失效时说明失效原因。反向替换该句后，文档重新得到 Round 3 已测 blob `5bf755a7e0b038d846d23b9c808984ac5ea5b355`。原型指纹 `f3777c6cf6a149fb64415f65468b40b112a373d2a44835484639916b382086a4` 仍适用，复用两种布局的隔离浏览器实测。

候选 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、Widget 文档 blob `cb324bd5cb2bcd2afb059e374f28c789baf3bbd3`，指纹 `8d32ae5d3d3bda13f832754168fd700e88eef4dbb6b03c37bb8ca03e72048ad2`。文档结构与空白检查通过，单项文案待独立复评。

## Round 4 — 2026-11-18

对象为上述 HEAD、文档 blob 和指纹。T3 确认独立 Claude Sonnet 5.5 执行结束，要求的 effort 为 low，实际档位未单独观察。Reviewer 检查 §2、§3、§6、Widgets.jsx 与 Round 3 浏览器实测。主控重算正式的八文件 Git blob 指纹，并反向替换唯一修改句后复现原文档 blob，补齐 reviewer 未完成的候选校验。

评审结论：PASS

完成门禁：VERIFIED

本候选 4/4。浏览器证据由旧内容状态汇入本轮，单句差异与原型身份均已核对。

- **SAI-UX-R3-F1 CLOSED**。有可用块的 large 页脚显示现有额度的观测时间。两端均无块且有已知失效时显示相应原因。§2、§3、§6 与过滤及页脚选择逻辑一致。

原型八文件指纹仍为 `f3777c6cf6a149fb64415f65468b40b112a373d2a44835484639916b382086a4`。本轮复用相同代码的中英文渲染和布局证据，保留 A11 和 D1 的字段范围。原生 WidgetKit、VoiceOver 和 App Group I/O 属于实现验收。
