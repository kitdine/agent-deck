---
status: historical
plan: usage-report-presentation
task: usage-bar-and-detail-semantics
---

# Review log — usage-report-presentation / usage-bar-and-detail-semantics

## Round 1 — 2026-08-07

- Reviewed state: uncommitted worktree on HEAD `aac6804ff2385305494b4185474cc00186a0cbe4`; scoped diff SHA-256 `cfe87f1bb5d227a4e9dd8e75f12423e6238e32d18f57f2b3f44a8520cb3d3146`.
- Scoped file SHA-256:
  - `usage_route_text_test.go`: `dbb62c2b13839e1ff88b47f341d428c41f6f6e2e46ea69378a569e4b7df1c9dc`
  - `usage_stats_text.go`: `c0c24ddb7d590d9b013b11804cd723e1b7477b4b790015b244192230d18a5345`
  - `usage_stats_text_test.go`: `ff9b03d5611dd32a0df8a38a94fe3e8b207e1e54ea9397714e4940089cef9c82`
  - `usage_text_primitives.go`: `2eaf32c62c3d55c191f1942b80d39a63a663e177fc134b2141c280810cb6df91`
- Reviewer: Codex
- Scope: fixed-baseline share bars, named-peak magnitude bars, aligned dimension detail columns, explicit continuation lines, and `CLIENTS` detail parity.

## 📋 评审报告 — usage-bar-and-detail-semantics

📊 总体评分：6/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`cmd/agentdeck/usage_text_primitives.go:96`] 对齐列会按固定字段宽度截断真实值，而不是保留值并按整列换行。

- 行为风险：`SESSIONS` 的字段宽度固定为 6，所有 model、client、provider 的 session 值都会先经过 `statsFit`；超过 6 个可见字符的值会被替换为省略号形式。比如测试夹具中的 `999,999,999` 无法完整呈现。这违反 Task 2 的“数值语义不变”“列按整列换行、不得静默丢弃”合同，并会让用户看到错误的会话计数。
- 证据：`usageAlignedColumns` 在第 96 行调用 `statsFit(column.value, column.width)`；`statsFit` 在 `usage_stats_text.go:907` 使用 `runewidth.Truncate`。三个维度在 `usage_stats_text.go:398`、`:436`、`:477` 都把完整 session 字符串送入该 6 字符字段。现有大会话测试在 `usage_stats_text_test.go:630` 和 `:666` 使用 `999_999_999`，但修改后不再断言 `999,999,999` 出现在文本中；`TestUsageStats` 测试族因此仍然通过。

💡 有界修复：让列宽至少容纳本 section 中该列的完整已格式化值，或在字段无法容纳时把整个 `LABEL value` 移到下一行；不要对 tokens、cost、status、sessions 的值调用截断型适配。补充 model、client、provider 的大 session/cost 值完整保留断言，并继续检查每一行不超过目标宽度。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- MODELS share bar 已改为固定 100% 标尺，并覆盖 zero、sub-cell、50% 与 100% 行为。
- TREND 保留 magnitude scaling，并在 section 标题中明确峰值。
- model/provider 的 secondary values 改为显式 continuation line，不再因窄屏静默丢弃。
- CLIENTS 已补充 tokens、cost、pricing status、sessions 四类详情字段。

### 📝 总结

