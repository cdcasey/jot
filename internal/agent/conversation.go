package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chris/jot/internal/llm"
)

const (
	conversationGap    = 10 * time.Minute
	summaryContextMax  = 3
	summarizePrompt    = "Summarize the following conversation concisely. Focus on: what was discussed, any decisions made, action items, and important context the user shared. Keep it under 200 words."
)

// RunWithConversation loads persistent conversation history, handles gap
// detection and summarization, runs the agent, and saves the updated history.
func (a *Agent) RunWithConversation(ctx context.Context, userID, message string) (string, error) {
	// Load existing conversation
	history, lastAt, err := a.db.LoadConversation(userID)
	if err != nil {
		return "", fmt.Errorf("loading conversation: %w", err)
	}

	// Gap detection: if enough time has passed, summarize and clear
	if len(history) > 0 && !lastAt.IsZero() && time.Since(lastAt) > conversationGap {
		summary, err := a.Summarize(ctx, history)
		if err != nil {
			// Don't lose messages on summarization failure — just log and continue
			log.Printf("summarization failed for %s, keeping raw messages: %v", userID, err)
		} else {
			if _, err := a.db.SaveConversationSummary(userID, summary, len(history)); err != nil {
				log.Printf("saving summary for %s: %v", userID, err)
			}
			history = nil
			if err := a.db.ClearConversation(userID); err != nil {
				log.Printf("clearing conversation for %s: %v", userID, err)
			}
		}
	}

	// Prepend recent summaries as context
	summaries, err := a.db.GetRecentSummaries(userID, summaryContextMax)
	if err != nil {
		log.Printf("loading summaries for %s: %v", userID, err)
	}
	var contextMessages []llm.Message
	if len(summaries) > 0 {
		// Build a single context block from summaries (oldest first)
		var sb strings.Builder
		sb.WriteString("Here's context from our previous conversations:\n\n")
		for i := len(summaries) - 1; i >= 0; i-- {
			fmt.Fprintf(&sb, "- %s\n", summaries[i].Summary)
		}
		contextMessages = []llm.Message{
			{Role: "user", Content: sb.String()},
			{Role: "assistant", Content: "Got it, I have that context."},
		}
	}

	// Build full history: context summaries + raw messages
	fullHistory := append(contextMessages, history...)

	// Run the agent
	reply, newHistory, err := a.Run(ctx, fullHistory, message)
	if err != nil {
		return "", err
	}

	// Strip the synthetic context messages before saving — we'll re-inject them next time
	if len(contextMessages) > 0 && len(newHistory) > len(contextMessages) {
		newHistory = newHistory[len(contextMessages):]
	}

	// Trim before persisting
	fixedTokens := llm.EstimateTokens(llm.SystemPrompt) + llm.EstimateToolsTokens(llm.AgentTools)
	budget := max(a.MaxContextTokens-fixedTokens, 1000)
	newHistory = llm.TrimMessages(newHistory, budget)

	if err := a.db.SaveConversation(userID, newHistory); err != nil {
		log.Printf("saving conversation for %s: %v", userID, err)
	}

	return reply, nil
}

// Summarize calls the LLM with a summarization prompt and no tools to produce
// a concise summary of the given messages.
func (a *Agent) Summarize(ctx context.Context, messages []llm.Message) (string, error) {
	// Build a flat text representation of the conversation for the summarizer
	var sb strings.Builder
	for _, m := range messages {
		if m.ToolCallID != "" {
			continue // skip tool results — they're noise for summaries
		}
		if m.Content == "" {
			continue
		}
		var role string
		switch m.Role {
		case "assistant":
			role = "Agent"
		case "user":
			role = "User"
		default:
			role = m.Role
		}
		fmt.Fprintf(&sb, "%s: %s\n", role, m.Content)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no summarizable content")
	}

	summaryMessages := []llm.Message{
		{Role: "user", Content: sb.String()},
	}

	resp, err := a.client.Chat(ctx, summarizePrompt, summaryMessages, nil)
	if err != nil {
		return "", fmt.Errorf("summarization LLM call: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}
