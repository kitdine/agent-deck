---
status: active
topic: snapshot-performance
subject: performance-contract
---

## Round 1 — 2026-09-09

## 📋 基准契约实现评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SPC-R1-F1 — 高：测量与达标判定不覆盖约定的完整 helper 周期。OPEN**

位置：cmd/agentdeck/snapshot_performance_contract_test.go:303–346、:514–537。

- 行为风险：将未达标刷新判为达标；后续优化比较使用了不等价的工作量。
- 证据：只启动一个 Go test 子进程，子进程直接调用 refreshDesktopIndexes 和
  Service.Build，没有实际两次 CLI helper 启动。父进程计算 total，却仅将差额
  放入 ProcessStartupMS；WithinTarget 和汇总使用排除启动的子进程 WallMS。
  captureSnapshotPerformanceReference 还在计时和资源统计内查询逻辑表、排序及
  序列化对照数据，这些不是产品刷新工作；Build 也没有配置实际 CLI 的 Vault。
- 确定性判定反例：worker 900 ms、进程总计 1200 ms、CPU 100 ms、RSS 20 MiB，
  当前公式给出 within_target=true，需求规定的完整周期应失败。此为公式级反例，
  非真实耗时采样；源码调用链证明漏项和额外工作，不依赖重复大数据实验。
- 💡 修复：把真实命令初始化、两次 helper 调用及解码纳入明确的端到端 wall/CPU/RSS
  指标，达标与统计使用该指标；逻辑表核验移到测量之外，阶段诊断单独记录。
  若保留单进程内部微基准，应明确分类，不能用于完整周期达标或标作该基线。
  为“内部小于阈值、完整周期超限”及核验开销隔离增加有意义的保护。

**SPC-R1-F2 — 高：deadline 不覆盖快照，子进程失败会丢弃样本报告。OPEN**

位置：cmd/agentdeck/snapshot_performance_contract_test.go:243–300、:303–334、:514–530。

- 行为风险：声称有截止时间的测量可以继续挂起；异常样本和先前已完成样本不能
  写入最终报告，破坏“保留所有样本、区分失败”的基准契约。
- 证据：WithTimeout 的 ctx 只传给 refresh；成功后 Build 使用 context.Background。
  父进程 exec.Command 没有截止/回收边界。Build 错误或 partial 使用 t.Fatal，
  子进程非零退出后父进程再次 t.Fatalf，绕过位于全部循环之后的报告写入。
  完整样本出现 partial、崩溃或缺少结果行时均沿此分支终止，不生成失败样本。
- 💡 修复：deadline 贯穿完整测量，并对失去响应的子进程提供有界终止/回收。
  将 snapshot 错误、partial、超时、非零退出及缺失结果行转为结构化失败样本；
  即便最终测试失败，也应先可靠保存已收集样本及错误分类。增加隔离失败/超时
  验证，证明不会悬挂或丢报告，不靠延长超时解决。

### 🟡 改进建议 — 推荐

无额外开放建议。

### 🟢 优点

合成语料同时覆盖两客户端；已有完整 JSON golden、分块校验和逻辑表对照。
失败阶段未加入公开 JSON，现有过阈值样本也没有被报告为已达性能目标。

### 📝 总结

Reviewer: Codex；session 01a085b3-3e60-7ae2-9524-3a1316aec423。
Method: 单一主评审角色的差异和基准控制流审查、精确内容身份核对及公式级反例。
本会话未实现该任务；未委派代理。该评审不声称冷上下文独立性。
Scope: desktop.go/desktop_test.go 的基准支撑改动、新增 performance 测试、
平台 RSS helper、合成 golden/README 和任务交接；未审查后续优化实现。

Reviewed state: HEAD f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f；
scoped fingerprint 7b041af49f3dbd9632ccc8ad8b7cc7953db32da39f06e01f53c2263bbd8dc748。
按既有 ContentState recipe 重算并匹配：head 行加八个有序 git blob/path 行，
含 desktop.go、desktop_test.go、三个 snapshot_performance 测试文件、
testdata/snapshot-performance 的 README/golden、以及同步前 tasks.md。
测试主体 blob e57ec7c06469139619f1a06c13096bf41dca2ec2；本轮不改代码或测试。
Workspace: agent-deck.snapshot-performance / feature/snapshot-performance。
Task: ad-sp-performance-contract；逻辑 WorkUnit: snapshot-performance:performance-contract；
规范节点 ID: urn:ce:agent-deck:work-unit:snapshot-performance-performance-contract。

Evidence:

- 源码控制流和 900/1200 ms 公式反例已直接核对；发现确定性阻断后停止广泛验证。
  未重复 full Go、代表语料或性能测试；历史测试通过不能否定测量边界缺陷。
- 既有 IMPLEMENT gate VERIFIED 交接不是当前可复用 gate 结果：固定
  gate-status.cypher 对同一 target 查询返回 NOT_VERIFIED，四项历史 pass
  evidence 均 malformed。其 work_unit_id 投影为逻辑名，而 requires 的节点
  是上述 URN，违反 provider 的标识一致性契约。保留历史事实；修复后使用规范
  节点 ID、追加新证据并重查门禁，不覆盖旧观察，也不沿用未经核实的 VERIFIED。
- 当前反证绑定原始被评审内容状态；后续交接文字改变不把失败观察重标到新状态。

完成门禁：FAILED
新增反证节点 1 个、关系 2 条与提交数量一致，关系 preflight 全部 ok；固定
gate-status.cypher 回读确认反证 applicable=true、malformed=false、目标匹配，
measurement-contract 有失败证据，整体 FAILED。已有四项历史 pass 无法复用。
修复后的门禁应覆盖正确的测量/失败报告以及最终内容身份。

