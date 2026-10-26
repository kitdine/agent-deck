---
status: active
created: 2026-10-26
updated: 2026-10-26
---

# 辅助会话隔离：CLI 交互框架

## 1. 目标、范围与非目标

依据 [requirements.md](../requirements.md) 的 Round 2 已评审边界；需求完成门仍因下游文档集合
缺口开放，不把需求 PASS 当作整套设计或产品验收。本文为 **framework**：定义可见交互和字段需求，
architecture 之后进行 final-surface reconciliation，再冻结最终协议和 specimen 身份。

混合表面：普通逐行 CLI、显式全屏会话浏览器、JSON 自动化。目标是在不损失原始消耗的情况下，
找到用户工作、查看维护来源、理解 unknown、纠正单个会话分类。CLI 产品文案保持英文。
不实现新 TUI 框架，不改采集/定价，不允许 TUI 中写入，不加入路径/项目批量规则。

共享设计真相为 [Product Prototype](../../../../prototype/README.md) 的 CLI surface，新增 auxiliary 页签。
`?surface=cli&auxiliary=work` 为入口，`auxiliary=` 选择下文场景，`auxCols=40|48|80|120` 选择宽度。
舞台控件不是产品 CLI 控件。源数据来自 `prototype/src/auxiliaryCli.js`，React 展示为
`prototype/src/AuxiliaryCli.jsx`；不运行产品命令，不读取真实数据。

## 2. Terminal matrix

| 环境 | 行为 |
| --- | --- |
| macOS、TTY stdin/stdout、正常 TERM | 普通命令逐行输出；只有 --interactive 才进全屏 |
| --format json | 机器数据，禁止交互、ANSI 和提示输入；未指定范围默认 all |
| stdout pipe / stdin redirected | 逐行文本或 JSON；显式 --interactive 拒绝，给出 session list 替代命令 |
| TERM=dumb | 无控制码的逐行模式；显式 --interactive 拒绝 |
| NO_COLOR / --no-color | 不依赖颜色表达类别/选中/失败；选择有 >，类别有单词 |
| 屏幕阅读器 | 使用逐行 session list/show/classify 或 JSON；不要求读屏用户操作全屏 |
| 40 列、48×10、80×24、120×40 | 分别检查窄行、最小全屏、常规、宽高终端；见布局规则 |

## 3. 命令与可观察输出

下列新增语法为本框架提案，须由架构确认解析、错误代码、字段供应及兼容实现，不声称已经支持。

| 入口 | 用途与默认 |
| --- | --- |
| session list / session search <query> [--session-scope work\|auxiliary\|all] | 文本默认 work；JSON 默认 all；显式选择在两种输出中同义 |
| session --interactive [--session-scope work\|auxiliary\|all] | 初始默认 work，可临时切换；不读取或写入桌面默认 |
| session show <id> --client <client> | 明确 ID 可看辅助会话；详情不接受 --session-scope，避免产生“找不到已知会话”的误导 |
| session classify <id> --client <client> | 只读显示有效/自动分类、原因、规则版本和覆盖状态，不显示来源正文或路径 |
| session classify <id> --client <client> --set work\|auxiliary | 显式、可逆地设定一条覆盖；不二次确认；不能 --set unknown |
| session classify <id> --client <client> --clear | 删除该会话的手工覆盖并恢复当前自动结果；无覆盖时成功返回 no change |
| usage signals [--session-scope work\|auxiliary\|all] | 工作信号按范围选择；文本默认 work、JSON 默认 all；保留既有 client/period/kind 条件 |
| usage stats 的工作分析块 | 新增同名范围参数仅作用于会话/Activity/Workflow/Tooling；总 token、费用等消耗块仍为 all 并分别标注范围 |

`usage summary`、价格和额度命令不新增此参数；它们继续表达全部消耗。
`--all` 继续表示取消分页，不表示 session-scope all；两者可组合。
--session-scope 只接受单个合法值；未知值、重复冲突值和不支持该参数的命令在读取/写入前拒绝。
classify 的 --set 与 --clear 互斥；省略两者为查询。明确 client 避免跨客户端同 ID；
省略 client 且唯一时可沿用既有 ID 解析，歧义时不写入，提示指定 client。
不存在或被现有 exclude 隐藏的目标沿用 not-found 隐私边界，不借分类入口显示隐藏文本。

