import { useState, useSyncExternalStore } from "react";

export const TELEMETRY_SCENARIOS = ["unknown", "unconfigured", "waiting", "ready", "capture_only", "disconnected", "paused"];
// Codex facts and write outcomes are simulated by the stage. The prototype
// never reads or writes a real Codex configuration.
export const CODEX_STAGE = {
  memories: ["on", "off", "unknown"],
  exporter: ["absent", "other", "managed", "agentdeck"],
  write: ["written", "refused"],
  restore: ["removed", "incomplete"],
};
const stageParams = new URLSearchParams(typeof window === "undefined" ? "" : window.location.search);
let codexStage = {
  memories: CODEX_STAGE.memories.includes(stageParams.get("codexMemories")) ? stageParams.get("codexMemories") : "unknown",
  exporter: CODEX_STAGE.exporter.includes(stageParams.get("codexExporter")) ? stageParams.get("codexExporter") : "absent",
  write: CODEX_STAGE.write.includes(stageParams.get("codexWrite")) ? stageParams.get("codexWrite") : "written",
  restore: CODEX_STAGE.restore.includes(stageParams.get("codexRestore")) ? stageParams.get("codexRestore") : "removed",
};
const listeners = new Set();
const subscribe = listener => { listeners.add(listener); return () => listeners.delete(listener); };
const snapshot = () => codexStage;
function setCodexStage(key, value) {
  codexStage = { ...codexStage, [key]: value };
  listeners.forEach(listener => listener());
}

