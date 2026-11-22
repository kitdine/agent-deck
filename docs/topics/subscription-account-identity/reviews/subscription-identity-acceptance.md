---
status: active
topic: subscription-account-identity
subject: subscription-identity-acceptance
---

# 订阅身份集成验收评审

## Round 1 — 2026-11-20，Claude 承载分支组合

### 📋 独立评审

📊 本轮范围评分 7/10

评审结论：FAIL

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

**SAI-T7-R1-F1**（P1）。`OpenRefreshCore` 持有打开 core 时的物理目录句柄。CLI 随后再按名称取 quota 刷新锁。若名称在两步之间改指另一合法目录，新的锁可通过 `CheckQuotaRefreshLock` 的条目所有权和文本路径检查。`RunRefresh` 此时调用 `BeginRefreshCycle`，能在后续 delivery guard 检查前尝试推进旧 core 的 `quota_cycle.seq`。独立静态评审和主控代码核对一致，未观察到实际错库写入。现有回归在取得原锁以后替换路径，覆盖另一顺序。

修复应在首次周期推进和探测前核对锁取得时的物理目录和已打开 core 的目录身份，复用已持有的描述符及原锁条目检查。使用隔离用例验证 core 打开后、取得锁前替换的顺序，使不匹配在周期增量及探测前被拒，并保留未更换根的成功对照。

#### 🟡 建议改进

无另行发现。

#### 🟢 通过的范围

已核对三个主线冲突的前后像、模式和回放树，保留原有物理条目所有权、截止检查及两边回归。`doctor.Service.LockCheck` 验收清单和拒绝探针与已评审基线覆盖一致。最终清单及三个带标签的入口探针通过，包含 `LockCheck`、限时取得刷新锁及 serving 验证。Store race 145 个顶层测试和 16 个相关交互、消费者测试的未变化范围可复用，未覆盖 F1 的顺序。

#### 📝 摘要

独立 OpenCode `openai/gpt-6.2-astra` 评审，T3 配置 `variant=xhigh`。签名来源 `1f80036d8777643e6b596f48c7c7db24a603a056`，已验证目标 `929e95233a2658d3ed956c58cd88b01c16d7190c`，隔离结果树 `32999f846ccd6632ebfe5e3f4f8c89bbe9ea33d9`。三个冲突修复树为 `80f13d7162df8a76f9c9368338d8cf040ceb277c`，后续只增补验收测试 blob `09d287a1d899dc16ae5ad8ea8fbf4a671e5c97a2`。主控按相同身份复算关键实现，核对复放、标签验收及实际调用顺序。

最终 Go、race/vet/build、宿主 XCTest、隐私、沙箱和桌面集成检查留到 F1 修复后的相关状态。原签名来源的 N1 与 N4 回执仍记录各自实际二进制行为，需核对受新锁证明影响的端到端适用性。无变动的 N2 渲染证据可复用。真实 WidgetKit 宿主的读取、呈现和到期显示仍待证明隔离后验收。用户选择 Task 7 完整通过后开正式 PR。

## Round 2 — 2026-11-20，物理锁绑定复评

### 📋 独立复评

📊 本轮范围评分 10/10

复评结论：PASS

完成门禁：NOT_VERIFIED

#### 🔴 严重问题

无。

#### 🟡 建议改进

无新增发现。

#### 🟢 已关闭发现

**SAI-T7-R1-F1 CLOSED。** `RunRefresh` 在启动刷新周期前，比较取锁时目录句柄与已打开 core 的目录句柄。独立红对照在只加回归的树上无错误，返回序号 1、库内序号 1、探测 1 次。修复后同一替换顺序返回 `ErrLockLost`，返回序号、库内序号和探测次数均为 0。同根对照保留 1 次推进和 1 次探测。既有条目所有权、token 检查和绝对截止继续保护其他顺序。当前生产入口均经过 `RunRefresh`。

#### 📝 摘要

Claude Code Opus `claude-opus-5-5` 冷上下文复评，T3 配置 `effort=xhigh`。来源签名提交 `1f80036d8777643e6b596f48c7c7db24a603a056`，目标 `929e95233a2658d3ed956c58cd88b01c16d7190c`。修复前树 `32999f846ccd6632ebfe5e3f4f8c89bbe9ea33d9`，补丁 `d3b89a0fdb1bf2267607fe5208b68b4f05539550d4f784bffd58f225278efb4e`，修复树 `19f3a581c21abca5b60d2e114f1eb45337a12c5b`。独立评审重放补丁。主控核对 7 项交付校验和、3,689 个固定路径的 blob 与模式、三路径差分、签名和作用范围日志。

