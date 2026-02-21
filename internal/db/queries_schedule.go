package db

import (
	"fmt"
	"strings"
)

// ListSchedules returns all schedules, optionally only enabled ones.
func (d *DB) ListSchedules(enabledOnly bool) ([]Schedule, error) {
	q := "SELECT id, name, cron_expr, prompt, enabled, COALESCE(last_run,''), created_at FROM schedules"
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
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.Prompt, &enabled, &s.LastRun, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning schedule: %w", err)
		}
		s.Enabled = enabled == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateSchedule creates a new schedule and returns its ID.
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
