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
