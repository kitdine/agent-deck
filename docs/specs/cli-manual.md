---
status: active
created: 2026-07-16
---

# AgentDeck CLI 目标命令手册

本文档定义 Phase 9 CLI usability、内置 `official` provider baseline，以及
credential-owned provider configuration 的正式命令契约。执行状态以
`docs/README.md` 为准。

## 设计原则

- 命令按资源分组，业务命令最多两层：`provider add`、`credential update`、
  `price status`。
- 资源标识使用位置参数；可变属性使用 flags。
- 不保留旧位置参数形式、`provider credential ...` 或 `usage price ...` 兼容入口。
- 用户使用 provider name 和 credential shorthand；`--credential` 是唯一的 shorthand
  flag。完整逻辑 reference 由 AgentDeck 生成并在 credential 输出中展示，不能由用户
  指定。
- 一个 provider 可以拥有多个命名 credential；同一个 credential 可以绑定一个或多个
  client，同一个 provider/client 也可以绑定多个 credential。
- Provider 只是 credential 的逻辑分组。Endpoint、multiplier 和 client bindings 均归
  credential 所有；provider list/show 只聚合展示 clients 和 credential 数量。
- Credential 只有一个底层创建服务。`provider add` 只是原子编排“创建 provider +
  credential”，当 provider 已存在时则直接新增该命名 credential；`credential add`
  只为已存在 provider 增加 credential，不实现第二套生成、规范化或加密写入逻辑。
- credential value 只通过 TTY 无回显输入或标准输入的一行读取，绝不接受命令参数、
  flag 或环境变量。
- 默认 text collection 使用统一的 `+`、`-`、`|` ASCII grid；usage 报告使用其
  专用的宽度感知 section/row primitives；显式 `--format json` 才输出稳定 envelope。
- 面向人的 text 输出把 instant 渲染为本机时区、精确到秒，并明确标出时区：表格时间
  列在 header 中使用 `FIELD (<zone>)`，detail 与行首时间在值后追加 `<zone>`。JSON、
  NDJSON 和存储继续使用 UTC RFC 3339；不含 instant 的输出不虚构时区，无法解析的
  时间值原样保留。
- `official` 是 Codex 内置 provider，不存入 providers 表，不创建 credential，不访问
  credential vault 或 `auth.json`。

## 全局 Flags

| Flag | 含义 | 是否必填 | 示例 |
| --- | --- | --- | --- |
| `--format text\|json\|ndjson` | 输出格式；`ndjson` 仅允许 `watch` | 否，默认 `text` | `agentdeck provider list --format json` |
| `--state-dir <path>` | 覆盖 AgentDeck 状态根目录 | 否，默认 `~/.agentdeck` | `agentdeck doctor --state-dir /tmp/ad-state` |
| `--no-color` | 禁用终端颜色 | 否 | `agentdeck doctor --no-color` |
| `--quiet` | 抑制非必要 text 输出；错误和机器输出不受影响 | 否 | `agentdeck usage scan --quiet` |
| `--verbose` | 在 text 输出中保留完整技术 provenance；JSON 始终保留 | 否 | `agentdeck price history --verbose` |
| `--version` | 输出构建身份并退出 | 否 | `agentdeck --version` |
| `-h, --help` | 显示当前命令帮助 | 否 | `agentdeck provider add --help` |

## Provider

Provider definition 是命名 credential 的逻辑分组。每个 credential 独立持有 endpoint、
成本 multiplier 和 client bindings，并通过 `--credential <shorthand>` 选择。Provider
definition 的 JSON 只包含 aggregate `clients` 和 `credential_count`，不重复 endpoint、
multiplier、reference 或 credential 明细；`provider status` 只通过复数 `credentials`
返回 credential 明细，不再保留单数 `credential` 投影。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `provider list` | 列出 custom 与内置 provider definition；不读取 credential ciphertext | 无命令专属参数 | 无 | `agentdeck provider list` |
| `provider current` | 按 client 显示当前 provider、credential shorthand、选择时间、路由（直连还是经 wrapper）和实际写入的 endpoint；不读取或解密 credential value | 无 | 无 | `agentdeck provider current` |
| `provider show <name>` | 显示一个 provider definition；不检查 credential readiness | `name`：provider name | `name` 必填 | `agentdeck provider show official` |
| `provider status [name]` | 检查全部或指定 provider 的 client、credential readiness 和 active selection；readiness 只检查 secret row 是否存在，不解密 | `name`：可选过滤 | 否 | `agentdeck provider status aigocode` |
| `provider add <name>` | Provider 不存在时原子创建 provider 和 credential；provider 已存在时新增该 credential；相同 metadata 和 secret 已存在时无提示成功 | `--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>`；`--credential <shorthand>` | `name`、`--endpoint`、`--clients` 必填；其余可选 | `agentdeck provider add aigocode --credential codex --endpoint https://api.example.com/v1 --clients codex` |
| `provider update <name>` | 更新一个 credential 的 endpoint、multiplier 或 bindings；未指定字段保持不变，不处理 credential value | `--credential <shorthand>`；`--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>` | `name` 必填；metadata flag 至少一个；credential 唯一时可省略 shorthand | `agentdeck provider update aigocode --credential codex --multiplier 1.2` |
| `provider remove <name>` | 在一个 SQLite transaction 中删除 custom provider、credential metadata 与 ciphertext | 无 | `name` 必填 | `agentdeck provider remove aigocode` |
| `provider use <name>` | 切换 client 到 provider；client 或 credential 唯一时自动推断；`--via` 让本次切换走 provider 的 wrapper URL | `--client codex\|claude`；`--credential <short-name>`；`--config-path <path>`；`--via` | `name` 必填；client/credential 仅在无法唯一推断时必填 | `agentdeck provider use aigocode --client codex --credential work` |
| `provider set-wrapper <name>` | 设置或清除 provider 的 wrapper URL 及其协议声明；只写存储，不切换任何 client | `--url <url>`；`--clear`；`--kind plain\|headroom` | `name` 必填；`--url` 与 `--clear` 互斥且必须二选一；`--kind` 不能与 `--clear` 同用 | `agentdeck provider set-wrapper aigocode --url https://127.0.0.1:8788 --kind headroom` |
| `provider recover` | 检查中断的 `provider use` operations；credential/provider 删除不需要外部 recovery | 无 | 无 | `agentdeck provider recover` |

`provider status` 的 `CODEX ACTIVE` / `CLAUDE ACTIVE` 单元格直接显示 credential
shorthand；未激活以及内置 `official` credential 显示 `-`。指定 provider 的 detail
额外显示逐 client 的 active、credential、selected-at、`ROUTE` 与实际写入的
`ENDPOINT`。`provider list` 增加 `WRAPPER` 列，`provider show` 在配置了 wrapper 时
增加 `wrapper:` 行，`provider current` 增加 `ROUTE` 与 `ENDPOINT` 列；JSON 侧对应
新增可选 `wrapper_url` 字段，以及 selection 上的 `via_wrapper` 与 `endpoint`。

### Provider Wrapper 与 `--via`

Wrapper 是用户自己运行在 upstream 前面的代理（本地或局域网的压缩、日志、路由层）。
它是 provider 拥有的一个可选 URL，不是独立实体，内置 `official` 同样可以配置。

- **每 provider 一个，不按 credential、不按 client 存储**：一个 wrapper 实例对应一个
  upstream 地址；同一实例同时服务两种 client 协议，Codex 追加一次 `/v1`，Claude 直接
  使用 base。
- **归一化规则与 Codex-bound credential endpoint 完全一致**：无论该 provider 实际绑定
  哪些 client，末尾 `/v1` 一律去除，不保留 Claude-only credential 那种末尾 `/v1`。
  归一化会改写你输入的值，而 text 成功输出与 `provider add`/`provider update` 一样
  只说明完成的动作和资源名，不回显存储值；用 `provider show <name>` 或 `--format
  json` 确认实际存下来的 URL。
