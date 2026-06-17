## Context

Habit tracking previously lived as `category='habit'` free-text memories. That
representation had no integrity constraint, so multiple contradictory records
for the same habit and date could accumulate, and reports derived from them were
wrong. The memory layer has since been stripped entirely (Phase 7); Jot is now a
stateless command surface whose durable state lives in structured tables
(`things`, `schedules`, `watches`/`watch_results`, `notes`).

This change reintroduces habits as structured data. The guiding principle is a
hard separation between **recording** and **judging**: the system records only
whether a habit happened on a day. Targets, cadence, streaks, and success are not
stored or computed — the user judges by looking at a grid (a later web-app change).

Constraints inherited from the project identity: SQLite-only, no filesystem or
shell access, pure-Go driver (`modernc.org/sqlite`), and tools are the only way
the LLM touches state.

## Goals / Non-Goals

**Goals:**
- Structurally prevent the original contradiction bug via `UNIQUE(habit_id, date)`.
- One LLM tool (`log_habit`) covering log, idempotent re-log, and toggle-off.
- Local-calendar-date semantics reusing the existing timezone path.
- Leave the raw daily entries as a migration-free substrate for a future grid.

**Non-Goals:**
- No streaks, targets, cadence, or success/failure logic.
- No visual grid or web rendering (separate web-app change).
- No new timezone mechanism, no new external dependency, no new sandbox capability.
- No `create_habit`/`delete_habit`/`list_habits` tools in this change — kept to one tool surface.

## Decisions

### Decision: Two tables, definitions separate from logs

`habits` (`id`, `name UNIQUE`, `active`, `created_at`) and `habit_logs`
(`id`, `habit_id` FK → habits ON DELETE CASCADE, `date`, `created_at`,
`UNIQUE(habit_id, date)`).

*Why:* Separating definitions from logs lets a habit exist before it is logged,
lets it be renamed without rewriting log rows, and makes the future grid a simple
join. *Alternative considered:* a single denormalized table keyed by `(name, date)`
— rejected because renaming a habit would orphan or require rewriting every log
row, and a typo'd name would be indistinguishable from a real habit.

### Decision: `UNIQUE(habit_id, date)` with no-op-on-conflict insert

Logging uses `INSERT ... ON CONFLICT(habit_id, date) DO NOTHING`. The constraint
is the integrity guarantee; the conflict clause makes re-logging a harmless no-op.

*Why:* This is the structural fix for the original bug — the database, not
application code, makes a second contradictory row impossible. *Alternative
considered:* check-then-insert in Go — rejected as racy and as re-locating the
invariant into application code where it can drift.

### Decision: `log_habit` auto-creates an unknown habit, then logs

When the named habit is absent, `log_habit` inserts it (`active=1`) and logs the
entry in the same call.

*Why:* Keeps the tool surface at one tool (the seed's explicit intent) and
matches the existing name-resolving pattern of `update_schedule`/`update_watch`.
*Trade-off accepted:* a typo silently creates a near-duplicate habit
("meditate" vs "meditation"). Acceptable for a single-user sandbox where the user
sees the habit list; a future rename tool or fuzzy match can address it without
schema change. *Alternative considered:* require the habit to exist and add a
separate `create_habit` tool — rejected as more tools for no current benefit.

### Decision: Date is a local calendar date via `userLocation()`

The `(habit, date)` date is a local calendar `YYYY-MM-DD`, derived from the
user's timezone. Implementation reuses `agent.userLocation()`, which reads the
`notes.timezone` row and falls back to server-local time (confirmed at
`internal/agent/agent.go:411`). When the tool is called without an explicit date,
it uses `time.Now().In(userLocation())` formatted as `2006-01-02`.

*Why:* "Did I do it today?" is a human-calendar question; storing UTC dates would
misclassify late-evening or early-morning entries near the timezone boundary.
Reusing the existing path avoids a second source of truth for timezone.
*Alternative considered:* store UTC date — rejected for boundary errors.

### Decision: Add a range read query now, but expose no grid

`queries_habits.go` includes a "logs for a habit between two dates" query so the
later web-app change can render the grid with no schema or query migration. It is
not wired to any tool or HTTP surface in this change.

*Why:* The seed explicitly calls out the read the grid will need; adding it now
keeps the data layer complete and the future change additive-only.

## Risks / Trade-offs

- **Typo'd habit names create near-duplicates** (consequence of auto-create) →
  Mitigation: single-user sandbox; the habit list is visible; a future rename or
  fuzzy-resolve can fix without migration. Not solved here by design.
- **Timezone changes retroactively reinterpret stored dates** → Mitigation: dates
  are stored as already-resolved local calendar strings, not recomputed from
  timestamps, so a later timezone change does not rewrite history.
- **Auto-create hides "habit doesn't exist" from the user** → Mitigation: the tool
  result reports whether a new habit was created so the LLM can surface it.
- **Scope creep toward judging** → Mitigation: spec and design explicitly bar
  streaks/targets/cadence; the range query returns raw rows only.

## Migration Plan

- New tables only; no data backfill (the old `category='habit'` memories were
  removed with the memory layer and are not migrated).
- `schema.sql` defines both tables; `db.go migrate()` runs idempotent
  `CREATE TABLE IF NOT EXISTS` for existing databases.
- Rollback: drop the two tables; no other table references them, so removal is
  clean. The `ON DELETE CASCADE` on `habit_logs` means deleting a habit removes
  its logs.

## Open Questions

None. The two seed-level open choices are resolved above: `log_habit`
auto-creates unknown habits (confirmed with user), and the date source reuses
`notes.timezone` / `userLocation()` (confirmed against
`internal/agent/agent.go:411`).
