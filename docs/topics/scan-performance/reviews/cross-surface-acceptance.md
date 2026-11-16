---
status: active
topic: scan-performance
subject: cross-surface-acceptance
---

# cross-surface-acceptance 评审记录

## Round 1 — 2026-10-30

### 📋 Task 8 独立实现评审

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra；独立于本任务实现的评审角色。
- Method：单评审者源码与差异检查、已记录证据的结构化核验；无委派。
- Scope：Task 8 的 21 个候选文件，测量编排、P09/P06 统计、重建握手、信号退出、schema 33、跨表面测试、公开契约与原型默认值，以及最终矩阵的证据身份与复用。
- Reviewed state：HEAD `27bbe5f65483ff18e4e40ca85518b4a8cf42d3e4`；候选指纹 `4ba2968db433acf3a1d8645b9f37cab53a814a0cdaea7b614f51c091a7018aab`。
  算法为 SHA-256(`head=<HEAD>` 与排序后的 `path:git-blob` 行以 LF 连接，无尾 LF)。评审入口实算 21 个文件，与 CEv1 `scan-perf-csa-final-20261030-state` 的逐文件身份一致。本轮新建记录与状态交接不属于该入口候选指纹。
- WorkUnit：`scan-performance:cross-surface-acceptance`。
- 完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

#### CSA-R1-F1 — P2：最终矩阵缺少各测量工具版本的源码与构建身份映射（OPEN）

位置：`docs/topics/scan-performance/tasks.md:350–352` 的完成证据指针；规范 `performance-evaluation.md:277–282`；CEv1 `scan-perf-csa-final-20261030:full-metric-matrix` 的 `measurement_matrix.conditions.*.arms.*.identity` 与 `source_state_notes`。

- 行为风险：最终矩阵混用三个采集工具二进制，另用一个分析工具。只有二进制哈希，无法确定每个批次实际执行的工具源码与构建配置，也无法据此复现旧版工具，或独立检查工具变更是否确实只影响所重测条件。当前最终候选的源码身份不能反向代表此前所有测量工具构建。
- 证据：逐条件提取 `perf_tool_sha256` 得到：

  | 采集工具 SHA-256 | 正式条件 |
  | --- | --- |
  | `02d2ed597e45a4dfb2bb95d2d4a37756ac2194f0425c531f5336f48be69768f4` | P01-first、P01-repeat、P02、P08、P10、P09-B0、P09-S0、P06 |
  | `593059bee9c7d042e23bfddfc5aa7b47a4b515edbdd1aa94b0a4208dce4c930f` | P01-rebuild、P03 |
  | `021fa1b4a49f6f0a8694bb12b94cc9bcfca8a233304891ba8e6dcf3de8ee82cd` | P04、P11-first、P11-full、P07 |

  分析工具为 `b09e2ed1c058c7d92e60276249ad43de57ae76e333235d8906008b7abb00e42f`。该证据记录了 accepted S0 的 HEAD/候选指纹、B0 提交、通用 Go/Xcode 版本及一个 PerfDriver 适配文件的源码摘要，但没有以上各 Go 工具版本的源码摘要与构建配置映射。按前两个旧工具哈希查询本仓库 CEv1，命中仅此矩阵记录，无独立的对应构建来源记录。矩阵的 `metadata_objects` 是输入语料/模板身份，不是工具源码身份。
- 规范要求：测量契约明确要求 CEv1 保存工具源码内容摘要和构建配置；多版本工具本身不是缺陷，缺的是各版本与批次之间可核验的来源和适用性关联。入口门禁虽返回 VERIFIED，但只能检查已声明的 pass 图谱，不能替代这一内容审查。
- 💡 修复建议：从保留的构建来源恢复三个采集工具及分析工具的确切源码状态、摘要算法与文件范围、构建命令/配置、相关工具链，并绑定到对应二进制及 pilot/formal/分析批次；对不同工具版本到最终状态的复用给出范围化影响判断。以新的证据记录显式替代不完整的完成观察，不改写旧事实。能够恢复并核验身份时复用原始有效样本；仅对身份无法恢复或适用性无法成立的条件补测，不重跑无关全矩阵。
- Disposition：OPEN；本任务修复范围，不以“后续完善”放行。

### 🟡 建议改进 — 推荐

无额外建议。

### 🟢 优点

