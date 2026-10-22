# SP-IP-R7-F1 修复候选交接

工作区 `agent-deck.scan-performance`，分支 `feature/scan-performance`；同一 Beads
`ad-scan-performance-incremental-projections`。修复范围仅 Round7 记录的 fork 成员闭合。
候选已完成，待未产出本修复的独立角色复评；未自判 Review PASS。

四个变更文件：scan_performance_p07.py、p07_rss_support.py、p07_rss_support_test.py、
p07measure/README.md，均位于 scripts 下。核对 [修复内容清单](incremental-update-repair-r7-content-states-20261021.json)、
[压缩差异](incremental-update-repair-r7-delta-20261021.patch.gz)、
[结果](incremental-update-repair-r7-result-20261021.json) 和
[门禁](incremental-update-repair-r7-gate-20261021.json)。差异的旧文件 SHA256 与 R7 评审候选一致。

新仪表 `scan-ip-bounded-instrumentation-a1f70d08e822de14`；产品
`scan-ip-bounded-product-e14fca302e5e8572` 和编排
`scan-ip-bounded-orchestration-067e1bcc6aee7761` 完全未变，适用验证与技术证据复用。

fork 事件不再只留日志：与晚登记共同触发三次上限的同帧子树重查和新 resident 采集。
Darwin NOTE_FORK 只有父身份且可能合并，无法排除已逃逸后代；所以即使补到了可见
子进程，也不把枚举稳定或一个已知退出当完整证明。未知 fork、最终 drain 晚事件或
预算耗尽均使帧/trace 无效；首条未知事实以常量状态保留到后续帧，不恢复虚假完整。
这是 fail-closed 修复，不是正式 RSS 通道或性能验收通过。

原评审确定性复现修后采到1/2/9，合成RSS合计正确，complete=false；原复现文件不改。
两个受影响 Python 模块33项全部通过，含新增8项确定性与1项原生 fork 控制。
命令：`PYTHONPATH=scripts AGENTDECK_P07_NATIVE=1 python3 -B -m unittest -v p07_rss_support_test scan_performance_p07_test`。
原生进程检查使用 require_escalated，完整私有 stdout/stderr 和退出码保留。没有真实
HOME/cohort 访问、GUI、点击、P10 或代表性性能采样。不存在 Go 改动，无需重跑 Go/App UI。

P07 完整事务384.921506/285.331956ms反例与旧无效RSS保持原义，精确产品门仍FAILED。
三项技术准则复用，update-latency未满足；不关闭 Task3。R7原FAIL、Round6原PASS保留。
修复确认后完成 Round7 原范围内剩余新增差异审查，复用已经核对且未变的部分。
保留用户撤销点击/P10计时的决定，不恢复任何相关采集工程。

L0已核对新增源码、当地文档链接及限定diff。全局whitespace仍只有旧冻结patch的31个
空上下文行，退出2；原patch摘要未变，本修复不改它，也不把此检查报为通过。

```text
复评：scan-performance / incremental-projections / SP-IP-R7-F1
WORKFLOW_WORKSPACE: agent-deck.scan-performance

按 reviews/incremental-projections.md Round7 及本修复附记，先独立复评R7-F1，
再按Round7原范围完成剩余新增差异审查；不重复Round6或未变验证。
关注有界同帧重查、父/子birth、未知fork跨帧无效、最终drain和预算耗尽。
保留P07 max FAIL、旧无效trace及点击/P10计时撤销决定。
不委派，不进入Tasks4–7，不提交、推送、安装、发布或部署。
```
