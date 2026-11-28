---
status: active
created: 2026-11-17
---

# v0.7.5 版本契约

本版本集中处理既有缺陷。版本成员来自 Beads `ad-v075-iteration`。2026-11-27 对账后，交付成员为 11 项原计划代码修复、1 项 CI 流程修复，以及当日纳入的 2 项修复（设置窗口 1x 布局振荡、桌面扫描失败重跑）；`codex-subagent-jsonl-session-identity` 由用户移出本版本，旧 Cask 首次升级验收/处置仍为 deferred。成员资格、实现、评审、提交及集成分别登记。

用户授权自动完成并发布 v0.7.5，覆盖本版本的契约、开发、独立评审、验证、按 issue 提交及推送、创建 PR、符合门禁的合并、版本集成和稳定版发布。允许按可用性使用现有 Codex、Claude、Antigravity、OpenCode harness。每个 PR 须在 push 与 PR Actions 通过后取得针对实际 head 的 Codex review，并关闭范围内发现。Cask 验收维持延期，隔离原生验收沿用用户已批准的本机配置边界。

本契约只负责版本成员、依赖、集成和稳定契约对账。新增账号、wire 或会话身份决策由所属功能主题承载，经过适用的文档评审后才能实施；不得用版本任务文档代替功能设计。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [x] | [x] |
| 2. `v0-7-5-contract` | [x] | [ ] |

这些任务是版本集成与对账边界，不是把原有 Bug 复制成新的实现任务。文档任务为 `ad-v0-7-5-contract-doc-tasks-design`；当前未创建尚未批准的集成实现任务。

## 范围与非目标

- 修复额度探测、套餐与账号辨识、构成展示、扫描健康、读池等待、分类性能、JSONL 子会话身份和中断/测试同步。
- CI 作业平台与 `Verify` 耗时以一个 P1 issue 交付，实际托管验证通过并经 PR 78 合并。[集成记录](reviews/assemble.md)
- 不纳入定价系统重构、订阅月费估算、模型价格映射或 OpenCode 产品支持。
- 不通过更改总 token、金额、定价、原始套餐枚举或模型分组掩盖展示问题。
- 不读取认证文件、保存令牌、记录原始身份诊断或把真实会话作为自动化夹具。
- 不新增 Claude 额度窗口；额外模型行不自动形成新额度产品。
- 不修改旧 v0.7.0 工作树或其暂存区来清理迁移来源。
- Cask 项继续保留 deferred；版本成员资格不自动授权修改已安装应用、Homebrew 状态或执行卸载升级。

## 成员与优先级

下列 ID 统一带 `ad-bug-` 前缀。A 表示恢复既有契约；B 表示需要所属主题先完成新契约设计。批次表示优先级，不增加无依据的批次间阻塞。

