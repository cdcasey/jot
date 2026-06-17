## Why

Habit tracking was removed in an earlier simplification in favor of `category='habit'` free-text memories. Free text could not enforce one entry per habit per day, so contradictory records for the same date accumulated and habit reports came out wrong. The fix is structural: restore habits as structured data with a uniqueness guarantee so the same `(habit, date)` can never hold two conflicting records.

This change deliberately separates **recording** from **judging**. The system only records whether a habit happened on a given day; it computes nothing about targets, cadence, streaks, or success. The judging is done by the user's own eyes against the visual grid (which lands in a separate web-app change).

## What Changes

- **ADDED** two SQLite tables:
  - `habits` — definitions (`id`, `name`, `active`, `created_at`).
  - `habit_logs` — daily entries (`id`, `habit_id`, `date`, `created_at`) with `UNIQUE(habit_id, date)`.
- **ADDED** one LLM tool, `log_habit`, that toggles a habit's entry for a local calendar date:
  - Logging inserts a row (no-op on conflict). Toggling off deletes the row.
  - If the named habit does not exist, it is auto-created (`active=1`) and then logged in the same call.
- Entry presence is the entire data model: a row at `(habit, date)` means "did"; absence means "did not".
- Date is the user's **local calendar date**, resolved through the existing `notes.timezone` / `userLocation()` path — no new timezone source is introduced.

Non-goals (explicitly out of scope): no streaks, targets, cadence, or success/failure logic; no visual grid (lives in the web-app change); no read-only HTTP or filesystem access — the SQLite-only sandbox identity is preserved.

## Capabilities

### New Capabilities
- `habit-tracking`: Structured, record-only habit logging — habit definitions, a uniqueness-guaranteed daily log, and a single toggle tool exposed to the LLM. The raw daily entries are the exact substrate a future judging/grid layer reads, with no migration required.

### Modified Capabilities
<!-- None. The `stateless-agent` capability is unaffected: log_habit is one more stateless tool call, persisting durable state to structured tables exactly like the existing thing/schedule/watch tools. -->

## Impact

- **Schema** (`internal/db/schema.sql`): two new tables. `internal/db/db.go` `migrate()` gains idempotent `CREATE TABLE IF NOT EXISTS` for existing DBs.
- **Queries** (`internal/db/`): new `queries_habits.go` (definitions, toggle/upsert, delete, range read) + struct types in `queries.go`. Reuses `nullStr`/`updateRow` helpers where applicable.
- **Tools** (`internal/llm/tools.go`): tool count 14 → 15. One new `log_habit` definition.
- **Tool execution** (`internal/agent/agent.go`): one new dispatch arm; reuses `userLocation()` for date resolution.
- **System prompt** (`internal/llm/prompt.go`): brief mention that habits are recorded (not judged) via `log_habit`.
- **Evals** (`eval/cases.json`): add cases for logging a new habit, toggling off, and idempotent re-log.
- **Docs** (`CLAUDE.md`): document the two tables and the new tool.
