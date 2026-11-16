// Existing historical DB with today's hinted files not yet ingested.
// All input files are hard-linked from the private frozen copy, never live HOME.
import fs from 'node:fs';import path from 'node:path';import crypto from 'node:crypto';
import {execFile} from 'node:child_process';import {promisify} from 'node:util';
process.umask(0o077);
const run=promisify(execFile),root=path.resolve(process.argv[2]??'');
if(!/^\/private\/tmp\/agentdeck-scan-performance-firstload\.[A-Za-z0-9]+$/.test(root))throw Error('Private firstload root required');
const binary=path.join(root,'agentdeck'),home=path.join(root,'history-home'),seed=path.join(root,'history-seed');
if(fs.existsSync(home)||fs.existsSync(seed))throw Error('Requires unused history fixture');
const inv=JSON.parse(fs.readFileSync(path.join(root,'private-inventory.json'),'utf8'));
const cap=JSON.parse(fs.readFileSync(path.join(root,'capture.json'),'utf8'));
const start=new Date(cap.start),end=new Date(cap.end),days=[cap.start.slice(0,10),new Date(end.getTime()-1).toISOString().slice(0,10)].map(x=>x.replaceAll('-','/'));
const isToday=x=>x.mtime_ms>=start.getTime()||x.relative.startsWith('.codex/')&&days.some(d=>x.relative.includes('/sessions/'+d+'/')||x.relative.includes('/archived_sessions/'+d+'/'));
function link(x){const target=path.join(home,x.relative);fs.mkdirSync(path.dirname(target),{recursive:true,mode:0o700});fs.linkSync(path.join(root,'home',x.relative),target);}
for(const dir of ['.codex/sessions','.codex/archived_sessions','.claude/projects'])fs.mkdirSync(path.join(home,dir),{recursive:true,mode:0o700});
for(const x of inv.filter(x=>!isToday(x)))link(x);
const env={...process.env,HOME:home,CODEX_HOME:path.join(home,'.codex'),TZ:cap.timeZone};delete env.AGENTDECK_FIRSTLOAD_DAY_HINTS;
async function cli(state,args){const t=performance.now();const r=await run(binary,['--state-dir',state,'--format','json',...args],{env,maxBuffer:32*1024*1024});return {ms:performance.now()-t,value:JSON.parse(r.stdout)};}
await cli(seed,['scan']);
const before=await cli(seed,['usage','stats','--period','today','--no-scan']);
for(const x of inv.filter(isToday))link(x);
const oracle=JSON.parse(fs.readFileSync(path.join(root,'private-oracle-snapshot.json'),'utf8')).data.usage.presentation.scopes.find(x=>x.client==='all').periods.items.find(x=>x.period==='today').totals;
const compare=totals=>['tokens','events','sessions','known_catalog_base_cost','known_provider_cost'].every(k=>JSON.stringify(totals[k])===JSON.stringify(oracle[k]));
async function clone(state){
 fs.mkdirSync(state,{mode:0o700});
 const code=`import sqlite3,pathlib,sys\nsrc=pathlib.Path(sys.argv[1]); dst=pathlib.Path(sys.argv[2])\nfor name in ('agentdeck.sqlite3','sessions.sqlite3'):\n a=sqlite3.connect(src.joinpath(name).as_uri()+'?mode=ro',uri=True)\n b=sqlite3.connect(dst/name)\n a.backup(b);b.close();a.close()\n`;
 await run('python3',['-c',code,seed,state]);
 // Copy only regular ancillary files. Runtime sockets/locks/receipts do not
 // belong to the cloned database; the new state has its own finite request.
 for(const entry of fs.readdirSync(seed,{withFileTypes:true}))if(entry.isFile()&&entry.name==='desktop-derived-cache.json')fs.copyFileSync(path.join(seed,entry.name),path.join(state,entry.name));
}
const samples=[];
for(let pair=0;pair<2;pair++)for(const mode of(pair%2?['today_usage_first','current_full_refresh']:['current_full_refresh','today_usage_first'])){
 const state=path.join(root,`history-${pair}-${mode}`);await clone(state);
 const scan=await cli(state,mode==='today_usage_first'?['scan','--scope','usage']:['scan']);
 const result=await cli(state,mode==='today_usage_first'?['usage','stats','--period','today','--no-scan']:['desktop','snapshot']);
 const totals=mode==='today_usage_first'?result.value.data.totals:result.value.data.usage.presentation.scopes.find(x=>x.client==='all').periods.items.find(x=>x.period==='today').totals;
 if(!compare(totals))throw Error('Pending-today refresh differs from full oracle');
 const sample={pair,mode,scan_wait_ms:scan.ms,presentation_ms:result.ms,data_ready_ms:scan.ms+result.ms,today_totals_equal:true,
  available_scope:mode==='today_usage_first'?'today usage only; sessions/work signals/other panels may still be pending':'full desktop snapshot'};
 samples.push(sample);console.log(JSON.stringify(sample));
 if(mode==='today_usage_first'){
  // Complete the missing domains before disposing this fixture. Not included
  // in first usable today-usage time; it is independent background completion.
  const follow=await cli(state,['scan']);const full=await cli(state,['desktop','snapshot']);
  if(!compare(full.value.data.usage.presentation.scopes.find(x=>x.client==='all').periods.items.find(x=>x.period==='today').totals))throw Error('Follow-up totals differ');
  sample.remaining_full_refresh_ms=follow.ms+full.ms;
 }
}
fs.writeFileSync(path.join(root,'history-today-results.json'),JSON.stringify({kind:'history-pending-today-pilot-v1',samples,
 seeded_historical_files:inv.filter(x=>!isToday(x)).length,newly_available_today_files:inv.filter(isToday).length,
 today_was_empty_before:before.value.data.totals.tokens===0&&before.value.data.totals.events===0,
 seeded_state_had_full_today:compare(before.value.data.totals),
 helper_sha256:crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex'),
 limits:['Data preparation only; no native frame','2 pilot pairs; no final p95 claim','usage first exposes today usage, not a full popover or complete session/work-signal result','Fixture setup/history import and SQLite backup excluded because historical data is the precondition','Sources and original mtimes frozen; new files represent pending day input, not production changes']},null,2)+'\n',{mode:0o600});
