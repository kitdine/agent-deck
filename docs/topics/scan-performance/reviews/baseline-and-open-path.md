---
status: active
topic: scan-performance
subject: baseline-and-open-path
---

# baseline-and-open-path 评审记录

> **证据引用处置 — 2026-10-22**：Neo4j 恢复后，按用户要求移除本 topic 期间以文件形式保存的
> 回退证据与实验记录（原 `docs/topics/scan-performance/evidence/`，以及 Task 1 的
> `docs/evidence/records/`）。本文原指向这些文件的链接已改为纯文本，文中出现的上述路径均不再是
> 可用引用；原件仅可从提交 `c6ea1ca6551f525cc8a757ccf2e7889fabf36402` 的 Git 历史追溯。
> 结构化观测、内容身份与门禁结果以 Neo4j CEv1 为准；历史观察、结论与评审裁决保持原义。

> 2026-10-17 权威迁移说明：当前唯一证据权威为 Neo4j 的 `completion-evidence/v1`，
> namespace 为 `github.com/kitdine/agent-deck`。本文旧轮次中的离线查询结果仅为历史事实，
> 不改写为当时执行过 Neo4j 查询。原始记录可按原 ID 在 `GitEvidenceRecord` 归档中定位；
> 正式 gate 使用 `CEv1Node` 的 WorkUnit 与 ContentState ID，不能把旧 gate-request ID 当作 WorkUnit。
> 历史测量原件及其哈希保持不变；原件内旧采样器的本地 state 路径仅为当时的代码记录，
> 重放时必须通过 MCP 从 Neo4j 导出对应原始 state 到私有输入，不能恢复已撤销的本地 provider。
> 本次仅移除旧策略的操作指令和本地路径引用；历史判定、原始证据 ID 和任务完成状态不变。

## Round 1 — 2026-10-11

### 📋 Task 1 独立实现评审

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；本任务实现者为 Codex，本评审未参与其产品实现。
- Method：单评审者源码及脱敏 E0 记录核对、实际脚本语句的隔离顺序复现、Go/Swift 定向验证；未委派。
- Scope：Task 1 的 Go/Swift 诊断、原生 baseline harness、来源影响图及 E0 记录；不评审尚未实施的 Tasks 2–7。
- Reviewed state：HEAD `14078ec7c2ca9fb19e22a30177dfc5ece2729c48`；338 个范围内文件的本地内容指纹
  `08d3f2b36b74daced2aad12984dfee53b85f2b481323b97400002fa6a9f1891a`。
- WorkUnit：`scan-performance:baseline-and-open-path`；Beads：`ad-scan-performance-baseline-and-open-path`。
- Authority：用户 2026-10-11 批准的 v0.7.0 历史离线证据。
  不以 Neo4j 恢复作为前置，不把历史图谱门禁声明当作本轮已查询的结果。
- Contract：`scan-baseline-contract-r1-20261012`；ContentState：`scan-baseline-state-r1-20261012`；
  gate request：`scan-baseline-gate-r1-20261012`，均已迁入 Neo4j，按原始记录 ID 查询。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### SP-R1-F1 — P1：样本切换在等待有限 worker 结束前就重写来源

位置：`scripts/scan-performance-baseline.py:343–346`；等待逻辑在同文件 `89–100`。

- 行为风险：cold native helper 达到 120 秒 deadline 后，已接受的有限 scan 仍可继续。
  下一样本先执行 `today.write_bytes(original_today)` 和 `os.utime`，之后才进入
  `restore_state` 等待 `scan.lock` 消失。即使重写字节相同，截断/重写与 metadata
  修改也发生在前一 worker 尚可能读取来源时，破坏冻结输入和样本间隔离；不能把
  “重置数据库前已等待”扩写为“来源也在 worker 空闲后才重置”。
- 证据：从实际脚本 AST 提取上述四条连续语句，在隔离目录内保留代表活跃 worker 的
  lock，执行真实 `write_bytes`/`utime`，仅替换首次进入的 `restore_state` 为观察器。
  结果为 `lock_present_at_first_idle_check=true`、
  `source_already_rewritten_at_first_idle_check=true`。这是确定的调用顺序复现，
  不声称重现了真实语料损坏。脚本自身 `93–95` 已说明 helper timeout 不终止 worker；
  E0 的 `native_bound_baseline.samples[0]` 确实记录 `embedded_helper_timed_out`。
- 💡 有界修复：将 quiescence 屏障放在每轮任何来源/数据库/展示状态修改之前；屏障
  失败时不修改 fixture。对超时后的样本交接加入隔离回归；重新评估受影响 E0 样本，
  保留历史观察，仅补充必要的失效说明和重测。
