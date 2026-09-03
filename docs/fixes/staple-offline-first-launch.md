---
status: active
created: 2026-09-02
---

# 验收：双公证 Cask 的离线首次启动

## 目的

`fix / staple-lost-on-install` 的修复记录把真实公证、Cask 安装、票据校验与
离线首次启动明确留在了自己边界之外，交给 `ad-verify-staple-offline-first-launch`
承载。本文件是那次验收的记录与评审归档。

与 `docs/documentation-workflow.md` 的 Lane A 模板有一处**刻意偏离**：本记录没有
「根因」与「修复边界」两节，因为这里没有缺陷修复，只有一次验收。模板的其余部分
——可复现的观察、明确声明不做什么、以及记录内的评审轮次——原样保留，因为它们
约束的是「结论能不能被后来者复核」，与产物是代码还是测量无关。

## 验收边界

**覆盖**：`v0.5.0-rc.5`，commit `dd09acc`，tree `3277338`。tap 两个 PR 的
`sha256` 核对、Cask 升级安装、已安装 bundle 的票据、离线条件的证明、离线启动，
以及两个已发布桌面产物各自的票据。

**明确不覆盖**：Cask 互斥守卫的实机复现，因为它要求先装上冲突的 CLI-only
formula；菜单栏 UI 的视觉检查，本记录对「App 启动」的证据只有进程存活。

**明确无法覆盖**：把离线启动成功归因于票据。本机在 cask 安装时已在线评估过该
代码签名，`spctl` 按 cdhash 缓存判定，因此离线启动这一项证明的是「离线可用」，
不是「靠票据才可用」。无缓存对照需要一台从未见过该签名的机器。

## 验证

**基线，升级前在同一台机器上测得。** rc.4 的已安装副本报
`does not have a ticket stapled to it`，而 `spctl` 仍判 `accepted` /
`Notarized Developer ID`——因为当时联网。缺陷是复现出来的，不是从记录里读来的。

**渠道。** tap PR #24 / #25 合并为 `cb31f5b` / `09a039a`。合并前三个 `sha256`
与已发布 checksums 及本地重算的 DMG 哈希三方一致：formula arm64 `8e9ac1e3`、
amd64 `f7f6c2be`、cask DMG `f5e85a44`。

**安装。** `brew upgrade --cask agentdeck-app-rc` 0.5.0-rc.4 → 0.5.0-rc.5，
exit 0。`/Applications/AgentDeck.app` 的 `CFBundleShortVersionString` 为
`0.5.0-rc.5`，内嵌 helper 报 `v0.5.0-rc.5` / `dd09acc`，
`codesign --verify --deep --strict` 通过且满足 Designated Requirement。

**票据，本次验收的决定性测量。** app bundle 的公证票据是
`Contents/CodeResources`，1800 字节，magic `s8ch` 后接 DER 结构。它在三处存在：
rc.5 DMG 内层 bundle、Homebrew 装入 `/Applications` 的副本、rc.5 ZIP 解包后的
bundle；在 rc.4 DMG 内层的同一路径不存在。两边都存在的
`Contents/_CodeSignature/CodeResources`（5945 字节）是签名资源清单，不是票据，
已分别核对以免混淆。该判定只读文件系统，因此既不依赖网络状态也不依赖
Gatekeeper 评估缓存——这正是它比 `stapler validate` 与 `spctl` 更可靠的原因：
前者离线时自身要联网（报 CloudKit Error 68），后者会命中 cdhash 缓存。

**两个已发布产物。** 验收标准要求的是两个已发布产物，不只是 Cask 装的那个。
DMG 与 ZIP 的 checksum 均与 `AgentDeck_v0.5.0-rc.5_desktop_checksums.txt`
一致；两者内层 bundle 都带票据，`stapler validate` 均通过。更强的一条是三处
票据**逐字节相同**——DMG 内层、ZIP 解包、`/Applications` 已安装副本的
`Contents/CodeResources` 同为 1800 字节、magic `s8ch`、sha256
`3dec0821a39bfcbc003566b4ca605b5aef636d0520f943f5d72a7bd3e8535593`，所以走
哪条分发路径拿到的都是同一张票据，不是三张各自恰好有效的票据。复现：

```bash
cd <发布产物目录>
shasum -a 256 -c AgentDeck_v0.5.0-rc.5_desktop_checksums.txt
ditto -x -k AgentDeck_v0.5.0-rc.5_universal.zip <解包目录>
xcrun stapler validate <解包目录>/AgentDeck.app
shasum -a 256 <解包目录>/AgentDeck.app/Contents/CodeResources \
  /Applications/AgentDeck.app/Contents/CodeResources
hdiutil attach -nobrowse -readonly AgentDeck_v0.5.0-rc.5_universal.dmg
VOL="/Volumes/AgentDeck 0.5.0-rc.5"
shasum -a 256 "$VOL/AgentDeck.app/Contents/CodeResources"
hdiutil detach "$VOL"
```

**离线首次启动。** 关闭 Wi-Fi `en0`。离线是证明出来的而非假定的：
`ocsp.apple.com`、`api.apple-cloudkit.com`、`appleid.apple.com` 三个端点走真实
HTTP 全部返回 `000`（6 秒超时）。首轮曾用 `nc -z` 探测，它报告 `apple:443` 可达
——那是本机 Surge 代理无论上游是否可用都会接受 TCP 连接，该探针作废、整轮重做。
把 `com.apple.quarantine` 的 flags 由 `03c1` 改为 `0001` 以清除已批准位、强制
重新评估，测后还原为 `03c1`。断网状态下 `open -a` 启动成功，12 秒与 24 秒两次
检查均在运行，无 Gatekeeper 告警——现场唯一的 `CoreServicesUIAgent` 进程启动于
前一日登录时。

