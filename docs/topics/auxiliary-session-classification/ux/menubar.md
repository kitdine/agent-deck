---
status: active
created: 2026-10-26
updated: 2026-10-27
---

# 辅助会话隔离：菜单栏交互框架

## 设计阶段、依据与范围

本文为 framework，对应 [requirements.md](../requirements.md) 的会话来源、三范围、完整消耗与
跨表面口径要求。需求与 CLI framework 已独立评审 PASS，文档集合门禁仍开放。
本轮不修改 Go/Swift 产品代码，不定义 Settings 覆盖管理或 Widget 布局，不设计新扫描/定价策略。
architecture 必须落实字段后再进行 final-surface reconciliation。

继续使用 [共享 Product Prototype](../../../../prototype/README.md) 的 Popover，
入口 `?tab=sessions&auxMenu=normal`。原有 client、period、lang、theme、width 控件仍可用；
新增 auxMenu 舞台轴仅控制合成状态，不是产品设置。`auxDefault=work|auxiliary|all` 模拟持久化默认。
标本来源为 `prototype/src/AuxiliarySessions.jsx` 与 `auxiliarySessions.css`，由原 App/Popover 挂载，
不创建另一套窗口或独立产品原型。保留图像与身份见 [manifest.json](prototype/menubar/manifest.json)。

## 用户任务与布局

在会话页中回答三件事：当前统计包含谁、为何某条会话在这里、怎样查看被默认隔离的维护会话。
保留原顶部 client、消耗 hero、period、tab 顺序；会话页滚动区内依次为：

1. 工作 / 辅助 / 全部三段选择，明确标题/辅助功能名称为“会话范围 / Session scope”。
2. 范围影响说明、当前 client/period/scope，以及 unknown 或状态提示。
3. 所选集合的会话、项目、已知时长摘要；时长缺失数量单独说明。
4. 会话列表及分类标签，点击进入本页只读来源详情。
5. Activity / Workflow / Tooling 的同范围入口；各族独立表达 unavailable/partial。

范围控件置于会话页，而非顶部全局 client/period 行：后者会暗示价格/额度也被过滤。
相比只给“隐藏辅助”开关，三段选择能直接检查辅助子集，并使 unknown 归属明确。
选中使用文字、边框与粗体，不靠颜色。全部图标保留文字/无障碍名称。

## 选择与统计契约

| 范围 | 可见集合与文案 |
| --- | --- |
| 工作 Work | work + unknown；首次默认；“包含 N 个来源未知会话 / Includes N sessions with unknown origin.” |
| 辅助 Auxiliary | 仅已确认 auxiliary；不能把用户任务子代理整体隐藏 |
| 全部 All | 上述并集；保留每行真实类别与 unknown 说明 |

现有 client 和 period 与新 scope 组合，先过滤再计数、分组、分页与截断。
会话身份按 canonical client/session ID 去重，不在本 UX 重新合并父子或 worktree。
项目数为所选集合 distinct project，工作与辅助可能同项目，不能将两个项目数相加。
已知时长沿用既有会话跨度/时间归属，不解释成人工投入时长；缺失项不补零。
本框架用“已知时长 / Known duration”显示可加小计并单列未知数；最终架构须供应该数值。
若保留现有平均时长，则必须与相同选择、已知样本分母及未知数绑定，不将已知小计作为平均值。

顶部金额、token、调用量和用量口径会话/项目继续为当前 client/period 的全部消耗。
顶部新增“用量总览：全部会话 · 不受下方会话范围影响 / Usage overview: all sessions · unaffected by session scope”。
工作页数量来自会话索引，usage 事件聚合的会话数可能不同；架构与最终文案必须保留数据域说明，
不能为使数字相等删除未关联 usage 事件。菜单栏金额/Token 不随会话 scope 变化。
既有 menubarScope=follow 仍只消费其原有 client 等合同，不扩大到本次会话 scope。
额度来源和价格完整性均保持独立；来源 unknown 不等同于价格 unknown。