### 扫描与范围过滤矩阵

沿用 [CLI Manual](../../../specs/cli-manual.md) 的 usage 扫描契约，范围过滤不额外发起扫描，
不等于所有命令都 stored-only；本主题不改变采集策略。

| 入口 | 扫描行为与反馈 |
| --- | --- |
| session list/search/show、classify 查询/修改、session --interactive 及 s 切换 | 不隐式扫描；读取已提交状态，覆盖写入只更新分类状态及其派生，不启动日志重扫 |
| usage stats 默认 | 保留报告前同步扫描；随后对工作分析应用范围，消耗块仍为 all |
| usage stats --no-scan | 保留直接使用已存聚合；范围参数不抵消 --no-scan |
| usage signals | 每次报告前尝试同步扫描；没有 --no-scan 或 stored-only 模式；范围参数不改变此行为 |

长扫描保留现有 stderr progress、quiet 抑制与取消行为；不向 JSON stdout 插入进度或提前伪造最终值。
扫描失败时保留原有错误/部分结果判定，不转换成新后台任务。特别是 signals 继续读取上次已提交派生，
envelope 为 `partial: true`、warnings 包含 `scan_incomplete`；文本明确标 PARTIAL 和旧已提交数据。
仅扫描失败但存储可读时仍按现有结果退出语义处理；存储也不可读时沿用运行错误，不能补零。
stats 的扫描错误、warnings/partial 及可读旧聚合行为同样沿用原命令，不因 scope 选项改变。
标本 signalsScanning/statsScanning 将进度独立放在 stderr，stdout 为空；
signalsScanIncomplete 展示失败后的部分结果，statsStored 展示 --no-scan 路径。

### 集合、分页与信息层级

work = work + unknown；auxiliary = auxiliary；all = 两者并集。未知状态不隐藏。
先应用 client/project/时间与范围，再去最终集合排序、分页、top-N 和统计。
沿用 canonical 去重优先级、时间归属及既有排序；范围不得改变身份规则。
标题 → Scope → unknown 提示/partial/stale → 行 → pagination → 精确保留参数的下一页命令。
每行类别必须可辨，辅助默认不出现在 work，但真实任务子代理保留。

JSON 不指定范围时保留旧 data 集合形状、分页及旧字段含义，不能为了附加 scope 把数组改成对象。
新元数据位置由 architecture 选择可加且兼容的方案。显式范围只改变所选集合和其分页 total。
JSON 详情、查询和写入沿用 envelope；机器警告通过 warnings/partial，绝不混入人类 banner。

搜索结果展示其所属会话类别，匹配文档数与匹配会话数分别命名；不能用文档行数充当会话数。
所选工作分析与总消耗同屏时分别写 `Scope: work` 和 `Usage scope: all`，
`All usage and costs remain unchanged.` 是范围说明，不宣称现有价格准确。

### 分类详情与修改回执

查询依次列 Client、Session、Effective、Automatic、Reason、Rule、Override。
Reason 使用受控原因码，文案解释 unknown、冲突或混合用途；缺字段写 unavailable，不伪造 none。
手工覆盖保留 automatic 分类。保存回执只有持久化成功后才显示 CLASSIFICATION SAVED，
同时给出撤销命令；clear 给出自动结果。重复 --set 相同值及重复 --clear 返回 NO CHANGE。
不承诺所有表面已刷新：投影尚未发布显示 `Views: pending publication`。
若保存失败显示 `Classification was not saved.`，保留此前覆盖，不显示成功回执。
若提交结果因连接中断不能确认，显示 `Save status unknown; inspect classification before retrying.`，
用只读 classify 命令核对；不能在此情形断言“未保存”。
保存后的视图发布失败是已保存但显示待刷新，不能与持久化失败混为一谈。

## 4. 状态与转换