下一步指令：修复：snapshot-performance / reviews/performance-contract.md / SPC-R1-F1、SPC-R1-F2
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Round 2 — 2026-09-09

## 📋 基准契约修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。SPC-R1-F1、SPC-R1-F2 均 CLOSED，未发现新的阻断问题。

### 🟡 改进建议 — 推荐

无开放建议。

### 🟢 优点

- SPC-R1-F1 CLOSED：runSnapshotPerformanceSample 在父进程启动刷新前计时，
  分别启动 refresh-indexes/snapshot 子进程并调用 execute CLI 入口，包含正常
  参数处理和 Vault 构造；解码后确定 WallMS。CPU 为两 helper 之和，RSS 取其
  最大值；逻辑行查询移到此区间外并单列 VerificationMS。达标与汇总均使用
  完整 WallMS。1200 ms 周期反例和核验开销隔离已有针对性回归保护。
- SPC-R1-F2 CLOSED：一个 sample ctx 贯穿两次 CommandContext，超时由父进程
  终止并等待子进程退出。helper 非零、缺失/无法解析结果、snapshot 命令错误、
  分块解码失败和 partial 都返回结构化样本；代表测量先保存报告，再以 t.Errorf
  报告不完整样本。hang_snapshot 和 missing_snapshot 隔离测试覆盖有界返回及
  先前完整样本与失败样本共同持久化。业务初始化不再直接绕过 CLI 入口。

### 📝 总结

Reviewer: Codex；session 01a085b3-3e60-7ae2-9524-3a1316aec423。
Method: 单一主评审角色的逐项源码/控制流复核及精确状态证据复用；未参与修复，
未使用子代理，不宣称冷上下文独立性。Scope: Round 1 两项修复及相关回归保护。

Reviewed state: HEAD f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f；进入复评时按
Round 1 的八文件 recipe 重算得到修复交接指纹
739ec732c7e75c92a4b26a5d0804beedfa422908dd358136221946153c1f1fc7。
状态同步只修改 tasks.md 的 Task 1 Review 勾选及交接，产品、测试、golden 和
依赖未变；最终同 recipe 指纹为
d3a1dca4a04ca25bbf573b5093fed9dd6b9a23d8f6e0de9558393660a025e997。
Workspace: agent-deck.snapshot-performance / feature/snapshot-performance。
Task: ad-sp-performance-contract；WorkUnit 节点保持
urn:ce:agent-deck:work-unit:snapshot-performance-performance-contract。

Evidence:

- 已核对 sample、helper、worker、报告控制流及新增失败/阈值测试。
  固定 gate-status.cypher 对修复输入查询返回 VERIFIED；四项修复证据均
  applicable=true、malformed=false、target_matches=true，无缺失/失效/未决影响。
  旧 Round 1 反证保留在旧状态，对修复状态不适用；旧 malformed 证据未被重写。
- 复用 repair 指纹下的 complete-equivalence、reproducible-corpus、
  measurement-contract、l2-verification-clean 四项证据：包含 focused、
  affected packages、完整 vendored Go suite 以及格式检查。本轮不重复这些
  同状态测试，也不重复代表语料测量。最终状态仅交接改变，采用目标绑定 roll-up。
- 修复证据引用的 focused 日志 digest 为
  e8c0cf9107f6bae3dde7587a373c438217e55dc0a727f9ac9135534be95e028b；
  报告 digest 为 598057c029391a5b408019bde01c7577ce8d1854bc135d439260591f58d69343。
  此处为门禁认可的已有证据引用，不声称本轮重新执行或直接读取私有报告。
- 本次属于受控 CLI 入口等价测量：实际子进程是 Go test executable，内含
  固定时钟/源目录和输出包装；不宣称最终发布二进制、原生 Swift 解码器或
  打包安装验收完成。两类证据不能混同；后续集成/最终验收仍按任务计划执行。

完成门禁：VERIFIED
最终交接状态的固定 gate-status.cypher 查询通过：四项目标绑定 roll-up 有效，
缺失、失效和未决影响均为空；原观察状态未改。新增 5 个节点、12 条关系，
数量与提交一致；关系 preflight 全部 ok，门禁回读确认四项复用证据。
本 Task 完成的是基准契约，86.539 s 冷导入、7.691 s 未变化刷新仍是未达标基线，
不是性能优化完成；主题其余五项实现任务仍开放。

Task checkpoint：ad-sp-performance-contract；最终 content_state d3a1dca4a04ca25bbf573b5093fed9dd6b9a23d8f6e0de9558393660a025e997；门禁 VERIFIED。
提交建议：经单独授权提交本任务 CLI 支撑、测试、合成 fixture/README 和评审状态；此前文档任务尚未交付，暂存时明确其依赖和贡献边界，不夹带后续任务。
推送建议：经单独授权、提交对象/签名及远端核验后推送候选 origin/feature/snapshot-performance；本轮未执行。

下一步指令：开发：snapshot-performance / shared-ingestion
WORKFLOW_WORKSPACE: agent-deck.snapshot-performance

## Replanning disposition — 2026-09-10

The user requested a complete topic replan and permitted topic-code replacement.
See [whole-topic review and replacement plan](tasks.md#round-2--2026-09-10).
The rounds above remain historical facts for their own content. They do not
approve the revised document set, authorize delivery, or prescribe the current
next command. The current Tasks matrix is the only execution plan.
