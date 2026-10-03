---
status: historical
created: 2026-10-02
retired: 2026-10-03
---

# 缺陷：分发测试退出后留下隔离 fixture 的 LaunchServices 注册

## 现象

Bug：`ad-bug-disttest-fixture-registrations-linger`。既有标识隔离修复后，
分发测试仍会删除 fixture 文件而保留注册；历史实测每次新增 7 条 `disttest`
记录。本批仅恢复本次 fixture 的生命周期，用户明确选择 Lane A。

基线为 `67a3248b6cf3e244ee7facef3597ad8b31ea8803`，工作区
`agent-deck.fix.disttest-fixture-registrations-linger`，分支
`fix/disttest-fixture-registrations-linger`。主 writer session 为
`01a0ffa3-b8f5-7c01-8b5a-d855b1a6f559`。

## 根因

`scripts/test-macos-distribution.sh` 的三个 EXIT trap 仅删除临时目录和
本次 Homebrew tap；section 6 在 detach 前也没有反注册挂载副本。删除后
`Info.plist` 已消失，注册记录却仍在。专用 bundle identifier 已保护出货 app，
但没有完成 fixture 自身的生命周期。

真实 macOS 判别实验另确认：对从未注册的副本，`lsregister -u` 返回
`kLSApplicationNotFoundErr` (`-10814`)；同一 fixture 显式注册后，注册和反注册
均 exit 0，精确路径记录随后消失。因此不能把未注册副本当作清理失败，也不能
仅凭错误码宣布成功。

## 修复边界

- 单一 EXIT 清理处理普通退出、失败和 INT/TERM，保留原失败退出码。
- 仅枚举本次物理临时目录内的 `.app` / `.appex`，不跟随符号链接，扩展先于 host；
  挂载副本在 detach 前反注册，失败退出时也尝试清理剩余副本和本次 tap。
- 仅当 `-10814` 且实时 registry 查询证明精确路径无记录时接受“已不存在”。
  枚举、查询、反注册或 detach 的其他失败不能证明清理完成；保留 fixture 并失败。
- 保持 fixture 专用标识、App Group、打包/签名/公证模拟、Cask 和出货行为契约。
  不增加 doctor 行为，不清理历史/用户注册，不重启 daemon，不安装出货 app。
- 新增隔离生命周期回归测试并接入 `make check-macos-distribution`。
  `AGENTDECK_LSREGISTER` 仅供该测试脚本注入记录 stub，默认使用系统工具。

## 验证

- RED：未改实现时 `python3 -B scripts/test-macos-distribution-cleanup.py`
  4/4 按预期失败，均因没有反注册调用。原始日志：
  `/tmp/agentdeck-disttest-fifth/red-cleanup.log`。
- 初始 GREEN：第一轮候选的 7/7 隔离测试通过；覆盖正常/失败退出、清理顺序、路径含空格、
  反注册和 detach 失败、不存在但仍有记录、registry 读取失败。
  日志：`/tmp/agentdeck-disttest-fifth/green-cleanup-final.log`。
- `bash -n scripts/test-macos-distribution.sh`、`make check-whitespace` 和
  `git diff --check` 通过。
- 原生验证中首次遇到未注册副本的 `-10814`，保留了本次路径；随后做精确路径
  注册/反注册实验，修正上述失败分类。原始日志与读回：
  `/tmp/agentdeck-disttest-fifth/native/explicit-*.log`。
- 最终真实隔离分发测试 `scripts/test-macos-distribution.sh` 通过；挂载后显式注册
  本次 fixture，使清理可观测。捕获到本次挂载路径的真实记录，结束后本次根目录
  精确路径记录为 **0**。首次保留的 fixture 也已用最终清理逻辑清除。
  日志：`/tmp/agentdeck-disttest-fifth/native/distribution.log`、
  `registered-on-mount.log`、`after-distribution.log`。
- `make check-macos-distribution` 的 Cask 子项首次因 harness 将 TMPDIR 改到
  `/private/tmp` 下而触发 Homebrew prefix 防护。源代码确认指定的 `brew-temp`
  尚不存在时 Homebrew 回退到默认 `/private/tmp`，因此这是 harness 条件问题。
  恢复 runtime 原有 TMPDIR，仅重跑未变的 `scripts/test-cask-migration.sh`，exit 0，
  `cask migration and mutual exclusion: PASS`。日志：
  `/tmp/agentdeck-disttest-fifth/native/cask-migration-final.log`。
  聚合的三个子项均有最终有效通过证据，没有为已通过部分重复运行。

