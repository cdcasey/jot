package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chris/jot/internal/llm"
)

type ConversationSummary struct {
	ID           int64  `json:"id"`
	UserID       string `json:"user_id"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"message_count"`
	CreatedAt    string `json:"created_at"`
}

// LoadConversation returns the stored messages and last_message_at for a user.
// Returns empty slice and zero time if no row exists.
func (d *DB) LoadConversation(userID string) ([]llm.Message, time.Time, error) {
	var raw string
	var lastAt string
	err := d.conn.QueryRow(
		`SELECT messages, last_message_at FROM conversations WHERE user_id = ?`, userID,
	).Scan(&raw, &lastAt)
	if err != nil {
		// No row — return empty
		return nil, time.Time{}, nil
	}
	var msgs []llm.Message
	if err := json.Unmarshal([]byte(raw), &msgs); err != nil {
		return nil, time.Time{}, fmt.Errorf("unmarshaling conversation: %w", err)
	}
	t, _ := time.Parse("2006-01-02 15:04:05", lastAt)
	return msgs, t, nil
}

// SaveConversation upserts the conversation messages for a user.
func (d *DB) SaveConversation(userID string, messages []llm.Message) error {
	b, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshaling conversation: %w", err)
	}
	_, err = d.conn.Exec(`
		INSERT INTO conversations (user_id, messages, last_message_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(user_id) DO UPDATE SET
			messages = excluded.messages,
			last_message_at = excluded.last_message_at,
			updated_at = excluded.updated_at`,
		userID, string(b),
	)
	if err != nil {
		return fmt.Errorf("saving conversation: %w", err)
	}
	return nil
}

// ClearConversation resets the messages for a user to an empty array.
func (d *DB) ClearConversation(userID string) error {
	_, err := d.conn.Exec(`
		UPDATE conversations SET messages = '[]', updated_at = datetime('now')
		WHERE user_id = ?`, userID,
	)
	return err
}

// SaveConversationSummary stores a summarized conversation.
func (d *DB) SaveConversationSummary(userID, summary string, msgCount int) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO conversation_summaries (user_id, summary, message_count)
		VALUES (?, ?, ?)`,
		userID, summary, msgCount,
	)
	if err != nil {
		return 0, fmt.Errorf("saving conversation summary: %w", err)
	}
	return res.LastInsertId()
}

// GetRecentSummaries returns the most recent summaries for a user, newest first.
func (d *DB) GetRecentSummaries(userID string, limit int) ([]ConversationSummary, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := d.conn.Query(`
		SELECT id, user_id, summary, message_count, created_at
		FROM conversation_summaries
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying summaries: %w", err)
	}
	defer rows.Close()
	var summaries []ConversationSummary
	for rows.Next() {
		var s ConversationSummary
		if err := rows.Scan(&s.ID, &s.UserID, &s.Summary, &s.MessageCount, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// PruneOldSummaries deletes summaries older than the given number of days.
func (d *DB) PruneOldSummaries(olderThanDays int) (int64, error) {
	res, err := d.conn.Exec(`
		DELETE FROM conversation_summaries
		WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", olderThanDays),
	)
	if err != nil {
		return 0, fmt.Errorf("pruning summaries: %w", err)
	}
	return res.RowsAffected()
}
