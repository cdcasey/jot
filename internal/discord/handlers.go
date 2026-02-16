package discord

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/chris/jot/internal/llm"
)

// maxStoredTokens limits how much history we keep in memory per channel.
// This is a safety cap for RAM — the agent does its own precise trimming
// before each API call. We keep it generous so the agent has context to
// work with, but bounded so a long-running bot doesn't eat memory.
const maxStoredTokens = 100000

// Per-channel conversation history.
var (
	histories   = make(map[string][]llm.Message)
	historiesMu sync.Mutex
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

	content := strings.TrimSpace(m.Content)
	// Strip mention from message
	content = strings.TrimSpace(stripMention(content, s.State.User.ID))
	if content == "" {
		return
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	historiesMu.Lock()
	history := histories[m.ChannelID]
	historiesMu.Unlock()

	reply, newHistory, err := b.agent.Run(context.Background(), history, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		s.ChannelMessageSend(m.ChannelID, "Something went wrong. Try again?")
		return
	}

	// Cap stored history to prevent unbounded memory growth.
	// Use a generous budget — this just prevents the map from growing forever.
	// The agent also trims before each API call for precise context management.
	newHistory = llm.TrimMessages(newHistory, maxStoredTokens)

	historiesMu.Lock()
	histories[m.ChannelID] = newHistory
	historiesMu.Unlock()

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
		end := maxLen
		if end > len(s) {
			end = len(s)
		}
		// Try to split at a newline
		if idx := strings.LastIndex(s[:end], "\n"); idx > 0 {
			end = idx + 1
		}
		chunks = append(chunks, s[:end])
		s = s[end:]
	}
	return chunks
}