验证选择为分发 fixture 范围的 L3：隔离生命周期测试、既有完整分发/Cask suite、
真实 Mach-O 构建/签名/DMG 挂载与注册生命周期、格式检查。Go 产品、依赖、并发和
wire 契约未改变，故不重复本地 Go 全量/race/vet/cross-build；交付仍要求 exact-head CI。

Completion gate: VERIFIED

WorkUnit：`fix:disttest-fixture-registrations-linger`，独立任务，无 containing topic。
独立冷复评已 PASS，产品 merge-bound 任务门禁 VERIFIED 5/5；产品 PR #36 已交付。
此记录仅在实际产品 merge 后退休，退休文档通过单独 PR 交付。
`v0.6.5` contract/assemble 未完成；原生菜单栏图标验收仍为 user-deferred。

## Review — Round 1

## 📋 独立冷上下文审查报告

📊 总体评分：8/10

✅ Verdict: **FAIL**

发现 **1 项 in-scope finding：DFR-R1-F1**。这是供主 writer 核验的独立审查结论，不替代其最终 verdict、记录或交付操作。

- **Reviewed state:** 候选 tree `77283435f831fa324d569ea56a14c8de33c52652`；基线 `67a3248b6cf3e244ee7facef3597ad8b31ea8803`。
- **Reviewer:** Codex，本次全新上下文独立只读 reviewer。
- **Method:** 审查规则读取、限定源码审查、scoped blob 校验、原始日志核对，以及直接提取候选中 AWK 程序的纯内存判别实验。发现决定性问题后停止广泛验证。
- **Scope:** 仅指定的 Makefile、两个 distribution 测试脚本、Lane A fix record 及必要受影响行为。其他 Bug、doctor、contract/assemble、deferred 菜单栏验收和历史用户注册均排除。
- **Completion gate: NOT_VERIFIED**。未查询或写入 CEv1，未操作 Beads、review records 或 Git delivery。

导出目录没有 `.git` 和 `.codegraph/`。tree 身份取自指定 `review-tree.txt`；本次直接计算的四个 blob 与 diff 的候选 index 前缀一致：

| 文件 | 候选 blob |
|---|---|
| Makefile | `36d68403abf01e61cc01bcab25444a5a70eb5758` |
| test-macos-distribution.sh | `dd139c67aec006271fe5d63c04668a1150e1f416` |
| test-macos-distribution-cleanup.py | `6c0111741639fa0185a29b1f97c750b2359d794d` |
| disttest-fixture-registrations-linger.md | `319b0e559f62c3c51ba8fcfc56f63cdf7dae5d9e` |

### 🔴 严重问题 — 必须修复

**DFR-R1-F1 — registry“精确路径”比较会解释路径中的反斜杠，可能错误证明注册已不存在。**

- **Severity:** P2／中。
- **Location:** [scripts/test-macos-distribution.sh:44](/tmp/agentdeck-disttest-fifth/review-snapshot/scripts/test-macos-distribution.sh:44)，关联删除分支第 78–79 行。
- **Risk:** `awk -v fixture="$fixture"` 会解释变量赋值中的反斜杠转义。例如，路径中的字面量 `\t` 被转换成制表符，registry 原始路径却保留反斜杠。因此，即使 dump 包含完全相同的实际路径，比较仍可能报告“无记录”。当反注册返回 `-10814` 时，该错误证明会让清理被接受，随后删除 fixture，留下本 Bug 要消除的注册残留。包含反斜杠的物理 `TMPDIR` 未被脚本禁止。
- **Evidence:** 本次原样提取候选的 AWK 程序，通过 `subprocess.run` 在内存执行；没有调用原生 `lsregister` 或写入 fixture。两个输入都包含待查路径的真实相同文本：

```text
control: exact_path_in_dump=True; awk_exit=1; helper_accepts_absent=False
literal-backslash: exact_path_in_dump=True; awk_exit=0; helper_accepts_absent=True
```

失败用例的路径为：

