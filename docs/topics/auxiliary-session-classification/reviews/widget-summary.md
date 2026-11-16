---
status: active
topic: auxiliary-session-classification
subject: widget-summary
---

# Widget Summary 评审

## Round 1 — 2026-11-09

## 📋 Widget Summary 独立评审

📊 总体评分: 6/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-WS-R1-F1 — OPEN，P2：保留的旧会话数字显示为刚更新。** `apps/macos/AgentDeckWidget/WidgetDomain.swift:186` 的会话行丢弃 summary 成功时间，`WidgetViews.swift:630` 的相对时间采用外层快照时间，且 stale 不影响会话文字。新外层快照成功、会话批次失败并保留两天前 summary 时，旧数字会配上 `Updated now`，违背 `ux/widget.md` 的旧数据及成功时间共同保留合同。独立诊断以外层 2026-08-15、summary 2026-08-13 复现 stale 与 fresh 文本一致。修复须将实际 summary 成功时间和 stale 传至显示层，并覆盖新外层时间与保留旧 summary 的消费者回归。

**ASC-WS-R1-F2 — OPEN，P2：宿主恢复路径把缺失 unknown_count 转成合法 null。** `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:75` 的 Codable 聚合将缺失字段读为 nil，`WidgetSnapshotPublisher.swift:134` 保留旧条目后重新编码为显式 null。独立往返诊断中，原畸形文件在 Widget 直接解码为 unavailable，经宿主保留并重新发布后却显示 Work 5、unknown nil、stale。`ux/widget.md` 明确规定字段缺失不同于 null。修复须在宿主保留前拒绝缺失字段，同时允许显式 null，并覆盖完整保留和重新发布路径。

**ASC-WS-R1-F3 — OPEN，P2：未知 reason 仍显示有效计数。** `apps/macos/AgentDeckWidget/WidgetSnapshot.swift:77` 将 reason 解码为任意字符串，消费者未拒绝未知值。独立诊断传入 `future_reason` 后仍显示 Work 5、Unknown 2，违反 `ux/widget.md` 未知原因只显示通用不可用的合同。修复须采用封闭原因枚举或等价验证，令未知值仅降级会话行并保留独立消耗，补充兼容回归。

### 🟡 建议改进 — 推荐

无。本轮所有发现均为必须关闭的合同问题。

### 🟢 优点

- 实现覆盖九种 client/period 组合、固定 generation/as-of、整批拒绝和有界重试；联合 fixture 对各组合提供不同计数。
- App Group 聚合以白名单限制内容，旧 `sessions.total` 保持原义。

### 📝 总结

- Reviewer: Codex `gpt-6.1-sol`，独立冷上下文，只读；主代理核对候选摘要、合同和发现位置。
- Method: 单评审者源码与合同核对、隔离 Swift 生产源码诊断、xcresult 读回与渲染附件目视；CodeGraph 未定位新增文件后使用目标路径检查。
- Reviewed state: HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 个产品/测试路径有序 `path blob\n` 清单 SHA-256 `c171251a8777eb4b66ad605209e4602e344f2b8f57db6a8d513f4b20b3b073ca`；`tasks.md` blob `c7feba9c406175563cd6a85185f9b8ee38fc3d0d`。
- Evidence: 审查者读回 xcresult：Shared 146/146，App 233 通过、3 跳过，Widget 55/55；三尺寸普通字号附件目视。隔离 Swift 诊断复现上述三项缺陷。Go 全量、race 和 vet 仍是开发者报告，未独立读回原始执行证据。
- Baseline: `desktop/fixtures/v1/verify.swift` 对当前与 HEAD 的 schema-ahead fixture 均报 `invalid schema-ahead fixture`，不是本候选新引入的失败。
- Acceptance scope: 用户在独立评审线程明确表示 VoiceOver 和其他辅助功能，包括无障碍大字号/Dynamic Type，不需要也不验收；这些不作为本轮缺口。真实 WidgetKit 时间线调度、系统编辑配置和桌面点击仍未验收。
- WorkUnit: `auxiliary-session-classification:widget-summary`；候选 ContentState 请求 `auxiliary-session-classification:widget-summary:candidate:c171251a8777eb4b66ad605209e4602e344f2b8f57db6a8d513f4b20b3b073ca` 尚未登记。完整 canonical gate 返回 `missing_target_state`，13 项 required criterion 无已绑定证据。
- Completion gate: NOT_VERIFIED
- Disposition: ASC-WS-R1-F1、F2、F3 均 OPEN；Review 保持未勾选。修复后须独立复评，且不得把本轮 FAIL 或当前门禁描述为通过。

