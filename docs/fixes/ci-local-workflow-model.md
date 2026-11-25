---
status: active
created: 2026-11-24
---

# 缺陷：CI 工作流测试的本地 runner 模型不支持 pwsh、混合插值，空映射与 env 大小写语义偏差

协调载体：`ad-bug-ci-local-runner-pwsh-shell`、`ad-bug-ci-local-expression-interpolation`、`ad-bug-ci-local-yaml-empty-map-env-case`；版本：v0.7.5；Lane A。
工作区：`agent-deck.fix.ci-local-workflow-model`，分支 `fix/ci-local-workflow-model`，基线 HEAD `3f7f78aaf65f0cabf434130ab670f9e9d27100ce`。

## 现象

`fix:ci-duplicate-runs-and-macos-concurrency` Round 7 留下三项 P3（CI-DUP-R7-F1..F3），当时按用户 2026-11-23 的决定延后。2026-11-24 用户决定在 v0.7.5 内一并修复。三项都在 `scripts/ci/docs_tools_test.py` 的本地 GitHub runner 模型里，它只用于测试 `.github/workflows/*.yml`，不影响产品：

1. 内置 shell 列表缺 `pwsh`，遇 `shell: pwsh` 时 fail-closed 报错。
2. `resolve_github_expression` 只支持整串单个 `${{ }}`，不支持文本与多段表达式混排。
3. Psych AST 的裸空映射是 `Scalar("")`，使用时抛 `AttributeError`；`env` 上下文一律 casefold，与非 Windows runner 大小写敏感的语义不一致。

## 根因

本地模型只实现了当时工作流用到的语法子集：shell 表缺 `pwsh`，表达式求值器假定整串为单个表达式，YAML 转换未区分裸空值，上下文路径解析对所有上下文统一 casefold。

## 修复边界

只改 `scripts/ci/docs_tools_test.py`，不改任何工作流：

- `pwsh` 内置为 `-command ". '{0}'"`，脚本扩展名 `.ps1`；未带 `{0}` 的自定义 shell 仍 fail-closed。
- 表达式支持文本与多段 `${{ }}` 混排；扫描结束符时识别单引号字符串与 `''` 转义；未闭合、空表达式、未知函数与不可用上下文仍 fail-closed。`runner` 不进入表达式上下文白名单。
- 只把映射条目里无标签、无锚点、未加引号且为空的值归一化为 `{}`；显式标签、锚点、序列空元素、空文档与仅注释文档保持基线行为；`env` 中的映射或列表值 fail-closed。同一层 `env` 对进入该层时的快照求值。
- runner OS 只作求值器内部参数：显式 OS 与 Windows 标签只做大小写归一化后精确比较；Windows 标签为官方白名单（`windows`、`windows-latest`、`windows-2025`、`windows-2022`、`windows-2019`、`windows-2025-vs2026`、`windows-11-arm`、`windows-11-vs2026-arm`）；`runs-on` 先完成结构校验（对象只允许 `group` 与 `labels`，`labels` 须为字符串或字符串列表，`group` 须为非空有效组名）并对全部标签求值，再应用显式 OS 或推断结果；步骤 `if` 经真实调用点获得 OS。

## 验证

作者在每轮修复中先写回归，使其在上一候选上失败，再修复。最终候选（修复轮 4）SHA-256 `3d98651a3bab9fd590a100e4f96fd5006d7bde7b11bb2f2adfec8422d160308d`（`git diff --full-index 3f7f78a`）：`python3 scripts/ci/docs_tools_test.py` 52 项、`make check-ci-docs-tools` 中 scripts/ci 80 项与 scripts/hooks 93 项通过，`git diff --check` 干净。五份现有工作流与基线 `3f7f78a` 对比：108 个触发判定、27,972 个 `if` 判定一致，85 个命名步骤的运行设置除新增 `runner_os` 外无差异；解析树仅 `ci.yml` 的 `pull_request` 与 `cask-real-brew.yml` 的 `workflow_dispatch` 两处裸空值由 `''` 变为 `{}`。

## Review — Round 1

评审结论：FAIL

完成门禁：NOT_VERIFIED

Codex `gpt-6.2-sol` low 冷评审（作者 AGY Gemini 4 Argon high）。F1 PASS（与 actions/runner 源码一致）；F2、F3 FAIL。五项 P3：`runner.os` 被放进表达式上下文；同层 `env` 求值依赖键顺序；显式 `env: ""` 被当作 `{}` 接受；步骤 `if` 丢失 runner OS；按任意标签子串推断 Windows。

## Review — Round 2

评审结论：FAIL

完成门禁：NOT_VERIFIED

