import { ChartBar, ChartPieSlice, ClockCounterClockwise, Gauge, ShieldCheck } from "@phosphor-icons/react";
import { meta, quota, quotaMeta, rhythm, scope } from "./data.js";
import { catalogs, formatCost, formatDate, formatHourRangeShort, formatShare, formatTokens } from "./i18n.js";
import { StageControls, useStagePrefs } from "./Stage.jsx";

// 尺寸按真机比例：small 正方，medium 与 small 同高、约两倍宽，large 与 medium 同宽、约 2.2 倍高。
// large 不再是横跨整行的宽条——那个比例真机上不存在，会让人高估它能放下的内容。
// 桶数是 ux/widget.md:133-135 的规格，不是随手取的：small 7、medium 20、large 90。
// 故意不导出：contract.js 另抄一份文档的数字去断言，两边各自独立写下同一个规格，
// 改错了才会对不上。共用一个常量的话，改常量会同时挪动渲染和期望，断言就永远为真。
const SMALL_BUCKETS = 7;
const MEDIUM_BUCKETS = 20;

const KINDS = [
  { key: "magnitude", Icon: ChartBar },
  { key: "composition", Icon: ChartPieSlice },
  { key: "trust", Icon: ShieldCheck },
  { key: "rhythm", Icon: ClockCounterClockwise },
  { key: "quota", Icon: Gauge },
];

function Frame({ kind, size, lang, scopeText, children, Icon, footText }) {
  const dict = catalogs[lang];
  return (
    <article className={`widget widget-${size}`} aria-label={`${dict.widgets.kinds[kind].title} · ${dict.widgets.sizes[size]}`}>
      <header>
        <span>
          <Icon size={12} weight="fill" />
          {dict.widgets.kinds[kind].title}
        </span>
        <small>{scopeText}</small>
      </header>
      <div className="widget-body">{children}</div>
      <footer>{footText ?? dict.status.justNow}</footer>
    </article>
  );
}

function MiniBars({ values, tone = "info" }) {
  const max = Math.max(...values, 0.0001);
  return (
    <div className="mini-bars">
      {values.map((value, index) => (
        <i key={index} className={`tone-${tone}`} style={{ height: `${Math.max(3, (value / max) * 100)}%` }} />
      ))}
    </div>
  );
}

