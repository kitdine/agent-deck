---
status: active
created: 2026-10-05
---

# P3 footer 窄边界信号：行政核验与处置

原 issue：`ad-bug-footer-routes-narrow-truncation`（P3，v0.6.5 成员）。
本记录是未确认信号的行政证据载体，不是 Lane A/B 产品修复；没有实现、
失败回归或产品交付声明。版本会员清单与聚合任务保持未完成。

## 原始信号与处置范围

[原评审](../topics/schema-version-signal/reviews/ux-menubar-schema-signal.md)
第 286–292 行记录 en/280pt 的 `Codex aigocode · Claude official` 在 CSS
原型中 short by 2px，明确指出 CSS grid 与 HStack/Spacer、字体度量的差异，
将其列为待 native 核实信号。live Beads 描述另称原型五态均出现，并建议使用
`AGENTDECK_TEST_WIDTH=280` 与真实路由**名字**核对；没有要求生产账户状态。

2026-10-05 操作者接受限定、证据支持的 not-a-defect 行政处置，并明确允许
本地证据/任务文档、必要独立评审、 scoped CEv1 和 Beads 状态更新。
该处置只针对上述原始路由字串的 prototype-to-native 推断；不宣布五态、
任意 provider 名称、全布局或已安装生产状态栏入口验收通过。

## 源与既有策略

证据源为 release merge `8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`，
tree `dcaa17faf974fee689ed47cf1cef2019b2afb12d`。
`source-manifest.json` 的 13 个 Swift 文件已逐一重新计算 Git blob，全部匹配。
核心 blob：MenuBarSurfaceView `87f19bad97134543f6b3c4f4435a93323550a94a`，
MenuBarViewModel `0f2c9056dcefb999709b2b574b164860a1fcc731`。
复制源位于下述 operator-local BASE 的 `isolated/source/app/` 和 `shared/`。
完整 surface 与依赖未修改；QA fixture/host 不属于产品代码。

- [MenuBarSurfaceView](../../apps/macos/AgentDeckApp/MenuBarSurfaceView.swift)
  第 87–105 行：defaultWidth=420、narrowWidth=280；宽度 override 在 `#if DEBUG`。
  第 186–214 行：dataSurface 使用同一 FooterView，位于内容之后的固定区域。
  第 870–905 行：HStack/Spacer；标签 caption、路由 caption/medium，
  `lineLimit(1)`、`truncationMode(.middle)`，没有缩放兜底。
- [MenuBarViewModel](../../apps/macos/AgentDeckApp/MenuBarViewModel.swift)
  第 1314–1322 行：正常路由按 client/provider 名称拼接；schema signal 或空路由
  会改变文案。FooterView 没有原型 normal/empty/aged/partial/pending 五态分支，
  但这项源码事实不能替代各状态的运行证据。
- [历史几何契约](../archive/topics/desktop-app/ux/menubar.md) 第 561–566 行：
  default 420pt、narrow 280pt、footer 固定一行。
- [历史验收路线](../archive/topics/desktop-app/acceptance/menubar-experience.md)
  第 53 行：280pt 使用 `AGENTDECK_TEST_WIDTH=280` acceptance harness。

## 既有直接证据

BASE：`/tmp/agentdeck-p3-fullpopover-20261004`。
RUN：`BASE/hit-target-position-retry-20261005`。
这些是 operator-local 保留原件，临时路径不是交付下载链接。

| 范围 | 观察 | 边界 |
| --- | --- | --- |
| 8 个原字串：en/zh-Hans × light/dark × 280/420 | 全部 footer 字串完整；四组 narrow/wide route-glyph mask 在仅平移归一化后相同 | 完整 MenuBarSurfaceView 的 offscreen NSHostingView，合成 fixture，1px/pt |
| 4 个故意超长字串：en/zh-Hans × light/dark × 280 | 每个均可见 middle ellipsis | 正对照证明检测与现行截断策略可观察；不建立新的长名字接受政策 |
| 2 个观察到的 cpa 名字：en/zh-Hans × light × 280 | `Codex cpa · Claude cpa` 完整 | 名字放入合成 fixture；不是生产 snapshot 或真实账户执行 |
| 1 个真实可见 NSPopover：en/light、原字串、280pt | 完整原字串可见，无 footer 省略号或左右裁切 | 临时 synthetic QA App；没有生产 status-item/controller |

Offscreen 原件：`BASE/qa-receipt.json`、`qa-result.md`、`screenshots/`、
`glyph-comparison.json` 和 comparator `GlyphComparison.swift`。
14 张 PNG 摘要再次匹配 receipt；glyph-mask 比较只取 footer 的高对比路由墨迹，
排除 dim label/chevron，不使用理论宽度或 AX 文本代替实际字形。
环境为 macOS 26.7.1 (25G313)、Swift 6.3、x86_64、默认 large Dynamic Type。

