---
status: active
created: 2026-11-05
updated: 2026-11-11
---

# Installed State Ownership — Tasks

本文件是本主题唯一的状态权威。来源缺陷是 Beads `ad-bug-dev-schema-migrates-release-state`（Lane B），只有本主题的修复交付后才能关闭它。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
| ux/menubar.md | n/a | n/a |
| ux/widget.md | n/a | n/a |
| ux/settings.md | n/a | n/a |

三个 n/a 的理由：

- **菜单栏**：只在 Health 详情里增加一行 `installation` 检查。这一行沿用现有检查行的样式，没有新布局，也没有新状态；新增的检查名和恢复文案在 [architecture.md](architecture.md#app-与小组件) 中给出。
- **小组件**：Debug 配置改用独立的扩展身份，这是构建身份的变化，界面不变。
- **Settings**：不涉及。

需求和 CLI 保持已通过状态。installer-receipts 设计修订已通过架构 Round 12、任务分解 Round 15 独立复评，全部发现关闭，两项 Review 勾选；按用户决定删除工作区根目录未跟踪的构建产物 `agentdeck` 后，L0 全部通过，两份文档的证据门禁 VERIFIED（见架构 Round 12 的门禁同步）；任务 5 cask 开发可在对应开发指令下继续。CLI 的逐字符标本在共享原型 `?surface=cli&ownership=<state>`（`prototype/src/ownership.js`）。逐文档记录见
[需求](reviews/requirements.md)、[CLI](reviews/ux-cli.md)、
[架构](reviews/architecture.md)、[任务分解](reviews/tasks.md)。
`installation-identity` 已通过独立复评，提交态门禁 VERIFIED，并在本地提交 `1425256d96b510e060a9bd0496df769b67495c33` 中交付；见[任务评审](reviews/installation-identity.md)。`state-root-guard` 已通过 Round 2 独立复评；评审候选与证据门禁见[任务评审](reviews/state-root-guard.md)，本地提交交付由 Beads `ad-installed-state-ownership-state-root-guard` 的交付记录及对应 Git 提交核验。`client-config-guard` 已通过 Round 4 独立复评，全部发现已关闭，精确状态门禁 VERIFIED，待授权提交；见[任务评审](reviews/client-config-guard.md)。`installation-diagnostics` 已通过 Round 3 独立复评，候选身份、证据门禁和交付建议见[任务评审](reviews/installation-diagnostics.md)。任务 5 保留 Round 4 独立复评 PASS，validation 已通过；当前门禁 NOT_VERIFIED，仅缺 real-brew，按既定时序例外等待另行授权交付，见[任务评审](reviews/installer-receipts.md) Validation follow-up；任务 7 已通过 Round 1 独立评审，已按 Apple 官方规则更正注册前提，完成门禁 VERIFIED；候选身份与验证边界见[任务评审](reviews/debug-app-identity.md)；任务 6 已通过 Round 2 独立复评（ISO-APP-R1-F1 closed），候选身份与证据门禁见[任务评审](reviews/app-state-location.md)，待授权提交；任务 8、9 尚未开发，各任务开工仍需对应开发指令。来源缺陷保持开放。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `installation-identity` | [x] | [x] |
| 2. `state-root-guard` | [x] | [x] |
| 3. `client-config-guard` | [x] | [x] |
| 4. `installation-diagnostics` | [x] | [x] |
| 5. `installer-receipts` | [x] | [x] |
| 6. `app-state-location` | [x] | [x] |
| 7. `debug-app-identity` | [x] | [x] |
| 8. `ownership-acceptance` | [ ] | [ ] |
| 9. `contract-reconciliation` | [ ] | [ ] |

### 分解原则与开工条件

本稿以已交付任务 1 为基线，保留九个任务锚点。重写改变的是剩余工作的交付边界与验收归属，产品行为仍以需求、CLI 和架构为准。历史 Round 8 评审不覆盖本稿。

- 每项任务的实现、测试和完成条件，只消费已交付前置任务或本任务产生的能力。安装命令、App 或客户端守卫尚未实现时，不得以替身测试宣称它们已经通过验收。
- 一个功能由一个任务实现；跨任务组合由任务 8 验收。局部通过不等于 A1–A9 整体通过。任务 8 复用仍有效的局部证据，补齐组合路径和最终入口清单。
- 守卫上线即会改变 doctor 的错误路径，因此其部分报告与基础 `installation` 检查归任务 2 同时交付。任务 4 只扩展受管命令诊断和 helper 路径查询。
- 接管发布、暂停、崩溃、旧清理与新安装的交错测试归实现安装协议的任务 5；任务 2 只验证状态访问会话与已存在的锁/续登接口。
- 任务 2 必须覆盖其候选上已有的全部 Cobra 叶子命令，不能按实现者认定“是否消费状态”缩小清单；任务 3 的真实客户端配置拒绝、任务 4/5 尚未新增的命令归各自产出任务。任务 8 在最终叶子集上复跑完整矩阵，并组合所有安装方式与 App。
- 本稿评审通过前，既有未开始的实现任务均由文档任务阻塞；不得把草稿范围写成已批准调度。通过后由评审交接核对下表，更新原任务及依赖，不另建一批重复任务。任务 2 此前的开发指令已由用户撤回；恢复开发需要新指令。

### 依赖与执行顺序

下表只列直接能力依赖；所有未开始任务另有“本稿通过评审及对应开发授权”的共同前提。编号是稳定身份，不等于必须按编号连续执行。

| 任务 | 直接能力依赖 | 本任务完成时可独立检查的产物 |
| --- | --- | --- |
| 1 `installation-identity` | 已交付基线 | Resolver、回执、物理路径解析、所有权锁、续登函数 |
| 2 `state-root-guard` | 1 | Go 状态访问边界、会话生命期、CLI 拒绝/静默路径、doctor 短路报告 |
| 3 `client-config-guard` | 2 | 各客户端配置写入边界及 `client_config` 错误 |
| 4 `installation-diagnostics` | 2 | 受管命令归属诊断、`desktop state-location` |
| 5 `installer-receipts` | 2 | 真正的安装/认领/卸载协议及其完整交错测试 |
| 6 `app-state-location` | 4、7 | App 使用 helper 路径、原生共享会话和 Health 文案 |
| 7 `debug-app-identity` | 无代码前置；需有效签名身份，不需要门户注册 App Group | 独立构建身份、签名与静态隔离证据、现有宿主测试回归 |
| 8 `ownership-acceptance` | 3、5、6（传递包含 1、2、4、7） | 最终命令集合、真实安装协议与 App 的跨面 A1–A9 验收 |
| 9 `contract-reconciliation` | 8 | 与已验收行为一致的稳定契约和用户说明 |

建议执行序列：**1（已交付）→ 2 → 3 → 4 → 5 → 7 → 6 → 8 → 9**。任务 7 的真实签名核验与无签名宿主测试分别取证；当前 Team ID 前缀 App Group 无门户注册前提。任务 7 自己验证构建身份变更的原生回归；任务 6 再消费该身份实现并验证新的状态路径/会话行为。现有 `scripts/test-macos-app.sh` 已隔离 HOME 和宿主状态，不能以“等任务 6 隔离”为由延后任务 7 的回归。多个任务会修改 `main.go`、`platform/state.go`、telemetry 或原生构建，因此按序交接文件，不并行修改。

### 共用验证边界

所有测试一律使用临时 `HOME`、临时状态目录，并按[测试与验收的隔离](architecture.md#测试与验收的隔离)替换"真实账户主目录"，绝不触碰本机真实的 `~/.agentdeck`、`~/.claude`、`~/.codex` 或回执。

凡是证明"未安装进程不访问真实状态"的测试，都必须在[访问屏障](architecture.md#测试与验收的隔离)下运行。屏障禁止访问 `P` 中各入口之下的任何内容，只允许读取入口本身的元数据。每次运行前先做两项对照：

- 正向对照：用待测二进制真正执行一次"物理解析 → 守卫 → 拒绝"，必须以 `state_ownership_refused` 结束，不能被终止；
- 阴性对照：对目录内部访问一次，进程必须被终止。

只断言文件清单、内容和 mtime 不变，不能作为"没有访问"的证据。屏障不可用（`sandbox-exec` 缺失，或者嵌套时 `sandbox_apply` 返回 exit 71）时测试失败，这类测试要在没有外层沙箱的环境中运行。

任务 2 建立 Go 屏障/账户/进程夹具；任务 3、4、5、8 复用，任务 6 自己产出下文定义的原生夹具和验收 helper。任务 7 使用已有的隔离原生 harness。新测试文件的落点由对应包现有测试和验收脚本发现，新增后在任务评审记录中给出实际命令、输入、预期和覆盖边界。Go 账户替换只在 `agentdeck_acceptance` 二进制或包内私有测试缝生效；Swift 账户/锁路径仅由任务 6 的测试依赖注入替换。生产账户解析不接受环境或命令覆盖，App 构建/打包脚本不带验收 tag。

Go 验证统一经 `scripts/run-go-test.sh`。L2 的 Go 核心变更要求定向测试及 `scripts/run-go-test.sh ./...`；L3 再补相应 race、vet、构建或安装检查。测试夹具可以直接使用任务 1 的持锁回执 API 建立身份，但这只证明状态消费方，不能替代任务 5 的安装命令证据。任何仍需后续任务实现的行为，都不能填进本任务的通过条件或以 skip 充数。

### 1. `installation-identity`

**结果**：进程能可靠地判断自己是否已安装（需求 G1）。

**文件**：
- 新建 `internal/installation/installation.go`、`receipt.go` 及对应测试；
- 新建 `internal/installation/resolver_acceptance.go`，带 `//go:build agentdeck_acceptance`；
- `internal/platform/state.go` 及测试：增加 Resolver 注入、真实账户主目录解析、物理解析 `walk` 和受保护集合 `P` 的计算、未导出且只在 `_test.go` 中替换的注入字段；
- 新建 `internal/platform/ownership_lock.go` 及测试：所有权锁的共享和排他取得，各带 5 秒上限；
- `cmd/agentdeck/main.go`：启动时注入 Resolver。
- 验证工具补齐：`cmd/agentdeck-perf/run.go`、`run_test.go`，以及 `internal/perftrace` 的 `perftrace.go`、`perftrace_test.go`、`lock_unix.go`、`lock_other.go`；复用仓库既有的 trace 完成屏障，范围与来源见任务评审记录。

**要求**：
- 按[架构](architecture.md#安装身份判定)实现判定步骤：只认唯一的用户回执，没有 keg 回执；回执登记不可变的真实路径，判定时直接与 `self` 比较，**不解析任何链接**；`Lstat` 报告不存在就视为过期。
- 实现[回执格式](architecture.md#回执格式)的全部校验：版本号、绝对真实路径、channel 枚举、`installation_id` 格式、未知字段、4 KiB 上限、`O_NOFOLLOW`。每次登记生成新的 128-bit 随机 ID；测试同路径/channel/版本、相同时间戳的两次登记仍有不同 ID。ID 仅用于清理条件，不改变 `Resolve` 的路径判定。回执只有一种记录，没有 `state` 字段，也没有预留记录。
- 未注入 Resolver 时，默认按未安装处理。判定结果不在进程内永久缓存。
- 测试覆盖五个原因码；四种 channel；通过符号链接调用；保留下来的旧 keg、两个 Homebrew 前缀、stable 与 rc 同时存在时只有回执登记的那一个是已安装；回执在进程运行中改写后，下一次判定立即反映。还要证明 `$HOME` 和环境变量不影响生产构建的判定。
- **opt 切换不改变所有权**：回执登记 keg A 时，把 opt 从 A 切到 B，B 仍然判为未安装（`receipt_other_executable`），A 仍然是已安装。这是评审 Round 3 反例的直接回归测试。
- 按 [formula 升级续登](architecture.md#formula-升级续登)实现续登条件的判定和续登函数。任务 2 在两个入口调用它：默认目录解析、守卫的真实根分支。表驱动测试覆盖：四个条件全部满足；以及逐一缺少其中一个条件——旧 keg 仍存在、跨 formula、跨前缀、`self` 不是 opt 当前指向的 keg。
- 验收 tag 文件只在 `agentdeck_acceptance` 下编译。用 `go tool nm` 检查普通构建产物中没有它的符号，这项检查加入 `check-install`。
- 按[物理解析](architecture.md#什么算指向真实状态目录)实现 `walk`：在解析符号链接之前不做词法 `Clean`；对 `next` 先与 `P` 比较，命中就拒绝，不再 `Lstat`；返回实际要用的物理路径。表驱动测试覆盖架构列出的全部反例，其中必须包括评审给出的"链接后接 `..` 回到真实目录"（`outside/jump → account/inner`，候选为 `outside/jump/../.agentdeck/sentinel`）。另外用一个对照测试证明：同一候选路径如果先 `Clean`，会被误判为根外，而 `walk` 会拒绝。
- 按[所有权锁与交接](architecture.md#所有权锁与交接)实现锁。测试覆盖：共享锁之间不互斥；共享锁和排他锁互斥；进程退出后锁自动释放；两种锁各自超时时返回 `ownership_busy` 或 `owner_running`。

**验证：L2**。targeted 测试，以及 `scripts/run-go-test.sh ./...`。

开发交接（2026-11-06）：已提供 `installation.Resolver`、严格回执读写及锁内 formula 续登；`platform` 提供 Resolver 注入、物理路径解析和共享/排他锁，CLI 仅接入启动注入。真实状态访问守卫及其调用点仍由任务 2 实现。

使用 Go 1.27.3，installation/platform 定向及 `-race` 测试、验收 tag 隔离测试、`make vet build-all check-install` 均通过。`check-install` 已包含普通构建的 `go tool nm` 验收 Resolver 缺席检查，并用带 tag 构建作符号存在的正向对照。路径测试验证物理解析与拒绝前的调用顺序，不代替任务 2 的操作系统访问屏障验收。

2026-11-07 已补齐验证：原阻断用例、全量 Go 回归（28 个包）、相关 race、vet、双架构 build-all 与 check-install 均通过。诊断、验证工具同步范围、精确目标门禁及交付状态见[任务评审记录](reviews/installation-identity.md#validation-follow-up--2026-11-07)。历史失败及其原始证据保留在该记录和 CEv1 中。

### 2. `state-root-guard`

**依赖**：任务 1。**交付结果**：Go 的状态访问入口统一受保护，未安装进程默认隔离，拥有者以共享会话访问真实状态，长驻访问失效时停止；CLI 状态错误、Hook 静默和 doctor 短路同时可用。覆盖 G2/G3/G9 的 Go 边界及 G8 基础提示；完整 A1/A2/A9 在任务 8 汇总。

**文件**：
- `cmd/agentdeck/main.go`：`stateRoot`、`shellStateRoot`、错误码映射、文本渲染、TTY 提示；
- `internal/installation`：`StateRoot`；
- `internal/platform/state.go`：`GuardStateRoot`、`RevalidateOwnership`；
- 在以下位置的打开和加锁入口加守卫：`internal/store`（`store.go`、sessions、lock）、`internal/credentialvault`、`internal/scanruntime`、`internal/hookrefusal`，以及 telemetry receiver 配置的读写；
- 在长驻循环中加复核：`watch`、telemetry 监听、desktop bootstrap 流、scan worker 的每一轮；
- `internal/doctor/doctor.go`、`newDoctorCommand` 和文本渲染：基础 `installation` 检查、状态缺失与拒绝的部分报告；
- 上述包的测试，以及 `cmd/agentdeck` 的固定测试。

**输入/输出接口**：消费任务 1 的 Resolver、物理解析、共享/排他锁和 formula 续登；产出 `installation.StateRoot`、`GuardStateRoot`、`RevalidateOwnership`、状态拒绝类型及 CLI 映射。安装器不存在时，测试通过任务 1 的持锁回执 API 建立隔离身份。

**实施顺序与验收**：
- 按[状态目录解析](architecture.md#状态目录解析)实现默认目录，用任务 1 的 `walk` 判断"指向真实状态目录"。
- `GuardStateRoot` 返回物理路径。所有调用者此后都只能使用这个返回值，不能再使用原始字符串；评审材料中要列出每个调用点，并说明它改用了返回值。
- 已安装进程第一次访问真实状态前，按[所有权会话](architecture.md#所有权锁与交接)取共享锁，并在锁内判定身份；共享锁一直持有到对真实状态的访问全部结束。覆盖单次命令、Hook、`OpenReadOnly`、scan worker 和长驻循环。重新枚举所有"接收 `stateRoot` 参数、并会创建、打开、加锁、stat 或删除文件"的导出函数，以及所有长驻循环，最终清单记录在评审材料中。
- **续登的两个入口（ISO-ARCH-R4-F2）**：按[状态目录解析](architecture.md#状态目录解析)，没传 `--state-dir` 时，`StateRoot` 在选择默认目录之前先检查续登条件，满足就续登，然后重新判定。续登成功就使用真实默认目录；续登失败返回 `state_ownership_refused`，不退回隔离目录。显式指向真实状态时，由守卫在拒绝之前尝试续登。显式指向非真实目录时不续登。两个入口调用任务 1 的同一个续登函数；续登过程只读写回执和所有权锁，不访问 `P`。
- 新增错误码 `state_ownership_refused`，退出码 `1`，details 为 `resource` 和 `reason`。文本和 TTY 提示与原型 `?surface=cli&ownership=` 中 `notice`、`refused_state`、`revoked` 三个状态逐字一致（见 [ux/cli.md](ux/cli.md#标本)）。
- Hook 和 statusline 遇到拒绝时保持静默失败开放：exit 0，不输出，也不写任何拒绝记录。
- **doctor 与守卫同批交付**：把 `installation` 放在第一个检查，字段和文本遵守 [CLI 契约](ux/cli.md#doctor-的-installation-检查)。隔离目录缺失时保持 `state_missing`、`partial: true`、`checks_skipped` 和 exit 0；显式真实根被拒绝时，在 `newDoctorCommand` 构造只有 `installation` 与 `state` 的部分报告，不把被拒绝路径交给 `doctor.Service`。两种结尾文案分别与 `doctor_not_installed` / `doctor_formula_unclaimed`、`doctor_refused` 标本一致。formula 恢复提示可以引用已批准但由任务 5 实现的命令，本任务不执行或宣称验证该命令。`managed_command_owner` 扩展归任务 4。
- **现有 Cobra 叶子的结构性兜底（ISO-TASKS-R9-F1）**：从本候选的真实命令树和 `cmd/agentdeck/contract_test.go` 固定清单枚举全部已有叶子，逐项提供合法参数、stdin、有效数据、默认/显式非真实/显式真实路径分支、退出码/输出及契约依据。覆盖表与枚举结果必须相互校验，缺项或多余条目都失败；不能由实现者先判断“实际消费状态根”再选择被测命令。现有 `leafCommands` 为 JSON 固定契约排除的 shell 脚本、交互和内部 runtime 表面须单列其运行器、预期及依据，scan worker 也在清单中；这些排除不构成状态访问测试豁免。无状态入口仍在屏障下验证其既定结果，不能以缺参、非法输入、未知命令或无依据 skip 代替覆盖。
- **包级与路径覆盖**：另行枚举接收状态根并访问文件的导出函数，与 CLI 清单分开核对；包级直接调用、worker 子进程和 doctor 拒绝路径都在屏障及正负对照下运行。路径组覆盖直接路径、符号链接、`..`、真实根为链接及子路径，并复用任务 1 的大小写/Unicode 别名反例。
- **本任务输出断言**：空 provider 夹具默认 `provider list` 为 exit 0 / `No providers.`；默认 Hook/statusline 遵守正常隔离处理结果；显式真实根下普通命令 exit 1、doctor 成功部分报告 exit 0、Hook/statusline 静默 exit 0。文本 TTY 只提示一次，JSON/NDJSON、非 TTY、`--quiet` 无提示。客户端配置写入命令照常纳入，配置目标放在非真实账户的临时 HOME。任务 2 唯一不承担的后续行为是任务 3 的真实账户配置拒绝，以及任务 4/5 在本候选尚不存在的新命令；分别注明产出者，不从现有叶子集中删去配置命令。任务 8 在最终叶子集上复跑并加入这些后续行为。
- **受控交错测试（ISO-ARCH-R1-F2）**：用测试钩子在指定位置暂停，逐一构造以下顺序，断言结果：
  1. A 已取得共享锁并打开真实状态 → B 尝试接管：B 等到上限后以 `owner_running` 失败，回执不变；A 正常完成。
  2. B 已取得排他锁 → Hook 启动 A：A 在取共享锁处等待；B 写入新回执并释放锁后，A 读到新回执，在访问之前被拒绝。这一步在访问屏障下运行，证明 A 没有访问。
  3. A 只读打开（`OpenReadOnly`）期间 → B 尝试接管：结果同第 1 条，证明读路径同样受保护。
  4. 持锁跨轮的长驻进程运行期间 → B 尝试接管：B 失败；长驻进程退出后 B 成功，此后旧可执行文件在屏障下被拒绝。
  5. 长驻进程逐轮持锁、在两轮之间释放 → B 在两轮之间接管：长驻进程在下一轮的复核处以 `state_ownership_refused` 退出，用轮次计数证明它没有开始新一轮。
  6. **评审 Round 3 反例**：旧 keg A 是登记的拥有者，持有共享锁、正在读或写会话中 → opt 从 A 切到 B → 启动 B：B 不是拥有者，不满足续登条件（A 仍存在），在访问之前被拒绝；A 的会话不受影响。
  7. 续登：A 持有共享锁时删除 keg A、opt 指向 B → 启动 B：B 尝试续登，排他锁一直等到 A 的会话结束（超过上限则以 `ownership_busy` 拒绝本次访问，回执不变）；A 结束后，B 在锁内续登并成为拥有者；此后 A 的残留进程在下一次过守卫时以 `executable_unresolved` 被拒绝。
  8. **默认命令续登（ISO-ARCH-R4-F2）**：满足续登条件的新 keg，执行一条不带 `--state-dir` 的普通命令。断言：在选择目录之前完成续登，命令使用真实默认目录（注入的真实目录）中的原有数据，而不是空的隔离目录。续登取锁超时时，命令以 `state_ownership_refused`（`ownership_busy`）失败，不读写隔离目录。显式 `--state-dir <隔离目录>` 时不续登。

   这里的 B 是通过任务 1 的真实排他锁和回执 API 操作临时账户的测试进程，不是尚未实现的安装命令。测试须执行真实 `flock` 和状态消费路径，不能仅靠锁的 mock。`state.lock` 的现有语义不变，交错测试不依赖它。旧版第 8–10、12 条全部移到任务 5 的安装协议交错表，保留要求，变更验收归属。
- 已安装身份下的现有测试全部通过，行为不变。

**验证：L3**。定向覆盖 installation/platform/store/credentialvault/scanruntime/hookrefusal/telemetry/watch/doctor 和 `cmd/agentdeck`，以实现后的入口清单补充其他包；Go 全量回归，platform/store/scanruntime 及其他改动并发包的 `-race`，`make vet`，屏障下全部现有叶子及包级/验收二进制测试。评审材料必须同时给出叶子清单与逐项结果、守卫返回路径的调用点清单、会话释放时机、上述 8 种交错结果和 doctor 固定输出。该门禁只需要任务 1 与本任务产物。


#### 开发交接 — 2026-11-07

本节是 Task 2 的实现与验证材料，不是独立评审结论。Dev 已完成，Review 保持未勾选。候选基线为 `c2484dee09405f578bb39037a6c5eac717ec8ad2`；工作区 `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。

CEv1 WorkUnit 为 `installed-state-ownership:state-root-guard`，候选 ContentState 为 `installed-state-ownership:state-root-guard:dev-20261107-Gas5YUA6`。该记录保存最终 HEAD、逐路径 blob 清单、SHA-256 指纹、工具链和验证事实。指纹配方是 `head=<HEAD>\n` 后接按路径排序的 `<path>=<git-blob>\n`；状态文件不嵌入自己的指纹。独立评审是尚待满足的单独 criterion。

**交付行为与会话生命期**

- 未安装默认目录为 `$HOME/.agentdeck-dev`；显式非真实目录不续登；默认解析和显式真实根使用任务 1 的同一续登函数。续登取锁失败不会转入空隔离目录。
- `GuardStateRoot` 在读取受保护内容之前判定、取共享锁并锁内重读身份，返回实际使用的物理路径。普通 CLI、Hook、只读池、worker 和跨轮句柄共用进程会话；单个 Store.Close 不提前释放它。生产 CLI 持锁直到内核完成进程退出。显式 `ReleaseStateOwnership` 只用于所有句柄、池、锁和后台工作均已结束的会话边界。
- watch 每轮、telemetry 每个请求/事件、bootstrap 每次流输出以及 worker 接收/每轮执行均复核。失败退出前保留清理生命期；worker 的 JSON 往返保留 typed refusal，三种原因与三个 scope 的回归测试覆盖了原先丢失原因的路径。
- CLI 错误、TTY 一次性提示、Hook/statusline 静默和 doctor 的缺失/拒绝部分报告均已接入。空隔离库存的文本按原型输出 `No providers.`，虚拟 official 定义仍保留在既有 JSON 合同中；已安装文本不变。
- 触及 AgentDeck 状态的 18 个测试包通过私有 `_test.go` 初始化隔离 HOME/账户；platform 保留 nil Resolver 身份语义，只替换私有路径解析边界。两个旧的独立 CLI 运行夹具改用 `agentdeck_acceptance`。perf 工具的测试环境注入只用于注册的带 tag 夹具，生产环境白名单和生产账户解析没有环境覆盖开关。

**Go 入口与物理路径使用清单**

`TestOwnershipPackageEntrypointInventory` 独立枚举参数或接收者持有状态根的导出入口，并与直接调用清单核对。62 个入口逐个在操作系统访问屏障内运行。纯路径构造/惰性构造器，以及消费既有 Store、由下层已守卫锁/配置 API 承担文件边界的 quota 方法，均在测试清单中逐项说明，不充作访问屏障结果。

| 包 | 直接验证的入口 | 返回路径的使用与拒绝行为 |
| --- | --- | --- |
| platform | `GuardStateRoot`、`RevalidateOwnership`、`EnsureStateRoot`、`EnsureStateRootWithFS` | Guard 返回物理路径；EnsureStateRootWithFS 将返回值交给 MkdirAll/Chmod；Revalidate 失败保留会话到调用者完成句柄清理。 |
| installation | `StateRoot`、`StateRootWithHome` | 默认和显式入口都返回 Guard 的结果；HOME 和 override 的 link/.. 均保留到物理解析；两个续登入口复用同一 RenewFormula。 |
| store | `Open`、`OpenExisting`、`OpenReadOnly`、`OpenWithLockHeld`、`OpenSessions`、`OpenSessionsReadOnly`、`AcquireLock`、`AcquireScanLock`、`AcquireQuotaRefreshLock`、`AcquireDerivedSnapshotCacheLock`、`AcquireServingSnapshotLock`、`InspectScanLockContention`、`OpenServingGenerationReader`、`ReadServingGeneration` | 各 open、read-only、sessions 和 serving reader 先替换 stateRoot；锁的最底层及 contention 包装均使用物理根；无 error 返回的 Inspect 拒绝后返回 nil。 |
| credentialvault | `Vault.Seal`、`Vault.SealExisting`、`Vault.Open`、`Vault.InspectKey`、`Vault.InspectKeyIDs`、`Vault.InitializeNew` | key/createSeedExclusive 对 Vault 作局部副本并替换 stateRoot；读、创建、同步与 KeyPath 计算在副本上执行，避免共享 Vault 字段竞态。 |
| hookrefusal | `Read`、`Live`、`Write`、`Clear` | Read/Live 拒绝后返回无记录；Write/Clear 返回拒绝错误；不增加安装拒绝记录。 |
| telemetry | `LoadConfig`、`SaveConfig`、`EnsureToken`、`ReadToken`、`ReadListenerState`、`ReadStatus`、`WriteManagedBlock`、`RemoveManagedBlock`、`Serve`、`Receiver.ServeHTTP` | 配置、token、listener、managed record、status 和 Serve 先替换根；每个 HTTP 请求和监听循环事件先复核。 |
| scanruntime | `Client.Request`、`Client.RequestWithProgress`、`Serve`、`WithMaintenance`、`AcquireMaintenance` | prepareStateRoot 替换旧的 Abs/Clean 链；Client、worker 和 maintenance 共用物理根；worker 接收/每轮执行复核，结果线路保留拒绝 resource/reason/revoked。 |
| doctor | `Service.Check` | Service 入口先守卫；CLI 拒绝分支直接构造 installation/state 两行，拒绝路径不传给 Service。 |
| session | `CheckHealth` | CheckHealth 在 stat/SQLite open 前替换 stateRoot。 |
| extension | `DoctorFromStateRoot`、`DoctorFromStateRootWithDiscoverer` | 两个 DoctorFromStateRoot 入口汇入已替换根的实现，先守卫再 stat 核心库。 |
| backup | `Service.Create`、`Restore` | Create 替换 Service.StateRoot；Restore 完成输入归档校验后、首次目标访问前替换 targetRoot。 |
| desktop | `Service.Build`、`Service.BuildToday`、`Service.PublishDerivedSnapshotCache`、`Service.PublishServingSnapshot`、`Service.ReadServingSnapshot`、`Service.ReadOrBootstrapServingSnapshot` | Build、BuildToday、两种 publish、read/bootstrap 和 serving read 在目录/缓存/数据库访问前替换 Service.StateRoot。 |
| provider | `Service.Use`、`Service.UseCredential`、`Service.ProjectAttributionGateStatus`、`Service.RefreshProjectAttributionGate` | Use/UseCredential、project attribution gate 和 managed backup path 在相应状态文件访问前使用物理根。 |
| watch | `Service.Poll` | CLI 将已经守卫的物理根传给 Service；Poll 在任何 snapshot/scan callback 前复核。 |

CLI 的 `stateRoot` 缓存已守卫结果；`shellStateRoot` 的显式根使用同一物理结果，未指定根时保持生成命令的可移植默认形式。所有权共享锁与原有 `state.lock`/扫描锁分别管理。

**Cobra 入口与逐项结果**

以下 80 行与当前真实声明命令树、`contract_test.go` 的固定 JSON leaf 清单双向校验。默认分支不传 `--state-dir`；显式非真实分支传临时隔离根；显式真实分支传注入账户的 `.agentdeck`。`$BASE` 是本次临时夹具根，`$EXT` 和 `$ALERT` 由真实扫描/刷新生成。JSON 字段合同来自现有固定 fixture；所有权错误、提示和 doctor 文案另与 `prototype/src/ownership.js` 的标本逐字/逐字段比较。

普通行使用实际 CLI 命令树的子进程；runtime 行使用真实入口加有限 context；session 交互行使用真实 PTY，画面出现后发送 q。价格更新采用已有的注入 HTTP transport，输入为有效 direct-provider 价格；客户端配置始终指向临时 HOME。extension enable/disable 的既定 `extension_read_only` 是有效对象上的适配器合同，不是缺参或非法输入。

| 入口与有效实参 | stdin / 特定数据 | 默认与显式非真实结果（各一次） | 显式真实结果 |
| --- | --- | --- | --- |
| `backup create $BASE/new.adb` | `passphrase\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `backup inspect $BASE/input.adb` | 真实加密的合成备份；`passphrase\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `backup list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `backup restore $BASE/input.adb` | 真实加密的合成备份；`passphrase\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `credential add p --credential second --endpoint https://example.invalid --clients codex` | `synthetic-secret\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `credential list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `credential remove p --credential second` | 临时 HOME / 状态夹具 | 0 / JSON 成功，data=null | 1 / state_ownership_refused |
| `credential show p` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `credential update p --rotate` | `replacement-secret\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop quota-alerts ack --id $ALERT` | 合成 quota refresh 返回的合法 alert ID | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop quota-refresh` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop quota-settings --reading on` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop quota-statusline disable` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop quota-statusline enable` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop refresh-indexes` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `desktop snapshot` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `doctor` | 临时 HOME / 状态夹具 | 0 / JSON 部分报告 state_missing | 0 / 部分报告 state_ownership_refused |
| `extension adopt $EXT` | 扫描后的临时 skill ID；需要时先 adopt | 0 / json 成功输出 | 1 / state_ownership_refused |
| `extension disable $EXT` | 扫描后的临时 skill ID；需要时先 adopt | 1 / `extension_read_only` | 1 / state_ownership_refused |
| `extension doctor` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `extension enable $EXT` | 扫描后的临时 skill ID；需要时先 adopt | 1 / `extension_read_only` | 1 / state_ownership_refused |
| `extension list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `extension release $EXT` | 扫描后的临时 skill ID；需要时先 adopt | 0 / JSON 成功，data=null | 1 / state_ownership_refused |
| `extension scan` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `extension show $EXT` | 扫描后的临时 skill ID；需要时先 adopt | 0 / json 成功输出 | 1 / state_ownership_refused |
| `price history` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `price list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `price override --file $BASE/prices.json` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `price status` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `price update` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider add new --endpoint https://example.invalid --clients codex` | `synthetic-secret\n` | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider current` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider recover` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider remove p` | 临时 HOME / 状态夹具 | 0 / JSON 成功，data=null | 1 / state_ownership_refused |
| `provider set-wrapper p --url https://wrapper.invalid` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider show p` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider status` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider update p --multiplier 1` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `provider use p --client codex --no-shell-setup` | 临时 HOME / 状态夹具 | 0 / JSON 成功，data=null | 1 / state_ownership_refused |
| `quota` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `quota capture` | `{"session_id":"ownership-session","model":{"id":"claude-sonnet-4"}}` | 0 / 静默 | 0 / 静默 |
| `run codex -- fixture` | 已选中的合成 provider；临时 codex 脚本 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `scan` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session exclude --kind session --value ownership-session` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session list` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session purge-index` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session rebuild` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session scan` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session search ownership` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `session show ownership-session` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `state migrate` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `telemetry config-preview` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `telemetry disable` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `telemetry enable --write-codex-config` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `telemetry import $BASE/otel.json` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `telemetry status` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage diagnose` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage hook event codex` | `{"hook_event_name":"SessionStart","session_id":"ownership-session"}` | 0 / 静默 | 0 / 静默 |
| `usage hook remove --client all` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage hook setup --client all` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage hook status --client all` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage rebuild` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage scan` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage sessions` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage signals --period all` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage stats --from 2026-07-14 --to 2026-07-20` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `usage summary` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `version` | 临时 HOME / 状态夹具 | 0 / json 成功输出 | 1 / state_ownership_refused |
| `watch --interval 20ms` | 临时 HOME / 状态夹具 | 0 / ndjson，实际入口运行后取消 context | 1 / state_ownership_refused |
| `completion zsh` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `shell-init zsh` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `shell env codex` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `shell setup zsh` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `shell status zsh` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `shell remove zsh` | 临时 HOME / 状态夹具 | 0 / text 成功输出 | 1 / state_ownership_refused |
| `session --interactive` | 临时 HOME / 状态夹具 | 0 / PTY 进入画面后 q 退出 | 1 / state_ownership_refused |
| `scan-worker` | 临时 HOME / 状态夹具 | 0 / text，实际入口运行后取消 context | 1 / state_ownership_refused |
| `desktop bootstrap` | 临时 HOME / 状态夹具 | 0 / ndjson 成功输出 | 1 / state_ownership_refused |
| `telemetry serve` | 临时 HOME / 状态夹具 | 0 / text，实际入口运行后取消 context | 1 / state_ownership_refused |

240 个分支均通过。Cobra 执行时另外加入的表面也在同一屏障下验证，未用固定 JSON 清单将其排除：

| 框架表面 | 默认 / 显式非真实 | 显式真实 | 输出依据 |
| --- | --- | --- | --- |
| `help provider` | 0 / Usage | 1 / state_ownership_refused | 实际 help 命令经过公共 pre-run |
| `--help` | 0 / Usage | 0 / Usage | Cobra 在 pre-run 前完成帮助 |
| `--version` | 0 / 固定版本文本 | 1 / state_ownership_refused | 根命令经过公共 pre-run |
| `__complete provider ""` | 0 / completion directive | 0 / completion directive | Cobra 内部补全协议 |
| `__completeNoDesc provider ""` | 0 / completion directive | 0 / completion directive | 同一内部协议的无说明别名 |

**访问屏障、输出与交错**

每条屏障观察之前都运行真实 Guard 拒绝的正向对照（exit 1），以及对受保护子项 stat 的负向对照（SIGKILL）；不以 mtime 不变替代无访问证据，不跳过不可用屏障。目录、根链接链、目标/后代、祖先链接、link/..、大小写、NFC/NFD 与 31 个非起始字符的原生等价组均通过。Task 1 的字面 U+034F 区分及缺失目标反例在 Go 全量中保持通过。默认 HOME 的 link/.. 和缺失分量后 .. 另有回归，物理解析前不做词法清理。

| 交错 | 观察结果 |
| --- | --- |
| I1 读写会话持锁后接管 | B 等待上限后 owner_running；回执逐字段不变；A 完成 |
| I2 排他先到、Hook 后到 | A 先观察旧身份并等共享锁；B 锁内换回执后，A 在屏障下静默 exit 0 |
| I3 只读会话持锁 | 与 I1 相同；只读池未提前放开交接 |
| I4 跨轮持锁 | B 被拒绝；A 两轮完成并退出后 B 成功；旧 A 再访问被屏障内拒绝 |
| I5 两轮之间释放 | B 成功接管；A 下一轮复核失败，callback 计数仍为 1 |
| I6 opt 切换、旧 keg 仍在 | B receipt_other_executable；A 现有会话正常完成 |
| I7 删除旧 keg 后续登 | A 活着持锁时 B ownership_busy，回执不变且无隔离目录回退；A 关闭/释放后 B 续登并读到原数据；A 残留进程下一次守卫 executable_unresolved |
| I8 默认续登与显式隔离 | 显式隔离不改变回执；不带 override 的 B 续登并读取 existing-owner-data |

A/B 为真实进程，使用真实 flock 及任务 1 的回执 API；不声称执行过尚未交付的安装命令。

固定输出覆盖 `notice`、`refused_state`、`revoked`、`doctor_not_installed`、`doctor_formula_unclaimed`、`doctor_refused`。TTY 文本只提示一次；非 TTY、quiet、JSON/NDJSON 无提示。拒绝报告不执行其他诊断，不写 Hook 拒绝记录。

**验证命令与范围**

环境为 macOS 26.7.3、Go 1.27.3（darwin/amd64）；Go 命令均使用规定 runner。访问屏障套件在无外层客户端沙箱的执行环境运行。

| 检查 | 结果 |
| --- | --- |
| `scripts/run-go-test.sh ./...` | 28 个包通过；触及状态的测试包使用隔离账户；既有手动性能场景的条件 skip 不计作本任务覆盖 |
| `scripts/run-go-test.sh -tags agentdeck_acceptance ./cmd/agentdeck -run '^TestOwnership'` | 80×3 命令分支、5×3 框架表面、62 个包入口、路径组、原型/TTY 和 8 种交错通过；本任务套件无 skip |
| `scripts/run-go-test.sh -race ./internal/platform ./internal/installation ./internal/store ./internal/credentialvault ./internal/scanruntime ./internal/telemetry ./internal/watch ./internal/desktop ./cmd/agentdeck-perf` | 相关并发包通过 |
| `make vet build-all`；`go build -mod=vendor ./cmd/agentdeck-perf` | vet、macOS arm64/amd64 CLI 和验证工具构建通过；输出使用临时目录 |
| `bash scripts/check-installation-resolver.sh` | 普通产物没有 acceptanceAccountHome 符号，带 tag 正向对照有该符号 |
| `scripts/check-docs.py` 的 `scoped_errors`（本次 Markdown 变更及其入链/主题集合）；`make check-whitespace`；`git diff --check` | 限定检查通过；混合改动聚合检查的既有问题见下文 |

本次构建的 SHA-256：arm64 CLI `09bd36d83773eef0249fe5f7ab9397275c8b2f0001bbbd75de5087c7f3c5e848`；amd64 CLI `2cd766d3623154bc08b493669db07dfda7c479994b63ce0002bc2b17f37886ae`；验证工具 `7231791d6ded148644075cb9a55672789f538d504f23fdbf9661862b4bee295d`。这些摘要标识本次验证产物，不是发布或安装声明。

**原生兼容性与已知基线限制**

当前四份桌面 fixture 的 installation 行经真实 `AgentDeckShared` 模块解码后，name/status/code/reason 分别保留为 installation/warning/not_installed/receipt_missing；第五份 legacy envelope 仍可解码。检查使用 `decodeDesktopWireEnvelopeV1`，逐份查找 installation 行并断言上述字段，输入是仓库中的五份真实 fixture，没有替换 schema 值。

额外运行完整 `AgentDeckFoundationVerifier` 时，当前候选和未修改 HEAD 的五份 fixture 都在 `schema version pair` 失败：验证器 `apps/macos/AgentDeckVerification/main.swift:126` 期待 supportedCount=32，HEAD 的 schema-ahead fixture 已是 33。本次没有修改原生源码或这个版本对。该既有验证器/fixture 不一致已记录在 Beads `ad-bug-native-verifier-schema-pair-stale`（待用户分流，未修复）；完整原生验证器不记为通过，也不代替 Task 6/7 的原生验收。

客户端真实配置写入保护、安装发布/卸载协议及 App 状态位置仍由任务 3–7 交付；A1/A2/A9 的全产品组合由任务 8 汇总。来源缺陷保持开放。本任务等待独立评审，尚未提交或推送。

文档聚合器另有既有范围限制：`python3 scripts/check-docs.py` 对本次混合改动报告一条历史 `health-recovery/reviews/extension-inventory-recovery.md:72` 的嵌套 `file://` 链接不支持。该行在基线 HEAD 已存在且未修改；`link_errors` 在解析 URL scheme 之前用变更目标的 basename 匹配 unsupported 行，因此 `extension.go` 命中了旧工作树的外部 URI。聚合检查其余已解析链接没有诊断。使用原检查器 API `scoped_errors([('M', None, 'docs/topics/installed-state-ownership/tasks.md')], repository_root, 'HEAD')` 验证本次文档、其入链和完整主题集合通过。聚合命令不记为通过；该限制由 Beads `ad-bug-doc-check-historical-file-url-scope` 承接，历史评审未改写。


#### 修复交接 — 2026-11-07

本次只补齐 worker 接收拒绝的跨 socket 类型传递。拒绝响应可携带
`ownership_refusal`，客户端恢复原有的 `resource`、`reason` 与 `revoked`；
普通拒绝仍保留原始错误字符串。状态目录解析、共享锁生命期、扫描分派和终态
`Result` 线路没有因本次修复改写。

新增真实 Unix socket 回归在客户端完成守卫后、worker 开始处理前切换身份，
覆盖三种原因、三个 scope 和普通/serving 两种读取方式共 18 例；每例同时核验
没有启动扫描、没有返回 round ID、receipt journal 字节保持不变。有效拥有者
完成扫描和普通拒绝保持非所有权错误作为对照。CLI 回归核验包装错误的错误码、
exit 1、仅含 resource/reason 的 JSON details，以及已批准的 revoked 文本。

候选 ContentState 使用 `installed-state-ownership:state-root-guard:repair-r1-f1-20261107-WxkADPnB`。
Go 1.27.3 的定向回归、全量 Go 回归、scanruntime 包竞态、`go vet` 与 macOS
双架构构建已通过；新增 CLI 回归在竞态检查中通过。CLI 全包竞态超时的限定
核验和既有聚合文档检查限制见评审记录，不将其记作全包通过。未受本次传输
修改影响的访问屏障、所有权会话、续登和八种交错按原观察作范围核验后复用。
完整修复映射、验证及门禁状态见
[任务评审](reviews/state-root-guard.md)。以上是修复交接时的验证边界；当前
独立复评结论以该记录的 Round 2 为准，交付状态以本文件 Tasks 矩阵及 Beads 为准。

### 3. `client-config-guard`

**依赖**：任务 2（传递包含任务 1）。**结果**：未安装进程不能写真实客户端配置（需求 G4，验收 A4）。任务 2 的错误类型与 CLI 映射在此复用并增加 `resource: client_config`。

**文件**：
- `internal/platform/state.go`：`GuardClientWrite`；
- `internal/provider/config.go`、`internal/usagehook/config.go`、`internal/telemetry/codexconfig.go`、`internal/shellconfig/config.go` 及对应测试；
- 如果重新枚举时发现其他写入者，也一并纳入。

**输入/输出接口**：消费身份/物理解析和状态守卫；产出 `GuardClientWrite` 及各客户端唯一写入汇合点的保护。实现时确认架构要求的“物理返回路径用于写入”如何贯通现有写入函数，不能只有一次检查却继续使用原始路径。

**实施顺序与验收**：
- 按[写入边界](architecture.md#真实客户端配置写入边界)在每个包唯一的写入入口加守卫。
- 重新枚举 `internal/` 和 `cmd/` 下所有写入账户主目录的函数，结论记录在评审材料中。
- 测试证明三点：目标位于注入的真实主目录时拒绝，且文件逐字节不变；目标位于临时 `HOME` 时允许；已安装身份下行为不变。
- 覆盖 provider 切换、`telemetry enable/disable --write-codex-config`、usage Hook 和 statusline 的 setup/restore、`shell setup/remove`。
- 默认隔离状态和显式非真实状态两组都覆盖“配置目标位于注入真实账户”：exit 1、`state_ownership_refused`、`resource: client_config`、真实配置逐字节不变；显式真实状态可能先被任务 2 拒绝，不能拿它冒充客户端守卫证据。模板、回滚、恢复、删除和临时文件写入都计入写入清单。
- `refused_client_config` 文本与 JSON 固定测试在本任务完成；安装身份夹具仍使用任务 1 的 API，无须等待任务 5。只读会话扫描输入仍允许读取。

**验证：L3**。因涉及真实客户端配置与凭据写入边界，运行 platform/provider/usagehook/telemetry/shellconfig/CLI 定向测试、Go 全量测试、`make vet`，并为改动中的并发写入路径选择对应 `-race` 包。记录客户端文件未变与状态访问屏障是不同的证明。

#### 开发交接 — 2026-11-08

本节是任务 3 的实现与验证材料，不是独立评审结论。Dev 已勾选，Review 保持未勾选。候选基线为 `2867c9c32fbd7349aff6cda81f90e2eb8ce81b4d`（任务 2 交付提交）；工作区 `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。CEv1 WorkUnit 为 `installed-state-ownership:client-config-guard`，候选 ContentState 记录最终 HEAD、逐路径 blob 清单与指纹（配方同任务 2）。

**守卫语义**

- 新增 `platform.GuardClientWrite(path) (string, error)`（`internal/platform/client_guard.go`）。受保护集合是 `phys(account)`：逻辑账户主目录、其入口链接链和物理目标，每次调用只读入口元数据重新计算。候选路径按完整物理解析判定，位于该集合下时：已安装身份放行；未安装身份（含 formula 续登失败）返回 `ErrStateOwnershipRefused{Resource: "client_config"}`，reason 取身份原因。临时 `HOME` 位于集合外，直接放行。
- 返回值是“物理父目录 + 末级名称”。所有写入者都替换或删除目录项本身，shell 与 Hook 写入者还拒绝末级符号链接；解析父目录可把写入固定在被判定的位置，又不把“替换链接”变成“经链接写入”。末级链接指向账户内时仍按完整解析拒绝。
- 拒绝错误按 resource 选择文案：`client_config` 使用原型消息，文本块的 `next` 为客户端配置专用一行，不输出状态目录的 manual prerequisite。JSON details 仍只含 `resource`、`reason`。

**写入者与守卫点**

| 包 | 守卫点 | 使用返回路径 |
| --- | --- | --- |
| provider | `atomicPrivateReplace`（四个 `Write*Config` 唯一汇合点）；`Service.UseCredential`（`Use` 经由它）在备份与操作记录之前预检主配置与调用方指定的备份目的地；公开的 `WriteRedactedBackup` 守卫自身目的地 | stat、临时文件目录、重命名、chmod 均用返回路径；拒绝时不写备份、不建操作记录。托管备份由 `managedBackupPath` 落在已过状态守卫的根下，走包内未守卫的 `writeRedactedBackup`；自定义目的地一律按客户端写入判定（ISO-CCG-R1-F1） |
| usagehook | `writeAtomicChecked`、`removePath`（全部 Hook/statusline 写入与回滚删除）；`mutate` 与四个 statusline 公开方法先预检，使拒绝以错误返回而非结果行 | 目录创建、临时文件、重命名、删除均用返回路径；statusline prior 记录写在状态目录，改走不带客户端守卫的内部写入，由状态守卫负责 |
| telemetry | `WriteManagedBlock` 在写摘要记录与建目录前；`RemoveManagedBlock` 在读到记录后、替换前 | 读取、恢复、替换均用返回路径；CLI 的 enable 分支透传拒绝，不再折叠为 `codex_config_write_failed` |
| shellconfig | `Setup`、`SetupIfUnconfigured`、`Remove` 在任何目录、临时文件、备份、回滚前对全部选中目标预检 | 写入与回滚使用物理路径；结果行保留请求路径用于显示 |

**写入者重新枚举**：对 `internal/` 与 `cmd/` 中全部 `os.WriteFile/Create/CreateTemp/OpenFile/Rename/Remove/RemoveAll/MkdirAll/Chmod/Symlink` 调用逐个核对目标。账户主目录下的写入者只有上表四个包。其余写入目标均为状态目录或其派生位置，由任务 2 的状态守卫覆盖：`backup`、`credentialvault`、`desktop/derived_cache`、`hookrefusal`、`provider/project_gate` 与 provider 托管备份（调用方指定的备份目的地属于客户端写入，见上表 provider 行）、`telemetry` 的 config/token/journal/listener、`perftrace`（调用方指定的跟踪文件）、`cmd/agentdeck` 的 `purge-index`；`cmd/agentdeck-perf` 只写调用方指定的语料/工作目录。`quota capture` 只读取 prior statusline 命令。`extension` 没有直接文件写入。没有发现清单外的客户端写入者。

**测试**

- platform：账户内路径、缺失父目录、外部末级链接指向账户、经账户别名入口的路径和账户根本身均拒绝且不创建任何项；临时 HOME 经链接时返回物理父目录且不跟随末级链接；已安装放行；账户主目录不可解析时 `real_root_unresolved` 失败关闭。
- provider/usagehook/telemetry/shellconfig：注入临时账户后，各写入入口与包内汇合点在账户内拒绝，账户树逐字节不变且无临时文件、备份、目录残留；provider 拒绝时零操作记录；自定义备份目的地（账户内目标、父目录别名、末级链接）经直接 API 与 `Service.Use` 均被拒绝且零操作记录，临时目的地与已安装身份照常写入；telemetry 拒绝时摘要记录不写、已有记录不丢；临时 HOME 与已安装身份下 setup/restore/remove 照常完成。原有 shellconfig 故障注入测试改用物理临时目录，以匹配写入者使用的物理路径。
- CLI（`agentdeck_acceptance`，屏障与正负对照下运行）：默认隔离状态与显式非真实状态两组，各覆盖 `provider use`（codex/claude）、`telemetry enable --write-codex-config`、`telemetry disable`（账户内放置已登记块）、`usage hook setup/remove`、`desktop quota-statusline enable/disable`、`shell setup/remove`，共 20 例：exit 1、`state_ownership_refused`、`resource: client_config`、`reason: receipt_missing`，账户客户端文件树逐字节不变。`refused_client_config` 文本与 JSON 与原型逐字一致（固定测试数据由原型生成）；同一未安装构建对临时 HOME 的写入成功。显式真实状态目录的拒绝由任务 2 证明，未计入本任务证据。客户端文件未变与状态访问屏障是两种证明：屏障只覆盖真实状态目录，客户端拒绝以文件树比较证明。

**验证（L3）**：`scripts/run-go-test.sh ./...` 全部通过；`scripts/run-go-test.sh -race ./internal/platform ./internal/provider ./internal/usagehook ./internal/telemetry ./internal/shellconfig` 通过、无竞态报告；`GOFLAGS=-tags=agentdeck_acceptance scripts/run-go-test.sh ./cmd/agentdeck ./internal/platform ./internal/installation ./internal/ownershiptest` 通过（含任务 2 的叶子矩阵与新增客户端配置用例，屏障在无外层沙箱的环境中运行）；`make vet` 通过。工具链 Go 1.27.3 darwin/amd64，vendor 模式。

上述开发交接保留实现时的观察。任务 3 已通过 Round 4 独立复评并待授权提交，评审结果与证据门禁见[任务评审](reviews/client-config-guard.md)。安装协议、诊断与 App 仍由任务 4–7 交付，跨面组合归任务 8。

### 4. `installation-diagnostics`

**依赖**：任务 2。**结果**：在已有基础安装诊断上增加受管命令归属，App 能从 helper 获得状态目录（需求 G6、G8，验收 A3）。

**文件**：
- `internal/doctor/doctor.go` 及测试；
- `cmd/agentdeck/main.go` 的 doctor 渲染及固定测试：补完整报告中的受管命令检查；
- `internal/desktop/desktop.go` 及测试，确认检查能传到 health 部分；
- `cmd/agentdeck/desktop.go` 及测试：新增隐藏命令 `desktop state-location`；
- `internal/usagehook` 中只读的受管命令枚举，如需导出；
- JSON 固定测试。

**输入/输出接口**：消费任务 2 的已守卫路径及基础 doctor 报告；产出 `managed_command_owner` 检查和隐藏的 `desktop state-location` JSON 契约供任务 6 消费。

**实施顺序与验收**：
- 只读枚举受管 Hook/statusline 命令，在已安装且未因状态短路时执行 `managed_command_owner`。字段与 [CLI 契约](ux/cli.md#managed_command_owner-检查) 一致：匹配、PATH-only、不同绝对拥有者三类，JSON 不包含路径；不同拥有者给出重新登记命令。测试配置用固定夹具，证明的是枚举和诊断，不执行尚未实现的安装命令。
- 任务 2 已交付的 `installation` 首行、缺失/拒绝部分报告及 Hook 拒绝不写记录继续保持；新增诊断不能穿透这两条短路。五个 `doctor_*` 标本作为完整固定测试集，在本任务补齐完整报告并回归部分报告。
- `desktop state-location` 的输出按[架构](architecture.md#app-与小组件)定义；不是拥有者时，绝不返回真实路径。
- 固定测试覆盖：已安装和未安装两种身份的 doctor 文本与 JSON、部分报告、`managed_command_owner` 的三种情况，以及 state-location 的输出。

安装拒绝不写入 `hook-refusals.json`；schema-version-signal 的现有测试保持不变。

**验证：L2**，因为改的是线路和文本契约。需要 targeted 测试和 Go 全量测试。

#### 开发交接 — 2026-11-08

**实现**：
- `internal/usagehook/managed.go`：新增只读的 `Manager.ManagedCommands()`，读取 `~/.codex/hooks.json` 与 `~/.claude/settings.json` 中 AgentDeck 生成的 Hook 命令（`<程序> [--state-dir <dir>] usage hook event <client>`）和 Claude `statusLine`（`<程序> [--state-dir <dir>] quota capture`）。程序可以是 PATH 上的 `agentdeck`，也可以是基名为 `agentdeck` 的绝对路径（含 `shellQuote` 编码）。它不建目录、不写文件；文件缺失、无法读取或格式无效时不贡献命令。
- `internal/doctor/managed_command.go`：`managed_command_owner` 只在已安装身份下运行，位置紧跟 `hook_deliveries`，在状态缺失或被拒绝的短路之后，因此两类部分报告都不包含它。绝对路径命令经 `EvalSymlinks` 与当前程序真实路径比较；程序不存在也算不同拥有者。判定规则：没有不匹配、且存在绝对路径命令或没有受管命令时为 `ok`；只有 PATH 命令时为 `ok` 并带 `reason: path_resolved`；存在不匹配时为 `warning (managed_command_owner_mismatch)`，`reason` 是受影响的客户端（按 `claude,codex` 顺序，逗号分隔）。`recovery_command` 规则：两个客户端的 Hook 都不匹配时为 `agentdeck usage hook setup --client all`；只有一个客户端时为对应的 `--client <client>`；只有 statusline 不匹配时为 `agentdeck desktop quota-statusline enable`。文本附加行是 `reason` 和 `next`，`recovery` 由现有通用分支输出。JSON 只用现有 `Check` 字段，不含路径。
- `cmd/agentdeck/desktop_state_location.go`：新增隐藏命令 `desktop state-location`，只接受 `--format json`。数据为 `installed`、`channel`（已安装时）或 `reason`（未安装时），以及 `state_root`。状态目录经守卫：显式目录被拒绝时改报隔离默认根，不报错，也不给出真实路径。公共 pre-run 对这条命令放行拒绝，与 doctor 相同。
- 通过 desktop 健康部分，这项检查进入 snapshot。合成语料的 snapshot 参考文件只多了一项 `managed_command_owner: ok`，已重新生成。

**设计判断（供评审核对）**：CLI 契约只给出单客户端 Hook 的标本。双客户端时 `reason` 用逗号连接；statusline 不匹配时以 `desktop quota-statusline enable` 作为重新登记命令；`next` 文案按客户端拼接，例如 “Claude and Codex hooks point to another AgentDeck program and are ignored.”。当前 Hook 写入者只登记 PATH 形式；绝对路径 Hook 只可能来自旧拥有者，枚举仍覆盖这种形式。

**测试**：
- `internal/usagehook/managed_test.go`：枚举 PATH、带引号的绝对路径、`--state-dir`、statusline 和非受管命令；读取后配置字节不变；缺失或无效配置不贡献命令，也不建目录。
- `internal/doctor/managed_command_test.go`：覆盖无命令、拥有者匹配、仅 PATH、Claude 不匹配、双客户端不匹配、statusline 不匹配、程序已不存在七种情况，并确认结果不含路径。另验证：已安装身份下状态缺失的部分报告不运行此检查，可读状态下运行，未安装身份下不运行。
- `cmd/agentdeck/ownership_doctor_render_test.go`：将 `doctor_installed`、`doctor_command_owner` 标本的检查送入 doctor 写出器，文本和 JSON 与原型逐字一致。两份固定数据由 `prototype/src/ownership.js` 生成，加入 `ownership-golden.json`，键序与原型一致。其余三个 `doctor_*` 标本由任务 2 的验收测试继续回归。
- `cmd/agentdeck/desktop_state_location_test.go`：已安装身份下，显式目录和默认目录各一例；未安装身份下，默认返回隔离根；显式指向真实根时返回隔离根，输出不含账户路径，真实目录也未被创建；文本格式被拒绝。
- 任务 2 叶子矩阵（`agentdeck_acceptance`）登记 `desktop.state-location`。default 和 explicit-isolated 两个分支按普通命令断言；explicit-real 分支断言 exit 0、`installed:false`，且输出不含真实账户路径。

**验证（L2）**：
- `scripts/run-go-test.sh ./...` 通过。`make vet` 通过。对新增和改动的 Go 文件跑 `gofmt -l`，输出为空。
- `GOFLAGS=-tags=agentdeck_acceptance scripts/run-go-test.sh ./cmd/agentdeck ./internal/platform ./internal/installation ./internal/ownershiptest`：`TestOwnershipLeafInventory` 和 `desktop.state-location` 的三个分支均通过。
- 已知不稳定：叶子矩阵的 `watch` 行在 default 或 explicit-isolated 分支偶发 `snapshot session: context deadline exceeded`。在 `-count=3` 定向重跑中，6 次 watch 运行失败 2 次；同一套验收的第一次完整运行中 watch 通过。原因是验收子进程使用 1 秒 context（`ownership_guard_acceptance_test.go:75`），session 快照在负载下超时。`internal/watch` 不依赖本任务改动的 doctor、usagehook 和 desktop，因此这不是本任务引入的问题，也不在本任务范围内修复。

### 5. `installer-receipts`

**依赖**：任务 2（传递包含任务 1）。**结果**：四种安装方式都通过唯一的用户回执取得所有权，并且只有在旧拥有者停止之后才允许接管（需求 G7、G9，验收 A2、A6、A7、A9 的安装侧）。

**文件**：
- `packaging/homebrew/agentdeck-app.rb.tmpl`、`packaging/homebrew/agentdeck.rb.tmpl`（formula 只改 caveats，不写回执）；
- `scripts/render-homebrew-cask.sh` 和 `scripts/render-homebrew-formula.sh`（如需修改）；
- `scripts/manage-install.sh`；
- 新建 `scripts/install-local-app.sh`；
- `Makefile`：新增 `install-macos-app` 及对应的卸载目标，增加 `TAKEOVER`；
- 新建 `cmd/agentdeck/install.go` 及测试：公开命令 `install claim`；隐藏命令 `install repair-identity`（由原 App helper 持锁验证有效 cask 回执及当前程序，输出原 installation ID，不依赖清理材料且不写入）、`install register`（带 `--source` 和 `--cleanup-manifest` 时在锁内发布；cask 带 `--require-unowned`，授权修复另带 `--authorized-repair`，修复有效原安装还须传预先捕获的 `--expected-installation`）、`install unregister`（携带 channel、路径、原 ID 和清单，`--remove` 仅锁内条件删除；授权容忍材料缺失时不删除，结果 `skipped_unverified`；绑定模式由 attempt 层传递内部 `--require-identity`，在清理锁内核对原身份，身份不匹配或无法验证仍非零失败）；
- `internal/installation`：在所有权排他锁内写入和删除回执、生成安装 ID、写本次清理清单并校验清理归属；`resolver_acceptance.go`（仅验收 tag）在 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 未设置时读取 `HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME`；
- `scripts/test-install.sh`、`scripts/test-completion-install.sh`、`scripts/test-cask-migration.sh` 及新增的 `scripts/test-install-local-app.sh`：凡是会安装的测试都改用验收版程序和临时账户目录；
- 新建 `scripts/test-cask-real-brew.sh` 和 `.github/workflows/cask-real-brew.yml`：真实 brew 回归，只在 CI 运行；
- `scripts/verify-release-artifacts.sh`：删除对发布产物的安装冒烟，因为发布产物会把回执写进运行者的真实账户；`scripts/test-release-distribution.sh` 检查 formula 的认领提示。

**输入/输出接口**：消费任务 1 的回执/排他锁及任务 2 的真实状态共享会话；产出 `install claim/register/unregister`、清理清单、安装脚本和完整安装器测试。本任务用 CLI 会话证明 `owner_running`，不要求尚未完成的原生 App；App 与安装器的组合归任务 8。

**实施顺序**：先完成 helper 安装协议及失败注入，再接入 make/local App/cask/formula，然后运行下表交错测试，最后由 CI 执行 cask 的真实 Homebrew 完整分派。脚本不能先上线一个锁外发布版本等待后续修复。

**验收要求**：
- 按[安装回执](architecture.md#安装回执)实现每种方式的写入、拒绝冲突和卸载清理。cask、make、本地 App 调用暂存新程序的 `install register --source` 或保留 helper 的 `install unregister --remove`；formula 保持由用户 claim、升级按任务 2 续登、删除 keg 后回执过期的独立契约。安装脚本自己既不写回执，也不把文件移到最终位置，也不写任何 keg 回执。
- 按 [formula 认领](architecture.md#formula-认领)实现 `install claim`：只能认领当前 opt 指向的 keg；回执登记 keg 内的真实路径，**不登记 opt 链接**；文本和 JSON 与原型中 `claim_*` 四个状态逐字一致。测试覆盖：旧 keg、非 opt 指向的 keg、两个前缀、stable 与 rc、`--takeover` 的成功和拒绝。这些测试都放在临时目录搭建的 Cellar/opt 布局中进行。
- 接管只以[所有权排他锁](architecture.md#所有权锁与交接)作为同步边界，不使用 `lsof`。测试中让一个替身进程持有共享锁来模拟旧拥有者的会话，断言 `TAKEOVER=1`、`install-macos-app` 和 `install claim --takeover` 都以 `owner_running` 拒绝，回执不变；替身释放锁之后，接管成功。
- **在锁内发布**：按[安装回执](architecture.md#安装回执)，由 `install register --source` 在一次持锁内完成原子发布和回执写入。单个文件用 `rename(2)`；App bundle 用 `renamex_np(RENAME_SWAP)` 交换，再删除旧 bundle；任何失败都不改目标路径和回执。`manage-install.sh` 原有的暂存、校验和清单逻辑保留，只把最后的重命名交给这条命令。测试中，替身进程持有共享锁时，发布以 `owner_running` 失败，目标和回执都不动；替身释放锁之后，一次完成。在发布与写回执之间注入失败，断言目标路径仍为原内容、回执未变。
- 回执写入前，要确认 `executable` 满足 `EvalSymlinks(executable) == executable`，并且就是正在安装或认领的程序；写入要原子化，权限为 `0600`。
- **完整清理链**：按[卸载与失败清理](architecture.md#卸载与失败清理)，正常卸载、升级卸载、安装失败回滚和旧清理重试都经过带原清单 ID 的 `unregister`；材料单独缺失的恢复走下文的身份绑定替换，不伪造原清单或先删回执。清单在发布前由 helper 独占创建并同步落盘，位于最终目标之外；相同版本重装生成新 ID/新清单，旧清单不能被覆盖或从当前回执刷新。测试回执匹配时清理、路径/channel/ID 任一不匹配时跳过、回执和目标均不存在时幂等成功、回执缺失/损坏而目标存在时保留并报错。helper 缺失时使用私有保留副本；副本/清单不可用必须报告未完成，不能直接 `rm`。小组件注销也必须在归属匹配之后、同一次持锁期间执行。
- **回滚与失败注入**：按架构分别实现发布块内部 rescue、后续 artifact 的 `uninstall_preflight` 回调和经典卸载脚本。回调捕获本次不可变 attempt，不重新读取可变定位文件或当前回执。覆盖发布后标记/暂存清理失败、binary 失败、前序回滚 artifact 抛错、强杀与 `--force` 跳过脚本；不可达或失败的清理必须保留 Caskroom 外的恢复材料并报告未完成。
- **cask 发布与材料**：formula 冲突仍在 `preflight_steps` 检查；经典 Ruby `preflight` 发布，经典 `uninstall_preflight` 只在本次安装闭包中有 attempt 时回滚；正常卸载经 `quit` 后的经典 script 条件清理。不使用 `app` 标签或路径式小组件注销。经典 flight DSL 已弃用，CI 加载或分派不支持即阻塞，不修改 Homebrew 或静默换入口。
- **每次尝试隔离**：实现架构规定的外部 `attempts/<ID>` 目录、不可变清单/helper、外部恢复索引和 `.agentdeck-attempt` 定位文件。目录独占创建并拒绝符号链接；材料在发布前落盘。普通安装、同版本重装、旧版恢复均分配新 ID 和路径；保留旧材料，不自动 GC。隐藏 register 增加 `--require-unowned`，在所有权锁内要求“没有有效拥有者（回执缺失、无效或过期都算无主）且目标不存在”，防止恢复旧版覆盖竞争者或有效拥有者；保持其他安装方式原有覆盖规则。
- **无主判定与授权修复**：按[无主判定与授权修复](architecture.md#cask-无主判定与授权修复)实现，不能由实现者自行选择恢复语义。`HOMEBREW_AGENTDECK_REPAIR=1` 授权无主容忍/替换；有效原安装另须一次捕获的 `HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION`。模板各入口读取命令继承的固定值，转成显式 `--authorized-repair`/`--expected-installation`，helper 不读环境，不允许刷新 ID 或在失败后删去预期 ID 降级。无绑定时仍拒绝全部有效拥有者；有绑定只允许锁内 ID/channel/规范 executable 全匹配的原 cask 安装，不是同路径覆盖权。材料缺失的 unregister 只容忍不删除；有绑定但身份不匹配/无法验证时非零拒绝。回滚/rescue 只用新 attempt 的清单 ID，永不带授权参数。失败提示按无主修复或身份绑定修复给出带固定 helper/token 的完整命令，身份捕获失败不执行 brew；`doctor` 和状态拒绝文案不变。
- **helper 层的授权矩阵**（开发机、验收版 helper、临时账户目录，不执行暂存 helper）：register 判定表每一行分别验证无授权、仅授权、授权加预期 ID，检查目标/回执字节及退出原因。有效拥有者覆盖相同 channel/路径、不同 channel、不同路径；只有精确绑定原 cask ID 才可替换，空值/格式无效 ID、单独传 ID、其他 ID 均拒绝。过期回执加空目标发布成功，无主现存目标仅在无绑定授权时可替换；绑定模式无法证明原身份则拒绝。授权替换在发布后、写回执前失败恢复原对象/回执，提交后才删旧对象。原 unregister 的正常清理、`skipped_superseded`、`owner_running`、清单不一致和删除失败语义保留，绑定修复遇到后继身份另须非零拒绝。
- **有效原回执、材料单独丢失**（ISO-TASKS-R13-F1）：分别仅删除清单、仅删除保留 helper 副本、仅删除 `.agentdeck-attempt`，保持原回执及 App helper 有效。每例通过真实 `repair-identity` → 授权 unregister → register 验证：无授权拒绝；只有授权但无预期 ID 不可覆盖原拥有者；一次捕获 ID 后，材料缺失卸载仅报告并保留 App/回执，发布持锁匹配后生成全新 attempt/清单/ID，剩余旧材料逐字节不变，随后普通无授权卸载成功。身份捕获不依赖三类材料、无任何写入；App helper 也不可用时报告不可恢复边界，不用 shell 模拟校验。用受控钩子分别在身份捕获后、卸载容忍后且发布前安装同 channel/路径/版本的 B，原绑定调用非零拒绝，B App/回执不变；再测 B 回执缺失/损坏仍保留 B、不降级。修复成功后延迟原 ID 的清理不得删除新实例；在交换后、回执提交前失败恢复原 App/回执。helper 层实际命令和完整 brew 链分别验证，不能把若干独立拒绝断言当作恢复成功。
- **开发机上的 cask 检查**（`scripts/test-cask-migration.sh`，不执行 Homebrew 暂存的 helper）：渲染声明满足上述约束；按声明用临时账户直接调用验收 helper，覆盖材料/定位校验、缺失材料拒绝、条件卸载、无主安装锁内检查；用真实 Homebrew 方法和无副作用 artifact 替身验证回滚分派。模板及渲染后的所有清理路径无最终 App 或外部材料根的 `delete`、`trash`、`rmdir` 或 shell 兜底，binary 只删除链接。渲染后的声明另做静态检查：`--authorized-repair` 只由 `HOMEBREW_AGENTDECK_REPAIR` 决定，回滚块和 rescue 的清理调用不带它；没有 `writable_paths`/`writable_base`；没有由 `HOME`、`~` 或 `Dir.home` 推导的 AgentDeck 路径（材料和索引路径只来自 helper）。
- **CI 的真实 brew 回归**（`scripts/test-cask-real-brew.sh`，由 `.github/workflows/cask-real-brew.yml` 在 GitHub 托管的 macOS runner 上运行）。脚本没有 CI 专用开关时拒绝运行，避免在开发机上触发 Gatekeeper 弹窗。prefix、Caskroom、cache、`HOME`、`appdir` 和账户目录都是临时目录；账户目录通过 `HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 指向一个与 `HOME` 不同的独立临时目录，以证明模板不依赖 `HOME`。被安装的 App 内是验收版 helper，运行前用符号检查确认。以下各点都必须通过，任一项失败，都停下来回到设计重新决定，不能静默改用别的方式：
  - 可行性探测：验收版 helper 能在 cask 步骤中执行，未被终止；
  - 经典 flight stanza 可加载；preflight/helper 仅写临时账户的回执、外部尝试目录及 `appdir`。权限边界与[风险](architecture.md#风险)中的账户解析一行一致：它不依赖 `writable_paths`，`HOME` 与账户目录不同时，回执、清单、材料和索引只在账户目录，`HOME` 下没有；账户目录不可写时，安装非零退出、没有回执和已发布的 App、没有回退写入；
  - 不带 `app` 标签的 cask 能正常安装、升级、卸载，`binary` 和补全链接照旧；
  - 升级时旧版本的卸载先于新版本的 preflight 执行；同版本重装时，新清单路径不与旧清单冲突；
  - 完整执行“旧 O 卸载 → 新 N 发布 → 后续失败 → N 清理 → 旧版恢复 R”，无竞争者时旧版恢复成功，R 使用全新清单/ID，定位文件指向 R，O/N 清单与 helper 不变；加入同 channel 竞争者 B 时 R 在锁内拒绝，B 的 App/回执逐字节不变；延迟 O/N 清理不能删除 R/B；
  - `test-cask-widget-registration.rb` 的小组件注册仍然有效，卸载时在锁内注销成功；
  - 真正卸载后不留回执，其他安装方式随后可以正常登记，不需要 `--takeover`；
  - 沿真实 Homebrew 完整分派执行下表 I4，不能在 helper 返回时就停止测试；观察所有后续文件操作，确认新 App 和回执保留，旧 ID 的延迟回滚不会删除同 channel 的新安装；
  - 分别在发布成功但 preflight 尚未返回时（标记写入、暂存清理）和后续 binary 链接处注入失败，断言原 attempt 条件清理可达、brew 非零退出；即使回滚中断，purge 后外部清单/helper/索引仍可恢复；破坏最终 App 内 helper 后仍可在锁内正常清理；缺少私有副本或清单时，即使 Homebrew 强制跳过脚本也不删除最终 App，验收报告残余而非卸载成功；
  - 损坏安装的授权修复：两种损坏——删除回执、把回执改成无效内容——各做两轮，第一轮外部材料完整，第二轮同时删除外部材料。每一轮里，不带 `HOMEBREW_AGENTDECK_REPAIR=1` 的 `brew reinstall --cask` 非零退出，App、Caskroom 记录和仍在的外部材料逐字节不变，失败输出含带本 cask token 的完整修复命令；带该变量重跑成功，生成新的 attempt、清单和 installation ID，仍在的旧材料不变，App 被替换，小组件注册有效，随后不带变量的 `brew uninstall --cask` 成功清理。再用“目标处有无回执的残留 App，且 Homebrew 不记录该 cask”的状态验证 `brew install --cask`：不带变量被拒绝、目标不变；带变量替换成功；
  - 有效原回执、仅清理材料丢失（ISO-TASKS-R13-F1）：三轮分别只删清单、只删保留 helper 副本、只删定位文件，保持 App helper 和回执有效。各轮无授权重装非零拒绝；仅授权变量但不绑定原 ID 时也不能替换有效安装。用原 App helper 实际捕获 ID，执行带两个变量的完整 `brew reinstall --cask`，确认卸载容忍阶段未删除 App/回执，随后发布产生新 attempt/清单/ID，仍在的旧材料不变，小组件注册有效，再不带变量普通卸载成功。不得复用“损坏回执且同时删材料”的用例冒充此状态；
  - 绑定修复的竞争者交错：对上述三轮各在捕获原 ID 后、卸载容忍后且发布前暂停，再通过真实安装协议放入同 channel/路径/版本的 B，恢复完整 brew 分派，原修复须非零拒绝，B App/回执逐字节不变；另测 B 回执缺失/损坏仍拒绝、不删预期 ID 后重试。无竞争者的修复在交换后、提交回执前失败时恢复原 App/回执；成功后运行原 ID 的延迟清理，修复后的新安装保持完整。断言预期 ID 始终为捕获的原值，不因重新求值或旧版恢复而刷新；
  - 授权不越过其他有效拥有者：先放入有效拥有者（make 与 local_app 各自的回执加对应对象，以及仍存在的 formula keg 的回执），仅带授权变量或另带不匹配预期 ID 的 `brew install --cask` 都以 `owned_by_other` 失败，拥有者的对象和回执逐字节不变，输出含迁移命令；
  - 过期 formula 回执迁移：回执登记一个已被删除的 keg 路径，不带任何变量的 `brew install --cask` 成功，回执改为 cask，没有使用 `--takeover`；keg 仍在时同一命令以 `owned_by_other` 失败并打印迁移命令，目标不存在、回执不变；
  - 恢复旧版 R 仍然严格：目标处有回执丢失的竞争者对象，且没有设置该变量时，R 以 `receipt_missing` 拒绝，竞争者对象不变。
- 本地 App 安装沿用 ad-hoc 签名，不加 hardened runtime，并执行 `lsregister`。
- 所有测试都在临时 `HOME`、`PREFIX`、`APPDIR` 和临时账户目录下进行；会安装的测试脚本在安装前确认被安装的程序是验收版，不是就中止。

**安装协议交错表**：I1–I4 承接旧任务 2 第 8–10、12 条，I5 补齐本次身份绑定恢复。它们必须运行本任务的真实安装 helper 代码与命令，使用受控同步钩子，不能仅调用锁 API 或跑协议模型。

| 编号 | 受控顺序 | 必须观察到的结果 |
| --- | --- | --- |
| I1 发布与登记 | A 的任务 2 真实读写会话持共享锁；安装方执行 `register --source`。释放 A 后重试，在发布后、写回执前暂停 | 首次 `owner_running`，目标与回执不变；重试在一次排他锁内发布/登记，中间任何共享会话都等待；失败注入恢复原内容与回执 |
| I2 同路径竞争 | A/B 同 channel/目标、各自暂存源及新清单；暂停 A 的锁内发布，启动 B；另测共用同一清单和不同 channel | B 在取排他锁处等待，等待期间不读写安装对象；A 完成发布和登记、释放锁后，B 才在自己的排他锁内看到 A 的正式回执并覆盖；从无“改目标时还有另一方会话”；同清单独占创建失败且不改原清单/目标；不同 channel 无 takeover 时 `owned_by_other`，且目标 P 与原回执逐字节不变 |
| I3 暂停与崩溃 | A 锁内 SIGSTOP，测试时钟推进超过旧方案的 601 秒；B 登记和访问同时尝试；再测 A 取锁前暂停、恢复，以及发布进程崩溃 | 锁内暂停时 B 的登记/访问分别 `owner_running`/`ownership_busy`，没有超时租约放行，暂停期间任何一方都不改 P、不读取发布中间状态；A 恢复后在同一次连续持锁内完成发布和登记。锁外暂停时 B 正常完成登记并开启会话；A 恢复后先取排他锁，等待 B 会话结束后成功，或达到上限以 `owner_running` 失败，等待/失败期间 P 和回执不变。崩溃由内核释放锁，目标是旧/新完整内容之一，B 随后可正常登记 |
| I4 延迟完整清理 | A helper 锁内清理后返回；B 发布同路径并开启真实共享会话；继续 A 的全部卸载/失败回滚/重试。helper 层在本地验收测试中运行；包括 Homebrew 后续分派的完整链在 CI 的真实 brew 回归中运行 | B App/回执逐字节不变、会话有效；A 无锁外最终删除；条件清理在 B 持锁时超时，释放后因原 ID 不匹配跳过；B 分别为 local_app 与同 channel、同版本 homebrew_cask 重装 |
| I5 绑定修复与后继 | A 回执及 App helper 有效，三类清理材料分别单独丢失；捕获 A 的 ID，分别在捕获后、卸载容忍后且 register 前插入同 channel/路径/版本 B；另测 B 回执再缺失/损坏，以及无竞争者的完整恢复/普通卸载 | 预期 ID 始终为 A，不能读取 B 的 ID 刷新；有后继时非零拒绝，B App/回执逐字节不变，材料缺失不跳过核对或降级。无竞争者时生成新 ID/材料并可普通卸载；交换后提交前失败恢复 A；A 的延迟清理不删除修复后的新实例。本地 helper 链与 CI 真实 brew 链均覆盖 |

**验证：L3**。本地：安装 helper/CLI/installation 的定向与 Go 全量测试、验收 tag 下的交错与叶子矩阵测试、相关并发包 `-race`、`make vet`、`make check-install`（含本地 App 安装测试）、`scripts/test-cask-migration.sh`、`scripts/test-release-distribution.sh` 与模板检查。CI：`cask-real-brew` 回归。先检查现有脚本的隔离方式，再选命令；触及系统 tap、签名、注册等真实环境的操作必须另外获得授权，不能由测试名推定安全；开发机上不运行会执行 Homebrew 暂存 helper 的步骤。真实 brew 的已批准可行性条件任一失败，保持本任务未完成并返回相应设计决策，不把失败转交任务 8。

**证据时序例外（2026-11-09 用户决定，仅限本任务的 `real-brew` 准则）**：CI 只能运行已推送的提交，所以 CEv1 的 `real-brew` 准则只能在提交并推送之后取得。处理顺序如下：

1. 独立评审依据本地证据给出结论；其余准则照常在提交前通过门禁。
2. 评审 PASS 后，Beads 进入 `awaiting_commit` 不以 `real-brew` 为前提，这是唯一的例外。
3. 用户另行授权提交并推送 topic 分支后，以 CI 回归结果作为 `real-brew` 证据，绑定到该提交的树。CI 运行记录会过期，证据直接写下检查事实、提交和结果。
4. 门禁 VERIFIED 后任务关闭。CI 失败时任务回到 `in_progress`，修复、复评，再由用户另行授权新的提交和推送。

评审 PASS 不代表 `real-brew` 已满足；提交和推送仍须用户逐次授权。


#### 开发交接（2026-11-10）

本地实现已包含安装 helper、make/local App/cask 接入、身份绑定修复与 attempt 材料。接续开发修复了非法修复参数在 cask 创建暂存副本后才被拒绝导致的残留，补齐 `repair-identity` 和三个 `attempt` 命令的访问屏障矩阵，并增加 `scripts/test-cask-real-brew.sh` 与 `.github/workflows/cask-real-brew.yml`。CI 脚本仅允许带 `--ci` 的 GitHub 托管 macOS runner 运行，执行真实 Homebrew 安装、升级恢复、材料缺失修复、竞争者交错、强杀和延迟清理。

本地证据使用 Go 1.27.3。全量 Go 1,590 个顶层测试、installation/platform race、`make vet`、`make check-install`、双架构构建及 arm64 体积检查、cask 迁移脚本、发布分发脚本、Homebrew 声明与分派检查均通过。验收 tag 的 installation 包、I1–I5（I5 含 21 个子场景）和最终完整叶子命令访问屏障矩阵通过。CI 脚本的 shell/Python/YAML 语法及本地入口拒绝检查通过。

首次整包 CLI 验收出现 `watch` 一秒截止时间失败，并在后续 schema-signal 用例执行到第 5 秒时达到十分钟整包上限。补齐安装命令清单后单独执行完整叶子矩阵通过，包括 `watch`；I1–I5 复用首次运行中的通过结果。整包验收未记为通过，后续实际使用中仍需观察该时间相关失败。

精确候选与逐准则结果记录在 CEv1 WorkUnit `installed-state-ownership:installer-receipts`、ContentState `installed-state-ownership:installer-receipts:implement-20261110-Qx9ye6HAPTmIcTMg`。独立评审及 `real-brew` 仍待验证，任务保持开放。CI 当前仅完成脚本实现和静态检查，真实 runner 执行结果按上文例外补齐。

#### 评审交接（2026-11-10）

任务 5 的早期评审及推送 CI 结果保留各自的历史范围。Round 4 独立复评 PASS 按未改变的安装逻辑范围保留，Dev 与 Review 均已勾选。用户授权处理的 scanruntime 午夜夹具 panic 已修复；全量 Go 28 包、1,591 项顶层测试及 scanruntime race 47 项通过，validation 已通过。当前证据目标为 `installed-state-ownership:installer-receipts:validation-scanruntime-20261111-midnight`，完成门禁 NOT_VERIFIED，七项通过、仅缺 real-brew；按本任务已批准时序例外进入 `awaiting_commit`，任务保持开放。当前候选身份、诊断与验证边界见[任务评审记录](reviews/installer-receipts.md) Validation follow-up。真实 brew 未在本机执行，未提交或推送。

### 6. `app-state-location`

**依赖**：任务 4、任务 7。**结果**：App 不再自己推算状态目录；Health 面板显示安装检查，已安装 App 具有原生所有权会话（需求 G6/G9）。

**文件**：
- `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift`；
- `apps/macos/AgentDeckApp/AgentDeckApp.swift`；
- `apps/macos/AgentDeckApp/DesktopCopy.swift`；
- 对应的 `AgentDeckTests`、`AgentDeckAppTests`，以及 `AgentDeckVerification/main.swift` 中现有的参数断言。
- `scripts/test-macos-app.sh`：只在测试夹具中构建验收 helper、传递临时账户/测试产物路径，并清理本次启动的进程；
- 新增 `internal/installation/native_fixture_test.go`：原生测试使用的临时回执/排他锁驱动，只编译进 Go 测试二进制；
- `EmbeddedHelperRunner.swift` 中的原生所有权会话及 `AgentDeckApp.swift` 的生命周期装配：增加内部账户解析依赖，默认读系统账户数据库，仅测试构造时替换；对应 Swift 测试通过 `@testable` 使用。

**输入/输出接口**：消费任务 4 的 `desktop state-location`、任务 1 的回执/锁 API 和任务 7 的独立 Debug 身份；产出本轮状态路径、原生共享锁生命期、Health 文案及本任务自己的原生验收夹具。任务 2 提供已守卫的 Go 行为，不提供原生 helper 产物。实际安装器与 App 交接归任务 8。

**原生测试如何取得安装身份（ISO-TASKS-R9-F2）**：由任务 6 完成以下两层，不增加 `state-location` 线路字段，也不改变生产安装判定。

1. **Swift 确定性测试**：经现有 `EmbeddedHelperProcess` 注入返回 `state-location` 结果的替身，覆盖持锁前后结果改变、查询失败和释放路径。原生会话的账户解析依赖在内部测试构造时指定临时账户；锁仍由真实 `flock` 实现，路径为该账户的 `Library/Application Support/AgentDeck/ownership.lock`，不能从 `HOME` 或 `state_root` 反推。生产装配固定使用系统账户数据库，不能接受环境变量、CLI 参数或线路字段覆盖；该替身结果只证明 Swift 消费/生命周期，不证明 Go Resolver。
2. **Swift + 真实 Go helper 夹具**：`scripts/test-macos-app.sh` 在本次临时测试根内单独执行 `go build -mod=vendor -tags agentdeck_acceptance -o <fixture>/AgentDeck.app/Contents/Helpers/agentdeck ./cmd/agentdeck`。这里的 `AgentDeck.app` 是传给现有 `EmbeddedHelperRunner(appBundleURL:process:environment:)` 的临时 bundle 布局，使用 `FoundationEmbeddedHelperProcess` 真正启动其中的 helper；不替换正常 Debug/Release/Perf 产物，不修改 `scripts/build-macos-app.sh`、打包脚本或发布构建的 tag。
3. **夹具身份与同一把锁**：新 Go 测试驱动在 `_test.go` 内给 Resolver 注入经过校验的临时账户，调用任务 1 的真实 `AcquireOwnershipLock` / `WriteReceipt` 登记临时 bundle 中 helper 的物理路径。它通过 `scripts/run-go-test.sh ./internal/installation -run '^TestNativeOwnershipFixture$'` 接受仅测试入口读取的夹具参数，提供登记、排他锁竞争、回执改写及模拟手工删除动作；所有输入须位于本次临时根，默认不能回退真实账户。helper 环境显式包含同一个 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME`，安装身份用例的 `HOME` 也设为该临时账户，使默认状态根命中注入的真实根；未安装组使用独立 HOME 和不匹配/缺失回执。Swift 测试构造注入同一个账户，因此 Go 与 Swift 确实竞争同一路径的锁，不是各测各的锁。
4. **原生生命周期与证明边界**：XCTest/原生验证入口构造 App 实际使用的所有权会话及刷新/关闭回调，消费上述真实 helper；验证已安装判定、共享锁阻塞排他进程、下一轮复核及关闭后释放。App Group 存储和偏好沿用现有临时目录/测试域注入。缺少临时账户、helper 或注入入口时测试必须失败，不能启动生产默认路径补救。完整 App 进程与实际安装器的联合验收仍在任务 8，不能用替身响应宣称已经执行；生产产物仍只做 tag 缺席的静态检查。

**实施顺序与验收**：
- 按[架构](architecture.md#app-与小组件)实现：每一轮刷新前查询 `state_root`，本轮内使用，不跨轮缓存。**只有当 helper 是拥有者、并且 `state_root` 指向真实状态时**，App 才用 `flock(2)` 取所有权共享锁，并在锁内再查询一次确认，然后持有到退出；条件不成立时不取锁，之前持有的立即释放（ISO-ARCH-R3-F1）。删除显式传入的 `--state-dir`，删除 `HOME/.agentdeck` 的推算。
- 读快照文件、等扫描锁、读 `receiver.json` 都改用本轮取得的路径；取锁超时或查询失败时，按 helper 不可用处理。测试覆盖以下几点：
   - 已安装的 App 运行期间，任务 1 的排他锁测试进程拿不到锁；App 退出后可取锁并更新临时回执；
   - **未安装的 App（例如 Debug App）单独运行时，排他锁测试进程仍能取锁、写入或撤销临时回执**。这里只证明 App 不阻塞锁；实际安装、卸载、接管命令的成功由任务 8 消费任务 5 后验证；
  - App 取锁前后所有权发生转移时，锁内的再次查询发现条件不成立，立即释放锁；
  - 回执被手工删除之后，下一轮改用隔离目录并释放锁。
- 增加 `installation`、`managed_command_owner` 检查名，以及 `not_installed`、`managed_command_owner_mismatch` 说明的中英文文案。

**验证：L3**。由于新增跨进程锁与原生生命周期，运行 `make test-macos-app`、AgentDeckVerification 对应现有脚本，以及上面两层隔离的 Swift 会话/真实 helper/排他锁竞争测试；任务 7 的旧结果不能代替本任务改动后的回归。记录实际驱动命令、Go/Swift 解析出的同一临时锁路径、被测 helper 物理路径与回执身份；不能把构建成功或替身返回 installed 当成真实拥有者/锁生效。原生运行前确认临时 HOME、账户注入、App Group 和偏好域隔离；无法满足时报告该任务的验证阻塞。


#### 开发交接 — 2026-11-10

- 工作区 `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`，基线 HEAD `a4a5d684bc1e2d067f8e92113aa1c7f064bd4ba5`；依赖任务 4、7 均已本地交付。本轮未提交、推送、安装或修改开发者账号资源。
- `EmbeddedHelperRunner` 删除构造时 `HOME/.agentdeck` 推算与全部 `--state-dir` 注入。snapshot、serving restore、bootstrap 及 quota/telemetry 请求重新执行 `desktop state-location --format json`；本轮路径用于 serving 文件探测和扫描锁等待。查询失败/畸形输出不回退默认路径。
- 新增内部 `NativeOwnershipSession`：系统账户由 `getpwuid_r(getuid())` 解析，生产不接受环境/命令/线路字段覆盖；仅内部测试构造允许注入临时账户。installed 且物理 state_root 等于账户真实根时取得共享 flock，最长等待 5 秒，并在锁内再次查询。并发查询串行化；失去拥有者资格或查询失败释放锁；App 退出同步关闭描述符，包含正在异步复核的临时锁，关闭后的迟到查询不能重新持锁。
- 路径比较使用最近存在祖先的 `realpath` 加缺失组件，拒绝无法解析的路径和悬空符号链接，覆盖首次启动时 `.agentdeck` 尚不存在的 `/tmp` / `/private/tmp` 别名。不能仅依赖 Foundation `resolvingSymlinksInPath`：本轮最小实验确认它对既有父目录与缺失叶子的别名处理不同，真实 helper 交错测试曾据此发现 App 未持锁；修复后同一排他断言通过。
- App 的 termination 回调接入同步释放；telemetry controller 每轮重新定位 `telemetry/receiver.json` 监视器，查询失败撤销旧监视器并停止 listener。保留 `.codex/config.toml` 的客户端配置监视。Health 新增 Installation/安装、Hook command owner/Hook 命令归属，以及未安装和归属不匹配的英中文案。
- 原生夹具由 `scripts/test-macos-app.sh` 在本次隔离根单独构建验收 tag helper；`internal/installation/native_fixture_test.go` 仅进入测试二进制，严格校验临时根/账户/helper 物理路径，使用真实 `AcquireOwnershipLock` / `WriteReceipt`。驱动支持 register、mismatch、probe 和模拟手工 delete；不配置参数时跳过驱动入口，XCTest 集成用例缺少参数则失败，不回退真实账户。构建、打包脚本及生产 Go 行为未修改；正常 Debug helper 的 `go version -m` 为 Go 1.27.3、darwin/amd64，不含 acceptance build tag。
- 可复现主命令：将已有 Go 1.27.3 bin 放到本次进程 PATH 首位，设置 `GOCACHE=/private/tmp/agent-deck-go-build` 后执行 `make test-macos-app`。harness 内运行 `go build -mod=vendor -tags agentdeck_acceptance` 到临时 bundle；经 `scripts/run-go-test.sh ./internal/installation -c -o <fixture>/native-driver` 编译驱动，并通过该包装器的 `-run '^TestNativeOwnershipFixture$'` 注册夹具。Swift 用 FoundationEmbeddedHelperProcess 启动真实 helper/驱动，共用临时账户；后续驱动调用同一 TestNativeOwnershipFixture 入口。
- 最终运行观测（路径仅为当次身份事实，不是保留工件引用）：运行根标识 `agentdeck-macos-xctest.YDy3Od`，helper 的物理路径为 `/private/tmp/agentdeck-macos-xctest.YDy3Od/AgentDeck.app/Contents/Helpers/agentdeck`；Go 与 Swift 竞争 `/private/tmp/agentdeck-macos-xctest.YDy3Od/account/Library/Application Support/AgentDeck/ownership.lock`。末次注册回执 `installation_id=47c3667b4dd6dad4f25bd03a82a5df26`、channel `local_app`，executable 与上述 helper 一致。夹具随测试退出清理；重现时生成新身份，不依赖这个临时目录仍然存在。
- 真实 helper 测试通过：已安装/真实根时排他驱动报告忙；模拟手工删除回执后下一轮返回隔离根并允许排他锁；回执登记另一个 helper 时不持长期锁；重新注册后关闭会话允许排他锁。这里只验证原生会话与真实 Go Resolver/锁交互，不声称完成真实安装器和完整 App 进程的 Task 8 A5 联合验收。
- 最终 `make test-macos-app` exit 0、TEST SUCCEEDED：Shared 133、App 168、Widget 51，共 352 项，0 failure，1 项既有 schema 手工矩阵 opt-in skip；原日期焦点用例本次通过，harness 未报告 helper 泄漏。新增确定性用例覆盖锁内资格改变、非真实状态不持锁、查询失败释放、锁超时、wire 拒绝、两轮路径变化、查询/复核中退出和物理路径别名；App 用例验证 receiver 监视器随路径变化、失败后清除。
- `swift run --disable-sandbox --package-path apps/macos --scratch-path <临时构建缓存> AgentDeckFoundationVerifier` 携带 `desktop/fixtures/v1/` 的 complete、partial、empty-client、legacy、schema-ahead 五个快照，通过全部 Foundation/helper 参数边界验证。首次执行暴露既有 verifier 的 schema 32 断言与当前 schema 33 契约/夹具不符；仅校准测试断言为 33，并保留 31→32、31/32→33 兼容和反向/未知版本拒绝，不修改产品 schema。最后在已编译的最终候选 verifier 再执行上述五个夹具，同样通过。
- 失败历史保留：首次完整回归因 Swift 夹具把 `/private/tmp` 改成 `/tmp` 而被测试驱动拒绝；修正路径传递后，真实排他断言暴露上述物理路径比较缺陷。两项均修复并由最终原始全量命令验证；中途主动取消的一次构建不算通过。未使用重试、放宽断言/超时或跳过集成用例掩盖失败。
- `bash -n scripts/test-macos-app.sh`、局部文档链接、空白和 `git diff --check` 通过。CEv1 WorkUnit `installed-state-ownership:app-state-location`，候选 ContentState `installed-state-ownership:app-state-location:dev-20261110-3Xphhahl`；Dev 完成、Review 未执行。完成门禁 NOT_VERIFIED，独立评审项待补；Beads 转 `in_review` 交接，不宣称 Task 或 Topic 完成。

#### 修复交接 — 2026-11-10

- Round 1 所需修复已完成，原始完整 XCTest 353 项、0 failure、1 项既有 opt-in skip；详见[修复记录](reviews/app-state-location.md#repair-1--2026-11-10)。候选 `installed-state-ownership:app-state-location:repair-r1-f1-20261110`，完成门禁 NOT_VERIFIED，仅待独立复评；Dev 保持完成、Review 未勾选，Beads 为 `in_review`。未提交或推送。

### 7. `debug-app-identity`

**结果**：提供独立 Debug 构建身份（需求 G5），供任务 6 安全运行原生夹具；实际双 App 数据与生命周期互不干扰的 A5 组合验收归任务 8。本任务没有 Go 功能前置依赖。

**签名验收前提**：`N2FZ2FNRTU.group.com.kitdine.agentdeck.debug` 使用 macOS 的 `<Team ID>.<group name>` 格式，无需门户注册或 App Group provisioning profile。验证实际签名团队为 `N2FZ2FNRTU`，App/Widget 的实际 entitlements 与 Info.plist 一致；无签名 XCTest 不能替代这些真实签名证据。依据与适用边界见[架构](architecture.md#app-与小组件)。用户禁止注册，本任务不创建或修改开发者账号资源。

**文件**：
- `apps/macos/AgentDeck.xcodeproj/project.pbxproj`：只改 Debug 配置的 bundle ID 和 `AGENTDECK_APP_GROUP`；
- 检查小组件的 sandbox 校验脚本是否需要同步修改。

**输入/输出接口**：消费架构中已确定的 bundle ID/App Group 表和有效的签名身份；产出 Debug 的 App/Widget 标识、entitlements 与 Info.plist，Release/Perf 配置保持原值。

**实施顺序与验收**：
- 按[架构](architecture.md#app-与小组件)的表格修改，Release 和 Perf 配置逐项不变。
- 证明 Debug 构建产物的 entitlements 和 Info.plist 中都是 Debug 的 App Group。
- Release 构建的签名、`check-widget-sandbox` 都不变。

**验证：L3**。构建 Debug、Release 两种产物，对比 Release/Perf 配置，检查实际签名、Info.plist、entitlements 并运行 `make check-widget-sandbox`。按 `scripts/build-macos-app.sh` 的配置入口选择命令；若 sandbox 检查含固定 Release ID，同步成按构建配置校验，不削弱 Release 断言。**任务 7 必须在自己的候选上运行 `make test-macos-app`**：现有 `scripts/test-macos-app.sh` 为 Debug 宿主设置临时 `HOME`、`CFFIXED_USER_HOME`、`AGENTDECK_TEST_HOME` / `TEST_RUNNER_AGENTDECK_TEST_HOME`，使用 `CODE_SIGNING_ALLOWED=NO`；App 将 helper HOME、App Group 存储和偏好切至测试夹具，现有测试无需等任务 6。确认 `AgentDeckAppTests` 的 Debug `TEST_HOST` 能加载并完成回归；仅 Command Line Tools 下的 Foundation verifier 不覆盖宿主，缺完整 Xcode 时明确保留该项未完成。任务 6 为自己的行为变更再运行该门禁，不接收任务 7 未解决的宿主回归。真实签名、团队前缀和 entitlements 一致性仍需单独验证，不能用无签名通过代替；不要求门户注册。


#### 开发进展与验证边界 — 2026-11-10

以下保留当时的开发取证历史；其中门户注册前提的判断已由本任务后续平台规则勘误纠正，不再是当前完成条件。

- 工作区 `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`，基线 HEAD `62e2f239002899fd5d0ba4bf26ea9280c66ba827`。修改 Xcode 项目两个 Debug 目标的 bundle ID/App Group、`scripts/check-widget-sandbox.sh`，并在 `MenuBarViewModelTests.swift` 的两个既有失败消息中补充激活请求结果、activation policy 和前台应用 bundle ID；未改 Go/Swift 产品行为、测试断言、等待期限或跳过条件，未提交或推送。
- Debug App/Widget 分别为 `com.kitdine.agentdeck.debug` / `com.kitdine.agentdeck.debug.widget`，二者 App Group 为 `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug`。对项目对象逐项比较，全部 Release/Perf 对象与 HEAD 相同；共享 xcconfig、entitlements 模板与 Info.plist 模板未变。
- 环境：macOS 26.7.3、Xcode 26.4 (17E192)、Go 1.27.3 darwin/amd64。运行命令前将已有 Go 1.27.3 的 bin 放到本次进程 PATH 首位，修正原 PATH 的 Go 1.27.1 与 GOROOT 1.27.3 不一致；未改全局环境。原生 harness 的 `ps` 及 XCTest 经执行权限批准，仍使用其原有临时 HOME、CFFIXED_USER_HOME 和测试域隔离。
- `make test-macos-app` 内的 Debug 构建通过；`make build-all DIST_DIR=<临时目录>` 后以 `AGENTDECK_APP_CONFIGURATION=Release AGENTDECK_DIST_DIR=<同目录> AGENTDECK_MACOS_BUILD_DIR=<独立临时构建目录> bash scripts/build-macos-app.sh` 构建 Release 通过。Debug 为 x86_64；Release App、Widget 和 helper 为 x86_64/arm64。产物的 App/Widget Info.plist 均与各自配置的身份一致。
- 在临时产物副本上按打包脚本相同顺序，对嵌套代码、helper、Widget、App 执行 `codesign --force --sign <本机 Developer ID> --options runtime --timestamp --generate-entitlement-der`；App/Widget 传入按各自产物 App Group 展开的原 entitlements 模板。两种配置的 `codesign --verify --deep --strict` 均通过，签名团队为 `N2FZ2FNRTU`；解码实际签名确认 App Group 与 Info.plist 一致，Widget sandbox 为 true。此为本地签名证据，不是账号端注册或运行时容器访问证据。Release 构建仍自动登记了临时 App（临时 `REGISTER_APP_WITH_LAUNCH_SERVICES=NO` 设置未阻止它），随后已用 `lsregister -u <该临时 App>` 撤销；未启动、安装或公证 Release 产物。
- `bash -n scripts/check-widget-sandbox.sh`、`make check-widget-sandbox` 通过。独立临时副本中将 Debug bundle 改回安装版身份、将 Release 默认 bundle 改成 Debug 身份，两个负例均被新校验拒绝；原 sandbox/私有数据边界断言保留。
- 完整 XCTest 两次均为 Shared 122 项通过、App 167 项中 1 项失败和 1 项既有 opt-in skip、Widget 51 项通过。唯一失败为 `MenuBarViewModelTests.testDateEventsInvalidateRenderedSurfacesWithoutRefreshOrFilterChanges`：首轮 line 206 的 rearm 渲染检查失败，数值已更新但宿主 active/key 均为 false；串行复跑在 line 193 的前台/key-window 前置检查失败。复用同一已构建产物、原隔离 harness、`test-without-building -only-testing:AgentDeckAppTests/MenuBarViewModelTests/testDateEventsInvalidateRenderedSurfacesWithoutRefreshOrFilterChanges -test-iterations 3 -test-repetition-relaunch-enabled YES` 的三个独立宿主均通过。此为最初候选的失败记录，未以定向通过替代完整回归；后续结果如下。
- 用户在本轮明确表示“那就不查呗”，随后要求继续执行，因此停止 Apple 开发者账号端注册查询；注册状态保持未核验，不把本机证书或签名成功写成已注册。
- 继续诊断时，将 `MenuBarChromeTests` 全组与日期用例组合运行，37 项全部通过；“前序测试保留窗口必然导致失败”的假设未得到复现。仅加入上述失败诊断后，再执行原始 `make test-macos-app`，Shared 122 项、App 167 项、Widget 51 项均通过，合计 340 项、0 failure、1 项既有 opt-in skip（schema 手工验收矩阵）。这次完整套件的日期用例通过，耗时 23.396 秒。未增加重试、延长超时、收起其他窗口或修改产品激活策略来获得通过。
- 诊断结论：Go 工具链冲突已定位并以单次进程 PATH 修正；原生焦点失败属间歇现象，完整运行已通过，但具体外部焦点竞争或套件时序根因尚未确定，不能声称已根治。保留失败历史和增强诊断供独立评审；任务 6 不承接本任务未执行的回归。
- 开发交接时 Dev 已完成、Review 未执行。CEv1 WorkUnit 为 `installed-state-ownership:debug-app-identity`，最终候选状态 `installed-state-ownership:debug-app-identity:dev-20261110-xdSv0auq`；复用未变化的身份/Release/Perf/sandbox/本地签名证据，新增完整 XCTest 结果。注册未核验和独立评审缺口保留，任务完成门禁不得宣称 VERIFIED。无真实双 App 运行、安装或 A5 组合验收声明。

#### 评审状态 — 2026-11-10

- Round 1 独立实现评审 PASS 保留，见[任务评审及平台前提更正](reviews/debug-app-identity.md)。更正后的验收 WorkUnit 为 `installed-state-ownership:debug-app-identity:acceptance-v2`，当前 ContentState 为 `installed-state-ownership:debug-app-identity:correction-20261110`，门禁 VERIFIED，等待授权提交。旧注册门禁及开发历史保留，不声称注册已发生；未提交或推送。

### 8. `ownership-acceptance`

**依赖**：任务 3、5、6，传递包含任务 1、2、4、7。**结果**：在完整实现上验证 A1–A9 的组合路径，查验最终入口清单，形成主题验收证据。

**文件/发现入口**：复用 `cmd/agentdeck/contract_test.go`、现有 CLI/schema-signal 验收测试、任务 2 的屏障夹具、任务 5 的安装脚本测试，以及任务 6 的原生验证入口。新增集成用例落在对应验收套件内，实际文件与可重现命令必须写入评审材料，不另造脱离产品实现的协议模型。

**输入/输出接口**：消费任务 1–7 已交付的真实能力和可复用证据，产出逐命令结果矩阵、跨进程与跨安装方式场景、生产静态验证及操作员事项。未完成的前置任务是开工阻塞，不是本任务隐含实现范围。

**验收运行器**（ISO-TASKS-R1-F1）：按[测试与验收的隔离](architecture.md#测试与验收的隔离)使用验收二进制，构建命令为 `go build -tags agentdeck_acceptance -o <临时目录>/agentdeck ./cmd/agentdeck`。`AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 指向临时账户主目录，回执、注入的真实状态目录、客户端配置都在这个目录下；同时使用临时 `HOME`、`PREFIX`、`APPDIR`。scan worker 和 helper 是同一个可执行体，并继承这个环境变量。

**实施顺序与验收**：先确认全部前置能力与环境，再建立完整命令矩阵，随后运行安装/CLI/App 串联场景，最后汇总未覆盖项和静态产物证据。

**完整 Cobra 结构性验收（承接 ISO-TASKS-R7-F1、ISO-TASKS-R9-F1）**：在最终候选重新枚举并执行全部叶子矩阵，包括任务 2 已覆盖的命令、任务 4/5 新增命令和任务 3 的真实配置拒绝；不能只复用任务 2 的旧清单或只测新增项。记录有效参数、stdin、数据、目录分支、预期退出码/输出及契约依据，固定契约因 shell/交互/内部表面而分组的入口也沿任务 2 的独立运行器承接。每例独立账户与 HOME，初始未安装且不满足续登；默认隔离根不命中 P。缺参、无效输入、未知命令均不能冒充覆盖；不解析状态的入口也必须记录既定结果及依据。

| 输入 | 命令/目标 | 预期及能力来源 |
| --- | --- | --- |
| 不传 `--state-dir` | 普通状态命令 | 任务 2：正常使用隔离根；空 `provider list` exit 0、`No providers.`；仅文本 TTY 非 quiet 有 notice |
| 同上 | doctor | 任务 2/4：隔离根缺失为 `state_missing` 部分报告、exit 0；其他夹具遵守对应 doctor 契约 |
| 同上 | 写真实客户端配置 | 任务 3：`state_ownership_refused`、`resource: client_config`、exit 1、配置不变；非真实账户的临时配置保留正常结果 |
| 同上 | Hook/statusline | 任务 2：保持隔离调用的正常处理与输出，不能统一断言安装拒绝 |
| 显式真实根或子路径 | 普通状态命令 | 任务 2：内容访问前拒绝，`resource: state_root`、exit 1、refused_state 文本/JSON |
| 同上 | doctor | 任务 2/4：成功 envelope、partial/checks_skipped、exit 0，仅 installation/state；后续诊断不运行 |
| 同上 | Hook/statusline | 任务 2：stdout/stderr 为空、exit 0、不写拒绝记录 |
| 任意状态参数 | 不解析状态的命令、install claim/register/unregister | 保持各自契约；任务 5 的安装命令只操作回执/锁/安装对象，不强制套用状态拒绝 |
| helper 路径查询 | desktop state-location | 任务 4/6：未拥有真实状态时不返回真实路径；App 无路径时不自行推算 |

默认与显式两组均使用屏障和正负对照：不访问 P 内部、不被屏障终止；文件清单/内容/mtime 不变仅为补充证据。显式路径覆盖直接、符号链接、`..`、真实根自身为链接及其子路径，worker/helper 继承屏障与账户注入。包级入口复用任务 2 的直接调用证据，并对最终新增/变更入口补测。满足 formula 续登条件的实例放在独立夹具，不能污染默认隔离组。**统一的是访问边界，不是所有命令的退出码。**

**跨面场景**：
- 按以下顺序走一遍：
  1. 未安装：在访问屏障下运行，确认没有访问真实目录、客户端配置的写入被拒绝；
  2. 用本地 App 安装方式把验收二进制装入临时 `APPDIR`：确认成为已安装，能迁移注入的真实目录；
  3. 用另一个程序尝试接管：确认被拒绝；
  4. 旧拥有者有一个会话正持有所有权共享锁（例如一个 `watch` 进程）时加 `TAKEOVER=1`：确认以 `owner_running` 拒绝，回执不变；
  5. 停止旧拥有者后加 `TAKEOVER=1` 接管：确认成功，旧可执行文件随后在屏障下被拒绝；
  6. 在临时 Cellar/opt 布局中执行 formula `install claim`、旧 keg 认领、`--takeover`；然后模拟一次升级：建立新 keg，把 opt 切到新 keg，删除旧 keg，然后用新 keg 执行一条**不带 `--state-dir` 的普通命令**，确认它在排他锁内续登，并读到注入真实目录中的原有数据；再模拟一次保留旧 keg 的升级，确认不续登、新 keg 按未安装处理；
   6a. 用一个未安装的 App 替身持续运行（它的 helper 不是拥有者），在临时安装目标上分别执行任务 5 的实际安装、接管、卸载完整命令链，确认均成功且不会出现 `owner_running`。卸载必须运行到 `install unregister --remove` 及安装方式的后续步骤完成，断言目标和匹配回执按契约清理；替身全过程保持运行。各场景重建独立安装夹具，不能仅以“排他锁可取得”代替命令成功；
  7. 卸载：确认回执被清理；
  8. 损坏回执：确认 `doctor` 给出原因；`doctor --state-dir <注入的真实目录>` 输出部分报告；
  9. 运行 schema-version-signal 的回归测试，确认仍然通过（A8）。
- 生产产物只做有限的静态验证：`make build-all` 的产物中没有验收 Resolver 的符号，构建命令中没有 `agentdeck_acceptance`。它的运行时行为由验收二进制和包内测试覆盖，这一点在评审材料中明确写出，不能声称已用生产产物做过运行时验收。
- 在上述串联之外逐项核对四种安装 channel 的当前拥有者能够读取/迁移注入状态；保留旧 keg、双前缀、stable/rc、过期回执、跨安装方式接管各有明确结果。只运行 local_app 和 formula 两条路径不能覆盖 A2/A6 全集；可复用任务 5 对同一候选已产生的有效证据。
- A5 在任务 6、7 完成后验证：Debug 与安装身份 App 同时运行，状态目录、App Group、偏好域、小组件列表和元数据分别独立；未安装 App 持续运行期间，任务 5 的实际安装、接管和卸载（包含 `install unregister --remove` 完整清理链）均成功，卸载目标及匹配回执按契约清理；已安装 App 持锁时实际接管失败、退出后成功。隔离原生替身证据与真实双 App 运行必须分开记录；后者若需要本机注册/安装，由操作员执行或明确豁免，未执行且未豁免时 A5 保持未完成。
- 用真实本机安装做验收会触碰真实状态，不在本任务内执行。在交付说明中列为操作员事项：用 `make install-macos-app` 重装本机；formula 用户需要执行一次 `install claim`。

**验证：L3**。执行最终 CLI/worker/helper 屏障矩阵、安装与 App 串联场景、A8 回归、`make build-all` 与验收符号静态检查；按实际新改的集成测试及消费者选择回归与 race 范围。复用同一内容态仍有效的任务 1–7 证据，不因进入任务 8 重跑所有组件测试；原生/真实 brew/屏障证据不能互相替代。每项 A1–A9 必须有可定位的实际结果或用户明确接受的豁免，不能仅由测试命令 exit 0 推定。

### 9. `contract-reconciliation`

**依赖**：任务 8（传递包含任务 1–7）。**结果**：产品契约与已验收实现一致。

**文件**：`docs/specs/cli-design.md`、`docs/specs/cli-manual.md`、`README.md`。

**要求**：按[兼容性与契约同步](architecture.md#兼容性与契约同步)更新 Local State、Build Identity and Installation、Output and Errors、Doctor 这几节，以及安装和故障排查说明。

**输入/输出接口与验收**：消费任务 8 的行为结果与限制，逐项对照 CLI 文本/JSON、安装方式、formula 首次 claim 与自动续登、App 路径、拒绝恢复、真实环境操作员步骤，产出稳定规范、手册和 README。说明回执/所有权锁/清理清单/helper 不属于便携状态；保留已安装兼容和 schema_ahead 契约，不把未执行验收写成支持保证。文档闭环不授权安装、提交、推送或发布。

**验证：L0**。需要 `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`make check-whitespace`、`git diff --check`。

## 覆盖与交接核对

### 需求归属

| 需求/验收 | 实现责任 | 最终组合证据 |
| --- | --- | --- |
| G1，A3 身份判定 | 1 已交付；2 基础 installation 报告 | 8 的损坏/缺失/过期/其他拥有者矩阵 |
| G2/G3，A1/A2 状态边界 | 2 Go 访问链；4 helper 路径；6 原生消费 | 8 的完整叶子命令、worker/helper/App 及四种安装身份 |
| G4，A4 客户端配置 | 3 | 8 的默认隔离状态 + 真实配置写入组合 |
| G5，A5 Debug 隔离 | 7 构建身份；6 原生状态路径 | 8 双 App 运行；操作员结果或明确豁免单列 |
| G6 App 路径 | 4 生产方，6 消费方 | 8 真实 helper 与 App 串联 |
| G7，A6/A7 唯一拥有者及升级 | 1 身份基础；2 自动续登入口；5 安装协议 | 8 从无回执到安装/升级/清理的完整路径 |
| G8 可见性 | 2 提示/错误/基础诊断；3 配置拒绝；4 完整诊断；5 claim；6 Health | 8 固定输出矩阵，9 稳定契约 |
| G9，A9 安全交接 | 2 Go 会话；5 发布清理；6 App 会话 | 8 实际安装器与长驻/原生消费者组合 |
| A8 schema-version-signal | 2–6 保持各自受影响回归 | 8 完整兼容回归；9 保持契约 |

### 本次验收归属与迁移核对

| 来源与断言 | 当前归属/是否迁移 | 在该任务可完成的依据 |
| --- | --- | --- |
| 旧任务 2 的真实客户端配置拒绝 | 迁至 3 实现与定向验收，8 全叶子组合 | 3 同时交付所有写入守卫及错误渲染；现有配置命令的非真实 HOME 路径仍由 2 测试 |
| 旧任务 4 实现、旧任务 2 验收的 doctor 部分报告 | 实现并入 2；4 只扩展完整诊断 | 拒绝与例外同批交付，不依赖后续 4 |
| 旧任务 2 交错 8–10、12 | 迁至 5 I1–I4 | 5 具备 register/unregister/清理链，并依赖 2 的真实会话；I2/I3 保留目标不变及暂停恢复的完整结果 |
| 旧任务 2 在当时候选上已有的全部 Cobra 叶子 | 保留在 2；8 在最终集合复跑 | 现有命令以非真实账户临时配置即可验证状态边界；不依赖未来命令或真实配置拒绝 |
| 任务 4/5 后来新增的命令及最终组合 | 各任务定向验收，8 纳入最终叶子矩阵 | 8 的依赖闭包已包含生产者；不用于削弱任务 2 的现有清单 |
| 旧任务 6：未安装 App 运行时实际安装、卸载、接管成功 | 6 证明真实原生会话不阻塞锁；实际命令链迁至 8 的 6a/A5 | 6 有自己产生的原生身份夹具；8 再消费 5 的安装命令与 6 的 App，两处均包含 unregister --remove 及后续卸载步骤 |
| 旧任务 7：make test-macos-app 宿主回归 | 保留在 7；撤销重写稿迁至 6 的安排 | 现有临时 HOME/无签名 harness 已可运行；6 只对自己的新行为再跑，不承接 7 的回归缺口 |
| 旧任务 7：Debug 与安装 App 的全数据共存 | 迁至 8；7 仍承担自身构建和宿主回归 | 8 同时具备 6 的路径隔离与 7 的身份隔离；真实运行/操作员豁免单列 |

### 评审与风险边界

下一次分解评审必须逐条执行：从验收断言反查实际被测入口 → 确认其实现任务 → 核对该任务是否在当前任务依赖闭包内。仅有需求覆盖表、无环的任务编号或 L0 通过都不足以证明可实施。逐项检查上面重新归属的测试没有丢失，且没有把不可验证的完成条件藏进“后续验收”。

旧 Round 8 的具体遗漏与本次重写交接记在 [tasks 评审记录](reviews/tasks.md#重新分解--2026-11-07)。它的历史结论和已交付任务 1 的事实保留；本稿需新的独立评审，不由作者自签 PASS。

仍有明确的环境前提：任务 5 在 CI 的 GitHub 托管 macOS runner 上以隔离的真实 brew 执行验收版 helper 的能力（开发机上会被 Gatekeeper 拦截，不在开发机运行）。任务 7 的门户注册前提已按 Apple 官方 macOS 规则撤销，真实签名验收仍保留。屏障不可用阻塞需要屏障的任务；原生隔离不可用阻塞任务 6/8 对应证明。失败时保留准确结果并暂停该边界，不能降低验收标准。实现仅在临时状态上试验；不通过数据库降级、真实目录回滚或锁外文件删除来恢复测试/安装。可交付回退和残余清理按架构的锁内安装协议处理。
