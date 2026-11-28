---
status: historical
retired: 2026-11-27
created: 2026-11-17
---

# 缺陷：GUI helper 找不到原生安装的 Claude CLI

载体：`ad-bug-claude-quota-probe-gui-path`；v0.7.5；Lane A。
工作区：`agent-deck.fix.claude-quota-probe-gui-path`；分支 `fix/claude-quota-probe-gui-path`。

## 现象

GUI/helper 使用的固定 PATH 不包含 `~/.local/bin`；原生安装器将 Claude 放在该目录时，额度探测执行裸名 `claude` 并报 `probe_failed`。用户手工建立的 `/usr/local/bin/claude` 链接只是本机绕过，不是产品修复，也不是测试前提。

## 根因

`ProbeClaudeProse` 原先直接使用 `exec.CommandContext` 查找裸名，完全依赖 PATH。原生 helper 的固定 PATH 保留系统与 Homebrew 目录，故交互 shell 可找到的原生安装不会自动进入 helper 的搜索路径。

## 修复边界

- 显式带路径命令继续原样使用，路径失效不能悄悄切换到另一个安装。
- 裸名优先使用现有 PATH；找不到时才检查当前 HOME 下两个固定安装位置 `.local/bin` 与 `.claude/local`。
- fallback 每一条字面目录链、真实目录链和符号链接跳转须不能被其他用户替换：从 `/` 检查目录，属主为 root 或当前用户，group/other 不可写；仅 sticky 可写目录在其下一级条目也属于 root 或当前用户时例外放行。最终文件须为 root 或当前用户拥有、group/other 不可写的普通可执行文件。符号链接最多8跳；PATH 成功匹配和显式路径不扩大到这套 HOME fallback 检查。
- 不加载 shell rc、不新增配置入口、不改变 Swift 固定 PATH；不接入账号身份、套餐映射或解析器扩展。
- 发现失败沿用 spawn/probe_failed 分类及已有超时边界。真实 GUI 的完整启动验收与受限 PATH 的隔离测试分别报告。
- 代码仅涉及 `internal/quota/claude.go`、三个 `claude_exec*.go` 和 `claude_test.go`；没有修改 Swift、安装 App 或真实认证/会话状态。

## 验证

基线 HEAD：`4ac56db32e13c4e925b648c620ba76fc588a7781`。
主控核对的最终代码/测试身份：

| 路径 | Blob |
| --- | --- |
| internal/quota/claude.go | 6fbf9fbd622c167809a7fe749d1ded4b193f2f05 |
| internal/quota/claude_exec.go | 269a40558301c211f6e1fc0fb0dd59aba098d9d5 |
| internal/quota/claude_exec_other.go | 31f43aa025e97033932d4e9a9e091e2d19c09b2f |
| internal/quota/claude_exec_unix.go | f021e79e2c4449323ac4ca5a3f740f259ae16724 |
| internal/quota/claude_test.go | bb0554952a8dc4f1eb013512971cb1c9ff45d975 |

包含 HEAD、以上五文件及 go.mod/go.sum/vendor/modules.txt 的 SHA-256 为 `a49b9c0d0d0cd8558eb71ca95532a244f300b9d9a6524fe961f8173a1e8acfa6`，CEv1 保存相同 recipe；三个新增文件明确计入，不以仅 tracked diff 作为完整指纹。

主控从 T3 原始命令回读核对：`scripts/run-go-test.sh -race -count=1 ./internal/quota ./internal/desktop` 与 `scripts/run-go-test.sh -count=1 ./cmd/agentdeck` 的 wrapper 均报告通过，`go vet ./internal/quota ./cmd/agentdeck` 无诊断。未运行全仓 `./...`。
开发者报告：修复前受限 PATH、原生安装链接和 fallback 超时三个回归失败；该失败先行的完整原始结果尚待独立回读或最小 baseline overlay 复核，不能只凭开发者文字注册为通过证据。