- Disposition：OPEN，本 Task 修复。

#### SP-R1-F2 — P1：最新数据帧延迟包含旧帧同步 OCR，排除 OCR 的声明不成立

位置：`apps/macos/AgentDeckApp/MenuBarItemController.swift:237–246,296–313`；
`scripts/scan-performance-baseline.py:239`。

- 行为风险：更新开始后先调用 `baselineFrame(hero: hero)` 校验旧值，该函数在
  `@MainActor` 上同步执行 `VNImageRequestHandler.perform` 后才返回；随后等待
  refresh，再取新帧。`update_latest = new.end - start` 因而包含旧帧 OCR，且 OCR
  会占用 UI actor、影响待恢复的刷新/绘制。所得数值混入验证器成本，不能按当前
  声明解释为排除了 OCR 的 native update 延迟，也不适合作为后续优化的同口径基线。
- 证据：实际顺序为 `start → old baselineFrame（含同步 OCR）→ await update.value
  → new baselineFrame.end`。只有新帧自己的 OCR 位于 `new.end` 之后。脱敏 E0 最终
  两样本的 `update_latest` 为 25,773,257,782 ns 和 25,474,320,866 ns，且其
  limitations 仍声明 OCR 被排除；本评审不臆测其中 OCR 的具体占比。
- 💡 有界修复：将帧获取与数值验证分离，先保留内存 bitmap 和计时终点，再在不阻塞
  被测更新路径的位置验证旧/新帧；保证失败样本仍不进入成功分布。增加能检测验证器
  耗时混入的有界检查；更正已有记录的适用性并按修复后的口径补齐更新样本。
- Disposition：OPEN，本 Task 修复。

### 🟡 改进问题 — 本轮仍须修复

#### SP-R1-F3 — P2：最终报告记录的点击刺激与实际执行方式不一致

位置：`scripts/scan-performance-baseline.py:263`；
`apps/macos/AgentDeckApp/MenuBarItemController.swift:280–288`；
`docs/topics/scan-performance/evidence/baseline-and-open-path.json:1184`。

- 行为风险：报告将刺激写成 activation 后调用 status-item `performClick`，而最终
  实现构造并 post `leftMouseDown`/`leftMouseUp`。动作分发与 AppKit 事件路由的
  触发路径不同；该字段是后续 A/B 判断同刺激、同口径的重要身份信息。
- 证据：最终 aggregate verifier 的 harness SHA-256 与当前脚本一致，所有声明的
  源码 SHA-256 也与当前文件一致，但它仍携带错误的 `click_stimulus` 值。
  Task 1 文本记录已描述为鼠标 down/up，机器记录和文本互相矛盾。
- 💡 有界修复：修正新报告的刺激身份；对保留历史样本添加明确更正/适用性记录，
  不改写原观察或假称全局物理点击。加入刺激标识与所用 verifier 路径一致的检查。
- Disposition：OPEN，本 Task 修复。

### 🟢 优点

- Go 计量显式启用、仅 stderr 聚合输出；默认 v1 stdout 形状保持不变。
- 原生 trace 限定样本/失败容量，以固定 stage 聚合高频 presentation；定向测试覆盖
  5,000 次 presentation 不挤掉其他阶段，且不序列化原始 helper 输出。
- E0 明确保存 cold timeout、错误 fixture 的历史限制和旧/新数据区别；没有将失败
  帧补零，也没有宣称 E1/P01 或最终尾延迟预算通过。
- 最终 aggregate verifier 的当前源码/harness 哈希可核对；早期 stage 截断记录
  已限定不用于完整阶段归属，不与本轮的三个新问题混淆。

### 📝 总结

本轮 FAIL 源于 Task 1 自身的实验隔离、更新延迟归属和刺激身份缺陷，不因 Neo4j
不可用，也不因尚未实现的 Task 2 性能优化。三个发现均须关闭才能通过复评。

Evidence：

- `scripts/run-go-test.sh ./internal/scanruntime ./cmd/agentdeck -run
  'Test(IngestionProfileIsOptInAndMeasuresActualRereads|ScanPerformanceDiagnosticsPreserveStdoutAndRedactSources)$'`
  通过；两项定向测试覆盖 opt-in、未变来源零重读、append 与 stdout/脱敏。
- `swift test --package-path apps/macos --filter DesktopLogPolicyTests`：2 tests，0 failures。
  此包检查 Shared 日志策略，不等于完整原生 App/UI 性能验收。
