---
status: active
created: 2026-09-01
updated: 2026-09-02
---

# 缺陷：可确定的归因被判为推断

本记录覆盖 Codex 与 Claude 两侧的读时归因判定。写入侧的
`SessionStart` 准入缺陷由 [`claude-startup-route.md`](claude-startup-route.md)
单独承载；两者互补：那一份让新会话重新写得出 route，本记录修正读取侧对
既有数据的判定。

## 现象

菜单栏「归因」面板在「今天」这一档显示可确定 0%、推断 100%。切到更长周期
可以看出这不是功能失效，而是新数据停止产生可确定归因：

| 周期 | 可确定 | 推断 |
| --- | --- | --- |
| 今天 | 0 事件 / $0 / 0% | 1,365 事件 / $286.65 / 100% |
| 近 7 天 | 2,475 事件 / $432.26 / 39.7% | 3,815 事件 / $655.75 / 60.3% |
| 近 30 天 | 25,208 事件 / $3,425.91 / 62.5% | 14,058 事件 / $2,054.44 / 37.5% |

数据取自 `agentdeck desktop snapshot --format json` 的
`presentation.scopes[0].quality`，30 天的 25,208 与 `agentdeck usage summary`
的 `EXACT ATTRIBUTION` 一致。

全库口径：事件 95,534，其中可确定 25,208（26%）、推断 50,133（52%）、
未归因 20,193（21%）。推断的主体是没有 hook route 的事件——Codex 38,377、
Claude 5,793——而按两个客户端各自的配置语义，其中绝大多数是可确定的。

金额当前不受影响：`quality` 只是随 price 与 multiplier 一并返回的标签，
`priceForEvent` 不读它。下述根因二会在事件落入遗留 route 的有效区间时返回错误
provider 与 multiplier，但本机四条遗留 route 的这些区间当前都没有事件。

## 根因

### 一、`timeline_snapshot` 分支硬编码 `estimated`，不分客户端

修复前的 `internal/usage/usage.go` `resolveSessionAttribution`（现位于 `:2933`）
在没有定位到
route 时回退到会话起点的 provider 时间线快照，并把 quality 写死：

```go
result.quality = "estimated"
result.reason = attributionReasonTimelineSnapshot
```

`docs/archive/topics/usage-attribution-precision/architecture.md` 的目标解析
顺序第 3 步同样写作 `-> estimated / timeline_snapshot`，实现与该文档一致。

但这与该 topic 自己的 Confirmed Decision 冲突。`requirements.md` 第一条是：

> **`exact` means determinable, not process-bound.** The quality dimension
> describes whether the provider and multiplier for an event are determinable,
> not which code path recorded them.

而 architecture 对 Codex 的论证恰恰支持可确定：

> **Codex** activates configuration only on restart. … The resulting effective
> route is the exact configuration the process loaded and will keep for its
> lifetime.

该论证用于第 2 步（有 route）。第 3 步取的是同一条 provider 时间线的同一个
值——写 route 时 hook 读的是 `CurrentProviderSnapshot`，事后回退读的是
`SnapshotAt(session start)`，数据源相同。architecture 从未论证为什么同一个
事实经由 route 是 `exact`、经由时间线就只能是 `estimated`。

测试把这一不一致固化成并排的两条断言
（`internal/usage/usage_test.go` `TestReadPriceResolverUsesClientTimeSemantics`）：

```go
{"codex spans global switch",  "codex", "codex-session",    …, "codexA", "2", "exact"},
{"timeline fallback stays at session start", "codex", "fallback-session", …, "codexA", "2", "estimated"},
```

同客户端、同 provider、同 multiplier、同样跨越全局切换，唯一差别是有没有一行
route 记录。

### 二、未采纳的 Claude `ConfigChange` route 会返回错误的 provider 与 multiplier

Claude 只会热生效一种转换：无 key → 首次加 key。换 key、删 key 都不被运行中的
会话采纳，要重启才生效。当前写入侧对此处理正确——`classifyConfigChange` 对这些
情况返回 `routeEffectRetain` 且不写 route，所以**新数据**沿用前一条 route，判定
为 `exact`。

问题出在 `switch-effectiveness-boundary` task 3 之前写入的历史 route：它们把
未被采纳的变更也写进了 `usage_session_routes`。读取侧
`resolveSessionAttribution` 命中这类 route 时，返回的是 `route.provider`——
**那个未生效的新 provider**——只是把 quality 标成 `estimated`。

「没有被采纳」是一个确定的结论，它恰恰意味着会话仍在用 `prior` 那个 provider。
所以此处既不该是推断，返回的 provider 也不该是 route 记录的那个。

本机存在 4 条这样的 route：

| id | route provider | 倍率 | 前序 provider | 前序倍率 |
| --- | --- | --- | --- | --- |
| 37 | official | **1** | **cubence** | **1.2** |
| 89 | official | 1 | sssaicode | 1.0 |
| 98 | official | 1 | sssaicode | 1.0 |
| 99 | official | 1 | sssaicode | 1.0 |

第 37 条 removal 位于 `2026-08-10T11:53:36`，下一条 `SessionStart official`
已在 `11:55:52` 重新定段；两者之间有 **0 个事件**。此前归给 removal 的 169 个
事件实际都发生在新 `SessionStart` 之后，因此不会读到 route 37。Round 2 曾发现
第一次 Repair 向任意早期 selection 回溯，使另外 4 个会话错误继承 1.2 倍率；
R2-F1 已删除这条回溯。首条 ConfigChange 现在只接受会话 `first_at` snapshot 的
同值佐证，或在 route 晚于 `first_at` 时把它作为 prior；其余情况 fail closed。

修复仍保留 provider/multiplier 回归：如果其他数据库确有事件落入这种遗留区间，
读取侧必须返回前序有效 route，不能把未采纳的新 selection 当成实际计费路线。

### 三、验收未覆盖无 route 这一档

`tasks.md` 的 Development Task 2 记录写作「reproduces 162 exact versus 4
estimated routes and 12,395 exact versus 534 estimated Claude events」。那是
Claude ConfigChange-positioned 事件的分类，读起来像全局结论。

验收当日（2026-08-26）的真实构成：

| client | 事件总数 | 有 route | 无 route |
| --- | --- | --- | --- |
| claude | 9,944 | 6,739 | 3,205 |
| codex | 78,619 | 21,240 | **57,379** |

占当时全部事件 65% 的 Codex 无 route 事件从未进入验收视野。而
`requirements.md` 明确要求过验收必须 names its denominator：

