// Existing DB, with and without derived cache. No live source access or scan.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
process.umask(0o077);
const run=promisify(execFile),root=path.resolve(process.argv[2]??'');
if(!/^\/private\/tmp\/agentdeck-scan-performance-firstload\.[A-Za-z0-9]+$/.test(root))throw Error('Private firstload root required');
const home=path.join(root,'home'),state=path.join(root,'fresh-v2-0-full'),binary=path.join(root,'agentdeck');
const env={...process.env,HOME:home,CODEX_HOME:path.join(home,'.codex'),TZ:'America/New_York'};
delete env.AGENTDECK_FIRSTLOAD_DAY_HINTS;
const oracle=JSON.parse(fs.readFileSync(path.join(root,'private-oracle-snapshot.json'),'utf8'));
const expected=oracle.data.usage.presentation.scopes.find(x=>x.client==='all').periods.items.find(x=>x.period==='today').totals;
async function cli(args){const t=performance.now();const r=await run(binary,['--state-dir',state,'--format','json',...args],{env,maxBuffer:32*1024*1024});return {ms:performance.now()-t,value:JSON.parse(r.stdout)};}
const samples=[];
for(const condition of ['derived_cache_present','derived_cache_absent']){
 for(let pair=0;pair<3;pair++){
  for(const mode of(pair%2?['today_query','full_snapshot']:['full_snapshot','today_query'])){
   const file=path.join(state,'desktop-derived-cache.json');
   let cacheRemoved=false;
   if(condition==='derived_cache_absent'&&fs.existsSync(file)){fs.unlinkSync(file);cacheRemoved=true;}
   const result=await cli(mode==='today_query'?['usage','stats','--period','today','--no-scan']:['desktop','snapshot']);
   const totals=mode==='today_query'?result.value.data.totals:result.value.data.usage.presentation.scopes.find(x=>x.client==='all').periods.items.find(x=>x.period==='today').totals;
   if(!['tokens','events','sessions','known_catalog_base_cost','known_provider_cost'].every(k=>JSON.stringify(totals[k])===JSON.stringify(expected[k])))throw Error('Stored today totals differ');
   const value={condition,pair,mode,data_ready_ms:result.ms,today_totals_equal:true,cache_removed:cacheRemoved,
    result_scope:mode==='today_query'?'today usage statistics only; not a full popover envelope':'complete desktop snapshot'};
   samples.push(value);console.log(JSON.stringify(value));
  }
 }
}
const result={kind:'existing-db-today-bootstrap-pilot-v1',samples,counts_checked:['tokens','events','sessions','known_catalog_base_cost','known_provider_cost'],
 snapshot_cache:'complete serving envelope absent throughout; derived cache presence varied explicitly',
 limits:['CLI data preparation, not native frame','today_query covers usage totals/models/trend, not complete quota/provider/session/work-signals UI','No log changes; all required today events already committed; unimported today data is a separate scenario','3 pilot pairs per derived-cache condition; OS page cache warm; no final p95 claim'],
 helper_sha256:crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex')};
fs.writeFileSync(path.join(root,'db-bootstrap-results.json'),JSON.stringify(result,null,2)+'\n',{mode:0o600});
