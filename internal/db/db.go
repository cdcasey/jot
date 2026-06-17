package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("running data migrations: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

// migrate handles data migrations for existing databases (idempotent).
func (d *DB) migrate() error {
	// Add fire_at/fired columns to schedules if missing (pre-simplification DBs).
	if !d.columnExists("schedules", "fire_at") {
		if _, err := d.conn.Exec(`ALTER TABLE schedules ADD COLUMN fire_at TEXT`); err != nil {
			return fmt.Errorf("adding fire_at to schedules: %w", err)
		}
		if _, err := d.conn.Exec(`ALTER TABLE schedules ADD COLUMN fired INTEGER DEFAULT 0`); err != nil {
			return fmt.Errorf("adding fired to schedules: %w", err)
		}
	}

	// Migrate reminders → one-shot schedules.
	if d.tableExists("reminders") {
		if _, err := d.conn.Exec(`INSERT INTO schedules (name, cron_expr, prompt, fire_at, fired, enabled)
			SELECT 'reminder-' || id, '', prompt, fire_at, fired, 1 FROM reminders`); err != nil {
			return fmt.Errorf("migrating reminders: %w", err)
		}
	}

	// Add updated_at to watches if missing (added after initial watch schema).
	if d.tableExists("watches") && !d.columnExists("watches", "updated_at") {
		if _, err := d.conn.Exec(`ALTER TABLE watches ADD COLUMN updated_at TEXT DEFAULT (datetime('now'))`); err != nil {
			return fmt.Errorf("adding updated_at to watches: %w", err)
		}
	}

	// Drop memory triggers and FTS table before the base memories table.
	for _, trigger := range []string{"memories_ai", "memories_ad", "memories_au"} {
		if _, err := d.conn.Exec("DROP TRIGGER IF EXISTS " + trigger); err != nil {
			return fmt.Errorf("dropping trigger %s: %w", trigger, err)
		}
	}
	if _, err := d.conn.Exec("DROP TABLE IF EXISTS memories_fts"); err != nil {
		return fmt.Errorf("dropping memories_fts: %w", err)
	}

	// Drop removed tables.
	for _, table := range []string{"check_ins", "skills", "reminders", "habit_logs", "memories", "conversations", "conversation_summaries"} {
		if _, err := d.conn.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("dropping %s: %w", table, err)
		}
	}

	return nil
}

func (d *DB) tableExists(name string) bool {
	var n int
	d.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return n > 0
}

func (d *DB) columnExists(table, column string) bool {
	rows, err := d.conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}
