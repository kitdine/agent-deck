---
status: active
created: 2026-09-06
---

# 缺陷：同步 tasks.md 状态矩阵被误报为未认领的分解任务

## 现象

2026-09-06，`schema-version-signal` 的 surface framework 阶段（`设计：
schema-version-signal / ux/menubar-schema-signal.md`）收尾时，Stop hook 阻塞并报：

```
docs/topics/schema-version-signal/tasks.md is being edited, but its task
ad-svs-doc-tasks-design (schema-version-signal) is still `open`. Drafting a
document is the `in_progress` state, and it should be claimed.
```

该次对 `tasks.md` 的全部改动是本阶段强制的状态同步：勾选
`ux/menubar-schema-signal.md` 的 Draft 单元格、把解释未勾选行的那句「Three rows」
改为「Two rows」、并记录该 surface 文档 D8 得出的 widget 结论——而这条结论正是
`tasks.md` 自己那段文字预先要求写在那里的。Tasks 矩阵一行都没有产生：文件本身写着
锚点在阶段 8 定义，且要等本文件通过评审才可开发，而阶段 5、6（`architecture.md`
及其评审）都尚未开始。

hook 要求的动作，是把一个没有开始的分解任务标成 `in_progress`。那正是
`.agent-instructions/beads.md` 记录的、文档工作当初要独立 dispatch 的原因：
「a task sat claimed and `in_progress` while nothing was being implemented, which
is the opposite of what dispatch should report」。所以这条报告不是提醒，而是要求
一次错误的 dispatch 写入，方向与 hook 自身的目的相反。

移除修复守卫后的失败优先回归给出两个确定失败，报文与线上观察逐字一致：
`test_documents_matrix_sync_does_not_claim_the_decomposition_task` 与
`test_an_untracked_tasks_file_holding_no_task_rows_is_not_reported` 各返回一条
`is being edited ... is still \`open\`` 的 note，而不是空列表。

## 根因

`scripts/hooks/beads-consistency.py` 的 check 3 判定完全基于路径：凡是改动过的
`docs/topics/<topic>/<document>.md`，只要同名文档任务处于 `open` 就报告，不读取
文件内容，也无从判断改的是哪一部分。

对文档集中的其它文件这是对的——起草它们就是 `in_progress`。`tasks.md` 是唯一的
例外，因为它同时是两样东西：

- **该 topic 唯一的状态权威**。阶段 1 创建它的 Documents 矩阵，此后每个阶段都要
  回来勾选自己刚起草的那一行；`docs/documentation-workflow.md` 的阶段表把这条同步
  写进了每一个阶段。
- **阶段 8 的交付物**。`ad-<topic>-doc-tasks-design` 产出的只有 Tasks 矩阵。

一个路径匹配无法区分这两类写入，于是每一次跨阶段的状态同步都会被读成「有人在起草
分解」。窗口还恰好总是开着：check 3 只看 `open` 状态，而分解任务从 topic 创建到
阶段 8 启动之间一直是 `open`，也正是阶段 1–7 反复改写 Documents 矩阵的整段时间。

## 修复边界

- 新增 `head_text(root, rel, deadline)`：读取一个路径的已提交内容，任何失败或未跟踪
  都返回空字符串——「尚未记录任何内容」是对未跟踪文件的诚实读法，不是报告的理由。
- 新增 `decomposition_changed(root, rel, deadline)`：比较工作树与 HEAD 两侧的
  `matrix_rows`。该函数只识别任务行，Documents 矩阵永远不匹配它（主体单元格没有
  反引号，这一点既有实现与既有测试都已声明），因此勾选 Draft、增删文档集行、或
  改动周边散文都不会让它为真。
- check 3 中仅对 `tasks.md` 加一道守卫：Tasks 矩阵没有变化就不报。其余每一个文档
  的触发条件一字未动。
- 补四条回归：Documents 矩阵同步不报、Tasks 矩阵起草仍然报、未跟踪且没有任务行的
  `tasks.md` 不报、其它文档仍按路径匹配照常报。

**明确不改**：check 3 对其它文档的判定、check 1/2/4/5、Stop-hook transport、
`matrix_rows` 与 `decomposition_passed` 的语法、Beads 查询方式，以及 hook「只报告、
从不写 Beads」的边界。check 3 的 `.endswith(".md")` 会漏掉 `git status --porcelain`
把整个新目录（如 `docs/topics/<topic>/ux/`）报成一条带斜杠条目的情形——这是同一函数
里的相邻缺陷，需要独立 finding，不借本次 Lane A 修复扩张。同样独立的还有 check 5：
它对 `drafting`／`repairing` 做整词匹配，不区分「description 在复述生命周期」与
「description 把这两个词当普通英文动词使用」。本次修复过程中它就在新建的
`ad-bug-tasks-md-status-sync-false-claim` 上实报了一次——描述里写了一句
"could not tell drafting a document from ..."。改写描述即可绕开，但判定本身仍然过宽。

