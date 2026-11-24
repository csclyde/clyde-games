const assert=require("node:assert/strict"),test=require("node:test"),path=require("node:path");
const sourceDir=process.env.CLYDE_BOT_TEST_SOURCE_DIR||path.join(__dirname,"../src");
const {snapshotForManual}=require(path.join(sourceDir,"forumFeedback"));
const {registerFeedbackReactionHandler,SUCCESS_EMOJI}=require(path.join(sourceDir,"discordFeedback"));
const {markSelected}=require(path.join(sourceDir,"feedbackBridge"));

function fixture(){
 const source={id:"100",guildId:"1169710427387744297",kind:"discord_text",projectId:"cursemark",enabled:true};
 const events=[],records=new Map(),errors=[];let submissions=0,checkFailures=0;
 const make=(id,channelId)=>({id,guildId:"1169710427387744297",channelId,createdTimestamp:1795219000000,content:"Synthetic fixture "+id,author:{id:"900",username:"fixture",bot:false},attachments:new Map(),reactions:{cache:new Map()},url:`https://discord.com/channels/1169710427387744297/${channelId}/${id}`});
 const starter=make("300","100"),reply=make("301","300"),otherReply=make("302","300");
 const thread={id:"300",guildId:"1169710427387744297",parentId:"100",type:11,archived:false,permissionsFor:()=>({has:()=>true}),messages:{fetch:async options=>{
  if(options.limit)return new Map([[reply.id,reply],[otherReply.id,otherReply]]);
  assert.equal(options.force,true);assert.equal(options.cache,false);
  return [reply,otherReply].find(m=>m.id===options.message);
 }}};
 const parent={id:"100",guildId:"1169710427387744297",type:0,permissionsFor:()=>({has:()=>true}),messages:{fetch:async options=>{assert.equal(options.message,starter.id);assert.equal(options.force,true);assert.equal(options.cache,false);return starter}}};
 starter.channel=parent;starter.hasThread=true;
 for(const m of [reply,otherReply]){m.channel=thread;m.react=async emoji=>{assert.equal(records.size,1,"check must follow durable intake");events.push(["react",m.id,emoji]);if(checkFailures-- >0)throw Error("mock check denied")}}
 const client={user:{id:"bot"},channels:{fetch:async id=>{events.push(["channel",id]);if(id===source.id)return parent;if(id===thread.id)return thread;throw Error("unrelated channel lookup")}}};
 const api={syncSources:async()=>({items:source.enabled?[source]:[],manual:{guild_id:"1169710427387744297",default_project:"",sources:[source]}}),submitSnapshot:async s=>{
  submissions++;events.push(["submit",s.id]);const key=`${s.feedback.Project}:${s.guild_id}:${s.channel_id}:${s.id}`;
  if(!records.has(key))records.set(key,{feedback_id:1,snapshot:s});return {id:s.id,durable:true,feedback_id:records.get(key).feedback_id};
 },markResult:async(id,result)=>events.push(["markResult",id,result.check_marked]),intakeStatus:async()=>({id:starter.id,guild_id:"1169710427387744297",channel_id:source.id,scope_id:source.id,project:source.projectId,durable:records.size===1,manual_trigger_id:reply.id})};
 return {source,events,records,errors,starter,reply,otherReply,thread,parent,client,api,submissions:()=>submissions,failChecks:n=>checkFailures=n};
}
function handler(f){let callback;registerFeedbackReactionHandler({...f.client,on:(_,cb)=>callback=cb},f.api,{}, {info(){},error:(label,metadata)=>f.errors.push({label,metadata})});return callback}

