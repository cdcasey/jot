package db

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// --- Things ---

func TestCreateAndListThings(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateThing("buy milk", "from the store", "high", "2025-12-31", []string{"errands", "food"})
	if err != nil {
		t.Fatalf("CreateThing: %v", err)
	}

	things, err := d.ListThings("", "", "")
	if err != nil {
		t.Fatalf("ListThings: %v", err)
	}
	if len(things) != 1 {
		t.Fatalf("expected 1 thing, got %d", len(things))
	}
	th := things[0]
	if th.ID != id {
		t.Errorf("expected ID %d, got %d", id, th.ID)
	}
	if th.Title != "buy milk" {
		t.Errorf("expected title %q, got %q", "buy milk", th.Title)
	}
	if th.Notes != "from the store" {
		t.Errorf("expected notes %q, got %q", "from the store", th.Notes)
	}
	if th.Priority != "high" {
		t.Errorf("expected priority %q, got %q", "high", th.Priority)
	}
	if th.DueDate != "2025-12-31" {
		t.Errorf("expected due_date %q, got %q", "2025-12-31", th.DueDate)
	}
	if th.Status != "open" {
		t.Errorf("expected status %q, got %q", "open", th.Status)
	}
	if len(th.Tags) != 2 || th.Tags[0] != "errands" || th.Tags[1] != "food" {
		t.Errorf("expected tags [errands, food], got %v", th.Tags)
	}
}

func TestListThingsFilters(t *testing.T) {
	d := openTestDB(t)

	d.CreateThing("task A", "", "high", "", []string{"work"})
	d.CreateThing("task B", "", "low", "", []string{"personal"})
	id3, _ := d.CreateThing("task C", "", "normal", "", []string{"work"})
	d.UpdateThing(id3, map[string]any{"status": "active"})

	tests := []struct {
		name      string
		status    string
		priority  string
		tag       string
		wantCount int
	}{
		{"no filter", "", "", "", 3},
		{"by status open", "open", "", "", 2},
		{"by status active", "active", "", "", 1},
		{"by priority high", "", "high", "", 1},
		{"by tag work", "", "", "work", 2},
		{"by status+tag", "open", "", "work", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			things, err := d.ListThings(tt.status, tt.priority, tt.tag)
			if err != nil {
				t.Fatalf("ListThings: %v", err)
			}
			if len(things) != tt.wantCount {
				t.Errorf("expected %d things, got %d", tt.wantCount, len(things))
			}
		})
	}
}

func TestCompleteThing(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateThing("finish this", "", "", "", nil)
	err := d.CompleteThing(id)
	if err != nil {
		t.Fatalf("CompleteThing: %v", err)
	}

	things, _ := d.ListThings("done", "", "")
	if len(things) != 1 {
		t.Fatalf("expected 1 done thing, got %d", len(things))
	}
	if things[0].Status != "done" {
		t.Errorf("expected status %q, got %q", "done", things[0].Status)
	}
	if things[0].CompletedAt == "" {
		t.Error("expected completed_at to be set")
	}
}

func TestUpdateThing(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateThing("original title", "original notes", "normal", "", nil)

	err := d.UpdateThing(id, map[string]any{"title": "new title", "priority": "urgent"})
	if err != nil {
		t.Fatalf("UpdateThing: %v", err)
	}

	things, _ := d.ListThings("", "", "")
	if things[0].Title != "new title" {
		t.Errorf("expected title %q, got %q", "new title", things[0].Title)
	}
	if things[0].Priority != "urgent" {
		t.Errorf("expected priority %q, got %q", "urgent", things[0].Priority)
	}
	// Notes should be unchanged
	if things[0].Notes != "original notes" {
		t.Errorf("notes changed unexpectedly: got %q", things[0].Notes)
	}
}

func TestUpdateThingEmptyFields(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateThing("title", "", "", "", nil)

	// Empty fields map should be a no-op, not an error
	err := d.UpdateThing(id, map[string]any{})
	if err != nil {
		t.Fatalf("UpdateThing with empty fields: %v", err)
	}
}

func TestUpdateThingRejectsBogusColumn(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateThing("task", "", "", "", nil)

	err := d.UpdateThing(id, map[string]any{"title\"; DROP TABLE things; --": "pwned"})
	if err == nil {
		t.Fatal("expected error for disallowed column, got nil")
	}
}