```text
/private/tmp/fixtures\test/agentdeck-macos-distribution.ABC123/AgentDeck.app
```

其 dump 输入为 `path: <上述路径> (0x123)`。普通含空格路径正确返回“存在”；字面反斜杠路径错误返回“缺席”。这直接证伪第 43–53 行的精确路径保证。

- **Regression protection:** 当前测试覆盖空格，但未覆盖反斜杠；[scripts/test-macos-distribution-cleanup.py:135](/tmp/agentdeck-disttest-fifth/review-snapshot/scripts/test-macos-distribution-cleanup.py:135) 的 remaining-registration 用例因而未检出此缺陷。
- **Bounded remediation:** 使待比较路径按字面值进入比较器，避免 AWK 赋值转义；增加一个“`-10814` 且 registry 仍包含字面反斜杠精确路径”的隔离回归用例，要求失败并保留 fixture。
- **Disposition:** **OPEN**。交主 writer 核验；本 reviewer 未创建或更新任何 carrier。

### 🟡 建议改进 — 推荐

无其他已证实的 in-scope findings。未将尚未动态验证的分支或个人实现偏好登记为问题。

### 🟢 优点

- 临时根目录先通过 `pwd -P` 固定为物理路径；`find -depth -type d -print0` 默认不跟随符号链接，并让嵌套 extension 先于 host 被处理。
- section 6 的正常路径明确先反注册挂载副本，再 detach。EXIT 路径记录清理失败、继续尝试其余清理，并在失败时保留临时根目录。
- 原运行失败码优先保留；成功运行发生清理失败时转为非零。INT／TERM 的源码处理明确为 130／143。
- Homebrew 的已存在目标检查发生在 `fixture_root` 归属赋值之前；正常删除后清空该变量，避免拒绝路径被 EXIT 当作本次 tap 删除。
- 枚举失败、非可接受反注册错误、dump 失败和 detach 失败均进入失败路径。`pipefail` 防止 dump 查询失败被 AWK 的零退出码掩盖；但精确路径比较仍受 DFR-R1-F1 影响。
- 回归测试使用真实脚本 prologue／EXIT trap。RED 的四个“没有反注册调用”失败能够检出原始缺陷，且新测试已接入 Makefile 聚合入口。

### 📝 总结

**Evidence：**

| 原始证据 | 本次直接核对的结果 |
|---|---|
| [red-cleanup.log](/tmp/agentdeck-disttest-fifth/red-cleanup.log) | 4 个失败，均缺少预期反注册调用 |
| [green-cleanup-final.log](/tmp/agentdeck-disttest-fifth/green-cleanup-final.log) | 7 tests，`OK` |
| [distribution.log](/tmp/agentdeck-disttest-fifth/native/distribution.log) | cleanup tests 和真实 distribution 子项通过；该次聚合随后因 Homebrew 临时目录防护失败 |
| [cask-migration-final.log](/tmp/agentdeck-disttest-fifth/native/cask-migration-final.log) | `cask migration and mutual exclusion: PASS` |
| [registered-on-mount.log](/tmp/agentdeck-disttest-fifth/native/registered-on-mount.log) | 本次挂载 app 与 mount 路径的两条 registry 记录 |
| [after-distribution.log](/tmp/agentdeck-disttest-fifth/native/after-distribution.log) | 0 行，与提供的本次路径筛选结果一致 |
| [format-final.log](/tmp/agentdeck-disttest-fifth/format-final.log) | 空日志；单凭该文件无法独立读出检查命令及退出码 |
| 本次纯内存实验 | 候选 AWK 对实际存在的反斜杠路径错误返回“缺席” |

正常退出、原失败码保留及常见失败路径获得源码和既有测试支持；INT／TERM 的动态行为未在本次验证。挂载、tap 归属和枚举规则也未新增原生实验。这里没有把源码顺序等同于所有运行时窗口都已验证。

首次最小实验因 shell here-document 需要临时文件而被只读沙箱拒绝；随后改用纯内存参数执行成功，未提升权限或产生诊断文件。未重复全量、race、vet、build 或原生注册操作。

候选改善了主要生命周期路径，但 **DFR-R1-F1 使“registry 精确路径不存在”这一关键清理前提不可靠**，因此当前不能 PASS。主 writer 的待核验修复范围仅为该 finding；Completion gate 保持 **NOT_VERIFIED**。

