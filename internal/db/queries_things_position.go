package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// positionStep is the spacing between adjacent things' positions. Wide spacing
// lets most reorders pick an integer midpoint without renumbering. When two
// neighbors end up adjacent (gap collapse), the column is renumbered to restore
// spacing — see renumberColumnTx.
const positionStep = 1000

// positionStepStr is positionStep as a string for inlining into SQL (the value
// is a trusted compile-time constant, never user input).
var positionStepStr = strconv.Itoa(positionStep)

// ThingsByStatus returns things with the given status ordered by position
// ascending. Used to render a single kanban column. status "dropped" is a valid
// argument (archived view); the board simply does not request it.
func (d *DB) ThingsByStatus(status string) ([]Thing, error) {
	query := `SELECT id, title, COALESCE(notes,''), status, priority,
		COALESCE(tags,'[]'), COALESCE(due_date,''), created_at, updated_at,
		COALESCE(completed_at,'') FROM things WHERE status = ? ORDER BY position ASC, id ASC`
	return d.scanThings(query, status)
}

// MoveThing places thing id into column targetStatus, positioned between the
// things afterID and beforeID (either may be 0 to mean "no neighbor on that
// side"). When targetStatus differs from the thing's current status it is a
// cross-column move: status and (un)completion are updated together with
// position in one transaction. When no integer fits between the neighbors, the
// target column is renumbered and the move retried.
func (d *DB) MoveThing(id int64, targetStatus string, afterID, beforeID int64) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning move tx: %w", err)
	}
	defer tx.Rollback()

	pos, err := positionBetweenTx(tx, targetStatus, afterID, beforeID, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.DateTime)
	if err := applyMoveTx(tx, id, targetStatus, pos, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing move: %w", err)
	}
	return nil
}

// positionBetweenTx computes a position for excludeID between afterID/beforeID
// within targetStatus, renumbering the column if no integer gap remains.
func positionBetweenTx(tx *sql.Tx, targetStatus string, afterID, beforeID, excludeID int64) (int64, error) {
	after, err := neighborPosTx(tx, afterID)
	if err != nil {
		return 0, err
	}
	before, err := neighborPosTx(tx, beforeID)
	if err != nil {
		return 0, err
	}

	switch {
	case afterID == 0 && beforeID == 0:
		// Empty column (or dropping as the only card): place at the end.
		max, err := maxPositionTx(tx, targetStatus, excludeID)
		if err != nil {
			return 0, err
		}
		return max + positionStep, nil
	case afterID == 0:
		// Dropped at the top.
		return before - positionStep, nil
	case beforeID == 0:
		// Dropped at the bottom.
		return after + positionStep, nil
	default:
		if before-after > 1 {
			return after + (before-after)/2, nil
		}
		// No gap: renumber the column, then recompute against fresh positions.
		if err := renumberColumnTx(tx, targetStatus); err != nil {
			return 0, err
		}
		return positionBetweenTx(tx, targetStatus, afterID, beforeID, excludeID)
	}
}

func neighborPosTx(tx *sql.Tx, id int64) (int64, error) {
	if id == 0 {
		return 0, nil
	}
	var pos int64
	if err := tx.QueryRow("SELECT position FROM things WHERE id = ?", id).Scan(&pos); err != nil {
		return 0, fmt.Errorf("reading neighbor %d position: %w", id, err)
	}
	return pos, nil
}

func maxPositionTx(tx *sql.Tx, status string, excludeID int64) (int64, error) {
	var max sql.NullInt64
	if err := tx.QueryRow(
		"SELECT MAX(position) FROM things WHERE status = ? AND id != ?", status, excludeID,
	).Scan(&max); err != nil {
		return 0, fmt.Errorf("reading max position for %s: %w", status, err)
	}
	return max.Int64, nil
}

// renumberColumnTx rewrites every position in a status column to restore wide
// spacing (positionStep, 2*positionStep, ...) in the current visual order. This
// is the shared fallback for gap collapse and is intentionally column-scoped.
func renumberColumnTx(tx *sql.Tx, status string) error {
	_, err := tx.Exec(`
		WITH ranked AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY position ASC, id ASC) AS rn
			FROM things WHERE status = ?
		)
		UPDATE things
		SET position = (SELECT rn FROM ranked WHERE ranked.id = things.id) * `+positionStepStr+`
		WHERE status = ?`,
		status, status,
	)
	if err != nil {
		return fmt.Errorf("renumbering column %s: %w", status, err)
	}
	return nil
}

// applyMoveTx writes status, position, updated_at, and (un)stamps completed_at
// based on whether the target column is "done".
func applyMoveTx(tx *sql.Tx, id int64, targetStatus string, pos int64, now string) error {
	var res sql.Result
	var err error
	if targetStatus == "done" {
		res, err = tx.Exec(
			"UPDATE things SET status = ?, position = ?, completed_at = ?, updated_at = ? WHERE id = ?",
			targetStatus, pos, now, now, id,
		)
	} else {
		res, err = tx.Exec(
			"UPDATE things SET status = ?, position = ?, completed_at = NULL, updated_at = ? WHERE id = ?",
			targetStatus, pos, now, id,
		)
	}
	if err != nil {
		return fmt.Errorf("moving thing %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("thing %d not found", id)
	}
	return nil
}
