# 账号与订阅适配评审

## Round 1 — 2026-11-18

评审结论：PASS

独立评审限 Claude 与共享适配的 11 文件。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，候选 `ed3d7cb875c63cf20051718f7311f8704387e8fd738c7163c6c9e59ae2955093`。主控重算适配指纹和五份批准文档，33 个冻结存储路径继续匹配其已验收候选。

Claude Sonnet 5.5 独立核对命令解析和共享执行环境、各自截止时刻、进程收尾、auth-status 保守分类、套餐脱敏和 HMAC 密钥创建。评审建议限定交接范围 PASS。T3 请求档位为 high，实际推理档位未独立观察。评审方执行聚焦 quota、竞态及 vet 检查。主控核对作者的 quota 全包、聚焦竞态、隐私及空白结果。已批准夹具只有登录状态，登出退出码与个人组织形状待原生验收。

- **SAI-IA-CLAUDE-R1-F1 · 额外模型行。** A9 和 P2 要求覆盖 `Current week (<model>)`。现有噪声用例以 `Current something else` 开头。主控核对本项缺证，补回归以保证模型行不修改有效窗口，也不能替代缺失的全模型周窗口。

**SAI-IA-CLAUDE-R1-F1 CLOSED。** `internal/quota/claude_test.go` 新增 `Current week (Sonnet)` 输入，确认两个有效窗口逐字段一致。缺少全模型周行时，只有 Sonnet 周行会得到 `missing_line` 和零窗口。主控核对了实际 diff、测试执行结果和同一 HEAD 下 12 文件候选 `ff02c268fc0e904db0d6eed7bcc8547b9eb464441cb69cfdb1646e6ee752b8b7`。`scripts/run-go-test.sh ./internal/quota -run '^TestParseClaudeProse' -count=1` 及限定路径空白检查通过。原 11 个文件和存储候选保持相同 blob，沿用对应评审及验证结果。

完成门禁：NOT_VERIFIED

完整 `identity-adapters` 锚点处于开发中。Codex 专属适配和运行时接线依各自约定推进。

## Round 2 — 2026-11-18

评审结论：FAIL

完成门禁：NOT_VERIFIED

本轮覆盖完整 Task 2。独立 Claude Sonnet 5.5 核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、联合候选 `9ed8e16b0cdbc35116b6b72fdae3de00f1477cebc06c12910689eff9f395e216` 和 Codex 五文件切片 `1c990f6def3bc7e85ee19650883947fa1595df65d4783f60860abbbd599fa41e`。T3 配置为 high，实际推理档位未独立观察。评审方使用隔离的 `HOME` 与 `CODEX_HOME` 重建 codex-cli 0.169.1 schema，三个来源摘要与夹具一致。评审方复跑聚焦竞态、vet 和隐私检查，并以合成假进程探查两侧应答和退出的顺序。

- **SAI-IA-CODEX-R1-F1 · High · A1.3。** `internal/quota/codex_identity.go` 将进程截止导致的 `Wait` 失败写回两个结果。身份已成功、额度不应答时，身份被改成 `probe_failed`，调度器会因错误的失败尝试推进身份退避。主控在原实现上运行新增 `TestCodexIdentitySurvivesUsageDeadline`，额度得到预期 deadline，身份却丢失，测试退出 1。应保留已经应答的身份，同时继续拒绝自行异常退出或被信号终止的进程输出。
- **SAI-IA-CODEX-R1-N1 · 分派器。** 服务端带 `method` 的请求复用等待中的 ID 时会被当作应答。主控新增 `server_request_shares_ID` 用例，确认原实现得到 `parse_failed`。

评审另外提出将当前 schema 夹具接入 Go 测试及扩展套餐投影断言的建议。独立重建的 schema 支持 `certified_common_key: null`，现有合成测试覆盖共同键缺失、冲突、精确匹配和隐私筛查。运行时调度由 Task 4 承接。Claude 登出退出码、个人组织形状按 Task 7 原生矩阵验收。

### 有界修复

- **SAI-IA-CODEX-R1-F1 → 已修复，待独立复评。** 记录本地到达进程截止时发出的终止，并用 `Wait` 的进程退出状态核对。额度请求还未应答时，本地截止只使额度失败，先前已完成的身份结果保留。自行非零退出或收到信号的进程输出仍被丢弃。新增用例实测身份为 `identified`、额度为 `probe_failed`。
- **SAI-IA-CODEX-R1-N1 → 已修复，待独立复评。** 单 reader 先识别携带 `method` 的服务端消息，随后按数字 ID 投递应答。带相同 ID 的服务端请求与真正的身份应答按序到达时，身份与额度测试均通过。

修复只涉及 `internal/quota/codex_identity.go` 和 `internal/quota/codex_identity_test.go`。联合 58 路径候选 `bc6df6fccfbab088b9ccb803bee31a2994c888db1704445e7987e962dc5465ef`，Codex 五文件候选 `e5d5f0af5b6550092522a42b2613308d32e2a79355c928ec4b433f7568d746d5`。主控以两份原 blob 代回重算原联合指纹，确认其余路径一致。

