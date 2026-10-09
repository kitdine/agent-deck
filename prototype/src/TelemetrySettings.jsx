import { useState } from "react";

export const TELEMETRY_SCENARIOS = ["unknown", "unconfigured", "waiting", "ready", "capture_only", "disconnected", "paused"];
const text = {
  zh: {
    group: "实时用量", label: "启用 OTel 用量采集", hint: "默认关闭。接收 Codex 的实时用量；不需要设置刷新周期。关闭后保留已采集数据。",
    off: "采集已关闭", unknown: "接收端已就绪 · 等待首次事件，连接状态未知", unknownHint: "尚无可验证的配置或接收观察。闲置没有事件不表示未配置或断连。", waitingHint: "最近已收到有效事件。HTTP 请求独立，不代表持续连接。", unconfigured: "接收端已启用 · 配置检查未发现有效 exporter", waiting: "已收到有效事件 · 等待新用量", ready: "正在接收 · 用量可计入", capture_only: "正在接收 · 用量尚未计入", disconnected: "已确认接收异常 · 保留已有数据", paused: "App 已退出 · 接收已暂停",
    setup: "首次启用需要检查 Codex 的遥测配置。此开关只启用 AgentDeck 接收端，不会自动覆盖 Codex 配置。",
    capture: "请求身份尚未验证，观测先保存；不直接加入总量。", pausedHint: "默认随 App 起停。独立后台模式需要另外启用。",
    preview: "查看连接配置", hide: "收起配置", notice: "原型配置示例，未应用到真实 Codex。令牌已隐藏；请审阅并合并已有配置。",
    privacy: "仅接收用量、模型和必要身份字段，不保存提示、回答正文或工具输出。",
    stage: "遥测连接标本", demo: "连接状态由舞台模拟，开关本身不代表数据已经收齐。",
  },
  en: {
    group: "Live usage", label: "Enable OTel usage collection", hint: "Off by default. Receive live Codex usage without a refresh interval. Collected data is kept when disabled.",
    off: "Collection is off", unknown: "Receiver ready · Waiting for first event; connection unknown", unknownHint: "No verified configuration or reception observation yet. Silence does not mean unconfigured or disconnected.", waitingHint: "A valid event was received recently. HTTP requests do not establish a continuous connection.", unconfigured: "Receiver enabled · Configuration check found no valid exporter", waiting: "Valid event received · Waiting for new usage", ready: "Receiving · Usage can be counted", capture_only: "Receiving · Usage is not counted yet", disconnected: "Confirmed reception failure · Previous data is retained", paused: "App exited · Receiver paused",
    setup: "First-time setup requires checking Codex telemetry configuration. This switch enables the AgentDeck receiver and does not overwrite Codex configuration.",
    capture: "Request identity is not verified. Observations are saved without adding them to totals.", pausedHint: "The receiver follows the App lifetime. Independent background mode is enabled separately.",
    preview: "View connection configuration", hide: "Hide configuration", notice: "Prototype configuration example; not applied to Codex. Token is redacted. Review and merge with existing configuration.",
    privacy: "Only usage, model and required identity fields are retained. Prompts, response text and tool output are not stored.",
    stage: "Telemetry connection specimens", demo: "Connection state is simulated by the stage. Enabling the switch does not prove complete collection.",
  },
};

export function TelemetryStageControls({ lang, scenario, onScenario }) {
  const t = text[lang];
  return <section className="scan-stage-controls" aria-label={t.stage}>
    <label>{t.stage}<select data-telemetry-scenario value={scenario} onChange={event => onScenario(event.target.value)}>
      {TELEMETRY_SCENARIOS.map(key => <option key={key} value={key}>{t[key]}</option>)}
    </select></label><small>{t.demo}</small>
  </section>;
}

export function TelemetrySettings({ lang, enabled, onEnabled, scenario = "unknown" }) {
  const t = text[lang];
  const [preview, setPreview] = useState(false);
  const status = enabled ? (TELEMETRY_SCENARIOS.includes(scenario) ? scenario : "unknown") : "off";
  const config = '[otel]\nexporter = { otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "json", headers = { "X-AgentDeck-Token" = "<local-token>" } } }\nlog_user_prompt = false';
  return <div className="settings-group telemetry-settings" data-group="telemetry">
    <span className="settings-group-title">{t.group}</span>
    <div className="settings-field">
      <div className="settings-label"><span>{t.label}</span><small id="settings-otel-hint">{t.hint}</small></div>
      <div className="settings-control"><button type="button" role="switch" aria-checked={enabled} aria-label={t.label} aria-describedby="settings-otel-hint settings-otel-privacy" className={`switch${enabled ? " on" : ""}`} data-otel-switch onClick={() => onEnabled(!enabled)}><i /></button></div>
    </div>
    <div className="telemetry-details" data-otel-status={status}>
      <p role="status" aria-live="polite" aria-atomic="true">{t[status]}</p>
      {enabled && status === "unknown" && <p>{t.unknownHint}</p>}
      {enabled && status === "waiting" && <p>{t.waitingHint}</p>}
      {enabled && status === "unconfigured" && <p>{t.setup}</p>}
      {enabled && status === "capture_only" && <p>{t.capture}</p>}
      {enabled && status === "paused" && <p>{t.pausedHint}</p>}
      <p id="settings-otel-privacy">{t.privacy}</p>
      <button type="button" className="telemetry-preview-button" aria-expanded={preview} onClick={() => setPreview(!preview)}>{preview ? t.hide : t.preview}</button>
      {preview && <div data-otel-preview><p>{t.notice}</p><pre>{config}</pre></div>}
    </div>
  </div>;
}
