---
status: active
topic: v0-6-0-contract
subject: v0-6-0-contract
---

# Version contract task reviews

## Round 1 — 2026-09-27

## 📋 v0.6.0 版本契约任务评审

📊 总体评分：9/10

✅ 评审结论：PASS

Reviewer: Codex。Method: 单评审者的文档契约与来源核对；复用已交付的五批集成记录，未重跑未变更的产品测试。Scope: Task 2 的版本级契约闭合、`tasks.md` 本轮交接差异及其对现行规格、手册、索引和版本来源的陈述；不评审发布候选或实际安装行为。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须关闭

无。

### 🟢 优点

- 保留五个已集成方向及 cost transparency 的排除决定，且把技术预检、RC1 标签和发布留在独立边界。
- `cli-design.md` version 30 的单条 v0.6.0 历史记录、`cli-manual.md` 的 schema 30 和 wire-v1 说明、`docs/README.md` 的版本指针一致；`runtime_error` 到 `schema_ahead` 的收窄仍保持普通错误退出码 1。
- 明确保留桌面原生、性能和 health-recovery 真实客户端观察的验收限制，没有把 waiver 或模拟观察改写成技术 PASS。

### 📝 总结

Reviewed state: HEAD `25778bb504de1ae09f4ba9bc0f7ba87e9b39f5f0`；`docs/topics/v0-6-0-contract/tasks.md` blob `b45c5fe6d33c6624aba95a750473ec65f0d170e0`；scoped SHA-256 `56a4cba55dd89f686f6c1c095d05c6ce24da774c638b68cbc57cc47d6061e783`，按 `head=<HEAD>;document=<blob>` 计算。Task WorkUnit 按项目绑定为 `v0-6-0-contract:v0-6-0-contract`；此内容指纹不是未经查询便可声称存在的 CEv1 ContentState 节点。

Evidence: `bash scripts/check-topic-docs.sh` exit 0；`make check-whitespace` exit 0；`git diff --check` exit 0。现行 `cli-design.md` frontmatter 为 30，v0.6.0 history row 覆盖五个方向与兼容性变化；手册描述 core schema 30 和 desktop wire 1，索引指向 version 30。`internal/store/store.go` 的 `CurrentSchemaVersion` 为 30；`internal/desktop/desktop.go` 与 `apps/macos/AgentDeckShared/DesktopWire.swift` 的 wire version 均为 1；`apps/macos/Config/AgentDeck.xcconfig` 声明 marketing version 0.6.0；`Makefile` 从 Git tag 注入 CLI 版本，`scripts/render-homebrew-cask.sh` 从 stable/RC tag 生成 cask 版本。既有 aggregate `assemble` Round 8 与 main merge-result VERIFIED 4/4 仅用于前置集成边界，不能替代本任务门禁。

Completion gate: VERIFIED (5/5)。后续可用的 Neo4j Cypher/CEv1 MCP 会话在仓库 namespace `github.com/kitdine/agent-deck` 中建立 Task WorkUnit `v0-6-0-contract:v0-6-0-contract`、上述精确目标 ContentState 和五项 required criteria：`assembly-prerequisite`、`contract-reconciliation`、`version-identity`、`acceptance-limits`、`review-l0`。节点写入 12/12、关系预检 15/15 `ok`、关系写入 15/15。固定 `gate-status.cypher` 对目标 `v0-6-0-contract:v0-6-0-contract:review-r1:56a4cba55dd89f686f6c1c095d05c6ce24da774c638b68cbc57cc47d6061e783` 返回 VERIFIED；五项证据均 `target_matches=true`、`applicable=true`，无 missing、invalidated 或 unresolved item。前置 `assemble` 在 PR #12 main merge-result `3aa48a4` 的独立 gate 仍为 VERIFIED 4/4。本 Task 尚未提交，包含的 topic 完成边界尚未跨越。

本轮无未关闭 finding。评审 PASS 只表示上述契约内容可接受；原生和性能验收限制仍如实保留，提交与发布各有独立边界。
