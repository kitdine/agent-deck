---
status: active
topic: desktop-app
subject: ux/widget.md
---

# Review log — desktop-app / ux/widget.md

## Round 1 — 2026-08-17

### 📋 独立设计评审 — desktop-app / ux/widget.md

📊 总体评分：6/10

✅ 结论：FAIL

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`，工作区对本文档
  无改动；`docs/topics/desktop-app/ux/widget.md` blob
  `c317583f8ab9a521c9b49d51c16462f4cb1319fe`；比对基准为同一 HEAD 的
  `architecture.md`（blob `e23ccc7cab3545f4e6c19ab15d5cc33e6261c4fb`）、
  `ux/menubar.md`（blob `5303e0d14556da181632f80ccc802b3f82c3a068`）、
  `requirements.md` 与 `ux/prototype/desktop-surfaces.html`
- Reviewer: claude-code
- Target class: design / contract。尚无任何实现满足它，因此适用 premise validity、
  consistency、scenario coverage、decision completeness、internal contradiction、
  hidden coupling 等维度，而不是代码维度。
- Method: 单 agent 有界评审。先按文档自述的推导（四个问题 → 四类 widget；尺寸选
  深度不选主题）核对内部一致性，再把它声明需要的每一个字段逐条回到
  `architecture.md` 的 App Group projection 清单核验——这是本 topic 已经出过两次
  同类缺陷的地方（R9-F2 与 Round 10 的 prototype `Month` 残留），因此按"供给方
  实际列出了什么"而不是按"文档说已供给"来判断。
- Scope: `docs/topics/desktop-app/ux/widget.md` 全文，及其对 `architecture.md`
  投影、`requirements.md` 验收边界、`ux/menubar.md` 共享词汇、prototype widget
  板块的依赖
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0（项目自带的文档集审计，
  是本类目标的必需证据）；`make check-whitespace` -> exit 0；`git diff --check`
  -> exit 0；上列 HEAD 与 blob 由 `git rev-parse` / `git hash-object` 实测。未改动
  任何产品代码、测试、配置或被评审的文档。

#### 🔴 严重问题 — 必须修复

**W-F1 — `docs/topics/desktop-app/ux/widget.md:85-88,97,110`: `Period` 参数与三期间
对比所需的分期间聚合，投影并未供给，而 widget 没有任何别的取数路径。**

- 行为风险：`magnitude` medium 要同时显示 today / 7 days / 30 days 各自的 cost 与
  tokens（`:97`），large 再叠加（`:98`）；`composition` 也接受 `Period`（`:85`），
  即要求"某一期间的 top-N model shares"。实现者只有三条路，每条都被已通过评审的
  上游边界挡住：在 Swift 侧按 90 个日桶求和，违反 `requirements.md:132-133` 的
  "no second aggregation layer in Swift" 与 `ux/menubar.md:55` 的同一条禁令；改为
  向 helper 请求，违反本文件 `:229` 的"never invokes the helper"（widget 本就没有
  这个能力）；或者干脆渲染不出来——`composition` 的分期间 model shares 属于这一类，
  因为日桶只有 `(date, tokens, cost, sessions)`，根本不带 model 维度，无论怎么求和
  都得不到 7d 的模型份额。
- 证据：`architecture.md:438-456` 的投影清单只有"aggregate usage totals, counts,
  pricing completeness, and cost strings"与"a bounded daily series … plus **the
  period's** `peak` bucket and average"——单数的"the period"，全文件 `period` 一词
  只在此处及无关的 periodic refresh 出现，没有任何一处声明投影同时携带 today、7d、
  30d 三份聚合；`architecture.md:385-396` 的 helper execution contract 只有
  recent-session limit 与超时，没有 period 参数；`requirements.md:132-137` 授权了
  三个期间，因此这不是需求缺口而是投影缺口。prototype 把该缺口画实了：
  `ux/prototype/desktop-surfaces.html:591-622` 的 magnitude medium 与 large 同屏
  列出 `TODAY $12.47 / 7 DAYS $78.20 / 30 DAYS $291.55`。
- 💡 有界修复：在 `architecture.md` 的投影清单里明确写出"三个受支持期间各自的
  totals 与 top-N model shares（按 `today`/`7d`/`30d`），并给出条目上界"，让 Go 侧
  一次投影承载三份；或者删掉 widget 的 `Period` 参数与三期间对比，只保留当前期间
  加日趋势。两者皆可，但必须选一个——本轮不作推定，因为这决定的是投影体积与
  publication 成本，属于 architecture 的决策而非本文档的。

#### 🟡 建议改进 — 推荐

**W-F2 — `docs/topics/desktop-app/ux/widget.md:144-163`: 复用了 menu bar 的
qualifier 名称却重新定义了它们的条件，而文档自称没有引入第二套词汇。**

- 行为风险：`:144-146` 说"reuse the menu-bar model rather than inventing a second
  one, because a user seeing both must not meet two vocabularies for one
  condition"。但 `stale` 在 `ux/menubar.md:123` 的条件是"coordinator 处于
  `refreshing` 或 `degraded` 且保留了上一份快照"——一个刷新状态，与年龄无关；在本
  文件 `:156` 变成"15 分钟到 6 小时"的年龄区间。`aged` 在 `menubar.md:124` 是
  ">15 分钟"，在本文件 `:157` 是">6 小时"。同名不同义且阈值相差一个数量级，文档
  没有一处说明这个分歧。实现者若照字面"复用同一个模型"抽出共享的 qualifier 类型，
  会在两个 surface 上得到互相矛盾的判定。
- 证据：`ux/menubar.md:120-124` 与本文件 `:154-163` 的两张 qualifier 表；`:161-163`
  只解释了 widget 为什么需要 `aged`，没有解释为什么沿用 `stale` 这个名字去表达一个
  完全不同的条件。
- 💡 有界改进：明写这两个 qualifier 在 widget 上是**按年龄**判定的，说明 widget 没有
  刷新状态机因而无法沿用 menu bar 的判定，并二选一：要么给 widget 的年龄判定换一组
  名字，要么在两份文档里把 `stale`/`aged` 统一为年龄语义并对齐阈值。

**W-F3 — `docs/topics/desktop-app/ux/widget.md:39,135,137`: `rhythm` 读取的
"active days"、"quietest and busiest day names" 既不在投影清单里，也不在本文件自己的
Data requirements 表里。**

- 行为风险：与 W-F1 同源但范围更小。`:39` 把"active days"列为 `rhythm` 读取的字段，
  `:135` 让 small 显示"Active-day count over the last 30 days"，`:137` 让 large 显示
  "the quietest and busiest day names"；而 `:271` 的 Data requirements 只声明了
  `7×24 hour-of-week intensity`。实现者要么再在 Swift 侧数非零日桶（同样触碰
  W-F1 的禁令），要么发明一个未入契约的字段。
- 证据：`architecture.md:446-447` 只有 7×24 强度网格与 90 日桶，无 active-day 计数、
  无最忙/最闲日名；本文件 `:261-272` 的 Data requirements 表也没有这两行——即"文档
  自己声明要的字段"和"文档自己列出的需求"不一致。
- 💡 有界改进：把这两项加进 Data requirements 表并在 `architecture.md` 里供给（两者
  都是对 90 日桶的一次性归约，成本很低），或者把 `rhythm` small/large 改成只用
  7×24 网格与日桶本身能直接呈现的内容。

**W-F4 — `docs/topics/desktop-app/ux/widget.md:195`: `empty` 的 `magnitude` 文案与
menu bar 对同一条件的固定文案差一个词。**

- 行为风险：本文件写 `No activity today` / `今天没有活动`；`ux/menubar.md:508` 对
  current、无 issue 表面的同一条件固定为 `No local activity today` /
  `今天没有本地活动`。`:201-203` 自己给出的理由——"读到两句不同的话会合理地认为这是
  两种不同的状况"——正好适用于此。`:205-207` 解释的是 `empty` **按 kind** 分化，不是
  按 surface 分化，因此这条差异没有被任何理由覆盖。这与 `ux/menubar.md` 刚在
  Round 12 修掉的 R11-N2 是同一类近似串。
- 证据：本文件 `:195` 与 `ux/menubar.md:508`、`:516-522`（后者说明 `local` 与
  "today" 的断言边界是刻意保留的）。
- 💡 有界改进：把 `:195` 改为 `No local activity today` / `今天没有本地活动`，与
  `menubar.md:508` 对齐；其余 kind 的 `Nothing to break down yet` 保持不变，它本来
  就是 menu bar 没有的 kind 专属文案。

#### 🟢 优点

- **集合是被推导出来的，不是被列举的。** `:24-45` 用"四类事实 ↔ 四个问题"关闭了
  widget 集合，并给出了新增第五个的判定条件（要先给出第五个问题）；`:43-45` 用同一
  把尺子驳回了 per-project / per-session——既是 composition 问题，投影又排除了它需要
  的标识符。这让"为什么是四个"可以被反驳，而不是靠品味。
- **尺寸选深度不选主题（`:50-69`）。** 每个 kind 在三个尺寸上回答同一个问题，大尺寸
  只是证据更多，因而 `:239-242` 的 Dynamic Type 降级规则是自然推论而不是补丁——降一
  级就是降深度。旧稿把 `rhythm` 定成 medium、`trust` 定成 small 的错误也被点名记录。
- **surface/qualifier 真值表对 cache 存在性、版本支持、年龄穷尽（`:165-176`）**，
  且明确不支持的 cache 版本渲染 unavailable 而非尝试部分读取，与 foundation 的
  fail-closed 规则一致。`:178-180` 的"`empty` 是每个 widget 自己的"也堵掉了一个真实
  的错误呈现：没花钱的一天不该让 `rhythm` 显示为空。
- **`trust` 的立论与隐私边界。** `:125-129` 说明未归因成本永不并入头条数字；
  `:247-253` 的三条否定断言（只读投影、无禁用值、绝不写入）与 `architecture.md` 的
  排除清单对齐。
- **timeline 的双向 clamp（`:222-226`）** 给出了两个方向各自的理由，而不只是一个
  区间；`:229` 明确 widget 不轮询、不调用 helper。
- **prototype 与本文档同构。** `ux/prototype/desktop-surfaces.html:584-655` 的
  widget 板块就是四类三尺寸，`:666-675` 记录了旧稿七卡片的问题与尺寸规则——不存在
  R9-F1 那类"散文改了、产物没跟上"的分叉。

#### 📝 总结

评审对象是上述 HEAD 与 blob。文档的推导、尺寸模型、真值表、隐私与 timeline 都具备
实现基础，四项发现里有三项是小范围的对齐问题；使本轮为 FAIL 的是 W-F1：本文档向
投影要的分期间聚合，`architecture.md` 的投影清单没有供给，而 widget 又没有第二条
取数路径，因此 `magnitude` 的三期间对比与 `composition` 的 `Period` 参数目前无法
实现。这不是措辞问题，它决定投影体积与 publication 成本，修复要落在 `architecture.md`。

**跨目标影响，且它推翻了本日更早的一个结论。** W-F1 的根因是投影只承载单一期间，
而 `ux/menubar.md:760` 的 period switcher 行同样声明 `today`/`7d`/`30d` 已被供给。
那一行是 Round 10 对 R9-F2 的修复，并由本人在 Round 13 判为 PASS、随
commit `10ce01e` 一并提交。该 PASS 是错的：R9-F2 的实质是"切换器声称的粒度投影没
供给"，Round 10 把粒度从 week/month 收窄到 7d/30d，但没有回答收窄之后这三个期间
由谁生成——写成"backed by the daily `buckets` series"恰恰指向 Swift 侧求和，即
`menubar.md:55` 自己禁止的那件事。Round 13 核对了这一行与 `widget.md`、
`requirements.md` 的措辞一致性，却没有回到投影清单确认"provisioned"这个词成立，
这与 R9-F2 当初的失效方式完全相同。因此：

- `architecture.md` 与 `ux/menubar.md` 的 Document gate 一并重开，两者的
  `Review` 单元格取消勾选，CEv1 记录改回 `FAILED`；
- 该结论追加为 `reviews/menubar-experience.md` 的 Round 14（更正轮），不改写
  Round 13 已写下的内容；
- 已提交的 commit `10ce01e` 不回滚：它记录的修复本身是真的（specimen 重画、
  `Month` 残留清除、框线与文案对齐），错的只是"因此可以关闭 gate"这一步。

残余不确定性：W-F1 的两条有界修复路线（投影承载三期间，或 widget 只保留当前期间）
在成本上不对称，但本轮没有测量 publication 体积，因此不推荐其一。若选择前者，
`ux/menubar.md` 的对应行随之成立；若选择后者，menu bar 的 period switcher 也要一并
重新设计，因为两个 surface 共用同一份投影。

证据：`git rev-parse HEAD` -> `10ce01e790d5330e632da081cfa681f36cb9e086`；
`git hash-object docs/topics/desktop-app/ux/widget.md` ->
`c317583f8ab9a521c9b49d51c16462f4cb1319fe`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F1 W-F2 W-F3 W-F4
```