| 批次 / 优先级 | Issue | 路线与完成边界 | 允许的代码范围与验证 |
| --- | --- | --- | --- |
| CI / P1 | `ci-runner-and-verify-duration` | A。按用途和依赖审计每个 workflow/job，将确认可移植的工作安排到 Linux。按 Actions 计时定位 `Verify` 瓶颈，评估降低重复初始化或按包并行，保持现有检查覆盖。 | CI workflow 和验证脚本。记录逐 job 的平台依据、真实 push/PR 的分类与门禁输出，以及可比的执行和排队耗时。已随 PR 78 并入。 |
| Batch 1 / P1 | `claude-quota-probe-gui-path` | A；恢复 GUI/helper 对用户已安装 CLI 的发现，不依赖手工软链，不加载 shell rc。保留显式可执行文件覆盖及原有信任边界。 | `internal/quota/` 的发现/启动边界，必要的 `AgentDeckShared/EmbeddedHelperRunner.swift` 及相关测试；L2，涉及执行路径信任按 L3。 |
| Batch 1 / P1 | `claude-quota-output-plan` | B，整体由 `subscription-account-identity` 承载：包含已授权账号/套餐扩展和原解析/缓存恢复的诊断处置；不把同一 Bug 拆成两个可独立关闭的 lane。历史 parse_failed 未精确复现，不预设持续格式漂移。 | quota adapter/domain/store、desktop wire 及 CLI/App/Widget；必要的身份备份/恢复消费者为 `internal/backup/backup.go`、`internal/backup/backup_test.go`，具体行为先由所属主题批准；L3。 |
| Batch 1 / P1 | `codex-quota-plan-display` | B；原始 `planType` 与展示名分离，纳入既有 issue 已批准的 account/read 账号展示。已观察样本 `prolite` 对应官方 App 的 ChatGPT Pro 100；映射仅影响文案，未知值回退原值。 | quota/store/desktop wire、原生消费者，以及新增身份必要的 `internal/backup/backup.go`、`internal/backup/backup_test.go`；具体行为先由所属主题批准；L3。 |
| Batch 2 / P2 | `menubar-token-mix-double-count` | A；构成切片互斥、之和与代表总量一致，保留 Codex 缓存为 input 子集、Claude 缓存独立的计量语义。共享投影变更先在所属契约明确。 | `MenuBarViewModel.swift` 及测试；必要时 usage presentation/shared wire/Widget；UI L1，共享投影 L2。 |
| Batch 2 / P2 | `all-scope-duplicate-model-rows` | 沿用用户选择 B：保持 `(client, model)` 拆分，在全部客户端视图显式标注客户端。此方案字母不是 Bug lane。 | `apps/macos/AgentDeckApp/MenuBarPanelViews.swift` 模型行及 MenuBarViewModel、WidgetViews、必要共享格式化与测试；L1 加渲染检查。 |
| Batch 2 / P2 | `serving-health-stale-after-refresh` | A；复用迁入候选，Refresh 清除已结束扫描及已解决警告，真实活动锁仍可见，覆盖旧 persisted snapshot 和所有读取路径。 | 下列八文件候选，必要时经边界确认覆盖原生直读路径；L3。 |
| Batch 2 / P2 | `health-self-scan-lock` | A；与上一项共用一个实现者、一个候选；分别保留 issue 验收映射，不重复修改同源代码。 | 同一候选与 L3 证据，不能因关联而略过验收。 |
| Batch 3 / P2 | `selection-verify-200ms-deadline` | A；消除高负载下内部短期限导致的误判，保留有界等待、调用者取消、generation 向量验证及真实不可用错误。 | `internal/store/serving_generation.go`、相关读池及 `internal/sessionorigin/` 消费者/测试；L3。 |
| Batch 3 / P2 | `turn-classification-quadratic` | 本轮路线委托下的 A 仅适用于无 schema 变更的预聚合，消除逐 turn 重复扫描且结果不变；若必须索引/迁移则停止 A 并按下文重新分诊，编号分配不代替授权。 | `internal/usage/signals.go` 与分类测试；无迁移 L2。获准变更方案后，store 迁移及测试按 L3。 |
| ~~Batch 3 / P2~~ | `codex-subagent-jsonl-session-identity` | 2026-11-27 用户决定停止推进并移出 v0.7.5，Beads 转为 deferred，现有工作树、分支与验收材料保留供后续重做。本版本不交付该项。 | 不纳入本版本集成。 |
| 新增 / P1 | `desktop-scan-failed-retry-loop` | A（2026-11-27 用户确认）；原为发布门禁，用户改为本版本直接修复：自动刷新失败退避与来源变化最小间隔、FTS 批量清理、结构化诊断日志。修复记录 `docs/archive/fixes/desktop-scan-failed-retry-loop.md`。 | Swift 刷新调度、`internal/session`、`internal/scanruntime`；PR 86。 |
| 新增 / P1 | `6d5i`（Beads `ad-6d5i`） | A（2026-11-27 用户确认）；1x 显示器上设置窗口布局无限振荡、主线程持续满载，关闭窗口后不停止。修复记录 `docs/archive/fixes/settings-window-layout-oscillation.md`。 | 设置窗口视图与控制器及测试；PR 85。 |
| 新增 / P3 | `health-copy-reset-test-timing`（Beads `ad-58gc`） | A，test-only；健康项复制用例固定等待只给 1.6 s 复位任务留 100 ms，负载高的 CI 上失败并阻塞 PR 87；改为有界轮询，产品代码不变。修复记录 `docs/archive/fixes/health-copy-reset-test-timing.md`。 | `apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift`；PR 88。 |
| 新增 / P3 | `quota-identity-relative-time-test`（Beads `ad-31m5`） | A，test-only；额度身份卡用例只在开头取一次时间，矩阵跑满约 30 s 后读取年龄落在相对时间取整边界，CI 上渲染与断言分落两侧并阻塞 PR 87；改为按每组渲染时间生成读取时间，产品代码不变。修复记录 `docs/archive/fixes/quota-identity-relative-time-test.md`。 | `apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift`；PR 89。 |
| Batch 3 / P2 | `cask-uninstall-unregistered-widget` | 保留 deferred；新模板已交付不等于旧 Cask 首次升级验证通过。记录仍未验证的路径和处置，执行真实安装操作前另需授权。 | 当前仅文档与只读核验；未来限定 Homebrew 模板及安装验收，L3。 |
| Batch 4 / P3 | `watch-interrupt-mid-round-exit-1` | A；轮内调用者取消与既有轮间正常停止一致。独立运行错误、emit 错误及锁释放失败保持失败，不产生虚假成功 fingerprint。 | `internal/watch/` 及必要 cmd 信号/退出码测试；L3。 |
| Batch 4 / P3 | `native-ownership-query-test-readiness` | A，test-only；continuation 安装后显式确认就绪，替代固定 yield 次数；保留锁、释放和取消断言及有界 teardown。 | `AgentDeckTests/EmbeddedHelperRunnerTests.swift`；L1，受控延迟调度。 |

