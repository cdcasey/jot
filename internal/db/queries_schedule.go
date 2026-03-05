package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ListSchedules returns all schedules, optionally only enabled ones.
func (d *DB) ListSchedules(enabledOnly bool) ([]Schedule, error) {
	q := "SELECT id, name, cron_expr, prompt, enabled, COALESCE(last_run,''), COALESCE(fire_at,''), fired, created_at FROM schedules"
	if enabledOnly {
		q += " WHERE enabled = 1"
	}
	q += " ORDER BY created_at ASC"
	rows, err := d.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("listing schedules: %w", err)
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var s Schedule
		var enabled, fired int
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.Prompt, &enabled, &s.LastRun, &s.FireAt, &fired, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning schedule: %w", err)
		}
		s.Enabled = enabled == 1
		s.Fired = fired == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateSchedule creates a new recurring schedule and returns its ID.
func (d *DB) CreateSchedule(name, cronExpr, prompt string) (int64, error) {
	res, err := d.conn.Exec(
		"INSERT INTO schedules (name, cron_expr, prompt) VALUES (?, ?, ?)",
		name, cronExpr, prompt,
	)
	if err != nil {
		return 0, fmt.Errorf("creating schedule: %w", err)
	}
	return res.LastInsertId()
}

// CreateOneShot creates a one-shot schedule (reminder) that fires at a specific time.
func (d *DB) CreateOneShot(name, prompt, fireAt string) (int64, error) {
	res, err := d.conn.Exec(
		"INSERT INTO schedules (name, cron_expr, prompt, fire_at) VALUES (?, '', ?, ?)",
		name, prompt, fireAt,
	)
	if err != nil {
		return 0, fmt.Errorf("creating one-shot schedule: %w", err)
	}
	return res.LastInsertId()
}

// ListPendingOneShots returns one-shot schedules that are due and not yet fired.
func (d *DB) ListPendingOneShots() ([]Schedule, error) {
	q := `SELECT id, name, cron_expr, prompt, enabled, COALESCE(last_run,''), COALESCE(fire_at,''), fired, created_at
		FROM schedules WHERE fire_at IS NOT NULL AND fire_at <= datetime('now') AND fired = 0`
	return d.scanSchedules(q)
}

// ListUpcomingOneShots returns one-shot schedules that haven't fired yet and are in the future.
func (d *DB) ListUpcomingOneShots() ([]Schedule, error) {
	q := `SELECT id, name, cron_expr, prompt, enabled, COALESCE(last_run,''), COALESCE(fire_at,''), fired, created_at
		FROM schedules WHERE fire_at IS NOT NULL AND fire_at > datetime('now') AND fired = 0
		ORDER BY fire_at ASC`
	return d.scanSchedules(q)
}

// MarkOneShotFired marks a one-shot schedule as fired.
func (d *DB) MarkOneShotFired(id int64) error {
	res, err := d.conn.Exec("UPDATE schedules SET fired = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("marking one-shot fired: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("schedule %d not found", id)
	}
	return nil
}

// UpdateSchedule updates fields on a schedule by ID.
func (d *DB) UpdateSchedule(id int64, fields map[string]any) error {
	allowed := map[string]bool{"cron_expr": true, "prompt": true, "enabled": true}
	if len(fields) == 0 {
		return nil
	}
	var setClauses []string
	var args []any
	for col, val := range fields {
		if !allowed[col] {
			return fmt.Errorf("disallowed column %q for schedules", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE schedules SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := d.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updating schedule %d: %w", id, err)
	}
	return nil
}

// DeleteSchedule deletes a schedule by name.
func (d *DB) DeleteSchedule(name string) error {
	_, err := d.conn.Exec("DELETE FROM schedules WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("deleting schedule: %w", err)
	}
	return nil
}

// RecordScheduleRun updates last_run to now for a schedule.
func (d *DB) RecordScheduleRun(id int64) error {
	_, err := d.conn.Exec(
		"UPDATE schedules SET last_run = datetime('now') WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("recording schedule run: %w", err)
	}
	return nil
}

func (d *DB) scanSchedules(query string, args ...any) ([]Schedule, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying schedules: %w", err)
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var s Schedule
		var enabled, fired int
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.Prompt, &enabled, &s.LastRun, &s.FireAt, &fired, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning schedule: %w", err)
		}
		s.Enabled = enabled == 1
		s.Fired = fired == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetScheduleByName returns a schedule by name, or nil if not found.
func (d *DB) GetScheduleByName(name string) (*Schedule, error) {
	q := `SELECT id, name, cron_expr, prompt, enabled, COALESCE(last_run,''), COALESCE(fire_at,''), fired, created_at
		FROM schedules WHERE name = ?`
	var s Schedule
	var enabled, fired int
	err := d.conn.QueryRow(q, name).Scan(&s.ID, &s.Name, &s.CronExpr, &s.Prompt, &enabled, &s.LastRun, &s.FireAt, &fired, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting schedule %q: %w", name, err)
	}
	s.Enabled = enabled == 1
	s.Fired = fired == 1
	return &s, nil
}