## Round 2 — 2026-08-17

- Reviewed state: repair of Round 1's four findings, against
  `docs/topics/desktop-app/ux/widget.md` blob
  `c317583f8ab9a521c9b49d51c16462f4cb1319fe`, the exact blob Round 1 judged.
- Reviewer: claude-code (repair round for Round 1's FAIL — an independent
  Re-review is still required before the `ux/widget.md` Document gate may be
  ticked; this round does not close it and authorizes no commit)
- Scope: W-F1, W-F2, W-F3, W-F4 as named in the repair command.

- W-F1's two bounded paths were both legitimate per Round 1's own text, and
  choosing between them changes `architecture.md`'s projection cost — a
  decision the review explicitly declined to presume. The repair owner asked
  the user directly rather than picking one; the user chose **extend the
  projection to carry three periods** over restricting `widget.md` to a
  single current period. W-F3's field gap was asked in the same pass, given
  its Round-1-stated cost is low and its bounded fix is the same shape (add a
  cheap Go-side reduction to the projection); the user chose to add the two
  fields rather than trim `rhythm`.

- Round 1 findings, dispositions:
  - **W-F1** `Period` and the three-period comparison have no projection
    backing -> **Fixed by extending `architecture.md`'s projection**, per the
    user's chosen path. The App Group projection's data-minimization list now
    carries per-period `totals` (`today`/`7d`/`30d`, each with counts, pricing
    completeness, and cost strings) and per-period top-N model shares (≤ 12
    per period), replacing the prior singular, unscoped totals and model-share
    bullets. A new paragraph in `architecture.md` records why this is a
    second same-day revision, ties it to W-F1 and W-F3 by name, and states
    the bound change: twelve models per period, thirty-six model-share
    entries total across the three periods. `widget.md`'s Data requirements
    table is rewritten to cite the per-period fields explicitly, including a
    row naming that the three-period comparison needs all three periods'
    `totals` in one payload rather than a Swift-side reduction of one.
    `architecture.md:441-472`; `ux/widget.md:255-275`.
  - **Consequential correction, not separately authorized but required by
    W-F1's own resolution:** `ux/menubar.md:760`'s period switcher row
    described its backing as "the daily `buckets` series" — the exact
    Swift-side-summation path `requirements.md:132-133` and `ux/menubar.md:55`
    forbid, and the wording Round 14 of `reviews/menubar-experience.md`
    identified as the unresolved substance of R9-F2. Extending the projection
    makes the row's conclusion true, but the stated mechanism was still wrong
    and would have reproduced the same defect class under an accurate label.
    Reworded to name the per-period totals instead. `ux/menubar.md:760`.
  - **W-F2** `stale`/`aged` reused across two different derivations ->
    **Fixed by renaming the widget's age qualifiers**, per Round 1's first
    offered path (rather than unifying both documents to age semantics, which
    would have reopened `ux/menubar.md` a second time in this same repair).
    Widget-local age tiers are now `aging` (15 min – 6 h) and `old` (> 6 h),
    distinct from the menu bar's refresh-state-derived `stale`/`aged`. The
    displayed *copy* is unchanged and still matches the menu bar's freshness
    wording exactly — only the qualifier identifiers differ — and a new
    paragraph states explicitly why: sharing one name across two different
    derivations invites implementers to share one Swift type across both
    surfaces, which would be wrong on at least one of them. Updated
    everywhere the old names appeared: the qualifier table, the exhaustive
    surface/qualifier table, the Copy table, the timeline-clamp paragraph, the
    accessibility announcement order, and manual checklist item 6.
    `ux/widget.md:142-247,325-326`.
  - **W-F3** `rhythm`'s active-day count and busiest/quietest day names
    unprovisioned -> **Fixed by extending the projection**, per the user's
    chosen path, in the same `architecture.md` edit as W-F1: both fields are
    stated as a one-time Go-side reduction over the already-provisioned
    90-day daily series, not a second query, keeping them outside the
    Swift-aggregation prohibition. Added to `widget.md`'s Data requirements
    table as two explicit rows. `architecture.md:446-449`;
    `ux/widget.md:265-266`.
  - **W-F4** `empty`/`magnitude` copy diverged from the menu bar's fixed
    string by one word -> **Fixed.** `:195`→now in the Copy table's `empty`,
    `magnitude` row: changed from `No activity today` to
    `No local activity today` / `今天没有本地活动`, matching
    `ux/menubar.md:508` exactly — the same correction Round 12 made to
    R11-N2 in the sibling document. `ux/widget.md`'s Copy table.

- Not touched: `ux/prototype/desktop-surfaces.html`'s widget gallery already
  renders the three-period comparison (`:591-622`, cited as W-F1's own
  evidence that the prototype had drawn the gap in), so no prototype edit was
  needed for the chosen path. `trust`'s period-agnostic design and
  `rhythm`'s 30-day-fixed design were unaffected by either decision and were
  not reopened.

- Evidence: `make check-whitespace` passes; `bash scripts/check-topic-docs.sh`
  passes; `git diff --check` passes; no product code, test, or configuration
  was changed. This remains a contract-document repair, spanning three
  documents (`architecture.md`, `ux/widget.md`, `ux/menubar.md`) because
  W-F1's own bounded-fix text named `architecture.md` as the fix location and
  its resolution has a direct, single-line consequence in `ux/menubar.md`.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No
  Document gate is closed and no commit is authorized by this round. This
  also does not itself resolve `reviews/menubar-experience.md`'s Round 14
  reopening — that record's own Document gates need their own independent
  Re-review against the now-extended projection; this round only performs
  the fix Round 14 named as the resolution path.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-widget.md / Round 2
```

## Round 3 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`，以下三份文档均为
  未提交工作区状态：`ux/widget.md` blob
  `faf88b5a9c7fa596ce037cb7e96623402a8434b8`、`architecture.md` blob
  `301f3f6fd0bf12c6330728b587d834ca308a734e`、`ux/menubar.md` blob
  `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`；
  `ux/prototype/desktop-surfaces.html` blob
  `8a8c8e5d16acfa41206ac789429078e92baefe89`（未改动）。
- Reviewer: claude-code（与 Round 2 修复同一 agent；每条处置回到被改动的文本与其
  供给方重新判定）
- Method: 单 agent 有界复评。先按 Round 1 的四条 finding 逐条核对修复落点，再针对
  本次修复的形态做一次专门检查——W-F1 与 W-F3 的修复方式是"给投影补字段"，这类修复
  的典型残留是**只补了 finding 点名的那几个元素，同一参数或同一窗口管辖的其他元素
  没跟上**。因此把 `Period` 参数管辖的每一个展示元素、以及 `rhythm` 涉及的每一个
  时间窗口分别列举核对，而不是只看被点名的行。
- Scope: W-F1、W-F2、W-F3、W-F4 的处置；本次跨三份文档的修复是否引入回归；
  `ux/menubar.md` 与 `architecture.md` 的连带改动是否与其自身契约一致
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮为 FAIL，`ux/widget.md` 的 Document gate 不关闭。
  `architecture.md` 内容已被本次修复改变，其在 `reviews/menubar-experience.md`
  Round 14 被重开的 gate 仍未关闭，且现在绑定的是一个尚未被复评的新内容状态。

#### 🔴 严重问题 — 必须修复

**W-F5 — `docs/topics/desktop-app/ux/widget.md:110,281`: `composition` large 的
per-client subtotals 没有随 `Period` 分期间供给，与 W-F1 同类且同一参数。**

- 处置：**新发现。** W-F1 的修复把 `totals` 与 top-N model shares 改成了分期间，
  但 `composition` large 展示的第三样东西——per-client subtotals——没有一起改。
- 行为风险：`:85` 让 `composition` 接受 `today`/`7d`/`30d`；`:110` 让它的 large
  尺寸显示 per-client subtotals。当用户把 `Period` 设为 `7d` 或 `30d`，该行没有
  对应数据：投影的 per-client subtotals 是单份、不分期间的。实现者仍然只剩 W-F1
  已经排除干净的那三条路——Swift 侧按日桶求和（日桶不带 client 维度，因而根本不可
  能）、调用 helper（widget 不能）、或渲染不出来。也就是说 W-F1 的阻断在
  `composition` large 上原样存在，只是换了一个字段。
- 证据：`architecture.md:455-456` 的 per-client/per-provider subtotals 条目没有
  period 限定，本次修复也未触及它（`git diff` 显示该行未变）；`ux/widget.md:281`
  的 Data requirements 行仍是 `composition` client rows | per-client subtotals，
  是本次重写 Data requirements 表时唯一没有加期间限定的 `composition` 行——同表
  `:280` 的 model rows 已改为"per-period top-N model shares, ≤ 12 per period"。
- 💡 有界修复：与 W-F1 选定的路线保持一致，把 per-client subtotals 也改为分期间
  供给（`architecture.md` 条目加期间限定并声明上界），同时更新 `:281`；或者把
  `composition` large 的 client 行限定为当前期间并在文档中写明该例外。前者与已选
  路线一致，后者需要说明为什么同一个 widget 的两个维度期间语义不同。