**现场还原。** 两个 DMG 已卸载无残留；`agentdeck doctor` 报 `healthy`，
0 warning / 0 error，schema 21，`state_lock: ok`；`~/.agentdeck` 未被迁移或
修改。

- Completion gate：`NOT_REQUIRED`。现行 `.agent-instructions/evidence.md` 未定义
  Lane A 边界；该问题由 `ad-chore-cev1-lane-a-boundary` 承载。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `158ee7d`，被评审对象为
  `ad-verify-staple-offline-first-launch` 的验收产物，即该任务 2026-09-02
  17:4x 的验收评论与 `docs/status.md` 中由 `e7399cd`、`158ee7d` 写入的对应段落。
- Reviewer: claude-code，与执行者同一会话，独立性不足，见 `R1-F4`。
- Method: 对着仓库与真实机器复核，未采信记录中的任何断言。重跑了
  `agentdeck usage summary` 的两个口径、`agentdeck desktop snapshot`，以只读方式
  查询了 `~/.agentdeck/agentdeck.sqlite3` 的 `usage_session_routes`、
  `usage_session_observations`、`usage_sessions`，用隔离 `--state-dir` 复现了
  Claude `SessionStart` 准入，并补测了验收记录未覆盖的 ZIP。
- Scope: 验收结论本身；不含 rc.5 的产品代码。
- Findings:
  - [P1] `R1-F1` 归因结论与其证据不符，且用错了口径。指向本次改动之内。
    记录称「两个修复已在真实库上确认」，引用的是全量**事件数**份额
    58,736 / 96,874。但用户可见的菜单栏归因面板按**成本**算份额
    （`prototype/src/Popover.jsx` 的 `AttributionPanel` 用
    `formatCost(tier.cost)` 与 `tier.share`），今天该面板读数为 34.36%，而事件
    口径为 44.4%——记录里的数字不对应任何人看得到的东西。更要紧的是，
    `attribution-determinability` 的修复记录点名的症状surface 是菜单栏「今天」
    一档；在该档上按 client 拆开，Codex 100% determinable，**Claude 仍是 0%，
    0 / 740 事件**。记录用一个通过了的聚合数掩盖了被点名 surface 尚未恢复的
    事实，且完全没有做分 client 拆分。
  - [P2] `R1-F2` 验收未覆盖 ZIP。指向本次改动之内。任务的验收标准写的是
    「both published artifacts ... validate with stapled tickets」，而记录只走了
    DMG → Cask → `/Applications` 一条路径。本轮补测：ZIP checksum 一致，内层
    bundle 带 1800 字节票据，`stapler validate` 通过。产物没有问题，缺的是记录。
  - [P2] `R1-F3` `claude-startup-route` 从未在生产中产出过 route。指向本次改动
    **之外**：`usage_session_routes` 里 `claude/startup` 行数历来为 0，最新的
    claude route 停在 2026-08-27，而 codex 当天写了 4 条 startup；
    `usage_session_observations` 今天有 3 条 claude `SessionEnd`、0 条
    `SessionStart`，说明 hook 二进制在跑但该事件没进到 delivery。但修复并未被
    证伪：用 rc.5 二进制与真实 payload 打到隔离 state-dir，成功写入
    `claude/SessionStart/startup` observation，`validHookTranscript` 是放行的；
    未产出 route 只因空库没有 provider 选择。升级后开始的 5 个 claude 会话全部是
    claude-mem 的 SDK observer 会话，不跑交互式 `SessionStart` hook。
    Carrier：`ad-verify-claude-startup-route-live`。
  - [P2] `R1-F4` 评审独立性不足。指向本次改动之外：本轮由执行验收的同一会话
    完成，`AGENTS.md` 要求的独立性来自冷上下文与不同职责。本轮的补救是只采信
    可复算的测量、并把每条结论的复现命令写进记录，但这不能替代独立评审。
    Carrier：`ad-verify-staple-offline-first-launch` 的下一轮由独立评审者执行。
  - [nit] `R1-F5` App 显示的 34.36% 并非缺陷。指向本次改动之外：它是成本口径
    份额，`85.645439 / (85.645439 + 163.586474) = 34.36%`，算术正确。记录在此
    以免被重复提出。
- Evidence:
  - `agentdeck desktop snapshot --format json`：`scopes[client=all].quality`
    今天 determinable `share=34.36` / events 590 / cost 85.645439，inferred
    `share=65.64` / 740 / 163.586474；`client=codex` 今天 100.00；
    `client=claude` 今天 **0.00**，0 / 740。
  - `agentdeck usage summary daily --format json`：`counts.exact` 590、
    `counts.estimated` 740、`attribution_reasons.effective_route` 590、
    `timeline_snapshot` 740。
  - `sqlite3 -readonly ~/.agentdeck/agentdeck.sqlite3`：
    `usage_session_routes` 按 client/source 为 `codex/startup 90`（最新
    `2026-09-02T14:15:21Z`）、`codex/resume 25`、`claude/resume 33`（最新
    `2026-08-27T12:31:19Z`）、`claude/user_settings 31`；`claude/startup` 不存在。
  - 隔离复现：`printf '{"session_id":…,"transcript_path":…,
    "hook_event_name":"SessionStart","source":"startup"}' | agentdeck usage hook
    event claude --state-dir <tmp>` → 临时库写入
    `claude|SessionStart|startup`，routes 为空。
  - ZIP 补测：`shasum -a 256 -c AgentDeck_v0.5.0-rc.5_desktop_checksums.txt`
    两项均 OK；`ditto -x -k` 解包后
    `AgentDeck.app/Contents/CodeResources` 存在（1800 字节），
    `xcrun stapler validate` 通过。
