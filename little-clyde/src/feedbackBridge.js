const http=require("node:http");
const {timingSafeEqual}=require("node:crypto");
const {buildFeedbackPayload,SUCCESS_EMOJI}=require("./discordFeedback");
const {hasBug,ownBug,threadScope,collectForum}=require("./forumFeedback");
const BUG_EMOJI="\u{1FAB2}";
async function snapshotMessage(message, config, relatedThread) {
  const attachments = [...(message.attachments?.values?.() || [])].map(a => ({
    id: a.id, name: a.name, url: a.url, proxy_url: a.proxyURL, size: a.size, content_type: a.contentType,
  }));
  const snapshot = {
    id: message.id, guild_id: message.guildId, channel_id: message.channelId,
    timestamp: new Date(message.createdTimestamp).toISOString(), edited_timestamp: message.editedTimestamp ? new Date(message.editedTimestamp).toISOString() : null,
    author: { id: message.author?.id, username: message.author?.username, tag: message.author?.tag, bot: Boolean(message.author?.bot) },
    content: message.content || "", url: message.url, attachments,
    reactions: [...(message.reactions?.cache?.values?.() || [])].map(r => ({ name: r.emoji.name, id: r.emoji.id, count: r.count, me: r.me })),
    reference: message.reference || null,
    feedback: buildFeedbackPayload(message, { ...config, feedbackProject: config.feedbackProject }),
  };
  // Related context only. Never follow a reference into a different channel.
  if (message.reference?.channelId === message.channelId && message.reference?.messageId) {
    try {
      const reply = await message.channel.messages.fetch({message:message.reference.messageId,force:true,cache:false});
      snapshot.reply_context = { id: reply.id, content: reply.content, author_id: reply.author?.id, timestamp: new Date(reply.createdTimestamp).toISOString(), url: reply.url };
    } catch { snapshot.reply_context_unavailable = true; }
  }
  const thread = relatedThread || (message.hasThread ? message.thread : null);
  if (thread?.id === message.id && thread?.parentId === message.channelId && thread?.guildId === message.guildId) {
    try {
      const replies = await thread.messages.fetch({ limit: 21 });
      snapshot.thread_context = [...replies.values()].slice(0, 20).map(m => ({ id: m.id, content: m.content, author_id: m.author?.id, timestamp: new Date(m.createdTimestamp).toISOString(), url: m.url, attachments: [...(m.attachments?.values?.() || [])].map(a => ({ name: a.name, url: a.url, size: a.size })) }));
      snapshot.thread_context_truncated = replies.size > 20;
    } catch { snapshot.thread_context_unavailable = true; }
  }
  return snapshot;
}