Main writer 核验：新增精确回归用例在该候选上失败（原退出码为 0，期望非零），
证据 `/tmp/agentdeck-disttest-fifth/red-DFR-R1-F1.log`。因此采用上述 FAIL；
`DFR-R1-F1` 为 OPEN，修复后必须独立复评。Reviewer session：
`01a0ffc6-84df-7082-aa62-d576ee7b5253`，实际退出 0，CPA / gpt-6.1-sol / xhigh / default，
read-only / on-request；仅最后报告写入本批证据路径，无产品写入。

## 修复 — DFR-R1-F1

`DFR-R1-F1` 已在待复评候选中修复：通过临时环境变量将物理路径按字面值传入
`awk`，不再使用会解释反斜杠的 `-v`。新增“registry 仍含字面反斜杠路径”用例，
修复前 exit 0 的错误结果使该用例按预期 RED；修复后最终 **8/8** 通过。
直接证据：`/tmp/agentdeck-disttest-fifth/red-DFR-R1-F1.log` 和
`/tmp/agentdeck-disttest-fifth/green-DFR-R1-F1.log`。修复完成不自发声明 Review PASS；
该 finding 的最终 CLOSED disposition 等独立复评确认。

复用评估：本次代码变化仅改变路径进入比较器的方式；此前真实 fixture 路径没有
反斜杠，两种传入方式在这些输入上相同，反注册、枚举、签名、打包、detach、tap 和
Cask 实现均未改变。既有原生注册 0 残留、真实打包和 Cask 证据仍有效，遵用户指令
不重跑。旧 cleanup 证据不能覆盖该反斜杠缺陷，使用本次 8-test 通过记录替换其贡献。
文档/格式证据随本候选刷新；CEv1 用明确 impact 与 target-bound roll-up 绑定复用，
不重标旧 observation。

## Review — Round 2

## 📋 Round 2 独立冷上下文复评报告

📊 总体评分：9/10

✅ Verdict: **PASS**

**DFR-R1-F1 已独立核验为 CLOSED；本轮无新增 in-scope findings。** 本报告供主 writer 核验，不替代其最终 verdict、工作流记录或交付决定。

- **Reviewed state:** tree `ffaa9e5e6f906bbd29a0bc88a59b3267deebfdcb`，取自指定 `rereview-tree.txt`；基线 `67a3248b6cf3e244ee7facef3597ad8b31ea8803`。本轮计算的四个 scoped blob 均与 `rereview.diff` 候选 index 一致。
- **Reviewer:** Codex，本次全新冷上下文、独立只读 reviewer。
- **Method:** 读取指定审查规则，检查限定源码、基线 diff、repair diff 和原始日志；直接提取候选比较器及 EXIT 清理函数进行纯内存隔离实验。未采用原 reviewer 或主 writer 的结论作为证据。
- **Scope:** 唯一 Bug `ad-bug-disttest-fixture-registrations-linger`，Lane A；仅指定四个文件及必要生命周期行为。其他 Bug、doctor、contract/assemble、deferred 菜单栏验收、历史用户注册均排除。
- **Completion gate: VERIFIED**。主 writer 随后通过现有 Neo4j MCP 对本 reviewed state 查询为 5/5；独立 reviewer 本身未查询/写 CEv1、未操作 Beads、review records 或 Git delivery。

### 🔴 严重问题 — 必须修复

无未关闭的 in-scope findings。

**历史 finding DFR-R1-F1 复核：CLOSED**

- **Severity:** P2／中。
- **Location:** [scripts/test-macos-distribution.sh:44](/tmp/agentdeck-disttest-fifth/rereview-snapshot/scripts/test-macos-distribution.sh:44)；回归用例位于 [scripts/test-macos-distribution-cleanup.py:148](/tmp/agentdeck-disttest-fifth/rereview-snapshot/scripts/test-macos-distribution-cleanup.py:148)。
- **Risk:** 原 `awk -v` 会解释路径中的反斜杠，可能错误证明注册缺席，继而删除仍有注册的 fixture。
- **Evidence:** 候选通过环境变量传值，再由 `ENVIRON` 读取；本轮原样提取比较器验证如下。返回 `1` 表示路径存在，返回 `0` 表示缺席。

  | 输入 | 旧实现 | 新候选 | 正确结果 |
  |---|---:|---:|---:|
  | 普通含空格路径，registry 中存在 | 1 | 1 | 1 |
  | 字面反斜杠路径，registry 中存在 | 0 | 1 | 1 |
  | 字面反斜杠路径，registry 中缺席 | 0 | 0 | 0 |

