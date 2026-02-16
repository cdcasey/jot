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
	case "list_todos":
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
		}
		status, _ := getString(params, "status")
		priority, _ := getString(params, "priority")
		result, err = a.db.ListTodos(projectID, status, priority)

	case "create_todo":
		title, _ := getString(params, "title")
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
		}
		notes, _ := getString(params, "notes")
		priority, _ := getString(params, "priority")
		dueDate, _ := getString(params, "due_date")
		id, e := a.db.CreateTodo(title, projectID, notes, priority, dueDate)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "update_todo":
		id, _ := getInt(params, "id")
		fields := make(map[string]any)
		for _, k := range []string{"title", "notes", "status", "priority", "due_date"} {
			if v, ok := params[k]; ok {
				fields[k] = v
			}
		}
		if v, ok := params["project_id"]; ok {
			fields["project_id"] = v
		}
		err = a.db.UpdateTodo(id, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "complete_todo":
		id, _ := getInt(params, "id")
		err = a.db.CompleteTodo(id)
		if err == nil {
			result = map[string]any{"status": "completed"}
		}

	case "list_projects":
		status, _ := getString(params, "status")
		result, err = a.db.ListProjects(status)

	case "create_project":
		name, _ := getString(params, "name")
		desc, _ := getString(params, "description")
		id, e := a.db.CreateProject(name, desc)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

	case "update_project":
		id, _ := getInt(params, "id")
		fields := make(map[string]any)
		for _, k := range []string{"name", "description", "status"} {
			if v, ok := params[k]; ok {
				fields[k] = v
			}
		}
		err = a.db.UpdateProject(id, fields)
		if err == nil {
			result = map[string]any{"status": "updated"}
		}

	case "get_summary":
		result, err = a.db.GetSummary()

	case "list_ideas":
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
		}
		tag, _ := getString(params, "tag")
		result, err = a.db.ListIdeas(projectID, tag)

	case "create_idea":
		title, _ := getString(params, "title")
		content, _ := getString(params, "content")
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
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
		id, e := a.db.CreateIdea(title, content, projectID, tags)
		if e != nil {
			err = e
		} else {
			result = map[string]any{"id": id, "status": "created"}
		}

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
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
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
		id, e := a.db.SaveMemory(content, category, "agent", tags, projectID, expiresAt)
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
		var projectID *int64
		if v, ok := getInt(params, "project_id"); ok {
			projectID = &v
		}
		result, err = a.db.SearchMemories(query, category, tag, projectID, since, int(limit))

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