套餐账号与 JSONL 功能主题使用无版本号的稳定名称。来源 issue 各自由 `subscription-account-identity` 和 `codex-jsonl-session-identity` 的独立工作树承载，当前实现、评审与证据按所属工作树中的任务和正式记录核对。

原生路径简称映射：`MenuBarViewModel` / `MenuBarViewModel.swift` 为 `apps/macos/AgentDeckApp/MenuBarViewModel.swift`；`MenuBarPanelViews` / `MenuBarPanelViews.swift` 为 `apps/macos/AgentDeckApp/MenuBarPanelViews.swift`；`WidgetViews` / `WidgetViews.swift` 为 `apps/macos/AgentDeckWidget/WidgetViews.swift`；DesktopWire / shared wire 为 `apps/macos/AgentDeckShared/DesktopWire.swift`；EmbeddedHelperRunner / helper 为 `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift`；ownership 测试为 `apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift`。共享测试包括 `apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift`、`apps/macos/AgentDeckTests/DesktopWireTests.swift` 和上述 ownership 测试；其余测试路径由所属主题或 Lane A fix 的批准范围明确后登记。简称不授权修改同名的其他文件。

## 所属主题必须覆盖的契约

下列是设计与验收要求，不在本版本文档中发明具体字段或协议。

### 额度、套餐与账号

- 保留原始 `plan`、窗口、百分比、reset 语义；显示名不替换原 wire 枚举。
- 明确新增字段 absent/null、错误、陈旧与旧快照解码行为；列全 CLI、菜单栏、Widget、存储和本地化消费者。
- Claude auth status 与 usage 使用同一 CLI、环境和配置上下文；身份与额度分别记录来源和时间。
- 当前登录账号不证明旧额度归属。不得因 auth status 成功直接把 Claude attribution 改为 confirmed；切换账号/组织后不向旧额度贴新邮箱。
- Codex 同一 app-server 的 account/read 与额度读取也需验证认证上下文一致；切换、身份读取失败、未登录、API-key 模式、旧 CLI 不支持和字段缺失均需明确状态。
- 仅保留批准的最小脱敏展示信息，不持久化原始邮箱、令牌或原始 usage 正文。
- Pro 100 映射保留样本与内部枚举含义的证据边界；不推算月费、权限、配额或地区渠道价格。
- 所属主题明确修订 `subscription-quota/requirements.md` 的“Claude 两通道无账号/套餐”和“opaque 套餐直接显示”约束，以及其 architecture 的 planType 映射表：新增公开身份通道与独立显示名；原始枚举保留，不参与计费/权限。保留旧主题历史，在稳定契约对账时记录替代关系。
- 原解析问题仅对可复现形态修复。无法重现的旧坏 stdout 标记“未复现，不作猜测性解析修改”，不伪造失败先行证据。原 Bug 的诊断/恢复验收和账号扩展全部满足所属主题任务 gate 且授权交付后才能关闭，局部通过不关闭整体。

