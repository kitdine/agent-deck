---
status: historical
created: 2026-09-02
retired: 2026-09-02
---

# 缺陷：Cask 互斥守卫拒绝后留下已安装记录

## 现象

v0.5.0-rc.4 的真实 `brew install --cask` 验证中，机器已经安装 CLI-only
`agentdeck` formula 时，Cask preflight 打印了迁移提示并以失败状态退出，但
Homebrew 仍留下 `Caskroom/agentdeck-app`。因此后续查询把 Cask 视为已安装，
与提示所声明的“不要让 formula 与 Cask 共存”不一致。

旧版 `scripts/test-cask-migration.sh` 没有执行 Homebrew。它从渲染后的 Cask
读取冲突 formula 列表，再由自有的 `cask_install` 提前返回，因而无法观察
Homebrew 对 preflight 异常的清理行为。

聚焦回归测试在生产模板修改前稳定复现：

```text
bash scripts/test-cask-migration.sh

real brew refusal left an installed cask artifact:
.../isolated-brew/Caskroom/agentdeck-app
exit 1
```

## 根因

`packaging/homebrew/agentdeck-app.rb.tmpl:31` 原来调用 `odie`。Homebrew 6.0.21
中的 `odie` 通过 `exit 1` 抛出 `SystemExit`，而 Cask 安装器的 artifact 回滚
只捕获普通异常。该异常绕过回滚后，preflight 之前写入的 Caskroom receipt
没有被清理；错误文字与非零退出码都不能证明安装状态已经回滚。

## 修复边界

- 在 `packaging/homebrew/agentdeck-app.rb.tmpl:34` 将守卫改为抛出普通异常，
  保留既有触发条件、迁移文字、两个 formula channel 与用户状态边界。
- 在 `scripts/test-cask-migration.sh:153` 增加隔离的真实 Homebrew 路径：临时
  prefix、Cellar、Caskroom、tap、cache、HOME 与 appdir；只复用本机 Homebrew
  代码，不接触真实安装状态，不下载或发布 AgentDeck artifact。
- 回归同时断言命令失败、迁移提示存在、冲突 formula 保留，以及 Caskroom、
  app 和 `agentdeck` command 均不存在。既有本地 installer 继续覆盖两个
  formula channel、升级、卸载、completion links 与外部 command owner。
- 不修改 Cask 的用户可见契约、release workflow、版本、checksum、真实 tap、
  `~/.agentdeck` 或任何已安装内容。

## 验证

- RED：`bash scripts/test-cask-migration.sh`，旧模板失败，明确留下
  `isolated-brew/Caskroom/agentdeck-app`。
- GREEN：同一命令，`cask migration and mutual exclusion: PASS`。
- L3：
  `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer make check-macos-distribution`，
  `macOS distribution packaging: PASS` 且
  `cask migration and mutual exclusion: PASS`。
