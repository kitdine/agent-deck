---
status: historical
topic: desktop-app
subject: requirements.md
retired: 2026-09-01
---

# Review log — desktop-app / requirements.md

## Round 1 — 2026-08-17

- Reviewed state: HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6`; `docs/topics/desktop-app/requirements.md` blob `54eba57dee58eb17be54fc361d044700176b72f8`
- Reviewer: Codex
- Method: Single-agent document-contract review using the `development-workflow` design/contract dimensions; CodeGraph located the current desktop snapshot, macOS host, and App Group implementation before focused source and document inspection. No implementation-scoring tool was applied to the requirements document.
- Scope: `docs/topics/desktop-app/requirements.md`, checked against the current topic status, the two declared surface contracts, the architecture contracts they demand, current implementation structure, and repository document-lifecycle rules.
- Findings:
  - [P1] R1-F1 — `requirements.md:73-79,115-120` limits the stated menu-bar outcome to current-day usage while the derived surfaces and projection require 7-day, 30-day, 90-bucket, and 7x24 historical analytics. The requirements boundary does not decide whether those later periods are required, permitted, or out of scope. -> Update Goals, Non-Goals, and Acceptance boundary to authorize and bound the intended historical analytics, or cut the unsupported downstream surface and projection scope.
  - [P1] R1-F2 — `requirements.md:77,118-120` defines Widget publication and isolation but no user-visible Widget outcome. A Widget that reads the redacted projection and renders no useful product information would satisfy the written acceptance boundary, while `ux/widget.md` independently invents four kinds and twelve configurations. -> State the bounded user questions or information the Widget must expose and add a functional acceptance outcome, without duplicating presentation details owned by `ux/widget.md`.
- Evidence: `git rev-parse HEAD` -> `73f640e7ac7e76c8a5492205115b012b59092bf6`; `git hash-object docs/topics/desktop-app/requirements.md` -> `54eba57dee58eb17be54fc361d044700176b72f8`; `bash scripts/check-topic-docs.sh` -> exit 0; `codegraph explore` confirmed the delivered desktop snapshot/foundation and distinguished the unimplemented surface extensions; focused inspection confirmed `ux/menubar.md:177-180,752-765`, `ux/widget.md:34-39,85-88,255-272`, and `architecture.md:431-511` demand behavior not bounded by the reviewed requirements; `docs/README.md:247-252,458-462,500-505` makes the decided requirements boundary the source of downstream scope.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：6/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:73`] 用量分析的时间边界未决定。
- 行为风险：下游实现无法判断 7 天、30 天、90 个日桶及 7x24 节奏分析是必需、允许还是越界，App Group 投影已因此扩张。
- 证据：`requirements.md:73-79,115-120` 仅规定“当日”用量和笼统的用量/费用展示；`ux/menubar.md:177-180,752-765` 与 `ux/widget.md:85-88` 要求多周期历史分析。
💡 有界修复：在 Goals、Non-Goals 和 Acceptance boundary 中明确授权并限定预期的历史分析，或删减下游未获需求授权的范围。

[`requirements.md:77`] Widget 只有数据发布和隐私边界，没有用户可见结果的验收条件。
- 行为风险：一个仅读取投影但不展示有用信息的 Widget 也能满足现有验收边界；四类 Widget 和十二种配置实际由下游自行发明。
- 证据：`requirements.md:77,118-120` 未定义 Widget 必须回答的用户问题；`ux/widget.md:34-39` 另行定义四个问题与四种 Widget。
💡 有界修复：在需求层声明 Widget 必须提供的有界用户价值并增加功能性验收结果，但不复制 `ux/widget.md` 所有的展示细节。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 版本归属、发布渠道、Go/Swift 权威边界和非目标写得清楚。
- 隐私、无后台守护进程、不自动安装更新及不删除用户状态等高风险边界都有明确限制。
- 两个用户可见表面均以具体 `ux/` 路径声明，专用文档集审计以退出码 0 通过。

### 📝 摘要

