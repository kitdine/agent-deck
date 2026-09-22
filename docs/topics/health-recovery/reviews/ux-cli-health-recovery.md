---
status: active
topic: health-recovery
subject: ux/cli-health-recovery.md
---

# CLI Health Recovery Review

## Round 1 — 2026-09-21

## 📋 health-recovery / ux/cli-health-recovery.md framework 评审

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`docs/topics/health-recovery/ux/cli-health-recovery.md:356`](../ux/cli-health-recovery.md) `CLI-R1-F1` 把诊断命令定义为不持有排他状态锁且不产生 mutation，但没有记录当前 `extension doctor` 的入口违反该契约。
- 行为风险：实现者可能只修改 doctor 的分类和 renderer，却继续通过共享 `withExtensions` 调用 `commandOptions.openStore`；该路径进入 `store.Open`、获取 `state.lock`，并允许创建、迁移及权限修复，因此只读诊断仍会因自身要诊断的锁而失败或改变状态。
- 证据：`cmd/agentdeck/main.go:2883-2950` 将 `extension doctor` 绑定到 `withExtensions`；`cmd/agentdeck/main.go:1775-1780` 的 `openStore` 调用 `store.Open`；`internal/store/store.go:301-335` 获取排他锁并准备可写 SQLite 文件。文档仅在 unresolved questions 中讨论“doctor 检查两个锁”，没有把 extension doctor 的现有冲突列为必须 provision 的数据/入口边界。
💡 有界修复：明确记录此 current-behavior conflict，并要求 architecture 为 `extension doctor` provision 不创建、不迁移、不 chmod、不获取 `state.lock` 的只读库存入口，同时定义 missing state、schema ahead 与 unreadable database 的 UX 结果。

[`docs/topics/health-recovery/ux/cli-health-recovery.md:208`](../ux/cli-health-recovery.md) `CLI-R1-F2` 的 `effect: Updates AgentDeck inventory only` 与复用的现有 `extension scan` 行为不一致。
- 行为风险：用户得到错误的 mutation 边界；库存事务提交后，命令还会写 `watch.fingerprint.extension`。若该后置写入失败或 context 已取消，当前命令可在库存已经提交后返回错误，也与“atomic commit 后报告 committed result”的取消契约冲突。
- 证据：`extension.Scan` 在 `internal/extension/extension.go:88-117` 完成库存替换；随后 `cmd/agentdeck/main.go:2898-2910` 计算 roots fingerprint 并通过 `SetSetting` 写入 core database。相同的 inventory-only 文案在 aggregate doctor specimen 第 291 行再次出现。
💡 有界修复：决定是把用户可见 effect 改为准确涵盖 AgentDeck-owned derived extension state，还是由 architecture 改变 scan 边界；同时明确库存已提交但 fingerprint 后置步骤失败/取消时的 text、JSON、exit status 和 follow-up diagnosis 语义。

[`docs/topics/health-recovery/ux/cli-health-recovery.md:327`](../ux/cli-health-recovery.md) `CLI-R1-F3` 没有冻结脚本依赖的 `reason` 与 `action_kind` 精确 token，却声明产品决策已经完成。
- 行为风险：architecture/implementation 必须自行选择 `live` 还是 `lock_live`、以及带空格的 `synchronize inventory` / `manual prerequisite` 是否就是机器 token；文本、JSON、doctor check 和 desktop wire 因而可能产生不兼容枚举。
- 证据：JSON specimen 使用 `reason: "lock_live"`，而第 331–332 行只要求 `live, legacy, unknown or another reviewed bounded value` 和自然语言 action 名称；第 408 行又声明 observable contract 不再留 UX 选择。
💡 有界修复：列出 resource、lock reason、extension reason、action kind 的完整精确 ASCII token 集及 unknown/default 行为，并让全部 text/JSON specimens 和 architecture data requirements 引用同一集合。

### 🟡 建议改进 — 推荐

