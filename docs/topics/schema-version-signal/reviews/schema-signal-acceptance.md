---
status: active
topic: schema-version-signal
subject: schema-signal-acceptance
---

# Schema Signal Acceptance — Review

## Round 1 — 2026-09-08

## 📋 Task 5 评审报告

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SSA-R1-F1 — 中：专项验收测试破坏常规原生测试入口。**

位置：`apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift:448`。

- 行为风险：新加入普通 XCTest target 的 `testSchemaSignalNativeMatrix` 无条件
  `try XCTUnwrap(AGENTDECK_TEST_HOME)`，随后还要求特定目录前缀。
  未手动注入专项环境的常规 `make test-macos-app`/Xcode Test 会直接失败，
  不再能沿用 Task 4 的普通原生测试入口。
- 证据：Makefile:66–67 路由至 `scripts/test-macos-app.sh`；该脚本的 Xcode
  分支执行完整 scheme，没有隔离 home 注入或专项测试过滤。
  `AgentDeck.xcscheme` 的 AppTests target 为 skipped=NO，TestAction 继承
  LaunchAction 环境，但 scheme 未配置该变量。仓库内该变量的运行时读取仅在
  App 启动与本新增断言处；没有常规 runner 初始化它。
  因而在未设置变量的正常环境下，该新增断言的输入必为 nil。
  这是源码和完整入口配置确定的失败条件，不冒称已运行失败命令。
- 当前成功的 isolated native log 只证明专门准备环境后的单项验收运行通过，
  不能证明默认测试接入兼容。已有手工 UI 豁免不包括新增测试使标准套件失败。
- 💡 修复范围：让专项 fixture 的准备与测试发现/执行契约一致。可将专项验收
  明确设为 opt-in，缺少前置隔离环境时 skip 而不是失败，并保留显式隔离运行的
  真实断言；或在获授权的 runner/scheme 边界于 App 启动前准备隔离目录。
  不在测试体启动后才设置 home，不用真实用户状态满足前置条件，不把 skip 当成
  手工验收通过。补充安全的入口回归，区分普通入口与显式专项入口。
- 处置：OPEN；本轮仅评审，不修改代码、测试或配置。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

真实编译二进制覆盖 doctor quick/full、snapshot、持锁拒绝及两个 Hook 客户端，
检查 JSON、拒绝记录和路由数量，且使用合成 home/state。native matrix 明确区分
渲染/模型检查与实际辅助功能操作；交接没有将相同 PNG 宣称为文字放大生效。

### 📝 总结

- Reviewer：Codex；Method：独立于实现执行的当前源码、入口配置、测试与证据
  审阅；未委派，不声称完全冷上下文。未运行可能启动非隔离 host 的原生命令。
- Scope：Task 5 两个测试文件及其测试接入/验收记录，不重开 Tasks 1–4 的产品实现。
- Workspace：agent-deck.schema-version-signal，feature/schema-version-signal。
- Reviewed state：HEAD `488e787a41b96191cfd031bee5537a3ed4f6b189`；
  fingerprint `b52b94e946fd3fc3267316a12bef9e38bc6dcb6c354565a9ab39c123ab5c8a92`，
  现场按 head=<HEAD> 加两个排序的 ;path=blob 条目重算一致。
- Evidence：manifest `/private/tmp/agentdeck-schema-acceptance-evidence.json`；
  CLI 日志 `agentdeck-go-test.4AZD99` SHA-256
  `4106aece1b163e0064962a977c40d22142af4738caa17514d3f0bc6be4ddba06`；
  isolated native 日志 `/private/tmp/agentdeck-schema-acceptance-native-isolated.log`
  SHA-256 `8644b391c049ba8a124c325e1162bf7adfb6aad90d3732b113057cd946da4103`。
  摘要与 manifest 一致。已有专项通过事实保留；确认入口缺陷后不扩大验证。
- 用户决策：按 tasks.md 的 Task 5 user waiver 和 Beads handoff
  `01a08187-f7a6-756a-9b72-61560a4c9eed`，实际文字放大/窄布局判断、VoiceOver
  朗读/展开操作、notice-to-Health 点击导航已由用户明确豁免并接受风险。
  本轮不重新要求执行这些已豁免检查，也不将其写为实际通过。
