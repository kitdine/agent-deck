---
status: active
topic: auxiliary-session-classification
subject: architecture.md
---

# 架构评审

## Round 1 — 2026-10-27

## 📋 分类架构与数据合同评审

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-ARCH-R1-F1 — P2 — 未匹配/已失去会话的覆盖没有可执行的撤销合同（OPEN）**

位置：architecture.md:139–150、197–202、216；关联 131–135 行及 ux/settings.md:69–72。

- 行为风险：恢复后 unmatched、原会话已不存在的覆盖仍会出现在 Settings 列表，但撤销动作可能被可见会话查找拒绝，形成不能清除的用户记录。实现者必须擅自放宽 not-found 规则或新增协议，才能满足已批准 UX。
- 证据：Settings 明确“原始会话已不存在或恢复后尚未匹配……允许删除该覆盖记录”，不是要求删除源会话。架构也要求保留旧覆盖供逐条撤销，但写入流程要求查询目标后 CAS，并统一规定“目标不可见沿用 session_not_found”。协议只定义按 session ID 的 classify --clear 和 classify list，没有说明 --clear 可独立定位已有覆盖、无需可见会话，或另给覆盖记录撤销 API；Settings 必须通过这些 Go 命令操作。
- 判别场景：core 存有 codex/archived-1 的 auxiliary 覆盖（revision=4、unmatched），sessions 无该 canonical 会话。classify list 能返回该覆盖；Settings 选择撤销。若按 §5 的可见目标规则处理，则 session_not_found；若直接以覆盖键写 tombstone，则绕过了未写明例外的统一规则。当前架构无法唯一决定合法行为、返回 effective/automatic 是否 unavailable，以及重复撤销的 no_change。
- 💡 修复建议：明确把“查询/设置可见会话分类”和“撤销已有用户覆盖记录”分成不同的目标解析规则，可复用 --clear 但必须写清例外。规定 unmatched/会话消失时按覆盖记录身份及 expected_revision/request_id 做 CAS/tombstone；不读回或暴露被 exclude 的源会话内容，不把无会话伪造成自动分类结果。明确不存在覆盖时的幂等结果、并发变化与请求读回语义，并补充 Settings/CLI 对应验收例。无需在本轮实现代码。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 用户持久覆盖与自动派生分开，回执、CAS、提交后权限错误和发布待完成有独立语义。
- 维护适配器限制版本、列白名单和只读边界，不把来源目录/模型名当作用途证据。
- 明确跨数据库读向量、epoch 与 token mismatch 的保守退化；保留旧 wire 字段及 App Group 聚合隐私边界。

### 📝 总结

- Reviewer: Codex，独立评审会话，未参与本架构设计。
- Method: 单评审者全文合同与四份已评审 UX 对照；CodeGraph 定位现有 SetSettings/backup 等路径，再核对相关当前源码。对未匹配覆盖构造最小合同反例；决定性阻塞后停止广泛验证，未运行产品测试。
- Scope: architecture.md 的设计决策及需求/UX 可实现性，不是产品实现或实际 observer 数据验收。未修改产品、测试、配置、原型或架构正文。
- Reviewed state: HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob `7b586840ec6cf60a7a37493441a371d0a333324e`。
- Fingerprint: `fbe4dda0c252bf9de18a04f1b2922485a67955b57a88176c09a9fcfb77349fe2`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: settings.md 的未匹配覆盖删除要求与 architecture §5/§7 直接对照；上述反例是合同演绎，不冒充未实现命令的运行输出。原型中的 archived-1 只是说明场景，不是用户数据。
- Evidence: 当前 SetSettings 在 commit 后执行 secureFiles，可能返回已持久化的权限错误；backup 通过 snapshot API 并对空目标恢复，与文档该部分边界一致。只读核验安装包 worker-service.cjs/SessionStore.js 的 SHA-256，分别匹配文档 `aed0b798e04c40e2c3600e2e5976f4eb470b84c96bc1ea48fea9201157a556f4` / `4ed2bf28cfc01b37589e92f2ace8f243f25dbef0aa84df07d5d1b4140e1907c7`；未访问真实日志、数据库或凭据。哈希一致不证明实际 SDK observer 启用、映射命中或覆盖率。
- Evidence: 记录及矩阵同步后 `python3 scripts/check-docs.py` 通过（包括空白、diff、本地链接、记录结构及文档集检查）。文档集成稿不等于 tasks 分解或 final UX 已通过。
- WorkUnit: `auxiliary-session-classification:architecture.md`；ContentState: `auxiliary-session-classification:arch-r1-fbe4dda0c252bf9d`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-ARCH-R1-F1 为 fail；文档完整性按实际检查结果记录，不因语义 FAIL 虚构结构失败。
- Residual uncertainty: 其余分类/适配器/并发/备份实现仍需独立隔离 fixture 与最终内容验证。本轮停止于确定缺口，不作全部架构细节无缺陷声明。

修复：auxiliary-session-classification / reviews/architecture.md / ASC-ARCH-R1-F1

## Repair candidate — 2026-10-27（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-ARCH-R1-F1，未委派，未修改产品、测试、配置或 UX 标本。
- ASC-ARCH-R1-F1 → repaired in candidate，待独立复评确认关闭。§5 分开查询/--set 的可见会话解析
  与 --clear 的 core 覆盖键解析；§7 明确复用 --clear 的例外及列表提供 revision，不引入源读取权限。
