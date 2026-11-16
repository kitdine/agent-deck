---
status: historical
topic: otel-version-tolerant-metering
subject: builtin-rule-set
retired: 2026-11-16
---

# 内置自动规则评审

## Round 1 — 2026-11-15

## 📋 Fable 独立实现评审

📊 总分：8.4/10（独立报告84/100）

✅ Verdict: FAIL

### 🔴 必须修复

- OVM-T3-R1-F1 — P2 — OPEN。scanruntime 的派生发布条件额外要求 session completed，
  索引或分类失败阻止已成功 usage 发布。主调度复现 usage completed/session failed 时
  publication=0，修复恢复 usage 独立发布且保留成功会话先分类的顺序。强化回归通过真实
  serving publisher 确认可用用量快照，等待独立复评。
- OVM-T3-R1-F2 — P3 — OPEN。normal/bootstrap 扫描身份 pending 后直接返回，不归约
  可读会话。主调度隔离反例保留未评估 Codex 来源并增加可读 Claude 会话，旧路径无
  session_origins 行。共用 settleAndClassify 允许 pending 时归约，再保留 pending 错误，
  回归得到 work/default_work，等待独立复评。
- OVM-T3-R1-F3 — P3 — OPEN。DesktopCopy 的 source unavailable 覆盖提示仍称 Unknown。
  修复为中英文“会话保留在工作中”，同步本地化资源。原生验证进行中。
- OVM-T3-R1-F4 — P3 — OPEN。Codex新计数字段掩盖0/0时的receiver关闭/未知/无记录提示，
  adapter失败文案在行内和footer重复。保留零记录状态，exec-only准确显示实际计数，
  移除重复footer，并补充三种渲染状态及行映射断言，等待原生验证与复评。
- OVM-T3-R1-F5 — P3 — OPEN。CLI来源状态只显示background_sessions，exec-only看似无记录。
  主调度失败先行断言复现，修复同时输出推断辅助数与OTel所有会话数，定向回归通过。
- OVM-T3-R1-F6 — P3 — OPEN。clear后索引存在但无法打开时静默跳过刷新。主调度以缺表
  索引复现已saved回执却partial=false且无warning，修复区分不存在与其他打开错误，
  后者返回classification_refresh_pending并保留已提交clear，定向回归通过。

### 🟡 改进建议

无独立于上述必须关闭事项的偏好项。P3不豁免修复。

### 🟢 已确认优点

规则优先级、fallback与证据分离、版本无关家族、override和旧规则重算、schema34只读、
coverage诊断、receiver重试及新transport计数验证符合合同。真实0.169 happy-path验收覆盖
普通/ephemeral exec与TUI标题，分类和目录价格符合有界推断策略。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692。
代码61路径SHA256为7785462db1bfb7652be68126d8ee1507aff8011039d1e8f5bd8a30c59a40a29f，
加4份合同的65路径SHA256为63c2eaae38974b24d542b96f09ec21a70cc68312ed2c666739de22573662d8f9。
配方为head=<HEAD>换行与按路径排序的path=blob换行。

