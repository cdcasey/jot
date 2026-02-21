package db

import (
	"encoding/json"
	"fmt"
	"time"
)

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
