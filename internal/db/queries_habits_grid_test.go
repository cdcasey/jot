package db

import "testing"

func TestHabitGridReflectsLogsReadOnly(t *testing.T) {
	d := openTestDB(t)

	med, _, _ := d.GetOrCreateHabit("meditate")
	d.GetOrCreateHabit("read") // a second habit with no logs in range
	if err := d.LogHabit(med.ID, "2026-06-16"); err != nil {
		t.Fatalf("LogHabit: %v", err)
	}

	rows, err := d.HabitGrid("2026-06-15", "2026-06-17")
	if err != nil {
		t.Fatalf("HabitGrid: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 habit rows, got %d", len(rows))
	}

	byName := map[string]HabitGridRow{}
	for _, r := range rows {
		byName[r.Habit.Name] = r
	}
	// Filled cell where a log exists.
	if !byName["meditate"].Done["2026-06-16"] {
		t.Error("expected meditate filled on 2026-06-16")
	}
	// Empty where none.
	if byName["meditate"].Done["2026-06-15"] {
		t.Error("expected meditate empty on 2026-06-15")
	}
	if len(byName["read"].Done) != 0 {
		t.Errorf("expected read to have no filled cells, got %v", byName["read"].Done)
	}

	// Read-only: the grid query must not have created any habit_logs rows.
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM habit_logs").Scan(&n); err != nil {
		t.Fatalf("counting logs: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 habit_log (the one we created), got %d", n)
	}
}

func TestHabitGridExcludesOutOfRangeAndInactive(t *testing.T) {
	d := openTestDB(t)
	med, _, _ := d.GetOrCreateHabit("meditate")
	_ = d.LogHabit(med.ID, "2026-06-10") // before window
	_ = d.LogHabit(med.ID, "2026-06-16") // in window

	// Deactivate a second habit; it should not appear as a row.
	gone, _, _ := d.GetOrCreateHabit("gone")
	mustExec(t, d, "UPDATE habits SET active = 0 WHERE id = ?", gone.ID)

	rows, err := d.HabitGrid("2026-06-15", "2026-06-17")
	if err != nil {
		t.Fatalf("HabitGrid: %v", err)
	}
	if len(rows) != 1 || rows[0].Habit.Name != "meditate" {
		t.Fatalf("expected only active meditate row, got %+v", rows)
	}
	if rows[0].Done["2026-06-10"] {
		t.Error("out-of-range date should not be filled")
	}
	if !rows[0].Done["2026-06-16"] {
		t.Error("in-range date should be filled")
	}
}
