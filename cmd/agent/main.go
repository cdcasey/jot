package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
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
	"github.com/chris/jot/internal/web"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	client, err := llm.NewClient(llm.ProviderConfig{
		Provider:    cfg.LLMProvider,
		APIKey:      cfg.LLMAPIKey,
		AuthToken:   cfg.LLMAuthToken,
		Model:       cfg.LLMModel,
		BaseURL:     cfg.LLMBaseURL,
		Temperature: cfg.LLMTemperature,
	})
	if err != nil {
		log.Fatalf("failed to create LLM client: %v", err)
	}

	ag := agent.New(database, client, cfg.MaxContextTokens)

	wr := watch.NewRunner(database, client)
	ag.SetWatchRunner(wr)

	// Embedded web UI: started only when WEB_PORT is set, sharing the DB handle.
	// Runs in both CLI and bot modes.
	startWebServer(cfg, database)

	// If Discord token is set, run as bot
	if cfg.DiscordToken != "" {
		runBot(cfg, database, ag, wr)
		return
	}

	// Otherwise, CLI mode
	runCLI(ag)
}

// startWebServer launches the embedded web UI in a background goroutine when
// WEB_PORT is set. WEB_PORT may be a bare port ("8080" -> binds loopback only)
// or a full host:port ("100.x.y.z:8080") to expose over a tailnet interface.
// There is no application-level auth; access control is delegated to the network
// layer (e.g. Tailscale ACLs).
func startWebServer(cfg *config.Config, database *db.DB) {
	if cfg.WebPort == "" {
		return
	}
	srv, err := web.New(database)
	if err != nil {
		log.Fatalf("failed to init web server: %v", err)
	}
	addr := cfg.WebPort
	if !strings.Contains(addr, ":") {
		addr = "127.0.0.1:" + addr // bare port binds loopback only
	}
	go func() {
		log.Printf("web UI listening on http://%s", addr)
		if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
			log.Printf("web server stopped: %v", err)
		}
	}()
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

		reply, _, err := ag.Run(ctx, nil, input)
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