// --- Notes ---

func TestGetSetNote(t *testing.T) {
	d := openTestDB(t)

	// Nonexistent key returns empty
	val, err := d.GetNote("missing")
	if err != nil {
		t.Fatalf("GetNote(missing): %v", err)
	}
	if val != "" {
		t.Errorf("expected empty for missing key, got %q", val)
	}

	// Set and get
	if err := d.SetNote("greeting", "hello"); err != nil {
		t.Fatalf("SetNote: %v", err)
	}
	val, err = d.GetNote("greeting")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected %q, got %q", "hello", val)
	}

	// Upsert
	if err := d.SetNote("greeting", "hi"); err != nil {
		t.Fatalf("SetNote(upsert): %v", err)
	}
	val, _ = d.GetNote("greeting")
	if val != "hi" {
		t.Errorf("expected %q after upsert, got %q", "hi", val)
	}
}

// --- Overdue ---

func TestListThingsOverdue(t *testing.T) {
	d := openTestDB(t)

	d.CreateThing("open task", "", "", "", nil)
	d.CreateThing("overdue task", "", "", "2020-01-01", nil)
	id3, _ := d.CreateThing("done task", "", "", "2020-01-01", nil)
	d.CompleteThing(id3)

	things, err := d.ListThings("", "", "")
	if err != nil {
		t.Fatalf("ListThings: %v", err)
	}

	overdueCount := 0
	for _, th := range things {
		if th.Overdue {
			overdueCount++
			if th.Title != "overdue task" {
				t.Errorf("unexpected overdue thing: %q", th.Title)
			}
		}
	}
	if overdueCount != 1 {
		t.Errorf("expected 1 overdue thing, got %d", overdueCount)
	}

	// Done things should not be marked overdue even with a past due_date.
	doneThings, _ := d.ListThings("done", "", "")
	for _, th := range doneThings {
		if th.Overdue {
			t.Errorf("done thing %q should not be overdue", th.Title)
		}
	}
}

// --- Schedules ---

func TestCreateAndListSchedules(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateSchedule("daily-review", "0 9 * * *", "Do a daily review.")
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	schedules, err := d.ListSchedules(false)
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	s := schedules[0]
	if s.ID != id {
		t.Errorf("expected ID %d, got %d", id, s.ID)
	}
	if s.Name != "daily-review" {
		t.Errorf("expected name %q, got %q", "daily-review", s.Name)
	}
	if s.CronExpr != "0 9 * * *" {
		t.Errorf("expected cron %q, got %q", "0 9 * * *", s.CronExpr)
	}
	if !s.Enabled {
		t.Error("expected schedule to be enabled by default")
	}
}

func TestCreateScheduleDuplicateName(t *testing.T) {
	d := openTestDB(t)

	_, err := d.CreateSchedule("same-name", "0 9 * * *", "prompt")
	if err != nil {
		t.Fatalf("first CreateSchedule: %v", err)
	}
	_, err = d.CreateSchedule("same-name", "0 10 * * *", "other prompt")
	if err == nil {
		t.Error("expected error on duplicate schedule name, got nil")
	}
}

func TestUpdateScheduleEnableDisable(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateSchedule("toggle-me", "0 9 * * *", "prompt")

	// Disable it
	err := d.UpdateSchedule(id, map[string]any{"enabled": 0})
	if err != nil {
		t.Fatalf("UpdateSchedule(disable): %v", err)
	}

	// Should not appear in enabledOnly list
	schedules, _ := d.ListSchedules(true)
	if len(schedules) != 0 {
		t.Errorf("expected 0 enabled schedules, got %d", len(schedules))
	}

	// Re-enable
	d.UpdateSchedule(id, map[string]any{"enabled": 1})
	schedules, _ = d.ListSchedules(true)
	if len(schedules) != 1 {
		t.Errorf("expected 1 enabled schedule after re-enable, got %d", len(schedules))
	}
}

