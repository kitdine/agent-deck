// Synthetic character specimens; no product commands or private data are executed.
const list = (scope, lines, total, unknown = 0) => [
  'AGENTDECK · SESSIONS', `Scope: ${scope}`, ...(unknown ? [`Unknown origin: ${unknown} (included in Work)`] : []),
  'All usage and costs remain unchanged.', '', ...lines, '', `Showing ${total ? 1 : 0}-${total} of ${total}`,
].join('\n');
export const AUXILIARY_CLI = {
  work: {command: 'agentdeck session list', text: list('work (default)', ['codex  task-01  WORK     demo', 'codex  child-1  WORK     demo', 'claude old-01   UNKNOWN  demo'], 3, 1)},
  auxiliary: {command: 'agentdeck session list --session-scope auxiliary', text: list('auxiliary (explicit)', ['claude maint-1  AUXILIARY  maintenance'], 1)},
  all: {command: 'agentdeck session list --session-scope all', text: list('all (explicit)', ['codex  task-01  WORK       demo', 'codex  child-1  WORK       demo', 'claude old-01   UNKNOWN    demo', 'claude maint-1  AUXILIARY  maintenance'], 4, 1)},
  empty: {command: 'agentdeck session list --session-scope auxiliary', text: list('auxiliary (explicit)', ['No auxiliary sessions in this selection.', 'Try: agentdeck session list --session-scope all'], 0)},
  noIndex: {command: 'agentdeck session list', text: 'AGENTDECK · SESSIONS\nScope: work (default)\nNo indexed sessions.\nRun: agentdeck session scan'},
  unknown: {command: 'agentdeck session classify old-01 --client claude', text: 'SESSION ORIGIN\nClient: claude\nSession: old-01\nEffective: unknown\nAutomatic: unknown\nReason: origin_metadata_missing\nRule: origin-v1\nOverride: none\nIncluded in Work.\nNo source content is displayed.'},
  detail: {command: 'agentdeck session show maint-1 --client claude', text: 'SESSION · maint-1\nClient: claude\nOrigin: AUXILIARY\nReason: maintenance_origin\nOverride: none\nOpened by explicit ID; scope is not applied.\n\nDOCUMENTS\nNo approved visible documents.\nShowing 0-0 of 0'},
  override: {command: 'agentdeck session classify old-01 --client claude --set work', text: 'CLASSIFICATION SAVED\nClient: claude\nSession: old-01\nEffective: work (user override)\nAutomatic: unknown\nReason: origin_metadata_missing\nViews: pending publication\nAll usage and costs remain unchanged.\nUndo: agentdeck session classify old-01 --client claude --clear'},
  cleared: {command: 'agentdeck session classify old-01 --client claude --clear', text: 'CLASSIFICATION CLEARED\nClient: claude\nSession: old-01\nEffective: unknown (automatic)\nViews: pending publication\nUnknown origin remains included in Work.'},
  partial: {command: 'agentdeck session list', text: list('work (default)', ['PARTIAL: origin metadata is incomplete.', 'Unresolved sessions remain UNKNOWN.', 'claude old-01  UNKNOWN  demo'], 1, 1)},
  stale: {command: 'agentdeck session list', text: list('work (default)', ['STALE: showing last published selection.', 'Snapshot: 2026-10-26T16:00:00Z', 'Classification update is pending.', 'codex task-01  WORK  demo'], 1)},
  unavailable: {command: 'agentdeck session list', stderr: true, text: 'Session index unavailable.\nNo counts can be reported.\nRun: agentdeck doctor'},
  invalid: {command: 'agentdeck session list --session-scope other', stderr: true, text: 'Invalid session scope: other\nExpected: work, auxiliary, all'},
  ambiguous: {command: 'agentdeck session classify shared-01 --set work', stderr: true, text: 'Session ID is ambiguous across clients.\nSpecify --client codex or --client claude.\nNo classification was changed.'},
  writeFailed: {command: 'agentdeck session classify old-01 --client claude --set work', stderr: true, text: 'Classification was not saved.\nPrevious override remains unchanged.\nRun: agentdeck doctor\nRetry the command after resolving the issue.'},
  saveUnknown: {command: 'agentdeck session classify old-01 --client claude --set work', stderr: true, text: 'Save status unknown.\nInspect classification before retrying.\nRun: agentdeck session classify old-01 --client claude'},
  noChange: {command: 'agentdeck session classify old-01 --client claude --clear', text: 'NO CHANGE\nClient: claude\nSession: old-01\nEffective: unknown (automatic)\nOverride: none\nUnknown origin remains included in Work.'},
  scopeFailed: {command: 'agentdeck session --interactive', text: 'AGENTDECK · SESSIONS · READ ONLY\nScope: work | Unknown: 1\nRequested scope auxiliary failed.\nShowing previous published selection.\n\n> codex  task-01  WORK     demo\n  codex  child-1  WORK     demo\n  claude old-01   UNKNOWN  demo\nrow 1/3\nesc back · s scope · q quit'},
  notFound: {command: 'agentdeck session classify missing-01 --client codex', stderr: true, text: 'Session not found.\nNo classification was changed.\nUse: agentdeck session list --session-scope all'},
  nonTTY: {command: 'agentdeck session --interactive', stderr: true, text: 'Interactive sessions require TTY input and output and a usable TERM.\nUse: agentdeck session list --session-scope work'},
  browser: {command: 'agentdeck session --interactive', text: 'AGENTDECK · SESSIONS · READ ONLY\nScope: work | Unknown: 1\nAll usage and costs remain unchanged.\n\n> codex  task-01  WORK     demo\n  codex  child-1  WORK     demo\n  claude old-01   UNKNOWN  demo\n\nrow 1/3\ns scope · enter open · q/esc quit'},
  scopeMenu: {command: 'agentdeck session --interactive', text: 'SELECT SESSION SCOPE\n> Work (includes unknown)\n  Auxiliary\n  All\n\nUp/Down select · Enter apply\nEscape cancel · q quit'},
  loading: {command: 'agentdeck session --interactive', text: 'AGENTDECK · SESSIONS · READ ONLY\nRequested scope: auxiliary\nLoading committed session data...\nNo new counts available yet.\n\nq/esc quit'},
  tooSmall: {command: 'agentdeck session --interactive', text: 'Terminal too small.\nResize to at least 48x10.\nq/esc quit'},
  signals: {command: 'agentdeck usage signals --session-scope work', text: 'WORK SIGNALS\nScope: work (explicit)\nUnknown origin: included\nActivity: unavailable\nWorkflow: unavailable\nTooling: unavailable\nAll usage and costs remain unchanged.'},
  signalsScanning: {command: 'agentdeck usage signals --session-scope work', stderrText: 'Scanning usage sources...\nScope filtering adds no scan.', text: ''},
  signalsScanIncomplete: {command: 'agentdeck usage signals --session-scope work', stderrText: 'Warning: scan_incomplete', text: 'WORK SIGNALS\nScope: work (explicit)\nPARTIAL: scan_incomplete\nShowing last committed data.\nActivity: unavailable\nWorkflow: unavailable\nTooling: unavailable'},
  statsScanning: {command: 'agentdeck usage stats --session-scope work', stderrText: 'Scanning usage sources...\nScope filtering adds no scan.', text: ''},
  statsStored: {command: 'agentdeck usage stats --no-scan --session-scope work', text: 'USAGE STATS\nUsage scope: all\nStored aggregates (--no-scan).\n\nWORK SIGNALS\nScope: work (explicit)\nUnknown origin: included'},
};

