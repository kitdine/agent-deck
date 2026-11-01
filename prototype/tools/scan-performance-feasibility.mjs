// Isolated mechanism experiment. Not the product engine or native App acceptance.
import fs from "node:fs";
import path from "node:path";
import http from "node:http";
import crypto from "node:crypto";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { DatabaseSync } from "node:sqlite";

const run = promisify(execFile);
const root = path.resolve(process.argv[2] ?? "");
if (!root.startsWith("/private/tmp/agentdeck-scan-performance-proof.") || !fs.existsSync(path.join(root, "private-inventory.json"))) throw Error("An authorized private isolated corpus is required");
process.umask(0o077);
const home = path.join(root, "home"), state = path.join(root, "state"), binary = path.join(root, "agentdeck");
const env = { ...process.env, HOME:home, CODEX_HOME:path.join(home,".codex"), XDG_CONFIG_HOME:path.join(home,".config"), XDG_STATE_HOME:path.join(home,".local/state") };
const sha = b => crypto.createHash("sha256").update(b).digest("hex");
const now = () => performance.now();
const stats = values => { const v=[...values].sort((a,b)=>a-b); return {n:v.length,min:v[0],median:v[Math.ceil(v.length/2)-1],p95:v[Math.ceil(v.length*.95)-1],p99:v[Math.ceil(v.length*.99)-1],max:v.at(-1)}; };
const privateWrite = (name, value) => fs.writeFileSync(path.join(root,name),JSON.stringify(value,null,2)+"\n",{mode:0o600});
async function cli(args) {
  const start=now();
  const {stdout}=await run(binary,["--state-dir",state,"--format","json",...args],{cwd:root,env,maxBuffer:16*1024*1024});
  return {ms:now()-start,bytes:Buffer.from(stdout),value:JSON.parse(stdout)};
}
function identity() {
  const core=new DatabaseSync(path.join(state,"agentdeck.sqlite3"),{readOnly:true});
  const sessions=new DatabaseSync(path.join(state,"sessions.sqlite3"),{readOnly:true});
  try { core.exec("PRAGMA busy_timeout=50");sessions.exec("PRAGMA busy_timeout=50");
    return {core:core.prepare("SELECT CAST(epoch AS TEXT) AS epoch FROM derived_snapshot_generation WHERE singleton=1").get().epoch,
      sessions:sessions.prepare("SELECT CAST(epoch AS TEXT) AS epoch FROM session_index_generation WHERE singleton=1").get().epoch};
  } finally {core.close();sessions.close();}
}
function markerTotals() {
  const db=new DatabaseSync(path.join(state,"agentdeck.sqlite3"),{readOnly:true});
  try { return db.prepare("SELECT count(*) AS events,coalesce(sum(input_tokens+output_tokens),0) AS tokens FROM usage_events WHERE source_path=?").get(marker); }
  finally {db.close();}
}
function workerProfiles() {
  const p=path.join(state,"scan-receipts.json");if(!fs.existsSync(p))return [];
  const result=[];
  const visit=v=>{if(v&&typeof v==="object"){if(Object.hasOwn(v,"worker_cpu_time_ms"))result.push(v);for(const item of Object.values(v))visit(item);}};
  visit(JSON.parse(fs.readFileSync(p,"utf8")));return result;
}
let cached, publication=0;
const clients=new Set();
function publish(bytes) {
  if(!JSON.parse(bytes).data?.usage?.available)throw Error("Usage snapshot unavailable");
  cached=bytes;publication++;
  const document={identity:identity(),sha256:sha(bytes),payload:bytes.toString()};
  const temp=path.join(root,"serving-snapshot.tmp"),file=path.join(root,"serving-snapshot.json");
  const fd=fs.openSync(temp,"w",0o600);fs.writeFileSync(fd,JSON.stringify(document));fs.fsyncSync(fd);fs.closeSync(fd);fs.renameSync(temp,file);
  for(const client of clients)client.write(`data: ${JSON.stringify({publication,checksum:document.sha256})}\n\n`);
}
async function refresh() {const scan=await cli(["scan"]);const snapshot=await cli(["desktop","snapshot"]);publish(snapshot.bytes);return {scan_ms:scan.ms,snapshot_ms:snapshot.ms,total_ms:scan.ms+snapshot.ms,payload_sha256:sha(snapshot.bytes)};}

