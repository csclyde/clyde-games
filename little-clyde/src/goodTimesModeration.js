const { PermissionFlagsBits } = require("discord.js");

const GOOD_TIMES_CHANNEL_NAME = "good-times";
const PROTECTED_USERNAME = "c_l_y_d_e_";
const DELETE_MESSAGE_SECONDS = 24 * 60 * 60;

function normalizeName(name) {
  return String(name || "").trim().toLowerCase();
}

function isProtectedClydeIdentity(member, config = {}) {
  const protectedUsername = normalizeName(config.protectedDiscordUsername || PROTECTED_USERNAME);
  const protectedUserId = config.protectedDiscordUserId || "";
  const user = member?.user || {};

  if (protectedUserId && user.id === protectedUserId) {
    return true;
  }

  const names = [
    user.username,
    user.globalName,
    user.tag,
    member?.displayName,
    member?.nickname,
  ].map(normalizeName);

  return names.includes(protectedUsername);
}

function hasServerRole(member) {
  return Boolean(member?.roles?.cache?.some((role) => role.id !== member.guild?.id));
}

function isAdmin(member) {
  return Boolean(member?.permissions?.has?.(PermissionFlagsBits.Administrator));
}

function isProtectedMember(member, config = {}) {
  if (!member || !member.user) {
    return true;
  }

  return Boolean(
    member.user.bot ||
      member.id === member.guild?.ownerId ||
      isProtectedClydeIdentity(member, config) ||
      isAdmin(member) ||
      hasServerRole(member)
  );
}

function isGoodTimesMessage(message) {
  return Boolean(
    message?.guild &&
      message.author &&
      !message.author.bot &&
      message.channel?.name === GOOD_TIMES_CHANNEL_NAME
  );
}

async function resolveMessageMember(message) {
  if (message.member) {
    return message.member;
  }

  return message.guild?.members?.fetch?.(message.author.id);
}

function registerGoodTimesModerationHandler(client, config = {}, logger = console) {
  client.on("messageCreate", async (message) => {
    if (!isGoodTimesMessage(message)) {
      return;
    }

    try {
      const member = await resolveMessageMember(message);
      if (isProtectedMember(member, config)) {
        return;
      }

      if (!member.bannable) {
        logger.warn(`Skipping unbannable good-times member ${member.id}`);
        return;
      }

      await member.ban({
        deleteMessageSeconds: DELETE_MESSAGE_SECONDS,
        reason: "No-role account posted in #good-times",
      });

      logger.info(`Banned no-role good-times member ${member.id}`);
    } catch (error) {
      logger.error("Failed to moderate good-times message", error);
    }
  });
}

module.exports = {
  DELETE_MESSAGE_SECONDS,
  GOOD_TIMES_CHANNEL_NAME,
  PROTECTED_USERNAME,
  hasServerRole,
  isAdmin,
  isGoodTimesMessage,
  isProtectedClydeIdentity,
  isProtectedMember,
  registerGoodTimesModerationHandler,
};
