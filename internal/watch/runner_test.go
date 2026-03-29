package watch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/llm"
)

// mockLLM implements llm.Client for testing.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Chat(_ context.Context, _ string, _ []llm.Message, _ []llm.Tool) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.Response{Content: m.response}, nil
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestRunWatchNewItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<h2>Hamlet - Austin Playhouse - March 15</h2>
			<h2>Macbeth - City Theatre - April 1</h2>
		</body></html>`))
	}))
	defer srv.Close()

	d := openTestDB(t)
	watchID, _ := d.CreateWatch("test-watch", "Extract auditions", []string{srv.URL}, "")

	w, _ := d.GetWatchByName("test-watch")

	mock := &mockLLM{response: fmt.Sprintf(`[
		{"title": "Hamlet - Austin Playhouse", "body": "Auditions March 15", "source_url": "%s"},
		{"title": "Macbeth - City Theatre", "body": "Auditions April 1", "source_url": "%s"}
	]`, srv.URL, srv.URL)}

	runner := NewRunner(d, mock)
	results, err := runner.RunWatch(context.Background(), *w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 new results, got %d", len(results))
	}
	if results[0].Title != "Hamlet - Austin Playhouse" {
		t.Errorf("unexpected title: %s", results[0].Title)
	}

	// Verify they're in the DB
	stored, _ := d.ListWatchResults(watchID, false, 10)
	if len(stored) != 2 {
		t.Errorf("expected 2 stored results, got %d", len(stored))
	}

	// Verify last_run was recorded
	w, _ = d.GetWatchByName("test-watch")
	if w.LastRun == "" {
		t.Error("expected last_run to be set")
	}
}

func TestRunWatchDedup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>Hamlet auditions</p>`))
	}))
	defer srv.Close()

	d := openTestDB(t)
	d.CreateWatch("dedup-watch", "Extract auditions", []string{srv.URL}, "")
	w, _ := d.GetWatchByName("dedup-watch")

	mock := &mockLLM{response: `[{"title": "Hamlet", "body": "Open call", "source_url": ""}]`}
	runner := NewRunner(d, mock)

	// First run — should find 1 new item
	results1, err := runner.RunWatch(context.Background(), *w)
	if err != nil {
		t.Fatalf("first RunWatch: %v", err)
	}
	if len(results1) != 1 {
		t.Fatalf("expected 1 new result, got %d", len(results1))
	}

	// Second run — same item, should be deduped
	results2, err := runner.RunWatch(context.Background(), *w)
	if err != nil {
		t.Fatalf("second RunWatch: %v", err)
	}
	if len(results2) != 0 {
		t.Errorf("expected 0 new results on re-run, got %d", len(results2))
	}
}

func TestRunWatchNoURLs(t *testing.T) {
	d := openTestDB(t)
	w := db.Watch{ID: 1, Name: "empty"}

	runner := NewRunner(d, &mockLLM{})
	_, err := runner.RunWatch(context.Background(), w)
	if err == nil {
		t.Error("expected error for watch with no URLs")
	}
}

func TestRunWatchAllFetchesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := openTestDB(t)
	d.CreateWatch("fail-watch", "Extract stuff", []string{srv.URL}, "")
	w, _ := d.GetWatchByName("fail-watch")

	runner := NewRunner(d, &mockLLM{})
	_, err := runner.RunWatch(context.Background(), *w)
	if err == nil {
		t.Error("expected error when all fetches fail")
	}
}

func TestRunWatchLLMReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>No auditions listed</p>`))
	}))
	defer srv.Close()

	d := openTestDB(t)
	d.CreateWatch("empty-watch", "Extract auditions", []string{srv.URL}, "")
	w, _ := d.GetWatchByName("empty-watch")

	mock := &mockLLM{response: `[]`}
	runner := NewRunner(d, mock)

	results, err := runner.RunWatch(context.Background(), *w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
}

func TestRunWatchLLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>content</p>`))
	}))
	defer srv.Close()

	d := openTestDB(t)
	d.CreateWatch("llm-fail", "Extract stuff", []string{srv.URL}, "")
	w, _ := d.GetWatchByName("llm-fail")

	mock := &mockLLM{err: fmt.Errorf("rate limited")}
	runner := NewRunner(d, mock)

	_, err := runner.RunWatch(context.Background(), *w)
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestRunWatchMarkdownFences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>auditions</p>`))
	}))
	defer srv.Close()

	d := openTestDB(t)
	d.CreateWatch("fence-watch", "Extract auditions", []string{srv.URL}, "")
	w, _ := d.GetWatchByName("fence-watch")

	// LLM wraps response in markdown code fences
	mock := &mockLLM{response: "```json\n[{\"title\": \"Hamlet\", \"body\": \"Open call\", \"source_url\": \"\"}]\n```"}
	runner := NewRunner(d, mock)

	results, err := runner.RunWatch(context.Background(), *w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result despite markdown fences, got %d", len(results))
	}
}

func TestContentHash(t *testing.T) {
	// Same title with different casing/spacing should produce same hash
	h1 := contentHash("Hamlet Auditions")
	h2 := contentHash("  hamlet auditions  ")
	if h1 != h2 {
		t.Errorf("expected same hash for normalized titles, got %s vs %s", h1, h2)
	}

	// Different titles should produce different hashes
	h3 := contentHash("Macbeth Auditions")
	if h1 == h3 {
		t.Error("expected different hashes for different titles")
	}
}

func TestParseExtractedItems(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"valid JSON", `[{"title":"A","body":"B","source_url":"C"}]`, 1, false},
		{"empty array", `[]`, 0, false},
		{"with fences", "```json\n[{\"title\":\"A\",\"body\":\"\",\"source_url\":\"\"}]\n```", 1, false},
		{"invalid JSON", `not json`, 0, true},
		{"multiple items", `[{"title":"A","body":"","source_url":""},{"title":"B","body":"","source_url":""}]`, 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parseExtractedItems(tt.input)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(items) != tt.want {
				t.Errorf("expected %d items, got %d", tt.want, len(items))
			}
		})
	}
}
