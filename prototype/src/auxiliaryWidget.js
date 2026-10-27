// Aggregate-only synthetic projection; no session identities or source data.
export const AUX_WIDGET_STATES=['normal','empty','unknown','partial','legacy','defaultUnknown','pending','stale'];
export function auxiliaryWidgetProjection(period,scope,state){
  if(['legacy','defaultUnknown'].includes(state)||!['work','auxiliary','all'].includes(scope))return {available:false,scope:null,count:null,unknown:null,reason:state};
  const totals=period==='today'?{work:3,auxiliary:1,all:4}:{work:4,auxiliary:2,all:6};
  const count=state==='empty'?0:state==='unknown'?(scope==='auxiliary'?0:totals.all):totals[scope];
  const unknown=state==='empty'||scope==='auxiliary'?0:state==='unknown'?count:state==='partial'?null:1;
  return {available:true,scope,count,unknown,reason:state};
}