### Repair 1 交接（待复评）

- 修复者报告针对 F1 保留 summary 成功时间并显示年龄，针对 F2 区分缺失键与显式 null，针对 F3 拒绝未知 reason；修复处置尚未由评审者确认。
- HEAD `79be6193cea36ab3137df147ace5fb30428df956`，30 个产品/测试路径指纹 `50f74c24763666cba916de77ae9c2d249a86986fcf4ba54bac90af7254cb5fbc`；主代理已按有序 path/blob 清单独立核对。
- 修复者报告 `scripts/test-macos-app.sh` 最终退出 0：Shared 147 通过，App 236 项含 3 跳过，Widget 57 通过；前两次为修复引入的编译错误。原生 WidgetKit 时间线、配置及桌面点击仍未验收。保留时间的 60 秒容差和宿主整组拒绝畸形条目的语义须复评。

修复：auxiliary-session-classification / widget-summary，处理 ASC-WS-R1-F1、F2、F3。

## Round 2 — 2026-11-09

## 📋 Widget Summary 独立复评

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-WS-R2-F1 — OPEN，P2：一个畸形条目会清除其他有效窗口的 summary。** `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:199` 严格解码整个 items 数组；一个条目缺少 `unknown_count` 时外层 decoder 将整组视为缺失，`WidgetSnapshotPublisher.swift:137` 随后无法保留有效条目。独立诊断以九个组合中仅 codex/7d 缺键复现：Widget 直接读取得到 8 个有效、1 个不可用；冷宿主保留并重新发布后变为 0 个有效、9 个不可用，独立消耗仍可读。此行为扩大了 `ux/widget.md` 对受影响分类行的降级范围。修复须在不把缺失字段转成合法 null 的前提下保留其他条目及其原身份/时间，并覆盖九条目冷宿主保留与重新发布；保持原子发布。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 原始 stale、缺键/null 与未知 reason 三项复现路径已得到针对性修复；独立消耗保持可读。

### 📝 总结

- Reviewer: Codex `gpt-6.1-sol`，新建独立冷上下文，只读；主代理核对候选摘要、合同与发现位置。
- Method: 工作区绑定 CodeGraph、目标源码/合同核对、隔离生产 Swift 源码诊断与 xcresult 读回。
- Reviewed state: HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 个产品/测试路径有序 `path blob\n` 清单 SHA-256 `50f74c24763666cba916de77ae9c2d249a86986fcf4ba54bac90af7254cb5fbc`。
- Disposition: ASC-WS-R1-F1、F2、F3 对原始触发序列 FIXED；F2 相关新回归以 ASC-WS-R2-F1 OPEN 承接。60 秒容差在 stale=true 的 0、1、59、60 秒诊断均保留 summary 时间；未发现其复现 F1 的受支持路径。
- Evidence: 复评者读回最终 xcresult 437 通过、3 跳过、0 失败，两个重启 bundle 各 1 项通过；隔离生产源码诊断退出 0。Go 全量、race、vet 仍为开发者报告；本轮未重跑未改变的广泛套件。schema-ahead fixture 检查失败为先前确认的基线。
- Acceptance: 用户排除 VoiceOver、Dynamic Type 等辅助功能验收。真实 WidgetKit 时间线调度、系统配置和桌面点击仍未验收；本轮未重新目视渲染附件。
- WorkUnit: `auxiliary-session-classification:widget-summary`；候选请求 `auxiliary-session-classification:widget-summary:candidate:50f74c24763666cba916de77ae9c2d249a86986fcf4ba54bac90af7254cb5fbc` 的完整 canonical gate 返回 `missing_target_state`，13 项 required criterion 缺已绑定证据。
- Completion gate: NOT_VERIFIED
- Disposition: ASC-WS-R2-F1 OPEN；Review 保持未勾选，修复后须独立复评。

