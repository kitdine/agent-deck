---
status: active
created: 2026-11-18
---

# CLI 计价展示

沿用现行 `usage summary`、`usage stats`、`usage sessions`、`session show` token 详情和 `price list`。默认只展示解释当前金额所需的最少项。调用详情可复算；JSON 扩展字段可选且与旧金额字符串、`null` 和覆盖字段兼容。[现行 CLI 手册](../../../specs/cli-manual.md) · [计价契约](../architecture.md)

以下标本使用构造数据及 [需求价卡](../requirements.md#可复算验收)。主金额优先为完整 API 等价，再展示已知小计，随后是完整参考金额。渠道估算读取 `pricing.provider_estimate`，和套餐状态独立呈现。既有 JSON 读者保留旧字段，`provider_cost` 仍表示旧估算口径。

```text
API equivalent USD 3.512016
Provider estimate USD 3.512016
Billing path     Official API key
Pricing          gpt-6.2-astra · Standard · Long context
```

```text
API equivalent   USD 1.856
Provider estimate USD 2.32
Billing path     Proxy · 1.25x at request time
```

```text
API equivalent   Partial · USD 1.656 known
Missing evidence Cache write TTL
Reference        USD 1.856–2.056 · 30m/12h cache writes at Standard
```

部分金额标本输入恰有 272,000 输入、100,000 缓存读、20,000 缓存写和 10,000 输出。可证的普通输入为 152,000。写入只可能为 30m 或 12h，候选价格为 10 和 20 USD / 百万 token，因此区间仅展示于详情。

```text
API equivalent   Unavailable · Reviewer model unresolved
Plan status      Auto-review · No extra charge · No quota used
```

```text
API equivalent   USD 3.51 · Recorded requests
Coverage         Codex approval review usage may be missing
```

```text
API equivalent   Unavailable · Context pricing scope unverified
Reference        USD 1.360005 · GPT-5.4 · 272001 uncached input · Request-level context assumed
```

```text
API equivalent   Unavailable · Executed tier unverified
Reference        USD 3.712 · Same short Astra usage · Requested Fast tier assumed
```

```text
API equivalent   USD 0.00 · Verified API rate
Pricing          Explicit zero rate · Applicable interval verified
```

```text
API equivalent   Reference USD 0.84 · Prices as of 2026-11-18
Pricing          Historical rates unavailable
```

```text
API equivalent   Unavailable · Historical tariff unavailable
Provider estimate Unavailable · Billing path unverified
```

```text
API equivalent   USD 1.35 · GPT-6.1 Sol · Standard · Long context
Plan status      Included · No subscription long-context surcharge
```

```text
Invocation · gpt-6.2-astra · Standard
Input          152000 × 8.00 / 1M     USD 1.216
Cache read     100000 × 0.40 / 1M     USD 0.040
Cache write     20000 × 10.00 / 1M    USD 0.200 · 30m
Output          10000 × 40.00 / 1M    USD 0.400
API equivalent                        USD 1.856
Pricing rule   Astra Standard · ≤272000 input · OpenAI API price table
```

调用明细另列原始与实际模型、事件时间、渠道/执行档依据、适用区间及来源。CLI 文本使用现行英文和宽度规则；JSON 的 `pricing` 与金额保持各自可空，完整原因与参考依据见 [架构字段](../architecture.md#json-与原生-wire)。参考按 `cache_write_ttl_range`、`per_request_context` 或 `requested_tier` 列出实际假设。期间级 `Coverage` 提示为 `activity_coverage_unavailable`，与金额栏独立。验收窄 TTY、非 TTY、分页及 CLI/桌面金额一致性。
