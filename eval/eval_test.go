package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chris/jot/internal/llm"
)

func TestEval(t *testing.T) {
	// Require explicit opt-in — don't run during `go test ./...`
	if os.Getenv("RUN_EVAL") == "" {
		t.Skip("skipping eval (set RUN_EVAL=1 or use `make eval`)")
	}

	provider := envOr("LLM_PROVIDER", "anthropic")
	model := envOr("LLM_MODEL", "")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	cfg := llm.ProviderConfig{
		Provider:  provider,
		Model:     model,
		APIKey:    apiKey,
		AuthToken: authToken,
	}
	if provider == "openai" {
		cfg.APIKey = openaiKey
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("creating LLM client: %v", err)
	}

	casesPath := filepath.Join(".", "cases.json")
	if _, err := os.Stat(casesPath); os.IsNotExist(err) {
		// Try from repo root.
		casesPath = filepath.Join("eval", "cases.json")
	}

	displayModel := model
	if displayModel == "" {
		displayModel = provider + " (default)"
	}

	RunEval(t, casesPath, client, displayModel)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