> Implementation acceptance must separately report event count, run count,
> provider-cost amount, and provider-cost share for each quality bucket so a
> claim such as `exact cost = 0 / 0%` names its denominator and can be reproduced.

### 四、`before_adoption` 实际上永不出现

`resolveSessionAttribution` 用 `timelineExists(client)`（即 `HasClient`）在两个
未归因原因之间选择：

```go
exists, _ := timelineExists(event.Client)
result.reason = attributionReasonBeforeAdoption
if exists {
    result.reason = attributionReasonCoverageGap
}
```

它问的是「该客户端有没有任何一条 selection」，不是「事件那一刻之前有没有」。
只要用过一次 AgentDeck 切换，全部历史数据就从「采纳前」变成「覆盖缺口」。
`requirements.md` 要求 `usage summary` 区分这两者，而现在只有一个能被触发。

### 五、写入侧与读取侧使用了两套 no-key 判据

同一个「无 key」概念有两个实现：

- 写入侧 `classifyPriorAuthentication`（`internal/usage/routes.go:217`）：
  `snapshot.Official && snapshot.Credential == ""` —— 真的检查凭据
- 读取侧 `routeQuality`：`priorProvider == "official"` —— 用 provider 名近似

`official` 恰好等价于无 key 是当前数据的巧合，不是契约。任何一个没有配置凭据的
自定义 provider，或给 official 配上凭据，都会让两侧分叉。

## 查证记录

以下事实由本机数据与 `openai/codex` 源码交叉确认，是判定规则的依据。

### Codex：session_id 编码了进程启动时刻

Codex 的 `session_id` 是 UUIDv7，前 48 位为毫秒时间戳，与 rollout 文件名逐秒
吻合（本机 UTC 与本地相差 7 小时）：

| session_id | 版本 | 解出时间 | 文件名 |
| --- | --- | --- | --- |
| `01a05e39-77c4-…` | v7 | 2026-09-01T18:26:59.652Z | `11-26-59` |
| `01a05e44-15d0-…` | v7 | 2026-09-01T18:38:35.472Z | `11-38-35` |
| `01a05c13-983e-…` | v7 | 2026-09-01T08:26:23.166Z | `01-26-23` |

全库 288 个 Codex 会话的 `session_id` 全部是 v7。Claude 侧 115 个会话全部是
v4，不含时间信息。

### Codex：一个 session_id 下的多个 rollout 文件不是 resume

`session_meta` 的字段组合区分四种来源：

| `session_id` | `thread_source` | `source` | 文件数 | 含义 |
| --- | --- | --- | --- | --- |
| 指向他人 | `subagent` | subagent 对象 | 552 | 子代理派生 |
| 指向他人 | `user` | cli/vscode/exec | 255 | fork |
| 指向他人 | `guardian_review` | subagent 对象 | 16 | guardian 审查 |
| 自身或 null | `user` | cli/vscode/exec | 22 | 新建会话 |

DeepWiki 对 codex 源码的分析确认 resume 打开并追加已有 rollout 文件
（"opens an existing rollout"，带 tail 修复逻辑），fork 才新建文件
（"the child thread gets its own rollout file with inherited metadata"）。

### Codex：rollout 文件不含可靠的 resume 标记

`codex-rs/history/src/lib.rs` 的 `RolloutItem` 枚举共 11 个变体
（`SessionMeta` `ResponseItem` `InterAgentCommunication`
`InterAgentCommunicationMetadata` `Compacted` `TurnContext` `TokenUsageRecord`
`WorldState` `SecurityRiskScore` `EventMsg` `RealtimeItem`），**没有任何
resume 或 session-configured 变体**。本机采样到的 `event_msg` 类型全集
（`agent_message` `context_compacted` `item_completed` `task_complete`
`task_started` `thread_settings_applied` `token_count` `turn_aborted`
`user_message`）同样没有。

`turn_context` 与 `thread_settings_applied` 都不是进程启动标记：

- `codex-rs/core/src/session/turn_input.rs`：
  `let emit_thread_settings_applied = self.thread_settings_update.is_some();`
  —— 它是「本 turn 携带设置更新」的标记
- 实测：一个文件的 4 条 `turn_context` 设置字段逐字相同（model / effort /
  approval_policy / personality / collaboration_mode / comp_hash），不是设置
  变更触发
- 实测：切模型（`gpt-5.6-terra` → `gpt-5.6-sol`）时写了 2 条相隔 6 毫秒的
  `thread_settings_applied`，所以它也会被切模型触发
- 实测：存在只有 `turn_context` 而无 `thread_settings_applied` 的时刻，两者
  配对不严格
- 采样 236 个多 `turn_context` 文件，其中 16 个的 model 变化过

结论：**首次启动时刻可靠**（`session_meta` / 文件名 / UUIDv7），**resume 时刻
只有 hook 能给**。

hook 本身不完整：一个会话在 2026-08-29 有 3 次 resume（08:21 首启，14:11、
14:37、14:53 三次恢复，各自带 `thread_settings_applied` + `turn_context`），
而 `usage_session_routes` 只记下了 14:11 那一次——该表最后一条正是
`2026-08-29T14:11:46`，hook 从那一刻起就失效了。

### Codex：subagent 与父同进程，继承父的运行时配置

`codex-rs/core/src/agent/control/spawn.rs`：

```rust
struct SpawnAgentThreadInheritance { … }
// Child threads inherit model context, not the parent's cumulative usage state.
config = build_agent_resume_config(&turn)
    .map_err(|_| "cannot resume multi-agent v2 child … with the current parent settings")
Arc::ptr_eq(&self.state, &thread.session.services.agent_control.state)
```

`Arc::ptr_eq` 只在同一进程内有意义；配置由 `build_agent_resume_config(&turn)`
从父的 turn 构造而非重读 `config.toml`。父 resume 后重新加载子线程时取的是父
当前的 turn 设置，因此子始终跟随父的当前段。

### Codex：rollout 不记录具体 provider

AgentDeck 写入 `~/.codex/config.toml` 的形状是固定表名加内层 `name`：

```toml
model_provider = 'custom'
[model_providers.custom]
name = "official"
```

因此 `session_meta.model_provider` 恒为 `custom`，不含具体 provider 名。全库
856 个 `custom`、3 个 `ccswitch`、2 个无字段。那 3 个 `ccswitch` 文件在
2026-06-30，早于时间线起点，属于 `before_adoption`，不影响当前归因。该字段
唯一的用途是检测「配置被第三方工具接管」，作为 Backlog 候选另行处理。

### Claude：transcript 不含任何 provider 线索

