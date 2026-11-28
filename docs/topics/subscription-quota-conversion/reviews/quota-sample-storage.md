---
status: active
topic: subscription-quota-conversion
subject: quota-sample-storage
---

# 额度采样存储评审记录

## Round 1 — 2026-11-19

### 📋 额度采样合成候选评审

📊 总分 7/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `QSS-C-R1-F1` · 中。CLI 的 schema 12、存储的 schema 33 和会话来源的 schema 34 测试从当前库构造历史库时，留下 `quota_session_samples` 和挂载在其他表的三个采样触发器。真实升级随后撞到已有对象。独立探针在移除四个对象后通过 12→37 升级。修正三个夹具，并断言常规升级达到当前 schema 且创建采样表。
- `QSS-C-R1-F2` · 中。`internal/store/pricing_facts_test.go` 以迁移列表末尾位置构造 schema 35。追加采样迁移后，夹具提前执行了计价迁移，导致注入预期冲突的操作失败。固定 schema 35 起点，保留迁移 36 的原子回滚断言，并验证正常打开重试后达到当前 schema。
- `QSS-C-R1-F3` · 低。`cmd/agentdeck/quota_sample.go` 的 `start`、`finish`、`cancel` 缺子命令简短帮助，`TestEveryLeafCommandHasActionableHelp` 失败。为三个动作提供操作说明。
- `QSS-C-R1-F4` · 中。`cmd/agentdeck/e2e_test.go` 未通过真实根命令采集三个成功响应。重生成 GUI 金标时三个 `quota.sample.*` 的成功结构都会变成 `null`。用隔离合成探测器跑真实命令，采集成功结构并核对敏感字段未输出。

#### 🟡 建议改进

- `QSS-C-R1-F5` · 低。`architecture.md` 把动作成功结果的时间字段描述为可空，当前 JSON 省略不适用字段。按动作明确应出现的字段，验证后关闭。

#### 🟢 已满足的部分

36→37 迁移冲突回滚与重试，代次条件发布，重放，账号及路由失效，较新额度缓存保护，用量清除和样本引用清理，备份及恢复的空闲页与 WAL 清理均通过本轮核对。Go 与 Swift 接受 36 的保留发布，拒绝 35 及超前 schema。App Group v1 回放通过。

#### 📝 总结

额度 HEAD `467899ccf637557c98e7298db049f433651460e3`，24 个源文件指纹 `6e8e82f9afbf219cd9b1cff5eae07031e6cf1b770b10bf3e1ee61956d16a750b`。签名计价依赖 `d772a79edb20dc8d7a041a30cec24bb6937b0488` 的 schema 为 36。两者共同基线是 `a224b0b824d2cad451d3bd83153832d65506f0fe`。在隔离副本启用迁移 37 和 CLI 注册，合成候选指纹为 `28b7136c5885948a50dc00099ae1e882a82aea05a2c61cae45809f07887d5ab3`，由计价提交、额度 HEAD、额度指纹及相对计价树按路径排序的 51 行 mode/blob/path 生成。

Claude Code Opus 5.5 冷上下文只读核验 `v080-quota-sample-storage-integration-review-e9-a1`，请求档为 high，实际推理档未暴露。主控核对候选、终态与 `scripts/run-go-test.sh -p 1 ./...` 的完整输出。27 个包通过，1,870 个顶层测试通过、8 个跳过、6 个失败，失败集中在 CLI 和历史升级夹具。六个受影响包的定向竞态检查、Darwin 双架构构建、Go vet、三项 Swift 测试及两套五夹具校验通过。

主控在签名计价基线上单独复现了 `TestIsolatedEndToEndFlow` 的旧金标差异。旧档把空时段的 API 金额采为 `null`。计价复评后的代码在确认零调用时输出完整零金额。共享金标在本次合成修复中由真实端到端输出重新生成。

完成门禁：NOT_VERIFIED

这是当前计价基线的隔离合成评审。待修复后复核组合内容，实际版本组装时按纳入的迁移定号。

#### 下一步指令

