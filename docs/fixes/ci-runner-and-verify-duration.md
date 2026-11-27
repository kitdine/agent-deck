---
status: active
created: 2026-11-19
---

# 缺陷：CI 分类依赖 macOS runner，Verify 串行 race 接近一小时

## 现象

主线 `45dd1f6` 的两次 CI 尝试中，`classify` 在步骤开始前被取消。第二次没有分配 runner，文档门禁因缺少分类输出而失败。相同推送区间在本地能解析出已知文档范围。[CI 基线](https://github.com/kitdine/agent-deck/actions/runs/44306059099)

主线 `ea4bd77` 的成功基线中，`classify` 和 `documentation` 分别排队 3 秒，`verify` 排队 2 秒。`Verify` 步骤执行 58 分 32 秒，其中普通 Go 测试约 3 分 26 秒，串行 race 约 54 分 57 秒。该运行显示 setup-go 命中缓存，旧 `Verify` 日志仅输出包测试的保留路径。候选会单独计时 Go 测试与 vet，并输出每个包的结果和耗时。[成功基线](https://github.com/kitdine/agent-deck/actions/runs/44474655411)

## 根因

`classify` 使用 Git、Bash 和 Python，文档门禁使用 Git、Python 和 Node。两项均占用 macOS runner。主要执行成本集中在一个 `macos-15` 作业内按 `-p 1` 依次执行各 Go 包的 race 测试。CI 的 setup-go 依据 `go env GOCACHE` 缓存构建产物，现有 Makefile 默认将实际检查指向另一个目录。

## 修复边界

2026-11-19 用户确认实施并交付本项。

| 工作流与作业 | 关键依赖 | runner |
| --- | --- | --- |
| CI `classify`、`documentation` | Git 完整历史、Bash、Python、Node 22 和仓库文档工具 | Ubuntu |
| CI `portable`、`verify` | 文件卫生、测试包装器自测和依赖作业状态 | Ubuntu |
| CI `go_checks`、三个 `race` 分片 | Go、macOS 安装与 SQLite 相关测试。保留 race 的单包并行上限及 20 分钟测试超时 | macOS 15 |
| CI `desktop` | Xcode 26、Swift、App 和 Widget 构建、沙箱与分发检查 | macOS 26。纯文档路径使用既有 Ubuntu 通过分支 |
| Cask real Homebrew、Published RC Cask | 实际 Homebrew、App、Widget、系统卸载与 Cask 安装 | 保留 macOS 26 |
| Release Preflight | arm64 工件、安装预检、Xcode 和 Swift 桌面验证 | 保留 macOS 26 |
| Release `release`、`homebrew`、`desktop` | arm64 发布验证、Homebrew、Xcode 26、签名与公证 | 保留现有 macOS runner |

`verify` 继续作为合并门禁。仅在可信分类确认纯文档且文档检查成功时走纯文档路径。其他路径要求可移植检查、Go 测试与 vet、全部 race 分片和文档检查都成功。汇总同时检查矩阵结果与每个分片在测试成功后写出的独立标记。文档步骤仍由已知 diff 决定验收范围。race 由 `go list ./...` 确定平台包集，CLI 独占一个分片，其余按排序交错分成两份。各 CI Go 作业使用 setup-go 的构建缓存路径。本地 `make verify` 继续运行完整验证。

## 验证

本地验证覆盖文档工具、包装器输出与权限、包分片完整性和 `verify` 的成功及失败分支。Linux 的执行和 GNU `stat` 路径由实际 Actions 验证。真实 push 与 PR 完成后记录实际排队、步骤耗时、缓存命中、包耗时及临界路径，并按最终候选对照上面的基线。

## Review — Round 1

评审结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 核对 HEAD `ea4bd772c062ac1024c0eec871c29043634822f4` 与八路径候选 `a8313f89289e5035ead94aaa51aa6927194eda6a97adcdd0fb995a91eef3d0dc`。本地验证通过，包括 `actionlint`、54 项 CI 工具测试、93 项 Python hook 测试、14 项 Node hook 测试及包装器自测。实际 Darwin 包集 31 个，包含一个 CLI 包和两组各 15 个包。分片命令失败时不产生成功标记。即使矩阵返回成功，缺少任一标记也会阻止 `verify`。独立评审确认可信分类、文档失败门禁、缓存路径和 `verify`、`desktop` 标识，未发现阻断问题。

此轮核对本地候选。签名提交和 PR 用于真实 push 和 PR 验证。托管 Ubuntu、macOS 分片标记传递、实际缓存与排队、端到端耗时，以及合并前要求仍待核对。
