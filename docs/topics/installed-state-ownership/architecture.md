---
status: active
created: 2026-11-05
updated: 2026-11-13
---

# Installed State Ownership — Architecture

依赖 [需求](requirements.md) 与 [CLI 界面](ux/cli.md)。原始现状说明在 `660c815`（`main`，schema 33）上核对；原有的 `文件:行号` 指这个提交。2026-11-13 的 cask 简化修订另以专题 HEAD `fe938e0b6c1d43862391d03721055536f339694e` 核对共享机制消费者，并记录下文 F1–F10 探测事实。本稿是待评审的设计，不表示新模板已实现；历史评审记录保持原样，原架构 Round 12 / 任务分解 Round 15 的 PASS 不覆盖本次变化。

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
make install / local App
  └─ 暂存新程序的 `install register --source`：取得所有权排他锁 → 锁内发布 → 写唯一用户回执 → 释放
cask
  ├─ Homebrew `app "AgentDeck.app"`：创建、替换、删除 App（不受所有权锁约束）
  └─ postflight_steps → 已安装 helper `install register`（无 --source）：排他锁内登记回执
formula `install claim`：排他锁内认领既有 keg；安装步骤不写回执
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

**回执登记真实路径，不登记可变指向的链接（ISO-ARCH-R1-F2 第三轮）。** Repair 2 曾让 formula 登记 opt 链接，并在判定时实时解析。复评证明这样不安全：`brew upgrade` 会在旧会话仍持有共享锁时改变 opt 的指向，新 keg 无需排他锁就被认作拥有者。formula 的修正保留，但不再将“安装对象不可变”推广到 cask：

