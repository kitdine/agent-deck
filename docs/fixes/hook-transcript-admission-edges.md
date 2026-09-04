---
status: active
created: 2026-09-03
---

# 缺陷：hook transcript 准入的其余边界仍会静默丢 route

## 现象

`claude-startup-route` 修复让 `SessionStart` 在 transcript 尚未写出时也能写 route，
但只对 `source == "startup"` 一种情况放行。Review Round 1 当时提出两个未覆盖边界，
本记录把它们测实。

准入行为用一次可控探针逐格测出（临时测试驱动 `usage hook event claude`，读 `usage_session_routes`
与 `usage_session_observations` 计数，测后删除）：

| source | 父目录存在 / 文件未建 | 父目录也不存在 | 文件已存在 |
| --- | --- | --- | --- |
| `startup` | routes=1 ✓ | **routes=0 ✗** | routes=1 ✓ |
| `clear` | **routes=0 ✗** | routes=0 ✗ | routes=1 ✓ |
| `fork` | **routes=0 ✗** | routes=0 ✗ | routes=1 ✓ |
| `compact` | routes=0 | — | routes=0 / observations=1 |
| `resume` | routes=0 | — | — |

同一 source 在「文件已存在」时都能写出 route，只在「文件未建」时被丢——这正是时序缺陷的
形状，不是 source 本身不被支持。

被拒绝时 `cmd/agentdeck/main.go:2971` 直接 `return nil`，**不留任何痕迹**。因此本机数据库里
`clear` / `fork` 的 route 与 observation 计数都是 0，这个 0 无法区分「从未发生」与「发生了被丢」，
诊断只能靠上面的探针，不能靠历史数据。

新项目那一格另有文件系统佐证：`~/.claude/projects/<project>/` 与该项目第一个 transcript
的 birthtime 相同（实测 `-Users-jobshen-codes-kitdine-ai-tools` 两者同为 `08-25 22:57:03`，
差 0 秒），即项目目录不是先于会话存在的，而是与首个 transcript 一起被创建。

## 根因

`cmd/agentdeck/main.go` 的 `validHookTranscript`，原第 3100-3102 行：

```go
case client == usagehook.ClientClaude && event.Source == "startup" && errors.Is(err, os.ErrNotExist):
    resolvedPath, err = filepath.EvalSymlinks(filepath.Dir(event.TranscriptPath))
```

两处与既有契约不符：

1. **source 集合过窄**。`internal/usagehook/event.go:88` 对 Claude `SessionStart` 接受
   `startup`、`resume`、`clear`、`compact`、`fork` 五种，而例外分支只认 `startup`。
   `clear` 与 `fork` 同样开启一个带新 session id 的会话，撞上同一时序。
2. **只解析直接父目录**。`EvalSymlinks(filepath.Dir(...))` 要求项目目录此刻已存在，而上文
   实测表明新项目的目录与首个 transcript 同时创建，所以新项目的第一个会话必然落空。

承接的契约是 `usage_session_routes` 已确立的那条——被接受的 `SessionStart` 写出一条 route；
错的只是准入判定，不是这条契约。

## 修复边界

**改**：

1. 新增 `claudeSourceStartsTranscript`，把例外分支的 source 集合从 `startup` 扩到
   `startup` / `clear` / `fork`。
2. 新增 `resolveWithinExistingAncestor` 取代 `EvalSymlinks(Dir(path))`：向上走到**最近一个
   存在的祖先**再解析，然后把剩余路径段拼回。组件是否「存在」由 `os.Lstat` 判定而非
   `EvalSymlinks`——两者对 dangling symlink 的回答不同，见 Repair Round 1 的 `R1-F1`。
   存在的组件必须能解析，否则拒绝，因此 containment 检查不被削弱。

**不改**：

- **`resume` 与 `compact` 不进例外分支**。`resume` 指向一份 Claude 已写出的 transcript，
  文件缺失是不匹配而非时序，保持 fail-closed；`compact` 延续当前会话，
  `main.go:2979` 已明确它不带 selection、本就不写 route，与本缺陷无关。上表两格的
  实测结果与这一判断一致。
- **不改 Codex 分支**。例外分支自始只对 Claude 开放，Codex 的 transcript 时序未测，
  沿用现状。
- **不放宽 containment**。解析后的路径仍必须落在 `~/.claude/projects` 之下，回归里用
  「目录不存在 + 路径在 root 之外」这一组合专门锁住它。
