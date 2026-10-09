---
status: active
topic: scan-performance
subject: performance-evaluation.md
---

## Round 1 — 2026-10-09

## 📋 性能评估方法与结论评审

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- 撤回浏览器首帧加速比，明确两次 native UI 尝试无有效样本。
- 三种已存数据前置状态、12 对原生模型 A/B、3 对首采 pilot 与 2 对待入库 pilot 分开归因。
- 主动保留原位 15.545s、usage 12.045s、621.5MiB 负结果，不宣称实时或资源门已通过。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；用户指定 actor `opencode`。
- Method: 独立主会话审查全部方法/决策门，核对三个脱敏结果 JSON、manifest 与 seven harness 源码的相关路径，重算摘要；不重新访问真实日志、数据库或重跑性能实验。
- Scope: 测量归因、摘要算术、适用边界及 E0–E4 计划。需求 P01–P11 是未来产品验收，并非本文件 PASS 的测量成果。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`。
- ContentState: `scan-performance:state:01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`；SHA256(`head=<HEAD>;document=<blob>`)。
- Supporting specimen/result identity: manifest SHA256 `739b67d01e67f7a5427946e41a80c2302ab64f9d8149901c9cef6ff109a428fe`，43 个列入文件逐项 SHA256 全匹配。此处记录依赖身份，不更改既定 document digest recipe。
- Evidence: `bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace` 退出 0；原生 A/B 摘要为 1919.1852805ms / 15.0885635ms；首采为 75437.596834ms / 1553.772468ms；与文中四舍五入一致。firstload harness 保留整文件解析与交错顺序；history harness 从 SQLite backup 分组；native harness `testNativeDataPreparationAB` 仅计 coordinator/decoder/model，未计 popover。
- Dependencies: requirements 的验收提案与本文件一致；tasks 对 E1 的入口/出口使用问题由 `reviews/tasks.md` 承接，不是实验结果缺陷。
- Limitations: 原始私有输入已按既有交接清理，本轮不能独立复现历史性能观察；只批准可复查摘要、方法及诚实限制，不认证真实 native frame、尾延迟、OTel 或资源预算。
- Completion gate: VERIFIED
- Gate evidence: Neo4j MCP 标准 gate-status 查询在上述精确 ContentState 上返回两项 required criteria 均通过，missing/invalidated/unresolved 均为空。
- 产品实现、安装、发布与整套设计批准仍未完成。

### Task checkpoint：ad-scan-performance-doc-perf

文档 blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`，门禁 VERIFIED，待授权交付。

提交建议：性能评估文档、其引用的已审摘要/实验 harness 及本轮记录；共享标本依赖需保持本轮身份，可等待修复后的完整设计批次。

推送建议：仅考虑 `feature/scan-performance`；远端目标未核验，须先完成授权提交与依赖范围核验，再另行授权推送。

## Round 2 — 2026-10-09

## 📋 性能评估结论复用与依赖影响复核

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

本记录原无开放 finding。Task 2 已正确区分 pilot 输入与 E1 产品出口，未改变历史实验或将其升级为产品验收。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 原内容身份/测量依赖核对与结论复用，未重复真实规模或 native 实验。
- Scope: 全设计复评中的测量结论适用性；无新增、未闭环或回归问题。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`，与 Round 1 完全一致。
- ContentState: `scan-performance:state:01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`。
- Evidence reuse: 三份性能结果及七份原实验 harness 的哈希仍与 Round 1 manifest 对应值一致。新 manifest `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7` 变更的是合成 UX/输出及检查标本，不改变性能测量归因；复用原两项 required criteria 的证据与限制。
- Completion gate: VERIFIED
- 原始私有输入未留存、native first-frame 未通过、实时/资源负结果仍保留；设计 PASS 不改变这些限制。

Task checkpoint：ad-scan-performance-doc-perf；content_state `01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`；gate VERIFIED。

提交建议：评估文档、已审结果/harness 与评审记录，可随完整设计批次授权交付。

推送建议：feature/scan-performance；远端未核验，先完成授权提交，再另行授权推送。