// 90 天在 286px 宽里画成柱子每根只有 3px，会糊成栅栏，所以 large 用面积曲线
function AreaChart({ values, height = 62 }) {
  const max = Math.max(...values, 0.0001);
  const step = 100 / (values.length - 1);
  const points = values.map((value, index) => [index * step, 100 - (value / max) * 100]);
  const line = points.map(([x, y], index) => `${index === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
  const area = `${line} L100,100 L0,100 Z`;
  return (
    <svg className="area" viewBox="0 0 100 100" preserveAspectRatio="none" style={{ height }} aria-hidden="true">
      <path className="area-fill" d={area} />
      <path className="area-line" d={line} vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function WRow({ label, value, share, tone, lang, dot }) {
  return (
    <div className="w-row">
      <div>
        <span>
          {dot && <i className={`dot tone-${tone}`} />}
          {label}
        </span>
        <b>{value}</b>
        <strong>{formatShare(share, lang)}</strong>
      </div>
      <div className="w-track">
        <i className={`tone-${tone}`} style={{ width: `${Math.max(share, 1.5)}%` }} />
      </div>
    </div>
  );
}

function WStats({ items }) {
  return (
    <div className="w-stats" style={{ "--columns": items.length }}>
      {items.map((item) => (
        <div key={item.label}>
          {/* qualifier 归到标签那一行：large 已经顶到 330 pt，chip 再加一行就溢出；
              而值那一行放不下完整日期——"$11.89" 36 pt 加 "8月17日" 34 pt 超出 72 pt
              的内容宽度会被 ellipsis 吃掉。标签行只用 61 pt，日期能保持完整格式。 */}
          <span>
            {item.label}
            {item.sub && <em>· {item.sub}</em>}
          </span>
          <strong>{item.value}</strong>
        </div>
      ))}
    </div>
  );
}

function Heat({ lang, compact = false }) {
  const dict = catalogs[lang];
  const data = rhythm.all;
  return (
    <div className={`w-heat${compact ? " compact" : ""}`}>
      {[1, 2, 3, 4, 5, 6, 0].map((weekday) => (
        <div key={weekday}>
          <span>{dict.rhythm.weekdaysShort[weekday]}</span>
          <div>
            {data.cells
              .filter((cell) => cell.weekday === weekday)
              .map((cell) => (
                <i key={cell.hour} className={`level-${cell.intensity}`} />
              ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function WLegend({ lang }) {
  const dict = catalogs[lang];
  return (
    <div className="w-legend">
      <span>{dict.rhythm.low}</span>
      <div>
        {[0, 1, 2, 3, 4, 5].map((value) => (
          <i key={value} className={`level-${value}`} />
        ))}
      </div>
      <span>{dict.rhythm.high}</span>
    </div>
  );
}

/* ------------------------------------------------------------------ 四个族 */

function Magnitude({ size, lang }) {
  const dict = catalogs[lang];
  const today = scope("all", "today");
  const week = scope("all", "7d");
  const month = scope("all", "30d");
  const incomplete = !today.pricingComplete;
  const scopeText = size === "small" ? dict.periods.today : `${dict.widgets.allClients} · ${size === "large" ? dict.periods["30d"] : dict.periods.today}`;

  if (size === "small") {
    return (
      <Frame kind="magnitude" size={size} lang={lang} scopeText={scopeText} Icon={ChartBar}>
        <strong className="w-headline accent">
          {incomplete ? "≈" : ""}
          {formatCost(today.totals.cost, lang)}
        </strong>
        <small className="w-support">
          {formatTokens(today.totals.tokens)} · {today.totals.sessions} {dict.hero.sessions}
        </small>
        <MiniBars values={month.daily.slice(-SMALL_BUCKETS).map((item) => item.value)} />
      </Frame>
    );
  }

  if (size === "medium") {
    // 20 根柱子是 medium 的规格，不是 30；轴的起点必须跟着这 20 天走，
    // 否则轴说的是 30 天前而柱子画的是 20 天。
    const mediumBuckets = month.daily.slice(-MEDIUM_BUCKETS);
    return (
      <Frame kind="magnitude" size={size} lang={lang} scopeText={scopeText} Icon={ChartBar}>
        <div className="w-periods">
          {[
            [dict.periods.today, today],
            [dict.periods["7d"], week],
            [dict.periods["30d"], month],
          ].map(([label, value]) => (
            <div key={label}>
              <span>{label}</span>
              <strong>{formatCost(value.totals.cost, lang, { compact: true })}</strong>
              {/* 成本单独一个数没法自检，token 紧跟在下面——每个尺寸都一样 */}
              <small>{formatTokens(value.totals.tokens)}</small>
            </div>
          ))}
        </div>
        <MiniBars values={mediumBuckets.map((item) => item.value)} />
        <div className="w-axis">
          <span>{formatDate(mediumBuckets[0].date, lang)}</span>
          <span>{formatDate(meta.lastDate, lang)}</span>
        </div>
      </Frame>
    );
  }

  return (
    <Frame kind="magnitude" size={size} lang={lang} scopeText={scopeText} Icon={ChartBar}>
      <div className="w-hero">
        <strong className="accent">
          {incomplete ? "≈" : ""}
          {formatCost(month.totals.cost, lang)}
        </strong>
        {incomplete && <em className="w-flag">{dict.status.costIncomplete(month.pricing.unpricedEvents)}</em>}
      </div>
      <small className="w-support">
        {formatTokens(month.totals.tokens)} · {month.totals.sessions} {dict.hero.sessions}
      </small>
      <div className="w-periods tight">
        {[
          [dict.periods.today, today],
          [dict.periods["7d"], week],
        ].map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{formatCost(value.totals.cost, lang, { compact: true })}</strong>
          </div>
        ))}
      </div>
      <AreaChart values={month.daily.map((item) => item.value)} height={58} />
      <div className="w-axis">
        <span>{formatDate(meta.firstDate, lang)}</span>
        <span>{formatDate(meta.lastDate, lang)}</span>
      </div>
      <WStats
        items={[
          { label: dict.usage.avgPerDay, value: formatCost(month.averagePerDay, lang) },
          {
            label: dict.usage.peak,
            value: formatCost(month.peak.totals.cost, lang),
            sub: formatDate(month.peak.date, lang),
          },
          { label: dict.usage.cacheHit, value: formatShare(month.cacheHitShare, lang) },
        ]}
      />
    </Frame>
  );
}

function Composition({ size, lang }) {
  const dict = catalogs[lang];
  const view = scope("all", size === "large" ? "30d" : "today");
  const top = view.models[0];
  const scopeText = size === "small" ? dict.periods.today : `${dict.widgets.allClients} · ${size === "large" ? dict.periods["30d"] : dict.periods.today}`;

  if (size === "small") {
    return (
      <Frame kind="composition" size={size} lang={lang} scopeText={scopeText} Icon={ChartPieSlice}>
        <span className="w-eyebrow">{dict.widgets.topModel}</span>
        <strong className="w-model">{top.model}</strong>
        <strong className="w-headline model-b">{formatShare(top.share, lang)}</strong>
        <div className="w-track">
          <i className="tone-model-b" style={{ width: `${top.share}%` }} />
        </div>
        <small className="w-support">{dict.widgets.ofTokens(formatTokens(top.tokens), formatTokens(view.totals.tokens))}</small>
      </Frame>
    );
  }

  if (size === "medium") {
    return (
      <Frame kind="composition" size={size} lang={lang} scopeText={scopeText} Icon={ChartPieSlice}>
        <div className="w-list">
          {view.models.slice(0, 4).map((model) => (
            <WRow key={model.model} dot label={model.model} value={formatTokens(model.tokens)} share={model.share} tone={model.tone} lang={lang} />
          ))}
        </div>
      </Frame>
    );
  }

  return (
    <Frame kind="composition" size={size} lang={lang} scopeText={scopeText} Icon={ChartPieSlice}>
      <span className="w-eyebrow">{dict.breakdown.models}</span>
      <div className="w-list">
        {view.models.slice(0, 4).map((model) => (
          <WRow key={model.model} dot label={model.model} value={formatTokens(model.tokens)} share={model.share} tone={model.tone} lang={lang} />
        ))}
      </div>
      <div className="w-divider">
        <span className="w-eyebrow">{dict.breakdown.tokenMix}</span>
        <small className="tone-text-warn">{dict.breakdown.cacheWriteBilled}</small>
      </div>
      <div className="w-stack">
        {view.tokenMix.map((item) => (
          <i key={item.key} className={`tone-${item.tone}`} style={{ width: `${item.share}%` }} />
        ))}
      </div>
      <div className="w-mix">
        {view.tokenMix.map((item) => (
          <div key={item.key}>
            <span>
              <i className={`dot tone-${item.tone}`} />
              {dict.breakdown[item.key]}
            </span>
            <b>{formatTokens(item.value)}</b>
            <small>{formatShare(item.share, lang)}</small>
          </div>
        ))}
      </div>
      <div className="w-inline">
        <span>{dict.breakdown.subtotals}</span>
        {view.clientSubtotals.map((item) => (
          <strong key={item.client}>
            {dict.clients[item.client]} {formatCost(item.cost, lang, { compact: true })}
          </strong>
        ))}
      </div>
    </Frame>
  );
}

function Trust({ size, lang }) {
  const dict = catalogs[lang];
  const view = scope("all", "today");
  const [determinable, inferred, unattributed] = view.quality;
  const tones = { determinable: "good", inferred: "model-b", unattributed: "warn" };

  if (size === "small") {
    return (
      <Frame kind="trust" size={size} lang={lang} scopeText={dict.periods.today} Icon={ShieldCheck}>
        <span className="w-eyebrow">{dict.attribution.determinable}</span>
        <strong className="w-headline good">{formatShare(determinable.share, lang)}</strong>
        <div className="w-track">
          <i className="tone-good" style={{ width: `${determinable.share}%` }} />
        </div>
        <small className="w-support">
          {dict.attribution.inferred} {formatCost(inferred.cost, lang)} · {dict.attribution.unattributed}{" "}
          {formatCost(unattributed.cost, lang)}
        </small>
      </Frame>
    );
  }

  if (size === "medium") {
    return (
      <Frame kind="trust" size={size} lang={lang} scopeText={dict.periods.today} Icon={ShieldCheck}>
        <div className="w-list">
          {view.quality.map((tier) => (
            <WRow
              key={tier.quality}
              label={dict.attribution[tier.quality]}
              value={formatCost(tier.cost, lang)}
              share={tier.share}
              tone={tones[tier.quality]}
              lang={lang}
            />
          ))}
        </div>
        <div className="w-coverage">
          <div>
            <span>{dict.attribution.coverage}</span>
            <strong className="tone-text-info">{formatShare(view.pricing.coverage, lang)}</strong>
          </div>
          <div className="w-track">
            <i className="tone-info" style={{ width: `${view.pricing.coverage}%` }} />
          </div>
        </div>
      </Frame>
    );
  }

  return (
    <Frame kind="trust" size={size} lang={lang} scopeText={dict.periods.today} Icon={ShieldCheck}>
      <span className="w-eyebrow">{dict.widgets.measurement}</span>
      <strong className="w-headline good">{formatShare(determinable.share, lang)}</strong>
      <small className="w-support">{dict.widgets.determinateCost}</small>
      <div className="w-list gap">
        {view.quality.slice(1).map((tier) => (
          <WRow
            key={tier.quality}
            label={dict.attribution[tier.quality]}
            value={formatCost(tier.cost, lang)}
            share={tier.share}
            tone={tones[tier.quality]}
            lang={lang}
          />
        ))}
      </div>
      <div className="w-divider">
        <span className="w-eyebrow">{dict.attribution.byProvider}</span>
        <small>{formatShare(view.pricing.coverage, lang)}</small>
      </div>
      <div className="w-list">
        {view.providerQuality.map((item) => (
          <WRow
            key={item.provider}
            label={dict.clients[item.provider]}
            value={formatCost(item.cost, lang)}
            share={item.share}
            tone="good"
            lang={lang}
          />
        ))}
      </div>
      <div className="w-note">
        <span>{dict.attribution.unpriced}</span>
        <strong>{view.pricing.identifiers[0]}</strong>
        <small>{dict.attribution.incompleteNote}</small>
      </div>
    </Frame>
  );
}

function Rhythm({ size, lang }) {
  const dict = catalogs[lang];
  const data = rhythm.all;
  const calendar = scope("all", "30d").daily;
  const max = Math.max(...calendar.map((item) => item.value), 0.0001);
  const level = (value) => (value <= 0 ? 0 : Math.min(5, Math.max(1, Math.ceil((value / max) * 5))));
  const scopeText = `${dict.periods["30d"]}`;

  if (size === "small") {
    return (
      <Frame kind="rhythm" size={size} lang={lang} scopeText={scopeText} Icon={ClockCounterClockwise}>
        <span className="w-eyebrow">{dict.widgets.activeDays}</span>
        <strong className="w-headline accent">
          {data.activeDays}
          <em> / 30</em>
        </strong>
        <small className="w-support">
          {dict.widgets.busiestAt(dict.rhythm.weekdays[data.busiestDay], formatHourRangeShort(data.peakStart, data.peakEnd, lang))}
        </small>
      </Frame>
    );
  }

  if (size === "medium") {
    return (
      <Frame kind="rhythm" size={size} lang={lang} scopeText={scopeText} Icon={ClockCounterClockwise}>
        <div className="w-axis-row">
          <div className="w-hour-axis" aria-hidden="true">
            {["00", "06", "12", "18", "24"].map((mark) => (
              <span key={mark}>{mark}</span>
            ))}
          </div>
          <WLegend lang={lang} />
        </div>
        <Heat lang={lang} compact />
      </Frame>
    );
  }

  return (
    <Frame kind="rhythm" size={size} lang={lang} scopeText={scopeText} Icon={ClockCounterClockwise}>
      <div className="w-axis-row">
        <span className="w-eyebrow">{dict.rhythm.hourOfWeek}</span>
        <WLegend lang={lang} />
      </div>
      <div className="w-hour-axis" aria-hidden="true">
        {["00", "06", "12", "18", "24"].map((mark) => (
          <span key={mark}>{mark}</span>
        ))}
      </div>
      <Heat lang={lang} />
      <div className="w-divider">
        <span className="w-eyebrow">{dict.widgets.context90}</span>
        <small>
          {formatDate(meta.firstDate, lang)} – {formatDate(meta.lastDate, lang)}
        </small>
      </div>
      <div className="w-calendar">
        {calendar.map((item) => (
          <i key={item.iso} className={`level-${level(item.value)}`} />
        ))}
      </div>
      <WStats
        items={[
          { label: dict.rhythm.active, value: `${data.activeDays} / 30` },
          { label: dict.rhythm.busiest, value: dict.rhythm.weekdays[data.busiestDay] },
          { label: dict.rhythm.quietest, value: dict.rhythm.weekdays[data.quietestDay] },
        ]}
      />
    </Frame>
  );
}

// 额度。小组件的刷新目标是 3–5 分钟，而额度探测刻意更慢，所以这里显示的
// 一定是一个比小组件自身周期更旧的数字——每个尺寸都必须带上读取时刻，
// 否则用户会把上一次探测的百分比当成此刻的百分比。
//
// 尺寸决定信息量，不决定信息种类：
//   small  只放最紧的那个窗口，一个百分比加一条倒计时
//   medium 每个 official 客户端一行，仍只取各自最紧的窗口
//   large  展开到每个窗口，并带上官方重置次数
// 三个尺寸都用 bothOfficial 变体：一个尺寸展示门禁、另一个展示满额数据，
// 会让同一张画板上的三张卡互相矛盾。
function worstWindow(entry) {
  if (!entry.applicable || entry.windows.length === 0) return null;
  return entry.windows.reduce((worst, item) => (item.used_percent > worst.used_percent ? item : worst));
}

function quotaWidgetTone(percent) {
  if (percent >= 90) return "warn";
  if (percent >= 75) return "model-b";
  return "model-a";
}

function quotaAgo(observedAt, now) {
  const minutes = Math.max(0, Math.round((now - observedAt) / 60));
  if (minutes < 1) return "<1m";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function quotaEta(resetsAt, now) {
  const minutes = Math.max(0, Math.floor((resetsAt - now) / 60));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d${hours % 24 ? `${hours % 24}h` : ""}`;
}

// 窗口标签必须带上额度名称。Codex Plus 的四个窗口里有两对跨度相同——主限额和
// 每模型限额各有一个 5 小时窗与 7 天窗——只写跨度会得到两组一模一样的标签，
// 读者无法分辨 62% 是账号主限额还是某个模型的限额。
function widgetWindowLabel(window, dict) {
  const span = dict.quota.windowMins(window.window_minutes);
  return window.label ? `${window.label} · ${span}` : span;
}

// 归属说明必须是看得见的一行，不能只塞进 title 或辅助名称：需求 11 要的是
// 「屏幕上说出来」，一个读屏才听得到的声明保护不了用眼睛看数字的人。
// 三个尺寸都用短式，因为 148px 的卡放不下整句，而"放不下"不是省略的理由。
function QuotaAttribution({ entry, lang }) {
  if (entry.attribution_confirmed !== false) return null;
  return <small className="w-attribution">{catalogs[lang].quota.attributionShort}</small>;
}

function QuotaClientBlock({ entry, size, lang, now, windows }) {
  const dict = catalogs[lang];
  return (
    <section className="w-quota-client" data-client={entry.client}>
      <header>
        <strong>{dict.clients[entry.client]}</strong>
        {entry.plan && <small>{entry.plan}</small>}
        <QuotaAttribution entry={entry} lang={lang} />
      </header>
      <div className="w-quota-client-windows">
        {windows.map((window) => (
          <div className="w-quota-row" key={`${entry.client}-${window.key}`}>
            <div>
              <span title={widgetWindowLabel(window, dict)}>{widgetWindowLabel(window, dict)}</span>
              <b>{quotaEta(window.resets_at, now)}</b>
              <strong>{formatShare(window.used_percent, lang)}</strong>
            </div>
            <div className="w-track">
              <i
                className={`tone-${quotaWidgetTone(window.used_percent)}`}
                style={{ width: `${window.used_percent}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

// 无数据卡的脚注必须说出真实原因。写死 never_probed 会在「读取已关闭」时
// 告诉用户"尚未成功探测"，把一个开关问题说成一个故障。
// 本尺寸下的最终形态由 ux/widget-quota.md 的设计轮次裁定；这里只保证
// 现在说的是实话。
function QuotaNoData({ size, lang, scopeText, reason = "never_probed" }) {
  const dict = catalogs[lang];
  return (
    <Frame
      kind="quota"
      size={size}
      lang={lang}
      scopeText={scopeText}
      Icon={Gauge}
      footText={dict.quota.reasons[reason] ?? reason}
    >
      <p className="w-quota-none">
        {reason === "probe_disabled" ? dict.quota.readingOff : dict.widgets.quotaNoData}
      </p>
    </Frame>
  );
}

function QuotaWidget({ size, lang, quotaVariant = "normal", widgetClient = "codex" }) {
  const dict = catalogs[lang];
  const payload = quota(quotaVariant);
  const now = quotaMeta.now;
  // “没有时不展示”在结构层执行：没有窗口、门禁不适用或探测失败的端，
  // 根本不生成 block，不用一张“不可用”卡去占另一个端的空间。
  const available = payload.clients.filter(
    (entry) => entry.applicable && !entry.failure && entry.windows.length > 0,
  );
  // 无数据时的原因取自实际载荷，而不是假设成「尚未探测」。
  const noDataReason = payload.clients.every((entry) => entry.failure === "probe_disabled")
    ? "probe_disabled"
    : "never_probed";

  // small 与 medium 都只呈现配置里选定的那一端；它们的差别是深度，不是范围。
  // 选定端无数据时不偷换成另一端——配置说 Codex 却看到 Claude，配置就不可信了。
  if (size === "small" || size === "medium") {
    const entry = available.find((item) => item.client === widgetClient);
    if (!entry) return <QuotaNoData size={size} lang={lang} scopeText="" reason={noDataReason} />;
    const age = dict.quota.observedAt(quotaAgo(entry.observed_at, now));

    if (size === "small") {
      // small 只有一个数字的位置，所以给已用最高的那个窗口——会先挡住工作的
      // 是它，而不是返回顺序里的第一个。
      const window = worstWindow(entry);
      return (
        <Frame
          kind="quota"
          size={size}
          lang={lang}
          scopeText={dict.clients[entry.client]}
          Icon={Gauge}
          footText={age}
        >
          <span className="w-eyebrow" title={widgetWindowLabel(window, dict)}>
            {widgetWindowLabel(window, dict)}
          </span>
          <strong className={`w-headline ${quotaWidgetTone(window.used_percent)}`}>
            {formatShare(window.used_percent, lang)}
          </strong>
          <div className="w-track">
            <i
              className={`tone-${quotaWidgetTone(window.used_percent)}`}
              style={{ width: `${window.used_percent}%` }}
            />
          </div>
          <small className="w-support">{dict.quota.resetsIn(quotaEta(window.resets_at, now))}</small>
          <QuotaAttribution entry={entry} lang={lang} />
        </Frame>
      );
    }

    // medium：同一端的全部窗口。官方重置次数不进小组件——小组件回答
    // 「还剩多少额度」，而重置次数是一项需要读状态与到期日才有意义的账户资产，
    // 一个没有展开位置的表面放一个孤零零的计数，只会引出它答不了的问题。
    return (
      <Frame
        kind="quota"
        size={size}
        lang={lang}
        scopeText={dict.clients[entry.client]}
        Icon={Gauge}
        footText={age}
      >
        <div className="w-quota-rows">
          {entry.windows.map((window) => (
            <div className="w-quota-row" key={window.key}>
              <div>
                <span title={widgetWindowLabel(window, dict)}>{widgetWindowLabel(window, dict)}</span>
                <b>{quotaEta(window.resets_at, now)}</b>
                <strong>{formatShare(window.used_percent, lang)}</strong>
              </div>
              <div className="w-track">
                <i
                  className={`tone-${quotaWidgetTone(window.used_percent)}`}
                  style={{ width: `${window.used_percent}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <QuotaAttribution entry={entry} lang={lang} />
      </Frame>
    );
  }

  // large：两端同时呈现，上下等分，各自在自己那一半里居中。
  // 等分而不是按窗口数分配高度，是因为两端的相对重要性不随窗口多少变化。
  if (available.length === 0)
    return (
      <QuotaNoData size={size} lang={lang} scopeText={dict.widgets.allClients} reason={noDataReason} />
    );
  const observedAt = available.map((entry) => entry.observed_at).filter((value) => value != null);
  const age = observedAt.length
    ? dict.quota.observedAt(quotaAgo(Math.min(...observedAt), now))
    : dict.quota.reasons.never_probed;
  return (
    <Frame
      kind="quota"
      size={size}
      lang={lang}
      scopeText={dict.widgets.allClients}
      Icon={Gauge}
      footText={age}
    >
      <div className="w-quota-split">
        {available.map((entry) => (
          <QuotaClientBlock
            key={entry.client}
            entry={entry}
            size={size}
            lang={lang}
            now={now}
            windows={entry.windows}
          />
        ))}
      </div>
    </Frame>
  );
}

const RENDERERS = { magnitude: Magnitude, composition: Composition, trust: Trust, rhythm: Rhythm, quota: QuotaWidget };

function Group({ kind, Icon, lang, quotaVariant, widgetClient }) {
  const dict = catalogs[lang];
  const Renderer = RENDERERS[kind];
  return (
    <section className="widget-group">
      <div className="group-head">
        <span>
          <Icon size={16} weight="fill" />
          <strong>{dict.widgets.kinds[kind].name}</strong>
        </span>
        <p>{dict.widgets.kinds[kind].question}</p>
      </div>
      <div className="widget-grid">
        <div className="size-tag tag-small">{dict.widgets.sizes.small}</div>
        <div className="size-tag tag-medium">{dict.widgets.sizes.medium}</div>
        <div className="size-tag tag-large">{dict.widgets.sizes.large}</div>
        <Renderer size="small" lang={lang} quotaVariant={quotaVariant} widgetClient={widgetClient} />
        <Renderer size="medium" lang={lang} quotaVariant={quotaVariant} widgetClient={widgetClient} />
        <Renderer size="large" lang={lang} quotaVariant={quotaVariant} widgetClient={widgetClient} />
      </div>
    </section>
  );
}

export function WidgetGallery() {
  const prefs = useStagePrefs();
  const { lang, theme } = prefs;
  const dict = catalogs[lang];
  return (
    <main className="board" data-theme={theme}>
      <StageControls prefs={prefs} showState={false} showWidgetClient />
      <header className="board-head">
        <h1>{dict.widgets.boardTitle}</h1>
        <p>{dict.widgets.boardSubtitle}</p>
      </header>
      <div className="board-grid">
        {KINDS.map(({ key, Icon }) => (
          <Group key={key} kind={key} Icon={Icon} lang={lang} quotaVariant={prefs.quota} widgetClient={prefs.widgetClient} />
        ))}
      </div>
    </main>
  );
}
