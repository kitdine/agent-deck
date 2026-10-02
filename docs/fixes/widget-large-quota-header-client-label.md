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


## Review — Round 2

## 📋 GitHub current-head 评审

📊 综合评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

GH32-F1（P2）：单客户端 producer 配置使 large 正文只有一端。原先无条件 .all 错误；已以实际 presentedQuotaClients 的唯一客户端命名，二端仍为 .all。新增 selected×intent 4组单客户端回归，原实现4失败，修复后48Widgettests及全套隔离 XCTest通过。
GitHub review `5392380679`，原 head `3ce81caa0a1a864190df092da25eca9468ca25d0`。
主代理直接核实 producer/构建消费者与 RED；旧 source 的失败事实追加至 CEv1，不覆盖历史观察。

### 🟡 改进建议

无。

### 🟢 优点

已有双端和浏览器正常态回归仍保留。

### 📝 总结

Reviewer: GitHub Codex，主代理验证。Method: remote diff review + local failure-first reproduction。
Reviewed state: `3ce81caa0a1a864190df092da25eca9468ca25d0`。
Completion gate: FAILED，该source有当前适用的回归失败证据。
GH32-F1 -> repaired in following candidate; independent rereview required.

## Review — Round 3

## 📋 单客户端与测试依赖隔离冷独立复评

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。GH32-F1 CLOSED：上述修复由独立冷上下文 `widget_r2_cold` 核验。

### 🟡 改进建议

无。既有 WB-R1-F1、WB-R2-F1 保持关闭，未发现回归。

### 🟢 优点

验证真实producer选端、effective family和失败表面，并以构建模块图守卫开发依赖隔离。

### 📝 总结

Reviewer: `widget_r2_cold`，主代理裁决。Method/Scope: cold read-only full batch + forward repair consumer/test review。
Reviewed state: HEAD `3ce81caa0a1a864190df092da25eca9468ca25d0` + scoped SHA256 `43d05cdb0993516d266bbd728cf4f6f6d21aa5c43a6828cb96d6674a75c7c78b`。
Evidence: `/tmp/widget-batch-r2-red.log`、`widget-batch-r2-green.log`、`widget-batch-r2-build-red.log`、`widget-batch-r2-build-green.log`；main直接检查48Widgettests/fullsuite TEST SUCCEEDED、production模块排除、dev双appearance PASS和L0。
原14组browser颜色检查仍适用未改变的CSS/探针内容；不宣称原生Increase Contrast或桌面WidgetKit安装验收。
Completion gate: VERIFIED，候选 `43d05cdb0993516d266bbd728cf4f6f6d21aa5c43a6828cb96d6674a75c7c78b` 的3/3 required criteria通过；missing/invalidated/unresolved均为空。

### Task checkpoint

建议新候选门禁通过后签名forward commit并普通push；每次新head重新等待GitHubreview/CI，不改写历史。
