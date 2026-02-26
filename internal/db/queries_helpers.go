package db

import (
	"fmt"
	"strings"
)

var allowedColumns = map[string]map[string]bool{
	"things":   {"title": true, "notes": true, "status": true, "priority": true, "due_date": true, "tags": true, "completed_at": true},
	"skills":   {"name": true, "description": true, "content": true, "tags": true},
	"memories": {"content": true, "category": true, "tags": true, "expires_at": true},
}

// updateRow is a generic helper for updating a row's fields.
func (d *DB) updateRow(table string, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	allowed, ok := allowedColumns[table]
	if !ok {
		return fmt.Errorf("unknown table: %s", table)
	}
	var setClauses []string
	var args []any
	for col, val := range fields {
		if !allowed[col] {
			return fmt.Errorf("disallowed column %q for table %s", col, table)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(setClauses, ", "))
	res, err := d.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updating %s %d: %w", table, id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%s %d not found", table, id)
	}
	return nil
}

func nullStr(s string) any {
	if s == "" || s == "null" {
		return nil
	}
	return s
}
