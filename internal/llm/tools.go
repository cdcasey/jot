package llm

var AgentTools = []Tool{
	{
		Name:        "list_things",
		Description: "List things, optionally filtered by status, priority, or tag. Items past their due date are marked overdue.",
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
		Name:        "list_schedules",
		Description: "List all schedules, including both recurring (cron) and one-shot reminders.",
		Parameters:  obj(nil),
	},
	{
		Name:        "create_schedule",
		Description: "Create a schedule. For recurring tasks, provide cron_expr. For one-shot reminders, provide fire_at instead (local time).",
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
	{
		Name:        "list_watches",
		Description: "List all web watches (URL monitors that extract info on a schedule).",
		Parameters:  obj(nil),
	},
	{
		Name:        "create_watch",
		Description: "Create a web watch that periodically fetches URLs and extracts information. The prompt tells the LLM what to extract from the page content.",
		Parameters: objReq(map[string]any{
			"name":      prop("string", "Unique name slug, e.g. 'austin-theatre-auditions'"),
			"prompt":    prop("string", "Extraction instructions, e.g. 'Extract theatre auditions with show name, company, dates, and requirements'"),
			"urls":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "URLs to fetch and extract from"},
			"cron_expr": prop("string", "Cron expression for how often to run, e.g. '0 9 * * 1' for Monday 9am. Omit for manual-only."),
		}, "name", "prompt", "urls"),
	},
	{
		Name:        "update_watch",
		Description: "Update a watch by name. Can change prompt, urls, cron_expr, or enabled.",
		Parameters: objReq(map[string]any{
			"name":      prop("string", "Watch name to update"),
			"prompt":    prop("string", "New extraction prompt"),
			"urls":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New list of URLs"},
			"cron_expr": prop("string", "New cron expression"),
			"enabled":   prop("boolean", "true to enable, false to disable"),
		}, "name"),
	},
	{
		Name:        "delete_watch",
		Description: "Delete a watch by name. Also removes all stored results for that watch.",
		Parameters: objReq(map[string]any{
			"name": prop("string", "Watch name to delete"),
		}, "name"),
	},
	{
		Name:        "run_watch",
		Description: "Manually trigger a watch to run now. Fetches URLs, extracts new items, and returns what was found. Use this to test a watch or get results on demand.",
		Parameters: objReq(map[string]any{
			"name": prop("string", "Watch name to run"),
		}, "name"),
	},
	{
		Name:        "list_watch_results",
		Description: "List stored results for a watch. Returns previously extracted items, optionally only unnotified ones.",
		Parameters: objReq(map[string]any{
			"name":            prop("string", "Watch name to list results for"),
			"unnotified_only": prop("boolean", "If true, only return results that haven't been delivered yet"),
			"limit":           prop("integer", "Max results to return (default 50)"),
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