| 路径 | blob |
| --- | --- |
| apps/macos/AgentDeckApp/DesktopCopy.swift | 80c6c6061cfb90500c3d03b3bc5a72f1fb3594c5 |
| apps/macos/AgentDeckApp/Localizable.xcstrings | 96476622a0d2472e11122b82f1e0fcd72bc54e9f |
| apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | 58e457d9107bc35e1cff409488d0584b615b281e |
| apps/macos/AgentDeckApp/SessionScopeController.swift | f9bef935aeb266da79db3014d7e62060e746b832 |
| apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | 7b9d0842b108dacc723c55525892e6a3ab67a4f7 |
| apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | ad41f874cbc7010c66425415db6463b163e24c85 |
| apps/macos/AgentDeckShared/DesktopWire.swift | 895ec52a490af1e800536eccf36a9d0aa18ef8f8 |
| apps/macos/AgentDeckShared/SessionOriginTransport.swift | 06ac2629214d0c4afe3ebe6371c790bd237237ca |
| apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift | 7104552ad32eb84ff050bcf5fa149d668b86e30d |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | 46c3563666a8fff4601b547ef47c43bd59351bcb |
| apps/macos/AgentDeckTests/SessionOriginTransportTests.swift | f71f509623ec375e9de84e8850f9e08517b8acac |
| apps/macos/AgentDeckVerification/main.swift | e5e8f62e2e56a82c145123f0e9b1e591707da753 |
| cmd/agentdeck/main.go | 950a931f589e7631dd89cbb6ef6e54943294b4cd |
| cmd/agentdeck/main_test.go | b0f5a1a4ec39f1366a0e818958333812f5fab396 |
| cmd/agentdeck/session_origin.go | 7ba44726b03cb8eeaad01149475d267b186f93fb |
| cmd/agentdeck/session_origin_acceptance_test.go | e8e6935196425a53eade303e7ebd335b0b8b89d0 |
| cmd/agentdeck/session_origin_repair_test.go | 05c572cc9847b7a2de2c9bd8b55d40169b31c962 |
| cmd/agentdeck/session_origin_test.go | 62415e67cc9f476190d087715c95197443b153fb |
| cmd/agentdeck/telemetry.go | 0273c1fc7a8b5ce4312e7f4c305730642480b657 |
| cmd/agentdeck/telemetry_test.go | 4fb1a1405a98f81e013714753c048bff419013ae |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | c3545925ff939bd303058709aa8d2d04f0334065 |
| cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json | 37098c0db956c889077ae0052d6f87a7aa8699be |
| desktop/fixtures/v1/session-scope-batch.json | 155827a1b0b52587e102f5ab2b90d42169376292 |
| desktop/fixtures/v1/snapshot-complete.json | 1a4aa7173a79fae01316300563893a7e7a164470 |
| desktop/fixtures/v1/snapshot-empty-client.json | 0a8ea28293e47e71202b442a103ffd79a3fe19c4 |
| desktop/fixtures/v1/snapshot-schema-ahead.json | d40570393d5c74c91edd1584ed80cbfa5a0db207 |
| docs/specs/cli-design.md | 3eb7223aeea81ba7b21062818d1c5c8ced85b7fe |
| docs/topics/auxiliary-session-classification/architecture.md | 2616ca05438ac4624d1106eb438bac0c4f05d278 |
| docs/topics/auxiliary-session-classification/requirements.md | 1b4fc3ad1b8635b2f20f564bc2c7b425a6160e38 |
| docs/topics/scan-performance/architecture.md | 6b305630e64a70a4456157ab3f12c3f435f3e491 |
| internal/desktop/serving_schema.go | 6e97b11a245224d64cd404bb9840ac76415f30d0 |
| internal/desktop/serving_schema_test.go | 5fab83e263dd5a16e4279c7b958dcbb0dddee8e8 |
| internal/scanruntime/scanruntime.go | 95d44cdf0a9f94f5becd57e57430b6534f20e13a |
| internal/scanruntime/serving.go | 936c133e46c05c1d030d69668d2f9187645c3c1a |
| internal/scanruntime/source_identity_test.go | 95eb39bc9c502594f271472b2cb34d70fd003cb0 |
| internal/sessionorigin/codexotel.go | ebfa096a86a616d05ea5734fdabdbb0ffe2fe418 |
| internal/sessionorigin/reconcile.go | e7372727d0cfee4fcc0085972812d47b7d9beb62 |
| internal/sessionorigin/reconcile_test.go | ec456c2a5a0d287404ca055a5c0578f655399f9d |
| internal/sessionorigin/reducer.go | cb2c8bbcbd354deeadc214c3591e773859a635db |
| internal/sessionorigin/reducer_test.go | c34b35bfb0518f16c77079edc8dc03826d4124b1 |
| internal/sessionorigin/selection.go | 8dfb2711b41385665eb9829e949da0b5a5b58a7b |
| internal/sessionorigin/selection_test.go | fb53d1ec206c4bf2a6b435786f3f4a81e6b69502 |
| internal/sessionorigin/sessionorigin.go | 38ff37b5af798d6519e335dc465ec2894234eb27 |
| internal/sessionorigin/sessionorigin_test.go | 2910d1bf200ba9c82b8d7fdbfbcd6f52f01f48d3 |
| internal/store/migrations.go | 2e71d493bcbdc00236993df6ec98ee7ad928ce9e |
| internal/store/session_origin.go | 7f5d5ced627de7b976c6c012f9a48019f67581cc |
| internal/store/session_origin_derive.go | 5adf5f2a05e56ba4b0b4f0ae88d94078b63645bb |
| internal/store/session_origin_test.go | 8b12e016502d712933287112ce38f40c65ce4ae2 |
| internal/store/store.go | 9a6c838ee757ba691e5015055205a5dccd1b0f22 |
| internal/store/telemetry.go | c2664ae1429a2a9dbca85cc392f24ebbb0ba4e38 |
| internal/store/telemetry_admission.go | 7304327b5cd5482bcf7878c4d66155f042e71137 |
| internal/store/telemetry_admission_test.go | 8f884f2c4bdfc6d99ced1c3304bec394c9410c8b |
| internal/store/telemetry_ownership.go | caa0acd0745ac916f53bae41a72866ab5f193af7 |
| internal/store/telemetry_ownership_test.go | 0202dba5aa4c2c8bb0730f2904d2442b7bc82614 |
| internal/store/usage_partitions_test.go | 8c9fcfe63b27dd8a95a2fdc483fe910b025f7c30 |
| internal/telemetry/admission_test.go | 954adf1bc581f28c4100d227a359916ebb44a801 |
| internal/telemetry/ownership_test.go | 6880d8c92f0908b2ec6fa454ace95680db97f8b9 |
| internal/telemetry/projection.go | 7f48fe678fe0b98deffb582d1221bac52baecbb3 |
| internal/telemetry/receiver.go | b2d8e6d68503ad05d1f13f2fd9defe25b642daf5 |
| internal/telemetry/reconcile.go | dd9c40d0b99f0d556209837283076ef8e07e5328 |
| internal/telemetry/source_identity_test.go | 135f368f928ded864139070ee08061271f98ffc0 |
| internal/usage/source_identity.go | f6aee082bcb28280991364dfaf3aff37b404c8b1 |
| internal/usage/source_identity_test.go | 4422c3cb55d33553f34acd2e775747a55b1736e5 |
| internal/usage/source_pipeline.go | c7ec5eae7b0c3f7c3d7f893439ce722b8cc9a605 |
| internal/usage/usage.go | fd929b88c027cc19a8e9528f31562edf488b69df |

Reviewer: 独立冷上下文Claude/Fable5.2 medium，主调度直接核对六处代码并复现四条Go反例。
Method: 静态合同/源码审读，65路径blob核验，无新增Reviewer测试。主调度补做失败先行回归。
Scope: 规则、归约、迁移只读兼容、来源计数/transport、回调/重试、clear、CLI/native文案及同步合同。
Evidence: Reviewer复用handoff中的Go1673、race147+5+5、构建和native/渲染及真实验收摘要。
主调度另直接核对日志、61文件指纹、真实CLI readback与中英文窄窗口渲染。
Limitation: F1后停止扩大验证，测试断言强度、全部14图/金标、marker与JSONL副本并存、
receiver关停ctx等剩余边界需复评。原生schema opt-in矩阵未执行，非额外必需门槛。
Completion gate: NOT_VERIFIED

修复范围为F1–F6，Task3修复周期1/3。当前等待最终验证及独立复评。

### R1 修复验证

主调度已复现F1/F2/F5/F6并修复，定向Go回归通过。F1进一步经真实serving publisher验证
usage可用；F2共用normal/bootstrap路径在pending时先分类再返回错误；F6保留saved回执。
F3同步中英文Work提示；F4保留0/0的receiver状态、正数owned显示真实计数并移除重复footer。
原生行状态断言和新增capture-off/unknown/exec-only渲染场景纳入验证。

