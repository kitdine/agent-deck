---
status: active
created: 2026-11-05
updated: 2026-11-06
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
| 端到端验收 | 带 build tag `agentdeck_acceptance` 构建的验收二进制，例如 `go build -tags agentdeck_acceptance -o <临时目录>/agentdeck ./cmd/agentdeck` | 只有这个 tag 下才会编译 `resolver_acceptance.go`，它从 `AGENTDECK_ACCEPTANCE_ACCOUNT_HOME` 读取临时账户主目录，回执路径随之落到该目录下。scan worker 和 helper 都是同一个可执行体，并继承这个环境变量，所以子进程同样被隔离 |
| 生产产物 | `make build`、`make build-all`、App 打包、formula 和 cask 的产物 | 不带这个 tag。只做有限的静态验证：用 `go tool nm` 确认产物中没有验收 Resolver 的符号，并确认构建命令中没有这个 tag。生产产物的运行时行为由前两层覆盖 |

验收 tag 只是测试工具，不是生产后门：

- 发布构建、Makefile 的 `build` 和 `build-all`、App 打包脚本、Homebrew 模板都不得带这个 tag；
- 上面的符号检查加入 `check-install` 和发布前检查。

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
<暂存的新程序> install register --channel <c> --executable <目标路径> [--source <暂存的文件或 App> --cleanup-manifest <本次安装的清理清单>] [--takeover]
<可信的程序> install unregister --channel <c> --executable <目标路径> --expected-installation <id> --cleanup-manifest <本次安装的清理清单> [--remove]
```

`install register` 在一个进程内、一次连续持有排他锁，依次执行：

1. 以非阻塞方式重试取排他锁，总等待上限 5 秒。取不到就以 `owner_running` 失败，什么都不改。拿到锁，说明此刻没有任何访问会话。
2. 在锁内检查现有回执，决定是否允许：
   - 没有回执、回执无效、回执过期，或回执登记的就是 `<目标路径>` 且安装方式相同（同一安装方式的重装或升级）：允许；无效回执的恢复仍须满足安装方式已有的目标替换授权（如 `FORCE=1`），清理命令本身不因此获得删除权；
   - 回执登记的是 `<目标路径>`，但安装方式不同：没有 `--takeover` 时以 `owned_by_other` 拒绝；
   - 回执登记的是另一个仍存在的路径：没有 `--takeover` 时以 `owned_by_other` 拒绝。
3. 为这次登记生成新的 `installation_id`。传了 `--source` 时，先在锁内写好本次安装的清理清单（见[卸载与失败清理](#卸载与失败清理)），再把暂存的内容原子地发布到 `<目标路径>`。暂存内容事先放在目标的同一目录、同一文件系统里。
   - 单个文件：用 `rename(2)` 覆盖；
   - App bundle（目录）：用 `renamex_np(RENAME_SWAP)` 与旧 bundle 原子交换，再删除换下来的旧 bundle。
   - 任何一步失败，目标路径保持原样，回执也不改，然后释放锁并报错。
4. 确认 `EvalSymlinks(<目标路径>) == <目标路径>`，并且它就是刚发布的程序，然后原子写入含本次 `installation_id` 的回执。
5. 释放排他锁。

这些命令都是隐藏命令，只供安装方式调用，都不访问真实状态目录；冲突和取锁失败使用 `install_claim_refused` 及已有原因码。清理时无法确认归属的结果见下节。安装脚本把失败原样转给用户，并以非零状态结束。

**为什么这样能闭合复评的两个反例。**

- **没有需要区分身份的跨步骤状态。** 发布与登记是同一个进程、在同一次持锁内完成的，不存在"先撤销、稍后再由另一个命令完成"的中间状态。第二个安装方式即使用完全相同的参数调用 `install register`，也只能在第一个释放锁之后进入。那时它看到的是第一个已经完成的正式回执，按第 2 步的规则判定；如果它被允许覆盖，也是在它自己持有排他锁、没有任何会话的时候覆盖。所以不需要预留，也不需要事务凭据。
- **没有基于时间的过期。** 不存在会过期的预留，也就不存在"暂停后恢复的旧安装器"在别人登记之后才动手的窗口：安装器的发布动作只能发生在它持有排他锁期间。它被暂停时，仍然持有锁，其他进程都进不来，恢复后在同一次持锁内完成；它崩溃时，内核随进程退出释放锁，而一个不持锁的进程不会去改目标文件。Repair 4 的预留记录、`state` 字段和 10 分钟过期都已删除。

### 卸载与失败清理

**完整清理链都必须遵守同一边界（ISO-ARCH-R4-F1 第六轮）。** 删除目标 App 的唯一入口是带预期安装身份的 `install unregister --remove`。正常卸载、升级旧版本卸载、安装失败后的 Homebrew 回滚、重试与损坏安装清理都使用它；不在 helper 返回后再对最终路径执行 `rm`、`delete`、`trash`、`rmdir` 或恢复旧 bundle。后续清理可以删除本次安装私有的暂存文件，不能跟随链接去删最终路径。

**清理清单固定“要清理哪一次安装”。** `register --source` 必须同时传 `--cleanup-manifest`，由持锁的 helper 在发布前以独占创建、`0600`、同步落盘方式写入 `{installation_id, channel, executable, remove_root}`。其中 `remove_root` 是目标文件本身或包含目标 helper 的 App bundle 根，必须是经过校验的绝对物理路径。清单放在最终安装路径之外的本次安装私有目录（cask 的 Caskroom 安装记录、本地安装的清单目录），由相应安装方式保存到卸载/回滚结束。每次安装使用新的清单文件；不能覆盖旧清单，也不能在清理时从“当前回执”补填或刷新预期 ID。写清单失败则不发布；发布失败留下的清单不授予删除权限。它只是一次操作的预期参数，不授予状态访问权。

`install unregister` 在一次连续持有的所有权排他锁内执行以下判断和动作；没有无条件删除或 `--takeover` 绕过分支：

1. 取得排他锁，超时以 `owner_running` 拒绝，文件与回执都不改。
2. 校验清单与命令传入的 ID、channel、executable 一致，且 `remove_root` 仅为该可执行文件本身，或以 `Contents/Helpers/agentdeck` 为相对路径的 App 根；不允许任意祖先目录。读取并校验回执结构，而不是直接使用 `Resolve` 的已安装结论：helper 丢失时，回执仍可标识待清理的安装。只有回执的 `installation_id`、`channel`、`executable` 与清单中的预期值全部一致，才允许清理。路径与 channel 一致但 ID 不同也算后来的安装。
3. 回执有效但任一预期值不匹配：幂等跳过，内部结果为 `skipped_superseded`、exit 0，不改回执，不删除文件，不注销其小组件。回执和目标都不存在时也是幂等成功。回执缺失/损坏而目标仍存在，或清单缺失/无效时，不能证明归属：保留目标，以非零状态报告失败；隐藏命令复用 `install_claim_refused` 和已有的 `receipt_missing` / `receipt_invalid` 原因，提示通过原安装方式重新安装修复回执后再卸载。公开 `install claim` 的原因和界面不扩展。
4. 匹配时，校验删除根仍是清单指定的普通文件或 bundle，拒绝符号链接和类型不符。在同一持锁期间完成该安装的小组件注销（同步执行 `pluginkit -r` 等需要的清理），撤销其回执，再删除目标；目标已不存在时仅清理匹配回执。根路径不能只因 helper 缺失就重算为另一个路径。任一失败均报告失败，保留尚未清理的文件和清单；不得释放锁后追加兜底删除。撤销后删除失败/崩溃留下的残余按第 3 步恢复，不假装卸载完成。
5. 全部动作结束后释放锁。若外层安装器后来又要求回滚或重复卸载，仍须携带原清单的 ID，重新取锁并执行上述比较；不得把旧操作升级成清理新安装。

**损坏 helper 不等于允许绕过协议。** cask 在其私有安装记录中保留可执行的清理 helper 副本；本地安装同样保留，或使用经过安装产物校验的暂存 helper。它们只执行同一个 `unregister` 协议，不依赖最终 App 内的 helper 完好。副本或清单不可用就报告未完成，保留目标并走重新安装恢复；禁止回退到 shell 直接删除。安装失败发生在成功登记之前时，回滚仅能在原 `register` 持锁期间恢复它已改动的文件；该进程退出后，没有匹配回执的残余也按第 3 步处理。成功登记后外层步骤失败，则用本次清单条件清理，不能无条件恢复旧安装覆盖新拥有者。

### cask 的完整安装与卸载链

**cask 改为由 install step 发布 App。** Homebrew 的 `app` 标签会在我们的进程之外移动 App，无法纳入这把锁，所以 cask 不再使用 `app` 标签：

- install step 保存清理 helper，并以 `run` 执行暂存目录中的 helper：`#{staged_path}/AgentDeck.app/Contents/Helpers/agentdeck install register --channel homebrew_cask --executable #{appdir}/AgentDeck.app/Contents/Helpers/agentdeck --source <按上文同文件系统要求准备的暂存 App> --cleanup-manifest <本次清单>`。
- uninstall 步骤先沿用 `uninstall quit` 请求退出 App，再由保存的清理 helper 执行 `install unregister --channel homebrew_cask --executable <原 helper 路径> --expected-installation <原清单 ID> --cleanup-manifest <原清单> --remove`；小组件注销放进通过归属校验后的持锁阶段，不保留独立的路径式注销脚本。
- **移除 `uninstall delete:` 兜底。** `uninstall`、`zap`、postflight、回滚及 `--force` 路径均不得声明能覆盖最终 App/其父目录的删除规则，也不得自行恢复最终 App。Homebrew 的 `binary` 和补全链接仅管理链接本身，不可递归删除其指向的 App；私有 Caskroom 清理也不能跟随链接删除最终 App。helper 缺失或失败时即使 Homebrew 跳过脚本，也没有另一条删除 App 的路径。
- 回执目录、`appdir`、私有清单和清理 helper 目录都要声明为 step 的写入路径。真实 brew 测试须确认 Caskroom 的清单/helper 在所有清理调用结束前仍可用。

