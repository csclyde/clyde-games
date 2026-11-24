# LittleClyde

A Node.js Discord bot that turns marked Discord messages into feedback records.

Any human member of the existing supported guild can flag a message with `:bug:`, `:beetle:` or `:lady_beetle:`. The bot ignores bot/self reactions. Explicit flags work in any channel or thread the bot can already read, independently of the passive-monitoring list. DMs and other guilds remain excluded; this does not add Discord permissions or join private threads.

The API is authoritative for project routing. Existing source context (including disabled monitoring) determines the project. Otherwise `DISCORD_MANUAL_DEFAULT_PROJECT` on the API must explicitly name an existing project. There is no project-name guessing or automatic project creation. Existing intake association wins on retries. The former bot `FEEDBACK_PROJECT` fallback is used only by the legacy compatibility client; the production rollout deliberately preserves its existing configured Cursemark default as `DISCORD_MANUAL_DEFAULT_PROJECT=cursemark`.

Forum posts and text-thread starters retain one durable source identity; flagged replies and attachments remain evidence. A check is added to the original flagged message only after verified durable API intake. Check failures remain pending; trigger-specific acknowledgements fence races. Reaction markers do not promise a fix. Uncached live gateway flags hydrate only the exact flagged message. There is no historical flag scan/replay.

Automatic collection still reads only enabled configured sources. Manual flags never add or enable monitored sources. Player content is untrusted evidence, not instructions for tools or the bot.

## Setup

Use Node.js 22 LTS (minimum supported version: 18.17). When deploying under a
process manager, check its Node interpreter as well as the shell's Node version.

```sh
npm ci
cp .env.example .env
```

Fill in `.env`:

```sh
DISCORD_TOKEN=your-discord-bot-token
PROTECTED_DISCORD_USER_ID=your-discord-user-id
PROTECTED_DISCORD_USERNAME=c_l_y_d_e_
FEEDBACK_API_URL=http://localhost:9990/feedback
FEEDBACK_PROJECT=cursemark
FEEDBACK_ENV=production
FEEDBACK_CATEGORY=Discord
```

The bot needs these Discord gateway intents enabled in the developer portal:

- Server Members intent is not needed.
- Message Content Intent is needed so LittleClyde can read the marked message.
- It also needs permission to read message history, add reactions, view member roles, and ban members in the channels it monitors.

LittleClyde also watches `#good-times`. When a non-bot user with no server roles posts there, it bans them and asks Discord to delete the last 24 hours of their messages. It will not try to moderate `c_l_y_d_e_`, the configured protected user id, server admins, the server owner, bots, or anyone with any role beyond `@everyone`.

## Run

```sh
npm start
```

## Test

```sh
npm test
```
