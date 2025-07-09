package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var Token string

func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Get bot token from environment variable
	Token = os.Getenv("DISCORD_TOKEN")
	if Token == "" {
		log.Fatal("DISCORD_TOKEN environment variable is required")
	}
}

func main() {
	// Create a new Discord session using the bot token
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	// Register the messageCreate func as a callback for MessageCreate events
	dg.AddHandler(messageCreate)

	// Open a websocket connection to Discord
	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection: ", err)
	}

	// Wait here until CTRL-C or other term signal is received
	fmt.Println("Social Media Embed Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if the message is "!ping"
	if m.Content == "!ping" {
		// Respond with "pong"
		s.ChannelMessageSend(m.ChannelID, "pong")
		return
	}

	// Check for social media links and create embeds
	createSocialMediaEmbed(s, m)
}

// createSocialMediaEmbed detects social media links and creates appropriate embeds
func createSocialMediaEmbed(s *discordgo.Session, m *discordgo.MessageCreate) {
	message := m.Content

	// Debug logging
	fmt.Printf("Checking message: %s\n", message)

	// Define regex patterns for different social media platforms
	patterns := map[string]*regexp.Regexp{
		"instagram": regexp.MustCompile(`https?://(?:www\.)?instagram\.com/(?:p|reel|reels|tv)/([a-zA-Z0-9_-]+)/?`),
		"twitter":   regexp.MustCompile(`https?://(?:www\.)?(?:twitter\.com|x\.com)/([a-zA-Z0-9_]+)/status/([0-9]+)`),
		"tiktok":    regexp.MustCompile(`https?://(?:www\.)?tiktok\.com/@([a-zA-Z0-9_.]+)/video/([0-9]+)`),
		"youtube":   regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]+)`),
		"reddit":    regexp.MustCompile(`https?://(?:www\.)?reddit\.com/r/([a-zA-Z0-9_]+)/comments/([a-zA-Z0-9_]+)`),
	}

	// Check each platform
	for platform, pattern := range patterns {
		fmt.Printf("Checking %s pattern: %s\n", platform, pattern.String())
		if pattern.MatchString(message) {
			fmt.Printf("Found %s match!\n", platform)
			matches := pattern.FindStringSubmatch(message)
			fmt.Printf("Matches: %v\n", matches)
			if len(matches) > 1 {
				if platform == "instagram" {
					// For Instagram, send the kkinstagram link as plain text
					kkinstagramURL := regexp.MustCompile(`instagram\.com`).ReplaceAllString(message, "kkinstagram.com")
					s.ChannelMessageSend(m.ChannelID, kkinstagramURL)
					// Edit the original message to remove the Instagram link and prevent embeds
					editedMessage := regexp.MustCompile(`https?://(?:www\.)?instagram\.com/(?:p|reel|reels|tv)/([a-zA-Z0-9_-]+)/?`).ReplaceAllString(message, "[Instagram link removed]")
					s.ChannelMessageEdit(m.ChannelID, m.ID, editedMessage)
				} else {
					// For other platforms, send embeds as before
					embed := createEmbedForPlatform(platform, matches, message, m.Author)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				}
				return
			}
		}
	}

	fmt.Printf("No social media links found in message\n")
}

// createEmbedForPlatform creates a Discord embed based on the social media platform
func createEmbedForPlatform(platform string, matches []string, originalURL string, author *discordgo.User) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    author.Username,
			IconURL: author.AvatarURL(""),
		},
		Timestamp: "",
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Social Media Embed Bot",
		},
	}

	switch platform {
	case "instagram":
		embed.Title = "📸 Instagram Post"
		embed.Description = fmt.Sprintf("Instagram post by %s", author.Username)
		embed.Color = 0xE4405F // Instagram pink
		embed.URL = regexp.MustCompile(`instagram\.com`).ReplaceAllString(originalURL, "kkinstagram.com")
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e7/Instagram_logo_2016.svg/132px-Instagram_logo_2016.svg.png",
		}

	case "twitter":
		embed.Title = "🐦 Twitter Post"
		embed.Description = fmt.Sprintf("Twitter post by @%s", matches[1])
		embed.Color = 0x1DA1F2 // Twitter blue
		embed.URL = originalURL
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/commons/thumb/6/6f/Logo_of_Twitter.svg/2491px-Logo_of_Twitter.svg.png",
		}

	case "tiktok":
		embed.Title = "🎵 TikTok Video"
		embed.Description = fmt.Sprintf("TikTok video by @%s", matches[1])
		embed.Color = 0x000000 // TikTok black
		embed.URL = originalURL
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/en/thumb/a/a9/TikTok_logo.svg/2560px-TikTok_logo.svg.png",
		}

	case "youtube":
		embed.Title = "📺 YouTube Video"
		embed.Description = fmt.Sprintf("YouTube video shared by %s", author.Username)
		embed.Color = 0xFF0000 // YouTube red
		embed.URL = originalURL
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", matches[1]),
		}

	case "reddit":
		embed.Title = "🤖 Reddit Post"
		embed.Description = fmt.Sprintf("Reddit post in r/%s", matches[1])
		embed.Color = 0xFF4500 // Reddit orange
		embed.URL = originalURL
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/en/thumb/8/82/Reddit_logo_and_wordmark.svg/1200px-Reddit_logo_and_wordmark.svg.png",
		}
	}

	return embed
}
