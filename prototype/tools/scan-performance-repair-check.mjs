// Synthetic UX acceptance only. No native timing, real log or producer claims.
import fs from 'node:fs';
import path from 'node:path';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
const run=promisify(execFile),base=process.argv[2];
if(!/^http:\/\/127\.0\.0\.1:\d+$/.test(base??''))throw Error('Local synthetic prototype required');
const root=process.cwd(),out=path.join(root,'docs/topics/scan-performance/ux/prototype/serving');
const session=(await run('agent-browser',['session','id','--scope','worktree','--prefix','scan-repair'])).stdout.trim();
async function browser(...args){return(await run('agent-browser',args,{env:{...process.env,AGENT_BROWSER_SESSION:session},maxBuffer:4*1024*1024})).stdout;}
async function evaluate(code){const raw=(await browser('eval',code)).trim();return JSON.parse(raw);}
const checks=[];
function check(name,passed){checks.push({name,passed:!!passed});if(!passed)throw Error('Synthetic acceptance failed: '+name);}
async function open(query){await browser('open',base+'/?'+query);}
for(const lang of ['zh','en'])for(const theme of ['dark','light'])for(const width of [280,420]){
 await browser('set','viewport','900','1500');await open(`serving=mixed&lang=${lang}&theme=${theme}&width=${width}&tab=usage`);
 const initial=await evaluate(`(()=>({phase:document.querySelector('[data-serving-status]')?.dataset.phase,hero:document.querySelector('.hero')?.innerText}))()`);
 check(`M1-F1 ${lang}/${theme}/${width} today usage readable and counts unknown`,initial.phase==='mixed'&&initial.hero.includes('—')&&/\d/.test(initial.hero));
 await browser('screenshot',path.join(out,`mixed-${lang}-${theme}-${width}.png`));
 await browser('click','.segmented.periods button:nth-child(2)');
 check(`M1-F1 ${lang}/${theme}/${width} 7d unavailable`,await evaluate(`!!document.querySelector('[data-domain-unavailable="7d"]') && document.querySelector('.hero').innerText.includes('—')`));
 await browser('click','.segmented.periods button:nth-child(3)');
 check(`M1-F1 ${lang}/${theme}/${width} 30d partial`,await evaluate(`document.querySelector('[data-serving-status]').dataset.phase==='mixed' && !document.querySelector('[data-domain-unavailable="30d"]')`));
 await browser('click','[data-tab="sessions"]');
 check(`M1-F1 ${lang}/${theme}/${width} sessions unavailable`,await evaluate(`!!document.querySelector('[data-domain-unavailable="sessions"]')`));
 await browser('click','.segmented.clients button:nth-child(2)');
 const before=await evaluate(`JSON.stringify([...document.querySelectorAll('[role="tab"][aria-selected="true"]')].map(x=>x.textContent.replace(/[\d.$≈—]+/g,'')))`);
 await browser('click','[data-serving-publish]');
 const after=await evaluate(`JSON.stringify([...document.querySelectorAll('[role="tab"][aria-selected="true"]')].map(x=>x.textContent.replace(/[\d.$≈—]+/g,'')))`);
 check(`M1-F1 ${lang}/${theme}/${width} atomic publication preserves choices`,before===after&&await evaluate(`document.querySelector('[data-serving-status]').dataset.phase==='memory'&&!document.querySelector('[data-domain-unavailable]')`));
}
for(const lang of ['zh','en'])for(const theme of ['dark','light']){
 await open(`serving=rebuilding&settings=1&telemetry=unknown&lang=${lang}&theme=${theme}`);
 check(`S1-F1 ${lang}/${theme} default off`,await evaluate(`document.querySelector('[data-otel-switch]').getAttribute('aria-checked')==='false'`));
 await browser('click','[data-otel-switch]');
 check(`S1-F1 ${lang}/${theme} enabled defaults unknown`,await evaluate(`document.querySelector('[data-otel-status]').dataset.otelStatus==='unknown'`));
 await browser('screenshot',path.join(out,`settings-unknown-${lang}-${theme}.png`));
 for(const state of ['unknown','unconfigured','waiting','ready','capture_only','disconnected','paused']){
  await browser('select','[data-telemetry-scenario]',state);
  check(`S1-F1 ${lang}/${theme} ${state}`,await evaluate(`document.querySelector('[data-otel-status]').dataset.otelStatus===${JSON.stringify(state)}`));
 }
 await browser('click','[data-otel-switch]');
 check(`S1-F1 ${lang}/${theme} disabled preserves UI`,await evaluate(`document.querySelector('[data-otel-status]').dataset.otelStatus==='off'`));
}
for(const cols of [40,80])for(const scenario of ['app','absent','independent','stopped','mismatch','telemetry','telemetry_unknown','stored_partial','preview']){
 await open(`surface=cli&engine=${scenario}&cols=${cols}&lang=en&theme=${scenario==='telemetry'?'light':'dark'}`);
 const v=await evaluate(`(()=>{const value=JSON.parse(document.querySelector('[data-engine-json]').textContent);return {value,out:document.querySelector('[data-engine-json-stdout]').textContent.trim(),err:document.querySelector('[data-engine-json-stderr]').textContent.trim()}})()`);
 check(`C1-F2 ${cols}/${scenario} full public JSON`,v.value.schema_version===1&&typeof v.value.generated_at==='string'&&Array.isArray(v.value.warnings)&&typeof v.value.partial==='boolean'&&Object.hasOwn(v.value,'data')&&!v.value.command.includes(' '));
 check(`C1-F2 ${cols}/${scenario} JSON channel`,scenario==='mismatch'?v.out===''&&v.err.startsWith('{')&&v.value.data===null:v.err===''&&v.out.startsWith('{'));
 if(scenario==='stored_partial')check(`C1-F1 ${cols} coverage warning`,v.value.command==='usage.summary'&&v.value.partial&&v.value.warnings.includes('source_coverage_partial'));
 if(scenario==='telemetry_unknown')check(`S1-F1 ${cols} CLI unknown`,v.value.data.producer_configured==='unknown'&&v.value.data.last_received_at===null);
 if(cols===80&&['app','mismatch','telemetry','telemetry_unknown','stored_partial'].includes(scenario))await browser('screenshot',path.join(out,scenario==='app'?'cli-app-en-dark.png':scenario==='mismatch'?'cli-mismatch-en-dark.png':scenario==='telemetry'?'cli-telemetry-en-light.png':`cli-${scenario}-en-dark.png`));
}
fs.writeFileSync(path.join(out,'repair-checks.json'),JSON.stringify({kind:'scan-performance-review-r1-repair-synthetic-acceptance',checks,assertions:checks.length,all_passed:checks.every(x=>x.passed),scope:'Synthetic prototype UI and complete CLI envelopes only; no native performance/producer/runtime acceptance'},null,2)+'\n');
console.log(JSON.stringify({assertions:checks.length,all_passed:true,session}));
