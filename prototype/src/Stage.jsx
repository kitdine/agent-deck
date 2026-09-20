import { useEffect, useState } from "react";
import { catalogs } from "./i18n.js";

const STATES = ["normal", "empty", "aged", "partial", "pending", "unavailable", "schema", "schemaStacked"];

// 额度读取状态是独立的标本轴。它不能并进 STATES：用量数据状态与额度读取状态
// 可以同时发生，合成一个控件就无法检验“用量过期 + 额度也过期”。
const QUOTAS = ["normal", "bothOfficial", "codexPlus", "prose", "parseFailed", "stale", "neverProbed", "readingOff"];
const REFRESHES = ["idle", "refreshing", "failed", "storageFailed", "wake", "recovered", "firstFailure"];
const WIDGET_CLIENTS = ["codex", "claude"];

// 菜单栏图标可以贴在屏幕任意水平位置，而弹层朝左还是朝右正是由此决定。
// 舞台默认居中，居中时左右余量恒等，左侧分支就永远走不到——一条标本演示不了
// 的规则等于没有被评审过。这个轴让三种锚点都能看见。
const ANCHORS = ["left", "center", "right"];

// 面板的两个宽度：420 是常规宽度，280 是实现里 AGENTDECK_TEST_WIDTH 的窄边界。
// 有了它，「这一行在窄边界会不会被截断」才是标本阶段能判定的问题，而不是留给真机。
const WIDTHS = ["420", "280"];

// 原型舞台自己的控制条，不属于产品界面：切语言、切外观、切数据状态。
export function useStagePrefs() {
  const params = new URLSearchParams(window.location.search);
  const [lang, setLang] = useState(params.get("lang") === "en" ? "en" : "zh");
  const [theme, setTheme] = useState(params.get("theme") === "light" ? "light" : "dark");
  const [state, setState] = useState(STATES.includes(params.get("state")) ? params.get("state") : "normal");
  const [width, setWidth] = useState(WIDTHS.includes(params.get("width")) ? params.get("width") : "420");
  const [quota, setQuota] = useState(QUOTAS.includes(params.get("quota")) ? params.get("quota") : "normal");
  const [refresh, setRefresh] = useState(REFRESHES.includes(params.get("refresh")) ? params.get("refresh") : "idle");
  const [widgetClient, setWidgetClient] = useState(
    WIDGET_CLIENTS.includes(params.get("widgetClient")) ? params.get("widgetClient") : "codex",
  );
  const [anchor, setAnchor] = useState(ANCHORS.includes(params.get("anchor")) ? params.get("anchor") : "center");

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.setAttribute("lang", lang === "zh" ? "zh-Hans" : "en");
  }, [theme, lang]);

  return { lang, setLang, theme, setTheme, state, setState, width, setWidth, quota, setQuota, refresh, setRefresh, widgetClient, setWidgetClient, anchor, setAnchor };
}

function Group({ label, value, options, onChange }) {
  return (
    <div className="stage-group">
      <span>{label}</span>
      <div>
        {options.map(([key, text]) => (
          <button
            type="button"
            key={key}
            data-value={key}
            className={value === key ? "active" : ""}
            onClick={() => onChange(key)}
          >
            {text}
          </button>
        ))}
      </div>
    </div>
  );
}

export function StageControls({ prefs, showState = true, showWidth = true, showQuota = true, showRefresh = false, showWidgetClient = false, showAnchor = false }) {
  const { lang, setLang, theme, setTheme, state, setState, width, setWidth, quota, setQuota, refresh, setRefresh, widgetClient, setWidgetClient, anchor, setAnchor } = prefs;
  const dict = catalogs[lang];
  const surface = new URLSearchParams(window.location.search).get("surface");
  const link = (target, text) => {
    const next = new URLSearchParams(window.location.search);
    if (target) next.set("surface", target);
    else next.delete("surface");
    return (
      <a className={surface === target || (!surface && !target) ? "active" : ""} href={`?${next.toString()}`}>
        {text}
      </a>
    );
  };

  return (
    <div className="stage-controls">
      <nav className="stage-nav">
        {link(null, "Popover")}
        {link("widgets", lang === "zh" ? "小组件" : "Widgets")}
        {link("states", lang === "zh" ? "状态" : "States")}
        {link("cli", "CLI")}
        <a href="?scan=waiting">{lang === "zh" ? "扫描进度" : "Scan progress"}</a>
      </nav>
      <div className="stage-groups">
        {showState && (
          <Group
            label={dict.states.state}
            value={state}
            options={STATES.map((key) => [key, dict.states[key === "normal" ? "normal" : key]])}
            onChange={setState}
          />
        )}
        {showQuota && (
          <Group
            label={dict.states.quota}
            value={quota}
            options={QUOTAS.map((key) => [key, dict.states.quotaVariants[key]])}
            onChange={setQuota}
          />
        )}
        {showRefresh && (
          <Group
            label={dict.states.refresh}
            value={refresh}
            options={REFRESHES.map((key) => [key, dict.states.refreshVariants[key]])}
            onChange={setRefresh}
          />
        )}
        {showWidgetClient && (
          <Group
            label={dict.states.widgetClient}
            value={widgetClient}
            options={WIDGET_CLIENTS.map((key) => [key, dict.clients[key]])}
            onChange={setWidgetClient}
          />
        )}
        {showAnchor && (
          <Group
            label={dict.states.anchor}
            value={anchor}
            options={ANCHORS.map((key) => [key, dict.states.anchors[key]])}
            onChange={setAnchor}
          />
        )}
        {showWidth && (
          <Group
            label={dict.states.width}
            value={width}
            options={WIDTHS.map((key) => [key, `${key} pt`])}
            onChange={setWidth}
          />
        )}
        <Group
          label={dict.states.theme}
          value={theme}
          options={[
            ["dark", dict.states.themeDark],
            ["light", dict.states.themeLight],
          ]}
          onChange={setTheme}
        />
        <Group
          label={dict.states.language}
          value={lang}
          options={[
            ["zh", "中文"],
            ["en", "EN"],
          ]}
          onChange={setLang}
        />
      </div>
    </div>
  );
}

export { STATES, WIDTHS };