// Full-screen character specimen, not the product TUI or input implementation.
export const AUXILIARY_FULLSCREEN = ['browser', 'scopeFailed', 'scopeMenu', 'loading', 'tooSmall'];
export function auxiliaryFrame(state, cols, rows, selected = 1) {
  if (!AUXILIARY_FULLSCREEN.includes(state)) return null;
  const fit = lines => [...lines, ...Array(Math.max(0, rows - lines.length)).fill('')].join('\n');
  if (cols < 48 || rows < 10 || state === 'tooSmall') {
    return {mode: 'tooSmall', text: fit(['Terminal too small.', 'Resize to at least 48x10.', 'q/esc quit'])};
  }
  if (state === 'scopeMenu') {
    const items = ['Work (includes unknown)', 'Auxiliary', 'All'];
    const choice = Math.min(3, selected);
    return {mode: state, text: fit(['SELECT SESSION SCOPE', ...items.map((x,i) => `${i + 1 === choice ? '>' : ' '} ${x}`), ...Array(rows - 6).fill(''), 'Up/Down select · Enter apply', 'Escape cancel · q quit'])};
  }
  if (state === 'loading') {
    return {mode: state, text: fit(['AGENTDECK · SESSIONS · READ ONLY', 'Requested scope: auxiliary', 'Loading committed session data...', 'No new counts available yet.', ...Array(rows - 5).fill(''), 'q/esc quit'])};
  }
  const count = 12;
  const chosen = Math.max(1, Math.min(count, selected));
  const capacity = rows - 6; // Four header/status rows, two footer rows.
  const start = Math.max(0, Math.min(chosen - 1, count - Math.min(count, capacity)));
  const visible = Array.from({length: Math.min(capacity, count)}, (_, offset) => {
    const n = start + offset + 1;
    return `${n === chosen ? '>' : ' '} codex task-${String(n).padStart(2, '0')}  ${n === 12 ? 'UNKNOWN' : 'WORK   '} demo`;
  });
  const header = ['AGENTDECK · SESSIONS · READ ONLY', 'Scope: work | Unknown: 1',
    state === 'scopeFailed' ? 'Requested scope auxiliary failed.' : 'All usage and costs remain unchanged.',
    state === 'scopeFailed' ? 'Showing previous published selection.' : ''];
  return {mode: state, text: [...header, ...visible, ...Array(Math.max(0, capacity - visible.length)).fill(''),
    `row ${chosen}/${count} · visible ${start + 1}-${start + visible.length}`,
    state === 'scopeFailed' ? 'esc back · s scope · q quit' : 's scope · enter open · q/esc quit'].join('\n')};
}
export const AUXILIARY_JSON = `Required additive data (framework only; final envelope/field placement awaits architecture):
requested_scope: omitted | work | auxiliary | all
effective_scope: all when JSON scope is omitted
origin: effective, automatic, reason_code, rule_version, override
selection: total, unknown_count, publication_identity, availability
mutation: saved/no_change, effective_origin, publication_pending

Fixture oracle: task-01 work, child-1 work, old-01 unknown, maint-1 auxiliary.
No scope: 4 rows. Explicit work: 3. Auxiliary: 1. All: 4.
--page 1 --limit 1 with auxiliary: maint-1; total 1.
Legacy fields and envelope warnings/partial retain their meanings.
Errors go to stderr; stdout is empty. No ANSI or confirmation prompts.`;
