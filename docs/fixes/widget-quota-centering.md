---
status: active
created: 2026-10-07
---

# 缺陷：额度 Widget 正文未在各自范围内垂直居中

## 现象

用户提供的 v0.6.5-rc.1 原生截图中，medium 和 large 的 Codex 单窗口内容紧贴标题，正文下方留下大量空白。用户确认的 Lane A 边界为：在各自范围内垂直居中；一个客户端使用完整正文，两个客户端上下等分，各自在半区内居中。

Beads：`ad-bug-widget-quota-centering`。工作区 `agent-deck.fix.widget-quota-centering`，分支 `fix/widget-quota-centering`，基点为实时核对的 `origin/release/v0.6.x`（`8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`）。CEv1 WorkUnit 为 `fix:widget-quota-centering`。

## 根因

[`QuotaWidgetView`](../../apps/macos/AgentDeckWidget/WidgetViews.swift) 只在双客户端 large 分支中创建居中弹性槽。single 分支直接返回客户端块，外层 `WidgetFrame` 的 `.topLeading` 把它贴在正文顶部。

安装版本提交 `5e6d162366f1b82f3056de594c707c33d4a1aee2`、main、远端 main 和修复基点的 `WidgetViews.swift` blob 均为 `e1e55f37fee1976cd958bd1c4c42557ac55e12ac`，排除了本地代码与安装源码不同这一解释。既有测试虽然在名称中提到单端，却只检查单端布局枚举；实际 midpoint 断言仅覆盖双端。

## 修复边界

仅修改额度 Widget 的单槽垂直布局并扩展既有几何 preference 观测；标题、页脚、窗口数据和客户端筛选不变。真实 SwiftUI 渲染断言覆盖 Codex/Claude、一窗/多窗、small/medium/large，保留双端等高半区居中的既有验证。

长扫描性能问题另由 `ad-bug-menubar-long-scan-timeout` 延期跟踪，已纳入 [`roadmap.md`](../roadmap.md) 的下一版本规划候选；本次不修改扫描、超时或后台工作状态。

## 验证

验证等级 L1；原生测试必须使用临时 HOME/state。完整日志和 xcresult 附件保留于 `/private/tmp/agentdeck-quota-centering/`。

验证环境：Xcode 26.4（17E192），Debug、签名关闭、真实 SwiftUI `NSHostingView` 渲染。使用临时 `HOME`、`CFFIXED_USER_HOME`、`AGENTDECK_TEST_HOME` 和测试 runner 前缀变量；测试结束未遗留本工作区新建的 helper。没有启动或替换 `/Applications/AgentDeck.app`。

- RED：先在单端分支仅增加几何观测并保持 `.topLeading`，通过 `run-widget-tests.sh red AgentDeckWidgetTests/WidgetPresentationTests/testSingleClientQuotaWidgetsCenterContentWithinTheirWholeBody` 执行回归，exit 65。12 个组合均仅在 `content.midY == slot.midY` 断言失败。例如 Codex 一窗 medium 的 midpoint 为 26.5/59，large 为 26.5/158.5。未出现编译、fixture 或环境错误。
- GREEN：单端槽改为 `.center`，通过 `run-widget-tests.sh green AgentDeckWidgetTests/WidgetPresentationTests` 执行相关完整呈现 suite，exit 0；24 tests、0 failures，包含上述 12 个组合和既有双端等高槽 midpoint 断言。
- 构建：仓库 `scripts/build-macos-app.sh` 及 GREEN 的 Xcode 原生目标编译均成功。没有 Go 产品变更，不将 Go 全量测试作为此 L1 修复的验证范围。
- 视觉核对：已导出并查看修复前后的 Codex 一窗 large 与修复后的 medium 原生 PNG；标题和页脚位置保持，正文移至完整正文区中央。xcresult 中保留全部渲染附件。这是隔离原生 View 渲染证据，尚非安装版 WidgetKit 验收。

