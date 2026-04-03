package llm

import "fmt"

type ProviderConfig struct {
	Provider    string
	APIKey      string
	AuthToken   string // OAuth token (Bearer auth)
	Model       string
	BaseURL     string
	Temperature *float64 // nil = provider default
}

func NewClient(cfg ProviderConfig) (Client, error) {
	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicClient(cfg.APIKey, cfg.AuthToken, cfg.Model, cfg.Temperature), nil
	case "openai":
		return NewOpenAIClient(cfg.APIKey, cfg.Model, "", cfg.Temperature), nil
	case "gemini":
		if cfg.Model == "" {
			cfg.Model = "gemini-2.5-flash"
		}
		return NewOpenAIClient(cfg.APIKey, cfg.Model, "https://generativelanguage.googleapis.com/v1beta/openai/", cfg.Temperature), nil
	case "ollama":
		if cfg.Model == "" {
			cfg.Model = "llama3.1"
		}
		return NewOpenAIClient("ollama", cfg.Model, cfg.BaseURL, cfg.Temperature), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