Claude transcript 记录的字段为 `apiBlockIndex, cwd, effort, entrypoint,
gitBranch, isSidechain, message, parentUuid, requestId, sessionId, timestamp,
type, userType, uuid, version`；`message.model` 只有模型名（如
`claude-opus-5`），`message.usage` 含 `service_tier`、`inference_geo` 等。
**没有 provider、base_url 或任何认证信息。**

因此 Claude 侧不存在独立于 AgentDeck 的 provider 旁证，唯一来源是自身的 hook
与 provider 时间线。这是与 Codex 的本质差异：Codex 至少有 UUIDv7 给出精确的
启动时刻。

### Claude：key 状态的判据是充分的

`provider_selections.credential_name_snapshot` 为空即无 key（订阅模式），
非空即有 key。本机 Claude 的 25 条选择中 official（无凭据）出现 11 次，与
keyed 频繁交替。判断「无 key → 首次加 key」所需的全部信息都在该字段里。

### Claude：route 构成

| hook_event | source | provider | 条数 |
| --- | --- | --- | --- |
| SessionStart | resume | official / sssaicode / cubence / akile | 33 |
| ConfigChange | user_settings | official / sssaicode / cubence | 31 |

`SessionStart` **全部是 `resume`，零 `startup`**——即
[`claude-startup-route.md`](claude-startup-route.md) 所修的缺陷。
`provider='unknown'` 的 route（`ConfigMatched=false` 分支）在本机一条都没有，
该路径未被真实数据验证过。

## 修复边界

### Codex 侧判定规则

| 场景 | 结果 |
| --- | --- |
| `agentdeck run` 启动（`usage_runs.exact = 1`） | 可确定 |
| 有 hook route，按事件时刻定位到段 | 可确定 |
| 无 route，段起点在时间线覆盖期内，**会话跨度内时间线无 provider 切换** | **可确定** |
| 无 route，覆盖期内，跨度内有切换且无法定出段边界 | 推断 |
| 段起点早于该客户端第一条 selection | 未归因 `before_adoption` |
| subagent 事件 | 跟随父在该时刻所处的段，不独立判定 |

第三行是修复主体：Codex 只在进程启动时加载配置且终生不变，会话跨度内时间线
没有任何切换时，无论中途是否 resume 过，加载到的都是同一个 provider，因此
可确定，不需要知道 resume 边界。

### Claude 侧判定规则

Claude 的「可采纳转换」有且只有一种：无 key → 首次加 key。除此之外的任何
时间线变动（换 key、删 key、keyed→keyed）都不热生效，运行中的会话仍在使用
启动时那个 provider——这是确定的，不是不确定的。

| 场景 | 结果 |
| --- | --- |
| `agentdeck run` 启动 | 可确定 |
| 有 `SessionStart` route | 可确定 |
| 有 `ConfigChange` route，且为 无key→有key | 可确定，provider 取 route |
| 有 `ConfigChange` route，但为换 key / 删 key（历史遗留） | **可确定，provider 取 `prior` 而非 `route`** |
| 无 route，只能取得 `first_at` 处的 timeline snapshot | **推断** |
| 段起点早于 claude 第一条 selection | 未归因 `before_adoption` |

第四行同时修正 quality 与 provider；它是唯一具备金额影响路径的一条，但本机
当前没有事件落入四条遗留 route 的错误区间。

### 共用口径

- `before_adoption`：事件时刻早于该客户端第一条 provider selection。这是采纳
  AgentDeck 之前的历史，永久未归因，属于数据边界而非缺陷，不得被「补」成
  可确定。
- `coverage_gap`：事件时刻落在覆盖期内却查不到条目。按此口径应近乎不出现，
  一旦出现即表示时间线有洞，是可诊断的数据问题。

两者仍同属「未归因」一档。**不新增任何 quality 档位，不新增 reason。**

「无 key」的判据统一为 `credential_name_snapshot` 是否为空，读取侧不再以
`provider == "official"` 近似。

### Claude 无 route 的证据上限

Claude 的 `usage_sessions.first_at` 是首个可计量事件，不是进程启动时刻；两者差距
也没有 1–2 分钟上限。实测 route 可比 `first_at` 早 8 分 43 秒，且无 route 会话
`41b64b36` 的 `first_at` 只比一次 no-key -> first-key selection 晚 63.3 秒，说明
旧候选接受的风险已经在本机命中。

Claude session ID 是 UUIDv4，transcript 又不记录 provider，故无 route 时不存在
可证明真实启动边界的来源。修复后的上限是：仍用 `first_at` snapshot 给出最可能的
provider 与 multiplier，但 quality 保持 `estimated`；只有 `SessionStart`、已证明
的 `ConfigChange` route 或 exact run 才能升为 `exact`。Codex 的无 route 判定仍按
完整 `first_at..last_at` 会话跨度检查 timeline，因为 R1-F7 的缺口只发生在 Claude
无可验证启动边界的路径。

### 不做的事

- 不回填 `usage_session_routes`。route 是观测记录，按时间线反推合成写入等于
  制造「当时观测到了」的证据，并且会冲掉 `usage-attribution-precision` 评审
  记录中那份 166 行 route / 4 行必须留 `estimated` / 534 事件 4.13% 的实测基线。
- 不做 schema 迁移、不改写入侧、不做数据回填。quality 是读时派生的，判定规则
  修正后全部历史事件在下次读取时自动重算。
- 本机当前金额不变；仅保留对其他数据库中可能落入遗留 route 区间的正确读取规则。

## 预期结果

以下结果绑定 Round 2 Repair 的只读 SQLite `VACUUM INTO` 快照
（SHA-256 `f7557532559d3d99234ba95f6ee05692ed6831c59976f2c8b9c415bebac03048`）
与本轮修复候选，不外推到会继续增长的真实数据库：

| 全历史 quality | 事件数 |
| --- | ---: |
| `exact` | 58,416 |
| `estimated` | 17,477 |
| `unattributed` | 20,193 |
| 合计 | 96,086 |

reason 分母同为 96,086：`effective_route=25,755`、
`timeline_snapshot=50,138`、`before_adoption=20,193`，其余三项为 0。
全历史 `known_provider_cost=8988.163881770`；R2-F1 的长距离回溯已不再把 457 个
事件错误归给 1.2 倍率 provider。

`desktop snapshot` 的 30 天 all-client quality 记录给出每档金额与占比：

| quality | 事件数 | known provider cost | share |
| --- | ---: | ---: | ---: |
| determinable | 29,538 | 3931.288098260 | 71.28% |
| inferred | 9,282 | 1584.034476150 | 28.72% |
| unattributed | 0 | 0.000000000 | 0.00% |

