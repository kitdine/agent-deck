---
status: active
created: 2026-09-03
---

# 验收：真实 Claude startup route 生效

## 目的

`fix / claude-startup-route` 修复了 Claude `source=startup` 在 transcript 尚未
创建时被入口校验丢弃的问题。它的代码与隔离回归已经通过，但当时没有一个修复版本
下新启动的真实交互式 Claude Code 会话，无法证明真实 Hook delivery 会把 route 与
observation 写进用户数据库。

本记录完成 `ad-verify-claude-startup-route-live` 的 task-owned 验收。它不重放 Hook，
不手工插入或回填 route，而是复核一个已经自然产生的真实会话。

本文件对 Lane A fix record 模板有一处**刻意偏离**：它没有「现象」「根因」与
「修复边界」三节，因为这里不修复新缺陷，只验收既有修复的真实运行结果。对应信息
改由「目的」「验收边界」与下面的实测证据承载；记录内仍保留明确 scope、可复现
命令、残余风险和独立 Review，避免把模板缺失误认为内容遗漏。

## 验收边界

**覆盖**：一个在 `v0.5.0-rc.5` 安装后启动的真实交互式 Claude Code session；
`SessionStart/startup` route 与 observation；该 session 的全部 usage invocation；
Go 生成的 desktop quality projection；二进制、Cask 安装与 transcript 时间线。

**不覆盖**：多次 startup 的统计稳定性，`resume`、`clear`、`compact`、`fork`，
claude-mem SDK observer session，以及无 route 会话能否从 transcript metadata 推导
可靠进程起点。最后一个问题只属于 `ad-bug-claude-no-route-quality` 的独立调查；
本验收不改变它的 fail-closed 边界。

## 候选身份

- Session：`c3e1097b-f39f-4118-ad69-e084e06c2304`。
- Hook 注册：Claude `SessionStart` 执行 `agentdeck usage hook event claude`。
- PATH：`/usr/local/bin/agentdeck` 指向
  `/Applications/AgentDeck.app/Contents/Helpers/agentdeck`。
- 二进制：`v0.5.0-rc.5`，commit
  `dd09acc96ad12b94e67a32bea30ba2007cfdd769`，release build；bundle version
  `0.5.0-rc.5` / build `14`。
- Caskroom `0.5.0-rc.5` 创建于 2026-09-02 10:34:06 PDT，app 于 10:34:07
  完成修改。startup route 写于同日 20:26:37 PDT，晚约 9 小时 52 分，故该 session
  明确运行在 rc.5 安装之后。
- 当前仓库基线为 HEAD `3b861e0416223e39cf1db26249a35a3bb2d51979`、tree
  `7242dfb040599d5ca452123769f3f1a3298155f1`；它标识本记录的内容基线，不冒充所
  验收的已安装二进制身份。

## 真实 startup delivery

用户数据库中只有一条 Claude `SessionStart/startup` route，且与同 session、同
时间的 observation 一一对应：

| Field | Observed value |
| --- | --- |
| route id | `180` |
| observation id | `9` |
| observed_at | `2026-09-03T03:26:37.375364Z` |
| provider / multiplier / via_wrapper | `official` / `1` / `0` |
| hook_event / source | `SessionStart` / `startup` |
| route_effect | `advance` |
| delivery_id | `e54df75fe3f3efd0055b4853e829ee5b` |

同 session 的唯一 usage source basename 是
`c3e1097b-f39f-4118-ad69-e084e06c2304.jsonl`。APFS birth time 为
2026-09-02 20:26:51 PDT，比 route 晚 14 秒；首个 usage event 在 20:27:03 PDT，
又晚约 12 秒。真实 Hook 因而确实走过“startup 到达时 transcript 尚不存在”的原
缺陷路径，而不是只证明一个已经存在 transcript 的普通写入。

## Session attribution

