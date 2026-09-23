import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ScanStatus, ScanEmptyPopover } from "./ScanProgress.jsx";
import {
  ArrowClockwise,
  ArrowsClockwise,
  CaretLeft,
  CaretRight,
  CaretUp,
  ChartBar,
  ChartPieSlice,
  Check,
  ClockCounterClockwise,
  Code,
  FileText,
  Gauge,
  ShieldCheck,
  SpinnerGap,
  Warning,
  WarningCircle,
  Wrench,
} from "@phosphor-icons/react";
import {
  HEALTH,
  HEALTH_SCHEMA,
  HEALTH_SCHEMA_STACKED,
  PENDING_CAPTURE,
  PROVIDER,
  WORK_SIGNALS,
  buckets,
  meta,
  quota,
  quotaMeta,
  rhythm,
  scope,
} from "./data.js";
import {
  catalogs,
  formatCost,
  formatDate,
  formatDuration,
  formatHourRange,
  formatHourRangeShort,
  formatNumber,
  formatShare,
  formatTokens,
  formatWeekdayDate,
  relativeTime,
} from "./i18n.js";
import { HEALTH_RECOVERY } from "./healthRecovery.js";

// 额度排在第一位，并且是默认页：一个被排到首位却不是默认打开的 tab，等于
// 用位置说它最重要、又用默认值说它不是。
// 与 styles.css 的 .quota-flyout 一致；判断左右要用真实宽度，不能靠估。
const FLYOUT_WIDTH = 232;
const FLYOUT_GAP = 10;
// 从触发行移到明细要跨过 FLYOUT_GAP。间隙由 .quota-flyout 的 ::before 补成
// 可悬停区域，这个宽限只用来吸收两个指针事件之间的那一拍。
const CREDITS_CLOSE_GRACE = 160;

// 触发行和明细之间的实际空白不是 CSS 里那 10px：明细定位在面板坐标系上，
// 触发行还隔着卡片内边距和面板内边距，实测约 35px，而且随宽度和主题变。
// 所以通路不能用一个写死的伪元素去铺，只能按两个真实 rect 算。
// 只读指针位置、不铺任何覆盖层，因此不会替面板内容吃掉悬停。
function withinCreditsRegion(x, y, trigger, flyout) {
  const inside = (rect) => x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
  if (inside(trigger) || inside(flyout)) return true;
  // 水平通路：明细在触发行右侧或左侧时，两者之间那条带子也算"正在路上"。
  // 覆盖式布局两个矩形本来就重叠，没有通路可言。
  let lo;
  let hi;
  if (flyout.left >= trigger.right) {
    lo = trigger.right;
    hi = flyout.left;
  } else if (flyout.right <= trigger.left) {
    lo = flyout.right;
    hi = trigger.left;
  } else {
    return false;
  }
  if (x < lo || x > hi) return false;
  // 纵向取两者的并集而不是交集：斜着慢慢挪过去的人不该掉出通路。
  return y >= Math.min(trigger.top, flyout.top) && y <= Math.max(trigger.bottom, flyout.bottom);
}

const TABS = [
  { key: "quota", Icon: Gauge },
  { key: "usage", Icon: ChartBar },
  { key: "breakdown", Icon: ChartPieSlice },
  { key: "attribution", Icon: ShieldCheck },
  { key: "sessions", Icon: Code },
];

// 数据状态。normal 以外的五种都是这一版新增的，用来检验界面在数据不好时还站不站得住。
export const SURFACE_STATES = ["normal", "empty", "aged", "partial", "pending", "unavailable", "schema", "schemaStacked"];

// 本条件的两种排布：仅此一项问题，以及叠加其他问题。
const SCHEMA_STATES = ["schema", "schemaStacked"];

function refreshPresentation(scenario, state) {
  const ordinaryAge = state === "aged" ? 512 : 0;
  switch (scenario) {
    case "refreshing": return { status: "refreshing", issue: null, age: 4 };
    case "failed": return { status: "failed", issue: "retainedFailure", age: 18 };
    case "storageFailed": return { status: "idle", issue: "storageFailure", age: 0 };
    case "wake": return { status: "refreshing", issue: null, age: 487 };
    case "recovered": return { status: "success", issue: null, age: 0 };
    case "firstFailure": return { status: "failed", issue: "firstFailure", age: 0 };
    default: return { status: "idle", issue: null, age: ordinaryAge };
  }
}

// schema 两态的健康负载与其余六态不同：它们是本条件下 doctor 的实测返回及其叠加变体。
function healthOf(state, recovery = "baseline") {
  if (recovery !== "baseline" && HEALTH_RECOVERY[recovery]) return HEALTH_RECOVERY[recovery];
  if (state === "schema") return HEALTH_SCHEMA;
  if (state === "schemaStacked") return HEALTH_SCHEMA_STACKED;
  return HEALTH;
}

function useDict(lang) {
  return catalogs[lang];
}

function Bar({ share, tone, muted }) {
  return (
    <div className={`bar-track${muted ? " muted" : ""}`}>
      <i className={`tone-${tone}`} style={{ width: `${Math.max(share, share > 0 ? 1.5 : 0)}%` }} />
    </div>
  );
}

function Row({ label, dot, value, share, tone, lang }) {
  return (
    <div className="data-row">
      <div>
        <span>
          {dot && <i className={`dot tone-${tone}`} />}
          {label}
        </span>
        <b>{value}</b>
        <strong>{formatShare(share, lang)}</strong>
      </div>
      <Bar share={share} tone={tone} />
    </div>
  );
}

function StatGrid({ items }) {
  return (
    <div
      className="stat-grid"
      data-dense={items.length > 3 ? "1" : undefined}
      style={{ "--columns": items.length }}
    >
      {items.map((item) => (
        <div key={item.label}>
          <span>{item.label}</span>
          <strong className={item.tone ? `tone-text-${item.tone}` : undefined}>{item.value}</strong>
          {item.note && <small>{item.note}</small>}
        </div>
      ))}
    </div>
  );
}

function EmptyNote({ text }) {
  return <p className="empty-note">{text}</p>;
}

/* ---------------------------------------------------------------- 趋势图 */