现有测试覆盖 PATH 优先、显式路径失效不回退、找不到命令、超时、不可信权限/目录/悬空链接及相对 PATH。环境为隔离 HOME/PATH 和假命令；未执行真实 Claude、LaunchServices 或已安装 App 验收。
开发者试运行 Windows vet 在既有 platform/unix.Flock 边界失败；本项目主支持 macOS，此结果不作为本修复的通过证据或已修复项。

实现来源为 Claude/Sonnet 5.5，请求 effort=medium；实际执行配置依赖 T3 可观察字段，未据请求值声称执行证明。
独立评审与 `fix:claude-quota-probe-gui-path` gate 尚未完成。没有 commit、push、安装或 Review PASS。

## Review — Round 1

完成门禁：NOT_VERIFIED

2026-11-17 格式补记：主控在该轮精确候选上查询此值；不改变原 FAIL 或将建议结论冒充 gate。

独立方法：冷上下文 Codex/gpt-6.1-sol，requested reasoningEffort=high；主控核对当前 fallback 源码中的目录检查和 LookPath 错误分支，记录正式裁决。被评五文件候选为 `a49b9c0d0d0cd8558eb71ca95532a244f300b9d9a6524fe961f8173a1e8acfa6`。

### 📋 执行路径独立评审报告

📊 总体评分：6/10

✅ 评审结论：FAIL

#### 🔴 严重问题 — 必须修复

**CQP-R1-F1 — 高：fallback 的可替换祖先未被验证。**
位置：claude_exec.go 的 trustedClaudeExecutable。
仅检查安装目录和最终文件的 owner/mode，不能保证入口的父目录或符号链接目标的父目录不可被其他用户替换。主控检查源码确认没有相应路径链校验；Reviewer 的两个隔离 overlay 分别以 world-writable `.local` 和 world-writable 链接目标父目录复现执行成功（windows=2），预期应为 spawn 拒绝。
💡 有界修复：只在固定 HOME fallback 的入口及解析后目标范围验证到可信锚点的必要路径分量，拒绝其他用户可替换的链，并保留正常原生安装链接；不改 PATH 成功项或显式路径的既有语义。

**CQP-R1-F2 — 中：ErrDot 被 fallback 静默绕过。**
位置：claude_exec.go 的 LookPath 失败分支。
原 `exec.CommandContext` 会保留 `exec.ErrDot`；当前任何 LookPath 错误都尝试 HOME fallback，在 CWD 中存在相对 PATH 命令且另有有效 HOME 安装时，反而执行另一个目标。主控确认此分支未区分 not-found 与 ErrDot；Reviewer 隔离复现 LookPath=ErrDot、Probe=成功。
💡 有界修复：只有真正 not-found 才尝试 fallback；ErrDot 原样经现有包装传递并保持 spawn/probe_failed，补双候选共存回归。

#### 🟡 建议改进 — 推荐

无其他已验证发现。

#### 🟢 优点

原生安装发现与符号链接、PATH 优先、显式路径及失效不回退、原有失败分类均有回归。Reviewer 用原 HEAD 的 claude.go 外部 overlay 运行既有受限 PATH 测试，观察到找不到命令而失败，补齐了最小 baseline 失败证据；最终现有 ProbeClaudeProse 测试通过。

#### 📝 总结

两项均是当前范围内的实际缺陷，不能以通过现有测试代替关闭。Reviewer 在三个决定性失败场景后停止扩大验证，未操作真实 Claude/App/账号；全包/race/vet 沿用仍适用的原观察，不宣称完整 GUI 验收。交回同一 Developer 以更高思考档位做第 1 轮有界修复，仍无 Git 交付。

下一步指令：修复：fix / claude-quota-probe-gui-path / CQP-R1-F1, CQP-R1-F2

## 修复 — CQP-R1-F1 / CQP-R1-F2

