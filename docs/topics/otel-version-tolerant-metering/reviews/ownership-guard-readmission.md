---
status: active
topic: otel-version-tolerant-metering
subject: ownership-guard-readmission
---

# 线程排他与历史补计评审

## Round 1 — 2026-11-15

## 📋 独立实现评审

📊 总分：8/10

✅ Verdict: FAIL

### 🔴 必须修复

OVM-T2-R1-F1 — P2 — OPEN。internal/telemetry/reconcile.go 在 PrepareSources 已提交
撤销后，若 Store.SettleTelemetry 返回取消或其他非 identity-pending 错误，会在合并
preparedChanged 与保存 projectionPending 之前返回。后续准备/结算都无新变化，跳过
SessionsChanged，旧 OTel 投影一直保留。projection.go 同一路径错误返回 false，丢失
已提交 changed 事实。

独立Reviewer在隔离数据库建立一个owned会话并投影，准备回调提交撤销后取消context，
再以有效context重试，得到 stale projections=1、callbacks=0、Changed=false。
主调度以 TestCommittedPreparationCancellationRecoversProjection 复现相同失败，
另外验证 SettleAndProject 已提交变化的取消路径错误报告 changed=false。

最小修复为在后续错误返回前保存待投影状态、保留changed，成功投影后再清除；
追加同reconciler重试及无内存状态的scan/rebuild投影入口恢复回归。当前修复候选
已使定向回归通过，等待完整验证与独立复评，发现保持OPEN。

### 🟡 改进建议

无额外偏好项。

### 🟢 已确认优点

迁移、来源补填和原子排他有相应测试与既有验证，保留Record/Key和历史时间。
复评仍需继续错误恢复及持久输入边界核查。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；38文件SHA256
 e0640d8ca2130db23bee44f97495e874a6c3c4a2a8c65b9f39b7036183331ce7。
配方为head=<HEAD>换行，再按路径排序path=blob换行；包括保留的Task1代码，不含主题文档。

| 路径 | blob |
| --- | --- |
| apps/macos/AgentDeckShared/DesktopWire.swift | 895ec52a490af1e800536eccf36a9d0aa18ef8f8 |
| apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift | 7104552ad32eb84ff050bcf5fa149d668b86e30d |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | 46c3563666a8fff4601b547ef47c43bd59351bcb |
| apps/macos/AgentDeckVerification/main.swift | e5e8f62e2e56a82c145123f0e9b1e591707da753 |
| cmd/agentdeck/main.go | 6dac45f12be3b1b3bff7ae72ef8075f7e65b5427 |
| cmd/agentdeck/main_test.go | b0f5a1a4ec39f1366a0e818958333812f5fab396 |
| cmd/agentdeck/telemetry.go | 554496d48bdc8840af40a8334d8e00b5414b0408 |
| cmd/agentdeck/telemetry_test.go | 60eefe93a7c9950e009a9dcc7860c6e564bf5a7b |
| cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json | e501f3393038b9213234d670986e370e0d9fe6e1 |
| desktop/fixtures/v1/snapshot-complete.json | 80d418efbb868e8a4defaa5c1186b2af6b2b02f7 |
| desktop/fixtures/v1/snapshot-empty-client.json | b59212d7ba5dc5c168fe401d700e0388598a5b09 |
| desktop/fixtures/v1/snapshot-schema-ahead.json | d40570393d5c74c91edd1584ed80cbfa5a0db207 |
| internal/desktop/serving_schema.go | 6e97b11a245224d64cd404bb9840ac76415f30d0 |
| internal/desktop/serving_schema_test.go | 5fab83e263dd5a16e4279c7b958dcbb0dddee8e8 |
| internal/scanruntime/scanruntime.go | 53e3ae4b5969d8d5686c818a74d20e4686820ea8 |
| internal/scanruntime/serving.go | 0d9ffe06c3f07b19125e2a2401446da3fbe5c625 |
| internal/scanruntime/source_identity_test.go | 95eb39bc9c502594f271472b2cb34d70fd003cb0 |
| internal/sessionorigin/reconcile_test.go | 53c952f2c851112b08cd2f8df5e820d4c794c443 |
| internal/store/migrations.go | 2e71d493bcbdc00236993df6ec98ee7ad928ce9e |
| internal/store/session_origin.go | 7f5d5ced627de7b976c6c012f9a48019f67581cc |
| internal/store/session_origin_test.go | 8b12e016502d712933287112ce38f40c65ce4ae2 |
| internal/store/store.go | 9a6c838ee757ba691e5015055205a5dccd1b0f22 |
| internal/store/telemetry.go | f5603f461ba1cce7cff5e72e7ac3c5dfd5a54c06 |
| internal/store/telemetry_admission.go | 7304327b5cd5482bcf7878c4d66155f042e71137 |
| internal/store/telemetry_admission_test.go | 8f884f2c4bdfc6d99ced1c3304bec394c9410c8b |
| internal/store/telemetry_ownership.go | caa0acd0745ac916f53bae41a72866ab5f193af7 |
| internal/store/telemetry_ownership_test.go | 0202dba5aa4c2c8bb0730f2904d2442b7bc82614 |
| internal/store/usage_partitions_test.go | 8c9fcfe63b27dd8a95a2fdc483fe910b025f7c30 |
| internal/telemetry/admission_test.go | 954adf1bc581f28c4100d227a359916ebb44a801 |
| internal/telemetry/ownership_test.go | 6880d8c92f0908b2ec6fa454ace95680db97f8b9 |
| internal/telemetry/projection.go | 938f70f7153b235ee7028621138de53c7633981f |
| internal/telemetry/receiver.go | 30feb1a25331ac7387c7fc35f4b47de418b1058c |
| internal/telemetry/reconcile.go | d8bfe65d9decb3abc3e23b0f0cad00f2287dd446 |
| internal/telemetry/source_identity_test.go | fb82ddab53f5b0a1ca781c7bfb47155218b4131b |
| internal/usage/source_identity.go | f6aee082bcb28280991364dfaf3aff37b404c8b1 |
| internal/usage/source_identity_test.go | 4422c3cb55d33553f34acd2e775747a55b1736e5 |
| internal/usage/source_pipeline.go | c7ec5eae7b0c3f7c3d7f893439ce722b8cc9a605 |
| internal/usage/usage.go | fd929b88c027cc19a8e9528f31562edf488b69df |

