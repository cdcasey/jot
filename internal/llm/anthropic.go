package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const anthropicAPI = "https://api.anthropic.com/v1/messages"

type AnthropicClient struct {
	apiKey    string
	authToken string
	model     string
	http      *http.Client
}

func NewAnthropicClient(apiKey, authToken, model string) *AnthropicClient {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &AnthropicClient{
		apiKey:    apiKey,
		authToken: authToken,
		model:     model,
		http:      &http.Client{},
	}
}

// Raw API request/response types

type anthRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []anthText    `json:"system,omitempty"`
	Messages  []anthMessage `json:"messages"`
	Tools     []anthTool    `json:"tools,omitempty"`
}

type anthText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthBlock
}

type anthBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthResponse struct {
	Content []anthBlock `json:"content"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *AnthropicClient) Chat(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*Response, error) {
	// Build tools
	anthTools := make([]anthTool, len(tools))
	for i, t := range tools {
		schema := map[string]any{"type": "object"}
		if props, ok := t.Parameters["properties"]; ok {
			schema["properties"] = props
		}
		if req, ok := t.Parameters["required"]; ok {
			schema["required"] = req
		}
		anthTools[i] = anthTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}
	}

	// Build messages
	var anthMsgs []anthMessage
	for _, m := range messages {
		switch m.Role {
		case "user":
			if m.ToolCallID != "" {
				anthMsgs = append(anthMsgs, anthMessage{
					Role: "user",
					Content: []anthBlock{{
						Type:      "tool_result",
						ToolUseID: m.ToolCallID,
						Content:   m.Content,
					}},
				})
			} else {
				anthMsgs = append(anthMsgs, anthMessage{
					Role:    "user",
					Content: m.Content,
				})
			}
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var blocks []anthBlock
				if m.Content != "" {
					blocks = append(blocks, anthBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					inputJSON, _ := json.Marshal(tc.Params)
					blocks = append(blocks, anthBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: inputJSON,
					})
				}
				anthMsgs = append(anthMsgs, anthMessage{
					Role:    "assistant",
					Content: blocks,
				})
			} else {
				anthMsgs = append(anthMsgs, anthMessage{
					Role:    "assistant",
					Content: m.Content,
				})
			}
		}
	}

	reqBody := anthRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    []anthText{{Type: "text", Text: systemPrompt}},
		Messages:  anthMsgs,
		Tools:     anthTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("User-Agent", "jot/1.0")

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	} else if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("anthropic chat: %s %s", resp.Status, string(respBody))
	}

	var anthResp anthResponse
	if err := json.Unmarshal(respBody, &anthResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{}
	for _, block := range anthResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			params := map[string]any{}
			_ = json.Unmarshal(block.Input, &params)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:     block.ID,
				Name:   block.Name,
				Params: params,
			})
		}
	}

	return result, nil
}
