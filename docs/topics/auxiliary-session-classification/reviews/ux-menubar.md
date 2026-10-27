---
status: active
topic: auxiliary-session-classification
subject: ux/menubar.md
---

# 菜单栏框架评审

## 2026-10-27 证据范围与协调勘误

用户授权修正 Beads 状态后，确认先前将整个文档集缺稿归入本前置文档
document-integrity 的阻塞理由过宽。依据 docs/documentation-workflow.md 的文档集规则，
设计期间允许声明未编写文档；完整集合须在 tasks.md 文档集评审前补齐。
本勘误不改变语义 PASS、不撤销原发现、不豁免文档集/CI 检查，也不新建语义评审轮次。
下面各轮原 BLOCKED 及提交建议作为当时记录保留；当前协调以本勘误的交付对象及 CEv1 新观察为准。

- 交付对象：`298f71ea8817139941e50a5af59440a65457c77f`；正文 blob `6589bae7e4a2bd11e3cc3731ec02a79815f0d912`，与已评审正文一致。
- 新 ContentState：`auxiliary-session-classification:delivered-ux-menubar.md-b98aca24b20e28ee`；指纹 `b98aca24b20e28ee0bc4692b7925182559bda50583c86e41e88d17b8f6e5bd7a`。
- 交付目标查询结果：VERIFIED（2026-10-27）；missing criteria 与 unresolved impacts 均为空。
  此结果仅绑定上述交付对象的 framework，不宣称当前全主题或最终 UX 已完成。
- 复用来源：`auxiliary-session-classification:ux-menubar-r2-c6c7ee3e6b178140:independent-review`。新观察绑定交付对象并通过 rolls_up 显式引用；
  提交只固化已评审内容与记录，不引入新的正文决策。标本按各自交付 commit 核验，未用当前其他表面改动冒充旧 manifest。
- 本文检查范围：格式、真实本地链接、评审记录、空白和已绑定标本完整性；不包含尚未设计的另一文档。
  完整文档集缺稿仍由 tasks.md 承接。检查结果及门禁以新目标 CEv1 记录为准。
- 协调边界：本次仅确认已交付 framework；整份 UX 文档仍待 architecture 后 final-surface reconciliation，不关闭任务。后续共享原型变化不能因本次绑定历史交付对象而自动获得最终表面认可。

历史段落中的“尚未提交/不得提交”和旧 BLOCKED 不再用作当前待评审队列依据；
所有已有历史事实和旧目标证据保持不变。

## Round 1 — 2026-10-27

## 📋 菜单栏交互框架评审

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-MENU-R1-F1 — P2 — 重试成功后的 tab 往返恢复过期失败态，提示与集合矛盾（OPEN）**

位置：prototype/src/AuxiliarySessions.jsx:23–26、48；prototype/src/Popover.jsx:1407–1408、1811–1812。
关联 ux/menubar.md 的“临时选择、默认与生命周期”和“状态、入口与恢复”。

