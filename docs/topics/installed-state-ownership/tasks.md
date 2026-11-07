---
status: active
created: 2026-11-05
updated: 2026-11-07
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

文档集共四份，都已写成初稿。四份可以作为一批，由一次独立评审覆盖，但要分别给出结论。

当前文档评审结论：需求、CLI、架构、任务分解均 PASS。CLI 的逐字符标本在共享原型 `?surface=cli&ownership=<state>`（`prototype/src/ownership.js`）。逐文档记录见
[需求](reviews/requirements.md)、[CLI](reviews/ux-cli.md)、
[架构](reviews/architecture.md)、[任务分解](reviews/tasks.md)。
`installation-identity` 已通过独立复评并补齐验证，交付状态以 Beads 为准；见
[任务评审](reviews/installation-identity.md)。其余实现任务尚未进入开发。来源缺陷保持开放。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `installation-identity` | [x] | [x] |
| 2. `state-root-guard` | [ ] | [ ] |
| 3. `client-config-guard` | [ ] | [ ] |
| 4. `installation-diagnostics` | [ ] | [ ] |
| 5. `installer-receipts` | [ ] | [ ] |
| 6. `app-state-location` | [ ] | [ ] |
| 7. `debug-app-identity` | [ ] | [ ] |
| 8. `ownership-acceptance` | [ ] | [ ] |
| 9. `contract-reconciliation` | [ ] | [ ] |

任务分解已通过评审，实现任务按本矩阵在 Beads 中登记。每个任务开工前还需要用户下达开发指令。多个任务会改到同一个文件，比如 `cmd/agentdeck/main.go`，这些任务按顺序进行，不并行。

