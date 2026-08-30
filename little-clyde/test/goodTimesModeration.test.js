const assert = require("node:assert/strict");
const test = require("node:test");

const {
  DELETE_MESSAGE_SECONDS,
  hasServerRole,
  isGoodTimesMessage,
  isProtectedClydeIdentity,
  isProtectedMember,
  registerGoodTimesModerationHandler,
} = require("../src/goodTimesModeration");

function roleCache(roleIds, guildId = "guild-1") {
  return {
    some(predicate) {
      return roleIds.some((id) => predicate({ id }));
    },
  };
}

function member(overrides = {}) {
  const base = {
    id: "user-1",
    user: {
      id: "user-1",
      username: "plain_user",
      bot: false,
    },
    guild: {
      id: "guild-1",
      ownerId: "owner-1",
    },
    roles: {
      cache: roleCache(["guild-1"]),
    },
    permissions: {
      has: () => false,
    },
    bannable: true,
  };

  return {
    ...base,
    ...overrides,
    user: {
      ...base.user,
      ...overrides.user,
    },
    guild: {
      ...base.guild,
      ...overrides.guild,
    },
    roles: overrides.roles || base.roles,
    permissions: overrides.permissions || base.permissions,
  };
}

test("recognizes messages in #good-times only", () => {
  assert.equal(isGoodTimesMessage({
    guild: { id: "guild-1" },
    author: { bot: false },
    channel: { name: "good-times" },
  }), true);

  assert.equal(isGoodTimesMessage({
    guild: { id: "guild-1" },
    author: { bot: false },
    channel: { name: "general" },
  }), false);
});

test("treats @everyone as no server role", () => {
  assert.equal(hasServerRole(member()), false);
});

test("detects any non-everyone server role", () => {
  assert.equal(hasServerRole(member({
    roles: { cache: roleCache(["guild-1", "role-1"]) },
  })), true);
});

test("protects c_l_y_d_e_ by username, display name, and configured id", () => {
  assert.equal(isProtectedClydeIdentity(member({
    user: { username: "c_l_y_d_e_" },
  })), true);

  assert.equal(isProtectedClydeIdentity(member({
    displayName: "c_l_y_d_e_",
  })), true);

  assert.equal(isProtectedClydeIdentity(member({
    user: { id: "clyde-id" },
  }), { protectedDiscordUserId: "clyde-id" }), true);
});

test("protects bots, server owner, admins, clyde, and anyone with a role", () => {
  assert.equal(isProtectedMember(member({ user: { bot: true } })), true);
  assert.equal(isProtectedMember(member({ id: "owner-1" })), true);
  assert.equal(isProtectedMember(member({
    permissions: { has: () => true },
  })), true);
  assert.equal(isProtectedMember(member({
    user: { username: "c_l_y_d_e_" },
  })), true);
  assert.equal(isProtectedMember(member({
    roles: { cache: roleCache(["guild-1", "role-1"]) },
  })), true);
});

test("allows moderation only for non-bot, non-admin, no-role non-clyde members", () => {
  assert.equal(isProtectedMember(member()), false);
});

test("handler bans only no-role good-times members and deletes 24 hours", async () => {
  let handler;
  const banned = [];
  const testMember = member({
    ban: async (options) => banned.push(options),
  });

  registerGoodTimesModerationHandler({
    on(event, callback) {
      assert.equal(event, "messageCreate");
      handler = callback;
    },
  }, {}, { info() {}, warn() {}, error() {} });

  await handler({
    guild: { id: "guild-1" },
    author: { id: "user-1", bot: false },
    channel: { name: "good-times" },
    member: testMember,
  });

  assert.equal(banned.length, 1);
  assert.equal(banned[0].deleteMessageSeconds, DELETE_MESSAGE_SECONDS);
});

test("handler never bans c_l_y_d_e_", async () => {
  let handler;
  const banned = [];
  const clydeMember = member({
    user: { username: "c_l_y_d_e_" },
    ban: async (options) => banned.push(options),
  });

  registerGoodTimesModerationHandler({
    on(event, callback) {
      handler = callback;
    },
  }, {}, { info() {}, warn() {}, error() {} });

  await handler({
    guild: { id: "guild-1" },
    author: { id: "user-1", bot: false },
    channel: { name: "good-times" },
    member: clydeMember,
  });

  assert.equal(banned.length, 0);
});
