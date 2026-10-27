---
status: active
created: 2026-10-27
updated: 2026-10-27
---

# 辅助会话隔离：Settings 交互框架

## 依据、阶段与边界

依据 [需求](../requirements.md)、已评审的 [菜单栏框架](menubar.md) 和 [CLI 框架](cli.md)。
本文件为 framework，定义独立 Settings 窗口的默认范围和单会话覆盖管理。
architecture 之后须最终核对字段、版本、读写协议和跨表面发布；本轮不修改产品 Go/Swift、
真实偏好、会话日志或用户数据库，不声明原生实现完成。

复用 [共享原型](../../../../prototype/README.md) 的 SettingsWindow，新增“会话范围与分类 / Session scope and origin”组。
入口 `?settings=1&auxSettings=normal`；合成舞台支持 normal/loading/loadFailed/saving/saveFailed/
 saveUnknown/conflict/pending。顶栏 Scenario、Open settings、Complete pending 明确标 stage，
不是产品设置或用户恢复按钮。状态由 App 级模型持有，关闭 Settings 不丢失已接受的操作。
源码为 `AuxiliarySettings.jsx`、`auxiliarySettingsModel.js`、`auxiliarySettings.css`，
内容身份与保留截图见 [manifest](prototype/settings/manifest.json)。

## 用户目标与分组

用户先决定以后默认看谁，再查证并纠正具体会话；两种操作分开保存，不能用一项写入顺带更新另一项。
与“点击选项即写入”相比，本组采用选择后明确保存：能在覆盖前看清 client/ID、自动分类和生效范围，
也能区分草稿与已保存值。操作可逆，不增加二次确认弹窗或全局 Apply。

组内顺序：说明 → 持续状态提示 → 默认范围 → 精确会话查询与分类 → 现有覆盖。
现有登录、刷新、额度等设置保持原入口与合同，不共享本组写入请求或错误状态。
常规 Settings 宽度 520 pt，窄可用宽度 320 pt；窗口体纵向滚动，标题/关闭入口固定可达。
长 ID、原因和帮助文字自然换行，控件不横向外溢，不以截断隐藏目标身份。

## 默认范围

| 控件/信息 | 行为 |
| --- | --- |
| 已保存 Saved | 展示最近确认的值；首次确定无配置时为 Work；读取失败/未知时显示 Unavailable，不能猜测 Work |
| Choose default scope | Work (includes unknown)、Auxiliary、All；只是草稿，不即时更改已保存值 |
| Save default scope | 仅写此偏好；同值返回 No change，不能重复触发投影或重扫 |
| 帮助 | “用于下次打开菜单栏和 Widget，不改写当前临时范围，也不改变 CLI 默认” |

成功保存后，当前已打开菜单栏的临时范围不变；关闭再打开消费新默认。
Widget 按新的持久默认生成下一份允许发布的快照，仍受 WidgetKit 刷新时机约束，不能宣称立即更新。
CLI 文本默认 Work、JSON 默认 All 的需求不变。本操作不影响全部 token、金额、额度或采集范围。
应用重启须读回持久值；原型仅模拟同一 App 页面内窗口关闭/重开，刷新网页会重置合成数据。

## 单会话分类与覆盖

客户端下拉（Codex/Claude）+ 完整 canonical session ID + Find session。
不按目录、项目或模型猜测，不提供批量规则；client 必选，避免同 ID 跨客户端歧义。
空 ID 禁用查找；查找为只读、无隐式 scan。编辑 client/ID 立即清除旧结果和旧可保存目标，
禁止把旧详情的保存按钮作用到新输入。

查找结果依次展示安全项目标签、完整 client/ID、effective、automatic、受控 reason、rule version、
user override。有效/自动分类的 work、auxiliary、unknown 保持三个不同值；规则缺失显示 unavailable。
分类原因不显示正文、路径、系统提示或凭据。旧数据缺信息时仍能理解 unknown；unknown 不是失败或辅助。
找不到或已被现有 exclude 隐藏的会话使用统一“未找到可见会话”，不暴露被排除文本或绕过访问边界。

新的分类选择只有 Work、Auxiliary、Use automatic origin：

