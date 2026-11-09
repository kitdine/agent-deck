// Cross-page lifecycle checks for the tabbed Settings window (ASC-SET-R9-F1).
// Switching pages hides the others; it must not unmount them. A write fact, the
// ownership of AgentDeck's managed Codex block, an unresolved result or a
// confirmation in progress must survive a page round trip.
// Run once per Codex stage, from ?settings=1&settingsPane=live&lang=<zh|en>&theme=<dark|light> plus
//   (none) | &codexWrite=refused | &codexRestore=incomplete | &codexExporter=other
// via agent-browser eval "$(cat prototype/tools/check-settings-pane-lifecycle.js)".
// Synthetic fixture only: no real Codex configuration is read or written.
(async()=>{
const failures=[];let checks=0;const tick=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
const $=s=>document.querySelector(s),$$=s=>[...document.querySelectorAll(s)];
const test=(n,v)=>{checks++;if(!v)failures.push(n);};
const click=async s=>{const el=$(s);if(!el)throw new Error('missing '+s);el.click();await tick();};
for(let i=0;i<50&&!(document.readyState==='complete'&&$('[data-otel-switch]'));i++)await new Promise(r=>setTimeout(r,100));
await new Promise(r=>setTimeout(r,300));
const q=new URLSearchParams(location.search);
const stage=q.get('codexExporter')==='other'?'conflict':q.get('codexWrite')==='refused'?'refused':q.get('codexRestore')==='incomplete'?'incomplete':'written';
const on=()=>$('[data-otel-switch]').getAttribute('aria-checked')==='true';
const info=()=>$('[data-otel-result]')?.dataset.otelResult??null;
const row=()=>$('[data-otel-row-result]')?.dataset.otelRowResult||null;
const consent=()=>$('[data-otel-consent]')?.dataset.otelConsent??null;
const visible=()=>$$('.settings-body > .settings-group:not([hidden])').map(g=>g.dataset.group??'menubar').join();
// General, then back to Live usage: the telemetry group is hidden, not rebuilt.
const roundtrip=async label=>{
 const node=$('[data-group="telemetry"]');
 await click('.settings-tabs [data-pane="general"]');
 test(label+': only General is shown',visible()==='general,menubar'&&node.hidden===true);
 await click('.settings-tabs [data-pane="live"]');
 test(label+': same telemetry instance shown again',$('[data-group="telemetry"]')===node&&node.hidden===false&&visible()==='telemetry');
};

test('opens on Live usage',$('.settings-tabs [aria-selected="true"]')?.dataset.pane==='live'&&visible()==='telemetry');
await click('[data-otel-switch]');
if(stage==='conflict'){
 test('conflict confirmation shown',consent()==='conflict'&&!on());
 await roundtrip('pending conflict');
 test('conflict confirmation kept',consent()==='conflict'&&!on());
 // A lost confirmation is already a failure; reopen it so the later steps still run.
 if(consent()===null)await click('[data-otel-switch]');
 await click('[data-otel-receiver-only]');
 test('receiver on with configuration shown',on()&&!!$('[data-otel-preview]'));
 await roundtrip('receiver only');
 test('receiver and configuration kept',on()&&!!$('[data-otel-preview]')&&consent()===null);
 await click('[data-otel-switch]');
 test('user configuration not removed',!on()&&info()===null&&row()===null);
}else{
 test('write confirmation shown',consent()==='write'&&!on());
 await roundtrip('pending write');
 test('write confirmation kept',consent()==='write'&&!on());
 // A lost confirmation is already a failure; reopen it so the later steps still run.
 if(consent()===null)await click('[data-otel-switch]');
 await click('[data-otel-write]');
 if(stage==='refused'){
  test('refused write leaves collection off',row()==='refused'&&!on());
  await roundtrip('refused');
  test('refused result kept',row()==='refused'&&!on());
 }else{
  test('written',info()==='written'&&on());
  await roundtrip('written');
  test('written result kept',info()==='written'&&on()&&consent()===null);
  await click('[data-otel-switch]');
  if(stage==='incomplete'){
   test('restore conflict reported after a round trip',row()==='incomplete'&&!on());
   await roundtrip('restore conflict');
   test('restore conflict kept',row()==='incomplete'&&!on());
  }else{
   test('managed block removed after a round trip',info()==='removed'&&!on());
   await click('[data-otel-switch]');
   test('turning on again asks to write again',consent()==='write'&&!on());
  }
 }
}
return {stage,checks,failures};
})()