修复轮 1 由 AGY Gemini 4 Hydrogen low 完成，Codex `gpt-6.2-sol` medium 复评。`runner` 上下文、同层快照、步骤 `if` 关闭；仍开放：空值归一化过宽（`!!str`、嵌套空映射、`env` 中的映射/列表、空文档），Windows 识别漏掉 `windows-11-arm` 等官方标签与对象形态 `labels`、显式 OS 按子串匹配；步骤 `if` 测试未走真实调用点。

## Review — Round 3

评审结论：FAIL

完成门禁：NOT_VERIFIED

修复轮 2 由 AGY Gemini 4 Neon medium 完成，Codex `gpt-6.2-sol` medium 复评。仍开放：带锚点的空值被归一化；正则接受 `windows-2025-tools`；`labels` 类型校验可绕过且首个 Windows 标签提前返回。

## Review — Round 4

评审结论：FAIL

完成门禁：NOT_VERIFIED

修复轮 3（改为官方 Windows 标签白名单）由 AGY Gemini 4 Neon 完成，Codex `gpt-6.2-sol` medium 复评。仍开放：调用方传入显式 OS 时跳过 `runs-on` 校验；空串、纯空白或表达式 `group` 落入默认；匹配前 `strip()`。三轮修复已满额，用户授权额外第 4 轮，并指示若其后只剩更细的同类边界则以 Beads 载体收尾。

## Review — Round 5

评审结论：PASS

完成门禁：VERIFIED

修复轮 4 由 AGY Gemini 4 Neon 完成，Claude Sonnet 5.5 medium 冷复评（与作者不同 harness；模型与 effort 经 Decisions 选择）。

- **关闭项。** 显式 OS 在 `runs-on` 结构校验、group 求值与全部标签求值之后才生效，`runner.os` 表达式在任何显式 OS 下仍被拒绝；`group` 空串、纯空白、求值为空或含非法上下文均 fail-closed；Windows 标签与显式 OS 只做 casefold 后精确比较。`runs-on` 对象只允许 `group` 与 `labels`，与 GitHub 语法一致；`[self-hosted, Windows, ARM64]` 识别为 Windows。此前各轮已关闭项无回退。
- **验证。** 补丁与工作树一致；52 + 80 + 93 项通过；五份工作流与基线判定一致；新回归在上一候选 `90332b02` 上 15 处失败；四个变异中三个被拦截，一个为等价变异（见 F4）。
- **非阻断 P3。** 均只影响本地测试模型的边界输入，按用户对第 4 轮的指示延后：CI-LOCAL-R4-F1 空 `runs-on`（`[]`、空串、空 `labels`）被当作 Linux 接受；CI-LOCAL-R4-F2 `runs-on` 求值透传 `env`、`steps` 等 GitHub 不允许的上下文；CI-LOCAL-R4-F3 显式 OS 键名大小写与空值处理不一致；CI-LOCAL-R4-F4 `runner_labels` 的空 group 检查缺直接单测 -> open `ad-bug-ci-local-runs-on-edge-validation`。
- **门禁。** CEv1 WorkUnit `fix:ci-local-workflow-model` 在该候选上独立评审与本地验证满足。托管 CI 结果由 PR 检查验证。

## Review — Round 6

评审结论：PASS

完成门禁：VERIFIED

PR 84 的 documentation 检查在 Linux runner 上失败：`test_pwsh_builtin_shell_uses_dot_source_command_and_ps1_extension` 的 `PWSH` 子用例报 `FileNotFoundError: workflow shell not found: PWSH`。测试只创建了 `pwsh` 桩，macOS 文件系统大小写不敏感所以本地通过。本地模型按原样大小写解析可执行文件、按小写匹配内置参数，与 actions/runner 的 `ScriptHandler`、`WhichUtil`、`ScriptHandlerHelpers` 一致，产品模型无需改动。主控只改测试：额外创建 `PWSH` 桩副本（大小写不敏感时已存在则跳过）。

Codex `gpt-6.2-sol` low 冷评审（作者为主控，档位与 effort 经 Decisions 选择）PASS，无发现：在真实大小写敏感的 HFS+ 卷上基线复现该错误、候选通过；原有子进程、退出码、`.ps1` 内容与点源格式断言保持；`scripts/ci/` 下未发现其他依赖大小写不敏感的测试。`docs_tools_test.py` 52 项、`make check-ci-docs-tools` 80 + 93 项通过，`git diff --check` 干净。候选 SHA-256 `9a04ec71936750fce748a1ad835597401ee9ef545ce8ace5ddf968b71d1dda85`（`git diff --full-index 3f7f78a`，取于追加本轮记录之前）。CEv1 WorkUnit `fix:ci-local-workflow-model` 已在该候选上补记证据。