test("text-thread reply uses the configured project's parent starter identity and preserves flagged evidence",async()=>{
 const f=fixture(),s=await snapshotForManual(f.reply,{},f.api,f.client);
 assert.equal(s.id,f.starter.id);assert.equal(s.channel_id,f.source.id);assert.equal(s.scope_id,f.source.id);assert.equal(s.feedback.Project,"cursemark");
 assert.equal(s.trigger_message.id,f.reply.id);assert.equal(s.trigger_message.channel_id,f.thread.id);assert.equal(s.thread_context.length,2);
 assert.match(s.feedback.Message,/Manually flagged thread reply/);assert.match(s.feedback.Message,/Synthetic fixture 301/);
});
test("partial reaction and uncached text-thread channel hydrate safely before one durable intake and check",async()=>{
 const f=fixture();delete f.reply.channel;const run=handler(f);
 await run({partial:true,fetch:async()=>({emoji:{name:"\u{1FAB2}"},message:{partial:true,fetch:async()=>f.reply}})},{bot:false});
 assert.equal(f.errors.length,0);assert.equal(f.records.size,1);assert.equal(f.submissions(),1);
 assert(f.events.findIndex(e=>e[0]==="submit")<f.events.findIndex(e=>e[0]==="react"));
 assert.deepEqual(f.events.find(e=>e[0]==="react"),["react",f.reply.id,SUCCESS_EMOJI]);
});
test("concurrent/repeated flags and different replies retain one canonical ticket",async()=>{
 const f=fixture(),run=handler(f);const flag=m=>({emoji:{name:"\u{1FAB2}"},message:m});
 await Promise.all([run(flag(f.reply),{bot:false}),run(flag(f.reply),{bot:false})]);
 assert.equal(f.submissions(),1);
 await run(flag(f.reply),{bot:false});await run(flag(f.otherReply),{bot:false});
 assert.equal(f.errors.length,0);assert.equal(f.records.size,1);
 assert.deepEqual(f.events.filter(e=>e[0]==="submit").map(e=>e[1]),["300","300","300"]);
});
test("check failure keeps one ticket; existing selected retry marks only its public-thread reply",async()=>{
 const f=fixture(),run=handler(f);f.failChecks(1);await run({emoji:{name:"\u{1FAB2}"},message:f.reply},{bot:false});
 assert.equal(f.records.size,1);assert.equal(f.errors.length,1);assert(f.events.some(e=>e[0]==="markResult"&&e[2]===false));
 const marked=await markSelected(f.client,f.api,f.starter.id);assert.equal(marked.check_marked,true);assert.equal(f.records.size,1);assert.equal(f.submissions(),1);
 assert.equal(f.events.filter(e=>e[0]==="react").at(-1)[1],f.reply.id);
});
test("unverified or failed intake never gets a check, and bot self-reactions never intake",async()=>{
 for(const submitSnapshot of [async()=>({id:"300",durable:false,feedback_id:0}),async()=>{throw Error("mock API failure")}]){
  const f=fixture();f.api.submitSnapshot=submitSnapshot;const run=handler(f);
  await run({emoji:{name:"\u{1FAB2}"},message:f.reply},{bot:false});assert.equal(f.errors.length,1);assert.equal(f.events.some(e=>e[0]==="react"),false);
 }
 const f=fixture();await handler(f)({emoji:{name:"\u{1FAB2}"},message:f.reply},{bot:true});assert.equal(f.submissions(),0);assert.equal(f.events.length,0);
});
test("foreign-guild, unrelated-parent and unreadable threads remain rejected",async()=>{
 for(const change of [f=>f.source.guildId="999",f=>f.thread.parentId="999"]){
  const f=fixture();change(f);await assert.rejects(snapshotForManual(f.reply,{},f.api,f.client),/default project|unrelated channel|unavailable/);assert.equal(f.records.size,0);
 }
 const f=fixture();f.thread.permissionsFor=()=>({has:()=>false});await assert.rejects(snapshotForManual(f.reply,{},f.api,f.client),/permission/);
});
test("thread routing uses the persisted source project rather than the fallback project",async()=>{
 const f=fixture();f.source.projectId="necrovale";
 const snapshot=await snapshotForManual(f.reply,{feedbackProject:"cursemark"},f.api,f.client);
 assert.equal(snapshot.feedback.Project,"necrovale");assert.equal(snapshot.scope_id,f.source.id);
});
test("disabled monitoring and already-readable private text threads do not block explicit human flags",async()=>{
 const f=fixture();f.source.enabled=false;f.thread.type=12;
 const snapshot=await snapshotForManual(f.reply,{},f.api,f.client);assert.equal(snapshot.feedback.Project,"cursemark");
});
test("missing starter, foreign starter and retry channel substitution fail closed",async()=>{
 const f=fixture();f.parent.messages.fetch=async()=>({...f.starter,channelId:"other"});await assert.rejects(snapshotForManual(f.reply,{},f.api,f.client),/identity/);
 const g=fixture();g.parent.messages.fetch=async()=>{throw Error("mock 404")};await assert.rejects(snapshotForManual(g.reply,{},g.api,g.client),/mock 404/);
 const h=fixture();await h.api.submitSnapshot(await snapshotForManual(h.reply,{},h.api,h.client));h.thread.parentId="other";await assert.rejects(markSelected(h.client,h.api,h.starter.id),/scope/);assert.equal(h.events.some(e=>e[0]==="react"),false);
});
test("archived reply retry stays pending without unarchiving or marking",async()=>{
 const f=fixture();await f.api.submitSnapshot(await snapshotForManual(f.reply,{},f.api,f.client));f.thread.archived=true;
 const result=await markSelected(f.client,f.api,f.starter.id);assert.equal(result.check_marked,false);assert.match(result.error,/Archived thread/);assert.equal(f.events.some(e=>e[0]==="react"),false);assert.equal(f.thread.archived,true);
});
test("routing errors log only correlation IDs and classification, excluding API bodies and player text",async()=>{
 const f=fixture();f.api.submitSnapshot=async()=>{throw Error("Feedback API returned 403: PRIVATE_FIXTURE_BODY token=secret")};
 await handler(f)({emoji:{name:"\u{1FAB2}"},message:f.reply},{bot:false});
 const text=JSON.stringify(f.errors);assert.match(text,/Feedback API returned 403/);assert.match(text,/301/);assert.doesNotMatch(text,/PRIVATE_FIXTURE_BODY|token=secret|Synthetic fixture/);
});
