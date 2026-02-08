package llm

var AgentTools = []Tool{
	{
		Name:        "list_todos",
		Description: "List todos, optionally filtered by project_id, status, or priority.",
		Parameters: obj(map[string]any{
			"project_id": prop("integer", "Filter by project ID"),
			"status":     prop("string", "Filter by status: pending, in_progress, done, cancelled"),
			"priority":   prop("string", "Filter by priority: low, normal, high, urgent"),
		}),
	},
	{
		Name:        "create_todo",
		Description: "Create a new todo item.",
		Parameters: objReq(map[string]any{
			"title":      prop("string", "Title of the todo"),
			"project_id": prop("integer", "Optional project ID to associate with"),
			"notes":      prop("string", "Additional notes"),
			"priority":   prop("string", "Priority: low, normal, high, urgent"),
			"due_date":   prop("string", "Due date in YYYY-MM-DD format"),
		}, "title"),
	},
	{
		Name:        "update_todo",
		Description: "Update a todo by ID. Can change title, notes, status, priority, due_date, or project_id.",
		Parameters: objReq(map[string]any{
			"id":         prop("integer", "Todo ID"),
			"title":      prop("string", "New title"),
			"notes":      prop("string", "New notes"),
			"status":     prop("string", "New status: pending, in_progress, done, cancelled"),
			"priority":   prop("string", "New priority: low, normal, high, urgent"),
			"due_date":   prop("string", "New due date in YYYY-MM-DD format"),
			"project_id": prop("integer", "New project ID"),
		}, "id"),
	},
	{
		Name:        "complete_todo",
		Description: "Mark a todo as done.",
		Parameters: objReq(map[string]any{
			"id": prop("integer", "Todo ID to complete"),
		}, "id"),
	},
	{
		Name:        "list_projects",
		Description: "List all projects, optionally filtered by status.",
		Parameters: obj(map[string]any{
			"status": prop("string", "Filter by status: active, paused, completed, archived"),
		}),
	},
	{
		Name:        "create_project",
		Description: "Create a new project.",
		Parameters: objReq(map[string]any{
			"name":        prop("string", "Project name"),
			"description": prop("string", "Project description"),
		}, "name"),
	},
	{
		Name:        "update_project",
		Description: "Update a project by ID. Can change name, description, or status.",
		Parameters: objReq(map[string]any{
			"id":          prop("integer", "Project ID"),
			"name":        prop("string", "New name"),
			"description": prop("string", "New description"),
			"status":      prop("string", "New status: active, paused, completed, archived"),
		}, "id"),
	},
	{
		Name:        "get_summary",
		Description: "Get a summary of active projects, pending todos, overdue items, and recent activity.",
		Parameters: obj(nil),
	},
	{
		Name:        "list_ideas",
		Description: "List ideas, optionally filtered by project_id or tag.",
		Parameters: obj(map[string]any{
			"project_id": prop("integer", "Filter by project ID"),
			"tag":        prop("string", "Filter by tag"),
		}),
	},
	{
		Name:        "create_idea",
		Description: "Capture a new idea.",
		Parameters: objReq(map[string]any{
			"title":      prop("string", "Idea title"),
			"content":    prop("string", "Idea details"),
			"project_id": prop("integer", "Optional project ID"),
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tags for the idea"},
		}, "title"),
	},
	{
		Name:        "save_memory",
		Description: "Save a memory for future reference. Use this to remember important context, decisions, blockers, user preferences, or events. Be specific and include temporal context (e.g. 'as of Feb 2026'). Choose the right category.",
		Parameters: objReq(map[string]any{
			"content":    prop("string", "What to remember. Write a clear, specific sentence."),
			"category":   prop("string", "One of: observation, decision, blocker, preference, event, reflection"),
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Freeform tags for retrieval"},
			"project_id": prop("integer", "Optional project ID to link to"),
			"expires_at": prop("string", "Optional expiry datetime (YYYY-MM-DD HH:MM:SS). Omit for permanent memories."),
		}, "content", "category"),
	},
	{
		Name:        "search_memories",
		Description: "Search past memories by text, category, tag, or project. Returns matches ordered by recency. Use this to recall context before answering questions.",
		Parameters: obj(map[string]any{
			"query":      prop("string", "Text to search for in memory content"),
			"category":   prop("string", "Filter by category: observation, decision, blocker, preference, event, reflection"),
			"tag":        prop("string", "Filter by tag"),
			"project_id": prop("integer", "Filter by project ID"),
			"since":      prop("string", "Only memories after this date (YYYY-MM-DD)"),
			"limit":      prop("integer", "Max results (default 10)"),
		}),
	},
	{
		Name:        "list_recent_memories",
		Description: "List the most recent memories, optionally filtered by category. Use at conversation start or check-ins to re-establish context.",
		Parameters: obj(map[string]any{
			"category": prop("string", "Filter by category: observation, decision, blocker, preference, event, reflection"),
			"limit":    prop("integer", "Max results (default 10)"),
		}),
	},
	{
		Name:        "get_note",
		Description: "Retrieve a stored note by key. Use this for persistent memory.",
		Parameters: objReq(map[string]any{
			"key": prop("string", "Note key"),
		}, "key"),
	},
	{
		Name:        "set_note",
		Description: "Store or update a note by key. Use this as a scratchpad for persistent memory.",
		Parameters: objReq(map[string]any{
			"key":   prop("string", "Note key"),
			"value": prop("string", "Note value"),
		}, "key", "value"),
	},
}

// Helper functions for building JSON Schema objects.

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func obj(properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func objReq(properties map[string]any, required ...string) map[string]any {
	s := obj(properties)
	s["required"] = required
	return s
}
