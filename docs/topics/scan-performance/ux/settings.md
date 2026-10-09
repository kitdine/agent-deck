---
status: active
created: 2026-10-09
updated: 2026-10-09
---

# Settings：显式启用 OTel 实时用量

本轮按用户要求补充的 final design candidate；尚未独立评审。
共享 [产品原型](../../../../prototype/README.md) 的独立 Settings 窗口是唯一标本。
入口 `?serving=rebuilding&settings=1&telemetry=unknown`。

## 开关与配置所有权

新增“实时用量 / Live usage”组，包含“启用 OTel 用量采集 / Enable OTel usage collection”。
默认关闭。开启仅启用 AgentDeck 的本地接收端，不自动覆盖 Codex exporter、模型或 Memories 配置。
首次连接通过“查看连接配置 / View connection configuration”预览；用户审阅并合并 Codex 配置。
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
恢复但无新事件回unknown，配置观察独立保留，开关反映实际listener状态。

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
