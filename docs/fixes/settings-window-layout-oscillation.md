---
status: active
created: 2026-11-27
---

# 缺陷：1x 显示器上设置窗口布局无限振荡

载体：`ad-6d5i`；v0.7.5；Lane A（用户 2026-11-27 确认）。
工作区：`agent-deck.fix.settings-window-layout-oscillation`；分支 `fix/settings-window-layout-oscillation`，
路径 `.worktrees/fix-settings-window-layout-oscillation`，基线 `main` `33b0eb16edd710cf0c32805f0836e635309f63f8`。

## 现象

在 1x 非 Retina 显示器上打开设置窗口后，App 主线程持续占用约 100% CPU；关闭设置窗口后仍不停止，直到退出 App。
2x 显示器上不出现。原生测试全量运行时，同一宿主进程里排在 Settings 窗口套件之后的 async 用例变慢 150–1000 倍，
带固定时限的用例（如 `SwitchFlowTests/testConfirmationUsesExactlyTheResolvedOption`）失败。

## 根因（2026-11-27 诊断）

方法：只读，在 `git archive 33b0eb16` 副本中二分、1x/2x 对照、`sample` 采样和试改。

- `SettingsWindowView.swift` 的 `SettingsPaneView` 用 `ViewThatFits(in: .vertical) { page; ScrollView(.vertical) { page } }`，
  而 `MenuBarItemController.swift` 按 `sizeThatFits.height.rounded(.up)` 把窗口设为内容高度（额度页 465 pt，通用页 308 pt）。
  1x 显示器上的像素对齐让 `page` 比窗口高 1 pt，`ViewThatFits` 换成 `ScrollView`；在 `ScrollView` 里 `page` 又测得不超过窗口，
  于是换回来，形成 465 → 466 → 464.5 → 465 的四步循环。每一圈都重建 `QuotaClientCheckbox`（`NSViewRepresentable`）
  并触发 AppKit 布局，`sample` 中 2783/2786 个样本在这条链上。
- `SettingsWindow` 设了 `isReleasedWhenClosed = false`，关闭时不摘除 `contentViewController`；App 全程持有 `settingsController`，
  所以关闭窗口后循环继续。测试中 `SettingsWindowPaneTests/testTheSheetHoldsThePage` 留下的 sheet 窗口让 `SettingsWindow`
  永不释放，后续用例的主 RunLoop 被占满。
- 已排除：持续的 `CVDisplayLink` 或动画、非默认 RunLoop 模式、泄漏的 Timer 或观察者、App Nap。

## 修复边界

1. 去掉 `ViewThatFits`。`page` 使用纵向固有高度；按内容高度与窗口上限决定是否包一层 `ScrollView`，切换带滞回，
   1 pt 级的像素对齐差异不能来回翻转。窗口尺寸和视觉结果与基线一致，2x 显示器行为不变。
2. 窗口不可见时不响应 `paneContentChanged`。`SettingsWindowController` 在关闭和释放时拆除内容：结束 sheet、关闭窗口、
   摘除 `contentViewController` 和 tab、释放闭包。重新打开设置窗口时行为与基线一致。
3. 测试改用上述拆除路径清理窗口。
4. 不改设置项、布局视觉、公共协议或 Go 代码。

## 验证

见 Review — Round 2 与文末主控核对。固定 `backingScaleFactor = 1` 的回归用例在基线 `33b0eb16` 上断言失败、修复后通过；完整隔离 `scripts/test-macos-app.sh`（不排除任何测试类）退出 0。

## Review — Round 1

评审结论：FAIL

完成门禁：NOT_VERIFIED

发现：0 P1 / 0 P2 / 2 P3。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Claude/Opus 不同 harness；只读，在临时副本中运行完整隔离全量（不排除任何测试类）、基线回归对照与三项变异。被评候选 `git diff --binary --full-index 33b0eb16 -- apps/macos`，SHA-256 `d26c54c80bba6c76d407b64089df7bba7508244fc12d923901d2723683a5c78f`，6 个文件。

- 根因已消除：固定 `backingScaleFactor = 1` 的回归用例在基线上断言失败（通用页多报 536 次、额度页多报 688 次并误入滚动视图），修复后通过；完整全量退出 0，`SwitchFlowTests` 0.908 s，`TelemetrySettingsControllerTests` 0.496 s。变异"去掉滞回""拆除不摘 contentViewController"被检出。
- 视觉差异（可见高度约 600 pt 以下的屏幕上窗口与滚动区域等高、不再上下留白）可接受：常规屏幕与基线一致，短屏表现更符合原型 `prototype/src/settingsTabs.css`。
- **SWO-R1-F1 / P3**：`MenuBarItemController.swift` `tearDown()` 以 `window.isVisible` 判断是否需要 `close()`，窗口已隐藏（`orderOut` 或 Cmd-H）时拆除或析构会跳过 `window.close()` 与 `sessionOrigin?.windowClosed()`，残留 `isManaging == true` 和查找草稿。
- **SWO-R1-F2 / P3**：Cmd-H 隐藏后经 `NSWindow.didChangeOcclusionStateNotification` 重新显示时补做 resize 的路径没有自动化覆盖；删除整个 occlusion 观察者注册的变异下全部测试仍通过。

## Review — Round 2

评审结论：PASS

完成门禁：VERIFIED

发现：0 P1 / 0 P2 / 0 P3。CEv1 WorkUnit `fix:settings-window-layout-oscillation` 在候选 `2dac3b94…e35f` 上记录原生验证、独立评审与范围三项证据，门禁查询三项均为 pass。

独立方法：冷上下文 AGY/gemini-4-neon，thought_level=high，与作者 Claude/Opus 不同 harness；只读，在临时副本中运行完整隔离全量（不排除任何测试类）、基线回归对照与两项变异。被评候选 `git diff --binary --full-index 33b0eb16 -- apps/macos`，SHA-256 `2dac3b94fab3469c1abc035f9b4c334efe27d4fb80a03e0e61db73a24702e35f`。

- SWO-R1-F1 关闭：`tearDown()` 拆为公开入口与 `tearDown(afterClose:)`，只有 `onClose` 传 `true`；先置空 `self.window` 再 `close()`，阻断回调重入；用户正常关闭只执行一次 `windowClosed()`。新用例覆盖隐藏窗口的显式拆除与控制器析构；把判断改回 `window.isVisible` 的变异被 4 处断言检出。
- SWO-R1-F2 关闭：新用例不经 `show()`，以 `orderFront` 加 occlusion 通知驱动生产代码中真实注册的观察者；删除观察者注册的变异被 2 处断言检出。
- 完整隔离全量退出 0：Shared 200、App 283（3 跳过）、Widget 75、重启验收两阶段通过；`SwitchFlowTests` 0.849 s，`TelemetrySettingsControllerTests` 0.372 s。基线 `33b0eb16` 上 1x 回归用例断言失败（通用页多报 584 次、额度页 672 次并误入滚动视图）。
- R1 关于根因消除、视觉差异可接受、拆除路径安全的结论保持有效。

## 验证（主控核对）

主控核对补丁哈希与工作树 `git diff --binary --full-index 33b0eb16 -- apps/macos` 一致（`2dac3b94…e35f`）。未在真实 1x 显示器上手工验收。
