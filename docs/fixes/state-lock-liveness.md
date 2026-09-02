---
status: active
created: 2026-09-02
---

# 缺陷：残留 state.lock 会永久锁死 CLI

## 现象

v0.5.0-rc.4 安装验证的第 6 项发现：进程异常退出后留下的
`state.lock` 会让所有后续写状态命令等待到 `ErrStateBusy`，只能由用户手工
删除。本机从 2026-08-30 到 2026-09-01 因此无法读取真实状态；数据库没有损坏，
只是被残留锁持续挡住。`scan.lock` 经过同一个 `acquireNamedLock` 路径，具有相同
缺陷。

聚焦回归测试先启动并等待一个子进程退出，再分别把该死亡 PID 写入
`state.lock` 和 `scan.lock`。生产代码修改前，两次获取都稳定失败：

```text
scripts/run-go-test.sh ./internal/store \
  -run '^TestAcquireNamedLockReclaimsDeadOwner$'

store_test.go:1253: acquire after owner exit: state_busy: timed out waiting for state lock
go test failed with status 1
```

## 根因

[`acquireNamedLock`](../../internal/store/store.go)（`internal/store/store.go:394`）
原来只用 `O_CREATE|O_EXCL` 创建锁文件。文件存在时，它在超时前重复轮询，但既不
记录持有者 PID，也不检查持有进程是否仍然存在，因此进程退出后没有任何路径能把
残留文件与仍被持有的锁区分开。

[`checkLock`](../../internal/doctor/doctor.go)（`internal/doctor/doctor.go:197`）
会把修改时间超过十分钟的 `state.lock` 报告为 `stale_lock`，但 Doctor 是只读
诊断；该判断没有参与获取，也不能安全证明一个长时间运行的持有者已经死亡。

## 修复边界

- 新锁 token 使用 `v1:<pid>:<128-bit nonce>`。旧二进制只把自己写入的 token
  当作不透明字节做 release 比较，因此仍能读取并正确处理这种格式；新二进制对旧
  随机 token 保持保守，不因无法确定持有者而删除它。
- 自动回收只覆盖本修复版本及之后写入的 `v1:` 锁。升级不会解开升级前已经残留的
  32 字符十六进制随机 token，也不能处理本修复正式发布前由旧二进制留下的新残留锁；
  这包括 现象 一节记录的 2026-08-30 至 2026-09-01 事故类型。旧 `state.lock`
  仍由 `agentdeck doctor` 的 `stale_lock`（修改时间超过十分钟）发现；确认没有
  AgentDeck 进程仍在运行后，用户仍须手工删除该锁。Doctor 不检查 `scan.lock`，
  旧格式的残留 `scan.lock` 同样需要在确认没有扫描进程后手工删除。
- macOS 上以 signal 0 查询 PID：成功或 `EPERM` 都视为存活，只有明确的
  `ESRCH` 才视为死亡；其他结果与非 macOS 平台均视为未知并保持 `state_busy`。
- 确认死亡后，删除前再次读取并比较完整 token。若另一个持有者已替换文件，获取方
  不删除新 token。并发回收者还必须先在同一个锁文件 inode 上取得非阻塞
  `flock`；只有一个回收者能执行存活检查与删除，等待者不会跨 inode 删除新锁。
  PID 被复用时也只会保守地保持 busy，不会错误回收。
- `state.lock` 与 `scan.lock` 共用同一回收实现；现有排他性、当前所有者 token
  校验、等待超时与 context 取消语义不变。
- 不修改 Doctor 报告、SQLite schema、数据库内容、CLI 错误码或用户运行时配置。

## 验证

- RED：上述聚焦测试在生产代码修改前失败，`state.lock` 与 `scan.lock` 均返回
  `state_busy`；测试日志 SHA-256 为
  `3cbb9d70f111d1105e03b5fa6835cca7a6280f94aed18e19339e32d29b3bf198`。
- GREEN：同一聚焦测试在修复后通过；日志 SHA-256 为
  `e7fa79b6557efd5b6fd1a9fd4e65f8901a0e07c8f2c7dc996e63e4739e3ec709`。