验证绑定：HEAD `8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`；`WidgetViews.swift` blob `c41771781424fa2f2741b362a54ad7446a0c8873`；`WidgetPresentationTests.swift` blob `a9242b824152cd300628abc305fb7d52e17df74b`。RED log SHA-256 `5b2c2e94e08f5e62ab191f9bf8362f79281951b64fb84f28382666dbc643dbf5`；GREEN log `d3b106e3d5711e1acea12093f64a05fe0e1818d6ea189bfe6ca4497e682ac124`。测试 harness 位于上述证据目录，SHA-256 `56d2774e1f627934d065e4a79f977796f5fef90c5d13ea46a8953e57fa2708a1`，沿用仓库测试脚本的 HOME/state 和 helper 清理边界。

实施及 Review 交接时 CEv1 门禁为 **BLOCKED**：当时没有可调用的 CEv1/Neo4j MCP 工具，未执行门禁查询，也没有可证实完整同步的已配置镜像。没有绕过 MCP 访问后台或创建空本地证据库。该历史状态由下述 2026-10-08 门禁恢复结果承接，不改写为当时已完成。

实施交接时本记录保持 active，独立评审、CEv1 门禁和交付均未完成。下述用户授权的独立评审更新 Review 状态；CEv1 与交付边界继续保留，没有提交、推送、PR、合并或安装。

## Review — Round 1

## 📋 额度 Widget 各自范围居中修复评审 — 2026-10-08

📊 综合评分：10/10（仅本轮代码、回归保护与规划边界）

✅ 结论：PASS

Checklist: 54/54 complete；Incomplete: None。

- **Reviewer：** `/root/quota_centering_cold_review`，冷上下文独立角色；主会话核对直接证据并负责本记录和裁决。
- **Method：** 用户明确授权的一次初始独立评审，`fork_turns=none`；合并 UI/layout 与 tests/oracles 问题为一个 lens，默认会话模型层级，无覆写或嵌套委派。主会话完成一次最终 closure；没有第二轮或复跑未变产品测试。
- **Scope：** `WidgetViews.swift` 单槽居中、`WidgetPresentationTests.swift` 真实几何回归、修复载体及 `roadmap.md` 中用户授权的延期规划边界。支持路径为既有 model、`WidgetFrame`、quota view、renderer 和 preference。排除性能实现、数据/账号操作、安装、发布、集成和 CEv1 后台操作。
- **Reviewed state：** dirty candidate；base/HEAD 均为 `8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`。冻结 manifest SHA-256 `b3d0e8918b7d55cda6b815a88d171b7fde80369b13ea0693f8c8a83ae09f27e1`；源码 blob `c41771781424fa2f2741b362a54ad7446a0c8873`，测试 blob `a9242b824152cd300628abc305fb7d52e17df74b`，评审前载体 blob `4cf9ec942ce2b87b1d9588d851611ef9e98021bb`，roadmap blob `41822be0f3da6e5d5ab4b872a2267f0ce218aa09`。独立角色和主会话分别核对一致。本轮只追加评审记录，未修改评审过的产品、测试或规划内容。
- **Completion gate：** VERIFIED。2026-10-08 session reload 后 MCP 只读 Cypher 实际成功，`fix:widget-quota-centering` 注册后的首次证据门禁已通过 7/7；门禁详情及最终文档同步后的精确状态见下述恢复记录。Review 当时的 BLOCKED 是历史传输能力缺失；代码 Review PASS 与证据门禁仍分别判断。

### 🔴 严重问题 — 必须修复

无。没有成立的 `WQC-R1-Fn` finding。

### 🟡 改进建议 — 推荐

无。没有将风格偏好、安装验收空缺或 CEv1 能力缺失包装成产品缺陷。

### 🟢 优点

四行产品差异在 quota 单槽边界复用 SwiftUI frame 和既有观测机制，保持共享 `WidgetFrame`、双端槽和数据筛选不变。新增回归观察真实布局，RED 提供了原缺陷的 12 个反证，覆盖由单纯布局枚举遗漏的行为。

### 📝 总结