- 行为风险：用户已经成功查看辅助集合，切 tab 返回却看到失败提示与禁用控件；界面同时声称保留 Work，却展示 Auxiliary，破坏范围、列表、统计和状态的一致性。
- 复现：以英文 280 pt 打开共享入口 `?tab=sessions&auxMenu=failed&lang=en&width=280`；点击 Retry Auxiliary，确认 state=normal、scope=auxiliary、count=1；切 Usage，再切 Sessions。无需重新选择失败场景或发起新请求。
- 直接观察：返回后 state=failed、scope=auxiliary、count=1，唯一记录 maint-1；notice 为 `Auxiliary could not load; previous Work data retained.`，三个范围按钮 disabled=true。页面确实在 Usage 时卸载了 asc-panel。首次重试使用浏览器控件点击；tab 坐标点击遇到遮挡后改用页面 DOM click 和双 requestAnimationFrame 等待。因此这是浏览器合成标本状态转换实证，不是原生鼠标/Swift 证据。
- 根因证据：auxScope 位于 Popover 并跨 tab 保留；recovered 位于 AuxiliarySessions，tab 切换导致组件卸载后恢复 false。父级 auxScenario 仍为 failed，未变化，因此父级初始化 work 的 effect 不重跑；子级重新从 failed 场景渲染，而集合仍按 auxiliary 计算。client/period effect 也清除 recovered，须在同一修复边界内核对。
- 💡 修复建议：使当前 selection 与对应请求/恢复状态共用能够跨 tab 保留的生命周期，或以一致的状态对象更新；不能只改提示词遮掩矛盾。补充 failed → retry success → Usage → Sessions 的组合检查，并核对成功后的 client/period 变化与关闭重开默认。返回后应保留成功的辅助 selection/normal 状态，或明确建模新请求，任何情况下不能旧失败标签配新集合。更新受影响标本、manifest 和检查脚本，保留框架与原生验收边界。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- scope 与顶部消耗分离，unknown、项目集合并集和已知时长/缺失样本有明确口径。
- 来源详情只读，复制命令有失败恢复；文档区分浏览器标本与原生 AX、Dynamic Type、持久化验收。
- 14 项 manifest 文件哈希全部匹配，保留截图的来源与历史修正明确。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与菜单栏设计。
- Method: 单评审者，复用 development-workflow 规则；只读文档、共享原型及检查脚本，使用 agent-browser 隔离会话执行上述最小场景；查看 failed-en-light-280.png。发现决定性阻塞后停止广泛验证，未运行产品测试、未重新执行 920 项设计自检或 axe。
- Scope: ux/menubar.md framework 及其共享标本。不是 final-surface 或实现交付评审；不重判已有 requirements/CLI PASS。
- Reviewed state: HEAD `0e74d0c98a5403338d637e1eb0095c1df7c41de4`；document blob `21cb923fa83cc08ac42cf44d5867347f37e28220`。
- Specimen identity: manifest SHA-256 `3506728ca52f3d773e5b083166846dd61961c258f37d39d20ca99686a02e2f97`；14 项源文件/检查脚本/截图全部 MATCH。
- Fingerprint: `fe94273ad7afd2ea7293c3f2e8b55e442bbf32949b7943e9936edfb70a113133`，沿用 SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 重试前为 failed/work/3，重试后 normal/auxiliary/1，tab 往返后 failed/auxiliary/1 与旧 Work 提示并存。现有 check-auxiliary-menubar.js 分别测试正常态跨 tab 和失败态重试，但没有连接这两个转换，故设计自检通过不能排除此反例。
- Environment limitation: 原有本地服务不可达；本轮临时 Vite 服务直接读取本工作区源码，复用主工作区现有 React/Vite 依赖，不改项目配置、不安装依赖。无关 widget-contrast-probe 的 axe-core 解析报错未阻止本菜单栏页面渲染；未宣称 axe 通过。临时服务、浏览器及缓存不作为持久引用。
- Evidence: `python3 scripts/check-docs.py` 报告 Settings、Widget、architecture 三份声明未编写文档，完整性门禁保持开放，不填占位或删除矩阵；评审记录和矩阵同步后的结构、空白及 diff 检查单独记录于 CEv1。
- WorkUnit: `auxiliary-session-classification:ux/menubar.md`；ContentState: `auxiliary-session-classification:ux-menubar-r1-fe94273ad7afd2ea`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-MENU-R1-F1 为 fail；document-integrity 因三份下游文档未完成为 blocked。Beads 返回修复，Review 保持未勾选。
- Residual uncertainty: 首载、真实请求竞态、原生生命周期/无障碍及最终字段供应仍需后续阶段证据；本轮没有作全面通过声明。

修复：auxiliary-session-classification / reviews/ux-menubar.md / ASC-MENU-R1-F1

## Repair candidate — 2026-10-27（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-MENU-R1-F1，未委派、未修改 Go/Swift 产品代码。
- ASC-MENU-R1-F1 → repaired in candidate，待独立复评关闭。范围与呈现状态合并为 Popover
  持有的 auxSelection；重试原子设置 normal/auxiliary。AuxiliarySessions 不再保存 recovered，
  client/period effect 只清除详情与复制反馈，不撤销请求成功结果。
- 明确生命周期：当前窗口内成功结果跨 tab/client/period 保持；舞台明确切换状态才模拟新状态。
  关闭重开重新初始化窗口；舞台仍为 failed 时一致呈现 failed/work，不混用旧 auxiliary 集合。
  normal 舞台下关闭重开恢复已保存默认，未新增采集、存储或产品状态策略。
- Candidate identity：HEAD `0e74d0c98a5403338d637e1eb0095c1df7c41de4`；文档 blob
  `6589bae7e4a2bd11e3cc3731ec02a79815f0d912`；manifest SHA-256
  `cd85cdbc7730aa2cd338177424d484db515d9745f2c717552b708fb40a9ff49e`。
- 验证：`npm --prefix prototype run build` 成功；共享检查脚本加入恢复后组合序列，
  中英 × 280/420、深色四组各 149 项，共 596 项无失败。新增断言包括真实卸载 asc-panel、
  重试后 tab 往返的 normal/auxiliary/1、maint-1 身份、范围按钮可用、旧失败提示不存在、
  九种 client/period 组合仍为恢复成功状态，以及关闭重开的一致起点和默认范围。
- 独立执行原最小反例并读回：state=normal、scope=auxiliary、count=1，notice 为
  `Includes 0 sessions with unknown origin.`；保留 `r1fix-retry-tab-en-280.png` 并目视核对。
  新源文件/脚本/截图哈希已写入原 manifest，初稿检查保留为 previous_design_checks。
- 复用未改动颜色、布局与语义标记的历史设计检查；未重新宣称 axe/native 通过。
  浏览器合成状态转换不等于真实 IO 竞态或原生生命周期验收。
