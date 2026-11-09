---
status: active
topic: installed-state-ownership
subject: installation-diagnostics
---

# Installation Diagnostics — Review

## Round 1 — 2026-11-08

## 📋 Task 4 独立评审

📊 综合评分：6/10

✅ 评审结论：FAIL

- Reviewer：Codex，独立评审角色；未参与本候选的开发或修复。
- Method：核对批准契约、生产调用路径、既有测试和逐路径 blob；使用临时 Go overlay 在隔离 HOME/账户/状态目录中运行实际 CLI 恢复链路。未修改生产代码、仓库测试或配置，未委派。
- Scope：`installation-diagnostics` 的受管命令枚举、doctor 归属诊断及文本/JSON、隐藏 `desktop state-location`，以及它们调用的守卫和恢复入口。
- Reviewed state：HEAD `01e955f97a3a18ff566a4c81109856d926d75d59`；入场候选 SHA-256 `db16b5f4a0d0fa7d8d2622f78996a09284b05f5ac875a6332bfceb7d8e8788bb`；状态同步后 SHA-256 `b3b0afc4803dd3712b08a65c924d1def333d7e506c6ddd4e31cdd2a5ccdc1617`。
- WorkUnit：`installed-state-ownership:installation-diagnostics`。
- ContentState：`installed-state-ownership:installation-diagnostics:review-r1-20261108-KnETkXaA`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

**ISO-DIAG-R1-F1 · P2 · FIXED（待复评，见 Repair R1）— 诊断给出的 Hook 恢复命令不会清除它检测到的旧绝对路径命令**

