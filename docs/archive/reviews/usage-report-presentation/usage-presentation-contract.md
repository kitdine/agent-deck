---
status: historical
plan: usage-report-presentation
task: usage-presentation-contract
---

# Review log — usage-report-presentation / usage-presentation-contract

## Round 1 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD `28b83b56311d33886dcb7db21a105ec512b9c887`.
- Scoped diff SHA-256: `3fa99b2dbbedc5ffcd1ca9af62a2fad0d0c341e93accbd03085a32b78199a9f7`.
- Scoped file SHA-256:
  - `docs/README.md`: `c4201413cf5b747cff421ef1c0c31a8d8b3378104f35fe6178e7cbe829f551fd`
  - `docs/plans/usage-report-presentation.md`: `2cd85d0daf0105f0565cdbf85125cc136013baeadb2ea9c61c0fa8d6c6c77af0`
  - `docs/specs/cli-design.md`: `b619b28357067b3f4eb62f407e365d690597ed9ba262704901e5d43fc9b8c8c5`
  - `docs/specs/cli-manual.md`: `00ffc563d331fb4613399188d21804235820e33a78374e563b81e41e076fa9d1`
- Reviewer: Codex
- Scope: Task 6 contract reconciliation for the shared usage report renderer,
  interactive stats viewer, documentation index, plan status, JSON invariance,
  and unchanged specification version.

## 📋 评审报告 — usage-presentation-contract

📊 总体评分：7/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`docs/specs/cli-design.md:963`] 当前规范仍把所有集合型 text 输出约束为共享 ASCII grid，与新写入的 usage 响应式 renderer 合同直接冲突。

- 行为风险：同一权威规范一处要求每个 collection-shaped text 使用仅含 `+`、`-`、`|` 的 grid 且 cell 不换行，另一处又要求 `usage summary`、`usage sessions`、`usage diagnose` 和 stats 使用 section、bar、对齐列及无损 continuation。实现者和回归测试无法据此判断 usage family 应保持哪种输出，Task 6 因而没有完成“把已交付行为协调进 living contract”的验收目标。
- 证据：第 963–975 行使用无例外的 `Every collection-shaped text result`，并规定 cells `are not truncated or wrapped`；第 1318–1335 行则规定 usage text 使用 shared command-layer primitives，且窄屏 secondary fields 必须进入 lossless continuation lines。当前实现已由 `renderUsageFamilySummary`、`renderUsageFamilySessions`、`renderUsageFamilyDiagnose` 和 `statsTextRenderer` 走 usage primitives，而不是通用 ASCII grid。

💡 有界修复：只收窄第 963 行开始的通用 grid 合同，使其明确适用于仍使用该 renderer 的命令集合，并明确排除使用专用响应式 primitives 的 usage report family；保留后续 usage 专节为该 family 的权威 text 合同。同步手册或索引仅限于消除由此产生的直接表述冲突，不改 specification version、JSON、产品代码或其它命令合同。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 新增 usage 专节准确覆盖固定 100% share baseline、peak-relative trend、对齐详情、结构化 cache、KPI 合并和 content-aware layout。
- interactive 合同与已交付的七区 state、20 行分页、TTY 预检、按键及 terminal restoration 路径一致。
- 手册已移除 usage family 使用统一 ASCII grid 的旧说法，并保持 JSON、pricing、attribution 和 specification version 边界不变。
- 文档索引和计划状态准确表达 Task 6 已开发、待独立评审；`git diff --check` 通过。

### 📝 总结

评审对象绑定 HEAD `28b83b5` 加四文件 scoped diff `3fa99b2dbbedc5ffcd1ca9af62a2fad0d0c341e93accbd03085a32b78199a9f7`。新增 usage 合同主体与实现一致，但 living design 中仍保留一个无例外的全局 ASCII-grid 要求，造成直接且可执行的合同冲突；因此 Task 6 结论为 FAIL，Review 保持未勾选。发现决定性阻断项后未运行全仓 L2，待有界修复进入复评。

### 下一步指令

修复：`usage-report-presentation / usage-presentation-contract`

## Repair — 2026-08-10

