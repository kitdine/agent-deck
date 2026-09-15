---
status: active
created: 2026-09-13
---

# 缺陷：原生 XCTest 使用真实 state root 导致安装版不可用

## 现象

`snapshot-performance` 的原生 XCTest 启动 Debug `AgentDeck.app` 后，内嵌
helper 未带 `--state-dir` 使用真实 `~/.agentdeck`。真实数据库被迁移到
schema 25 后，仍为 v0.5.0 的安装版 helper 只能报告 `schema_ahead`，菜单栏
中的 provider 与 usage 数据不可用。测试退出后还留下了 Debug scan helper
和 worker。

Beads 记录：`ad-123k`。

## 根因

`scripts/test-macos-app.sh` 直接运行 xcodebuild，没有为 XCTest test host
创建或传入隔离 HOME。`AgentDeckApplicationDelegate` 仅在专项验收显式设置
`AGENTDECK_TEST_HOME` 时启用隔离，因此普通 App tests 构造默认
`EmbeddedHelperRunner`。runner 将真实 HOME 传给 helper，但 helper argv
没有显式 state root；统一 scan 随后从真实 home 推导 state，并启动独立
worker。

现场数据库的 `schema_metadata.version` 为 25；当前 worktree 支持 25，安装版
v0.5.0 helper 不支持。现场残留进程的可执行路径来自该 worktree 的 Debug
App，而不是安装版 App，因此安装版损坏或用户主动迁移不是最早因果偏差。

## 修复边界

- `EmbeddedHelperRunner` 从其受控 HOME 推导 state root，并将
  `--state-dir <HOME>/.agentdeck` 显式传给所有 helper 命令。
- Debug App 识别 XCTest host 后禁止自动 refresh，因此普通 App tests 不启动
  embedded helper。专项验收仍可启动真实 Debug App，但只接受 AgentDeck 的
  受控 `/tmp` 或 `/private/tmp` HOME 前缀。
- macOS XCTest 入口另为 xcodebuild 创建唯一临时 HOME，隔离 Core Foundation
  preferences；退出前只检测本次新增、来自当前 worktree 绝对 helper 路径的
  进程，清理也只终止这些精确进程。
- 不删除、不降级、不重建真实数据库。安装版恢复使用支持 schema 25 的临时
  CLI/helper，旧安装版保持原样直至单独授权的更新。

## 验证

- RED：`EmbeddedHelperRunnerTests.testUsesOnlyEmbeddedHelperAndArrayArguments`
  在 scan 与 snapshot 的实际 argv 均缺少显式 state root，产生两项精确断言
  失败。
- GREEN：同一聚焦 XCTest 在 runner 统一前置 `--state-dir` 后通过。
- 最终聚焦 XCTest：scan/snapshot argv、provider argv、XCTest 自动 refresh
  禁用及安全临时 HOME 三项回归均通过。
- `scripts/test-macos-app.sh` 完成 Debug App 构建；App tests 70 项通过、1 项按
  既有原生验收约定跳过，Widget tests 22 项通过，并且脚本没有发现本次新增
  的 Debug helper/worker。Shared tests 的 42 项中有 1 项既有流式进程测试在
  冷构建负载下以 5 秒预算超时；该测试的聚焦复现以相同预算 4.082 秒通过。
  这被记录为资源敏感的既有 test/harness 风险，本修复未放宽 timeout、跳过
  测试或修改其断言。
- `bash -n scripts/test-macos-app.sh`、`make check-whitespace` 和
  `git diff --check` 通过。
- 临时 CLI 在隔离 state 初始化 schema 25；独立 helper 的 doctor 对该 state
  报 schema 25 `ok`。临时恢复 App 的 ad-hoc codesign 验证通过。
- 真实 `~/.agentdeck` 及其权限受限备份均为 schema 25，SQLite
  `quick_check` 为 `ok`。真实统一 scan 因活动 Codex 源在扫描期间变化而在
  5 分钟边界原子拒绝提交；没有把该结果记为恢复成功，也未重复运行。
- 临时 CLI 随后的真实 state doctor 为 `healthy`，只读 desktop snapshot 为
  `partial=false`，provider、usage、sessions 均 available。临时恢复 App 已从
  `/private/tmp` 启动，其 helper argv 显式包含真实 `--state-dir`；安装于
  `/Applications` 的 v0.5.0 App 未被覆盖。

## Review — Round 1

## 📋 xctest-state-isolation 独立评审

📊 总体评分：9.5/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `EmbeddedHelperRunner` 从传给 helper 的同一受控 `HOME` 推导 state root，
  并通过统一入口为 scan、snapshot 和 provider 命令前置显式
  `--state-dir`，避免环境推导与实际 argv 分叉。
- Debug App 在 XCTest host 内禁用自动 refresh；专项验收 HOME 只接受
  AgentDeck 自有的 `/tmp` 或 `/private/tmp` 前缀，误传真实 HOME 会立即失败。
- XCTest 入口同时隔离 `HOME` 与 `CFFIXED_USER_HOME`，并只跟踪、终止本次
  测试新增且可执行文件绝对路径属于当前 worktree 的 helper/worker，不会清理
  既有或其他安装来源的进程。
- 回归断言直接覆盖原始缺陷的 scan/snapshot argv，并覆盖 provider argv、
  XCTest 自动 refresh 禁用和不安全 HOME 拒绝路径。

### 📝 摘要

评审对象为 `fix:xctest-state-isolation`，基线 HEAD `cbeaa4b2`，范围为
`AgentDeckApp.swift`、`DesktopPreferencesTests.swift`、
`EmbeddedHelperRunner.swift`、`EmbeddedHelperRunnerTests.swift`、
`scripts/test-macos-app.sh` 与本 fix 记录的最终候选内容。实现恢复了既有
XCTest 数据隔离契约，没有引入新的用户可见行为决策。

证据复用开发阶段同一产品候选上的 RED/GREEN 聚焦 XCTest、App 70 项、Widget
22 项和 Shared 41 项通过结果，以及 `bash -n scripts/test-macos-app.sh`、
`make check-whitespace`、`git diff --check` 和测试后无新增 Debug helper/worker
的检查。Shared 套件中一个未修改的既有流式进程测试在冷构建负载下超过 5 秒，
但同一测试以原 5 秒预算聚焦复现于 4.082 秒通过；该资源敏感性仍是 harness
残余风险，不构成本候选缺陷，也未通过放宽 timeout 或跳过测试掩盖。

逐项检查 helper 调用点后，所有实际 embedded helper 命令均经过统一
`helperArguments`；测试 runner 的隔离目录和进程匹配边界与修复记录一致。
未记录任何 finding，评审结论为 PASS。
