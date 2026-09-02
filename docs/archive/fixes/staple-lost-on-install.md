---
status: historical
created: 2026-09-02
retired: 2026-09-02
---

# 缺陷：Cask DMG 内的 bundle 丢失公证票据

## 现象

v0.5.0-rc.4 的发布产物中，DMG wrapper 自身通过 `stapler validate`，但只读挂载
该 DMG 后，其中的 `AgentDeck.app` 报告没有 stapled ticket；通过 Cask 安装到
`/Applications` 的 app 得到相同结果。已安装 bundle 与 DMG 内 bundle 的 51 个
路径一致，严格 codesign 验证和 Designated Requirement 也一致，因此 Homebrew
copy 不是票据丢失点。

该状态仍可能在联网时通过 Gatekeeper，因为系统能够查询 Apple 公证服务；离线
首次启动则失去 stapling 本应提供的保证。

生产脚本修改前，交付版完整回归首先由 submit 计数断言稳定复现：

```text
env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer \
  bash scripts/test-macos-distribution.sh

packaging made 1 notarization submissions, want bundle and DMG
exit 1
```

在同一旧实现候选中屏蔽先行的 submit 计数和顺序断言后，DMG 镜像断言独立报告
`the Cask DMG was assembled before the bundle was stapled`，确认该后置断言同样
能够识别 DMG 内未 staple 的 bundle，而不是不可达代码。

## 根因

旧版 `scripts/package-macos-app.sh` 先把未 staple 的 app 复制到 staging 并构建
DMG，随后只 staple DMG wrapper，再 staple staging 之外的原始 app。最终 ZIP
从已 staple 的原 app 生成，所以 ZIP 正确；DMG 内早已冻结的副本没有票据。

原测试只解压 ZIP 并检查 stub stapler 写入的 marker，没有挂载 DMG 检查其中的
bundle，因此错误顺序仍能通过分发验证。

## 修复边界

- 将已签名 bundle 先打入仅供 `notarytool` 使用的临时 ZIP carrier；第一次提交
  通过后 staple 并 validate 原 app。
- 把已 staple 的 app 复制进 staging 后构建 DMG；对完成的 DMG 进行第二次独立
  submit、staple 和 validate，因为 DMG ticket 绑定 disk image 本身。
- 发布 ZIP 仍最后从同一个已 staple app 生成；临时 carrier 位于 package temp
  root，既不进入 DMG，也不进入 `dist/` 或 checksum 清单。
- 分发回归要求恰好两次 submit，锁定 bundle 与 DMG 各自的 submit/staple/
  validate 顺序，并只读挂载 DMG 验证内部 app 的 staple marker；既有 ZIP marker
  断言保留。
- 同步更正 `docs/specs/cli-design.md` 的打包顺序说明。版本、artifact 名称、签名
  identity、Cask、发布 workflow 与真实安装状态均不改变。

## 验证

- RED：上述聚焦命令在旧顺序上退出 1，实际首先报告只有一次 notarization
  submission；屏蔽该先行计数及顺序断言后，同一候选由 DMG 镜像断言报告 bundle
  staple 前已经组装 DMG。
- GREEN：相同命令在顺序修复后输出
  `macOS distribution packaging: PASS`；补强双提交及全顺序断言后再次通过。
- 该套件同时覆盖 ad-hoc/Developer ID 签名分支、缺失 credential fail-closed、
  ZIP 和 DMG 组装、Gatekeeper 调用契约、临时 tap 清理及 release-verify 可达性。
- `bash -n scripts/package-macos-app.sh scripts/test-macos-distribution.sh`、
  `make check-whitespace` 与 `git diff --check` 均通过。
- 实机离线首次启动尚未对本候选执行。它需要真实的两次 Apple 公证、由该候选
  生成的发布形态 DMG/Cask 安装，以及断网 Gatekeeper/launch 验证；当前授权不包含
  Apple service、发布或真实安装变更。stub marker 只证明组装顺序，不冒充这项
  real-state acceptance。该发布期验证由
  `ad-verify-staple-offline-first-launch` 承载；该任务依赖本 fix 的已提交实现，并
  保留上述每项真实验证及清理要求，各项外部动作仍须取得其自身授权。