- 候选逐文件身份与交接一致；最终矩阵完整保留 14 个正式条件及 pilot、1,104 行运行单元环境信息和一次外部打断。
- 独立核对双臂 helper SHA-256、模板身份引用和双臂运行单元 ABBA 顺序通过。P09 保留每臂自己的 worker 成本，P06 提供有符号逐对差值与中位数。
- schema 33 的方向性兼容和未来 schema 拒绝有回归保护；新增 Widget 对照覆盖 CLI wire 的逐值投影，而非仅验证可解码。
- 历史失败、未运行的安装态验收、并发 Go 超时根因未知及首次 P06 偏好恢复不确定性均明确保留，没有用后续成功抹去。

### 📝 总结

#### Evidence

1. 入口候选指纹重新计算匹配，21 个文件；生产代码、测试、配置保持只读。
2. 标准 CEv1 gate 查询入口结果为 VERIFIED，五项 required criteria 齐全，missing/invalidated/unresolved 均为空。独立检查发现 CSA-R1-F1 后，新增 `scan-perf-csa-review-r1-20261030:full-metric-matrix`，以 `not_verified` 表达证据不足并显式 supersede 原完成观察；最终门禁为上述 NOT_VERIFIED。这不表示性能测量值已被证明错误。
3. 对返回的矩阵作确定性审计：14 个条件的工具无效数均为 0；候选 helper 均为 `0d56d5ec50ae600b2c7b24f837099c8b4f7ef8fe8728ba0c968c2204ba434d19`，S0 helper 均为 `3b52bcd9f94e6892780e4cce3f1900146ee5237a0284191055322402f8ee909a`；每个模板身份引用均可解析；双臂条件逐单元的开始时间满足奇数 A→B、偶数 B→A；P09 各 20 对；环境行合计 1,104。
4. 复用 `scan-perf-csa-closeout-20261030:check:*` 经 final roll-up 绑定的检查事实：全 Go 串行 1,462 个顶层测试、最终 CLI/perftrace 409 个、native 334 个（一个既有 opt-in 手工矩阵跳过）、Widget 投影 18 个、信号 race、双架构构建、vet、文档与原型检查。未因评审阶段变化重跑这些检查；没有将其解释为安装态 Widget 或 L4 验收。
5. 本轮记录以 `python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/cross-surface-acceptance.md`、`python3 scripts/check-docs.py` 与 `git diff --check` 收口。它们只证明文档结构与卫生，不替代证据内容审查。

#### 当前报告的关键数值（来源尚待 CSA-R1-F1 补齐，不作最终验收结论）

| 指标 | 基线 → 候选 | 变化 |
| --- | --- | --- |
| P01 首击 median / p95 | 270.806 / 392.772 → 108.116 / 188.296 ms | −60.1% / −52.1% |
| P01 重复 median | 85.801 → 77.965 ms | −9.1%；比值区间跨 1 |
| P01 重建中 median | 186.017 → 90.657 ms | −51.3% |
| P02 median | 437.557 → 351.322 ms | −19.7%；p95/p99 比值区间跨 1 |
| P03 client / period / panel median | 99.621 / 99.751 / 99.780 → 66.222 / 66.390 / 66.456 ms | 约 −33.4% |
| P04 median | 979.986 → 1,149.640 ms | +17.3%，变慢 |
| P06 三对中位差 | CPU 单核占比 +0.000008676；稳定 RSS +2,048 KiB；维护激活差 0 | 小幅资源增加；不出区间 |
| P07 事务 p95 / max | 1.197 / 322.965 → 27.506 / 181.654 ms | p95 变差、max 改善；事务数 252,958 → 15,709，不将分布变化称为全面改善 |
| P07 重建聚合 RSS 上界（三次） | 176,218,112 / 169,684,992 / 171,900,928 → 186,445,824 / 183,259,136 / 178,094,080 bytes | 候选三次均较对应基线高 |
| P08 append 读取 | 4,184,133 → 65,064 bytes | −98.4%；候选在 1,040,696 bytes 界内；两臂 no-op decode/read 为 0 |
| P09 中位 wall（对 B0 / 对 S0） | 14,215.502 → 10,356.735 / 12,147.883 → 10,526.493 ms | −27.1% / −13.3% |
| P10 median | 5,532.680 → 2,469.487 ms | −55.4% |
| P11 首批 / 完整历史 median | 1,375.635 → 1,358.720 / 21,353.423 → 19,663.770 ms | −1.2% / −7.9%；首批比值区间跨 1 |

