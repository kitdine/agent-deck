---
status: active
created: 2026-11-17
---

# 订阅账号与套餐辨识 — Requirements Review

## Round 1 — 2026-11-17

完成门禁：NOT_VERIFIED

2026-11-17规范字段补记：主控对原候选bc4086b9899ffe35a74624c6cea6472ffb3798c97b067dc2cf1b77f14e5a05ec回查此值，保留原FAIL和全部finding。

对象：HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`，requirements.md blob `9911cc0ffbeef105ce5ff7a8cafebfa2d2188ff0`，指纹 `bc4086b9899ffe35a74624c6cea6472ffb3798c97b067dc2cf1b77f14e5a05ec`。
独立方法：冷上下文 Claude/Sonnet 5.5，requested effort=high，需求边界评审；只读检查现有 Bug、旧 quota 约束、源码调用和备份恢复路径。主控核对旧 #11 明文、旧通道/opaque 约束、当前 adapter 与恢复清理及草案实际缺项后记录正式结论。

## 📋 需求边界评审报告

📊 总体评分：6.5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SAI-REQ-R1-F1 — 高：旧条款修订清单不完整。**
旧 Named gap、Goals 与验收 #11 禁止为 Claude 额度命名账号，并接受换号后旧数值继续显示；新目标允许当前登录身份展示，却未明确有限修订的表面、通知/导出/日志边界和新授权来源。
💡 修复：逐条列旧条款、当前替代、保留部分及用户授权，保持“身份展示不等于额度归属确认”。

**SAI-REQ-R1-F2 — 高：身份读取没有明确纳入总开关、official/选择门禁及探测预算。**
新增进程/RPC 不能绕过既有读取关闭时零探测和数据不显示的要求；独立退避与手动刷新规则也需决定。
💡 修复：把身份纳入 quota 读取流程，列出门禁、间隔/预算、独立失败退避与关闭后的显示规则。

**SAI-REQ-R1-F3 — 高：账号与额度的证明标准和降级状态未决定。**
当前样本未证明 account/read 与额度具有共同稳定标识；同一进程、邮箱/套餐一致或前后两次读取不自动证明关联。Claude usage 无账号 ID，不能留出未定义的 true 路径。
💡 修复：定义共同键的最低证明标准、无法证明时的独立身份展示、Codex 原额度隔离与新增身份关联的区别，以及 Claude 在本主题保持未确认。

**SAI-REQ-R1-F4 — 中高：已知身份变化后的 Claude 旧额度没有选定处置。**
“不串号”不能同时被理解为继续显示与失效隐藏；查询失败的未知状态也需区别已知变化。
💡 修复：明确已知可比较上下文变化时的缓存/通知失效，未知时保留还是隐藏及其真实年龄，不把旧额度命名为某个已证明账号。

### 🟡 其余必须关闭的边界

**SAI-REQ-R1-F5 — 中：隐私数据分类、掩码和关联值未定义。**
组织/工作区显示名与原始 ID 不是同一类数据；邮箱脱敏时点、私有关联值不可枚举、配置/认证路径解析即丢弃均需明确。
💡 修复：补数据分类表、掩码规则、允许表面及私有随机秘密约束，不把无盐邮箱哈希当隐私保护。

**SAI-REQ-R1-F6 — 中：新增身份的备份/恢复规则缺失。**
当前恢复会清空源机器 quota 观察与通知，新身份同样不能在目标机器冒充当前账号。
💡 修复：身份与关联秘密不进入便携备份/一般导出，Restore 防御性清空旧身份，并补验收。

**SAI-REQ-R1-F7 — 中：原 Bug 的诊断与解析验收未完整映射。**
缺行/形状/时区类别、最近尝试/最后成功、额外模型行、旧失败恢复及历史无法复现时的收口条件需写明。
💡 修复：保留严格两行约定及额外行忽略规则，补元数据和支持矩阵；不得猜测修复未复现的坏 stdout。

**SAI-REQ-R1-F8 — 低：套餐来源冲突与语言/CLI 文本范围未决定。**
planTier/subscriptionType 差异、品牌名是否翻译以及 CLI 原始文本/新增显示字段的关系影响 UX 前提。
💡 修复：给出保守来源优先级与冲突降级，固定中英文品牌名和 CLI 文本兼容边界。

### 🟢 优点

来源、证据局限、身份与额度分离、opaque 原值保留、无价格推算和既有两个 Bug 范围均正确。GUI PATH 独立修复依赖已声明。

### 📝 总结

八项是边界决定，不把它们留给未批准的 UX/architecture 自行发明。完整主题文档审计的三项 planned-not-written 是本阶段预期，未宣称整套设计已完整；本轮不要求渲染样例或精确 wire schema。正式 verdict 为 FAIL，进入同范围第 1 轮修订。

下一步指令：修复：subscription-account-identity / requirements.md / SAI-REQ-R1-F1, SAI-REQ-R1-F2, SAI-REQ-R1-F3, SAI-REQ-R1-F4, SAI-REQ-R1-F5, SAI-REQ-R1-F6, SAI-REQ-R1-F7, SAI-REQ-R1-F8


## Round 2 — 2026-11-17

方法：冷上下文Claude/Sonnet5.5，requested effort=high，只读需求边界复评；主控核对修订和独立报告后记录正式裁决。HEAD为4ac56db32e13c4e925b648c620ba76fc588a7781；requirements.md blob为a868691a98a6c5139618276b6b0ab2d5ff04cfee，指纹0dd2507f5ecd6d0a045760116329745fbaa03b5fd4cd339d94de32758ce6d61e，均实际匹配。运行时model=claude-sonnet-5-5，实际effort unknown；无pending或后台操作。

### 📋 需求边界复评报告

📊 总体评分：9/10

评审结论：PASS

完成门禁：VERIFIED

2026-11-18 在同一需求指纹补登原 Round 2 和当前格式检查的四条证据。回查 VERIFIED 4/4，原评审日期及结论保持。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议及后续约束

需求没有新finding。两项实现提醒由已存在的ad-subscription-account-identity-doc-arch-design承载：身份上下文失效须同时处理quota_alert_notices去重状态；交付稳定契约对账须覆盖cli-design Non-goals中旧账号/plan存储限制，说明仅持久化批准展示数据、无凭据。这些是已批准需求在架构/对账中的落实，不据此豁免真实需求缺口，也不新增产品范围。

#### 🟢 逐项关闭

| Finding | 状态和依据 |
| --- | --- |
| SAI-REQ-R1-F1 | CLOSED：逐条列旧operator/Goals/验收等被替代内容、保留边界和真实用户授权，身份展示不等于额度归属。 |
| SAI-REQ-R1-F2 | CLOSED：读取总开关、official、selected、间隔/预算、独立失败退避覆盖身份；手动刷新或渲染不绕过，关闭不显示旧身份。 |
| SAI-REQ-R1-F3 | CLOSED：共同稳定键和同探测上下文才允许关联；邮箱/进程/前后身份相同不足够；Claude归属始终false，Codex额度accountId隔离与显示关联分开。 |
| SAI-REQ-R1-F4 | CLOSED：已知可比身份变化令旧额度/提醒失效并拒旧在途结果；未知只保留真实年龄的历史/上次读取，退出登录或API-key不显示旧窗口。 |
| SAI-REQ-R1-F5 | CLOSED：字段分类、明确邮箱掩码、组织名去控制/双向字符及64字符限制、本机secret相关的不可枚举关联值、认证路径等解析即丢弃。 |
| SAI-REQ-R1-F6 | CLOSED：身份展示/关联值/本机秘密不进便携备份或一般导出，Restore不论档案版本防御清空。 |
| SAI-REQ-R1-F7 | CLOSED：区分真实尝试/成功时间和缺行/形状数值/时区错误，strict两行/额外模型行忽略不变；历史不可复现处置和原Bug整体关闭条件明确。 |
| SAI-REQ-R1-F8 | CLOSED：套餐来源优先级、tier/type冲突降级、品牌语言、CLI raw兼容及映射不影响价格/权限/配额均明确。 |

#### 📝 总结

内部状态与验收矩阵一致，未扩展到登录控制、账号库存或认证存储读取。requirements内容在复评后未改，仅tasks.md的Documents Review按PASS同步；其余UX、architecture、tasks分解仍Draft未完成，不能以此声明文档集或原Bug实现完成。主控最终L0与本Document gate另行登记，不把评审PASS当VERIFIED。

Reviewer未独立重采真实auth/account状态、未读原Bug正文或运行产品测试；源事实按文档中已核验并注明局限的记录复用，另直接核对现有Restore清理三张quota表。工具链/凭据/已安装App未操作，原始输出不依赖临时引用。repair_count为1，后续UX/architecture遵循已批准边界。

## Round 3 — 2026-11-18

### 📋 订阅账号需求复评

📊 综合评分：10/10

✅ 复评结论：PASS

### 🔴 必须修复

无。

### 🟡 其他改进

无。

### 🟢 已关闭

用户在 2026-11-18 明确“以 AgentDeck 提交为准，失效后禁止提交旧提醒”。账号变化和验收矩阵第 6 行均规定，失效事务提交后旧世代提醒提交调用为零。此前提交的通知以实际受理回调和显示时间记录平台观察。旧额度仍在失效后停止显示，旧在途结果不得恢复它。

### 📝 小结

主控和独立复评核对需求中受影响的两处、架构 A5.7 和任务 N4 的实际判定一致。两个来源 issue、隐私、原额度隔离以及其他验收行继续按原合同。两份 UX 文档和原型指纹保持与原核验一致。 正式目标 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、requirements.md blob `1227052e49c0d4d6e58e5e8aa362b62afe6d331e`、指纹 `a2f195f52d532c4db4b8cf77d5754f97562d3f91da830363bc39a6a6339f4bea`。独立冷复评由 T3 确认为 Claude Sonnet 5.5，报告请求选项 `effort=low`。实际推理档位未独立观察。检查范围为已明确的通知验收决定及受影响文档。

完成门禁：VERIFIED

CEv1 精确状态通过 4/4。主控读回全部证据关系，确认无缺项、失效或待决影响。同态文档集合、链接、评审记录结构、空白和主题限定 diff 检查均退出 0。

任务检查点 `ad-subscription-account-identity-doc-req-design`，目标状态 `subscription-account-identity:requirements.md:state:a2f195f52d532c4db4b8cf77d5754f97562d3f91da830363bc39a6a6339f4bea`。提交建议为本文件和相应评审记录单独提交。推送建议为随获准的 Claude issue 候选交付。
