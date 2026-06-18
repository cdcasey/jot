## Why

Jot is reorienting to a web-primary, structured tool. Discord is great for quick capture and push notifications, but it is a poor surface for *looking at* and *organizing* what you're tracking. A board for ideas and projects, plus a grid for habits, is where the value of the structured tables actually becomes visible. Server-rendered Go keeps the single-binary, SQLite-only shape intact — no Node, no build step, no separate service.

## What Changes

- Add an embedded web layer to the existing binary: an `internal/web` package, served behind a `WEB_PORT` env var, reading and writing the same SQLite DB. No separate service, no Node, no build step.
- Add a **three-column kanban** over `things`: `open` = Ideas, `active` = Active, `done` = Done. `dropped` is treated as archived and filtered off the board (reachable, but not shown as a column).
- Add a **per-habit calendar grid** over `habit_logs`: a cell is filled where a `habit_logs` row exists for that `(habit, date)`, empty otherwise.
- Add **manual ordering** to `things` via a new `position` column. Order is persisted on each drag so a card stays where it is dropped.
- Dragging a card **across columns** writes `status`, bumps `updated_at`, and stamps `completed_at` when dropped into Done; dragging **within a column** persists order only.
- Drag-and-drop uses htmx + Sortable.js against server endpoints; the habit grid may be read-only or click-to-toggle (resolved in design).

## Capabilities

### New Capabilities

- `web-ui`: An embedded, server-rendered web interface over the existing SQLite tables. Covers the kanban board over `things` (column mapping, ordering, cross-column moves), the habit calendar grid over `habit_logs`, manual `position` ordering of things, and the serving/exposure model (`WEB_PORT`, sandbox identity, no new external access).

### Modified Capabilities

<!-- None. There is no existing `things` capability spec in openspec/specs/ to modify;
     the `position` column and its drag-persistence behavior are introduced as
     requirements inside the new `web-ui` capability. The habit log write path is
     already owned by the existing `habit-tracking` spec and is cross-referenced,
     not redefined. -->

## Impact

- **New code**: `internal/web` package (HTTP handlers, server-rendered templates, static assets for htmx + Sortable.js). New query helpers in `internal/db` for position-ordered reads of `things` and grid reads of `habit_logs`.
- **Schema**: `things` gains a `position` (sort_order) column, with an idempotent `migrate()` step for existing DBs. Position must be backfilled for existing rows.
- **Config**: new `WEB_PORT` env var; web server started from `cmd/agent/main.go` alongside the Discord bot and scheduler.
- **Dependencies**: depends on the foundational web layer (proposal 1) and the already-merged habit tables (proposal 2 / `habit-tracking` spec). No CGO; the pure-Go `modernc.org/sqlite` driver is retained. htmx and Sortable.js are vendored static assets (no npm).
- **Non-goals**: charts and calendar *views* are out of MVP scope; no React/SPA/build step. A React island is reserved strictly for possible future interactive charts — it is not the foundation.
