# 额度投递守卫评审

## Round 1 — 2026-11-18

评审结论：PASS

Claude Sonnet 5.5 对 Go A5.7 两文件交接建议 PASS。T3 请求 high，实际推理档位未独立观察。评审匹配 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和候选 `9b12260f98d2d45506b7aca1022958e85c1cc01087cbeb566c40a2b3bc10e650`。测试中 helper 按 ownership session、refresh 锁、目录锁、SQLite 写事务的顺序持锁。镜像文件及目录同步先于失效提交。提交报错时保留领先镜像。后续对账读取当时已提交的库值。

### 生产接线约束

- **任务 4 的本周期绑定。** 在已取得 ownership session 时建立核心库与物理状态目录的关联，记录或核验打开核心库时对应的目录身份。第一次 Go 对账取到的目录必须与该身份相符。当前 `pragma_database_list` 返回路径，不能单凭路径证明打开的库属于当前目录。保持会话至周期结束，并让工作入口持有效 refresh 锁。`GuardStateRoot` 在目录锁等待预算之前调用，依赖已存在的会话。
- **Swift 提交与验收。** App 按 `state_root_id` 和世代复核，持锁到 `UNUserNotificationCenter.add(_:withCompletionHandler:)` 发起并返回，解锁后等待回调。任务 7 分开记录失效提交、调用发起、回调及横幅。既定验收要求失效提交后旧世代调用为零。

### 证据与补测

独立评审执行 Go 定向 race、vet 和格式检查。主控核对全量 Go 和原候选指纹。FIFO/SIGSTOP 夹具在子进程持锁时验证 helper 返回忙碌，继续、终止及 exec 场景均通过。失效后重放旧世代候选时 FIFO 无额外事件。夹具的提交时间戳取在 `ApplyClaudeIdentity` 返回之后。时间戳与真实通知提交时序由后续原生验收补证。延迟约束失败验证镜像保留与下一轮对账。

主控补测普通文件权限、共享 inode、FIFO 和目录占位。锁忙场景断言身份尝试表无新增记录。原 Go 生产 blob 保持 `871f71eb3cb79dc78cc0d471e9164ad588634a27`，补测后的两文件候选为 `f2d973f74d6c5e4ed969f118db30b5201112c63cade333711ff1e2c95a4f9b0a`。`scripts/run-go-test.sh -race ./internal/quota -run '^TestQuotaDelivery(Reconciliation|StoppedHolder)$' -count=1`、`go vet -mod=vendor ./internal/quota`、格式与空白检查通过。

完成门禁：NOT_VERIFIED

Task 3 仍在开发。Swift 投递、helper 告警信封和带世代 ACK 尚待接线与联合测试。真实原生提交留待任务 7。

## Round 2 — 2026-11-18，Swift A5.7

评审结论：PASS

独立 Claude Sonnet 5.5 评审对 Swift 五文件候选 `69b755ce182f5630ceb124220c9473e581227233e9fffe1d62c1b49a38cd4336` 建议限定范围 PASS，实际推理档位未独立观察。评审核对相同 runner 的 ownership 会话、目录持锁期间同步提交、锁外等待回调、旧 helper 缺证降级和跨语言 epoch 格式。主控确认原候选五项定向 XCTest 通过。

- **F1** 重新核对物理根时跟随末级符号链接。与 Go 使用同一物理目录身份，指向相同根的链接可以投递。
- **F2** 用真实 flock 争用通知标记旧目录已打开，随后替换根目录并释放旧锁。旧描述符仍匹配候选与有效旧镜像，投递因当前物理路径身份变化被拒绝。
- **F3** 保留原非 Claude 提醒的提交路径和 ACK 顺序。测试包含旧客户端字段。回调用例增大了定向测试的及时回调窗口，生产等待仍为五秒。

修订后五文件候选 `a5dcb84b669efbb2099b832bc0a99b3918ce6aec73bfc80f4046e6ffc8fab173`。用项目认可的隔离测试主目录完成六项定向 XCTest，覆盖 App notifier 四项、信封解码及支持上限和未来 schema 夹具，退出 0。App 使用假通知 poster。最初全量运行发现旧夹具仍将 36 当作未来 schema。修正夹具后全量重跑在 Xcode 停滞并由主控结束。两次定向运行的临时目录前缀不符合 Debug App 的隔离校验，定位 precondition 后以认可前缀重跑成功。

完成门禁：NOT_VERIFIED

后续把 Go 同一刷新周期的 `state_root_id`、提醒世代和事务内 ACK 接入该 Swift 候选，再完成联合测试及真实原生提交证据。

## Round 3 — 2026-11-18，Task 3 联合评审

评审结论：PASS

完成门禁：VERIFIED

