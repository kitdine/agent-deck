---
status: active
created: 2026-11-23
---

# 缺陷：CI 在 feature push/PR 和 main 推送中重复触发并并发申请过多 macOS arm64 runner 导致分配失败

协调载体：`ad-bug-ci-duplicate-runs-and-macos-concurrency`；版本：v0.7.5；Lane A。
工作区：`agent-deck.fix.ci-duplicate-runs-and-macos-concurrency`，分支 `fix/ci-duplicate-runs-and-macos-concurrency`，基线 HEAD `47958314190de7a8d6a7f2ff73c1db0bb5226abb`。

## 现象

在 PR 78（`a9126c1`）把分类、文档和可移植检查迁到 Ubuntu 并将 Go race 拆为 3 分片后，GitHub Actions 仍然反复出现重复执行和 macOS arm64 runner 分配失败：

1. **同一分支 push 与 PR 并发跑两套完整重型 CI**：每次向带 PR 的 `feature/*` 或 `fix/*` 分支推送时，`CI` 同时触发 `push` 和 `pull_request` 两次完整运行（各申请 5 个 macOS runner：`go_checks` + 3 个 `race` + `desktop`），若改动触及 `cmd/agentdeck/**` 等路径再叠加 `Cask real Homebrew`（`+1 macos-26`），单次 PR 推送同时请求 10–11 个 macOS arm64 runner。实测样本包括：
   - `fix/codex-quota-plan-display`（2026-11-23）：[CI push 45183583046](https://github.com/kitdine/agent-deck/actions/runs/45183583046)、[CI PR 45183596099](https://github.com/kitdine/agent-deck/actions/runs/45183596099)（4 秒后并发启动）、[Cask PR 45183596151](https://github.com/kitdine/agent-deck/actions/runs/45183596151)。
   - `feature/subscription-account-identity`（2026-11-23）：[CI push 45125394971](https://github.com/kitdine/agent-deck/actions/runs/45125394971)、[CI PR 45125400950](https://github.com/kitdine/agent-deck/actions/runs/45125400950)、[Cask PR 45125400968](https://github.com/kitdine/agent-deck/actions/runs/45125400968)。
   - `feature/subscription-account-identity`（2026-11-22）：[CI push 45033379939](https://github.com/kitdine/agent-deck/actions/runs/45033379939)、[CI PR 45033412275](https://github.com/kitdine/agent-deck/actions/runs/45033412275)、[Cask PR 45033412309](https://github.com/kitdine/agent-deck/actions/runs/45033412309)。
2. **`main` 推送并发申请 6 个 macOS runner，直接因拿不到机器失败**：2026-11-23 PR 80 合并到 `main` 后，[CI push 45200002736](https://github.com/kitdine/agent-deck/actions/runs/45200002736)（5 个 macOS runner）与 [Cask push 45200002752](https://github.com/kitdine/agent-deck/actions/runs/45200002752)（1 个 `macos-26`）在同秒启动。`Go race (2)`（排队 15 分钟）与 `desktop`（排队 20 分钟）均未分配到 runner 而被取消，GitHub 注释均为 `The job was not acquired by Runner of type hosted even after multiple attempts` 与 `Due to capacity constraints, jobs targeting macOS arm64 runners may experience longer queue times.`，导致 `verify` 失败。
3. **3 个 race 分片耗时严重失衡**：在 [CI PR 45183596099](https://github.com/kitdine/agent-deck/actions/runs/45183596099) 中，`Go race (0)`（仅 `cmd/agentdeck`）执行 13m46s，`Go race (2)` 执行 12m59s，而 `Go race (1)` 执行 35m26s（在 `45200002736` 中达 39m17s），排队还额外消耗 18–20 分钟。

## 根因

1. **`.github/workflows/ci.yml` 的 `on.push.branches` 为 `["**"]` 且与 `pull_request` 分离并发组**：任何非主线分支在推送并开启或更新 PR 时都会触发两套完整 CI；未开 PR 的分支推送也会拉起全套 macOS 作业。
2. **`.github/workflows/cask-real-brew.yml` 的 `on.push` 未配置 `paths` 且残留已归档分支**：`push.branches` 包含 `main` 与已合并归档的 `feature/installed-state-ownership`，没有路径过滤，导致每次合并到 `main`（无论是否改动安装器、Cask 或 CLI）都额外抢占一台 `macos-26` runner。
3. **单轮 CI 瞬时并发拉起 5 台 macOS runner 且 race 分片按字母奇偶划分失衡**：
   - `classify` 结束后，`go_checks`（~4m）、3 个 `race` 分片与 `desktop`（~12m）同时竞争 macOS arm64 容量；`go_checks` 失败时也已提前申请 3 台 race Mac。
   - `scripts/ci/go_race_packages.py` 将 `cmd/agentdeck` 独立为 shard 0，剩余 30 个包按字母序奇偶拆成 shard 1 与 shard 2，巧合地把全部 5 个最重库包（`internal/backup` 716s、`internal/usage` 479s、`cmd/agentdeck-perf` 333s、`internal/store` 220s、`internal/scanruntime` 119s）全部分入 shard 1（总计 ~2000s），而 shard 2 仅 ~657s。相比“保留失衡 3 分片并加 `max-parallel: 2`”（最坏墙钟 `13m + 39m = 52m` 且仍需申请 3 台 race runner），改为 2 个按实测耗时加权均衡的分片可将单轮 race 申请数从 3 降到 2，同时把临界路径压到 ~29m。

## 修复边界

保持现有全部产品验证与平台覆盖不变（31 个 Darwin Go 包的 `make test`、`make vet`、`make test-race` `-p 1 -timeout=20m`、Ubuntu 文档/可移植门禁、`macos-26` 桌面与分发验证、`cask-real-brew` 路径覆盖及发布工作流均不降级），仅优化触发去重与 macOS 并发调度：

| 文件 | 变更内容 |
| --- | --- |
| `.github/workflows/ci.yml` | 1. `on.push.branches` 收敛为 `main` 和 `release/**`，保留 `pull_request`，消除 feature/fix 分支 push 与 PR 的重复 CI；保留按 `event_name + ref` 隔离的 concurrency group。<br>2. `race` 增加 `needs: [classify, go_checks]` 并在 `if:` 中要求 `needs.go_checks.result == 'success'`（4 分钟内快速失败不白申请 race Mac）；`strategy` 设为 `max-parallel: 2`、`matrix.shard: [0, 1]`。<br>3. `verify` 在保留分类异常时回退运行完整产品检查的前提下，新增 `test "$CLASSIFY_RESULT" = success` 强制要求 `classify` 成功，同时继续对 `documentation`、`portable`、`go_checks`、`race` 与 `RACE_SHARD_0`/`RACE_SHARD_1` 做严格校验；整轮取消时 `verify` 因 `if: ${{ !cancelled() }}` 不执行并落为 `skipped`（整轮结论 `cancelled`，不得误绿），即使脚本收到全 `cancelled` 输入也非零退出。 |
| `scripts/ci/go_race_packages.py` | 将默认分片数调整为 `SHARD_COUNT = 2`，采用基于实测耗时权重的确定性 LPT（Longest Processing Time first）贪心均衡装箱（未知新包默认权重 1，同权重按包名稳定排序，分片内按字母序输出），保留无重复、无空分片与 `cmd/agentdeck` 存在性校验。 |
| `.github/workflows/cask-real-brew.yml` | 移除已归档分支 `feature/installed-state-ownership`，`on.push` 仅监听 `main` 并与 `on.pull_request` 保持完全一致的 `paths` 过滤，完整覆盖工作流实际消费的构建与验收输入：`apps/macos/**`、`cmd/agentdeck/**`、`internal/**`、`go.mod`、`go.sum`、`vendor/**`、`packaging/homebrew/**`、`scripts/*cask*`、`scripts/build-macos-app.sh`、`scripts/check-macos-widget-intents.sh`、`.github/workflows/cask-real-brew.yml`；保留 `workflow_dispatch`。 |
| `docs/documentation-workflow.md` | 同步说明 CI 的 `push` 触发范围为 `main` 与 `release/**`，feature/fix 分支走 `pull_request`，push 与 PR 并发组仍相互独立。 |
| `scripts/ci/docs_tools_test.py`、`scripts/ci/go_race_packages_test.py` | 补齐触发范围、`race` 对 `go_checks` 依赖与 `max-parallel: 2`/2 分片、`race` step output → job output → `verify` 环境变量一对一映射与端到端分片缺失校验、`verify` 单个上游异常（含 `classify` 失败/跳过/取消）与合法 docs-only 跳过及整轮取消校验、`cask-real-brew.yml` 构建脚本/Xcode 工程/Go 构建输入触发与纯文档跳过校验、`go_race_packages_test.py` 的 `unittest.main()` 直接执行与 CLI 覆盖测试。 |

未改动的工作流：`cask-recovery.yml`（仅 `workflow_dispatch`）、`release-preflight.yml`（仅 `workflow_dispatch`）、`release.yml`（仅 `v*` tag 与 `workflow_dispatch`），均无重复自动触发。

优化前后 macOS arm64 runner 占用对比：

| 场景 | 修复前总申请数 | 修复前并发峰值 | 修复后总申请数 | 修复后并发峰值 |
| --- | --- | --- | --- | --- |
| PR 推送（未命中 Cask 路径） | 10（push 5 + PR 5） | 10 | **4**（仅 PR：`go_checks` 1 + `race` 2 + `desktop` 1） | **3**（前 4m 为 2，4–12m 为 3，12m 后为 2） |
| PR 推送（命中 Cask 路径） | 11（push 5 + PR 5 + Cask 1） | 11 | **5**（PR CI 4 + Cask 1） | **3–4** |
| `main` 推送（未命中 Cask 路径） | 6（CI 5 + 无条件 Cask 1） | 6 | **4**（仅 CI） | **3** |
| `main` 推送（命中 Cask 路径） | 6（CI 5 + Cask 1） | 6 | **5**（CI 4 + Cask 1） | **3–4** |

## 验证

候选基线 HEAD：`47958314190de7a8d6a7f2ff73c1db0bb5226abb`。

### 失败先行证据

在修改 `.github/workflows/ci.yml`、`.github/workflows/cask-real-brew.yml` 和 `scripts/ci/go_race_packages.py` 前，先写入新回归测试并运行 `python3 -m unittest discover -s scripts/ci -p '*_test.py' -v`，56 项中新增与更新断言按预期失败（4 FAIL + 1 import ERROR）：

- `test_ci_triggers_push_only_on_integration_branches_and_keeps_pull_request`：FAIL（旧 `ci.yml` 为 `push.branches: ["**"]`）。
- `test_race_depends_on_go_checks_and_caps_parallelism_to_two_shards`：FAIL（旧 `race` 仅 `needs: classify`，无 `max-parallel`，为 3 分片）。
- `test_verify_requires_every_product_job_on_mixed_or_unknown_scope`：FAIL（旧 `verify` 要求 `RACE_SHARD_2`）。
- `test_cask_real_brew_filters_main_push_paths_and_drops_stale_feature_branch`：FAIL（旧 `cask-real-brew.yml` 含过期分支且 `push` 无 `paths`）。
- `go_race_packages_test`：ERROR（旧模块无 `package_weight` 且返回 3 个不均衡分片）。

### 修复后本地验证与候选内容身份

- `make check-ci-docs-tools` 通过 CI 工具与工作流测试 **72/72** 和 Python Hook 测试 **93/93**。六项新增回归覆盖管道退出码、无 Bash 回退、显式解释器、YAML 标签、脚本工作目录与环境、首个 step 的条件求值。
- `python3 scripts/ci/go_race_packages_test.py -v`（直接执行）：**3/3 OK**。
- 同一脚本 `false | cat` 在省略 shell 时由 `bash -e` 执行，退出 0。显式 `shell: bash` 启用 `pipefail` 后退出 1，显式 `sh` 和自定义 `bash {0}` 均退出 0。路径中只有 `sh` 时，省略 shell 仍会在脚本的首个失败命令处退出。见 [ScriptHandler.cs](https://github.com/actions/runner/blob/1b67e2178bc1443c96686b6c9d478c18b7577ba3/src/Runner.Worker/Handlers/ScriptHandler.cs) 和 [ScriptHandlerHelpers.cs](https://github.com/actions/runner/blob/1b67e2178bc1443c96686b6c9d478c18b7577ba3/src/Runner.Worker/Handlers/ScriptHandlerHelpers.cs)。
- `verify` 在空工作区执行 YAML 中的实际脚本。测试覆盖工作目录和 workflow、job、step 三层环境变量的优先级，并验证上游失败时，新 job 的首个 step 仍以 `success()` 开始。给本机 `BASH_ENV` 指定退出 48 的启动文件时，分类的两项、文档检查和 race 分片共四项曾误失败。四项改用同一执行函数后均通过。见 [StepsRunner.cs](https://github.com/actions/runner/blob/1b67e2178bc1443c96686b6c9d478c18b7577ba3/src/Runner.Worker/StepsRunner.cs)。
- 仓库已有的 Ruby 标准库 Psych 将 `runs-on` 解码为标签。复评的五种等价写法全部通过，包括带注释的单元素 flow 序列和双引号 Unicode 转义。空序列、重复标签、额外标签、macOS 标签和外观像列表的字符串均被门禁拒绝。
- 原 R5 的 **25/25** 项变异按预期通过或失败，其中 2 项接受、23 项拒绝。每一拒绝用例均命中了原复评记录中的失败测试。与官方 `@actions/expressions` 0.3.62 单独对照的 **17/17** 项返回相同的求值或解析错误。
- `make check-whitespace`、`git diff --check`：无输出，全部通过。
- Ruby `YAML.load_file` 解析 `.github/workflows/ci.yml` 与 `.github/workflows/cask-real-brew.yml`：全部通过。
- 实测仓库完整 Darwin 包列表（`go list -mod=vendor ./...`，共 31 个包）经 `scripts/ci/go_race_packages.py` 划分：
  - Shard 0：10 个包，估算权重 `1739s`（约 29.0m）
  - Shard 1：21 个包，估算权重 `1739s`（约 29.0m）
  - 两分片覆盖全部 31 个包、无重复无遗漏，预估耗时完全均衡（差值 `0s`）。

候选文件 blob：

| 路径 | Blob |
| --- | --- |
| `.github/workflows/ci.yml` | `7237ea38b326ee79b472caab872910d54b4e4b4d` |
| `.github/workflows/cask-real-brew.yml` | `468af72b69f6c658d33e71da1016e95a028cdbc6` |
| `scripts/ci/go_race_packages.py` | `48c4d9eb77ab87a0e3ab4b0d968304fc7732c160` |
| `scripts/ci/go_race_packages_test.py` | `6d1ab80db4252a09e61c095d1030c8b465d190de` |
| `scripts/ci/docs_tools_test.py` | `5c4f3b48217d4e4c68b95564dbd9568854e53a9b` |
| `docs/documentation-workflow.md` | `4291968a1670de184d606cfa74b608cddd9ec00b` |

独立评审、签名提交、推送、PR 创建及真实托管 Actions（PR 与 `main` 推送的实际排队、分片耗时、缓存命中与门禁输出）交由主控线程推进。

## Review — Round 1

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立评审 Codex `gpt-6.2-astra` medium，作者 Antigravity Gemini 4 Neon high。P1：Cask 实测工作流的 `paths` 漏掉构建输入；`verify` 未要求 `classify` 成功；本记录缺可解析评审段。P2：race 分片输出接线无测试；契约中 push 与 PR 的措辞留待 v0.7.5 合同统一。

## Review — Round 2

评审结论：FAIL

完成门禁：NOT_VERIFIED

工作流改动（`ci.yml` `7237ea38`、`cask-real-brew.yml` `468af72b`）判定正确，此后未再改动；分片接线与 runner 项关闭。测试存在两处假绿：`verify.if` 求值未读取真实 `needs`；Cask PR 分支过滤无测试。

## Review — Round 3

评审结论：FAIL

完成门禁：NOT_VERIFIED

Cask 项关闭。本地表达式求值器仍有三处缺口：字符串比较区分大小写；短路求值跳过未知函数与上下文；`failure()` 不看祖先结论。修复方向改为锁定 `verify` 最终门禁结构并让求值器 fail-closed。

## Review — Round 4

评审结论：FAIL

完成门禁：NOT_VERIFIED

上轮三项关闭。P1：结构锁未覆盖 `continue-on-error`、runner 与额外步骤；P2：跨类型比较未按 Actions 规则转换；P3：数字词法拒绝负数、指数与十六进制。用户批准超出三轮上限继续修复直至通过。

## Review — Round 5

评审结论：FAIL

完成门禁：NOT_VERIFIED

上轮三项关闭。P1：顶层 `defaults.run.shell` 继承可绕过门禁；P2：全角数字被转为数值；P2：`needs.0` 与带符号十六进制被接受；P3：带引号的 `runs-on` 被误拒。复评者以官方 `@actions/expressions` 0.3.62 对照确认。

## Review — Round 6

评审结论：FAIL

完成门禁：NOT_VERIFIED

上轮四项关闭。P2：省略 `shell` 时本地测试启用 `pipefail`，而 runner 使用 `bash -e {0}`，`false | cat` 本地退出 1、线上退出 0；P3：flow 序列与转义写法的 `runs-on` 被误拒。按用户指示下一轮改由 Codex `gpt-6.2-astra` max 修复。

## Review — Round 7

评审结论：PASS

完成门禁：VERIFIED

独立冷复评 Antigravity Gemini 4 Neon high（本轮作者 Codex，与作者不同 harness）。测试 blob `5c4f3b48217d4e4c68b95564dbd9568854e53a9b`，工作流与其他脚本 blob 与上表一致。

- **关闭项。** 省略 `shell` 时按 runner 选择 `bash -e {0}`（无 bash 回退 `sh -e {0}`），显式 `shell: bash` 才启用 `pipefail`；参数模板、工作目录、三层 `env` 与首 step 条件求值与 actions/runner `1b67e217` 的 `ScriptHandler.cs`、`ScriptHandlerHelpers.cs`、`StepsRunner.cs` 逐项一致。`runs-on` 经 Ruby 标准库 Psych 解码，五种合法等价写法通过，空序列、重复或额外标签、group 映射与重复键被拒。本机 `BASH_ENV` 导致的四项误失败已消除。
- **验证。** `make check-ci-docs-tools` 72 + 93 通过，`go_race_packages_test.py` 3/3，`make check-whitespace` 与 `git diff --check` 无输出。官方表达式库 17/17 对照一致；23 项变异与探针符合预期，其中 7 项为复评者新设计，包括 `| cat` 管道绕过、首 step `if: failure()`、job 级 `env` 覆盖与缺失工作目录。Ruby 与 Psych 为 ubuntu-latest 与 macOS 预装，仓库既有发布工作流已依赖，无新增依赖。
- **非阻断 P3。** 三项均 fail-closed，且当前工作流未使用这些语法；按用户 2026-11-23 的延后决定以 C 通道载体跟踪。CI-DUP-R7-F1：内置 shell 列表未含 `pwsh` -> open `ad-bug-ci-local-runner-pwsh-shell`。CI-DUP-R7-F2：`env` 不支持混合文本插值 -> open `ad-bug-ci-local-expression-interpolation`。CI-DUP-R7-F3：空映射与 `env` 上下文大小写语义有局部偏差 -> open `ad-bug-ci-local-yaml-empty-map-env-case`。
- **门禁。** CEv1 WorkUnit `fix:ci-duplicate-runs-and-macos-concurrency` 在该候选上独立评审与本地验证满足。托管 Actions 的实际排队与门禁输出由 PR 检查验证。