评审对象为上述四个 Task 2 文件及其未提交差异。固定比例 bar、命名峰值和 continuation line 的方向符合设计，但对齐列会截断真实数值，因此存在用户可见的错误输出，Round 1 结论为 FAIL。最小 `TestUsageStats` 测试族通过，说明当前覆盖没有捕获该回归；发现决定性阻塞问题后未运行更广 L2 套件。Task 2 保持开发完成、评审未通过。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsageStats -count=1` — PASS
  - 更广 L2 验证 — 未运行；已存在决定性阻塞 finding

## Round 2 — 2026-08-07

- Reviewed state: uncommitted worktree on HEAD `aac6804ff2385305494b4185474cc00186a0cbe4`; scoped diff SHA-256 `8c16940dcb7801b25f6208c188d015784c79c9988c8904ab49d9d671d86d66b7`.
- Scoped file SHA-256:
  - `usage_route_text_test.go`: `dbb62c2b13839e1ff88b47f341d428c41f6f6e2e46ea69378a569e4b7df1c9dc`
  - `usage_stats_text.go`: `c0c24ddb7d590d9b013b11804cd723e1b7477b4b790015b244192230d18a5345`
  - `usage_stats_text_test.go`: `7f8f6397531066dbbcbf60b994d01c1efa2af7941568013b6b7c3db283612ce6`
  - `usage_text_primitives.go`: `d900253f832fa8bc743fd3c8b4e012572c130b718197bf83c17df19ce42bee1f`
- Reviewer: Codex
- Scope: Round 1 value-truncation finding, complete-value regression tests, cross-row aligned-column contract, and targeted usage verification.

## 📋 复评报告 — usage-bar-and-detail-semantics

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`cmd/agentdeck/usage_text_primitives.go:96`] 完整值已保留，但列宽按每一行单独扩展，同一 section 的数字右边界不再一致。

- 处置：新发现。
- 行为风险：普通行的 `SESSIONS` 使用 6 字符值宽，而 `999,999,999` 行扩展为 11 字符值宽。两行的 `SESSIONS` 数字结束位置相差 5 列，无法“逐位对齐”。这违反 Task 2 的跨行 aligned detail columns 合同；大值修复不能以失去 section 内列对齐为代价。
- 证据：`valueWidth := max(column.width, statsVisibleWidth(column.value))` 在每次 `usageAlignedColumns` 调用内独立计算。`TestUsageStatsDimensionDetailsPreserveLargeValues` 分别只给 model、client、provider 一个 row；`TestUsageAlignedColumnsPreserveFieldValues` 也只验证一次调用，因此都没有比较同一 section 中普通值与大值行的列边界。当前 `TestUsage` 测试族仍通过。

💡 有界修复：在渲染一个 section 前，根据该 section 所有可见 rows 计算共享的 tokens、cost、status、sessions 值宽，并让每一行使用同一组宽度。空间不足时继续按完整列换行，不得截断值。增加同一 section 至少两行（普通值与大值）的 model、client、provider 测试，在 48、60、80、100 列下同时断言完整值、逐列右边界一致及每行不越界。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 已关闭 Round 1 截断 finding：`usageAlignedColumns` 不再以原固定宽度截断值。
- 新增 model、client、provider 大 session/cost 值完整保留测试。
- share bars、named peak、explicit continuations 和 JSON 不变证据未被本轮修复影响。

### 📝 总结

Round 1 的错误数值显示已关闭，没有回归；Round 2 新发现跨行列宽逐行变化，导致 Task 2 的 aligned-column 目标仍未完全实现，因此结论保持 FAIL。当前 `TestUsage` 测试族通过，但没有覆盖同一 section 多行的对齐位置。发现决定性阻塞问题后未运行更广 L2 套件，Task 2 继续保持开发完成、评审未通过。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsage -count=1` — PASS
  - 更广 L2 验证 — 未运行；已存在决定性阻塞 finding

### 🛠 修复指令

只修复同一 section 多行共享 detail-column 宽度的问题：预计算 section 级列宽，完整保留值并按整列换行；补充 model、client、provider 在 48、60、80、100 列下普通值与大值共存的完整值、列边界和行宽断言。不要修改 share-bar、trend、continuation、JSON、Task 3、Task 5 或 session 行为。

## Round 3 — 2026-08-07

- Reviewed state: uncommitted worktree on HEAD `aac6804ff2385305494b4185474cc00186a0cbe4`; scoped diff SHA-256 `dba27f5374e25f047b662e3e06ee5493f81a8b38c3c543626a807e78eeabbdd6`.
- Scoped file SHA-256:
  - `usage_route_text_test.go`: `dbb62c2b13839e1ff88b47f341d428c41f6f6e2e46ea69378a569e4b7df1c9dc`
  - `usage_stats_text.go`: `d38b61362130cacde2b2b8d4743582dca8206c1d44922f5a0d49a3f6b3436364`
  - `usage_stats_text_test.go`: `4f3f943cde22ea30337ebc378ff493073897a861b3ef60c12ae349382dd9bf82`
  - `usage_text_primitives.go`: `bb891bba43f991c82289f5db2613d46eb1560cc07d4bf39e11467176b8d10d8a`
- Reviewer: Codex
- Scope: Round 2 section-shared width finding, defensive value preservation in the shared primitive, multi-row/multi-width regression tests, and targeted usage verification.

## 📋 复评报告 — usage-bar-and-detail-semantics

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`cmd/agentdeck/usage_text_primitives.go:116`] `usageAlignedColumns` 再次按传入固定宽度截断值，现有 targeted test 直接失败。

