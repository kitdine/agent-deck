---
status: historical
created: 2026-10-03
retired: 2026-10-03
---

# 缺陷：App Group 容器不可用时菜单栏不报告 Widget 发布失败

## 现象

Beads: `ad-bug-widget-publisher-unavailable-silent`。release 基线 `71f6f2145b6936dea67ff81baed045687cc6f1e2` 上，nil App Group store 的 full/quota 发布保持 `neverPublished`，菜单栏无已有 Widget notice。隔离 XCTest 已复现。

## 根因

`DesktopRefreshCoordinator.publishWidgetSnapshot` 在推进 generation 和记录结果前对 nil publisher 返回。A4 的菜单栏数据、full attempt、Widget publication 是独立状态；失败不应令新菜单栏数据 stale。`WidgetSnapshotPublisherIssue.storageUnavailable` 和现有 notice 已定义，无需新错误码或 copy。

## 修复边界

用户明确授权本批现有契约 Lane A 端到端迭代。workspace `agent-deck.health-source-truthfulness-batch`，branch `fix/health-source-truthfulness-batch`，唯一产品 writer session `01a0ff3b-0fcc-7291-9a40-19c621cb649e`。

先推进一次 publication generation；nil publisher 记录 `failedBeforeCommit(storageUnavailable)`。有 publisher 的写入、reload 和结果处理不变。存储依赖在 coordinator 初始化时固定：恢复测试通过重新创建带可用隔离 store 的 coordinator 验证现有生命周期，不新增热切换或重试接口。合成健康菜单栏 fixture 使用独立 store 并在 teardown 清理；nil fixture 专门覆盖失败。

## 验证

- 共享层 RED：`swift-red-valid.log` 中 full generation 1、quota generation 2 均实测 `neverPublished`；恢复测试的初始失败断言也失败。
- App RED：`swift-app-red.log` 中现有 notice 数量为 0、文案 nil；安全临时 HOME 下正常执行。
- 前置命令的错误 target、缺失测试构造依赖，以及不符合既有 safe-home 前缀的 App-host 启动崩溃均单独保存，不计作产品 RED。后者 stack 指向 `AgentDeckApp.swift:153`；修正隔离路径后恢复正常测试执行。
- 回归覆盖 full/quota 独立 publication generation、新菜单栏数据不 stale/不 badge、单一已有 notice、coordinator 重建后成功写入。publisher 单次写入与语义 reload 由其未修改的既有测试覆盖。
- 原始日志和结果包：`/tmp/agentdeck-health-source-20261002/`。最终结果、冷评审与 exact-state 身份见本记录的 review round。
- 原生菜单栏图标验收仍为用户 deferred；本批不宣称完成整体 v0.6.5 contract/assemble。

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

nil 分支先推进 generation 后记录现有 typed failure；有效 publisher 路径保持。三个新 XCTest 保护原缺陷，健康 fixture 独立并带 teardown。

### 📝 总结

- Reviewed state: HEAD `71f6f2145b6936dea67ff81baed045687cc6f1e2` + scoped SHA-256 `5f071b2487f30b3be055e5e8acb8b1a60688663946f76ad3376f755aabeaf26c`。精确 15-path manifest 在 `/tmp/agentdeck-health-source-20261002/candidate.manifest`；载体使用截至 `review-history` marker 的冻结 prefix，避免报告自引用。
- Reviewer: 全新 CLI session `01a0ff51-3744-7b52-ae66-94843dea03d7`，`gpt-6.1-sol xhigh`，official/default，read-only/never。Method: 单评审者冷读，0 子代理，不继承 writer 会话或 memory；先代码/契约/断言后载体。主 writer 逐项核对源码分支、日志与范围后录入本报告。
- Scope: nil App Group 的 full/quota 发布、已有 notice、生命周期恢复与 publisher 不变量。路径：apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift; apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift; apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift; 本 Fix 载体。
- Evidence: `go-red.log`、`go-green.log`、`swift-red-valid.log`、`swift-app-red.log` 分别记录有效 RED 与对应恢复；全量 `scripts/run-go-test.sh ./...`、`scripts/run-go-test.sh -race ./internal/desktop ./internal/quota`、`make vet check-arm64-size`、`make test-macos-app` PASS。XCTest Shared 85、App 139、Widget 48，零 failure，1 项既有 opt-in native schema matrix skip。`make check-whitespace`、`git diff --check` 无诊断。日志与完整冷报告位于 `/tmp/agentdeck-health-source-20261002/`，未重复广泛验证。
- Completion gate: VERIFIED。WorkUnit `fix:widget-publisher-unavailable-silent`，ContentState `fix:widget-publisher-unavailable-silent:candidate:round1:01a0ff3b`；required criteria 7/7，missing/invalidated/unresolved 均为空。节点与关系批次已直接 readback，39/39 关系一致。
- 限制：恢复是隔离 store 与 coordinator 重建，不证明真实 App Group 恢复；较新 status-line 正向 supersession 主要通过未改变的源码条件与调用链审查，新增矩阵不覆盖该正向场景。原生菜单栏图标仍 deferred，schema skip 不是验收；整体 v0.6.5 contract/assemble 未完成。

### Task checkpoint

- Task: `ad-bug-widget-publisher-unavailable-silent`；独立 PASS，exact candidate gate VERIFIED，无修复项。
- 提交建议：以该 Task 的代码、测试和本 Fix 载体组成一个 signed logical commit；仅上述路径，不含 binding、contract topic 或全局 status。
- 推送建议：两项 logical commits 完成对象/SSH 签名检查并把可复用证据绑定最终 immutable HEAD 后，普通 push 本批 fix 分支；draft PR 指向 `release/v0.6.x`，待 exact-head GitHub Codex review/CI 通过再 merge commit。

## Delivery and retirement — 2026-10-03 UTC

- Logical commit: `37a824e8f80afe0d97768856d25595009c2dbc2a`，actual subject/body/Codex trailer、four-path scope 与 SSH signature 已核实。
- Reviewed/pushed batch HEAD: `1b75cd6966641cc5e144332772393b3f1572002d`；[PR #34](https://github.com/kitdine/agent-deck/pull/34) 指向 `release/v0.6.x`。
- GitHub Codex review: [exact-head report](https://github.com/kitdine/agent-deck/pull/34#issuecomment-5964149712) 明确 reviewed `1b75cd6966`，无 review/inline findings。Push 与 PR 的四个 desktop/verify CI checks 均 SUCCESS，全部 head SHA 匹配；证据在 `/tmp/agentdeck-health-source-20261002/product-checks-final.json`。
- Actual merge: `62863c804f4af665af767c5a54ecc366c74609c8`；parents `71f6f2145b6936dea67ff81baed045687cc6f1e2`、`1b75cd6966641cc5e144332772393b3f1572002d`；tree `74542556a484fa35afc15316e583abe9b2ae3b47` 与 source 相同，GitHub signature verified/valid；API 与 fetched Git object、remote release ref 已比较。
- WorkUnit `fix:widget-publisher-unavailable-silent`，merge ContentState `fix:widget-publisher-unavailable-silent:merge:62863c804f4af665af767c5a54ecc366c74609c8` 的 gate VERIFIED 7/7，无 missing/invalidated/unresolved。15 protected blobs 对 Git object 完全匹配，使用明确 target-bound roll-up 与 scope-aware preserves assessment；无产品检查重跑。
- 本退役严格在 product merge 与 required evidence finalization 后发生；归档按另一个合法 PR 交付，无直接 shared-release push。历史 review 与证据身份保留；本批仍不代表 v0.6.5 contract/assemble 或 deferred 原生验收完成。
