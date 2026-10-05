---
status: historical
created: 2026-10-02
updated: 2026-10-03
retired: 2026-10-03
---

# 缺陷：分发测试退出后留下隔离 fixture 的 LaunchServices 注册

## 现象

Bug：`ad-bug-disttest-fixture-registrations-linger`。既有标识隔离修复后，
分发测试仍会删除 fixture 文件而保留注册；历史实测每次新增 7 条 `disttest`
记录。本批仅恢复本次 fixture 的生命周期，用户明确选择 Lane A。

基线为 `67a3248b6cf3e244ee7facef3597ad8b31ea8803`，工作区
`agent-deck.fix.disttest-fixture-registrations-linger`，分支
`fix/disttest-fixture-registrations-linger`。初始主 writer session 为
`01a0ffa3-b8f5-7c01-8b5a-d855b1a6f559`；其正常退出后，由新 session
`01a10042-437b-70a3-b50d-239e932f8d2b` 承接本批最终交付。

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

产品完成门禁：VERIFIED；归档候选门禁另见最后的主 writer finalization。

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
- **Location:** scripts/test-macos-distribution.sh:44（本地溯源：`/tmp/agentdeck-disttest-fifth/review-snapshot/scripts/test-macos-distribution.sh:44`；2026-10-05 观察 SHA-256 `c865c4023d36bab58e87aaebf8311afb445d6e741bfa26dac54d5380f67c46b2`，非追认历史哈希），关联删除分支第 78–79 行。
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

- **Regression protection:** 当前测试覆盖空格，但未覆盖反斜杠；scripts/test-macos-distribution-cleanup.py:135（本地溯源：`/tmp/agentdeck-disttest-fifth/review-snapshot/scripts/test-macos-distribution-cleanup.py:135`；2026-10-05 观察 SHA-256 `e7400e83a42917e4be3a158c2433e5eaf04637240b9c18db20ba43cba4f59926`，非追认历史哈希） 的 remaining-registration 用例因而未检出此缺陷。
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
| red-cleanup.log（本地溯源：`/tmp/agentdeck-disttest-fifth/red-cleanup.log`；2026-10-05 观察 SHA-256 `ded627ac496079647b0199c1dc03f4e950f0559d6b9af681de0ba9532e6c3409`，非追认历史哈希） | 4 个失败，均缺少预期反注册调用 |
| green-cleanup-final.log（本地溯源：`/tmp/agentdeck-disttest-fifth/green-cleanup-final.log`；2026-10-05 观察 SHA-256 `f79110a8c5e57e4a89014eeeb7f83f4c791e7383e7d2fca66931d1b4955be072`，非追认历史哈希） | 7 tests，`OK` |
| distribution.log（本地溯源：`/tmp/agentdeck-disttest-fifth/native/distribution.log`；2026-10-05 观察 SHA-256 `7cc203e0da03192b282a52a51d862c2612abc7bdb52580a667ac5de8c3dcac6e`，非追认历史哈希） | cleanup tests 和真实 distribution 子项通过；该次聚合随后因 Homebrew 临时目录防护失败 |
| cask-migration-final.log（本地溯源：`/tmp/agentdeck-disttest-fifth/native/cask-migration-final.log`；2026-10-05 观察 SHA-256 `96fb38c8926f305607daf3b43cf07286c0fdb4525c2f64f9df9d2c208de3ef77`，非追认历史哈希） | `cask migration and mutual exclusion: PASS` |
| registered-on-mount.log（本地溯源：`/tmp/agentdeck-disttest-fifth/native/registered-on-mount.log`；2026-10-05 观察 SHA-256 `f3441dd7696c250864c198c7db441bd2579b1e7de3626f6f5c793a23a30134d6`，非追认历史哈希） | 本次挂载 app 与 mount 路径的两条 registry 记录 |
| after-distribution.log（本地溯源：`/tmp/agentdeck-disttest-fifth/native/after-distribution.log`；2026-10-05 观察 SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`，非追认历史哈希） | 0 行，与提供的本次路径筛选结果一致 |
| format-final.log（本地溯源：`/tmp/agentdeck-disttest-fifth/format-final.log`；2026-10-05 观察 SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`，非追认历史哈希） | 空日志；单凭该文件无法独立读出检查命令及退出码 |
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