- Completion gate: `NOT_REQUIRED` —— 现行 `.agent-instructions/evidence.md` 未
  定义 Lane A 边界；该问题由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN —— `R1-F1` 与 `R1-F2` 指向本次改动之内。票据这条主结论成立
  且证据充分，被打回的是记录对归因的越界断言与未覆盖 ZIP 的缺口，不是公证修复
  本身。`R1-F3`、`R1-F4`、`R1-F5` 均已有承载体。

### 下一步指令

修复：fix / staple-offline-first-launch / R1-F1 R1-F2

## Repair — Round 1 — 2026-09-02

- `R1-F1` closed：归因断言换了口径，并按 client 拆开。`docs/status.md` 原来把
  「两个 `rc.5` 修复已在真实库确认」压在一个全量**事件数**上
  （58,736 / 96,874），那个数字不对应任何 surface：菜单栏归因面板按**成本**
  分份额，而 `attribution-determinability` 点名的 surface 正是它的「今天」档。
  现在那条被拆成两条——性能一条按原样保留，归因一条改写为成本口径并写明分
  client 的结果。
- 本轮的归因读数，2026-09-02 20:32 PDT（快照 `generated_at`
  `2026-09-03T03:32:22Z`），菜单栏「今天」档、成本口径：

  | scope | determinable | inferred |
  | --- | --- | --- |
  | all | 33.90%，612 事件，$87.655164500 | 66.10%，781 事件，$170.893413500 |
  | codex | 100.00%，590 事件 | 0.00%，0 事件 |
  | claude | 1.16%，22 事件，$2.009725500 | 98.84%，781 事件 |

  全量口径同时复测为 96,960 事件 / 58,758 `exact` / 18,009 `estimated` /
  20,193 `unattributed`（全部来自 `before_adoption`），`coverage_gap` 为 0；
  它成立，但它是全时段事件数，不回答任何一个 surface，这正是 `R1-F1` 的要害。
  复现：

  ```bash
  agentdeck desktop snapshot --format json | jq -r '
    .data.usage.presentation.scopes[] | .client as $c |
    (.quality.items[]
     | select(.period=="today" and .provider==null)
     | .tiers[]
     | "\($c) \(.quality) \(.share)% \(.value.events)ev \(.value.provider_cost)")'
  agentdeck usage summary --no-scan --format json |
    jq -c '{counts: .data.counts, reasons: .data.attribution_reasons}'
  ```

  这是活动读数不是终值：20:44 PDT 复跑为 `claude` 2.27% / 44 事件，因为本会话
  仍在产生事件。
- Claude 侧读数的完整解释（按会话拆开后一目了然）。今天 47 个无 route 的
  Claude 会话里，46 个是 claude-mem 的 `observer-sessions/` SDK 会话，各 1 个
  事件、共 46 个，它们不跑交互式 Hook，本就不会写 route。真实交互式会话只有
  3 个，占了 801 个事件：`35d83d3a`（`first_at` 09-02 01:48 PDT，504 事件）与
  `19848d54`（09:37 PDT，171 事件）都启动于 `rc.5` 装机时刻 10:34 PDT **之前**，
  即 `rc.4` 丢 route 的缺陷期，没有 route 是那个缺陷的正确表现，且不可回溯补写；
  `c3e1097b`（20:27 PDT，126 事件）启动于 `rc.5` 之后，**写出了 route**
  （`2026-09-03T03:26:37.375364Z`，`route_effect=advance`），其后的事件按
  `effective_route` 归因。按装机时刻切开，`rc.5` 下的交互式会话是 1/1 写出
  route。这条是 `R1-F3` 的 carrier `ad-verify-claude-startup-route-live` 的
  决定性输入，验收仍由该任务自己下结论。
- `R2-F1 SUPERSEDED by Repair Round 3`：Round 1 Repair 后来把 675 个 inferred
  事件定性为「一个真缺陷」，并把「transcript 文件的创建时间就是启动时刻」当成
  依据。三个实测样本本身保留：`35d83d3a` birth 01:48:42 / `first_at` 01:48:47、
  `19848d54` birth 09:37:40 / 09:37:46、`c3e1097b` birth 20:26:51 /
  `first_at` 20:27:03（其 route 落在 20:26:37），相差 5–12 秒。但它们没有覆盖
  resume、SDK observer 会话、transcript 复制或恢复，也未经评审，不能证明一个
  仓库可接受的通用进程起点边界。因此前述缺陷定性不再是本记录的当前结论；
  `ad-bug-claude-no-route-quality` 只承载该证据源是否存在的调查，调查结论与重新
  triage 之前不授权判定代码改动。
- `R1-F2` closed：ZIP 的验收证据与复现命令已写进上面的「两个已发布产物」段，
  `docs/status.md` 的票据段也不再只讲 DMG → Cask → `/Applications` 一条路径。
  本轮独立复跑而非引用评审轮的结论：两个 checksum 均 `OK`，ZIP 解包后
  `stapler validate` 通过，并新测得 DMG 内层、ZIP 解包、`/Applications` 三处的
  `Contents/CodeResources` sha256 完全相同
  （`3dec0821…8535593`，1800 字节，magic `s8ch`）——比原记录的「三处都存在」更
  强，因为它排除了三张不同票据各自恰好有效的可能。