`scripts/run-go-test.sh -race ./internal/quota -run 'Test(Codex|ProbeCodex|ParseCodex|QuotaPackagesReadNoCredential)' -count=1`、`go vet -mod=vendor ./internal/quota`、`make check-privacy` 通过。首次全包测试与 race 并行时，Claude 的 1 秒超长输出夹具返回 `probe_failed`。该夹具单跑三次通过，后续 `scripts/run-go-test.sh ./internal/quota -count=1 -p 1` 全包通过。

## Round 3 — 2026-11-18

复评结论：PASS

完成门禁：VERIFIED

独立 Claude Sonnet 5.5 对完整 Task 2 进行只读复评。工作树 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，联合 58 路径候选 `bc6df6fccfbab088b9ccb803bee31a2994c888db1704445e7987e962dc5465ef`，Codex 五文件切片 `e5d5f0af5b6550092522a42b2613308d32e2a79355c928ec4b433f7568d746d5`。主控重算并提取 Task 2 原始路径归属，16 文件指纹为 `5a612fae6ad35fd2271d1d5dccd2b5fbf095184fe6a7e4150e088f2f467a4fd5`。T3 配置 high，实际推理档位未独立观察。

- **SAI-IA-CODEX-R1-F1 CLOSED。** 真正的本地进程截止与自行非零退出、终止信号分别验证。额度沉默到本地截止时保留已应答的身份。独立探针补验自行 `exit 7`、自行 SIGKILL、干净退出、调用者取消、畸形应答后挂起及仅查额度的旧入口。
- **SAI-IA-CODEX-R1-N1 CLOSED。** ID 3 与 ID 2 的服务端请求均不会抢占应答。额外探针覆盖带字符串 ID 的服务端消息。

独立执行定向 race、三轮 Codex 身份矩阵、串行 quota 全包、vet、格式与隐私检查均通过。主控核对相同候选上的失败先行用例与修复后结果。Claude 夹具、共享脱敏密钥、官方 Codex schema 摘要及套餐边界沿用已核对的证据。全应答后本地 Wait 截止的行为沿用原有拒绝语义。调度与原生验收分别由 Task 4 和 Task 7 承接。

主控追加 Claude adapter、prose、密钥与筛查的定向 race，退出 0。CEv1 目标 `subscription-account-identity:identity-adapters:state:5a612fae6ad35fd2271d1d5dccd2b5fbf095184fe6a7e4150e088f2f467a4fd5` 的六项证据通过，涵盖 vendor 夹具、Codex 传输、Claude adapter、身份套餐投影、隐私密钥与独立评审。

## Round 4 — 2026-11-19，按来源拆分的交付复评

复评结论：PASS

完成门禁：VERIFIED

独立 Claude Sonnet 5.5 复评在签名 Task 1 基线 `815be9d2cf3b8afb5e208fc8eb73c2d49037a965` 上分别核对 Claude 和共享候选的 12 个路径，以及叠加 Codex 后的 16 个路径。候选指纹分别为 `a5a83ea5e84135df1872910a948c2d84a2309e2ad6ede8d2f3c774950e9f6701` 和 `70bd6b77a9ce6350ba0ab42163b76d993a947134084c39a3d58a52dcd312a8bc`。T3 请求 `effort=high`，实际推理档位未独立观察。

主控复算两个清单及逐路径 blob。与此前的相应切片相比，生产、依赖和厂商夹具一致。唯一替换的路径是 `internal/quota/claude_identity_test.go`，旧正文 `95186ce9395c73a71ec007ca8d650255829194bf` 不可用，采用 `4fc9f980a729524dc8bcf1ef5aae9638d83f7a8c`。替换版覆盖分类、筛查、进程异常、`WaitDelay` 和身份超时后的额度读取。它只调用本任务适配器和 Task 1 接口。载体保持旧 Codex 用量探测。叠加的五个路径零模糊重放，逐字节匹配 Round 3 的 Codex 切片。

两个候选的 quota 全包、各自相关的定向 race、vet、隐私、格式及空白检查均通过。两个候选分别通过 desktop 和 cmd 的额度与旧库消费路径测试。载体涵盖旧 `ProbeCodex` 回归，叠加另验证单 reader、独立请求结果、禁用令牌刷新和 `prolite` 显示。使用隔离的合成进程和已获准的夹具。

完整 Task 2 的门禁对 16 路径叠加候选 `subscription-account-identity:identity-adapters:state:70bd6b77a9ce6350ba0ab42163b76d993a947134084c39a3d58a52dcd312a8bc` 通过 6/6。12 路径载体单独通过其构建、旧入口和 Claude 行为验证。Claude 调用者取消缺少直接进程夹具，交 Task 4 的有界刷新路径及 Task 7 补验。真实登出退出码、个人组织形状和可用的共同键证据仍按 Task 7 的授权范围核对。