#### 🟡 建议改进 — 推荐

**W-F6 — `docs/topics/desktop-app/ux/widget.md:88,135,283-284`: `rhythm` 展示的是
30 天窗口，W-F3 补进投影的两个字段却是 90 天窗口。**

- 处置：**新发现。** W-F3 的修复补了字段，但补进来的窗口与消费它的展示不是同一个。
- 行为风险：`:88` 明确 `rhythm` "is always the last 30 days"，`:135` 让 small 显示
  "Active-day count over the last 30 days"；而 `architecture.md:446-449` 与本文件
  `:283-284` 供给的是"over the same 90-day window"/"over the 90-day window"。90 天
  的活跃天数是一个标量，无法还原出 30 天的活跃天数；要得到后者只能在 Swift 侧数
  日桶，正是 W-F3 修复所要避免的那件事。`:137` 的 large 又确实用 90 天热力图，因此
  文档内部对 `rhythm` 的窗口本就有两种说法（`:88` 的 30 天与 `:137` 的 90 天），
  本次修复把这一模糊固化进了投影契约。
- 证据：`ux/widget.md:88`、`:135`、`:137`、`:283-284`；`architecture.md:446-449`；
  `:39` 只写"active days"而不带窗口。
- 💡 有界改进：为 `rhythm` 的每个元素各自写明窗口，并让投影供给同一个窗口——最省的
  做法是把两个新字段改为 30 天窗口（与 `:88`、`:135` 一致），把 `:137` 的 90 天
  热力图作为显式例外写明它用的是已供给的 90 日桶本身。

#### 🟢 优点

- **W-F1 已关闭（就其点名的两个元素而言）。** `architecture.md:441-449` 的投影清单
  现在写明"per-period usage totals for the three supported periods — `today`,
  `7d`, `30d` — each with counts, pricing completeness, and cost strings"与
  "per-period top-N model shares … for each of the three supported periods"，
  `:469-482` 记录了为什么这是同日第二次修订、点名了 W-F1 与 W-F3，并把上界改写为
  "twelve models per period, thirty-six model-share entries total"——上界随契约一起
  更新，而不是留给实现者推算。`ux/widget.md:277-286` 的 Data requirements 表随之
  重写，并新增一行明确"三期间对比需要三份 `totals` 在同一份 payload 里，而不是对
  其中一份做 Swift 侧归约"。这一行把 W-F1 的失效方式写进了契约本身。
- **连带修正了 `ux/menubar.md:760` 的机制描述，而不只是它的结论。** 原文"backed by
  the daily `buckets` series"正是 Round 14 认定的 R9-F2 实质；现在改为"backed by
  the projection's per-period totals — not a Swift-side reduction over the daily
  `buckets` series"。扩投影本可以让原结论"变真"，修复没有就此收手，而是把错误的
  机制一并改掉——否则同一类缺陷会在一个正确的标签下复现。
- **W-F2 已关闭，且关闭方式比原建议更保守。** widget 的年龄档改名为 `aging`/`old`
  （`:163-164`、`:178-179`），与 menu bar 由刷新状态推导的 `stale`/`aged` 区分开；
  `:144-155` 写明了理由——同名不同推导会诱使实现者在两个 surface 上共用一个 Swift
  类型，而它至少在其中一个上是错的。**显示文案保持不变**（`:198-199` 仍是
  `Updated <relative>` / `Last updated <relative>`），因此"用户不应为同一状况读到
  两句不同的话"这个原始理由没有被牺牲——分歧只落在标识符上。改名覆盖完整：
  qualifier 表、穷尽真值表、Copy 表、timeline clamp 段（`:232`）、无障碍播报顺序
  （`:246-247`）、manual checklist 第 6 项（`:325-326`）全部同步，文件内再无遗留的
  旧标识符（`:148`、`:150` 的 `stale`/`aged` 是刻意引用 menu bar 的名字）。
- **W-F4 已关闭。** `:201` 改为 `No local activity today` / `今天没有本地活动`，与
  `ux/menubar.md:508` 逐字一致；其余 kind 的 `Nothing to break down yet` 未动，
  它本就是 menu bar 没有的 kind 专属文案。
- **W-F3 的字段确实进了契约。** `architecture.md:446-449` 把 active-day count 与
  busiest/quietest day names 写成"a one-time reduction over the daily series the
  producer already holds, not a second query"，理由与既有的日序列、7×24 网格同类，
  没有把它包装成新能力。窗口不一致是 W-F6，字段供给本身成立。
- **修复对 `Period` 无关的部分保持了边界。** `trust` 的期间无关设计与 `rhythm` 固定
  30 天的设计都未被顺手改动，prototype 也未改——因为它本就画着三期间对比，是 W-F1
  的证据而非受害者。

#### 📝 总结

逐条处置：W-F1 关闭（就其点名的 `totals` 与 model shares 而言）、W-F2 关闭、
W-F3 的字段供给成立、W-F4 关闭；无回归。本轮为 FAIL，因为修复的形态留下了两处
同源残留：W-F5 是 `Period` 管辖的第三个元素 per-client subtotals 没有跟着分期间，
W-F6 是 W-F3 补进来的两个字段用的是 90 天窗口而消费它的展示写的是 30 天。

两者与 W-F1、W-F3 是同一个失效方式，只是换了元素：**修复回答了 finding 点名的那
几行，没有回答"同一个参数/同一个窗口还管辖哪些行"。** 这也是本 topic 反复出现的
形态——R9-F2 收窄了粒度却没回答由谁生成，Round 10 修了散文却漏了 prototype 的
`Month`，Round 12 修了框线却是 R11 才发现文案。修复边界应当由"受同一决定支配的
元素集合"划定，而不是由 finding 的行号划定。

跨目标状态：`architecture.md` 与 `ux/menubar.md` 在本次修复中都被改动，两者的
Document gate 自 `reviews/menubar-experience.md` Round 14 起本就重开，现在还各自
绑定了一个未被复评的新内容状态。W-F5 的修复很可能再次落在 `architecture.md`，
因此这两份文档的独立复评应当在 W-F5、W-F6 关闭之后一并进行，而不是现在。

残余不确定性：W-F6 给出的最省做法（把两个新字段改为 30 天窗口）假定 `:137` 的
90 天热力图直接用已供给的 90 日桶渲染。若 large 的 busiest/quietest 确实想描述
90 天，则 `rhythm` 需要同时声明两个窗口，那是设计取舍而非本轮可代为决定的。

证据：`git rev-parse HEAD` -> `10ce01e790d5330e632da081cfa681f36cb9e086`；
`git hash-object` -> `ux/widget.md` `faf88b5a…`、`architecture.md` `301f3f6f…`、
`ux/menubar.md` `6c3bfed4…`；`bash scripts/check-topic-docs.sh` -> exit 0；
`make check-whitespace` -> exit 0；`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F5 W-F6
```

## Round 4 — 2026-08-17

- Reviewed state: repair of Round 3's two findings, against `ux/widget.md`
  blob `faf88b5a9c7fa596ce037cb7e96623402a8434b8`, `architecture.md` blob
  `301f3f6fd0bf12c6330728b587d834ca308a734e`, and `ux/menubar.md` blob
  `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f` — the exact three blobs Round 3
  judged, confirmed matching by `git hash-object` before editing.
- Reviewer: claude-code (repair round for Round 3's FAIL — an independent
  Re-review is still required before any Document gate may be ticked; this
  round does not close one and authorizes no commit)
- Scope: W-F5 and W-F6 as named in the repair command.

- Round 3 findings, dispositions:
  - **W-F5** `composition` large's per-client subtotals not period-scoped ->
    **Fixed by splitting `architecture.md`'s shared bullet**, consistent with
    W-F1's chosen path. The single "per-client and per-provider subtotals,
    each with its attribution quality counts" bullet served two consumers
    with different period semantics — `composition` (takes `Period`) and
    `trust` (never takes `Period`) — which Round 3 named as the reason a
    uniform per-period change would be wrong. Split into: (1) per-period
    per-client subtotals for the three supported periods, serving
    `composition`'s client rows; (2) per-client/per-provider
    attribution-quality subtotals for the current period only, serving
    `trust`'s quality rows unchanged. A new paragraph records why the split
    rather than a uniform change: making the whole bullet per-period would
    have given `trust` two periods of data it never asked for and cannot
    display. `ux/widget.md`'s Data requirements table and the `trust` quality
    rows entry are updated to name the current-period scope explicitly.
    `architecture.md:452-460,491-499`; `ux/widget.md:266-281`.
  - **W-F6** rhythm's new fields used a 90-day window while consuming prose
    stated 30 days -> **Fixed via the review's own stated cheapest path**:
    the active-day count and busiest/quietest day names are rescoped from 90
    days to 30, matching `ux/widget.md:88`'s stated `rhythm` default and
    `:135`'s explicit 30-day statement for the active-day count. The 90-day
    heatmap needs no separate field — it renders directly from the
    already-provisioned bounded daily series — so nothing here serves it.
    `:137`'s prose is reworded: it had put "the quietest and busiest day
    names" in the same sentence as "the 90-day daily heatmap," reading as a
    description of that heatmap's own window even though the field behind it
    is the small tier's 30-day figure; now states explicitly that it is the
    same last-30-days figure as small, not a description of the heatmap
    beside it. `architecture.md:447-451,501-508`; `ux/widget.md:135-137,
    268-270,284-285`.

- Residual the fix does not resolve, recorded per Round 3's own note rather
  than decided here: Round 3 flagged that if large's busiest/quietest day
  names were *meant* to describe the 90-day heatmap rather than the 30-day
  default, `rhythm` would need to declare two windows, which it called "a
  design tradeoff, not this round's to decide." The chosen fix follows the
  review's own stated cheapest path (30 days) rather than reopening that
  tradeoff; if a future round decides the 90-day reading was intended, it
  will need a fifth field, not a wording change.

- Verification performed: re-checked, for every element `Period` governs
  (`composition` model rows, client rows, and the `magnitude` three-period
  comparison) and every element `rhythm`'s two windows govern (grid, small
  active-day count, large heatmap, large busiest/quietest), that the Data
  requirements table's stated field matches what `architecture.md` now
  provisions — the same element-by-governing-parameter sweep Round 3 used to
  find W-F5 and W-F6, repeated here rather than re-checking only the two
  named findings' own lines.

- Evidence: `make check-whitespace` passes; `bash scripts/check-topic-docs.sh`
  passes; `git diff --check` passes; no product code, test, or configuration
  was changed. This remains a contract-document repair, spanning
  `architecture.md` and `ux/widget.md`; `ux/menubar.md` needed no edit for
  either finding (verified: no menu-bar text references `rhythm`'s day-named
  fields, and its client-tab subtotal requirement is written without a
  period claim, so the projection split is a superset for it, not a change).
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No
  Document gate is closed and no commit is authorized by this round.
  `architecture.md` and `ux/menubar.md` remain reopened from
  `reviews/menubar-experience.md` Round 14, now bound to content this round
  changed again; that record's own gates still need their own independent
  Re-review, together with this one, per Round 3's note that both should be
  reviewed together once W-F5/W-F6 close rather than separately.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-widget.md / Round 4
