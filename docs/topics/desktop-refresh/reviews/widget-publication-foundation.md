---
status: active
topic: desktop-refresh
subject: widget-publication-foundation
---

# Widget Publication Foundation Review

## Round 1 — 2026-09-20

## 📋 Widget publication foundation 实现评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**WPF-R1-F1 — 高：bounded reader 没有验证 `lstat` 与 `fstat` 指向同一文件，path-replacement 竞态可绕过预检。**

位置：`apps/macos/AgentDeckShared/AppGroupSnapshotBytes.swift:16`–`:35`；
测试缺口位于 `apps/macos/AgentDeckTests/AppGroupSnapshotStoreTests.swift:263`。

- 行为风险：reader 在路径上执行 `lstat` 后，以 `O_NOFOLLOW` 打开路径并执行
  `fstat`，但只分别检查 regular/type/size，从未比较两份 `stat` 的 `st_dev` 与
  `st_ino`。路径若在两次系统调用之间被另一个进程替换为不同 regular file，
  新文件会被当成已经通过初始路径预检的对象继续读取。Task 1 明确要求覆盖
  path replacement；该实现不能证明读取对象就是 `lstat` 验证的对象，会让
  host semantic baseline 建立在竞态替换后的未绑定内容上。
- 证据：当前源码中 `pathStatus` 仅用于 `isRegular`/`st_size`，`openedStatus`
  也仅用于 type/size/reserve；两者没有任何 identity 比较。现有
  `testBoundedReaderRejectsSymlinkAndNPlusOneBytes` 只覆盖静态 symlink 和已存在的
  N+1 文件，没有在 `lstat` 与 `open` 之间替换路径，因此 15/15 聚焦测试通过
  不能验证这一要求。

💡 修复范围：在 bounded reader 中验证初始 `lstat` 与打开后 `fstat` 的
`st_dev`/`st_ino` 身份一致，不一致时返回固定 typed failure；增加确定性的
path-replacement 回归测试，证明替换对象不会被接受，同时保留 symlink、N/N+1、
file-growth 和 bounded-allocation 语义。不要用睡眠概率触发竞态。

Disposition：OPEN，交回同一任务
`ad-dr-widget-publication-foundation-dev` 修复。

### 🟡 建议改进 — 推荐

无。本轮已有决定性阻塞 finding，按项目规则停止扩大广泛验证。

### 🟢 优点

- `WidgetSnapshotPublisher` 将 admission barrier 与 verified generation 分离，
  并在可注入 suspension 后、rename 前重新检查 generation；既有确定性交错测试
  覆盖 newer publication 与 newer pre-commit failure supersede older write。
- store-owned unconditional reload 已移除；full/quota publication 进入同一个
  publisher，显式 CLI all-kind reload 仍保持独立。
- semantic projection 排除 generated/schedule 字段，覆盖五个 kind 的核心数据，
  unknown baseline、empty diff 与 post-commit indeterminate 均有聚焦保护。

### 📝 总结

- Reviewer：Codex 主代理；Method：代码与测试正式评审，使用 workspace-bound
  CodeGraph 定位发布调用路径后直接核对候选源码、Task 1 合同和聚焦 Swift 测试；
  与实现执行角色分开，未委派，也不声称完全冷上下文。
- Scope：Task 1 的八个 source/test/project/status 路径；生产代码、测试和配置
  保持只读。本轮未进入 Task 2 的 scheduler/coordinator 重构，也未评审 Task 3
  的 Widget typed loader。
