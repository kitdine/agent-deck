import { useState } from "react";
import { CheckCircle, SpinnerGap, WarningCircle } from "@phosphor-icons/react";

// Design-only synthetic data. No real engine, telemetry, cache or native timings.
export const SERVING_STATES = ["memory", "disk", "rebuilding", "partial", "mixed", "first", "failed", "midnight", "invalid"];
const copy = {
  zh: {
    memory: ["数据已准备好", "当前显示已保存的数据"],
    disk: ["已恢复上次数据", "后台正在检查更新，不影响查看"],
    rebuilding: ["后台正在更新", "当前显示上次数据，日期和范围保持不变"],
    partial: ["正在补齐数据", "数值仅包含已采集部分，尚不完整"],
    mixed: ["今天用量已准备，其他数据仍在补齐", "今天可读；会话和项目未知，7天未准备，30天仅含已采集部分"],
    first: ["正在准备数据", "尚无可用数据，准备好后会自动显示"],
    failed: ["更新未完成", "当前显示上次数据，可继续查看或重试"],
    midnight: ["今天的数据尚未准备好", "今天显示 —；其他时段仍可查看已保存数据"],
    invalid: ["暂时无法读取数据", "已保存数据未通过检查，重新准备后才能显示"],
    retry: "重试", stage: "数据准备标本", publish: "发布准备好的数据",
  },
  en: {
    memory: ["Data is ready", "Showing saved data"],
    disk: ["Previous data restored", "Checking for updates in the background"],
    rebuilding: ["Updating in the background", "Previous data keeps its original date and range"],
    partial: ["Preparing more data", "Values contain collected data only and are incomplete"],
    mixed: ["Today usage is ready; other data is still preparing", "Today is readable; sessions and projects are unknown, 7 days unavailable, 30 days partially collected"],
    first: ["Preparing your data", "No data is available yet. It will appear when ready"],
    failed: ["Update incomplete", "Previous data remains available. You can retry"],
    midnight: ["Today's data is not ready", "Today shows —. Saved data for other periods remains available"],
    invalid: ["Data cannot be read yet", "Saved data did not pass validation. Preparing a new view"],
    retry: "Retry", stage: "Data readiness specimens", publish: "Publish prepared data",
  },
};

export function useServingScenario() {
  const requested = new URLSearchParams(window.location.search).get("serving");
  const enabled = SERVING_STATES.includes(requested);
  const [phase, setPhase] = useState(enabled ? requested : "memory");
  const [publication, setPublication] = useState(1);
  const hasSnapshot = !["first", "invalid"].includes(phase);
  const active = ["disk", "rebuilding", "partial", "mixed", "first", "midnight", "invalid"].includes(phase);
  return { enabled, phase, hasSnapshot, active, publication,
    ageMinutes: ["disk", "rebuilding", "failed", "midnight"].includes(phase) ? 15 : 0,
    select: setPhase, requestRefresh: () => setPhase(hasSnapshot ? "rebuilding" : "first"),
    publish: () => { setPublication((n) => n + 1); setPhase("memory"); },
  };
}

export function ServingStageControls({ serving, lang }) {
  const t = copy[lang];
  return <section className="scan-stage-controls" aria-label={t.stage}>
    <label>{t.stage}<select data-serving-state value={serving.phase} onChange={(event) => serving.select(event.target.value)}>
      {SERVING_STATES.map((key) => <option key={key} value={key}>{t[key][0]}</option>)}
    </select></label>
    <button type="button" data-serving-publish onClick={serving.publish}>{t.publish}</button>
    <small>Publication {serving.publication} · {lang === "zh" ? "模拟数据，不是性能测量" : "Synthetic data, not a performance measurement"}</small>
  </section>;
}