| 状态 | 呈现与恢复 |
| --- | --- |
| normal | 所选列表/统计、明确范围、unknown 数量；无待办动画 |
| filtered empty | No auxiliary sessions in this selection.；可改 all，不建议重建 |
| empty index | No indexed sessions.；给出 agentdeck session scan，但不自动执行 |
| unknown / legacy metadata | UNKNOWN，保留在 work，显示受控原因及检查命令 |
| partial | 保留已知记录，注明覆盖不完整；未知归类，未知计数本身不可知时写 unavailable |
| stale / publication pending | 显示最后成功 publication 的时间与范围；不混入新代 total |
| loading | 显示 Requested scope，不显示新范围的伪造零值；浏览器可退出 |
| unavailable | 无可信列表/计数时不显示 0，诊断走 stderr/JSON error，给 doctor；不暗中扫描 |
| invalid / ambiguous / not found | 明确输入问题与恢复方式，无写入 |
| saved / cleared / no change | 明确结果、effective/automatic 与发布状态；修改只发生在 classify 命令 |
| write failed / save unknown | 分开表达确定失败和无法确认的提交，使用查询核对后再重试 |

已有有效浏览器列表的范围加载失败：保留旧列表和旧 Scope，显示 requested scope failed；
Escape 返回旧列表，s 可重新选择。没有旧结果则错误页，q/Escape 退出。
快照数据不可读和仅分类字段缺失不同：后者允许 unknown，前者不能伪造完整 unknown 列表。

## 5. 布局、宽高与退化

普通文本 80 列为常规标本，40 列堆叠次要信息，先省模型/项目次要列，不截断类别和警告。
完整 ID 和可复制命令在详情逐行输出；长命令可自然折行，复制后不得插入额外续行参数。
复制命令保持单条逻辑行；浏览器标本仅自然折行，不插入换行字符或伪造可执行续行。
文本以可见单元宽度计算，控制字符消毒；CJK、emoji、组合字符不得截断编码或把控制码送终端。
JSON 保留合法转义后的完整值。管道输出不按未知宽度截断数据。

全屏至少 48×10。保留标题/范围/警告最多 4 行、分页/帮助 2 行，剩余区域容纳列表；
警告较长使用一行摘要并在只读详情显示完整信息，不能吞掉退出帮助。
窄屏省次要列；不足最小尺寸只显示 resize 指示和 q/esc，不显示被裁掉的可操作行。
Resize 保留 canonical 选中项、范围和各详情页状态，调整 viewport 使选中可见。
范围变化后保留仍可见的选中身份，否则选新集合首条；空集合无选择。
Browser 40 列使用 tooSmall，普通文本 40 列仍可用。

共享标本增加 `auxRows=8|10|24|40` 和 `auxSelected=1|8|12` 舞台轴；全屏状态由
columns/rows 自动计算，不要求手工选 tooSmall 才退化。小于 48 列或 10 行时一律拒绝可操作列表。
浏览器与 scopeFailed 使用 12 条合成工作视图记录，前 4 行固定标题/范围/状态，末 2 行固定
row/visible 范围及帮助，中间容量为 rows−6；选中第 8/12 条时自动平移 viewport 保持 > 行可见。
80×10 与 80×24 同宽不同高度也必须呈现不同容量。scopeMenu/loading 保留明确行边界及退出帮助。
终端框的高度按 rows×固定行高绘制，补空白行到完整帧；框外舞台/命令标签不计入终端行预算。

## 6. 输入生命周期与无障碍

- 列表：Up/Down/Home/End 选择，PageUp/PageDown 翻 viewport，Enter 打开只读详情。
  `s` 打开范围选择菜单（Work/Auxiliary/All）；Up/Down 选择，Enter 应用，Escape 取消。
- 详情 Escape 回列表，列表 Escape 退出；范围菜单 Escape 仅取消。`q` 在任一层退出。
  不抢占详情既有分页/section 键；范围选择仅在列表可用，详情保持打开时的身份。
- 独立 Escape 的歧义等待须有界，沿用现有输入解码器，不引入无限等待。
- Ctrl-C、中断、EOF、context cancellation、读写错误与 resize 竞态均停止 reader 后恢复 raw mode、
  光标、alternate screen 和 signal handler；退出后不得吃掉 shell 输入。