determinable 与 inferred 两档均带 `cost_incomplete=true`，所以上表明确写的是
可累计的 known provider cost，不把缺价部分伪装成完整总额。

## 验证

实现仍限于读时路径：

- `storedEvent` 同时加载 `usage_sessions.first_at` 与 `last_at`。Codex 无 route
  quality 按完整会话跨度判定；缺失 `first_at` 时不再把 event time 当成可靠起点，
  而是保持 `estimated`。
- Claude 无 route 时只把 `first_at` snapshot 当 provider/multiplier fallback，
  quality 保持 `estimated`。有 route 时按时间顺序折叠整个 route 前缀；第一条
  `ConfigChange` 晚于 `first_at` 时才用会话 snapshot 作 prior；route 早于
  `first_at` 时只接受与 route provider/multiplier/wrapper 完全相同的会话 snapshot
  作佐证，不再向更早的全局 selection 回溯。连续未采纳变更沿用已经折叠出的有效
  prior。
- `Service.summarize` 一次加载 `readPriceResolver`，不再为每个事件重载完整
  provider timeline。1003 个事件的 Summary 查询预算被回归限制为至多 6 次。
- 已无生产调用的 `Service.eventAttributionForEvent`、`Service.priceForEvent` 及其
  专属数据库 route/price helpers 已删除；行为回归只验证生产 `readPriceResolver`
  或其公开 Summary/Presentation 消费路径。
- `ProviderTimeline` 保留定位覆盖与变化枚举；无生产调用的 `HasClient`、
  `ProviderTimelineExists` 和被 R2-F1 否定的任意历史回溯 helper 均已删除。

Repair RED 精确复现 R1-F3 至 R1-F7：整段跨切换会话的早期事件被误升 `exact`；
缺失起点被误升 `exact`；1003 事件 Summary 执行 5016 次 SQL；route 早于
`first_at` 时错误返回 `official/1`；连续未采纳 route 错误返回原始前序
`keyB/9`；Claude 无 route 反例被误升 `exact`。同一聚焦命令在修复后全部 GREEN。

其他回归确认：

- known `SessionStart` route 与 exact run 的优先级不变；credential 证据不足、
  同名 provider 但 multiplier 改变时继续 fail closed。
- `before_adoption` 使用定位时刻，`coverage_gap` 仍需已有覆盖却缺 snapshot，二者
  都不进入 provider spend。
- 历史 Claude keyed rotation/removal 的合成用例在生产 `readPriceResolver` 上返回
  折叠后的 prior provider 与 multiplier；本机真实四个遗留区间事件数为 0。
- `snapshot-complete.json` 通过 producer 重生成，`before_adoption` 从 0 更新为 2；
  无 update 环境变量的可复现性测试随后通过。
- R1-F9 两个不足凭据的用例改名为 `unresolved-current-selection` 与
  `unresolved-first-key-selection`，名称与 `estimated/ambiguous_route` 期望一致。

执行结果：

- 聚焦 Repair RED：FAIL，且仅为上述目标原因；对应 GREEN：PASS。
- `env GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh
  ./internal/usage`：PASS。
- `env GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh
  ./internal/store ./internal/desktop`：PASS。
- `env AGENTDECK_UPDATE_FIXTURES=1 GOCACHE=/private/tmp/agent-deck-go-build
  scripts/run-go-test.sh ./internal/desktop -run
  '^TestCanonicalFixturesAreReproducibleProducerOutput$'`：PASS；去掉 update 环境变量
  后再次 PASS。
- `env GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：
  PASS。
- 同一只读数据库快照上的候选 `usage summary --no-scan` 与
  `desktop snapshot` 产生 预期结果 一节的全历史及 30 天数据。
- `make check-whitespace`：PASS；`git diff --check`：PASS。

## 未决

- **`model_provider` 字段的利用**属于 Backlog，本次不使用。
- `turn_context` 是否还有本记录未观察到的触发原因，未穷尽验证。这不影响判定
  规则，因为规则不依赖它定段。
- **Claude 侧「无 key」判定的可靠性存疑，承载于 `ad-chore-a6f1-nokey-premise`。**
  2026-09-02 的全库 finding 审计发现 `A6-F1` 是 103 条中唯一没有闭环痕迹的一条，
  现由 `ad-chore-a6f1-nokey-premise` 承载；
  它主张：会话可先经 `env.ANTHROPIC_API_KEY` 或 `apiKeyHelper` 认证，随后该字段
  被移出受管文件而运行中的进程并未重新认证；此时对当前文件的冲突扫描看不到任何
  冲突，候选状态被提升为 `no-key`，于是后续的加 key 会被记成一条进程从未采纳的
  first-key route。本记录仅有 route 的 Claude 分支仍依赖「无 key → 首次加 key
  是唯一热生效转换」；无 route 分支已在 R1-F7 修复中固定为 `estimated`，不再
  依赖该前提。route 分支的这一保留由 `A6-F1` 继续承载，承载体同为
  `ad-chore-a6f1-nokey-premise`。
- Claude 的 `SessionStart` 接受 `fork` source，本机无数据，行为未验证。
- `ConfigMatched=false` 写入 `provider='unknown'` 的分支本机零数据，未被真实
  数据验证过。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `75767f11b5942789e5692f0b1cffc219980a02a7` 加未提交工作区，
  blob:
  `a62911078ed1ce11493cbe9e3b3bfcf476c8571e internal/usage/usage.go`、
  `569a4276e285bbc54cce62bf6a370016df2a2eac internal/usage/usage_test.go`、
  `a5d99a55aa79033a863dd1803c1bfd51ab44ff8b internal/usage/routes_test.go`、
  `19f570ea39cb375e75a70491f6d7faaa2af3b930 internal/usage/presentation_test.go`、
  `b8eb187d84652965b1845a711b701709efb32599 internal/store/providers.go`、
  `a8a86abd5b33b0f4d2ad9ca85a0910b0ee032717 internal/store/providers_test.go`、
  `5be6bec1f7a18e572a91f6e19a0d2368e7972a0f docs/fixes/attribution-determinability.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 仓库核对 + 实测。对着 `~/.agentdeck/agentdeck.sqlite3` 的只读副本
  （95,765 事件 / 37 selections / 37 operations / 177 routes）分别构建 HEAD 与候选
  二进制并对比 `usage summary --no-scan`、`usage sessions`、`desktop snapshot`；
  把三个行为级新测试逐字抽出、在 HEAD 实现上单独运行以验证 RED；全量
  `scripts/run-go-test.sh ./...`。本项目对 `docs/fixes/**` 未提供文档集校验器
  （`scripts/check-topic-docs.sh` 只读 `docs/topics/**`），因此该类证据缺席而非未运行。