- **`--clear` 是唯一跳过归一化的路径**：它写入空值表示"没有 wrapper"，而空字符串本身
  不是合法 endpoint。
- **存了 wrapper 不等于会走 wrapper**：路由按每次切换选择，只有 `provider use --via`
  才把 wrapper URL 写进 client 配置。因此插入或移除代理不改变任何存储状态，已配置的
  wrapper 也不会静默影响没有要求它的切换。
- **wrapper 只覆盖 endpoint 字段**：provider 身份、credential、multiplier 和用量归属都
  不变。custom provider 走 `--via` 时仍然写入同一个 credential；`official` 走 `--via`
  时依旧不写 credential，只是让 client 用自己的登录态经代理访问原厂。
- **切换成功后 stderr 打印一行 effective route**（`effective route: <client>
  direct|via wrapper, endpoint <url>`；`official` 直连时为 `no endpoint written`）。
  它只是提示信息：不改变 exit status，不进入 stdout 的 JSON envelope，`--quiet` 下不
  输出。因为写入之后，client 配置本身无法区分直连与经过代理。
- **`--via` 但该 provider 没有配置 wrapper 时命令失败**，且在触及任何 client 配置文件
  之前就失败。
- **`--kind` 声明这个 wrapper 说什么协议**：`plain`（默认）或 `headroom`。AgentDeck
  看不到 wrapper 到底是什么、也从不去探测它，所以协议只能由你明说，不会从 URL 猜。
  声明为 `plain` 与不声明完全等价，在任何命令的输出里都无法区分——`wrapper_kind`
  只在非默认时出现在 JSON 里，文本里也只在非默认时以 `wrapper: <url> (headroom)`
  的形式标注。
- **`set-wrapper` 是整体设置，不是部分更新**。它没有部分更新形式（本 CLI 里承担
  修改语义的是 `provider update`），因此省略 `--kind` 等同于把声明设回默认，和第一次
  设置时省略它是同一回事。当这次调用因此覆盖掉了一个**非默认**的既有声明时，stderr
  会打印一行提示，点明被替换掉的旧值：

  ```text
  advisory: wrapper kind reset to plain (was headroom); pass --kind headroom to keep it
  ```

  它遵循与其它 advisory 相同的规则：只进 stderr，不进 JSON envelope，不改 exit
  status，`--quiet` 下不输出。首次设置、`plain` → `plain`、`headroom` → `headroom`
  以及 `--clear` 都不触发——`--clear` 本来就是你明确要求移除整个 wrapper。

```bash
agentdeck provider set-wrapper aigocode --url https://127.0.0.1:8788
agentdeck provider set-wrapper aigocode --url https://127.0.0.1:8788 --kind headroom
agentdeck provider use aigocode --client codex --via
agentdeck provider set-wrapper official --url https://127.0.0.1:8788
agentdeck provider use official --client claude --via
agentdeck provider set-wrapper aigocode --clear
```

### Project Attribution

把 wrapper 声明为 `headroom` 只是允许项目归属；某个 client 只有在最新一次
`provider use --via` 仍指向该 provider 当前的 Headroom wrapper 时才 eligible。
项目值是当前目录经过安全 percent-encoding 的 basename，不写入 AgentDeck 数据库
或 client 配置。用户已经设置的 attribution 值优先。

直接启动的 Codex 或 Claude CLI 可以通过 managed shell integration 归属；GUI app
启动不在保证范围内。安装 binary 或 command completion 不会配置 shell integration，
package uninstall 也不会修改 startup file。

#### 配置、检查与移除

通常无需单独 setup：成功的交互式 text `provider use --via` 在至少一个 client
变为 eligible、stderr 是 TTY、未使用 `--quiet`、JSON/NDJSON、
`--no-shell-setup`，且用户没有拒绝时，会为当前使用中的 shell 自动配置。
非 TTY、`--quiet`、JSON/NDJSON 或 `--no-shell-setup` 都不会写 startup file。
自动配置失败不回滚已经成功的 provider switch，只改为提示手动 setup。

也可以显式管理：

```bash
agentdeck shell setup [bash|zsh|fish] [--rc <path>]
agentdeck shell status [bash|zsh|fish] [--rc <path>]
agentdeck shell remove [bash|zsh|fish] [--rc <path>]
```

无参数命令覆盖所有 in-use shell：默认 startup file 已存在，或它就是当前 invoking
shell。当前 shell 即使 startup file 不存在也会被包含；仅仅安装了另一个 shell
不会创建它的文件。zsh 使用 `${ZDOTDIR:-$HOME}/.zshrc`，fish 使用
`${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish`；bash 根据 invoking shell
是否为 login shell选择 `.bash_profile` 或 `.bashrc`，同时纳入已经存在的 Bash
startup file。显式 shell 只处理该 shell；非默认路径只能作为唯一目标。
`--shell <bash|zsh|fish>` 与 positional shell 等价；`--rc <path>` 选择
非默认 startup file，因此要求单 shell 操作。

`setup` 幂等安装 AgentDeck-owned block；合法旧版本可升级，相同 block 报
`unchanged`，重复、截断、被编辑或 hash 无效的 region 会拒绝覆盖。
`status` 只读，分别报告：

- 持久配置：`absent`、`configured`、`modified`、`invalid`；
- 当前会话 JSON：`active`、`inactive`、`inherited_from_ancestor`；text 将最后
  一种显示为 `inactive (marker inherited from ancestor shell)`；
- 每个 client 的 route 资格及原因；
- negative-gate marker 与当前资格是否一致。

显式检查一个 startup file 缺失的 shell 仍返回该 shell 的一个 `absent` 结果。
`remove` 只删除校验通过的 AgentDeck-owned block，保留 startup file 其他每个
字节和独立的 completion block；不存在时幂等成功，被编辑或 invalid 时拒绝自动
删除。它打印的当前会话停用命令在函数存在或不存在时都安全。无参数多目标操作逐项
报告并继续处理，成功目标会保留，任一失败使整体失败。

`shell remove` 还记录“不再自动配置”；后续 eligible switch 不会重装。
显式 `shell setup` 清除此选择。

#### Presence guard 与调用成本

受管 block 在生成函数前检查 AgentDeck 是否仍在 `PATH`：bash/zsh 使用
`command -v agentdeck >/dev/null 2>&1`，fish 使用 `type -q agentdeck`。
binary 不存在时 block 静默惰性，Codex/Claude 继续解析到真实 client；因此 package
uninstall 后留下的 block 不会破坏新 shell。仅 binary 缺失被静默处理；找到
AgentDeck 后若生成或 source 失败，错误仍然可见。

wrapper 先检查 `<state-root>/project-attribution.enabled`。marker 不存在时直接
调用真实 client，不启动 AgentDeck。marker 存在时，每次 client 调用多启动一个
AgentDeck resolver 进程并执行一次只读数据库打开，然后仍完整检查该 client
是否 eligible。在实测的 Intel macOS 26.6 主机上，这条路径每次增加约
0.1–0.2 秒；这是环境相关量级，不是性能保证。marker 是机器本地派生状态，不进入
portable backup；restore 后缺失是合法状态。provider switch 在 selection commit
后 best-effort 刷新 marker；刷新失败不回滚或弄失败已完成的 switch，missing/stale
状态继续由 `shell status` 和 `agentdeck doctor` 诊断，后续成功刷新才会重建。

#### Resolver 与兼容入口

`agentdeck shell env <codex|claude>` 是受支持的 resolver。eligible 时 stdout
只输出最终环境变量值；route 不 eligible、状态不可读或没有项目值时 stdout 为空、
退出码为 `0`。不支持的 client 使用标准参数错误。

`agentdeck shell-init <bash|fish|zsh>` 是隐藏但仍可调用的兼容原语，只向 stdout
输出 `codex` 和 `claude` 函数；单独运行不会安装、激活或写文件。bash/zsh 可用：

```bash
eval "$(agentdeck shell-init bash)"
```

zsh 将参数换为 `zsh`；fish 3.4 或更新版本使用：

```fish
agentdeck shell-init fish | source
```

