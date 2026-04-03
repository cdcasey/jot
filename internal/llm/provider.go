package llm

import "fmt"

type ProviderConfig struct {
	Provider  string
	APIKey    string
	AuthToken string // OAuth token (Bearer auth)
	Model     string
	BaseURL   string
}

func NewClient(cfg ProviderConfig) (Client, error) {
	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicClient(cfg.APIKey, cfg.AuthToken, cfg.Model), nil
	case "openai":
		return NewOpenAIClient(cfg.APIKey, cfg.Model, ""), nil
	case "gemini":
		if cfg.Model == "" {
			cfg.Model = "gemini-2.5-flash"
		}
		return NewOpenAIClient(cfg.APIKey, cfg.Model, "https://generativelanguage.googleapis.com/v1beta/openai/"), nil
	case "ollama":
		if cfg.Model == "" {
			cfg.Model = "llama3.1"
		}
		return NewOpenAIClient("ollama", cfg.Model, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
