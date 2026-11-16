---
status: active
topic: installed-state-ownership
subject: ux/cli.md
---

# CLI UX Review

## Round 1 — 2026-11-05

### 📋 CLI 最终交互契约评审报告

📊 总体评分：7/10

✅ 评审结论：FAIL

- Reviewer: Codex；独立于本批文档作者的当前评审会话，未委派。
- Method: 设计契约评审、原始缺陷交叉核验、所需源码调用路径核验及临时目录内的设计规则反例；未执行真实安装、真实数据操作或产品实现。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership`，分支 `feature/installed-state-ownership`。
- Scope: 声明的全部新增 CLI 状态、doctor、错误 envelope、静默 Hook，以及与架构和稳定 Doctor 契约的一致性。
- Reviewed state: blob `2757f21e64e53cbcc734be5ca32a4cb51487bd81`；fingerprint `97973006fab5c75e5fb87909daa29f156b78c832359c992df5472e8820462cee`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:ux/cli.md`。
- Target ContentState: `installed-state-ownership:ux-cli:review-20261105-a0fXv0-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 查询返回本 WorkUnit / ContentState 的 FAILED；review 失败证据 applicable=true，L0 通过证据有效；invalidated_evidence 与 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py`、`make check-whitespace`、`git diff --check` 全部 exit 0；主题集合检查也为 exit 0。静态检查不代表语义评审通过。

#### 🔴 严重问题 — 必须修复

**ISO-UX-R1-F1 — P2 — doctor 在真实状态被拒绝时的结果未与统一解析器对齐。**

