---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — 菜单栏

范围：菜单栏「额度」页每张客户端卡片新增的「当前登录」区域、套餐展示名、登录上下文失效与 usage 解析细分文案。页签条、卡片顺序、窗口行、重置三语义、额度时间与 [subscription-quota 的菜单栏额度 UX](../../subscription-quota/ux/menubar-quota.md) 相同，本文不重述，只写差异。

本文是 surface framework；字段由 [`architecture.md`](../architecture.md) 供给或拒绝。本文与原型冲突时以原型为准。

## 1. 样例来源

样例来自 [`/prototype/`](../../../../prototype/)，即共享产品原型。下表的 `quota=` 变体由本主题新增到原型。

| 状态 | 原型入口（额度状态 / Quota state） |
| --- | --- |
| M1 两端身份已识别；Codex 关联未确认 | `identityUnconfirmed` |
| M2 Codex 身份与额度关联已确认（需共同键证据；当前样本不可达，仅用于状态覆盖） | `identityConfirmed` |
| M3 身份未知 / 上次读取 | `identityUnknown` |
| M4 Claude 登录上下文变化，旧额度失效 | `contextChanged` |
| M5 Claude 退出订阅登录或切到 API key | `signedOut`、`claudeApiKey` |
| M6 Claude 套餐来源冲突 | `planConflict` |
| M7 usage 解析失败细分 | `parseFailed`（格式异常）、`parseFailedMissingLine`（缺行）、`parseFailedTimezone`（时区无法解析） |
| M8 身份关联秘密重置 | `contextKeyReset` |
| M9 失效后收到新 usage | `contextRecovered` |
| M10 身份到期 | `expiredMailbox` |
| M11 合成的 64 字符套餐展示名 | `longPlan` |
| 既有 Q1–Q7 | 不变；读取关闭、门禁和从未探测时不出现身份区域 |
| 窄宽度 / 英文 | stage bar 选 `280 pt` / `EN`，加 `&measure=1` 运行量具 |

## 2. 布局与层级

卡片自上而下：

1. 标题行（已有）：客户端名称（统一用 `DesktopPresentationModelV1.clientDisplayLabel`）、额度侧套餐胶囊、额度来源 · 额度读取时间。
2. Claude 归属说明行（已有，文案不变）：`无法确认账号归属`。
3. 窗口行（已有）；登录上下文失效时由一行「已失效」状态替代（§4）。
4. 官方重置次数 / 本地观察到重置（已有）。
5. 当前登录（新增）：放在卡片底部，上方一条分隔线，自带标题。

放在底部而不是标题下面：放在标题下，邮箱会读成「这张卡的主人」，即额度属于这个邮箱，这正是需求禁止的说法。底部配独立标题和独立时间，读者看到的是两件并列的事实：这是额度，那是当前登录。额度读取时间仍紧挨窗口行，不被身份区域隔开。

当前登录区域内部：

| 行 | 内容 | 出现条件 |
| --- | --- | --- |
| a | 当前且已识别时显示 `当前登录` 与脱敏邮箱。保留旧邮箱时改用 `上次识别的登录`。无邮箱时，视觉与无障碍名称直接显示状态。上次读到的退出或非订阅状态带 `上次读取` 提示 | 身份对象存在 |
| b | 组织或工作区名称与 `登录套餐` 展示名，英文使用 `Organization`、`Workspace` 和 `Sign-in plan` | 名称存在，或身份侧套餐与额度侧不同 |
| c | 读取时间和失败原因。旧邮箱的这行紧接在邮箱之后 | 有读取时间或失败原因 |
| d | Codex 显示 `与此卡额度的账号关联未确认` 或 `与此卡额度为同一账号`。确认文案要求身份仍在有效期 | Codex 有可显示的脱敏邮箱 |

标题胶囊显示额度侧套餐的紧凑名，例如 `Pro 100`。帮助与无障碍名称给出完整名和安全原值，例如 `ChatGPT Pro 100（原值 prolite）` 和 `ChatGPT Pro 100 (raw value prolite)`。只有缺少 `plan_display` 键的旧快照回退到 `plan`。显式 null 或对象没有 `short_name` 时省略胶囊。套餐缺失行使用套餐投影的原因，并区分额度来源未报告与原值不可安全显示。

身份侧套餐来自 Codex account/read 或 Claude auth status。两侧套餐相同时省略行 b 的套餐，有值的组织或工作区仍可见。身份侧来源冲突使用对应提示。Claude 的缺失行明确写 `额度来源不提供套餐`，身份侧另标 `登录套餐`。