- **不回填历史**。被丢弃的 route 没有留痕，无法反推，按 `claude-no-route-quality` 已确立的
  原则，按时间线合成写入等于制造证据。

**已知残余**：例外分支接受的是一条尚不存在的路径，其中未存在的中间目录理论上可能在
判定之后被创建成指向外部的符号链接。这是原 `startup` 分支就有的性质，本次没有引入也没有
消除；`transcript_path` 由 Claude Code 在同一进程内提供，不是不可信输入。

**Lane A 判定依据**：契约已存在——被接受的 `SessionStart` 写出 route，`ParseEvent` 也早已
接受这五种 source。本修复不新增 source、不新增状态、不改输出形状、不改任何用户可见行为，
只让准入判定覆盖 `ParseEvent` 已经接受的输入。

## 验证

L2：本修复改变持久化 route 的写入条件。

**失败优先**：新增 `TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists`
（`cmd/agentdeck/hook_boundary_test.go`），九个子用例。修复前 **5 个失败、4 个通过**：

```text
FAIL  clear with an existing project directory        routes = 0, want 1
FAIL  fork with an existing project directory         routes = 0, want 1
FAIL  startup in a brand-new project                  routes = 0, want 1
FAIL  clear in a brand-new project                    routes = 0, want 1
FAIL  fork in a brand-new project                     routes = 0, want 1
PASS  startup with an existing project directory
PASS  resume without a transcript stays rejected
PASS  startup outside the projects root stays rejected
PASS  clear outside the projects root stays rejected
```

失败的正是两个边界，通过的正是既有行为与安全边界——即这个回归在修复前就能证明它测的是
缺陷本身，而不是把准入整体放宽。修复后九个全部通过。

**命令与结果**：

```text
./scripts/run-go-test.sh ./cmd/agentdeck/ -run 'TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists' -count=1
    → FAIL 5/9（修复前，见上）
./scripts/run-go-test.sh ./cmd/agentdeck/ -run 'TestUsageHookEvent' -count=1   → passed
./scripts/run-go-test.sh ./... -count=1                                        → passed
go vet ./...                                                                    → 无输出
make check-whitespace                                                           → 无输出
git diff --check                                                                → clean
gofmt -l（本次改动的五个文件）                                                   → 无输出
```

**未能覆盖的**：`clear` 与 `fork` 在真实 Claude Code 中是否确实产生新 session id 与新
transcript、其创建延迟是多少，本记录没有实测——触发它们需要中断一个真实交互会话。修复对
这个未知量不敏感：若它们确实新建文件，本修复消除丢弃；若它们复用既有 transcript，
则走的是原本就通过的「文件已存在」分支，行为不变。两种情形下放宽都不会产生新的错误 route，
因为 containment 与 session-id/文件名匹配两道检查都未放松。

## Review — Round 1

- Reviewed state: HEAD `5ff2e80f22dc5d6855faf9dd00ab9f56cce49c8c`；
  `cmd/agentdeck/main.go`、`cmd/agentdeck/hook_boundary_test.go` 的 scoped diff
  SHA-256 `e305ab1d6f2843ef84c4e54efcf9a78441569c5b78538756afe45d91b24f801c`；
  implementation content state
  `urn:ce:agent-deck:state:workspace:4e7695f3fe72f1d07b2a2467cc315dab28890c3b5f442f23b06fe667f8f0bca9`；
  本记录评审前 blob `c655a57540b9593c568ac0780a43ec88d8457b5c`。
- Reviewer: Codex（单 agent、默认模型层级的独立 code/security review）。
- Method: 通读原 `claude-startup-route` Review Round 1 的 finding/carrier 链，使用
  CodeGraph 核对 `runUsageHookEvent` → `validHookTranscript` → route persistence
  与 `ParseEvent` source 合同，审阅两文件 scoped diff；图索引未展开新 helper 尾部与
  九子用例后，使用一次完整 diff 回退。针对 `EvalSymlinks` 的 `ErrNotExist` 分支运行
  一个 `/private/tmp` 隔离 Go 探针，得到决定性 dangling-symlink 反例后删除探针并停止
  全仓验证。
- Scope: `cmd/agentdeck/main.go`、`cmd/agentdeck/hook_boundary_test.go` 与本记录；
  `docs/topics/schema-version-signal/` 及其他工作树内容排除。生产代码、测试、配置与真实
  Claude/session 状态全程只读。

### 📋 评审报告：fix / hook-transcript-admission-edges

