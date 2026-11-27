---
status: active
created: 2026-11-17
---

# 缺陷：全部客户端同名模型行缺少客户端区分

载体ad-bug-all-scope-duplicate-model-rows，v0.7.5，Lane A，工作区agent-deck.fix.all-scope-duplicate-model-rows。用户已明确选方案B：保留后端(client,model)拆分，菜单栏和Widget的all视图标注客户端，例如gpt-6-luna · Codex / gpt-6-luna · Claude。方案字母B不是Bug lane B。

## 现象与根因

当前基线4ac56db32e13c4e925b648c620ba76fc588a7781：App BreakdownModelRow只显示row.label；Widget Composition小/中/大消费只显示model名。同名模型跨client的两行在数值正确时外观无区分。旧v0.7.0待选说明已被2026-11-17用户B选择及v0.7.5路线委托有限替代。

## 边界

只为all客户端模型呈现加可见且可访问的client标识，不合并行，不改排序、份额、计量、价格、wire、持久化、来源/排除或套餐。单客户端选择保持现有简洁语义；long model不得挤掉client区分。共享原生唯一写者现为本修复AGY Opus，tokenmix已签名冻结并创建PR72；不得复制tokenmix改动或改其分支。允许必要的native投影/视图/直接测试；原型和正式记录由主控同步。

## 验证与交付

先以同名Codex/Claude合成夹具观察原UI标签回归失败，修后证明两行仍拆分、数值/份额/排序/ID保留，覆盖App与Widget相关family及客户端切换。复用或补保护既有Go grouping契约，Go测试用项目wrapper。原生测试仅合规隔离HOME，不触安装App或真实账号。结果/身份/独立评审/gate待完成；用户已授权通过后分别签名提交、推送origin和建PR，push/PR Actions全绿后Codex review；无merge/release/install。

## 实现

Developer 为 AGY Gemini 4 Hydrogen（请求 thought_level=high，实际 unknown）；先派的 AGY Claude Opus 因该模型族额度耗尽在改代码前失败，无残留改动。共享展示辅助 `DesktopPresentationModelV1.display(inScope:)` 仅在 scope 为 `all` 且 client 非空、非 `all` 时生成 `<model> · <Client>`，单客户端保持模型名；无 wire/Codable 字段变化。菜单栏行稳定 ID 改为 `client\0model`（重复时追加确定的 occurrence），着色仍按基础模型名；`BreakdownModelRow` 把模型名（中段截断）与客户端后缀分开渲染，后缀和数值/份额列优先保留。Widget small/medium/large 同样分开渲染并在 accessibility 中带客户端。Go 只在 `internal/usage/presentation_test.go` 补 `(client, model)` 拆分与单客户端不带 client 的保护。

Developer 报告修复前三个新增 App/Widget 测试失败（xcodebuild 退出 65：标签无客户端、OCR 无 Codex/Claude），修复后 DesktopWire/MenuBarViewModel/MenuBarChrome/WidgetPresentation 四类 156 tests 0 failures，Go 新测试与 `./internal/usage` 通过。首轮 GREEN 时 App OCR 断言因中段截断不再包含完整模型名而失败，随后放宽为前缀加尾段（容忍 OCR 把 variant 认成 varlant）并精确匹配客户端后缀。

## Review — Round 1

方法：冷独立 Claude Opus 5.5（请求 effort=high，实际 unknown），与 Gemini Developer 不同 harness，只读。核对 12 个候选 blob 一致；隔离 HOME 下四类 156 tests 0 failures，Go presentation 测试通过。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分 8/10。生产行为满足方案 B 且无越界；与已合并的 PR #72 无语义冲突，`MenuBarViewModel.swift`、`WidgetViews.swift`、`WidgetTestFixtures.swift` 预计有文本冲突。

- MR-R1-F1 Medium：Widget medium/large 的 accessibility 断言调用的 `WidgetAccessibility.modelRow` 在生产中无人调用，生产行由 `ShareMetricRow` 用自身 label 生成描述。把 medium/large 的 label 改回 `item.model` 时画面仍显示客户端后缀、测试全绿，而 VoiceOver 读不出客户端。
- MR-R1-F2 Low：新增一份客户端显示名映射，未知 client 被首字母大写（`opencode` → `Opencode`），与同面板客户端行原样显示不一致；仓库已有多份同类映射。
- MR-R1-F3 Low/说明：App OCR 断言能抓住"去掉客户端"与尾部截断吃掉后缀，分不清分开渲染与整体中段截断；单客户端反向断言偏弱，精确语义已由 ViewModel 测试覆盖。不阻塞。

prototype 影响位置经 Reviewer 核对准确，并补充 `.data-row span`、`.w-row span` 的尾部省略同样会吃掉后缀。

## 修复 — MR-R1-F1, MR-R1-F2

repair_count=1，由主控（Claude Code，Claude Opus 5.5）直接修复。F1：`ShareMetricRow` 新增可选 accessibility 描述，并以 `init(model:display:tone:)` 作为模型行唯一构造入口；可见 label、截断前缀、保留后缀与 `WidgetAccessibility.modelRow` 描述都从同一个 display 派生，测试断言的描述即生产所用描述。F2：`DesktopPresentationModelV1.clientDisplayLabel` 改为公开，未知值原样返回；`MenuBarViewModel.clientLabel` 的非 `all` 分支复用它，所以客户端行与模型后缀用同一来源。新增 `testPresentationModelDisplayShowsUnknownClientsAsReported`。F3 不改。

