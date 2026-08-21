import { useEffect, useRef } from "react";
import { WarningCircle, X } from "@phosphor-icons/react";
import { catalogs } from "./i18n.js";

// 设置是独立窗口，不是 popover 里的一页——macOS 上 ⌘, 打开的东西从来不长在弹出面板里。
// 这里的每一项都对应实现里真实存在的偏好键，没有凭空加开关。

// 解释文字要成为控件的 accessible description，就得有一个稳定、与语言无关的 ID：
// 屏幕阅读器读的是 aria-describedby 指向的节点，不是视觉上排在下面的那一行。
const hintId = (key) => `settings-${key}-hint`;

function Switch({ checked, onChange, label, describedBy, disabled }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      aria-describedby={describedBy}
      disabled={disabled}
      className={`switch${checked ? " on" : ""}`}
      onClick={() => onChange(!checked)}
    >
      <i />
    </button>
  );
}

function Segmented({ value, options, onChange, label, describedBy }) {
  return (
    <div className="settings-segmented" role="radiogroup" aria-label={label} aria-describedby={describedBy}>
      {options.map(([key, text]) => (
        <button
          type="button"
          role="radio"
          aria-checked={value === key}
          key={key}
          className={value === key ? "active" : ""}
          onClick={() => onChange(key)}
        >
          {text}
        </button>
      ))}
    </div>
  );
}

function Field({ id, label, hint, children, error }) {
  return (
    <div className="settings-field">
      <div className="settings-label">
        <span>{label}</span>
        {hint && <small id={hintId(id)}>{hint}</small>}
        {/* 这个容器一直在 DOM 里，空的时候不占高度也不加间距。live region 必须先
            存在、随后才被填入内容，才会被宣布；跟着错误一起插进来的 region 通常
            不会。空 region 只有这一个，所以一次失败只宣布一次。 */}
        <div className="settings-error" role="status" aria-live="polite">
          {error && (
            <small className="tone-text-bad">
              <WarningCircle size={11} weight="fill" aria-hidden="true" /> {error}
            </small>
          )}
        </div>
      </div>
      <div className="settings-control">{children}</div>
    </div>
  );
}

export function SettingsWindow({ lang, prefs, onChange, onClose, embedded = false }) {
  const dict = catalogs[lang];
  const windowRef = useRef(null);
  const loginRefusedOnce = useRef(false);

  useEffect(() => {
    // 焦点给窗口本身而不是关闭按钮：Esc 照样能关，也不会在红绿灯上留一圈焦点环
    if (!embedded) windowRef.current?.focus();
  }, [embedded]);

  useEffect(() => {
    const onKey = (event) => {
      if (event.key === "Escape" && !embedded) onClose?.();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, embedded]);

  const set = (key) => (value) => onChange({ ...prefs, [key]: value });

  // SMAppService 真的会拒绝注册，而失败行的两条契约——"出现时被宣布"和"在下一次
  // 成功修改时清除，而不是定时消失"——只有真走一遍开关才验得到。第一次开启被拒，
  // 再开一次成功，正好把两条都走完。拒绝时只存一个布尔，文案在渲染时取，
  // 这样切换语言后失败行跟着换语言而不是留在旧语言上。
  const toggleLaunchAtLogin = (value) => {
    if (value && !loginRefusedOnce.current) {
      loginRefusedOnce.current = true;
      onChange({ ...prefs, launchAtLogin: false, loginItemRefused: true });
      return;
    }
    onChange({ ...prefs, launchAtLogin: value, loginItemRefused: false });
  };

  return (
    <section
      className={`settings-window${embedded ? " embedded" : ""}`}
      role="dialog"
      aria-label={dict.settings.title}
      ref={windowRef}
      tabIndex={-1}
    >
      <header>
        {!embedded && (
          <div className="traffic-lights">
            <button type="button" className="light close" onClick={onClose} aria-label={dict.settings.close}>
              <X size={8} weight="bold" />
            </button>
            <i className="light" aria-hidden="true" />
            <i className="light" aria-hidden="true" />
          </div>
        )}
        <strong>{dict.settings.title}</strong>
      </header>

      <div className="settings-body">
        <div className="settings-group">
          <span className="settings-group-title">{dict.settings.general}</span>
          <Field
            id="launchAtLogin"
            label={dict.settings.launchAtLogin}
            hint={dict.settings.launchAtLoginHint}
            error={prefs.loginItemRefused ? dict.settings.loginItemRefused : null}
          >
            <Switch
              checked={prefs.launchAtLogin}
              onChange={toggleLaunchAtLogin}
              label={dict.settings.launchAtLogin}
              describedBy={hintId("launchAtLogin")}
            />
          </Field>
          <Field id="periodicRefresh" label={dict.settings.periodicRefresh} hint={dict.settings.periodicRefreshHint}>
            <Switch
              checked={prefs.periodicRefresh}
              onChange={set("periodicRefresh")}
              label={dict.settings.periodicRefresh}
              describedBy={hintId("periodicRefresh")}
            />
          </Field>
        </div>

        <div className="settings-group">
          <span className="settings-group-title">{dict.settings.menubar}</span>
          <Field id="menubarValue" label={dict.settings.menubarValue} hint={dict.settings.menubarValueHint}>
            <Segmented
              value={prefs.menubarValue}
              label={dict.settings.menubarValue}
              describedBy={hintId("menubarValue")}
              onChange={set("menubarValue")}
              options={[
                ["cost", dict.settings.valueCost],
                ["tokens", dict.settings.valueTokens],
                ["icon", dict.settings.valueIcon],
              ]}
            />
          </Field>
          <Field id="menubarScope" label={dict.settings.menubarScope} hint={dict.settings.menubarScopeHint}>
            <Segmented
              value={prefs.menubarScope}
              label={dict.settings.menubarScope}
              describedBy={hintId("menubarScope")}
              onChange={set("menubarScope")}
              options={[
                ["all", dict.settings.scopeAll],
                ["follow", dict.settings.scopeFollow],
              ]}
            />
          </Field>
        </div>

      </div>
    </section>
  );
}

export const DEFAULT_PREFS = {
  launchAtLogin: false,
  periodicRefresh: false,
  menubarValue: "cost",
  menubarScope: "all",
  loginItemRefused: false,
};