```

## Round 5 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`，以下均为未提交
  工作区状态：`ux/widget.md` blob `46c70e0e8fa919487824d60b235f40c84aab90a0`、
  `architecture.md` blob `95e53db6262746c1009a72f78424189a2557bff5`、
  `ux/menubar.md` blob `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`（Round 4 未改动，
  与 Round 3 判定的一致）、`ux/prototype/desktop-surfaces.html` blob
  `8a8c8e5d16acfa41206ac789429078e92baefe89`（未改动）。
- Reviewer: claude-code
- Method: 单 agent 有界复评。W-F5、W-F6 逐条回到被改动的文本与投影清单核验。随后
  重复 Round 3 用来发现这两项的那次扫描——**按"受同一决定支配的元素集合"逐个走查，
  而不是只看被点名的行**——并把它扩展到这次修复引入的新分界：`Period` 管辖的每个
  元素、`rhythm` 两个窗口管辖的每个元素、以及本次新拆出的"per-period 客户端小计"与
  "当前期间归因质量小计"两个 bullet 各自的消费者。
- Scope: W-F5、W-F6 的处置；Round 4 跨两份文档的修复是否引入回归；`Period` 与
  `rhythm` 窗口扫描；新拆分 bullet 的消费者覆盖
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，`ux/widget.md` 的 Document gate 不关闭；
  `architecture.md` 再次被改动，其自 `reviews/menubar-experience.md` Round 14 起
  重开的 gate 仍未关闭。

#### 🔴 严重问题 — 必须修复

**W-F8 — `docs/topics/desktop-app/ux/widget.md:121-122,129,280`: `trust` 每一档
显示的是**金额**，投影只供给**计数**，而本文件自己的 Data requirements 行也只要了
计数。**

- 处置：**新发现。** 与 W-F1、W-F5 同类，且这次的不一致落在本文档内部：body 要的
  东西和它自己的需求表要的东西不是一回事。
- 行为风险：`:121` 让 small 显示"the inferred and unattributed **amounts**"，
  `:122` 让 medium 显示三档"with their own **amounts** and shares"，`:129` 更把
  "Unattributed cost is shown as its own amount"作为本 widget 存在的理由。而
  `architecture.md:458-460` 供给的是"determinable, inferred, unattributed
  **counts**"——按质量档的**条数**，不是金额。投影里没有第二处能提供按档金额：
  per-period totals 是聚合值，per-period 客户端小计按 client 分，model shares 按
  model 分，都不带质量档维度。实现者因此只能自行发明一个未入契约的字段，或者把
  counts 当成金额显示——后者会让本文档 `:125-129` 声称要消除的那类错误数字
  重新出现在它自己的界面上。
- 证据：`architecture.md:458-460` 的 bullet 全文只有 counts；
  `rg 'unattributed|determinable|inferred'` 在 `architecture.md` 只命中该 bullet 与
  其理由段（`:496`），全文件无按档金额；本文件 `:280` 的 Data requirements 行写的是
  "per-client and per-provider attribution counts, current period only"，与 `:121`
  `:122` `:129` 的金额需求直接矛盾。
- 💡 有界修复：与 W-F1、W-F5 已选路线一致，在 `architecture.md` 的归因质量 bullet
  里明确每档同时携带 cost 与 count（`(quality, cost, tokens, count, share)` 一类的
  形状），并把本文件 `:280` 改为同时点名金额与计数；或者把 `trust` 各尺寸改为只
  展示条数与占比，并同步改写 `:129` 的立论。前者与本文档的产品主张一致，后者会
  削弱它，但两者都必须在文档里选定。

#### 🟡 建议改进 — 推荐

**W-F7 — `docs/topics/desktop-app/ux/widget.md:268`: 引用了一个按 `architecture.md`
自己的编号并不存在的修订序号。**

- 处置：**新发现，非阻塞。**
- 行为风险：Data requirements 表的表头作用是把本表绑定到供给方的一个确切状态；
  序号对不上时，读者无法确定该表对应哪一次修订。不影响任何字段语义。
- 证据：本文件 `:268` 写"`architecture.md`'s **fourth** 2026-08-17 revision"；
  `architecture.md` 自述只有三次——`:477` 的 "a second, same-day revision"、
  `:491` 的 "a third, same-day revision"，而 rhythm 窗口变更被明确记为
  "changed window in the **same pass**"（即并入第三次），加上最初那次共三次。
- 💡 有界改进：把 `:268` 改为 "third"，或在 `architecture.md` 把窗口变更单列为第四次
  修订。两份文档采用同一套编号即可。

#### 🟢 优点

- **W-F5 已关闭，且关闭方式比原建议更精确。** Round 3 给的两条路线都是"整条 bullet
  怎么办"，Round 4 发现该 bullet 其实服务两个期间语义不同的消费者，于是把它拆成两条：
  `architecture.md:456-457` 的 per-period 客户端小计（服务 `composition` large 的
  client 行）与 `:458-460` 的当前期间归因质量小计（服务 `trust`）。`:491-499` 记录了
  为什么拆而不是整体改：整体 per-period 会给 `trust` 两个它从不请求也无法显示的期间。
  本文件 `:279-280` 两行随之分别写明 "one set per supported period" 与 "current
  period only"。这正是 Round 3 指出的失效方式的反面——按消费者而不是按 bullet 边界
  划定修复。
- **W-F6 已关闭，且把导致歧义的那句话本身改掉了。** `architecture.md:447-451` 把两个
  归约字段的窗口从 90 天改为 30 天，并写明"the same window `rhythm` defaults to
  everywhere else"；`:501-508` 记录了原因。更关键的是 `ux/widget.md:137` 的措辞：
  原文把"quietest and busiest day names"与"the 90-day daily heatmap"放在同一句，读起来
  像是在描述那张热力图的窗口，现在明确写成"for the same last-30-days window as small
  — a separate figure from the heatmap beside it, not a description of it"。修复没有
  只改数字，而是改掉了让人读错的那句话。
- **90 天热力图不需要新字段这一点被明确记录。** `:504-506` 说明它直接由已供给的
  90 日序列渲染，因此两个新字段只服务 30 天档——避免了下一轮再问"那 90 天那份呢"。
- **Round 4 自述做了元素级扫描，且该扫描确实抓到了本轮之外的东西。** 它逐一核对了
  `Period` 与 `rhythm` 两个窗口管辖的每个元素；本轮独立重做同一扫描，`Period` 侧
  （magnitude small/medium/large、composition small/medium/large 六个元素）与
  `rhythm` 侧（grid、small 活跃天数、large 热力图、large 最忙/最闲日名四个元素）
  的字段与供给逐条对上，没有遗漏。W-F8 落在 `trust`——一个不受 `Period` 也不受
  `rhythm` 窗口支配的维度，因此不在该扫描的覆盖范围内。
- **`ux/menubar.md` 确实不需要为这两项改动。** 独立核实：其 `:765` 的 Trust 行与
  `:768` 的 Client tabs 行都没有期间声明，因此投影拆分对它是超集而非变更；
  menu bar 文本也不引用 `rhythm` 的按日字段。Round 4 的"未改动"判断成立。
- **W-F1~W-F4 无回归。** 逐项复看：per-period totals 与 model shares（`:441-455`）、
  `aging`/`old` 改名的六处同步、`No local activity today` 文案、Data requirements 的
  三期间行均保持 Round 3 判定时的状态。

#### 📝 总结

逐条处置：W-F5 关闭（按消费者拆分 bullet，比原建议更精确）、W-F6 关闭（窗口与措辞
一并改正）；W-F1～W-F4 无回归；Round 4 未改动 `ux/menubar.md` 的判断经独立核实成立。
本轮 FAIL，因为新增两项：W-F8（阻断）与 W-F7（非阻塞）。

W-F8 与 W-F1、W-F5 是同一个问题的第三次出现：**某个展示元素要的东西，投影没有供给。**
前两次分别由"`Period` 管辖哪些元素"和"同一条 bullet 服务哪些消费者"暴露；这一次落在
`trust`，它既不受 `Period` 支配也不受 `rhythm` 窗口支配，因此前两轮的扫描维度都覆盖
不到它。可以据此收敛的判断是：**该按"Data requirements 表的每一行 ↔ 投影清单的某一
条 bullet"做一次一对一映射，逐行确认字段形状（不只是字段名）能支撑该行所属尺寸的
展示内容**，而不是每次沿着上一轮暴露出的那个维度再扫一遍。金额与计数的差别正是"形状"
而非"名字"层面的，按名字扫描（"attribution counts" 对 "attribution counts"）会显示一致。

跨目标：`ux/menubar.md:179` 与其 specimen `:243-244` 同样按质量档显示金额
（`Determinable $11.90`、`Inferred $0.57`），因此 W-F8 的供给侧修复一旦落地，
menu bar 侧无需再改；但若选择第二条路线（只展示条数与占比），menu bar 的 Trust 段与
specimen 必须一并重画。该项归属 `reviews/menubar-experience.md`，其 gate 自 Round 14
起已重开，不计入本文档的未关闭项。另记：menu bar 的 period switcher 究竟管辖哪些
section 仍未写明——client tabs 明确"filter every section at once"，period switcher 没有
对应句子。投影拆分后这个问题变得更尖锐（归因质量只有当前期间），同样归属该记录。

残余不确定性：W-F8 的两条路线中，第一条需要确认 `usage stats` 是否已经按质量档输出
成本；本轮未核验 CLI 侧，因为这属于修复时的可行性检查而非本轮的判定依据——文档层面
的缺口无论 CLI 是否已具备该能力都成立。

证据：`git rev-parse HEAD` -> `10ce01e790d5330e632da081cfa681f36cb9e086`；
`git hash-object` -> `ux/widget.md` `46c70e0e…`、`architecture.md` `95e53db6…`、
`ux/menubar.md` `6c3bfed4…`；`bash scripts/check-topic-docs.sh` -> exit 0；
`make check-whitespace` -> exit 0；`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F7 W-F8
```

## Round 6 — 2026-08-17

- Reviewed state: repair of Round 5's two findings, against `ux/widget.md`
  blob `46c70e0e8fa919487824d60b235f40c84aab90a0` and `architecture.md` blob
  `95e53db6262746c1009a72f78424189a2557bff5` — the exact two blobs Round 5
  judged, confirmed matching by `git hash-object` before editing.
  `ux/menubar.md` was not reviewed as needing a change by Round 5 and was not
  touched by this repair.
- Reviewer: claude-code (repair round for Round 5's FAIL — an independent
  Re-review is still required before any Document gate may be ticked; this
  round does not close one and authorizes no commit)
- Scope: W-F7 and W-F8 as named in the repair command.