### 📋 Round 2 独立冷上下文复评报告

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
- **Location:** [scripts/test-macos-distribution.sh:44](https://github.com/kitdine/agent-deck/blob/391c9f15b38f05b69c23c97bd1346f4c752e207e/scripts/test-macos-distribution.sh#L44)（原本地位置：`/tmp/agentdeck-disttest-fifth/rereview-snapshot/scripts/test-macos-distribution.sh:44`）；回归用例位于 [scripts/test-macos-distribution-cleanup.py:148](https://github.com/kitdine/agent-deck/blob/391c9f15b38f05b69c23c97bd1346f4c752e207e/scripts/test-macos-distribution-cleanup.py#L148)（原本地位置：`/tmp/agentdeck-disttest-fifth/rereview-snapshot/scripts/test-macos-distribution-cleanup.py:148`）。
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
- 产品 merge-bound CEv1 为 VERIFIED 5/5。`DFR-R1-F1 CLOSED`，产品修复无开放 findings。
  旧候选的失效观察和 FAIL 保留为历史；没有 unresolved current impact。
- 本记录在上述实际 merge 和门禁完成之后移到 `docs/archive/fixes/`，保留两轮独立
  review 和原始 finding 历史；无 archive-index 条目。该退休仅用独立文档 PR，
  不直接 push release，不改产品源码，不删除任何 branch/worktree。

交付元数据偏差：产品 GitHub merge message 的 subject 为默认 merge 形式，且遗漏
`Co-Authored-By: Codex <noreply@openai.com>`。这不是签名或产品 tree 的失败；源逻辑提交
具备规定的 Conventional Commit subject、完整 body、trailer 和 SSH 签名。
按用户禁止改史的约束保留实际 merge，不声称所有 commit message 都满足规则。
后续退休逻辑提交及归档 merge **必须**满足完整 subject/body/trailer 要求。
归档 merge 尚未发生；是否符合要求须在实际 merge 后按 commit 对象核验，
核验结果及上述产品 merge 偏差记录在最终交接中。

归档文档 PR 的实际交付和最终任务状态由本 scope 的 CEv1、Beads 及最终回执记录；
不在该文件中制造自身未来 commit/merge 身份。`v0.6.5` contract/assemble 仍未完成，
原生菜单栏图标仍 user-deferred，其他三个候选 Bug 均不在本批。

## Review — Round 3：归档文档审查

## 📋 PR #37 exact-head 归档记录审查

📊 总体评分：8/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

`DFR-ARCH-R1-F1`，P2，`docs/archive/fixes/disttest-fixture-registrations-linger.md:302`。
旧陈述把未来归档 merge 的完整 message 当作已发生事实；在该 reviewed head 上
归档 PR 尚未合并，无法核验，且与下一段不制造未来 merge 身份的承诺冲突。

Evidence：GitHub thread `PRRT_kwDOTe7lus6oj8tW` / discussion `4171695831`，
review `PRR_kwDOTe7lus8AAAABQc4t7w` 明确 reviewed commit `1d3fc3f715`。
主 writer 已实际读回 thread、review 和本地源码，确认发现成立。

💡 修复：改为明确未来要求，标明尚未发生并等待实际 merge 后核验。
Disposition：已在待审查候选中完成文案修复；CLOSED 等新 exact-head 独立审查确认，
不因 writer 修复自行宣布 review PASS。

### 🟡 建议改进 — 推荐

无其他已核验 findings。

### 🟢 优点

产品 merge 的签名、tree、refs 和元数据偏差有实际证据；未为修正文案改写历史。

### 📝 总结

Reviewed state：`1d3fc3f715fa3cd845c8555fc11b50e49f19a93f`，tree
`0d04621490dfd9d52fb6f6610fd009a0fdfe8802`。
Reviewer：GitHub Codex；Method：exact-head 只读审查，主 writer 直接来源核验。
Scope：归档 carrier 的交付陈述；产品代码、测试及 Makefile 无变更。
Completion gate: NOT_VERIFIED

本候选只修复未来时态并追加本 finding 历史。归档 head 变化后必须重新绑定 CEv1、
请求 exact-head Codex review 并通过该 head 的 CI，之后才可执行归档 merge。

## 归档反馈与复评候选 — 2026-10-03

以下为 Round 4 产生之前的修复历史；当时的 active/pending 状态不代表最终状态，
当前 disposition 与门禁见后续 Round 4 和主 writer finalization。

`DFR-ARCH-R2-F1`，P2，GitHub thread `PRRT_kwDOTe7lus6okFbq` / discussion
`4171750733`，review `PRR_kwDOTe7lus8AAAABQc8x_A` 明确 reviewed commit `da102021c2`。
主 writer 实际读回 thread 与源码，确认：若直接退休该 head，最新适用 Round 3 仍 FAIL，
`DFR-ARCH-R1-F1` 等待 CLOSED，确实会 strand finding。原第一条 thread 已 outdated，
但尚 unresolved；outdated 不等于 CLOSED。

为修复该归档生命周期缺口，本 carrier **已暂时恢复 active 并返回 docs/fixes/**，
供全新空上下文只读冷复评。产品 PR #36 已交付的事实、两轮产品 review 与 Round 3 FAIL
保持历史原样；没有修改产品。旧未来 merge 陈述已变成明确要求，但两项 finding 的最终
CLOSED 与后续 PASS 必须来自独立复评，不由 writer 自发宣布。

本候选的退休条件：实际独立复评通过，主 writer 核验原始报告并把合法后续 round 与
两项 disposition 写回该 carrier，相关 CEv1 绑定完成后，才再次移入 docs/archive/fixes。
归档 PR #37 的实际 merge 仍未发生；新的交付 head 仍须通过 exact-head GitHub review/CI。


## Review — Round 4：归档 carrier 独立文档复评 — 2026-10-03

## 📋 第五批唯一 Bug 归档与 closure 准备复评报告

📊 总体评分：9/10

✅ Verdict: PASS

两项原 P2 finding 均在本候选中关闭；未发现新增范围内 finding。此结论仅覆盖候选文档修复，不宣告归档交付、GitHub thread resolution 或任务完成。

### 🔴 严重问题 — 必须修复

无。原两项 finding 的逐项 disposition 见下文。

### 🟡 建议改进 — 推荐

无新增 finding。后续报告回写、证据绑定和交付核验属于尚待执行的 finalization 条件。

### 🟢 优点

**DFR-ARCH-R1-F1 — CLOSED**

- 原始反馈：thread `PRRT_kwDOTe7lus6oj8tW`，discussion `4171695831`；原 review 明确指向 `1d3fc3f715`。
- 实际源码：carrier 第 298 行起（本地溯源：`/tmp/agentdeck-disttest-fifth/archive-cold-snapshot/docs/fixes/disttest-fixture-registrations-linger.md:298`；原 snapshot blob `2cfaa14f5321b3ee5dd8d57c03fc64e2aced8a5a`；2026-10-05 观察 SHA-256 `47ef5463be8c19e233dfd2371864fd8cb4bcb0029b523684dec1df343720b775`，非追认历史哈希）保留产品 merge message 的真实偏差，并在第 302–304 行写明后续归档提交及 merge **必须**满足要求、归档 merge **尚未发生**、须在实际 merge 后核验。
- 第 348–349、364–366 行把 CEv1、后续审查及新 head 的 GitHub review/CI 写为要求，没有宣告这些未来步骤已通过。
- 原缺陷是把未来 merge message 合规性写成事实；当前文字已消除该错误。关闭依据是候选源码，而非 thread 的 outdated 状态。

**DFR-ARCH-R2-F1 — CLOSED**

- 原始反馈：thread `PRRT_kwDOTe7lus6okFbq`，discussion `4171750733`；原 review 明确指向 `da102021c2`，要求“完成并记录复评，或先保持 carrier active”。
- 实际文件位于 `docs/fixes/disttest-fixture-registrations-linger.md`，frontmatter 为 `status: active`；旧归档路径不存在，`retired:` 已移除。
- 第 310 行起（本地溯源：`/tmp/agentdeck-disttest-fifth/archive-cold-snapshot/docs/fixes/disttest-fixture-registrations-linger.md:310`；原 snapshot blob `2cfaa14f5321b3ee5dd8d57c03fc64e2aced8a5a`；2026-10-05 观察 SHA-256 `47ef5463be8c19e233dfd2371864fd8cb4bcb0029b523684dec1df343720b775`，非追认历史哈希）保留 Round 3 `Verdict: FAIL` 和 F1 等待独立确认的历史，没有改写为历史 PASS。
- 第 359 行起（本地溯源：`/tmp/agentdeck-disttest-fifth/archive-cold-snapshot/docs/fixes/disttest-fixture-registrations-linger.md:359`；原 snapshot blob `2cfaa14f5321b3ee5dd8d57c03fc64e2aced8a5a`；2026-10-05 观察 SHA-256 `47ef5463be8c19e233dfd2371864fd8cb4bcb0029b523684dec1df343720b775`，非追认历史哈希）明确记录恢复 active，并规定真实独立复评、writer 核验、后续 round 与两项 disposition 回写、相关 CEv1 绑定完成后，才可再次退休。
- 因此，当前候选没有把待办 finding 留在 historical carrier 中。候选在复评前保留 FAIL 和待关闭状态是准确记录；本报告现在实际产生后，writer 才能记录新的 PASS。

文档第 294–296 行保留此前移入归档的历史，第 359–366 行记录随后恢复 active 的修复，二者构成时间顺序，不构成当前生命周期矛盾。

### 📝 总结

**Reviewed state**

| 项目 | 内容身份 |
|---|---|
| 复评基线 | `da102021c2f83458a069d09f1de1b9eb6eed5945` |
| immutable 候选 tree | `825738602e97d9cfdbe08f80016411ec5f9feefd` |
| 唯一候选文档 | `docs/fixes/disttest-fixture-registrations-linger.md` |
| document blob | `2cfaa14f5321b3ee5dd8d57c03fc64e2aced8a5a` |

**Reviewer 实际 session：** `01a10022-d965-72c3-a12c-76b12d79c779`，由本进程的当前 session/thread 环境标识读取。模型、推理与 tier 按本次任务指定为 `gpt-6.1-sol / xhigh / default`；未读取历史 session 核验配置。

**Method：** 单 reviewer，只读文档复评；直接读取候选、原始反馈、规定的生命周期和复评格式规则；以内存计算核对 Git blob、index/tree、diff 回退、commit 对象及保存的 GitHub 原始数据。未委派，未激活 workflow phase，未生成 token。

**Scope：** 仅本 Bug 的归档 carrier、两项反馈修复及 closure 准备。产品文件仅进行身份核对，没有重新审查产品行为或运行产品/native tests。

**Evidence**

| 实际核对 | 结果 |
|---|---|
| `archive-cold.index` 与 `archive-cold-tree.txt` | index 校验及 tree 重建匹配候选 tree；文档 blob 匹配 |
| `archive-cold.diff` 反向应用 | 匹配当前候选；重建基线 tree `3af15a52616629ddcdef100c9b85375da166c964`，与实际 `da102021…` commit 对象一致 |
| 保存的 product merge API 与 scoped 文件 | 三个产品/test/Makefile blob 均与 `41f43651…` 对应原始数据一致 |
| 将候选文档 blob 换回产品原始 blob | 重建 tree 为 `3b2c8e2d99014036a2548253ca88c5d12343ccc2`，与产品提交及 merge tree 相同，确认差异只在 carrier |
| 两个 archive 逻辑 commit 对象 | 对象哈希分别匹配 `1d3fc3f715…`、`da102021c2…`；subject、body、精确 Codex trailer 均存在；保存的签名日志为 Good |
| 产品 merge 对象与原始 API | 对象哈希匹配 `41f43651…`；parents/tree、verification payload 相符；签名文本归一化尾部空行后相符，原始 API 为 `verified: true / valid` |
| `product-review.json` | 完成 comment `5965299752` 明确 reviewed commit `391c9f15b3`；保存的 inline threads 为空 |
| 产品 CI 原始数据 | 两个 run 的四个 job ID 与 head `391c9f15…` 绑定；最终 jobs 均 completed/success。较早 check-runs 快照仍为 in_progress，未误当作最终成功数据 |

产品 blobs：

- Makefile：`36d68403abf01e61cc01bcab25444a5a70eb5758`
- distribution 脚本：`7b861b689dfecf9fc9d839f31453e4e1643f9c75`
- cleanup 测试：`61227a91ce33e35361bfb3225804a2fc479211b0`

**Completion gate: NOT_VERIFIED**

本 reviewer 未查询或写入 CEv1。文档中产品 merge-bound 的历史 VERIFIED 声明，不替代当前归档候选的完成门禁。

**全部 disposition：** `DFR-ARCH-R1-F1 CLOSED`；`DFR-ARCH-R2-F1 CLOSED`；still open：无；regressed：无；新 findings：无。保存的原始数据中，两项 GitHub thread 仍 unresolved；本报告关闭的是候选中的缺陷，没有修改远端 thread 状态。

**后续 artifact finalization 的精确条件：**

1. 主 writer 核验本实际报告与上述候选身份，追加合法 Round 4、PASS 和两项 CLOSED 依据；保留先前 FAIL、finding 历史及本独立报告的 `NOT_VERIFIED`。writer 后续实际门禁结果应另行记录。
2. 对包含真实报告及 dispositions 的内容完成相关 CEv1 绑定和要求的门禁，再执行退休。最终归档内容改为 historical、加入实际 retired 日期；路径及 blob 变化须纳入最终交付状态绑定。
3. 新交付 head 必须取得该 head 的 GitHub review/CI 结果，并处理两项 thread 的实际 closure；本报告不能代替后续 head 的审查结果。
4. 归档 merge 实际发生后，才核验其 commit 身份、tree、parents、签名及 subject/body/trailer，并记录真实结果。不得预写未来 merge 或未来合规 PASS。

**产品 M1 偏差保留：** 实际 `41f43651…` 使用默认 merge subject，遗漏 `Co-Authored-By: Codex <noreply@openai.com>`。该偏差已由对象和原始 API 确认，不能因逻辑提交合规或后续归档 merge 合规而抹除。

**Task checkpoint：** 本候选文档复评 PASS；任务完成边界仍未验证。提交建议仅覆盖 writer 最终化后的单一 carrier；推送建议以最终 CEv1、明确交付权限及 PR #37 的实际分支核验为前提。本文仅作为 stdout/last-message 报告，未执行任何写入或交付动作。

## 主 writer finalization — 2026-10-03

新 session `01a10042-437b-70a3-b50d-239e932f8d2b` 直接核验上述完整原始报告，
SHA-256 `43df9b7487499fcb5f7a853135aa61f403cb2cbcdaff3b1e6bbe8cc1f739c68b`；
原始 events 确认独立 session `01a10022` 实际完成，未修改 raw report。
核验范围包括候选 tree/blob、active 路径、两条 GitHub 原始反馈、产品未变 blobs，
以及产品 merge 的实际 parents/tree、PGP signature 与 verification payload。
原始核验回执：`/tmp/agentdeck-disttest-fifth/main-round4-verification.json`。

采用独立 Round 4 PASS 9/10。`DFR-ARCH-R1-F1 CLOSED`：源码将未来 merge
合规改为要求，并明确实际 merge 后核验；`DFR-ARCH-R2-F1 CLOSED`：复评时保持
carrier active，现在已把真实完整 PASS 和逐项 closure 写回，未留下归档中的待办 finding。
所有旧 FAIL、原待关闭状态及失效观察保留为历史；当前开放 finding 为 0。
独立报告自身的 `Completion gate: NOT_VERIFIED` 保持原样；主 writer 门禁独立记录。
归档 PR #37 尚未 merge，新的最终 head 必须通过 GitHub review/CI 才能交付。

主 writer 实际门禁：`fix:disttest-fixture-registrations-linger:state:archive-final-active-fb52b31f`
为 **VERIFIED 5/5**，missing criteria 与 unresolved candidate impacts 均为空；
原始 MCP 结果在 `/tmp/agentdeck-disttest-fifth/ce-archive-final-active.json`。
该门禁覆盖完整 Round 4 和两项 closure 的 active-final tree `fb52b31f`。
在此实际 PASS、closure 和门禁完成之后，本记录再次合法退休为 historical；
最终 archive 路径、blob、staged/committed tree 须另行目标绑定，PR #37 review/CI/merge
仍按后续实际结果核验。本段不把候选门禁当成未来归档交付完成。
