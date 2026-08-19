import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  ShieldCheck,
  SpinnerGap,
  Warning,
  WarningCircle,
  Wrench,
} from "@phosphor-icons/react";
import { HEALTH, PENDING_CAPTURE, PROVIDER, buckets, meta, rhythm, scope } from "./data.js";
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

const TABS = [
  { key: "usage", Icon: ChartBar },
  { key: "breakdown", Icon: ChartPieSlice },
  { key: "attribution", Icon: ShieldCheck },
  { key: "sessions", Icon: Code },
];

// 数据状态。normal 以外的四种都是这一版新增的，用来检验界面在数据不好时还站不站得住。
export const SURFACE_STATES = ["normal", "empty", "aged", "partial", "unavailable"];

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
    <div className="stat-grid" style={{ "--columns": items.length }}>
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

function SessionsPanel({ view, lang, state, signal, onSignal }) {
  const dict = useDict(lang);
  if (signal) return <SignalDetail kind={signal} lang={lang} onBack={() => onSignal(null)} />;
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
        <small className="pending-flag">{dict.sessions.pending}</small>
      </div>
      <div className="signal-grid">
        {SIGNALS.map(({ key, Icon, tone }) => (
          <button type="button" key={key} className="signal-card" onClick={() => onSignal(key)}>
            <span>
              <Icon size={14} weight="bold" className={`tone-text-${tone}`} />
              {dict.sessions[key]}
              <CaretRight size={12} />
            </span>
            <strong>{signalSummary(key, lang)}</strong>
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

function signalSummary(kind, lang) {
  const dict = catalogs[lang];
  if (kind === "activity") {
    const top = PENDING_CAPTURE.activity[0];
    return `${dict.sessions.activityKinds[top.key]} ${formatShare(top.share, lang)}`;
  }
  if (kind === "workflow") {
    return `${dict.sessions.firstEdit} ${formatDuration(PENDING_CAPTURE.workflow.firstEditMinutes, lang)}`;
  }
  return `${formatNumber(PENDING_CAPTURE.tooling.calls, lang)} ${dict.sessions.toolCalls}`;
}

function SignalDetail({ kind, lang, onBack }) {
  const dict = useDict(lang);
  const { Icon, tone } = SIGNALS.find((item) => item.key === kind);
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
      <div className="pending-banner">
        <Warning size={14} weight="fill" />
        <span>{dict.sessions.pendingHint}</span>
      </div>
      {kind === "activity" && (
        <div className="card">
          {PENDING_CAPTURE.activity.map((item) => (
            <Row
              key={item.key}
              label={dict.sessions.activityKinds[item.key]}
              value={`${formatCost(item.cost, lang)} · ${formatNumber(item.events, lang)}`}
              share={item.share}
              tone={item.tone}
              lang={lang}
            />
          ))}
        </div>
      )}
      {kind === "workflow" && (
        <>
          <div className="metric-grid">
            {[
              { label: dict.sessions.firstEdit, value: formatDuration(PENDING_CAPTURE.workflow.firstEditMinutes, lang), note: dict.sessions.median },
              { label: dict.sessions.filesTouched, value: formatNumber(PENDING_CAPTURE.workflow.filesTouched, lang) },
              { label: dict.sessions.iterationDepth, value: PENDING_CAPTURE.workflow.iterationDepth, note: dict.sessions.turnsPerEdit },
              { label: dict.sessions.editsPerSession, value: formatNumber(PENDING_CAPTURE.workflow.editsPerSession, lang) },
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
              {PENDING_CAPTURE.workflow.topFile} ×{PENDING_CAPTURE.workflow.topFileCount}
            </strong>
          </div>
        </>
      )}
      {kind === "tooling" && (
        <>
          <div className="card">
            {PENDING_CAPTURE.tooling.rows.map((item) => (
              <div className="list-row" key={item.key}>
                <b>{dict.sessions.toolKinds[item.key]}</b>
                <small>
                  {formatNumber(item.calls, lang)} {dict.sessions.toolCalls}
                </small>
                <strong>{formatCost(item.cost, lang)}</strong>
              </div>
            ))}
          </div>
          <div className="inline-row">
            <span>{dict.sessions.topServer}</span>
            <strong>
              {PENDING_CAPTURE.tooling.topServer} · {formatNumber(PENDING_CAPTURE.tooling.topServerCalls, lang)}
            </strong>
          </div>
        </>
      )}
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

  if (state === "unavailable") return null;

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
function Notices({ lang, state, refreshFailed, onOpenHealth }) {
  const dict = useDict(lang);
  const failing = HEALTH.checks.filter((check) => check.status !== "ok");
  const rows = [];
  if (state === "unavailable") rows.push({ key: "unreadable", tone: "bad", text: dict.status.unreadable });
  if (state === "partial" || refreshFailed) rows.push({ key: "partial", tone: "warn", text: dict.status.partial });
  if (failing.length > 0 && state !== "unavailable") {
    rows.push({
      key: "health",
      tone: "warn",
      text: dict.status.healthProblem(failing.length),
      action: onOpenHealth,
    });
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
          <div key={row.key} className={`notice tone-${row.tone}`} role="status">
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
function HealthDetail({ lang, onBack }) {
  const dict = useDict(lang);
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
        {HEALTH.checks.map((check) => (
          <div className="list-row" key={check.name}>
            <b>{dict.status.checks[check.name]}</b>
            <small />
            <strong className={check.status === "failed" ? "tone-text-bad" : check.status === "warning" ? "tone-text-warn" : "tone-text-good"}>
              {dict.status.checkStatus[check.status]}
            </strong>
          </div>
        ))}
      </div>
      <p className="detail-note">{dict.status.healthNote}</p>
    </section>
  );
}

function ProviderMenu({ lang, onChoose, onClose }) {
  const dict = useDict(lang);
  const ref = useRef(null);
  useEffect(() => {
    const onDown = (event) => {
      if (!ref.current?.contains(event.target)) onClose();
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [onClose]);
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

export function Popover({ lang, state = "normal", embedded = false, onClientChange }) {
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
    return TABS.some((item) => item.key === value) ? value : "usage";
  });
  const [signal, setSignal] = useState(() => {
    const value = params?.get("signal");
    return ["activity", "workflow", "tooling"].includes(value) ? value : null;
  });
  const [refreshStatus, setRefreshStatus] = useState("idle");
  const [ageMinutes, setAgeMinutes] = useState(state === "aged" ? 512 : 0);
  const [providerMenu, setProviderMenu] = useState(false);
  const [pending, setPending] = useState(null);
  const [healthOpen, setHealthOpen] = useState(false);
  const [toast, setToast] = useState("");
  const attempts = useRef(0);
  const timer = useRef(null);
  const toastTimer = useRef(null);

  useEffect(
    () => () => {
      window.clearTimeout(timer.current);
      window.clearTimeout(toastTimer.current);
    },
    [],
  );

  useEffect(() => {
    setAgeMinutes(state === "aged" ? 512 : 0);
  }, [state]);

  const view = useMemo(() => scope(client, period), [client, period]);
  const unavailable = state === "unavailable";

  const showToast = (message) => {
    window.clearTimeout(toastTimer.current);
    setToast(message);
    toastTimer.current = window.setTimeout(() => setToast(""), 2000);
  };

  const refresh = useCallback(() => {
    window.clearTimeout(timer.current);
    attempts.current += 1;
    setRefreshStatus("refreshing");
    timer.current = window.setTimeout(() => {
      if (attempts.current === 1) {
        setRefreshStatus("failed");
        return;
      }
      setRefreshStatus("success");
      setAgeMinutes(0);
      timer.current = window.setTimeout(() => setRefreshStatus("idle"), 1600);
    }, 1100);
  }, []);

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

  const heroCost = unavailable ? null : state === "empty" ? 0 : view.totals.cost;
  const incomplete = !unavailable && state !== "empty" && !view.pricingComplete;
  const providerText = PROVIDER.routes.map((route) => `${dict.clients[route.client]} ${route.provider}`).join(" · ");

  return (
    <section className={`popover${embedded ? " embedded" : ""}`} aria-label={dict.app}>
      <header>
        <div className="brand">
          <img src="/agentdeck-robot.png" alt="" width={22} height={22} />
          <strong>{dict.app}</strong>
        </div>
        <div className="header-right">
          {!unavailable && (
            <span className="freshness">
              {refreshStatus === "success" ? dict.status.justNow : relativeTime(ageMinutes, lang)}
            </span>
          )}
          {refreshStatus === "failed" ? (
          <div className="refresh failed" role="status">
            <WarningCircle size={14} weight="fill" />
            <span>{dict.refreshFailed}</span>
            <button type="button" onClick={refresh}>
              {dict.retry}
            </button>
          </div>
        ) : (
          <button
            type="button"
            className="refresh"
            onClick={refresh}
            disabled={refreshStatus === "refreshing"}
            aria-label={`${dict.refresh} ⌘R`}
          >
            {refreshStatus === "refreshing" ? (
              <SpinnerGap size={15} className="spin" />
            ) : refreshStatus === "success" ? (
              <Check size={15} weight="bold" />
            ) : (
              <ArrowClockwise size={15} />
            )}
            <span>
              {refreshStatus === "refreshing"
                ? dict.refreshing
                : refreshStatus === "success"
                  ? dict.updated
                  : dict.refresh}
            </span>
          </button>
          )}
        </div>
      </header>

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
            <b>{unavailable ? "—" : formatCost(state === "empty" ? 0 : scope(key, period).totals.cost, lang, { compact: true })}</b>
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
          <strong>{unavailable ? "—" : formatTokens(state === "empty" ? 0 : view.totals.tokens)}</strong>
          <span>
            {unavailable
              ? dict.status.unavailable
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
            role="tab"
            aria-selected={tab === key}
            className={tab === key ? "active" : ""}
            onClick={() => {
              setTab(key);
              setSignal(null);
            }}
          >
            <Icon size={14} weight={tab === key ? "fill" : "regular"} />
            {dict.tabs[key]}
            {state === "partial" && key === "attribution" && <i className="tab-warn" aria-label={dict.status.partial} />}
          </button>
        ))}
      </nav>

      <div className="scroll">
        {healthOpen ? (
          <HealthDetail lang={lang} onBack={() => setHealthOpen(false)} />
        ) : unavailable ? (
          <>
            <Notices lang={lang} state={state} refreshFailed={refreshStatus === "failed"} onOpenHealth={() => setHealthOpen(true)} />
            <div className="unavailable">
              <WarningCircle size={26} />
              <p>{dict.status.unavailable}</p>
            </div>
          </>
        ) : (
          <>
            <Notices lang={lang} state={state} refreshFailed={refreshStatus === "failed"} onOpenHealth={() => setHealthOpen(true)} />
            {tab === "usage" && <UsagePanel view={view} lang={lang} state={state} />}
            {tab === "breakdown" && <BreakdownPanel view={view} lang={lang} state={state} />}
            {tab === "attribution" && <AttributionPanel view={view} lang={lang} state={state} />}
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
            onClick={() => setProviderMenu((value) => !value)}
          >
            <span>{dict.footer.providers}</span>
            <strong>{providerText}</strong>
            <CaretUp size={13} className={providerMenu ? "flip" : undefined} />
          </button>
          {providerMenu && <ProviderMenu lang={lang} onChoose={choose} onClose={() => setProviderMenu(false)} />}
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