完整Go检查在最终生产代码上得到1675项PASS、1条旧CLI文案断言FAIL。仅更新该断言后，
TestSessionOriginSourceStatusStatesAndPrivacy定向PASS，其余结果按不变内容复用。
相关race、vet、双架构build/size和原生完整套件通过。最后的native小范围文案调整另做
行状态与双语渲染定向检查，结果随复评收口。未将早先失败命令表述为全绿运行。
最终amd64 CLI SHA256为6f24fb2d860aac06ad6f54d020d652f4738caa36f1501dc986a5e7b070dc2a72，
在既有真实producer隔离状态回读exec=work且background/owned0/0、ephemeral=work且0/1、
TUI为work+auxiliary且1/1。原producer输入与记录继续复用，不重复模型调用。
主调度首次回读误用空state目录，已改为harness实际agentdeck-state目录并逐项断言通过。

原生定向2项测试PASS，20张中英文460pt渲染重新生成。主调度查看exec-only英文和增强不可读中文画面，计数无No records矛盾，重复footer已移除，未见明显裁切。

## Round 2 — 2026-11-15

## 📋 Fable 闭环复评

📊 总分：8.8/10

✅ Verdict: FAIL

### 🔴 必须修复

OVM-T3-R1-F1、F2、F3、F4、F5、F6 -> CLOSED。用量独立发布、scanruntime pending分类、
Work文案、零计数提示/重复footer、CLI两个计数和clear打开失败告警均已修复并有对应验证。

OVM-T3-R2-F1 — P3 — OPEN。watch session Scan闭包在SettleAndProject身份pending时提前
返回，跳过Reconcile；watch.Poll/Run继而退出，与“下次变化重试”的注释不符。
Reviewer隔离HOME/state保留缺失Codex来源事实并加入真实Claude JSONL，运行session-only
watch得到scan session错误、无事件、已索引1会话而派生分类0。主调度独立复现同一入口。
修复候选先分类、以scan_pending事件明确未完成、保留未确认检查点，后续poll自动重试，
其他域继续；成功后完成并确认检查点，普通错误继续传播。

OVM-T3-R2-F2 — P3 — OPEN（主调度补充发现）。旧watch检查点仅含索引epoch和来源指纹，
未变化来源可使规则升级后的首次分类完全跳过。隔离回归存入旧v1检查点，watch两秒无事件、
未触发默认分类。修复将活动reducer/rule版本纳入共用检查点编码，scanruntime/watch一致，
旧检查点自动触发刷新。失败先行和修复后定向回归已完成，等待独立复评关闭。

### 🟡 改进建议

无额外必须项。缺失索引clear例外主要经代码确认，原有核心clear测试继续保留。

### 🟢 已确认优点

原六项全部关闭。receiver取消/重启、import、规则归约、金标变动和四张代表性渲染经复核。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692。
代码62路径95cd381dd2a7c8592c480ee35e619bcabb68caf1b4553d4693f6b6ef70fdde46；
含五合同67路径7a601dfb2b6911029b88f7cc6d60061fc668501a6063fb449e697fd8635457e9。
Reviewer: 冷上下文Fable5.2 medium；主调度核查/复现并记录。
Method: blob核对、静态路径及隔离watch复现，临时测试清理后指纹一致。
Scope: R1关闭与watch遗漏入口。主调度补充规则升级检查点反例。
Evidence: Reviewer直接核对1675PASS+旧断言FAIL、该项定向PASS、race10PASS及四张渲染，
未将组合覆盖描述为一次全绿命令。主调度新增watch pending/初始规则刷新及轮询恢复回归。
Limitation: selection/reconcile断言强度、marker与JSONL副本并存、全部合同同步和真实回读
仍需后续完整审查；原生schema可选矩阵未执行。旧未引用Unknown文案键未被列为缺陷。
Completion gate: NOT_VERIFIED

