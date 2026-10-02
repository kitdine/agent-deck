---
status: active
created: 2026-10-02
---

# 缺陷：Large quota Widget 表头误标单客户端

## 现象

Beads `ad-bug-widget-large-quota-header-client-label`。用户授权 Lane A，两个 Widget presentation issue 一批一 PR，目标 `release/v0.6.x`。
Base `2eed6aff500c0691e8f02ad80976bc8e5dff2b58`；branch `fix/widget-presentation-batch`。

## 根因

quotaScopeClient 在 large 返回 entry.client，但 presentedQuotaClients 会显示两端；真实 QuotaWidgetIntent 只可选 Codex/Claude。

## 修复边界

large 返回 .all；small/medium 保持既有选择。真实 intent 两端×三尺寸检查 scope、英文表头、正文客户端集合；另留12张 native尺寸 light/dark 渲染附件。
本成员独立 WorkUnit `fix:widget-large-quota-header-client-label`；publisher lifecycle 不属于本批。
不安装生产 Widget、不修改真实数据、不替代独立版本治理工作区的 assembly。

## 验证

RED: 同一原生回归4条断言失败（Codex/Claude各scope与All clients文案）。GREEN: scripts/test-macos-app.sh 全部成功，47 Widget tests通过；完整应用/共享套件通过，有既存1项skip。12张隔离 NSHostingView 附件导出，主代理检查两端large、Codex medium、Claude small，表头与正文一致。
原始日志在 `output/widget-batch-evidence/`，渲染原件在最终 XCTest xcresult 与 `/tmp/widget-batch-native-attachments/`。
L1 renderer + scoped browser regression；没有 Go 核心代码变化。`make check-whitespace`、`git diff --check` 与 prototype build 通过。
浏览器检查只覆盖成本说明；原生 NSHostingView fixture 不是桌面 WidgetKit 安装验收。原生 Increase Contrast 未执行，单独分类且不冒称 browser WCAG 已覆盖。

## Review — Round 1

## 📋 Widget 展示批次冷独立评审与复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议

无在本成员范围内的未关闭发现。

### 🟢 优点

成对正常/边界测试复现原缺陷，修改限定现有展示契约；测试与交付证据逐成员保留。

### 📝 总结

Reviewer: 冷上下文 `widget_cold_review`；主代理负责裁决与证据核验。
Method/Scope: read-only 独立源码/消费者/测试/记录检查，加两轮增量复评。主代理直接核验测试原始日志和浏览器结果。
Reviewed state: HEAD `2eed6aff500c0691e8f02ad80976bc8e5dff2b58` + scoped SHA256 `bc7bce2ff24480a361080829f03188ec72e7872f7c601879fa12bdc3af77a7b5`。
Evidence: native RED/GREEN/final logs、14组浏览器矩阵、真实尺寸隔离渲染、L0/build；没有用测试成功替代未执行的桌面安装。
Completion gate: VERIFIED，Neo4j MCP exact candidate `urn:agentdeck:widget-batch:candidate:bc7bce2ff24480a361080829f03188ec72e7872f7c601879fa12bdc3af77a7b5`，3/3 required criteria，missing/invalidated/unresolved 全空。

### Task checkpoint

本成员修复、独立评审与 exact-state gate 已完成；建议按用户既有授权签名提交并普通推送，远端 current-head review/CI 与实际 merge gates 仍须完成。
本批无 topic gate；集成门禁独立于两个 Task。