## 验证

L1（hook 单元测试）。RED，先移除守卫：

```bash
python3 scripts/hooks/beads_consistency_test.py
# Ran 37 tests ... FAILED (failures=2)
# - ['docs/topics/example/tasks.md is being edited, but its task doc-tasks '
#    '(example) is still `open`. Drafting a document is the `in_progress` state, '
#    'and it should be claimed.']
```

GREEN，恢复守卫：

```bash
python3 scripts/hooks/beads_consistency_test.py
# Ran 37 tests in 0.060s
# OK
```

对真实工作树实跑该 hook（`docs/topics/schema-version-signal/tasks.md` 仍处于修改
状态，`ad-svs-doc-tasks-design` 仍为 `open`）：

```bash
echo '{"hook_event_name":"Stop","stop_hook_active":false}' \
  | python3 scripts/hooks/beads-consistency.py --runtime claude
# exit=0，无输出
```

误报消失，且 hook 对当前仓库状态未报告任何其它分歧。

## 授权说明

本次开发授权来自用户在会话中的直接指令「直接修复这个hook的错误」，同时也是对
lane 的裁定（Lane A：实现从未满足既有意图，无新用户可见行为需要决定）。因此没有
铸造 `Authorize Development` Gate——补铸一个再由 agent 自行关闭，只会伪造一次人工
批准。评审未被豁免：本记录的 Review 轮次尚未开始。

## Review — Round 1

- 评审内容状态：HEAD `3aaf6c6349c1a0f4a4a427cbde28159ee1f20999`，三个未提交文件的
  blob 分别为 `scripts/hooks/beads-consistency.py` `70076bcc`、
  `scripts/hooks/beads_consistency_test.py` `2923db38`、本记录 `2ef59d52`。
- 评审者：Claude Code
- 方法：Lane A 的单一评审问题——这次改动是否让实现满足既有意图，以及回归测试在实现
  停止满足时是否会失败。守卫逻辑逐行读过；`matrix_rows` 的「Documents 矩阵永不匹配」
  这一前提对着正则与真实矩阵行独立核对；RED 在 scratchpad 副本上复核而非改动仓库代码；
  新增的两个函数用真实仓库状态直接调用验证。
- 范围：`beads-consistency.py` 的 `head_text`、`decomposition_changed` 与 check 3 的
  守卫；`beads_consistency_test.py` 新增的四条回归；本记录的现象、根因、边界与验证四节。
  check 1/2/4/5 与 Stop-hook transport 未改动，只在判断相邻缺陷归属时读过。
- 发现：
  - [P2] R1-F1 — 新增的 `head_text` 没有任何直接测试。四条新回归全部
    `mock.patch.object(MODULE, "head_text", ...)`，于是 mock 里假定的契约与真实实现之间
    没有任何断言把两者绑住：把 `head_text` 改成失败时抛异常或返回 `None`，四条测试
    依然全绿，而 hook 在真实仓库会崩。这条契约不是可有可无的——守卫在「未跟踪的
    `tasks.md`」下的行为完全依赖它返回空字符串，而那正是阶段 1 刚创建 topic 时的常态，
    也正是第三条回归声称覆盖的场景，却是靠 mock 断言的。`docs/documentation-workflow.md`
    对 Lane A 的就绪条件就是「回归测试在实现停止满足契约时会失败」，这里不满足。
    评审期间以真实调用手工验证过三种输入（已提交路径 20626 字节、未跟踪路径 0、
    不存在路径 0）与预算耗尽（返回空串），但手工验证不是回归保护 -> open；补一条直接
    调用 `head_text` 的测试，覆盖守卫真正依赖的那条分支即可，不需要覆盖全部三个错误出口。
  - [P2] R1-F2 — check 3 的 `.endswith(".md")` 看不到 `git status` 报出的整目录条目。
    本记录在「修复边界」里自陈了它并明确排除，判断正确，但当时没有点名 carrier，
    而裸的「需要独立 finding」不是归属。评审期间实测坐实：工作区当时正有一条
    `docs/topics/schema-version-signal/ux/` 目录条目，hook 退出 0 且不报告
    -> open，carrier `ad-bug-hook-check3-directory-entries`。
  - [P2] R1-F3 — check 5 对 `drafting`／`repairing` 的整词匹配不区分「description 在
    复述生命周期」与「把这两个词当普通英语动词用」。同样已自陈、同样正确地排除在
    本次修复之外、同样缺 carrier -> open，carrier `ad-bug-hook-check5-lifecycle-word-match`。
  - [P2] R1-F4 — `.agent-instructions/beads.md` 的 Lane A 一节明文要求一个
    `Authorize Development` Gate，本次没有铸造。「授权说明」给出的理由成立——授权来自
    用户的直接指令，事后补铸再自行关闭只会伪造一次人工批准——但规则与实做的分歧本身
    没有归属，下一个 Lane A fix 会在同一个岔口重新论证一遍
    -> open，carrier `ad-lane-a-gate-vs-direct-instruction`。
