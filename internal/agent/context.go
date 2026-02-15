package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/chris/jot/internal/db"
)

// BuildCheckInPrompt creates a prompt for the scheduled check-in.
func BuildCheckInPrompt(database *db.DB) (string, error) {
	// Prune expired memories
	if _, err := database.PruneExpiredMemories(); err != nil {
		log.Printf("warning: pruning memories: %v", err)
	}

	summary, err := database.GetSummary()
	if err != nil {
		return "", fmt.Errorf("building check-in context: %w", err)
	}
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ") // Summary struct marshal cannot fail

	var b strings.Builder
	b.WriteString("It's time for a check-in.\n\n## Summary\n")
	b.Write(summaryJSON)

	// Last check-in for continuity
	lastSummary, lastDate, err := database.GetLastCheckIn()
	if err != nil {
		log.Printf("warning: getting last check-in: %v", err)
	}
	b.WriteString("\n\n## Last Check-In\n")
	if lastSummary != "" {
		fmt.Fprintf(&b, "(%s): %s", lastDate, lastSummary)
	} else {
		b.WriteString("This is the first check-in.")
	}

	// Recent memories for context
	memories, err := database.GetRecentMemoriesForCheckIn(7)
	if err != nil {
		log.Printf("warning: getting recent memories: %v", err)
	}
	if len(memories) > 0 {
		b.WriteString("\n\n## Recent Memories (last 7 days)\n")
		for _, m := range memories {
			fmt.Fprintf(&b, "- [%s] [%s] %s\n", m.CreatedAt, m.Category, m.Content)
		}
	}

	// Skills tagged for check-ins
	skills, err := database.ListSkills("check-in")
	if err != nil {
		log.Printf("warning: loading check-in skill: %v", err)
	}
	if len(skills) > 0 {
		b.WriteString("\n\n## Available Skills\n")
		for _, s := range skills {
			fmt.Fprintf(&b, "**%s**: %s\n%s\n\n", s.Name, s.Description, s.Content)
		}
	}

	b.WriteString("\n\nBased on the above, provide a brief check-in. Reference specific memories and past context where relevant. If there are blockers from previous conversations, ask if they're resolved. Mention overdue items. Suggest priorities. Keep it concise and useful.")

	return b.String(), nil
}