[`docs/topics/health-recovery/ux/cli-health-recovery.md:73`](../ux/cli-health-recovery.md) `CLI-R1-F4` 的 fixed-label 表遗漏了 specimen 使用的 `manual prerequisite:`。
- 行为风险：renderer、golden tests 与 screen-reader 顺序没有权威定义该行是独立机器稳定标签，还是 `next:` prose 的一部分。
- 证据：固定标签表只列 `next:`、`diagnose:`、`recovery:`、`effect:`，但第 145 行引入第五种 label。
💡 有界修复：把该 label 加入固定语义与信息层级，或合并到既有 `next:`；随后统一 legacy/unknown specimens 和验证 oracle。

[`docs/topics/health-recovery/ux/cli-health-recovery.md:58`](../ux/cli-health-recovery.md) `CLI-R1-F5` 把外部 discovery diagnostics 的 control-character sanitization 描述成现有策略，但当前 extension doctor renderer 不经过 sanitizer。
- 行为风险：架构与任务分解可能把必要的终端注入防护误判为已存在，导致换行、ANSI/OSC 或控制字符破坏行层级和复制语义。
- 证据：`internal/extension/extension.go:153-155` 保留 discovery error/diagnostics；`cmd/agentdeck/main.go:4506-4511` 通过 `textList` 输出；`cmd/agentdeck/main.go:5067-5072` 的 `textList` 只是 `strings.Join`。`internal/output/table.go:57-89` 的 sanitizer 仅服务 table cells，不覆盖该路径。
💡 有界修复：把这句话改为待实现的显式安全 contract，并在 verification design 中加入 extension doctor text/error 的 ANSI、OSC、C0/C1、换行和 invalid UTF-8 fixtures；JSON 保留合法转义且不丢稳定分类。

### 🟢 优点

- 正确选择非交互 line mode，并完整覆盖 TTY、non-TTY、窄宽、重定向、`TERM=dumb`、`NO_COLOR` 与 JSON 分界。
- 锁 live/legacy/unknown/modern-stale 状态和扩展 discovery/stale/drift/duplicate 状态的用户路径清晰，避免以通用 scan 或删除命令掩盖原因。
- stdout/stderr、退出码、取消、无 raw mode、无隐藏 polling 的约束适合脚本化恢复，也为 PTY 与纯状态测试提供了清晰 oracle。
- verification matrix 覆盖危险删除、schema 优先级、库存原子性、窄宽、控制序列和取消清理，框架主体可在修复五项契约缺口后复用。

### 📝 总结

- Reviewed state：HEAD `7537f52d0433acf78c922954400ed0470485a69b`；`docs/topics/health-recovery/ux/cli-health-recovery.md` blob `6141e8d214ea0c00ecc5340f925336538637bc3e`；内容状态 `urn:ce:agent-deck:content-state:health-recovery:ux-cli-health-recovery:7537f52:e90101353a8a35c850a34f50a6067c925ee9837df012a9eb2b1530360c4c1217`。
- Reviewer：Codex，正式 REVIEW 角色；Method：设计/契约评审，使用 `designing-terminal-experiences` 的 line-mode、terminal matrix、fallback、accessibility、input lifecycle 与 verification 维度，并以 CodeGraph 定位后聚焦核对当前 CLI/source contracts。
- Scope：CLI framework 文档及其引用的 reviewed requirements；源码仅用于验证现状前提和隐藏耦合。未评审 menubar UX、Go 类型、最终 architecture 或实现任务。
- Findings：`CLI-R1-F1` 至 `CLI-R1-F5` 均为目标文档内的开放 finding；没有外部 carrier，也没有可在 FAIL 下延后的项。
- Evidence：实时 Beads 归属为 `ad-hr-doc-ux-cli-health-recovery-design`；候选 HEAD/blob/digest 与既有 CEv1 ContentState 一致。决定性源码证据来自 `cmd/agentdeck/main.go`、`internal/store/store.go`、`internal/extension/extension.go`、`internal/output/output.go` 与 `internal/output/table.go`。设计 handoff 已在同一候选状态记录 `make check-whitespace` 和 `git diff --check` PASS，本轮在决定性 finding 后未重复宽泛验证。
- Completion gate：FAILED — formal review criterion 被本轮五项开放 finding 直接否证；L0 通过不覆盖 UX/contract failure。
- Residual uncertainty：未运行实现测试或真实 PTY，因为本轮对象是未实现 framework，且现有源码已经提供决定性反证；后续修复只需更新该文档及 topic-local handoff。