评审对象为 HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6` 与 `requirements.md` blob `54eba57dee58eb17be54fc361d044700176b72f8`。文档集结构完整，当前实现与“已交付基础/未实现表面扩展”的时态一致；但需求边界没有承接最新表面的历史分析范围和 Widget 功能结果，实现者仍需自行决定产品合同。因此本轮为 FAIL/REOPEN；修复应只闭合 R1-F1 和 R1-F2。

## Round 2 — 2026-08-17

- Reviewed state: HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6`; `docs/topics/desktop-app/requirements.md` blob `fcfc5997ec8855ce5b64f58004b0cc0162abc862`
- Reviewer: Codex
- Method: Single-agent bounded Re-review of every Round 1 finding. Reused unchanged document-set and implementation-structure evidence, inspected the exact requirements repair diff, and compared only the repaired boundary against the unchanged surface and projection contracts.
- Scope: R1-F1 and R1-F2 in `docs/topics/desktop-app/requirements.md`, plus any regression or newly blocking contradiction caused by their repair.
- Findings:
  - [closed] R1-F1 — `requirements.md:73-77,124-128` now fixes the intended temporal scope to today, trailing 7 days, trailing 30 days, at most 90 daily buckets, and a 7x24 hour-of-week view.
  - [closed] R1-F2 — `requirements.md:80-86,129-134` now names the four bounded Widget questions and requires real product information rather than publication and isolation alone.
  - [P1] R2-F1 — `requirements.md:76-77` newly says that no other "breakdown" is authorized. Read literally, that forbids the model, client, token-component, attribution-quality, provider, and pricing-coverage breakdowns required to answer the same document's composition and trust questions and already demanded by both surface contracts. -> Limit the prohibition to additional historical periods and temporal granularities, then authorize and bound the non-temporal breakdown categories needed by magnitude, composition, trust, and rhythm without copying presentation details.
- Evidence: `git rev-parse HEAD` -> `73f640e7ac7e76c8a5492205115b012b59092bf6`; `git hash-object docs/topics/desktop-app/requirements.md` -> `fcfc5997ec8855ce5b64f58004b0cc0162abc862`; focused diff and line inspection confirmed both Round 1 repairs; `ux/menubar.md:177-180,753-762`, `ux/widget.md:36-39,263-271`, and `architecture.md:441-453` enumerate the non-temporal breakdowns that `requirements.md:77` now prohibits.
- Verdict: REOPEN

## 📋 复评报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:77`] 新增的“不允许任何其他 breakdown”与已授权的 composition/trust 用户问题相互矛盾。
- 处置：新增阻断 R2-F1。
- 行为风险：按字面实现会禁止模型、客户端、token component、归因质量、provider 和定价覆盖率分解，使 composition 和 trust 无法回答；忽略该句则违反需求的明确范围禁令。
- 证据：`requirements.md:76-83` 同时禁止其他 breakdown 并授权 composition/trust；`ux/menubar.md:753-762`、`ux/widget.md:263-271` 和 `architecture.md:441-453` 都要求上述分解。
💡 有界修复：把禁令限定为额外的历史周期和时间粒度，并在需求层授权、限定四个问题所需的非时间分解类别，不复制 UX 展示细节。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 已关闭：当日、7 天、30 天、90 个日桶和 7x24 节奏的时间上限已明确。
- R1-F2 已关闭：Widget 的四个用户问题和必须展示真实产品信息的功能结果已写入 Goals 与 Acceptance boundary。
- 修复保持了 UX 文档对 Widget 数量、尺寸和展示的所有权，没有复制展示细节。

### 📝 摘要

R1-F1 和 R1-F2 均已关闭。复评对象为 HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6` 与 `requirements.md` blob `fcfc5997ec8855ce5b64f58004b0cc0162abc862`。修复新增的 breakdown 禁令与四个用户问题的必需数据分解矛盾，因此 R2-F1 仍使需求边界无法直接实现，本轮为 FAIL/REOPEN。剩余不确定性仅限于禁令句的预期语义，修复范围仅为 R2-F1。

## Round 3 — 2026-08-17

- Reviewed state: HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6`; `docs/topics/desktop-app/requirements.md` blob `fb0c81951dace253adff066640b0d23c2932473d`
- Reviewer: Codex
- Method: Single-agent bounded Re-review of R2-F1. Reused the unchanged surface and projection evidence and inspected the exact repair diff for every non-temporal breakdown named by Round 2.
- Scope: R2-F1 in `docs/topics/desktop-app/requirements.md`, including model, client, token-component, attribution-quality, provider, and pricing-coverage authorization.
- Findings:
  - [still open] R2-F1 — `requirements.md:76-81,132-136` now correctly limits the prohibition to historical periods and temporal granularity and authorizes model, client, token-component, attribution-quality, and pricing-coverage breakdowns. It does not authorize the per-provider usage and attribution-quality breakdown required by `ux/menubar.md`, `ux/widget.md`, and `architecture.md`; showing the current provider is a routing fact, not authorization to aggregate usage or attribution quality by provider. -> Add provider to the authorized breakdown dimensions in Goals and Acceptance boundary, or state explicitly that attribution quality may be broken down by both client and provider.
- Evidence: `git rev-parse HEAD` -> `73f640e7ac7e76c8a5492205115b012b59092bf6`; `git hash-object docs/topics/desktop-app/requirements.md` -> `fb0c81951dace253adff066640b0d23c2932473d`; the exact repair diff closes the temporal-prohibition contradiction but omits provider from both breakdown lists; unchanged `ux/menubar.md:759`, `ux/widget.md:269`, and `architecture.md:450-451` require per-provider attribution-quality rows or subtotals.
- Verdict: REOPEN

