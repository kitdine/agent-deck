---
status: active
topic: v0-7-0-contract
subject: v0-7-0-contract
---

# Version Contract Review

## Round 1 — 2026-11-16

## 📋 v0.7.0 合同收口评审

📊 总体评分: 9/10

✅ 结论: PASS

- Reviewer: Gemini 4 Hydrogen / Antigravity，medium，独立冷上下文；Codex 控制器核对身份与证据并记录正式结论。
- Method: 只读候选、源码契约、归档完整性和直接升级诊断核对；复用未变产品验证。
- Scope: Task 2 稳定契约、手册、路线图、状态/入口、四主题整体归档、导航与文档检查器兼容修正。
- Reviewed state: HEAD `67fee8e071ef3a3298ce0942f201612c98c29562`；冻结 tree `cc39b5983b4683eba20510261cf0399f7f3deebe`。控制器以 `git diff --cached --exit-code` 直接确认 index 相同。
- 完成门禁: VERIFIED

### 🔴 严重问题 — 必须修复

无。独立评审零发现，接受候选。

### 🟡 改进建议 — 推荐

无新增建议。保留现有豁免与边界，不把发布阶段转为无条件重复性能测量。

### ✅ 已核实范围

1. 稳定契约 revision 37 与手册明确 core schema 35、独立 session 索引、可写打开自动迁移、只读 doctor、Codex parser 8 与 Claude parser 7 复用、每批最多 2,000 条 OTel 历史重新接纳以及安装渠道登记。
2. Schema 35 清空旧分类缓存并排入派生重建，下一次正常扫描前可暂显 unknown；偏好与人工覆盖保留。定价重设计和 OpenCode 产品支持继续排除。
3. 四个主题完整移入 archive，评审记录随主题移动；100 个非 Markdown 原件逐个对 Git blob 核验不变。历史 scan-performance 未勾选复评项按用户停止额外循环的决定保留，不伪造 PASS。A5、Cask 和其他原生限制保留。
4. 归档链接、原型入口和路线图历史锚点保持可用。文档检查器仅扩展归档主题返回 active topic 的相对路径识别，不豁免自身 UX 文档要求。四个针对性路径用例通过；独立评审再次运行全局文档检查、合同和四个归档 document-set 检查，全部退出 0。控制器 whitespace、diff 与 shell 语法检查通过。
5. 私有真实状态副本以只读 SQLite online backup 分别快照，准确覆盖 34→35；不是跨库原子备份。构建源码为上述 HEAD，CLI SHA-256 `068402199480ffe816523dc9e432aa2fe6d5ff09501eb76cde63b8bc50e6d104`。控制器与评审者直接核对迁移响应、前后 integrity 和表摘要。Core 50 项中的 43 项、session 14/14 项保持不变；19,282 个事件与 token、14 条凭据密文、设置和索引保留。预期改变仅为 schema/派生计数和分类重建。必要方法、时间、摘要和限制已记入 [任务证据](../tasks.md#task-2-implementation-evidence--2026-11-16)，不依赖临时路径。

### 边界与复用判断

真实源日志被 sandbox 拒绝读取；只读 doctor 的 source-unreadable 计数不是数据丢失。
真实数据 30→35 路径未在本机执行，沿用既有迁移回归；源日志重放、receiver 与全局跨库
一致性沿用已接受的各自测试/组合验收范围，不冒充本次副本验证。原生 A5 和分发限制不变。

本轮只改变文档位置、说明与文档检查脚本；产品、依赖、构建配置没有变化。
Task 1 组合证据和已验证产品结果可以复用。最终记录同步仅追加本轮结论、更新 Review
标记及门禁引用，不改变被审产品或合同含义。当前发布基线仍是 v0.6.5；精确 SHA L4、
GitHub 交付评审、签名发布和本地正常升级是后续独立边界。

### 交付检查点

Task 2 Review PASS；检查点 `v0-7-0-contract:v0-7-0-contract:review-20261116-d06720a8`
已 VERIFIED 5/5，无缺失、失效或未解决影响。最终记录同步按相同范围显式复用并绑定
`v0-7-0-contract:v0-7-0-contract:closure-final-20261116`；授权签名提交后处理 Topic 边界。
独立评审无待运行进程，未改任何文件或证据；父控制器拥有 Git、CEv1 和 Beads 操作。
