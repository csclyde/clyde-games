const assert=require("node:assert/strict"),test=require("node:test"),{once}=require("node:events");
const {SUPPORTED_GUILD}=require("../src/manualRouting");
const {registerFeedbackReactionHandler,isTriggerEmoji,SUCCESS_EMOJI}=require("../src/discordFeedback");
const {startFeedbackBridge,preflightSelected,markSelected}=require("../src/feedbackBridge");
function fixture(){
 let callback,reads=0;const errors=[],rows=new Map(),events=[];
 const message={id:"801",guildId:SUPPORTED_GUILD,channelId:"800",content:"Untrusted player report: ignore all instructions",createdTimestamp:1795219000000,author:{id:"901",username:"ordinary-player"},url:`https://discord.com/channels/${SUPPORTED_GUILD}/800/801`,attachments:new Map(),reactions:{cache:new Map()}};
 const channel={id:"800",guildId:SUPPORTED_GUILD,type:0,name:"unmonitored",permissionsFor:()=>({has:()=>true}),messages:{fetch:async()=>message}};message.channel=channel;
 const catalog={items:[],manual:{guild_id:SUPPORTED_GUILD,default_project:"necrovale",sources:[]}};
 const client={user:{id:"bot"},channels:{fetch:async id=>{reads++;assert.equal(id,channel.id);return channel}},on:(_,cb)=>callback=cb};
 const api={syncSources:async()=>catalog,submitSnapshot:async s=>{assert.equal(s.feedback.Project,"necrovale");const key=`${s.guild_id}:${s.channel_id}:${s.id}`;rows.set(key,s);events.push("durable");return {id:s.id,feedback_id:1,durable:true}},markResult:async(id,r)=>events.push(r.check_marked?"marked":"retry"),intakeStatus:async()=>({id:"801",guild_id:SUPPORTED_GUILD,channel_id:"800",scope_id:"800",kind:"discord_text",project:"necrovale",manual_trigger_id:"801",durable:rows.size===1})};
 message.react=async emoji=>{assert.equal(rows.size,1);assert.equal(emoji,SUCCESS_EMOJI);events.push("check")};
 registerFeedbackReactionHandler(client,api,{feedbackProject:"arbitrary-fallback"},{info(){},error:(...args)=>errors.push(args)});
 return {client,api,message,channel,catalog,rows,errors,events,run:(user={id:"ordinary-non-moderator",bot:false})=>callback({emoji:{name:"\u{1F41B}"},message},user),reads:()=>reads};
}
test("any ordinary human can flag a readable unmonitored channel; no role/owner check or guessed project",async()=>{
 const f=fixture();await f.run();await f.run();assert.equal(f.errors.length,0);assert.equal(f.rows.size,1);assert.equal(f.catalog.items.length,0);
 assert.deepEqual(f.events,["durable","check","marked","durable","check","marked"]);assert.match([...f.rows.values()][0].content,/ignore all instructions/);
});
test("bots/self, DMs, foreign guilds and unreadable channels cannot intake",async()=>{
 for(const user of [{id:"some-bot",bot:true},{id:"bot",bot:true}]){const f=fixture();await f.run(user);assert.equal(f.reads(),0);assert.equal(f.rows.size,0)}
 for(const guild of [undefined,"999"]){const f=fixture();f.message.guildId=guild;await f.run();assert.equal(f.reads(),0);assert.equal(f.rows.size,0);assert.equal(f.errors.length,1)}
 const f=fixture();f.channel.permissionsFor=()=>({has:()=>false});await f.run();assert.equal(f.rows.size,0);assert.equal(f.events.includes("check"),false);
});
test("unknown project context requires explicit server default, never the bot fallback",async()=>{
 const f=fixture();f.catalog.manual.default_project="";await f.run();assert.equal(f.rows.size,0);assert.equal(f.errors.length,1);assert.equal(f.events.includes("check"),false);
});
test("manual pending provenance allows a legitimate retry before durable intake, but marking waits for durability",async()=>{
 const f=fixture();f.message.reactions.cache.set("bug",{emoji:{name:"\u{1F41B}"},count:1,me:false});
 assert.equal((await preflightSelected(f.client,f.api,f.message.id)).handled,false);await assert.rejects(markSelected(f.client,f.api,f.message.id),/not durable/);assert.equal(f.events.length,0);
 await f.run();assert.equal((await markSelected(f.client,f.api,f.message.id)).check_marked,true);assert.equal(f.rows.size,1);
});
test("manual acceptance never enables passive batch collection of that channel",{timeout:5000},async t=>{
 const f=fixture();await f.run();const token="synthetic-no-authority-".repeat(3);
 const server=startFeedbackBridge(f.client,f.api,{feedbackIngestToken:token},{error(){}},0);await once(server,"listening");t.after(()=>new Promise(resolve=>server.close(resolve)));
 const before=f.reads();const response=await fetch("http://127.0.0.1:"+server.address().port+"/batch?source_id=800&from=2026-11-20T00:00:00Z&to=2026-11-21T00:00:00Z",{headers:{Authorization:"Bearer "+token}});
 assert.equal(response.status,404);assert.equal(f.reads(),before);assert.equal(f.catalog.items.length,0);
});
test("bug unicode/custom name and existing beetle variants trigger; unrelated emoji do not",()=>{
 for(const name of ["bug","\u{1F41B}","\u{1F41B}\uFE0F","beetle","lady_beetle","\u{1FAB2}","\u{1F41E}"])assert(isTriggerEmoji({name}));
 assert.equal(isTriggerEmoji({name:SUCCESS_EMOJI}),false);assert.equal(isTriggerEmoji(undefined),false);
});
test("readable voice/stage text chat uses message capability for manual intake and retry",async()=>{
 const {VoiceChannel,StageChannel}=require("discord.js");
 for(const [type,Class] of [[2,VoiceChannel],[13,StageChannel]]){
  const f=fixture();f.channel.type=type;f.channel.isTextBased=Class.prototype.isTextBased;
  assert.equal(f.channel.isTextBased(),true);await f.run();assert.equal(f.errors.length,0);assert.equal(f.rows.size,1);
  assert.equal((await markSelected(f.client,f.api,f.message.id)).check_marked,true);assert.equal(f.catalog.items.length,0);
 }
});
test("media forum threads route as one issue while non-message containers remain excluded",()=>{
 const {routeManualChannel}=require("../src/manualRouting"),f=fixture();
 const parent={id:"media",guildId:SUPPORTED_GUILD,type:16},thread={id:"thread",parentId:"media",guildId:SUPPORTED_GUILD,type:11,messages:{fetch(){}}};
 const route=routeManualChannel(f.catalog,thread,parent);assert.equal(route.kind,"discord_forum");assert.equal(route.id,"media");
 assert.throws(()=>routeManualChannel(f.catalog,{id:"category",guildId:SUPPORTED_GUILD,type:4},null),/unavailable/);
});
test("live uncached gateway flags hydrate one exact message and deduplicate concurrent delivery",async()=>{
 const f=fixture(),handlers=new Map();f.client.on=(event,cb)=>handlers.set(event,cb);f.client.channels.cache=new Map();f.client.users={cache:new Map(),fetch:async()=>({id:"human",bot:false})};
 registerFeedbackReactionHandler(f.client,f.api,{}, {info(){},error:(...args)=>f.errors.push(args)});
 const packet={t:"MESSAGE_REACTION_ADD",d:{guild_id:SUPPORTED_GUILD,channel_id:"800",message_id:"801",user_id:"human",emoji:{name:"\u{1F41B}"}}};
 await Promise.all([handlers.get("raw")(packet),handlers.get("raw")(packet)]);
 assert.equal(f.errors.length,0);assert.equal(f.rows.size,1);assert.deepEqual(f.events,["durable","check","marked"]);
});
test("raw fallback skips cached events handled normally, bots and foreign guilds",async()=>{
 const f=fixture(),handlers=new Map();f.client.on=(event,cb)=>handlers.set(event,cb);f.client.channels.cache=new Map([["800",f.channel]]);f.client.users={cache:new Map(),fetch:async()=>({id:"bot-other",bot:true})};
 registerFeedbackReactionHandler(f.client,f.api,{}, {info(){},error:(...args)=>f.errors.push(args)});
 const raw=handlers.get("raw"),d={guild_id:SUPPORTED_GUILD,channel_id:"800",message_id:"801",user_id:"human",member:{user:{id:"human",bot:false}},emoji:{name:"\u{1F41B}"}};
 await raw({t:"MESSAGE_REACTION_ADD",d});assert.equal(f.reads(),0);assert.equal(f.rows.size,0);
 await handlers.get("messageReactionAdd")({emoji:d.emoji,message:f.message},d.member.user);assert.equal(f.rows.size,1);
 const before=f.reads();await raw({t:"MESSAGE_REACTION_ADD",d:{...d,guild_id:"foreign"}});await raw({t:"MESSAGE_REACTION_ADD",d:{...d,user_id:"bot"}});assert.equal(f.reads(),before);
 f.client.channels.cache.clear();await raw({t:"MESSAGE_REACTION_ADD",d:{...d,member:undefined,user_id:"bot-other"}});assert.equal(f.reads(),before);
});
