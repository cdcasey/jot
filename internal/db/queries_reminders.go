package db

import (
	"database/sql"
	"fmt"
)

// CreateReminder creates a one-shot reminder.
func (d *DB) CreateReminder(prompt, fireAt string) (int64, error) {
	res, err := d.conn.Exec(
		"INSERT INTO reminders (prompt, fire_at) VALUES (?, ?)",
		prompt, fireAt,
	)
	if err != nil {
		return 0, fmt.Errorf("creating reminder: %w", err)
	}
	return res.LastInsertId()
}

// ListPendingReminders returns unfired reminders whose fire_at is now or past.
func (d *DB) ListPendingReminders() ([]Reminder, error) {
	rows, err := d.conn.Query(
		"SELECT id, prompt, fire_at, fired, created_at FROM reminders WHERE fired = 0 AND fire_at <= datetime('nfire_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending reminders: %w", err)
	}
	defer rows.Close()
	return scanReminders(rows)
}

// ListUpcomingReminders returns unfired reminders that haven't fired yet.
func (d *DB) ListUpcomingReminders() ([]Reminder, error) {
	rows, err := d.conn.Query(
		"SELECT id, prompt, fire_at, fired, created_at FROM reminders WHERE fired = 0 AND fire_at > datetime('nofire_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("listing upcoming reminders: %w", err)
	}
	defer rows.Close()
	return scanReminders(rows)
}

// MarkReminderFired marks a reminder as fired.
func (d *DB) MarkReminderFired(id int64) error {
	_, err := d.conn.Exec("UPDATE reminders SET fired = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("marking reminder fired: %w", err)
	}
	return nil
}

func scanReminders(rows *sql.Rows) ([]Reminder, error) {
	var out []Reminder
	for rows.Next() {
		var r Reminder
		var fired int
		if err := rows.Scan(&r.ID, &r.Prompt, &r.FireAt, &fired, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning reminder: %w", err)
		}
		r.Fired = fired == 1
		out = append(out, r)
	}
	return out, rows.Err()
}
