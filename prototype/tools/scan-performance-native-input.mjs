// Prepare only an already-authorized isolated corpus. No live source access.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {DatabaseSync} from 'node:sqlite';
process.umask(0o077);
const root=path.resolve(process.argv[2]??'');
if(!root.startsWith('/private/tmp/agentdeck-macos-xctest.native-ab.')) throw Error('Private native root required');
const home=path.join(root,'home'), state=path.join(home,'.agentdeck');
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
const hashes=[];let files=0,bytes=0;
function visit(dir){for(const entry of fs.readdirSync(dir,{withFileTypes:true})){
 const p=path.join(dir,entry.name);
 if(entry.isDirectory())visit(p);
 else if(entry.isFile()&&entry.name.endsWith('.jsonl')){const b=fs.readFileSync(p);files++;bytes+=b.length;hashes.push(`${b.length}:${sha(b)}`);}
}}
for(const folder of ['.codex/sessions','.codex/archived_sessions','.claude/projects'])visit(path.join(home,folder));
const binary=path.join(root,'repo/apps/macos/build/agentdeck');
const payload=execFileSync(binary,['--state-dir',state,'--format','json','desktop','snapshot'],{
 env:{...process.env,HOME:home,CODEX_HOME:path.join(home,'.codex'),XDG_CONFIG_HOME:path.join(home,'.config'),XDG_STATE_HOME:path.join(home,'.local/state')},maxBuffer:16*1024*1024});
if(!JSON.parse(payload).data?.usage?.available)throw Error('No usable baseline usage');
function epoch(file,table){const db=new DatabaseSync(path.join(state,file),{readOnly:true});try{return db.prepare(`SELECT CAST(epoch AS TEXT) AS epoch FROM ${table} WHERE singleton=1`).get().epoch;}finally{db.close();}}
const identity={core:epoch('agentdeck.sqlite3','derived_snapshot_generation'),sessions:epoch('sessions.sqlite3','session_index_generation')};
fs.writeFileSync(path.join(home,'proof-saved.json'),JSON.stringify({...identity,sha256:sha(payload),payload:payload.toString('base64')}),{mode:0o600});
const summary={files,bytes,corpus_digest:sha(hashes.sort().join('\n')),corpus_digest_algorithm:'SHA256 of sorted size:content-SHA256 entries joined by LF, no final LF',stock_helper_sha256:sha(fs.readFileSync(binary)),saved_envelope_sha256:sha(payload),saved_envelope_bytes:payload.length,base_head:'32df0ae1223796f1e43bbd45a40f1efb9f7ff56e',corpus_is_frozen:true};
fs.writeFileSync(path.join(root,'input-summary.json'),JSON.stringify(summary,null,2)+'\n',{mode:0o600});
console.log(JSON.stringify(summary));