位置：[managed_command.go:67](../../../../internal/doctor/managed_command.go#L67)，以及恢复消费者 [config.go:575](../../../../internal/usagehook/config.go#L575)。

- 行为风险：旧绝对路径 Hook 被报告为 `managed_command_owner_mismatch` 后，用户执行 `agentdeck usage hook setup --client claude`，setup 报告 `configured`，旧命令却仍然留在配置中。下一次 doctor 再次报告相同警告和相同恢复命令，用户无法通过给出的恢复动作完成清理。旧命令和新登记命令并存。
- 契约依据：[CLI 的 managed_command_owner 检查](../ux/cli.md#managed_command_owner-检查)要求提供对应的重新登记命令；[架构的只读和 Hook 路径](../architecture.md#只读和-hook-路径)将旧拥有者的绝对路径登记作为该诊断的主要场景。
- 源码证据：新增 `parseManagedCommand` 接受绝对路径；`managedCommandOwnerCheck` 在检测到它指向其他程序时建议 setup。实际恢复入口 `runUsageHookLifecycle` 调用 `usagehook.Manager.Setup`；它的 `managedHookCommand` 只接受 `agentdeck` 或 `agentdeck --state-dir ...` 前缀。旧绝对路径条目因此被当作非受管条目保留，`updateEvent` 只追加新的 PATH 命令。
- 动态证据：隔离测试向临时 Claude 配置的 `SessionStart` 写入 `/retired/agentdeck usage hook event claude`，通过真实 `run` 入口先执行 doctor，再解析并执行其返回的恢复命令，最后再次执行 doctor。结果为 `before=managed_command_owner_mismatch; setup=configured; after=managed_command_owner_mismatch; old_route_retained=true`。断言恢复后警告消失失败，测试耗时 0.03 秒，包结果 FAIL（1.339 秒）。
- 现有覆盖缺口：`TestManagedCommandOwnerCases` 只断言恢复字符串，固定标本测试只验证写出器；均未执行该恢复动作并复查诊断。

💡 修复建议：让诊断与其恢复消费者对受管绝对路径 Hook 使用一致的识别和安全迁移规则，使建议命令能够替换或移除已确认的旧受管路由，同时保留无关 Hook。补充执行恢复命令后重新诊断的回归，覆盖 Claude、Codex、带引号路径及重复执行。此发现属于 Task 4 的诊断/恢复集成，不要求执行任务 5 尚未实现的安装命令。

### 🟡 改进建议 — 推荐

无额外发现。

### 🟢 做得好的地方

- 受管命令结果不包含可执行路径；枚举只读，不执行被检查的命令。
- 状态拒绝和目录缺失在新检查之前短路。隐藏路径查询的回退继续调用 `StateRootWithHome` 和 `GuardStateRoot`，没有把未经守卫的路径直接返回。
- 新增命令已登记进叶子命令矩阵，已有测试明确检查未安装身份下真实路径不出现在输出中。

### 📝 总结

本轮 FAIL 的阻断项为 ISO-DIAG-R1-F1。实际 CLI 复现证明新增诊断的恢复建议无法完成它承诺的旧路由处理。任务 Review 保持未勾选，Dev 历史交接保留；不提交、不推送，也不宣称主题完成。

**Evidence：**

| 检查 | 结果与边界 |
| --- | --- |
| 候选身份核对 | 入场 HEAD 与开发 ContentState 一致，20 个 subject blob 全部一致。评审仅更改任务状态摘要并新增本记录；最终 subject 清单只有 tasks.md 的 blob 变化。 |
| 开发态标准 CEv1 查询 | 目标 `installed-state-ownership:installation-diagnostics:dev-20261108-XAHWvdUO` 为 NOT_VERIFIED，4/5 required criteria 有通过证据，仅缺 independent-review，invalidated/unresolved 均为空。此结果是开发态事实，不是本轮 PASS。 |
| `scripts/run-go-test.sh -overlay=<新建隔离 overlay.json> ./cmd/agentdeck -run '^TestInstallationDiagnosticsReviewRecovery$' -timeout 90s` | 使用 `GO_TEST_BIN` 指定已安装的 Go 1.27.3，通过临时测试调用真实 doctor/setup/doctor 链路；FAIL，观察值见 F1。overlay 及日志均为一次性诊断，必要事实已直接保存在本记录，不依赖它们留存。 |
| 编译环境校正 | 默认 `go` 是 1.27.1，但继承的 GOROOT/编译器为 1.27.3，首次构建报 `compile: version "go1.27.3" does not match go tool version "go1.27.1"`，未运行测试。随后仅为该命令设置 GO_TEST_BIN 为 GOROOT 内的 1.27.3；未改全局配置。有效复现使用 Go 1.27.3 darwin/amd64，与开发证据声明相同。 |
| 验证范围 | 获得决定性阻断复现后停止扩大验证；没有重跑全量 Go/race/访问屏障。开发态通过证据保留原状态，watch 超时的历史归因未由本轮独立重现或认证。 |
| 文档收口 | 本记录的 review-records 检查、check-docs、installed-state-ownership 的 check-topic-docs、check-whitespace 与 git diff --check 均通过。它们只验证文档结构与格式，不改变 FAIL 或证据门禁。 |

**内容身份说明：** 本轮沿用文档声明的配方：UTF-8 `head=<HEAD>\n`，再按路径排序连接 `<path>=<git-blob>\n`，换行为真实 LF，末尾保留 LF。开发 ContentState 保存的聚合值 `a023aadfa190ccb052284717cbb9e01f33fb5c04c1b206eff0398d70912a1145` 无法按该配方复算；逐文件 blob 和 HEAD 已全部匹配，因此本轮保存新的可复算 ContentState，不重写旧观测，也不单凭旧聚合值宣称复用。该元数据澄清不替代产品发现。

**状态同步后的 20 个 subject：**

| 路径 | Git blob |
| --- | --- |
| `cmd/agentdeck/desktop.go` | `ea0fc5d5d491c8927be858c03cee8bc06f1166d9` |
| `cmd/agentdeck/desktop_state_location.go` | `f939d9465bd088650f975e8afb71c8c0139323f7` |
| `cmd/agentdeck/desktop_state_location_test.go` | `ff519a95bc6624932a9ef754450816df208c048e` |
| `cmd/agentdeck/ownership.go` | `5bab6c0648f7ed5e4659ea9c5c2fd294a8dd8e6c` |
| `cmd/agentdeck/ownership_doctor_render_test.go` | `624856006bc840c76367ec10b63839f2430e0e4c` |
| `cmd/agentdeck/ownership_guard_acceptance_test.go` | `5da47f8284753515bf837d9b52edc7bef44d495d` |
| `cmd/agentdeck/testdata/ownership-golden.json` | `5237f90de85eb3dcef328486e11a290a71b84c8e` |
| `cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json` | `d8d3d776c60a4de50c9010cc2ee9506fdcb7573a` |
| `docs/topics/installed-state-ownership/architecture.md` | `9a5e5a349489c7dacbeefb68ee327c19e7ba7861` |
| `docs/topics/installed-state-ownership/requirements.md` | `2c4f3e37b8452ab2483908a98cc8be2a08366cf0` |
| `docs/topics/installed-state-ownership/tasks.md` | `0f863d3797d68de5f70fa3ca1ee4b4def06c645d` |
| `docs/topics/installed-state-ownership/ux/cli.md` | `5e427a1189ef21c7d1e5b150e27ab04f6221f20a` |
| `internal/doctor/doctor.go` | `735950cb27ecd5e4537a915f85d7f7e80e48c19c` |
| `internal/doctor/installation.go` | `07daed6829c47847f2f4942842c90df9ae9a8c40` |
| `internal/doctor/managed_command.go` | `6440e9311567fa791ce98100a4154fc32c08860f` |
| `internal/doctor/managed_command_test.go` | `ef0c54e2a557da5f9a94f30230edc4e6ccd03f80` |
| `internal/usagehook/managed.go` | `cb5fccfd161fde97a1da870f8120dbdb26b96c0f` |
| `internal/usagehook/managed_test.go` | `085b0a9fc4bab5803c4fed13ce60e9d7128829c1` |
| `prototype/src/ownership.js` | `b0d100100fc4caf9bf56dda156e6edd641d86fa4` |
| `scripts/run-go-test.sh` | `f13ad1e2533b82eee4c574cffcf97d0917ae5939` |

本记录自身不属于 subject 清单，以免形成自引用。它记录的事实绑定上表中的生产、测试、契约和任务状态。本轮失败证据绑定最终 ContentState；其他开发观测保留原始状态，不将未重新评估的通过结果自动转写。3 个节点及 5 条预检通过的关系均已逐项读回，数量和属性一致。标准门禁查询确认 managed-command-owner 与 independent-review 两项具有适用于最终状态的失败证据；invalidated_evidence 仅包含开发态 managed-command-owner 观测，unresolved_candidate_impacts 为空。新失败证据 supersedes 该开发态通过观测，保留原测试事实，禁止将其继续当作完整恢复契约的通过证据复用。

## Repair R1 — 2026-11-09

- 修复者：Claude Code，开发角色（非独立评审）。
- 处理发现：ISO-DIAG-R1-F1 → FIXED（待复评）。

**ISO-DIAG-R1-F1 修复：**

- 恢复消费者：[config.go](../../../../internal/usagehook/config.go) 新增 `retiredHookRoute`。`updateEvent`（setup 与 remove 共用）遇到旧拥有者写入的 AgentDeck Hook 时会移除它，setup 随后写入当前的 PATH 形式命令。旧 Hook 的识别规则与诊断枚举相同，都调用 `parseManagedCommand`：命令是绝对路径的 `agentdeck`，可以带引号或 `--state-dir`，并以 `usage hook event <client>` 结尾。除 `command` 外，条目字段必须与生成形状一致；多出字段的条目仍视为用户数据保留。无关 Hook 不受影响。`managedHookCommand` 与 Hook 状态统计保持不变，状态语义没有扩大。
- 诊断一致性：[managed.go](../../../../internal/usagehook/managed.go) 只把 `managedStatusLineCommand` 能识别的 statusline 报为受管命令，也就是 `quota-statusline enable` 实际能替换的形式。不加引号的绝对路径 statusline 不再被报告；这种形式没有恢复手段可以清除。
- 回归：`TestManagedCommandOwnerHookRecoveryClearsMismatch` 覆盖 Claude、Codex、两者都不匹配且路径带引号三种情况，依次执行 doctor、恢复命令里指定客户端的 `Setup`、再次 doctor，并重复执行一次。断言：恢复后检查为 ok，配置中不再出现旧路径，无关 Hook 恰好保留一份。

### 下一步指令

复评：installed-state-ownership / installation-diagnostics

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Round 2 — 2026-11-09

## 📋 Task 4 修复候选独立复评

📊 综合评分：7/10

✅ 复评结论：FAIL

- Reviewer：Codex，独立复评角色；未参与 Repair R1 的生产代码或测试修改。
- Method：复用已加载契约与 Round 1 证据，检查三个变更文件及 setup/remove 的变更判定；运行修复者新增的定向回归和隔离实际 CLI 探针。生产代码、仓库测试和配置保持只读，未委派。
- Scope：ISO-DIAG-R1-F1 及其修复涉及的旧绝对路径 Hook 清理、重复 setup、remove 和无关 Hook 保留；不扩大到安装协议或 App。
- Reviewed state：HEAD `01e955f97a3a18ff566a4c81109856d926d75d59`；入场 21-subject SHA-256 `6405f771828efedd83571f198984ea043555dc76cfba9ced18bb08f3967b4d1a`；状态同步后 SHA-256 `5d1084a725cf2bc7a078ffd9b8ecf13b53e814eb1d9dd78a6e89e7d6089ecf8e`。
- WorkUnit：`installed-state-ownership:installation-diagnostics`。
- ContentState：`installed-state-ownership:installation-diagnostics:rereview-r2-20261109-5tY6mqzu`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

**ISO-DIAG-R1-F1 · P2 · STILL_OPEN（Repair R2 已修复，待复评）— 新旧 Hook 并存时，恢复仍提前返回而不清理旧命令**

位置：[config.go:338](../../../../internal/usagehook/config.go#L338)，以及同源的删除持久化判定 [config.go:396](../../../../internal/usagehook/config.go#L396)。

- 处置：原发现部分修复但仍未关闭。Repair R1 的 FIXED 是开发交接，独立复评只确认“只有旧 Hook”分支已恢复；新旧命令并存的有效配置仍复现原恢复失败。
- 行为风险：Round 1 中用户执行原恢复命令后，恰好会留下“旧绝对路径命令 + 当前 PATH 命令”的配置。修复后再次执行 doctor 建议的 setup，结果为 `unchanged/configured`，旧路由保留，doctor 继续报告 `managed_command_owner_mismatch`。原故障已经影响的用户仍无法完成恢复。
- 源码证据：`prepareSetup` 仍只根据 `analysis.exact != 1` 设置 `needsChange`；所有标准事件各有一条当前命令时，会在 342–343 行直接返回。新加的 `retiredHookRoute` 只在后续 `updateEvent` 内运行，所以清理分支根本不会被调用。
- 同源的受影响路径：`prepareRemove` 虽然调用 `updateEvent` 并在内存中删去旧命令，`removed` 却仍只由当前 PATH 命令的 `analysis.exact > 0` 决定。仅有旧绝对路径命令时，404–405 行返回 `absent`，丢弃已计算的删除结果。此观察说明相同的变更判定遗漏，不另立发现或扩大任务边界。
- 动态证据：实际 `run` 入口的隔离探针先用 setup 生成完整当前配置，再在 `SessionStart` 追加带引号的旧绝对路径 Hook 和 `printf keep`。doctor 返回恢复命令后，探针执行该命令并再次 doctor。结果为 `setup=unchanged; old_route_retained=true; after=managed_command_owner_mismatch`。仅旧路由的 remove 则为 `remove=absent; old_route_retained=true; after=managed_command_owner_mismatch`。两种情况的无关 Hook 都恰好保留一份。
- 覆盖边界：新增 `TestManagedCommandOwnerHookRecoveryClearsMismatch` 的三个子例均通过，但初始配置均只有旧路由。重复执行发生在第一次成功清理之后，不能覆盖“第一次恢复前已经有新旧路由”的情况。

💡 修复建议：把旧受管路由的清理需求纳入 setup 的变更判定，并让共享删除路径的实际变化参与 remove 的持久化判定。补齐完整当前配置与旧路由并存、只有旧路由的 remove、重复调用及无关 Hook 保留的回归；保持受管条目边界，不扩大对用户自定义条目的删除权限。

### 🟡 改进建议 — 推荐

无额外独立发现。

### 🟢 做得好的地方

- 仅旧路由的实际 CLI 控制场景现已通过：setup 返回 `configured`，旧命令移除，doctor 不再报告归属不匹配。
- 修复者新增的 Claude、Codex 和双客户端带引号路径三个回归均通过。
- 本轮三个实际 CLI 场景均保留且未重复无关 Hook；清理识别也保留了“额外字段视作用户数据”的边界。

### 📝 总结

发现处置：ISO-DIAG-R1-F1 为 STILL_OPEN；无已关闭发现、无单独新增发现。修复覆盖了原始简单输入，但没有覆盖原故障运行后留下的配置。任务 Review 继续未勾选，待有界修复；不提交、不推送，不完成主题或启动下一任务。

**Evidence：**

| 检查 | 结果与边界 |
| --- | --- |
| 修复候选比较 | HEAD 与 Round 1 相同。生产/测试变化仅为 `internal/usagehook/config.go`、`internal/usagehook/managed.go` 和 `internal/doctor/managed_command_test.go`；本轮仅改任务状态摘要并追加复评记录。 |
| 修复者新增回归 | 通过必用 runner，在 Go 1.27.3 darwin/amd64、vendor 模式下运行 `TestManagedCommandOwnerHookRecoveryClearsMismatch`：claude、codex、both-quoted 均 PASS，测试 0.13 秒，doctor 包 2.559 秒。 |
| 实际 CLI 定向探针 | `scripts/run-go-test.sh -overlay=<新建隔离 overlay.json> ./cmd/agentdeck -run '^TestInstallationDiagnosticsRereviewRecovery$' -timeout 90s`，通过 GO_TEST_BIN 指定同一 Go 1.27.3。legacy-only PASS（0.05 秒）；coexisting-canonical FAIL（0.03 秒）；remove-legacy-only FAIL（0.00 秒）；总测试 0.08 秒，cmd 包 FAIL（1.059 秒）。 |
| 探针隔离和有效性 | 使用仓库 TestMain 的临时账户、每例临时 HOME/状态目录及临时 Go overlay，没有改仓库测试或真实配置。探针按实际 `data.clients` 字段解析生命周期结果；首次错误使用 `data.results` 的未完成观察不作为产品结论，仅采用校正后的完整运行。必要输入、输出和根因均保存在本记录，不依赖临时日志。 |
| CEv1 | 入场修复状态登记并查询为 NOT_VERIFIED；没有发现修复者登记的精确新状态验证证据。最终新增 3 节点和 4 条预检通过关系均读回一致；标准门禁为 FAILED，managed-command-owner 与 independent-review 各有最终目标上的失败证据。历史开发态 managed-command-owner 通过观测仍因 Round 1 supersession 不可复用；unresolved_candidate_impacts 为空。 |
| 验证限制 | 取得决定性失败复现后停止扩大验证，未补跑全量 Go、race 或访问屏障。历史宽范围通过观测保留原状态；本轮不把它们直接重定向为修复态通过。 |
| 文档收口 | 本记录的 review-records、check-docs、installed-state-ownership 的 check-topic-docs、check-whitespace 与 git diff --check 通过；仅验证结构和格式，不改变复评或门禁结论。 |

**内容身份：** 沿用 Round 1 的真实 LF/末尾 LF 指纹配方。候选增加修复实际改变的 `internal/usagehook/config.go`，因此 subject 从 20 个扩展为 21 个。除下列替换/新增条目外，全部沿用 Round 1 表中的相同 blob；完整 21 项也保存在最终 CEv1 ContentState。评审记录自身继续排除，避免自引用。

| 替换或新增路径 | Git blob |
| --- | --- |
| `docs/topics/installed-state-ownership/tasks.md` | `59f9f874bfd0edea3fccd266bdc4194e6034c3e8` |
| `internal/doctor/managed_command_test.go` | `26dbf1f85fb215d222d6fbbc2ef43e85162b28aa` |
| `internal/usagehook/config.go` | `0419982f7b90ca58f8fa293e17f8c1415c5c037a` |
| `internal/usagehook/managed.go` | `93a45ea1ae39f72d897ff3545d4e533372d14481` |

### 下一步指令

修复：installed-state-ownership / installation-diagnostics / ISO-DIAG-R1-F1

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Repair R2 — 2026-11-09

- 修复者：Claude Code，开发角色（非独立评审）。
- 处理发现：ISO-DIAG-R1-F1 → FIXED（待复评）。Round 2 指出的 setup 提前返回与 remove 丢弃删除结果两条路径均已处理。
- 候选：HEAD `01e955f97a3a18ff566a4c81109856d926d75d59`；相对 Round 2 只改两个文件，`internal/usagehook/config.go` blob `676e14124a513aebd4740cc2d52b4cb84110781c`，`internal/doctor/managed_command_test.go` blob `66d1436ef7dc191b524866044358c8d6e1a519fc`。

**ISO-DIAG-R1-F1 修复：**

- 变更判定：[config.go](../../../../internal/usagehook/config.go) 的 `eventAnalysis` 新增 `retired` 计数。`analyzeEvent` 先用 `retiredHookRoute` 识别旧拥有者路由并计数，与 `updateEvent` 的判定顺序一致。`prepareSetup` 在 `exact != 1 || retired > 0` 时进入写入路径，因此新旧路由并存时不再提前返回 `unchanged`。`prepareRemove` 在 `exact > 0 || retired > 0` 时持久化，因此只有旧路由时不再丢弃已计算的删除并返回 `absent`。
- 边界保持：`retired` 只参与 setup/remove 的变更判定。Hook 状态统计（同样调用 `analyzeEvent`）只读 `exact` 与 `modified`，语义不变。旧路由识别规则没有改动，所以带额外字段的条目仍然算用户数据，不会被删。
- 回归：`TestManagedCommandOwnerHookRecoveryClearsCoexistingAndRemovedRoutes` 覆盖 Claude 和 Codex，每个客户端分别测不带引号与带引号路径，共 8 个子例。
  - `setup-coexisting-*`：先用 setup 写出完整的当前配置，再在 `SessionStart` 追加旧绝对路径 Hook 与 `printf keep`。确认 doctor 报告 `managed_command_owner_mismatch`，且恢复命令为 `agentdeck usage hook setup --client <client>`。之后连续 setup 两次，结果依次为 `configured`、`unchanged`。每次之后 doctor 为 ok，配置中没有旧路径，`printf keep` 恰好一份。
  - `remove-legacy-only-*`：配置只含旧路由与无关 Hook。连续 remove 两次，结果依次为 `removed`、`absent`。每次之后 doctor 为 ok，旧路径已清除，无关 Hook 恰好一份。

**Evidence：**

| 检查 | 结果与边界 |
| --- | --- |
| 修复前红灯 | 用 Go overlay 把 `config.go` 替换为撤回本轮四处改动后的版本（blob `e85e8080bfc1b00e2f092300cc8871ea3be431aa`，即 Round 2 评审的变更判定逻辑），通过 `scripts/run-go-test.sh` 运行 `TestManagedCommandOwnerHookRecovery`。新测试 8 个子例全部 FAIL：setup 第一次返回 `unchanged`，期望 `configured`；remove 第一次返回 `absent`，期望 `removed`。与 Round 2 实际 CLI 探针观察一致。R1 的三个子例仍 PASS。 |
| 修复后定向 | `scripts/run-go-test.sh ./internal/doctor/ -run 'TestManagedCommandOwner'`：新测试 8 个子例、R1 回归 3 个子例及既有 7 种归属情况全部 PASS（doctor 包 1.579 秒）。 |
| 受影响包 | `scripts/run-go-test.sh ./internal/usagehook/ ./internal/doctor/` 全部通过（usagehook 4.292 秒，doctor 8.299 秒）。`go vet -mod=vendor` 对两个包通过，`gofmt -l` 无输出。 |
| CLI 入口 | `scripts/run-go-test.sh ./cmd/agentdeck/ -run '(?i)(usagehook\|usage_hook\|UsageHook\|Hook\|Doctor\|Ownership)'`：17 个顶层测试通过（26.314 秒）。 |
| 工具链 | Go 1.27.3 darwin/amd64，vendor 模式；测试 HOME 与配置均为每例临时目录，未接触真实客户端配置。 |
| 未执行 | 未重跑 `./...` 全量、race 或 `agentdeck_acceptance` 叶子矩阵；本轮只改 `internal/usagehook` 的变更判定和一项测试。CEv1：修复不跨越 WorkUnit 边界，本轮未写入新状态或证据，门禁结论留给复评。 |

### 下一步指令

复评：installed-state-ownership / installation-diagnostics

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Round 3 — 2026-11-09

## 📋 Task 4 Repair R2 独立复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：Codex，独立复评角色。
- Method：逐项核对 Round 2 发现、Repair R2 差异、配置写入调用链和新增回归，使用隔离实际 CLI 重放失败场景，并完成当前候选的 Go 全量回归。
- Scope：ISO-DIAG-R1-F1 的 setup/remove 变更判定、旧路由迁移、幂等性、客户端配置保留，以及 Task 4 的必需证据。
- Reviewed state：HEAD `01e955f97a3a18ff566a4c81109856d926d75d59`，入场 SHA-256 `7d07802462ea68d7f26311760e1e18184a82f76ecdfbcc83ef16ac50bd39e2b9`，最终 21-subject SHA-256 `cdf0bfe945a55d8c1794326931575d9be6b1d2851f4edc9f130a5aa33dee099b`。
- WorkUnit：`installed-state-ownership:installation-diagnostics`。
- ContentState：`installed-state-ownership:installation-diagnostics:rereview-r3-20261109-NJQZou9y`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。ISO-DIAG-R1-F1 已关闭。

### 🟡 改进建议 — 推荐

无。

### 🟢 做得好的地方

**ISO-DIAG-R1-F1 · CLOSED**

`analyzeEvent` 使用与 `updateEvent` 相同的 `retiredHookRoute` 判定并计数。`prepareSetup` 在存在旧路由时进入更新，`prepareRemove` 将旧路由清理计入持久化条件。Round 2 的两条失败分支均得到实际 CLI 验证。[setup 变更判定](../../../../internal/usagehook/config.go#L338) · [remove 持久化判定](../../../../internal/usagehook/config.go#L394) · [旧路由计数](../../../../internal/usagehook/config.go#L422)

- Claude 和 Codex 均覆盖仅旧路由、新旧路由并存和仅旧路由 remove，共 6 个实际 CLI 场景。
- 路径包含空格、单引号和显式 `--state-dir`。每个场景连续调用两次，setup 依次返回 `configured`、`unchanged`，remove 依次返回 `removed`、`absent`。
- 每次调用后 doctor 均消除 `managed_command_owner_mismatch`，`printf keep` 恰好保留一份。
- 受管条目的生成形状与额外字段保护边界继续由共享判定控制，Hook 状态统计继续消费 `exact` 和 `modified`。

### 📝 总结

发现处置为 ISO-DIAG-R1-F1 CLOSED，新增发现为无。Task 4 的 Review 已勾选。Topic 尚有任务 5–9，下一实现任务是 `installer-receipts`。真实安装协议与 App 组合验收按任务分解由后续任务承担。

**Evidence**

| 检查 | 结果与适用范围 |
| --- | --- |
| 候选绑定 | HEAD 与 Round 2 相同。Repair R2 的两个 Go blob 与交接记录完全匹配。最终同步相对入场候选只改变 tasks.md 的状态摘要、Review 单元格和更新日期。 |
| 实际 CLI | `GO_TEST_BIN=<Go 1.27.3>/bin/go scripts/run-go-test.sh -overlay=<新建隔离 overlay.json> ./cmd/agentdeck -run '^TestInstallationDiagnosticsRound3CLIRecovery$' -timeout 90s` 通过。6 个场景、12 次恢复/删除观察，测试 0.22 秒，cmd 包 1.572 秒。使用临时账户、HOME、配置和状态目录。 |
| L2 全量 | `GO_TEST_BIN=<Go 1.27.3>/bin/go scripts/run-go-test.sh ./...` 退出 0。cmd/agentdeck 为 187.057 秒。新增 8 子例回归、固定 doctor 标本、state-location 单元测试、合成 snapshot 契约均通过。 |
| 修复材料复用 | Repair R2 的受影响包测试、两个包的 vet 与格式结果绑定两个已核实 blob，Go 1.27.3 darwin/amd64、vendor 模式与本轮一致。共享配置写入代码变化后的全量回归由本轮补齐。 |
| 状态路径验收复用 | 开发态 `state-location` 证据包含安装/未安装身份、真实路径拒绝、访问屏障三分支和叶子清单。相关命令、守卫依赖、测试、工具链和 HEAD 均匹配。两轮修复涉及的 usagehook 枚举与写入路径独立于隐藏路径查询。采用最终目标上的 roll-up 保存复用判断及原观察引用。 |
| Doctor 标本复用 | 原固定标本与部分报告观察保留，相关 prototype、golden、renderer 和短路路径 blob 匹配。本轮全量再次覆盖完整标本渲染与 snapshot 契约，修复的写入消费者在部分报告短路之后发挥作用。 |
| 文档收口 | review-records、check-docs、installed-state-ownership 的 check-topic-docs、check-whitespace 与 git diff --check 均通过。最终门禁与 checkpoint 元数据同步后再次核对记录格式。 |
| CEv1 | 标准门禁 VERIFIED，5/5 required criteria 均有适用于最终目标的通过证据，missing_criteria 与 unresolved_candidate_impacts 为空。6 个最终节点、14 条预检通过关系均精确读回。历史开发态 managed-command-owner 观测仍列入 invalidated_evidence，本轮新目标的通过证据满足该准则。Task 4 已通过 contains 关联到既有 Topic，层级检查无环。 |

临时 overlay 和测试日志用于本轮验证。必要输入、观察和内容身份已记录在此，收口后清理临时材料。

**内容身份**

沿用 Round 2 的 21-subject 清单和真实 LF、末尾 LF 配方。下表替换 Round 2 对应 blob，其余 subject 保持原表值。完整清单保存在最终 CEv1 ContentState。评审记录自身排除以避免自引用。

| 替换路径 | Git blob |
| --- | --- |
| `docs/topics/installed-state-ownership/tasks.md` | `0a9303e3c221f18914a7ffa5574aa625b935a537` |
| `internal/doctor/managed_command_test.go` | `66d1436ef7dc191b524866044358c8d6e1a519fc` |
| `internal/usagehook/config.go` | `676e14124a513aebd4740cc2d52b4cb84110781c` |

Task checkpoint：`ad-installed-state-ownership-installation-diagnostics`，content_state `sha256:cdf0bfe945a55d8c1794326931575d9be6b1d2851f4edc9f130a5aa33dee099b`，gate `VERIFIED`。
提交建议：取得提交授权后，将 Task 4 的 CLI、doctor、usagehook 实现与测试、两个固定数据文件、复评记录及 tasks.md 状态变更作为一个逻辑提交，按实际贡献核验 trailers。
推送建议：核验实际提交树、完整提交消息、贡献者 trailers 和 SSH 签名，取得单独推送授权后推送 `feature/installed-state-ownership` 至 `origin` 同名分支。当前分支尚未设置 upstream。

### 下一步指令

开发：installed-state-ownership / installer-receipts

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership
