---
status: active
topic: subscription-account-identity
subject: quota-refresh-publication
---

# 刷新与发布评审

## Round 1 — 2026-11-18

评审结论：FAIL

完成门禁：NOT_VERIFIED

本轮评审范围为 Task 4 的首个 Claude／共享 Go 刷新切片。独立 Claude Sonnet 5.5 核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、67 路径候选 `1cecfd7693888065d165f3af913f69deced9903c4608bd323fde82a1f7657909` 和 12 文件切片 `d458a026e31a3d998780195d0a51c417175e478e80e468a88d934ac41feab78e`。T3 配置为 high。评审以 Go CLI 加假 Claude 测试和只读 overlay 核对目录对账、周期与世代顺序、两侧门禁、退避、闸门恢复、旧 Codex usage、修剪与 ACK。独立运行的 quota 和 desktop race、CLI 聚焦测试和 vet 通过。

- **SAI-QRP-CLAUDE-R1-F1 · Medium · 所有权入口清单。** `desktop.Service.RefreshQuotaCycle` 新增了带 `root` 参数的导出入口。`TestOwnershipPackageEntrypointInventory` 要求显式分类。主控在原候选使用 `scripts/run-go-test.sh -tags agentdeck_acceptance ./cmd/agentdeck -run '^TestOwnershipPackageEntrypointInventory$' -count=1` 复现，唯一未覆盖项是该方法。

### 组合验收观察

O1 的慢身份探针在 22.8 秒的合成上下文中使本轮跳过 usage。A5.6 允许这个已失效、闸门关闭的中间态，生产身份预算为 4.5 秒。组合验收覆盖实际预算与退出次序。

O2 的公开对账入口用于原 Task 3 测试。实际 helper 使用打开 core 时绑定的物理目录，新的对账证据由目录替换测试提供。O3 的进程内在途退场前可以修复 epoch 镜像，探针证明没有启动 Claude 探测。

O4 依架构术语表核对。Linux 采用 `CLOCK_BOOTTIME`，macOS 采用 `CLOCK_MONOTONIC`。刷新与状态栏捕获共用各平台的时钟来源。

对账、修剪和真实 Go helper 路由的变更纳入 Task 4 组合验收。Task 3 原验收绑定原候选。

### 定向修复

**SAI-QRP-CLAUDE-R1-F1 已验证修复。** `cmd/agentdeck/ownership_packages_acceptance_test.go` 将 `RefreshQuotaCycle` 登记为消费已守卫 core 的入口，记录调用方持有刷新锁的前提。CLI 在调用前通过 `OpenRefreshCore` 打开 core，并取得该物理根的 quota refresh 锁。

同一带标签命令由退出 1 变为退出 0，格式和空白检查通过。验收文件 blob `a47ddd5996f39f0530560a17fe5decb52aaa6b0f`，修复后 68 路径候选 `dd1317337e90b74f20e633283336cc07e8b0cabff84273426f29447389086b52`。去掉该验收文件复算得到原 67 路径指纹。Task 4 继续开发，整锚独立评审须核对这项验收。

## Round 2 — 2026-11-18

评审结论：PASS

完成门禁：NOT_VERIFIED

PASS 限 Task 4 的八文件 Go 状态栏捕获切片。独立 Claude Sonnet 5.5 复算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、73 路径候选 `cb6ac9de299e6f3b80aedb3b333604a3257841ef17eb50a05be46c7c671a6592` 和切片 `6bd2a91d7f6f685f4b4366440f591d4105e67e5fc2f520c30e7bd25f7496200d`。T3 配置为 high。主控以三份原时钟文件和这次新增的五条路径重建前一 68 路径候选，确认切片外的范围一致。

独立变异验证时钟采样必须早于首次读 stdin。隔离 CLI 探针通过了含 NUL 与非法 UTF-8 的原样透传、链式调用顺序、默认关闭和授权限制。SQLite 触发器令周窗口写入失败时，五小时窗口一起回滚。开放闸门要求确认世代、同启动标识且 `entry_mono > capture_open_mono`。旧 `open_initial` 仍允许捕获。刷新和捕获共用 macOS `CLOCK_MONOTONIC` 或 Linux `CLOCK_BOOTTIME`。

主控核对 quota／CLI 全包、聚焦 race、vet、隐私与带标签所有权清单通过。独立评审另跑隔离 CLI 探针、定向 race 和跨平台构建。Linux 与 FreeBSD 的证据为构建或测试编译，运行证据位于 macOS。

- **SAI-QRP-CLAUDE-R1-F1 CLOSED**：Round 1 定向修复已将 `desktop.Service.RefreshQuotaCycle` 纳入所有权入口清单。主控在本轮核对 `TestOwnershipPackageEntrypointInventory` 通过；该发现已闭合。

