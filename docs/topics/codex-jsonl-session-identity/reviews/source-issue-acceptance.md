---
status: active
topic: codex-jsonl-session-identity
subject: source-issue-acceptance
---

# 来源问题验收评审

## Round 1 — 2026-11-25，10a 验收 harness 首评

评审结论：FAIL

完成门禁：NOT_VERIFIED

Task 10 拆为 10a 和 10b。10a 提供隔离验收 harness、真实旧二进制围栏矩阵（OldBinaryFence），以及只依赖已提交工作的 A1–A5 观测；依赖未提交 Task 8 的子测试一律显式标为 PENDING。作者为 Claude `claude-opus-5-5` high，基线 `6d255ed`。独立冷评审为 AGY `gemini-4-neon` high，请求号 `v075-e21-cjsi-rebuild-task10a-review-45d788ec`。

- **CJSI-SIA-10A-R1-F1（P1）** 逻辑摘要与通配允许列表会掩盖对私有索引的写入：变异 R4c 能整体通过。
- **CJSI-SIA-10A-R1-F2（P2）** 构建脚本拒绝 /tmp 的检查可以被绕过。
- **CJSI-SIA-10A-R1-F3（P2）** 泄漏 `/tmp/ad-s-*` socket 目录。
- **CJSI-SIA-10A-R1-F4（P2）** M3 与原行为不等价；缺 CodexForeign；缺右锚断言。
- **CJSI-SIA-10A-R1-F5（P2）** overlay 升级器重复实现迁移步骤。
- **CJSI-SIA-10A-R1-F6（P2）** 缺少 classify、scope、`show --tokens` 入口；A5 依赖原始 INSERT 来证明。
- **CJSI-SIA-10A-R1-F7（P3）** 围栏文件的权限豁免。
- **CJSI-SIA-10A-R1-F8（P3）** 断言需要收紧；10b 的 LaunchServices 需要隔离。

## Round 2 — 2026-11-25，10a R1 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R1 候选 `669605e4`，6 个路径。新增 sandbox-exec 宿主探测守卫，F5 待 Task 2b 提供可挂钩迁移后再处理。评审为 OpenCode `openai/gpt-6.2-sol` high。

- CJSI-SIA-10A-R1-F4、F6、F7 closed。
- CJSI-SIA-10A-R1-F1、F2、F3、F8 由以下发现取代。
- **CJSI-SIA-10A-R2-F1（P1）** 允许列表会隐藏 `.scan-receipts-*` 和锁文件写入（变异 R6）。
- **CJSI-SIA-10A-R2-F2（P2）** /tmp 的物理别名。
- **CJSI-SIA-10A-R2-F3（P2）** `installation.json` 没有放进 sandbox。
- **CJSI-SIA-10A-R2-F4（P2）** socket 清理的归属。
- **CJSI-SIA-10A-R2-F5（P3）** phase 与 head 的断言。

## Round 3 — 2026-11-25，10a R2 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R2 候选 `0ef3e328`，17 个变异全部被杀死。评审为 Codex `gpt-6.2-sol` high。

- CJSI-SIA-10A-R2-F2 到 F5 closed；R2-F1 由 R3-F1、R3-F2 取代。
- **CJSI-SIA-10A-R3-F1（P1）** quiesce 在快照前回收过期的 `scan.lock`（变异 R10）。
- **CJSI-SIA-10A-R3-F2（P1）** 整轮回执快照漏掉 journal 删除（变异 R11）。
- **CJSI-SIA-10A-R3-F3（P2）** `~/.claude.json` 没有放进 sandbox。

## Round 4 — 2026-11-25，10a R3 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R3 候选 `815ec86f`，修复轮 3/3，22 个变异全部被杀死。评审为 Codex `gpt-6.2-sol` high，在同一评审线程进行。评审运行末尾被 OpenAI cyber_policy 截断，结论依据由主控核对日志确认。

- CJSI-SIA-10A-R3-F1、F2、F3 closed：R1–R11、R4b、R4c、R6、M1–M6 全部被杀死。
- **CJSI-SIA-10A-R4-F1（P2）** 锁文件为 FIFO 时，观测者会一直挂起到测试超时（R12）。
- **CJSI-SIA-10A-R4-F2（P2）** 回执 journal 被删除后又以相同内容重建，检测不到（R13，exit 0）。