没有性能数值通过门；不因 P04 或资源变差判 FAIL。本轮 FAIL 仅来自来源证据不满足已批准测量契约。没有发现需要本轮直接修改产品的确证缺陷。按阻断发现停止扩大验证，保留未运行手工/安装态验收及既有并发 Go 超时的限制。

Task 8 Review 保持未通过，返回原任务修复；本轮不跨 Topic 完成边界。Topic 的 CEv1 WorkUnit/contains 关系尚未建立，不能据 Task 8 的旧 gate 声称整个 topic 已完成；在最终任务通过后的包含单元收口中按项目权威处理。

#### 下一步指令

修复：scan-performance / reviews/cross-surface-acceptance.md / CSA-R1-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

### CSA-R1-F1 修复交接 — 2026-10-30

Disposition：CSA-R1-F1 -> repaired in candidate，等待独立复评确认关闭。原 Round 1
FAIL、NOT_VERIFIED 和缺证事实保持不变；本节是修复说明，不是新评审轮次或 PASS。

修复者：Codex；无委派。未修改产品、采样工具或测量值；新增
`scripts/reproduce-csa-tool-builds.py` 保存精确恢复与构建配方，并同步本记录及 Task 8 状态。
CEv1 新观察 `scan-perf-csa-repair-r1-20261030:full-metric-matrix` 显式替代
`scan-perf-csa-review-r1-20261030:full-metric-matrix` 和旧的不完整完成观察，绑定
`scan-perf-csa-repair-r1-20261030-state`；其他四项准则按无产品变化的影响判断复用。

#### 源码与构建来源

以下四个二进制均由恢复源码重新构建，逐字节 SHA-256 与矩阵记录完全一致（4/4）。
这证明恢复源码可生成实际运行的工具；不是仅以相同 dirty HEAD 推断来源。
每版完整源码摘要覆盖 1,643 个文件：`go list -mod=vendor -deps -json ./cmd/agentdeck-perf`
选中的编译/嵌入文件（含 GOROOT 和 vendor），另含 `go.mod`、`go.sum`、`vendor/modules.txt`。
算法为 SHA-256（按路径排序的 `path:sha256(file)` 行以 LF 拼接，无尾 LF）。CEv1 保留
84 个项目/锁文件的逐文件摘要、GOROOT/vendor 分组摘要、工具链二进制摘要及版本依赖。

| 版本 | 二进制 SHA-256 | 完整源码 SHA-256 | 适用批次 |
| --- | --- | --- | --- |
| schema33 | `021fa1b4a49f6f0a8694bb12b94cc9bcfca8a233304891ba8e6dcf3de8ee82cd` | `0d6a24e3de040b6f7362e0a0b7095cb6c77931d192aeea624ff7b23747610ebc` | P04、P11-first、P11-full、P07；每项双臂 pilot/formal |
| layout | `02d2ed597e45a4dfb2bb95d2d4a37756ac2194f0425c531f5336f48be69768f4` | `c04910fd52415bdc9de43233af1a5213898499dca498f14fb33693f90fe2af5d` | P01-first、P01-repeat、P02、P08、P10、P09-B0、P09-S0、P06；每项 pilot/formal，P09 为 paired |
| analysis | `b09e2ed1c058c7d92e60276249ad43de57ae76e333235d8906008b7abb00e42f` | `c9e907eeecabaf02429da5963fd59b52313611dd7a50f542ba87cb7d10aad6d1` | P06 正式结果的最终配对差/中位差分析；其余比较由对应采集版本生成 |
| sync | `593059bee9c7d042e23bfddfc5aa7b47a4b515edbdd1aa94b0a4208dce4c930f` | `c13cd36ace6a655c825114bba107df4033fcb4c568842568acd073628714b3b3` | P01-rebuild、P03；每项双臂 pilot/formal |

公共构建配置：Go 1.27.3，darwin/amd64，GOAMD64=v1，CGO_ENABLED=1，gc/exe，
`go build -mod=vendor -trimpath [-overlay=<version.json>] -o <output> ./cmd/agentdeck-perf`；
无 tags 或 ldflags，GOFLAGS/GOEXPERIMENT 为空，CC=clang，CXX=clang++，
CGO C/C++/link flags 为 `-O2 -g`，CPP flags 为空。内嵌 VCS revision 均为
`27bbe5f65483ff18e4e40ca85518b4a8cf42d3e4`，time=`2026-10-29T17:00:02Z`，modified=true。
原 App 构建工具链仍为 Xcode 26.4 (17E192)，本次未重建 App/helper/framework。

