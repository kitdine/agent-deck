---
status: active
created: 2026-09-03
---

# 缺陷：无主 finding 判定既会漏判也会误判

## 现象

`scripts/hooks/beads-consistency.py` 的 `ownerless_findings` 有两个方向相反、根因相同
的错误：

1. 一行同时写 `X1-F1 closed; X1-F2 moved to follow-up` 时，只要整行出现一个
   closure word，旧实现就把这一行的每个 finding ID 都加入 `closed`。即使
   `X1-F2` 的原 finding 只有 bare `-> open`、从未得到 carrier，检查也返回空列表，
   让一个本应阻止 PASS 的 finding 静默漏过。
2. 仓库真实记录已经使用 `A6-F1 — SUPERSEDED` 关闭旧 finding，并在后续 finding
   中承接其问题；旧 closure vocabulary 不含 `superseded`，所以当前实现仍把归档
   `docs/archive/topics/switch-effectiveness-boundary/reviews/architecture.md` 的
   `A6-F1` 报成 ownerless。这个误报曾触发一个追逐已解决问题的 P1 chore。

失败优先回归在修复前得到两个确定失败：same-line fixture 返回 `[]` 而不是
`["X1-F2"]`，归档记录返回的 ownerless 列表错误包含 `A6-F1`。

## 根因

`ownerless_findings` 在 `scripts/hooks/beads-consistency.py:231-236` 遍历一行内每个
`FINDING_ID` 时，对每个 ID 都执行同一个 `FINDING_CLOSED.search(line)`。closure
词只与整行绑定，没有与它实际处置的 ID 绑定。与此同时，`FINDING_CLOSED` 的固定
词表缺少仓库已使用的 `SUPERSEDED`。

函数与测试的 docstring 还沿用了「A6-F1 从未在后续轮次出现」的旧叙事；权威 review
record 已证明 Round 8 明确写了 `A6-F1 — SUPERSEDED`。这不会改变运行结果，但会让
后来者继续用错误历史解释检查规则。

## 修复边界

- 把换行延续的 Markdown bullet 合并为 logical block，再按英文/中文句号与分号切成
  clause。closure 位于 clause 首个 ID 前时，只作用于该 clause 随后的 ID；位于 ID
  后时只作用于最近 ID，除非 `all`、`both`、`are`、`were`、`均`、`都`、`全部`
  明确声明 group disposition。em dash、`->` 与 `处置：` 都作为仓库既有的显式
  disposition bridge。
- 独立的 `See <ID>` / `参见 <ID>` clause 是跨记录引用，不在当前记录中新建 finding。
  一个 ID 后的 suffix closure 不会再泄漏到下一 clause 或兄弟 ID。
- 在既有 closure vocabulary 中加入大小写不敏感的 `superseded`，覆盖真实 A6-F1
  记录。
- 增加 same-line 与真实归档 A6-F1 两个 regression，并保留现有 bare open、Beads
  carrier、Backlog carrier、普通 later-round closure 和 missing-file tests。
- 修正 `ownerless_findings` 与测试类 docstring 对 A6-F1 的错误历史说明。

**明确不改**：`FINDING_ID` grammar、finding bullet 的 carrier 边界、PASS 扫描范围、
Stop-hook transport、Beads 查询与状态协调。相邻问题需要独立 finding，不借本 Lane A
修复扩张。

## 验证

RED：

```bash
python3 scripts/hooks/beads_consistency_test.py \
  OwnerlessFindingsTest.test_closure_word_applies_only_to_the_id_it_follows \
  OwnerlessFindingsTest.test_archived_superseded_disposition_counts_as_closed
```

旧实现运行 2 tests / 2 failures：same-line case 得到 `[]`，A6-F1 case 得到
`['A6-F1']`。

GREEN：

```bash
# 两个失败优先回归
python3 scripts/hooks/beads_consistency_test.py \
  OwnerlessFindingsTest.test_closure_word_applies_only_to_the_id_it_follows \
  OwnerlessFindingsTest.test_archived_superseded_disposition_counts_as_closed
# 完整 ownerless finding 行为
python3 scripts/hooks/beads_consistency_test.py OwnerlessFindingsTest
# 完整 Stop-hook test module
python3 scripts/hooks/beads_consistency_test.py
```

结果依次为 2/2、7/7、28/28 通过。最终内容还需通过 `make check-whitespace` 与
`git diff --check`；这些 L0 结果和 CEv1 gate 写入本阶段 handoff，独立 Review 另行
拥有 verdict。

## Review — Round 1 — 2026-09-03

