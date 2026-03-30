package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// CreateWatch creates a new watch and returns its ID.
func (d *DB) CreateWatch(name, prompt string, urls []string, cronExpr string) (int64, error) {
	urlsJSON := "[]"
	if len(urls) > 0 {
		b, _ := json.Marshal(urls)
		urlsJSON = string(b)
	}
	res, err := d.conn.Exec(
		"INSERT INTO watches (name, prompt, urls, cron_expr) VALUES (?, ?, ?, ?)",
		name, prompt, urlsJSON, cronExpr,
	)
	if err != nil {
		return 0, fmt.Errorf("creating watch: %w", err)
	}
	return res.LastInsertId()
}

// ListWatches returns all watches, optionally only enabled ones.
func (d *DB) ListWatches(enabledOnly bool) ([]Watch, error) {
	q := `SELECT id, name, prompt, urls, cron_expr, enabled, COALESCE(last_run,''), created_at, COALESCE(updated_at,'') FROM watches`
	if enabledOnly {
		q += " WHERE enabled = 1"
	}
	q += " ORDER BY created_at ASC"
	return d.scanWatches(q)
}

// GetWatchByName returns a watch by name, or nil if not found.
func (d *DB) GetWatchByName(name string) (*Watch, error) {
	q := `SELECT id, name, prompt, urls, cron_expr, enabled, COALESCE(last_run,''), created_at, COALESCE(updated_at,'')
		FROM watches WHERE name = ?`
	rows, err := d.conn.Query(q, name)
	if err != nil {
		return nil, fmt.Errorf("getting watch %q: %w", name, err)
	}
	defer rows.Close()
	watches, err := d.scanWatchRows(rows)
	if err != nil {
		return nil, err
	}
	if len(watches) == 0 {
		return nil, nil
	}
	return &watches[0], nil
}

// UpdateWatch updates fields on a watch by ID.
func (d *DB) UpdateWatch(id int64, fields map[string]any) error {
	return d.updateRow("watches", id, fields)
}

// DeleteWatch deletes a watch by name. Cascades to watch_results.
func (d *DB) DeleteWatch(name string) error {
	_, err := d.conn.Exec("DELETE FROM watches WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("deleting watch: %w", err)
	}
	return nil
}

// RecordWatchRun updates last_run to now for a watch.
func (d *DB) RecordWatchRun(id int64) error {
	_, err := d.conn.Exec(
		"UPDATE watches SET last_run = datetime('now') WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("recording watch run: %w", err)
	}
	return nil
}

// SaveWatchResult inserts a new result, ignoring duplicates (same watch + hash).
// Returns the new row ID, or 0 if it was a duplicate.
func (d *DB) SaveWatchResult(watchID int64, contentHash, title, body, sourceURL string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT OR IGNORE INTO watch_results (watch_id, content_hash, title, body, source_url)
		 VALUES (?, ?, ?, ?, ?)`,
		watchID, contentHash, title, nullStr(body), nullStr(sourceURL),
	)
	if err != nil {
		return 0, fmt.Errorf("saving watch result: %w", err)
	}
	id, _ := res.LastInsertId()
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, nil // duplicate — already existed
	}
	return id, nil
}

// ListWatchResults returns results for a watch, optionally only unnotified ones.
func (d *DB) ListWatchResults(watchID int64, unnotifiedOnly bool, limit int) ([]WatchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id, watch_id, content_hash, title, COALESCE(body,''), COALESCE(source_url,''), first_seen, notified
		FROM watch_results WHERE watch_id = ?`
	args := []any{watchID}
	if unnotifiedOnly {
		q += " AND notified = 0"
	}
	q += " ORDER BY first_seen DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.conn.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing watch results: %w", err)
	}
	defer rows.Close()

	var out []WatchResult
	for rows.Next() {
		var r WatchResult
		var notified int
		if err := rows.Scan(&r.ID, &r.WatchID, &r.ContentHash, &r.Title, &r.Body, &r.SourceURL, &r.FirstSeen, &notified); err != nil {
			return nil, fmt.Errorf("scanning watch result: %w", err)
		}
		r.Notified = notified == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkResultsNotified marks the given result IDs as notified.
func (d *DB) MarkResultsNotified(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf("UPDATE watch_results SET notified = 1 WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := d.conn.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("marking results notified: %w", err)
	}
	return nil
}

// PruneOldWatchResults deletes watch results older than the given number of days.
func (d *DB) PruneOldWatchResults(olderThanDays int) (int64, error) {
	res, err := d.conn.Exec(
		"DELETE FROM watch_results WHERE first_seen < datetime('now', ?)",
		fmt.Sprintf("-%d days", olderThanDays),
	)
	if err != nil {
		return 0, fmt.Errorf("pruning old watch results: %w", err)
	}
	return res.RowsAffected()
}

// --- internal helpers ---

func (d *DB) scanWatches(query string, args ...any) ([]Watch, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying watches: %w", err)
	}
	defer rows.Close()
	return d.scanWatchRows(rows)
}

func (d *DB) scanWatchRows(rows *sql.Rows) ([]Watch, error) {
	var out []Watch
	for rows.Next() {
		var w Watch
		var urlsRaw string
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Prompt, &urlsRaw, &w.CronExpr, &enabled, &w.LastRun, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning watch: %w", err)
		}
		w.Enabled = enabled == 1
		if urlsRaw != "" {
			_ = json.Unmarshal([]byte(urlsRaw), &w.URLs)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
