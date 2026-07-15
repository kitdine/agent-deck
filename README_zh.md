# AgentDeck

[English](README.md) | [简体中文](README_zh.md)

> 一个以 macOS 为首要平台的本地 CLI，用一套命令管理 Codex 和 Claude 的
> provider、用量、会话、扩展、诊断与加密备份。

AgentDeck 面向需要使用多个 Codex 或 Claude provider 的开发者。它提供统一的本地
控制入口，不需要把凭证或会话数据上传到托管服务。Provider 定义保存在 SQLite 中，
凭证留在 macOS Keychain，客户端会话日志始终作为只读数据源。

> **预发布状态：** Phase One 与 version/installation baseline 均已完成实现和独立
> 评审；interactive credential input 与受管 shell completion 安装已经通过完整
> release verification，初评问题已经修复并通过独立复评。当前尚未通过 Homebrew
> 提供安装。

```bash
make build
./dist/agentdeck --help
```

## 为什么使用 AgentDeck

- **统一 CLI：** 通过同一套命令树管理 provider 切换、用量、会话、扩展、诊断与备份。
- **本地优先：** 常规命令不依赖托管的 AgentDeck 服务，也不会暴露网络端口。
- **凭证隔离：** Provider 密钥保存在 macOS Keychain，而不是 AgentDeck 数据库或客户端
  配置备份中。
- **可恢复变更：** Provider 配置写入使用操作日志和脱敏备份，中断后可以诊断和恢复。
- **适合自动化：** 命令支持 text 和 JSON 输出，`watch` 还支持带版本的 NDJSON 事件。

## 当前能力

| 命令 | 功能 |
| --- | --- |
| `agentdeck provider` | 管理 provider、Keychain 凭证引用、provider 选择、状态与恢复。 |
| `agentdeck usage` | 扫描本地用量记录，汇总费用和 token，诊断归因并管理价格目录。 |
| `agentdeck session` | 扫描和搜索允许索引的可见会话文本，管理排除规则、重建或独立清除索引。 |
| `agentdeck extension` | 发现 plugin、MCP server 和 skill，检查健康状态，显式接管或释放管理状态。 |
| `agentdeck watch` | 在前台增量扫描用量、会话和扩展，并输出带版本的 NDJSON 事件。 |
| `agentdeck backup` | 创建、列出、认证、检查和恢复使用 age 加密的可移植备份。 |
| `agentdeck doctor` | 执行快速或完整的只读诊断，并给出可操作的恢复建议。 |
| `agentdeck run` | 将一次明确的 Codex 或 Claude 子进程执行与对应的用量记录关联。 |
| `agentdeck version` | 输出版本、commit、构建时间和 Go runtime identity，便于问题诊断。 |

只有当 adapter 提供明确的原生开关时，扩展的 enable/disable 才会执行写操作；否则保持
只读。可移植备份只能恢复到不存在或为空的 AgentDeck 状态目录。

## 环境要求

- macOS，当前凭证实现依赖 Keychain
- Go `1.26.0`
- GNU Make

依赖已提交到 `vendor/`，常规构建和测试不会下载 Go module。

## 从源码构建

构建本地开发二进制文件：

```bash
make build
./dist/agentdeck --help
```

构建支持的两种 macOS 架构：

```bash
make build-all
```

生成文件位于：

```text
dist/agentdeck_darwin_arm64
dist/agentdeck_darwin_amd64
```

## 从源码安装

为当前用户安装源码构建版本：

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
agentdeck version
```

安装过程会识别实际调用 Make 的 fish、zsh 或 bash，生成对应 completion，并在该
shell 的用户 rc 文件中加入唯一的 AgentDeck 标记 source block。需要时可以覆盖检测
结果或 rc 路径：

```bash
make install COMPLETION_SHELL=fish
make install COMPLETION_SHELL=zsh COMPLETION_RC="$HOME/.config/zsh/.zshrc"
make install COMPLETION_SHELL=none
```

`COMPLETION_SHELL=none` 明确表示只安装 binary。无法识别 shell、rc 路径不安全或
managed block 冲突时，安装会停止且不会留下部分 binary 或 rc 修改。

默认安装路径与 AgentDeck 用户状态相互独立：

```text
~/.local/bin/agentdeck                  # 可执行文件
~/.local/share/agentdeck/               # 安装归属 manifest
~/.agentdeck/                           # 数据库、索引和备份
```

需要其他用户级目录时，可以指定 `PREFIX`：

```bash
make install PREFIX="$HOME/tools/agentdeck"
```

后续升级和卸载该安装时，必须继续传入相同的 `PREFIX`：

```bash
make install PREFIX="$HOME/tools/agentdeck" FORCE=1
make uninstall PREFIX="$HOME/tools/agentdeck"
```

安装默认拒绝覆盖已有二进制文件或 manifest。确认当前 binary identity 后，显式执行升级：

```bash
make install FORCE=1
agentdeck version
```

安全删除未被修改的安装：

```bash
make uninstall
```

卸载前会核对 binary、生成的 completion 和完整 managed rc block。block 外的用户
修改会被保留；任一受管 artifact 或 block 被修改后都会 fail closed。卸载始终保留
rc 文件、`~/.agentdeck/`、Keychain 凭证、客户端配置和备份。
空的安装目录可能继续保留，因为它们不是 manifest 所有的 artifact，卸载不会声明对
这些目录的所有权。

## 快速开始

先执行只读诊断并查看命令帮助：

```bash
./dist/agentdeck doctor
./dist/agentdeck provider --help
./dist/agentdeck usage --help
```

扫描并查看本地用量和会话：

```bash
./dist/agentdeck usage scan
./dist/agentdeck usage summary
./dist/agentdeck session scan
./dist/agentdeck session search "project name"
```

发现本地扩展：

```bash
./dist/agentdeck extension scan
./dist/agentdeck extension list
```

自动化场景可以使用 JSON，前台 watcher 可以使用 NDJSON：

```bash
./dist/agentdeck --format json provider list
./dist/agentdeck --format ndjson watch
```

执行会修改状态的操作前，先运行 `agentdeck <command> --help` 查看准确参数和安全约束。

## 构建身份

报告问题时应附上 binary identity：

```bash
agentdeck version
agentdeck --version
agentdeck --format json version
```

除非通过 Make 显式传入 `VERSION`、`COMMIT` 和 `BUILD_TIME`，源码构建默认分别显示
`dev`、`unknown` 和 `unknown`。后续 release 工具可以注入这些值，不改变运行时契约。

## Shell completion

completion 命令只负责把脚本输出到 stdout。`make install` 负责持久启用；临时启用或
交给自定义 completion manager 时可以直接使用 generator：

```bash
agentdeck completion fish | source
agentdeck completion zsh > /tmp/_agentdeck
agentdeck completion bash > /tmp/agentdeck.bash
```

如果持久 completion 未生效，应检查所选 rc 文件中
`# >>> agentdeck completion >>>` 到 `# <<< agentdeck completion <<<` 之间的
managed block。不要手工修改该 block；内容与 ownership manifest 不一致时，安装和
卸载会按设计 fail closed。

