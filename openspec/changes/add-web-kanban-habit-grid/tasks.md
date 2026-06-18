## 1. Schema & ordering

- [ ] 1.1 Add `position` column to `things` in `internal/db/schema.sql`
- [ ] 1.2 Add an idempotent `migrate()` step that adds `position` if missing (guard on column existence, matching existing migration style)
- [ ] 1.3 Backfill `position` for existing rows: per status, ordered deterministically (e.g. by `updated_at`/`id`), stepped with wide spacing (~1000)
- [ ] 1.4 Add a unit test verifying migration adds + backfills `position` and is idempotent on a second run

## 2. Position-aware thing queries

- [ ] 2.1 Add `internal/db` query: list `things` grouped by status, ordered by `position` ascending, excluding `dropped`
- [ ] 2.2 Add query/helper to set a single thing's `position` to a fractional value between two neighbors (single-row update)
- [ ] 2.3 Add a column renumber helper (restore wide spacing) used both as the no-gap fallback and by backfill
- [ ] 2.4 Add a cross-column move helper: set `status` + `position` + bump `updated_at`; stamp `completed_at` on move into Done, clear it on move out of Done — single operation
- [ ] 2.5 Table-driven tests for reorder (one row changes), no-gap renumber fallback, and cross-column move (status/position/completed_at transitions)

## 3. Habit grid reads

- [ ] 3.1 Add `internal/db` query that returns, per active habit, the set of `habit_logs` dates within the chosen window (read-only)
- [ ] 3.2 Add a helper that shapes the query result into grid rows (habit × date cells) for templating
- [ ] 3.3 Unit test: filled cell where a log exists, empty where none, and confirm no write path is invoked

## 4. Web layer — kanban

- [ ] 4.1 Add kanban `html/template` view: three columns (Ideas/Active/Done) rendering cards from the position-ordered query
- [ ] 4.2 Add htmx + Sortable.js wiring on the columns for drag-and-drop (vendored static assets, pinned versions, no npm)
- [ ] 4.3 Add handler for within-column reorder → call fractional-position helper (with renumber fallback)
- [ ] 4.4 Add handler for cross-column move → call cross-column move helper
- [ ] 4.5 Handlers return the updated card/column fragment for htmx swap

## 5. Web layer — habit grid

- [ ] 5.1 Add read-only habit grid `html/template` view rendering filled/empty cells from the grid query
- [ ] 5.2 Route the grid view; confirm it issues no write to `habit_logs`

## 6. Serving & exposure

- [ ] 6.1 Start the web server from `cmd/agent/main.go` only when `WEB_PORT` is set, sharing the existing DB handle
- [ ] 6.2 Bind to loopback/tailnet interface; no application-level auth in MVP
- [ ] 6.3 Confirm binary with no `WEB_PORT` set leaves CLI/Discord/scheduler behavior unchanged (no server started)
- [ ] 6.4 Document `WEB_PORT` in `config.example.yaml` / `.env` notes and update CLAUDE.md project structure for `internal/web`

## 7. Verification

- [ ] 7.1 `go test ./...` passes (migration, position, grid, move tests)
- [ ] 7.2 `go vet ./...` clean
- [ ] 7.3 Manual smoke: run with `WEB_PORT` set, verify board columns/order, drag within + across columns persists, Done stamps `completed_at`, grid reflects `habit_logs`
