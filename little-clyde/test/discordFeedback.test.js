const assert = require("node:assert/strict");
const test = require("node:test");

const { buildFeedbackPayload, isTriggerEmoji } = require("../src/discordFeedback");

test("recognizes beetle trigger emojis", () => {
  assert.equal(isTriggerEmoji({ name: "beetle" }), true);
  assert.equal(isTriggerEmoji({ name: "lady_beetle" }), true);
  assert.equal(isTriggerEmoji({ name: "🪲" }), true);
  assert.equal(isTriggerEmoji({ name: "🐞" }), true);
  assert.equal(isTriggerEmoji({ name: "✅" }), false);
});

test("builds feedback payload from a Discord message", () => {
  const message = {
    id: "333",
    guildId: "111",
    channelId: "222",
    content: "This needs a better ending.",
    url: "https://discord.com/channels/111/222/333",
    author: {
      id: "444",
      tag: "player#1234",
    },
    channel: {
      name: "playtest",
    },
    attachments: new Map([["a", { url: "https://example.com/screenshot.png" }]]),
  };

  const payload = buildFeedbackPayload(message, {
    feedbackProject: "Cursemark",
    feedbackEnv: "production",
    feedbackCategory: "Discord",
  });

  assert.equal(payload.PID, "444");
  assert.equal(payload.Project, "Cursemark");
  assert.equal(payload.Rating, 0);
  assert.equal(payload.Env, "production");
  assert.equal(payload.Category, "Discord");
  assert.equal(payload.Platform, "Discord");
  assert.match(payload.Message, /This needs a better ending\./);
  assert.match(payload.Message, /https:\/\/discord.com\/channels\/111\/222\/333/);
  assert.match(payload.Message, /player#1234 \(444\)/);
  assert.match(payload.Message, /https:\/\/example.com\/screenshot.png/);
});