本节的绝对数绑定到实现验收快照：`agentdeck session show` 的 `generated_at` 为
`2026-09-03T07:26:15.654459Z`，当时该会话仍在增长。以下 events 与 cost 是该时刻
的可复算测量，不是会话终值；Review 于 07:42Z 复算到 221 invocations，正是这条
时间边界需要显式记录的原因。

只读 SQL 复核到该 session 有 220 个 events、一个 source，时间范围
`2026-09-03T03:27:03.941Z..2026-09-03T07:20:10.52Z`；220/220 都不早于 route。

随后用安装的 rc.5 执行：

```bash
agentdeck session show c3e1097b-f39f-4118-ad69-e084e06c2304 \
  --client claude --tokens --all --format json
```

过滤掉 documents 与 source path 后，结果为 220/220 invocations、无 invocation
warning、无 summary warning、无 unpriced component；`provider_cost` 与
`known_provider_cost` 均为 `37.197367000`。安装 commit 的
`internal/usage/session_usage.go` 明确给每个非 `exact` invocation 追加
`<quality> attribution` warning，因此空 warning 集证明全部 220 个 event 的读时
quality 为 `exact`。route 表本身的 `quality=estimated` 是被观察边界的落盘字段，
不是这些 event 经 `effective_route` 解析后的最终质量。

## Desktop projection

`agentdeck desktop snapshot --format json` 在
`2026-09-03T07:27:57.576658Z` 生成的 Claude quality projection 为：

| Period | Determinable | Inferred | Unattributed |
| --- | --- | --- | --- |
| today | 42 events / $10.276684000 / 100.00% | 0 | 0 |
| 7d | 220 events / $37.197367000 / 6.32% | 2,702 events / $551.256801500 / 93.68% | 0 |
| 30d | 4,736 events / $1,133.229951250 / 40.44% | 9,524 events / $1,669.033452400 / 59.56% | 0 |

7d 的 determinable bucket 与本 session 在 event count、token count
`55,869,166` 和 provider cost 三项完全相同；`today` 的 42 events 是本地午夜后
的子集。desktop DTO 是聚合而非 session DTO，这组三项相等是当前快照把该 session
投影进 determinable bucket 的可核对关系，不声称它提供逐 session 字段。

## 复现

以下检查均只读；输出只保留任务所需字段，不读取或记录 transcript 内容：

```bash
agentdeck version
plutil -extract CFBundleShortVersionString raw -o - \
  /Applications/AgentDeck.app/Contents/Info.plist
plutil -extract CFBundleVersion raw -o - \
  /Applications/AgentDeck.app/Contents/Info.plist
sqlite3 -readonly ~/.agentdeck/agentdeck.sqlite3 \
  '.schema usage_session_routes' '.schema usage_session_observations'
sqlite3 -readonly -json ~/.agentdeck/agentdeck.sqlite3 '<session-bound route,
  observation, session, and event-count SELECT>'
agentdeck session show c3e1097b-f39f-4118-ad69-e084e06c2304 \
  --client claude --tokens --all --format json | jq '<usage-only projection>'
agentdeck desktop snapshot --format json | jq '<claude quality tiers only>'
```

## 结果与残余风险

Task acceptance 对这个明确 session 已满足：真实 rc.5 startup delivery 同时写出
route 与 observation；该时间戳验收快照中的全部 events 在 usage session 读路径中
为 exact，并在 desktop 7d determinable bucket 中以相同 events、tokens 与 cost
出现。没有 route 或 observation 被手工写入、回填或改写。

样本量仍是 rc.5 后真实交互 startup 的 1/1。本记录因此证明修复在一个真实 startup
路径上生效，不把它外推成多次启动的统计稳定性，也不覆盖其他 SessionStart source。
是否扩大验证是后续独立判断，不是本 Task acceptance 的隐藏前提。

## Completion evidence

`fix:claude-startup-route-live` WorkUnit 与六条 required criteria 在验收前建立。最终
CEv1 evidence 只在本记录与 `docs/status.md` 同步后的内容状态上追加；门禁结果由
本阶段 handoff 报告，后续独立 Review 仍单独拥有 verdict。