持久配置应使用 `agentdeck shell setup`。该隐藏命令不能删除：受管 block 动态
调用它以便升级 binary 而无需重写 dotfile，生成 wrapper 与旧 block 仍调用其
resolver alias，而且 `v0.2.1-rc.1` 已通过 Homebrew RC 与手册发布手工 source
方式。隐藏的 `shell-init --project-environment <client>` 在这些消费者消失前
必须与 `shell env <client>` 保持逐字节等价。

#### 生效方式与 route advisory

有三种生效方式：

1. `agentdeck run codex -- ...` 和 `agentdeck run claude -- ...` 为其启动的
   child process 注入。Codex 使用 `HEADROOM_PROJECT` 与受管
   `env_http_headers` mapping；Claude 使用 `ANTHROPIC_CUSTOM_HEADERS`。
2. managed shell function 在每次 client 启动时按当前目录和当前 route 动态
   resolver；不 eligible 或读取失败时不注入但仍原样启动真实 client。
3. 用户可以自己维护 project-scoped settings，例如
   `.claude/settings.local.json`；AgentDeck 只提供 recipe，不创建或修改它。

```json
{
  "env": {
    "ANTHROPIC_CUSTOM_HEADERS": "X-Headroom-Project: my%20project"
  }
}
```

`my%20project` 替换为目录 basename 的 percent-encoded 值；已有 custom headers
必须保留，并用换行追加。Claude app 是否读取该 settings、何时需要重启由 Claude
决定，AgentDeck 不作保证。

`provider use` 的 attribution advisory 只写 stderr，`--quiet` 下抑制，不进入
JSON，也不改变退出码：

- 切入 eligible route 且 integration 已配置：说明归属已生效、新 shell 已持久，
  并给出当前 shell 激活命令；
- 切入 eligible route 但未配置：提示一次 `agentdeck shell setup`；
- 从 eligible route 切出且 integration 已配置：说明函数仍安装但立即停止注入；
- 其他组合不输出 attribution advisory。

`provider set-wrapper` 的提示只说明机制前提：eligible route、派生 marker 与已配置
shell integration；仅修改 wrapper metadata 不会被表述为“归属已经生效”。