Reviewer: 冷上下文Codex/gpt-6-astra medium，主调度直接核对源码和失败回归并采纳。
Method: 只读调用链检查及隔离最小复现。临时复现清理后候选指纹一致。
Scope: schema35、身份补填/readmission、原子撤销、前缀保护、错误传播、Go/Swift消费者。
Evidence: 复用最终Go29包1663顶层PASS、定向race4包19PASS、vet/双架构/size通过。
原生主套件163+237（3初始skip）+59，两个restart阶段另行各1通过，零失败。
原生schema opt-in矩阵未执行；该项不是当前Task2合同的必需独立门槛，不能宣称其验收完成。
新增最小复现失败且候选不变，主调度补充的失败先行回归覆盖已提交撤销后的取消路径。
Limitation: 决定性发现后停止扩大验证，剩余错误恢复组合、持久输入变体和调用入口待复评。
Task3分类/文案/真实producer验收仍待完成。
Completion gate: NOT_VERIFIED

修复范围为OVM-T2-R1-F1，Task2修复周期1/3。没有提交或部署。

### 修复候选验证

主调度在准备变化合并后、任何结算错误返回前保存projectionPending，并使
SettleAndProject错误路径返回实际Changed。仅改reconcile.go、projection.go与
source_identity_test.go。新回归先在旧实现复现changed=false、callbacks=0、stale=1，
修复后通过；无内存状态的scan/rebuild入口也能清理持久旧投影且不恢复贡献。
修复候选38文件SHA256为257399af408e2c852602040bed9c01a0b3c662c09544bb0c9f801a3d7ece09f9。
最终Go全套、对应定向race、vet、双架构build和size检查通过；native/schema源未变，
复用Round1对应证据。等待独立复评，F1尚未由本段自判关闭。

## Round 2 — 2026-11-15

## 📋 投影恢复独立复评

📊 总分：8/10

✅ Verdict: FAIL

### 🔴 必须修复

OVM-T2-R1-F1 -> CLOSED。结算错误前保留已提交changed及projectionPending，
SettleAndProject返回真实changed；同实例重试与无内存投影入口回归证明旧投影可清理。

OVM-T2-R2-F1 — P2 — OPEN。reconcile.go在准备已提交撤销、后续普通prepareErr存在而
结算/投影成功时清除projectionPending后返回prepareErr。receiver.go仅在成功或
identity-pending错误时通知Published。本次普通错误跳过通知，后续成功重试又无变化，
因此App可持续显示旧金额直到其他独立刷新。

独立Reviewer以Go overlay和真实Serve/core/index复现 prepares=5、contributions=0、
projections=0、publications=0。主调度直接核对相同控制流，并新增真实Serve回归，
普通准备错误及准备错误叠加投影失败两种场景在旧实现分别重试493/496次仍无通知。
修复保留普通准备错误下尚未通知的变化，后续成功时报告Changed；成功或
identity-pending通知路径保持原语义。定向回归已通过，完整验证与复评待完成。

### 🟡 改进建议

无额外偏好项。

### 🟢 已确认优点

