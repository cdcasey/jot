## 1. Schema & Migration

- [x] 1.1 Add `habits` table to `internal/db/schema.sql` (`id`, `name TEXT UNIQUE NOT NULL`, `active INTEGER DEFAULT 1`, `created_at TEXT DEFAULT (datetime('now'))`)
- [x] 1.2 Add `habit_logs` table to `schema.sql` (`id`, `habit_id INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE`, `date TEXT NOT NULL`, `created_at TEXT DEFAULT (datetime('now'))`, `UNIQUE(habit_id, date)`)
- [x] 1.3 Idempotent creation handled by `CREATE TABLE IF NOT EXISTS` in schema.sql (run on every `Open()`). **Also removed `habit_logs` from the `migrate()` DROP TABLE IF EXISTS loop** — leftover from the old removal that would have dropped the new table on every startup.

## 2. Queries

- [x] 2.1 Add `Habit` and `HabitLog` struct types to `internal/db/queries.go`
- [x] 2.2 Create `internal/db/queries_habits.go` with `GetOrCreateHabit(name)` returning the habit (auto-create with `active=1` if absent)
- [x] 2.3 Add `LogHabit(habitID, date)` using `INSERT ... ON CONFLICT(habit_id, date) DO NOTHING` (idempotent no-op on re-log)
- [x] 2.4 Add `UnlogHabit(habitID, date)` that deletes the row for `(habit_id, date)`, leaving the habit definition untouched
- [x] 2.5 Add `RenameHabit(habitID, newName)` (or reuse `updateRow`) verifying log rows are unaffected
- [x] 2.6 Add `HabitLogsInRange(habitID, start, end)` returning entries ordered by date (substrate for future grid; not tool-exposed)

## 3. LLM Tool

- [x] 3.1 Add `log_habit` tool definition to `internal/llm/tools.go` (params: `name` required; `date` optional local `YYYY-MM-DD`; `done` optional bool defaulting true for toggle direction) — tool count 14 → 15
- [x] 3.2 Add the `log_habit` dispatch arm in `internal/agent/agent.go`: resolve/auto-create habit, resolve date via `userLocation()` (default to current local date when omitted), then log or unlog
- [x] 3.3 Return a result that reports whether a new habit was created so the LLM can surface it
- [x] 3.4 Add a brief note to `internal/llm/prompt.go` that habits are recorded (not judged) via `log_habit`

## 4. Tests & Evals

- [x] 4.1 Table-driven unit tests in `queries_habits_test.go`: auto-create, log, idempotent re-log (one row), unlog, rename preserves logs, range read
- [x] 4.2 Add eval cases to `eval/cases.json`: log a new habit (asserts `log_habit` call), toggle off, idempotent re-log
- [x] 4.3 Run `go test ./...` and confirm green

## 5. Docs

- [x] 5.1 Update `CLAUDE.md`: document `habits` + `habit_logs` tables, the `log_habit` tool, and the updated tool count
- [x] 5.2 Update memory index / project notes if the tool count or table count is recorded there
