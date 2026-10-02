---
status: historical
created: 2026-10-02
retired: 2026-10-02
---

# 缺陷：默认交互探针在普通详情上断言待采集，无法发出 ALL PASS

## 现象

Beads：`ad-bug-prototype-probe-pending-assertion`。Lane A；本批仅此项与
`ad-bug-prototype-statchip-fidelity`，工作区 `agent-deck.prototype-fidelity-batch`，
分支 `fix/prototype-fidelity-batch`，基线 `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`。

未修改源码时，以 `?probe=1&lang=en|zh` 运行完整探针，均得到 7 FAILED：
待采集断言恒假，另有刷新与读取关闭的六条旧探针断言失配。

## 根因

`SignalDetail` 仅在 `state === "pending"` 显示 `.pending-banner`，探针却在
普通状态进入详情后要求它存在。另两组是既有界面变化后的探针漂移：刷新控件本身
已是 `button.refresh.failed`，而非含有按钮的容器；`59fa9f9` 已让读取关闭时隐藏
额度 tab 并回到用量页，旧断言仍要求显示两张未读取额度卡。

## 修复边界

仅改 `prototype/src/probe.js`：显式进入普通状态并验证无待采集提示，再切 pending，
逐个进入三种信号详情验证完整本地化提示，返回后重新定位 React 重建的节点。
刷新重试选择器及读取关闭断言同步到既有界面行为，恢复完整探针的 ALL PASS。
未改这些界面行为、原生产品、footer、依赖或全局运行配置。

## 验证

- 最终候选 HEAD：`4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`。
- 两项共享的代码、测试、依赖及原生契约指纹：
  `34a4d7b5ca7cd27d8728abba5367c850e965b3e634b0cbe73f5fcb0196072116`。
- RED：未修改基线中英完整探针均 7 FAILED，包含本项的恒假断言。
- GREEN：构建预览 `http://127.0.0.1:4186/?probe=1&lang=en|zh&run=final`，
  两种语言全部通过；实际计数以证据文件为准。
- `npm run build --prefix prototype` 通过；按 L1 选择原型检查，不运行无影响的 Go 或原生套件。
- 本机证据目录：`/tmp/agentdeck-prototype-fidelity-01a0fd06/`，
  `red-probe-{en,zh}.txt`、`final-probe-{en,zh}.txt`、`build.txt`、`content-state.json`。
  开发服务器曾返回旧模块，其结果未作为 GREEN；最终使用构建预览。
- 浏览器证据只覆盖原型；菜单栏图标真机验收由用户 deferred，不标为通过。

## Review — Round 1

## 📋 pending 探针修复独立评审

📊 综合评分：9/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 本轮必须闭合

无。

### 🟢 优点

| 验收项 | 实现及独立证据 | 结果 |
| --- | --- | --- |
| 普通详情无待采集误标 | 显式 normal；既有 SignalDetail 条件及中英完整探针 | PASS |
| pending 三种详情有完整本地化提示 | 切换并断言 pending；每次返回后重新定位入口 | PASS |
| 刷新及读取关闭符合既有界面 | 源码交叉核对实际按钮和 tab 行为；中英最终断言 | PASS |
| 回归量具能检测原缺陷 | 两种语言基线各 7 FAILED，最终各 89 PASS、0 FAIL | PASS |
| 原型构建 | `build-hooks.txt` 成功，源码和依赖哈希与候选一致 | PASS |

### 📝 总结

Reviewer：`prototype_selective_rereview`，独立冷上下文、只读；主 agent 直接核查
源码、清单、计数及截图后裁决。批次为既有冷评审的选择性后续轮，已消耗 2/2 轮；
本记录首次形成完整报告。Checklist：54/54 complete；Incomplete：None。
Method：工作区绑定的 CodeGraph，结果不适合时精确源码检查；Git 差异、独立逐文件
哈希、既有 RED/GREEN、二十个矩阵及实际截图检查。未重跑未变更套件。

Reviewed state：HEAD `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b`；
可独立重算的产品指纹
`7c8bd300976de22e9ab132964867ca134e61765497f01aaca7a6b66b97f6170d`。
清单为证据目录的 `resume-product-manifest.json`：对其 payload 使用 UTF-8、
排序键、紧凑 JSON、无末尾换行计算 SHA-256，包含排序后的 11 项路径/哈希、HEAD
及 Node `v26.10.0`。旧清单的每项哈希均已核对；旧聚合指纹的配方未保留，
不声称重新算出了旧聚合值。

Tests：UPDATE 既有探针，保留中英用户路径和失败/恢复信号；无新增依赖或低价值测试。
Documentation：UPDATE 本修复记录；未新增 topic 或改全局状态。
Completion gate：由 CEv1 WorkUnit
`urn:ce:agent-deck:work-unit:fix:prototype-probe-pending-assertion`
保存候选、不可变提交及实际合并内容的目标绑定结果。
Residual risks：仅原型浏览器验收；菜单栏图标真机验收仍 user-deferred。

## Delivery

本项 SSH 签名提交：`5bf15930ee037e09839974a69903a10560bdf514`；同批最终产品
head：`5a96475d9e1cbe79f8f856b7a6c6e6d00d093721`。
[PR #33](https://github.com/kitdine/agent-deck/pull/33) 在精确 head 的 Codex Review
无发现及 CI `verify` / `desktop` 全部成功后转 ready，实际 merge commit 为
`951b16dcb036c8e2b24ca3b2fcb0a2e367324efa`。
实核 parents 为 `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` 与上述 head；
tree `2182087b84f63dc5497d975b104c7100668c8f95` 与已评审产品 tree 相同。
两项产品提交的 SSH 签名、完整消息及 Codex trailer 已验证；GitHub 报告实际 merge
PGP 签名 `verified: true, reason: valid`。本项实际 merge CEv1 VERIFIED 5/5。
按用户明确约束，记录保持 active 直至实际合并；现在才归档，并保留全部评审历史。
原生菜单栏图标验收仍 user-deferred；本批没有发版或部署。
