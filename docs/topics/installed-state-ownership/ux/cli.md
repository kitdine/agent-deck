---
status: active
created: 2026-11-05
---

# CLI：安装身份、隔离提示与拒绝

依赖 [需求](../requirements.md)。字段来源和判定规则见 [架构](../architecture.md)。

CLI 输出沿用产品契约：英文文案；JSON 使用现有的命令 envelope 和错误 envelope（`code`、`message`、`details`）；JSON 里不出现文件路径。文本输出可以显示以 `~` 开头的路径，便于人工排查。

## 标本

逐字符输出以共享产品原型为准，本文件不再复制一份，以免两处不一致。原型入口是 CLI 页的 `ownership` 页签：

```text
?surface=cli&ownership=<state>
```

数据在 [`prototype/src/ownership.js`](../../../../prototype/src/ownership.js)，说明见 [原型 README](../../../../prototype/README.md#安装状态所有权标本installed-state-ownership)，结构检查用 [`prototype/tools/installed-state-ownership-check.mjs`](../../../../prototype/tools/installed-state-ownership-check.mjs)。每个状态分开展示 stderr、stdout、退出码和 `--format json`。

| state | 命令 | 证明什么 |
| --- | --- | --- |
| `notice` | `go run ./cmd/agentdeck provider list` | 未安装提示只在 stderr 输出一行，位于命令自身输出之前；JSON 不变 |
| `refused_state` | `./dist/agentdeck --state-dir ~/.agentdeck scan` | 普通命令在访问真实目录之前被拒绝；错误块的格式、`details`；exit 1 |
| `refused_client_config` | `go run ./cmd/agentdeck telemetry enable --write-codex-config` | 写客户端配置被拒绝；exit 1 |
| `revoked` | `agentdeck watch` | 长驻进程失去所有权后，在下一轮开始前退出；exit 1 |
| `doctor_installed` | `agentdeck doctor` | 已安装：`installation` 排在第一个并显示安装方式；同时有 `managed_command_owner` 检查 |
| `doctor_not_installed` | `./dist/agentdeck doctor` | 未安装：给出原因、隔离目录和下一步；不运行 `managed_command_owner`。隔离目录尚不存在，所以沿用现有的缺失目录短路：部分报告，`checks_skipped` |
| `doctor_formula_unclaimed` | `$(brew --prefix)/opt/agentdeck/bin/agentdeck doctor` | 未认领的 formula：`recovery_command` 为 `agentdeck install claim`；同样是缺失目录短路的部分报告 |
| `doctor_refused` | `./dist/agentdeck doctor --state-dir ~/.agentdeck` | 状态目录被拒绝时的部分报告：只有 `installation` 与 `state`，`partial: true`，exit 0 |
| `doctor_command_owner` | `agentdeck doctor` | 接管后，旧拥有者的绝对路径 Hook 被指出，并给出重新登记命令 |
| `claim_ok` | `… install claim` | 认领成功 |
| `claim_owned_by_other` | `agentdeck install claim` | 已有其他拥有者；不显示其路径 |
| `claim_owner_running` | `agentdeck install claim --takeover` | 仍有拥有者会话持有所有权锁，接管被拒绝 |
| `claim_not_current_keg` | `…/Cellar/agentdeck/0.7.0/bin/agentdeck install claim` | 保留下来的旧 keg 不能认领 |

这些标本只证明文案、行序、字段、退出码以及输出位置（stdout 或 stderr），不证明守卫、所有权锁、回执或任何运行时行为；运行时行为由实现任务的测试证明。doctor 标本沿用 `renderDoctorText`（`cmd/agentdeck/main.go:5225`）的头部和检查行格式，与本主题无关的现有检查做了省略。

## 不变的部分

- `agentdeck version` 和 `--version` 的五行文本与 JSON 数据字段保持不变。契约规定 data 字段"恰好"是 version、commit、branch、build_time、go_version 五个，本主题不改。安装身份在 `doctor` 中报告。
- 已安装进程的所有命令输出不变。
- Hook 和 statusline 命令保持现有的静默失败开放（fail-open）契约。它们由已安装程序登记；所有权转移后，旧拥有者以绝对路径登记的命令会被静默拒绝。这种情况不另写任何记录，只在已安装程序 `doctor` 的 `managed_command_owner` 检查中可见。

## 状态矩阵

| 状态 | 状态目录 | 写真实客户端配置 | 文本输出 | JSON |
| --- | --- | --- | --- | --- |
| 已安装 | 照旧，默认是 `~/.agentdeck` 或显式 `--state-dir` | 照旧 | 不变 | 不变 |
| 未安装，未传 `--state-dir` | 隔离目录 `~/.agentdeck-dev` | 拒绝 | stderr 是 TTY 时先输出一行提示（`notice`） | 不变，没有提示 |
| 未安装，`--state-dir` 指向非真实目录 | 使用显式目录 | 拒绝 | 同上，提示中显示该目录 | 不变 |
| 未安装，`--state-dir` 指向真实状态目录或其中的路径，包括经符号链接或 `..` 到达的 | 不访问 | 不适用 | `state_ownership_refused` 错误（`refused_state`）；`doctor` 例外，见下一行 | 错误 envelope，exit 1 |
| 未安装，`doctor --state-dir` 指向真实状态目录 | 不访问 | 不适用 | 部分报告（`doctor_refused`） | 成功 envelope，`partial: true`，`warnings` 含 `checks_skipped`，exit 0 |
| 未安装，命令要写真实客户端配置 | 不涉及 | 拒绝，文件逐字节不变 | `state_ownership_refused` 错误（`refused_client_config`） | 错误 envelope，exit 1 |
| 曾是拥有者的长驻进程（`watch`、监听等），所有权已被转移 | 下一轮开始前关闭 | 不适用 | 以 `state_ownership_refused` 退出（`revoked`） | 错误 envelope，exit 1 |
| 已安装，但取所有权共享锁超时（正在被接管） | 不访问 | 不适用 | `state_ownership_refused`，reason 为 `ownership_busy` | 错误 envelope，exit 1 |

## 未安装提示

只在以下条件同时成立时输出：文本格式、stderr 是 TTY、没有 `--quiet`。每个进程只输出一次，在命令自身的输出之前；不输出到 stdout，JSON 和 NDJSON 都不受影响。格式为：

```text
note: not the installed AgentDeck (<reason>); using isolated state ~/.agentdeck-dev
```

显式传了 `--state-dir` 时，后半句改为 `using state <目录>`。

## 拒绝：真实状态目录

- 错误码 `state_ownership_refused`，exit 1。
- JSON 错误 envelope 写到 stderr，`details` 为 `{"resource": "state_root", "reason": "<原因码>"}`。
- 文本错误块依次为：首行 `<code>: <message>`，然后是 `resource`、`reason`、`next`、`manual prerequisite`、`diagnose`。逐字内容见 `refused_state`；长驻进程失去所有权时的文案见 `revoked`。

拒绝发生在打开、创建、加锁、迁移之前，也发生在访问真实目录里任何内容之前。

## 拒绝：真实客户端配置

`details` 为 `{"resource": "client_config", "reason": "<原因码>"}`，exit 1。逐字内容见 `refused_client_config`。

## `doctor` 的 `installation` 检查

`installation` 是第一个检查，在 `state` 之前。

doctor 在状态目录上短路时，报告都是部分报告（ISO-UX-R3-F1）：只包含 `installation` 和 `state` 两个检查，`partial: true`，`warnings: ["checks_skipped"]`，exit 0。短路分两种：

| 原因 | `state` 检查 | 结尾一行 | 标本 |
| --- | --- | --- | --- |
| 状态目录缺失：现有 `doctor.Service` 的行为（`internal/doctor/doctor.go:72-76`），不变 | `warning (state_missing)` | `checks_skipped: remaining diagnostics require a readable core database`（现有） | `doctor_not_installed`、`doctor_formula_unclaimed` |
| 状态目录被拒绝：本主题新增 | `error (state_ownership_refused)` | `checks_skipped: the requested state directory was refused; no other diagnostics ran` | `doctor_refused` |

新增 `installation` 检查不改变缺失目录短路原有的部分报告语义。只有没有短路的报告才是完整报告，例如 `doctor_installed`、`doctor_command_owner`。

| 情况 | status | code | reason | 文本附加行 | recovery_command |
| --- | --- | --- | --- | --- | --- |
| 已安装 | `ok` | — | 安装方式：`homebrew_cask`、`homebrew_formula`、`make`、`local_app` | `  channel: <安装方式>` | — |
| 未安装 | `warning` | `not_installed` | 下表的原因码 | `  reason:`、`  state: isolated ~/.agentdeck-dev`、`  next:` | 程序位于 Homebrew formula 的 keg 中时，为 `agentdeck install claim`，文本另加 `  recovery:` 行 |

现有渲染器只为少数检查输出附加行，所以这些附加行由新增的渲染分支负责。检查行本身沿用 `name: status (code)` 格式。JSON 只使用现有的 `Check` 字段，不包含路径。

### 状态目录被拒绝时的部分报告

未安装进程的 `doctor --state-dir` 指向真实状态目录时，不返回错误，而是输出一份部分报告：

- 只有 `installation` 和 `state` 两个检查。`state` 为 `error (state_ownership_refused)`，附加 `reason`、`next` 两行。
- 结尾一行是 `checks_skipped: the requested state directory was refused; no other diagnostics ran`。现有部分报告的结尾一行说的是"需要可读的核心数据库"，不适用于这种情形，所以用这句替代。
- JSON 为成功 envelope，`partial: true`，`warnings: ["checks_skipped"]`，exit 0。

这沿用 Doctor 契约：报告成功渲染就以 exit 0 结束，没执行的检查用 `checks_skipped` 说明。逐字内容见 `doctor_refused`。

### `managed_command_owner` 检查

只在已安装程序的 `doctor` 中运行。它只读地检查客户端配置中由 AgentDeck 管理的 Hook 和 statusline 命令，看它们是否指向当前拥有者。

| 情况 | status | code | reason | recovery_command |
| --- | --- | --- | --- | --- |
| 没有受管命令，或绝对路径命令都指向当前拥有者 | `ok` | — | — | — |
| 只有基于 PATH 的 `agentdeck` 命令 | `ok` | — | `path_resolved`（运行时由客户端的 PATH 决定，doctor 无法复现） | — |
| 有绝对路径命令指向别的程序 | `warning` | `managed_command_owner_mismatch` | 受影响的客户端：`claude`、`codex` | 对应的重新登记命令，例如 `agentdeck usage hook setup --client claude` |

不显示那个程序的路径。逐字内容见 `doctor_command_owner`。

### 原因码

`installation` 检查、`state_ownership_refused` 错误和未安装提示共用以下原因码：

| reason | 含义 |
| --- | --- |
| `receipt_missing` | 没有找到可用的回执，或回执登记的程序已不存在 |
| `receipt_invalid` | 回执无法读取、格式错误或版本不支持 |
| `receipt_other_executable` | 回执登记的是另一个可执行文件，包括所有权已被转移的情形 |
| `executable_unresolved` | 无法解析当前可执行文件的真实路径 |
| `real_root_unresolved` | 无法解析真实账户主目录或真实状态目录入口；为安全起见，拒绝所有显式指定的状态目录 |
| `state_path_unresolved` | 指定的状态目录无法安全解析：缺失路径之后有 `..`、链接循环或权限错误 |
| `ownership_busy` | 已安装进程在 5 秒内取不到所有权共享锁，通常是正在接管 |

## `install claim`

供 Homebrew formula 安装的 CLI 认领所有权。这个命令不访问真实状态目录，只写回执。

- 失败时错误码为 `install_claim_refused`，exit 1；JSON `details` 为 `{"reason": "<原因码>"}`。
- 成功时 JSON data 为 `{"channel":"homebrew_formula","takeover":false}`。
- 逐字内容见 `claim_ok`、`claim_owned_by_other`、`claim_owner_running`、`claim_not_current_keg`。

| reason | 含义 |
| --- | --- |
| `not_current_formula_keg` | 当前程序不在 Homebrew formula 的 keg 中，或者不是 opt 链接指向的当前 keg |
| `owned_by_other` | 回执登记的是另一个仍存在的程序，且没有加 `--takeover` |
| `owner_running` | 在 5 秒内取不到所有权排他锁：仍有拥有者的会话在访问真实状态 |

安装方式调用的隐藏命令 `install register` 使用同一个错误码和同一组原因码。它不是面向用户的命令，所以没有单独的标本。

## 文案约束

- 不说 "development build"，因为未安装的原因也可能是回执损坏。统一说 "not the installed AgentDeck / program"；只在 `next` 行里把开发构建当作一种预期情形提及。
- 拒绝文案都提供下一步可执行的出路，并且不提任何绕过开关，因为这样的开关不存在。
- 不显示回执登记的其他程序路径，避免在共享终端输出中暴露本机安装布局。原因码已足够排查。
