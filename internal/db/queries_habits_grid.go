package db

import "fmt"

// HabitGridRow is one habit's row in the calendar grid: the habit plus the set
// of dates (within the requested window) on which it was logged. A cell is
// "filled" when its date is present in Done.
type HabitGridRow struct {
	Habit Habit
	Done  map[string]bool
}

// HabitGrid returns, for every active habit, the set of habit_logs dates within
// [start, end] inclusive. It is strictly read-only — it creates, updates, and
// deletes nothing — and is the substrate the read-only web grid renders. Habits
// with no logs in range still appear, with an empty Done set, so the grid shows
// every active habit as a row.
func (d *DB) HabitGrid(start, end string) ([]HabitGridRow, error) {
	rows, err := d.conn.Query(
		`SELECT h.id, h.name, h.active, h.created_at, l.date
		 FROM habits h
		 LEFT JOIN habit_logs l
		   ON l.habit_id = h.id AND l.date >= ? AND l.date <= ?
		 WHERE h.active = 1
		 ORDER BY h.name ASC, l.date ASC`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("querying habit grid: %w", err)
	}
	defer rows.Close()

	var out []HabitGridRow
	byID := map[int64]int{} // habit id -> index in out
	for rows.Next() {
		var (
			id       int64
			name     string
			active   int
			created  string
			date     *string // NULL when the habit has no log in range
		)
		if err := rows.Scan(&id, &name, &active, &created, &date); err != nil {
			return nil, fmt.Errorf("scanning habit grid row: %w", err)
		}
		idx, ok := byID[id]
		if !ok {
			out = append(out, HabitGridRow{
				Habit: Habit{ID: id, Name: name, Active: active == 1, CreatedAt: created},
				Done:  map[string]bool{},
			})
			idx = len(out) - 1
			byID[id] = idx
		}
		if date != nil {
			out[idx].Done[*date] = true
		}
	}
	return out, rows.Err()
}
