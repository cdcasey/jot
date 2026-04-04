package config

import (
	"os"
	"path/filepath"
	"testing"
)

// clearLLMEnv unsets all LLM-related env vars so tests start clean.
// Returns a restore function.
func clearLLMEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"LLM_PROVIDER", "LLM_MODEL", "LLM_TEMPERATURE",
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN",
		"OPENAI_API_KEY", "GEMINI_API_KEY",
		"OLLAMA_BASE_URL",
		"DISCORD_BOT_TOKEN", "DISCORD_WEBHOOK_URL", "DISCORD_USER_ID",
		"DATABASE_PATH", "CHECK_IN_CRON", "MAX_CONTEXT_TOKENS",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFrom_YAMLAnthropicModel(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-123")

	path := writeYAML(t, `
models:
  my-claude:
    provider: anthropic
    model: claude-sonnet-4-20250514
    temperature: 0.5
active_model: my-claude
`)

	cfg := LoadFrom(path)

	if cfg.LLMProvider != "anthropic" {
		t.Errorf("provider = %q, want anthropic", cfg.LLMProvider)
	}
	if cfg.LLMModel != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", cfg.LLMModel)
	}
	if cfg.LLMAPIKey != "sk-test-123" {
		t.Errorf("api key = %q, want sk-test-123", cfg.LLMAPIKey)
	}
	if cfg.LLMTemperature == nil || *cfg.LLMTemperature != 0.5 {
		t.Errorf("temperature = %v, want 0.5", cfg.LLMTemperature)
	}
	if cfg.ActiveModel != "my-claude" {
		t.Errorf("active_model = %q, want my-claude", cfg.ActiveModel)
	}
}

func TestLoadFrom_YAMLOpenAIModel(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai-test")

	path := writeYAML(t, `
models:
  gpt:
    provider: openai
    model: gpt-4o
active_model: gpt
`)

	cfg := LoadFrom(path)

	if cfg.LLMProvider != "openai" {
		t.Errorf("provider = %q, want openai", cfg.LLMProvider)
	}
	if cfg.LLMAPIKey != "sk-openai-test" {
		t.Errorf("api key = %q, want sk-openai-test", cfg.LLMAPIKey)
	}
	if cfg.LLMTemperature != nil {
		t.Errorf("temperature = %v, want nil", *cfg.LLMTemperature)
	}
}

func TestLoadFrom_YAMLOllamaBaseURL(t *testing.T) {
	clearLLMEnv(t)

	path := writeYAML(t, `
models:
  local:
    provider: ollama
    model: llama3.1
    base_url: http://myhost:11434/v1
active_model: local
`)

	cfg := LoadFrom(path)

	if cfg.LLMProvider != "ollama" {
		t.Errorf("provider = %q, want ollama", cfg.LLMProvider)
	}
	if cfg.LLMBaseURL != "http://myhost:11434/v1" {
		t.Errorf("base_url = %q, want http://myhost:11434/v1", cfg.LLMBaseURL)
	}
	if cfg.LLMAPIKey != "" {
		t.Errorf("api key = %q, want empty for ollama", cfg.LLMAPIKey)
	}
}

func TestLoadFrom_MultipleModels(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai")

	path := writeYAML(t, `
models:
  claude:
    provider: anthropic
    model: claude-sonnet-4-20250514
  gpt:
    provider: openai
    model: gpt-4o
active_model: gpt
`)

	cfg := LoadFrom(path)

	if cfg.LLMProvider != "openai" {
		t.Errorf("provider = %q, want openai (active_model=gpt)", cfg.LLMProvider)
	}
	if len(cfg.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(cfg.Models))
	}
	if _, ok := cfg.Models["claude"]; !ok {
		t.Error("models map missing 'claude'")
	}
}

func TestLoadFrom_InvalidActiveModel(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-fallback")

	path := writeYAML(t, `
models:
  claude:
    provider: anthropic
    model: claude-sonnet-4-20250514
active_model: nonexistent
`)

	cfg := LoadFrom(path)

	// Should fall back to env vars.
	if cfg.LLMProvider != "openai" {
		t.Errorf("provider = %q, want openai (env fallback)", cfg.LLMProvider)
	}
	if cfg.LLMAPIKey != "sk-fallback" {
		t.Errorf("api key = %q, want sk-fallback", cfg.LLMAPIKey)
	}
}

func TestLoadFrom_NoYAML_EnvFallback(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gem-key")
	t.Setenv("LLM_MODEL", "gemini-2.5-flash")
	t.Setenv("LLM_TEMPERATURE", "0.3")

	cfg := LoadFrom("/nonexistent/config.yaml")

	if cfg.LLMProvider != "gemini" {
		t.Errorf("provider = %q, want gemini", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gemini-2.5-flash" {
		t.Errorf("model = %q, want gemini-2.5-flash", cfg.LLMModel)
	}
	if cfg.LLMAPIKey != "gem-key" {
		t.Errorf("api key = %q, want gem-key", cfg.LLMAPIKey)
	}
	if cfg.LLMTemperature == nil || *cfg.LLMTemperature != 0.3 {
		t.Errorf("temperature = %v, want 0.3", cfg.LLMTemperature)
	}
}

func TestLoadFrom_NoYAML_DefaultsToAnthropic(t *testing.T) {
	clearLLMEnv(t)

	cfg := LoadFrom("/nonexistent/config.yaml")

	if cfg.LLMProvider != "anthropic" {
		t.Errorf("provider = %q, want anthropic (default)", cfg.LLMProvider)
	}
	if cfg.DatabasePath != "./data.db" {
		t.Errorf("database path = %q, want ./data.db", cfg.DatabasePath)
	}
	if cfg.MaxContextTokens != 180000 {
		t.Errorf("max context tokens = %d, want 180000", cfg.MaxContextTokens)
	}
}

func TestResolveAPIKey(t *testing.T) {
	tests := []struct {
		provider string
		envKey   string
		envVal   string
	}{
		{"anthropic", "ANTHROPIC_API_KEY", "ant-key"},
		{"openai", "OPENAI_API_KEY", "oai-key"},
		{"gemini", "GEMINI_API_KEY", "gem-key"},
		{"ollama", "ANTHROPIC_API_KEY", ""}, // ollama has no key
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			clearLLMEnv(t)
			if tt.envVal != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}
			got := resolveAPIKey(tt.provider)
			if got != tt.envVal {
				t.Errorf("resolveAPIKey(%q) = %q, want %q", tt.provider, got, tt.envVal)
			}
		})
	}
}
