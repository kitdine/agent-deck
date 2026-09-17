---
status: active
topic: schema-version-signal
subject: menubar-schema-presentation
---

# Menu Bar Schema Presentation — Review

## Round 1 — 2026-09-08

## 📋 Task 4 评审报告

📊 总体评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- App 层按稳定 code 判断 schema signal；notice 保留 offline/failing 优先级、
  health.problems > 1 阈值，仅抑制 partial 和两项明确的后果 warning。
  独立 session warning、未知 warning、aged 时间和健康快照重置均有针对性断言。
- HealthCheckRow 保留可选 code/count/supportedCount，不以缺失数字制造零。
  schema cause/recovery 与 Hook count prose 分离，schema_outdated 仍使用命令复制分支。
  DisclosureGroup 可访问性表示绑定展开状态，标签按名称、状态、可见 prose 排序；
  prose 自身隐藏以避免重复朗读，复制命令仍保留独立控件。
- 中英文展开/折叠原生 PNG 已直接查看：展开显示 99/23 与升级说明，比例字体，
  无复制按钮；折叠只保留状态与 disclosure。与 D4/D5 及 prototype 的对应文案一致。
- 八个本地化键进入 allKeys，版本/计数格式化及 no-update-check 测试通过；
  panel/footer/provider popover 使用规定归因文案。徽标语义留在 App 层，
  Shared refresh-state、Widget、tab marks 和几何常量未改变。

### 📝 总结

- Reviewer：Codex；Method：与实现执行分开的评审角色，直接核对 D1–D9、九个
  App/AppTests 文件 diff、测试源码、原生截图及原始日志/CEv1；未委派，不声称
  完全冷上下文。worktree 无 CodeGraph 索引，使用限定源码检查。
- Scope：Task 4 的菜单栏 presentation；未进入 Task 5 全链路或手工验收。
- Findings：无。
- Workspace：agent-deck.schema-version-signal，feature/schema-version-signal。
- Reviewed state：HEAD `64956c603b9b9802b18c1ee9a728344baf161bbd`；
  fingerprint `cbfcf5df79bf3dd3e4ae169e2c3eb4c1a56504a8ea94b2673abcfcbf83f14913`。
  现场按 head=<HEAD> 加排序的九个 ;path=blob 条目重算一致；状态文档排除。
- Evidence：现场确认 Xcode 26.4 / 17E192；原生日志
  `/private/tmp/agentdeck-menubar-schema-native-en.log` SHA-256
  `e22f34d17abccf09fff247057d1831e0259dc070fb480bb6d5f91c3ade24aecd`。
  核对 Shared 40、App 66、Widget 22 项均零失败，包含新增 schema tests、
  双语 key inventory 和 no-update-check；复用原生结果，不因评审阶段重跑。
  expanded PNG 的 en/zh-Hans SHA-256 分别为
  `69ac36265fede785a6141d1cfee2154a51166fd25840c58c0a3c12d618feebce`、
  `c44bfc6be41c3e03e6b03c4e734b347a720bda55dedc5bd5571d8319c8a70303`，
  与 CEv1 相符；同时查看了两种语言的 collapsed PNG。
  prototype/src/Popover.jsx 的 cause/recovery/count 分支与 i18n.js 双语值作为比较来源，
  未改动 prototype。
- Completion gate：VERIFIED（4/4）。现场固定 gate-status.cypher 查询：
  WorkUnit `urn:ce:agent-deck:work-unit:schema-version-signal-menubar-schema-presentation`；
  target `urn:ce:agent-deck:state:implement:menubar-schema-presentation:di7gVes8KvamLBPG`。
  notice-precedence、health-disclosure、copy-and-chrome、native-verification
  均有适用 pass，missing/invalidated/unresolved 均为空。
- 限制：全原生测试结果固定 AGENTDECK_TEST_LOCALE=en 和
  TEST_RUNNER_AGENTDECK_TEST_LOCALE=en；不声称任意系统语言下全套通过。
  初始系统中文运行遇到的既有英文 literal assertion 已在实现交接说明，
  不作为本任务引入的缺陷。行级渲染和模型可访问性字符串检查不等于完整
  VoiceOver 操作或全界面布局验收；这些仍属于 Task 5。
- 文档检查：本轮评审与状态同步后，make check-whitespace、
  bash scripts/check-topic-docs.sh、topic/canonical main 的 git diff --check
  均通过。Beads 已同步 awaiting_commit、round-1。

Task checkpoint：ad-svs-menubar-schema-presentation-dev；content_state=cbfcf5df79bf3dd3e4ae169e2c3eb4c1a56504a8ea94b2673abcfcbf83f14913；gate=VERIFIED。

提交建议：授权后按 Task 4 提交九个 App/AppTests 文件及本记录、tasks.md、docs/status.md 对应变更；canonical main 状态投影单独处理，不混合暂存。

推送建议：提交对象、归属 trailer、SSH 签名和已提交内容证据核验后，另行授权推送 feature/schema-version-signal 至确认的远端同名分支；不直接推送 main 或合并。

### 下一步指令

开发：schema-version-signal / schema-signal-acceptance

WORKFLOW_WORKSPACE: agent-deck.schema-version-signal
