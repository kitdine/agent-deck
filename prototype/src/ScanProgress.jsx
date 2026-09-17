import { useEffect, useState } from "react";
import { CheckCircle, SpinnerGap, WarningCircle } from "@phosphor-icons/react";

export const SCAN_PHASES = ["waiting", "checking", "importing", "usage-complete", "statistics", "completed", "partial", "failed"];
const sequence = SCAN_PHASES.slice(0, 6);
const copy = {
  en: { waiting: "Waiting for current scan", checking: "Checking source files", importing: "Importing", "usage-complete": "Finishing session import", statistics: "Calculating statistics", completed: "Refresh complete", partial: "Scan partly completed", failed: "Refresh failed", usage: "Usage", session: "Sessions", unknown: "File count is not known yet", retained: "Showing the previous snapshot", initial: "Your data will appear when the snapshot is ready", retry: "Retry", unavailable: "No snapshot available yet", sessionFailed: "Session scan failed", counts: "files committed", skipped: "skipped", complete: "Complete" },
  zh: { waiting: "等待当前扫描", checking: "检查源文件", importing: "正在导入", "usage-complete": "正在完成会话导入", statistics: "计算统计数据", completed: "刷新完成", partial: "扫描部分完成", failed: "刷新失败", usage: "用量", session: "会话", unknown: "文件总数尚未确定", retained: "当前显示上次快照", initial: "快照准备好后将在这里显示数据", retry: "重试", unavailable: "暂时没有可用快照", sessionFailed: "会话扫描失败", counts: "个文件已提交", skipped: "已跳过", complete: "已完成" },
};

// Specimen-only worker lives in App, outside the mounted subscriber.
// It never reads real source files or launches an actual worker.
export function useScanScenario() {
  const params = new URLSearchParams(window.location.search);
  const requested = params.get("scan");
  const enabled = SCAN_PHASES.includes(requested);
  const [phase, setPhase] = useState(enabled ? requested : "waiting");
  const [prior, setPrior] = useState(params.get("prior") !== "0");
  const [playing, setPlaying] = useState(false);
  const [round, setRound] = useState(1);
  const [published, setPublished] = useState(requested === "completed");
  useEffect(() => { if (phase === "completed") setPublished(true); }, [phase]);
  useEffect(() => {
    if (!playing) return;
    const i = sequence.indexOf(phase);
    if (i < 0 || i === 5) { setPlaying(false); return; }
    const timer = window.setTimeout(() => setPhase(sequence[i + 1]), 1300);
    return () => window.clearTimeout(timer);
  }, [phase, playing]);
  const select = (value) => { setPlaying(false); setPublished(value === "completed"); setRound((n) => n + 1); setPhase(value); };
  const restart = () => { if (published) setPrior(true); setPublished(false); setPhase("waiting"); setRound((n) => n + 1); setPlaying(true); };
  const next = () => { const i = sequence.indexOf(phase); if (i >= 0 && i < 5) setPhase(sequence[i + 1]); };
  return { enabled, phase, prior, playing, round, published, hasSnapshot: prior || published,
    active: !["completed", "partial", "failed"].includes(phase), select, restart, next,
    setPlaying, setPrior: (v) => { setPrior(v); setPublished(phase === "completed"); } };
}

export function ScanStageControls({ scan, lang, attached = true }) {
  const zh = lang === "zh";
  return <section className="scan-stage-controls" aria-label={zh ? "扫描原型舞台" : "Scan specimen controls"}>
    <label>{zh ? "扫描状态" : "Scan state"}<select data-scan-phase value={scan.phase} onChange={(e) => scan.select(e.target.value)}>{SCAN_PHASES.map((p) => <option key={p} value={p}>{copy[lang][p]}</option>)}</select></label>
    <label><input data-scan-prior type="checkbox" checked={scan.prior} onChange={(e) => scan.setPrior(e.target.checked)} />{zh ? "已有快照" : "Previous snapshot"}</label>
    <button type="button" data-scan-next onClick={scan.next} disabled={!sequence.includes(scan.phase) || scan.phase === "completed"}>{zh ? "下一阶段" : "Next stage"}</button>
    <button type="button" data-scan-play onClick={() => scan.playing ? scan.setPlaying(false) : scan.restart()}>{scan.playing ? (zh ? "暂停演示" : "Pause demo") : (zh ? "播放完整流程" : "Play full flow")}</button>
    <span data-scan-attachment>{attached ? (zh ? "前台已连接" : "Subscriber attached") : (zh ? "前台已离开 · 后台继续" : "Subscriber detached · background continues")}</span>
  </section>;
}

export function ScanStatus({ scan, lang, empty = false }) {
  const t = copy[lang];
  const failure = ["failed", "partial"].includes(scan.phase);
  const known = !["waiting", "checking"].includes(scan.phase);
  const usageDone = ["usage-complete", "statistics", "completed", "partial", "failed"].includes(scan.phase);
  const sessionDone = ["statistics", "completed"].includes(scan.phase);
  const Icon = failure ? WarningCircle : scan.phase === "completed" ? CheckCircle : SpinnerGap;
  return <section className={"scan-status" + (failure ? " scan-error" : "") + (empty ? " scan-empty-status" : "")} data-scan-status data-phase={scan.phase}>
    <div className="scan-status-title" role="status" aria-live="polite" aria-atomic="true">
      <Icon size={17} aria-hidden="true" className={scan.active ? "spin" : ""} /><strong>{t[scan.phase]}</strong>
    </div>
    {known ? <div className="scan-domain-counts" aria-live="off">
      <span>{t.usage}<b>{usageDone ? t.complete : "640 / 1,720"}</b></span>
      <span>{t.session}<b>{failure ? t.sessionFailed : sessionDone ? t.complete : usageDone ? "1,280 / 1,720" : "512 / 1,720"}</b></span>
      {!usageDone && <small>{lang === "zh" ? "按已提交文件计数 · 已跳过 240" : "Committed files · 240 skipped"}</small>}
    </div> : <p>{t.unknown}</p>}
    {scan.phase !== "completed" && <p>{scan.hasSnapshot ? t.retained : failure ? t.unavailable : t.initial}</p>}
    {failure && <button type="button" data-scan-retry onClick={scan.restart}>{t.retry}</button>}
  </section>;
}