## Round 5 — 2026-11-26，10a R4 复评

评审结论：FAIL

完成门禁：NOT_VERIFIED

R4 候选 `86d77cd4`（用户批准超出 3 轮上限的第 4 轮），6 个路径，24 个变异全部被杀死，没有一个靠超时杀死。评审为 Codex `gpt-6.2-sol` high。

- CJSI-SIA-10A-R4-F1 closed：非常规锁文件改为非阻塞的有界读取。
- CJSI-SIA-10A-R4-F2 closed：回执按 inode 和内容一起比较。
- **CJSI-SIA-10A-R5-F1（P2）** `cjsiAccStrictJSON` 用 `Decoder.More()` 判断尾随内容，会放过多出的 `]` 或 `}`，而生产的 `json.Unmarshal` 会拒绝整个 journal。

## Round 6 — 2026-11-26，10a R5 窄复评

评审结论：PASS

完成门禁：NOT_VERIFIED

R5 候选 `dfbb4fb055269529e3d58c532032a20c6091e28bfc13fb45ab19e0def958f5c3`（用户批准第 5 轮）。改为解码一次之后再解码一次，必须得到 `io.EOF`。评审为 Codex `gpt-6.2-sol` high。

- CJSI-SIA-10A-R5-F1 closed：独立探针对 65,815 个尾随输入核对验收解析器与 `json.Unmarshal`，结论全部一致；M7 被杀死。

F5 仍然要等 Task 2b 提交，所以本轮门禁不关闭。

## Round 7 — 2026-11-26，10a R6 窄复评（F5 切换）

评审结论：FAIL

完成门禁：NOT_VERIFIED

R6 候选 `02ff523039b3ce5eab622bb774c203c40fcb6b2f058a9cea92b39ca9d1d3f323`，基线改为已提交的 `0d91b76`。评审为 AGY `gemini-4-neon` high（窄复评档位经 Decisions 评估）。

- CJSI-SIA-10A-R1-F5 closed：完整升级调用 `store.MigrateState`；中断模式调用已提交的 `migrateCJSIManagedRootWithReady`，暂停只注入在 beforeReady 钩子里；harness 中已没有自己拼装的迁移步骤。
- 断言没有削弱：落盘阶段计数、升级器遗留锁记录、24 个入口矩阵都不变。
- M8、M9 和评审自写的 M10 都被行为断言杀死。带外部二进制的验收 80 通过、11 跳过、0 失败，宿主 LaunchServices 命中为 0。
- **CJSI-SIA-10A-R6-F1（P3）** A1CLIReadSurfaces 的 pending 原因文字仍写着旧基线 `6d255ed`。

## Round 8 — 2026-11-26，10a R7 主控核对

评审结论：PASS

完成门禁：NOT_VERIFIED

R7 只改文字。累计补丁 `56f74784caf34bb42595ca96514d8f7e2fb24ffbc01fe44e59e835c4f2637317` 共 6 个路径，基线 `0d91b76`；增量 `dd902d787bd0c9c9e61425168a8f4255cbc7e31f06d7c12a64dd22c1889904b0` 只涉及 1 个路径。主控逐行核对了增量：只改了文件头注释和 A1CLIReadSurfaces 的 pending 原因，逻辑、断言和跳过集合都没有变化，所以没有再派冷复评。

- CJSI-SIA-10A-R6-F1 closed：pending 原因改为依赖基线 `0d91b76` 上尚未提交的 2b-1/2b-2a；6 个路径中已不再出现 `6d255ed`。
- 验证：带外部二进制的定向验收 80 通过、11 跳过、0 失败；gofmt、`git diff --check`、补丁重放均退出 0。

10a 完成。仍为 pending 的 11 个子测试都依赖 Task 8 的 2b-1/2b-2a，由 10b 在 Task 8 提交后启用；Task 10 的门禁与最终 L3 一起关闭，tasks.md 中 Task 10 这一行暂不勾选。
