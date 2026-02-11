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

// --- Projects ---

func TestCreateAndListProjects(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateProject("test project", "a description")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	projects, err := d.ListProjects("")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, projects[0].ID)
	}
	if projects[0].Name != "test project" {
		t.Errorf("expected name %q, got %q", "test project", projects[0].Name)
	}
	if projects[0].Description != "a description" {
		t.Errorf("expected description %q, got %q", "a description", projects[0].Description)
	}
	if projects[0].Status != "active" {
		t.Errorf("expected status %q, got %q", "active", projects[0].Status)
	}
}

func TestListProjectsFilterByStatus(t *testing.T) {
	d := openTestDB(t)

	d.CreateProject("active one", "")
	id2, _ := d.CreateProject("paused one", "")
	d.UpdateProject(id2, map[string]any{"status": "paused"})

	active, err := d.ListProjects("active")
	if err != nil {
		t.Fatalf("ListProjects(active): %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active project, got %d", len(active))
	}
	if active[0].Name != "active one" {
		t.Errorf("expected %q, got %q", "active one", active[0].Name)
	}

	paused, err := d.ListProjects("paused")
	if err != nil {
		t.Fatalf("ListProjects(paused): %v", err)
	}
	if len(paused) != 1 {
		t.Fatalf("expected 1 paused project, got %d", len(paused))
	}
}

func TestUpdateProject(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateProject("original", "desc")

	err := d.UpdateProject(id, map[string]any{"name": "renamed"})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	projects, _ := d.ListProjects("")
	if projects[0].Name != "renamed" {
		t.Errorf("expected name %q, got %q", "renamed", projects[0].Name)
	}
	// Description should be unchanged
	if projects[0].Description != "desc" {
		t.Errorf("description changed unexpectedly: got %q", projects[0].Description)
	}
}

// --- Todos ---

func TestCreateAndListTodos(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateTodo("buy milk", nil, "from the store", "high", "2025-12-31")
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	todos, err := d.ListTodos(nil, "", "")
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, todos[0].ID)
	}
	if todos[0].Title != "buy milk" {
		t.Errorf("expected title %q, got %q", "buy milk", todos[0].Title)
	}
	if todos[0].Notes != "from the store" {
		t.Errorf("expected notes %q, got %q", "from the store", todos[0].Notes)
	}
	if todos[0].Priority != "high" {
		t.Errorf("expected priority %q, got %q", "high", todos[0].Priority)
	}
	if todos[0].DueDate != "2025-12-31" {
		t.Errorf("expected due_date %q, got %q", "2025-12-31", todos[0].DueDate)
	}
	if todos[0].Status != "pending" {
		t.Errorf("expected status %q, got %q", "pending", todos[0].Status)
	}
}

func TestListTodosFilters(t *testing.T) {
	d := openTestDB(t)

	pid, _ := d.CreateProject("proj", "")
	d.CreateTodo("task A", &pid, "", "high", "")
	d.CreateTodo("task B", nil, "", "low", "")
	id3, _ := d.CreateTodo("task C", &pid, "", "normal", "")
	d.UpdateTodo(id3, map[string]any{"status": "in_progress"})

	tests := []struct {
		name      string
		projectID *int64
		status    string
		priority  string
		wantCount int
	}{
		{"no filter", nil, "", "", 3},
		{"by project", &pid, "", "", 2},
		{"by status pending", nil, "pending", "", 2},
		{"by status in_progress", nil, "in_progress", "", 1},
		{"by priority high", nil, "", "high", 1},
		{"by project+status", &pid, "pending", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todos, err := d.ListTodos(tt.projectID, tt.status, tt.priority)
			if err != nil {
				t.Fatalf("ListTodos: %v", err)
			}
			if len(todos) != tt.wantCount {
				t.Errorf("expected %d todos, got %d", tt.wantCount, len(todos))
			}
		})
	}
}

func TestCompleteTodo(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateTodo("finish this", nil, "", "", "")
	err := d.CompleteTodo(id)
	if err != nil {
		t.Fatalf("CompleteTodo: %v", err)
	}

	todos, _ := d.ListTodos(nil, "done", "")
	if len(todos) != 1 {
		t.Fatalf("expected 1 done todo, got %d", len(todos))
	}
	if todos[0].Status != "done" {
		t.Errorf("expected status %q, got %q", "done", todos[0].Status)
	}
	if todos[0].CompletedAt == "" {
		t.Error("expected completed_at to be set")
	}
}

func TestUpdateTodo(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateTodo("original title", nil, "original notes", "normal", "")

	err := d.UpdateTodo(id, map[string]any{"title": "new title", "priority": "urgent"})
	if err != nil {
		t.Fatalf("UpdateTodo: %v", err)
	}

	todos, _ := d.ListTodos(nil, "", "")
	if todos[0].Title != "new title" {
		t.Errorf("expected title %q, got %q", "new title", todos[0].Title)
	}
	if todos[0].Priority != "urgent" {
		t.Errorf("expected priority %q, got %q", "urgent", todos[0].Priority)
	}
	// Notes should be unchanged
	if todos[0].Notes != "original notes" {
		t.Errorf("notes changed unexpectedly: got %q", todos[0].Notes)
	}
}