投影须正确处理旧 `open_initial` 下已有状态栏窗口而 envelope 尚不存在的状态。该入口写入世代内窗口，envelope 的 prose 时间与来源由相应事务维护。Task 4 继续实现 Codex 调度、Go／Swift 投影及组合验收。

## Round 3 — 2026-11-18

评审结论：PASS

完成门禁：NOT_VERIFIED

PASS 限 Task 4 的五文件 Go Codex 身份调度切片。独立 Claude Sonnet 5.5 重算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和切片 `1282bad5325a871077fd3781152dd370f80007ca6ce971081a22b036c36bc44a`。T3 配置为 high。主控核对 75 路径联合候选 `7fb5dc0efda10c10a88adcf0bb248f6efb6c04e877b7f4efe70b6344dd70d128`、三包全量与带标签清单。实际 CLI 和 quota 定向 race 覆盖了单进程请求、独立到期、手动退避、余量退出、周期过期以及事务回滚。隔离对照验证生产适配器在用户 ID 和账户 ID 字面相等时仍保留未确认关联。独立评审的预算、过期周期和退避变异均被用例拦截。

**SAI-QRP-CODEX-R1-L1 CLOSED。** 新增 `TestScheduledCodexLegacyProbeSkipsIdentityOnly`。旧 probe 首次写入额度，同一间隔内仅身份到期时不再发起 probe 或推进尝试。定向 race 通过。在隔离 overlay 移除旧 probe 的身份到期守卫时，测试观察到调用次数由 1 变为 2，按预期失败。新测试 blob `d56ea965b0e2f518a9f3f09f02ac5d00e86032ba`，补测后联合候选 `28d2a369532b3463cdd2ae4fc4359c867f7ae4588489caab41f6149b873ecb99`、切片 `d4c1b36d8f2829430c0ce76851642c1e80a7ebcb7ada8f2b3c1bff87addc39e4`。将原测试 blob 代回验证得到评审候选。

生产配对 adapter 依据到期掩码产生结果。隔离探针遵守同一个回调契约。RefreshQuotaCycle 的实际入口已由 CLI 测试覆盖。Go 订阅投影、Swift 消费和组合验收继续由 Task 4 承接。

## Round 4 — 2026-11-19

评审结论：PASS

完成门禁：NOT_VERIFIED

PASS 限 Task 4 的 14 路径 Go 订阅投影。独立 Claude Sonnet 5.5 重算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、82 路径候选 `e11bf0ef656e1c85bff7ed19f47ed539c83b8617d2c994fc0996b846460c6958` 和切片 `a70956a52fc71c945bbb34449cb9c2f370161a5a2c4792bed71c36a4f5d6df2a`。配置档位 high。主控重建重试输入，核对三包回归和定向 race。

评审核对只读事务的首次读固定快照，再采样同一投影时钟。隔离探针证实精确 epoch、revision、同一 now、确认关联的周期组合、隐私哨兵拒绝，以及旧库已知失败顺序的两种方向。直读和 bootstrap 用当前状态替换缓存订阅段，损坏设置导致订阅段不可用，其他载荷保留。旧 CLI 文本和 WireVersion 1 行为通过。夹具与 golden 保留既有字段断言。

Swift 使用 Go 实际生成的 `internal/desktop/testdata/subscription-identity-v1.json`（blob `eab7898b73d1cb98171c884258b64ec69e359784`）。共享夹具覆盖 Codex 已确认关联和 Claude 的稀疏状态栏窗口。Swift 测试另覆盖 absent、null、last_read、失败细节和未知枚举。旧库验收依据 schema 合成夹具，真实升级验收归组合范围。订阅重投影随缓存读执行，其性能将随实际读取路径验收。Task 4 继续 Swift 消费和组合验收。

## Round 5 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 评审限 Task 4 的四文件 Swift 消费切片，配置档位 high。评审与主控复算 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 下的切片指纹 `259d815c4a49c1aafa9d00c8c7034758954b8e01bf69a86fcade90e406630c5f`，并核对 Go 夹具 blob `eab7898b73d1cb98171c884258b64ec69e359784`。作者的 Xcode 和评审在隔离副本中的 SwiftPM 各通过 100 项。评审另用变异和隔离探针检查实际安装路径。完整 Task 4 组合验收仍待执行。

- **SAI-QRP-SWIFT-R1-F1（中）** `publishStandaloneQuota` 可被改回无条件替换而现有测试全绿。通过实际独立刷新路径送回较旧的候选，断言较新的已装订阅段保留。待修复。
- **SAI-QRP-SWIFT-R1-F2（中）** App Group 预期值调用了被测裁剪函数，令 `legacyWidgetSubscription` 返回原对象仍全绿。用含身份的 Go 夹具走实际发布路径，在落盘 JSON 中独立断言完整订阅私有字段缺席。待修复。
- **SAI-QRP-SWIFT-R1-F3（低）** 现有用例不能识别非 bootstrap 全量刷新跳过合并 quota 候选。为回退入口补测试并核对实际安装的订阅段。待修复。
- **SAI-QRP-SWIFT-R1-F4（低）** 现有用例不能识别首次 serving 安装接受 header epoch 不匹配的订阅段。验证保留 serving 的其他部分并将该订阅段置为不可用。待修复。

