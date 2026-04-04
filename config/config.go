package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// ModelConfig defines a named LLM model from config.yaml.
type ModelConfig struct {
	Provider    string   `yaml:"provider"`
	Model       string   `yaml:"model"`
	BaseURL     string   `yaml:"base_url"`
	Temperature *float64 `yaml:"temperature"`
}

// YAMLConfig is the top-level structure of config.yaml.
type YAMLConfig struct {
	Models      map[string]ModelConfig `yaml:"models"`
	ActiveModel string                `yaml:"active_model"`
}

type Config struct {
	// LLM (resolved from YAML active_model)
	LLMProvider    string
	LLMModel       string
	LLMAPIKey      string
	LLMAuthToken   string   // Anthropic OAuth token
	LLMBaseURL     string
	LLMTemperature *float64

	// All defined models (for eval or future multi-model use)
	Models      map[string]ModelConfig
	ActiveModel string

	// App
	DiscordToken     string
	DiscordWebhook   string
	DiscordUserID    string
	DatabasePath     string
	CheckInCron      string
	MaxContextTokens int
}

func Load() *Config {
	return LoadFrom("config.yaml")
}

// LoadFrom loads config from the given YAML path (if it exists) plus .env and
// environment variables. If the YAML file is missing, all LLM settings fall
// back to env vars for backward compatibility.
func LoadFrom(yamlPath string) *Config {
	_ = godotenv.Load() // secrets from .env

	cfg := &Config{
		DiscordToken:     os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordWebhook:   os.Getenv("DISCORD_WEBHOOK_URL"),
		DiscordUserID:    os.Getenv("DISCORD_USER_ID"),
		DatabasePath:     envOr("DATABASE_PATH", "./data.db"),
		CheckInCron:      envOr("CHECK_IN_CRON", "0 9 * * *"),
		MaxContextTokens: envInt("MAX_CONTEXT_TOKENS", 180000),
		LLMAuthToken:     os.Getenv("ANTHROPIC_AUTH_TOKEN"),
	}

	yc, err := loadYAML(yamlPath)
	if err != nil {
		// No config.yaml — fall back to env vars for backward compat.
		cfg.LLMProvider = envOr("LLM_PROVIDER", "anthropic")
		cfg.LLMModel = os.Getenv("LLM_MODEL")
		cfg.LLMTemperature = envFloat64("LLM_TEMPERATURE")
		cfg.LLMBaseURL = envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1")
		cfg.LLMAPIKey = resolveAPIKey(cfg.LLMProvider)
		return cfg
	}

	cfg.Models = yc.Models
	cfg.ActiveModel = yc.ActiveModel

	mc, ok := yc.Models[yc.ActiveModel]
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: active_model %q not found in config.yaml, falling back to env vars\n", yc.ActiveModel)
		cfg.LLMProvider = envOr("LLM_PROVIDER", "anthropic")
		cfg.LLMModel = os.Getenv("LLM_MODEL")
		cfg.LLMTemperature = envFloat64("LLM_TEMPERATURE")
		cfg.LLMBaseURL = envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1")
		cfg.LLMAPIKey = resolveAPIKey(cfg.LLMProvider)
		return cfg
	}

	cfg.LLMProvider = mc.Provider
	cfg.LLMModel = mc.Model
	cfg.LLMBaseURL = mc.BaseURL
	cfg.LLMTemperature = mc.Temperature
	cfg.LLMAPIKey = resolveAPIKey(mc.Provider)

	return cfg
}

func loadYAML(path string) (*YAMLConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yc YAMLConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &yc, nil
}

// resolveAPIKey picks the right env var based on provider (env-var fallback path only).
func resolveAPIKey(provider string) string {
	switch provider {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "gemini":
		return os.Getenv("GEMINI_API_KEY")
	default:
		return os.Getenv("ANTHROPIC_API_KEY")
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

func envFloat64(key string) *float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return &f
		}
	}
	return nil
}
