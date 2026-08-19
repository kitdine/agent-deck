import { useEffect, useRef, useState } from "react";
import { CaretRight, Check } from "@phosphor-icons/react";
import { Popover } from "./Popover.jsx";
import { DEFAULT_PREFS, SettingsWindow } from "./Settings.jsx";
import { scope } from "./data.js";
import { catalogs, formatCost, formatTokens } from "./i18n.js";
import { StageControls, useStagePrefs } from "./Stage.jsx";

// 菜单栏图标：左键开关 popover，右键或双击弹出原生风格菜单。
// 设置 / 检查更新 / 关于 / 退出 都在这个菜单里，所以 popover 底部不必再堆入口。
function MenuBarMenu({ lang, prefs, onPrefs, onSettings, onClose }) {
  const dict = catalogs[lang];
  const ref = useRef(null);
  const [submenu, setSubmenu] = useState(false);

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

  const values = [
    ["cost", dict.settings.valueCost],
    ["tokens", dict.settings.valueTokens],
    ["icon", dict.settings.valueIcon],
  ];

  return (
    <div className="menubar-menu" ref={ref} role="menu">
      <div className="submenu-anchor" onMouseEnter={() => setSubmenu(true)} onMouseLeave={() => setSubmenu(false)}>
        <button type="button" role="menuitem" className="has-submenu" aria-expanded={submenu}>
          {dict.settings.menubarValue}
          <CaretRight size={12} />
        </button>
        {submenu && (
          <div className="submenu" role="menu">
            {values.map(([key, label]) => (
              <button
                type="button"
                role="menuitemradio"
                aria-checked={prefs.menubarValue === key}
                key={key}
                onClick={() => {
                  onPrefs({ ...prefs, menubarValue: key });
                  onClose();
                }}
              >
                {prefs.menubarValue === key ? <Check size={12} weight="bold" /> : <i />}
                {label}
              </button>
            ))}
          </div>
        )}
      </div>
      <hr />
      <button type="button" role="menuitem" className="menu-settings" onClick={onSettings}>
        {dict.menu.settings}
        <small>⌘,</small>
      </button>
      <button type="button" role="menuitem">
        {dict.menu.about}
      </button>
      <hr />
      <button type="button" role="menuitem">
        {dict.menu.quit}
        <small>⌘Q</small>
      </button>
    </div>
  );
}

export function App() {
  const stage = useStagePrefs();
  const { lang, theme, state } = stage;
  const dict = catalogs[lang];
  const params = new URLSearchParams(window.location.search);
  const [open, setOpen] = useState(true);
  const [menu, setMenu] = useState(false);
  const [settings, setSettings] = useState(params.get("settings") === "1");
  const [prefs, setPrefs] = useState(DEFAULT_PREFS);
  const [client, setClient] = useState("all");

  useEffect(() => {
    const onKey = (event) => {
      if ((event.metaKey || event.ctrlKey) && event.key === ",") {
        event.preventDefault();
        setSettings(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const menubarClient = prefs.menubarScope === "follow" ? client : "all";
  const view = scope(menubarClient, "today");
  const incomplete = !view.pricingComplete;
  const value =
    state === "unavailable"
      ? "—"
      : prefs.menubarValue === "tokens"
        ? formatTokens(state === "empty" ? 0 : view.totals.tokens)
        : `${incomplete ? "≈" : ""}${formatCost(state === "empty" ? 0 : view.totals.cost, lang)}`;

  return (
    <main className="stage" data-theme={theme}>
      <StageControls prefs={stage} />
      <div className="stage-body">
        <div className="menubar-strip">
          <div className="menubar-item-wrap">
            <button
              type="button"
              className={`menubar-item${open ? " open" : ""}`}
              aria-expanded={open}
              aria-label={dict.app}
              onClick={() => setOpen((current) => !current)}
              onDoubleClick={() => setMenu(true)}
              onContextMenu={(event) => {
                event.preventDefault();
                setMenu(true);
              }}
            >
              <img src="/agentdeck-robot.png" alt="" width={17} height={17} />
              {prefs.menubarValue !== "icon" && <strong>{value}</strong>}
            </button>
            {menu && (
              <MenuBarMenu
                lang={lang}
                prefs={prefs}
                onPrefs={setPrefs}
                onSettings={() => {
                  setMenu(false);
                  setSettings(true);
                }}
                onClose={() => setMenu(false)}
              />
            )}
          </div>
          <p className="hint">{dict.menu.hint}</p>
        </div>
        {open && <Popover lang={lang} state={state} onClientChange={setClient} />}
      </div>
      {settings && (
        <div className="window-backdrop" onMouseDown={() => setSettings(false)}>
          <div onMouseDown={(event) => event.stopPropagation()}>
            <SettingsWindow lang={lang} prefs={prefs} onChange={setPrefs} onClose={() => setSettings(false)} />
          </div>
        </div>
      )}
    </main>
  );
}
