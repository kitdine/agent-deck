---
status: active
created: 2026-11-05
updated: 2026-11-09
---

# Installed State Ownership — Architecture

依赖 [需求](requirements.md) 与 [CLI 界面](ux/cli.md)。下文的现状说明都在 `660c815`（`main`，schema 33）上逐条核对过；`文件:行号` 指这个提交里的位置。

## 总体结构

```text
cmd/agentdeck main
  └─ 安装 Resolver 注入 platform（进程级；未注入时按未安装处理）
       ├─ 状态目录解析        stateRoot()/shellStateRoot() → installation.StateRoot
       ├─ 状态访问守卫        platform.GuardStateRoot：持有所有权共享锁 → 在锁内判定 → 返回实际要用的物理路径
       ├─ 长驻进程复核        platform.RevalidateOwnership：每一轮复核，失去所有权就退出
       └─ 客户端配置写入守卫  platform.GuardClientWrite
App (Swift)
  └─ 每轮：helper `desktop state-location` → 文件探测；仅当服务于真实状态时持有共享锁到退出
安装方式 (cask / make install / local App) 与 `install claim`（formula）
  └─ 暂存新程序的 `install register [--source]`：取得所有权排他锁 → 在锁内发布文件 → 写唯一的用户回执 → 释放
     文件的发布和删除都只在这一次持锁内完成，没有跨步骤的中间状态
formula 升级：新 keg 首次访问时在排他锁内续登（旧 keg 已删除、同 formula 同前缀）
所有权锁  account/Library/Application Support/AgentDeck/ownership.lock（在真实状态目录之外）
```

判定只在运行时进行，依据只有用户回执和当前进程的可执行文件路径。版本号、`buildinfo`、环境变量、命令参数都不影响判定结果（需求 G1）。

## 安装身份判定

新增包 `internal/installation`，对外提供：

```go
type Identity struct {
    Installed bool
    Channel   string // homebrew_cask | homebrew_formula | make | local_app；仅 Installed
    Reason    string // receipt_missing | receipt_invalid | receipt_other_executable | executable_unresolved | real_root_unresolved；仅未安装
}
func Resolve() Identity
```

判定步骤，任何一步失败都返回未安装：

1. 用 `os.Executable()` 取得自身路径，再经 `filepath.EvalSymlinks` 得到真实路径 `self`。失败时原因为 `executable_unresolved`。通过 `/usr/local/bin/agentdeck → /Applications/AgentDeck.app/Contents/Helpers/agentdeck` 这类链接调用时，`self` 是链接指向的 App 内 helper。
2. 用 `os/user.Current().HomeDir` 取真实账户主目录 `account`，这是系统账户数据库里的值，不读 `$HOME`。测试和夹具会改写 `HOME`，但改写后不应因此获得或失去安装身份。
3. 读取唯一的用户回执 `account/Library/Application Support/AgentDeck/installation.json`：
   - 不存在：未安装，原因为 `receipt_missing`。
   - 存在但无效：未安装，原因为 `receipt_invalid`。
   - 有效：`executable` 是写入时已经解析好的真实路径，判定时**不再解析任何链接**，直接比较。
     - `Lstat(executable)` 报告不存在（登记的程序已被删除）：视为过期回执，原因为 `receipt_missing`。
     - `executable == self`：已安装。
     - 否则：未安装，原因为 `receipt_other_executable`。

这里只有一个权威。没有 keg 回执，也没有任何程序自带的标记（ISO-ARCH-R1-F1），所以保留下来的旧 keg、其他 Homebrew 前缀、同时安装的 stable 与 rc formula，都不会与回执登记的那一个同时成为拥有者。

**回执登记的是不可变的安装对象，不是会被别的操作改变指向的链接（ISO-ARCH-R1-F2 第三轮）。** Repair 2 曾让 formula 登记 opt 链接，并在判定时实时解析。复评证明这样不安全：`brew upgrade` 会在旧会话仍持有共享锁时改变 opt 的指向，新 keg 无需排他锁就被认作拥有者。现在改为：