- Reviewed state: HEAD `905fa6c0f0f6ac33c9a9ad9846783ce80864f858`；候选状态
  `urn:ce:agent-deck:state:workspace:ca65c0386adb236d25fb44ef8b98439033febfb7793d8a44b9ca1ee9737927b0`；
  `scripts/hooks/beads-consistency.py` blob
  `238293c131897620bdbdce9dc72feb43240e56d2`，
  `scripts/hooks/beads_consistency_test.py` blob
  `1e32a1552d32c3cdd05fa72bb17bd70f2be12e20`，本记录评审前 blob
  `22f97e2daed78d25c3b0b5e4540458961a488151`。
- Reviewer: Codex（单 agent、默认模型层级的独立代码与测试评审）。
- Method: 先用 CodeGraph 核对 `ownerless_findings` 的调用路径与 blast radius，再检查
  精确 task diff；将候选函数应用于仓库现有 review/fix records，并以内存加载的 HEAD
  实现对同一文件集作结果对照。发现决定性 reproducer 后按评审规则停止宽泛验证。
- Scope: `scripts/hooks/beads-consistency.py` 的 finding closure 识别、
  `scripts/hooks/beads_consistency_test.py` 的回归保护、既有 review-record 兼容性，以及
  本 Lane A fix 记录能否完成自己的 PASS 生命周期。Stop-hook transport、Beads 查询、
  其他 hook 检查及无关 `schema-version-signal` 工作不在本轮范围。

### 📋 评审报告：fix / ownerless-findings-same-line

📊 总体评分：4/10

✅ 结论：FAIL

### 🔴 严重问题——必须修复

[`scripts/hooks/beads-consistency.py:230`] R1-F1 `[P1]`：按“当前 ID 结束到下一
ID 开始”截取 disposition，只能识别 closure 词位于 ID 后面的写法，破坏了仓库已经
使用的 prefix/group closure；候选因此会为已关闭 finding 制造新的 ownerless 结果。

- 行为风险：任何采用 `all closed: <ID>`、`<ID1> 与 <ID2> 均已关闭` 等合法既有写法
  的 PASS record，只要以后被修改，就可能被 Stop hook 错误阻断。更直接地，本文件
  第 13 行的说明性 `X1-F1 closed; X1-F2 moved to follow-up` 已被候选函数判为
  `X1-F2` ownerless；追加 PASS 后，本任务自己的必需 review artifact 会触发该阻断。
- 证据：真实记录
  `docs/archive/topics/desktop-app/reviews/desktop-app-contract.md:353-355` 写明
  “three findings, all closed: CD1-F1 ...”，closure 词位于 ID 前；候选返回
  `CD1-F1`，HEAD 不返回。对全部现有 topic review records 的同态对照还显示候选在
  4 份记录中新引入 `CD1-F1`、`CD1-F2`、`R9-F2`、`A1-F1`、`H3-F1` 假阳性，
  同时正确消除了 `A6-F1` 假阳性。新增测试只覆盖
  `X1-F1 closed; X1-F2 ...` 的 suffix 分隔形式，没有覆盖真实 prefix/group 形式。

💡 有界修复：在不恢复“整行一个 closure 关闭全部 ID”的前提下，让 disposition
解析同时支持仓库现有的 prefix/group closure 与逐 ID suffix closure；增加至少一个
真实 prefix/group record 回归，并增加本 fix record 到达 PASS 后不会自我误报的回归。
用同一组现有 review records 做 HEAD/候选结果对照，候选允许移除真实误报
`A6-F1`，但不得新增 ownerless ID。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- `superseded` 使用大小写不敏感匹配，真实 A6-F1 回归能够证明旧假阳性被消除。
- same-line 测试准确锁定了“一个 suffix closure 不得泄漏到后续兄弟 ID”的原始漏判。
- 变更保持在一个函数、两项回归与一个 Lane A 记录内，没有扩张 Stop-hook transport
  或 Beads 状态协议。

### 📝 总结

评审对象由上述 HEAD、三份 blob 与候选状态唯一标识。修复解决了两个原始 reproducer，
但当前切片规则以另一方向破坏真实 closure 语法，并会阻断本 fix 自身的 PASS 收尾；
因此现有 `existing-closure-compatibility` 证据被决定性反例推翻。剩余不确定性仅是修复者
选择何种 clause 解析方式，不影响必须同时保留 prefix/group 与逐 ID suffix 语义的边界。

- Findings:
  - R1-F1 `[P1]` -> open；修复范围仅为上述 closure clause 兼容性与对应回归。
- Evidence: 候选/HEAD 同态扫描的决定性差异、真实
  `desktop-app-contract.md:353-355` reproducer、当前 fix record 自检，以及已复用的
  同状态 Development 证据（focused 2/2、Ownerless 7/7、hook module 28/28、L0）。
- Completion gate: `FAILED`
- Verdict: REOPEN