### 构成与子会话

- 混合客户端构成不能把所有原始 input/cache 字段直接相加，也不能从混合 input 一律扣掉全部 cache read。
- 方案 B 保持 `(client, model)` identity、数值和占比；App 与 Widget 标签规则一致。
- JSONL 明确 rollout 身份、事件所属 thread、parent 关系和继承历史的优先级；覆盖新旧事件并存、仅新事件、append/restart、重复扫描及 OTel/JSONL 并存。
- 历史回填可重复执行，来源只读；保留 override/exclusion、金额语义及来源所有权，未知身份不猜测为父或子。

## 依赖、工作区与写入边界

1. 本契约的成员与依赖独立评审；新契约主题按 requirements → UX framework → architecture → UX final → tasks 的适用顺序完成设计评审。版本文档不代替这些审批。
2. 不依赖新契约的 Lane A 修复可先执行，分别使用原 Bug 与 `docs/fixes/<slug>.md`；已有工作树优先复用。
3. GUI PATH 修复先于最终同环境额度联合验收。Claude/Codex 账号实现共享 quota/store/wire，按一个 owner 或串行任务安排写入。
4. 所有成员跨工作区的原生共享路径由主控统一登记写者或串行顺序，覆盖账号主题、token mix、模型行与必要的 serving 原生读取修复；包含 DesktopWire、WidgetViews、MenuBarViewModel、MenuBarPanelViews、EmbeddedHelperRunner 及其共享测试。独立工作树也不能同时推进互不协调的同文件改动。
5. serving 与 self-scan 共用候选；classification、selection、JSONL 先对齐 store/schema。当前 main `4ac56db` 的 core schema 为 35；新迁移由主控核对实际目标 main、全部在途批准迁移及候选后顺序分配，不在本契约静态预留 36，也不由 Worker 自选编号。
6. watch 和 native test-only 可独立并行。一个工作区每个重叠范围只允许一个写者；转移前观察旧写者及其子操作停止。
7. 主控维护文档、正式评审结论、状态、Beads、CEv1 与 Git；Worker 仅修改批准的生产代码及测试，或只读提供设计/评审材料。
8. 完成修复并通过评审与所需证据后保留 `awaiting_commit`。未经交付授权不关闭原 Bug，不通过文档任务关闭缺陷。

A 路线只恢复现有语义。若需要新增 wire 字段、持久状态语义、公开输出形态或产品优先级决策，立即停止该 A 实现，在原 Bug 记录 B 设计与授权来源；已有用户授权覆盖同 issue 决策时按所属主题的设计/独立评审推进，超出授权的 lane 或范围选择必须等待用户确认。增加文件或补原生读取路径本身不是 B 的判据。索引/schema 方案若超出本轮委托选择的无迁移性能修复边界，也先完成上述分诊。Token mix 由暂存问题进入本轮 A，依据是本次修复方式委托及既有计量语义恢复，未授权重定义原始 token 字段。

合同工作树为 `feature/v0-7-5-contract`，只承载本版本文档与记录，不作为产品代码汇集分支。当前基线为 local main `4ac56db32e13c4e925b648c620ba76fc588a7781`；不为了同步基线自动 fetch/pull/rebase/merge。

每个活跃 A 修复使用独立工作区 `agent-deck.fix.<slug>`，分支 `fix/<slug>`；B 使用所属功能主题工作区。T3 创建或复用工作树后，必须先验证实际路径、分支、共同 Git 目录与执行根范围，再登记绑定并开始产品阶段。已创建的工作树不重新创建。

serving 候选保留在 `fix/serving-health-stale-after-refresh`。旧工作树 staged tree `3b8a5af698557b0bb5d023bfeebb8a98fa563693` 仅是迁移来源；正式候选以新工作树当前八文件内容指纹为准：

```text
cmd/agentdeck/acceptance_test.go
internal/desktop/desktop.go
internal/desktop/serving_cache.go
internal/desktop/serving_health_test.go
internal/doctor/doctor.go
internal/doctor/self_lock_test.go
internal/scanruntime/serving_health_test.go
internal/store/store.go
```

## 集成与稳定契约任务

### 1. `assemble`

