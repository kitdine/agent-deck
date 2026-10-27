(async()=>{
const failures=[];let checks=0;const tick=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
const $=s=>document.querySelector(s),test=(n,v)=>{checks++;if(!v)failures.push(n);};
const select=async(s,v)=>{$(s).value=v;$(s).dispatchEvent(new Event('change',{bubbles:true}));await tick();};
const click=async s=>{$(s).click();await tick();};
const scene=s=>select('[data-as-stage]',s);const status=()=>$('.as-settings').dataset.asStatus;
const reopen=async()=>{await click('.settings-window .close');await click('[data-as-open]');};
for(const s of ['normal','loading','loadFailed','saving','saveFailed','saveUnknown','conflict','pending']){
 await scene(s);test(s+' geometry',[...document.querySelectorAll('.as-settings *')].filter(x=>x.clientWidth&&x.scrollWidth>x.clientWidth+1).length===0);
 if(s==='loading'||s==='loadFailed'){
  test(s+' unavailable',$('[data-as-default]').value===''&&$('[data-as-save-default]').disabled);
  await click('.settings-window .close');await click('.menubar-item');await click('.menubar-item');
  test(s+' reopened consumer blocks unconfirmed default',!!$('[data-aux-default-unavailable]')&&!$('[data-asc-count]')&&!$('.asc-scope'));
  await click('[data-tab="usage"]');await click('[data-tab="sessions"]');
  test(s+' tab roundtrip still unavailable',!!$('[data-aux-default-unavailable]')&&!$('[data-asc-count]'));
  await click('[data-as-open]');
  if(s==='loading')await click('[data-as-complete]');else await click('[data-as-read]');
  test(s+' load recovery',status()==='ready');
  test(s+' confirmed read initializes consumer',!$('[data-aux-default-unavailable]')&&$('.asc-panel')?.dataset.ascScope==='work'&&$('[data-asc-count]')?.textContent==='3');
  await click('.settings-window .close');await click('.menubar-item');await click('.menubar-item');
  test(s+' confirmed reopen remains valid',$('.asc-panel')?.dataset.ascScope==='work'&&$('[data-asc-count]')?.textContent==='3');
  await click('[data-as-open]');continue;
 }
 await select('[data-as-default]','auxiliary');const before=$('[data-as-confirmed]').textContent;await click('[data-as-save-default]');
 if(['saving','saveFailed','saveUnknown','conflict'].includes(s)){
  test(s+' old committed value',$('[data-as-confirmed]').textContent===before);await reopen();test(s+' survives window',status()===s);
  await click(s==='saving'?'[data-as-complete]':s==='saveFailed'?'[data-as-retry]':'[data-as-read]');
  test(s+' resolved status',['saved','reloaded'].includes(status()));test(s+' readback value',$('[data-as-default]').value===(s==='conflict'?'all':'auxiliary'));
 }else test(s+' committed',$('[data-as-default]').value==='auxiliary'&&['saved','pending'].includes(status()));
}
await scene('normal');await click('[data-as-find]');test('effective override visible',!!$('[data-as-result]'));
await select('[data-as-choice]','auto');await click('[data-as-save-origin]');test('clear removes override without querying classification',!$('[data-as-clear="claude:old-01"]')&&$('[data-as-choice]').value==='auto'&&/not queried|未查询/.test($('[data-as-effective]').textContent));await click('[data-as-refresh-origin]');test('independent query restores automatic unknown',/Unknown|未知/.test($('[data-as-automatic]').textContent));
await click('[data-as-save-origin]');test('idempotent clear',status()==='noChange');
await select('[data-as-choice]','auxiliary');await click('[data-as-save-origin]');await reopen();await click('[data-as-find]');test('override persists reopening',$('[data-as-choice]').value==='auxiliary');
await click('[data-as-clear="codex:archived-1"]');test('orphan override removed',!$('[data-as-clear="codex:archived-1"]'));test('focus retained after removal',document.activeElement.hasAttribute('data-as-clear'));
await click('[data-as-clear="claude:old-01"]');test('empty override focus',document.activeElement===$('[data-as-find]'));
await select('[data-as-client]','codex');test('client change clears old target',!$('[data-as-result]'));await click('[data-as-find]');test('not found never saves',!$('[data-as-save-origin]'));
await scene('normal');await select('[data-as-default]','all');await reopen();test('unsubmitted draft discarded',$('[data-as-default]').value==='work');
await select('[data-as-default]','auxiliary');await click('[data-as-save-default]');await click('.settings-window .close');
const prior=$('.asc-panel')?.dataset.ascScope;test('current popover unchanged',prior==='work');await click('.menubar-item');await click('.menubar-item');test('next popover uses saved default',$('.asc-panel')?.dataset.ascScope==='auxiliary');
await click('[data-as-open]');
await scene('receiptMissing');await select('[data-as-default]','auxiliary');await click('[data-as-save-default]');const request=$('[data-as-request]').textContent;await reopen();await click('[data-as-read]');test('missing receipt remains unknown',status()==='saveUnknown'&&$('[data-as-save-default]').disabled&&$('[data-as-receipt]').textContent.includes('unknown'));test('missing receipt retains request identity',$('[data-as-request]').textContent===request);await click('[data-as-retry]');test('same request recovery unlocks group',status()==='saved'&&!$('[data-as-save-default]').disabled&&!$('[data-as-request]'));test('recovery retains request id',$('[data-as-receipt]').textContent.includes('fixture-request-1'));await select('[data-as-default]','all');await click('[data-as-save-default]');test('new operation after recovery',status()==='saved'&&$('[data-as-confirmed]').textContent.match(/All|全部/));
await scene('replayed');test('old receipt does not remove new override',status()==='replayed'&&!!$('[data-as-clear="claude:old-01"]')&&$('[data-as-receipt]').textContent.includes('revision 3'));await click('[data-as-find]');test('current higher revision still visible',$('[data-as-choice]').value==='auxiliary');
for(const source of ['sourceUnconfigured','sourceUnavailable','sourceUnsupported']){await scene(source);await click('[data-as-find]');test(source+' source state present',!!$('[data-as-source]'));test(source+' manual writes enabled',!$('[data-as-save-origin]').disabled);test(source+' automatic origin unknown',/Unknown|未知/.test($('[data-as-automatic]').textContent));}
await scene('saving');await click('[data-as-clear="codex:archived-1"]');test('pending clear retains row and request',!!$('[data-as-clear="codex:archived-1"]')&&!!$('[data-as-request]'));await click('[data-as-complete]');test('pending clear restores next focus',!$('[data-as-clear="codex:archived-1"]')&&document.activeElement?.dataset.asClear==='claude:old-01');
await scene('permissionWarning');await select('[data-as-default]','auxiliary');await click('[data-as-save-default]');test('post-commit warning keeps saved value',status()==='saved'&&$('[data-as-default]').value==='auxiliary'&&$('[data-as-warning]').textContent.length>0);
return {checks,failures};
})()