- Completion gate：`NOT_REQUIRED`。现行 `.agent-instructions/evidence.md` 未定义
  Lane A WorkUnit；本修复不创建或修改 CEv1 图谱状态。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `9a956b9bf5bbf60dd10de95f5fcfb74f5c43fbc3` 加未提交工作区，
  blob:
  `1c39a5eb91d98005ca7a8a9cbf9b5d6edba990fe scripts/package-macos-app.sh`、
  `98da3e7f49f837ca7c8090382d5ec65ff5d81ce3 scripts/test-macos-distribution.sh`、
  `b198b505d1c489e79e704894904b497da2c52dc5 docs/specs/cli-design.md`、
  `6a738a3b2fea8e4fa6ff47b68dfcfe76353f1a32 docs/fixes/staple-lost-on-install.md`
- Reviewer: claude-code（独立于实现方 codex）
- Method: 仓库核对 + 实测。跑 GREEN；把交付版 `test-macos-distribution.sh` 放进
  HEAD 打包脚本的 worktree 独立复现 RED；再在该 worktree 里单独屏蔽提交计数与
  顺序断言，确认 DMG 镜像断言本身确实会触发而不是死代码；核对临时 carrier 是否
  可能进入 `dist/` 或 checksum 清单；核对 `cli-design.md` 改的是描述还是契约。
  `docs/fixes/**` 无项目自带文档集校验器。
- Scope: `scripts/package-macos-app.sh` 的公证与 staple 顺序、
  `scripts/test-macos-distribution.sh` 新增断言、`cli-design.md` 打包顺序段落，
  以及本记录四节。版本、artifact 名称、签名 identity、Cask、release workflow
  与真实安装状态确未改动。
- Findings:
  - [P2] `R1-F1` 验证 一节引用的 RED 输出不是交付物实际产生的那一条。
    记录写「新增的 DMG 镜像回归稳定复现 … `the Cask DMG was assembled before the
    bundle was stapled`」。把交付版测试放到 HEAD 的旧打包顺序上跑，实际得到的是
    `packaging made 1 notarization submissions, want bundle and DMG`——提交计数
    断言（`scripts/test-macos-distribution.sh:337`）排在 DMG 镜像断言
    （`:377-391`）之前，先失败，后者根本到不了。记录自己的 GREEN 描述其实透露了
    成因：「补强双提交及全顺序断言后再次通过」，说明顺序断言是在那次 RED 之后
    才加的，而 RED 引用没有跟着更新。
    单独屏蔽计数与顺序断言后重跑，DMG 镜像断言确实输出记录引用的那句并失败，
    因此它不是死代码——错的只是记录引用了一条读者复现不出来的输出。
    💡 把 验证 里的 RED 引用改成交付测试实际产生的那条，或明确写出 DMG 断言是
    在屏蔽先行断言后单独验证的。-> open
  - [P2] `R1-F2` 真实状态验收没有承载体。指向本次改动之外：
    工单的验收条件要求「a bundle installed through the cask validates with a
    stapled ticket，或记录说明为何不可能**以及用什么替代**」，并要求「offline
    first launch is exercised, not inferred」。记录诚实地写明两者都没做、也写明
    为什么（需要真实的两次 Apple 公证、由该候选产出的发布形态 DMG 与 Cask 安装、
    断网 Gatekeeper/launch 验证），这一点值得肯定；但它没有给出「用什么替代」，
    也没有任何 Beads issue 或 Backlog 条目承接这项发布期验证。按本项目的 finding
    闭环规则，PASS 之前它必须落到承载体上；`switch-effectiveness-boundary` 的
    `real-session-acceptance` 是由操作者显式豁免的先例，豁免同样是一次要被记下的
    决定。
    💡 建一条发布期验证的 Beads issue 并在 验证 一节引用它，或由用户显式豁免并
    把豁免写进记录。-> open，承载体
    `ad-verify-staple-offline-first-launch`（Round 1 Repair 建立，Round 2 复评
    补记于此，使承载关系与 finding ID 出现在同一条目内）
