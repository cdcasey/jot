## 1. Schema & ordering

- [x] 1.1 Add `position` column to `things` in `internal/db/schema.sql`
- [x] 1.2 Add an idempotent `migrate()` step that adds `position` if missing (guard on column existence, matching existing migration style)
- [x] 1.3 Backfill `position` for existing rows: per status, ordered deterministically (e.g. by `updated_at`/`id`), stepped with wide spacing (~1000)
- [x] 1.4 Add a unit test verifying migration adds + backfills `position` and is idempotent on a second run

## 2. Position-aware thing queries

- [x] 2.1 Add `internal/db` query: list `things` grouped by status, ordered by `position` ascending, excluding `dropped`
- [x] 2.2 Add query/helper to set a single thing's `position` to a fractional value between two neighbors (single-row update)
- [x] 2.3 Add a column renumber helper (restore wide spacing) used both as the no-gap fallback and by backfill
- [x] 2.4 Add a cross-column move helper: set `status` + `position` + bump `updated_at`; stamp `completed_at` on move into Done, clear it on move out of Done — single operation
- [x] 2.5 Table-driven tests for reorder (one row changes), no-gap renumber fallback, and cross-column move (status/position/completed_at transitions)

## 3. Habit grid reads

- [x] 3.1 Add `internal/db` query that returns, per active habit, the set of `habit_logs` dates within the chosen window (read-only)
- [x] 3.2 Add a helper that shapes the query result into grid rows (habit × date cells) for templating
- [x] 3.3 Unit test: filled cell where a log exists, empty where none, and confirm no write path is invoked

## 4. Web foundation (this change owns it)

- [x] 4.1 Create the `internal/web` package: a `Server` holding `*db.DB`, with a `New(*db.DB)` constructor and a `Handler()`/`http.Handler` exposing the routes
- [x] 4.2 Add a base `html/template` layout (parsed once at startup) and a template set embedded via `embed.FS` (parsed per-page: layout+page, so each page's "content" is distinct)
- [x] 4.3 Vendor htmx + Sortable.js as pinned static files under `internal/web/static/`, embedded via `embed.FS` and served at `/static/` (no npm, no front-end build step)
- [x] 4.4 Wire `WEB_PORT` into `config.Config` (new field, read from env) so the foundation has a port to bind

## 5. Web layer — kanban

- [x] 5.1 Add kanban `html/template` view: three columns (Ideas/Active/Done) rendering cards from the position-ordered query
- [x] 5.2 Add htmx + Sortable.js wiring on the columns for drag-and-drop (using the vendored static assets)
- [x] 5.3 Add handler for within-column reorder → call fractional-position helper (with renumber fallback)
- [x] 5.4 Add handler for cross-column move → call cross-column move helper
- [x] 5.5 ~~Handlers return the updated card/column fragment for htmx swap~~ — DEVIATION: `/things/move` returns `204 No Content`. Sortable.js already moves the card in the DOM optimistically; returning a fragment to swap would fight that. Errors surface as non-2xx. Revisit if we add server-side validation that can reject a move.

## 6. Web layer — habit grid

- [x] 6.1 Add read-only habit grid `html/template` view rendering filled/empty cells from the grid query
- [x] 6.2 Route the grid view; confirm it issues no write to `habit_logs`

## 7. Serving & exposure

- [x] 7.1 Start the web server from `cmd/agent/main.go` (goroutine) only when `WEB_PORT` is set, sharing the existing DB handle, in both CLI and bot modes
- [x] 7.2 Bind to loopback/tailnet interface; no application-level auth in MVP (bare port → 127.0.0.1; host:port → that interface)
- [x] 7.3 Confirm binary with no `WEB_PORT` set leaves CLI/Discord/scheduler behavior unchanged (no server started)
- [x] 7.4 Document `WEB_PORT` in `config.example.yaml` / `.env` notes and update CLAUDE.md project structure for `internal/web`

## 8. Verification

- [x] 8.1 `go test ./...` passes (migration, position, grid, move tests)
- [x] 8.2 `go vet ./...` clean
- [x] 8.3 Manual smoke: run with `WEB_PORT` set, verify board columns/order, drag within + across columns persists, Done stamps `completed_at`, grid reflects `habit_logs`
