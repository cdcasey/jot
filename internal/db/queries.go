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

type Schedule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CronExpr  string `json:"cron_expr,omitempty"`
	Prompt    string `json:"prompt"`
	Enabled   bool   `json:"enabled"`
	LastRun   string `json:"last_run,omitempty"`
	FireAt    string `json:"fire_at,omitempty"`
	Fired     bool   `json:"fired,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Watch struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Prompt    string   `json:"prompt"`
	URLs      []string `json:"urls"`
	CronExpr  string   `json:"cron_expr,omitempty"`
	Enabled   bool     `json:"enabled"`
	LastRun   string   `json:"last_run,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type WatchResult struct {
	ID          int64  `json:"id"`
	WatchID     int64  `json:"watch_id"`
	ContentHash string `json:"content_hash"`
	Title       string `json:"title"`
	Body        string `json:"body,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	FirstSeen   string `json:"first_seen"`
	Notified    bool   `json:"notified"`
}
