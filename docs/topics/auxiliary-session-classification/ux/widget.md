---
status: active
created: 2026-10-27
updated: 2026-10-27
---

# 辅助会话隔离：Widget 交互框架

## 阶段、来源与范围

依据 [requirements.md](../requirements.md)、已评审 [Settings](settings.md) 与 [菜单栏](menubar.md)
框架；继承 [Widget refresh 合同](../../desktop-refresh/ux/widget-refresh.md)。本轮为 framework，
architecture 必须供应字段后再作 final-surface reconciliation，不修改 Go/Swift 产品代码。

保持现有五种 kind：Magnitude、Composition、Trust、Rhythm、Quota；三种 system family 不变。
本工作树 `prototype/src/Widgets.jsx` 的 Magnitude 小号和大号已有会话数字，中号没有；
本设计替换已有会话数字的语义，不新增独立会话 Widget、不向中号塞入新的 KPI。
其余四种 kind 的消耗、归因、节律与额度不因会话 scope 改变，既有 client/period 配置仍生效。

共享 [Product Prototype](../../../../prototype/README.md) 的 Widgets surface 为唯一标本。
入口 `?surface=widgets&auxWidget=normal` 聚焦展示受影响的 Magnitude 三尺寸；
不传 auxWidget 时仍显示全部五种。Published scope 与 Saved default 为两条独立舞台轴，
不是 Widget 上的可操作选择器。源文件及保留截图身份见 [manifest](prototype/widget/manifest.json)。

## 用户目标与信息层级

用户在桌面上辨认“这是哪一类会话的数量、未知来源是否包含、数据有多旧”，
同时继续读到全部消耗。Widget 是只读快照，不提供分类覆盖、范围切换、刷新扫描或后台进程控制。
调整默认范围走 Settings；菜单栏的临时范围不传播到 Widget。

既有 header、金额/token、图表和 footer 层级保留：

| 尺寸 | 会话范围呈现 | 保持的内容 |
| --- | --- | --- |
| small，148×148 原型比例 | 原 token/会话辅助行改为 token + All usage / 全部消耗；下一行 Work 3 sessions · Unknown 1 / 工作 3 会话 · 未知 1 | 今日金额、今日 token、7 桶图、成功时间 footer |
| medium，286×148 | 不新增会话数，也不新增无关联的 Work 标签 | 今日/7d/30d 三期成本与 token、20 桶图和轴；全部为消耗口径 |
| large，286×330 | 原 token/会话辅助区使用相同两行方式；范围数量对应本卡 30d 窗口 | 30d 金额/token、原有小计、90 桶趋势与统计、成功时间 footer |

原型尺寸保持现有真机比例，不把小号扩大来容纳文案。新增文字不低于既有 Widget 9px 下限。
小号英文范围行允许自然换行；本表面的 7 桶图最低高度为 14px、上间距 4px，优先容纳完整单位与未知说明。
桶数和可用的趋势含义不变；此紧凑规则不改变其他 kind 或中号的图表布局。
工作数量行的 scope 与数值不可分离；旧 totals.sessions 不直接改名为 work count。
All usage 明确修饰金额/token，范围行明确修饰会话数。不同数据域计数不能强行相等。

## 聚合口径与文案

| 快照 scope | 英文短标签 | 中文 | 成员 |
| --- | --- | --- | --- |
| work | Work | 工作 | work + unknown；任务子代理保留 |
| auxiliary | Aux | 辅助 | 已确认后台维护；Aux 的完整无障碍名称为 Auxiliary |
| all | All | 全部 | 两者并集 |

Unknown N 表示同一会话集合中的未知来源，不是未定价次数、未归属调用或扫描失败数。
未知数为 0 可显示 0；未知覆盖不足则 Unknown — / 未知 —，不能补 0 或声称完全分类。
没有会话且数据可信时 count=0、unknown=0；不可用不是空集。
计数沿用 canonical 去重、client/period 与窗口的统计规则；过滤先于聚合，Widget 不再分类或去重。
small 和 large 分别对应自己的当前 client/period 投影，不能复用别的窗口的 scope/count。

数据少于四位时用本地化整数；数值很长时允许本地化紧凑数字，但无障碍读出完整整数和单位，
不得截断成看似更小的整数。未知标记、范围和时间不能靠 hover tooltip 才能理解。
生产数字格式的极大计数、Dynamic Type 与辅助功能验证由最终表面/实现承担；本轮合成计数为 0–6。

原型只使用聚合：today 的 Work/Aux/All 为 3/1/4，30d 为 4/2/6；Work/All 各含 1 个 unknown。
unknown 场景将所选窗口全部视为未知，Work/All 保留全部、Aux 为 0；partial 的未知数量不可知。
不将 synthetic aggregate 说成真实用户规模或真实分类准确率。

## 默认值、发布与快照身份

宿主读取已确认的持久默认，生成 scope/count/unknown/period/client/publication/time 的一致投影。
Widget 不直接读取 Settings 存储、SQLite、源日志或 helper；只读取隐私受限 App Group 快照。
尚未读取或读取失败的默认不等于 Work。只有明确确认无配置时才能应用首次 Work 默认。

- 无已确认默认、也无可靠旧快照：会话范围与计数不可用；仍可显示同一支持投影中独立可读的消耗。
- 有可靠旧快照：新的默认尚未发布/WidgetKit 尚未刷新时，显示旧 scope **及其旧 count/time**。
  例如保存 Auxiliary 后卡片仍显示 Work 3 是允许的；不能仅把标签改成 Auxiliary 却保留 3。
