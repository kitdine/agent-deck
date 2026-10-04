---
status: active
topic: launchservices-recovery
subject: requirements.md
---

## Round 1 — 2026-10-03

## 📋 LaunchServices Recovery 需求边界独立评审

📊 综合评分：9/10

✅ 评审结论：PASS

Reviewer：全新本地只读 Codex CLI `01a10235-4217-7692-a2c9-cb57dfd06580`，未参与产品写作、未委派。主 writer `01a101d7-cbe1-7692-b209-c1b6190fb27d` 复核冻结身份、现行源码、精确回移比较、实际 CLI exit 0 和状态收口；评审记录/Beads/CEv1 由主 writer 独占。
Method：冷读冻结文档、现行权威核对、本工作区 CodeGraph 与一次精确源码回退、既有 native/L0 回执核对、内存文本比较；不复用旧评审结论作为本轮 PASS。显式 gpt-6.1-sol/xhigh、on-request/read-only，沿用现有 provider/default tier 配置；服务端计费和实际 response tier 未验证。
Scope：完整 requirements、初始 Documents/Tasks 声明、主规范单一例外回移，以及相关现有 doctor/desktop 行为和安全/性能边界。不是未写出的 UX/architecture 或最终 decomposition 审批。
Reviewed state：HEAD `24623ec8bf0e172bf8e630ebe203163e76634403`；requirements.md blob `1b8f4e001162f2fe3ca5016e854c7ccbf2040846`；初始 tasks.md blob `19aeacaad305159b917af109bf77728fc646d9e7`；feature cli-design.md blob `0247e9d7dd693d22553d265b91443ac4b5f190e3`。

### 🔴 严重问题 — 必须修复

无。未登记 `LS-REQ-R1-Fn` 或其他开放、回归、替代 finding。

### 🟡 改进建议 — 建议处理

无。未来 UX、architecture、字段命名与实施拆解遵循已声明的正常依赖顺序，不将未完成的后续阶段误记为本需求边界缺陷。

### 🟢 优点

- 现有 `doctor.Service.Check`、CLI 两种 renderer 和专用 desktop DTO 支持所述前提。保留现有 partial/提前返回/`checks_skipped`、错误及成功报告退出语义；不将客户端 extension inventory 混作 OS 注册资源。
- 主应用 API 和 appex 专用 PlugInKit 分开，枚举范围、控制失败、超时/截断/未知格式和空 appex 均不能假装健康。不同 live build 仅支持可能冲突；副本和注册条目不证明 Widget 因果或恢复。
- 指引要求确认过时副本、保护规范 app/Widget 并区分注册变化与实际 Widget 恢复；没有自动清理或 copyable OS recovery action。
- 原始路径/build-entry 细节限制在本地 doctor；desktop 继续专用 allowlist 和未知值 fail-closed。既有性能目标未获新豁免；500 ms/1.5 s 是待实现验证的上限，不是假造测量。
- Documents 初始集完整声明，UX/architecture 保持未写、Draft 未勾选，Tasks 未批准。需求先审、后续框架/契约/final UX/decomposition 的顺序与现行工作流一致。
- 主规范回移是 14 行单一插入、0 删除；独立比较与主 writer 比较均确认新条款等于批准来源，移除插入后原 release-line 基线不变，没有整包导入 main。

### 📝 总结

本轮 PASS 仅表示需求边界、初始文档集与精确例外回移通过。不存在实现、collision/root-cause/recovery 或 Widget timeline 验收 PASS；未关闭 origin。所有下一阶段内容仍须独立评审。

Evidence：

