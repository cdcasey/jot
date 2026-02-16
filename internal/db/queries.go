package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

// ListThings returns things, optionally filtered by status, priority, or tag.
func (d *DB) ListThings(status, priority, tag string) ([]Thing, error) {
	query := `SELECT id, title, COALESCE(notes,''), status, priority,
		COALESCE(tags,'[]'), COALESCE(due_date,''), created_at, updated_at,
		COALESCE(completed_at,'') FROM things WHERE 1=1`
	var args []any
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if priority != "" {
		query += " AND priority = ?"
		args = append(args, priority)
	}
	if tag != "" {
		query += " AND tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}
	query += " ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 WHEN 'low' THEN 3 END, updated_at DESC"
	return d.scanThings(query, args...)
}

// CreateThing creates a new thing and returns its ID.
func (d *DB) CreateThing(title, notes, priority, dueDate string, tags []string) (int64, error) {
	if priority == "" {
		priority = "normal"
	}
	var tagsJSON string
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}
	res, err := d.conn.Exec(
		"INSERT INTO things (title, notes, priority, due_date, tags) VALUES (?, ?, ?, ?, ?)",
		title, nullStr(notes), priority, nullStr(dueDate), nullStr(tagsJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("creating thing: %w", err)
	}
	return res.LastInsertId()
}

// UpdateThing updates fields on a thing by ID.
func (d *DB) UpdateThing(id int64, fields map[string]any) error {
	return d.updateRow("things", id, fields)
}

// CompleteThing marks a thing as done.
func (d *DB) CompleteThing(id int64) error {
	now := time.Now().UTC().Format(time.DateTime)
	_, err := d.conn.Exec(
		"UPDATE things SET status = 'done', completed_at = ?, updated_at = ? WHERE id = ?",
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("completing thing: %w", err)
	}
	return nil
}

// GetSummary returns a high-level summary of current state.
func (d *DB) GetSummary() (*Summary, error) {
	s := &Summary{}

	if err := d.conn.QueryRow("SELECT COUNT(*) FROM things WHERE status IN ('open','active')").Scan(&s.OpenThings); err != nil {
		return nil, fmt.Errorf("counting open things: %w", err)
	}

	// Overdue things
	now := time.Now().UTC().Format("2006-01-02")
	overdue, err := d.scanThings(
		`SELECT id, title, COALESCE(notes,''), status, priority,
			COALESCE(tags,'[]'), COALESCE(due_date,''), created_at, updated_at,
			COALESCE(completed_at,'') FROM things
			WHERE due_date < ? AND due_date != '' AND status NOT IN ('done','dropped')
			ORDER BY due_date`, now,
	)
	if err != nil {
		return nil, fmt.Errorf("querying overdue: %w", err)
	}
	s.OverdueThings = overdue

	// Recent things (last 5 created)
	recent, err := d.scanThings(
		`SELECT id, title, COALESCE(notes,''), status, priority,
			COALESCE(tags,'[]'), COALESCE(due_date,''), created_at, updated_at,
			COALESCE(completed_at,'') FROM things
			ORDER BY created_at DESC LIMIT 5`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying recent: %w", err)
	}
	s.RecentThings = recent

	return s, nil
}

func (d *DB) scanThings(query string, args ...any) ([]Thing, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying things: %w", err)
	}
	defer rows.Close()
	var things []Thing
	for rows.Next() {
		var t Thing
		var tagsJSON string
		if err := rows.Scan(&t.ID, &t.Title, &t.Notes, &t.Status, &t.Priority, &tagsJSON, &t.DueDate, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scanning thing: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &t.Tags)
		things = append(things, t)
	}
	return things, rows.Err()
}

// GetNote retrieves a note by key.
func (d *DB) GetNote(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM notes WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting note: %w", err)
	}
	return value, nil
}

// SetNote stores or updates a note by key.
func (d *DB) SetNote(key, value string) error {
	_, err := d.conn.Exec(
		"INSERT INTO notes (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')",
		key, value, value,
	)
	if err != nil {
		return fmt.Errorf("setting note: %w", err)
	}
	return nil
}

// CreateCheckIn stores a check-in summary.
func (d *DB) CreateCheckIn(summary string) (int64, error) {
	res, err := d.conn.Exec("INSERT INTO check_ins (summary) VALUES (?)", summary)
	if err != nil {
		return 0, fmt.Errorf("creating check-in: %w", err)
	}
	return res.LastInsertId()
}

// SaveMemory stores a new memory and returns its ID.
func (d *DB) SaveMemory(content, category, source string, tags []string, thingID *int64, expiresAt string) (int64, error) {
	var tagsJSON string
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}
	res, err := d.conn.Exec(
		"INSERT INTO memories (content, category, source, tags, thing_id, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		content, category, source, nullStr(tagsJSON), thingID, nullStr(expiresAt),
	)
	if err != nil {
		return 0, fmt.Errorf("saving memory: %w", err)
	}
	return res.LastInsertId()
}

