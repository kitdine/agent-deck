---
status: active
topic: cost-transparency
subject: tariff-catalog
---

# 历史价卡评审记录

## Round 1 — 2026-11-18

### 📋 历史价卡评审

📊 总分 8/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-C-R1-F1` · 中。`internal/usage/price_rules.go:115` 对首发费率也要求越过无时区日期的最晚边界。独立探针在 Astra、Haiku 和 GPT‑6.1 Sol 的首日用量得到缺价。区分有证的首次费率与存在旧价的切换，保留历史适用范围。
- `CT-C-R1-F2` · 低。`internal/usage/price_rules.go:689` 按保证有效区间查重，错过可能重叠的 07-29 和 07-30 声明。按各自可能区间核对，接受同一切换点相接的区间。
- `CT-C-R1-F3` · 低。`internal/usage/price_rules.go:560` 的大小写折叠比 JSON 解码匹配更窄。Unicode 键能让后写值静默生效。用 ASCII 字段合约防止隐藏覆盖。

#### 🟡 建议改进

主控复核了 [Enterprise USD 官方价卡](https://help.openai.com/en/articles/20001415-chatgpt-rate-card-enterprise-token-based-pricing)，GPT‑5.6 Sol 的促销至少覆盖 11 月 21 日。目录的 `observed` 区间保守地绑定已公布的覆盖范围，不能据此推导之后的费率。

#### 🟢 已满足的部分

固定 LiteLLM 基础、规则摘要与生成稳定性、原模型表、用户要求的六个金额及套餐选择域均已核对。

#### 📝 总结

HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，八个实现文件指纹 `8d4130d68ae63d4711e6e424a70ff54ea29e61f5055cc33c17f82df414afcc93`。Claude Code Opus 5.5 冷上下文只读评审 `v080-tariff-catalog-review-e3-a1`，配置 `high`，实际推理档未独立观测。主控核对精确作用域、旧模型表以及已有 Go 与固定目录复现结果。三个定向探针支持上述发现。未重新读取所有外部价源。

完成门禁：NOT_VERIFIED

## Round 2 — 2026-11-18

### 📋 历史价卡复评

📊 总分 9/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。`CT-C-R1-F1` 关闭，有来源证明的首次费率覆盖首发日，已知旧价切换保留不确定窗口。`CT-C-R1-F2` 关闭，校验检测可能重叠的声明，完全相同的切换点可相接。`CT-C-R1-F3` 关闭，JSON 规则键限定 ASCII，并回归覆盖 Unicode 别名。

#### 🟡 建议改进

无须修改的发现。已知当前目录的非 effective 规则和首发规则的渠道或服务档互斥。

#### 🟢 已满足的部分

71 条规则和带有效期证据的 40 条首次费率通过生成及解码。旧模型表与基线按正则化 JSON 后逐项一致。用户要求的套餐政策及不同付费渠道已有明确来源。

#### 📝 总结

HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，八个源码和数据文件指纹 `85ce643b3279c6f19cd05243c083c445c1ecf4ab71945b8d1e407c7e492f8a58`。Claude Code Opus 5.5 冷上下文只读复评 `v080-tariff-catalog-rereview-e3-a2`，配置 `high`，实际推理档未独立观测。主控核对终态及一致的精确代码指纹。完整 generator、usage、CLI 包检查及 Go build、vet、文档和空白通过，独立复评的价格规则定向用例也通过。原始下载曾返回 503，取回固定提交 `f2479cc704f6e63d5510929d30ce8e11ffe43467` 的 Git blob `c9d871fc41d5ae95746c83e26718db24cc4660b1`，经大小和 blob 校验后通过已有生成器的生成与逐字节检查。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:tariff-catalog`，ContentState `cost-transparency:tariff-catalog:state:85ce643b3279c6f19cd05243c083c445c1ecf4ab71945b8d1e407c7e492f8a58`。三项必需证据适用，缺项、失效和未决影响均为空。

#### 任务检查点

该任务达到本地提交边界。提交和推送依各自的交付授权执行。计价事实存储依本任务的真实交接继续。