- Work/Auxiliary 设置针对 canonical client/session 的覆盖；真实任务子代理默认仍为工作。
- Automatic 只撤销覆盖，恢复**当前**自动结果，不保证恢复为最初值；不删除会话、usage 或日志。
- “Save this session origin”只提交当前已显示结果的身份和版本；自动结果始终保留可查。
- 成功后 effective/override 一起更新，automatic 不伪造为用户选择；撤销不存在的覆盖为 No change。
- 保存和派生发布分开：持久成功但发布未完成显示 Saved / publication pending，
  用户可知道列表暂时仍旧；不把投影延迟报告为覆盖未保存。

现有覆盖列表仅显示 client/完整 ID、目标类别及 matched/unmatched 状态，可逐条撤销。
原始会话已不存在或恢复后尚未匹配时明确“未匹配，未应用”，不误套到另一个会话；允许删除该**覆盖记录**。
列表无覆盖时显示 No user overrides；列表读取失败为 unavailable，不能展示空列表冒充已清除。
正式数据获取须有界分页（每页 20 项、稳定 client/ID 排序），页码/总数与可见列表同版本；
标本只有两条覆盖，无需假造分页控件。新增覆盖通过精确查询入口完成，不提供“清除全部”。

## 保存状态机与窗口生命周期

每次操作绑定类型、client/ID（若适用）、期望版本和请求 ID；接受后与窗口视图分离。
不以原型的同步回调推断真实 IO 一定成功，架构须提供下列可判别状态。

| 状态 | 可信数据/控制与用户动作 |
| --- | --- |
| loading | 未确认默认/覆盖；禁用本组写入，可关闭；不显示默认 Work 的假选中值 |
| loadFailed | 已保存值与列表 unavailable；可 Read latest values；其他设置仍可用 |
| dirty | 明确已保存值与待选值；尚未发送；关闭丢弃草稿、不二次询问 |
| saving | 显示目标与请求值，保持旧已确认值；禁用同组竞争写入/查找；关闭不撤销已接受写入 |
| saveFailed | 明确 Not saved，旧值保留；Retry same operation 仅重试相同目标，也可修改选择后提交新操作 |
| saveUnknown | 结果不明，旧值仅为上次确认；禁止再次写入；Read latest values 查证，而不是假称失败或自动重试 |
| conflict | 其他 CLI/窗口修改已发生，禁止覆盖新值；Read latest values 后重新选择、显式保存，不自动重放旧草稿 |
| saved | 仅在持久化确认后显示；控件与已保存值一致，提示应用时机 |
| pending | 已保存、派生发布未完成；旧快照继续可读，时间保持真实；不谎称 Widget 已更新 |
| noChange | 幂等完成，无重复写入；保留准确有效/自动分类 |

本组一次只接受一个写操作；其他偏好组不被无谓禁用。保存错误持续到下一次读回或成功操作，
不定时消失。状态提示与 pending target 在窗口关闭/重开后保持；局部草稿、查询输入不代表操作所有权。
原型 saving 由舞台 Complete pending 模拟后台完成；不是产品“强制成功”按钮。
结果未知标本通过读回确认已落盘；冲突标本读回模拟外部最新值（默认 All 或覆盖 Auxiliary），
真实实现必须接受未落盘/已落盘/目标消失等实际结果，不把某一种 fixture 当通用结论。

关闭行为：Escape 或关闭按钮关闭窗口，未提交草稿丢弃；saving/unknown 保存在 App 级状态，
重开仍可查证。应用退出后的请求收尾、去重和读回属于 architecture，不授予取消或重放未确认写入的权力。
覆盖必须跨重启/重建保留，purge-index 不意外清除；backup/restore 的身份匹配和可移植边界按需求落实。

## 与菜单栏/Widget 的边界