Task3修复周期2/3，当前验证与复评待完成。无提交或部署。

### R2 修复验证

三个反例先失败：Poll身份pending返回错误；真实session-only watch报错；旧v1检查点
使规则刷新无事件。修复后pending事件、未确认检查点重试、其他域继续、CLI分类和规则升级
回归均通过。文本pending明确自动重试，不冒充completed。共用规则检查点编码用于watch及
scanruntime，旧bootstrap哨兵保持兼容。补齐相应旧格式测试导入/断言后定向检查通过。
最终完整Go、相关race、vet、双架构/size和原生最终构建通过，Swift/渲染源未变按R1复用。
代码66路径指纹2d4b371e7ad7074212f0904c9c48e2993b3e9537821a7fc701b72aca505d6d70，
含六合同72路径指纹7660d2bd6f871bd1af273a0579b82ba8fc2f46ac7832ca57d0207cf54844f153。
等待独立复评关闭F1/F2。

## Round 3 — 2026-11-15

## 📋 Fable 最终独立复评

📊 总分：9.3/10

✅ Verdict: PASS

### 🔴 必须修复

无新增发现。OVM-T3-R1-F1至F6保持CLOSED。
OVM-T3-R2-F1 -> CLOSED。watch身份pending先分类，再发出非致命scan_pending；
不确认指纹、其他域继续、下一轮自动重试。独立CLI复现同进程两次pending后恢复completed，
检查点为v2，重启无冗余事件；三域场景usage清理遗失来源后全部完成且会话为work。
OVM-T3-R2-F2 -> CLOSED。共用WatchFingerprint包含epoch、来源、reducer和有序ruleIDs，
全部成功写入点统一，旧检查点触发刷新，bootstrap哨兵兼容，仓库无其他格式解析者。

### 🟡 改进建议

无可操作缺陷。旧用例名称和未引用文案键不影响本候选行为。

### 🟢 已确认优点

规则/归约断言核对类别、原因、规则ID，覆盖旧证据退役、override、schema34只读、
覆盖缺口、fallback非证据和优先级。真实0.169读回确认主/ephemeral exec为work，TUI主
为work而标题为auxiliary。普通会话自动归类，配置失败保留诊断，避免手工逐会话设置。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692。
代码66路径2d4b371e7ad7074212f0904c9c48e2993b3e9537821a7fc701b72aca505d6d70，
含六合同72路径7660d2bd6f871bd1af273a0579b82ba8fc2f46ac7832ca57d0207cf54844f153。
配方为head=<HEAD>换行与排序path=blob换行。评审开始/结束及临时复现清理后一致。

