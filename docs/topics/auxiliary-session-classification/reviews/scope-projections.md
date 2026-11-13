---
status: active
topic: auxiliary-session-classification
subject: scope-projections
---

# scope-projections 评审记录

## Round 1 — 2026-11-05

### 📋 scope-projections 实现评审

📊 综合评分：7/10

✅ 评审结论：FAIL

- Reviewer：Codex。
- Method：单代理独立代码审查，CodeGraph 定位与当前源码核验，Go overlay 隔离反例；未修改产品、测试或配置。
- Scope：任务 3 的 Selection、范围过滤、来源读取与 publication 变更；发现决定性发布反例后停止扩大验证。
- Authority：[任务 3](../tasks.md#3-scope-projections)、[架构 §6](../architecture.md#6-查询范围跨数据库一致性与发布)。
- Dispatch：`ad-auxiliary-session-classification-scope-projections`；开发交接评论 `28e3df3f-aa5b-5c93-a4a8-e1e3c398ff68`。

### 🔴 严重问题（必须修复）

**ASC-SP-R1-F1 · P1 · OPEN — 已失效 epoch 的迟到成功结果可覆盖恢复后的发布。**

位置：[internal/sessionorigin/publication.go:72](../../../../internal/sessionorigin/publication.go#L72)。
`Publish` 只在 previous 与 candidate 的两个 epoch 均相同时比较 revision；epoch 不同就直接接受候选。
成功分支完全忽略 `observed`。这只能处理首次切换，不能区分新 epoch 与已经失效的旧 epoch。

- 行为风险：请求 A 在旧 epoch 完成读取/校验后延迟返回；恢复或重建后，请求 B 发布新 epoch；
  A 随后到达会把 B 覆盖为恢复前的列表和计数，并返回 `available=true, stale=false`。
  旧身份本应失效，不能因跨 epoch 禁止 revision 大小比较而重新生效。
- 证据：隔离探针先接受新 epoch，然后把旧候选送入相同 `Publish`，显式提供新 epoch 的 observed vector。
  core 和 sessions 两个子例都失败。旧向量为 `(9,99,9,99)`；新向量分别为 `(2,1,9,99)` 与 `(9,99,2,1)`。
  两例均返回旧候选且 available=true、stale=false。epoch 故意从 9 变为 2，避免把大小比较误当修复。
- 现有测试：[publication_test.go](../../../../internal/sessionorigin/publication_test.go) 的
  `TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs` 通过，但只检验旧→新的一次替换，未覆盖新发布后的旧结果迟到。
- 💡 修复建议：在发布边界用当前 epoch 身份及构建请求代次/失效机制识别候选归属；
  失效 epoch 的候选不得替换当前发布，即使构建本身成功。不得比较跨 epoch 的 revision 或假设 epoch 单调递增。
  同时保留首次合法 epoch 切换、同 epoch 迟到拒绝与失败时保留完整旧快照的既有语义。
  增加 core 恢复、sessions 重建、两者切换后的迟到成功结果回归；无可信当前身份时应保守拒绝或 unavailable。
- 处置：OPEN，回交本任务修复；不将缺陷推迟到 desktop 接入任务。

### 🟡 建议改进（推荐）

无独立建议项。本轮停止后尚未审完的其他验收维度不视为已通过，也不据此新建未经核验的发现。

### 🟢 优点

- Selection 在读取后核对双库向量，并限定最多重试两次；分类 token 与覆盖 binding 有显式核验。
- 同 epoch 的旧 revision 与不可比较向量被拒绝；该既有回归在本次探针运行中通过。
- 开发交接的 16 路径候选指纹与评审入口实际内容一致。

### 📝 总结

Reviewed state：HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3`；评审入口 16 路径指纹
`4d01e34ef312c4ad8ffd14eb568ebab06261652851ff5b081dda61722ab86528`。
配方为 SHA-256(`head=<HEAD>` 加换行，再加 LC_ALL=C 路径排序的 `path=<git hash-object>` 行，每行含末尾换行)。
范围为下表 15 个产品/测试文件及入口 tasks.md（blob `7b4b0757542173ff583f325918fb5428853a7dec`）。
评审后的 tasks.md 只追加本轮交接；评审记录不纳入自身指纹，避免递归身份。
最终状态指纹为 `3eaa678f204fdcd18c496f9676a97d7eeb1b0905611b31c17617c3d6afdb9558`；
仅 tasks.md blob 变为 `7398b448b2c459610083e3f852f13bcf59db8669`，其余 15 个产品及测试 blob 保持不变。
由此确认探针反例适用于最终状态；未仅因交接文档变化重跑产品检查。

| 产品或测试路径 | blob |
| --- | --- |
| internal/desktop/desktop.go | `295f8fa5edb85d1bda2addd171221ea9d25666a9` |
| internal/session/scope_snapshot.go | `f0b27e400ea2a86d16a7c26e4d21501acb5d7efa` |
| internal/session/scope_snapshot_test.go | `4ec847bebb60cc4d55d01cbbf79cd4c187be096f` |
| internal/session/session.go | `c416bb7ab6206c1e28a6741ec2f48359d9750c0c` |
| internal/sessionorigin/publication.go | `2cbe34830b01c22a04c4e17ed3914f8fb6f6309a` |
| internal/sessionorigin/publication_test.go | `373537fd83bba761963447e8af7870847f64bdc0` |
| internal/sessionorigin/selection.go | `dc47783f0742a22abb286110cd832a691d436511` |
| internal/sessionorigin/selection_test.go | `f3aa4cd991f05b6bc8fcbba69cfefab3bef9bece` |
| internal/store/session_origin_derive.go | `1cd3ec273928284df81cdcce44eab0c29b3e7f14` |
| internal/usage/session_filter.go | `fe5261b4b94033f2f926a867cf36b41a6197a0a1` |
| internal/usage/session_filter_test.go | `00f6a9c4b2f481ea0465935b1c2da55a05b7514d` |
| internal/usage/signals_batch.go | `538877850b305ff36450495d2c747e37db615c60` |
| internal/usage/signals_cost.go | `818d806e86737027c4a0f937485d17ad7a37630d` |
| internal/usage/signals_metrics.go | `037e088e218e0ed67e14dd878f73497755fd0d4f` |
| internal/usage/signals_report.go | `91d53f9aeb874dbb82d893fd2d3535b0ced48ea4` |

Evidence：使用 Go 1.27.3 darwin/amd64，`GO_TEST_BIN` 与 `GOROOT` 均选定
`/Users/jobshen/.local/share/mise/installs/go/1.27.3` 的对应入口；PATH 的 Go 为 1.27.1，未用于本次测试。
依赖 blob：go.mod `bc41e2c6481bba3582c8e5f208d4339cdc8ee0fd`；
go.sum `b037abba0aebb3609441e94cb6db5035b013cbfe`；vendor/modules.txt `35c60c6060305c34e187087aef6a98f82e9beea0`。

通过 `scripts/run-go-test.sh -overlay <overlay.json> ./internal/sessionorigin -run
'^(TestReviewRejectsRetiredEpochAfterRestore|TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs)$'`
执行一次：既有测试 PASS；新探针的 core/sessions 两个子例 FAIL，包耗时 1.172s，退出码 1。
overlay 仅把下列外部临时测试映射为该包的虚拟 `zz_review_epoch_test.go`，没有写入工作树测试。
完整反例保留如下，不依赖临时文件的留存：

```go
package sessionorigin

import "testing"

func TestReviewRejectsRetiredEpochAfterRestore(t *testing.T) {
    for _, changed := range []string{"core", "sessions"} {
        t.Run(changed, func(t *testing.T) {
            selection := Selection{Scope: ScopeWork}
            oldVector := Vector{CoreEpoch: 9, UsageRevision: 99, SessionEpoch: 9, SessionRevision: 99}
            newVector := oldVector
            if changed == "core" {
                newVector.CoreEpoch, newVector.UsageRevision = 2, 1
            } else {
                newVector.SessionEpoch, newVector.SessionRevision = 2, 1
            }
            old := &Publication{ID: "before-restore", Snapshot: Snapshot{Selection: selection, Vector: oldVector}}
            fresh := &Publication{ID: "after-restore", Snapshot: Snapshot{Selection: selection, Vector: newVector}}
            _, retained := Publish(selection, old, fresh, nil, &newVector)
            if retained != fresh {
                t.Fatal("fresh restored generation was not accepted")
            }
            view, retained := Publish(selection, retained, old, nil, &newVector)
            if retained == old || view.Publication == old {
                t.Fatalf("retired epoch republished: candidate=%+v observed=%+v available=%v stale=%v", oldVector, newVector, view.Available, view.Stale)
            }
        })
    }
}
```

按项目规则，在决定性反例后停止扩大产品验证；本轮未重跑全量、race、vet 或 macOS 检查。
开发者报告的 L3 结果保留在交接中，本轮未把它们登记为独立通过证据。
任务门禁入口为 NOT_VERIFIED（8 项缺证据、无 invalidated/unresolved 项）。最终 ContentState 为
`auxiliary-session-classification:scope-projections:review-20261105-FXI45j-final-state`。
已向 `same-generation-publication` 与 `independent-review` 两项准则分别登记失败证据；
3 个新增节点精确读回一致，4 条关系预检均为 ok，写入数量 4；规范门禁查询返回 FAILED，
两项失败证据均适用于精确目标，invalidated/unresolved 为空。其余六项未因本轮提前停止而获得通过声明。

完成门禁：FAILED

记录校验：`python3 scripts/check-docs.py` 结构检查通过，`make check-whitespace` 与 `git diff --check` 无报错。
该校验只验证记录结构与卫生，不改变产品 FAIL 或证据门禁。Beads 已回交 in_progress、解除评审指派并加 round-1；
交接评论 `e5b7951a-a78b-52b5-9152-d37c591b6d1f`。CLI 提示 workspace gate lock 权限不可用、以 ungated 模式运行；
任务更新结果已读回确认，没有把锁警告当成主写入失败而重复操作。

修复范围：ASC-SP-R1-F1 与其发布时序回归。任务 Review 保持未勾选，不形成提交检查点；未提交、推送或修改全局状态。

## Repair 1 — 2026-11-05

- Implementer：claude-code（非评审者，不自判 PASS）。
- Scope：仅 ASC-SP-R1-F1 及其发布时序回归；只改 `internal/sessionorigin/publication.go` 与 `publication_test.go`。

### ASC-SP-R1-F1 处置：已修复，待复评

根因：`Publish` 的成功分支只按 previous 与 candidate 两者是否同 epoch 决定比较，完全不读 `observed`；
epoch 不同即接受，无法区分“新的存活 epoch”与“已退役 epoch 的迟到结果”。

修复：`observed`（发布边界读取的当前向量）成为存活 epoch 的唯一可信身份。

| 情形 | 修复后行为 |
| --- | --- |
| 成功候选 epoch ≠ observed | 退役身份的迟到结果：不替换；有存活发布则继续展示它（非 stale），否则 unavailable/`generation_unavailable` |
| previous epoch ≠ observed | previous 已退役：不作为比较对象，也不作为失败时的 stale 回退 |
| observed 缺失且候选相对 previous 改变 epoch | 无可信身份，候选不展示，视图 unavailable，保留 previous 待下一次可信判定 |
| 候选与 observed 同 epoch | 首次合法切换直接接受；与 previous 同 epoch 时仍拒绝较旧或不可比较向量 |
| 构建失败 | 仅当 previous 仍属存活 epoch 时保留为 stale，否则 unavailable |

不比较跨 epoch 的 revision，也不假设 epoch 单调递增。

回归：`TestRetiredEpochResultsNeverReplaceTheLivePublication` 覆盖 core 恢复、sessions 重建、两者同时替换，
以及新 epoch 数值更大的情形；每例依次断言首次切换被接受、旧候选迟到不覆盖、无存活发布时旧候选不展示、
失败不回退到退役发布、无 observed 时两个方向的 epoch 变化均不展示。既有同 epoch 表驱动测试改为显式传入存活身份。

评审反例重放：把 Round 1 记录中的 `TestReviewRejectsRetiredEpochAfterRestore` 原文经 `-overlay` 映射为包内虚拟测试
（未写入工作树），与 `TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs` 同跑：两者及 core/sessions 子例全部 PASS；
overlay 临时文件已删除。

变异（各自恢复后重跑通过）：
- 去掉“候选 epoch ≠ observed”拒绝分支：新回归失败。
- 去掉“无 observed 时 epoch 变化”分支：新回归失败。
- 保留退役 previous：新回归与 `TestFailedBuildsKeepTheLastCompleteResultAsStale` 失败。

### 验证

Go 1.27.3 darwin/amd64。
- `go vet ./...`：通过。
- `scripts/run-go-test.sh ./... -count=1`：通过。
- `scripts/run-go-test.sh -race ./internal/sessionorigin/ -count=1`：通过。

其余受影响包（session/usage/store/desktop）本轮 blob 未变，沿用开发阶段 race 结果。

### 候选

HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3` 加 16 个路径，按 Round 1 记录的同一配方计算（不含本评审记录）。
指纹为 `4930bb850ba1e324a811c4e874fd8b279ffadaa3497c271e9a27cf5b0d448e31`。

相对 Round 1 评审入口，变化的 blob：

| 路径 | blob |
| --- | --- |
| internal/sessionorigin/publication.go | `75c6d1b7fedb4c9f7581f7067f45a79323e1c522` |
| internal/sessionorigin/publication_test.go | `4ae1a7c6448382c21a0030f65db5fdb57be65ea7` |
| docs/topics/auxiliary-session-classification/tasks.md | `5580c951f118ae721a0238e6f8195bc2210515db` |

其余 13 个产品/测试 blob 与 Round 1 表一致。Review 保持未勾选；未新增 CEv1 通过证据，未提交或推送。

## Round 2 — 2026-11-05

### 📋 scope-projections Repair 1 复评

📊 综合评分：8/10

✅ 复评结论：FAIL

- Reviewer：Codex。
- Method：单代理独立复评，核验 Repair 1 交接与精确候选、CodeGraph 当前源码及隔离 Go overlay；产品、测试和配置只读。
- Scope：ASC-SP-R1-F1 与其修复影响的成功发布、缺失当前身份、epoch 切换及失败回退分支。
- Dispatch：`ad-auxiliary-session-classification-scope-projections`；Repair 1 交接评论 `f878233b-65f3-5ea4-a32d-7d96041e40b7`。

### 🔴 严重问题（必须修复）

**ASC-SP-R1-F1 · P1 · 仍 OPEN — 缺少可信当前 epoch 时，首次候选或与 previous 同 epoch 的候选仍被标为新鲜可用。**

位置：[internal/sessionorigin/publication.go:95](../../../../internal/sessionorigin/publication.go#L95)。
本轮处置为部分修复，不把上一轮已修好的分支继续描述为失败：有 observed 时，退役候选确实已被拒绝。
剩余问题是 observed=nil 只在 previous 非空且两者 epoch 不同的情况下拒绝；previous=nil 或两者同 epoch 时，
代码仍走到第 101 行无条件接受 candidate。

- 行为风险：旧构建在恢复前已完成，发布边界无法读取当前身份（observed=nil）；
  没有 previous，或 previous 也是相同旧 epoch 时，旧候选仍被展示为 `available=true, stale=false`。
  candidate 与 previous 互相一致，只证明二者属于同一旧身份，不能证明该身份在恢复后仍存活。
  这绕过了 Repair 1 声明的“observed 是唯一可信存活身份”，也未满足 Round 1 的“无可信当前身份时保守拒绝或 unavailable”。
- 证据：`TestReviewMissingPublicationIdentityFailsClosed/no_previous` 与 `/same_previous_epoch` 均失败。
  输入 candidate epoch `(9,9)`、revision `(99,99)`，observed=nil；两个子例均返回
  `available=true stale=false candidate=candidate-before-restore`。反例不假设 epoch 数字单调，也不比较跨 epoch revision。
- 💡 修复建议：在成功候选的所有路径统一处理缺少可信当前身份，不能只检查 candidate/previous 是否跨 epoch。
  缺少 observed 时不得把新候选当作新鲜可用结果；可保留已有完整发布作为待核验状态，或按现有失败降级合同处理，
  无可用旧结果则 unavailable。增加 previous=nil、previous 与 candidate 同 epoch 两例，并保留有 observed 的现有切换回归。
- 处置：仍 OPEN；沿用原 finding，不扩大到其他任务，也不更改既有验收目标。

### 🟡 建议改进（推荐）

无独立建议项。

### 🟢 优点

- 有可信 observed 时，core 恢复、sessions 重建、两者同时切换和新 epoch 数字更大的四个子例全部通过。
- 同 epoch 迟到/不可比较向量拒绝、已有完整结果失败回退两个既有测试通过。
- 本次只改 publication 实现与其测试；其余 13 个产品/测试 blob 与 Round 1 一致。

### 📝 总结

Finding disposition：ASC-SP-R1-F1 部分修复、仍 OPEN；本轮无其他新 finding。
未恢复上一轮广泛检查：在本轮决定性反例后停止扩大产品验证；其他尚缺证据的准则不作通过声明。

Reviewed state：HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3`；入口 16 路径指纹
`4930bb850ba1e324a811c4e874fd8b279ffadaa3497c271e9a27cf5b0d448e31`，与 Repair 1 交接一致。
使用 Round 1 的同一配方和同一 16 路径范围，排除评审记录以避免自引用。
publication.go blob `75c6d1b7fedb4c9f7581f7067f45a79323e1c522`；
publication_test.go blob `4ae1a7c6448382c21a0030f65db5fdb57be65ea7`；入口 tasks.md blob
`5580c951f118ae721a0238e6f8195bc2210515db`。其余产品/测试 blob 与 Round 1 表逐项相同。
交接同步后的最终 16 路径指纹为 `f87fec70cb22b130736c7c8f8cf9d2ba7e75172d109d2e558321e22aade24ac9`；
仅 tasks.md blob 变为 `1f5a5b56be1b49f78418543d013841a1e1ae915a`，15 个产品/测试 blob 均与本轮入口一致。
反例可直接适用于此最终状态；历史 Round 1 失败保留，不改写为 PASS。

Evidence：Go 1.27.3 darwin/amd64；显式 GO_TEST_BIN 与 GOROOT 均使用同一安装。
go.mod、go.sum、vendor/modules.txt blob 与 Round 1 相同。
本轮通过项目 wrapper 一次运行：

```sh
scripts/run-go-test.sh -overlay <overlay.json> ./internal/sessionorigin -run '^(TestReviewMissingPublicationIdentityFailsClosed|TestRetiredEpochResultsNeverReplaceTheLivePublication|TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs|TestFailedBuildsKeepTheLastCompleteResultAsStale)$'
```

结果：3 个仓库既有/修复后测试通过（包含 4 个 epoch 切换子例）；新增隔离探针的两个子例失败，
包耗时 1.221s，退出码 1。overlay 把下列外部临时源码映射为包内虚拟 `zz_review_epoch_identity_test.go`，
未更改工作树测试。完整探针与必要输出保留在本记录，不依赖临时日志：

```go
package sessionorigin

import "testing"

func TestReviewMissingPublicationIdentityFailsClosed(t *testing.T) {
    selection := Selection{Scope: ScopeWork}
    oldVector := Vector{CoreEpoch: 9, UsageRevision: 99, SessionEpoch: 9, SessionRevision: 99}
    old := &Publication{ID: "candidate-before-restore", Snapshot: Snapshot{Selection: selection, Vector: oldVector}}
    previous := &Publication{ID: "previous-before-restore", Snapshot: Snapshot{Selection: selection, Vector: oldVector}}
    for _, tc := range []struct {
        name string
        previous *Publication
    }{{"no_previous", nil}, {"same_previous_epoch", previous}} {
        t.Run(tc.name, func(t *testing.T) {
            // The boundary's current epoch cannot be established, e.g. its
            // read failed during restore. A successful pre-restore build is
            // not a trustworthy observation of the currently live identity.
            view, _ := Publish(selection, tc.previous, old, nil, nil)
            if view.Available && !view.Stale {
                t.Fatalf("unconfirmed epoch published as fresh: available=%v stale=%v candidate=%s", view.Available, view.Stale, view.Publication.ID)
            }
        })
    }
}
```

本轮未重跑全量/race/vet；Repair 1 的开发者验证报告保留，不把它当成本轮独立验收通过。
入口规范门禁返回 NOT_VERIFIED（新 target 尚未登记；旧失败证据不匹配新 target，未误算为当前失败）。

最终 ContentState：`auxiliary-session-classification:scope-projections:rereview-20261105-qzkHo2-final-state`。
已登记本轮 `same-generation-publication` 和 `independent-review` 两条失败证据；3 个新节点精确读回一致，
4 条关系预检为 ok、写入数量 4。规范门禁返回 FAILED，两条本轮证据匹配目标且适用；
历史失败未被重标为新内容，invalidated/unresolved 均为空，其余六项仍无本轮通过证据。

完成门禁：FAILED

记录检查：`make check-whitespace`、`python3 scripts/check-docs.py`、`git diff --check` 均通过。
Beads 回交为 in_progress、未指派、round-2；交接评论 `fe568455-4170-571b-8e24-74e90f198d22`。
CLI 仍提示 workspace gate lock 权限不可用而以 ungated 模式运行，主写入结果已读回确认。

修复范围仍为 ASC-SP-R1-F1；任务 Review 未勾选，不形成提交检查点。未提交、推送或修改全局状态。

## Repair 2 — 2026-11-05

- Implementer：claude-code（非评审者，不自判 PASS）。
- Scope：仅 Round 2 仍 OPEN 的 ASC-SP-R1-F1；只改 `internal/sessionorigin/publication.go` 与 `publication_test.go`。

### ASC-SP-R1-F1 处置：已修复，待复评

根因：Repair 1 只在“observed 缺失且 previous 存在且与候选跨 epoch”时拒绝。
previous 为空，或 previous 与候选同 epoch 时，仍落到无条件接受分支。
候选与 previous 彼此一致只说明二者同属一个身份，不能说明该身份在恢复或重建后仍然存活。

修复：成功分支中 `observed == nil` 统一为一个分支，任何候选都不再以 available=true、stale=false 展示。

| observed 缺失时 | 修复后行为 |
| --- | --- |
| 无 previous | unavailable / `generation_unavailable`，不保留候选 |
| previous 与候选同 epoch（无论新旧 revision） | 展示 previous，stale=true，原因 `generation_unavailable`；候选不替换 |
| previous 与候选跨 epoch | unavailable / `generation_unavailable`，保留 previous 待可信判定（同 Repair 1） |

有 observed 时的首次合法切换、退役候选拒绝、同 epoch 迟到/不可比较拒绝以及失败回退语义均不变。

回归：
- 新增 `TestMissingLiveIdentityNeverPublishesACandidateAsFresh`，覆盖无 previous、同代 previous、较旧 revision 的同 epoch previous，并确认读到 observed 后同一候选正常发布。
- 既有“另一选择”用例改为显式传入存活身份。

评审反例重放：
- 把 Round 1 的 `TestReviewRejectsRetiredEpochAfterRestore` 与 Round 2 的 `TestReviewMissingPublicationIdentityFailsClosed` 原文经 `-overlay` 映射为包内虚拟测试，未写入工作树。
- 与 publication 全部仓库测试同跑，两个反例的 core、sessions、no_previous、same_previous_epoch 子例全部 PASS。
- overlay 临时文件已删除。

变异：把该分支恢复为 Repair 1 的条件（仅跨 epoch 拒绝）后，新回归失败；已恢复并重跑通过。

### 验证

Go 1.27.3 darwin/amd64。
- `go vet ./...`：通过。
- `scripts/run-go-test.sh ./... -count=1`：通过。
- `scripts/run-go-test.sh -race ./internal/sessionorigin/ -count=1`：通过。

其余包 blob 未变，沿用开发阶段的 race 结果。

### 候选

HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3` 加 16 个路径，配方同前（不含本评审记录）。
指纹为 `5af0f9de294f33c06520cfec41ea96e280bb3cfd16466a8161bf8f357362cba3`。

相对 Round 2 入口，变化的 blob：

| 路径 | blob |
| --- | --- |
| internal/sessionorigin/publication.go | `a56037b0399f1f610bf7ace7788eff539c3e7330` |
| internal/sessionorigin/publication_test.go | `3a966e038230b4ffd425b75cbda285779f1eb19a` |
| docs/topics/auxiliary-session-classification/tasks.md | `ddf4201a0b43dd1651506c171425c0ed330182fc` |

其余 13 个产品/测试 blob 不变。Review 保持未勾选；未新增 CEv1 通过证据，未提交或推送。

## Round 3 — 2026-11-05

### 📋 scope-projections Repair 2 复评

📊 综合评分：8/10

✅ 复评结论：FAIL

- Reviewer：Codex。
- Method：单代理独立复评，核验 Repair 2 候选与当前源码；用外部 Go overlay 原文重放前两轮反例，并验证尚未覆盖的依赖时钟准则。产品、测试与配置只读。
- Scope：ASC-SP-R1-F1、受影响 publication 分支，以及本任务尚未通过的 `dependency-clock` 准则。
- Authority：[任务 3](../tasks.md#3-scope-projections) 的最终 SQL 依赖清单与直接 SQL/FTS 输入要求；[架构 §6](../architecture.md#6-查询范围跨数据库一致性与发布) 明确每个读取依赖的 I/U/D（含直接 SQL）须推进 revision。
- Dispatch：`ad-auxiliary-session-classification-scope-projections`；Repair 2 交接评论 `4dd8259a-98ed-5d6f-a0fc-08e9761503d8`。

### 🔴 严重问题（必须修复）

**ASC-SP-R3-F1 · P2 · NEW / OPEN — 新搜索投影读取 FTS，但直接 SQL 修改 FTS 不推进 serving revision。**

位置：[internal/session/scope_snapshot.go:36](../../../../internal/session/scope_snapshot.go#L36)，
实际读取在 [scope_snapshot.go:111](../../../../internal/session/scope_snapshot.go#L111)。
`ReadIndexSnapshot` 依赖 `session_documents`，却只依靠其他表的 trigger 覆盖列举出的 helper 写路径。
这没有覆盖直接对 `session_documents` 的 SQL 写入；本任务未为新读取依赖补齐等效的事务时钟保障。

- 行为风险：搜索结果改变但向量不变，`Reader.Select` 最后的向量核对无法识别该变化。
  同一个 selection 与 publication ID 可以代表不同的文档列表和 selected_session_count，破坏以代次判断缓存/迟到结果的依据。
  这是任务 3 新搜索投影依赖未被时钟覆盖的问题，不是要求修改另一个任务或增加新的验收条件。
- 证据：隔离 fixture 中，先插入可搜索的 `clockneedle` 文档；读取 Work 搜索得到文档数 1、所选会话数 1。
  直接执行 `UPDATE session_documents SET text='replacement' WHERE source_path='/c/c-1.jsonl'` 后再次读取，
  两个数量都变为 0，但前后向量均为 `(4186908161130853031,16,5393721372544960246,14)`，publication ID 也相等。
  此 SQL 成功提交，并非回滚或未生效。epoch 数字来自隔离 fixture，不需要复现相同随机数，只需断言前后相等。
- 现有测试局限：`TestEverySelectionDependencyHasARevisionTrigger` 通过，但其 sessions 表清单只有
  session_metadata/session_sources/session_exclusions；FTS 测试只验证已列举 helper 的写路径，不能证明直接 SQL 被覆盖。
- 💡 修复建议：把搜索实际读取的 FTS 输入写入纳入同事务的 serving revision 保障，包含直接 SQL I/U/D，
  并验证回滚不推进可见版本。若需要 schema 变化，使用批准的 append-only migration；不能只在现有 helper 中手工推进版本，
  或移除直接 SQL 验收来掩盖缺口。补充内容变化→向量变化→publication 身份变化，以及交错写入重试的回归。
- 处置：OPEN，回交本任务修复。

### 🟡 建议改进（推荐）

无独立建议项。

### 🟢 优点

- **ASC-SP-R1-F1 CLOSED**：缺少 observed 的成功候选统一降级；无 previous 时 unavailable，同 epoch 的完整 previous 仅 stale 保留。
- Round 1 原文探针 core/sessions，以及 Round 2 原文探针 no_previous/same_previous_epoch 全部通过。
- 五个 publication 仓库测试通过，包括四种 epoch 切换子例、缺失当前身份、同 epoch 迟到拒绝和失败回退。

### 📝 总结

Finding disposition：ASC-SP-R1-F1 CLOSED；ASC-SP-R3-F1 NEW / OPEN。本轮 FAIL 来自新的依赖时钟反例，
不撤销前述修复的关闭结论。决定性反例后停止扩大产品验证；其余未覆盖准则不作通过声明。

Reviewed state：HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3`；入口 16 路径指纹
`5af0f9de294f33c06520cfec41ea96e280bb3cfd16466a8161bf8f357362cba3`，与 Repair 2 交接一致。
配方与范围沿用 Round 1（15 个产品/测试路径加 tasks.md，排除评审记录自身）。
publication.go blob `a56037b0399f1f610bf7ace7788eff539c3e7330`；publication_test.go blob
`3a966e038230b4ffd425b75cbda285779f1eb19a`；入口 tasks.md blob `ddf4201a0b43dd1651506c171425c0ed330182fc`。
其余 13 个产品/测试 blob 沿用 Round 1 表且当前候选吻合。
最终指纹为 `0b297172a7014c9e331e0b45bc871ffe1fe4a740f8e909e927b9c1f06b742c10`；
仅 tasks.md blob 变为 `383b0ea3bd23c62195c0663df7ce5b3338a7f14b`，15 个产品/测试 blob 与本轮入口一致，
故本轮观察适用于最终状态，不因交接文本变化重复产品测试。

Evidence：Go 1.27.3 darwin/amd64，显式 GO_TEST_BIN/GOROOT；go.mod、go.sum、vendor/modules.txt blob 与 Round 1 相同。
以项目 wrapper 一次执行以下选择（前两轮探针原文和本轮 FTS 探针通过 overlay 注入；没有改写工作树测试）：

```sh
scripts/run-go-test.sh -overlay <overlay.json> ./internal/sessionorigin -run '^(TestReview.*|TestPublicationIdentityIsTheSelectionAndItsGeneration|TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs|TestFailedBuildsKeepTheLastCompleteResultAsStale|TestRetiredEpochResultsNeverReplaceTheLivePublication|TestMissingLiveIdentityNeverPublishesACandidateAsFresh|TestEverySelectionDependencyHasARevisionTrigger)$'
```

结果：8 个顶层测试 PASS（5 个 publication、1 个既有依赖 trigger 检查、2 个旧探针），
`TestReviewDirectFTSWriteChangesSelectionGeneration` FAIL；包耗时 1.399s，退出码 1。
失败事实为 `docs 1->0, selected sessions 1->0, vector unchanged, publication IDs equal=true`。
FTS 探针的完整源码如下，复用包内既有隔离 `mixedFixture`；原件不依赖临时文件留存：

```go
package sessionorigin

import (
    "testing"
    "time"
)

func TestReviewDirectFTSWriteChangesSelectionGeneration(t *testing.T) {
    s := mixedFixture(t)
    if _, err := s.sessions.DB.Exec(`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES('/c/c-1.jsonl','claude','c-1','2026-10-29T08:00:00Z','user','clockneedle')`); err != nil {
        t.Fatal(err)
    }
    selection := Selection{Scope: ScopeWork, Query: "clockneedle"}
    before := s.selectScope(t, selection)
    if _, err := s.sessions.DB.Exec(`UPDATE session_documents SET text='replacement' WHERE source_path='/c/c-1.jsonl'`); err != nil {
        t.Fatal(err)
    }
    after := s.selectScope(t, selection)
    if len(before.Documents) != 1 || len(after.Documents) != 0 || before.SelectedSessionCount != 1 || after.SelectedSessionCount != 0 {
        t.Fatalf("fixture did not change search result: docs=%d/%d sessions=%d/%d", len(before.Documents), len(after.Documents), before.SelectedSessionCount, after.SelectedSessionCount)
    }
    if before.Vector == after.Vector {
        beforeID := NewPublication(before, time.Unix(1, 0)).ID
        afterID := NewPublication(after, time.Unix(2, 0)).ID
        t.Fatalf("direct FTS UPDATE changed docs 1->0 and selected sessions 1->0 without advancing vector %+v; publication IDs equal=%v", before.Vector, beforeID == afterID)
    }
}
```

本轮未重跑全量、race、vet；开发者验证报告保留，不升级为本轮独立通过证据。
入口规范门禁为 NOT_VERIFIED（新 target 尚未登记；历史失败均不匹配新 target）。

最终 ContentState 为 `auxiliary-session-classification:scope-projections:rereview-20261105-llviKE-final-state`。
本轮 `dependency-clock` 与 `independent-review` 两条失败证据已登记；3 个新节点精确读回一致，
4 条关系预检 ok、写入数量 4。规范门禁返回 FAILED，两条本轮失败证据适用；
invalidated/unresolved 均为空，历史失败保持其原状态，其他准则尚未获本轮通过声明。

完成门禁：FAILED

记录检查：`make check-whitespace`、`python3 scripts/check-docs.py`、`git diff --check` 均通过。
Beads 已回交 in_progress、未指派、round-3；交接评论 `f973da6f-b4fd-5f7b-aee4-2c607e16301e`。
CLI 的 workspace gate lock 权限警告仍存在，主写入以 ungated 模式成功，返回状态已核实。

修复范围仅 ASC-SP-R3-F1 及其时钟/一致性回归；ASC-SP-R1-F1 已关闭。任务 Review 未勾选，无提交检查点，未提交或推送。

## Repair 3 — 2026-11-05

- Implementer：claude-code（非评审者，不自判 PASS）。
- Scope：仅 ASC-SP-R3-F1 及其时钟/一致性回归（ASC-SP-R1-F1 已在 Round 3 关闭，未再改动 publication）。

### ASC-SP-R3-F1 处置：已修复，待复评

根因：搜索投影读取 FTS5 表 `session_documents`，但该表不在 sessions 侧 serving revision trigger 清单内。
FTS5 虚表本身不能建 trigger；开发阶段只论证了 helper 写路径会顺带写 sources/metadata，直接 SQL 写入不被覆盖。

修复：FTS5 的每次 INSERT/UPDATE/DELETE 都由模块在同一事务内写其内容影子表 `session_documents_content`（普通表）。
在该表上安装与其余 sessions 依赖同族的 `serving_revision_session_documents_content_{INSERT,UPDATE,DELETE}` trigger。

- 实现位置：`internal/store/serving_generation.go`，`ensureSessionServingStorage` 的表清单加入该表。
- 安装方式：该函数是 sessions 侧 serving trigger 的既有安装点，每次可写 `OpenSessions` 在事务内以 `CREATE TRIGGER IF NOT EXISTS` 幂等安装，并排在 FTS 表创建之后。
- 不涉及表结构变化：没有新增列或表，也没有手工在 helper 中推进版本。
- 已有索引在升级后的首次可写打开即获得 trigger。
- 先用隔离探针确认：所用 modernc SQLite 允许在影子表上建 trigger；直接 SQL 的 INSERT、UPDATE、DELETE 都会推进 revision，回滚不推进。

回归：

| 测试 | 断言 |
| --- | --- |
| `TestEverySelectionDependencyHasARevisionTrigger` | sessions 审计清单加入 `session_documents_content`；直接 SQL 增加 `session_documents` 的 INSERT/UPDATE/DELETE 三步，均须推进 revision |
| `TestSearchTextChangesMoveTheSelectionGeneration`（新） | 回滚的文本修改不动向量与结果；提交的直接 UPDATE 使结果 1→0、SessionRevision 增大、publication ID 改变；在读取与核验之间插入文本写入触发一次重试，接受的候选含新文本 |
| `TestExistingIndexGainsTheSearchTextTriggers`（新） | 删去 trigger 的旧索引在下次可写打开后重新具备三条 trigger |

评审探针重放：
- Round 3 的 `TestReviewDirectFTSWriteChangesSelectionGeneration`，以及 Round 1、Round 2 的两份探针，原文经 `-overlay` 注入（未写入工作树）。
- 三者与上表回归同跑，全部 PASS；overlay 临时文件已删除。

变异：从清单中去掉 `session_documents_content` 后，上表三个测试均失败。session 包既有 helper 写路径测试仍通过，这是预期结果，因为 helper 路径同时由 sources/metadata 覆盖。已恢复并重跑通过。

写放大（记录，交任务 9 按集成预算测量）：每条文档行的写入额外执行一次 sessions revision 更新。扫描按批插入时，额外更新次数随行数增加。

### 验证

Go 1.27.3 darwin/amd64。
- `go vet ./...`：通过。
- `scripts/run-go-test.sh ./... -count=1`：通过。
- `scripts/run-go-test.sh -race ./internal/store/ ./internal/session/ ./internal/sessionorigin/ ./internal/desktop/ -count=1`：通过。

usage 包本轮 blob 未变，沿用开发阶段的 race 结果。

### 候选

HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3` 加 17 个路径；新增 `internal/store/serving_generation.go`，配方同前，不含本评审记录。
指纹为 `36849b1a407eb415e956c65885edd0bc399409850f6400941238db60f45f0aa2`。

相对 Round 3 入口变化或新增的 blob：

| 路径 | blob |
| --- | --- |
| internal/store/serving_generation.go（新纳入） | `36d33608d881d7c7341523dc98a145b787b5e662` |
| internal/session/scope_snapshot.go（仅依赖注释） | `2ea4e4a6add4f472f1eaa45442888e196523722f` |
| internal/sessionorigin/selection_test.go | `e82cd81e1f266a6c4bc81b703836dfd5167e7abc` |
| docs/topics/auxiliary-session-classification/tasks.md | `3511cecdcecaa7f0bd9c6137e899d77b31069b64` |

其余 13 个产品/测试 blob 不变。Review 保持未勾选；未新增 CEv1 通过证据，未提交或推送。

## Round 4 — 2026-11-05

### 📋 scope-projections Repair 3 复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：Codex。
- Method：单代理独立复评，当前源码与调用链核验、三轮原文反例重放、任务级定向回归；复用精确候选已有 L3 验证记录。产品、测试及配置只读。
- Scope：ASC-SP-R3-F1 的 FTS 事务时钟修复、ASC-SP-R1-F1 的回归及任务 3 已声明的八项准则。
- Dispatch：`ad-auxiliary-session-classification-scope-projections`；Repair 3 交接评论 `df3246aa-a76c-5e66-ae24-034e5bfa7e40`。

### 🔴 严重问题（必须修复）

无。ASC-SP-R3-F1 CLOSED；ASC-SP-R1-F1 保持 CLOSED。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- **ASC-SP-R3-F1 CLOSED**：`ensureSessionServingStorage` 在 FTS 创建后、同一事务安装普通内容影子表的 I/U/D 版本触发器。
  直接 SQL 的三个操作均推进 revision；回滚不推进；提交的文本变化改变搜索结果、向量及 publication ID。
  读取与核验间交错写入会重试，接受的候选包含新文本；旧索引下次可写打开补齐三条 trigger。
- **ASC-SP-R1-F1 保持 CLOSED**：前两轮原文探针与全部 publication 仓库测试通过；缺少当前身份不再把候选当新鲜结果，
  有 observed 时保留合法 epoch 切换和同 epoch 迟到拒绝语义。
- 范围划分、token 过渡、覆盖 binding、独立 signals oracle、局部分区失效与只读切换均通过本轮定向回归。

### 📝 总结

Finding disposition：ASC-SP-R1-F1 CLOSED（Round 3 关闭、本轮未回归）；ASC-SP-R3-F1 CLOSED；无新增 finding。
Review PASS 与证据门禁、授权提交分别成立，不将本轮结论延伸为 Topic 或原生界面完成。

Reviewed state：HEAD `ac5f21cdf81c01e58ec20f4a80443f565e1be3c3`，入口 17 路径指纹
`36849b1a407eb415e956c65885edd0bc399409850f6400941238db60f45f0aa2`，与 Repair 3 交接一致。
范围在前几轮 16 路径基础上增加 `internal/store/serving_generation.go`；配方仍为 SHA-256 的
`head=<HEAD>` 与 LC_ALL=C 排序 `path=<blob>` 行，每行末尾 LF，评审记录自身不参与指纹。
最终 17 路径指纹为 `b4c312ffee287b36faa49953e9d858118b3d34788f6cbace5b3ab25b1a7d066f`；
仅 tasks.md 的状态/交接变化，blob 为 `c66b8ed760dbfb93c5faa0578daef203cecd63df`。
16 个产品/测试 blob 与入口完全一致；Repair 3 表列出的三个产品/测试变化及此前历史表共同给出其逐文件身份。

Evidence（本轮执行）：Go 1.27.3 darwin/amd64；显式 GO_TEST_BIN/GOROOT；go.mod、go.sum、vendor/modules.txt
分别仍为 `bc41e2c6481bba3582c8e5f208d4339cdc8ee0fd`、`b037abba0aebb3609441e94cb6db5035b013cbfe`、
`35c60c6060305c34e187087aef6a98f82e9beea0`。三个外部 overlay 文件分别来自 Round 1–3 的完整原文探针；
没有修改工作树测试。执行命令：

```sh
scripts/run-go-test.sh -overlay <overlay.json> ./internal/sessionorigin ./internal/session ./internal/usage -run '^Test(Review|ScopesPartition|TokenMismatch|OverridesApply|UsageOnlyOrigins|TimeWindows|SearchSeparates|InterleavedWrites|RolledBackWrites|EverySelection|SelectingWrites|IncrementalSelection|UnreadableStores|Reclassification|OriginWrite|SearchTextChanges|ExistingIndex|Publication|Publish|FailedBuilds|RetiredEpoch|MissingLive|EveryDocument|IndexSnapshot|ProjectGroup|SessionFilter|FilteredBatches)'
```

31 个顶层测试 PASS，0 FAIL、0 SKIP，退出码 0；sessionorigin 4.106s、session 1.720s、usage 4.805s。
三轮旧反例全部 PASS，包括 FTS UPDATE 后 publication ID 改变；完整原文和此前失败观察继续保留在历史轮次。

Evidence（明确复用，不称为本轮独立执行）：Repair 3 的 `go vet ./...`、`scripts/run-go-test.sh ./... -count=1`、
`scripts/run-go-test.sh -race ./internal/store/ ./internal/session/ ./internal/sessionorigin/ ./internal/desktop/ -count=1` 均报告 PASS，
生产者为 claude-code，来源为本记录 Repair 3 与上述精确交接评论。其候选指纹、工具链与当前产品/测试、依赖一致。
usage race 沿用任务开发交接 `28e3df3f-aa5b-5c93-a4a8-e1e3c398ff68` 与 tasks.md 的最终成功记录：
usage 的代码、测试及相关依赖在三次修复中未变；publication 和 FTS 影子表修改不改变 usage 包的共享内存访问路径。
本轮亦以独立库 oracle 重验其读取结果。依据项目证据复用规则，不因复评阶段或状态文档变化重复上述广泛检查。
CEv1 将已有 L3 记录作为明确标注来源的观察，并由最终状态的 l3-regression 证据显式 roll-up，保留来源状态。

| 准则 | 本轮核验与依据 |
| --- | --- |
| scope-selection | canonical/List 顺序、三范围并集、分页 total、项目集合、缺失/倒序时间、DST 25 小时、搜索两种计数 |
| token-transition | 新副本导致 token mismatch 时回到 unknown；覆盖 matched/unmatched/出生变化/tombstone；usage-only 不造文本会话 |
| same-generation-publication | 两库交错提交、最多两次重试、回滚；三个原文探针、epoch/缺身份/迟到/回退回归 |
| dependency-clock | 原有 trigger 依赖审计加 FTS 影子表；直接 I/U/D、回滚、交错重试、旧库可写打开补齐；现有 helper 路径推进版本 |
| scoped-signals | 独立库只索引所选会话的 oracle；include/exclude/跨 client、shared batch/partition projected 与单次读一致；读取不改 usage 事件与金额来源 |
| partition-invalidation | 仅目标会话的 client/model/day 分区 dirty；切换零写入；增量/全量归约一致；两会话协调 revision +4/2 markers、无变化 +0、单覆盖 +1/1 marker |
| l3-regression | 复用上述精确候选的完整 Go、vet、受影响 race 报告；本轮另执行 31 个定向测试 |
| independent-review | 两个历史 finding 关闭，当前变更与批准合同一致，本轮 PASS |

剩余边界：FTS 每文档行额外一次 revision 写入已记录；最终集成性能预算属于任务 9。
CLI/desktop/Settings/Widget 的消费接入及原生验收仍属于后续任务，不以本轮 Go fixture 代替。
任务层级由 tasks.md 的九任务矩阵和 Beads parent-child/blocks 关系确定；任务 3 之后尚有任务 4–9。
当前 CEv1 未登记 Topic 节点/contains，因此不据空图推断 Topic 完成，也不查询不存在的 Topic gate；本轮仅跨 Task 证据边界。

最终 ContentState：`auxiliary-session-classification:scope-projections:rereview-20261105-zkDsLH-final-state`。
10 个节点精确读回一致（最终状态、L3 来源观察、8 项最终证据）；19 条关系预检全部 ok、写入数量 19。
规范门禁查询得到 VERIFIED（8/8），missing/invalidated/unresolved 均为空。L3 保留原候选来源观察并显式 roll-up；
历史失败仍绑定历史内容，没有被改写或当作当前失败。

完成门禁：VERIFIED

Task checkpoint：`ad-auxiliary-session-classification-scope-projections`；content_state `b4c312ffee287b36faa49953e9d858118b3d34788f6cbace5b3ab25b1a7d066f`；gate VERIFIED（8/8）。
提交建议：将本任务 18 个路径（16 个产品/测试文件、tasks.md、此评审记录）作为一个逻辑提交；执行提交需用户授权。
推送建议：经用户授权推送 `origin/feature/auxiliary-session-classification`；交付时核对远端差异、提交内容、消息与签名。

下一任务为 `cli-contract`，Beads `ad-auxiliary-session-classification-cli-contract` 仍为 open/未指派，
其 scope-projections 依赖须在本任务授权提交交付并关闭后解除。本轮不认领或执行下一任务。

记录校验：`make check-whitespace`、`python3 scripts/check-docs.py`、`git diff --check` 通过。
评审进入 awaiting_commit 检查点；用户随后明确授权本任务本地提交，Codex 保留指派处理交付。
Beads CLI 的 workspace gate lock 权限警告仍存在，主操作以 ungated 模式成功，返回结果已核实。

### 授权本地交付 — 2026-11-05

用户在本轮 VERIFIED 检查点后明确发出“提交”。范围为本任务 18 个路径，产品/测试 blob 必须与上述候选一致，
仅 tasks.md 和本记录追加交付元数据。执行前核验暂存范围、消息与 Claude/Codex 贡献者 trailer；
本提交对象提供实际内容原件。实际提交 ID、tree、SSH 签名核验与提交态 CEv1 对账由对应 Beads 与 CEv1 留存，
不在本提交正文写入自引用 SHA。未授权推送或开始 cli-contract。

### 保证范围变更 — 2026-11-11

本节只记录后续合同变化，不改写上面各轮的结论与证据；它们仍绑定各自当时的内容。

ASC-SP-R3-F1 在 Round 4 的关闭依据是 `session_documents_content` 上的三个 revision 触发器，保证任何写入（含直接 SQL）
都推进 sessions 侧 serving revision。任务 9 测得这些触发器使 FTS5 每次内容写入都刷盘并合并段：带触发器时全量扫描比合并基慢约 11.5%，
移除后约 3%，触发器约占 8 个百分点；
用户于 2026-11-11 决定收窄该保证：触发器移除，改由 session 包的产品写入路径在同一事务显式推进 revision，
直接 SQL 不再覆盖。现行合同见 [architecture.md §6](../architecture.md)，变更的评审与证据见
[integrated-acceptance.md](integrated-acceptance.md) 的 ASC-IA-R1-F1 与 ASC-IA-R1-F5。
ASC-SP-R3-F1 所指的缺陷（产品写入 FTS 后搜索投影沿用旧 revision）在新合同下仍由回归测试覆盖；
其原先对直接 SQL 的覆盖自该日起不再成立。
