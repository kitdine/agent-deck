---
status: active
created: 2026-10-09
updated: 2026-10-09
---

# 菜单栏：打开即查看已准备的数据

设计阶段为 final candidate，尚无独立 PASS。依赖 [需求](../requirements.md)、
[架构字段供给](../architecture.md#coverage-与字段供给) 和 [先行评估](../performance-evaluation.md)。
使用共享 [产品原型](../../../../prototype/README.md)，不另建 popover。

## 打开与阅读

左键打开/关闭、右键或双击菜单、现有快捷键和默认额度 tab 保留。
打开操作只绑定内存中已准备的数据。App 启动恢复磁盘快照与后台更新均不占点击路径。
第一帧必须有可读数据才算 P01 的成功；窗口/骨架/spinner 不是可用数据。

人工验收分三次进行：App 已有数据时点击，检查立即可读；重启 App 后立即点击，
检查先恢复上次数据及其真实日期；写入一条已知用量后保持 popover 打开，检查数值自动更新。
三者计时起点和前置状态不同，不能拿扫描耗时与已有内存的点击耗时计算加速比。
现有点击本就不扫描，warm 路径允许测得没有改善；重点验证恢复和更新等待是否缩短。

已有快照：保留客户端、时段、面板选择以及可交互的内容；后台工作不禁用整个 popover。
顶部数据时间保留真实 age，后台状态在 header 下的一条简短说明中呈现。
后台完成时原子更新数据，不切 tab、不清筛选、不移动滚动位置或焦点、不关闭/重开 popover。
关闭界面不取消已接受的有限工作；重新打开直接绑定当前 publication，不播放旧进度。

初次没有数据：显示品牌和“正在准备数据”，不显示伪造 0 值卡片。
已存数据库的 bootstrap 不等历史扫描；部分数据到达即可用明确的 incomplete 状态展示。
缺失组用 —/unavailable，真正完整的零才使用原有 empty 状态。

全新安装先从今天相关日志准备第一批数值，header说明“今天已采集用量 · 历史正在补齐”。
今天的金额、tokens和已有可信计数先可读；未准备的项目/工作信号/额度等用—。
已有数据库则直接读今天，避免先准备全部历史视图。切换其他时段时按该窗口的
partial/unavailable规则展示，不能把第一批今天准备好解读为周/月全量已完成。
共享原型新增mixed：today usage可读、sessions/projects未知、7d unavailable、30d partial。
发布后原子切换memory并保留client/period/panel；只呈现状态，不运行真实首批调度。

当天值尚未入库时保留旧可用视图及日期，今天新值的等待独立报告。
若已跨天，昨天的today不再进入今天卡片；可查看带实际截止日期的已保存历史视图。

## 状态与文案

| serving 状态 | 英文主文案 / 说明 | 中文主文案 / 说明 | 数字 |
| --- | --- | --- | --- |
| memory | Data is ready / Showing saved data | 数据已准备好 / 当前显示已保存的数据 | 显示当前 publication |
| disk | Previous data restored / Checking for updates in the background | 已恢复上次数据 / 后台正在检查更新，不影响查看 | 显示旧数据，保留原窗口与 age |
| rebuilding | Updating in the background / Previous data keeps its original date and range | 后台正在更新 / 当前显示上次数据，日期和范围保持不变 | 保留旧 publication |
| partial | Preparing more data / Values contain collected data only and are incomplete | 正在补齐数据 / 数值仅包含已采集部分，尚不完整 | 不作完整总量承诺 |
| mixed | Today usage is ready; other data is still preparing | 今天用量已准备，其他数据仍在补齐 | today可读，session/project为—，7d未准备，30d仅部分 |
| first | Preparing your data / No data is available yet. It will appear when ready | 正在准备数据 / 尚无可用数据，准备好后会自动显示 | 无数字 |
| failed | Update incomplete / Previous data remains available. You can retry | 更新未完成 / 当前显示上次数据，可继续查看或重试 | 有效旧数字保留；Retry / 重试 |
| midnight | Today's data is not ready / Today shows —. Saved data for other periods remains available | 今天的数据尚未准备好 / 今天显示 —；其他时段仍可查看已保存数据 | today 不复用昨日值；其他有效视图可读 |
| invalid | Data cannot be read yet / Saved data did not pass validation. Preparing a new view | 暂时无法读取数据 / 已保存数据未通过检查，重新准备后才能显示 | 无效缓存不显示；Retry / 重试 |

OTel 的内部传输、writer、worker 数、generation 等细节不出现在日常 popover。
已知采集缺口显示在现有完整性/Health 详情，文案为“部分用量尚未收录” /
“Some usage is not recorded”；不因 OTel 关闭将所有 JSONL 数据解释成不可用。
未启用 OTel 的历史未知不是“收齐”，也不能显示一个无证据的缺失 token 数。

## 各面板与一致性

- 默认额度面板读取自己的最后 observation；慢网络/额度读取失败不阻塞 usage 和首屏。
  定时额度与 scan coverage 不是一个时钟，已有关闭额度状态照常工作。
- 用量、构成、归因共用 publication 的 scope/window/price revision；成本不完整继续沿用估算/未知规则。
- 会话和工作信号按架构的 domain availability 判读；usage 新、session 未准备时不得显示假新会话数。
- provider footer 可以是旧观察，执行切换时仍由真实命令验证当前状态，不用旧缓存执行写操作。
- 跨天/时区后，仅失效的视图显示 unavailable；有效历史视图仍可切换，不将整个 App 归为刷新失败。
- schema incompatibility、真正不可读数据库保留已有 signal/错误详情，不降级成“后台更新中”。

刷新按钮请求后台一次有限更新；已有重建时合并或安排 follow-up，前台不清空数字。
排队和执行可持续超过旧 120 秒界限，显示等待/更新，不自动变成失败。
真实失败才进入 failed；重试开始后旧数据仍可阅读。

## 布局与无障碍

复用 420 pt / 280 pt 的既有 popover、header、filters、cards 和 footer。
状态说明可换行，不用 ellipsis 吞掉 incomplete/失败含义；按钮有完整 accessible name。
一条 polite、atomic status live region 报阶段变化，不逐事件/逐文件播报；旧刷新 announcement
在该状态条出现时关闭，避免重复。更新不抢焦点；关闭/重开沿用原键盘行为。
新增状态条使用静态 icon 表达阶段，不逐事件刷新或反复显示百分比；header 沿用既有刷新控件，
尊重 reduced motion，不以额外动画打断阅读。

VoiceOver、Dynamic Type、AppKit focus、真实主线程 frame 和睡眠恢复必须 native 验收。
DOM 属性、截图与浏览器交互不等于这些已通过。

## 共享标本

入口 `?serving=rebuilding`；舞台控制支持上表全部九态，并提供“发布准备好的数据”。
舞台只模拟 publication，用户界面不出现该按钮或 publication 数字。
`lang=zh|en`、`theme=dark|light`、`width=420|280`、`tab=quota|usage|breakdown|attribution|sessions`
沿用共享原型参数。与旧 scan 舞台同时指定时 serving 优先，避免两个 live regions 互相冲突。

标本绑定 [manifest](prototype/serving/manifest.json)；状态与交互检查记录在
[checks](prototype/serving/checks.json)。截图直接来自同一共享原型：

- [恢复后的默认额度页](prototype/serving/disk-zh-dark-420.png)：恢复不等额度或 scan，保留数据时间。
- [后台重建的窄界面](prototype/serving/rebuilding-en-light-280.png)：数据可读，说明可换行。
- [首次准备](prototype/serving/first-zh-dark-280.png)：没有伪数字。
- [真实更新失败](prototype/serving/failed-en-dark-280.png)：旧数据与重试同时存在。
- [跨天](prototype/serving/midnight-zh-light-420.png)：today 没有昨日的值；可切其他时段。
- [部分成果](prototype/serving/partial-en-light-420.png)：partial 与完整结果可区分。

这些使用共享合成数据，证明状态、层级、双语文案、范围规则与交互；不证明缓存恢复速度、
真实数据库完整性或常驻进程生命周期。检查的实际范围和任何 limitation 以 checks 为准。

## Round 1 修复标本

入口`?serving=mixed&tab=usage`。today用量显示而session/project为—；切7d为未准备，
30d为部分结果，会话/额度域未准备。点击舞台发布后原子就绪，client/period/panel选择保持。

- [英文浅色420](prototype/serving/mixed-en-light-420.png)
- [英文深色280](prototype/serving/mixed-en-dark-280.png)
- [中文深色420](prototype/serving/mixed-zh-dark-420.png)
- [中文浅色280](prototype/serving/mixed-zh-light-280.png)

修复交互见[repair-checks](prototype/serving/repair-checks.json)，覆盖四组语言/主题及两种宽度。
这是合成availability与交互证据，不是native首帧或真实调度性能。
