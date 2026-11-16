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
  unknown: {command: 'agentdeck session classify old-01 --client claude', text: 'SESSION ORIGIN\nClient: claude\nSession: old-01\nEffective: unknown\nAutomatic: unknown\nReason: origin_metadata_missing\nRule: origin-v1\nOverride: none\nRevision: 6\nIncluded in Work.\nNo source content is displayed.'},
  detail: {command: 'agentdeck session show maint-1 --client claude', text: 'SESSION · maint-1\nClient: claude\nOrigin: AUXILIARY\nReason: maintenance_origin\nOverride: none\nOpened by explicit ID; scope is not applied.\n\nDOCUMENTS\nNo approved visible documents.\nShowing 0-0 of 0'},
  override: {command: 'agentdeck session classify old-01 --client claude --set work --request-id set-A', text: 'CLASSIFICATION SAVED\nTarget: claude / old-01\nRequest ID: set-A\nOutcome: saved\nRevision: 7\nEffective: work (user override)\nAutomatic: unknown\nReason: origin_metadata_missing\nViews: pending publication\nAll usage and costs remain unchanged.\nUndo: agentdeck session classify old-01 --client claude --clear'},
  cleared: {command: 'agentdeck session classify archived-1 --client codex --clear --expected-revision 4 --request-id clear-A', text: 'OVERRIDE CLEARED\nTarget: codex / archived-1\nRequest ID: clear-A\nOutcome: saved\nRevision: 5\nOverride: none\nAutomatic: unavailable (not queried)\nEffective: unavailable (not queried)\nViews: pending publication\nNo session source was queried.'},
  partial: {command: 'agentdeck session list', text: list('work (default)', ['PARTIAL: origin metadata is incomplete.', 'Unresolved sessions remain UNKNOWN.', 'claude old-01  UNKNOWN  demo'], 1, 1)},
  stale: {command: 'agentdeck session list', text: list('work (default)', ['STALE: showing last published selection.', 'Snapshot: 2026-10-26T16:00:00Z', 'Classification update is pending.', 'codex task-01  WORK  demo'], 1)},
  unavailable: {command: 'agentdeck session list', stderr: true, text: 'Session index unavailable.\nNo counts can be reported.\nRun: agentdeck doctor'},
  invalid: {command: 'agentdeck session list --session-scope other', stderr: true, text: 'Invalid session scope: other\nExpected: work, auxiliary, all'},
  ambiguous: {command: 'agentdeck session classify shared-01 --set work', stderr: true, text: 'Session ID is ambiguous across clients.\nSpecify --client codex or --client claude.\nNo classification was changed.'},
  writeFailed: {command: 'agentdeck session classify old-01 --client claude --set work', stderr: true, text: 'Classification was not saved.\nPrevious override remains unchanged.\nRun: agentdeck doctor\nRetry the command after resolving the issue.'},
  saveUnknown: {command: 'agentdeck session classify archived-1 --client codex --clear --request-id clear-A', stderr: true, text: 'origin_write_unknown: Save status unknown.\nRequest ID: clear-A\nRead the request receipt before retrying.\nRun: agentdeck session classify --request clear-A\nDo not retry with a new request ID.'},
  noChange: {command: 'agentdeck session classify archived-1 --client codex --clear --expected-revision 5 --request-id clear-B', text: 'NO CHANGE\nTarget: codex / archived-1\nRequest ID: clear-B\nOutcome: no_change\nRevision: 5\nOverride: none\nAutomatic: unavailable (not queried)\nEffective: unavailable (not queried)\nNo new publication work.'},
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
  overrideList: {command: 'agentdeck session classify list --page 1 --limit 20', text: 'USER OVERRIDES\nCLIENT  SESSION     VALUE      BINDING    REVISION\ncodex   archived-1  auxiliary  unmatched  4\nShowing 1-1 of 1\nNo source content is displayed.'},
  clearMissing: {command: 'agentdeck session classify missing-01 --client codex --clear --expected-revision 0 --request-id clear-zero', text: 'NO CHANGE\nTarget: codex / missing-01\nRequest ID: clear-zero\nOutcome: no_change\nRevision: 0\nOverride: none\nAutomatic: unavailable (not queried)\nEffective: unavailable (not queried)'},
  conflict: {command: 'agentdeck session classify archived-1 --client codex --clear --expected-revision 4 --request-id clear-C', stderr:true, text: 'origin_conflict: Override revision changed.\nExpected: 4\nCurrent: 6\nNo override was cleared.\nRun: agentdeck session classify list\nReview the latest value before a new request.'},
  receiptReplay: {command:'agentdeck session classify --request clear-A',text:'REQUEST RECEIPT\nRequest ID: clear-A\nTarget: codex / archived-1\nOutcome: saved\nRevision: 5\nThis is the original operation result.\nIt does not describe the current override.\nInspect current values: agentdeck session classify list'},
  requestUnknown: {command:'agentdeck session classify --request unknown-A',text:'REQUEST STATUS UNKNOWN\nRequest ID: unknown-A\nOutcome: unknown\nNo confirmed receipt is available.\nNo mutation was retried.'},
  scopeGet: {command:'agentdeck session scope get',text:'DEFAULT SESSION SCOPE\nConfirmed: yes\nSaved: work\nRevision: 0\nReason: no_configuration\nCLI defaults are unchanged.'},
  scopeUnconfirmed: {command:'agentdeck session scope get',stderr:true,text:'Default session scope is unavailable.\nConfirmed: no\nSaved: unavailable\nNo Work default is assumed.'},
  scopeSet: {command:'agentdeck session scope set auxiliary --expected-revision 0 --request-id scope-A',text:'DEFAULT SCOPE SAVED\nRequest ID: scope-A\nOutcome: saved\nSaved: auxiliary\nRevision: 1\nApplies on next menu-bar open and Widget publication.\nCurrent temporary scope and CLI defaults are unchanged.'},
  sourceSet: {command:'agentdeck session origin-source set claude-mem --data-dir /example/claude-mem',text:'ORIGIN SOURCE CONFIGURED\nSource: claude-mem\nLocation: configured (not displayed)\nMapping coverage: not verified\nNo source scan was started.'},
  sourceDisabled: {command:'agentdeck session origin-source disable claude-mem',text:'ORIGIN SOURCE DISABLED\nSource: claude-mem\nAutomatic evidence invalidated.\nUser overrides and all usage are retained.\nNo source scan was started.'},
  sourceUnavailable: {command:'agentdeck session classify old-01 --client claude',text:'SESSION ORIGIN\nClient: claude\nSession: old-01\nEffective: unknown\nAutomatic: unknown\nReason: origin_source_unavailable\nOverride: none\nIncluded in Work; source availability is incomplete.'},
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
// Final-surface JSON examples use the existing envelope shape plus approved additive fields.
const envelope=(command,data,extra={})=>({schema_version:1,command,generated_at:'2026-10-27T09:00:00Z',data,warnings:[],partial:false,...extra});
const target={client:'codex',session_id:'archived-1'};
const clearReceipt={request_id:'clear-A',target,outcome:'saved',override:null,revision:5,publication_pending:true,automatic:null,effective:null,classification_available:false,classification_reason:'not_queried'};
const error=(command,code,message,details)=>envelope(command,null,{error:{code,message,...details?{details}:{}}});
const rows=[{client:'codex',session_id:'task-01'},{client:'codex',session_id:'child-1'},{client:'claude',session_id:'old-01'},{client:'claude',session_id:'maint-1'}];
export const AUXILIARY_JSON_EXAMPLES={
  unknown:envelope('session.classify',{target:{client:'claude',session_id:'old-01'},classification_available:true,origin:{automatic:'unknown',effective:'unknown',reason_code:'origin_metadata_missing',rule_version:'origin-v1',override:null,revision:6}}),
  override:envelope('session.classify',{request_id:'set-A',target:{client:'claude',session_id:'old-01'},outcome:'saved',override:'work',revision:7,publication_pending:true,automatic:'unknown',effective:'work',classification_available:true}),
  all:envelope('session.list',rows,{session_selection:{requested_scope:'all',effective_scope:'all',source_default:'explicit',availability:'available',unknown_count:1,read_vector:{core_epoch:1,scope_data_revision:12,session_epoch:2,session_revision:9},publication_id:'fixture-publication-12'}}),
  jsonDefault:envelope('session.list',rows,{session_selection:{requested_scope:null,effective_scope:'all',source_default:'json_compat',availability:'available',unknown_count:1,read_vector:{core_epoch:1,scope_data_revision:12,session_epoch:2,session_revision:9},publication_id:'fixture-publication-12'}}),
  auxiliary:envelope('session.list',[rows[3]],{session_selection:{requested_scope:'auxiliary',effective_scope:'auxiliary',source_default:'explicit',availability:'available',unknown_count:0,read_vector:{core_epoch:1,scope_data_revision:12,session_epoch:2,session_revision:9},publication_id:'fixture-publication-12'}}),
  cleared:envelope('session.classify',clearReceipt),
  noChange:envelope('session.classify',{...clearReceipt,request_id:'clear-B',outcome:'no_change',publication_pending:false}),
  clearMissing:envelope('session.classify',{...clearReceipt,request_id:'clear-zero',target:{client:'codex',session_id:'missing-01'},outcome:'no_change',revision:0,publication_pending:false}),
  receiptReplay:envelope('session.classify.request',clearReceipt),
  requestUnknown:envelope('session.classify.request',{request_id:'unknown-A',outcome:'unknown'}),
  saveUnknown:error('session.classify','origin_write_unknown','Save status unknown',{request_id:'clear-A'}),
  conflict:error('session.classify','origin_conflict','Override revision changed',{expected_revision:4,current_revision:6}),
  overrideList:envelope('session.classify.list',[{...target,value:'auxiliary',binding_status:'unmatched',revision:4}],{pagination:{overrides:{page:1,limit:20,total:1,shown:1,has_more:false,next_page:null}}}),
  scopeGet:envelope('session.scope.get',{confirmed:true,value:'work',revision:0,reason:'no_configuration'}),
  scopeUnconfirmed:error('session.scope.get','runtime_error','Default session scope unavailable',{default_scope:{confirmed:false,value:null,revision:null,reason:'read_failed'}}),
  scopeSet:envelope('session.scope.set',{request_id:'scope-A',outcome:'saved',confirmed:true,value:'auxiliary',revision:1,publication_pending:true}),
  sourceSet:envelope('session.origin-source.set',{source:'claude-mem',configured:true,enabled:true,mapping_coverage:'not_verified'}),
  sourceDisabled:envelope('session.origin-source.disable',{source:'claude-mem',configured:true,enabled:false,evidence_invalidated:true,mapping_coverage:'unavailable'}),
};
export const AUXILIARY_JSON=JSON.stringify(AUXILIARY_JSON_EXAMPLES.jsonDefault,null,2);
