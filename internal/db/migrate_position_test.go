package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateAddsAndBackfillsPosition simulates an existing database whose
// things table predates the position column, then runs migrate() and verifies
// the column is added, backfilled per status with wide spacing, and that a
// second migrate() is idempotent.
func TestMigrateAddsAndBackfillsPosition(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Build the current schema (so every table migrate() touches exists), then
	// drop the position column to simulate a database that predates it.
	mustRaw(t, conn, schema)
	mustRaw(t, conn, `ALTER TABLE things DROP COLUMN position`)
	if colExistsRaw(t, conn, "things", "position") {
		t.Fatal("setup failed: position column still present")
	}
	// Two open, one done — distinct updated_at so backfill order is deterministic.
	mustRaw(t, conn, `INSERT INTO things (title, status, updated_at) VALUES
		('a','open','2026-01-01 00:00:00'),
		('b','open','2026-01-02 00:00:00'),
		('c','done','2026-01-01 00:00:00')`)

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if !d.columnExists("things", "position") {
		t.Fatal("position column not added")
	}

	// Open column backfilled by (updated_at, id): a=step, b=2*step.
	a := positionByTitle(t, conn, "a")
	b := positionByTitle(t, conn, "b")
	c := positionByTitle(t, conn, "c")
	if a != positionStep || b != 2*positionStep {
		t.Errorf("open backfill: a=%d b=%d, want %d, %d", a, b, positionStep, 2*positionStep)
	}
	// Done column is partitioned independently, so c restarts at step.
	if c != positionStep {
		t.Errorf("done backfill: c=%d, want %d", c, positionStep)
	}

	// Idempotent: second run must not error or change positions.
	if err := d.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if positionByTitle(t, conn, "a") != a {
		t.Error("second migrate changed positions")
	}
}

func mustRaw(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func colExistsRaw(t *testing.T, conn *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

func positionByTitle(t *testing.T, conn *sql.DB, title string) int64 {
	t.Helper()
	var pos int64
	if err := conn.QueryRow("SELECT position FROM things WHERE title = ?", title).Scan(&pos); err != nil {
		t.Fatalf("position of %q: %v", title, err)
	}
	return pos
}
