---
status: active
created: 2026-10-26
updated: 2026-10-27
---

# 辅助会话隔离：菜单栏最终表面

## 设计阶段、依据与范围

本文为 final-surface reconciliation 候选，落实 [requirements.md](../requirements.md) 与已评
[architecture.md](../architecture.md) Round 2 PASS/VERIFIED（blob `5b5a3b5694827900a9380e3879058a6baca4919a`）。
前置文档证据归属已勘误，不能再把下游缺稿写成需求门禁阻塞；原 framework 评审保留为历史。
本轮不修改 Go/Swift 产品代码，不定义 Settings 覆盖管理或 Widget 布局，不设计新扫描/定价策略。
本轮落实已定字段、默认确认和降级规则；当前最终表面仍需独立评审。

继续使用 [共享 Product Prototype](../../../../prototype/README.md) 的 Popover，
入口 `?tab=sessions&auxMenu=normal`。原有 client、period、lang、theme、width 控件仍可用；
新增 auxMenu 舞台轴仅控制合成状态，不是产品设置。`auxDefault=work|auxiliary|all` 模拟已确认持久化默认。
新增 legacy、defaultUnknown、sourceUnavailable、inputMismatch 四态分别覆盖新投影缺失、默认未确认、
来源不可用及证据代次不一致；不把这些原因都伪装成无会话或 0。
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
使用架构 counts.known_duration_ms 的“已知时长 / Known duration”小计，单列 missing_duration_count。
全部会话时长均缺失时小计为 null、显示 —；可信空集为 0。这里不再显示旧平均时长，不能将小计作为平均值。

顶部金额、token、调用量和用量口径会话/项目继续为当前 client/period 的全部消耗。
顶部新增“用量总览：全部会话 · 不受下方会话范围影响 / Usage overview: all sessions · unaffected by session scope”。
工作页数量来自会话索引，usage 事件聚合的会话数可能不同；范围说明保留数据域差异，
不能为使数字相等删除未关联 usage 事件。菜单栏金额/Token 不随会话 scope 变化。
既有 menubarScope=follow 仍只消费其原有 client 等合同，不扩大到本次会话 scope。
额度来源和价格完整性均保持独立；来源 unknown 不等同于价格 unknown。

当前原型 fixture：today/all clients 的工作有唯一用户覆盖为 work 的 task-01、自动分类为 unknown 且无覆盖的任务子代理 child-1，以及来源未知 old-01，
辅助有 maint-1；工作/辅助/全部会话数为 3/1/4，项目数为 2/1/2，已知时长为 45/10/55 分钟。
7d 加入无覆盖、自动分类 unknown 的 task-02（进入 Work），30d 再加一条辅助。today 的 Work/All 各含 2 个 unknown，
7d/30d 各含 3 个 unknown；sourceUnavailable/inputMismatch 的 today unknown 为 3，只保留 task-01 的有效覆盖。
元数据 missing 的 old-01 时长为未知，不能加成 0。
工作信号未有同一 fixture 的可靠测量，三族显示不可用，不填未经核对的演示值。
该会话 fixture 与原消耗演示数据分属不同数据域，不作为总消耗与会话统计等量关系的证据。
只有 task-01 的 effective=work 来自用户覆盖、revision=2；child-1 的 effective/automatic 均为 unknown、
revision=0，原因明确说明任务子代理样例的用途未确认、因此保留在 Work，不能被改写为“用户覆盖”。
task-02 同样无覆盖，不再保留不可达的 user_task/task_child/manual_work 原因文案；
显式 override 舞台另模拟用户对 old-01 的覆盖。短 ID 是呈现样例，不是适配器 UUID 输入 fixture。
来源不可用/代次不匹配时，不再沿用旧 auxiliary；有有效绑定的用户覆盖保留，其他自动分类降为 unknown。

## 临时选择、默认与生命周期

- 每次打开 Popover 从**已确认**默认初始化，仅确认无配置时首次为工作。关闭后再次打开恢复已保存默认；
  本次 scope 在切换 tab、client、period 后保留，不能意外重置。