默认偏好与菜单栏临时选择不同；Settings 保存不会立刻切换当前会话页。
覆盖提交可改变之后发布的成员集合，但不能改变当前选择器的 scope 字符串。
需要同代更新列表/会话数/项目数/时长/信号；partial/stale 继续显式表达。
本标本连通默认偏好到下次 Popover 初始化，但不模拟覆盖向菜单栏所有投影发布，
也不伪装 Widget 刷新或真实存储；这些由架构与最终表面验收覆盖。
默认值无法读取时，正式客户端不得以未知偏好生成新快照；保留已有可靠快照或显示不可用。
默认偏好向消费者同时传递值与确认状态；未确认值不是 Work，也不能用空值 fallback 成 Work。
本标本的 loading/loadFailed 没有旧可靠偏好或快照来源，因此关闭 Settings、重开 Popover 或切 tab
均显示“默认会话范围尚未确认；会话视图不可用”，不生成范围选择、列表与计数。
只有明确读回成功后才初始化新 selection；本合成读回确认无配置，故使用 Work。
正常已保存默认的真实读回必须使用其实际值，不因本 fixture 返回 Work 而固定默认。

## 键盘、焦点与辅助功能

原生 Settings 窗口沿用 ⌘, 入口与 Escape 关闭；本组用 label 关联 native select/input/button，
Tab/Shift-Tab 按默认 → 查询 → 结果 → 覆盖列表顺序，Enter/Space 激活标准控件。
帮助通过 aria-describedby/原生 accessibility description 绑定，不只视觉贴近。
正常操作不移动到无关设置；保存反馈用稳定 polite live region，持续状态不重复创建或逐行广播。
删除列表当前行后焦点移到下一条撤销按钮，若无下一条则到覆盖标题/查询入口；失败保留操作焦点。
未知结果与冲突必须能通过键盘到达 Read latest values；禁用态保留可读原因。
深浅主题复用语义色，文案中英完整；不单靠颜色、spinner 或短暂 toast 区别成功/失败。
native VoiceOver、Dynamic Type、焦点恢复和窗口与 App 生命周期需要单独验证，浏览器截图不能替代。

## 字段供应与验收设计

| 领域 | architecture 必须供应 / 可观察 oracle |
| --- | --- |
| 偏好 | 确认值、是否已读取、版本、草稿/待保存分离；保存后重启与跨窗口读回 |
| 覆盖 | canonical 身份、automatic/effective、override、reason/rule、matched 状态；撤销不改原始数据 |
| 并发 | request ID/期望版本、单写者或等价保障；CLI 并发冲突不静默丢更新 |
| 查询 | 有界读、稳定分页、未找到/不可用分离；输入改变清除旧目标；排除边界不泄漏 |
| 操作 | failure/unknown/confirmed/pending 分开，重试幂等；关闭重开及应用重启不制造假成功 |
| 集成 | 新默认用于下次打开与 Widget，临时范围不被覆盖；全部消耗不变，无额外日志重扫 |
| 验证 | 正常/失败/未知/冲突/保存中关闭重开、no-change、unknown 自动结果、未匹配覆盖、窄宽度和中英主题 |

字段或能力不能供应时，架构明确拒绝并返回受影响设计，不允许用永久禁用或假成功代替需求。
不定义新数据库表、迁移版本或最终 wire 键；不以设计稿中演示的 reason/rule 值宣称支持真实维护源。

## 自检与影响评估

2026-10-27：共享原型构建通过。中英 × 深浅 × 320/520 八组，各 42 项，共 336 项浏览器
断言无失败，覆盖八种场景、操作跨关闭重开、默认范围联动、撤销/无变化、未匹配覆盖与焦点。
浅色设置组与深色来源详情的 scoped axe 4.12.1 均为 0 violations、0 incomplete。
保留成功保存、未知结果、并发冲突和覆盖详情四张截图；已目视检查窄宽度与滚动内容。
这些仅为合成标本结果，不证明真实持久化、备份恢复、系统退出或 native accessibility。

App/Settings 的新分支仅在 auxSettings 舞台参数下启用；Popover 新默认参数只参与新窗口初始化。
未启用此参数时复跑菜单栏 149 项检查，无失败，已有需求/CLI/菜单栏语义不被本轮重新判定。
旧菜单栏 manifest 保留为历史目标，不改写其原哈希来冒充本轮证据；新依赖身份绑定于本文件的 manifest。
后续架构/final-surface 仍需核对新设置与所有消费者的联合内容状态。