## Provider 配置示例

下面使用虚构的 endpoint 和凭证引用。`provider add` 会无回显提示一次 credential，
并在创建 provider 的同时把凭证保存到 macOS Keychain。

```bash
./dist/agentdeck provider add work https://api.example.com/v1 work-codex 1 codex
./dist/agentdeck provider show work
```

`provider credential add` 和 `provider credential update` 继续用于独立预置与轮换。
自动化可以通过 stdin 提供一行 credential；credential 不接受 CLI 参数或环境变量。

选择 provider 时，需要提供客户端配置路径和脱敏备份的目标路径：

```bash
./dist/agentdeck provider use \
  work codex \
  "$HOME/.codex/config.toml" \
  "$HOME/.agentdeck/codex-config.redacted.toml"
```

AgentDeck 只修改约定的 provider 字段，并保留其他客户端配置。

## 本地数据与隐私

AgentDeck 默认使用 `~/.agentdeck/` 作为持久化状态目录：

```text
~/.agentdeck/
├── agentdeck.sqlite3   # provider、用量、扩展和备份元数据
└── sessions.sqlite3    # 可独立清除的可见会话索引
```

可以用 `--state-dir <path>` 创建隔离状态。AgentDeck 会保留调用时的当前目录作为
project-scope extension 发现上下文；运行时不会使用安装目录，也不会切换到
`~/.agentdeck/`。

- Provider 凭证值保存在 macOS Keychain。
- Codex 和 Claude 会话日志是只读输入。
- 会话索引只保存允许收录的可见对话字段。
- 清除会话索引不会删除客户端源日志。
- 常规命令不会探测 provider host，也不会访问网络。
- 只有显式执行 `agentdeck usage price update` 时才允许访问网络。
- 自动化测试使用临时 home、合成日志和虚假 secret store。

## 开发与验证

开发时执行有针对性的检查，交付前执行完整 release gate：

```bash
make test
make test-race
make vet
make release-verify
```

`make release-verify` 会运行 Go 测试、race detector、`go vet`、两种 macOS 架构构建、
arm64 stripped 二进制体积门禁以及仓库隐私扫描。

清理生成的二进制文件：

```bash
make clean
```

## 项目结构

```text
cmd/agentdeck/   Cobra CLI 入口与端到端契约测试
internal/        Provider、用量、会话、扩展、备份和平台实现
scripts/         发布隐私检查
docs/specs/      已批准的行为和架构契约
docs/plans/      执行状态与完成门禁
vendor/          已提交的 Go 依赖
```

## 文档与状态

- [文档索引](docs/README.md)
- [Phase One 实施计划](docs/plans/2026-07-13-agentdeck-cli.md)
- [CLI 设计与契约](docs/specs/2026-07-13-agentdeck-cli-design.md)

仓库代码、测试、配置、Git 历史和以上 active 文档共同构成项目事实来源。

## 发布路线

只有首个版本化 release 为 arm64 和 amd64 提供签名或 checksum 校验的 macOS 归档后，
才会开始 Homebrew 集成。计划中的使用方式是：

```bash
brew tap kitdine/tap
brew install agentdeck
```

这些命令目前尚不可用。未来 formula 将安装不可变的 release 归档，而不是从持续变化的
Git 分支构建。

## 参与贡献

改动应遵循 active 设计和计划的范围，保持隐私边界，并在交付前运行
`make release-verify`。仓库特有的开发和授权规则见 [AGENTS.md](AGENTS.md)。

## License

本仓库当前尚未包含 license 文件。
