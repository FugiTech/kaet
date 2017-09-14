# kaet
Chat bot for twitch.tv/kate

## Environment variables

Configuration of the bot is done with environment variables. The following are what is used and how to obtain proper credentials.
* BOT_CHANNEL: The twitch streamer's channel to join
* BOT_USER: The Twitch username of the bot
* BOT_PASSWORD: the Twitch oauth token for the bot's Twitch account, allowing it to access Twitch chat
* BOT_GITHUB_SECRET: the Github oauth token to authenticate Github when the webhook notifies the bot of new commits
* BOT_CLIENT_ID: the Twitch API token (used to grab uptime and current game from Twitch's Kraken API)
* BOT_CLIENT_SECRET: the Twitch API secret (used to grab uptime and current game from Twitch's Kraken API)
* BOT_MASHAPE_KEY: API key for mashape, used to grab game ratings from the IGN Game Ratings API