同一候选中，释放 held 发布物的完整 Widget 写入需要保留原成功门槛。订阅实际更新时按 A10.3 写入，保留既有会话摘要。独立刷新返回相同订阅时需要保留 App Group 写失败后的恢复路径。补充对应回归证据。按 A11，显式 null 或缺失 `projection` 的段与已有带键段比较时视为最旧。渲染身份仍须按 `current_until` 检查到期。App Group 精简身份允许列表由后续 Widget 任务接手。

## Round 6 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Sonnet 5.5 复评限 Swift 四文件，配置档位 high。评审与主控核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、四路径 `9620f4a0a9fe6dbe1b712d65bba7e9322c915d07995aa1d687d1b6d1c24403a8`、Go 夹具 `eab7898b73d1cb98171c884258b64ec69e359784` 及当时 82 路径产品候选 `9d53b2fc4fa211029a426bf2ea9ad2f8c2a223fe945513b4c97e22d3aacc5ceb`。Xcode 的四套合计 114 项通过。复评在隔离 SwiftPM 副本用相同范围做定向变异验证。

- **SAI-QRP-SWIFT-R1-F1 CLOSED。** 独立 quota 入口拒绝覆盖较新订阅段，绕过排序会令新增测试失败。
- **SAI-QRP-SWIFT-R1-F2 CLOSED。** 含身份的 Go 夹具经过实际 Widget 发布后，独立检查 JSON 字节和旧套餐字段。对五类隐私字段逐项注入泄漏均被测试拦住。
- **SAI-QRP-SWIFT-R1-F3 CLOSED。** 非 bootstrap 的完整刷新安装较新配额段，禁用合并会令用例失败。
- **SAI-QRP-SWIFT-R1-F4 CLOSED。** 首次服务安装拒绝错误 epoch 的订阅段，保持同一发布物的其他数据。

复评另确认 absent 和 null 排序、held 释放时的完整成功门槛和独立订阅写、相等 standalone 结果重试写入失败。启动 partial 含新投影时按 A10.3 发布订阅，完整刷新随后更新其他段。Task 4 组合证据以真实 Go partial 验证该转换和 Widget 受影响内容。隔离 SwiftPM 副本缺少独立原生所有权 harness，涉该 harness 的测试由原生验收提供。Go 子进程稳定性验证、Task 4 组合证据和独立总门禁仍待收口。

## Round 7 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 组合评审绑定 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、82 路径 `878f6bc34783b8596b7649d4ae43502ebac2a4bb8f57a3cf92ceded18bc758ee`、73 路径 Go 范围 `05c9c362383045ee53110e103c8b0da674b7c74e8954cbdaf3c165fd609a7c56`、Swift 四路径 `9620f4a0a9fe6dbe1b712d65bba7e9322c915d07995aa1d687d1b6d1c24403a8` 和夹具 `eab7898b73d1cb98171c884258b64ec69e359784`，配置档位 high。

**SAI-QRP-R7-F1（中）** `internal/quota/claude_identity_test.go` 的超限输入一开始就不是 JSON。隔离变异关闭 `stdout.overflow` 判定后依然得到 `parse_failed`，密钥也不会创建。独立探针用合法登录 JSON 加超限尾随空白，基线正确拒绝，变异返回 `identified` 并写入本地密钥。修复测试应让它直接区分该守卫，分别验证分类和密钥未创建。待复评。

刻意超时与继承 stdout 检查保持有效，去除 `WaitDelay` 会进入 deadline 分支并失败。三项目标测试带 race 运行八轮均通过。隔离编译 helper 验证 Claude 刷新与 JSON 读取分开运行、身份和 usage 顺序、换账号失效与重确认、解析失败净化与恢复，以及门禁下零探测。真实 Go 首次导入产生 partial 再到 complete，订阅投影可用且顺序递增。Swift 单元与探针证据支持其安装规则。Widget partial 呈现在后续 Widget 任务和原生验收验证。

真实 helper 流进入 Swift 协调器、失效与 usage 提交之间的中间态，以及仅 Claude 的隔离 A13.3 以外的组合场景尚无本轮直接证据。内嵌 helper、GUI PATH、App Group／WidgetKit 和真实通知留在原生验收范围。生产变更不在 F1 修复范围。

## Round 8 — 2026-11-19

