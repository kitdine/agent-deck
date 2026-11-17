---
status: active
created: 2026-11-17
---

# 缺陷：原生 ownership query 测试靠固定 yield 次数判断就绪

载体 `ad-bug-native-ownership-query-test-readiness`；v0.7.5；Lane A，test-only。
工作区 `agent-deck.fix.native-ownership-query-test-readiness`；分支 `fix/native-ownership-query-test-readiness`。用户随后明确授权评审和门禁通过后分别提交、推送 origin、创建 PR，并要求 Codex review；不包含 merge、release、deploy 或安装。下文未授权/未提交的早期表述保留为当时的检查点，当前交付状态见末尾补记。

## 现象

`testClosingWhileQueryIsInFlightCannotReacquireOwnership` 原先最多调用 1000 次 Task.yield 后就断言 SuspendedOwnershipQuery 已安装 continuation。调度延迟会使循环先耗尽，未就绪的任务也可能在清理时留下悬挂 await。原 issue 没有观察到该具体测试的 CI 失败；本修复不把受控探针当作真实 CI 事故。

## 根因

固定 yield 次数与实际 continuation 安装没有因果同步。结束操作先于 query 入口时，旧 helper 也没有保存可供后来入口直接返回的结果。

## 修复边界

- 仅修改 `apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift`，没有修改生产 runner、Swift package 或真实账号/状态。
- continuation 安装后调用显式 installed 回调；测试通过 XCTest expectation 等待实际状态，readiness 超时为 5 秒，失败即返回并执行 teardown。
- 测试 helper 保存 finish 的结果，处理 finish-before-query；teardown 关闭 session、取消任务、恢复 query 并等待 work.result，避免仅在成功断言路径清理。
- 保留 cancelled、真实 flock 互斥/释放及关闭后不能重新获取所有权的断言，不用 sleep 作为就绪判断。
- 受控延迟只用于隔离证明，临时探针已从最终文件移除。

## 验证

