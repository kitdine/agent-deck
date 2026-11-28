---
status: active
topic: subscription-quota-conversion
subject: requirements.md
---

# requirements.md 评审记录

## Round 1 — 2026-11-18

### 📋 requirements.md 设计评审

📊 总分 6.5/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `QCV-D-R1-F1` · 阻塞 · 第 19–22 行。账号归属是条件估值的前提，但现有用量事件缺会话所属账号。Claude 配额观测自身也没有账号 ID。两个实际客户端都可能只显示估值不可用。 处置为开放。
  - 💡 修订范围为在当前基线上定义明确受控的条件测量。Codex 校验探测端点的账号摘要，Claude 保留同账号条件和已知身份变化的拒绝路径。
- `QCV-D-R1-F2` · 重要 · 第 20、26 行。同一账号的全局百分比无法证明区间中只有目标会话，现有来源没有逐请求扣额。 处置为开放。
  - 💡 修订范围为限定生产输出为同期变化及明确带假设的估值，用隔离夹具验证算式。
- `QCV-D-R1-F3` · 次要 · 交付边界。CLI 用户契约未引用框架，菜单栏及 Widget 首发状态未列明；语言适配一词混淆客户端与文本。 处置为开放。
  - 💡 修订范围为引用首发 CLI，明确 native 会话估值不适用及客户端字段缺失。

#### 🟡 建议改进

无额外建议。

#### 🟢 已满足的部分

确定了单一会话金额、分窗估算、有依据的舍入区间和旧数据的永久缺口。

#### 📝 总结

评审状态为 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，文档 blob `eeb0a957c6ee25979756ff62c6875668592559b7`。方法为 Claude Code Opus 5.5 独立冷上下文只读评审，主控按实际候选和源代码复核。T3 请求 `v080-quota-version-design-review-e2-a1`，选择的推理档为 `high`，实际运行档未独立观测。客户端证据见 [窗口观察](../../../../internal/quota/state.go)、[Codex 账号字段](../../../../internal/quota/codex.go) 和 [Claude status line](../../../../internal/quota/statusline.go)。

证据为绑定文档的 `bash scripts/check-topic-docs.sh subscription-quota-conversion` 和 `python3 scripts/check-docs.py` 通过，语义缺口见本轮发现。

完成门禁：NOT_VERIFIED

下一步修订本轮开放发现并对新文档状态复评。

## Round 2 — 2026-11-18

### 📋 设计复评

📊 总分 9/10

✅ Verdict: PASS

#### 🔴 必须修复

无开放发现。F1、F2、F3 关闭。两端账号证据按客户端区分，真实输出限定为同期观察与条件估值，首发表面已声明。

#### 🟡 建议改进

无须修复的发现。

#### 🟢 已满足的部分

独立复评复核了原始发现、源码可见字段及报价分子的同范围约束，候选能支持明确带假设的估值和不能估值的原因。

#### 📝 总结

状态为 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，文档 blob `d5e329ddc1bd8c06556cb12c3b73a7956707ea86`。Claude Code Opus 5.5 冷上下文只读复评，配置 `high`，实际推理档未独立观测。请求 `v080-quota-version-design-rereview-e2-a2`。主控核对同一 blob 和实际终态。复用已通过的文档结构/链接/空白检查，不延伸到产品验收。复评中额外建议涉及标本取值提示和防御夹具的边界，不改变已冻结的估值或读写契约。

完成门禁：VERIFIED

CEv1 WorkUnit `subscription-quota-conversion:requirements.md`，ContentState `subscription-quota-conversion:requirements.md:state:55359028d46c0bd1bac207272cccb8b9d4ba075c406f94a8cbc5afe401326a28`。三项必要证据均适用，三个列表中的缺项、失效与未决影响均为空。

#### 文档任务检查点

提交建议为当前文档及评审记录等待对应动作授权。推送建议按该待授权交付的范围核对分支和签名。