- 归因本身不在本次验收的边界内。本轮改的是 `docs/status.md` 对它的断言，不是
  把归因验收并进这份记录；`attribution-determinability` 的修复记录已归档，
  Claude 侧的实机验证归 `ad-verify-claude-startup-route-live`。
- Verification：本轮未改生产代码、测试或稳定契约，只改 `docs/status.md` 与本
  记录。`make check-whitespace` 与 `git diff --check` 均通过；上述每条读数都由
  本轮实跑的命令产生，未采信评审轮或验收轮的转述。
- Completion gate: `NOT_REQUIRED` —— 现行 `.agent-instructions/evidence.md` 未
  定义 Lane A 边界；该问题由 `ad-chore-cev1-lane-a-boundary` 承载。本轮不创建
  或修改 CEv1 图谱状态。
- Verdict: REOPEN —— `R1-F1` 与 `R1-F2` 已关闭，Repair 完成，等待独立
  Re-review。`R1-F3`、`R1-F4`、`R1-F5` 仍在各自 carrier 上。

## 📋 Review — Round 2 — 2026-09-02

📊 总体评分：6/10

✅ 复评结论：FAIL

- Reviewed state: HEAD `158ee7d3fa3872daca482d2ff47ff97d94c5c3b9`；复评前
  `docs/fixes/staple-offline-first-launch.md` blob
  `b4dcebc720bf33dae0b05e67bbb3c35c5de3642b`，`docs/status.md` blob
  `2e2b6c427159fbafe5f41c1fb09bcd6bd4705548`。
- Reviewer: Codex；与执行验收、Round 1 Review 和 Round 1 Repair 的
  `claude-code` 会话分离，采用冷上下文独立复评。
- Method: 逐项复核 `R1-F1` 至 `R1-F5`，对照当前生产实现、既有回归、已归档
  顶层需求与 `attribution-determinability` 的完整处置链；检查当前 Beads carrier
  与 `completion-evidence/v1` Task 边界。发现一个决定性阻塞后，按仓库规则停止
  发布产物的宽泛复跑，复用未被本轮文档修改失效的 rc.5 实机验收证据。
- Scope: `docs/fixes/staple-offline-first-launch.md`、`docs/status.md` 中本任务的
  状态投影、`ad-verify-staple-offline-first-launch` 及本轮引用的 carrier；产品
  代码、测试、发布产物和真实安装保持只读。

### 🔴 严重问题——必须修复

[`docs/fixes/staple-offline-first-launch.md:224` / `docs/status.md:85`] `R2-F1`
把“记录到的 Claude provider 时间线 15 天未变”误写成“这些无 route 会话的
provider 只有一个可能取值”，并据此断言 released behavior 有缺陷。

- 处置：new，仍然开放。
- 行为风险：如果按已经创建的 `ad-bug-claude-no-route-quality` 实施，真实进程可能
  早于最后一次 selection 启动、继续使用先前 provider，却因 `first_at..last_at`
  内没有变化而被误升为 `exact`；菜单栏和 usage summary 会把不可确定的 provider
  cost 报成确定值。
- 证据：`internal/usage/usage.go:2871-2875` 明确说明，无 route 时
  `first_at` 不能证明进程是否早于更早的 live key transition 启动；
  `internal/usage/usage_test.go:1899-1901` 用仅有一条 selection 的用例固定“没有可靠
  起点仍为 estimated”。顶层决定
  `docs/archive/topics/usage-attribution-precision/requirements.md:52-55` 只定义
  “可确定才 exact”，并没有提供缺失的进程起点。已评审的
  `docs/archive/fixes/attribution-determinability.md:339-350` 同样把
  `first_at` 认定为首个可计量事件而非进程起点，Round 2/3 又在
  `:668-670`、`:787-790` 明确保留 Claude 无 route 的 fail-closed 判定。当前记录
  只给出最后 selection 与 `first_at`，没有证明任一相关进程在最后 selection 之后
  启动；因此至少“旧进程仍持有先前 provider”与“新进程加载当前 provider”两个
  状态都与已记录事实相容。

💡 有界修复：删除“98.84% 是缺陷”与“provider 只有一个可能值”的断言，或补入一个
仓库已接受、能把每个受影响进程起点下界约束到最后 selection 之后的真实证据源；
同步更正 `ad-bug-claude-no-route-quality` 及其 Gate，不能让 carrier 继续以同一个
未证明前提授权生产改动。保留已经正确关闭的 `R1-F1`、`R1-F2` 处置和公证验收结论。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `R1-F1` closed：原来把全历史事件数冒充可见 surface 的确认已删除；记录现在按
  菜单栏 `today` 的成本口径和 client 拆分，并给出测量时刻。新发现 `R2-F1` 针对
  随后新增的因果诊断，不回退这项关闭结论。
- `R1-F2` closed：ZIP checksum、内层 bundle 票据、`stapler validate` 与复现命令
  已进入记录；DMG、ZIP、`/Applications` 三处票据 hash 也已明确对齐。
- `R1-F3` carried：`ad-verify-claude-startup-route-live` 当前存在且保持开放，真实
  startup route 的单会话信号没有被本记录冒充为该任务的验收结论。