- Python AST 隔离执行实际样本重置语句，得到 SP-R1-F1 的两个 true 观察；
  Swift 顺序与最终 E0 刺激字段的只读检查确认 SP-R1-F2/F3。
- 最终 aggregate verifier：harness 和声明的源码哈希全部匹配当前候选；早期记录
  的两个 Swift 差异及其已存在的 stage-attribution 不复用说明均单独保留。
- 已有全量 Go/race/build/native 测试声明作为实现者历史记录保留，不冒称本轮重新
  执行或重新查询了其旧 CEv1 门禁。确定阻塞问题后未扩大验证。
- 未采集真实 HOME、重跑大规模语料或操作生产数据库；未修改产品、测试或配置。

修复边界：仅 SP-R1-F1、SP-R1-F2、SP-R1-F3 及其受影响的 E0 记录、回归和任务状态。
后续 serving/bootstrap、watch/OTel、≥100 native 和 ≥20 heavy 门仍由各自任务承接。

## Repair — 2026-10-11

- Actor：OpenCode / GPT-6.1 Sol；correlation `scan-baseline-repair-ses_ed1e20dcfffegfBHSoHJw2g2p3`。
- Scope：仅 SP-R1-F1/F2/F3、定向回归、E0 适用性与替代测量；无委派、提交、推送或 Task 2。
- 本段是修复交接，不是新评审轮次；Round 1 的独立 FAIL 和原证据保持不变。

### Finding-to-change

- **SP-R1-F1 → repaired in candidate，待独立复评。** 每轮先调用带 worker-idle
  屏障的 `restore_state`，然后才 rollback 来源 bytes/mtime；屏障超时不会修改来源、
  数据库或展示 fixture。回归通过 AST 执行实际 per-sample reset 语句：模拟已接受 worker
  在 sleep 时仍能读到全部旧输入；模拟 300s 屏障失败时全部输入与 lock 保持原样。
- **SP-R1-F2 → repaired in candidate，待独立复评。** `baselineFrame()` 只取得
  内存 CGImage 和 draw/readback 终点；`baselineFrameIsValid` 承接 Vision 验证。
  update 路径先保存旧 bitmap，await refresh，再保存新 bitmap/终点，最后验证两个图像。
  `update_latest` 仍用 `new.end - start`；旧/新无效或数据未变化仍判失败，不进入成功分布。
  有界源码契约回归检查 capture 无 OCR、两个 capture/await/validation 的顺序和失败 guard；
  它是静态顺序保护，不冒充 UI 性能或任意验证器耗时的动态测试。
- **SP-R1-F3 → repaired in candidate，待独立复评。** Swift verifier 返回固定
  `clickStimulus`，Python report 使用相同常量，并拒绝 native 返回的不匹配身份。
  回归同时核对常量与实际 mouse-down/up postEvent 路径，以及匹配/不匹配的 launch 结果。
  实际刺激仍为 App-local 鼠标事件，不称全局物理点击；初次 routing 后才 activation。

### 验证与历史适用性

- 首次修复前，5 项回归有 4 FAIL、1 ERROR：来源被提前写回、屏障失败仍改来源、
  capture 内含同步 Vision、刺激不匹配未被拒绝、尚无统一刺激常量。修复后这些检查通过；
  另补匹配刺激及 executed binary 身份正向检查。
- `python3 -m unittest discover -s scripts -p 'scan_performance_baseline_test.py'`：6 tests，0 failures。
- `bash scripts/build-macos-app.sh`：unsigned Debug App build 成功；未安装或改动既有 App。
- 旧 E0 的原始样本、数值、hash 和声明保留；其新增 `repair_r1_applicability` 明确缩小
  先前 reuse/OCR-exclusion 声明，所有旧 native update_latest 均不用于 OCR-free 对照。
- 用户在本次修复中明确批准重新冻结三个 JSONL 根；只读原日志，不读取凭据或修改原输入。
  初次工具 120s 截止中断了准备阶段，不是 native cold 样本；恢复时发现 6 个来源已在
  私有 preparation stash，空闲后逐个验证原 inventory size/SHA-256 并 copy2 回缺失位置，
  不覆盖既有文件，保留 stash。无两个 harness 进程并行后，复用此冻结集重新执行完整测量。
