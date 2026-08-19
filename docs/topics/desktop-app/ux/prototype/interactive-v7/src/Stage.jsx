import { useEffect, useState } from "react";
import { catalogs } from "./i18n.js";

const STATES = ["normal", "empty", "aged", "partial", "unavailable"];

// 原型舞台自己的控制条，不属于产品界面：切语言、切外观、切数据状态。
export function useStagePrefs() {
  const params = new URLSearchParams(window.location.search);
  const [lang, setLang] = useState(params.get("lang") === "en" ? "en" : "zh");
  const [theme, setTheme] = useState(params.get("theme") === "light" ? "light" : "dark");
  const [state, setState] = useState(STATES.includes(params.get("state")) ? params.get("state") : "normal");

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.setAttribute("lang", lang === "zh" ? "zh-Hans" : "en");
  }, [theme, lang]);

  return { lang, setLang, theme, setTheme, state, setState };
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

export function StageControls({ prefs, showState = true }) {
  const { lang, setLang, theme, setTheme, state, setState } = prefs;
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

export { STATES };