## 3. 状态矩阵

| 状态 | 身份区域 | 额度部分 |
| --- | --- | --- |
| 读取关闭 / 非 official / 客户端未选中 | 不出现（也不显示保留的旧身份） | 同旧 Q7 / Q1 / 不渲染卡片 |
| 已识别，当前 | 行 a–c；Codex 加行 d | 正常 |
| 已识别，关联已确认（仅 Codex 共同键） | 行 d = `与此卡额度为同一账号` | 正常 |
| 身份读取失败或已到期，有旧值 | `上次识别的登录` 加弱化色邮箱。紧邻显示原读取时间和实际原因 | 额度按自身时间和状态显示，Claude 归属说明仍在 |
| 身份未知，无旧值 | `当前身份未知` + 原因 | 正常 |
| 未登录 / 非订阅（Claude） | `未登录订阅账号` / `当前不是订阅登录`；不显示旧邮箱 | 旧窗口失效，见 §4 |
| 未登录 / 非订阅（Codex） | 同上状态词 | 额度不失效，按真实年龄显示（D3） |
| 已知上下文变化（Claude） | 按实际身份状态显示 | 窗口区为「已失效」行，直到失效后首次新 usage 成功提交 |
| 失效后新 usage 已成功 | 按实际身份状态显示 | 显示新窗口和未确认的归属说明 |
| 套餐来源冲突 | 行 b 显示 `套餐来源不一致`；`help` 列出两个原值 | — |
| 未知套餐原值 | 显示原值本身（如 `max`），不推断等级 | — |
| 两端分属不同账号 | 每张卡各显示自己的身份；不比较、不提示 | 各自独立 |

本表面不出现多账号库存、历史账号列表或账号切换控件。被替换的身份不保留为后台历史，界面只有「当前」和「上次读取」两种身份。

## 4. 失效行

已知 Claude 上下文变化、退出登录、切到 API key，或关联秘密重置后，窗口区收成一行：

```text
订阅额度                                    已失效
登录上下文已变化，旧额度已失效
```

`已失效` 表示以前读取的额度因已知登录变化而丢弃。窗口区的选词函数读取 `quota_context.reason`。`context_changed` 对应 `quota.context.changed`，`signed_out` 对应 `quota.context.signedOut`，`non_subscription` 对应 `quota.context.nonSubscription`，`identity_key_reset` 对应 `quota.context.keyReset`。失效期间标题行的额度来源与时间显示 `—`。首个新 usage 成功提交后显示新窗口。重新确认前到达的 Claude 状态栏推送不会恢复窗口（D2，architecture A5）。

## 5. 文案

品牌套餐名在中英文界面都保持英文：`ChatGPT Pro 100` / `Pro 100` / `Claude Pro` / `Pro`。

| Key | zh-Hans | en-US |
| --- | --- | --- |
| `quota.identity.title` | 当前登录 | Signed in |
| `quota.identity.lastKnown` | 上次识别的登录 | Last known sign-in |
| `quota.identity.pastStatus(status)` | 上次读取 · {status} | Last read · {status} |
| `quota.identity.unknown` | 当前身份未知 | Current identity unknown |
| `quota.identity.signedOut` | 未登录订阅账号 | Not signed in to a subscription |
| `quota.identity.nonSubscription` | 当前不是订阅登录 | Not a subscription sign-in |
| `quota.identity.lastRead(rel)` | 上次读取 · {rel} | last read {rel} |
| `quota.identity.workspace` | 工作区 | Workspace |
| `quota.identity.organization` | 组织 | Organization |
| `quota.identity.planPrefix` | 登录套餐 | Sign-in plan |
| `quota.identity.associationUnconfirmed` | 与此卡额度的账号关联未确认 | Not confirmed as the account behind this quota |
| `quota.identity.associationConfirmed` | 与此卡额度为同一账号 | Same account as this quota |
| `quota.identity.reasons.never_probed` | 尚未读取身份 | Identity not read yet |
| `quota.identity.reasons.probe_failed` | 身份读取失败 | Identity read failed |
| `quota.identity.reasons.parse_failed` | 身份信息无法识别 | Identity output not recognized |
| `quota.identity.reasons.incomparable` | 身份字段不完整，无法比较 | Identity fields incomplete |
| `quota.identity.maskedSpoken` | 部分隐藏 | partly hidden |
| `quota.planDisplay.conflict` | 套餐来源不一致 | Plan sources disagree |
| `quota.planDisplay.raw(raw)` | 原值 {raw} | raw value {raw} |
| `quota.reasons.plan_not_reported` | 额度来源不提供套餐 | not reported by quota source |
| `quota.reasons.plan_unsafe` | 套餐原值不可安全显示 | plan value cannot be safely displayed |
| `quota.discarded` | 已失效 | Discarded |
| `quota.context.changed` | 登录上下文已变化，旧额度已失效 | Sign-in context changed; earlier quota discarded |
| `quota.context.signedOut` | 已退出订阅登录，旧额度已失效 | Signed out of the subscription; earlier quota discarded |
| `quota.context.nonSubscription` | 已切换为非订阅登录，旧额度已失效 | Switched to a non-subscription sign-in; earlier quota discarded |
| `quota.context.keyReset` | 无法与之前的登录比较，旧额度已失效 | Cannot compare with the earlier sign-in; earlier quota discarded |
| `quota.parseDetail.missing_line` | 缺少约定的额度行 | expected usage line missing |
| `quota.parseDetail.shape_invalid` | 额度行的格式或数值异常 | usage line shape or value invalid |
| `quota.parseDetail.timezone_unresolved` | 重置时间或时区无法解析 | reset time or time zone unresolved |