第1轮有界修复，仍待独立复评。仅 `exec.ErrNotFound` 进入固定 HOME fallback，`exec.ErrDot` 原样经 spawn 包装传递；新回归同时放置相对 PATH 命令与有效 HOME 安装。祖先检查覆盖 HOME/.local、符号链接目标父目录、符号链接化安装目录和中间跳转；root/current-user 属主规则为通过系统父目录所需，sticky 例外有当前用户持有后继项的正例。安全检查仍局限固定 HOME fallback，不加载 shell 配置、不改全局 PATH。

主控直接核对五文件 Git blob，并把当前 SHA256 与原始命令输出的五个 SHA256 逐项比较，一致：

| 路径 | Blob |
| --- | --- |
| internal/quota/claude.go | 6fbf9fbd622c167809a7fe749d1ded4b193f2f05 |
| internal/quota/claude_exec.go | 151a6c50ef2f1c3bc39b74b15479b377842d3d72 |
| internal/quota/claude_exec_other.go | 1fbb894dce5e20a8a2c4712cdafaef15fe0cb10a |
| internal/quota/claude_exec_unix.go | dc66e5432e6a8af9251a95a103ff8c1efbc78679 |
| internal/quota/claude_test.go | 026732f9200315e752e0c99fae4a63b9337ec386 |

完整候选以同一 HEAD+五文件+go.mod/go.sum/vendor/modules.txt 的既有 recipe 计算，最终 SHA256 见本轮 CEv1 ContentState；上方初版身份保留作历史。

主控回读原始 T3 command_execution：修复前 ErrDot 双候选用例与六项可替换祖先用例均失败；修复后 Claude 相关用例45 PASS、0 FAIL，quota+desktop race 和完整 cmd wrapper 均明确报告 passed，vet、build退出0，gofmt无输出。不以管道最终退出码代替 wrapper 的实际 passed 输出。临时日志不是持久引用，不重复未变套件。

当前代码与测试被冻结，Developer 终态无后台子操作。没有执行真实 Claude、已安装 App、认证状态或完整 LaunchServices/GUI 验收；受限 PATH 假命令回归不冒称上述验收。非Unix fallback 拒绝信任；Windows 构建受既有 platform 限制，未声明支持。用户已授权评审/gate通过后提交、推送 origin、创建 PR，并要求 Codex review；无 merge/install/release/deploy。主控升级到 Astra 做冷独立复评，确认 root/sticky/符号链接边界与旧发现关闭；尚未通过、提交或推送。


## Review — Round 2

方法：新冷上下文Codex/Astra，requested reasoningEffort=xhigh；T3确认executedModel=gpt-6-astra。主控核对当前路径清理代码与原始overlay命令/输出后裁决，未重复相同探针。被评候选daa898704d90678e6238238d59d01b1b915f88154e10264dd95341482a7035a6，组成五文件、HEAD和依赖逐项匹配；Developer/Reviewer无pending。

### 📋 独立复评报告

📊 总体评分：6/10

评审结论：FAIL

完成门禁：FAILED

#### 🔴 严重问题 — 必须修复

**CQP-R2-F1 — 高：提前清理链接目标中的..导致验证和执行对象不同。**
位置：claude_exec.go对相对target使用filepath.Join，以及随后filepath.Clean(target)。若入口链接到safe/alias/../claude，alias再指向untrusted/subdir，词法清理验证的是safe/claude，实际OS解析却指向untrusted/claude。主控读到独立overlay真实输出：实际文件mode0777、trusted=true；ProbeClaudeProse返回windows2、err=nil，安全sentinel本应exit19。项目wrapper测试退出1，准确证明实际执行了未信任目标；不依赖竞态或真实CLI。
💡 有界整改：按真实解析顺序处理链接与..，验证经过的路径分量/链接和末端，保证验证对象与实际执行对象一致。保留root/current-user、sticky例外、链接跳数、PATH成功项及显式路径原语义，补确定性红绿回归。

#### 🟡 建议改进 — 推荐

无其他确认发现；决定性阻断后停止扩大验证。

#### 🟢 旧发现状态