## Repair — Round 1 — 2026-09-03

- `R1-F1` closed：原始“ID 到下一 ID”切片被 logical-clause parser 取代。prefix
  closure 只覆盖同一 clause 中随后出现的 IDs；suffix closure 默认只覆盖最近的
  preceding ID，只有显式 group marker 才覆盖兄弟 IDs。这样同时保留
  `all closed: CD1-F1 ...`、`X1-F1、X1-F2 均已关闭`、
  `A8-F1 requirements consequence — CLOSED`、
  `T6-F2 ... -> Fixed` 与逐 ID `X1-F1 closed; X1-F2 ...` 的既有语义。
- Markdown `sentence.** Next sentence` 被识别为句界，防止前一句 ID 进入后一句
  closure；独立 `See A1-F1 ...` / `参见 ...` clause 被当作跨记录 reference，而非
  当前记录新 finding。没有采用“只认 severity declaration”的方案，因为对全部现有
  records 的同态诊断证明它会删除超出本 finding 的既有结果。
- 新增四项失败优先回归在旧候选上 4/4 失败：prefix group 返回两个 ownerless、
  suffix group 留下第一个 ID、真实 desktop contract 报 `CD1-F1/CD1-F2/CD1-F3`，
  self-PASS fixture 报五个说明性/reference IDs。修复后连同原 two-way regressions
  共六项全部通过。
- 本记录中的说明性/reference IDs `X1-F2`、`CD1-F1`、`CD1-F2`、`R9-F2`、
  `A1-F1`、`H3-F1` 与 `T6-F2` are all closed as examples；它们不是本记录提出的
  findings。
  真实 `A6-F1 — SUPERSEDED` 仍由 repository vocabulary 直接关闭。
- HEAD/候选同态诊断覆盖全部现有 topic review records 与 fix records；最终结果没有
  新增任何 ownerless ID。真实 prefix/group 或 reference 语义还移除了若干 HEAD 的
  假阳性，但没有以恢复“整行 closure”换取兼容。
- Verification：原始 two-way focused regressions、R1-F1 六项 focused regressions、
  `OwnerlessFindingsTest` 12/12 与完整 `beads_consistency_test.py` 33/33 均通过；
  `make check-whitespace` 与 `git diff --check` 通过。
- 不涉及：`FINDING_ID` grammar、finding carrier 边界、PASS 扫描范围、Stop-hook
  transport、Beads 查询/状态协议与无关 `schema-version-signal` 工作均未改动。
- Completion gate：Round 1 Review 的失败 evidence 保持不可变；Repair 不自签新的
  review evidence，由独立 Re-review 在最终内容状态上重新查询。
- Verdict: REOPEN —— `R1-F1` 已关闭，Repair 完成，等待独立 Re-review。

## Re-review — Round 2 — 2026-09-03

- Reviewed state: HEAD `905fa6c0f0f6ac33c9a9ad9846783ce80864f858`；复评前
  `scripts/hooks/beads-consistency.py` blob
  `03c2be95c8843a17ccb7d9161c73d50c971a6f59`，
  `scripts/hooks/beads_consistency_test.py` blob
  `930e4a14163fdde60e4e0638ae46989988f9615c`，本记录 blob
  `54ca3bca1fb151f01f641b2aeb3d0bc94f9978ff`；scoped fingerprint
  `81ea083403fd8674965d4dab0864340f7ff3d5f1f71a026ed1115a770b856d6b`。
- Reviewer: claude-code。记录、实现与 Repair Round 1 均由 codex 完成，本 reviewer
  未参与其中任何一项，对本主题是冷上下文。
- Method: 不采信记录中的任何数字与关闭声明。重跑 GREEN；用 `git show HEAD:` 取出
  修复前实现，在**仓库目录内**临时替换后跑同一组回归以取得真实 RED，随后按
  sha256 校验还原；再以 HEAD 与候选两份实现对全部记录做同态对照扫描，检验
  `R1-F1` 的核心边界「不得新增 ownerless ID」。
- Scope: `ownerless_findings` 的 closure 识别、`beads_consistency_test.py` 的回归
  保护、既有 review record 兼容性、本记录自身。Stop-hook transport、Beads 查询与
  状态协议、`FINDING_ID` grammar 均未在本轮改动范围内，也未复查其内部实现。