func TestUpdateTodoEmptyFields(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateTodo("title", nil, "", "", "")

	// Empty fields map should be a no-op, not an error
	err := d.UpdateTodo(id, map[string]any{})
	if err != nil {
		t.Fatalf("UpdateTodo with empty fields: %v", err)
	}
}

func TestUpdateTodoRejectsBogusColumn(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateTodo("task", nil, "", "", "")

	err := d.UpdateTodo(id, map[string]any{"title\"; DROP TABLE todos; --": "pwned"})
	if err == nil {
		t.Fatal("expected error for disallowed column, got nil")
	}
}

func TestUpdateProjectRejectsBogusColumn(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateProject("proj", "")

	err := d.UpdateProject(id, map[string]any{"evil_col": "bad"})
	if err == nil {
		t.Fatal("expected error for disallowed column, got nil")
	}
}

// --- Ideas ---

func TestCreateAndListIdeas(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateIdea("cool idea", "some details", nil, []string{"go", "ai"})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	ideas, err := d.ListIdeas(nil, "")
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 1 {
		t.Fatalf("expected 1 idea, got %d", len(ideas))
	}
	if ideas[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, ideas[0].ID)
	}
	if ideas[0].Title != "cool idea" {
		t.Errorf("expected title %q, got %q", "cool idea", ideas[0].Title)
	}
	if len(ideas[0].Tags) != 2 || ideas[0].Tags[0] != "go" || ideas[0].Tags[1] != "ai" {
		t.Errorf("expected tags [go, ai], got %v", ideas[0].Tags)
	}
}

func TestListIdeasFilterByTag(t *testing.T) {
	d := openTestDB(t)

	d.CreateIdea("idea 1", "", nil, []string{"go", "ai"})
	d.CreateIdea("idea 2", "", nil, []string{"rust"})
	d.CreateIdea("idea 3", "", nil, nil)

	ideas, err := d.ListIdeas(nil, "go")
	if err != nil {
		t.Fatalf("ListIdeas(tag=go): %v", err)
	}
	if len(ideas) != 1 {
		t.Fatalf("expected 1 idea with tag 'go', got %d", len(ideas))
	}
	if ideas[0].Title != "idea 1" {
		t.Errorf("expected %q, got %q", "idea 1", ideas[0].Title)
	}
}

func TestListIdeasFilterByProject(t *testing.T) {
	d := openTestDB(t)

	pid, _ := d.CreateProject("proj", "")
	d.CreateIdea("linked idea", "", &pid, nil)
	d.CreateIdea("unlinked idea", "", nil, nil)

	ideas, err := d.ListIdeas(&pid, "")
	if err != nil {
		t.Fatalf("ListIdeas(project): %v", err)
	}
	if len(ideas) != 1 {
		t.Fatalf("expected 1 idea for project, got %d", len(ideas))
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

	d.CreateProject("active proj", "")
	pid2, _ := d.CreateProject("archived proj", "")
	d.UpdateProject(pid2, map[string]any{"status": "archived"})

	d.CreateTodo("pending task", nil, "", "", "")
	d.CreateTodo("overdue task", nil, "", "", "2020-01-01")
	id3, _ := d.CreateTodo("done task", nil, "", "", "")
	d.CompleteTodo(id3)

	s, err := d.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if s.ActiveProjects != 1 {
		t.Errorf("expected 1 active project, got %d", s.ActiveProjects)
	}
	if s.PendingTodos != 2 {
		t.Errorf("expected 2 pending todos, got %d", s.PendingTodos)
	}
	if len(s.OverdueTodos) != 1 {
		t.Errorf("expected 1 overdue todo, got %d", len(s.OverdueTodos))
	}
	if len(s.RecentTodos) != 3 {
		t.Errorf("expected 3 recent todos, got %d", len(s.RecentTodos))
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

func TestSearchMemoriesByProject(t *testing.T) {
	d := openTestDB(t)

	pid, _ := d.CreateProject("proj", "")
	d.SaveMemory("project memory", "observation", "agent", nil, &pid, "")
	d.SaveMemory("general memory", "observation", "agent", nil, nil, "")

	results, err := d.SearchMemories("", "", "", &pid, "", 10)
	if err != nil {
		t.Fatalf("SearchMemories(project): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for project, got %d", len(results))
	}
	if results[0].Content != "project memory" {
		t.Errorf("expected %q, got %q", "project memory", results[0].Content)
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
		t.Fatalf("expected 1 idea, got %d", len(skills))
	}
	if skills[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, skills[0].ID)
	}
	if skills[0].Name != "cool skill" {
		t.Errorf("expected title %q, got %q", "cool idea", skills[0].Name)
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
