## 1. Schema & migration

- [x] 1.1 In `internal/db/schema.sql`, remove the `conversations`, `conversation_summaries`, and `memories` table definitions, the `memories_fts` virtual table, and the three triggers (`memories_ai`, `memories_ad`, `memories_au`).
- [x] 1.2 In `internal/db/db.go` `migrate()`, add idempotent drops for existing DBs: `DROP TRIGGER IF EXISTS memories_ai/memories_ad/memories_au`, `DROP TABLE IF EXISTS memories_fts`, then `DROP TABLE IF EXISTS memories`, `conversations`, `conversation_summaries`.
- [x] 1.3 Remove the FTS5 backfill statement from startup (if present in `db.go`) so it no longer references `memories`/`memories_fts`.

## 2. Query layer

- [x] 2.1 Delete `internal/db/queries_conversations.go` and `internal/db/queries_conversations_test.go`.
- [x] 2.2 Delete `internal/db/queries_memories.go`.
- [x] 2.3 In `internal/db/queries_helpers.go`, remove `memories` from `allowedColumns`.
- [x] 2.4 Remove memory-related cases from `internal/db/queries_test.go`.

## 3. LLM tools & prompt

- [x] 3.1 In `internal/llm/tools.go`, remove the five memory tool definitions (`save_memory`, `search_memories`, `list_recent_memories`, `update_memory`, `delete_memory`).
- [x] 3.2 In `internal/llm/prompt.go`, strip all memory/summary/cross-conversation-continuity language from the system prompt.
- [x] 3.3 Update `internal/llm/prompt_test.go` to match the pared prompt (remove assertions on memory wording).

## 4. Agent (statelessness)

- [x] 4.1 In `internal/agent/conversation.go`, delete `RunWithConversation`, `Summarize`, and gap-detection/summary logic. Delete the file if nothing remains.
- [x] 4.2 In `internal/agent/agent.go`, remove any memory tool dispatch/handlers and memory references.
- [x] 4.3 Update `internal/agent/agent_test.go` to remove conversation/memory tests.

## 5. Callers

- [x] 5.1 In `cmd/agent/main.go`, replace `RunWithConversation` usage (CLI REPL) with stateless `Run`.
- [x] 5.2 In `internal/discord/handlers.go`, replace `RunWithConversation` with `Run` and remove conversation-persistence wiring.
- [x] 5.3 In `internal/scheduler/scheduler.go`, replace `RunWithConversation` with `Run`, and remove the `PruneOldSummaries` call / summary-pruning from `pruneOldData()`.

## 6. Evals

- [x] 6.1 In `eval/eval.go`, remove memory seeding/assertion helpers tied to the dropped tables.
- [x] 6.2 In `eval/cases.json`, remove eval cases that exercise memory tools or conversation continuity.

## 7. Verify

- [x] 7.1 Run `go build ./...` and `go vet ./...`; fix any remaining references to removed symbols.
- [x] 7.2 Run `go test ./...` and confirm the suite is green.
- [x] 7.3 Smoke-test: `echo "idea: redesign onboarding" | go run ./cmd/agent` produces a single `create_thing` with `status='open'` and no memory/summary activity.
- [x] 7.4 Apply the migration against a copy of a populated `data.db` and confirm the dropped tables/triggers are gone with no errors.
- [x] 7.5 Update `CLAUDE.md` (tool count, schema, structure) and the memory index to reflect the removed layer.