- `python3 scripts/check-docs.py` 退出 1，仅余 Settings、Widget、architecture 三份声明未编写文档。
  该缺口保持开放，不填占位、不削减矩阵。原评审 FAIL/OPEN 与原内容状态 FAILED 保留；
  新候选等待独立复评和证据门，不宣称 CEv1 VERIFIED，Review 仍未勾选。

## Round 2 — 2026-10-27

## 📋 菜单栏框架修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-MENU-R1-F1 — CLOSED**：Popover 的 auxSelection 同时持有 scope/scenario，重试以
normal/auxiliary 一次更新；子组件不再持有 recovered，client/period 变化只清除详情/复制反馈。
文档明确成功状态跨当前窗口的 tab/client/period 保持，窗口重开与舞台状态初始化分开。

独立浏览器重放：failed/work/3 → retry → normal/auxiliary/1 → Usage（asc-panel 确实卸载）
→ Sessions，仍为 normal/auxiliary/1、唯一 maint-1、disabled=false，提示
`Includes 0 sessions with unknown origin.`；原来的旧失败状态与新集合混合不再出现。

### 📝 总结

- Reviewer: Codex，原独立评审会话，未参与修复。
- Method: 单评审者源码/文档核对，agent-browser 隔离浏览器 DOM click 与双 requestAnimationFrame 重放；不委派、不修复产品或标本。采用与 Round 1 一致的临时源码服务和依赖方式。
- Scope: ASC-MENU-R1-F1 的请求恢复、tab/client/period 与窗口生命周期；仅 framework，不是最终架构供应或 native 验收。
- Reviewed state: HEAD `0e74d0c98a5403338d637e1eb0095c1df7c41de4`；document blob `6589bae7e4a2bd11e3cc3731ec02a79815f0d912`。
- Specimen identity: manifest SHA-256 `cd85cdbc7730aa2cd338177424d484db515d9745f2c717552b708fb40a9ff49e`；15 项源文件、脚本与截图均 MATCH。
- Fingerprint: `c6c7ee3e6b1781403458f17f36beaf1834bcb6ea15bb32b71bd4362de9bac6c2`；SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 除原反例外，独立遍历 3 client × 3 period，辅助计数 oracle 分别为 all=[1,1,2]、codex=[0,0,1]、claude=[1,1,1]；九组全部满足 normal/auxiliary、正确计数、按钮可用、无旧失败提示。failed 舞台关闭重开一致回到 failed/work/3；normal 舞台临时选择辅助后关闭重开恢复 normal/work/3。目视查看哈希匹配的 r1fix-retry-tab-en-280.png，恢复后的标签、列表和提示一致。
- Evidence reuse: 原有颜色/布局/语义标记未改变，未重复广泛浏览器或 axe 检查。修复者 596 项检查和 build 保留在 Repair 段，本轮没有冒充独立复跑。已核对脚本新增恢复后组合序列与本 finding 对应。
- Evidence boundary: 合成浏览器状态转换不证明真实 IO 竞态、Swift/AX、Dynamic Type 或持久化。环境仍有无关 axe-core 解析警告，菜单栏可渲染；不作新 axe 通过声明。
- WorkUnit: `auxiliary-session-classification:ux/menubar.md`；ContentState: `auxiliary-session-classification:ux-menubar-r2-c6c7ee3e6b178140`。
- Completion gate: BLOCKED
- Gate scope: independent-review 为 pass；document-integrity 仍缺 ux/settings.md、ux/widget.md、architecture.md。原 Round 1 失败证据仍属于原目标，不覆盖或抹除历史。
- Hierarchy: 主题 Documents 尚未完成，且 architecture 后仍需 final-surface reconciliation；本轮不查询或宣称 Topic 完成。
- Evidence: 记录及矩阵同步后，`python3 scripts/check-docs.py` 退出 1，仅报告三份下游文档未编写；评审记录解析、`make check-whitespace`、`git diff --check` 均退出 0。
- Disposition: ASC-MENU-R1-F1 CLOSED；Review 勾选只代表本轮 framework PASS，Beads 保持 in_review 等待证据缺口解决。

Task checkpoint：ad-auxiliary-session-classification-doc-ux-menubar；content_state `c6c7ee3e6b1781403458f17f36beaf1834bcb6ea15bb32b71bd4362de9bac6c2`；gate BLOCKED。
提交建议：门禁 VERIFIED 后，经授权提交菜单栏文档、共享原型相关改动、检查脚本、manifest/截图、评审记录及矩阵；前置为完成三份下游文档并通过文档完整性检查。
推送建议：门禁 VERIFIED 后，经授权推送 feature/auxiliary-session-classification；前置同上，remote/upstream 尚未解析，提交对象和 SSH 签名留待交付核验。

完成已声明的 ux/settings.md、ux/widget.md 和 architecture.md，通过文档完整性检查并重算门禁；保留本轮 framework PASS。