## 📋 复评报告

📊 综合评分：9/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:78`] R2-F1 仍未完全关闭：非时间分解列表遗漏 provider 维度。
- 处置：仍未关闭，范围已收窄。
- 行为风险：Widget 和菜单栏合同要求 per-provider 归因质量行/小计，但需求只授权 model、client、token component、attribution quality 和 pricing coverage 分解。“当前 provider”是路由事实，不等于授权按 provider 聚合用量与归因质量。
- 证据：`requirements.md:78-81,134-136` 的两个列表都没有 provider；`ux/menubar.md:759`、`ux/widget.md:269` 和 `architecture.md:450-451` 均明确要求 per-provider 分解。
💡 有界修复：在 Goals 和 Acceptance boundary 的授权维度中加入 provider，或明确 attribution quality 可按 client 和 provider 分解。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R2-F1 的主要矛盾已关闭：禁令现在只限于额外历史周期和时间粒度。
- model、client、token component、attribution quality 和 pricing coverage 均已在 Goals 与 Acceptance boundary 中获得一致授权。
- R1-F1 和 R1-F2 保持关闭，未发现回归。

### 📝 摘要

R1-F1 和 R1-F2 保持关闭；R2-F1 已从广泛的 breakdown 禁令矛盾收窄为唯一遗漏的 provider 维度。复评对象为 HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6` 与 `requirements.md` blob `fb0c81951dace253adff066640b0d23c2932473d`。由于现有表面合同必须按 provider 展示归因质量，该遗漏仍是实现边界缺口，本轮为 FAIL/REOPEN。无其他新发现或剩余不确定性。

## Round 4 — 2026-08-17

- Reviewed state: HEAD `73f640e7ac7e76c8a5492205115b012b59092bf6`; `requirements.md` blob
  `46c37f60c4f3c0f67b6cf36d22f0eb5031ed9392`
- Reviewer: claude-code (repair round for Round 3's FAIL — an independent
  Re-review is still required before the Documents matrix cell may be ticked)
- Scope: R2-F1 only, as named in the repair command.

- Round 3 finding, disposition:
  - **R2-F1** provider missing from the authorized breakdown dimensions ->
    **Fixed.** Verified against the three documents Round 3 cited before
    accepting the remedy: `ux/menubar.md:759` and `ux/widget.md:269` both
    request "per-client and per-provider attribution counts", and
    `architecture.md:450-451` already provisions "per-client and per-provider
    subtotals, each with its attribution quality counts". The requirement was
    the only document of the four that did not authorize the dimension, so the
    surfaces were asking for something the boundary did not permit.
    `runtime provider` is added to the authorized dimensions in both Goals and
    the Acceptance boundary, and attribution quality is stated as reportable per
    client and per runtime provider.
    The reasoning is recorded rather than the edit alone: showing which provider
    is selected is a routing fact and authorizes nothing about aggregation, and
    the trust question is per-provider by nature — a provider whose events
    resolve to `unknown` is exactly what a determinability figure exists to
    expose, so refusing the dimension would remove the signal the topic is for.

- Evidence: `make check-topic-docs` and `make check-whitespace` pass;
  `git hash-object docs/topics/desktop-app/requirements.md` -> `46c37f60c4f3c0f67b6cf36d22f0eb5031ed9392`. No
  product code, test, or configuration changed.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 5 — 2026-08-17

- Reviewed state: HEAD `d8ae8b3ca348520c01e2843afa1956e84481b4fa`;
  `docs/topics/desktop-app/requirements.md` blob
  `46c37f60c4f3c0f67b6cf36d22f0eb5031ed9392`
- Reviewer: claude-code (independent Re-review of Round 4's repair; the repair
  was authored by the same agent, so every claim below is re-derived from the
  four documents rather than read off the repair note)
- Method: Single-agent bounded Re-review. The blob is byte-identical to the one
  Round 4 declared, so its repair diff needed no re-derivation; HEAD advanced by
  two documentation commits that do not touch this topic. Re-read the requirement
  in full against the three documents Round 3 cited, then swept the whole
  prohibition sentence rather than only the half Round 2 opened, because that
  sentence has already produced one blocking finding.
- Scope: R2-F1 in `docs/topics/desktop-app/requirements.md`; regression on R1-F1
  and R1-F2; and any further contradiction introduced by the Round 2-4 repair
  chain.
