package llm

var AgentTools = []Tool{
	{
		Name:        "list_things",
		Description: "List things, optionally filtered by status, priority, or tag.",
		Parameters: obj(map[string]any{
			"status":   prop("string", "Filter by status: open, active, done, dropped"),
			"priority": prop("string", "Filter by priority: low, normal, high, urgent"),
			"tag":      prop("string", "Filter by tag"),
		}),
	},
	{
		Name:        "create_thing",
		Description: "Create a new thing to track.",
		Parameters: objReq(map[string]any{
			"title":    prop("string", "What the thing is"),
			"notes":    prop("string", "Additional details or context"),
			"priority": prop("string", "Priority: low, normal, high, urgent"),
			"due_date": prop("string", "Due date in YYYY-MM-DD format"),
			"tags":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tags for categorization"},
		}, "title"),
	},
	{
		Name:        "update_thing",
		Description: "Update a thing by ID. Can change title, notes, status, priority, due_date, or tags.",
		Parameters: objReq(map[string]any{
			"id":       prop("integer", "Thing ID"),
			"title":    prop("string", "New title"),
			"notes":    prop("string", "New notes"),
			"status":   prop("string", "New status: open, active, done, dropped"),
			"priority": prop("string", "New priority: low, normal, high, urgent"),
			"due_date": prop("string", "New due date in YYYY-MM-DD format"),
			"tags":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tags"},
		}, "id"),
	},
	{
		Name:        "complete_thing",
		Description: "Mark a thing as done.",
		Parameters: objReq(map[string]any{
			"id": prop("integer", "Thing ID to complete"),
		}, "id"),
	},
	{
		Name:        "get_summary",
		Description: "Get a summary of open things, overdue items, and recent activity.",
		Parameters:  obj(nil),
	},
	{
		Name:        "save_memory",
		Description: "Save a memory for future reference. Use this to remember important context, decisions, blockers, user preferences, or events. Be specific and include temporal context (e.g. 'as of Feb 2026'). Choose the right category. Use category 'habit' to log recurring activity entries like 'gym: done' or 'meditation: skipped'.",
		Parameters: objReq(map[string]any{
			"content":    prop("string", "What to remember. Write a clear, specific sentence."),
			"category":   prop("string", "One of: observation, decision, blocker, preference, event, reflection, habit"),
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Freeform tags for retrieval"},
			"thing_id":   prop("integer", "Optional thing ID to link to"),
			"expires_at": prop("string", "Optional expiry datetime (YYYY-MM-DD HH:MM:SS). Omit for permanent memories."),
		}, "content", "category"),
	},
	{
		Name:        "search_memories",
		Description: "Search past memories by text, category, tag, or thing. Returns matches ordered by recency. Use this to recall context before answering questions.",
		Parameters: obj(map[string]any{
			"query":    prop("string", "Text to search for in memory content"),
			"category": prop("string", "Filter by category: observation, decision, blocker, preference, event, reflection, habit"),
			"tag":      prop("string", "Filter by tag"),
			"thing_id": prop("integer", "Filter by thing ID"),
			"since":    prop("string", "Only memories after this date (YYYY-MM-DD)"),
			"limit":    prop("integer", "Max results (default 10)"),
		}),
	},
	{
		Name:        "list_recent_memories",
		Description: "List the most recent memories, optionally filtered by category. Use at conversation start or check-ins to re-establish context.",
		Parameters: obj(map[string]any{
			"category": prop("string", "Filter by category: observation, decision, blocker, preference, event, reflection, habit"),
			"limit":    prop("integer", "Max results (default 10)"),
		}),
	},
	{
		Name:        "update_memory",
		Description: "Update a memory by ID. Can change content, category, tags, or expires_at. Use this to correct or enrich existing memories.",
		Parameters: objReq(map[string]any{
			"id":         prop("integer", "Memory ID to update"),
			"content":    prop("string", "New content text"),
			"category":   prop("string", "New category: observation, decision, blocker, preference, event, reflection, habit"),
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tags"},
			"expires_at": prop("string", "New expiry datetime (YYYY-MM-DD HH:MM:SS), or empty string to make permanent"),
		}, "id"),
	},
	{
		Name:        "delete_memory",
		Description: "Delete a memory by ID. Use when a memory is no longer relevant or was created in error.",
		Parameters: objReq(map[string]any{
			"id": prop("integer", "Memory ID to delete"),
		}, "id"),
	},
	{
		Name:        "get_time",
		Description: "Get the current system time. Use this when doing things like setting up cron jobs for check-ins and reminders.",
		Parameters:  obj(nil),
	},
	{
		Name:        "list_schedules",
		Description: "List all schedules, including both recurring (cron) and one-shot reminders.",
		Parameters:  obj(nil),
	},
	{
		Name:        "create_schedule",
		Description: "Create a schedule. For recurring tasks, provide cron_expr. For one-shot reminders, provide fire_at instead. Always call get_time first.",
		Parameters: objReq(map[string]any{
			"name":      prop("string", "Unique name slug, e.g. 'weekly-review' or 'reminder-call-dentist'"),
			"cron_expr": prop("string", "Cron expression for recurring schedules, e.g. '0 9 * * *'. Omit for one-shot reminders."),
			"prompt":    prop("string", "What to tell the agent when this schedule fires"),
			"fire_at":   prop("string", "Local datetime for one-shot reminders: 'YYYY-MM-DD HH:MM:SS'. Omit for recurring schedules."),
		}, "name", "prompt"),
	},
	{
		Name:        "update_schedule",
		Description: "Update a schedule by name. Can change cron_expr, prompt, or enabled.",
		Parameters: objReq(map[string]any{
			"name":      prop("string", "Schedule name to update"),
			"cron_expr": prop("string", "New cron expression"),
			"prompt":    prop("string", "New prompt"),
			"enabled":   prop("boolean", "true to enable, false to disable"),
		}, "name"),
	},
	{
		Name:        "delete_schedule",
		Description: "Delete a schedule by name.",
		Parameters: objReq(map[string]any{
			"name": prop("string", "Schedule name to delete"),
		}, "name"),
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