- Scope: `internal/usage/usage.go`、`internal/store/providers.go` 及四个测试文件的
  读时归因判定；本记录的 现象 / 根因 / 修复边界 / 预期结果 / 验证 五节。写入侧、
  schema、存量 route 未在范围内且确未改动。
- Findings:
  - [P1] `R1-F1` 全量测试并未通过，`./...` PASS 的记载不成立。
    `internal/desktop` 的 `TestCanonicalFixturesAreReproducibleProducerOutput/
    snapshot-complete.json` 失败：本次判定把 `before_adoption` 从 0 改成 2，而
    `desktop/fixtures/v1/snapshot-complete.json:97` 未同步。HEAD 上同一用例通过，
    故由本次改动引入。该夹具是 Go 与 macOS App 共用的 wire 契约样本
    （`apps/macos/AgentDeckTests/DesktopWireTests.swift` 等 10 处读它），不是内部
    快照。-> open
  - [P1] `R1-F2` 根因二的金额结论与实测不符，本次改动**未改变任何金额**。
    记录称会话 `7e50334e` 在 `2026-08-10T11:53:36` removal 之后的 169 个事件被
    少算 20%。实测：该会话 removal 之后的第一个事件在 `11:56:55`，而
    `usage_session_routes` 第 38 条 `SessionStart official` 在 `11:55:52` 就已重新
    定段，落在 route 37 区间内的事件是 **0 个**。两个二进制对该会话给出的
    `known_provider_cost` 同为 `59.196947200`，全库 `known_provider_cost` 同为
    `8936.463924770`。Beads 授权门的可观测效果描述据此写成，需一并更正。-> open
  - [P1] `R1-F3` 预期结果与验证的量化分母无法复现，实现规则与 修复边界 的规则表
    不是同一条。规则表写「会话跨度内时间线无 provider 切换」，实现取的是
    `snapshotsBetween(client, 段起点, 事件时刻)`（`internal/usage/usage.go:3012`），
    即切换之前的事件仍是 `exact`。实测全库：`exact 73,796 / estimated 1,776 /
    unattributed 20,193`，而预期结果推出的是 `≈68,390 / 6,963 / 20,181`；数据自
    09-01 起只增长了 231 个事件，解释不了 5,187 的差。
    `TestTimelineSnapshotQualityMatchesMeasuredNoRouteDenominators` 断言的是它自己
    声明的常量之和，无法证伪这些分母；`requirements.md` 要求的每档 provider-cost
    金额与占比也没有出现在 验证 一节。-> open
  - [P1] `R1-F4` 没有会话起点时判定窗口塌缩，任何事件都会被判成 `exact`。
    `sessionStartAt`（`internal/usage/usage.go:2773`）在 `sessionStart` 缺失时回落到
    事件时刻，于是 `snapshotsBetween(client, eventAt, eventAt)` 恒为空，
    `timelineSnapshotQuality` 对两个客户端都返回 `exact`，并按事件时刻的快照归因
    ——而这恰恰是无法确定进程加载了哪份配置的情形，改动前它是 `estimated`。
    `TestReadPriceResolverFallsBackToEventAtWithoutSessionStart` 与
    `TestReadPriceResolverUsesSessionRouteOnlyAfterBoundary` 的 `before` 用例都已把
    这一行为固化为期望。本机当前 0 个事件命中（`usage_events` 左连
    `usage_sessions` 无空 `first_at`），属潜伏缺陷。-> open
  - [P2] `R1-F5` Claude `ConfigChange` 的 `prior` 既可能晚于 route，也可能自身未被
    采纳。无前序 route 时 `prior` 取 `sessionStartAt(event)` 处的快照
    （`internal/usage/usage.go:2957`），而 route 可以早于 `first_at`：会话
    `9c776945` 的 route 135 在 `03:54:33`、`first_at` 在 `03:55:23`；会话
    `8aa56214` 的 route 97/98 比 `first_at` 早 8 分 43 秒。有前序 route 时
    `match.prior` 是原始行而不是该 route 的判定结果，因此连续两条未采纳的
    ConfigChange（`8aa56214` 97→98、`d469cbda` 96→99 即此形状）会把一个同样未被
    采纳的 provider 当作留存值。本机这两处恰好各自得到正确答案，代码没有机制在
    它们不正确时发现。-> open
  - [P2] `R1-F6` 每个事件重新加载整条 provider 时间线。
    `Service.eventAttributionForEvent`（`internal/usage/usage.go:3673`）在
    `Service.summarize` 的逐事件循环里调用 `s.Store.LoadProviderTimeline(ctx)`。
    实测 `usage summary --no-scan`（95,765 事件）：HEAD 9:18 / 543.6s user，
    候选 8:26 / 565.5s user——墙钟未变差（省下的逐事件 `ProviderSnapshotAt`
    与 `ProviderTimelineExists` 抵消了它），但 user CPU 上升约 4%，且开销从此
    随时间线行数增长，没有任何东西约束它。既有的 6 次查询预算测试只覆盖
    `readPriceResolver` 那条路径。-> open
  - [P2] `R1-F7` 前提声明的 1–2 分钟窗口有反例，且该风险在本机已命中一个会话。
    会话 `8aa56214` 的 route 比 `first_at` 早 8 分 43 秒，说明窗口不以 1–2 分钟为界；
    无 route 的 Claude 会话 `41b64b36` 的 `first_at` 只比
    `2026-08-06T07:54:16` 那次「无 key → 首次加 key」晚 63.3 秒，正是记录声明接受的
    那种情形，其 66 个事件现在被报为 `exact`。记录却写「有可采纳转换的会话为 0 个」，
    读起来像风险未曾发生。-> open
  - [nit] `R1-F8` `HasClient` 与 `ProviderTimelineExists` 在切到 `HasClientAt` 后
    已无生产调用方，只剩测试引用。-> open
  - [nit] `R1-F9` `TestAttributionResolversClassifyRouteEffectFromPriorState` 的
    `first-key` 与 `no-prior` 两个用例名与其新期望（`estimated`）相反：它们现在
    描述的是凭据证据不足而 fail closed，不是 first-key 被采纳。-> open
