package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chris/jot/config"
	"github.com/chris/jot/internal/llm"
	"github.com/joho/godotenv"
)

func TestEval(t *testing.T) {
	if os.Getenv("RUN_EVAL") == "" {
		t.Skip("skipping eval (set RUN_EVAL=1 or use `make eval`)")
	}

	// Load .env from project root (same as the main app).
	_ = godotenv.Load("../.env")

	// Load YAML config from project root (same as .env above).
	cfg := config.LoadFrom("../config.yaml")

	agentClient, agentModel := buildClient(t, cfg, "LLM_PROVIDER", "LLM_MODEL")
	judgeClient, judgeModel := buildClient(t, cfg, "LLM_EVAL_PROVIDER", "LLM_EVAL_MODEL")

	casesPath := filepath.Join(".", "cases.json")
	if _, err := os.Stat(casesPath); os.IsNotExist(err) {
		casesPath = filepath.Join("eval", "cases.json")
	}

	RunEval(t, casesPath, agentClient, judgeClient, agentModel, judgeModel)
}

// buildClient creates an LLM client. Env var overrides (e.g. LLM_EVAL_MODEL)
// take precedence over the YAML active_model, so you can still do:
//
//	LLM_MODEL=claude-haiku-3-5-20241022 make eval
func buildClient(t *testing.T, cfg *config.Config, providerVar, modelVar string) (llm.Client, string) {
	t.Helper()

	provider := envOr(providerVar, "")
	model := envOr(modelVar, "")

	// Fall back to the base vars if the specific ones aren't set.
	if provider == "" {
		provider = envOr("LLM_PROVIDER", "")
	}
	if model == "" {
		model = envOr("LLM_MODEL", "")
	}

	// If still empty, use the YAML active_model config.
	if provider == "" {
		provider = cfg.LLMProvider
	}
	if model == "" {
		model = cfg.LLMModel
	}
	if provider == "" {
		provider = "anthropic"
	}

	pcfg := llm.ProviderConfig{
		Provider:  provider,
		Model:     model,
		AuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
	}

	// Resolve API key: check YAML models for an api_key_env pointer,
	// otherwise fall back to well-known env vars.
	pcfg.APIKey = resolveAPIKey(provider)

	if provider == "ollama" {
		pcfg.BaseURL = envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1")
		if mc, ok := cfg.Models[cfg.ActiveModel]; ok && mc.Provider == "ollama" && mc.BaseURL != "" {
			pcfg.BaseURL = mc.BaseURL
		}
	}

	client, err := llm.NewClient(pcfg)
	if err != nil {
		t.Fatalf("creating %s/%s client: %v", providerVar, modelVar, err)
	}

	displayModel := model
	if displayModel == "" {
		displayModel = provider + " (default)"
	}
	return client, displayModel
}

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