当前原型 fixture：today/all clients 的工作有 task-01、任务子代理 child-1、来源未知 old-01，
辅助有 maint-1；工作/辅助/全部会话数为 3/1/4，项目数为 2/1/2，已知时长为 45/10/55 分钟。
7d 加一条工作，30d 再加一条辅助。元数据 missing 的 old-01 时长为未知，不能加成 0。
工作信号未有同一 fixture 的可靠测量，三族显示不可用，不填未经核对的演示值。
该会话 fixture 与原消耗演示数据分属不同数据域，不作为总消耗与会话统计等量关系的证据。

## 临时选择、默认与生命周期

- 每次打开 Popover 从已保存默认初始化，首次为工作。关闭后再次打开恢复已保存默认；
  本次 scope 在切换 tab、client、period 后保留，不能意外重置。
- 切换为临时查看，不写默认、不修改分类、不影响 Widget/CLI；修改持久化默认由 Settings UX 管理。
- 选择读取已提交视图，不额外启动 scan；原刷新/后台采集仍遵循其所属契约，不在此改成 stored-only。
- 更新成功后 scope、列表、统计和信号以同一 publication/selection 一起替换。
  期间不先切新数字或把旧列表冠以新 scope。
- 后发选择覆盖先发请求；旧响应不得覆盖新选择。关闭窗口取消呈现等待，不取消已接受的共享扫描。
- 已成功恢复的 selection 与请求状态由同一窗口级状态对象持有；切 tab 卸载会话页或改变
  client/period 不能恢复旧失败标志。重试成功后始终保持 normal/auxiliary，直到明确的新请求或窗口关闭。
  重新打开窗口按已保存默认初始化；标本若舞台仍选 failed，则重新模拟一致的 failed/work 起点，
  不是复用上个窗口的 auxiliary 集合配上失败提示。
- loading 时控件暂不可操作并显示请求范围；保留旧范围选中和旧数据。首载无旧数据只显示加载说明。
  不用假 0、假成功时间或无限 spinner 掩盖请求状态。
- 失败保留旧 selection，范围按钮暂禁用、重试针对失败请求；没有旧数据则 unavailable。用户仍可退出、切其他 tab。
  原型 failed/loading 固定模拟 work → auxiliary，重试可恢复到辅助；这不是实际 IO 或计时证明。

## 状态、入口与恢复

| 状态 | 可见反馈与操作 |
| --- | --- |
| normal | 当前范围、unknown 数、摘要/列表/信号同口径 |
| empty | “此筛选下没有会话 / No sessions in this selection”；可选全部、改 client/period，不自动扫描或清库 |
| unknown / 旧索引 | 来源未知保持在工作；说明用途未确认，不把 unknown 标签写成辅助 |
| partial | 来源覆盖不完整但保留可读记录；unknown 数不可知时显示不可用；各信号族独立降级 |
| stale / pending publication | 保留最后成功内容与其时间，说明分类更新待发布，不把 refresh attempt 时间当成功时间 |
| loading | 明示请求范围与当前仍显示的范围；新 scope 数据未齐不发布 |
| failed with previous | 说明失败的 auxiliary 请求、保留 work，提供“重试辅助范围 / Retry Auxiliary” |
| unavailable | 不显示数量或 0；提示用顶部刷新重试；原有 schema/health 顶层错误优先 |
| override | 类别后明确用户覆盖，详情保留 automatic 和原因；没有静默改分类 |
| copy failed | 命令仍可选中手工复制；错误常驻到下一次操作，不用短暂 toast 掩盖 |

scope 为空不代表全库为空；已有 exclude/隐私边界继续生效，全部不会重新暴露排除文本。
顶部全局 unavailable/schema 继续由现有 Popover 呈现，本局部状态不伪造已恢复的顶层数据。
原有节律区保持其固定窗口及“不受上方筛选影响”说明，不纳入工作 scope。

## 来源详情与工作信号详情

点击行进入滚动区内详情：返回按钮、client + session ID + 安全项目标签、Effective、Automatic、
受控原因说明、rule version、覆盖状态。不显示完整源路径、正文、提示或凭据。
示例子代理原因写“用户任务的子代理，保留为工作 / User task child, retained as Work”。
规则版本缺失显示不可用，不以 origin-v1 冒充真实结果；原型 origin-v1 是合成值。

提供“复制查询命令 / Copy inspect command”，复制精确 client/ID 的只读
`agentdeck session classify <id> --client <client>`，完整转义由架构/CLI 实现负责。
本菜单栏详情不执行 shell、不直接修改覆盖。用户可在 CLI 查证后显式 set/clear；
Settings 的覆盖管理入口另行设计，本轮不制造未实现的 Settings 按钮。