- 证据：`python3 scripts/hooks/beads_consistency_test.py` 在仓库内 `Ran 37 tests ... OK`。
  RED 复核在 scratchpad 副本上移除那一道守卫后重跑，得到与本记录逐字一致的两条失败
  （`test_documents_matrix_sync_does_not_claim_the_decomposition_task` 与
  `test_an_untracked_tasks_file_holding_no_task_rows_is_not_reported`，报文均为
  `... is being edited, but its task doc-tasks (example) is still \`open\` ...`）；
  副本另有一条 `test_project_contract_grants_stage_internal_state_transitions` 的 ERROR，
  经查是该测试读 `SCRIPT.parents[2]/AGENTS.md`、副本目录下无此文件所致，是复核环境的
  副产品，不构成第三条失败，本记录写的 `FAILED (failures=2)` 准确。真实工作树实跑
  `echo '{"hook_event_name":"Stop","stop_hook_active":false}' | python3 scripts/hooks/beads-consistency.py --runtime claude`
  得 exit=0 无输出。直接调用新增函数：`head_text` 对已提交路径返回 20626 字节、对未跟踪
  路径与不存在路径均返回空串、预算耗尽时返回空串；`decomposition_changed` 对真实的
  `docs/topics/schema-version-signal/tasks.md` 返回 `False`，两侧 `matrix_rows` 均为 `[]`。
  `matrix_rows` 的前提独立核对：`ANCHOR_CELL` 要求反引号内为 `[a-z0-9][a-z0-9-]*`，
  所以即便有人给 Documents 矩阵的文档名加上反引号，`` `requirements.md` `` 也因点号
  匹配失败，前提比注释声称的更稳。守卫位置在 `if not stuck: continue` 之后，
  因此每个 topic 至多一次 `git show`。
- 完成门禁：NOT_VERIFIED — 本轮 REOPEN，任务边界未跨越。按
  `.agent-instructions/beads.md` 的 Lane A 规则，本 fix 的 CEv1 边界是
  `unit_kind: task`、`work_unit_id: fix:tasks-md-status-sync-false-claim`，其上没有
  topic 边界；本轮未做 CEv1 写入。
- 结论：REOPEN

## Repair — Round 1

- 处置范围：仅 R1-F1。R1-F2、R1-F3、R1-F4 已在 Round 1 各自点名 carrier
  （`ad-bug-hook-check3-directory-entries`、`ad-bug-hook-check5-lifecycle-word-match`、
  `ad-lane-a-gate-vs-direct-instruction`），本轮不动它们，也不把它们的结论搬进本记录。
- R1-F1 closed：新增 `test_head_text_returns_committed_content_and_empty_for_an_unrecorded_path`
  （`scripts/hooks/beads_consistency_test.py`），直接调用 `head_text`，不经任何 mock：
  已提交路径 `.agent-instructions/beads.md` 必须读回非空内容；HEAD 中不存在的路径
  `docs/topics/zzz-not-a-topic/tasks.md` 必须读回空串；预算耗尽时读回空串且
  `subprocess.run` 未被调用。
- 为什么两个方向都断言，而不只覆盖守卫依赖的那一条分支：finding 允许只覆盖一条，但
  「恒返回空串」这种实现同时满足未跟踪分支，却会让阶段 8 真正起草时的比较永远与空集
  相较。只断言空串的一侧，正好放过这一类实现。两条断言各值一行。
- 变异验证（RED，逐次改动仓库文件后立即还原，还原以 `git diff --stat` 复核）：
  - `head_text` 恒返回 `""` → `Ran 38 tests ... FAILED (failures=1)`，唯一失败是新测试。
  - 预算耗尽仍起子进程（`remaining_timeout(deadline) or 5.0`）→ 同样 `failures=1`，
    唯一失败仍是新测试。
  - 未变异 → `Ran 38 tests ... OK`。
  两次变异都不会让原有 37 条中的任何一条失败，这正是 R1-F1 指出的缺口：契约此前
  只活在 mock 里。