- Evidence:
  - GREEN：`env DEVELOPER_DIR=… bash scripts/test-macos-distribution.sh` →
    `macOS distribution packaging: PASS`。
  - RED（独立复现）：交付版测试 + HEAD 打包脚本 →
    `packaging made 1 notarization submissions, want bundle and DMG`。
  - DMG 断言有效性：在同一 worktree 屏蔽计数与顺序断言后重跑 →
    `the Cask DMG was assembled before the bundle was stapled`。
  - 顺序修改核对：`scripts/package-macos-app.sh:205-231` 依次为 carrier ZIP →
    submit → staple/validate bundle → `ditto` 进 staging → `hdiutil create` →
    submit DMG → staple/validate DMG；`:234` 的发布 ZIP 仍最后从同一个已 staple
    的 app 生成。与 Beads design 记录的方案 1 顺序逐条一致。
  - carrier 不会泄漏：它位于 `package_root`（`mktemp -d`，由 `trap` 清理），而
    checksum 由 `:249` 的 `shasum -a 256 "$(basename "$dmg")"
    "$(basename "$zip_archive")"` 显式点名两个已发布产物生成。
  - 契约改动是更正而非新决定：旧文写「One submission covers both, because the
    ticket is issued against the code signature they share」——该句为假；新文改为
    两次提交并说明 DMG ticket 绑定 disk image，而用户可见承诺「neither
    direct-download artifact needs the network to clear Gatekeeper on first
    launch」逐字保留。`cli-design.md` 无 `updated:` 字段，故无需更新该字段。
  - `bash -n scripts/package-macos-app.sh scripts/test-macos-distribution.sh`：
    PASS；`bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - 发布 job 未设 `timeout-minutes`，两次 `--wait` 不触及超时上限。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A 边界，该流程缺口由 `ad-chore-cev1-lane-a-boundary` 承载。本记录未新建
  任何 CEv1 节点。
- Verdict: REOPEN

### 下一步指令

修复：fix / staple-lost-on-install / R1-F1 R1-F2

## Repair — Round 1 — 2026-09-02

- `R1-F1` closed：现象与验证不再把后置 DMG marker 断言写成完整交付测试的首个
  RED。记录现在引用实际首先出现的
  `packaging made 1 notarization submissions, want bundle and DMG`，并另行说明
  reviewer 屏蔽 submit 计数与顺序断言后独立确认的 DMG marker 失败。
- `R1-F2` carried：真实双公证、Cask 安装、ticket validate 与断网首次启动由
  Beads 任务 `ad-verify-staple-offline-first-launch` 承载。该任务为 `open`，依赖
  `ad-bug-staple-lost-on-install` 的已提交实现；其 acceptance 绑定一个精确候选
  commit 与 artifact checksums，并要求清理临时 tap/install、保持现有 AgentDeck
  state 不变。Apple 服务、真实安装、发布和任何豁免仍保留各自的显式授权边界。
- Verification：本轮未改生产代码、测试或稳定契约；只更新本记录并创建上述
  Beads carrier。`make check-whitespace` 与 `git diff --check` 均通过。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A WorkUnit；本轮不创建或修改 CEv1 图谱状态。
- Verdict: REOPEN —— `R1-F1` 已关闭，`R1-F2` 已进入明确 carrier；Repair 完成，
  等待独立 Re-review。

## Re-review — Round 2 — 2026-09-02

- Reviewed state: HEAD `9a956b9bf5bbf60dd10de95f5fcfb74f5c43fbc3` 加未提交工作区，
  blob:
  `1c39a5eb91d98005ca7a8a9cbf9b5d6edba990fe scripts/package-macos-app.sh`、
  `98da3e7f49f837ca7c8090382d5ec65ff5d81ce3 scripts/test-macos-distribution.sh`、
  `b198b505d1c489e79e704894904b497da2c52dc5 docs/specs/cli-design.md`、
  `78de78a93ca05e05da6d0d983ddd882a1a899a8d docs/fixes/staple-lost-on-install.md`
  （本轮复评随后把 `R1-F2` 的承载体补记进 Round 1 该条目，记录 blob 因此前移。）
- Reviewer: claude-code（独立于实现方 codex）
- Method: 核对两条 finding 的处置；三个代码/测试/契约 blob 与 Round 1 逐字相同，
  故复用该轮自行执行的 GREEN、RED 与 DMG 断言有效性实测；查 Beads 核实新建的
  承载体确实存在且验收可执行；用 `beads-consistency.py` 自身逻辑复核无主
  finding，并对其结果做了一次反向验证。
- Findings:
  - `R1-F1` closed：现象 一节的代码块改为引用交付版测试实际首先产生的
    `packaging made 1 notarization submissions, want bundle and DMG`，并另起一段
    说明屏蔽先行的计数与顺序断言后，DMG 镜像断言独立报出
    `the Cask DMG was assembled before the bundle was stapled`，因此该后置断言
    并非不可达代码。验证 一节的 RED 条目同步改写。与我在 Round 1 的实测逐字吻合。
  - `R1-F2` carried：`ad-verify-staple-offline-first-launch` 已建立并保持 `open`，
    `depends-on` 本 fix 任务，验收写得可执行——对一个精确候选 commit，两个已发布
    产物与 Cask 安装后的 `AgentDeck.app` 均须带票据通过 `stapler validate`，断网
    下首次启动被 Gatekeeper 接受且应用能启动，临时 tap/install 清理，既有
    AgentDeck state 不变，证据须点明 commit 与 artifact checksum。Apple 服务、
    真实安装与发布仍各自保留显式授权边界。Round 1 该条目原先只写「建一条 Beads
    issue 或由用户豁免」，承载体 ID 不在条目内，本轮复评已补记。
  - [P2] `R2-F1` new，指向本次改动之外：`ownerless_findings`
    （`scripts/hooks/beads-consistency.py:211`）把闭合词绑定到「行」而不是
    「ID」，因此一行里同时出现多个 finding ID 和一个闭合词时，该行会把这一行的
    所有 ID 一并标记为已闭合。Repair 轮次那种收尾行——`Verdict: REOPEN ——
    X1-F1 已关闭，X1-F2 已进入明确 carrier`——就会连带把没有任何承载体的 X1-F2
    判成闭合。这正是 `75767f1` 引入该门禁要防的情况，而且失败是静默的：空列表
    与「记录干净」外观完全一致。已用最小夹具复现：两条 bullet 都以裸 `-> open`
    结尾、无任何承载体，加一行同时点名两个 ID 且含 `已关闭`，返回空列表。
    承载体 `ad-bug-ownerless-findings-same-line`。-> open
- Evidence:
  - 三个代码/测试/契约 blob 与 Round 1 相同（`1c39a5eb`、`98da3e7f`、`b198b505`），
    故 Round 1 自行执行的 GREEN（`macOS distribution packaging: PASS`）、RED
    （交付版测试 + HEAD 打包脚本 → submit 计数失败）与「屏蔽先行断言后 DMG 断言
    独立失败」三项结论在同一内容状态上继续成立；本轮唯一改动是本 Markdown 记录。
  - `bash scripts/check-whitespace.sh`：PASS；`git diff --check`：PASS。
  - Beads：`ad-verify-staple-offline-first-launch` `open`、`task`、P2、
    `depends-on ad-bug-staple-lost-on-install`；
    `ad-bug-ownerless-findings-same-line` 已建立、`open`、P2。
  - `beads-consistency.py` 的 `ownerless_findings` 对本记录返回空列表；但该结果
    本身受 `R2-F1` 影响，故本轮的无主判断以规则原文人工核对为准，而非以该工具
    输出为准。
- Completion gate: NOT_REQUIRED —— 现行 `.agent-instructions/evidence.md` 未定义
  Lane A 边界；本修复未创建或修改任何 CEv1 图谱状态。该契约缺口由
  `ad-chore-cev1-lane-a-boundary` 承载。
- Verdict: PASS

### Task checkpoint

Task checkpoint：`ad-bug-staple-lost-on-install`，内容状态 HEAD
`9a956b9bf5bbf60dd10de95f5fcfb74f5c43fbc3` 加上述四个 blob；完成门
`NOT_REQUIRED`。

提交建议：本次评审边界即一次提交的范围——`scripts/package-macos-app.sh`、
`scripts/test-macos-distribution.sh`、`docs/specs/cli-design.md` 与
`docs/fixes/staple-lost-on-install.md`。工作区里的
`docs/topics/schema-version-signal/` 属于另一个任务，不得一并暂存。贡献者
trailer 按 `.agent-instructions/beads.md` 的 Commit-checkpoint contributor
attribution 从本任务的 Beads 评论解析（实现与 Repair 为 codex，评审记录为
claude-code）。

推送建议：目标 `origin/main`，前提是用户明确授权。本地 `main` 已有八笔未推送
的提交，一次推送会把它们一并发布。修复要真正生效还需要一次发布，而其真实效果
由 `ad-verify-staple-offline-first-launch` 在发布后验证。
