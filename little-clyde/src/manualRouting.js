// Explicit human flags use project context, not passive monitoring membership.
const SUPPORTED_GUILD = "1169710427387744297";
function manualPolicy(catalog) {
 const policy=catalog.manual;
 if (!policy || policy.guild_id!==SUPPORTED_GUILD) throw new Error("Manual intake policy unavailable");
 return policy;
}
function isManualMessageChannel(channel) {
 return channel?.guildId===SUPPORTED_GUILD && typeof channel.messages?.fetch==="function" && (typeof channel.isTextBased!=="function" || channel.isTextBased());
}
function routeManualChannel(catalog, channel, parent, stickyProject) {
 const policy=manualPolicy(catalog);
 if (!isManualMessageChannel(channel)) throw new Error("Manual Discord guild or channel unavailable");
 const threaded=[10,11,12].includes(channel.type);
 if (threaded && (parent?.guildId!==SUPPORTED_GUILD || parent.id!==channel.parentId || !(isManualMessageChannel(parent)||[15,16].includes(parent.type)))) throw new Error("Manual thread parent unavailable");
 if ([15,16].includes(parent?.type) && channel.type!==11) throw new Error("Invalid forum thread");
 const kind=threaded&&[15,16].includes(parent.type)?"discord_forum":"discord_text";
 const scope=threaded?parent.id:channel.id;
 const known=(policy.sources||[]).find(s=>s.guildId===SUPPORTED_GUILD&&s.id===scope&&s.kind===kind);
 const projectId=stickyProject||known?.projectId||policy.default_project;
 if (!projectId) throw new Error("Manual Discord default project is not configured");
 return {id:scope,guildId:SUPPORTED_GUILD,kind,projectId};
}
module.exports={SUPPORTED_GUILD,manualPolicy,isManualMessageChannel,routeManualChannel};