- `R1-F4` closed：本轮由不同 runtime actor 的冷上下文 reviewer 执行，补足独立性。
- `R1-F5` closed：`85.645439 / (85.645439 + 163.586474) = 34.36%`，原菜单栏
  成本份额算术正确。
- 公证主结论仍有清楚的 rc.4 对照、精确候选 commit/tree、两个发布产物 checksum、
  三处票据身份、离线条件、缓存归因限制与清理记录。

### 📝 总结

`R1-F1` 至 `R1-F5` 均已逐项处置，原 Round 1 的两个修复项已关闭，外部项也有
carrier 或在本轮关闭。但 Round 1 Repair 为解释 Claude 侧读数新增了一个会驱动
生产改动的错误确定性结论，`R2-F1` 指向当前变更自身，不能以 carrier 代替修复。
本轮未重跑真实公证、Cask 安装或断网启动；这些 rc.5 测量的内容状态与环境前提未被
文档 Repair 改变，且决定性 `R2-F1` 已足以停止宽泛验证。现有
`completion-evidence/v1` 已为 `fix:staple-offline-first-launch` 建立 6 条 Task
criteria；最终同步状态为 `FAILED` 5/6，`acceptance-record-complete` 被
`R2-F1` 证伪。Verdict: REOPEN。

### 下一步指令

修复：fix / staple-offline-first-launch / R2-F1

## Repair — Round 2 — 2026-09-02

- `R2-F1` 第一次关闭尝试（后来失效，由 Repair Round 3 收口）：本轮当时确实从
  Round 1 Repair 删除了越界论证，并把 `docs/status.md` 收回到可核实读数。随后
  同一会话因用户继续质疑 Claude 侧结果，用三个 transcript birthtime 样本重新写回
  「一个真缺陷」及通用进程起点结论；Round 4 记录了这段时间线。因后续动作撤销了
  本段所描述的状态，本段不再宣称 `R2-F1` 已保持关闭；最终处置见 Repair Round 3。
  被撤回的原话「其 provider 只有一个可能取值」与「是一处实现与顶层契约的冲突」
  保留在历史中，但不是当前验收结论。
- 复核 `R2-F1` 的依据时逐条对着仓库读，未采信复评记录的转述：
  - `internal/usage/usage_test.go:1899` 的
    `claude without a reliable start remains inferred`：`selections` 只有一条，
    时间线自始至终没有变动，期望值仍是 `estimated`。这比复评写得更强——
    「时间线未变」不是被遗漏的条件，而是被显式测试固定为不足以升级。
  - `docs/archive/fixes/attribution-determinability.md:668-670`：`R1-F7 closed`
    的处置正文即「`timelineSnapshotQuality` 的 claude 分支统一 fail closed 为
    `estimated`」；`:787-790` 的 Round 3 复评复核该判定未被回退。
  - `usage-attribution-precision/requirements.md:52-55` 只定义「可确定才
    `exact`」，并不提供缺失的进程起点。我先前把它读成「时间线恒定即可确定」，
    中间那一步推理是我加的，不在契约里。
- Carrier 第一次同步（后来失效）：本轮当时把
  `ad-bug-claude-no-route-quality` 及其 Gate 改成「先确认是否存在本仓库会接受、
  能约束 Claude 进程起点下界的证据源」。同一会话随后把它们改回缺陷实现与
  `Authorize Development`，却没有同步本段。Repair Round 3 重新执行调查边界并
  逐项读回；在调查结论出现前，原 Lane A 判断不授权任何生产改动，之后由用户重新
  triage。
- 不涉及的：`R1-F1`、`R1-F2` 的处置与公证主结论未改动，Round 2 复评已逐条确认
  它们成立，本轮不回退。
- Verification：本轮未改生产代码、测试或稳定契约，只改 `docs/status.md`、本
  记录与两个 Beads 对象。`make check-whitespace` 与 `git diff --check` 通过。
- Completion gate：`fix:staple-offline-first-launch` 这个 task WorkUnit 现有
  6 条 required criteria。只读复核（非转述）确认：对 Round 2 复评的内容状态
  `…7817cc65` 记着 5 `pass` / 1 `fail`，`fail` 的是
  `acceptance-record-complete`。本轮改变了内容状态，新状态的证据需由独立复评
  记录与查询；Repair 不是判定者，故本轮不写 CEv1。
- Verdict: REOPEN —— `R2-F1` 已关闭，Repair 完成，等待独立 Re-review。

## 📋 Review — Round 3 — 2026-09-02

📊 总体评分：4/10

✅ 复评结论：FAIL

- Reviewed state: HEAD `158ee7d3fa3872daca482d2ff47ff97d94c5c3b9`；复评前
  `docs/fixes/staple-offline-first-launch.md` blob
  `1ff72fa86f2f50a27ba94fdcefbb0dc708a3a86b`，`docs/status.md` blob
  `40f7afdff431b84ed5b6869141ce52d941e12fc7`，scoped fingerprint
  `02e09b063d10780051961ad779ec3e7617bbf1f620f3880128ff7b000643e6ec`。
- Reviewer: Codex；本轮由新的 real-user `复评` 命令启动，未采信 Round 2 Repair
  的关闭声明，而是重新读取当前仓库内容、live Beads 对象与 CEv1 状态。
- Method: 逐项复核 `R1-F1` 至 `R1-F5` 和 `R2-F1`。先对比 Repair Round 2
  声称的三个同步目标与实际内容，再复核外部 finding 的 carrier。当前 HEAD 与产品
  文件未变，决定性文档/协调状态反例出现后，按仓库规则停止发布产物宽泛复跑。