- 独立 reviewer 的 HEAD 和三份 blob 检查 exit 0、全部匹配；只读内存比较与两次 git show exit 0。例外段落 SHA-256 `86e5497b5f8f9dba658ffa3ed14fa802829f9834c3c5bfd4ab3ab5692a803165`。
- 主 writer 在冻结态执行 make check-whitespace、git diff --check，各自 exit 0；关联空日志 digest 已核实，reviewer 正确复用而未重跑。原回执 `/tmp/agentdeck-doctor-v065-resume/requirements-candidate.json`、`primary-exception-backport.json`。
- 主 writer 直接核对 doctor.Check/Report、desktop.loadHealth/healthSnapshot/HealthCheck 专用字段，以及既有 snapshot-performance 目标；认可独立报告的当前源码前提，未把 CodeGraph 关系当运行时证明。
- 已有 target-limited ordinary-user PlugInKit 回执 exit 0/0.159 s、一个规范 nested appex，只是来源可行性。旧 application API host 四 URL/Finder 控制和 appex -10814，以及真实 dump 五秒 timeout 全部保留原边界；本轮未重跑 probe。
- 未运行全主题 check-topic-docs：本轮不审最终 decomposition 或宣称文档集已就绪；初始声明的后续 Draft 明确未完成，现行生命周期允许该状态。未运行产品套件、构建、系统恢复或全局运行时检查。
- reviewer 首次 here-document 比较因 read-only sandbox 的临时文件限制未执行；改为内存比较成功，未提权。主 writer 保存独立 raw report 与实际 CLI exit 0；reviewer 未写 repo/Beads/CEv1/work-state，也未直接调用 Hook。

完成门禁：VERIFIED。
主 writer 的 canonical exact-state gate 返回 boundary/safety/acceptance 3/3 pass，missing/invalidated/unresolved 为空；8 个节点和 9 条关系经 standard templates、关系 preflight 与 exact payload/endpoints read-back 核实。requirements 与主规范 blob 未变，matrix 只同步本 requirements Review 和交接；没有覆盖初始 ContentState 身份或将后续文档宣称完成。原生回执仍只是可行性，没有变成 recovery PASS。
最终 make check-whitespace、git diff --check 各自 exit 0。当前 release-line 的 parser 有独立 pure `latest_review_section` helper；首次提取缺失该依赖返回一次 NameError 后，先通过 CodeGraph 静态核实调用链，再仅隔离编译这两个 pure 函数和三项 regex constants，得到 PASS/NOT_VERIFIED 并核对准确 requirements.md blob qualifier。未执行 runtime helper、Hook entry point 或 Beads 子调用。查询后只更新唯一 live gate，再检查最终 PASS/VERIFIED。收据 `/tmp/agentdeck-doctor-v065-resume/requirements-final-validation.json`、`requirements-r1-final-gate.json`。
WorkUnit：`launchservices-recovery:requirements.md`；ContentState：`launchservices-recovery:requirements.md:state:278f0949b0cd72da990247f74f451646aa0dafc7f1f00d71d6aca272779fa93c`；dispatch：`ad-launchservices-recovery-doc-req-design`。

主 writer 同步仅勾选本 requirements 的 Review 并更新当前交接；初始 Documents 集、空 Tasks、需求正文和主规范 blob 不变。记录该 scope-aware metadata assessment，不改写已登记 ContentState 的初始 matrix 身份。
原报告 `/tmp/agentdeck-doctor-v065-resume/requirements-cold-review.md` 与 JSONL/stderr 同目录保留。

Task checkpoint：requirements 本轮 PASS、scoped gate VERIFIED；建议提交本任务的 requirements、初始 matrix、精确 primary exception backport 和本评审记录。未来 contracts、implementation 和 topic delivery 边界保持开放。
提交建议：required gate VERIFIED 后，生成新的 signed English Conventional logical commit，实际检查 body、Codex trailer、SSH signature、parent/tree 和文件边界；不将未来 Doc/implementation 混入。
推送建议：ordinary 同名 feature push/Draft PR 目标 release/v0.6.x；最终 topic head 的 GitHub review/CI 和 ancestry-preserving merge 仍须完成，发布排除。
下一项：本任务证据与逻辑交付后，进入 `ux/cli.md` framework；原 Bug 保持 open，实现仍禁止。