📊 总体评分：4/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`cmd/agentdeck/main.go:3145`] R1-F1 `[P1]`：
`resolveWithinExistingAncestor` 把 `EvalSymlinks(parent)` 的所有
`os.ErrNotExist` 都解释成“该目录尚未创建”，但一个**已经存在的 dangling symlink**
也返回同一错误。函数因此会越过该 symlink，把它的词法 basename 当作普通未建目录
拼回，最终 root containment 错误通过。

- 行为风险：在 `~/.claude/projects` 内创建 `-dangling -> <root 外尚不存在目标>`，
  再投递 `-dangling/<session>.jsonl` 的 `startup`、`clear` 或 `fork`。leaf `Lstat`
  返回 `ErrNotExist`，新 resolver 返回 projects root 下的词法路径，
  `filepath.Rel` 得到 `-dangling/<session>.jsonl` 并接受；随后当前 provider route
  被持久化，尽管路径中一个在判定时已经存在的组件实际指向 root 外。目标一旦创建，
  该路径真实解析到 root 外，直接违反本修复“不放宽 containment”与“每个确实存在的
  路径组件仍然被解析”的边界。
- 证据：隔离探针逐字复用了 resolver 与 `filepath.Rel` 判定，输出
  `leaf_is_not_exist=true`、`dangling_component_exists=true mode=Lrwxr-xr-x`、
  `dangling_eval_is_not_exist=true`、`relative=-dangling/edge-session.jsonl`、
  `accepted=true`。既有 `hook_boundary_test.go:199-205` 只覆盖 symlink 指向**已存在**
  的 root 外目录；新增九子用例只覆盖普通不存在目录与词法 root 外路径，均不能发现
  dangling 形状。

💡 有界修复：向上寻找祖先时先对当前候选执行 `os.Lstat`。只有 `Lstat` 本身为
`os.ErrNotExist` 才把该组件加入未创建 remainder；一旦 `Lstat` 成功（包括 symlink），
就必须让 `EvalSymlinks` 成功并继续现有 containment，否则拒绝。增加
“projects root 内 dangling symlink 指向 root 外尚不存在目标，transcript 在其下”
的回归并断言 route 保持 0；保留普通缺失目录的 `startup/clear/fork` 放行，以及
`resume/compact`、Codex 和既有 symlink 行为。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `startup`、`clear`、`fork` 的集合与 `ParseEvent` 已接受的 session-starting source
  边界一致；`resume` 与 `compact` 的排除理由和调用点行为一致。
- 九子用例能在修复前区分五个目标缺口与四个既有/安全行为，并覆盖普通新项目目录和
  词法 root 外路径。
- 新 helper 没有改变 Codex 分支、route payload、SQLite schema 或历史数据。

### 📝 总结

评审对象由上述 HEAD、两文件 scoped diff、implementation content state 与评审前记录
blob 唯一标识。source 集合扩展和普通缺失目录处理符合承接的 R1-F1/R1-F2，但 ancestor
resolver 无法区分缺失目录与已存在的 dangling symlink，构成可复现的 root-containment
绕过。因此本轮 FAIL/REOPEN；决定性 P1 后未重复聚焦或全仓 L2，修复后需在最终内容状态
重新验证。

- Findings:
  - R1-F1 `[P1]` -> open；拒绝已存在但无法解析的 dangling symlink，同时保留普通
    未创建目录的准入。
- Evidence: 原 `claude-startup-route` Review Round 1 finding/carrier；CodeGraph 生产
  调用链与 `ParseEvent` source 合同；两文件 scoped diff；上述隔离 Go 探针；CEv1
  entry gate 对非空 7 条 criteria 返回 `NOT_VERIFIED`。
- Completion gate: `FAILED`
- Verdict: REOPEN

### 下一步指令

修复：fix / hook-transcript-admission-edges / R1-F1

## Repair — Round 1 — 2026-09-04

### `R1-F1` —— 成立，已修复；这是本次修复自己引入的绕过

finding 准确，且我先独立复现了它再动手。原 `resolveWithinExistingAncestor` 用
`EvalSymlinks(parent)` 的 `ErrNotExist` 判定「该组件尚未创建」，但**一个已经存在的
dangling symlink 返回同一个错误**。于是循环越过它，把它的词法 basename 当普通未建
目录拼回，`filepath.Rel` 得到 root 之内的相对路径，containment 通过。

