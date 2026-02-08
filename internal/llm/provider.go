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
	case "ollama":
		if cfg.Model == "" {
			cfg.Model = "llama3.1"
		}
		return NewOpenAIClient("ollama", cfg.Model, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
