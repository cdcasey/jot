package db

import (
	"strings"
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

// --- Summary ---

func TestGetSummary(t *testing.T) {
	d := openTestDB(t)

	d.CreateThing("open task", "", "", "", nil)
	d.CreateThing("overdue task", "", "", "2020-01-01", nil)
	id3, _ := d.CreateThing("done task", "", "", "", nil)
	d.CompleteThing(id3)

	s, err := d.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if s.OpenThings != 2 {
		t.Errorf("expected 2 open things, got %d", s.OpenThings)
	}
	if len(s.OverdueThings) != 1 {
		t.Errorf("expected 1 overdue thing, got %d", len(s.OverdueThings))
	}
	if len(s.RecentThings) != 3 {
		t.Errorf("expected 3 recent things, got %d", len(s.RecentThings))
	}
}

// --- Check-ins ---

func TestGetLastCheckIn(t *testing.T) {
	d := openTestDB(t)

	// No check-ins yet
	summary, date, err := d.GetLastCheckIn()
	if err != nil {
		t.Fatalf("GetLastCheckIn (empty): %v", err)
	}
	if summary != "" || date != "" {
		t.Errorf("expected empty, got summary=%q date=%q", summary, date)
	}

	// Create one
	_, err = d.CreateCheckIn("everything is fine")
	if err != nil {
		t.Fatalf("CreateCheckIn: %v", err)
	}
	summary, date, err = d.GetLastCheckIn()
	if err != nil {
		t.Fatalf("GetLastCheckIn: %v", err)
	}
	if summary != "everything is fine" {
		t.Errorf("expected %q, got %q", "everything is fine", summary)
	}
	if date == "" {
		t.Error("expected date to be set")
	}
}

// --- Memories ---

func TestSaveAndSearchMemories(t *testing.T) {
	d := openTestDB(t)

	id, err := d.SaveMemory("blocked on API review", "blocker", "agent", []string{"api"}, nil, "")
	if err != nil {
		t.Fatalf("SaveMemory: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	// Search by text
	results, err := d.SearchMemories("API", "", "", nil, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories(text): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "blocked on API review" {
		t.Errorf("expected content %q, got %q", "blocked on API review", results[0].Content)
	}

	// Search by category
	results, _ = d.SearchMemories("", "blocker", "", nil, "", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result by category, got %d", len(results))
	}

	// Search by tag
	results, _ = d.SearchMemories("", "", "api", nil, "", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result by tag, got %d", len(results))
	}

	// Search miss
	results, _ = d.SearchMemories("nonexistent", "", "", nil, "", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMemoryExpiry(t *testing.T) {
	d := openTestDB(t)

	// Permanent memory
	d.SaveMemory("permanent note", "observation", "agent", nil, nil, "")

	// Already expired
	past := time.Now().UTC().Add(-24 * time.Hour).Format(time.DateTime)
	d.SaveMemory("expired note", "observation", "agent", nil, nil, past)

	// Future expiry
	future := time.Now().UTC().Add(24 * time.Hour).Format(time.DateTime)
	d.SaveMemory("future note", "observation", "agent", nil, nil, future)

	// ListRecentMemories should exclude expired
	memories, err := d.ListRecentMemories("", 10)
	if err != nil {
		t.Fatalf("ListRecentMemories: %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("expected 2 non-expired memories, got %d", len(memories))
	}

	// PruneExpiredMemories should delete the expired one
	pruned, err := d.PruneExpiredMemories()
	if err != nil {
		t.Fatalf("PruneExpiredMemories: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	// After prune, total should still be 2 (permanent + future)
	memories, _ = d.ListRecentMemories("", 10)
	if len(memories) != 2 {
		t.Errorf("expected 2 memories after prune, got %d", len(memories))
	}
}

func TestSearchMemoriesByThing(t *testing.T) {
	d := openTestDB(t)

	thingID, _ := d.CreateThing("my project", "", "", "", nil)
	d.SaveMemory("thing memory", "observation", "agent", nil, &thingID, "")
	d.SaveMemory("general memory", "observation", "agent", nil, nil, "")

	results, err := d.SearchMemories("", "", "", &thingID, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories(thing): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for thing, got %d", len(results))
	}
	if results[0].Content != "thing memory" {
		t.Errorf("expected %q, got %q", "thing memory", results[0].Content)
	}
}

func TestListRecentMemoriesFilterByCategory(t *testing.T) {
	d := openTestDB(t)

	d.SaveMemory("a blocker", "blocker", "agent", nil, nil, "")
	d.SaveMemory("a decision", "decision", "agent", nil, nil, "")

	memories, err := d.ListRecentMemories("blocker", 10)
	if err != nil {
		t.Fatalf("ListRecentMemories(blocker): %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(memories))
	}
	if memories[0].Category != "blocker" {
		t.Errorf("expected category %q, got %q", "blocker", memories[0].Category)
	}
}

// --- FTS Search ---

func TestSearchMemoriesFTS(t *testing.T) {
	d := openTestDB(t)

	d.SaveMemory("blocked on API review from the platform team", "blocker", "agent", []string{"api"}, nil, "")
	d.SaveMemory("decided to use PostgreSQL for the new service", "decision", "agent", nil, nil, "")
	d.SaveMemory("the API gateway latency is too high", "observation", "agent", []string{"api"}, nil, "")

	// FTS should find both API-related memories
	results, err := d.SearchMemories("API", "", "", nil, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories(FTS): %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 FTS results for 'API', got %d", len(results))
	}

	// FTS + category filter
	results, err = d.SearchMemories("API", "blocker", "", nil, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories(FTS+category): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 FTS result for 'API' + blocker, got %d", len(results))
	}
	if results[0].Category != "blocker" {
		t.Errorf("expected category %q, got %q", "blocker", results[0].Category)
	}

	// FTS query with no matches
	results, _ = d.SearchMemories("kubernetes", "", "", nil, "", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'kubernetes', got %d", len(results))
	}
}

func TestFTSSyncOnInsert(t *testing.T) {
	d := openTestDB(t)

	d.SaveMemory("unique snowflake memory", "observation", "agent", nil, nil, "")

	results, err := d.SearchMemories("snowflake", "", "", nil, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestFTSSyncOnDelete(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.SaveMemory("ephemeral thought", "observation", "agent", nil, nil, "")

	// Should find it
	results, _ := d.SearchMemories("ephemeral", "", "", nil, "", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	// Delete and verify FTS no longer finds it
	err := d.DeleteMemory(id)
	if err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}
	results, _ = d.SearchMemories("ephemeral", "", "", nil, "", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

// --- Memory Management ---

func TestUpdateMemory(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.SaveMemory("original content", "observation", "agent", []string{"tag1"}, nil, "")

	err := d.UpdateMemory(id, map[string]any{"content": "updated content", "category": "decision"})
	if err != nil {
		t.Fatalf("UpdateMemory: %v", err)
	}

	// Verify the update via search
	results, _ := d.SearchMemories("updated", "", "", nil, "", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "updated content" {
		t.Errorf("expected content %q, got %q", "updated content", results[0].Content)
	}
	if results[0].Category != "decision" {
		t.Errorf("expected category %q, got %q", "decision", results[0].Category)
	}
}

func TestUpdateMemoryNotFound(t *testing.T) {
	d := openTestDB(t)

	err := d.UpdateMemory(9999, map[string]any{"content": "nope"})
	if err == nil {
		t.Error("expected error updating nonexistent memory, got nil")
	}
}

func TestDeleteMemory(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.SaveMemory("to be deleted", "observation", "agent", nil, nil, "")

	err := d.DeleteMemory(id)
	if err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	// Verify gone
	results, _ := d.ListRecentMemories("", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(results))
	}

	// Deleting nonexistent should error
	err = d.DeleteMemory(9999)
	if err == nil {
		t.Error("expected error deleting nonexistent memory, got nil")
	}
}

func TestResolveMemory(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.SaveMemory("blocked on code review", "blocker", "agent", nil, nil, "")

	err := d.ResolveMemory(id, "review completed by Sarah")
	if err != nil {
		t.Fatalf("ResolveMemory: %v", err)
	}

	// Verify category changed and content appended
	results, _ := d.SearchMemories("blocked", "", "", nil, "", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Category != "resolved" {
		t.Errorf("expected category %q, got %q", "resolved", results[0].Category)
	}
	if !strings.Contains(results[0].Content, "Resolution: review completed by Sarah") {
		t.Errorf("expected resolution text in content, got %q", results[0].Content)
	}
}

// --- Skills ---

func TestCreateAndListSkills(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateSkill("cool skill", "skill description", "some fancy skill contents", []string{"go", "ai"})
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}

	skills, err := d.ListSkills("go")
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, skills[0].ID)
	}
	if skills[0].Name != "cool skill" {
		t.Errorf("expected name %q, got %q", "cool skill", skills[0].Name)
	}
	if len(skills[0].Tags) != 2 || skills[0].Tags[0] != "go" || skills[0].Tags[1] != "ai" {
		t.Errorf("expected tags [go, ai], got %v", skills[0].Tags)
	}
}

func TestGetSkillByName(t *testing.T) {
	d := openTestDB(t)

	d.CreateSkill("my-skill", "a description", "skill content", nil)

	// Found
	skill, err := d.GetSkill("my-skill")
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if skill == nil {
		t.Fatal("expected skill, got nil")
	}
	if skill.Name != "my-skill" {
		t.Errorf("expected name %q, got %q", "my-skill", skill.Name)
	}
	if skill.Description != "a description" {
		t.Errorf("expected description %q, got %q", "a description", skill.Description)
	}
	if skill.Content != "skill content" {
		t.Errorf("expected content %q, got %q", "skill content", skill.Content)
	}

	// Not found
	skill, err = d.GetSkill("nonexistent")
	if err != nil {
		t.Fatalf("GetSkill(nonexistent): %v", err)
	}
	if skill != nil {
		t.Errorf("expected nil for nonexistent skill, got %+v", skill)
	}
}

func TestListSkillsFilterByTag(t *testing.T) {
	d := openTestDB(t)

	d.CreateSkill("skill-1", "desc", "content", []string{"go", "cli"})
	d.CreateSkill("skill-2", "desc", "content", []string{"python"})
	d.CreateSkill("skill-3", "desc", "content", nil)

	skills, err := d.ListSkills("go")
	if err != nil {
		t.Fatalf("ListSkills(go): %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill with tag 'go', got %d", len(skills))
	}
	if skills[0].Name != "skill-1" {
		t.Errorf("expected %q, got %q", "skill-1", skills[0].Name)
	}

	// No filter returns all
	skills, err = d.ListSkills("")
	if err != nil {
		t.Fatalf("ListSkills(all): %v", err)
	}
	if len(skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(skills))
	}
}

func TestUpdateSkill(t *testing.T) {
	d := openTestDB(t)

	d.CreateSkill("updatable", "original desc", "original content", []string{"tag1"})

	err := d.UpdateSkill("updatable", map[string]any{"description": "new desc"})
	if err != nil {
		t.Fatalf("UpdateSkill: %v", err)
	}

	skill, _ := d.GetSkill("updatable")
	if skill.Description != "new desc" {
		t.Errorf("expected description %q, got %q", "new desc", skill.Description)
	}
	// Content should be unchanged
	if skill.Content != "original content" {
		t.Errorf("content changed unexpectedly: got %q", skill.Content)
	}
}

func TestDeleteSkill(t *testing.T) {
	d := openTestDB(t)

	d.CreateSkill("to-delete", "desc", "content", nil)

	err := d.DeleteSkill("to-delete")
	if err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}

	skill, _ := d.GetSkill("to-delete")
	if skill != nil {
		t.Errorf("expected skill to be deleted, but found %+v", skill)
	}

	// Deleting nonexistent should error
	err = d.DeleteSkill("nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent skill, got nil")
	}
}

func TestCreateSkillDuplicateName(t *testing.T) {
	d := openTestDB(t)

	_, err := d.CreateSkill("unique-name", "desc", "content", nil)
	if err != nil {
		t.Fatalf("first CreateSkill: %v", err)
	}

	_, err = d.CreateSkill("unique-name", "other desc", "other content", nil)
	if err == nil {
		t.Error("expected error creating duplicate skill name, got nil")
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

// --- Reminders ---

func TestCreateAndListReminders(t *testing.T) {
	d := openTestDB(t)

	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)
	id, err := d.CreateReminder("check the build", future)
	if err != nil {
		t.Fatalf("CreateReminder: %v", err)
	}

	reminders, err := d.ListUpcomingReminders()
	if err != nil {
		t.Fatalf("ListUpcomingReminders: %v", err)
	}
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	r := reminders[0]
	if r.ID != id {
		t.Errorf("expected ID %d, got %d", id, r.ID)
	}
	if r.Prompt != "check the build" {
		t.Errorf("expected prompt %q, got %q", "check the build", r.Prompt)
	}
	if r.Fired {
		t.Error("expected reminder to not be fired")
	}
}

func TestListPendingReminders(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)

	d.CreateReminder("due now", past)
	d.CreateReminder("not yet", future)

	pending, err := d.ListPendingReminders()
	if err != nil {
		t.Fatalf("ListPendingReminders: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(pending))
	}
	if pending[0].Prompt != "due now" {
		t.Errorf("expected %q, got %q", "due now", pending[0].Prompt)
	}
}

func TestListUpcomingReminders(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)

	d.CreateReminder("already due", past)
	d.CreateReminder("upcoming", future)

	upcoming, err := d.ListUpcomingReminders()
	if err != nil {
		t.Fatalf("ListUpcomingReminders: %v", err)
	}
	if len(upcoming) != 1 {
		t.Fatalf("expected 1 upcoming reminder, got %d", len(upcoming))
	}
	if upcoming[0].Prompt != "upcoming" {
		t.Errorf("expected %q, got %q", "upcoming", upcoming[0].Prompt)
	}
}

func TestMarkReminderFired(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	id, _ := d.CreateReminder("fire me", past)

	err := d.MarkReminderFired(id)
	if err != nil {
		t.Fatalf("MarkReminderFired: %v", err)
	}

	pending, _ := d.ListPendingReminders()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after firing, got %d", len(pending))
	}
}

func TestPendingRemindersExcludesFired(t *testing.T) {
	d := openTestDB(t)

	past := time.Now().UTC().Add(-time.Minute).Format(time.DateTime)
	id, _ := d.CreateReminder("already fired", past)
	d.MarkReminderFired(id)
	d.CreateReminder("also due but not fired", past)

	pending, err := d.ListPendingReminders()
	if err != nil {
		t.Fatalf("ListPendingReminders: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].Prompt != "also due but not fired" {
		t.Errorf("expected unfired reminder, got %q", pending[0].Prompt)
	}
}
