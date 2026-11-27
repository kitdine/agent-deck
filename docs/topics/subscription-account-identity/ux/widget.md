---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — Widget

范围：额度 Widget（small、medium、large）的套餐展示名、large 的当前登录行与登录上下文失效降级。尺寸、选端、窗口行、新鲜度页脚和归属短句沿用 [subscription-quota 的 Widget 额度 UX](../../subscription-quota/ux/widget-quota.md)。本文与原型冲突时以原型为准。

## 1. 是否显示账号

用户 2026-11-17 决定（D1）：只有 large 显示脱敏邮箱；small、medium 不显示任何账号信息。任何尺寸都不显示组织或工作区名称。

约束：

- Widget 常驻桌面，处在录屏、共享屏幕和旁人可见的环境里，因此只出现已脱敏的邮箱，形状与菜单栏相同（如 `a***e@example.test`），完整邮箱不进入 App Group 快照。
- 邮箱不能读成「这些数字属于这个账号」。large 的邮箱放在每个客户端块底部的独立行，带 `当前登录` 前缀，与额度数字之间隔一条细分隔线；Claude 块的 `账号未确认` 短句保留在标题行，不因显示邮箱而删除。
- large 仅在 `identified`、`freshness = current` 且渲染时刻早于有效的 `current_until` 时显示邮箱。

## 2. 各尺寸规则

| 尺寸 | 套餐 | 当前登录 | 归属 | 失效 / 未知 |
| --- | --- | --- | --- | --- |
| small | 不显示 | 不显示 | Claude `账号未确认`（不变） | 选定端失效时显示无数据，页脚说明失效原因 |
| medium | 不显示 | 不显示 | 同上 | 同上 |
| large | Codex 标题旁显示额度侧 `plan_display.short_name`，长名按可用宽度省略 | 每个符合 §1 条件的客户端块底部显示一行脱敏邮箱 | Claude 标题保留 `账号未确认`。Codex 登录行内可标出关联未确认 | 失效的端不生成块。有可用块时页脚显示观测时间。两端都无块时显示 `无额度数据`，其中任一端已知失效则在页脚说明实际原因 |

Claude 块不显示套餐：Claude 套餐来自 auth status，属于身份侧，放在额度数字旁读起来像「这些数字属于 Pro 账号」。菜单栏把它放在独立身份区域；Widget large 的当前登录行只放邮箱，不加套餐。

Codex large 的登录行在关联未确认时追加 `与额度关联未确认`，不另占一行。只有本次快照提供 `confirmed` 时省略提示。额度仍按 accountId 隔离。

## 3. 降级规则

- 旧快照缺少 `plan_display` 键时回退到 `plan`。显式 null 或对象没有 `short_name` 时省略套餐。安全的未知原值按投影显示。
- `identity` 缺失、为 null 或含未知枚举时，large 省略当前登录行，其余渲染不变；small、medium 不读取 `identity`。
- `quota_context` 存在时该端无可用窗口，原因取 `quota_context.reason`。small 和 medium 保留选端。首个新 usage 成功提交并清除 `quota_context` 后显示新窗口。
- 读取关闭沿用 `未读取`。

## 4. 文案

| Key | zh-Hans | en-US |
| --- | --- | --- |
| `widgets.quotaContextChanged` | 登录已变化 | Sign-in changed |
| `widgets.quotaIdentityKeyReset` | 旧额度无法关联 | Prior quota unlinked |
| `widgets.quotaSignedIn(email)` | 当前登录 {email} | Signed in {email} |
| `widgets.quotaAssociationUnconfirmed` | 与额度关联未确认 | quota unconfirmed |

页脚与当前登录行宽度必须在原型 `&measure=1` 下按中英文两种语言、三个尺寸验证。邮箱超宽时原生实现用中间截断，保住首尾与顶级域；原型受 CSS 限制以尾部省略近似。

large 的客户端块限制在卡片宽度内。长套餐或邮箱可省略，窗口百分比和归属保留在卡片内。浏览器验收分别测量这些可读元素的边界。

登录行的无障碍名称使用与菜单栏相同的口语脱敏形，并朗读适用的关联提示。原型中新增文字使用 `--muted`，应在两个客户端的深浅背景上达到 4.5:1。

## 5. 与菜单栏的一致性

- 套餐紧凑名使用同一个 wire 字段 `plan_display.short_name`。
- 脱敏邮箱使用同一个 `identity.email_masked`，不在 Widget 侧再脱敏或还原。
- 失效判定使用同一个 `quota_context`。
- 归属短句仍为 `账号未确认`。
- 菜单栏显示「已失效」时，Widget 不会仍显示旧数字。

## 6. 原型改动

| 文件 | 位置 | 改动 |
| --- | --- | --- |
| `prototype/src/Widgets.jsx` | `QuotaClientBlock` 标题 | 按 §3 选择 Codex 套餐，超宽时省略 |
| `prototype/src/Widgets.jsx` | `QuotaClientBlock`（large） | 渲染当前登录行、关联提示和口语脱敏名称，条件见 §1–2 |
| `prototype/src/Widgets.jsx` | `QuotaWidget` 的 `available` 过滤、`noDataReason`、`QuotaNoData` | 排除 `quota_context` 已失效的端；页脚原因支持 `widgets.quotaContextChanged` |
| `prototype/src/i18n.js` | widgets 段 | 新 key |
| `prototype/src/probe.js` / 量具 | 额度 Widget | 检查尺寸边界、脱敏与朗读、过期、新窗口恢复及长套餐 |

可用 `?surface=widgets&quota=<state>&widgetClient=codex|claude` 复现。选端只作用于 small 和 medium。

| 状态入口 | 应观察的行为 |
| --- | --- |
| `identityUnconfirmed`、`identityConfirmed` | large 的 Codex 登录行按关联状态切换提示。small、medium 不显示账号 |
| `identityUnknown`、`expiredMailbox` | large 省略旧邮箱，额度仍可读 |
| `contextChanged`、`signedOut`、`claudeApiKey` | large 保留 Codex。选 Claude 的 small、medium 显示 `登录已变化` |
| `contextKeyReset` | large 保留 Codex。选 Claude 的 small、medium 显示 `旧额度无法关联` |
| `contextRecovered` | Claude 的新窗口重新可见 |
| `longPlan` | large 省略 64 字符合成展示名，保留窗口百分比与归属文案 |

## 7. 数据需求

客户端级需要 `plan_display` 的状态和紧凑名，以及 `quota_context.reason`。large 需要 `identity.state`、`identity.freshness`、`identity.current_until`、`identity.email_masked`。App Group 不写入组织资料。

large 的 Codex 关联提示使用 A11 允许的既有 `quota_association` 两态枚举。原型展示两种关联状态。快照缺少证明时按未确认渲染，原生 App Group 读写待实现验收。

## 8. 样例不能证明的内容

WidgetKit 真实渲染，以及 App Group 中新旧快照混读的行为，转为手工验收与原生单测。
