import { ChartBar, ClockCounterClockwise, WarningCircle } from "@phosphor-icons/react";
import { Popover, SURFACE_STATES } from "./Popover.jsx";
import { catalogs } from "./i18n.js";
import { StageControls, useStagePrefs } from "./Stage.jsx";

// 每个状态一句话，说明它由什么触发——原型上看得见状态长什么样，也看得见它为什么会出现。
const CAUSE = {
  zh: {
    normal: "快照可读、计价完整、刷新成功",
    empty: "快照可读，但所选范围内没有任何事件",
    aged: "超过 6 小时没有成功刷新，时间戳改用「上次更新于」",
    partial: "部分数据域不可用，或上一次刷新失败但保留了旧快照",
    unavailable: "没有快照，或缓存版本不受支持——一律不显示零，而是让用户去打开主程序",
  },
  en: {
    normal: "Snapshot readable, pricing complete, refresh succeeded",
    empty: "Snapshot readable, but no events in the selected range",
    aged: "No successful refresh for over 6 hours; the timestamp switches to “last updated”",
    partial: "Some domains unavailable, or the last refresh failed while an old snapshot was kept",
    unavailable: "No snapshot, or an unsupported cache version — never render a zero, ask the user to open the app",
  },
};

// widget 没有刷新状态机，它的降级只有这几种，其中占位骨架绝不能出现真实数字
function WidgetState({ kind, lang }) {
  const dict = catalogs[lang];
  const copy = {
    placeholder: { title: dict.widgets.kinds.magnitude.title, body: null },
    unavailable: { title: dict.widgets.kinds.magnitude.title, body: dict.status.unavailable },
    empty: { title: dict.widgets.kinds.magnitude.title, body: dict.status.empty },
    aged: { title: dict.widgets.kinds.rhythm.title, body: null },
  }[kind];
  return (
    <article className={`widget widget-small widget-state ${kind}`}>
      <header>
        <span>
          {kind === "aged" ? <ClockCounterClockwise size={12} weight="fill" /> : <ChartBar size={12} weight="fill" />}
          {copy.title}
        </span>
      </header>
      <div className="widget-body">
        {kind === "placeholder" && (
          <div className="skeleton">
            <i style={{ width: "62%", height: 20 }} />
            <i style={{ width: "80%", height: 8 }} />
            <i style={{ width: "100%", height: 34 }} />
          </div>
        )}
        {kind === "unavailable" && (
          <div className="widget-message">
            <WarningCircle size={18} />
            <span>{copy.body}</span>
          </div>
        )}
        {kind === "empty" && (
          <div className="widget-message">
            <strong>—</strong>
            <span>{copy.body}</span>
          </div>
        )}
        {kind === "aged" && (
          <div className="widget-message">
            <strong>23 / 30</strong>
            <span>{dict.rhythm.active}</span>
          </div>
        )}
      </div>
      <footer className={kind === "aged" ? "tone-text-warn" : undefined}>
        {kind === "placeholder"
          ? "—"
          : kind === "aged"
            ? dict.status.aged(lang === "zh" ? "9 小时前" : "9 hours ago")
            : dict.status.justNow}
      </footer>
    </article>
  );
}

export function StateBoard() {
  const prefs = useStagePrefs();
  const { lang, theme } = prefs;
  const dict = catalogs[lang];
  return (
    <main className="board states-board" data-theme={theme}>
      <StageControls prefs={prefs} showState={false} />
      <header className="board-head">
        <h1>{dict.states.boardTitle}</h1>
        <p>{dict.states.boardSubtitle}</p>
      </header>
      <div className="state-row">
        {SURFACE_STATES.map((state) => (
          <div className="state-cell" key={state}>
            <div className="state-label">
              <strong>{dict.states[state]}</strong>
              <small>{CAUSE[lang][state]}</small>
            </div>
            <div className="state-frame">
              <Popover lang={lang} state={state} embedded />
            </div>
          </div>
        ))}
      </div>
      <header className="board-head sub">
        <h2>{lang === "zh" ? "小组件降级" : "Widget degradation"}</h2>
        <p>
          {lang === "zh"
            ? "小组件没有刷新状态机，只按快照年龄与可用性降级；占位骨架绝不含真实数字。"
            : "A widget has no refresh state machine; it degrades purely on snapshot age and availability. The placeholder never carries real values."}
        </p>
      </header>
      <div className="widget-state-row">
        {["placeholder", "unavailable", "empty", "aged"].map((kind) => (
          <div key={kind}>
            <WidgetState kind={kind} lang={lang} />
            <small>
              {kind === "placeholder"
                ? lang === "zh"
                  ? "画廊占位"
                  : "Gallery placeholder"
                : kind === "unavailable"
                  ? dict.states.unavailable
                  : kind === "empty"
                    ? dict.states.empty
                    : dict.states.aged}
            </small>
          </div>
        ))}
      </div>
    </main>
  );
}