- Round 5 findings, dispositions:
  - **W-F8** `trust` displays quality-tier amounts but the projection supplied
    only counts -> **Fixed by extending `architecture.md`'s attribution-quality
    bullet**, the path Round 5 itself named as consistent with W-F1/W-F5's
    already-chosen "supply more from the projection" direction. Each quality
    tier (determinable, inferred, unattributed) now carries
    `(cost, tokens, count, share)` instead of a bare count, matching the shape
    already used by the projection's other per-dimension breakdowns (model
    shares, client subtotals). Scope stays current-period-only, for the same
    reason W-F5's split gave it that scope. `ux/widget.md`'s Data requirements
    row for `trust` is rewritten to name the full shape and to cover both the
    small/medium amount display and large's per-provider rows explicitly,
    closing the exact mismatch Round 5 found between the row's own stated need
    (counts) and the body's stated need (amounts). Did not verify whether
    `usage stats` already emits cost broken out by attribution quality — Round
    5 explicitly filed that under repair-time feasibility, not this round's
    documentation judgment. `architecture.md:458-460,510-521`;
    `ux/widget.md:264-281`.
  - **W-F7** `ux/widget.md`'s Data requirements header cited a revision
    ordinal (`fourth`) `architecture.md` never used for itself -> **Fixed by
    making both documents' ordinals explicit and consistent**, rather than
    picking whichever single number happened to be right first. Round 5's own
    evidence showed the mismatch was really that `architecture.md` had never
    given W-F6's window change its own ordinal — it called the change "the
    same pass" as W-F5's split without saying which numbered revision that
    was. Named it explicitly as `architecture.md`'s fourth revision, and, since
    this same round adds W-F8's shape change, named that a fifth. Updated
    `ux/widget.md:268` to cite "fifth" to match. Both documents now use one
    shared, explicit sequence: second (W-F1), third (W-F5), fourth (W-F6),
    fifth (W-F8). `architecture.md:502-521`; `ux/widget.md:268`.

- Not touched: `ux/menubar.md`. Round 5's own cross-target note said its Trust
  section and specimen already display quality-tier amounts
  (`Determinable $11.90`, `Inferred $0.57`), so W-F8's supply-side fix needs
  no matching edit there; that would only be required under the fix's second,
  not-chosen path (counts-only display). Verified this repair round: `rg
  'Determinable|Inferred|Unattributed'` in `ux/menubar.md` shows only
  dollar-amount specimens, consistent with a document that already assumed
  the shape this repair just made real.

- Verification performed: re-ran the element-by-governing-parameter sweep
  Round 3 and Round 5 both used, extended to the new `(cost, tokens, count,
  share)` shape — confirmed no other Data requirements row still asks for a
  bare count where the body it serves needs an amount, and no other
  `architecture.md` bullet was left citing a stale ordinal.

- Evidence: `make check-whitespace` passes; `bash scripts/check-topic-docs.sh`
  passes; `git diff --check` passes; no product code, test, or configuration
  was changed. This remains a contract-document repair spanning
  `architecture.md` and `ux/widget.md` only.
- Verdict: REOPEN — repair complete, awaiting independent Re-review. No
  Document gate is closed and no commit is authorized by this round.
  `architecture.md` remains reopened from `reviews/menubar-experience.md`
  Round 14, now bound to content this round changed again.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-widget.md / Round 6
```

## Round 7 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：7/10

✅ 结论：FAIL

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`，以下均为未提交
  工作区状态：`ux/widget.md` blob `1e44c3e915b70611fa6c253d384e20d3eab21cdb`、
  `architecture.md` blob `34ca107974e4d9623db9a62703127cd041dd8987`、
  `ux/menubar.md` blob `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`（未改动）、
  `ux/prototype/desktop-surfaces.html` blob
  `8a8c8e5d16acfa41206ac789429078e92baefe89`（未改动）。
- Reviewer: claude-code
- Method: 单 agent 有界复评。W-F7、W-F8 逐条核验后，执行 Round 5 结论里点名的那次
  收敛检查——**把 Data requirements 的每一行与投影清单的某一条 bullet 做一对一映射，
  逐行确认字段的形状而非名字**——并把它扩展到 Round 5 未覆盖的那一维：`Client`
  配置参数。前几轮分别沿 `Period`（Round 3）与 `rhythm` 窗口（Round 5）扫描，
  `Client` 是本文档三个支配维度中唯一没有被任何一轮走查过的。
- Scope: W-F7、W-F8 的处置；Round 6 的修复是否引入回归；Data requirements 全表的
  行↔bullet 形状映射；`Client` 参数管辖的元素集合
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，`ux/widget.md` 的 Document gate 不关闭；
  `architecture.md` 再次被改动，其自 `reviews/menubar-experience.md` Round 14 起
  重开的 gate 仍未关闭。

#### 🔴 严重问题 — 必须修复

**W-F9 — `docs/topics/desktop-app/ux/widget.md:73-83`: `Client` 参数管辖全部四类
widget 的全部尺寸，但投影按 client 分的只有三样东西，其余全部只有聚合值。**

- 处置：**新发现。** 与 W-F1、W-F5、W-F8 同类，落在第三个、也是最后一个支配维度上。
- 行为风险：`Client` 有 `all`/`codex`/`claude` 三个取值且每个 widget 都带它
  （`:73-80`），因此每个 widget 的每个尺寸都必须能在单客户端下渲染。按 client 分的
  供给只有三条：token 分量的 client 级（`architecture.md:443-444`）、per-period
  per-client 小计的 cost 与 token 合计（`:456-457`）、以及归因质量小计
  （`:458-461`）。其余全部只有全局聚合，因此在 `Client = codex` 下：
  - `magnitude` small 的 7 桶 sparkline、medium 的 20 桶柱状图、large 的 90 桶图与
    `avg/day`/`peak`/cache-hit 全部无数据——日序列（`:445-446`）不按 client 分；
    其 session 计数也只有"aggregate session availability and count"（`:463`）。
  - `composition` 三个尺寸全部无数据——top-N model shares（`:453-455`）按 model 键，
    不按 client 分，因此单客户端的模型份额无法得出。
  - `rhythm` 三个尺寸全部无数据——7×24 网格、活跃天数、最忙/最闲日名（`:447-452`）
    都是全局聚合。
  - `trust` medium/large 的 pricing coverage（`:462-463`）是全局值，只有质量档行
    本身按 client 分。
  实现者只剩 W-F1 已排除干净的那三条路：Swift 侧再聚合（且多数情况下连原始维度都
  没有，做不到）、调用 helper（widget 不能）、或渲染不出来。
- 证据：`rg 'per-client|client level|client identifiers' architecture.md` 在投影段
  只命中 `:440`、`:444`、`:456`、`:458` 四处，其中 `:440` 只是 client 标识符清单；
  本文件 Data requirements 表（`:271-284`）十三行中只有 `composition` client rows
  与 `trust` quality rows 两行带 client 维度，其余十一行都没有，而 `Client` 参数
  按 `:73-80` 管辖全部十二个配置。
- 💡 有界修复：与 W-F1、W-F5、W-F8 已选定的"供给侧补齐"路线一致，在
  `architecture.md` 里为每一项需要按 client 呈现的聚合声明 client 维度，并给出
  上界（client 是封闭小集合，成本主要在日序列与 7×24 网格上：3 份日序列与 3 份网格
  而不是 1 份）；或者把 `Client` 限定为它当前真正能支撑的范围——例如只作用于
  `trust` 与 `magnitude` 的 cost/token 头条，并在文档中明写其余 widget 忽略该参数
  或不提供该参数。后者更省，但会让"每个 widget 一个 `Client` 参数"这句话不再成立，
  需要一并改写 `:73`。本轮不作推定：这同样是投影体积与 publication 成本的取舍。
- 附带：`ux/menubar.md:182-183` 的 client tabs 声明"filter every section at once"，
  面对的是同一处缺口且范围相同；它归属 `reviews/menubar-experience.md`，其 gate
  自 Round 14 起已重开，不计入本文档的未关闭项。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **W-F8 已关闭，且形状与清单里其他分维度一致。** `architecture.md:458-461` 现在把
  三档写成 `(cost, tokens, count, share)`，与 model shares 的 `(model, tokens,
  cost, share)` 同构；`:510-521` 记录了理由，并逐条排除了"别处是否已有按档成本"
  ——per-period totals 是单一聚合、model shares 按 model 键、client 小计按 client
  键，都没有质量档维度。这正是本轮要求的"按形状而非名字"的论证方式。
  `ux/widget.md:279` 的行也改写为同时点名 small/medium 的金额与 large 的
  per-provider 行，Round 5 指出的"文档自己的需求表与自己的 body 互相矛盾"已消除。
- **W-F7 已关闭，且修的是编号本身而不是挑一个数字。** Round 5 的证据显示真正的问题
  是 `architecture.md` 从未给 W-F6 的窗口变更编号（只说"same pass"）。修复把它显式
  记为第四次、把本轮的形状变更记为第五次，两份文档现在共用一套编号：第二次
  （W-F1）、第三次（W-F5）、第四次（W-F6）、第五次（W-F8），`ux/widget.md:268`
  引用第五次。歧义的来源被消掉了，而不是被绕过。
- **修复对未选路线保持了边界。** `ux/menubar.md` 未改动，且理由经独立核实成立：其
  Trust 段与 specimen 本就按质量档显示金额，供给侧修复对它是超集。修复轮明确记录了
  "未核验 `usage stats` 是否已按质量档输出成本"，把可行性与文档判定分开——这是诚实的
  边界声明，不是遗漏。
- **W-F1～W-F6 无回归。** Data requirements 十三行逐行回到投影清单核对形状：
  per-period totals、三期间对比、日序列、每期间 avg/peak、cache-hit、per-period
  model shares、per-period client 小计、质量档四元组、pricing coverage、7×24 网格、
  30 天活跃天数、30 天最忙/最闲日名、freshness——在 `Client = all` 下全部对得上。

#### 📝 总结

逐条处置：W-F8 关闭、W-F7 关闭，W-F1～W-F6 无回归。本轮 FAIL，因为按 Round 5 自己
点名的收敛检查执行时，在 `Client` 维度上发现 W-F9：该参数管辖全部十二个配置，而投影
按 client 分的只有三样东西。

这是同一问题的第四次出现，但它也把问题的边界划完了。本文档的支配维度只有三个——
`Client`、`Period`、`rhythm` 的时间窗口——外加 kind × size 的展示矩阵。`Period` 在
Round 3 走查、窗口在 Round 5 走查、`Client` 在本轮走查。因此**下一轮修复应当一次性
关闭整个参数空间，而不是再沿一个维度补一次**：以 `Client`（3）×`Period`（3，仅对
`magnitude` 与 `composition`）×kind（4）×size（3）为矩阵，对每个格子确认它显示的
每一个数字在投影里有对应形状的字段。交叉项尤其要看——例如 `composition` 在
`Client = codex` 且 `Period = 7d` 下需要"按客户端且按期间的 model shares"，这是两个
维度的乘积，补齐任一单维都不够。

