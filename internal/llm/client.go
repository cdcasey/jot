package llm

import "context"

type Message struct {
	Role       string     `json:"role"` // user, assistant, system
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // for tool result messages
}

type ToolCall struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

type Client interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*Response, error)
}
