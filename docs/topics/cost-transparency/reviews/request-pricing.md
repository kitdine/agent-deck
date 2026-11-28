---
status: active
topic: cost-transparency
subject: request-pricing
---

# request-pricing 评审记录

## Round 1 — 2026-11-19

### 📋 逐调用计价评审

📊 总分 8/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-RP-R1-F1` · 中 · Codex 逐笔计数。末次调用缺写入总量但含两个零值 TTL 桶，已准入增量实际含 5 个写入 token。`pricingCounts` 将桶合计直接当作总量，隔离的 `Scan` 和 `PriceInvocations` 产出 `complete`、`0.000880000`，缺少结构化原因。桶合计推导写入总量时须核对已准入计数，冲突时阻止精确金额。加入经过实际扫描和读入口的回归。处置为开放。

#### 🟡 建议改进

交接报告任务时记录 Codex 生产者执行档、端点和审批数据的可用性，以及用量事实版本向量。未获上游证实的 full-session 与分段规则依据当前契约保留未证状态。

#### 🟢 已满足的部分

读事务和归因代次、价卡历史切换、语义优先级、公开 API 与订阅和 Enterprise 渠道、手工夹具的 Guardian 资格、额度交接字段与旧 v1 金额得到核对。真实采集的已知缺口保持未知。

#### 📝 总结

状态为 HEAD `57330f13f08276f25177b3bfe53731b4d001d1eb`，树 `2945ba1b5d05d17b2f66994214c9cb016bc4a65f`，六路径指纹 `1d311598c9df106756b0aa266465715febd0a2b5fce331e4b396c3cfb8143dda`。Claude Code Opus 5.5 冷上下文只读评审 `v080-request-pricing-review-e6-a1`。实际推理档未观测。主控核对精确候选和 Go 日志。全仓 Go 30 包通过，共 1,836 个顶层测试通过，8 个跳过。定向竞态在 usage 和 store 运行，包含 usage 的便携恢复夹具。独立评审通过临时覆盖文件在正常入口复现计数问题。

完成门禁：NOT_VERIFIED

该候选的 CEv1 查询返回 `missing_work_unit`。修复第一项发现后针对新指纹复评。

## Round 2 — 2026-11-19

### 📋 逐调用计价复评

📊 总分 9/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-RP-R1-F1` 已关闭。只在 Codex 的两个 TTL 桶合计与已准入写入同值时推导总量。冲突进入 `unavailable` 与 `conflicting_counts`。真实扫描夹具捕获原来 `complete`、`0.000880000` 的回归，旧会话金额仍相等。
- `CT-RP-R2-F1` · 低 · 正向回归缺口。独立评审在隔离探针证实合计一致的推导能产生精确金额。将其一律当成冲突时原有定向测试仍全部通过。补充合计一致时的精确金额断言。处置为开放。

#### 🟡 建议改进

原始计数列的零缺少字段存在性证据时，后续可以区分缺证与冲突的诊断。当前保守输出保留金额缺失，不影响合同金额和额度分子。

#### 🟢 已满足的部分

生产计价修复、旧 v1 读数和 Claude 行为得到独立核对。主控保留工作区的既有范围。

#### 📝 总结

HEAD `57330f13f08276f25177b3bfe53731b4d001d1eb`，树 `2945ba1b5d05d17b2f66994214c9cb016bc4a65f`，六路径指纹 `9cdad953798538ba01b41de61d320b8134c818023bec7592d1876dcc6caf2633`。先按路径字节序排列，再连接每行 `<blob><TAB><path><LF>` 取 SHA-256。独立评审 `v080-request-pricing-rereview-e6-a2` 由 Claude Code Opus 5.5 执行，实际推理档未观测。评审确认生产修复通过，并指出低级测试缺口。主控按检查点规则要求补足该用例。当前候选 Go usage 包、定向竞态、构建、vet 和文档检查均通过。

完成门禁：NOT_VERIFIED

该候选的 CEv1 查询返回 `missing_work_unit`。补足正向回归后复核最后候选。

## Round 3 — 2026-11-19

### 📋 逐调用计价正向回归复核

📊 总分 9.5/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。

- `CT-RP-R2-F1` 已关闭。移除 Codex 观测总量后，两个 TTL 桶仍与已准入的 20,000 个写入 token 对齐。用例经推导路径得到 `complete` 与 USD `1.856000000`，原因列表为空。仅在隔离覆盖中强制拒绝全部推导时，同一断言按预期失败。
- `CT-RP-R1-F1` 维持关闭。生产五文件与 Round 2 一致，真实 `Scan` 和 `PriceInvocations` 仍对冲突计数给出 `unavailable` 与 `conflicting_counts`，旧 v1 金额和会话汇总断言仍通过。

#### 🟡 建议改进

按原交接记录处理旧列零值的存在性、实际执行档和活动覆盖。原生 App Group 金额回放属于报告任务。

#### 🟢 已满足的部分

收敛候选覆盖逐调用价格选择、日期依据、快照读一致性、金额与资格边界、旧金额回放。当前轮只新增正向测试，沿用对应的生产验收证据。

#### 📝 总结

HEAD `57330f13f08276f25177b3bfe53731b4d001d1eb`，树 `2945ba1b5d05d17b2f66994214c9cb016bc4a65f`。六路径指纹为 `4d2fed6eed8af1deb6807e965c9f622c69cf4908380d0b6a8fdd400fbfd39f02`，先按路径字节序排列，再连接每行 `<blob><TAB><path><LF>` 取 SHA-256。Claude Code Opus 5.5 冷上下文只读复核 `v080-request-pricing-rereview-e6-a3`，实际推理档未观测。复核者运行匹配两个真实命名测试的定向验证，两项均通过。主控另复算全部六个 blob，验证 `TestRequestPricing` 和 `TestRequestFacts` 定向竞态、vet、格式与文档检查。复用 Round 2 的 usage 包回归和构建，以及覆盖原有范围的全仓回归。已知的实际采集缺口继续保留未知，金额和资格的完整分支使用隔离夹具。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:request-pricing`，ContentState `cost-transparency:request-pricing:state:4d2fed6eed8af1deb6807e965c9f622c69cf4908380d0b6a8fdd400fbfd39f02`。三项必要证据适用，缺项、失效与未决影响均为空。

#### 任务检查点

本地签名提交包含六个已评审的实现与测试文件、本评审记录和任务矩阵的任务 3 状态。完成本地提交并验证提交树后交给 `pricing-reporting`。报告读取沿用 `PriceInvocations` 的相同调用集合、旧金额、价卡摘要和知识截止。原生 App Group v1 的回放在报告候选上验收。