export function ServingStatus({ serving, lang, period = "today" }) {
  const t = copy[lang];
  const failed = ["failed", "invalid"].includes(serving.phase);
  const Icon = failed ? WarningCircle : serving.active ? SpinnerGap : CheckCircle;
  return <section className={`serving-status${failed ? " serving-error" : ""}`} data-serving-status data-phase={serving.phase} data-publication={serving.publication}>
    <div role="status" aria-live="polite" aria-atomic="true"><Icon size={16} aria-hidden="true" /><strong>{t[serving.phase][0]}</strong></div>
    <p>{t[serving.phase][1]}</p>
    {serving.phase === "midnight" && period !== "today" && <p>{lang === "zh" ? "此时段显示已保存数据，保持原范围" : "This period shows saved data with its original range"}</p>}
    {failed && <button type="button" data-serving-retry onClick={serving.requestRefresh}>{t.retry}</button>}
  </section>;
}

export function ServingEmptyPopover({ serving, lang, width }) {
  return <section className="popover scan-empty" data-testid="menubar-popover" data-serving-snapshot="none" style={{ "--popover-w": `${width}px` }} data-width={width} aria-label="AgentDeck">
    <header><div className="brand"><img src="/agentdeck-ad.png" alt="" width={22} height={22} /><strong>AgentDeck</strong></div></header>
    <div className="scan-loading-body"><ServingStatus serving={serving} lang={lang} /></div>
  </section>;
}

const engineSamples = {
  app: { command: "engine status", exit: 0,
    text: "Engine: running\nMode: App-attached\nIndependent mode: disabled\nData: ready · publication 12\nBackground: rebuilding\nTelemetry: disabled\nApp exit: App-owned work stops",
    data: { state: "running", mode: "app", independent: false, serving: { publication_id: "12", state: "ready" }, jobs: { state: "rebuilding" }, telemetry: { state: "disabled" } } },
  absent: { command: "engine status", exit: 0,
    text: "Engine: stopped\nIndependent mode: disabled\nData: saved snapshot available\nSynchronous commands start a finite worker.\nusage stats --no-scan reads stored data.",
    data: { state: "stopped", mode: "none", independent: false, serving: { state: "saved" }, telemetry: { state: "disabled" } } },
  independent: { command: "engine start", exit: 0,
    text: "Independent mode enabled.\nThe engine continues after the App exits.\nRun agentdeck engine stop to disable it.\nNo login item or system service was installed.",
    data: { state: "running", mode: "independent", independent: true, autostart: false } },
  stopped: { command: "engine stop", exit: 0,
    text: "Independent mode disabled.\nThe App-attached engine remains active.\nExisting finite CLI tasks keep their accepted scope.",
    data: { state: "running", mode: "app", independent: false } },
  mismatch: { command: "scan", exit: 1, text: "", error: "engine_protocol_mismatch",
    stderr: "engine_protocol_mismatch: running engine is incompatible.\nStored-only reads remain available.\nRestart at a safe checkpoint; no process was terminated." },
  telemetry: { command: "telemetry status", exit: 0,
    text: "Receiver: enabled\nListener: running (App-attached)\nMetering: capture_only\nReason: exact request identity not verified\nPending observations: 3\nHistorical coverage: unknown\nNo pending tokens were added to totals.",
    data: { receiver: "enabled", listener: "running", engine_mode: "app", metering: "capture_only", reason_code: "identity_unverified", pending_observations: 3, historical_coverage: "unknown" } },
  stored_partial: { command: "usage summary daily --no-scan", commandId: "usage.summary", exit: 0,
    text: "Period: today\nTokens: 133\nEvents: 1\nKnown catalog base cost: $0.01",
    stderr: "source_coverage_partial: historical import is incomplete; showing committed data.",
    warnings: ["source_coverage_partial"], partial: true,
    data: { tokens: { input_tokens: 111, output_tokens: 22 }, counts: { events: 1 }, attribution_reasons: {},
      catalog_base_cost: "0.01", provider_cost: null, known_catalog_base_cost: "0.01", known_provider_cost: "0",
      unattributed_catalog_base_cost: "0", model_coverage: [], unpriced_components: [], warnings: [] } },
  telemetry_unknown: { command: "telemetry status", exit: 0,
    text: "Receiver: enabled\nListener: running (App-attached)\nProducer configuration: unknown\nReception: waiting for first verified event\nConnection: unknown (independent HTTP requests)\nMetering: capture_only\nHistorical coverage: unknown",
    data: { receiver: "enabled", listener: "running", producer_configured: "unknown", last_received_at: null,
      reception: "unknown", metering: "capture_only", historical_coverage: "unknown" } },
  preview: { command: "telemetry config-preview", exit: 0,
    text: "# Proposed configuration; Codex config was not changed.\n# Merge with your user-level configuration after review.\n[otel]\nexporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"json\", headers = { \"X-AgentDeck-Token\" = \"<local-token>\" } } }\nlog_user_prompt = false\n# Local token is redacted by default.\n# Endpoint above is illustrative.",
    data: { applied: false, token_redacted: true, transport: "otlp-http-json", endpoint: "http://127.0.0.1:4318/v1/logs" } },
};

