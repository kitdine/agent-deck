---
status: active
topic: cost-transparency
subject: pricing-facts-storage
---

# pricing-facts-storage 评审记录

## Round 1 — 2026-11-18

### 📋 计价事实存储实现评审

📊 总分 7/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-PFS-R1-F1` · 高 · 本地价卡对象入口允许无效日期、价格、组件和来源进入不可变目录。隔离 CLI 回归中，六份无效文件均被接受，随后 `price status`、`price list` 和 `usage summary --no-scan` 因 `effective_from: "yesterday"` 读取失败。导入事务开始前校验模型日期、非负价格、已知组件和 HTTPS 来源。补拒绝导入并保持报告可读的回归。处置为开放。
- `CT-PFS-R1-F2` · 中 · 相同版本的规则和内置价卡以编码字节判断冲突。向已保存规则 JSON 增加空白或重新缩进同一份内置价卡，都导致幂等重导入失败。扫描入口需要导入内置版本，因此只改编码也可能阻塞扫描。按已校验的规则摘要和旧模型表语义判定，保留真正的内容冲突与原始导入出处。处置为开放。

#### 🟡 建议改进

任务 3 需在解释器中确定如何区分旧库缺少事实与调用边界未证实，并核对请求档参考所需的证据。它们属于逐调用计价交接。

#### 🟢 已满足的部分

迁移、重复扫描、事件事实、备份恢复和 Go/Swift schema 边界得到复核。v1 金标只调整 schema 诊断。独立评审的定向 Go 检查通过。原生 opt-in 横幅渲染没有运行。

#### 📝 总结

状态为 HEAD `a8514d14798f84534e026ac6493e186088750c60`、29 路径指纹 `0c2343f62a162e04f1f6bd35215d891669bcadc005407351edbe1c19fa835fad`。独立方法为 Claude Code Opus 5.5 冷上下文只读评审 `v080-pricing-facts-storage-review-e4-a1`。实际推理档未观测。主控在相同产品代码上用隔离状态复现两项发现，新增回归均先失败。

完成门禁：NOT_VERIFIED

该候选的 CEv1 查询返回 `missing_work_unit`。两项发现关闭后对修订候选独立复评。

## Round 2 — 2026-11-18

### 📋 计价事实存储复评

📊 总分 9/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。

- `CT-PFS-R1-F1` 已关闭。官方价卡对象在开事务前校验无凭据的 HTTPS 来源、RFC 3339 日期、已知组件和非负价格。六种拒绝用例证明仅原价卡留在历史中，`price status`、`price list` 和 `usage summary --no-scan` 仍然可读。
- `CT-PFS-R1-F2` 已关闭。内置价卡的原文编码不同时，先核对旧模型表，规则以重新校验的语义摘要判定。两种重编码可重试，更改 v1 费率或 v2 规则仍报冲突。原始来源摘要和投影修订保持原值；旧库缺失规则时在同一事务补写。

#### 🟡 建议改进

可在未来需要扩展导入契约时补充官方价卡重编码的独立测试、内置规则删除与补写的组合断言。当前行为已由存储实现、既有碰撞和补写测试以及定向复评覆盖。请求档与缺失事实的解释继续走任务 3 交接。

#### 🟢 已满足的部分

复评确认标准文件入口、事务、来源记录、已存规则和旧模型表的兼容边界与设计一致。修订仅四个相关文件，其余 25 个候选文件沿用 Round 1 的评审。Swift、迁移和 v1 fixture 身份一致，复用原生证据。

#### 📝 总结

HEAD `a8514d14798f84534e026ac6493e186088750c60`，基线树 `44887973fe6bc638c791c37d05e3e04105a35f05`，29 路径指纹 `8807c8ac4e891454262d1a774112fe5f1cdd95c8fcf9a5b50f0ea9eb6323c1e6`。指纹按路径字节序排列 `<git hash-object --no-filters blob><TAB><path><LF>` 后取 SHA-256。Claude Code Opus 5.5 冷上下文只读复评 `v080-pricing-facts-storage-rereview-e5-a2`，配置 `high`，实际推理档未观测。主控核对 29 个候选 blob、终态及回归证据。

主控运行 `scripts/run-go-test.sh -p 2 ./internal/usage ./cmd/agentdeck ./internal/store ./internal/desktop ./internal/backup` 和定向竞态检查通过。Go 构建、vet、gofmt、空白和文档检查通过。复评者另外在 Go 1.27.3 上运行 39 个通过的价卡、导入与 CLI 竞态用例。复用的 Xcode 26.4 证据包括共享套件 182 项零失败、App 套件 256 项三跳过零失败，两项重启分别通过，两个 Foundation 验证器覆盖五个 v1 fixture。原生 opt-in schema 横幅渲染未运行。迁移 36 属于当前 pre-v0.7.5 组合，变更基线时重审编号与兼容。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:pricing-facts-storage`，ContentState `cost-transparency:pricing-facts-storage:state:8807c8ac4e891454262d1a774112fe5f1cdd95c8fcf9a5b50f0ea9eb6323c1e6`。三项必要证据适用，缺项、失效与未决影响均为空。

#### 任务检查点

本地签名提交包含这 29 个已评审的产品、测试、fixture 和手册文件，以及本评审记录和任务矩阵的任务 2 状态。完成本地提交并验证实际提交树后交给 `request-pricing` 与 `quota-sample-storage`。版本 PR 处理按操作者要求暂缓。