### Repair 2 交接（待复评）

- 修复者报告宿主逐条解码 summary：仅丢弃畸形条目，保留其他条目身份及时间；缺键不转为合法 null，未知 set-level reason 仍拒绝整组。该处置尚待独立复评。
- HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 路径有序 path/blob 摘要 `fdd7a8c35ee53cdf956d2ecc1db17765598a080223514be64f62849928d783d4`，主代理已独立核对。
- 修复者报告 `swift test` Shared 149 通过，`make test-macos-app` App 236 项含 3 跳过、Widget 58 项，两个重启阶段通过；空白及 diff 检查通过。实际 WidgetKit 时间线、系统配置及桌面点击仍未验收。

修复：auxiliary-session-classification / widget-summary，处理 ASC-WS-R2-F1 并保留 Round 1 修复。

## Round 3 — 2026-11-09

## 📋 Widget Summary 独立复评

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-WS-R3-F1 — OPEN，P2：丢弃畸形重复项会把有歧义的选区变成有效数字。** `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:206` 将无法解码的项及其 client/period 身份一起丢弃；`apps/macos/AgentDeckWidget/WidgetDomain.swift:189` 随后只看到一个匹配项。独立生产源码诊断以九个有效项加一个缺失 `unknown_count` 的 codex/7d 重复项复现：直接读取时该行 unavailable，其余八行有效；第一次冷宿主重新发布后 codex/7d 错误显示 Work 105、Unknown 2，第二次仍然显示；独立消耗始终可读。没有新有效批次却将有歧义的输入变成可信数字，违反重复选区不可用边界。修复须在丢弃畸形载荷前保留可读取的 client/period 身份，对重复身份整体保持不可用，同时保留其他窗口及原 publication/time；补两次冷宿主重新发布的回归。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 单个畸形非重复项的原始回归已限制在对应窗口，其他八个有效条目和时间保持。

### 📝 总结

- Reviewer: Codex `gpt-6.1-sol`，新建独立冷上下文，只读；主代理核对候选摘要和发现位置。
- Method: 源码/合同核对、隔离生产 Swift 源码诊断和 xcresult 读回。
- Reviewed state: HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 个产品/测试路径有序 `path blob\n` 清单 SHA-256 `fdd7a8c35ee53cdf956d2ecc1db17765598a080223514be64f62849928d783d4`。
- Disposition: ASC-WS-R1-F1、F2、F3 与 ASC-WS-R2-F1 的原始触发均 CLOSED；新回归 ASC-WS-R3-F1 OPEN。原 F2 的缺键经两次冷宿主发布仍不可用，显式 null 保持合法；未知 reason 拒绝且消耗可读。
- Evidence: 独立生产源码诊断退出 0，复现上述重复项行为；复评者读回 xcresult 440 通过、3 跳过、0 失败，以及两个重启 bundle 各 1 项通过。xcresult 未内嵌完整候选指纹；Go 全量/race/vet 仍为开发报告。schema-ahead fixture verifier 为已记录的基线失败。
- Acceptance: 用户排除 VoiceOver、Dynamic Type 等所有辅助功能验收。真实 WidgetKit 时间线调度、系统配置和桌面点击仍未验收。
- WorkUnit: `auxiliary-session-classification:widget-summary`；完整 canonical gate 对候选请求 `auxiliary-session-classification:widget-summary:candidate:fdd7a8c35ee53cdf956d2ecc1db17765598a080223514be64f62849928d783d4` 返回 `missing_target_state`，13 项 required criterion 缺已绑定证据。
- Completion gate: NOT_VERIFIED
- Disposition: ASC-WS-R3-F1 OPEN；Review 未勾选，Repair 3 后须独立复评。

### Repair 3 交接（待复评）