- 前置：适用文档与各修复独立评审通过，所需 CEv1 gate 对实际候选有效；明确 Cask deferred 处置和未验证范围。
- 在获得对应交付/集成授权后，按 [Branching](../../../.agent-instructions/branching.md) 记录 source/target/result 内容身份、接口消费者、合并类别、冲突解决和交互验证。
- 范围：批准的成员提交及必要集成修复；不包含功能扩展。无文本冲突不代替集成评审。
- 验证：复用仍有效的成员证据，只补跨成员交互与失效范围；Go 核心契约变化执行规定的核心回归，原生消费者执行相关回归。按最终风险取 L3。
- 用户已授权完成并发布 v0.7.5，包含必要合并和集成。逐项记录实际并入与证据。总 assemble 仍待剩余成员和集成门禁。

### 2. `v0-7-5-contract`

- 前置：assemble 的真实集成结果、独立评审及相应 gate；不能对各分支未集成候选提前宣布版本完成。
- 将已批准且已集成的稳定行为对账到 `docs/specs/cli-design.md`、`cli-manual.md` 及适用项目状态/版本记录；保持原始兼容字段和隐私边界。
- 在各自获得授权的交付与证据最终化边界，归档已交付 fix 至 `docs/archive/fixes/`，保留 historical/retired 元数据，不加 fix 索引项；已完成主题整体归档至 `docs/archive/topics/` 并更新 `docs/archive/README.md`。对账稳定导航和失效链接，只有导航拓扑变化才修改 `docs/README.md`，不提前退休未完成主题。
- 验证：受影响文档发现、链接、whitespace、diff 检查（L0）；对账揭示产品差异时回到相应修复，不以文档覆盖缺口。
- 不包含版本标签、发布预检、Homebrew 更新、安装或生产验收授权。

## 验证与完成边界

- L0：`bash scripts/check-topic-docs.sh v0-7-5-contract`、`make check-whitespace`、`git diff --check` 及相关链接检查。
- L1–L3 按 [Project Rules](../../../.agent-instructions/project-rules.md#testing-and-verification--测试与验证) 选择；Go 测试使用 `scripts/run-go-test.sh`。不把完整 release 检查附加到所有修复。
- A 修复提供修复前失败、修复后成功的回归证据；并发改动增加相关 race、vet 与取消/清理检查。
- 所有独立评审用冷上下文和独立角色，优先不同 harness；候选变更按实际影响复评，不凭 Worker 自述勾选 Review。
- 证据绑定代码、测试、依赖、配置、toolchain 与相关环境；阶段切换不使未变证据失效。
- 每个 A 的 `fix:<slug>` gate 独立适用；关联问题可复用同候选证据，但保留各自验收映射。
- 原生 UI 验收使用现有绑定工作树与现有本机配置，先绑定隔离 fixture、应用身份、进程与窗口。需要切换或新建工作树、修改本机配置或更改 VoiceOver 设置的项目保持未验证。实际 App 验收须有原生观察证据。
- 版本联合验收覆盖账号切换、App/Widget 一致性、token 构成守恒、旧快照刷新、高负载 selection、长会话分类、JSONL/OTel 去重、轮内外 SIGINT/SIGTERM 和 native readiness。
- Cask 的旧版本首次升级及其他实际环境操作在获得具体授权后执行；未执行项保留未验证，不能用新模板测试代替。

## 发布门禁：扫描失败重跑复验

2026-11-25 用户把 `ad-bug-desktop-scan-failed-retry-loop` 纳入本版本作为发布门禁。2026-11-27 用户更正：该问题不延后，发布的版本必须包含其修复；修复已随 PR 86 并入，版本号仍为 v0.7.5。

- 发布预检前，用集成后的 main 构建，在通过 `sqlite3 .backup` 得到的隔离状态拷贝上复验：冷、热及连续多轮（含来源追加）不出现 `scan_failed` 循环，诊断日志行为符合契约。不在真实状态目录上运行候选构建。
- 复验结论登记在 [集成记录](reviews/assemble.md)；未通过时不打标签、不发布，转回修复。
- 仅在实际层级到达边界时查询 Topic/Integration gate；逐项 PR 不等于版本已集成，未经授权的 merge 仍是明确的未完成边界。
