package discord

import (
	"context"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only respond to DMs or when mentioned
	isDM := m.GuildID == ""
	isMentioned := false
	for _, u := range m.Mentions {
		if u.ID == s.State.User.ID {
			isMentioned = true
			break
		}
	}

	if !isDM && !isMentioned {
		return
	}

	if isDM {
		_ = b.db.SetNote("discord_user_id", m.Author.ID)
	}

	content := strings.TrimSpace(m.Content)
	// Strip mention from message
	content = strings.TrimSpace(stripMention(content, s.State.User.ID))
	if content == "" {
		return
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	reply, err := b.agent.RunWithConversation(context.Background(), m.Author.ID, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		s.ChannelMessageSend(m.ChannelID, "Something went wrong. Try again?")
		return
	}

	// Discord has a 2000 char limit; split if needed
	for _, chunk := range splitMessage(reply, 2000) {
		s.ChannelMessageSend(m.ChannelID, chunk)
	}
}

func stripMention(s, userID string) string {
	s = strings.ReplaceAll(s, "<@"+userID+">", "")
	s = strings.ReplaceAll(s, "<@!"+userID+">", "")
	return s
}

func splitMessage(s string, maxLen int) []string {
	if len(s) <= maxLen {
		return []string{s}
	}
	var chunks []string
	for len(s) > 0 {
		end := min(maxLen, len(s))
		// Try to split at a newline
		if idx := strings.LastIndex(s[:end], "\n"); idx > 0 {
			end = idx + 1
		}
		chunks = append(chunks, s[:end])
		s = s[end:]
	}
	return chunks
}
