package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/llm"
	"github.com/chris/jot/internal/watch"
)

const maxToolRounds = 10

type Agent struct {
	db               *db.DB
	client           llm.Client
	watchRunner      *watch.Runner
	MaxContextTokens int
}

func New(database *db.DB, client llm.Client, maxContextTokens int) *Agent {
	return &Agent{db: database, client: client, MaxContextTokens: maxContextTokens}
}

// SetWatchRunner sets the watch runner for manual watch execution via tools.
func (a *Agent) SetWatchRunner(wr *watch.Runner) {
	a.watchRunner = wr
}

// Run takes a user message, runs the tool-calling loop, and returns the final text response.
func (a *Agent) Run(ctx context.Context, history []llm.Message, userMessage string) (string, []llm.Message, error) {
	// Prepend current time to user message so the LLM has temporal context
	// without embedding it in the system prompt (which would break caching).
	loc := a.userLocation()
	now := time.Now().In(loc)
	zone, _ := now.Zone()
	timePrefix := fmt.Sprintf("[Current time: %s, %s %s (%s)]\n\n",
		now.Format("Monday"),
		now.Format("2006-01-02 15:04"),
		zone,
		loc.String(),
	)

	messages := make([]llm.Message, len(history))
	copy(messages, history)
	messages = append(messages, llm.Message{Role: "user", Content: timePrefix + userMessage})

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
		resp, err := a.chatWithRetry(ctx, llm.SystemPrompt, trimmed, llm.AgentTools)
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
			result := a.executeTool(ctx, tc.Name, tc.Params)
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

// chatWithRetry wraps client.Chat with retry on rate limit (429) errors.
func (a *Agent) chatWithRetry(ctx context.Context, systemPrompt string, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	return llm.ChatWithRetry(ctx, a.client, systemPrompt, messages, tools)
}

func (a *Agent) executeTool(ctx context.Context, name string, params map[string]any) string {
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

	case "list_schedules":
		result, err = a.db.ListSchedules(false)

	case "create_schedule":
		name, _ := getString(params, "name")
		prompt, _ := getString(params, "prompt")
		fireAt, hasFireAt := getString(params, "fire_at")
		cronExpr, _ := getString(params, "cron_expr")
		if hasFireAt && fireAt != "" {
			// One-shot reminder
			fireAtUTC, convertErr := a.localToUTC(fireAt)
			if convertErr != nil {
				err = convertErr
			} else {
				id, e := a.db.CreateOneShot(name, prompt, fireAtUTC)
				if e != nil {
					err = e
				} else {
					result = map[string]any{"id": id, "status": "created", "fire_at_utc": fireAtUTC}
				}
			}
		} else {
			id, e := a.db.CreateSchedule(name, cronExpr, prompt)
			if e != nil {
				err = e
			} else {
				result = map[string]any{"id": id, "status": "created"}
			}
		}

	case "update_schedule":
		name, _ := getString(params, "name")
		sched, e := a.db.GetScheduleByName(name)
		if e != nil {
			err = e
			break
		}
		if sched == nil {
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
		err = a.db.UpdateSchedule(sched.ID, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "delete_schedule":
		name, _ := getString(params, "name")
		err = a.db.DeleteSchedule(name)
		if err == nil {
			result = map[string]any{"status": "deleted"}
		}

	case "list_watches":
		result, err = a.db.ListWatches(false)

	case "create_watch":
		name, _ := getString(params, "name")
		prompt, _ := getString(params, "prompt")
		cronExpr, _ := getString(params, "cron_expr")
		var urls []string
		if v, ok := params["urls"]; ok {
			if arr, ok := v.([]any); ok {
				for _, u := range arr {
					if s, ok := u.(string); ok {
						urls = append(urls, s)
					}
				}
			}
		}
		id, e := a.db.CreateWatch(name, prompt, urls, cronExpr)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "update_watch":
		name, _ := getString(params, "name")
		w, e := a.db.GetWatchByName(name)
		if e != nil {
			err = e
			break
		}
		if w == nil {
			result = map[string]any{"error": "watch not found: " + name}
			break
		}
		fields := make(map[string]any)
		if v, ok := getString(params, "prompt"); ok {
			fields["prompt"] = v
		}
		if v, ok := getString(params, "cron_expr"); ok {
			fields["cron_expr"] = v
		}
		if v, ok := params["urls"]; ok {
			if arr, ok := v.([]any); ok {
				var urls []string
				for _, u := range arr {
					if s, ok := u.(string); ok {
						urls = append(urls, s)
					}
				}
				b, _ := json.Marshal(urls)
				fields["urls"] = string(b)
			}
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
		err = a.db.UpdateWatch(w.ID, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "delete_watch":
		name, _ := getString(params, "name")
		err = a.db.DeleteWatch(name)
		if err == nil {
			result = map[string]any{"status": "deleted"}
		}

	case "run_watch":
		name, _ := getString(params, "name")
		if a.watchRunner == nil {
			result = map[string]any{"error": "watch runner not configured"}
			break
		}
		w, e := a.db.GetWatchByName(name)
		if e != nil {
			err = e
			break
		}
		if w == nil {
			result = map[string]any{"error": "watch not found: " + name}
			break
		}
		newResults, e := a.watchRunner.RunWatch(ctx, *w)
		if e != nil {
			err = e
		} else {
			result = map[string]any{
				"new_items": len(newResults),
				"results":   newResults,
			}
		}

	case "list_watch_results":
		name, _ := getString(params, "name")
		w, e := a.db.GetWatchByName(name)
		if e != nil {
			err = e
			break
		}
		if w == nil {
			result = map[string]any{"error": "watch not found: " + name}
			break
		}
		unnotifiedOnly := false
		if v, ok := params["unnotified_only"]; ok {
			if b, ok := v.(bool); ok {
				unnotifiedOnly = b
			}
		}
		limit := 0
		if v, ok := params["limit"]; ok {
			if f, ok := v.(float64); ok {
				limit = int(f)
			}
		}
		result, err = a.db.ListWatchResults(w.ID, unnotifiedOnly, limit)

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

// userLocation returns the user's timezone from the "timezone" note,
// falling back to the server's local timezone.
func (a *Agent) userLocation() *time.Location {
	loc := time.Now().Location()
	if tz, err := a.db.GetNote("timezone"); err == nil && tz != "" {
		if parsed, err := time.LoadLocation(tz); err == nil {
			loc = parsed
		} else {
			log.Printf("invalid timezone note %q, using server local: %v", tz, err)
		}
	}
	return loc
}

// localToUTC parses a "YYYY-MM-DD HH:MM:SS" string as local time and converts to UTC.
func (a *Agent) localToUTC(fireAt string) (string, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", fireAt, a.userLocation())
	if err != nil {
		return "", fmt.Errorf("parsing fire_at %q: %w", fireAt, err)
	}
	return t.UTC().Format("2006-01-02 15:04:05"), nil
}