- 修复者报告以不含数字的 unavailable marker 保留可读取的畸形 client/period 身份，使重复选区在冷宿主重新发布后继续不可用；无法读取的身份丢弃。该处置及其合同等价性待独立复评。
- HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 路径有序 path/blob 摘要 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672`，主代理已独立核对。
- 修复者报告隔离 `make test-macos-app` App 236 项含 3 跳过、Widget 59 项通过，两个重启阶段通过；Shared `swift test` 151 通过。其后仅删除测试文件末尾一行空白，相关测试组重跑 7/7，完整 Xcode 套件未在该空白变更后重跑。真实 WidgetKit 时间线、配置和桌面点击仍未验收。

修复：auxiliary-session-classification / widget-summary，处理 ASC-WS-R3-F1 并保留此前修复。

## Round 4 — 2026-11-09

## 📋 Widget Summary 独立复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 畸形载荷保留可读取的选区身份，以不含数字的 unavailable marker 保持重复项不可用；其他窗口及原 publication/time 保留。
- 缺键与显式 null、未知 reason、独立消耗和旧成功时间均保持原有边界。

### 📝 总结

- Reviewer: Codex `gpt-6-astra / medium`，新建独立冷上下文，只读；主代理核对候选摘要、marker 与 Widget 唯一性路径。
- Method: 生产 Swift 源码与合同核对、16 个隔离场景经直接读取与两次全新 publisher 从磁盘重新读取并原子发布，共 48 个观测，及既有 xcresult 读回。
- Reviewed state: HEAD `79be6193cea36ab3137df147ace5fb30428df956`；30 个产品/测试路径有序 `path blob\n` 清单 SHA-256 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672`。
- Disposition: ASC-WS-R1-F1、ASC-WS-R1-F2、ASC-WS-R1-F3、ASC-WS-R2-F1、ASC-WS-R3-F1 均 CLOSED；无新发现。畸形与有效重复项在两次冷 publisher 重新发布后仍不可用，其他窗口保持原数据及时间；marker 只含 available=false、client、period。
- Evidence: 当前生产源码诊断 48/48 观测通过。复评者独立读回既有 Xcode 结果：Shared 151 通过、App 233 通过及 3 跳过、Widget 59 通过，两个重启 bundle 各 1 项通过；0 失败。完整套件运行后测试文件删除一行末尾空白，故不称其为最终 blob 上的完整套件运行；修复者报告其后相关测试组 7/7，原始结果尚未独立定位。Go 全量/race/vet 保留为开发报告，未独立重跑。
- Acceptance: 用户明确排除 VoiceOver、Dynamic Type 和所有辅助功能验收。诊断使用独立 publisher 实例，不等于独立操作系统宿主进程；真实 WidgetKit 时间线调度、系统配置和桌面点击仍未验收。schema-ahead fixture verifier 为先前确认的基线失败。
- WorkUnit: `auxiliary-session-classification:widget-summary`；候选 ContentState `auxiliary-session-classification:widget-summary:candidate:25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672` 已登记；完整 canonical gate 为 10/13 项有可用证据，`atomic-recovery`、`native-display`、`timeline-actions-budget` 缺目标绑定证据。
- Completion gate: NOT_VERIFIED
- Disposition: 独立复评 PASS；Task 完成门禁尚未满足，保持 `in_review`，提交须等待证据和验收缺口处理。Review PASS 不代表 CEv1 VERIFIED 或真实 WidgetKit 验收通过。

### 独立验证交接 — 2026-11-09