- 位置：[ux/cli.md](../ux/cli.md) 第 25、58、93 行；[architecture.md](../architecture.md#状态目录解析) 第 76–81、158 行。
- 行为风险：`doctor --state-dir <真实目录>` 可能直接 exit 1 且没有 installation 检查，也可能输出 partial 的成功报告；当前两条要求不能同时据以实现或编写固定测试。
- 证据：UX 第 93 行保证“状态目录缺失或被拒绝时它仍会输出”，状态矩阵则将真实根拒绝定义为错误 envelope / exit 1。架构要求统一的 `installation.StateRoot` 对真实 override 返回 error，只在 `doctor.Service` 内新增检查。当前 `newDoctorCommand`（`cmd/agentdeck/main.go:3498-3517`）在 `opts.stateRoot()` 返回错误时立即退出，尚未调用 Service。`cli-design.md` 的 Doctor 契约又规定已渲染的诊断报告使用成功 envelope、exit 0，失败打开时标 partial / checks_skipped。
- 💡 修复建议：明确 doctor 的受拒绝路径是安全的 partial 报告还是普通拒绝，并统一 UX、架构、命令入口顺序及退出码；若保留第 93 行承诺，明确在不访问真实根的情况下如何产出 installation 检查及跳过其余检查。补充这一状态的文本/JSON样例和固定测试要求。
- Disposition: OPEN。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

错误原因使用稳定枚举，普通拒绝给出可执行恢复方向；stderr 提示与 JSON/NDJSON 分离；不输出其他安装的路径。

#### 📝 总结

本轮覆盖 framework 与 final 契约一致性，但没有批准 final surface。已核对 `newDoctorCommand`、`doctor.Service.Check`、`writeDoctorResult` 的实际调用顺序，以及 [稳定 CLI 契约](../../../specs/cli-design.md#doctor)。已找到决定性契约冲突，未继续开展全面原型或运行时验收；现有代码块样例不构成共享产品原型已验收的声明。

Evidence: 路径为 `newDoctorCommand → opts.stateRoot → doctor.Service.Check → writeDoctorResult`；错误从第二步返回时不会执行第三步。未安装路径不存在产品实现，本轮没有声称运行了未来行为。文档结构和链接的最终 L0 结果登记到 CEv1。

修复范围：ISO-UX-R1-F1，并同步该决策的架构与任务测试要求。

## Repair 1 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-UX-R1-F1，并按发现要求同步架构和任务中的对应部分。同时把架构修复带来的新 CLI 表面写进本文档：`install claim`、`managed_command_owner` 检查、新原因码 `real_root_unresolved`。
- Beads：`ad-installed-state-ownership-doc-ux-cli` 由 claude-code 领取修复。

### ISO-UX-R1-F1 处置：已修复，待复评

决定：未安装进程的 `doctor --state-dir <真实目录>` 输出一份**安全的部分报告**，不按普通拒绝处理。理由是 Doctor 契约（`cli-design.md` Doctor 一节）已经规定：报告成功渲染就以 exit 0 结束，失败打开时标记 partial 并给出 `checks_skipped`。部分报告正好落在这条现有契约内，也满足原第 93 行"被拒绝时仍会输出 installation 检查"的承诺。

- [ux/cli.md](../ux/cli.md#状态矩阵)：状态矩阵把真实目录被拒绝拆成两行，"普通命令：错误 envelope，exit 1"和"`doctor`：成功 envelope，`partial: true`，`checks_skipped`，exit 0"，两行不再互相冲突。
- [ux/cli.md](../ux/cli.md#状态目录被拒绝时的部分报告)：新增文本和 JSON 样例。报告只含 `installation` 与 `state`（`error`、`state_ownership_refused`），其余检查跳过，不访问被拒绝的目录。
- [architecture.md](../architecture.md#doctor)：确定入口顺序。`newDoctorCommand`（`cmd/agentdeck/main.go:3498-3517`）收到 `ErrStateOwnershipRefused` 时，不返回错误，而是直接构造这份报告，经 `writeDoctorResult` 输出，并且不把被拒绝的目录交给 `doctor.Service`，因此不会执行任何 `os.Stat`、锁诊断或 hookrefusal 读取。其他错误仍按原样返回。
- [tasks.md](../tasks.md#4-installation-diagnostics)：任务 4 增加这条路径，要求在访问屏障下用固定测试覆盖。

### 随修复同步的表面

- 新增 [`install claim`](../ux/cli.md#install-claim) 的成功和拒绝样例、`install_claim_refused` 错误码，以及三个原因码。它来自用户对 ARCH-F1 的决定。
- 新增 `managed_command_owner` 检查的三种情况及样例。它来自 ARCH-F4 的修复。重新登记命令已与 `cli-manual.md` 第 989 行核对，写为 `agentdeck usage hook setup --client claude`。
- 原因码表增加 `real_root_unresolved`（ARCH-F3），并说明 `receipt_missing`、`receipt_other_executable` 覆盖回执过期和所有权已转移的情形。
- 状态矩阵新增"长驻进程中途失去所有权"一行（ARCH-F2）。

### 验证

结构检查与 [需求记录](requirements.md#验证) 中的 Repair 1 验证相同，均通过。部分报告样例中的 envelope 字段已与 `internal/output/output.go` 的 `Envelope` 核对；`data.checks` 沿用 `doctor.Report` 的 `checks` 字段。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`；`ux/cli.md` blob `f535a397ed6a474402e3e40cf9b0886fb56f99b2`；指纹 `98ee0ef11e2159fb724e8d9c6e21ada74209cd577df5af43f19da8f7fcc6a844`，配方同 Round 1。Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 2 — 2026-11-05

### 📋 CLI 交互契约复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；复核 claude-code 的 Repair 1 候选，未委派，未修改设计正文、产品代码、测试或配置。
- Method: 按原发现逐项处置；复用相同 HEAD 的源码事实，并检查修复新增的契约。设计反例与生产运行验收严格区分。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `f535a397ed6a474402e3e40cf9b0886fb56f99b2`；fingerprint `98ee0ef11e2159fb724e8d9c6e21ada74209cd577df5af43f19da8f7fcc6a844`。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>`。
- WorkUnit: `installed-state-ownership:ux/cli.md`；Beads: `ad-installed-state-ownership-doc-ux-cli`。
- Target ContentState: `installed-state-ownership:ux-cli:rereview-r2-20261105-G-VuYw-state`。
- 完成门禁：FAILED
- Gate evidence: Neo4j 固定 gate-status 对本轮精确目标返回 FAILED；本轮 review/L0 证据均适用，旧状态证据 target_matches=false；无失效证据或未决影响。
- L0 results: `python3 scripts/check-docs.py` exit 0，覆盖未跟踪 Markdown 的链接、记录解析及内部 whitespace/diff/topic 检查；显式主题集合检查也为 exit 0。
- Scope: ISO-UX-R1-F1，以及新加入的 install claim、managed_command_owner 和所有权丢失状态；按最终 surface 的要求复评。

#### 🔴 严重问题 — 必须修复

**ISO-UX-R2-F1 — P2 — 新增交互状态没有接入或引用共享产品原型。**

- 位置：[ux/cli.md](../ux/cli.md) 第 10、121–150、152–171、183–212 行。
- Disposition: NEW / OPEN。
- 行为风险：公共认领命令、拒绝后的 doctor 部分报告和受管命令归属检查只有独立 Markdown 样例，没有项目唯一 CLI 标本中的状态入口，无法确认逐字符输出、现有 doctor 报告前后文及状态覆盖；开发者必须自行选取一种显示形态。
- 证据：[Documentation Workflow](../../../documentation-workflow.md#the-specimen-requirement) 明确要求产品 surface 使用共享 Product Prototype 并引用相应 surface/state。[产品原型 README](../../../../prototype/README.md) 将 CLI 逐字符输出纳入唯一设计真相。当前候选没有任何原型引用；工作树变更仅为本主题八份 Markdown，原型未新增本主题状态；对 prototype/src、prototype/tools 的新错误码和检查名检索也没有命中。本轮新增的公共 install claim 更需要可评的完整状态标本。
- 💡 修复建议：在已有共享 CLI 原型中补齐本主题新增状态，引用其可访问的 surface/state 和明确内容身份，并说明每个标本证明什么；复用已有版式。不要另建第二份原型或把未来产品运行测试当成当前标本。
- 本项是设计产物要求，不要求本轮实现 Go 产品功能。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-UX-R1-F1 — CLOSED。** 第 25–26、121–150 行明确普通命令 exit 1、doctor 使用 partial 成功 envelope/exit 0；architecture 第 266–273 行把拒绝处理放在 newDoctorCommand 中，不进入 doctor.Service；任务 4 第 125、133 行承接这一顺序。原先“入口先退出却承诺仍有检查”的冲突已消除。

新增认领错误区分非当前 keg、另有拥有者和旧拥有者仍运行；不泄露对方路径。

#### 📝 总结

原发现 1 项关闭，新增 ISO-UX-R2-F1 未关闭，最终 surface 不能 PASS。尚不存在可纳入 ContentState 的共享标本身份；本状态记录仅包含候选文档，不冒充完整的原型评审证据。

Evidence: 当前候选内容和共享原型权威直接核对；复用同一 HEAD 的 newDoctorCommand / writeDoctorResult 和稳定 Doctor 契约，文档已给出将要修改的入口顺序。该复评没有执行未来的产品行为。L0 与 CEv1 结果在收口字段登记。

修复范围：ISO-UX-R2-F1；保持 ISO-UX-R1-F1 已关闭，并与架构的后续交接/访问边界保持一致。

## Repair 2 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-UX-R2-F1；同时同步架构 Repair 2 带来的文案变化。修改了 `ux/cli.md`，并在共享原型中新增本主题的状态。
- Beads：`ad-installed-state-ownership-doc-ux-cli` 由 claude-code 领取修复。

### ISO-UX-R2-F1 处置：已修复，待复评

**在现有的共享 CLI 原型中补齐本主题的状态。** 没有另建原型，复用了 `launchservices` 页签的版式：

- 新增 `prototype/src/ownership.js`，包含 13 个状态：未安装提示、三种普通拒绝、五种 doctor、四种认领。
- `prototype/src/Cli.jsx` 新增 `ownership` 页签和状态选择器，入口为 `?surface=cli&ownership=<state>`。每个状态分开展示 stderr、stdout、退出码和 `--format json`。
- `prototype/README.md` 新增"安装状态所有权标本（installed-state-ownership）"一节，写明入口、状态清单、证明什么、不证明什么。
- 新增结构检查 `prototype/tools/installed-state-ownership-check.mjs`，检查内容包括：状态集合、envelope 字段、JSON 中没有路径、退出码与 error 的对应关系、错误首行格式、doctor 头部和逐条检查行、`installation` 排在第一个、只有被拒绝的报告是部分报告、`managed_command_owner` 只在已安装程序中出现。

**`ux/cli.md` 不再复制逐字符输出。** 改为新增 [标本](../ux/cli.md#标本) 一节，列出 13 个状态各自的命令和证明内容，正文只保留语义：状态矩阵、字段、原因码、文案约束。这样避免文档和原型各存一份、互相漂移。

**修复中发现并纠正了自己的一处错误。** Repair 1 中 `ux/cli.md` 的 doctor 样例用的是自造的对齐列格式（`installation  ok  homebrew_cask`），与实际的 `renderDoctorText`（`cmd/agentdeck/main.go:5225`）不符。原型标本改为真实格式：

- 头部为 `status:`、`mode:`、`warnings:`、`errors:` 四行；
- 每个检查一行 `name: status (code)`；
- 附加行以两个空格缩进；
- 部分报告的结尾一行沿用 `checks_skipped:` 前缀，但文案改为适用于"目录被拒绝"的说法。

原 Markdown 样例已删除。

### 随架构修复同步

- `claim_owner_running` 的文案改为 "The current owner is still using AgentDeck state."，`next` 一行加上了"暂停 Codex 和 Claude 会话"，与所有权排他锁的语义一致。
- 原因码表新增 `state_path_unresolved` 和 `ownership_busy`；状态矩阵新增"取所有权共享锁超时"一行。
- `install register` 使用同一个错误码和同一组原因码。它是隐藏命令，没有单独的标本。

### 验证

- `node prototype/tools/installed-state-ownership-check.mjs`：13 个状态全部通过。阴性对照：在临时副本中把被拒绝报告的 `partial` 改为 `false`，检查失败（exit 1），并准确报出 `doctor_refused: only the refused doctor report is partial`。
- **原型编译**：worktree 中没有 `node_modules`，所以没有安装依赖，而是用主检出中已有的 esbuild 对 `src/Cli.jsx` 的整个导入图做打包检查，React 和图标库作为外部依赖，结果 exit 0，产物中包含 `data-cli-ownership`。esbuild 报告的 `i18n.js` 重复键 `tabQuestions` 在 HEAD 上已经存在，与本次修改无关。
- 本轮没有在浏览器中渲染截图，所以只声称了字符内容和结构，没有声称视觉验收。
- 文档结构检查与 [架构记录](architecture.md#repair-2--2026-11-05) 中 Repair 2 的验证相同，均通过。

### 候选

HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`，配方同 Round 1：

| 路径 | blob | 指纹 |
| --- | --- | --- |
| `docs/topics/installed-state-ownership/ux/cli.md` | `c3bb69c9e0ce2e2c0b1a1d8d406a5d7fe84d9425` | `7f631b521db4394c78cdda4daf96900d725af12fe13311f1efb394ccfd0f7644` |
| `prototype/src/ownership.js` | `2ec5dbbb6c8c8859cc9cdaeb3cab1e20a430dcd7` | `2d32a26a55fcae82bf375158c5f618ced3a2607d9fe579479b83207794e978c8` |
| `prototype/src/Cli.jsx` | `6e818b45e8510cd06ffedbba7320a7769de6a686` | `da2db0b9271ac0c607253c690d3aadefc7ee9b34f14259802eaa7d4549690088` |
| `prototype/README.md` | `57d2304799f980a0dbc22f5cdfe6351a7a249901` | `129350fed73e5ef3633972b2bae9867f64d9dba0b0f5a5435e98bd22b63216a2` |
| `prototype/tools/installed-state-ownership-check.mjs` | `235f4425321866714752c808a3a8ce4a90e89c54` | `4f0e946983aa89404e4954a7e8b09b20277699e92bf963c63bfada76a11c0d68` |

Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 3 — 2026-11-05

### 📋 CLI 标本复评报告

📊 总体评分：7/10

✅ 复评结论：FAIL

- Reviewer: Codex；独立复核 claude-code Repair 2，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `c3bb69c9e0ce2e2c0b1a1d8d406a5d7fe84d9425`；fingerprint `d04c4b9c1d57c21807e4413b6ccc1f39a704fa6ac21cafcc806e28f09da6e170`。
- WorkUnit: `installed-state-ownership:ux/cli.md`；Beads: `ad-installed-state-ownership-doc-ux-cli`。
- Target ContentState: `installed-state-ownership:ux-cli:rereview-r3-20261105-l2xNcW-state`。
- 完成门禁：FAILED
- Gate evidence: 本轮精确态查询为 FAILED；review 失败证据与 L0 通过证据均适用，历史状态未被覆盖；invalidated_evidence 和 unresolved_candidate_impacts 均为空。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0。前者含空白、diff、链接、文档集合和记录结构检查；结构通过不代表语义通过。
- Method / Scope: 复核 ISO-UX-R2-F1、13 个共享标本及其检查器；用 agent-browser 在隔离浏览器中检查实际渲染和状态切换，并核对现有 Doctor 契约。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Prototype manifest SHA-256: `b23015a23e68af272fa43d82c87ff198e8589c4267678ecc73abe0ca113412b3`。
- Manifest recipe: 对下表按路径排序，拼接 `path=blob\n`（包含末尾 LF），计算 SHA-256；其余导入图继承 HEAD。

| 路径 | Git blob |
| --- | --- |
| prototype/README.md | 57d2304799f980a0dbc22f5cdfe6351a7a249901 |
| prototype/package-lock.json | 9eda43590d2bb26f987e49e4f46ae9fad899b437 |
| prototype/package.json | dd5dc88efe11ab061ad70eee34ddc435af82f527 |
| prototype/src/Cli.jsx | 6e818b45e8510cd06ffedbba7320a7769de6a686 |
| prototype/src/ownership.js | 2ec5dbbb6c8c8859cc9cdaeb3cab1e20a430dcd7 |
| prototype/tools/installed-state-ownership-check.mjs | 235f4425321866714752c808a3a8ce4a90e89c54 |

#### 🔴 严重问题 — 必须修复

**ISO-UX-R3-F1 — P2 — NEW / OPEN：缺失状态目录的 doctor 标本错误地报告检查完整。**

- 位置：[ownership.js](../../../../prototype/src/ownership.js) 第 99–119 行；[标本检查器](../../../../prototype/tools/installed-state-ownership-check.mjs) 第 45–47 行；[ux/cli.md](../ux/cli.md#标本) 第 29–30、38 行。
- 行为风险：按这些“逐字一致”的标本实现后，首次运行/尚未认领且没有状态目录的 doctor 会隐藏其余诊断未执行这一事实，破坏既有 partial 与 checks_skipped 契约。
- 证据：doctor_not_installed 与 doctor_formula_unclaimed 都含 `state_missing`，但序列化结果为 `partial:false`、`warnings:[]`，文本也没有 `checks_skipped:`。浏览器实际显示了相同 JSON。当前 HEAD 的 `doctor.Service.Check` 在状态目录缺失时设置 `Partial=true` 后返回，`writeDoctorResult` 据此添加 warnings/checks_skipped；[稳定 Doctor 契约](../../../specs/cli-design.md#doctor) 明确规定 missing-state short circuit 是部分报告。检查器却把“只有 doctor_refused 才 partial”写成通过条件，所以它的 13 states ok 不能证明与产品契约一致。
- 💡 修复建议：两种缺失状态目录的标本均恢复 partial=true、checks_skipped warning 与适当的文本结尾；让检查器根据真正的短路原因断言 partial，而不是按“只有 refused”限制。保留已安装正常报告及被拒绝报告各自的语义。
- 该问题由本轮新增标本暴露，不能以“省略无关检查”解释：缺失目录时，这些检查是没有执行。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-UX-R2-F1 — CLOSED。** 共享 CliSurface 新增 ownership 页签和 13 状态选择器；文档改为引用唯一数据源，标本明确区分文本、JSON 与退出码。实际渲染检查了 doctor_not_installed、doctor_refused、claim_owner_running；两种 doctor 页截图已目视检查。先前缺少共享标本的问题已解决。ISO-UX-R1-F1 保持 CLOSED。

#### 📝 总结

原开放项关闭；新增 ISO-UX-R3-F1 未关闭，故 FAIL。本轮 UX 身份已包含原型 manifest，不再使用纯文档指纹冒充标本覆盖。

Evidence: `node prototype/tools/installed-state-ownership-check.mjs` 输出 `13 states ok` / exit 0；独立读取两个 state_missing 标本均得到 `partial:false,warnings:[],checks_skipped_text:false`。使用目标 worktree 的完整 main.jsx 导入图在临时目录打包并通过 loopback 提供页面；只读复用 canonical main 中现有依赖，两个 checkout 的 package-lock 相同，没有安装或更改依赖。未修改的 i18n.js 存在 duplicate tabQuestions 构建警告；打包仍 exit 0。浏览器与服务器在检查后关闭；临时输出不是正式引用。该证据证明渲染和契约差异，不证明未来 Go 运行行为。最终 L0 与门禁在收口字段登记。

修复范围：ISO-UX-R3-F1；不重开已关闭的原型入口和 doctor 拒绝路由发现。

## Repair 3 — 2026-11-05

- Implementer：claude-code。原文档作者，不是评审者，不自判 PASS。
- Scope：ISO-UX-R3-F1。修改 `prototype/src/ownership.js`、`prototype/tools/installed-state-ownership-check.mjs`、`prototype/README.md` 和 `ux/cli.md`。
- Beads：`ad-installed-state-ownership-doc-ux-cli` 由 claude-code 领取修复。

### ISO-UX-R3-F1 处置：已修复，待复评

确认属实：`doctor.Service.Check`（`internal/doctor/doctor.go:72-76`）在状态目录缺失时设置 `Partial=true` 后返回，`writeDoctorResult` 据此加上 `checks_skipped` 告警和结尾一行。Repair 2 中两个含 `state_missing` 的标本把它写成了完整报告；检查器又把"只有被拒绝的报告才是部分报告"写成了通过条件，所以没能发现这个错误。

**标本。** `doctor_not_installed` 和 `doctor_formula_unclaimed` 恢复为部分报告：

- JSON 为 `partial: true`、`warnings: ["checks_skipped"]`；
- 文本以现有的 `checks_skipped: remaining diagnostics require a readable core database` 结尾；
- 两个标本的 `proves` 说明都写明，这是沿用现有的缺失目录短路。

`doctor_refused` 保持原有的拒绝语义，`doctor_installed` 和 `doctor_command_owner` 保持完整报告。

**检查器。** 改为根据真正的短路原因断言，不再按状态名硬编码：

- `state` 检查的 code 为 `state_missing` 或 `state_ownership_refused` 时，必须是部分报告：带有 `checks_skipped` 告警，以对应的结尾一行结束，并且只包含 `installation` 和 `state` 两个检查；
- 其余报告必须是完整报告，文本中不能出现 `checks_skipped:`。

**文档。** [ux/cli.md](../ux/cli.md#doctor-的-installation-检查) 新增"两种短路"表，写明新增 `installation` 检查不改变缺失目录短路原有的部分报告语义；标本表中两行的说明也已更新。原型 README 中对检查器的描述同步修改。

### 验证

- `node prototype/tools/installed-state-ownership-check.mjs`：13 个状态全部通过。
- **阴性对照。** 在临时副本中把 `doctor_not_installed` 恢复成 Round 3 时的非部分报告，检查器报出两条失败：`partial must be true when the state check code is state_missing`，以及 checks_skipped 告警缺失，exit 1。临时副本已删除。
- 用主检出中已有的 esbuild 对 `src/Cli.jsx` 的导入图做打包检查，exit 0。
- 文档结构检查同 [架构记录](architecture.md#repair-3--2026-11-05) 中 Repair 3 的验证，都通过。
- 本轮没有在浏览器中重新渲染，只声称了字符内容和结构。

### 候选

按 Round 3 的配方：

- 原型 manifest：对下表按路径排序，拼接 `path=blob\n` 后取 SHA-256；
- 文档指纹：`head=<HEAD>;document=<blob>;prototype=<manifest>` 的 SHA-256。

用同一配方重算 Round 3 的值，得到 manifest `b23015a2…`、指纹 `d04c4b9c…`，与 Round 3 记录一致。

| 路径 | Git blob |
| --- | --- |
| prototype/README.md | 6f99e94fbca06d77ba5366e241f04cd5e2cf39b7 |
| prototype/package-lock.json | 9eda43590d2bb26f987e49e4f46ae9fad899b437 |
| prototype/package.json | dd5dc88efe11ab061ad70eee34ddc435af82f527 |
| prototype/src/Cli.jsx | 6e818b45e8510cd06ffedbba7320a7769de6a686 |
| prototype/src/ownership.js | b0d100100fc4caf9bf56dda156e6edd641d86fa4 |
| prototype/tools/installed-state-ownership-check.mjs | 4c7f9dbff8c2a9a36e0b1407d9bf89c12941ae0d |

- HEAD `660c81586f4c13e04e66dbc05cde5331bf09acec`
- Prototype manifest SHA-256：`0301083cf12950bc868a91533819b85834b35b62d943477b8f7a870515b3b65f`
- `ux/cli.md` blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a`；指纹 `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`

Review 保持未勾选；没有新增 CEv1 通过证据，也没有提交。

## Round 4 — 2026-11-05

### 📋 CLI 标本复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；复核 claude-code 的 Repair 3，未委派，未修复设计、产品代码或标本。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a`；fingerprint `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`。
- WorkUnit: `installed-state-ownership:ux/cli.md`；Beads: `ad-installed-state-ownership-doc-ux-cli`。
- Target ContentState: `installed-state-ownership:ux-cli:rereview-r4-20261105-QC3iyb-state`。
- 完成门禁：VERIFIED
- Gate evidence: 本轮固定 gate-status 对精确目标返回 VERIFIED；本轮 review 与 L0 证据均适用，无失效证据或未决影响。历史内容状态和结论保留。
- L0 results: `python3 scripts/check-docs.py` 与 `bash scripts/check-topic-docs.sh installed-state-ownership` 均 exit 0；前者覆盖空白、diff、链接、文档集合和记录结构。
- Method / Scope: ISO-UX-R3-F1、两种缺失目录标本、检查器反例和文档说明；复用未变化的 CliSurface 渲染布局事实。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Prototype manifest SHA-256: `0301083cf12950bc868a91533819b85834b35b62d943477b8f7a870515b3b65f`。
- Manifest recipe: 下表按路径排序，拼接 `path=blob\n`（末尾 LF 保留）后取 SHA-256；其余导入图继承 HEAD。

| 路径 | Git blob |
| --- | --- |
| prototype/README.md | 6f99e94fbca06d77ba5366e241f04cd5e2cf39b7 |
| prototype/package-lock.json | 9eda43590d2bb26f987e49e4f46ae9fad899b437 |
| prototype/package.json | dd5dc88efe11ab061ad70eee34ddc435af82f527 |
| prototype/src/Cli.jsx | 6e818b45e8510cd06ffedbba7320a7769de6a686 |
| prototype/src/ownership.js | b0d100100fc4caf9bf56dda156e6edd641d86fa4 |
| prototype/tools/installed-state-ownership-check.mjs | 4c7f9dbff8c2a9a36e0b1407d9bf89c12941ae0d |

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

**ISO-UX-R3-F1 — CLOSED。** doctor_not_installed 和 doctor_formula_unclaimed 均恢复 `partial:true`、`warnings:["checks_skipped"]`，文本结尾为现有的 `checks_skipped: remaining diagnostics require a readable core database`。检查器从 state.code 判断短路，分别核对 missing/refused 的结尾和检查集合，不再把“只有 refused 才 partial”当作约束。

独立负向检查将 doctor_not_installed 的 JSON 在内存中改回旧错误 `partial:false,warnings:[]`，检查器以 exit 1 拒绝，并同时报告 partial 与 checks_skipped 两条预期诊断。候选源文件未改动。ISO-UX-R1-F1、ISO-UX-R2-F1 继续 CLOSED。

#### 📝 总结

本文件全部已记录发现关闭，无新增发现，PASS。文档的“两种短路”表、数据、检查器和同一 HEAD 下的 Doctor 契约一致。架构的自动续登和替换仍有独立问题；本通过不证明未来运行时行为。

Evidence: `node prototype/tools/installed-state-ownership-check.mjs` 为 `13 states ok` / exit 0；独立断言确认两个缺失目录标本的 partial、warning、文本 trailer 全部正确；回退旧错误的内存负向对照 exit 1 且诊断匹配。Cli.jsx、package.json、package-lock 与 Round 3 的 blob 均相同；既有状态选择、DOM 渲染和布局事实继续适用。变化只在标本文字、字段和检查器，本轮用实际模块运行结果核验它们，没有声称重新截图，也没有为未变化布局重复启动浏览器。最终 L0 与 CEv1 结果在收口字段登记。

Task checkpoint：ad-installed-state-ownership-doc-ux-cli；content_state `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`；gate VERIFIED。
提交建议：经明确授权交付 ux/cli.md、共享原型相关四个变更文件、该评审记录及必要状态片段；也可待剩余设计修复后按授权成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式推送授权及对应提交对象、签名、范围检查。本轮未提交或推送。

## Round 5 — 2026-11-05

### 📋 CLI 契约复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；复核 claude-code Repair 4，未委派，未修复候选内容。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a`；fingerprint `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`。
- WorkUnit: `installed-state-ownership:ux/cli.md`；Beads: `ad-installed-state-ownership-doc-ux-cli`。
- Target ContentState: `installed-state-ownership:ux-cli:rereview-r4-20261105-QC3iyb-state`。
- 完成门禁：VERIFIED
- Method / Scope: 核对 CLI 文档、六项原型 manifest 输入和既有 13 状态语义是否变化；复用 Round 4 的通过结果。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Prototype manifest SHA-256: `0301083cf12950bc868a91533819b85834b35b62d943477b8f7a870515b3b65f`，配方和完整文件表同 Round 4。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-UX-R1-F1、ISO-UX-R2-F1、ISO-UX-R3-F1 保持 CLOSED。文档与全部原型 manifest 输入的 blob 均未变化，缺失目录的 partial/checks_skipped 修复和正负检查仍适用。

#### 📝 总结

Evidence: 精确态查询继续返回 VERIFIED，复用 Round 4 的 review/L0 证据，missing、invalidated、unresolved 均为空。本轮没有重复标本测试或浏览器渲染。此通过只覆盖已定义的 CLI 契约；不能把架构复用现有错误码当成新预留/恢复语义已经获批。预留方案尚未通过，修复若改变用户可见语义，应同步本文件并评审受影响内容。

Task checkpoint：ad-installed-state-ownership-doc-ux-cli；content_state `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`；gate VERIFIED。
提交建议：经明确授权交付 CLI 文档、相关共享原型变更、评审记录及必要状态片段；也可等待其余设计成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式授权及对应提交对象、签名和范围检查。本轮未提交或推送。

## Round 6 — 2026-11-05

### 📋 CLI 契约复评与证据复用

📊 总体评分：9/10

✅ 复评结论：PASS

- Reviewer: Codex；独立复核 claude-code Repair 5，未委派，未修复设计、产品代码或测试。
- HEAD: `660c81586f4c13e04e66dbc05cde5331bf09acec`。
- Workspace: `agent-deck.installed-state-ownership` / `feature/installed-state-ownership`。
- Reviewed state: document blob `5e427a1189ef21c7d1e5b150e27ab04f6221f20a`；fingerprint `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`。
- WorkUnit: `installed-state-ownership:ux/cli.md`；Beads: `ad-installed-state-ownership-doc-ux-cli`。
- Target ContentState: `installed-state-ownership:ux-cli:rereview-r4-20261105-QC3iyb-state`。
- 完成门禁：VERIFIED
- Method / Scope: 核对 CLI 文档及全部原型 manifest 输入未变化，评估隐藏安装命令实现方案对既定 CLI 表面的影响。
- Identity recipe: SHA-256 of `head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`。
- Prototype manifest SHA-256: `0301083cf12950bc868a91533819b85834b35b62d943477b8f7a870515b3b65f`；配方及文件表同 Round 4。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 改进建议 — 推荐

无。

#### 🟢 做得好的部分

ISO-UX-R1-F1、ISO-UX-R2-F1、ISO-UX-R3-F1 保持 CLOSED。文档和六项 manifest 输入的 blob 与 Round 4 相同；两个 doctor 短路与共享标本的语义未变。

#### 📝 总结

Evidence: 当前同一 ContentState 查询仍为 VERIFIED，复用 Round 4 的 review/L0 证据；无 missing、invalidated 或 unresolved 项。本轮未重复未变化标本的测试和浏览器渲染。此结论只覆盖已定义的 CLI 契约，不表示新的 cask 安装/卸载机制已运行通过。

Task checkpoint：ad-installed-state-ownership-doc-ux-cli；content_state `17fce06c3c357d72a244ff30bb571a5d2b8340fd90c87b32828755e5a3af0b62`；gate VERIFIED。
提交建议：经明确授权交付 CLI 文档、相关共享原型变更、评审记录及必要状态片段，也可等待其余设计成批交付。
推送建议：目标 origin/feature/installed-state-ownership；需显式授权及对应提交对象、签名和范围检查。本轮未提交或推送。
