package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chris/jot/config"
	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/discord"
	"github.com/chris/jot/internal/llm"
	"github.com/chris/jot/internal/scheduler"
	"github.com/chris/jot/internal/watch"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	apiKey := cfg.AnthropicKey
	switch cfg.LLMProvider {
	case "openai":
		apiKey = cfg.OpenAIKey
	case "gemini":
		apiKey = cfg.GeminiKey
	}

	client, err := llm.NewClient(llm.ProviderConfig{
		Provider:    cfg.LLMProvider,
		APIKey:      apiKey,
		AuthToken:   cfg.AnthropicToken,
		Model:       cfg.LLMModel,
		BaseURL:     cfg.OllamaBaseURL,
		Temperature: cfg.LLMTemperature,
	})
	if err != nil {
		log.Fatalf("failed to create LLM client: %v", err)
	}

	ag := agent.New(database, client, cfg.MaxContextTokens)

	wr := watch.NewRunner(database, client)
	ag.SetWatchRunner(wr)

	// If Discord token is set, run as bot
	if cfg.DiscordToken != "" {
		runBot(cfg, database, ag, wr)
		return
	}

	// Otherwise, CLI mode
	runCLI(ag)
}

func runCLI(ag *agent.Agent) {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)

	// Check if stdin is a pipe (non-interactive)
	stat, _ := os.Stdin.Stat()
	isPipe := (stat.Mode() & os.ModeCharDevice) == 0

	if !isPipe {
		fmt.Print("jot> ")
	}

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			if !isPipe {
				fmt.Print("jot> ")
			}
			continue
		}
		if input == "exit" || input == "quit" {
			break
		}

		reply, err := ag.RunWithConversation(ctx, "cli", input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			fmt.Println(reply)
		}

		if isPipe {
			break
		}
		fmt.Print("jot> ")
	}
}

func runBot(cfg *config.Config, database *db.DB, ag *agent.Agent, wr *watch.Runner) {
	bot, err := discord.NewBot(cfg.DiscordToken, ag, database)
	if err != nil {
		log.Fatalf("failed to start Discord bot: %v", err)
	}
	defer bot.Close()

	if cfg.DiscordUserID != "" {
		if err := database.SetNote("discord_user_id", cfg.DiscordUserID); err != nil {
			log.Printf("warning: failed to seed discord_user_id note: %v", err)
		}
	}

	sched := scheduler.New(database, ag, cfg.DiscordWebhook, bot.SendDM, wr)
	sched.SeedDefaultSchedule(cfg.CheckInCron)
	sched.Start()
	defer sched.Stop()

	log.Println("bot is running. Press Ctrl+C to exit.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down.")
}