- 未覆盖且有意留白：`subprocess` 抛异常那条出口仍无回归。Round 1 明确写了不需要覆盖
  全部三个错误出口，本轮不扩张。
- 实现未改动：`head_text`、`decomposition_changed` 与 check 3 的守卫一字未动，本轮
  只增加测试。

## Review — Round 2

- 评审内容状态：HEAD `3aaf6c6349c1a0f4a4a427cbde28159ee1f20999`（未变）。
  `scripts/hooks/beads-consistency.py` 仍是 Round 1 的 `70076bcc`——本轮实现一行未动；
  `scripts/hooks/beads_consistency_test.py` 由 `2923db38` 变为 `0f3ac323`；本记录在写入
  本轮之前为 `d3dba544`。CEv1 绑定的范围指纹取本轮定稿后的三个文件，见下。
- 评审者：Claude Code
- 方法：按 Re-review 契约逐条复核 Round 1 的四条 finding，并独立复核修复自己声称的
  变异验证——不改动仓库文件，改为加载真实模块后替换 `MODULE.head_text`，再单独和整套
  运行测试。R1-F2 与 R1-F3 的 carrier 状态从 Beads 实查。
- 范围：新增的 `test_head_text_returns_committed_content_and_empty_for_an_unrecorded_path`
  与本记录的 `## Repair — Round 1` 一节。实现未变，不重复 Round 1 已通过的守卫逻辑复核。
- 发现：
  - R1-F1 closed。新测试直接调用 `head_text`，不经任何 mock，并且两个方向都断言：
    已提交路径 `.agent-instructions/beads.md` 读回非空、HEAD 中不存在的路径读回空串、
    预算耗尽读回空串且 `subprocess.run` 未被调用。独立变异复核确认它真的会杀死变异——
    `head_text` 恒返回空串，整套 38 条中唯一失败的是这条新测试；预算耗尽仍起子进程
    （`remaining_timeout(deadline) or 5.0`），整套唯一失败仍是它。两次变异都不动摇原有
    37 条，正是 Round 1 指出的缺口所在。修复还多做了一步且理由成立：只断言空串一侧会
    放过「恒返回空串」这类实现，而那类实现会让阶段 8 的真实比较永远与空集相较。
  - R1-F2、R1-F3、R1-F4 均维持 Round 1 的处置，本轮未重新论证：三者都指向本次改动之外，
    各自的 carrier 经 Beads 实查仍为 `open`——`ad-bug-hook-check3-directory-entries`、
    `ad-bug-hook-check5-lifecycle-word-match`、`ad-lane-a-gate-vs-direct-instruction`。
    R1-F3 在 Round 1 之后又添了一个实例：为它建的 carrier 本身被 check 5 实报，因为
    描述必须引用那两个退役状态名才能说清缺陷是什么；该实例已记入那个 carrier，
    不搬进本记录。
  - 无新增 finding。本轮只增加测试，实现 blob 与 Round 1 逐字一致，已核验。
- 证据：`python3 scripts/hooks/beads_consistency_test.py` → `Ran 38 tests ... OK`。
  变异复核（不改仓库文件，替换已加载模块的 `head_text` 后运行）：未变异时新测试通过；
  变异一「恒返回空串」与变异二「预算耗尽仍起子进程」，单独运行该测试均失败，整套运行
  均为 `run=38 failed=1`，失败者都只有
  `test_head_text_returns_committed_content_and_empty_for_an_unrecorded_path`。
  复核前后 `git hash-object scripts/hooks/beads-consistency.py` 均为 `70076bcc`，
  确认实现未被改动。`MODULE.HOOK_BUDGET` 存在（`beads-consistency.py:100`，值 10.0），
  新测试用的是真实预算而非硬编码。测试耗时由 0.060s 增至 0.147s，多出的是两次真实
  `git show`。新测试锚定真实仓库路径这一点与既有的
  `test_project_contract_grants_stage_internal_state_transitions` 同风格。
- 完成门禁：VERIFIED — Lane A 任务边界，`unit_kind: task`、
  `work_unit_id: fix:tasks-md-status-sync-false-claim`，其上没有 topic 边界。证据绑定
  HEAD `3aaf6c6` 加本轮定稿后三个工作产物的范围指纹；指纹配方、节点 id 与门禁查询结果
  记在该 Beads 任务的评论里。授权提交产生后按 `.agent-instructions/evidence.md` 以
  不可变 Git tree 重记并 `supersedes` 本次候选态。
- 结论：PASS