export function ScanEmptyPopover({ scan, lang, width }) {
  return <section className="popover scan-empty" style={{ "--popover-w": width + "px" }} data-width={width} data-scan-snapshot="none" aria-label="AgentDeck">
    <header><div className="brand"><img src="/agentdeck-robot.png" alt="" width={22} height={22} /><strong>AgentDeck</strong></div></header>
    <div className="scan-loading-body"><ScanStatus scan={scan} lang={lang} empty /></div>
  </section>;
}

export function CliScan({ stage }) {
  const scan = useScanScenario();
  const params = new URLSearchParams(window.location.search);
  const [scope, setScope] = useState(["usage", "session"].includes(params.get("scope")) ? params.get("scope") : "all");
  const [mode, setMode] = useState(["pipe", "json", "quiet"].includes(params.get("mode")) ? params.get("mode") : "tty");
  const [receipt, setReceipt] = useState(null);
  const [detached, setDetached] = useState(false);
  const [columns, setColumns] = useState(params.get("cols") === "40" ? 40 : 80);
  useEffect(() => { setReceipt(null); setDetached(false); }, [scan.round, scope]);
  const usageDone = ["usage-complete", "statistics", "completed", "partial", "failed"].includes(scan.phase);
  const sessionDone = ["statistics", "completed"].includes(scan.phase);
  const failed = ["partial", "failed"].includes(scan.phase);
  useEffect(() => {
    if (receipt || detached) return;
    if ((scope === "usage" && usageDone) || (scope !== "usage" && (sessionDone || failed))) {
      setReceipt({ scope, usage: "completed", sessions: sessionDone ? "completed" : failed ? "failed" : "processing", exit: scope === "usage" || !failed ? 0 : 1 });
    }
  }, [scope, usageDone, sessionDone, failed, receipt, detached]);
  let progress = "";
  if (!receipt && !detached && mode === "tty") {
    progress = ["waiting", "checking"].includes(scan.phase) ? copy.en[scan.phase] : scope === "all" ? (columns === 40 ? "Usage 640/1720 · Sessions 512/1720" : "Usage 640/1720 · Sessions 512/1720 files committed · 240 skipped") : (scope === "usage" ? "Usage 640" : "Sessions 512") + "/1720 files committed";
    if (scan.phase === "usage-complete" && scope === "session") progress = "Sessions 1280/1720 files committed";
    if (scan.phase === "usage-complete" && scope === "all") progress = columns === 40 ? "Usage done · Sessions 1280/1720" : "Usage complete · Sessions 1280/1720 files committed";
  }
  const stdout = receipt ? mode === "json" ? JSON.stringify({ command: "scan", data: { scope: receipt.scope, usage: receipt.usage, sessions: receipt.sessions }, partial: receipt.exit !== 0 }, null, 2) : receipt.exit ? "Scan incomplete: sessions failed." : scope === "all" ? "Scan complete: usage and sessions." : "Scan complete: " + scope + "." : "";
  const stderr = detached ? "Detached from scan." : receipt?.exit ? "Session scan failed." : progress;
  return <>
    <ScanStageControls scan={scan} lang={stage.lang} attached={!detached && !receipt} />
    <div className="scan-cli-options">
      <label>Scope<select data-cli-scope value={scope} onChange={(e) => setScope(e.target.value)}>{["all", "usage", "session"].map((s) => <option key={s}>{s}</option>)}</select></label>
      <label>Output<select data-cli-mode value={mode} onChange={(e) => setMode(e.target.value)}>{["tty", "pipe", "quiet", "json"].map((s) => <option key={s}>{s}</option>)}</select></label>
      <label>Columns<select data-cli-cols value={columns} onChange={(e) => setColumns(Number(e.target.value))}><option>80</option><option>40</option></select></label>
      <button type="button" data-cli-detach disabled={detached || !!receipt} onClick={() => setDetached(true)}>Ctrl-C · detach</button>
    </div>
    <p className="cli-note">{stage.lang === "zh" ? "舞台标本：stdout 与 stderr 分开呈现；退出后输出冻结，后台流程仍可继续。JSON 展示新 scan 命令的结果草案，不替换旧命令格式。" : "Specimen: stdout and stderr are separate. Output freezes after exit while background work continues. JSON is the proposed new scan result, not a replacement for legacy formats."}</p>
    <div className="terminal scan-terminal" data-columns={columns} style={{ maxWidth: (columns + 6) + "ch" }}>
      <div className="terminal-bar"><span>agentdeck scan{scope !== "all" ? " --scope " + scope : ""}{mode === "quiet" ? " --quiet" : mode === "json" ? " --format json" : ""}</span></div>
      <div className="scan-stream-label">stdout</div><pre data-scan-stdout>{stdout || "\u00a0"}</pre>
      <div className="scan-stream-label">stderr</div><pre data-scan-stderr aria-live="off">{stderr || "\u00a0"}</pre>
    </div>
    <p className="cli-note" data-scan-exit>{receipt ? "Exit " + receipt.exit : detached ? "Exit 130" : "Foreground running"}</p>
  </>;
}
