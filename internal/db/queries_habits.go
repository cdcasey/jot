package db

import (
	"database/sql"
	"fmt"
)

// GetOrCreateHabit returns the habit with the given name, creating it
// (active=1) if it does not yet exist. The second return value is true when a
// new habit was created, so callers can surface that to the user.
func (d *DB) GetOrCreateHabit(name string) (*Habit, bool, error) {
	h, err := d.getHabitByName(name)
	if err != nil {
		return nil, false, err
	}
	if h != nil {
		return h, false, nil
	}
	res, err := d.conn.Exec("INSERT INTO habits (name) VALUES (?)", name)
	if err != nil {
		return nil, false, fmt.Errorf("creating habit %q: %w", name, err)
	}
	id, _ := res.LastInsertId()
	h, err = d.getHabitByID(id)
	if err != nil {
		return nil, false, err
	}
	return h, true, nil
}

// LogHabit records that a habit was done on a local calendar date. Re-logging
// the same (habit, date) is a harmless no-op thanks to the unique constraint.
func (d *DB) LogHabit(habitID int64, date string) error {
	_, err := d.conn.Exec(
		`INSERT INTO habit_logs (habit_id, date) VALUES (?, ?)
		 ON CONFLICT(habit_id, date) DO NOTHING`,
		habitID, date,
	)
	if err != nil {
		return fmt.Errorf("logging habit %d on %s: %w", habitID, date, err)
	}
	return nil
}

// UnlogHabit removes a habit's entry for a date, leaving the habit definition
// untouched. Deleting an absent entry is a no-op.
func (d *DB) UnlogHabit(habitID int64, date string) error {
	_, err := d.conn.Exec(
		"DELETE FROM habit_logs WHERE habit_id = ? AND date = ?",
		habitID, date,
	)
	if err != nil {
		return fmt.Errorf("unlogging habit %d on %s: %w", habitID, date, err)
	}
	return nil
}

// RenameHabit changes a habit's name. Log rows reference habit_id and are
// unaffected. (habits has no updated_at column, so updateRow is not used.)
func (d *DB) RenameHabit(habitID int64, newName string) error {
	res, err := d.conn.Exec("UPDATE habits SET name = ? WHERE id = ?", newName, habitID)
	if err != nil {
		return fmt.Errorf("renaming habit %d: %w", habitID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("habit %d not found", habitID)
	}
	return nil
}

// HabitLogsInRange returns a habit's log entries between start and end
// (inclusive) ordered by date. This is the substrate for a future visual grid;
// it is not exposed as an LLM tool. Dates without an entry are simply absent.
func (d *DB) HabitLogsInRange(habitID int64, start, end string) ([]HabitLog, error) {
	rows, err := d.conn.Query(
		`SELECT id, habit_id, date, created_at FROM habit_logs
		 WHERE habit_id = ? AND date >= ? AND date <= ?
		 ORDER BY date ASC`,
		habitID, start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("listing habit logs: %w", err)
	}
	defer rows.Close()

	var out []HabitLog
	for rows.Next() {
		var l HabitLog
		if err := rows.Scan(&l.ID, &l.HabitID, &l.Date, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning habit log: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// --- internal helpers ---

func (d *DB) getHabitByName(name string) (*Habit, error) {
	return scanHabitRow(d.conn.QueryRow(
		"SELECT id, name, active, created_at FROM habits WHERE name = ?", name,
	))
}

func (d *DB) getHabitByID(id int64) (*Habit, error) {
	return scanHabitRow(d.conn.QueryRow(
		"SELECT id, name, active, created_at FROM habits WHERE id = ?", id,
	))
}

func scanHabitRow(row *sql.Row) (*Habit, error) {
	var h Habit
	var active int
	switch err := row.Scan(&h.ID, &h.Name, &active, &h.CreatedAt); err {
	case nil:
		h.Active = active == 1
		return &h, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, fmt.Errorf("scanning habit: %w", err)
	}
}
