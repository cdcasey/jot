package llm

import (
	"context"
	"log"
	"strings"
	"time"
)

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

const MaxRetries = 3

// ChatWithRetry wraps Client.Chat with retry on rate limit (429) errors.
func ChatWithRetry(ctx context.Context, client Client, systemPrompt string, messages []Message, tools []Tool) (*Response, error) {
	for attempt := 0; ; attempt++ {
		resp, err := client.Chat(ctx, systemPrompt, messages, tools)
		if err == nil {
			return resp, nil
		}
		if attempt >= MaxRetries-1 || !strings.Contains(err.Error(), "429") {
			return nil, err
		}
		wait := time.Duration(15*(attempt+1)) * time.Second
		log.Printf("rate limited, retrying in %s (attempt %d/%d)", wait, attempt+1, MaxRetries)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
