---
status: historical
created: 2026-10-03
retired: 2026-10-03
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

- 修复恢复了既有 requirements 的会话库独立性。[requirements.md](../../topics/schema-version-signal/requirements.md#L246) 明确将 `sessions.sqlite3` 排除于核心库 schema 问题之外，并承认可读会话仍能呈现。
- 修复与既有 UX D2 一致。[D2](../../topics/schema-version-signal/ux/menubar-schema-signal.md#L152) 只抑制 `provider_unavailable`、`usage_unavailable` 两码，保留独立会话警告。
- 因果说明准确反映现有生产者。[Service.Build](../../../internal/desktop/desktop.go#L271) 在核心库打开失败时产生 provider/usage 警告，并在分支外调用 `loadSessions`；[loadSessions](../../../internal/desktop/desktop.go#L613) 仅在自己的 open/list 失败时产生 `sessions_unavailable`。[OpenSessionsReadOnly](../../../internal/store/store.go#L87) 打开独立会话库并检查两张会话表，不读取核心库 schema version。

### 📝 总结

候选通过本轮评审。[修复段落](../../topics/schema-version-signal/architecture.md#L486) 将错误的共同抑制集合改为 D2 的两个核心库警告，并明确会话失败的独立来源。产品差异仅涉及第三条因果解释；requirements、UX、生产代码和测试均与 HEAD 一致，没有新增行为、警告码、wire contract 或 UX 规则。carrier 保持 `active`。

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

## 交付与退休 — 2026-10-03

产品 PR [#38](https://github.com/kitdine/agent-deck/pull/38) 已实际合并。

- 逻辑 source commit：`c426b249098a844f5ec61fa97c2fdfac2aa1e151`；实际 SSH
  signature、Conventional subject、非空 body、exact Codex trailer 均已核验。
- 实际 merge：`132fca40c16928c9e7846a298bc8e583ff700c84`；parents 为
  `4b4b67a6c4b7bc93f3e7d4117c28579c62d7cbf6` 与上述 source commit，
  tree `79c61bf8c5b455205a676a796146768342d80c66`。
- 实际 refs、tree、完整 merge message 与 trailer、GitHub signature/payload
  均已核验；merge 中本 carrier 保持 active。证据：
  `/tmp/agentdeck-arch-sixth/product-merge-verification.json`。
- Exact-head GitHub Codex review 明确绑定 `c426b24909`，无具体 findings，
  inline comments 与 change requests 为空。四项 Actions（push/PR 各
  verify、desktop）全部 success；见 `product-github-review-final.json`
  与 `product-ci-final.json`。
- Merge-bound CEv1 state
  `fix:arch-sessions-unavailable-not-a-consequence:state:product-merge-132fca40`
  经实际 MCP gate 返回 VERIFIED（5/5），无 missing/unresolved；见
  `ce-product-merge-132fca40.json`。

在上述实际交付与证据收口之后，按既有 Fix records 规则迁移本 carrier 并设为
historical。本次退休只涉及 carrier 的位置、frontmatter、相对链接与交付记录，
不修改 architecture 或产品内容；按既有约定不新增 archive-index entry。
Round 1 的 active 状态陈述和审查时限制属于当时的历史上下文，保留而不重写。

归档本身走独立 PR 的 review/CI/merge。其最终 refs/parents/tree/message/signature、
CE final、Bug close/unassign 与 clean released handoff 以
`/tmp/agentdeck-arch-sixth/final-clean-release.json` 和对应 Beads released comment
记录；本历史文档不预先宣称这些未来操作已经完成。


## Review — Round 2

## 📋 ASU-ARCH-R2 — 合法退休载体独立评审

📊 综合评分：10/10

✅ **Verdict: PASS**

- **Reviewer session ID:** `01a100ca-ffd5-7920-9c9b-95c0c26aab89`，实际读取自 `CODEX_THREAD_ID`。
- **Method:** 新独立 cold-context CLI reviewer；只读 shell 检查规则、载体差异、本地 Git 对象及既有捕获证据。无委派。
- **Scope:** 仅 Bug `ad-bug-arch-sessions-unavailable-not-a-consequence` 的归档退休载体；不重新评审产品修复，不扩展至其他 Bug、contract/assemble 或图标。

**Reviewed state:**

| 项目 | 内容身份 |
|---|---|
| Branch | `fix/arch-sessions-unavailable-not-a-consequence` |
| HEAD | `c426b249098a844f5ec61fa97c2fdfac2aa1e151` |
| 实际产品 merge | `132fca40c16928c9e7846a298bc8e583ff700c84` |
| 归档载体 blob | `b862b36c96f32c9af41b18a14c06d4beb330be0c` |
| Architecture blob，未变 | `fd7d8067d44963a9c2714356f7e60e9549a6a520` |
| Candidate fingerprint | `7a4847caceb762e975224c42bb85272bb1bb7fce5ce6a57779b07b42e1dbe684` |

### 🔴 严重问题 — 必须修复

无。

**Findings:** 无。本轮未生成 `ASU-ARCH-R2-Fn` 发现；严重程度与 Disposition 不适用。无开放、延期或转移的范围内发现。

原产品 Round 1 无 findings。本次没有将此前 CPA 归档评审失败且未产生 verdict 的尝试视为 PASS；该历史事实保留，本报告是对现有候选的独立判断。

### 🟡 建议改进 — 推荐

无。未发现范围内可行动缺陷，包括轻微缺陷。

### 🟢 优点

- 退休发生在实际产品 PR #38 合并及其证据收口之后。产品 merge 内载体仍为 `active`，没有提前退休。
- 原观察、根因、修复边界、评审、证据和限制说明均保留。历史 `active` 陈述及原候选 gate 有明确时间和对象，没有被误读为当前归档状态。
- 六个相对链接均按迁移后的目录深度修正，目标文件存在。
- 变更限于载体搬迁、历史 frontmatter、相对链接和实际交付来源；没有 architecture、产品代码、测试或 archive-index 变更。

### 📝 总结

该候选符合 `docs/documentation-workflow.md` 的 Fix records 生命周期及 review-record 历史保留要求。新增交付记录与实际产品 merge 对应；归档 PR、最终 CE、Bug close/unassign 和 released handoff 被明确表述为后续操作，没有虚假完成声明。

**Evidence:**

| 核对项 | 结果 |
|---|---|
| 工作区范围 | `git status --short` 仅显示原载体删除和归档载体新增 |
| 载体差异 | 与 HEAD 的 active carrier 比较，仅有退休字段、相对链接及交付退休段落；历史评审未删除 |
| 候选身份 | 实际 `git hash-object` 与两个声明 blob 一致；独立计算 manifest SHA-256 匹配 fingerprint |
| 产品保持不变 | 产品 merge 与 HEAD 的 tree 同为 `79c61bf8c5b455205a676a796146768342d80c66`；两者间无文件差异 |
| 活动载体交付状态 | HEAD 与产品 merge 内原载体 blob 均为 `bf0318e9c87e43e0ca9cb52918998f704be6ca55`，frontmatter 为 `active` |
| 实际 merge 对象 | 本地对象 parents 为 `4b4b67a6c4b7bc93f3e7d4117c28579c62d7cbf6`、上述 source commit；tree 和完整 message 与交付材料一致 |
| Message 与签名证明 | source 对象含 SSH signature；source、merge 均有 Conventional subject、非空 body 和 exact Codex trailer。既有验证材料记录 SSH 验证成功、GitHub signature valid、payload/signature 匹配 |
| PR 与 refs | 捕获 PR #38 为已合并，merge 时间 `2026-10-03T07:14:21Z`；本地 `origin/release/v0.6.x` 指向实际 merge。陈旧本地 release ref 未作为交付依据 |
| GitHub review / CI | 既有 exact-head review 材料绑定 source commit；四项 CI 均为该 HEAD 的 completed/success |
| 产品 CE | 捕获 MCP gate 对 `state:product-merge-132fca40` 返回 VERIFIED；五项 required criteria 有有效证据，无 missing、invalidated 或 unresolved impacts |
| 归档 L0 | `archive-candidate.json` 明确记录 whitespace、diff-check、topic-docs 三项退出码均为 `0`；未重跑 |
| 相对链接 | 六个目标文件均存在；活动目录中的原载体已不存在 |

**Completion gate: NOT_VERIFIED**

这是当前归档候选的门禁状态。产品 merge 的 VERIFIED 不自动覆盖归档候选；writer 尚需记录本报告、完成显式证据复用评估并查询对应 gate。本轮 PASS 不代表归档交付、Bug 关闭或最终 handoff 已完成。

**限制与权限记录：**

- CodeGraph 已知因 `env: node: No such file or directory` 降级；未重试或使用其他工作区索引。
- 未重复工具链接受、产品测试或原产品验证；未请求在线服务，也未重新查询 CE。交付证明使用本地对象及既有捕获材料。
- 最后一次补充比对命令在 shell 展开 here-document 时退出 `1`，原文为：`zsh:1: can't create temp file for here document: operation not permitted`。Ruby 未执行，额外的归一化签名/payload 比对及 review-count 断言未取得结果，不计作通过证据。该命令已停止，未重试或绕过权限；此补充检查处于非阶段 `AUTHORIZATION_WAIT`，未创建 phase token。
- 本轮未写入文件、Git ref、Beads/CE 记录或外部服务；正式评审记录及门禁收口由主 writer 负责。

### Writer 接手与记录

原 CPA 冷审 `01a100a2-b5a7-73b2-a316-aaf22120ca18` 因服务 high demand 未产生 verdict，未算作通过。新 official 冷审为上述独立只读 session，实际 exit 0；原始报告完整保存在 `/tmp/agentdeck-arch-sixth/official-resume/archive-cold-result.md`，SHA-256 为 `ef45cbc93f586ffcee6739f0fa8e151d90a9440295afef2203c5b7e5e3bedc60`。

本批 official CLI writer session `01a100c5-933c-7e91-83c6-fe81b2035434` 按明确 handoff 接手；transport 恢复仍使用同一 session、`gpt-6.1-sol / xhigh / service_tier=default / on-request / workspace-write`。provider 实际读回 official direct、无 base_url。Bug owner 保持 codex，slot 持有。writer 核对原报告与冻结 carrier、产品 merge 证据及相对链接后原样记录 verdict 和限制；未重复有效产品测试。

Ruby 补充检查未执行、不计通过，未重试或绕过；已有有效产品 signature/payload 证明保持其原始状态。正式报告追加本身仅记录该候选审查结果，不修改被审查的退休事实、既有产品契约或历史报告。最终 staged/commit/merge 对象、CE gate 与 close/unassign 仍需分别核验。本批交付证据以 `/tmp/agentdeck-arch-sixth/official-resume/` 为主；原 evidence root 的有效产品证据继续复用。

**Writer completion gate: VERIFIED（5/5）**

标准 Neo4j MCP 对原冻结归档候选 state
`fix:arch-sessions-unavailable-not-a-consequence:state:archive-candidate-7a4847ca`
返回 VERIFIED，missing、invalidated、unresolved 均为空。记录了产品 merge
证据的显式 preserves 评估及 target-bound roll-up；未重跑有效产品测试。
provider 写响应仅返回更新摘要，writer 已通过实际 MCP readback 核对
11 个 node 的完整 payload 与 25 个 relation 的种类、端点及数量。
完整证据：`/tmp/agentdeck-arch-sixth/official-resume/ce-archive-candidate.json`。
原始冷审报告中的 NOT_VERIFIED 保留为审查当时的状态。

Task checkpoint：本 Bug 的 carrier 退休及正式评审记录；冻结候选完成门 VERIFIED。
提交建议：仅本 carrier 的搬迁、退休元数据、链接、交付来源及追加报告，
在最终 staged state 的完成门通过后创建已授权的 signed logical archive commit。
推送建议：普通 push 本 issue branch，独立 draft archive PR；exact-head GitHub
Codex review、required CI 及实际 merge/final CE/lifecycle 均须继续核验。
无 containing topic gate，本批结束后停止，不生成下一批指令。
