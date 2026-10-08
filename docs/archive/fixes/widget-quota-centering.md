---
status: historical
created: 2026-10-07
retired: 2026-10-08
---

# 缺陷：额度 Widget 正文未在各自范围内垂直居中

## 交付与退役 — 2026-10-08

用户已明确授权提交、推送、PR、代码审查、合并、RC 发布及本地安装。产品修复由 [PR #51](https://github.com/kitdine/agent-deck/pull/51) 交付至 `release/v0.6.x`：最终源码提交 `33a20a8b20103f53b65cce808fa6c4cf3e9d8b5c`、合并提交 `0d2aa6077007a8a5d7c0a37366b83949486fbf9a`，实际树均为 `2a4ceaa8568ef3ca21dd0ac82966eba4e1450c61`。最终 exact-head GitHub review 无新 finding，CI 4/4 通过；源码 SSH 签名与 merge 的 GitHub GPG `valid` 已核对。

精确合并目标 `fix:widget-quota-centering:state:commit-0d2aa6077007a8a5d7c0a37366b83949486fbf9a` 已取得 CEv1 VERIFIED（7/7，missing、invalidated、unresolved 均空）。通过显式 Change、逐项 preserves 和 target-bound roll-up 复用相同树的 26 项原生测试及独立复评，没有改写旧 `d382866` 的 FAILED 观察。原始查询收据为 `/private/tmp/agentdeck-quota-centering/cev1-pr51-merge-gate.json`。

依据已交付的修复边界退役本载体，保留下方全部历史。**WQC-D-F1 — CLOSED in archive candidate：** [PR #52 comment 4216224193](https://github.com/kitdine/agent-deck/pull/52#discussion_r4216224193) 指出的 active fix 已移至 `docs/archive/fixes/`，设置 historical/retired，并修复本载体相对链接；不新增 archive-index 条目。

RC2 发布与本地安装仍属后续交付边界；本载体退役不冒充它们已完成。PR #52 的三项红色 CI 已定位为 main workflow 检出旧 release head 而缺少文档工具；Go 与 desktop 产品步骤成功，仅文档依赖门禁失败。最小修正通过 [PR #53](https://github.com/kitdine/agent-deck/pull/53) 在 main 处理，保持 release→main 前传方向。长扫描性能 Bug 继续 deferred，本批不实施。

## 现象

用户提供的 v0.6.5-rc.1 原生截图中，medium 和 large 的 Codex 单窗口内容紧贴标题，正文下方留下大量空白。用户确认的 Lane A 边界为：在各自范围内垂直居中；一个客户端使用完整正文，两个客户端上下等分，各自在半区内居中。

Beads：`ad-bug-widget-quota-centering`。工作区 `agent-deck.fix.widget-quota-centering`，分支 `fix/widget-quota-centering`，基点为实时核对的 `origin/release/v0.6.x`（`8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`）。CEv1 WorkUnit 为 `fix:widget-quota-centering`。

## 根因

[`QuotaWidgetView`](../../../apps/macos/AgentDeckWidget/WidgetViews.swift) 只在双客户端 large 分支中创建居中弹性槽。single 分支直接返回客户端块，外层 `WidgetFrame` 的 `.topLeading` 把它贴在正文顶部。

安装版本提交 `5e6d162366f1b82f3056de594c707c33d4a1aee2`、main、远端 main 和修复基点的 `WidgetViews.swift` blob 均为 `e1e55f37fee1976cd958bd1c4c42557ac55e12ac`，排除了本地代码与安装源码不同这一解释。既有测试虽然在名称中提到单端，却只检查单端布局枚举；实际 midpoint 断言仅覆盖双端。

## 修复边界

仅修改额度 Widget 的单槽垂直布局并扩展既有几何 preference 观测；标题、页脚、窗口数据和客户端筛选不变。真实 SwiftUI 渲染断言覆盖 Codex/Claude、一窗/多窗、small/medium/large，保留双端等高半区居中的既有验证。

长扫描性能问题另由 `ad-bug-menubar-long-scan-timeout` 延期跟踪，已纳入 [`roadmap.md`](../../roadmap.md) 的下一版本规划候选；本次不修改扫描、超时或后台工作状态。

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

## Review — Round 2

## 📋 PR #51 多窗口边界评审 — 2026-10-08

📊 综合评分：7/10（被评审提交 `d382866`）

✅ 结论：FAIL

- **Reviewer：** GitHub `chatgpt-codex-connector[bot]` exact-head review `5451726560`；主会话以真实渲染验证并裁决。
- **Method：** 单一界面边界 finding 的聚焦反例与负对照；CI 4/4 作为既有证据，不替代 finding 处置。
- **Scope：** single-client medium/large 的任意合法窗口数量及首行在正文范围内的可见性；没有进入 scanner、账号或新 UI 控件。
- **Reviewed state：** commit `d382866c2b532f4eeb8e01837d97f45931211486`，tree `e3d984a39e28d8bef6e901e6eb4771f7a4cb86fc`，产品 blob `c41771781424fa2f2741b362a54ad7446a0c8873`。临时几何观测只增加 absolute 坐标，不改变被诊断的布局。
- **Completion gate：** FAILED（该提交的布局边界被新反例否定；原有已通过观察保留为历史，不覆盖它们）。

### 🔴 严重问题 — 必须修复

**WQC-R2-F1 — P2；`apps/macos/AgentDeckWidget/WidgetViews.swift:449`；single 槽没有限定为真实可用正文，多窗口时首行落在卡片上方。**

- **行为风险：** medium/large 必须接受 `quotaWindows` 返回的所有已报告窗口；`WidgetDomain.swift:315-321` 明确任意窗口数。仅覆盖一/两窗口不能保证不把领先数据放到卡片外。
- **证据：** [GitHub comment 4214984015](https://github.com/kitdine/agent-deck/pull/51#discussion_r4214984015)。12 窗口、两客户端分别作为单端、两尺寸的实际 viewport RED 共四项失败；Codex medium 的 content height 482、card height 155、content minY -162，large minY -62.5。Claude 对应 -169.5/-70。实际卡片 minY 为 0。
- **限定根因：** relative slot 会随 intrinsic minimum 膨胀至 482/497，不能代表可用正文。只将 `.center` 改成 `.topLeading` 的负对照仍得到相同负坐标，因此仅改 alignment 不足以完成修复。首个 relative-slot fixture guard 的失败不是有效 RED，不把它计作反例成功。
- **处置：** 此 reviewed commit 上 OPEN；限定 working repair 已完成，等待独立复评和新的 exact-head GitHub review。原 Round 1 PASS 仍是其所观察的一/两窗口候选历史。

💡 **限定修复：** single 正文槽按真实可用尺寸布局；内容能容纳时居中，不能容纳时从槽顶部显示；保护固定标题、页脚，保持模型和 ForEach 的所有窗口。用 actual card/content geometry 检测越界，不用膨胀后的 slot 当 viewport。采用平台 [ViewThatFits](https://developer.apple.com/documentation/swiftui/viewthatfits) 选择第一个能容纳的子视图，2026-10-08 已读取 Apple 官方定义。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

既有正常内容的 12 个几何场景和双端居中检查可直接保留；新增失败信号指向真实卡片边界。

### 📝 总结

必须修复 WQC-R2-F1 后再合并。此前 CI 全通过并不能关闭该 finding。无新版本已发布，未安装未合并候选。

工作区修复就绪证据：产品 blob `be2dfc52956d72d4ad864d4e57b9d07a694fd000`，测试 blob `05cb9b3a6366940b720b29687df36558857086f4`。`overflow-viewport-red.log` SHA-256 `57bcbe4cdc58fb06ea025b9f1015c2861fb494f571d9667c6c556add8265b229`；top 对照 `12f8204c42f42ae7860bdd737f3308680973027ca6b72cd7cf38460c638346bf`；修复 suite `overflow-fit-green.log` `c62c160776bfcfb066938e621a33d85dc57d8387a278670ab817d8d097104129`，exit 0、25 tests、0 failures，均为隔离 HOME/state。

第一版 minHeight 双 frame 修复仍在 actual viewport 断言失败，已放弃；最终使用显式 GeometryReader 槽和 ViewThatFits，不提高时间预算、削弱断言或限制 windows 数量。新增 absolute geometry ID 是仓库拥有的观测契约；正常单端检查仍确认只有一个语义 slot/content，双端实现未改。

本次用户全流程授权已涵盖 finding 修复与复评。旧提交上的修复范围仅 WQC-R2-F1；当前 working candidate 完成上述修复，下一操作是独立复评，不重复已执行的修复。Task 和交付边界仍打开。

## Review — Round 3

## 📋 PR #51 溢出修复选择性复评 — 2026-10-08

📊 综合评分：10/10（最终修复代码与回归保护）

✅ 结论：PASS

Checklist: 54/54 complete；Incomplete: None。

- **Reviewer：** `/root/quota_centering_cold_review`，独立只读角色；主会话核对直接结果并负责 finding 处置。
- **Method：** 同一 Task 的唯一选择性 follow-up，累计两轮；没有嵌套代理或第三轮。原样复用未改变的来源，重新核对改变的边界；独立角色未重复 suite，完成一次 closure。
- **Scope：** single 有限正文槽、正常居中、溢出顶部保留、裁剪及归属提示可见性；没有扩大到双端溢出、窗口展示重设计、scanner、安装或发布。
- **Reviewed state：** HEAD `d382866c2b532f4eeb8e01837d97f45931211486` 加 working correction；产品 blob `cdb376d43eb0cb6762d1ae8226885badd44d68be`、测试 blob `b4d794f335fa25c1cb892c17c6fa4ce16034b116`、评审前 carrier `5237f540ac0735d3513220dfb1365de6f0a42f00`、roadmap `41822be0f3da6e5d5ab4b872a2267f0ce218aa09`。独立角色和主会话均核对实际最终代码。
- **Completion gate：** VERIFIED。本轮 corrected evidence 已实际取得7/7通过；旧提交 FAILED 保留。以下门禁收据与最终状态绑定承接本报告的 metadata 同步。

### 🔴 严重问题 — 必须修复

当前无 OPEN finding。

**WQC-R2-F1 → CLOSED in candidate。** 有限 GeometryReader 槽阻止 intrinsic minimum 撑大整个正文；平台 ViewThatFits 首选能容纳的居中 candidate，溢出 candidate 从正文顶部显示。四个 12 窗口 actual viewport 反例现通过；正常 12 场景及原双端几何检查仍通过，模型和 ForEach 没有 row cap。

**WQC-R3-F1 — P2 — CLOSED in candidate；初始裁切修复隐藏了 Claude medium 三窗口的归属提示。**

- **位置：** `WidgetViews.swift` overflow candidate 内部的末尾归属提示。
- **行为风险与因果：** 初始 `be2dfc…` 将提示放在整体被裁切的窗口块末尾。相同三窗口场景的旧 `d382866` 布局通过，初始新 candidate 提示 CGRect `(0,156,136,10)` 超出卡片 `(0,0,338,155)`，更不在实际 clipped body 内；这是改变造成的 C6 归属提示可见性回归。
- **证据：** `attribution-old-control.log` exit 0；`attribution-new-red.log` exit 65 的真实 Text 位置失败。没有以观测 ID 数量冒充可见性或 AX 证明。
- **已执行的限定纠正：** overflow 的可裁切窗口块使用 `includesAttribution: false`；同一 predicate/helper 在可见正文末端输出一次提示。normal/dual 默认保持 `true` 和原位置，文案、条件、字体和颜色均未改。最终 Text 同时处于 card 与实际 clipped slot 内。

💡 **纠正依据：** 保留既有 Widget UX 的可见归属提示；继续复用已验证的 [ViewThatFits](https://developer.apple.com/documentation/swiftui/viewthatfits) 和单一提示 helper，未增加自定义布局框架或依赖。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

两个新增测试方法分别提供“首行被裁切”和“归属提示消失”的独立失败信号，包含旧/新负对照；正常居中与双端证据保留。

### 📝 总结

最终相关 suite exit 0，26 tests、0 failures；原生测试仍使用隔离 HOME/state 和既有 helper 清理。字段如下：

| 验收 | 直接证据 | 结果 |
| --- | --- | --- |
| 正常内容居中 | 既有 12 场景 midpoint 检查 | PASS |
| 12 窗口保留首行于有限正文 | 四个 actual card/content/slot absolute 几何场景 | PASS |
| 模型/ForEach 保留所有窗口 | count 12 和未截断的窗口集合 | PASS |
| header/footer 受保护 | 正文内部裁剪、正文有限高度 | PASS |
| Claude 提示可见 | C3 old/new controls；最终同时处于 card/slot | PASS |
| 不新增重复提示 | overflow block 排除提示，仅外部同一 helper 输出；normal/dual 默认不变 | PASS（静态组合与平台单一 child 合同） |

覆盖 ledger：SCOPE 1–8 P；TRACE 1–6/8 P、7/9 C；DESIGN 6–11 P、1–5 C；VERIFY 1–14 P、15 C；CLOSE 2–9/11 P、1/10 C。P 为 PROVEN，C 为 CLEARED，共 54/54，没有 PENDING/UNPROVEN。未改变的条件复用首轮来源；变更处按最终源码和原始结果核对。

证据保留于 `/private/tmp/agentdeck-quota-centering/`：viewport RED `57bcbe4cdc58fb06ea025b9f1015c2861fb494f571d9667c6c556add8265b229`；top control `12f8204c42f42ae7860bdd737f3308680973027ca6b72cd7cf38460c638346bf`；C3 old control `6299ad8efa6e77168ae4d3cc13094614163e9a6985ac15e753696ddf6af27418`；C3 RED `080c1e3b56e260e994bf63c937ff6227d878134e8c911a03ec876cb78846bc21`；最终 `attribution-fit-green.log` SHA-256 `75e22794600407d95b9adc16227fc719a6dec2c2ab597b03992eb00893fd2f24`。

测试决策：KEEP 正常/双端测试，ADD actual viewport 与 C3 归属提示方法，UPDATE 观测 ID 过滤；相比旧提交新增两个不同失败信号的测试。文档决策为同一 carrier 追加历史和新处置，不创建第二报告。

限制与收窄：dictionary key count 证明的是观测 ID 集合，不能单独证明 AX tree 唯一性；Round 2 中“语义 slot/content”只指其布局观测，不作 AX 证明。唯一提示依据分支组合和平台单一 child 合同；未运行实机 VoiceOver。有限视口会裁剪末尾窗口，但模型及 ForEach 保留全部，不宣称 12 个窗口可同时可见。fixture 不等同于安装版 WidgetKit。独立角色的 xcresult summary 因 TestReport 缓存写入限制不可用，未换权限重试；结论来自原始日志，不将失败工具作产品 finding。

### Task checkpoint

- Task `ad-bug-widget-quota-centering` / WorkUnit `fix:widget-quota-centering`；两个 finding 均已在上述最终代码关闭，Review PASS。
- 新内容的 task gate 已实际 VERIFIED7/7；本报告同步后以最终精确状态收据确认，再到 awaiting_commit。
- 提交建议：门禁 VERIFIED 后，按用户已经明确授予的全流程交付授权追加新的逻辑修复提交，不改写已推送的 `d382866`。
- 推送建议：检查实际消息/body/Codex trailer/SSH signature后普通推送，再请求 exact-head GitHub review；不在旧 review 上直接 merge。
- 后续：相同 PR 修复关闭后继续已授权的 merge、前传 main、同 SHA preflight、RC 和本地安装，不启动性能实现。

本轮 CEv1 修复目标 `fix:widget-quota-centering:state:repair-ee8b6db3e99c59eed43677171840b2d596c67a809ffc81f7e936d10cf13008f8` 已实际 VERIFIED7/7（23/23节点、39/39关系读回，预检全部ok，missing/invalidated/unresolved空）。它以新cdb376/b4d794原生26结果和本次独立复评为依据，未复用被反例否定的旧代码检查。metadata同步后的最终gate详情保留于 `/private/tmp/agentdeck-quota-centering/cev1-repair-final-gate.json`；不覆盖旧commit失败记录，不进行graph retarget冒充，也不重复产品测试。