| 路径 | blob |
| --- | --- |
| apps/macos/AgentDeckApp/DesktopCopy.swift | c9a507ed76ba08be91e054efafac4e777441b92d |
| apps/macos/AgentDeckApp/Localizable.xcstrings | 503e121d15eda01a331b99d32da21c7cd8f401f5 |
| apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | ded7c3f2b511a62e2600a16d111eee9c8829314d |
| apps/macos/AgentDeckApp/SessionScopeController.swift | f9bef935aeb266da79db3014d7e62060e746b832 |
| apps/macos/AgentDeckApp/SettingsWindowView.swift | 3376511752b7104f2aaf83a44f2800f6f7470251 |
| apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | f226389d99ff4ab02b0e17809353d9f3e9a44ee9 |
| apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | ad41f874cbc7010c66425415db6463b163e24c85 |
| apps/macos/AgentDeckShared/DesktopWire.swift | 895ec52a490af1e800536eccf36a9d0aa18ef8f8 |
| apps/macos/AgentDeckShared/SessionOriginTransport.swift | 06ac2629214d0c4afe3ebe6371c790bd237237ca |
| apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift | 7104552ad32eb84ff050bcf5fa149d668b86e30d |
| apps/macos/AgentDeckTests/DesktopWireTests.swift | 46c3563666a8fff4601b547ef47c43bd59351bcb |
| apps/macos/AgentDeckTests/SessionOriginTransportTests.swift | f71f509623ec375e9de84e8850f9e08517b8acac |
| apps/macos/AgentDeckVerification/main.swift | e5e8f62e2e56a82c145123f0e9b1e591707da753 |
| cmd/agentdeck/main.go | e88bf1a268d5d249f69ea0977526c38ae65288cc |
| cmd/agentdeck/main_test.go | 89b697424cf452dc38de18fdbd1ecad1b0e19e6b |
| cmd/agentdeck/session_origin.go | bba47a59778ee50126774140b4eec8b47334ec08 |
| cmd/agentdeck/session_origin_acceptance_test.go | e8e6935196425a53eade303e7ebd335b0b8b89d0 |
| cmd/agentdeck/session_origin_repair_test.go | 05c572cc9847b7a2de2c9bd8b55d40169b31c962 |
| cmd/agentdeck/session_origin_test.go | af0c1dd293a93c096493d8279fc5fdc158113c61 |
| cmd/agentdeck/telemetry.go | 0273c1fc7a8b5ce4312e7f4c305730642480b657 |
| cmd/agentdeck/telemetry_test.go | 4fb1a1405a98f81e013714753c048bff419013ae |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | c3545925ff939bd303058709aa8d2d04f0334065 |
| cmd/agentdeck/testdata/snapshot-performance/synthetic-snapshot.json | 37098c0db956c889077ae0052d6f87a7aa8699be |
| cmd/agentdeck/watch_origin_test.go | ef5efc1e946faeb8cb1b000b8f9faa4e21bc4db5 |
| desktop/fixtures/v1/session-scope-batch.json | 155827a1b0b52587e102f5ab2b90d42169376292 |
| desktop/fixtures/v1/snapshot-complete.json | 1a4aa7173a79fae01316300563893a7e7a164470 |
| desktop/fixtures/v1/snapshot-empty-client.json | 0a8ea28293e47e71202b442a103ffd79a3fe19c4 |
| desktop/fixtures/v1/snapshot-schema-ahead.json | d40570393d5c74c91edd1584ed80cbfa5a0db207 |
| docs/specs/cli-design.md | dfd770d33fde138eea5c30670c6f4ff6b3fde9df |
| docs/topics/auxiliary-session-classification/architecture.md | 2616ca05438ac4624d1106eb438bac0c4f05d278 |
| docs/topics/auxiliary-session-classification/requirements.md | 1b4fc3ad1b8635b2f20f564bc2c7b425a6160e38 |
| docs/topics/otel-version-tolerant-metering/architecture.md | 70184c9942c3265ed9b02e4b6531dbbd0c5f0762 |
| docs/topics/otel-version-tolerant-metering/tasks.md | 461522b2ca1d686f2c0fdd3abba5e5f428ab1712 |
| docs/topics/scan-performance/architecture.md | 6b305630e64a70a4456157ab3f12c3f435f3e491 |
| internal/desktop/serving_schema.go | 6e97b11a245224d64cd404bb9840ac76415f30d0 |
| internal/desktop/serving_schema_test.go | 5fab83e263dd5a16e4279c7b958dcbb0dddee8e8 |
| internal/scanruntime/scanruntime.go | 7c7d83e3a91f518ec7c797f72ad388d16aa10431 |
| internal/scanruntime/scanruntime_test.go | 012f86b625d822cf37b225b15a15ac9c4ad20c56 |
| internal/scanruntime/serving.go | cde1ff73383a3d4029c87707e93bc7fb6d71b28a |
| internal/scanruntime/source_identity_test.go | 0f51fe94435f715811d6949af77f08a1f46227d4 |
| internal/sessionorigin/codexotel.go | ebfa096a86a616d05ea5734fdabdbb0ffe2fe418 |
| internal/sessionorigin/reconcile.go | e7372727d0cfee4fcc0085972812d47b7d9beb62 |
| internal/sessionorigin/reconcile_test.go | ec456c2a5a0d287404ca055a5c0578f655399f9d |
| internal/sessionorigin/reducer.go | 11ece78288ec1e5c6626b79f733bac998bb7636f |
| internal/sessionorigin/reducer_test.go | c34b35bfb0518f16c77079edc8dc03826d4124b1 |
| internal/sessionorigin/selection.go | 8dfb2711b41385665eb9829e949da0b5a5b58a7b |
| internal/sessionorigin/selection_test.go | fb53d1ec206c4bf2a6b435786f3f4a81e6b69502 |
| internal/sessionorigin/sessionorigin.go | 38ff37b5af798d6519e335dc465ec2894234eb27 |
| internal/sessionorigin/sessionorigin_test.go | 2910d1bf200ba9c82b8d7fdbfbcd6f52f01f48d3 |
| internal/store/migrations.go | 2e71d493bcbdc00236993df6ec98ee7ad928ce9e |
| internal/store/session_origin.go | 7f5d5ced627de7b976c6c012f9a48019f67581cc |
| internal/store/session_origin_derive.go | 5adf5f2a05e56ba4b0b4f0ae88d94078b63645bb |
| internal/store/session_origin_test.go | 8b12e016502d712933287112ce38f40c65ce4ae2 |
| internal/store/store.go | 9a6c838ee757ba691e5015055205a5dccd1b0f22 |
| internal/store/telemetry.go | c2664ae1429a2a9dbca85cc392f24ebbb0ba4e38 |
| internal/store/telemetry_admission.go | 7304327b5cd5482bcf7878c4d66155f042e71137 |
| internal/store/telemetry_admission_test.go | 8f884f2c4bdfc6d99ced1c3304bec394c9410c8b |
| internal/store/telemetry_ownership.go | caa0acd0745ac916f53bae41a72866ab5f193af7 |
| internal/store/telemetry_ownership_test.go | 0202dba5aa4c2c8bb0730f2904d2442b7bc82614 |
| internal/store/usage_partitions_test.go | 8c9fcfe63b27dd8a95a2fdc483fe910b025f7c30 |
| internal/telemetry/admission_test.go | 954adf1bc581f28c4100d227a359916ebb44a801 |
| internal/telemetry/ownership_test.go | 6880d8c92f0908b2ec6fa454ace95680db97f8b9 |
| internal/telemetry/projection.go | 7f48fe678fe0b98deffb582d1221bac52baecbb3 |
| internal/telemetry/receiver.go | b2d8e6d68503ad05d1f13f2fd9defe25b642daf5 |
| internal/telemetry/reconcile.go | dd9c40d0b99f0d556209837283076ef8e07e5328 |
| internal/telemetry/source_identity_test.go | 135f368f928ded864139070ee08061271f98ffc0 |
| internal/usage/source_identity.go | f6aee082bcb28280991364dfaf3aff37b404c8b1 |
| internal/usage/source_identity_test.go | 4422c3cb55d33553f34acd2e775747a55b1736e5 |
| internal/usage/source_pipeline.go | c7ec5eae7b0c3f7c3d7f893439ce722b8cc9a605 |
| internal/usage/usage.go | fd929b88c027cc19a8e9528f31562edf488b69df |
| internal/watch/watch.go | 3b1bd901e611724279e88a85211a8ef563418f9b |
| internal/watch/watch_test.go | b22a0997f9da5bc2830e1e88752dec30ce32f13c |