## Review — Round 1 — 2026-09-03

- Reviewed state: HEAD `3b861e0416223e39cf1db26249a35a3bb2d51979`；
  `docs/fixes/claude-startup-route-live.md` blob
  `5c76d9a586aae6847990808e355d8cac806a5726`，`docs/status.md` blob
  `1f6b2150170e9e742d65d8f9ee4d68a63a05568d`；scoped fingerprint
  `347d935a33a4a5cdc3e01850aad5930afbf2b9a8144a4b592d22deb91e39e461`，与
  `fix:claude-startup-route-live` 现有 evidence 绑定的内容状态一致。
- Reviewer: claude-code。**独立性是部分的**：本记录由 codex 撰写，我未参与它的
  起草；但被验收的会话正是本 reviewer 所在的会话，且我此前向该任务提交过
  route 与会话拆分的观察评论。因此我对记录是冷的，对被观察对象不是。
- Method: 不采信记录中的任何数字，逐条重跑只读命令与 SQL 复算；另核对记录用作
  推论前提的实现代码，以及 `docs/status.md` 的对应改动与 CEv1 现状。
- Scope: 本记录、`docs/status.md` 的该段、`fix:claude-startup-route-live` 的 CEv1
  对象。产品代码、测试与已安装二进制只读。
- Findings:
  - [P2] `L1-F1` 记录与状态投影把一个**仍在增长的会话**的瞬时计数写成了定值，
    且未标注测量时刻。指向本次改动之内。记录「Session attribution」节写
    「220 个 events」「`provider_cost` 与 `known_provider_cost` 均为
    `37.197367000`」，`docs/status.md` 写 `all 220 stored invocations read
    determinable`；本轮复算为 **221 invocations / `37.447301500`**，而
    `desktop snapshot` 的 `today` 档也已从记录的 `100.00% / 0 inferred` 变为
    43 事件 determinable 加 1 事件 inferred——后者来自另一个会话，与本验收对象
    无关。差异全部来自会话仍在运行，不是记录测错。问题在呈现：同一份记录的
    「Desktop projection」节标注了 `generated_at`，而「Session attribution」节
    没有，于是复核者拿到不同数字时无从判断是记录过时还是记录有误。
    `docs/status.md` 受影响更重，因为它是长期文档，`220` 会一直错下去，而去掉
    数字的表述（「该会话的全部 stored invocations 读作 determinable」）永远为真。
  - [nit] `L1-F2` 记录未声明它对 Lane A fix record 模板的刻意偏离。指向本次改动
    之内。它没有「现象/根因/修复边界」三节，因为这是验收而非修复——这与同目录
    的 `staple-offline-first-launch.md` 情形相同，而那份记录在开头显式声明了偏离
    及理由。此处缺这一句，后来者无法区分刻意与疏漏。
  - 以下断言逐条独立复算，**全部成立**：
    - route 与 observation：库中只有一条 Claude `SessionStart/startup` route，
      route id `180` / observation id `9` / `2026-09-03T03:26:37.375364Z` /
      `official` / `1` / `via_wrapper=0` / `route_effect=advance` /
      delivery `e54df75fe3f3efd0055b4853e829ee5b`，与记录逐字段一致。
    - 原缺陷路径确实被走过：transcript APFS birth 为 2026-09-02 20:26:51 PDT，
      比 route 的 20:26:37 PDT **晚 14 秒**，首个 usage event 再晚约 12 秒。
      这正是「startup 到达时 transcript 尚不存在」，不是普通写入。
    - 候选身份：`agentdeck version` 报 `v0.5.0-rc.5` / `dd09acc`，
      `/usr/local/bin/agentdeck` 符号链接指向 App bundle 内 helper，bundle
      `0.5.0-rc.5` / build `14`，Caskroom `0.5.0-rc.5` 创建于 2026-09-02
      10:34:06 PDT，早于 route 约 9 小时 52 分。
    - 推论前提属实：`internal/usage/session_usage.go:118-119` 对每个
      `quality != "exact"` 的 invocation 追加 `<quality> attribution` warning，
      因此空 warning 集确实蕴含全部 invocation 为 `exact`。且
      `git diff dd09acc..HEAD -- internal/ cmd/` 为空，工作区代码即已安装二进制
      的代码，该推论对本次观察有效。
    - 会话读路径：221 invocations，带 warning 的 invocation 为 0，summary
      warning 为空，`unpriced_components` 为空；会话 221/221 个 event 均不早于
      route。
    - desktop 投影关系：7d determinable bucket 为 221 events /
      56,314,950 tokens / `$37.447301500`，与该会话的 events、token 合计与
      provider cost **三项完全相同**，记录声称的可核对关系成立。
  - 边界声明恰当：记录明确不覆盖多次 startup 的统计稳定性、`resume` / `clear` /
    `compact` / `fork`、SDK observer 会话，以及无 route 会话的进程起点推导，并把
    最后一项划归 `ad-bug-claude-no-route-quality`，未越界。
  - CEv1 现状恰当：六条 evidence 的 `assessor_kind` 为
    `implementation_verification`，disposition 写明 `independent review pending`，
    没有冒充独立评审的 verdict。
