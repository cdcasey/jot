package db

import (
	"database/sql"
	"fmt"
	"time"
)

// LogHabit inserts a habit log entry.
func (d *DB) LogHabit(habit, outcome, notes, loggedAt string) (int64, error) {
	res, err := d.conn.Exec(
		"INSERT INTO habit_logs (habit, outcome, notes, logged_at) VALUES (?, ?, ?, ?)",
		habit, outcome, nullStr(notes), loggedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("logging habit: %w", err)
	}
	return res.LastInsertId()
}

// ListHabits returns a summary of all tracked habits.
func (d *DB) ListHabits() ([]HabitSummary, error) {
	rows, err := d.conn.Query(`
		SELECT habit,
		       SUM(CASE WHEN logged_at >= date('now', '-7 days') THEN 1 ELSE 0 END),
		       MAX(logged_at)
		FROM habit_logs
		GROUP BY habit
		ORDER BY MAX(logged_at) DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing habits: %w", err)
	}
	defer rows.Close()

	var out []HabitSummary
	for rows.Next() {
		var h HabitSummary
		if err := rows.Scan(&h.Habit, &h.Last7Days, &h.LastLogged); err != nil {
			return nil, fmt.Errorf("scanning habit summary: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHabitStats returns statistics for a single habit.
// days controls the window for outcome counts; streaks use full history.
// today should be the current date in the user's timezone.
func (d *DB) GetHabitStats(habit string, days int, today time.Time) (*HabitStats, error) {
	stats := &HabitStats{Habit: habit, Days: days}

	// 1. Outcome counts within the day window
	rows, err := d.conn.Query(
		`SELECT outcome, COUNT(*) FROM habit_logs
		 WHERE habit = ? AND logged_at >= date(?, '-' || ? || ' days')
		 GROUP BY outcome`,
		habit, today.Format("2006-01-02"), days,
	)
	if err != nil {
		return nil, fmt.Errorf("counting habit outcomes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var outcome string
		var count int
		if err := rows.Scan(&outcome, &count); err != nil {
			return nil, fmt.Errorf("scanning outcome count: %w", err)
		}
		switch outcome {
		case "done":
			stats.DoneCount = count
		case "skipped":
			stats.SkippedCount = count
		case "partial":
			stats.PartialCount = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. All "done" dates for streak calculation (full history)
	dateRows, err := d.conn.Query(
		`SELECT DISTINCT logged_at FROM habit_logs
		 WHERE habit = ? AND outcome = 'done'
		 ORDER BY logged_at ASC`,
		habit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying done dates: %w", err)
	}
	defer dateRows.Close()
	var dates []string
	for dateRows.Next() {
		var d string
		if err := dateRows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scanning done date: %w", err)
		}
		dates = append(dates, d)
	}
	if err := dateRows.Err(); err != nil {
		return nil, err
	}

	stats.CurrentStreak = computeCurrentStreak(dates, today)
	stats.LongestStreak = computeLongestStreak(dates)

	// 3. Recent logs (last 10, any outcome)
	logRows, err := d.conn.Query(
		`SELECT id, habit, outcome, COALESCE(notes,''), logged_at, created_at
		 FROM habit_logs WHERE habit = ? ORDER BY logged_at DESC LIMIT 10`,
		habit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying recent habit logs: %w", err)
	}
	defer logRows.Close()
	stats.RecentLogs, err = scanHabitLogs(logRows)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func scanHabitLogs(rows *sql.Rows) ([]HabitLog, error) {
	var out []HabitLog
	for rows.Next() {
		var h HabitLog
		if err := rows.Scan(&h.ID, &h.Habit, &h.Outcome, &h.Notes, &h.LoggedAt, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning habit log: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// computeCurrentStreak walks backward from the end of sorted done dates.
// The streak is only "live" if the most recent date is today or yesterday.
func computeCurrentStreak(dates []string, today time.Time) int {
	if len(dates) == 0 {
		return 0
	}

	todayDate := today.Truncate(24 * time.Hour)

	last, err := time.Parse("2006-01-02", dates[len(dates)-1])
	if err != nil {
		return 0
	}

	// Streak is only live if last done is today or yesterday
	gap := todayDate.Sub(last)
	if gap > 24*time.Hour {
		return 0
	}

	streak := 1
	for i := len(dates) - 2; i >= 0; i-- {
		cur, err := time.Parse("2006-01-02", dates[i])
		if err != nil {
			break
		}
		next, _ := time.Parse("2006-01-02", dates[i+1])
		if next.Sub(cur) == 24*time.Hour {
			streak++
		} else {
			break
		}
	}
	return streak
}

// computeLongestStreak walks forward through sorted dates tracking the max consecutive run.
func computeLongestStreak(dates []string) int {
	if len(dates) == 0 {
		return 0
	}

	longest := 1
	current := 1
	for i := 1; i < len(dates); i++ {
		prev, err1 := time.Parse("2006-01-02", dates[i-1])
		cur, err2 := time.Parse("2006-01-02", dates[i])
		if err1 != nil || err2 != nil {
			current = 1
			continue
		}
		if cur.Sub(prev) == 24*time.Hour {
			current++
		} else {
			current = 1
		}
		if current > longest {
			longest = current
		}
	}
	return longest
}
