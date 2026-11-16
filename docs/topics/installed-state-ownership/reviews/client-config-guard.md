---
status: active
topic: installed-state-ownership
subject: client-config-guard
---

# client-config-guard Review

## Round 1 — 2026-11-08

### 📋 客户端配置写入边界独立评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-CCG-R1-F1 · P1 · FIXED — 自定义备份目标绕过客户端配置守卫

`Service.UseCredential` 新增的预检只检查 `configPath`，随后仍将独立的 `backupPath` 传给 `WriteRedactedBackup`。后者直接对 `destination` 创建父目录、调用 `os.WriteFile` 并 chmod，完全绕过本轮加入的 `atomicPrivateReplace` 守卫。调用方可让主配置位于允许的临时 HOME，同时把备份目标设为注入账户的 `.codex/config.toml`。未安装身份下，守卫对该目标返回拒绝，两个写入 API 却返回成功并覆盖配置。[预检位置](../../../../internal/provider/service.go#L960) · [备份调用](../../../../internal/provider/service.go#L987) · [实际写入](../../../../internal/provider/config.go#L829)

- 行为风险是未安装进程仍能经公开的包 API 改写账户内客户端配置，违反 Task 3 对全部写入汇合点、备份及无副作用拒绝的要求。默认托管备份由 `managedBackupPath` 计算，但这两个 API 也接受调用方提供的目的地。现有开发交接把 provider 备份统一归入状态目录，遗漏了这个分支。
- 隔离复现覆盖 `WriteRedactedBackup(ClientCodex, source, destination)` 和 `Service.Use(ctx, OfficialProviderName, ClientCodex, source, destination)`。两例的 source、state 均位于账户之外，destination 为临时注入账户的真实配置名。事先执行 `GuardClientWrite(destination)`，确认 `resource=client_config`、`reason=receipt_missing`。随后 writer_error 为 nil，目标内容由 48 字节变为 148 字节。
- 现有 provider 守卫测试将受保护路径作为主配置传入，备份路径均为空，无法覆盖这项独立目的地输入。[测试位置](../../../../internal/provider/client_guard_test.go#L88)
- 💡 修复范围覆盖备份目的地的授权检查及写入路径使用，在目录创建、文件写入、chmod 和 operation 记录之前拒绝未授权目的地。保留状态守卫已授权的托管备份语义，明确处理自定义目的地与托管状态目录的区别。增加直接 API 与 Service 入口的回归，覆盖账户内目标、父目录别名和末级链接，以及允许的临时目标与已安装身份。同步 Task 3 的写入者清单，证明拒绝时文件树和操作记录均未改变。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的方面

- 主配置写入在创建备份和 operation 之前执行预检，`atomicPrivateReplace` 再次检查并使用物理返回路径。拒绝错误保留 `client_config` 资源语义。
- provider 的账户注入及 TestMain 为复现提供了独立临时账户和 HOME，复现仅访问这些夹具。现有测试同时包含临时 HOME 与已安装身份的成功对照。

### 📝 总结

| 字段 | 记录 |
| --- | --- |
| Reviewer | Codex，独立评审角色，续接会话 `01a1b6cb-4632-7ef0-95b3-cb8ad15303dc`；Task 3 实现者为 Claude Code |
| Method | 工作区绑定核验、CodeGraph 调用链、当前源码与候选 diff 检查、23 个 subject 的逐文件 Git blob 核验、隔离 Go overlay 复现 |
| Scope | Task 3 的客户端配置守卫、provider/usagehook/telemetry/shellconfig 写入入口及 CLI 拒绝映射；决定性发现位于 provider 备份目的地 |
| Workspace | `agent-deck.installed-state-ownership`，`feature/installed-state-ownership` |
| 输入 HEAD | `2867c9c32fbd7349aff6cda81f90e2eb8ce81b4d` |
| 输入 ContentState | `installed-state-ownership:client-config-guard:dev-20261108-qDKCHnI7` |
| 输入指纹 | `sha256:a9643a19a193761b7948fdbdf11cf87c2a3e025f190f0f0530f3d421dc06c2ec`，23 个 subject 与原诊断时完全一致 |
| 关键 blob | `internal/provider/config.go=6a127f52e914843ac9c2433848919116f235c1ca`；`internal/provider/service.go=60aa4b4b5cca8ebe26ba4d6379c0c5eb0c498f1f`；`internal/provider/client_guard_test.go=3a961842108e4dedfdad8c1588dcceb2a4b55611` |
| 验证环境 | Go 1.27.3 darwin/amd64，vendor 模式，Darwin 25.6.0，注入的临时账户与 HOME |
| 状态同步 | Task 3 Dev 保持勾选，Review 未勾选；本记录承载修复项，tasks.md 仅同步阶段摘要 |

#### Evidence

诊断在中断前执行完成，续接时读取了保留的完整结果，并核对当前 23 个 subject 与诊断候选一致。命令为 `scripts/run-go-test.sh -overlay <本次生成的 overlay.json> ./internal/provider -run '^TestReviewClientConfigBackupBoundary$'`，Go 1.27.3 与 `GOTOOLCHAIN=local`。runner 返回 exit 0，包耗时 1.110s。该诊断的断言要求漏洞出现，因此两个子测试 PASS 对应产品边界被突破。

以下测试可通过新建临时 overlay 追加到 provider 包，复用同包的隔离辅助函数。复现材料完整保留于本节，临时诊断文件在交接后清理。

```go
package provider

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"

    "github.com/kitdine/agent-deck/internal/platform"
)

func TestReviewClientConfigBackupBoundary(t *testing.T) {
    for _, mode := range []string{"direct-backup-api", "service-custom-backup"} {
        t.Run(mode, func(t *testing.T) {
            account := injectAccount(t, false)
            source := filepath.Join(t.TempDir(), ".codex", "config.toml")
            destination := filepath.Join(account, ".codex", "config.toml")
            before := []byte("# protected configuration\nmodel = 'preserve-me'\n")
            writeGuardFixture(t, source, guardCodexConfig)
            writeGuardFixture(t, destination, string(before))
            _, denied := platform.GuardClientWrite(destination)
            requireClientConfigRefusal(t, denied)

            var writeErr error
            if mode == "direct-backup-api" {
                writeErr = WriteRedactedBackup(ClientCodex, source, destination)
            } else {
                ctx, service, _ := newGuardService(t)
                writeErr = service.Use(ctx, OfficialProviderName, ClientCodex, source, destination)
            }
            after, readErr := os.ReadFile(destination)
            if writeErr != nil || readErr != nil || bytes.Equal(before, after) {
                t.Fatalf("hypothesis not reproduced: writer_error=%v read_error=%v unchanged=%t",
                    writeErr, readErr, bytes.Equal(before, after))
            }
            t.Logf("guard_refused=true resource=client_config reason=receipt_missing writer_error=nil protected_destination_changed=true bytes_before=%d bytes_after=%d",
                len(before), len(after))
        })
    }
}
```

两个子测试均观察到 `guard_refused=true resource=client_config reason=receipt_missing writer_error=nil protected_destination_changed=true bytes_before=48 bytes_after=148`。

入口门禁在补齐原有五项准则的查询字段投影后，结果为 NOT_VERIFIED，四项已有精确状态 PASS，唯一缺项为 independent-review。本轮反例直接否定 writer-boundaries 的全覆盖断言，其原观察作为历史事实保留，新失败证据显式取代该项的通过结论。

既有全量 Go、五包 race、CLI acceptance 和 vet 的执行结果保留。发现成立后结束广泛验证，修复后的覆盖范围由最终改动确定。既有文档验证记录包含一项未改动归档文档的解析限制，具体为 `docs/archive/plans/agentdeck-cli.md` 的缩进列表续行链接。

#### Reviewed state 与证据同步

最终 ContentState 为 `installed-state-ownership:client-config-guard:review-r1-20261108-e93bac63`，指纹为 `sha256:e93bac63f76d626fd360b241e9088d33a28ee284a6c9b66c624fa3a7de394dc9`。沿用已登记的 HEAD 加 23 个排序 subject blob 配方，仅 `tasks.md` 的日期和阶段摘要改变，其 blob 为 `df8bccf685d70933321a3d1b7efd0b9797da4ab5`。本评审记录作为证据载体，位于既有指纹配方之外。

CEv1 使用固定模板登记 11 个节点和 23 条关系。节点精确读回一致，全部关系经 preflight 后写入，端点与类型读回一致。原候选的两例复现记录为 writer-boundaries 的失败证据，并显式 supersede 同一准则的原通过观察。最终候选通过 target-bound roll-up 引用该失败，并记录独立评审 FAIL。其余三项准则以状态摘要变更的 preserves assessment 和 target-bound roll-up 复用有效观察。

标准 `gate-status.cypher` 在最终 ContentState 返回三个 required criteria 有效通过，writer-boundaries 与 independent-review 具有适用失败证据。invalidated 集合只有被新反例取代的原 writer-boundaries 证据，unresolved candidate impacts 为空。

文档检查的默认工作区模式 `python3 scripts/check-docs.py`、`make check-whitespace` 与 `git diff --check` 通过。首次只传 `--base HEAD` 时检查器因缺少 `--head` 参数抛出 TypeError，改用默认工作区模式后完成本次检查。

Completion gate：FAILED

本轮存在一项开放的 P1 发现，Task 3 继续停留在修复边界。复现验证了包 API 的可达路径，CLI 自定义备份参数及并发路径未纳入这项诊断。

### 下一步指令

修复：installed-state-ownership / reviews/client-config-guard.md / ISO-CCG-R1-F1

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership


## Repair — 2026-11-08

修复者：Claude Code（Task 3 实现者）。只处理 ISO-CCG-R1-F1。

| 发现 | 改动 |
| --- | --- |
| ISO-CCG-R1-F1 | `internal/provider/config.go`：公开的 `WriteRedactedBackup` 先对目的地执行 `GuardClientWrite`，在目录创建、写入、chmod 之前拒绝，并写入返回的物理路径；原写入逻辑移入包内未守卫的 `writeRedactedBackup`。`internal/provider/service.go`：`UseCredential` 对调用方指定的 `backupPath` 在 operation 记录之前预检并改用物理路径；托管备份（`managedBackupPath`，位于已过状态守卫的根下，可能是账户内的隔离默认根）仍走未守卫写入，保留原语义。 |

回归测试（`internal/provider/client_guard_test.go`）：
- `TestProviderCustomBackupDestinationRefusedUnderRealAccount`：账户内目标、父目录别名、末级链接三种形状，各覆盖直接 API 与 `Service.Use`，共 6 例。主配置位于允许的临时 HOME。断言为 `client_config` 拒绝、受保护目录逐字节不变且无残留文件；Service 入口还断言零 operation 记录且主配置未改。
- `TestProviderCustomBackupDestinationAllowedOutsideAccountAndInstalled`：未安装时的临时目的地与已安装身份下的账户内目的地，直接 API 与 Service 均成功写入。

验证（L3，修复范围）：`scripts/run-go-test.sh ./...` 通过；`scripts/run-go-test.sh -race ./internal/provider` 通过；`make vet` 通过；新增 8 个测试项均 PASS。CLI 不暴露自定义备份目的地参数，acceptance 用例不变。tasks.md 的 Task 3 写入者表与写入者重新枚举已同步。

## Round 2 — 2026-11-08

### 📋 客户端配置写入边界独立复评

📊 总体评分：8/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### ISO-CCG-R2-F1 · P2 · 新增 — 拒绝路径的「未写备份」断言检查了错误目录，无法保护该要求

[internal/provider/client_guard_test.go:103](../../../../internal/provider/client_guard_test.go#L103) 在主配置被拒绝后读取 `StateRoot/backups`，以证明拒绝发生在备份之前。托管备份实际由 `managedBackupPath` 写到 `StateRoot/client-backups/<client>/<op>.redacted.*`（[service.go:1079](../../../../internal/provider/service.go#L1079)）。`backups` 目录在 provider 包内从不创建，`os.ReadDir` 总是返回不存在错误，这条断言永远不会触发。

- 处置：新增。该测试属于 Task 3 候选，Round 1 未发现。当前产品行为正确，缺陷在于回归保护形同虚设。
- 行为风险：Task 3 provider 行要求拒绝时不写备份、不建操作记录。主配置预检若被移到 `writeRedactedBackup` 之后、`CreateOperation` 之前，被拒绝的切换会把账户内配置的脱敏副本写入状态目录，而操作记录计数仍为 0，账户树也不变，现有测试全部通过。
- 证据：变异 overlay 把 `UseCredential` 中 `GuardClientWrite(configPath)` 预检移到 `writeRedactedBackup` 调用之后，其他代码不变。`TestProviderSwitchRefusesClientConfigUnderRealAccount` 仍 PASS。同一变异下，临时探针在拒绝后读取 `StateRoot/client-backups`，观察到 `entries=1` 并失败。未变异时同一探针得到 `backups`、`client-backups` 均不存在，结果 PASS。
- 💡 修复范围：让拒绝断言检查托管备份的真实位置，例如由 `managedBackupPath` 的目录推导，或断言整个 `StateRoot` 下没有备份文件。同一断言可补到自定义备份拒绝用例的 Service 分支。修复后用上述变异证明断言能够失败。只改测试。

### 🟡 建议改进 — 推荐

#### ISO-CCG-R2-F2 · P3 · 新增 — 注释仍称 `atomicPrivateReplace` 是包内唯一的客户端写入

[internal/provider/config.go:853](../../../../internal/provider/config.go#L853) 写着 `atomicPrivateReplace is the only client-file write in this package`，[client_guard_test.go:107](../../../../internal/provider/client_guard_test.go#L107) 写着 `The package's single write point`。修复后，自定义备份目的地按客户端写入处理。公开的 `WriteRedactedBackup` 自行守卫，`UseCredential` 预检后经 `writeRedactedBackup` 写入。同文件 780 行的新注释也明确称其为 `a client-side write like any other`，两处注释互相矛盾。

- 处置：新增。随本轮修复候选出现，或在修复后失效。按本项目规则，PASS 前必须关闭。
- 行为风险：只涉及注释，没有运行时影响。但它陈述的正是导致 ISO-CCG-R1-F1 的错误假设（守卫只需放在一个汇合点）。后续写入者若信任这条注释，可能新增未守卫的客户端写入路径。tasks.md 的 provider 行已正确写为「四个 `Write*Config` 唯一汇合点」，代码注释没有同步。
- 证据：当前源码 config.go:780–795、853–855 与 service.go:966–975。
- 💡 修复范围：把两处注释改为准确的范围。`atomicPrivateReplace` 是四个 `Write*Config` 的唯一汇合点。调用方指定的备份目的地由 `WriteRedactedBackup`，以及 `UseCredential` 的预检守卫。只改注释，不改行为。

### 🟢 做得好的方面

- **ISO-CCG-R1-F1 closed**：公开的 `WriteRedactedBackup` 在 `MkdirAll`、`WriteFile`、`Chmod` 之前调用 `GuardClientWrite`，并写入返回的物理路径。`UseCredential` 在 `ConfigFingerprint`、备份和 `CreateOperation` 之前预检调用方的 `backupPath`，并改用物理路径。托管备份仍走未守卫的 `writeRedactedBackup`，它位于已过 `GuardStateRoot` 的根下，保留原语义。`RedactedBackupPath` 只被记录进 operations 表，此后没有其他写入或删除使用它。
- Round 1 的复现原样重放（从本记录 Round 1 Evidence 节提取为临时 overlay 测试），两个子测试均报告 `hypothesis not reproduced: writer_error=This AgentDeck is not the installed program and cannot change client configuration in your home directory. read_error=<nil> unchanged=true`。漏洞已不可达。
- 新回归 `TestProviderCustomBackupDestinationRefusedUnderRealAccount` 覆盖账户内目标、父目录别名、末级链接三种形状，直接 API 与 `Service.Use` 各 6 例，断言 `client_config` 拒绝、受保护目录逐字节不变、零操作记录和主配置未改。`TestProviderCustomBackupDestinationAllowedOutsideAccountAndInstalled` 覆盖未安装临时目的地与已安装账户内目的地。变异 overlay 同时移除 `WriteRedactedBackup` 与 `UseCredential` 中的 backupPath 守卫后，6 个拒绝用例全部失败，允许用例仍 PASS，说明这组回归真实保护该边界。
- provider 包内全部 `os.*` 写入点复核：除 `atomicPrivateReplace` 与 `writeRedactedBackup` 外，只有 `project_gate.go` 写 `StateRoot`。没有发现其他客户端写入者。tasks.md 的 provider 写入者行与重新枚举已同步。

### 📝 总结

| 发现 | 处置 |
| --- | --- |
| ISO-CCG-R1-F1 | closed — 修复已落地，Round 1 复现不再成立，回归经变异证明有效 |
| ISO-CCG-R2-F1 | 新增，开放 — 拒绝路径的备份断言检查错误目录，需修复 |
| ISO-CCG-R2-F2 | 新增，开放 — 过时的单一写入点注释，需修复 |

| 字段 | 记录 |
| --- | --- |
| Reviewer | Claude Code，独立复评角色，冷上下文新会话 `22ccdf21-9311-4299-a340-66427c44c06a`；未参与 Task 3 实现与 ISO-CCG-R1-F1 修复会话 |
| Method | 工作区与候选核验，修复 diff 与当前源码审查，provider 写入点枚举，Round 1 复现重放，三组 overlay 变异（移除备份目的地守卫，将主配置预检后移，拒绝后备份目录探针），focused race 测试与 vet |
| Scope | ISO-CCG-R1-F1 的修复范围（`internal/provider/config.go`、`service.go`、`client_guard_test.go`），以及同一 provider 拒绝路径的回归保护。其余三个包与 CLI 映射沿用 Round 1 结论，与 Round 1 候选相比 blob 未变 |
| Workspace | `agent-deck.installed-state-ownership`，`feature/installed-state-ownership` |
| 输入 HEAD | `2867c9c32fbd7349aff6cda81f90e2eb8ce81b4d` |
| 输入候选指纹 | `sha256:886d5c0c5368cce3ca0e4bc84d5e1cd0576a6d2d7c70b6d2f4b4d59d3954022d`（同一 23-subject 配方）。相对 Round 1 只有 4 个 blob 改变：`internal/provider/config.go=72026bccdb3bd07af05838ab6019e624db845b97`，`internal/provider/service.go=8ef9c48aa73cdfd666d1d1623d135e13fb59f021`，`internal/provider/client_guard_test.go=dc81370dace4215d60abc187ea88c4aa3c53cbd9`，`docs/topics/installed-state-ownership/tasks.md=2860cbc0cdacc93bb7f89a8b9b6d41a27654dc59` |
| 验证环境 | Go 1.27.3 darwin/amd64，vendor 模式，Darwin 25.6.0，注入的临时账户与 HOME |
| 状态同步 | Task 3 Dev 保持勾选，Review 未勾选；tasks.md 只更新两处阶段摘要 |

#### Evidence

- `scripts/run-go-test.sh -race -count=1 ./internal/provider ./internal/platform -v` 运行两次均通过（provider 210.8s/228.8s，platform 14.3s/14.7s）。新增 8 个测试项全部 PASS，`TestProviderSwitch*` 两项 PASS。`go vet ./internal/provider ./internal/platform` 通过。
- Round 1 复现：`scripts/run-go-test.sh -overlay <overlay> ./internal/provider -run '^TestReviewClientConfigBackupBoundary$' -count=1`。测试源为本记录 Round 1 Evidence 节的代码块原文。该测试的断言要求漏洞出现，因此 FAIL 表示边界成立：两个子测试均报告 writer_error 为 client_config 拒绝、目标未变。
- 变异 A 移除 `WriteRedactedBackup` 中的 `GuardClientWrite`，并把 `UseCredential` 的 backupPath 守卫替换为空语句。`-run TestProviderCustomBackupDestination` 下 6 个拒绝子测试 FAIL，允许测试 PASS。
- 变异 B 将 `UseCredential` 的 `GuardClientWrite(configPath)` 预检移到 `writeRedactedBackup` 调用之后。`TestProviderSwitchRefusesClientConfigUnderRealAccount` PASS（ISO-CCG-R2-F1 的反例）。在同一变异下，探针 `TestReviewRefusalLeavesNoManagedBackup` 在拒绝后报告 `client-backups: entries=1` 并 FAIL；未变异时两目录均不存在，探针 PASS。探针源码如下，可经 overlay 追加到 provider 包复现：

```go
package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewRefusalLeavesNoManagedBackup(t *testing.T) {
	account := injectAccount(t, false)
	ctx, service, _ := newGuardService(t)
	codex := filepath.Join(account, ".codex", "config.toml")
	writeGuardFixture(t, codex, guardCodexConfig)
	requireClientConfigRefusal(t, service.Use(ctx, OfficialProviderName, ClientCodex, codex, ""))
	for _, dir := range []string{"backups", "client-backups"} {
		entries, err := os.ReadDir(filepath.Join(service.StateRoot, dir))
		t.Logf("%s: entries=%d err=%v", dir, len(entries), err)
		if err == nil && len(entries) != 0 {
			t.Errorf("%s written before refusal", dir)
		}
	}
}
```

- 修复交接中的 `scripts/run-go-test.sh ./...` 与 `make vet` 结果针对同一修复候选，本轮未重跑全量。临时 overlay、变异副本与探针文件位于会话 scratchpad，未进入工作树，交接后可清理。

#### Reviewed state 与证据同步

最终 ContentState 为 `installed-state-ownership:client-config-guard:rereview-r2-20261108-f04642f3`，指纹为 `sha256:f04642f3efebcd0623cf81341b54d79140dbb30c6281680ab7a6d2a079ed3d9b`。它沿用 HEAD 加 23 个排序 subject blob 的配方，相对输入候选只有 `tasks.md` 的两处阶段摘要改变，其 blob 为 `05fa46d0eddf150ec8aa6fc0834624f77ea96244`。本评审记录位于指纹配方之外。

CEv1 新增 3 个节点：上述 ContentState，以及 `…:rereview-r2-20261108-f04642f3:evidence:independent-review`（outcome `fail`）与 `…:evidence:writer-boundaries`（outcome `pass`，覆盖本状态下的产品行为，并注明 ISO-CCG-R2-F1 的测试保护限制）。另新增 4 条 `observed_at`/`satisfies` 关系。写入前已按精确 id 确认无既有节点，关系 preflight 全部为 `ok`，创建数与提交数一致。Round 1 的历史证据未改写，也未被 supersede。写入前的门禁为 NOT_VERIFIED（`missing_target_state`）。写入后 `gate-status.cypher` 返回 FAILED：writer-boundaries 有效通过，independent-review 失败。cli-contract、validation、guard-semantics 未在本状态登记 preserves 评估，仍列为缺项，下一候选需重新评估或重新验证。unresolved candidate impacts 为空。

Completion gate：FAILED

ISO-CCG-R1-F1 已关闭，但本轮有两项开放发现，Task 3 继续停留在修复边界。

### 下一步指令

修复：installed-state-ownership / reviews/client-config-guard.md / ISO-CCG-R2-F1, ISO-CCG-R2-F2

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Repair — 2026-11-08（Round 2 发现）

修复者：Claude Code。只处理 ISO-CCG-R2-F1、ISO-CCG-R2-F2，未改产品行为。

| 发现 | 改动 |
| --- | --- |
| ISO-CCG-R2-F1 | `internal/provider/client_guard_test.go`：新增 `requireNoManagedBackup`，由 `managedBackupPath` 推导 codex 与 claude 的托管备份目录，断言拒绝后目录不存在或为空。它替换主配置拒绝用例中检查 `StateRoot/backups` 的无效断言，并补到自定义备份拒绝用例的 Service 分支。 |
| ISO-CCG-R2-F2 | `internal/provider/config.go` 中 `atomicPrivateReplace` 的注释改为“四个 `Write*Config` 的唯一汇合点”，并说明调用方指定的备份目的地由 `WriteRedactedBackup` 与 `UseCredential` 的预检守卫。测试里的“single write point”注释同步改正。 |

变异证明：按复评的变异 B，把 `UseCredential` 的 `GuardClientWrite(configPath)` 预检移到 `writeRedactedBackup` 之后，`TestProviderSwitchRefusesClientConfigUnderRealAccount` 变为 FAIL。恢复源码后经 `git diff` 与源码核对，预检已回到备份之前。

验证：provider 包测试、`scripts/run-go-test.sh ./...` 与 `make vet`，结果见交接。

## Round 3 — 2026-11-08

### 📋 客户端配置写入边界独立复评

📊 总体评分：9/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

#### ISO-CCG-R3-F1 · P3 · 新增 — `internal/provider/client_guard_test.go` 未经 gofmt 格式化

[internal/provider/client_guard_test.go:108](../../../../internal/provider/client_guard_test.go#L108) 的 `WriteCodexConfig` 单行函数字面量超出 gofmt 的单行函数体规则。`gofmt -l internal/provider internal/platform` 只列出该文件，`gofmt -d` 要求把它展开为三行函数体。该文件是 Task 3 新增的未跟踪文件。

- 处置：新增。不是本轮修复改动的行，前两轮评审未检出。按本项目规则，PASS 前必须关闭。
- 行为风险：没有运行时影响。项目没有 gofmt 门禁，但当前 HEAD 的非 vendor Go 文件中只有 `cmd/agentdeck/usage_stats_viewer_test.go` 一个文件未格式化（既有文件，不属本任务）。新增文件不应扩大这一偏差。后续任何人对该文件执行 gofmt 或保存时自动格式化，都会产生与改动无关的 diff。
- 证据：`gofmt -l $(git diff --name-only -- '*.go'; git ls-files --others --exclude-standard -- '*.go')` 在 Task 3 的全部改动和新增 Go 文件中只报告该文件。
- 💡 修复范围：对该文件执行 `gofmt -w`，不改其他内容。修复后 `gofmt -l` 对 Task 3 改动与新增的 Go 文件为空，provider 包测试仍通过。

### 🟢 做得好的方面

- **ISO-CCG-R2-F1 closed**：新增的 `requireNoManagedBackup` 由 `managedBackupPath` 推导 codex 与 claude 的托管备份目录，断言拒绝后目录不存在或为空。它替换了检查 `StateRoot/backups` 的无效断言，并补到自定义备份拒绝用例的 Service 分支。本轮独立重放变异 B：用 go overlay 把 `UseCredential` 中 `GuardClientWrite(configPath)` 预检移到 `writeRedactedBackup` 之后，工作树不变。`TestProviderSwitchRefusesClientConfigUnderRealAccount` 在 `client_guard_test.go:103` 报告 `managed backup written before refusal: …/state/client-backups/codex [… .redacted.toml]` 并 FAIL。断言现在能捕获该回归。
- **ISO-CCG-R2-F2 closed**：`config.go:853` 的注释改为「四个 `Write*Config` 的唯一汇合点」，并说明调用方指定的备份目的地由 `WriteRedactedBackup` 与 `UseCredential` 预检守卫，与 780 行注释及 tasks.md provider 行一致。测试注释（105–106 行）同步改正。在 `internal/`、`cmd/` 与 topic 文档中检索 `only client-file`、`single write point`、`StateRoot, "backups"`，均无残留。
- 修复只改测试与注释：相对 Round 2 输入候选，只有 `internal/provider/client_guard_test.go` 与 `internal/provider/config.go` 两个 blob 改变，`service.go` blob 不变（`8ef9c48a…`），预检仍位于备份写入与操作记录之前（service.go:962–975、993、1004）。
- ISO-CCG-R1-F1 保持关闭：自定义备份目的地的 6 个拒绝子测试与允许测试在当前候选上全部 PASS。

### 📝 总结

| 发现 | 处置 |
| --- | --- |
| ISO-CCG-R1-F1 | closed（Round 2 已关闭，本轮回归仍 PASS） |
| ISO-CCG-R2-F1 | closed — 断言改查真实托管备份目录，变异 B 下 FAIL |
| ISO-CCG-R2-F2 | closed — 注释范围准确，无残留表述 |
| ISO-CCG-R3-F1 | 新增，开放 — 新增测试文件未经 gofmt |

| 字段 | 记录 |
| --- | --- |
| Reviewer | Claude Code，独立复评角色，会话 `22ccdf21-9311-4299-a340-66427c44c06a`；未参与 Task 3 实现与任何一轮修复 |
| Method | 修复 diff 与当前源码审查，候选 blob 对比，go overlay 变异 B 重放（工作树不变），残留表述检索，focused race 测试，vet，gofmt |
| Scope | ISO-CCG-R2-F1、ISO-CCG-R2-F2 的修复范围（`internal/provider/client_guard_test.go`、`internal/provider/config.go`），以及 R1-F1 回归保持 |
| Workspace | `agent-deck.installed-state-ownership`，`feature/installed-state-ownership` |
| 输入 HEAD | `2867c9c32fbd7349aff6cda81f90e2eb8ce81b4d` |
| 输入候选指纹 | `sha256:4c61cf196754862ab831810e1bc5dbc67fb00499a238c76d634d090bb4ae6c23`（同一 23-subject 配方）。相对 Round 2 最终状态只有 2 个 blob 改变：`internal/provider/client_guard_test.go=82fae8af21792a8c1d176d628dfc4888af14c662`，`internal/provider/config.go=3679a22e3e05ffb2949ea1b88b5f52b22e83dfe7` |
| 验证环境 | Go 1.27.3 darwin/amd64，vendor 模式，Darwin 25.6.0，注入的临时账户与 HOME |
| 状态同步 | Task 3 Dev 保持勾选，Review 未勾选；tasks.md 只更新两处阶段摘要 |

#### Evidence

- `scripts/run-go-test.sh -race ./internal/provider/ ./internal/platform/` 通过（provider 144.1s，platform 14.6s）。`go vet -mod=vendor ./internal/provider/ ./internal/platform/` 通过。
- 变异 B：`scripts/run-go-test.sh -overlay=<overlay> -run 'TestProviderSwitchRefusesClientConfigUnderRealAccount|TestProviderCustomBackupDestination' ./internal/provider/`。overlay 只把 `service.go` 中三行 `GuardClientWrite(configPath)` 预检移到 `writeRedactedBackup` 调用之后。结果：`TestProviderSwitchRefusesClientConfigUnderRealAccount` FAIL（见上），`TestProviderCustomBackupDestination*` 全部 PASS。
- `gofmt -l internal/provider internal/platform` 输出 `internal/provider/client_guard_test.go`（ISO-CCG-R3-F1）。
- 修复交接中的 `scripts/run-go-test.sh ./...` 与 `make vet` 针对同一修复候选，本轮未重跑全量。变异副本与 overlay 位于会话 scratchpad，未进入工作树。

#### Reviewed state 与证据同步

最终 ContentState 为 `installed-state-ownership:client-config-guard:rereview-r3-20261108-8c57ab91`，指纹为 `sha256:8c57ab91d115b62c554f458eb40df3aa1e1ab7ac87cb444ca0d1047bfb575058`。它沿用 HEAD 加 23 个排序 subject blob 的配方，相对输入候选只有 `tasks.md` 的两处阶段摘要改变，其 blob 为 `dde5f7d202adbfde6cc85b9d1b24264aaa466032`。本评审记录位于指纹配方之外。

CEv1 新增上述 ContentState，以及 `…:rereview-r3-20261108-8c57ab91:evidence:independent-review`（outcome `fail`）与 `…:evidence:writer-boundaries`（outcome `pass`）。另新增 4 条 `observed_at`/`satisfies` 关系。写入前已按精确 id 确认无既有节点，创建数与提交数一致（3 个节点、4 条关系）。Round 1、Round 2 的历史证据未改写。写入后 `gate-status.cypher` 返回 FAILED：writer-boundaries 有效通过，independent-review 失败。cli-contract、validation、guard-semantics 未在本状态登记 preserves 评估，仍列为缺项，下一候选需重新评估或重新验证。unresolved candidate impacts 为空。

Completion gate：FAILED

ISO-CCG-R2-F1、ISO-CCG-R2-F2 已关闭，但本轮有一项开放发现，Task 3 继续停留在修复边界。

### 下一步指令

修复：installed-state-ownership / reviews/client-config-guard.md / ISO-CCG-R3-F1

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership

## Repair — 2026-11-08（Round 3 发现）

修复者：Claude Code。只处理 ISO-CCG-R3-F1。

| 发现 | 改动 |
| --- | --- |
| ISO-CCG-R3-F1 | 对 `internal/provider/client_guard_test.go` 执行 `gofmt -w`，只展开单行函数字面量，不改其他内容。 |

验证：对 Task 3 改动与新增的 Go 文件执行 `gofmt -l`，输出为空；`scripts/run-go-test.sh ./internal/provider -count=1` 通过。

## Round 4 — 2026-11-08

### 📋 客户端配置写入边界独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 做得好的方面

- **ISO-CCG-R3-F1 closed**：修复后的 `internal/provider/client_guard_test.go`（blob `f09e3eef…`）只把 `WriteCodexConfig` 的单行函数字面量展开为三行。把这三行折回原样后，`git hash-object` 恰好得到 Round 3 的 blob `82fae8af21792a8c1d176d628dfc4888af14c662`，说明除 gofmt 展开外没有其他改动。对 Task 3 全部改动与新增的 Go 文件执行 `gofmt -l`，输出为空。
- 修复没有触及产品代码：相对 Round 3 最终状态，只有该测试文件和 tasks.md 的 blob 改变。ISO-CCG-R1-F1、ISO-CCG-R2-F1、ISO-CCG-R2-F2 保持关闭，对应的回归测试在本轮全量与 race 运行中通过。
- 评审者在本候选上完整重跑了 Task 3 的 L3 验证，结果见 Evidence。cli-contract、guard-semantics、validation 三项不再依赖开发阶段的观察。

### 📝 总结

| 发现 | 处置 |
| --- | --- |
| ISO-CCG-R1-F1 | closed（Round 2） |
| ISO-CCG-R2-F1 | closed（Round 3） |
| ISO-CCG-R2-F2 | closed（Round 3） |
| ISO-CCG-R3-F1 | closed — 只有 gofmt 展开，`gofmt -l` 为空 |

| 字段 | 记录 |
| --- | --- |
| Reviewer | Claude Code，独立复评角色，会话 `22ccdf21-9311-4299-a340-66427c44c06a`；未参与 Task 3 实现与任何一轮修复 |
| Method | 修复 diff 的逐字节还原核对，gofmt 检查，候选 blob 对比，Task 3 L3 验证完整重跑 |
| Scope | ISO-CCG-R3-F1 的修复范围，以及 Task 3 全部门禁条件在本候选上的重新验证 |
| Workspace | `agent-deck.installed-state-ownership`，`feature/installed-state-ownership` |
| 输入 HEAD | `2867c9c32fbd7349aff6cda81f90e2eb8ce81b4d` |
| 输入候选指纹 | `sha256:1e9b940325c4b4bc6814fa3bd790e8e0667af39752bc95382e00bf3d617907fc`（同一 23-subject 配方）。相对 Round 3 最终状态只有 `internal/provider/client_guard_test.go=f09e3eefa39e36507608566b7996da6c871627a8` 改变 |
| 验证环境 | Go 1.27.3 darwin/amd64，vendor 模式，Darwin 25.6.0；测试使用临时 HOME 和注入的临时账户 |
| 状态同步 | Task 3 Review 勾选；tasks.md 两处阶段摘要更新为 Round 4 PASS、待授权提交 |

#### Evidence

以下命令在输入候选上运行。它与最终状态的唯一差别是 tasks.md 的状态行，所有代码与测试 blob 都相同。

- `scripts/run-go-test.sh ./...` 通过：28 个包 ok，0 FAIL。跳过的测试是既有的、需手动启用的性能与原生注册夹具。
- `scripts/run-go-test.sh -race ./internal/platform ./internal/provider ./internal/usagehook ./internal/telemetry ./internal/shellconfig` 通过（platform 14.4s，provider 167.1s，usagehook 9.2s，telemetry 72.7s，shellconfig 13.3s），无数据竞争报告。
- `GOFLAGS=-tags=agentdeck_acceptance scripts/run-go-test.sh ./cmd/agentdeck ./internal/platform ./internal/installation ./internal/ownershiptest` 通过（cmd/agentdeck 307.5s；ownershiptest 无测试文件）。
  - `TestOwnershipClientConfigRefusedUnderDefaultAndExplicitState` 的 20 个子测试 PASS。
  - `TestOwnershipClientConfigGoldenAndTemporaryHomeAllowed` PASS。
  - 4 个 `TestGuardClientWrite*` PASS。
- `make vet` 通过。
- `gofmt -l` 对 Task 3 改动与新增的 Go 文件输出为空。
- 本记录写入后，`python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 通过。

#### Reviewed state 与证据同步

最终 ContentState 为 `installed-state-ownership:client-config-guard:rereview-r4-20261108-a7d65d96`，指纹为 `sha256:a7d65d96eaf4bedc3e40ad4e479afda17d1a2e62590c5f7851e776ba3a581b57`。它沿用 HEAD 加 23 个排序 subject blob 的配方，相对输入候选只有 `tasks.md` 改变，其 blob 为 `5820802328d1c7ab65a7ec98d8f8c2a74d450a68`。本评审记录位于指纹配方之外。

CEv1 新增 6 个节点：上述 ContentState，以及 independent-review、writer-boundaries、guard-semantics、cli-contract、validation 五条证据，outcome 均为 `pass`。另新增 10 条 `observed_at`/`satisfies` 关系。写入前已按精确 id 确认无既有节点，创建数与提交数一致。历史证据未改写。写入后 `gate-status.cypher` 返回 VERIFIED：五项必需条件都有本状态下有效的 pass 证据，missing criteria 与 unresolved candidate impacts 都为空。

Completion gate：VERIFIED

Task checkpoint：`ad-installed-state-ownership-client-config-guard`，ContentState `installed-state-ownership:client-config-guard:rereview-r4-20261108-a7d65d96`（`sha256:a7d65d96…`），门禁 VERIFIED。

提交建议：只提交 Task 3 范围，即上述 23 个 subject 中相对 HEAD 改变或新增的文件，以及本评审记录。按计划任务拆成一个提交。提交需要另行授权。

推送建议：推送到 `origin/feature/installed-state-ownership`，前提是上述本地提交已获授权并完成。推送需要另行授权。

### 下一步指令

开发：installed-state-ownership / installation-diagnostics

WORKFLOW_WORKSPACE: agent-deck.installed-state-ownership