## Review — Round 7

评审结论：FAIL

完成门禁：NOT_VERIFIED

用户在 PR 84 合并确认时指示 Round 5 延后的四项也要修掉（载体 `ad-bug-ci-local-runs-on-edge-validation` 重新打开）。修复轮 5 由 AGY Gemini 4 Hydrogen medium 完成：空 `runs-on`（`[]`、空串、空 `labels`）报错；`runs-on` 求值上下文限于 GitHub 允许集合；显式 OS 只认规范键且值须为非空字符串；补 `runner_labels` 空 group 的直接单测。Codex `gpt-6.2-sol` medium 复评关闭四项字面量情形，新发现：表达式结果为数组/对象时先被字符串化后才做非空检查；显式 `runner_os` 参数提前返回、跳过上下文校验；`runs-on` 接受 `success()` 等状态函数。

## Review — Round 8

评审结论：FAIL

完成门禁：NOT_VERIFIED

修复轮 6 由 AGY Gemini 4 Neon high 完成，改为显式策略表：19 个 workflow 键各自声明允许的上下文、特殊函数与结果类型，求值按调用点查表，未列组合 fail-closed；表达式结果按原始类型校验后再字符串化；OS 元数据先校验后应用优先级；`runs-on` 与非 `if` 键拒绝状态函数。Codex `gpt-6.2-sol` high 复评逐行对照 GitHub 官方上下文可用性表，未发现越权放行；新发现：`release.yml` 证书与签名步骤引用的 `secrets.*`、`runner.temp` 无法注入假值（基线既有限制）；带策略求值静默丢弃未知调用方上下文键（回退）；策略矩阵测试从被测表读取预期。

## Review — Round 9

评审结论：FAIL

完成门禁：NOT_VERIFIED

修复轮 7（同一 AGY Neon 线程）：测试可显式注入假 `secrets` 与 `runner.temp`，`runner.os` 仍只作内部元数据；12 个求值入口一致拒绝未知调用方上下文键；策略矩阵测试独立声明 19 行预期。Codex `gpt-6.2-sol` high 复评新发现：部分 fixture 缺字段时回退为空串；已知上下文在带/不带策略入口处理不一致；`runner` fixture 可扩展未建模字段。用户随后设定止点：再修一轮，之后若复评只报测试工具 fixture 接口类更深的 P3，则合并并以 Beads 载体收尾。

## Review — Round 10

评审结论：PASS

完成门禁：VERIFIED

修复轮 8（同一 AGY Neon 线程），候选 SHA-256 `c09b6558a4d8d51adce113400af99978c19f6c973a92d327fbe575bd4b3c5682`（`git diff --full-index 3f7f78a`，取于追加本轮记录之前）：引用 `secrets.<name>` 或 `runner.temp` 时须显式注入非空字段；所有入口先经同一函数校验调用方键再按字段策略取值；表达式只开放 `runner.temp`。Codex `gpt-6.2-sol` high 冷复评（与作者不同 harness；effort 经 Decisions 选择）。

- **关闭项。** Round 9 三项关闭；F1–F3、P3.1–P3.5、PWSH 大小写桩（真实大小写敏感 HFS+ 卷）、R4–R7 各项回归通过；策略表 19 行与 GitHub 官方表一致。
- **验证。** `python3 scripts/ci/docs_tools_test.py` 59 项、`make check-ci-docs-tools` 87 + 93 项通过，`git diff --check` 干净。五份工作流 108 个触发与 27,972 个 `if` 判定与 `3f7f78a` 一致；79 个命名步骤运行设置除新增 `runner_os` 外一致，另 6 个引用 `secrets` 的步骤在完整假值 fixture 下可解析。新回归在上一候选 `ac53807d` 上 13 处失败、7 处错误；评审者独立变异两项均被拦截。Round 5 的 `ad-bug-ci-local-runs-on-edge-validation` 四项已在修复轮 5–8 关闭。
- **按用户止点规则延后的 P3（均为 B 类：测试工具调用方 fixture 接口严格度，A 类与 C 类为 0）。** CI-LOCAL-R8-F1 直接求值入口在禁用表达式的字段上先查字段策略、后校验调用方 fixture 键，与插值入口错误优先级不一致；CI-LOCAL-R8-F2 `secrets` fixture 接受 casefold 后重名的键，取值依赖插入顺序 -> open `ad-bug-ci-local-fixture-contract-edges`。
- **门禁。** CEv1 WorkUnit `fix:ci-local-workflow-model` 在该候选上补记独立评审、本地验证与范围证据。托管 CI 结果由 PR 检查验证。
