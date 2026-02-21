package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// CreateCheckIn stores a check-in summary.
func (d *DB) CreateCheckIn(summary string) (int64, error) {
	res, err := d.conn.Exec("INSERT INTO check_ins (summary) VALUES (?)", summary)
	if err != nil {
		return 0, fmt.Errorf("creating check-in: %w", err)
	}
	return res.LastInsertId()
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
