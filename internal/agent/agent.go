package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/llm"
)

const maxToolRounds = 10

type Agent struct {
	db               *db.DB
	client           llm.Client
	MaxContextTokens int
}

func New(database *db.DB, client llm.Client, maxContextTokens int) *Agent {
	return &Agent{db: database, client: client, MaxContextTokens: maxContextTokens}
}

// Run takes a user message, runs the tool-calling loop, and returns the final text response.
func (a *Agent) Run(ctx context.Context, history []llm.Message, userMessage string) (string, []llm.Message, error) {
	messages := make([]llm.Message, len(history))
	copy(messages, history)
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	// Fixed costs: system prompt + tool definitions.
	fixedTokens := llm.EstimateTokens(llm.SystemPrompt) + llm.EstimateToolsTokens(llm.AgentTools)
	messageBudget := a.MaxContextTokens - fixedTokens
	if messageBudget < 1000 {
		messageBudget = 1000 // floor so we always have room for at least the current turn
	}

	for i := 0; i < maxToolRounds; i++ {
		trimmed := llm.TrimMessages(messages, messageBudget)
		if len(trimmed) < len(messages) {
			log.Printf("context trimmed: %d → %d messages", len(messages), len(trimmed))
		}
		resp, err := a.client.Chat(ctx, llm.SystemPrompt, trimmed, llm.AgentTools)
		if err != nil {
			return "", nil, fmt.Errorf("llm chat: %w", err)
		}

		// No tool calls — we have a final answer
		if len(resp.ToolCalls) == 0 {
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
			return resp.Content, messages, nil
		}

		// Append assistant message with tool calls
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results
		for _, tc := range resp.ToolCalls {
			result := a.executeTool(tc.Name, tc.Params)
			log.Printf("tool %s → %s", tc.Name, truncate(result, 200))
			messages = append(messages, llm.Message{
				Role:       "user",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "I hit the maximum number of tool calls. Here's what I have so far.", messages, nil
}

func (a *Agent) executeTool(name string, params map[string]any) string {
	var result any
	var err error

	switch name {
	case "list_things":
		status, _ := getString(params, "status")
		priority, _ := getString(params, "priority")
		tag, _ := getString(params, "tag")
		result, err = a.db.ListThings(status, priority, tag)

	case "create_thing":
		title, _ := getString(params, "title")
		notes, _ := getString(params, "notes")
		priority, _ := getString(params, "priority")
		dueDate, _ := getString(params, "due_date")
		var tags []string
		if v, ok := params["tags"]; ok {
			if arr, ok := v.([]any); ok {
				for _, t := range arr {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}
		}
		id, e := a.db.CreateThing(title, notes, priority, dueDate, tags)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "update_thing":
		id, _ := getInt(params, "id")
		fields := make(map[string]any)
		for _, k := range []string{"title", "notes", "status", "priority", "due_date"} {
			if v, ok := params[k]; ok {
				fields[k] = v
			}
		}
		if v, ok := params["tags"]; ok {
			if arr, ok := v.([]any); ok {
				var tags []string
				for _, t := range arr {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
				b, _ := json.Marshal(tags)
				fields["tags"] = string(b)
			}
		}
		err = a.db.UpdateThing(id, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "complete_thing":
		id, _ := getInt(params, "id")
		err = a.db.CompleteThing(id)
		if err == nil {
			result = map[string]any{"status": "completed"}
		}

	case "get_summary":
		result, err = a.db.GetSummary()

	case "get_note":
		key, _ := getString(params, "key")
		val, e := a.db.GetNote(key)
		if e != nil {
			err = e
		} else if val == "" {
			result = map[string]any{"value": nil, "message": "no note found for this key"}
		} else {
			result = map[string]any{"value": val}
		}

	case "set_note":
		key, _ := getString(params, "key")
		value, _ := getString(params, "value")
		err = a.db.SetNote(key, value)
		if err == nil {
			result = map[string]any{"status": "saved"}
		}

	case "save_memory":
		content, _ := getString(params, "content")
		category, _ := getString(params, "category")
		expiresAt, _ := getString(params, "expires_at")
		var thingID *int64
		if v, ok := getInt(params, "thing_id"); ok {
			thingID = &v
		}
		var tags []string
		if v, ok := params["tags"]; ok {
			if arr, ok := v.([]any); ok {
				for _, t := range arr {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}
		}
		id, e := a.db.SaveMemory(content, category, "agent", tags, thingID, expiresAt)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "saved"}
		}

	case "search_memories":
		query, _ := getString(params, "query")
		category, _ := getString(params, "category")
		tag, _ := getString(params, "tag")
		since, _ := getString(params, "since")
		limit, _ := getInt(params, "limit")
		var thingID *int64
		if v, ok := getInt(params, "thing_id"); ok {
			thingID = &v
		}
		result, err = a.db.SearchMemories(query, category, tag, thingID, since, int(limit))

	case "update_memory":
		id, _ := getInt(params, "id")
		fields := make(map[string]any)
		for _, k := range []string{"content", "category", "expires_at"} {
			if v, ok := params[k]; ok {
				fields[k] = v
			}
		}
		if v, ok := params["tags"]; ok {
			if arr, ok := v.([]any); ok {
				var tags []string
				for _, t := range arr {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
				b, _ := json.Marshal(tags)
				fields["tags"] = string(b)
			}
		}
		err = a.db.UpdateMemory(id, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "delete_memory":
		id, _ := getInt(params, "id")
		err = a.db.DeleteMemory(id)
		if err == nil {
			result = map[string]any{"status": "deleted"}
		}

	case "list_recent_memories":
		category, _ := getString(params, "category")
		limit, _ := getInt(params, "limit")
		result, err = a.db.ListRecentMemories(category, int(limit))

	case "get_time":
		now := time.Now()
		formattedLocal := now.Format(time.RFC3339)
		formattedUTC := now.UTC().Format(time.RFC3339)
		date := now.Format("2006-01-02")
		weekday := now.Weekday().String()
		result = map[string]any{"local": formattedLocal, "utc": formattedUTC, "date": date, "day": weekday}

	case "create_skill":
		name, _ := getString(params, "name")
		description, _ := getString(params, "description")
		content, _ := getString(params, "content")
		var tags []string
		if v, ok := params["tags"]; ok {
			if arr, ok := v.([]any); ok {
				for _, t := range arr {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}
		}
		id, e := a.db.CreateSkill(name, description, content, tags)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "get_skill":
		name, _ := getString(params, "name")
		skill, e := a.db.GetSkill(name)
		if e != nil {
			err = e
		} else if skill == nil {
			result = map[string]any{"error": "skill not found", "name": name}
		} else {
			result = skill
		}

	case "list_skills":
		tag, _ := getString(params, "tag")
		result, err = a.db.ListSkills(tag)

	case "update_skill":
		name, _ := getString(params, "name")
		fields := make(map[string]any)
		for _, k := range []string{"description", "content", "tags"} {
			if v, ok := params[k]; ok {
				fields[k] = v
			}
		}
		err = a.db.UpdateSkill(name, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "delete_skill":
		name, _ := getString(params, "name")
		err = a.db.DeleteSkill(name)
		if err == nil {
			result = map[string]any{"status": "deleted"}
		}

	case "list_schedules":
		result, err = a.db.ListSchedules(false)

	case "create_schedule":
		name, _ := getString(params, "name")
		cronExpr, _ := getString(params, "cron_expr")
		prompt, _ := getString(params, "prompt")
		id, e := a.db.CreateSchedule(name, cronExpr, prompt)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	// ListSchedules works for now. Create GetScheduleByName if schedules gets too big.
	case "update_schedule":
		name, _ := getString(params, "name")
		sched, e := a.db.ListSchedules(false)
		if e != nil {
			err = e
			break
		}
		var schedID int64
		for _, s := range sched {
			if s.Name == name {
				schedID = s.ID
				break
			}
		}
		if schedID == 0 {
			result = map[string]any{"error": "schedule not found: " + name}
			break
		}
		fields := make(map[string]any)
		if v, ok := getString(params, "cron_expr"); ok {
			fields["cron_expr"] = v
		}
		if v, ok := getString(params, "prompt"); ok {
			fields["prompt"] = v
		}
		if v, ok := params["enabled"]; ok {
			if b, ok := v.(bool); ok {
				if b {
					fields["enabled"] = 1
				} else {
					fields["enabled"] = 0
				}
			}
		}
		err = a.db.UpdateSchedule(schedID, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "delete_schedule":
		name, _ := getString(params, "name")
		err = a.db.DeleteSchedule(name)
		if err == nil {
			result = map[string]any{"status": "deleted"}
		}

	case "create_reminder":
		prompt, _ := getString(params, "prompt")
		fireAt, _ := getString(params, "fire_at")
		id, e := a.db.CreateReminder(prompt, fireAt)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "list_reminders":
		includeFired, _ := params["include_fired"].(bool)
		if includeFired {
			result, err = a.db.ListAllReminders()
		} else {
			result, err = a.db.ListUpcomingReminders()
		}

	case "cancel_reminder":
		id, _ := getInt(params, "id")
		err = a.db.MarkReminderFired(id)
		if err == nil {
			result = map[string]any{"status": "cancelled"}
		}

	default:
		result = map[string]any{"error": "unknown tool: " + name}
	}

	if err != nil {
		result = map[string]any{"error": err.Error()}
	}

	b, _ := json.Marshal(result) // result is always a simple map or slice; marshal cannot fail
	return string(b)
}

// Param extraction helpers — LLMs send numbers as float64 in JSON.
func getInt(params map[string]any, key string) (int64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}

func getString(params map[string]any, key string) (string, bool) {
	v, ok := params[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
