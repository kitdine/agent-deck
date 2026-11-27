---
status: active
created: 2026-11-24
---

# 缺陷：Go 测试在临时目录含空格或位于真实 HOME 下时失败

协调载体：`ad-bug-usage-test-tempdir-with-spaces`；版本：v0.7.5；Lane A。
工作区：`agent-deck.fix.usage-test-tempdir-with-spaces`，分支 `fix/usage-test-tempdir-with-spaces`，基线 HEAD `3f7f78aaf65f0cabf434130ab670f9e9d27100ce`。

## 现象

2026-11-24 CJSI Task 7 的独立评审在路径含空格的隔离目录（`~/Library/Application Support/...`）下运行全量 Go 测试，`internal/usage` 的 `TestWorkflowMetricsCountOnlyWrittenFileRows` 失败：`tool file rows = 1 (1 written), want 3`。主控在纯 `bbb06da`、`fc2183b`、`3b71a07` 上以默认 TMPDIR 运行均通过；把 TMPDIR 换成含空格的目录即复现。随后在含空格 TMPDIR 下跑 `scripts/run-go-test.sh -p 1 ./...`，另有三个包失败：

- `internal/activity`：`TestParserExtractsCodexWorkSignalMetadata` 把路径拆成 `with`、`Application` 等文件名。
- `internal/doctor`：`TestManagedCommandOwner*` 期望 mismatch，实得 `Status:ok`。
- `internal/usagehook`：`TestSetupStatusLineRefusesAnotherInstallationWithoutMutation` 中安装 B 覆盖了 A 的托管路由并创建 prior。该测试是已关闭 P1 `ad-bug-quota-statusline-cross-installation-conflict` 的回归测试。

另外，`internal/shellconfig` 约 25 个测试在 TMPDIR 位于真实 HOME 之下时（无论是否含空格）报 `This AgentDeck is not the installed program and cannot change client configuration in your home directory.`

## 根因

全部是测试夹具或测试隔离问题，生产代码没有缺陷：

1. **usage、activity、doctor、usagehook**：夹具把 `t.TempDir()` 下的绝对路径不加引号拼进 shell 命令或 Hook/statusLine 命令。路径含空格时按 shell 语义被拆成多个词：usage/activity 的读写分流不再成立；doctor 的命令首词不再是 `agentdeck` 可执行文件；usagehook 的 `--state-dir` 额外参数使托管路由识别失败。生产写入方本就对路径加引号（`cmd/agentdeck/quota.go` 对 `executable` 与 `--state-dir` 调用 `shellQuote`），解析方 `internal/usagehook/config.go` 与 `managed.go` 经 `statusLineShellWord` 支持裸词与单引号编码，真实安装在含空格的状态目录下识别正确，P1 未在生产复发。
2. **shellconfig**：安装所有权契约（`docs/archive/topics/installed-state-ownership/architecture.md`，`internal/platform/client_guard.go`）按物理账户主目录拒绝未安装程序写入。测试包沿用操作系统真实账户，TMPDIR 落在真实 HOME 下时保护按设计拒绝写入。

## 修复边界

只改 5 个测试文件，生产代码零改动：

| 文件 | 变更 |
| --- | --- |
| `internal/usage/signals_metrics_test.go` | `cat` 与重定向路径加单引号。 |
| `internal/activity/activity_test.go` | `exec_command` 与 JS `tools.exec_command` 中的路径加引号。 |
| `internal/doctor/managed_command_test.go` | `fixtureProgram(path, forceQuote)` 对含非安全字符的路径加引号；安全路径仍生成裸命令，保留对历史裸命令的覆盖。 |
| `internal/usagehook/cross_installation_test.go` | 按生产方式引用 `--state-dir`；P1 回归固定使用含空格的 `state root`，默认 TMPDIR 下也验证 B 被拒绝、A 路由字节不变、B 无 prior、A 的 prior 仍可用。 |
| `internal/shellconfig/client_guard_test.go` | `TestMain` 把测试进程 HOME 隔离到临时目录并注入独立的未安装账户解析器；`injectAccount` 恢复先前值。专门的守卫用例仍验证拒写与文件内容不变，保护未被削弱。 |

## 验证

### 失败先行证据

- 基线 `bbb06da` 默认 TMPDIR 通过、含空格 TMPDIR 失败（主控复现，材料在 `AgentDeck-Acceptance/CJSI-rebuild/baseline-triage`）。
- 修复前同一含空格 TMPDIR 下，doctor 裸路径的 Claude、Codex 用例失败而带引号的 `both-quoted` 通过；usagehook 正确引用含空格路径的对照用例已能拒绝 B。

### 修复后本地验证

作者（Codex `gpt-6.2-astra` high）在两种 TMPDIR 下运行 `scripts/run-go-test.sh -p 1 ./...`，30 个包全部通过，退出码 0：HOME 之外且含空格的目录；系统默认目录。5 个文件 `gofmt -l` 无输出，`git diff --check` 与 `make vet` 退出 0。一次 HOME 内的诊断运行中三个 macOS LaunchServices 用例在 1 秒期限内超时，两次完整 `-p 1` 运行均通过（既有并行期限用例，与本修复无关）。候选补丁 `git diff --full-index 3f7f78a` 的 SHA-256 为 `c0ebeb6a805d74f966827a8019f3cb474a9f2872ecb6aeb039fd684f7c90ed3f`。

## Review — Round 1

评审结论：PASS

完成门禁：VERIFIED

独立冷评审 Antigravity Gemini 4 Hydrogen low（作者为 Claude 主控与 Codex，评审为不同 harness；档位经 Decisions 评估为 T3、Hydrogen、low）。

- **结论。** 0 个发现。独立核对生产解析端（`managedStatusLineStateDir`、`parseManagedCommand` 经 `statusLineShellWord`，支持 `shellQuote` 的 `'"'"'` 编码）与写入端（`quota.go` 对 `executable` 与 `stateDir` 调用 `shellQuote`），确认失败源于夹具未加引号而非产品缺陷；shellconfig 的隔离不削弱保护，守卫用例仍验证拒写；doctor 夹具保留历史裸命令覆盖。
- **验证。** 补丁 SHA-256 与工作树 `git diff --full-index` 一致。HOME 之外含空格 TMPDIR 下 usage、activity、doctor、usagehook、shellconfig 五包通过；gofmt、`git diff --check` 干净。两项变异（去掉 usage 路径引号、去掉 usagehook `--state-dir` 引号）分别使目标测试失败，退出码 1。
- **既有期限用例。** 评审者在默认 TMPDIR 下以默认并行运行时，`internal/doctor` 的 `TestRegistrationHostLimitUsesUniqueNormalizedIdentities` 在 `launchservices_darwin_test.go:123` 的 1 秒期限内超时；与本修复的 5 个文件无关，是已知的并行期限敏感用例。主控随后以 `-p 1` 重跑 `internal/doctor` 通过，作者的两次全量 `-p 1` 运行亦通过。
- **门禁。** CEv1 WorkUnit `fix:usage-test-tempdir-with-spaces` 在该候选上独立评审与本地验证满足。托管 CI 结果由 PR 检查验证。
