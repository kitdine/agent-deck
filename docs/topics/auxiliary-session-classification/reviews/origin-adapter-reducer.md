---
status: active
topic: auxiliary-session-classification
subject: origin-adapter-reducer
---

# origin-adapter-reducer 评审记录

## Round 1 — 2026-11-05

### 📋 origin-adapter-reducer 实现评审

📊 总体评分：7/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进问题 — 必须修复

**ASC-OAR-R1-F1 — P2 — 只检查数据库 schema，未确认受支持的插件版本就接受维护映射。**

- 位置：`internal/sessionorigin/adapters/claudemem/claudemem.go:128-131`、`:224-279`；
  调用端 `internal/sessionorigin/reconcile.go:109-125`。
- 合同：[architecture.md §3.2](../architecture.md#32-支持范围与查询) 要求识别安装版本与 schema 指纹，
  首版仅支持已审查的 15.2.1；更高或未知版本默认不适配，插件候选由扩展定位并校验包版本。
  [tasks.md 任务 2](../tasks.md#2-origin-adapter-reducer) 将显式配置、版本/schema 白名单列为本任务实现范围。
- 行为风险：未知安装或插件升级后，只要数据库仍为 schema 68 且保留两列和唯一索引，适配器就继续接受映射，
  将未经版本合同确认的会话归为 auxiliary。数据 schema 版本不能证明 observer 的用途语义仍与已审查版本一致。
- 原因：`Read` 只接收 dataDir 与候选 ID；`validateSchema` 校验 max(version)、列名和唯一索引，
  随后直接写入 `ValidatedVersion="sdk-schema-68"`。`AdapterConfig` 只有 data_dir/disabled，
  `Reconcile` 也没有先解析、验证或传入插件版本。当前测试的 newer/older version 仅变更数据库 schema，
  没有覆盖插件版本未知或在 schema 不变时升级的情况。
- 实证：隔离 HOME 与仅包含忠实 schema-68 WAL 数据库的临时根，没有受支持的插件安装信息。
  调用 `Read(ctx, root, []string{observerID})` 得到
  `outcome=ok validated_version=sdk-schema-68 mapped=map[11111111-1111-4111-8111-111111111111:7]`。
  定向反例 `TestReviewRejectsUnconfirmedPluginVersion` 失败，测试实际执行 0.05 s；包结果 FAIL。
- 处置：OPEN，载体为本任务 `ad-auxiliary-session-classification-origin-adapter-reducer`。

💡 修复建议：按已批准合同接入插件候选与包版本确认，将其与配置的数据根及 schema 校验一起作为适配前提；
未知、更高或不能确认关联的版本不产生维护证据，并使已有自动证据失效、保持用户覆盖。
补充受支持版本成功、未知版本拒绝、schema 不变但插件版本升级拒绝的隔离回归。
不要将本轮修复变成未经批准的“schema 相同即兼容”合同放宽。

#### 🟢 优点

- 映射查询使用参数化 canonical UUID 候选与 200 项批次，只读取 ID 和 schema 元数据；
  不以 content_session_id、路径或模型推断用途。
- Codex 内建规则显式列出两条 support_entry；Reconcile 按 client 分流，用户覆盖与派生证据分开保存。
- 只在分类或证据变化时替换相应派生行，保留 unchanged evidence 的观察时间。
  这些是已检查实现的正面事实，不替代尚未完成的完整验收。

#### 📝 总结

- Reviewer：Codex，独立于本候选的开发者 claude-code；接受 Beads 交接
  `f26d7767-962e-52ab-93e7-10349ebd7f27`，未使用子代理。
- Method：有效主题绑定核验、任务/架构合同对照、主题自身 CodeGraph 定位、聚焦源码审查、隔离 Go overlay 反例。
  CodeGraph 对适配器路径的查询返回了 store 来源，故对该已知候选路径改用一次直接源码读取。
- Scope：任务 2 的 14 个产品/测试文件及其直接存储、scan/rebuild 调用合同；本轮只写评审与任务状态记录。
  在决定性反例后停止广泛验证，其他行为没有获得本轮完整 PASS。
- Reviewed state：HEAD `55dcf411454b17322eea73b983a7360bb88e6352` + 下表 14 个 blob；
  fingerprint `7426c2d11d6ec11dce8edb0795f0ed32bf6bdaea0e10eb8f49fbec4b614cade4`。
  算法：UTF-8 `head=<完整 HEAD>\n` 后按路径字典序连接 `path=blob\n`，保留最后一个 LF，计算 SHA-256。
  不包含 tasks.md 或评审记录；本轮这些状态文档更新不改变候选身份。
- 开发交接的摘要 `a9e50e716877f4e843ad079e03bbb5a450857f72677b3923a7e12965704744de` 未能由其简述的编码重现；
  本轮以明确算法和逐文件 blob 标识候选，不将开发者全量/vet/race 声明当作已核实的本轮通过证据。
- Environment：隔离 fixture，无私人运行库；有效复现工具链 `go1.27.3 darwin/amd64`，
  `GO_TEST_BIN=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go`，
  `GOROOT=/Users/jobshen/.local/share/mise/installs/go/1.27.3`，包装器默认 `GOCACHE=/private/tmp/agent-deck-go-build`。
  最初 PATH Go 1.27.1 与 GOROOT 1.27.3 不匹配，编译失败，没有执行产品测试；随后仅切换测试二进制并成功执行该反例。
- Evidence：通过临时 overlay 注入下列同包反例，仓库生产文件、测试与配置均未修改。
  重现时把临时文件映射到 `internal/sessionorigin/adapters/claudemem/review_probe_test.go`，使用
  `scripts/run-go-test.sh -overlay "$diagnostic_overlay" ./internal/sessionorigin/adapters/claudemem -run '^TestReviewRejectsUnconfirmedPluginVersion$'`。
  `$diagnostic_overlay` 是重现时新建的 Go overlay JSON；本记录保留完整必要事实，不依赖原临时文件或日志。

```go
package claudemem

import (
    "context"
    "testing"
)

func TestReviewRejectsUnconfirmedPluginVersion(t *testing.T) {
    t.Setenv("HOME", t.TempDir())
    f := newFixture(t, SupportedSchemaVersion)
    result := Read(context.Background(), f.dir, []string{observerID})
    if result.Outcome == OutcomeOK || len(result.Mapped) != 0 {
        t.Fatalf("unconfirmed plugin version accepted: outcome=%s validated_version=%s mapped=%v",
            result.Outcome, result.ValidatedVersion, result.Mapped)
    }
}
```

| 候选路径 | Git blob |
| --- | --- |
| cmd/agentdeck/main.go | `ee608833c0313d3f75e5637b9d3f2907febd706c` |
| internal/scanruntime/scanruntime.go | `ba79fc50d49f3c8cb638d891f21eddce56804705` |
| internal/scanruntime/serving.go | `8163ddfed7bb282f360e512d1c19d2a3024c76e9` |
| internal/sessionorigin/adapters/claudemem/claudemem.go | `1759f385c60e541c24d91f672b2c0849c8d475d7` |
| internal/sessionorigin/adapters/claudemem/claudemem_test.go | `b5182e1e916953e7b24e1c62692e30bcd0e84182` |
| internal/sessionorigin/adapters/claudemem/claudememtest/claudememtest.go | `1cc66913c171f59cd4028f36ea6a6490722c54b8` |
| internal/sessionorigin/binding.go | `fe5c64df175193972fc30d13da06653b1d6def1f` |
| internal/sessionorigin/codexotel.go | `f01494f9d523757edada197696a6b63038b6ce39` |
| internal/sessionorigin/reconcile.go | `8e09db13abf9a4eef0f49212febf95d91ba52d1c` |
| internal/sessionorigin/reconcile_test.go | `a7a0d901eda35baa9351aa156619293ebfb64c43` |
| internal/sessionorigin/reducer.go | `e79f766527daf7cc25d0cf06797b95ca685d8c94` |
| internal/sessionorigin/reducer_test.go | `72a4bd202c243d55898525840b8dd84366b0acd7` |
| internal/sessionorigin/sourceconfig.go | `f1c25302ecafce9680e7a3a39237d80fbbc91bc9` |
| internal/store/session_origin_derive.go | `a2b709450e7995b7ad9fb389bd46dee85d3abfbb` |

- WorkUnit：`auxiliary-session-classification:origin-adapter-reducer`；七项 required criteria 已核对任务合同。
- ContentState：`auxiliary-session-classification:origin-adapter-reducer:review-r1-20261105-7426c2d1-go1273`。
- 完成门禁：FAILED。标准 `gate-status.cypher` 对上述 WorkUnit/ContentState 返回两项适用失败证据
  （claude-mem-adapter、independent-review），另外五项没有本轮通过证据；无失效或未决影响项。
  节点批次 3/3、关系预检 4/4 为 ok、关系写入后读回 4/4；写工具只返回更新摘要，数量与属性另经只读核对。
  初始 Go 1.27.1 环境的候选状态只用于缺口查询，未绑定本轮反例；有效反例绑定上述 Go 1.27.3 状态。
  不进入 Task 提交或 Topic 完成边界。
- 记录校验：`python3 scripts/check-review-records.py docs/topics/auxiliary-session-classification/reviews/origin-adapter-reducer.md`、
  `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 均通过；结构检查不代表语义 PASS。
- Beads 回交：状态 `in_progress`、指派已释放、`round-1`；交接评论 `20515e76-e387-53ed-a9cb-561b566a3a6f`。
  包装器报告本地 workspace gate lock 无写权限并继续运行，实际主操作返回成功；未改动权限或服务。
- 修复范围仅为 ASC-OAR-R1-F1。Review 保持未勾选；没有提交、推送、PR、合并或发布。

## Repair 1 — 2026-11-05

修复者：claude-code（候选开发者，不自评 PASS）。范围仅 ASC-OAR-R1-F1。

### 处置映射

| 发现 | 修改 | 回归保护 |
| --- | --- | --- |
| ASC-OAR-R1-F1 | 适配器在打开数据库前先确认已安装插件版本：经扩展定位读取 Claude Code 插件目录（`installed_plugins.json`），要求恰好一个 user 作用域的 `claude-mem@thedotmack`，其目录版本与安装路径下 `package.json`（name `claude-mem-plugin`）均为已审查的 `15.2.1`；否则 unsupported/`plugin_version_unsupported`（目录不可读为 unavailable/`plugin_catalog_unreadable`），不查询数据库、不产生维护证据。`ValidatedVersion` 改为 `15.2.1/sdk-schema-68`。`Reconcile` 新增 `Home` 并由三处调用端传入；不支持时候选为 unknown/origin_source_unavailable，已有自动证据失效，用户覆盖保留。未放宽为“schema 相同即兼容”。 | `TestReadRejectsUnconfirmedPluginVersion`（含评审反例：无插件目录；未知更高 15.9.0、较低 15.2.0；schema 不变而插件升级 15.3.0；目录与 package 版本不一致；包名不同；无 package.json；其他 marketplace；仅 project 作用域；两个 user 安装；目录不可读；同库下已审查版本成功），各失败场景断言数据库零查询；`TestPluginUpgradeInvalidatesClaudeMemEvidence`（schema 68 不变、插件升级后证据删除、运行摘要 unsupported 且无 validated_version、覆盖保留）。 |

扩展定位：`internal/extension` 抽出只读 `ClaudePluginInstalls`/`ClaudePluginCatalogPath`，原 `scanClaudePlugins` 改为复用同一解析，
行为不变（`internal/extension` 测试通过）。这是合同“插件候选由扩展定位提供”的接点，未改扩展清单的输出。
本机实际安装为 15.9.0，修复后真实环境中该适配器将报 unsupported，符合首版合同。

变异：令插件确认恒通过，两项新回归均失败；已恢复。

### 验证（候选内容如下）

- `go vet ./...`：通过。
- `scripts/run-go-test.sh ./...`：通过。
- `scripts/run-go-test.sh -race ./internal/sessionorigin/... ./internal/extension/ ./internal/scanruntime/`：通过。
- `scripts/run-go-test.sh -race -timeout 45m ./cmd/agentdeck/`：通过。
- 全部为隔离 fixture 与临时 HOME；工具链为包装器默认的 `go`，未在评审者环境重跑。

### 候选身份

HEAD `55dcf411454b17322eea73b983a7360bb88e6352` + 15 个路径（新增 `internal/extension/extension.go`），
按本记录 Round 1 的算法计算 fingerprint `fa21c34ef16decc2db4aaf0af22640ffa19df9868d0524860bece843300ae017`。
开发交接的 `a9e50e71…` 不可重现的原因已查明：当时按本地区域设置排序路径（`claudemem_test.go` 排在 `claudemem.go` 前），
以本记录算法按 Round 1 的 14 个 blob 可重现 `7426c2d1…`。

| 候选路径 | Git blob |
| --- | --- |
| cmd/agentdeck/main.go | `5c06c646a4d2586669c5ffedf745f31b69ce339c` |
| internal/extension/extension.go | `ca7edd33e8f42e49654b686c3a064ef7ee8be0d4` |
| internal/scanruntime/scanruntime.go | `434ad195e49237ba23aaa7ea603f75ee60228032` |
| internal/scanruntime/serving.go | `42c4d0d7105a801dd9201aba3a05af052065e05c` |
| internal/sessionorigin/adapters/claudemem/claudemem.go | `f850c922bec57980ee28c60106e5959f088a4f69` |
| internal/sessionorigin/adapters/claudemem/claudemem_test.go | `d3e596f4f582f892a6d416586108ee5719cee467` |
| internal/sessionorigin/adapters/claudemem/claudememtest/claudememtest.go | `b3fa1072f91057dd887eec2bc6df41da204d403d` |
| internal/sessionorigin/binding.go | `fe5c64df175193972fc30d13da06653b1d6def1f` |
| internal/sessionorigin/codexotel.go | `f01494f9d523757edada197696a6b63038b6ce39` |
| internal/sessionorigin/reconcile.go | `27e627bad15e02e96e652756624a44c5d9e5237c` |
| internal/sessionorigin/reconcile_test.go | `5a4e579e3df2c78fde8f79d040ba6c990289ed79` |
| internal/sessionorigin/reducer.go | `e79f766527daf7cc25d0cf06797b95ca685d8c94` |
| internal/sessionorigin/reducer_test.go | `72a4bd202c243d55898525840b8dd84366b0acd7` |
| internal/sessionorigin/sourceconfig.go | `f1c25302ecafce9680e7a3a39237d80fbbc91bc9` |
| internal/store/session_origin_derive.go | `a2b709450e7995b7ad9fb389bd46dee85d3abfbb` |

未新增 CEv1 证据，未修改 Round 1 结论；Review 保持未勾选，待独立复评。未提交、未推送。

## Round 2 — 2026-11-05

### 📋 origin-adapter-reducer 修复复评

📊 总体评分：8/10

✅ 复评结论：FAIL

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进问题 — 必须修复

**ASC-OAR-R1-F1 — P2 — 部分修复，仍未关闭：混合作用域安装使不受支持版本被忽略。**

- 位置：`internal/sessionorigin/adapters/claudemem/claudemem.go:196-206`（`confirmPlugin`）；
  `internal/sessionorigin/reconcile.go:113-128` 沿用其确认结果生成维护证据。
- 处置：STILL OPEN。本轮不是重新声称“完全没有版本检查”：无安装、单一未知/升级版本拒绝及撤销旧证据均已修复。
  原发现要求在来源版本不能确认时不产生维护证据，混合作用域情况下仍未满足。
- 合同：[architecture.md §3.2](../architecture.md#32-支持范围与查询) 要求确认来源版本，未知或更高版本不适配；
  Round 1 的修复范围明确包含“不能确认关联的版本不产生维护证据”。显式授权数据根不等于确认其生产者版本。
- 原因：`confirmPlugin` 先丢弃所有非 user/空 scope 的安装，再对剩余集合检查恰好一个 15.2.1。
  但 `Read(home, dataDir, candidates)` 没有安装与 dataDir/候选会话的关联信息，`Reconcile` 也不提供此关联。
  同一插件同时存在 user 15.2.1 与 project 15.9.0 时，无法由这些输入证明所读数据来自前者。
- 行为风险：项目级安装覆盖或共存且共享/使用该配置数据根时，不受支持版本的映射仍被标记为
  `15.2.1/sdk-schema-68` 并用于 auxiliary 分类，可能错误隐藏会话；已有辅助证据也不会仅因这个安装变化而失效。
  反例证明的是准入与版本归属缺口，没有宣称 15.9.0 的真实用途语义已经发生变化。
- 实证：隔离 schema-68 WAL fixture 与安装目录；同一 `claude-mem@thedotmack` 有 user 15.2.1 和 project 15.9.0
  两个条目，二者包名和 package.json 版本各自匹配，project 条目带有效临时 projectPath。
  `TestReviewRejectsMixedScopePluginVersions` 得到
  `outcome=ok version=15.2.1/sdk-schema-68 mapped=map[11111111-1111-4111-8111-111111111111:7]`，0.04 s 后断言失败。
- 载体：原任务 `ad-auxiliary-session-classification-origin-adapter-reducer`，不另建发现或扩大任务合同。

💡 修复建议：版本确认不得通过过滤 scope 隐藏冲突安装。在没有可验证的安装—配置数据根关联时，
对同一插件的混合/未知版本保守拒绝，且使已有自动证据失效、保留覆盖；若实现精确关联，则只接受已证实由受支持
版本提供的来源。补充 user 15.2.1 与 project/其他有效作用域未知或更高版本共存的回归及协调层撤销断言。

#### 🟢 优点

- `TestReadRejectsUnconfirmedPluginVersion` 通过（0.08 s）：无安装、单一未知/较低/较高版本、包名与版本不一致、
  缺包清单、其他 marketplace、仅 project 安装、两个 user 安装等情况不产生映射，失败场景不查询数据库；受支持版本仍能映射。
- `TestPluginUpgradeInvalidatesClaudeMemEvidence` 通过（0.18 s）：schema 保持 68、唯一安装升级至 15.3.0 后，
  自动证据删除、结果 unknown、摘要 unsupported，用户覆盖保留。
- 三处生产调用已传入 Home；扩展目录解析复用既有入口，未从用户原始会话或正文推断插件版本。

#### 📝 总结

- Reviewer：Codex；Method：复用本会话已加载的工作区、合同与 CEv1 规则，核对 Repair 1 交接
  `183d88c4-0a2f-507f-9350-3198d0a5c64c`，主题自身 CodeGraph 同步/定位，聚焦修复代码与隔离反例。
  未委派，未修改产品、测试或配置；原先尚未完成的全面验收不由本轮定向复评替代。
- Scope：ASC-OAR-R1-F1、版本确认/扩展目录解析及受影响的协调入口。唯一旧发现部分修复但仍 OPEN；无新增独立发现。
- Reviewed state：HEAD `55dcf411454b17322eea73b983a7360bb88e6352`，Repair 1 表中 15 个 blob 已逐一核对相同；
  按 Round 1 明确算法计算 fingerprint `fa21c34ef16decc2db4aaf0af22640ffa19df9868d0524860bece843300ae017`，与修复交接一致。
  本轮记录和 tasks.md 更新不属于产品候选摘要。
- Evidence：Go 1.27.3 darwin/amd64；显式
  `GO_TEST_BIN=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go` 与
  `GOROOT=/Users/jobshen/.local/share/mise/installs/go/1.27.3`；包装器默认隔离 GOCACHE。
  执行 `scripts/run-go-test.sh -overlay "$diagnostic_overlay" ./internal/sessionorigin/... -run '^(TestReadRejectsUnconfirmedPluginVersion|TestPluginUpgradeInvalidatesClaudeMemEvidence|TestReviewRejectsMixedScopePluginVersions)$'`。
  前两项 PASS，第三项 FAIL；sessionorigin 包 2.239 s、claudemem 包 1.332 s，命令退出 1。
  原修复者全量/vet/race 通过记录保留；在此决定性反例后不再运行广泛验证，也不据此签发其余准则的通过证据。
- 重现：以候选已有 `newFixture` 和 `claudememtest.Home` 创建上述两个安装；给 project 条目补 projectPath 后，
  对同一 schema-68 fixture 执行 `Read`。下列临时同包 Go 文件由 overlay 注入，不写入仓库；
  `$diagnostic_overlay` 是重现时新建的 overlay JSON，映射到
  `internal/sessionorigin/adapters/claudemem/mixed_install_test.go`。本记录不依赖原临时日志或文件。

```go
package claudemem

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem/claudememtest"
)

func TestReviewRejectsMixedScopePluginVersions(t *testing.T) {
    f := newFixture(t, SupportedSchemaVersion)
    user := claudememtest.ReviewedInstall()
    project := user
    project.Scope, project.Version, project.Package = "project", "15.9.0", "15.9.0"
    home := claudememtest.Home(t, user, project)
    path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
    data, err := os.ReadFile(path)
    if err != nil { t.Fatal(err) }
    var catalog struct {
        Version int `json:"version"`
        Plugins map[string][]map[string]string `json:"plugins"`
    }
    if err := json.Unmarshal(data, &catalog); err != nil { t.Fatal(err) }
    catalog.Plugins[SupportedPluginID][1]["projectPath"] = t.TempDir()
    data, err = json.Marshal(catalog)
    if err != nil { t.Fatal(err) }
    if err := os.WriteFile(path, data, 0o600); err != nil { t.Fatal(err) }
    result := Read(context.Background(), home, f.dir, []string{observerID})
    if result.Outcome == OutcomeOK || len(result.Mapped) != 0 {
        t.Fatalf("mixed plugin versions accepted without source binding: outcome=%s version=%s mapped=%v",
            result.Outcome, result.ValidatedVersion, result.Mapped)
    }
}
```

- WorkUnit：`auxiliary-session-classification:origin-adapter-reducer`，原七项 required criteria 不变。
- ContentState：`auxiliary-session-classification:origin-adapter-reducer:rereview-r2-20261105-fa21c34e`。
- 完成门禁：FAILED。标准 `gate-status.cypher` 返回本轮 claude-mem-adapter 与 independent-review 两项适用失败证据；
  另外五项仍缺本轮通过证据，无失效或未决影响项。Round 1 失败保留在原状态，对本轮状态不适用。
  本轮 ContentState 1/1 与 evidence 2/2 节点经读回确认，关系预检 4/4 为 ok、写入后数量 4/4；未覆盖旧证据。
- 记录校验：本记录的 `python3 scripts/check-review-records.py`、`make check-whitespace`、`git diff --check` 退出 0。
  `python3 scripts/check-docs.py` 退出 1：`docs/topics/health-recovery/reviews/extension-inventory-recovery.md`
  中既有 `file:///` 链接被判为 unsupported link syntax / indented list continuation。
  该文件工作区与 HEAD blob 均为 `1f08472758796aa4ea41f2ecf9c03f04c2c69409`，本轮未改动；不扩大修复范围，也不将全局检查计为通过。
- 回交：Beads `in_progress`、指派释放、`round-2`；评论 `bafeef5f-e93d-589d-897a-df90ab61e9b9`。
  最终核对候选 15/15 blob 与本轮开审相同；临时反例及日志已保留必要事实于本记录后清理。
- Review 保持未勾选；不进入任务提交或 Topic 完成边界；没有 Git 交付。

## Repair 2 — 2026-11-05

修复者：claude-code（候选开发者，不自评 PASS）。范围仅 Round 2 仍 OPEN 的 ASC-OAR-R1-F1（混合作用域安装）。

### 处置映射

| 发现 | 修改 | 回归保护 |
| --- | --- | --- |
| ASC-OAR-R1-F1（Round 2） | `confirmPlugin` 不再先按 scope 过滤：收集目录中插件名为 `claude-mem` 的全部安装（任意 scope、版本、marketplace），仅当恰好一个、且它是 user 作用域的 `claude-mem@thedotmack`、目录版本与 `package.json`（`claude-mem-plugin`）均为 15.2.1 时确认；否则 unsupported/`plugin_version_unsupported`，不查数据库、不产生证据。由于没有安装与配置数据根的可验证关联，任何第二个安装（含同版本的 project 安装或其他 marketplace）一律保守拒绝。无关插件不受影响。 | `TestReadRejectsMixedPluginInstalls`：user 15.2.1 + project 15.9.0（评审反例）、+ project 15.2.1、+ local 15.3.0、+ 其他 marketplace 15.2.1、+ 未知 scope 15.9.0 均拒绝且数据库零查询；无关插件并存仍可适配。`TestMixedPluginInstallInvalidatesClaudeMemEvidence`：已有辅助证据在出现 project 15.9.0 后删除、结果 unknown/origin_source_unavailable、摘要 unsupported，覆盖保留。 |

评审反例 `TestReviewRejectsMixedScopePluginVersions` 按 Round 2 记录的代码以 overlay 重放：PASS（0.04 s），未写入仓库。
变异：恢复“仅 user 作用域且精确 ID”的过滤，两项新回归均失败；已恢复。

### 验证（候选内容如下）

- `go vet ./...`：通过。
- `scripts/run-go-test.sh ./...`：通过。
- `scripts/run-go-test.sh -race ./internal/sessionorigin/... ./internal/extension/ ./internal/scanruntime/`：通过。
- `cmd/agentdeck` 本轮未改（blob 与 Repair 1 相同），沿用 Repair 1 的 `-race` 通过结果。
- 隔离 fixture 与临时 HOME；包装器默认 `go`，未在评审者环境重跑。

### 候选身份

HEAD `55dcf411454b17322eea73b983a7360bb88e6352` + 15 个路径，按 Round 1 算法 fingerprint
`fb4c319d2d8233d73042c39085080742515006799ee0c7b22f42f97f7d825154`。相对 Repair 1 仅下列 4 个 blob 变化，其余 11 个不变。

| 候选路径 | Git blob |
| --- | --- |
| cmd/agentdeck/main.go | `5c06c646a4d2586669c5ffedf745f31b69ce339c` |
| internal/extension/extension.go | `ca7edd33e8f42e49654b686c3a064ef7ee8be0d4` |
| internal/scanruntime/scanruntime.go | `434ad195e49237ba23aaa7ea603f75ee60228032` |
| internal/scanruntime/serving.go | `42c4d0d7105a801dd9201aba3a05af052065e05c` |
| internal/sessionorigin/adapters/claudemem/claudemem.go | `f8923e4bd8c27eaf404fe2d7b7041f8136715763` |
| internal/sessionorigin/adapters/claudemem/claudemem_test.go | `718906bbf604c8e9e3ec34e0958b4f848d104548` |
| internal/sessionorigin/adapters/claudemem/claudememtest/claudememtest.go | `b34021441f10371e255c723cd79bfe653eeb4ee3` |
| internal/sessionorigin/binding.go | `fe5c64df175193972fc30d13da06653b1d6def1f` |
| internal/sessionorigin/codexotel.go | `f01494f9d523757edada197696a6b63038b6ce39` |
| internal/sessionorigin/reconcile.go | `27e627bad15e02e96e652756624a44c5d9e5237c` |
| internal/sessionorigin/reconcile_test.go | `6603c57c9f41afabd6d40c4e3220e5ed0cf33a07` |
| internal/sessionorigin/reducer.go | `e79f766527daf7cc25d0cf06797b95ca685d8c94` |
| internal/sessionorigin/reducer_test.go | `72a4bd202c243d55898525840b8dd84366b0acd7` |
| internal/sessionorigin/sourceconfig.go | `f1c25302ecafce9680e7a3a39237d80fbbc91bc9` |
| internal/store/session_origin_derive.go | `a2b709450e7995b7ad9fb389bd46dee85d3abfbb` |

变化路径：`claudemem.go`、`claudemem_test.go`、`claudememtest/claudememtest.go`、`reconcile_test.go`。
未新增 CEv1 证据，未修改 Round 1/2 结论；Review 保持未勾选，待独立复评。未提交、未推送。

## Round 3 — 2026-11-05

### 📋 origin-adapter-reducer 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进问题 — 必须修复

无。

#### 🟢 优点

**ASC-OAR-R1-F1 — CLOSED。** `confirmPlugin` 先统计所有作用域、所有 marketplace 中名为 claude-mem 的安装，
只有唯一安装为受支持的 user `claude-mem@thedotmack`、目录版本和包清单均为 15.2.1 才继续读取数据库。
Round 2 的混合版本反例已纳入 `TestReadRejectsMixedPluginInstalls`，含真实形状的 projectPath；
同版本多作用域、local、其他 marketplace、未知 scope 均保守拒绝，且断言数据库零查询，无关插件不受影响。
`TestMixedPluginInstallInvalidatesClaudeMemEvidence` 验证已有辅助证据被撤销、结果 unknown、运行摘要 unsupported、覆盖保留。
两项均在本轮定向 race 和全量测试通过；Round 1 的无安装/单一升级版本回归也通过。

- 映射仍只读 ID 与格式元数据，不以内容 ID、路径、模型或类似 ID 推断维护用途。
- Codex 两条支持表映射、晚到 rollout 撤销、跨 client 隔离及用量事件/token/金额不变都有隔离端到端断言。
- 归约、稳定出生绑定、token 编码、增量/全量等价、摘要事务故障回滚及覆盖保留的定向测试通过。

#### 📝 总结

- Reviewer：Codex，独立于开发/修复者 claude-code；未委派。
- Method：复用已加载的工作区与合同，接受 Repair 2 交接 `7b469eec-d3e2-52f0-aea5-05621a415fcc`，
  同步主题自身 CodeGraph、聚焦修复与受影响源代码/测试，并核对原先因阻断而未签发的任务验收证据。
- Findings：唯一旧发现 ASC-OAR-R1-F1 已关闭；无仍开放、回归或新增发现。Round 1/2 的历史 FAIL 及反例保留。
- Scope：任务 2 的 15 个产品/测试文件及直接调用、存储合同；生产代码、测试与配置保持只读。
- Reviewed state：HEAD `55dcf411454b17322eea73b983a7360bb88e6352`，Repair 2 表中 15 个 blob 逐一核对一致；
  Round 1 的明确算法得到 fingerprint `fb4c319d2d8233d73042c39085080742515006799ee0c7b22f42f97f7d825154`。
  相对 Round 2 仅适配器、适配器测试、fixture helper、协调测试四个 blob 改变；其余十一项及 HEAD 依赖/配置未变。
  本轮记录与任务矩阵更新不改变产品候选身份。
- Environment：Go 1.27.3 darwin/amd64，显式设置
  `GO_TEST_BIN=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go` 与
  `GOROOT=/Users/jobshen/.local/share/mise/installs/go/1.27.3`，`GOCACHE=/private/tmp/agent-deck-go-build`。
  全部是隔离 fixture/临时 HOME，没有读取私人来源数据库。

验证结果（必要事实留在本记录，不引用临时日志）：

| 验证 | 命令与结果 |
| --- | --- |
| 分类/适配器/存储定向 race | `scripts/run-go-test.sh -race ./internal/sessionorigin/... ./internal/store -run '(Origin\|ClaudeMem\|Read\|Binding\|InputToken\|Reduce\|Effective\|Codex\|Unchanged\|Duplicate\|SourceConfig\|Incremental\|Plugin)'`；退出 0，59 个顶层测试 PASS、0 skip；sessionorigin 24.114 s、claudemem 5.211 s、store 42.533 s |
| Go core 全量 | `scripts/run-go-test.sh ./...`；退出 0，29 个有测试包通过、1545 个顶层测试 PASS、6 skip；cmd/agentdeck 206.897 s、extension 11.765 s、scanruntime 60.539 s、sessionorigin 15.302 s、claudemem 12.849 s、store 33.835 s |
| vet | 显式 Go 1.27.3 执行 `go vet -mod=vendor ./...`，退出 0 |

全量的六个 skip 是 `TestLaunchServicesRefreshPerformanceFixture`、`TestIngestionStageProfile`、
`TestSnapshotPerformanceRepresentativeCorpus`、`TestSnapshotStageProfile`、`TestSnapshotSignalsPairedProfile`、
`TestNativeRegistrationReadOnlyAcceptance`；不计为通过，也不声称完成原生/私人性能样本验收，均不属于本任务的适配器隔离验收。
Repair 2 原有全量/vet/race 声明保留；本轮未直接复用其环境不完整的声明，因为它只记“包装器默认 go”，而本会话已发现过
PATH/GOROOT 版本不一致。故用明确工具链完成上述一次验证；后续阶段可按本候选及实际影响复用，不能仅因阶段变化重跑。

七项任务准则核对：

| Criterion suffix | 本轮证据覆盖 |
| --- | --- |
| claude-mem-adapter | 支持版本+schema 双重确认，跨 scope/marketplace 冲突拒绝，ID 查询白名单，0/1/200/201 批次，WAL/缺 sidecar/权限/超时/版本漂移，以及忠实 observer fixture 正反例；修复前反例已被正式回归覆盖 |
| codex-otel-rule | exec/TUI 精确支持条目、来源 token、未知条目、晚到 rollout 撤销和跨 client；用量身份/token/金额前后比较 |
| reducer-binding-token | 封闭枚举顺序、冲突/混合/环标志、缺失与 mismatch 的保守归约、仅 matched 覆盖、稳定出生身份与 canonical token；未虚构 parser 未提供的用途信息 |
| adapter-run-atomicity | `TestOriginDerivationIsAtomicWithRunSummary`（race 1.96 s）在提交前注入失败，摘要/分类无残留；ok/partial/unavailable/unsupported 与证据同事务，禁用/配置不可读/未配置不写新运行摘要，覆盖保留 |
| incremental-full-equivalence | 增量变化序列与清空派生后全量重建的分类/证据值一致；无新 adapter run 的 unchanged pass 不重写行、不推进 serving revision；来源离开索引只清派生行 |
| l3-regression | 上述全量、vet 与相关 race，含隔离 WAL、权限、超时、版本漂移和事务边界检查 |
| independent-review | 本轮独立复评 PASS，ASC-OAR-R1-F1 CLOSED，无开放发现 |

范围解释：已启用 claude-mem 的每次协调仍须按任务 2/架构 §4 写新的 finished_at 运行摘要，这本身是新状态并推进 serving revision；
不将无 adapter run 的 no-change 断言推广为“已配置适配器每轮零写”。额外写放大由任务 9 的集成预算验收；
sessions 侧 token 列、过渡窗口屏蔽及最终 scope 发布属于任务 3，本轮未宣称完成。

- WorkUnit：`auxiliary-session-classification:origin-adapter-reducer`；七项 required criteria 保持原身份与要求。
- ContentState：`auxiliary-session-classification:origin-adapter-reducer:rereview-r3-20261105-fb4c319d`。
- 完成门禁：VERIFIED（7/7）。标准 `gate-status.cypher` 对上述 WorkUnit/ContentState 返回全部七项必需准则有效通过，
  missing_criteria、invalidated_evidence、unresolved_candidate_impacts 均为空。
  新状态节点 1/1、通过证据节点 7/7 经读回核对，关系预检 14/14 为 ok、写后读回 14/14；旧失败证据保留且不适用于新状态。
- Hierarchy：本 WorkUnit 当前没有图中的 contains 父/子关系；项目 tasks.md 与 Beads 明确这是九项中的任务 2，
  任务 3 `scope-projections` 为 open、受本任务提交交付阻塞。没有达到 Topic 完成边界，不查询或声称 Topic VERIFIED。
- 文档检查限制：Round 2 已确认的 health-recovery 历史记录链接问题未在本轮修改，仍不把全局 check-docs 判为通过；
  本轮记录和状态同步使用结构与空白校验，保留原限制，不扩大修复范围。
- 本轮记录校验：`python3 scripts/check-review-records.py docs/topics/auxiliary-session-classification/reviews/origin-adapter-reducer.md`、
  `make check-whitespace`、`git diff --check` 均退出 0；结构通过不替代上述语义复评与 CEv1 结果。
- Beads：`awaiting_commit`，评审指派已释放，`round-3`；检查点评论 `bfd90cee-0266-5eb5-a5ed-bbb5c91ccf74`。

Task checkpoint：`ad-auxiliary-session-classification-origin-adapter-reducer`；content_state=`fb4c319d2d8233d73042c39085080742515006799ee0c7b22f42f97f7d825154`；gate=`VERIFIED`。
提交建议：按单一任务边界提交上述 15 个产品/测试文件及本评审记录、tasks.md，共 17 个路径；不包含全局状态或其他主题，执行须由用户单独授权。
推送建议：建议目标为 `origin/feature/auxiliary-session-classification`（当前未配置上游）；须先完成授权提交，核验实际提交范围、消息、trailers、SSH 签名和目标 ref，并取得用户单独推送授权。

以上仅为建议，未执行提交或推送。当前任务完成授权提交并关闭后，下一任务为 `scope-projections`。

### 本地提交交付 — 2026-11-05

用户在 Round 3 Task checkpoint 后明确授权“提交”，范围为上述 15 个已评审产品/测试文件以及 tasks.md、本记录，共 17 个路径。
本次提交将上述候选和必要交付记录作为一个任务边界；产品 blob 保持 Round 3 状态，复用 VERIFIED（7/7）证据。
本提交的 Git 对象为交付原件；实际提交 ID、tree、SSH 签名及提交态 CEv1 对账保存在本任务 Beads 和 CEv1。
贡献者：claude-code 提供实现与两轮修复，Codex 提供本记录及任务状态/交付记录；提交保留双方 trailer。
授权仅为本地提交，不包含推送、PR、合并或发布；`scope-projections` 仅在本任务交付核验与关闭后解除前置阻塞。