升级时，Homebrew 先执行旧版本的 uninstall 步骤（请求退出 App 后，在锁内核验、注销并删除），再执行新版本的 install step（在锁内发布并登记）。旧卸载成功后若另一个安装先登记了同一路径，旧卸载链的后续步骤不再操作最终 App；延迟的条件清理也会因 ID 不同跳过。新版本 install step 仍按锁内冲突规则处理；失败后的 Homebrew 回滚只能使用新版本本次清单，不能删去竞争者或在锁外恢复旧 App。

以下几点都必须在真实 brew 回归测试中实际验证，任一项不成立，cask 部分就要回到设计重新决定：

- 不带 `app` 标签、只用 install step 发布 App 的 cask，能正常安装、升级、卸载；
- 小组件注册（`test-cask-widget-registration.rb`）仍然有效；
- 升级时 uninstall 步骤先于 install step 执行。
- 完整卸载、失败回滚和损坏 helper 清理都遵守上述条件删除；在旧 helper 返回后插入同路径新安装，继续所有剩余 Homebrew 步骤（含强制路径），新 App 与回执保持完整。覆盖同 channel 重装，不能只靠 channel 不同阻止误删。

| 安装方式 | 写入 | 冲突与接管 | 卸载 |
| --- | --- | --- | --- |
| Homebrew cask（`packaging/homebrew/agentdeck-app.rb.tmpl`） | 按上文，由 install step 执行带 `--source` 的 `install register`，在锁内发布 App 并登记 | Homebrew 不提供接管参数，所以不传 `--takeover`。回执登记了别的安装时，以 `owned_by_other` 失败，Homebrew 条件回滚，并打印迁移命令，格式沿用现有的 formula 冲突提示 | 由保存的 helper 携带本次清单身份执行 `install unregister --remove`，无路径式兜底 |
| Homebrew formula（`packaging/homebrew/agentdeck.rb.tmpl`） | 安装步骤不写任何回执。首次安装时，caveats 提示用户运行一次 `$(brew --prefix)/opt/<formula>/bin/agentdeck install claim`，见 [formula 认领](#formula-认领)。升级后的切换见 [formula 升级续登](#formula-升级续登) | 由 `install claim` 和升级续登处理。keg 是新路径，Homebrew 创建它时不会碰到已登记的文件 | 卸载（或升级后清理）删除 keg，回执随之过期，按"不存在"处理 |
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

- cask 升级时，旧版本的 uninstall 步骤和新版本的 install step 各自在排他锁内完成注销和发布；
- formula 升级后自动续登；
- `make install` 用户升级时本来就要重新运行它。

所以按受支持方式升级的用户不需要额外操作，现有数据原样可用（需求 A7）。已经在用 formula 的用户，第一次升级到包含本主题的版本后，需要认领一次，因为这时还没有可以续登的回执；未认领时，`doctor` 和 TTY 提示会给出 `install claim` 命令。

**测试隔离。** 安装脚本的测试已经使用临时 `PREFIX`；新的回执写入同样只写入临时 `HOME`。cask 的写入在现有的真实 brew 回归测试中验证：`scripts/test-cask-migration.sh` 运行真实的 `brew install --cask`，并把 prefix、Cellar、Caskroom、cache 和 `HOME` 都指向临时目录。

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

- 新增错误码 `state_ownership_refused` 和 `install_claim_refused`；新增公开命令 `install claim`，以及只供安装方式调用的隐藏命令 `install register`（可带 `--source`，在锁内发布文件）、`install unregister`（可带 `--remove`）；doctor 新增 `installation` 和 `managed_command_owner` 两个检查，以及状态目录被拒绝时的部分报告。按 `cli-design.md` 的版本语义，这属于 MINOR 变更。
- `cli-design.md` 的 Local State 一节还要写明：`~/Library/Application Support/AgentDeck/` 下的 `installation.json` 和 `ownership.lock` 都属于安装所有权，不属于状态目录，也不在便携备份范围内；安装方式私有的清理清单和 helper 副本也不属于便携状态。`installation_id` 只约束旧清理操作，不改变公开 CLI 或运行时路径判定。
- 已安装进程现有的所有输出、退出码、默认目录都不变。
- 主题收尾时同步以下内容：
  - `cli-design.md` 的 Local State（隔离目录、回执位置）、Build Identity and Installation（回执和接管）、Output and Errors（错误码表）、Doctor（`installation` 检查）；
  - `cli-manual.md` 的安装和故障排查部分；
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
| 清理 helper、清单或回执损坏，无法证明归属 | 保留目标并报告未完成，经原安装方式重新安装修复后再卸载；不以自动删除代替归属证明 |
| 不带 `app` 标签、只用 install step 发布 App 的 cask 在 Homebrew 中的行为不符合预期，包括升级顺序和小组件注册 | 作为交付条件，在真实 brew 回归测试中验证；验证失败时，cask 部分要回到设计重新决定 |
| cask 安装步骤写 `$HOME` 被沙箱拒绝 | 作为交付条件，在真实 brew 回归测试中验证。验证失败时，cask 这一行回到设计重新决定，不能静默改用别的方式 |
| 回执被误删，导致安装版看到空数据 | 回执缺失时安装版按未安装运行，`doctor` 和 Health 面板会明确说明原因；重新安装即可恢复，数据不受影响 |
| 首次升级时，本机现有的手工 0.7.0-dev 安装没有回执 | 用 `make install-macos-app` 重新安装一次即可，这一点写入主题的验收说明 |