- --clear 要求明确 client/session_id，可处理 unmatched、源消失、purge-index 后或已 exclude 的目标。
  有效覆盖写 tombstone 并加版本；已有 tombstone 或从未存在记录在 CAS 成功后 no_change。
  core 读取失败不等于没有覆盖；新请求先 CAS 再判幂等；相同请求重放原回执，不删除其后新建的覆盖。
- clear 回执 automatic/effective=null、classification_available=false、reason=not_queried，
  不伪造无会话的自动分类。Settings 另行查询仍可见会话或刷新覆盖列表，旧回执不能覆盖更高版本。
  未确认提交仍按 request_id 读回，不以 session_not_found 推断撤销成功。
- 判别场景推演：archived-1/unmatched/revision4 且 sessions 无记录，clear expected4/requestA
  得到 tombstone5；新 requestB/expected5 为 no_change5；他人 set6 后重放 requestA 仅返回原5回执，
  不清除6。另列不存在键、exclude、并发修改和提交结果未知，共七条 Settings/CLI 验收案例。
  这是合同一致性推演，不是未实现命令的运行测试。
- Candidate identity：HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob
  `5b5a3b5694827900a9380e3879058a6baca4919a`。
- 验证：与 Settings 的“未匹配覆盖可逐条撤销”要求对照，并核对全文 --clear、session_not_found、
  no_change、请求读回及 UI 版本约束的一致性。`python3 scripts/check-docs.py` 通过；未运行产品测试。
- 原 Round 1 FAIL/OPEN 和原目标 FAILED 保留，新候选不自发声明 Review PASS 或 CEv1 VERIFIED。
  tasks.md 仅记录待复评，最终 UX 核对仍等待架构独立评审通过。

## Round 2 — 2026-10-27

## 📋 架构修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-ARCH-R1-F1 — CLOSED**：§5 区分可见会话查询/设置与 core 覆盖撤销。
--clear 要求明确 client/session_id，以 core 记录或 tombstone 为目标，无 sessions/源日志前置条件；
§7 的命令例外、列表键/revision 和 §9 七条验收例一致，Settings 的 unmatched 删除要求已有协议落点。

### 📝 总结

- Reviewer: Codex，原独立评审会话，未参与修复。
- Method: 单评审者逐 finding 核对 §5/§7/§9、原 Settings 要求和原反例；不委派、不执行未实现命令，不修改产品、测试、配置或 UX。
- Scope: ASC-ARCH-R1-F1 及其身份解析、CAS、幂等/回执、错误和 UI 更新语义。未变更的源码事实沿用 Round 1 核对，HEAD 未变且无产品脏文件；不将本轮文档复评当作实现验收。
- Reviewed state: HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；architecture blob `5b5a3b5694827900a9380e3879058a6baca4919a`。
- Fingerprint: `54920214c6ee71310902644ff66ff9a4ed1a02e2a433c57a6d4b714b998d225b`；SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: 原 archived-1/unmatched/revision4、sessions 无记录场景，现在以 clear expected4/requestA 写 tombstone5；automatic/effective=null、classification_available=false/not_queried，不伪造自动分类。被 exclude 的源同样可撤销用户记录，但不解除 exclude 或读取私有内容。
- Evidence: 新请求 expected5 重复 clear 得 no_change5；从未存在键 expected0 得 no_change0；core 读取错误不等于不存在。读取后记录改到5时 expected4 冲突，即使目标已是 tombstone 也先 CAS。旧 requestA 在他人 set6 后仅重放原5回执、不改6；UI 也不能用旧回执抹掉已知高版本。这些规则形成一致的合同链。
- Evidence: 提交结果未知按 request_id 查证，未确认前不删除列表项；无会话时保持分类不可用，可见会话需另行查询当前 automatic/effective。以上是设计合同推演，不是运行测试；七场景须落实为后续 L2/L3 证据。
- WorkUnit: `auxiliary-session-classification:architecture.md`；ContentState: `auxiliary-session-classification:arch-r2-54920214c6ee7131`。
- Completion gate: VERIFIED
- Gate scope: 本架构文档的独立复评和完整性检查；不认证维护适配器实际覆盖、跨库一致性实现、迁移/恢复或性能。
- Hierarchy: 架构通过后四份 UX 仍需 final-surface reconciliation，tasks.md 尚未分解/评审；本轮不跨越主题完成边界，不创建实现任务。
- Evidence: 记录与矩阵同步后的 `python3 scripts/check-docs.py` 通过，涵盖空白、diff、本地链接、评审记录及文档集合结构；不等于实现分解或最终 UX 的语义通过。
- Disposition: ASC-ARCH-R1-F1 CLOSED；旧 Round 1 FAIL 和旧目标 FAILED 保留，当前结果只绑定新候选。
- Residual uncertainty: 真实维护格式的隔离端到端验收、原生交互/发布和最终 scan-performance 集成仍未执行，必须由实现及其证据门覆盖。

Task checkpoint：ad-auxiliary-session-classification-doc-arch；content_state `54920214c6ee71310902644ff66ff9a4ed1a02e2a433c57a6d4b714b998d225b`；gate VERIFIED。
提交建议：经授权提交 architecture.md、对应评审记录及 tasks.md 的架构状态；不包含产品实现或最终 UX 修改。
推送建议：经授权提交并核验提交对象、Codex trailer 与 SSH 签名后，推送 feature/auxiliary-session-classification；remote/upstream 尚未解析。

设计：auxiliary-session-classification / ux/cli.md（final-surface reconciliation）