修复：subscription-quota-conversion / quota-sample-storage / QSS-C-R1-F1、QSS-C-R1-F2、QSS-C-R1-F3、QSS-C-R1-F4、QSS-C-R1-F5

## Round 2 — 2026-11-19

### 📋 额度采样合成候选复评

📊 总分 9/10

✅ Verdict: PASS

#### 🔴 必须修复

本轮合成候选的五项发现全部关闭。

- `QSS-C-R1-F1` 关闭。schema 12、33、34 的夹具移除样本表及三个触发器。三个用例均断言常规升级达到 `CurrentSchemaVersion` 并安装样本表，原有历史读取、恢复和超前 schema 断言保留。
- `QSS-C-R1-F2` 关闭。回滚测试从明确的 schema 35 起点构造，保留迁移 36 的冲突与原子回滚断言。重试后版本达到当前 schema，原有价卡及请求事实断言通过。
- `QSS-C-R1-F3` 关闭。三个采样子命令均提供简短操作帮助。
- `QSS-C-R1-F4` 关闭。端到端测试通过真实根命令和合成探测器记录 start、cancel、start、finish 的成功结构，断言三次探测和敏感字段未输出。GUI 金标由该测试生成，三个采样结构都与约定吻合。既有差异仅位于 `usage.stats` 和 `desktop.snapshot` 的成功结构，反映已通过计价复评的空时段完整零金额。常规运行的金标比较通过。
- `QSS-C-R1-F5` 关闭。合成候选的架构文档逐一约定开始、结束、取消时返回的时间字段，与 JSON 一致。

#### 🟡 建议改进

无本轮开放发现。完成响应中的 `quota_valuation` 与会话估值由后续 `quota-session-reporting` 交付。

#### 🟢 已满足的部分

Go 全仓、采样和备份定向竞态、真实 CLI 契约、Darwin 双架构构建通过。Swift 的方向兼容、超前 schema 和 App Group v1 三个定向测试，以及两套五夹具校验通过。Swift 输入与 R1 相同，首轮 Foundation 调用的参数顺序已纠正。

#### 📝 总结

同一额度 HEAD、计价签名提交与共用基线。R2 相对签名计价树包含 56 个路径，按 Round 1 的方法复算指纹为 `6ba959c705927f572a9da16550dcbc81b740f028429b78ee08e29362419ad918`。相对 R1 仅有 8 个变更，含 6 个 Go 文件、GUI 金标和额度架构文档。

Claude Code Opus 5.5 冷上下文只读复评 `v080-quota-sample-storage-integration-rereview-e9-a2`，请求档为 high，实际推理档未暴露。评审核验候选的 56 个 blob、完整树、相对 R1 的八项变更及金标，本轮没有新增发现。主控核验候选与终态，使用 Go 1.27.1 darwin/amd64 运行 `scripts/run-go-test.sh -p 1 ./...`，30 个包通过，1,876 个顶层测试通过、8 个跳过、零失败。两次 `-race -p 1 -run` 定向检查覆盖七个包，共 28 个顶层测试通过，包含 CLI、旧库迁移、采样、使用记录、serving 和备份恢复。Darwin arm64、amd64 构建均成功。Swift 校验使用隔离夹具和现有设置。

完成门禁：VERIFIED

主控按用户确认的顺序，将 R2 实现落到 `agent-deck.subscription-quota-conversion`。本地额度工作树使用 schema 37，已验收计价依赖固定为 `d772a79edb20dc8d7a041a30cec24bb6937b0488`。核对 100 个产品和测试相关路径与 R2 一致，重新在该工作树运行真实 CLI、端到端金标、编号迁移与 retained serving 检查，五项通过。

CEv1 WorkUnit `subscription-quota-conversion:quota-sample-storage` 的 ContentState 为 `subscription-quota-conversion:quota-sample-storage:state:5fed0fd53be1eb0b7f91df56cdee13648ead22ece874cb462044f6c4c88e4a03`，三项证据均适用，缺项、失效及未决影响列表为空。此本地验收交接给 `quota-session-reporting`。版本契约组装时按届时 `main` 复核迁移编号和兼容性。