复评结论：PASS

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 复评限 SAI-QRP-R7-F1，配置档位 high。HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`，82 路径 `03a7d645a13ed810b150bb22873700bd34228ecd90d339b355f6324896cc1813`，73 路径 Go `c4485e8014f82926ac31395ed3150d1cb258350ad90b47b1dbcaad7b8853d7ea`。主控与评审以先前测试 blob 重建 Round 7，确认其余产品、Swift 和 Go 夹具一致。新测试 blob 为 `4fc9f980a729524dc8bcf1ef5aae9638d83f7a8c`。

**SAI-QRP-R7-F1 CLOSED。** 超限夹具写入可识别的合成登录 JSON 和超过 64 KiB 的尾随空白。分类与密钥创建分别断言。主控定向、race 和变异验证通过。独立复评带 race 四轮共 16 项通过。关闭超限守卫会触发分类和密钥两项断言，保留失败分类而提前加载密钥只触发密钥断言。去除 WaitDelay 仍会使对应测试失败。fixture 启动预算和刻意超时探针保留原设定。

继承 Round 7 其余组合结论。Task 4 仍需判定真实 helper 到 Swift 的端到端回放与关键失效中间态的适用证据。内嵌 helper、GUI PATH、App Group／WidgetKit 和通知继续按原生验收安排。

### 补充组合证据

独立隔离运行的 82 路径逐文件 SHA-256 清单在构建前后均为 `cfeb5478cb835f0e4d32a84662cd350f4e41aa72fe1015529b67e5d6534c9255`，73 路径 Go 清单为 `514f8442e8035400f03a86e5ee6b84fef64de6c17e06d6a1f932098576904788`。主控在相同文件集上同时复算两种清单，分别得到上述原始文件摘要和本轮的 Git blob 摘要 `03a7d645a13ed810b150bb22873700bd34228ecd90d339b355f6324896cc1813`、`c4485e8014f82926ac31395ed3150d1cb258350ad90b47b1dbcaad7b8853d7ea`。运行使用的 Go helper SHA-256 为 `cc9fc5629099659a71bf1288415f3e0d6d1a4d8144631cb6a23157897edda685`，Go 1.27.1，darwin/amd64。

A13.3 五步通过。实际刷新和独立 JSON 读取核对了首次成功、`missing_line`、`shape_invalid`、`timezone_unresolved` 以及恢复后 Fable 额外行被忽略。窗口与失败时间及字段均和只读 SQLite 对照。A13.2 的 FIFO 在真实失效提交后阻塞假 usage，独立读看到 `context_changed`、无窗口和 `capture_gate=blocked`。同一轮继续完成后上下文清空，新窗口为 41% 和 13%，闸门重新开放。该证据覆盖同一隔离 helper、合成进程、CLI 和 SQLite。历史根因未复现。Swift coordinator 组合证据及原生验收仍按任务归属核对。

隔离 smoke 通过生产 `EmbeddedHelperRunner`、`DesktopHost` 和 Swift 协调器安装完整发布。它观察到两个 Claude 窗口及同一 core epoch，helper SHA-256 为 `fc4ced025a8bce104903bc4aaa41a4071f4a6b568634292d50966b7d3d20ed90`。部分发布到完整发布的过渡、A13.2 生产刷新入口的 SIGSTOP 投递交错、注入最坏延迟下的 37 秒边界，以及实际 perf trace 隐私哨兵继续按 Task 4 的收据补核。真实界面和通知验收归后续任务。

同一 R8 产品候选补验 A10.3/A11 的部分到完整发布过渡。真实 Go bootstrap 产生的部分发布、较旧完整发布、当前完整发布依序进入生产 `EmbeddedHelperRunner`、`DesktopHost` 和 Swift 协调器。三次使用同一 core epoch 与 boot。部分发布 revision 24 时，隔离 `AppGroupSnapshotStore` 读回 57%／23%。较旧完整发布携带 revision 18 和 41%／13%，其余区段安装成功，额度保持 revision 24 与 57%／23%。当前完整发布 revision 25 到达后，协调器在 terminal 转为 ready，两个 coverage 均 complete，App Group `partial=false` 且窗口仍为 57%／23%。最终回放、helper 查询和编译均通过。初次回放的等待条件校正为在协调器检查排序，并在 App Group 用生产 reader 核对落盘额度。

主控复核执行日志和文件 SHA-256。helper `43786f16ad9a8052cbd717ad4f6c38510a043ce616017d4ff5cd15751d43ef38`，部分 `0270435ee23320d57bda1b2edac09b33efa0c0d2eea72b4a17715694f5655d71`，较旧完整 `dc6c790f5c07a8203ae224dc164363c429498fb451e41c4a2b66d9c832112000`，当前完整 `4f8e9be4711cc3a3b189a47a5beab763115b46733d7b89749779359ccbddc3cd`。当前候选的 Go 投影 14 路径、Swift 四路径及共享 Go 夹具与 Round 4、Round 6 已验范围逐一匹配，沿用其一致读取、兼容及解码证据。原生界面和通知仍由后续验收覆盖。

真实生产刷新入口的 A13.2 SIGSTOP 回放通过。`waitpid(WUNTRACED)` 核对两个受控持锁进程。对账被挡时 1.148 s 结束且零 Claude 调用，已对账后的守卫被挡时 1.540 s 结束且只调用 auth。两条路径均保持数据库和镜像世代 0，零身份或 usage 写入。`SIGCONT` 后 FIFO 提交先于下一轮，后者 0.514 s 内推进数据库和镜像至 1，旧候选零投递。

同一 R8 helper 捕获十份真实 perf JSONL，共 61 条 quota 记录。七个原始隐私哨兵和 Claude 派生关联值均无匹配。Codex 哨兵走实际 `account/rateLimits/read`，对应 37% 窗口提交。合集 SHA-256 `a7425bea3330ae120ba467d361cd87754ca327999a29c748f1bfe13817bc62db`。该捕获与同范围既有数据库、JSON 和错误脱敏证据一起核对。两组验证均只使用合成根目录和子进程，当前 82／73 字节清单同 R8。

A1.5 仍需精确预算证明。13.35 s refresh 锁、3.65 s auth 与 9.65 s usage 的组合在 27.139 s 内写入两窗口。持续锁争用在 14.518 s 后返回 `state_busy`，零调用、零写入。持有 SQLite 写锁时跳过 usage 且保留 quota 行，rollback trace 为 1.009196468 s，超过字面 1 s。该 trace 的放弃取得锁与清理边界需区分，所有子步骤上限同时注入的 37 s 也待验。门禁继续开放到精确证据完成。

墙钟回拨与前跳补证通过。四个隔离 `RecordStatusLinePayload` 用例固定 boot、世代 2 与 opening mono 10。−1 h 与 +1 h 下，entry mono 10 均写零行，11 均写两窗口，SQLite 与 `Store.Windows` 一致。窗口是 22.4%／3%，来源 `claude_status_line`、世代 2，reset 保持实际夹具值。探针 SHA-256 `e641b50628a451bdbe08b9d39207bc84ccf022fc0585f2b7905d1bbfc3e21ce7`。R2 八路径 `6bd2a91d7f6f685f4b4366440f591d4105e67e5fc2f520c30e7bd25f7496200d` 同当前 R8，沿用实际入口和两个窗口回滚验证。

A1.5 精确定位确认 R8 仍失败。受控 SQLite 写锁下，最后一次尝试前 elapsed 1.003753628 s，拒绝继续尝试 1.004029189 s，之后才开始清理。另复现上一 boot 恢复后换号导致两次 gate 写、探测阶段 20 s，以及延迟唤醒后 14.425 s 才取得 refresh 锁。八路径修复补丁 SHA-256 `15d87af42701d9e03825d22f5b0e9c1b067ec71f54e398637175ebeafdb01dee`，主控核对八个前像和补丁检查。其独立 overlay 证据包含四个失败在前、成功在后用例，以及受控组合 14.4 + 2.1 + 19 + 1.5 = 37 s。修复候选待独立评审。物理运行近上限成功为 36.643176353 s，最后一次锁尝试在 cutoff 前，唤醒观测存在亚毫秒开销。受控预算与 OS 调度下的物理返回范围分别记录。

## Round 9 — 2026-11-19

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 Claude Opus 5.5 评审限 R8 原始前像上的八路径 A1.5 修复，补丁 `15d87af42701d9e03825d22f5b0e9c1b067ec71f54e398637175ebeafdb01dee`。八个前像、三个交付物和八个实际 postimage 均核对。受控正常路径精确 37 s，held 路径 14.4 s，恢复换号只写一次闸门并将 usage 留给下一周期的测试成立。独立复跑的真实时钟成功路径为 36.636 s，使用近上限注入。

- **SAI-QRP-R9-F1（阻塞）**。新导出的 `AcquireQuotaRefreshLockBefore` 缺带标签的 ownership inventory 行和 barrier 探针，实际验收失败。
- **SAI-QRP-R9-F2（阻塞）**。先 pin WAL 读快照使 SQLite 不再调用 busy handler。`busy_timeout=1000` 虽可读回，20／100／400 ms 的常见短争用均立即返回忙，原实现三例均能成功。刷新序号等失败会抑制本轮探测。继续保留 A1.5 的有界短争用等待和实际时限，准备证据充分的替代实现。
- **SAI-QRP-R9-F3**。交付测试只验证已过期入口，去除循环里的 cutoff 后仍通过。临时延迟唤醒夹具可捕获 14.425 s 的迟取锁，须加入永久回归。
- **SAI-QRP-R9-F4**。原 `lock-wall.log` 的通过记录与已发布夹具的 Fatal 条件矛盾，退出验收证据。独立用该夹具复跑，记录最后尝试比 cutoff 早 3.306477 ms，放弃观测晚 193.541 µs，物理总耗时 14.401 s。此为限定条件内实测。
- **SAI-QRP-R9-F5**。恢复 `CheckQuotaRefreshLock` 的注释归属及避免 `RefreshLockDeadline` 遮蔽内置名称。

主控在独立副本重放 F1／F3／F5 的四文件补丁，并按 Git blob 和字节摘要核对全部九个输入及输出。补丁 SHA-256 `78a561cec2406ee3de54f1b0301f8cead340c284efc8aa007a093dda79316658`，清单 SHA-256 `cb9493cac64bb4a9db4ee440eed2e4c4b6b3ddefa0e751703ad827d01842eaad`。原带标签清单只缺 `AcquireQuotaRefreshLockBefore`，补齐后该入口的 barrier 探针及两个对照通过。`TestQuotaRefreshLockCannotRetryAfterCutoff` 使用虚拟时钟延后唤醒，仅移除循环截止判断即可复现迟取锁，已过期入口用例仍通过。定向 race 与 CLI 测试通过。首次包级运行的两个 quota 探针失败，定向和串行 quota 套件重跑均通过。待 F2 修复后验证并复评完整候选。

F2 实验记录已核对。现行驱动设置 `busy_timeout=1000` 后，即使父 context 只有 150 ms，持锁对照仍在 1.014766266 s 返回。注册 SQLite 自定义 busy handler 将 pragma 读值改为 0，重新设 1000 则替换该 handler。隔离实验用两个临时驱动文件增加每条语句的回调；20／100／400／950 ms 的短争用及 150 ms 父截止内的 80 ms 争用均成功写入。持锁对照一次在 race 下于 1.000544404 s 从 SQLite 返回，当时尚未清理。人为延迟唤醒后拒绝继续重试，迟到获得锁时也在执行写入前回滚。身份写失败时零 Claude usage 调用。

主控校验收据并在独立副本逐一重放十个文件。应用补丁 SHA-256 `fd20132b45f41bfb7fa76d5f02890113c08d7be6e6e68d92c9e4f04f508c36b1`，临时驱动补丁 `8a515bfcd2e6907b0e67f938767921fc3ece88830984483674863d418c88a1c1`。试验仍不满足现行 A1.5，F2 保持开放。需要明确写锁尝试的单调截止及进程被调度后的返回时间分别如何验收，然后在同一组合候选复评。

2026-11-19 用户批准采用单调截止。A1.5／A13.2 已记入专用连接上的有界取锁、迟到回滚、事务体至多一次、恢复连接设置和分开记录真实耗时。37 秒 helper 和 40 秒 Swift 预算继续执行。此处保留本轮 FAIL 的适用候选，修订后的契约及组合实现待独立验证。

### 单调截止组合候选

组合 85 路径 `e2d6aa54ab506905e84eb7ca110c39599257652af30e75e1e5c81a6aab085146` 位于隔离验证副本。以 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和架构 blob `4db37b3f81fe00ac264e7cbae63cb45565aac681` 为准。主控从实际 82 个 R8 文件重算原产品指纹，并把 13 路径总补丁 `37063693bb078f6399c07c2ad984cec263a2f35756216798fd0b23a0cfe05b56` 在同前像副本逐字节重放。依赖沿用现有 modernc v1.53.0。

组合的 quota、desktop、store 和 CLI 包均通过。首次命令中的 quota 套件检出仍使用原 9 s 的单侧 Codex 夹具，改为区分单侧和双侧新预算的 11 s 后全包通过。其余三包的执行适用于同一产品字节。定向 race、vet、所有权清单与新增入口的 barrier、desktop refresh 集成合同均通过。失败注入走实际 refresh 写入口。使用原有两个负向对照确认回滚失败会滞留写锁、恢复失败会污染池，良好候选在两种情况均丢弃物理连接。

最终组合在六处可定位计时注入下运行实际 helper 命令路径。受控正常刷新和末次修剪回滚均为 37.000 s。持续持有 refresh 锁在 9.400 s 返回 busy，零探测、零刷新写入。实际时钟近上限成功为 36.523513394 s。另用独立物理持锁回放记录授权时钟，最后一次取锁授权比 cutoff 早 21.810293 ms，返回比 cutoff 晚 1.182143 ms，命令共 9.401754434 s，零探测、零刷新写入。这里分别绑定准入和物理耗时，覆盖原不一致计时收据的替代验证。

本地验证已完成，候选进入独立代码复评。A5.7 的新锁对象按用户决定另行评审与组合，本次计时以当前隔离候选已有的守卫路径执行。

## Round 10 — 2026-11-19

复评结论：FAIL

完成门禁：NOT_VERIFIED

独立 OpenCode Astra 核对 85 路径候选 `e2d6aa54ab506905e84eb7ca110c39599257652af30e75e1e5c81a6aab085146`。模型选择为 `openai/gpt-6.2-astra`，T3 配置 `variant=max`。复评验证真实 blob、带标签准入、独立取锁截止和失败清理的变异，以及 37 s 受控组合与实际计时。**SAI-QRP-R9-F1／F2／F3／F4／F5 CLOSED**。先前 Claude 运行受服务额度限制结束，未产生评审结论。

- **SAI-QRP-R10-F1 OPEN**。上一 boot 的捕获闸门已经在本周期恢复，后续 usage 准入仍预留另一笔 2 s 闸门写入。恢复后同账号只需 15 s，现实现要求 17 s。独立场景在 8.4 s 取得 refresh 锁，身份提交后剩约 16 s。结果为一笔闸门写、零 usage 进程、零 usage 提交。6.4 s 取得锁的宽松对照有一次 usage。修复需去掉已消费的闸门配额，保持换号后的延迟和每次写入清理配额，并用近截止状态绑定实际 usage 提交。

## Round 11 — 2026-11-19

复评结论：PASS

完成门禁：VERIFIED

**SAI-QRP-R10-F1 CLOSED**。独立 OpenCode 复评核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、A1.5 架构 blob `4db37b3f81fe00ac264e7cbae63cb45565aac681` 和 85 路径候选 `33d81473bae26d7d8aab50c9ddb0482f24cbb817544527bb0b9aaf8bb7b97892`。T3 选择 `openai/gpt-6.2-astra`、`variant=max`。相较 Round 10，仅 `refresh_schedule.go` 和 `refresh_schedule_test.go` 两处变化。闸门发布资格同时决定预留和实际写入。此前 SAI-QRP-R9-F1、SAI-QRP-R9-F2、SAI-QRP-R9-F3、SAI-QRP-R9-F4 和 SAI-QRP-R9-F5 的关闭证据继续适用。

旧实现的近截止回归为零 usage。修复后的定向 race 断言同一账号剩余 16 s 时，实际写库从 80% 更新为 81%。换号仍然保留阻塞和未确认状态，下个后台周期可刷新。受控 helper 在 8.4 s 交接 refresh 锁。同账号一笔闸门写、一笔 usage 提交，共 36 s。换号一笔闸门写、零 usage，共 21.5 s。正常和末次修剪回滚仍为 37 s，持续持锁仍在 9.4 s 返回忙碌且零探测、零写入。恢复夹具仅启用 Claude，正常全周期覆盖两客户端。上述均为合成时钟，实际耗时沿用同适用路径的有限回放证据。

### A5.7 采纳后验收

在采纳后的 88 路径候选 `bf02b04a359e83db4e034ace1a1d29bc9792136a912b8869df4b48a940874453` 上，CEv1 通过 9/9。独立复评覆盖的 A1.5 调度与清理保持生效。A5.7 的实现和交接复评见 [投递守卫 Round 5](quota-delivery-guard.md#round-5--2026-11-19a57-独立复评)。

- 受控时钟把首次守卫初始化和取锁计入同一 G。正常和末步修剪回滚含清理均在 37 s 返回，持续持锁在 9.4 s 返回 busy，零探测和刷新写入。真实时钟缩短延迟回放记录为 36.501451123 s。Swift 仍使用 40 s 超时。
- 真实隔离 helper 五步刷新和独立 JSON 读取一致，涵盖 `missing_line`、`shape_invalid`、`timezone_unresolved` 和恢复时忽略 Fable 额外行。只读 SQLite 记录一致。历史根因未复现。
- 独立文件持锁期间，竞争路径零身份及 usage 写入，两处观察耗时分别为 1.033422 s 和 1.063988 s。持锁方恢复后，下一轮在 FIFO 暂停时读到 `context_changed`、零窗口和阻塞闸门。解除暂停后读到 41%／13% 的新窗口和开放闸门。旧世代的后续提交为零。
- 当前 helper 的 JSON、stderr、SQLite 和实际性能轨迹共检查 59 个文件，17 份轨迹共 129 条，未出现原始哨兵。四个 Go 包测试、vet、定向 race 和桌面刷新集成通过。沿用有效的一致读、Swift 发布和状态栏时钟证据。完整 macOS 校验及实际 App 验收仍是后续任务。

主控从实盘 R8 验证全部前像并采纳已复评的 13 路径补丁 `0c56708583c27fcfe8b956c7cc7683a9ed963c7d8b7c69d8cd68a9b159570853`。采纳后 85 个产品 blob 全部与复评一致，HEAD 和 Git 暂存区保持原状态。本轮结论限 A1.5 独立修复。新版 A5.7 守卫仍按其实现与组合证据完成任务 4 复验。

### 独立交付切片与补验候选

主控从签名的 Task 3 `47207d34b45d6ff98cb60ad33bec385c84c2525c` 重放 Claude／共享切片 48 路径、已单独复评的 P2 Codex 先决补丁 5 路径及 Codex Task 4 增量 12 路径。全文件与模式一致，零模糊、零偏移。对应三棵 Git 树为 `11ae22021fd74b32b7be0404ac7b72a05f1ae85b`、`a905a90bf11de55ad7700ae321e901d3bc2f69d2` 和 `2c7c65ed9ff2aa766f2a25a06b3e2219a48c4133`。Claude 切片沿用旧 Codex usage-only 路径，Codex 增量接入同一进程内的身份和额度请求、各自的提交与投影。

**SAI-QRP-R12-F1 OPEN（修复候选已验证，待独立复评）。** 真实命令收集告警时，最初有效的刷新锁凭据被删或 SQLite 中的 cycle 已推进，第二次证明检查返回空值。旧行为丢弃 Claude 和根证明，保留原 Codex 告警，却返回 `partial:false`。修复把该空结果交给已有 `quota_alerts_failed` 路径。两个故障场景在修复前复现，修复后两个切片的全包和定向 race 均通过。首次检查为空和原有显式错误的入口继续沿其既有合同。

相对曾验收的 88 路径组合，85 个 blob 一致。变化是上述一处生产文件、A7 旧库失败时刻断言、双锁顺序夹具，另新增真实命令的晚期证明失效回归。两个切片均通过四包 Go 回归、定向 race、vet、带标签所有权和桌面集成，以及各 67 项定向 Swift 宿主测试。真实 CLI 至 Swift 的探针均保留一次 Codex 提交与 ACK、零次 Claude 提交与 ACK。

受控时钟的成功和修剪回滚均为 37 s，持续争用为 9.4 s。同账号剩余 15 s 时提交一次 usage，低于判定点的负向对照未发起 usage。两边同时抵达近上限的 SQLite 写入时，独立观察到分支按忙碌上限退出。错峰载体和组合探针分别验证完整成功。已核对 20 份回执摘要。真实安装 App、系统通知和 Widget 行为按 Task 7 验收。

## Round 12 — 2026-11-19，来源切片与晚期证明复评

复评结论：PASS

完成门禁：VERIFIED

独立 Claude Opus 5.5 分别通过 Claude／共享树 `11ae22021fd74b32b7be0404ac7b72a05f1ae85b` 与 P2 先决树 `a905a90bf11de55ad7700ae321e901d3bc2f69d2` 上的 Codex Task 4 组合树 `2c7c65ed9ff2aa766f2a25a06b3e2219a48c4133`。基线为签名 Task 3 `47207d34b45d6ff98cb60ad33bec385c84c2525c`。T3 请求 `effort=max`，实际推理档位未独立观察。

**SAI-QRP-R12-F1 CLOSED。** 两切片独立复现删除刷新锁凭据或推进 cycle 时的误报完整成功。加入四行受限映射后，原 Codex ID 保留，Claude 告警与证明被省略，并返回唯一的 `quota_alerts_failed`。普通及 race 回归通过。持有且仍有效的投递守卫继续产出完整证明；首次空证明和有效显式错误沿各自既有入口处理。

**O1（边界观察）。** 在首次证明之前删除刷新锁凭据时，原 Codex 告警仍可提交，Claude 因缺证被抑制，当前信封没有 warning。复现中的绑定守卫文件仍有效。A5.7 的 partial warning 要求覆盖守卫不可用。保留既有首次空证明的语义，本轮发现限已经获得初始证明后失效。

**E1（证据适用性闭合）。** 主控在两个最终切片核对受控夹具仅替换测试中的截止行，并直接重跑准确 15 s 和少 1 ns 的对应分支。恰到边界时各提交一次 usage，负对照各观察到零 usage。两切片的 37 s、9.4 s、单闸门写与独立清理适用受控时钟。近上限的同时 SQLite 写入复评确认可在各自单调截止返回忙碌，无事务体重试。

两来源的 Go、各 67 项 Swift 宿主、投影和守卫组合回执已核对。Claude 切片保持旧 Codex 只读额度探测和原计划输入，Codex 叠加增加同进程身份和额度请求、确认条件与读面。已验证旧五版本 A7 失败时刻和 CJSI 兼容双锁夹具。安装 App 和系统原生行为仍归 Task 7。

完整组合候选 CEv1 `subscription-account-identity:quota-refresh-publication:state:077fa5b8b73bdbbbfe97a4212a75ac512dd1dee62225d2594a39bac9e39535ad` 通过 9/9。Claude／共享切片的单独证据绑定 `subscription-account-identity:quota-refresh-publication:state:e9afbdecedb09c63d6c3d6f7eae2ed0a118b3f1b6dc8ad10a9771f91acdeadb9`。先签 Claude／共享交付；Codex 按已有 P2 与 Task 4 增量顺序接到最终共享基线。