残余不确定性：W-F9 的两条路线成本差距明显（按 client 分的日序列与 7×24 网格会把这
两项体积乘以客户端数），而本轮同样没有测量 publication 体积。若选择限定 `Client`
作用域的第二条路线，`ux/menubar.md` 的 client tabs 也必须同步限定，因为两个 surface
共用同一份投影且该文档目前声明 tabs 过滤所有 section。

证据：`git rev-parse HEAD` -> `10ce01e790d5330e632da081cfa681f36cb9e086`；
`git hash-object` -> `ux/widget.md` `1e44c3e9…`、`architecture.md` `34ca1079…`、
`ux/menubar.md` `6c3bfed4…`；`bash scripts/check-topic-docs.sh` -> exit 0；
`make check-whitespace` -> exit 0；`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F9
```

## Round 8 — 2026-08-17

- Reviewed state: HEAD `10ce01e790d5330e632da081cfa681f36cb9e086`；修复后的未提交
  工作区状态：`ux/widget.md` blob `77b08af1…`、`architecture.md` blob
  `24e095d1…`、`ux/menubar.md` blob `6c3bfed4…`（未改动）、
  `ux/prototype/desktop-surfaces.html` blob `8a8c8e5d…`（未改动）。修复前的三个
  blob 与 Round 7 声明逐字节一致。
- Reviewer: claude-code（修复轮 — Document cell 勾选前仍需独立复评）
- Scope: W-F9，如修复命令点名。

- Round 7 finding，处置：
  - **W-F9** `Client` 参数管辖全部十二个配置，投影按 client 分的只有三样东西 ->
    **已修复，走供给侧补齐路线。** Round 7 给了两条路线且明确"本轮不作推定"，因为
    取舍点是 publication 体积而它未被测量。**本轮测了**：按 client 分之后的条目数
    上界是 906，当前是 309，约 2.9 倍；绝对量在一个不含任何 per-event 数据的聚合上
    仍然很小。第二条路线（限定 `Client` 作用域）要改写 `:73` 的"每个 widget 一个
    `Client` 参数"、让四类 widget 中三类忽略该参数，并连带限定 `ux/menubar.md` 的
    client tabs——为省下 600 个条目而让一个已声明的配置参数在多数配置下失效，不成
    比例。因此与 W-F1、W-F5、W-F8 已选定的路线保持一致。
    `architecture.md` 的九条 bullet 加上 client scope：日序列、7×24 网格加两个
    rhythm 标量、model shares、per-period totals 与 session count、pricing
    coverage。**其中两条取的是两个参数的乘积而非各自一份**——`composition` 的 model
    shares 与 `magnitude` 的 per-period totals 同时受 `Period` 与 `Client` 管辖，
    只补任一单维都会把交叉项留在原地，这正是 Round 7 总结里点名要求的检查。
    `ux/widget.md` 的 Data requirements 表新增 `Varies by` 一列，逐行写明每行受哪
    个（或哪两个）参数管辖，并在表头写明"每一行都在配置的 `Client` scope 下读取，
    `all` 是一个 scope 而不是没有 scope"。
    一处例外显式记录：`composition` 的 client rows 本身就是按 client 的枚举，在单
    客户端下只剩一行，这是本文档的展示选择而非第二个字段。
    上界段随契约一起更新，写出三个 scope 下的绝对数（270 桶、504 格、108 条），并
    规定截断在每个 scope 内独立进行，避免一个繁忙客户端吃掉另一个的预算。
- 连带影响，均由该 finding 驱动：`architecture.md` 的修订编号推进到第六次，理由段
  记录了"投影沿 `Period`、client 小计、rhythm 窗口三次扩展，都没人问第三个支配参数
  管辖什么"，把失效方式而不仅是修复写进契约。
- 未改动 `ux/menubar.md`：其 `:182-183` 的 client tabs "filter every section at
  once" 面对同一缺口，Round 7 已指出范围相同；供给侧补齐使该声明成立，因此它无需
  改动。若当初选了限定路线，这句话必须同步限定——路线选择的连带范围在此记录。
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试或配置。
- 残余不确定性：体积按条目数上界估算，不是序列化字节数；若 App Group 缓存有硬性
  字节预算，需要在实现任务里按真实 payload 复核。Round 7 提出的完整参数空间走查
  （`Client` × `Period` × kind × size）本轮只覆盖了 W-F9 点名的 `Client` 维度及其
  与 `Period` 的交叉项，未逐格枚举全部三十六个格子。
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 9 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewed state: HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`（Round 8 记录写的
  是 `10ce01e7`；其后新增的唯一提交 `3c6a9c9 docs: approve switch effectiveness
  requirements` 未触及 `docs/topics/desktop-app/` 任何文件，经
  `git diff --stat 10ce01e..HEAD -- docs/topics/desktop-app/` 确认为空，因此被评审
  内容不受影响）。未提交工作区状态：`ux/widget.md` blob
  `77b08af14427c384b91a8fd27f078444558ba9b3`、`architecture.md` blob
  `24e095d1943daae4665691b5b90961fd8a8cb21b`、`ux/menubar.md` blob
  `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`（未改动）、
  `ux/prototype/desktop-surfaces.html` blob `8a8c8e5d…`（未改动）。
- Reviewer: claude-code
- Method: 单 agent 有界复评。W-F9 逐条核验后，执行 Round 7 要求而 Round 8 明确声明
  未做的那件事——**逐格枚举完整参数空间**：kind(4) × size(3) = 12 个配置，每个配置
  再乘 `Client`(3)，`magnitude` 与 `composition` 再乘 `Period`(3)，对每格显示的每
  一个数字回到投影清单确认存在对应形状的字段。随后再扫一遍本文档显示但未出现在
  Data requirements 表里的元素——这是 W-F3 当初的发现方式。
- Scope: W-F9 的处置；36 格参数空间枚举；Data requirements 表的行覆盖完整性；
  本次修复是否引入回归
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，`ux/widget.md` 的 Document gate 不关闭；
  `architecture.md` 再次被改动，其自 `reviews/menubar-experience.md` Round 14 起
  重开的 gate 仍未关闭。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

**W-F10 — `docs/topics/desktop-app/ux/widget.md:209-214,271-286`: `Cost incomplete`
是本文档显示的元素，却既不在 Data requirements 表里，也没有声明它的 client scope。**

- 处置：**新发现，非阻塞。** 与 W-F3 同类（body 显示而需求表未列），但这次多了一层：
  该元素的 scope 在本轮修复之后变得有歧义。
- 行为风险：`:198` 的 Copy 表固定了 `Cost incomplete` / `成本不完整`，`:209-214`
  规定它必须随成本一起显示且不得隐藏该数字。但它读的是投影里的 pricing
  completeness，而该字段在本轮修复后落在两条互相重叠的 bullet 之间：
  `architecture.md:441-443` 的 per-period totals "each with counts, pricing
  completeness, and cost strings" **没有 client scope**，`:461-464` 新增的
  "per-period usage totals and session counts **per client scope**" **没有提 pricing
  completeness**。因此在 `Client = codex` 下，实现者无法判断这面标签讲的是 codex 的
  成本是否完整，还是全局的——若取全局，一个只影响 claude 的未定价模型会让 codex 的
  数字被标成不完整，而这恰恰是本文档 `:125-129` 立论要消除的那类不真实呈现。同一轮
  修复把 `trust` 的 pricing coverage 明确改成了 per scope（`:466-469`），却没有把
  它所对应的那面标签一起处理。
- 证据：`ux/widget.md:271-286` 的 Data requirements 表十四行中没有 pricing
  completeness 行；`:198`、`:209-214` 显示并规范该标签；
  `architecture.md:441-443` 与 `:461-464` 两条 bullet 对 per-period usage totals
  各说一半且 scope 不同；`:473` 的 "aggregate session availability and count" 与
  `:461-464` 的 per-scope session count 之间同样存在重叠。
- 💡 有界改进：在 Data requirements 表加一行——`Cost incomplete` 标签 ←
  per-period pricing completeness，`Varies by` 为 `Client × Period`——并请
  `architecture.md` 把两条重叠 bullet 合并为一条带 client scope 的 per-period
  totals（含 counts、session count、pricing completeness、cost strings），同时说明
  `:473` 的 aggregate session 条目是否仍有独立用途。合并后重叠与歧义一并消失。

#### 🟢 优点

- **W-F9 已关闭，且交叉项确实被处理了。** Round 7 特别要求的是乘积而非单维：
  `architecture.md:453-456` 的 model shares 写明 "for each of the three supported
  periods **and each client scope**"，`:461-464` 的 per-period totals 与 session
  count 同样按 scope 提供。逐格核对 36 格后确认——`magnitude` 三个尺寸 ×3 client
  ×3 period 的 headline、token 行、session 计数、sparkline、20 桶与 90 桶图、
  `avg/day`、`peak`、cache-hit 全部有对应字段；`composition` 三尺寸 ×3×3 的模型行
  与分量拆分同样；`trust` 三尺寸 ×3 client 的三档金额、coverage 条与未定价清单
  同样；`rhythm` 三尺寸 ×3 client 的网格、活跃天数、最忙/最闲日名、90 天热力图
  同样。没有一格落在供给之外。
- **选路是量化之后做的，而不是又一次"两条都行"。** Round 7 明说取舍点是
  publication 体积且未测量；Round 8 测了（906 对 309 条目，约 2.9 倍），并说明
  第二条路线要让四类 widget 中三类忽略一个已声明的配置参数、还要连带限定
  `ux/menubar.md` 的 client tabs——为省 600 个条目而让参数在多数配置下失效不成比例。
  这正是把"本轮不作推定"的悬置项按其应有方式关闭。
- **上界随契约一起更新，并规定了 scope 内独立截断。** `:559-569` 写出三个 scope 下
  的绝对数（270 桶、504 格、108 条），并明确 truncation 在每个 scope 内独立进行，
  "so a busy client cannot consume another's budget"——这堵掉了一个真实的失效：
  全局 top-12 会让繁忙客户端挤掉另一个客户端的全部模型行。
- **本文档新增的 `Varies by` 列把支配关系写成了可核对的东西。** `:271-286` 每行
  标注受哪个或哪两个参数管辖，表头写明 `all` 是一个 scope 而不是没有 scope。
  `composition` client rows 的例外被单列说明（`:288-291`）：它本身就是按 client 的
  枚举，单客户端下只剩一行，是展示选择而非第二个字段。这一列的存在使下一轮的
  同类检查从"通读全文"变成"核对一列"。
- **失效方式被写进契约而不只是修复。** `architecture.md:534-543` 记录了投影沿
  `Period`、client 小计、rhythm 窗口扩展过三次，"without anyone asking what the
  third dominating parameter governs"。
- **W-F1～W-F8 无回归。** 质量档四元组、30 天 rhythm 字段、修订编号（两份文档同为
  第六次）、`aging`/`old` 命名、`No local activity today` 文案均保持既有状态。

#### 📝 总结

W-F9 关闭，且是这一系列 finding 里关闭得最彻底的一次：交叉项、上界、scope 内截断、
支配关系列、失效方式记录都到位，36 格枚举无一落空。本轮 FAIL 仅因 W-F10 —— 一个
显示元素缺行且 scope 未定义，与 W-F3 同类，一处表行加一次 bullet 合并即可关闭。

归属其他目标、不计入本文档未关闭项的一项：`architecture.md:534` 写 "**Nine**
bullets gained a **client scope**"，但清单里本轮获得 scope 的是五条（日序列、
7×24 网格加两个归约、model shares、新增的 per-period totals 与 session count、
pricing coverage），而该句自己随后枚举的是六样东西。数字与清单和它自己的枚举都对
不上，属 W-F7 同类。`architecture.md` 的 gate 自 `reviews/menubar-experience.md`
Round 14 起已重开，该项在其自身复评中关闭；`ux/menubar.md` 未被本轮修复改动，其
client tabs "filter every section at once" 的声明因供给侧补齐而成立。

残余不确定性：体积按条目数上界估算而非序列化字节数，Round 8 已如实记录；若 App
Group 缓存存在硬性字节预算，需在实现任务里按真实 payload 复核。本轮的 36 格枚举
是按文档声明的显示内容做的，不能替代实现期的真实渲染验收。

证据：`git rev-parse HEAD` -> `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`；
`git diff --stat 10ce01e..HEAD -- docs/topics/desktop-app/` 为空；
`git hash-object` -> `ux/widget.md` `77b08af1…`、`architecture.md` `24e095d1…`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F10
```

