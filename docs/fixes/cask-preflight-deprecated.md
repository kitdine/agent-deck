---
status: active
created: 2026-09-26
---

# 缺陷：Homebrew cask 预检语法阻断 RC1 发布验证

## 现象

`scripts/test-macos-distribution.sh` 在 Homebrew 加载渲染出的
`agentdeck-app` cask 时拒绝弃用告警：`Calling \`preflight\` is deprecated!
Use \`preflight_steps\` instead.`。该脚本属于 `make release-verify`，因此
v0.6.0-rc.1 的技术预检不能通过。2026-09-26 在当前 main 与 Homebrew 7.0.6
上复现，原始日志保存在 `/private/tmp/agentdeck-rc1-cask-baseline.log`；测试
创建的临时 tap 已由脚本清理。

## 根因

`packaging/homebrew/agentdeck-app.rb.tmpl` 使用 `preflight do` 执行冲突 CLI
公式检查。当前 Homebrew 仍可加载它，但会发出弃用告警；发行测试正确地把
加载期告警作为失败。Homebrew 的 `preflight_steps` 是受限的声明式 DSL，不能
直接搬入原来的任意 Ruby 循环与异常，需要用路径守卫和失败的安装步骤表达
同一合同。

## 修复边界

按操作者 2026-09-26 的 Lane A 决定，仅替换 cask 预检语法及其直接测试。
`agentdeck` 和 `agentdeck-rc` 两个 CLI 公式都必须被拒绝，输出可读迁移指引；
Homebrew 拒绝安装后不能留下 Caskroom 收据、App 或命令链接，也不能触及
`~/.agentdeck`。不改变 Homebrew 发布渠道、应用安装内容或版本范围。

## 验证

原模板的 `bash scripts/test-macos-distribution.sh` 退出 1，加载期逐字报告
`Calling \`preflight\` is deprecated`。替换为 `preflight_steps` 后，同一
脚本 PASS，且其临时 Homebrew tap 已清理。`bash scripts/test-cask-migration.sh`
在本工具外层沙盒下先被 `sandbox-exec: sandbox_apply: Operation not permitted`
阻止，未进入产品断言；在许可的非沙盒环境以隔离 HOME、Cellar、Caskroom 和
应用目录重跑后 PASS，覆盖冲突公式拒绝、迁移文案与无残留回滚。修复后的原始
日志分别在 `/private/tmp/agentdeck-rc1-cask-repair.log` 与
`/private/tmp/agentdeck-rc1-cask-migration-unsandboxed.log`。

## Review — Round 1

待独立评审；实现阶段不自判 PASS。
