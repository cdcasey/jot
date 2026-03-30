package db

import (
	"testing"
)

// --- Watches ---

func TestCreateAndListWatches(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateWatch("austin-theatre", "Extract auditions", []string{"https://example.com/auditions"}, "0 9 * * 1")
	if err != nil {
		t.Fatalf("CreateWatch: %v", err)
	}

	watches, err := d.ListWatches(false)
	if err != nil {
		t.Fatalf("ListWatches: %v", err)
	}
	if len(watches) != 1 {
		t.Fatalf("expected 1 watch, got %d", len(watches))
	}
	w := watches[0]
	if w.ID != id {
		t.Errorf("expected ID %d, got %d", id, w.ID)
	}
	if w.Name != "austin-theatre" {
		t.Errorf("expected name %q, got %q", "austin-theatre", w.Name)
	}
	if len(w.URLs) != 1 || w.URLs[0] != "https://example.com/auditions" {
		t.Errorf("unexpected URLs: %v", w.URLs)
	}
	if w.CronExpr != "0 9 * * 1" {
		t.Errorf("expected cron %q, got %q", "0 9 * * 1", w.CronExpr)
	}
	if !w.Enabled {
		t.Error("expected watch to be enabled by default")
	}
}

func TestCreateWatchDuplicateName(t *testing.T) {
	d := openTestDB(t)

	_, err := d.CreateWatch("dupe", "prompt", nil, "")
	if err != nil {
		t.Fatalf("first CreateWatch: %v", err)
	}
	_, err = d.CreateWatch("dupe", "other", nil, "")
	if err == nil {
		t.Error("expected error on duplicate watch name, got nil")
	}
}

func TestGetWatchByName(t *testing.T) {
	d := openTestDB(t)

	_, err := d.CreateWatch("my-watch", "extract stuff", []string{"https://a.com", "https://b.com"}, "")
	if err != nil {
		t.Fatalf("CreateWatch: %v", err)
	}

	w, err := d.GetWatchByName("my-watch")
	if err != nil {
		t.Fatalf("GetWatchByName: %v", err)
	}
	if w == nil {
		t.Fatal("expected watch, got nil")
	}
	if len(w.URLs) != 2 {
		t.Errorf("expected 2 URLs, got %d", len(w.URLs))
	}

	// Not found
	w, err = d.GetWatchByName("nope")
	if err != nil {
		t.Fatalf("GetWatchByName(missing): %v", err)
	}
	if w != nil {
		t.Error("expected nil for missing watch")
	}
}

func TestUpdateWatch(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateWatch("updatable", "old prompt", nil, "")

	err := d.UpdateWatch(id, map[string]any{"prompt": "new prompt", "enabled": 0})
	if err != nil {
		t.Fatalf("UpdateWatch: %v", err)
	}

	w, _ := d.GetWatchByName("updatable")
	if w.Prompt != "new prompt" {
		t.Errorf("expected updated prompt, got %q", w.Prompt)
	}
	if w.Enabled {
		t.Error("expected watch to be disabled")
	}
}

func TestUpdateWatchDisallowedColumn(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateWatch("guarded", "prompt", nil, "")
	err := d.UpdateWatch(id, map[string]any{"name": "hacked"})
	if err == nil {
		t.Error("expected error on disallowed column, got nil")
	}
}

func TestDeleteWatch(t *testing.T) {
	d := openTestDB(t)

	_, _ = d.CreateWatch("doomed", "prompt", nil, "")
	err := d.DeleteWatch("doomed")
	if err != nil {
		t.Fatalf("DeleteWatch: %v", err)
	}

	w, _ := d.GetWatchByName("doomed")
	if w != nil {
		t.Error("expected watch to be deleted")
	}
}

func TestRecordWatchRun(t *testing.T) {
	d := openTestDB(t)

	id, _ := d.CreateWatch("runner", "prompt", nil, "0 9 * * *")
	err := d.RecordWatchRun(id)
	if err != nil {
		t.Fatalf("RecordWatchRun: %v", err)
	}

	w, _ := d.GetWatchByName("runner")
	if w.LastRun == "" {
		t.Error("expected last_run to be set")
	}
}