- 候选指纹和 HEAD 未变。独立验证读取 Xcode 结果：Shared 151 通过、App 233 通过和 3 跳过、Widget 59 通过，两个重启阶段各 1 项通过；完整套件后的测试源码仅删除末尾一行空白，相关测试组在最终内容上 7/7，完整套件按语义复用，不称为最终 blob 上执行。
- 独立验证找回 Go 1.27.3 的最终全量日志：29 个包、1615 个顶层 PASS、无失败；desktop/sessionorigin 与 CLI 选定 race 运行通过，原始命令记录 `make vet` 退出 0。之前两次全量运行失败，与最终通过运行分开；当前默认 Go 1.27.1 不作为这些结果的执行环境。
- 2026-11-09 15:36 UTC，独立验证使用隔离临时目录及当前生产 Shared Swift 源码执行独立进程诊断：60 次原子替换、400 次读取，0 混合快照、0 解码失败；写入进程在 prepare 后、commit 前退出时旧文件字节不变；后续两个进程保留九组数据与时间，仅标 stale。该诊断不是实际 WidgetKit 验收，临时产物已清理。
- 首次原生预检发现运行中的 Widget 扩展来自已安装 `/Applications/AgentDeck.app`，原候选扩展同 bundle ID 且未签名，因此未操作桌面 Widget。后续建立的独立候选及其部分验收见下文。
- CEv1 已登记此候选 ContentState、Round 4 独立复评和九项软件验证的目标绑定证据。Go 来源候选 `c171251a8777eb4b66ad605209e4602e344f2b8f57db6a8d513f4b20b3b073ca` 到当前候选只改八个 Swift 路径；Xcode 来源候选 `b35ebed38b90e7dca8c73eab5f46be9c3e0e9b253dddc8d83fe2602ef589b41e` 到当前候选仅删测试文件末尾一行空白。两处均记录变更、逐项 preserves 评估及目标绑定 rollup；L3 另包含当前候选的隔离跨进程检查。关系预检 60/60 与节点 34/34、关系 60/60 按 ID 读回均匹配。写工具只返回更新计数器，遵照用户此前明确同意的按 ID 精确读回确认。完整 canonical gate 仍 NOT_VERIFIED：10/13 可用，`atomic-recovery`（真实 WidgetKit 延迟部分）、`native-display`（真实系统配置及显示）和 `timeline-actions-budget`（真实调度及点击）缺目标绑定证据；未运行的原生验收未记为通过。

### 隔离 WidgetKit 部分验收 — 2026-11-09

- 验证者以候选 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672` 的临时副本构建并签名独立 App 和 Widget，使用独立 bundle ID、私有 App Group、临时 HOME 和九组选区 fixture；报告宿主与扩展的 PID/路径、签名和注册身份均已核对。生产仓库内容未改变。
- 系统 Widget 选择器列出独立测试提供者 `AgentDeckWidgetTest-Du4Zc6`；验证者观察到小号和大号预览显示 `Work 32 sessions · Unknown 1`，中号显示消耗。公开 WidgetKit 配置查询确认已添加一张测试 `Magnitude / systemSmall`。这些是预览和配置存在性观察，不证明实际桌面卡片的全部配置、延迟、时间线或点击行为。
- 验证工具未能绑定新增测试卡片：NotificationCenter 返回既有 Clock I，其他系统宿主返回 timeout 或无法识别扩展。测试宿主与扩展进程已结束、扩展注册已撤销；注销后公开 SDK 仍报告 1 张测试配置，卡片移除尚未确认。测试生成物为后续准确恢复保留，不作为持久证据引用。已安装 Widget 仍运行；未声称桌面完全恢复。
- 本次为部分原生观察，不改变 CEv1 `NOT_VERIFIED` 10/13。真实配置矩阵、WidgetKit 延迟/调度和桌面点击仍未验收。

Task checkpoint：候选 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672`，门禁 NOT_VERIFIED。
提交建议：等待候选证据门禁及真实 WidgetKit 验收后，按任务边界本地提交。
推送建议：本次未授权推送。

### 真实 WidgetKit 验收 — 2026-11-09（America/New_York）

