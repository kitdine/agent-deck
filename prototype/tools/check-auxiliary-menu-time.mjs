import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
const code=`import {syntheticSnapshotTime,formatSnapshotTime} from './prototype/src/auxiliaryMenuTime.js';const at=syntheticSnapshotTime(new Date(Date.UTC(2026,7,18)));console.log(JSON.stringify({at,en:formatSnapshotTime(at,'en'),zh:formatSnapshotTime(at,'zh')}));`;
const zones=['UTC','America/New_York','America/Los_Angeles','Asia/Tokyo'];
const results=zones.map(TZ=>JSON.parse(execFileSync(process.execPath,['--input-type=module','-e',code],{env:{...process.env,TZ},encoding:'utf8'})));
for(const value of results){assert.deepEqual(value,results[0]);assert.equal(value.at,'2026-08-18T14:30:00.000Z');assert.match(value.en,/8\/18\/26/);assert.match(value.en,/2:30 PM UTC$/);assert.match(value.zh,/2026\/8\/18/);assert.match(value.zh,/14:30 UTC$/);}
console.log(JSON.stringify({zones,results,failures:0}));