- Completion gate：VERIFIED（两个实际证据复用项 + 两个 explicit_user_waiver 项）。
  固定 gate 查询的 WorkUnit 为
  `urn:ce:agent-deck:work-unit:schema-version-signal-schema-signal-acceptance`，
  target 为 `urn:ce:agent-deck:state:user-waiver:schema-signal-acceptance:YQcgzs1dU_fQB7wt`。
  两个 native waiver 记录明确 executed=false、waived_not_tested；其余两个
  required criteria 为 binary-cross-path/evidence-reuse。missing/invalidated/unresolved
  为空。该门禁没有标准 XCTest 入口兼容性的 criterion；它不推翻本次 REVIEW FAIL，
  也不意味着四项测试实际执行通过。保留既有观察，不伪造对已豁免项的失败执行。
- 限制：实际 VoiceOver/文字放大/窄布局/点击导航未验证；初始非隔离 host run
  仍从验收中排除。本轮未读取或修改用户数据库。修复后重新评审 SSA-R1-F1。
- 文档检查：本轮记录与 topic 状态同步后，make check-whitespace、
  bash scripts/check-topic-docs.sh、git diff --check 均通过。
  Beads 已退回 in_progress、round-1；main 和全局 docs/status.md 未修改。

### 下一步指令

修复：schema-version-signal / schema-signal-acceptance / SSA-R1-F1

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal

## SSA-R1-F1 修复记录 — 2026-09-08

- 修复者：Codex；SSA-R1-F1 -> repaired in candidate，等待复评；Round 1 FAIL 保留。
- 原因确认：普通 runner/scheme 不提供专项 Home，旧 XCTUnwrap 必定抛错。
  修复将前置条件集中为纯环境判定；未明确设置
  `AGENTDECK_TEST_SCHEMA_ACCEPTANCE=1` 或缺少有效隔离 Home 时抛 XCTSkip。
  合法显式环境继续执行原矩阵真实断言，不在 App 启动后设置 Home。
- 新入口回归覆盖空环境、仅 Home、仅开关、无效 Home、关闭开关及完整合法环境；
  验证缺失条件产生 XCTSkip 而非失败。环境字典用例不启动非隔离宿主。
- 普通入口：`make test-macos-app` 在 Xcode 26.4、英文测试语言和预先隔离的
  Home 下通过，专项开关明确 unset；日志确认矩阵 skipped，入口回归 passed。
  提供 Home 仅用于宿主安全，不用于隐式开启专项；未运行非隔离宿主作为反例。
  日志 `/private/tmp/agentdeck-ssa-r1-f1-standard.log`，SHA-256
  `7fd1e35d077c8b735f4afa2a7d08bac67b83ab411ce76354a33b1d83a3166edd`。
- 专项入口：同一隔离 Home，设置
  `TEST_RUNNER_AGENTDECK_TEST_SCHEMA_ACCEPTANCE=1`，使用
  `xcodebuild ... -only-testing:AgentDeckAppTests/SchemaSignalAcceptanceTests test`，
  2 tests / 0 failures，矩阵实际执行而非 skip。日志
  `/private/tmp/agentdeck-ssa-r1-f1-opt-in.log`，SHA-256
  `d57980a77d425fe10caeee3cfcfecea2fe7744d51a79ad6d46145ae376868c8e`。
  两次均使用 `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`、
  `TEST_RUNNER_AGENTDECK_TEST_LOCALE=en`，Home 为
  `/private/tmp/agentdeck-menubar-acceptance.Zxtd3b`，并在受控权限下运行 Xcode。
- 内容：HEAD `488e787a41b96191cfd031bee5537a3ed4f6b189`；沿用两个排序测试文件的
  head/path/blob 配方，新 fingerprint
  `a96751361c42c5ba9f9fbb048ec7b61db0b8c2e06d4b220af8e1263191fa4097`。
  仅修改 native 测试入口/回归，CLI 测试与生产内容不变，复用原 CLI 实测。
- 证据边界：新增 required criterion `native-entry-compatibility`；原两项人工
  豁免继续明确标为未实测，不因 skip 或矩阵执行转换成实际 VoiceOver/大字号通过。
  当前修复候选已追加新证据，不覆盖历史观察。固定 gate 为 VERIFIED（5/5，
  含保留的两项用户豁免）；6 个状态/证据节点、14 条关系回读一致，预检全部通过。
  状态 ID 为 `urn:ce:agent-deck:state:repair:schema-signal-acceptance:2hb5VL55p85M-Brc`。
  whitespace、topic-docs 和 diff 检查通过。未提交或推送。

