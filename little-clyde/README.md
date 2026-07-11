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
FEEDBACK_API_URL=http://localhost:9990/feedback
FEEDBACK_PROJECT=Cursemark
FEEDBACK_ENV=production
FEEDBACK_CATEGORY=Discord
```

The bot needs these Discord gateway intents enabled in the developer portal:

- Server Members intent is not needed.
- Message Content Intent is needed so LittleClyde can read the marked message.
- It also needs permission to read message history and add reactions in the channels it monitors.

## Run

```sh
npm start
```

## Test

```sh
npm test
```