func TestSaveWatchResultDedup(t *testing.T) {
	d := openTestDB(t)

	watchID, _ := d.CreateWatch("dedup-test", "prompt", nil, "")

	// First insert — should succeed
	id1, err := d.SaveWatchResult(watchID, "abc123", "Hamlet Auditions", "Open call Friday", "https://example.com")
	if err != nil {
		t.Fatalf("first SaveWatchResult: %v", err)
	}
	if id1 == 0 {
		t.Error("expected non-zero ID on first insert")
	}

	// Duplicate — same watch + hash
	id2, err := d.SaveWatchResult(watchID, "abc123", "Hamlet Auditions", "Open call Friday", "https://example.com")
	if err != nil {
		t.Fatalf("duplicate SaveWatchResult: %v", err)
	}
	if id2 != 0 {
		t.Errorf("expected 0 ID on duplicate, got %d", id2)
	}

	// Different hash — should succeed
	id3, err := d.SaveWatchResult(watchID, "def456", "Macbeth Auditions", "Callbacks Monday", "https://example.com")
	if err != nil {
		t.Fatalf("second SaveWatchResult: %v", err)
	}
	if id3 == 0 {
		t.Error("expected non-zero ID on different hash")
	}
}

func TestListWatchResultsUnnotified(t *testing.T) {
	d := openTestDB(t)

	watchID, _ := d.CreateWatch("notify-test", "prompt", nil, "")
	d.SaveWatchResult(watchID, "aaa", "Result A", "", "")
	d.SaveWatchResult(watchID, "bbb", "Result B", "", "")

	// Both unnotified
	results, err := d.ListWatchResults(watchID, true, 10)
	if err != nil {
		t.Fatalf("ListWatchResults: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 unnotified results, got %d", len(results))
	}

	// Mark one notified
	err = d.MarkResultsNotified([]int64{results[0].ID})
	if err != nil {
		t.Fatalf("MarkResultsNotified: %v", err)
	}

	// Only one unnotified now
	results, err = d.ListWatchResults(watchID, true, 10)
	if err != nil {
		t.Fatalf("ListWatchResults after mark: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 unnotified result, got %d", len(results))
	}

	// All results (notified + unnotified)
	all, err := d.ListWatchResults(watchID, false, 10)
	if err != nil {
		t.Fatalf("ListWatchResults(all): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 total results, got %d", len(all))
	}
}

func TestDeleteWatchCascadesResults(t *testing.T) {
	d := openTestDB(t)

	watchID, _ := d.CreateWatch("cascade-test", "prompt", nil, "")
	d.SaveWatchResult(watchID, "aaa", "Result", "", "")

	d.DeleteWatch("cascade-test")

	// Results should be gone too (foreign key cascade)
	results, err := d.ListWatchResults(watchID, false, 10)
	if err != nil {
		t.Fatalf("ListWatchResults after cascade: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after cascade delete, got %d", len(results))
	}
}

func TestListWatchesEnabledOnly(t *testing.T) {
	d := openTestDB(t)

	_, _ = d.CreateWatch("enabled-one", "prompt", nil, "")
	id2, _ := d.CreateWatch("disabled-one", "prompt", nil, "")
	d.UpdateWatch(id2, map[string]any{"enabled": 0})

	all, _ := d.ListWatches(false)
	if len(all) != 2 {
		t.Errorf("expected 2 total watches, got %d", len(all))
	}

	enabled, _ := d.ListWatches(true)
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled watch, got %d", len(enabled))
	}
	if enabled[0].Name != "enabled-one" {
		t.Errorf("expected enabled watch name %q, got %q", "enabled-one", enabled[0].Name)
	}
}

func TestPruneOldWatchResults(t *testing.T) {
	d := openTestDB(t)

	watchID, _ := d.CreateWatch("prune-test", "prompt", nil, "")

	// Insert a result, then backdate it to 200 days ago.
	d.SaveWatchResult(watchID, "old-hash", "Old Result", "", "")
	d.conn.Exec(
		"UPDATE watch_results SET first_seen = datetime('now', '-200 days') WHERE content_hash = 'old-hash'",
	)

	// Insert a recent result.
	d.SaveWatchResult(watchID, "new-hash", "New Result", "", "")

	// Prune at 180 days — should remove the old one.
	n, err := d.PruneOldWatchResults(180)
	if err != nil {
		t.Fatalf("PruneOldWatchResults: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 pruned, got %d", n)
	}

	// Only the new result should remain.
	results, _ := d.ListWatchResults(watchID, false, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 remaining result, got %d", len(results))
	}
	if results[0].Title != "New Result" {
		t.Errorf("expected New Result to survive, got %q", results[0].Title)
	}
}
