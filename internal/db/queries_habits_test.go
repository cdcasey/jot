package db

import (
	"testing"
)

// countHabitLogs returns the number of habit_logs rows for a habit+date.
func countHabitLogs(t *testing.T, d *DB, habitID int64, date string) int {
	t.Helper()
	var n int
	if err := d.conn.QueryRow(
		"SELECT COUNT(*) FROM habit_logs WHERE habit_id = ? AND date = ?", habitID, date,
	).Scan(&n); err != nil {
		t.Fatalf("counting habit logs: %v", err)
	}
	return n
}

func TestGetOrCreateHabit(t *testing.T) {
	d := openTestDB(t)

	h, created, err := d.GetOrCreateHabit("meditate")
	if err != nil {
		t.Fatalf("GetOrCreateHabit: %v", err)
	}
	if !created {
		t.Error("expected habit to be newly created")
	}
	if h.Name != "meditate" || !h.Active {
		t.Errorf("unexpected habit: %+v", h)
	}

	// Second call resolves the existing habit without creating.
	h2, created2, err := d.GetOrCreateHabit("meditate")
	if err != nil {
		t.Fatalf("GetOrCreateHabit (2nd): %v", err)
	}
	if created2 {
		t.Error("expected existing habit, not a new one")
	}
	if h2.ID != h.ID {
		t.Errorf("expected same habit id %d, got %d", h.ID, h2.ID)
	}
}

func TestLogHabitIsIdempotent(t *testing.T) {
	d := openTestDB(t)
	h, _, _ := d.GetOrCreateHabit("run")

	if err := d.LogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("LogHabit: %v", err)
	}
	// Re-logging the same (habit, date) must be a harmless no-op.
	if err := d.LogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("LogHabit (re-log): %v", err)
	}
	if got := countHabitLogs(t, d, h.ID, "2026-06-16"); got != 1 {
		t.Errorf("expected exactly 1 log row, got %d", got)
	}
}

func TestUnlogHabit(t *testing.T) {
	d := openTestDB(t)
	h, _, _ := d.GetOrCreateHabit("stretch")
	if err := d.LogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("LogHabit: %v", err)
	}

	if err := d.UnlogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("UnlogHabit: %v", err)
	}
	if got := countHabitLogs(t, d, h.ID, "2026-06-16"); got != 0 {
		t.Errorf("expected 0 log rows after unlog, got %d", got)
	}

	// Habit definition must survive the unlog.
	hh, _, _ := d.GetOrCreateHabit("stretch")
	if hh.ID != h.ID {
		t.Errorf("habit definition changed after unlog: %d != %d", hh.ID, h.ID)
	}

	// Unlogging an absent entry is a no-op, not an error.
	if err := d.UnlogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("UnlogHabit (absent): %v", err)
	}
}

func TestRenameHabitPreservesLogs(t *testing.T) {
	d := openTestDB(t)
	h, _, _ := d.GetOrCreateHabit("meditate")
	if err := d.LogHabit(h.ID, "2026-06-16"); err != nil {
		t.Fatalf("LogHabit: %v", err)
	}

	if err := d.RenameHabit(h.ID, "meditation"); err != nil {
		t.Fatalf("RenameHabit: %v", err)
	}

	// Same habit id, new name, log row intact.
	logs, err := d.HabitLogsInRange(h.ID, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("HabitLogsInRange: %v", err)
	}
	if len(logs) != 1 || logs[0].Date != "2026-06-16" {
		t.Errorf("rename altered logs: %+v", logs)
	}

	renamed, _, _ := d.GetOrCreateHabit("meditation")
	if renamed.ID != h.ID {
		t.Errorf("expected same habit id after rename, got %d != %d", renamed.ID, h.ID)
	}

	if err := d.RenameHabit(99999, "ghost"); err == nil {
		t.Error("expected error renaming nonexistent habit")
	}
}

func TestHabitLogsInRange(t *testing.T) {
	d := openTestDB(t)
	h, _, _ := d.GetOrCreateHabit("water")
	for _, date := range []string{"2026-06-10", "2026-06-12", "2026-06-20"} {
		if err := d.LogHabit(h.ID, date); err != nil {
			t.Fatalf("LogHabit %s: %v", date, err)
		}
	}

	tests := []struct {
		name      string
		start     string
		end       string
		wantDates []string
	}{
		{"full month", "2026-06-01", "2026-06-30", []string{"2026-06-10", "2026-06-12", "2026-06-20"}},
		{"narrow range excludes endpoints", "2026-06-11", "2026-06-13", []string{"2026-06-12"}},
		{"inclusive endpoints", "2026-06-10", "2026-06-12", []string{"2026-06-10", "2026-06-12"}},
		{"empty range", "2026-07-01", "2026-07-31", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := d.HabitLogsInRange(h.ID, tc.start, tc.end)
			if err != nil {
				t.Fatalf("HabitLogsInRange: %v", err)
			}
			if len(logs) != len(tc.wantDates) {
				t.Fatalf("expected %d logs, got %d (%+v)", len(tc.wantDates), len(logs), logs)
			}
			for i, want := range tc.wantDates {
				if logs[i].Date != want {
					t.Errorf("log %d: expected date %s, got %s", i, want, logs[i].Date)
				}
			}
		})
	}
}