Reviewer: 独立冷上下文Claude/Fable5.2 medium，主调度核对候选/日志/真实读回并采纳。
Method: 只读源码、合同、断言、日志和渲染核查，加隔离CLI恢复复现。
Scope: Task3及六合同全部声明范围，补齐此前未审断言/副本/错误链路/实际读回。
Evidence: 最终Go29包1680顶层PASS零失败、race3包15项、vet/双架构/size/原生构建通过。
Reviewer直接执行watch/sessionorigin及相关cmd/scanruntime定向组和两条CLI恢复复现，通过。
原生最终内容完整套件164+237+59零失败，3初始skip及两个独立restart阶段；Settings37项
通过并生成20图。主调度读回该日志；原有定向渲染证据仍按范围有效，完整重跑作为补充。
真实Codex0.169二进制e57da7be8695c079d2d1d156dab79708e43ebb6cf7e34527b8b01d6aa942788e，
隔离HOME/state与本地mock、无凭据。exec100/10仅JSONL，ephemeral100/10仅OTel，
TUI主100/10JSONL加标题100/10OTel，无双计。分类分别work、work、work+auxiliary。
主调度核对原始读回目录价0.000400000/0.000400000/0.000800000，provider_cost为null。
最后真实producer调用沿用原验收，watch修复未触及其happy接口；最终读回复用范围已检查。
Contract impact: primary CLI、aux architecture/requirements、scan-performance architecture、
本topic architecture/tasks均与实现一致。普通失败独立发布、pending分类/重试、clear例外和
规则检查点为已授权范围内的实现澄清。Task3 Review勾选为本轮结果同步，未改任务/验收。
Limitation: 原生schema可选矩阵未执行，非必需门槛；TUI无rollout主会话可被推断辅助，
exec Memories保留work。session-only watch对永久缺失身份每轮pending，等待正常usage清理。
仅索引可见而usage清单不可见的合成副本仍遵循core marker推断，不宣称完美用途识别。
Completion gate: VERIFIED

