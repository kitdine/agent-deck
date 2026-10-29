// Synthetic UX acceptance for the Codex Memories linkage and consented
// Codex configuration write (scan-performance otel design, 2026-10-29).
// Prototype only: no real Codex configuration, receiver or producer.
import fs from 'node:fs';
import path from 'node:path';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
const run=promisify(execFile),base=process.argv[2];
if(!/^http:\/\/127\.0\.0\.1:\d+$/.test(base??''))throw Error('Local synthetic prototype required');
const root=process.cwd(),out=path.join(root,'docs/topics/scan-performance/ux/prototype/serving');
const session=(await run('agent-browser',['session','id','--scope','worktree','--prefix','scan-otel-linkage'])).stdout.trim();
async function browser(...args){return(await run('agent-browser',args,{env:{...process.env,AGENT_BROWSER_SESSION:session},maxBuffer:4*1024*1024})).stdout;}
async function evaluate(code){return JSON.parse((await browser('eval',code)).trim());}
const checks=[];
function check(name,passed){checks.push({name,passed:!!passed});if(!passed)throw Error('Synthetic acceptance failed: '+name);}
const state=`(()=>{const s=document.querySelector('[data-otel-switch]');const g=document.querySelector('[data-group="telemetry"]');const row=document.querySelector('[data-otel-row-result]');const tone=row?.className||'';return {on:s.getAttribute('aria-checked')==='true',disabled:s.disabled,status:document.querySelector('[data-otel-status]').dataset.otelStatus,live:document.querySelector('.telemetry-details [role="status"]').textContent,consent:document.querySelector('[data-otel-consent]')?.dataset.otelConsent??null,consentText:document.querySelector('[data-otel-consent]')?.innerText??'',result:document.querySelector('[data-otel-result]')?.dataset.otelResult??null,row:row?.dataset.otelRowResult??'',rowText:row?.textContent??'',rowLive:row?.getAttribute('aria-live'),rowColor:getComputedStyle(row).color,dim:getComputedStyle(document.getElementById('settings-otel-hint')).color,notCounted:!!document.querySelector('[data-otel-not-counted]'),preview:!!document.querySelector('[data-otel-preview]'),modal:!!document.querySelector('[role="dialog"][aria-modal="true"]')};})()`;
async function settings(lang,theme,params){await browser('set','viewport','1280','1400');await browser('open',`${base}/?serving=rebuilding&settings=1&telemetry=unknown&lang=${lang}&theme=${theme}&${params}`);return evaluate(state);}
for(const lang of ['zh','en'])for(const theme of ['dark','light']){
 const tag=`${lang}/${theme}`;
 let v=await settings(lang,theme,'codexMemories=off');
 check(`${tag} memories off with collection off disables switch and explains`,v.disabled&&!v.on&&v.status==='memoriesOff'&&v.live.length>0);
 if(lang==='zh'&&theme==='dark')await browser('screenshot',path.join(out,'settings-linkage-memories-off-zh-dark.png'));
 v=await settings(lang,theme,'codexMemories=on&codexExporter=absent&codexWrite=written&codexRestore=removed');
 check(`${tag} memories on and collection off warns not counted`,!v.on&&!v.disabled&&v.notCounted);
 check(`${tag} collection is never on automatically`,!v.on&&v.status==='off');
 if(lang==='en'&&theme==='light')await browser('screenshot',path.join(out,'settings-linkage-not-counted-en-light.png'));
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} switch opens inline consent, not a modal, without writing`,v.consent==='write'&&!v.on&&!v.modal&&v.consentText.includes('~/.codex/config.toml')&&v.consentText.includes('BEGIN AgentDeck OTel (managed)')&&!v.consentText.match(/[0-9a-f]{64}/));
 if(lang==='zh'&&theme==='light')await browser('screenshot',path.join(out,'settings-linkage-consent-zh-light.png'));
 await browser('click','[data-otel-cancel]');v=await evaluate(state);
 check(`${tag} cancel leaves everything off`,!v.on&&v.consent===null&&v.result===null);
 await browser('click','[data-otel-switch]');await browser('click','[data-otel-write]');v=await evaluate(state);
 check(`${tag} written turns on with restart notice`,v.on&&v.result==='written'&&v.row==='');
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} turning off removes managed block`,!v.on&&v.result==='removed'&&v.row==='');
 v=await settings(lang,theme,'codexMemories=on&codexExporter=absent&codexWrite=refused');
 check(`${tag} live region exists empty before failure`,v.rowLive==='polite'&&v.rowText==='');
 await browser('click','[data-otel-switch]');await browser('click','[data-otel-write]');v=await evaluate(state);
 check(`${tag} refused write keeps switch off with error tone`,!v.on&&v.row==='refused'&&v.rowText.length>0&&v.rowColor!==''&&v.rowColor!==v.dim);
 if(lang==='en'&&theme==='dark')await browser('screenshot',path.join(out,'settings-linkage-refused-en-dark.png'));
 await browser('select','[data-codex-stage="write"]','written');await browser('click','[data-otel-switch]');await browser('click','[data-otel-write]');v=await evaluate(state);
 check(`${tag} retry succeeds and clears error`,v.on&&v.row===''&&v.result==='written');
 v=await settings(lang,theme,'codexMemories=on&codexExporter=other');
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} existing exporter is not overwritten`,v.consent==='conflict'&&!v.on&&!v.consentText.includes('BEGIN AgentDeck'));
 if(lang==='zh'&&theme==='dark')await browser('screenshot',path.join(out,'settings-linkage-conflict-zh-dark.png'));
 await browser('click','[data-otel-receiver-only]');v=await evaluate(state);
 check(`${tag} receiver only shows the preview to merge`,v.on&&v.preview&&v.consent===null);
 v=await settings(lang,theme,'codexMemories=unknown&codexExporter=absent');
 check(`${tag} unknown memories neither blocks nor warns`,!v.disabled&&!v.notCounted);
 await browser('click','[data-otel-switch]');await browser('click','[data-otel-self]');v=await evaluate(state);
 check(`${tag} self configure turns on and expands preview`,v.on&&v.preview&&v.result==='selfConfigured');
 v=await settings(lang,theme,'codexMemories=on&codexExporter=managed&codexRestore=incomplete');
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} managed exporter enables without consent`,v.on&&v.consent===null);
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} restore incomplete leaves switch off with warning tone`,!v.on&&v.row==='incomplete'&&v.rowColor!==v.dim);
 if(lang==='en'&&theme==='light')await browser('screenshot',path.join(out,'settings-linkage-restore-incomplete-en-light.png'));
 v=await settings(lang,theme,'codexMemories=on&codexExporter=absent&codexWrite=written&codexRestore=removed');
 await browser('click','[data-otel-switch]');await browser('click','[data-otel-write]');
 await browser('select','[data-codex-stage="memories"]','off');v=await evaluate(state);
 check(`${tag} memories turned off stops receiver but the switch can still turn collection off`,v.on&&!v.disabled&&v.status==='memoriesStopped');
 if(lang==='en'&&theme==='dark')await browser('screenshot',path.join(out,'settings-linkage-memories-stopped-en-dark.png'));
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} turning off with memories off removes the managed block`,!v.on&&v.result==='removed'&&v.disabled&&v.status==='memoriesOff');
 await browser('select','[data-codex-stage="memories"]','on');v=await evaluate(state);
 check(`${tag} memories back on keeps collection off and only warns`,!v.on&&!v.disabled&&v.notCounted&&v.status==='off');
 v=await settings(lang,theme,'codexMemories=on&codexExporter=agentdeck');
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} user-owned exporter enables without consent`,v.on&&v.consent===null);
 await browser('click','[data-otel-switch]');v=await evaluate(state);
 check(`${tag} disabling never removes the user's own exporter`,!v.on&&v.result==='userConfigKept'&&v.row==='');
 if(lang==='zh'&&theme==='light')await browser('screenshot',path.join(out,'settings-linkage-user-config-kept-zh-light.png'));
}
fs.writeFileSync(path.join(out,'otel-linkage-checks.json'),JSON.stringify({kind:'scan-performance-otel-linkage-synthetic-acceptance',checks,all_passed:checks.every(x=>x.passed),scope:'Synthetic prototype Settings specimens only; no real Codex configuration, receiver, producer or native acceptance'},null,2)+'\n');
console.log(`${checks.length} checks passed`);
