import assert from 'node:assert/strict';
import {initialAuxSettings as initial,auxiliarySettingsTransition as step,SETTING_SESSIONS} from '../src/auxiliarySettingsModel.js';
let checks=0;const test=(name,f)=>{f();checks++;};
const save=(m,action)=>step(m,{type:'save',action});
test('independent default and override revisions',()=>{const m=save(initial(),{type:'default',value:'auxiliary',expected_revision:0,request_id:'a'});assert.equal(m.defaultRevision,1);assert.equal(m.versions['claude:old-01'],2);});
test('core-only unmatched clear and tombstone',()=>{assert(!SETTING_SESSIONS.some(x=>x.id==='archived-1'));const m=save(initial(),{type:'override',key:'codex:archived-1',value:'auto',expected_revision:4,request_id:'clear'});assert.equal(m.versions['codex:archived-1'],5);assert.equal(m.overrides['codex:archived-1'],undefined);assert.equal(m.receipt.automatic,null);assert.equal(m.receipt.effective,null);assert.equal(m.receipt.classification_reason,'not_queried');});
test('no-change after CAS, not before',()=>{const m=save(initial(),{type:'override',key:'codex:archived-1',value:'auto',expected_revision:4,request_id:'a'});const ok=save(m,{type:'override',key:'codex:archived-1',value:'auto',expected_revision:5,request_id:'b'});assert.equal(ok.status,'noChange');assert.equal(ok.receipt.revision,5);const stale=save(m,{type:'override',key:'codex:archived-1',value:'auto',expected_revision:4,request_id:'c'});assert.equal(stale.status,'conflict');});
test('absent override remains absent',()=>{const m=save(initial(),{type:'override',key:'codex:missing',value:'auto',expected_revision:0,request_id:'a'});assert.equal(m.receipt.outcome,'no_change');assert.equal(m.receipt.revision,0);assert.equal(m.overrides['codex:missing'],undefined);});
test('old receipt cannot clear newer override',()=>{const a={type:'override',key:'claude:old-01',value:'auto',expected_revision:2,request_id:'clear'};let m=save(initial(),a);m=save(m,{type:'override',key:'claude:old-01',value:'auxiliary',expected_revision:3,request_id:'set'});m=save(m,a);assert.equal(m.status,'replayed');assert.equal(m.receipt.revision,3);assert.equal(m.versions['claude:old-01'],4);assert.equal(m.overrides['claude:old-01'],'auxiliary');});
test('request id payload collision',()=>{let m=save(initial(),{type:'default',value:'all',expected_revision:0,request_id:'same'});m=save(m,{type:'default',value:'work',expected_revision:1,request_id:'same'});assert.equal(m.status,'invalidRequest');assert.equal(m.defaultScope,'all');});
test('unknown reads original result once',()=>{let m=save(initial('saveUnknown'),{type:'default',value:'auxiliary',request_id:'u'});assert.equal(m.defaultScope,'work');m=step(m,{type:'read'});assert.equal(m.defaultRevision,1);assert.equal(m.receipt.request_id,'u');assert.equal(step(m,{type:'read'}).defaultRevision,1);});
test('missing receipt stays unknown',()=>{let m=save(initial('receiptMissing'),{type:'default',value:'auxiliary',request_id:'u'});m=step(m,{type:'read'});assert.equal(m.status,'saveUnknown');assert.equal(m.receipt.outcome,'unknown');assert.equal(m.defaultScope,'work');assert.equal(m.defaultRevision,0);});
test('conflict readback never replays draft',()=>{let m=save(initial('conflict'),{type:'default',value:'auxiliary'});m=step(m,{type:'read'});assert.equal(m.defaultScope,'all');assert.equal(m.defaultRevision,1);assert.equal(m.receipt,null);});
test('clear needs independent inspect',()=>{let m=step(initial(),{type:'inspect',key:'claude:old-01'});m=save(m,{type:'override',key:'claude:old-01',value:'auto'});assert.equal(m.inspected['claude:old-01'],undefined);m=step(m,{type:'inspect',key:'claude:old-01'});assert.equal(m.inspected['claude:old-01'],3);});
test('unconfirmed preference retains null',()=>{for(const state of ['loading','loadFailed']){const m=initial(state);assert.equal(m.defaultConfirmed,false);assert.equal(m.defaultScope,null);}});
test('missing receipt retry preserves identity and unlocks subsequent writes',()=>{
 for(const action of [{type:'default',value:'auxiliary'},{type:'override',key:'claude:old-01',value:'auto'}]){
  let m=save(initial('receiptMissing'),action);const pending=m.pending;const next=m.nextRequest;
  assert.equal(step(m,{type:'retry'}),m);m=step(m,{type:'read'});
  assert.equal(save(m,{type:'default',value:'all'}),m);
  m=step(m,{type:'retry'});assert.equal(m.status,'saved');assert.equal(m.pending,null);
  assert.equal(m.nextRequest,next);assert.equal(m.receipt.request_id,pending.request_id);
  assert.equal(m.receipts[pending.request_id].payload,JSON.stringify([pending.type,pending.key??null,pending.value,pending.expected_revision]));
  const revision=m.receipt.revision;assert.equal(step(m,{type:'retry'}),m);
  m=save(m,{type:'default',value:'all'});assert.equal(m.status,'saved');assert.equal(m.receipt.request_id,`fixture-request-${next}`);assert(revision>0);
 }
});
test('missing receipt retry returns old receipt without clearing newer override',()=>{
 const a={type:'override',key:'claude:old-01',value:'auto',request_id:'old',expected_revision:2};
 let m=step(save(initial('receiptMissing'),a),{type:'read'});
 let server=save(initial(),a);server=save(server,{type:'override',key:a.key,value:'auxiliary'});
 m=step({...m,serverState:server},{type:'retry'});assert.equal(m.status,'replayed');assert.equal(m.pending,null);assert.equal(m.receipt.revision,3);assert.equal(m.versions[a.key],4);assert.equal(m.overrides[a.key],'auxiliary');
});
test('missing receipt retry respects original CAS and conflict read ends pending',()=>{
 let m=step(save(initial('receiptMissing'),{type:'default',value:'auxiliary'}),{type:'read'});
 m=step({...m,defaultScope:'all',defaultRevision:1},{type:'retry'});assert.equal(m.status,'conflict');assert.equal(m.defaultScope,'all');assert.equal(m.pending.expected_revision,0);
 m=step(m,{type:'read'});assert.equal(m.status,'reloaded');assert.equal(m.pending,null);assert.equal(m.receipt,null);assert.equal(m.defaultScope,'all');
});
console.log(JSON.stringify({model_contract_checks:checks,failures:0,limit:'synthetic model only'}));
