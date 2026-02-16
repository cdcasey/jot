package llm

// TrimMessages trims a message history to fit within a token budget.
//
// The budget should already account for the system prompt, tool definitions,
// and a reserve for the model's output. This function only manages the
// message list itself.
//
// Strategy:
//  1. Group messages into logical units (a user message + assistant reply,
//     or an assistant tool-call + all its tool results).
//  2. Always keep the most recent group (the active turn).
//  3. Drop the oldest groups first until the total fits within budget.
//
// Tool-call pairs are never split — either the whole exchange stays or goes.
func TrimMessages(messages []Message, maxTokens int) []Message {
	if len(messages) == 0 {
		return messages
	}

	groups := groupMessages(messages)

	total := 0
	for _, g := range groups {
		total += g.tokens
	}

	if total <= maxTokens {
		return messages
	}

	// Always keep the last group (active turn). Trim from the front.
	kept := total
	dropUntil := 0
	for dropUntil < len(groups)-1 && kept > maxTokens {
		kept -= groups[dropUntil].tokens
		dropUntil++
	}

	// Rebuild the message slice from the surviving groups.
	var trimmed []Message
	for _, g := range groups[dropUntil:] {
		trimmed = append(trimmed, g.messages...)
	}
	return trimmed
}

// messageGroup is a logical unit of conversation that must be kept or
// dropped as a whole. For example, an assistant message with tool calls
// plus all the subsequent tool-result messages form one group.
type messageGroup struct {
	messages []Message
	tokens   int
}

// groupMessages splits a message slice into logical groups:
//
//   - A user message (non-tool-result) is its own group.
//   - An assistant message with no tool calls is its own group.
//   - An assistant message with tool calls + the following tool-result
//     messages form a single group.
func groupMessages(messages []Message) []messageGroup {
	var groups []messageGroup
	i := 0
	for i < len(messages) {
		msg := messages[i]

		// Assistant message with tool calls: group it with subsequent tool results.
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			group := messageGroup{}
			group.messages = append(group.messages, msg)
			group.tokens += EstimateMessageTokens(msg)
			i++
			// Collect all tool result messages that follow.
			for i < len(messages) && messages[i].ToolCallID != "" {
				group.messages = append(group.messages, messages[i])
				group.tokens += EstimateMessageTokens(messages[i])
				i++
			}
			groups = append(groups, group)
			continue
		}

		// Any other message is its own group.
		groups = append(groups, messageGroup{
			messages: []Message{msg},
			tokens:   EstimateMessageTokens(msg),
		})
		i++
	}
	return groups
}