基线 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`。
主控直接核对唯一修改文件及 blob `3008219b4f337776f66d85cf67ff28b91de42c17`；Package.swift 未改，blob `25c1460a3c75d349ba897cf7003ae343afe0f3be`。
按 `head=<HEAD>;apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift=<blob>;apps/macos/Package.swift=<blob>` 生成的最终 SHA-256 为 `c33cc05fd0abfd567cde610fa6f356081b75e87364209bc2dfc99e5b8baa7d44`。

主控从原始 T3 command_execution 回读以下事实：

| 检查 | 结果 |
| --- | --- |
| 临时受控探针：query 入口延迟 300ms，原固定 yield 与修后等待对照 | 共 2 例，原做法 waiting=false 并失败；显式等待 completed 且测试通过；该次退出 1 是预期的原算法失败 |
| 修后两个关闭/query 定向测试 | Developer 报告退出 0，2 例 0 失败；最终更广的相关测试覆盖同一用例 |
| 最终 EmbeddedHelperRunnerTests，跳过需要外部隔离 fixture 的 receipt-driver 集成例 | 原始输出 exit0，34 例 0 失败；文件 blob 与上述最终身份一致 |
| git diff --check | exit0 |

实际运行使用 `swift test --scratch-path <isolated-scratch>`；最终选择 `--filter EmbeddedHelperRunnerTests --skip testNativeHelperAndGoReceiptDriverShareOwnershipLock`。临时目录已清理，不作为持久引用。
中间含探针的广泛执行还遇到未提供 native harness fixture 的原有 receipt-driver 用例；这是未运行完整 `scripts/test-macos-app.sh` 的环境前提，不将其冒称通过或作为本修复改动。

工具链为 Swift6.3，Xcode26.4（17E192），macOS darwin/amd64。主控已使用新工作树自己的 CodeGraph index 同步；该工具对本查询主要返回生产依赖，测试改动以精确 diff 与原始测试结果核对。
没有重跑不受 test-only 改动影响的 Go 全仓，没有操作已安装 App，也没有真实 CI 失败或原生 GUI 验收声明。当前独立评审及 `fix:native-ownership-query-test-readiness` gate 尚未完成。

## Review — Round 1

完成门禁：VERIFIED

格式补记（2026-11-17）：对应本轮精确候选 `c33cc05fd0abfd567cde610fa6f356081b75e87364209bc2dfc99e5b8baa7d44` 已查询的 3/3 结果；保留测试范围、独立裁决与未运行项，不追溯声称评审者查询了 CEv1。

方法：冷上下文 Codex/gpt-6.1-sol，requested reasoningEffort=medium，独立只读代码/测试评审。主控核对源变更、Developer 原始测试结果、Reviewer 对 HEAD/两个 blob 的独立核对及其终态无待执行子操作后记录正式结论。

### 📋 独立代码与测试评审

📊 总体评分：9.5/10

✅ 评审结论：PASS

#### 🔴 严重问题 — 必须修复

无。

#### 🟡 建议改进 — 推荐

无范围内未解决事项，不生成虚构 finding ID。

#### 🟢 优点及核对

- continuation 保存后才发出 installed 信号；actor 串行化使 finish 无法破坏安装顺序，等待有实际因果关系。
- finish-before-query 保存返回值，重复 finish 不会重复恢复 continuation。
- 断言失败与 readiness 超时仍走 teardown；先完成 session/task 清理，再删除临时账号目录。
- 成功断言前没有 cancel work，cancelled 仍检验 session.close 的效果；真实 flock 与关闭后不可重获所有权的断言保留。
- 未运行的 fixture-dependent receipt-driver 例不使用被修改的 SuspendedOwnershipQuery，源影响检查确认它不是本改动的验证缺口。

#### 📝 总结

被评指纹 `c33cc05fd0abfd567cde610fa6f356081b75e87364209bc2dfc99e5b8baa7d44`，HEAD 和两个 blob 与冻结目标一致。复用主控已核验的受控延迟对照、最终 34/0 和 diff 检查，本轮没有重复测试。结论仅覆盖此次测试修复，不声称完整 App harness、GUI 或真实历史 CI 复现。

Task checkpoint：`fix:native-ownership-query-test-readiness`，候选如上；提交建议：唯一 Swift 测试文件及本 fix 记录，等待最终 gate 和明确提交授权；推送建议：无当前授权，等待授权提交及相应远端/CI/签名检查。没有包含 Topic 完成边界。

## 最终门禁与交付边界

CEv1 对 `fix:native-ownership-query-test-readiness:state:c33cc05fd0abfd567cde610fa6f356081b75e87364209bc2dfc99e5b8baa7d44` 查询为 **VERIFIED 3/3**：受控延迟就绪、相关原生回归与独立评审均有目标绑定证据。
Beads 进入 awaiting_commit；代码与本记录仍未提交，未推送、安装或发布。提交建议为该单任务边界，推送仍须另行授权。原 Bug 不因本地验收被关闭。

## 交付与 CI 格式补记 — 2026-11-17

已签名提交 `7ff4185e8cd28c80ea7175e5f2586ef607217fa0` 已推送 origin 并创建 [PR #71](https://github.com/kitdine/agent-deck/pull/71)；其提交态 gate 为 VERIFIED 3/3。原 Bug 保持 in_review，等待远端 Actions 与 Codex review。

首轮 CI 的 `python3 scripts/check-docs.py --base 4ac56db32e13c4e925b648c620ba76fc588a7781 --head 7ff4185e8cd28c80ea7175e5f2586ef607217fa0` 报本轮缺可解析的完成门禁字段。本文补齐唯一的规范字段，并同步新的交付授权；唯一 Swift 测试文件与 Package.swift 均不变，不重跑未变产品验证。文档修正通过同一 L0 后另作追加提交，不改写原提交。
