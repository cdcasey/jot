package llm

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSystemPromptContainsTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	prompt := BuildSystemPrompt(loc)

	now := time.Now().In(loc)
	weekday := now.Format("Monday")

	if !strings.Contains(prompt, weekday) {
		t.Errorf("expected prompt to contain weekday %q", weekday)
	}
	if !strings.Contains(prompt, "America/New_York") {
		t.Error("expected prompt to contain IANA zone name")
	}
	if strings.Contains(prompt, "get_time") {
		t.Error("prompt should not reference get_time")
	}
}

func TestBuildSystemPromptFallbackUTC(t *testing.T) {
	prompt := BuildSystemPrompt(time.UTC)

	if !strings.Contains(prompt, "UTC") {
		t.Error("expected prompt to contain UTC when given UTC location")
	}
}

func TestNoGetTimeTool(t *testing.T) {
	for _, tool := range AgentTools {
		if tool.Name == "get_time" {
			t.Error("get_time tool should have been removed")
		}
	}
}
