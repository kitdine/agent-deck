---
status: active
topic: subscription-quota-conversion
subject: quota-session-reporting
---

# 会话额度估值评审记录

## Round 1 — 2026-11-19

### 📋 会话额度估值独立评审

📊 总分 8/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `QSR-R1-F1` · 中。`cmd/agentdeck/main.go` 将 Tokens 改为只读打开后，`internal/usage/request_pricing.go` 仍要求 schema 32 新增的 `usage_derived_metadata`。升级后直接读取 schema 30 或 31 会返回原始 SQL 错误，Tokens 和估值一并丢失。独立诊断复现 schema 31，主控新增用例复现 30 和 31，35 和 36 仍通过。保持只读，让旧版本提供原有 Tokens 和明确的缺失测量。用 JSON、文本和交互 Tokens 核对旧会话索引、原有用量与数据库不变性。

#### 🟡 建议改进

- `QSR-R1-F2` · 低。`internal/usage/session_quota.go` 在采样提交后另开报告事务。此前计价读取在事务外读路由并比对修订号，并发入库可使已保存测量的 `finish` 返回错误。收窄该读边界或对临时修订冲突有限重试，保留代次门禁。

#### 🟢 已满足的部分

代次守卫、分页无关性、窗口金额和计价生效边界、同池竞争、排除项、定价缺口、已观测重置及隐私约束均得到核对。GUI 金标仅新增 `quota.sample.finish` 成功返回的 `data.quota_valuation`。

#### 📝 总结

工作区 `agent-deck.subscription-quota-conversion`，HEAD `467899ccf637557c98e7298db049f433651460e3`。15 个路径的评审指纹 `685eab60e7c51541f0dd55f5458aeec884ae26ea9f0a3745309745c550cbfd92`，CLI 手册 blob `5bc9456e8b55d4489b1a70c743677b2e4f157509`。计价依赖为签名提交 `d772a79edb20dc8d7a041a30cec24bb6937b0488`，采样基线实际工作树门禁已通过。

Claude Code Opus 5.5 冷上下文只读评审 `v080-quota-session-reporting-review-e10-a2`，请求档为 high，实际推理档未暴露。第一次派发在形成结论前达到额度上限，本次完成同一首轮评审。主控核对终态、候选指纹、源码与定向红测。原候选全仓 Go 回归通过 30 个包，1,881 项通过、8 项跳过，定向 race 通过 10 项。独立评审运行五项定向测试及 schema 31 隔离诊断。

完成门禁：NOT_VERIFIED

#### 下一步指令

修复：subscription-quota-conversion / quota-session-reporting / QSR-R1-F1、QSR-R1-F2

## Round 2 — 2026-11-19

### 📋 会话额度估值复评

📊 总分 9.5/10

✅ Verdict: PASS

#### 🔴 必须修复

- `QSR-R1-F1` 关闭。在一个只读事务中判定 schema、读取路由历史并选取计价规则。schema 32 起核对版本修订数据，30 和 31 可继续读旧用量。真实 JSON、文本及交互 Tokens 在 30、31、35、36 下均显示原有用量及缺失测量。核心库、会话索引及 WAL 内容一致，schema 未升迁，探测次数为零。本地夹具裁剪了相关新增表和索引，复评同时按实际迁移定义核对了旧版的查询依赖。
- `QSR-R1-F2` 关闭。路由和计价元数据使用同一个快照，采样保存后跨事务比对 revision 的竞态已消除。已有发布缓存的读内版本、发布和最终一致性校验仍适用。完成时的代次守卫及重放测试通过。

#### 🟡 建议改进

本轮无新发现。

#### 🟢 已满足的部分

原有归因、隐私、价格与窗口时间边界、分页和 GUI 金标适用。定向竞态 29 项、vet 及 Darwin amd64 和 arm64 构建通过。验证仅使用隔离夹具及现有设置。

#### 📝 总结

