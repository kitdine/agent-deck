import { useEffect, useState } from "react";
import { catalogs } from "./i18n.js";

const STATES = ["normal", "empty", "aged", "partial", "pending", "unavailable", "schema", "schemaStacked"];

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

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.setAttribute("lang", lang === "zh" ? "zh-Hans" : "en");
  }, [theme, lang]);

  return { lang, setLang, theme, setTheme, state, setState, width, setWidth };
}

function Group({ label, value, options, onChange }) {
  return (
    <div className="stage-group">
      <span>{label}</span>
      <div>
        {options.map(([key, text]) => (
          <button type="button" key={key} className={value === key ? "active" : ""} onClick={() => onChange(key)}>
            {text}
          </button>
        ))}
      </div>
    </div>
  );
}

export function StageControls({ prefs, showState = true, showWidth = true }) {
  const { lang, setLang, theme, setTheme, state, setState, width, setWidth } = prefs;
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