func TestRecordScheduleRun(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateSchedule("run-me", "0 9 * * *", "prompt")

	schedules, _ := d.ListSchedules(false)
	if schedules[0].LastRun != "" {
		t.Errorf("expected empty last_run, got %q", schedules[0].LastRun)
	}

	err := d.RecordScheduleRun(id)
	if err != nil {
		t.Fatalf("RecordScheduleRun: %v", err)
	}

	schedules, _ = d.ListSchedules(false)
	if schedules[0].LastRun == "" {
		t.Error("expected last_run to be set after run")
	}
}

func TestDeleteSchedule(t *testing.T) {
	d := openTestDB(t)

	d.CreateSchedule("to-delete", "0 9 * * *", "prompt")

	err := d.DeleteSchedule("to-delete")
	if err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}

	schedules, _ := d.ListSchedules(false)
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules after delete, got %d", len(schedules))
	}
}

// --- One-Shot Schedules (Reminders) ---

func TestCreateOneShot(t *testing.T) {
	d := openTestDB(t)

	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)
	id, err := d.CreateOneShot("reminder-dentist", "call the dentist", future)
	if err != nil {
		t.Fatalf("CreateOneShot: %v", err)
	}

	upcoming, err := d.ListUpcomingOneShots()
	if err != nil {
		t.Fatalf("ListUpcomingOneShots: %v", err)
	}
	if len(upcoming) != 1 {
		t.Fatalf("expected 1 upcoming one-shot, got %d", len(upcoming))
	}
	s := upcoming[0]
	if s.ID != id {
		t.Errorf("expected ID %d, got %d", id, s.ID)
	}
	if s.Prompt != "call the dentist" {
		t.Errorf("expected prompt %q, got %q", "call the dentist", s.Prompt)
	}
	if s.Fired {
		t.Error("expected one-shot to not be fired")
	}
	if s.CronExpr != "" {
		t.Errorf("expected empty cron_expr, got %q", s.CronExpr)
	}
}

func TestListPendingOneShots(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)

	d.CreateOneShot("reminder-due", "due now", past)
	d.CreateOneShot("reminder-future", "not yet", future)

	pending, err := d.ListPendingOneShots()
	if err != nil {
		t.Fatalf("ListPendingOneShots: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending one-shot, got %d", len(pending))
	}
	if pending[0].Prompt != "due now" {
		t.Errorf("expected %q, got %q", "due now", pending[0].Prompt)
	}
}

func TestMarkOneShotFired(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	id, _ := d.CreateOneShot("reminder-fire", "fire me", past)

	err := d.MarkOneShotFired(id)
	if err != nil {
		t.Fatalf("MarkOneShotFired: %v", err)
	}

	pending, _ := d.ListPendingOneShots()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after firing, got %d", len(pending))
	}
}

func TestPendingOneShotsExcludesFired(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	id, _ := d.CreateOneShot("reminder-fired", "already fired", past)
	d.MarkOneShotFired(id)
	d.CreateOneShot("reminder-unfired", "also due but not fired", past)

	pending, err := d.ListPendingOneShots()
	if err != nil {
		t.Fatalf("ListPendingOneShots: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].Prompt != "also due but not fired" {
		t.Errorf("expected unfired one-shot, got %q", pending[0].Prompt)
	}
}

func TestListSchedulesIncludesOneShots(t *testing.T) {
	d := openTestDB(t)

	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)
	d.CreateSchedule("daily-review", "0 9 * * *", "Do a daily review.")
	d.CreateOneShot("reminder-call", "call mom", future)

	schedules, err := d.ListSchedules(false)
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 2 {
		t.Fatalf("expected 2 schedules (1 cron + 1 one-shot), got %d", len(schedules))
	}
}

func TestGetScheduleByName(t *testing.T) {
	d := openTestDB(t)

	d.CreateSchedule("my-schedule", "0 9 * * *", "test prompt")

	s, err := d.GetScheduleByName("my-schedule")
	if err != nil {
		t.Fatalf("GetScheduleByName: %v", err)
	}
	if s == nil {
		t.Fatal("expected schedule, got nil")
	}
	if s.Name != "my-schedule" {
		t.Errorf("expected name %q, got %q", "my-schedule", s.Name)
	}

	// Not found
	s, err = d.GetScheduleByName("nonexistent")
	if err != nil {
		t.Fatalf("GetScheduleByName(missing): %v", err)
	}
	if s != nil {
		t.Errorf("expected nil for nonexistent, got %+v", s)
	}
}
