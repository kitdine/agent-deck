# AgentDeck 产品原型

产品全部界面的唯一设计真相：菜单栏面板、小组件、设置窗口，以及 CLI 的逐字符输出。

它在 2026-08-20 从 `docs/topics/desktop-app/ux/prototype/interactive-v7/` 移到仓库根，
因为一份原型是整个产品表面的标本，不是某一个 topic 的资产。`work-signals` 需要同一份
标本，而在第二个 topic 下再放一份拷贝，等于制造第二个设计真相。

```bash
npm install
npm run dev -- --port 4175
```

| 页面 | 地址 |
| --- | --- |
| Popover | `http://127.0.0.1:4175/` |
| 12 个小组件 | `http://127.0.0.1:4175/?surface=widgets` |
| 状态一览 | `http://127.0.0.1:4175/?surface=states` |
| CLI 输出 | `http://127.0.0.1:4175/?surface=cli` |

CLI 那一页渲染的是逐字符的真实输出，不是示意图。终端里能被设计的只有分节、对齐列，
以及一个数字读不出来时写什么，所以那三件事必须以最终形态出现才可评。它的文本不随
语言开关变化：AgentDeck 的 CLI 输出是单一英文的，与面板不同。

URL 参数：`lang=zh|en`、`theme=dark|light`、`state=normal|empty|aged|partial|unavailable`、
`tab=usage|breakdown|attribution|sessions`、`signal=activity|workflow|tooling`、`settings=1`。
页面顶部的控制条不是产品界面，是原型舞台自己的开关。

菜单栏图标：左键开关面板，**右键或双击**弹出菜单（菜单栏显示内容 / 设置… / 关于 / 退出），
`⌘,` 也能直接打开设置。设置是**独立窗口**，不是面板里的一页。

## 与 v6 的差别

**筛选作用域一致化。** v6 的四个卡片里，用量和构成随客户端与时段变，归因和节律不变，
筛选器管得着一半、管不着另一半，点了没反应也没有任何说明。v7 把 tab 收敛成
**用量 / 构成 / 归因 / 会话** 四个全联动的面板；**节律**移出 tab、放在筛选器下方
独立一块，标题上写明"近 30 天 · 不受上方筛选影响"，往下滚就能看到。

**会话成为一个 tab。** 会话本来就能被客户端和时段过滤，和另外三个是同一类。
原来挂在用量面板底部的活动 / 工作流 / 工具三块也移到这里——它们讲的都是"会话里发生了什么"。

**出口走菜单栏图标。** 设置、关于、退出放在图标的右键（或双击）菜单里。
设置窗口里的每一项都对应实现里真实存在的偏好键：开机启动（`SMAppService`，失败时就地显示错误）、
定时刷新、菜单栏显示内容（成本 / Token / 仅图标）、菜单栏统计范围（全部 / 跟随面板筛选）。
更新检查本期不做，界面上也就没有它。

**状态信息各归其位。** 更新时间挪到右上角与刷新按钮成一组；`≈` 的解释（成本不完整）紧贴金额；
健康、部分不可用、读取失败统一成内容区顶部的提示条，点健康那条进入二级页面看全部检查项。
展开式的行内列表要么压住内容、要么把 footer 顶变形，两个问题这样都不存在。
于是 **footer 只剩服务商一行**，弹出方向朝上，箭头也就朝上。

**tab 选择器去掉数值。** 那行数值与 hero 重复，去掉后这一条从 61px 收到 40px，内容区多出的高度还给面板。

**小组件 large 改成真机比例。** v6 的 large 是 `width:100%` 跨列的横条（约 518×330，还随窗口变），
真机上不存在这个比例。v7 改成与 medium 同宽、约两倍高的竖版（286×330，对齐真机 338×354）。
换成真实比例后立刻暴露了七处内容溢出，都已按真实尺寸重排。

**新增状态一览页。** 空数据 / 过期 / 部分不可用 / 完全不可用四种降级，以及小组件的占位骨架，
在 v6 里一种都没有。占位骨架不含任何真实数字。

**一份数据。** popover 与小组件共用 `data.js`：各期总额、峰值、活跃天、最忙星期、会话统计
全部由 90 天日序列与 7×24 网格派生。v6 里同一屏会出现"热图说周二最忙、柱状图说周三最高"、
"今日视图显示 30 天的客户端小计"、"每根柱子的模型占比都一样"这类互相打脸的地方，
根因是同一份事实被写死了好几遍。

**视觉。** 深色为基准（对齐当前实现的观感），浅色并行一套；颜色语义收敛到
橙=金额与选中、蓝=活动量与时间、绿=可信、琥珀=需要注意、红=失败，模型另用一套低饱和分类色；
popover 内字号下限 10px、小组件内 9px；分段控件改成 macOS 的凹槽+浮起，
客户端用主色、时段降一级，两组过滤器不再视觉同权。

