package llm

import "encoding/json"

// charsPerToken is the average number of characters per token.
// This is a rough heuristic — real tokenizers vary, but 4 chars/token
// is a well-known approximation for English text and works well enough
// for context budgeting.
const charsPerToken = 4

// EstimateTokens returns a rough token count for a string.
func EstimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) + charsPerToken - 1) / charsPerToken // round up
}

// EstimateMessageTokens returns the estimated token count for a single message.
// This accounts for content, tool calls, and per-message overhead (role, framing).
func EstimateMessageTokens(m Message) int {
	tokens := 4 // per-message overhead (role tokens, delimiters)
	tokens += EstimateTokens(m.Content)
	for _, tc := range m.ToolCalls {
		tokens += EstimateTokens(tc.Name)
		if params, err := json.Marshal(tc.Params); err == nil {
			tokens += EstimateTokens(string(params))
		}
		tokens += 4 // tool call framing overhead
	}
	if m.ToolCallID != "" {
		tokens += EstimateTokens(m.ToolCallID) + 2
	}
	return tokens
}

// EstimateMessagesTokens returns the total estimated tokens for a slice of messages.
func EstimateMessagesTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateMessageTokens(m)
	}
	return total
}

// EstimateToolsTokens returns the estimated tokens for tool definitions.
// Tool schemas are serialized as JSON in API requests and count against the context.
func EstimateToolsTokens(tools []Tool) int {
	total := 0
	for _, t := range tools {
		total += EstimateTokens(t.Name)
		total += EstimateTokens(t.Description)
		if schema, err := json.Marshal(t.Parameters); err == nil {
			total += EstimateTokens(string(schema))
		}
		total += 10 // per-tool framing overhead
	}
	return total
}
