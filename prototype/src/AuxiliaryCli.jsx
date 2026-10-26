import { useState } from 'react';
import { AUXILIARY_CLI, AUXILIARY_JSON, auxiliaryFrame } from './auxiliaryCli.js';
export function AuxiliaryCli() {
  const params = new URLSearchParams(window.location.search);
  const [state, setState] = useState(params.get('auxiliary') || 'work');
  const [cols, setCols] = useState(params.get('auxCols') || '80');
  const [rows, setRows] = useState(params.get('auxRows') || '24');
  const [selected, setSelected] = useState(params.get('auxSelected') || '1');
  const sample = AUXILIARY_CLI[state] || AUXILIARY_CLI.work;
  const frame = auxiliaryFrame(state, Number(cols), Number(rows), Number(selected));
  return <section aria-label="Auxiliary session CLI specimens">
    <p className="cli-note">Framework specimen · synthetic data · controls below are prototype tools. No product commands run.</p>
    <label>Scenario <select value={state} onChange={e => setState(e.target.value)}>{Object.keys(AUXILIARY_CLI).map(key => <option key={key}>{key}</option>)}</select></label>{' '}
    <label>Columns <select value={cols} onChange={e => setCols(e.target.value)}>{['40', '48', '80', '120'].map(key => <option key={key}>{key}</option>)}</select></label>
    {' '}<label>Rows <select value={rows} onChange={e => setRows(e.target.value)}>{['8', '10', '24', '40'].map(key => <option key={key}>{key}</option>)}</select></label>
    {' '}<label>Selected row <select value={selected} onChange={e => setSelected(e.target.value)}>{['1', '8', '12'].map(key => <option key={key}>{key}</option>)}</select></label>
    <p className="cli-note">{frame ? `Full-screen boundary: ${cols} columns × ${rows} rows. Twelve synthetic work-view rows for viewport checks.` : 'Line output: row count does not truncate output.'}</p>
    <p><code>$ {sample.command}</code></p>
    <div className="terminal" style={{width: frame ? 'max-content' : undefined, maxWidth: frame ? 'none' : '100%'}}><div className="terminal-bar"><span>{sample.stderr ? 'stderr' : 'stdout / terminal frame'}</span></div>
      <pre data-auxiliary-state={state} data-frame-mode={frame?.mode || 'line'} data-frame-rows={frame ? rows : undefined} tabIndex={0} style={{width: `${cols}ch`, maxWidth: frame ? 'none' : '100%', boxSizing: 'content-box', fontSize: '13px', lineHeight: '20px', height: frame ? `${Number(rows) * 20}px` : undefined, whiteSpace: frame ? 'pre' : 'pre-wrap', overflowWrap: frame ? 'normal' : 'anywhere', overflow: frame ? 'hidden' : undefined}}><code>{frame?.text ?? sample.text}</code></pre></div>
    {sample.stderrText && <div className="terminal"><div className="terminal-bar"><span>stderr · existing scan lifecycle</span></div><pre data-scan-progress style={{whiteSpace: 'pre-wrap'}}>{sample.stderrText}</pre></div>}
    <details><summary>Machine-readable data requirements</summary><pre style={{whiteSpace: 'pre-wrap'}}>{AUXILIARY_JSON}</pre></details>
  </section>;
}
