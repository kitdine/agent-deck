---
status: active
topic: auxiliary-session-classification
subject: ux/widget.md
---

# Widget 框架评审

## Round 1 — 2026-10-27

## 📋 Widget 交互框架评审

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Published scope 与 Saved default 分离，旧范围、数量和时间作为同一快照保留，不把新偏好贴到旧数量上。
- 保留全部消耗与旧字段原义；unknown、未知覆盖、默认未确认、旧版本和整份读取失败分层明确。
- 只传聚合字段，隐私边界不扩张；沿用五 kind、三尺寸、刷新合同及原生验收要求。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与 Widget 设计。
- Method: 单评审者文档/共享源码核对、Node 合成聚合断言、隔离 agent-browser DOM 场景转换及保留截图目视检查；未委派、未修改产品或标本。
- Scope: Widget framework，尤其 Magnitude small/large 原有会话行；不是 final-surface、真实 App Group/WidgetKit 或规模性能验收。
- Reviewed state: HEAD `6b288ac27bb1c773212dd1baaee91d6317a4687d`；document blob `3b15e60bb77b3883dc3528e5dd5fd806657a2f25`。
- Specimen identity: manifest SHA-256 `e03374dbc2410075a610330ccb0cf3e8a9a9956aa676496114985ba05fb446b7`；11 项绑定文件全部 MATCH。
- Fingerprint: `c61abe910893bb436dcd4f491c3c889ee85753688b8dd98c9fac620173b86eda`；SHA-256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 对 today/30d × 八状态 × 三范围，共 48 个合成投影检查 Work+Aux=All、辅助 unknown=0、partial 未知数为 null、legacy/defaultUnknown 不生成 scope/count；未知 scope token 返回 unavailable。全部通过，只覆盖声明的合成窗口，不代表实际 client/period 配置实现。
- Evidence: 独立浏览器读回 Work 的 small/large 分别为 3/4、Unknown=1；仅改 Saved default 后显示不变。切已发布 Auxiliary 后为 1/2、Unknown=0，AX 名称展开 Auxiliary 并含 Today/30 days。partial 为 Unknown —；defaultUnknown 两处均为 Session scope unavailable。期间费用文本不变。
- Evidence: pending 显示原 Work 数量和 `Last updated 8 hours ago`。No snapshot yet、Container unavailable、Read failed、Unsupported version 四场景各有 3 张 typed failure，分类计数节点均为 0，确认整卡失败优先。DOM 尺寸为 148×148、286×148、286×330。查看保留 work-en-dark.png，范围、单位、未知说明与图表在既定比例内呈现。
- Contract inspection: 现有 AgentDeckWidgets.swift 使用五种 Widget 和三 family；Magnitude 使用 ClientPeriodWidgetIntent。WidgetViews.swift:140–160、676–735 的生产表面读取实际 entry client/period，不可在实现中把本标本固定的 all-clients/today/30d 示例变成硬编码配置。本文已要求实际 client/period 同代投影，后续 architecture/final/native 验收必须兑现。
- Contract inspection: desktop-refresh/ux/widget-refresh.md 与当前 Frame 的整卡 failure 替换、旧年龄和无宿主探测边界一致。聚合源未含身份/项目/正文；真实 serialization allowlist 仍需实现验收。
- Evidence reuse: 设计者 build、920 项浏览器和 axe 结果保留为原作者证据；本轮执行上述独立窄范围检查，不冒充复跑全部。无关 axe-core 开发依赖告警未阻止页面，不宣称本轮 axe/native 通过。
- WorkUnit: `auxiliary-session-classification:ux/widget.md`；ContentState: `auxiliary-session-classification:ux-widget-r1-c61abe910893bb43`。
- Completion gate: VERIFIED
- Gate scope: 仅 framework 的独立评审、现有文档本地链接/结构/空白与 manifest 完整性；architecture 缺稿归属 tasks.md 文档集检查，不能反向阻塞此框架。原有门禁范围勘误保持有效。
- Evidence: widget 文档/评审记录/tasks/prototype README 四项独立文档链接及记录检查为 0 errors；`make check-whitespace`、`git diff --check` 成功。完整 topic audit 退出 1，仅 architecture 未编写，保留该文档集缺口。
- Hierarchy: 四份 UX framework 现在均通过，architecture 的框架输入已齐；Widget 最终核对仍待 architecture。本轮不关闭整个 Widget 文档任务或主题，不授予架构设计或交付权限。
- Residual uncertainty: 极大计数、所有实际 client/period 组合、动态字号/VoiceOver、WidgetKit 刷新、跨进程原子发布与真实性能仍未验收。共享消费者后续变化须在 final-surface 核对联合内容身份。

Task checkpoint：ad-auxiliary-session-classification-doc-ux-widget；content_state `c61abe910893bb436dcd4f491c3c889ee85753688b8dd98c9fac620173b86eda`；gate VERIFIED（framework）。
提交建议：经授权提交 Widget 文档、共享 Widgets/聚合源、检查脚本、manifest/截图、此评审记录及矩阵的 Widget 范围；此前状态勘误属于另一已授权修改范围，提交时区分。
推送建议：经授权提交并验证对象、Codex trailer 与 SSH 签名后推送 feature/auxiliary-session-classification；remote/upstream 尚未解析。不以 framework PASS 宣称 final-surface 完成。

设计：auxiliary-session-classification / architecture.md