真实 NSPopover 图生成于 2026-10-05 05:03:19 UTC，内容 280×760pt，
outer 306×786pt、backingScale=2、PNG 560×1520px；isShown/windowVisible=true。
requested Aqua，effective VibrantLight，Aqua best match。截图是可见 popover 的
内容 NSView bitmap，不包含窗口阴影/chrome。源字体角色为 caption/medium；
`.AppleSystemUIFont` 代理不能证明 SwiftUI 实际字体文件。

持久 Library 图：`actual-popover-en-light-normal-280.png`，157711 bytes。
Library pointer：`libfile_16ec01ae3ce081918a40b7a516571ab5`；
backing file：`file_00000000ab7481f69549731c60f219b2`。
PNG SHA-256：`d1ee76b04edbe65f0f8feb459d8da99dfd12add3f860b44e395038eea21c30a1`。
对应 RUN JSON SHA-256：`3b3f4332441a6683cb6cb455781dc41df4af07fcb4a3470a9db43903261d43d5`。
Library 当前读取返回 OCR；本轮直接检查 local PNG，没有重新上传或渲染。

## 2026-10-05 来源与验收要求修正

保留 `BASE/qa-result.md`、RUN `result.md`/`receipt.json` 和先前失败记录原文。
其旧结论要求 installed Release 280pt 与真实账户执行，或写触发来源未知。
本次新增更正依据，而非改写历史：

1. 实际用户直接确认“刚手动点击过 QA 按钮”；parent 在 05:05 UTC 亦确认。
   RUN `manual-click-user-confirmation.json`、`supervisor-helper-review.json`
   与 `supervisor-verification.json.report_provenance` 纳入后到确认。
   原 QA writer 未收到该输入，所以其 unknown-origin 文字保留为当时记录。
   自动输入取消，OS posts=0；QA 与旧 CLI 均自然 exit 0。
2. 原信号要求真实路由名字，历史 acceptance 使用 harness，源码宽度 override
   又为 DEBUG-only。因此 installed Release 280pt/真实账户并非本行政处置的
   既定前置条件；它们仍是**未取得的生产入口证据**，不能写成已测通过。
3. 本处置依赖八个原字串 renders、四个可见截断正对照、两名字对照、四组
   字形比较及一个真实可见 popover，超出“不能复现一次”的证据强度。
   它们支持限定信号没有被确认为 native 产品缺陷，不证明未测情况无缺陷。

## 保留限制与执行边界

原型五态没有完整 native 矩阵；arm64、其他 Dynamic Type/字体/缩放、
生产状态栏控制器/生产数据与所有交互均未取得本范围证据。
上方 header/body 在强制窄宽度有裁切，仅记观察，不新增 issue 或实现。
长字符串截断符合现行源码策略；未决定修改字串、缩放或接受政策。

RUN `supervisor-verification.json` 核实历史清理：唯一 QA ID unregister exit 0，
bundle 仍在时公开查询 -10814，再精确删除 exit 0；最终 QA bundle/注册缺席。
生产 27 bundle 文件、22 protected hashes、572 历史 stat fingerprints、原生产
PID/start/path、4 LS URLs 和 main HEAD/clean 保持基线。live SQLite/shared
snapshot 元数据有变化；没有写入/恢复，未建立 QA 因果，不声称数据字节不变。
本轮只复核原件，没有新 App/build/run、OS event、注册或 cleanup。

## 行政结论与独立核验边界

限定处置为 evidence-backed not-a-defect：撤销把 2px CSS 原型信号推断为已证实
native 缺陷的前提，无已实施修复，无代码 PR。源/案例/环境变化或新直接反例
可重新开启原 issue；未测状态不是本轮 PASS 声明。

独立评审完整报告在本文件追加；本记录不能自行签发 review PASS。
`tasks.md` 修订另受既有 `v0-6-5-contract:tasks.md` 的 coverage/authority/readiness
门禁约束，旧 exact-state VERIFIED 不自动适用于新 blob。没有建立 fictitious
`fix:footer-routes-narrow-truncation` 产品 WorkUnit，也不跨 Topic/Release 边界。
Beads 行政关闭须在独立核验及适用文档门禁后读回；Git 交付仍未授权。

## Review — Round 1 — 2026-10-05

## 📋 P3 行政处置及版本任务行补充独立评审

📊 总体评分：9/10

✅ 评审结论：PASS（仅限冻结行政文档候选及 P3 行修订）

**Reviewer：** 本轮独立、冷上下文本地 Codex reviewer，未参与候选写作，未继承作者或 QA 执行上下文，未启动其他 worker。记录及最终落盘由主执行者负责。