- Evidence:
  - `scripts/run-go-test.sh ./internal/usage ./internal/store`：PASS。
  - `scripts/run-go-test.sh ./...`：**FAIL**，唯一失败为
    `internal/desktop/TestCanonicalFixturesAreReproducibleProducerOutput/
    snapshot-complete.json`（`before_adoption` have 0 / want 2）；同一用例在 HEAD
    worktree 上 `ok`。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - RED 复现：把 `TestAttributionResolversClassifyTimelineDeterminability`、
    `TestAttributionResolversKeepPositionedTimelineGapUnattributed`、
    `TestAttributionResolversRetainPriorForLegacyClaudeConfigChange` 逐字抽到 HEAD
    worktree 单独运行——前者 6 个子用例与后者 2 个子用例按目标原因失败
    （timeline 仍 `estimated`、`coverage_gap` 误标、removal 返回 `official/1` 而非
    `cubence/1.2`），中间那个作为 GREEN 守卫在 HEAD 上即通过。
  - 全库对比（同一份数据库副本，`usage summary --no-scan --format json`）：
    HEAD `exact 25,384 / estimated 50,188 / unattributed 20,193`、
    `before_adoption 0 / coverage_gap 20,193 / ambiguous_route 222`；
    候选 `exact 73,796 / estimated 1,776 / unattributed 20,193`、
    `before_adoption 20,193 / coverage_gap 0 / ambiguous_route 0`；
    `known_provider_cost` 两侧同为 `8936.463924770`。
  - `desktop snapshot`「今天」档：HEAD `exact 121 / estimated 77` 且带
    `estimated attribution` 警告，候选 `exact 198 / estimated 0` 且无警告。
  - SQL（数据库只读副本）：会话 `7e50334e` removal 之后共 169 事件、其中落在
    `[11:53:36, 11:55:52)` 的为 0；Claude 无 route 会话 95 个 / 5,871 事件，
    Codex 无 route 会话 200 个 / 58,558 事件。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界，该流程缺口
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN

### 下一步指令

修复：fix / attribution-determinability / R1-F1 R1-F2 R1-F3 R1-F4 R1-F5 R1-F6 R1-F7 R1-F8 R1-F9

## Repair — Round 1 — 2026-09-02

- `R1-F1` closed: canonical `snapshot-complete.json` 由 producer 重生成，普通
  reproducibility 模式随后通过；共享 fixture 不再落后于 Go producer。
- `R1-F2` closed: 现象、根因二、不做事项与预期结果均改为本机当前金额不变；
  removal 到下一条 `SessionStart` 之间为 0 事件。已关闭的 Development Gate 描述
  同步更正，不再声称 169 个事件少算 20%。
- `R1-F3` closed: `storedEvent` 加载 `last_at`，timeline quality 使用完整
  `first_at..last_at` 会话跨度；删除以自声明常量冒充实测分布的测试。预期结果改为
  绑定只读快照的全历史事件分母，以及 30 天每档 known provider cost 与 share。
- `R1-F4` closed: `sessionSpan` 显式返回起点是否可知；缺失 `first_at` 时 snapshot
  仍可给出 provider/multiplier，但 quality 固定为 `estimated`。两个原先固化错误
  `exact` 的回归均已恢复为 `estimated`。
- `R1-F5` closed: positioned match 携带 route 全前缀，读取侧按顺序折叠有效状态；
  首条 `ConfigChange` 的 prior 取当前 selection 之前的 timeline snapshot，不再取
  可能晚于 route 的 `first_at`。route-before-first_at 与连续两条未采纳变更均有
  双 resolver 回归。
- `R1-F6` closed: `Service.summarize` 改用一次加载的 `readPriceResolver`；1003 个
  事件的 Summary 查询预算由 RED 的 5016 次约束为至多 6 次。
- `R1-F7` closed: 删除 1–2 分钟上限与“风险接受”结论，记录 8 分 43 秒反例和本机
  已命中的 63.3 秒窗口；Claude 无 route quality 统一 fail closed 为 `estimated`。
- `R1-F8` closed: 删除无生产调用的 `ProviderTimeline.HasClient` 与
  `Store.ProviderTimelineExists`，测试统一到定位时语义 `HasClientAt`。
- `R1-F9` closed: 两个凭据证据不足的用例改名为
  `unresolved-current-selection` 与 `unresolved-first-key-selection`。
- Verification: 聚焦 RED/GREEN、`internal/usage`、`internal/store`、
  `internal/desktop`、canonical fixture producer/reproducibility、全仓 Go L2、
  whitespace 与 diff check 均按 验证 一节通过。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界；流程缺口仍
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN — R1-F1 through R1-F9 repair complete, awaiting independent
  Re-review.

## Re-review — Round 2 — 2026-09-02