- Findings:
  - `R1-F1` **closed**，其核心边界经独立复算成立。按 hook 的**真实扫描范围**
    （`beads-consistency.py:578` 只取 `/reviews/` 与 `docs/fixes/` 下的 `.md`）
    做 HEAD/候选同态对照：候选**新增 ownerless ID 为 0**；同时消除 11 处既有
    假阳性，分布在 `desktop-app` 的 7 份 review（`R3-F1`×6、`CD1-F3`）、
    `switch-effectiveness-boundary/reviews/architecture.md` 的 `A6-F1`、
    `usage-attribution-precision` 的 `T1-F2` 与 `A2-F1`，以及本记录自身的
    `CD1-F2`、`R9-F2` 两个说明性 ID。方向与 `R1-F1` 的有界修复要求一致：允许
    移除真实误报，不得新增。
  - 本轮为说明对照结果而引用的既有记录 ID —— `R3-F1`、`CD1-F3`、`T1-F2`、
    `A2-F1`、`A6-F1`、`CD1-F2`、`R9-F2` —— 都是对其他记录的引用，不是本记录提出的
    finding，在此全部已关闭。这条声明本身也是修复行为的一次现场验证：新规则没有
    把上一条 `R1-F1 closed` 的 closure 泄漏给同一 bullet 内的引用 ID，Stop hook
    因而正确报出它们缺少处置。
  - RED 独立复现成立。在仓库内以修复前实现跑 `OwnerlessFindingsTest`，12 项中
    **4 项失败**，覆盖两个原始缺陷方向与 `R1-F1` 引入的两个真实场景：
    `test_closure_word_applies_only_to_the_id_it_follows`（same-line 漏判）、
    `test_archived_superseded_disposition_counts_as_closed`（`A6-F1` 误报）、
    `test_real_prefix_group_record_introduces_no_ownerless_findings`（真实
    prefix/group 记录）、
    `test_cross_record_see_clause_does_not_introduce_a_finding`（跨记录引用）。
    替换后的实现已按 sha256 校验还原，工作区未被本轮污染。
  - GREEN 复算成立：`OwnerlessFindingsTest` 12/12、完整
    `beads_consistency_test.py` 33/33 通过，与 Repair Round 1 记录的数字一致。
    `make check-whitespace` 与 `git diff --check` 通过。
  - 修复边界被遵守：改动集中在 `ownerless_findings` 一个函数、回归测试与本记录，
    `FINDING_ID` grammar、carrier 边界、PASS 扫描范围、Stop-hook transport 与
    Beads 协议均未触碰，与「明确不改」一节相符。
  - 检查过但**不构成 finding**：主体「验证」一节的数字（RED 2/2、
    `OwnerlessFindingsTest` 7/7、hook module 28/28）是 Round 1 之前只有两项新回归
    时的状态，现已分别为 4/12、12/12、33/33。因 Repair Round 1 在同一文件内明确
    记录了当前数字，且「验证」节列出的命令本身仍然正确可跑，读者不会被误导，故
    不作为 finding；若日后重排该节，宜一并同步。
  - 另记一项**超出本轮范围**的观察，指向本次改动之外：按候选实现，仓库中 7 份
    `Verdict: PASS` 的归档 review record 仍报出 ownerless ID（`A16-F5`、
    `A16-F2`、`A18-F1`、`H3-F1`、`A1-F3`、`A1-F5`、`A1-F1`、`A2-F1`、`R1-F2`、
    `R4-F1`）。这些在修复前后同样报出，不是本次引入；且 hook 只扫描**本工作区
    改动过**的文件，归档记录不会被改动，因此实际不会触发阻断。是否属于真实无主
    finding 需要逐条判读，超出本 Lane A 修复的边界。Carrier：本记录所属的
    `ad-bug-ownerless-findings-same-line` 已交付后，若要追查应另立 issue；本轮
    不因此阻断 PASS。
- Evidence:
  - `python3 scripts/hooks/beads_consistency_test.py`（33/33）与
    `… OwnerlessFindingsTest`（12/12）。
  - RED：`git show HEAD:scripts/hooks/beads-consistency.py` 取出修复前实现，
    `/bin/cp -f` 临时替换后跑 `OwnerlessFindingsTest` 得 4 项失败，随后还原并以
    sha256 比对确认与替换前一致；`git diff --stat` 仍为 `+87/-15`。
  - 同态对照：分别以两份实现载入 `ownerless_findings`，遍历 `/reviews/` 与
    `docs/fixes/` 下全部记录，比较集合差；新增集为空，消除集 11 处。
  - `make check-whitespace`、`git diff --check` 均通过。
- Completion gate: `VERIFIED` —— 五条 required criteria 在本轮复评后的内容状态上
  记录 review 级 evidence 后全部 `pass`；Round 1 Review 对
  `existing-closure-compatibility` 的 `fail` 与实现阶段的
  `implementation_verification` 证据均保持不可变，新证据以 `supersedes` 指向
  Round 1 的失败记录。
- Verdict: PASS —— `R1-F1` 已闭合，两个原始缺陷方向均有失败优先回归保护，且候选
  未在既有记录上新增任何 ownerless ID。