- 原生两对四条件 E0 复核已完成，harness exit 1；完整脱敏替代事实见
  修复 E0。共 5,372 文件、
  3,477,927,102 bytes，新 corpus `ee716afe6b2e90713b33317b8dcab5899e3fc0e96e2a35ea7f0cf46b5f2f7c96`。
  6 个成功样本全部五个 tab 与 old/new numeric bitmap 有效、latestDataChanged=true；
  update_latest 为 11,228,110,096–11,547,022,424 ns。
- 两个 empty 样本初始/五个 warm 数值帧曾匹配，但随后更新捕获为 `popover_invisible`；
  整个失败样本不进入成功分布，缺失 update 终点记 null，不填 0、不猜修复前后的 OCR 成本。
  本次未出现 120s native helper timeout；F1 的超时交接保护来自实际重置语句的隔离回归，
  不冒称真实 timeout-handoff 已重现。没有在本轮诊断或扩展修复 popover visibility 问题。
- 所有样本的 executed App/helper hash、刺激身份、harness 与声明的源码 hash 均匹配候选。
  替代记录中 input/probes、8 组 DB/帧终点和固定八阶段 aggregates 已逐字段与 export 核对。
  所有样本 droppedStages=0、helperFailures=[]；失败样本仍保留完整诊断。
- `xcodebuild ... -only-testing:AgentDeckAppTests/MenuBarChromeTests/` 的
  `testMenuBarPopoverActivationClearsInitialResponderAcrossQuotaInsertion` 和
  `testQuotaPanelKeepsTheSharedClientHeroAndPeriodContext`：2 tests，0 failures，隔离 HOME。
  这两项为 hosted rendering/focus 回归；真实 standalone 性能证据来自上面的 E0，不相互替代。
- 新输入/业务日和 verifier 口径不同，不计算与旧 E0 的加速比；失去 loading click 时记录
  readyClickRequired=true，不称原始 first-click 成功。保留默认 quota，未配置官方凭据或测真实 quota 网络。
- 原 Go/parser/统计/Shared 计量未在修复中改变；复用 Round 1 中有精确内容绑定的 source-impact/privacy
  观察，另评估新 bitmap/常量仍仅内存验证或固定脱敏字段。未因阶段改变重跑完整 Go/race/release 门。
- 修复候选证据交接（不是 Task 完成声明）：历史离线证据 contract
  `scan-baseline-contract-repair1-20261012` 新增回归及替代 E0 的 scope，五项必需 criterion 不变；
  state `scan-baseline-state-repair1-20261012`，fingerprint
  `17d5cb67753e844c8efd6051e53a4364139effecb51c824b4c3de48c85c33aad`。
  gate request `scan-baseline-gate-repair1-20261012` 查询为 NOT_VERIFIED，已有四项有效观察，
  仅缺 independent-review；没有 failed criteria、invalidated evidence 或 unresolved impacts。
  独立复评还须确认修复与本次等 criterion 的 scope revision；修复者不复用旧 independent FAIL 为 PASS。

```json
{"work_unit_id":"scan-performance:baseline-and-open-path","target_content_state":"scan-baseline-state-repair1-20261012","missing_criteria":["independent-review"],"invalidated_evidence":[],"unresolved_candidate_impacts":[],"allowed_scope":"baseline-and-open-path independent re-review of SP-R1-F1/F2/F3 and replacement E0","authority_ceiling":"repair and in-scope handoff only; no delivery or next-phase execution"}
```

## Round 2 — 2026-10-12

### 📋 Task 1 独立复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer：OpenCode / GPT-6 Astra；沿用 Round 1 评审角色，未参与本次修复。
- Method：单评审者逐项源码核对、回归保护审阅、精确文件身份及替代 E0 的只读校验；复用未失效验证，无委派。
- Scope：SP-R1-F1/F2/F3、修复引入的回归/证据及其影响；不修复代码或启动 Task 2。
- Reviewed state：HEAD `14078ec7c2ca9fb19e22a30177dfc5ece2729c48`；内容指纹
  `17d5cb67753e844c8efd6051e53a4364139effecb51c824b4c3de48c85c33aad`。
- WorkUnit：`scan-performance:baseline-and-open-path`；Beads：`ad-scan-performance-baseline-and-open-path`。
- Authority：用户批准的 v0.7.0 历史离线证据。
- Contract：`scan-baseline-contract-repair1-20261012`；ContentState：`scan-baseline-state-repair1-20261012`；
  gate request：`scan-baseline-gate-repair1-20261012`；原始记录已迁入 Neo4j，按 ID 查询。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。SP-R1-F1、SP-R1-F2 已关闭，未发现本修复新引入的阻塞问题。

### 🟡 改进问题 — 本轮仍须修复

无。SP-R1-F3 已关闭。

