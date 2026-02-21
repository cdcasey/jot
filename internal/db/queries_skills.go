package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// CreateSkill creates a new skill and returns its ID.
func (d *DB) CreateSkill(name, description, content string, tags []string) (int64, error) {
	var tagsJSON string
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}
	res, err := d.conn.Exec(
		"INSERT INTO skills (name, description, content, tags) VALUES (?, ?, ?, ?)",
		name, description, content, nullStr(tagsJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("creating skill: %w", err)
	}
	return res.LastInsertId()
}

// GetSkill retrieves a skill by name.
func (d *DB) GetSkill(name string) (*Skill, error) {
	var s Skill
	var tagsJSON string
	err := d.conn.QueryRow("SELECT id, name, description, content, COALESCE(tags,'[]'), created_at, updated_at FROM skills WHERE name = ?", name).Scan(&s.ID, &s.Name, &s.Description, &s.Content, &tagsJSON, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting skill: %w", err)
	}
	_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
	return &s, nil
}

// ListSkills returns skills, optionally filtered by tags.
func (d *DB) ListSkills(tag string) ([]Skill, error) {
	query := "SELECT id, name, description, content, COALESCE(tags,'[]'), created_at, updated_at FROM skills WHERE 1=1"
	var args []any
	if tag != "" {
		query += " AND tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}
	query += " ORDER BY created_at DESC"
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	defer rows.Close()
	var skills []Skill
	for rows.Next() {
		var s Skill
		var tagsJSON string
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Content, &tagsJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning skill: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// UpdateSkill updates fields on a skill by name (names are unique in the skills schema).
func (d *DB) UpdateSkill(name string, fields map[string]any) error {
	skill, err := d.GetSkill(name)
	if err != nil {
		return fmt.Errorf("fetching skill for update: %w", err)
	}
	if skill == nil {
		return fmt.Errorf("no such skill exists: %s", name)
	}
	return d.updateRow("skills", skill.ID, fields)
}

// DeleteSkill deletes a skill by name
func (d *DB) DeleteSkill(name string) error {
	skill, err := d.GetSkill(name)
	if err != nil {
		return fmt.Errorf("fetching skill for delete: %w", err)
	}
	if skill == nil {
		return fmt.Errorf("no such skill exists: %s", name)
	}
	_, err = d.conn.Exec("DELETE from skills where id = ?", skill.ID)
	if err != nil {
		return fmt.Errorf("deleting skill: %w", err)
	}
	return nil
}
