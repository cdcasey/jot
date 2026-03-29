package llm

import (
	"strings"
	"testing"
)

func TestSystemPromptNoGetTime(t *testing.T) {
	if strings.Contains(SystemPrompt, "get_time") {
		t.Error("system prompt should not reference get_time")
	}
}

func TestSystemPromptIsStatic(t *testing.T) {
	// SystemPrompt should be a constant — no time or dynamic content.
	if strings.Contains(SystemPrompt, "2026") {
		t.Error("system prompt should not contain a year — time belongs in the user message")
	}
}

func TestNoGetTimeTool(t *testing.T) {
	for _, tool := range AgentTools {
		if tool.Name == "get_time" {
			t.Error("get_time tool should have been removed")
		}
	}
}