### 🟢 优点与逐项关闭

- **SP-R1-F1 CLOSED**：`scripts/scan-performance-baseline.py:346–351` 已在来源
  bytes/mtime 修改前调用 `restore_state`；该函数 `97–111` 在等待 worker-idle
  成功前不修改数据库或展示状态。两项隔离回归执行实际 AST 重置语句，分别覆盖
  活跃 worker 仍读取完整旧 fixture，以及 300 秒屏障失败后所有 fixture/lock 保持不变。
  历史 timeout 后样本的隔离声明已通过追加适用性说明撤回，未重写原始数值。
- **SP-R1-F2 CLOSED**：`MenuBarItemController.swift:240–254,304–325` 已将
  capture 与 Vision 验证分开。新旧 bitmap 与两个终点均在任何该次 update OCR 前
  保存；旧/新验证失败或数据未变化仍失败。顺序保护测试明确是源码契约检查，不
  冒称动态性能测试。历史 native update_latest 已排除于 OCR-free 对照，未猜测或
  扣除 OCR 成本。新的六个完整样本提供修正后端点。
- **SP-R1-F3 CLOSED**：原生 `baselineClickStimulus`、返回的 `clickStimulus` 和
  Python `CLICK_STIMULUS` 一致，描述实际 App-local mouse-down/up 路由；harness
  `133–134` 拒绝不匹配的刺激身份。匹配/不匹配用例和源码事件路径均有检查。
  旧记录追加更正，不把最终实现身份倒套给所有早期 pilot。

### 📝 总结与证据

三项原发现全部关闭；本轮 PASS 是 Task 1 的计量基线/归属交付判断，不是扫描性能
预算、完整 UI 验收或 v0.7.0 发布就绪声明。

独立核对与复用：

- 对比原始和修复 ContentState，原范围内仅 controller、harness、原 E0 适用性发生
  变化；新增恰为定向回归与替代 E0。原 Go/Shared 生产和测试内容保持一致。
- 两版 contract 的五项必需 criterion 完全一致；scope revision 仅增加上述两个
  新产物，未删减来源影响/隐私/独立评审要求。Task 1 contract 哈希及 HEAD 已核对。
- 替代 E0 的 harness SHA-256 和全部声明的 source SHA-256 与当前文件一致；
  刺激字段与实际 harness 常量一致。八样本逐项校验：六个成功样本都有八个正数
  帧终点，两空库失败样本均为 `popover_invisible` 且两个 update 终点为 null，
  success_distribution=false。成功 update_latest 范围重新计算为
  11,228,110,096–11,547,022,424 ns，与记录一致；harness exit 1 没有被改成成功。
- 复用修复精确内容状态下的六项 Python 回归、Debug build、两项 hosted App 回归及
  native 两对四条件记录。逐项审阅测试保护与记录事实，不因复评阶段重跑相同检查。
  原 Round 1 source-impact/privacy 通过显式 target-bound preservation/dependency
  复用；本轮核对实际变更集合及新内存 bitmap/固定字符串，未发现新增持久化内容。
- 新的只读身份/样本一致性校验通过；复评入口本地 gate 仅缺 independent-review。
  本轮添加独立观察 `scan-baseline-r2-independent-review-20261012` 后查询同一 gate：
  VERIFIED，5/5，missing/failed/invalidated/unresolved 均为空；历史 FAIL 保持原样。

剩余不确定性：两个空库样本的 popover visibility 原因未确定；它们作为失败观察
保留，不用于成功分布。本基线任务没有承诺消除这些运行失败；真正的空库/更新
可用性与最终 native 门由 `tasks.md` 的 Task 2 和 Task 7 接续，不能继承本轮 PASS
为那些验收门通过。未宣称本轮真实复现 helper timeout-handoff，也不作新旧语料加速比。

Task checkpoint：`ad-scan-performance-baseline-and-open-path`；content_state
`17d5cb67753e844c8efd6051e53a4364139effecb51c824b4c3de48c85c33aad`；gate VERIFIED。

提交建议：待用户授权，将本 Task 的计量实现、修复、测试、E0、本地证据和评审/任务状态作为
同一边界提交；不夹带 canonical main 的证据工具治理改动或 Tasks 2–7。

推送建议：待用户授权提交并核对提交内容/签名后，推送 `origin/feature/scan-performance`；
远端状态与分支规则在实际推送时核实，本轮未查询远端或执行交付。

Task 2–7 尚未完成，未跨越 Topic 完成边界；下一任务是 `serving-snapshot-and-bootstrap`。
