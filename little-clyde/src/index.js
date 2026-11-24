const { Client, GatewayIntentBits, Partials } = require("discord.js");
const { readConfig } = require("./config");
const { createFeedbackClient } = require("./feedbackClient");
const { registerFeedbackReactionHandler } = require("./discordFeedback");
const { startFeedbackBridge } = require("./feedbackBridge");
const { registerGoodTimesModerationHandler } = require("./goodTimesModeration");

let config;
try {
  config = readConfig();
} catch (error) {
  console.error(error.message);
  process.exit(1);
}

const client = new Client({
  intents: [
    GatewayIntentBits.Guilds,
    GatewayIntentBits.GuildMessages,
    GatewayIntentBits.GuildMessageReactions,
    GatewayIntentBits.MessageContent,
  ],
  partials: [Partials.Message, Partials.Channel, Partials.Reaction],
});

client.once("ready", (readyClient) => {
  console.log(`LittleClyde is watching as ${readyClient.user.tag}`);
  startFeedbackBridge(client, feedbackClient, config);
});

const feedbackClient = createFeedbackClient(config);
registerFeedbackReactionHandler(client, feedbackClient, config);
registerGoodTimesModerationHandler(client, config);

client.login(config.discordToken);
