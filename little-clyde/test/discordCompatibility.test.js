const assert = require("node:assert/strict");
const test = require("node:test");
const {
  Client, Collection, GatewayIntentBits, Partials, PermissionFlagsBits,
  PermissionsBitField, REST, Routes,
} = require("discord.js");
const { MockAgent } = require("undici");
const { isProtectedMember } = require("../src/goodTimesModeration");
const { registerFeedbackReactionHandler, SUCCESS_EMOJI } = require("../src/discordFeedback");

test("updated client supports the bot's intents, partials, and real permission objects offline", async () => {
  const client = new Client({
    intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildMessages,
      GatewayIntentBits.GuildMessageReactions, GatewayIntentBits.MessageContent],
    partials: [Partials.Message, Partials.Channel, Partials.Reaction],
  });
  try {
    assert.equal(client.options.intents.has(GatewayIntentBits.MessageContent), true);
    const member = {
      id: "member", user: { id: "member", username: "test-user", bot: false },
      guild: { id: "guild", ownerId: "owner" },
      roles: { cache: new Collection([["guild", { id: "guild" }]]) },
      permissions: new PermissionsBitField(PermissionFlagsBits.Administrator),
    };
    assert.equal(isProtectedMember(member), true);
    member.permissions = new PermissionsBitField();
    assert.equal(isProtectedMember(member), false);
    member.roles.cache.set("role", { id: "role" });
    assert.equal(isProtectedMember(member), true);
  } finally {
    await client.destroy();
  }
});

test("updated REST and undici retry a rate limit using an agent that forbids network access", async () => {
  const agent = new MockAgent();
  agent.disableNetConnect();
  const pool = agent.get("https://discord.com");
  const route = "/api/v10/users/%40me";
  pool.intercept({ path: route, method: "GET" }).reply(429, { retry_after: 0.001, global: false }, {
    headers: { "content-type": "application/json", "retry-after": "0.001" },
  });
  pool.intercept({ path: route, method: "GET" }).reply(200, { id: "mock-bot", bot: true }, {
    headers: { "content-type": "application/json" },
  });
  const rest = new REST().setToken("test-token-never-sent").setAgent(agent);
  try {
    assert.deepEqual(await rest.get(Routes.user("@me")), { id: "mock-bot", bot: true });
    agent.assertNoPendingInterceptors();
  } finally {
    rest.clearHashSweeper();
    rest.clearHandlerSweeper();
    await agent.close();
  }
});

test("partial feedback reaction is submitted once and acknowledged only after acceptance", async () => {
  let handler;
  const events = [];
  const message = {
    id: "message", content: "feedback", author: { id: "author", username: "tester" },
    reactions: { cache: new Collection() },
    react: async (emoji) => events.push(["react", emoji]),
  };
  registerFeedbackReactionHandler({ on: (_, callback) => { handler = callback; } }, {
    submitFeedback: async (payload) => events.push(["submit", payload.PID]),
  }, {}, { info() {}, error(error) { throw error; } });
  const reaction = {
    partial: true,
    fetch: async () => ({ emoji: { name: "beetle" }, message: {
      partial: true, fetch: async () => message,
    } }),
  };
  await handler(reaction, { bot: false });
  assert.deepEqual(events, [["submit", "author"], ["react", SUCCESS_EMOJI]]);
  message.reactions.cache.set("success", { emoji: { name: SUCCESS_EMOJI } });
  await handler(reaction, { bot: false });
  assert.equal(events.length, 2);
});
