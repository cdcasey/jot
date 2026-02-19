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
	"github.com/chris/jot/internal/service"
)

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1]); err != nil {
			log.Fatal(err)
		}
		return
	}

	run()
}

func runCommand(cmd string) error {
	switch cmd {
	case "install":
		return service.Install()
	case "uninstall":
		return service.Uninstall()
	case "start":
		return service.Start()
	case "stop":
		return service.Stop()
	case "restart":
		return service.Restart()
	case "status":
		return service.Status()
	case "logs":
		return service.Logs()
	case "run":
		run()
		return nil
	default:
		return fmt.Errorf("unknown command: %s\nusage: jot [install|uninstall|start|stop|restart|status|logs|run]", cmd)
	}
}

func run() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	apiKey := cfg.AnthropicKey
	if cfg.LLMProvider == "openai" {
		apiKey = cfg.OpenAIKey
	}

	client, err := llm.NewClient(llm.ProviderConfig{
		Provider:  cfg.LLMProvider,
		APIKey:    apiKey,
		AuthToken: cfg.AnthropicToken,
		Model:     cfg.LLMModel,
		BaseURL:   cfg.OllamaBaseURL,
	})
	if err != nil {
		log.Fatalf("failed to create LLM client: %v", err)
	}

	ag := agent.New(database, client, cfg.MaxContextTokens)

	// If Discord token is set, run as bot
	if cfg.DiscordToken != "" {
		runBot(cfg, database, ag)
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

	var history []llm.Message

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

		reply, newHistory, err := ag.Run(ctx, history, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			fmt.Println(reply)
			history = newHistory
		}

		if isPipe {
			break // single exchange in pipe mode
		}
		fmt.Print("jot> ")
	}
}

func runBot(cfg *config.Config, database *db.DB, ag *agent.Agent) {
	bot, err := discord.NewBot(cfg.DiscordToken, ag)
	if err != nil {
		log.Fatalf("failed to start Discord bot: %v", err)
	}
	defer bot.Close()

	// Start scheduler if cron is configured
	if cfg.CheckInCron != "" && cfg.DiscordWebhook != "" {
		sched := scheduler.New(cfg.CheckInCron, cfg.DiscordWebhook, database, ag)
		sched.Start()
		defer sched.Stop()
	}

	log.Println("bot is running. Press Ctrl+C to exit.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down.")
}
