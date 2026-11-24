const TRIGGER_EMOJI_NAMES = new Set(["bug", "beetle", "lady_beetle", "\u{1F41B}", "\u{1FAB2}", "\u{1F41E}"]);
const SUCCESS_EMOJI = "\u2705";

function isTriggerEmoji(emoji) {
  return TRIGGER_EMOJI_NAMES.has(emoji?.name?.replace(/\uFE0F/g, ""));
}

function attachmentLines(message) {
  const attachments = Array.from(message.attachments?.values?.() || []);
  if (attachments.length === 0) {
    return [];
  }

  return ["", "Attachments:", ...attachments.map((attachment) => attachment.url)];
}

function buildFeedbackPayload(message, config) {
  const authorTag = message.author?.tag || message.author?.username || "Unknown user";
  const messageUrl = message.url || `https://discord.com/channels/${message.guildId}/${message.channelId}/${message.id}`;
  const content = message.content?.trim() || "[No text content]";

  return {
    PID: message.author?.id || "",
    Project: config.feedbackProject,
    Message: [
      content,
      ...attachmentLines(message),
      "",
      `Discord message: ${messageUrl}`,
      `Author: ${authorTag} (${message.author?.id || "unknown"})`,
      `Channel: ${message.channel?.name || message.channelId || "unknown"}`,
    ].join("\n"),
    Rating: 0,
    Env: config.feedbackEnv,
    Category: config.feedbackCategory,
    Platform: "Discord",
  };
}

async function fetchPartial(resource, label) {
  if (!resource.partial) {
    return resource;
  }

  try {
    return await resource.fetch();
  } catch (error) {
    throw new Error(`Could not fetch partial ${label}: ${error.message}`);
  }
}

function hasSuccessReaction(message) {
  return Boolean(message.reactions?.cache?.some((reaction) => reaction.emoji.name === SUCCESS_EMOJI));
}

function registerFeedbackReactionHandler(client, feedbackClient, config, logger = console) {
  const inFlight = new Map();
  const handleReaction = async (reaction, user) => {
    if (user.bot) {
      return;
    }

    try {
      const fullReaction = await fetchPartial(reaction, "reaction");
      if (!isTriggerEmoji(fullReaction.emoji)) {
        return;
      }

      const message = await fetchPartial(fullReaction.message, "message");
      if (inFlight.has(message.id)) { await inFlight.get(message.id); return; }
      const intake = (async () => {
        if (feedbackClient.submitSnapshot) {
          const { snapshotForManual } = require("./forumFeedback");
          const result = await feedbackClient.submitSnapshot(await snapshotForManual(message, config,feedbackClient,client));
          if (!result?.durable || !result.feedback_id) throw new Error("Feedback intake not verified");
          try {await message.react(SUCCESS_EMOJI);await feedbackClient.markResult?.(result.id,{check_marked:true,trigger_message_id:message.id})}catch(error){await feedbackClient.markResult?.(result.id,{check_marked:false,trigger_message_id:message.id,error:"Manual check reaction failed; retry pending"});throw error}
          return;
        } else {
          // Compatibility for embedders; deployed client always uses snapshots.
          if (hasSuccessReaction(message)) return;
          await feedbackClient.submitFeedback(buildFeedbackPayload(message, config));
        }
        await message.react(SUCCESS_EMOJI);
      })();
      inFlight.set(message.id, intake);
      try { await intake; } finally { inFlight.delete(message.id); }

      logger.info(`Submitted Discord message ${message.id} to feedback`);
    } catch (error) {
      // Correlate routing/API failures without logging message text, auth or API bodies.
      logger.error("Failed to submit Discord feedback", {
        guild_id: reaction.message?.guildId,
        channel_id: reaction.message?.channelId,
        message_id: reaction.message?.id,
        parent_id: reaction.message?.channel?.parentId,
        stage_error: error.message === "Discord channel is not an enabled project source" ? "source_not_enabled" :
          /^Feedback API returned \d+:/.test(error.message) ? error.message.match(/^Feedback API returned \d+/)[0] :
          error.message === "Feedback intake not verified" ? "intake_not_verified" : "intake_or_mark_failed",
        code: typeof error.code === "number" ? error.code : undefined,
      });
    }
  };
  client.on("messageReactionAdd", handleReaction);
  // discord.js can discard reactions for uncached guild channels/users before
  // emitting messageReactionAdd. Hydrate only an explicit live flag, never scan.
  if (client.channels?.cache && client.users?.fetch) {
    const pendingRaw=new Set();
    client.on("raw",async packet=>{
      const d=packet?.d;
      if(packet?.t!=="MESSAGE_REACTION_ADD" || d?.guild_id!==require("./manualRouting").SUPPORTED_GUILD || !isTriggerEmoji(d.emoji) || d.user_id===client.user.id)return;
      if(client.channels.cache.has(d.channel_id) && (d.member?.user || client.users.cache?.has(d.user_id)))return;
      const key=`${d.channel_id}:${d.message_id}:${d.user_id}`;
      if(pendingRaw.has(key))return;pendingRaw.add(key);
      try {
        const user=d.member?.user||await client.users.fetch(d.user_id);
        if(user.bot)return;
        const channel=await client.channels.fetch(d.channel_id);
        if(channel?.guildId!==d.guild_id || !require("./manualRouting").isManualMessageChannel(channel) || !channel.permissionsFor(client.user)?.has(["ViewChannel","ReadMessageHistory"]))throw Error("Flagged channel unavailable");
        const message=await channel.messages.fetch({message:d.message_id,force:true,cache:false});
        if(message.guildId!==d.guild_id||message.channelId!==d.channel_id||message.id!==d.message_id)throw Error("Flagged message identity mismatch");
        await handleReaction({emoji:d.emoji,message},user);
      } catch {logger.error("Failed to hydrate Discord feedback flag",{guild_id:d.guild_id,channel_id:d.channel_id,message_id:d.message_id,stage_error:"flag_unavailable"})}
      finally {pendingRaw.delete(key)}
    });
  }
}

module.exports = {
  SUCCESS_EMOJI,
  buildFeedbackPayload,
  isTriggerEmoji,
  registerFeedbackReactionHandler,
};
