# Social Media Embed Bot

A Discord bot built with discordgo that automatically detects social media links and creates beautiful embeds for different platforms.

## Features

- **Automatic Link Detection**: Scans messages for social media links
- **Platform-Specific Embeds**: Creates custom embeds for each platform
- **Supported Platforms**:
  - 📸 Instagram (posts, reels, TV)
  - 🐦 Twitter/X (posts)
  - 🎵 TikTok (videos)
  - 📺 YouTube (videos)
  - 🤖 Reddit (posts)
- **Rich Embeds**: Includes platform logos, colors, and metadata
- **Ping Command**: Still responds to `!ping` with "pong"

## Setup

### 1. Create a Discord Bot

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name
3. Go to the "Bot" section in the left sidebar
4. Click "Add Bot"
5. Copy the bot token (you'll need this later)

### 2. Invite the Bot to Your Server

1. Go to the "OAuth2" > "URL Generator" section
2. Select "bot" under scopes
3. Select the permissions you want to give the bot:
   - Send Messages
   - Read Message History
   - Embed Links
   - Use External Emojis
4. Copy the generated URL and open it in your browser to invite the bot

### 3. Configure Environment Variables

Create a `.env` file in the project root:
```bash
DISCORD_TOKEN=your-bot-token-here
```

### 4. Run the Bot

```bash
go run main.go
```

## Usage

### Automatic Embed Creation

Simply post a social media link in any channel where the bot has access:

- **Instagram**: `https://www.instagram.com/p/ABC123/`
- **Twitter**: `https://twitter.com/username/status/123456789`
- **TikTok**: `https://www.tiktok.com/@username/video/123456789`
- **YouTube**: `https://www.youtube.com/watch?v=ABC123`
- **Reddit**: `https://www.reddit.com/r/subreddit/comments/123456`

The bot will automatically detect the link and create a platform-specific embed with:
- Platform logo and branding colors
- Author information
- Clickable link to the original post
- Platform-specific metadata

### Manual Commands

- `!ping` - Bot responds with "pong"

## Example Embeds

Each platform gets its own unique embed style:

- **Instagram**: Pink theme with camera emoji
- **Twitter**: Blue theme with bird emoji  
- **TikTok**: Black theme with music note emoji
- **YouTube**: Red theme with TV emoji
- **Reddit**: Orange theme with robot emoji

## Stopping the Bot

Press `Ctrl+C` to stop the bot gracefully.

## Dependencies

- Go 1.24.4 or later
- discordgo v0.27.1
- godotenv v1.5.1

## Customization

You can easily extend the bot to support more platforms by:
1. Adding new regex patterns in the `patterns` map
2. Creating new cases in the `createEmbedForPlatform` function
3. Adding platform-specific colors, logos, and metadata 