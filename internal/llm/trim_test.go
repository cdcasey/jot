package llm

import (
	"strings"
	"testing"
)

func TestTrimMessages_UnderBudget(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	got := TrimMessages(msgs, 100000)
	if len(got) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(got))
	}
}

func TestTrimMessages_Empty(t *testing.T) {
	got := TrimMessages(nil, 100)
	if len(got) != 0 {
		t.Errorf("expected 0 messages, got %d", len(got))
	}
}

func TestTrimMessages_DropsOldestFirst(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
		{Role: "assistant", Content: "second answer"},
		{Role: "user", Content: "third question"},
		{Role: "assistant", Content: "third answer"},
	}

	// Budget enough for ~2 groups only (the last 2 messages).
	// Each message is roughly 4 (overhead) + content tokens.
	// Use a budget that forces at least the first pair to be dropped.
	budget := EstimateMessagesTokens(msgs[2:])
	got := TrimMessages(msgs, budget)

	if len(got) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(got))
	}
	// The oldest messages should have been dropped.
	if got[0].Content == "first question" {
		t.Error("expected oldest messages to be trimmed, but 'first question' is still present")
	}
	// The newest messages should be preserved.
	last := got[len(got)-1]
	if last.Content != "third answer" {
		t.Errorf("expected last message to be 'third answer', got %q", last.Content)
	}
}

func TestTrimMessages_KeepsToolCallPairsTogether(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "old question"},
		{Role: "assistant", Content: "old answer"},
		// A tool-call exchange that should stay as a unit.
		{Role: "user", Content: "what are my todos?"},
		{
			Role:      "assistant",
			ToolCalls: []ToolCall{{ID: "call_1", Name: "list_todos", Params: map[string]any{}}},
		},
		{Role: "user", Content: `[{"id":1,"title":"Buy milk"}]`, ToolCallID: "call_1"},
		{Role: "assistant", Content: "You have one todo: Buy milk."},
	}

	// Budget: enough for the tool exchange + final answer but not the old pair.
	budget := EstimateMessagesTokens(msgs[2:])
	got := TrimMessages(msgs, budget)

	// Verify the old pair was dropped.
	for _, m := range got {
		if m.Content == "old question" || m.Content == "old answer" {
			t.Errorf("expected old messages to be trimmed, found %q", m.Content)
		}
	}

	// Verify tool call and result stayed together.
	hasToolCall := false
	hasToolResult := false
	for _, m := range got {
		if len(m.ToolCalls) > 0 && m.ToolCalls[0].ID == "call_1" {
			hasToolCall = true
		}
		if m.ToolCallID == "call_1" {
			hasToolResult = true
		}
	}
	if hasToolCall != hasToolResult {
		t.Error("tool call and tool result were split — they must stay together")
	}
}

func TestTrimMessages_MultipleToolResults(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "do two things"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call_a", Name: "list_todos", Params: map[string]any{}},
				{ID: "call_b", Name: "get_summary", Params: map[string]any{}},
			},
		},
		{Role: "user", Content: `[]`, ToolCallID: "call_a"},
		{Role: "user", Content: `{"active":1}`, ToolCallID: "call_b"},
		{Role: "assistant", Content: "Done."},
	}

	// The tool-call group (assistant + 2 results) must stay together.
	groups := groupMessages(msgs)
	// Expected: [user "do two things"] [assistant+call_a+call_b] [assistant "Done."]
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[1].messages) != 3 {
		t.Errorf("tool-call group should have 3 messages (assistant + 2 results), got %d", len(groups[1].messages))
	}
}

func TestTrimMessages_AlwaysKeepsLastGroup(t *testing.T) {
	// Even if the last group alone exceeds the budget, we still keep it
	// (the caller should handle the truly-too-large case).
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("x", 10000)},
	}
	got := TrimMessages(msgs, 1)
	if len(got) != 1 {
		t.Errorf("expected last group to be preserved even over budget, got %d messages", len(got))
	}
}

func TestGroupMessages(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{
			Role:      "assistant",
			ToolCalls: []ToolCall{{ID: "c1", Name: "get_summary", Params: map[string]any{}}},
		},
		{Role: "user", Content: `{}`, ToolCallID: "c1"},
		{Role: "assistant", Content: "a2"},
	}

	groups := groupMessages(msgs)

	// user "q1" | assistant "a1" | user "q2" | assistant+toolresult | assistant "a2"
	if len(groups) != 5 {
		t.Fatalf("expected 5 groups, got %d", len(groups))
	}
	if len(groups[3].messages) != 2 {
		t.Errorf("tool-call group should have 2 messages, got %d", len(groups[3].messages))
	}
}