const text = {
  zh: {
    group: "实时用量", label: "启用 OTel 用量采集", hint: "默认关闭。接收 Codex 的实时用量；不需要设置刷新周期。关闭后保留已采集数据。",
    off: "采集已关闭", unknown: "接收端已就绪 · 等待首次事件，连接状态未知", unknownHint: "尚无可验证的配置或接收观察。闲置没有事件不表示未配置或断连。", waitingHint: "最近已收到有效事件。HTTP 请求独立，不代表持续连接。", unconfigured: "接收端已启用 · 配置检查未发现有效 exporter", waiting: "已收到有效事件 · 等待新用量", ready: "正在接收 · 用量可计入", capture_only: "正在接收 · 用量尚未计入", disconnected: "已确认接收异常 · 保留已有数据", paused: "App 已退出 · 接收已暂停",
    setup: "首次启用需要检查 Codex 的遥测配置。开关只在你同意后才写入 Codex 配置。",
    capture: "请求身份尚未验证，观测先保存；不直接加入总量。", pausedHint: "默认随 App 起停。独立后台模式需要另外启用。",
    preview: "查看连接配置", hide: "收起配置", notice: "原型配置示例，未应用到真实 Codex。令牌已隐藏；请审阅并合并已有配置。",
    privacy: "仅接收用量、模型和必要身份字段，不保存提示、回答正文或工具输出。",
    stage: "遥测连接标本", demo: "连接状态由舞台模拟，开关本身不代表数据已经收齐。",
    codexStage: "Codex 标本", codexDemo: "Codex 记忆、导出配置与写入结果由舞台模拟；原型不读写真实 Codex 配置。",
    memories: "Codex 记忆", exporter: "导出配置", write: "写入结果", restore: "移除结果",
    memoriesOff: "Codex 记忆未开启 · 无需采集", memoriesOffHint: "其他 Codex 用量已由本地日志统计。开启 Codex 记忆后可在此启用采集。",
    memoriesStopped: "Codex 记忆已关闭 · 接收已停止", memoriesStoppedHint: "关闭采集可移除 Codex 配置中的 AgentDeck 段；保持开启时，Codex 记忆重新开启后恢复接收。",
    userConfigKept: "接收端已关闭 · 你自行添加的 Codex OTel 配置未改动",
    notCounted: "Codex 记忆的后台用量未统计", notCountedHint: "开启采集并允许写入 Codex 配置后才能统计。",
    consentTitle: "开启采集需要修改 Codex 配置", consentFile: "文件：~/.codex/config.toml",
    consentBody: "只追加这一段，不改动其他内容；关闭采集时移除这一段。重启 Codex 后生效。",
    writeAndOn: "写入并开启", selfConfigure: "我自己配置", cancel: "取消",
    conflict: "Codex 已有其他 OTel 配置，AgentDeck 不会覆盖。请按连接配置自行合并。", receiverOnly: "仅开启接收端",
    written: "已写入 Codex 配置 · 重启 Codex 后生效", refused: "无法写入 Codex 配置，未做任何修改。可重试或选择自己配置。",
    selfConfigured: "连接配置已展开，请自行合并到 Codex 配置",
    removed: "已从 Codex 配置移除 AgentDeck 段 · 重启 Codex 后停止导出",
    incomplete: "Codex 配置已被修改，AgentDeck 未改动它。请手动检查 ~/.codex/config.toml 的 [otel] 段。",
  },
  en: {
    group: "Live usage", label: "Enable OTel usage collection", hint: "Off by default. Receive live Codex usage without a refresh interval. Collected data is kept when disabled.",
    off: "Collection is off", unknown: "Receiver ready · Waiting for first event; connection unknown", unknownHint: "No verified configuration or reception observation yet. Silence does not mean unconfigured or disconnected.", waitingHint: "A valid event was received recently. HTTP requests do not establish a continuous connection.", unconfigured: "Receiver enabled · Configuration check found no valid exporter", waiting: "Valid event received · Waiting for new usage", ready: "Receiving · Usage can be counted", capture_only: "Receiving · Usage is not counted yet", disconnected: "Confirmed reception failure · Previous data is retained", paused: "App exited · Receiver paused",
    setup: "First-time setup requires checking Codex telemetry configuration. The switch changes Codex configuration only after you agree.",
    capture: "Request identity is not verified. Observations are saved without adding them to totals.", pausedHint: "The receiver follows the App lifetime. Independent background mode is enabled separately.",
    preview: "View connection configuration", hide: "Hide configuration", notice: "Prototype configuration example; not applied to Codex. Token is redacted. Review and merge with existing configuration.",
    privacy: "Only usage, model and required identity fields are retained. Prompts, response text and tool output are not stored.",
    stage: "Telemetry connection specimens", demo: "Connection state is simulated by the stage. Enabling the switch does not prove complete collection.",
    codexStage: "Codex specimens", codexDemo: "Codex Memories, export configuration and write outcomes are simulated by the stage; the prototype never reads or writes a real Codex configuration.",
    memories: "Codex Memories", exporter: "Export configuration", write: "Write outcome", restore: "Removal outcome",
    memoriesOff: "Codex Memories is off · Collection is not needed", memoriesOffHint: "Other Codex usage is already counted from local logs. Turn on Codex Memories to enable collection here.",
    memoriesStopped: "Codex Memories turned off · Receiver stopped", memoriesStoppedHint: "Turn off collection to remove AgentDeck's block from the Codex configuration. If you leave it on, receiving resumes when Codex Memories is turned back on.",
    userConfigKept: "Receiver turned off · Your own Codex OTel configuration was not changed",
    notCounted: "Codex Memories background usage is not counted", notCountedHint: "Turn on collection and allow the Codex configuration change to count it.",
    consentTitle: "Collection needs a Codex configuration change", consentFile: "File: ~/.codex/config.toml",
    consentBody: "Only this block is appended; nothing else changes. It is removed when collection is turned off. Takes effect after Codex restarts.",
    writeAndOn: "Write and turn on", selfConfigure: "I'll configure it", cancel: "Cancel",
    conflict: "Codex already has another OTel configuration. AgentDeck will not overwrite it. Merge the connection configuration yourself.", receiverOnly: "Turn on receiver only",
    written: "Codex configuration updated · Takes effect after Codex restarts", refused: "Could not update the Codex configuration; nothing was changed. Retry or configure it yourself.",
    selfConfigured: "Connection configuration is shown below; merge it into your Codex configuration",
    removed: "Removed AgentDeck's block from the Codex configuration · Codex stops exporting after restart",
    incomplete: "The Codex configuration was changed elsewhere; AgentDeck left it untouched. Check the [otel] section of ~/.codex/config.toml.",
  },
};