- **Bounded remediation:** 已完成字面传值，并新增“`-10814` 且 registry 仍含反斜杠精确路径”的回归用例；该用例要求非零退出并保留临时目录。原始 RED 日志显示旧实现错误返回 `0`；最终 GREEN 日志显示 **8 tests，OK**。
- **Disposition:** **CLOSED**。未发现 still open 或 regressed；无替代 finding。

### 🟡 建议改进 — 推荐

无已证实的新增 in-scope findings。未将未执行的额外实验或实现偏好登记为问题。

### 🟢 优点

- **临时目录边界明确：** `pwd -P` 固定本次物理目录；`find -depth -type d -print0` 不跟随符号链接，并让嵌套 `.appex` 先于 host 处理。
- **挂载清理顺序正确：** 正常路径在 detach 前反注册挂载副本；EXIT 路径也先枚举、尝试反注册，再尝试 detach。清理错误导致非零结果并保留临时根目录，同时继续尝试剩余清理。
- **退出码保持正确：** 本轮提取真实清理函数、替换副作用命令后，正常退出为 `0`、原失败保持 `37`、INT 为 `130`、TERM 为 `143`；原运行成功时，反注册、detach 或删除失败均转为非零。
- **Homebrew tap 归属处理正确：** 已存在目标的拒绝发生在 `fixture_root` 赋值前；正常删除后清空归属变量，避免 EXIT 删除拒绝路径。
- **失败不能证明成功：** 枚举失败明确返回失败；非可接受反注册错误累计失败；`-10814` 只有在精确路径缺席时才接受；`pipefail` 防止 registry 查询失败被 AWK 结果掩盖。
- **回归保护能检出原 Bug：** 测试执行真实脚本 prologue／EXIT trap，断言反注册调用、嵌套顺序和 detach 前处理。原始四个 RED 均因缺少反注册调用失败；Makefile 已接入该测试。

### 📝 总结

**Evidence：**

| 原始证据／本轮检查 | 核对结果 |
|---|---|
| `red-cleanup.log` | 4 个预期失败，均缺少反注册调用 |
| `green-cleanup-final.log` | 7 tests，OK；作为此前生命周期证据 |
| `red-DFR-R1-F1.log` | 新用例因实际退出码 `0` 而失败 |
| `green-DFR-R1-F1.log` | 最终 8 tests，OK |
| `native/distribution.log` | distribution 子项 PASS；聚合随后因 Homebrew 临时目录防护失败 |
| `native/cask-migration-final.log` | Cask 子项最终 PASS |
| `native/registered-on-mount.log` | 捕获本次挂载 app 和 mount 路径记录 |
| `native/after-distribution.log` | 本次路径读回文件为 0 行；不据此断言整个 registry 无记录 |
| `format-repair.log` | whitespace 和 diff 检查均明确 `exit=0`；旧空格式日志未单独作为成功证明 |
| 本轮纯内存实验及 `bash -n` | 比较器、清理退出码检查符合预期；语法检查 exit 0 |

**原生证据复用评估成立，范围限于此前普通路径。** `repair.diff` 只改变路径进入比较器的方式及新增回归用例；此前实际路径没有反斜杠，本轮实验确认普通路径语义保持一致。反注册调用、枚举、打包、detach、tap 和 Cask 实现未随该修复改变，因此未重复原生打包。

信号实验使用隔离 stub，未验证真实挂载期间的异步信号窗口；反斜杠修复具有源码、纯内存实验及回归证据，但没有对应原生打包证据。上述边界未被扩大为运行时完成声明。

本候选关闭了 **DFR-R1-F1**，未发现其他 in-scope defect，独立复评结论为 **PASS**。独立报告形成时门禁尚未完成；主 writer 已随后完成本 reviewed state 的 5/5 VERIFIED 门禁。Git 交付边界仍待执行。