联合评审匹配 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe` 和 11 文件候选 `d2458cef852d5dd5dcf077c095e83f3e20445a0dda2c765ce350a628949a38ee`。独立 Claude Sonnet 5.5 核对并定向验证单一读事务、qa1/qa2、事务内 ACK、旧世代丢弃、保护目录与世代信封以及 Swift 兼容。主控复核实际 Go 包测试、race、vet、隐私检查、六项隔离 XCTest 和生成实际 Go 信封后由 AgentDeckShared 解码的夹具。

主控补充两条回归。缺失 provider outcome 时即使守卫有效也不发提醒。旧 schema 29、30、35 只读 Codex 提醒为世代 0，并由旧账本去重。两项定向 race 通过。补测后 12 文件候选 `130eebc40d2afc94a6ce332793b1171b9165b6b51174125bf9525ed6a23302f6`，原评审的生产 blob 保持一致。本候选 Task 3 门禁五项通过。

Task 4 将实际 refresh cycle、已打开核心库的物理目录身份与 reconciled guard 串入生产 helper，并把 alert notices 修剪纳入 A1.5 的有界写入事务及争用测试。直接 ACK 已接通，当前没有有效守卫时 Claude 不发提醒。真实原生调用、回调与横幅仍是 Task 7 的验收。

## Round 4 — 2026-11-19，A5.7 独立文件守卫

评审结论：FAIL

完成门禁：NOT_VERIFIED

独立 OpenCode 评审核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、87 路径候选 `4f197020d6073562afc49c9b8e229709705e0b9c2e9095b55ff3e49365126405` 及补丁 `10dd59d3d0fda6fd71c0737a93ca06d2a62c0a89e1e87f1bc7907800c7648a2c`。T3 选择 `openai/gpt-6.2-astra`、`variant=max`。初始化、同 inode 恢复、共享等待预算、真实 exec 的 CLOEXEC 对照、世代提交次序与混合 Codex 告警交接均有通过证据。本轮限 A5.7 实现与验收接线。

- **SAI-QDG-R4-F1 OPEN**。Swift 在一个批次内沿用一次守卫校验。主控在首个 `add` 后替换同名守卫，旧实现继续提交第二条并返回两个 ACK ID。每个 Claude `add` 前须在同一描述符和截止内复核绑定和物理根。
- **SAI-QDG-R4-F2 OPEN**。Go helper 在 Claude usage 期间丢失守卫后仍可输出完整成功、Claude 告警及缓存的根身份。主控复现失效输出，持有有效守卫的对照继续返回两个客户端。须在信封签发证明时验证实际绑定，失效时保留 Codex 并发出既有 warning，避免再等待投递锁。
- **SAI-QDG-R4-F3 OPEN**。两个 Go／Swift 跨语言用例依赖测试 peer，旧 `test-macos-app.sh` 在标准 Xcode 路径只生成基础原生驱动，未生成和传递这两个 peer，缺失会跳过用例。须在该测试入口构建并传递 peer，将缺失变为测试失败。

A1.5 近截止复评、CJSI 读先开组合及真实安装 App 仍有各自的证据判定。待三项修复与定向复评。

### Round 4 修复候选

88 路径候选 `8bd2e682c928b30c6b48cff3b35c0cc8947a57337e89920cc52e62213ca3aa9f`，十文件补丁 `35c209bb6ca3d56174f77651fa85c4dc838df0d5cbbe279d5e1f70005cf31975`。主控逐前像回放，候选的全部产品 blob 可重算。

Swift 每一条 Claude 提交前都在同一投递描述符、同一截止内复核。两条健康告警均可提交，首条后替换守卫的回归只提交第一条并只返回该 ACK。Go 在收集完告警后重新查验绑定文件再发证明。usage 中丢失守卫的回归从两个告警和完整信封变为单条 Codex 与既有 warning。持锁但文件身份有效的对照保持两条告警，proof 查验无需等待 flock。

标准 macOS Xcode 入口构建并传递两个 Go 测试 peer。缺少任一 peer 的强制模式对照均报失败。实际标准入口运行 `QuotaAlertNotifierTests` 十项全部通过，包括两个跨语言用例及中途替换用例。混合信封为 Codex 提交与 ACK 各一，Claude 均为零。整套 macOS 测试退出 65，另有 `WidgetSessionScopeBatchTests` 针对 `project`、`revision` 的两个断言及 `MenuBarViewModelTests` 宿主未激活的失败。主控逐字节核对相应测试、共享 Swift 源和 snapshot 输入与前一候选一致。真实安装 App 的验收状态由任务 7 记录。

默认 Darwin 临时目录路径下，旧候选的崩溃恢复与 CLOEXEC 子进程用例均在实际操作前报不可用。使用物理临时目录的对照通过。测试夹具负责解析路径后，两组在默认环境均通过。全 quota、定向 race、vet 与 desktop refresh 集成合同通过。叠加后的隔离 CJSI Go 组合 `674f5dee092580c67b8e9b4a7c0e0cd8f2741eb4abfd5cc8f46ed0f07182773b` 通过读先开、排空准入与保留连接、告警世代 ACK 及运行中守卫失效回归。此为本轮工作者的验证，待独立复评。

主控从 88 路径候选构建两个最终测试 peer。使用完整隔离原生所有权夹具的定向 Xcode 重跑通过 `QuotaAlertNotifierTests` 全部十项，退出 0。混合信封重现 Codex 提交与 ACK 各一，Claude 均为零。

## Round 5 — 2026-11-19，A5.7 独立复评

复评结论：PASS

完成门禁：VERIFIED

独立 OpenCode 复评核对 HEAD `a224b0b824d2cad451d3bd83153832d65506f0fe`、88 路径候选 `8bd2e682c928b30c6b48cff3b35c0cc8947a57337e89920cc52e62213ca3aa9f` 及十文件补丁 `35c209bb6ca3d56174f77651fa85c4dc838df0d5cbbe279d5e1f70005cf31975`。T3 选择 `openai/gpt-6.2-astra`、`variant=max`。复评限 SAI-QDG-R4-F1、SAI-QDG-R4-F2 和 SAI-QDG-R4-F3。

- **SAI-QDG-R4-F1 CLOSED**。Swift 每次同步提交前，使用同一守卫描述符与截止复核文件名、inode 和根绑定。复评核对健康对照的两条提交，以及首条之后替换守卫时的一条提交和一个 ACK ID。
- **SAI-QDG-R4-F2 CLOSED**。Go 收集告警后重验当前周期和守卫，再签发证明。独立用例在收集期间替换守卫，保留相同 Codex ID，输出一条现有 warning，去掉 Claude 告警及根证明。守卫身份有效但被持锁时仍给出两条告警和完整信封。
- **SAI-QDG-R4-F3 CLOSED**。标准 Xcode 路径实际运行两个 Go／Swift peer 用例。同一 helper 信封保留 Codex 提交和 ACK 各一，Claude 均为零。缺失 peer 的对照令两个必选用例失败。Darwin 目录别名修订限测试夹具。

主控按前像采纳 12 路径补丁 `24bf0e662a2fad38b3935ab3121d9f19d87327fe99e31d6feb1b1d9916dad640`。两个调度文件保持 A1.5 修订，合并后与已经通过的隔离 CJSI Go 候选 `674f5dee092580c67b8e9b4a7c0e0cd8f2741eb4abfd5cc8f46ed0f07182773b` 逐字节一致。订阅工作树 88 个产品 blob 全部核对，指纹 `bf02b04a359e83db4e034ace1a1d29bc9792136a912b8869df4b48a940874453`。

这一状态通过 CEv1 5/5。当前 helper 与 Go 持锁进程核对了竞争时零身份和 usage 写入、恢复后先提交旧通知再提交失效、新一代下旧候选零提交。组合测试核对了先只读打开的 v36 库在保留 SQLite 连接期间仍阻挡目录排空，同时真实 refresh 可以取得独立守卫。Go 额度、桌面、命令和存储包测试及 vet、定向 race、隐私检查通过。

完整 macOS 校验仍有前述 App Group 断言和菜单栏激活失败。此轮复评结论仅适用于 A5.7 的三个修复项。实际安装 App 的通知验收仍由任务 7 负责。

## Round 6 — 2026-11-19，独立交付切片复评

复评结论：PASS

完成门禁：VERIFIED

独立 Claude Opus 5.5 复评核对签名基线 `40dd46642718de33f4994a1d22d6640562ea37ce`、16 路径候选 `254963c1a566601b00a4899f280bcdb041f9e113a688945291894ce68414bf50` 及产品树 `d224d1e2a30fc1204ce19c166bbfe51eccdc53fd`。T3 请求 `effort=max`，实际推理档位未独立观察。主控和复评方分别核对严格重放、16 个前后像与完整产品树，补丁 SHA-256 为 `1b738647557798769e0ea0c4244156fb8807790191091ceff95511f549e66807`。

9 个工作文件沿用 Round 5 的已评审产品字节。其余差异限独立刷新入口、阶段接口及对应夹具。守卫仍使用 A5.7 独立文件，首次初始化与锁等待共享同一截止。绑定持久性、失效后复核、跨锁域、世代 ACK 和旧 Codex 路径均有通过证据。晚期文件替换的混合信封保留现有 partial warning，Codex 提交和 ACK 各 1 次，Claude 均为 0，根证明省略。

精确候选的 quota、store 及独立最终 CLI 全包回归、定向 race、vet 和 17 项 Swift 定向测试通过。缺 peer 对照令两个必选跨语言用例分别失败。复评另外执行真实独立 CLI 的三个检查，核对缺守卫时的 partial warning，以及关提醒或 Claude 非官方时不发该 warning。该 App runner 只解码提醒，独立切片的警告仍留在 CLI 信封。

Task 4 接续物理根锚定、刷新周期及锁、写入与报告预算和真实生产组合探针。复评核对了 40 个交接跨度，包括 9 段 runner 发布代码。集成阶段另核对周期中途失效时的 warning；当前独立生产入口使用零值周期，Claude 投递保持关闭。v36 保留连接的 CJSI 组合证据另随精确组合记录。安装 App 和实际通知的提交与横幅验收仍归 Task 7。

独立候选 CEv1 状态 `subscription-account-identity:quota-delivery-guard:state:254963c1a566601b00a4899f280bcdb041f9e113a688945291894ce68414bf50` 通过 5/5。
