package db

import "testing"

// positionOf returns a thing's current position.
func positionOf(t *testing.T, d *DB, id int64) int64 {
	t.Helper()
	var pos int64
	if err := d.conn.QueryRow("SELECT position FROM things WHERE id = ?", id).Scan(&pos); err != nil {
		t.Fatalf("reading position of %d: %v", id, err)
	}
	return pos
}

func statusOf(t *testing.T, d *DB, id int64) string {
	t.Helper()
	var s string
	if err := d.conn.QueryRow("SELECT status FROM things WHERE id = ?", id).Scan(&s); err != nil {
		t.Fatalf("reading status of %d: %v", id, err)
	}
	return s
}

func completedAtOf(t *testing.T, d *DB, id int64) string {
	t.Helper()
	var c string
	if err := d.conn.QueryRow("SELECT COALESCE(completed_at,'') FROM things WHERE id = ?", id).Scan(&c); err != nil {
		t.Fatalf("reading completed_at of %d: %v", id, err)
	}
	return c
}

// seedOpen creates n open things and returns their ids in creation order. Each
// gets a successively larger position from CreateThing's bottom-placement.
func seedOpen(t *testing.T, d *DB, n int) []int64 {
	t.Helper()
	var ids []int64
	for range n {
		id, err := d.CreateThing("t", "", "normal", "", nil)
		if err != nil {
			t.Fatalf("CreateThing: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestNewThingGetsBottomPosition(t *testing.T) {
	d := openTestDB(t)
	ids := seedOpen(t, d, 3)
	a, b, c := positionOf(t, d, ids[0]), positionOf(t, d, ids[1]), positionOf(t, d, ids[2])
	if !(a < b && b < c) {
		t.Fatalf("expected increasing positions, got %d, %d, %d", a, b, c)
	}
}

func TestMoveWithinColumnUpdatesOnlyMovedCard(t *testing.T) {
	d := openTestDB(t)
	ids := seedOpen(t, d, 3) // visual order: ids[0], ids[1], ids[2]
	before0, before2 := positionOf(t, d, ids[0]), positionOf(t, d, ids[2])

	// Move ids[2] to between ids[0] and ids[1].
	if err := d.MoveThing(ids[2], "open", ids[0], ids[1]); err != nil {
		t.Fatalf("MoveThing: %v", err)
	}

	// Untouched neighbors keep their positions.
	if positionOf(t, d, ids[0]) != before0 {
		t.Errorf("ids[0] position changed: %d -> %d", before0, positionOf(t, d, ids[0]))
	}
	// Moved card now sits between ids[0] and ids[1].
	movedPos := positionOf(t, d, ids[2])
	if !(positionOf(t, d, ids[0]) < movedPos && movedPos < positionOf(t, d, ids[1])) {
		t.Errorf("moved card not between neighbors: %d", movedPos)
	}
	if before2 == movedPos {
		t.Error("expected moved card position to change")
	}
}

func TestMoveRenumbersWhenNoGap(t *testing.T) {
	d := openTestDB(t)
	ids := seedOpen(t, d, 3)
	// Force adjacent positions so there is no integer gap between ids[0] and ids[1].
	mustExec(t, d, "UPDATE things SET position = 10 WHERE id = ?", ids[0])
	mustExec(t, d, "UPDATE things SET position = 11 WHERE id = ?", ids[1])
	mustExec(t, d, "UPDATE things SET position = 12 WHERE id = ?", ids[2])

	if err := d.MoveThing(ids[2], "open", ids[0], ids[1]); err != nil {
		t.Fatalf("MoveThing: %v", err)
	}
	// After the renumber fallback, the moved card lands between its neighbors.
	p0, p1, p2 := positionOf(t, d, ids[0]), positionOf(t, d, ids[1]), positionOf(t, d, ids[2])
	if !(p0 < p2 && p2 < p1) {
		t.Errorf("after renumber, expected %d < %d < %d", p0, p2, p1)
	}
}

func TestCrossColumnMoveSetsStatusAndCompletion(t *testing.T) {
	d := openTestDB(t)
	ids := seedOpen(t, d, 1)
	id := ids[0]

	// open -> done: stamps completed_at.
	if err := d.MoveThing(id, "done", 0, 0); err != nil {
		t.Fatalf("MoveThing to done: %v", err)
	}
	if statusOf(t, d, id) != "done" {
		t.Errorf("status = %q, want done", statusOf(t, d, id))
	}
	if completedAtOf(t, d, id) == "" {
		t.Error("expected completed_at to be stamped on move into Done")
	}

	// done -> active: clears completed_at.
	if err := d.MoveThing(id, "active", 0, 0); err != nil {
		t.Fatalf("MoveThing to active: %v", err)
	}
	if statusOf(t, d, id) != "active" {
		t.Errorf("status = %q, want active", statusOf(t, d, id))
	}
	if completedAtOf(t, d, id) != "" {
		t.Error("expected completed_at cleared on move out of Done")
	}
}

func TestThingsByStatusOrdersByPosition(t *testing.T) {
	d := openTestDB(t)
	ids := seedOpen(t, d, 3)
	// Reverse the visual order via positions.
	mustExec(t, d, "UPDATE things SET position = 300 WHERE id = ?", ids[0])
	mustExec(t, d, "UPDATE things SET position = 200 WHERE id = ?", ids[1])
	mustExec(t, d, "UPDATE things SET position = 100 WHERE id = ?", ids[2])

	got, err := d.ThingsByStatus("open")
	if err != nil {
		t.Fatalf("ThingsByStatus: %v", err)
	}
	want := []int64{ids[2], ids[1], ids[0]}
	if len(got) != len(want) {
		t.Fatalf("got %d things, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("position %d: got id %d, want %d", i, got[i].ID, w)
		}
	}
}

func mustExec(t *testing.T, d *DB, query string, args ...any) {
	t.Helper()
	if _, err := d.conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