### 修复交接

复评：schema-version-signal / schema-signal-acceptance

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal

## Round 2 — 2026-09-08

## 📋 Task 5 复评报告

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**SSA-R1-F1：CLOSED。** `MenuBarChromeTests.swift:447` 的纯环境判定先检查
AGENTDECK_TEST_SCHEMA_ACCEPTANCE=1，再检查隔离 Home；缺少前置条件抛 XCTSkip。
普通 suite 因发现专项测试而失败的路径已移除，显式合法入口仍执行原矩阵断言。
新增入口回归覆盖空环境、单项参数、关闭开关、无效 Home 和完整合法环境，
不为这些反例启动非隔离宿主，也不在 App 启动后才配置 Home。

### 📝 总结

- Reviewer：Codex；Method：与修复执行分开的复评角色，直接核对当前源码、
  入口回归、原始日志和 CEv1 血缘；未委派，不声称完全冷上下文。
- Scope：SSA-R1-F1 的原生测试接入和 Task 5 验收交接。唯一既有 finding 已关闭，
  未发现新增阻塞；既有用户豁免不被重开。
- Reviewed state：HEAD `488e787a41b96191cfd031bee5537a3ed4f6b189`；
  fingerprint `a96751361c42c5ba9f9fbb048ec7b61db0b8c2e06d4b220af8e1263191fa4097`。
  现场按两个排序测试文件的 head/path/blob 配方重算一致；状态文档排除。
- Evidence：现场核验 standard log SHA-256
  `7fd1e35d077c8b735f4afa2a7d08bac67b83ab411ce76354a33b1d83a3166edd`，
  opt-in log SHA-256 `d57980a77d425fe10caeee3cfcfecea2fe7744d51a79ad6d46145ae376868c8e`，
  均与修复证据相符。普通 canonical suite 在预先隔离 Home、专项开关未设置时
  TEST SUCCEEDED，入口回归 passed、矩阵 skipped；专项运行两个测试均 passed，
  矩阵实际执行而非 skip。安全 Home 只是宿主隔离条件，不是隐式开启验收。
  空环境行为由纯环境回归覆盖；未运行不安全的无隔离 App 宿主。
  复用未改变的 CLI/产品证据，没有因复评重跑测试。
- Completion gate：VERIFIED（5/5；三个有测试/复用证据的 criterion，两个用户豁免）。
  固定 gate-status.cypher 查询 WorkUnit
  `urn:ce:agent-deck:work-unit:schema-version-signal-schema-signal-acceptance`，
  target `urn:ce:agent-deck:state:repair:schema-signal-acceptance:2hb5VL55p85M-Brc`。
  native-entry-compatibility 有实际回归证据，binary-cross-path/evidence-reuse
  保留有效血缘，missing/invalidated/unresolved 均为空。
  native-rendering-matrix/native-accessibility-operation 明确沿用
  explicit_user_waiver_reused、waived_not_tested、executed=false。
- 限制：实际文字放大、窄布局判断、VoiceOver 朗读/展开操作、实际点击导航仍未
  实测，按已记录用户决定豁免；此 PASS 不将 skip、PNG 或模型驱动操作转换为
  那些实际验收通过。初始非隔离运行继续排除，原历史观察不覆盖或改写。
- 文档检查：本轮同步后 make check-whitespace、bash scripts/check-topic-docs.sh、
  git diff --check 均通过；Beads 已转为 awaiting_commit、round-2。不更新 main
  或全局 docs/status.md。

Task checkpoint：ad-svs-schema-signal-acceptance-dev；content_state=a96751361c42c5ba9f9fbb048ec7b61db0b8c2e06d4b220af8e1263191fa4097；gate=VERIFIED（含两项用户豁免）。

提交建议：授权后在当前 feature worktree 按 Task 5 提交两个测试文件、本评审记录和 tasks.md，保留豁免与未实测边界；不包含全局状态或 companion main 提交。

推送建议：提交对象、归属 trailer、SSH 签名及已提交内容证据核验后，再另行授权推送 feature/schema-version-signal 至确认的远端同名分支；不直接推送 main 或合并。

### 下一步指令

开发：schema-version-signal / contract-reconciliation

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal
