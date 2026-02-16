package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	LLMProvider      string // anthropic, openai, ollama
	AnthropicKey     string // API key (X-Api-Key header)
	AnthropicToken   string // OAuth token (Authorization: Bearer header)
	OpenAIKey        string
	LLMModel         string
	OllamaBaseURL    string
	DiscordToken     string
	DiscordWebhook   string
	DatabasePath     string
	CheckInCron      string
	MaxContextTokens int    // max tokens for LLM context window (0 = use default)
}

func Load() *Config {
	_ = godotenv.Load() // ignore error if no .env

	return &Config{
		LLMProvider:      envOr("LLM_PROVIDER", "anthropic"),
		AnthropicKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicToken:   os.Getenv("ANTHROPIC_AUTH_TOKEN"),
		OpenAIKey:        os.Getenv("OPENAI_API_KEY"),
		LLMModel:         os.Getenv("LLM_MODEL"),
		OllamaBaseURL:    envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1"),
		DiscordToken:     os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordWebhook:   os.Getenv("DISCORD_WEBHOOK_URL"),
		DatabasePath:     envOr("DATABASE_PATH", "./data.db"),
		CheckInCron:      envOr("CHECK_IN_CRON", "0 9 * * *"),
		MaxContextTokens: envInt("MAX_CONTEXT_TOKENS", 0),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
