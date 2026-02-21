package db

import (
	"database/sql"
	"fmt"
)

// GetNote retrieves a note by key.
func (d *DB) GetNote(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM notes WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting note: %w", err)
	}
	return value, nil
}

// SetNote stores or updates a note by key.
func (d *DB) SetNote(key, value string) error {
	_, err := d.conn.Exec(
		"INSERT INTO notes (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')",
		key, value, value,
	)
	if err != nil {
		return fmt.Errorf("setting note: %w", err)
	}
	return nil
}
