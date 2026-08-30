# LittleClyde

A Node.js Discord bot that turns marked Discord messages into feedback records.

When someone reacts to a message with `:beetle:` / `:lady_beetle:`, LittleClyde sends that message to the Clyde feedback API. After the API accepts it, the bot reacts to the original Discord message with `:white_check_mark:`.

## Setup

```sh
npm install
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