**Method：** 冻结身份核对、现行规则与原始契约审查、限定源码追踪、既存证据哈希核对、全部指定 PNG 原始分辨率目视检查及历史评审处置核对。CodeGraph 启动器因缺少可用 node 失败后，采用限定源码读取。

**Scope：** `docs/fixes/footer-routes-narrow-truncation.md` 及 `docs/topics/v0-6-5-contract/tasks.md` 的 P3 一行修订。全程只读；未运行 App、构建、重渲染、比较器、产品测试或手动 Hook，未查询或修改 CEv1、Beads，未执行 Git 交付。

**Reviewed state：**

| 对象 | 冻结身份 |
| --- | --- |
| HEAD | `787cf40c4ba36d59d8deba92c32cf282e66d9c99` |
| P3 行政载体 blob | `6d786ef8b19fb4670add7a8b5bd080124a46bb0f` |
| tasks.md blob | `00e596bb1d6b131d28864683c1bdbbf97086309b` |

工作树文件与 `/tmp/agentdeck-p3-disposition-20261005/` 冻结副本、candidate manifest 的 blob 和 SHA-256 均匹配。

### 🔴 严重问题 — 必须修复

无。

`P3-ADM-R1-Fn` 范围内 findings：无。
`V065-P3-R5-Fn` 补充 tasks 文档 findings：无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **处置与原始 issue 身份一致。** 原评审第 286–292 行明确记录的是 CSS 原型的 2px 信号，并承认布局与字体度量差异；它没有成立 native 产品缺陷。候选保留了这一未确认性质，没有虚构根因、失败回归、产品修复或代码 PR。
- **证据强度超过一次未复现。** 八个原字串离屏案例、四个超长正对照、两个 cpa 名字对照及一个真实可见 NSPopover 的 footer，均与候选的限定观察相符。
- **旧报告得到追加式修正。** 用户手动点击确认被明确标为后到证据；旧 `UNESTABLISHED` 文字和失败记录保留，自动输入取消与 OS posts=0 没有被改写为自动化成功。
- **版本和交付边界保留。** tasks.md 仅修改 P3 行，保留十九项成员、所有聚合矩阵状态及既有排除范围；没有推进 assembly、whole-version completion 或 Git 交付。

### 📝 总结

**现有证据足以支持本次限定行政处置。** 所支持的结论是：原字串的 CSS 窄边界信号，在指定 release 源和已测 native 环境中没有被确认为产品缺陷，可以按候选明示的边界记录 evidence-backed not-a-defect。该判断不是五态、任意路由名字或整体 native 布局的无缺陷证明。

**源身份及隔离边界：** 独立按 Git blob 配方核对了全部 13 个复制 Swift 输入，无不匹配；核心 surface/model blob 同时与工作树链接目标一致。源 merge/tree 的归属依据既存 manifest 与 receipt，本轮未取得远端对象或更新 refs。源码确认 default 420pt、DEBUG-only 280pt override，以及固定 FooterView 的 HStack/Spacer、caption/medium、单行中间截断策略。QA host 使用合成 fixture、禁止 switching 的 adapter、内存 preferences 和 nil snapshotStore；没有生产 status-item/controller 路径。

**视觉及字形证据：** 14 张离屏 PNG 的哈希均与 receipt、input read-back 匹配，逐案 JSON 元数据一致。八个原字串案例完整可见，四个长名字案例均可见中间省略号，两个 cpa 对照完整可见。`GlyphComparison.swift` 在 footer 区域筛选高对比墨迹，仅归一化平移；四组 narrow/wide 非空 mask 的相等结果与目视观察一致。它支持这些已测图像的字形保持，不独立证明所有字体、状态或环境均正确；比较器未重新执行。

真实 popover PNG 完整显示 `Codex aigocode · Claude official`。PNG/JSON 摘要分别匹配：

- PNG：`d1ee76b04edbe65f0f8feb459d8da99dfd12add3f860b44e395038eea21c30a1`
- JSON：`3b3f4332441a6683cb6cb455781dc41df4af07fcb4a3470a9db43903261d43d5`

源码、元数据及后到的直接用户确认共同支持临时可见 NSPopover 的来源说明；没有独立 CG popover witness，不能据此增加生产入口或自动输入成功声明。

**验收假设修正成立。** 原 issue 要求真实路由名字；历史 narrow acceptance 明确采用 harness，源码 override 又仅在 DEBUG。没有依据把 installed Release 280pt 或生产账户执行升为本行政处置的既定前置条件。候选仍将这些生产入口证据列为未取得，修正没有把缺失证据改成 PASS。