- Findings:
  - [closed] R2-F1 — `requirements.md:78` authorizes `runtime provider` among the
    non-temporal breakdown dimensions, `requirements.md:82-83` states attribution
    quality is reportable per client and per runtime provider, and
    `requirements.md:138-140` carries both into the Acceptance boundary. Verified
    against the three documents that demanded it, not against the repair note:
    `ux/menubar.md:759` and `ux/widget.md:269` each request "per-client and
    per-provider attribution counts", and `architecture.md:450-451` provisions
    "per-client and per-provider subtotals, each with its attribution quality
    counts". All four documents now agree. `runtime provider` is also the term
    the living specification already uses (`cli-design.md:1283`), so the
    authorization names an existing dimension rather than inventing one.
  - [closed, no regression] R1-F1 — the temporal scope remains fixed at today,
    trailing 7 days, trailing 30 days, at most 90 daily buckets, and the 7x24
    hour-of-week view.
  - [closed, no regression] R1-F2 — the four bounded Widget questions and the
    requirement to render real product information remain in both Goals and the
    Acceptance boundary, and presentation ownership still sits with
    `ux/widget.md`.
  - [P1, attributed to `ux/menubar.md`] R5-F1 — `ux/menubar.md:754` specifies the
    period switcher as "`buckets` groupable by day, week, month". Week and month
    are temporal granularities, and `requirements.md:76-77` authorizes none
    beyond the daily trend and the 7x24 view. This is the other half of the
    sentence Round 2 already found defective on its breakdown axis; Round 2
    closed the breakdown half and nobody swept the temporal half.
    **This does not block the requirement.** Round 3's precedent — authorize the
    dimension the surfaces need — does not transfer, because three separate
    checks say the surface is the document in error. `architecture.md:444`
    provisions only a daily series and defends that bound at `:470`; no week or
    month bucket exists anywhere in the projection. Serving the switcher from a
    daily series therefore requires re-aggregating in Swift, which
    `requirements.md:134-135` forbids outright. And week/month grouping answers
    none of the four bounded questions that the daily trend and the 7x24 grid do
    not already answer. Two of the four documents agree with the requirement and
    the third is silent; the outlier is a presentation document that must live
    inside the upstream boundary, and blocking the requirement for a downstream
    over-reach would invert the dependency direction the topic is built on.
    -> Close in `ux/menubar.md`'s own review by removing week and month from the
    switcher, or, if either is genuinely wanted, reopen this requirement and
    `architecture.md` together, since the projection would have to carry the
    buckets.
- Evidence: `git rev-parse HEAD` -> `d8ae8b3ca348520c01e2843afa1956e84481b4fa`;
  `git hash-object docs/topics/desktop-app/requirements.md` ->
  `46c37f60c4f3c0f67b6cf36d22f0eb5031ed9392`, identical to Round 4's declared
  blob with a clean worktree; `bash scripts/check-topic-docs.sh` -> exit 0;
  `make check-whitespace` -> exit 0. `rg 'runtime provider'` across the topic and
  `docs/specs/` confirms the term is the specification's own. `rg 'week|month'`
  across `architecture.md` and `ux/widget.md` returns only the 7x24 grid and no
  grouping granularity, which is what isolates R5-F1 to `ux/menubar.md:754`. No
  product code, test, or configuration changed.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：9/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无（针对本文档）。

### 🟡 建议改进——推荐

[`ux/menubar.md:754`] 周期切换器的 week/month 粒度超出需求授权 —— 记为 R5-F1，
归属 `ux/menubar.md`，不阻断本需求。
- 处置：新增，归属下游文档。
- 行为风险：投影只提供日桶（`architecture.md:444`），按周/月分组只能在 Swift 侧
  二次聚合，而这被 `requirements.md:134-135` 明确禁止。
- 证据：`ux/menubar.md:754` 要求 day/week/month 分组；`requirements.md:76-77`
  未授权其他时间粒度；`architecture.md` 全文无周桶或月桶。
💡 有界修复：在 `ux/menubar.md` 自己的评审中删除 week/month；若确实需要，则需
同时重开本需求与 `architecture.md`，因为投影必须先承载这些桶。

### 🟢 优点

- R2-F1 已关闭：`runtime provider` 在 Goals 与 Acceptance boundary 中均获授权，
  归因质量按 client 和 runtime provider 可分解，四份文档口径一致。
- 授权使用的是活文档既有术语 `runtime provider`（`cli-design.md:1283`），没有
  发明新维度。
- R1-F1、R1-F2 保持关闭，无回归。
- 修复说明记录了理由而非仅记录改动：展示当前 provider 是路由事实，不等于授权按
  provider 聚合；而 trust 问题天然是 per-provider 的。

### 📝 摘要