// SearchMemories searches memories by text query, category, tag, thing, and date.
func (d *DB) SearchMemories(query, category, tag string, thingID *int64, since string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 10
	}
	q := "SELECT id, content, category, COALESCE(tags,'[]'), thing_id, source, COALESCE(expires_at,''), created_at FROM memories WHERE (expires_at IS NULL OR expires_at > datetime('now'))"
	var args []any
	if query != "" {
		q += " AND content LIKE ?"
		args = append(args, "%"+query+"%")
	}
	if category != "" {
		q += " AND category = ?"
		args = append(args, category)
	}
	if tag != "" {
		q += " AND tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}
	if thingID != nil {
		q += " AND thing_id = ?"
		args = append(args, *thingID)
	}
	if since != "" {
		q += " AND created_at >= ?"
		args = append(args, since)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	return d.scanMemories(q, args...)
}

// ListRecentMemories returns the most recent memories, optionally filtered by category.
func (d *DB) ListRecentMemories(category string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 10
	}
	q := "SELECT id, content, category, COALESCE(tags,'[]'), thing_id, source, COALESCE(expires_at,''), created_at FROM memories WHERE (expires_at IS NULL OR expires_at > datetime('now'))"
	var args []any
	if category != "" {
		q += " AND category = ?"
		args = append(args, category)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	return d.scanMemories(q, args...)
}

// GetRecentMemoriesForCheckIn returns memories from the last N days, prioritizing blockers and decisions.
func (d *DB) GetRecentMemoriesForCheckIn(days int) ([]Memory, error) {
	q := `SELECT id, content, category, COALESCE(tags,'[]'), thing_id, source, COALESCE(expires_at,''), created_at
		FROM memories
		WHERE created_at > datetime('now', '-' || ? || ' days')
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY
		  CASE category WHEN 'blocker' THEN 0 WHEN 'decision' THEN 1 WHEN 'event' THEN 2 ELSE 3 END,
		  created_at DESC
		LIMIT 20`
	return d.scanMemories(q, days)
}

// GetLastCheckIn returns the most recent check-in summary and date.
func (d *DB) GetLastCheckIn() (string, string, error) {
	var summary, createdAt string
	err := d.conn.QueryRow("SELECT summary, created_at FROM check_ins ORDER BY created_at DESC LIMIT 1").Scan(&summary, &createdAt)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("getting last check-in: %w", err)
	}
	return summary, createdAt, nil
}

// PruneExpiredMemories deletes memories past their expiry.
func (d *DB) PruneExpiredMemories() (int64, error) {
	res, err := d.conn.Exec("DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at < datetime('now')")
	if err != nil {
		return 0, fmt.Errorf("pruning memories: %w", err)
	}
	return res.RowsAffected()
}

func (d *DB) scanMemories(query string, args ...any) ([]Memory, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying memories: %w", err)
	}
	defer rows.Close()
	var memories []Memory
	for rows.Next() {
		var m Memory
		var tagsJSON string
		if err := rows.Scan(&m.ID, &m.Content, &m.Category, &tagsJSON, &m.ThingID, &m.Source, &m.ExpiresAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

var allowedColumns = map[string]map[string]bool{
	"things": {"title": true, "notes": true, "status": true, "priority": true, "due_date": true, "tags": true, "completed_at": true},
	"skills": {"name": true, "description": true, "content": true, "tags": true},
}

// updateRow is a generic helper for updating a row's fields.
func (d *DB) updateRow(table string, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	allowed, ok := allowedColumns[table]
	if !ok {
		return fmt.Errorf("unknown table: %s", table)
	}
	var setClauses []string
	var args []any
	for col, val := range fields {
		if !allowed[col] {
			return fmt.Errorf("disallowed column %q for table %s", col, table)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(setClauses, ", "))
	_, err := d.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updating %s %d: %w", table, id, err)
	}
	return nil
}

func nullStr(s string) any {
	if s == "" || s == "null" {
		return nil
	}
	return s
}

// CreateSkill creates a new skill and returns its ID.
func (d *DB) CreateSkill(name, description, content string, tags []string) (int64, error) {
	var tagsJSON string
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}
	res, err := d.conn.Exec(
		"INSERT INTO skills (name, description, content, tags) VALUES (?, ?, ?, ?)",
		name, description, content, nullStr(tagsJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("creating skill: %w", err)
	}
	return res.LastInsertId()
}

// GetSkill retrieves a skill by name.
func (d *DB) GetSkill(name string) (*Skill, error) {
	var s Skill
	var tagsJSON string
	err := d.conn.QueryRow("SELECT id, name, description, content, COALESCE(tags,'[]'), created_at, updated_at FROM skills WHERE name = ?", name).Scan(&s.ID, &s.Name, &s.Description, &s.Content, &tagsJSON, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting skill: %w", err)
	}
	_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
	return &s, nil
}

// ListSkills returns skills, optionally filtered by tags.
func (d *DB) ListSkills(tag string) ([]Skill, error) {
	query := "SELECT id, name, description, content, COALESCE(tags,'[]'), created_at, updated_at FROM skills WHERE 1=1"
	var args []any
	if tag != "" {
		query += " AND tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}
	query += " ORDER BY created_at DESC"
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	defer rows.Close()
	var skills []Skill
	for rows.Next() {
		var s Skill
		var tagsJSON string
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Content, &tagsJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning skill: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// UpdateSkill updates fields on a skill by name (names are unique in the skills schema).
func (d *DB) UpdateSkill(name string, fields map[string]any) error {
	skill, err := d.GetSkill(name)
	if err != nil {
		return fmt.Errorf("fetching skill for update: %w", err)
	}
	if skill == nil {
		return fmt.Errorf("no such skill exists: %s", name)
	}
	return d.updateRow("skills", skill.ID, fields)
}

// DeleteSkill deletes a skill by name
func (d *DB) DeleteSkill(name string) error {
	skill, err := d.GetSkill(name)
	if err != nil {
		return fmt.Errorf("fetching skill for delete: %w", err)
	}
	if skill == nil {
		return fmt.Errorf("no such skill exists: %s", name)
	}
	_, err = d.conn.Exec("DELETE from skills where id = ?", skill.ID)
	if err != nil {
		return fmt.Errorf("deleting skill: %w", err)
	}
	return nil
}
