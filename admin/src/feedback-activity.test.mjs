import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createFeedbackActivity,startNavigationPolling} from './feedback-activity.mjs';
const row=(id,ids,complete=true)=>({id,openFeedbackIds:ids.map(String),feedbackIdsComplete:complete});
const memory=()=>{const data=new Map();return {getItem:k=>data.get(k),setItem:(k,v)=>data.set(k,v)}};
test('exact ID acknowledgements survive reload, net-zero counts, late commits and account separation',()=>{
 const storage=memory(),a=createFeedbackActivity({scope:'user:1',storage});
 assert.equal(a.ingest([row('cursemark',[20]),row('necrovale',[])]).any,false);
 assert.equal(a.ingest([row('cursemark',[22]),row('necrovale',[])]).any,true);
 a.review('cursemark',[22]);assert.equal(a.state().any,false);
 assert.equal(a.ingest([row('cursemark',[22,21])]).any,true,'lower ID committed later must notify');
 a.review('cursemark',[22]);assert.equal(a.state().any,true,'viewing a different card cannot clear unseen');
 const reload=createFeedbackActivity({scope:'user:1',storage});assert.equal(reload.ingest([row('cursemark',[22,21])]).any,true);
 reload.review('cursemark',[21]);assert.equal(reload.state().any,false);
 const other=createFeedbackActivity({scope:'user:2',storage});assert.equal(other.ingest([row('cursemark',[22,21])]).any,false);assert.notEqual(a.key,other.key);
 assert.equal(a.ingest([row('cursemark',[22,21]),row('new-game',[31])]).unseen['new-game'],true);
});
test('incomplete or malformed activity never silently clears; inaccessible storage remains functional',()=>{
 const a=createFeedbackActivity({scope:'legacy',storage:{getItem(){throw Error()},setItem(){throw Error()}}});
 assert.equal(a.ingest([row('cursemark',[1],false)]).any,true);a.review('cursemark',[1]);assert.equal(a.state().any,true);
 assert.equal(a.ingest([row('cursemark',[1])]).any,false);assert.equal(a.state().persistent,false);
 a.ingest([row('cursemark',[2])]);assert.throws(()=>a.ingest([row('cursemark',['invalid'])]));assert.equal(a.state().any,true);
});
test('cross-tab review merges without acknowledging unknown arrivals',()=>{
 const storage=memory(),a=createFeedbackActivity({scope:'user:1',storage});a.ingest([row('cursemark',[1])]);a.ingest([row('cursemark',[2,3])]);
 const b=createFeedbackActivity({scope:'user:1',storage});b.ingest([row('cursemark',[2,3])]);b.review('cursemark',[2]);a.merge(storage.getItem(a.key));assert.equal(a.state().any,true);a.review('cursemark',[3]);assert.equal(a.state().any,false);
});
test('poller coalesces catch-up, uses background delay, retains state on failure and stops cleanly',async t=>{
 t.mock.timers.enable({apis:['setTimeout']});
 const doc=new EventTarget();doc.visibilityState='visible';const win=new EventTarget(),nav={onLine:true};let calls=0,applied=0,errors=0,finish;
 const polling=startNavigationPolling({load:()=>{calls++;return new Promise(r=>finish=r)},apply:()=>applied++,onError:()=>errors++,document:doc,window:win,navigator:nav});
 assert.equal(calls,1);win.dispatchEvent(new Event('focus'));win.dispatchEvent(new Event('online'));t.mock.timers.tick(100);assert.equal(calls,1);
 finish([]);await Promise.resolve();await Promise.resolve();t.mock.timers.tick(0);assert.equal(calls,2);finish([]);await Promise.resolve();await Promise.resolve();
 doc.visibilityState='hidden';doc.dispatchEvent(new Event('visibilitychange'));t.mock.timers.tick(59999);assert.equal(calls,2);t.mock.timers.tick(1);assert.equal(calls,3);
 nav.onLine=false;win.dispatchEvent(new Event('offline'));assert.equal(errors,1);polling.stop();finish([]);await Promise.resolve();await Promise.resolve();t.mock.timers.tick(300000);assert.equal(calls,3);assert.equal(applied,2);
});

test("project IDs cannot collide with object prototypes",()=>{const a=createFeedbackActivity({scope:"legacy",storage:memory()});a.ingest([row("constructor",[1]),row("__proto__",[])]);assert.equal(a.ingest([row("constructor",[1,2]),row("__proto__",[3])]).any,true);a.review("constructor",[2]);a.review("__proto__",[3]);assert.equal(a.state().any,false);});

test("concurrent tab acknowledgements persist their union",()=>{const storage=memory(),a=createFeedbackActivity({scope:"user:1",storage});a.ingest([row("cursemark",[1])]);const b=createFeedbackActivity({scope:"user:1",storage});a.ingest([row("cursemark",[2,3])]);b.ingest([row("cursemark",[2,3])]);a.review("cursemark",[2]);b.review("cursemark",[3]);a.merge(storage.getItem(a.key));const reload=createFeedbackActivity({scope:"user:1",storage});assert.equal(reload.ingest([row("cursemark",[2,3])]).any,false);});
