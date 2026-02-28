package db

type Thing struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Notes       string   `json:"notes,omitempty"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

type Memory struct {
	ID        int64    `json:"id"`
	Content   string   `json:"content"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags,omitempty"`
	ThingID   *int64   `json:"thing_id,omitempty"`
	Source    string   `json:"source"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type Summary struct {
	OpenThings    int     `json:"open_things"`
	OverdueThings []Thing `json:"overdue_things,omitempty"`
	RecentThings  []Thing `json:"recent_things,omitempty"`
}

type Skill struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type Schedule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CronExpr  string `json:"cron_expr"`
	Prompt    string `json:"prompt"`
	Enabled   bool   `json:"enabled"`
	LastRun   string `json:"last_run,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Reminder struct {
	ID        int64  `json:"id"`
	Prompt    string `json:"prompt"`
	FireAt    string `json:"fire_at"`
	Fired     bool   `json:"fired"`
	CreatedAt string `json:"created_at"`
}

type HabitLog struct {
	ID        int64  `json:"id"`
	Habit     string `json:"habit"`
	Outcome   string `json:"outcome"`
	Notes     string `json:"notes,omitempty"`
	LoggedAt  string `json:"logged_at"`
	CreatedAt string `json:"created_at"`
}

type HabitStats struct {
	Habit         string     `json:"habit"`
	Days          int        `json:"days"`
	DoneCount     int        `json:"done_count"`
	SkippedCount  int        `json:"skipped_count"`
	PartialCount  int        `json:"partial_count"`
	CurrentStreak int        `json:"current_streak"`
	LongestStreak int        `json:"longest_streak"`
	RecentLogs    []HabitLog `json:"recent_logs"`
}

type HabitSummary struct {
	Habit      string `json:"habit"`
	Last7Days  int    `json:"last_7_days"`
	LastLogged string `json:"last_logged"`
}