`quota.identity.reasons.never_probed` 只用于身份对象已存在但 `unknown_reason = never_probed` 的情形（例如门禁刚打开、身份尚未读取而额度已有值）；身份对象为 null 时（读取关闭、门禁、从未探测）整个区域不出现，见 §1。

解析细分作为第二行小字，出现在 `输出格式无法识别` 之下。`failure_detail` 缺失（旧 producer）时只显示原文案。

## 6. 窄宽度与无障碍

- 280 pt 保留可见的简短登录标签。原生邮箱中间截断以保留首尾和顶级域，原型使用尾部省略。套餐胶囊按可用空间收缩，包括 16 字符的宽字形原值。保留额度来源和时间，`Pro 100` 保持单行。
- 420 pt 下的长展示名仍可省略，帮助与无障碍名称提供安全全文。
- 身份区域合并为一个无障碍元素。朗读使用与视觉一致的状态，退出或非订阅时直接说出状态词。旧邮箱和旧状态后紧随原读取时间，其余信息按组织、套餐和关联顺序朗读。`***` 以 `quota.identity.maskedSpoken` 朗读。
- 身份区域不可点击，不弹层，不复制完整邮箱（界面本来就没有完整邮箱）。

## 7. 原型改动

| 文件 | 位置 | 改动 |
| --- | --- | --- |
| `prototype/src/data.js` | `codexClient` / `claudeClient` | 增加 `plan_display`、`identity`、`quota_context`、`failure_detail` 默认值；fixture 只放脱敏邮箱，不出现完整邮箱 |
| `prototype/src/data.js` | `QUOTA_VARIANTS` | §1 的身份、失效、恢复、长套餐和解析细分变体。`identityConfirmed` 只模拟有共同键的情形 |
| `prototype/src/Stage.jsx` | `QUOTAS` | 追加新变体 |
| `prototype/src/i18n.js` | zh/en `quota` 块与两处 `quotaVariants` | 加入 §5 全部 key 与变体名 |
| `prototype/src/Popover.jsx` | `QuotaClient` | 按 §2 选择套餐。按 §3–4 渲染身份、失效窗口和解析细分 |
| `prototype/src/styles.css` | quota 段 | 新增 `.quota-identity` 及其行样式和窄宽度规则 |
| `prototype/src/probe.js` | quota 断言段 | 分别检查视觉掩码、对应的口语脱敏形和合成泄漏反例。单独检查失效文案和数值、旧身份标注和窄宽度布局 |

## 8. 数据需求（交给 architecture）

| 需求 | 用途 |
| --- | --- |
| 每客户端 `identity`，门禁关闭时为 null | 区域是否出现 |
| `identity.state` / `unknown_reason` / `freshness` / `current_until` / `observed_at` | 行 a 与行 c |
| `identity.email_masked`（已脱敏） | 行 a；完整邮箱不得进入 payload |
| `identity.organization_name` 与 `organization_kind` | 行 b |
| `identity.plan_display` 与客户端级 `plan_display` | 胶囊与行 b，含原值和冲突状态 |
| `identity.quota_association` | 行 d |
| `quota_context.reason` | §4 |
| `failure_detail` | 解析细分 |

## 9. 样例不能证明的内容

真实 SF 字体下的截断、Dynamic Type、VoiceOver 对脱敏邮箱的实际朗读、深浅色切换，转为手工验收。