const managedBlock = '# BEGIN AgentDeck OTel (managed)\n[otel]\nlog_user_prompt = false\nexporter = { otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "json", headers = { "X-AgentDeck-Token" = "<local-token>" } } }\n# END AgentDeck OTel (managed)';

export function TelemetryStageControls({ lang, scenario, onScenario }) {
  const t = text[lang];
  const stage = useSyncExternalStore(subscribe, snapshot);
  return <>
    <section className="scan-stage-controls" aria-label={t.stage}>
      <label>{t.stage}<select data-telemetry-scenario value={scenario} onChange={event => onScenario(event.target.value)}>
        {TELEMETRY_SCENARIOS.map(key => <option key={key} value={key}>{t[key]}</option>)}
      </select></label><small>{t.demo}</small>
    </section>
    <section className="scan-stage-controls" aria-label={t.codexStage}>
      {Object.entries(CODEX_STAGE).map(([key, values]) => <label key={key}>{t[key]}<select data-codex-stage={key} value={stage[key]} onChange={event => setCodexStage(key, event.target.value)}>
        {values.map(value => <option key={value} value={value}>{value}</option>)}
      </select></label>)}
      <small>{t.codexDemo}</small>
    </section>
  </>;
}

// result: null | {kind, tone} where tone is info | bad | warn. The live
// region exists before any result so a failure is announced, not inserted.
export function TelemetrySettings({ lang, enabled, onEnabled, scenario = "unknown", hidden = false }) {
  const t = text[lang];
  const stage = useSyncExternalStore(subscribe, snapshot);
  const [preview, setPreview] = useState(false);
  const [consent, setConsent] = useState(false);
  const [result, setResult] = useState(null);
  const [managed, setManaged] = useState(stage.exporter === "managed");
  const memoriesOff = stage.memories === "off";
  const running = enabled && !memoriesOff;
  const status = memoriesOff ? (enabled ? "memoriesStopped" : "memoriesOff") : enabled ? (TELEMETRY_SCENARIOS.includes(scenario) ? scenario : "unknown") : "off";
  const config = '[otel]\nexporter = { otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "json", headers = { "X-AgentDeck-Token" = "<local-token>" } } }\nlog_user_prompt = false';
  // managed is AgentDeck's own block; agentdeck is the user's own exporter
  // pointing at this receiver. Both enable directly; only managed is removed.
  const ownsBlock = managed || stage.exporter === "managed";
  const exporterConfigured = ownsBlock || stage.exporter === "agentdeck";

  const toggle = () => {
    // With Memories off the switch can still turn collection off, never on.
    if (memoriesOff && !enabled) return;
    if (enabled) {
      setConsent(false);
      onEnabled(false);
      if (ownsBlock) {
        setResult(stage.restore === "removed" ? { kind: "removed", tone: "info" } : { kind: "incomplete", tone: "warn" });
        setManaged(false);
      } else if (stage.exporter === "agentdeck") setResult({ kind: "userConfigKept", tone: "info" });
      else setResult(null);
      return;
    }
    if (exporterConfigured) { setResult(null); onEnabled(true); return; }
    setResult(null);
    setConsent(true);
  };
  const write = () => {
    setConsent(false);
    if (stage.write === "refused") { setResult({ kind: "refused", tone: "bad" }); return; }
    setManaged(true);
    setResult({ kind: "written", tone: "info" });
    onEnabled(true);
  };
  const selfConfigure = () => { setConsent(false); setPreview(true); setResult({ kind: "selfConfigured", tone: "info" }); onEnabled(true); };
  const receiverOnly = () => { setConsent(false); setPreview(true); setResult(null); onEnabled(true); };
  const rowResult = result && result.tone !== "info";

  return <div className="settings-group telemetry-settings" data-group="telemetry" data-codex-memories={stage.memories} hidden={hidden}>
    <span className="settings-group-title">{t.group}</span>
    <div className={`settings-field${rowResult ? " settings-error" : ""}`}>
      <div className="settings-label"><span>{t.label}</span><small id="settings-otel-hint">{t.hint}</small>
        <small className={rowResult ? `tone-text-${result.tone}` : undefined} role="status" aria-live="polite" aria-atomic="true" data-otel-row-result={rowResult ? result.kind : ""}>
          {rowResult && <><span aria-hidden="true">{result.tone === "bad" ? "⊘ " : "⚠ "}</span>{t[result.kind]}</>}
        </small>
      </div>
      <div className="settings-control"><button type="button" role="switch" aria-checked={enabled} aria-disabled={memoriesOff && !enabled} disabled={memoriesOff && !enabled} aria-label={t.label} aria-describedby="settings-otel-hint settings-otel-privacy" className={`switch${enabled ? " on" : ""}`} data-otel-switch onClick={toggle}><i /></button></div>
    </div>
    <div className="telemetry-details" data-otel-status={status}>
      <p role="status" aria-live="polite" aria-atomic="true">{t[status]}</p>
      {status === "memoriesOff" && <p>{t.memoriesOffHint}</p>}
      {status === "memoriesStopped" && <p>{t.memoriesStoppedHint}</p>}
      {stage.memories === "on" && !enabled && <p className="tone-text-warn" data-otel-not-counted><span aria-hidden="true">⚠ </span>{t.notCounted}<br />{t.notCountedHint}</p>}
      {consent && stage.exporter !== "other" && <div className="telemetry-consent" data-otel-consent="write" role="group" aria-label={t.consentTitle}>
        <p><strong>{t.consentTitle}</strong></p><p>{t.consentFile}</p><pre>{managedBlock}</pre><p>{t.consentBody}</p>
        <div className="telemetry-consent-actions"><button type="button" data-otel-write onClick={write}>{t.writeAndOn}</button><button type="button" data-otel-self onClick={selfConfigure}>{t.selfConfigure}</button><button type="button" data-otel-cancel onClick={() => setConsent(false)}>{t.cancel}</button></div>
      </div>}
      {consent && stage.exporter === "other" && <div className="telemetry-consent" data-otel-consent="conflict" role="group" aria-label={t.conflict}>
        <p>{t.conflict}</p>
        <div className="telemetry-consent-actions"><button type="button" data-otel-receiver-only onClick={receiverOnly}>{t.receiverOnly}</button><button type="button" data-otel-cancel onClick={() => setConsent(false)}>{t.cancel}</button></div>
      </div>}
      {result && result.tone === "info" && <p data-otel-result={result.kind}>{t[result.kind]}</p>}
      {running && status === "unknown" && <p>{t.unknownHint}</p>}
      {running && status === "waiting" && <p>{t.waitingHint}</p>}
      {running && status === "unconfigured" && <p>{t.setup}</p>}
      {running && status === "capture_only" && <p>{t.capture}</p>}
      {running && status === "paused" && <p>{t.pausedHint}</p>}
      <p id="settings-otel-privacy">{t.privacy}</p>
      <button type="button" className="telemetry-preview-button" aria-expanded={preview} onClick={() => setPreview(!preview)}>{preview ? t.hide : t.preview}</button>
      {preview && <div data-otel-preview><p>{t.notice}</p><pre>{config}</pre></div>}
    </div>
  </div>;
}