- confirmed=false/value=null 不能 fallback Work。没有可靠旧 selection 时显示默认未确认、无范围控件/分类计数；
  指向 Settings 读回。已有来源明确的旧快照可保留其旧范围与成功时间，但不能重标为新默认。
  defaultUnknown 标本模拟无可靠旧快照；Settings loadFailed/loading 跨关闭重开的原组合行为继续有效。
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
| legacy 新 view 缺失 | 分类范围不可用、无分类计数或选择器；旧 sessions.total 不得当成 Work；独立 usage 可读 |
| defaultUnknown | 默认未确认，无默认 Work 或数字；去 Settings 读回，不启动隐式扫描 |
| sourceUnavailable | 展示来源不可用原因；未确认用途为 unknown，用户覆盖保留；不把该情况当作整库不可读 |
| inputMismatch | 标明来源代次不匹配，陈旧自动证据不用于隐藏会话；同代缺口计入 partial |
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
合成标本的成功时间固定为数据日 2026-08-18 的 14:30 UTC，与共享数据日对齐；stale/loading/failed
通过共享 formatDate 的 UTC 约定格式化 generated_at，并明确显示 UTC。不再受查看者时区影响而退到 08-17。
该日期/时刻只属于可重复的设计 fixture；真实成功时间必须来自生产 DTO，不能用此固定值替代。

## 来源详情与工作信号详情

点击行进入滚动区内详情：返回按钮、client + session ID + 安全项目标签、Effective、Automatic、
受控原因说明、rule version、覆盖状态。不显示完整源路径、正文、提示或凭据。
用户覆盖示例的 Effective 明示“用户覆盖”，Automatic 仍为 unknown，Reason 保留来源元数据缺失的原因，
不把覆盖动作伪造成自动分类依据。
规则版本缺失显示不可用，不以 origin-v1 冒充真实结果；原型 origin-v1 是合成值。另显示 override revision，
它不是 publication_id 或源 parser version；没有覆盖时仍可有 tombstone revision。

提供“复制查询命令 / Copy inspect command”，复制精确 client/ID 的只读
`agentdeck session classify --id '<id>' --client '<client>'`。始终使用 --id 避免保留子命令歧义；
每个参数按 shell 单引号转义，标本的复制内容和可见命令共用同一函数，不执行插值或 shell。
本菜单栏详情不执行 shell、不直接修改覆盖。用户可在 CLI 查证后显式 set/clear；
覆盖修改通过 Settings 已有单会话入口或 CLI 执行，本面板不增加修改/撤销按钮。
用户覆盖撤销的 core-only、CAS 和 not_queried 回执合同属于架构/CLI；不能从旧回执猜本详情的当前分类。

Activity/Workflow/Tooling 详情显示同一 client/period/scope 和独立 availability。
切换 scope 从详情返回列表并重新选择；切 client/period 也返回列表，避免详情身份与新范围不匹配。
不可用族可进入了解原因，不显示假图表、成本或 0；缺一族不影响其他族或列表。

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

## 已定 DTO 与接收规则

新 App 消费 wire-v1 的可加 `session_scope_view`，不得重解释旧 sessions.total/items/periods/work_signals。
旧 App 忽略新增 view；新 App 收到缺失 view 时进入 legacy，而非取旧 total 补成 Work。

| 架构字段 | 最终表面 |
| --- | --- |
| available、selection.scope/client/period | 能否显示分类视图与三项选择标签；缺失/未知枚举不是合法默认 |
| publication_id、generated_at | 列表/统计/信号的同代来源与真实成功时间；本地请求时间不替代 generated_at |
| origin_coverage | 完整性、unknown_count 与安全原因；unknown_count 不可知显示不可用，不从列表页猜完整总数 |
| counts.sessions/projects | canonical 会话数与 distinct 项目数，不拿 recent 的行数当 total |
| counts.known_duration_ms/missing_duration_count | 已知时长小计与缺失样本；全部缺失时显示 —，不得使用 0 伪造测量 |
| recent[].origin | effective/automatic/reason_code/rule_version/override/revision，按架构白名单进入详情 |
| work_signals.activity/workflow/tooling | 各族 available/items 独立降级；匹配同一 selection 与 publication |
| 默认 DTO confirmed/value/revision/reason | 仅 confirmed=true 才使用 value；无配置的 work/revision0 与读取失败 null 不混同 |

视图接收先核对最新 request ID 和 selection，再核对 publication 及所需字段；过期响应丢弃。
没有完整候选时，不把新 total 与旧 recent 拼接。架构对跨库向量最多重试两次后返回旧完整快照/stale
或无旧值/unavailable；UI 接受该显式结果，不再自行拼接或无限重试。input_token 不一致是服务归约的
unknown/partial 原因，不是 UI 猜用途的许可。范围改变不发起源适配器访问或原始日志扫描。

