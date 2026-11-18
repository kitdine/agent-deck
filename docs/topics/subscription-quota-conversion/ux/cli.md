---
status: active
created: 2026-11-18
---

# 会话用量和配额估值 CLI

## 入口与输出位置

- `agentdeck session show <id> --client <client> --tokens` 的 Tokens 区在现有费用下方给出 `Quota valuation`，读取已存配对。`--page` 仍只改变调用列表，配额项是完整会话或明示片段结果。交互 Tokens 页沿用同一只读结果，既有 text/JSON 和交互组合限制保持适用。
- `agentdeck quota sample start|finish|cancel --session <id> --client codex|claude` 是独立的显式操作。支持全局 `--format json` 或 text。探测动作仅用于 `start` 和 `finish`，沿用已启用、官方路由和认证门禁。`cancel` 清除对应的进行中采样。不存在的会话用统一 `session_not_found` 失败路径。参数或格式错误为 exit 2，运行错误和动作拒绝为 exit 1，成功或幂等重放为 exit 0。失败在 stderr 产生既有错误格式。
- `start` 成功输出 `Measurement started` 和首端点时间；重复同会话返回已开始状态并保留端点。已有另一会话时显示 `Another measurement is active`。`finish` 显示完成和各窗口状态，无首端点时返回 `Start a measurement first`；已知仍有未入账或未完成用量时返回 `Usage is still settling`，可重试。结束后的重复调用返回已完成样本。`cancel` 可重放，直接确认清除状态。

采样入口选择已知会话，首次创建会话之前会话 ID 不可用。会话此前已有用量时显示 `Measured portion` 和对应的时段与片段金额。顶部金额仍为全会话结果。[现有 CLI](../../../specs/cli-manual.md) · [边界与状态](../architecture.md)

```text
API equivalent USD 6.00
5-hour             4 pp observed · about USD 150 if this session used all of it
Weekly             2 pp observed · about USD 300 if this session used all of it
```

```text
API equivalent USD 6.00
Weekly             4 pp observed · other usage detected · estimate unavailable
```

```text
API equivalent Partial · USD 2.40 known
5-hour             No comparable start reading
Weekly             Reset during measurement · estimate unavailable
```

```text
Quota valuation    Probe disabled
Quota valuation    Measurement in progress · finish to see window changes
Quota valuation    No saved measurement · prior readings cannot be paired
```

```text
API equivalent USD 18.00
Measured portion  14:05–14:21 UTC · USD 6.00
Weekly · codex    4 pp observed · USD 120–200 if this session used all of it
Weekly · review   No comparable finish reading
```

```text
API equivalent USD 6.00
Weekly             4 pp observed · about USD 150 based on reported percentages
                   Account unverified · assumes the same account and no other usage
```

窗口名称、变化、是否有估值在窄列先出现，假设及来源置于详情。所有生产变化使用 `observed`。精度已证时显示区间和取整假设；未知时只说明按报告百分比估算。确证无法配对时只展示实际可用读数与原因。

## 数据需求

| 表面字段 | 需要的数据 |
| --- | --- |
| API 等价、已知小计 | 会话金额、口径、币种和价格覆盖。 |
| 测量状态、片段及时间 | 采样状态、与选中会话匹配的样本、上下界和本窗分子。 |
| 窗口读数与变动 | 窗口键、显示名/池、前后时间和来源、两端百分比、可比较性和精度。 |
| 假设、估值及缺口 | 账号关系、入账假设、可能竞争用量、同组分子、点估及上下界、原因。 |

JSON 在 `data.usage.quota_valuation` 带可选字段。金额字符串、整体状态、逐窗 `observed_delta_pp`、`estimate`、`assumptions`、`reason` 和端点时间映射到 [架构字段](../architecture.md#输出)。直接采样的成功 envelope 带动作、状态、起止和匹配结果。测试覆盖只读路径的无网络与写入、每种样例和 `start`、`finish`、`cancel` 的参数及退出码。