## Round 10 — 2026-08-17

- Reviewed state: HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`；修复前的
  `ux/widget.md` blob `77b08af14427c384b91a8fd27f078444558ba9b3`、
  `architecture.md` blob `24e095d1943daae4665691b5b90961fd8a8cb21b`，与 Round 9
  评审状态一致；修复后的 blob 分别为
  `dd5e7a2cd6ec43b7244a0dfc298e0f123991437f` 与
  `3dbd51325d92e97673d37bcde8416a0035829ade`。
- Reviewer: codex（修复轮；Document gate 仍保持重开，独立复评前不关闭）
- Scope: W-F10，如修复命令点名。

- **W-F10** `Cost incomplete` 显示元素缺少 Data requirements 行，且 pricing
  completeness 与 client-scoped totals 被拆在两条互相重叠的投影 bullet 中 ->
  **已修复。** `ux/widget.md` 的 Data requirements 表新增 `Cost incomplete`
  行，字段为伴随显示 totals 的 per-period pricing completeness，支配维度明确为
  `Client` × `Period`。表前的修订说明同步推进到第七次。
- `architecture.md` 把两条 per-period totals bullet 合并为一条：三个受支持期间均按
  client scope 供给，且同一 cell 同时携带 counts、session count、pricing
  completeness 与 cost strings。这样单客户端下的 completeness 标签限定的是同一
  client、同一 period 的显示数字，不再可能拿全局状态修饰局部成本。
- Round 9 要求说明 `aggregate session availability and count` 是否仍有独立用途。
  经对 `architecture.md`、`ux/widget.md`、`ux/menubar.md` 与 `requirements.md`
  的消费者检索确认：surface 只读取 per-period session count，没有 projection-wide
  availability 的消费者；来源不可用已由 projection 的 `partial` state 表达。因此
  删除该条目：其中 count 与合并后的字段重复，availability 则是无所有者的缓存数据。
- 未处理 Round 9 明确归属 `reviews/menubar-experience.md` 的 “Nine bullets” 编号
  问题；它不属于本次 W-F10 授权范围。未改动产品代码、测试、配置、prototype、
  `ux/menubar.md`、任务矩阵或 Document gate。
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；
  `make check-whitespace` -> exit 0；`git diff --check` -> exit 0。
- Verdict: REOPEN — W-F10 repair complete, awaiting independent Re-review.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-widget.md / Round 10
```

## Round 11 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：8/10

✅ 结论：FAIL

- Reviewed state: HEAD `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`；未提交工作区：
  `ux/widget.md` blob `dd5e7a2cd6ec43b7244a0dfc298e0f123991437f`、
  `architecture.md` blob `3dbd51325d92e97673d37bcde8416a0035829ade`、
  `ux/menubar.md` blob `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`（未改动）。
  修复前的两个 blob 与 Round 9 判定的一致。
- Reviewer: claude-code（Round 10 的修复由 codex 完成，本轮为不同 agent 的独立复评）
- Method: 单 agent 有界复评。W-F10 逐条核验；对 Round 10 主动做的删除动作
  （移除 `aggregate session availability and count`）独立核实其消费者确实不存在；
  随后重做 Round 9 用来发现 W-F10 的那次扫描——**逐一列出本文档规定要显示或要使用的
  每一个元素，检查它在 Data requirements 表里有没有行、在投影清单里有没有字段**。
  Round 9 那次只覆盖了 Copy 表与四类 widget 的 body，本轮把 Timeline、
  Accessibility、Configuration 三节一并纳入，因为它们同样规定了读取行为。
- Scope: W-F10 的处置；删除条目的回归风险；显示/使用元素的行覆盖完整性
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: 本轮 FAIL，`ux/widget.md` 的 Document gate 不关闭；
  `architecture.md` 再次被改动，其自 `reviews/menubar-experience.md` Round 14 起
  重开的 gate 仍未关闭。

#### 🔴 严重问题 — 必须修复

**W-F11 — `docs/topics/desktop-app/ux/widget.md:231,283`: timeline 的 refresh-after
读取"投影的 next suggested refresh time"，而投影清单不含该字段。**

- 处置：**新发现。** 与 W-F10 同类，落在 Round 9 那次扫描没有覆盖的 Timeline 节。
- 行为风险：`:231` 规定 `timeline` 条目是"one entry for now, plus refresh-after at
  the projection's next suggested refresh time, clamped to 15 minutes minimum and
  60 minutes maximum"。clamp 只约束上下界，基准值来自那个字段；字段不存在时实现者
  没有基准可 clamp，只能自行发明一个（例如固定 15 分钟或固定 60 分钟），而这两个
  端点恰是 `:233-235` 说明要避免的两种失效——低于 15 分钟浪费 WidgetKit 预算，
  高于 60 分钟会漂过 `old` 阈值还不请求刷新。因此这不是措辞缺口，而是一个必需行为
  缺少输入。
- 证据：`architecture.md:439` 的投影 bullet 只有"generation time, partial state,
  and **last successful** refresh time"；同文件 `:68` 把"generated time and **next
  suggested** refresh time"放在 wire snapshot 契约里，两者是不同对象——widget 只读
  投影（`architecture.md:713`、本文件 `:11`、`:258`）。投影清单的引导句是"It may
  contain **only**"，因此未列出的字段不是"没写全"，而是不允许持久化。本文件
  `:283` 的 Data requirements 行也只写了 `generated_at` 与 last successful
  refresh，没有 next-refresh 行——与 W-F10 之前的 `Cost incomplete` 完全同型。
  对照 `ux/menubar.md:771`：菜单栏的 freshness 行明确列了 `next_refresh_at`，
  因为它读的是 wire snapshot 而不是投影。
- 💡 有界修复：在 `architecture.md` 的投影 bullet 2 加入 next suggested refresh
  time（它与 `generated_at` 同源、同为单值时间戳，不涉及 scope 与 bound），并在本
  文件的 Data requirements 表补一行——timeline refresh-after ← 投影的 next
  suggested refresh time，`Varies by` 为 neither（与 freshness 行同理，publication
  是一次事件）；或者改写 `:231`，规定 refresh-after 由固定策略推导并说明该策略如何
  同时避开 `:233-235` 指出的两种失效。前者一处 bullet 加一行表；后者要重写 clamp
  的立论。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **W-F10 已关闭，且是按合并而不是打补丁关闭的。** `architecture.md:441-443` 现在
  是一条 bullet：三个受支持期间 **for each client scope**，同一 cell 同时携带
  counts、session count、pricing completeness 与 cost strings。这样
  `Cost incomplete` 限定的就是它旁边那个数字，不可能再拿全局状态修饰局部成本——
  Round 9 指出的歧义连同两条 bullet 的重叠一起消失了。本文件 `:271` 新增
  `Cost incomplete` 行，`Varies by` 为 `Client` × `Period`，与合并后的字段一致；
  修订编号推进到第七次，两份文档一致（`architecture.md:550-559`、本文件 `:267`）。
- **删除动作先做了消费者检索，不是顺手清理。** 独立核实成立：投影只被 widget 读取
  （`architecture.md:713`、本文件 `:11`、`:258`），菜单栏读的是 wire snapshot；
  `rg 'availability'` 在本 topic 的两份 surface 文档里只命中菜单栏关于本地快照
  可用性的两处叙述，与投影条目无关。被删条目的 count 与合并后的 per-period session
  count 重复，availability 的语义已由投影既有的 `partial` state 表达。删除是对的，
  且理由被写进了 `:556-559` 而不只是留在评审记录里。
- **修复轮如实划定了未授权范围。** Round 9 归属 `reviews/menubar-experience.md` 的
  "Nine bullets" 编号问题未被顺手改动，记录里明确说明了原因。这一轮由 codex 执行、
  与前几轮不同 agent，边界仍然守住了。
- **W-F1～W-F9 无回归。** 逐项复看：per-period 三期间与 client scope 的乘积、
  36 格枚举涉及的全部字段、质量档四元组、30 天 rhythm 字段、`aging`/`old` 命名、
  `No local activity today` 文案、`Varies by` 列、per-scope 上界与 scope 内独立
  截断，均保持 Round 9 判定时的状态。
- **`rhythm` small 的"单个最忙小时"经复核不是缺口。** 它是对已供给的 7×24 网格取
  极值，属于展示层选择而非第二个字段；本轮明确记录这一判断，避免下一轮重新讨论。

#### 📝 总结

W-F10 关闭。本轮 FAIL 仅因 W-F11：timeline 的 refresh-after 基准字段不在投影清单
里，而清单的引导句是"may contain only"，因此这不是遗漏而是禁止。修复是一处 bullet
加一行表。

W-F10 与 W-F11 是同一次扫描的两次收获，差别只在扫描范围：Round 9 扫了 Copy 表与四类
widget 的 body，本轮把 Timeline、Accessibility、Configuration 三节纳入后立刻又出一项。
可以据此收敛：**"文档规定要读取的东西"不止出现在 body 与 Copy 表里**，Timeline 的
输入、Accessibility 的图表摘要来源、Configuration 的 scope 标识都算，因此
Data requirements 表的完整性判据应当是"本文档任何一处提到要读投影的地方都有对应
行"，而不是"每个可见元素都有对应行"。本文档已无更多规定读取行为的小节，
Verification 与 Security 两节只做断言不做读取，因此这一维度到此走完。

归属其他目标、不计入本文档未关闭项：`architecture.md` 第六次修订段仍写"Nine
bullets gained a client scope"，而本轮的合并与删除又让 bullet 数变了一次，该句与
清单的偏差比 Round 9 记录时更大。它在 `reviews/menubar-experience.md` 的复评中关闭。