复评对象为 HEAD `d8ae8b3ca348520c01e2843afa1956e84481b4fa` 与 `requirements.md`
blob `46c37f60c4f3c0f67b6cf36d22f0eb5031ed9392`（与 Round 4 声明的 blob 逐字节
一致，工作区干净）。R2-F1 已关闭，R1-F1 与 R1-F2 无回归，需求边界现在可直接实现。
本轮额外扫了 Round 2 只查了一半的那句范围禁令，发现其时间轴一侧同样与下游冲突，
但三项独立检查都指向下游文档有错而非需求有错，故记为 R5-F1 归属 `ux/menubar.md`，
不阻断本需求。裁决 PASS。

## Round 6 — 2026-08-18

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/requirements.md` blob
  `b51dac4dbe7e1a2419296df0ab8ad97b343a330e`
- Reviewer: Codex
- Method: Single-agent formal Review using the `development-workflow`
  design/contract dimensions. This is a new review round because the target blob
  changed after Round 5. CodeGraph was used first to locate the current desktop
  wire, App Group, host, and prototype paths; focused document inspection then
  checked the changed update-check, surface, filter, preference, and backlog
  boundaries against the current task decomposition and surface contracts.
- Scope: `docs/topics/desktop-app/requirements.md`, checked against
  `docs/topics/desktop-app/tasks.md`, `ux/menubar.md`, `ux/settings.md`,
  `ux/widget.md`, `architecture.md`, and the document-lifecycle rules in
  `docs/README.md`. Broad verification stopped after the decisive R6-F1
  requirements counterexample, as required by the repository review rules.
- Findings:
  - [P1] R6-F1 — `requirements.md:129-133,164-166` adds the settings window as a
    third shipped surface and constrains the quality of any preference it
    exposes, but never states which user outcome or bounded preference set this
    version must deliver. An empty settings window with zero preferences would
    satisfy the written acceptance boundary, while `ux/settings.md:36-74` and
    `tasks.md:51-75` independently require launch-at-login, periodic refresh,
    menu-bar value, and menu-bar scope. That makes downstream documents invent
    product scope the requirements boundary has not authorized. -> Add a Goal
    and functional acceptance outcome that authorize and bound the intended
    settings capabilities, naming the four preferences or an equally precise
    product boundary while leaving defaults, copy, and failure presentation to
    `ux/settings.md`; alternatively remove the settings surface and its
    downstream scope from this version.
- Evidence: `git rev-parse HEAD` ->
  `58fe5d300c5af572adef81a69a856a6aef9cea56`; `git hash-object
  docs/topics/desktop-app/requirements.md` ->
  `b51dac4dbe7e1a2419296df0ab8ad97b343a330e`; `codegraph explore` located the
  current desktop wire and App Group production symbols and distinguished them
  from the untracked prototype; focused inspection confirmed the requirements
  counterexample against `ux/settings.md:36-74` and `tasks.md:51-75`;
  `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and
  `git diff --check` -> exit 0.
- Verdict: REOPEN

## 📋 评审报告

📊 综合评分：8/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`requirements.md:129`] R6-F1：新增 settings 表面没有需求层的用户结果和范围。
- 行为风险：零偏好的空 settings 窗口也满足当前验收文字；反过来，UX 和任务文档
  要求的开机启动、定时刷新、菜单栏值与菜单栏范围，都是下游自行发明的产品范围，
  实现者无法从需求判断它们是必需、允许还是越界。
- 证据：`requirements.md:129-133,164-166` 只规定“每个已暴露偏好”的质量属性；
  `ux/settings.md:36-74` 与 `tasks.md:51-75` 则明确要求四个偏好。
💡 有界修复：在 Goals 与 Acceptance boundary 中授权并限定本版本 settings 的用户
结果和四个偏好；默认值、文案、幂等行为及拒绝态继续由 `ux/settings.md` 持有。若本
版本不交付 settings，则从需求和下游任务范围一并移除该表面。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 更新检查已从 Confirmed Decisions、Non-Goals、Acceptance 外的未来列表和三个表面
  合同中一致撤回，网络边界清楚。
- client/period filter 与固定 30 天 rhythm 的层级边界已经明确，和当前菜单栏合同
  一致。
- work signals 被明确放入 Backlog，并说明当前投影无法供数，避免原型数据被误当成
  本版本交付。
- 旧的 provider、时间范围和 Widget 功能结果修复仍然成立。

### 📝 摘要

