---
status: active
created: 2026-10-27
updated: 2026-11-08
---

# 辅助会话隔离：Settings 最终表面

## 依据、阶段与边界

依据 [需求](../requirements.md)、[菜单栏](menubar.md)、[CLI](cli.md) 与 [架构](../architecture.md)
（当前 blob `416d63c922b9ea445a53a9774388efa13776af1c`）。需求只规定 Settings「提供默认查看范围、分类说明与
覆盖管理入口；准确控件、文案和布局由 UX 决定」。

**2026-11-07 重新设计。** 用户看过已评审通过的旧表面后判定其 UX/UI 不可接受，授权推翻旧设计、
以 AgentDeck 现有设计语言重新设计。旧设计的最后版本在提交 `2c90d4d`（integration-repair-r5）。
它的 PASS 只对旧表面成立，不迁移到本文件。本文件是重新设计候选，**需要独立评审**。

本轮改变的是位置、控件、写入时机、信息层级和文案。下列内容保持不变：需求、架构协议、
状态机（`prototype/src/auxiliarySettingsModel.js` 及其 14 项合同检查）和来源状态语义。
本轮不修改产品 Go/Swift、真实偏好、会话日志或用户数据库。原生 `settings-control` 实现按旧设计完成，
需要按本文件重做，见[影响](#影响)。

**2026-11-08 分页定为默认。** 用户 2026-11-07 看过重新设计后提出「整个 settings 太长了，是不是要考虑多 tab 页的设计」。
这是要求，不是待选项；上一版却把分页写成「候选，待用户确认」，原型默认仍是长列表，所以 Round 8 评审和原生实现都按长列表进行，
原生窗口的控件样式也没有对照原型。本版把工具栏分页写成唯一的产品布局（见[窗口分页](#窗口分页)），
并补上原生整窗的视觉对齐与验收要求（见[整窗视觉对齐](#整窗视觉对齐)）。
Round 8 的 PASS 只对旧 blob `2b873cd65cb8e6c68cdd900f5ca41533de04e011` 成立，本文件**需要重新评审**。
「会话」组自身的行、控件、状态机和文案不变。

复用[共享原型](../../../../prototype/README.md)的 SettingsWindow，新增「会话 / Sessions」页。
入口 `?settings=1&settingsPane=sessions&auxSettings=normal&auxMenu=normal&tab=sessions`。
源码为 `AuxiliarySettings.jsx`、`auxiliarySettingsModel.js`、`auxiliarySettings.css`，
内容身份与截图见 [manifest](prototype/settings/manifest.json)。

## 旧设计为什么不行

| 旧表面 | 问题 | 新做法 |
| --- | --- | --- |
| 纵向网页表单：标签、换行、通栏下拉框 | 同一个窗口里，其他组都是「左说明、右控件」的设置行，这一组像拼进来的另一个产品 | 用同一套 `.settings-field` / `SettingsRow` 行 |
| 三个值用原生下拉框，另加「保存默认范围」按钮 | 其他偏好全部选中即生效，只有这里要先选再保存；窗口里出现了唯一一个草稿态 | 分段控件，选中即保存 |
| 「默认版本：0」、请求 ID、`revision 4`、`origin-v1`、原始回执块 | 协议字段直接露给用户，用户读不懂，也不需要读 | 平时不显示；只在需要用户行动时出现（未知结果时的请求 ID），其余放进 tooltip |
| 查找、结果、覆盖列表、来源说明全部平铺在主列表 | 一组设置占掉大半个窗口，低频维护操作挤占高频偏好 | 主列表只留一行摘要和「管理…」，维护操作进 sheet |
| 七段说明文字散落各处 | 字号、间距几乎一样，看不出主次 | 每行只有一句说明；解释跟着状态出现 |
| 窗口专门放宽到 520 pt，另设 320 pt | 与其他设置窗口宽度不一致 | 沿用 460 pt；长 ID 换行 |
| 舞台控件 `position:fixed` 盖在窗口上 | 原型本身看起来就乱 | 舞台控件排在窗口上方，不再浮盖 |

## 布局

设置窗口按工具栏分页（见[窗口分页](#窗口分页)）。「会话」是第二页，排在「通用」（含菜单栏偏好）之后、「实时用量」之前：
默认范围决定菜单栏「会话」页下次打开显示什么，与菜单栏偏好相邻。页内三行；这一页只有一组，组标题不显示，页名已在标题栏和工具栏里：

```
                              会话
            ⚙ 通用    ▣ 会话    ∿ 实时用量    ◔ 额度
─────────────────────────────────────────────────────────────
默认会话范围                                     ┌────┬────┬────┐
菜单栏「会话」页与 Widget 下次打开时使用；      │工作│辅助│全部│
「工作」含来源未确认的会话。                     └────┴────┴────┘
⚠ 状态行（仅在有话要说时出现） · 操作链接
─────────────────────────────────────────────────────────────
手动分类                                               [管理…]
已手动分类 2 个会话，1 个未生效
─────────────────────────────────────────────────────────────
自动识别
只把后台维护会话识别为辅助；任务子代理始终算作工作。
● claude-mem    可用 · 15.2.1
● Codex 后台    1 条后台会话记录
```

「管理…」打开「手动分类」sheet。sheet 从标题栏下方落下，盖住窗口正文；打开期间窗口正文 inert：

```
┌ 手动分类 ───────────────────────────────────────────────┐
│ 自动识别有误时，把单个会话改为工作或辅助。只影响会话统计，│
│ 不删除会话，也不改动原始日志。                           │
├──────────────────────────────────────────────────────────┤
│ [状态条：仅在有话要说时出现]                             │
│ 查找会话                                                  │
│ [Codex|Claude] [ 完整会话 ID               ] [查找]       │
│ 完整 ID 可在菜单栏的会话详情或 agentdeck session list 中复制│
│ ┌ research                                    Claude ┐   │
│ │ old-01                                             │   │
│ │ 分类                         [自动|工作|辅助]      │   │
│ │ 当前计入   工作 · 手动                             │   │
│ │ 自动识别   未知 · 缺少来源信息                     │   │
│ │ 选「自动」只撤销手动分类，不删除会话。             │   │
│ └────────────────────────────────────────────────────┘   │
│ 已手动分类 (2)                                            │
│ ┌ Claude old-01                          工作   移除 ┐   │
│ │ Codex  archived-1                      辅助   移除 │   │
│ │ ⚠ 未匹配到会话，未生效                             │   │
│ └────────────────────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────┤
│                                                  [完成]   │
└──────────────────────────────────────────────────────────┘
```

视觉全部取自现有 token：分组标题 11 pt `--dim`，行标题 13 pt，说明 11 pt `--dim`，
分段控件与开关沿用 `.settings-segmented` / `.switch`。sheet 用 `--surface`，卡片与列表用 `--bg` 做内凹。
语义色仍只有四档：good 表示已确认，warn 表示需要注意，bad 表示失败，其余一律中性。
主按钮「完成」用 `--accent-strong`（白字对比度 ≥ 4.5:1），状态操作链接用 `--accent`。
「移除」用中性色：破坏性的小动作不该是全行最抢眼的元素。
客户端标签（Codex/Claude）是 `--surface-3` 小胶囊；会话 ID 用等宽字体并允许任意处换行，不截断。

## 窗口分页

<!-- 兼容锚点：本节 2026-11-08 前名为「窗口分页（候选，待用户确认）」。 -->
<a id="窗口分页候选待用户确认"></a>

用户 2026-11-07 提出设置窗口太长、要考虑多页设计；2026-11-08 起工具栏分页是**唯一的产品布局**，单页长列表不再是产品状态。
依据：加入「会话」组之后，单页设置窗口有 5 组 15 行，内容约 1325 px；900 px 高的屏幕上可见约 762 px，要滚动约 1.7 屏。
按 macOS 偏好设置的惯例改成工具栏分页后，每一页都不用滚动。各组的行和合同都不变，只换容器。

| 页 | 图标（原型 / SF Symbol） | 内容 | 窗口高度（原型实测，含标题栏与工具栏） |
| --- | --- | --- | --- |
| 通用 | GearSix / `gearshape` | 开机时启动、自动更新；「菜单栏」小节：显示内容、统计范围 | 433 px |
| 会话 | Stack / `square.stack.3d.up` | 本文件的三行 | 367 px（中文）/ 390 px（英文） |
| 实时用量 | Pulse / `waveform.path.ecg` | OTel 采集开关、状态与配置预览（scan-performance） | 308 px 起 |
| 额度 | Gauge / `gauge.with.dots.needle.50percent` | 订阅额度 7 行（subscription-quota） | 611 px |

划分依据是「用户为什么打开设置」，而不是按技术来源：通用是装好后调一次的外观与启动项，
会话是「看谁、怎么分类」，实时用量和额度各自有独立的授权与外部副作用（写 Codex 配置、写 Claude statusLine）。
「菜单栏」只有两行，单独成页太薄，放在「通用」页里，保留小节标题。

- 标题栏显示当前页名（「通用」「会话」…），工具栏与标题栏是同一块材质。选中页用 `--surface-2` 圆角底，
  图标改用 accent 色的实心版；未选中页用中性色。
- 窗口宽度固定 460 pt，高度跟随当前页内容，只有超过屏幕时页内才滚动。只有一组的页隐藏组标题，因为页名已经说明了；
  「通用」页保留「菜单栏」小节标题。
- **切页只换显示，不重建页。** 窗口打开期间四页的实例都保留，切走只是隐藏。各组仍需行动或有外部副作用的状态
  在切页往返后原样保留，包括：写入事实（例如实时用量刚写入 Codex 托管段）、配置所有权（关闭采集时据此移除
  AgentDeck 托管段）、未解决的结果（写入被拒、恢复冲突、保存未知、冲突）和进行中的确认（写入同意区、已有配置冲突区）。
  切页不能让任何一组退回初始状态，也不能清除仍需用户处理的结果。新显示的页从顶部开始。
  关窗之后的保留规则不变，仍由各组自己的文档规定。
- 同一次 App 运行内记住上次停留的页，关窗再开回到原处；首次打开为「通用」。
  其他表面需要直达某一页时（例如额度提示引导用户开启读取），以页 key 打开。
- 打开「手动分类」sheet 时，工具栏和正文一起 inert，不能切到别的页；「会话」页比 sheet 矮，
  sheet 打开期间窗口暂时增高到能放下它（原生对应 sheet 高于父窗口时的系统行为）。
- 键盘与辅助功能：`tablist`/`tab`/`tabpanel`，roving tabindex，左右方向键切页；面板由当前 tab 标注。
  原生工具栏分页由系统提供对应的键盘与 VoiceOver 行为，需原生验收确认。
- 原生：现有 `SettingsWindow` 改为以 `NSTabViewController`（`tabStyle = .toolbar`）为内容，
  每页一个 `NSHostingController`，窗口标题和高度由系统随页切换并带动画；各组视图（`SettingsRow` 等）不变。
  四个页控制器在窗口生命周期内常驻，切页不重建，也不用随页变化的 `.id` 让 SwiftUI 重建页内视图。
  写入事实、配置所有权、未解决结果与进行中的确认放在 App 级 controller（现有 `TelemetrySettingsController`、
  `QuotaSettingsController`、`SessionOriginSettingsController`），视图不持有；页实例常驻只是第二道保障。
  工具栏选中项的底色和图标着色由系统绘制，这是允许的原生差异；页的顺序、名称、图标和每页内容必须与原型一致。

这一改动改变的是整个 Settings 窗口的容器：scan-performance（实时用量）与 subscription-quota（额度）两组
各自成为一页，行、文案和合同不变。原型 `settingsTabs=0` 仍能打开旧的单页长列表，只用于对照和重放历史检查，不是产品状态。

## 整窗视觉对齐

原型是设计真相，原生 Settings 窗口要按**整窗**对照原型，不能只对照新加的一组。2026-11-08 用户看到的原生窗口有三处与原型不符，
本节把对应规则写死：

| 元素 | 原型 | 原生规则 |
| --- | --- | --- |
| 分段控件（显示内容、统计范围、默认会话范围、读取间隔、提醒阈值、sheet 的客户端与分类） | 选中段为中性：`--surface` 底加细阴影，文字正文色（`.settings-segmented button.active`） | 系统默认的 `Picker(.segmented)` 外观；**不得**用 accent 填充选中段。窗口根部不施加 `.tint(DesktopVisualTheme.accent)`，分段控件也不单独设 accent tint |
| 开关 | 开启态 accent | 每个 `Toggle(.switch)` 自带 `.tint(DesktopVisualTheme.accent)`（现状即如此，保留） |
| 次要按钮（管理…、查看连接配置等） | 中性：`--surface-2` 底、`--line` 边、正文色文字 | 系统中性按钮（`.bordered` 或默认样式，不设 tint）；文字不用 accent |
| 主按钮 | 只有 sheet 的「完成」：`--accent-strong` 底、白字 | `.borderedProminent` + `.tint(accentStrong)`，只用于「完成」 |
| 状态操作链接 | `--accent` | `.buttonStyle(.link)`，accent |
| 设置行 | 左侧标题加说明，右侧控件；说明折行时整行随之增高 | 任何文字都不得与分隔线或相邻行重叠；行高按自然高度测量 |

复选框（额度的客户端选择）沿用系统样式，归 subscription-quota 定义，本文件不改。

## 原生整窗验收

- 四页逐页渲染整个窗口内容（不只渲染「会话」组），中英 × 深浅各一套；会话页另渲染 sheet。
- 每张渲染都要满足：宽度不超过 460 pt；测得的高度等于自然高度（把同一内容放在更高画布顶部，逐行像素一致），
  所以任何行不会被挤压重叠；900 pt 屏幕上每页不出现滚动。
- 分段控件的选中段是中性色：在选中段内取样，饱和度低，不是 accent / accentStrong 的橙色。
- 与原型截图（manifest 的 `tabs-*.png`）并排目视核对：页序、页名、图标、每页的行与控件类型一致，控件着色规则符合上表。
  允许的差异只有系统绘制的部分：标题栏与工具栏材质、工具栏选中样式、系统控件的精确尺寸与字体渲染。
- 原生窗口在页间切换时标题与高度随页变化；关窗再开回到上次停留的页；sheet 打开期间不能切页。
- 切页往返回归：实时用量写入成功 → 切到通用再切回 → 关闭采集，必须走移除托管段的分支；
  写入被拒、恢复冲突、写入同意区和已有配置冲突区在切页往返后仍显示且可操作；
  「会话」页的状态行与在途保存在切页往返后保持。原型对应 `prototype/tools/check-settings-pane-lifecycle.js`。

<!-- 兼容锚点：本节原名「默认范围」，既有评审记录按旧锚点引用。 -->
<a id="默认范围"></a>

## 默认会话范围

| 元素 | 行为 |
| --- | --- |
| 分段控件 | 工作、辅助、全部。选中项永远是**最近确认**的值。点击另一项就发起一次保存，没有草稿态 |
| 读取中 / 读取失败 | 三项都不选中且禁用，绝不显示猜出来的「工作」；失败时状态行给「重新读取」 |
| 写入在途 | 选中态停在已确认值上；目标项带脉冲点（`aria-busy`），并略微变淡 |
| 说明 | 「菜单栏「会话」页与 Widget 下次打开时使用；「工作」含来源未确认的会话。」 |

点击当前已选中的项不会发送请求。保存成功后，已打开的菜单栏保持当前临时范围，下次打开时使用新默认。
Widget 按新的持久默认生成下一份允许发布的快照，仍受 WidgetKit 刷新时机约束，不能宣称立即更新。
CLI 文本默认 Work、JSON 默认 All 的需求不变。本操作不影响全部 token、金额、额度或采集范围。
应用重启后须读回持久值；原型只模拟同一 App 页面内的窗口关闭和重开，刷新网页会重置合成数据。

**为什么选中即保存。** 旧设计要求先选再保存，理由是「能在覆盖前看清目标，也能区分草稿与已保存值」。
新设计用另外的方式满足这两点：分段控件永远只表示已确认值，在途写入只用脉冲点表示，
所以不存在「看起来选了但其实没保存」的状态；三个值都可逆，下一次点击就能改回。
窗口里其他所有偏好也是这样工作的。

## 手动分类

### 入口行

标题「手动分类」。说明行是一句摘要：

- 列表读取失败：「覆盖列表暂不可读」
- 没有覆盖：「纠正自动识别有误的单个会话」
- 有覆盖：「已手动分类 N 个会话」，其中有未匹配的覆盖时追加「，M 个未生效」

sheet 关着时，如果一次覆盖写入仍在途，或者失败、结果未知、冲突、待发布、带权限警告，
这一行也显示对应状态，用户不必打开 sheet 才知道还有事没完成。

### 查找

客户端分段控件（Codex/Claude）、完整会话 ID 输入框（等宽字体，回车即查找）、「查找」按钮。
不按目录、项目或模型猜测，不提供批量规则；必须选客户端，避免同一 ID 跨客户端歧义。
ID 为空时「查找」禁用。查找只读，不会触发隐式扫描。修改客户端或 ID 会立即清除旧结果，
旧卡片上的控件不能作用到新输入。查找输入属于窗口草稿，关闭窗口即丢弃。

找不到会话，或会话已被现有 exclude 隐藏，都显示同一句「没有找到可见的会话。请确认客户端与完整 ID。」，
既不泄露被排除的会话，也不绕过访问边界。

### 会话卡片

| 区域 | 内容 |
| --- | --- |
| 头部 | 安全项目标签（粗体）、完整 ID（等宽，可换行）、客户端标签 |
| 分类 | 分段控件：自动、工作、辅助。没有覆盖时选中「自动」；点击即写入覆盖，或者撤销覆盖 |
| 当前计入 | 有效分类与来由：「工作 · 手动」「辅助 · 自动识别」「未知 · 自动识别，归入「工作」范围」 |
| 自动识别 | 自动分类与可读原因：「辅助 · claude-mem 后台维护」「未知 · 缺少来源信息」「未知 · 识别来源不可用」 |
| 脚注 | 「选「自动」只撤销手动分类，不删除会话。」 |

work、auxiliary、unknown 始终是三个不同的值；unknown 不是失败，也不是辅助。
受控 reason code 和规则版本（如 `maintenance_origin · origin-v1`）只放在「自动识别」一行的 tooltip
里（原生用 `.help`），不出现在正文。分类原因不显示正文、路径、系统提示或凭据。

写入后的读回规则：

- 设为工作或辅助成功后，按确认结果更新当前计入和覆盖；自动结果不伪装成用户选择。
- 选「自动」会发出 core-only clear，只撤销覆盖，不删除会话、usage 或日志。
  clear 的回执不带当前分类（`classification_available=false / not_queried`），所以卡片仍显示时，
  控制器紧接着另发一次只读的可见会话查询，用它的结果补上「当前计入」和「自动识别」。
  查询完成前两行显示「正在读取…」。查询失败时显示「当前分类暂不可读」和「重新读取」，
  不沿用 clear 前的值，也不用回执去推断。
  会话未匹配或已被 exclude 时同样保持不可用，不绕过源隐私边界。（原型的查询是同步的，不模拟失败。）
- 没有有效覆盖时，CAS 成功的撤销结果为 no_change，不递增目标 revision。
- 持久化成功但派生发布未完成时显示「已保存。会话视图尚未更新，列表可能暂时是旧的」；
  不把投影延迟报告成覆盖没有保存。

### 已手动分类列表

每行依次是客户端标签、完整 ID、目标类别胶囊和「移除」。行的顺序按 client/ID 稳定排序。

- 已匹配的行，主体是按钮：点击后把客户端和 ID 填进查找栏并打开它的卡片。
- 未匹配的行（原始会话已不存在，或恢复后尚未匹配）不可点击，第二行用 warn 色写
  「未匹配到会话，未生效」。它不会被套到另一个会话上；可以移除这条**覆盖记录**。
- 列表为空时显示虚线框「还没有手动分类的会话」。读取失败时显示「列表暂不可读，不会当作没有覆盖」
  并给「重新读取」，绝不用空列表冒充已清除。
- 「移除」直接提交条目的 core 键、expected_revision 和 request_id，不要求 sessions 里存在可见会话。
  在途时，该按钮换成转圈图标，行保持原位，直到结果确定。
- 正式数据有界分页（每页 20 项，稳定 client/ID 排序），页码、总数与可见列表同版本；
  标本只有两条覆盖，不假造分页控件。不提供「清除全部」；新增覆盖只能通过精确查找。
- 撤销后的 tombstone 不在列表中展示，但内部保留版本以防 ABA；这不是删除源会话或覆盖的历史版本号。
  旧回执不能删除当前更高 revision 的覆盖；重放之后重新读取列表，再决定显示什么。

## 自动识别（只读）

标题「自动识别」，说明「只把后台维护会话识别为辅助；任务子代理始终算作工作」。这同时是需求要求的分类说明。
下面固定两行来源：圆点 · 名称 · 状态，三列对齐。数据都来自只读命令 `session origin-source status` 的 JSON
（字段与 reason_code 枚举见[架构](../architecture.md) §7），Settings 不自行读库或配置文件，
不执行命令，不提供路径编辑器，不自动探测 HOME，也不启动插件。reason code 只放在 tooltip 里。

**claude-mem（可配置适配器）**

| 状态（reason） | 圆点 | 文案（中文 / English） |
| --- | --- | --- |
| 可用 | good | 可用 · 15.2.1 / Available · 15.2.1 |
| source_not_configured | 中性 | 未配置 · 可通过 CLI 设置 / Not configured · set up with the CLI |
| source_unavailable | warn | 无法读取元数据 / Metadata unreadable |
| source_version_unsupported | warn | 版本暂不支持 / Version not supported |
| source_disabled | 中性 | 已停用 / Disabled |
| source_not_yet_read | 中性 | 尚未读取 / Not read yet |
| source_config_unreadable | warn | 本机来源配置不可读 / Local source configuration unreadable |
| core_unavailable | bad | 本机用量库不可读 / Local usage database unreadable |

**Codex 后台（内建 OTel 来源）**：没有 configured/enabled 控件。

| 状态 | 数据字段 | 圆点 | 文案（中文 / English） |
| --- | --- | --- | --- |
| 有记录 | available=true、background_sessions>0 | good | N 条后台会话记录 / N background session records |
| 未开启 | available=true、0、otel_receiver_disabled | 中性 | 无记录 · OTel 采集未开启（见「实时用量」）/ No records · OTel capture is off (see Live usage) |
| 已开启无记录 | available=true、0、no_background_sessions | 中性 | 无记录 · 采集已开启，尚无后台会话 / No records · capture is on, no background session yet |
| 开关未知 | available=true、0、otel_receiver_unknown | 中性 | 无记录 · 采集状态未知 / No records · capture state unknown |
| core 不可读 | available=false、null、core_unavailable | bad | 本机用量库不可读 / Local usage database unreadable |

三种「无记录」都不是错误，也不等于来源不可用。有记录时，即使接收器已关闭，也显示计数（历史标记仍然有效）。
未知 reason_code 显示中性「状态原因暂不可显示」，不显示为可用。采集开关只在「实时用量」组，这里只做说明。

两行下方的说明按失效位置区分，两种情况不能共用一句：

- **外部来源不可用**：claude-mem 因 core 以外的原因不可用（未配置、已停用、元数据不可读、版本不支持、
  本机来源配置不可读），core 仍可读。说明为「来源不可用时，自动识别结果为「未知」并计入工作；手动分类不受影响。」
  此时 core 里已确认的覆盖继续有效，仍可对可见目标设置或撤销覆盖。
- **core 不可读**（两行都是 core_unavailable）：默认范围、覆盖、自动识别结果都存在 core 里，此时一项也读不到。
  说明改为「本机用量库暂不可读：当前分类和手动分类都无法读取，暂时不能修改。已保存的手动分类不会被删除。」，
  后接「重新读取」。不承诺手动分类可用，不推断自动识别为「未知」，也不把读取失败前看到的覆盖当作仍然生效。
  「手动分类」行摘要为「覆盖列表暂不可读」；sheet 顶部状态条说明「本机用量库暂不可读，暂时不能查找或修改手动分类。」，
  查找控件禁用，列表按[已手动分类列表](#已手动分类列表)显示不可读并给「重新读取」。
  重新读取成功后，来源状态、覆盖列表和默认范围都按读回结果显示；仍然失败则保持本状态。

配置存在不代表映射覆盖已经验证。外部来源不可用不等于默认偏好读取失败；core 不可读时两者同时发生。
标本的可用状态是合成元数据快照，不代表真实插件的运行状态。

## 写入状态与反馈

一次只接受一个写操作（默认范围或任一覆盖）。状态出现在**发起它的控件下面**：
默认范围的状态显示在该行说明之下；覆盖操作的状态显示在 sheet 顶部的状态条里，sheet 关着时显示在「手动分类」行。
另一个控件被锁定时，只用一句中性提示说明原因（「默认范围的修改尚未完成，完成后可更改」）。
每次操作绑定类型、client/ID（若适用）、expected_revision 和请求 ID；操作被接受后与窗口视图分离。

| 状态 | 色调 | 文案（默认范围 / 覆盖） | 操作 | 锁定 |
| --- | --- | --- | --- | --- |
| loading | 中性转圈 | 正在读取已保存的范围… | — | 是 |
| loadFailed | bad | 无法读取已保存的范围，不会假定为「工作」。 | 重新读取 | 是 |
| saving | 中性转圈 | 正在保存为「辅助」… / 正在把 old-01 设为「辅助」… / 正在移除 archived-1 的手动分类… | — | 是 |
| saveFailed | bad | 未能保存，仍为「工作」。/ 未能保存，分类保持不变。 | 重试 | 否 |
| saveUnknown | warn | 无法确认是否已保存，确认前暂停修改。＋请求 ID | 查询结果 | 是 |
| saveUnknown（回执仍未知） | warn | 仍无法确认是否已保存。＋请求 ID | 再次查询 · 按原请求重试 | 是 |
| conflict | warn | 已在别处被修改，没有覆盖。 | 读取最新值 | 是 |
| reloaded | 中性 | 已读取最新值「全部」，如需更改请重新选择。/ 已读取最新状态，如需更改请重新选择。 | — | 否 |
| saved | good | 已保存。已打开的菜单栏保持当前范围，下次打开时生效。/ 已保存。会话视图会在下次更新时反映。/ 已移除手动分类，恢复自动识别。 | — | 否 |
| pending | 中性 | 已保存。Widget 会在系统下次刷新时更新。/ 已保存。会话视图尚未更新，列表可能暂时是旧的。 | — | 否 |
| saved + 权限警告 | warn（另一行） | 已保存，但状态文件的权限加固失败。可运行 agentdeck doctor 检查，已保存的值不会回滚。 | — | 否 |
| noChange | 中性 | 没有变化。 | — | 否 |
| replayed | 中性 | 这个请求此前已经完成；这里显示的是最新状态。 | — | 否 |
| invalidRequest | bad | 请求 ID 与内容不一致，没有写入。 | — | 否 |

规则（与旧设计的合同一致）：

- 只在持久化确认后显示「已保存」；状态行不用 spinner 或短暂 toast 区分成败，
  图标、颜色和文字三者同时表达；错误一直保留，直到下一次读回或下一次成功操作，不会定时消失。
- saveFailed 时可以重试同一操作，也可以改选后发起新操作。
- saveUnknown：「查询结果」按同一请求 ID 读回执。回执仍未知时，提供「按原请求重试」：
  复用 App 级 pending 保留的 request_id、目标、负载和 expected_revision，不从窗口草稿生成新请求，也不递增请求 ID。
  核心先查同 ID 回执：已提交就返回原回执，不再执行，也不覆盖较新的值；未提交才按原 CAS 执行一次。
  确定的 saved / no_change / replayed 结束 pending 并解锁本组；回执不是当前分类，当前值按独立读回显示。
  若遇到 CAS 冲突，保持锁定，直到「读取最新值」结束旧 pending；用户重新选择后才能发起新请求。
  若仍然超时或未知，保留 pending，以及查询和同 ID 重试的入口。
- conflict：其他 CLI 或窗口的修改已经发生，禁止覆盖新值；读取最新值后由用户重新选择，不自动重放旧选择。
- 请求 ID 只在 saveUnknown 时显示（标签「请求」加等宽 ID，单独一行）：用户可能需要用 CLI 核对。
  其他状态都不显示 revision、请求 ID 或回执字段。
- 原型中，saving 由舞台的 Complete pending 模拟后台完成；它不是产品里的「强制成功」按钮。
  saveUnknown 标本在独立的模拟服务端已完成写入，只是客户端没收到确认，读回后才展示原回执；
  receiptMissing 标本读不到确认回执，仍保持未知，不因为「查不到」就推断没保存。
  冲突标本读回的是模拟的外部最新值（默认 All，或覆盖 Auxiliary）；真实实现必须接受实际结果
  （未落盘、已落盘、目标消失等），不把某一种 fixture 当成通用结论。

## 窗口生命周期

Escape 或关闭按钮关闭窗口；sheet 打开时，Escape 只关闭 sheet。查找输入是窗口草稿，关闭即丢弃，不再二次询问。
saving / unknown / conflict 保存在 App 级状态，关闭窗口不会撤销已接受的写入，重开后仍可查证。
状态行和在途目标在窗口关闭、重开后保持。
应用退出后的请求收尾、去重和读回属于 architecture，不授权取消请求，也不授权换一个新 ID 盲目重写；
恢复时只使用保留的原请求身份。覆盖必须跨重启和重建保留，purge-index 不得意外清除它；
backup/restore 的身份匹配和可移植边界按需求落实。

## 与菜单栏/Widget 的边界

默认偏好与菜单栏的临时选择不是一回事；在 Settings 保存不会立刻切换当前的会话页。
提交覆盖可以改变之后发布的成员集合，但不改变当前选择器的 scope 字符串。
列表、会话数、项目数、时长和信号需要同代更新；partial / stale 继续显式表达。
本标本把默认偏好连到下次 Popover 初始化，但不模拟覆盖向菜单栏所有投影的发布，
也不伪装 Widget 刷新或真实存储；这些由架构与最终表面验收覆盖。
默认值无法读取时，正式客户端不得以未知偏好生成新快照，只能保留已有的可靠快照，或者显示不可用。
默认偏好向消费者同时传递值和确认状态；未确认的值不是 Work，也不能把空值 fallback 成 Work。
本标本的 loading / loadFailed 没有旧的可靠偏好或快照来源，所以无论关闭 Settings、重开 Popover 还是切 tab，
都显示「默认会话范围尚未确认；会话视图不可用」，不生成范围选择、列表和计数。
只有明确读回成功后才初始化新的 selection。本合成读回确认没有配置，所以用 Work；
真实读回到已保存的默认时必须用实际值，不因为这个 fixture 返回 Work 就把默认写死。

## 键盘、焦点与辅助功能

- ⌘, 打开设置窗口，Escape 关闭。分段控件是 `radiogroup`，每个选项是 `radio` 并带 `aria-checked`；
  原生实现为 `Picker(.segmented)`。说明文字通过 `aria-describedby`（原生 accessibility hint）绑定到控件。
- Tab 顺序：默认范围 → 管理… →（sheet 内）客户端 → ID → 查找 → 卡片分类 → 列表行 → 移除 → 完成。
- 打开 sheet 时焦点进入 ID 输入框（锁定时落在 sheet 本身）；关闭后回到「管理…」。
  sheet 是 `aria-modal` 对话框，窗口正文 inert。
- 焦点封闭在 sheet 内，Tab / Shift+Tab 首尾循环：在 sheet 内最后一个可聚焦控件（通常是「完成」）按 Tab
  回到第一个，在第一个按 Shift+Tab 到最后一个；焦点停在 sheet 本身时，Tab 进入第一个、Shift+Tab 进入最后一个。
  焦点不会离开仍打开的 sheet，落到 inert 的正文、工具栏或窗口外。锁定时输入控件禁用，循环在仍可用的
  状态操作（查询结果、按原请求重试、读取最新值、重新读取）和「完成」之间进行，所以这些操作始终可以用键盘到达。
  Escape 只关闭 sheet、关闭后回到「管理…」的规则不变。原生 `.sheet` 由系统提供模态焦点范围，仍需原生验收确认。
- 状态行是常驻的 polite live region（`aria-atomic`），只在内容变化时朗读；操作链接放在 region 外面，可以单独聚焦。
- 移除一行后，焦点移到下一行的「移除」；没有下一行就回到 ID 输入框。失败时焦点留在原处。
- 未知结果可以用键盘到达「查询结果」；回执仍未知时可以到达「按原请求重试」；冲突时可以到达「读取最新值」。
  禁用的控件仍然可读，锁定原因写在状态行里。
- 深浅主题都只用现有 token。文案中英完整。native VoiceOver、Dynamic Type、焦点恢复，以及窗口与 App 的生命周期，
  需要单独验证；浏览器截图不能替代。

## 原生实现映射

| 原型 | SwiftUI |
| --- | --- |
| 设置行 | 现有 `SettingsRow(label:note:status:statusAction:)`；状态行为 `SettingsRowStatus`，操作为 `.buttonStyle(.link)` |
| 默认范围 | `Picker(selection:)` + `.pickerStyle(.segmented)` + `.labelsHidden()`，系统默认外观、不设 accent tint。绑定的 getter 返回已确认值，setter 提交保存；读不到时 selection 为 nil，控件禁用 |
| 在途标记 | 目标项旁边的小圆点；或者在 `SettingsRowStatus` 用 `ProgressView().controlSize(.small)` |
| 管理… | `Button("管理…")`，系统中性按钮、不设 tint；`.sheet(isPresented:)` 挂在 Settings 窗口上 |
| sheet | 固定宽度跟随窗口；内容 `ScrollView`；底部「完成」是 `.keyboardShortcut(.defaultAction)` 的 `.borderedProminent` 按钮，tint 用 accentStrong |
| 客户端 / ID / 查找 | `Picker(.segmented)` + `TextField`（`.monospaced()`，`.onSubmit` 查找）+ `Button` |
| 会话卡片 | 圆角背景容器；事实区是固定标签列的 `HStack` 行（`Grid` 遇到折行文字会少算高度，导致行间重叠） |
| 覆盖列表 | `VStack` + `Divider` 行；「移除」是 `.borderless` 按钮，在途时换成 `ProgressView` |
| reason code / 规则版本 | `.help(...)` |
| 来源圆点 | `Circle().fill(...)`，6–7 pt，颜色取 `DesktopVisualTheme` 的 good/warn/bad/dim |

窗口宽度保持现有 `SettingsWindowView` 的 460 pt，不另设宽度。窗口容器见[窗口分页](#窗口分页)，
控件着色与行高见[整窗视觉对齐](#整窗视觉对齐)，渲染验收见[原生整窗验收](#原生整窗验收)。

## 已定协议与验收设计

Settings 只调用 Go JSON 接口，沿用 versioned envelope，不解析文本，也不直接读 SQLite。

| 动作 | 协议与使用的数据 |
| --- | --- |
| 默认读取/保存 | session scope get/set；confirmed/value/revision/reason；未确认不补 work，保存时携带最后确认的版本 |
| 查找/设置 | session classify --id <id> --client <client>，可见性与绑定验证；origin 的 automatic/effective/reason/rule/revision |
| 撤销后的当前分类 | clear 成功后另发一次同一个只读 classify 查询；失败时保持不可用，不用回执推断 |
| 枚举覆盖 | session classify list --page/--limit；条目键/value/binding_status/revision，计数与页一致 |
| 撤销 | classify --clear 加明确 client；不依赖源存在；回执 override=null，automatic/effective=null，classification_available=false/not_queried |
| 未知结果读回 | classify --request <request-id>；已知回执保留原目标、outcome、revision；未确认则 outcome=unknown |
| 来源状态 | 由 Go 提供只读的安全元数据；sourceUnavailable 不会导致用户覆盖被删除，也不会让按钮永久禁用 |

请求 ID 由 App controller 在提交前分配并保留；重建窗口不生成新 ID。重试同一操作时复用 ID 和负载；
用同一个 ID 提交不同负载为 invalid_request。origin_conflict 时读取最新版本，再由用户重新选择，不能自动覆盖。
default revision 与各覆盖键的 revision 相互独立；clear 留下的版本 tombstone 保留，不能因为删除了记录就回到 0。
原型模拟独立版本、回执以及确定或未知的结果，但不把本地模型称作真实 CAS、数据库事务或崩溃恢复的证明。

| 领域 | 已定合同 / 后续可观察 oracle |
| --- | --- |
| 偏好 | 确认值、是否已读取、版本；控件只显示确认值，在途单独标记；保存后跨重启、跨窗口读回 |
| 覆盖 | canonical 身份、automatic/effective、override、reason/rule、matched 状态；撤销不改原始数据 |
| 并发 | request ID/期望版本、单写者或等价保障；与 CLI 并发冲突时不静默丢失更新 |
| 查询 | 有界读、稳定分页、未找到与不可用分开；输入改变即清除旧目标；排除边界不泄漏 |
| 操作 | failure/unknown/confirmed/pending 分开，重试幂等；关闭、重开及应用重启不制造假成功 |
| 集成 | 新默认用于下次打开与 Widget，临时范围不被覆盖；全部消耗不变，没有额外的日志重扫 |
| 验证 | 正常、失败、未知、冲突、保存中关闭重开、no-change、unknown 自动结果、未匹配覆盖、中英与深浅主题 |

字段缺失时按已定的 availability/unknown 规则降级，不允许用假成功代替需求；本文件不重新定义架构数据库或迁移方案。
不以演示用的 reason/rule 值宣称支持真实维护源。

## 影响

- **原生实现（任务 6 settings-control）**：当前未提交的实现已经按 2026-11-07 版重做了「会话」组和 sheet，
  控制器 `SessionOriginSettingsController`（写入状态机、请求身份、读取代次与 ASC-SC-R4-F1 的已应用计数）继续适用。
  它仍是单页长列表加 `ScrollView`，并在窗口根部施加 accent tint，使全窗分段控件和次要按钮变成橙色；
  额度组「客户端」行的说明折行后压到下一行。按本版需要重做窗口容器（工具栏分页）、去掉窗口级 accent tint、
  修正行高，并按[原生整窗验收](#原生整窗验收)补逐页整窗渲染回归。它在 Repair 3 后的待复评候选随之作废，
  重做后整体重新评审。
- **其他专题**：scan-performance（实时用量）与 subscription-quota（额度）两组各自成为一页，行、文案和合同不变；
  它们的颜色随窗口级 tint 去掉而改为本文件[整窗视觉对齐](#整窗视觉对齐)的规则，与共享原型一致。
  共享原型的 scan-performance 历史检查脚本 `scan-performance-repair-check.mjs`、`scan-performance-otel-linkage.mjs`
  验证的是长列表，已固定带 `settingsTabs=0`，检查内容不变。
- **评审**：Round 8 的 PASS 只对旧 blob `2b873cd65cb8e6c68cdd900f5ca41533de04e011` 成立；本版需要重新评审后才能作为实现依据。
- **不受影响**：需求、架构、CLI、菜单栏和 Widget 的合同；状态模型的合同检查；「会话」组自身的行、文案与状态规则。

## 自检 — 2026-11-07（重新设计候选）

- 共享原型 `npm run build` 通过。
- `node prototype/tools/check-auxiliary-settings-final.mjs`：14 项模型合同检查，0 失败
  （模型只新增 `lastOperation` 字段，用于决定状态显示在哪个控件下，不改变任何转移）。
- `prototype/tools/check-auxiliary-settings.js` 已按新界面重写，每组 103 项，
  中英 × 深浅四组共 412 项，0 失败。覆盖：分组位置与窗口宽度、主表面不出现协议字段、八种场景、
  未确认默认不猜值、读取恢复与消费者初始化、选中即保存、锁定规则（saveFailed 不锁）、
  在途脉冲点只在 saving/saveUnknown 出现、请求 ID 只在未知时出现、关闭重开后状态保持、
  sheet 模态（正文 inert、Escape 只关 sheet、焦点进出）、clear 后自动独立查询、点击已选项不发请求、
  列表行打开卡片、未匹配覆盖不可点、移除后的焦点、换客户端清除旧卡片、未找到不提供写入、
  默认只影响下次打开、receiptMissing 同 ID 恢复、replayed、claude-mem 三种降级、Codex 五态、
  覆盖写入在途时 sheet 关闭后仍报告、权限警告。
  首轮 zh/light 曾有 1 项因页面尚未就绪而失败，连续重跑均通过；脚本已加入就绪等待，四组随后全部重跑通过。
- 分页候选（`settingsTabs=1&settingsPane=sessions`）：同一脚本在分页窗口下另加 8 项检查（页的顺序、
  当前页独占正文、标题即页名、tabpanel 标注、roving tabindex、方向键切页、关窗再开记住页、sheet 打开时工具栏 inert 且窗口增高），
  中英 × 深浅四组各 111 项，共 444 项，0 失败。首轮有 3 项失败，原因是脚本的选择器先匹配到菜单栏 popover 自己的 tab；
  选择器限定到 `.settings-tabs` 后，四组全部重跑通过。长列表模式复跑 zh/dark、en/light 各 103 项，0 失败。
  scoped axe（`.settings-window`）：通用页 zh/dark 0 violations、0 incomplete；会话页 en/light 0 violations，
  2 项 incomplete（`#as-default-help`、`#as-detect-help`，axe 判定元素部分重叠，无法确定背景；`--dim`/`--bg` 浅色手算 4.81）；
  额度页 zh/light 0 violations，4 项 incomplete，全部是 subscription-quota 两条禁用行的文字，同样是背景无法确定（禁用控件的文字不在 WCAG 对比度要求之内）。
- 未启用本组参数时，`prototype/tools/check-auxiliary-menubar.js` 176 项，0 失败（分页改动后复跑仍为 0 失败）。
- scoped axe 4.12.1（wcag2a/wcag2aa）：主表面（zh/dark normal、en/light sourceUnavailable）均为
  0 violations、0 incomplete。sheet（zh/light 卡片、en/dark conflict）0 violations，
  有 1 项 incomplete `color-contrast`（`#as-sheet-desc`，遮罩重叠，axe 无法确定背景色）。
  对它和 sheet 内其余文字的 token 对手算对比度：`--dim`/`--surface` 深色 4.96、浅色 5.08，
  `--muted`/`--surface-2` 6.40 / 5.04，warn/`--surface-2` 6.54 / 6.63，accent 链接/`--surface-2` 5.17 / 4.95，
  白字/`--accent-strong` 4.61 / 5.42，均 ≥ 4.5。
- 截图（已目视核对）：`redesign-group-zh-dark.png`、`redesign-group-source-unavailable-en-light.png`、
  `redesign-sheet-zh-dark.png`、`redesign-sheet-conflict-en-light.png`、`redesign-unknown-zh-dark.png`。
  旧截图保留为历史，不代表本设计。

以上只是内存模型和浏览器呈现的验证，不证明 Go CAS、磁盘提交、真实来源或 Widget、
原生辅助功能，也不证明请求故障能够恢复。

## 修复自检 — 2026-11-07（Round 7 后）

- ASC-SET-R7-F1：[自动识别](#自动识别只读)按失效位置区分说明。core 不可读时，两行来源都显示「本机用量库不可读」，
  说明改为当前分类和手动分类都不可读、暂时不能修改、已保存的不会被删除，并附「重新读取」；sheet 状态条说明查找与修改暂不可用。
  外部来源不可用仍用原说明，手动分类照常可写。标本里 core 不可读以 loadFailed 表示，重新读取成功后来源、覆盖、默认范围按读回结果恢复。
- ASC-SET-R7-F2：sheet 打开期间 Tab / Shift+Tab 在 sheet 内首尾循环；焦点因控件被禁用而丢失时，下一次 Tab 回到 sheet。
  窗口外的原型舞台控件不受影响。
- ASC-SET-R7-F3：在「默认会话范围」前保留 `默认范围` 兼容锚点，既有评审记录的链接恢复。
- `check-auxiliary-settings.js` 新增 10 项：焦点循环 6 项（含锁定的 saveUnknown 下只能在「查询结果」和「完成」之间循环），
  core 不可读与恢复 4 项；三种 source* 场景还断言说明属于外部来源一类。长列表每组 113 项，中英 × 深浅共 452 项，0 失败；
  分页每组 121 项，共 484 项，0 失败。模型合同 14 项 0 失败；菜单栏 176 项 0 失败；`npm run build` 通过。
- 真实按键复核（agent-browser `press`，不是合成事件）：长列表 en/light 和分页 zh/dark 下，焦点在「完成」上按 Tab 到客户端选项，
  按 Shift+Tab 回到「完成」，sheet 保持打开，焦点始终在 sheet 内。
- scoped axe 4.12.1（loadFailed）：主表面 en/light、zh/dark 都是 0 violations、0 incomplete。sheet 0 violations，
  incomplete 只有 color-contrast（遮罩下无法确定背景）：en/light 为 `#as-sheet-desc`、状态条 bad 文字、查找说明里的 `code`，
  zh/dark 为 `#as-sheet-desc`、`#as-lookup-help`。手算 bad 文字对状态条背景：浅色 5.93，深色 4.69；其余文字沿用上文 token 对，均 ≥ 4.5。
- 只验证合成模型和浏览器呈现，不证明原生 `.sheet` 的焦点范围、VoiceOver 或真实 core 故障恢复。

## 自检 — 2026-11-08（分页定为默认）

- 原型：`SettingsWindow` 默认分页（`settingsTabs=0` 才回到长列表对照）；`settingsPane` 缺省为「通用」，
  不带 `auxSettings` 时只有通用、实时用量、额度三页（实测）。`npm run build` 通过。
- 模型合同 `check-auxiliary-settings-final.mjs`：14 项，0 失败。
- `check-auxiliary-settings.js` 从 `?settings=1&settingsPane=sessions&auxSettings=normal&auxMenu=normal&tab=sessions` 运行：
  分页窗口中英 × 深浅四组各 121 项，共 484 项，0 失败；`settingsTabs=0` 长列表 zh/dark、en/light 各 113 项，0 失败。
  菜单栏 `check-auxiliary-menubar.js` 176 项，0 失败。
- scan-performance 长列表入口（加 `settingsTabs=0`）实测仍是长列表且 OTel 开关存在；未重跑这两个脚本，
  因为它们会覆盖 scan-performance 的历史截图。
- 每页窗口高度（视口 1280×1000）：通用 433、会话 367（中文）/ 390（英文）、实时用量 308、额度 611 px，均无页内滚动。
- 截图（已逐张目视核对）：`tabs-general-zh-dark.png`、`tabs-sessions-en-light.png`、`tabs-sessions-zh-dark.png`、
  `tabs-live-zh-light.png`、`tabs-quota-en-dark.png`、`tabs-sheet-zh-light.png`。原型的分段控件选中段均为中性底，
  次要按钮均为中性，这就是[整窗视觉对齐](#整窗视觉对齐)要求原生对齐的样子。
- 只验证浏览器呈现，不证明原生工具栏分页、窗口动画、VoiceOver 或原生渲染；这些由任务 6 的[原生整窗验收](#原生整窗验收)覆盖。

## 修复自检 — 2026-11-08（Round 9 后）

- ASC-SET-R9-F1：标本的正文容器原来以当前页为 key，各组按当前页条件挂载，切页会卸载实时用量组并丢掉其本地的
  托管所有权、结果和同意区。现在四页都挂载，非当前页只设 `hidden`（`.settings-group` 自带 `display:flex`，
  `settingsTabs.css` 显式隐藏），正文不再随切页重建，切页时把新页滚回顶部。[窗口分页](#窗口分页)写明切页保留规则，
  原生映射要求页控制器常驻、状态归 App 级 controller，[原生整窗验收](#原生整窗验收)增加切页往返回归。
- 新增 `prototype/tools/check-settings-pane-lifecycle.js`，按 Codex 标本四种情形运行：默认写入（写入同意区往返、写入后往返、
  关闭采集走移除分支、再开启重新要求同意）、写入被拒、恢复冲突、已有其他配置（冲突区往返、仅开启接收端后往返、关闭不移除用户配置）。
  每次往返还断言只显示当前页、切回的是同一个组实例。中英 × 深浅四组各 43 项，共 172 项，0 失败。
- 修复前复现：临时把实时用量组改回按当前页条件挂载，en/light 四种情形全部失败。默认写入情形与评审证据一致：
  写入后往返结果消失，关闭采集得不到 `removed`。之后恢复修复后的 `Settings.jsx`，SHA-256 与修复版一致，全量复跑通过。
- 评审的原路径（`?settings=1&auxSettings=normal&auxMenu=normal&tab=sessions&lang=en&theme=light`，实时用量 → 开启 → 写入并开启 →
  通用 → 实时用量 → 关闭）现在依次为 `written`、`written`、`removed`，再开启出现写入同意区。
- `check-auxiliary-settings.js` 只统计未隐藏的组；分页四组各 121 项、长列表两组各 113 项，0 失败。菜单栏 176 项、
  模型合同 14 项 0 失败，`npm run build` 通过。各页窗口高度不变：中文 通用 433、会话 367、实时用量 308、额度 575 px，
  英文 通用 468、会话 390、实时用量 325、额度 611 px；视觉没有变化，截图未重拍。
- 只验证合成浏览器标本；不证明原生页控制器的生命周期或真实 Codex 配置写入。

## 历史

2026-10-27 至 2026-11-04 的 framework、final-surface 与 integration-repair（R1–R5）自检，
以及 336 / 536 / 70 项浏览器断言、两组 axe、权限警告、Codex 来源行等记录，都针对旧表面，
保存在提交 `2c90d4d` 的本文件中；对应截图仍在 `prototype/settings/` 目录，manifest 的 `superseded_manifest` 原样保留。
这些记录证明的是旧设计满足它自己的合同，不能用来认证本设计。