Go 1.27.3 串行全量为 30 个包通过，另 1 个包无用例。定向 race 50 条通过、1 项大小写不敏感卷跳过。Store、quota、desktop、backup 的 broader race 497 条通过、7 项跳过。CLI race 单独复跑 467 条通过、4 项跳过。默认并行时 6 个有时限的用例失败，独立串行复跑和串行全量均通过。vet、Darwin 构建、隐私、Widget sandbox、桌面刷新集成通过。隔离宿主 XCTest 的 Shared、App、Widget 分别执行 198、273、73 项，均无失败，主 App suite 有 3 项跳过，其中 2 项在各自重启阶段通过。内嵌 helper 与该树的 CLI 构建字节一致。Linux 交叉构建在目标 main 已有的 macOS 专用调用处失败，未列入本轮 Task 7 验收。

本轮完成锁修复复评及可执行的宿主检查。原 N1/N4 观察绑定旧 helper，最终 helper 的真实 App 探测、发布与通知仍待补证。无变动的 N2 渲染按原观察范围沿用。N3 的真实 App Group 消费、三种尺寸和到期呈现仍需可验证的隔离 WidgetKit 宿主。正式 PR 等 Task 7 完整通过。

#### 最终构建验收补证

- **N1。** 在签名隔离 App 中绑定实际窗口、内嵌 helper 和合成 Claude CLI。树 `19f3a581c21abca5b60d2e114f1eb45337a12c5b` 的五步诊断依次观察到两个窗口 82% 和 3%、缺行、形状错误、时区无法解析及恢复，与 CLI JSON 和 SQLite 相符。真实 App 从世代 0 进入 1 时清除旧窗口，并在世代 1 恢复。关闭后得到 `probe_disabled`，三份隔离 App Group 快照符合允许字段。
- **N2。** 源码树 `ce93c6a0369186ea4417618f9912621fbb3e847c` 与最终树的 87 个 macOS 路径中，唯一不同的是 `AgentDeckTests/EmbeddedHelperRunnerTests.swift`。旧签名测试 App 的 16 个观察用于相同的 280/420 pt、中英文、当前/上次读取、到期及来源时刻渲染。最终树另有四个 280 pt 的绑定状态栏 popover 观察。内嵌 helper 与 N1 签名后像字节一致，在合成 HOME 经 CLI 实际写入 64%/17% 窗口。世代 0→1 后两窗口被丢弃，菜单显示上下文变化；世代 1 恢复为 24%/7%，两次可用态均明确额度归属未确认。关闭读取时 App 隐藏额度和身份区块，CLI 返回 `probe_disabled`，供应商调用零次，夹具缓存仍存在。主控核对五项校验和、签名、App/helper 字节和四幅原生图。新增回执 SHA-256 为 `d76a27f5de978eccb6f1cbe3d7ed82d788a0a097cad6537db36d273e22a51f00`。现有 N2 标准仍要求系统 VoiceOver，依用户授权跳过需要修改本机辅助功能配置的操作，该部分开放。
- **N3。** 本机签名测试 App 与 Widget 使用同一唯一隔离 App Group。精确内嵌 helper 驱动合成 Claude 输出，App PID 48270 写入 61%/14% 两窗口，真实测试共享容器快照摘要为 `d8e17c1f3821699ebdd1eaf0d8ba1b0d5bd28d07bde0b6fb9f69dd2ac8cef782`。主控核对签名、四项回执校验和、三个二进制及物理快照。准确 App 和扩展可绑定至本机 Xcode 的 Apple WidgetKit Simulator 路径，但自动发起运行在权限预检返回 `-1744`，未启动扩展。正在准备普通 Xcode 手动启动。实际扩展读取、尺寸和到期显示仍开放，回执中的时刻来自生产者。
- **N4。** 使用原有通知授权的隔离签名 App。正向观测到一次提交、一次系统转发、成功回调、一次 SQLite ACK 和横幅，权限请求计数为零。暂停 App 后，持有两个世代 0 候选，世代 1 提交后恢复 App，旧提交、系统转发、横幅及成功 ACK 均为零。夹具进程已收尾。

N1/N4 原生回执摘要为 `446460da256a3ee2ccb81a8aaee84641d0a0ae240db89fb63308078d6cdb0608`。沿用项目既有的精确读回，按提交内容核对五个观察节点的身份和属性及十条关系的预检与写后端点。写入工具返回 mutation counters。固定 CEv1 gate 在最终树的 ContentState 为 NOT_VERIFIED，N1、N4、独立集成评审、适用 L3 和签名隔离出处共 5/7 已满足。剩余 N2 菜单及系统 VoiceOver 标准，以及 N3 App Group 消费及到期呈现。正式 PR 条件沿用本轮裁定。