验收映射及主会话核对结果如下。独立报告为输入；结论以实际差异、哈希、原始 xcresult、日志和渲染证据为依据。

| 验收 | 实现/直接证据 | 结果 |
| --- | --- | --- |
| 单端占完整正文并垂直居中 | `QuotaWidgetView` 单端内容外的 `.frame(maxHeight: .infinity, alignment: .center)`；实际 content/slot midpoint 及 medium/large PNG 一致 | PASS |
| 双端上下等高、各自在半区居中 | 既有双端分支未改；两槽等高、两处 midpoint 断言及双端 PNG 一致 | PASS |
| 回归能检测原缺陷 | RED 为 12 个 midpoint 失败；GREEN 新回归通过；未发现环境或 fixture 失败 | PASS |
| oracle 来自真实 View | `NSHostingView` layout/display 后捕获 production `content.<client>`/`slot.<client>` preference，不以预期公式回灌坐标 | PASS |
| 保持标题、页脚、窗口、选择及 attribution | 相关数据/选择与可访问性路径未改；配置渲染和既有相关回归通过，Claude small 保留归属提示 | PASS |
| L1 相关验证 | 构建成功；主会话直接读取原始 GREEN xcresult summary：24 passed、0 failed、0 skipped | PASS |

已排除外层 `.topLeading` 覆盖居中、单端仍只占半区、oracle 仅检查枚举/自造坐标，以及数据选择/归属提示发生变化等假设。实际结构、midpoint、原始结果和 PNG 彼此吻合。

**覆盖 ledger。** P 为 PROVEN，C 为 CLEARED；按下列 ID 集合逐项计数为 54/54，没有 PENDING 或 UNPROVEN。条件项以本差异未触发为依据清除，未将读取或工具失败当作证明。

| 章节 | P | C | 证据/清除依据 |
| --- | --- | --- | --- |
| SCOPE | 1–8 | 无 | 用户授权、已发布 Widget 的 L1 边界、直接差异和冻结身份、一次冷角色派发；独立角色全程只读 |
| TRACE | 1–6、8 | 7、9 | entry→model→frame→quota view→renderer、单/双端与空集合路径；没有新注册或异步变化 |
| DESIGN | 6、7、9、11 | 1–5、8、10 | quota owning boundary、RED 因果证据和最小 frame 差异；没有认证、数据、资源所有权、迁移或通用自定义机制变化 |
| VERIFY | 1–14 | 15 | 原生 renderer、稳定 geometry ID、ADD/KEEP 决策、直接原始日志/summary/PNG、harness 隔离和窄文档边界；无新增运行操作 |
| CLOSE | 1–4、7、9、11 | 5、6、8、10 | 主会话实际派发和最终核对、一次 closure、仅范围内因果/材料性裁决；无成立 finding、纠正研究或不可替代的代码验收前提缺失 |

**测试与文档决策。** 新单端真实几何测试 ADD；既有双端几何、窗口选择和配置渲染测试 KEEP。净增一个有独立缺陷信号的测试方法，包含 12 个场景；没有新增依赖、快照基线或 quarantine。fix carrier ADD 并承接本轮报告，roadmap UPDATE 仅登记下一版本规划候选；不修改 `docs/status.md` 或创建并行 main 状态记录。

**命令与限制。** 独立角色只读核对源码、Git、哈希、原始 `xcrun xcresulttool get test-results summary` 和五张 PNG，复用 RED/GREEN 而未重跑。主会话另从 GREEN xcresult 直接取得相同 24/0/0 结果；summary 保留在证据目录，SHA-256 `884137ced72a4c9d78fa6721501fce97778ecb51d612414c1c46804295aa0413`。CodeGraph 未完整提供部分测试 helper 后，独立角色一次降级至符号边界源码检查。原先 L0 composite receipt 无单项 exit 记录，独立角色未声称自己执行过两项；正式报告写入后的新文档状态由主会话执行严格 L0 并保存结果。临时 Skill 报告的已核对结果已提取到本载体，不形成第二报告权威。

