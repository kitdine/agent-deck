// Aggregate-only synthetic projection; no session identities or source data.
export const AUX_WIDGET_STATES=['normal','empty','unknown','partial','legacy','defaultUnknown','pending','stale','malformed','selectionMismatch','unknownScope'];
// Proposed aggregate DTO decoder specimen; not the production Swift codec.
export function decodeSessionScopeSummary(summary,selection){
  const unavailable=reason=>({available:false,scope:null,count:null,unknown:null,reason});
  if(!summary||summary.available!==true)return unavailable('summary_unavailable');
  if(!['work','auxiliary','all'].includes(summary.scope))return unavailable('unsupported_scope');
  if(summary.client!==selection.client||summary.period!==selection.period)return unavailable('selection_mismatch');
  if(typeof summary.publication_id!=='string'||!summary.publication_id||typeof summary.generated_at!=='string'||!Number.isFinite(Date.parse(summary.generated_at)))return unavailable('incomplete_summary');
  if(!Number.isSafeInteger(summary.count)||summary.count<0||!(summary.unknown_count===null||Number.isSafeInteger(summary.unknown_count)&&summary.unknown_count>=0&&summary.unknown_count<=summary.count))return unavailable('invalid_counts');
  if(summary.scope==='auxiliary'&&summary.unknown_count!==0)return unavailable('invalid_counts');
  return {available:true,scope:summary.scope,count:summary.count,unknown:summary.unknown_count,reason:summary.reason??null};
}
export function auxiliaryWidgetProjection(period,scope,state){
  if(['legacy','defaultUnknown'].includes(state)||!['work','auxiliary','all'].includes(scope))return {available:false,scope:null,count:null,unknown:null,reason:state};
  const totals=period==='today'?{work:3,auxiliary:1,all:4}:{work:4,auxiliary:2,all:6};
  const count=state==='empty'?0:state==='unknown'?(scope==='auxiliary'?0:totals.all):totals[scope];
  const unknown=state==='empty'||scope==='auxiliary'?0:state==='unknown'?count:state==='partial'?null:1;
  const summary={available:true,scope,count,unknown_count:unknown,client:'all',period,publication_id:'fixture-publication-1',generated_at:'2026-08-18T14:30:00Z',reason:state};
  if(state==='malformed')delete summary.unknown_count;
  if(state==='selectionMismatch')summary.period=period==='today'?'30d':'today';
  if(state==='unknownScope')summary.scope='future-scope';
  return decodeSessionScopeSummary(summary,{client:'all',period});
}