Activity/Workflow/Tooling 详情显示同一 client/period/scope 和独立 availability。
切换 scope 从详情返回列表并重新选择；切 client/period 也返回列表，避免详情身份与新范围不匹配。
不可用族可进入了解原因，不显示假图表、成本或 0；最终布局仍需按 architecture 提供字段核对。

## 键盘、焦点、尺寸与辅助功能

- 使用系统按钮语义；Tab/Shift-Tab 按可见顺序，Enter/Space 激活。三段为按钮组，各项 aria-pressed，
  无强制鼠标交互；不借用 tablist 却遗漏箭头键协议。
- 进入来源/信号详情聚焦返回按钮，返回恢复到原触发行；若该行在更新后消失，回到范围控件。
  Escape 优先退当前详情，再按现有 Popover 关闭规则处理；不同时触发两个返回层级。
- 状态变化使用 polite live region，仅播报范围结果/失败摘要，不逐行朗读所有记录。
  复制反馈独立短文本；刷新公告与 scope 公告不得重复播报相同事件。
- 常规 420 pt，窄边界 280 pt。三段标签不截断；说明自然折行；摘要三列、标签可换行，
  记录标签/原因可换行且不靠 tooltip 才能读全。详情命令可自然折行且复制保留原始一行。
- 复用现有 Popover 760 pt 高度与滚动区，控制条/摘要/列表不外溢；下方内容通过滚动到达。
  不把“在滚动区域外”当作裁切错误，也不能用 overflow hidden 隐藏重要操作。
- 深浅两套主题复用语义 token；普通文字至少 4.5:1，字号不低于现有 10 pt 边界。
  新组件浏览器可访问性检查与 native VoiceOver/Dynamic Type 证明分列。

## 数据供应与验收映射

| 数据/行为 | 架构必须处置及后续 oracle |
| --- | --- |
| selection | client/period/scope、request ID、publication ID/time；迟到响应丢弃，列表/统计/信号一致发布 |
| 分类 | effective/automatic、reason、rule version、override 与 unknown_count；同 canonical 身份一致 |
| 统计 | 去重会话数、distinct 项目、known-duration 小计及 missing 数；跨日/DST/非可加值复算 |
| 消耗 | 保留全部事件/token/cost；scope 切换前后 hero 与菜单栏数值不变 |
| 失败 | no prior/prior、partial/stale、字段 unavailable；保存与派生更新状态不混同 |
| 交互 | 三范围、client/period 组合、切 tab、关闭重开默认、详情往返/焦点、复制成功/失败、重试 |
| 原生 | VoiceOver 读序与 live region、Dynamic Type、焦点/快捷键、真实查询竞态及窗口生命周期 |

标本检查覆盖中英 × 深浅 × 420/280、三范围集合、九种状态、已知/未知时长、
选定 client/period、详情和 retry。浏览器尺寸、DOM/AX 与截图证明只覆盖标本；不证明真实查询延迟、
Swift wire、Widget 或 SQLite。性能继承 scan-performance 的最终预算，禁止以范围切换触发全量重扫。

## 交接

本框架不等待另一份 UX 才能评审，但最终数据字段与 Settings 默认/覆盖、Widget 范围需在
architecture 后一起核对。缺字段必须显式拒绝并返回相关设计，不允许界面自行编造来源。
本轮不宣称自动维护识别、原生运行或最终表面已完成。

2026-10-26 自检：共享原型构建通过；中英 × 深浅 × 280/420 八组，每组 115 项，
共 920 项范围/布局/交互断言无失败。最终仅调整失败态按钮禁用和快照日期对齐后，
复用未受影响的集合、焦点和布局结果，定向核对禁用、日期、重试以及复制成功/失败五项均通过。
另核对 auxDefault=auxiliary 关闭重开恢复辅助；scoped axe 4.12.1 对浅色辅助视图与深色
来源详情检查均为 0 violations/0 incomplete。截图目视核对、源码和检查脚本身份随 manifest 保留。
这些结果不包含 native AX、Dynamic Type 或产品测试，未以浏览器通过宣布 CEv1 VERIFIED。