同一工作树及 HEAD。17 个路径的 R2 指纹为 `f376c3d31afd1737e45ddc3e98fcfa5c053b745f7e1d60c0e29b4dd5042a2a01`，按路径字节序拼接 `blob<TAB>path<LF>` 后取 SHA-256，CLI 手册 blob 与 R1 一致。相对 R1 变更为 `quota_session_reporting_test.go`、`internal/store/providers.go`、`internal/usage/request_pricing.go`。

Claude Code Opus 5.5 冷上下文只读复评 `v080-quota-session-reporting-rereview-e10-a1`，请求档 high，复评端可见推理值为 15。主控核对终态、候选和测试；独立复评直接核验 17 个 blob、旧版迁移查询依赖、缓存发布分支以及 CLI 和 usage 定向测试。全仓回归通过 29 个包，usage 包的两项固定查询数断言失败。移除事务外 revision 查询后，实际查询数各减少一次，测试仍要求旧次数。工作树后续仅修正计数断言，重新验证最终候选。

完成门禁：NOT_VERIFIED

## Round 3 — 2026-11-19

### 📋 会话额度估值查询数补验

📊 总分 9.5/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。QSR-R1-F1 和 QSR-R1-F2 的修复与 R2 一致。

#### 🟡 建议改进

无新增发现。

#### 🟢 已满足的部分

本轮仅调整 `internal/usage/usage_test.go` 的四条精确查询数断言。Summary 为 8 次，Sessions 为 9 次，Stats 的 3 条和 1,003 条事件夹具均为 9 次。价卡诊断的 6 次查询、金额、内容及大样本固定查询数断言保留。主控两项定向测试通过，独立检查重复三次共六项通过。R2 同一生产代码通过的定向 race 覆盖 3 个包、29 个顶层测试，vet 和 Darwin 双架构构建证据继续适用。

#### 📝 总结

工作树保持同一 HEAD，schema 37。18 个路径的 R3 指纹为 `53afe96f276bca57264f96bad02d5c0c45814f68e743df6f5b867989ed0fa064`，CLI 手册仍与 R1 一致。新增测试文件 blob 为 `8a7e8cf13185dfa1f72b9ee722ca8c0cf5163fbf`。额度工作树使用 `d772a79edb20dc8d7a041a30cec24bb6937b0488` 的签名计价源作为已物化依赖。两分支共同祖先为 `a224b0b824d2cad451d3bd83153832d65506f0fe`。

Claude Code Opus 5.5 冷上下文只读补验 `v080-quota-session-reporting-rereview-e10-a2`，请求档 high，实际推理档未暴露。主控核对终态、Git 祖先关系、18 个 blob、唯一测试差异及红绿日志。最终候选在现有工作树以 Go 1.27.1 运行 `scripts/run-go-test.sh -p 1 ./...`，30 个包通过，1,881 个顶层测试通过、8 个跳过，零失败。

完成门禁：VERIFIED

会话额度估值 WorkUnit `subscription-quota-conversion:quota-session-reporting` 的 ContentState 为 `subscription-quota-conversion:quota-session-reporting:state:53afe96f276bca57264f96bad02d5c0c45814f68e743df6f5b867989ed0fa064`。三项证据均适用，缺项、失效和未决影响列表为空。

#### 任务检查点

会话估值已在现有额度工作树完成本地验收。后续本地签名提交的任务范围包括 18 个实现与测试路径、CLI 手册、本记录及任务矩阵。

#### 专题检查点

现有额度工作树按 105 个代码、测试及手册路径合成的 ContentState 为 `subscription-quota-conversion:state:93ab9509f1b514a2f9f47a4a8be668651d1dcddfe140f9ac9c48b1921f011af9`。专题门禁 `VERIFIED 1/1`，复用窗口估值、采样存储和会话估值三项任务的确切证据。PR 和重基线由后续版本契约处理。