协议背景见 Headroom 的
[project attribution issue](https://github.com/headroomlabs-ai/headroom/issues/802)
与
[v0.27.0 release note](https://github.com/headroomlabs-ai/headroom/releases/tag/v0.27.0)。
这些第三方链接只出现在本手册；命令 advisory 只链接 AgentDeck 自己的文档。
### 客户端切换提示（stderr）

切换成功后，除 effective route 外还会在 stderr 打印客户端对应的提示行，前缀
`advisory:`。

- **Codex 生效提示（每次 Codex 切换都有）**：AgentDeck 只修改磁盘上的 Codex
  配置文件，无法更新运行中 client 已经加载的配置。提示建议新建 Codex 会话或重启
  正在运行的会话，以确保本次切换生效。它不声称 Codex 会实时重载 provider 配置，
  也不套用下述 Claude 特有的实时读取或冲突凭据语义。

Claude 切换还会打印：

- **重启提示（每次 Claude 切换都有）**：运行中的 Claude client 会实时读取
  `~/.claude/settings.json`，因此切换会在不重启的情况下影响正在进行的会话，并可能
  重置该会话已协商的能力。提示建议重启正在运行的 Claude 会话。
- **冲突凭据源提示（仅在切换到 `official` 时）**：`env.ANTHROPIC_API_KEY` 与
  `apiKeyHelper` 是 AgentDeck **不拥有**的字段，但 Claude 会优先采用它们，从而覆盖
  `official` 选择。存在其中任意一个时，AgentDeck 照常完成切换，并报告是哪个字段
  造成冲突——只报告字段名和文件路径，**永远不打印字段值**，也**绝不删除**这些字段
  来让自己的选择"获胜"。哪些取值才算"配置了凭据"见下方检测边界。

所有提示与 effective route 行遵循同一套规则：只走 stderr，不进入 stdout 的 JSON
envelope，不改变 exit code，`--quiet` 下不输出。Claude 设置文件读不到或不是合法
JSON 时，只丢掉冲突提示，不会让已经成功的切换失败。

**检测边界（没有提示 ≠ 没有冲突）**：

- **只检查 AgentDeck 管理的那一个 Claude settings 文件**——`~/.claude/settings.json`，
  或 `--config-path` 指定的文件——里的 `env.ANTHROPIC_API_KEY` 与 `apiKeyHelper`
  两个字段。**其它来源一概不在检测范围**，包括：shell 导出的环境变量
  （`export ANTHROPIC_API_KEY=...` 同样会被 Claude 采用并覆盖 `official` 选择）；
  以及 Claude 可能读取的其它 settings 作用域（具体清单以 Claude 官方文档为准）。
  AgentDeck 只写、也只读它管理的这一个文件。
- 冲突提示**只针对切换到 `official`**。切换到 custom provider 时，即使
  `env.ANTHROPIC_API_KEY` 与 AgentDeck 写入的 `env.ANTHROPIC_AUTH_TOKEN` 同时存在
  也不提示；两者会以不同的 header 一起发出，最终哪个生效取决于上游服务。
- 只有**非空字符串**才算配置了凭据。`null`、空字符串、布尔、数字、对象、数组都不
  提示：这两个字段对 Claude 都是字符串语义，其他形态要么为空、要么是错误配置，
  取不出凭据。纯空白（如 `" "`）算非空，会提示——Claude 会真的拿它去认证并失败。

### `provider add` Flags

| Flag | 含义 | 是否必填 | 默认值或推断 | 示例 |
| --- | --- | --- | --- | --- |
| `--endpoint <url>` | 当前 credential 的 base endpoint | 是 | Codex-bound 输入末尾可带 `/v1`，存储时自动去除；Claude-only 输入保留末尾 `/v1` | `--endpoint https://api.example.com/v1` |
| `--clients <list>` | 当前 credential 绑定的 clients，逗号分隔 | 是 | 无 | `--clients codex,claude` |
| `--multiplier <decimal>` | 当前 credential 的成本倍率，必须为非负有限十进制 | 否 | `1` | `--multiplier 0.8` |
| `--credential <shorthand>` | Credential shorthand；不是 reference | 否 | `default`；完整 reference 始终生成 `<provider>-<credential>-ref` | `--credential work` |

同一 provider 下添加不同 credential：

```bash
agentdeck provider add sssaicode \
  --credential claude \
  --endpoint https://claude.example/v1 \
  --clients claude

agentdeck provider add sssaicode \
  --credential codex \
  --endpoint https://codex.example/v1 \
  --clients codex \
  --multiplier 0.4
```

第一条命令创建 provider 和 `claude` credential，完整 reference 为
`sssaicode-claude-ref`。第二条命令自动识别 provider 已存在，仅新增 `codex` credential，
完整 reference 为 `sssaicode-codex-ref`。每条命令只为待新增 credential 提示一次 token。
若同名 credential 的 metadata 和 ciphertext 已完全存在，命令不提示 token 并成功返回；若
metadata 不同，提示使用 `credential update`；若 ciphertext 缺失，提示使用
`credential update --rotate`。

Endpoint 根据 credential bindings 规范化：只要 credential 绑定 Codex，输入末尾的
`/v1` 就会从存储值中自动去除，写入 Codex config 时再精确追加一次；Claude-only
credential 的末尾 `/v1` 保留。因此 Codex 用户可传 Claude 风格 base endpoint，也可传
已经以 `/v1` 结尾的地址，最终都不会产生 `/v1/v1`。Endpoint 必须包含 scheme 和
host；带 userinfo、query string 或 fragment 的地址会被拒绝，避免把 token 等敏感参数
混入 metadata 或生成语义不明确的客户端地址。

这里不存在两套 credential 来源：

- `provider add` 既可创建 provider 和首个 credential，也可在 provider 已存在时新增
  `work`、`personal` 等 credential。
- `credential add` 只能引用已经存在的 provider；它提供显式的 credential 资源入口。
- 两条命令调用同一个 short-name/reference 规范化、冲突检查、无回显输入和加密写入实现。
- Provider、credential metadata 与 AES-256-GCM ciphertext 在同一个 SQLite transaction
  中一起提交或回滚；不存在外部 secret compensation。

### `provider use` 推断规则

1. `official` 支持 `--client codex` 与 `--client claude`，不接受 `--credential`；省略
   `--client` 时默认 Codex。Codex 直连切换时设置
   `[model_providers.custom].name = "official"`，并移除 `base_url` 与
   `experimental_bearer_token`。缺少 custom table 或 `name` 时自动补齐，其他 TOML
   字段、注释和顺序保持不变。Claude 直连切换移除 `env.ANTHROPIC_BASE_URL` 与
   `env.ANTHROPIC_AUTH_TOKEN`，保留 `env` 对象和其余所有字段。两个 client 走 `--via`
   时写入 wrapper URL，仍然不写 credential。
2. Custom provider 只支持一个 client 时省略 `--client`；支持多个时必须指定。
3. 指定 client 只有一个适用 credential 时省略 `--credential`。
4. 同一 provider/client 有多个 credential 时必须填写短名称。
5. `--config-path` 只用于非标准安装；标准路径和受管备份自动解析。

```bash
agentdeck provider use official
agentdeck provider use aigocode --client codex
agentdeck provider use aigocode --client codex --credential work
```

## Credential

Credential shorthand 在一个 provider 内唯一。完整 reference 始终是
`<provider>-<credential>-ref`，包括默认 shorthand `default`，且不包含 client。用户通过
`<provider>` 和 `--credential` 操作；不存在 `--name` 或可传入 reference 的 flag。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `credential list [provider]` | 列出 credential metadata、client bindings 和 readiness；只检查 secret row，不返回或解密 value | `provider`：可选；`--client <client>`：可选过滤 | 无 | `agentdeck credential list aigocode --client codex` |
| `credential show <provider>` | 显示一个命名 credential 的完整 reference、endpoint、multiplier、bindings 和 readiness，不返回 value | `--credential <shorthand>` | `provider` 必填；shorthand 默认 `default` | `agentdeck credential show aigocode --credential work` |
| `credential add <provider>` | 为已存在 provider 新增命名 credential，读取一次 token，可同时绑定多个 clients | `--credential <shorthand>`；`--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>` | `provider`、`--endpoint`、`--clients` 必填；shorthand 默认 `default`，multiplier 默认 `1` | `agentdeck credential add aigocode --credential work --endpoint https://api.example.com --clients codex,claude` |
| `credential update <provider>` | 更新 endpoint、multiplier、client bindings，并可原子轮换 token | `--credential <shorthand>`；`--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>`；`--rotate` | `provider` 必填；shorthand 默认 `default`；四个更新 flag 至少一个 | `agentdeck credential update aigocode --credential work --multiplier 0.8 --rotate` |
| `credential remove <provider>` | 在一个 SQLite transaction 中删除命名 credential metadata 与 ciphertext，不删除 provider definition 或客户端配置 | `--credential <shorthand>` | `provider` 必填；shorthand 默认 `default` | `agentdeck credential remove aigocode --credential backup` |

`credential update --rotate`、`credential remove` 与 provider 删除都依赖 SQLite
transaction 原子性，不创建 provider-removal operation 或 recovery secret。即使 provider
已用于历史 usage，也允许删除 live definition、credential metadata 与 ciphertext；历史
归因继续使用不可变 selection snapshot。

### 多 Credential 用例

同一个 provider/client 使用多个账号：

```bash
agentdeck credential add aigocode --credential work --endpoint https://api.example.com --clients codex,claude
agentdeck credential add aigocode --credential personal --endpoint https://api.example.com/v1 --clients codex
agentdeck provider use aigocode --client codex --credential work
agentdeck provider use aigocode --client codex --credential personal
```

生成的 reference 示例：

```text
default  -> aigocode-default-ref
work     -> aigocode-work-ref
personal -> aigocode-personal-ref
```

如果 `work` 同时绑定 Codex 和 Claude，token 只加密存储一次，两端复用同一 ciphertext。

## Usage

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `usage scan` | 增量扫描本地 Codex/Claude usage sources | 无 | 无 | `agentdeck usage scan` |
| `usage summary [daily\|weekly\|monthly]` | 默认扫描后汇总全部历史，或按本机时区快捷查看今天、本周（周一开始）、本月 | 可选周期位置参数、`--no-scan` | 否；`--no-scan` 直接使用已存聚合 | `agentdeck usage summary weekly --no-scan` |
| `usage stats` | 默认扫描后输出 KPI、趋势、封顶模型列表、cache hit、client/provider 占比、均值、峰值、计价覆盖和 activity；`--interactive` 显式打开只读 TTY viewer；`--activity` 的 `MODEL ACTIVITY` range 在 text 中使用本机时区并在值后标出 | `--period`、`--from/--to`、`--group-by`、`--metric`、`--client`、`--model`、`--provider`、`--activity`、`--no-scan`、`--top`、`--interactive` | 默认 `7d/auto/tokens`；日期必须成对；`--provider` 接受精确 runtime provider 名称且不做枚举校验；`--activity` 必须与 `--model` 同用；`--no-scan` 直接使用已存聚合；`--top` 必须为非负整数，不传使用各区块默认封顶，`0` 还原未封顶完整文本列表，正整数 N 统一覆盖该次输出的封顶值；`--interactive` 仅允许 text、TTY stdin/stdout、非 dumb TERM 且至少 48x10 | `agentdeck usage stats --provider official --no-scan` |
| `usage sessions` | 按 session 分列展示各类 token、成本和计价状态；使用共享响应式列原语，窄终端将次要 token 放到 continuation 行；`FIRST`/`LAST` 在 text 中使用本机时区并在列名标出 | 无 | 无 | `agentdeck usage sessions` |
| `usage diagnose` | 展示 source、event、session、run、价格覆盖和 attribution 诊断 | 无 | 无 | `agentdeck usage diagnose` |
| `usage rebuild` | 逐 source 原子重建 usage metadata；失败 source 保留旧数据并返回 partial warning | 无 | 无 | `agentdeck usage rebuild` |

`usage scan` 目标计数：

| 字段 | 含义 |
| --- | --- |
| `files` | 本次扫描的 source files |
| `imported` | 首次插入的唯一 usage events |
| `updated` | 已存在 event snapshot 被更新的数量 |
| `ignored_non_usage` | 合法且正常、但本来就不是 usage 的消息、工具和元数据事件 |
| `unsupported_usage` | 看似 usage 记录但缺少必要 ID、model、session 或 token 字段 |
| `malformed` | 无法解析的完整 JSON 行 |
| `source_resets` | truncate、replacement、identity change 或 validated anchor mismatch 引发的 source 重扫次数；内容未变化的显式 rebuild 不计入 |

#### Usage Hook 生命周期

`agentdeck usage hook setup|status|remove [--client codex|claude|all]` 采用与
`agentdeck shell setup|status|remove` 相同的生命周期语义：`setup` 只合并
AgentDeck-owned command-hook entries、幂等；`status` 报告 `absent`、`configured`、
`modified` 或 `invalid`；`remove` 只移除可验证为 AgentDeck-owned 的 entries，绝不覆盖
已编辑或不可验证的受管区域，也不会因安装软件包而自动执行。实现不同之处是 Hook 合并
JSON（Codex `hooks.json`、Claude `settings.json`），shell 管理文本 block，因此不共享
实现代码。Codex 新 Hook 可能仍需用户在 `/hooks` 中信任；AgentDeck 只报告此限制，不会
修改 trust state 或绕过它。

安装的 handler 调用隐藏的 `agentdeck usage hook event <codex|claude>`。它只接受已知、
有界的生命周期事件，stdout 保持为空且失败不会阻止 client 启动、恢复、设置更新或退出。
Hook 缺失、未信任、禁用或失败时，usage attribution 回退到既有的 estimated
session-start 行为。`agentdeck run` 仍是兼容的低层 exact-attribution launcher，不再是
resume 的主要归属机制；并发 managed runs 不阻止 client，而将受影响的 open runs 降级为
`estimated`。

### Usage 默认输出

- 不指定 `--format` 时输出 text；usage 报告使用共享的宽度感知 section、bar、对齐列
  和响应式行原语，只有显式 `--format json` 才输出稳定 JSON envelope。
- `usage summary` 以稀疏 Emoji 标题区分总览、token totals 和 model coverage。
  `catalog_base_cost`、`provider_cost` 仍只在所有 event 都能完整计价时提供；存在
  unknown model 或缺失价格组件时保持 unavailable，同时通过明确标注的
  `known_catalog_base_cost`、`known_provider_cost`、priced/unpriced event 数和逐 model
  coverage 展示可验证的已知小计，不能把已计价工作隐藏掉。
- `usage summary` 和 `usage stats` 默认在输出前同步扫描；`--no-scan` 跳过该扫描并立即使用已存聚合，不会将扫描移到后台。
- `usage sessions` 将 input、cached input、output、cache read、cache creation、5m
  write 和 1h write token 分成独立列。无法形成完整成本时，已知小计显示为
  `(partial)`，status 列列出 warning 和 unpriced component。
- Claude `cache_creation_tokens > 0` 且两项 TTL bucket 都为零时，保留原始 token
  字段并按公开的默认 5 分钟 cache-write rate 计价；text/JSON warning 明示
  `defaulted 5m cache creation TTL`。只有非零 bucket 未覆盖总额的矛盾形状仍将余数列为
  `missing_components`，不会按模型名或拼写猜测 TTL。
- Claude model 只在两侧都为 `claude-` 名称时把点号与连字符视为等价版本标点；
  Codex 名称和其他不相等的 model 不做模糊匹配。
- `usage stats --period` 支持 `today|7d|30d|week|month|6m|all`；`week` 仍表示本机
  时区从本周一 00:00 到当前时刻的当前自然周，与滚动 `7d` 明确区分。也可用本机日期
  `--from YYYY-MM-DD --to YYYY-MM-DD`（to 当日包含在范围内）。`group-by` 支持
  `auto|hour|day|week|month`，`metric` 支持 `tokens|cost|sessions`。JSON 稳定包含
  `range`、`timezone`、`totals`、`buckets`、`models`、`clients`、`providers`、`cache_sessions`、`activity`、`peak`、
  `coverage` 和确定排序的 `unpriced_models`。totals、bucket、model、client 均保留
  input、output、cached-read、cache-write 分量。`providers` 按 client + runtime provider
  分组：exact event 使用 run snapshot，estimated event 使用 session-start provider timeline，
  无法归属的 event 进入明确的 `unknown` bucket；同名 provider 不跨 Codex/Claude 合并，
  `--provider unknown` 只选择该 bucket。provider filter 在内存派生后收窄整个报告；工具
  activity 没有 run binding，因此过滤时仅按 session-start snapshot 做 session 级近似。
  **路由是上报元数据，不是分组键**：经 wrapper 的 event 与直连 event 同属一个
  provider 行，`--provider <name>` 两者都选中，经代理的订阅流量仍记在 `official`
  且 multiplier 仍为 `1`。provider 行附加可选 JSON 字段 `wrapper_events`（该 provider
  下经 wrapper 的 event 数），没有 wrapper 时该字段整个不出现，text 的 `PROVIDERS`
  行也不加任何后缀；有 wrapper 时 text 在该行明细后追加 `N via wrapper`。路由与
  provider 始终取自同一时刻：estimated event 用 session-start snapshot（provider 本来
  就来自它），exact run-bound event 用 **run 起始时刻**的 snapshot——run 记录了
  provider 名却没记录路由，按 run 起始取快照才能保证会话中途换路由时不会报错方向。
  若该时刻的 snapshot 指向另一个 provider（run 跨越了一次 provider 切换），该 event
  的路由不上报，宁可少报也不把路由算到别的 provider 头上。
  默认 text 使用响应式 Balanced 报告：compact KPI、以 100% 为固定基线的
  `MODELS`/`CLIENTS`/`PROVIDERS` share bars，以及以本序列 peak 为基线的 `TREND`
  magnitude bars。行内 share 只显示一次；tokens、cost、pricing status 和 sessions
  在对齐列中纵向可比，溢出字段进入 continuation 行；`CLIENTS` 与其它维度保持相同
  的详情深度。
  `MODELS`、`PROVIDERS`、`UNPRICED MODELS`、按 model 的 cache 明细和 cache session 文本
  各自按现有排序（模型/provider 按占比降序，其余保持原顺序）只显示前若干行——默认
  上限分别为 8、8、12、8、10——并在被截断时追加 `+K more <区块> in JSON` 提示；JSON
  始终包含全部行。`CLIENTS` 天然很小，不封顶。`--top N` 覆盖这五个区块当次输出的上限：
  不传保留各自默认值，`--top 0` 还原未截断的完整文本列表（等同 JSON 的行数），正整数
  `--top N` 把五个上限统一改成 N；无论 `--top` 取值如何，`--format json` 都不受影响。
  `TREND` 是时间序列，遵循独立的“最近连续窗口”规则（默认最多 48 个 bucket，超出时
  只保留最新的连续窗口并追加 `+K earlier buckets in JSON`），不参与排名式截断，也不受
  `--top` 影响。`CACHE HIT RATE` 使用结构化 model rows 和有上限的 subordinate session
  list；完整 session ID 保留在受控的 grouped detail commands 中。宽终端在两侧内容都足够
  时使用双栏，否则按内容量堆叠；窄终端保留 identity/primary value 并将次要字段下移。
  至少覆盖 7 个自然日且不是 hour buckets 时，底部显示全宽 7x24 Activity Heatmap。
  真实 TTY 可使用克制颜色，`--no-color`、重定向和机器格式不输出 ANSI 控制码。
- `AVG COST`、`PEAK` 和 `PRICED` 与其它 KPI 同处 header 区域；`AVG COST / SESSION`
  明确其平均基准。无数据、单 bucket、全 100% share、全未计价和 unavailable cost
  均渲染显式状态，不以空屏或 `$0.00` 代替未知。
- `usage stats --interactive` 只在显式传入时启用，要求 text、TTY stdin/stdout、可用
  TERM 且至少 48x10；资格检查在打开 store、扫描和 raw mode 之前完成。viewer 是只读
  的，提供 Overview、Trend、Models、Clients、Providers、Cache、Coverage 七个 section，
  每个 section 独立保留 page/selection/viewport，20 行分页。左右/Tab 切换 section，
  上下/Home/End 选择，PageUp/PageDown 翻页，`?` 切换帮助，`q` 或独立 Escape 退出；
  Ctrl-C、EOF、取消、resize 和错误都恢复终端。`--top` 先于 viewer 分页生效，不能与
  `--format json` 同用；不合格终端应改用普通 `usage stats`。
- Stats 的 `timezone` 是稳定的 IANA zone 名称；无法解析本机 zoneinfo 名称时使用
  `UTC+HH:MM` offset 标识。Hour buckets 使用带 offset 的 RFC3339 边界，因此 DST
  回拨时两个同名本地小时仍是两个独立 bucket。
- Codex cache hit rate 定义为 `cached_input_tokens / input_tokens`。Claude 的 logical
  input 定义为 ordinary input + cache read + cache write，cache hit rate 定义为
  `cache_read / logical_input`；cache write 只显示 token 量，不显示成第二种“命中率”。
  totals 和混合 bucket 不生成语义不一致的跨 client 单一比率。
- 未计价 model 只排除在费用计算之外，仍正常参与 token/share/session/event/cache/
  activity/tool 汇总。`usage stats --model <model> --activity` 展示 active session/day、
  时间范围、工具总数、完成/失败、可用耗时和按安全工具名的分布；usage DB 只保存工具名、
  时间、状态、耗时及来源所有权，不保存参数、结果、命令文本、环境或 reasoning。
- `metric=cost` 只在对应范围全部计价时提供 `metric_value`、`share`、peak `value`
  和 `average_cost_per_session`。混合 priced/unpriced 数据将这些完整值设为 `null`，
  另由 `known_metric_value`、`known_share`、`known_value` 和
  `known_average_cost_per_session` 返回已知小计；Stats text 仅在相邻费用字段标记
  `KNOWN`，并以紧凑 `UNPRICED MODELS` 区块列出 model 与缺失组件，不再输出通用
  partial 脚注。完全无可用金额时显示 `unavailable`，JSON 用 `null` 保留未知状态，
  不以 `$0.00` 冒充完整费用。
- Schema v10 将已有和新写入的 `usage_events.event_at` 统一为 UTC RFC3339Nano，
  并从规范化事件重算 session `first_at/last_at`。SummaryRange、Stats、最早事件和
  session 边界均按绝对时间工作，不对保留 offset 的原始文本做范围比较。
- Stats 对范围事件只做一次索引扫描，并分别批量加载价格层与 metadata-only provider
  timeline；run multiplier、session attribution、历史 provider snapshot 和有效价格
  在内存中一次聚合，不按 event 追加 SQL，也不读取 credential value。

### Usage Scan 性能契约

- 每次成功的 standalone `usage scan` 都保存与扫描结果同一时点的 usage source
  inventory/checkpoint，供后续 scan 和 watch 共用。
- Inventory 明确输出 `added`、`appended`、`mutated` 和 `removed` paths；scanner 只读取
  这些 changed paths。Standalone scan 处理与 checkpoint 必须使用同一次稳定 inventory。
- Unchanged file 只比较 metadata/checkpoint，不能打开或读取文件内容，也不能写数据库。
- Append-only file 从持久化 cursor 附近的校验 anchor 开始，只读取必要 overlap 与新增
  suffix；不能重新读取、hash 或逐行跳过完整历史前缀。
- 如果活跃日志在 inventory 之后继续纯追加，scanner 会验证已读取快照范围和 cursor
  anchor 未变化，提交该稳定前缀，并让后续 scan 从旧 checkpoint 补齐新增 suffix；
  不把正常追加报告为 source mutation。
- Scanner 在提交前重新读取并比较本次 bounded snapshot bytes 和 cursor anchor；即使
  size、mtime 与 inventory 相同，扫描期间发生的 in-place rewrite 也不能通过仅比较
  metadata 被接受。该验证仍只读取 anchor 与本次 suffix，不回读完整历史前缀。
- Truncate、replacement、identity change 或 anchor mismatch 只重建受影响 source，不能
  让一个文件的变化触发全部 usage sources 全量重扫。
- Source reset/replacement 在单个 source transaction 内删除旧 events/run bindings、写入
  新 events 并重建 session aggregation。Removed source 同样清理 source state、events、
  run-source metadata 和 session aggregation；失败时不得留下 partial rebuild。
- Scanner 保留 partial-line、stable event key、source mutation 和 exact run byte-range
  契约。性能优化不能通过跳过必要 mutation detection 获得。
- `usage rebuild` 是显式全量验证/重建入口；它不再全局先删后扫，而是逐 source 在单个
  transaction 内替换旧 metadata。单个 source 不稳定或重建失败时保留该 source 的旧
  events、cursor、event-to-run binding、run-source byte-range metadata 和 session
  aggregation。相同 stable event key 出现在多个 source 时，canonical path 的确定性
  优先级决定 event owner；rebuild 按同一优先级处理，低优先级 source 不得跨 transaction
  改写尚未成功重建的 owner。命令以 `partial: true` 返回
  `usage_source_unstable` 或 `usage_source_rebuild_failed` warning，并且不推进 watch
  checkpoint。`--quiet` 仍必须显示该 partial warning，只能静默没有 warning 的完整
  text 成功。普通 `usage scan` 和 watch 使用增量路径。
- 增加可注入 file reader/inventory 和性能回归测试，证明 unchanged scan 读取 0 个内容
  bytes、append scan 的读取量与新增 suffix 近似线性，而不是与历史文件总大小成正比。

## Price

价格命令从旧的 `usage price ...` 提升为顶层 `price ...`，不保留旧入口。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `price status` | 查看 active price catalogs、覆盖和 provenance；不联网 | 无 | 无 | `agentdeck price status` |
| `price history` | 查看 immutable catalog 历史 | 无 | 无 | `agentdeck price history` |
| `price list [model]` | 查看当前组件级合并后的有效费率，单位 USD / 1M tokens | `model`：可选精确过滤；`--provider openai\|anthropic` | 无 | `agentdeck price list gpt-5.6-sol` |
| `price update` | 自动解析并下载最新 LiteLLM canonical raw catalog | `--commit <40-char-sha>` | 无；`--commit` 为可选复现入口 | `agentdeck price update` |
| `price override` | 导入本地 official component override | `--file <json>` | `--file` 必填 | `agentdeck price override --file prices.json` |

`price history` 的 `EFFECTIVE`、`price status` 中嵌入的 catalog 历史，以及
`price list --verbose` provenance 的 `EFFECTIVE` 在 text 中使用本机时区并在列名标出；
对应 JSON 始终保留 UTC RFC 3339。

默认执行 `price update` 时，AgentDeck 先通过 GitHub API 解析 LiteLLM `main`
的最新 commit，再从该 commit 对应的 canonical raw URL 下载并记录 provenance。
指定 `--commit` 会跳过最新版本解析，用于复现或回滚。命令不接受 `--url`；实际下载
URL 始终由已验证的 commit 唯一推导，避免 URL、commit 与内容不一致。显式非法
commit 会在访问状态目录或初始化 HTTP client 前以 `invalid_argument` 拒绝。生产
HTTP client 的总超时为 60 秒；失败不会写入 catalog。`content_sha256` 始终表示下载
原文的 SHA-256，并在 update、status 和 history 中保持一致。Commit discovery 和
pinned raw catalog 下载对 transient transport/read error、HTTP 408/429/5xx 以及可识别的
truncated JSON 最多尝试三次；非 retryable 错误立即返回，只有完整 catalog 通过解析与
校验后才会导入状态。

`price status` 以当前绝对时间同时确定 top-level provenance、active catalogs、
`available`、model 和 component 数量；future-only catalog 返回 unavailable，当前与
future 并存时所有这些字段只描述当前有效集合。合法 RFC3339 offset 会先解析为绝对
时间，不参与字符串范围排序。

事件成本优先使用事件时间有效的历史费率；历史模型或单个组件缺失时仅从当前有效
费率补缺，绝不覆盖已可计算的历史组件，也不增加 fallback/estimate 标记。历史和
当前本地目录都不存在的模型继续 unpriced。默认 text 的 status/history/list/update/
override 使用可读表格并隐藏长 URL、完整 commit/SHA；JSON 和 `--verbose` 保留完整
provenance。

## Session

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `session --interactive` | 以只读 TTY browser 浏览已索引 sessions，并进入分区详情；不会隐式执行 scan | `--interactive` | 不接受位置参数；要求 text 输出以及 TTY stdin/stdout | `agentdeck session --interactive` |
| `session scan` | 增量建立可清除的 session 搜索索引；启用时在 stderr 报告聚合进度 | 无 | 无 | `agentdeck session scan` |
| `session list` | 列出索引中的 sessions | `--client codex\|claude`；`--page`、`--limit`、`--all` | 无 | `agentdeck session list --client codex --page 2` |
| `session search <query>` | 搜索 approved visible session text | `query`：搜索文本；`--client`：可选过滤；`--page`、`--limit`、`--all` | `query` 必填 | `agentdeck session search "provider timeout" --client codex --page 2` |
| `session show <session-id>` | 分区显示一个 session；可按需读取安全 activity/tool 元数据或 normalized invocation usage | `--client codex\|claude`、`--activity`、`--tokens`、`--interactive`、`--page`、`--limit`、`--all` | `session-id` 必填；跨 client 冲突时 `--client` 条件必填 | `agentdeck session show 019abc --client codex --tokens --page 2` |
| `session exclude` | 持久化索引排除规则 | `--kind project\|path\|session\|client`；`--value <value>` | 两个 flags 均必填 | `agentdeck session exclude --kind client --value claude` |
| `session rebuild` | 重建 purgeable index，不删除源日志 | 无 | 无 | `agentdeck session rebuild` |
| `session purge-index` | 删除 sessions.sqlite3，不删除源日志或 core DB | 无 | 无 | `agentdeck session purge-index` |

`session purge-index` 仅清除 session watch checkpoint，不影响 usage/extension checkpoint；
下次 session watch 会 bootstrap 重建索引。`session show` 在同一 ID 同时存在于 Codex 和
Claude 时返回歧义错误并要求 `--client`。Session 与 credential 的 `--client` 都只接受
`codex|claude`。
`session show --activity` 只在调用时读取所选 source，显示工具名、时间、状态和可用耗时；
这些数据不写入 `sessions.sqlite3`，参数、结果、命令文本、环境和 reasoning 始终不显示。Text 默认每页 20 条并显示总数与可复制的下一页命令（保留 `--state-dir`、`--client`、`--activity` 与 limit）；`--limit` 必须为 1 至 1000。JSON 仅在显式分页时加入确定的 `pagination`，否则保持完整集合。`--activity` 始终先输出完整 session 的调用、状态、时长与按工具汇总，再分页显示安全明细。
普通 text 的 Documents、Activity 与 Invocations 记录时间会在可换行的值中明确本机显示时区；
空值或无法解析的时间不会虚构时区，JSON 继续保留 UTC RFC 3339。

`session search` 的每条 approved document 都返回该记录自身已归一化为 UTC 的
`event_at`。可解析时间按时间倒序排列，缺少或无法解析的时间稳定排在其后；text
输出使用本地时区显示时间，`event_at` 为空时显示 `—`。显式使用分页 flags 时，JSON
输出在 `documents` 下返回该页内容，并在 `pagination.search` 中返回分页元数据。

`session scan` 和 `session rebuild` 的进度始终写入 stderr，因此 JSON stdout
保持可机读。启用时只报告 source file processed/total、approved document 和
skipped 的聚合计数；不输出 source path 或 source content，`--quiet` 会抑制它。

`session show` 始终显示 SESSION metadata 与 DOCUMENTS。`--activity` 增加既有
安全 activity aggregate/detail section。`--tokens` 增加完整 normalized usage
summary 和按时间排序的 invocation rows；显式分页时 JSON 返回
`pagination.invocations`。token 保持整数，price 保持 decimal string 或
unavailable。pricing/attribution warning 位于 `data.usage.warnings`，每条
invocation 的 warning 位于 `data.invocations[].warnings`；顶层 envelope 的
`warnings` 与 `partial` 只表示 command-level state，不能替代这两组 usage warning。

普通 `session show` text 使用有界的 section/record/labeled-continuation 画布，不再按
terminal 宽度切换为 ASCII table。`COLUMNS` 为正整数时决定 redirected text 宽度，
否则使用 100；TTY text 先读取 terminal width，并仍允许 `COLUMNS` 显式覆盖。每一行
都不超过该可见宽度。DOCUMENTS 展开当前有界 page 的 approved visible text；ACTIVITY
按 summary 与 safe call metadata 分层；TOKENS 先解释完整 summary，再把每个 normalized
invocation 的全部 token/cache component、catalog/provider cost、pricing status、unpriced
component 与 warning 放到续行。invocation sequence 仅表示 usage chronological position，
不宣称与 conversation turn 可靠对应。JSON envelope、字段、分页和 UTC 值不受 text 布局影响。

`session --interactive` 是根级、显式的只读索引浏览入口，不会隐式执行
`session scan`。索引为空时显示可复制的 `agentdeck session scan` 提示。列表中使用
Up/Down/Home/End 选择，PageUp/PageDown 翻页，Enter 打开现有详情 viewer；详情中的
Escape 返回列表，列表中的 Escape 退出，`q` 在任一层退出。进入根级 browser 或
`session show --interactive` 等待用户输入前都会释放 state lock。整个根级浏览流程
不显示 source path；列表显示 compact Project 标识，并将缺失 Model 明确标为 `unknown`。该入口只接受 text output 与 TTY
stdin/stdout，不属于 JSON 或 desktop DTO surface。

`session show --interactive` 打开只读 TTY viewer，其中包含 OVERVIEW、DOCUMENTS、
ACTIVITY 和 TOKENS section，以及各 section 独立的 lazy page state。它要求 text
output 和 TTY stdin/stdout，且不能与 `--activity`、`--tokens`、`--page`、`--limit`
或 `--all` 同时使用。自动化和 desktop 消费应使用普通 `--format json` command；
text 和 interactive viewer 都不是 DTO contract。

根 browser 与详情 viewer 共用亮色 semantic roles、active tab、selection、warning、status
和 no-color label contract。详情的 OVERVIEW、DOCUMENTS、ACTIVITY、TOKENS 各自保留
page、selection 与 viewport；selected document 展开有界 approved text preview 和完整页命令，
selected activity 只展开 safe metadata，selected invocation 展开全部 token/cost/pricing/warning
详情。48x10 是 interactive minimum；窄、短 frame 受控降级，宽 frame 只在 detail 确实需要
额外高度时使用 list/detail split。

这组有界的 session DTO 是 v0.4.0 的桌面依赖合同，已满足 v0.5.0 计划
`desktop-wire-contract` 的入口条件。desktop client 可消费 versioned JSON envelope
中的 session metadata/documents、optional safe activity、optional usage/invocations、
named pagination、warnings 和 partial state。后续 desktop wire contract 仍必须负责
一个 coherent snapshot、wire version 和 Go-owned redaction，而不是解析 CLI text。

## Extension

Extension ID 是稳定资源标识，继续使用位置参数。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `extension scan` | 扫描 Codex/Claude 原生 plugin、MCP 和 skill；报告 found/added/updated/removed/unchanged 和排序汇总 | 无 | 无 | `agentdeck extension scan` |
| `extension list` | 列出发现的 extensions | `--client`、`--kind`：可选过滤 | 无 | `agentdeck extension list --client codex --kind skill` |
| `extension show <id>` | 显示 extension metadata 和 diagnostics | `id`：extension ID | `id` 必填 | `agentdeck extension show codex:skill:user:sample` |
| `extension doctor` | 检查 drift、duplicate 和 missing path | 无 | 无 | `agentdeck extension doctor` |
| `extension adopt <id>` | 记录 AgentDeck 管理 metadata，不复制原生内容 | `id` | `id` 必填 | `agentdeck extension adopt codex:skill:user:sample` |
| `extension release <id>` | 释放管理 metadata，不删除原生 extension | `id` | `id` 必填 | `agentdeck extension release codex:skill:user:sample` |
| `extension enable <id>` | 请求启用；adapter 无可靠写入契约时返回 `extension_read_only` | `id` | `id` 必填 | `agentdeck extension enable codex:skill:user:sample` |
| `extension disable <id>` | 请求禁用；adapter 无可靠写入契约时返回 `extension_read_only` | `id` | `id` 必填 | `agentdeck extension disable codex:skill:user:sample` |

Skill discovery follows valid directory links for ordinary skills, `.system`
child skills, and a linked `.system` directory. Link target content changes and
target switches retain the canonical extension ID and produce deterministic
`updated`/drift results. A broken or cyclic link fails the scan atomically:
the prior inventory, managed flag, and adopted fingerprint remain unchanged.
After the link becomes valid again, scanning resumes without losing adoption.
Hidden skill directories remain excluded except for the explicit `.system`
namespace, and watch fingerprints never recursively follow link cycles.

## Backup

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `backup create [path]` | 创建加密 `.adb` backup；passphrase 不进入参数或环境变量；text 成功信息追加本机时区的 `created` | `path`：可选；`--include-sessions` | 无；path 默认受管 backup 目录 | `agentdeck backup create --include-sessions` |
| `backup list` | 列出默认 portable backup 目录；`MODIFIED` 在 text 中使用本机时区并在列名标出 | 无 | 无 | `agentdeck backup list` |
| `backup inspect <path>` | 解密、校验并显示 manifest，不恢复；`created` 在 text 中使用本机时区并在值后标出 | `path` | `path` 必填 | `agentdeck backup inspect backup.adb` |
| `backup restore <path>` | 恢复到空 state root；失败时补偿本次创建内容 | `path` | `path` 必填；目标 state root 必须为空 | `agentdeck backup restore backup.adb --state-dir /tmp/restored` |

Portable backup 只导出 `provider_credentials` 与 `credential_secrets` 当前 join 到的
credential，并只在内存和 age 加密 stream 中处理明文。`credential.key` 永不进入 archive；
restore 为目标机器创建新 key，并在一个 transaction 中替换 snapshot ciphertext。

## Doctor、Watch、Run、Version、Help 与 Completion

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `usage hook setup\|status\|remove` | 安装、检查或移除 Codex/Claude 的 AgentDeck-owned session-route Hook entries；不修改无关 hooks，Codex trust 限制由 status 明示 | `--client codex\|claude\|all` | 可选；默认 `all` | `agentdeck usage hook setup --client codex` |
| `doctor` | quick read-only diagnostics；检查 key 权限、按 sealed key version 推导的 key ID、算法/版本、nonce 和 secret ownership，不解密 | `--full`：额外认证全部 credential ciphertext，并增加 usage、session、extension 和价格深度检查 | 无 | `agentdeck doctor --full` |
| `state migrate` | 显式将本地 core state 迁移到当前 schema；doctor 永不自动迁移 | 无 | 无 | `agentdeck state migrate` |
| `watch` | 前台监控 local sources；复用各 domain 已成功 scan 的 checkpoint，不重复 bootstrap 已完成的扫描；text 行首时间使用本机时区并在值后标出，NDJSON 保持 UTC | `--interval <duration>`；`--domains <list>` | 均可选；interval 默认 `1m`，domains 默认 `usage,session,extension` | `agentdeck watch --interval 30s --domains usage --format ndjson` |
| `run <codex\|claude> [-- <args...>]` | 兼容的低层 launcher；启动客户端并建立 exact/estimated usage attribution，允许无 child args；session resume 以 Hook boundary 为主 | client：位置参数；dash 后参数可为空 | client 必填 | `agentdeck run codex --` |
| `version` | 输出 release、commit、branch、Go version 和 UTC build time | 无 | 无 | `agentdeck version --format json` |
| `help [command-path]` | 显示 root 或指定命令帮助 | command path 可选 | 无 | `agentdeck help credential update` |
| `completion <bash\|fish\|zsh>` | 只输出指定 shell completion script | shell | shell 必填；PowerShell 不支持且不出现在 help/completion 中 | `agentdeck completion zsh` |

`version` 的 `UTC Build Time` 是例外：它是构建时注入、用于跨机器比较的 immutable
release/support identity，不是运行时领域 instant，因此保持固定 UTC 格式，并在字段名中
明确标出 UTC。

Doctor 对 core schema 使用四态契约：schema 12 quick/full 报告
`schema_outdated`、`count=12` 和 `agentdeck state migrate`；完整 schema 13
报告 `ok`、`count=13`；schema 13 缺 `usage_tool_calls` 只报告
`schema_incompatible`，不再附加旧 schema warning；未来 schema 报告
`unknown_schema` 且不提供虚假 recovery。Text 和 JSON 都不输出原始 SQL、
SQLite 查询或驱动错误。`state migrate` 的 text 成功信息明确确认完成，JSON
返回 `migrated: true`。

### Watch 扫描规则

- Usage、session、extension 使用独立 domain checkpoint；成功执行相应 standalone scan
  后，watch 启动不能再次扫描该 domain 的 unchanged backlog。
- 某个 domain 没有 checkpoint 时，watch 只对该 domain 执行首次 bootstrap；例如只运行
  过 `usage scan` 时，usage 不重复扫描，但尚未建立的 session index 仍可首次扫描。
- `watch` text 以本地时间和领域语义显示完成/跳过摘要；NDJSON 保持版本化、机器可读事件。各 domain 的 changes 都表示真实 added+updated+removed 逻辑变化，extension 不使用 found 总数。Usage 以逻辑 events/records、session 以当前可见 documents 为单位；删除重复 source 且仍有副本接管时为 0，删除最后来源时为实际消失的逻辑行数，而不是 source path 数。
- Session 的可见 document 序列按 kind/text 内容做确定性差分，不按数组位置或 source path 识别：开头、中间或末尾插入/删除一条只计一个逻辑变化，单条内容替换计一个 updated，重复文本仍保持确定对齐。
- `--domains` 允许只监控需要的 domain。`--domains usage` 不得隐式运行 session 或
  extension scanner，也不得创建或打开 `sessions.sqlite3`。
- 后续 poll 先比较廉价 inventory，仅把新增、append、mutated、removed paths 交给对应
  scanner；单个 changed path 不能触发全域内容读取。
- Standalone scan 与 watch 必须共用同一个 inventory、checkpoint 和 incremental scanner
  实现，不能维护两套变化判定逻辑。

## 默认 Text 输出

- collections：统一的 `+`、`-`、`|` ASCII grid，单空格 padding、逐行 separator，并按终端显示宽度对齐。
- `provider status` collection：`CODEX ACTIVE` 与 `CLAUDE ACTIVE` 直接显示当前
  credential shorthand，未激活和内置 official credential 显示 `-`。
- detail：标签字段，不输出 Go DTO 或 JSON。
- empty：明确说明没有结果。
- mutation：说明完成的动作和资源名，不输出 credential value。
- doctor：显示 `healthy`、`degraded` 或 `unhealthy`，分别统计 warnings/errors。
- usage scan：解释 ignored、unsupported、malformed、imported、updated 和 source reset。
- `--quiet`：只抑制非必要 text 成功信息，不改变 JSON、错误或 exit code。

## JSON 与敏感信息

- 保持版本化 envelope；本轮不因 official provider 提高 `schema_version`。
- CLI 参数重构不改变 credential value 从不进入 JSON/text/log 的安全契约。
- Provider definition JSON 只显示 aggregate `clients` 和 `credential_count`，不包含
  endpoint、multiplier、reference 或嵌套 credentials；readiness 和 credential metadata
  只由 `provider status` 的复数 `credentials`、`credential ...` 或 doctor 检查。
- Usage 新分类采用 additive JSON 字段；旧 `unsupported` 可在一个过渡期作为总和保留，
  但不再用于 text 输出。
- Doctor 增加权威 `status` 和 warning/error counts；旧字段只作为 JSON 兼容字段，
  text 不再依赖单一 boolean。

## 实现与评审边界

- 直接删除旧的 `provider edit`、`provider credential ...`、`usage price ...` 和旧位置
  参数语法；不创建 aliases 或隐藏兼容命令。
- Multiple credentials 使用 provider/credential/client binding 数据模型和 active
  credential selection；在 AgentDeck state 内，credential value 持久化时只以 SQLite
  认证密文存在。
- Credential creation、reference generation、加密与 transaction 写入只有一个 service
  implementation；`provider add` 仅负责编排首次设置或已有 provider 的 credential 新增。
- Usage/watch 性能修复必须保留 source mutation 与 byte-range attribution 契约，并覆盖
  standalone scan 后启动 watch 不重复扫描的回归测试。
- `official` 同时内置于 Codex 与 Claude，不参与 credential 模型，不写 providers
  表或 credential references。
- 不读取、修改或测试真实 HOME、auth.json、credential key file 或 `.vscode/`。
- 实现后必须更新 CLI spec、Phase 9、README 双语、JSON/text golden，并运行 targeted
  tests、`git diff --check`、完整 `make release-verify`，最后清理生成产物。
