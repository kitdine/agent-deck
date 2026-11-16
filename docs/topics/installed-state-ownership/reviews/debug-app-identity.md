---
status: active
topic: installed-state-ownership
subject: debug-app-identity
---

# debug-app-identity Review

## Round 1 — 2026-11-10

## 📋 Task 7 独立实现评审

📊 总体评分：9/10

✅ Verdict: PASS

- Reviewer: Codex，独立评审会话；未参与本候选实现，未修改产品代码、测试或配置。
- Method: 对照已批准的 Task 7/G5/架构身份表，检查精确差异、Xcode 对象继承、身份消费路径及原生 harness；使用本工作区 CodeGraph 定位消费者，源代码核对配置驱动行为；复用 CEv1 的精确状态证据。
- Scope: `project.pbxproj` 的 App/Widget Debug 身份、`check-widget-sandbox.sh` 的身份断言、`MenuBarViewModelTests.swift` 的失败诊断，以及相关 Info.plist、entitlements、共享 xcconfig、构建/测试入口。Task 6 的状态路径和会话实现、Task 8 的真实双 App A5 组合验收不在本轮范围。
- Reviewed state: HEAD `62e2f239002899fd5d0ba4bf26ea9280c66ba827`；开发候选 `installed-state-ownership:debug-app-identity:dev-20261110-xdSv0auq`；SHA-256 `95177387c2b7bc83d6edf35e02d49eca6f3c490400bd5f2e4eebe88a208c9dbd`。
- 指纹配方：`head=<HEAD>\n`，随后按路径排序拼接 `<path>=<git-blob>\n`。范围为 Makefile、App/Widget entitlements、pbxproj、App/Widget Info.plist、MenuBarViewModelTests.swift、AgentDeck.xcconfig、architecture.md、tasks.md、build-macos-app.sh、check-macos-widget-intents.sh、check-widget-sandbox.sh、test-macos-app.sh。全部 15 个 blob 与开发 ContentState 一致。
- Completion gate: VERIFIED

### 🔴 严重问题 — 必须修复

无。没有发现本候选引入的可操作缺陷。

### 🟡 改进建议 — 建议处理

无。本轮不把未执行的操作员注册证明伪装成代码缺陷，也不把尚未定位因果的历史焦点间歇失败认定为身份变更造成的回归。

### 🟢 优点

- `project.pbxproj:719` 与 `:918` 分别覆盖 App/Widget 的 Debug App Group；bundle ID 为 `com.kitdine.agentdeck.debug` 与 `.debug.widget`。直接解析 HEAD/候选工程对象，只发现两个 Debug 对象中这两个设置有差异；所有 Release/Perf 对象完全相同，共享配置和模板未变。
- AppGroupSnapshotStore 与 WidgetSnapshotReader 从各自 `Bundle.main` 的 `AgentDeckAppGroupIdentifier` 读取标识，两个 plist 和 entitlements 继续消费同一个 build setting。测试宿主按产品路径定位，未绑定旧 bundle ID。系统通知通过当前 App 的 UNUserNotificationCenter 使用宿主身份，无需额外硬编码替换。
- `check-widget-sandbox.sh:32` 新增目标和配置级身份断言，覆盖 Debug、Release、Perf；原有 entitlements、只读 Widget、私有路径检查完整保留。静态脚本不声称能证明注册或运行时容器访问。
- `MenuBarViewModelTests.swift:184` 保留原激活调用，仅捕获返回值并补充失败消息；断言、两秒前台等待和渲染等待均未放宽，也未加入重试或跳过。

### 📝 总结

实现符合 Task 7 的独立构建身份边界，评审 PASS；没有开放的本轮 finding。该结论不表示整个任务证据门禁通过，也不代表 Task 8 的真实双 App 验收完成。

Evidence:

1. 本轮通过 `git hash-object` 与既定 SHA-256 配方复核全部候选输入，与开发状态逐项一致；通过 `plutil -convert json` 解析 HEAD 和候选 pbxproj，逐对象断言变化仅限两个 Debug 对象的 `PRODUCT_BUNDLE_IDENTIFIER` / `AGENTDECK_APP_GROUP`，结果通过。
2. 复用 CEv1 `dev-20261110-xdSv0auq:debug-identities`、`:release-perf-preserved`、`:widget-sandbox` 的目标态证据及其 `dev-20261110-4JwLmDqF` 来源：Debug x86_64、Release universal 构建通过；Debug/Release 实际 Developer ID 签名 strict/deep 验证通过，团队 N2FZ2FNRTU，实际 entitlements 和 plist 与各自 group 一致；`bash -n`、`make check-widget-sandbox` 通过，两个独立临时身份回退负例均被拒绝。复用是因为产品、构建输入和工具链未变，不是以本轮读代码代替构建。
3. 复用最终候选 `:xctest-host`：原始 `make test-macos-app` exit 0，Shared 122、App 167、Widget 51，合计 340 项、0 failure、1 项既有 opt-in skip；日期用例通过，23.396 秒，无 helper 泄漏报告。原有临时 HOME/CFFIXED_USER_HOME/测试域和无签名 Debug TEST_HOST harness 未变，因此不因进入评审重复整个套件。
4. 保留先前两次全量日期窗口焦点失败，以及三次独立定向通过、Chrome+日期组合 37 项通过的历史事实。它们不能证明间歇焦点问题已根治；最终全量通过只支持本次候选回归结果。没有足够证据将历史竞争归因于这次身份修改，本轮不新增推测性 finding。
5. 首次精确门禁查询返回 NOT_VERIFIED：4/6 有效，缺 independent-review 与 signed-registration；无 invalidated evidence 或 unresolved candidate impacts。评审后补充独立评审证据，并将状态摘要的纯文档变化以显式目标态 roll-up 复用本地证据；最终仅 signed-registration 未满足。
6. 记录结构、局部链接、空白检查在本轮收口时验证。未执行安装、启动双 App、账号查询或 Git 交付。