Main writer 核验：原始比较器输出与隔离退出码结果已直接核对；主工作树代码 blobs
与复评候选一致。采用本轮 PASS，`DFR-R1-F1 CLOSED`，无未关闭或新增 findings。
Reviewer session：`01a0ffd1-fac1-7a92-9bd2-7b2e04b18ba3`，实际退出 0；
持久化上下文确认 gpt-6.1-sol / xhigh / read-only / on-request，default tier。
原始独立报告及其未同步门禁字段保存在
`/tmp/agentdeck-disttest-fifth/cold-rereview-result.md`，本 carrier 的门禁字段由主 writer
按实际 MCP 结果收口。状态：
`fix:disttest-fixture-registrations-linger:state:rereview-ffaa9e5e`，5/5 VERIFIED；
旧候选 cleanup/scope 与 FAIL 观察属于历史失效记录，当前适用观察均通过、无 unresolved impact。

Task checkpoint：`fix:disttest-fixture-registrations-linger`，独立 task，无 containing topic。
提交建议：本 issue 的脚本、测试、Makefile 接线与 active fix record，单一签名逻辑提交。
推送建议：普通 push 到 `origin/fix/disttest-fixture-registrations-linger`，draft PR 指向
`release/v0.6.x`，exact-head Codex review/CI 通过后才 merge；仅 merge 后独立文档 PR 归档。
用户已授权上述动作。新候选只追加核验后的本轮报告与门禁状态，复用代码和原生证据；
实际提交前将当前 carrier 格式及 target-bound CEv1 再绑定到最终 staged tree。

## 交付与退休 — 2026-10-03

- 产品逻辑提交：`391c9f15b38f05b69c23c97bd1346f4c752e207e`，tree
  `3b2c8e2d99014036a2548253ca88c5d12343ccc2`。实际 subject/body/精确 Codex trailer
  已检查，SSH ED25519 签名验证通过；只含本 issue 的四个文件。
- 产品 PR：[#36](https://github.com/kitdine/agent-deck/pull/36)，实际 merge
  `41f43651cb5b436b7ff374fe46b39e21f92e2e0a`。parents 为 release 基线
  `67a3248b6cf3e244ee7facef3597ad8b31ea8803` 和上述产品提交；tree 与产品提交相同。
  实时 release/fix/PR head refs 与实际 Git 对象逐一匹配。GitHub PGP signature 及
  verification payload 与本地实际 commit object 匹配且 `verified: true / valid`。
- GitHub Codex：请求 `5965272302`，完成 `5965299752`，明确 reviewed commit
  `391c9f15b3`；无 inline threads/findings。两套 CI 的四项 check-runs 全部 success，
  对应该 exact head；`make verify` 的 full Go/race/vet 与 desktop build/tests/distribution
  均已在 CI 通过。证据：`product-review.json`、`product-ci-final.json` 和
  `product-merge-verification.json`，均在 `/tmp/agentdeck-disttest-fifth/`。
- Merge-bound CEv1 为 VERIFIED 5/5。`DFR-R1-F1 CLOSED`，当前没有开放 findings。
  旧候选的失效观察和 FAIL 保留为历史；没有 unresolved current impact。
- 本记录在上述实际 merge 和门禁完成之后移到 `docs/archive/fixes/`，保留两轮独立
  review 和原始 finding 历史；无 archive-index 条目。该退休仅用独立文档 PR，
  不直接 push release，不改产品源码，不删除任何 branch/worktree。

交付元数据偏差：产品 GitHub merge message 的 subject 为默认 merge 形式，且遗漏
`Co-Authored-By: Codex <noreply@openai.com>`。这不是签名或产品 tree 的失败；源逻辑提交
具备规定的 Conventional Commit subject、完整 body、trailer 和 SSH 签名。
按用户禁止改史的约束保留实际 merge，不声称所有 commit message 都满足规则。
后续退休逻辑提交及 merge 使用完整 subject/body/trailer，此偏差保留在最终交接中。

归档文档 PR 的实际交付和最终任务状态由本 scope 的 CEv1、Beads 及最终回执记录；
不在该文件中制造自身未来 commit/merge 身份。`v0.6.5` contract/assemble 仍未完成，
原生菜单栏图标仍 user-deferred，其他三个候选 Bug 均不在本批。
