---
status: active
created: 2026-10-02
---

# 缺陷：Widget 成本不完整说明对比度不足

## 现象

Beads `ad-bug-widget-cost-incomplete-contrast`。用户授权 Lane A，两个 Widget presentation issue 一批一 PR，目标 `release/v0.6.x`。
Base `2eed6aff500c0691e8f02ad80976bc8e5dff2b58`；branch `fix/widget-presentation-batch`。

## 根因

可读数据态 9px 说明使用 --dim，在 light 背景 #ede8e0 上颜色 #676f7b 的 axe 对比度仅 4.16:1。

## 修复边界

仅将 .w-note small 改用现有 --muted，保留字号、语义和层级。新增按需加载的 scoped axe 探针；用户明确批准固定 axe-core@4.12.1 开发依赖。
本成员独立 WorkUnit `fix:widget-cost-incomplete-contrast`；publisher lifecycle 不属于本批。
不安装生产 Widget、不修改真实数据、不替代独立版本治理工作区的 assembly。

## 验证

浏览器 RED: axe 4.12.1 serious color-contrast，4.16 < 4.5。GREEN: Chrome light/dark × fresh/aging/old/hostAbsent/unchanged/changed/recovered 共14组，零 violations、零 incomplete，实际 widgetRefresh 身份匹配。
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

WB-R1-F1（P3）：npm10 重写了14处无关 libc 元数据；主代理与冷评均确认，已恢复 base 字节对应元数据，仅保留 axe 的12行新增。CLOSED。
WB-R2-F1（P3）：探针输出错误 state 身份，不能区分 widgetRefresh；改为实际 widgetRefresh 与 lang，主代理重跑14组，独立复评确认 CLOSED。

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
