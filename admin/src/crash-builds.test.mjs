import assert from 'node:assert/strict';
import {crashBuildDate,crashBuildSeries} from './crash-builds.mjs';
const row=(Build,extra={})=>({Project:'cursemark',Build,OccurrenceVersion:1,Count:1,...extra});
const rows=[row('2026-11-22',{IngestedAt:'2020-01-01'}),row('2026-11-20T16:00:00Z'),row('2026-11-19',{IngestedAt:'2099-01-01'}),row('2026-11-20T08:00:00Z'),row('2026-11-19'),row('opaque-build-a'),row(''),row('2026-11-18',{OccurrenceVersion:0,Count:42}),row('2026-11-17',{OccurrenceVersion:0,Count:1})];
const before=JSON.stringify(rows),s=crashBuildSeries(rows);
assert.deepEqual(s.builds.map(b=>[b.build,b.count]),[['2026-11-19',2],['2026-11-20T08:00:00Z',1],['2026-11-20T16:00:00Z',1],['2026-11-22',1],['',1],['opaque-build-a',1]]);
assert.equal(s.recorded,7);assert.equal(s.legacyRows,2);assert.equal(s.legacyOccurrences,43);assert.equal(s.max,2);assert.equal(JSON.stringify(rows),before);
assert.equal(s.builds.some(b=>b.build==='2026-11-21'),false); // No complete release catalog: never invent a zero.
assert.equal(s.builds.some(b=>b.build==='2026-11-18'),false); // Legacy totals are not attributed to the first retained build.
assert.equal(s.builds.at(-1).date,null);
assert.equal(crashBuildSeries([row('2026-11-22'),row('2026-11-22')]).builds[0].count,2);
assert.deepEqual(crashBuildSeries([]),{builds:[],recorded:0,legacyRows:0,legacyOccurrences:0,max:1});
assert.equal(crashBuildSeries([row('2026-11-22',{OccurrenceVersion:0,Count:0})]).legacyOccurrences,0);
assert.equal(crashBuildSeries([row('same'),row('same',{Project:'necrovale'})]).builds.length,2);
for(const bad of ['','unknown','1234','11/20/2026','2026-02-30','2026-13-01','2026-11-22T25:00:00Z','2026-11-22T10:60:00Z','2026-11-22T10:00:60Z','2026-11-22T10:00:00+25:00','release-2026-11-22'])assert.equal(crashBuildDate(bad),null,bad);
assert.equal(crashBuildDate('2024-02-29').day,'2024-02-29');
assert.equal(crashBuildDate('2026-11-22 10:20:30').time,Date.UTC(2026,10,22,10,20,30));
assert.equal(crashBuildDate('2026-11-22T12:20:30+02:00').time,crashBuildDate('2026-11-22T10:20:30Z').time);
const precise=crashBuildSeries([row('2026-11-22T00:00:00.000000009Z'),row('2026-11-22T00:00:00.000000001Z')]);assert.equal(precise.builds[0].build,'2026-11-22T00:00:00.000000001Z');
assert.deepEqual(crashBuildSeries([...rows].reverse()),s);
console.log('PASS build-date ordering, per-build counts, same-day IDs, timezone/nanoseconds, unknowns, legacy exclusion, single/empty builds, scope and no invented zero builds');
