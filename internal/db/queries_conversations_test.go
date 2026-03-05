package db

import (
	"testing"
	"time"

	"github.com/chris/jot/internal/llm"
)

func TestSaveAndLoadConversation(t *testing.T) {
	d := openTestDB(t)

	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	if err := d.SaveConversation("user1", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, lastAt, err := d.LoadConversation("user1")
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Role != "user" || loaded[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", loaded[0])
	}
	if loaded[1].Role != "assistant" || loaded[1].Content != "hi there" {
		t.Errorf("unexpected second message: %+v", loaded[1])
	}
	if lastAt.IsZero() {
		t.Error("expected non-zero last_message_at")
	}
}

func TestLoadConversationMissing(t *testing.T) {
	d := openTestDB(t)

	loaded, lastAt, err := d.LoadConversation("nonexistent")
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(loaded))
	}
	if !lastAt.IsZero() {
		t.Errorf("expected zero time, got %v", lastAt)
	}
}

func TestSaveConversationUpsert(t *testing.T) {
	d := openTestDB(t)

	msgs1 := []llm.Message{{Role: "user", Content: "first"}}
	msgs2 := []llm.Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
	}

	if err := d.SaveConversation("user1", msgs1); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := d.SaveConversation("user1", msgs2); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, _, err := d.LoadConversation("user1")
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages after upsert, got %d", len(loaded))
	}
}

func TestClearConversation(t *testing.T) {
	d := openTestDB(t)

	msgs := []llm.Message{{Role: "user", Content: "hello"}}
	if err := d.SaveConversation("user1", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}
	if err := d.ClearConversation("user1"); err != nil {
		t.Fatalf("ClearConversation: %v", err)
	}

	loaded, _, err := d.LoadConversation("user1")
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty after clear, got %d", len(loaded))
	}
}

func TestSaveAndGetSummaries(t *testing.T) {
	d := openTestDB(t)

	id, err := d.SaveConversationSummary("user1", "discussed groceries", 4)
	if err != nil {
		t.Fatalf("SaveConversationSummary: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	summaries, err := d.GetRecentSummaries("user1", 10)
	if err != nil {
		t.Fatalf("GetRecentSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Summary != "discussed groceries" {
		t.Errorf("unexpected summary: %s", summaries[0].Summary)
	}
	if summaries[0].MessageCount != 4 {
		t.Errorf("expected message_count 4, got %d", summaries[0].MessageCount)
	}
}

func TestGetRecentSummariesLimit(t *testing.T) {
	d := openTestDB(t)

	for i := range 5 {
		if _, err := d.SaveConversationSummary("user1", "summary", i); err != nil {
			t.Fatalf("SaveConversationSummary %d: %v", i, err)
		}
	}

	summaries, err := d.GetRecentSummaries("user1", 2)
	if err != nil {
		t.Fatalf("GetRecentSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestGetRecentSummariesDifferentUsers(t *testing.T) {
	d := openTestDB(t)

	d.SaveConversationSummary("user1", "user1 summary", 2)
	d.SaveConversationSummary("user2", "user2 summary", 3)

	s1, _ := d.GetRecentSummaries("user1", 10)
	s2, _ := d.GetRecentSummaries("user2", 10)
	if len(s1) != 1 || len(s2) != 1 {
		t.Errorf("expected 1 summary per user, got user1=%d user2=%d", len(s1), len(s2))
	}
}

func TestPruneOldSummaries(t *testing.T) {
	d := openTestDB(t)

	// Insert a summary with an old date
	d.conn.Exec(`INSERT INTO conversation_summaries (user_id, summary, message_count, created_at) VALUES (?, ?, ?, ?)`,
		"user1", "old summary", 5, time.Now().AddDate(0, 0, -60).Format("2006-01-02 15:04:05"))

	// Insert a recent summary
	d.SaveConversationSummary("user1", "recent summary", 3)

	deleted, err := d.PruneOldSummaries(30)
	if err != nil {
		t.Fatalf("PruneOldSummaries: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	remaining, _ := d.GetRecentSummaries("user1", 10)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
	if remaining[0].Summary != "recent summary" {
		t.Errorf("wrong summary survived: %s", remaining[0].Summary)
	}
}