恢复配方以该 HEAD 和当前 Task 8 候选源码为输入，先断言 run/compare/stats 三个输入文件
摘要，再通过 Go overlay 重建，不覆盖工作树源码：sync 使用当前源码；analysis 回退 run.go
相对该 HEAD 的三个重建握手 hunk（原始起始行 846、1039、1051）；layout 再恢复该 HEAD
的 compare.go；schema33 再去掉 `checkUnitOutcome` 中 `invalid.json` 检查与 P08/P09
专用分支（到 `files, err := readSample(dir)` 前）。其余源码均保持相同。
原始修改记录确认了完整布局修复块；早期构建测试记录也没有后补的 P06 中位差测试。
这些历史线索只用于定位；最终来源证明是四个二进制的完整哈希匹配。

复现方式：在记录的 HEAD 上应用对应候选源码，使用已有 Go 1.27.3，运行
`python3 scripts/reproduce-csa-tool-builds.py --go <go1.27.3-executable> --out <new-output-directory>`。
脚本只构建，不启动测量或安装态应用；输出目录必须不存在。输出可重新生成，不作为耐久引用。
构建配方内容 SHA-256 为 `d8aec89d3995e95bd599bde5552ac489c33832b4f1b38c2ee395522280a98da0`。

#### 复用适用性与验证

- schema33 → layout：只修复单元结束后的失败检查路径与分目录读取。旧版所采用的
  P04/P11/P07 不走 P08/P09 专用分支；已接受样本的工具无效数为 0，新增 invalid 检查
  不改变这些样本的采集时刻、端点或统计。早期 P08 失败仍保留，最终 P08/P09 均用 layout。
- layout → analysis：只改 P06 比较输出，排除非 sample 元数据并增加有符号配对中位差；
  不改变采集。用重建的 analysis 对保留 P06 正式输入重新分析，解析 JSON 与最终矩阵
  的 P06 comparison 完全相等；未运行新的 30 分钟窗口，也未覆盖原始比较或样本。
- analysis → sync：只改 P01-rebuild 的就绪请求/启动确认及重建过程控制；对应重建条件
  已在最终矩阵以 sync 双臂重测。P03 同批使用 sync，但不调用重建路径。其他条件不调用
  此路径；统计定义、产品 helper 和既有逐值/生命周期验证均不受影响。
- CEv1 将每个条件的 formal/pilot 逐臂 `perf_tool_sha256` 解析到上述版本；分析来源按
  上表单独绑定，保留原矩阵所有数值、失败、打断和适用性限制，不用新观察抹去旧事实。
- 修复验证仅包含四版精确重建、P06 原始输入重分析、逐臂来源映射及文档/空白检查；
  未因阶段变化重复产品全套测试或性能矩阵。独立复评尚未执行，Task 8 Review 仍未勾选。

复评：scan-performance / reviews/cross-surface-acceptance.md / CSA-R1-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 2 — 2026-10-31

### 📋 Task 8 来源修复独立复评

📊 总体评分：9/10

✅ 结论：PASS

- Reviewer：OpenCode / GPT-6 Astra；独立于修复实现，无委派。
- Method：限定 finding 的源码检查、四版离线重建、CEv1 结构化逐项对照与独立统计复算；沿用首轮未变检查。
- Scope：CSA-R1-F1，复现脚本、来源清单、52 个 pilot/formal 逐臂绑定及版本复用判断。
- Reviewed state：HEAD `27bbe5f65483ff18e4e40ca85518b4a8cf42d3e4`；入口候选指纹
  `914a6abf41c31c64ddbc1265b22dbb590dc77ffc82d51c4198e44f06432fa9c5`，23 个逐文件 blob 全部匹配。
  本轮仅追加评审与同步 tasks 状态；同步后身份由 `scan-perf-csa-rereview-r2-20261031-state`
  的 scoped_files 与同一指纹算法固定，不将入口指纹冒充同步后的身份。
