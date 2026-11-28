# 订阅账号存储兼容评审

## Round 1 — 2026-11-18

### 📋 quota-storage-compatibility

📊 综合评分：9.5/10

✅ 评审结论：PASS

### 🔴 必须修复

无。

### 🟡 兼容观察

- v27–29 无可靠失败时序时，状态栏窗口以前会覆盖失败。当前实现按 A7 采用保守顺序，`parse_failed` 屏蔽数字和提醒，旧使用者仍可按既有规则读取。这是任务 1 的版本化读取与兼容验收。
- 世代或周期保护生效后，旧 `PutEnvelopeFailure` 入口丢弃写入。新接线使用明确的 `CommitClaudeUsageFailure`、事务结果及当前世代。任务 3、4 仍负责守卫和运行时接线。

### 🟢 已核对

- 迁移 36 换代 quota 表，旧 schema 的只读查询仍可选择正确表和缺省列。迁移 31 沿用已批准的旧表链。已预热 WAL 旧写入者拒绝、多版本迁移和 schema 滞后快照的订阅字段重读均有测试覆盖。
- 同一周期的已提交身份、有效世代及 usage 成功才确认新上下文。捕获闸门另行提交。手动失败保留后台退避。套餐拒绝用私有分类位，原值不进入新表。便携快照与 Restore 清理额度、身份和关联残留。
- Swift 兼容上限为 36。主控对照 core 版本方向和五份 Foundation 夹具，隔离 verifier 通过。直接相关 Go 竞态用例、受影响包 `go vet` 与 `make check-privacy` 通过。

### 📝 小结

候选为 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 及 33 个代码、测试与 schema 夹具路径，指纹 `da0a6dcddb442232090ac066199d93b4bd37f7ec737e53e03c711c89d1f9511c`。独立冷评审由 T3 确认执行 Claude Sonnet 5.5，请求 `effort=medium`，实际推理档位未独立观察。评审方运行 store、quota、backup 与 desktop 包测试并通过。主控重算指纹，并独立核对两条观察、重新确认事务与 Swift 核心版本变化。

完成门禁：VERIFIED

精确候选 CEv1 通过 6/6。源码与测试保持一致。首次全量跑在其他检查并行时有两个未改动的 Claude CLI 用例触及子进程截止。其他检查结束后，`scripts/run-go-test.sh ./... -p 1 -count=1` 串行通过。记录保留此次运行条件及此前的失败事实。

任务检查点 `ad-subscription-account-identity-quota-storage-compatibility-dev`，目标 `subscription-account-identity:quota-storage-compatibility:state:da0a6dcddb442232090ac066199d93b4bd37f7ec737e53e03c711c89d1f9511c`。当前存储任务待提交，创建 PR 依来源 issue 的授权与完整验收范围。后续锚点交付真实目录投递守卫、运行时接线与原生验收。

### XCTest schema 夹具补证 — 2026-11-18

Task 3 的全量 XCTest 发现 `DesktopRefreshCoordinatorTests.swift` 仍期待拒绝实际已支持的 schema 36。主控把同一用例改为保留旧 35、接受当前最大值并拒绝最大值加一。当前 Swift 兼容上限仍为 36。定向 XCTest 执行通过。

补证后的 Task 1 候选为 `fa7d8c233054da5c3ce7fe636a03f6ef19045721e34bb543795caa9098b10caf`，原生产文件保持一致，变更仅为该测试文件 `2a1b2c14e67836c25755c635ab695eab77ed0e6b`。上文的 CEv1 VERIFIED 绑定原精确候选。交付时需要对实际新候选绑定适用证据。全量 Xcode 在该次重跑中停滞，未产生通过结果。

## Round 2 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

独立 Claude Sonnet 5.5 按 `effort=high` 核对重建的 33 路径交付候选 `7ab3e7168e1650fc9be26c2c1584fcb26df95ff25143e5e6c1004bdbd8174f04`，逐项复算 blob。候选以 `a224b0b824d2cad451d3bd83153832d65506f0fe` 为产品基线，交付基线 `6c7b5d407cd23f135718b3121b29814160194612` 的产品与工具输入一致。此裁决面向重建的 Task 1 切片。Round 1 和历史 XCTest 补证仍各有其确定的内容状态。

重建包含旧提醒 ID、版本化账本、未知失败顺序的保守判断，以及旧 schema 缓存订阅段重读。三张 quota 表在 schema 36 原子替换，迁移 31 保留。语义边界经独立复评核对，投递守卫与刷新预算由后续任务实现。`identity_storage_test.go` 仍覆盖 v27–35 的只读存储矩阵。世代为零的 Codex 提醒去重断言归 Task 3，保存在联合测试中。

**提醒账本保留。** 本切片采用经评审的运行条件。启用探测与提醒且有至少一个 official 客户端时，schema 36 的提醒评估会清理过期账本。两个客户端均非 official 时，本轮跳过清理。旧只读库跳过清理并读取可用的旧账本，低于 schema 29 时没有可读账本。此处保留差异影响清理时机，旧提醒 ID 与阈值判断沿用原规则。

**Task 4 交接。** Task 1 保守判断旧失败顺序，完整的 `BuildSubscription` 失败时刻展示归 Task 4。当前联合候选已实现 `FailureObservedAt` 缺失时使用 `FailureAt`。现有旧库矩阵验证失败原因及省略额度窗口。主控以只加 `failure_at` 显示断言的测试 overlay 验证 v27／28／29／30／35，五项通过。Task 4 交付切片须纳入这项断言。

独立核对了初轮 store、desktop、backup、cmd 的通过与 quota 的后续通过、定向 race、Swift Foundation 夹具以及隐私回执。主控补跑该候选的三项 schema XCTest，全部通过，并留下五个 Go 包范围 vet 和候选树空白检查退出 0 的明示回执。完整串行 Go 回归 `scripts/run-go-test.sh -p 1 ./...` 在精确候选上退出 0。

CEv1 门禁 `subscription-account-identity:quota-storage-compatibility:state:7ab3e7168e1650fc9be26c2c1584fcb26df95ff25143e5e6c1004bdbd8174f04` 通过 6/6。
