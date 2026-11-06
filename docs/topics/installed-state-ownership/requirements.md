---
status: active
created: 2026-11-05
---

# Installed State Ownership — Requirements

版本归属由 `vX-Y-Z-contract` 主题决定，不在本文件中指定。

## 来源

本主题来自 Beads 缺陷 `ad-bug-dev-schema-migrates-release-state`，即开发构建升级真实库、导致已安装版本无法使用。2026-11-05 由用户分流为 Lane B：修复需要定义新的行为契约，不能只是恢复某个已有契约。

同类事故已经发生过两次。下表中，已安装版本支持的 schema、真实库的 schema 和故障表现都是实测结果；"触发者"一列中，凡是没有经过取证的，都明确标为推断：

| 日期 | 已安装版本支持的 schema | 真实库被升级到 | 触发者 | 结果 |
| --- | --- | --- | --- | --- |
| 2026-08-30 | 18 | 21 | 已取证：`main` 的开发构建在真实状态目录上运行期间，迁移 19–21 先后落地，见 [schema-version-signal 需求](../schema-version-signal/requirements.md#how-the-condition-was-produced) | 已安装的 App 和 CLI 报告 `schema_ahead`，provider、usage 不可用 |
| 2026-10-18 | 30 | 31 | **未取证**：具体是哪个进程、在什么时间执行的升级，都没有查明（来源缺陷描述原文："Which development process ran the upgrade, and when, was never established."）。当时 scan-performance 工作区的候选代码含有迁移 31，并且开发构建默认就会打开真实目录，所以具备触发条件，但这只是推断，不能当作已锁定的触发者 | 正式版 v0.6.5 的只读快照返回 `schema_ahead`（count 31，supported_count 30），并报告 `provider_unavailable`、`sessions_unavailable` |

两次的数据库都完好（`quick_check` 为 ok），问题是版本不匹配。第二次的恢复方式是：用户决定在本地构建并安装 v0.7.0-dev，没有降级数据库。

本主题的必要性不依赖第二次事故的具体触发者。可以确认的事实是：任何未安装进程都能按默认路径打开真实库并迁移它（见下文"问题"）；这个缺陷已由源码核实，并且本身足以导致同类事故。

### 为什么现在要改开发构建的运行方式

[schema-version-signal](../schema-version-signal/requirements.md#how-the-condition-was-produced) 当时只决定把这种状况报告清楚，明确不改开发构建的运行方式。理由是：两个已安装版本共存、回滚或共享状态目录时，同样的状况也会出现。那个信号契约至今有效，本主题不改动它。

但信号只能在事故发生后让人看懂，阻止不了下一次。第二次事故证明，开发构建能升级真实库，这件事本身就是缺陷：

- 迁移只能向前，无法撤销；
- 每个正在开发的主题都会把 schema 往上推。2026-11-05 时，`auxiliary-session-classification` 工作区已经是 schema 34，而已安装的 0.7.0 只支持 33。

## 问题

在 `main` 的 `660c815`（schema 33）上核实到以下现状：

1. **默认状态目录不区分运行方式。** CLI 没传 `--state-dir` 时，用的就是 `$HOME/.agentdeck`（`cmd/agentdeck/main.go` 里的 `stateRoot` → `platform.StateRoot`）。没有环境变量或配置能改这个默认值。`go run`、`go test`、`dist/` 产物、worktree 构建都会落到真实目录。
2. **写入就迁移。** `store.Open` → `openAtExistingRoot` 打开库后立即调用 `migrate`，不检查调用者是谁。只读路径 `OpenReadOnly` 遇到更高版本的库会拒绝；写入路径则会直接把库升级上去。
3. **App 自己推算状态目录。** `EmbeddedHelperRunner` 用 `HOME/.agentdeck` 自己算出状态目录，再显式传给 helper。`AgentDeckApp` 也直接读 `.agentdeck/telemetry/receiver.json`。所以从 DerivedData 运行的 Debug App 同样操作真实目录。
4. **Debug App 和安装版共用身份。** Debug 和 Release 配置的 bundle ID、App Group entitlement 相同。Debug App 会写真实的 App Group 容器（`~/Library/Group Containers/N2FZ2FNRTU.group.com.kitdine.agentdeck`）和偏好设置，小组件元数据也会和安装版互相干扰。
5. **开发构建能改真实客户端配置。** provider 切换、`telemetry enable --write-codex-config`、usage hook 和 statusline 的安装、`shell setup` 会写入 `~/.codex/config.toml`、`~/.claude/settings.json` 和 shell rc 文件。开发构建写进去的 hook 或 statusline 指向开发版程序，之后 Claude、Codex 会一直用真实状态调用它。
6. **没有任何事实能区分"已安装"和"其他运行方式"。** `buildinfo` 只有 Version、Commit、Branch、BuildTime。本地安装的 dev 版本号带 `-dev`，`go build` 的是 `dev`，都分辨不出是否已安装。

## 目标

**原则：真实路径的所有权跟着安装走，与版本号和构建方式无关。**

- **G1 安装身份。** 每个进程在运行时判断自己是不是当前安装的那个程序。判断依据只有一个：安装程序写下的安装回执。版本号、构建方式、编译时标记、环境变量、命令参数都不作为依据。判断不了时，一律按"未安装"处理。
- **G2 已安装程序独占真实状态。** 只有当前安装的程序能读写真实状态（`~/.agentdeck`），也只有它能执行真实库的 schema 迁移。不论装的是正式版、rc 还是本地构建的 dev 版，只要它是回执里登记的那个程序，就是合法拥有者。
- **G3 未安装进程默认隔离，且绝不碰真实状态。** 未安装进程不带参数时自动使用隔离状态目录。显式用 `--state-dir` 指向真实状态目录（包括通过符号链接）会被拒绝，并且在读取或迁移之前就拒绝。没有任何开关能绕过。
- **G4 未安装进程不写真实客户端配置。** 未安装进程可以读取真实的会话来源，供扫描使用；但对真实账户主目录下客户端配置和 shell 配置的写入一律拒绝。
- **G5 Debug App 使用独立身份。** Debug 配置的 App、小组件、App Group、偏好设置域都和安装版分开，互不共享。
- **G6 App 不再自己推算状态目录。** App 和小组件使用的状态目录，以 helper 解析的结果为准。
- **G7 只有一个回执、一个拥有者。** 当前账户只有一份用户回执，它是认定当前拥有者的唯一权威，同一时间只登记一个可执行文件。
  - Homebrew cask、`make install`、本地 App 安装在安装时写入这份回执，卸载时清理。
  - Homebrew formula 的安装步骤不能写 `$HOME`，所以 formula 安装的 CLI 要由用户在安装后显式运行一次认领命令（2026-11-05 用户决定），认领之前一律按未安装处理。
  - 程序自身附带的标记一律不能授予所有权。
- **G9 所有权交接要安全。** 安装身份不在进程内永久缓存：每次打开或加锁真实状态之前，都要重新确认自己仍是回执登记的拥有者；长驻进程每一轮都要复核，发现所有权已转移就停止访问并退出。接管只能在旧拥有者停止访问真实状态之后进行：真实状态的锁仍被持有、或已安装的 App 仍在运行时，接管会被拒绝（2026-11-05 用户决定）。
- **G8 状态可见。** 用户能看到当前进程是否已安装、为什么、用的是哪个状态目录；拒绝时给出带类型码的错误，并说明如何恢复。

## 非目标

- 不改 schema-version-signal 的 `schema_ahead` 信号契约。`schema_ahead` 仍然在检测到更高版本时拒绝，不会为了兼容而放宽版本检查。
- 不提供数据库降级或版本标记回退。用真实数据做实验，只能先经授权用 SQLite backup 做副本，再在副本上进行。
- 已安装程序在用 `--state-dir` 指定其他目录（例如备份恢复的目标目录）时，行为不变。
- 不限制读取真实的会话来源（`~/.claude/projects`、`~/.codex/sessions` 等）。读取它们是产品的输入，不是状态写入。
- 不处理多用户或多账户共享安装的情形。所有权按 macOS 当前账户的密码数据库主目录计算。
- 不改动发布、公证、tap 自动化的流程，只在安装和卸载步骤里增加回执的写入与清理。

## 术语

| 术语 | 定义 |
| --- | --- |
| 真实账户主目录 | 当前用户在系统账户数据库里登记的主目录，与 `$HOME` 环境变量无关 |
| 真实状态目录 | 真实账户主目录下的 `.agentdeck/` |
| 真实客户端配置 | 真实账户主目录下由 AgentDeck 写入的客户端和 shell 配置文件，范围以[架构](architecture.md#真实客户端配置写入边界)为准 |
| 安装回执 | 当前账户唯一的一份记录，由安装步骤或 formula 认领写入，登记唯一已安装可执行文件的真实路径和安装方式 |
| 已安装进程 | 自身可执行文件的真实路径与有效回执登记的路径完全一致的进程。每次访问真实状态之前都要重新确认 |
| 未安装进程 | 其他所有进程，包括 `go run`、`go test`、`dist/` 产物、worktree 构建、DerivedData 里的 App，以及回执缺失、损坏或指向别的文件时的任何程序 |
| 隔离状态目录 | 未安装进程默认使用的状态目录，位置见[架构](architecture.md#状态目录解析) |

## 影响范围

| 层面 | 变化 |
| --- | --- |
| CLI（`cmd/agentdeck`） | 状态目录统一经由安装身份解析；新增拒绝错误码；`doctor` 新增 `installation` 检查；未安装进程在 TTY 上提示自己在用隔离目录。界面见 [ux/cli.md](ux/cli.md) |
| 存储（`internal/store`、`internal/platform`） | 打开和迁移前先检查状态目录与安装身份是否匹配 |
| 客户端配置写入（`internal/provider`、`internal/usagehook`、`internal/shellconfig`、`internal/telemetry`、`internal/backup`） | 写入真实客户端配置前先检查安装身份 |
| App 与小组件（`apps/macos`） | 状态目录改为从 helper 获取；Debug 配置使用独立身份；Health 面板显示 `installation` 检查，沿用现有检查行的样式，只新增文案 |
| 安装（Homebrew cask/formula 模板、`scripts/manage-install.sh`、新增的本地 App 安装入口） | 写入和清理回执；强制只有一个拥有者，接管前要求旧拥有者已停止访问；formula 的 caveats 引导用户认领 |
| CLI 认领命令 | 新增 `install claim`，供 formula 安装的 CLI 认领所有权，界面见 [ux/cli.md](ux/cli.md) |
| 产品契约（`docs/specs/cli-design.md`、`docs/specs/cli-manual.md`） | 主题收尾时同步 Local State、Build Identity and Installation、Output and Errors、Doctor 这几节 |

### 界面

- **CLI**：[ux/cli.md](ux/cli.md)。
- **菜单栏 App**：不新增布局或状态。`installation` 检查使用现有 Health 详情的检查行样式，只增加中英文检查名和恢复文案，由[架构](architecture.md#app-与小组件)定义。因此不单独写 `ux/menubar.md`，Documents 矩阵中标为 n/a 并写明理由。
- **小组件**：不改展示。Debug 配置换用独立的小组件扩展身份，属于构建身份的变化，不是界面变化，因此 n/a。
- **Settings**：不改动，n/a。

## 验收边界

- **A1** 由 `go run`、`go test`、`dist/agentdeck`、worktree 中 `make build` 产生的进程，以及 DerivedData 中 Debug 或 Release 配置的 App，在任何入口都不打开、不迁移、不创建、不锁定真实状态目录下的任何文件。回归测试要覆盖：默认目录、显式 `--state-dir`、指向真实目录的符号链接、scan worker 子进程、App 启动的 helper。
- **A2** 以下程序在回执有效时照常使用真实状态目录，现有行为不变，包括迁移：用 Homebrew cask、`make install`、本地 App 安装装好的程序，以及认领过的 Homebrew formula 程序。
- **A3** 回执缺失、格式错误、指向不存在或不同路径的文件时，进程都按未安装处理。`doctor` 的 `installation` 检查要给出具体原因，并引导用户重新安装。
- **A4** 未安装进程写真实客户端配置时会被拒绝，并返回带类型码的错误，现有文件逐字节不变。目标位于非真实主目录（例如测试用的临时 `HOME`）时，写入不受影响。
- **A5** Debug App 与已安装 App 同时运行时互不影响：数据、App Group 容器、偏好设置、小组件列表和小组件元数据都各自独立。
- **A6** 同一时间只有一个安装能拥有真实状态，包括以下情形：保留下来的旧 Homebrew keg、不同的 Homebrew 前缀、stable 与 rc 两个 formula 同时安装、回执过期、在不同安装方式之间迁移。已有回执登记着另一个仍然存在的程序时，安装方式或认领命令要么拒绝并给出迁移提示，要么在用户显式同意替换后接管。"显式同意"的具体形式由[架构](architecture.md#安装回执)定义。
- **A9** 接管完成之后，旧拥有者的进程（包括长驻进程和 App）不能再打开、加锁或读取真实状态。旧拥有者仍在访问时，接管会被拒绝，回执保持不变。
- **A7** 已安装程序升级到包含本主题的版本后，回执由新版本的安装步骤写入，现有数据无需迁移就能继续使用。未写回执的手工复制安装会变成未安装进程，并在 `doctor` 和 TTY 提示中说明原因。
- **A8** schema-version-signal 的现有契约和测试全部保持通过。

## 前提

- formula 的 `post_install` 在 Homebrew 沙箱中运行，不能写 `$HOME`：Homebrew `formula_installer.rb` 的 `post_install` 只允许写 keg、日志和 `HOMEBREW_PREFIX` 下的链接目录。因此 formula 改为由用户在安装后认领，见 G7。
- cask 的安装步骤可以声明 `$HOME` 下的写入路径（Homebrew `cask/artifact/install_steps.rb` 的 `sandbox_write_paths`），所以 cask 在安装时直接写用户回执。这一点要在真实 brew 回归测试中验证，这是交付条件。
- Debug 配置的独立 App Group 需要在开发者账号下注册。由于签名和配置文件属于操作员环境，这一步列为人工前提，见[任务](tasks.md)。