- Closed the blocking contract conflict in `docs/specs/cli-design.md`.
- The generic ASCII-grid rule now explicitly applies only to commands using that
  renderer and excludes `usage summary`, `usage stats`, `usage sessions`, and
  `usage diagnose`; the invariant list carries the same boundary.
- The dedicated usage section remains authoritative for responsive primitives,
  lossless continuation lines, and interactive rendering. No product code,
  JSON contract, specification version, or unrelated command contract changed.
- Verification: `git diff --check` passed; targeted search confirms no remaining
  unqualified usage-family ASCII-grid rule. Current scoped hashes:
  `docs/specs/cli-design.md` `b2b632bd888d7678a7ed910cccc439f0d1675888701719955994cfa83ee45fe7`,
  `docs/specs/cli-manual.md` `00ffc563d331fb4613399188d21804235820e33a78374e563b81e41e076fa9d1`,
  `docs/plans/usage-report-presentation.md` `2cd85d0daf0105f0565cdbf85125cc136013baeadb2ea9c61c0fa8d6c6c77af0`,
  `docs/README.md` `c4201413cf5b747cff421ef1c0c31a8d8b3378104f35fe6178e7cbe829f551fd`.

### 下一步指令

复评：`usage-report-presentation / usage-presentation-contract`

## Round 2 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD `28b83b56311d33886dcb7db21a105ec512b9c887`.
- Scoped diff SHA-256: `ad58fef76219a68c95347222c1e35bc844f19da088611be84386cfbf0b149dc8`.
- Scoped file SHA-256:
  - `docs/README.md`: `c4201413cf5b747cff421ef1c0c31a8d8b3378104f35fe6178e7cbe829f551fd`
  - `docs/plans/usage-report-presentation.md`: `2cd85d0daf0105f0565cdbf85125cc136013baeadb2ea9c61c0fa8d6c6c77af0`
  - `docs/specs/cli-design.md`: `b2b632bd888d7678a7ed910cccc439f0d1675888701719955994cfa83ee45fe7`
  - `docs/specs/cli-manual.md`: `00ffc563d331fb4613399188d21804235820e33a78374e563b81e41e076fa9d1`
- Reviewer: Codex
- Scope: Round 1 ASCII-grid contract-conflict closure, documentation-index
  consistency, unchanged product/JSON/version boundaries, and still-valid L2
  evidence for the committed usage implementation.

## 📋 复评报告 — usage-presentation-contract

📊 总体评分：8/10

✅ 结论：FAIL

### 🔴 严重问题 — 必须修复

[`docs/README.md:431`] 权威项目索引仍把已经交付并 Review PASS 的 Task 5 描述为被 session viewer 阻塞。

- 处置：新发现。
- 行为风险：`docs/README.md` 是本仓库的权威执行状态源，但该段仍声称 usage plan 的 interactive task 被 prerequisite 阻塞；同文件第 501 行、计划状态矩阵及已提交的 `28b83b5` 则表明 Task 5 已 Review PASS、Task 6 正在最终复评。恢复工作或启动 `v0.4.0` release plan 时会据此得到互相冲突的依赖状态，Task 6 的“update the documentation index”验收目标尚未完成。
- 证据：第 431–435 行仍写明计划为 active 且 “its task 5 is blocked on `interactive-session-viewer`”；`docs/plans/usage-report-presentation.md` 的状态矩阵已将 Task 5 Dev/Review 均勾选，HEAD `28b83b5` 是 `feat: add interactive usage stats viewer`。

💡 有界修复：只更新 `docs/README.md` 的 `v0.4.0` usage-presentation 叙述，使其准确说明 Tasks 1–5 已交付并独立评审、Task 6 正在最终合同复评，且 session viewer prerequisite 已满足。保留当前 active-plan 链接直到 Task 6 Review PASS 后按既有退休流程归档；不得提前宣称整个计划完成、解除 release gate、提高 specification version 或改动其它 roadmap 状态。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Round 1 finding 已关闭：`cli-design.md` 的通用 ASCII-grid 规则现在仅约束实际使用该 renderer 的命令，并明确排除四个 usage report 命令。
- 规范 invariant 已同步收窄，usage 专节继续唯一承载响应式 section、bar 和 lossless continuation 合同，没有留下第二个无限定 grid 要求。
- 修复没有触及产品代码、JSON、specification version 或其它命令合同；`git diff --check` 通过。
- 当前 usage viewer 相关代码 SHA-256 与 Task 5 Round 6 PASS 记录完全一致，Git 状态也没有代码、依赖或生成文件变化；其 focused、全仓 L2、race、build 和 compiled-binary PTY 证据仍可复用。

