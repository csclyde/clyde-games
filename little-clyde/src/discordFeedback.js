const TRIGGER_EMOJI_NAMES = new Set(["beetle", "lady_beetle", "🪲", "🐞"]);
const SUCCESS_EMOJI = "✅";

function isTriggerEmoji(emoji) {
  return TRIGGER_EMOJI_NAMES.has(emoji.name);
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
  client.on("messageReactionAdd", async (reaction, user) => {
    if (user.bot) {
      return;
    }

    try {
      const fullReaction = await fetchPartial(reaction, "reaction");
      if (!isTriggerEmoji(fullReaction.emoji)) {
        return;
      }

      const message = await fetchPartial(fullReaction.message, "message");
      if (hasSuccessReaction(message)) {
        logger.info(`Skipping already-confirmed message ${message.id}`);
        return;
      }

      const payload = buildFeedbackPayload(message, config);
      await feedbackClient.submitFeedback(payload);
      await message.react(SUCCESS_EMOJI);

      logger.info(`Submitted Discord message ${message.id} to feedback`);
    } catch (error) {
      logger.error("Failed to submit Discord feedback", error);
    }
  });
}

module.exports = {
  SUCCESS_EMOJI,
  buildFeedbackPayload,
  isTriggerEmoji,
  registerFeedbackReactionHandler,
};