- 读取只在需要时获取已提交数据，不因等待按键占有独占 state 锁。
- TUI 不承担覆盖写入；用户通过逐行 classify 显式操作。两种方案中选择该方案是为了保留
  当前 READ ONLY 契约与可脚本化复现；放在 TUI 里的编辑对话框虽少一步，但会扩大状态/确认边界。
- 不使用动画传递唯一状态；加载文字常驻。英文文字、ASCII 类别和 > 选择标记在无色时完整可用。
  全屏拒绝非 TTY 或 TERM=dumb 时，stderr 给出 `Use: agentdeck session list --session-scope work`。

## 7. 字段需求与 Verification design

架构须逐项供应或明确拒绝：effective/automatic class、reason code、rule version、override 状态；
canonical identity、selection 的 total/unknown_count、publication identity/time、availability/partial/stale；
写入 saved/no_change/uncertain 与 publication_pending。unknown_count 的分母是分页前选定会话集合；
auxiliary 范围为 0，all/work 显示各自未知数量；不可知为 unavailable。
不要求原始正文、完整 source path、线程提示或凭据。browser/文本/JSON 必须共享事实，不能各自分类。
最终 JSON 键位置、CLI 错误代码/退出码映射、事务与投影读写方案由 architecture 定义，
输入错误沿用退出 2、运行错误沿用现有分类，不能用本框架自造 wire 字段替换稳定合同。

| 风险 | 后续实现的可观察 oracle |
| --- | --- |
| 默认与显式三范围 | work/auxiliary/unknown、任务子代理、同 ID 不同 client 的夹具；文本与 JSON 默认分列；显式集合一致 |
| 分页与统计 | 筛选后 total/next-page，--all 与 --session-scope all 区分，搜索文档/会话数分列；旧 JSON golden |
| 覆盖与恢复 | set/clear/no-change、歧义/不存在、事务失败/提交结果未知、保存后发布失败；事件/token 不变 |
| TUI 状态 | 纯状态机覆盖 s 菜单、取消、保留选择、空态、失败回退与独立详情页 |
| 终端生命周期 | 真实 PTY 覆盖 48×10/80×24/120×40、40 列拒绝、resize、Escape、Ctrl-C、EOF、故障、raw/cursor/reader 恢复 |
| 宽度与隐私 | CJK/emoji/组合字符、恶意控制码、长 ID、无色、pipe、TERM=dumb、正文/路径未泄漏 |
| 数据供应 | 按入口扫描矩阵验证：会话/分类/TUI 切换无扫描；stats 默认扫描及 --no-scan；signals 始终尝试扫描、stderr progress 与 scan_incomplete/partial；范围不增加扫描轮次。另验分页懒加载、并发发布一致性与锁释放 |

本轮浏览器标本只证明字符、标签、状态覆盖和宽度呈现，不证明 PTY、原生读屏、数据读取或持久化。
截图和源文件身份由 [manifest.json](prototype/cli/manifest.json) 固定；没有真实日志或生产运行测量。

2026-10-26 初稿的 25 场景 × 4 宽度检查仅证明字符渲染及横向溢出，不能证明全屏高度合同。
该限制及原截图保留于 Round 1 历史；本轮修复的尺寸检查与新增截图见 manifest 和修复交接。
现有开发依赖缺少无关的 widget-contrast-probe 所需 axe-core，未安装或修改依赖；
CLI 页面可渲染，未据此宣称 axe 无障碍检查通过。

## 8. 决定、假设与后续核对

已决定：默认遵循需求；参数名 --session-scope 与原有 --all 不混用；分类入口 classify；TUI 保持只读。
假设：架构能提供单代范围投影和有界查询；字段缺失不能被 UX 默认为已完成。
本轮不扩大需求，没有新增用户决策阻塞。architecture 必须确认字段、兼容容器、错误与保存状态，
之后按依赖影响重新核对本文件和共享标本；本 framework Draft 不代表 final-surface 已通过。
