package llm

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"short", "hi", 1},
		{"exactly four chars", "test", 1},
		{"five chars rounds up", "hello", 2},
		{"typical sentence", "The quick brown fox jumps over the lazy dog.", 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want int
	}{
		{
			name: "simple user message",
			msg:  Message{Role: "user", Content: "hello"},
			want: 4 + 2, // overhead + "hello"
		},
		{
			name: "empty message",
			msg:  Message{Role: "assistant"},
			want: 4, // just overhead
		},
		{
			name: "message with tool call",
			msg: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "list_todos", Params: map[string]any{"status": "pending"}},
				},
			},
			// overhead(4) + name(3) + params_json + tool_framing(4)
			want: 4 + 3 + 5 + 4,
		},
		{
			name: "tool result message",
			msg:  Message{Role: "user", Content: `[{"id":1,"title":"Buy milk"}]`, ToolCallID: "call_1"},
			// overhead(4) + content(8) + toolcallid(2) + framing(2)
			want: 4 + 8 + 2 + 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateMessageTokens(tt.msg)
			if got != tt.want {
				t.Errorf("EstimateMessageTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	got := EstimateMessagesTokens(messages)
	// msg1: 4+2=6, msg2: 4+2=6
	want := 12
	if got != want {
		t.Errorf("EstimateMessagesTokens() = %d, want %d", got, want)
	}
}

func TestEstimateToolsTokens(t *testing.T) {
	tools := []Tool{
		{
			Name:        "get_summary",
			Description: "Get a summary.",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
	got := EstimateToolsTokens(tools)
	if got <= 10 {
		t.Errorf("EstimateToolsTokens() = %d, expected > 10 for a tool with name+desc+schema", got)
	}
}

func TestEstimateToolsTokens_AllAgentTools(t *testing.T) {
	got := EstimateToolsTokens(AgentTools)
	// Sanity check: 21 tools with schemas should be in a reasonable range.
	// This test guards against the estimate being wildly off.
	if got < 200 || got > 5000 {
		t.Errorf("EstimateToolsTokens(AgentTools) = %d, expected between 200 and 5000", got)
	}
	t.Logf("AgentTools estimated tokens: %d", got)
}
