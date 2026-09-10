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

function Field({ id, label, hint, children, error, errorTone = "bad" }) {
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
            <small className={`tone-text-${errorTone}`}>
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
  // 状态栏通路是本产品唯一一个写别人文件的开关，所以它有两种失败，而且必须
  // 分开说：写入被拒是"没做成"——开关必须留在关闭，否则界面在替一个没发生的
  // 操作打包票；关闭时的恢复冲突是"做了一半"——AgentDeck 的命令确实移除了，
  // 但原值在此期间被改过，不能覆盖用户自己编辑的东西，所以只能报告并请人工检查。
  // 标本按 launch-at-login 已有的那条路子模拟：第一次拒绝、再来一次成功，
  // 这样"出现时被宣布"和"下一次成功修改时清除"两条契约都走得到。
  const statuslineRefusedOnce = useRef(false);
  const statuslineConflictOnce = useRef(false);
  const toggleQuotaStatusline = (value) => {
    if (value && !statuslineRefusedOnce.current) {
      statuslineRefusedOnce.current = true;
      onChange({
        ...prefs,
        quotaStatusline: false,
        quotaStatuslineWriteRefused: true,
        quotaStatuslineRestoreIncomplete: false,
      });
      return;
    }
    if (!value && prefs.quotaStatusline && !statuslineConflictOnce.current) {
      statuslineConflictOnce.current = true;
      onChange({
        ...prefs,
        quotaStatusline: false,
        quotaStatuslineWriteRefused: false,
        quotaStatuslineRestoreIncomplete: true,
      });
      return;
    }
    onChange({
      ...prefs,
      quotaStatusline: value,
      quotaStatuslineWriteRefused: false,
      quotaStatuslineRestoreIncomplete: false,
    });
  };

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
        <div className="settings-group" data-group="general">
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

        {/* 订阅额度。三个开关的层级是有依赖的，不是并列：
            不读取额度 → 状态栏通路与提醒都无从谈起；
            不开提醒 → 阈值不可编辑。禁用而不是隐藏，
            因为隐藏会让人以为产品没有这个能力。 */}
        <div className="settings-group" data-group="quota">
          <span className="settings-group-title">{dict.settings.quota}</span>
          <Field id="quotaProbe" label={dict.settings.quotaProbe} hint={dict.settings.quotaProbeHint}>
            <Switch
              checked={prefs.quotaProbe}
              onChange={set("quotaProbe")}
              label={dict.settings.quotaProbe}
              describedBy={hintId("quotaProbe")}
            />
          </Field>
          <Field id="quotaInterval" label={dict.settings.quotaInterval} hint={dict.settings.quotaIntervalHint}>
            <Segmented
              value={prefs.quotaInterval}
              label={dict.settings.quotaInterval}
              describedBy={hintId("quotaInterval")}
              onChange={set("quotaInterval")}
              options={[
                ["5m", "5m"],
                ["15m", "15m"],
                ["30m", "30m"],
              ]}
            />
          </Field>
          {/* 写入 ~/.claude/settings.json 需要显式同意，所以提示里必须写清
              「会串接哪一条既有命令」。同意一个看不见后果的动作不算同意。 */}
          <Field
            id="quotaStatusline"
            label={dict.settings.quotaStatusline}
            hint={
              prefs.existingStatusLine
                ? `${dict.settings.quotaStatuslineHint} · ${dict.settings.quotaStatuslineChained(prefs.existingStatusLine)}`
                : `${dict.settings.quotaStatuslineHint} · ${dict.settings.quotaStatuslineNone}`
            }
            error={
              prefs.quotaStatuslineWriteRefused
                ? dict.settings.quotaStatuslineWriteRefused
                : prefs.quotaStatuslineRestoreIncomplete
                  ? dict.settings.quotaStatuslineRestoreIncomplete
                  : null
            }
            errorTone={prefs.quotaStatuslineWriteRefused ? "bad" : "warn"}
          >
            <Switch
              checked={prefs.quotaStatusline}
              onChange={toggleQuotaStatusline}
              label={dict.settings.quotaStatusline}
              describedBy={hintId("quotaStatusline")}
              disabled={!prefs.quotaProbe}
            />
          </Field>
          <Field id="quotaAlerts" label={dict.settings.quotaAlerts} hint={dict.settings.quotaAlertsHint}>
            <Switch
              checked={prefs.quotaAlerts}
              onChange={set("quotaAlerts")}
              label={dict.settings.quotaAlerts}
              describedBy={hintId("quotaAlerts")}
              disabled={!prefs.quotaProbe}
            />
          </Field>
          <Field id="quotaThresholds" label={dict.settings.quotaThresholds} hint={null}>
            <Segmented
              value={prefs.quotaThresholds}
              label={dict.settings.quotaThresholds}
              onChange={set("quotaThresholds")}
              options={[
                ["75", "75%"],
                ["90", "90%"],
                ["75+90", "75% · 90%"],
              ]}
            />
          </Field>
          <Field id="quotaResetNotice" label={dict.settings.quotaResetNotice} hint={null}>
            <Switch
              checked={prefs.quotaResetNotice}
              onChange={set("quotaResetNotice")}
              label={dict.settings.quotaResetNotice}
              disabled={!prefs.quotaProbe || !prefs.quotaAlerts}
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
  // 三个额度开关默认全关：requirements.md 的 opt-in 契约写在这里，
  // 不写在文档的散文里——默认值是唯一会被真机读到的那一份。
  quotaProbe: false,
  quotaStatuslineWriteRefused: false,
  quotaStatuslineRestoreIncomplete: false,
  quotaInterval: "5m",
  quotaStatusline: false,
  quotaAlerts: false,
  quotaThresholds: "75+90",
  quotaResetNotice: false,
  // 用户已有的 statusLine 命令，用于在同意前把「会串接什么」说清楚。
  existingStatusLine: "python3 ~/.claude/statusline.py",
};
