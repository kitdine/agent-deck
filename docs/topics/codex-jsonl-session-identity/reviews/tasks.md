# JSONL 任务分解评审

## Round 1 — 2026-11-18

### 📋 Codex JSONL 子会话身份与用量归属任务分解

📊 综合评分：9.5/10

✅ 评审结论：PASS

### 🔴 必须修复

无。

### 🟡 其他改进

无。

### 🟢 已核对

- 10 个实现锚点划分来源证明、状态围栏、账本与 OTel、索引与读面、同代发布及恢复和验收。C1–C4 明确接口冻结、无环依赖和单写者。共享 `usage.go`、`main.go` 和迁移文件均有确定的归属及交接。
- A1–A7 的 14 行映射覆盖原缺陷，包含缺源和可恢复旧账、子线程与直接父线程、用量金额对账、数据最小化。详情顺序保留 `AXBC` 中间插入和分页断言，候选持久化覆盖两种同步屏障、旧票读取、CAS 和崩溃恢复。
- 复用了需求及架构 Round 3 的已验证合同。保留明文 `.jsonl` 与现有公开读面。隔离旧、新二进制构建和综合故障验收由锚点 10 收口。目标 main 同步和版本分配各有主控检查点。

### 📝 小结

评审对象为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781`、tasks.md blob `f80d9244a8952d82ac90edec187a1eea54790e69`、指纹 `ef53f628821b1d751d1ea54fc25568900880c554297966f4ce7cc12839b109f7`。已批准输入为需求 blob `f5ebb617dd1acf54f3728712adc5f2f518d343e2` 与架构 blob `a4f258224d7380cfbf7b018662793f8e3245b6af`。T3 确认独立 Antigravity 评审完成并回报请求选项 `thought_level=medium`。Worker 自报 Gemini 4 Argon Medium，T3 未独立回报实际模型。

主控核对了来源验收、依赖与代码写入归属、14 行断言和旧二进制入口。正式 PASS 后勾选 tasks.md 的 Review 格。反转这一格可复现原评审 blob。最终 blob 为 `20bede4a595555dcdc88b0559b63fe38190d6d87`，指纹为 `edc0c9f9d58c84d8ba325d0a60493e3a8d4c5d488c183478b1cdbf768d8810f5`。本轮执行文档及脚本语法检查。L2/L3 产品测试在实现期取得。

完成门禁：VERIFIED

精确目标 CEv1 通过 4/4，覆盖验收映射、任务分解、格式检查和独立评审。主控读回 14 个节点、17 条关系，未见缺项、失效或未决影响。独立评审证据按唯一 Review 格的适用性核对复用于最终指纹。

任务检查点 `ad-codex-jsonl-session-identity-doc-tasks-design`，目标状态 `codex-jsonl-session-identity:tasks.md:state:edc0c9f9d58c84d8ba325d0a60493e3a8d4c5d488c183478b1cdbf768d8810f5`。提交建议为本文件与已评审任务分解单独提交。推送建议为随已授权的来源 issue 候选交付。

### 开发基线对齐 — 2026-11-18

经用户授权，本地 `feature/codex-jsonl-session-identity` 由 `4ac56db32e13c4e925b648c620ba76fc588a7781` 快进到目标 `a224b0b824d2cad451d3bd83153832d65506f0fe`。两端 Git tree 分别为 `3273cc824d40916b010e53152ca292b1a617dcc6` 和 `3fe7ac41f894e70a2b00b3d0febdf6ef59ba29f5`。主控核对了 6 个本地改动路径及原暂存状态。主线的 `assistant_turns` 预聚合与 watch 取消及释放路径已纳入，符合架构中预先核对的目标。core schema 仍为 35。订阅主题先使用 36。本主题先开展不依赖 core 编号的 `native-source-proof`，待目标包含前置迁移后再分配自己的 core 编号。

文档 blob 与对应的既有评审一致。主控按实际影响记录 preserves 和同态 roll-up。当前文档证据状态如下，每项 VERIFIED 4/4。

- requirements.md `codex-jsonl-session-identity:requirements.md:state:3871fba70bb539785ae12edc90e6d62cadc7f66baa3318930cf9eec19fb9ddf4`
- architecture.md `codex-jsonl-session-identity:architecture.md:state:f9829674e56ffea52e197164e02c82114a62f612c0ee8a9ee796e2b40226686d`
- tasks.md `codex-jsonl-session-identity:tasks.md:state:320733c9c4bdc628410b25141b519d17e6fc929d9a48338909fea9151fb7de0a`

### C1 捕获与布局接口冻结 — 2026-11-18

主控匹配 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、已批准的任务文档和纯解析候选 `1553ed3ecc256dc7d4451805531a7fd5e600420ec143429f3fa3689731a49c02`，核对了现有会话打开路径、布局围栏及读取边界。该冻结明确代码交接。C1 的迁移、完整重放和跨域验收保持待完成。

- **来源捕获归任务 1。** Codex 专用 `ReadCodexJSONL` 显式接受 context、已验证起点、独占捕获上界及正的单行字节上限。上限含终止换行，由调用方以实际扫描范围给出。这次实现使用已约定的 `CodexCaptureBounds{Start, End, MaxLineBytes}` 内部类型。通用 reader 沿用现有语义。
- 遇超长行，读取端只保留有界缓冲并丢弃内容。读到完整换行才发出无内容的内部超长边界，禁止从该行确认身份、配对或计量。预算用尽而未结束的行保留在同一 `CompleteEnd` 之后，下次重新读取。边界前的短记录和后续独立证据按各自已证范围处理。可见正文仍走独立短生命周期输入。
- **检查点分开绑定范围、待决和撤回。** `CompleteEnd` 是完整行边界，`ReplayFrom` 只标追加可补证的范围。归约输入必须是连续、同代且已验证的完整前缀。换代、前缀变化和迟到反证按已发表证明另行核对并撤回，仍走保护与票据提交。持久格式仅保留必要的非正文证明与范围，含代际和 parser 版本。现有全前缀 reducer 留作对照，生产级有界工作集另行接线。
- **状态布局归任务 2。** 两个既有会话打开入口和调用方使用同一受管 selected locator。读取按已选状态、布局版本、epoch 及实际文件身份核验，区分从未建立、已 purge、暂存和损坏。旧固定名 `sessions.sqlite3` 保持物理围栏。排空旧连接后才封存并换布局；校验并提交选中前，普通读者不接触暂存。结果不明按协调头读回，迁移与 Restore 的具体版本和故障验收在持久实现落地后执行。