export function EngineCli({ stage }) {
  const requested = new URLSearchParams(window.location.search).get("engine");
  const [columns, setColumns] = useState(new URLSearchParams(window.location.search).get("cols") === "40" ? 40 : 80);
  const [scenario, setScenario] = useState(engineSamples[requested] ? requested : "app");
  const sample = engineSamples[scenario];
  const envelope = {
    schema_version: 1, command: sample.commandId ?? sample.command.split(" ").slice(0, 2).join("."),
    generated_at: "2026-10-09T12:00:00Z", data: sample.error ? null : sample.data,
    warnings: sample.warnings ?? [], partial: sample.partial ?? false,
    ...(sample.error ? { error: { code: sample.error, message: "Running engine is incompatible" } } : {}),
  };
  return <>
    <label className="scan-cli-options">{stage.lang === "zh" ? "引擎标本" : "Engine specimen"}<select data-engine-scenario value={scenario} onChange={(event) => setScenario(event.target.value)}>{Object.keys(engineSamples).map((key) => <option key={key}>{key}</option>)}</select></label>
    <label className="scan-cli-options">Columns<select data-engine-cols value={columns} onChange={(event) => setColumns(Number(event.target.value))}><option>40</option><option>80</option></select></label>
    <p className="cli-note">{stage.lang === "zh" ? "拟新增命令的逐字符设计标本；现有 scan / --no-scan 契约不改变。所有数据为合成值。" : "Character specimens for proposed commands. Existing scan / --no-scan contracts remain. All data is synthetic."}</p>
    <div className="terminal engine-terminal" data-engine-cli={scenario} data-engine-columns={columns} style={{maxWidth: `calc(${columns}ch + 34px)`}}><div className="terminal-bar"><span>agentdeck {sample.command}</span></div><div className="scan-stream-label">stdout</div><pre data-engine-stdout>{sample.text || "\u00a0"}</pre><div className="scan-stream-label">stderr</div><pre data-engine-stderr>{sample.stderr || "\u00a0"}</pre></div>
    <div className="terminal engine-terminal" style={{maxWidth: `calc(${columns}ch + 34px)`}}><div className="terminal-bar"><span>--format json</span></div>
      <div className="scan-stream-label">stdout</div><pre data-engine-json-stdout data-engine-json={!sample.error ? scenario : undefined}>{sample.error ? "\u00a0" : JSON.stringify(envelope, null, 2)}</pre>
      <div className="scan-stream-label">stderr</div><pre data-engine-json-stderr data-engine-json={sample.error ? scenario : undefined}>{sample.error ? JSON.stringify(envelope, null, 2) : "\u00a0"}</pre>
    </div>
    <p className="cli-note" data-engine-exit>Exit {sample.exit}</p>
  </>;
}