- Reviewed state: HEAD `75767f11b5942789e5692f0b1cffc219980a02a7` 加未提交工作区，
  blob:
  `99d60867f84b25f76a559d491abc90e63bf64050 internal/usage/usage.go`、
  `07a2393b86ef17483aec98b792fd1df4803961c2 internal/usage/usage_test.go`、
  `19f570ea39cb375e75a70491f6d7faaa2af3b930 internal/usage/presentation_test.go`、
  `f1f4076479fd40ec01e1cfc4d8eca7dbb50153f3 internal/store/providers.go`、
  `bb2ec1da643d1c4770a259324f7c091b216ca7c6 internal/store/providers_test.go`、
  `c091f8cadc6d4b1d71f5e4020a13aecf1372478b desktop/fixtures/v1/snapshot-complete.json`、
  `e34a2025d7c53184bf6bc503f3b05d95988be68e docs/fixes/attribution-determinability.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 逐条核对 Round 1 的九条 finding 处置。评审自取一份 `VACUUM INTO` 只读
  快照（96,009 事件，SHA-256
  `ef561603126657eef6da8b0be3b46ed444b101bf476635b29396f50f57f58c54`），在同一份
  数据上分别运行 HEAD、Round 1 候选与本轮候选的 `usage summary --no-scan`、
  `usage sessions`、`desktop snapshot`，逐会话比较 `known_provider_cost`；全量
  `scripts/run-go-test.sh ./...`。
- Findings:
  - `R1-F1` closed：`scripts/run-go-test.sh ./...` 全绿（20 个包 `ok`，无 FAIL）；
    `desktop/fixtures/v1/snapshot-complete.json` 的 `before_adoption` 已随
    producer 重生成。
  - `R1-F2` closed：现象、根因二、修复边界与不做事项都改写为「route 37 与下一条
    `SessionStart` 之间 0 个事件」，不再声称 169 个事件少算 20%；Beads 授权门
    描述亦已更正。
  - `R1-F3` closed：`storedEvent` 加载 `last_at`，`sessionSpan` 用完整
    `first_at..last_at`；自声明常量的分母测试删除，改为
    `TestSummaryUsesWholeSessionSpanForTimelineDeterminability` 走真实
    `Service.Summary`。预期结果的数字可复现：评审自取快照上实测
    `exact 58,378 / estimated 17,438 / unattributed 20,193`（记录快照 58,333 /
    17,429 / 20,193，差额与 54 个新增事件一致）；30 天分档实测
    `determinable 29,500 / 3937.207001470 / 71.40%`、
    `inferred 9,243 / 1576.819552150 / 28.60%`（记录 29,455 / 3928.400825470 /
    71.38%、9,234 / 1575.486580150 / 28.62%）。
  - `R1-F4` closed：`sessionSpan` 返回 `startKnown`，缺失 `first_at` 时 quality
    固定 `estimated`；`TestReadPriceResolverFallsBackToEventAtWithoutSessionStart`
    与 `routes_test.go` 的 `before` 用例都已回到 `estimated`。
  - `R1-F5` **regressed** —— 见下方 `R2-F1`。折叠 route 前缀这一半是对的，但
    「首条 `ConfigChange` 用 `snapshotBefore(current.SelectedAt)` 取 prior」引入了
    一个比原缺陷更严重的错误。
  - `R1-F6` closed：`Service.summarize` 改走一次性 `readPriceResolver`，
    并有 `Summary SQL queries for 1003 events` ≤ 6 的预算回归。同一快照上
    `usage summary --no-scan` 由 HEAD 的 9 分 18 秒降到 **3.6 秒**。
    副作用见 `R2-F2`。
  - `R1-F7` closed：新增「Claude 无 route 的证据上限」一节，记入 8 分 43 秒与
    63.3 秒两个反例，删除「风险接受」结论；`timelineSnapshotQuality` 的 claude
    分支统一 fail closed 为 `estimated`。
  - `R1-F8` closed：`ProviderTimeline.HasClient` 与 `Store.ProviderTimelineExists`
    已删除。遗留注释见 `R2-F3`。
  - `R1-F9` closed：两个用例改名为 `unresolved-current-selection` 与
    `unresolved-first-key-selection`，与 `estimated/ambiguous_route` 期望一致。
  - [P1] `R2-F1` new/regressed：`resolvePositionedRoutes` 的
    `snapshotBefore(client, current.SelectedAt)`（`internal/usage/usage.go:2977`）
    与会话生命周期无关，会取到该 selection 之前的那次切换——可能发生在会话开始
    之前很多天——并把它当成「运行中的会话仍在使用的 provider」留存下来。
    受影响的是「首条 route 是 `ConfigChange`、其前无同会话 route」的会话；本机有
    4 个，共 **457 个事件**被归给会话从未使用过的 provider，且 quality 判为
    `exact`、reason 为 `effective_route`：
    | 会话 | `first_at` | route | 留存到 | 事件 |
    | --- | --- | --- | --- | ---: |
    | `9c776945` | 08-22T03:55:23 | 135 official@03:54:33 | akile / 1.2（08-18T09:43 的 selection） | 152 |
    | `93ad151b` | 08-26T08:52:21 | 165 official@09:55:13 | akile / 1.2 | 250 |
    | `adab3e12` | 08-26T05:57:41 | 162 official@06:19:47 | akile / 1.2 | 19 |
    | `3b964752` | 08-11T04:15:52 | 47 official@05:11:29 | cubence / 1.2（08-10T07:23 的 selection） | 36 |
    这几个会话都在相应的 key removal 之后数小时到数天才开始，启动时必然是
    `official`（无 key），留存 1.2 倍率的 provider 既是错的 provider 也是错的
    金额。逐会话对比：HEAD 与 Round 1 候选完全一致（418 个会话 0 处差异），
    HEAD 与本轮候选有 4 处差异，30 天 known provider cost 由
    `5499.688891410` 升到 `5514.026553620`，**+14.337662210**。这直接否定了
    记录中「本机当前金额不变」「HEAD 与修复候选的 `known_provider_cost` 相同」
    两处结论。新增的
    `TestAttributionResolversFoldLegacyClaudeRouteHistory/route before first event
    uses previous timeline state` 把回溯距离设成 0.5 秒，因此固化了近距离的正确
    答案，覆盖不到真实数据里的远距离回溯。-> open
  - [P2] `R2-F2` new：`(*Service).eventAttributionForEvent` 与
    `(*Service).priceForEvent`（`internal/usage/usage.go:3716`、`:3772`）在
    `R1-F6` 修复后已无任何生产调用方，只剩测试。它们仍保留每事件一次
    `LoadProviderTimeline`，而大量回归里 `read` 与 `service` 两个 resolver 的
    一致性断言，现在比较的是生产从不走的那条路径。-> open
  - [nit] `R2-F3` new：`ProviderTimeline.HasClientAt` 的注释仍写
    “Unlike HasClient”（`internal/store/providers.go:240`），而 `HasClient` 已被
    本次改动删除。-> open
- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：
    PASS，20 个包 `ok`，无 `--- FAIL`。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 同一快照 `usage summary --no-scan --format json`：候选
    `exact 58,378 / estimated 17,438 / unattributed 20,193`，
    `effective_route 25,717 / timeline_snapshot 50,099 / before_adoption 20,193`，
    `known_provider_cost 8986.867860980`，耗时 3.586s（HEAD 同命令 9 分 18 秒）。
  - 同一快照 `usage sessions` 逐会话 `known_provider_cost`：HEAD vs Round 1
    候选 0 处差异；HEAD vs 本轮候选 4 处差异（见 `R2-F1` 表）。
  - 同一快照 `desktop snapshot` 30 天 all-client：HEAD
    `determinable 25,495 / 3466.536855360 / 63.03%`、
    `inferred 13,248 / 2033.152036050 / 36.97%`；候选
    `determinable 29,500 / 3937.207001470 / 71.40%`、
    `inferred 9,243 / 1576.819552150 / 28.60%`。
  - SQL（同一快照）：四个受影响会话的 `first_at`、route 与事件数如 `R2-F1` 表。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界，该流程缺口
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN

### 下一步指令

修复：fix / attribution-determinability / R2-F1 R2-F2 R2-F3

## Repair — Round 2 — 2026-09-02

- `R2-F1` closed: 删除 `SnapshotBefore(current.SelectedAt)` 的任意历史回溯。
  positioned route 前缀仍按会话内顺序折叠；首条 `ConfigChange` 晚于 `first_at`
  时以会话 snapshot 作 prior，早于 `first_at` 时只接受与 route 的
  provider/multiplier/wrapper 完全相同的会话 snapshot 作同值佐证，否则保持
  `estimated`。强化回归把 current selection 与 route 拉开两天，修复前错误返回
  `keyB/9/exact`，修复后返回会话实际的 `official/1/exact`；连续未采纳 route 仍
  正确留在最初有效 provider。最终只读快照的全历史
  `known_provider_cost=8988.163881770`，不再包含 Round 2 发现的 457 事件
  1.2 倍率误归因。Development Gate 描述同步删除易漂移的错误金额断言。
- `R2-F2` closed: 删除无生产调用的 `Service.eventAttributionForEvent`、
  `Service.priceForEvent`、`storedSessionRouteAt`、`mergedPriceAt` 与
  `usageModelMatches`。相关测试删除 dead Service path 对照并统一改名为生产
  `TestReadPriceResolver...`；公开 Summary/Presentation 测试继续覆盖消费者路径。
- `R2-F3` closed: `ProviderTimeline.HasClientAt` 注释不再引用已删除的
  `HasClient`。
- Verification: R2-F1 强化用例先按目标原因 RED 后 GREEN；`internal/usage`、
  `internal/store`、canonical fixture reproducibility 与最终
  `scripts/run-go-test.sh ./...` 均 PASS。最终 L0 结果见 验证 一节。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界；流程缺口仍
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN — R2-F1 through R2-F3 repair complete, awaiting independent
  Re-review.

## Re-review — Round 3 — 2026-09-02

- Reviewed state: HEAD `75767f11b5942789e5692f0b1cffc219980a02a7` 加未提交工作区，
  blob:
  `60648d2f2dba9d1d35e76edf2561582d8e690ffb internal/usage/usage.go`、
  `a1dd8dc2e7ffe25f4a508517ff5b62a1d4480118 internal/usage/usage_test.go`、
  `19f570ea39cb375e75a70491f6d7faaa2af3b930 internal/usage/presentation_test.go`、
  `81e5a7418324e2e74b8c79ac3ef8c35091e0f955 internal/store/providers.go`、
  `13f1333c8285de662a7c0ce403c654dd277c4424 internal/store/providers_test.go`、
  `c091f8cadc6d4b1d71f5e4020a13aecf1372478b desktop/fixtures/v1/snapshot-complete.json`、
  `393d3e9342c8570f60041a1150834dec279c8b87 docs/fixes/attribution-determinability.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 逐条核对 Round 2 的三条 finding。评审自取一份独立的 `VACUUM INTO`
  只读快照（96,124 事件，SHA-256
  `a38de7dee491aacf492cc32b14d7f9970175c1ab6e52ee94fba418d7897c4fd3`），在同一份
  数据上运行 HEAD、Round 2 候选与本轮候选的 `usage sessions`、
  `usage summary --no-scan`、`desktop snapshot` 并逐会话比对；全量
  `scripts/run-go-test.sh ./...`；核对被删除符号是否还有生产调用方。
