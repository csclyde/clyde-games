const { createHash } = require("node:crypto");
const { isTriggerEmoji } = require("./discordFeedback");


const BASELINE = Date.parse("2026-11-16T05:58:21Z");
function hasBug(message) { return [...(message.reactions?.cache?.values?.() || [])].some(r => isTriggerEmoji(r.emoji) && r.count > 0); }
function ownBug(message) { return [...(message.reactions?.cache?.values?.() || [])].some(r => isTriggerEmoji(r.emoji) && r.count > 0 && r.me); }
function snowflakeTime(id) { return Number((BigInt(id) >> 22n) + 1420070400000n); }
function threadScope(thread,source) { return thread?.guildId === source.guildId && thread?.parentId === source.id && thread.type === 11; }
function evidenceHash(snapshot) {
 return createHash("sha256").update(JSON.stringify([snapshot.title,snapshot.content,snapshot.edited_timestamp,snapshot.attachments,snapshot.thread_context])).digest("hex");
}
async function forumChannel(client,source) {
 const forum=await client.channels.fetch(source.id);
 if(forum?.guildId!==source.guildId || forum.type!==15) throw new Error("Forum scope mismatch");
 if(!forum.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"])) throw new Error("Forum history permission unavailable");
 return forum;
}
async function forumSnapshot(thread, config, source, trigger) {
 if(!threadScope(thread,source)) throw new Error("Forum thread scope mismatch");
 const starter=await thread.fetchStarterMessage({force:true,cache:false});
 if(!starter || starter.id!==thread.id) throw new Error("Forum starter unavailable");
 const {snapshotMessage}=require("./feedbackBridge");
 const snapshot=await snapshotMessage(starter,config);
 snapshot.kind="forum_thread";snapshot.parent_id=source.id;snapshot.scope_id=source.id;snapshot.title=thread.name;
 snapshot.thread_archived=Boolean(thread.archived);snapshot.thread_locked=Boolean(thread.locked);
 snapshot.thread_context=[];let before;const seen=new Set();let complete=false;
 for(let page=0;page<20;page++) {
  const messages=await thread.messages.fetch({limit:100,...(before?{before}:{})});
  const ordered=[...messages.values()].sort((a,b)=>BigInt(a.id)<BigInt(b.id)?-1:1);
  for(const m of ordered) if(m.id!==starter.id) snapshot.thread_context.push({id:m.id,content:m.content,author_id:m.author?.id,timestamp:new Date(m.createdTimestamp).toISOString(),edited_timestamp:m.editedTimestamp?new Date(m.editedTimestamp).toISOString():null,url:m.url,attachments:[...(m.attachments?.values?.()||[])].map(a=>({id:a.id,name:a.name,url:a.url,size:a.size,content_type:a.contentType})),reactions:[...(m.reactions?.cache?.values?.()||[])].map(r=>({name:r.emoji.name,id:r.emoji.id,count:r.count,me:r.me}))});
  if(messages.size<100){complete=true;break}
  before=ordered[0]?.id;if(!before||seen.has(before))throw new Error("Thread pagination stalled");seen.add(before);
 }
 if(!complete)throw new Error("Forum thread exceeds 20 message pages");
 snapshot.thread_context.sort((a,b)=>BigInt(a.id)<BigInt(b.id)?-1:1);
 snapshot.handled=hasBug(starter);snapshot.context_timestamp=new Date(Math.max(starter.createdTimestamp,starter.editedTimestamp||0,...snapshot.thread_context.map(m=>Math.max(Date.parse(m.timestamp),Date.parse(m.edited_timestamp)||0)))).toISOString();
 snapshot.evidence_hash=evidenceHash({...snapshot,thread_context:snapshot.thread_context.map(({reactions,...m})=>m)});
 if(trigger){snapshot.trigger_message={id:trigger.id,channel_id:trigger.channelId,content:trigger.content,author_id:trigger.author?.id,url:trigger.url};
  if(trigger.id!==starter.id)snapshot.feedback.Message+="\n\nManually flagged forum reply:\n"+require("./discordFeedback").buildFeedbackPayload(trigger,config).Message;
 }
 snapshot.feedback.Message="Forum: "+thread.name+"\n\n"+snapshot.feedback.Message;
 return snapshot;
}
async function snapshotForManual(message,config,feedbackClient,client=message.client) {
 const {manualPolicy,routeManualChannel,SUPPORTED_GUILD}=require("./manualRouting");
 // Reject DMs/other guilds before looking up content or channel metadata.
 if(message.guildId!==SUPPORTED_GUILD)throw new Error("Manual Discord guild unavailable");
 const catalog=await feedbackClient.syncSources();manualPolicy(catalog);
 const channel=client?.channels?await client.channels.fetch(message.channelId,{force:true}):message.channel;
 if(channel?.guildId!==SUPPORTED_GUILD||channel.id!==message.channelId)throw new Error("Manual channel identity mismatch");
 if(!channel.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"]))throw new Error("Manual history permission unavailable");
 const threaded=[10,11,12].includes(channel.type);
 const parent=threaded?await client.channels.fetch(channel.parentId,{force:true}):null;
 const source=routeManualChannel(catalog,channel,parent);
 if(source.kind==="discord_forum")return forumSnapshot(channel,{...config,feedbackProject:source.projectId},source,message);
 if(threaded){
  const starter=await parent.messages.fetch({message:channel.id,force:true,cache:false});
  if(starter.id!==channel.id||starter.channelId!==source.id||starter.guildId!==source.guildId)throw new Error("Manual reply starter identity mismatch");
  const snapshot=await require("./feedbackBridge").snapshotMessage(starter,{...config,feedbackProject:source.projectId},channel);
  snapshot.scope_id=source.id;snapshot.kind="discord_text";
  snapshot.trigger_message={id:message.id,channel_id:message.channelId,content:message.content,author_id:message.author?.id,timestamp:new Date(message.createdTimestamp).toISOString(),url:message.url,attachments:[...(message.attachments?.values?.()||[])].map(a=>({id:a.id,name:a.name,url:a.url,size:a.size,content_type:a.contentType}))};
  snapshot.feedback.Message+="\n\nManually flagged thread reply:\n"+require("./discordFeedback").buildFeedbackPayload(message,{...config,feedbackProject:source.projectId}).Message;
  return snapshot;
 }
 const snapshot=await require("./feedbackBridge").snapshotMessage(message,{...config,feedbackProject:source.projectId});snapshot.scope_id=source.id;snapshot.kind="discord_text";return snapshot;
}
async function collectForum(client,config,source,from,to,baseline=Date.parse(source.baselineAt)) {
 const forum=await forumChannel(client,source);const threads=new Map();
 const active=await forum.threads.fetchActive();for(const t of active.threads.values())if(threadScope(t,source))threads.set(t.id,t);
 let before;const seen=new Set();let complete=false;
 for(let page=0;page<20;page++) {
  const batch=await forum.threads.fetchArchived({type:"public",limit:100,...(before?{before}:{})});
  for(const t of batch.threads.values()){if(!threadScope(t,source))throw new Error("Archived forum scope mismatch");threads.set(t.id,t)}
  if(!batch.hasMore){complete=true;break}
  const dates=[...batch.threads.values()].map(t=>t.archiveTimestamp).filter(Number.isFinite);
  before=Math.min(...dates);if(!Number.isFinite(before)||seen.has(before))throw new Error("Archived pagination stalled");seen.add(before);
 }
 if(!complete)throw new Error("Forum exceeds 20 archived listing pages");
 const messages=[];
 for(const t of threads.values()) {
  const created=t.createdTimestamp||snowflakeTime(t.id);if(created>to)continue;
  const last=t.lastMessageId?snowflakeTime(t.lastMessageId):created;
  if(created<baseline && last<baseline) {
   // Retain identity knowledge only, without importing handled historical content.
   messages.push({id:t.id,channel_id:t.id,guild_id:source.guildId,parent_id:source.id,kind:"forum_thread",timestamp:new Date(created).toISOString(),context_timestamp:new Date(last).toISOString(),historical_identity_only:true,handled:true,title:t.name});continue;
  }
  const snapshot=await forumSnapshot(t,{...config,feedbackProject:source.projectId},source);
  if(created<baseline){snapshot.context_only=true;snapshot.handled=true;snapshot.skip_reason="baseline"}
  if(snapshot.handled)snapshot.skip_reason=snapshot.skip_reason||"existing_bug_marker";
  messages.push(snapshot);
 }
 return {guild_id:source.guildId,forum_id:source.id,messages,complete:true,coverage:{private_threads:"excluded",archived_public:"all visible pages up to 20",thread_messages:"up to 20 pages of 100; limit fails",old_baseline_threads:"identity only; new replies are context-only; older edits without a new reply are not exhaustively detected"}};
}
module.exports={BASELINE,hasBug,ownBug,threadScope,forumChannel,forumSnapshot,snapshotForManual,collectForum};
