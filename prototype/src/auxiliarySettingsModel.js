// Memory-only interaction fixture. No product IO, persistence or real CAS execution.
export const SETTINGS_SCENARIOS=['normal','loading','loadFailed','saving','saveFailed','saveUnknown','receiptMissing','conflict','pending','replayed','permissionWarning','sourceUnconfigured','sourceUnavailable','sourceUnsupported','codexReceiverOff','codexNoRecords','codexReceiverUnknown'];
export const SETTING_SESSIONS=[
  {client:'codex',id:'task-01',project:'demo',automatic:'unknown',reason:'origin_metadata_missing'},
  {client:'claude',id:'old-01',project:'research',automatic:'unknown',reason:'origin_metadata_missing'},
  {client:'claude',id:'maint-1',project:'demo',automatic:'auxiliary',reason:'maintenance_origin'},
];
export const settingKey=x=>`${x.client}:${x.id}`;
export function targetRevision(m,a){return a.type==='default'?m.defaultRevision:(m.versions[a.key]??0);}
export function initialAuxSettings(scenario='normal') {
  const confirmed=!['loading','loadFailed'].includes(scenario);
  const model={scenario,status:confirmed?'ready':scenario,defaultScope:confirmed?'work':null,defaultConfirmed:confirmed,defaultRevision:0,
    overrides:{'claude:old-01':'work','codex:archived-1':'auxiliary'},versions:{'claude:old-01':2,'codex:archived-1':4},inspected:{},
    pending:null,receipt:null,receipts:{},serverState:null,nextRequest:1,faultUsed:false,revision:1,warnings:[]};
  if(scenario==='replayed'){model.status='replayed';model.overrides['claude:old-01']='auxiliary';model.versions['claude:old-01']=4;model.receipt={request_id:'fixture-old-clear',target:'claude:old-01',operation:'clear',outcome:'saved',revision:3,override:null,automatic:null,effective:null,classification_available:false,classification_reason:'not_queried'};}
  return model;
}
const payload=a=>JSON.stringify([a.type,a.key??null,a.value,a.expected_revision]);
function apply(m,a){
  const previous=m.receipts[a.request_id];
  if(previous)return previous.payload===payload(a)?{...m,status:'replayed',receipt:previous.receipt,pending:null}:{...m,status:'invalidRequest',pending:null};
  const revision=targetRevision(m,a);
  if(a.expected_revision!==revision)return {...m,status:'conflict',pending:a,serverState:null};
  const same=a.type==='default'?m.defaultScope===a.value:(m.overrides[a.key]??'auto')===a.value;
  const nextRevision=same?revision:revision+1;
  const overrides={...m.overrides},versions={...m.versions},inspected={...m.inspected};
  if(a.type==='override'){
    if(a.value==='auto'){delete overrides[a.key];delete inspected[a.key];}else{overrides[a.key]=a.value;inspected[a.key]=nextRevision;}
    if(!same)versions[a.key]=nextRevision;
  }
  const receipt={request_id:a.request_id,target:a.type==='default'?'default_scope':a.key,operation:a.type==='override'&&a.value==='auto'?'clear':a.type,
    outcome:same?'no_change':'saved',revision:nextRevision,publication_pending:!same&&m.scenario==='pending',
    ...(a.type==='override'&&a.value==='auto'?{override:null,automatic:null,effective:null,classification_available:false,classification_reason:'not_queried'}:{})};
  return {...m,overrides,versions,inspected,warnings:m.scenario==='permissionWarning'?['state_permissions_warning']:m.warnings,defaultScope:a.type==='default'?a.value:m.defaultScope,defaultRevision:a.type==='default'?nextRevision:m.defaultRevision,
    status:same?'noChange':m.scenario==='pending'?'pending':'saved',pending:null,serverState:null,receipt,
    receipts:{...m.receipts,[a.request_id]:{payload:payload(a),receipt}},revision:m.revision+1};
}
export function auxiliarySettingsTransition(m,event){
  if(event.type==='scenario')return initialAuxSettings(event.value);
  if(event.type==='inspect')return {...m,inspected:{...m.inspected,[event.key]:m.versions[event.key]??0}};
  if(event.type==='read'){
    if(m.status==='saveUnknown')return m.serverState?{...m.serverState,status:'saved',serverState:null,pending:null}:{...m,receipt:{request_id:m.pending.request_id,target:m.pending.key??'default_scope',outcome:'unknown',revision:null}};
    if(m.status==='conflict')return {...(m.serverState??m),status:'reloaded',serverState:null,pending:null,receipt:null,revision:(m.serverState?.revision??m.revision)+1};
    return {...m,status:'ready',pending:null,defaultScope:m.defaultConfirmed?m.defaultScope:'work',defaultConfirmed:true};
  }
  if(event.type==='retry'&&m.status==='saveUnknown'&&m.receipt?.outcome==='unknown')return apply(m.serverState??m,m.pending);
  if(event.type==='complete'&&m.status==='loading')return {...m,status:'ready',defaultScope:'work',defaultConfirmed:true};
  if((event.type==='complete'&&m.status==='saving')||(event.type==='retry'&&m.status==='saveFailed'))return apply(m,m.pending);
  if(event.type!=='save'||['loading','loadFailed','saving','saveUnknown','conflict'].includes(m.status))return m;
  const a={...event.action,expected_revision:event.action.expected_revision??targetRevision(m,event.action),request_id:event.action.request_id??`fixture-request-${m.nextRequest}`};
  const next={...m,nextRequest:m.nextRequest+1};
  if(m.receipts[a.request_id]||a.expected_revision!==targetRevision(m,a))return apply(next,a);
  const same=a.type==='default'?m.defaultScope===a.value:(m.overrides[a.key]??'auto')===a.value;
  if(!same&&!m.faultUsed&&['saving','saveFailed','saveUnknown','receiptMissing','conflict'].includes(m.scenario)){
    const pending={...next,status:m.scenario==='receiptMissing'?'saveUnknown':m.scenario,pending:a,faultUsed:true,receipt:null};
    if(m.scenario==='saveUnknown')pending.serverState=apply({...next,faultUsed:true},a);
    if(m.scenario==='conflict')pending.serverState=apply({...next,faultUsed:true},{...a,request_id:'fixture-external-write',value:a.type==='default'?'all':'auxiliary'});
    return pending;
  }
  return apply(next,a);
}
