## Context

Jot is a single-binary Go app: Discord bot + stateless agent + scheduler + watch
runner, all over one SQLite DB (`modernc.org/sqlite`, no CGO). It is reorienting
to be web-primary — the web app becomes where the user looks at and organizes
ideas/projects (a board) and habits (a grid), while Discord drops to capture +
push notifications.

This change is run 3 of 3. It depends on:
- Proposal 1 (foundational web layer): the `internal/web` package, `WEB_PORT`
  wiring into `cmd/agent/main.go`, base templates/static asset serving, and the
  htmx + Sortable.js plumbing. This change builds the two MVP views on top of it.
- Proposal 2 (`habit-tracking`, already merged): the `habits` / `habit_logs`
  tables and `log_habit`. The grid reads those tables.

Constraints: no Node, no SPA, no build step in the MVP; SQLite-only sandbox
identity preserved; pure-Go driver retained so future containerization stays
simple (Ollama stays on the Mac Mini host, reachable via `host.docker.internal`).

## Goals / Non-Goals

**Goals:**
- A three-column kanban over `things` (Ideas/Active/Done; dropped archived off-board) with drag-and-drop that persists order and cross-column status changes.
- Manual ordering of `things` via a new `position` column, persisted per drop.
- A read-only per-habit calendar grid over `habit_logs`.
- Keep the single-binary, SQLite-only, no-build-step shape intact.

**Non-Goals:**
- Charts and calendar *views* (out of MVP).
- Click-to-toggle habit logging from the grid (read-only in MVP; logging stays in `log_habit`).
- React / SPA / any front-end build step. A React island is reserved strictly for possible future interactive charts — not the foundation.
- Application-level authentication (delegated to the network layer in MVP).

## Decisions

### Templating: `html/template` over `templ`

Use the stdlib `html/template`. Rationale: lowest friction, zero new
dependencies, and — critically — **no codegen step**, which preserves the
"no separate ui build step" identity. `templ` offers type-safe components but requires a
`templ generate` step and a new toolchain dependency; the MVP's two views don't
justify that. Revisit if the template surface grows large or starts duplicating
logic that types would catch.

### Position storage: fractional/sparse over integer renumber

On drop, set the moved card's `position` to a value between its new neighbors,
touching only that one row. Seed positions with wide spacing (e.g. integer steps
of ~1000, or a REAL column) so most drops just pick a midpoint. Provide a
**renumber fallback**: when no representable value fits between neighbors (gap
collapse, or float precision exhausted), renumber that one column to restore
spacing, then place the card. Rationale: cheap, single-row writes for the common
case; bounded worst case. Integer-renumber-every-drop was rejected because it
rewrites the whole column on each move — needless write amplification for a
board that will rarely have many cards per column, but the pattern is wasteful
and the fractional approach is barely more code given the fallback is shared with
backfill.

### Habit grid: read-only in MVP

The grid derives every cell from `habit_logs` and never writes. Rationale: the
web app's stated job is "where I look"; logging already has a single owner
(`log_habit`, via Discord/agent). Keeping the write path single-sourced avoids
two surfaces racing on `UNIQUE(habit_id, date)` and keeps this change small.
Click-to-toggle is a clean future addition that would reuse the exact `log_habit`
insert/delete path.

### Exposure: loopback/Tailscale, no app auth

Bind `WEB_PORT` to loopback or the tailnet interface and rely on Tailscale ACLs.
No application-level auth in the MVP. Rationale: single-user sandbox; the network
layer is the right place for access control and avoids building session/secret
handling now. The spec explicitly forbids assuming it's safe to bind to a public
interface unprotected, which is the guardrail if exposure ever changes.

### Cross-column move = one persisted operation

A cross-column drag writes `status`, `position`, and `updated_at` together, and
stamps/clears `completed_at` based on whether the target is Done. Doing it in one
DB operation keeps the board and the underlying `things` semantics consistent
with how `complete_thing` already behaves (it stamps `completed_at`).

## Risks / Trade-offs

- **Float precision / gap collapse in fractional positions** → Renumber-column fallback (shared with backfill) bounds the worst case; spec has a scenario for it.
- **Read-only grid feels inert** → Acceptable for MVP given the "look, don't log here" framing; toggle is a small, well-isolated follow-up.
- **No app auth** → Mitigated by binding to loopback/tailnet only; spec forbids unprotected public binding. If exposure changes, app-auth becomes a required follow-up.
- **htmx + Sortable.js as vendored assets** → Pin versions and vendor them as static files so there is still no npm/build step; depends on proposal 1 having set up static serving.
- **Two processes (web + scheduler) writing the same SQLite file** → SQLite single-writer; keep web writes short (single-row position/status updates) and reuse the existing DB connection handling to avoid `database is locked`.

## Migration Plan

1. Add `position` to `things` via an idempotent `migrate()` step (guard on column existence, consistent with how existing migrations check before altering).
2. Backfill existing rows with deterministic, well-spaced positions (e.g. per-status, ordered by current `updated_at`/`id`, stepped by ~1000).
3. New columns/behavior are additive; rollback is dropping the web server (unset `WEB_PORT`) — the `position` column is harmless to leave in place.

## Open Questions

- Date window for the habit grid (rolling N days vs current month) — pick the smallest useful default in implementation; not a spec-level concern.
- Whether `position` is INTEGER (stepped) or REAL — both satisfy the spec; choose during implementation based on the migrate/backfill helper that's cleanest.
