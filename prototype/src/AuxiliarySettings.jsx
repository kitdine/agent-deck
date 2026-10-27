import {useEffect,useState} from 'react';
import {SETTING_SESSIONS,settingKey} from './auxiliarySettingsModel.js';
import './auxiliarySettings.css';
export function AuxiliarySettings({lang,model,dispatch}){
  const zh=lang==='zh',t=(a,b)=>zh?a:b;
  const names={work:t('工作','Work'),auxiliary:t('辅助','Auxiliary'),all:t('全部','All'),unknown:t('未知','Unknown'),auto:t('使用自动分类','Use automatic origin')};
  const [draft,setDraft]=useState(model.defaultScope),[client,setClient]=useState('claude'),[id,setId]=useState('old-01'),[target,setTarget]=useState(null),[choice,setChoice]=useState('work');
  useEffect(()=>setDraft(model.defaultScope),[model.defaultScope,model.revision]);
  useEffect(()=>{if(target){setChoice(model.overrides[settingKey(target)]??'auto');}},[model.revision]);
  const unreadable=['loading','loadFailed'].includes(model.status);
  const locked=unreadable||['saving','saveUnknown','conflict'].includes(model.status);
  const messages={ready:'',loading:t('正在读取设置，尚未确认默认值。','Reading settings; default not yet confirmed.'),loadFailed:t('设置读取失败，未显示推测默认值。','Settings could not be read. No default is assumed.'),saving:t('正在保存；以下仍为已确认值。关闭窗口不会取消已接受写入。','Saving; confirmed values remain below. Closing does not cancel an accepted write.'),saveFailed:t('未保存。已确认值不变；可重试。','Not saved. Confirmed values unchanged; retry available.'),saveUnknown:t('保存结果未知。先重新读取，禁止盲目重写。','Save outcome unknown. Read back before another write.'),conflict:t('设置已被其他操作更改。重新读取后再决定，不覆盖新值。','Settings changed elsewhere. Read back before deciding; do not overwrite.'),saved:t('已保存。现有菜单栏临时范围不变；下次打开使用默认。','Saved. Current temporary scope stays; next open uses the default.'),pending:t('已保存；视图更新待发布。Widget 可能仍显示旧快照。','Saved; view publication pending. Widget may still show an older snapshot.'),noChange:t('没有变化，未重复写入。','No change; no duplicate write.'),reloaded:t('已读取最新值。请重新确认要保存的选择。','Latest values loaded. Review your choice before saving.')};
  const lookup=()=>{const value=SETTING_SESSIONS.find(x=>x.client===client&&x.id===id.trim());setTarget(value||false);if(value)setChoice(model.overrides[settingKey(value)]??'auto');};
  const save=(action)=>dispatch({type:'save',action});
  const clearOverride=(key)=>{
    const buttons=Array.from(document.querySelectorAll('[data-as-clear]'));
    const index=buttons.findIndex(b=>b.dataset.asClear===key);
    save({type:'override',key,value:'auto'});
    requestAnimationFrame(()=>{
      const remaining=Array.from(document.querySelectorAll('[data-as-clear]'));
      (remaining.find(b=>b.dataset.asClear===key)||remaining[Math.min(index,remaining.length-1)]||document.querySelector('[data-as-find]'))?.focus();
    });
  };
  return <section className="settings-group as-settings" aria-labelledby="as-title" data-as-status={model.status}>
    <h2 id="as-title">{t('会话范围与分类','Session scope and origin')}</h2>
    <p id="as-description">{t('只隔离后台维护；任务子代理保留为工作。未知来源仍可见，总用量与费用完整保留。','Only background maintenance is isolated. Task agents stay in Work. Unknown origins remain visible; all usage and costs are retained.')}</p>
    <div className="as-status" role="status" aria-live="polite" aria-atomic="true">{messages[model.status]}</div>
    {model.pending&&<p className="as-pending">{t('待确认操作：','Pending operation: ')}{model.pending.type==='default'?t('默认范围','Default scope'):model.pending.key} → {names[model.pending.value]}</p>}
    {['loadFailed','saveUnknown','conflict'].includes(model.status)&&<button data-as-read type="button" onClick={()=>dispatch({type:'read'})}>{t('重新读取','Read latest values')}</button>}
    {model.status==='saveFailed'&&<button data-as-retry type="button" onClick={()=>dispatch({type:'retry'})}>{t('重试同一操作','Retry same operation')}</button>}
    <div className="as-block">
      <h3>{t('默认会话范围','Default session scope')}</h3>
      <p id="as-default-help">{t('用于下次打开菜单栏和 Widget。不会改写当前临时范围，也不改变 CLI 默认。','Used on next menu-bar open and by Widget. Does not replace the current temporary scope or CLI defaults.')}</p>
      <p>{t('已保存：','Saved: ')}<strong data-as-confirmed>{unreadable?t('不可用','Unavailable'):names[model.defaultScope]}</strong></p>
      <label>{t('选择默认范围','Choose default scope')}<select data-as-default value={unreadable?'':draft} disabled={locked} aria-describedby="as-default-help" onChange={e=>setDraft(e.target.value)}>{unreadable&&<option value="">{t('不可用','Unavailable')}</option>}{['work','auxiliary','all'].map(k=><option key={k} value={k}>{names[k]}{k==='work'?t('（含未知）',' (includes unknown)'):''}</option>)}</select></label>
      <button data-as-save-default type="button" disabled={locked} onClick={()=>save({type:'default',value:draft})}>{t('保存默认范围','Save default scope')}</button>
    </div>
    <div className="as-block">
      <h3>{t('单会话分类','Single-session origin')}</h3>
      <p id="as-origin-help">{t('按客户端与完整会话 ID 查找；仅修改这一会话，不按目录或项目批量操作。','Look up a client and complete session ID. Changes apply to this session only, never a directory or project.')}</p>
      <label>{t('客户端','Client')}<select data-as-client disabled={locked} value={client} onChange={e=>{setClient(e.target.value);setTarget(null);}}><option value="codex">Codex</option><option value="claude">Claude</option></select></label>
      <label>{t('完整会话 ID','Complete session ID')}<input data-as-id value={id} disabled={locked} aria-describedby="as-origin-help" onChange={e=>{setId(e.target.value);setTarget(null);}} /></label>
      <button data-as-find type="button" disabled={locked||!id.trim()} onClick={lookup}>{t('查找会话','Find session')}</button>
      {target===false&&<p role="status">{t('未找到可见会话。请检查客户端与 ID；不会绕过排除规则。','No visible session found. Check client and ID; exclusions are not bypassed.')}</p>}
      {target&&<div className="as-result" data-as-result>
        <h4>{target.client} · {target.id}</h4><p>{target.project}</p>
        <dl><dt>{t('有效分类','Effective origin')}</dt><dd data-as-effective>{names[model.overrides[settingKey(target)]??target.automatic]}</dd><dt>{t('自动分类','Automatic origin')}</dt><dd>{names[target.automatic]} · {target.reason} · origin-v1</dd><dt>{t('用户覆盖','User override')}</dt><dd>{model.overrides[settingKey(target)]?names[model.overrides[settingKey(target)]]:t('无','None')}</dd></dl>
        <label>{t('新的分类选择','New origin selection')}<select data-as-choice value={choice} disabled={locked} onChange={e=>setChoice(e.target.value)}>{['work','auxiliary','auto'].map(k=><option key={k} value={k}>{names[k]}</option>)}</select></label>
        <p>{t('选择“自动”只撤销覆盖，不删除会话或原始数据。','Automatic removes only the override; it does not delete the session or source data.')}</p>
        <button data-as-save-origin type="button" disabled={locked} onClick={()=>save({type:'override',key:settingKey(target),value:choice})}>{t('保存此会话分类','Save this session origin')}</button>
      </div>}
    </div>
    <div className="as-block"><h3>{t('现有覆盖','Existing overrides')}</h3>
      {unreadable?<p>{t('覆盖列表不可用','Override list unavailable')}</p>:Object.entries(model.overrides).length===0?<p>{t('没有用户覆盖','No user overrides')}</p>:Object.entries(model.overrides).map(([key,value])=>{
        const present=SETTING_SESSIONS.some(x=>settingKey(x)===key);
        return <div className="as-override" key={key}><b>{key}</b><span>{names[value]} · {present?t('已匹配会话','Matched session'):t('未匹配，未应用','Unmatched, not applied')}</span><button type="button" data-as-clear={key} aria-label={t('撤销覆盖：','Remove override: ')+key} disabled={locked} onClick={()=>clearOverride(key)}>{t('撤销此覆盖','Remove this override')}</button></div>;
      })}
    </div>
  </section>;
}