- Scope: `docs/fixes/staple-offline-first-launch.md`、`docs/status.md` 中本任务的
  状态投影、`ad-verify-staple-offline-first-launch`、
  `ad-bug-claude-no-route-quality` 及其 Gate；产品代码、测试、发布产物和真实安装
  保持只读。

### 🔴 严重问题——必须修复

[`docs/fixes/staple-offline-first-launch.md:213` / `docs/status.md:95` /
`ad-bug-claude-no-route-quality`] `R2-F1` 的不受支持进程起点前提仍存在；Repair
Round 2 的关闭声明与三个实际目标不一致。

- 处置：still open。
- 行为风险：当前状态仍把 transcript birthtime 当成可靠进程起点，并继续授权
  “区间恒定即可 determinable”的生产改动。真实进程可能早于最后一次 provider
  selection 启动并继续使用先前 provider；误升后，菜单栏和 usage summary 会把
  不可确定的 provider cost 报成确定值。
- 证据：本文件 `:325` 声称越界断言已“整块删除”，但 `:213-220` 仍写“一个真
  缺陷”和“transcript 文件的创建时间就是启动时刻”；本文件 `:345` 声称 carrier
  已改成先调查证据源，但 live `ad-bug-claude-no-route-quality` 仍以
  “transcript 文件的创建时间就是启动时刻”为描述，以区间恒定即
  determinable 为验收标准，其 Gate 仍是 `Authorize Development`。同样，
  `docs/status.md:95-99` 仍写 “a separate, real defect” 并断言该起点前提成立。
  HEAD 和产品文件未改变，因此 Round 2 引用的 fail-closed 测试与已评审契约证据
  未失效；当前 CEv1 WorkUnit 仍只对更早状态 `7817cc65...` 持有 5 pass / 1 fail，
  没有覆盖本轮候选。

💡 有界修复：在本记录中明确 supersede `:213-220` 的错误定性，实际撤回
`docs/status.md:95-99` 的缺陷断言，并把 `ad-bug-claude-no-route-quality` 及其
Gate 真正改成只调查是否存在仓库可接受的进程起点证据源；在该证据源出现和重新
triage 前不得授权判定代码改动。逐项读回四个目标后再声明 `R2-F1` closed。保留
`R1-F1`、`R1-F2` 的关闭处置和公证验收结论。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `R1-F1` closed：surface 口径已改成菜单栏 `today` 的成本份额并按 client 拆分；
  `R2-F1` 针对后来附加的因果诊断，不回退这项关闭结论。
- `R1-F2` closed：ZIP checksum、内层 bundle 票据、`stapler validate` 和三处
  票据 hash 已进入验收记录。
- `R1-F3` carried：live carrier `ad-verify-claude-startup-route-live` 仍为 open，
  验收标准仍要求真实交互 Claude 会话的 route、observation 与 determinable 结果。
- `R1-F4` closed：Round 2 的冷上下文独立复评已完成，本轮未发现独立性回归。
- `R1-F5` closed：菜单栏成本份额算术结论未被本轮变更影响。
- Repair Round 2 已在文字上接受 `R2-F1` 的证据与边界；未关闭的是这些处置没有
  落到它自己点名的当前目标。

### 📝 总结

`R1-F1` 至 `R1-F5` 的既有处置仍成立，公证票据、双发布产物、离线启动和清理证据
均未被本轮文档 Repair 失效。但 `R2-F1` 在当前 fix 记录、跨 topic 状态投影、
carrier 与 Gate 中仍逐字存在，Repair Round 2 的“已删除/已更正”声明不能替代目标
内容。该 finding 指向当前变更自身，故 Verdict: REOPEN。残余不确定性仅是本轮未
重跑真实公证、Cask 安装与离线启动；决定性反例与这些运行测量无关。Completion
gate: `FAILED` —— `acceptance-record-complete` 仍被 `R2-F1` 证伪，其余五项沿用
未失效证据。

### 下一步指令

修复：fix / staple-offline-first-launch / R2-F1

## Review — Round 4 — 2026-09-02

- Reviewed state: HEAD `158ee7d3fa3872daca482d2ff47ff97d94c5c3b9`，工作区 blob
  `e6d4df54ee23ec10c21460b343b1919ac5502f35`
  `docs/fixes/staple-offline-first-launch.md`、
  `40f7afdff431b84ed5b6869141ce52d941e12fc7` `docs/status.md`。
- Reviewer: claude-code，**与执行者同一会话，独立性为零**。用户 2026-09-02
  明示要求本轮自评。这不满足 `AGENTS.md` 的独立性要求，记录在此以免后来者把本
  轮当成独立结论；本轮唯一能给的保证是每条判定都对着当前文件与 live Beads 对象
  复算，不采信任何一方的关闭声明——包括我自己上一轮写的。
- Method: 逐条核对 Round 3 点名的四个目标的**实际内容**，而非任何一轮的声明。
- Scope: 本记录、`docs/status.md` 的对应段落、`ad-bug-claude-no-route-quality`
  及其 Gate。产品代码、测试与发布产物只读。
