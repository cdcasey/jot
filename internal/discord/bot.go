package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
)

type Bot struct {
	session *discordgo.Session
	agent   *agent.Agent
	db      *db.DB
}

func NewBot(token string, ag *agent.Agent, database *db.DB) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("creating Discord session: %w", err)
	}

	bot := &Bot{session: s, agent: ag, db: database}
	s.AddHandler(bot.onMessage)
	s.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsGuildMessages

	if err := s.Open(); err != nil {
		return nil, fmt.Errorf("opening Discord connection: %w", err)
	}

	log.Printf("Discord bot connected as %s", s.State.User.Username)
	return bot, nil
}

func (b *Bot) Close() {
	b.session.Close()
}

// SendDM sends a message to a Discord user via DM channel.
func (b *Bot) SendDM(userID, content string) error {
	ch, err := b.session.UserChannelCreate(userID)
	if err != nil {
		return fmt.Errorf("creating DM channel: %w", err)
	}
	for _, chunk := range splitMessage(content, 2000) {
		if _, err := b.session.ChannelMessageSend(ch.ID, chunk); err != nil {
			return fmt.Errorf("sending DM: %w", err)
		}
	}
	return nil
}
