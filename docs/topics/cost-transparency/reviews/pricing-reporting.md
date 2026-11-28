---
status: active
topic: cost-transparency
subject: pricing-reporting
---

# pricing-reporting 评审记录

## Round 1 — 2026-11-19

### 📋 计价报告实现评审

📊 总分 7/10

✅ Verdict: FAIL

#### 🔴 必须修复

- `CT-REP-R1-F1` · 高 · 增量缓存的计价读快照早于分区 Begin。写入一个仍 dirty 的后续分区只合并失效标记。旧快照漏掉该写入，Begin 后的发布校验也无法识别遗漏。独立隔离探针中重建和缓存各读 600 条，全量读 601 条。主控用两分区触发器固定同一窗口，重建和缓存均只读 2 条、30 个输入 token，全量读 3 条、60 个输入 token。提前取得每个待重建分区的序号，再建立读取事件的同一快照，保留最终代次和元数据检查。处置为开放。
- `CT-REP-R1-F2` · 中 · 有证据的空时段报告 `unavailable`，菜单栏的空 bucket 也显示金额不可用。主控在空库复现 summary、stats 和原生 payload 的 API 金额不可用。空调用集合提供完整零 API 金额，渠道估算继续可空，零总额不产生峰值或占比。补齐普通菜单栏空时段和 bucket 的隔离验证。处置为开放。

#### 🟡 建议改进

评审记录了 CLI 标本文案的细节差异，以及用量发布失败时保留上一完整版本的行为。两项属于本轮可接受范围。

#### 🟢 已满足的部分

调用集合、半开窗口、快照读取一致性、额度接口、期间覆盖提示、可比金额、来源与参考、旧 wire 与 App Group v1 投影兼容、`price list --at` 和 CLI 手册得到核对。

#### 📝 总结

状态为 HEAD `dfaa5e1a7dfc1c16af4c9d5e323ffe3c33fd89fa`、树 `13470c80fb8902034893cc57a5697d3280c96e1b`，40 路径指纹 `b9afc9b985d6d922a9da1a9b122eafb5379b076a9101dfec938fab4dc386f4f9`。Claude Code Opus 5.5 冷上下文只读评审 `v080-pricing-reporting-review-e7-a1`，评审者报告有效推理档 low，执行应用范围 epoch 8。主控核对候选，复现两项缺陷。原候选的定向 Go、竞态、CLI 数据和 149 项普通原生夹具为其通过部分提供证据。

当前验收以操作者 epoch 8 为准。VoiceOver 及其他辅助功能测试、需要更改本机 macOS 设置的测试均排除。常规原生验证使用现有设置下的隔离夹具。

完成门禁：NOT_VERIFIED

该候选的 CEv1 查询返回 `missing_work_unit`。修订两条发现后复评新指纹。

## Round 2 — 2026-11-19

### 📋 计价报告复评

📊 总分 9.5/10

✅ Verdict: PASS

#### 🔴 必须修复

两项发现均已关闭。

- `CT-REP-R1-F1` 已关闭。待重建分区全部取得代次后再统一打开事件快照。初读和重开读的计价元数据代次必须一致，发布与最终代次校验仍在。推进缓存算法版本以退役旧缓存。两分区回归验证重建、缓存和完整读取都得到 3 条事件、60 个输入 token，第二次增量读取确实复用了缓存。
- `CT-REP-R1-F2` 已关闭。成功读取的空时段报告完整 USD 零，渠道估算为空，零总额的峰值和占比为空。常规 Swift 夹具保留连续小时序列，验证显式为空及自动补出的 24 个小时都显示零金额。

#### 🟡 建议改进

本轮无新增发现。其余支持条件与既有产品承诺一致。

#### 🟢 已满足的部分

复评确认快照中止及缓存复用路径、历史金额的新鲜度、正常 CLI 字段和菜单栏空状态。现有 App Group v1 投影回放通过。继续采用 epoch 8 的用户验收边界，VoiceOver 等辅助功能测试及需要更改本机 macOS 设置的测试为排除项。

#### 📝 总结

HEAD `dfaa5e1a7dfc1c16af4c9d5e323ffe3c33fd89fa`，树 `13470c80fb8902034893cc57a5697d3280c96e1b`，40 路径指纹 `0c1aba9707763097901b24adc2667140dc32a2e80177120b98fd57afb7d8a94f`。指纹按 `HEAD<TAB><sha><LF>`、`TREE<TAB><sha><LF>` 与按路径字节序排列的 `<blob><TAB><path><LF>` 拼接后取 SHA-256。主控复算一致，`MenuBarChromeTests.swift` 的完整 blob 为 `41e2cfce135ec5182a9ba6c894e75b235d9508db`，以此更正简报的单文件转录。复评为 `v080-pricing-reporting-rereview-e8-a3`，Claude Code Opus 5.5 冷上下文只读，评审者报告实际推理档 low。先前额度退出发生在得出结论前，未另计评审轮。

主控核对 Go usage 与 desktop 测试、定向竞态、CLI 合成快照、构建、vet 和结构检查。最后的普通原生空时段用例在项目认可的隔离目录通过，1 项、零失败、零跳过。复用同一生产 blob 的 280/420 中英文菜单栏渲染和 App Group v1 回放。复评者沿用上述验证，并核对并发写入、代次和缓存有效性。

完成门禁：VERIFIED

CEv1 WorkUnit `cost-transparency:pricing-reporting`，ContentState `cost-transparency:pricing-reporting:state:0c1aba9707763097901b24adc2667140dc32a2e80177120b98fd57afb7d8a94f`。三项必要证据均适用，缺项、失效与未决影响为空。

#### 任务检查点

本地签名提交包含 40 个已评审的实现、夹具、测试和 CLI 手册文件，以及本评审记录和任务矩阵的任务 4 状态。提交并核对证据后，按实际组合基线交接额度采样的正式迁移、生产命令和原生兼容。

#### 计价工作树的 CLI 金标补验（2026-11-19）

主控在签名提交 `d772a79edb20dc8d7a041a30cec24bb6937b0488` 上复现 `TestIsolatedEndToEndFlow` 的旧空时段结构差异，并在计价工作树用端到端生产者重生成 GUI JSON 金标。仅 `usage.stats` 和 `desktop.snapshot` 的成功结构更新，空时段使用既有完整零金额输出。这两部分与额度采样 R2 独立复评核对通过的金标完全相同。

金标 `cmd/agentdeck/testdata/phase7/gui-json-contract.json` 的 Git blob 为 `cdbe44f8056fd2e0b20eea800645c77fd6c1f34e`。在现有工作树以 Go 1.27.1 运行 `TestIsolatedEndToEndFlow`、`TestGUIJSONContractFixture` 和 `TestEveryLeafSyntaxErrorUsesStableJSON`，三项通过。原生与发布路径的证据仍绑定相同实现。

本地内容指纹 `bd46a72e2d1a74d8e7d18cf06062a8f06039400ff6d23bf5e20e1a64ef8348a4` 对应的 `cost-transparency:pricing-reporting` 门禁为 `VERIFIED 3/3`，`cost-transparency` 专题门禁为 `VERIFIED 1/1`。金标和本补验记录形成待单独授权的本地提交范围。