这直接违反本记录「修复边界」里自己写下的两条——「不放宽 containment」与「每个确实
存在的路径组件仍然被解析」。原九子用例发现不了：既有 `hook_boundary_test.go:199-205`
只覆盖 symlink 指向**已存在**的 root 外目录，新增用例只覆盖普通不存在目录与词法
root 外路径，都不是 dangling 形状。

**改**：判定组件是否缺失改用 `os.Lstat`，不再用 `EvalSymlinks`。
`Lstat` 本身报 `ErrNotExist` 的组件才算「尚未创建」并继续向上；一旦 `Lstat` 成功
（symlink 也算成功），就必须 `EvalSymlinks` 成功，否则返回错误由
`validHookTranscript` 拒绝。语义因此变成：**存在的组件必须能解析，否则拒绝**。

**不改**：普通缺失目录的 `startup` / `clear` / `fork` 放行不变；`resume` / `compact`
的排除不变；Codex 分支不变；既有 symlink 指向已存在 root 外目录的拒绝行为不变。

**回归**：`TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists`
新增 4 个子用例，用 `danglingTo` 在 projects root 内建一条指向未创建目标的符号链接，
并在 fixture 里先断言它确为 symlink（`Lstat` 成功）且不可解析（`Stat` 报
`ErrNotExist`），再断言 `routes` 保持 0。三条覆盖目标落在 root 外，第四条覆盖目标
落回 root 内——后者同样拒绝，因为「存在的组件必须能解析」不看目标位置；Claude 从不
创建这种链接，拒绝是安全的读法。

**失败优先**（修复前，13 子用例中 **4 失败 9 通过**）：

```text
FAIL  startup through a dangling symlink to an uncreated target outside the root stays rejected
FAIL  clear   through a dangling symlink to an uncreated target outside the root stays rejected
FAIL  fork    through a dangling symlink to an uncreated target outside the root stays rejected
FAIL  startup through a dangling symlink to an uncreated target inside  the root stays rejected
PASS  其余九条（含新项目放行、resume 与词法 root 外拒绝）
```

失败的四条正是绕过本身，通过的九条说明修复前后既有行为未被牵动。修复后 13 条全部通过。

## Repair — Round 1 verification — 2026-09-04

```text
./scripts/run-go-test.sh ./cmd/agentdeck/ -run 'TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists' -count=1
    → FAIL 4/13（修复前，见上）
./scripts/run-go-test.sh ./cmd/agentdeck/ -run 'TestUsageHookEvent' -count=1   → passed（13/13）
./scripts/run-go-test.sh ./... -count=1                                        → passed
go vet ./...                                                                    → 无输出
make check-whitespace                                                           → 无输出
git diff --check                                                                → clean
gofmt -l cmd/agentdeck/{main.go,hook_boundary_test.go}                          → 无输出
```

评审因决定性 P1 停掉的聚焦与全仓 L2 已在本轮补跑。

**「已知残余」一节需要重读**：原文说「未存在的中间目录理论上可能在判定之后被创建成
指向外部的符号链接」，并称这是原 `startup` 分支就有的性质。前半句仍成立（判定后创建
属 TOCTOU，本修复不解决）；但本轮修掉的是另一件事——**判定当时就已存在**的 dangling
symlink，那不是 TOCTOU 而是判定本身的漏洞，原文把两者混为一谈，因此曾把一个可当场
复现的绕过归类为理论风险。现在两者分开：判定时存在的组件一律必须解析；判定之后才
出现的链接仍不在本修复范围内。

## Re-review — Round 2

- Reviewed state: HEAD `5ff2e80f22dc5d6855faf9dd00ab9f56cce49c8c`；
  `cmd/agentdeck/main.go`、`cmd/agentdeck/hook_boundary_test.go` 的 scoped diff
  SHA-256 `5d0a1036555fec01fa6ab1469bc466e022b53b65a434b2a7fbb82e89daec998a`；
  implementation content state
  `urn:ce:agent-deck:state:workspace:772122e80ecdfe6cc6e240eb3d7eeac78a0e9e131ddeeaec83fc2fd0e3dbaa83`；
  本记录复评前 blob `dae21b46e27ec371f46495ed73e426367c726f69`。
- Reviewer: Codex（单 agent、默认模型层级的独立 finding-by-finding
  code/security Re-review）。
- Method: 逐项复核 R1-F1 的生产修法、4 个新增 dangling-symlink 回归与完整
  Review/Repair 链；运行 13 子用例及完整 `TestUsageHookEvent` 集合。另用一个
  `/private/tmp` 隔离 Go 探针验证“最近存在祖先是普通文件”的相邻形状由入口
  `Lstat` 的 `not a directory` 拒绝而不会进入例外分支，探针随后删除。最终运行
  全仓 vendored Go suite、vet、whitespace、diff 与两文件 gofmt 检查。