#### 本机 WidgetKit 启动记录

普通 Xcode 界面打开隔离工程 `N3FixtureWidget`，通过 **Run Without Building** 启动本机 Apple 签名的 WidgetKit Simulator。选择器有两个同名 AgentDeck，直接打开精确测试 `.appex` 的按钮禁用。为区分测试条目，使用同一源码树与 App Group，复制并为测试 App、扩展更换 bundle ID 和显示名，再用同一团队签名。重签前执行文件与原签名夹具逐字节一致，嵌入 helper 保持原字节，扩展保有沙箱及测试 App Group。

实际测试 App PID 48716 写入 Claude 两窗口 61%/14%。快照摘要 `619d3b633bf53395cf1fef7f82b92a04a15137cf5aa1dcd62a3289b216beb31d`。Apple 宿主 PID 49130 在同一快照的有效期内打开选择器。系统 PlugInKit 已登记 `AgentDeck N3 Fixture`，选择器未出现该唯一条目，扩展进程未启动。三个尺寸、真实时间线及到期观察仍缺。已退出本次宿主与 Xcode 并撤销测试别名登记。N3 保持未验证，Task 7 保持 5/7。

原始观察、签名与登记回执位于受控 AgentDeck-Acceptance Handoffs 目录 `v075-e21-sai-task7-widgetkit-native-ui-19f3a581`，七项回执校验和已核对。

## Round 3 — 2026-11-22，N3 真实桌面 WidgetKit 与 N2 VoiceOver 处置

评审结论：PASS

N3 PASS；N2 的系统 VoiceOver 部分按用户授权跳过，记为未执行。

完成门禁：VERIFIED

- **N3。** 复用树 `19f3a581c21abca5b60d2e114f1eb45337a12c5b` 的签名测试 App 与 Widget（Developer ID，Team N2FZ2FNRTU），四个二进制 sha256 与原出处一致，未重建。测试 App 内嵌的精确 helper 在隔离 HOME 中驱动合成 Claude CLI，直接写入唯一测试 App Group；英文快照 `24dd1e18`（App PID 72957）、中文快照 `5231f92d`（App PID 63083），两窗口 61%/14%，邮箱已脱敏，`attribution_confirmed=false`，无完整邮箱与组织信息。真实桌面添加 small、medium、large 三个测试组件，扩展 PID 46111（英文）与 63316（中文，仅测试副本应用语言）。chronod 记录重载后三个尺寸时间线刷新成功。small 与 medium 显示 61% 及 61%/14% 并标「账号未确认」，任何时刻不显示邮箱；large 显示「当前登录」及脱敏邮箱；各尺寸无组织名，Claude 块不显示套餐。到期后 9 秒内 large 的登录行消失、额度数字保留，small 与 medium 不变，符合到期 entry 约定。测试组件、扩展进程、测试语言偏好与三条测试登记均已清理，正式 App 与真实 App Group 未接触。
- **限制。** 扩展读取为间接证据：渲染值与快照逐项一致，加 chronod 重载与时间线日志，无文件级读取跟踪。桌面操作依赖用户为本次验收临时开启的 T3 辅助功能授权。
- **N2。** 渲染、绑定状态栏及中英文观察沿用 Round 2 记录。系统 VoiceOver 观察由用户明确授权跳过，记为未执行，不作为本轮阻塞。
- **回执。** 受控 AgentDeck-Acceptance Handoffs 目录 `v075-e21-sai-task7-n3-desktop-19f3a581`，HANDOFF.md SHA-256 `1576f82ef413c6ca02ffb1a7ca7e105432b7fc9d2cd10710f7564ea1e1b14da0`，SHA256SUMS `36e8ede05c1840f931d1d3493bd3cd9a1fcc08e6755e15e91732689b0d9e87aa`，136 项主控复核全部一致。
- **补丁恢复与门禁。** 树 `19f3a581` 的三路径锁修复补丁原存于 /private/tmp，主机重启后丢失，已从 OpenCode 与 Codex 会话记录精确恢复：修复前树 `32999f846ccd6632ebfe5e3f4f8c89bbe9ea33d9`（`1f80036` 与 `929e952` 的合并解决结果）加补丁（SHA-256 `d3b89a0fdb1bf2267607fe5208b68b4f05539550d4f784bffd58f225278efb4e`）重放得到 `19f3a581c21abca5b60d2e114f1eb45337a12c5b`。CEv1 在该精确树上 7/7 满足：N3 证据记录桌面观察；N2 证据注明 VoiceOver 部分经用户豁免未执行。