- Reviewed state：workspace `agent-deck.desktop-refresh`，branch
  `feature/desktop-refresh`；HEAD
  `1a96354e86b3bec08ef7048c803844033403157f`；八路径有序
  `path<TAB>git-blob` manifest SHA-256
  `5399a8a52ab28eca79f58d0a6133209c3be48b6effa57246ca39791bc4566042`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:widget-publication-foundation:implement:5399a8a52ab28eca79f58d0a6133209c3be48b6effa57246ca39791bc4566042`。
- Evidence：`swift test --package-path apps/macos --filter AppGroupSnapshotStoreTests`
  在 SwiftPM nested sandbox 拒绝后以同一命令非沙箱执行，15 项测试、0 failure；
  build 仅有既有 `Info.plist` unhandled-resource 与无效 `await` warning。该套件
  没有 path-replacement 用例。本轮同步了 topic 自有 CodeGraph 索引并确认
  production full/quota 发布已路由到 publisher；显式 CLI reload 与 verification
  helper 的 direct store write 不构成 legacy runtime bypass。
- Review/status L0：`bash scripts/check-topic-docs.sh`、`make check-whitespace`
  与 `git diff --check` 均通过；whitespace 首次仅命中本轮 SwiftPM 测试生成的
  `.build` 派生文件，使用 SwiftPM clean 并删除其遗留的可重建
  `workspace-state.json` 后，同一检查通过。
- Completion gate：FAILED。`bounded-private-snapshot-io` 与
  `l3-publication-verification` 均被 WPF-R1-F1 的当前态证据否定；其余 criteria
  不在本轮伪造 PASS。Task 的 Review cell 保持未勾选，Dev 历史保持完成。
- Residual uncertainty：发现决定性高严重度问题后未运行全 macOS suite、native
  XCTest 或 Task 1 其余 L3 矩阵；这些应在修复达到最终相关内容状态后执行。

### 下一步指令

修复：desktop-refresh / reviews/widget-publication-foundation.md / WPF-R1-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — 2026-09-20

- `WPF-R1-F1 -> repaired in candidate.` `AppGroupSnapshotBytes.readBounded`
  now compares the path preflight's `st_dev`/`st_ino` with the opened
  descriptor's `fstat` identity before allocating or reading bytes. A mismatch
  returns the existing fixed `.unsafeFile` typed failure; the symlink,
  regular-file, N/N+1, bounded-allocation and post-read size checks are
  unchanged.
- The regression uses a synchronous `afterLstat` test seam to replace the path
  with a different regular-file inode immediately before `open`. Against the
  pre-fix identity logic it failed deterministically because no error was
  thrown; after the guard it passes without sleeps or probabilistic scheduling.
- Repair candidate：HEAD
  `1a96354e86b3bec08ef7048c803844033403157f`；Round 1 相同八路径有序
  `path<TAB>git-blob` manifest SHA-256
  `ddeb9454f57768f88bde3a75787adc83befbd51edb6a7987d1290159cd77c567`。
- Verification：focused red reproducer 1/1 failed before the identity guard and
  passed after it；`AppGroupSnapshotStoreTests` 16/16 PASS；`AgentDeckShared`
  and `AgentDeckWidget` Debug targets both built universal arm64/x86_64
  successfully.

Round 1 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`WPF-R1-F1` 是否 CLOSED 与新候选的 completion gate 由独立复评决定。

## Round 2 — 2026-09-20

## 📋 Widget publication foundation 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **WPF-R1-F1 → CLOSED。** `AppGroupSnapshotBytes.readBounded` 在分配和读取前
  比较 path preflight 与 opened descriptor 的 `st_dev`/`st_ino`；身份不一致
  返回固定 `.unsafeFile`，不再接受 `lstat`/`open` 窗口内替换出的 regular file。
- 新测试通过同步 `afterLstat` seam 在 `open` 前替换为不同 inode，不依赖睡眠或
  概率调度；修复交接保留了旧实现 1/1 RED 与修复后 GREEN 的对应证据。
- 相邻 bounded-read、atomic publication、generation superseding、semantic diff、
  unknown-baseline recovery 和 exact reload 行为在同一聚焦套件中保持通过。

### 📝 总结

- Finding disposition：`WPF-R1-F1` CLOSED；无 regressed、still-open 或新增
  finding。Round 1 的唯一 finding 已完整处置，因此本轮 PASS。
- Reviewer：Codex 主代理；Method：对修复候选做 finding-scoped 源码复评，
  核对 Round 1 风险、两文件变化、确定性回归测试和相邻 publisher/storage 套件；
  未修改生产代码、测试或配置，未委派。
- Scope：`AppGroupSnapshotBytes.swift` 的 identity guard、
  `AppGroupSnapshotStoreTests.swift` 的 path-replacement regression，以及相同
  Task 1 八路径内容态；未进入 Task 2/3 实现。
- Reviewed state：workspace `agent-deck.desktop-refresh`，branch
  `feature/desktop-refresh`；HEAD
  `1a96354e86b3bec08ef7048c803844033403157f`。修复候选代码态为
  `ddeb9454f57768f88bde3a75787adc83befbd51edb6a7987d1290159cd77c567`；
  Task Review 同步后的最终八路径有序 `path<TAB>git-blob` manifest SHA-256 为
  `5ce4ed818333d3e4b35d48ad29a37d48dd10fa07e60c5e1f48c8ae8874436533`。
- Evidence：`swift test --package-path apps/macos --filter AppGroupSnapshotStoreTests`
  在最终代码态执行 16 项、0 failure；新增
  `testBoundedReaderRejectsPathReplacementBetweenLstatAndOpen` PASS。修复交接中
  同一 `ddeb9454…` 代码态的 `AgentDeckShared` 与 `AgentDeckWidget` universal
  arm64/x86_64 Debug build 证据可复用；本轮没有因阶段变化重复构建。
- Completion gate：VERIFIED。五项 required criteria 均绑定最终 ContentState
  的适用 PASS evidence，missing/invalidated/unresolved 为空；Round 1 FAIL 仍只
  绑定旧 `5399a8a5…` ContentState，不污染修复态。
- Residual uncertainty：full macOS suite 与 native installed-Widget acceptance
  仍按任务分解由 Task 5 承担；本 PASS 不把这些后续边界声称为已完成。

Task checkpoint：`ad-dr-widget-publication-foundation-dev`；
content_state=`5ce4ed818333d3e4b35d48ad29a37d48dd10fa07e60c5e1f48c8ae8874436533`；
gate=`VERIFIED`。

提交建议：授权后按 Task 1 边界提交八个实现/source/test/project/status 路径及
本评审记录；排除其他 topic、全局 `docs/status.md` 与 `.codegraph`/`.build` 派生物。

推送建议：仅在提交对象的完整消息、Codex trailer、Task 范围、SSH 签名和分支
核验通过后，另行授权推送 `feature/desktop-refresh` 到确认的远端同名分支；
不推送 `main`、不合并。

### 下一步指令

开发：desktop-refresh / refresh-coordination-and-scheduling

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh
