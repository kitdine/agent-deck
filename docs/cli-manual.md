# AgentDeck CLI 目标命令手册

**Status:** active

本文档定义 Phase 9 CLI usability、Codex `official` provider baseline，以及
credential-owned provider configuration 的正式命令契约。执行状态以
`docs/plans/2026-07-13-agentdeck-cli.md` 为准。

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
- 默认 text collection 使用统一的 `+`、`-`、`|` ASCII grid；显式
  `--format json` 才输出稳定 envelope。
- `official` 是 Codex 内置 provider，不存入 providers 表，不创建 credential，不访问
  credential vault 或 `auth.json`。

## 全局 Flags

| Flag | 含义 | 是否必填 | 示例 |
| --- | --- | --- | --- |
| `--format text\|json\|ndjson` | 输出格式；`ndjson` 仅允许 `watch` | 否，默认 `text` | `agentdeck provider list --format json` |
| `--state-dir <path>` | 覆盖 AgentDeck 状态根目录 | 否，默认 `~/.agentdeck` | `agentdeck doctor --state-dir /tmp/ad-state` |
| `--no-color` | 禁用终端颜色 | 否 | `agentdeck doctor --no-color` |
| `--quiet` | 抑制非必要 text 输出；错误和机器输出不受影响 | 否 | `agentdeck usage scan --quiet` |
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
| `provider show <name>` | 显示一个 provider definition；不检查 credential readiness | `name`：provider name | `name` 必填 | `agentdeck provider show official` |
| `provider status [name]` | 检查全部或指定 provider 的 client、credential readiness 和 active selection；readiness 只检查 secret row 是否存在，不解密 | `name`：可选过滤 | 否 | `agentdeck provider status aigocode` |
| `provider add <name>` | Provider 不存在时原子创建 provider 和 credential；provider 已存在时新增该 credential；相同 metadata 和 secret 已存在时无提示成功 | `--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>`；`--credential <shorthand>` | `name`、`--endpoint`、`--clients` 必填；其余可选 | `agentdeck provider add aigocode --credential codex --endpoint https://api.example.com/v1 --clients codex` |
| `provider update <name>` | 更新一个 credential 的 endpoint、multiplier 或 bindings；未指定字段保持不变，不处理 credential value | `--credential <shorthand>`；`--endpoint <url>`；`--clients <list>`；`--multiplier <decimal>` | `name` 必填；metadata flag 至少一个；credential 唯一时可省略 shorthand | `agentdeck provider update aigocode --credential codex --multiplier 1.2` |
| `provider remove <name>` | 在一个 SQLite transaction 中删除 custom provider、credential metadata 与 ciphertext | 无 | `name` 必填 | `agentdeck provider remove aigocode` |
| `provider use <name>` | 切换 client 到 provider；client 或 credential 唯一时自动推断 | `--client codex\|claude`；`--credential <short-name>`；`--config-path <path>` | `name` 必填；client/credential 仅在无法唯一推断时必填 | `agentdeck provider use aigocode --client codex --credential work` |
| `provider recover` | 检查中断的 `provider use` operations；credential/provider 删除不需要外部 recovery | 无 | 无 | `agentdeck provider recover` |

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

1. `official` 固定为 Codex，不接受 `--credential`；切换时设置
   `[model_providers.custom].name = "official"`，并移除 `base_url` 与
   `experimental_bearer_token`。缺少 custom table 或 `name` 时自动补齐，其他 TOML
   字段、注释和顺序保持不变。
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
| `usage summary` | 扫描后汇总 token、catalog base cost 和 provider cost | 无 | 无 | `agentdeck usage summary` |
| `usage sessions` | 按 session 展示 usage 和成本 | 无 | 无 | `agentdeck usage sessions` |
| `usage diagnose` | 展示 source、event、session、run、价格覆盖和 attribution 诊断 | 无 | 无 | `agentdeck usage diagnose` |
| `usage rebuild` | 删除可重建 usage metadata 后全量重扫；不删除源日志、provider 或 credential | 无 | 无 | `agentdeck usage rebuild` |

`usage scan` 目标计数：

| 字段 | 含义 |
| --- | --- |
| `files` | 本次扫描的 source files |
| `imported` | 首次插入的唯一 usage events |
| `updated` | 已存在 event snapshot 被更新的数量 |
| `ignored_non_usage` | 合法且正常、但本来就不是 usage 的消息、工具和元数据事件 |
| `unsupported_usage` | 看似 usage 记录但缺少必要 ID、model、session 或 token 字段 |
| `malformed` | 无法解析的完整 JSON 行 |
| `source_resets` | truncate、replacement 或 identity change 引发的 source 重扫次数 |

### Usage Scan 性能契约

- 每次成功的 standalone `usage scan` 都保存与扫描结果同一时点的 usage source
  inventory/checkpoint，供后续 scan 和 watch 共用。