- Evidence:
  - `sqlite3 -readonly` 查 `usage_session_routes` / `usage_session_observations`
    /`usage_events`（按 session 限定）。
  - `stat -f '%SB'` 取 transcript birth time；`agentdeck version`、
    `plutil -extract` 取二进制与 bundle 身份；`stat` 取 Caskroom 创建时刻。
  - `agentdeck session show … --tokens --all --format json`，统计 invocation 数、
    带 warning 的 invocation 数、summary warning 与 unpriced。
  - `agentdeck desktop snapshot --format json`，取 claude scope 的三档 quality。
  - `rg -n 'attribution' internal/usage/session_usage.go`；
    `git diff --stat dd09acc..HEAD -- internal/ cmd/`。
- Completion gate: `NOT_VERIFIED` —— 六条 required criteria 现有 evidence 均为
  `implementation_verification`，绑定内容状态
  `347d935a…`；`L1-F1` 使 `acceptance-record-complete` 在该状态上不成立。修复后
  由复评在新的内容状态上记录 review 级 evidence 并重新查询。
- Verdict: REOPEN —— `L1-F1`、`L1-F2` 均指向本次改动之内。**验收的实质结论成立
  且已被独立复算证实**：真实 rc.5 startup delivery 同时写出 route 与
  observation，走的正是原缺陷时序，该会话全部 event 读作 `exact`，并按三项相等
  投影进 desktop determinable bucket。被打回的只是数字的呈现方式与一句模板偏离
  声明。

### 下一步指令

修复：fix / claude-startup-route-live / L1-F1 L1-F2

## Repair — Round 1 — 2026-09-03

- `L1-F1` closed：`Session attribution` 现把 220 events 与 `$37.197367000` 明确
  绑定到 `generated_at=2026-09-03T07:26:15.654459Z`，并写明 session 当时仍在
  增长；Review 复算到 221 的结果作为为什么必须标时的旁证保留。「结果与残余风险」
  改成该时间戳快照中的全部 events，不再把瞬时总数写成长期结论。
- `docs/status.md` 同步删除 `220`，只陈述该时间戳验收快照中的全部 stored
  invocations 均读作 determinable，且 desktop bucket 在同一快照上与 session 的
  events、tokens、cost 相等。它不再随活动 session 增长而立即失真。
- `L1-F2` closed：文件开头已明确声明对 Lane A fix record 模板的刻意偏离，并解释
  验收任务为何用「目的」「验收边界」与实测证据替代「现象」「根因」「修复边界」；
  scope、复现、残余风险与独立 Review 仍完整保留。
- 不涉及：route/observation 事实、原缺陷时序、binary/Cask 身份、desktop 投影、
  产品代码、测试、配置、真实运行数据与 `schema-version-signal` 均未改动。
