---
status: active
created: 2026-10-09
updated: 2026-10-09
---

# CLI：共用事实与发布，保留命令自己的语义

final candidate，尚无独立 PASS。依赖 [需求](../requirements.md)、[架构](../architecture.md)。
现有入口以 [CLI Manual](../../../specs/cli-manual.md) 为准。以下 engine/telemetry 命令是
待实现的设计，不代表现在能调用；原型 CLI 文字是单一英文，舞台解释可双语。

第一阶段无需新增 engine 命令：App 与 CLI 继续使用现有有限扫描协调者和同一数据库。
扫描完成后发布同一 serving 快照，App 得到 publication 通知；只读 CLI 不依赖 App 运行。
以下 App 引擎/engine 命令仅用于有条件的常驻扩展，其收益或持续接收需求成立后才选入实现。
App首次当天优先不是CLI默认报告的新语义。全范围请求仍等有限inventory的相应域完整；
App首批partial publication不等于CLI全量完成。拟新增--no-scan的持久coverage读取；
当前代码只按本次scanErr决定partial，并不自动识别首批历史未齐。
新增行为由[持久覆盖契约](../architecture.md#持久来源覆盖与stored-only读取)供给，不能称为现有行为。
实验使用现有usage stats --period today --no-scan和scan --scope usage验证分阶段可行性，
前者仅供给用量统计，后者仅提前结束当前订阅；不代表其他域或native面板全部已经准备。

## 现有命令接入

| 调用 | 有 App 引擎 | 没有引擎 | 等待与结果 |
| --- | --- | --- | --- |
| `usage stats/summary --no-scan` | 只读已提交数据，常用 scope 可命中预备结果 | 只读数据库，不能启动采集 | 原有 text/JSON 结构、partial/未知规则；不缩小任意过滤范围 |
| 默认 stats/summary、usage signals | 请求有限 scan，join 被覆盖的 watermark | 按需 worker 接受有限任务 | 保留同步扫描及失败后原有 partial 行为；signals 不新增 --no-scan |
| `scan`、usage/session scan | 同一协调者与有界流水线 | 有限 worker；无 lease 后退出 | 保留 scoped receipt、退出码、幂等 request、v1 NDJSON |
| `watch` | 共享 source checkpoints/调度，独立 subscriber | 有限于 watch 生命周期的按需模式 | Ctrl-C 停止 watch，不删除已存结果，不开启独立后台模式 |
| `desktop snapshot --wire-version 1` | 只读既有兼容 snapshot | 只读；不扫描 | 原 v1 envelope，不混入新 metadata |
| proposed `desktop snapshot --wire-version 2` | 读 serving publication | 校验磁盘 snapshot 或 readonly bootstrap | 明确 coverage、日期和 identity；无效不造 0，miss 不扫描 |

CLI 和 App 不分别打开 writer 竞争同一批来源。一次扫描的 inventory/end offsets 冻结，
实时更新持续到达不使命令无限等待。命令退出后停止自身输出，后台后续 publication 不追加 stdout。
默认命令仍按其 freshness 语义执行，不能把较旧数据无提示地作为同步 scan 的成功结果。

TTY progress 只到 stderr、按既有规则合并；pipe/quiet 不出现 spinner 或重写控制码。
JSON stdout 只有一个 command envelope，NDJSON 仅在明确支持的 scan/watch 路径输出事件。
Ctrl-C scan 只 detach 当前订阅者，exit 130，已经接受的有限任务按原契约继续。

## 有条件扩展：Proposed engine 命令

| 命令 | 行为 | 幂等/退出 |
| --- | --- | --- |
| `engine status` | 只读 runtime、mode、active jobs、serving、receiver capability | stopped 也是 exit 0；不可诊断才 exit 1；不 launch |
| `engine start` | 创建显式 independent lease，允许 App 退出后运行 | 已启用返回现状 exit 0；不注册自启动服务 |
| `engine stop` | 移除 independent lease，checkpoint；不终止其他所有者接受的任务 | 没有该 lease 为 exit 0；App lease 存在必须明确说仍 active |

默认 App-attached：

```text
$ agentdeck engine status
Engine: running
Mode: App-attached
Independent mode: disabled
Data: ready · publication 12
Background: rebuilding
Telemetry: disabled
App exit: App-owned work stops
```

独立模式是显式产品选择，普通 scan 不等同于该选择：

```text
$ agentdeck engine start
Independent mode enabled.
The engine continues after the App exits.
Run agentdeck engine stop to disable it.
No login item or system service was installed.

$ agentdeck engine stop
Independent mode disabled.
The App-attached engine remains active.
Existing finite CLI tasks keep their accepted scope.
```

JSON成功使用现有完整公共envelope：schema_version=1、点分command、UTC RFC3339 generated_at、
data、warnings数组、partial布尔。shell engine status对应engine.status，
telemetry config-preview对应telemetry.config-preview，不使用空格command。
status 的 required data 是：
`state` (`running/stopped`)、`mode` (`app/independent/finite/none`)、`independent`、
`serving.state` (`ready/saved/partial/unavailable`)、可用时的 publication_id、
`jobs.state`、`telemetry.state`。无实体字段省略或按 schema 明确 null，不输出伪造的 0 或空 PID。
错误使用原 command error envelope；具体 machine keys 保持英文。
成功JSON在stdout，失败JSON在stderr且stdout为空；失败仍带公共字段和error、data:null。
逐字符标本分别标明text/JSON的stdout/stderr，不能把错误JSON放进无通道区域。

## 首批导入中的stored-only提示

stats/summary --no-scan仍只读已提交数据、exit0，不启动采集；新增读取同事实提交的coverage。
来源未齐/覆盖未知/已知更新未提交分别给source_coverage_partial、source_coverage_unknown、
source_update_pending和partial:true。text提示在stderr，不冒充扫描失败；JSON warnings/partial在stdout，
不混进度。与pricing/scan警告按OR保留。完整有限inventory的相关域与缺口提交后才清提示；
旧库无记录为unknown，不能把已有值当覆盖证明。

```text
$ agentdeck usage summary daily --no-scan
# stdout: normal summary of committed today usage
# stderr:
source_coverage_partial: historical import is incomplete; showing committed data.
# exit 0
```

完整JSON标本使用usage.summary、warnings:["source_coverage_partial"]、partial:true。
可用数字均为合成已提交值；读取错误仍按公共错误契约至stderr、exit1。

不兼容 active engine 不自动被杀死：

```text
# stdout is empty, stderr:
engine_protocol_mismatch: running engine is incompatible.
Stored-only reads remain available.
Restart at a safe checkpoint; no process was terminated.
# exit 1
```

## Proposed telemetry 命令

| 命令 | 用户决策与输出 |
| --- | --- |
| `telemetry config-preview` | 显示 user-level Codex 配置提案；不应用；local token 默认 redacted |
| `telemetry config-preview --reveal-local-token` | 用户显式要求可复制的本地 token 片段；不进入日志/报告 |
| `telemetry enable` / `telemetry disable` | 只启停 AgentDeck receiver 配置；enable 不隐含独立常驻或 Codex exporter 改动 |
| `telemetry status` | enabled、实际 listener 状态、capture_only/reconciled、pending/conflict、coverage 与 gaps |
| `telemetry import <otel-json-file>` | 用户明确指定的真实 OTLP JSON 记录；同一校验/去重，不接受 CPA 文件 |

配置示例中的 port 只是示意，实际由 receiver 配置供给。保留已有用户 exporter 的合并决策，
不让 CLI 自动覆盖配置。启用后 App 未运行且 independent 未启用时 receiver 为 stopped，
status 必须说明；不能只因配置 enabled 就说正在接收。

```text
$ agentdeck telemetry status
Receiver: enabled
Listener: running (App-attached)
Producer configuration: unknown
Connection: unknown (independent HTTP requests)
Metering: capture_only
Reason: exact request identity not verified
Pending observations: 3
Historical coverage: unknown
No pending tokens were added to totals.
```

该状态 exit 0：诊断命令成功说明当前 capability，并不宣称采集完整。
import 的 malformed/unsupported 输入 exit 1；可持久化但 reconciliation pending 时
`partial: true` 并说明未计入，不将 import 成功包装成全部用量已恢复。
重复导入产生 no-op receipt；禁止 silent conflict overwrite、近似匹配或历史推算。

## 终端边界与可访问性

普通 text 不要求 TTY，不引入 curses/全屏 viewer，独立启动/停止不做隐藏确认。
40 列换行完整保留 mode/reason/coverage，不用颜色作为唯一意义；80 列按同样顺序。
正常状态无 ANSI 控制码，`--no-color` 和 `--quiet` 沿用既有规范；错误不被 quiet 吞掉。
status 不以无限 progress 等待 engine；有限 deadline 后说明 unavailable，与 stopped 区分。
配置 token 只在明确 reveal 命令输出，不通过 status、NDJSON、scan receipts 或 health 泄露。

telemetry status默认producer_configured:unknown和last_received_at:null，不以enabled、静默或单次HTTP推断连接。
明确listener/持久化错误才报告异常，idle保留最后有效观察。

## 共享逐字符标本

入口 `?surface=cli&engine=app`。`engine=absent|independent|stopped|mismatch|telemetry|telemetry_unknown|stored_partial|preview`
覆盖 absent、显式独立模式、App lease 保留、版本不匹配、capture-only、未应用配置提案。
`cols=40|80` 及可见 Columns 舞台控件覆盖窄终端与正常宽度；text/JSON 同时按该宽度呈现。
旧 `?surface=cli&scan=waiting` 继续作为同步 scan 的标本；本主题不换旧 stdout contract。

- [App-attached 引擎](prototype/serving/cli-app-en-dark.png)：text/JSON 命令语义对应。
- [capture-only](prototype/serving/cli-telemetry-en-light.png)：未计入的观测与历史 unknown 明示。
- [版本冲突](prototype/serving/cli-mismatch-en-dark.png)：stdout 空、stderr 原因、exit 1。

共享 [manifest](prototype/serving/manifest.json) 和 [checks](prototype/serving/checks.json)
绑定源码/图像/实际浏览器检查范围。标本不启动真实引擎、不修改配置，不证明 IPC、
真实 Ctrl-C、App quit 或 PTY 的 runtime 行为；这些属于 implementation acceptance。

## Round 1 修复标本

[首批stored-only覆盖提示](prototype/serving/cli-stored_partial-en-dark.png)同时展示完整
usage.summary公共JSON、warnings/partial和成功读取exit0；[未知producer](prototype/serving/cli-telemetry_unknown-en-dark.png)
保留unknown与null。app/telemetry/mismatch截图已按修复后的完整envelope和stdout/stderr重生成。
40/80列、全部九种场景的字段与通道检查见[repair-checks](prototype/serving/repair-checks.json)。