- Inventory 明确输出 `added`、`appended`、`mutated` 和 `removed` paths；scanner 只读取
  这些 changed paths。Standalone scan 处理与 checkpoint 必须使用同一次稳定 inventory。
- Unchanged file 只比较 metadata/checkpoint，不能打开或读取文件内容，也不能写数据库。
- Append-only file 从持久化 cursor 附近的校验 anchor 开始，只读取必要 overlap 与新增
  suffix；不能重新读取、hash 或逐行跳过完整历史前缀。
- Truncate、replacement、identity change 或 anchor mismatch 只重建受影响 source，不能
  让一个文件的变化触发全部 usage sources 全量重扫。
- Source reset/replacement 在单个 source transaction 内删除旧 events/run bindings、写入
  新 events 并重建 session aggregation。Removed source 同样清理 source state、events、
  run-source metadata 和 session aggregation；失败时不得留下 partial rebuild。
- Scanner 保留 partial-line、stable event key、source mutation 和 exact run byte-range
  契约。性能优化不能通过跳过必要 mutation detection 获得。
- `usage rebuild` 是显式全量验证/重建入口；普通 `usage scan` 和 watch 使用增量路径。
- 增加可注入 file reader/inventory 和性能回归测试，证明 unchanged scan 读取 0 个内容
  bytes、append scan 的读取量与新增 suffix 近似线性，而不是与历史文件总大小成正比。

## Price

价格命令从旧的 `usage price ...` 提升为顶层 `price ...`，不保留旧入口。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `price status` | 查看 active price catalogs、覆盖和 provenance；不联网 | 无 | 无 | `agentdeck price status` |
| `price history` | 查看 immutable catalog 历史 | 无 | 无 | `agentdeck price history` |
| `price update` | 自动解析并下载最新 LiteLLM canonical raw catalog | `--commit <40-char-sha>` | 无；`--commit` 为可选复现入口 | `agentdeck price update` |
| `price override` | 导入本地 official component override | `--file <json>` | `--file` 必填 | `agentdeck price override --file prices.json` |

默认执行 `price update` 时，AgentDeck 先通过 GitHub API 解析 LiteLLM `main`
的最新 commit，再从该 commit 对应的 canonical raw URL 下载并记录 provenance。
指定 `--commit` 会跳过最新版本解析，用于复现或回滚。命令不接受 `--url`；实际下载
URL 始终由已验证的 commit 唯一推导，避免 URL、commit 与内容不一致。显式非法
commit 会在访问状态目录或初始化 HTTP client 前以 `invalid_argument` 拒绝。生产
HTTP client 的总超时为 60 秒；失败不会写入 catalog。`content_sha256` 始终表示下载
原文的 SHA-256，并在 update、status 和 history 中保持一致。

## Session

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `session scan` | 增量建立可清除的 session 搜索索引 | 无 | 无 | `agentdeck session scan` |
| `session list` | 列出索引中的 sessions | `--client codex\|claude`：可选过滤 | 无 | `agentdeck session list --client codex` |
| `session search <query>` | 搜索 approved visible session text | `query`：搜索文本；`--client`：可选过滤 | `query` 必填 | `agentdeck session search "provider timeout" --client codex` |
| `session show <session-id>` | 显示一个 session；ID 唯一时自动推断 client | `--client codex\|claude` | `session-id` 必填；跨 client 冲突时 `--client` 条件必填 | `agentdeck session show 019abc --client codex` |
| `session exclude` | 持久化索引排除规则 | `--kind project\|path\|session\|client`；`--value <value>` | 两个 flags 均必填 | `agentdeck session exclude --kind client --value claude` |
| `session rebuild` | 重建 purgeable index，不删除源日志 | 无 | 无 | `agentdeck session rebuild` |
| `session purge-index` | 删除 sessions.sqlite3，不删除源日志或 core DB | 无 | 无 | `agentdeck session purge-index` |

`session purge-index` 仅清除 session watch checkpoint，不影响 usage/extension checkpoint；
下次 session watch 会 bootstrap 重建索引。`session show` 在同一 ID 同时存在于 Codex 和
Claude 时返回歧义错误并要求 `--client`。Session 与 credential 的 `--client` 都只接受
`codex|claude`。

## Extension