**残余风险。** 这是隔离 `NSHostingView` fixture 证据，未覆盖安装版 WidgetKit、实机 VoiceOver 或完整语言/Dynamic Type 矩阵。单端尺寸 guard 为 `slot.height > canvas.height / 2`，不是独立测量标题/页脚边界；完整正文结论同时依赖实际 frame 组合及 PNG，不宣称穷尽未来所有布局退化。roadmap 的扫描数字由主会话先前直接运行证据支持，独立 lens 仅审其延期规划边界。CEv1 的历史能力缺失已经恢复；这不扩大本次代码评审为安装或发布验收。

### Task checkpoint

- **Task：** `ad-bug-widget-quota-centering`；WorkUnit binding `fix:widget-quota-centering`，没有 containing topic gate。审查身份为上述冻结 HEAD、四个 blob 和 manifest；本轮评审记录追加后的载体属于新的文档状态，代码/测试证据仍绑定原状态，未冒充 graph retarget。
- **Review：** PASS；**Task completion gate：** VERIFIED（已取得下述恢复查询结果）。本记录同步后，Beads 仅在最终精确状态门禁仍为 VERIFIED 时进入 `awaiting_commit`；Bug 关闭和 WorkUnit delivered lifecycle 继续等待实际授权交付，不重复已完成的 Review。
- **提交建议：** 最终精确状态门禁确认后，由用户另行授权仅本批四路径的逻辑提交；保留 required body、Codex trailer 与 SSH signature 检查。
- **推送建议：** 实际提交对象通过检查后再由用户另行授权推送；本次门禁恢复授权不执行交付或安装。
- **下一项前提：** 本批提交、推送和安装的明确用户授权，按各自交付边界处理。本 Lane A Task 的交付边界保持打开。

### CEv1 门禁恢复 — 2026-10-08

用户明确恢复 MCP、重新加载 session 并要求继续。新的会话工具曝光和实际 namespaced Cypher 均成功；此前两次 initialize 失败与 HTTP 可达性检查没有被当作图操作成功。

namespace 为 `github.com/kitdine/agent-deck`，WorkUnit 为 `fix:widget-quota-centering`。七项 required criteria 分别为 single-center、dual-center、regression-oracle、native-build、isolated-native-tests、independent-review、scoped-records，来源为用户确认的布局边界、Lane A readiness、独立评审与项目 L0/L1 验证规则。

首次声明查询正确返回 NOT_VERIFIED（7 项缺证据）。通过当前固定模板写入既有原生/RED/评审观察后，保留原始观察状态；仅为 carrier 追加报告的变化记录显式 preserves 判断及 target-bound roll-up，没有复跑产品检查或重新评审。RED 的 `pass` 指“成功检测预期错误布局”，不把旧产品贴顶布局记作正确。其历史 source blob `4167d2cd6fe0baf9cc7da902fe4d602b836da1e3` 由确切的工具变更历史重建，回归测试/harness 与 GREEN 相同。

评审后原始目标 `fix:widget-quota-centering:state:post-review-dee6f1aa7bda3530e71ec09f271e10ddd362a7e6d94dd7d2bbeada898d8394f0` 已取得 VERIFIED：7 required、7 passing top-level evidence，missing、invalidated、unresolved 均为空。请求节点的完整 payload 和关系 endpoint/kind/profile 已读回核对；两批节点为 9/9、25/25，两批关系为 7/7、43/43，关系写入前预检全部 ok。MCP write 返回更新统计而非 query RETURN，处理数以 exact-ID readback 确认，没有重放成功写入。

该结果覆盖本记录更新前的 carrier `3aac5e8b8190d4922a244e70f1141e3fb9c86ffc`。本节只同步已取得的门禁状态；随后对新的文档状态执行严格 L0、记录 Change/ImpactAssessment 与新目标绑定，并将最终精确状态的 MCP 查询完整结果保存到 `/private/tmp/agentdeck-quota-centering/cev1-final-gate.json`。此路径是原始查询收据，不是本地 fallback 或第二证据权威；最终状态以其 `target_content_state` 和结果 envelope 为准。
