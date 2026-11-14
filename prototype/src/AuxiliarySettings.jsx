import {useEffect,useLayoutEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {CheckCircle,CircleNotch,Info,Warning,WarningCircle} from '@phosphor-icons/react';
import {SETTING_SESSIONS,settingKey,targetRevision} from './auxiliarySettingsModel.js';
import './auxiliarySettings.css';

// 「会话」设置组，2026-11-07 重新设计（旧版见提交 2c90d4d）。
// 它必须读起来像同一个窗口里的另一组设置，而不是嵌进来的一张网页表单：
// 同样的「左标题与说明、右控件」行，同样的分段控件，选中即保存。
// 查找单个会话和覆盖列表属于偶尔才做的维护，放进「手动分类」sheet，
// 主列表只留一行摘要。版本号、请求回执、reason code 这些协议字段
// 只在确实需要用户采取行动时出现，平时不占位置。
// 状态机仍是 auxiliarySettingsModel.js：一次只接受一个写操作，
// 失败/未知/冲突各自有出路，关闭窗口不会取消已被接受的写入。

const LOCKING=['saving','saveUnknown','conflict'];
const UNREADABLE=['loading','loadFailed'];
// 外部适配器不可用（core 仍可读）。core 不可读是另一种失效，见 coreUnavailable。
const SOURCE_DOWN=['sourceUnconfigured','sourceUnavailable','sourceUnsupported'];
const FOCUSABLE='button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),a[href],[tabindex]:not([tabindex="-1"])';
const CLIENTS={codex:'Codex',claude:'Claude'};
const splitKey=key=>{const at=key.indexOf(':');return [key.slice(0,at),key.slice(at+1)];};
const ICONS={good:CheckCircle,info:Info,warn:Warning,bad:WarningCircle,busy:CircleNotch};

// 分段控件。与 Settings.jsx 的 Segmented 同一外观，多了两件事：
// 读不到已保存值时一个都不选中（不能显示一个猜出来的「工作」）；
// 写入进行中时，目标选项带一个脉冲点，选中态仍停在已确认的值上。
function Choice({label,describedBy,options,value,pending,disabled,onPick,attr}){
  return <div className="settings-segmented as-segmented" role="radiogroup" aria-label={label} aria-describedby={describedBy}>
    {options.map(([key,text])=>{
      const checked=value===key,busy=pending===key;
      return <button type="button" role="radio" key={key} aria-checked={checked} aria-busy={busy||undefined} disabled={disabled}
        className={`${checked?'active':''}${busy?' pending':''}`} {...{[attr]:key}} onClick={()=>{if(!checked)onPick(key);}}>
        {text}{busy&&<i className="as-pending-dot" aria-hidden="true"/>}
      </button>;
    })}
  </div>;
}

// live region 必须先存在、再被填入内容才会被朗读，所以容器常驻，空时不占高度。
// 操作按钮放在 region 外面，保持各自可以被单独聚焦和激活。
function StatusArea({status,warning,attr}){
  const Icon=status&&ICONS[status.tone];
  return <>
    <div role="status" aria-live="polite" aria-atomic="true" {...{[attr]:status?status.tone:''}}>
      {status&&<small className={`as-status as-is-${status.tone}`}>
        {Icon&&<Icon size={12} weight={status.tone==='busy'?'bold':'fill'} className={status.tone==='busy'?'as-spin':undefined} aria-hidden="true"/>}
        <span>{status.text}{status.request&&<> <span className="as-request">{status.requestLabel} <code data-as-request>{status.request}</code></span></>}</span>
      </small>}
      {warning&&<small className="as-status as-is-warn" data-as-warning><Warning size={12} weight="fill" aria-hidden="true"/><span>{warning}</span></small>}
    </div>
    {status?.actions?.length>0&&<span className="as-actions">{status.actions.map(a=><button type="button" className="as-link" key={a.attr} {...{[a.attr]:''}} onClick={a.onClick}>{a.label}</button>)}</span>}
  </>;
}

export function AuxiliarySettings({lang,model,dispatch,hidden=false}){
  const zh=lang==='zh',t=(a,b)=>zh?a:b;
  const names={work:t('工作','Work'),auxiliary:t('辅助','Auxiliary'),all:t('全部','All'),unknown:t('未知','Unknown'),auto:t('自动','Automatic')};
  const rootRef=useRef(null),manageRef=useRef(null),idRef=useRef(null),sheetRef=useRef(null),clearFocus=useRef(null);
  const [host,setHost]=useState(null),[sheet,setSheet]=useState(false);
  const [client,setClient]=useState('codex'),[id,setId]=useState(''),[target,setTarget]=useState(null);

  const unreadable=UNREADABLE.includes(model.status);
  const locked=unreadable||LOCKING.includes(model.status);
  const op=model.pending?.type??model.lastOperation;
  // core 不可读时默认范围、覆盖和自动识别结果一项也读不到（标本里与 loadFailed 同时发生）；
  // 重新读取成功后，全部按读回结果显示。外部来源不可用只影响自动识别，手动分类照常。
  const coreUnavailable=model.status==='loadFailed';
  const sourceAvailable=!coreUnavailable&&!SOURCE_DOWN.includes(model.scenario);
  const read=()=>dispatch({type:'read'}),retry=()=>dispatch({type:'retry'});
  const save=action=>dispatch({type:'save',action:{...action,expected_revision:targetRevision(model,action)}});

  // 每个写操作的状态只出现在发起它的那个控件下面；另一个控件只说明自己为什么暂时不能用。
  const describe=scope=>{
    const s=model.status,p=model.pending;
    if(s==='loading')return scope==='default'?{tone:'busy',text:t('正在读取已保存的范围…','Reading the saved scope…')}:null;
    if(s==='loadFailed')return scope==='default'?{tone:'bad',text:t('无法读取已保存的范围，不会假定为「工作」。','The saved scope could not be read. Work is not assumed.'),actions:[{label:t('重新读取','Read again'),attr:'data-as-read',onClick:read}]}:null;
    if(op!==scope)return LOCKING.includes(s)?{tone:'dim',text:scope==='default'?t('一项手动分类的修改尚未完成，完成后可更改。','A classification change is still in progress.'):t('默认范围的修改尚未完成，完成后可更改。','A default scope change is still in progress.')}:null;
    const sid=p?.key?splitKey(p.key)[1]:'';
    const clearing=p?.type==='override'&&p.value==='auto';
    switch(s){
      case 'saving':return {tone:'busy',text:scope==='default'?t(`正在保存为「${names[p.value]}」…`,`Saving as ${names[p.value]}…`):clearing?t(`正在移除 ${sid} 的手动分类…`,`Removing the manual classification of ${sid}…`):t(`正在把 ${sid} 设为「${names[p.value]}」…`,`Setting ${sid} to ${names[p.value]}…`)};
      case 'saveFailed':return {tone:'bad',text:scope==='default'?t(`未能保存，仍为「${names[model.defaultScope]}」。`,`Not saved. Still ${names[model.defaultScope]}.`):t('未能保存，分类保持不变。','Not saved. The classification is unchanged.'),actions:[{label:t('重试','Retry'),attr:'data-as-retry',onClick:retry}]};
      case 'saveUnknown':{
        const still=model.receipt?.outcome==='unknown';
        return {tone:'warn',request:p?.request_id,requestLabel:t('请求','Request'),
          text:still?t('仍无法确认是否已保存。','Still unable to confirm whether it was saved.'):t('无法确认是否已保存，确认前暂停修改。','The save could not be confirmed. Changes are paused until it is.'),
          actions:still?[{label:t('再次查询','Check again'),attr:'data-as-read',onClick:read},{label:t('按原请求重试','Retry the original request'),attr:'data-as-retry',onClick:retry}]:[{label:t('查询结果','Check result'),attr:'data-as-read',onClick:read}]};
      }
      case 'conflict':return {tone:'warn',text:t('已在别处被修改，没有覆盖。','Changed elsewhere. Nothing was overwritten.'),actions:[{label:t('读取最新值','Load latest'),attr:'data-as-read',onClick:read}]};
      case 'reloaded':return {tone:'info',text:scope==='default'?t(`已读取最新值「${names[model.defaultScope]}」，如需更改请重新选择。`,`Loaded the latest value, ${names[model.defaultScope]}. Choose again to change it.`):t('已读取最新状态，如需更改请重新选择。','Loaded the latest state. Choose again to change it.')};
      case 'saved':return {tone:'good',text:scope==='default'?t('已保存。已打开的菜单栏保持当前范围，下次打开时生效。','Saved. An open menu bar keeps its scope; the next open uses this default.'):model.receipt?.operation==='clear'?t('已移除手动分类，恢复自动识别。','Manual classification removed. Automatic detection applies.'):t('已保存。会话视图会在下次更新时反映。','Saved. Session views reflect it on their next update.')};
      case 'pending':return {tone:'info',text:scope==='default'?t('已保存。Widget 会在系统下次刷新时更新。','Saved. The Widget updates on its next system refresh.'):t('已保存。会话视图尚未更新，列表可能暂时是旧的。','Saved. Session views have not been updated yet and may be briefly out of date.')};
      case 'noChange':return {tone:'dim',text:t('没有变化。','No change.')};
      case 'replayed':return {tone:'info',text:t('这个请求此前已经完成；这里显示的是最新状态。','That request had already completed. The latest state is shown.')};
      case 'invalidRequest':return {tone:'bad',text:t('请求 ID 与内容不一致，没有写入。','The request ID does not match its contents. Nothing was written.')};
      default:return null;
    }
  };
  const warningFor=scope=>op===scope&&model.warnings.includes('state_permissions_warning')
    ?t('已保存，但状态文件的权限加固失败。可运行 agentdeck doctor 检查，已保存的值不会回滚。','Saved, but hardening the state file permissions failed. Run agentdeck doctor to check; the saved value is kept.'):null;

  // ---- 单会话查找：输入一变就丢掉旧结果，旧卡片上的控件不能作用到新输入上。
  const targetKey=target?settingKey(target):null;
  const classificationAvailable=!!target&&model.inspected[targetKey]===(model.versions[targetKey]??0);
  const lookup=(c=client,value=id)=>{
    const found=SETTING_SESSIONS.find(x=>x.client===c&&x.id===value.trim());
    setTarget(found||false);
    if(found)dispatch({type:'inspect',key:settingKey(found)});
  };
  // 撤销回执不带当前分类。卡片仍在显示时，立刻另发一次只读查询把它补上，
  // 而不是让人再去按一个「查询当前分类」。
  useEffect(()=>{if(target&&!locked&&!classificationAvailable)dispatch({type:'inspect',key:targetKey});},[targetKey,locked,classificationAvailable]);

  const clearOverride=key=>{
    const buttons=Array.from(document.querySelectorAll('[data-as-clear]'));
    clearFocus.current={key,index:buttons.findIndex(b=>b.dataset.asClear===key)};
    save({type:'override',key,value:'auto'});
  };
  // 移除一行后焦点落到下一行的「移除」；没有下一行就回到 ID 输入框。失败时留在原处。
  useEffect(()=>{
    if(!clearFocus.current||LOCKING.includes(model.status))return;
    const {key,index}=clearFocus.current;
    const frame=requestAnimationFrame(()=>{
      const remaining=Array.from(document.querySelectorAll('[data-as-clear]'));
      (remaining.find(b=>b.dataset.asClear===key)||remaining[Math.min(index,remaining.length-1)]||idRef.current)?.focus();
      if(model.status!=='saveFailed')clearFocus.current=null;
    });
    return()=>cancelAnimationFrame(frame);
  },[model.revision,model.status]);

  // ---- sheet：挂在设置窗口上而不是设置组里，打开时窗口正文 inert，Escape 只关 sheet。
  useLayoutEffect(()=>{setHost(rootRef.current?.closest('.settings-window')??null);},[]);
  // 分页窗口里工具栏也一起 inert：sheet 打开时不能切到别的页。sheet 从正文顶部落下，分页时即工具栏下方。
  useEffect(()=>{
    const blocked=[host?.querySelector('.settings-body'),host?.querySelector('.settings-tabs')].filter(Boolean);
    if(!blocked.length)return;
    blocked.forEach(node=>{node.inert=sheet;});
    if(sheet)requestAnimationFrame(()=>(idRef.current&&!idRef.current.disabled?idRef.current:sheetRef.current)?.focus());
    return()=>{blocked.forEach(node=>{node.inert=false;});};
  },[host,sheet]);
  // 模态焦点范围：Tab / Shift+Tab 在 sheet 内首尾循环，不落到 inert 的正文、工具栏或页面上。
  // 监听挂在 document 上，这样焦点因控件被禁用而掉到 body 时，下一次 Tab 也回到 sheet 里。
  // 窗口外的原型舞台控件不属于设置窗口，保持浏览器默认的 Tab 行为。
  useEffect(()=>{
    if(!sheet)return;
    const onKey=e=>{
      const node=sheetRef.current,active=document.activeElement;
      if(e.key!=='Tab'||!node)return;
      const inside=node.contains(active);
      if(!inside&&active&&active!==document.body&&!host?.contains(active))return;
      const items=[...node.querySelectorAll(FOCUSABLE)].filter(x=>x.getClientRects().length>0);
      e.preventDefault();
      if(!items.length){node.focus();return;}
      const first=items[0],last=items[items.length-1];
      const at=items.indexOf(active);
      if(at<0)(e.shiftKey?last:first).focus();
      else items[(at+(e.shiftKey?-1:1)+items.length)%items.length].focus();
    };
    document.addEventListener('keydown',onKey);
    return()=>document.removeEventListener('keydown',onKey);
  },[host,sheet]);
  const sheetTop=host?.querySelector('.settings-body')?.offsetTop??38;
  const closeSheet=()=>{setSheet(false);requestAnimationFrame(()=>manageRef.current?.focus());};

  const entries=Object.entries(model.overrides).sort(([a],[b])=>a.localeCompare(b));
  const unmatched=entries.filter(([key])=>!SETTING_SESSIONS.some(x=>settingKey(x)===key)).length;
  const summary=unreadable?t('覆盖列表暂不可读','Override list unavailable')
    :entries.length===0?t('纠正自动识别有误的单个会话','Correct individual sessions that were detected wrongly')
    :t(`已手动分类 ${entries.length} 个会话`,`${entries.length} sessions classified manually`)+(unmatched?t(`，${unmatched} 个未生效`,`, ${unmatched} not applied`):'');

  // ---- 识别来源：一行一个来源，圆点表态，文字说人话；reason code 只放在 title 里。
  const memSource=coreUnavailable?{tone:'bad',text:t('本机用量库不可读','Local usage database unreadable'),code:'core_unavailable'}
    :{sourceUnconfigured:{tone:'dim',text:t('未配置 · 可通过 CLI 设置','Not configured · set up with the CLI'),code:'source_not_configured'},
      sourceUnavailable:{tone:'warn',text:t('无法读取元数据','Metadata unreadable'),code:'source_unavailable'},
      sourceUnsupported:{tone:'warn',text:t('版本暂不支持','Version not supported'),code:'source_version_unsupported'}}[model.scenario]
    ??{tone:'good',text:t('可用 · 15.2.1','Available · 15.2.1'),code:'available'};
  const codexState=coreUnavailable?'coreUnavailable':{codexReceiverOff:'receiverOff',codexNoRecords:'noRecords',codexReceiverUnknown:'receiverUnknown'}[model.scenario]||'records';
  const codexSource={
    records:{tone:'good',text:t('1 条后台会话记录','1 background session record'),code:'available'},
    receiverOff:{tone:'dim',text:t('无记录 · OTel 采集未开启（见「实时用量」）','No records · OTel capture is off (see Live usage)'),code:'otel_receiver_disabled'},
    noRecords:{tone:'dim',text:t('无记录 · 采集已开启，尚无后台会话','No records · capture is on, no background session yet'),code:'no_background_sessions'},
    receiverUnknown:{tone:'dim',text:t('无记录 · 采集状态未知','No records · capture state unknown'),code:'otel_receiver_unknown'},
    coreUnavailable:{tone:'bad',text:t('本机用量库不可读','Local usage database unreadable'),code:'core_unavailable'},
  }[codexState];

  // 脉冲点只表示「写入在途或结果未定」；失败与冲突时请求已经结束，不再显示。
  const inFlight=['saving','saveUnknown'].includes(model.status);
  const defaultPending=inFlight&&model.pending?.type==='default'?model.pending.value:null;
  const overridePending=inFlight&&model.pending?.type==='override'?model.pending:null;

  const card=target&&(()=>{
    const override=model.overrides[targetKey];
    const automatic=sourceAvailable?target.automatic:'unknown';
    const reasonCode=sourceAvailable?target.reason:'origin_source_unavailable';
    const reason={maintenance_origin:t('claude-mem 后台维护','claude-mem background maintenance'),origin_metadata_missing:t('缺少来源信息','origin metadata missing'),origin_source_unavailable:t('识别来源不可用','detection source unavailable')}[reasonCode];
    const effective=override??automatic;
    const reading=t('正在读取…','Reading…');
    return <div className="as-card" data-as-result>
      <div className="as-card-head">
        <div><b>{target.project}</b><code className="as-id">{target.id}</code></div>
        <span className="as-client-tag">{CLIENTS[target.client]}</span>
      </div>
      <div className="as-card-choice">
        <span id="as-choice-label">{t('分类','Classification')}</span>
        <Choice label={t('这个会话的分类','Classification for this session')} describedBy="as-choice-note" attr="data-as-choice-option"
          options={[['auto',names.auto],['work',names.work],['auxiliary',names.auxiliary]]}
          value={override??'auto'} pending={overridePending?.key===targetKey?overridePending.value:null} disabled={locked}
          onPick={value=>save({type:'override',key:targetKey,value})}/>
      </div>
      <dl>
        <dt>{t('当前计入','Counts as')}</dt>
        <dd data-as-effective>{classificationAvailable?<>{names[effective]}<span className="as-dim"> · {override?t('手动','manual'):t('自动识别','detected')}{effective==='unknown'?t('，归入「工作」范围',', included in Work'):''}</span></>:reading}</dd>
        <dt>{t('自动识别','Detected')}</dt>
        <dd data-as-automatic title={`${reasonCode} · origin-v1`}>{classificationAvailable?<>{names[automatic]}<span className="as-dim"> · {reason}</span></>:reading}</dd>
      </dl>
      <p className="as-card-note" id="as-choice-note">{t('选「自动」只撤销手动分类，不删除会话。','Automatic only removes the manual classification; the session is not deleted.')}</p>
    </div>;
  })();

  const sheetNode=sheet&&host&&createPortal(
    <div className="as-scrim" style={{top:sheetTop}} onMouseDown={e=>{if(e.target===e.currentTarget)closeSheet();}}>
      <div className="as-sheet" role="dialog" aria-modal="true" aria-labelledby="as-sheet-title" aria-describedby="as-sheet-desc" data-as-sheet tabIndex={-1} ref={sheetRef}
        onKeyDown={e=>{if(e.key==='Escape'){e.stopPropagation();closeSheet();}}}>
        <header className="as-sheet-head">
          <h3 id="as-sheet-title">{t('手动分类','Manual classification')}</h3>
          <p id="as-sheet-desc">{t('自动识别有误时，把单个会话改为工作或辅助。只影响会话统计，不删除会话，也不改动原始日志。','When automatic detection is wrong, set a single session to Work or Auxiliary. Only session statistics change; sessions and source logs are never deleted or modified.')}</p>
        </header>
        <div className="as-sheet-body">
          <div className="as-banner"><StatusArea status={coreUnavailable?{tone:'bad',text:t('本机用量库暂不可读，暂时不能查找或修改手动分类。','The local usage database cannot be read right now, so sessions cannot be found or reclassified.')}:describe('override')} warning={warningFor('override')} attr="data-as-banner"/></div>
          <section aria-labelledby="as-lookup-title">
            <h4 className="as-section-title" id="as-lookup-title">{t('查找会话','Find a session')}</h4>
            <div className="as-lookup-bar">
              <Choice label={t('客户端','Client')} attr="data-as-client-option" options={Object.entries(CLIENTS)} value={client} disabled={locked}
                onPick={value=>{setClient(value);setTarget(null);}}/>
              <input ref={idRef} data-as-id className="as-input" value={id} disabled={locked} spellCheck={false} autoComplete="off"
                placeholder={t('完整会话 ID','Complete session ID')} aria-label={t('完整会话 ID','Complete session ID')} aria-describedby="as-lookup-help"
                onChange={e=>{setId(e.target.value);setTarget(null);}}
                onKeyDown={e=>{if(e.key==='Enter'&&id.trim()&&!locked){e.preventDefault();lookup();}}}/>
              <button type="button" className="as-button" data-as-find disabled={locked||!id.trim()} onClick={()=>lookup()}>{t('查找','Find')}</button>
            </div>
            <p className="as-help" id="as-lookup-help">{t('完整 ID 可在菜单栏的会话详情或 ','Copy the complete ID from the menu bar session details or ')}<code>agentdeck session list</code>{t(' 中复制。','.')}</p>
            {target===false&&<p className="as-notfound" role="status" data-as-notfound><Info size={13} weight="fill" aria-hidden="true"/>{t('没有找到可见的会话。请确认客户端与完整 ID。','No visible session found. Check the client and the complete ID.')}</p>}
            {card}
          </section>
          <section aria-labelledby="as-overrides-title">
            <h4 className="as-section-title" id="as-overrides-title">{t('已手动分类','Manually classified')}{!unreadable&&entries.length>0&&<span className="as-count">{entries.length}</span>}</h4>
            {unreadable?<div className="as-empty" data-as-list-unavailable><span>{t('列表暂不可读，不会当作没有覆盖。','The list cannot be read right now; it is not treated as empty.')}</span>{model.status==='loadFailed'&&<button type="button" className="as-link" data-as-read-list onClick={read}>{t('重新读取','Read again')}</button>}</div>
              :entries.length===0?<p className="as-empty" data-as-empty>{t('还没有手动分类的会话。','No sessions are classified manually yet.')}</p>
              :<ul className="as-list">{entries.map(([key,value])=>{
                const [c,sid]=splitKey(key);
                const matched=SETTING_SESSIONS.some(x=>settingKey(x)===key);
                const removing=overridePending?.key===key&&overridePending.value==='auto';
                const ident=<><span className="as-client-tag">{CLIENTS[c]??c}</span><code className="as-id">{sid}</code></>;
                return <li className="as-row" key={key} data-as-row={key} data-matched={matched}>
                  {matched
                    ?<button type="button" className="as-row-main" data-as-open-row={key} disabled={locked} aria-label={t(`查看 ${CLIENTS[c]} ${sid}`,`Show ${CLIENTS[c]} ${sid}`)} onClick={()=>{setClient(c);setId(sid);lookup(c,sid);}}>{ident}</button>
                    :<div className="as-row-main">{ident}<small><Warning size={11} weight="fill" aria-hidden="true"/>{t('未匹配到会话，未生效','No matching session; not applied')}</small></div>}
                  <span className="as-tag" data-kind={value}>{names[value]}</span>
                  <button type="button" className="as-link as-remove" data-as-clear={key} disabled={locked} aria-label={t(`移除 ${CLIENTS[c]??c} ${sid} 的手动分类`,`Remove the manual classification of ${CLIENTS[c]??c} ${sid}`)} onClick={()=>clearOverride(key)}>
                    {removing?<CircleNotch size={12} weight="bold" className="as-spin" aria-hidden="true"/>:t('移除','Remove')}
                  </button>
                </li>;
              })}</ul>}
          </section>
        </div>
        <footer className="as-sheet-foot"><button type="button" className="as-button primary" data-as-sheet-done onClick={closeSheet}>{t('完成','Done')}</button></footer>
      </div>
    </div>,host);

  return <div className="settings-group as-settings" data-group="sessions" data-as-status={model.status} ref={rootRef} hidden={hidden}>
    <span className="settings-group-title" id="as-title">{t('会话','Sessions')}</span>

    <div className="settings-field">
      <div className="settings-label">
        <span>{t('默认会话范围','Default session scope')}</span>
        <small id="as-default-help">{t('菜单栏「会话」页与 Widget 下次打开时使用；「工作」含来源未确认的会话。','Used when the menu bar Sessions tab or the Widget next opens. Work includes sessions of unconfirmed origin.')}</small>
        <StatusArea status={describe('default')} warning={warningFor('default')} attr="data-as-default-status"/>
      </div>
      <div className="settings-control">
        <Choice label={t('默认会话范围','Default session scope')} describedBy="as-default-help" attr="data-as-default-option"
          options={[['work',names.work],['auxiliary',names.auxiliary],['all',names.all]]}
          value={unreadable?null:model.defaultScope} pending={defaultPending} disabled={locked}
          onPick={value=>save({type:'default',value})}/>
      </div>
    </div>

    <div className="settings-field">
      <div className="settings-label">
        <span>{t('手动分类','Manual classification')}</span>
        <small id="as-manual-summary" data-as-summary>{summary}</small>
        {!sheet&&<StatusArea status={['saved','noChange','replayed','reloaded'].includes(model.status)?null:describe('override')} warning={warningFor('override')} attr="data-as-manual-status"/>}
      </div>
      <div className="settings-control">
        <button type="button" className="as-button" ref={manageRef} data-as-manage aria-haspopup="dialog" aria-describedby="as-manual-summary" onClick={()=>setSheet(true)}>{t('管理…','Manage…')}</button>
      </div>
    </div>

    <div className="settings-field">
      <div className="settings-label">
        <span>{t('自动识别','Automatic detection')}</span>
        <small id="as-detect-help">{t('只把后台维护会话识别为辅助；任务子代理始终算作工作。','Only background maintenance is detected as Auxiliary; task sub-agents always count as Work.')}</small>
        <div className="as-sources" data-as-source>
          <div className="as-source-row" data-as-source-mem data-state={memSource.code} title={memSource.code}>
            <i className={`as-dot as-is-${memSource.tone}`} aria-hidden="true"/><span>claude-mem</span><small className={`as-is-${memSource.tone}`}>{memSource.text}</small>
          </div>
          <div className="as-source-row" data-as-source-codex data-state={codexState} title={codexSource.code}>
            <i className={`as-dot as-is-${codexSource.tone}`} aria-hidden="true"/><span>{t('Codex 后台','Codex background')}</span><small className={`as-is-${codexSource.tone}`}>{codexSource.text}</small>
          </div>
        </div>
        {coreUnavailable
          ?<small className="as-source-note" data-as-source-note="core">{t('本机用量库暂不可读：当前分类和手动分类都无法读取，暂时不能修改。已保存的手动分类不会被删除。','The local usage database cannot be read right now: current and manual classifications are unavailable and cannot be changed. Saved manual classifications are not deleted.')} <button type="button" className="as-link" data-as-read-core onClick={read}>{t('重新读取','Read again')}</button></small>
          :!sourceAvailable&&<small className="as-source-note" data-as-source-note="adapter">{t('来源不可用时，自动识别结果为「未知」并计入工作；手动分类不受影响。','While a source is unavailable, detection reports Unknown, which counts as Work. Manual classification still works.')}</small>}
      </div>
    </div>
    {sheetNode}
  </div>;
}