Task checkpoint

Task3评审通过，修复周期保持2/3。提交建议等待本状态门禁、专题边界及提交授权；
推送建议等待已验证提交和单独授权。主调度继续完成专题证据与交接收口。

### 最终状态同步

复评后仅 tasks.md 的 Review 勾选和末尾完成说明同步，无产品、测试或验收语义变化。
最终 tasks.md blob `90f70e0674ba15ab73db92f189fb5e4231af654b`；上列其余71路径不变。
Task3最终72路径指纹 `aecc8b6e4ce6877042ea517708bc256b73a704213ff0cba86f57ac9a3b310533`。
专题加入未变 requirements.md 后73路径指纹 `0974dd9fd038596f4229110e9b6bb36fd582bc2d218a236abb07f567a8c8eacc`。
主调度确认状态同步不影响已完成独立评审、测试及真实producer验收；不重跑未变产品验证。

CEv1最终查询：WorkUnit `otel-version-tolerant-metering:builtin-rule-set`，ContentState `otel-version-tolerant-metering:builtin-rule-set:state:aecc8b6e4ce68770`，VERIFIED 6/6；missing、invalidated、unresolved均为空。

### 专题完成门禁与交付边界

WorkUnit `otel-version-tolerant-metering`，ContentState
`otel-version-tolerant-metering:state:0974dd9fd038596f`：VERIFIED 3/3。
三项专题条件为集成计量、自动体验、合同与集成验证；缺失、失效及未解决影响均为空。
直接包含三个实现WorkUnit，分别已通过4/4、7/7、6/6。
Task1和Task2的原证据不改写：通过显式change/impact preserves及目标状态roll-up复用。
最终测试及独立复评确认新增分类回调、pending重试和规则检查点不削弱准入、JSONL排他、
历史原日期补计、准备已提交后的投影/发布恢复。当前架构和任务文档精确状态各VERIFIED4/4；
需求保持原epoch9通过状态，未改变。状态勾选与完成说明不影响测试适用性。

三个任务本地实现、评审和证据完成，等待提交授权；尚未提交、推送、安装或修改生产状态。
已知限制：TUI无rollout用途是有界推断；exec后台Memories仍留在Work；仅session域遇到
永久未评估来源会持续发scan_pending，直到来源可评估或usage扫描清理。可选native schema
矩阵未运行，不构成当前必需门槛。无运行中的worker或构建任务。
临时验收目录保留，不作为持久证据引用；必要事实已写入本记录和CEv1。
