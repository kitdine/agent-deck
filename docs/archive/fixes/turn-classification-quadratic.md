---
status: historical
retired: 2026-11-27
created: 2026-11-17
---

# 缺陷：长会话 turn classification 重复扫描

载体ad-bug-turn-classification-quadratic，v0.7.5，Lane A；工作区agent-deck.fix.turn-classification-quadratic，分支fix/turn-classification-quadratic。实现已完成并通过独立源码评审，最终门禁及交付由主控负责。

## 现象与根因

历史长会话诊断发现classifySourceTurns逐turn的EXISTS反复通过(client,session)索引遍历同会话事件。历史25分钟扫描不是本轮测量。新测试保留HEAD原1281字节SQL作为oracle，真实SQLite计划确认旧查询存在CORRELATED SCALAR SUBQUERY和会话索引探测。

## 修复边界

sourceTurnShapesSQL新增MATERIALIZED assistant_turns，先按source选去重client/session，再按client/session/turn聚合usage_events并LEFT JOIN。NULL turn不作为assistant信号，多事件不复制行，跨source事件/工具归属保留。工具聚合、分类规则及幂等写回不改。没有持久索引、schema迁移或状态/override/exclusion契约变化，core schema仍35。GROUP BY及临时索引有构建成本，不声称严格O(N)。

## 内容身份与验证

HEAD4ac56db32e13c4e925b648c620ba76fc588a7781；signals.go blob fac10a35d863db65ca07dddb3e842deda9632c94；新增source_classification_scale_test.go blob8573e6790838e1640e97a8eaf1845c13f9a0c0d5。新增未跟踪测试计入完整HEAD+两文件+go.mod/go.sum/vendor/modules.txt recipe，由CEv1保存，不用tracked diff代替身份。

主控读当前生产SQL和完整回归文件，并直接回读OpenCode原始测试输出：600turn对照、重跑幂等、旧/新plan、逐turn reference四项均PASS，package4.793s；plan为SEARCH e USING INDEX usage_events_client_session和SEARCH t USING AUTOMATIC COVERING INDEX三元键LEFT-JOIN，无CORRELATED或SCAN t。测试先断言旧计划确有退化，再拒绝新计划的逐turn全扫描，不设timing CI阈值。

Developer报告的500/2000/8000turn基准作为补充观察，不依赖临时原始文件证明验收。先前full usage race默认10分钟超时保留，不冒称首跑通过；主控按Makefile -race -p 1 -timeout=20m完整运行usage包，原始完成输出PASS、ok616.434s，受检两blob与当前一致。usage/activity/store普通回归及定向race52、vet/whitespace由原交接记录保留；没有阶段变化重跑广泛suite。新独立执行环境为Go1.27.3 darwin/amd64，旧说明中工具链数字不冒称本轮实际执行证明。

## Review — Round 1

方法：冷独立OpenCode/Astra，requested variant=xhigh，actual variant/effort unknown，T3未暴露executedModel。独立来源与Claude/Sonnet Developer不同harness，当前OpenCode执行正常。主控核对源码、两个blob、原始四测试/plan输出及最终完整usage race结果后裁决。

### 📋 独立评审报告

📊 总体评分：9.5/10

评审结论：PASS

完成门禁：VERIFIED

#### 🔴 严重问题 — 必须修复

无，不生成虚构TCQ-R1-Fn。

#### 🟡 建议改进 — 推荐

无已证实范围内低级缺口。

#### 🟢 优点及核对

原SQL精确oracle、600turn合成fixture覆盖缺assistant、NULL、重复事件、client/source隔离及工具形状；分类写回等价并可重复执行。真实plan移除逐turn会话事件重探测，GROUP BY使SQLite给物化t建立automatic covering index，无新持久索引。查询路径仍按client/session/turn，不能为了性能偷偷按event source过滤而改归属。主控已核对这些边界。

#### 📝 总结与Task checkpoint

结论绑定上述两文件及vendored SQLite；未用真实用户DB/转录/安装App验收，没增加源码/数据库写入。Reviewer只读，无子代理、后台或pending。OpenCode旧usage-limit观察不再作不可用断言；若未来再次限额，保留任务并向用户询问是否更换具体connect。

Task提交建议为两个Go文件及本记录，同一issue独立提交。用户已授权review/gate通过后签名commit、push origin、创建PR；push/PR Actions全绿后再发GitHub Codex review。无merge、release或install授权，不因源码PASS提前关闭原Bug或宣布版本完成。

## Task checkpoint — 2026-11-17

主控在被评候选上记录 CEv1 证据并重新查询：ContentState `3f8e87e1f775251b6fe2935ec2db56d2f4d0546c3541966f49d56f4b5c3b8e38`（HEAD、上述两文件及 go.mod/go.sum/vendor/modules.txt 的 recipe）下 failure-first、scaling、regression、independent-review 四项 required criterion 均为 pass，gate VERIFIED 4/4。本 A 修复无 containing topic gate。按用户授权进入签名提交、推送和单独 PR；原 Bug 不因本地验收关闭，等待远端 Actions 与 Codex review。
