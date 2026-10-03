---
status: active
created: 2026-10-03
---

# 缺陷：architecture.md 把独立会话失败归入核心库拒绝的后果

## 现象

Beads `ad-bug-arch-sessions-unavailable-not-a-consequence` 指向
`docs/topics/schema-version-signal/architecture.md` 的
“Why the envelope-level cause is refused” 第三条。原文将
`sessions_unavailable` 与核心库失败时的 provider/usage 警告放进同一
“fixed set”，称抑制它们精确复现生产者行为。独立会话库读取失败因此被错误
解释为 `schema_ahead` 的后果。

## 根因

该段混淆了共同出现与因果关系。`internal/desktop/desktop.go` 的
`Service.Build` 在核心库 `OpenReadOnly` 失败分支产生 provider/usage 警告；
`loadSessions` 在分支外运行。`internal/store/store.go` 的
`OpenSessionsReadOnly` 打开独立 `sessions.sqlite3`，检查两张会话表，
不读取核心库 schema version。会话警告来自它自己的 open/list 失败。

既有权威一致：`requirements.md` 的 Out of scope 明确排除会话库；已评审
`ux/menubar-schema-signal.md` D2 只抑制 provider/usage 两码，保留独立会话
警告。UX-R3-F1 已闭合，本批不重新决定 UX 或契约。

## 修复边界

- 用户预选 Lane A；仅恢复 existing requirements 与已评审 D2。
- 只修正 architecture 第三条的因果论证，说明可读会话仍渲染、独立会话
  open/list 失败仍保留警告。
- 无行为、错误码、wire contract、UX、产品源码或产品测试改动。
- 唯一产品 writer：Codex session `01a10070-f657-7a11-be75-a637319b6927`，
  `gpt-6.1-sol / xhigh`，现有 CPA，`service_tier=default`。
- Workspace `agent-deck.fix.arch-sessions-unavailable-not-a-consequence`；
  branch `fix/arch-sessions-unavailable-not-a-consequence`；base 为已获授权
  fetch 后的 `origin/release/v0.6.x`：
  `4b4b67a6c4b7bc93f3e7d4117c28579c62d7cbf6`。不采用陈旧本地 release ref。
- Carrier 在实际产品 PR merge 前保持 active；交付后通过独立归档 PR 退休。
  其他 Bug、contract/assemble 与 deferred native 图标不属于本批。

## 验证

证据目录：`/tmp/agentdeck-arch-sixth`。本批为 L0 文档修复；针对性运行既有
生产者测试证明契约前提，未创建或重建真实数据库。

1. RED：修改 architecture 前，运行
   `python3 /tmp/agentdeck-arch-sixth/check-causal-rationale.py docs/topics/schema-version-signal/architecture.md`，
   exit 1。原始文档保存在 `architecture-before.md`，来自上述 base commit；
   检查结果保存在 `RED.json`。
2. GREEN：同一检查加 `--sensitivity`，exit 0；验证核心因果段只含 D2 两码、
   会话独立性、可读/不可读会话两个场景。检查器另拒绝三种反例：把会话加入
   抑制集合、耦合核心 schema、隐藏独立会话失败。结果见 `GREEN.json`。
   检查器 SHA-256：
   `a1e32206631d4c8d7375de0c727676e8d40020811668b19c19983f979815cfe7`。
   可对 `architecture-before.md` 重放 RED；不需回滚工作区。
3. `AGENTDECK_GO_TEST_LOG=/tmp/agentdeck-arch-sixth/runtime-independence.log GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./internal/desktop -run '^TestSchemaAheadProducerKeepsIndependentSessionAvailability$'`：
   PASS，覆盖核心 schema ahead 下会话库可读与不可读两种 producer 输出。
4. 最终 L0、cold independent review、exact-head GitHub review/CI 与交付 refs
   由后续实际证据记录；不能由本段预先宣称完成。

CEv1 WorkUnit：`fix:arch-sessions-unavailable-not-a-consequence`，无 containing
topic gate。实际 Neo4j MCP 已成功创建并读回五项 required criteria：contract、
regression、verification、review、scope；entry gate `NOT_VERIFIED`，未豁免。

CodeGraph 在所选 slot entry 时同步失败：`env: node: No such file or directory`。
按 Toolchain 降级为限定文档与生产者源码检查，未使用旧 issue 或其他工作区索引。

## Review — Round 1

**Checklist: 54/54 complete**（含本范围不适用项）
**Incomplete: None**

## 📋 ASU-R1 — 会话失败因果说明独立评审

📊 综合评分：10/10

✅ Verdict: PASS

- **Reviewer:** Codex session `01a10081-99af-7ac1-91b3-5afb00085dc0`
- **Method:** cold independent CLI process/read-only；直接检查源码、契约、差异及捕获证据，独立重放文档检查器。无委派。
- **Scope:** 仅 Bug `ad-bug-arch-sessions-unavailable-not-a-consequence`，用户选定的 Lane A 文档修复；审查架构拒绝 envelope-level cause 的第三条理由及其 active carrier。
- **Reviewed state:** 下列身份均已核对一致：