上轮投影恢复缺陷已关闭；迁移、线程身份、补填与前缀原子保护已有覆盖。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；38文件SHA256
257399af408e2c852602040bed9c01a0b3c662c09544bb0c9f801a3d7ece09f9。
相对Round1仅projection.go=7f48fe678fe0b98deffb582d1221bac52baecbb3、
reconcile.go=9b181a5a99ded51124ff481a9cdaeb07dda7df2a、
source_identity_test.go=417c7c9504ebd9ee46e331296acef4bfbd3a4f52变化。
Reviewer: 冷上下文Codex/gpt-6-astra medium；主调度独立复现并记录。
Method: 只读合同/源码/调用链核查和隔离故障注入，无仓库改动。
Scope: 迁移CHECK/triggers/恢复、来源metadata、64/65批次、NULL、normal/cold/prefix、
readiness、SQL-Go字节、历史日期金额及receiver/import/scan/rebuild/watch接线。
Evidence: 复用29包1664顶层PASS、定向race4项、vet/双架构/size，native按Round1复用。
新增overlay反例失败，清理后指纹一致。主调度失败先行回归和修复后定向验证同样确认该缺陷。
Limitation: 新阻断后停止扩大验证，普通错误/投影失败/发布恢复组合尚需复评；
线程集合独立测量未被本轮重新确认。native opt-in矩阵仍未执行，非本任务额外必需门槛。
Completion gate: NOT_VERIFIED

Task2修复周期2/3，待OVM-T2-R2-F1关闭。Task3仍待实施，无提交或部署。

### Round2 修复验证

修复候选38文件SHA256为577ec61da4f498864aa43c6d732aab0a2eb2b69077f111d9224c689be5037596。
相对本轮被审候选仅reconcile.go与source_identity_test.go变化，前者保留普通prepareErr
抑制通知时的已提交Changed，后者覆盖实际Serve的普通准备错误及投影失败组合。
两种反例先失败，修复后通过；最终Go全套、相关race、vet、双架构build/size与diff检查通过。
原生/迁移/source准备代码未变，复用对应既有证据。F1等待独立复评确认关闭。

## Round 3 — 2026-11-15

## 📋 发布恢复闭环复评

📊 总分：10/10

✅ Verdict: PASS

### 🔴 必须修复

无新增发现。OVM-T2-R1-F1 保持 CLOSED，错误返回保留Changed与待投影状态。
OVM-T2-R2-F1 -> CLOSED，普通准备错误抑制通知时保留待发布变化，恢复成功后仍通知Published。
真实Serve红绿回归覆盖单独准备错误与叠加首次投影失败，恢复前不提前发布，恢复后
贡献和旧投影均为零。identity-pending及取消/结算错误的组合经控制流与已有回归核查。

### 🟡 改进建议

无可操作缺陷。

### 🟢 已确认优点

迁移、有限批次身份准备、原子撤销及历史补计与恢复链路具备证据。
失败项保留NULL而后续批次可推进，未准备完成不晋升或恢复重复贡献。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；38文件SHA256
577ec61da4f498864aa43c6d732aab0a2eb2b69077f111d9224c689be5037596。
配方与Round1一致。相对Round2候选仅reconcile.go=41f8a5be3f7a5a626ca2eceedeb28fc81f62308a、
source_identity_test.go=e4b2b0213defe711a640520481ea0e31be255055变化；其他blob沿用Round1及Round2差量。
Reviewer: 冷上下文Codex/gpt-6-astra medium；主调度复现和修复两轮问题并记录最终裁决。
Method: 只读合同、调用链和回归断言核查，核对38文件指纹。
Scope: Task2完整声明范围，重点普通错误/投影失败/取消/identity-pending/发布重试组合，
以及import、scan/serving partial、watch接线。无新增阻断导致的未审范围。
Evidence: 直接核对Go29包1665顶层PASS、定向race5项、红绿Serve反例及vet/双架构/size通过。
未变store/usage/scanruntime race、迁移只读恢复与原生消费者证据按Round1/2复用。
Environment: go1.27.3 darwin/amd64；原生Xcode26.4，隔离fixture。
Limitation: 原生schema opt-in矩阵未执行；集合构建测量不代表整个settle上界。
Task3自动分类、文案和真实producer验收仍待完成。本PASS不代表整个主题可交付。
Completion gate: VERIFIED

Task checkpoint

Task2评审通过，修复周期保持2/3。CEv1 WorkUnit otel-version-tolerant-metering:ownership-guard-readmission 在 ContentState otel-version-tolerant-metering:ownership-guard-readmission:state:577ec61da4f49886 的7项必需证据VERIFIED，无缺失/失效/未解决影响。提交建议等待完整交付边界及提交授权；
推送建议等待已验证提交和单独授权。下一任务为builtin-rule-set。