- 执行者：claude-code（主代理），按用户指令完成剩余原生验收；不是独立评审，Round 4 PASS 不变。
- 候选：HEAD `79be6193cea36ab3137df147ace5fb30428df956` 加 30 路径摘要 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672`（验收前后各重算一次，未变）。以 `scripts/build-macos-app.sh` 在工作树外的临时 DerivedData 构建 Debug，复制后改为独立身份 `com.kitdine.agentdeck.acceptance.wsaccept` 与 `.widget`、私有 App Group `N2FZ2FNRTU.group.com.kitdine.agentdeck.acceptance.wsaccept`、显示名 AgentDeckWSAccept，Developer ID（团队 N2FZ2FNRTU）签名。Xcode 26.4，Go 1.27.3。
- 加载核对：pluginkit 只登记一份该候选扩展，路径为候选副本；实际运行的扩展进程来自候选副本，沙箱生效、无子进程、无沙箱拒绝。已安装 `/Applications/AgentDeck.app` 的扩展是另一进程，未被操作。
- 私有 fixture：隔离 HOME 中放 1,262 个合成 Codex/Claude 会话文件，经真实 `session scan` 索引，Auxiliary/Work 用 `session classify --set` 设定，其余为 Unknown；每个会话一条用量事件；默认 Work 已确认。宿主发布的九组 Work 与 CLI `desktop session-scope` oracle 一致：all 5/14/1,250（Unknown 1/2/1,236），codex 2/6/7（0/1/1），claude 3/8/1,243（1/1/1,235）。
- 桥接限制：隔离模式下宿主按设计把快照写到测试 HOME 下的 `app-group/`，不写真实容器。验收时把宿主发布的原字节以临时文件加 rename 原子复制进私有容器。宿主直接写 App Group 的路径由既有单元测试和跨进程证据覆盖，本次没有重新观察。
- 配置与显示：用系统 Widget 编辑器把小号 Magnitude 依次配置为全部九种 client/period，每次渲染的会话行都与 oracle 一致；大号的 all/today 显示 Work 5、Unknown 1，claude/30d 显示 Work 1,243、Unknown 1,235；中号不显示计数。系统语言 en-US、区域 en_US，千位分隔正确；Token 行在每种组合下都等于全量用量。zh-Hans 未做原生渲染。
- 延迟下保留旧快照：用 CLI 保存默认 Auxiliary（revision 1，`publication_pending`）后宿主没有新发布，卡片保持 Work 1,250 / Unknown 1,236 和原成功年龄。随后宿主发布完整 Auxiliary 批次（all 4/8/12，codex 1/2/5，claude 3/6/7，Unknown 0），复制进容器时不调用任何 reload。卡片在 WidgetKit 自己调度刷新（23:08:58）之前完整保留旧 Work 快照和旧年龄，之后整体换成 Aux 12 / Unknown 0 和新年龄；大号 claude/30d 换为 Aux 7 / Unknown 0。全程未观察到 scope、计数和时间混搭。
- 时间线调度：chronod 对候选 Magnitude 时间线按策略下限 3 分钟安排 `timelineExhausted` 刷新（例：23:04:02 生成的时间线排到 23:07:02，budgeted）；23:08:58 的刷新没有任何 `reloadTimelines` 调用，并读到了新快照。
- 桌面点击：chronod 记录 `Launched com.kitdine.agentdeck.acceptance.wsaccept for click on` 候选 magnitude Widget，由正在运行的隔离宿主（同一 PID）接收；没有新实例，已安装 AgentDeck 未被拉起，默认范围仍为 auxiliary revision 1，没有命令或写入。点击时临时设置的 `launchctl` 测试 HOME 兜底已在点击后撤销。
- 范围：用户明确排除 VoiceOver、Dynamic Type 和其他辅助功能验收。操作方式：computer-use 无法授权系统 Widget 编辑器，经用户同意改用 scratchpad 内的 CGEvent 小工具和 `screencapture`，只截内置屏、只操作测试卡；用户的时钟 Widget 保持不变。
- 测试工具问题（不属产品缺陷）：最初用 `/private/tmp/...` 拼写的 HOME 播种，宿主把 `AGENTDECK_TEST_HOME` 规范化为 `/tmp/...`，同一批源文件以两种路径出现，会话扫描 `scan_failed`，summary 一直为 `summary_unavailable`。改用 `/tmp` 拼写重新播种后恢复正常。
- 清理：三张测试卡已从桌面移除；候选包已注销并删除，私有 App Group 容器、fixture、截图和日志已删除，隔离宿主与扩展进程已结束。上一轮 `AgentDeckWidgetTest-Du4Zc6` 遗留的 SDK 测试配置在本机已不可查询（该扩展早已注销），内置屏桌面上也看不到，其余显示器未检查。
- CEv1：为 `atomic-recovery`、`native-display`、`timeline-actions-budget` 各新增一条绑定当前候选 ContentState 的证据。`atomic-recovery` 另 rolls_up 到既有的 L3 跨进程证据，`native-display` 带用户排除范围。节点 3/3、关系 7/7 均已核对，关系预检无冲突。完整门禁 13/13，VERIFIED。

Task checkpoint：候选 `25f664e8bc7d7edfe6f596f865b18e66651c630cff3460751332a3023285c672`，门禁 VERIFIED。
提交建议：按用户授权做本地提交。
推送建议：未授权推送或合并。
