# v0.7.5 集成评审

## Round 1 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

本轮限 CI／Verify 修复的实际并入，版本 assemble 继续验证其余成员。epoch 11 发布授权覆盖本次合并。

- source `fix/ci-runner-and-verify-duration` 为 `a9126c1b1048e5d4eea6a1e8677e56a886ed761c`，pre-merge `main` 为 `ea4bd772c062ac1024c0eec871c29043634822f4`。候选预合并 `cddafae55c23a5bac451f6c019bc33a610791ff1` 的树为 `bea917364ab4fe13b69f0cc183186020416a607f`。
- PR 78 执行保留祖先的无冲突合并。GitHub 验签通过的 `929e95233a2658d3ed956c58cd88b01c16d7190c` 实际父提交依次为上述 target/source，树与已验预合并树一致。远端 main 指向该结果。
- 集成覆盖 Ubuntu 分类、文档及便携检查、macOS 原生与 Go 作业、race 分片标记及最终 `verify`。push `44512490714` 和 PR `44512718937` 各 9 项通过，三分片各事件合计 31 个无重复包。CEv1 源修复精确树 3/3 `VERIFIED`。独立评审通过，Codex 对同一 head 完成且零发现。[Push 运行](https://github.com/kitdine/agent-deck/actions/runs/44512490714) · [PR 运行](https://github.com/kitdine/agent-deck/actions/runs/44512718937) · [Codex 评审摘要](https://github.com/kitdine/agent-deck/pull/78#issuecomment-7008613990)

## Round 2 — 2026-11-27

评审结论：PASS

完成门禁：NOT_VERIFIED

版本 assemble 对账：其余成员已逐项经独立评审、PR Actions 与针对实际 head 的 Codex review 后并入 `main`。本轮记录并入身份；发布门禁复验与 L4 预检另行登记。

- 并入 v0.7.0（`4ac56db3`）之后的成员 PR：70 watch 中断退出码、71 原生 ownership 测试就绪、72 菜单栏 token 构成、73 turn 分类平方复杂度、74 Claude 额度 GUI 路径、75 全部客户端重复模型行、76 有界 serving 验证、77 刷新后陈旧扫描锁健康、78 CI 平台与 Verify 耗时、79 订阅账号身份、80 Codex 套餐显示、81 CI 重复运行与 macOS 并发、82 CI P3 载体文档、83 usage 测试含空格临时目录、84 CI 本地工作流模型、85 设置窗口 1x 布局振荡（`93a1a749`）、86 桌面扫描失败重跑（`470b0d74`）。
- 集成结果：`origin/main` = `470b0d74`。PR 85 与 86 均对实际 head 取得 Codex review 无发现，PR Actions 9 项全部通过；PR 86 合并前与 PR 85 结果做三方合并检查无冲突。
- 移出：`codex-subagent-jsonl-session-identity` 按用户决定移出本版本，未并入任何内容；`cask-uninstall-unregistered-widget` 维持 deferred。
- 新增成员的 CEv1：`fix:settings-window-layout-oscillation` 在 `13160c9b` 上三项 pass；`fix:desktop-scan-failed-retry-loop` 四项 pass。

## Round 3 — 2026-11-27

评审结论：PASS

完成门禁：NOT_VERIFIED

发布门禁复验：独立执行方 Codex/gpt-6.2-sol（reasoningEffort=high），以集成后 `main` `470b0d749ba0e24b457fe386d0e45a4f5999f2b8` 构建的未 strip helper，在当日重新以 `sqlite3 -readonly .backup` 取得的三库隔离拷贝与 APFS 克隆转录上，按 App 实际 bootstrap 路径运行；真实状态只在快照时只读访问，未对真实状态、App Group 或已安装 App 运行任何二进制。

- 冷启动、热启动与三轮追加转录重放共五轮，usage 与 session 全部 `completed`，无 `scan_failed`，无失败诊断条目；收据 round ID 与两域结果一致。
- 冷启动墙钟 145.28 s（上次诊断 v0.7.0 721.90 s、main 574.78 s）；冷启动全量会话扫描区间 4.307 s（上次 501.596 s、358.553 s），其中 2,752 个失效路径的 FTS 批量清理 2.209 s；热启动墙钟 9.60 s（上次约 36.6 s）。两次使用不同快照，对比同时受数据规模与负载影响。
- 合成失败验证诊断日志结构化字段与 0600 权限。Swift 侧失败退避与来源最小间隔属 App 行为，不在 CLI 复验范围，已由隔离原生测试与独立评审覆盖。
- 门禁结论：PASS，允许进入 L4 与发布预检。
