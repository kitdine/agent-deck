// Bounded first-install/today-priority experiment. Live capture requires the
// caller's explicit prior authorization and --capture-authorized.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
const run=promisify(execFile);
process.umask(0o077);
const root=path.resolve(process.argv[2]??'');
if(!root.startsWith('/private/tmp/agentdeck-scan-performance-firstload.'))throw Error('Private firstload root required');
const home=path.join(root,'home'),binary=path.join(root,'agentdeck');
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
const write=(name,value)=>fs.writeFileSync(path.join(root,name),JSON.stringify(value,null,2)+'\n',{mode:0o600});
const now=()=>performance.now();
const timeZone='America/New_York';
const day=new Intl.DateTimeFormat('en-CA',{timeZone,year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date());
const offset=new Intl.DateTimeFormat('en-US',{timeZone,timeZoneName:'longOffset'}).formatToParts(new Date(day+'T12:00:00Z')).find(x=>x.type==='timeZoneName').value.replace('GMT','');
const start=new Date(day+'T00:00:00'+offset),end=new Date(start.getTime()+86400000);
const dirs=[start.toISOString().slice(0,10),new Date(end.getTime()-1).toISOString().slice(0,10)].map(x=>x.replaceAll('-','/'));
const roots=['.codex/sessions','.codex/archived_sessions','.claude/projects'];
let inventory;
if(process.argv.includes('--capture-authorized')){
 const original=process.env.HOME;
 inventory=[];const began=now();
 function visit(dir){if(!fs.existsSync(dir))return;for(const ent of fs.readdirSync(dir,{withFileTypes:true})){
  const source=path.join(dir,ent.name);if(ent.isDirectory()){visit(source);continue;}
  if(!ent.isFile()||!ent.name.endsWith('.jsonl'))continue;
  const stat=fs.statSync(source),relative=path.relative(original,source),target=path.join(home,relative);
  fs.mkdirSync(path.dirname(target),{recursive:true,mode:0o700});
  const input=fs.openSync(source,'r'),output=fs.openSync(target,'wx',0o600),hash=crypto.createHash('sha256');
  let bytes=0;const buffer=Buffer.alloc(1024*1024);
  try{while(bytes<stat.size){const n=fs.readSync(input,buffer,0,Math.min(buffer.length,stat.size-bytes),bytes);if(!n)break;fs.writeSync(output,buffer.subarray(0,n));hash.update(buffer.subarray(0,n));bytes+=n;}}
  finally{fs.closeSync(input);fs.closeSync(output);}
  fs.utimesSync(target,stat.atime,stat.mtime);
  inventory.push({relative,bytes,mtime_ms:stat.mtimeMs,sha256:hash.digest('hex')});
 }}
 for(const dir of roots)visit(path.join(original,dir));
 for(const dir of roots)fs.mkdirSync(path.join(home,dir),{recursive:true,mode:0o700});
 write('private-inventory.json',inventory);
 write('capture.json',{files:inventory.length,bytes:inventory.reduce((s,x)=>s+x.bytes,0),capture_ms:now()-began,
  corpus_digest:sha(inventory.map(x=>`${x.bytes}:${x.sha256}`).sort().join('\n')),
  corpus_digest_algorithm:'SHA256 of sorted size:content-SHA256 entries joined by LF; no final LF',day,timeZone,start:start.toISOString(),end:end.toISOString(),original_mtime_preserved:true});
 console.log(JSON.stringify({phase:'capture',...JSON.parse(fs.readFileSync(path.join(root,'capture.json'),'utf8'))}));
 if(process.argv.includes('--capture-only'))process.exit(0);
}else inventory=JSON.parse(fs.readFileSync(path.join(root,'private-inventory.json'),'utf8'));
const selectBegin=now();
const selected=inventory.filter(x=>x.mtime_ms>=start.getTime()||
 (x.relative.startsWith('.codex/')&&dirs.some(day=>x.relative.includes('/sessions/'+day+'/')||x.relative.includes('/archived_sessions/'+day+'/'))));
const selection_ms=now()-selectBegin;
console.log(JSON.stringify({phase:'planned_sources',full_files:inventory.length,today_hint_files:selected.length,today_hint_bytes:selected.reduce((n,x)=>n+x.bytes,0)}));
const env={...process.env,HOME:home,CODEX_HOME:path.join(home,'.codex'),XDG_CONFIG_HOME:path.join(home,'.config'),XDG_STATE_HOME:path.join(home,'.local/state'),TZ:timeZone};
delete env.AGENTDECK_FIRSTLOAD_DAY_HINTS;
async function cli(state,args,dayHints=false){
 const e={...env};if(dayHints)e.AGENTDECK_FIRSTLOAD_DAY_HINTS='1';
 const t=now();const result=await run(binary,['--state-dir',state,'--format','json',...args],{env:e,cwd:root,maxBuffer:32*1024*1024});
 return {ms:now()-t,bytes:Buffer.from(result.stdout),value:JSON.parse(result.stdout)};
}
function canonical(v){if(Array.isArray(v))return v.map(canonical);if(v&&typeof v==='object')return Object.fromEntries(Object.entries(v).sort(([a],[b])=>a.localeCompare(b)).map(([k,x])=>[k,canonical(x)]));return v;}
function today(v){return v.data.usage.presentation.scopes.map(scope=>({client:scope.client,period:scope.periods.items.find(x=>x.period==='today')}));}
function digest(v){return sha(JSON.stringify(canonical(v)));}
function checkSnapshot(v){if(!v.data?.usage?.available||!today(v).find(x=>x.client==='all')?.period)throw Error('No valid today usage');}
const samples=[];let oracle,fullState;
for(let pair=0;pair<3;pair++){
 for(const variant of (pair%2?['today_hints','full']:['full','today_hints'])){
  const state=path.join(root,`fresh-v2-${pair}-${variant}`);if(fs.existsSync(state))throw Error('Requires empty fresh state');
  const hints=variant==='today_hints';
  const scan=await cli(state,['scan'],hints),snapshot=await cli(state,['desktop','snapshot']);
  checkSnapshot(snapshot.value);
  const scanned=scan.value.data.usage.changes.files;
  if(scanned!==(hints?selected.length:inventory.length))throw Error('Selection effect or full discovery not verified');
  const value={pair,variant,scan_ms:scan.ms,snapshot_ms:snapshot.ms,data_ready_ms:scan.ms+snapshot.ms,
   source_files:scanned,source_bytes:(hints?selected:inventory).reduce((n,x)=>n+x.bytes,0),today_projection_sha256:digest(today(snapshot.value)),
   snapshot_sha256:sha(snapshot.bytes)};
  if(variant==='full'){if(!oracle){oracle=today(snapshot.value);fullState=state;write('private-oracle-snapshot.json',snapshot.value);}else if(digest(oracle)!==value.today_projection_sha256)throw Error('Full baselines disagree');}
  if(oracle)value.today_matches_full=digest(oracle)===value.today_projection_sha256;
  samples.push(value);write('pilot-samples.json',samples);
  console.log(JSON.stringify({phase:'fresh_data_ready',...value}));
 }
}
const existing=[];
for(let pair=0;pair<5;pair++){
 const modes=pair%2?['readonly_snapshot','current_refresh']:['current_refresh','readonly_snapshot'];
 for(const mode of modes){
  const scan=mode==='current_refresh'?await cli(fullState,['scan']):null;
  const snap=await cli(fullState,['desktop','snapshot']);checkSnapshot(snap.value);
  if(digest(today(snap.value))!==digest(oracle))throw Error('Existing DB today differs');
  existing.push({pair,mode,scan_ms:scan?.ms??0,snapshot_ms:snap.ms,data_ready_ms:(scan?.ms??0)+snap.ms,today_matches_full:true});
 }
}
write('existing-samples.json',existing);
const summary={kind:'firstload-today-priority-pilot-v1',capture:JSON.parse(fs.readFileSync(path.join(root,'capture.json'),'utf8')),
 selection:{method:'cold only: local-day mtime hints plus Codex UTC-date directories intersecting local today',hinted_files:selected.length,hinted_bytes:selected.reduce((s,x)=>s+x.bytes,0),in_memory_manifest_selection_ms:selection_ms,discovery_included_in_scan:true},
 helper_sha256:sha(fs.readFileSync(binary)),experimental_discover_sha256:sha(fs.readFileSync(path.join(root,'repo/internal/ingest/ingest.go'))),
 first_install:samples,existing_db:existing,
 limitations:['Data readiness from CLI scan entry through complete snapshot JSON; not native popover/frame or full process launch','Day hints prioritize sources, not proof of complete day coverage; compare to full oracle on this corpus','Whole selected files are parsed to retain cumulative-token/session context; old files modified today are included','Other periods/history from hinted-only state are incomplete and must not be advertised as complete','Hint filter only runs against fresh empty isolated databases; never enable on existing database because missing sources could imply removal','Page cache not purged; 3 fresh-state pairs are pilot, not final tail acceptance','No OTel producer or event-update improvement measured']};
write('firstload-results.json',summary);
console.log(JSON.stringify({phase:'matrix_complete',first_install_pairs:3,existing_pairs:5,today_projection_matches:samples.filter(x=>x.variant==='today_hints').every(x=>x.today_matches_full)}));
