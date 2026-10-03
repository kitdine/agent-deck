---
status: active
created: 2026-10-03
---

# 缺陷：解析失败投影错误地沿用上一次成功探测的来源

## 现象

Beads: `ad-bug-quota-parse-failure-source-misattribution`。release 基线 `71f6f2145b6936dea67ff81baed045687cc6f1e2` 上，先 status-line 成功、后 Claude prose 解析失败的 failure-only 投影把本次失败时间与旧 source 配对；首次失败 source 为空。

## 根因

`subscriptionClient` 的失败分支使用从成功 window/envelope 选出的 source。C5 要求来源贯穿投影，requirements clause 6 要求 parse failure 展示失败时间且不重呈旧值。失败 envelope 的 route 已确定：Claude prose 或 Codex app-server；`PutEnvelopeFailure` 不提供成功记录的 source，不能依赖旧 window 推断。

## 修复边界

用户明确授权本批现有契约 Lane A 端到端迭代。workspace `agent-deck.health-source-truthfulness-batch`，branch `fix/health-source-truthfulness-batch`，唯一产品 writer session `01a0ff3b-0fcc-7291-9a40-19c621cb649e`。

仅 failure-only 分支从 client 的既有 envelope probe route 取得 source；沿用现有 wire 名。保留成功 source 选择、更新 status-line supersession、probe_failed 保留旧 figure/age 的行为；不改 schema、探测、provider gate 或实际用户状态。

## 验证

- `go-red.log`：五个子场景均仅在失败 source 断言上失败，包括旧 status-line 后 Claude parse、Claude/Codex 首次 parse/probe failure。
- `go-green.log`：修复后全部 `TestBuildSubscription` 回归 PASS；失败时间、reason 和无 figure 断言保持。
- 所有存储和 HOME 来自 `t.TempDir()`，无网络 probe 或真实账号数据。
- 原始日志：`/tmp/agentdeck-health-source-20261002/`。最终 full/race/build、冷评审与 exact-state 身份见本记录的 review round。
- 本批不修改 `feature/v0-6-5-contract`；整体 contract/assemble 和原生菜单栏图标 deferred 验收不由本记录关闭。

<!-- review-history -->

## Review — Round 1 — 2026-10-03

## 📋 独立冷读交付评审

📊 总体评分: 9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。Findings: None；无未关闭的 in-scope finding。

### 🟡 建议改进 — 推荐

无 actionable finding。限制按证据边界保留，不扩展本批契约。

### 🟢 优点

五场景真实隔离 SQLite 回归证明失败 source/时间/reason/空 windows；变化只在 failure-only 返回分支，probe_failed retained figure 与 status-line 条件保持。

### 📝 总结

- Reviewed state: HEAD `71f6f2145b6936dea67ff81baed045687cc6f1e2` + scoped SHA-256 `5f071b2487f30b3be055e5e8acb8b1a60688663946f76ad3376f755aabeaf26c`。精确 15-path manifest 在 `/tmp/agentdeck-health-source-20261002/candidate.manifest`；载体使用截至 `review-history` marker 的冻结 prefix，避免报告自引用。
- Reviewer: 全新 CLI session `01a0ff51-3744-7b52-ae66-94843dea03d7`，`gpt-6.1-sol xhigh`，official/default，read-only/never。Method: 单评审者冷读，0 子代理，不继承 writer 会话或 memory；先代码/契约/断言后载体。主 writer 逐项核对源码分支、日志与范围后录入本报告。
- Scope: failure-only 来源、首次失败、成功 precedence 和 retained-figure 语义。路径：internal/desktop/subscription.go; internal/desktop/subscription_test.go; 本 Fix 载体。
- Evidence: `go-red.log`、`go-green.log`、`swift-red-valid.log`、`swift-app-red.log` 分别记录有效 RED 与对应恢复；全量 `scripts/run-go-test.sh ./...`、`scripts/run-go-test.sh -race ./internal/desktop ./internal/quota`、`make vet check-arm64-size`、`make test-macos-app` PASS。XCTest Shared 85、App 139、Widget 48，零 failure，1 项既有 opt-in native schema matrix skip。`make check-whitespace`、`git diff --check` 无诊断。日志与完整冷报告位于 `/tmp/agentdeck-health-source-20261002/`，未重复广泛验证。
- Completion gate: VERIFIED。WorkUnit `fix:quota-parse-failure-source-misattribution`，ContentState `fix:quota-parse-failure-source-misattribution:candidate:round1:01a0ff3b`；required criteria 6/6，missing/invalidated/unresolved 均为空。节点与关系批次已直接 readback，39/39 关系一致。
- 限制：恢复是隔离 store 与 coordinator 重建，不证明真实 App Group 恢复；较新 status-line 正向 supersession 主要通过未改变的源码条件与调用链审查，新增矩阵不覆盖该正向场景。原生菜单栏图标仍 deferred，schema skip 不是验收；整体 v0.6.5 contract/assemble 未完成。

### Task checkpoint

- Task: `ad-bug-quota-parse-failure-source-misattribution`；独立 PASS，exact candidate gate VERIFIED，无修复项。
- 提交建议：以该 Task 的代码、测试和本 Fix 载体组成一个 signed logical commit；仅上述路径，不含 binding、contract topic 或全局 status。
- 推送建议：两项 logical commits 完成对象/SSH 签名检查并把可复用证据绑定最终 immutable HEAD 后，普通 push 本批 fix 分支；draft PR 指向 `release/v0.6.x`，待 exact-head GitHub Codex review/CI 通过再 merge commit。
