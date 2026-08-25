---
status: active
topic: desktop-app
subject: menubar-experience acceptance execution
---

# `menubar-experience` 人工验收与安全执行方案

本文只负责**如何安全执行和留证**，不重新定义验收要求。权威要求仍在：

- [`ux/menubar.md` Manual checklist](../ux/menubar.md#manual-checklist)
- [`ux/settings.md` Manual checklist](../ux/settings.md#manual-checklist)
- [`tasks.md` task 3](../tasks.md#3-menubar-experience)
- [`reviews/menubar-experience.md`](../reviews/menubar-experience.md) 记录实际结果

## 安全原则

1. 默认不为验收修改开发者个人电脑的系统级辅助功能、字体、显示、登录项、TCC、
   VoiceOver 或 provider 配置。
2. 能通过 SwiftUI environment、XCTest、XCUITest 或 synthetic transport 注入的状态，
   必须在 CI/测试进程内注入，不能要求用户切换系统设置。
3. 会修改系统状态、真实 AgentDeck/provider 状态或可能残留配置的用例，只能在可重置的
   临时 Mac、虚拟机或专用 self-hosted runner 上执行。
4. 个人电脑上的人工验收只保留无法由通用 runner 复现、且本身无持久副作用的显示拓扑
   与菜单栏手势检查。
5. 不允许对真实 provider 执行 switch 验收；switch flow 使用 synthetic candidate、
   stub transport 和隔离状态根。
6. 未取得证据的 required item 不是 PASS。只有用户显式接受残余风险后，才能以 waiver
   关闭，并同时写入 review record 与 release notes。

## 当前自动化基线

- `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer bash
  scripts/test-macos-app.sh` 已覆盖 App/Shared XCTest。
- 当前 GitHub Actions 仅使用 `macos-15`，没有专用 desktop UI acceptance workflow。
- 后续 workflow 推荐名：`.github/workflows/desktop-ui-acceptance.yml`，使用
  `workflow_dispatch`，绑定 exact commit SHA。
- 如果 GitHub-hosted `macos-26` runner 可用，先使用 hosted runner；否则使用标签
  `[self-hosted, macOS, agentdeck-acceptance]` 的可重置专用 Mac。
- 每次 CI 必须上传：`.xcresult`、截图矩阵、AX tree/焦点顺序、环境清单和测试摘要。

## 验收路由矩阵

| 验收项 | 默认执行位置 | 安全实现 | 需要的证据 | 个人电脑 |
| --- | --- | --- | --- | --- |
| View model、filter、qualifier、switch 状态 | GitHub Actions | XCTest + synthetic fixtures/transport | `.xcresult`、test count | 不需要 |
| Reduce Motion | GitHub Actions | 向 view/harness 注入 `accessibilityReduceMotion = true` | normal/reduced-motion 截图与断言 | **禁止改系统设置** |
| Increase Contrast | GitHub Actions | 注入 `accessibilityContrast = .increased` | normal/increased 截图差异、文字/状态断言 | **禁止改系统设置** |
| Light/Dark | GitHub Actions | 注入 `colorScheme`，分别渲染 | 两套截图，无固定 hex/不可读状态 | 不需要切系统外观 |
| 最大 Dynamic Type | GitHub Actions | 注入最大 accessibility Dynamic Type | 280pt 与 420pt 截图，无裁切/截断 | **禁止改系统字体** |
| `en` / `zh-Hans` | GitHub Actions | launch argument 或 bundle locale 注入 | 每个 surface 的双语言截图 | 不改系统语言 |
| 280pt narrow layout | GitHub Actions | `AGENTDECK_TEST_WIDTH=280` acceptance harness | 全 surface 截图、滚动边界 | 不需要 |
| 内容滚动与固定 footer | GitHub Actions | XCUITest/hosted app，构造超高内容 | header/filter 可达，footer 不被内容推出 | 不需要 |
| Accessibility label/value/hint | GitHub Actions | XCTest/XCUITest 读取 AX tree | AX tree JSON 与断言 | 不需要 VoiceOver |
| 键盘焦点顺序 | GitHub Actions | XCUITest 发送 Tab/Shift-Tab/Escape | 每步 focused element 与可见 focus ring | 不开 Full Keyboard Access |
| Provider switch flow | GitHub Actions | stub transport + synthetic state | confirmation、single-flight、failure/indeterminate 截图/断言 | **禁止真实 switch** |
| Login-item success/refusal/approval | 临时 Mac | 专用 bundle/TCC 状态；运行后重置 VM | 三种真实状态截图和系统状态 | **禁止修改个人登录项** |
| VoiceOver 实际朗读顺序 | 临时 Mac + 人工 | 预授权 Accessibility，启动 VoiceOver，运行后恢复快照 | 逐项朗读记录或录屏 | **禁止在个人机执行** |
| 多显示器 popover 定位 | 专用多屏 Mac；可选个人机 | 只打开/关闭 popover，不改配置 | 每块屏幕的截图与 visible-frame 数据 | 可选、无副作用 |
| 状态栏 glyph | GitHub Actions | `MenuBarChromeTests` 以 2× 渲染 production glyph | `.xcresult` 中 normal/badged PNG 附件与画布占用断言 | 不需要 |
| 状态栏左键/右键/双击 | 专用 Mac；可选个人机 | 不触发 provider switch/login item | popover/menu 截图与动作结果 | 可选、无副作用 |
| Settings/About/Quit | 专用 Mac；可选个人机 | 不修改 login item；Quit 后可重启 | 窗口/菜单/metadata 截图 | 可选、无副作用 |

## GitHub Actions / 临时 Mac 方案

### 1. 无系统副作用的 hosted CI

在 `desktop-ui-acceptance.yml` 中执行：

1. checkout exact SHA；安装项目要求的 Xcode/Go 环境；
2. 运行 `scripts/test-macos-app.sh`；
3. 用 dedicated acceptance harness 渲染以下矩阵：
   - width：`420`、`280`；
   - locale：`en`、`zh-Hans`；
   - color scheme：Light、Dark；
   - contrast：normal、increased；
   - motion：normal、reduced；
   - Dynamic Type：default、maximum accessibility；
4. 对截图执行尺寸、裁切、空白边界和 footer/header 可见性断言；
5. 运行 AX tree 和键盘焦点顺序测试；
6. 上传 `.xcresult`、截图和 machine-readable acceptance summary。

这些状态必须通过 view environment 或 launch arguments 注入，禁止运行会持久修改 runner
系统设置的 `defaults write`、System Settings automation 或全局辅助功能切换。

### 2. 可重置 self-hosted Mac

以下用例不能可靠交给普通 hosted runner：真实 VoiceOver speech、login-item/TCC、
多显示器菜单栏锚定。使用专用机器时：

1. 从干净 APFS snapshot/VM image 启动；
2. 安装 exact-SHA app；
3. 执行 VoiceOver/login item/多屏清单并录屏；
4. 导出结果；
5. 恢复 snapshot，不能把设置残留给下一次运行。

## 个人电脑最小清单

以下项目可由用户选择执行；它们不应修改持久系统配置：

- [ ] 正常/异常状态下，菜单栏 glyph 清晰可辨，文字模式与 icon-only 模式正确。
- [ ] 在每块已连接屏幕打开 popover；header、filters、hero、panel switcher 和 footer
      都在当前屏幕可见范围内，超高内容只在内部滚动。
- [ ] 左键切换 popover；右键和双击打开 item menu。
- [ ] Settings、About、Quit 路径可达；Escape 关闭 Settings。
- [ ] 当前默认字体/外观下无明显裁切、重叠、空壳窗口或不可读状态。

个人电脑上不要执行：

- VoiceOver、最大字体、Increase Contrast、Reduce Motion 的系统级切换；
- login-item 注册/拒绝/approval；
- 真实 provider switch；
- 修改系统语言、显示缩放、TCC 或 Accessibility 权限；
- 任何需要 `sudo`、重启 Finder/Dock/loginwindow 或注销用户的验收步骤。

## 人工结果模板

将结果追加到 `reviews/menubar-experience.md`，不要在本文件维护第二份状态：

```text
Environment: <commit, app hash, macOS, Xcode, displays>
Item: <authoritative checklist item>
Result: PASS | FAIL | NOT_RUN
Evidence: <screenshot/video/xcresult/AX-tree digest>
Side effects: none | <isolated runner reset confirmation>
Notes: <observed behavior>
```

## 无法覆盖时的 waiver 与 release note

若 CI 和可重置 Mac 都无法提供某项 required evidence：

1. Review 保持 `in_review`，不得静默标记 PASS。
2. 用户必须显式接受该项残余风险；review record 记录接受人、范围和理由。
3. Release notes 必须包含下面的明确条目：

```markdown
## Known verification gaps

- Not verified: <exact behavior and environment>.
  Risk: <user-visible or operational consequence>.
  Reason: <why hosted CI and isolated Mac could not execute it>.
  Mitigation: <tests or controls that do exist>.
```

4. 不得使用“accessibility tested”“fully validated”或同等表述概括包含 waiver 的版本。
5. 后续取得证据时，新增 review round 和 release-note correction，不改写历史记录。