- Completion gate：`NOT_REQUIRED`。现行
  `.agent-instructions/evidence.md` 仅定义 document、task、topic、release 四种
  CEv1 边界；Lane A fix 没有 topic，也没有已定义的 WorkUnit 绑定。该流程缺口
  由 `ad-chore-cev1-lane-a-boundary` 承载，本修复不另造图谱状态。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `e040b33028f297f0edc754078e4362c6647a8a00` 加未提交工作区，
  blob:
  `a3377b33c769d982bbf2b9998949cdfd7484d4cc packaging/homebrew/agentdeck-app.rb.tmpl`、
  `f0fb7046926181ec409173f8d351db7b36b19943 scripts/test-cask-migration.sh`、
  `7e2afe640122d881a427a6b8a84a37c363da776d docs/fixes/cask-preflight-abort.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 仓库核对 + 实测。对着本机 Homebrew 6.0.21 的源码核对根因链
  （`cask/installer.rb` 的 `rescue => e` 与 `brew.rb` 的顶层异常呈现）；把新版
  `test-cask-migration.sh` 放进 HEAD 模板的 worktree 单独运行以独立复现 RED；
  在临时 worktree 里给测试加一行 dump 以查看真实 `brew` 的拒绝输出；运行
  `scripts/test-macos-distribution.sh`；比对测试前后真实 Caskroom、
  `/Applications` 与 taps。`docs/fixes/**` 无项目自带文档集校验器。
- Scope: `packaging/homebrew/agentdeck-app.rb.tmpl` 的守卫抛出方式、
  `scripts/test-cask-migration.sh` 新增的真实 Homebrew 路径，以及本记录四节。
  release workflow、版本、checksum、真实 tap 与已安装内容确未改动。
- Findings:
  - [P2] `R1-F1` 本次改写了文件顶部的 boundary，却留下一处指向它的交叉引用。
    `scripts/test-cask-migration.sh:53-57` 仍写着「whether Homebrew accepts the
    cask carrying it is `scripts/test-macos-distribution.sh`'s tap-backed load
    check, **per the boundary stated at the top of this file**」。新的顶部
    boundary 已不再提 `test-macos-distribution.sh`，也不再有那条分工陈述；顺着
    指针读过去的人，看到的是「本文件会驱动一次真实 `brew install --cask`」。
    这正是 desktop-app topic 已经付过一次代价的同一类缺陷——
    `docs/archive/topics/desktop-app/reviews/unified-desktop-distribution.md:243`
    的 Round 3 第一条 finding 就是针对这几行的悬空引用开的 P2，并为此走了一轮
    repair（此处只给位置，不复写那条 ID，以免它被当成本记录自己的 finding）。
    同一处还有第二半：`Makefile:143-146` 把 `check-macos-distribution` 需要
    Homebrew 的理由写成「the cask load check creates and removes a throwaway
    tap」，而现在多了一条更重的理由——本测试会跑真实 `brew install --cask`，
    且 `release-verify` 确实到达它（由 `test-macos-distribution.sh` 第 9 节断言
    并在本轮实测通过）。
    💡 边界内的两处注释修改：把交叉引用改写成新的分工陈述，并在 Makefile 注释
    里补上真实安装这条理由。-> open
- Evidence:
  - 根因链核对（本机 Homebrew 6.0.21）：`cask/installer.rb:323` 的
    `install_artifacts` 执行 preflight，其 `rescue => e`（即
    `rescue StandardError`）负责 artifact 回滚并在 `ensure` 中
    `purge_versioned_files`；`odie` 抛出的 `SystemExit` 不是 `StandardError`，
    因此绕过回滚，`stage` 阶段写下的 Caskroom 版本目录得以存活。记录所述机制
    属实。
  - RED 独立复现：把新版 `test-cask-migration.sh` 放进 HEAD（`odie` 模板）的
    worktree 运行，得到
    `real brew refusal left an installed cask artifact: …/isolated-brew/Caskroom/agentdeck-app`，
    与记录引用的失败一致。
  - GREEN：`bash scripts/test-cask-migration.sh` →
    `cask migration and mutual exclusion: PASS`（6.9 秒）。
  - 真实 `brew` 输出（临时 worktree 中加 dump 观察，修复后模板）：
    `Error: agentdeck/test/agentdeck-app: The CLI-only agentdeck formula is
    installed and already owns…` 后接两行迁移命令与 `~/.agentdeck` 说明，随后
    `==> Purging files for version 1.2.3 of Cask agentdeck-app`。呈现干净、无
    backtrace，与 `brew.rb:208` 的 `rescue RuntimeError … onoe e … exit 1`
    一致；回滚确实发生。
  - 隔离性实测：测试前后真实 `/usr/local/Caskroom` 目录列表逐项相同，
    `/Applications` 条目数不变，真实 taps 未新增。
  - `scripts/test-macos-distribution.sh`：`macOS distribution packaging: PASS`，
    其中包含 tap-backed cask 加载检查与「release-verify 是否到达这些脚本」的
    断言。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 发布路径中无 `brew audit`，因此 `raise` 不触及 cask 风格检查。
  - 记录引用的行号属实：模板 `raise` 在 `:34`，
    `real_brew_install_refusal` 定义在 `:153`。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A 边界，该流程缺口由 `ad-chore-cev1-lane-a-boundary` 承载。本记录未新建
  任何 CEv1 节点，与上一份 Lane A 修复的结论一致。
- Verdict: REOPEN

### 下一步指令

修复：fix / cask-preflight-abort / R1-F1

## Repair — Round 1 — 2026-09-02

- `R1-F1` closed：`scripts/test-cask-migration.sh` 的 formula parser 注释不再
  指向已改写的顶部 boundary；它现在明确区分 declaration-driven 本地检查、
  `real_brew_install_refusal` 执行的真实 `brew install --cask` 拒绝路径，以及
  `test-macos-distribution.sh` 独立负责的 throwaway-tap load 检查。
- `R1-F1` closed：Makefile 的 `check-macos-distribution` 注释现在同时说明两项
  Homebrew 前提：本地 prefix 中的 throwaway-tap load，以及完全临时 prefix 中的
  真实 Cask install/rollback。注释仍明确该 target 不触达 Apple 服务、已发布 tap
  或持久安装。
- Verification：本轮只修改注释与本修复记录，不改变 Cask 模板、测试逻辑或发布
  行为；`bash -n scripts/test-cask-migration.sh`、`make check-whitespace` 与
  `git diff --check` 均通过。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A WorkUnit；本轮不创建或修改 CEv1 图谱状态。
- Verdict: REOPEN —— `R1-F1` 已关闭，Repair 完成，等待独立 Re-review。

## Re-review — Round 2 — 2026-09-02

- Reviewed state: HEAD `e040b33028f297f0edc754078e4362c6647a8a00` 加未提交工作区，
  blob:
  `a3377b33c769d982bbf2b9998949cdfd7484d4cc packaging/homebrew/agentdeck-app.rb.tmpl`、
  `c308e12d9196e4775b3cb0214d5ae8252e19e615 scripts/test-cask-migration.sh`、
  `6a5f37adfae4a0d1427174bc950f48d3eff98c1c Makefile`、
  `337b0aa624232e45c3a935e69c12c9da02695e84 docs/fixes/cask-preflight-abort.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 核对 `R1-F1` 两个半边的处置文本是否自洽且属实；用 `git diff -U0`
  逐 hunk 确认本轮改动确实只落在注释上；核对 Makefile 新增声明的每一项事实；
  重跑 `bash -n` 与 `scripts/test-cask-migration.sh`；用 `make -n release-verify`
  确认聚合门可达性未变。Cask 模板 blob 与 Round 1 逐字相同，故复用该轮对真实
  `brew` 拒绝行为、回滚与错误文案的实测结果。
- Findings:
  - `R1-F1` closed，两个半边都补上了：
    `scripts/test-cask-migration.sh:55-59` 的注释不再写「per the boundary stated
    at the top of this file」，改为自洽地说清三方分工——本 parser 供
    declaration-driven 检查使用，`real_brew_install_refusal` 通过真实
    `brew install --cask` 走拒绝路径，`test-macos-distribution.sh` 另行负责
    throwaway-tap 的 load 检查。没有任何悬空指针。
    `Makefile:140-144` 改为「It requires Homebrew twice」，两条理由分别点名
    throwaway-tap load 与「runs `brew install --cask` inside a fully temporary
    prefix and verifies that a preflight refusal rolls back」，并保留
    「不触达 Apple 服务或已发布 tap、不留下安装」的声明。
    新声明逐条核对属实：两个脚本在 `brew` 缺失时都是硬失败而非跳过
    （`test-macos-distribution.sh:108`、`test-cask-migration.sh:179`）；
    「fully temporary prefix」与「leaves no installation behind」由 Round 1 的
    隔离性实测支持。
- Evidence:
  - 本轮为注释-only，机械核对而非声称：`git diff -U0` 的四个 hunk 中，
    `@@ -8,10 +8,6 @@` 与 `@@ -58,4 +54,4 @@` 全是注释，
    `@@ -156,0 +153,93 @@` 与 `@@ -202,0 +292,6 @@` 是 Round 1 已评审过的
    `real_brew_install_refusal` 及其调用点；Makefile 的非注释改动行数为 0。
  - Cask 模板 blob `a3377b33` 与 Round 1 相同，生产行为未变，故 Round 1 实测的
    真实 `brew` 拒绝输出（干净 `Error:` + 迁移文案 + `==> Purging files`）
    与隔离性结论继续成立。
  - `bash -n scripts/test-cask-migration.sh`：PASS；
    `bash scripts/test-cask-migration.sh`：`cask migration and mutual exclusion:
    PASS`。
  - `make -n release-verify` 仍到达 `check-widget-sandbox.sh`、
    `test-macos-distribution.sh`、`test-cask-migration.sh` 三者。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A 边界；本修复未创建或修改任何 CEv1 图谱状态。该契约缺口由
  `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: PASS

### Task checkpoint

Task checkpoint：`ad-bug-cask-preflight-abort`，内容状态 HEAD
`e040b33028f297f0edc754078e4362c6647a8a00` 加上述四个 blob；完成门
`NOT_REQUIRED`。

提交建议：本次评审边界即一次提交的范围——
`packaging/homebrew/agentdeck-app.rb.tmpl`、`scripts/test-cask-migration.sh`、
`Makefile` 与 `docs/fixes/cask-preflight-abort.md`。工作区里的
`docs/topics/schema-version-signal/` 属于另一个任务，不得一并暂存。贡献者
trailer 按 `.agent-instructions/beads.md` 的 Commit-checkpoint contributor
attribution 从本任务的 Beads 评论解析（实现与 Repair 为 codex，评审记录为
claude-code）。

推送建议：目标 `origin/main`，前提是用户明确授权。注意本地 `main` 已有七笔未推送
的提交（`f282002`、`8f75be9`、`75767f1`、`ccf2059`、`853f02f`、`3c9f0e5`、
`e040b33`），一次推送会把它们一并发布。修复本身要到用户手里还需要一次发布，
因为 tap 分发的是渲染后的 cask。
