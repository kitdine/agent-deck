// Browser checks for the redesigned Sessions settings group (2026-11-07).
// Run from ?settings=1&settingsPane=sessions&auxSettings=normal&auxMenu=normal&tab=sessions via
//   agent-browser eval "$(cat prototype/tools/check-auxiliary-settings.js)"
// Synthetic fixture only: no real storage, CAS, Widget or native accessibility claim.
(async()=>{
const failures=[];let checks=0;const tick=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
const $=s=>document.querySelector(s),$$=s=>[...document.querySelectorAll(s)];
const test=(n,v)=>{checks++;if(!v)failures.push(n);};
const click=async s=>{const el=typeof s==='string'?$(s):s;if(!el)throw new Error('missing '+s);el.click();await tick();};
const select=async(s,v)=>{$(s).value=v;$(s).dispatchEvent(new Event('change',{bubbles:true}));await tick();};
const typeId=async v=>{const i=$('[data-as-id]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,v);i.dispatchEvent(new Event('input',{bubbles:true}));await tick();};
const scene=s=>select('[data-as-stage]',s);
const status=()=>$('.as-settings').dataset.asStatus;
const checked=attr=>$(`[${attr}][aria-checked="true"]`)?.getAttribute(attr)??null;
const openSheet=async()=>{if(!$('[data-as-sheet]'))await click('[data-as-manage]');};
const closeSheet=async()=>{if($('[data-as-sheet]'))await click('[data-as-sheet-done]');};
const find=async(client,id)=>{await openSheet();await click(`[data-as-client-option="${client}"]`);await typeId(id);await click('[data-as-find]');};
const reopen=async()=>{await click('.settings-window .close');await click('[data-as-open]');};
const noOverflow=root=>[...root.querySelectorAll('*')].filter(x=>x.clientWidth&&x.scrollWidth>x.clientWidth+1).length===0;
// The first interaction must not race the initial render or a dev-server reload.
for(let i=0;i<50&&!(document.readyState==='complete'&&$('.as-settings'));i++)await new Promise(r=>setTimeout(r,100));
await new Promise(r=>setTimeout(r,300));

// Layout: its own pane in the tabbed window (default; open with &settingsPane=sessions), or, for the
// historical comparison only (?settingsTabs=0), one group after the menu bar group in the long list.
const tabbed=!!$('.settings-window.tabbed');
const groups=$$('.settings-body > .settings-group:not([hidden])').map(g=>g.dataset.group??g.querySelector('.settings-group-title')?.textContent);
if(!tabbed)test('sessions group follows menu bar group',groups.indexOf('sessions')===2);
else{
 const tabs=$$('.settings-tabs [role="tab"]');
 test('four panes in order',tabs.map(x=>x.dataset.pane).join()==='general,sessions,live,quota');
 test('sessions pane selected and alone',$('.settings-tabs [aria-selected="true"]')?.dataset.pane==='sessions'&&groups.join()==='sessions');
 test('title names the pane',$('.settings-window > header strong').textContent===$('.settings-tabs [aria-selected="true"] span').textContent);
 test('tabpanel labelled by its tab',$('#settings-pane[role="tabpanel"]')?.getAttribute('aria-labelledby')==='settings-tab-sessions');
 test('roving tabindex',tabs.filter(x=>x.tabIndex===0).length===1);
 $('.settings-tabs [aria-selected="true"]').focus();
 $('.settings-tabs').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));await tick();
 test('arrow key moves to the next pane',$('.settings-tabs [aria-selected="true"]')?.dataset.pane==='live'&&document.activeElement?.dataset.pane==='live'&&$('[data-group="telemetry"]')?.hidden===false&&$('[data-group="sessions"]')?.hidden===true);
 await click('.settings-tabs [data-pane="sessions"]');
 await click('.settings-window .close');await click('[data-as-open]');
 test('pane remembered after reopen',$('.settings-tabs [aria-selected="true"]')?.dataset.pane==='sessions');
 await click('[data-as-manage]');
 test('sheet blocks the tabs',$('.settings-tabs').inert===true&&$('.settings-window').getBoundingClientRect().height>=Math.min(700,innerHeight-48)-1);
 await click('[data-as-sheet-done]');
 test('tabs restored after sheet',$('.settings-tabs').inert===false);
}
test('standard window width',Math.round($('.settings-window').getBoundingClientRect().width)===Math.min(460,innerWidth-32));
test('no protocol fields on the main surface',!/revision|版本：|request|请求 ID/i.test($('.as-settings').textContent));

for(const s of ['normal','loading','loadFailed','saving','saveFailed','saveUnknown','conflict','pending']){
 await scene(s);
 test(s+' geometry',noOverflow($('.as-settings')));
 if(s==='loading'||s==='loadFailed'){
  test(s+' no guessed default',checked('data-as-default-option')===null&&$('[data-as-default-option="work"]').disabled);
  test(s+' override summary unavailable',/不可读|unavailable/i.test($('[data-as-summary]').textContent));
  await openSheet();test(s+' list not shown as empty',!!$('[data-as-list-unavailable]')&&!$('[data-as-empty]'));await closeSheet();
  await click('.settings-window .close');await click('.menubar-item');await click('.menubar-item');
  test(s+' reopened consumer blocks unconfirmed default',!!$('[data-aux-default-unavailable]')&&!$('[data-asc-count]')&&!$('.asc-scope'));
  await click('[data-tab="usage"]');await click('[data-tab="sessions"]');
  test(s+' tab roundtrip still unavailable',!!$('[data-aux-default-unavailable]')&&!$('[data-asc-count]'));
  await click('[data-as-open]');
  if(s==='loading')await click('[data-as-complete]');else await click('[data-as-read]');
  test(s+' load recovery',status()==='ready'&&checked('data-as-default-option')==='work');
  test(s+' confirmed read initializes consumer',!$('[data-aux-default-unavailable]')&&$('.asc-panel')?.dataset.ascScope==='work'&&$('[data-asc-count]')?.textContent==='3');
  await click('.settings-window .close');await click('.menubar-item');await click('.menubar-item');
  test(s+' confirmed reopen remains valid',$('.asc-panel')?.dataset.ascScope==='work'&&$('[data-asc-count]')?.textContent==='3');
  await click('[data-as-open]');continue;
 }
 // Selecting a segment is the save; there is no separate Save button or draft.
 await click('[data-as-default-option="auxiliary"]');
 if(['saving','saveFailed','saveUnknown','conflict'].includes(s)){
  test(s+' confirmed value stays selected',checked('data-as-default-option')==='work');
  // saveFailed leaves the group writable: retry the same request, or choose again.
  test(s+' lock matches contract',$('[data-as-default-option="all"]').disabled===(s!=='saveFailed')&&!$('[data-as-default-status]').textContent.includes('undefined'));
  test(s+' pending dot only while in flight',!!$('[data-as-default-option="auxiliary"] .as-pending-dot')===['saving','saveUnknown'].includes(s));
  test(s+' request id only when unconfirmed',!!$('[data-as-request]')===(s==='saveUnknown'));
  await reopen();test(s+' survives window',status()===s);
  await click(s==='saving'?'[data-as-complete]':s==='saveFailed'?'[data-as-retry]':'[data-as-read]');
  test(s+' resolved status',['saved','reloaded'].includes(status()));
  test(s+' readback value',checked('data-as-default-option')===(s==='conflict'?'all':'auxiliary'));
 }else test(s+' committed',checked('data-as-default-option')==='auxiliary'&&['saved','pending'].includes(status())&&$('[data-as-default-status]').textContent.length>0);
}

// Manual classification sheet.
await scene('normal');
await openSheet();
test('sheet is modal over the window',!!$('[data-as-sheet][aria-modal="true"]')&&$('.settings-body').inert===true);
test('focus starts in the ID field',document.activeElement===$('[data-as-id]'));
test('find disabled for empty ID',$('[data-as-find]').disabled);
test('sheet geometry',noOverflow($('[data-as-sheet]')));
$('[data-as-sheet]').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));await tick();
test('Escape closes only the sheet',!$('[data-as-sheet]')&&!!$('.settings-window')&&$('.settings-body').inert===false);
test('focus returns to Manage',document.activeElement===$('[data-as-manage]'));

