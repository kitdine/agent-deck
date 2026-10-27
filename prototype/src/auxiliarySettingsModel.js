// Synthetic, memory-only design model. No filesystem or product mutations.
export const SETTINGS_SCENARIOS=['normal','loading','loadFailed','saving','saveFailed','saveUnknown','conflict','pending'];
export const SETTING_SESSIONS=[
  {client:'codex',id:'task-01',project:'demo',automatic:'work',reason:'user_task'},
  {client:'claude',id:'old-01',project:'research',automatic:'unknown',reason:'origin_metadata_missing'},
  {client:'claude',id:'maint-1',project:'demo',automatic:'auxiliary',reason:'maintenance_origin'},
];
export const settingKey=x=>`${x.client}:${x.id}`;
export function initialAuxSettings(scenario='normal') {const confirmed=!['loading','loadFailed'].includes(scenario);return {scenario,status:confirmed?'ready':scenario,defaultScope:confirmed?'work':null,defaultConfirmed:confirmed,overrides:{'claude:old-01':'work','codex:archived-1':'auxiliary'},pending:null,faultUsed:false,revision:1};}
function commit(m,action){
  if(action.type==='default')return {...m,defaultScope:action.value};
  const overrides={...m.overrides};if(action.value==='auto')delete overrides[action.key];else overrides[action.key]=action.value;
  return {...m,overrides};
}
export function auxiliarySettingsTransition(m,event){
  if(event.type==='scenario')return initialAuxSettings(event.value);
  if(event.type==='read'){
    if(m.status==='saveUnknown')return {...commit(m,m.pending),status:'saved',pending:null,revision:m.revision+1};
    if(m.status==='conflict'){
      const changed=commit(m,{...m.pending,value:m.pending.type==='default'?'all':'auxiliary'});
      return {...changed,status:'reloaded',pending:null,revision:m.revision+1};
    }
    return {...m,status:'ready',pending:null,defaultScope:m.defaultConfirmed?m.defaultScope:'work',defaultConfirmed:true};
  }
  if(event.type==='complete'&&m.status==='loading')return {...m,status:'ready',defaultScope:'work',defaultConfirmed:true};
  if((event.type==='complete'&&m.status==='saving')||(event.type==='retry'&&m.status==='saveFailed'))return {...commit(m,m.pending),status:'saved',pending:null,revision:m.revision+1};
  if(event.type!=='save'||['loading','loadFailed','saving','saveUnknown','conflict'].includes(m.status))return m;
  const a=event.action;
  const unchanged=a.type==='default'?m.defaultScope===a.value:(m.overrides[a.key]??'auto')===a.value;
  if(unchanged)return {...m,status:'noChange',pending:null};
  if(!m.faultUsed&&['saving','saveFailed','saveUnknown','conflict'].includes(m.scenario))return {...m,status:m.scenario,pending:a,faultUsed:true};
  return {...commit(m,a),status:m.scenario==='pending'?'pending':'saved',pending:null,revision:m.revision+1};
}