注册限制：Task 7 要求操作员确认 `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug` 注册。开发交接明确记录用户“那就不查呗”，本轮遵守该查询边界；该决定没有被解释为注册已完成或验收标准已豁免。实际本地签名成功也不能替代账号端证明。

评审后 ContentState：`installed-state-ownership:debug-app-identity:review-r1-20261110-7YHlNPY3`。与被审候选相比仅 tasks.md 的 Review/交接摘要改变；产品、测试、依赖、配置、构建脚本保持一致。最终门禁缺口仅为 `installed-state-ownership:debug-app-identity:signed-registration`，Beads 保持 `in_review`，所有权释放供后续证据交接；任务 6、8、9 仍未完成，不跨越 Topic 完成边界。

Task checkpoint：`ad-installed-state-ownership-debug-app-identity`；上述评审后 ContentState；NOT_VERIFIED。

提交建议：门禁 VERIFIED 后，单独提交 Task 7 的工程身份、sandbox 校验、测试诊断及对应 tasks/review 记录；前提为补齐 Debug App Group 注册证明，并取得明确提交授权。

推送建议：门禁 VERIFIED 后，核验签名提交并在明确授权下推送 `origin/feature/installed-state-ownership`；当前前提仍为注册证明，尚未核验远端交付状态。

下一步前提：由操作员提供 Debug App Group 注册证明，随后同步该精确候选的 signed-registration 证据并重查任务门禁；无需重复已完成的代码评审。

## 平台前提更正与证据同步 — 2026-11-10

本节更正 Round 1 的注册前提和交付阻塞判断；上文保留原候选、原门禁查询及当时建议作为历史，不代表当前仍需注册。不是新的产品评审轮次，原独立实现评审 PASS 保留。

### 更正依据与授权

用户质疑注册要求并明确“首先不允许你注册”，随后要求继续核实及更正。Apple 官方 [Accessing app group containers in your existing macOS app](https://developer.apple.com/documentation/xcode/accessing-app-group-containers#Use-app-groups-that-you-dont-provision) 的 “Use app groups that you don’t provision” 小节明确：macOS 的 `<Developer team identifier>.<group name>` 格式不需要 provisioning profile，也不需要门户注册；系统检查前缀与签名团队匹配。2026-11-10 读取官方 Markdown 正文核实。该格式不支持其他 Apple 平台或 Keychain Access Groups；本任务是 macOS App/Widget 容器身份，不涉及这些用途。

当前值 `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug` 以 Team ID 开头，属于上述格式；不能因名称中间含 `group` 就按以 `group.` 开头的 provisioned 标识处理。因此 architecture.md、Task 7 和旧 `signed-registration` 准则中的门户注册要求是错误平台前提，撤销它不是豁免有效的安全验收。未注册任何资源，也未更改账号、签名、配置或产品。

### 当前证据与适用性

- 逻辑任务仍为 `installed-state-ownership:debug-app-identity` / Beads `ad-installed-state-ownership-debug-app-identity`。
- 为保留 CEv1 的追加式历史，旧 WorkUnit、准则和 NOT_VERIFIED 观察不改写；当前验收版本为 `installed-state-ownership:debug-app-identity:acceptance-v2`。该版本显式指向原逻辑任务和旧验收版本，不是新的实施任务，不改变 Topic 任务分解。
- 六项必需准则：Debug 身份、Release/Perf 不变、sandbox、XCTest 宿主、独立实现评审、实际签名及 macOS Team ID 前缀规则。最后一项取代错误的门户注册要求，保留 Debug/Release 实际签名和 entitlements 核验。
- 新 ContentState：`installed-state-ownership:debug-app-identity:correction-20261110`。继续使用原有 HEAD 加 15 个范围 blob 的配方。相对 Round 1 收口状态，只有 architecture.md/tasks.md 的平台前提与状态文案变化；配置、产品、测试和构建输入完全相同。
- 五项既有通过证据显式 roll-up 到新准则；实际签名证据复用原 Debug/Release 取证：团队 N2FZ2FNRTU、strict/deep 验证通过、签名 entitlements 与各自产物 Info.plist 一致。新增官方平台规则适用性核验，不制造注册证明、不重复原生套件，也不扩张到 Task 8 的容器运行/A5 组合验收。
- 这次事实勘误不改变身份选择、实现行为或任务边界；不重新签发 architecture/tasks 的整篇独立评审。先前评审对其他内容的结论仍绑定原状态，本节仅对已核实的平台前提更正负责。
- 当前验收版本最终精确状态门禁 VERIFIED 6/6；missing、invalidated 和 unresolved 列表均为空。记录结构、链接、空白和差异检查通过。Beads 进入 awaiting_commit，尚未交付或关闭。

### 当前交付检查点

Task checkpoint：`ad-installed-state-ownership-debug-app-identity`；ContentState `installed-state-ownership:debug-app-identity:correction-20261110`；验收 WorkUnit `installed-state-ownership:debug-app-identity:acceptance-v2`；VERIFIED。

提交建议：取得明确授权后，提交 Task 7 的实现、诊断、校验脚本、architecture/tasks 事实勘误和本评审记录；保留相关贡献者署名并核验 SSH 签名。

推送建议：取得独立推送授权后，核验提交范围、正文、trailer 和签名，再推送 `origin/feature/installed-state-ownership`；未执行推送。

后续任务为 `app-state-location`，仍需用户的开发指令；无需注册、登录开发者账号或重复本次实现评审。
