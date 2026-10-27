import { useEffect, useRef, useState } from 'react';
import './auxiliarySessions.css';
import { meta } from './data.js';
import { syntheticSnapshotTime, formatSnapshotTime } from './auxiliaryMenuTime.js';
export const AUX_MENUBAR_STATES = ['normal','empty','unknown','partial','stale','loading','failed','unavailable','override','legacy','defaultUnknown','sourceUnavailable','inputMismatch'];
const fixtures = [
  {id:'task-01',client:'codex',project:'demo',kind:'work',minutes:30,days:0,calls:4,reason:'origin_missing',override:true,automatic:'unknown',revision:2},
  {id:'child-1',client:'codex',project:'demo',kind:'unknown',minutes:15,days:0,calls:2,reason:'task_child_unconfirmed'},
  {id:'old-01',client:'claude',project:'research',kind:'unknown',minutes:null,days:0,calls:null,reason:'origin_missing'},
  {id:'maint-1',client:'claude',project:'demo',kind:'auxiliary',minutes:10,days:0,calls:3,reason:'maintenance_origin'},
  {id:'task-02',client:'claude',project:'research',kind:'unknown',minutes:20,days:2,calls:2,reason:'origin_missing'},
  {id:'maint-2',client:'codex',project:'archive',kind:'auxiliary',minutes:5,days:15,calls:1,reason:'maintenance_origin'},
];
export function auxiliarySelection(client, period, scope, state) {
  const days = {today:1,'7d':7,'30d':30}[period];
  let data=fixtures.map(x=>({automatic:x.kind,override:false,revision:0,...x}));
  data=data.map(x=>state==='unknown'?{...x,kind:'unknown',automatic:'unknown',override:false,reason:'origin_missing'}:state==='override'&&x.id==='old-01'?{...x,kind:'work',override:true,revision:3}:x);
  if(['sourceUnavailable','inputMismatch'].includes(state))data=data.map(x=>({...x,kind:x.override?x.kind:'unknown',automatic:'unknown',reason:state==='sourceUnavailable'?'origin_source_unavailable':'origin_input_mismatch'}));
  if(state==='empty') data=[];
  return data.filter(x=>(client==='all'||client===x.client)&&x.days<days&&(scope==='all'||(scope==='work'?x.kind!=='auxiliary':x.kind==='auxiliary')));
}
export function auxiliaryMenuView(client,period,scope,state){
  if(['legacy','defaultUnknown'].includes(state))return null;
  const recent=auxiliarySelection(client,period,scope,state);
  const known=recent.filter(x=>x.minutes!==null);
  return {available:state!=='unavailable',selection:{client,period,scope},publication_id:`fixture-${state}-${client}-${period}-${scope}`,generated_at:syntheticSnapshotTime(meta.today),
    origin_coverage:{complete:!['partial','sourceUnavailable','inputMismatch'].includes(state),unknown_count:state==='partial'?null:recent.filter(x=>x.kind==='unknown').length},
    counts:{sessions:recent.length,projects:new Set(recent.map(x=>x.project)).size,known_duration_ms:recent.length&&known.length===0?null:known.reduce((n,x)=>n+x.minutes*60000,0),missing_duration_count:recent.length-known.length},recent,
    work_signals:{activity:{available:false},workflow:{available:false},tooling:{available:false}}};
}
export function originInspectCommand(item){
  const quote=value=>"'"+String(value).replace(/'/g,"'\\''")+"'";
  return `agentdeck session classify --id ${quote(item.id)} --client ${quote(item.client)}`;
}
export function AuxiliarySessions({lang,client,period,scope,onScope,scenario,onRetry}) {
  const zh=lang==='zh';const t=(a,b)=>zh?a:b;
  const labels={work:t('工作','Work'),auxiliary:t('辅助','Auxiliary'),all:t('全部','All'),unknown:t('未知','Unknown')};
  const [detail,setDetail]=useState(null);const [copied,setCopied]=useState(false);
  const origin=useRef(null);const back=useRef(null);const root=useRef(null);
  useEffect(()=>{setDetail(null);setCopied(false);},[client,period,scenario]);
  const state=scenario;
  const view=auxiliaryMenuView(client,period,scope,state);
  const data=view?.recent??[];
  const unknown=view?.origin_coverage.unknown_count;
  const minutes=view?.counts.known_duration_ms===null?null:(view?.counts.known_duration_ms??0)/60000;
  const projects=view?.counts.projects;
  const close=()=>{setDetail(null);setCopied(false);requestAnimationFrame(()=>{
    const selector=origin.current?.id?`[data-asc-id="${origin.current.id}"]`:`[data-asc-signal="${origin.current?.signal}"]`;
    (root.current?.querySelector(selector)||root.current?.querySelector('.asc-scope button'))?.focus();
  });};
  useEffect(()=>{if(detail)back.current?.focus();},[detail]);
  const choose=(value)=>{if(state==='loading'||state==='failed')return;onScope(value);setDetail(null);setCopied(false);};
  const notice={empty:t('此筛选下没有会话。可切换全部或调整时段。','No sessions in this selection. Try All or another period.'),unknown:t('来源尚未确认；保留在工作视图。','Origin is unconfirmed; retained in Work.'),partial:t('部分来源信息缺失；未知会话仍可见。','Some origin metadata is missing; unknown sessions remain visible.'),stale:t('显示上次成功数据，分类更新待发布。','Showing last successful data; classification update pending.'),loading:t('正在读取辅助范围；仍显示工作范围旧数据。','Loading Auxiliary; still showing previous Work data.'),failed:t('辅助范围读取失败；保留工作范围旧数据。','Auxiliary could not load; previous Work data retained.'),unavailable:t('会话数据不可用；无法报告数量。','Session data unavailable; counts cannot be reported.'),override:t('包含用户覆盖；自动分类记录保留。','Includes a user override; automatic classification retained.')};
  const reasonText={task_child_unconfirmed:t('任务子代理样例：用途未确认，以未知保留在工作视图','Task-agent fixture: purpose unconfirmed, retained in Work as Unknown'),origin_missing:t('来源信息缺失，不猜测用途','Origin metadata missing; purpose not inferred'),maintenance_origin:t('已识别的后台维护来源','Recognized background maintenance origin')};
  const open=(item)=>{origin.current=typeof item==='string'?{signal:item}:{id:item.id};setDetail(item);};
  if(!view)return <section className="panel asc-panel" data-asc-state="legacy" role="status"><p>{t('范围投影缺失；会话范围不可用。','Scope projection missing; session scope unavailable.')}</p><p>{t('旧的全部会话数字不能当作工作数量。请更新 AgentDeck 后刷新。','Legacy all-session totals cannot be used as Work counts. Update AgentDeck, then refresh.')}</p></section>;
  const extraNotice={sourceUnavailable:t('来源元数据不可用；未确认用途保留为未知，覆盖仍有效。','Origin metadata unavailable; unconfirmed origins stay Unknown. User overrides remain effective.'),inputMismatch:t('来源代次不匹配；过期分类不用于隐藏会话。','Source generation mismatch; stale origin does not hide sessions.')};
  const extraReason={origin_source_unavailable:t('来源元数据不可用','Origin source unavailable'),origin_input_mismatch:t('分类证据与来源代次不匹配','Origin evidence does not match source generation')};
  const unknownCopy=unknown===null?t('来源未知数量不可用。','Unknown-origin count unavailable.'):t(`包含 ${unknown} 个来源未知会话。`,`Includes ${unknown} sessions with unknown origin.`);
  return <section ref={root} className="panel asc-panel" data-asc-state={state} data-asc-scope={scope} data-asc-publication={view.publication_id} onKeyDown={e=>{if(e.key==='Escape'&&detail){e.stopPropagation();close();}}}>
    <div className="asc-scope" role="group" aria-label={t('会话范围','Session scope')} aria-describedby="asc-scope-note">
      {['work','auxiliary','all'].map(key=><button type="button" key={key} aria-pressed={scope===key} disabled={['loading','failed'].includes(state)} onClick={()=>choose(key)}>{labels[key]}</button>)}
    </div>
    <p id="asc-scope-note" className="asc-note">{t('仅影响会话与工作分析。总用量和费用保留全部。','Affects sessions and work analysis only. Usage and costs include all sessions.')}</p>
    <p className="asc-note" data-asc-selection>{labels[scope]} · {client==='all'?t('全部客户端','All clients'):client} · {period==='today'?t('今天','Today'):period}</p>
    <div role="status" aria-live="polite" aria-atomic="true" className="asc-notice">{extraNotice[state]||notice[state]||unknownCopy}</div>
    {view.available&&(extraNotice[state]||notice[state])&&<p className="asc-note" data-asc-unknown>{unknownCopy}</p>}
    {['stale','loading','failed'].includes(state)&&<p className="asc-note" data-asc-generated-at={view.generated_at}>{t('会话快照：','Session snapshot: ')}{formatSnapshotTime(view.generated_at,lang)}</p>}
    {state==='failed'&&<button type="button" onClick={onRetry}>{t('重试辅助范围','Retry Auxiliary')}</button>}
    {state==='unavailable'?<p className="asc-note">{t('可通过顶部刷新重试；刷新遵循原有扫描行为。','Retry using Refresh above; existing scan behavior is unchanged.')}</p>:detail?<>
      <button type="button" ref={back} onClick={close}>{t('返回会话','Back to sessions')}</button>
      {typeof detail==='string'?<div className="card asc-detail"><h3>{detail}</h3><p>{t('范围与会话列表一致。','Same scope as the session list.')}</p><p>{t('本标本未提供该信号的可靠测量值，不补零。','No reliable measurement for this signal in this specimen; not zero.')}</p><strong>{t('不可用','Unavailable')}</strong></div>:<div className="card asc-detail">
        <h3>{detail.id}</h3><p>{detail.client} · {detail.project}</p><dl><dt>{t('有效分类','Effective origin')}</dt><dd>{labels[detail.kind]}{detail.override?t('（用户覆盖）',' (user override)'):''}</dd><dt>{t('自动分类','Automatic origin')}</dt><dd>{labels[detail.automatic]}</dd><dt>{t('原因','Reason')}</dt><dd>{extraReason[detail.reason]||reasonText[detail.reason]}</dd><dt>{t('规则版本','Rule version')}</dt><dd>{state==='sourceUnavailable'?t('不可用','Unavailable'):'origin-v1'}</dd><dt>{t('覆盖版本','Override revision')}</dt><dd>{detail.revision}</dd></dl>
        <p>{t('查看与修正分类可用 CLI；此面板只读。','Use the CLI to inspect or correct origin; this panel is read only.')}</p>
        <code className="asc-command">{originInspectCommand(detail)}</code>
        <button type="button" onClick={async()=>{try{await navigator.clipboard.writeText(originInspectCommand(detail));setCopied('ok');}catch{setCopied('failed');}}}>{t('复制查询命令','Copy inspect command')}</button>
        <p role="status">{copied==='ok'?t('已复制','Copied'):copied==='failed'?t('复制失败，请选择上方命令手动复制。','Copy failed; select the command above to copy manually.'):''}</p>
      </div>}
    </>:<>
      <div className="asc-stats" data-asc-stats><div><span>{t('会话','Sessions')}</span><strong data-asc-count>{view.counts.sessions}</strong></div><div><span>{t('项目','Projects')}</span><strong data-asc-projects>{projects}</strong></div><div><span>{t('已知时长','Known duration')}</span><strong data-asc-minutes>{minutes===null?'—':`${minutes} min`}</strong></div></div>
      {view.counts.missing_duration_count>0&&<p className="asc-note">{t(`${view.counts.missing_duration_count} 个会话时长未知，不计为零。`,`${view.counts.missing_duration_count} session durations unknown, not zero.`)}</p>}
      <div className="card asc-list">{data.length?data.map(item=><button type="button" data-asc-id={item.id} key={item.id} onClick={e=>open(item,e)}><span><b>{item.project}</b><small>{item.client} · {item.id}</small></span><span>{labels[item.kind]}{item.override?t('（覆盖）',' (override)'):''}</span></button>):<p>{t('没有匹配会话','No matching sessions')}</p>}</div>
      <p className="asc-note">{t('项目数按所选集合去重，工作与辅助项目可能重合。','Projects are distinct within the selection; Work and Auxiliary may overlap.')}</p>
      <div className="asc-signals">{[t('活动','Activity'),t('工作流','Workflow'),t('工具','Tooling')].map(name=><button type="button" key={name} data-asc-signal={name} onClick={()=>open(name)}>{name} · {t('不可用','Unavailable')} ›</button>)}</div>
    </>}
  </section>;
}