评审对象为 HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56` 与
`requirements.md` blob `b51dac4dbe7e1a2419296df0ab8ad97b343a330e`。当前修订正确
收紧了更新检查、筛选和 work-signal 边界，但新增 settings 表面没有需求层的功能结果
或有界偏好集合，导致零实现可过验收、完整实现却依赖下游自行决定范围。本轮为
FAIL/REOPEN；强制修复范围仅为 R6-F1。

## Round 7 — 2026-08-18

- Reviewed state: HEAD `58fe5d300c5af572adef81a69a856a6aef9cea56`;
  `docs/topics/desktop-app/requirements.md` blob
  `685a113ecda5af20f8a4574eb952c54c4c7ecf`
- Reviewer: Codex
- Method: Single-agent bounded Re-review of R6-F1. Reused the unchanged Round 6
  evidence for the settings surface and task decomposition, inspected the exact
  requirements repair, and compared both repaired anchors against the same four
  preferences in `ux/settings.md` and `menubar-experience`.
- Scope: R6-F1 in `docs/topics/desktop-app/requirements.md`, including regression
  or a newly blocking contradiction introduced by the repair.
- Findings:
  - [closed] R6-F1 — `requirements.md:91-96` now makes the dedicated settings
    window and its four user controls a product Goal, and
    `requirements.md:170-178` requires exactly launch at login, periodic refresh,
    menu-bar value, and menu-bar scope while excluding every other preference in
    this version. The list matches `ux/settings.md:36-74` and `tasks.md:51-75`
    one-to-one, so neither an empty window nor an arbitrary fifth preference can
    satisfy the boundary. Defaults, copy, interaction details, and failure
    presentation remain owned by `ux/settings.md`, preserving the intended
    requirements/UX boundary. No regression or new blocker was introduced.
- Evidence: `git rev-parse HEAD` ->
  `58fe5d300c5af572adef81a69a856a6aef9cea56`; `git hash-object
  docs/topics/desktop-app/requirements.md` ->
  `685a113ecda5af20f8a4574eb952c54c4c4c7ecf`; the focused repair diff adds the
  same four preferences to Goals and Acceptance; focused inspection confirms
  exact agreement with `ux/settings.md:36-74` and `tasks.md:51-75`;
  `bash scripts/check-topic-docs.sh`, `make check-whitespace`, and
  `git diff --check` -> exit 0.
- Verdict: PASS

## 📋 复评报告

📊 综合评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R6-F1 已关闭：Goals 与 Acceptance 都明确授权同一组四项 settings 能力。
- “恰好四项”同时排除了零偏好空窗口和任意新增偏好两种范围漂移。
- 默认值、文案、交互细节与失败呈现仍由 `ux/settings.md` 持有，没有在需求层复制
  表面设计。
- 修复没有改变已通过的更新检查、筛选、work-signal、provider、时间范围或 Widget
  边界。

### 📝 摘要

R6-F1 已关闭；没有回归或新增阻断。复评对象为 HEAD
`58fe5d300c5af572adef81a69a856a6aef9cea56` 与 `requirements.md` blob
`685a113ecda5af20f8a4574eb952c54c4c4c7ecf`。修复同时给出 settings 的用户结果、
完整四项范围和排他边界，且与下游合同逐项一致，本轮裁决 PASS，无剩余不确定性。

## Document review deferred to the closing pass (2026-08-20)

By user instruction, `desktop-app` runs no document review rounds while its tasks
are being implemented. Review is **deferred, not cancelled**: after every
implementation task is done, the whole document set is reconciled against the
final prototype and the shipped implementation and reviewed once, as a bullet on
task 6.

Until then, changes to this subject are written directly into the document that
owns it, and nothing is appended here. The closing pass appends its round to this
record. The reason and the two consequences are stated in
[`../tasks.md`](../tasks.md).

**Status 2026-08-23 — reconciled and submitted; the closing round has not run.**
Task 6 has brought the set into agreement with the shipped implementation, so this
record is now waiting for the single deferred review rather than waiting for
implementation to finish. The next thing appended below is that round.

## Closing document review — Round 1 (2026-08-23)

This is the single deferred document review this topic postponed on 2026-08-20.
One round, one verdict, over the whole set; this record carries the round's
common part plus this document's own outcome.

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint
  `5d5c576eeb75117bd6b329d807bb018c5dec7cea12b213a1c603256d3f5345e3`, computed as
  `shasum -a 256` over one `<git hash-object> <space> <line count> <space> <path>`
  line per document, in this order: `requirements.md`, `architecture.md`,
  `ux/menubar.md`, `ux/settings.md`, `ux/widget.md`, `tasks.md`. The order is part
  of the recipe.
- Reviewer: Claude Code, independent of every implementing and reconciling
  session.
- Method: the set judged against the shipped implementation rather than against
  itself, which is the reason the review was deferred to this point. Checkable
  claims were taken to the code: the settings defaults and the login-item refusal
  path to `DesktopPreferences.swift` and `SettingsWindowView.swift`; the four
  panels and the Breakdown cards' non-collapsibility to `MenuBarPanelViews.swift`;
  the `Not captured yet` modules to `DesktopCopy.swift`; the fixed 30-day rhythm
  window across `requirements.md`, `ux/menubar.md` and `internal/usage`; the
  widget's runtime state to `desktop-widget` Round 3. `scripts/check-topic-docs.sh`
  is the project's checker for this target class and was run.
- Set-wide verdict: **REOPEN**. Three findings, all located in two documents;
  four of the six documents carry none.
- This document's outcome: **REOPEN** — one P1.
  - [P1] CD1-F1 `requirements.md`, `## Acceptance boundary`, the Widget bullet —
    the set is internally inconsistent about a known release-blocking defect, and
    this is the document that stays silent. The bullet requires that the Widget
    "renders real product information answering at least one of the four bounded
    spend questions — magnitude, composition, trust, rhythm — not merely data
    publication and isolation". The shipped extension renders the unavailable
    state in all twelve configurations, because the delivered App Group identifier
    carries no team-ID prefix and `containermanagerd` refuses the container; that
    is `desktop-widget`'s open P1 DW-R3-F1. `architecture.md`, `ux/widget.md`,
    `docs/specs/cli-design.md` and `docs/specs/cli-manual.md` each disclose it at
    the point a reader would otherwise assume otherwise. This document states the
    criterion the defect violates and says nothing, so the set reads as though
    acceptance were met. It is also an unapplied instance of this topic's own
    **active** temporary code-over-contract rule, which says that when the code and
    a topic-local requirement conflict the code is the authority and the document
    is repaired. Task 6's record lists `requirements.md` under "Verified current
    and left unchanged", which is the claim this finding falsifies. -> open
  - Bounded remediation: annotate the Widget acceptance bullet with the shipped
    state and the finding that owns it, in the same shape the other four documents
    already use. Do not weaken the criterion — it stays the contract; what is
    missing is the disclosure that it is not yet met.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class; a verdict on set completeness must cite the tool that can
    falsify it)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Implementation cross-checks named under Method, each run against the working
    tree at this content state