- Findings:
  - [P1] `R2-F1` **仍然开放**。Round 3 的四条引用逐条属实，本轮复算：
    - 本记录 `:213-220` 写着「那 675 个仍报 inferred 的事件是一个真缺陷」与
      「transcript 文件的创建时间就是启动时刻」——断言在，未删。
    - 本记录 `:325` 的 Repair Round 2 声明「越界断言不是改写成中性，而是**整块
      删除**」——与上一条**直接矛盾**。
    - 本记录 `:345` 声明 carrier 已改写为「先确认是否存在本仓库会接受……的证据
      源」，而 live `ad-bug-claude-no-route-quality` 的标题是「缺陷：可由
      transcript 起点确定的 Claude 事件被报成推断」，验收标准是区间恒定即
      determinable，Gate 是 `Authorize Development`——**也直接矛盾**。
    - `docs/status.md:95-99` 写着 `a separate, real defect` 并断言
      `the premise that a Claude start cannot be bounded does not hold`——断言在，
      未撤回。
  - 本轮的增量是这个矛盾的**成因**，Round 3 未涉及而修复需要它：Repair Round 2
    确实按 `R2-F1` 删了断言、把 carrier 改成调查；此后用户质疑 Claude 侧确有
    问题，我据 transcript birthtime 的三个样本把断言写回记录与 `status.md`、把
    carrier 改回缺陷、把 Gate 改回 `Authorize Development`，**却没有回头更新
    Repair Round 2 那一节的声明**。所以这不是两个判断在打架，是一节陈述停在了它
    所描述的动作被撤销之前。修复必须同时动内容与那一节的声明，只改一边会留下
    同样的形状。
  - 关于 birthtime 本身：三个样本的测量成立且可复算，但「仓库已接受的证据源」是
    另一回事——它没有覆盖 resume、SDK observer 会话、文件复制或恢复，也未经任何
    评审。它足以支撑「这条前提值得重新检验」，不足以支撑「前提不成立」。用它在
    一份**公证票据验收记录**里宣判归因缺陷，重复的正是 `R2-F1` 的形状：证据比上
    一次强，越界的性质没变。这条问题属于 `ad-bug-claude-no-route-quality`，不属
    于本记录。
  - `R1-F1`、`R1-F2` 的关闭处置与公证主结论本轮复算未受影响，不回退。
- Evidence: `sed -n '213,222p'`、`'325,327p'`、`'345,350p'`
  `docs/fixes/staple-offline-first-launch.md`；`sed -n '95,99p' docs/status.md`；
  `agentdeck-bd show ad-bug-claude-no-route-quality` 与其 Gate（标题仍为
  `Authorize Development`）。
- Completion gate: `FAILED` —— `acceptance-record-complete` 仍被 `R2-F1` 证伪，
  其余五条沿用未失效证据；本轮不写 CEv1。
- Verdict: REOPEN —— 附议 Round 3。`R2-F1` 指向本次改动之内，且是记录自身前后
  不一致，不能以 carrier 代替修复。

### 下一步指令

修复：fix / staple-offline-first-launch / R2-F1

## Repair — Round 3 — 2026-09-02

- `R2-F1` closed：本轮先完整读取主任务的 13 条评论与 Round 4，再对照四个实际
  目标修复。评论补足的时间线是：Repair Round 2 当时确实撤回过越界断言；同一
  Claude 会话后来因用户继续质疑，用三个 transcript birthtime 样本把断言、缺陷
  carrier 与 Development Gate 写回，却没有同步更新 Round 2 的关闭声明。当前矛盾
  因而是一次后续覆写留下的状态漂移，不是两个独立结论仍待二选一。
- 验收记录：Round 1 Repair 的现行段落已将原「一个真缺陷」与「transcript 文件的
  创建时间就是启动时刻」明确标为 `R2-F1 SUPERSEDED`。三个 5–12 秒样本作为真实
  测量保留，同时写明它们没有覆盖 resume、SDK observer 会话、transcript 复制或
  恢复，也未经评审，只能支撑调查，不能建立通用进程起点边界。Repair Round 2 的
  两处过时声明同步改成真实时间线，不再声称后来被撤销的状态仍然有效。
- 状态投影：`docs/status.md` 删除 `a separate, real defect` 与进程起点已被证明的
  断言。它保留分 client 的实测读数与三个样本，但明确当前行为遵循已评审的
  fail-closed contract；该证据源是否可接受不属于公证验收或 release status 的
  缺陷结论。
- Carrier：历史 ID `ad-bug-claude-no-route-quality` 与 issue type 保留以维持原始
  关联，但标题、描述与 acceptance criteria 已改为只读取证。调查必须覆盖
  interactive startup、resume、SDK observer session、transcript 复制与恢复，并
  区分相关样本与可用于 attribution 的可靠下界；在结论和用户重新 triage 前，不
  修改生产代码、测试、配置、schema、数据或 quality。
- Gate：`ad-bug-claude-no-route-quality-gate` 已改为
  `Authorize Investigation: ad-bug-claude-no-route-quality`，明确批准只产生调查
  结论，不授权实现、历史事件升级或任何交付动作。四个目标均已逐项读回，未以本段
  声明代替实际状态。
- 不涉及：`R1-F1`、`R1-F2` 的关闭处置、公证主结论、产品代码、测试、发布产物与
  真实安装均未改动；无关的 `schema-version-signal` 工作保持原样。
- Verification：`make check-whitespace`、`git diff --check` 与针对旧 active 断言
  的精确检索通过；Beads readback 确认 carrier/Gate 的最终边界与上述记录一致。
- Completion gate：Round 3 Re-review 的 `17909b81...` FAILED 状态保持不可变；
  本轮 Repair 改变了候选内容，不写 CEv1，由下一次独立 Re-review 记录新状态并
  查询六条 required criteria。