- 快照必须带已发布 scope，而不是渲染时读取最新偏好补标签。未知 scope token/不完整聚合不能 fallback Work。
- 新快照被接受时 scope/count/unknown/client/period/time 一起替换；同代字段缺失按不可用处理。
- 已有可读旧投影的 stale/pending 保留数据及成功时间；失败的读请求不能伪造新时间。
  宿主是否存在不由 Widget 进程探测，读得到旧数据只显示年龄。
- Settings 写入成功不承诺立即改变卡片；沿用 WidgetKit 机会与现有 reload/fallback 合同。
  没有对本功能新增轮询、OS 唤醒或后台服务的授权。

舞台的 Published scope 改变模拟接受新快照；Saved default 改变只模拟宿主偏好已保存，
必须不改变当前卡片的 scope/count/金额。pending/stale 演示旧年龄，不显示“保存已实时生效”。
此模型不模拟真实 WidgetKit 调度、磁盘写入或跨进程原子发布。

## 状态与优先级

| 状态 | 会话行 | 消耗与 footer |
| --- | --- | --- |
| normal | scope + count + Unknown N | 消耗完整性及成功年龄沿用现有字段 |
| empty | scope 0 · Unknown 0 | 不据会话空集把总费用清零 |
| unknown | Work/All 保留未知会话；Aux 0 | 未知分类不改费用 |
| partial | 已知总会话数，Unknown — | 不伪造未知覆盖完整；保留费用自己的 partial |
| legacy | Session scope unavailable / 会话范围不可用 | 旧快照无新分类字段，不将旧 total 改名为 Work；独立消耗可继续显示 |
| defaultUnknown | 同上，不显示范围/计数 | 无确认默认不创建新正常分类数据；原型此场景明确无旧可靠分类快照 |
| pending / stale | 旧已发布 scope/count 保留 | 旧成功时间与既有 old/aging 语义，不报告宿主在线/离线 |
| missing/container/read/unsupported | 整张卡使用原有 typed unavailable | 读取失败优先于局部 scope 状态；不能保留数值或用 0 代替 |

读取成功但缺新字段与整份快照不可读是两层错误，不互相覆盖。
重试成功恢复实际支持的投影，仍保留其 partial、旧年龄或 legacy 限制。
占位 skeleton 只用于系统占位，不作为运行期未知默认的假数据。
现有 footer 对 fresh/aging/old 的分档不变，不把 timeline 重读当作成功刷新。

## 隐私、可访问性与操作

App Group 新增字段只允许聚合 count、unknown_count、scope、availability/reason 及所需快照关联元数据。
禁止会话 ID、父子关系、项目名、完整路径、模型/来源标记原文、正文、凭据或用户覆盖清单进入 Widget。
分类原因对本表面只用“范围未确认/字段不支持/覆盖不完整”等安全状态，不泄漏识别输入。

点击 Widget 沿用已有宿主打开动作；不新增写入或可执行命令，不改变用户默认范围。
系统 Widget 编辑中的 client/period 保留原合同，不新增每张卡持久化 scope 覆盖以免与 Settings 默认冲突。
小号 Aux 的 VoiceOver 名称必须展开 Auxiliary；所有会话数字读出 scope、数量、unknown 和时间窗口。
金额/token 与工作数量使用不同标签；无色下仍可辨。失败原因与旧年龄用文本表达，不以图标颜色代替。

## 架构数据需求与验收映射

| 领域 | 必须供应 / 后续 oracle |
| --- | --- |
| 可信默认 | 值与确认状态，不能把初始化值当已确认；无配置/读取失败区别明确 |
| 快照 | 聚合 scope/count/unknown/client/period 同一 publication，旧字段原义保持；旧消费者兼容与新字段缺失 |
| 一致性 | Work+Aux=All 会话数，同 client/period 对照 CLI/菜单栏；价格总量不变 |
| 发布 | 新偏好未发布、发布失败、旧快照可读、读失败、恢复及宿主缺席；不重标旧数字 |
| 隐私 | App Group serialization allowlist；无 canonical ID/项目/正文/路径/override 明细 |
| 尺寸 | 三尺寸原边界、7/20/90 桶结构不变；标签不溢出/截断，未知不补零 |
| 原生 | WidgetKit timeline、VoiceOver/Dynamic Type、实际客户端/时段配置、跨进程失败与升级验收 |

本轮新设计仅涉及 Magnitude 已有会话行；其他 kind 复用既有语义，不重开此前独立主题。
architecture 要对字段与格式逐项供应或拒绝；不得以永久隐藏计数代替本需求。
Widget framework 评审通过后才满足 architecture 的最后一个框架输入，不在本轮关闭任务或提前解开该依赖。

## 设计自检与证据范围

2026-10-27 最终原型构建成功；中英 × 深浅四组，每组 230 项，共 920 项断言通过。
覆盖八状态、三已发布范围、三尺寸边界、费用不变、保存默认不重标旧数据，以及 7/20 桶结构。
四类整份快照读取失败分别覆盖 pending，各为三张 typed failure、零个分类计数；该结果复用在最终
文字/局部图表高度修改后，因为失败分支完全替换 children、未受这些修改影响。
scoped axe 4.12.1 在浅色 All 与深色 Work 均为 0 violations/0 incomplete；修复早期 span 缺角色的问题后取得。
最终小号图表最低高度调整不改颜色或语义标记，复用上述可访问性结果；不宣称 native AX 通过。
保留正常、旧快照、默认未确认与版本不支持标本，内容哈希见 manifest。
检查仅覆盖合成聚合和浏览器设计，不证明真实来源、极大计数、WidgetKit、App Group 或性能验收。
