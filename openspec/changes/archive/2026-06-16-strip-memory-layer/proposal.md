## Why

Jot is being reoriented from a chat-first personal assistant into a web-primary tool for viewing and organizing ideas, projects, and habits. The conversational-memory layer — persistent history, auto-summaries, and free-text memories — has stopped earning its place: it grew large, and remembering preferences/observations is no longer a goal for Jot. Everything worth remembering will live in structured tables, so the LLM should be reduced to a thin, stateless command surface.

## What Changes

- **BREAKING** Remove the `conversations` table and all persistent conversation history. The agent no longer carries state between messages.
- **BREAKING** Remove the `conversation_summaries` table and the gap-detection auto-summarization logic (the >10 min trigger) and its summary pruning.
- **BREAKING** Remove the `memories` table, its `memories_fts` FTS5 virtual table, and the three sync triggers (`memories_ai`, `memories_ad`, `memories_au`).
- **BREAKING** Remove the 5 memory LLM tools: `save_memory`, `search_memories`, `list_recent_memories`, `update_memory`, `delete_memory`.
- Pare the system prompt to remove all memory/summary/continuity references.
- Reduce the agent to a stateless command interface: surviving tools are `create_thing` (quick capture), the 4 schedule tools, and the 6 watch tools (11 total).
- **KEPT (unchanged)**: `schedules` (reminders), `watches` / `watch_results`, `notes` (internal config), and all `things` behavior.

## Capabilities

### New Capabilities

- `stateless-agent`: The agent's runtime behavior as a stateless, single-exchange command surface — no conversation persistence, no summarization, no memory tools. Defines the surviving tool set and the prompt contract.

### Modified Capabilities

<!-- No pre-existing specs in openspec/specs/; nothing to modify. -->

## Impact

- **Schema** (`internal/db/schema.sql`): drop `conversations`, `conversation_summaries`, `memories`, `memories_fts`, and memory triggers. Migration (`internal/db/db.go`) must `DROP TABLE` these for existing DBs.
- **Queries**: delete `internal/db/queries_conversations.go` and `internal/db/queries_memories.go`; remove `memories` from `allowedColumns` in `queries_helpers.go`.
- **LLM tools** (`internal/llm/tools.go`): remove the 5 memory tool definitions.
- **System prompt** (`internal/llm/prompt.go`): strip memory/summary/continuity language.
- **Agent** (`internal/agent/conversation.go`): remove `RunWithConversation`, `Summarize`, and gap-detection; callers (Discord handlers, scheduler, CLI REPL) revert to stateless `Run`.
- **Discord** (`internal/discord/handlers.go`) and **scheduler** (`internal/scheduler/scheduler.go`): drop conversation-persistence wiring and summary pruning.
- **Tests/evals**: remove memory/conversation unit tests and eval cases.
- No new external access; SQLite-only sandbox identity preserved.