## Round 2 — 2026-09-21

## 📋 health-recovery / ux/cli-health-recovery.md framework 复评

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `CLI-R1-F1` 已关闭：文档明确把当前 `extension doctor` 经 `withExtensions` 和可写 `store.Open` 的冲突列为架构必须解决的前提，并冻结了不获取 `state.lock`、不创建、不迁移、不启用 WAL、不 chmod、不写 setting 的只读入口及四类可观察结果。
- `CLI-R1-F2` 已关闭：`extension scan` 的 effect 现在准确覆盖库存和 `watch.fingerprint.extension` 两项 AgentDeck-owned derived state，并定义了库存提交后 fingerprint 更新失败或取消的 `extension_sync_incomplete`、`inventory_committed: true`、非零退出和后续诊断语义。
- `CLI-R1-F3` 已关闭：schema version 1 的 `resource`、lock/extension `reason`、`action_kind` 完整 token 集和未知值 fail-closed 行为已经冻结，text/JSON specimen 与架构数据要求引用同一词汇。
- `CLI-R1-F4` 已关闭：`manual prerequisite:` 已进入固定标签表，明确为外部确认或人工步骤的 prose，不能被解析成可执行恢复命令。
- `CLI-R1-F5` 已关闭：单行终端消毒被明确为待实现安全契约，并覆盖 ANSI CSI、OSC、C0/C1、CR/LF/tab、DEL、无效 UTF-8 及 JSON 合法转义 fixtures。
- 框架仍保持非交互、无危险锁删除、原因特定动作、稳定 stdout/stderr 与退出码边界；新增契约没有越过 architecture 的类型和实现责任。

### 📝 总结

- Finding disposition：`CLI-R1-F1`、`CLI-R1-F2`、`CLI-R1-F3`、`CLI-R1-F4`、`CLI-R1-F5` 全部 closed；无 still-open、regressed、superseded 或新增 finding。
- Reviewed state：HEAD `7537f52d0433acf78c922954400ed0470485a69b`；`docs/topics/health-recovery/ux/cli-health-recovery.md` blob `1b94d9e25de806b3b68d645216aaba28d37c210b`；内容状态 `urn:ce:agent-deck:content-state:health-recovery:ux-cli-health-recovery:7537f52:1d95ea77b6fff6ac58ad65424f1b546a4d9f262e7bd70a7e53978fc49a2fa7de`。
- Reviewer：Codex，正式 REREVIEW 角色；Method：逐项 finding disposition、全文契约一致性检查，并用 workspace-bound CodeGraph 复核当前 `extension doctor`、`extension scan`、`store.Open`、`ReplaceExtensions`、`SetSetting` 与 `textList` 路径。
- Scope：CLI framework 文档、Round 1 五项 finding 及其当前源码前提；未评审 menubar UX、Go 实现、最终 architecture 或后续 final-surface reconciliation。
- Evidence：当前源码仍证明 `extension doctor` 复用可写入口、库存事务先于 fingerprint setting 写入、`textList` 未执行终端消毒；修复后的文档将这些事实转换为明确的新入口、失败边界、机器词汇和 fixtures。`make check-whitespace` 与 `git diff --check` 对最终复评记录状态通过。
- Completion gate：VERIFIED — 当前内容状态的三个 required criteria（L0、UX framework、formal review）均有直接 PASS 证据，且 Round 1 FAIL 仅绑定旧内容状态。
- Residual uncertainty：本轮只批准 framework 的可实现性与内部一致性；architecture 仍须 provision 每个请求字段，之后 final CLI reconciliation 必须评估是否需要新一轮材料变更复评。该后续边界不构成本轮开放 finding。