- Verification：`make check-whitespace` 与 `git diff --check` 通过；精确检索确认
  `docs/status.md` 不再包含活动 session 的 `all 220` 长期断言。
- Completion gate：旧内容状态 `347d935a...` 的实现证据保持不可变；本轮 Repair
  改变候选内容且不自签 review evidence，由独立 Re-review 绑定新状态并重新查询。
- Verdict: REOPEN —— `L1-F1`、`L1-F2` 已关闭，Repair 完成，等待独立
  Re-review。

## Re-review — Round 2 — 2026-09-03

- Reviewed state: HEAD `3b861e0416223e39cf1db26249a35a3bb2d51979`；复评前
  `docs/fixes/claude-startup-route-live.md` blob
  `533092cbec05ab4681570294caf8b5bb9209fa78`，`docs/status.md` blob
  `99f1b5df25773f6056780d2d1f824c2c098ea7c6`；scoped fingerprint
  `6b23ac3be6e96730d64fae738d20d26bbe5b9978a8a45a18aba7893dd7493d41`。
- Reviewer: claude-code，独立性与 Round 1 相同——记录与本轮 Repair 均由 codex
  撰写，我未参与；但被验收的会话正是本 reviewer 所在的会话。对文档是冷的，对被
  观察对象不是。
- Method: 逐条核对 Repair Round 1 声明的四个目标的**实际内容**，而非声明本身；
  另重跑结论支柱的只读检查，确认修复未动事实。
- Findings:
  - `L1-F1` **closed**。四个目标逐条落实：
    - `Session attribution` 节现以「本节的绝对数绑定到实现验收快照」开头，写明
      `generated_at=2026-09-03T07:26:15.654459Z` 与「当时该会话仍在增长」，并把
      Review 复算到 221 作为「为什么必须标时」的旁证保留。
    - 「结果与残余风险」改为「该时间戳验收快照中的全部 events」，不再把瞬时总数
      写成长期结论。
    - `docs/status.md` 删掉了 `220`，改为「every invocation present in its
      timestamped acceptance snapshot reads determinable」，长期文档不再随活动
      会话失真。
    - `Desktop projection` 节本就标注了 `generated_at`，Repair 未改动它，与
      `L1-F1` 的诉求一致。
    - 顺带核实记录引用的时间戳字段真实存在：`agentdeck session show --format
      json` 的顶层确有 `generated_at`。
  - `L1-F2` **closed**：文件「目的」节末尾已声明对 Lane A fix record 模板的刻意
    偏离，说明为何用「目的」「验收边界」与实测证据替代「现象」「根因」「修复
    边界」，并指出 scope、复现、残余风险与独立 Review 仍完整保留。
  - Repair 声明与实际内容**一致**，未出现声明漂移。
  - 结论支柱重跑后全部成立，且不受会话增长影响：route id `180` 与 observation
    id `9` 字段不变；transcript birth 20:26:51 PDT 仍比 route 的 20:26:37 晚
    14 秒；读路径本轮复算为 242 invocations，带 warning 者 0、summary warning
    空、`unpriced_components` 空——增长使绝对数再次变化，而「全部 invocation 为
    `exact`」这一性质不变，正是修复后表述所主张的形式。
- Evidence: `sed -n` 读取四个目标的当前内容；`sqlite3 -readonly` 复查 route 与
  observation；`stat -f '%SB'` 复查 transcript birth；
  `agentdeck session show … --tokens --all --format json` 统计 invocation 与
  warning；`agentdeck session show … --format json | jq keys` 确认
  `generated_at` 字段存在。
- Completion gate: `VERIFIED` —— 六条 required criteria 在本轮复评后的内容状态上
  记录 review 级 evidence 后全部 `pass`；实现阶段绑定 `347d935a…` 的
  `implementation_verification` 证据保持不可变，新证据以 `supersedes` 指向它。
- Verdict: PASS —— `L1-F1`、`L1-F2` 均已闭合，验收的实质结论在两轮独立复算中
  保持成立。