- 锁边界：`scripts/run-go-test.sh ./internal/store -run
  '^(TestAcquireNamedLock|TestReclaimLockFileGuard|TestLock|TestAcquireScanLock|TestOpenRespectsMigrationLock)'`，
  PASS；日志 SHA-256 为
  `ebfa3313e8947c6d2701baaf72281ad5164acd2bb0c8eb34216113f13d7d98e0`。
  覆盖死亡持有者回收、存活/未知/旧 token 保留、替换竞态、release ownership、
  并发回收串行化以及 state/scan 锁独立性。
- L3：`scripts/run-go-test.sh ./...`、
  `scripts/run-go-test.sh -race ./internal/store`、
  `go vet -mod=vendor ./internal/store` 均 PASS；Darwin arm64 与 amd64 的
  `go build -mod=vendor -trimpath ./cmd/agentdeck` 均 PASS。全套与 race 日志的
  SHA-256 分别为
  `15b7a899e186cbf72150e6e02571d2ae270b1dd46ee8b91185a58707ef7a37df`、
  `14baeda6524ce5935c34e9846dd8e52b6e4bae47fb34c23aee309fec2ffc2d53`。
- L0：`make check-whitespace` 与 `git diff --check` 均 PASS。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `853f02f4ee83a7dd603b8a436cce835d71534cdb` 加未提交工作区，
  blob:
  `96f74307ceca22daa4112842ea6d1503f23c7f30 internal/store/store.go`、
  `f5e31abc7808b76dbe1bbf0bbb3bfe18b11bfa31 internal/store/store_test.go`、
  `1f8a9fe0a5de7c63c3a2a1c50c6413dbe2f26888 internal/store/process_darwin.go`、
  `023a30e98eae878b7857a176dbba4fff73ddbead internal/store/process_unsupported.go`、
  `5dff20426279c6613f3704148abff5408d43ff47 docs/fixes/state-lock-liveness.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 仓库核对 + 实测。把 `TestAcquireNamedLockReclaimsDeadOwner` 逐字抽到
  HEAD worktree 单独运行以独立复现 RED；用候选二进制在隔离 `--state-dir` 上分别
  造「旧格式 token」与「`v1:` + 已死 PID」两种残留锁，观察真实 CLI 行为；全量
  `scripts/run-go-test.sh ./...`、`-race ./internal/store`、`go vet`。
  `docs/fixes/**` 无项目自带的文档集校验器（`scripts/check-topic-docs.sh` 只读
  `docs/topics/**`），该类证据缺席而非未运行。
- Scope: `acquireNamedLock` 的回收路径与 token 格式、两个平台文件、四条新回归，
  以及本记录的 现象 / 根因 / 修复边界 / 验证 四节。Doctor、schema、CLI 错误码
  确未改动。
- Findings:
  - [P2] `R1-F1` 现象 记录的那次事故不会被本次修复治愈，而记录没有说。
    回收要求 token 是 `v1:<pid>:<nonce>`（`internal/store/store.go:524`
    `lockOwnerPID`），而这个格式是本次修复才引入的
    （`internal/store/store.go:516` `newLockToken`）。已发布的每个二进制写的都是
    旧的 32 位十六进制随机 token，对它们 `lockOwnerPID` 返回 not-ok，回收直接
    放弃。也就是说：**升级到本修复并不能解开机器上已经存在的那把锁，也解不开
    下一个版本发布之前任何一次崩溃留下的锁**——那正是 现象 一节开头记的
    8/30–9/1 那次事故的那一类。实测（候选二进制，隔离 `--state-dir`）：
    旧格式 token 残留锁 -> `state_busy: timed out waiting for state lock`；
    同一二进制对 `v1:<已死 PID>:<nonce>` -> 立即回收并正常返回。
    修复边界 只用机制语言写了「新二进制对旧随机 token 保持保守」，没有把这个
    结论说出来，读者会以为 现象 里的事故已经不会再发生。
    💡 边界内的改法：在 修复边界 里写明回收只覆盖本版本及之后写入的锁，升级前
    留下的锁仍需手工删除，并指出 `agentdeck doctor` 的 `stale_lock`
    （`internal/doctor/doctor.go:197`，按 mtime 超过十分钟判定）就是发现它的
    途径。若要连旧锁一起回收，那需要引入一个时间阈值，属于新的用户可见决定，
    应重新分流而不是塞进本次修复。-> open
  - [nit] `R1-F2` `flock` 失败会把「忙」变成硬错误。
    `tryLockReclaimFile` 的错误经 `internal/store/store.go:451` 冒泡到 `:425`，
    使 `AcquireLock` 返回裸 errno 而不是 `ErrStateBusy`。调用方普遍按
    `errors.Is(err, ErrStateBusy)` 分支，因此一次 `ENOLCK`/`EINTR` 之类的瞬时
    失败会变成一个它们不认识的错误，而修复前这条路径只会退化为等待到超时。
    💡 拿不到回收守卫就是「没拿到」，返回 `false, nil` 让既有的超时路径接管，
    行为与修复前一致。-> open
  - [nit] `R1-F3` `state_busy` 不告诉用户任何可执行的下一步。指向本次改动之外：
    错误文案是既有实现，且 修复边界 明确把 CLI 错误码排除在外。但 `R1-F1` 之后
    它就是旧锁场景下用户唯一的信号，而实测输出只有
    `state_busy: timed out waiting for state lock`，没有提到锁文件、doctor
    或任何补救动作——这正是那次事故拖了三天的原因。
    💡 归属另一个承载体：建一条 Beads issue，或记入 `roadmap.md` Backlog。
    本轮不要求在此修复。-> open，承载体
    `ad-bug-state-busy-recovery-guidance`（Round 1 Repair 建立，Round 2 复评补记
    于此，使承载关系与 finding ID 出现在同一条目内）
- Evidence:
  - RED 独立复现：把 `TestAcquireNamedLockReclaimsDeadOwner` 抽到 HEAD worktree
    运行，`state lock` 与 `scan lock` 两个子用例都以
    `acquire after owner exit: state_busy: timed out waiting for state lock`
    失败，与记录引用的报错一致。
  - `env GOCACHE=… scripts/run-go-test.sh ./...`：PASS，20 个包 `ok`，无
    `--- FAIL`。
  - `env GOCACHE=… scripts/run-go-test.sh -race ./internal/store`：PASS；
    `go vet -mod=vendor ./internal/store`：PASS。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 真实 CLI 行为（隔离 `--state-dir`，候选二进制）：旧格式 token 残留锁使
    `provider list` 输出 `state_busy: timed out waiting for state lock`；换成
    `v1:99991:<nonce>` 后同一命令正常输出且锁文件被删除。
- Completion gate: NOT_REQUIRED —— 现行 CEv1 契约未定义 Lane A 边界，该流程缺口
  由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: REOPEN

### 下一步指令

修复：fix / state-lock-liveness / R1-F1 R1-F2 R1-F3

## Repair — Round 1 — 2026-09-02

- `R1-F1` closed：修复边界已明确自动回收只覆盖本修复版本及之后写入的 `v1:`
  锁。升级前已有、以及正式发布前由旧二进制新留下的随机 token 锁仍不会自动解开；
  `state.lock` 由 `agentdeck doctor` 的 `stale_lock` 发现，并在确认没有 AgentDeck
  进程存活后手工删除。Doctor 不检查旧 `scan.lock`，其手工删除也必须先确认没有
  扫描进程。
- `R1-F2` closed：`acquireNamedLockWithChecks` 增加可注入的回收守卫边界；
  `tryReclaimLock` 返回错误时，`reclaimLockFromDeadProcess` 现在返回
  `false, nil`，由既有 timeout/context 路径产生 `ErrStateBusy`，不再向调用方泄漏
  裸 `flock` errno。新回归同时断言不会执行 PID 检查或修改原锁 token。
- `R1-F3` carried：CLI recovery 文案属于本改动之外的新用户可见决定，已由
  `ad-bug-state-busy-recovery-guidance` 承载；该 bug 保持 `open`，等待显式
  Bug-lane 选择，未创建 Development Gate，也未在本轮修改错误码或输出。
- Verification：R1-F2 聚焦 RED 得到
  `acquire error = reclaim guard unavailable, want state_busy`；同一测试 GREEN。
  RED/GREEN 日志 SHA-256 分别为
  `6aa00fe9cd1af6584f875c2a48e26d9ab75bb5c97a7da5a4b8f27f599c59e00a`、
  `b468259b5baa27e769e355269e893c894545a6f90ca60883eb5872ca4ea12cc9`。
  完整锁边界、`scripts/run-go-test.sh ./...`、
  `scripts/run-go-test.sh -race ./internal/store`、
  `go vet -mod=vendor ./internal/store`、Darwin arm64/amd64 构建均 PASS；三份
  测试日志 SHA-256 分别为
  `714a661ddd8c9b54a7d623e1bd26bee6a729da5bc6e56935a3cfc344ed4c3077`、
  `6e551410715f11bcbe5cfca9c263d90c41deb95bfb689888f90f896a7894a06a`、
  `0744863784f3ea259e366c0c0292b105ce05e644ec17f204ca49345907ff39e4`。
- Completion gate: VERIFIED — `fix:state-lock-liveness` 的 Repair 最终候选满足
  全部六项 required criteria；Round 1 中的 `NOT_REQUIRED` 是评审当时对 Lane A
  CEv1 契约的判断，不替代本轮按当前项目规则执行的门禁查询。
- Verdict: REOPEN — R1-F1、R1-F2 已关闭，R1-F3 已进入明确 carrier；修复完成，
  等待独立 Re-review。

## Re-review — Round 2 — 2026-09-02

- Reviewed state: HEAD `853f02f4ee83a7dd603b8a436cce835d71534cdb` 加未提交工作区，
  blob:
  `cb2a367a0d29dc4ddb281a2d67567450a4308d87 internal/store/store.go`、
  `a22de56fc0da66ecec0e3b8822c4030240ac50a3 internal/store/store_test.go`、
  `1f8a9fe0a5de7c63c3a2a1c50c6413dbe2f26888 internal/store/process_darwin.go`、
  `023a30e98eae878b7857a176dbba4fff73ddbead internal/store/process_unsupported.go`、
  `dff6066c333e9279864be83fa4e3a8d3b847c51c docs/fixes/state-lock-liveness.md`
  （本轮复评随后把 `R1-F3` 的承载体补记进 Round 1 该条目，记录 blob 因此前移。）
- Reviewer: claude-code（独立于实现方 codex）
- Method: 逐条核对 Round 1 的三条 finding。用候选二进制在隔离 `--state-dir` 上
  实测 `R1-F1` 所声称的发现途径；读代码核对 `R1-F2`；查 Beads 核对 `R1-F3` 的
  承载体；直接查 `neo4j` CEv1 图核对本轮新写的 `Completion gate: VERIFIED`，
  并与 `.agent-instructions/evidence.md` 的边界表对照；全量
  `scripts/run-go-test.sh ./...`、`-race ./internal/store`、`go vet`。
- Findings:
  - `R1-F1` closed：修复边界 新增一条，明确写出自动回收只覆盖本版本及之后写入的
    `v1:` 锁，点名 现象 里 2026-08-30 至 09-01 那次事故属于不被覆盖的一类，并给出
    `agentdeck doctor` 的 `stale_lock` 作为发现途径与「确认无进程后手工删除」的
    补救。实测确认该途径成立：旧格式 token、mtime 一小时前的 `state.lock` 使
    `doctor` 输出 `{"name":"state_lock","status":"warning","code":"stale_lock"}`，
    同一状态目录下 `provider list` 仍返回 `state_busy`。记录还自行补上了我未提出
    的一点——Doctor 不检查 `scan.lock`，旧格式残留 `scan.lock` 没有发现途径；核对
    `internal/doctor/doctor.go:197` 确实只 stat `state.lock`，属实。
  - `R1-F2` closed：新增 `reclaimFileLock` 注入边界，`tryReclaimLock` 返回错误时
    `reclaimLockFromDeadProcess` 改为 `false, nil`，交回既有 timeout/context 路径。
    `TestAcquireNamedLockTreatsReclaimGuardFailureAsBusy` 断言返回 `ErrStateBusy`、
    不泄漏守卫错误、不执行 PID 检查、不改动原 token。仅剩的 `return false, err`
    在 `os.Remove` 失败处，那是状态目录本身不可写，不属于「忙」，保持硬错误合理。
  - `R1-F3` carried：`ad-bug-state-busy-recovery-guidance` 已建立并保持 `open`
    （P2，bug，无 Development Gate），description 记明来源为本记录 Round 1 R1-F3。
    Round 1 该条目原先只写了「建一条 Beads issue 或记入 Backlog」，没有机器可读的
    承载体，PASS 时会被 Stop hook 判为无主；本轮复评已把承载体 ID 补记进该条目。
  - [P2] `R2-F1` new：本轮就地决定了 Lane A 的 CEv1 边界，而该问题仍未闭合。
    `.agent-instructions/evidence.md` 的四边界表规定 `task` 的 `work_unit_id`
    形状是 `<topic>:<task-anchor>`，Lane A 修复没有 topic；本轮新建
    `urn:ce:agent-deck:work-unit:fix-state-lock-liveness`
    （`unit_kind: task`、`work_unit_id: fix:state-lock-liveness`）等于选定了
    「Lane A 用 `fix` 占据 topic 位」这一个答案。`ad-chore-cev1-lane-a-boundary`
    仍是 `open`，其 description 把这件事写成两选一的未决问题；而同日交付的上一份
    Lane A 修复 `attribution-determinability` 记的是 `NOT_REQUIRED`，其 Beads 评论
    写的是「CEv1 remains NOT_REQUIRED under the current Lane A boundary contract」。
    于是同一个契约问题在同一天有了两个相反的答案，`evidence.md` 却一字未改——
    数据先于规范落地，正是本仓库反复付过代价的那种漂移。
    💡 二选一，且是用户的决定不是修复轮次的：要么本轮 gate 退回 `NOT_REQUIRED`
    并把答案留给该 chore；要么先按流程修改 `evidence.md` 定义 Lane A 边界、关闭
    chore，再回填两份 fix 记录使其一致。-> open
  - [P2] `R2-F2` new：`VERIFIED` 绑定的内容状态漏掉了本次改动的两个生产文件。
    该 WorkUnit 节点的 `target_state_recipe` 属性写的是「HEAD plus scoped blob
    fingerprint over internal/store/store.go, internal/store/store_test.go, and
    docs/fixes/state-lock-liveness.md」——三个文件，而本次改动是五个。缺的两个是
    `internal/store/process_darwin.go` 与 `internal/store/process_unsupported.go`，
    前者装着 `lockProcessAlive` 与 `tryLockReclaimFile` 的全部实现，也就是这次修复
    的核心。按这个 recipe，改动存活性判据或 flock 守卫都不会让证据失效，
    `VERIFIED` 因此没有覆盖它声称覆盖的内容。记录也没有写出摘要的拼接方式，
    所以我无法独立复算 `bbd17dc6…` 这个 `target_content_state`。
    💡 recipe 扩到五个文件并按新内容状态重新记录证据；若 `R2-F1` 选择退回
    `NOT_REQUIRED`，本条随之消失。-> open
- Evidence:
  - `env GOCACHE=… scripts/run-go-test.sh ./...`：PASS，20 个包 `ok`，无
    `--- FAIL`；`-race ./internal/store`：PASS；`go vet -mod=vendor
    ./internal/store`：PASS。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 隔离 `--state-dir` 实测（候选二进制）：旧格式 token 且 mtime 一小时前的
    `state.lock` -> `doctor` 报 `stale_lock`，`provider list` 报
    `state_busy: timed out waiting for state lock`。
  - CEv1 直查（`neo4j` read cypher）：`urn:ce:agent-deck:work-unit:
    fix-state-lock-liveness` 存在，6 条 criterion，三个内容状态各 6 条
    `outcome: pass` 的 evidence；节点属性 `unit_kind: task`、
    `work_unit_id: fix:state-lock-liveness`、`target_state_recipe` 覆盖三个文件。
  - Beads：`ad-bug-state-busy-recovery-guidance` 存在且 `open`；
    `ad-chore-cev1-lane-a-boundary` 仍 `open`。
- Completion gate: 本轮不采信 `VERIFIED`。门禁本身可查且六项 criterion 均为
  `pass`，但其 WorkUnit 形状未被 `evidence.md` 定义（`R2-F1`），且绑定的内容状态
  不含两个生产文件（`R2-F2`）。在这两条闭合前，本记录的证据边界按
  `NOT_REQUIRED` 对待，理由与 Round 1 相同。
- Verdict: REOPEN

### 下一步指令

修复：fix / state-lock-liveness / R2-F1 R2-F2

## Repair — Round 2 — 2026-09-02

- `R2-F1` closed for this fix：本轮采用当前权威契约已经给出的答案，而不在一次
  scoped Repair 中替项目作新的 Lane A 设计决定。`.agent-instructions/evidence.md`
  只定义 document、task、topic、release 四种边界，Lane A 不属于 topic，故本记录
  的 Completion gate 恢复为 `NOT_REQUIRED`；是否新增 Lane A unit kind / ID shape
  继续由 `ad-chore-cev1-lane-a-boundary` 决定。
- `R2-F2` closed as non-applicable：既然本 fix 当前没有 CEv1 gate，三文件
  `target_state_recipe` 不再被用来证明本任务完成。误建的
  `fix:state-lock-liveness` WorkUnit 与三个 workspace content state 保持 append-only
  原样，不删除、不改写，也不作为有效门禁；其缺少两个平台生产文件的问题已通过
  comment `01a06265-550e-796e-a9a5-9f7ac673f8b2` 关联到上述 chore。若未来正式定义
  Lane A 边界，新的 recipe 必须覆盖本任务全部五个文件并公开摘要拼接规则。
- Verification：本轮未改生产代码、测试或平台实现；继续复用 Re-review Round 2
  对相同 `cb2a367a` / `a22de56f` code/test blob 的全仓 Go、store race 与 vet
  结果。本轮仅修改此记录，并以 `make check-whitespace` 与 `git diff --check`
  验证最终文档内容。
- Completion gate: NOT_REQUIRED — 当前项目契约未定义 Lane A completion-evidence
  边界；图中已有记录不改变该契约，也不作为本 fix 的 PASS 前提。
- Verdict: REOPEN — R2-F1 与 R2-F2 在本 fix 范围内均已关闭，等待独立
  Re-review。

## Re-review — Round 3 — 2026-09-02

- Reviewed state: HEAD `853f02f4ee83a7dd603b8a436cce835d71534cdb` 加未提交工作区，
  blob:
  `cb2a367a0d29dc4ddb281a2d67567450a4308d87 internal/store/store.go`、
  `a22de56fc0da66ecec0e3b8822c4030240ac50a3 internal/store/store_test.go`、
  `1f8a9fe0a5de7c63c3a2a1c50c6413dbe2f26888 internal/store/process_darwin.go`、
  `023a30e98eae878b7857a176dbba4fff73ddbead internal/store/process_unsupported.go`、
  `c670376163481987e2f9caf015433e353a038c4d docs/fixes/state-lock-liveness.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 核对 `R2-F1`、`R2-F2` 两条处置。四个代码/测试/平台 blob 与 Round 2
  受评时逐字相同，故复用该轮自行执行的全仓 Go、`-race ./internal/store` 与
  `go vet` 结果，只重跑受本轮文档改动影响的检查；直接查 `neo4j` 确认图为
  append-only 未被删改；查 Beads 确认承载体与关联评论确实存在；用
  `beads-consistency.py` 自身的 `ownerless_findings` 复核全部 finding 的承载状态。
- Findings:
  - `R2-F1` closed：Completion gate 恢复为 `NOT_REQUIRED`，理由写成「`evidence.md`
    只定义 document/task/topic/release 四种边界，Lane A 不属于 topic」，并把是否
    新增 Lane A unit kind / ID shape 明确留给 `ad-chore-cev1-lane-a-boundary`。
    核对该 chore：仍为 `open`，且新增一条 codex 评论，点名
    `fix:state-lock-liveness` 这个 WorkUnit 与三个 workspace content state
    写在 `evidence.md` 定义 Lane A 边界之前、不得当作有效 Lane A 门禁，并把长期
    解释留在该 chore。同日两份 Lane A 修复的门禁结论因此重新一致，均为
    `NOT_REQUIRED`。Repair Round 1 里那行 `VERIFIED` 保留未改写，符合本项目
    「被取代的记录仍是不可变证据」的惯例，最新一轮的结论以本记录末尾为准。
  - `R2-F2` closed as non-applicable：本 fix 当前不以 CEv1 作为完成前提，三文件
    `target_state_recipe` 不再被用来证明任务完成；缺两个平台生产文件这一点已随
    同一条 chore 评论转移，并写明未来若正式定义 Lane A 边界，新 recipe 必须覆盖
    本任务全部五个文件并公开摘要拼接规则。误建节点按 append-only 保留，未删除、
    未改写——`neo4j` 复查为 1 个 work_unit、6 条 criterion、18 条 evidence
    （3 个内容状态 × 6），与 Round 2 观测完全一致。
  - Round 1 三条经复核仍闭合：`R1-F1` 的 修复边界 条目与 `R1-F2` 的
    `reclaimFileLock` 实现及其回归所在的四个 blob 本轮一字未动；`R1-F3` 的承载体
    `ad-bug-state-busy-recovery-guidance` 仍为 `open`。
- Evidence:
  - 四个代码/测试/平台 blob 与 Round 2 相同（`cb2a367a`、`a22de56f`、`1f8a9fe0`、
    `023a30e9`），Round 2 自行执行的 `scripts/run-go-test.sh ./...`（20 个包 `ok`）、
    `-race ./internal/store`、`go vet -mod=vendor ./internal/store` 结果在同一内容
    状态上继续成立；本轮唯一改动是本 Markdown 记录。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - CEv1 直查：`state-lock-liveness` 相关节点计数 work_unit 1 / criterion 6 /
    evidence 18，与 Round 2 一致，无删除亦无新写入。
  - Beads：`ad-chore-cev1-lane-a-boundary` `open` 且带上述关联评论；
    `ad-bug-state-busy-recovery-guidance` `open`。
  - `beads-consistency.py` 的 `ownerless_findings`：空列表，全部 finding 均已闭合
    或带承载体。
- Completion gate: NOT_REQUIRED —— 现行 `evidence.md` 未定义 Lane A
  completion-evidence 边界；图中已有节点不改变该契约，也不作为本 fix 的 PASS
  前提。该契约缺口由 `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: PASS

### Task checkpoint

Task checkpoint：`ad-bug-state-lock-liveness`，内容状态 HEAD
`853f02f4ee83a7dd603b8a436cce835d71534cdb` 加上述五个 blob；完成门
`NOT_REQUIRED`。

提交建议：本次评审边界即一次提交的范围——`internal/store/store.go`、
`internal/store/store_test.go`、`internal/store/process_darwin.go`、
`internal/store/process_unsupported.go` 与 `docs/fixes/state-lock-liveness.md`。
工作区里的 `docs/topics/schema-version-signal/` 属于另一个任务，不得一并暂存。
贡献者 trailer 按 `.agent-instructions/beads.md` 的 Commit-checkpoint contributor
attribution 从本任务的 Beads 评论解析（实现与两轮 Repair 为 codex，评审与两轮
Re-review 记录为 claude-code）。

推送建议：目标 `origin/main`，前提是用户明确授权。注意本地 `main` 已有五笔未推送
的提交（`f282002`、`8f75be9`、`75767f1`、`ccf2059`、`853f02f`），一次推送会把它们
一并发布。