// Modal focus scope: Tab / Shift+Tab cycle inside the open sheet (ASC-SET-R7-F2).
const press=async(shift=false)=>{(document.activeElement||document.body).dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',shiftKey:shift,bubbles:true,cancelable:true}));await tick();};
const inSheet=()=>!!$('[data-as-sheet]')?.contains(document.activeElement);
await openSheet();
$('[data-as-sheet-done]').focus();await press();
test('Tab from Done wraps to the first sheet control',inSheet()&&document.activeElement===$('[data-as-client-option="codex"]'));
await press(true);
test('Shift+Tab from the first control wraps to Done',document.activeElement===$('[data-as-sheet-done]'));
document.activeElement.blur();await press();
test('Tab from lost focus returns into the sheet',inSheet()&&!!$('[data-as-sheet]'));
await closeSheet();
await scene('saveUnknown');await find('claude','old-01');await click('[data-as-choice-option="auxiliary"]');
await press();
test('locked sheet: Tab reaches Check result',status()==='saveUnknown'&&document.activeElement===$('[data-as-sheet] [data-as-read]'));
await press();
test('locked sheet: Tab reaches Done',document.activeElement===$('[data-as-sheet-done]'));
await press();
test('locked sheet: Tab wraps back to Check result',document.activeElement===$('[data-as-sheet] [data-as-read]'));
await click('[data-as-sheet] [data-as-read]');await closeSheet();
await scene('normal');

await find('claude','old-01');
test('lookup shows the session card',!!$('[data-as-result]')&&checked('data-as-choice-option')==='work');
test('effective shows manual override',/工作|Work/.test($('[data-as-effective]').textContent)&&/手动|manual/.test($('[data-as-effective]').textContent));
await click('[data-as-choice-option="auto"]');
test('clear removes the override',!$('[data-as-clear="claude:old-01"]')&&checked('data-as-choice-option')==='auto');
test('clear is followed by a separate read of the current classification',/未知|Unknown/.test($('[data-as-automatic]').textContent)&&/未知|Unknown/.test($('[data-as-effective]').textContent));
const before=status();await click('[data-as-choice-option="auto"]');
test('selecting the current option sends nothing',status()===before);
await click('[data-as-choice-option="auxiliary"]');
await reopen();test('lookup draft discarded on reopen',(await openSheet(),$('[data-as-id]').value==='')&&!$('[data-as-result]'));
await click('[data-as-open-row="claude:old-01"]');
test('list row opens its session',checked('data-as-choice-option')==='auxiliary'&&$('[data-as-id]').value==='old-01');
test('unmatched override is marked, not applied',$('[data-as-row="codex:archived-1"]')?.dataset.matched==='false'&&!$('[data-as-open-row="codex:archived-1"]'));
await click('[data-as-clear="codex:archived-1"]');
test('orphan override removed',!$('[data-as-clear="codex:archived-1"]'));
test('focus moves to the next Remove',document.activeElement?.dataset.asClear==='claude:old-01');
await click('[data-as-clear="claude:old-01"]');
test('empty list focuses the ID field',document.activeElement===$('[data-as-id]')&&!!$('[data-as-empty]'));
await click('[data-as-client-option="codex"]');
test('client change clears the old card',!$('[data-as-result]'));
await click('[data-as-find]');
test('not found never offers a write',!!$('[data-as-notfound]')&&!$('[data-as-choice-option]'));
await closeSheet();

// Default affects the next open, not the open menu bar.
await scene('normal');
await click('[data-as-default-option="auxiliary"]');await click('.settings-window .close');
test('current popover unchanged',$('.asc-panel')?.dataset.ascScope==='work');
await click('.menubar-item');await click('.menubar-item');
test('next popover uses saved default',$('.asc-panel')?.dataset.ascScope==='auxiliary');
await click('[data-as-open]');

// Unconfirmed save: same request identity until a definite outcome.
await scene('receiptMissing');
await click('[data-as-default-option="auxiliary"]');
const request=$('[data-as-request]')?.textContent;
await reopen();await click('[data-as-read]');
test('missing receipt remains unknown',status()==='saveUnknown'&&$('[data-as-default-option="all"]').disabled&&checked('data-as-default-option')==='work');
test('missing receipt retains request identity',!!request&&$('[data-as-request]').textContent===request);
await click('[data-as-retry]');
test('same request recovery unlocks group',status()==='saved'&&!$('[data-as-default-option="all"]').disabled&&!$('[data-as-request]'));
await click('[data-as-default-option="all"]');
test('new operation after recovery',status()==='saved'&&checked('data-as-default-option')==='all');

await scene('replayed');
test('replayed receipt keeps the newer override',status()==='replayed'&&/2/.test($('[data-as-summary]').textContent));
await find('claude','old-01');
test('current higher revision still visible',checked('data-as-choice-option')==='auxiliary');
test('replayed explained in the sheet',$('[data-as-banner]').dataset.asBanner==='info');
await closeSheet();

for(const source of ['sourceUnconfigured','sourceUnavailable','sourceUnsupported']){
 await scene(source);
 test(source+' source row degraded',$('[data-as-source-mem]').dataset.state!=='available'&&$('[data-as-source-note]')?.dataset.asSourceNote==='adapter');
 await find('claude','old-01');
 test(source+' manual writes enabled',!$('[data-as-choice-option="auxiliary"]').disabled);
 test(source+' automatic origin unknown',/未知|Unknown/.test($('[data-as-automatic]').textContent));
 await closeSheet();
}
for(const [codex,state] of [['codexReceiverOff','receiverOff'],['codexNoRecords','noRecords'],['codexReceiverUnknown','receiverUnknown']]){
 await scene(codex);test(codex+' codex row',$('[data-as-source-codex]').dataset.state===state&&!$('[data-as-source-note]'));
}
await scene('loadFailed');test('core unavailable on both rows',$('[data-as-source-codex]').dataset.state==='coreUnavailable'&&$('[data-as-source-mem]').dataset.state==='core_unavailable');
// Core unavailable is not an adapter outage: nothing promises manual classification still works (ASC-SET-R7-F1).
test('core note does not promise manual classification',$('[data-as-source-note]')?.dataset.asSourceNote==='core'&&!/不受影响|still works/.test($('[data-as-source-note]').textContent)&&!!$('[data-as-read-core]'));
await openSheet();
test('sheet explains the core lock',$('[data-as-banner]').dataset.asBanner==='bad'&&$('[data-as-id]').disabled&&$('[data-as-find]').disabled&&!!$('[data-as-list-unavailable]'));
await closeSheet();
await click('[data-as-read-core]');
test('core recovery restores sources',status()==='ready'&&$('[data-as-source-mem]').dataset.state==='available'&&$('[data-as-source-codex]').dataset.state==='records'&&!$('[data-as-source-note]'));
await openSheet();test('core recovery restores lookup',!$('[data-as-id]').disabled&&$('[data-as-banner]').dataset.asBanner==='');await closeSheet();

// An override write in flight keeps its row, survives closing the sheet, and restores focus.
await scene('saving');
await openSheet();await click('[data-as-clear="codex:archived-1"]');
test('pending clear keeps the row',!!$('[data-as-clear="codex:archived-1"] .as-spin')&&$('[data-as-banner]').dataset.asBanner==='busy');
await closeSheet();
test('closed sheet still reports the write',$('[data-as-manual-status]')?.dataset.asManualStatus==='busy');
test('default locked by the override write',$('[data-as-default-option="all"]').disabled&&$('[data-as-default-status]').dataset.asDefaultStatus==='dim');
await openSheet();await click('[data-as-complete]');await tick();
test('pending clear restores next focus',!$('[data-as-clear="codex:archived-1"]')&&document.activeElement?.dataset.asClear==='claude:old-01');
await closeSheet();

await scene('permissionWarning');
await click('[data-as-default-option="auxiliary"]');
test('post-commit warning keeps saved value',status()==='saved'&&checked('data-as-default-option')==='auxiliary'&&!!$('[data-as-warning]'));
return {checks,failures};
})()
