## Context

Jot today is a stateful chat-first assistant. `agent.RunWithConversation` loads per-user history from the `conversations` table, prepends up to 3 recent summaries from `conversation_summaries`, and on a >10 min gap calls `Summarize` to compress and clear raw messages. Free-text memories live in `memories` with an FTS5 mirror (`memories_fts`) kept in sync by three triggers, exposed via five LLM tools. The scheduler also prunes old summaries and persists reminder/watch runs through the same conversation path.

The product is being reoriented to web-primary, structured data. This change removes the conversation-continuity and free-text-memory machinery, leaving a stateless command surface used mainly for Discord quick-capture, while reminders and watches keep firing to Discord unchanged.

## Goals / Non-Goals

**Goals:**
- Drop `conversations`, `conversation_summaries`, `memories` (+ `memories_fts` + triggers) from the schema, with a migration that also drops them on existing DBs.
- Delete the conversation/memory query layer and the five memory LLM tools.
- Revert all callers (Discord handlers, scheduler, CLI REPL) to a stateless `Run`.
- Pare the system prompt; keep the agent capable of `create_thing` quick-capture plus schedule and watch management.
- Keep `schedules`, `watches`/`watch_results`, `notes`, and all `things` behavior intact.

**Non-Goals:**
- The `notes` → env-var migration (separate cleanup).
- Any web UI.
- The `log_habit` tool / habits (arrives in a later proposal).
- Migrating existing memory or conversation rows into `things` (no data carry-over).

## Decisions

**Decision: Keep the 6 watch-management tools exposed to the agent.**
Rationale: Watches are unchanged and remain a high-value push feature; keeping their tools lets the agent create/run/inspect watches conversationally. Alternative considered — making watches config-only and dropping the 6 tools to shrink the surface to 5. Rejected for now: watches stay user-managed via Discord, and config-only management would force a YAML edit + restart for every change. Final tool set: `create_thing` + 4 schedule tools + 6 watch tools = 11.

**Decision: Replace `RunWithConversation` with stateless `Run` at every call site rather than leaving a no-op shim.**
Rationale: A shim that ignores history would silently keep the abstraction and confuse future readers. Deleting `RunWithConversation`/`Summarize` and updating callers makes the statelessness explicit and removes the only consumers of the dropped tables. Alternative — keep `RunWithConversation` but stub persistence — rejected as it preserves dead surface area.

**Decision: Migration drops tables idempotently via `DROP TABLE IF EXISTS` (plus dropping `memories_fts` and the three triggers) in `migrate()`.**
Rationale: The existing `migrate()` already handles destructive schema evolution (it dropped skills/habits/check_ins tables in the prior simplification), so this follows the established pattern. FTS5 virtual table and triggers must be dropped before/with the base table to avoid dangling references. Alternative — a versioned migration framework — rejected as overkill for a single-file SQLite app.

**Decision: Remove `memories` from `allowedColumns` in `queries_helpers.go`.**
Rationale: `updateRow` is driven by `allowedColumns`; leaving `memories` there after dropping the table would be a latent bug. `things` remains the only entry alongside any others still valid.

## Risks / Trade-offs

- **[Existing DBs lose memory/conversation data irreversibly on first run]** → Documented as a BREAKING change with **Migration** notes in the spec; users wanting to keep a memory must re-capture it as a thing before upgrading. Acceptable given the product reorientation.
- **[A missed call site still references `RunWithConversation` and fails to compile]** → Mitigated by `go build ./... && go vet ./...` after the deletion; the compiler enumerates every caller.
- **[Eval/unit tests reference removed tables or tools and break the suite]** → Tasks include removing memory/conversation tests and eval cases; run `go test ./...` to confirm green.
- **[FTS5 triggers or the virtual table left undropped cause migration errors on existing DBs]** → Explicitly drop `memories_fts` and the three triggers in `migrate()` and verify against a copy of a populated `data.db`.

## Migration Plan

1. Update `schema.sql` (remove tables/index/triggers) for fresh DBs.
2. Extend `migrate()` to `DROP TABLE IF EXISTS conversations, conversation_summaries, memories;` plus `DROP TABLE IF EXISTS memories_fts;` and `DROP TRIGGER IF EXISTS memories_ai/ad/au;`.
3. Delete query files and memory tools; update `allowedColumns`.
4. Replace `RunWithConversation`/`Summarize` usages with `Run`; delete those functions and `conversation.go` if it becomes empty.
5. Strip the system prompt; remove summary-pruning from the scheduler.
6. Remove memory/conversation tests and eval cases.
7. `go build ./... && go vet ./... && go test ./...`; smoke-test CLI quick-capture and a reminder fire.

Rollback: revert the branch; the dropped tables are not recreated automatically, so a rollback on an already-migrated DB leaves the old tables absent — acceptable since they are unused after this change.

## Open Questions

- None blocking. The watch-tool exposure question is resolved (keep exposed).