- formula 登记的是 keg 内的真实路径 `<prefix>/Cellar/<formula>/<version>/bin/agentdeck`。
- 回执自身只在所有权排他锁内改写；登记的文件被删除时回执过期，不授予其他路径所有权。
- make 和本地 App 在可能被登记或已被登记的路径上创建、替换、删除文件，仍由持有排他锁的进程完成，见[安装回执](#安装回执)。
- **用户于 2026-11-13 撤回该文件生命周期规则对 cask 的适用。** Homebrew 可以在锁外替换同一登记路径的 App；新 helper 即使尚未重新登记，也可能凭相同路径匹配旧回执。路径身份不区分同路径前后的二进制世代；这项保证削弱见[风险](#风险)，不与旧设计等价。
- formula 升级产生的是新路径，由下文的[升级续登](#formula-升级续登)在排他锁内完成切换。

**只比路径，不比哈希。** statusline 和 Hook 调用得很频繁，每次对 30 MB 的二进制算哈希代价太高。make/本地 App 的锁内发布保证不变；cask 不再保证文件替换瞬间没有旧会话，也不保证与本地 App 在同路径交错时的延迟清理安全。不为这些风险新增世代、哈希或修复机制。不经过安装方式、手工覆盖已安装文件仍不受支持。`make uninstall` 原有的哈希校验不受影响。

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
- `installation_id` 是每次登记（含 cask 升级的 postflight）、发布、接管或 formula 认领/续登时新生成的 128-bit 随机值，编码为 32 位小写十六进制；同路径、同 channel、同版本重装也必须换值。它保留用于 make/本地 App 的[清理归属校验](#卸载与失败清理)；cask 不再绑定清理 ID。它不参与 `Resolve` 的路径判定，不是另一份安装权威，也不是预留或有期限的凭据；
- `installed_at` 是这次登记的时间，不是第一次安装时间；不要求它或 cask 的 `installation_id` 跨升级保持不变，见下文[升级字段与过期诊断核对](#升级字段与过期诊断核对)；
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

按用户 2026-11-13 对 G9/A9 的决定，“接管”限定为登记路径或安装方式发生变化的回执交接；cask 同路径、同 channel 升级只是 Homebrew 替换文件，不属于接管，不保证替换瞬间无旧会话。但相同路径的 postflight 重登也仍须排他锁，不因该定义获得取锁失败豁免；升级中拒绝与恢复的验收见[缩减回归](#验证位置与缩减回归)。

1. 以非阻塞方式重试取得排他锁，总等待上限 5 秒。取不到，说明仍有拥有者的进程在访问真实状态：拒绝，`reason` 为 `owner_running`，回执不变。
2. 在排他锁内检查现有回执，按[安装回执](#安装回执)的规则决定拒绝还是写入。如果带有 `--source`，就先在锁内原子地发布文件，然后原子写入新回执。
3. 释放排他锁。make/本地 App 的文件发布和回执写入在同一次持锁内；cask 只在这里登记，文件已经由 Homebrew 发布。

**这样得到的保证：**

- 共享锁和排他锁互斥。一旦接管方拿到排他锁，就说明此刻没有任何进程正处在访问真实状态的会话中。
- 接管方释放锁之前，回执已经换成新的。回执转到不同路径后，旧拥有者（含被 Hook 启动的旧路径程序）取得共享锁后因路径不匹配而被拒绝。cask 同路径替换不在这项“旧路径被拒绝”的证明内：旧进程与新文件可能对应同一个登记路径，不能以 ID 改变声称旧世代被拒绝。

对排他锁内的回执接管，评审给出的时序"B 的检查没看到 A → Hook 启动 A → A 读到旧回执并打开 → B 写新回执 → A 继续"仍被阻止；这不约束 cask 在登记前的锁外文件动作：

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

唯一用户回执写在 `account/Library/Application Support/AgentDeck/installation.json`。`account` 由 helper 从系统账户数据库取得，不由 `$HOME` 推导；正常 Homebrew 调用中两者一致。cask 步骤的写入声明以父级 brew 的 HOME 为基准，两者不一致时的拒绝决定见下文。

**make/本地 App 保留锁内发布（ISO-ARCH-R4-F1 第五轮）。** 这两种方式在可能被登记或已被登记的路径上创建、替换或删除文件，仍只能由持有所有权排他锁的进程完成，且在释放锁前更新回执。脚本只准备暂存文件，不自己移动到最终位置或写回执。**cask 明确例外**：由 Homebrew 管理 App 文件，只在 postflight 登记回执。formula 仍由 Homebrew 创建不同路径的新 keg、删除旧 keg；写回执由 claim/续登负责。隐藏命令保留以下接口：

```text
<已安装的 cask helper> install register --channel homebrew_cask --executable <appdir 内 helper>
<暂存的新程序> install register --channel <make|local_app> --executable <目标路径> --source <暂存的文件或 App> --cleanup-manifest <本次安装的清理清单> [--takeover]
<可信的清理 helper> install unregister --channel <make|local_app> --executable <目标路径> --expected-installation <id> --cleanup-manifest <本次安装的清理清单> --remove
```

`install register` 的共同部分在一个进程内、一次连续持有排他锁，依次执行：

1. 以非阻塞方式重试取排他锁，总等待上限 5 秒。取不到就以 `owner_running` 失败，什么都不改。拿到锁，说明此刻没有任何访问会话。
2. 在锁内检查现有回执，决定是否允许：
   - 没有回执、回执无效、回执过期，或回执登记的就是 `<目标路径>` 且安装方式相同（同一安装方式的重装或升级）：允许。make/本地 App 对已有目标的替换授权仍由原安装脚本决定（`FORCE=1`）；cask 的文件动作由 Homebrew 决定，不在 register 中新增替换授权。清理命令不因回执无效而获得删除权；
   - 回执登记的是 `<目标路径>`，但安装方式不同：没有 `--takeover` 时以 `owned_by_other` 拒绝；
   - 回执登记的是另一个仍存在的路径：没有 `--takeover` 时以 `owned_by_other` 拒绝。
3. 为这次登记生成新的 `installation_id`。make/本地 App 传 `--source`：先在锁内写本次清理清单（见[卸载与失败清理](#卸载与失败清理)），再原子发布到目标。暂存内容事先放在目标的同一目录、同一文件系统里。
   - 单个文件：用 `rename(2)` 覆盖；
   - App bundle（目录）：用 `renamex_np(RENAME_SWAP)` 与旧 bundle 原子交换，再删除换下来的旧 bundle。
   - 回执提交前失败则在锁内恢复原目标和回执；提交后的失败按本次清单条件清理，不无条件恢复旧对象。
   - cask 不传 `--source` 或清单：运行命令的程序必须就是被登记的 helper；register 不发布或删除 App。
4. 确认 `EvalSymlinks(<目标路径>) == <目标路径>`，并且是本次发布的程序（有 source）或当前程序（无 source），原子写入新的 ID 和 `installed_at`。
5. 释放排他锁。

这些命令都是隐藏命令，只供安装方式调用，都不访问真实状态目录；冲突和取锁失败仍用 `install_claim_refused` 及已有原因码。失败非零返回给安装方式；make/本地 App 依下节清理，cask 由 Homebrew 回滚。不再提供 cask 的授权修复参数或身份捕获命令，删除/保留范围见下文核对表。

**make/本地 App 仍能闭合原复评的两个反例；不再作为 cask 的证明。**

- **没有需要区分身份的跨步骤状态。** 发布与登记是同一个进程、在同一次持锁内完成的，不存在"先撤销、稍后再由另一个命令完成"的中间状态。第二个安装方式即使用完全相同的参数调用 `install register`，也只能在第一个释放锁之后进入。那时它看到的是第一个已经完成的正式回执，按第 2 步的规则判定；如果它被允许覆盖，也是在它自己持有排他锁、没有任何会话的时候覆盖。所以不需要预留，也不需要事务凭据。
- **没有基于时间的过期。** 不存在会过期的预留，也就不存在"暂停后恢复的旧安装器"在别人登记之后才动手的窗口：安装器的发布动作只能发生在它持有排他锁期间。它被暂停时，仍然持有锁，其他进程都进不来，恢复后在同一次持锁内完成；它崩溃时，内核随进程退出释放锁，而一个不持锁的进程不会去改目标文件。Repair 4 的预留记录、`state` 字段和 10 分钟过期都已删除。

### 卸载与失败清理

**本节仅约束 make/本地 App（ISO-ARCH-R4-F1 第六轮）。** 目标文件或 App 的卸载、失败回滚、重试与损坏安装清理仍通过带预期安装身份的 `install unregister --remove`；helper 返回后不得再对最终路径执行 `rm`、`delete`、`trash`、`rmdir` 或恢复旧对象。后续清理只可删除本次私有暂存文件。cask 不调用 unregister、不清理回执，文件删除由 Homebrew 负责。

**清理清单固定“要清理哪一次安装”。** `register --source` 必须同时传 `--cleanup-manifest`，由持锁的 helper 在发布前以独占创建、`0600`、同步落盘方式写入 `{installation_id, channel, executable, remove_root}`。其中 `remove_root` 是目标文件本身或包含目标 helper 的 App bundle 根，必须是经过校验的绝对物理路径。清单放在 make/本地 App 的目标之外的私有清单目录，保存到卸载/回滚结束。每次安装使用新文件；不覆盖旧清单，也不从当前回执补填或刷新预期 ID。写清单失败则不发布；发布失败留下的清单不授予删除权限。它只是预期参数，不授予状态访问权。

`install unregister` 在一次连续持有的所有权排他锁内执行以下判断和动作；没有无条件删除或 `--takeover` 绕过分支：

1. 取得排他锁，超时以 `owner_running` 拒绝，文件与回执都不改。
2. 校验清单与命令传入的 ID、channel、executable 一致，且 `remove_root` 仅为该可执行文件本身，或以 `Contents/Helpers/agentdeck` 为相对路径的 App 根；不允许任意祖先目录。读取并校验回执结构，而不是直接使用 `Resolve` 的已安装结论：helper 丢失时，回执仍可标识待清理的安装。只有回执的 `installation_id`、`channel`、`executable` 与清单中的预期值全部一致，才允许清理。路径与 channel 一致但 ID 不同也算后来的安装。
3. 回执有效但任一预期值不匹配：幂等跳过，内部结果为 `skipped_superseded`、exit 0，不改回执、不删除文件、不注销小组件。回执和目标都不存在时也是幂等成功。回执缺失/损坏而目标仍存在，或清单缺失/无效时，不能证明归属：保留目标并非零失败，仍用 `install_claim_refused` 与 `receipt_missing` / `receipt_invalid`，提示通过原安装方式重装修复后再卸载。清单只固定预期安装，不单独证明归属；不新增授权容忍分支。公开 `install claim` 的原因和界面不扩展。
4. 匹配时，校验删除根仍是清单指定的普通文件或 bundle，拒绝符号链接和类型不符。在同一持锁期间完成该安装的小组件注销（同步执行 `pluginkit -r` 等需要的清理），撤销其回执，再删除目标；目标已不存在时仅清理匹配回执。根路径不能只因 helper 缺失就重算为另一个路径。任一失败均报告失败，保留尚未清理的文件和清单；不得释放锁后追加兜底删除。撤销后删除失败/崩溃留下的残余按第 3 步恢复，不假装卸载完成。
5. 全部动作结束后释放锁。若外层安装器后来又要求回滚或重复卸载，仍须携带原清单的 ID，重新取锁并执行上述比较；不得把旧操作升级成清理新安装。

**损坏 helper 不等于允许绕过协议。** 本地 App 继续保存私有清理 helper；make 的回滚/卸载继续使用经安装产物校验的临时副本。它们只执行同一个 unregister 协议。副本或清单不可用就报告未完成、保留目标，通过原安装方式恢复，不回退到 shell 直接删除。登记前失败仅能在原 register 持锁期间恢复它已改动的文件；该进程退出后的无匹配回执残余按第 3 步处理。登记后外层步骤失败用本次清单条件清理，不能无条件恢复旧安装覆盖新拥有者。以上不再约束 Homebrew 的 cask 回滚。

### cask 的完整安装与卸载链

**2026-11-13 用户决定：文件生命周期交还 Homebrew。** 恢复 `app "AgentDeck.app"`，由 Homebrew 创建、替换、删除 App。formula 冲突检查仍为原 `preflight_steps`；`conflicts_with`、`binary`、补全与 caveats 保持不变。删除经典 Ruby `preflight` / `uninstall_preflight`，不再自行发布 App 或实现失败升级恢复协议。Homebrew 自身的回滚不是 AgentDeck 的锁内条件清理，不再声称两者等价。

**安装后登记与迁移提示。** 使用已安装的 helper，而非暂存/保留副本；不带 `--source`、清单、接管或授权修复参数。为避免 cask 用户收到无法使用的 `--takeover` 指引，postflight 由 `/bin/sh -c` 薄包装调用 helper（固定 text 格式）。只在 helper 非零且 stderr 含完整的 `  reason: owned_by_other` 行时，由模板替换该次 stderr，输出下面指定的迁移提示；stdout 直通，其他失败的 stderr 和所有 helper 退出码原样保留。临时 stderr 文件仅为本步骤的诊断缓冲，退出删除，不是安装材料、身份凭据或恢复协议。

选择包装而不改 helper 通用提示，是为了保留 make/本地 App/claim 的既有接管文案。代价是新增模板侧诊断适配，必须由渲染/桩与 real-brew 检查证明。**下列是修复后的目标声明，不是 F2/F8 已运行的声明**；两项探测使用的是下文单列的直调 helper 形式，不能为包装后的沙箱可执行性或提示认证。

```ruby
app "AgentDeck.app"

postflight_steps do
  run "/bin/sh",
      args: ["-c", <<~'SH', "agentdeck-cask-register", "{{appdir}}/AgentDeck.app/Contents/Helpers/agentdeck", "@CASK_TOKEN@"],
        set -u
        helper=$1
        token=$2
        errors=$(/usr/bin/mktemp) || exit 1
        trap '/bin/rm -f "$errors"' EXIT
        "$helper" --format text install register --channel homebrew_cask --executable "$helper" 2>"$errors"
        status=$?
        if [ "$status" -ne 0 ] && /usr/bin/grep -Fqx -- '  reason: owned_by_other' "$errors"; then
          printf '%s\n' \
            'install_claim_refused: Another AgentDeck installation owns AgentDeck state.' \
            '  reason: owned_by_other' \
            "  next: Keep the current installation, or uninstall it with its original installation method, then retry this Homebrew operation for $token." >&2
        else
          /bin/cat "$errors" >&2
        fi
        exit "$status"
      SH
      writable_paths: ["Library/Application Support/AgentDeck"],
      writable_base: :home,
      print_stdout: true
end
```

register 只在排他锁内登记：无主（回执缺失、无效或登记路径已删除）可写；相同 cask channel/路径可重新登记；仍有其他 channel 或其他路径的有效拥有者则 `owned_by_other` 拒绝，原回执不变。cask 不传 `--takeover`；该拒绝向用户显示的完整三行就是上面 printf 的 `install_claim_refused`、`reason` 和 `next`（token 由模板渲染），不再转发 helper 的通用 takeover 下一步。用户可保留当前安装；迁移时先用原方式卸载（make uninstall 或 make uninstall-macos-app 等），再重试本次 brew 操作。formula 冲突的原 preflight 提示不变。首次安装的拒绝/回滚曾由直调 helper 版本 F8 证明；包装后的等价退出与迁移提示仍须新回归。不要把“回执未改”推广为同路径竞争中的文件从未被 Homebrew 改过。

**升级中登记被拒（纳入缩减回归，不设豁免）。** 即使同路径升级不算回执接管，postflight 重登仍须取得排他锁。旧 watch、telemetry 会话或频繁 Hook 可使其以 owner_running 非零失败；不能因路径相同而跳过登记、放宽取锁或改为成功。预期最终状态为 brew 非零、Homebrew 恢复旧版本 App 及旧安装记录，旧回执逐字节不变、仍匹配原 helper 路径。用户应先停止占用/暂停客户端，再重试原升级命令；不能把首次安装 F8 的“无 App/无记录”套到升级失败。

这一恢复预期**未实测**。本次只读核对本机 Homebrew 提交 `53c5856230b72e866ee4da77cfb9d39880291a99`（Round 13 记录版本为 7.0.13-97-g53c5856）：`cask/installer.rb` 的 install_artifacts 包含 PostflightSteps，异常清理后上抛；`cask/upgrade.rb` 在异常路径调用旧安装器 revert_upgrade，而 `revert_upgrade` 会恢复备份并重跑旧 artifacts。**恢复时可能再次执行旧 postflight，仍遇到持锁拒绝或重写回执**，所以源码走查不证明旧 App/旧回执满足上述预期，更不证明探测所用 7.0.11 的行为。真实 brew 须验证完整失败及恢复末态；任一条件不成立就保持任务 5 未完成、返回设计，不新增同路径重登豁免或 AgentDeck 恢复机制。

**HOME 不一致：失败，不回退。** `writable_base: :home` 跟随父级 brew HOME，而生产 helper 仍写账户数据库主目录。用户决定两者不一致时登记失败，由 Homebrew 回滚安装；不改写到沙箱 HOME、brew HOME 或其他位置，不加新探测/修复机制。这由 F2 的拒写对照与 F3 推导，**生产 HOME 不一致时的拒绝尚未直接实测**，必须由缩减后的 real-brew 回归补证。步骤内临时 HOME 不等于父级 brew HOME，不据此改变生产账户解析。

**卸载：不清理回执。** 保留 `uninstall quit: "com.kitdine.agentdeck"`，然后由沙箱外的 `uninstall script:` 做小组件条件注销，最后由 Homebrew 删除 App/链接和安装记录。不调用 unregister；不增 zap 删除回执的规则。App 删除后原回执过期，其他安装方式可无 `--takeover` 覆盖它（F7）。卸载命令不传 `--appdir`，回调使用 Homebrew 安装记录恢复的 appdir（F6）。

小组件查询为 `/usr/bin/pluginkit -m -A -D -vv -i com.kitdine.agentdeck.widget`；目标为 `#{appdir}/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex`。生产标识核对自 `apps/macos/AgentDeck.xcodeproj/project.pbxproj` 的 Release 配置；不是探测使用的 `com.kitdine.agentdeck.debug.widget`。只做三条判定：

1. 查询退出码非 0：脚本失败，返回该退出码。
2. stdout 的 `Path = ` 行（允许前导空白）中，有与目标扩展路径**完全相等**的一行：执行 `/usr/bin/pluginkit -r <目标>`，原样返回注销退出码；不做子串/前缀匹配。
3. 其余：视为未登记，跳过并成功。不做空结果措辞白名单，也不按 stderr 非空或 stdout 的 `match:` 文本另加拒绝层。非零查询失败的控制已验证；退出 0 伴诊断及输出格式变化仍是风险，不能声称已经覆盖。

main 的既有卸载缺陷对应 Beads `ad-bug-cask-uninstall-unregistered-widget`。它当前为延期，本次只引用、不改变状态。**新模板不能追溯修复已安装旧 cask 的卸载定义**：从已发布版本升级时，Homebrew 使用安装记录里的旧 caskfile 执行旧卸载；若仍是无条件 pluginkit -r，未登记扩展可在新 postflight 执行前阻断升级。故“新模板已交付且新安装卸载成功”不等于存量用户的首次升级已修好；缺陷关闭还须其载体明确核实/处置该存量升级限制，不能靠设计或探测通过关闭。

此顺序在上述 Homebrew `53c5856` 的 upgrade.rb 中可见（加载 installed_caskfile，旧安装器 start_upgrade 先于新 install_artifacts），并与首轮探测的旧卸载失败相符；不是对所有旧版本的运行验证。真实 /Applications、公证发布 DMG 上该未登记状态是否出现、正常启动旧 App 后能否恢复升级仍未证实。用户侧排障和限制列入下文手册同步清单；这仍由既有缺陷承载，不单独阻塞本稿，不扩入新的安装恢复机制。

#### 探测事实与未验证范围

以下事实直接保存，不依赖会过期的 CI 链接。环境为 GitHub 托管 runner、macOS **26.7.2 Build 25G332 arm64**；Homebrew **7.0.11**，源码提交 `c6069d2a943b71b0a90d91fc1ac93cb9c1634dcc`。使用真实 Homebrew 源码，prefix、Caskroom、缓存、日志、appdir 隔离在临时目录；本地 ZIP、ad-hoc 签名 App。探测分支 `probe/cask-postflight-receipt`，四轮提交依次为 `0855e4fc`、`649cd20f`、`6f2a2f36`、`c6059787`。分支与工作区保留供开发参考，不是本稿已实现的证明。

| 编号 | 观察事实与边界 |
| --- | --- |
| F1 | steps 沙箱内的未公证、ad-hoc helper 可执行，未被终止；不推广到开发机或其他环境。 |
| F2 | 生产 helper（无 `agentdeck_acceptance` tag）按下方原始**直调 helper** 的 postflight 声明在真实账户目录生成 `ownership.lock` 与有效 `installation.json`。删空 writable_paths 后同一调用失败：`chmod /Users/runner/Library/Application Support/AgentDeck: operation not permitted`。该单目录是本次探测的最小声明集合；不是对当前 shell 包装版本的验证。 |
| F3 | `writable_base: :home` 解析到父级 brew HOME，不是步骤内沙箱 HOME，也不是账户数据库主目录；改 brew HOME 到临时目录后，标记文件写入该临时目录。HOME 不一致时生产登记失败并回滚只是 F2/F3 的推导与设计决定，未直接实测。 |
| F4 | 完整安装 1.2.3 → 升级 1.2.4 → 卸载均 exit 0；安装、升级后 `installed=true`。升级再次执行 postflight，ID 从 `02fa9b77535590a791d5433445f88dfb` 变为 `acc2deedc4c0d9ea3b490a9e2381de92`，`installed_at` 从 `2026-11-14T03:49:43Z` 变为 `2026-11-14T03:49:50Z`。另一组省略重新登记的同路径升级仍 `installed=true`，回执逐字节不变。 |
| F5 | 未登记时查询 exit 0、stderr 空、stdout 原文为 `"  (no matches)\n"`。main 的 `if [ -d "$1" ]; then /usr/bin/pluginkit -r "$1"; fi` 因注销 exit 1 / `remove: no plugin at …` 阻断卸载和升级，App/安装记录保留；新三条判定在同一状态跳过并成功。合成查询失败控制非零，未被跳过。真实已登记注销未测。 |
| F6 | `brew uninstall` 不接受 `--appdir`；安装记录 `.metadata/config.json` 的 `explicit.appdir` 保存原路径，卸载回调目标位于该目录。 |
| F7 | 卸载后 App 与 Caskroom 记录不存在，回执仍在且与升级后逐字节一致。留存 helper 副本报告 `installed=false`、`receipt_missing`；另一份不同路径 helper 以 make channel、不带 `--takeover` 登记 exit 0 并覆盖过期回执。副本只是探测工具，不是新 cask 的保留材料。 |
| F8 | 预置登记另一个存活可执行文件的有效回执后装 cask：register 以 `owned_by_other` 拒绝，brew exit 1 并自行回滚；appdir 无 App、Caskroom 无记录、原回执逐字节不变。 |
| F9 | 全部 brew 调用未出现针对探测 cask 的弃用警告；不证明所有 Homebrew 版本兼容。 |
| F10 | 验收 helper 配合 `HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=/private/tmp/…`，steps 沙箱可写该目录；只证明这一临时目录场景，不证明任意账户目录都可写，也不改变 F3。 |

F2 的原始声明（探测事实保留；当前设计的 shell 包装见上文）：

```ruby
postflight_steps do
  run "{{appdir}}/AgentDeck.app/Contents/Helpers/agentdeck",
      args: ["install", "register", "--channel", "homebrew_cask",
             "--executable", "{{appdir}}/AgentDeck.app/Contents/Helpers/agentdeck"],
      writable_paths: ["Library/Application Support/AgentDeck"],
      writable_base: :home,
      print_stdout: true
end
```

本地六项固定输入用桩验证（不调用 brew/真实 PlugInKit）：exit 0 的 `"  (no matches)\n"`、空 stdout、`(0 plug-ins)` 均跳过成功；精确目标路径调用注销桩并原样返回 7；只有其他路径时跳过成功；查询 exit 9 则失败且不注销。额外 stderr / `match:` 拒绝层被删除的依据仅为合成查询失败控制实际非零，不能推广为真实服务故障全部可识别。

**未验证，必须保留：**

- 扩展已登记时注销成功；四轮探测均未调用 `pluginkit -a`；
- 真实 PlugInKit 查询服务故障，尤其 exit 0 但带诊断文本；
- 生产 HOME 与账户数据库主目录不一致时的拒绝与回滚（仅推出）；
- `/private/tmp` 之外的临时账户目录是否可写；
- 真实 `/Applications`、公证发布版 DMG、其他 macOS/Homebrew 版本、Intel。
- 新 shell 包装在真实 Homebrew steps 沙箱中的执行、诊断替换和失败回滚；F2/F8 的直调结果不能直接覆盖它；
- 升级中 postflight 因 owner_running 等原因拒绝后的完整恢复末态，尤其旧 App/安装记录恢复及旧回执不变；本机 7.0.13 源码不能代替 7.0.11 或交付支持版本的实测；
- 已发布旧 cask 的未登记扩展是否阻断真实用户首次升级，以及正常启动旧 App 后重试的处理办法；原缺陷继续承载，不视为新模板已自动修复。

#### 删除与共享机制核对

本次是设计撤除，不在此轮改代码或清理已有磁盘材料。以下逐项核对 `fe938e0b` 的 `scripts/manage-install.sh`、`scripts/install-local-app.sh`、`cmd/agentdeck/install.go`、`internal/installation/install.go` / `attempt.go` 及 cask 模板；不能按 cask 的删减整包删除通用安装协议。

| 机制 | make / 本地 App 消费者核对 | 新设计范围 |
| --- | --- | --- |
| cask attempt 目录、外部索引、`.agentdeck-attempt` 与 `install attempt` 命令组 | 两脚本均不使用；由 cask 模板消费 `attempt.go` | 撤除 cask 的 prepare/complete/cleanup、材料定位与索引设计；不迁移成新机制。 |
| 清理清单与 installation ID 绑定 | make 记录 `cleanup_manifest` / `installation_id`，本地 App 的 install-record 同样记录；卸载/回滚都传原 ID | 仅撤除 cask 清单/绑定；保留通用 `CleanupManifest`、回执 `installation_id`、`unregister --expected-installation --cleanup-manifest --remove`。register 的预期 ID 修复分支不保留。 |
| 保留 helper 副本 | make 回滚/卸载使用校验后的临时副本；本地 App 持久保存私有副本 | 仅撤除 cask 副本；两种本地清理方式不变。 |
| `install repair-identity` | make/本地 App 均不调用，命令本身限定 `homebrew_cask` | 撤除命令、cask 身份捕获与相关恢复验收。 |
| `--require-unowned` | 两脚本均不传；Register 的此分支为 cask source 发布服务 | 撤除该 cask 分支；保留普通 register 无主/相同安装/显式 takeover 规则。 |
| `--authorized-repair`、`--require-identity` | 两脚本均不传；由 cask attempt/绑定修复使用 | 撤除 cask 授权容忍/绑定修复，不删除本地通用条件清理。 |
| `HOMEBREW_AGENTDECK_REPAIR`、`HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION` | 两脚本均不读；仅 cask 模板映射为修复参数 | 撤除两个变量及修复提示；仅验收构建的账户注入变量仍保留。 |
| 经典 `preflight` / `uninstall_preflight` | 两脚本不使用 Ruby flight DSL | 撤除经典 flight 块；保留 formula 冲突用 `preflight_steps`、新增 postflight_steps、保留 quit/script 卸载入口。 |
| 失败升级恢复 O/N/R、cask I4/I5 brew 链 | 本地脚本不使用该恢复协议，但仍有自己的锁内发布失败恢复及条件回滚 | 撤除 cask 专用恢复和对应验收；保留 make/本地 App 的通用发布/清理交错。Homebrew 自身回滚不扩展为新的 AgentDeck 协议。 |

<a id="cask-无主判定与授权修复"></a>
原“cask 无主判定与授权修复”锚点只为历史评审链接保留；旧协议已撤除，不是当前入口。历史轮次不改写，新设计不继承其 PASS。

#### 升级字段与过期诊断核对

1. **没有剩余逻辑要求 cask ID/时间跨升级不变。** 按上述删除边界，cask 的清理清单、attempt、repair-identity 与预期 ID 消费者全部撤出；make/本地 App 只保存各自安装的 ID，不保存 cask ID。`receipt.go` 仍校验 ID 格式和非零时间，并在每次登记生成新 ID/时间；`Resolve`、formula 续登资格、状态/App 守卫只使用回执路径/相应 channel，不比较 ID/时间；`install.register` 输出只报告本次 ID，不建立持久依赖。因此同路径 cask 升级的身份不依赖重新登记，而实际 postflight 仍再次登记，F4 的 ID/时间变化是预期行为，不承诺跨版本恒定。
2. **过期回执与缺失回执共用 `receipt_missing`，无需新诊断。** `Resolver.Resolve` 对登记 executable 的 `Lstat` 不存在返回该原因；`ownerOrReason` 同样归一化为无主。F7 是“文件仍在、登记路径消失”的既有规则，不是误报文件不存在。App 已删除时不能声称运行了该 App 内 helper；探测由留存副本得到该结果。保持 `installed=false` / `receipt_missing`，不增加 stale 分支、字段或恢复提示。

#### 验证位置与缩减回归

真实 brew 仍只在 CI 的 GitHub 托管 macOS runner 执行；脚本无 CI 专用开关即拒绝，开发机不运行会执行 Homebrew 内 helper 的步骤。验收 App 先静态确认带 `agentdeck_acceptance`；prefix、Caskroom、cache、日志、appdir、账户和 HOME 都隔离在临时目录。F10 仅为 `/private/tmp` 夹具可行性，不把这次生产 HOME 探测复制为触碰真实账户的回归。

real-brew 仍按以下五类组织；第 2 类必须含升级中登记被拒后的完整 Homebrew 恢复验证。撤除的 AgentDeck 自定义 O/N/R 恢复协议、cask I4/I5、损坏材料授权修复和真实扩展登记仍不恢复：

1. 安装：`app` + shell 包装的 postflight 成功，helper installed=true，App/回执/链接正确，helper stdout 直通、诊断临时文件不残留；
2. 升级：正常分支 postflight 再执行，ID/时间更新、installed=true、原有状态可用。另用隔离真实 CLI 会话持有共享锁，确认新版本文件已由 Homebrew 发布且实际 postflight 尝试登记后以 owner_running 拒绝；一直运行到 brew 失败/恢复结束，断言 brew 非零、旧版 App 内容/版本与安装记录恢复、旧回执逐字节不变且 executable 仍匹配原路径。用测试留存副本检查旧路径存在与身份，不能只看 App 目录存在；停止占用后按原升级命令重试须成功。记录旧 postflight 是否被再次调用、是否改回执或再次失败；不得提前释放占用、只在发布前失败或只观察首次取锁失败来冒充该末态。任一恢复断言失败就回设计，不加豁免。此组为新定义到新定义的升级，不证明存量旧 cask 的首次升级；
3. 卸载后接管：未登记扩展跳过注销成功，App/Caskroom 消失，回执字节不变且过期；不同路径的 make helper 无 takeover 登记成功；
4. 有效其他拥有者：以不同存活路径建立有效回执，实际包装步骤的 register owned_by_other、brew 非零回滚 App/记录，原回执/对象不变；输出保留 code/reason，包含模板迁移提示、实际 token，不能出现 cask 用不了的 --takeover 指引。不能用 formula preflight 提前拒绝代替；
5. HOME 不一致：验收账户注入与父级 brew HOME 使用不同的隔离目录，登记非零、Homebrew 回滚、无回退回执；F10 的 `/private/tmp` 可写不能冒充该负向证明。该夹具必须先确认声明范围外的账户写入会被拒绝；若环境仍允许写入，记录设计假设未成立并返回设计，不加新机制或假记通过。生产行为仍未实测。

**扩展已登记时的注销验证归已有发布工作流**（`.github/workflows/release.yml` / `cask-recovery.yml`、`scripts/test-macos-distribution.sh` 及其 widget 检查），不进入本 real-brew 回归，不将探测未登记分支等同于已登记注销成功。开发只调整新模板所需的兼容检查，不借此改动发布流程。开发机保留声明渲染、formula 冲突桩、六项注销逻辑桩及本地 make/App 安装协议检查；不把桩当真实原生服务证据。

| 安装方式 | 写入 | 冲突与接管 | 卸载 |
| --- | --- | --- | --- |
| Homebrew cask（`packaging/homebrew/agentdeck-app.rb.tmpl`） | Homebrew `app` 发布，postflight 执行已安装 helper 的无 source register，排他锁内写回执 | 不传 takeover。无主可登记，相同 cask 路径可重登；其他有效拥有者拒绝，Homebrew 自行回滚；同路径文件交错不保证安全 | quit 后按查询退出码/精确路径条件注销；Homebrew 删除 App，回执留下并过期；无清单/helper/ID 绑定 |
| Homebrew formula（`packaging/homebrew/agentdeck.rb.tmpl`） | 安装步骤不写任何回执。首次安装时 caveats 提示运行一次 `$(brew --prefix)/opt/<formula>/bin/agentdeck install claim`，见 [formula 认领](#formula-认领)。升级切换见 [formula 升级续登](#formula-升级续登) | 由 claim 和升级续登处理；keg 为新路径 | 删除 keg 后回执过期，按无主处理；之后安装 cask 不需要 takeover |
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

- cask 同路径升级由 Homebrew 替换 App，不清理旧回执；新 helper 的路径仍匹配时不依赖重新登记即可判为 installed。实际 postflight 会再次登记并生成新 ID/时间，已在 F4 观察到；文件替换不受所有权锁约束；
- formula 升级后自动续登；
- `make install` 用户升级时本来就要重新运行它。

在安装/登记成功的支持环境中，新 cask 定义之间的正常升级无需额外所有权操作，现有数据原样可用（A7 的正常升级路径）；不把它扩展成任意 HOME 或交错时序都保证升级成功。升级中登记被拒须验证 Homebrew 恢复旧 App/记录且旧回执不变，停止占用后重试，不新增同路径取锁豁免。**从已发布旧 cask 升级的第一次卸载仍执行旧定义**，可因未登记扩展而在新模板生效前失败；新模板不自动修复此路径，范围及未证实环境见[既有缺陷说明](#cask-的完整安装与卸载链)，由原缺陷承载，手册须交代处理办法。已经在用 formula 的用户第一次升级到包含本主题的版本后仍需认领一次，因为还没有可续登的回执；未认领时 doctor 和 TTY 提示给出 claim 命令。

**测试隔离。** 安装脚本测试使用临时 `PREFIX`、验收版程序和临时账户目录。cask 写入由 CI 真实 brew 回归验证（见 [cask 的完整安装与卸载链](#cask-的完整安装与卸载链)）；prefix、Cellar、Caskroom、cache、HOME、账户都隔离。正常登记使用父级 brew HOME 与注入账户一致的夹具；不一致单列为预期拒绝/回滚的负向场景，不再要求“不一致也成功”。

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

已安装 App 的共享锁仍会阻止回执接管；cask 保留“先退出 App”，但 Homebrew 文件替换本身不等待该锁，不能据此宣称替换瞬间没有旧 CLI/worker 会话。未安装 App 从不持有长期会话。每轮重新查询覆盖回执被手工删除等变化；查询变为不成立时，App 释放锁、改用隔离目录。`state-location` 的输出如下：

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
- 当前采用 macOS 的 `<Team ID>.<group name>` 标识，无需在开发者门户注册，也无需为该 App Group 配置 provisioning profile。必须核对实际签名团队与 `N2FZ2FNRTU` 前缀一致，并检查 App/Widget 的实际 entitlements 与 Info.plist。此规则不适用于以 `group.` 开头的 provisioned 标识。依据：[Apple：Use app groups that you don’t provision](https://developer.apple.com/documentation/xcode/accessing-app-group-containers#Use-app-groups-that-you-dont-provision)，2026-11-10 核验；这是平台前提勘误，不改变身份表或签名验收。

**安装版 App 的回执。** cask 安装的 App 中，helper 路径就是回执里登记的路径，App 和它调用的 helper 都按已安装处理。从 DerivedData 运行的任何配置，helper 都是未安装的。

## 兼容性与契约同步

- 新增错误码 `state_ownership_refused`、`install_claim_refused`，公开命令 `install claim`；隐藏 register 的无 source 登记用于 cask，`--source --cleanup-manifest` 锁内发布用于 make/本地 App；隐藏 unregister 的原 ID/清单条件清理用于 make/本地 App。cask 专用 attempt、身份捕获、授权参数及配对环境变量从本设计撤除，不再列为用户恢复入口。doctor 新增 installation、managed_command_owner 检查和状态被拒绝时的部分报告。按 `cli-design.md` 版本语义属于 MINOR 变更。
- `cli-design.md` 的 Local State 一节还要写明：`~/Library/Application Support/AgentDeck/` 下的 `installation.json` 和 `ownership.lock` 都属于安装所有权，不属于状态目录，也不在便携备份范围内；安装方式私有的清理清单和 helper 副本也不属于便携状态。`installation_id` 只约束旧清理操作，不改变公开 CLI 或运行时路径判定。
- 已安装进程正常状态访问的输出、退出码、默认目录不变；cask 安装/卸载及保证的变化以本稿为准，不以这句兼容性说明覆盖。
- 主题收尾时同步以下内容：
  - `cli-design.md` 的 Local State（隔离目录、回执位置、过期即 receipt_missing）、Build Identity and Installation（cask 由 Homebrew 管文件/锁内登记，本地方式继续锁内发布与条件清理；不再提供 cask 修复协议）、Output and Errors（错误码）、Doctor（installation 检查）；
  - `cli-manual.md` 的安装和故障排查说明：cask 用 Homebrew 安装/升级/重装，卸载留存过期回执；HOME 不一致须恢复正常调用环境后重试，不回退写入；保留原 formula 冲突与迁移提示。写明两项撤回保证和 PlugInKit 输出风险，不把它们写成与原设计等价，不再宣传已撤除的授权修复入口；
    - owned_by_other 使用模板迁移提示：保留当前安装，或按原安装方式卸载后重试本次 brew 操作；不建议 cask --takeover。owner_running 则停止 watch/telemetry、退出 App、暂停客户端后重试，区分首次安装回滚与升级恢复旧版的预期；
    - 存量旧 cask 若在 `remove: no plugin at …` 处阻断升级，应先保留旧 App/安装记录/回执，确认执行的是旧卸载定义。具备可信、可正常启动的已发布 Release App 时，可按其既有正常启动注册路径启动、退出后再试原升级命令；当前源码的正常启动有注册调用，但**该处理在受影响旧包及真实环境中的有效性尚未验证，不承诺能解决**。仍失败时沿既有缺陷报告并取得经验证的处置，不把更新 tap、新模板、brew reinstall 或 --force 宣称为必然绕过；不自动编辑 Caskroom、手工删 App/回执或执行额外注册。此处列手册待交付内容，不在本轮操作用户安装或修改发布流程；
  - `README.md` 的 `make install` 段落，增加 `TAKEOVER=1` 和 `make install-macos-app`。
- schema-version-signal 的契约和测试保持不变（需求 A8）。

## 风险

| 风险 | 处理 |
| --- | --- |
| 漏掉某个守卫调用点 | 用结构性验收兜底：按 CLI 固定测试枚举出的全部 Cobra 叶子命令，逐一在未安装身份、显式指向（注入的）真实目录下运行，同时在[访问屏障](#测试与验收的隔离)下运行，断言没有任何访问，并且文件不变 |
| 接管瞬间旧拥有者被 Hook 再次启动 | 回执接管仍由所有权锁串行：旧会话先持共享锁则接管拒绝；新回执登记不同路径后旧路径运行时被拒绝。同路径 cask 文件替换不在此证明内，不能按 ID 改变推断旧世代已被拒绝 |
| Hook 频繁调用，导致排他锁拿不到 | 接管方以 `owner_running` 拒绝，回执不变，提示暂停客户端后重试；不会出现两个拥有者 |
| formula 用户首次安装后忘记认领 | 未认领时按未安装运行，不触碰真实状态；`doctor`、TTY 提示和 formula 的 caveats 都会给出 `install claim` 命令。之后的升级由续登自动完成 |
| formula 升级后保留了旧 keg（关闭清理或 pin） | 不续登；旧 keg 仍是拥有者，新 keg 按未安装处理，期间 PATH 中的 `agentdeck` Hook 被静默拒绝。`doctor` 提示 `install claim --takeover` |
| 安装方式在排他锁内被暂停或很慢 | 锁继续阻止真实状态会话及其他回执写入，超时分别为 ownership_busy / owner_running；进程退出时内核释放。make/本地 App 文件动作也受它约束，Homebrew 的 cask 文件动作不受约束 |
| make/本地 App 发布过程中崩溃 | 保留原子发布、回执写入与条件清理规则；不推广为 Homebrew 回滚的保证 |
| **撤回保证：本地 App 与 cask 同路径交错，旧安装延迟清理不影响新安装** | **不再保证。** cask 无清单/ID 绑定，Homebrew 的旧卸载、回滚或后续文件动作可能影响同路径的新 App；本地 helper 的条件清理仍在，但不能保护它之外的 Homebrew 动作。不新增恢复机制；Task 5/8 不得把旧 cask I4/I5 记为通过 |
| **撤回保证：cask 替换文件的瞬间没有旧进程会话** | **不再保证。** uninstall quit 仍先退出 App，但旧 CLI/worker 可能持共享锁，且新文件可能凭同路径旧回执取得身份。register 本身仍等待排他锁，不能倒推此前文件替换也等待了锁 |
| **新增残余风险：PlugInKit 不再输出 `Path = ` 行，或 exit 0 伴诊断** | 三条判定将跳过注销并成功，不因不认识的输出报错；不做空措辞白名单。真实服务故障未测，发布验证另行覆盖已登记注销，不将桩或 F5 当作该证明 |
| make/本地 App 的清理 helper、清单或回执损坏 | 保留目标并报告未完成，由原安装方式恢复；不删通用清理协议。cask 不再依赖这些材料，不为其恢复引入身份绑定入口 |
| formula 或 cask 卸载后留下过期回执 | 登记路径不存在即无主；其他方式无需 takeover 可登记。路径仍在、属于其他有效拥有者时 register 拒绝；cask 的不同路径拒绝/回滚纳入缩减回归 |
| cask `app` + postflight 在其他环境不可行 | F1–F10 只覆盖所列环境；真实 /Applications、公证 DMG、其他版本和 Intel 未测，不声称已验证。交付按缩减的真实 brew 回归及已有发布验证分别取证 |
| 新 shell 包装遗漏迁移提示或改变步骤结果 | 模板只替换 owned_by_other 的错误输出，保留 code/reason、stdout、原退出码；其他失败直通，不给 cask --takeover。真实 steps 包装未测，不能复用 F2/F8 为其认证；安装/有效拥有者两类 real-brew 必须覆盖 |
| 升级重登因旧会话/Hook 取不到排他锁，Homebrew 恢复不完整 | 新 postflight owner_running 仍非零；预期旧 App/记录恢复、旧回执逐字节不变，停止占用后重试。revert_upgrade 会重跑旧 artifacts，可能再次拒绝或写回执；仅源码推演不算已满足，必须运行完整升级失败回归。任一恢复条件失败即回设计，不加重登豁免或自定义恢复 |
| 已发布旧 cask 的卸载脚本阻断首次升级 | 新模板不会替换已安装的旧卸载定义；真实 /Applications/公证包受影响范围及正常启动旧 App 后重试是否有效未证实。由 ad-bug-cask-uninstall-unregistered-widget 继续承载，手册写限制及排障，不因新模板交付即宣称存量路径修复，也不单独阻塞本设计 |
| cask HOME 与账户数据库主目录不一致 | 决定登记失败、Homebrew 回滚、不换位置写回执；基于 F2/F3 推导，生产拒绝尚未直接实测。F10 显示 /private/tmp 账户可写，不能用它证明负向拒绝；负向夹具不能拒绝时返回设计，不增加机制 |
| CI runner 同样拦截未公证的验收版 helper | CI 任务先做可行性探测，失败即判 Task 5 未完成并回到设计，不降级为只做声明检查 |
| 开发机上运行真实 brew 回归会触发 Gatekeeper 弹窗 | 回归脚本没有 CI 专用开关时拒绝运行；开发机只运行不执行暂存 helper 的检查 |
| 回执被误删，导致安装版看到空数据 | 回执缺失时安装版按未安装运行，`doctor` 和 Health 面板会明确说明原因；重新安装即可恢复，数据不受影响 |
| 首次升级时，本机现有的手工 0.7.0-dev 安装没有回执 | 用 `make install-macos-app` 重新安装一次即可，这一点写入主题的验收说明 |

### 保证变化与需求核对

**仍成立的边界：** 未安装进程不能访问或迁移真实状态；当前账户只有一份回执、同一时间只登记一个拥有者路径；跨安装方式的回执接管仍在排他锁内，旧会话阻止接管，切到不同登记路径后旧拥有者运行时被拒绝。这里的“一个拥有者”是现有路径模型，不承诺同路径旧/新二进制世代互斥；上述两项文件生命周期保证已撤回，不能以这些保留边界声称新旧等价。

**用户已确认的需求范围决定（2026-11-13，真实用户修复指令中的 ISO-ARCH-R13-F4 决定）：**

- G7：cask 卸载后回执过期即无主；make install 与本地 App 保持卸载时清理。
- G9/A9：限定为回执接管，即登记路径或安装方式变化的交接；cask 同路径升级由 Homebrew 替换文件，不属于接管，不保证替换瞬间没有旧会话。
- A6：按登记路径定义“一个安装”，同路径前后版本不视为两个安装。

本稿引用上述决定，不再把它们列为“待用户决定”。**requirements.md 原文尚未修改，必须由它自己的设计与评审落地**；这里不替代需求文档的修改/评审，不把文字确认当运行验收通过。

| 条目 | 核对结论 | 已确认范围及后续文本归属 |
| --- | --- | --- |
| G7 | 单一回执/唯一登记路径、安装登记保留 | 按上述决定分方式：cask 过期即无主，make/本地 App 清理；原文由需求流程修订。 |
| G9 | 每次状态访问复核、长驻轮次复核、排他锁内的回执接管保留 | 按上述定义限定接管；同路径 cask 升级无文件瞬间停机保证，重登仍需锁。原文由需求流程加入范围。 |
| A2 | 有效回执下四种安装程序的正常状态访问/迁移不变 | 保留；不扩大为文件生命周期保证或 HOME 不一致时安装也成功。 |
| A6 | 单一回执、旧 keg/双前缀/stable/rc、过期无主、不同路径拒绝/显式接管仍成立 | 按用户已确认的登记路径定义验收；同路径前后版本不是两个安装。需求流程补入该定义，不声明世代互斥。 |
| A7 | 成功的正常升级仍由 postflight 登记；ID/时间变化无依赖冲突，现有数据不因所有权协议另迁移 | 保留正常升级验收；不承诺 HOME 不一致或已接受交错风险下升级必成功。 |
| A9 | 不同登记路径的旧拥有者拒绝、旧会话阻止回执接管保留 | 按 G9 的同一定义限定；cask 同路径升级不是接管。决定已取得，需求文本和实际验收分别待各自流程完成，不因决定即声称 A9 已完整验收。 |

#### requirements.md 需单独修订的原文

以下逐字引用当前需求，只标明后续设计/评审需落地的范围，**本轮不改 requirements.md**：

- **G7，第 56 行：**

  > Homebrew cask、`make install`、本地 App 安装在安装时写入这份回执，卸载时清理。

  后续按决定分开 cask 的过期无主与两种本地安装的卸载清理。

- **G9，第 59 行：**

  > **G9 所有权交接要安全。** 安装身份不在进程内永久缓存：每次打开或加锁真实状态之前，都要重新确认自己仍是回执登记的拥有者；长驻进程每一轮都要复核，发现所有权已转移就停止访问并退出。接管只能在旧拥有者停止访问真实状态之后进行：真实状态的锁仍被持有、或已安装的 App 仍在运行时，接管会被拒绝（2026-11-05 用户决定）。

  后续明确“接管”是登记路径或安装方式变化的回执交接，cask 同路径升级不在该文件替换停机保证内；身份/状态守卫本身不撤回。

- **A6，第 109 行：**

  > **A6** 同一时间只有一个安装能拥有真实状态，包括以下情形：保留下来的旧 Homebrew keg、不同的 Homebrew 前缀、stable 与 rc 两个 formula 同时安装、回执过期、在不同安装方式之间迁移。已有回执登记着另一个仍然存在的程序时，安装方式或认领命令要么拒绝并给出迁移提示，要么在用户显式同意替换后接管。"显式同意"的具体形式由[架构](architecture.md#安装回执)定义。

  后续加入“一个安装”按登记路径定义，同路径前后版本不视为两个安装；不同路径/方式拒绝及迁移提示不删。

- **A9，第 110 行：**

  > **A9** 接管完成之后，旧拥有者的进程（包括长驻进程和 App）不能再打开、加锁或读取真实状态。旧拥有者仍在访问时，接管会被拒绝，回执保持不变。

  后续沿用 G9 的回执接管定义，明确同路径 cask 升级例外，不扩成无条件文件生命周期保证。

- **连带安装范围文字，第 69、91 行：**

  > 不改动发布、公证、tap 自动化的流程，只在安装和卸载步骤里增加回执的写入与清理。

  > 写入和清理回执；强制只有一个拥有者，接管前要求旧拥有者已停止访问；formula 的 caveats 引导用户认领。

  同一需求流程按上述已确认决定核对这两处通用概述，避免又把 cask 写成卸载清理；不扩大本次决定，也不在本轮顺手改写需求的其他历史文字。A2/A7 原文保留正常路径，无新决定。

任务 8 旧条目保留作对照，适用范围按本次决定同步，见 [cask 验收适用性核对](tasks.md#cask-验收适用性核对)。需求文本落地、新的独立设计评审及运行验收是后续各自的边界；不得把它们写成用户尚未作决定，也不得用旧 PASS 或事故范围外反例自动恢复已删除机制。
