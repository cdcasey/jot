package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chris/jot/internal/llm"
	"github.com/joho/godotenv"
)

func TestEval(t *testing.T) {
	if os.Getenv("RUN_EVAL") == "" {
		t.Skip("skipping eval (set RUN_EVAL=1 or use `make eval`)")
	}

	// Load .env from project root (same as the main app).
	_ = godotenv.Load("../.env")

	agentClient, agentModel := buildClient(t, "LLM_PROVIDER", "LLM_MODEL")
	judgeClient, _ := buildClient(t, "LLM_EVAL_PROVIDER", "LLM_EVAL_MODEL")

	casesPath := filepath.Join(".", "cases.json")
	if _, err := os.Stat(casesPath); os.IsNotExist(err) {
		casesPath = filepath.Join("eval", "cases.json")
	}

	RunEval(t, casesPath, agentClient, judgeClient, agentModel)
}

// buildClient creates an LLM client from the given env var names.
// The fallback vars (providerVar → LLM_PROVIDER, modelVar → LLM_MODEL) are used
// when the primary vars are empty — so LLM_EVAL_PROVIDER falls back to LLM_PROVIDER.
func buildClient(t *testing.T, providerVar, modelVar string) (llm.Client, string) {
	t.Helper()

	provider := envOr(providerVar, "")
	model := envOr(modelVar, "")

	// Fall back to the base vars if the specific ones aren't set.
	if provider == "" {
		provider = envOr("LLM_PROVIDER", "anthropic")
	}
	if model == "" {
		model = envOr("LLM_MODEL", "")
	}

	cfg := llm.ProviderConfig{
		Provider:  provider,
		Model:     model,
		APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
	}
	if provider == "openai" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("creating %s/%s client: %v", providerVar, modelVar, err)
	}

	displayModel := model
	if displayModel == "" {
		displayModel = provider + " (default)"
	}
	return client, displayModel
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