残余不确定性：W-F11 的第一条路线假定 next suggested refresh time 与 `generated_at`
同源、不随 client scope 变化。若实现期发现刷新建议本身按 scope 不同，投影需要按
scope 携带它——本轮按文档现状判断，不代为决定实现期的形状。

证据：`git rev-parse HEAD` -> `3c6a9c9781ac29b73a5a3bc241b4a5a38b72afaf`；
`git hash-object` -> `ux/widget.md` `dd5e7a2c…`、`architecture.md` `3dbd5132…`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 下一步

```text
修复：desktop-app / reviews/ux-widget.md / W-F11
```

## Round 12 — 2026-08-17

- Reviewed state: current HEAD
  `8a13af7155f5a5303b76bfd70ac21cb918ecedb7`；修复前的
  `ux/widget.md` blob `dd5e7a2cd6ec43b7244a0dfc298e0f123991437f`、
  `architecture.md` blob `3dbd51325d92e97673d37bcde8416a0035829ade`，与 Round 11
  评审状态一致；修复后的 blob 分别为
  `d9f8f41672e46cfde1577e382c154f2067884881` 与
  `165dcc2b26926aeb53da9df362318f4183cd58ac`。
- Reviewer: codex（修复轮；Document gate 仍保持重开，独立复评前不关闭）
- Scope: W-F11，如修复命令点名。

- **W-F11** timeline 的 refresh-after 读取 projection 中不存在的 next suggested
  refresh time -> **已修复，采用 finding 给出的 projection 补齐路线。** 该值已由
  wire snapshot 提供，是与 `generated_at` 同源且不随 client 或 period 变化的单值
  时间戳；把它投影给 widget 比另造固定 cadence 更小，并保留现有双向 clamp 的含义。
- `architecture.md` 的 projection allowlist 第二条现在同时包含 generation time、
  partial state、last successful refresh time 与 next suggested refresh time。
  第八次同日修订段记录字段来源、无 scope 维度，以及 widget 继续负责 15–60 分钟
  clamp，未扩大为新的刷新策略。
- `ux/widget.md` 的 Data requirements 表新增 `Timeline refresh-after` 行：字段为
  next suggested refresh time，`Varies by` 为 `neither`，因为 publication 是一次
  事件；表前修订说明同步推进到第八次。
- 未改动 timeline 的现有 clamp、产品代码、测试、配置、prototype、
  `ux/menubar.md`、任务矩阵或 Document gate，也未处理其他评审目标的发现。
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；
  `make check-whitespace` -> exit 0；`git diff --check` -> exit 0。
- Verdict: REOPEN — W-F11 repair complete, awaiting independent Re-review.

#### 📌 下一步

```text
复评：desktop-app / reviews/ux-widget.md / Round 12
```

## Round 13 — 2026-08-17

### 📋 独立复评 — desktop-app / ux/widget.md

📊 总体评分：9/10

✅ 结论：PASS

- Reviewed state: HEAD `8a13af7155f5a5303b76bfd70ac21cb918ecedb7`（Round 12 记录
  同此。自 Round 11 的 `3c6a9c97` 以来新增的唯一提交 `8a13af7 docs: approve switch
  effectiveness architecture` 未触及 `docs/topics/desktop-app/`，经
  `git diff --stat 3c6a9c9..HEAD -- docs/topics/desktop-app/` 确认为空）。未提交
  工作区：`ux/widget.md` blob `d9f8f41672e46cfde1577e382c154f2067884881`、
  `architecture.md` blob `165dcc2b26926aeb53da9df362318f4183cd58ac`、
  `ux/menubar.md` blob `6c3bfed4e92ea71d8a02f6916d1451a19c5e7f5f`（未改动）、
  `ux/prototype/desktop-surfaces.html` blob `8a8c8e5d…`（未改动）。
- Reviewer: claude-code（Round 12 的修复由 codex 完成）
- Method: 单 agent 有界复评。W-F11 核验后，按 Round 11 定下的完整性判据把全文再走
  一遍——**逐节确认"本文档说要读投影的每一处"是否都有对应行与对应字段**，而不是只看
  可见元素。同时把 W-F1～W-F11 全部逐项回看，确认十一项在当前内容状态下均已关闭
  且无回归。
- Scope: W-F11 的处置；W-F1～W-F10 的回归；全文读取点的覆盖完整性
- Evidence: `bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace`
  -> exit 0；`git diff --check` -> exit 0。未改动任何产品代码、测试、配置或被评审
  的文档。
- Completion evidence: `ux/widget.md` 的 Document gate 随本轮关闭，CEv1 按本轮的
  确切内容状态（HEAD 加 blob 指纹）记录并复查为 `VERIFIED`。**`architecture.md` 与
  `ux/menubar.md` 的 gate 不随本轮关闭**：前者被本系列修复反复改动、后者自
  `reviews/menubar-experience.md` Round 14 起重开，两者需要各自的独立复评。

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无。

#### 🟢 优点

- **W-F11 已关闭，且选的是成本更小的那条路。** `architecture.md:439-440` 的第二条
  bullet 现在是"generation time, partial state, last successful refresh time, and
  next suggested refresh time"；`:562-568` 记录了理由——该标量已由 wire snapshot
  提供，投影它比另造一套固定 cadence 更小，且不引入 client 或 period 维度，widget
  继续负责 15–60 分钟 clamp。本文件 `:297` 新增 `Timeline refresh-after` 行，
  `Varies by` 为 `neither`，与 freshness 行同理。修订编号推进到第八次，两份文档
  一致。修复没有顺手改写 clamp 的立论，边界正确。
- **十一项 finding 全部关闭，且关闭方式各自贴合成因。** W-F1 与 W-F9 用乘积供给
  （`Period` × `Client`），W-F5 按消费者拆分 bullet，W-F10 反向把两条重叠 bullet
  合并，W-F6 改窗口并改掉引起误读的那句话，W-F2 改名而保留显示文案，W-F8 把计数
  扩成 `(cost, tokens, count, share)` 四元组，W-F3/W-F4/W-F7/W-F11 各自补齐字段、
  文案与编号。没有一项是靠调整措辞绕过去的。
- **文档现在自带下一次检查的把手。** `Varies by` 列（`:283-298`）把支配关系写成可
  逐行核对的东西；`architecture.md:534-548` 记录了"沿三个维度扩展过却没人问第三个
  支配参数管辖什么"的失效方式；per-scope 上界与 scope 内独立截断
  （`:570-580`）堵掉了繁忙客户端挤占另一客户端预算的实际失效。这些都不是本轮的
  finding 要求的，是修复轮主动留下的。
- **走查已到边界。** 逐节确认：Configuration、四类 widget 的 body、Surface and
  qualifiers、Copy、Cost completeness、Timeline、Accessibility 七节里每一处读取都
  有对应字段；Verification 与 Security 两节只做断言不做读取；36 格
  （kind × size × `Client` × `Period`）在 Round 9 已逐格枚举且本轮字段未减。
- **两项经复核判定不成立，记录以免下轮重开：**（1）`rhythm` small 的"单个最忙小时"
  是对已供给 7×24 网格取极值，属展示层选择；（2）Surface/qualifier 与 Configuration
  读取的 cache schema version、`partial` flag、client identifiers 三者都在投影
  清单里（`architecture.md:438-441`）却没有 Data requirements 行——与 W-F10、W-F11
  不同类，那两项的实质是**供给缺失或 scope 歧义**，而这三项供给明确、命名明确，
  缺行不产生任何实现歧义。Data requirements 表的既有编辑边界是"展示数据加
  timeline 基准值"，本轮认可该边界。

#### 📝 总结

W-F11 关闭，W-F1～W-F10 无回归，disposition 矩阵内无任何未关闭项——minor 也没有——
因此结论为 PASS。评审对象是上述 HEAD 与三个 blob 的确切状态。

这份文档走了十三轮，值得记下它为什么收敛得这么慢：十一项 finding 里有六项
（W-F1、W-F3、W-F5、W-F8、W-F9、W-F10、W-F11）是同一个问题——**展示要的字段投影没有
供给**——每次沿着上一轮暴露的那一个维度扫描，就只能发现那一个维度上的缺口。真正
让它停下来的是判据的改变：从"检查被点名的行"到"检查同一决定支配的元素集合"
（Round 3），到"逐行做行↔bullet 的形状映射"（Round 5），到"逐格枚举完整参数空间"
（Round 7、9），最后到"本文档任何一处说要读投影的地方都有对应行"（Round 11）。
判据每扩一次，就再抓出一到两项；扩到覆盖全文读取点之后，本轮没有再抓到。

`architecture.md` 在这十三轮里被改了七次（第二至第八次同日修订），全部由本文档的
finding 驱动。它的 Document gate **不随本轮关闭**：这些改动没有任何一次被独立复评
过，且它自 `reviews/menubar-experience.md` Round 14 起本就重开。`ux/menubar.md`
同样保持重开，其 `:754` 的 period switcher 行虽已在 Round 2 修复中改写，但同样未被
独立复评。两者应当一并复评，因为它们绑定的是同一份投影契约。

已知归属 `architecture.md`、留给该文档自身复评的一项：其第六次修订段写"Nine
bullets gained a **client scope**"，与清单实际条数及该句自身的枚举都对不上，且此后
的合并与删除又让偏差扩大。

残余不确定性：投影体积按条目数上界估算而非序列化字节数；36 格枚举依据的是文档声明的
展示内容，不能替代实现期的真实渲染验收；`(cost, tokens, count, share)` 是否已由
`usage stats` 按质量档输出，本系列始终按"文档层面的缺口独立于 CLI 现状"判断，未核验
CLI 侧——实现任务需要先确认这一点。

证据：`git rev-parse HEAD` -> `8a13af7155f5a5303b76bfd70ac21cb918ecedb7`；
`git diff --stat 3c6a9c9..HEAD -- docs/topics/desktop-app/` 为空；
`git hash-object` -> `ux/widget.md` `d9f8f416…`、`architecture.md` `165dcc2b…`；
`bash scripts/check-topic-docs.sh` -> exit 0；`make check-whitespace` -> exit 0；
`git diff --check` -> exit 0。

#### 📌 Task checkpoint

```text
Task checkpoint：desktop-app / ux/widget.md（blob d9f8f416，Review Round 13 PASS，
                CEv1 Document gate VERIFIED）
提交建议：ux/widget.md、architecture.md、reviews/ux-widget.md、
          reviews/menubar-experience.md、tasks.md、docs/README.md —— 按 hunk 排除
          并行工作的 v0-5-0-contract/tasks.md、switch-effectiveness-boundary/ 及
          docs/README.md 中不属本 task 的行
推送建议：未解析 —— 当前分支 main，项目未授权在此提交或推送；两者均需显式授权
```

`architecture.md` 虽随本 task 改动而进入同一提交范围，其 Document gate 仍未关闭。

#### 📌 下一步

```text
复评：desktop-app / reviews/menubar-experience.md
```