- WorkUnit：`scan-performance:cross-surface-acceptance`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。CSA-R1-F1 已关闭；没有仍未解决、回归或新增的阻断 finding。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- CSA-R1-F1 → CLOSED：独立运行 `python3 scripts/reproduce-csa-tool-builds.py --go <go1.27.3-executable> --out <new-output-directory>`，
  schema33/layout/analysis/sync 四版完整二进制 SHA-256 均精确匹配修复表，完整源码摘要亦 4/4 匹配。
  每版 1,643 个文件；84 个项目/锁文件逐项摘要、GOROOT/vendor 分组摘要（vendor/modules.txt 属 vendor 组）、
  工具链二进制、构建配置和内嵌 build info 均与 CEv1 独立比较一致。
- 从矩阵自身枚举全部 formal/pilot 臂，与 batch_bindings 做集合相等检查：52/52，无遗漏、重复或未知工具。
  每臂 collector SHA 与版本清单一致；P06 formal 单独绑定 analysis，其他比较绑定各自采集版本。
- 修复后的 measurement_matrix 与首轮权威矩阵完整 JSON 相等，14 个正式条件、原始失败和限制未被改写。
  用矩阵保存的 P06 两臂逐样本数值独立计算配对差与中位差，全部相等；本轮未声称重新读取原始 trace。
- 源码差异核对支持既有复用判断：schema33→layout 只改变结束检查；layout→analysis 只改变 P06 输出；
  analysis→sync 只改变重建握手。受重建影响条件已使用 sync，其余条件不走该分支。

### 📝 总结

唯一 finding CSA-R1-F1 已由可精确重建的源码/构建身份和完整逐臂映射关闭，复评 PASS。
修复候选的标准门禁五项均通过；旧不完整 pass 和首轮 not_verified 被显式 supersede，保留历史。
本轮状态同步不改变产品、测试、工具、依赖或测量，五项通过证据按范围评估绑定至同步后的新状态，
原观察仍保留原身份。文档结构、评审记录结构与 `git diff --check` 为本轮收口检查。

保留限制：首次 P06 原偏好未知、验收域保持关闭；原默认并发 Go 超时根因未知；未追加性能批次、
安装态 Widget/手工 schema/L4 验收。性能不存在数值通过阈值，PASS 不等于用户认可整体提升。

Task checkpoint：`ad-scan-performance-cross-surface-acceptance`；内容状态 `scan-perf-csa-rereview-r2-20261031-state`；门禁 VERIFIED。
提交建议：经用户授权后单独提交 Task 8 产品、测试、契约、复现脚本及本轮评审/状态文件；交付前核对最终指纹与签名。
推送建议：提交并核验提交态门禁后，经单独授权推送 `feature/scan-performance`；远端目标需交付时核实。

包含单元边界：tasks.md 当前八个任务均已评审；Topic `scan-performance` 的层级与聚合门禁独立收口。
Topic 尚不能宣布完成：requirements.md 的整体收益判断属于用户，Task 8 的证据不能替代该决定；
此前各 Task 对最终专题内容的聚合适用性也必须明确，不以 Review 勾选代替证据。

#### 用户验收与包含单元收口 — 2026-10-31

用户在明确询问“是否接受当前结果及上述退化，完成 Topic 验收”后答复“接受”。
据此接受当前性能对比，包括首击 median −60.1%、P10 median −55.4%，以及 P04 median +17.3%、
RSS 增加等已披露取舍；不增加数值阈值，不抹去历史失败或测量限制。

Topic 已建立当前八个 Task 的 contains 层级；跨任务验收和各 Task 的证据聚合已通过规范门禁，
唯一剩余的 `scan-performance:user-performance-acceptance` 由本次真实用户决定补齐。
本次仅同步两份状态/评审文件；产品、测试、工具、契约、测量和依赖未变，复用已有通过证据，未重测。

Unit completion checkpoint：`scan-performance`；验收内容状态 `scan-perf-topic-accepted-20261031-state`；
Topic 验收结果 VERIFIED。该状态同时承接 Task 8 的五项通过证据。
Round 2 的入口状态、结论与历史门禁保持原义；本节为后续用户验收决定，不是新一轮复评。
提交建议：经单独授权交付 Task 8 的已评审变更及验收记录，并核验提交态。
推送建议：提交态核验后再经单独授权推送 topic 分支；本次确认不授权提交、推送、合并、归档或发布。