- 处置：Round 1 value-preservation finding 回归。
- 行为风险：当前 renderer 的三个 section 会先调用 `usageAlignColumnRows`，因此这些路径通常获得扩宽后的列；但共享 primitive 本身不再保证完整值，任何未预对齐调用都会产生错误显示。Task 2 已有 primitive-level 合同与测试，不能依赖所有当前和未来调用方都先修正宽度。
- 证据：`usageAlignedColumns` 使用 `statsPadLeft(column.value, column.width)`，而 `statsPadLeft` 会经 `statsFit` 截断。`TestUsageAlignedColumnsPreserveFieldValues` 实际输出 `COST $123,456,78… SESSIONS 999,9…`，导致当前 `TestUsage` 测试族 FAIL。

💡 有界修复：保留现有 section 级 `usageAlignColumnRows`；同时在 `usageAlignedColumns` 内把每个字段的有效值宽设为 `max(column.width, statsVisibleWidth(column.value))`，再右对齐。这样共享 section 仍使用统一宽度，未预对齐的调用也不会截断值。现有失败测试已经是最小回归门禁。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 已关闭 Round 2 跨行对齐 finding：MODELS、CLIENTS、PROVIDERS 分别预计算并复用 section 级共享列宽。
- 新测试覆盖同一 section 普通值与大值共存，并覆盖 48、60、80、100 列的完整值、session 右边界和行宽。
- share bars、named peak、continuation lines 与 JSON 路径没有被扩大修改。

### 📝 总结

Round 2 的跨行对齐问题已关闭，但 Round 1 的 primitive-level 完整值保证发生回归，且目标测试实际失败，因此 Round 3 结论仍为 FAIL。由于 targeted suite 已失败，未运行更广 L2 套件。Task 2 继续保持开发完成、评审未通过。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsage -count=1` — FAIL
  - 失败测试：`TestUsageAlignedColumnsPreserveFieldValues`
  - 更广 L2 验证 — 未运行；targeted gate 已失败

### 🛠 修复指令

保留 section 级共享宽度逻辑，仅恢复 `usageAlignedColumns` 的防御性完整值保证：使用不小于值可见宽度的有效字段宽度后再右对齐。不要删除或放宽现有 value-preservation、multi-row、48/60/80/100 行宽测试；不要修改 Task 3、Task 5、session、JSON、share bar、trend 或 continuation 行为。

## Round 4 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD `aac6804ff2385305494b4185474cc00186a0cbe4`; scoped diff SHA-256 `91e9a8355b8bef9e605face20abd676e4f2c8b0e6bdc7dc9f78d318fb91efe86`.
- Scoped file SHA-256:
  - `usage_route_text_test.go`: `dbb62c2b13839e1ff88b47f341d428c41f6f6e2e46ea69378a569e4b7df1c9dc`
  - `usage_stats_text.go`: `d38b61362130cacde2b2b8d4743582dca8206c1d44922f5a0d49a3f6b3436364`
  - `usage_stats_text_test.go`: `4f3f943cde22ea30337ebc378ff493073897a861b3ef60c12ae349382dd9bf82`
  - `usage_text_primitives.go`: `d1a6b34205bf5ff529e1f69f5f0692b24be1db289936abb4835f8e99b9c8c2df`
- Reviewer: Codex
- Scope: Round 3 primitive-level value-preservation regression, prior section-alignment fix, unchanged Task 2 semantics, targeted usage gate, and final L2 verification.

## 📋 复评报告 — usage-bar-and-detail-semantics

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 已关闭 Round 3 primitive-level 回归：`usageAlignedColumns` 使用固定宽度与完整值可见宽度的较大者，直接调用不再截断 cost 或 sessions。
- Round 2 section 级共享宽度保持不变，MODELS、CLIENTS、PROVIDERS 的普通值与大值仍逐列对齐。
- Round 1 的完整值保证、fixed-100% share bars、named-peak magnitude bars、explicit continuation lines 和 CLIENTS 对等详情全部保持闭合。
- Targeted usage tests、全仓 L2 tests 与 scoped diff check 均通过。

### 📝 总结

Round 1 value truncation、Round 2 cross-row alignment 和 Round 3 primitive regression 均已关闭，没有仍开放、回归或新阻塞 finding。复评对象绑定上述新候选；Round 3 后仅 `usage_text_primitives.go` 发生有界修复，因此其余已通过证据可复用。Task 2 达到 Review PASS，残余风险限于未提交候选的交付边界。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsage -count=1` — PASS
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS
  - scoped `git diff --check` — PASS