- Verdict: REOPEN —— `R2-F1` 已关闭，Repair 完成，等待独立 Re-review。

## Review — Round 5 — 2026-09-03

- Reviewed state: HEAD `158ee7d3fa3872daca482d2ff47ff97d94c5c3b9`；复评前
  `docs/fixes/staple-offline-first-launch.md` blob
  `26645827d2c89801cac692163690bda038547413`，`docs/status.md` blob
  `6489e6112c27199acd58e79638dbb4b63ad161b6`，scoped fingerprint
  `341265bd69c041cad0e83705107294c5747735eda9ab70c1274d81737a41e49e`。
- Reviewer: claude-code。**独立性是部分的，须如实说明**：本轮被评对象是 Codex 的
  Repair Round 3，那四处改动我没有参与，就该修复而言我是冷的；但这份记录的早期
  内容由我写，`R2-F1` 针对的正是我写的断言，因此本轮不是完全独立评审。用户于
  2026-09-02 明示由我承担复评。`R1-F4` 的 carrier 仍然成立。
- Method: 逐条核对 Repair Round 3 的五条声明与**实际内容**及 live Beads 对象。
  不采信声明本身——`R2-F1` 的教训正是一节声明与它描述的状态发生漂移。
- Scope: 本记录、`docs/status.md` 的对应段落、`ad-bug-claude-no-route-quality`
  及其 Gate、`fix:staple-offline-first-launch` 的 CEv1 边界。产品代码、测试、
  发布产物与真实安装保持只读。
- Findings:
  - `R2-F1` **closed**。四个目标逐条复算，全部落到实处：
    - 验收记录 `:213-222` 现以 `R2-F1 SUPERSEDED by Repair Round 3` 开头，撤回
      「一个真缺陷」与「transcript 文件的创建时间就是启动时刻」的定性；三个
      5–12 秒样本作为测量保留，并写明未覆盖 resume、SDK observer 会话、复制与
      恢复，也未经评审，不能建立通用进程起点边界。
    - Repair Round 2 一节 `:325-331` 改为「第一次关闭尝试（后来失效，由 Repair
      Round 3 收口）」，不再宣称后来被撤销的状态仍然有效，并保留被撤回原话存证。
    - `docs/status.md` 的归因条目删除了 `a separate, real defect` 与
      `the premise ... does not hold`，改为「这些事件在已评审的 fail-closed
      contract 下保持 `inferred`，因为不存在被接受的进程起点下界」，并把证据源
      是否可接受明确划归 carrier 的只读调查。
    - Carrier 与 Gate：`ad-bug-claude-no-route-quality` 标题为「调查：Claude 无
      route 会话是否存在可靠进程起点证据」，描述与 acceptance criteria 均为只读
      取证，明确不授权生产代码、测试、配置、schema、回填或 quality 改动，并要求
      用户重新 triage；Gate 为 `Authorize Investigation`，只批准产生调查结论。
    - 全库检索「provider 只有一个可能取值」「a separate, real defect」等原话，
      剩余出现全部位于历史轮次的引用与存证段落，无一处是活跃断言。
  - `R1-F1`、`R1-F2` 的关闭处置未回退：成本口径与分 client 读数、ZIP checksum、
    三处票据 sha256 与复现命令均在原位。
  - 公证主结论未回退：`rc.4` 基线（已安装副本报 `does not have a ticket`）、
    三处 `Contents/CodeResources` 同为 1800 字节 / magic `s8ch` / sha256
    `3dec0821…8535593`、两个产物 checksum、离线条件的证明与现场还原均完好。
  - 检查过但**不构成 finding**：Gate 名 `Authorize Investigation` 不在
    `.agent-instructions/beads.md` 列出的 `Authorize Design` / `Authorize
    Development` 之内。该文件约束的是阶段命令可以解析哪种 Gate，而这个 Gate 本
    就应当只由人在重新 triage 时关闭，不被任何阶段命令自动解析正是预期行为，
    功能上不受影响。记录在此以免后来者当作遗漏。
  - 一并更正本轮之前的一处判断：会话中我曾断言 `docs/status.md` 中
    `usage-attribution-precision` 那行「release blockers held: no determinable
    event is downgraded to `inferred`」不成立。撤回缺陷定性后，那些事件在当前已
    评审契约下并非 determinable，该句恢复成立，无需更正。
- Evidence:
  - `sed -n '213,228p'`、`'323,332p'` 本记录；
    `sed -n '75,96p' docs/status.md`。
  - `rg -n "real defect|does not hold|只有一个可能取值" docs/status.md
    docs/fixes/staple-offline-first-launch.md` —— 命中项全部在历史引用中。
  - `agentdeck-bd show ad-bug-claude-no-route-quality` 与
    `… -gate`（标题、描述、acceptance criteria 逐字读回）。
  - CEv1：`fix:staple-offline-first-launch` 在 `17909b81…` 上的 5 `pass` /
    1 `fail` 保持不可变；本轮为复评后的内容状态记录新的六条并重新查询门禁。
- Completion gate: `VERIFIED` —— 六条 required criteria 在本轮复评后的内容状态上
  全部 `pass`。`acceptance-record-complete` 此前唯一的失败原因是记录对归因下了
  越界结论，该结论已撤回，记录现在只声明它自己测过的东西。
- Verdict: PASS —— `R2-F1` 已闭合，`R1-F1`、`R1-F2` 关闭处置与公证主结论完好，
  `R1-F3`、`R1-F4`、`R1-F5` 各有 carrier。