- Scope: R1-F1 及其修复触及的 `cmd/agentdeck/main.go`、
  `cmd/agentdeck/hook_boundary_test.go` 与本记录；
  `docs/topics/schema-version-signal/` 及其他工作树内容排除。生产代码、测试、配置与
  真实 Claude/session 状态全程只读。

### 📋 复评报告：fix / hook-transcript-admission-edges

📊 总体评分：10/10

✅ 结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- R1-F1 closed：祖先缺失现在由 `os.Lstat` 判定；一旦组件存在（含 symlink），
  `EvalSymlinks` 必须成功，否则 `validHookTranscript` 拒绝。原 dangling-symlink
  containment 绕过不再成立。
- 4 个新增子用例分别覆盖 `startup` / `clear` / `fork` 指向 root 外未创建目标，
  以及 `startup` 指向 root 内未创建目标；fixture 先证明组件确为 symlink 且不可解析，
  再从完整 CLI Hook 路径断言 route 保持 0。
- 普通缺失目录的三种 source 继续写 route；`resume`、词法 root 外路径、既有 symlink
  指向 root 外目录、`compact` 与 Codex 边界均未回归。
- 相邻的普通文件祖先形状在入口即被 `not a directory` 拒绝，不会被误当成缺失目录。
- 完整 hook-event 聚焦集合与全仓 suite 均通过；vet、whitespace、diff、gofmt clean，
  验证后 fingerprint 未漂移。

### 📝 总结

| Finding | 处置 |
| --- | --- |
| `claude-startup-route` R1-F1 | Closed；`startup` / `clear` / `fork` 的未建 transcript 准入已由本 Task 实现并通过端到端回归 |
| `claude-startup-route` R1-F2 | Closed；新项目中尚未创建的项目目录已由 existing-ancestor 解析覆盖并通过三种 source 的端到端回归 |
| 本记录 R1-F1 | Closed；缺失组件与已存在但不可解析的 symlink 已结构化区分 |
| Regressed finding | 无 |
| 新 finding | 无 |

复评对象由上述 HEAD、两文件 scoped diff、implementation content state 与复评前记录
blob 唯一标识。R1-F1 的完整 route 绕过已由生产逻辑和端到端回归闭合，普通新项目
准入与原安全边界保持。判定之后才创建 symlink 的 TOCTOU 仍是记录明示且未扩大的范围外
残余，不是本轮未关闭 finding。故本轮 PASS。

- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build ./scripts/run-go-test.sh
    ./cmd/agentdeck -run '^TestUsageHookEventAdmitsEverySessionStartingSourceBeforeTranscriptExists$'
    -count=1`：PASS（13/13）。
  - `env GOCACHE=/private/tmp/agent-deck-go-build ./scripts/run-go-test.sh
    ./cmd/agentdeck -run '^TestUsageHookEvent' -count=1`：PASS。
  - `env GOCACHE=/private/tmp/agent-deck-go-build ./scripts/run-go-test.sh ./...
    -count=1`：PASS。
  - `env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`：无输出。
  - `make check-whitespace`、`git diff --check`、
    `gofmt -l cmd/agentdeck/main.go cmd/agentdeck/hook_boundary_test.go`：无输出。
  - 验证后 scoped diff SHA-256 仍为 `5d0a1036…`。
- Completion gate: `VERIFIED`
- Verdict: PASS

Task checkpoint：`fix:hook-transcript-admission-edges` / implementation state
`772122e80ecdfe6cc6e240eb3d7eeac78a0e9e131ddeeaec83fc2fd0e3dbaa83` /
completion gate `VERIFIED`。

提交建议：仅提交 `cmd/agentdeck/main.go`、`cmd/agentdeck/hook_boundary_test.go` 与
`docs/fixes/hook-transcript-admission-edges.md`；排除
`docs/topics/schema-version-signal/` 及其他无关工作。提交前核对 staged files/hunks、
完整英文 Conventional Commit subject/body、贡献者 trailers 与 SSH 签名。

推送建议：目标为当前 `main` → `origin/main`，但 `main` 已 ahead 6；仅在另行获得
推送授权、当前 Task 的签名提交存在、完整 outgoing range 均确认应推送且远端没有需先
整合的提交后执行。

### 下一步指令

提交：fix / hook-transcript-admission-edges
