---
status: active
topic: cost-transparency
subject: requirements.md
---

# requirements.md 评审记录

## Round 1 — 2026-11-18

### 📋 requirements.md 设计评审

📊 总分 6.5/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-D-R1-F1` · 重要 · 验收样例。精确金额和阈值缺少逐组件单价、适用价卡及核验日期。补齐可复算的公式和证据。 处置为开放。
- `CT-D-R1-F2` · 重要 · 计价口径。个人订阅的渠道估算未定义，和现有 provider_cost 原生主金额有关。明确完整输出、套餐状态和渠道金额。 处置为开放。
- `CT-D-R1-F3` · 重要 · 历史重算。缺确定历史值时主金额会变化。明确参考态、现有 5m 默认和首发历史规则覆盖。 处置为开放。
- `CT-D-R1-F4` · 重要 · 来源决定。未逐项消解已撤销的旧排期与新用户范围。逐项记录价源审计、Sol 历史和延期项。 处置为开放。
- `CT-D-R1-F5` · 重要 · Codex 套餐与评审。未定义免费活动的证明条件和 guardian 未采集时的结果。分辨审批审查、PR、本地及 Security。 处置为开放。

#### 🟡 建议改进

无额外建议。

#### 🟢 已满足的部分

逐笔费率和原始用量分开、历史参考值带来源、已知零和未知分开。范围限定为计价及额度主题的输入。

#### 📝 总结

评审状态为 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，文档 blob `e2eee9ea2a04bc1e68648f12c484e97a06a9da09`。Claude Code Opus 5.5 冷上下文只读评审，配置 `high`，实际推理档未独立观测。请求为 `v080-pricing-design-review-e2-a1`，主控按原候选、实施基线和已核实的供应商证据裁决。结构检查通过。独立 Reviewer 核验了样本算术及内部形状，供应商链接由主控更早逐一核对，其逐文档引用是本轮修订内容。

完成门禁：NOT_VERIFIED

下一步修订该文档的开放发现并对相应候选复评。

## Round 2 — 2026-11-18

### 📋 计价设计复评

📊 总分 8.5/10

✅ Verdict: PASS

#### 🔴 必须修复

F1–F5 关闭。价卡、算例、套餐状态、历史参考、用户范围和审批审查的定义均通过复评。

#### 🟡 建议改进

Enterprise 普通输入价可在样例表复述；现有表、定义和算例组合已明确，当前内容可复用。

#### 🟢 已满足的部分

历史价及现行价的估算证据和 API／套餐区分得到复核，结构检查通过。共享原型的已评清单 SHA-256 为 `28ab4177523e74a504089ccdfced5cd46f9f526ba1782a78118ad6864badd5eb`。

#### 📝 总结

评审 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，文档 blob `5a78cb37ba61b8f1cb9bec138e0c80f58636f659`。独立方法为 Claude Code Opus 5.5 冷上下文只读复评，请求 `v080-pricing-design-rereview-e2-a2`，配置 `high`，实际执行档未独立观测。主控核对终态、候选和结构证据。复评重算了六个金额，供应商价卡复用主控的核验。原型检查覆盖其记录的内容，并不外推到原生界面。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:requirements.md`，ContentState `cost-transparency:requirements.md:state:d3344d8490763b502f740a13c85b7c1b010a24c23358211e91d75e2b5511ca85`。三项必要证据均适用；缺项、失效和未决影响为空。

#### 文档任务检查点

提交建议为当前需求与评审记录等待对应动作授权。推送建议按该待授权交付的范围核对分支和签名。

## Round 3 — 2026-11-18

### 📋 计价设计复评

📊 总分 9/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。新范围保留旧估价，新增渠道估算走独立字段。已有费率、历史重算和用户目标适用之前结论。未发现新的范围缺陷。

#### 🟡 建议改进

可在实现任务列出旧读者的其他金额路径以及跨口径差异。该验证与现有设计一致。

#### 🟢 已满足的部分

用户批准的范围清晰，精确计价、参考估价、旧读者兼容和已证订阅规则可以分别测试。

#### 📝 总结

HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，文档 blob `d3fc7a5b780ed86fb4daf71fc7f6b988442ee98e`。Claude Code Opus 5.5 冷上下文只读复评 `v080-pricing-design-rereview-e3-a3`，配置 `high`，实际推理档未独立观测。主控复算对应 blob 并核对终态。候选文档的结构、链接及空白检查通过。金额兼容是旧格式化器加隔离构造数据的回放，本轮属于设计验收。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:requirements.md`，ContentState `cost-transparency:requirements.md:state:485dae760cecbaed6fe02505ee44f2d0acfcbb15ae07a082a0079ce2ce1a3270`。三项必要证据适用，缺项、失效与未决影响为空。

#### 文档任务检查点

提交建议为对应文档和评审记录等待动作授权。推送建议在授权交付时核对提交范围、签名和远程状态。