原型同步：新增 `prototype/src/ModelLabel.jsx`（`modelKey` 与 `ModelLabel`），Breakdown 模型卡与 Composition small/medium/large 在 all 视图显示 `· <Client>` 后缀并以 client+model 作 key；`styles.css` 让模型名先省略、后缀保持可见。CSS 不能中段截断，原型以尾部省略近似原生的中段截断。主控用主工作区已有的 esbuild 只读检查三文件语法与 `src/main.jsx` 打包，均无错误；未运行原型 dev server，未安装依赖。

主控在隔离 HOME（`/private/tmp/agentdeck-macos-xctest.*`，HOME/CFFIXED_USER_HOME/AGENTDECK_TEST_HOME/TEST_RUNNER_AGENTDECK_TEST_HOME 同指）运行 DesktopWire、MenuBarViewModel、MenuBarChrome、WidgetPresentation 四类：xcodebuild 退出 0，27+95+35=157 tests 0 failures，新测试通过。修复后候选：

| 路径 | Blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarPanelViews.swift | 78e3c59085db209e6b2431e76988a957765de6f2 |
| apps/macos/AgentDeckApp/MenuBarViewModel.swift | b0106c2e8c4b4d06bdf52244c39e1ab1ef19022d |
| apps/macos/AgentDeckAppTests/AppTestFixtures.swift | 8052f68345ac761563ad86ae36fef7b1f3cd29b6 |
| apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift | 84a8333c9b113643782990c33117078149e61688 |
| apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift | a8db7dad1f46de167f489022d0592453b83cae92 |
| apps/macos/AgentDeckShared/DesktopWire.swift | 04e6eff92fa562ba7106e9c2958dc569efe3d4c7 |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | b1f789fcd968baaf8be69a19d4cf55f5289a6411 |
| apps/macos/AgentDeckWidget/WidgetDomain.swift | c84139c4f2188032d8172fd2c93ed09f35ba34ac |
| apps/macos/AgentDeckWidget/WidgetViews.swift | 662985d01d67e8431f8abf43c6b529c1009f4aab |
| apps/macos/AgentDeckWidgetTests/WidgetPresentationTests.swift | b2885e438484091442b7a9ad47afeda55285deda |
| apps/macos/AgentDeckWidgetTests/WidgetTestFixtures.swift | 700769108b15b4c470163d4194946cd4b1772958 |
| internal/usage/presentation_test.go | f1b781eb26d1a10385ea8652d46f95170a8f1338 |
| prototype/src/ModelLabel.jsx | 87a847ec19617cdb5b1f413a09d459c50add08d9 |
| prototype/src/Popover.jsx | 3f9a2334694737bf5b69eb097eee43073904f217 |
| prototype/src/Widgets.jsx | 6de6559686db0d15aad0acb65b89561ead3ce060 |
| prototype/src/styles.css | d3457d90593036567d87f8f77b178fd5d51ca454 |

修复者与主控为同一上下文，需冷独立复评后才能裁决。

## Review — Round 2

方法：冷独立 Codex，executedModel gpt-6.1-sol、reasoning effort=high（运行时提供），与 Gemini Developer 和主控修复者均不同 harness，只读。逐项核对上表 16 个 blob 一致；HEAD、16 个文件与 go.mod/go.sum/vendor/modules.txt 的 recipe SHA-256 为 `ef92a2398c3615a48a3818d4ac576a710e6cab0800d472fc56ff3bbb37f8d380`。

评审结论：PASS

完成门禁：VERIFIED

总体评分 9/10。MR-R1-F1 CLOSED：medium/large 共用的 `modelList` 只经 `init(model:display:tone:)` 构造，可见标签、截断前缀、后缀与 accessibility 描述从同一 display 派生，`body` 实际消费该描述；原先仅把 label 改回 `item.model` 即丢失客户端朗读的路径已消除。MR-R1-F2 CLOSED：共享映射未知值原样返回，`MenuBarViewModel.clientLabel` 的 `all` 本地化分支保留，其他调用点（客户端标签、subtotal、route 摘要、切换文案）行为不变。MR-R1-F3 维持不阻塞。无新发现，无 wire/Codable、后端、存储、排序或计量变化。

原型同步可接受：all 视图加 `· <Client>`、单客户端省略，`modelKey` 以 client+NUL+model 区分；模型专用选择器优先级 (0,2,1) 高于通用 span 规则 (0,1,1)；`.w-model` 只有 Composition small 使用；尾部省略近似原生中段截断，保留后缀这一核心行为一致。未执行浏览器渲染验收。

保留局限：通用 memberwise 初始化器仍供质量/provider 行使用，"唯一入口"是当前调用路径事实而非类型约束；测试未直接读取渲染后的 VoiceOver 树。

Reviewer 实际运行：16 个 blob 核对 MATCH；`scripts/run-go-test.sh -count=1 ./internal/usage -run TestPresentation` 退出 0（5 tests）；隔离 HOME 下 xcodebuild 五个定向测试（两个 DesktopWire、MenuBarViewModel、MenuBarChrome、WidgetPresentation 各一）退出 0、0 failures；esbuild 对三文件 JSX 转换退出 0，`main.jsx` 打包退出 0（首次误用 `--node-path` 参数退出 1 后改用 `NODE_PATH`）；`git diff --check` 退出 0。主控此前的四类 157 tests 结果见上节，Reviewer 未重跑全量。工具链为 Swift 6.3、Xcode 26.4（17E192）、Go 1.27.3 darwin/amd64。

Task checkpoint：主控在上述 ContentState 记录 failure-first、labels、regression、independent-review 四项 CEv1 证据并查询 gate。随后签名提交本 issue 的代码、测试、原型与本记录，合并 origin/main（含 PR #72）并在合并结果上重跑受影响测试后推送、创建单独 PR；Actions 全绿后请求 Codex review。原 Bug 不因本地验收关闭。