- formula 登记的是 keg 内的真实路径 `<prefix>/Cellar/<formula>/<version>/bin/agentdeck`。
- 回执所指的对象只能通过两种方式改变：一是在所有权排他锁内改写回执，二是删除被登记的文件。删除只会让回执过期、减少拥有者，不会产生新的拥有者。
- 所以安装操作里，凡是要在"可能被登记或已被登记的路径"上创建或替换文件的，都必须由持有所有权排他锁的那个进程亲自完成，并在释放锁之前更新回执，见[安装回执](#安装回执)。
- formula 升级产生的是新路径，由下文的[升级续登](#formula-升级续登)在排他锁内完成切换。

**只比路径，不比哈希。** statusline 和 Hook 调用得很频繁，每次对 30 MB 的二进制算哈希代价太高。受支持的安装方式都在排他锁内发布文件，所以登记路径上的文件只会在没有任何访问会话、也没有其他安装方式能介入的时候被替换。不经过安装方式、手工覆盖已安装文件的做法不受支持，不在本设计的保护范围内。`make uninstall` 原有的哈希校验不受影响。

**不永久缓存（ISO-ARCH-R1-F2）。** 判定结果不能作为进程内的永久事实，只用于下面两件事：

1. 在 TTY 提示、`doctor` 和 `desktop state-location` 中展示当前身份；
2. 选择默认状态目录。

访问真实状态的资格只在持有[所有权锁](#所有权锁与交接)期间、在锁内判定。判定只需要读一个不超过 4 KiB 的文件并解析两个路径，单次开销远小于打开数据库。

### 回执格式

```json
{"receipt_version": 1, "executable": "/Applications/AgentDeck.app/Contents/Helpers/agentdeck", "channel": "homebrew_cask", "installed_at": "2026-11-05T12:00:00Z", "installation_id": "c91ff1149478459e85c530791b876f65"}
```

Repair 4 曾为预留增加过 `state` 字段和预留记录，Repair 5 改为在锁内发布之后，它们已全部删除。回执只有上面这一种记录。

规则：

- `receipt_version` 必须是 `1`；
- `executable` 必须是绝对的真实路径：写入时要满足 `EvalSymlinks(executable) == executable`，并且它就是正在登记的那个程序。读取时，不满足"绝对路径、已 `Clean`"的回执一律视为无效；
- `channel` 只能取四个枚举值之一；
- `installation_id` 是每次发布、接管或 formula 认领/续登时新生成的 128-bit 随机值，编码为 32 位小写十六进制；同路径、同 channel、同版本重装也必须换值。它只用于[清理归属校验](#卸载与失败清理)，不参与 `Resolve` 的路径判定，不是另一份安装权威，也不是预留或有期限的凭据；
- 出现未知字段视为无效；
- 文件大小上限 4 KiB，权限为 `0600`，所在目录权限为 `0700`；
- 用"写临时文件再重命名"的方式原子写入。

读取时不跟随回执文件本身的符号链接（`O_NOFOLLOW`）。

## 状态目录解析

现状：`commandOptions.stateRoot()` 和 `shellStateRoot()`（`cmd/agentdeck/main.go:1699`、`:1994`）用 `--state-dir` 或 `platform.StateRoot("", home)`，也就是 `$HOME/.agentdeck`（`internal/platform/state.go:48`）。

新的 `installation.StateRoot(override string) (string, error)` 统一替代这两处：

选择默认目录之前，先处理续登（ISO-ARCH-R4-F2）。没传 override 时，`StateRoot` 按以下顺序执行：

1. 运行 `Resolve`。已安装，就使用真实默认目录。
2. 未安装时，检查 [formula 升级续登](#formula-升级续登) 的条件。这一步只读回执、opt 链接和自身路径，不访问 `P`。
3. 条件成立，就执行续登：在排他锁内重读条件后写回执。然后重新运行 `Resolve`；这次判定为已安装，就使用真实默认目录。续登失败（例如取锁超时，原因为 `ownership_busy`），这次调用不退回隔离目录，而是返回 `state_ownership_refused`。理由是：这个程序就是用户升级后的安装，它本该看到原有数据；如果悄悄改用一个空的隔离目录，会让数据看起来"丢失了"，下一次调用会再试。
4. 条件不成立，才使用隔离目录。

| 身份 | 未传 override | 传了 override |
| --- | --- | --- |
| 已安装（包括刚续登成功的） | `$HOME/.agentdeck`，与现状一致 | override，与现状一致 |
| 未安装，但满足续登条件 | 先续登，成功后按已安装处理；失败则返回 `state_ownership_refused`，不退回隔离目录 | override 不指向真实状态时，直接使用，不续登，尊重显式的隔离意图；指向真实状态时，由[状态访问守卫](#状态访问守卫)尝试续登 |
| 未安装，不满足续登条件 | `$HOME/.agentdeck-dev` | override；如果它指向真实状态目录，返回 `ErrStateOwnershipRefused{resource: state_root}` |

所以续登有两个入口：未传 override 的默认解析，以及显式指向真实状态的守卫分支。两者调用同一个续登函数。普通命令、Hook 和 App 的 helper 都不传 `--state-dir`（App 已在本主题中改为不传），它们都走第一个入口。

这里的 `$HOME` 指 `os.UserHomeDir()`，沿用现状，尊重测试对 `HOME` 的覆盖。所以测试中未安装进程的默认目录是 `<临时 HOME>/.agentdeck-dev`，不会落到真实账户主目录。

所有未安装的进程共用同一个 `~/.agentdeck-dev`。不同主题的开发构建如果 schema 不同，在这个目录里也会触发 `schema_ahead`。这只影响开发数据，属于预期情况；需要隔离时，可以显式指定 `--state-dir`。

### 什么算"指向真实状态目录"

真实目录和候选目录用**同一个物理解析函数**处理（ISO-ARCH-R1-F3）。这个函数**从不在解析符号链接之前做词法 `Clean`**（ISO-ARCH-R2-F1）：`a/link/..` 的含义是"link 指向的目录的父目录"，不是 `a`。先 `Clean` 会让判定的路径和操作系统实际访问的路径不一致。

**`walk(p, P)`**：按 POSIX 的物理语义逐个分量解析，返回解析后的物理路径或拒绝。

1. `p` 必须是绝对路径；相对路径先拼到当前工作目录上，不做 `Clean`。从 `/` 开始，令 `cur` 为已解析的物理前缀。
2. 依次取每个分量 `c`：
   - `c` 为空或 `.`：跳过。
   - `c` 为 `..`：`cur` 改为 `cur` 的物理父目录。`cur` 本身已经是物理路径，所以这一步只是去掉最后一个分量，结果与操作系统一致。
   - 否则令 `next = cur/c`。**先检查 `next` 是否等于 `P` 中任一路径，或位于其下；是则立即拒绝，不再访问它。** 否则对 `next` 执行 `Lstat`：
     - 是符号链接：`Readlink` 后，把链接目标与剩余分量拼成新路径（相对目标以 `cur` 为基准），从头继续解析。链接展开累计超过 40 次，按链接循环处理。
     - 是已存在的目录或文件：`cur = next`。
     - 不存在：剩余分量里不能再有 `..`（缺失路径之后的 `..` 没有确定的物理含义），否则拒绝；剩余分量按词法拼到 `next` 后面，结束解析。
     - 其他错误（例如权限不足）：拒绝。
3. 最终路径再与 `P` 比较一次（相等或位于其下即拒绝），然后返回这个路径。

**受保护集合 `P`**，由同一套物理解析得到，只计算一次：

- `E = phys(account)/.agentdeck`：真实目录的入口。`phys(account)` 是对账户主目录做同样的逐分量物理解析（主目录不在 `P` 中）；
- 如果 `E` 存在，再沿着它的符号链接链逐级解析，链上每一个入口，以及最终的目标目录 `T`，都加入 `P`。计算 `P` 时，对这些入口只做 `Lstat` 和 `Readlink`，不列目录、不打开。

拒绝的原因：

- 计算 `P` 时出错（账户主目录解析失败，或入口链上出现非"不存在"的错误）：`real_root_unresolved`。这时未安装进程拒绝使用任何显式指定的状态目录；它的默认隔离目录也要经过同样的判定。
- 候选路径命中 `P`：`receipt_*` 这类安装身份原因。
- 候选路径本身无法安全解析（缺失路径之后有 `..`、链接循环、权限错误）：`state_path_unresolved`。

**判定的路径就是实际访问的路径。** `GuardStateRoot` 返回物理路径，调用者之后的所有文件操作都必须使用这个返回值，不能再使用原始字符串。因此，"判定在根外、实际访问在根内"的情况不会出现。

只有当候选路径与 `P` 不相交，或者只是 `P` 的祖先目录时，才允许使用。例如 `--state-dir ~` 会在 `~` 下直接建库，不触碰 `~/.agentdeck`。

本设计防的是误用，不防同一用户在判定和打开之间故意替换符号链接；后者不在本主题范围内。

必须覆盖的反例：

- 真实目录本身是符号链接：候选路径写成链接入口、链接目标，或它们下面的子路径；
- **链接后接 `..` 回到真实目录**：评审给出的 `outside/jump → account/inner` 且候选为 `outside/jump/../.agentdeck/sentinel`。物理解析时，`jump` 先解析为 `account/inner`，`..` 得到 `account`，下一个分量 `.agentdeck` 命中 `E`，在访问它之前就被拒绝；
- 真实目录的某个祖先是符号链接；
- 真实目录尚不存在；
- 缺失路径之后有 `..`；
- 链接循环；
- 权限错误。

## 状态访问守卫

新增 `platform.GuardStateRoot(root string) (string, error)`：

1. 用[物理解析](#什么算指向真实状态目录)得到路径。路径不命中 `P` 时，直接返回该路径；这时不需要判定身份，也不取所有权锁。
2. 路径命中 `P` 时：
   - 未安装进程，直接拒绝，**在访问 `P` 中任何路径之前**。唯一的例外是满足 [formula 升级续登](#formula-升级续登) 条件的新 keg：它先尝试续登，续登只读写回执和所有权锁，不访问 `P`；续登成功后，按下一种情况继续；
   - 否则进入[所有权会话](#所有权锁与交接)：取得共享锁后，在锁内运行 `Resolve`。只有判定为已安装才返回路径；否则释放锁并拒绝，`reason` 取这次判定的原因。

它放在包级函数里，而不是只放在 CLI 层，原因有两个：防止有入口绕过 `stateRoot()`；并且没有注入 Resolver 的库调用者（例如测试）默认按未安装处理，同样受保护。

现有的 `state.lock` 不能用来串行交接（ISO-ARCH-R1-F2 复评已指出）：`openAtExistingRoot` 在函数返回时就释放了它（`internal/store/store.go:376-399`），返回的 `Store` 并不持有这把锁；`OpenReadOnly` 也完全不取锁（`:331-346`）。因此 `state.lock` 的语义保持不变，交接改由下面的所有权锁负责。

### 所有权锁与交接

<a id="交接前提"></a>原先的"交接前提"一节（用 `lsof` 检查进程）已由本节取代；保留这个锚点，是为了让评审记录里的历史链接仍然可用。

所有权锁是 `account/Library/Application Support/AgentDeck/ownership.lock`。它与回执放在同一目录，位于真实状态目录之外，所以接管方作为未安装进程也可以操作它。使用 `flock(2)` 咨询锁：持有者进程退出时，内核自动释放，不会留下需要人工清理的过期锁。

**所有权会话（共享锁）。** 已安装进程第一次访问真实状态之前，按顺序执行：

1. 取得共享锁。等待上限 5 秒，超时即拒绝，`reason` 为 `ownership_busy`。
2. 在锁内运行 `Resolve`，确认自己是拥有者。
3. 此后持有共享锁，直到它对真实状态的全部访问结束：关闭所有句柄、释放 `state.lock` 和其他锁、结束文件读取。

覆盖范围：

| 访问方 | 何时取得共享锁 | 何时释放 |
| --- | --- | --- |
| 单次命令和 Hook（读、写都算，包括 `OpenReadOnly`） | 第一次访问真实状态之前 | 进程退出 |
| scan worker | 同上；worker 是独立进程，自己取锁 | 进程退出 |
| 长驻进程（`watch`、telemetry 监听、desktop bootstrap 流） | 每一轮开始前；同时调用 `RevalidateOwnership` | 每一轮结束、关闭本轮句柄之后；如果句柄要跨轮保留，就一直持有，直到进程退出 |
| App（Swift），只限服务于真实状态的 App | 只有确认自己的 helper 是拥有者、并且 `state_root` 指向真实状态之后，才取共享锁，见 [App 与小组件](#app-与小组件) | App 退出时，或某一轮复核发现自己已不是拥有者时。需求 G9 规定"已安装的 App 仍在运行时，接管会被拒绝"，所以服务于真实状态的 App 在整个运行期间都持有共享锁 |
| 未安装的 App（例如从 DerivedData 运行的 Debug App） | 不取 | — 它只使用隔离目录，不访问真实状态，所以不阻止安装、卸载或接管（ISO-ARCH-R3-F1） |

**接管（排他锁）。** 所有写回执的路径都通过新程序的隐藏命令 `agentdeck install register`；`install claim` 是在它之上加了 formula 检查的公开入口。写回执按以下顺序：

1. 以非阻塞方式重试取得排他锁，总等待上限 5 秒。取不到，说明仍有拥有者的进程在访问真实状态：拒绝，`reason` 为 `owner_running`，回执不变。
2. 在排他锁内检查现有回执，按[安装回执](#安装回执)的规则决定拒绝还是写入。如果带有 `--source`，就先在锁内原子地发布文件，然后原子写入新回执。
3. 释放排他锁。发布文件和写回执都发生在同一次持锁之内。

**这样得到的保证：**

- 共享锁和排他锁互斥。一旦接管方拿到排他锁，就说明此刻没有任何进程正处在访问真实状态的会话中。
- 接管方释放锁之前，回执已经换成新的。之后任何进程，包括被 Hook 启动的旧可执行文件，取得共享锁后读到的都是新回执，旧拥有者因此在访问之前就被拒绝。

评审给出的时序"B 的检查没看到 A → Hook 启动 A → A 读到旧回执并打开 → B 写新回执 → A 继续"不再可能发生：

- 如果 A 先拿到共享锁，B 的排他锁就会一直等到 A 的整个会话结束；超过上限，B 以 `owner_running` 拒绝。
- 如果 B 先拿到排他锁，A 会在取共享锁这一步等待，之后读到新回执而被拒绝。

**长驻进程。** 新增 `platform.RevalidateOwnership(root) error`，在每一轮开始时调用：在共享锁内重新判定，失败时关闭句柄、释放锁，以 `state_ownership_refused` 退出。持锁跨轮的进程会让接管一直被拒绝，直到它退出；这正是需求 G9 要求的"停机交接"：用户需要先退出 App、停止 `watch` 或监听。

**残余风险。** macOS 的 `flock` 没有写者优先。Hook 被频繁调用时，排他锁可能在 5 秒内一直拿不到，接管方会收到 `owner_running`，提示用户暂停客户端后重试。结果只会是拒绝接管，不会出现两个拥有者。

调用点放在第一次文件系统操作之前。以下清单由实现任务按"所有接收 `stateRoot` 参数、并会创建、打开、加锁或删除文件的导出函数"重新枚举并全部覆盖：

| 位置 | 现状 |
| --- | --- |
| `platform.EnsureStateRoot` / `EnsureStateRootWithFS`（`internal/platform/state.go:57`） | 有 9 个调用方，包括 store、credentialvault、scanruntime、`cmd/agentdeck` |
| `store.open` / `OpenExisting` / `OpenReadOnly` / `OpenWithLockHeld` / `OpenSessions`（`internal/store/store.go:280-374` 等） | `openAtExistingRoot` 在 `:422` 处直接迁移 |
| `store.AcquireLock` / `AcquireScanLock` 以及 `schemaAheadAtRest` | 加锁前会创建锁文件 |
| `scanruntime.prepareStateRoot`（`internal/scanruntime/scanruntime.go:1549`）以及 worker 的启动路径 | worker 以 `--state-dir` 启动（`:365`），是同一个可执行文件，在子进程里同样经过守卫 |
| `credentialvault`、`hookrefusal`、telemetry receiver 配置的读写 | 都位于状态目录内 |

`ErrStateOwnershipRefused` 映射为错误码 `state_ownership_refused`，退出码 `1`，`details` 为 `{"resource","reason"}`。文本渲染按 [ux/cli.md](ux/cli.md#拒绝真实状态目录) 的样例，在 `renderCommandErrorText`（`cmd/agentdeck/main.go:558`）旁边新增一个分支。

## 真实客户端配置写入边界

新增 `platform.GuardClientWrite(path string) error`。未安装进程要写的目标，经上文的[物理解析](#什么算指向真实状态目录)后位于 `phys(account)` 之下时，就拒绝写入（`resource: client_config`）。写入同样使用解析后返回的路径。

覆盖的是 AgentDeck 会写入的全部客户端和 shell 配置，在各包唯一的写入函数入口处调用：

| 写入者 | 位置 |
| --- | --- |
| Codex/Claude provider 配置 | `provider.WriteCodexConfig`、`WriteOfficialCodexConfig`、`WriteCodexWrapperConfig`、`WriteClaudeConfig`（`internal/provider/config.go:332`、`:615`、`:666`、`:745`） |
| usage Hook 与 statusline | `usagehook.Manager.writeAtomic`（`internal/usagehook/config.go:1660`），它是这个包所有写入的汇合点 |
| Codex OTel 配置块 | `telemetry.WriteManagedBlock`、`RemoveManagedBlock`（`internal/telemetry/codexconfig.go:133`、`:225`） |
| shell rc 文件 | `shellconfig.Manager` 的原子写入路径：`writeTemporary`（`internal/shellconfig/config.go:777`）以及随后提交到目标文件的重命名 |

`660c815` 上已核对两点：`internal/extension` 没有直接写文件的调用；`backup.Restore`（`internal/backup/backup.go:234`）写入的是状态目录 `targetRoot`，不是客户端文件，所以由状态访问守卫覆盖。尽管如此，实现任务仍要重新枚举 `internal/` 和 `cmd/` 下所有写入账户主目录的函数，清单之外的写入者同样必须加守卫。

读取不受影响，包括扫描时读取的 `~/.claude/projects`、`~/.codex/sessions`，以及只读的 drift 检查。写入状态目录内部的文件（例如 provider 备份）由状态访问守卫负责，不归这里。

**测试的影响。** 测试会把 `HOME` 改成临时目录，但守卫比较的是真实账户主目录，所以测试写临时 `HOME` 下的文件仍被允许。要证明守卫确实生效，需要把"真实账户主目录"替换成一个临时目录，方法见[测试与验收的隔离](#测试与验收的隔离)。

## 测试与验收的隔离

生产代码判定身份时只看系统账户数据库和用户回执，不接受任何覆盖。要在不触碰本机真实账户状态的前提下测试和验收，分三层（ISO-TASKS-R1-F1）：

| 层 | 可执行体 | 账户主目录和回执的来源 |
| --- | --- | --- |
| 包内单元测试 | `go test` 测试二进制 | `internal/installation` 和 `internal/platform` 的未导出 Resolver 字段，只能在 `_test.go` 中替换 |
| 端到端验收 | 带 build tag `agentdeck_acceptance` 构建的验收二进制，例如 `go build -tags agentdeck_acceptance -o <临时目录>/agentdeck ./cmd/agentdeck` | 只有这个 tag 下才会编译 `resolver_acceptance.go`，它从 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 读取临时账户主目录，回执路径随之落到该目录下。这个变量未设置时，才读 `HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME`：`brew` 只把 `HOMEBREW_*` 变量传给 cask 步骤，CI 的真实 brew 回归靠它隔离。两个都未设置或不是绝对路径时，账户解析失败，不回退到真实账户。scan worker 和 helper 都是同一个可执行体，并继承这些环境变量，所以子进程同样被隔离 |
| 生产产物 | `make build`、`make build-all`、App 打包、formula 和 cask 的产物 | 不带这个 tag。只做有限的静态验证：用 `go tool nm` 确认产物中没有验收 Resolver 的符号，并确认构建命令中没有这个 tag。生产产物的运行时行为由前两层覆盖 |

验收 tag 只是测试工具，不是生产后门：

- 发布构建、Makefile 的 `build` 和 `build-all`、App 打包脚本、Homebrew 模板都不得带这个 tag；
- 上面的符号检查加入 `check-install` 和发布前检查；
- 会执行安装的测试脚本在安装前，用同样的符号检查确认被安装的程序是验收版，不是就中止。发布产物无法改用临时账户，所以发布验证不对发布产物执行安装冒烟。

**证明"没有访问"，而不只是"没有写入"（ISO-TASKS-R1-F2）。** 文件清单、内容和 mtime 都不变，只能证明没有写入，不能证明没有读取。因此，受保护路径的验收要在访问屏障下运行。

**被禁止的和被允许的。** 需求 G3 禁止未安装进程读写真实状态，指的是状态目录里的内容。物理解析为了识别真实目录，必须读取入口本身的元数据（复评已指出，Go 的 `filepath` 逐分量解析时会调用 `os.Lstat`）。因此规则是：

| 对象 | 允许 | 禁止 |
| --- | --- | --- |
| `P` 中每个入口本身（`E`、链上的入口、目标 `T`） | 只读元数据：`lstat`、`stat`、`readlink`（`file-read-metadata`），用于判断它是什么、指向哪里 | 打开、列目录、读内容、创建、删除、改名、改权限 |
| `P` 中任一入口**之下**的任何路径 | 无 | 一切访问，包括 `stat` |

物理解析的设计保证了守卫只会用到"允许"一列：它在计算 `P` 时只对入口做 `Lstat` 和 `Readlink`，在解析候选路径时只要碰到 `P` 就先拒绝，不会往下走。

**屏障配置：**

```text
(version 1)
(allow default)
(deny file-read* file-write* (subpath "<E>") (subpath "<T>") (with send-signal SIGKILL))
(allow file-read-metadata (literal "<E>") (literal "<T>"))
```

链上如果还有中间入口，给每个入口都各加一条 `subpath` 和 `literal`。

规则如下：

- **正向对照，证明守卫能在屏障内正常工作。** 每次运行先执行一次真实的"物理解析 → 守卫 → 拒绝"：用待测二进制，以未安装身份、`--state-dir` 指向注入的真实目录，运行一条不需要状态的命令。它必须以 `state_ownership_refused` 结束，不能被信号终止。这一步证明屏障允许的元数据访问已经足够让守卫完成判定。
- **阴性对照，证明屏障确实生效。** 同一配置下，故意执行一次目录内部的访问，例如 `ls <T>`，进程必须被终止。
- **断言**：每条被测命令都以 `state_ownership_refused`（或 doctor 的部分报告）结束，不被信号终止。子进程继承屏障；包级直接调用的测试，在同样配置下运行对应包的测试二进制。
- **屏障不可用时测试失败。** `sandbox-exec` 缺失，或者 `sandbox_apply` 失败（例如在一个已经运行于沙箱中的客户端里嵌套调用，返回 exit 71），测试都报告为失败并写明原因，不能降级为只做"无写入"断言。这类测试必须在没有外层沙箱的环境中运行，例如本机终端、CI 的 macOS runner，或者经批准在客户端沙箱外执行。

已安装身份下迁移注入的真实目录这类正向路径，不使用这个配置。

可行性：2026-11-05 在本机 macOS（Darwin 25.6.0）、没有外层沙箱的环境中，用临时目录验证过这个配置，夹具为 `acct/.agentdeck → moved`：

- 对入口 `lstat`、`readlink`、对目标 `stat`，都放行（exit 0）；
- 列出目标目录、`stat` 目录内的文件、经入口读取文件、在目录内写入，进程都被终止（exit 137）；
- 访问无关目录不受影响。

Round 2 评审在嵌套沙箱中得到 exit 71，与上述"嵌套时不可用"的结论一致。

## 安装回执

用户回执写在 `$HOME/Library/Application Support/AgentDeck/installation.json`，这是唯一的一份。安装程序以当前用户身份运行，它看到的 `$HOME` 与 Homebrew 的 `Dir.home` 一致，正常情况下等于真实账户主目录。

**文件发布和回执写入在同一把排他锁内完成（ISO-ARCH-R4-F1 第五轮）。** 规则是：任何可能成为或已经是回执登记对象的路径，其上的文件只能由**当前持有所有权排他锁的那个进程**来创建、替换或删除，并且同一个进程在释放锁之前完成回执的更新。唯一的例外是 Homebrew 在锁外删除旧的 formula keg：删除只会让回执过期、减少拥有者，不会产生新的拥有者，见[安装身份判定](#安装身份判定)。所有安装方式都不自己移动文件到最终位置，也不自己写回执，一律调用刚装好的新程序（暂存在临时位置）：

```text
<暂存的新程序> install register --channel <c> --executable <目标路径> [--source <暂存的文件或 App> --cleanup-manifest <本次安装的清理清单>] [--takeover] [--require-unowned [--authorized-repair [--expected-installation <原 ID>]]]
<可信的程序> install unregister --channel <c> --executable <目标路径> --expected-installation <id> --cleanup-manifest <本次安装的清理清单> [--remove] [--authorized-repair] [--require-identity]
<固定 App helper> install repair-identity --channel homebrew_cask --executable <固定 App helper>
```

`install register` 在一个进程内、一次连续持有排他锁，依次执行：

1. 以非阻塞方式重试取排他锁，总等待上限 5 秒。取不到就以 `owner_running` 失败，什么都不改。拿到锁，说明此刻没有任何访问会话。
2. 在锁内检查现有回执，决定是否允许：
   - 没有回执、回执无效、回执过期，或回执登记的就是 `<目标路径>` 且安装方式相同（同一安装方式的重装或升级）：允许；无效回执的恢复仍须满足安装方式已有的目标替换授权（make 与本地 App 为 `FORCE=1`；cask 没有 `FORCE`，它的对应授权见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)），清理命令本身不因此获得删除权；
   - 回执登记的是 `<目标路径>`，但安装方式不同：没有 `--takeover` 时以 `owned_by_other` 拒绝；
   - 回执登记的是另一个仍存在的路径：没有 `--takeover` 时以 `owned_by_other` 拒绝。
3. 为这次登记生成新的 `installation_id`。传了 `--source` 时，先在锁内写好本次安装的清理清单（见[卸载与失败清理](#卸载与失败清理)），再把暂存的内容原子地发布到 `<目标路径>`。暂存内容事先放在目标的同一目录、同一文件系统里。
   - 单个文件：用 `rename(2)` 覆盖；
   - App bundle（目录）：用 `renamex_np(RENAME_SWAP)` 与旧 bundle 原子交换，再删除换下来的旧 bundle。
   - 任何一步失败，目标路径保持原样，回执也不改，然后释放锁并报错。
4. 确认 `EvalSymlinks(<目标路径>) == <目标路径>`，并且它就是刚发布的程序，然后原子写入含本次 `installation_id` 的回执。
5. 释放排他锁。

这些命令都是隐藏命令，只供安装方式调用，都不访问真实状态目录；冲突和取锁失败使用 `install_claim_refused` 及已有原因码。清理时无法确认归属的结果见下节。安装脚本把失败原样转给用户，并以非零状态结束。`--require-unowned` 与 `--authorized-repair` 只由 cask 使用，含义见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)；其他安装方式不传它们，行为不变。

**为什么这样能闭合复评的两个反例。**

- **没有需要区分身份的跨步骤状态。** 发布与登记是同一个进程、在同一次持锁内完成的，不存在"先撤销、稍后再由另一个命令完成"的中间状态。第二个安装方式即使用完全相同的参数调用 `install register`，也只能在第一个释放锁之后进入。那时它看到的是第一个已经完成的正式回执，按第 2 步的规则判定；如果它被允许覆盖，也是在它自己持有排他锁、没有任何会话的时候覆盖。所以不需要预留，也不需要事务凭据。
- **没有基于时间的过期。** 不存在会过期的预留，也就不存在"暂停后恢复的旧安装器"在别人登记之后才动手的窗口：安装器的发布动作只能发生在它持有排他锁期间。它被暂停时，仍然持有锁，其他进程都进不来，恢复后在同一次持锁内完成；它崩溃时，内核随进程退出释放锁，而一个不持锁的进程不会去改目标文件。Repair 4 的预留记录、`state` 字段和 10 分钟过期都已删除。

### 卸载与失败清理

**完整清理链都必须遵守同一边界（ISO-ARCH-R4-F1 第六轮）。** 删除目标 App 的唯一入口是带预期安装身份的 `install unregister --remove`。正常卸载、升级旧版本卸载、安装失败后的 Homebrew 回滚、重试与损坏安装清理都使用它；不在 helper 返回后再对最终路径执行 `rm`、`delete`、`trash`、`rmdir` 或恢复旧 bundle。后续清理可以删除本次安装私有的暂存文件，不能跟随链接去删最终路径。

**清理清单固定“要清理哪一次安装”。** `register --source` 必须同时传 `--cleanup-manifest`，由持锁的 helper 在发布前以独占创建、`0600`、同步落盘方式写入 `{installation_id, channel, executable, remove_root}`。其中 `remove_root` 是目标文件本身或包含目标 helper 的 App bundle 根，必须是经过校验的绝对物理路径。清单放在最终安装路径之外的本次安装私有目录（cask 使用下述 Caskroom 外的持久尝试目录，本地安装使用其清单目录），由相应安装方式保存到卸载/回滚结束。每次安装使用新的清单文件；不能覆盖旧清单，也不能在清理时从“当前回执”补填或刷新预期 ID。写清单失败则不发布；发布失败留下的清单不授予删除权限。它只是一次操作的预期参数，不授予状态访问权。

`install unregister` 在一次连续持有的所有权排他锁内执行以下判断和动作；没有无条件删除或 `--takeover` 绕过分支：

1. 取得排他锁，超时以 `owner_running` 拒绝，文件与回执都不改。
2. 校验清单与命令传入的 ID、channel、executable 一致，且 `remove_root` 仅为该可执行文件本身，或以 `Contents/Helpers/agentdeck` 为相对路径的 App 根；不允许任意祖先目录。读取并校验回执结构，而不是直接使用 `Resolve` 的已安装结论：helper 丢失时，回执仍可标识待清理的安装。只有回执的 `installation_id`、`channel`、`executable` 与清单中的预期值全部一致，才允许清理。路径与 channel 一致但 ID 不同也算后来的安装。
3. 回执有效但任一预期值不匹配：幂等跳过，内部结果为 `skipped_superseded`、exit 0，不改回执，不删除文件，不注销其小组件。回执和目标都不存在时也是幂等成功。回执缺失/损坏而目标仍存在，或清单缺失/无效时，不能证明归属：保留目标，以非零状态报告失败；隐藏命令复用 `install_claim_refused` 和已有的 `receipt_missing` / `receipt_invalid` 原因，提示通过原安装方式重新安装修复回执后再卸载。外部清单完整也不改变这一点：清单只固定要清理哪一次安装，不单独证明归属，所以 `unregister` 在没有匹配回执时不会删除目标。cask 上“通过原安装方式重新安装修复”的可达形式，以及用户明确授权时对这类拒绝的容忍（`--authorized-repair`，仍不删除任何对象），见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)。公开 `install claim` 的原因和界面不扩展。
4. 匹配时，校验删除根仍是清单指定的普通文件或 bundle，拒绝符号链接和类型不符。在同一持锁期间完成该安装的小组件注销（同步执行 `pluginkit -r` 等需要的清理），撤销其回执，再删除目标；目标已不存在时仅清理匹配回执。根路径不能只因 helper 缺失就重算为另一个路径。任一失败均报告失败，保留尚未清理的文件和清单；不得释放锁后追加兜底删除。撤销后删除失败/崩溃留下的残余按第 3 步恢复，不假装卸载完成。
5. 全部动作结束后释放锁。若外层安装器后来又要求回滚或重复卸载，仍须携带原清单的 ID，重新取锁并执行上述比较；不得把旧操作升级成清理新安装。

**损坏 helper 不等于允许绕过协议。** cask 在 Caskroom 外的私有尝试目录中保留可执行的清理 helper 副本；本地安装同样保留，或使用经过安装产物校验的暂存 helper。它们只执行同一个 `unregister` 协议，不依赖最终 App 内的 helper 完好。副本或清单不可用就报告未完成，保留目标并走重新安装恢复（cask 的可达形式见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)）；禁止回退到 shell 直接删除。安装失败发生在成功登记之前时，回滚仅能在原 `register` 持锁期间恢复它已改动的文件；该进程退出后，没有匹配回执的残余也按第 3 步处理。成功登记后外层步骤失败，则用本次清单条件清理，不能无条件恢复旧安装覆盖新拥有者。

### cask 的完整安装与卸载链

**cask 改为由 install step 发布 App。** Homebrew 的 `app` 标签会在我们的进程之外移动 App，无法纳入这把锁，所以 cask 不再使用 `app` 标签。

**Homebrew 7 的步骤约束（2026-11-09 在本机隔离 prefix 中用 Homebrew 7.0.12 实测）。** 下面的结构以这些事实为前提：

- `preflight_steps` 等 `*_steps` 步骤在 cask 沙箱中运行（经典 `preflight` 块和 `uninstall script:` 不在其中，见下文）：`HOME` 被换成临时目录，真实主目录不可读；`appdir` 和 Caskroom 可写；主目录下的其他路径要用 `writable_paths:` 加 `writable_base: :home` 声明，声明后可读可写。
- 沙箱内调用 `pluginkit` 失败（`Connection invalid`）。经典的 `uninstall script:` 不在沙箱中运行，在 `uninstall quit:` 之后执行，`pluginkit` 可用。
- 步骤内用 `{{staged_path}}`、`{{appdir}}`、`{{version}}` 这类模板变量；经典 stanza 不展开它们，要用 Ruby 插值 `#{staged_path}`。
- `brew` 只把 `HOMEBREW_*` 环境变量传给步骤。
- 在本机上，步骤直接执行 Caskroom 中未经公证的程序，会被 Gatekeeper 以 SIGKILL 终止并在桌面弹窗，ad-hoc 签名也一样；只调用 `/bin/sh` 等系统程序不受影响。发布产物经过公证，不受此影响；这一条约束的是测试，见下文的验证位置。

**修订后的入口与兼容范围（ISO-ARCH-R8-F1/F2）。** formula 冲突检查仍用 `preflight_steps`；发布改用经典 Ruby `preflight` 块，并声明 `uninstall_preflight` 回滚块。经典 `uninstall script:` 只承担正常/升级卸载，不作为安装失败回调。Homebrew 源码提交 `47b42777c81a72198fd0598a2e5fb02f1701c63a` 的 `AbstractFlightBlock` 同时提供 install/uninstall phase；`install_artifacts` 只回滚成功返回的 artifact，最后无条件 purge staged_path。两个 flight 声明是独立 artifact，同类排序不保证先后，方案不得依赖它们之间的顺序：发布块内部负责自身失败；发布成功之后，两个块都能完成其 install phase，再进入 binary 等后续 artifact。

经典 flight DSL 在该版本仍可用但已弃用（`dsl.rb` 调用 `odeprecated`）。本方案显式承担此兼容性限制；CI 必须验证实际加载和完整分派，不抑制弃用错误、不修改 Homebrew、不以 `*_steps` 的 symlink 逆向执行冒充回滚。若支持版本拒绝这些 stanza，本任务阻塞并返回设计，不声称兼容。经典块不处于 steps 沙箱；其中仅调用受控 helper，真实账户解析与安装所有权锁仍是写入边界。原 steps 的 `writable_paths` 不是这些块的权限保证。

**每次尝试的材料与定位。** 由 helper 在真实账户的 `~/Library/Application Support/AgentDeck/installers/homebrew_cask/attempts/<随机 attempt ID>/` 独占创建 `0700` 私有目录（这里的 `~` 是 helper 经系统账户数据库解析出的账户主目录，不是 `HOME`；经典块只传 attempt ID，并使用 helper 返回的路径，见[风险](#风险)中的账户解析一行），保存经产物校验的 helper、`0600` 清单及尝试元数据；不得跟随符号链接。该目录位于最终 App、Caskroom、Homebrew backup/purge 根之外，不属于便携状态。attempt ID 是外层材料索引，清单内 installation ID 仍由 register 在锁内生成。暂存 App 也使用 attempt ID 命名，位于 appdir 同文件系统；不得删除另一次尝试的暂存路径。

在调用 register 前，先同步保存材料、尝试索引和 staged_path 中的定位文件 `.agentdeck-attempt`。定位文件只记录本次 attempt ID 和固定目标，不存可任意执行的路径；解析时限定在上述私有根并校验 token、版本、目标、账户及普通文件类型。外部索引保存相同信息，使 staged_path 被 purge 后仍能定位残余。每次 preflight（包括恢复旧版）都分配新目录并原子更新定位文件，绝不复用/覆盖旧清单或 helper。旧定位文件的替换不是清单的覆盖；旧尝试由不可变 ID 保留在外部索引中。

**安装及返回前失败。** Ruby preflight 在 formula 检查之后、binary 之前执行以下完整的 `begin/rescue/ensure` 边界：经 helper 准备独立材料并写入定位文件，复制暂存 App，调用 `install register --channel homebrew_cask --executable <最终 helper> --source <暂存 App> --cleanup-manifest <本次清单> --require-unowned`，然后写完成标记并清理本次暂存副本。隐藏的 `--require-unowned` 由 register 在同一次所有权排他锁内检查，默认只在“没有有效拥有者，且目标不存在”时发布。有效拥有者（含相同 channel、相同路径）默认以 `install_claim_refused / owned_by_other` 拒绝；没有有效拥有者但目标已存在，以 `receipt_missing`（回执缺失或过期）或 `receipt_invalid` 拒绝并保持目标不变。显式授权可修复无主目标，或精确绑定原 installation ID 的本安装，完整判定表见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)。“无主”不等于回执文件在磁盘上不存在，已认领 formula 卸载后的过期回执因此不阻断 cask 安装。不能在 Ruby/shell 中先检查、再无条件 register；不影响 make/local App 既有显式覆盖协议。

发布前把本次 attempt 身份保存到这次 cask 求值的闭包变量；`uninstall_preflight` 捕获同一变量。正常卸载另行求值 cask 时变量为空，所以该回滚块无操作，由经典 uninstall 在 quit 之后清理。回滚不得读取可能已被后续尝试替换的定位文件来重新选择 ID。

发布块任一步抛错（包括 register 成功后的标记写入和暂存清理失败）都在 rescue 中调用本次清单的条件清理，然后重新抛出原安装错误；清理失败另行报告并保留外部材料。清单尚未创建且本次尚未调用 register 时可安全无操作；调用过 register 后若清单缺失/损坏，必须保留目标并报未完成，不能凭缺少成功标记推定未发布。进程被强杀而无法运行 rescue 时，外部材料和索引仍在，后续恢复只按原 ID 重试，不保证崩溃时立即清理。

**后续 artifact 失败。** 两个 flight artifact 成功返回后，binary、补全或保存安装配置失败触发 `install_artifacts` 逆序回滚。`uninstall_preflight` 使用捕获的 attempt 调用同一个条件清理；preflight 自身的无操作逆向 phase 不承担清理。即使更早的回滚 artifact 抛错中断回滚遍历，或条件清理失败，外部材料也不会被 finally 的 `purge_versioned_files` 删除。记录未完成及 attempt ID，允许用保留 helper/清单重试；绝不把“没有执行回调”报告为清理成功。

**正常卸载与材料寿命。** 保留经典 `uninstall quit:`；其 script 在退出 App 后读取该安装记录的 `.agentdeck-attempt`，一次性捕获并验证本次身份，再调用保留 helper 的 `install unregister --expected-installation <原 ID> --cleanup-manifest <原清单> --remove`。小组件注销在归属匹配后的同一次持锁内完成。定位文件、清单或 helper 不可用时报告未完成，不推断无安装，也不从当前回执刷新 ID；用户给出授权修复时的容忍行为见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)。删除独立的路径式小组件注销脚本。

成功清理只标记该 attempt 已清理，保留不可变清单/helper/外部索引，以支持升级恢复和延迟重试；本任务不自动按年龄或版本回收这些材料。失败材料保持待恢复，报告精确 attempt ID。外部材料的后续垃圾回收不在本任务范围。`uninstall`、`zap`、强制跳过脚本、失败 purge 都没有删除最终 App、外部材料根或其父目录的规则；后续操作只删除本次暂存路径或链接本身。

**失败升级恢复。** 顺序明确为旧尝试 O 条件卸载并保留材料 → 新尝试 N 发布 → 后续 artifact 失败 → N 条件清理或报告残余 → Homebrew `restore_backup` 恢复旧 staged_path → 重新执行旧版本 preflight，创建恢复尝试 R。R 使用新的暂存路径、清单和 installation ID，更新定位文件到 R，O/N 清单逐字节不变。没有竞争者且 N 清理成功时，R 的锁内 `--require-unowned` 检查通过，旧版本重新发布成功；若 N 残余或竞争者 B 已占用路径，R 拒绝并保留 B，不在锁外恢复旧 bundle。O/N 的延迟清理携带原 ID，面对 R/B 只会跳过或等待锁，不删除恢复后的实例。Homebrew 的备份恢复仅作用于 staged_path，不能成为最终 App 的发布入口。

#### cask 无主判定与授权修复

**“无主”的含义（ISO-ARCH-R9-F1）。** `--require-unowned` 判定的是有没有有效拥有者，不是回执文件在不在磁盘上。有效拥有者沿用[安装身份判定](#安装身份判定)的归一化：回执有效，并且登记的程序仍然存在。回执缺失、无效，或回执有效但登记的程序已被删除（过期），都算无主。已认领的 formula 被卸载后，keg 被删、回执过期，按无主处理：cask 安装不被它阻断，也不需要 `--takeover`，登记成功后覆盖这份过期回执。锁内仍有有效拥有者时，默认拒绝并打印迁移命令；唯一新增例外是下文显式绑定原 installation ID 的本安装修复，不以相同 channel、路径或版本推定身份。

锁内的 register 判定如下（`--source` 发布；cask 总是带 `--require-unowned`）：

| 回执（锁内归一化） | 目标 | `--require-unowned` | 另加 `--authorized-repair`（绑定模式还须传原 ID） |
| --- | --- | --- | --- |
| 有效拥有者；未绑定原 ID，或 ID/channel/规范 executable 任一不匹配 | 任意 | `owned_by_other`，目标和回执不变 | 同左。相同 channel、路径、版本也不能代替 ID 匹配，cask 从不传 `--takeover` |
| 有效的原 cask 拥有者；与本次修复预先捕获的 ID/channel/规范 executable 全部匹配 | 存在，类型与来源包相符 | `owned_by_other`，目标和回执不变 | 显式传 `--expected-installation <原 ID>` 时，在同一次排他锁内验证并用 `RENAME_SWAP` 替换；失败恢复原对象和回执，提交成功后才删除换下对象 |
| 无主 | 不存在 | 发布并登记；过期回执被覆盖 | 同左 |
| 无主 | 存在，类型与来源包相符 | 回执缺失或过期以 `receipt_missing`、无效以 `receipt_invalid` 拒绝，目标和回执不变 | 未传预期 ID 时，用 `RENAME_SWAP` 原子替换，生成新 ID/清单，提交前失败恢复原对象和回执；已传预期 ID 却无法验证时仍拒绝，不能退回无主替换分支 |
| 无主 | 存在，但是符号链接或类型不符 | 按既有类型检查拒绝 | 同左 |

**有效回执、仅清理材料丢失（ISO-ARCH-R10-F1）。** 清单、保留的 helper 副本或 `.agentdeck-attempt` 单独丢失时，原回执和 App 内 helper 仍可有效。这与无主目标是两个独立状态，不能只设置授权变量后重跑普通 register，也不能为了修复而先删除回执。

可信身份来自原 App 内 helper 的隐藏只读命令 `install repair-identity --channel homebrew_cask --executable <固定 helper>`。用户开始这次修复时运行它一次；命令取得所有权排他锁，验证回执有效、登记的程序存在、channel 为 `homebrew_cask`，且登记的规范 executable、当前执行程序与用户选定 cask 的固定 helper 路径一致，成功只向 stdout 输出非空 installation ID。缺失/无效回执、其他 channel/路径、类型检查失败或取锁超时均非零退出，无目标、回执或材料写入。这个身份读取不依赖清理清单、保留副本、定位文件或可变外部索引，也不读取 App 内或暂存目录里的自报 ID。

捕获结果通过本条 brew 命令的 `HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION` 传递。模板将它原样转换成 helper 的 `--expected-installation <原 ID>`，与固定 channel/executable 一起构成本次修复的预期身份；helper 不读环境变量。它只与 `HOMEBREW_AGENTDECK_REPAIR=1` 配对使用，单独设置、空值或格式无效均拒绝；不持久化，不在卸载、preflight、回滚或重试中从当前回执刷新，也不能因校验失败而丢弃它后重试无绑定修复。新进程或重新求值 cask 时仍使用同一条命令继承的值，不依赖两个 Ruby 块共享闭包。

有预期 ID 时，卸载与 register 各自在自己的同一次连续排他锁内核对 ID/channel/规范 executable，核对与任何目标操作之间不得释放锁。`attempt cleanup` 向执行清理的 helper 显式传递内部参数 `--require-identity`，无论清理材料是否完整，都先在该排他锁内验证捕获身份；该参数须与 `--authorized-repair` 配对，完整材料在身份匹配后才进入条件删除。原身份仍匹配而材料不可用时，卸载只报告 `skipped_unverified`，保留原回执和 App；保留副本缺失时使用固定路径的 App 内 helper 执行这一检查，定位文件缺失时直接使用命令绑定的身份，不从外部索引猜 ID。随后 register 可按表中匹配原身份的一行原子替换，并建立全新的 attempt、清单和 installation ID。若原卸载已经正常完成，则只在无有效拥有者且目标不存在时按普通新装发布；绑定身份不能授权替换一个已无回执的现存对象。

身份捕获后出现同路径后继 B（即使 channel、版本都相同）时，带授权和预期 ID 的卸载、发布均以 `owned_by_other` 非零拒绝，B 的对象和回执不变；材料缺失也不能让脚本绕过锁内核对后 exit 0。若 B 的回执又缺失/损坏但目标仍在，仍以 `receipt_missing`/`receipt_invalid` 拒绝。普通旧清单的延迟清理继续使用原 ID 及既有 `skipped_superseded` 语义，不把该跳过变成本次修复的替换授权。材料和 App helper 同时不可用、因而无法完成上述检查时，不声称此有效身份恢复入口可用：保留对象、报告未完成，不靠 shell 模拟 ID 校验。

**为什么需要授权修复。** 目标仍在而回执丢失或损坏的 cask 安装，同时被两处挡住：`unregister` 不能证明归属，保留目标并失败；恢复用的新 register 又被 `--require-unowned` 拒绝。用户照“通过原安装方式重新安装”的提示去做，在 Homebrew 中走不通。下面三点都来自源码提交 `47b42777c81a72198fd0598a2e5fb02f1701c63a`（2026-11-09 逐行核对）：

- `brew reinstall` 先强制卸载已安装的 cask，再安装（`cask/installer.rb:193,272-278`）。经典 `uninstall script:` 以 `must_succeed: true` 运行，`--force` 只在脚本文件不存在时才跳过它（`cask/artifact/abstract_uninstall.rb:633-655`）。脚本失败会让卸载、重装和升级都在安装步骤之前中止，所以重装根本到不了 preflight。
- 对已安装且是最新版的 cask，带 `--force` 的 `brew install` 也不会重新安装：它只取未安装或过期的 cask（`cmd/install.rb:232-239,459-492`、`cask/upgrade.rb:58-80`）。`brew install --cask --force` 不能当作已被记录的 cask 的修复入口。
- 经典 flight 块和 `uninstall script:` 拿不到 `--force`：`AbstractFlightBlock` 接收 `force`、`predecessor` 却不转给块（`cask/artifact/abstract_flight_block.rb:39-41,55-57,77-80`），块里只有 `cask`、`command`、`token`、`staged_path`、`appdir` 等（`cask/dsl/base.rb:20-28`）。所以 Homebrew 的 `--force` 不能充当授权。

**授权修复的形式。** cask 的目标替换授权，是用户为这一条命令显式设置的环境变量 `HOMEBREW_AGENTDECK_REPAIR=1`。它授权无主目标的容忍/替换；配对传入预先捕获的 installation ID 时，另允许修复被精确选定的原 cask 安装，不授权接管其他有效拥有者，也不赋予缺失清单的清理命令删除权。经典块在 `brew` 进程内运行，`uninstall script:` 由 `brew` 以子进程启动，二者都继承调用者的环境（`system_command.rb:531-558` 的 `exec3` 以 `Process.spawn` 或 `exec` 启动，只合并命令自带的变量，不清除继承的环境）；变量沿用验收变量的 `HOMEBREW_AGENTDECK_` 前缀命名，CI 回归验证它们确实到达 helper 参数。模板各执行入口读取一次，将本条命令的固定值转成 `--authorized-repair` 及可选的 `--expected-installation`；helper 本身不读环境变量。回执缺失/无效的无主修复仍由用户运行：

```text
HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask <token>   # Homebrew 仍记录该 cask
HOMEBREW_AGENTDECK_REPAIR=1 brew install --cask <token>     # Homebrew 不再记录该 cask
```

仅清理材料丢失而回执有效时，提示改为下面的身份绑定入口；实际输出用 shell 安全引用后的固定 helper 路径和 cask token 替换占位符。`&&` 保证身份捕获失败时不执行 brew；该隐藏命令成功时保证非空 ID：

```text
agentdeck_repair_installation="$(<固定 App helper> install repair-identity --channel homebrew_cask --executable <固定 App helper>)" && HOMEBREW_AGENTDECK_REPAIR=1 HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION="$agentdeck_repair_installation" brew reinstall --cask <token>
```

Homebrew 已不记录该 cask 时，将这条命令的 `brew reinstall` 换为 `brew install`，仍先捕获身份。用户无需读写回执或手工删除 App。

1. 旧版本的卸载脚本带 `--authorized-repair` 及本次命令可选的预期 ID 调用 `unregister`。无绑定模式下，结果只在它本来会以 `receipt_missing` 或 `receipt_invalid` 拒绝时才不同：回执缺失/无效而目标仍在，或清单缺失/无效，此时不删除任何对象、不改回执，以 `skipped_unverified` 报告“未验证归属、未清理”并 exit 0。保留 helper 或定位文件也不可用时，无绑定脚本可同样报告而不删除；但不能据此覆盖有效拥有者。有绑定模式必须先完成上文的 helper 锁内身份检查，原身份匹配才可容忍材料缺失；不匹配、无法验证或无法调用 helper 均非零退出。回执与清单匹配时仍按正常协议清理；普通 `skipped_superseded`、`owner_running`、清单与命令不一致和匹配后删除失败的结果不变。单独授权 `brew uninstall` 的容忍只移除 Homebrew 记录，App 保留，报告明确说明未完成清理。
2. 新版本 preflight 带 `--require-unowned --authorized-repair` 及同一个可选预期 ID 调用 register，严格执行上表：无绑定只替换无主目标；有绑定可替换 ID/channel/规范 executable 全部匹配的原 cask 安装，遇到后继身份则拒绝，不退回无主替换。旧对象在回执提交前一直保留，提交前失败恢复原对象及回执；提交成功后才删除换下对象。
3. 缺失/无效回执的修复不依赖外部材料；有效回执且材料单独丢失则依赖上述 App helper 捕获与绑定，不再作无条件可达承诺。每次修复都分配新的 attempt、清单和 installation ID，仍在的旧材料逐字节不变；不伪造原清单，也不借新 ID 执行旧安装清理。

没有授权时行为不变：卸载、重装和升级以非零状态失败并保留目标。模板在 helper 因上述原因拒绝时，补充一行含本 cask token 的完整修复命令，让用户能照着做。这条提示只出现在安装方式的失败输出中，`doctor` 和状态拒绝的文案不提它（见 [ux/cli.md 的文案约束](ux/cli.md#文案约束)）。

**授权的边界。** 两个变量只对运行它们的那条 `brew` 命令生效，不写入任何文件；在 shell 配置里长期设置等于预先授权，不推荐。回滚块和 preflight 失败 rescue 的条件清理永远不带 `--authorized-repair`，只携带本次发布的新清单 ID，不把原修复 ID 作为清理对象：无法证明归属就报告未完成并保留材料。恢复旧版本 R 的 `--require-unowned` 仍然严格；命令带绑定身份时也不能因原安装已被 R/竞争者取代而刷新预期 ID，或退回无绑定替换。对有效拥有者的例外仅限这次显式授权且原身份仍精确匹配的 cask 修复；接管其他有效拥有者仍须 `TAKEOVER`、`install claim --takeover`，不能借修复实现。

**验证位置（2026-11-09 用户决定）。** 真实 brew 回归只在 CI 的 GitHub 托管 macOS runner 上运行，不在开发机上运行。原因是测试使用的验收版 helper 没有公证，在开发机上执行会被 Gatekeeper 拦截并弹窗。回归脚本在没有 CI 专用开关时拒绝运行；CI 任务先做可行性探测，在真实 brew 步骤中执行一次验收版 helper，被终止就判失败，不静默降级。下面每一项都必须在这个 CI 回归中实际验证，任一项不成立，cask 部分就回到设计重新决定：

- 可行性探测：验收版 helper 能在 cask 步骤中执行；
- 不带 `app` 标签、在 preflight 发布 App 的 cask，能正常安装、升级、卸载，`binary` 和补全链接照旧；
- 经典 flight 块可加载、回滚块可达；helper 只写隔离账户的回执/材料目录和 `appdir`。`HOME` 与账户目录取不同的临时目录运行时，回执、清单、材料和索引只在账户目录，`HOME` 下没有；账户目录不可写时安装非零退出、没有回执和已发布的 App，也没有回退写入（见[风险](#风险)中的账户解析一行）；
- 小组件注册（`test-cask-widget-registration.rb`）仍然有效，卸载时在锁内注销成功；
- 升级时旧版本的卸载先于新版本的 preflight 执行；同版本重装时，新安装的清单路径不与旧清单冲突；
- 分别注入发布块返回前失败、后续 binary 失败、回滚中断及升级失败恢复旧版；检查回调事件、失败退出码和 purge 后外部材料仍在；恢复 R 的路径/ID 与 O/N 不同，旧材料不变，无竞争者时旧版恢复成功，有竞争者时 R 拒绝且竞争者完整；
- 完整卸载、失败回滚和损坏 helper 清理都遵守条件删除；在旧 helper 返回后插入同路径新安装，继续所有剩余的 Homebrew 步骤（含强制路径），新 App 与回执保持完整。覆盖同 channel 重装，不能只靠 channel 不同来阻止误删；
- 损坏安装的授权修复（ISO-ARCH-R9-F1）：两种损坏——删除回执、把回执改成无效内容——各做两轮，第一轮外部材料完整，第二轮同时删除外部材料。每一轮里，不带 `HOMEBREW_AGENTDECK_REPAIR=1` 的 `brew reinstall --cask` 非零退出，App、Caskroom 记录和仍在的外部材料逐字节不变，失败输出含带本 cask token 的修复命令；带该变量重跑成功，生成新的 attempt、清单和 installation ID，仍在的旧材料不变，App 被替换，小组件注册有效。再用无回执的残留 App 且 Homebrew 不记录该 cask 的状态验证 `brew install --cask`：无变量被拒绝、目标不变；有变量替换成功。另放入有效拥有者（make 或 local_app 的回执加对应对象，以及仍存在的 formula keg 的回执），带变量的安装仍以 `owned_by_other` 失败，拥有者的对象和回执逐字节不变；
- 有效原回执、仅清理材料丢失（ISO-ARCH-R10-F1）：仅删除清单、仅删除保留 helper 副本、仅删除定位文件三例分别保持回执及 App helper 有效。无授权重装拒绝，只有授权变量而无预期 ID 也不能覆盖有效安装；经 App helper 一次捕获原 ID 后，沿完整 `brew reinstall` 成功建立新 attempt/清单/ID，旧材料不变，随后普通无授权卸载成功。分别在捕获后、卸载容忍后且发布前插入同 channel/路径/版本的后继 B，继续原命令必须拒绝，B App/回执不变；B 回执再缺失/损坏也不能降级替换。成功修复后的延迟旧清理不得删除新实例；在替换后、回执提交前注入失败须恢复原 App/回执。Task 5 在 helper 层和真实 brew 链分别覆盖这些状态，不能用模型替代；
- 过期 formula 回执迁移：回执登记一个已被删除的 keg 路径，不带任何变量的 `brew install --cask` 成功，回执改为 cask，没有用 `--takeover`；keg 仍在时同一命令以 `owned_by_other` 失败，并打印迁移命令，目标不存在、回执不变；
- 恢复旧版本 R 仍然严格：目标处有回执丢失的竞争者对象，且没有设置变量时，R 以 `receipt_missing` 拒绝，竞争者对象不变；回滚块和 rescue 的清理调用不带 `--authorized-repair`（渲染后的声明可直接检查）。

开发机上仍运行不执行暂存 helper 的部分：已有的 formula 冲突 preflight 拒绝（只调用 `/bin/sh`），以及按 cask 声明驱动、在本地直接调用验收版 helper 的条件删除检查。上文判定表的每一行（有效拥有者、过期回执、缺失与无效回执加已存在目标、类型不符，各自带与不带 `--authorized-repair`），以及 `unregister --authorized-repair` 的容忍与不容忍情形，也在开发机上由验收版 helper 在临时账户目录中直接验证；渲染后的 cask 声明另做静态检查：`--authorized-repair` 只由 `HOMEBREW_AGENTDECK_REPAIR` 决定，回滚块和 rescue 的清理调用不带它，没有 `writable_paths`/`writable_base`，也没有由 `HOME`、`~` 或 `Dir.home` 推导的 AgentDeck 路径。

| 安装方式 | 写入 | 冲突与接管 | 卸载 |
| --- | --- | --- | --- |
| Homebrew cask（`packaging/homebrew/agentdeck-app.rb.tmpl`） | 按上文，由经典 `preflight` 执行带 `--source --require-unowned` 的 `install register`，在锁内发布 App 并登记 | 不传 `--takeover`。默认拒绝有效拥有者；显式授权仅能替换无主目标，或替换预先绑定且锁内 ID/channel/规范 executable 全部匹配的原 cask 安装，不能覆盖后继。过期回执不阻断。入口见 [cask 无主判定与授权修复](#cask-无主判定与授权修复) | 经典 `uninstall` 在 `quit` 后携带原清单身份条件清理，无路径式兜底；材料缺失默认失败，授权容忍不删除；带预期 ID 时必须先由 helper 持锁核对，不能靠脚本猜身份 |
| Homebrew formula（`packaging/homebrew/agentdeck.rb.tmpl`） | 安装步骤不写任何回执。首次安装时，caveats 提示用户运行一次 `$(brew --prefix)/opt/<formula>/bin/agentdeck install claim`，见 [formula 认领](#formula-认领)。升级后的切换见 [formula 升级续登](#formula-升级续登) | 由 `install claim` 和升级续登处理。keg 是新路径，Homebrew 创建它时不会碰到已登记的文件 | 卸载（或升级后清理）删除 keg，回执随之过期，按“无主”处理（含义见 [cask 无主判定与授权修复](#cask-无主判定与授权修复)；之后安装 cask 不需要 `--takeover`） |
| `make install`（`scripts/manage-install.sh`） | 保留暂存、校验和清单逻辑并保存本次清理清单；最后把暂存文件放到 `BINDIR/agentdeck` 的那次重命名，改由暂存的新程序执行带 `--source` 的 `install register`（channel 为 `make`） | 规则见上文第 2 步。`TAKEOVER=1` 时传 `--takeover`。`FORCE=1` 的含义不变：它只授权替换目标路径上的文件；没有它时，目标已存在就照旧拒绝 | `make uninstall` 先校验现有清单，再携带本次清单身份执行 `install unregister --remove` |
| 本地 App 安装（新增 `make install-macos-app` → `scripts/install-local-app.sh`） | 把 `MACOS_APP` 复制到 `APPDIR` 下的暂存位置（同一文件系统），由其中的 helper 执行带 `--source` 的 `install register`（channel 为 `local_app`），保存本次清理清单，发布后再把 `/usr/local/bin/agentdeck` 链接到 App 内的 helper，并执行 `lsregister` | 规则与 `make install` 相同，接管也用 `TAKEOVER=1`。目标位置已有正在运行的已安装 App 时，它持有共享锁，发布会以 `owner_running` 失败，提示先退出它 | 新增对应的卸载目标，携带本次清单身份执行 `install unregister --remove` |

本地 App 安装沿用现有的 ad-hoc 签名，不加 hardened runtime（加了会导致 App 无法加载 `AgentDeckShared.framework`）。它是 0.7.0-dev 这类本地构建唯一受支持的安装方式。2026-10-18 那次手工复制没有写回执，按本设计属于未安装。

### formula 认领

`agentdeck install claim [--takeover]` 是新增的公开命令，供 formula 安装的 CLI 认领所有权，界面见 [ux/cli.md](ux/cli.md#install-claim)。按顺序检查：

1. **只能认领当前的 formula keg。** `self` 必须匹配 `<prefix>/Cellar/<formula>/<version>/bin/agentdeck`，其中 `<formula>` 为 `agentdeck` 或 `agentdeck-rc`；同时 `EvalSymlinks(<prefix>/opt/<formula>)` 必须等于这个 keg 目录。保留下来的旧 keg、或不是当前 opt 指向的 keg，一律拒绝，原因为 `not_current_formula_keg`。
2. **在排他锁内处理现有回执。** 先按[所有权锁与交接](#所有权锁与交接)取得排他锁，取不到就以 `owner_running` 拒绝。拿到锁之后：
   - 没有回执、回执无效、回执过期，或回执登记的就是自己：直接写入；
   - 回执登记的是另一个仍存在的程序：没有 `--takeover` 时以 `owned_by_other` 拒绝；有 `--takeover` 时写入。拿到排他锁这件事本身就证明了旧拥有者此刻没有任何访问会话。
3. **写入。** 回执写为 `executable = self`（keg 内的真实路径 `<prefix>/Cellar/<formula>/<version>/bin/agentdeck`）、`channel = homebrew_formula`，然后释放排他锁。不登记 opt 链接，原因见[安装身份判定](#安装身份判定)。

这个命令本身不访问真实状态目录，因此可以由未安装进程执行，这也是它存在的意义。它只写回执，不写客户端配置。stable 与 rc、不同 Homebrew 前缀之间的选择，就是用户对哪个程序执行了认领。

接管的同步规则只有一条，见[所有权锁与交接](#所有权锁与交接)。第一轮修复中用 `lsof` 检查进程、再与写回执相邻执行的做法已删除：复评指出，检查和写入之间仍有空窗，无法阻止已经通过守卫的进程继续访问。

### formula 升级续登

`brew upgrade` 会生成新的 keg 路径。Homebrew 的安装步骤写不了 `$HOME`，没法替新 keg 登记。按用户 2026-11-05 的决定，由新 keg 自己在所有权排他锁内续登，不需要用户再认领一次。

**触发时机。** 新 keg 的进程第一次需要状态目录时触发，有两个入口，调用同一个续登函数（ISO-ARCH-R4-F2）：

- 没传 `--state-dir` 时，在[状态目录解析](#状态目录解析)选择默认目录之前；这是普通命令最常走的路径；
- 显式指向真实状态时，在[状态访问守卫](#状态访问守卫)判定自己不是拥有者之后。

显式指向非真实目录时不续登。四个条件全部满足才续登，否则照常处理：

1. 回执有效，channel 为 `homebrew_formula`；
2. 回执登记的 keg 与 `self` 属于**同一个** `<prefix>` 和**同一个** `<formula>`，也就是只有 `<version>` 不同；
3. 回执登记的 keg 路径已不存在。brew 默认会在升级后清理旧 keg；
4. `self` 正是当前 opt 指向的 keg，判定方法与[认领](#formula-认领)第 1 步相同。

**续登过程。**

1. 以非阻塞方式重试取排他锁，总等待上限 5 秒。旧 keg 的进程即使文件已被删除，仍可能在运行并持有共享锁；排他锁会一直等到它们的会话全部结束。超时就以 `ownership_busy` 拒绝本次访问；Hook 下次被调用时会再试。
2. 在锁内重新读回执，再核对一遍以上四个条件，以防在等锁期间回执被改动。条件仍然成立，就原子写入 `executable = self`；否则放弃，按当时的回执判定。
3. 释放排他锁，然后像普通已安装进程一样，取共享锁、在锁内判定，继续访问。

**不续登的情形。** 旧 keg 仍然存在，例如用户关闭了清理，或者 pin 了旧版本：旧 keg 仍是拥有者，新 keg 按未安装处理，`doctor` 提示运行 `install claim --takeover`。跨 formula（stable 与 rc 之间）或跨前缀时，也一律不续登，因为这属于换一个安装，而不是同一个安装升级。

**为什么安全。** 续登只发生在排他锁内，并且只在旧 keg 已被删除的时候发生，因此：

- 旧会话结束之前，续登拿不到锁；
- 旧 keg 的文件已经不在，不会再有新的旧版本进程启动；
- 旧进程下一次过守卫时，自身路径解析失败（`executable_unresolved`），会被拒绝。

评审给出的反例"旧会话持锁期间 opt 被切换，新 keg 不经排他锁就成为拥有者"因此不再成立：

- opt 被切换本身不改变回执；
- 旧 keg 被删除之前，新 keg 不满足续登条件；
- 删除之后，新 keg 也要等旧会话结束、拿到排他锁才能续登。

**升级和迁移。** 回执由新版本的安装步骤或续登写入：

- cask 升级时，旧版本的卸载脚本和新版本的 preflight 各自在排他锁内完成注销和发布；
- formula 升级后自动续登；
- `make install` 用户升级时本来就要重新运行它。

所以按受支持方式升级的用户不需要额外操作，现有数据原样可用（需求 A7）。已经在用 formula 的用户，第一次升级到包含本主题的版本后，需要认领一次，因为这时还没有可以续登的回执；未认领时，`doctor` 和 TTY 提示会给出 `install claim` 命令。

**测试隔离。** 安装脚本的测试使用临时 `PREFIX`，安装的程序是验收版，回执只写入临时账户目录。cask 的写入在 CI 的真实 brew 回归中验证（见 [cask 的完整安装与卸载链](#cask-的完整安装与卸载链)），prefix、Cellar、Caskroom、cache、`HOME` 和账户目录都指向临时目录。

## 只读和 Hook 路径

- `OpenReadOnly` 一样经过状态访问守卫：未安装进程对真实目录连读都不允许（需求 G3）。
- Hook 和 statusline 命令被守卫拒绝时（例如所有权已被转移），仍然遵守 Hook 的静默失败开放契约：exit 0，不输出。

**安装拒绝不写入任何拒绝记录（ISO-ARCH-R1-F4）。** schema-version-signal 的 `hook-refusals.json`（`internal/hookrefusal/record.go`）只接受 `code=schema_ahead` 且存储版本高于支持版本的记录，并且写在被访问的状态目录里。安装拒绝既没有 schema 版本差，也不能为了记录诊断去写受保护的真实状态目录。因此：

- 这个机制保持原样，不扩展；
- 不新增其他拒绝记录；
- 安装拒绝只通过两个只读诊断展示：
  1. `installation` 检查，说明当前运行的这个程序是否已安装；
  2. 新增的 `managed_command_owner` 检查。

**`managed_command_owner` 检查。** 它在已安装程序的 `doctor` 中运行。只读地取出客户端配置中由 AgentDeck 管理的 Hook 和 statusline 命令，识别方式沿用 `managedAgentDeckExecutable`（`internal/usagehook/config.go:1009`）。对其中的绝对路径命令执行 `EvalSymlinks`，结果不等于当前拥有者时，报告 `warning`，code 为 `managed_command_owner_mismatch`，并给出重新登记的命令。基于 PATH 的 `agentdeck` 命令在 Claude 或 Codex 运行时解析，doctor 无法可靠复现那个环境，所以只报告为 `ok`，并在 reason 中注明是基于 PATH 的命令。这正是接管之后需要提醒的情形：旧拥有者以绝对路径登记的 Hook 会被静默拒绝。未安装进程的 `doctor` 不运行这个检查，因为它不是这些命令的拥有者。

## doctor

`doctor.Service` 在 `state` 检查之前新增 `installation` 检查，在 Hook 相关检查旁边新增 `managed_command_owner` 检查，各字段见 [ux/cli.md](ux/cli.md#doctor-的-installation-检查)。只使用现有的 `Check` 字段（`name`、`status`、`code`、`reason`、`recovery_command`），不新增 JSON 字段，也不包含路径。

**状态目录被拒绝时，doctor 输出部分报告（ISO-UX-R1-F1）。** 现状是 `newDoctorCommand`（`cmd/agentdeck/main.go:3498-3517`）在 `opts.stateRoot()` 出错时直接返回错误，不会调用 `doctor.Service`。改为：

1. `opts.stateRoot()` 返回 `ErrStateOwnershipRefused` 时，`newDoctorCommand` 不返回错误，而是构造一份只包含两个检查的报告：
   - `installation` 检查；
   - `state` 检查：`status=error`，`code=state_ownership_refused`，`reason` 为拒绝原因。
2. 这份报告标记 `Partial=true`，沿用现有的 `checks_skipped` 告警，并经现有的 `writeDoctorResult` 输出。
3. 这条路径不会把被拒绝的目录交给 `doctor.Service`，所以不会对真实目录执行任何 `os.Stat`、锁诊断或 hookrefusal 读取。
4. 退出码为 `0`，沿用 Doctor 契约"成功渲染的报告即使有问题也以 exit 0 结束"；JSON 为成功 envelope，`partial: true`，`warnings` 包含 `checks_skipped`。

其他错误，例如 `--format` 无效，仍按原样返回错误。

desktop 快照的 health 部分来自同一个 `doctor.Service`（`internal/desktop/desktop.go:899`），所以 App 不需要新的线路字段。

## App 与小组件

**状态目录由 helper 决定。** `EmbeddedHelperRunner` 不再用 `HOME/.agentdeck` 推算状态目录（`apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:490`），调用 helper 时也不再显式传 `--state-dir`（`:838`）。改为（需求 G9）：

1. 每一轮刷新开始前，执行隐藏命令 `agentdeck desktop state-location --format json`，本轮内使用它返回的结果；结果不跨轮缓存。这个查询不需要 App 持锁：helper 自己在需要时取共享锁。
2. **App 只有在服务于真实状态时才持有长期会话（ISO-ARCH-R3-F1）。** 判断条件是：返回 `installed: true`，并且 `state_root` 指向真实状态。
   - 条件成立、而 App 还没有持锁：用 `flock(2)` 对[所有权锁](#所有权锁与交接)取共享锁，等待上限 5 秒；取得后**在锁内再查询一次** `state-location`，确认条件仍然成立，以防在取锁之前所有权已经转移。确认后持有到 App 退出；不成立就立即释放。取锁超时（正在被接管），本轮按 helper 不可用处理，下一轮再试。
   - 条件不成立，例如 Debug App、或回执登记的是别的程序：不取锁；如果之前持有，就立即释放。之后只使用隔离目录，不阻止任何安装、卸载或接管。

已安装的 App 运行期间一直持有共享锁，所以接管必须等它退出，与 cask 现有的"先退出 App"一致。未安装的 App 从不持有长期会话。每轮重新查询则覆盖了回执被协议之外的方式改动的情形，例如被手工删除；查询结果变为不成立时，App 释放锁、改用隔离目录。`state-location` 的输出如下：

```json
{"schema_version":1,"command":"desktop.state-location","generated_at":"…","data":{"installed":true,"channel":"homebrew_cask","state_root":"/Users/u/.agentdeck"},"warnings":[],"partial":false}
```

- 未安装时，`data` 给出 `installed:false`、`reason` 和隔离目录。
- 这个命令属于 desktop 线路契约，供 App 内部使用，不出现在用户文档中；它是 desktop 命名空间下唯一会包含路径的输出，因为 App 需要这个路径来做文件探测。
- 读取快照文件（`:638`）、等待扫描锁（`:778`）、读取 `AgentDeckApp.swift:215` 中的 `receiver.json`，都改用本轮取得的 `state_root`。
- `state-location` 本身经过守卫：这个 App 的 helper 不是拥有者、而状态目录又指向真实目录时，它返回的是隔离目录，不会给出真实路径。App 的直接文件探测因此不会触碰它不拥有的真实状态。
- 执行失败时按 helper 不可用处理，沿用现有错误路径，不退回到自行推算。

**Health 面板。** 在 `DesktopCopy` 的检查名表中加入 `installation`（英文 "Installation"，中文 "安装"）和 `managed_command_owner`（英文 "Hook command owner"，中文 "Hook 命令归属"），同时加入 `not_installed`、`managed_command_owner_mismatch` 的中英文说明，沿用现有检查行的文案风格。布局和状态都不变。

**Debug 配置使用独立身份**（需求 G5）。Debug 配置下修改以下 build setting，Release 和 Perf 配置不变：

| 设置 | Release | Debug |
| --- | --- | --- |
| App 的 `PRODUCT_BUNDLE_IDENTIFIER` | `com.kitdine.agentdeck` | `com.kitdine.agentdeck.debug` |
| 小组件的 `PRODUCT_BUNDLE_IDENTIFIER` | `com.kitdine.agentdeck.widget` | `com.kitdine.agentdeck.debug.widget` |
| `AGENTDECK_APP_GROUP` | `N2FZ2FNRTU.group.com.kitdine.agentdeck` | `N2FZ2FNRTU.group.com.kitdine.agentdeck.debug` |

- 这个 App Group 同时被 entitlements 和两个 Info.plist 中的 `AgentDeckAppGroupIdentifier` 引用（`AgentDeck.entitlements`、`AgentDeckWidget/WidgetSnapshot.swift:77`），所以改一个 build setting 就够了。
- 偏好设置域随 bundle ID 自动分开。
- 小组件的 kind 字符串保持不变，因为扩展的 bundle ID 已经不同，系统会把它们当作不同的小组件。
- `QuotaAlertNotifier` 使用 `Bundle.main.bundleIdentifier`，会自动分开。
- 在开发者账号下注册 Debug App Group 是人工前提。

**安装版 App 的回执。** cask 安装的 App 中，helper 路径就是回执里登记的路径，App 和它调用的 helper 都按已安装处理。从 DerivedData 运行的任何配置，helper 都是未安装的。

## 兼容性与契约同步

- 新增错误码 `state_ownership_refused` 和 `install_claim_refused`；新增公开命令 `install claim`，以及只供安装方式调用的隐藏命令 `install register`（可带 `--source`，在锁内发布文件；cask 另带 `--require-unowned`，用户授权修复时再带 `--authorized-repair`，身份绑定模式还带 `--expected-installation <原 ID>`）、`install unregister`（可带 `--remove`；用户授权修复时可带 `--authorized-repair`，身份绑定清理由 attempt 层再传内部 `--require-identity`）、`install repair-identity --channel homebrew_cask --executable <固定 App helper>`（只读捕获原 installation ID）；cask 的授权修复入口是用户设置的环境变量 `HOMEBREW_AGENTDECK_REPAIR=1`，身份绑定模式须配对 `HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION=<原 ID>`，只出现在安装方式的失败提示和手册里。两种入口及身份捕获、参数传递和拒绝规则以 [cask 无主判定与授权修复](#cask-无主判定与授权修复) 为准；doctor 新增 `installation` 和 `managed_command_owner` 两个检查，以及状态目录被拒绝时的部分报告。按 `cli-design.md` 的版本语义，这属于 MINOR 变更。
- `cli-design.md` 的 Local State 一节还要写明：`~/Library/Application Support/AgentDeck/` 下的 `installation.json` 和 `ownership.lock` 都属于安装所有权，不属于状态目录，也不在便携备份范围内；安装方式私有的清理清单和 helper 副本也不属于便携状态。`installation_id` 只约束旧清理操作，不改变公开 CLI 或运行时路径判定。
- 已安装进程现有的所有输出、退出码、默认目录都不变。
- 主题收尾时同步以下内容：
  - `cli-design.md` 的 Local State（隔离目录、回执位置）、Build Identity and Installation（回执和接管，以及上述隐藏命令、身份绑定参数和配对环境变量）、Output and Errors（错误码表）、Doctor（`installation` 检查）；
  - `cli-manual.md` 的安装和故障排查部分，分别写明 [cask 无主判定与授权修复](#cask-无主判定与授权修复) 中的两条入口：回执丢失或损坏的无主修复使用 `HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask <token>`（Homebrew 不再记录该 cask 时改用 `brew install --cask <token>`）；回执有效、仅清理材料丢失时，先用原 App helper 的 `install repair-identity` 捕获一次原 ID，成功后才在同一条 brew 命令中配对上述两个变量，按该节给出的完整绑定命令执行。身份捕获失败不运行 brew，后续不得刷新 ID 或降级为无绑定重试。无绑定模式只替换无主目标；绑定模式仅允许在锁内精确匹配原 ID/channel/规范 executable 的原 cask 安装修复，不接管其他有效拥有者，也不赋予缺失清单的清理操作删除权；
  - `README.md` 的 `make install` 段落，增加 `TAKEOVER=1` 和 `make install-macos-app`。
- schema-version-signal 的契约和测试保持不变（需求 A8）。

## 风险

| 风险 | 处理 |
| --- | --- |
| 漏掉某个守卫调用点 | 用结构性验收兜底：按 CLI 固定测试枚举出的全部 Cobra 叶子命令，逐一在未安装身份、显式指向（注入的）真实目录下运行，同时在[访问屏障](#测试与验收的隔离)下运行，断言没有任何访问，并且文件不变 |
| 接管瞬间旧拥有者被 Hook 再次启动 | 由所有权锁串行：接管方持有排他锁期间，Hook 进程在取共享锁这一步等待，之后读到新回执而被拒绝；Hook 进程如果先拿到共享锁，接管就一直等到它退出 |
| Hook 频繁调用，导致排他锁拿不到 | 接管方以 `owner_running` 拒绝，回执不变，提示暂停客户端后重试；不会出现两个拥有者 |
| formula 用户首次安装后忘记认领 | 未认领时按未安装运行，不触碰真实状态；`doctor`、TTY 提示和 formula 的 caveats 都会给出 `install claim` 命令。之后的升级由续登自动完成 |
| formula 升级后保留了旧 keg（关闭清理或 pin） | 不续登；旧 keg 仍是拥有者，新 keg 按未安装处理，期间 PATH 中的 `agentdeck` Hook 被静默拒绝。`doctor` 提示 `install claim --takeover` |
| 安装方式在持有排他锁时被暂停或很慢 | 锁一直被它持有，其他进程拿不到锁：已安装进程的会话以 `ownership_busy` 拒绝，其他安装方式以 `owner_running` 失败，不会有两方同时改动；它崩溃时，内核释放锁 |
| 发布过程中崩溃 | 文件发布是原子操作（`rename` 或 `RENAME_SWAP`），目标路径要么是旧内容，要么是新内容；回执在发布成功之后才写入。崩溃后，重新运行同一安装方式即可 |
| 卸载/回滚延迟到另一次同路径安装之后 | 原清单 ID 与新回执不匹配，条件清理跳过；移除 cask 的锁外删除兜底，外层不会继续删最终 App |
| 清理 helper、清单或回执损坏，无法证明归属 | 保留目标并报告未完成，经原安装方式授权重装修复后再普通卸载。cask 区分两条路径：回执缺失/无效时可显式授权替换无主目标；回执有效且仅清理材料丢失时，须由仍有效的 App helper 先捕获原 ID，并在卸载/发布各自的排他锁内核对 ID/channel/规范 executable 后恢复，其他有效拥有者及同路径后继均拒绝。不能捕获或验证时保留对象、报告限制，不承诺任意损坏都可恢复。完整入口与失败原子性见 [cask 无主判定与授权修复](#cask-无主判定与授权修复) |
| 已认领 formula 被卸载后留下过期回执，随后安装 cask | 过期回执按无主处理，cask 不需要 `--takeover`，登记成功后覆盖它；keg 仍在或其他拥有者仍存在时照旧以 `owned_by_other` 拒绝。CI 回归两种情形都验证 |
| 不带 `app` 标签、在 preflight 发布 App 的 cask 在 Homebrew 中的行为不符合预期，包括升级顺序、同版本重装和小组件注册 | 作为交付条件，在 CI 的真实 brew 回归中验证；验证失败时，cask 部分回到设计重新决定 |
| cask 经典 flight 块的写入落点：账户解析、`HOME` 与账户主目录不一致、写入失败（ISO-ARCH-R9-F2） | 经典 `preflight`、`uninstall_preflight` 在 `brew` 进程内以调用者身份运行，`uninstall script:` 由 `brew` 以子进程启动，它们都不在 steps 沙箱中；`writable_paths`、`writable_base` 对它们不适用，本方案也不声明它们，不再有“沙箱拒写”这一层保护。写入边界改由下面两点保证：① 回执、所有权锁、私有尝试目录、helper 副本、清单和外部索引的路径只由 helper 解析并写入，经真实账户（系统账户数据库，不读 `HOME`；仅验收构建可由受限变量覆盖，两者都缺失则失败，不回退）；经典块只生成 attempt ID、传递暂存路径并使用 helper 返回的路径，不用 `HOME`、`~` 或 `Dir.home` 推导任何 AgentDeck 路径；② 所有权排他锁和锁内的条件检查。所以 `HOME` 与账户主目录不一致不是拒绝条件，也不会让回执或材料落到 `HOME` 下，需要的只是“没有影响”。写入失败（账户目录不可写、磁盘满、取锁超时、材料落盘失败）使 preflight 失败，并按“安装及返回前失败”回滚，不换位置重试。CI 的真实 brew 回归把 `HOME` 与账户目录设为不同的临时目录，断言回执、清单、材料、索引只在账户目录；再把账户目录设为不可写，断言非零退出、没有回执和已发布的 App、没有回退写入。验收变量只进验收构建，生产产物不含它 |
| CI runner 同样拦截未公证的验收版 helper | CI 任务先做可行性探测，失败即判 Task 5 未完成并回到设计，不降级为只做声明检查 |
| 开发机上运行真实 brew 回归会触发 Gatekeeper 弹窗 | 回归脚本没有 CI 专用开关时拒绝运行；开发机只运行不执行暂存 helper 的检查 |
| 回执被误删，导致安装版看到空数据 | 回执缺失时安装版按未安装运行，`doctor` 和 Health 面板会明确说明原因；重新安装即可恢复，数据不受影响 |
| 首次升级时，本机现有的手工 0.7.0-dev 安装没有回执 | 用 `make install-macos-app` 重新安装一次即可，这一点写入主题的验收说明 |