### 📝 总结

Round 1 的唯一 finding 在 scoped candidate `ad58fef76219a68c95347222c1e35bc844f19da088611be84386cfbf0b149dc8` 中已关闭且未回归。复评同时发现权威 documentation index 仍保留已失效的 Task 5 blocked 状态，因此新增一个同属 Task 6 的阻断 finding，结论保持 FAIL，Review 不勾选。已有产品验证绑定未变代码可复用；发现决定性文档阻断后未重复运行全仓 L2。

### 下一步指令

修复：`usage-report-presentation / usage-presentation-contract`

## Repair Round 2 — 2026-08-10

- Closed the new documentation-index finding from the previous re-review.
- Updated `docs/README.md` so Tasks 1–5 are described as delivered and
  independently reviewed, the session viewer prerequisite is satisfied, and
  Task 6 remains the final open contract review. The active plan and the
  `v0.4.0` release gate remain unchanged.
- Verification: `git diff --check` passed and a targeted search found no stale
  `task 5 is blocked` statement. Current `docs/README.md` SHA-256:
  `3fc36d0d79cd1af0d0aa054534c9c8274a32b73b5adc1ee30309c6932ae9bbbd`.

### 下一步指令

复评：`usage-report-presentation / usage-presentation-contract`

## Round 3 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD `28b83b56311d33886dcb7db21a105ec512b9c887`.
- Scoped diff SHA-256: `c2e29f62eec03128c3bdad2e6c5b5c66e8a57e26f518a4ad9ddb345ac9c5a63a`.
- Scoped file SHA-256:
  - `docs/README.md`: `3fc36d0d79cd1af0d0aa054534c9c8274a32b73b5adc1ee30309c6932ae9bbbd`
  - `docs/plans/usage-report-presentation.md`: `2cd85d0daf0105f0565cdbf85125cc136013baeadb2ea9c61c0fa8d6c6c77af0`
  - `docs/specs/cli-design.md`: `b2b632bd888d7678a7ed910cccc439f0d1675888701719955994cfa83ee45fe7`
  - `docs/specs/cli-manual.md`: `00ffc563d331fb4613399188d21804235820e33a78374e563b81e41e076fa9d1`
- Reviewer: Codex
- Scope: Round 1 contract-conflict closure, Round 2 documentation-index
  closure, final contract consistency, and reusable exact-state verification.

## 📋 复评报告 — usage-presentation-contract

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题 — 必须修复

无。Round 1 与 Round 2 findings 均已关闭，没有回归或新阻断项。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Round 1 finding 已关闭：通用 ASCII-grid 合同及 invariant 均明确排除 usage report family，响应式 usage 专节成为唯一权威 text 合同。
- Round 2 finding 已关闭：documentation index 准确说明 Tasks 1–5 已交付并独立评审、session viewer prerequisite 已满足、Task 6 是最后开放的合同复评。
- usage contract 与当前实现一致覆盖固定 share baseline、peak-relative trend、对齐和 continuation、结构化 cache、KPI 合并、content-aware layout 与七区 interactive viewer。
- 产品代码、JSON、pricing、attribution 和 specification version 均未改变；当前代码 SHA-256 与 Task 5 Round 6 PASS 记录一致，focused、全仓 L2、race、build 与 compiled-binary PTY 证据仍有效。
- 当前文档 candidate 的 `git diff --check` 通过。

### 📝 总结

Round 1 与 Round 2 的全部 findings 在 candidate `c2e29f62eec03128c3bdad2e6c5b5c66e8a57e26f518a4ad9ddb345ac9c5a63a` 中关闭且无回归。Task 6 合同、索引和计划状态一致，未改变 JSON、产品行为或 specification version；未变代码复用此前 exact-state L2/race/build/PTY 证据，文档差异检查通过。Task 6 复评结论为 PASS，完成的 feature plan 与 review records 按仓库生命周期规则退休。

### 下一步指令

提交：`usage-report-presentation / usage-presentation-contract`