所有测试一律使用临时 `HOME`、临时状态目录，并按[测试与验收的隔离](architecture.md#测试与验收的隔离)替换"真实账户主目录"，绝不触碰本机真实的 `~/.agentdeck`、`~/.claude`、`~/.codex` 或回执。

凡是证明"未安装进程不访问真实状态"的测试，都必须在[访问屏障](architecture.md#测试与验收的隔离)下运行。屏障禁止访问 `P` 中各入口之下的任何内容，只允许读取入口本身的元数据。每次运行前先做两项对照：

- 正向对照：用待测二进制真正执行一次"物理解析 → 守卫 → 拒绝"，必须以 `state_ownership_refused` 结束，不能被终止；
- 阴性对照：对目录内部访问一次，进程必须被终止。

只断言文件清单、内容和 mtime 不变，不能作为"没有访问"的证据。屏障不可用（`sandbox-exec` 缺失，或者嵌套时 `sandbox_apply` 返回 exit 71）时测试失败，这类测试要在没有外层沙箱的环境中运行。

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

**依赖**：任务 1。**结果**：未安装进程默认使用隔离目录，在任何入口都访问不到真实状态目录；失去所有权的进程在下一次访问前就停下来（需求 G2、G3、G9，验收 A1、A2、A9）。

**文件**：
- `cmd/agentdeck/main.go`：`stateRoot`、`shellStateRoot`、错误码映射、文本渲染、TTY 提示；
- `internal/installation`：`StateRoot`；
- `internal/platform/state.go`：`GuardStateRoot`、`RevalidateOwnership`；
- 在以下位置的打开和加锁入口加守卫：`internal/store`（`store.go`、sessions、lock）、`internal/credentialvault`、`internal/scanruntime`、`internal/hookrefusal`，以及 telemetry receiver 配置的读写；
- 在长驻循环中加复核：`watch`、telemetry 监听、desktop bootstrap 流、scan worker 的每一轮；
- 上述包的测试，以及 `cmd/agentdeck` 的固定测试。

**要求**：
- 按[状态目录解析](architecture.md#状态目录解析)实现默认目录，用任务 1 的 `walk` 判断"指向真实状态目录"。
- `GuardStateRoot` 返回物理路径。所有调用者此后都只能使用这个返回值，不能再使用原始字符串；评审材料中要列出每个调用点，并说明它改用了返回值。
- 已安装进程第一次访问真实状态前，按[所有权会话](architecture.md#所有权锁与交接)取共享锁，并在锁内判定身份；共享锁一直持有到对真实状态的访问全部结束。覆盖单次命令、Hook、`OpenReadOnly`、scan worker 和长驻循环。重新枚举所有"接收 `stateRoot` 参数、并会创建、打开、加锁、stat 或删除文件"的导出函数，以及所有长驻循环，最终清单记录在评审材料中。
- **续登的两个入口（ISO-ARCH-R4-F2）**：按[状态目录解析](architecture.md#状态目录解析)，没传 `--state-dir` 时，`StateRoot` 在选择默认目录之前先检查续登条件，满足就续登，然后重新判定。续登成功就使用真实默认目录；续登失败返回 `state_ownership_refused`，不退回隔离目录。显式指向真实状态时，由守卫在拒绝之前尝试续登。显式指向非真实目录时不续登。两个入口调用任务 1 的同一个续登函数；续登过程只读写回执和所有权锁，不访问 `P`。
- 新增错误码 `state_ownership_refused`，退出码 `1`，details 为 `resource` 和 `reason`。文本和 TTY 提示与原型 `?surface=cli&ownership=` 中 `notice`、`refused_state`、`revoked` 三个状态逐字一致（见 [ux/cli.md](ux/cli.md#标本)）。
- Hook 和 statusline 遇到拒绝时保持静默失败开放：exit 0，不输出，也不写任何拒绝记录。
- **结构性回归测试（ISO-TASKS-R7-F1）**：取 CLI 固定测试已枚举的全部 Cobra 叶子命令，逐一记录输入、目录分支、预期退出码/输出及其契约依据。每例使用独立夹具，初始身份为未安装且不满足 formula 续登条件；注入的真实账户目录与临时 `HOME` 均在夹具内，确认默认隔离根 `<临时 HOME>/.agentdeck-dev` 不命中保护集合 `P`。这里的“默认调用”只指不传 `--state-dir`，仍须提供命令所需的合法参数、stdin 和数据，不能用缺参或无效输入的提前退出代替守卫覆盖。按[状态目录解析](architecture.md#状态目录解析)和 [CLI 契约](ux/cli.md#状态矩阵)拆分断言：

  | 输入 | 命令/目标 | 预期 |
  | --- | --- | --- |
  | 未传 `--state-dir` | 普通状态命令 | 使用隔离根并保留该命令在夹具中的既定结果，不因未安装而统一拒绝。空 provider 夹具下 `provider list` 必须 exit 0、stdout 为 `No providers.`；文本 TTY 的 stderr 为 `notice` 标本提示，JSON/NDJSON、非 TTY 或 `--quiet` 不增加提示 |
  | 未传 `--state-dir` | `doctor` | 检查隔离根；隔离根缺失时按 `doctor_not_installed` 输出 `state_missing` 部分报告、exit 0，不能误报真实根被拒绝；其他隔离状态沿用对应 doctor 契约 |
  | 未传 `--state-dir` | 要写入注入真实账户目录下的客户端配置 | 状态目录仍隔离，但配置写入按原契约拒绝：`state_ownership_refused`、`resource: client_config`、exit 1，配置逐字节不变。写入非真实账户目录的临时配置则保留原有结果 |
  | 未传 `--state-dir` | Hook/statusline | 在隔离根上遵守原命令的处理与输出契约；只有遇到拒绝的场景才应用静默 fail-open，不能把正常隔离调用也设为安装拒绝 |
  | `--state-dir` 指向注入真实根或其子路径 | 普通状态命令 | 在访问内容前返回 `state_ownership_refused`、`resource: state_root`、exit 1，文本/JSON 与 `refused_state` 契约一致 |
  | 同上 | `doctor` | 按 `doctor_refused` 输出成功 envelope 的部分报告：`partial: true`、`warnings` 含 `checks_skipped`、exit 0，`state` 检查为拒绝；不把该目录交给后续诊断 |
  | 同上 | Hook/statusline | 静默 fail-open：exit 0、stdout/stderr 均为空，不写拒绝记录；内部拒绝不变成对外错误输出 |

  不解析状态目录的入口，以及 `install claim` 等有独立契约的命令，逐命令保留既定结果并注明依据，不强制套用普通状态命令的错误；枚举中不得有无预期或无依据跳过的条目。满足续登条件的新 keg 使用下面第 7、11 条的独立夹具，不混入默认隔离组。

  两组共用前述访问屏障及正负对照，统一断言不访问 `P` 内部、没有进程被屏障终止，并保留真实目录文件不变的补充断言；**统一的是访问边界，不是退出码或输出**。显式真实根组覆盖直接路径、经符号链接、经 `..`、真实根本身为链接及其子路径。scan worker 子进程同样继承屏障，按其实际接收的隔离/真实根分别验证既定结果或访问前拒绝。受保护包的直接调用也在屏障下分别验证隔离路径可用、真实路径被拒绝，按包级错误契约断言，不套用 CLI 渲染例外。
- **受控交错测试（ISO-ARCH-R1-F2）**：用测试钩子在指定位置暂停，逐一构造以下顺序，断言结果：
  1. A 已取得共享锁并打开真实状态 → B 尝试接管：B 等到上限后以 `owner_running` 失败，回执不变；A 正常完成。
  2. B 已取得排他锁 → Hook 启动 A：A 在取共享锁处等待；B 写入新回执并释放锁后，A 读到新回执，在访问之前被拒绝。这一步在访问屏障下运行，证明 A 没有访问。
  3. A 只读打开（`OpenReadOnly`）期间 → B 尝试接管：结果同第 1 条，证明读路径同样受保护。
  4. 持锁跨轮的长驻进程运行期间 → B 尝试接管：B 失败；长驻进程退出后 B 成功，此后旧可执行文件在屏障下被拒绝。
  5. 长驻进程逐轮持锁、在两轮之间释放 → B 在两轮之间接管：长驻进程在下一轮的复核处以 `state_ownership_refused` 退出，用轮次计数证明它没有开始新一轮。
  6. **评审 Round 3 反例**：旧 keg A 是登记的拥有者，持有共享锁、正在读或写会话中 → opt 从 A 切到 B → 启动 B：B 不是拥有者，不满足续登条件（A 仍存在），在访问之前被拒绝；A 的会话不受影响。
  7. 续登：A 持有共享锁时删除 keg A、opt 指向 B → 启动 B：B 尝试续登，排他锁一直等到 A 的会话结束（超过上限则以 `ownership_busy` 拒绝本次访问，回执不变）；A 结束后，B 在锁内续登并成为拥有者；此后 A 的残留进程在下一次过守卫时以 `executable_unresolved` 被拒绝。
  8. 替换已登记的文件：A 持有共享锁时，安装方式执行带 `--source` 的 `install register`，以 `owner_running` 失败，目标文件和回执都不动；A 结束后，同一条命令在一次持锁内完成发布和登记。用测试钩子在"发布之后、写回执之前"暂停，断言此时其他进程取共享锁会等待，在锁外看不到任何中间状态。
  9. **两个安装方式以相同 channel/目标竞争同一路径（ISO-ARCH-R4-F1 第五轮反例 1）**：A 和 B 都执行 `install register --channel make --executable P --source <各自的暂存文件> --cleanup-manifest <各自的新清单>`。用测试钩子让 A 停在锁内发布之前，同时启动 B：B 在取排他锁处等待，没有任何读写；A 完成发布和登记并释放锁之后，B 才进入，它看到的是 A 已完成的正式回执（同一路径、同一安装方式），在自己持锁、没有任何会话的状态下完成覆盖。断言任何时刻都不存在"P 被一方改写、同时另一方的会话正在使用 P"的情况。清单不授予提前进入锁或发布的资格；若两者误传同一个清单文件，后者独占创建失败，不能改写前者清单或目标。再用 A 已登记 P、B 的安装方式不同的组合，断言 B 以 `owned_by_other` 拒绝，且 P 未被改动。
  10. **安装方式暂停跨越原先的过期时间（ISO-ARCH-R4-F1 第五轮反例 2）**：A 在锁内暂停（SIGSTOP），用虚拟时钟推进 601 秒以上 → B 尝试登记 P、B 的客户端尝试开启会话：都拿不到锁，分别以 `owner_running`、`ownership_busy` 失败，没有任何一方改动 P 或读到中间状态 → A 恢复，在同一次持锁内完成。另测 A 在锁外暂停（发布之前尚未取锁）：此时 B 正常完成并开启会话；A 恢复后要先取排他锁，会等到 B 的会话结束或超时失败，期间不改 P。最后测 A 崩溃：内核释放锁，P 保持旧内容或新内容之一（原子发布），B 随后可以正常登记。
  11. **默认命令续登（ISO-ARCH-R4-F2）**：满足续登条件的新 keg，执行一条不带 `--state-dir` 的普通命令。断言：在选择目录之前完成续登，命令使用真实默认目录（注入的真实目录）中的原有数据，而不是空的隔离目录。续登取锁超时时，命令以 `state_ownership_refused`（`ownership_busy`）失败，不读写隔离目录。显式 `--state-dir <隔离目录>` 时不续登。
  12. **完整卸载链与同路径新安装（ISO-ARCH-R4-F1 第六轮）**：A 的 helper 在排他锁内清理旧 App 后返回 → B 发布并登记同一路径 → B 取得共享会话 → 继续 A 的全部卸载/回滚步骤。断言 B 的 App、回执逐字节不变，B 的会话仍有效；A 不存在锁外最终路径删除。A 的条件清理重试在 B 持锁时必须超时，B 退出后须因原清单 ID 不匹配而跳过。B 分别使用 `local_app` 和与 A 相同的 `homebrew_cask`，同版本重装也不能被旧清单误删。完整 Homebrew 分派由任务 5 验证。

  `state.lock` 的现有语义不变，交错测试不依赖它。
- 已安装身份下的现有测试全部通过，行为不变。

**验证：L3**。需要 targeted 测试和 Go 全量测试；涉及锁和 worker 子进程，还要对 store 和 scanruntime 跑 `-race`，并跑 vet。

### 3. `client-config-guard`

**依赖**：任务 1，与任务 2 顺序进行，因为两者都要改 `main.go`。**结果**：未安装进程不能写真实客户端配置（需求 G4，验收 A4）。

**文件**：
- `internal/platform/state.go`：`GuardClientWrite`；
- `internal/provider/config.go`、`internal/usagehook/config.go`、`internal/telemetry/codexconfig.go`、`internal/shellconfig/config.go` 及对应测试；
- 如果重新枚举时发现其他写入者，也一并纳入。

**要求**：
- 按[写入边界](architecture.md#真实客户端配置写入边界)在每个包唯一的写入入口加守卫。
- 重新枚举 `internal/` 和 `cmd/` 下所有写入账户主目录的函数，结论记录在评审材料中。
- 测试证明三点：目标位于注入的真实主目录时拒绝，且文件逐字节不变；目标位于临时 `HOME` 时允许；已安装身份下行为不变。
- 覆盖 provider 切换、`telemetry enable/disable --write-codex-config`、usage Hook 和 statusline 的 setup/restore、`shell setup/remove`。

**验证：L2**。targeted 测试和 Go 全量测试。

### 4. `installation-diagnostics`

**依赖**：任务 2。**结果**：用户能看到安装身份和受管命令的归属，App 能从 helper 获得状态目录（需求 G6、G8，验收 A3）。

**文件**：
- `internal/doctor/doctor.go` 及测试；
- `cmd/agentdeck/main.go` 中的 `newDoctorCommand` 及测试：增加状态目录被拒绝时的部分报告路径；
- `internal/desktop/desktop.go` 及测试，确认检查能传到 health 部分；
- `cmd/agentdeck/desktop.go` 及测试：新增隐藏命令 `desktop state-location`；
- `internal/usagehook` 中只读的受管命令枚举，如需导出；
- JSON 固定测试。

**要求**：
- 把 `installation` 检查放在第一个；`managed_command_owner` 检查只在已安装程序中运行。字段与 [ux/cli.md](ux/cli.md#doctor-的-installation-检查) 一致，不包含路径。文本中新增的检查行和附加行，与原型中五个 `doctor_*` 状态逐字一致。
- **部分报告的语义（ISO-UX-R3-F1）。** doctor 在状态目录上短路的两种情况都是部分报告，各自沿用自己的结尾一行：
  - 状态目录缺失：沿用现有的 `doctor.Service` 行为，`state_missing`，结尾一行为 `checks_skipped: remaining diagnostics require a readable core database`；对应 `doctor_not_installed` 和 `doctor_formula_unclaimed`；
  - 状态目录被拒绝：新的结尾一行，对应 `doctor_refused`。

  两种情况都是 `partial: true`、`warnings: ["checks_skipped"]`，并且只包含 `installation` 和 `state` 两个检查。新增 `installation` 检查不能改变现有缺失目录短路的部分报告语义；固定测试要覆盖"未安装 + 状态目录缺失"这一组合。
- 按[架构](architecture.md#doctor)实现 doctor 的部分报告：`stateRoot()` 返回 `ErrStateOwnershipRefused` 时，输出 `installation` 加上 `state` 的拒绝结果，`partial: true`、`checks_skipped`、exit 0，并且不把被拒绝的目录交给 `doctor.Service`。这条路径要在访问屏障下测试。
- `desktop state-location` 的输出按[架构](architecture.md#app-与小组件)定义；不是拥有者时，绝不返回真实路径。
- 固定测试覆盖：已安装和未安装两种身份的 doctor 文本与 JSON、部分报告、`managed_command_owner` 的三种情况，以及 state-location 的输出。

安装拒绝不写入 `hook-refusals.json`；schema-version-signal 的现有测试保持不变。

**验证：L2**，因为改的是线路和文本契约。需要 targeted 测试和 Go 全量测试。

### 5. `installer-receipts`

**依赖**：任务 1。**结果**：四种安装方式都通过唯一的用户回执取得所有权，并且只有在旧拥有者停止之后才允许接管（需求 G7、G9，验收 A2、A6、A7、A9）。

**文件**：
- `packaging/homebrew/agentdeck-app.rb.tmpl`、`packaging/homebrew/agentdeck.rb.tmpl`（formula 只改 caveats，不写回执）；
- `scripts/render-homebrew-cask.sh` 和 `scripts/render-homebrew-formula.sh`（如需修改）；
- `scripts/manage-install.sh`；
- 新建 `scripts/install-local-app.sh`；
- `Makefile`：新增 `install-macos-app` 及对应的卸载目标，增加 `TAKEOVER`；
- 新建 `cmd/agentdeck/install.go` 及测试：公开命令 `install claim`，隐藏命令 `install register`（带 `--source` 和 `--cleanup-manifest` 时在锁内发布文件或 App bundle）、`install unregister`（携带 channel、路径、`--expected-installation` 和 `--cleanup-manifest`，带 `--remove` 时在锁内条件删除）；
- `internal/installation`：在所有权排他锁内写入和删除回执、生成安装 ID、写本次清理清单并校验清理归属；
- `scripts/test-install.sh`、`scripts/test-cask-migration.sh` 及新增的本地 App 安装测试。

**要求**：
- 按[安装回执](architecture.md#安装回执)实现每种方式的写入、拒绝冲突和卸载清理。所有方式都调用暂存新程序的 `install register --source` 或 `install unregister --remove`。安装脚本自己既不写回执，也不把文件移到最终位置，也不写任何 keg 回执。
- 按 [formula 认领](architecture.md#formula-认领)实现 `install claim`：只能认领当前 opt 指向的 keg；回执登记 keg 内的真实路径，**不登记 opt 链接**；文本和 JSON 与原型中 `claim_*` 四个状态逐字一致。测试覆盖：旧 keg、非 opt 指向的 keg、两个前缀、stable 与 rc、`--takeover` 的成功和拒绝。这些测试都放在临时目录搭建的 Cellar/opt 布局中进行。
- 接管只以[所有权排他锁](architecture.md#所有权锁与交接)作为同步边界，不使用 `lsof`。测试中让一个替身进程持有共享锁来模拟旧拥有者的会话，断言 `TAKEOVER=1`、`install-macos-app` 和 `install claim --takeover` 都以 `owner_running` 拒绝，回执不变；替身释放锁之后，接管成功。
- **在锁内发布**：按[安装回执](architecture.md#安装回执)，由 `install register --source` 在一次持锁内完成原子发布和回执写入。单个文件用 `rename(2)`；App bundle 用 `renamex_np(RENAME_SWAP)` 交换，再删除旧 bundle；任何失败都不改目标路径和回执。`manage-install.sh` 原有的暂存、校验和清单逻辑保留，只把最后的重命名交给这条命令。测试中，替身进程持有共享锁时，发布以 `owner_running` 失败，目标和回执都不动；替身释放锁之后，一次完成。在发布与写回执之间注入失败，断言目标路径仍为原内容、回执未变。
- 回执写入前，要确认 `executable` 满足 `EvalSymlinks(executable) == executable`，并且就是正在安装或认领的程序；写入要原子化，权限为 `0600`。
- **完整清理链**：按[卸载与失败清理](architecture.md#卸载与失败清理)，正常卸载、升级卸载、安装失败回滚、重试和损坏安装清理全部经过带原清单 ID 的 `unregister`。清单在发布前由 helper 独占创建并同步落盘，位于最终目标之外；相同版本重装生成新 ID/新清单，旧清单不能被覆盖或从当前回执刷新。测试回执匹配时清理、路径/channel/ID 任一不匹配时跳过、回执和目标均不存在时幂等成功、回执缺失/损坏而目标存在时保留并报错。helper 缺失时使用私有保留副本；副本/清单不可用必须报告未完成，不能直接 `rm`。小组件注销也必须在归属匹配之后、同一次持锁期间执行。
- **回滚与失败注入**：成功登记后，在后续 Homebrew 步骤注入失败；回滚使用本次清单条件清理。登记前失败不能删除旧回执所属对象；释放锁后的恢复不能覆盖后来发布的对象。在 helper 返回、回滚回调开始及条件校验后分别暂停，插入同路径新安装，验证锁内串行或 ID 不匹配跳过。覆盖同 channel 同版本、不同 channel、清理重复调用、删除失败、清理 helper 缺失，以及 `--force` 跳过脚本的路径；删除失败保留残余和恢复材料并报告失败，不追加锁外兜底。
- **cask 改为由 install step 发布 App，不再使用 `app` 标签。** 以下各点都必须在真实 brew 回归测试中验证通过，任一项失败，都要停下来回到设计重新决定，不能静默改用别的方式：
  - install step 能写 `$HOME` 下的回执，也能写 `appdir`；
  - 不带 `app` 标签的 cask 能正常安装、升级、卸载，`binary` 和补全链接照旧；
  - 升级时，旧版本的 uninstall 步骤（`install unregister --remove`）先于新版本的 install step 执行；
  - `test-cask-widget-registration.rb` 的小组件注册仍然有效；
  - 真正卸载后不留回执，其他安装方式随后可以正常登记，不需要 `--takeover`。
  - 模板以及渲染后的 `uninstall`、`zap`、postflight、失败回滚中，没有能删除最终 App/父目录的 `delete`、`trash`、`rmdir` 或 shell 兜底。`binary` 仅删除链接本身，不跟随链接删 App。
  - 沿真实 Homebrew 完整分派执行任务 2 第 12 条；不能在 helper 返回时就停止测试。观察所有后续文件操作，确认新 App 和回执保留；同时证明旧 ID 的延迟回滚不会删除同 channel 的新安装。
  - Caskroom 的本次清单和可执行清理 helper 保留至全部清理调用结束；破坏最终 App 内 helper 后仍可在锁内正常清理。缺少私有副本或清单时，即使 Homebrew 强制跳过脚本也不删除最终 App，验收须报告残余而非卸载成功。
- 本地 App 安装沿用 ad-hoc 签名，不加 hardened runtime，并执行 `lsregister`。
- 所有测试都在临时 `HOME`、`PREFIX`、`APPDIR` 下进行。

**验证：L3**，因为改的是安装器行为。需要 `make check-install`、cask 真实 brew 回归测试、模板渲染测试，以及本地 App 安装测试。

### 6. `app-state-location`

**依赖**：任务 4。**结果**：App 不再自己推算状态目录；Health 面板显示安装检查（需求 G6）。

**文件**：
- `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift`；
- `apps/macos/AgentDeckApp/AgentDeckApp.swift`；
- `apps/macos/AgentDeckApp/DesktopCopy.swift`；
- 对应的 `AgentDeckTests`、`AgentDeckAppTests`，以及 `AgentDeckVerification/main.swift` 中现有的参数断言。

**要求**：
- 按[架构](architecture.md#app-与小组件)实现：每一轮刷新前查询 `state_root`，本轮内使用，不跨轮缓存。**只有当 helper 是拥有者、并且 `state_root` 指向真实状态时**，App 才用 `flock(2)` 取所有权共享锁，并在锁内再查询一次确认，然后持有到退出；条件不成立时不取锁，之前持有的立即释放（ISO-ARCH-R3-F1）。删除显式传入的 `--state-dir`，删除 `HOME/.agentdeck` 的推算。
- 读快照文件、等扫描锁、读 `receiver.json` 都改用本轮取得的路径；取锁超时或查询失败时，按 helper 不可用处理。测试覆盖以下几点：
  - 已安装的 App 运行期间，接管方拿不到排他锁；App 退出后接管成功；
  - **未安装的 App（例如 Debug App）单独运行时，安装、卸载、接管都能完成，不会出现 `owner_running`**；
  - App 取锁前后所有权发生转移时，锁内的再次查询发现条件不成立，立即释放锁；
  - 回执被手工删除之后，下一轮改用隔离目录并释放锁。
- 增加 `installation`、`managed_command_owner` 检查名，以及 `not_installed`、`managed_command_owner_mismatch` 说明的中英文文案。

**验证：L2**。需要 `make test-macos-app` 和 AgentDeckVerification。

### 7. `debug-app-identity`

**结果**：Debug App 与安装版完全分开（需求 G5，验收 A5）。

**人工前提**：在开发者账号下注册 `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug`。这一步由操作员完成，在评审材料中记录。

**文件**：
- `apps/macos/AgentDeck.xcodeproj/project.pbxproj`：只改 Debug 配置的 bundle ID 和 `AGENTDECK_APP_GROUP`；
- 检查小组件的 sandbox 校验脚本是否需要同步修改。

**要求**：
- 按[架构](architecture.md#app-与小组件)的表格修改，Release 和 Perf 配置逐项不变。
- 证明 Debug 构建产物的 entitlements 和 Info.plist 中都是 Debug 的 App Group。
- Release 构建的签名、`check-widget-sandbox` 都不变。

**验证：L3**，因为改的是构建。需要 Debug 和 Release 两种配置的构建、`make check-widget-sandbox`、`make test-macos-app`。Debug App 与安装版同时运行时互不影响，这一点由操作员人工验收，或者明确记录为豁免。

### 8. `ownership-acceptance`

**依赖**：任务 2–7。**结果**：在隔离环境中，验证需求的验收条件 A1–A9 能端到端成立。

**验收运行器**（ISO-TASKS-R1-F1）：按[测试与验收的隔离](architecture.md#测试与验收的隔离)使用验收二进制，构建命令为 `go build -tags agentdeck_acceptance -o <临时目录>/agentdeck ./cmd/agentdeck`。`AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 指向临时账户主目录，回执、注入的真实状态目录、客户端配置都在这个目录下；同时使用临时 `HOME`、`PREFIX`、`APPDIR`。scan worker 和 helper 是同一个可执行体，并继承这个环境变量。

**要求**：
- 按以下顺序走一遍：
  1. 未安装：在访问屏障下运行，确认没有访问真实目录、客户端配置的写入被拒绝；
  2. 用本地 App 安装方式把验收二进制装入临时 `APPDIR`：确认成为已安装，能迁移注入的真实目录；
  3. 用另一个程序尝试接管：确认被拒绝；
  4. 旧拥有者有一个会话正持有所有权共享锁（例如一个 `watch` 进程）时加 `TAKEOVER=1`：确认以 `owner_running` 拒绝，回执不变；
  5. 停止旧拥有者后加 `TAKEOVER=1` 接管：确认成功，旧可执行文件随后在屏障下被拒绝；
  6. 在临时 Cellar/opt 布局中执行 formula `install claim`、旧 keg 认领、`--takeover`；然后模拟一次升级：建立新 keg，把 opt 切到新 keg，删除旧 keg，然后用新 keg 执行一条**不带 `--state-dir` 的普通命令**，确认它在排他锁内续登，并读到注入真实目录中的原有数据；再模拟一次保留旧 keg 的升级，确认不续登、新 keg 按未安装处理；
  6a. 用一个未安装的 App 替身持续运行（它的 helper 不是拥有者），确认此时安装、接管都能完成，不会出现 `owner_running`；
  7. 卸载：确认回执被清理；
  8. 损坏回执：确认 `doctor` 给出原因；`doctor --state-dir <注入的真实目录>` 输出部分报告；
  9. 运行 schema-version-signal 的回归测试，确认仍然通过（A8）。
- 生产产物只做有限的静态验证：`make build-all` 的产物中没有验收 Resolver 的符号，构建命令中没有 `agentdeck_acceptance`。它的运行时行为由验收二进制和包内测试覆盖，这一点在评审材料中明确写出，不能声称已用生产产物做过运行时验收。
- 用真实本机安装做验收会触碰真实状态，不在本任务内执行。在交付说明中列为操作员事项：用 `make install-macos-app` 重装本机；formula 用户需要执行一次 `install claim`。

**验证：L3**。记录隔离证据，并区分三类：访问屏障下的证据、验收二进制的证据、生产产物的静态证据。

### 9. `contract-reconciliation`

**依赖**：任务 1–8。**结果**：产品契约与实现一致。

**文件**：`docs/specs/cli-design.md`、`docs/specs/cli-manual.md`、`README.md`。

**要求**：按[兼容性与契约同步](architecture.md#兼容性与契约同步)更新 Local State、Build Identity and Installation、Output and Errors、Doctor 这几节，以及安装和故障排查说明。

**验证：L0**。需要 `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh installed-state-ownership`、`make check-whitespace`、`git diff --check`。