Extension ID 是稳定资源标识，继续使用位置参数。

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `extension scan` | 扫描 Codex/Claude 原生 plugin、MCP 和 skill | 无 | 无 | `agentdeck extension scan` |
| `extension list` | 列出发现的 extensions | `--client`、`--kind`：可选过滤 | 无 | `agentdeck extension list --client codex --kind skill` |
| `extension show <id>` | 显示 extension metadata 和 diagnostics | `id`：extension ID | `id` 必填 | `agentdeck extension show codex:skill:user:sample` |
| `extension doctor` | 检查 drift、duplicate 和 missing path | 无 | 无 | `agentdeck extension doctor` |
| `extension adopt <id>` | 记录 AgentDeck 管理 metadata，不复制原生内容 | `id` | `id` 必填 | `agentdeck extension adopt codex:skill:user:sample` |
| `extension release <id>` | 释放管理 metadata，不删除原生 extension | `id` | `id` 必填 | `agentdeck extension release codex:skill:user:sample` |
| `extension enable <id>` | 请求启用；adapter 无可靠写入契约时返回 `extension_read_only` | `id` | `id` 必填 | `agentdeck extension enable codex:skill:user:sample` |
| `extension disable <id>` | 请求禁用；adapter 无可靠写入契约时返回 `extension_read_only` | `id` | `id` 必填 | `agentdeck extension disable codex:skill:user:sample` |

## Backup

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `backup create [path]` | 创建加密 `.adb` backup；passphrase 不进入参数或环境变量 | `path`：可选；`--include-sessions` | 无；path 默认受管 backup 目录 | `agentdeck backup create --include-sessions` |
| `backup list` | 列出默认 portable backup 目录 | 无 | 无 | `agentdeck backup list` |
| `backup inspect <path>` | 解密、校验并显示 manifest，不恢复 | `path` | `path` 必填 | `agentdeck backup inspect backup.adb` |
| `backup restore <path>` | 恢复到空 state root；失败时补偿本次创建内容 | `path` | `path` 必填；目标 state root 必须为空 | `agentdeck backup restore backup.adb --state-dir /tmp/restored` |

Portable backup 只导出 `provider_credentials` 与 `credential_secrets` 当前 join 到的
credential，并只在内存和 age 加密 stream 中处理明文。`credential.key` 永不进入 archive；
restore 为目标机器创建新 key，并在一个 transaction 中替换 snapshot ciphertext。

## Doctor、Watch、Run、Version、Help 与 Completion

| 命令 | 含义与典型用例 | 参数与 Flags | 必填规则 | 示例 |
| --- | --- | --- | --- | --- |
| `doctor` | quick read-only diagnostics；检查 key 权限、key ID、算法/版本、nonce 和 secret ownership，不解密 | `--full`：额外认证全部 credential ciphertext，并增加 usage、session、extension 和价格深度检查 | 无 | `agentdeck doctor --full` |
| `watch` | 前台监控 local sources；复用各 domain 已成功 scan 的 checkpoint，不重复 bootstrap 已完成的扫描 | `--interval <duration>`；`--domains <list>` | 均可选；interval 默认 `1m`，domains 默认 `usage,session,extension` | `agentdeck watch --interval 30s --domains usage --format ndjson` |
| `run <codex\|claude> [-- <args...>]` | 启动客户端并建立 exact/estimated usage attribution；允许无 child args | client：位置参数；dash 后参数可为空 | client 必填 | `agentdeck run codex --` |
| `version` | 输出 release、commit、branch、Go version 和 UTC build time | 无 | 无 | `agentdeck version --format json` |
| `help [command-path]` | 显示 root 或指定命令帮助 | command path 可选 | 无 | `agentdeck help credential update` |
| `completion <bash\|fish\|zsh>` | 只输出指定 shell completion script | shell | shell 必填；PowerShell 不支持且不出现在 help/completion 中 | `agentdeck completion zsh` |

### Watch 扫描规则

- Usage、session、extension 使用独立 domain checkpoint；成功执行相应 standalone scan
  后，watch 启动不能再次扫描该 domain 的 unchanged backlog。
- 某个 domain 没有 checkpoint 时，watch 只对该 domain 执行首次 bootstrap；例如只运行
  过 `usage scan` 时，usage 不重复扫描，但尚未建立的 session index 仍可首次扫描。
- `--domains` 允许只监控需要的 domain。`--domains usage` 不得隐式运行 session 或
  extension scanner，也不得创建或打开 `sessions.sqlite3`。
- 后续 poll 先比较廉价 inventory，仅把新增、append、mutated、removed paths 交给对应
  scanner；单个 changed path 不能触发全域内容读取。
- Standalone scan 与 watch 必须共用同一个 inventory、checkpoint 和 incremental scanner
  实现，不能维护两套变化判定逻辑。

## 默认 Text 输出

- collections：统一的 `+`、`-`、`|` ASCII grid，单空格 padding、逐行 separator，并按终端显示宽度对齐。
- `provider status` collection：独立布尔列 `CODEX ACTIVE` 与 `CLAUDE ACTIVE`。
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
- `official` 不参与 credential 模型，不写 providers 表或 credential references。
- 不实现 Claude official。
- 不读取、修改或测试真实 HOME、auth.json、credential key file 或 `.vscode/`。
- 实现后必须更新 CLI spec、Phase 9、README 双语、JSON/text golden，并运行 targeted
  tests、`git diff --check`、完整 `make release-verify`，最后清理生成产物。