| 项目 | 内容身份 |
|---|---|
| Base / HEAD | `4b4b67a6c4b7bc93f3e7d4117c28579c62d7cbf6` |
| Architecture blob | `fd7d8067d44963a9c2714356f7e60e9549a6a520` |
| Active carrier blob | `14273a5633f7e01c3f8588654867f0cfcd48a403` |
| Candidate fingerprint | `a9a9ba49be89488ae76efcb20444cb83781e009f413317b7e7d95701b33b933c` |

### 🔴 严重问题 — 必须修复

无。

**Findings:** 无。本轮未生成 `ASU-R1-Fn` 发现；无待处理、延期或转移的发现，Disposition 不适用。

### 🟡 建议改进 — 推荐

无。本轮没有可行动的范围内发现。

### 🟢 优点

- 修复恢复了既有 requirements 的会话库独立性。[requirements.md](../topics/schema-version-signal/requirements.md#L246) 明确将 `sessions.sqlite3` 排除于核心库 schema 问题之外，并承认可读会话仍能呈现。
- 修复与既有 UX D2 一致。[D2](../topics/schema-version-signal/ux/menubar-schema-signal.md#L152) 只抑制 `provider_unavailable`、`usage_unavailable` 两码，保留独立会话警告。
- 因果说明准确反映现有生产者。[Service.Build](../../internal/desktop/desktop.go#L271) 在核心库打开失败时产生 provider/usage 警告，并在分支外调用 `loadSessions`；[loadSessions](../../internal/desktop/desktop.go#L613) 仅在自己的 open/list 失败时产生 `sessions_unavailable`。[OpenSessionsReadOnly](../../internal/store/store.go#L87) 打开独立会话库并检查两张会话表，不读取核心库 schema version。

### 📝 总结

候选通过本轮评审。[修复段落](../topics/schema-version-signal/architecture.md#L486) 将错误的共同抑制集合改为 D2 的两个核心库警告，并明确会话失败的独立来源。产品差异仅涉及第三条因果解释；requirements、UX、生产代码和测试均与 HEAD 一致，没有新增行为、警告码、wire contract 或 UX 规则。carrier 保持 `active`。

**Evidence:**

| 核对项 | 独立核验结果 |
|---|---|
| 候选身份与差异 | 实际 blob 与候选声明一致；独立计算 manifest SHA-256 匹配 fingerprint；保存的 `product.diff` 与当前产品 diff 完全一致 |
| 原始文档 | `architecture-before.md` blob 为 `dd891436acbded2ff60e161f454cfffe5e9fd0c2`，匹配基线 |
| 检查器身份 | SHA-256 为 `a1e32206631d4c8d7375de0c727676e8d40020811668b19c19983f979815cfe7`，匹配 carrier 与捕获清单 |
| RED 重放 | 使用 `python3 -B`，退出码 `1`；六项检查均失败，与 `RED.json` 一致 |
| GREEN 重放 | 使用 `python3 -B` 加 `--sensitivity`，退出码 `0`；六项检查及三项反例检查均通过，与 `GREEN.json` 一致 |
| 反例敏感性 | 拒绝将会话加入抑制集合、耦合核心 schema、隐藏独立会话失败三种变体 |
| 现有 producer 测试 | 已检查测试及真实 `Service.Build` fixture 路径；捕获日志显示 `TestSchemaAheadProducerKeepsIndependentSessionAvailability` PASS |
| 文档链接 | 候选列出的 requirements 与 UX 链接目标均存在 |

回归保护能区分原缺陷：原始错误文档使检查器失败，修复文档通过，三种相关错误变体再次失败。既有 Go 测试提供生产者独立性的旁证；该测试本身不会检出文档表述错误。测试组合保持不变，无新增产品测试。

**材料限制：**

- CodeGraph entry 已因 `env: node: No such file or directory` 失败。本轮按限定源码检查降级，未重试或使用其他工作区索引。
- Go 测试结果来自捕获日志，本轮未重跑。它覆盖会话库可用与缺失；可用分支使用空会话索引，没有动态覆盖 list 失败或完整菜单栏呈现。list 失败的警告来源已由源码直接确认。
- 文档检查器验证文本约束，需结合源码与契约判断语义。三个 L0 日志为空，不能单凭其内容独立证明命令退出码；本轮未认证完整 L0 或交付门禁。

**Completion gate: VERIFIED**

Writer finalization：成功 Neo4j MCP gate 已对上述原始候选 state
`fix:arch-sessions-unavailable-not-a-consequence:state:candidate-a9a9ba49be89488a`
返回 VERIFIED，五项 required criteria 全通过，无 missing/unresolved。
完整 gate 与原始独立报告见 `/tmp/agentdeck-arch-sixth/ce-local-review-pass.json`
及 `cold-review-result.md`。原报告审查时的 NOT_VERIFIED 未被改写；此处为
writer 完成证据后的 canonical gate metadata。

本轮 Verdict 不代表 CEv1 completion。writer 已记录本报告并查询该候选完成门；实际 commit/PR/merge/归档门禁仍独立适用。本轮未修改任何文件、记录、Beads/CE 数据、Git ref 或外部服务。

Task checkpoint：本 Bug 的已评审 candidate；完成门禁 VERIFIED（5/5）。
提交建议：仅本 architecture 段落与 active carrier，签名逻辑 commit。
推送建议：普通 push 到本 issue branch，再走 draft PR、exact-head review/CI 与 merge commit。
不直接推共享 release，实际产品 PR merge 前 carrier 保持 active。