- Findings:
  - `R2-F1` closed：`SnapshotBefore` 及其调用已整体删除，`resolvePositionedRoutes`
    改为「route 晚于 `first_at` 时用会话 snapshot 作 prior；早于 `first_at` 时只接受
    provider/multiplier/wrapper 完全同值的会话 snapshot 作佐证，否则保持
    `estimated`」（`internal/usage/usage.go:2942`）。同一快照逐会话比对
    `known_provider_cost`：**HEAD 与本轮候选 418 个会话 0 处差异**；Round 2 候选与
    本轮候选正好在 `93ad151b`、`adab3e12`、`9c776945`、`3b964752` 四处回退到 HEAD
    值。强化用例 `route long after selection uses matching session snapshot` 把
    selection 与 route 拉开两天且 route 早于 `first_at`，覆盖的正是真实数据的形状。
  - `R2-F2` closed：`Service.eventAttributionForEvent`、`Service.priceForEvent`、
    `storedSessionRouteAt`、`mergedPriceAt`、`usageModelMatches` 全部删除，全仓已无
    引用；对照测试改名为生产路径的 `TestReadPriceResolver...`。删除未过头——
    `Store.ProviderSnapshotAt` 仍有生产调用方 `internal/usage/routes.go:240`。
  - `R2-F3` closed：`HasClientAt` 注释不再引用已删除的 `HasClient`。
  - 前两轮已关闭的行为经复核未被回退：`timelineSnapshotQuality` 的 claude 分支仍
    fail closed 为 `estimated`；`!startKnown` 仍固定 `estimated`；
    `Service.summarize` 仍走一次性 `readPriceResolver`；canonical fixture blob 与
    Round 2 相同。
- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build scripts/run-go-test.sh ./...`：
    PASS，20 个包 `ok`，无 `--- FAIL`。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 金额不变，可交叉验证：HEAD 逐会话 `known_provider_cost` 之和
    `8996.642383770` == 本轮候选逐会话之和 == 本轮候选
    `usage summary --no-scan` 的 `known_provider_cost`；30 天 all-client 两档合计
    HEAD `5523.801076410` == 候选 `5523.801076410`。
  - 预期结果 可复现：本轮快照比记录快照多 38 个事件，实测
    `exact 58,454 / estimated 17,477 / unattributed 20,193`、
    `effective_route 25,793 / timeline_snapshot 50,138 / before_adoption 20,193`
    ——`estimated`、`timeline_snapshot`、`before_adoption`、`unattributed` 与记录
    逐字相同，多出的 38 个事件全部落在 `exact`/`effective_route`。30 天
    `inferred 9,282 / 1584.034476150` 与记录逐字相同，`determinable` 仅多 38。
  - 性能：同一快照 `usage summary --no-scan` 耗时 2.4 秒（HEAD 同命令 9 分 18 秒）。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界，该流程缺口
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: PASS

### Task checkpoint

Task checkpoint：`ad-bug-attribution-determinability`，内容状态 HEAD
`75767f11b5942789e5692f0b1cffc219980a02a7` 加上述七个 blob；完成门
`NOT_REQUIRED`。

提交建议：本次评审边界即一次提交的范围——`internal/usage/usage.go`、
`internal/usage/usage_test.go`、`internal/usage/presentation_test.go`、
`internal/store/providers.go`、`internal/store/providers_test.go`、
`desktop/fixtures/v1/snapshot-complete.json` 与
`docs/fixes/attribution-determinability.md`。工作区里的
`docs/topics/schema-version-signal/` 属于另一个任务，不得一并暂存。贡献者
trailer 按 `.agent-instructions/beads.md` 的 Commit-checkpoint contributor
attribution 从本任务的 Beads 评论解析（实现 codex、评审 claude-code）。

推送建议：目标 `origin/main`，前提是用户明确授权。注意本地 `main` 已有三笔未推送
的提交（`f282002`、`8f75be9`、`75767f1`），一次推送会把它们一并发布。