// Browser first-data-frame experiment, served only over a private loopback port.
if(process.argv.includes("--ui-only")) {
 const saved=JSON.parse(fs.readFileSync(path.join(root,"serving-snapshot.json"),"utf8"));cached=Buffer.from(saved.payload);
 if(sha(cached)!==saved.sha256||JSON.stringify(identity())!==JSON.stringify(saved.identity))throw Error("Invalid serving input");
 const html=String.raw`<!doctype html><meta charset="utf-8"><title>Isolated first-screen probe</title>
 <style>body{font:14px system-ui;padding:24px;background:#111827;color:#eef2ff}.panel{width:420px;border:1px solid #64748b;border-radius:12px;padding:18px;background:#1e293b}dt{color:#cbd5e1;margin-top:12px}dd{font-size:24px;margin:4px 0}button{padding:8px 14px}small{display:block;margin:14px 0;color:#cbd5e1}</style>
 <h1>浏览器首个可用数据帧实验</h1><small>授权真实日志副本，使用当前 Go CLI 的真实统计。此小型实验面板不等于完整原型或原生 macOS popover。</small>
 <button id="icon">模拟点击 icon</button><small id="status">正在准备内存输入</small><section class="panel" hidden id="panel"><strong>已采集用量 · 今日</strong><dl><dt>Tokens</dt><dd id="tokens"></dd><dt>Events</dt><dd id="events"></dd><dt>Sessions</dt><dd id="sessions"></dd></dl></section>
 <script>
 let memory;const panel=document.querySelector('#panel');
 fetch('/cached').then(r=>r.json()).then(v=>{memory=v;window.inputReady=true;document.querySelector('#status').textContent='已准备，可开始测试';});
 window.probeSamples=[];
 window.runFirstScreen=async(mode='memory')=>{
   if(!memory)throw Error('input not ready');panel.hidden=true;const start=performance.now();
   const v=mode==='memory'?memory:await fetch(mode==='restore'?'/restore':'/refresh').then(r=>r.json());
   const scope=v.data.usage.presentation.scopes.find(x=>x.client==='all');const today=scope.periods.items.find(x=>x.period==='today');
   if(!v.data.usage.available||!today)throw Error('no usable data');
   for(const key of ['tokens','events','sessions'])document.querySelector('#'+key).textContent=today.totals[key].toLocaleString();
   panel.hidden=false;await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
   const result={mode,click_to_first_data_frame_ms:performance.now()-start,data_visible:!panel.hidden&&document.querySelector('#tokens').textContent.length>0};window.probeSamples.push(result);return result;
 };
 document.querySelector('#icon').onclick=()=>window.runFirstScreen(new URLSearchParams(location.search).get('mode')||'memory').then(r=>document.querySelector('#status').textContent=r.click_to_first_data_frame_ms.toFixed(1)+' ms');
 </script>`;
 const uiServer=http.createServer(async(req,res)=>{
  try {
   if(req.url?.startsWith('/ui')){res.writeHead(200,{'Content-Type':'text/html; charset=utf-8'});res.end(html);return;}
   if(req.url==='/refresh')await refresh();
   if(req.url==='/restore'){const doc=JSON.parse(fs.readFileSync(path.join(root,'serving-snapshot.json'),'utf8'));if(JSON.stringify(identity())!==JSON.stringify(doc.identity)||sha(doc.payload)!==doc.sha256)throw Error('restore mismatch');cached=Buffer.from(doc.payload);}
   if(['/cached','/refresh','/restore'].includes(req.url)){res.writeHead(200,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(cached);return;}
   res.writeHead(404);res.end();
  }catch{res.writeHead(500);res.end('{"error":"isolated probe failed"}');}
 });
 await new Promise(resolve=>uiServer.listen(0,'127.0.0.1',resolve));
 privateWrite('ui-server.json',{port:uiServer.address().port,pid:process.pid});
 console.log(JSON.stringify({phase:'first_screen_server',port:uiServer.address().port}));
 await new Promise(()=>{});
}

const marker=path.join(home,".codex/sessions/2026/10/09/scan-performance-proof.jsonl");
fs.mkdirSync(path.dirname(marker),{recursive:true,mode:0o700});
fs.writeFileSync(marker,[{type:"session_meta",payload:{id:"performance-proof-session",session_id:"performance-proof-session",cwd:root}},
 {type:"turn_context",payload:{turn_id:"performance-proof-turn",model:"gpt-5"}}].map(v=>JSON.stringify(v)).join("\n")+"\n",{mode:0o600});
await refresh(); // Register the synthetic marker before measured unchanged samples.
const server=http.createServer((req,res)=>{
 if(req.url==="/snapshot"){res.writeHead(200,{"Content-Type":"application/json","Cache-Control":"no-store","X-Snapshot-SHA256":sha(cached)});res.end(cached);}
 else if(req.url==="/events"){res.writeHead(200,{"Content-Type":"text/event-stream","Cache-Control":"no-store"});clients.add(res);req.on("close",()=>clients.delete(res));}
 else {res.writeHead(404);res.end();}
});
await new Promise(resolve=>server.listen(0,"127.0.0.1",resolve));
const url=`http://127.0.0.1:${server.address().port}`;
async function cachedRead(expected=sha(cached)) {const start=now();const response=await fetch(url+"/snapshot");const bytes=Buffer.from(await response.arrayBuffer());if(sha(bytes)!==response.headers.get("X-Snapshot-SHA256")||(expected&&sha(bytes)!==expected)||!JSON.parse(bytes).data.usage.available)throw Error("Cached output differs");return now()-start;}
const baseline=[],reads=[],boot=[];
for(let pair=0;pair<3;pair++) {
 if(pair%2)for(let i=0;i<100;i++)reads.push(await cachedRead());
 const measurement=await refresh();baseline.push(measurement);
 console.log(JSON.stringify({phase:"unchanged_pair",pair,...measurement}));
 if(!(pair%2))for(let i=0;i<100;i++)reads.push(await cachedRead());
}
for(let i=0;i<30;i++) {
 const t=now(),saved=JSON.parse(fs.readFileSync(path.join(root,"serving-snapshot.json"),"utf8"));
 if(JSON.stringify(identity())!==JSON.stringify(saved.identity)||sha(saved.payload)!==saved.sha256||!JSON.parse(saved.payload).data.usage.available)throw Error("Restore verification failed");
 boot.push(now()-t);
}

privateWrite("pre-watch-metrics.json",{baseline,reads,boot});
let lastSize=fs.statSync(marker).size,active=null,expected=0,eventStarted=0;
const freshness=[],duringReads=[],watchFailures=[];
function startWork(trigger) {
 const size=fs.statSync(marker).size;if(size<=lastSize)return;lastSize=size;
 if(active)throw Error("Unexpected overlapping controlled stimulus");
 active=(async()=>{
  const timing=await refresh();const totals=markerTotals();
  if(totals.events!==expected||totals.tokens!==expected*133)throw Error("Injected delta not accounted exactly");
  const value={trigger,append_to_publication_ms:now()-eventStarted,...timing,exact_marker_accounting:true};freshness.push(value);
  console.log(JSON.stringify({phase:"file_event",stimulus:expected,...value}));
 })().finally(()=>{active=null;});
}
const watcher=fs.watch(marker,()=>startWork("filesystem_event"));
let sequence=0;
for(let i=0;i<3;i++) {
 expected=++sequence;eventStarted=now();const count={input_tokens:111,cached_input_tokens:0,output_tokens:22,total_tokens:133};
 const cumulative={input_tokens:111*sequence,cached_input_tokens:0,output_tokens:22*sequence,total_tokens:133*sequence};
 fs.appendFileSync(marker,JSON.stringify({timestamp:new Date().toISOString(),type:"event_msg",payload:{type:"token_count",info:{last_token_usage:count,total_token_usage:cumulative}}})+"\n");
 await new Promise(resolve=>setImmediate(resolve));
 const startPublication=publication;
 while(!active&&publication===startPublication&&now()-eventStarted<2000)await new Promise(resolve=>setTimeout(resolve,5));
 if(!active&&publication===startPublication){watchFailures.push({stimulus:expected,timeout_ms:2000});startWork("explicit_signal_after_notification_timeout");}
 const operation=active;
 for(let j=0;j<30;j++) {duringReads.push(await cachedRead(null));await new Promise(resolve=>setTimeout(resolve,10));}
 if(operation)await operation;
 while(active)await active;
}
watcher.close();
const idleStart=process.cpuUsage(),idleAt=now();await new Promise(resolve=>setTimeout(resolve,3000));const idleCPU=process.cpuUsage(idleStart);
const results={kind:"scan-performance-isolated-mechanism-probe-v1",corpus:JSON.parse(fs.readFileSync(path.join(root,"corpus-summary.json"))),
 binary_sha256:sha(fs.readFileSync(binary)),initial_import:JSON.parse(fs.readFileSync(path.join(root,"initial-scan-summary.json"))),
 paired_unchanged_refresh:baseline,unchanged_refresh_ms:stats(baseline.map(v=>v.total_ms)),
 prepared_read_ms:stats(reads),disk_restore_identity_decode_ms:stats(boot),file_event_updates:freshness,
 file_event_to_publication_ms:stats(freshness.map(v=>v.append_to_publication_ms)),prepared_read_during_update_ms:stats(duringReads),
 filesystem_notification_failures:watchFailures,node_peak_rss_bytes:process.resourceUsage().maxRSS*1024,worker_profiles:workerProfiles(),
 idle_probe:{duration_ms:now()-idleAt,cpu_ms:(idleCPU.user+idleCPU.system)/1000},
 stock_pipeline_readonly:false,isolated_state_only:true,output_byte_equivalence:true,marker_accounting:markerTotals(),
 scope:"Existing stock Go CLI scan+snapshot vs experimental prepared serving; finite real corpus plus three synthetic append stimuli. HTTP data delivery, not native click-to-frame. Not the proposed incremental engine or real OTel producer."};
privateWrite("results.json",results);
console.log(JSON.stringify({phase:"summary",unchanged_refresh_ms:results.unchanged_refresh_ms,prepared_read_ms:results.prepared_read_ms,disk_restore_ms:results.disk_restore_identity_decode_ms,event_ms:results.file_event_to_publication_ms,during_update_ms:results.prepared_read_during_update_ms,node_peak_rss_bytes:results.node_peak_rss_bytes}));
for(const client of clients)client.end();server.close();
