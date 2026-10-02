---
status: active
created: 2026-10-02
---

# AD 共笔图标接入

用户已经选定 AD 共笔图标，并授权独立工程交付；GitHub issue #29，
Beads `ad-shared-stroke-icon`。本任务不重新选择或生成图标。

## 输入与边界

验收包 `AgentDeck-Icon-Review-v1.zip`，Library
`libfile_18670278130c8191a708e0c61daa5d14`，SHA-256
`83e01d3bd0956fb6b31209415cd20400194efb6661c67f9f8a74005a4b944ac0`。
预览 `libfile_9ffb579569808191931344c461597c8a`；选中原图
`libfile_484da62ba9a881919f9256dd38b0f638`。三者已在消费端 Mac 下载并查看。
保留橙色、深底、AD 共笔与 D 外环两切口，不重设计。

替换现有 AppIcon 的十个 PNG、18/36px 菜单栏模板，以及运行时
512px App 与 36px 菜单栏 PNG 副本。保持资源名称与加载行为。
Xcode asset catalog 继续编译 ICNS/Assets.car；不引入 Icon Composer。
菜单栏已有 18pt 模板与异常 badge 逻辑，popover/About 共用 App PNG。
当前 prototype 的三处品牌引用使用包内 128px 透明符号；保留旧图片。
Widget 的 SF Symbols 与历史 prototype 不属于品牌替换范围。
更新当前品牌来源说明，原机器人来源作为历史完整保留。
不修改其他 UI、Go 数据行为、用户配置、安装实例或 P2/P3 缺陷。

## 验收

1. 每一目标 PNG 与指定 ZIP 内对应文件字节一致；尺寸、透明边缘、
   菜单栏黑色 RGB/alpha 和 runtime 副本一致。
2. 实际 Xcode 构建成功，bundle 图标、独立 PNG 与资源名称正确；
   检查生成 ICNS 解码，运行相关菜单栏测试。
3. 隔离 Mac App 显示检查：18pt 菜单栏、popover/About 图标；
   16px 原尺寸显示须如实记录切口变弱。静态渲染与实机截图分开记录。
   不接受新安全权限，不使用生产数据；受阻时保留未完成项。
4. 冷独立 review、当前 head GitHub Codex review、CI 与适用 CEv1
   均通过后，才执行已授权的 merge commit 和 slot 释放。

## 风险与不保证事项

16px 两切口可能减弱，不能通过放大图代替原尺寸验收。
本任务没有完成商标审查；有限相似初筛中 Scalebranding 414379
图片尚未看清。来源记录不得声称唯一性或法律清算完成。

## 用户明确延后的验收项 — 2026-10-02

用户原话：“菜单啊如果无法验收那就先忽略，这个可以等后续我人肉验收以后再来修复。”
来源：`Sentinel_5d35b76b9a5c8191bd59d39bb43f871e`，父任务传达。
因此上述第 3 项的真实系统菜单栏人工视觉验收从本轮必需门禁移出，
留待用户后续人工检查；不标为通过，不申请新的辅助功能或其他安全权限。
16px 本轮证据只要求明确尺寸的静态/离屏像素检查，不冒称 1x 实机验收。
隔离 App 窗口截图证明应用内品牌已显示；它不代替菜单栏/Dock/Finder 验收。
本轮门禁仍要求精确资产映射、编译/模板/相关回归和独立代码评审。
GitHub 当前 head Review、CI、集成检查和适用 CEv1 仍是交付条件。