**覆盖限制保留充分。** 五态 native 矩阵、arm64、其他 Dynamic Type/字体/缩放、生产状态栏控制器及完整交互未覆盖。FooterView 没有对应原型五态的布局分支，只是源码事实，候选正确地没有用它代替运行证据。上方 header/body 裁切也未被包装成整体布局通过。

**清理及保留声明与既存记录一致。** 注销 exit 0、bundle 尚存在时公开查询 `-10814`、随后精确删除 exit 0 的顺序可核对。保存的前后记录显示 27 个 bundle 文件、22 个 protected hashes、572 个历史 stat fingerprints、生产 PID/start/path、四个 LS URLs 及 main HEAD/status 保持基线；live SQLite/shared snapshot 元数据发生变化。候选没有声称生产数据字节不变，也没有归因或恢复数据。本轮仅检查这些历史记录。

**历史及 L0：** 最新适用 tasks review 为 Round 4 PASS；Round 3 FAIL 及 `V065-LS-R3-F1/F2` 的关闭处置仍保留。本次 P3 行不改变其 doctor 契约或处置链。行政载体五个本地链接目标存在；own-class 检查采用主执行者提供的 `bash scripts/check-topic-docs.sh` exit 0 结果，已检查脚本无 scope option，未重放。最终 `make check-whitespace` 与 `git diff --check` 待主执行者记录报告后完成。

**独立 reviewer 的 CEv1 交接说明：待主执行者对新 exact state 评估。** 本 reviewer 未查询或签发 CEv1 状态。既有 `v0-6-5-contract:tasks.md` 的 coverage/authority/readiness 文档门禁继续适用；旧 exact-state VERIFIED 不覆盖新 blob。行政载体不自动成为 Lane A 产品 WorkUnit，也不能借此豁免文档门禁。

本报告认可冻结候选的限定行政处置及 P3 行补充。Beads 关闭、文档门禁收口、Git 交付和整个 v0.6.5 完成都仍待主执行者各自核实。


### 主 executor 收口

Reviewer session：`01a10a97-e1da-7bf2-a125-0f7fcb14dc7c`；新建 local Codex exec，
custom/cpa、gpt-6.1-sol/xhigh、read-only、local request service_tier=default，
CLI 0.160.0，自然 exit 0。未 resume/fork 作者或 QA 上下文。无 upstream Fast 声明。
主 executor：`01a10a8f-0f6c-7aa0-b230-7fb286b520bc`，同模型/provider；拥有最终记录。
主 executor 复核冻结两文件 blob、13 源 blob、14 PNG 摘要、native PNG/JSON、
核心源码与图像和适用门禁后接受上述限定 PASS；不是全布局或五态 PASS。
原始 reviewer 报告 SHA-256：`3aaf859204c0dea6ae532243d5be9c68ed39e2ca2e06c8d94028e66c7a8b5343`。
原文与完整 transcript 保留于 `/tmp/agentdeck-p3-disposition-20261005/`；
本权威报告仅将 reviewer 未执行的 CEv1 交接文字与主 executor 门禁字段区分。

完成门禁：NOT_REQUIRED。行政误报处置不恢复/决定产品行为，不属于 Lane A task；
版本 tasks.md 第 174–181 行与 Beads 的 administrative disposition 规则允许该
行政决定，没有适用于此独立行政 carrier 的产品完成门禁。该判断不来自空图记录；
缺失 WorkUnit 不会被当作通过。实际被修改的文档另受已登记的
`v0-6-5-contract:tasks.md` 三项 required gate 约束，其结果由 matching Round 5
和 CEv1 记录承担；未取得 VERIFIED 前不关闭行政协调。

Findings：无；历史 failure/unknown-origin 报告未删除或改写。
Git 交付未授权，未 commit/push/PR；本 carrier 保持 active，不伪造归档或产品交付。

Linked document gate：`v0-6-5-contract:tasks.md` 已在新目标
`f041ba70cad2cfff4111a15bb3c4f598592a31416abca4eb588a187a1b9b61ea`
取得 VERIFIED（3/3，missing/unresolved 为空，历史 invalidation 保留）。
主 executor 最终 L0：own-class check exit 0、16 本地链接目标有效、
make check-whitespace 与 git diff --check exit 0；复制报告的一行尾随空格
已修正，首次 failure log 保留。没有新产品验证。
Beads read-back：`ad-bug-footer-routes-narrow-truncation` 为 closed、无指派；
close_reason 与批准的 bounded not-a-defect 行政决定逐字匹配，原描述/notes/
priority/labels 保留。匹配文档 task `ad-v0-6-5-contract-doc-tasks-design`
为 awaiting_commit、无指派，旧交付历史与本次未交付修订分开。
最小后续边界：三份本地文档的显式 Git 交付授权；本轮自然结束，不开下一任务。
