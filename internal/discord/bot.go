package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/chris/jot/internal/agent"
)

type Bot struct {
	session *discordgo.Session
	agent   *agent.Agent
}

func NewBot(token string, ag *agent.Agent) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("creating Discord session: %w", err)
	}

	bot := &Bot{session: s, agent: ag}
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
