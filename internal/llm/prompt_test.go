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

func TestNoMemoryTools(t *testing.T) {
	memoryTools := map[string]bool{
		"save_memory": true, "search_memories": true, "list_recent_memories": true,
		"update_memory": true, "delete_memory": true,
	}
	for _, tool := range AgentTools {
		if memoryTools[tool.Name] {
			t.Errorf("memory tool %q should have been removed", tool.Name)
		}
	}
}

func TestSystemPromptNoMemoryReferences(t *testing.T) {
	for _, term := range []string{"save_memory", "search_memories", "list_recent_memories"} {
		if strings.Contains(SystemPrompt, term) {
			t.Errorf("system prompt should not reference %q", term)
		}
	}
}