**两套外观各有一套语义色取值。** 语义只有上面那五档，但取值不是一套。`:root` 里的
橙/蓝/绿/琥珀/红是深色外观的取值，同时充当图形填充；浅色外观在
`[data-theme="light"]` 里另给一组更深的取值——同一支橙放到白底上只有 3:1，`--warn`
坐到自己的 tint 上更掉到 2:1。另有 `--accent-strong` 专门做"白字压在橙底上"的填充
（选中的客户端分段、主按钮）：`--accent` 本身太亮，白字在它上面只有 3.16:1，两套外观
都不达标。正文字色一律按 WCAG AA 4.5:1 取值，弱化靠 `--dim` 而不是 `opacity`——
透明度会把一个已经达标的颜色重新混回不达标。

改动任何颜色 token 后，两套外观都要重跑一遍无障碍自检（见下），不要只看截图：
对比度不足在截图上常常"看着还行"。

**中英可切。** 数字、货币、日期、相对时间一律走 `Intl`，不在字符串里写死 `$` 或 `en-US`。

## 需要 Go 侧补的数据

界面上凡是当前投影拿不到的，都带"待采集"标记，没有装作已经有数据：

1. **活动 / 工作流 / 工具**（会话 tab 里的三块）——活动分类、工具调用计数、首次编辑与迭代深度，
   `internal/usage/presentation.go` 里没有对应字段，需要新增一条解析 session 日志的采集管线。
2. **归因按时段分组**——`quality` 与 `pricing` 现在只在客户端 scope 下分组，不带 period 维度。
   界面按"两个筛选器作用于所有数据域"设计，落地需要投影补 period。
3. **会话按时段分组**——`sessions` 现在是"最近 N 条"，不随 period 变。

## 自检工具

```bash
# 布局：列出所有被裁切的容器（正常输出 NO OVERFLOW）
open 'http://127.0.0.1:4175/?surface=widgets&measure=1'

# 交互：模拟真实事件走一遍关键路径并断言（正常输出 ALL PASS）
open 'http://127.0.0.1:4175/?probe=1'

# 尺寸合同：逐条数出 ux/widget.md 的 size-as-depth 规格（正常输出 ALL PASS）
open 'http://127.0.0.1:4175/?surface=widgets&contract=1'
```

两个都只在带参数时运行。截图只能证明界面长什么样，证明不了点下去会怎样，
`probe` 补的是后者：筛选联动、图表悬停/钉住/键盘、服务商菜单与确认、提示条与健康详情、
信号详情进出、刷新失败与重试、右键与双击菜单、设置窗口的开关与菜单栏显示模式，
以及设置窗口的无障碍合同——四项偏好的解释文字是否真的通过 `aria-describedby` 成为控件的
description，登录项被拒后开关是否留在真实状态、失败行是否出现且带图标、是否只落在一个
live region 里、其余控件是否仍可用、以及失败行是否在下一次成功修改时清除而不是定时消失。
共 49 项断言，中英各跑一轮。

`aria-describedby` 与 live region 这两条只有查真实 accessibility tree 才算验到，DOM 属性
对不对不等于 accessibility tree 里有没有。`probe` 查的是 DOM，跨过这条线要用 CDP 的
`Accessibility.getFullAXTree` 读 `description` 与 `live`——改动 `Settings.jsx` 的语义属性后
按这个方式复核，不要只看属性写上去了。

`contract` 查的是 specimen 有没有兑现 `ux/widget.md:133-135` 的 size-as-depth 表：small 的
7 桶、medium 三期各带 cost 与 tokens 且柱子正好 20 根、轴跟着这 20 天、large 的 90 桶填充线
与三个 stat chip 且 peak 带日期，共 13 项断言，中英各跑一轮。它还查 chip 有没有被
`text-overflow` 吃掉——溢出量具只看容器有没有被裁，看不见一行文字被 ellipsis 截断，peak
的日期第一版就是这样 DOM 里有、屏幕上没有。

`contract.js` 里的 7 / 20 / 90 是从文档另抄的一份，**不要**改成从 `Widgets.jsx` import
常量：共用一个常量时，把它改错会同时挪动渲染和期望，断言就永远为真。断言要能失败，
期望值就必须独立于被测代码。

第四项自检不在页面里，需要一个能跑 axe-core 的浏览器。改过颜色 token 后按下面这样
逐组跑，`.popover` 是 scope，`theme` 必须两套都跑：

```bash
agent-browser open 'http://127.0.0.1:4175/?theme=dark&lang=zh&state=normal&tab=usage'
agent-browser a11y --tags wcag2a,wcag2aa --selector .popover --json
```

完整一轮是 `theme`（dark/light）× `lang`（zh/en）× `state`（normal/empty/aged/
partial/unavailable）× `tab`（usage/breakdown/attribution/sessions）共 80 组，
`color-contrast` 应为 0 个节点。面板扫不到的子界面另跑：健康详情（点提示条）、
服务商菜单（点 `.provider-entry`）、信号详情（`tab=sessions&signal=workflow`）、
设置窗口（`settings=1`，scope 用 `.settings-window`）、`surface=widgets` 与
`surface=states`（scope 用 `.stage-body`）。确认层 `.dialog` 渲染在 `.popover`
之外，scoped 运行覆盖不到它，靠 `probe` 与手算覆盖。