CQP-R1-F2 CLOSED：只有ErrNotFound进入HOME fallback，ErrDot及其他查找错误保留spawn包装。
CQP-R1-F1 OPEN：R2-F1证明目录/链接信任仍可绕过，不能只凭六项旧祖先测试通过关闭；跟随本轮新finding一起修复。

#### 📝 总结

旧45项Claude、quota/desktop race、完整cmd、vet/build/gofmt结果按各自原状态保留，不构成这个路径反例的豁免。主控记录精确候选的独立评审失败后，gate为FAILED。隔离overlay已结束并清理，仓库只读、无后台操作；未验证真实LaunchServices/App，但这不是本轮FAIL原因。进入第2轮有界修复，原Developer上下文升级Opus/xhigh，不重置repair_count；未提交、推送或创建PR。


## 修复 — CQP-R2-F1

repair_count=2。原Developer上下文由Sonnet/high升级Opus5.5/xhigh；T3确认executedModel=claude-opus-5-5，模型无法观测实际effort，记录请求值与unknown。只改claude_exec.go与claude_test.go；HEAD、claude.go、两个owner helper与三项依赖不变。

新resolveTrustedClaudeCandidate从/按实际分量逐段走，遇链接将其原始target插入剩余分量前；绝对链接回/，..作用于实际到达的目录，不提前Clean。每次名字查找先检查当前真实目录；返回并执行验证得到的无符号链接真实路径。HOME构造同样不用filepath.Join抹去其..。符号链接8跳按整次解析计数，包括目录链接；原生正常相对链接、相对安装目录、两跳、循环/超长拒绝、sticky及可写祖先保护均有回归。fallback的argv0随真实版本路径变化，PATH命中和显式路径继续原语义，交复评核对。

主控直接读新walker并核对五文件blob与原始T3执行输出。真实红灯为只加测试时：SymlinkDotDotRejectsUntrustedKernelTarget仍执行0777目标；HomeWithDotDotFollowsKernelOrder走错sentinel并exit19；ReturnsVerifiedRealPath仍返回入口。正常可信KernelTarget/普通相对链接/循环及hop正反例旧代码已通过的部分仅是保护，不冒称红灯证据。

修后Claude相关wrapper_exit0，56 PASS/0 FAIL；quota/desktop race与完整cmd wrapper均打印go test passed且三包ok；zsh下PIPESTATUS空值如实保留，不编造数字退出码。wrapper43-44行仅在test退出0时打印passed并exit0。gofmt无输出/exit0、vet/build0，均为本轮新代码结果，不复用上一候选45例。未运行全仓./...测试、真实Claude/GUI/安装App或认证状态，无pending或遗留Go进程。

| 文件 | Blob |
| --- | --- |
| internal/quota/claude.go | 6fbf9fbd622c167809a7fe749d1ded4b193f2f05 |
| internal/quota/claude_exec.go | 6bb7775be374f9070ca6104289197c4b794e8fae |
| internal/quota/claude_exec_other.go | 1fbb894dce5e20a8a2c4712cdafaef15fe0cb10a |
| internal/quota/claude_exec_unix.go | dc66e5432e6a8af9251a95a103ff8c1efbc78679 |
| internal/quota/claude_test.go | 28a85c99e974315576400ca28e300049b16f424c |

完整指纹仍按已建立HEAD+五文件+go.mod/go.sum/vendor/modules.txt recipe重算并在新CEv1 ContentState登记。新方案继续依赖owner/mode权限模型，未评估macOS ACL；根属主只由系统目录间接覆盖，Windows仍为既有platform构建限制。这些限制及执行真实路径/总hop计数变化明确交冷独立复评，主控不以披露代替finding关闭。尚未自判PASS、提交或推送。


## Review — Round 3

冷独立OpenCode/Astra，requested variant=xhigh、实际variant unknown；被评候选ad22fbd1701a3976188003d1b34023f01042b1c4bf6523eed5b7add5b513c68e。主控已读mode快速接受分支并核对ACL授权可替换目录的反例，正式裁决如下。