快照与默认的信任状态由窗口/controller 保存，而不是某个 tab 的生命周期保存。
当 Settings 改变已确认默认，当前临时 selection 保留；重开才初始化新默认。
读取失败不能让隐含的 Work 穿过 controller；旧可靠快照也只能连同其旧 selection/time 呈现。
legacy/defaultUnknown 标本模拟无可靠旧分类视图，故无选择器或分类数字，顶部独立消耗不被抹去。

共享标本的 view 对象是合成呈现模型，展示上述字段的可见结果，不是产品 wire decoder 或真实请求调度器。
迟到响应和跨库故障须在实现验收中使用实际 DTO/并发夹具证明；本轮不以同步按钮回调冒充这些证据。

## 数据供应与验收映射

| 数据/行为 | 已定合同与后续 oracle |
| --- | --- |
| selection | client/period/scope、request ID、publication ID/time；迟到响应丢弃，列表/统计/信号一致发布 |
| 分类 | effective/automatic、reason、rule version、override 与 unknown_count；同 canonical 身份一致 |
| 统计 | 去重会话数、distinct 项目、known-duration 小计及 missing 数；跨日/DST/非可加值复算 |
| 消耗 | 保留全部事件/token/cost；scope 切换前后 hero 与菜单栏数值不变 |
| 失败 | no prior/prior、partial/stale、字段 unavailable；保存与派生更新状态不混同 |
| 交互 | 三范围、client/period 组合、切 tab、关闭重开默认、详情往返/焦点、复制成功/失败、重试 |
| 原生 | VoiceOver 读序与 live region、Dynamic Type、焦点/快捷键、真实查询竞态及窗口生命周期 |

标本检查覆盖中英 × 深浅 × 420/280、三范围集合、框架九态及最终新增四态、已知/未知时长、
选定 client/period、详情和 retry。浏览器尺寸、DOM/AX 与截图证明只覆盖标本；不证明真实查询延迟、
Swift wire、Widget 或 SQLite。性能继承 scan-performance 的最终预算，禁止以范围切换触发全量重扫。

## 交接

本轮已按架构完成菜单栏最终核对，当前候选等待独立最终表面评审。没有新增存储、扫描或修改入口。
与 Settings/Widget 的合同边界已落实；其各自 final-surface 仍由各自任务完成，不在本轮替其宣布通过。
缺字段显式不可用，不允许界面自行编造来源；不宣称自动维护识别、原生运行或实现验收完成。

2026-10-26 自检：共享原型构建通过；中英 × 深浅 × 280/420 八组，每组 115 项，
共 920 项范围/布局/交互断言无失败。最终仅调整失败态按钮禁用和快照日期对齐后，
复用未受影响的集合、焦点和布局结果，定向核对禁用、日期、重试以及复制成功/失败五项均通过。
另核对 auxDefault=auxiliary 关闭重开恢复辅助；scoped axe 4.12.1 对浅色辅助视图与深色
来源详情检查均为 0 violations/0 incomplete。截图目视核对、源码和检查脚本身份随 manifest 保留。
这些结果不包含 native AX、Dynamic Type 或产品测试，未以浏览器通过宣布 CEv1 VERIFIED。

### Final-surface 自检 — 2026-10-27

最终原型构建通过；中英 × 深浅 × 280/420 八组各 173 项，共 1,384 项浏览器断言无失败。
覆盖原有重试/标签/默认/焦点行为与新增四态、用户覆盖与自动 unknown 分离、缺失规则版本、
全部时长缺失不补零及带 --id/参数引用的只读复制命令。
最后将 stale/loading/failed 时间文案改为直接格式化 view.generated_at 后，
追加中英 × 280/420 × 三态共 12 个定向用例，时间同源和横向边界均通过；其他未变行为复用上述结果。
浅色来源不可用与深色来源详情的 scoped axe 4.12.1 均为 0 violations/0 incomplete，
时间格式化改动不涉及这些状态及辅助功能结构。四张最终标本已目视检查；旧截图/检查保留为历史。
实际 DTO 解码、异步乱序、真实数据库与 native AX 仍由实现验收承担，本轮无产品写入或性能保证。
