---
status: active
plan: usage-report-presentation
task: usage-stats-layout
---

# Review log — usage-report-presentation / usage-stats-layout

## Round 1 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD `08f5492d9b37f8e0e71d966317e5d0e221000768`; scoped diff SHA-256 `c7347190b35c72351e754286131c952b377789e0d241273e8a433870c182b842`.
- Scoped file SHA-256:
  - `usage_stats_text.go`: `ce44ba80453b7d9e9216ba867bab1b484cc03dc75c01de8aea4bccf66a97e2fd`
  - `usage_stats_text_test.go`: `717eacaf58b788c16d72b441e6de61bd7e222972a3f2057aefd0b414a6996315`
- Reviewer: Codex
- Scope: content-aware trend/ranking layout, structured cache model/session presentation, single footer affordance, KPI consolidation and explicit bases, and related text/JSON regression tests.

## 📋 评审报告 — usage-stats-layout

📊 总体评分：5/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`cmd/agentdeck/usage_stats_text.go:562`] 多个 cache model 的主行和详情行被分成两个批次输出，READ/WRITE 数据失去所属 model。

- 行为风险：第一轮循环先连续输出所有 `MODEL ...` 主行，第二轮循环才输出没有 model 标识的 READ/WRITE 详情。存在两个 model 时，第一个详情行紧跟最后一个 model 主行，用户会把数据关联到错误的 model；这违反“每个 cache model 是普通 row，带对齐详情列”的结构合同。
- 证据：第 562 行循环同时追加全部 model labels 并收集 `modelDetails`；`usageAlignColumnRows` 后，第 573 行第二个循环才批量追加所有详情。现有测试夹具虽然包含两个 cache model，但没有断言每个主行与其详情相邻。

💡 有界修复：第一轮只计算全部 model labels/details 和共享列宽；第二轮按 model 顺序交错输出“主行，然后该 model 的详情行”。增加两个 model 使用明显不同 READ/WRITE 值的顺序断言。

[`cmd/agentdeck/usage_stats_text.go:598`] 单一 footer 只携带第一条 cache session 的完整命令，其余 session 只剩被截断的 ID。

- 行为风险：session list 最多可显示多条，但每行 ID 经过 bounded `statsFit`。footer 固定使用 `shownSessions[0].DetailCommand`，因此第二条及后续 session 的完整 identifier 和可复制命令不再出现在文本输出中，用户无法可靠操作这些可见 session。
- 证据：第 589–596 行渲染所有 bounded session rows；第 598–602 行只读取 index 0 的 `DetailCommand`。新增测试只构造一个 session，所以 `strings.Count(...)=1` 无法保护多 session 行为。

💡 有界修复：保留“footer affordance 只打印一次”，但它必须为列表中每个可见 session 保留可操作映射，例如在 footer 内集中列出 bounded label 到完整 command 的映射，或采用一条明确的命令模板并让每个 row 保留可复制的完整 identifier。增加至少两个长且前缀相同 session ID 的测试，证明两者都可无歧义操作且 footer heading 只出现一次。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 两列布局同时检查宽度、窄列额外换行和 3:1 高度失衡，单 bucket/tall ranking 场景能回退 stacked layout。
- CACHE HIT RATE 已具备 CACHE MODELS/CACHE SESSIONS 层级、独立 top cap 和省略 footer。
- AVG COST / SESSION、PEAK metric、PRICED EVENTS 已合并到 KPI region 并明确 basis。
- JSON 集合保持原值的测试方向正确。

### 📝 总结

评审对象为上述两个 Task 3 文件及其未提交差异。Content-aware layout 与 KPI consolidation 符合主要设计方向，但 structured cache section 在多 model、多 session 的真实列表场景中破坏数据归属与操作可达性，因此 Round 1 结论为 FAIL。最小 cache section 测试通过，但其单 session 数据没有覆盖阻塞行为；发现决定性问题后未运行更广 L2 套件。Task 3 保持开发完成、评审未通过。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsageStatsCacheSection -count=1` — PASS
  - 更广 L2 验证 — 未运行；已存在决定性 blocking findings

## Round 2 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD `08f5492d9b37f8e0e71d966317e5d0e221000768`; scoped diff SHA-256 `21db7bdafdf4c75027c7b1ba3f1e5b47be1cead935205d37f318e0a9fd0ae679`.
- Scoped file SHA-256:
  - `usage_stats_text.go`: `1abb36df617d36fba5188c7c98c3955e8ed9aa3a39c86b5e1f34a5bcc03aa2f2`
  - `usage_stats_text_test.go`: `f0548baf08ad9a35524d1aca211c6e7e24e95272baa827d898acdc1002411b6e`
- Reviewer: Codex
- Scope: Round 1 cache model association and multi-session actionability findings, unchanged content-aware layout/KPI behavior, targeted cache tests, and final L2 verification.

## 📋 复评报告 — usage-stats-layout

📊 总体评分：9/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[`cmd/agentdeck/usage_stats_text_test.go:1472`] 缺少“第一条 session command 为空、第二条有值”的专门回归测试。

- 处置：新发现，非阻塞测试增强。
- 证据：生产循环会对空 `DetailCommand` 执行 `continue`，随后继续输出后续 session 映射，因此当前行为正确；现有双 session 测试两条命令均非空，不能单独锁定这个边界。

💡 有界改进：在后续测试维护中增加 first-empty/second-present fixture，断言 footer 仍包含 `[2]` 完整命令且没有 `[1]` 空命令行。无需阻塞 Task 3 交付。

### 🟢 优点

- 已关闭 cache model 关联 finding：共享 READ/WRITE 宽度预计算后，按 model 交错输出主行和对应详情。
- 已关闭 cache session actionability finding：每个可见 row 使用稳定 `[n]`，单一 `DETAIL COMMANDS` footer 为每个非空命令保留完整映射。
- 双 model 测试使用不同 token 值验证输出顺序；双长前缀 session 测试验证完整 ID、索引和单 footer。
- Content-aware layout、KPI consolidation 与 JSON 不变行为未被修复扩大修改。
- Targeted cache tests、全仓 L2 tests 与 scoped diff check 均通过。

### 📝 总结

Round 1 两项 blocking findings 均已关闭，没有仍开放、回归或新阻塞 finding。复评对象绑定上述新候选；content-aware layout、structured cache section、KPI basis 和 L2 verification 均达到 Task 3 合同。残余风险仅为一个非阻塞边界测试缺口，Task 3 达到 Review PASS。

- 验证证据：
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsageStatsCache -count=1` — PASS
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS
  - scoped `git diff --check` — PASS