评审结论：FAIL

完成门禁：NOT_VERIFIED

总体评分6/10。CQP-R3-F1 HIGH：Darwin目录mode0755且root/current-owner时直接可信，ACL仍可向everyone授予add_file/add_subdirectory/delete_child，允许其他主体替换bin目录。独立外部overlay实际添加此ACL，mode0755/trusted=true，Probe返回windows2/errnil，期待spawn拒绝的断言FAIL/wrapper退出1；未跨UID实施替换。主控源码检查确认当前未读取ACL，披露限制不能关闭原祖先不可替换保证。

CQP-R1-F1 OPEN；CQP-R1-F2 CLOSED（ErrNotFound限定）；CQP-R2-F1 CLOSED（真实逐分量与返回真实路径）。正常相对/绝对、HOME dotdot、sticky和hop等现有回归通过。此次probe为Go1.27.3 darwin/amd64；旧广泛记录环境按原记录保留，不冒称同一toolchain。

有界修复为Darwin HOME fallback纳入ACL：拒绝向其他主体授予目录条目替换、叶文件写入或改变安全属性的授权，不能可靠判定则拒绝；保留无害read/deny ACL，不改变PATH/显式路径语义。不跑真实CLI/账号/App。进入第3轮修复，复用Developer上下文升级Opus/max；累计次数不重置。真实gate由主控登记失败后更新此字段，不能把Review FAIL当已查询gate。

## 修复 — CQP-R3-F1

repair_count=3。Developer 为 Claude Opus 5.5（请求 effort=max，实际 effort unknown），沿用同一上下文。Darwin HOME fallback 在解析路径时先检查 `/`，再检查每个查找到的对象（每级目录、符号链接本身、最终文件）的 ACL：通过 `getattrlist` 读取对象自身（不跟随链接）的 ACL，经 libSystem 跳板（`claude_exec_darwin.s`，`//go:cgo_import_dynamic` 与 `//go:linkname` 接 `syscall.syscall6`）调用，不用 CGO、不调用外部命令、不改依赖。deny 条目放行；其他条目只有全部权限位属于只读集合才放行，新增/删除/替换条目、写内容、写属性或安全信息、改属主及未知权限位一律拒绝；不按授权主体、条目顺序或继承标志放宽；读取或解码失败拒绝。Linux 维持 owner/mode 规则；其他 Unix 的 HOME fallback 一律不放行。PATH 命中和显式路径不受影响。

Developer 报告修复前新增 ACL 回归 22 项中 19 项失败（`err = <nil>`，候选被执行而非拒绝），无害 deny/只读 ACL 正例在旧代码上已通过；修复后同命令 22 项通过。测试 ACL 仅加在新建临时对象上并以 `chmod -N` 清除。

## Review — Round 4

方法：冷独立 Codex，executedModel gpt-6.1-sol、reasoning effort=xhigh（运行时提供），与 Claude Developer 不同 harness，只读。被评候选为 HEAD `4ac56db32e13c4e925b648c620ba76fc588a7781` 上的九个代码/测试文件：

| 路径 | Blob |
| --- | --- |
| internal/quota/claude.go | 6fbf9fbd622c167809a7fe749d1ded4b193f2f05 |
| internal/quota/claude_exec.go | 36b68804f99eeeacca82ff455a852a7ced1cfdfc |
| internal/quota/claude_exec_darwin.go | 623633ab9f1c8dbe29214a0c37bc8fef3d464989 |
| internal/quota/claude_exec_darwin.s | b471fa81a39399262571f37b89d45a2300897b81 |
| internal/quota/claude_exec_darwin_test.go | 1944de7e557a9ebce19e6b9c572591f584b0240f |
| internal/quota/claude_exec_nodarwin.go | 615ea30823835269981f74ba49cbd760e5be3591 |
| internal/quota/claude_exec_other.go | 1fbb894dce5e20a8a2c4712cdafaef15fe0cb10a |
| internal/quota/claude_exec_unix.go | dc66e5432e6a8af9251a95a103ff8c1efbc78679 |
| internal/quota/claude_test.go | 28a85c99e974315576400ca28e300049b16f424c |

