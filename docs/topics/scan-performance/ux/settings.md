---
status: active
created: 2026-10-09
updated: 2026-10-30
---

# Settings：显式启用 OTel 实时用量

本轮按用户要求补充的 final design candidate；尚未独立评审。
2026-10-29 增补 [Codex 记忆联动与配置写入](#codex-记忆联动与配置写入)：用户决定由 AgentDeck 在征得同意后写入
Codex 配置，采集不自动开启、由用户确认；本节取代下文“不自动覆盖 Codex exporter”与“用户审阅并合并 Codex 配置”两处旧约束。
共享 [产品原型](../../../../prototype/README.md) 的独立 Settings 窗口是唯一标本。
入口 `?serving=rebuilding&settings=1&telemetry=unknown`。

## 开关与配置所有权

新增“实时用量 / Live usage”组，包含“启用 OTel 用量采集 / Enable OTel usage collection”。
默认关闭。开启时按[联动与配置写入](#codex-记忆联动与配置写入)征得同意后才写入 Codex 的 `[otel]` 受管段；
不改动模型、Memories 或其他 Codex 配置。“查看连接配置 / View connection configuration”预览始终可用，
供选择“我自己配置”或已有其他 exporter 的用户自行合并。
端口由接收配置提供，示例 4318 不是硬编码默认要求；本地 token 默认隐藏。

关闭时停止接收，不删除已有用量、历史观测和未处理 journal。用量总数不能因此减少。
默认接收端随 App 起停；App 退出后继续接收需要另行启用 independent mode。
独立模式不在本开关中被隐式开启。

现有 macOS App 已常驻。默认 JSONL 事件更新可以复用有限 helper，不因这个设置
新增常驻 Go 进程；显式 OTel 接收需要连续 listener，其生命周期仍遵守上述规则。
这里的实时指“有新记录就处理并发布”，不是打开 popover 时等待 OTel，也不是零延迟。
必须分别报告生产者导出、接收、计量和原生界面更新时间。
首次从已有日志整理今天、读取已存今天记录、恢复上次展示都不等待本开关开启或OTel到达。
OTel开启前的既有日志仍要走首次采集；实时接收不能凭空恢复之前没导出的历史。

本地 JSONL 的自动更新与 OTel 是两个独立控制。scan-performance 标本中的旧“定时刷新”
呈现为“自动更新本地数据”，解释为文件变化触发，无需设定周期；已有关闭偏好仍保留，
关闭后不偷偷监听/轮询，启动、手动和 CLI 的有限任务仍允许。额度自身的探测策略独立。
偏好沿用原键：已存的值（含明确关闭）保留，没有存值时为开启，与此前版本一直进行的文件监听一致
（2026-10-28 用户决定；此前截图标本中的默认关闭不适用于实现，2026-10-30 的交互原型已同步为默认开启；
旧截图仍按 manifest 中记录的原始内容状态保留）。说明文案中“OTel 采集单独控制”
一句随 `otel-capture-and-reconciliation` 的 OTel 开关一并加入。

## 状态

| 条件 | 中文 | English | 计量含义 |
| --- | --- | --- | --- |
| off | 采集已关闭 | Collection is off | 已存数据保留 |
| unknown | 接收端已就绪 · 等待首次事件，连接状态未知 | Receiver ready · Waiting for first event; connection unknown | 启用后的默认观察，不推断配置或持续连接 |
| unconfigured | 接收端已启用 · Codex 尚未配置 | Receiver enabled · Codex not configured | 不声称正在接收 |
| waiting | 已收到有效事件 · 等待新用量 | Valid event received · Waiting for new usage | 最近有事件，不保证持续连接 |
| ready | 正在接收 · 用量可计入 | Receiving · Usage can be counted | 仅在身份、去重及 transport 能力通过后 |
| capture_only | 正在接收 · 用量尚未计入 | Receiving · Usage is not counted yet | 保存观测，不加进总量 |
| disconnected | 已确认接收异常 · 保留已有数据 | Confirmed reception failure · Previous data is retained | 明确失败信号，不因静默产生 |
| paused | App 已退出 · 接收已暂停 | App exited · Receiver paused | 没有 independent lease 时 |

说明保留“仅接收用量、模型和必要身份字段，不保存提示、回答正文或工具输出”。
开关和状态分开判读：configured、listener running、actual observations、reconciled 是不同事实。
producer_configured为true/false/unknown，默认unknown。仅显式请求的脱敏配置检查可判定字段符合/缺失，
检查不可读仍unknown；文件符合不证明producer已经重启生效。认证、版本与格式均有效的事件证明最近收到事件，
进入waiting/receiving；OTLP/HTTP单次请求不证明持续连接。无事件、闲置或App重启都不推断未配置/断连。
明确bind失败、拒收/磁盘不可持久化或验证的producer错误反馈才报告异常及实际gap；
恢复但无新事件回unknown，配置观察独立保留，开关反映实际listener状态；唯一例外是 Codex 记忆关闭导致的停止，此时开关保持采集偏好并只能关闭（见[联动](#codex-记忆联动与配置写入)）。

## Codex 记忆联动与配置写入

依据：2026-10-29 隔离实验确认，目前只有 Codex 记忆（Memories）的后台请求不写本地日志、只能由 OTel 统计；
主会话与子代理已由本地日志统计（事实记录在 Bug `ad-bug-codex-subagent-usage-attribution`）。
guardian 审查与压缩请求尚未验证，验证确认同类后加入“需要采集的功能”，联动规则不变。

AgentDeck 以只读方式读取用户级 `~/.codex/config.toml` 的两项事实，Settings 打开与 App 启动时各读一次：

| 事实 | 取值 | 判定 |
| --- | --- | --- |
| Codex 记忆 | on / off / unknown | `features.memories = true` 为 on，`= false` 为 off；未设置、文件不可读或无法解析为 unknown（默认值随 Codex 版本变化，不推断） |
| Codex 导出配置 | managed / agentdeck / other / absent / unknown | managed：AgentDeck 写入的受管段；agentdeck：用户自行配置且指向本机接收端；other：已有其他 `[otel]`；absent：无 `[otel]`；不可读为 unknown |

命令行参数、profile 与 Desktop 内部设置无法读取，因此 unknown 不阻止开启，也不显示漏统计提示。

### 联动状态

| 条件 | 开关 | 中文 | English |
| --- | --- | --- | --- |
| 记忆 off，采集偏好关闭 | 不可用（接收端不运行） | Codex 记忆未开启 · 无需采集 | Codex Memories is off · Collection is not needed |
| 记忆 off 的说明 | — | 其他 Codex 用量已由本地日志统计。开启 Codex 记忆后可在此启用采集。 | Other Codex usage is already counted from local logs. Turn on Codex Memories to enable collection here. |
| 记忆 on，采集关闭 | 可用 | 警告：Codex 记忆的后台用量未统计 | Warning: Codex Memories background usage is not counted |
| 记忆 on，采集关闭的说明 | — | 开启采集并允许写入 Codex 配置后才能统计。 | Turn on collection and allow the Codex configuration change to count it. |
| 记忆 unknown | 可用 | 沿用上文“状态”表 | Same as the Status table |

记忆 on 时的漏统计警告同时出现在现有完整性 / Health 详情（“部分用量尚未收录 / Some usage is not recorded”）。
采集运行中把记忆关闭：接收端停止，已统计数据保留。此时开关**仍可用且显示为开**（它表示采集偏好，接收端是否运行由
状态行说明），只能用来关闭：

| 条件 | 开关 | 中文 | English |
| --- | --- | --- | --- |
| 记忆 off，采集偏好开启 | 可用，显示为开，只能关闭 | Codex 记忆已关闭 · 接收已停止 | Codex Memories turned off · Receiver stopped |
| 其说明 | — | 关闭采集可移除 Codex 配置中的 AgentDeck 段；保持开启时，Codex 记忆重新开启后恢复接收。 | Turn off collection to remove AgentDeck's block from the Codex configuration. If you leave it on, receiving resumes when Codex Memories is turned back on. |

在此状态关闭开关，按下文“写入、关闭与恢复的结果”处理（移除受管段、或保留用户自己的配置），之后开关变为不可用的
“Codex 记忆未开启 · 无需采集”；记忆重新开启后采集保持关闭，只显示漏统计警告，不按旧偏好自动恢复。

### 开启：征得同意后写入

采集**不自动开启**；记忆 on 也只显示警告和开关，由用户点击开启。点击开关后，在开关下方原位展开同意区（不用 modal，
不抢焦点外的控件）。同意区内容按导出配置事实决定：

| 导出配置 | 同意区 | 按钮 |
| --- | --- | --- |
| absent / unknown | 标题“开启采集需要修改 Codex 配置 / Collection needs a Codex configuration change”；显示文件 `~/.codex/config.toml` 与将追加的受管段（令牌隐藏）；说明“只追加这一段，不改动其他内容；关闭采集时移除这一段。重启 Codex 后生效。 / Only this block is appended; nothing else changes. It is removed when collection is turned off. Takes effect after Codex restarts.” | 写入并开启 / Write and turn on；我自己配置 / I'll configure it；取消 / Cancel |
| other | “Codex 已有其他 OTel 配置，AgentDeck 不会覆盖。请按连接配置自行合并。 / Codex already has another OTel configuration. AgentDeck will not overwrite it. Merge the connection configuration yourself.” | 仅开启接收端 / Turn on receiver only；取消 / Cancel |
| managed（AgentDeck 写入的受管段） | 不展开同意区，直接开启 | — |
| agentdeck（用户自行添加、指向本机接收端） | 不展开同意区，直接开启；该配置归用户所有，AgentDeck 永不修改或移除它 | — |

### 写入、关闭与恢复的结果

| 结果 | 开关 | 提示位置与样式 | 中文 | English | 处理 |
| --- | --- | --- | --- | --- | --- |
| 写入成功 | on | 状态行下方，信息色 | 已写入 Codex 配置 · 重启 Codex 后生效 | Codex configuration updated · Takes effect after Codex restarts | 状态行沿用上文（首次为 unknown）；下次成功变更清除 |
| 写入被拒（不可写、写入期间被改动或校验失败） | off | 开关所在行内，错误色并带图标 | 无法写入 Codex 配置，未做任何修改。可重试或选择自己配置。 | Could not update the Codex configuration; nothing was changed. Retry or configure it yourself. | 再次点击开关即重试；成功后清除 |
| 我自己配置 | on | 状态行下方 | 连接配置已展开，请自行合并到 Codex 配置 | Connection configuration is shown below; merge it into your Codex configuration | 自动展开“查看连接配置” |
| 关闭并移除受管段 | off | 状态行下方，信息色 | 已从 Codex 配置移除 AgentDeck 段 · 重启 Codex 后停止导出 | Removed AgentDeck's block from the Codex configuration · Codex stops exporting after restart | 已存数据保留 |
| 关闭（导出配置为用户自行添加） | off | 状态行下方，信息色 | 接收端已关闭 · 你自行添加的 Codex OTel 配置未改动 | Receiver turned off · Your own Codex OTel configuration was not changed | 已存数据保留；不触碰用户配置 |
| 关闭时受管段已被改动或不存在 | off | 开关所在行内，警告色并带图标 | Codex 配置已被修改，AgentDeck 未改动它。请手动检查 ~/.codex/config.toml 的 [otel] 段。 | The Codex configuration was changed elsewhere; AgentDeck left it untouched. Check the [otel] section of ~/.codex/config.toml. | 接收端已关闭；下次成功变更清除 |

结果提示使用已有的 polite/atomic live region（失败前已存在且为空），错误与警告分别使用错误色与警告色 token，
不只依赖颜色表达含义（带图标与文字）。任何结果都不删除已统计数据或 journal。

### 联动标本

舞台“Codex 标本”模拟记忆（on/off/unknown）、导出配置（absent/other/managed）、写入结果（written/refused）与
移除结果（removed/incomplete），可用 URL 参数 `codexMemories`、`codexExporter`、`codexWrite`、`codexRestore` 预置；
原型不读写真实 Codex 配置。标本：[记忆关闭](prototype/serving/settings-linkage-memories-off-zh-dark.png)、
[漏统计警告](prototype/serving/settings-linkage-not-counted-en-light.png)、
[同意区](prototype/serving/settings-linkage-consent-zh-light.png)、
[写入被拒](prototype/serving/settings-linkage-refused-en-dark.png)、
[已有其他 exporter](prototype/serving/settings-linkage-conflict-zh-dark.png)、
[恢复不完整](prototype/serving/settings-linkage-restore-incomplete-en-light.png)、
[记忆关闭后仍可关闭采集](prototype/serving/settings-linkage-memories-stopped-en-dark.png)、
[用户自配 exporter 关闭时保留](prototype/serving/settings-linkage-user-config-kept-zh-light.png)。
`prototype/tools/scan-performance-otel-linkage.mjs` 在 zh/en × dark/light 下逐项核对：记忆 off 且偏好关闭时开关不可用、
开启后关闭记忆时开关仍可关闭并移除受管段、之后记忆再开启采集保持关闭、用户自配 exporter 直接开启且关闭时不移除、
记忆 on 时只警告不自动开启、同意区原位展开且无 modal、取消不写、写入成功开启、写入被拒开关为 off 且失败行在失败前
已存在、重试后清除、已有 exporter 不覆盖、仅开启接收端与自己配置均展开预览、managed 直接开启、恢复不完整开关为 off
且为警告色、运行中关闭记忆即停止；结果见 [otel-linkage-checks](prototype/serving/otel-linkage-checks.json)，
全部通过。截图与检查不证明真实文件写入、恢复算法或 native 行为，这些属于实现验收。

## 交互与标本边界

状态条使用 polite/atomic live region；开关有 accessible name、checked 和解释文字 description。
状态改变不抢焦点；预览文本可换行；中英、深浅色和窄视口均保留完整含义。
舞台的“遥测连接标本”在产品窗口外，可模拟连接状态，不是用户自己设置的采集状态。
默认 off；关闭/重开 Settings 在当前页面保持选择。页面 reload 重置属于合成原型，不是偏好持久化验收。

配置预览明确“原型示例，未应用到真实 Codex”，不生成或输出真实 token。
原型不启动 listener、不编辑文件、不接收真实 OTel、不改变用量总数。
实际 listener 开启失败时 native 开关必须反映失败后的真实状态，此行为属于实现验收。

## 验证

必要检查：默认 off、开启后的unknown状态及有明确检查依据的未配置状态、预览未自动应用、capture-only、不删除已有数据、
关闭与重开、全部连接状态、双语/主题、live region/description、状态控件不被 modal 遮挡。
截图及源码身份在 [标本 manifest](prototype/serving/manifest.json)，
检查结果在 [checks](prototype/serving/checks.json)。
必要标本：[默认关闭](prototype/serving/settings-otel-off-zh-dark.png)、
[接收端启用但尚未配置](prototype/serving/settings-otel-unconfigured-zh-dark.png)、
[未计入的观测与配置预览](prototype/serving/settings-otel-preview-en-light.png)、
[小屏滚动窗口](prototype/serving/settings-otel-small-zh-light.png)。
四组语言/主题共 68 项交互通过。新增区域 axe 为四组零 violations，中文两组各有一项
color-contrast incomplete（几何背景判定），保留未自动认证标记；固定色值人工计算
深色 5.44:1、浅色 4.81:1，并核对图像。600×800 视口的窗口可容纳、正文可滚动且舞台可操作。
截图与浏览器检查不证明 Codex 生产者能力或 native 设置持久化；实验与 native 边界见
[先行评估](../performance-evaluation.md) 和 [任务分解](../tasks.md)。

## Round 1 修复标本

首次开启默认unknown：[中文深色](prototype/serving/settings-unknown-zh-dark.png)、
[英文浅色](prototype/serving/settings-unknown-en-light.png)。等待/接收只表示已观察的有效事件，
不存在由静默自动转断连的逻辑。四组语言/主题的切换与关闭见
[repair-checks](prototype/serving/repair-checks.json)。实际producer/listener仍留在实现验收。
