require("dotenv").config({ quiet: true });

function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`);
  }

  return value;
}

function readConfig() {
  return {
    discordToken: requireEnv("DISCORD_TOKEN"),
    feedbackApiUrl: process.env.FEEDBACK_API_URL || "http://localhost:9990/feedback",
    feedbackProject: process.env.FEEDBACK_PROJECT || "Discord",
    feedbackEnv: process.env.FEEDBACK_ENV || "production",
    feedbackCategory: process.env.FEEDBACK_CATEGORY || "Discord",
  };
}

module.exports = {
  readConfig,
};