依赖不变：go.mod `1e0f4f9b0e7a3f954ece7c5fe5557395aa3482d3`、go.sum `7d3ff20d8956788792df612c96e9983b930f5400`、vendor/modules.txt `74dfff1173a03668c543099d546cdd269ed9d3df`。HEAD、九文件与三项依赖的 recipe SHA-256 为 `12dad84cbefe94e027e2e916d187d355e3b3a91f8d4e099418aed22b330033df`。代码文件由五个增至九个，前文「代码仅涉及」一句的文件数以本节为准。

评审结论：PASS

完成门禁：VERIFIED

总体评分 9/10。CQP-R3-F1 CLOSED：真实 `chmod +a` 回归覆盖目录替换、叶文件写入、安全属性修改、符号链接与 sticky/delete_child，均拒绝。CQP-R1-F1 CLOSED：逐级 owner/mode 与 ACL 检查共同覆盖祖先、链接和目标。CQP-R1-F2 CLOSED 保持：仅 `exec.ErrNotFound` 进入 HOME fallback，ErrDot 仍可由 `errors.Is` 识别。CQP-R2-F1 CLOSED 保持：逐分量处理链接与 `..`，返回验证后的真实路径，总跳数上限 8。无新发现。

披露点裁决：其他 Unix 一律拒绝 HOME fallback 接受，属新增回退能力的可用性收缩，当前支持平台为 macOS；`.s` 文件接受，有正确平台约束，仅为 libSystem 跳板；linkname/动态导入接受并保留维护风险，Darwin amd64/arm64 在 `CGO_ENABLED=0 -trimpath -ldflags='-s -w'` 下链接成功，Reviewer 环境为 Go 1.27.1；有意误拒（忽略主体、deny 顺序与 inherit-only）接受，属保守拒绝；ACL 解码对照本机 SDK 核对头部 44 字节、ACE 24 字节、计数偏移 36、最大 128 项与权限位，检查 magic、精确长度与引用边界，未知位拒绝，未发现把写授权解码为无害的反例。

Reviewer 实际运行：`scripts/run-go-test.sh -count=1 ./internal/quota -run 'Claude|DarwinACL' -v` 退出 0；darwin/linux × amd64/arm64 的 `go vet ./internal/quota` 各退出 0；darwin amd64/arm64 完整 CLI 发布式构建各退出 0；linux 完整 CLI 构建退出 1，原因为 HEAD 未改动的 `internal/installation/install.go` 使用 Darwin 专有 `unix.RenamexNp`，`git diff --exit-code HEAD` 确认该文件不属于候选；linux `./internal/quota` 构建退出 0。

主控核对 blob 与 recipe 一致，并在 Go 1.27.3 darwin/amd64 上直接运行 `scripts/run-go-test.sh -count=1 ./internal/quota`（wrapper 报告 go test passed，退出 0）、`gofmt -l internal/quota`（无输出）与 `go vet ./internal/quota`（退出 0）。Developer 报告的 quota/desktop race、完整 cmd 包与 arm64 测试二进制编译按其原状态保留，不冒称主控重跑。arm64 未在真实 arm64 主机上执行测试，留给 CI 的 macOS runner。未执行真实 Claude、LaunchServices 或已安装 App 验收；gui-environment 证据来自隔离 HOME 与受限 PATH 的假命令回归。

Task checkpoint：主控在上述 ContentState 记录 failure-first、regression、gui-environment、independent-review 四项 CEv1 证据并重新查询 gate。按用户授权签名提交本 issue 的代码、测试与本记录，推送 origin 并创建单独 PR；Actions 全绿后请求 Codex review。原 Bug 不因本地验收关闭。