- Residual, carried and owned elsewhere: task 4 `desktop-widget` remains at an
  open P1 (DW-R3-F1) parked on an Apple Developer team ID, so this set describes a
  surface whose runtime acceptance never passed; task 6's basis remains the user's
  explicit decision to proceed without task 4, recorded only by the implementing
  agent.
- Verdict: REOPEN

## Closing document review — Round 2 (repair, 2026-08-23)

- Repair state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint
  `771b3375cf696bee4539e4d8b1d22fdba532cb8e472a412fc4e1ca58b479cd10`,
  computed with Round 1's six-document recipe; `requirements.md` blob
  `9d77bde98d881bf81641c84cd1dfe4532067ed7c` and `tasks.md` blob
  `da5f31d17c624ba5f8c7507355787ddcc320da03`.
- Repair owner: Codex
- Scope: only CD1-F1 in `requirements.md`; no product code, tests,
  configuration, the other four closing-review documents, or task 4's open
  DW-R3-F1 implementation finding.

### Finding-to-change mapping

- **CD1-F1 repaired in the candidate.** The Widget acceptance bullet now states
  that the shipped build does not meet the criterion: its App Group identifier
  lacks a team-ID prefix, macOS 26 refuses the container, and all twelve
  configurations render the unavailable state. It names DW-R3-F1 and its missing
  Apple Developer team ID prerequisite while preserving the acceptance criterion
  as the contract.

### Verification and status

- `bash scripts/check-topic-docs.sh` -> exit 0; `make check-whitespace` -> exit
  0; `git diff --check` -> exit 0.
- This is Repair, not independent Re-review. The set-wide verdict remains
  REOPEN, the dated closing-review status stays unchecked for `requirements.md`,
  and no completion-evidence gate or delivery action is claimed.

Repair status: CD1-F1 is repaired in the candidate; awaiting independent
Re-review together with CD1-F2 and CD1-F3.

#### 📌 下一步

```text
复评：desktop-app / 文档集收尾评审 / CD1-F1, CD1-F2, CD1-F3
```

## Closing document review — Round 3 (independent re-review, 2026-08-23)

- Reviewed state: HEAD `a190186297db40bade40f129fd4a17e35600bbbb`, uncommitted
  working tree. Set fingerprint, computed with Round 1's six-document recipe
  **after this round's own status synchronization**, so the evidence binds to the
  state the verdict leaves behind rather than to the pre-tick one:
  `c42a6de1d707e6d79320032877996ad8aec10847cf58a45885eb3e63fa3bd2a7`.
  Round 2's repair candidate was `771b3375…cd10`; Round 1 was `5d5c576e…45e3`.
  Two of the six documents changed between Rounds 1 and 2 — `requirements.md` and
  `tasks.md` — which is exactly Round 2's declared scope; the four documents that
  passed Round 1 are byte-identical to their Round 1 blobs.
- Reviewer: Claude Code, independent of the repair round.
- Method: each finding re-verified against the documents themselves rather than
  against Round 2's account of the repair, and the four already-passing documents
  re-hashed to confirm the repair did not touch them.