function TrendChart({ data, lang, state }) {
  const dict = useDict(lang);
  const [hover, setHover] = useState(null);
  const [pinned, setPinned] = useState(null);
  const [focus, setFocus] = useState(0);
  const listRef = useRef(null);
  const active = pinned ?? hover;
  const max = Math.max(...data.map((item) => item.cost), 0.0001);
  const peakIndex = data.reduce((best, item, index) => (item.cost > data[best].cost ? index : best), 0);

  const move = (delta) => {
    const next = Math.min(data.length - 1, Math.max(0, focus + delta));
    setFocus(next);
    setHover(next);
    listRef.current?.querySelectorAll(".bar")[next]?.focus();
  };

  const label = (item) =>
    item.kind === "hour" ? formatHourRange(item.hour, item.hour + 1) : formatDate(item.date, lang);

  if (state === "empty") {
    return (
      <div className="trend">
        <div className="bars empty" aria-hidden="true">
          {data.map((item) => (
            <span key={item.key} className="bar-ghost" />
          ))}
        </div>
        <EmptyNote text={dict.status.empty} />
      </div>
    );
  }

  return (
    <div className="trend">
      <div
        className="bars"
        ref={listRef}
        role="group"
        aria-label={`${dict.usage.trend} · ${data.length}`}
        onMouseLeave={() => setHover(null)}
      >
        {data.map((item, index) => (
          <button
            type="button"
            key={item.key}
            className={`bar${active === index ? " active" : ""}${index === peakIndex ? " peak" : ""}`}
            tabIndex={index === focus ? 0 : -1}
            aria-label={`${label(item)} ${formatCost(item.cost, lang)} ${formatTokens(item.totals.tokens)} tokens`}
            onMouseEnter={() => setHover(index)}
            onFocus={() => {
              setFocus(index);
              setHover(index);
            }}
            onBlur={() => setHover(null)}
            onClick={() => setPinned(pinned === index ? null : index)}
            onKeyDown={(event) => {
              if (event.key === "ArrowRight") {
                event.preventDefault();
                move(1);
              }
              if (event.key === "ArrowLeft") {
                event.preventDefault();
                move(-1);
              }
              if (event.key === "Home") {
                event.preventDefault();
                move(-data.length);
              }
              if (event.key === "End") {
                event.preventDefault();
                move(data.length);
              }
            }}
          >
            <span style={{ height: `${Math.max(2, (item.cost / max) * 100)}%` }} />
          </button>
        ))}
      </div>
      <div className="axis" aria-hidden="true">
        {data[0].kind === "hour" ? (
          <>
            <span>{dict.usage.hourAxis.start}</span>
            <span>{dict.usage.hourAxis.mid}</span>
            <span>{dict.usage.hourAxis.end}</span>
          </>
        ) : (
          <>
            <span>{formatDate(data[0].date, lang)}</span>
            <span>{formatDate(data[data.length - 1].date, lang)}</span>
          </>
        )}
      </div>
      <div className={`readout${active == null ? " idle" : ""}`} role="status">
        {active == null ? (
          <span className="readout-hint">
            {data[peakIndex] && `${dict.usage.peak} ${label(data[peakIndex])} · ${formatCost(data[peakIndex].cost, lang)}`}
          </span>
        ) : (
          <>
            <strong>{label(data[active])}</strong>
            <span>{formatCost(data[active].cost, lang)}</span>
            <span>{formatTokens(data[active].totals.tokens)}</span>
            <span>
              {formatNumber(data[active].totals.events, lang)} {dict.hero.events}
            </span>
            {pinned != null && <i className="pin" aria-hidden="true" />}
          </>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------ 四个 tab 面板 */

function UsagePanel({ view, lang, state }) {
  const dict = useDict(lang);
  const data = buckets(view.client, view.period);
  // 今日只有一天，"日均"和"峰值日"都等于当日总额，说了等于没说；
  // 这一档换成当天最贵的那个小时与事件数。
  const today = view.period === "today";
  const busiest = today && state !== "empty" ? data.reduce((best, item) => (item.cost > best.cost ? item : best), data[0]) : null;
  return (
    <section className="panel">
      <div className="card">
        <TrendChart data={data} lang={lang} state={state} />
        <StatGrid
          items={
            state === "empty"
              ? [
                  { label: dict.usage.busiestHour, value: "—" },
                  { label: dict.hero.events, value: "0" },
                  { label: dict.usage.cacheHit, value: "—" },
                ]
              : today
              ? [
                  {
                    label: dict.usage.busiestHour,
                    value: formatCost(busiest.cost, lang),
                    note: formatHourRange(busiest.hour, busiest.hour + 1),
                  },
                  { label: dict.hero.events, value: formatNumber(view.totals.events, lang) },
                  { label: dict.usage.cacheHit, value: formatShare(view.cacheHitShare, lang) },
                ]
              : [
                  { label: dict.usage.avgPerDay, value: formatCost(view.averagePerDay, lang) },
                  {
                    label: dict.usage.peak,
                    value: formatCost(view.peak.totals.cost, lang),
                    note: formatDate(view.peak.date, lang),
                  },
                  { label: dict.usage.cacheHit, value: formatShare(view.cacheHitShare, lang) },
                ]
          }
        />
      </div>
    </section>
  );
}

function BreakdownPanel({ view, lang, state }) {
  const dict = useDict(lang);
  const shown = view.models.slice(0, 4);
  const rest = view.models.length - shown.length;
  if (state === "empty") {
    return (
      <section className="panel">
        <div className="card">
          <EmptyNote text={dict.status.emptyOther} />
        </div>
      </section>
    );
  }
  return (
    <section className="panel">
      <div className="card">
        <div className="card-head">
          <span>{dict.breakdown.models}</span>
        </div>
        {shown.map((model) => (
          <Row
            key={model.model}
            dot
            label={model.model}
            value={formatTokens(model.tokens)}
            share={model.share}
            tone={model.tone}
            lang={lang}
          />
        ))}
        {rest > 0 && <p className="row-more">{dict.breakdown.others(rest)}</p>}
      </div>
      <div className="card">
        <div className="card-head">
          <span>{dict.breakdown.tokenMix}</span>
          <small className="tone-text-warn">{dict.breakdown.cacheWriteBilled}</small>
        </div>
        <div className="stack">
          {view.tokenMix.map((item) => (
            <i key={item.key} className={`tone-${item.tone}`} style={{ width: `${item.share}%` }} />
          ))}
        </div>
        <div className="mix-list">
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
      </div>
      <div className="inline-row">
        <span>{dict.breakdown.subtotals}</span>
        {view.clientSubtotals.map((item) => (
          <strong key={item.client}>
            {dict.clients[item.client]} {formatCost(item.cost, lang)}
          </strong>
        ))}
      </div>
    </section>
  );
}

function AttributionPanel({ view, lang, state }) {
  const dict = useDict(lang);
  const tones = { determinable: "good", inferred: "model-b", unattributed: "warn" };
  // partial 指某个数据域读不出来。这里让归因成为缺失的那个域：
  // 其余面板照常，用户能明确看到哪一块没有数据，而不是整屏都变灰。
  if (state === "partial") {
    return (
      <section className="panel">
        <div className="card">
          <div className="domain-missing">
            <WarningCircle size={20} />
            <strong>{dict.status.partial}</strong>
            <small>{dict.attribution.quality}</small>
          </div>
        </div>
      </section>
    );
  }
  if (state === "empty") {
    return (
      <section className="panel">
        <div className="card">
          <EmptyNote text={dict.status.emptyOther} />
        </div>
      </section>
    );
  }
  return (
    <section className="panel">
      <div className="card">
        <div className="card-head">
          <span>{dict.attribution.quality}</span>
        </div>
        {view.quality.map((tier) => (
          <Row
            key={tier.quality}
            label={dict.attribution[tier.quality]}
            value={formatCost(tier.cost, lang)}
            share={tier.share}
            tone={tones[tier.quality]}
            lang={lang}
          />
        ))}
      </div>
      <div className="card">
        <div className="card-head">
          <span>{dict.attribution.coverage}</span>
          <strong className="tone-text-info">{formatShare(view.pricing.coverage, lang)}</strong>
        </div>
        <Bar share={view.pricing.coverage} tone="info" />
        <div className="unpriced">
          <span>{dict.attribution.unpriced}</span>
          {view.pricing.identifiers.length === 0 ? (
            <small>{dict.attribution.unpricedNone}</small>
          ) : (
            <ul>
              {view.pricing.identifiers.map((identifier) => (
                <li key={identifier}>{identifier}</li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <div className="card">
        <div className="card-head">
          <span>{dict.attribution.byProvider}</span>
        </div>
        {view.providerQuality.map((item) => (
          <Row
            key={item.provider}
            label={dict.clients[item.provider]}
            value={formatCost(item.cost, lang)}
            share={item.share}
            tone="good"
            lang={lang}
          />
        ))}
      </div>
    </section>
  );
}

const SIGNALS = [
  { key: "activity", Icon: Code, tone: "info" },
  { key: "workflow", Icon: FileText, tone: "model-b" },
  { key: "tooling", Icon: Wrench, tone: "accent" },
];

/* ------------------------------------------------------------ 额度面板 */

// 倒计时而不是时钟时间：有用的问题是"还有多久能用"，不是"几点解封"。
// 跨时区、跨夏令时的时钟时间还需要读者自己换算，倒计时不需要。
// 裸时距与整句分开：把 resetsIn("12d") 再套进 creditExpires 会得到
// 「12d后重置到期」。凡是需要时距的地方取 etaText，需要整句的地方才包一层。
function etaText(target, now) {
  const remaining = target - now;
  const minutes = Math.max(0, Math.floor(remaining / 60));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  if (hours < 24) return restMinutes ? `${hours}h${restMinutes}m` : `${hours}h`;
  const days = Math.floor(hours / 24);
  const restHours = hours % 24;
  return restHours ? `${days}d${restHours}h` : `${days}d`;
}

function resetEta(resetsAt, now, dict) {
  if (resetsAt - now <= 0) return dict.quota.resetsNow;
  return dict.quota.resetsIn(etaText(resetsAt, now));
}

// 同一套阈值供 popover 与 widget 使用，一个颜色在两处含义相同。
export function quotaTone(percent) {
  if (percent >= 90) return "warn";
  if (percent >= 75) return "model-b";
  return "model-a";
}

// relativeTime() 返回的是"上次更新于 X 前"这样的整句，套进"X 读取"会得到病句。
// 额度这里要的是裸时距，所以直接走 Intl，不复用那个带语境的封装。
function plainAgo(seconds, lang) {
  const dict = catalogs[lang];
  const minutes = Math.max(0, Math.round(seconds / 60));
  // null 而不是 status.justNow：那个键是"刚刚更新"整句，套进"X读取"会得到
  // "刚刚更新读取"。不到一分钟另有专门文案，由调用方选。
  if (minutes < 1) return null;
  const formatter = new Intl.RelativeTimeFormat(dict.locale, { numeric: "auto" });
  if (minutes < 60) return formatter.format(-minutes, "minute");
  if (minutes < 60 * 24) return formatter.format(-Math.round(minutes / 60), "hour");
  return formatter.format(-Math.round(minutes / 1440), "day");
}

// 窄边界下模型名会把这一行截掉 101px（量具原话），所以 280pt 只留窗口跨度。
// 模型名不是消失，是退到 title/aria 里：窗口是主要事实，型号是限定语，
// 截断后的「GPT-5.3-Codex-Spar…」比没有型号更糟——它看着像另一个型号。
function windowLabel(window, dict, narrow) {
  const span = dict.quota.windowMins(window.window_minutes);
  if (!window.label) return span;
  return narrow ? span : `${window.label} · ${span}`;
}

function windowTitle(window, dict) {
  const span = dict.quota.windowMins(window.window_minutes);
  return window.label ? `${window.label} · ${span}` : span;
}

// 缺失字段的唯一渲染方式。带原因，不带原因就不该调用它。
// 三个词对应三件事，选词由原因决定，不由调用点各自判断：
//   not_official   不适用——产品故意没问，provider 不是 official
//   probe_disabled 未读取——用户把读取关了，谁也没问
//   其余           不可用——问了，客户端没答上来
// 把选词收在这里，是因为散在各个调用点的布尔标志迟早会有一处写反，
// 而写反的代价是告诉用户「坏了」，其实什么都没坏。
export function missingWord(reason, dict) {
  if (reason === "not_official") return dict.quota.notApplicable;
  if (reason === "probe_disabled") return dict.quota.readingOff;
  return dict.quota.unavailable;
}

function QuotaMissing({ label, reason, dict }) {
  return (
    <div className="quota-missing">
      <span>{label}</span>
      <b>{missingWord(reason, dict)}</b>
      <small>{dict.quota.reasons[reason] ?? reason}</small>
    </div>
  );
}

function QuotaClient({ entry, lang, now, narrow, openCredits, closeCredits, creditsOpenFor }) {
  const dict = useDict(lang);
  // 明细向右弹出，不在卡片里就地展开：就地展开会把下面的窗口和
  // 「本地观察到重置」整段推走，而这几行恰恰是读者要拿来对照的。
  // 弹层由 Popover 渲染，这里只上报锚点位置——面板 overflow: hidden，
  // 卡片内部的绝对定位元素出不去。
  const anchor = useRef(null);
  const credits = entry.reset_allowance?.credits ?? [];
  const creditsOpen = creditsOpenFor === entry.client;
  // 悬停即展开：明细是「看一眼」的信息，不是需要确认的动作，多一次点击
  // 只是把成本转嫁给每一次查看。键盘焦点走同一条路径，所以读屏与鼠标一致。
  const showCredits = () => credits.length > 0 && openCredits(entry, anchor.current);
  // 离开触发行不等于结束阅读——指针可能正朝明细走。真正的关闭交给延后的
  // 那一拍，进入明细会把它取消掉。
  const hideCredits = () => credits.length > 0 && closeCredits();
  const notApplicable = !entry.applicable;
  const ago = entry.observed_at == null ? null : plainAgo(now - entry.observed_at, lang);
  const age = entry.observed_at == null ? null : ago == null ? dict.quota.justRead : dict.quota.observedAt(ago);

  // 读取关闭是全局状态，不是这一端的字段缺失：套餐、重置次数、窗口会一起
  // 变成三行同样的「未读取」，读者要读三遍才知道是同一件事。卡片因此收成
  // 一行——保留端名，说清是什么状态，其余留给面板末尾那句提示。
  if (entry.failure === "probe_disabled") {
    return (
      <div className="card quota-client" data-client={entry.client}>
        <div className="card-head">
          <strong>{dict.clients[entry.client]}</strong>
          <small>—</small>
        </div>
        <QuotaMissing label={dict.quota.title} reason="probe_disabled" dict={dict} />
      </div>
    );
  }

  return (
    <div className="card quota-client" data-client={entry.client}>
      <div className="card-head">
        <strong>{dict.clients[entry.client]}</strong>
        {entry.plan ? (
          <span className="quota-plan">{entry.plan}</span>
        ) : null}
        <small>
          {entry.source ? dict.quota.sources[entry.source] : "—"}
          {age == null ? "" : ` · ${age}`}
          {entry.stale ? ` · ${dict.quota.stale}` : ""}
        </small>
      </div>

      {/* 账号归属不可确认是这一端每个数字的性质，不是某个字段缺失，所以
          自成一行而不是挤进上面的来源行：来源行在 280pt 下已经要放下
          来源、年龄和「已过期」，再追加一句会把最该读到的年龄挤掉。 */}
      {entry.attribution_confirmed === false && (
        <p className="quota-attribution">{dict.quota.attributionUnconfirmed}</p>
      )}

      {entry.windows.length === 0 ? (
        <QuotaMissing
          label={dict.quota.noWindows}
          reason={entry.failure ?? (notApplicable ? "not_official" : "not_reported")}
          dict={dict}
        />
      ) : (
        entry.windows.map((window) => (
          <div className="data-row" key={window.key}>
            <div>
              <span title={windowTitle(window, dict)}>{windowLabel(window, dict, narrow)}</span>
              <b>{resetEta(window.resets_at, now, dict)}</b>
              <strong>{formatShare(window.used_percent, lang)}</strong>
            </div>
            <Bar share={window.used_percent} tone={quotaTone(window.used_percent)} />
          </div>
        ))
      )}

      {/* 官方重置次数：与上面的窗口重置是两件事，所以自成一行且另有标题 */}
      {entry.reset_allowance ? (
        <>
          <div
            className={`quota-allowance${credits.length ? " expandable" : ""}`}
            ref={anchor}
            tabIndex={credits.length ? 0 : undefined}
            role={credits.length ? "button" : undefined}
            aria-expanded={credits.length ? creditsOpen : undefined}
            aria-haspopup={credits.length ? "dialog" : undefined}
            onMouseEnter={showCredits}
            onMouseLeave={hideCredits}
            onFocus={showCredits}
            onBlur={hideCredits}
          >
            <span>{dict.quota.allowance}</span>
            <b>
              {dict.quota.allowanceRemaining(entry.reset_allowance.remaining)}
              {credits.length > 0 && <CaretRight size={11} weight="bold" />}
            </b>
            {entry.reset_allowance.total == null && (
              <small>{dict.quota.allowanceTotalUnknown}</small>
            )}
          </div>
        </>
      ) : (
        <QuotaMissing
          label={dict.quota.allowance}
          reason={entry.reset_allowance_reason ?? "not_reported"}
          dict={dict}
        />
      )}

      {/* 本地观察到的重置：第三类语义，永远不与上面两类共用标签 */}
      {entry.observed_reset_at != null && (
        <div className="quota-observed">
          <span>{dict.quota.observedReset}</span>
          <b>{plainAgo(now - entry.observed_reset_at, lang) ?? dict.quota.justRead}</b>
        </div>
      )}

      {entry.plan == null && (
        <QuotaMissing
          label={dict.quota.plan}
          reason={entry.plan_reason ?? "not_reported"}
          dict={dict}
        />
      )}
    </div>
  );
}

// 侧边弹层。锚在触发它的那一行上、面板之外，主面板一行不动，读者可以同时
// 看见「剩 3 次」和它的明细。左右由实际可用空间决定，不写死一侧——菜单栏图标
// 可以在屏幕任意位置，固定朝右会在靠近右缘时把明细推出屏幕。
function CreditsFlyout({ entry, lang, now, top, side, onClose, nodeRef }) {
  const dict = useDict(lang);
  const ref = nodeRef;
  useEffect(() => {
    const onDown = (event) => {
      if (!ref.current?.contains(event.target)) onClose();
    };
    const onKey = (event) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  const credits = entry.reset_allowance?.credits ?? [];
  return (
    <div
      className={`quota-flyout quota-flyout-${side}`}
      style={{ top: `${top}px` }}
      ref={ref}
      role="dialog"
      aria-label={dict.quota.allowance}
    >
      <div className="quota-flyout-head">
        <strong>{dict.quota.allowance}</strong>
        <span>{dict.clients[entry.client]}</span>
      </div>
      <ul className="quota-credits">
        {credits.map((credit) => (
          <li key={credit.key}>
            <span>{credit.title}</span>
            <b>{dict.quota.creditStatus[credit.status] ?? credit.status}</b>
            <small>
              {dict.quota.creditGranted(plainAgo(now - credit.granted_at, lang) ?? dict.quota.justRead)}
              {" · "}
              {dict.quota.creditExpires(etaText(credit.expires_at, now))}
            </small>
          </li>
        ))}
      </ul>
      {/* 总数未提供是这一层的事实，不是脚注：厂商只列当前可见的额度。 */}
      <p className="quota-flyout-note">{dict.quota.allowanceTotalUnknown}</p>
    </div>
  );
}

function QuotaPanel({ lang, variant, narrow, openCredits, closeCredits, creditsOpenFor }) {
  const dict = useDict(lang);
  const payload = quota(variant);
  const now = quotaMeta.now;
  // 提示只出现一次。两张卡各挂一句「去设置里开」，读者会以为是两个开关。
  const readingOff = payload.clients.every((entry) => entry.failure === "probe_disabled");
  return (
    <section className="panel">
      {payload.clients.map((entry) => (
        <QuotaClient
          key={entry.client}
          entry={entry}
          lang={lang}
          now={now}
          narrow={narrow}
          openCredits={openCredits}
          closeCredits={closeCredits}
          creditsOpenFor={creditsOpenFor}
        />
      ))}
      <p className="quota-alerts-note">
        {readingOff ? dict.quota.readingOffHint : dict.quota.alertsOff}
      </p>
    </section>
  );
}

function SessionsPanel({ view, lang, state, signal, onSignal }) {
  const dict = useDict(lang);
  if (signal) return <SignalDetail kind={signal} lang={lang} state={state} onBack={() => onSignal(null)} />;
  const stats = view.sessions;
  if (state === "empty") {
    return (
      <section className="panel">
        <div className="card">
          <EmptyNote text={dict.status.empty} />
        </div>
      </section>
    );
  }
  return (
    <section className="panel">
      <StatGrid
        items={[
          { label: dict.sessions.count, value: formatNumber(stats.count, lang) },
          { label: dict.sessions.average, value: formatDuration(stats.averageMinutes, lang) },
          { label: dict.sessions.projects, value: formatNumber(stats.projects, lang) },
        ]}
      />
      <div className="signal-head">
        <span>{dict.sessions.signals}</span>
        {state === "pending" && <small className="pending-flag">{dict.sessions.pending}</small>}
      </div>
      <div className="signal-grid">
        {SIGNALS.map(({ key, Icon, tone }) => (
          <button type="button" key={key} className="signal-card" onClick={() => onSignal(key)}>
            <span>
              <Icon size={14} weight="bold" className={`tone-text-${tone}`} />
              {dict.sessions[key]}
              <CaretRight size={12} />
            </span>
            <strong>{signalSummary(key, lang, state)}</strong>
          </button>
        ))}
      </div>
      <div className="card">
        <div className="card-head">
          <span>{dict.sessions.byProject}</span>
        </div>
        {stats.byProject.map((item) => (
          <div className="list-row" key={item.project}>
            <b>{item.project}</b>
            <small>
              {formatNumber(item.sessions, lang)} {dict.sessions.count}
            </small>
            <strong>{formatDuration(item.minutes, lang)}</strong>
          </div>
        ))}
      </div>
      <div className="card">
        <div className="card-head">
          <span>{dict.sessions.recent}</span>
        </div>
        {stats.recent.map((item) => (
          <div className="list-row" key={item.sessionId}>
            <b>{item.project}</b>
            <small>
              {dict.clients[item.client]} · {item.model}
            </small>
            <strong>{formatDuration(item.minutes, lang)}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

// pending 是可读旧快照缺少工作信号字段的状态；unavailable 留给整份快照不可读。
function signalsFor(state) {
  return state === "pending" ? PENDING_CAPTURE : WORK_SIGNALS;
}

function signalSummary(kind, lang, state) {
  const dict = catalogs[lang];
  const data = signalsFor(state);
  if (kind === "activity") {
    const top = data.activity[0];
    return `${dict.sessions.activityKinds[top.key]} ${formatShare(top.share, lang)}`;
  }
  if (kind === "workflow") {
    return `${dict.sessions.firstEdit} ${formatDuration(data.workflow.firstEditMinutes, lang)}`;
  }
  return `${formatNumber(data.tooling.calls, lang)} ${dict.sessions.toolCalls}`;
}

// 一个大类一行，点开露出它的子类。子类只在这一层出现：概览行始终是四个，
// 面板宽度是固定的 280pt，八个并列的类别在那个宽度里读不成。
function ActivityRow({ item, lang, expanded, onToggle }) {
  const dict = useDict(lang);
  return (
    <>
      <button
        type="button"
        className={`activity-row${expanded ? " is-open" : ""}`}
        aria-expanded={expanded}
        onClick={onToggle}
      >
        <CaretRight size={11} className="activity-caret" />
        <Row
          label={dict.sessions.activityKinds[item.key]}
          value={`${formatCost(item.cost, lang)} · ${formatNumber(item.events, lang)}`}
          share={item.share}
          tone={item.tone}
          lang={lang}
        />
      </button>
      {expanded && (
        <div className="activity-sub">
          {item.sub.map((child) => (
            <div className="list-row" key={child.key}>
              <b>{dict.sessions.subKinds[child.key]}</b>
              <small>{formatShare(child.share, lang)}</small>
              <strong>{formatCost(child.cost, lang)}</strong>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function SignalDetail({ kind, lang, state, onBack }) {
  const dict = useDict(lang);
  const { Icon, tone } = SIGNALS.find((item) => item.key === kind);
  const data = signalsFor(state);
  const pending = state === "pending";
  const [open, setOpen] = useState(null);
  return (
    <section className="panel">
      <div className="detail-head">
        <button type="button" onClick={onBack}>
          <CaretLeft size={14} />
          {dict.sessions.back}
        </button>
        <span>
          <Icon size={15} weight="bold" className={`tone-text-${tone}`} />
          {dict.sessions[kind]}
        </span>
      </div>
      {pending && (
        <div className="pending-banner">
          <Warning size={14} weight="fill" />
          <span>{dict.sessions.pendingHint}</span>
        </div>
      )}
      {kind === "activity" && (
        <div className="card">
          {data.activity.map((item) =>
            item.sub ? (
              <ActivityRow
                key={item.key}
                item={item}
                lang={lang}
                expanded={open === item.key}
                onToggle={() => setOpen(open === item.key ? null : item.key)}
              />
            ) : (
              <Row
                key={item.key}
                label={dict.sessions.activityKinds[item.key]}
                value={`${formatCost(item.cost, lang)} · ${formatNumber(item.events, lang)}`}
                share={item.share}
                tone={item.tone}
                lang={lang}
              />
            ),
          )}
        </div>
      )}
      {kind === "workflow" && (
        <>
          <div className="metric-grid">
            {[
              { label: dict.sessions.firstEdit, value: formatDuration(data.workflow.firstEditMinutes, lang), note: dict.sessions.median },
              { label: dict.sessions.filesTouched, value: formatNumber(data.workflow.filesTouched, lang) },
              { label: dict.sessions.retries, value: formatNumber(data.workflow.retries, lang), note: dict.sessions.retriesNote },
              { label: dict.sessions.editsPerSession, value: formatNumber(data.workflow.editsPerSession, lang) },
            ].map((item) => (
              <div key={item.label}>
                <span>{item.label}</span>
                <strong>{item.value}</strong>
                {item.note && <small>{item.note}</small>}
              </div>
            ))}
          </div>
          <div className="inline-row">
            <span>{dict.sessions.topFile}</span>
            <strong>
              {data.workflow.topFile} ×{data.workflow.topFileCount}
            </strong>
          </div>
        </>
      )}
      {kind === "tooling" && (
        <>
          <div className="card">
            {data.tooling.rows.map((item) => (
              <div className="list-row" key={item.key}>
                <b>{dict.sessions.toolKinds[item.key]}</b>
                <small>
                  {formatNumber(item.calls, lang)} {dict.sessions.toolCalls}
                </small>
                <strong>{formatShare(item.share, lang)}</strong>
              </div>
            ))}
            {data.tooling.rows.length === 0 && <EmptyNote text={dict.sessions.pendingHint} />}
          </div>
          <div className="inline-row">
            <span>{dict.sessions.topServer}</span>
            <strong>
              {data.tooling.topServer} · {formatNumber(data.tooling.topServerCalls, lang)}
            </strong>
          </div>
        </>
      )}
    </section>
  );
}

// D6：标记只是指针，解释在面板体里。四个面板各自换成归因后的不可用行，
// 而不是整屏接管——整屏接管是 unavailable 态（连快照都没有）的处理。
function SchemaPanel({ lang, label }) {
  const dict = useDict(lang);
  return (
    <section className="panel">
      <div className="card">
        <div className="domain-missing">
          <WarningCircle size={20} />
          <strong>{dict.status.schemaSignalSectionUnavailable}</strong>
          <small>{label}</small>
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ 节律块 */

function RhythmBlock({ lang, state }) {
  const dict = useDict(lang);
  const data = rhythm.all; // 固定近 30 天全部客户端，不受上方筛选影响
  const [hover, setHover] = useState(null);
  const calendar = useMemo(() => scope("all", "30d").daily, []);
  const maxDaily = Math.max(...calendar.map((item) => item.value), 0.0001);
  const level = (value) => (value <= 0 ? 0 : Math.min(5, Math.max(1, Math.ceil((value / maxDaily) * 5))));

  if (state === "unavailable" || SCHEMA_STATES.includes(state)) return null;

  return (
    <section className="rhythm" aria-labelledby="rhythm-title">
      <div className="rhythm-head">
        <h2 id="rhythm-title">
          <ClockCounterClockwise size={15} weight="bold" />
          {dict.rhythm.title}
        </h2>
        <small>{dict.rhythm.scope}</small>
      </div>
      <StatGrid
        items={[
          { label: dict.rhythm.active, value: `${data.activeDays} / 30`, tone: "accent" },
          { label: dict.rhythm.busiest, value: dict.rhythm.weekdays[data.busiestDay] },
          { label: dict.rhythm.quietest, value: dict.rhythm.weekdays[data.quietestDay] },
          { label: dict.rhythm.peak, value: formatHourRangeShort(data.peakStart, data.peakEnd, lang) },
        ]}
      />

      <div className="section-label">
        <span>{dict.rhythm.hourOfWeek}</span>
        <small>{hover ? `${dict.rhythm.weekdays[hover.weekday]} ${formatHourRange(hover.hour, hover.hour + 1)}` : ""}</small>
      </div>
      <div className="hour-axis" aria-hidden="true">
        {["00", "06", "12", "18", "24"].map((mark) => (
          <span key={mark}>{mark}</span>
        ))}
      </div>
      <div className="heat" onMouseLeave={() => setHover(null)}>
        {[1, 2, 3, 4, 5, 6, 0].map((weekday) => (
          <div className="heat-row" key={weekday}>
            <span>{dict.rhythm.weekdaysShort[weekday]}</span>
            <div>
              {data.cells
                .filter((cell) => cell.weekday === weekday)
                .map((cell) => (
                  <i
                    key={cell.hour}
                    className={`level-${cell.intensity}`}
                    onMouseEnter={() => setHover(cell)}
                    title={`${dict.rhythm.weekdays[weekday]} ${formatHourRange(cell.hour, cell.hour + 1)}`}
                  />
                ))}
            </div>
          </div>
        ))}
      </div>
      <Legend dict={dict} />

      <div className="section-label">
        <span>{dict.rhythm.calendar}</span>
        <small>
          {formatDate(meta.firstDate, lang)} – {formatDate(meta.lastDate, lang)}
        </small>
      </div>
      <div className="calendar" aria-label={dict.rhythm.calendar}>
        {calendar.map((item) => (
          <i
            key={item.iso}
            className={`level-${level(item.value)}`}
            title={`${formatWeekdayDate(item.date, lang)} · ${formatCost(item.value, lang)}`}
          />
        ))}
      </div>
      <Legend dict={dict} />

      <StatGrid
        items={[
          { label: dict.rhythm.streak, value: dict.rhythm.days(data.longestStreak) },
          { label: dict.rhythm.lateNight, value: formatShare(data.lateNightShare, lang) },
          { label: dict.rhythm.weekend, value: formatShare(data.weekendShare, lang) },
        ]}
      />
    </section>
  );
}

function Legend({ dict }) {
  return (
    <div className="legend" aria-hidden="true">
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

/* ------------------------------------------------------- 状态条 / 服务商 / 弹窗 */

// 提示条统一放在内容区顶部：它是内容的一部分，跟着内容滚，
// 既不像浮层那样压住下面的数据，也不去挤已经很窄的 footer。
function Notices({ lang, state, healthRecovery, refreshIssue, onOpenHealth }) {
  const dict = useDict(lang);
  const schema = SCHEMA_STATES.includes(state);
  const health = healthOf(state, healthRecovery);
  const failing = health.checks.filter((check) => check.status !== "ok");
  const rows = [];
  if (state === "unavailable" && refreshIssue !== "firstFailure") rows.push({ key: "unreadable", tone: "bad", text: dict.status.unreadable });
  if (refreshIssue === "retainedFailure") rows.push({ key: "refresh", tone: "bad", text: dict.status.refreshRetained, announce: false });
  if (refreshIssue === "storageFailure") rows.push({ key: "storage", tone: "warn", text: dict.status.storageFailed, announce: false });
  if (refreshIssue === "firstFailure") rows.push({ key: "first-refresh", tone: "bad", text: dict.status.firstRefreshFailed, announce: false });
  // 刷新不上的快照没法断言这个条件此刻还成立，只能断言取快照时成立，所以它排在因由前面。
  if (state === "schemaStacked") rows.push({ key: "offline", tone: "bad", text: dict.status.offline });
  // 因由排在症状前面，而症状不再单独成行：partial 说的是这一条已经解释过的事。
  if (schema) {
    rows.push({ key: "schema", tone: "bad", text: dict.status.schemaSignalNotice, action: onOpenHealth });
  }
  if (healthRecovery !== "baseline") {
    const scenarios = healthRecovery === "combined" ? ["stateLive", "extensionStale"] : [healthRecovery];
    for (const scenario of scenarios) {
      rows.push({
        key: `health-recovery.${scenario}`,
        tone: scenario === "syncIncomplete" ? "bad" : "warn",
        text: dict.status.healthRecoveryNotices[scenario],
        action: onOpenHealth,
      });
    }
  }
  if (!schema && state === "partial") {
    rows.push({ key: "partial", tone: "warn", text: dict.status.partial });
  }
  // 计数条只在还有别的东西可数时才是信息：problems === 1 时它说的正是上面那一行。
  const countIsRedundant = schema && failing.length <= 1;
  if (failing.length > 0 && state !== "unavailable" && !countIsRedundant && healthRecovery === "baseline") {
    rows.push({
      key: "health",
      tone: schema ? "bad" : "warn",
      text: dict.status.healthProblem(failing.length),
      action: onOpenHealth,
    });
  }
  // 独立会话库不可读不是核心库版本超前的后果；URL 开关覆盖两者并存的标本。
  if (schema && new URLSearchParams(window.location.search).get("sessions") === "unavailable") {
    rows.push({ key: "sessions_unavailable", tone: "warn", text: dict.status.warningSessionsUnavailable });
  }
  if (rows.length === 0) return null;
  return (
    <div className="notices">
      {rows.map((row) =>
        row.action ? (
          <button type="button" key={row.key} className={`notice tone-${row.tone}`} onClick={row.action}>
            <Warning size={13} weight="fill" />
            <span>{row.text}</span>
            <CaretRight size={12} />
          </button>
        ) : (
          <div key={row.key} className={`notice tone-${row.tone}`} role={row.announce === false ? undefined : "status"}>
            <WarningCircle size={13} weight="fill" />
            <span>{row.text}</span>
          </div>
        ),
      )}
    </div>
  );
}

// 健康详情做成二级页面，和工作信号详情同一套模式：
// 展开式的行内列表要么挡住内容，要么把 footer 顶变形，这里两个问题都不存在。
function recoveryDetail(check, dict) {
  const key = check.code === "lock_live" ? `${check.resource}Live` : check.code;
  return dict.status.healthRecoveryDetails[key] ?? null;
}

function HealthCopyAction({ check, detail, dict }) {
  const [copied, setCopied] = useState(false);
  const command = check.recovery_command ?? check.diagnostic_command ?? null;
  const copyValue = check.copy_kind === "manual_prerequisite" ? detail?.manual : command;
  if (!copyValue) return null;
  const label = check.copy_kind === "manual_prerequisite"
    ? dict.status.copySafetySteps
    : check.action_kind === "synchronize_inventory"
      ? dict.status.copySyncCommand
      : dict.status.copyDiagnosticCommand;
  const copy = async () => {
    await navigator.clipboard?.writeText(copyValue);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };
  return (
    <div className="health-copy-action" data-action-kind={check.action_kind ?? "none"}>
      {command && <code>{command}</code>}
      <button type="button" onClick={copy}>{label}</button>
      <span role="status" aria-live="polite">{copied ? dict.status.copied : ""}</span>
    </div>
  );
}

function HealthDetail({ lang, state, healthRecovery, onBack }) {
  const dict = useDict(lang);
  const health = healthOf(state, healthRecovery);
  return (
    <section className="panel">
      <div className="detail-head">
        <button type="button" onClick={onBack}>
          <CaretLeft size={14} />
          {dict.sessions.back}
        </button>
        <span>
          <Warning size={15} weight="fill" className="tone-text-warn" />
          {dict.status.healthTitle}
        </span>
      </div>
      <div className="card">
        {health.checks.map((check) => {
          const detail = recoveryDetail(check, dict);
          const expanded = check.code === "schema_ahead" || check.code === "hook_deliveries_dropped" || !!detail;
          return (
          <div className={`list-row${expanded ? " expanded" : ""}`} data-health-code={check.code ?? "ok"} key={`${check.name}.${check.code ?? "ok"}`}>
            <b>{dict.status.checks[check.name]}</b>
            <small />
            <strong className={check.status === "failed" ? "tone-text-bad" : check.status === "warning" ? "tone-text-warn" : "tone-text-good"}>
              {dict.status.checkStatus[check.status]}
            </strong>
            {/* 两行都是散文：这一条没有可运行的命令，所以既不等宽也没有复制按钮。 */}
            {check.code === "schema_ahead" && (
              <div className="row-detail">
                <p>{dict.status.schemaSignalCause(check.count, check.supported_count)}</p>
                <p>{dict.status.schemaSignalRecovery}</p>
              </div>
            )}
            {/* 丢弃计数是这一条唯一的信息量，不给它因由行就只剩一个没有解释的警告。
                同样没有可运行的命令，所以只有一行、没有复制按钮。 */}
            {check.code === "hook_deliveries_dropped" && (
              <div className="row-detail">
                <p>{dict.status.schemaSignalHookDropped(check.count)}</p>
              </div>
            )}
            {detail && (
              <div className="row-detail health-recovery-detail">
                <p>{detail.cause}</p>
                <p>{detail.next}</p>
                {detail.effect && <p>{detail.effect}</p>}
                {detail.manual && <p><strong>{dict.status.manualPrerequisite}</strong> {detail.manual}</p>}
                <HealthCopyAction check={check} detail={detail} dict={dict} />
              </div>
            )}
          </div>
          );
        })}
      </div>
      <p className="detail-note">{dict.status.healthNote}</p>
    </section>
  );
}

function ProviderMenu({ lang, schema, onChoose, onClose }) {
  const dict = useDict(lang);
  const ref = useRef(null);
  useEffect(() => {
    const onDown = (event) => {
      if (!ref.current?.contains(event.target)) onClose();
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [onClose]);
  // 弹层 250 pt 宽且会换行，所以这里放得下一整句，不必压缩成 footer 那样的短语。
  if (schema) {
    return (
      <div className="provider-menu" ref={ref} role="status">
        <p className="menu-note">{dict.status.schemaSignalSwitchUnavailable}</p>
      </div>
    );
  }
  return (
    <div className="provider-menu" ref={ref} role="menu">
      {["codex", "claude"].map((client) => (
        <div key={client}>
          <small>{dict.clients[client]}</small>
          {PROVIDER.candidates
            .filter((item) => item.client === client)
            .map((item) => (
              <button
                type="button"
                role="menuitem"
                key={`${item.client}-${item.provider}`}
                disabled={item.reason === "wrapper_not_configured"}
                onClick={() => onChoose(item)}
              >
                <span>
                  <b>{item.provider}</b>
                  <em>{item.reason ? dict.reasons[item.reason] : dict.footer.ready}</em>
                </span>
                {item.reason === "already_selected" && <Check size={14} weight="bold" />}
              </button>
            ))}
        </div>
      ))}
    </div>
  );
}

function ConfirmDialog({ pending, lang, onCancel, onConfirm }) {
  const dict = useDict(lang);
  const ref = useRef(null);
  useEffect(() => {
    ref.current?.focus();
  }, []);
  return (
    <div className="dialog-backdrop" onMouseDown={onCancel}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        onMouseDown={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          if (event.key !== "Tab") return;
          const nodes = event.currentTarget.querySelectorAll("button");
          const first = nodes[0];
          const last = nodes[nodes.length - 1];
          if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
          }
        }}
      >
        <ArrowsClockwise size={22} />
        <h2 id="dialog-title">{dict.confirm.title(pending.provider, dict.clients[pending.client])}</h2>
        <p>{dict.confirm.body}</p>
        <div>
          <button type="button" ref={ref} onClick={onCancel}>
            {dict.confirm.cancel}
          </button>
          <button type="button" className="primary" onClick={onConfirm}>
            {dict.confirm.ok}
          </button>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ Popover */

export function Popover({ lang, state = "normal", quotaState = "normal", refreshScenario = "idle", healthRecovery = "baseline", embedded = false, width = "420", scan = null, onClientChange }) {
  const dict = useDict(lang);
  const [client, setClientState] = useState("all");
  const setClient = (value) => {
    setClientState(value);
    onClientChange?.(value);
  };
  const [period, setPeriod] = useState("today");
  // ?tab= 与 ?signal= 让每个面板都能直接链接过去，评审时不必一路点过来
  const params = typeof window === "undefined" ? null : new URLSearchParams(window.location.search);
  const [tab, setTab] = useState(() => {
    const value = params?.get("tab");
    return TABS.some((item) => item.key === value) ? value : TABS[0].key;
  });
  const [signal, setSignal] = useState(() => {
    const value = params?.get("signal");
    return ["activity", "workflow", "tooling"].includes(value) ? value : null;
  });
  // 额度状态由舞台上方独立开关传入；它与 usage 的 state 正交。
  const quotaVariant = quotaState;
  // 页脚的 provider 与额度面板读同一个对象：一个说 official 另一个说 aigocode
  // 是这份原型最该防的同屏矛盾。
  const quotaRoutes = quota(quotaVariant).routes ?? PROVIDER.routes;
  const initialRefresh = refreshPresentation(refreshScenario, state);
  const [refreshStatus, setRefreshStatus] = useState(initialRefresh.status);
  const [refreshIssue, setRefreshIssue] = useState(initialRefresh.issue);
  const [ageMinutes, setAgeMinutes] = useState(initialRefresh.age);
  const [providerMenu, setProviderMenu] = useState(false);
  const [pending, setPending] = useState(null);
  const [healthOpen, setHealthOpen] = useState(false);
  // 弹层锚在面板坐标系里：卡片在可滚动区内，滚动后它的 offsetTop 会变，
  // 所以位置在打开的那一刻用两个 rect 相减算出来，不缓存卡片的布局位置。
  const [creditsFlyout, setCreditsFlyout] = useState(null);
  const rootRef = useRef(null);
  // 关闭要延后一拍，因为「离开触发行」和「进入明细」是两个先后到达的事件：
  // 立刻关闭会在指针还在 10px 间隙里时把明细卸载掉，用户永远够不到它。
  // 进入明细会取消这次关闭，此后只要指针还在明细上就不再有计时器——
  // 阅读期间不会被任何超时打断，这不是"把定时器调长一点"。
  const creditsHold = useRef(null);
  const creditsNode = useRef(null);
  const cancelCreditsClose = useCallback(() => {
    window.clearTimeout(creditsHold.current);
    creditsHold.current = null;
  }, []);
  const closeCreditsSoon = useCallback(() => {
    window.clearTimeout(creditsHold.current);
    creditsHold.current = window.setTimeout(() => setCreditsFlyout(null), CREDITS_CLOSE_GRACE);
  }, []);
  const openCredits = useCallback((entry, anchorNode) => {
    window.clearTimeout(creditsHold.current);
    creditsHold.current = null;
    if (!entry || !anchorNode || !rootRef.current) {
      setCreditsFlyout(null);
      return;
    }
    const rootBox = rootRef.current.getBoundingClientRect();
    const anchorBox = anchorNode.getBoundingClientRect();
    // 左右由实际可用空间决定：菜单栏图标可能贴着屏幕右缘，固定朝右会把明细
    // 推出可视区。右侧放不下就朝左，两侧都放不下才盖在面板内侧。
    const room = FLYOUT_WIDTH + FLYOUT_GAP;
    const side =
      rootBox.right + room <= window.innerWidth
        ? "right"
        : rootBox.left - room >= 0
          ? "left"
          : "overlay";
    setCreditsFlyout({
      client: entry.client,
      entry,
      anchorNode,
      side,
      top: Math.max(8, anchorBox.top - rootBox.top - 10),
    });
  }, []);
  const [toast, setToast] = useState("");
  const attempts = useRef(0);
  const timer = useRef(null);
  const toastTimer = useRef(null);

  useEffect(
    () => () => {
      window.clearTimeout(timer.current);
      window.clearTimeout(toastTimer.current);
      window.clearTimeout(creditsHold.current);
    },
    [],
  );

  useEffect(() => {
    const next = refreshPresentation(refreshScenario, state);
    setRefreshStatus(next.status);
    setRefreshIssue(next.issue);
    setAgeMinutes(next.age);
    attempts.current = ["failed", "storageFailed", "firstFailure"].includes(refreshScenario) ? 1 : 0;
  }, [refreshScenario, state]);

  // 弹层锚在某一次布局中的按钮上；切额度状态、切宽度或滚动后那个坐标都失效。
  // 与其让明细漂在旧位置，直接关闭，下一次打开再从当前 rect 计算。
  useEffect(() => {
    setCreditsFlyout(null);
  }, [quotaVariant, width]);

  // 明细打开期间，唯一的判据是指针到底在不在「触发行 + 通路 + 明细」里。
  // 之前靠 mouseleave/mouseenter 配一个宽限：那只在指针一口气跨过去时成立，
  // 慢慢挪或者在通路里停一下，宽限就先到期了。位置是可以直接问的，不必猜。
  useEffect(() => {
    if (!creditsFlyout) return undefined;
    const onMove = (event) => {
      const trigger = creditsFlyout.anchorNode?.getBoundingClientRect();
      const flyout = creditsNode.current?.getBoundingClientRect();
      if (!trigger || !flyout) return;
      if (withinCreditsRegion(event.clientX, event.clientY, trigger, flyout)) {
        cancelCreditsClose();
      } else {
        closeCreditsSoon();
      }
    };
    // 指针整个移出窗口时不再有 mousemove，单靠上面那条判据会一直开着。
    const onOut = (event) => {
      if (!event.relatedTarget) closeCreditsSoon();
    };
    window.addEventListener("mousemove", onMove);
    document.addEventListener("mouseout", onOut);
    return () => {
      window.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseout", onOut);
    };
  }, [creditsFlyout, cancelCreditsClose, closeCreditsSoon]);

  const view = useMemo(() => scope(client, period), [client, period]);
  const unavailable = state === "unavailable" || refreshIssue === "firstFailure";
  const effectiveState = unavailable ? "unavailable" : state;
  // schema 态：快照是新的、也读得到，读不到的是核心库。所以时间戳照常显示，
  // 而每一个数据域都空——两件事在这一态里同时为真，unavailable 态里不是。
  const schema = SCHEMA_STATES.includes(state);
  const noData = unavailable || schema;

  const showToast = (message) => {
    window.clearTimeout(toastTimer.current);
    setToast(message);
    toastTimer.current = window.setTimeout(() => setToast(""), 2000);
  };

  const refresh = useCallback(() => {
    if (scan) {
      if (!scan.active) scan.restart();
      return;
    }
    if (refreshStatus === "refreshing") return;
    window.clearTimeout(timer.current);
    attempts.current += 1;
    setRefreshStatus("refreshing");
    setRefreshIssue(null);
    timer.current = window.setTimeout(() => {
      if (attempts.current === 1) {
        setRefreshStatus("failed");
        setRefreshIssue(unavailable ? "firstFailure" : "retainedFailure");
        return;
      }
      setRefreshStatus("success");
      setRefreshIssue(null);
      setAgeMinutes(0);
      timer.current = window.setTimeout(() => setRefreshStatus("idle"), 1600);
    }, 1100);
  }, [scan, refreshStatus, unavailable]);

  const choose = (item) => {
    setProviderMenu(false);
    if (item.reason === "already_selected") {
      showToast(dict.confirm.already(item.provider, dict.clients[item.client]));
      return;
    }
    setPending(item);
  };

  useEffect(() => {
    const onKey = (event) => {
      if (event.key === "Escape") {
        if (pending) setPending(null);
        else if (providerMenu) setProviderMenu(false);
        else if (signal) setSignal(null);
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "r") {
        event.preventDefault();
        refresh();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [pending, providerMenu, signal, refresh]);

  const heroCost = noData ? null : state === "empty" ? 0 : view.totals.cost;
  const incomplete = !noData && state !== "empty" && !view.pricingComplete;
  // 面板体、footer、弹层三处的归因文案：同一个条件谓词，三种长度。
  const providerText = schema
    ? dict.status.schemaSignalFooter
    : quotaRoutes.map((route) => `${dict.clients[route.client]} ${route.provider}`).join(" · ");

  const visibleRefreshStatus = scan ? (scan.active ? "refreshing" : scan.phase === "completed" ? "success" : "idle") : refreshStatus;
  const refreshAnnouncement =
    refreshIssue === "retainedFailure"
      ? dict.status.refreshRetained
      : refreshIssue === "storageFailure"
        ? dict.status.storageFailed
        : refreshIssue === "firstFailure"
          ? dict.status.firstRefreshFailed
          : visibleRefreshStatus === "refreshing"
            ? dict.refreshing
            : visibleRefreshStatus === "success"
              ? dict.updated
              : "";
  if (scan && !scan.hasSnapshot) return <ScanEmptyPopover scan={scan} lang={lang} width={width} />;

  return (
    <section
      ref={rootRef}
      data-scan-enabled={scan ? "true" : undefined}
      data-scan-snapshot={scan ? (scan.published ? "new" : "previous") : undefined}
      className={`popover${embedded ? " embedded" : ""}${String(width) === "280" ? " narrow" : ""}${
        creditsFlyout ? " flyout-open" : ""
      }`}
      style={{ "--popover-w": `${width}px` }}
      data-width={width}
      data-health-scenario={healthRecovery}
      aria-label={dict.app}
    >
      {creditsFlyout && (
        <CreditsFlyout
          side={creditsFlyout.side}
          entry={creditsFlyout.entry}
          lang={lang}
          now={quotaMeta.now}
          top={creditsFlyout.top}
          onClose={() => {
            cancelCreditsClose();
            setCreditsFlyout(null);
          }}
          nodeRef={creditsNode}
        />
      )}
      <header>
        <div className="brand">
          <img src="/agentdeck-robot.png" alt="" width={22} height={22} />
          <strong>{dict.app}</strong>
        </div>
        <div className="header-right">
          {!unavailable && (
            <span className="freshness">
              {visibleRefreshStatus === "success" ? dict.status.justNow : relativeTime(ageMinutes, lang)}
            </span>
          )}
          <button
            type="button"
            className={`refresh${visibleRefreshStatus === "failed" ? " failed" : ""}`}
            onClick={refresh}
            aria-disabled={scan ? scan.active : visibleRefreshStatus === "refreshing"}
            aria-label={visibleRefreshStatus === "failed" ? dict.refreshRetryLabel : `${dict.refresh} ⌘R`}
          >
            {visibleRefreshStatus === "failed" ? (
              <WarningCircle size={14} weight="fill" />
            ) : visibleRefreshStatus === "refreshing" ? (
              <SpinnerGap size={15} className="spin" />
            ) : visibleRefreshStatus === "success" ? (
              <Check size={15} weight="bold" />
            ) : (
              <ArrowClockwise size={15} />
            )}
            <span>
              {visibleRefreshStatus === "failed"
                ? dict.retry
                : visibleRefreshStatus === "refreshing"
                ? dict.refreshing
                : visibleRefreshStatus === "success"
                  ? dict.updated
                  : dict.refresh}
            </span>
          </button>
        </div>
        {!scan && (
          <span className="visually-hidden" role="status" aria-live="polite" aria-atomic="true">
            {refreshAnnouncement}
          </span>
        )}
      </header>

      {scan && <ScanStatus scan={scan} lang={lang} />}
      <div className="segmented clients" role="tablist" aria-label={dict.clients.all}>
        {["all", "codex", "claude"].map((key) => (
          <button
            type="button"
            key={key}
            role="tab"
            aria-selected={client === key}
            className={client === key ? "active" : ""}
            onClick={() => setClient(key)}
          >
            {dict.clients[key]}
            <b>{noData ? "—" : formatCost(state === "empty" ? 0 : scope(key, period).totals.cost, lang, { compact: true })}</b>
          </button>
        ))}
      </div>

      <div className="hero">
        <div>
          <span>
            {dict.periods[period]} ·{" "}
            {period === "today"
              ? formatWeekdayDate(meta.today, lang)
              : `${formatDate(view.window[0].date, lang)} – ${formatDate(meta.today, lang)}`}
          </span>
          <strong className={incomplete ? "incomplete" : undefined}>
            {heroCost == null ? "—" : `${incomplete ? "≈" : ""}${formatCost(heroCost, lang)}`}
          </strong>
          {incomplete && (
            <small className="hero-note tone-text-warn">{dict.status.costIncomplete(view.pricing.unpricedEvents)}</small>
          )}
        </div>
        <div>
          <strong>{noData ? "—" : formatTokens(state === "empty" ? 0 : view.totals.tokens)}</strong>
          <span>
            {noData
              ? schema
                ? dict.status.schemaSignalSectionUnavailable
                : dict.status.unavailable
              : `${formatNumber(state === "empty" ? 0 : view.totals.events, lang)} ${dict.hero.events} · ${formatNumber(
                  state === "empty" ? 0 : view.totals.sessions,
                  lang,
                )} ${dict.hero.sessions} · ${formatNumber(state === "empty" ? 0 : view.sessions.projects, lang)} ${
                  dict.hero.projects
                }`}
          </span>
        </div>
      </div>

      <div className="segmented periods" role="tablist" aria-label={dict.periods.today}>
        {["today", "7d", "30d"].map((key) => (
          <button
            type="button"
            key={key}
            role="tab"
            aria-selected={period === key}
            className={period === key ? "active" : ""}
            onClick={() => setPeriod(key)}
          >
            {dict.periodsShort[key]}
          </button>
        ))}
      </div>

      <nav className="tabs" role="tablist" aria-label={dict.tabs.usage}>
        {TABS.map(({ key, Icon }) => (
          <button
            type="button"
            key={key}
            data-tab={key}
            role="tab"
            aria-selected={tab === key}
            aria-label={dict.tabs[key]}
            title={dict.tabs[key]}
            className={tab === key ? "active" : ""}
            onClick={() => {
              setTab(key);
              setSignal(null);
              setCreditsFlyout(null);
            }}
          >
            <Icon size={14} weight={tab === key ? "fill" : "regular"} />
            <span className="tab-label">{dict.tabs[key]}</span>
            {((state === "partial" && key === "attribution") || schema) && (
              <i className="tab-warn" aria-label={schema ? dict.status.schemaSignalSectionUnavailable : dict.status.partial} />
            )}
          </button>
        ))}
      </nav>

      <div className="scroll" onScroll={() => setCreditsFlyout(null)}>
        {healthOpen ? (
          <HealthDetail lang={lang} state={state} healthRecovery={healthRecovery} onBack={() => setHealthOpen(false)} />
        ) : schema ? (
          <>
            <Notices lang={lang} state={effectiveState} healthRecovery={healthRecovery} refreshIssue={refreshIssue} onOpenHealth={() => setHealthOpen(true)} />
            <SchemaPanel lang={lang} label={dict.tabs[tab]} />
          </>
        ) : unavailable ? (
          <>
            <Notices lang={lang} state={effectiveState} healthRecovery={healthRecovery} refreshIssue={refreshIssue} onOpenHealth={() => setHealthOpen(true)} />
            <div className="unavailable">
              <WarningCircle size={26} />
              <p>{refreshIssue === "firstFailure" ? dict.status.noDataYet : dict.status.unavailable}</p>
            </div>
          </>
        ) : (
          <>
            <Notices lang={lang} state={effectiveState} healthRecovery={healthRecovery} refreshIssue={refreshIssue} onOpenHealth={() => setHealthOpen(true)} />
            {tab === "usage" && <UsagePanel view={view} lang={lang} state={state} />}
            {tab === "breakdown" && <BreakdownPanel view={view} lang={lang} state={state} />}
            {tab === "attribution" && <AttributionPanel view={view} lang={lang} state={state} />}
            {tab === "quota" && (
              <QuotaPanel
                lang={lang}
                variant={quotaVariant}
                narrow={String(width) === "280"}
                openCredits={openCredits}
                closeCredits={closeCreditsSoon}
                creditsOpenFor={creditsFlyout?.client ?? null}
              />
            )}
            {tab === "sessions" && (
              <SessionsPanel view={view} lang={lang} state={state} signal={signal} onSignal={setSignal} />
            )}
            <RhythmBlock lang={lang} state={state} />
          </>
        )}
      </div>

      <footer>
        <div className="footer-main">
          <button
            type="button"
            className="provider-entry"
            aria-expanded={providerMenu}
            aria-label={`${dict.footer.providers}: ${providerText}`}
            onClick={() => setProviderMenu((value) => !value)}
          >
            <span>{dict.footer.providers}</span>
            <strong>{providerText}</strong>
            <CaretUp size={13} className={providerMenu ? "flip" : undefined} />
          </button>
          {providerMenu && (
            <ProviderMenu lang={lang} schema={schema} onChoose={choose} onClose={() => setProviderMenu(false)} />
          )}
        </div>
      </footer>

      {pending && (
        <ConfirmDialog
          pending={pending}
          lang={lang}
          onCancel={() => setPending(null)}
          onConfirm={() => {
            showToast(dict.confirm.done(pending.provider, dict.clients[pending.client]));
            setPending(null);
          }}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={13} weight="bold" />
          {toast}
        </div>
      )}
    </section>
  );
}