async function scopedChannel(client,source,manual=false){const channel=await client.channels.fetch(source.id);if(channel?.guildId!==source.guildId||channel.id!==source.id||!(manual?require("./manualRouting").isManualMessageChannel(channel):[0,5].includes(channel.type)))throw new Error("Discord source mismatch");if(!channel.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"]))throw new Error("Discord history permission unavailable");return channel}
async function collectBatch(client,config,source,from,to){if(!Number.isFinite(from)||!Number.isFinite(to)||from>to)throw new Error("Invalid collection window");if(source.kind==="discord_forum"){const batch=await collectForum(client,config,source,from,to);return {...batch,source_id:source.id}}
 const channel=await scopedChannel(client,source);const messages=[];let before;const seen=new Set();
 for(let page=0;page<20;page++){const fetched=await channel.messages.fetch({limit:100,...(before?{before}:{})});if(!fetched.size)return {source_id:source.id,messages,complete:true};const ordered=[...fetched.values()].sort((a,b)=>BigInt(a.id)<BigInt(b.id)?-1:1);let reached=false;for(const m of ordered){if(m.guildId!==source.guildId||m.channelId!==source.id)throw new Error("Message scope mismatch");if(m.createdTimestamp<Math.max(from,Date.parse(source.baselineAt))){reached=true;continue};if(m.createdTimestamp>to||m.author?.bot||hasBug(m))continue;const snapshot=await snapshotMessage(m,{...config,feedbackProject:source.projectId});snapshot.kind="discord_text";snapshot.scope_id=source.id;messages.push(snapshot)}if(reached||fetched.size<100)return {source_id:source.id,messages,complete:true};before=ordered[0]?.id;if(!before||seen.has(before))throw new Error("Discord pagination stalled");seen.add(before)}throw new Error("Discord exceeds 20 message pages")}
async function selectedTarget(client,feedbackClient,id){
 if(!/^\d{1,22}$/.test(id))throw new Error("Invalid message ID");
 const status=await feedbackClient.intakeStatus(id),catalog=await feedbackClient.syncSources();
 let source;
 if(status.manual_trigger_id){
  const {manualPolicy}=require("./manualRouting");const policy=manualPolicy(catalog);
  if(status.guild_id!==policy.guild_id)throw new Error("Manual guild mismatch");
  source={id:status.scope_id,guildId:status.guild_id,projectId:status.project,kind:status.kind==="forum_thread"?"discord_forum":"discord_text"};
 }else source=catalog.items.find(s=>s.enabled&&s.id===status.scope_id&&s.projectId===status.project&&s.guildId===status.guild_id);
 if(!source)throw new Error("Source no longer enabled");
 let channel;if(source.kind==="discord_forum"){channel=await client.channels.fetch(status.channel_id);if(!threadScope(channel,source)||channel.id!==id)throw new Error("Forum identity mismatch")}else{channel=await scopedChannel(client,source,Boolean(status.manual_trigger_id));if(status.channel_id!==source.id)throw new Error("Text identity mismatch")}
 const message=await channel.messages.fetch({message:id,force:true,cache:false});return {status,source,channel,message};
}
async function preflightSelected(client,feedbackClient,id){const {status,message}=await selectedTarget(client,feedbackClient,id);return {handled:hasBug(message)&&!(status.manual_trigger_id||(status.durable&&ownBug(message))),durable:Boolean(status.durable)}}
async function manualCheckTarget(client, status, source, channel, message) {
 if (!status.manual_trigger_id || status.manual_trigger_id === message.id) return {channel, message};
 if (source.kind === "discord_forum") return {channel, message:await channel.messages.fetch({message:status.manual_trigger_id,force:true,cache:false})};
 // A text-channel thread shares its starter ID with its parent-channel message.
 // Resolve only that exact public thread; never search other channels for a reply.
 const thread=await client.channels.fetch(message.id);
 if (thread?.id!==message.id || thread.guildId!==source.guildId || thread.parentId!==source.id || ![10,11,12].includes(thread.type)) throw new Error("Manual reply thread scope mismatch");
 if (!thread.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"])) throw new Error("Manual reply history permission unavailable");
 const target=await thread.messages.fetch({message:status.manual_trigger_id,force:true,cache:false});
 if (target.guildId!==source.guildId || target.channelId!==thread.id || target.id!==status.manual_trigger_id) throw new Error("Manual reply identity mismatch");
 return {channel:thread,message:target};
}
async function markSelected(client,feedbackClient,id){const {status,source,channel,message}=await selectedTarget(client,feedbackClient,id);if(!status.durable)throw new Error("Intake is not durable");const result={bug_marked:false,check_marked:false,error:"",handled:false};if(hasBug(message)&&!ownBug(message)&&!status.manual_trigger_id)return {...result,handled:true};if(channel.archived)return {...result,error:"Archived thread: reactions unavailable; retained for retry without unarchiving"};const checked=await manualCheckTarget(client,status,source,channel,message);if(checked.channel.archived)return {...result,error:"Archived thread: reactions unavailable; retained for retry without unarchiving"};const target=checked.message;
 try{if(!status.manual_trigger_id&&!hasBug(message))await message.react(BUG_EMOJI);result.bug_marked=hasBug(message)||!status.manual_trigger_id}catch{return {...result,error:"Bug reaction failed; retry intake"}}
 try{await target.react(SUCCESS_EMOJI);result.check_marked=true}catch{result.error="Check reaction failed; retry intake"};return result}
const COMPLETION_EMOJI = "\u{1F6E0}\uFE0F";

// Called only by the authenticated, loopback feedback outbox. These IDs come
// from the durable intake record: forum starters are their own thread IDs.
async function markFeedbackCompleted(client, guildId, channelId, messageId) {
 const channel = await client.channels.fetch(channelId);
 if (channel?.id !== channelId || channel.guildId !== guildId ||
     typeof channel.messages?.fetch !== "function" ||
     (![0,5,10,11,12].includes(channel.type) && !require("./manualRouting").isManualMessageChannel(channel))) {
  throw new Error("Completion channel identity mismatch");
 }
 if (channel.archived || !channel.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"])) {
  throw new Error("Completion reaction unavailable for channel");
 }
 const message = await channel.messages.fetch({message:messageId,force:true,cache:false});
 if (message?.id !== messageId || message.guildId !== guildId || message.channelId !== channelId) {
  throw new Error("Completion message identity mismatch");
 }
 const ours = [...(message.reactions?.cache?.values?.() || [])].some(r =>
  r.me && r.emoji?.name?.replace(/\uFE0F/g,"") === "\u{1F6E0}");
 // Discord stores one reaction per emoji/bot; after an ambiguous failure the
 // fresh read avoids a redundant request and the reaction itself stays unique.
 if (!ours) await message.react(COMPLETION_EMOJI);
 return {applied:true};
}
async function sourceCatalog(client){const items=[];for(const guild of client.guilds.cache.values()){const channels=await guild.channels.fetch();for(const c of channels.values()){if(!c||![0,5,15].includes(c.type))continue;const p=c.permissionsFor(client.user);items.push({id:c.id,guildId:guild.id,kind:c.type===15?"discord_forum":"discord_text",name:c.name,parentId:c.parentId,canRead:Boolean(p?.has(["ViewChannel","ReadMessageHistory"])),canReact:Boolean(p?.has("AddReactions"))})}}return {items}}
function startFeedbackBridge(client,feedbackClient,config,logger=console,listenPort=9992){const token=config.feedbackIngestToken;if(!token||token.length<32){logger.error("Feedback bridge disabled: token unavailable");return null};const expected=Buffer.from(`Bearer ${token}`);const server=http.createServer(async(req,res)=>{const supplied=Buffer.from(req.headers.authorization||"");const send=(status,value)=>{res.writeHead(status,{"Content-Type":"application/json","Cache-Control":"no-store"});res.end(JSON.stringify(value))};if(supplied.length!==expected.length||!timingSafeEqual(supplied,expected))return send(401,{error:"unauthorized"});if(req.headers.origin)return send(403,{error:"browser origins forbidden"});try{const u=new URL(req.url,"http://127.0.0.1");if(req.method==="GET"&&u.pathname==="/source-catalog")return send(200,await sourceCatalog(client));if(req.method==="GET"&&u.pathname==="/batch"){const source=(await feedbackClient.syncSources()).items.find(s=>s.id===u.searchParams.get("source_id")&&s.enabled);if(!source)return send(404,{error:"source disabled or unknown"});const batch=await collectBatch(client,config,source,Date.parse(u.searchParams.get("from")),Date.parse(u.searchParams.get("to")));if(Buffer.byteLength(JSON.stringify(batch))>8*1024*1024)return send(413,{error:"batch exceeds limit",complete:false});return send(200,batch)}const completion=/^\/feedback-completion\/(\d{1,22})\/(\d{1,22})\/(\d{1,22})$/.exec(u.pathname);if(completion&&req.method==="POST")return send(200,await markFeedbackCompleted(client,...completion.slice(1)));const m=/^\/messages\/(\d{1,22})\/(mark|preflight)$/.exec(u.pathname);if(m&&req.method==="GET"&&m[2]==="preflight")return send(200,await preflightSelected(client,feedbackClient,m[1]));if(m&&req.method==="POST"&&m[2]==="mark")return send(200,await markSelected(client,feedbackClient,m[1]));return send(404,{error:"not found"})}catch{return send(502,{error:"Discord bridge operation failed",complete:false})}});server.requestTimeout=125000;server.headersTimeout=5000;server.on("error",e=>logger.error("Bridge listen failed",e.code));server.listen(listenPort,"127.0.0.1");return server}
module.exports={BUG_EMOJI,COMPLETION_EMOJI,markFeedbackCompleted,snapshotMessage,scopedChannel,collectBatch,markSelected,preflightSelected,sourceCatalog,startFeedbackBridge};