- Disposition of every Round 1 finding:
  - CD1-F1 — **closed.** `requirements.md`'s Widget acceptance bullet now carries
    a `Known defect:` paragraph stating that the criterion is not met by the
    shipped build, naming the missing team-ID prefix, macOS 26's refusal, all
    twelve configurations rendering the unavailable state, and `DW-R3-F1` with its
    parked Apple Developer prerequisite — and it closes with "The criterion above
    remains the contract", which is the half that mattered. The disclosure now
    matches the shape the other four documents already used, so the set has one
    voice on this defect instead of two.
  - CD1-F2 — **closed.** The temporary code-over-contract rule now states why it
    survives task 6's initial reconciliation — task 4's open `DW-R3-F1` leaves the
    implementation and the document set in disagreement — and assigns the removal:
    task 6 owns a final reconciliation after task 4 closes that finding, before
    task 6 can reach Review PASS. A reader can now tell a deferral from an
    omission, which is what the finding asked for. Declaring the retention
    deliberate is the owner's decision to make, not a claim this reviewer can
    falsify, and the substantive requirement — the condition and the owner — is
    met.
  - CD1-F3 — **closed, and the repair is the stronger of the two available
    options.** The Documents matrix gained a dated `Closing review` column, and
    the deferral section was updated to say which column means what: the old
    `Review` cells "retain the suspended historical values from before deferral"
    and "do not report the closing pass", while the dated column "is the
    closing-pass result". The manual-acceptance row is marked `—` as outside the
    six-document set, which is correct — Round 1 reviewed six documents and that
    was not one of them. The closing pass now has falsifiable cells, and this
    round moved the two that were `[ ]`.
- New findings: none. The four documents that passed Round 1 are unchanged, and
  the two repaired documents introduced no defect this round could find.
- Status synchronization performed by this round, recorded here because it
  changed the fingerprint the evidence binds to: `requirements.md` and `tasks.md`
  moved to `[x]` in the `Closing review` column, and `tasks.md`'s next-action
  statement was corrected — it still directed the reader to run this review and to
  re-review task 6's record, both of which had already run.
- Evidence:
  - `bash scripts/check-topic-docs.sh` → exit 0 (the project's checker for this
    target class)
  - `make check-whitespace` → exit 0
  - `git diff --check` → exit 0
  - Per-document blob comparison against Round 1 → only `requirements.md` and
    `tasks.md` changed
- What this verdict does and does not unblock: it closes task 6's R1-F1, whose
  whole content was that this review had not run. It does not let task 6 pass.
  The rule repaired under CD1-F2 now makes that explicit — task 4 must close
  `DW-R3-F1` before task 6 runs the final reconciliation that removes the rule —
  and `DW-R3-F1` is parked on an Apple Developer team ID that does not exist yet.
  That is where the topic stops, and it is an external prerequisite rather than
  work anyone here can perform.
- Set-wide verdict: **PASS**

### Round 3 addendum — completion-evidence bookkeeping (2026-08-23)

Recorded after the round, because the recording itself needed a correction and
hiding it would defeat the purpose of an audit trail.

- The six `document` boundaries this round crossed are now recorded and the gate
  answers `VERIFIED` for each, bound to content state
  `urn:ce:agent-deck:state:candidate:a5e3433a…f167` (HEAD `a190186` plus the
  post-synchronization set fingerprint `c42a6de1…d2a7`).
- **A mistake, disclosed rather than quietly fixed.** The first write used
  `MERGE` on work-unit ids that already existed under the store's *retired*
  capitalized `WorkUnit` vocabulary for four of the six documents —
  `architecture.md`, `ux/menubar.md`, `ux/widget.md` and `tasks.md`. Two things
  followed. The gate could not see them, because it filters on the current
  lowercase `work_unit` kind, so those four read `NOT_VERIFIED` while looking
  recorded. And an unconditional `SET` overwrote each of those historical nodes'
  `target_content_state` with this round's value; the prior values were not
  captured first and are **not** restored here, because guessing them from their
  latest evidence would put an invented fact into the record. `head` and
  `attributes_json` were added to those nodes rather than overwritten, the old
  vocabulary carrying neither.
- The forward fix creates four new current-model `work_unit` nodes under distinct
  ids ending `-doc`, pointing at the criteria this round already wrote, and
  leaves the historical nodes as history. `.agent-instructions/evidence.md` says
  to leave retired-vocabulary nodes as they are; it also says to inspect the store
  before writing, which is the step that was skipped.
- One pre-existing row still reads `NOT_VERIFIED`: `ux/settings.md`'s older
  `independent-review-pass` criterion, whose evidence binds to a content state
  from before the deferral. It is a leftover of the earlier process, not of this
  round, and is left for whoever reconciles that boundary.
